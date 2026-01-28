package validators

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	authorizationv1 "k8s.io/api/authorization/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RBACValidationError represents an RBAC permission validation failure.
type RBACValidationError struct {
	Resource          string
	MissingPermission string
	Details           string
}

func (e *RBACValidationError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("RBAC validation failed for %s: cannot grant permission %q: %s",
			e.Resource, e.MissingPermission, e.Details)
	}
	return fmt.Sprintf("RBAC validation failed for %s: cannot grant permission %q",
		e.Resource, e.MissingPermission)
}

// RBACValidator validates that the controller has permission to grant RBAC rules.
// It uses SubjectAccessReview to check if the controller's service account can
// grant the specified permissions before attempting to create Roles or ClusterRoles.
type RBACValidator struct {
	client            client.Client
	logger            logr.Logger
	serviceAccountNS  string
	serviceAccountName string
}

// NewRBACValidator creates a new RBACValidator with the given client and logger.
// The serviceAccountNS and serviceAccountName identify the controller's service account
// for permission checking.
func NewRBACValidator(c client.Client, logger logr.Logger, serviceAccountNS, serviceAccountName string) *RBACValidator {
	return &RBACValidator{
		client:            c,
		logger:            logger.WithName("RBACValidator"),
		serviceAccountNS:  serviceAccountNS,
		serviceAccountName: serviceAccountName,
	}
}

// ValidateRoleRules validates that the controller can grant all rules in a Role.
// Returns nil if all rules can be granted, or an error describing the first
// permission that cannot be granted.
func (v *RBACValidator) ValidateRoleRules(ctx context.Context, namespace string, rules []rbacv1.PolicyRule) error {
	for _, rule := range rules {
		if err := v.validatePolicyRule(ctx, namespace, rule); err != nil {
			return err
		}
	}
	return nil
}

// ValidateClusterRoleRules validates that the controller can grant all rules in a ClusterRole.
// Returns nil if all rules can be granted, or an error describing the first
// permission that cannot be granted.
func (v *RBACValidator) ValidateClusterRoleRules(ctx context.Context, rules []rbacv1.PolicyRule) error {
	return v.ValidateRoleRules(ctx, "", rules)
}

// validatePolicyRule checks if the controller can grant a single policy rule.
func (v *RBACValidator) validatePolicyRule(ctx context.Context, namespace string, rule rbacv1.PolicyRule) error {
	for _, apiGroup := range rule.APIGroups {
		for _, resource := range rule.Resources {
			for _, verb := range rule.Verbs {
				if err := v.checkPermission(ctx, namespace, apiGroup, resource, verb); err != nil {
					return err
				}
			}
		}
	}

	for _, nonResourceURL := range rule.NonResourceURLs {
		for _, verb := range rule.Verbs {
			if err := v.checkNonResourcePermission(ctx, nonResourceURL, verb); err != nil {
				return err
			}
		}
	}

	return nil
}

// checkPermission uses SubjectAccessReview to verify the controller can perform
// the specified action.
func (v *RBACValidator) checkPermission(ctx context.Context, namespace, apiGroup, resource, verb string) error {
	sar := &authorizationv1.SubjectAccessReview{
		Spec: authorizationv1.SubjectAccessReviewSpec{
			User: fmt.Sprintf("system:serviceaccount:%s:%s", v.serviceAccountNS, v.serviceAccountName),
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace: namespace,
				Verb:      verb,
				Group:     apiGroup,
				Resource:  resource,
			},
		},
	}

	if err := v.client.Create(ctx, sar); err != nil {
		v.logger.Error(err, "Failed to create SubjectAccessReview",
			"namespace", namespace,
			"apiGroup", apiGroup,
			"resource", resource,
			"verb", verb,
		)
		return &RBACValidationError{
			Resource:          formatResourceRef(apiGroup, resource),
			MissingPermission: verb,
			Details:           fmt.Sprintf("failed to check permission: %v", err),
		}
	}

	if !sar.Status.Allowed {
		v.logger.Info("Permission check failed",
			"namespace", namespace,
			"apiGroup", apiGroup,
			"resource", resource,
			"verb", verb,
			"reason", sar.Status.Reason,
		)

		details := "controller service account lacks this permission"
		if sar.Status.Reason != "" {
			details = sar.Status.Reason
		}

		return &RBACValidationError{
			Resource:          formatResourceRef(apiGroup, resource),
			MissingPermission: verb,
			Details:           details,
		}
	}

	v.logger.V(1).Info("Permission check passed",
		"namespace", namespace,
		"apiGroup", apiGroup,
		"resource", resource,
		"verb", verb,
	)

	return nil
}

// checkNonResourcePermission checks permission for non-resource URLs.
func (v *RBACValidator) checkNonResourcePermission(ctx context.Context, url, verb string) error {
	sar := &authorizationv1.SubjectAccessReview{
		Spec: authorizationv1.SubjectAccessReviewSpec{
			User: fmt.Sprintf("system:serviceaccount:%s:%s", v.serviceAccountNS, v.serviceAccountName),
			NonResourceAttributes: &authorizationv1.NonResourceAttributes{
				Path: url,
				Verb: verb,
			},
		},
	}

	if err := v.client.Create(ctx, sar); err != nil {
		v.logger.Error(err, "Failed to create SubjectAccessReview for non-resource URL",
			"url", url,
			"verb", verb,
		)
		return &RBACValidationError{
			Resource:          url,
			MissingPermission: verb,
			Details:           fmt.Sprintf("failed to check permission: %v", err),
		}
	}

	if !sar.Status.Allowed {
		v.logger.Info("Non-resource permission check failed",
			"url", url,
			"verb", verb,
			"reason", sar.Status.Reason,
		)

		details := "controller service account lacks this permission"
		if sar.Status.Reason != "" {
			details = sar.Status.Reason
		}

		return &RBACValidationError{
			Resource:          url,
			MissingPermission: verb,
			Details:           details,
		}
	}

	return nil
}

// ValidateRole validates a complete Role object before creation.
func (v *RBACValidator) ValidateRole(ctx context.Context, role *rbacv1.Role) error {
	if role.Name == "" {
		return &RBACValidationError{
			Resource:          "Role",
			MissingPermission: "",
			Details:           "role name is required",
		}
	}

	if role.Namespace == "" {
		return &RBACValidationError{
			Resource:          fmt.Sprintf("Role/%s", role.Name),
			MissingPermission: "",
			Details:           "role namespace is required",
		}
	}

	return v.ValidateRoleRules(ctx, role.Namespace, role.Rules)
}

// ValidateClusterRole validates a complete ClusterRole object before creation.
func (v *RBACValidator) ValidateClusterRole(ctx context.Context, clusterRole *rbacv1.ClusterRole) error {
	if clusterRole.Name == "" {
		return &RBACValidationError{
			Resource:          "ClusterRole",
			MissingPermission: "",
			Details:           "cluster role name is required",
		}
	}

	return v.ValidateClusterRoleRules(ctx, clusterRole.Rules)
}

// formatResourceRef formats an API group and resource into a readable string.
func formatResourceRef(apiGroup, resource string) string {
	if apiGroup == "" {
		return resource
	}
	return fmt.Sprintf("%s/%s", apiGroup, resource)
}

// DescribeRules returns a human-readable description of RBAC rules.
// Useful for logging and error messages.
func DescribeRules(rules []rbacv1.PolicyRule) string {
	var descriptions []string
	for _, rule := range rules {
		for _, apiGroup := range rule.APIGroups {
			for _, resource := range rule.Resources {
				desc := fmt.Sprintf("%s on %s", strings.Join(rule.Verbs, ","), formatResourceRef(apiGroup, resource))
				descriptions = append(descriptions, desc)
			}
		}
		for _, url := range rule.NonResourceURLs {
			desc := fmt.Sprintf("%s on %s", strings.Join(rule.Verbs, ","), url)
			descriptions = append(descriptions, desc)
		}
	}
	return strings.Join(descriptions, "; ")
}
