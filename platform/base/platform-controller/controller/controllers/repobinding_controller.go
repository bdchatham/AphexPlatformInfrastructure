package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	platformv1alpha1 "github.com/bdchatham/AphexPlatformInfrastructure/platform/platform-controller/controller/api/v1alpha1"
	triggersv1beta1 "github.com/tektoncd/triggers/pkg/apis/triggers/v1beta1"
)

const (
	repoBindingFinalizer = "platform.aphex/finalizer"
)

// RepoBindingReconciler reconciles a RepoBinding object
type RepoBindingReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	Log             logr.Logger
	TemplateCatalog *TemplateCatalog
}

// +kubebuilder:rbac:groups=aphex.io,resources=repobindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=aphex.io,resources=repobindings/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=aphex.io,resources=repobindings/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=resourcequotas,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=limitranges,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=triggers.tekton.dev,resources=triggerbindings,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=triggers.tekton.dev,resources=triggertemplates,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=triggers.tekton.dev,resources=eventlisteners,verbs=get;list;watch;create;update;patch

// Reconcile handles RepoBinding create/update/delete events
func (r *RepoBindingReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	logger := r.Log.WithValues("repobinding", request.NamespacedName)

	repoBinding, err := r.fetchRepoBinding(ctx, request)
	if err != nil {
		return r.handleFetchError(logger, err)
	}

	if !repoBinding.ObjectMeta.DeletionTimestamp.IsZero() {
		if err := r.handleDeletion(ctx, logger, repoBinding); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if err := r.ensureFinalizer(ctx, repoBinding); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.ensureOwnerReference(ctx, repoBinding); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.initializeStatus(ctx, logger, repoBinding); err != nil {
		return ctrl.Result{}, err
	}

	if r.isAlreadyReady(logger, repoBinding) {
		return ctrl.Result{}, nil
	}

	if err := r.validateAndUpdatePhase(ctx, logger, request, repoBinding); err != nil {
		return ctrl.Result{}, err
	}

	return r.executeProvisioningSteps(ctx, logger, request, repoBinding)
}

func (r *RepoBindingReconciler) executeProvisioningSteps(ctx context.Context, logger logr.Logger, request ctrl.Request, repoBinding *platformv1alpha1.RepoBinding) (ctrl.Result, error) {
	logger.Info("Reconciling RepoBinding",
		"repoOrg", repoBinding.Spec.RepoOrg,
		"repoName", repoBinding.Spec.RepoName,
		"pipelineName", repoBinding.Spec.PipelineName,
		"templateRef", repoBinding.Spec.TemplateRef)

	steps := []provisioningStep{
		{name: "namespace", statusField: &repoBinding.Status.NamespaceCreated, provisionFunc: r.provisionNamespace},
		{name: "pipeline", statusField: &repoBinding.Status.PipelineCreated, provisionFunc: r.provisionPipeline},
		{name: "service account", statusField: &repoBinding.Status.ServiceAccountCreated, provisionFunc: r.provisionServiceAccount},
		{name: "RBAC", statusField: &repoBinding.Status.RBACCreated, provisionFunc: r.provisionRBAC},
		{name: "resource limits", statusField: &repoBinding.Status.QuotasCreated, provisionFunc: r.provisionResourceLimits},
		{name: "network policy", statusField: &repoBinding.Status.NetworkPolicyCreated, provisionFunc: r.provisionNetworkPolicy},
		{name: "Terraform backend secret", statusField: &repoBinding.Status.TerraformSecretCreated, provisionFunc: r.provisionTerraformBackendSecret},
		{name: "EventListener namespace", statusField: &repoBinding.Status.TriggerBindingCreated, provisionFunc: r.updateEventListenerNamespaces},
		{name: "TriggerTemplate", statusField: &repoBinding.Status.TriggerTemplateCreated, provisionFunc: r.provisionTriggerTemplate},
		{name: "Trigger", statusField: &repoBinding.Status.TriggerCreated, provisionFunc: r.provisionTrigger},
	}

	for _, step := range steps {
		if !*step.statusField {
			if err := r.executeProvisioningStep(ctx, logger, request, repoBinding, step); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}
	}

	if err := r.handleAllowlistUpdate(ctx, logger, request, repoBinding); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.finalizeProvisioning(ctx, logger, request, repoBinding); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

type provisioningStep struct {
	name          string
	statusField   *bool
	provisionFunc func(context.Context, *platformv1alpha1.RepoBinding) error
}

func (r *RepoBindingReconciler) executeProvisioningStep(ctx context.Context, logger logr.Logger, request ctrl.Request, repoBinding *platformv1alpha1.RepoBinding, step provisioningStep) error {
	logger.Info("Provisioning " + step.name)

	if err := step.provisionFunc(ctx, repoBinding); err != nil {
		logger.Error(err, "Failed to provision "+step.name)
		if refetchErr := r.Get(ctx, request.NamespacedName, repoBinding); refetchErr != nil {
			logger.Error(refetchErr, "Failed to refetch RepoBinding")
			return refetchErr
		}
		return r.updateStatusToFailed(ctx, request, repoBinding, fmt.Sprintf("Failed to provision %s: %s", step.name, err.Error()))
	}

	if err := r.Get(ctx, request.NamespacedName, repoBinding); err != nil {
		logger.Error(err, "Failed to refetch RepoBinding")
		return err
	}

	*step.statusField = true
	if err := r.Status().Update(ctx, repoBinding); err != nil {
		logger.Error(err, "Failed to update RepoBinding status")
		return err
	}

	return nil
}

func (r *RepoBindingReconciler) handleAllowlistUpdate(ctx context.Context, logger logr.Logger, request ctrl.Request, repoBinding *platformv1alpha1.RepoBinding) error {
	if repoBinding.Status.AllowlistUpdated {
		return nil
	}

	logger.Info("Updating allowlist")
	configMap := &corev1.ConfigMap{}
	err := r.Get(ctx, client.ObjectKey{Name: "repo-allowlist", Namespace: "pipeline-system"}, configMap)

	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Allowlist ConfigMap not found, skipping (new architecture)")
			return r.markAllowlistUpdated(ctx, logger, request, repoBinding)
		}
		logger.Error(err, "Failed to get allowlist ConfigMap")
		if refetchErr := r.Get(ctx, request.NamespacedName, repoBinding); refetchErr != nil {
			logger.Error(refetchErr, "Failed to refetch RepoBinding")
			return refetchErr
		}
		return r.updateStatusToFailed(ctx, request, repoBinding, fmt.Sprintf("Failed to get allowlist: %s", err.Error()))
	}

	if err := r.provisionAllowlistEntry(ctx, repoBinding); err != nil {
		logger.Error(err, "Failed to update allowlist")
		if refetchErr := r.Get(ctx, request.NamespacedName, repoBinding); refetchErr != nil {
			logger.Error(refetchErr, "Failed to refetch RepoBinding")
			return refetchErr
		}
		return r.updateStatusToFailed(ctx, request, repoBinding, fmt.Sprintf("Failed to update allowlist: %s", err.Error()))
	}

	return r.markAllowlistUpdated(ctx, logger, request, repoBinding)
}

func (r *RepoBindingReconciler) markAllowlistUpdated(ctx context.Context, logger logr.Logger, request ctrl.Request, repoBinding *platformv1alpha1.RepoBinding) error {
	if err := r.Get(ctx, request.NamespacedName, repoBinding); err != nil {
		logger.Error(err, "Failed to refetch RepoBinding")
		return err
	}
	repoBinding.Status.AllowlistUpdated = true
	if err := r.Status().Update(ctx, repoBinding); err != nil {
		logger.Error(err, "Failed to update RepoBinding status")
		return err
	}
	return nil
}

func (r *RepoBindingReconciler) finalizeProvisioning(ctx context.Context, logger logr.Logger, request ctrl.Request, repoBinding *platformv1alpha1.RepoBinding) error {
	logger.Info("Updating RepoBinding status with webhook information")
	if err := r.updateRepoBindingStatusWithWebhookInfo(ctx, repoBinding); err != nil {
		logger.Error(err, "Failed to update RepoBinding status with webhook info")
		if refetchErr := r.Get(ctx, request.NamespacedName, repoBinding); refetchErr != nil {
			logger.Error(refetchErr, "Failed to refetch RepoBinding")
			return refetchErr
		}
		return r.updateStatusToFailed(ctx, request, repoBinding, fmt.Sprintf("Failed to update webhook info: %s", err.Error()))
	}

	if err := r.Get(ctx, request.NamespacedName, repoBinding); err != nil {
		logger.Error(err, "Failed to refetch RepoBinding")
		return err
	}

	repoBinding.Status.Phase = "Ready"
	repoBinding.Status.Message = "Tenant resources provisioned successfully"
	repoBinding.Status.LastReconcileTime = metav1.Now()
	if err := r.Status().Update(ctx, repoBinding); err != nil {
		logger.Error(err, "Failed to update RepoBinding status")
		return err
	}

	return nil
}

func (r *RepoBindingReconciler) fetchRepoBinding(ctx context.Context, request ctrl.Request) (*platformv1alpha1.RepoBinding, error) {
	repoBinding := &platformv1alpha1.RepoBinding{}
	err := r.Get(ctx, request.NamespacedName, repoBinding)
	return repoBinding, err
}

func (r *RepoBindingReconciler) handleFetchError(logger logr.Logger, err error) (ctrl.Result, error) {
	if errors.IsNotFound(err) {
		logger.Info("RepoBinding resource not found, ignoring since object must be deleted")
		return ctrl.Result{}, nil
	}
	logger.Error(err, "Failed to get RepoBinding")
	return ctrl.Result{}, err
}

func (r *RepoBindingReconciler) shouldSkipReconciliation(repoBinding *platformv1alpha1.RepoBinding) bool {
	return !repoBinding.ObjectMeta.DeletionTimestamp.IsZero()
}

func (r *RepoBindingReconciler) handleDeletion(ctx context.Context, logger logr.Logger, repoBinding *platformv1alpha1.RepoBinding) error {
	if !repoBinding.ObjectMeta.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(repoBinding, repoBindingFinalizer) {
			logger.Info("Cleaning up RepoBinding resources")

			orgNamespace := fmt.Sprintf("org-%s", repoBinding.Spec.AphexOrg)

			trigger := &triggersv1beta1.Trigger{}
			triggerName := fmt.Sprintf("%s-trigger", repoBinding.Spec.PipelineName)
			if err := r.Get(ctx, client.ObjectKey{Name: triggerName, Namespace: orgNamespace}, trigger); err == nil {
				logger.Info("Deleting Trigger", "name", triggerName)
				if err := r.Delete(ctx, trigger); err != nil && !errors.IsNotFound(err) {
					return fmt.Errorf("failed to delete Trigger: %w", err)
				}
			}

			triggerTemplate := &triggersv1beta1.TriggerTemplate{}
			templateName := fmt.Sprintf("%s-trigger-template", repoBinding.Spec.PipelineName)
			if err := r.Get(ctx, client.ObjectKey{Name: templateName, Namespace: orgNamespace}, triggerTemplate); err == nil {
				logger.Info("Deleting TriggerTemplate", "name", templateName)
				if err := r.Delete(ctx, triggerTemplate); err != nil && !errors.IsNotFound(err) {
					return fmt.Errorf("failed to delete TriggerTemplate: %w", err)
				}
			}

			appProject := &unstructured.Unstructured{}
			appProject.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "argoproj.io",
				Version: "v1alpha1",
				Kind:    "AppProject",
			})
			if err := r.Get(ctx, client.ObjectKey{Name: repoBinding.Spec.PipelineName, Namespace: "argocd"}, appProject); err == nil {
				logger.Info("Deleting ArgoCD AppProject", "name", repoBinding.Spec.PipelineName)
				if err := r.Delete(ctx, appProject); err != nil && !errors.IsNotFound(err) {
					return fmt.Errorf("failed to delete ArgoCD AppProject: %w", err)
				}
			}

			appList := &unstructured.UnstructuredList{}
			appList.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "argoproj.io",
				Version: "v1alpha1",
				Kind:    "Application",
			})
			if err := r.List(ctx, appList, client.InNamespace("argocd"), client.MatchingLabels{
				"platform.aphex/pipeline": repoBinding.Spec.PipelineName,
			}); err == nil {
				for _, app := range appList.Items {
					appName := app.GetName()
					logger.Info("Deleting ArgoCD Application", "name", appName)
					if err := r.Delete(ctx, &app); err != nil && !errors.IsNotFound(err) {
						return fmt.Errorf("failed to delete ArgoCD Application %s: %w", appName, err)
					}
				}
			}

			controllerutil.RemoveFinalizer(repoBinding, repoBindingFinalizer)
			return r.Update(ctx, repoBinding)
		}
	}
	return nil
}

func (r *RepoBindingReconciler) ensureFinalizer(ctx context.Context, repoBinding *platformv1alpha1.RepoBinding) error {
	if !controllerutil.ContainsFinalizer(repoBinding, repoBindingFinalizer) {
		controllerutil.AddFinalizer(repoBinding, repoBindingFinalizer)
		return r.Update(ctx, repoBinding)
	}
	return nil
}

func (r *RepoBindingReconciler) ensureOwnerReference(ctx context.Context, repoBinding *platformv1alpha1.RepoBinding) error {
	for _, ownerRef := range repoBinding.GetOwnerReferences() {
		if ownerRef.Kind == "Organization" {
			return nil
		}
	}

	org := &platformv1alpha1.Organization{}
	if err := r.Get(ctx, client.ObjectKey{Name: repoBinding.Spec.AphexOrg, Namespace: "platform-system"}, org); err != nil {
		return fmt.Errorf("failed to get organization %s: %w", repoBinding.Spec.AphexOrg, err)
	}

	if err := controllerutil.SetOwnerReference(org, repoBinding, r.Scheme); err != nil {
		return fmt.Errorf("failed to set owner reference: %w", err)
	}

	return r.Update(ctx, repoBinding)
}

func (r *RepoBindingReconciler) initializeStatus(ctx context.Context, logger logr.Logger, repoBinding *platformv1alpha1.RepoBinding) error {
	if repoBinding.Status.Phase == "" {
		repoBinding.Status.Phase = "Pending"
		repoBinding.Status.Message = "Starting onboarding process"
		repoBinding.Status.LastReconcileTime = metav1.Now()
		if err := r.Status().Update(ctx, repoBinding); err != nil {
			logger.Error(err, "Failed to update RepoBinding status")
			return err
		}
	}
	return nil
}

func (r *RepoBindingReconciler) isAlreadyReady(logger logr.Logger, repoBinding *platformv1alpha1.RepoBinding) bool {
	if repoBinding.Status.Phase == "Ready" {
		logger.Info("RepoBinding already in Ready state, skipping reconciliation")
		return true
	}
	return false
}

func (r *RepoBindingReconciler) validateAndUpdatePhase(ctx context.Context, logger logr.Logger, request ctrl.Request, repoBinding *platformv1alpha1.RepoBinding) error {
	if repoBinding.Status.Phase == "Pending" {
		if err := ValidateRepoBinding(repoBinding, r.TemplateCatalog); err != nil {
			logger.Error(err, "Validation failed")
			return r.updateStatusToFailed(ctx, request, repoBinding, fmt.Sprintf("Validation failed: %s", err.Error()))
		}

		repoBinding.Status.Phase = "Provisioning"
		repoBinding.Status.Message = "Provisioning tenant resources"
		repoBinding.Status.LastReconcileTime = metav1.Now()
		if err := r.Status().Update(ctx, repoBinding); err != nil {
			logger.Error(err, "Failed to update RepoBinding status")
			return err
		}
	}
	return nil
}

func (r *RepoBindingReconciler) updateStatusToFailed(ctx context.Context, request ctrl.Request, repoBinding *platformv1alpha1.RepoBinding, message string) error {
	repoBinding.Status.Phase = "Failed"
	repoBinding.Status.Message = message
	repoBinding.Status.LastReconcileTime = metav1.Now()
	return r.Status().Update(ctx, repoBinding)
}

// SetupWithManager sets up the controller with the Manager
func (r *RepoBindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&platformv1alpha1.RepoBinding{}).
		Owns(&corev1.Namespace{}).
		Owns(&corev1.ServiceAccount{}).
		Complete(r)
}
