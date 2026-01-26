package controllers

import (
	"bytes"
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8syaml "sigs.k8s.io/yaml"

	platformv1alpha1 "github.com/bdchatham/AphexPlatformInfrastructure/platform/platform-controller/controller/api/v1alpha1"
	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	triggersv1beta1 "github.com/tektoncd/triggers/pkg/apis/triggers/v1beta1"
)

// provisionNamespace creates or updates the pipeline namespace
func (r *RepoBindingReconciler) provisionNamespace(ctx context.Context, rb *platformv1alpha1.RepoBinding) error {
	repoLabel := fmt.Sprintf("%s-%s", rb.Spec.RepoOrg, rb.Spec.RepoName)

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: rb.Spec.PipelineName,
			Labels: map[string]string{
				"platform.aphex/pipeline":   rb.Spec.PipelineName,
				"platform.aphex/repo":       repoLabel,
				"platform.aphex/managed-by": "platform-controller",
				"aphex.dev/org":             rb.Spec.AphexOrg,
			},
		},
	}

	existingNs := &corev1.Namespace{}
	err := r.Get(ctx, client.ObjectKey{Name: rb.Spec.PipelineName}, existingNs)
	if err != nil {
		if errors.IsNotFound(err) {
			r.Log.Info("Creating pipeline namespace", "namespace", rb.Spec.PipelineName)
			if err := r.Create(ctx, namespace); err != nil {
				return fmt.Errorf("failed to create namespace: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to get namespace: %w", err)
	}

	r.Log.Info("Namespace already exists, updating labels", "namespace", rb.Spec.PipelineName)
	existingNs.Labels = namespace.Labels
	if err := r.Update(ctx, existingNs); err != nil {
		return fmt.Errorf("failed to update namespace labels: %w", err)
	}

	return nil
}

// provisionPipeline creates the Tekton Pipeline resource from the spec
func (r *RepoBindingReconciler) provisionPipeline(ctx context.Context, rb *platformv1alpha1.RepoBinding) error {
	// Decode YAML using Kubernetes decoder (handles JSON tags properly)
	pipeline := &tektonv1.Pipeline{}
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(rb.Spec.PipelineSpec)), 4096)
	if err := decoder.Decode(pipeline); err != nil {
		r.Log.Error(err, "Failed to parse pipeline YAML")
		return fmt.Errorf("failed to parse pipeline YAML: %w", err)
	}

	r.Log.Info("Parsed pipeline", "originalName", pipeline.Name, "kind", pipeline.Kind, "apiVersion", pipeline.APIVersion)

	// Set name and namespace from RepoBinding (override whatever is in the YAML)
	pipeline.Name = rb.Spec.PipelineName
	pipeline.Namespace = rb.Spec.PipelineName

	// Add labels for tracking
	if pipeline.Labels == nil {
		pipeline.Labels = make(map[string]string)
	}
	pipeline.Labels["platform.aphex/pipeline"] = rb.Spec.PipelineName
	pipeline.Labels["platform.aphex/managed-by"] = "platform-controller"

	// Check if pipeline already exists
	existingPipeline := &tektonv1.Pipeline{}
	err := r.Get(ctx, client.ObjectKey{Name: rb.Spec.PipelineName, Namespace: rb.Spec.PipelineName}, existingPipeline)

	if err != nil {
		if errors.IsNotFound(err) {
			r.Log.Info("Creating Pipeline resource", "name", rb.Spec.PipelineName, "namespace", rb.Spec.PipelineName)
			if err := r.Create(ctx, pipeline); err != nil {
				return fmt.Errorf("failed to create Pipeline: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to get Pipeline: %w", err)
	}

	r.Log.Info("Pipeline already exists, updating", "name", rb.Spec.PipelineName, "namespace", rb.Spec.PipelineName)
	pipeline.ResourceVersion = existingPipeline.ResourceVersion
	if err := r.Update(ctx, pipeline); err != nil {
		return fmt.Errorf("failed to update Pipeline: %w", err)
	}

	return nil
}

// provisionServiceAccount creates or updates the tenant service account
func (r *RepoBindingReconciler) provisionServiceAccount(ctx context.Context, rb *platformv1alpha1.RepoBinding) error {
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pipeline-runner",
			Namespace: rb.Spec.PipelineName,
		},
	}

	existingSA := &corev1.ServiceAccount{}
	err := r.Get(ctx, client.ObjectKey{Name: "pipeline-runner", Namespace: rb.Spec.PipelineName}, existingSA)
	if err != nil {
		if errors.IsNotFound(err) {
			r.Log.Info("Creating service account", "namespace", rb.Spec.PipelineName, "name", "pipeline-runner")
			if err := r.Create(ctx, serviceAccount); err != nil {
				return fmt.Errorf("failed to create service account: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to get service account: %w", err)
	}

	r.Log.Info("Service account already exists", "namespace", rb.Spec.PipelineName, "name", "pipeline-runner")
	return nil
}

// provisionRBAC creates or updates the pipeline RBAC (Role, RoleBinding, ClusterRole, ClusterRoleBinding)
func (r *RepoBindingReconciler) provisionRBAC(ctx context.Context, rb *platformv1alpha1.RepoBinding) error {
	// Use standard profile for all pipelines
	profile := "standard"

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

	// Create ArgoCD RoleBinding in argocd namespace
	if err := r.provisionArgoCDRoleBinding(ctx, rb); err != nil {
		return fmt.Errorf("failed to provision argocd rolebinding: %w", err)
	}

	// Create ArgoCD AppProject for pipeline isolation
	if err := r.provisionArgoCDAppProject(ctx, rb); err != nil {
		return fmt.Errorf("failed to provision argocd appproject: %w", err)
	}

	return nil
}

// provisionRole creates or updates the pipeline Role
func (r *RepoBindingReconciler) provisionRole(ctx context.Context, rb *platformv1alpha1.RepoBinding, profile string) error {
	role := r.buildRole(rb.Spec.PipelineName, profile)

	existingRole := &rbacv1.Role{}
	err := r.Get(ctx, client.ObjectKey{Name: "pipeline-runner", Namespace: rb.Spec.PipelineName}, existingRole)
	if err != nil {
		if errors.IsNotFound(err) {
			r.Log.Info("Creating role", "namespace", rb.Spec.PipelineName, "profile", profile)
			if err := r.Create(ctx, role); err != nil {
				return fmt.Errorf("failed to create role: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to get role: %w", err)
	}

	r.Log.Info("Updating role", "namespace", rb.Spec.PipelineName, "profile", profile)
	existingRole.Rules = role.Rules
	if err := r.Update(ctx, existingRole); err != nil {
		return fmt.Errorf("failed to update role: %w", err)
	}

	return nil
}

// provisionRoleBinding creates or updates the pipeline RoleBinding
func (r *RepoBindingReconciler) provisionRoleBinding(ctx context.Context, rb *platformv1alpha1.RepoBinding) error {
	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pipeline-runner",
			Namespace: rb.Spec.PipelineName,
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
				Namespace: rb.Spec.PipelineName,
			},
		},
	}

	existingRB := &rbacv1.RoleBinding{}
	err := r.Get(ctx, client.ObjectKey{Name: "pipeline-runner", Namespace: rb.Spec.PipelineName}, existingRB)
	if err != nil {
		if errors.IsNotFound(err) {
			r.Log.Info("Creating rolebinding", "namespace", rb.Spec.PipelineName)
			if err := r.Create(ctx, roleBinding); err != nil {
				return fmt.Errorf("failed to create rolebinding: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to get rolebinding: %w", err)
	}

	r.Log.Info("RoleBinding already exists", "namespace", rb.Spec.PipelineName)
	return nil
}

// provisionClusterRole creates or updates the pipeline ClusterRole for cluster-scoped Tekton Triggers resources
func (r *RepoBindingReconciler) provisionClusterRole(ctx context.Context, rb *platformv1alpha1.RepoBinding) error {
	clusterRoleName := fmt.Sprintf("pipeline-runner-%s", rb.Spec.PipelineName)

	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleName,
			Labels: map[string]string{
				"platform.aphex/pipeline":   rb.Spec.PipelineName,
				"platform.aphex/managed-by": "platform-controller",
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

// provisionClusterRoleBinding creates or updates the pipeline ClusterRoleBinding
func (r *RepoBindingReconciler) provisionClusterRoleBinding(ctx context.Context, rb *platformv1alpha1.RepoBinding) error {
	clusterRoleName := fmt.Sprintf("pipeline-runner-%s", rb.Spec.PipelineName)
	clusterRoleBindingName := fmt.Sprintf("pipeline-runner-%s", rb.Spec.PipelineName)

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleBindingName,
			Labels: map[string]string{
				"platform.aphex/pipeline":   rb.Spec.PipelineName,
				"platform.aphex/managed-by": "platform-controller",
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
				Namespace: rb.Spec.PipelineName,
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

// provisionArgoCDRoleBinding creates a RoleBinding in the argocd namespace
// granting the pipeline's service account permission to manage ArgoCD Applications
func (r *RepoBindingReconciler) provisionArgoCDRoleBinding(ctx context.Context, rb *platformv1alpha1.RepoBinding) error {
	roleBindingName := fmt.Sprintf("%s-argocd-access", rb.Spec.PipelineName)

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleBindingName,
			Namespace: "argocd",
			Labels: map[string]string{
				"platform.aphex/pipeline":   rb.Spec.PipelineName,
				"platform.aphex/managed-by": "platform-controller",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "argocd-application-deployer",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "pipeline-runner",
				Namespace: rb.Spec.PipelineName,
			},
		},
	}

	existingRB := &rbacv1.RoleBinding{}
	err := r.Get(ctx, client.ObjectKey{Name: roleBindingName, Namespace: "argocd"}, existingRB)
	if err != nil {
		if errors.IsNotFound(err) {
			r.Log.Info("Creating ArgoCD RoleBinding", "name", roleBindingName, "pipeline", rb.Spec.PipelineName)
			if err := r.Create(ctx, roleBinding); err != nil {
				return fmt.Errorf("failed to create ArgoCD RoleBinding: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to get ArgoCD RoleBinding: %w", err)
	}

	r.Log.Info("Updating ArgoCD RoleBinding", "name", roleBindingName, "pipeline", rb.Spec.PipelineName)
	existingRB.Subjects = roleBinding.Subjects
	existingRB.Labels = roleBinding.Labels
	if err := r.Update(ctx, existingRB); err != nil {
		return fmt.Errorf("failed to update ArgoCD RoleBinding: %w", err)
	}

	return nil
}

// provisionArgoCDAppProject creates an AppProject scoped to the pipeline's allowed destinations.
//
// NOTE: We use unstructured here instead of importing github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1
// because ArgoCD's API types have deep transitive dependencies on k8s.io/kubernetes which causes
// version conflicts with our k8s.io/api version. The unstructured approach avoids this dependency
// hell while still providing type-safe interaction with the ArgoCD CRD.
func (r *RepoBindingReconciler) provisionArgoCDAppProject(ctx context.Context, rb *platformv1alpha1.RepoBinding) error {
	projectName := rb.Spec.PipelineName

	appProject := &unstructured.Unstructured{}
	appProject.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "argoproj.io",
		Version: "v1alpha1",
		Kind:    "AppProject",
	})
	appProject.SetName(projectName)
	appProject.SetNamespace("argocd")
	appProject.SetLabels(map[string]string{
		"platform.aphex/pipeline":   rb.Spec.PipelineName,
		"platform.aphex/managed-by": "platform-controller",
	})

	spec := map[string]interface{}{
		"description": fmt.Sprintf("Project for %s pipeline", rb.Spec.PipelineName),
		"destinations": []interface{}{
			map[string]interface{}{
				"server":    "https://kubernetes.default.svc",
				"namespace": rb.Spec.PipelineName,
			},
			map[string]interface{}{
				"server":    "https://kubernetes.default.svc",
				"namespace": fmt.Sprintf("%s-*", rb.Spec.PipelineName),
			},
		},
		"sourceRepos": []interface{}{
			fmt.Sprintf("https://github.com/%s/%s", rb.Spec.RepoOrg, rb.Spec.RepoName),
			fmt.Sprintf("https://github.com/%s/%s.git", rb.Spec.RepoOrg, rb.Spec.RepoName),
		},
		"clusterResourceWhitelist": []interface{}{},
		"namespaceResourceWhitelist": []interface{}{
			map[string]interface{}{"group": "*", "kind": "*"},
		},
	}
	appProject.Object["spec"] = spec

	existingProject := &unstructured.Unstructured{}
	existingProject.SetGroupVersionKind(appProject.GroupVersionKind())
	err := r.Get(ctx, client.ObjectKey{Name: projectName, Namespace: "argocd"}, existingProject)
	if err != nil {
		if errors.IsNotFound(err) {
			r.Log.Info("Creating ArgoCD AppProject", "name", projectName, "pipeline", rb.Spec.PipelineName)
			if err := r.Create(ctx, appProject); err != nil {
				return fmt.Errorf("failed to create ArgoCD AppProject: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to get ArgoCD AppProject: %w", err)
	}

	r.Log.Info("Updating ArgoCD AppProject", "name", projectName, "pipeline", rb.Spec.PipelineName)
	existingProject.Object["spec"] = spec
	existingProject.SetLabels(appProject.GetLabels())
	if err := r.Update(ctx, existingProject); err != nil {
		return fmt.Errorf("failed to update ArgoCD AppProject: %w", err)
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

// provisionResourceQuota creates or updates the pipeline ResourceQuota
func (r *RepoBindingReconciler) provisionResourceQuota(ctx context.Context, rb *platformv1alpha1.RepoBinding) error {
	resourceQuota := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant-quota",
			Namespace: rb.Spec.PipelineName,
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: corev1.ResourceList{
				"requests.cpu":           resource.MustParse("4"),
				"limits.cpu":             resource.MustParse("8"),
				"requests.memory":        resource.MustParse("8Gi"),
				"limits.memory":          resource.MustParse("16Gi"),
				"persistentvolumeclaims": resource.MustParse("5"),
				"pods":                   resource.MustParse("20"),
			},
		},
	}

	existingRQ := &corev1.ResourceQuota{}
	err := r.Get(ctx, client.ObjectKey{Name: "tenant-quota", Namespace: rb.Spec.PipelineName}, existingRQ)
	if err != nil {
		if errors.IsNotFound(err) {
			r.Log.Info("Creating resource quota", "namespace", rb.Spec.PipelineName)
			if err := r.Create(ctx, resourceQuota); err != nil {
				return fmt.Errorf("failed to create resource quota: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to get resource quota: %w", err)
	}

	r.Log.Info("Updating resource quota", "namespace", rb.Spec.PipelineName)
	existingRQ.Spec.Hard = resourceQuota.Spec.Hard
	if err := r.Update(ctx, existingRQ); err != nil {
		return fmt.Errorf("failed to update resource quota: %w", err)
	}

	return nil
}

// provisionLimitRange creates or updates the pipeline LimitRange
func (r *RepoBindingReconciler) provisionLimitRange(ctx context.Context, rb *platformv1alpha1.RepoBinding) error {
	limitRange := &corev1.LimitRange{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant-limits",
			Namespace: rb.Spec.PipelineName,
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

	existingLR := &corev1.LimitRange{}
	err := r.Get(ctx, client.ObjectKey{Name: "tenant-limits", Namespace: rb.Spec.PipelineName}, existingLR)
	if err != nil {
		if errors.IsNotFound(err) {
			r.Log.Info("Creating limit range", "namespace", rb.Spec.PipelineName)
			if err := r.Create(ctx, limitRange); err != nil {
				return fmt.Errorf("failed to create limit range: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to get limit range: %w", err)
	}

	r.Log.Info("Updating limit range", "namespace", rb.Spec.PipelineName)
	existingLR.Spec.Limits = limitRange.Spec.Limits
	if err := r.Update(ctx, existingLR); err != nil {
		return fmt.Errorf("failed to update limit range: %w", err)
	}

	return nil
}

func (r *RepoBindingReconciler) provisionNetworkPolicy(ctx context.Context, rb *platformv1alpha1.RepoBinding) error {
	networkPolicy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant-isolation",
			Namespace: rb.Spec.PipelineName,
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
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"aphex.dev/org": rb.Spec.AphexOrg,
								},
							},
						},
					},
				},
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				// TODO: Implement allowlist-based egress policy
				{},
			},
		},
	}

	existingNP := &networkingv1.NetworkPolicy{}
	err := r.Get(ctx, client.ObjectKey{Name: "tenant-isolation", Namespace: rb.Spec.PipelineName}, existingNP)
	if err != nil {
		if errors.IsNotFound(err) {
			r.Log.Info("Creating network policy", "namespace", rb.Spec.PipelineName)
			if err := r.Create(ctx, networkPolicy); err != nil {
				return fmt.Errorf("failed to create network policy: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to get network policy: %w", err)
	}

	r.Log.Info("Updating network policy", "namespace", rb.Spec.PipelineName)
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
}`, rb.Spec.PipelineName, rb.Spec.PipelineName)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "terraform-backend-config",
			Namespace: rb.Spec.PipelineName,
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"backend.tf": backendConfig,
		},
	}

	existingSecret := &corev1.Secret{}
	err := r.Get(ctx, client.ObjectKey{Name: "terraform-backend-config", Namespace: rb.Spec.PipelineName}, existingSecret)
	if err != nil {
		if errors.IsNotFound(err) {
			r.Log.Info("Creating Terraform backend secret", "namespace", rb.Spec.PipelineName)
			if err := r.Create(ctx, secret); err != nil {
				return fmt.Errorf("failed to create Terraform backend secret: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to get Terraform backend secret: %w", err)
	}

	r.Log.Info("Updating Terraform backend secret", "namespace", rb.Spec.PipelineName)
	existingSecret.StringData = secret.StringData
	if err := r.Update(ctx, existingSecret); err != nil {
		return fmt.Errorf("failed to update Terraform backend secret: %w", err)
	}

	return nil
}

func (r *RepoBindingReconciler) updateEventListenerNamespaces(ctx context.Context, rb *platformv1alpha1.RepoBinding) error {
	orgNamespace := fmt.Sprintf("org-%s", rb.Spec.AphexOrg)
	eventListener := &triggersv1beta1.EventListener{}

	if err := r.Get(ctx, client.ObjectKey{Name: "github-listener", Namespace: orgNamespace}, eventListener); err != nil {
		return fmt.Errorf("failed to get EventListener: %w", err)
	}

	for _, ns := range eventListener.Spec.NamespaceSelector.MatchNames {
		if ns == rb.Spec.PipelineName {
			return nil
		}
	}

	eventListener.Spec.NamespaceSelector.MatchNames = append(
		eventListener.Spec.NamespaceSelector.MatchNames,
		rb.Spec.PipelineName,
	)

	if err := r.Update(ctx, eventListener); err != nil {
		return fmt.Errorf("failed to update EventListener: %w", err)
	}

	return nil
}

// provisionTriggerTemplate creates or updates the TriggerTemplate for pipeline execution
func (r *RepoBindingReconciler) provisionTriggerTemplate(ctx context.Context, rb *platformv1alpha1.RepoBinding) error {
	orgNamespace := fmt.Sprintf("org-%s", rb.Spec.AphexOrg)

	// Get template from catalog
	template, err := r.TemplateCatalog.Get(rb.Spec.TemplateRef)
	if err != nil {
		return fmt.Errorf("failed to get template from catalog: %w", err)
	}

	// Convert to TriggerTemplate and materialize in org namespace
	triggerTemplate := template.ToTriggerTemplate(orgNamespace, rb.Spec.AphexOrg)
	triggerTemplate.SetName(fmt.Sprintf("%s-trigger-template", rb.Spec.PipelineName))
	triggerTemplate.Labels["platform.aphex/pipeline"] = rb.Spec.PipelineName

	// Apply the TriggerTemplate
	if err := r.Client.Patch(ctx, triggerTemplate, client.Apply, client.ForceOwnership, client.FieldOwner("platform-controller")); err != nil {
		return fmt.Errorf("failed to apply TriggerTemplate: %w", err)
	}

	r.Log.Info("Materialized template from catalog",
		"template", rb.Spec.TemplateRef,
		"namespace", orgNamespace,
		"name", triggerTemplate.Name)

	return nil
}

func (r *RepoBindingReconciler) provisionTrigger(ctx context.Context, rb *platformv1alpha1.RepoBinding) error {
	orgNamespace := fmt.Sprintf("org-%s", rb.Spec.AphexOrg)

	// Find the pipeline's namespace
	pipelineNamespace, err := r.findPipelineNamespace(ctx, rb.Spec.PipelineName)
	if err != nil {
		return fmt.Errorf("failed to find pipeline %q: %w", rb.Spec.PipelineName, err)
	}

	// TODO: Remove bdchatham hardcode and use RepoOrg from spec
	// repoFullName := fmt.Sprintf("%s/%s", rb.Spec.RepoOrg, rb.Spec.RepoName)
	repoFullName := fmt.Sprintf("bdchatham/%s", rb.Spec.RepoName)

	trigger := &triggersv1beta1.Trigger{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "triggers.tekton.dev/v1beta1",
			Kind:       "Trigger",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-trigger", rb.Spec.PipelineName),
			Namespace: orgNamespace,
			Labels: map[string]string{
				"platform.aphex/pipeline":     rb.Spec.PipelineName,
				"platform.aphex/managed-by":   "platform-controller",
				"platform.aphex/organization": rb.Spec.AphexOrg,
			},
		},
		Spec: triggersv1beta1.TriggerSpec{
			Interceptors: []*triggersv1beta1.TriggerInterceptor{
				{
					Ref: triggersv1beta1.InterceptorRef{
						Name: "cel",
						Kind: triggersv1beta1.ClusterInterceptorKind,
					},
					Params: []triggersv1beta1.InterceptorParams{
						{
							Name:  "filter",
							Value: apiextensionsv1.JSON{Raw: []byte(fmt.Sprintf(`"%s"`, fmt.Sprintf("body.repository.full_name == '%s'", repoFullName)))},
						},
					},
				},
			},
			Bindings: []*triggersv1beta1.TriggerSpecBinding{
				{Ref: "github-push-binding"},
				{Name: "pipeline-name", Value: stringPtr(rb.Spec.PipelineName)},
				{Name: "pipeline-namespace", Value: stringPtr(pipelineNamespace)},
				{Name: "org-name", Value: stringPtr(rb.Spec.AphexOrg)},
				{Name: "repo-full-name", Value: stringPtr(repoFullName)},
				{Name: "event-type", Value: stringPtr("$(header.X-Github-Event)")},
				{Name: "event-id", Value: stringPtr("$(header.X-Github-Delivery)")},
				{Name: "triggered-at", Value: stringPtr("$(body.repository.pushed_at)")},
			},
			Template: triggersv1beta1.TriggerSpecTemplate{
				Ref: stringPtr(fmt.Sprintf("%s-trigger-template", rb.Spec.PipelineName)),
			},
		},
	}

	if err := r.Client.Patch(ctx, trigger, client.Apply, client.ForceOwnership, client.FieldOwner("platform-controller")); err != nil {
		return fmt.Errorf("failed to apply Trigger: %w", err)
	}

	r.Log.Info("Provisioned trigger with CEL filter",
		"trigger", trigger.Name,
		"template", *trigger.Spec.Template.Ref,
		"pipeline", rb.Spec.PipelineName,
		"repoFilter", repoFullName)

	return nil
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
	webhookSecretRef := fmt.Sprintf("webhook-%s", rb.Spec.PipelineName)

	for i, entry := range allowlist.Repos {
		if entry.Org == rb.Spec.RepoOrg && entry.Name == rb.Spec.RepoName {
			// Repository exists, update tenant mapping and webhook secret ref if different
			updated := false
			if entry.Tenant != rb.Spec.PipelineName {
				r.Log.Info("Updating tenant mapping for repository",
					"org", rb.Spec.RepoOrg,
					"repo", rb.Spec.RepoName,
					"oldTenant", entry.Tenant,
					"newTenant", rb.Spec.PipelineName)
				allowlist.Repos[i].Tenant = rb.Spec.PipelineName
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
					"tenant", rb.Spec.PipelineName)
				return nil
			}

			// Marshal back to YAML and update ConfigMap
			updatedYAML, err := k8syaml.Marshal(&allowlist)
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
		Tenant:           rb.Spec.PipelineName,
		Enabled:          true,
		WebhookSecretRef: webhookSecretRef,
	}
	allowlist.Repos = append(allowlist.Repos, newEntry)

	// Marshal back to YAML
	updatedYAML, err := k8syaml.Marshal(&allowlist)
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
		"tenant", rb.Spec.PipelineName)

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
	err = r.Get(ctx, client.ObjectKey{Name: "github-webhook-secret", Namespace: rb.Spec.PipelineName}, pipelineSecret)
	if err != nil {
		if errors.IsNotFound(err) {
			// Create a copy of the secret in the pipeline namespace
			pipelineSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "github-webhook-secret",
					Namespace: rb.Spec.PipelineName,
					Labels: map[string]string{
						"platform.aphex/managed-by":   "repobinding-controller",
						"platform.aphex/organization": rb.Spec.RepoOrg,
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
		"tenant", rb.Spec.PipelineName,
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

		// Try to get the pipeline in this namespace using typed client
		pipeline := &tektonv1.Pipeline{}
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
