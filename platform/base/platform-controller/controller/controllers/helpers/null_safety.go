// Package helpers provides shared utility components for platform controllers.
// These components encapsulate common patterns like status updates, finalizer management,
// and dependency resolution to ensure consistent behavior across all controllers.
package helpers

import (
	"errors"
	"fmt"
)

// ErrNilDependency is returned when a required dependency is nil.
var ErrNilDependency = errors.New("nil dependency")

// DependencyError represents an error caused by a nil or invalid dependency.
type DependencyError struct {
	DependencyName string
	ControllerName string
	Err            error
}

// Error implements the error interface.
func (e *DependencyError) Error() string {
	return fmt.Sprintf("controller %q: dependency %q is nil or invalid: %v",
		e.ControllerName, e.DependencyName, e.Err)
}

// Unwrap returns the underlying error for errors.Is/As support.
func (e *DependencyError) Unwrap() error {
	return e.Err
}

// NewDependencyError creates a new DependencyError.
func NewDependencyError(controllerName, dependencyName string) *DependencyError {
	return &DependencyError{
		DependencyName: dependencyName,
		ControllerName: controllerName,
		Err:            ErrNilDependency,
	}
}

// NullSafetyChecker provides null-safety validation for controller dependencies.
type NullSafetyChecker struct {
	controllerName string
	errors         []error
}

// NewNullSafetyChecker creates a new NullSafetyChecker for the given controller.
func NewNullSafetyChecker(controllerName string) *NullSafetyChecker {
	return &NullSafetyChecker{
		controllerName: controllerName,
		errors:         make([]error, 0),
	}
}

// CheckNotNil verifies that the given dependency is not nil.
// If nil, it records an error that can be retrieved via Validate().
// Returns the checker for method chaining.
func (c *NullSafetyChecker) CheckNotNil(dependency interface{}, name string) *NullSafetyChecker {
	if dependency == nil {
		c.errors = append(c.errors, NewDependencyError(c.controllerName, name))
	}
	return c
}

// Validate returns an error if any null-safety checks failed.
// Returns nil if all dependencies are valid.
func (c *NullSafetyChecker) Validate() error {
	if len(c.errors) == 0 {
		return nil
	}

	if len(c.errors) == 1 {
		return c.errors[0]
	}

	return fmt.Errorf("controller %q has %d nil dependencies: %w",
		c.controllerName, len(c.errors), errors.Join(c.errors...))
}

// HasErrors returns true if any null-safety checks failed.
func (c *NullSafetyChecker) HasErrors() bool {
	return len(c.errors) > 0
}

// Errors returns all recorded errors.
func (c *NullSafetyChecker) Errors() []error {
	return c.errors
}

// ValidateDependency is a convenience function for checking a single dependency.
// Returns a descriptive error if the dependency is nil.
func ValidateDependency(controllerName, dependencyName string, dependency interface{}) error {
	if dependency == nil {
		return NewDependencyError(controllerName, dependencyName)
	}
	return nil
}

// ValidateDependencies validates multiple dependencies at once.
// Returns an error listing all nil dependencies if any are found.
func ValidateDependencies(controllerName string, dependencies map[string]interface{}) error {
	checker := NewNullSafetyChecker(controllerName)
	for name, dep := range dependencies {
		checker.CheckNotNil(dep, name)
	}
	return checker.Validate()
}

// MustNotBeNil panics if the dependency is nil.
// Use this only during controller initialization where a nil dependency
// indicates a programming error that should fail fast.
func MustNotBeNil(dependency interface{}, name string) {
	if dependency == nil {
		panic(fmt.Sprintf("required dependency %q is nil", name))
	}
}
