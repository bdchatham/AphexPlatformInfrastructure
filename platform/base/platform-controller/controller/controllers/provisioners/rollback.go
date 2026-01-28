// Package provisioners provides standardized interfaces and utilities for resource provisioning.
package provisioners

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RollbackError represents a failure during rollback cleanup.
type RollbackError struct {
	OriginalError   error
	CleanupFailures []CleanupFailure
}

func (e *RollbackError) Error() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("provisioning failed: %v", e.OriginalError))
	if len(e.CleanupFailures) > 0 {
		sb.WriteString(fmt.Sprintf("; rollback had %d cleanup failures: ", len(e.CleanupFailures)))
		for i, f := range e.CleanupFailures {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(fmt.Sprintf("%s/%s: %v", f.Resource.Kind, f.Resource.Name, f.Error))
		}
	}
	return sb.String()
}

// Unwrap returns the original error for error chain support.
func (e *RollbackError) Unwrap() error {
	return e.OriginalError
}

// CleanupFailure represents a single resource cleanup failure.
type CleanupFailure struct {
	Resource ResourceRef
	Error    error
}

// RollbackManager handles cleanup of created resources on provisioning failure.
type RollbackManager struct {
	client client.Client
	logger logr.Logger
}

// NewRollbackManager creates a new RollbackManager with the given client and logger.
func NewRollbackManager(c client.Client, logger logr.Logger) *RollbackManager {
	return &RollbackManager{
		client: c,
		logger: logger.WithName("RollbackManager"),
	}
}

// Rollback attempts to clean up all resources tracked in the ProvisioningContext.
// It continues cleaning up even if individual deletions fail.
// Returns a RollbackError containing the original error and any cleanup failures.
func (rm *RollbackManager) Rollback(
	ctx context.Context,
	provisioningErr error,
	pc *ProvisioningContext,
) *RollbackError {
	resources := pc.GetCreatedResources()

	if len(resources) == 0 {
		rm.logger.V(1).Info("No resources to rollback")
		return &RollbackError{
			OriginalError:   provisioningErr,
			CleanupFailures: nil,
		}
	}

	rm.logger.Info("Starting rollback of created resources",
		"resourceCount", len(resources),
		"failedStep", pc.GetCurrentStep(),
		"originalError", provisioningErr.Error(),
	)

	var cleanupFailures []CleanupFailure

	for i := len(resources) - 1; i >= 0; i-- {
		ref := resources[i]

		rm.logger.V(1).Info("Rolling back resource",
			"kind", ref.Kind,
			"name", ref.Name,
			"namespace", ref.Namespace,
		)

		if err := rm.deleteResource(ctx, ref); err != nil {
			rm.logger.Error(err, "Failed to rollback resource",
				"kind", ref.Kind,
				"name", ref.Name,
				"namespace", ref.Namespace,
			)
			cleanupFailures = append(cleanupFailures, CleanupFailure{
				Resource: ref,
				Error:    err,
			})
		} else {
			rm.logger.Info("Successfully rolled back resource",
				"kind", ref.Kind,
				"name", ref.Name,
				"namespace", ref.Namespace,
			)
		}
	}

	if len(cleanupFailures) > 0 {
		rm.logger.Error(nil, "Rollback completed with failures",
			"totalResources", len(resources),
			"failedCleanups", len(cleanupFailures),
		)
	} else {
		rm.logger.Info("Rollback completed successfully",
			"totalResources", len(resources),
		)
	}

	return &RollbackError{
		OriginalError:   provisioningErr,
		CleanupFailures: cleanupFailures,
	}
}

// deleteResource deletes a resource by its reference.
func (rm *RollbackManager) deleteResource(ctx context.Context, ref ResourceRef) error {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion(ref.APIVersion)
	obj.SetKind(ref.Kind)
	obj.SetName(ref.Name)
	obj.SetNamespace(ref.Namespace)

	if err := rm.client.Delete(ctx, obj); err != nil {
		if errors.IsNotFound(err) {
			rm.logger.V(1).Info("Resource already deleted during rollback",
				"kind", ref.Kind,
				"name", ref.Name,
				"namespace", ref.Namespace,
			)
			return nil
		}
		return fmt.Errorf("failed to delete %s %s/%s: %w", ref.Kind, ref.Namespace, ref.Name, err)
	}

	return nil
}

// HasCleanupFailures returns true if the RollbackError has cleanup failures.
func (e *RollbackError) HasCleanupFailures() bool {
	return len(e.CleanupFailures) > 0
}

// OrphanedResources returns the list of resources that could not be cleaned up.
func (e *RollbackError) OrphanedResources() []ResourceRef {
	refs := make([]ResourceRef, len(e.CleanupFailures))
	for i, f := range e.CleanupFailures {
		refs[i] = f.Resource
	}
	return refs
}

// FormatOrphanedResources returns a human-readable string of orphaned resources.
func (e *RollbackError) FormatOrphanedResources() string {
	if len(e.CleanupFailures) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("Orphaned resources: ")
	for i, f := range e.CleanupFailures {
		if i > 0 {
			sb.WriteString(", ")
		}
		if f.Resource.Namespace != "" {
			sb.WriteString(fmt.Sprintf("%s/%s/%s", f.Resource.Kind, f.Resource.Namespace, f.Resource.Name))
		} else {
			sb.WriteString(fmt.Sprintf("%s/%s", f.Resource.Kind, f.Resource.Name))
		}
	}
	return sb.String()
}
