package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
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

// AgentReconciler reconciles an Agent object
type AgentReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	Log             logr.Logger
	Config          *config.Config
	statusHelper    *helpers.StatusHelper
	finalizerHelper *helpers.FinalizerHelper
}

// +kubebuilder:rbac:groups=aphex.io,resources=agents,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=aphex.io,resources=agents/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=aphex.io,resources=agents/finalizers,verbs=update
// +kubebuilder:rbac:groups=aphex.io,resources=knowledgebases,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete

func (r *AgentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Start metrics timer for reconciliation duration
	metricsCollector := metrics.GetMetricsCollector()
	timer := metricsCollector.NewReconcileTimer(constants.ControllerNameAgent)

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

	agent := &platformv1alpha1.Agent{}
	err := r.Get(ctx, req.NamespacedName, agent)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get Agent")
		timer.ObserveError(metrics.ClassifyError(err))
		return ctrl.Result{}, err
	}

	if r.finalizerHelper.IsBeingDeleted(agent) {
		result, err := r.handleDeletionWithHelper(ctx, logger, agent)
		if err != nil {
			timer.ObserveError(metrics.ClassifyError(err))
		} else {
			timer.ObserveSuccess()
			metrics.GetHealthState().RecordSuccess(constants.ControllerNameAgent)
		}
		return result, err
	}

	if !r.finalizerHelper.HasFinalizer(agent) {
		if err := r.validateAgent(agent); err != nil {
			logger.Error(err, "Validation failed, not adding finalizer")
			timer.ObserveError("validation")
			if patchErr := r.statusHelper.PatchStatus(ctx, agent, map[string]interface{}{
				"phase":   constants.PhaseFailed,
				"message": fmt.Sprintf("Validation failed: %s", err.Error()),
			}); patchErr != nil {
				logger.Error(patchErr, "Failed to update status after validation failure")
			}
			return ctrl.Result{}, nil
		}
		if err := r.finalizerHelper.EnsureFinalizer(ctx, agent); err != nil {
			timer.ObserveError(metrics.ClassifyError(err))
			return ctrl.Result{}, err
		}
		timer.ObserveSuccess()
		return ctrl.Result{Requeue: true}, nil
	}

	if agent.Status.Phase == "" {
		if err := r.statusHelper.PatchStatus(ctx, agent, map[string]interface{}{
			"phase":             constants.PhasePending,
			"message":           "Starting agent provisioning",
			"lastReconcileTime": metav1.Now().Format(time.RFC3339),
		}); err != nil {
			logger.Error(err, "Failed to update Agent status to Pending")
			timer.ObserveError(metrics.ClassifyError(err))
			return ctrl.Result{}, err
		}
		logger.Info("Set Agent phase to Pending", "name", agent.Name, "namespace", agent.Namespace)
		timer.ObserveSuccess()
		return ctrl.Result{Requeue: true}, nil
	}

	// Update status to Provisioning if still Pending
	if agent.Status.Phase == constants.PhasePending {
		if err := r.statusHelper.PatchStatus(ctx, agent, map[string]interface{}{
			"phase":             constants.PhaseProvisioning,
			"message":           "Provisioning agent resources",
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

	if err := r.reconcileModelServer(ctx, agent); err != nil {
		logger.Error(err, "Failed to reconcile model server")
		timer.ObserveError(metrics.ClassifyError(err))
		metricsCollector.RecordProvisioningStep(constants.ControllerNameAgent, "model_server", "error")
		if patchErr := r.statusHelper.PatchStatus(ctx, agent, map[string]interface{}{
			"phase":             constants.PhaseFailed,
			"message":           fmt.Sprintf("Model server reconciliation failed: %v", err),
			"lastReconcileTime": metav1.Now().Format(time.RFC3339),
		}); patchErr != nil {
			logger.Error(patchErr, "Failed to update status after model server failure")
		}
		return ctrl.Result{}, err
	}
	metricsCollector.RecordProvisioningStep(constants.ControllerNameAgent, "model_server", "success")

	select {
	case <-ctx.Done():
		timer.ObserveError("cancelled")
		return ctrl.Result{}, fmt.Errorf("context cancelled during provisioning: %w", ctx.Err())
	default:
	}

	if agent.Spec.Orchestration != nil {
		if err := r.reconcileOrchestrator(ctx, agent); err != nil {
			logger.Error(err, "Failed to reconcile orchestrator")
			timer.ObserveError(metrics.ClassifyError(err))
			metricsCollector.RecordProvisioningStep(constants.ControllerNameAgent, "orchestrator", "error")
			if patchErr := r.statusHelper.PatchStatus(ctx, agent, map[string]interface{}{
				"phase":             constants.PhaseFailed,
				"message":           fmt.Sprintf("Orchestrator reconciliation failed: %v", err),
				"lastReconcileTime": metav1.Now().Format(time.RFC3339),
			}); patchErr != nil {
				logger.Error(patchErr, "Failed to update status after orchestrator failure")
			}
			return ctrl.Result{}, err
		}
		metricsCollector.RecordProvisioningStep(constants.ControllerNameAgent, "orchestrator", "success")
	} else {
		if err := r.cleanupOrchestrator(ctx, agent); err != nil {
			logger.Error(err, "Failed to cleanup orchestrator")
			timer.ObserveError(metrics.ClassifyError(err))
			return ctrl.Result{}, err
		}
	}

	// Update status to Ready using StatusHelper
	if err := r.statusHelper.PatchStatus(ctx, agent, map[string]interface{}{
		"phase":             constants.PhaseReady,
		"message":           "Agent is ready",
		"lastReconcileTime": metav1.Now().Format(time.RFC3339),
	}); err != nil {
		logger.Error(err, "Failed to update Agent status to Ready")
		timer.ObserveError(metrics.ClassifyError(err))
		return ctrl.Result{}, err
	}

	timer.ObserveSuccess()
	metrics.GetHealthState().RecordSuccess(constants.ControllerNameAgent)
	logger.Info("Agent reconciled successfully", "name", agent.Name, "namespace", agent.Namespace)

	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (r *AgentReconciler) ensureHelpers(logger logr.Logger) error {
	if r.statusHelper == nil {
		retryCount := constants.DefaultStatusRetryCount
		if r.Config != nil {
			retryCount = r.Config.StatusRetryCount
		}
		r.statusHelper = helpers.NewStatusHelper(r.Client, logger, retryCount)
	}

	if r.finalizerHelper == nil {
		r.finalizerHelper = helpers.NewFinalizerHelper(r.Client, logger, constants.AgentFinalizer)
	}

	return nil
}

func (r *AgentReconciler) validateAgent(agent *platformv1alpha1.Agent) error {
	if agent.Spec.DisplayName == "" {
		return fmt.Errorf("agent displayName cannot be empty")
	}
	if agent.Spec.Model.Name == "" {
		return fmt.Errorf("agent model name cannot be empty")
	}
	if agent.Spec.Model.Provider == "" {
		return fmt.Errorf("agent model provider cannot be empty")
	}
	return nil
}

func (r *AgentReconciler) handleDeletionWithHelper(ctx context.Context, logger logr.Logger, agent *platformv1alpha1.Agent) (ctrl.Result, error) {
	if !r.finalizerHelper.NeedsCleanup(agent) {
		return ctrl.Result{}, nil
	}

	cleanupSteps := []helpers.CleanupStep{
		helpers.NewCleanupStep("Orchestrator resources", func(ctx context.Context) error {
			return r.cleanupOrchestratorResources(ctx, agent)
		}),
		helpers.NewCleanupStep("Model server resources", func(ctx context.Context) error {
			return r.cleanupModelServerResources(ctx, agent)
		}),
	}

	if err := r.finalizerHelper.HandleDeletionWithSteps(ctx, agent, cleanupSteps); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("Agent deletion completed", "agent", agent.Name)
	return ctrl.Result{}, nil
}

// cleanupOrchestratorResources removes orchestrator deployment and service.
func (r *AgentReconciler) cleanupOrchestratorResources(ctx context.Context, agent *platformv1alpha1.Agent) error {
	deploymentName := agent.Name
	serviceName := agent.Name

	// Delete orchestrator deployment
	deployment := &appsv1.Deployment{}
	if err := r.Get(ctx, client.ObjectKey{Name: deploymentName, Namespace: agent.Namespace}, deployment); err == nil {
		if err := r.Delete(ctx, deployment); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete orchestrator deployment: %w", err)
		}
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get orchestrator deployment for cleanup: %w", err)
	}

	// Delete orchestrator service
	service := &corev1.Service{}
	if err := r.Get(ctx, client.ObjectKey{Name: serviceName, Namespace: agent.Namespace}, service); err == nil {
		if err := r.Delete(ctx, service); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete orchestrator service: %w", err)
		}
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get orchestrator service for cleanup: %w", err)
	}

	return nil
}

// cleanupModelServerResources removes model server deployment, service, and PVC.
func (r *AgentReconciler) cleanupModelServerResources(ctx context.Context, agent *platformv1alpha1.Agent) error {
	deploymentName := fmt.Sprintf("%s-model", agent.Name)
	serviceName := fmt.Sprintf("%s-model", agent.Name)
	pvcName := fmt.Sprintf("%s-model-cache", agent.Name)

	// Delete model server deployment
	deployment := &appsv1.Deployment{}
	if err := r.Get(ctx, client.ObjectKey{Name: deploymentName, Namespace: agent.Namespace}, deployment); err == nil {
		if err := r.Delete(ctx, deployment); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete model server deployment: %w", err)
		}
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get model server deployment for cleanup: %w", err)
	}

	// Delete model server service
	service := &corev1.Service{}
	if err := r.Get(ctx, client.ObjectKey{Name: serviceName, Namespace: agent.Namespace}, service); err == nil {
		if err := r.Delete(ctx, service); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete model server service: %w", err)
		}
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get model server service for cleanup: %w", err)
	}

	// Delete model cache PVC
	pvc := &corev1.PersistentVolumeClaim{}
	if err := r.Get(ctx, client.ObjectKey{Name: pvcName, Namespace: agent.Namespace}, pvc); err == nil {
		if err := r.Delete(ctx, pvc); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete model cache PVC: %w", err)
		}
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get model cache PVC for cleanup: %w", err)
	}

	return nil
}


func (r *AgentReconciler) reconcileModelServer(ctx context.Context, agent *platformv1alpha1.Agent) error {
	logger := log.FromContext(ctx)

	select {
	case <-ctx.Done():
		return fmt.Errorf("context cancelled during model server reconciliation: %w", ctx.Err())
	default:
	}

	pvcName := fmt.Sprintf("%s-model-cache", agent.Name)
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: agent.Namespace,
			Labels: map[string]string{
				constants.LabelManagedBy: constants.ManagedByAgentController,
				"agent":                  agent.Name,
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: mustParseQuantity("50Gi"),
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(agent, pvc, r.Scheme); err != nil {
		return fmt.Errorf("failed to set controller reference on PVC: %w", err)
	}

	existingPVC := &corev1.PersistentVolumeClaim{}
	err := r.Get(ctx, client.ObjectKey{Name: pvcName, Namespace: agent.Namespace}, existingPVC)
	if err != nil {
		if errors.IsNotFound(err) {
			if err := r.Create(ctx, pvc); err != nil {
				return fmt.Errorf("failed to create PVC: %w", err)
			}
			logger.Info("Created model cache PVC", "name", pvcName, "namespace", agent.Namespace)
		} else {
			return fmt.Errorf("failed to get PVC: %w", err)
		}
	}

	port := agent.Spec.Model.Port
	if port == 0 {
		port = 8000
	}

	image := agent.Spec.Model.Image
	if image == "" {
		image = "vllm/vllm-openai:v0.14.1-cu130"
	}

	deploymentName := fmt.Sprintf("%s-model", agent.Name)
	serviceName := fmt.Sprintf("%s-model", agent.Name)
	replicas := int32(1)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: agent.Namespace,
			Labels: map[string]string{
				constants.LabelManagedBy:      constants.ManagedByAgentController,
				"app":                         "model-server",
				"agent":                       agent.Name,
				"app.kubernetes.io/name":      "model-server",
				"app.kubernetes.io/instance":  agent.Name,
				"app.kubernetes.io/part-of":   "archon",
				"app.kubernetes.io/component": "model",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":                         "model-server",
					"agent":                       agent.Name,
					"app.kubernetes.io/name":      "model-server",
					"app.kubernetes.io/instance":  agent.Name,
					"app.kubernetes.io/part-of":   "archon",
					"app.kubernetes.io/component": "model",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":                         "model-server",
						"agent":                       agent.Name,
						"app.kubernetes.io/name":      "model-server",
						"app.kubernetes.io/instance":  agent.Name,
						"app.kubernetes.io/part-of":   "archon",
						"app.kubernetes.io/component": "model",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "model-server",
							Image: image,
							Args: []string{
								"--model", agent.Spec.Model.Name,
								"--host", "0.0.0.0",
								"--port", fmt.Sprintf("%d", port),
								"--gpu-memory-utilization", "0.9",
								"--max-model-len", "8192",
							},
							Env: []corev1.EnvVar{
								{
									Name:  "NCCL_P2P_DISABLE",
									Value: "1",
								},
								{
									Name:  "NCCL_IB_DISABLE",
									Value: "1",
								},
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: port,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: mustParseQuantity("8Gi"),
									corev1.ResourceCPU:    mustParseQuantity("2"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: mustParseQuantity("16Gi"),
									corev1.ResourceCPU:    mustParseQuantity("4"),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "model-cache",
									MountPath: "/root/.cache/huggingface",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "model-cache",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: fmt.Sprintf("%s-model-cache", agent.Name),
								},
							},
						},
					},
				},
			},
		},
	}

	if agent.Spec.Model.GPUCount > 0 {
		deployment.Spec.Template.Spec.Containers[0].Resources.Limits["nvidia.com/gpu"] = *resource.NewQuantity(int64(agent.Spec.Model.GPUCount), resource.DecimalSI)
		runtimeClassName := "nvidia"
		deployment.Spec.Template.Spec.RuntimeClassName = &runtimeClassName
	}

	if agent.Spec.Model.Quantization != "" {
		deployment.Spec.Template.Spec.Containers[0].Args = append(
			deployment.Spec.Template.Spec.Containers[0].Args,
			"--quantization", agent.Spec.Model.Quantization,
		)
	}

	if err := controllerutil.SetControllerReference(agent, deployment, r.Scheme); err != nil {
		return fmt.Errorf("failed to set controller reference on deployment: %w", err)
	}

	existingDeployment := &appsv1.Deployment{}
	err = r.Get(ctx, client.ObjectKey{Name: deploymentName, Namespace: agent.Namespace}, existingDeployment)
	if err != nil {
		if errors.IsNotFound(err) {
			if err := r.Create(ctx, deployment); err != nil {
				return fmt.Errorf("failed to create deployment: %w", err)
			}
			logger.Info("Created model server deployment", "name", deploymentName, "namespace", agent.Namespace)
		} else {
			return fmt.Errorf("failed to get deployment: %w", err)
		}
	} else {
		deployment.ResourceVersion = existingDeployment.ResourceVersion
		if err := r.Update(ctx, deployment); err != nil {
			return fmt.Errorf("failed to update deployment: %w", err)
		}
		logger.V(1).Info("Updated model server deployment", "name", deploymentName, "namespace", agent.Namespace)
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: agent.Namespace,
			Labels: map[string]string{
				constants.LabelManagedBy:      constants.ManagedByAgentController,
				"app":                         "model-server",
				"agent":                       agent.Name,
				"app.kubernetes.io/name":      "model-server",
				"app.kubernetes.io/instance":  agent.Name,
				"app.kubernetes.io/part-of":   "archon",
				"app.kubernetes.io/component": "model",
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
				"app":   "model-server",
				"agent": agent.Name,
			},
		},
	}

	if err := controllerutil.SetControllerReference(agent, service, r.Scheme); err != nil {
		return fmt.Errorf("failed to set controller reference on service: %w", err)
	}

	existingService := &corev1.Service{}
	err = r.Get(ctx, client.ObjectKey{Name: serviceName, Namespace: agent.Namespace}, existingService)
	if err != nil {
		if errors.IsNotFound(err) {
			if err := r.Create(ctx, service); err != nil {
				return fmt.Errorf("failed to create service: %w", err)
			}
			logger.Info("Created model server service", "name", serviceName, "namespace", agent.Namespace)
		} else {
			return fmt.Errorf("failed to get service: %w", err)
		}
	} else {
		service.ResourceVersion = existingService.ResourceVersion
		service.Spec.ClusterIP = existingService.Spec.ClusterIP
		if err := r.Update(ctx, service); err != nil {
			return fmt.Errorf("failed to update service: %w", err)
		}
		logger.V(1).Info("Updated model server service", "name", serviceName, "namespace", agent.Namespace)
	}

	// Update model server status
	agent.Status.ModelServer.Deployed = true
	agent.Status.ModelServer.ServiceName = serviceName
	agent.Status.ModelServer.ServiceURL = fmt.Sprintf("http://%s.%s:%d", serviceName, agent.Namespace, port)
	if existingDeployment.Status.ReadyReplicas > 0 {
		agent.Status.ModelServer.ReadyReplicas = existingDeployment.Status.ReadyReplicas
	}

	return nil
}


func (r *AgentReconciler) reconcileOrchestrator(ctx context.Context, agent *platformv1alpha1.Agent) error {
	logger := log.FromContext(ctx)

	select {
	case <-ctx.Done():
		return fmt.Errorf("context cancelled during orchestrator reconciliation: %w", ctx.Err())
	default:
	}

	port := agent.Spec.Orchestration.Port
	if port == 0 {
		port = 8000
	}

	image := agent.Spec.Orchestration.Image
	if image == "" {
		image = "ghcr.io/bdchatham/archon-orchestrator:latest"
	}

	modelURL := fmt.Sprintf("http://%s-model.%s:%d", agent.Name, agent.Namespace, agent.Spec.Model.Port)
	if agent.Spec.Model.Port == 0 {
		modelURL = fmt.Sprintf("http://%s-model.%s:8000", agent.Name, agent.Namespace)
	}

	var queryURL string
	if agent.Spec.KnowledgeBase != nil {
		kbNamespace := agent.Spec.KnowledgeBase.Namespace
		if kbNamespace == "" {
			kbNamespace = agent.Namespace
		}

		kb := &platformv1alpha1.KnowledgeBase{}
		err := r.Get(ctx, client.ObjectKey{Name: agent.Spec.KnowledgeBase.Name, Namespace: kbNamespace}, kb)
		if err != nil {
			return fmt.Errorf("failed to get KnowledgeBase %s/%s: %w", kbNamespace, agent.Spec.KnowledgeBase.Name, err)
		}

		queryURL = fmt.Sprintf("http://query.%s:8080", kbNamespace)
	}

	deploymentName := agent.Name
	serviceName := agent.Name
	replicas := int32(1)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: agent.Namespace,
			Labels: map[string]string{
				constants.LabelManagedBy:      constants.ManagedByAgentController,
				"app":                         "orchestrator",
				"agent":                       agent.Name,
				"app.kubernetes.io/name":      "orchestrator",
				"app.kubernetes.io/instance":  agent.Name,
				"app.kubernetes.io/part-of":   "archon",
				"app.kubernetes.io/component": "orchestrator",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":                         "orchestrator",
					"agent":                       agent.Name,
					"app.kubernetes.io/name":      "orchestrator",
					"app.kubernetes.io/instance":  agent.Name,
					"app.kubernetes.io/part-of":   "archon",
					"app.kubernetes.io/component": "orchestrator",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":                         "orchestrator",
						"agent":                       agent.Name,
						"app.kubernetes.io/name":      "orchestrator",
						"app.kubernetes.io/instance":  agent.Name,
						"app.kubernetes.io/part-of":   "archon",
						"app.kubernetes.io/component": "orchestrator",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "orchestrator",
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
									Name:  "MODEL_URL",
									Value: modelURL,
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: mustParseQuantity("256Mi"),
									corev1.ResourceCPU:    mustParseQuantity("100m"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: mustParseQuantity("512Mi"),
									corev1.ResourceCPU:    mustParseQuantity("500m"),
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
										Path: "/ready",
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

	if queryURL != "" {
		deployment.Spec.Template.Spec.Containers[0].Env = append(
			deployment.Spec.Template.Spec.Containers[0].Env,
			corev1.EnvVar{
				Name:  "QUERY_URL",
				Value: queryURL,
			},
		)
	}

	if err := controllerutil.SetControllerReference(agent, deployment, r.Scheme); err != nil {
		return fmt.Errorf("failed to set controller reference on deployment: %w", err)
	}

	existingDeployment := &appsv1.Deployment{}
	deploymentErr := r.Get(ctx, client.ObjectKey{Name: deploymentName, Namespace: agent.Namespace}, existingDeployment)
	if deploymentErr != nil {
		if errors.IsNotFound(deploymentErr) {
			if err := r.Create(ctx, deployment); err != nil {
				return fmt.Errorf("failed to create deployment: %w", err)
			}
			logger.Info("Created orchestrator deployment", "name", deploymentName, "namespace", agent.Namespace)
		} else {
			return fmt.Errorf("failed to get deployment: %w", deploymentErr)
		}
	} else {
		deployment.ResourceVersion = existingDeployment.ResourceVersion
		if err := r.Update(ctx, deployment); err != nil {
			return fmt.Errorf("failed to update deployment: %w", err)
		}
		logger.V(1).Info("Updated orchestrator deployment", "name", deploymentName, "namespace", agent.Namespace)
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: agent.Namespace,
			Labels: map[string]string{
				constants.LabelManagedBy:      constants.ManagedByAgentController,
				"app":                         "orchestrator",
				"agent":                       agent.Name,
				"app.kubernetes.io/name":      "orchestrator",
				"app.kubernetes.io/instance":  agent.Name,
				"app.kubernetes.io/part-of":   "archon",
				"app.kubernetes.io/component": "orchestrator",
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
				"app":   "orchestrator",
				"agent": agent.Name,
			},
		},
	}

	if err := controllerutil.SetControllerReference(agent, service, r.Scheme); err != nil {
		return fmt.Errorf("failed to set controller reference on service: %w", err)
	}

	existingService := &corev1.Service{}
	serviceErr := r.Get(ctx, client.ObjectKey{Name: serviceName, Namespace: agent.Namespace}, existingService)
	if serviceErr != nil {
		if errors.IsNotFound(serviceErr) {
			if err := r.Create(ctx, service); err != nil {
				return fmt.Errorf("failed to create service: %w", err)
			}
			logger.Info("Created orchestrator service", "name", serviceName, "namespace", agent.Namespace)
		} else {
			return fmt.Errorf("failed to get service: %w", serviceErr)
		}
	} else {
		service.ResourceVersion = existingService.ResourceVersion
		service.Spec.ClusterIP = existingService.Spec.ClusterIP
		if err := r.Update(ctx, service); err != nil {
			return fmt.Errorf("failed to update service: %w", err)
		}
		logger.V(1).Info("Updated orchestrator service", "name", serviceName, "namespace", agent.Namespace)
	}

	// Update orchestrator status
	if agent.Status.Orchestrator == nil {
		agent.Status.Orchestrator = &platformv1alpha1.OrchestratorStatus{}
	}
	agent.Status.Orchestrator.Deployed = true
	agent.Status.Orchestrator.ServiceName = serviceName
	agent.Status.Orchestrator.ServiceURL = fmt.Sprintf("http://%s.%s:%d", serviceName, agent.Namespace, port)
	if existingDeployment.Status.ReadyReplicas > 0 {
		agent.Status.Orchestrator.ReadyReplicas = existingDeployment.Status.ReadyReplicas
	}

	return nil
}

func (r *AgentReconciler) cleanupOrchestrator(ctx context.Context, agent *platformv1alpha1.Agent) error {
	logger := log.FromContext(ctx)

	deploymentName := agent.Name
	serviceName := agent.Name

	deployment := &appsv1.Deployment{}
	deploymentErr := r.Get(ctx, client.ObjectKey{Name: deploymentName, Namespace: agent.Namespace}, deployment)
	if deploymentErr == nil {
		if err := r.Delete(ctx, deployment); err != nil {
			return fmt.Errorf("failed to delete deployment: %w", err)
		}
		logger.Info("Deleted orchestrator deployment", "name", deploymentName, "namespace", agent.Namespace)
	} else if !errors.IsNotFound(deploymentErr) {
		return fmt.Errorf("failed to get deployment for cleanup: %w", deploymentErr)
	}

	service := &corev1.Service{}
	serviceErr := r.Get(ctx, client.ObjectKey{Name: serviceName, Namespace: agent.Namespace}, service)
	if serviceErr == nil {
		if err := r.Delete(ctx, service); err != nil {
			return fmt.Errorf("failed to delete service: %w", err)
		}
		logger.Info("Deleted orchestrator service", "name", serviceName, "namespace", agent.Namespace)
	} else if !errors.IsNotFound(serviceErr) {
		return fmt.Errorf("failed to get service for cleanup: %w", serviceErr)
	}

	agent.Status.Orchestrator = nil

	return nil
}

func (r *AgentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return r.SetupWithManagerAndOptions(mgr, controller.Options{})
}

// SetupWithManagerAndOptions sets up the controller with the Manager and custom options.
// This allows configuring rate limiting and max concurrent reconciles.
func (r *AgentReconciler) SetupWithManagerAndOptions(mgr ctrl.Manager, opts controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&platformv1alpha1.Agent{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		WithOptions(opts).
		Complete(r)
}
