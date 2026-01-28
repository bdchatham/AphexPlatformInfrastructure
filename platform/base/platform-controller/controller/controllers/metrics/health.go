// Package metrics provides health state tracking for controller health checks.
package metrics

import (
	"sync"
	"time"

	"github.com/bdchatham/AphexPlatformInfrastructure/platform/platform-controller/controller/controllers/constants"
)

// HealthState tracks controller health for health checks.
// It records successful reconciliations and reports unhealthy if no recent success.
type HealthState struct {
	mu                      sync.RWMutex
	lastSuccessfulReconcile map[string]time.Time
	healthThreshold         time.Duration
}

var (
	healthInstance *HealthState
	healthOnce     sync.Once
)

// GetHealthState returns the singleton HealthState instance.
func GetHealthState() *HealthState {
	healthOnce.Do(func() {
		healthInstance = NewHealthState(constants.DefaultHealthThreshold)
	})
	return healthInstance
}

// NewHealthState creates a new HealthState with the specified threshold.
func NewHealthState(threshold time.Duration) *HealthState {
	return &HealthState{
		lastSuccessfulReconcile: make(map[string]time.Time),
		healthThreshold:         threshold,
	}
}

// RecordSuccess records a successful reconciliation for a controller.
func (h *HealthState) RecordSuccess(controller string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastSuccessfulReconcile[controller] = time.Now()
}

// IsHealthy returns true if all registered controllers have had recent successful reconciliations.
// A controller is considered healthy if it has reconciled successfully within the health threshold.
func (h *HealthState) IsHealthy() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.lastSuccessfulReconcile) == 0 {
		return true
	}

	now := time.Now()
	for _, lastSuccess := range h.lastSuccessfulReconcile {
		if now.Sub(lastSuccess) > h.healthThreshold {
			return false
		}
	}
	return true
}

// IsControllerHealthy returns true if a specific controller has had a recent successful reconciliation.
func (h *HealthState) IsControllerHealthy(controller string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	lastSuccess, exists := h.lastSuccessfulReconcile[controller]
	if !exists {
		return true
	}

	return time.Since(lastSuccess) <= h.healthThreshold
}

// GetLastSuccess returns the last successful reconciliation time for a controller.
// Returns zero time if the controller has never reconciled successfully.
func (h *HealthState) GetLastSuccess(controller string) time.Time {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.lastSuccessfulReconcile[controller]
}

// GetHealthThreshold returns the configured health threshold.
func (h *HealthState) GetHealthThreshold() time.Duration {
	return h.healthThreshold
}

// SetHealthThreshold updates the health threshold.
func (h *HealthState) SetHealthThreshold(threshold time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.healthThreshold = threshold
}

// GetControllerStatus returns a map of controller names to their health status.
func (h *HealthState) GetControllerStatus() map[string]bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	status := make(map[string]bool)
	now := time.Now()
	for controller, lastSuccess := range h.lastSuccessfulReconcile {
		status[controller] = now.Sub(lastSuccess) <= h.healthThreshold
	}
	return status
}

// Reset clears all recorded reconciliation times.
// This is primarily useful for testing.
func (h *HealthState) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastSuccessfulReconcile = make(map[string]time.Time)
}
