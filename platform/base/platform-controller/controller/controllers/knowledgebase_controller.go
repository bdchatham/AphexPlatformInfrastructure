package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	platformv1alpha1 "github.com/bdchatham/AphexPlatformInfrastructure/platform/platform-controller/controller/api/v1alpha1"
)

// KnowledgeBaseReconciler reconciles a KnowledgeBase object
type KnowledgeBaseReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=aphex.io,resources=knowledgebases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=aphex.io,resources=knowledgebases/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=aphex.io,resources=knowledgebases/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete

// Reconcile manages KnowledgeBase resources
func (r *KnowledgeBaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	kb := &platformv1alpha1.KnowledgeBase{}
	err := r.Get(ctx, req.NamespacedName, kb)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get KnowledgeBase")
		return ctrl.Result{}, err
	}

	if kb.Status.Phase == "" {
		kb.Status.Phase = "Pending"
		now := metav1.Now()
		kb.Status.LastReconcileTime = &now
		if err := r.Status().Update(ctx, kb); err != nil {
			log.Error(err, "Failed to update KnowledgeBase status to Pending")
			return ctrl.Result{}, err
		}
		log.Info("Set KnowledgeBase phase to Pending", "name", kb.Name, "namespace", kb.Namespace)
	}

	if err := r.validateSpec(kb); err != nil {
		kb.Status.Phase = "Failed"
		kb.Status.Message = fmt.Sprintf("Validation failed: %v", err)
		now := metav1.Now()
		kb.Status.LastReconcileTime = &now
		if updateErr := r.Status().Update(ctx, kb); updateErr != nil {
			log.Error(updateErr, "Failed to update KnowledgeBase status to Failed")
			return ctrl.Result{}, updateErr
		}
		log.Info("KnowledgeBase validation failed", "name", kb.Name, "namespace", kb.Namespace, "error", err)
		return ctrl.Result{}, nil
	}

	if kb.Spec.MCPServer.Enabled {
		if err := r.reconcileMCPServer(ctx, kb); err != nil {
			log.Error(err, "Failed to reconcile MCP server")
			kb.Status.Phase = "Failed"
			kb.Status.Message = fmt.Sprintf("MCP server reconciliation failed: %v", err)
			now := metav1.Now()
			kb.Status.LastReconcileTime = &now
			if updateErr := r.Status().Update(ctx, kb); updateErr != nil {
				log.Error(updateErr, "Failed to update status after MCP server failure")
			}
			return ctrl.Result{}, err
		}
	} else {
		if err := r.cleanupMCPServer(ctx, kb); err != nil {
			log.Error(err, "Failed to cleanup MCP server")
			return ctrl.Result{}, err
		}
	}

	kb.Status.Phase = "Ready"
	kb.Status.Message = fmt.Sprintf("Tracking %d repositories", len(kb.Spec.Repositories))
	now := metav1.Now()
	kb.Status.LastReconcileTime = &now
	if err := r.Status().Update(ctx, kb); err != nil {
		log.Error(err, "Failed to update KnowledgeBase status to Ready")
		return ctrl.Result{}, err
	}

	log.Info("KnowledgeBase reconciled successfully", "name", kb.Name, "namespace", kb.Namespace, "repositories", len(kb.Spec.Repositories))

	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (r *KnowledgeBaseReconciler) reconcileMCPServer(ctx context.Context, kb *platformv1alpha1.KnowledgeBase) error {
	log := log.FromContext(ctx)

	port := kb.Spec.MCPServer.Port
	if port == 0 {
		port = 8090
	}

	image := kb.Spec.MCPServer.Image
	if image == "" {
		image = "ghcr.io/bdchatham/archon-mcp-server:latest"
	}

	queryServiceURL := kb.Spec.MCPServer.QueryServiceURL
	if queryServiceURL == "" {
		queryServiceURL = fmt.Sprintf("http://query.%s:8080", kb.Namespace)
	}

	deploymentName := fmt.Sprintf("mcp-server-%s", kb.Name)
	serviceName := fmt.Sprintf("mcp-server-%s", kb.Name)
	replicas := int32(1)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: kb.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":              "mcp-server",
					"knowledgebase":    kb.Name,
					"app.kubernetes.io/name":      "mcp-server",
					"app.kubernetes.io/instance":  kb.Name,
					"app.kubernetes.io/part-of":   "archon",
					"app.kubernetes.io/component": "mcp",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":              "mcp-server",
						"knowledgebase":    kb.Name,
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
			log.Info("Created MCP server deployment", "name", deploymentName, "namespace", kb.Namespace)
		} else {
			return fmt.Errorf("failed to get deployment: %w", err)
		}
	} else {
		deployment.ResourceVersion = existingDeployment.ResourceVersion
		if err := r.Update(ctx, deployment); err != nil {
			return fmt.Errorf("failed to update deployment: %w", err)
		}
		log.Info("Updated MCP server deployment", "name", deploymentName, "namespace", kb.Namespace)
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: kb.Namespace,
			Labels: map[string]string{
				"app":              "mcp-server",
				"knowledgebase":    kb.Name,
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
			log.Info("Created MCP server service", "name", serviceName, "namespace", kb.Namespace)
		} else {
			return fmt.Errorf("failed to get service: %w", err)
		}
	} else {
		service.ResourceVersion = existingService.ResourceVersion
		service.Spec.ClusterIP = existingService.Spec.ClusterIP
		if err := r.Update(ctx, service); err != nil {
			return fmt.Errorf("failed to update service: %w", err)
		}
		log.Info("Updated MCP server service", "name", serviceName, "namespace", kb.Namespace)
	}

	kb.Status.MCPServer.Deployed = true
	kb.Status.MCPServer.ServiceName = serviceName
	kb.Status.MCPServer.ServiceURL = fmt.Sprintf("http://%s.%s:%d", serviceName, kb.Namespace, port)
	kb.Status.MCPServer.ReadyReplicas = existingDeployment.Status.ReadyReplicas

	return nil
}

func (r *KnowledgeBaseReconciler) cleanupMCPServer(ctx context.Context, kb *platformv1alpha1.KnowledgeBase) error {
	log := log.FromContext(ctx)

	deploymentName := fmt.Sprintf("mcp-server-%s", kb.Name)
	serviceName := fmt.Sprintf("mcp-server-%s", kb.Name)

	deployment := &appsv1.Deployment{}
	err := r.Get(ctx, client.ObjectKey{Name: deploymentName, Namespace: kb.Namespace}, deployment)
	if err == nil {
		if err := r.Delete(ctx, deployment); err != nil {
			return fmt.Errorf("failed to delete deployment: %w", err)
		}
		log.Info("Deleted MCP server deployment", "name", deploymentName, "namespace", kb.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get deployment for cleanup: %w", err)
	}

	service := &corev1.Service{}
	err = r.Get(ctx, client.ObjectKey{Name: serviceName, Namespace: kb.Namespace}, service)
	if err == nil {
		if err := r.Delete(ctx, service); err != nil {
			return fmt.Errorf("failed to delete service: %w", err)
		}
		log.Info("Deleted MCP server service", "name", serviceName, "namespace", kb.Namespace)
	} else if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get service for cleanup: %w", err)
	}

	kb.Status.MCPServer.Deployed = false
	kb.Status.MCPServer.ServiceName = ""
	kb.Status.MCPServer.ServiceURL = ""
	kb.Status.MCPServer.ReadyReplicas = 0

	return nil
}

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

func (r *KnowledgeBaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&platformv1alpha1.KnowledgeBase{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Complete(r)
}
