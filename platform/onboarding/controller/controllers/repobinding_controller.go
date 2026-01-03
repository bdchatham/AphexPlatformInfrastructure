package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	platformv1alpha1 "github.com/arbiter/jenkinsx-platform/onboarding-controller/api/v1alpha1"
)

const (
	repoBindingFinalizer = "platform.arbiter.io/finalizer"
)

// RepoBindingReconciler reconciles a RepoBinding object
type RepoBindingReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

// +kubebuilder:rbac:groups=platform.arbiter.io,resources=repobindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=platform.arbiter.io,resources=repobindings/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=platform.arbiter.io,resources=repobindings/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=resourcequotas,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=limitranges,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;update;patch

// Reconcile handles RepoBinding create/update/delete events
func (r *RepoBindingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("repobinding", req.NamespacedName)

	// Fetch the RepoBinding instance
	repoBinding := &platformv1alpha1.RepoBinding{}
	err := r.Get(ctx, req.NamespacedName, repoBinding)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("RepoBinding resource not found, ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get RepoBinding")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !repoBinding.ObjectMeta.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(repoBinding, repoBindingFinalizer) {
			// Perform cleanup if needed
			log.Info("Cleaning up RepoBinding resources")
			
			// Remove finalizer
			controllerutil.RemoveFinalizer(repoBinding, repoBindingFinalizer)
			if err := r.Update(ctx, repoBinding); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(repoBinding, repoBindingFinalizer) {
		controllerutil.AddFinalizer(repoBinding, repoBindingFinalizer)
		if err := r.Update(ctx, repoBinding); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Initialize status if needed
	if repoBinding.Status.Phase == "" {
		repoBinding.Status.Phase = "Pending"
		repoBinding.Status.Message = "Starting onboarding process"
		repoBinding.Status.LastReconcileTime = metav1.Now()
		if err := r.Status().Update(ctx, repoBinding); err != nil {
			log.Error(err, "Failed to update RepoBinding status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Skip if already in Ready state
	if repoBinding.Status.Phase == "Ready" {
		log.Info("RepoBinding already in Ready state, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// Update phase to Provisioning
	if repoBinding.Status.Phase == "Pending" {
		// Validate the RepoBinding spec
		if err := ValidateRepoBinding(repoBinding); err != nil {
			log.Error(err, "Validation failed")
			repoBinding.Status.Phase = "Failed"
			repoBinding.Status.Message = fmt.Sprintf("Validation failed: %s", err.Error())
			repoBinding.Status.LastReconcileTime = metav1.Now()
			if updateErr := r.Status().Update(ctx, repoBinding); updateErr != nil {
				log.Error(updateErr, "Failed to update RepoBinding status")
				return ctrl.Result{}, updateErr
			}
			return ctrl.Result{}, nil
		}

		repoBinding.Status.Phase = "Provisioning"
		repoBinding.Status.Message = "Provisioning tenant resources"
		repoBinding.Status.LastReconcileTime = metav1.Now()
		if err := r.Status().Update(ctx, repoBinding); err != nil {
			log.Error(err, "Failed to update RepoBinding status")
			return ctrl.Result{}, err
		}
	}

	// Perform reconciliation steps (will be implemented in subsequent subtasks)
	log.Info("Reconciling RepoBinding", 
		"repoOrg", repoBinding.Spec.RepoOrg,
		"repoName", repoBinding.Spec.RepoName,
		"tenantName", repoBinding.Spec.TenantName,
		"permissionProfile", repoBinding.Spec.PermissionProfile)

	// Provision tenant namespace
	if !repoBinding.Status.NamespaceCreated {
		log.Info("Provisioning tenant namespace")
		if err := r.provisionNamespace(ctx, repoBinding); err != nil {
			log.Error(err, "Failed to provision namespace")
			// Refetch before status update to avoid conflicts
			if refetchErr := r.Get(ctx, req.NamespacedName, repoBinding); refetchErr != nil {
				log.Error(refetchErr, "Failed to refetch RepoBinding")
				return ctrl.Result{}, refetchErr
			}
			repoBinding.Status.Phase = "Failed"
			repoBinding.Status.Message = fmt.Sprintf("Failed to provision namespace: %s", err.Error())
			repoBinding.Status.LastReconcileTime = metav1.Now()
			if updateErr := r.Status().Update(ctx, repoBinding); updateErr != nil {
				log.Error(updateErr, "Failed to update RepoBinding status")
				return ctrl.Result{}, updateErr
			}
			return ctrl.Result{}, err
		}
		// Refetch before status update to avoid conflicts
		if err := r.Get(ctx, req.NamespacedName, repoBinding); err != nil {
			log.Error(err, "Failed to refetch RepoBinding")
			return ctrl.Result{}, err
		}
		repoBinding.Status.NamespaceCreated = true
		if err := r.Status().Update(ctx, repoBinding); err != nil {
			log.Error(err, "Failed to update RepoBinding status")
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Provision service account
	if !repoBinding.Status.ServiceAccountCreated {
		log.Info("Provisioning service account")
		if err := r.provisionServiceAccount(ctx, repoBinding); err != nil {
			log.Error(err, "Failed to provision service account")
			// Refetch before status update to avoid conflicts
			if refetchErr := r.Get(ctx, req.NamespacedName, repoBinding); refetchErr != nil {
				log.Error(refetchErr, "Failed to refetch RepoBinding")
				return ctrl.Result{}, refetchErr
			}
			repoBinding.Status.Phase = "Failed"
			repoBinding.Status.Message = fmt.Sprintf("Failed to provision service account: %s", err.Error())
			repoBinding.Status.LastReconcileTime = metav1.Now()
			if updateErr := r.Status().Update(ctx, repoBinding); updateErr != nil {
				log.Error(updateErr, "Failed to update RepoBinding status")
				return ctrl.Result{}, updateErr
			}
			return ctrl.Result{}, err
		}
		// Refetch before status update to avoid conflicts
		if err := r.Get(ctx, req.NamespacedName, repoBinding); err != nil {
			log.Error(err, "Failed to refetch RepoBinding")
			return ctrl.Result{}, err
		}
		repoBinding.Status.ServiceAccountCreated = true
		if err := r.Status().Update(ctx, repoBinding); err != nil {
			log.Error(err, "Failed to update RepoBinding status")
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Provision RBAC
	if !repoBinding.Status.RBACCreated {
		log.Info("Provisioning RBAC")
		if err := r.provisionRBAC(ctx, repoBinding); err != nil {
			log.Error(err, "Failed to provision RBAC")
			// Refetch before status update to avoid conflicts
			if refetchErr := r.Get(ctx, req.NamespacedName, repoBinding); refetchErr != nil {
				log.Error(refetchErr, "Failed to refetch RepoBinding")
				return ctrl.Result{}, refetchErr
			}
			repoBinding.Status.Phase = "Failed"
			repoBinding.Status.Message = fmt.Sprintf("Failed to provision RBAC: %s", err.Error())
			repoBinding.Status.LastReconcileTime = metav1.Now()
			if updateErr := r.Status().Update(ctx, repoBinding); updateErr != nil {
				log.Error(updateErr, "Failed to update RepoBinding status")
				return ctrl.Result{}, updateErr
			}
			return ctrl.Result{}, err
		}
		// Refetch before status update to avoid conflicts
		if err := r.Get(ctx, req.NamespacedName, repoBinding); err != nil {
			log.Error(err, "Failed to refetch RepoBinding")
			return ctrl.Result{}, err
		}
		repoBinding.Status.RBACCreated = true
		if err := r.Status().Update(ctx, repoBinding); err != nil {
			log.Error(err, "Failed to update RepoBinding status")
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Provision resource limits (ResourceQuota and LimitRange)
	if !repoBinding.Status.QuotasCreated {
		log.Info("Provisioning resource limits")
		if err := r.provisionResourceLimits(ctx, repoBinding); err != nil {
			log.Error(err, "Failed to provision resource limits")
			// Refetch before status update to avoid conflicts
			if refetchErr := r.Get(ctx, req.NamespacedName, repoBinding); refetchErr != nil {
				log.Error(refetchErr, "Failed to refetch RepoBinding")
				return ctrl.Result{}, refetchErr
			}
			repoBinding.Status.Phase = "Failed"
			repoBinding.Status.Message = fmt.Sprintf("Failed to provision resource limits: %s", err.Error())
			repoBinding.Status.LastReconcileTime = metav1.Now()
			if updateErr := r.Status().Update(ctx, repoBinding); updateErr != nil {
				log.Error(updateErr, "Failed to update RepoBinding status")
				return ctrl.Result{}, updateErr
			}
			return ctrl.Result{}, err
		}
		// Refetch before status update to avoid conflicts
		if err := r.Get(ctx, req.NamespacedName, repoBinding); err != nil {
			log.Error(err, "Failed to refetch RepoBinding")
			return ctrl.Result{}, err
		}
		repoBinding.Status.QuotasCreated = true
		if err := r.Status().Update(ctx, repoBinding); err != nil {
			log.Error(err, "Failed to update RepoBinding status")
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Provision network policy
	if !repoBinding.Status.NetworkPolicyCreated {
		log.Info("Provisioning network policy")
		if err := r.provisionNetworkPolicy(ctx, repoBinding); err != nil {
			log.Error(err, "Failed to provision network policy")
			// Refetch before status update to avoid conflicts
			if refetchErr := r.Get(ctx, req.NamespacedName, repoBinding); refetchErr != nil {
				log.Error(refetchErr, "Failed to refetch RepoBinding")
				return ctrl.Result{}, refetchErr
			}
			repoBinding.Status.Phase = "Failed"
			repoBinding.Status.Message = fmt.Sprintf("Failed to provision network policy: %s", err.Error())
			repoBinding.Status.LastReconcileTime = metav1.Now()
			if updateErr := r.Status().Update(ctx, repoBinding); updateErr != nil {
				log.Error(updateErr, "Failed to update RepoBinding status")
				return ctrl.Result{}, updateErr
			}
			return ctrl.Result{}, err
		}
		// Refetch before status update to avoid conflicts
		if err := r.Get(ctx, req.NamespacedName, repoBinding); err != nil {
			log.Error(err, "Failed to refetch RepoBinding")
			return ctrl.Result{}, err
		}
		repoBinding.Status.NetworkPolicyCreated = true
		if err := r.Status().Update(ctx, repoBinding); err != nil {
			log.Error(err, "Failed to update RepoBinding status")
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Provision Terraform backend secret
	if !repoBinding.Status.TerraformSecretCreated {
		log.Info("Provisioning Terraform backend secret")
		if err := r.provisionTerraformBackendSecret(ctx, repoBinding); err != nil {
			log.Error(err, "Failed to provision Terraform backend secret")
			// Refetch before status update to avoid conflicts
			if refetchErr := r.Get(ctx, req.NamespacedName, repoBinding); refetchErr != nil {
				log.Error(refetchErr, "Failed to refetch RepoBinding")
				return ctrl.Result{}, refetchErr
			}
			repoBinding.Status.Phase = "Failed"
			repoBinding.Status.Message = fmt.Sprintf("Failed to provision Terraform backend secret: %s", err.Error())
			repoBinding.Status.LastReconcileTime = metav1.Now()
			if updateErr := r.Status().Update(ctx, repoBinding); updateErr != nil {
				log.Error(updateErr, "Failed to update RepoBinding status")
				return ctrl.Result{}, updateErr
			}
			return ctrl.Result{}, err
		}
		// Refetch before status update to avoid conflicts
		if err := r.Get(ctx, req.NamespacedName, repoBinding); err != nil {
			log.Error(err, "Failed to refetch RepoBinding")
			return ctrl.Result{}, err
		}
		repoBinding.Status.TerraformSecretCreated = true
		if err := r.Status().Update(ctx, repoBinding); err != nil {
			log.Error(err, "Failed to update RepoBinding status")
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Provision webhook secret
	if !repoBinding.Status.WebhookSecretCreated {
		log.Info("Provisioning webhook secret")
		if err := r.provisionWebhookSecret(ctx, repoBinding); err != nil {
			log.Error(err, "Failed to provision webhook secret")
			// Refetch before status update to avoid conflicts
			if refetchErr := r.Get(ctx, req.NamespacedName, repoBinding); refetchErr != nil {
				log.Error(refetchErr, "Failed to refetch RepoBinding")
				return ctrl.Result{}, refetchErr
			}
			repoBinding.Status.Phase = "Failed"
			repoBinding.Status.Message = fmt.Sprintf("Failed to provision webhook secret: %s", err.Error())
			repoBinding.Status.LastReconcileTime = metav1.Now()
			if updateErr := r.Status().Update(ctx, repoBinding); updateErr != nil {
				log.Error(updateErr, "Failed to update RepoBinding status")
				return ctrl.Result{}, updateErr
			}
			return ctrl.Result{}, err
		}
		// Refetch before status update to avoid conflicts
		if err := r.Get(ctx, req.NamespacedName, repoBinding); err != nil {
			log.Error(err, "Failed to refetch RepoBinding")
			return ctrl.Result{}, err
		}
		repoBinding.Status.WebhookSecretCreated = true
		if err := r.Status().Update(ctx, repoBinding); err != nil {
			log.Error(err, "Failed to update RepoBinding status, will retry")
			// Don't return error on conflict, just requeue
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Provision EventListener
	if !repoBinding.Status.EventListenerCreated {
		log.Info("Provisioning EventListener")
		if err := r.provisionEventListener(ctx, repoBinding); err != nil {
			log.Error(err, "Failed to provision EventListener")
			// Refetch before status update to avoid conflicts
			if refetchErr := r.Get(ctx, req.NamespacedName, repoBinding); refetchErr != nil {
				log.Error(refetchErr, "Failed to refetch RepoBinding")
				return ctrl.Result{}, refetchErr
			}
			repoBinding.Status.Phase = "Failed"
			repoBinding.Status.Message = fmt.Sprintf("Failed to provision EventListener: %s", err.Error())
			repoBinding.Status.LastReconcileTime = metav1.Now()
			if updateErr := r.Status().Update(ctx, repoBinding); updateErr != nil {
				log.Error(updateErr, "Failed to update RepoBinding status")
				return ctrl.Result{}, updateErr
			}
			return ctrl.Result{}, err
		}
		// Refetch before status update to avoid conflicts
		if err := r.Get(ctx, req.NamespacedName, repoBinding); err != nil {
			log.Error(err, "Failed to refetch RepoBinding")
			return ctrl.Result{}, err
		}
		repoBinding.Status.EventListenerCreated = true
		if err := r.Status().Update(ctx, repoBinding); err != nil {
			log.Error(err, "Failed to update RepoBinding status")
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Provision Ingress
	if !repoBinding.Status.IngressCreated {
		log.Info("Provisioning Ingress")
		if err := r.provisionIngress(ctx, repoBinding); err != nil {
			log.Error(err, "Failed to provision Ingress")
			// Refetch before status update to avoid conflicts
			if refetchErr := r.Get(ctx, req.NamespacedName, repoBinding); refetchErr != nil {
				log.Error(refetchErr, "Failed to refetch RepoBinding")
				return ctrl.Result{}, refetchErr
			}
			repoBinding.Status.Phase = "Failed"
			repoBinding.Status.Message = fmt.Sprintf("Failed to provision Ingress: %s", err.Error())
			repoBinding.Status.LastReconcileTime = metav1.Now()
			if updateErr := r.Status().Update(ctx, repoBinding); updateErr != nil {
				log.Error(updateErr, "Failed to update RepoBinding status")
				return ctrl.Result{}, updateErr
			}
			return ctrl.Result{}, err
		}
		// Refetch before status update to avoid conflicts
		if err := r.Get(ctx, req.NamespacedName, repoBinding); err != nil {
			log.Error(err, "Failed to refetch RepoBinding")
			return ctrl.Result{}, err
		}
		repoBinding.Status.IngressCreated = true
		if err := r.Status().Update(ctx, repoBinding); err != nil {
			log.Error(err, "Failed to update RepoBinding status")
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Update allowlist (keeping this for backward compatibility, but may be removed)
	if !repoBinding.Status.AllowlistUpdated {
		log.Info("Updating allowlist")
		// Skip allowlist update if ConfigMap doesn't exist (new architecture doesn't need it)
		configMap := &corev1.ConfigMap{}
		err := r.Get(ctx, client.ObjectKey{Name: "repo-allowlist", Namespace: "pipeline-system"}, configMap)
		if err != nil {
			if errors.IsNotFound(err) {
				log.Info("Allowlist ConfigMap not found, skipping (new architecture)")
				// Refetch before status update to avoid conflicts
				if refetchErr := r.Get(ctx, req.NamespacedName, repoBinding); refetchErr != nil {
					log.Error(refetchErr, "Failed to refetch RepoBinding")
					return ctrl.Result{}, refetchErr
				}
				repoBinding.Status.AllowlistUpdated = true
				if err := r.Status().Update(ctx, repoBinding); err != nil {
					log.Error(err, "Failed to update RepoBinding status")
					return ctrl.Result{Requeue: true}, nil
				}
				return ctrl.Result{Requeue: true}, nil
			}
			log.Error(err, "Failed to get allowlist ConfigMap")
			// Refetch before status update to avoid conflicts
			if refetchErr := r.Get(ctx, req.NamespacedName, repoBinding); refetchErr != nil {
				log.Error(refetchErr, "Failed to refetch RepoBinding")
				return ctrl.Result{}, refetchErr
			}
			repoBinding.Status.Phase = "Failed"
			repoBinding.Status.Message = fmt.Sprintf("Failed to get allowlist: %s", err.Error())
			repoBinding.Status.LastReconcileTime = metav1.Now()
			if updateErr := r.Status().Update(ctx, repoBinding); updateErr != nil {
				log.Error(updateErr, "Failed to update RepoBinding status")
				return ctrl.Result{}, updateErr
			}
			return ctrl.Result{}, err
		}
		
		// ConfigMap exists, update it
		if err := r.provisionAllowlistEntry(ctx, repoBinding); err != nil {
			log.Error(err, "Failed to update allowlist")
			// Refetch before status update to avoid conflicts
			if refetchErr := r.Get(ctx, req.NamespacedName, repoBinding); refetchErr != nil {
				log.Error(refetchErr, "Failed to refetch RepoBinding")
				return ctrl.Result{}, refetchErr
			}
			repoBinding.Status.Phase = "Failed"
			repoBinding.Status.Message = fmt.Sprintf("Failed to update allowlist: %s", err.Error())
			repoBinding.Status.LastReconcileTime = metav1.Now()
			if updateErr := r.Status().Update(ctx, repoBinding); updateErr != nil {
				log.Error(updateErr, "Failed to update RepoBinding status")
				return ctrl.Result{}, updateErr
			}
			return ctrl.Result{}, err
		}
		// Refetch before status update to avoid conflicts
		if err := r.Get(ctx, req.NamespacedName, repoBinding); err != nil {
			log.Error(err, "Failed to refetch RepoBinding")
			return ctrl.Result{}, err
		}
		repoBinding.Status.AllowlistUpdated = true
		if err := r.Status().Update(ctx, repoBinding); err != nil {
			log.Error(err, "Failed to update RepoBinding status")
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Update RepoBinding status with webhook information
	log.Info("Updating RepoBinding status with webhook information")
	if err := r.updateRepoBindingStatusWithWebhookInfo(ctx, repoBinding); err != nil {
		log.Error(err, "Failed to update RepoBinding status with webhook info")
		// Refetch before status update to avoid conflicts
		if refetchErr := r.Get(ctx, req.NamespacedName, repoBinding); refetchErr != nil {
			log.Error(refetchErr, "Failed to refetch RepoBinding")
			return ctrl.Result{}, refetchErr
		}
		repoBinding.Status.Phase = "Failed"
		repoBinding.Status.Message = fmt.Sprintf("Failed to update webhook info: %s", err.Error())
		repoBinding.Status.LastReconcileTime = metav1.Now()
		if updateErr := r.Status().Update(ctx, repoBinding); updateErr != nil {
			log.Error(updateErr, "Failed to update RepoBinding status")
			return ctrl.Result{}, updateErr
		}
		return ctrl.Result{}, err
	}

	// Refetch before final status update to avoid conflicts
	if err := r.Get(ctx, req.NamespacedName, repoBinding); err != nil {
		log.Error(err, "Failed to refetch RepoBinding")
		return ctrl.Result{}, err
	}

	// Update status to Ready
	repoBinding.Status.Phase = "Ready"
	repoBinding.Status.Message = "Tenant resources provisioned successfully"
	repoBinding.Status.LastReconcileTime = metav1.Now()
	if err := r.Status().Update(ctx, repoBinding); err != nil {
		log.Error(err, "Failed to update RepoBinding status")
		return ctrl.Result{Requeue: true}, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager
func (r *RepoBindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&platformv1alpha1.RepoBinding{}).
		Owns(&corev1.Namespace{}).
		Owns(&corev1.ServiceAccount{}).
		Complete(r)
}
