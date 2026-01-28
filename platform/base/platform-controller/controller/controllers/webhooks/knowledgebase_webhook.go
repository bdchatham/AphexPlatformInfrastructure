package webhooks

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	platformv1alpha1 "github.com/bdchatham/AphexPlatformInfrastructure/platform/platform-controller/controller/api/v1alpha1"
)

// KnowledgeBaseValidator validates KnowledgeBase resources at admission time.
// It implements webhook.CustomValidator interface.
type KnowledgeBaseValidator struct {
	logger logr.Logger
}

// NewKnowledgeBaseValidator creates a new KnowledgeBaseValidator.
func NewKnowledgeBaseValidator(logger logr.Logger) *KnowledgeBaseValidator {
	return &KnowledgeBaseValidator{
		logger: logger.WithName("KnowledgeBaseValidator"),
	}
}

// SetupWebhookWithManager registers the webhook with the manager.
func (v *KnowledgeBaseValidator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&platformv1alpha1.KnowledgeBase{}).
		WithValidator(v).
		Complete()
}

// ValidateCreate validates a KnowledgeBase on creation.
func (v *KnowledgeBaseValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	kb, ok := obj.(*platformv1alpha1.KnowledgeBase)
	if !ok {
		return nil, fmt.Errorf("expected KnowledgeBase, got %T", obj)
	}

	v.logger.V(1).Info("Validating KnowledgeBase creation",
		"name", kb.Name,
		"namespace", kb.Namespace,
	)

	if err := v.validateSpec(kb); err != nil {
		v.logger.Info("KnowledgeBase validation failed",
			"name", kb.Name,
			"error", err.Error(),
		)
		return nil, err
	}

	return nil, nil
}

// ValidateUpdate validates a KnowledgeBase on update.
func (v *KnowledgeBaseValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	kb, ok := newObj.(*platformv1alpha1.KnowledgeBase)
	if !ok {
		return nil, fmt.Errorf("expected KnowledgeBase, got %T", newObj)
	}

	v.logger.V(1).Info("Validating KnowledgeBase update",
		"name", kb.Name,
		"namespace", kb.Namespace,
	)

	if err := v.validateSpec(kb); err != nil {
		v.logger.Info("KnowledgeBase validation failed",
			"name", kb.Name,
			"error", err.Error(),
		)
		return nil, err
	}

	return nil, nil
}

// ValidateDelete validates a KnowledgeBase on deletion.
func (v *KnowledgeBaseValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	// No validation needed for deletion
	return nil, nil
}

// validateSpec validates the KnowledgeBase spec fields.
func (v *KnowledgeBaseValidator) validateSpec(kb *platformv1alpha1.KnowledgeBase) error {
	// Validate DisplayName (required)
	if kb.Spec.DisplayName == "" {
		return newValidationError("spec.displayName", "displayName is required")
	}
	if len(kb.Spec.DisplayName) > 256 {
		return newValidationError("spec.displayName",
			"displayName must be 256 characters or less")
	}

	// Validate Repositories (required, at least one)
	if len(kb.Spec.Repositories) == 0 {
		return newValidationError("spec.repositories",
			"at least one repository is required")
	}

	// Validate each repository
	for i, repo := range kb.Spec.Repositories {
		if err := v.validateRepository(&repo, i); err != nil {
			return err
		}
	}

	// Check for duplicate repository URLs
	seen := make(map[string]bool)
	for i, repo := range kb.Spec.Repositories {
		normalized := strings.ToLower(repo.URL)
		if seen[normalized] {
			return newValidationError(
				fmt.Sprintf("spec.repositories[%d].url", i),
				fmt.Sprintf("duplicate repository URL %q", repo.URL),
			)
		}
		seen[normalized] = true
	}

	// Validate MCP config if provided
	if kb.Spec.MCP != nil {
		if err := v.validateMCPConfig(kb.Spec.MCP); err != nil {
			return err
		}
	}

	return nil
}

// validateRepository validates a single repository configuration.
func (v *KnowledgeBaseValidator) validateRepository(repo *platformv1alpha1.Repository, index int) error {
	fieldPrefix := fmt.Sprintf("spec.repositories[%d]", index)

	// Validate URL (required)
	if repo.URL == "" {
		return newValidationError(fieldPrefix+".url", "repository URL is required")
	}

	// Validate URL format - must be a GitHub URL
	if !strings.HasPrefix(repo.URL, "https://github.com/") {
		return newValidationError(fieldPrefix+".url",
			"repository URL must start with https://github.com/")
	}

	// Validate URL matches expected pattern
	if !githubURLPattern.MatchString(repo.URL) {
		return newValidationError(fieldPrefix+".url",
			"repository URL must be a valid GitHub repository URL (https://github.com/org/repo)")
	}

	// Validate Branch if provided
	if repo.Branch != "" {
		if !isValidBranchName(repo.Branch) {
			return newValidationError(fieldPrefix+".branch",
				fmt.Sprintf("invalid branch name %q", repo.Branch))
		}
	}

	// Validate Paths if provided
	for j, path := range repo.Paths {
		if err := v.validatePath(path, fmt.Sprintf("%s.paths[%d]", fieldPrefix, j)); err != nil {
			return err
		}
	}

	return nil
}

// validatePath validates a documentation path.
func (v *KnowledgeBaseValidator) validatePath(path, field string) error {
	if path == "" {
		return newValidationError(field, "path cannot be empty")
	}

	// Path must start with .kiro/docs for security
	if !strings.HasPrefix(path, ".kiro/docs") {
		return newValidationError(field,
			"path must start with .kiro/docs for security")
	}

	// Path should not contain path traversal
	if strings.Contains(path, "..") {
		return newValidationError(field,
			"path cannot contain path traversal (..) sequences")
	}

	return nil
}

// validateMCPConfig validates the MCP server configuration.
func (v *KnowledgeBaseValidator) validateMCPConfig(mcp *platformv1alpha1.MCPConfig) error {
	// Validate Image (required when MCP is specified)
	if mcp.Image == "" {
		return newValidationError("spec.mcp.image",
			"image is required when MCP is configured")
	}

	// Validate Image format
	if strings.ContainsAny(mcp.Image, " \t\n") {
		return newValidationError("spec.mcp.image",
			"image cannot contain whitespace")
	}

	// Validate Port (required when MCP is specified)
	if mcp.Port == 0 {
		return newValidationError("spec.mcp.port",
			"port is required when MCP is configured")
	}
	if mcp.Port < 1024 || mcp.Port > 65535 {
		return newValidationError("spec.mcp.port",
			"port must be between 1024 and 65535")
	}

	// Validate Replicas if provided
	if mcp.Replicas < 0 {
		return newValidationError("spec.mcp.replicas",
			"replicas must be a non-negative integer")
	}

	// Validate QueryServiceURL if provided
	if mcp.QueryServiceURL != "" {
		if !strings.HasPrefix(mcp.QueryServiceURL, "http://") &&
			!strings.HasPrefix(mcp.QueryServiceURL, "https://") {
			return newValidationError("spec.mcp.queryServiceURL",
				"queryServiceURL must be a valid HTTP or HTTPS URL")
		}
	}

	return nil
}
