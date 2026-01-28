package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	platformv1alpha1 "github.com/bdchatham/AphexPlatformInfrastructure/platform/platform-controller/controller/api/v1alpha1"
	"github.com/bdchatham/AphexPlatformInfrastructure/platform/platform-controller/controller/controllers/config"
	"github.com/bdchatham/AphexPlatformInfrastructure/platform/platform-controller/controller/controllers/constants"
	"github.com/bdchatham/AphexPlatformInfrastructure/platform/platform-controller/controller/controllers/helpers"
	"github.com/bdchatham/AphexPlatformInfrastructure/platform/platform-controller/controller/controllers/metrics"
)

// KnowledgeBaseReconciler reconciles a KnowledgeBase object
type KnowledgeBaseReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	Log             logr.Logger
	Config          *config.Config
	statusHelper    *helpers.StatusHelper
	finalizerHelper *helpers.FinalizerHelper
}

// +kubebuilder:rbac:groups=aphex.io,resources=knowledgebases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=aphex.io,resources=knowledgebases/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=aphex.io,resources=knowledgebases/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// Reconcile manages KnowledgeBase resources
func (r *KnowledgeBaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Start metrics timer for reconciliation duration
	metricsCollector := metrics.GetMetricsCollector()
	timer := metricsCollector.NewReconcileTimer(constants.ControllerNameKnowledgeBase)

	if err := r.ensureHelpers(logger); err != nil {
		logger.Error(err, "Failed to initialize helpers")
		timer.ObserveError(metrics.ClassifyError(err))
		return ctrl.Result{}, err
	}

	timeout := constants.DefaultProvisioningTimeout
	if r.Config != nil {
		timeout = r.Config.ProvisioningTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	kb := &platformv1alpha1.KnowledgeBase{}
	err := r.Get(ctx, req.NamespacedName, kb)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get KnowledgeBase")
		timer.ObserveError(metrics.ClassifyError(err))
		return ctrl.Result{}, err
	}

	if r.finalizerHelper.IsBeingDeleted(kb) {
		result, err := r.handleDeletionWithHelper(ctx, logger, kb)
		if err != nil {
			timer.ObserveError(metrics.ClassifyError(err))
		} else {
			timer.ObserveSuccess()
			metrics.GetHealthState().RecordSuccess(constants.ControllerNameKnowledgeBase)
		}
		return result, err
	}

	if !r.finalizerHelper.HasFinalizer(kb) {
		if err := r.validateSpec(kb); err != nil {
			logger.Error(err, "Validation failed, not adding finalizer")
			timer.ObserveError("validation")
			if patchErr := r.statusHelper.PatchStatus(ctx, kb, map[string]interface{}{
				"phase":   constants.PhaseFailed,
				"message": fmt.Sprintf("Validation failed: %s", err.Error()),
			}); patchErr != nil {
				logger.Error(patchErr, "Failed to update status after validation failure")
			}
			return ctrl.Result{}, nil
		}
		if err := r.finalizerHelper.EnsureFinalizer(ctx, kb); err != nil {
			timer.ObserveError(metrics.ClassifyError(err))
			return ctrl.Result{}, err
		}
		timer.ObserveSuccess()
		return ctrl.Result{Requeue: true}, nil
	}

	if kb.Status.Phase == "" {
		if err := r.statusHelper.PatchStatus(ctx, kb, map[string]interface{}{
			"phase":             constants.PhasePending,
			"message":           "Starting KnowledgeBase provisioning",
			"lastReconcileTime": metav1.Now().Format(time.RFC3339),
		}); err != nil {
			logger.Error(err, "Failed to update KnowledgeBase status to Pending")
			timer.ObserveError(metrics.ClassifyError(err))
			return ctrl.Result{}, err
		}
		logger.Info("Set KnowledgeBase phase to Pending", "name", kb.Name, "namespace", kb.Namespace)
		timer.ObserveSuccess()
		return ctrl.Result{Requeue: true}, nil
	}

	// Update status to Provisioning if still Pending
	if kb.Status.Phase == constants.PhasePending {
		if err := r.statusHelper.PatchStatus(ctx, kb, map[string]interface{}{
			"phase":             constants.PhaseProvisioning,
			"message":           "Provisioning KnowledgeBase resources",
			"lastReconcileTime": metav1.Now().Format(time.RFC3339),
		}); err != nil {
			timer.ObserveError(metrics.ClassifyError(err))
			return ctrl.Result{}, err
		}
	}

	select {
	case <-ctx.Done():
		timer.ObserveError("cancelled")
		return ctrl.Result{}, fmt.Errorf("context cancelled before provisioning: %w", ctx.Err())
	default:
	}

	if err := r.reconcileRepositoryConfig(ctx, kb); err != nil {
		logger.Error(err, "Failed to reconcile repository configuration")
		timer.ObserveError(metrics.ClassifyError(err))
		metricsCollector.RecordProvisioningStep(constants.ControllerNameKnowledgeBase, "repository_config", "error")
		if patchErr := r.statusHelper.PatchStatus(ctx, kb, map[string]interface{}{
			"phase":             constants.PhaseFailed,
			"message":           fmt.Sprintf("Repository configuration failed: %v", err),
			"lastReconcileTime": metav1.Now().Format(time.RFC3339),
		}); patchErr != nil {
			logger.Error(patchErr, "Failed to update status after repository config failure")
		}
		return ctrl.Result{}, err
	}
	metricsCollector.RecordProvisioningStep(constants.ControllerNameKnowledgeBase, "repository_config", "success")

	select {
	case <-ctx.Done():
		timer.ObserveError("cancelled")
		return ctrl.Result{}, fmt.Errorf("context cancelled during provisioning: %w", ctx.Err())
	default:
	}

	// Reconcile MCP server if configured
	if kb.Spec.MCP != nil {
		if err := r.reconcileMCPServer(ctx, kb); err != nil {
			logger.Error(err, "Failed to reconcile MCP server")
			timer.ObserveError(metrics.ClassifyError(err))
			metricsCollector.RecordProvisioningStep(constants.ControllerNameKnowledgeBase, "mcp_server", "error")
			if patchErr := r.statusHelper.PatchStatus(ctx, kb, map[string]interface{}{
				"phase":             constants.PhaseFailed,
				"message":           fmt.Sprintf("MCP server reconciliation failed: %v", err),
				"lastReconcileTime": metav1.Now().Format(time.RFC3339),
			}); patchErr != nil {
				logger.Error(patchErr, "Failed to update status after MCP server failure")
			}
			return ctrl.Result{}, err
		}
		metricsCollector.RecordProvisioningStep(constants.ControllerNameKnowledgeBase, "mcp_server", "success")
	} else {
		if err := r.cleanupMCPServer(ctx, kb); err != nil {
			logger.Error(err, "Failed to cleanup MCP server")
			timer.ObserveError(metrics.ClassifyError(err))
			return ctrl.Result{}, err
		}
	}

	// Update status to Ready using StatusHelper
	statusPatch := map[string]interface{}{
		"phase":             constants.PhaseReady,
		"message":           fmt.Sprintf("Tracking %d repositories", len(kb.Spec.Repositories)),
		"lastReconcileTime": metav1.Now().Format(time.RFC3339),
	}

	// Include MCP status if deployed
	if kb.Spec.MCP != nil {
		statusPatch["mcp"] = map[string]interface{}{
			"deployed":      kb.Status.MCP.Deployed,
			"serviceName":   kb.Status.MCP.ServiceName,
			"serviceURL":    kb.Status.MCP.ServiceURL,
			"readyReplicas": kb.Status.MCP.ReadyReplicas,
		}
	}

	if err := r.statusHelper.PatchStatus(ctx, kb, statusPatch); err != nil {
		logger.Error(err, "Failed to update KnowledgeBase status to Ready")
		timer.ObserveError(metrics.ClassifyError(err))
		return ctrl.Result{}, err
	}

	timer.ObserveSuccess()
	metrics.GetHealthState().RecordSuccess(constants.ControllerNameKnowledgeBase)
	logger.Info("KnowledgeBase reconciled successfully",
		"name", kb.Name,
		"namespace", kb.Namespace,
		"repositories", len(kb.Spec.Repositories),
	)

	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}


// ensureHelpers initializes the helper components if not already done.
func (r *KnowledgeBaseReconciler) ensureHelpers(logger logr.Logger) error {
	if r.statusHelper == nil {
		retryCount := constants.DefaultStatusRetryCount
		if r.Config != nil {
			retryCount = r.Config.StatusRetryCount
		}
		r.statusHelper = helpers.NewStatusHelper(r.Client, logger, retryCount)
	}

	if r.finalizerHelper == nil {
		r.finalizerHelper = helpers.NewFinalizerHelper(r.Client, logger, constants.KnowledgeBaseFinalizer)
	}

	return nil
}

// validateSpec validates the KnowledgeBase spec before adding finalizer.
func (r *KnowledgeBaseReconciler) validateSpec(kb *platformv1alpha1.KnowledgeBase) error {
	if len(kb.Spec.Repositories) == 0 {
		return fmt.Errorf("repositories array cannot be empty")
	}

	for i, repo := range kb.Spec.Repositories {
		if repo.URL == "" {
			return fmt.Errorf("repository[%d]: URL cannot be empty", i)
		}

		if !strings.HasPrefix(repo.URL, "http://") && !strings.HasPrefix(repo.URL, "https://") {
			return fmt.Errorf("repository[%d]: URL must start with http:// or https://", i)
		}

		if repo.Branch != "" && !isValidBranchName(repo.Branch) {
			return fmt.Errorf("repository[%d]: invalid branch name '%s'", i, repo.Branch)
		}
	}

	return nil
}

// handleDeletionWithHelper performs cleanup using FinalizerHelper with verified cleanup.
func (r *KnowledgeBaseReconciler) handleDeletionWithHelper(ctx context.Context, logger logr.Logger, kb *platformv1alpha1.KnowledgeBase) (ctrl.Result, error) {
	if !r.finalizerHelper.NeedsCleanup(kb) {
		return ctrl.Result{}, nil
	}

	cleanupSteps := []helpers.CleanupStep{
		helpers.NewCleanupStep("Repository configuration", func(ctx context.Context) error {
			return r.cleanupRepositoryConfig(ctx, kb)
		}),
		helpers.NewCleanupStep("MCP server resources", func(ctx context.Context) error {
			return r.cleanupMCPServerResources(ctx, kb)
		}),
	}

	if err := r.finalizerHelper.HandleDeletionWithSteps(ctx, kb, cleanupSteps); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("KnowledgeBase deletion completed", "knowledgebase", kb.Name)
	return ctrl.Result{}, nil
}

// cleanupMCPServerResources removes MCP server deployment and service.
func (r *KnowledgeBaseReconciler) cleanupMCPServerResources(ctx context.Context, kb *platformv1alpha1.KnowledgeBase) error {
	deploymentName := fmt.Sprintf("mcp-server-%s", kb.Name)
	serviceName := fmt.Sprintf("mcp-server-%s", kb.Name)

	// Delete MCP server deployment
	deployment := &appsv1.Deployment{}
	if err := r.Get(ctx, client.ObjectKey{Name: deploymentName, Namespace: kb.Namespace}, deployment); err == nil {
		if err := r.Delete(ctx, deployment); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete MCP server deployment: %w", err)
		}
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get MCP server deployment for cleanup: %w", err)
	}

	// Delete MCP server service
	service := &corev1.Service{}
	if err := r.Get(ctx, client.ObjectKey{Name: serviceName, Namespace: kb.Namespace}, service); err == nil {
		if err := r.Delete(ctx, service); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete MCP server service: %w", err)
		}
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get MCP server service for cleanup: %w", err)
	}

	return nil
}

// reconcileRepositoryConfig creates or updates the ConfigMap containing repository configuration.
func (r *KnowledgeBaseReconciler) reconcileRepositoryConfig(ctx context.Context, kb *platformv1alpha1.KnowledgeBase) error {
	logger := log.FromContext(ctx)

	select {
	case <-ctx.Done():
		return fmt.Errorf("context cancelled during repository config reconciliation: %w", ctx.Err())
	default:
	}

	configMapName := fmt.Sprintf("%s-repos", kb.Name)

	// Build repository configuration data
	repoData := r.buildRepositoryConfigData(kb)

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: kb.Namespace,
			Labels: map[string]string{
				constants.LabelManagedBy:      constants.ManagedByKnowledgeBaseController,
				"knowledgebase":               kb.Name,
				"app.kubernetes.io/name":      "knowledgebase-config",
				"app.kubernetes.io/instance":  kb.Name,
				"app.kubernetes.io/part-of":   "archon",
				"app.kubernetes.io/component": "config",
			},
		},
		Data: repoData,
	}

	// Set owner reference for automatic garbage collection
	if err := controllerutil.SetControllerReference(kb, configMap, r.Scheme); err != nil {
		return fmt.Errorf("failed to set controller reference on configmap: %w", err)
	}

	existingConfigMap := &corev1.ConfigMap{}
	err := r.Get(ctx, client.ObjectKey{Name: configMapName, Namespace: kb.Namespace}, existingConfigMap)
	if err != nil {
		if errors.IsNotFound(err) {
			if err := r.Create(ctx, configMap); err != nil {
				return fmt.Errorf("failed to create configmap: %w", err)
			}
			logger.Info("Created repository configuration ConfigMap", "name", configMapName, "namespace", kb.Namespace)
		} else {
			return fmt.Errorf("failed to get configmap: %w", err)
		}
	} else {
		configMap.ResourceVersion = existingConfigMap.ResourceVersion
		if err := r.Update(ctx, configMap); err != nil {
			return fmt.Errorf("failed to update configmap: %w", err)
		}
		logger.V(1).Info("Updated repository configuration ConfigMap", "name", configMapName, "namespace", kb.Namespace)
	}

	return nil
}

// buildRepositoryConfigData builds the ConfigMap data from the KnowledgeBase spec.
func (r *KnowledgeBaseReconciler) buildRepositoryConfigData(kb *platformv1alpha1.KnowledgeBase) map[string]string {
	data := make(map[string]string)

	// Store display name and description
	data["displayName"] = kb.Spec.DisplayName
	if kb.Spec.Description != "" {
		data["description"] = kb.Spec.Description
	}

	// Store repository count
	data["repositoryCount"] = fmt.Sprintf("%d", len(kb.Spec.Repositories))

	// Store each repository configuration
	for i, repo := range kb.Spec.Repositories {
		prefix := fmt.Sprintf("repo.%d.", i)
		data[prefix+"url"] = repo.URL

		branch := repo.Branch
		if branch == "" {
			branch = "main"
		}
		data[prefix+"branch"] = branch

		if len(repo.Paths) > 0 {
			data[prefix+"paths"] = strings.Join(repo.Paths, ",")
		} else {
			data[prefix+"paths"] = ".kiro/docs"
		}
	}

	return data
}

// cleanupRepositoryConfig removes the repository configuration ConfigMap.
func (r *KnowledgeBaseReconciler) cleanupRepositoryConfig(ctx context.Context, kb *platformv1alpha1.KnowledgeBase) error {
	configMapName := fmt.Sprintf("%s-repos", kb.Name)

	configMap := &corev1.ConfigMap{}
	if err := r.Get(ctx, client.ObjectKey{Name: configMapName, Namespace: kb.Namespace}, configMap); err == nil {
		if err := r.Delete(ctx, configMap); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete repository configuration ConfigMap: %w", err)
		}
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get repository configuration ConfigMap for cleanup: %w", err)
	}

	return nil
}

func (r *KnowledgeBaseReconciler) reconcileMCPServer(ctx context.Context, kb *platformv1alpha1.KnowledgeBase) error {
	logger := log.FromContext(ctx)

	select {
	case <-ctx.Done():
		return fmt.Errorf("context cancelled during MCP server reconciliation: %w", ctx.Err())
	default:
	}

	image := kb.Spec.MCP.Image
	port := kb.Spec.MCP.Port

	queryServiceURL := kb.Spec.MCP.QueryServiceURL
	if queryServiceURL == "" {
		queryServiceURL = fmt.Sprintf("http://query.%s:8080", kb.Namespace)
	}

	replicas := kb.Spec.MCP.Replicas
	if replicas == 0 {
		replicas = 1
	}

	deploymentName := fmt.Sprintf("mcp-server-%s", kb.Name)
	serviceName := fmt.Sprintf("mcp-server-%s", kb.Name)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: kb.Namespace,
			Labels: map[string]string{
				constants.LabelManagedBy:      constants.ManagedByKnowledgeBaseController,
				"app":                         "mcp-server",
				"knowledgebase":               kb.Name,
				"app.kubernetes.io/name":      "mcp-server",
				"app.kubernetes.io/instance":  kb.Name,
				"app.kubernetes.io/part-of":   "archon",
				"app.kubernetes.io/component": "mcp",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":                         "mcp-server",
					"knowledgebase":               kb.Name,
					"app.kubernetes.io/name":      "mcp-server",
					"app.kubernetes.io/instance":  kb.Name,
					"app.kubernetes.io/part-of":   "archon",
					"app.kubernetes.io/component": "mcp",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":                         "mcp-server",
						"knowledgebase":               kb.Name,
						"app.kubernetes.io/name":      "mcp-server",
						"app.kubernetes.io/instance":  kb.Name,
						"app.kubernetes.io/part-of":   "archon",
						"app.kubernetes.io/component": "mcp",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "mcp-server",
							Image: image,
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: port,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "QUERY_SERVICE_URL",
									Value: queryServiceURL,
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: mustParseQuantity("128Mi"),
									corev1.ResourceCPU:    mustParseQuantity("50m"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: mustParseQuantity("256Mi"),
									corev1.ResourceCPU:    mustParseQuantity("200m"),
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/health",
										Port: intstr.FromString("http"),
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       30,
								FailureThreshold:    3,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/health",
										Port: intstr.FromString("http"),
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       10,
								FailureThreshold:    3,
							},
						},
					},
				},
			},
		},
	}

	// Set owner reference for automatic garbage collection
	if err := controllerutil.SetControllerReference(kb, deployment, r.Scheme); err != nil {
		return fmt.Errorf("failed to set controller reference on deployment: %w", err)
	}

	existingDeployment := &appsv1.Deployment{}
	err := r.Get(ctx, client.ObjectKey{Name: deploymentName, Namespace: kb.Namespace}, existingDeployment)
	if err != nil {
		if errors.IsNotFound(err) {
			if err := r.Create(ctx, deployment); err != nil {
				return fmt.Errorf("failed to create deployment: %w", err)
			}
			logger.Info("Created MCP server deployment", "name", deploymentName, "namespace", kb.Namespace)
		} else {
			return fmt.Errorf("failed to get deployment: %w", err)
		}
	} else {
		deployment.ResourceVersion = existingDeployment.ResourceVersion
		if err := r.Update(ctx, deployment); err != nil {
			return fmt.Errorf("failed to update deployment: %w", err)
		}
		logger.V(1).Info("Updated MCP server deployment", "name", deploymentName, "namespace", kb.Namespace)
	}

	select {
	case <-ctx.Done():
		return fmt.Errorf("context cancelled during MCP server service reconciliation: %w", ctx.Err())
	default:
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: kb.Namespace,
			Labels: map[string]string{
				constants.LabelManagedBy:      constants.ManagedByKnowledgeBaseController,
				"app":                         "mcp-server",
				"knowledgebase":               kb.Name,
				"app.kubernetes.io/name":      "mcp-server",
				"app.kubernetes.io/instance":  kb.Name,
				"app.kubernetes.io/part-of":   "archon",
				"app.kubernetes.io/component": "mcp",
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       port,
					TargetPort: intstr.FromString("http"),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Selector: map[string]string{
				"app":           "mcp-server",
				"knowledgebase": kb.Name,
			},
		},
	}

	// Set owner reference for automatic garbage collection
	if err := controllerutil.SetControllerReference(kb, service, r.Scheme); err != nil {
		return fmt.Errorf("failed to set controller reference on service: %w", err)
	}

	existingService := &corev1.Service{}
	err = r.Get(ctx, client.ObjectKey{Name: serviceName, Namespace: kb.Namespace}, existingService)
	if err != nil {
		if errors.IsNotFound(err) {
			if err := r.Create(ctx, service); err != nil {
				return fmt.Errorf("failed to create service: %w", err)
			}
			logger.Info("Created MCP server service", "name", serviceName, "namespace", kb.Namespace)
		} else {
			return fmt.Errorf("failed to get service: %w", err)
		}
	} else {
		service.ResourceVersion = existingService.ResourceVersion
		service.Spec.ClusterIP = existingService.Spec.ClusterIP
		if err := r.Update(ctx, service); err != nil {
			return fmt.Errorf("failed to update service: %w", err)
		}
		logger.V(1).Info("Updated MCP server service", "name", serviceName, "namespace", kb.Namespace)
	}

	// Update MCP status
	kb.Status.MCP.Deployed = true
	kb.Status.MCP.ServiceName = serviceName
	kb.Status.MCP.ServiceURL = fmt.Sprintf("http://%s.%s:%d", serviceName, kb.Namespace, port)
	if existingDeployment.Status.ReadyReplicas > 0 {
		kb.Status.MCP.ReadyReplicas = existingDeployment.Status.ReadyReplicas
	}

	return nil
}

func (r *KnowledgeBaseReconciler) cleanupMCPServer(ctx context.Context, kb *platformv1alpha1.KnowledgeBase) error {
	logger := log.FromContext(ctx)

	deploymentName := fmt.Sprintf("mcp-server-%s", kb.Name)
	serviceName := fmt.Sprintf("mcp-server-%s", kb.Name)

	deployment := &appsv1.Deployment{}
	err := r.Get(ctx, client.ObjectKey{Name: deploymentName, Namespace: kb.Namespace}, deployment)
	if err == nil {
		if err := r.Delete(ctx, deployment); err != nil {
			return fmt.Errorf("failed to delete deployment: %w", err)
		}
		logger.Info("Deleted MCP server deployment", "name", deploymentName, "namespace", kb.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get deployment for cleanup: %w", err)
	}

	service := &corev1.Service{}
	err = r.Get(ctx, client.ObjectKey{Name: serviceName, Namespace: kb.Namespace}, service)
	if err == nil {
		if err := r.Delete(ctx, service); err != nil {
			return fmt.Errorf("failed to delete service: %w", err)
		}
		logger.Info("Deleted MCP server service", "name", serviceName, "namespace", kb.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get service for cleanup: %w", err)
	}

	kb.Status.MCP.Deployed = false
	kb.Status.MCP.ServiceName = ""
	kb.Status.MCP.ServiceURL = ""
	kb.Status.MCP.ReadyReplicas = 0

	return nil
}

func (r *KnowledgeBaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return r.SetupWithManagerAndOptions(mgr, controller.Options{})
}

// SetupWithManagerAndOptions sets up the controller with the Manager and custom options.
// This allows configuring rate limiting and max concurrent reconciles.
func (r *KnowledgeBaseReconciler) SetupWithManagerAndOptions(mgr ctrl.Manager, opts controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&platformv1alpha1.KnowledgeBase{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		WithOptions(opts).
		Complete(r)
}
