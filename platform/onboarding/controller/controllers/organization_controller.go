package controllers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	appsv1 "k8s.io/api/apps/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	platformv1alpha1 "github.com/arbiter/jenkinsx-platform/onboarding-controller/api/v1alpha1"
)

const (
	organizationFinalizer = "platform.arbiter.io/organization-finalizer"
)

// OrganizationReconciler reconciles an Organization object
type OrganizationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=arbiter.io,resources=organizations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=arbiter.io,resources=organizations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=arbiter.io,resources=organizations/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch
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

	// Handle deletion
	if org.DeletionTimestamp != nil {
		return r.handleDeletion(ctx, org)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(org, organizationFinalizer) {
		controllerutil.AddFinalizer(org, organizationFinalizer)
		return ctrl.Result{}, r.Update(ctx, org)
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

	// Create Cloudflared Deployment
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("cloudflared-%s", org.Name),
			Namespace: org.Status.Namespace,
			Labels: map[string]string{
				"platform.arbiter.io/organization": org.Name,
				"platform.arbiter.io/managed-by":   "organization-controller",
				"app":                               "cloudflared",
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

	return nil
}

// handleDeletion cleans up organization resources and removes finalizer
func (r *OrganizationReconciler) handleDeletion(ctx context.Context, org *platformv1alpha1.Organization) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Handling organization deletion", "organization", org.Name)

	// Delete organization namespace (this cascades to all resources in the namespace)
	namespace := &corev1.Namespace{}
	err := r.Get(ctx, client.ObjectKey{Name: org.Status.Namespace}, namespace)
	if err == nil {
		if err := r.Delete(ctx, namespace); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to delete organization namespace: %w", err)
		}
		log.Info("Deleted organization namespace", "namespace", org.Status.Namespace)
	} else if !errors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("failed to get organization namespace: %w", err)
	}

	// Remove finalizer
	controllerutil.RemoveFinalizer(org, organizationFinalizer)
	if err := r.Update(ctx, org); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
	}

	log.Info("Organization deletion completed", "organization", org.Name)
	return ctrl.Result{}, nil
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
