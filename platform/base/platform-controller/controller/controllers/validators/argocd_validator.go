package validators

import (
	"fmt"
	"net/url"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/bdchatham/AphexPlatformInfrastructure/platform/platform-controller/controller/controllers/constants"
)

// ArgoCDValidationError represents an ArgoCD object validation failure.
type ArgoCDValidationError struct {
	ObjectType string
	ObjectName string
	Field      string
	Message    string
}

func (e *ArgoCDValidationError) Error() string {
	if e.ObjectName != "" {
		return fmt.Sprintf("ArgoCD %s %q validation failed: %s: %s",
			e.ObjectType, e.ObjectName, e.Field, e.Message)
	}
	return fmt.Sprintf("ArgoCD %s validation failed: %s: %s",
		e.ObjectType, e.Field, e.Message)
}

// ArgoCDValidator validates ArgoCD Application and AppProject objects.
// It ensures required fields are present and valid before attempting
// to create resources in the cluster.
type ArgoCDValidator struct{}

// NewArgoCDValidator creates a new ArgoCDValidator instance.
func NewArgoCDValidator() *ArgoCDValidator {
	return &ArgoCDValidator{}
}

// ValidateApplication validates an ArgoCD Application object.
// Returns nil if validation passes, or an error describing the validation failure.
func (v *ArgoCDValidator) ValidateApplication(app *unstructured.Unstructured) error {
	if err := v.validateArgoCDGVK(app, constants.ArgoCDAppKind); err != nil {
		return err
	}

	name := app.GetName()
	if name == "" {
		return &ArgoCDValidationError{
			ObjectType: "Application",
			Field:      "metadata.name",
			Message:    "name is required",
		}
	}

	namespace := app.GetNamespace()
	if namespace == "" {
		return &ArgoCDValidationError{
			ObjectType: "Application",
			ObjectName: name,
			Field:      "metadata.namespace",
			Message:    "namespace is required",
		}
	}

	spec, found, err := unstructured.NestedMap(app.Object, "spec")
	if err != nil || !found || len(spec) == 0 {
		return &ArgoCDValidationError{
			ObjectType: "Application",
			ObjectName: name,
			Field:      "spec",
			Message:    "spec is required",
		}
	}

	if err := v.validateApplicationSource(app, name); err != nil {
		return err
	}

	if err := v.validateApplicationDestination(app, name); err != nil {
		return err
	}

	if err := v.validateApplicationProject(app, name); err != nil {
		return err
	}

	return nil
}

// validateApplicationSource validates the source field of an Application.
func (v *ArgoCDValidator) validateApplicationSource(app *unstructured.Unstructured, name string) error {
	source, found, err := unstructured.NestedMap(app.Object, "spec", "source")
	if err != nil || !found || len(source) == 0 {
		sources, foundSources, _ := unstructured.NestedSlice(app.Object, "spec", "sources")
		if !foundSources || len(sources) == 0 {
			return &ArgoCDValidationError{
				ObjectType: "Application",
				ObjectName: name,
				Field:      "spec.source",
				Message:    "source or sources is required",
			}
		}
		for i, s := range sources {
			sourceMap, ok := s.(map[string]interface{})
			if !ok {
				return &ArgoCDValidationError{
					ObjectType: "Application",
					ObjectName: name,
					Field:      fmt.Sprintf("spec.sources[%d]", i),
					Message:    "invalid source format",
				}
			}
			if err := v.validateSourceFields(sourceMap, name, fmt.Sprintf("spec.sources[%d]", i)); err != nil {
				return err
			}
		}
		return nil
	}

	return v.validateSourceFields(source, name, "spec.source")
}

// validateSourceFields validates the fields within a source object.
func (v *ArgoCDValidator) validateSourceFields(source map[string]interface{}, appName, fieldPrefix string) error {
	repoURL, ok := source["repoURL"].(string)
	if !ok || repoURL == "" {
		return &ArgoCDValidationError{
			ObjectType: "Application",
			ObjectName: appName,
			Field:      fieldPrefix + ".repoURL",
			Message:    "repoURL is required",
		}
	}

	if _, err := url.Parse(repoURL); err != nil {
		return &ArgoCDValidationError{
			ObjectType: "Application",
			ObjectName: appName,
			Field:      fieldPrefix + ".repoURL",
			Message:    fmt.Sprintf("invalid URL format: %v", err),
		}
	}

	return nil
}

// validateApplicationDestination validates the destination field of an Application.
func (v *ArgoCDValidator) validateApplicationDestination(app *unstructured.Unstructured, name string) error {
	dest, found, err := unstructured.NestedMap(app.Object, "spec", "destination")
	if err != nil || !found || len(dest) == 0 {
		return &ArgoCDValidationError{
			ObjectType: "Application",
			ObjectName: name,
			Field:      "spec.destination",
			Message:    "destination is required",
		}
	}

	server, hasServer := dest["server"].(string)
	destName, hasName := dest["name"].(string)

	if !hasServer && !hasName {
		return &ArgoCDValidationError{
			ObjectType: "Application",
			ObjectName: name,
			Field:      "spec.destination",
			Message:    "either server or name is required",
		}
	}

	if hasServer && server == "" {
		return &ArgoCDValidationError{
			ObjectType: "Application",
			ObjectName: name,
			Field:      "spec.destination.server",
			Message:    "server cannot be empty if specified",
		}
	}

	if hasName && destName == "" {
		return &ArgoCDValidationError{
			ObjectType: "Application",
			ObjectName: name,
			Field:      "spec.destination.name",
			Message:    "name cannot be empty if specified",
		}
	}

	namespace, _ := dest["namespace"].(string)
	if namespace == "" {
		return &ArgoCDValidationError{
			ObjectType: "Application",
			ObjectName: name,
			Field:      "spec.destination.namespace",
			Message:    "namespace is required",
		}
	}

	return nil
}

// validateApplicationProject validates the project field of an Application.
func (v *ArgoCDValidator) validateApplicationProject(app *unstructured.Unstructured, name string) error {
	project, found, err := unstructured.NestedString(app.Object, "spec", "project")
	if err != nil || !found || project == "" {
		return &ArgoCDValidationError{
			ObjectType: "Application",
			ObjectName: name,
			Field:      "spec.project",
			Message:    "project is required",
		}
	}
	return nil
}

// ValidateAppProject validates an ArgoCD AppProject object.
// Returns nil if validation passes, or an error describing the validation failure.
func (v *ArgoCDValidator) ValidateAppProject(project *unstructured.Unstructured) error {
	if err := v.validateArgoCDGVK(project, constants.ArgoCDProjectKind); err != nil {
		return err
	}

	name := project.GetName()
	if name == "" {
		return &ArgoCDValidationError{
			ObjectType: "AppProject",
			Field:      "metadata.name",
			Message:    "name is required",
		}
	}

	namespace := project.GetNamespace()
	if namespace == "" {
		return &ArgoCDValidationError{
			ObjectType: "AppProject",
			ObjectName: name,
			Field:      "metadata.namespace",
			Message:    "namespace is required",
		}
	}

	spec, found, err := unstructured.NestedMap(project.Object, "spec")
	if err != nil || !found {
		return &ArgoCDValidationError{
			ObjectType: "AppProject",
			ObjectName: name,
			Field:      "spec",
			Message:    "spec is required",
		}
	}

	if err := v.validateAppProjectDestinations(spec, name); err != nil {
		return err
	}

	if err := v.validateAppProjectSourceRepos(spec, name); err != nil {
		return err
	}

	return nil
}

// validateAppProjectDestinations validates the destinations field of an AppProject.
func (v *ArgoCDValidator) validateAppProjectDestinations(spec map[string]interface{}, name string) error {
	destinations, found := spec["destinations"]
	if !found {
		return nil
	}

	destList, ok := destinations.([]interface{})
	if !ok {
		return &ArgoCDValidationError{
			ObjectType: "AppProject",
			ObjectName: name,
			Field:      "spec.destinations",
			Message:    "destinations must be an array",
		}
	}

	for i, dest := range destList {
		destMap, ok := dest.(map[string]interface{})
		if !ok {
			return &ArgoCDValidationError{
				ObjectType: "AppProject",
				ObjectName: name,
				Field:      fmt.Sprintf("spec.destinations[%d]", i),
				Message:    "invalid destination format",
			}
		}

		server, hasServer := destMap["server"].(string)
		destName, hasName := destMap["name"].(string)

		if !hasServer && !hasName {
			return &ArgoCDValidationError{
				ObjectType: "AppProject",
				ObjectName: name,
				Field:      fmt.Sprintf("spec.destinations[%d]", i),
				Message:    "either server or name is required",
			}
		}

		if hasServer && server == "" {
			return &ArgoCDValidationError{
				ObjectType: "AppProject",
				ObjectName: name,
				Field:      fmt.Sprintf("spec.destinations[%d].server", i),
				Message:    "server cannot be empty",
			}
		}

		if hasName && destName == "" {
			return &ArgoCDValidationError{
				ObjectType: "AppProject",
				ObjectName: name,
				Field:      fmt.Sprintf("spec.destinations[%d].name", i),
				Message:    "name cannot be empty if specified",
			}
		}
	}

	return nil
}

// validateAppProjectSourceRepos validates the sourceRepos field of an AppProject.
func (v *ArgoCDValidator) validateAppProjectSourceRepos(spec map[string]interface{}, name string) error {
	sourceRepos, found := spec["sourceRepos"]
	if !found {
		return nil
	}

	repoList, ok := sourceRepos.([]interface{})
	if !ok {
		return &ArgoCDValidationError{
			ObjectType: "AppProject",
			ObjectName: name,
			Field:      "spec.sourceRepos",
			Message:    "sourceRepos must be an array",
		}
	}

	for i, repo := range repoList {
		repoStr, ok := repo.(string)
		if !ok {
			return &ArgoCDValidationError{
				ObjectType: "AppProject",
				ObjectName: name,
				Field:      fmt.Sprintf("spec.sourceRepos[%d]", i),
				Message:    "sourceRepo must be a string",
			}
		}

		if repoStr == "" {
			return &ArgoCDValidationError{
				ObjectType: "AppProject",
				ObjectName: name,
				Field:      fmt.Sprintf("spec.sourceRepos[%d]", i),
				Message:    "sourceRepo cannot be empty",
			}
		}
	}

	return nil
}

// validateArgoCDGVK validates that an object has the expected ArgoCD GVK.
func (v *ArgoCDValidator) validateArgoCDGVK(obj *unstructured.Unstructured, expectedKind string) error {
	gvk := obj.GetObjectKind().GroupVersionKind()

	if gvk.Group != constants.ArgoCDGroup {
		return &ArgoCDValidationError{
			ObjectType: expectedKind,
			Field:      "apiVersion",
			Message:    fmt.Sprintf("expected group %q, got %q", constants.ArgoCDGroup, gvk.Group),
		}
	}

	if gvk.Version != constants.ArgoCDVersion {
		return &ArgoCDValidationError{
			ObjectType: expectedKind,
			Field:      "apiVersion",
			Message:    fmt.Sprintf("expected version %q, got %q", constants.ArgoCDVersion, gvk.Version),
		}
	}

	if gvk.Kind != expectedKind {
		return &ArgoCDValidationError{
			ObjectType: expectedKind,
			Field:      "kind",
			Message:    fmt.Sprintf("expected kind %q, got %q", expectedKind, gvk.Kind),
		}
	}

	return nil
}

// Common ArgoCD GVK definitions.
var (
	ArgoCDApplicationGVK = ExpectedGVK{
		Group:   constants.ArgoCDGroup,
		Version: constants.ArgoCDVersion,
		Kind:    constants.ArgoCDAppKind,
	}

	ArgoCDAppProjectGVK = ExpectedGVK{
		Group:   constants.ArgoCDGroup,
		Version: constants.ArgoCDVersion,
		Kind:    constants.ArgoCDProjectKind,
	}
)
