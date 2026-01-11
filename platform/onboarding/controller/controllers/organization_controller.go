package controllers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	platformv1alpha1 "github.com/arbiter/jenkinsx-platform/onboarding-controller/api/v1alpha1"
)

// OrganizationReconciler reconciles an Organization object
type OrganizationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=arbiter.io,resources=organizations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=arbiter.io,resources=organizations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=arbiter.io,resources=organizations/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch

// Reconcile manages Organization resources
func (r *OrganizationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the Organization instance
	org := &platformv1alpha1.Organization{}
	err := r.Get(ctx, req.NamespacedName, org)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Generate webhook secret if not provided
	if org.Spec.WebhookSecret == "" {
		secret, err := generateWebhookSecret()
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to generate webhook secret: %w", err)
		}
		org.Spec.WebhookSecret = secret
		if err := r.Update(ctx, org); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Set initial status
	if org.Status.Phase == "" {
		org.Status.Phase = "Pending"
		org.Status.Namespace = fmt.Sprintf("org-%s", org.Name)
		org.Status.WebhookURL = fmt.Sprintf("https://webhooks-%s.homelab.local", org.Name)
		if err := r.Status().Update(ctx, org); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Provision organization resources
	if err := r.provisionOrganization(ctx, org); err != nil {
		org.Status.Phase = "Failed"
		org.Status.Message = err.Error()
		r.Status().Update(ctx, org)
		return ctrl.Result{}, err
	}

	// Update status to Active
	org.Status.Phase = "Active"
	org.Status.Message = "Organization provisioned successfully"
	if err := r.Status().Update(ctx, org); err != nil {
		return ctrl.Result{}, err
	}

	log.Info("Organization reconciled successfully", "organization", org.Name)
	return ctrl.Result{}, nil
}

// provisionOrganization creates all necessary resources for an organization
func (r *OrganizationReconciler) provisionOrganization(ctx context.Context, org *platformv1alpha1.Organization) error {
	// Create organization namespace
	if err := r.provisionNamespace(ctx, org); err != nil {
		return fmt.Errorf("failed to provision namespace: %w", err)
	}

	// Create webhook secret
	if err := r.provisionWebhookSecret(ctx, org); err != nil {
		return fmt.Errorf("failed to provision webhook secret: %w", err)
	}

	// Create Cloudflared tunnel infrastructure
	if err := r.provisionCloudflaredTunnel(ctx, org); err != nil {
		return fmt.Errorf("failed to provision Cloudflared tunnel: %w", err)
	}

	// Create RBAC for organization admins
	if err := r.provisionRBAC(ctx, org); err != nil {
		return fmt.Errorf("failed to provision RBAC: %w", err)
	}

	return nil
}

// provisionNamespace creates the organization namespace
func (r *OrganizationReconciler) provisionNamespace(ctx context.Context, org *platformv1alpha1.Organization) error {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: org.Status.Namespace,
			Labels: map[string]string{
				"platform.arbiter.io/organization": org.Name,
				"platform.arbiter.io/managed-by":   "organization-controller",
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

	// Update labels if namespace exists
	existingNs.Labels = namespace.Labels
	return r.Update(ctx, existingNs)
}

// provisionWebhookSecret creates the GitHub webhook secret
func (r *OrganizationReconciler) provisionWebhookSecret(ctx context.Context, org *platformv1alpha1.Organization) error {
	// Create webhook secret in organization namespace
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "github-webhook-secret",
			Namespace: org.Status.Namespace,
			Labels: map[string]string{
				"platform.arbiter.io/organization": org.Name,
				"platform.arbiter.io/managed-by":   "organization-controller",
			},
		},
		Data: map[string][]byte{
			"secret": []byte(org.Spec.WebhookSecret),
		},
	}

	// Also create Cloudflared credentials secret in organization namespace
	cloudflaredSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("cloudflared-credentials-%s", org.Name),
			Namespace: org.Status.Namespace,
			Labels: map[string]string{
				"platform.arbiter.io/organization": org.Name,
				"platform.arbiter.io/managed-by":   "organization-controller",
			},
		},
		Data: map[string][]byte{
			"credentials.json": []byte("{}"), // Placeholder - will be updated by bootstrap
		},
	}

	// Create webhook secret
	existingSecret := &corev1.Secret{}
	err := r.Get(ctx, client.ObjectKey{Name: secret.Name, Namespace: secret.Namespace}, existingSecret)
	if err != nil {
		if errors.IsNotFound(err) {
			if err := r.Create(ctx, secret); err != nil {
				return err
			}
		} else {
			return err
		}
	} else {
		existingSecret.Data = secret.Data
		existingSecret.Labels = secret.Labels
		if err := r.Update(ctx, existingSecret); err != nil {
			return err
		}
	}

	// Create Cloudflared credentials secret
	existingCloudflaredSecret := &corev1.Secret{}
	err = r.Get(ctx, client.ObjectKey{Name: cloudflaredSecret.Name, Namespace: cloudflaredSecret.Namespace}, existingCloudflaredSecret)
	if err != nil {
		if errors.IsNotFound(err) {
			return r.Create(ctx, cloudflaredSecret)
		}
		return err
	}

	return nil
}

// provisionRBAC creates RBAC for organization admins
func (r *OrganizationReconciler) provisionRBAC(ctx context.Context, org *platformv1alpha1.Organization) error {
	// Create Role in organization namespace
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "organization-admin",
			Namespace: org.Status.Namespace,
			Labels: map[string]string{
				"platform.arbiter.io/organization": org.Name,
				"platform.arbiter.io/managed-by":   "organization-controller",
			},
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
				APIGroups: []string{"arbiter.io"},
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
				Name:      fmt.Sprintf("organization-admin-%s", adminUser),
				Namespace: org.Status.Namespace,
				Labels: map[string]string{
					"platform.arbiter.io/organization": org.Name,
					"platform.arbiter.io/managed-by":   "organization-controller",
				},
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
				Name:     "organization-admin",
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
	// Create Cloudflared ConfigMap
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cloudflared-config",
			Namespace: org.Status.Namespace,
			Labels: map[string]string{
				"platform.arbiter.io/organization": org.Name,
				"platform.arbiter.io/managed-by":   "organization-controller",
			},
		},
		Data: map[string]string{
			"config.yaml": fmt.Sprintf(`tunnel: %s
credentials-file: /etc/cloudflared/credentials/credentials.json

ingress:
  - hostname: webhooks-%s.homelab.local
    service: http://el-github-listener:8080
  - service: http_status:404`, org.Name, org.Name),
		},
	}

	existingConfigMap := &corev1.ConfigMap{}
	err := r.Get(ctx, client.ObjectKey{Name: configMap.Name, Namespace: configMap.Namespace}, existingConfigMap)
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

	return nil
}

// SetupWithManager sets up the controller with the Manager
func (r *OrganizationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&platformv1alpha1.Organization{}).
		Complete(r)
}

// int32Ptr returns a pointer to an int32 value
func int32Ptr(i int32) *int32 {
	return &i
}
