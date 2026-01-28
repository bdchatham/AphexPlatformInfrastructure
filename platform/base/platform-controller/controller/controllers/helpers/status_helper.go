// Package helpers provides shared utility components for platform controllers.
// These components encapsulate common patterns like status updates, finalizer management,
// and dependency resolution to ensure consistent behavior across all controllers.
package helpers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bdchatham/AphexPlatformInfrastructure/platform/platform-controller/controller/controllers/constants"
)

// StatusHelper provides conflict-safe status updates for Kubernetes resources.
// It uses client-go's retry.RetryOnConflict for automatic retry on conflict errors
// and supports efficient merge patch operations for status updates.
type StatusHelper struct {
	client     client.Client
	logger     logr.Logger
	retryCount int
}

// NewStatusHelper creates a new StatusHelper with the given client and logger.
// The retryCount parameter specifies the maximum number of retry attempts for
// status updates that encounter conflict errors.
func NewStatusHelper(c client.Client, logger logr.Logger, retryCount int) *StatusHelper {
	if retryCount < 1 {
		retryCount = constants.DefaultStatusRetryCount
	}
	return &StatusHelper{
		client:     c,
		logger:     logger.WithName("StatusHelper"),
		retryCount: retryCount,
	}
}

// UpdateStatusWithRetry updates the status of a resource with automatic retry on conflict.
// The mutate function is called to modify the resource's status before each update attempt.
// On conflict, the resource is re-fetched and the mutate function is called again.
func (h *StatusHelper) UpdateStatusWithRetry(
	ctx context.Context,
	obj client.Object,
	mutate func() error,
) error {
	key := client.ObjectKeyFromObject(obj)
	attempt := 0

	retryBackoff := retry.DefaultRetry
	retryBackoff.Steps = h.retryCount

	err := retry.RetryOnConflict(retryBackoff, func() error {
		attempt++

		if attempt > 1 {
			h.logger.V(1).Info("Retrying status update after conflict",
				"resource", key.String(),
				"kind", obj.GetObjectKind().GroupVersionKind().Kind,
				"attempt", attempt,
				"maxAttempts", h.retryCount,
			)

			if err := h.client.Get(ctx, key, obj); err != nil {
				return fmt.Errorf("failed to refetch resource for retry: %w", err)
			}
		}

		if err := mutate(); err != nil {
			return fmt.Errorf("status mutation failed: %w", err)
		}

		if err := h.client.Status().Update(ctx, obj); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		h.logger.Error(err, "Status update failed after all retries",
			"resource", key.String(),
			"kind", obj.GetObjectKind().GroupVersionKind().Kind,
			"attempts", attempt,
		)
		return fmt.Errorf("status update failed after %d attempts: %w", attempt, err)
	}

	h.logger.V(1).Info("Status update succeeded",
		"resource", key.String(),
		"kind", obj.GetObjectKind().GroupVersionKind().Kind,
		"attempts", attempt,
	)

	return nil
}

// PatchStatus uses merge patch semantics for efficient status updates.
// This is more efficient than full status updates as it only sends the changed fields.
func (h *StatusHelper) PatchStatus(
	ctx context.Context,
	obj client.Object,
	statusPatch map[string]interface{},
) error {
	key := client.ObjectKeyFromObject(obj)

	patchData := map[string]interface{}{
		"status": statusPatch,
	}

	patchBytes, err := json.Marshal(patchData)
	if err != nil {
		return fmt.Errorf("failed to marshal status patch: %w", err)
	}

	patch := client.RawPatch(types.MergePatchType, patchBytes)

	if err := h.client.Status().Patch(ctx, obj, patch); err != nil {
		h.logger.Error(err, "Status patch failed",
			"resource", key.String(),
			"kind", obj.GetObjectKind().GroupVersionKind().Kind,
		)
		return fmt.Errorf("failed to patch status: %w", err)
	}

	h.logger.V(1).Info("Status patch succeeded",
		"resource", key.String(),
		"kind", obj.GetObjectKind().GroupVersionKind().Kind,
	)

	return nil
}

// PatchStatusWithRetry combines merge patch semantics with retry-on-conflict logic.
// This provides both efficiency and resilience for status updates.
func (h *StatusHelper) PatchStatusWithRetry(
	ctx context.Context,
	obj client.Object,
	buildPatch func() (map[string]interface{}, error),
) error {
	key := client.ObjectKeyFromObject(obj)
	attempt := 0

	retryBackoff := retry.DefaultRetry
	retryBackoff.Steps = h.retryCount

	err := retry.RetryOnConflict(retryBackoff, func() error {
		attempt++

		if attempt > 1 {
			h.logger.V(1).Info("Retrying status patch after conflict",
				"resource", key.String(),
				"kind", obj.GetObjectKind().GroupVersionKind().Kind,
				"attempt", attempt,
				"maxAttempts", h.retryCount,
			)

			if err := h.client.Get(ctx, key, obj); err != nil {
				return fmt.Errorf("failed to refetch resource for retry: %w", err)
			}
		}

		statusPatch, err := buildPatch()
		if err != nil {
			return fmt.Errorf("failed to build status patch: %w", err)
		}

		patchData := map[string]interface{}{
			"status": statusPatch,
		}

		patchBytes, err := json.Marshal(patchData)
		if err != nil {
			return fmt.Errorf("failed to marshal status patch: %w", err)
		}

		patch := client.RawPatch(types.MergePatchType, patchBytes)

		return h.client.Status().Patch(ctx, obj, patch)
	})

	if err != nil {
		h.logger.Error(err, "Status patch failed after all retries",
			"resource", key.String(),
			"kind", obj.GetObjectKind().GroupVersionKind().Kind,
			"attempts", attempt,
		)
		return fmt.Errorf("status patch failed after %d attempts: %w", attempt, err)
	}

	h.logger.V(1).Info("Status patch with retry succeeded",
		"resource", key.String(),
		"kind", obj.GetObjectKind().GroupVersionKind().Kind,
		"attempts", attempt,
	)

	return nil
}

// UpdatePhase is a convenience method for updating just the phase and message fields.
// It uses patch semantics for efficiency.
func (h *StatusHelper) UpdatePhase(
	ctx context.Context,
	obj client.Object,
	phase string,
	message string,
) error {
	return h.PatchStatus(ctx, obj, map[string]interface{}{
		"phase":   phase,
		"message": message,
	})
}

// UpdatePhaseWithRetry updates the phase and message with retry-on-conflict logic.
func (h *StatusHelper) UpdatePhaseWithRetry(
	ctx context.Context,
	obj client.Object,
	phase string,
	message string,
) error {
	return h.PatchStatusWithRetry(ctx, obj, func() (map[string]interface{}, error) {
		return map[string]interface{}{
			"phase":   phase,
			"message": message,
		}, nil
	})
}

