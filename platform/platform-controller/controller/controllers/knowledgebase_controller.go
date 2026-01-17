package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	platformv1alpha1 "github.com/bdchatham/AphexPlatformInfrastructure/platform/platform-controller/controller/api/v1alpha1"
)

// KnowledgeBaseReconciler reconciles a KnowledgeBase object
type KnowledgeBaseReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=arbiter.io,resources=knowledgebases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=arbiter.io,resources=knowledgebases/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=arbiter.io,resources=knowledgebases/finalizers,verbs=update

// Reconcile manages KnowledgeBase resources
func (r *KnowledgeBaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the KnowledgeBase instance
	kb := &platformv1alpha1.KnowledgeBase{}
	err := r.Get(ctx, req.NamespacedName, kb)
	if err != nil {
		if errors.IsNotFound(err) {
			// Resource not found, nothing to do
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get KnowledgeBase")
		return ctrl.Result{}, err
	}

	// Set phase to Pending if not set
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

	// Validate specification
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
		// Don't requeue on validation failure - user needs to fix the spec
		return ctrl.Result{}, nil
	}

	// Update status to Ready
	kb.Status.Phase = "Ready"
	kb.Status.Message = fmt.Sprintf("Tracking %d repositories", len(kb.Spec.Repositories))
	now := metav1.Now()
	kb.Status.LastReconcileTime = &now
	if err := r.Status().Update(ctx, kb); err != nil {
		log.Error(err, "Failed to update KnowledgeBase status to Ready")
		return ctrl.Result{}, err
	}

	log.Info("KnowledgeBase reconciled successfully", "name", kb.Name, "namespace", kb.Namespace, "repositories", len(kb.Spec.Repositories))

	// Requeue after 5 minutes
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// validateSpec validates the KnowledgeBase specification
func (r *KnowledgeBaseReconciler) validateSpec(kb *platformv1alpha1.KnowledgeBase) error {
	// Validate repositories array is not empty
	if len(kb.Spec.Repositories) == 0 {
		return fmt.Errorf("repositories array cannot be empty")
	}

	// Validate each repository
	for i, repo := range kb.Spec.Repositories {
		// Validate URL format
		if !strings.HasPrefix(repo.URL, "https://github.com/") {
			return fmt.Errorf("repository[%d]: URL must start with https://github.com/", i)
		}

		// Validate branch name (if provided)
		if repo.Branch != "" && !isValidBranchName(repo.Branch) {
			return fmt.Errorf("repository[%d]: invalid branch name '%s'", i, repo.Branch)
		}

		// Validate paths (if provided)
		for j, path := range repo.Paths {
			if !strings.HasPrefix(path, ".kiro/docs") {
				return fmt.Errorf("repository[%d].paths[%d]: path must start with .kiro/docs", i, j)
			}
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager
func (r *KnowledgeBaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&platformv1alpha1.KnowledgeBase{}).
		Complete(r)
}
