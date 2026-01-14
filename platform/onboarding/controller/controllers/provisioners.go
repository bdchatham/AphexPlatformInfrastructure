package controllers

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"gopkg.in/yaml.v3"

	platformv1alpha1 "github.com/arbiter/jenkinsx-platform/onboarding-controller/api/v1alpha1"
	triggersv1beta1 "github.com/tektoncd/triggers/pkg/apis/triggers/v1beta1"
)

// provisionNamespace creates or updates the tenant namespace
func (r *RepoBindingReconciler) provisionNamespace(ctx context.Context, rb *platformv1alpha1.RepoBinding) error {
	repoLabel := fmt.Sprintf("%s-%s", rb.Spec.RepoOrg, rb.Spec.RepoName)
	
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: rb.Spec.TenantName,
			Labels: map[string]string{
				"platform.arbiter.io/tenant":     rb.Spec.TenantName,
				"platform.arbiter.io/repo":       repoLabel,
				"platform.arbiter.io/managed-by": "onboarding-controller",
			},
		},
	}

	// Try to get existing namespace
	existingNs := &corev1.Namespace{}
	err := r.Get(ctx, client.ObjectKey{Name: rb.Spec.TenantName}, existingNs)
	if err != nil {
		if errors.IsNotFound(err) {
			// Create new namespace
			r.Log.Info("Creating tenant namespace", "namespace", rb.Spec.TenantName)
			if err := r.Create(ctx, namespace); err != nil {
				return fmt.Errorf("failed to create namespace: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to get namespace: %w", err)
	}

	// Namespace exists, update labels if needed
	r.Log.Info("Namespace already exists, updating labels", "namespace", rb.Spec.TenantName)
	existingNs.Labels = namespace.Labels
	if err := r.Update(ctx, existingNs); err != nil {
		return fmt.Errorf("failed to update namespace labels: %w", err)
	}

	return nil
}

// setOwnerReference sets the RepoBinding as the owner of a resource
func setOwnerReference(rb *platformv1alpha1.RepoBinding, obj client.Object, scheme *runtime.Scheme) error {
	return controllerutil.SetControllerReference(rb, obj, scheme)
}

// provisionServiceAccount creates or updates the tenant service account
func (r *RepoBindingReconciler) provisionServiceAccount(ctx context.Context, rb *platformv1alpha1.RepoBinding) error {
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pipeline-runner",
			Namespace: rb.Spec.TenantName,
		},
	}

	// Try to get existing service account
	existingSA := &corev1.ServiceAccount{}
	err := r.Get(ctx, client.ObjectKey{Name: "pipeline-runner", Namespace: rb.Spec.TenantName}, existingSA)
	if err != nil {
		if errors.IsNotFound(err) {
			// Create new service account
			r.Log.Info("Creating service account", "namespace", rb.Spec.TenantName, "name", "pipeline-runner")
			if err := r.Create(ctx, serviceAccount); err != nil {
				return fmt.Errorf("failed to create service account: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to get service account: %w", err)
	}

	// Service account exists, nothing to update
	r.Log.Info("Service account already exists", "namespace", rb.Spec.TenantName, "name", "pipeline-runner")
	return nil
}

// provisionRBAC creates or updates the tenant RBAC (Role, RoleBinding, ClusterRole, ClusterRoleBinding)
func (r *RepoBindingReconciler) provisionRBAC(ctx context.Context, rb *platformv1alpha1.RepoBinding) error {
	// Determine permission profile (default to "standard")
	profile := rb.Spec.PermissionProfile
	if profile == "" {
		profile = "standard"
	}

	// Create namespace-scoped Role
	if err := r.provisionRole(ctx, rb, profile); err != nil {
		return fmt.Errorf("failed to provision role: %w", err)
	}

	// Create namespace-scoped RoleBinding
	if err := r.provisionRoleBinding(ctx, rb); err != nil {
		return fmt.Errorf("failed to provision rolebinding: %w", err)
	}

	// Create cluster-scoped ClusterRole for Tekton Triggers resources
	if err := r.provisionClusterRole(ctx, rb); err != nil {
		return fmt.Errorf("failed to provision cluster role: %w", err)
	}

	// Create cluster-scoped ClusterRoleBinding
	if err := r.provisionClusterRoleBinding(ctx, rb); err != nil {
		return fmt.Errorf("failed to provision cluster rolebinding: %w", err)
	}

	return nil
}


// provisionRole creates or updates the tenant Role
func (r *RepoBindingReconciler) provisionRole(ctx context.Context, rb *platformv1alpha1.RepoBinding, profile string) error {
	role := r.buildRole(rb.Spec.TenantName, profile)

	// Try to get existing role
	existingRole := &rbacv1.Role{}
	err := r.Get(ctx, client.ObjectKey{Name: "pipeline-runner", Namespace: rb.Spec.TenantName}, existingRole)
	if err != nil {
		if errors.IsNotFound(err) {
			// Create new role
			r.Log.Info("Creating role", "namespace", rb.Spec.TenantName, "profile", profile)
			if err := r.Create(ctx, role); err != nil {
				return fmt.Errorf("failed to create role: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to get role: %w", err)
	}

	// Role exists, update rules
	r.Log.Info("Updating role", "namespace", rb.Spec.TenantName, "profile", profile)
	existingRole.Rules = role.Rules
	if err := r.Update(ctx, existingRole); err != nil {
		return fmt.Errorf("failed to update role: %w", err)
	}

	return nil
}

// provisionRoleBinding creates or updates the tenant RoleBinding
func (r *RepoBindingReconciler) provisionRoleBinding(ctx context.Context, rb *platformv1alpha1.RepoBinding) error {
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pipeline-runner",
			Namespace: rb.Spec.TenantName,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     "pipeline-runner",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "pipeline-runner",
				Namespace: rb.Spec.TenantName,
			},
		},
	}

	// Try to get existing rolebinding
	existingRB := &rbacv1.RoleBinding{}
	err := r.Get(ctx, client.ObjectKey{Name: "pipeline-runner", Namespace: rb.Spec.TenantName}, existingRB)
	if err != nil {
		if errors.IsNotFound(err) {
			// Create new rolebinding
			r.Log.Info("Creating rolebinding", "namespace", rb.Spec.TenantName)
			if err := r.Create(ctx, roleBinding); err != nil {
				return fmt.Errorf("failed to create rolebinding: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to get rolebinding: %w", err)
	}

	// RoleBinding exists, nothing to update
	r.Log.Info("RoleBinding already exists", "namespace", rb.Spec.TenantName)
	return nil
}

// provisionClusterRole creates or updates the tenant ClusterRole for cluster-scoped Tekton Triggers resources
func (r *RepoBindingReconciler) provisionClusterRole(ctx context.Context, rb *platformv1alpha1.RepoBinding) error {
	clusterRoleName := fmt.Sprintf("pipeline-runner-%s", rb.Spec.TenantName)
	
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleName,
			Labels: map[string]string{
				"platform.arbiter.io/tenant":     rb.Spec.TenantName,
				"platform.arbiter.io/managed-by": "onboarding-controller",
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"triggers.tekton.dev"},
				Resources: []string{"clusterinterceptors", "clustertriggerbindings"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}

	existingCR := &rbacv1.ClusterRole{}
	err := r.Get(ctx, client.ObjectKey{Name: clusterRoleName}, existingCR)
	if err != nil {
		if errors.IsNotFound(err) {
			r.Log.Info("Creating ClusterRole for Tekton Triggers", "clusterRole", clusterRoleName)
			if err := r.Create(ctx, clusterRole); err != nil {
				return fmt.Errorf("failed to create ClusterRole: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to get ClusterRole: %w", err)
	}

	r.Log.Info("Updating ClusterRole for Tekton Triggers", "clusterRole", clusterRoleName)
	existingCR.Rules = clusterRole.Rules
	existingCR.Labels = clusterRole.Labels
	if err := r.Update(ctx, existingCR); err != nil {
		return fmt.Errorf("failed to update ClusterRole: %w", err)
	}

	return nil
}

// provisionClusterRoleBinding creates or updates the tenant ClusterRoleBinding
func (r *RepoBindingReconciler) provisionClusterRoleBinding(ctx context.Context, rb *platformv1alpha1.RepoBinding) error {
	clusterRoleName := fmt.Sprintf("pipeline-runner-%s", rb.Spec.TenantName)
	clusterRoleBindingName := fmt.Sprintf("pipeline-runner-%s", rb.Spec.TenantName)
	
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleBindingName,
			Labels: map[string]string{
				"platform.arbiter.io/tenant":     rb.Spec.TenantName,
				"platform.arbiter.io/managed-by": "onboarding-controller",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     clusterRoleName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "pipeline-runner",
				Namespace: rb.Spec.TenantName,
			},
		},
	}

	existingCRB := &rbacv1.ClusterRoleBinding{}
	err := r.Get(ctx, client.ObjectKey{Name: clusterRoleBindingName}, existingCRB)
	if err != nil {
		if errors.IsNotFound(err) {
			r.Log.Info("Creating ClusterRoleBinding for Tekton Triggers", "clusterRoleBinding", clusterRoleBindingName)
			if err := r.Create(ctx, clusterRoleBinding); err != nil {
				return fmt.Errorf("failed to create ClusterRoleBinding: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to get ClusterRoleBinding: %w", err)
	}

	r.Log.Info("Updating ClusterRoleBinding for Tekton Triggers", "clusterRoleBinding", clusterRoleBindingName)
	existingCRB.RoleRef = clusterRoleBinding.RoleRef
	existingCRB.Subjects = clusterRoleBinding.Subjects
	existingCRB.Labels = clusterRoleBinding.Labels
	if err := r.Update(ctx, existingCRB); err != nil {
		return fmt.Errorf("failed to update ClusterRoleBinding: %w", err)
	}

	return nil
}

// buildRole constructs a Role based on the permission profile
func (r *RepoBindingReconciler) buildRole(namespace, profile string) *rbacv1.Role {
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pipeline-runner",
			Namespace: namespace,
		},
	}

	// Standard rules (common to both profiles)
	standardRules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{""},
			Resources: []string{"pods", "pods/log"},
			Verbs:     []string{"get", "list", "create", "update", "delete", "watch"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"configmaps", "secrets"},
			Verbs:     []string{"get", "list", "create", "update", "delete"},
		},
		{
			APIGroups: []string{"tekton.dev"},
			Resources: []string{"pipelineruns", "taskruns"},
			Verbs:     []string{"get", "list", "create", "watch"},
		},
		{
			APIGroups: []string{"triggers.tekton.dev"},
			Resources: []string{"eventlisteners", "triggers", "triggerbindings", "triggertemplates", "interceptors"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"persistentvolumeclaims"},
			Verbs:     []string{"get", "list"},
		},
	}

	if profile == "elevated" {
		// Add elevated rules
		elevatedRules := []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"persistentvolumeclaims"},
				Verbs:     []string{"get", "list", "create", "update", "delete"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"services"},
				Verbs:     []string{"get", "list", "create", "update", "delete"},
			},
			{
				APIGroups: []string{"apps"},
				Resources: []string{"deployments"},
				Verbs:     []string{"get", "list", "create", "update", "delete"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"events"},
				Verbs:     []string{"get", "list", "watch"},
			},
		}
		// Keep all standard rules except the PVC rule (which gets replaced), then add elevated rules
		role.Rules = append(standardRules[:4], elevatedRules...)
	} else {
		role.Rules = standardRules
	}

	return role
}


// provisionResourceLimits creates or updates ResourceQuota and LimitRange
func (r *RepoBindingReconciler) provisionResourceLimits(ctx context.Context, rb *platformv1alpha1.RepoBinding) error {
	// Create ResourceQuota
	if err := r.provisionResourceQuota(ctx, rb); err != nil {
		return fmt.Errorf("failed to provision resource quota: %w", err)
	}

	// Create LimitRange
	if err := r.provisionLimitRange(ctx, rb); err != nil {
		return fmt.Errorf("failed to provision limit range: %w", err)
	}

	return nil
}

// provisionResourceQuota creates or updates the tenant ResourceQuota
func (r *RepoBindingReconciler) provisionResourceQuota(ctx context.Context, rb *platformv1alpha1.RepoBinding) error {
	resourceQuota := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant-quota",
			Namespace: rb.Spec.TenantName,
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: corev1.ResourceList{
				"requests.cpu":              resource.MustParse("4"),
				"limits.cpu":                resource.MustParse("8"),
				"requests.memory":           resource.MustParse("8Gi"),
				"limits.memory":             resource.MustParse("16Gi"),
				"persistentvolumeclaims":    resource.MustParse("5"),
				"pods":                      resource.MustParse("20"),
			},
		},
	}

	// Try to get existing resource quota
	existingRQ := &corev1.ResourceQuota{}
	err := r.Get(ctx, client.ObjectKey{Name: "tenant-quota", Namespace: rb.Spec.TenantName}, existingRQ)
	if err != nil {
		if errors.IsNotFound(err) {
			// Create new resource quota
			r.Log.Info("Creating resource quota", "namespace", rb.Spec.TenantName)
			if err := r.Create(ctx, resourceQuota); err != nil {
				return fmt.Errorf("failed to create resource quota: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to get resource quota: %w", err)
	}

	// Resource quota exists, update if needed
	r.Log.Info("Updating resource quota", "namespace", rb.Spec.TenantName)
	existingRQ.Spec.Hard = resourceQuota.Spec.Hard
	if err := r.Update(ctx, existingRQ); err != nil {
		return fmt.Errorf("failed to update resource quota: %w", err)
	}

	return nil
}

// provisionLimitRange creates or updates the tenant LimitRange
func (r *RepoBindingReconciler) provisionLimitRange(ctx context.Context, rb *platformv1alpha1.RepoBinding) error {
	limitRange := &corev1.LimitRange{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant-limits",
			Namespace: rb.Spec.TenantName,
		},
		Spec: corev1.LimitRangeSpec{
			Limits: []corev1.LimitRangeItem{
				{
					Type: corev1.LimitTypeContainer,
					Default: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
					DefaultRequest: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
					Max: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("2"),
						corev1.ResourceMemory: resource.MustParse("4Gi"),
					},
					Min: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("50m"),
						corev1.ResourceMemory: resource.MustParse("64Mi"),
					},
				},
			},
		},
	}

	// Try to get existing limit range
	existingLR := &corev1.LimitRange{}
	err := r.Get(ctx, client.ObjectKey{Name: "tenant-limits", Namespace: rb.Spec.TenantName}, existingLR)
	if err != nil {
		if errors.IsNotFound(err) {
			// Create new limit range
			r.Log.Info("Creating limit range", "namespace", rb.Spec.TenantName)
			if err := r.Create(ctx, limitRange); err != nil {
				return fmt.Errorf("failed to create limit range: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to get limit range: %w", err)
	}

	// Limit range exists, update if needed
	r.Log.Info("Updating limit range", "namespace", rb.Spec.TenantName)
	existingLR.Spec.Limits = limitRange.Spec.Limits
	if err := r.Update(ctx, existingLR); err != nil {
		return fmt.Errorf("failed to update limit range: %w", err)
	}

	return nil
}


// provisionNetworkPolicy creates or updates the tenant NetworkPolicy
func (r *RepoBindingReconciler) provisionNetworkPolicy(ctx context.Context, rb *platformv1alpha1.RepoBinding) error {
	networkPolicy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant-isolation",
			Namespace: rb.Spec.TenantName,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							PodSelector: &metav1.LabelSelector{},
						},
					},
				},
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				// Allow to same namespace
				{
					To: []networkingv1.NetworkPolicyPeer{
						{
							PodSelector: &metav1.LabelSelector{},
						},
					},
				},
				// Allow DNS queries to kube-system
				{
					To: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"kubernetes.io/metadata.name": "kube-system",
								},
							},
						},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: &[]corev1.Protocol{corev1.ProtocolUDP}[0],
							Port:     &intstr.IntOrString{Type: intstr.Int, IntVal: 53},
						},
					},
				},
				// Allow internet egress (excluding private IP ranges)
				{
					To: []networkingv1.NetworkPolicyPeer{
						{
							IPBlock: &networkingv1.IPBlock{
								CIDR: "0.0.0.0/0",
								Except: []string{
									"10.0.0.0/8",
									"172.16.0.0/12",
									"192.168.0.0/16",
								},
							},
						},
					},
				},
			},
		},
	}

	// Try to get existing network policy
	existingNP := &networkingv1.NetworkPolicy{}
	err := r.Get(ctx, client.ObjectKey{Name: "tenant-isolation", Namespace: rb.Spec.TenantName}, existingNP)
	if err != nil {
		if errors.IsNotFound(err) {
			// Create new network policy
			r.Log.Info("Creating network policy", "namespace", rb.Spec.TenantName)
			if err := r.Create(ctx, networkPolicy); err != nil {
				return fmt.Errorf("failed to create network policy: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to get network policy: %w", err)
	}

	// Network policy exists, update if needed
	r.Log.Info("Updating network policy", "namespace", rb.Spec.TenantName)
	existingNP.Spec = networkPolicy.Spec
	if err := r.Update(ctx, existingNP); err != nil {
		return fmt.Errorf("failed to update network policy: %w", err)
	}

	return nil
}


// provisionTerraformBackendSecret creates or updates the Terraform backend secret
func (r *RepoBindingReconciler) provisionTerraformBackendSecret(ctx context.Context, rb *platformv1alpha1.RepoBinding) error {
	backendConfig := fmt.Sprintf(`terraform {
  backend "kubernetes" {
    secret_suffix    = "%s"
    namespace        = "%s"
    in_cluster_config = true
  }
}`, rb.Spec.TenantName, rb.Spec.TenantName)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "terraform-backend-config",
			Namespace: rb.Spec.TenantName,
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"backend.tf": backendConfig,
		},
	}

	// Try to get existing secret
	existingSecret := &corev1.Secret{}
	err := r.Get(ctx, client.ObjectKey{Name: "terraform-backend-config", Namespace: rb.Spec.TenantName}, existingSecret)
	if err != nil {
		if errors.IsNotFound(err) {
			// Create new secret
			r.Log.Info("Creating Terraform backend secret", "namespace", rb.Spec.TenantName)
			if err := r.Create(ctx, secret); err != nil {
				return fmt.Errorf("failed to create Terraform backend secret: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to get Terraform backend secret: %w", err)
	}

	// Secret exists, update if needed
	r.Log.Info("Updating Terraform backend secret", "namespace", rb.Spec.TenantName)
	existingSecret.StringData = secret.StringData
	if err := r.Update(ctx, existingSecret); err != nil {
		return fmt.Errorf("failed to update Terraform backend secret: %w", err)
	}

	return nil
}


// provisionTriggerBinding creates or updates the TriggerBinding for GitHub webhooks
func (r *RepoBindingReconciler) provisionTriggerBinding(ctx context.Context, rb *platformv1alpha1.RepoBinding) error {
	triggerBinding := &triggersv1beta1.TriggerBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "triggers.tekton.dev/v1beta1",
			Kind:       "TriggerBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "github-push-binding",
			Namespace: rb.Spec.TenantName,
			Labels: map[string]string{
				"platform.arbiter.io/tenant":     rb.Spec.TenantName,
				"platform.arbiter.io/managed-by": "onboarding-controller",
			},
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

	if err := r.Client.Patch(ctx, triggerBinding, client.Apply, client.ForceOwnership, client.FieldOwner("onboarding-controller")); err != nil {
		return fmt.Errorf("failed to apply TriggerBinding: %w", err)
	}

	return nil
}

// provisionTriggerTemplate creates or updates the TriggerTemplate for pipeline execution
func (r *RepoBindingReconciler) provisionTriggerTemplate(ctx context.Context, rb *platformv1alpha1.RepoBinding) error {
	// Find the pipeline's namespace by searching across all namespaces
	pipelineNamespace, err := r.findPipelineNamespace(ctx, rb.Spec.PipelineName)
	if err != nil {
		return fmt.Errorf("failed to find pipeline %q: %w", rb.Spec.PipelineName, err)
	}

	r.Log.Info("Found pipeline in namespace", "pipeline", rb.Spec.PipelineName, "namespace", pipelineNamespace)

	// Define the TriggerTemplate GVK
	triggerTemplateGVK := schema.GroupVersionKind{
		Group:   "triggers.tekton.dev",
		Version: "v1beta1",
		Kind:    "TriggerTemplate",
	}
	
	// Build the TriggerTemplate spec
	triggerTemplate := &unstructured.Unstructured{}
	triggerTemplate.SetGroupVersionKind(triggerTemplateGVK)
	triggerTemplate.SetName(fmt.Sprintf("%s-trigger-template", rb.Spec.TenantName))
	triggerTemplate.SetNamespace(rb.Spec.TenantName)
	triggerTemplate.SetLabels(map[string]string{
		"platform.arbiter.io/tenant":     rb.Spec.TenantName,
		"platform.arbiter.io/managed-by": "onboarding-controller",
	})
	
	// Set the spec
	spec := map[string]interface{}{
		"params": []interface{}{
			map[string]interface{}{
				"name": "git-url",
			},
			map[string]interface{}{
				"name": "git-revision",
			},
		},
		"resourcetemplates": []interface{}{
			map[string]interface{}{
				"apiVersion": "tekton.dev/v1beta1",
				"kind":       "PipelineRun",
				"metadata": map[string]interface{}{
					"generateName": fmt.Sprintf("%s-run-", rb.Spec.TenantName),
				},
				"spec": map[string]interface{}{
					"pipelineRef": map[string]interface{}{
						"name": rb.Spec.PipelineName,
						"resolver": "cluster",
						"params": []interface{}{
							map[string]interface{}{
								"name":  "name",
								"value": rb.Spec.PipelineName,
							},
							map[string]interface{}{
								"name":  "namespace", 
								"value": pipelineNamespace,
							},
						},
					},
					"params": []interface{}{
						map[string]interface{}{
							"name":  "git-url",
							"value": "$(tt.params.git-url)",
						},
						map[string]interface{}{
							"name":  "git-revision",
							"value": "$(tt.params.git-revision)",
						},
					},
					"workspaces": []interface{}{
						map[string]interface{}{
							"name": "source",
							"volumeClaimTemplate": map[string]interface{}{
								"spec": map[string]interface{}{
									"accessModes": []interface{}{"ReadWriteOnce"},
									"resources": map[string]interface{}{
										"requests": map[string]interface{}{
											"storage": "1Gi",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	
	if err := unstructured.SetNestedMap(triggerTemplate.Object, spec, "spec"); err != nil {
		return fmt.Errorf("failed to set TriggerTemplate spec: %w", err)
	}
	
	// Apply the TriggerTemplate
	if err := r.Client.Patch(ctx, triggerTemplate, client.Apply, client.ForceOwnership, client.FieldOwner("onboarding-controller")); err != nil {
		return fmt.Errorf("failed to apply TriggerTemplate: %w", err)
	}
	
	return nil
}

// stringPtr returns a pointer to a string
func stringPtr(s string) *string {
	return &s
}


// AllowlistEntry represents a repository entry in the allowlist
type AllowlistEntry struct {
	Org              string `yaml:"org"`
	Name             string `yaml:"name"`
	Tenant           string `yaml:"tenant"`
	Enabled          bool   `yaml:"enabled"`
	WebhookSecretRef string `yaml:"webhookSecretRef,omitempty"`
}

// Allowlist represents the structure of the allowlist YAML
type Allowlist struct {
	Repos []AllowlistEntry `yaml:"repos"`
}

// provisionAllowlistEntry adds the repository to the Lighthouse allowlist
func (r *RepoBindingReconciler) provisionAllowlistEntry(ctx context.Context, rb *platformv1alpha1.RepoBinding) error {
	// Get the allowlist ConfigMap from pipeline-system namespace
	configMap := &corev1.ConfigMap{}
	err := r.Get(ctx, client.ObjectKey{Name: "repo-allowlist", Namespace: "pipeline-system"}, configMap)
	if err != nil {
		return fmt.Errorf("failed to get allowlist ConfigMap: %w", err)
	}

	// Parse the current allowlist
	allowlistYAML, ok := configMap.Data["allowlist.yaml"]
	if !ok {
		return fmt.Errorf("allowlist.yaml not found in ConfigMap")
	}

	// Unmarshal the YAML into the Allowlist struct
	var allowlist Allowlist
	if err := yaml.Unmarshal([]byte(allowlistYAML), &allowlist); err != nil {
		return fmt.Errorf("failed to parse allowlist YAML: %w", err)
	}

	// Check if repository is already in the allowlist
	webhookSecretRef := fmt.Sprintf("webhook-%s", rb.Spec.TenantName)
	
	for i, entry := range allowlist.Repos {
		if entry.Org == rb.Spec.RepoOrg && entry.Name == rb.Spec.RepoName {
			// Repository exists, update tenant mapping and webhook secret ref if different
			updated := false
			if entry.Tenant != rb.Spec.TenantName {
				r.Log.Info("Updating tenant mapping for repository", 
					"org", rb.Spec.RepoOrg, 
					"repo", rb.Spec.RepoName,
					"oldTenant", entry.Tenant,
					"newTenant", rb.Spec.TenantName)
				allowlist.Repos[i].Tenant = rb.Spec.TenantName
				updated = true
			}
			if entry.WebhookSecretRef != webhookSecretRef {
				r.Log.Info("Updating webhook secret reference for repository",
					"org", rb.Spec.RepoOrg,
					"repo", rb.Spec.RepoName,
					"webhookSecretRef", webhookSecretRef)
				allowlist.Repos[i].WebhookSecretRef = webhookSecretRef
				updated = true
			}
			allowlist.Repos[i].Enabled = true
			
			if !updated {
				r.Log.Info("Repository already in allowlist with correct configuration", 
					"org", rb.Spec.RepoOrg, 
					"repo", rb.Spec.RepoName,
					"tenant", rb.Spec.TenantName)
				return nil
			}
			
			// Marshal back to YAML and update ConfigMap
			updatedYAML, err := yaml.Marshal(&allowlist)
			if err != nil {
				return fmt.Errorf("failed to marshal updated allowlist: %w", err)
			}
			
			configMap.Data["allowlist.yaml"] = string(updatedYAML)
			if err := r.Update(ctx, configMap); err != nil {
				return fmt.Errorf("failed to update allowlist ConfigMap: %w", err)
			}
			
			return nil
		}
	}

	// Repository not found, add new entry
	newEntry := AllowlistEntry{
		Org:              rb.Spec.RepoOrg,
		Name:             rb.Spec.RepoName,
		Tenant:           rb.Spec.TenantName,
		Enabled:          true,
		WebhookSecretRef: webhookSecretRef,
	}
	allowlist.Repos = append(allowlist.Repos, newEntry)

	// Marshal back to YAML
	updatedYAML, err := yaml.Marshal(&allowlist)
	if err != nil {
		return fmt.Errorf("failed to marshal updated allowlist: %w", err)
	}

	// Update the ConfigMap
	configMap.Data["allowlist.yaml"] = string(updatedYAML)
	if err := r.Update(ctx, configMap); err != nil {
		return fmt.Errorf("failed to update allowlist ConfigMap: %w", err)
	}

	r.Log.Info("Added repository to allowlist", 
		"org", rb.Spec.RepoOrg, 
		"repo", rb.Spec.RepoName,
		"tenant", rb.Spec.TenantName)

	return nil
}

// updateRepoBindingStatusWithWebhookInfo updates the RepoBinding status with webhook configuration details
func (r *RepoBindingReconciler) updateRepoBindingStatusWithWebhookInfo(ctx context.Context, rb *platformv1alpha1.RepoBinding) error {
	// First, try to get webhook secret from organization namespace
	orgNamespace := fmt.Sprintf("org-%s", rb.Spec.AphexOrg)
	orgSecret := &corev1.Secret{}
	err := r.Get(ctx, client.ObjectKey{Name: "github-webhook-secret", Namespace: orgNamespace}, orgSecret)
	if err != nil {
		return fmt.Errorf("failed to get webhook secret from organization namespace %s: %w", orgNamespace, err)
	}
	
	// Copy the webhook secret to the pipeline namespace if it doesn't exist
	pipelineSecret := &corev1.Secret{}
	err = r.Get(ctx, client.ObjectKey{Name: "github-webhook-secret", Namespace: rb.Spec.TenantName}, pipelineSecret)
	if err != nil {
		if errors.IsNotFound(err) {
			// Create a copy of the secret in the pipeline namespace
			pipelineSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "github-webhook-secret",
					Namespace: rb.Spec.TenantName,
					Labels: map[string]string{
						"platform.arbiter.io/managed-by": "repobinding-controller",
						"platform.arbiter.io/organization": rb.Spec.RepoOrg,
					},
				},
				Type: orgSecret.Type,
				Data: orgSecret.Data,
			}
			
			if err := r.Create(ctx, pipelineSecret); err != nil {
				return fmt.Errorf("failed to copy webhook secret to pipeline namespace: %w", err)
			}
		} else {
			return fmt.Errorf("failed to check webhook secret in pipeline namespace: %w", err)
		}
	}
	
	webhookSecret, ok := pipelineSecret.Data["secret"]
	if !ok {
		return fmt.Errorf("webhook secret missing 'secret' key")
	}
	
	// Use organization-specific webhook URL
	rb.Status.WebhookURL = fmt.Sprintf("https://%s.arbiter-dev.com", rb.Spec.RepoOrg)
	
	// Update RepoBinding status with webhook information
	rb.Status.WebhookSecret = string(webhookSecret)
	
	// Build configuration instructions message
	instructions := fmt.Sprintf(`Registration successful!

Next steps - Configure GitHub webhook:
1. Go to: https://github.com/%s/%s/settings/hooks/new
2. Payload URL: %s
3. Content type: application/json
4. Secret: %s
5. Events: Push events, Pull request events
6. Active: ✓
7. Click "Add webhook"`, 
		rb.Spec.RepoOrg, 
		rb.Spec.RepoName, 
		rb.Status.WebhookURL,
		rb.Status.WebhookSecret)
	
	rb.Status.Message = instructions
	
	r.Log.Info("Updated RepoBinding status with webhook information", 
		"tenant", rb.Spec.TenantName,
		"webhookURL", rb.Status.WebhookURL)
	
	return nil
}

// findPipelineNamespace searches for a pipeline by name across all accessible namespaces
func (r *RepoBindingReconciler) findPipelineNamespace(ctx context.Context, pipelineName string) (string, error) {
	// List all namespaces
	namespaceList := &corev1.NamespaceList{}
	if err := r.List(ctx, namespaceList); err != nil {
		return "", fmt.Errorf("failed to list namespaces: %w", err)
	}

	// Search each namespace for the pipeline
	for _, ns := range namespaceList.Items {
		namespace := ns.Name
		
		// Try to get the pipeline in this namespace using unstructured client
		pipeline := &unstructured.Unstructured{}
		pipeline.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "tekton.dev",
			Version: "v1",
			Kind:    "Pipeline",
		})
		
		err := r.Get(ctx, client.ObjectKey{
			Name:      pipelineName,
			Namespace: namespace,
		}, pipeline)
		
		if err == nil {
			// Found it!
			return namespace, nil
		}
	}

	return "", fmt.Errorf("pipeline %q not found in any accessible namespace", pipelineName)
}
