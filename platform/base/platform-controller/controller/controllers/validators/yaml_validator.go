// Package validators provides validation components for platform controller resources.
// These validators ensure resources meet expected constraints before provisioning,
// catching errors early and providing descriptive error messages.
package validators

import (
	"bytes"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// YAMLValidationError represents a YAML validation failure with detailed context.
type YAMLValidationError struct {
	Field    string
	Expected string
	Actual   string
	Message  string
}

func (e *YAMLValidationError) Error() string {
	if e.Expected != "" && e.Actual != "" {
		return fmt.Sprintf("YAML validation failed for %s: expected %s, got %s", e.Field, e.Expected, e.Actual)
	}
	return fmt.Sprintf("YAML validation failed for %s: %s", e.Field, e.Message)
}

// YAMLValidator validates decoded YAML objects against expected schemas.
// It ensures parsed objects have the expected Kind and APIVersion before
// they are used for provisioning.
type YAMLValidator struct{}

// NewYAMLValidator creates a new YAMLValidator instance.
func NewYAMLValidator() *YAMLValidator {
	return &YAMLValidator{}
}

// ExpectedGVK defines the expected GroupVersionKind for validation.
type ExpectedGVK struct {
	Group   string
	Version string
	Kind    string
}

// ToGVK converts ExpectedGVK to a schema.GroupVersionKind.
func (e ExpectedGVK) ToGVK() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   e.Group,
		Version: e.Version,
		Kind:    e.Kind,
	}
}

// String returns a human-readable representation of the expected GVK.
func (e ExpectedGVK) String() string {
	if e.Group == "" {
		return fmt.Sprintf("%s/%s", e.Version, e.Kind)
	}
	return fmt.Sprintf("%s/%s/%s", e.Group, e.Version, e.Kind)
}

// ValidateAndDecode parses YAML content and validates it matches the expected GVK.
// Returns the decoded unstructured object if validation passes.
func (v *YAMLValidator) ValidateAndDecode(yamlContent []byte, expected ExpectedGVK) (*unstructured.Unstructured, error) {
	if len(yamlContent) == 0 {
		return nil, &YAMLValidationError{
			Field:   "content",
			Message: "YAML content is empty",
		}
	}

	obj := &unstructured.Unstructured{}
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(yamlContent), 4096)

	if err := decoder.Decode(obj); err != nil {
		return nil, &YAMLValidationError{
			Field:   "content",
			Message: fmt.Sprintf("failed to decode YAML: %v", err),
		}
	}

	if err := v.ValidateGVK(obj, expected); err != nil {
		return nil, err
	}

	return obj, nil
}

// ValidateGVK validates that an unstructured object has the expected GroupVersionKind.
func (v *YAMLValidator) ValidateGVK(obj *unstructured.Unstructured, expected ExpectedGVK) error {
	gvk := obj.GetObjectKind().GroupVersionKind()

	if gvk.Kind == "" {
		return &YAMLValidationError{
			Field:    "kind",
			Expected: expected.Kind,
			Actual:   "(empty)",
			Message:  "decoded object has no Kind specified",
		}
	}

	if gvk.Kind != expected.Kind {
		return &YAMLValidationError{
			Field:    "kind",
			Expected: expected.Kind,
			Actual:   gvk.Kind,
		}
	}

	if gvk.Version == "" {
		return &YAMLValidationError{
			Field:    "apiVersion",
			Expected: expected.ToGVK().GroupVersion().String(),
			Actual:   "(empty)",
			Message:  "decoded object has no apiVersion specified",
		}
	}

	if gvk.Group != expected.Group {
		return &YAMLValidationError{
			Field:    "group",
			Expected: expected.Group,
			Actual:   gvk.Group,
		}
	}

	if gvk.Version != expected.Version {
		return &YAMLValidationError{
			Field:    "version",
			Expected: expected.Version,
			Actual:   gvk.Version,
		}
	}

	return nil
}

// ValidateMultipleGVKs validates that an object matches one of several expected GVKs.
// Returns nil if the object matches any of the expected GVKs.
func (v *YAMLValidator) ValidateMultipleGVKs(obj *unstructured.Unstructured, expectedList []ExpectedGVK) error {
	if len(expectedList) == 0 {
		return &YAMLValidationError{
			Field:   "expectedGVKs",
			Message: "no expected GVKs provided for validation",
		}
	}

	gvk := obj.GetObjectKind().GroupVersionKind()

	for _, expected := range expectedList {
		if gvk.Group == expected.Group && gvk.Version == expected.Version && gvk.Kind == expected.Kind {
			return nil
		}
	}

	expectedStrs := make([]string, len(expectedList))
	for i, e := range expectedList {
		expectedStrs[i] = e.String()
	}

	return &YAMLValidationError{
		Field:    "apiVersion/kind",
		Expected: fmt.Sprintf("one of %v", expectedStrs),
		Actual:   gvk.String(),
	}
}

// ValidateRequiredFields checks that required fields are present in the unstructured object.
// fieldPaths should be dot-separated paths like "spec.template.spec.containers".
func (v *YAMLValidator) ValidateRequiredFields(obj *unstructured.Unstructured, fieldPaths []string) error {
	for _, path := range fieldPaths {
		if _, found, err := unstructured.NestedFieldNoCopy(obj.Object, splitPath(path)...); err != nil || !found {
			return &YAMLValidationError{
				Field:   path,
				Message: fmt.Sprintf("required field %q is missing or invalid", path),
			}
		}
	}
	return nil
}

// ValidateStringField checks that a string field has a non-empty value.
func (v *YAMLValidator) ValidateStringField(obj *unstructured.Unstructured, fieldPath string) error {
	value, found, err := unstructured.NestedString(obj.Object, splitPath(fieldPath)...)
	if err != nil {
		return &YAMLValidationError{
			Field:   fieldPath,
			Message: fmt.Sprintf("field %q is not a string: %v", fieldPath, err),
		}
	}
	if !found || value == "" {
		return &YAMLValidationError{
			Field:   fieldPath,
			Message: fmt.Sprintf("required string field %q is empty or missing", fieldPath),
		}
	}
	return nil
}

// splitPath splits a dot-separated field path into components.
func splitPath(path string) []string {
	var result []string
	current := ""
	for _, c := range path {
		if c == '.' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

// Common GVK definitions for Tekton resources.
var (
	TektonPipelineGVK = ExpectedGVK{
		Group:   "tekton.dev",
		Version: "v1",
		Kind:    "Pipeline",
	}

	TektonTaskGVK = ExpectedGVK{
		Group:   "tekton.dev",
		Version: "v1",
		Kind:    "Task",
	}

	TektonPipelineRunGVK = ExpectedGVK{
		Group:   "tekton.dev",
		Version: "v1",
		Kind:    "PipelineRun",
	}

	TektonTaskRunGVK = ExpectedGVK{
		Group:   "tekton.dev",
		Version: "v1",
		Kind:    "TaskRun",
	}

	TektonTriggerTemplateGVK = ExpectedGVK{
		Group:   "triggers.tekton.dev",
		Version: "v1beta1",
		Kind:    "TriggerTemplate",
	}

	TektonTriggerGVK = ExpectedGVK{
		Group:   "triggers.tekton.dev",
		Version: "v1beta1",
		Kind:    "Trigger",
	}

	TektonEventListenerGVK = ExpectedGVK{
		Group:   "triggers.tekton.dev",
		Version: "v1beta1",
		Kind:    "EventListener",
	}
)
