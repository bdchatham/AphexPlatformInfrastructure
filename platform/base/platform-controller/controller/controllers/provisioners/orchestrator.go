// Package provisioners provides standardized interfaces and utilities for resource provisioning.
package provisioners

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bdchatham/AphexPlatformInfrastructure/platform/platform-controller/controller/controllers/constants"
)

// Orchestrator coordinates provisioning operations with timeout, rollback, and tracking.
// It provides a high-level API for executing provisioning steps with production-ready
// error handling and cleanup.
type Orchestrator struct {
	client          client.Client
	logger          logr.Logger
	timeout         time.Duration
	timeoutRunner   *TimeoutRunner
	rollbackManager *RollbackManager
}

// NewOrchestrator creates a new provisioning Orchestrator.
func NewOrchestrator(c client.Client, logger logr.Logger) *Orchestrator {
	timeout := constants.DefaultProvisioningTimeout
	return &Orchestrator{
		client:          c,
		logger:          logger.WithName("ProvisioningOrchestrator"),
		timeout:         timeout,
		timeoutRunner:   NewTimeoutRunner(timeout, logger),
		rollbackManager: NewRollbackManager(c, logger),
	}
}

// WithTimeout returns a new Orchestrator with a custom timeout.
func (o *Orchestrator) WithTimeout(timeout time.Duration) *Orchestrator {
	return &Orchestrator{
		client:          o.client,
		logger:          o.logger,
		timeout:         timeout,
		timeoutRunner:   NewTimeoutRunner(timeout, o.logger),
		rollbackManager: o.rollbackManager,
	}
}

// ProvisionResult contains the result of a provisioning operation.
type ProvisionResult struct {
	Success          bool
	Error            error
	Duration         time.Duration
	CreatedResources []ResourceRef
	FailedStep       string
	RollbackError    *RollbackError
}

// Execute runs a series of provisioning steps with timeout and rollback support.
// If any step fails, it attempts to rollback all previously created resources.
func (o *Orchestrator) Execute(
	ctx context.Context,
	owner client.Object,
	steps []ProvisioningStep,
) *ProvisionResult {
	pc := NewProvisioningContext(o.logger)
	result := &ProvisionResult{
		Success: false,
	}

	o.logger.Info("Starting provisioning",
		"owner", client.ObjectKeyFromObject(owner).String(),
		"ownerKind", owner.GetObjectKind().GroupVersionKind().Kind,
		"stepCount", len(steps),
		"timeout", o.timeout,
	)

	timeoutCtx, cancel := context.WithTimeout(ctx, o.timeout)
	defer cancel()

	for i, step := range steps {
		if err := CheckContextCancellation(timeoutCtx, step.Name); err != nil {
			result.Error = err
			result.FailedStep = step.Name
			result.Duration = pc.Duration()
			result.CreatedResources = pc.GetCreatedResources()

			o.logger.Error(err, "Context cancelled before step",
				"step", i+1,
				"stepName", step.Name,
			)

			result.RollbackError = o.rollbackManager.Rollback(ctx, err, pc)
			return result
		}

		pc.SetStep(step.Name)

		o.logger.V(1).Info("Executing provisioning step",
			"step", i+1,
			"totalSteps", len(steps),
			"stepName", step.Name,
		)

		if err := o.timeoutRunner.RunStep(timeoutCtx, step); err != nil {
			result.Error = err
			result.FailedStep = step.Name
			result.Duration = pc.Duration()
			result.CreatedResources = pc.GetCreatedResources()

			o.logger.Error(err, "Provisioning step failed",
				"step", i+1,
				"stepName", step.Name,
				"duration", pc.Duration(),
			)

			result.RollbackError = o.rollbackManager.Rollback(ctx, err, pc)
			return result
		}
	}

	result.Success = true
	result.Duration = pc.Duration()
	result.CreatedResources = pc.GetCreatedResources()

	o.logger.Info("Provisioning completed successfully",
		"owner", client.ObjectKeyFromObject(owner).String(),
		"duration", result.Duration,
		"resourcesCreated", len(result.CreatedResources),
	)

	return result
}

// ExecuteWithTracking runs provisioning steps and tracks created resources.
// The trackFn is called after each successful step to record created resources.
func (o *Orchestrator) ExecuteWithTracking(
	ctx context.Context,
	owner client.Object,
	steps []TrackedProvisioningStep,
) *ProvisionResult {
	pc := NewProvisioningContext(o.logger)
	result := &ProvisionResult{
		Success: false,
	}

	o.logger.Info("Starting provisioning with tracking",
		"owner", client.ObjectKeyFromObject(owner).String(),
		"ownerKind", owner.GetObjectKind().GroupVersionKind().Kind,
		"stepCount", len(steps),
		"timeout", o.timeout,
	)

	timeoutCtx, cancel := context.WithTimeout(ctx, o.timeout)
	defer cancel()

	for i, step := range steps {
		if err := CheckContextCancellation(timeoutCtx, step.Name); err != nil {
			result.Error = err
			result.FailedStep = step.Name
			result.Duration = pc.Duration()
			result.CreatedResources = pc.GetCreatedResources()

			o.logger.Error(err, "Context cancelled before step",
				"step", i+1,
				"stepName", step.Name,
			)

			result.RollbackError = o.rollbackManager.Rollback(ctx, err, pc)
			return result
		}

		pc.SetStep(step.Name)

		o.logger.V(1).Info("Executing tracked provisioning step",
			"step", i+1,
			"totalSteps", len(steps),
			"stepName", step.Name,
		)

		created, err := step.Fn(timeoutCtx)
		if err != nil {
			result.Error = err
			result.FailedStep = step.Name
			result.Duration = pc.Duration()
			result.CreatedResources = pc.GetCreatedResources()

			o.logger.Error(err, "Provisioning step failed",
				"step", i+1,
				"stepName", step.Name,
				"duration", pc.Duration(),
			)

			result.RollbackError = o.rollbackManager.Rollback(ctx, err, pc)
			return result
		}

		for _, obj := range created {
			pc.RecordCreated(obj)
		}
	}

	result.Success = true
	result.Duration = pc.Duration()
	result.CreatedResources = pc.GetCreatedResources()

	o.logger.Info("Provisioning with tracking completed successfully",
		"owner", client.ObjectKeyFromObject(owner).String(),
		"duration", result.Duration,
		"resourcesCreated", len(result.CreatedResources),
	)

	return result
}

// TrackedProvisioningStep is a provisioning step that returns created resources.
type TrackedProvisioningStep struct {
	Name string
	Fn   func(ctx context.Context) ([]client.Object, error)
}

// FormatStatusMessage creates a status message from a ProvisionResult.
func (r *ProvisionResult) FormatStatusMessage() string {
	if r.Success {
		return fmt.Sprintf("Provisioning completed successfully in %v", r.Duration)
	}

	msg := fmt.Sprintf("Provisioning failed at step %q: %v", r.FailedStep, r.Error)

	if r.RollbackError != nil && r.RollbackError.HasCleanupFailures() {
		msg += "; " + r.RollbackError.FormatOrphanedResources()
	}

	return msg
}

// GetIdempotentHelper returns an IdempotentHelper using the orchestrator's client.
func (o *Orchestrator) GetIdempotentHelper() *IdempotentHelper {
	return NewIdempotentHelper(o.client, o.logger)
}
