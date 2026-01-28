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

// AgentValidator validates Agent resources at admission time.
// It implements webhook.CustomValidator interface.
type AgentValidator struct {
	logger logr.Logger
}

// NewAgentValidator creates a new AgentValidator.
func NewAgentValidator(logger logr.Logger) *AgentValidator {
	return &AgentValidator{
		logger: logger.WithName("AgentValidator"),
	}
}

// SetupWebhookWithManager registers the webhook with the manager.
func (v *AgentValidator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&platformv1alpha1.Agent{}).
		WithValidator(v).
		Complete()
}

// ValidateCreate validates an Agent on creation.
func (v *AgentValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	agent, ok := obj.(*platformv1alpha1.Agent)
	if !ok {
		return nil, fmt.Errorf("expected Agent, got %T", obj)
	}

	v.logger.V(1).Info("Validating Agent creation",
		"name", agent.Name,
		"namespace", agent.Namespace,
	)

	if err := v.validateSpec(agent); err != nil {
		v.logger.Info("Agent validation failed",
			"name", agent.Name,
			"error", err.Error(),
		)
		return nil, err
	}

	return nil, nil
}

// ValidateUpdate validates an Agent on update.
func (v *AgentValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	agent, ok := newObj.(*platformv1alpha1.Agent)
	if !ok {
		return nil, fmt.Errorf("expected Agent, got %T", newObj)
	}

	v.logger.V(1).Info("Validating Agent update",
		"name", agent.Name,
		"namespace", agent.Namespace,
	)

	if err := v.validateSpec(agent); err != nil {
		v.logger.Info("Agent validation failed",
			"name", agent.Name,
			"error", err.Error(),
		)
		return nil, err
	}

	return nil, nil
}

// ValidateDelete validates an Agent on deletion.
func (v *AgentValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	// No validation needed for deletion
	return nil, nil
}

// validateSpec validates the Agent spec fields.
func (v *AgentValidator) validateSpec(agent *platformv1alpha1.Agent) error {
	// Validate DisplayName (required)
	if agent.Spec.DisplayName == "" {
		return newValidationError("spec.displayName", "displayName is required")
	}
	if len(agent.Spec.DisplayName) > 256 {
		return newValidationError("spec.displayName",
			"displayName must be 256 characters or less")
	}

	// Validate Model (required)
	if err := v.validateModelSpec(&agent.Spec.Model); err != nil {
		return err
	}

	// Validate KnowledgeBase reference if provided
	if agent.Spec.KnowledgeBase != nil {
		if err := v.validateKnowledgeBaseConfig(agent.Spec.KnowledgeBase); err != nil {
			return err
		}
	}

	// Validate Orchestration config if provided
	if agent.Spec.Orchestration != nil {
		if err := v.validateOrchestrationConfig(agent.Spec.Orchestration); err != nil {
			return err
		}
	}

	return nil
}

// validateModelSpec validates the model server configuration.
func (v *AgentValidator) validateModelSpec(model *platformv1alpha1.ModelSpec) error {
	// Validate Provider (required)
	if model.Provider == "" {
		return newValidationError("spec.model.provider", "provider is required")
	}
	if !modelProviders[model.Provider] {
		validProviders := make([]string, 0, len(modelProviders))
		for p := range modelProviders {
			validProviders = append(validProviders, p)
		}
		return newValidationError("spec.model.provider",
			fmt.Sprintf("provider must be one of: %s", strings.Join(validProviders, ", ")))
	}

	// Validate Name (required)
	if model.Name == "" {
		return newValidationError("spec.model.name", "model name is required")
	}

	// Validate GPUCount if provided
	if model.GPUCount < 0 {
		return newValidationError("spec.model.gpuCount",
			"gpuCount must be a non-negative integer")
	}

	// Validate Port if provided
	if model.Port != 0 {
		if model.Port < 1024 || model.Port > 65535 {
			return newValidationError("spec.model.port",
				"port must be between 1024 and 65535")
		}
	}

	// Validate Image if provided
	if model.Image != "" {
		if err := v.validateContainerImage(model.Image, "spec.model.image"); err != nil {
			return err
		}
	}

	return nil
}

// validateKnowledgeBaseConfig validates the knowledge base reference.
func (v *AgentValidator) validateKnowledgeBaseConfig(kb *platformv1alpha1.KnowledgeBaseConfig) error {
	// Validate Name (required)
	if kb.Name == "" {
		return newValidationError("spec.knowledgeBase.name",
			"knowledgeBase name is required when knowledgeBase is specified")
	}

	// Validate Name follows Kubernetes naming conventions
	if !namespacePattern.MatchString(kb.Name) {
		return newValidationError("spec.knowledgeBase.name",
			"knowledgeBase name must be a valid Kubernetes name")
	}

	// Validate Namespace if provided
	if kb.Namespace != "" && !namespacePattern.MatchString(kb.Namespace) {
		return newValidationError("spec.knowledgeBase.namespace",
			"knowledgeBase namespace must be a valid Kubernetes namespace name")
	}

	return nil
}

// validateOrchestrationConfig validates the orchestration configuration.
func (v *AgentValidator) validateOrchestrationConfig(orch *platformv1alpha1.OrchestrationConfig) error {
	// Validate Port if provided
	if orch.Port != 0 {
		if orch.Port < 1024 || orch.Port > 65535 {
			return newValidationError("spec.orchestration.port",
				"port must be between 1024 and 65535")
		}
	}

	// Validate Image if provided
	if orch.Image != "" {
		if err := v.validateContainerImage(orch.Image, "spec.orchestration.image"); err != nil {
			return err
		}
	}

	return nil
}

// validateContainerImage validates a container image reference.
func (v *AgentValidator) validateContainerImage(image, field string) error {
	if image == "" {
		return nil
	}

	// Basic validation - image should not contain spaces or invalid characters
	if strings.ContainsAny(image, " \t\n") {
		return newValidationError(field, "image cannot contain whitespace")
	}

	// Image should have at least a name component
	parts := strings.Split(image, "/")
	lastPart := parts[len(parts)-1]
	if lastPart == "" {
		return newValidationError(field, "image must have a name component")
	}

	return nil
}
