// Package helpers provides shared utility components for platform controllers.
// These components encapsulate common patterns like status updates, finalizer management,
// and dependency resolution to ensure consistent behavior across all controllers.
package helpers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// CleanupFunc is a function that performs cleanup operations for a resource.
// It should return nil if cleanup succeeds, or an error if cleanup fails.
// The cleanup function receives a context for cancellation support.
type CleanupFunc func(ctx context.Context) error

// FinalizerHelper manages finalizer operations with verified cleanup.
// It ensures finalizers are only removed after all cleanup operations succeed,
// preventing orphaned dependent resources.
type FinalizerHelper struct {
	client    client.Client
	logger    logr.Logger
	finalizer string
}

// NewFinalizerHelper creates a new FinalizerHelper with the given client, logger,
// and finalizer name. The finalizer name should be unique to the controller
// (e.g., "platform.aphex/repobinding-finalizer").
func NewFinalizerHelper(c client.Client, logger logr.Logger, finalizer string) *FinalizerHelper {
	return &FinalizerHelper{
		client:    c,
		logger:    logger.WithName("FinalizerHelper"),
		finalizer: finalizer,
	}
}

// EnsureFinalizer adds the finalizer to the resource if not already present.
// This should be called after validation passes to ensure invalid resources
// don't get stuck with finalizers.
func (h *FinalizerHelper) EnsureFinalizer(ctx context.Context, obj client.Object) error {
	key := client.ObjectKeyFromObject(obj)

	if controllerutil.ContainsFinalizer(obj, h.finalizer) {
		h.logger.V(1).Info("Finalizer already present",
			"resource", key.String(),
			"kind", obj.GetObjectKind().GroupVersionKind().Kind,
			"finalizer", h.finalizer,
		)
		return nil
	}

	h.logger.V(1).Info("Adding finalizer",
		"resource", key.String(),
		"kind", obj.GetObjectKind().GroupVersionKind().Kind,
		"finalizer", h.finalizer,
	)

	controllerutil.AddFinalizer(obj, h.finalizer)

	if err := h.client.Update(ctx, obj); err != nil {
		h.logger.Error(err, "Failed to add finalizer",
			"resource", key.String(),
			"kind", obj.GetObjectKind().GroupVersionKind().Kind,
			"finalizer", h.finalizer,
		)
		return fmt.Errorf("failed to add finalizer %s: %w", h.finalizer, err)
	}

	h.logger.Info("Finalizer added successfully",
		"resource", key.String(),
		"kind", obj.GetObjectKind().GroupVersionKind().Kind,
		"finalizer", h.finalizer,
	)

	return nil
}

// HasFinalizer returns true if the resource has the managed finalizer.
func (h *FinalizerHelper) HasFinalizer(obj client.Object) bool {
	return controllerutil.ContainsFinalizer(obj, h.finalizer)
}

// IsBeingDeleted returns true if the resource has a deletion timestamp set.
func (h *FinalizerHelper) IsBeingDeleted(obj client.Object) bool {
	return !obj.GetDeletionTimestamp().IsZero()
}

// NeedsCleanup returns true if the resource is being deleted and has the finalizer.
// This is a convenience method combining IsBeingDeleted and HasFinalizer.
func (h *FinalizerHelper) NeedsCleanup(obj client.Object) bool {
	return h.IsBeingDeleted(obj) && h.HasFinalizer(obj)
}

// HandleDeletion performs cleanup and removes the finalizer only after all
// cleanup operations succeed. If any cleanup operation fails, the finalizer
// is retained and an error is returned to trigger reconciliation requeue.
func (h *FinalizerHelper) HandleDeletion(
	ctx context.Context,
	obj client.Object,
	cleanupFn CleanupFunc,
) error {
	key := client.ObjectKeyFromObject(obj)

	if !h.HasFinalizer(obj) {
		h.logger.V(1).Info("No finalizer present, skipping cleanup",
			"resource", key.String(),
			"kind", obj.GetObjectKind().GroupVersionKind().Kind,
		)
		return nil
	}

	h.logger.Info("Starting cleanup for deletion",
		"resource", key.String(),
		"kind", obj.GetObjectKind().GroupVersionKind().Kind,
		"finalizer", h.finalizer,
	)

	if err := cleanupFn(ctx); err != nil {
		h.logger.Error(err, "Cleanup failed, retaining finalizer",
			"resource", key.String(),
			"kind", obj.GetObjectKind().GroupVersionKind().Kind,
			"finalizer", h.finalizer,
		)
		return fmt.Errorf("cleanup failed, finalizer retained: %w", err)
	}

	h.logger.Info("Cleanup succeeded, removing finalizer",
		"resource", key.String(),
		"kind", obj.GetObjectKind().GroupVersionKind().Kind,
		"finalizer", h.finalizer,
	)

	return h.removeFinalizer(ctx, obj)
}

// HandleDeletionWithSteps performs cleanup using multiple named steps and removes
// the finalizer only after all steps succeed. Each step is logged for progress
// tracking. If any step fails, cleanup stops and the finalizer is retained.
func (h *FinalizerHelper) HandleDeletionWithSteps(
	ctx context.Context,
	obj client.Object,
	steps []CleanupStep,
) error {
	key := client.ObjectKeyFromObject(obj)

	if !h.HasFinalizer(obj) {
		h.logger.V(1).Info("No finalizer present, skipping cleanup",
			"resource", key.String(),
			"kind", obj.GetObjectKind().GroupVersionKind().Kind,
		)
		return nil
	}

	h.logger.Info("Starting cleanup for deletion",
		"resource", key.String(),
		"kind", obj.GetObjectKind().GroupVersionKind().Kind,
		"finalizer", h.finalizer,
		"totalSteps", len(steps),
	)

	for i, step := range steps {
		stepNum := i + 1

		h.logger.V(1).Info("Executing cleanup step",
			"resource", key.String(),
			"kind", obj.GetObjectKind().GroupVersionKind().Kind,
			"step", stepNum,
			"totalSteps", len(steps),
			"stepName", step.Name,
		)

		if err := step.Cleanup(ctx); err != nil {
			h.logger.Error(err, "Cleanup step failed, retaining finalizer",
				"resource", key.String(),
				"kind", obj.GetObjectKind().GroupVersionKind().Kind,
				"step", stepNum,
				"totalSteps", len(steps),
				"stepName", step.Name,
				"finalizer", h.finalizer,
			)
			return fmt.Errorf("cleanup step %q failed: %w", step.Name, err)
		}

		h.logger.Info("Cleanup step completed",
			"resource", key.String(),
			"kind", obj.GetObjectKind().GroupVersionKind().Kind,
			"step", stepNum,
			"totalSteps", len(steps),
			"stepName", step.Name,
		)
	}

	h.logger.Info("All cleanup steps succeeded, removing finalizer",
		"resource", key.String(),
		"kind", obj.GetObjectKind().GroupVersionKind().Kind,
		"finalizer", h.finalizer,
		"completedSteps", len(steps),
	)

	return h.removeFinalizer(ctx, obj)
}

// removeFinalizer removes the finalizer from the resource and updates it.
func (h *FinalizerHelper) removeFinalizer(ctx context.Context, obj client.Object) error {
	key := client.ObjectKeyFromObject(obj)

	controllerutil.RemoveFinalizer(obj, h.finalizer)

	if err := h.client.Update(ctx, obj); err != nil {
		h.logger.Error(err, "Failed to remove finalizer after successful cleanup",
			"resource", key.String(),
			"kind", obj.GetObjectKind().GroupVersionKind().Kind,
			"finalizer", h.finalizer,
		)
		return fmt.Errorf("failed to remove finalizer %s: %w", h.finalizer, err)
	}

	h.logger.Info("Finalizer removed successfully",
		"resource", key.String(),
		"kind", obj.GetObjectKind().GroupVersionKind().Kind,
		"finalizer", h.finalizer,
	)

	return nil
}

// CleanupStep represents a named cleanup operation for structured cleanup logging.
type CleanupStep struct {
	Name    string
	Cleanup CleanupFunc
}

// NewCleanupStep creates a new CleanupStep with the given name and cleanup function.
func NewCleanupStep(name string, cleanup CleanupFunc) CleanupStep {
	return CleanupStep{
		Name:    name,
		Cleanup: cleanup,
	}
}
