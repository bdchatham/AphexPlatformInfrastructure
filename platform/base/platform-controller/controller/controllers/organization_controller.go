package controllers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/cloudflare/cloudflare-go"
	esv1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1"
	esmeta "github.com/external-secrets/external-secrets/apis/meta/v1"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"

	platformv1alpha1 "github.com/bdchatham/AphexPlatformInfrastructure/platform/platform-controller/controller/api/v1alpha1"
	"github.com/bdchatham/AphexPlatformInfrastructure/platform/platform-controller/controller/controllers/config"
	"github.com/bdchatham/AphexPlatformInfrastructure/platform/platform-controller/controller/controllers/constants"
	"github.com/bdchatham/AphexPlatformInfrastructure/platform/platform-controller/controller/controllers/helpers"
	"github.com/bdchatham/AphexPlatformInfrastructure/platform/platform-controller/controller/controllers/metrics"
	triggersv1beta1 "github.com/tektoncd/triggers/pkg/apis/triggers/v1beta1"
)

// OrganizationReconciler reconciles an Organization object
type OrganizationReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	Log             logr.Logger
	Config          *config.Config
	statusHelper    *helpers.StatusHelper
	finalizerHelper *helpers.FinalizerHelper
}

// +kubebuilder:rbac:groups=aphex.io,resources=organizations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=aphex.io,resources=organizations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=aphex.io,resources=organizations/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=triggers.tekton.dev,resources=eventlisteners,verbs=get;list;watch;create;update;patch

// Reconcile manages Organization resources
func (r *OrganizationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Start metrics timer for reconciliation duration
	metricsCollector := metrics.GetMetricsCollector()
	timer := metricsCollector.NewReconcileTimer(constants.ControllerNameOrganization)

	// Initialize helpers if not already done (lazy initialization)
	if err := r.ensureHelpers(logger); err != nil {
		logger.Error(err, "Failed to initialize helpers")
		timer.ObserveError(metrics.ClassifyError(err))
		return ctrl.Result{}, err
	}

	// Apply provisioning timeout to the context
	timeout := constants.DefaultProvisioningTimeout
	if r.Config != nil {
		timeout = r.Config.ProvisioningTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Fetch the Organization instance
	org := &platformv1alpha1.Organization{}
	err := r.Get(ctx, req.NamespacedName, org)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		timer.ObserveError(metrics.ClassifyError(err))
		return ctrl.Result{}, err
	}

	// Handle deletion with verified cleanup using FinalizerHelper
	if r.finalizerHelper.IsBeingDeleted(org) {
		result, err := r.handleDeletionWithHelper(ctx, logger, org)
		if err != nil {
			timer.ObserveError(metrics.ClassifyError(err))
		} else {
			timer.ObserveSuccess()
			metrics.GetHealthState().RecordSuccess(constants.ControllerNameOrganization)
		}
		return result, err
	}

	if !r.finalizerHelper.HasFinalizer(org) {
		if err := r.validateOrganization(org); err != nil {
			logger.Error(err, "Validation failed, not adding finalizer")
			timer.ObserveError("validation")
			if patchErr := r.statusHelper.PatchStatus(ctx, org, map[string]interface{}{
				"phase":   constants.PhaseFailed,
				"message": fmt.Sprintf("Validation failed: %s", err.Error()),
			}); patchErr != nil {
				logger.Error(patchErr, "Failed to update status after validation failure")
			}
			return ctrl.Result{}, nil
		}
		if err := r.finalizerHelper.EnsureFinalizer(ctx, org); err != nil {
			timer.ObserveError(metrics.ClassifyError(err))
			return ctrl.Result{}, err
		}
		timer.ObserveSuccess()
		return ctrl.Result{Requeue: true}, nil
	}

	// Generate webhook secret if not provided
	if org.Spec.WebhookSecret == "" {
		secret, err := generateWebhookSecret()
		if err != nil {
			timer.ObserveError(metrics.ClassifyError(err))
			return ctrl.Result{}, fmt.Errorf("failed to generate webhook secret: %w", err)
		}
		org.Spec.WebhookSecret = secret
		if err := r.Update(ctx, org); err != nil {
			timer.ObserveError(metrics.ClassifyError(err))
			return ctrl.Result{}, err
		}
		timer.ObserveSuccess()
		return ctrl.Result{Requeue: true}, nil
	}

	// Initialize status if needed using StatusHelper
	if org.Status.Phase == "" {
		if err := r.statusHelper.PatchStatus(ctx, org, map[string]interface{}{
			"phase":      constants.PhasePending,
			"namespace":  fmt.Sprintf("org-%s", org.Name),
			"webhookURL": fmt.Sprintf("https://%s.%s", org.Name, constants.WebhookDomain),
			"message":    "Starting organization provisioning",
		}); err != nil {
			timer.ObserveError(metrics.ClassifyError(err))
			return ctrl.Result{}, err
		}
		timer.ObserveSuccess()
		return ctrl.Result{Requeue: true}, nil
	}

	// Update status to Provisioning if still Pending
	if org.Status.Phase == constants.PhasePending {
		if err := r.statusHelper.PatchStatus(ctx, org, map[string]interface{}{
			"phase":   constants.PhaseProvisioning,
			"message": "Provisioning organization resources",
		}); err != nil {
			timer.ObserveError(metrics.ClassifyError(err))
			return ctrl.Result{}, err
		}
	}

	// Provision organization resources
	if err := r.provisionOrganization(ctx, org); err != nil {
		timer.ObserveError(metrics.ClassifyError(err))
		if patchErr := r.statusHelper.PatchStatus(ctx, org, map[string]interface{}{
			"phase":   constants.PhaseFailed,
			"message": err.Error(),
		}); patchErr != nil {
			logger.Error(patchErr, "Failed to update status after provisioning failure")
		}
		return ctrl.Result{}, err
	}

	// Update status to Active if not already
	if org.Status.Phase != constants.PhaseActive {
		if err := r.statusHelper.PatchStatus(ctx, org, map[string]interface{}{
			"phase":   constants.PhaseActive,
			"message": "Organization provisioned successfully",
		}); err != nil {
			timer.ObserveError(metrics.ClassifyError(err))
			return ctrl.Result{}, err
		}
	}

	timer.ObserveSuccess()
	metrics.GetHealthState().RecordSuccess(constants.ControllerNameOrganization)
	logger.V(1).Info("Organization reconciled successfully", "organization", org.Name)
	return ctrl.Result{}, nil
}

// ensureHelpers initializes the helper components if not already done.
func (r *OrganizationReconciler) ensureHelpers(logger logr.Logger) error {
	if r.statusHelper == nil {
		retryCount := constants.DefaultStatusRetryCount
		if r.Config != nil {
			retryCount = r.Config.StatusRetryCount
		}
		r.statusHelper = helpers.NewStatusHelper(r.Client, logger, retryCount)
	}

	if r.finalizerHelper == nil {
		r.finalizerHelper = helpers.NewFinalizerHelper(r.Client, logger, constants.OrganizationFinalizer)
	}

	return nil
}

// validateOrganization validates the Organization spec before adding finalizer.
func (r *OrganizationReconciler) validateOrganization(org *platformv1alpha1.Organization) error {
	if org.Name == "" {
		return fmt.Errorf("organization name cannot be empty")
	}
	return nil
}

// handleDeletionWithHelper performs cleanup using FinalizerHelper with verified cleanup.
func (r *OrganizationReconciler) handleDeletionWithHelper(ctx context.Context, logger logr.Logger, org *platformv1alpha1.Organization) (ctrl.Result, error) {
	if !r.finalizerHelper.NeedsCleanup(org) {
		return ctrl.Result{}, nil
	}

	cleanupSteps := []helpers.CleanupStep{
		helpers.NewCleanupStep("Cloudflare tunnel", func(ctx context.Context) error {
			if err := r.deleteCloudflaredTunnel(ctx, org); err != nil {
				logger.Error(err, "Failed to delete Cloudflare tunnel, continuing with cleanup")
			}
			return nil
		}),
		helpers.NewCleanupStep("ClusterSecretStore", func(ctx context.Context) error {
			return r.cleanupClusterSecretStore(ctx, org)
		}),
		helpers.NewCleanupStep("ClusterRoleBinding", func(ctx context.Context) error {
			return r.cleanupClusterRoleBinding(ctx, org)
		}),
		helpers.NewCleanupStep("Organization namespace", func(ctx context.Context) error {
			return r.cleanupNamespace(ctx, org)
		}),
	}

	if err := r.finalizerHelper.HandleDeletionWithSteps(ctx, org, cleanupSteps); err != nil {
		return ctrl.Result{}, err
	}

	logger.V(1).Info("Organization deletion completed", "organization", org.Name)
	return ctrl.Result{}, nil
}

// cleanupClusterSecretStore removes the ClusterSecretStore for the organization.
func (r *OrganizationReconciler) cleanupClusterSecretStore(ctx context.Context, org *platformv1alpha1.Organization) error {
	store := &esv1.ClusterSecretStore{}
	storeName := fmt.Sprintf("org-%s-store", org.Name)
	if err := r.Get(ctx, client.ObjectKey{Name: storeName}, store); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to get ClusterSecretStore: %w", err)
	}
	if err := r.Delete(ctx, store); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete ClusterSecretStore: %w", err)
	}
	return nil
}

// cleanupClusterRoleBinding removes the ClusterRoleBinding for the organization's EventListener.
func (r *OrganizationReconciler) cleanupClusterRoleBinding(ctx context.Context, org *platformv1alpha1.Organization) error {
	crb := &rbacv1.ClusterRoleBinding{}
	crbName := fmt.Sprintf("eventlistener-%s", org.Name)
	if err := r.Get(ctx, client.ObjectKey{Name: crbName}, crb); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to get ClusterRoleBinding: %w", err)
	}
	if err := r.Delete(ctx, crb); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete ClusterRoleBinding: %w", err)
	}
	return nil
}

// cleanupNamespace removes the organization namespace.
func (r *OrganizationReconciler) cleanupNamespace(ctx context.Context, org *platformv1alpha1.Organization) error {
	namespace := &corev1.Namespace{}
	if err := r.Get(ctx, client.ObjectKey{Name: org.Status.Namespace}, namespace); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to get organization namespace: %w", err)
	}
	if err := r.Delete(ctx, namespace); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete organization namespace: %w", err)
	}
	return nil
}

// provisionOrganization creates all necessary resources for an organization
func (r *OrganizationReconciler) provisionOrganization(ctx context.Context, org *platformv1alpha1.Organization) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("context cancelled before provisioning: %w", ctx.Err())
	default:
	}

	// Create organization namespace
	if err := r.provisionNamespace(ctx, org); err != nil {
		return fmt.Errorf("failed to provision namespace: %w", err)
	}

	// Check context cancellation between steps
	select {
	case <-ctx.Done():
		return fmt.Errorf("context cancelled during provisioning: %w", ctx.Err())
	default:
	}

	// Create webhook secret
	if err := r.provisionWebhookSecret(ctx, org); err != nil {
		return fmt.Errorf("failed to provision webhook secret: %w", err)
	}

	// Check context cancellation between steps
	select {
	case <-ctx.Done():
		return fmt.Errorf("context cancelled during provisioning: %w", ctx.Err())
	default:
	}

	// Create SecretStore for ESO
	if err := r.provisionSecretStore(ctx, org); err != nil {
		return fmt.Errorf("failed to provision SecretStore: %w", err)
	}

	// Check context cancellation between steps
	select {
	case <-ctx.Done():
		return fmt.Errorf("context cancelled during provisioning: %w", ctx.Err())
	default:
	}

	// Create Cloudflared tunnel infrastructure
	if err := r.provisionCloudflaredTunnel(ctx, org); err != nil {
		return fmt.Errorf("failed to provision Cloudflared tunnel: %w", err)
	}

	// Check context cancellation between steps
	select {
	case <-ctx.Done():
		return fmt.Errorf("context cancelled during provisioning: %w", ctx.Err())
	default:
	}

	// Create RBAC for organization admins
	if err := r.provisionRBAC(ctx, org); err != nil {
		return fmt.Errorf("failed to provision RBAC: %w", err)
	}

	// Check context cancellation between steps
	select {
	case <-ctx.Done():
		return fmt.Errorf("context cancelled during provisioning: %w", ctx.Err())
	default:
	}

	if err := r.provisionEventListenerServiceAccount(ctx, org); err != nil {
		return fmt.Errorf("failed to provision EventListener ServiceAccount: %w", err)
	}

	return nil
}

// provisionNamespace creates the organization namespace with owner reference
func (r *OrganizationReconciler) provisionNamespace(ctx context.Context, org *platformv1alpha1.Organization) error {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: org.Status.Namespace,
			Labels: map[string]string{
				constants.LabelOrganization: org.Name,
				constants.LabelManagedBy:    constants.ManagedByOrganizationController,
				constants.LabelAphexOrg:     org.Name,
			},
		},
	}

	existingNs := &corev1.Namespace{}
	err := r.Get(ctx, client.ObjectKey{Name: org.Status.Namespace}, existingNs)
	if err != nil {
		if errors.IsNotFound(err) {
			return r.Create(ctx, namespace)
		}
		return err
	}

	existingNs.Labels = namespace.Labels
	return r.Update(ctx, existingNs)
}

// provisionWebhookSecret creates the GitHub webhook secret
func (r *OrganizationReconciler) provisionWebhookSecret(ctx context.Context, org *platformv1alpha1.Organization) error {
	// Create webhook secret in organization namespace
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.WebhookSecretName,
			Namespace: org.Status.Namespace,
			Labels:    r.orgLabels(org),
		},
		Data: map[string][]byte{
			"secret": []byte(org.Spec.WebhookSecret),
		},
	}

	existingSecret := &corev1.Secret{}
	err := r.Get(ctx, client.ObjectKey{Name: secret.Name, Namespace: secret.Namespace}, existingSecret)
	if err != nil {
		if errors.IsNotFound(err) {
			return r.Create(ctx, secret)
		}
		return err
	}

	existingSecret.Data = secret.Data
	existingSecret.Labels = secret.Labels
	return r.Update(ctx, existingSecret)
}

const esoServiceAccountName = constants.ESOSecretsReaderAccount

func (r *OrganizationReconciler) provisionSecretStore(ctx context.Context, org *platformv1alpha1.Organization) error {
	if err := r.provisionESOServiceAccount(ctx, org); err != nil {
		return err
	}
	if err := r.provisionESORole(ctx, org); err != nil {
		return err
	}
	if err := r.provisionESORoleBinding(ctx, org); err != nil {
		return err
	}
	return r.provisionESOSecretStore(ctx, org)
}

func (r *OrganizationReconciler) provisionESOServiceAccount(ctx context.Context, org *platformv1alpha1.Organization) error {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      esoServiceAccountName,
			Namespace: org.Status.Namespace,
			Labels:    r.orgLabels(org),
		},
	}
	return r.createOrUpdateObject(ctx, sa)
}

func (r *OrganizationReconciler) provisionESORole(ctx context.Context, org *platformv1alpha1.Organization) error {
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      esoServiceAccountName,
			Namespace: org.Status.Namespace,
			Labels:    r.orgLabels(org),
		},
		Rules: []rbacv1.PolicyRule{{
			APIGroups:     []string{""},
			Resources:     []string{"secrets"},
			ResourceNames: []string{"org-secrets"},
			Verbs:         []string{"get"},
		}},
	}
	return r.createOrUpdateObject(ctx, role)
}

func (r *OrganizationReconciler) provisionESORoleBinding(ctx context.Context, org *platformv1alpha1.Organization) error {
	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      esoServiceAccountName,
			Namespace: org.Status.Namespace,
			Labels:    r.orgLabels(org),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     esoServiceAccountName,
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      esoServiceAccountName,
			Namespace: org.Status.Namespace,
		}},
	}
	return r.createOrUpdateObject(ctx, rb)
}

func (r *OrganizationReconciler) provisionESOSecretStore(ctx context.Context, org *platformv1alpha1.Organization) error {
	ns := org.Status.Namespace
	store := &esv1.ClusterSecretStore{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("org-%s-store", org.Name),
			Labels: map[string]string{
				constants.LabelOrganization: org.Name,
				constants.LabelManagedBy:    constants.ManagedByOrganizationController,
				constants.LabelOwnerName:    org.Name,
				constants.LabelOwnerKind:    "Organization",
			},
		},
		Spec: esv1.SecretStoreSpec{
			Conditions: []esv1.ClusterSecretStoreCondition{{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						constants.LabelAphexOrg: org.Name,
					},
				},
			}},
			Provider: &esv1.SecretStoreProvider{
				Kubernetes: &esv1.KubernetesProvider{
					RemoteNamespace: ns,
					Server: esv1.KubernetesServer{
						CAProvider: &esv1.CAProvider{
							Type:      esv1.CAProviderTypeConfigMap,
							Name:      "kube-root-ca.crt",
							Key:       "ca.crt",
							Namespace: &ns,
						},
					},
					Auth: &esv1.KubernetesAuth{
						ServiceAccount: &esmeta.ServiceAccountSelector{
							Name:      esoServiceAccountName,
							Namespace: &ns,
						},
					},
				},
			},
		},
	}

	existing := &esv1.ClusterSecretStore{}
	err := r.Get(ctx, client.ObjectKey{Name: store.Name}, existing)
	if errors.IsNotFound(err) {
		return r.Create(ctx, store)
	}
	if err != nil {
		return err
	}
	store.SetResourceVersion(existing.GetResourceVersion())
	return r.Update(ctx, store)
}

func (r *OrganizationReconciler) orgLabels(org *platformv1alpha1.Organization) map[string]string {
	return map[string]string{
		constants.LabelOrganization: org.Name,
		constants.LabelManagedBy:    constants.ManagedByOrganizationController,
	}
}

func (r *OrganizationReconciler) createOrUpdateObject(ctx context.Context, obj client.Object) error {
	existing := obj.DeepCopyObject().(client.Object)
	err := r.Get(ctx, client.ObjectKeyFromObject(obj), existing)
	if errors.IsNotFound(err) {
		return r.Create(ctx, obj)
	}
	if err != nil {
		return err
	}
	obj.SetResourceVersion(existing.GetResourceVersion())
	return r.Update(ctx, obj)
}

// provisionRBAC creates RBAC for organization admins
func (r *OrganizationReconciler) provisionRBAC(ctx context.Context, org *platformv1alpha1.Organization) error {
	// Create Role in organization namespace
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.OrganizationAdminRoleName,
			Namespace: org.Status.Namespace,
			Labels:    r.orgLabels(org),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"*"},
				Verbs:     []string{"*"},
			},
			{
				APIGroups: []string{"tekton.dev"},
				Resources: []string{"*"},
				Verbs:     []string{"*"},
			},
			{
				APIGroups: []string{"aphex.io"},
				Resources: []string{"repobindings"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch"},
			},
		},
	}

	existingRole := &rbacv1.Role{}
	err := r.Get(ctx, client.ObjectKey{Name: role.Name, Namespace: role.Namespace}, existingRole)
	if err != nil {
		if errors.IsNotFound(err) {
			if err := r.Create(ctx, role); err != nil {
				return err
			}
		} else {
			return err
		}
	} else {
		existingRole.Rules = role.Rules
		existingRole.Labels = role.Labels
		if err := r.Update(ctx, existingRole); err != nil {
			return err
		}
	}

	// Create RoleBinding for admin users
	for _, adminUser := range org.Spec.AdminUsers {
		roleBinding := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%s", constants.OrganizationAdminRoleName, adminUser),
				Namespace: org.Status.Namespace,
				Labels:    r.orgLabels(org),
			},
			Subjects: []rbacv1.Subject{
				{
					Kind: "User",
					Name: adminUser,
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     constants.OrganizationAdminRoleName,
			},
		}

		existingRoleBinding := &rbacv1.RoleBinding{}
		err := r.Get(ctx, client.ObjectKey{Name: roleBinding.Name, Namespace: roleBinding.Namespace}, existingRoleBinding)
		if err != nil {
			if errors.IsNotFound(err) {
				if err := r.Create(ctx, roleBinding); err != nil {
					return err
				}
			} else {
				return err
			}
		}
	}

	return nil
}

func (r *OrganizationReconciler) provisionEventListenerServiceAccount(ctx context.Context, org *platformv1alpha1.Organization) error {
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.EventListenerServiceAccount,
			Namespace: org.Status.Namespace,
			Labels:    r.orgLabels(org),
		},
	}

	existingSA := &corev1.ServiceAccount{}
	err := r.Get(ctx, client.ObjectKey{Name: serviceAccount.Name, Namespace: serviceAccount.Namespace}, existingSA)
	if err != nil {
		if errors.IsNotFound(err) {
			if err := r.Create(ctx, serviceAccount); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s", constants.EventListenerServiceAccount, org.Name),
			Labels: map[string]string{
				constants.LabelOrganization: org.Name,
				constants.LabelManagedBy:    constants.ManagedByOrganizationController,
				constants.LabelOwnerName:    org.Name,
				constants.LabelOwnerKind:    "Organization",
			},
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      constants.EventListenerServiceAccount,
				Namespace: org.Status.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     constants.EventListenerAccessRole,
		},
	}

	existingClusterRoleBinding := &rbacv1.ClusterRoleBinding{}
	err = r.Get(ctx, client.ObjectKey{Name: clusterRoleBinding.Name}, existingClusterRoleBinding)
	if err != nil {
		if errors.IsNotFound(err) {
			if err := r.Create(ctx, clusterRoleBinding); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	return nil
}

// generateWebhookSecret generates a random webhook secret
func generateWebhookSecret() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// provisionCloudflaredTunnel creates Cloudflared tunnel infrastructure for the organization
func (r *OrganizationReconciler) provisionCloudflaredTunnel(ctx context.Context, org *platformv1alpha1.Organization) error {
	// Get platform namespace from config or use default
	platformNS := constants.DefaultPlatformNamespace
	if r.Config != nil {
		platformNS = r.Config.PlatformNamespace
	}

	// Get Cloudflare API token from cluster secret
	apiTokenSecret := &corev1.Secret{}
	err := r.Get(ctx, client.ObjectKey{Name: constants.CloudflareAPITokenSecret, Namespace: platformNS}, apiTokenSecret)
	if err != nil {
		return fmt.Errorf("failed to get Cloudflare API token secret: %w", err)
	}

	apiToken := string(apiTokenSecret.Data["token"])
	if apiToken == "" {
		return fmt.Errorf("Cloudflare API token not found in secret")
	}

	// Create Cloudflare API client
	api, err := cloudflare.NewWithAPIToken(apiToken)
	if err != nil {
		return fmt.Errorf("failed to create Cloudflare API client: %w", err)
	}

	// Get account ID (required for tunnel operations)
	accounts, _, err := api.Accounts(ctx, cloudflare.AccountsListParams{})
	if err != nil {
		return fmt.Errorf("failed to list Cloudflare accounts: %w", err)
	}
	if len(accounts) == 0 {
		return fmt.Errorf("no Cloudflare accounts found")
	}
	accountID := accounts[0].ID

	// Check if we already have valid tunnel credentials
	tunnelName := fmt.Sprintf("webhooks-%s", org.Name)
	existingCredentialsSecret := &corev1.Secret{}
	err = r.Get(ctx, client.ObjectKey{
		Name:      fmt.Sprintf("cloudflared-credentials-%s", org.Name),
		Namespace: org.Status.Namespace,
	}, existingCredentialsSecret)

	var tunnel cloudflare.Tunnel
	var tunnelSecret string

	if err == nil {
		// We have existing credentials, parse them to get tunnel info
		credentialsJSON := existingCredentialsSecret.Data["credentials.json"]
		var credentials map[string]interface{}
		if json.Unmarshal(credentialsJSON, &credentials) == nil {
			if tunnelID, ok := credentials["TunnelID"].(string); ok && tunnelID != "" {
				if secret, ok := credentials["TunnelSecret"].(string); ok && secret != "" {
					// We have valid existing credentials, use them
					tunnel = cloudflare.Tunnel{
						ID:     tunnelID,
						Name:   tunnelName,
						Secret: secret,
					}
					tunnelSecret = secret
				}
			}
		}
	}

	// If we don't have valid existing credentials, get or create tunnel
	if tunnel.ID == "" {
		tunnel, err = r.createFreshTunnelWithSecret(ctx, api, accountID, tunnelName)
		if err != nil {
			return fmt.Errorf("failed to get or create Cloudflare tunnel: %w", err)
		}
		tunnelSecret = tunnel.Secret

		if err := r.createTunnelDNSRecord(ctx, api, accountID, org.Name, tunnel.ID); err != nil {
			return fmt.Errorf("failed to create DNS record: %w", err)
		}
	}

	// Create tunnel credentials
	credentials := map[string]interface{}{
		"AccountTag":   accountID,
		"TunnelSecret": tunnelSecret,
		"TunnelID":     tunnel.ID,
	}
	credentialsJSON, err := json.Marshal(credentials)
	if err != nil {
		return fmt.Errorf("failed to marshal tunnel credentials: %w", err)
	}

	// Create Cloudflared credentials secret
	credentialsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("cloudflared-credentials-%s", org.Name),
			Namespace: org.Status.Namespace,
			Labels:    r.orgLabels(org),
		},
		Data: map[string][]byte{
			"credentials.json": credentialsJSON,
		},
	}

	existingCredentialsSecret = &corev1.Secret{}
	err = r.Get(ctx, client.ObjectKey{Name: credentialsSecret.Name, Namespace: credentialsSecret.Namespace}, existingCredentialsSecret)
	if err != nil {
		if errors.IsNotFound(err) {
			if err := r.Create(ctx, credentialsSecret); err != nil {
				return err
			}
		} else {
			return err
		}
	} else {
		existingCredentialsSecret.Data = credentialsSecret.Data
		existingCredentialsSecret.Labels = credentialsSecret.Labels
		if err := r.Update(ctx, existingCredentialsSecret); err != nil {
			return err
		}
	}

	// Create Cloudflared ConfigMap
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cloudflared-config",
			Namespace: org.Status.Namespace,
			Labels:    r.orgLabels(org),
		},
		Data: map[string]string{
			"config.yaml": fmt.Sprintf(`tunnel: %s
credentials-file: /etc/cloudflared/credentials/credentials.json

ingress:
  - hostname: %s.%s
    service: http://el-%s:8080
  - service: http_status:404`, tunnel.ID, org.Name, constants.WebhookDomain, constants.GitHubListenerName),
		},
	}

	existingConfigMap := &corev1.ConfigMap{}
	err = r.Get(ctx, client.ObjectKey{Name: configMap.Name, Namespace: configMap.Namespace}, existingConfigMap)
	if err != nil {
		if errors.IsNotFound(err) {
			if err := r.Create(ctx, configMap); err != nil {
				return err
			}
		} else {
			return err
		}
	} else {
		existingConfigMap.Data = configMap.Data
		existingConfigMap.Labels = configMap.Labels
		if err := r.Update(ctx, existingConfigMap); err != nil {
			return err
		}
	}

	// Create Cloudflared Deployment
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("cloudflared-%s", org.Name),
			Namespace: org.Status.Namespace,
			Labels: map[string]string{
				constants.LabelOrganization: org.Name,
				constants.LabelManagedBy:    constants.ManagedByOrganizationController,
				"app":                       "cloudflared",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "cloudflared",
					"org": org.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "cloudflared",
						"org": org.Name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "cloudflared",
							Image: "cloudflare/cloudflared:latest",
							Args:  []string{"tunnel", "--config", "/etc/cloudflared/config/config.yaml", "run"},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "config",
									MountPath: "/etc/cloudflared/config",
									ReadOnly:  true,
								},
								{
									Name:      "credentials",
									MountPath: "/etc/cloudflared/credentials",
									ReadOnly:  true,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "cloudflared-config",
									},
								},
							},
						},
						{
							Name: "credentials",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: fmt.Sprintf("cloudflared-credentials-%s", org.Name),
								},
							},
						},
					},
				},
			},
		},
	}

	existingDeployment := &appsv1.Deployment{}
	err = r.Get(ctx, client.ObjectKey{Name: deployment.Name, Namespace: deployment.Namespace}, existingDeployment)
	if err != nil {
		if errors.IsNotFound(err) {
			if err := r.Create(ctx, deployment); err != nil {
				return err
			}
		} else {
			return err
		}
	} else {
		existingDeployment.Spec = deployment.Spec
		existingDeployment.Labels = deployment.Labels
		if err := r.Update(ctx, existingDeployment); err != nil {
			return err
		}
	}

	// Create EventListener
	orgNamespace := fmt.Sprintf("org-%s", org.Name)
	eventListener := &triggersv1beta1.EventListener{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.GitHubListenerName,
			Namespace: orgNamespace,
			Labels:    r.orgLabels(org),
		},
		Spec: triggersv1beta1.EventListenerSpec{
			ServiceAccountName: constants.EventListenerServiceAccount,
			NamespaceSelector: triggersv1beta1.NamespaceSelector{
				MatchNames: []string{},
			},
			TriggerGroups: []triggersv1beta1.EventListenerTriggerGroup{
				{
					Name: "github-webhooks",
					TriggerSelector: triggersv1beta1.EventListenerTriggerSelector{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								constants.LabelOrganization: org.Name,
							},
						},
					},
					Interceptors: []*triggersv1beta1.TriggerInterceptor{
						{
							Name: stringPtr("github"),
							Ref: triggersv1beta1.InterceptorRef{
								Name: "github",
								Kind: triggersv1beta1.ClusterInterceptorKind,
							},
						},
					},
				},
			},
		},
	}

	existingEventListener := &triggersv1beta1.EventListener{}
	err = r.Get(ctx, client.ObjectKey{Name: eventListener.Name, Namespace: eventListener.Namespace}, existingEventListener)
	if err != nil {
		if errors.IsNotFound(err) {
			if err := r.Create(ctx, eventListener); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	triggerBinding := &triggersv1beta1.TriggerBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.GitHubPushBindingName,
			Namespace: orgNamespace,
			Labels:    r.orgLabels(org),
		},
		Spec: triggersv1beta1.TriggerBindingSpec{
			Params: []triggersv1beta1.Param{
				{
					Name:  "git-url",
					Value: "$(body.repository.clone_url)",
				},
				{
					Name:  "git-revision",
					Value: "$(body.after)",
				},
			},
		},
	}

	existingBinding := &triggersv1beta1.TriggerBinding{}
	err = r.Get(ctx, client.ObjectKey{Name: triggerBinding.Name, Namespace: triggerBinding.Namespace}, existingBinding)
	if err != nil {
		if errors.IsNotFound(err) {
			if err := r.Create(ctx, triggerBinding); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	return nil
}

func (r *OrganizationReconciler) createFreshTunnelWithSecret(ctx context.Context, api *cloudflare.API, accountID, tunnelName string) (cloudflare.Tunnel, error) {
	tunnels, _, err := api.ListTunnels(ctx, cloudflare.AccountIdentifier(accountID), cloudflare.TunnelListParams{
		Name: tunnelName,
	})
	if err != nil {
		return cloudflare.Tunnel{}, fmt.Errorf("failed to list tunnels: %w", err)
	}

	for _, tunnel := range tunnels {
		if tunnel.Name == tunnelName {
			err := api.DeleteTunnel(ctx, cloudflare.AccountIdentifier(accountID), tunnel.ID)
			if err != nil {
				return cloudflare.Tunnel{}, fmt.Errorf("failed to delete existing tunnel: %w", err)
			}
			break
		}
	}

	tunnelSecret := generateTunnelSecret()
	tunnel, err := api.CreateTunnel(ctx, cloudflare.AccountIdentifier(accountID), cloudflare.TunnelCreateParams{
		Name:   tunnelName,
		Secret: tunnelSecret,
	})
	if err != nil {
		return cloudflare.Tunnel{}, fmt.Errorf("failed to create tunnel: %w", err)
	}

	tunnel.Secret = tunnelSecret
	return tunnel, nil
}

// generateTunnelSecret generates a random secret for the tunnel
func generateTunnelSecret() string {
	secret := make([]byte, 32)
	rand.Read(secret)
	return base64.StdEncoding.EncodeToString(secret)
}

// deleteCloudflaredTunnel deletes the Cloudflare tunnel for the organization
func (r *OrganizationReconciler) deleteCloudflaredTunnel(ctx context.Context, org *platformv1alpha1.Organization) error {
	// Get platform namespace from config or use default
	platformNS := constants.DefaultPlatformNamespace
	if r.Config != nil {
		platformNS = r.Config.PlatformNamespace
	}

	// Get Cloudflare API token from cluster secret
	apiTokenSecret := &corev1.Secret{}
	err := r.Get(ctx, client.ObjectKey{Name: constants.CloudflareAPITokenSecret, Namespace: platformNS}, apiTokenSecret)
	if err != nil {
		return fmt.Errorf("failed to get Cloudflare API token secret: %w", err)
	}

	apiToken := string(apiTokenSecret.Data["token"])
	if apiToken == "" {
		return fmt.Errorf("Cloudflare API token not found in secret")
	}

	// Get tunnel credentials to find tunnel ID
	credentialsSecret := &corev1.Secret{}
	err = r.Get(ctx, client.ObjectKey{
		Name:      fmt.Sprintf("cloudflared-credentials-%s", org.Name),
		Namespace: org.Status.Namespace,
	}, credentialsSecret)
	if err != nil {
		if errors.IsNotFound(err) {
			// No credentials secret means tunnel was never created successfully
			return nil
		}
		return fmt.Errorf("failed to get tunnel credentials: %w", err)
	}

	// Parse tunnel credentials to get tunnel ID
	credentialsJSON := credentialsSecret.Data["credentials.json"]
	var credentials map[string]interface{}
	if err := json.Unmarshal(credentialsJSON, &credentials); err != nil {
		return fmt.Errorf("failed to parse tunnel credentials: %w", err)
	}

	tunnelID, ok := credentials["TunnelID"].(string)
	if !ok {
		return fmt.Errorf("tunnel ID not found in credentials")
	}

	accountID, ok := credentials["AccountTag"].(string)
	if !ok {
		return fmt.Errorf("account ID not found in credentials")
	}

	// Create Cloudflare API client
	api, err := cloudflare.NewWithAPIToken(apiToken)
	if err != nil {
		return fmt.Errorf("failed to create Cloudflare API client: %w", err)
	}

	if err := r.deleteTunnelDNSRecord(ctx, api, accountID, org.Name); err != nil {
		return fmt.Errorf("failed to delete DNS record: %w", err)
	}

	err = api.CleanupTunnelConnections(ctx, cloudflare.AccountIdentifier(accountID), tunnelID)
	if err != nil {
		return fmt.Errorf("failed to cleanup tunnel connections for %s: %w", tunnelID, err)
	}

	err = api.DeleteTunnel(ctx, cloudflare.AccountIdentifier(accountID), tunnelID)
	if err != nil {
		return fmt.Errorf("failed to delete Cloudflare tunnel %s: %w", tunnelID, err)
	}

	return nil
}

func (r *OrganizationReconciler) deleteTunnelDNSRecord(ctx context.Context, api *cloudflare.API, accountID, orgName string) error {
	zones, err := api.ListZones(ctx, "arbiter-dev.com")
	if err != nil {
		return fmt.Errorf("failed to list zones: %w", err)
	}
	if len(zones) == 0 {
		return fmt.Errorf("zone arbiter-dev.com not found")
	}
	zoneID := zones[0].ID

	hostname := fmt.Sprintf("%s.arbiter-dev.com", orgName)

	records, _, err := api.ListDNSRecords(ctx, cloudflare.ZoneIdentifier(zoneID), cloudflare.ListDNSRecordsParams{
		Name: hostname,
		Type: "CNAME",
	})
	if err != nil {
		return fmt.Errorf("failed to list DNS records: %w", err)
	}

	for _, record := range records {
		if err := api.DeleteDNSRecord(ctx, cloudflare.ZoneIdentifier(zoneID), record.ID); err != nil {
			return fmt.Errorf("failed to delete DNS record: %w", err)
		}
	}

	return nil
}

func (r *OrganizationReconciler) createTunnelDNSRecord(ctx context.Context, api *cloudflare.API, accountID, orgName, tunnelID string) error {
	zones, err := api.ListZones(ctx, "arbiter-dev.com")
	if err != nil {
		return fmt.Errorf("failed to list zones: %w", err)
	}
	if len(zones) == 0 {
		return fmt.Errorf("zone arbiter-dev.com not found")
	}
	zoneID := zones[0].ID

	hostname := fmt.Sprintf("%s.arbiter-dev.com", orgName)
	tunnelHostname := fmt.Sprintf("%s.cfargotunnel.com", tunnelID)

	_, err = api.CreateDNSRecord(ctx, cloudflare.ZoneIdentifier(zoneID), cloudflare.CreateDNSRecordParams{
		Type:    "CNAME",
		Name:    hostname,
		Content: tunnelHostname,
		Proxied: cloudflare.BoolPtr(true),
	})
	if err != nil {
		return fmt.Errorf("failed to create DNS record: %w", err)
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager
func (r *OrganizationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return r.SetupWithManagerAndOptions(mgr, controller.Options{})
}

// SetupWithManagerAndOptions sets up the controller with the Manager and custom options.
// This allows configuring rate limiting and max concurrent reconciles.
func (r *OrganizationReconciler) SetupWithManagerAndOptions(mgr ctrl.Manager, opts controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&platformv1alpha1.Organization{}).
		WithOptions(opts).
		Complete(r)
}

// int32Ptr returns a pointer to an int32 value
func int32Ptr(i int32) *int32 {
	return &i
}
