// Package provisioners provides standardized interfaces and utilities for resource provisioning.
// It implements idempotent create-or-update patterns with spec comparison, rollback support,
// and timeout handling for production-ready controller operations.
package provisioners

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bdchatham/AphexPlatformInfrastructure/platform/platform-controller/controller/controllers/constants"
)

// Provisioner defines the interface for resource provisioning operations.
// Implementations must be idempotent - calling Provision multiple times
// with the same input should produce the same result without side effects.
type Provisioner interface {
	// Provision creates or updates resources idempotently.
	// Returns nil if provisioning succeeds or resources already exist with correct spec.
	Provision(ctx context.Context, owner client.Object) error

	// Cleanup removes resources created by this provisioner.
	// Should be idempotent - calling Cleanup when resources don't exist should succeed.
	Cleanup(ctx context.Context, owner client.Object) error

	// Name returns the provisioner name for logging and metrics.
	Name() string
}

// ProvisioningContext tracks provisioning state for rollback support.
// It records created resources so they can be cleaned up on failure.
type ProvisioningContext struct {
	mu               sync.Mutex
	CreatedResources []ResourceRef
	StartTime        time.Time
	CurrentStep      string
	Logger           logr.Logger
}

// ResourceRef identifies a Kubernetes resource for tracking.
type ResourceRef struct {
	APIVersion string
	Kind       string
	Name       string
	Namespace  string
}

// NewProvisioningContext creates a new ProvisioningContext with the given logger.
func NewProvisioningContext(logger logr.Logger) *ProvisioningContext {
	return &ProvisioningContext{
		CreatedResources: make([]ResourceRef, 0),
		StartTime:        time.Now(),
		Logger:           logger.WithName("ProvisioningContext"),
	}
}

// SetStep updates the current provisioning step for logging and error reporting.
func (pc *ProvisioningContext) SetStep(step string) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.CurrentStep = step
	pc.Logger.V(1).Info("Starting provisioning step", "step", step)
}

// RecordCreated adds a resource to the list of created resources for rollback tracking.
func (pc *ProvisioningContext) RecordCreated(obj client.Object) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	ref := ResourceRef{
		APIVersion: obj.GetObjectKind().GroupVersionKind().GroupVersion().String(),
		Kind:       obj.GetObjectKind().GroupVersionKind().Kind,
		Name:       obj.GetName(),
		Namespace:  obj.GetNamespace(),
	}

	pc.CreatedResources = append(pc.CreatedResources, ref)
	pc.Logger.V(1).Info("Recorded created resource",
		"kind", ref.Kind,
		"name", ref.Name,
		"namespace", ref.Namespace,
	)
}

// GetCreatedResources returns a copy of the created resources list.
func (pc *ProvisioningContext) GetCreatedResources() []ResourceRef {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	result := make([]ResourceRef, len(pc.CreatedResources))
	copy(result, pc.CreatedResources)
	return result
}

// Duration returns the elapsed time since provisioning started.
func (pc *ProvisioningContext) Duration() time.Duration {
	return time.Since(pc.StartTime)
}

// GetCurrentStep returns the current provisioning step.
func (pc *ProvisioningContext) GetCurrentStep() string {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	return pc.CurrentStep
}


// IdempotentHelper provides utilities for idempotent create-or-update operations.
// It compares desired specs with existing specs to avoid unnecessary updates.
type IdempotentHelper struct {
	client client.Client
	logger logr.Logger
}

// NewIdempotentHelper creates a new IdempotentHelper with the given client and logger.
func NewIdempotentHelper(c client.Client, logger logr.Logger) *IdempotentHelper {
	return &IdempotentHelper{
		client: c,
		logger: logger.WithName("IdempotentHelper"),
	}
}

// CreateOrUpdateResult indicates the outcome of a create-or-update operation.
type CreateOrUpdateResult int

const (
	// ResultCreated indicates a new resource was created.
	ResultCreated CreateOrUpdateResult = iota
	// ResultUpdated indicates an existing resource was updated.
	ResultUpdated
	// ResultUnchanged indicates no changes were needed.
	ResultUnchanged
)

func (r CreateOrUpdateResult) String() string {
	switch r {
	case ResultCreated:
		return "created"
	case ResultUpdated:
		return "updated"
	case ResultUnchanged:
		return "unchanged"
	default:
		return "unknown"
	}
}

// CreateOrUpdate performs an idempotent create-or-update operation.
// It compares the desired spec with the existing spec using a hash annotation.
// If specs match, no update is performed.
func (h *IdempotentHelper) CreateOrUpdate(
	ctx context.Context,
	desired client.Object,
	specGetter func(client.Object) interface{},
) (CreateOrUpdateResult, error) {
	key := client.ObjectKeyFromObject(desired)
	kind := desired.GetObjectKind().GroupVersionKind().Kind

	specHash, err := computeSpecHash(specGetter(desired))
	if err != nil {
		return ResultUnchanged, fmt.Errorf("failed to compute spec hash: %w", err)
	}

	setSpecHashAnnotation(desired, specHash)

	existing := desired.DeepCopyObject().(client.Object)
	err = h.client.Get(ctx, key, existing)

	if err != nil {
		if errors.IsNotFound(err) {
			h.logger.Info("Creating resource",
				"kind", kind,
				"name", key.Name,
				"namespace", key.Namespace,
			)

			if err := h.client.Create(ctx, desired); err != nil {
				return ResultUnchanged, fmt.Errorf("failed to create %s %s: %w", kind, key.String(), err)
			}

			return ResultCreated, nil
		}
		return ResultUnchanged, fmt.Errorf("failed to get %s %s: %w", kind, key.String(), err)
	}

	existingHash := getSpecHashAnnotation(existing)
	if existingHash == specHash {
		h.logger.V(1).Info("Resource unchanged, skipping update",
			"kind", kind,
			"name", key.Name,
			"namespace", key.Namespace,
		)
		return ResultUnchanged, nil
	}

	desired.SetResourceVersion(existing.GetResourceVersion())

	h.logger.Info("Updating resource",
		"kind", kind,
		"name", key.Name,
		"namespace", key.Namespace,
		"reason", "spec changed",
	)

	if err := h.client.Update(ctx, desired); err != nil {
		return ResultUnchanged, fmt.Errorf("failed to update %s %s: %w", kind, key.String(), err)
	}

	return ResultUpdated, nil
}

// CreateIfNotExists creates a resource only if it doesn't already exist.
// Returns ResultCreated if created, ResultUnchanged if already exists.
func (h *IdempotentHelper) CreateIfNotExists(
	ctx context.Context,
	desired client.Object,
) (CreateOrUpdateResult, error) {
	key := client.ObjectKeyFromObject(desired)
	kind := desired.GetObjectKind().GroupVersionKind().Kind

	existing := desired.DeepCopyObject().(client.Object)
	err := h.client.Get(ctx, key, existing)

	if err != nil {
		if errors.IsNotFound(err) {
			h.logger.Info("Creating resource",
				"kind", kind,
				"name", key.Name,
				"namespace", key.Namespace,
			)

			if err := h.client.Create(ctx, desired); err != nil {
				return ResultUnchanged, fmt.Errorf("failed to create %s %s: %w", kind, key.String(), err)
			}

			return ResultCreated, nil
		}
		return ResultUnchanged, fmt.Errorf("failed to get %s %s: %w", kind, key.String(), err)
	}

	h.logger.V(1).Info("Resource already exists",
		"kind", kind,
		"name", key.Name,
		"namespace", key.Namespace,
	)

	return ResultUnchanged, nil
}

// Delete removes a resource if it exists.
// Returns nil if the resource doesn't exist (idempotent).
func (h *IdempotentHelper) Delete(ctx context.Context, obj client.Object) error {
	key := client.ObjectKeyFromObject(obj)
	kind := obj.GetObjectKind().GroupVersionKind().Kind

	if err := h.client.Delete(ctx, obj); err != nil {
		if errors.IsNotFound(err) {
			h.logger.V(1).Info("Resource already deleted",
				"kind", kind,
				"name", key.Name,
				"namespace", key.Namespace,
			)
			return nil
		}
		return fmt.Errorf("failed to delete %s %s: %w", kind, key.String(), err)
	}

	h.logger.Info("Deleted resource",
		"kind", kind,
		"name", key.Name,
		"namespace", key.Namespace,
	)

	return nil
}

// computeSpecHash computes a SHA256 hash of the spec for comparison.
func computeSpecHash(spec interface{}) (string, error) {
	data, err := json.Marshal(spec)
	if err != nil {
		return "", fmt.Errorf("failed to marshal spec: %w", err)
	}

	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash[:8]), nil
}

// setSpecHashAnnotation sets the spec hash annotation on an object.
func setSpecHashAnnotation(obj client.Object, hash string) {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[constants.AnnotationLastAppliedSpec] = hash
	obj.SetAnnotations(annotations)
}

// getSpecHashAnnotation gets the spec hash annotation from an object.
func getSpecHashAnnotation(obj client.Object) string {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return ""
	}
	return annotations[constants.AnnotationLastAppliedSpec]
}

// SetOwnerReference sets an owner reference on the dependent object.
// This enables automatic garbage collection when the owner is deleted.
func SetOwnerReference(owner, dependent client.Object) error {
	ownerGVK := owner.GetObjectKind().GroupVersionKind()
	if ownerGVK.Empty() {
		return fmt.Errorf("owner GVK is empty, ensure SetGroupVersionKind is called")
	}

	ownerRef := metav1.OwnerReference{
		APIVersion: ownerGVK.GroupVersion().String(),
		Kind:       ownerGVK.Kind,
		Name:       owner.GetName(),
		UID:        owner.GetUID(),
	}

	existingRefs := dependent.GetOwnerReferences()
	for i, ref := range existingRefs {
		if ref.UID == owner.GetUID() {
			existingRefs[i] = ownerRef
			dependent.SetOwnerReferences(existingRefs)
			return nil
		}
	}

	dependent.SetOwnerReferences(append(existingRefs, ownerRef))
	return nil
}

// SetClusterScopedOwnerLabels sets labels on cluster-scoped resources to track ownership.
// Since cluster-scoped resources cannot have owner references, we use labels instead.
func SetClusterScopedOwnerLabels(obj client.Object, ownerName, ownerNamespace, ownerKind string) {
	labels := obj.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}

	labels[constants.LabelOwnerName] = ownerName
	labels[constants.LabelOwnerKind] = ownerKind
	labels[constants.LabelManagedBy] = constants.ManagedByPlatformController

	if ownerNamespace != "" {
		labels["platform.aphex/owner-namespace"] = ownerNamespace
	}

	obj.SetLabels(labels)
}
