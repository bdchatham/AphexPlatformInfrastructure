package controllers

import (
	"fmt"
	"regexp"
	"strings"

	platformv1alpha1 "github.com/arbiter/jenkinsx-platform/onboarding-controller/api/v1alpha1"
)

var (
	// Approved GitHub organizations
	approvedOrgs = []string{
		"bdchatham",
	}

	// Privileged namespace names that cannot be used for tenants
	privilegedNamespaces = []string{
		"kube-system",
		"kube-public",
		"kube-node-lease",
		"default",
		"pipeline-system",
		"pipeline-catalog",
		"auth-system",
		"tekton-pipelines",
		"tekton-pipelines-resolvers",
	}

	// Valid namespace pattern
	namespacePattern = regexp.MustCompile(`^[a-z0-9-]+$`)

	// Valid permission profiles
	validPermissionProfiles = []string{"standard", "elevated"}
)

// ValidationError represents a validation failure
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidateRepoBinding validates a RepoBinding spec
func ValidateRepoBinding(rb *platformv1alpha1.RepoBinding) error {
	// Validate repository organization
	if err := validateOrganization(rb.Spec.RepoOrg); err != nil {
		return err
	}

	// Validate namespace name pattern
	if err := validateNamespacePattern(rb.Spec.TenantName); err != nil {
		return err
	}

	// Reject privileged namespace names
	if err := validateNotPrivilegedNamespace(rb.Spec.TenantName); err != nil {
		return err
	}

	// Validate permission profile
	if err := validatePermissionProfile(rb.Spec.PermissionProfile); err != nil {
		return err
	}

	return nil
}

// validateOrganization checks if the repository organization is in the approved list
func validateOrganization(org string) error {
	for _, approvedOrg := range approvedOrgs {
		if org == approvedOrg {
			return nil
		}
	}
	return &ValidationError{
		Field:   "repoOrg",
		Message: fmt.Sprintf("Repository organization '%s' not in approved list", org),
	}
}

// validateNamespacePattern checks if the namespace name matches the required pattern
func validateNamespacePattern(name string) error {
	if !namespacePattern.MatchString(name) {
		return &ValidationError{
			Field:   "tenantName",
			Message: "Namespace name must match pattern ^[a-z0-9-]+$",
		}
	}
	return nil
}

// validateNotPrivilegedNamespace checks if the namespace name is not a privileged namespace
func validateNotPrivilegedNamespace(name string) error {
	for _, privileged := range privilegedNamespaces {
		if name == privileged || strings.HasPrefix(name, privileged+"-") {
			return &ValidationError{
				Field:   "tenantName",
				Message: fmt.Sprintf("Cannot create namespace with privileged name '%s'", name),
			}
		}
	}
	return nil
}

// validatePermissionProfile checks if the permission profile is valid
func validatePermissionProfile(profile string) error {
	// Default to "standard" if empty
	if profile == "" {
		return nil
	}

	for _, validProfile := range validPermissionProfiles {
		if profile == validProfile {
			return nil
		}
	}
	return &ValidationError{
		Field:   "permissionProfile",
		Message: "Permission profile must be 'standard' or 'elevated'",
	}
}
