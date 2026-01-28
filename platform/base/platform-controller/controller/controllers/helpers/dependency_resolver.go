// Package helpers provides shared utility components for platform controllers.
// These components encapsulate common patterns like status updates, finalizer management,
// and dependency resolution to ensure consistent behavior across all controllers.
package helpers

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bdchatham/AphexPlatformInfrastructure/platform/platform-controller/controller/controllers/constants"
)

// DependencyResult represents the outcome of a dependency resolution attempt.
type DependencyResult struct {
	// Found indicates whether the dependency was found.
	Found bool
	// Result is the controller result to return when the dependency is not found.
	// This includes the RequeueAfter duration for graceful waiting.
	Result ctrl.Result
	// Message is a descriptive message about the dependency status.
	Message string
}

// DependencyResolver provides graceful waiting for cross-resource dependencies.
type DependencyResolver struct {
	client       client.Client
	logger       logr.Logger
	waitDuration time.Duration
}

// NewDependencyResolver creates a new DependencyResolver with the given client and logger.
// The waitDuration parameter specifies how long to wait before requeuing when a
// dependency is not found. If zero, the default of 30 seconds is used.
func NewDependencyResolver(c client.Client, logger logr.Logger, waitDuration time.Duration) *DependencyResolver {
	if waitDuration == 0 {
		waitDuration = constants.DefaultDependencyWaitTime
	}
	return &DependencyResolver{
		client:       c,
		logger:       logger.WithName("DependencyResolver"),
		waitDuration: waitDuration,
	}
}

// ResolveDependency attempts to fetch a dependency and returns a DependencyResult.
// If the dependency is found, it populates the provided object and returns Found=true.
// If the dependency is not found, it returns Found=false with a RequeueAfter result.
// Other errors are returned as-is for the caller to handle.
func (r *DependencyResolver) ResolveDependency(
	ctx context.Context,
	key client.ObjectKey,
	obj client.Object,
	dependencyKind string,
) (DependencyResult, error) {
	r.logger.V(1).Info("Resolving dependency",
		"kind", dependencyKind,
		"name", key.Name,
		"namespace", key.Namespace,
	)

	err := r.client.Get(ctx, key, obj)
	if err == nil {
		r.logger.V(1).Info("Dependency found",
			"kind", dependencyKind,
			"name", key.Name,
			"namespace", key.Namespace,
		)
		return DependencyResult{
			Found:   true,
			Message: fmt.Sprintf("%s %s found", dependencyKind, key.Name),
		}, nil
	}

	if errors.IsNotFound(err) {
		message := fmt.Sprintf("Waiting for %s %q to be created", dependencyKind, key.Name)
		if key.Namespace != "" {
			message = fmt.Sprintf("Waiting for %s %q in namespace %q to be created",
				dependencyKind, key.Name, key.Namespace)
		}

		r.logger.Info("Dependency not found, will requeue",
			"kind", dependencyKind,
			"name", key.Name,
			"namespace", key.Namespace,
			"requeueAfter", r.waitDuration,
		)

		return DependencyResult{
			Found: false,
			Result: ctrl.Result{
				RequeueAfter: r.waitDuration,
			},
			Message: message,
		}, nil
	}

	r.logger.Error(err, "Failed to resolve dependency",
		"kind", dependencyKind,
		"name", key.Name,
		"namespace", key.Namespace,
	)
	return DependencyResult{}, fmt.Errorf("failed to get %s %q: %w", dependencyKind, key.Name, err)
}

// ResolveMultipleDependencies resolves multiple dependencies and returns the first
// missing dependency result. If all dependencies are found, returns Found=true.
// This is useful when a resource depends on multiple other resources.
func (r *DependencyResolver) ResolveMultipleDependencies(
	ctx context.Context,
	dependencies []DependencySpec,
) (DependencyResult, error) {
	for _, dep := range dependencies {
		result, err := r.ResolveDependency(ctx, dep.Key, dep.Object, dep.Kind)
		if err != nil {
			return DependencyResult{}, err
		}
		if !result.Found {
			return result, nil
		}
	}

	return DependencyResult{
		Found:   true,
		Message: "All dependencies resolved",
	}, nil
}

// DependencySpec describes a dependency to be resolved.
type DependencySpec struct {
	// Key is the namespaced name of the dependency.
	Key client.ObjectKey
	// Object is the target object to populate when the dependency is found.
	Object client.Object
	// Kind is a human-readable description of the dependency kind for logging.
	Kind string
}

// NewDependencySpec creates a new DependencySpec with the given parameters.
func NewDependencySpec(key client.ObjectKey, obj client.Object, kind string) DependencySpec {
	return DependencySpec{
		Key:    key,
		Object: obj,
		Kind:   kind,
	}
}

// WaitingForDependency is a convenience method that returns true if the result
// indicates the controller should wait for a dependency.
func (r DependencyResult) WaitingForDependency() bool {
	return !r.Found && r.Result.RequeueAfter > 0
}

// PendingMessage returns a formatted message suitable for updating resource status
// to the Pending phase when waiting for dependencies.
func (r DependencyResult) PendingMessage() string {
	return r.Message
}
