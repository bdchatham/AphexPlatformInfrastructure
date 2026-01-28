package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	esv1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	platformv1alpha1 "github.com/bdchatham/AphexPlatformInfrastructure/platform/platform-controller/controller/api/v1alpha1"
	"github.com/bdchatham/AphexPlatformInfrastructure/platform/platform-controller/controller/controllers"
	"github.com/bdchatham/AphexPlatformInfrastructure/platform/platform-controller/controller/controllers/config"
	"github.com/bdchatham/AphexPlatformInfrastructure/platform/platform-controller/controller/controllers/constants"
	"github.com/bdchatham/AphexPlatformInfrastructure/platform/platform-controller/controller/controllers/metrics"
	"github.com/bdchatham/AphexPlatformInfrastructure/platform/platform-controller/controller/controllers/webhooks"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	triggersv1beta1 "github.com/tektoncd/triggers/pkg/apis/triggers/v1beta1"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(platformv1alpha1.AddToScheme(scheme))
	utilruntime.Must(tektonv1.AddToScheme(scheme))
	utilruntime.Must(triggersv1beta1.AddToScheme(scheme))
	utilruntime.Must(esv1.AddToScheme(scheme))
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var disableLeaderElection bool
	var probeAddr string
	var enableWebhooks bool
	var webhookPort int
	var certDir string
	var approvedOrgs string
	var maxConcurrentReconciles int
	var developmentMode bool

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", true,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&disableLeaderElection, "disable-leader-elect", false,
		"Disable leader election for development environments. "+
			"This flag takes precedence over leader-elect when set to true.")
	flag.BoolVar(&enableWebhooks, "enable-webhooks", false,
		"Enable admission webhooks for validating CRD resources.")
	flag.IntVar(&webhookPort, "webhook-port", 9443, "The port the webhook server binds to.")
	flag.StringVar(&certDir, "cert-dir", "/tmp/k8s-webhook-server/serving-certs",
		"The directory containing TLS certificates for the webhook server.")
	flag.StringVar(&approvedOrgs, "approved-orgs", "bdchatham",
		"Comma-separated list of approved GitHub organizations for RepoBindings.")
	flag.IntVar(&maxConcurrentReconciles, "max-concurrent-reconciles", constants.DefaultMaxConcurrentReconciles,
		"Maximum number of concurrent reconciles per controller.")
	flag.BoolVar(&developmentMode, "development", false,
		"Enable development mode with verbose logging. Defaults to production mode.")

	opts := zap.Options{
		Development: developmentMode,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	// Check environment variable for development mode (flag takes precedence)
	if !developmentMode {
		if envDev := os.Getenv(constants.EnvDevelopmentMode); envDev != "" {
			developmentMode = envDev == "true" || envDev == "1"
			opts.Development = developmentMode
		}
	}

	// Check environment variable for leader election (flag takes precedence)
	if !disableLeaderElection {
		if envLeader := os.Getenv(constants.EnvLeaderElectionEnable); envLeader != "" {
			enableLeaderElection = envLeader == "true" || envLeader == "1"
		}
	}

	// Disable leader election flag takes precedence
	if disableLeaderElection {
		enableLeaderElection = false
	}

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// Parse approved organizations
	approvedOrgsList := parseApprovedOrgs(approvedOrgs)
	setupLog.Info("Approved organizations configured", "orgs", approvedOrgsList)

	// Log configuration mode
	if developmentMode {
		setupLog.Info("Running in development mode with verbose logging")
	} else {
		setupLog.Info("Running in production mode")
	}

	// Initialize configuration manager
	configManager := config.NewManager(setupLog)
	if err := configManager.LoadFromEnvironment(); err != nil {
		setupLog.Error(err, "Failed to load configuration")
		os.Exit(1)
	}
	controllerConfig := configManager.GetConfig()

	// Create exponential backoff rate limiter for failed reconciliations
	rateLimiter := workqueue.NewTypedItemExponentialFailureRateLimiter[ctrl.Request](
		controllerConfig.BaseDelay,
		controllerConfig.MaxDelay,
	)

	// Configure manager options
	mgrOpts := ctrl.Options{
		Scheme:                 scheme,
		Metrics:                server.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       constants.LeaderElectionID,
		GracefulShutdownTimeout: &controllerConfig.ShutdownTimeout,
	}

	// Configure webhook server if enabled
	if enableWebhooks {
		mgrOpts.WebhookServer = webhook.NewServer(webhook.Options{
			Port:    webhookPort,
			CertDir: certDir,
		})
		setupLog.Info("Webhooks enabled", "port", webhookPort, "certDir", certDir)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), mgrOpts)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Log leader election status
	if enableLeaderElection {
		setupLog.Info("Leader election enabled", "leaderElectionID", constants.LeaderElectionID)
	} else {
		setupLog.Info("Leader election disabled (development mode)")
	}

	// Controller options with rate limiting and max concurrent reconciles
	controllerOpts := controller.Options{
		MaxConcurrentReconciles: maxConcurrentReconciles,
		RateLimiter:             rateLimiter,
	}
	setupLog.Info("Controller options configured",
		"maxConcurrentReconciles", maxConcurrentReconciles,
		"baseDelay", controllerConfig.BaseDelay,
		"maxDelay", controllerConfig.MaxDelay,
	)

	// Initialize template catalog
	templateCatalog := controllers.NewTemplateCatalog()
	setupLog.Info("template catalog initialized", "templates", templateCatalog.List())

	if err = (&controllers.RepoBindingReconciler{
		Client:          mgr.GetClient(),
		Scheme:          mgr.GetScheme(),
		Log:             ctrl.Log.WithName("controllers").WithName("RepoBinding"),
		TemplateCatalog: templateCatalog,
		Config:          controllerConfig,
	}).SetupWithManagerAndOptions(mgr, controllerOpts); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "RepoBinding")
		os.Exit(1)
	}

	if err = (&controllers.OrganizationReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Config: controllerConfig,
	}).SetupWithManagerAndOptions(mgr, controllerOpts); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Organization")
		os.Exit(1)
	}

	if err = (&controllers.KnowledgeBaseReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Config: controllerConfig,
	}).SetupWithManagerAndOptions(mgr, controllerOpts); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "KnowledgeBase")
		os.Exit(1)
	}

	if err = (&controllers.AgentReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Config: controllerConfig,
	}).SetupWithManagerAndOptions(mgr, controllerOpts); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Agent")
		os.Exit(1)
	}

	// Setup webhooks if enabled
	if enableWebhooks {
		setupLog.Info("Setting up webhooks")

		// RepoBinding webhook
		repoBindingValidator := webhooks.NewRepoBindingValidator(
			ctrl.Log.WithName("webhooks").WithName("RepoBinding"),
			approvedOrgsList,
		)
		if err = repoBindingValidator.SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "RepoBinding")
			os.Exit(1)
		}

		// Organization webhook
		organizationValidator := webhooks.NewOrganizationValidator(
			ctrl.Log.WithName("webhooks").WithName("Organization"),
		)
		if err = organizationValidator.SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "Organization")
			os.Exit(1)
		}

		// Agent webhook
		agentValidator := webhooks.NewAgentValidator(
			ctrl.Log.WithName("webhooks").WithName("Agent"),
		)
		if err = agentValidator.SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "Agent")
			os.Exit(1)
		}

		// KnowledgeBase webhook
		knowledgeBaseValidator := webhooks.NewKnowledgeBaseValidator(
			ctrl.Log.WithName("webhooks").WithName("KnowledgeBase"),
		)
		if err = knowledgeBaseValidator.SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "KnowledgeBase")
			os.Exit(1)
		}

		setupLog.Info("Webhooks configured successfully")
	}

	// Add health checks using HealthState for meaningful health reporting
	healthState := metrics.GetHealthState()
	healthzChecker := func(req *http.Request) error {
		if !healthState.IsHealthy() {
			return &healthCheckError{message: "no recent successful reconciliations"}
		}
		return nil
	}
	if err := mgr.AddHealthzCheck("healthz", healthzChecker); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthzChecker); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	// Initialize metrics collector to ensure metrics are registered
	_ = metrics.GetMetricsCollector()

	// Setup graceful shutdown with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handler for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		sig := <-sigChan
		setupLog.Info("Received shutdown signal, initiating graceful shutdown",
			"signal", sig.String(),
			"shutdownTimeout", controllerConfig.ShutdownTimeout,
		)
		cancel()
	}()

	setupLog.Info("Starting manager",
		"leaderElection", enableLeaderElection,
		"developmentMode", developmentMode,
		"maxConcurrentReconciles", maxConcurrentReconciles,
	)

	// Start the manager with the signal handler context
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "Problem running manager")
		os.Exit(1)
	}

	setupLog.Info("Manager stopped gracefully")
}

// healthCheckError implements the error interface for health check failures.
type healthCheckError struct {
	message string
}

func (e *healthCheckError) Error() string {
	return e.message
}

// parseApprovedOrgs parses a comma-separated list of approved organizations.
func parseApprovedOrgs(orgs string) []string {
	if orgs == "" {
		return nil
	}
	parts := strings.Split(orgs, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
