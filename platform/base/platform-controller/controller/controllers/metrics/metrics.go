// Package metrics provides Prometheus metrics for controller observability.
package metrics

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

// MetricsCollector manages custom controller metrics for observability.
// It exposes reconciliation duration, error counts, and provisioning step metrics.
type MetricsCollector struct {
	reconcileDuration *prometheus.HistogramVec
	reconcileErrors   *prometheus.CounterVec
	provisioningSteps *prometheus.CounterVec
	queueDepth        *prometheus.GaugeVec
}

var (
	instance *MetricsCollector
	once     sync.Once
)

// GetMetricsCollector returns the singleton MetricsCollector instance.
// The metrics are registered with controller-runtime's metrics registry on first call.
func GetMetricsCollector() *MetricsCollector {
	once.Do(func() {
		instance = newMetricsCollector()
		instance.register()
	})
	return instance
}

// newMetricsCollector creates a new MetricsCollector with all metrics initialized.
func newMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		reconcileDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "platform_controller_reconcile_duration_seconds",
				Help:    "Duration of reconciliation operations in seconds",
				Buckets: prometheus.ExponentialBuckets(0.01, 2, 14),
			},
			[]string{"controller", "result"},
		),
		reconcileErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "platform_controller_reconcile_errors_total",
				Help: "Total number of reconciliation errors",
			},
			[]string{"controller", "error_type"},
		),
		provisioningSteps: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "platform_controller_provisioning_steps_total",
				Help: "Total number of provisioning steps executed",
			},
			[]string{"controller", "step", "result"},
		),
		queueDepth: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "platform_controller_queue_depth",
				Help: "Current depth of the reconciliation queue",
			},
			[]string{"controller"},
		),
	}
}

// register registers all metrics with the controller-runtime metrics registry.
func (m *MetricsCollector) register() {
	metrics.Registry.MustRegister(
		m.reconcileDuration,
		m.reconcileErrors,
		m.provisioningSteps,
		m.queueDepth,
	)
}

// RecordReconcileDuration records the duration of a reconciliation operation.
// controller: the name of the controller (e.g., "RepoBinding", "Organization")
// duration: the time taken for the reconciliation
// result: "success" or "error"
func (m *MetricsCollector) RecordReconcileDuration(controller string, duration time.Duration, result string) {
	m.reconcileDuration.WithLabelValues(controller, result).Observe(duration.Seconds())
}

// RecordReconcileError increments the error counter for a specific controller and error type.
// controller: the name of the controller
// errorType: categorized error type (e.g., "validation", "timeout", "conflict", "permission", "unknown")
func (m *MetricsCollector) RecordReconcileError(controller string, errorType string) {
	m.reconcileErrors.WithLabelValues(controller, errorType).Inc()
}

// RecordProvisioningStep records a provisioning step execution.
// controller: the name of the controller
// step: the name of the provisioning step (e.g., "namespace", "pipeline", "rbac")
// result: "success" or "error"
func (m *MetricsCollector) RecordProvisioningStep(controller string, step string, result string) {
	m.provisioningSteps.WithLabelValues(controller, step, result).Inc()
}

// SetQueueDepth sets the current queue depth for a controller.
// controller: the name of the controller
// depth: the current number of items in the queue
func (m *MetricsCollector) SetQueueDepth(controller string, depth float64) {
	m.queueDepth.WithLabelValues(controller).Set(depth)
}

// ReconcileTimer provides a convenient way to time reconciliation operations.
type ReconcileTimer struct {
	controller string
	startTime  time.Time
	collector  *MetricsCollector
}

// NewReconcileTimer creates a new timer for measuring reconciliation duration.
func (m *MetricsCollector) NewReconcileTimer(controller string) *ReconcileTimer {
	return &ReconcileTimer{
		controller: controller,
		startTime:  time.Now(),
		collector:  m,
	}
}

// ObserveSuccess records a successful reconciliation with its duration.
func (t *ReconcileTimer) ObserveSuccess() {
	t.collector.RecordReconcileDuration(t.controller, time.Since(t.startTime), "success")
}

// ObserveError records a failed reconciliation with its duration and error type.
func (t *ReconcileTimer) ObserveError(errorType string) {
	duration := time.Since(t.startTime)
	t.collector.RecordReconcileDuration(t.controller, duration, "error")
	t.collector.RecordReconcileError(t.controller, errorType)
}

// ClassifyError categorizes an error into a standard error type for metrics.
func ClassifyError(err error) string {
	if err == nil {
		return "none"
	}

	errStr := err.Error()

	switch {
	case containsAny(errStr, "validation", "invalid", "required"):
		return "validation"
	case containsAny(errStr, "timeout", "deadline exceeded", "context deadline"):
		return "timeout"
	case containsAny(errStr, "conflict", "already exists", "resource version"):
		return "conflict"
	case containsAny(errStr, "permission", "forbidden", "unauthorized", "rbac"):
		return "permission"
	case containsAny(errStr, "not found", "does not exist"):
		return "not_found"
	case containsAny(errStr, "cancelled", "context canceled"):
		return "cancelled"
	default:
		return "unknown"
	}
}

// containsAny checks if the string contains any of the substrings.
func containsAny(s string, substrings ...string) bool {
	for _, sub := range substrings {
		if contains(s, sub) {
			return true
		}
	}
	return false
}

// contains is a simple case-insensitive substring check.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsLower(toLower(s), toLower(substr))))
}

func containsLower(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := range s {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}
