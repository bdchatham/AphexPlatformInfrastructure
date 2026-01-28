package webhooks

import (
	"context"
	"fmt"
	"net/mail"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	platformv1alpha1 "github.com/bdchatham/AphexPlatformInfrastructure/platform/platform-controller/controller/api/v1alpha1"
)

// OrganizationValidator validates Organization resources at admission time.
// It implements webhook.CustomValidator interface.
type OrganizationValidator struct {
	logger logr.Logger
}

// NewOrganizationValidator creates a new OrganizationValidator.
func NewOrganizationValidator(logger logr.Logger) *OrganizationValidator {
	return &OrganizationValidator{
		logger: logger.WithName("OrganizationValidator"),
	}
}

// SetupWebhookWithManager registers the webhook with the manager.
func (v *OrganizationValidator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&platformv1alpha1.Organization{}).
		WithValidator(v).
		Complete()
}

// ValidateCreate validates an Organization on creation.
func (v *OrganizationValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	org, ok := obj.(*platformv1alpha1.Organization)
	if !ok {
		return nil, fmt.Errorf("expected Organization, got %T", obj)
	}

	v.logger.V(1).Info("Validating Organization creation",
		"name", org.Name,
		"namespace", org.Namespace,
	)

	if err := v.validateSpec(org); err != nil {
		v.logger.V(1).Info("Organization validation failed",
			"name", org.Name,
			"error", err.Error(),
		)
		return nil, err
	}

	return nil, nil
}

// ValidateUpdate validates an Organization on update.
func (v *OrganizationValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	org, ok := newObj.(*platformv1alpha1.Organization)
	if !ok {
		return nil, fmt.Errorf("expected Organization, got %T", newObj)
	}

	v.logger.V(1).Info("Validating Organization update",
		"name", org.Name,
		"namespace", org.Namespace,
	)

	if err := v.validateSpec(org); err != nil {
		v.logger.V(1).Info("Organization validation failed",
			"name", org.Name,
			"error", err.Error(),
		)
		return nil, err
	}

	return nil, nil
}

// ValidateDelete validates an Organization on deletion.
func (v *OrganizationValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	// No validation needed for deletion
	return nil, nil
}

// validateSpec validates the Organization spec fields.
func (v *OrganizationValidator) validateSpec(org *platformv1alpha1.Organization) error {
	// Validate DisplayName (required)
	if org.Spec.DisplayName == "" {
		return newValidationError("spec.displayName", "displayName is required")
	}
	if len(org.Spec.DisplayName) > 256 {
		return newValidationError("spec.displayName",
			"displayName must be 256 characters or less")
	}

	// Validate AdminUsers (required, at least one)
	if len(org.Spec.AdminUsers) == 0 {
		return newValidationError("spec.adminUsers",
			"at least one admin user is required")
	}

	// Validate each admin user email
	for i, email := range org.Spec.AdminUsers {
		if err := v.validateEmail(email); err != nil {
			return newValidationError(
				fmt.Sprintf("spec.adminUsers[%d]", i),
				err.Error(),
			)
		}
	}

	// Check for duplicate admin users
	seen := make(map[string]bool)
	for i, email := range org.Spec.AdminUsers {
		normalized := strings.ToLower(email)
		if seen[normalized] {
			return newValidationError(
				fmt.Sprintf("spec.adminUsers[%d]", i),
				fmt.Sprintf("duplicate admin user %q", email),
			)
		}
		seen[normalized] = true
	}

	// Validate WebhookSecret if provided
	if org.Spec.WebhookSecret != "" {
		if len(org.Spec.WebhookSecret) < 16 {
			return newValidationError("spec.webhookSecret",
				"webhookSecret must be at least 16 characters for security")
		}
	}

	// Validate Organization name follows Kubernetes naming conventions
	if !namespacePattern.MatchString(org.Name) {
		return newValidationError("metadata.name",
			"organization name must be a valid Kubernetes name (lowercase alphanumeric and hyphens)")
	}

	// Validate Organization name is not a privileged namespace
	if isPrivilegedNamespace(org.Name) {
		return newValidationError("metadata.name",
			fmt.Sprintf("organization name %q conflicts with a privileged namespace", org.Name))
	}

	return nil
}

// validateEmail validates an email address format.
func (v *OrganizationValidator) validateEmail(email string) error {
	if email == "" {
		return fmt.Errorf("email address cannot be empty")
	}

	_, err := mail.ParseAddress(email)
	if err != nil {
		return fmt.Errorf("invalid email format %q: %w", email, err)
	}

	return nil
}
