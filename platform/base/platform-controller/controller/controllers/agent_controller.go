package controllers

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	platformv1alpha1 "github.com/bdchatham/AphexPlatformInfrastructure/platform/platform-controller/controller/api/v1alpha1"
)

// AgentReconciler reconciles an Agent object
type AgentReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=aphex.io,resources=agents,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=aphex.io,resources=agents/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=aphex.io,resources=agents/finalizers,verbs=update
// +kubebuilder:rbac:groups=aphex.io,resources=knowledgebases,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete

func (r *AgentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	agent := &platformv1alpha1.Agent{}
	err := r.Get(ctx, req.NamespacedName, agent)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get Agent")
		return ctrl.Result{}, err
	}

	if agent.Status.Phase == "" {
		agent.Status.Phase = "Pending"
		now := metav1.Now()
		agent.Status.LastReconcileTime = &now
		if err := r.Status().Update(ctx, agent); err != nil {
			log.Error(err, "Failed to update Agent status to Pending")
			return ctrl.Result{}, err
		}
		log.Info("Set Agent phase to Pending", "name", agent.Name, "namespace", agent.Namespace)
	}

	if err := r.reconcileModelServer(ctx, agent); err != nil {
		log.Error(err, "Failed to reconcile model server")
		agent.Status.Phase = "Failed"
		agent.Status.Message = fmt.Sprintf("Model server reconciliation failed: %v", err)
		now := metav1.Now()
		agent.Status.LastReconcileTime = &now
		if updateErr := r.Status().Update(ctx, agent); updateErr != nil {
			log.Error(updateErr, "Failed to update status after model server failure")
		}
		return ctrl.Result{}, err
	}

	if agent.Spec.Orchestration != nil {
		if err := r.reconcileOrchestrator(ctx, agent); err != nil {
			log.Error(err, "Failed to reconcile orchestrator")
			agent.Status.Phase = "Failed"
			agent.Status.Message = fmt.Sprintf("Orchestrator reconciliation failed: %v", err)
			now := metav1.Now()
			agent.Status.LastReconcileTime = &now
			if updateErr := r.Status().Update(ctx, agent); updateErr != nil {
				log.Error(updateErr, "Failed to update status after orchestrator failure")
			}
			return ctrl.Result{}, err
		}
	} else {
		if err := r.cleanupOrchestrator(ctx, agent); err != nil {
			log.Error(err, "Failed to cleanup orchestrator")
			return ctrl.Result{}, err
		}
	}

	agent.Status.Phase = "Ready"
	agent.Status.Message = "Agent is ready"
	now := metav1.Now()
	agent.Status.LastReconcileTime = &now
	if err := r.Status().Update(ctx, agent); err != nil {
		log.Error(err, "Failed to update Agent status to Ready")
		return ctrl.Result{}, err
	}

	log.Info("Agent reconciled successfully", "name", agent.Name, "namespace", agent.Namespace)

	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (r *AgentReconciler) reconcileModelServer(ctx context.Context, agent *platformv1alpha1.Agent) error {
	log := log.FromContext(ctx)

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
								"--max-model-len", "8192",
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
	err := r.Get(ctx, client.ObjectKey{Name: deploymentName, Namespace: agent.Namespace}, existingDeployment)
	if err != nil {
		if errors.IsNotFound(err) {
			if err := r.Create(ctx, deployment); err != nil {
				return fmt.Errorf("failed to create deployment: %w", err)
			}
			log.Info("Created model server deployment", "name", deploymentName, "namespace", agent.Namespace)
		} else {
			return fmt.Errorf("failed to get deployment: %w", err)
		}
	} else {
		deployment.ResourceVersion = existingDeployment.ResourceVersion
		if err := r.Update(ctx, deployment); err != nil {
			return fmt.Errorf("failed to update deployment: %w", err)
		}
		log.Info("Updated model server deployment", "name", deploymentName, "namespace", agent.Namespace)
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: agent.Namespace,
			Labels: map[string]string{
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
			log.Info("Created model server service", "name", serviceName, "namespace", agent.Namespace)
		} else {
			return fmt.Errorf("failed to get service: %w", err)
		}
	} else {
		service.ResourceVersion = existingService.ResourceVersion
		service.Spec.ClusterIP = existingService.Spec.ClusterIP
		if err := r.Update(ctx, service); err != nil {
			return fmt.Errorf("failed to update service: %w", err)
		}
		log.Info("Updated model server service", "name", serviceName, "namespace", agent.Namespace)
	}

	agent.Status.ModelServer.Deployed = true
	agent.Status.ModelServer.ServiceName = serviceName
	agent.Status.ModelServer.ServiceURL = fmt.Sprintf("http://%s.%s:%d", serviceName, agent.Namespace, port)
	if existingDeployment.Status.ReadyReplicas > 0 {
		agent.Status.ModelServer.ReadyReplicas = existingDeployment.Status.ReadyReplicas
	}

	return nil
}

func (r *AgentReconciler) reconcileOrchestrator(ctx context.Context, agent *platformv1alpha1.Agent) error {
	log := log.FromContext(ctx)

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
			log.Info("Created orchestrator deployment", "name", deploymentName, "namespace", agent.Namespace)
		} else {
			return fmt.Errorf("failed to get deployment: %w", deploymentErr)
		}
	} else {
		deployment.ResourceVersion = existingDeployment.ResourceVersion
		if err := r.Update(ctx, deployment); err != nil {
			return fmt.Errorf("failed to update deployment: %w", err)
		}
		log.Info("Updated orchestrator deployment", "name", deploymentName, "namespace", agent.Namespace)
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: agent.Namespace,
			Labels: map[string]string{
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
			log.Info("Created orchestrator service", "name", serviceName, "namespace", agent.Namespace)
		} else {
			return fmt.Errorf("failed to get service: %w", serviceErr)
		}
	} else {
		service.ResourceVersion = existingService.ResourceVersion
		service.Spec.ClusterIP = existingService.Spec.ClusterIP
		if err := r.Update(ctx, service); err != nil {
			return fmt.Errorf("failed to update service: %w", err)
		}
		log.Info("Updated orchestrator service", "name", serviceName, "namespace", agent.Namespace)
	}

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
	log := log.FromContext(ctx)

	deploymentName := agent.Name
	serviceName := agent.Name

	deployment := &appsv1.Deployment{}
	deploymentErr := r.Get(ctx, client.ObjectKey{Name: deploymentName, Namespace: agent.Namespace}, deployment)
	if deploymentErr == nil {
		if err := r.Delete(ctx, deployment); err != nil {
			return fmt.Errorf("failed to delete deployment: %w", err)
		}
		log.Info("Deleted orchestrator deployment", "name", deploymentName, "namespace", agent.Namespace)
	} else if !errors.IsNotFound(deploymentErr) {
		return fmt.Errorf("failed to get deployment for cleanup: %w", deploymentErr)
	}

	service := &corev1.Service{}
	serviceErr := r.Get(ctx, client.ObjectKey{Name: serviceName, Namespace: agent.Namespace}, service)
	if serviceErr == nil {
		if err := r.Delete(ctx, service); err != nil {
			return fmt.Errorf("failed to delete service: %w", err)
		}
		log.Info("Deleted orchestrator service", "name", serviceName, "namespace", agent.Namespace)
	} else if !errors.IsNotFound(serviceErr) {
		return fmt.Errorf("failed to get service for cleanup: %w", serviceErr)
	}

	agent.Status.Orchestrator = nil

	return nil
}

func (r *AgentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&platformv1alpha1.Agent{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Complete(r)
}
