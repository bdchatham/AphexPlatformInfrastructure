# Design Document: Platform Controller Production Hardening

## Overview

This design document describes the architectural changes and implementation patterns required to harden the Aphex Platform Controller for production use. The refactoring addresses 28 issues identified in the code audit, ranging from critical race conditions to code quality improvements.

The design follows Kubernetes controller best practices, leveraging controller-runtime patterns and the client-go retry utilities. The goal is to transform the controller from a functional prototype into a robust, observable, and maintainable production system.

## Architecture

### High-Level Architecture

The Platform Controller follows the standard Kubernetes operator pattern with four reconcilers managing custom resources:

```
┌─────────────────────────────────────────────────────────────────┐
│                     Platform Controller                          │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────┐ │
│  │Organization │  │ RepoBinding │  │    Agent    │  │Knowledge│ │
│  │ Reconciler  │  │  Reconciler │  │  Reconciler │  │  Base   │ │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘  └────┬────┘ │
│         │                │                │               │      │
│  ┌──────┴────────────────┴────────────────┴───────────────┴────┐ │
│  │                    Shared Components                         │ │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────┐ │ │
│  │  │  Status  │  │ Finalizer│  │  Config  │  │   Metrics    │ │ │
│  │  │  Helper  │  │  Helper  │  │  Manager │  │   Collector  │ │ │
│  │  └──────────┘  └──────────┘  └──────────┘  └──────────────┘ │ │
│  └─────────────────────────────────────────────────────────────┘ │
├─────────────────────────────────────────────────────────────────┤
│                    Controller Runtime                            │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────────┐ │
│  │  Manager │  │  Client  │  │  Cache   │  │  Rate Limiter    │ │
│  └──────────┘  └──────────┘  └──────────┘  └──────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

### Key Architectural Changes

1. **Shared Status Helper**: Centralized status update logic with retry-on-conflict
2. **Finalizer Helper**: Verified cleanup before finalizer removal
3. **Config Manager**: Externalized configuration for namespaces and defaults
4. **Metrics Collector**: Prometheus metrics for observability
5. **Provisioner Refactoring**: Idempotent, timeout-aware provisioning with rollback

## Components and Interfaces

### Status Helper Component

Handles all status updates with conflict retry logic using `client-go/util/retry`.

```go
// StatusHelper provides conflict-safe status updates
type StatusHelper struct {
    client client.Client
    logger logr.Logger
}

// UpdateStatusWithRetry updates status with automatic retry on conflict
func (h *StatusHelper) UpdateStatusWithRetry(
    ctx context.Context,
    obj client.Object,
    mutate func() error,
) error

// PatchStatus uses merge patch for efficient status updates
func (h *StatusHelper) PatchStatus(
    ctx context.Context,
    obj client.Object,
    patch client.Patch,
) error
```

### Finalizer Helper Component

Manages finalizer lifecycle with verified cleanup.

```go
// FinalizerHelper manages finalizer operations with cleanup verification
type FinalizerHelper struct {
    client    client.Client
    logger    logr.Logger
    finalizer string
}

// EnsureFinalizer adds finalizer if not present (after validation)
func (h *FinalizerHelper) EnsureFinalizer(
    ctx context.Context,
    obj client.Object,
) error

// HandleDeletion performs cleanup and removes finalizer only on success
func (h *FinalizerHelper) HandleDeletion(
    ctx context.Context,
    obj client.Object,
    cleanupFn func(context.Context) error,
) error
```

### Config Manager Component

Centralizes configuration with environment variable support.

```go
// Config holds controller configuration
type Config struct {
    PlatformNamespace    string
    AllowlistNamespace   string
    DefaultResourceQuota ResourceQuotaConfig
    ProvisioningTimeout  time.Duration
    MaxConcurrentReconciles int
    LeaderElectionEnabled   bool
    DevelopmentMode         bool
}

// ConfigManager loads and validates configuration
type ConfigManager struct {
    config *Config
}

// LoadFromEnvironment loads config from environment variables
func (m *ConfigManager) LoadFromEnvironment() error

// GetConfig returns the loaded configuration
func (m *ConfigManager) GetConfig() *Config
```

### Metrics Collector Component

Exposes Prometheus metrics for controller observability.

```go
// MetricsCollector manages custom controller metrics
type MetricsCollector struct {
    reconcileDuration    *prometheus.HistogramVec
    reconcileErrors      *prometheus.CounterVec
    provisioningSteps    *prometheus.CounterVec
    resourcesProvisioned *prometheus.GaugeVec
}

// RecordReconcileDuration records reconciliation duration
func (m *MetricsCollector) RecordReconcileDuration(
    controller string,
    duration time.Duration,
)

// RecordReconcileError increments error counter
func (m *MetricsCollector) RecordReconcileError(
    controller string,
    errorType string,
)
```

### Provisioner Interface

Standardized interface for resource provisioning with idempotency.

```go
// Provisioner creates and manages dependent resources
type Provisioner interface {
    // Provision creates or updates a resource idempotently
    Provision(ctx context.Context, owner client.Object) error
    
    // Cleanup removes resources created by this provisioner
    Cleanup(ctx context.Context, owner client.Object) error
    
    // Name returns the provisioner name for logging
    Name() string
}

// ProvisioningContext tracks provisioning state for rollback
type ProvisioningContext struct {
    CreatedResources []client.Object
    StartTime        time.Time
    CurrentStep      string
}
```

### Validator Interface

Validates resources before provisioning.

```go
// Validator validates custom resources
type Validator interface {
    // Validate checks resource validity
    Validate(ctx context.Context, obj client.Object) error
}

// WebhookValidator implements admission webhook validation
type WebhookValidator struct {
    validators map[schema.GroupVersionKind]Validator
}
```



## Data Models

### Constants Package

Centralizes all magic strings and configuration defaults.

```go
package constants

const (
    // Finalizers
    RepoBindingFinalizer    = "platform.aphex/repobinding-finalizer"
    OrganizationFinalizer   = "platform.aphex/organization-finalizer"
    AgentFinalizer          = "platform.aphex/agent-finalizer"
    KnowledgeBaseFinalizer  = "platform.aphex/knowledgebase-finalizer"
    
    // Service Accounts
    PipelineRunnerSA = "pipeline-runner"
    
    // Namespaces (defaults, overridable via config)
    DefaultPlatformNamespace  = "platform-system"
    DefaultAllowlistNamespace = "pipeline-system"
    
    // Labels
    LabelManagedBy     = "app.kubernetes.io/managed-by"
    LabelOwnerName     = "platform.aphex/owner-name"
    LabelOwnerNamespace = "platform.aphex/owner-namespace"
    LabelOwnerKind     = "platform.aphex/owner-kind"
    LabelPipeline      = "platform.aphex/pipeline"
    
    // Annotations
    AnnotationLastAppliedSpec = "platform.aphex/last-applied-spec"
    
    // Phases
    PhasePending      = "Pending"
    PhaseProvisioning = "Provisioning"
    PhaseReady        = "Ready"
    PhaseFailed       = "Failed"
    PhaseDeleting     = "Deleting"
    
    // Timeouts
    DefaultProvisioningTimeout = 5 * time.Minute
    DefaultDependencyWaitTime  = 30 * time.Second
    DefaultShutdownTimeout     = 30 * time.Second
    
    // Rate Limiting
    DefaultMaxConcurrentReconciles = 3
    DefaultBaseDelay               = 1 * time.Second
    DefaultMaxDelay                = 60 * time.Second
)
```

### Resource Quota Configuration

```go
// ResourceQuotaConfig defines configurable resource limits
type ResourceQuotaConfig struct {
    CPURequest    resource.Quantity
    CPULimit      resource.Quantity
    MemoryRequest resource.Quantity
    MemoryLimit   resource.Quantity
    PodCount      int64
}

// DefaultResourceQuota returns sensible defaults
func DefaultResourceQuota() ResourceQuotaConfig {
    return ResourceQuotaConfig{
        CPURequest:    resource.MustParse("1"),
        CPULimit:      resource.MustParse("4"),
        MemoryRequest: resource.MustParse("1Gi"),
        MemoryLimit:   resource.MustParse("8Gi"),
        PodCount:      10,
    }
}
```

### Health State Tracking

```go
// HealthState tracks controller health for health checks
type HealthState struct {
    mu                    sync.RWMutex
    lastSuccessfulReconcile map[string]time.Time
    healthThreshold       time.Duration
}

// RecordSuccess records a successful reconciliation
func (h *HealthState) RecordSuccess(controller string)

// IsHealthy returns true if recent reconciliations succeeded
func (h *HealthState) IsHealthy() bool
```



## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system—essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.*

### Property 1: Status Update Retry on Conflict

*For any* status update operation that encounters a conflict error, the StatusHelper SHALL retry the operation up to 3 times, fetching fresh resource state before each retry, and return an error only after all retries are exhausted.

**Validates: Requirements 1.1, 1.2, 1.3**

### Property 2: Verified Finalizer Cleanup

*For any* resource with a finalizer that is being deleted, the finalizer SHALL only be removed after all cleanup operations return success. If any cleanup operation fails, the finalizer SHALL remain and the reconciliation SHALL be requeued.

**Validates: Requirements 2.1, 2.2, 2.3**

### Property 3: Idempotent Resource Provisioning

*For any* resource provisioning operation, if the desired spec equals the existing spec, no update SHALL be performed. If the specs differ, an update SHALL be performed. This property SHALL hold for all resource types created by provisioners.

**Validates: Requirements 3.1, 3.2, 3.3, 3.4**

### Property 4: RBAC Permission Validation

*For any* Role or ClusterRole creation, the controller SHALL validate it has permission to grant all specified rules before attempting creation. If validation fails, a descriptive error SHALL be returned and the resource status SHALL be updated with the error.

**Validates: Requirements 4.1, 4.2, 4.3**

### Property 5: Provisioning Timeout and Context Cancellation

*For any* provisioning operation, a context timeout of 5 minutes SHALL be applied. If the timeout is exceeded or the context is cancelled, the operation SHALL stop and return an appropriate error. Context cancellation SHALL be checked between all provisioning steps.

**Validates: Requirements 5.1, 5.2, 5.3, 5.4, 16.1, 16.2, 16.3**

### Property 6: Safe YAML Parsing

*For any* YAML parsing operation, the decoded object SHALL be validated to have the expected Kind and APIVersion. If validation fails, a descriptive error SHALL be returned and the resource status SHALL be updated.

**Validates: Requirements 6.1, 6.2, 6.3**

### Property 7: Graceful Dependency Waiting

*For any* cross-resource reference where the referenced resource does not exist, the controller SHALL update status to "Pending" with a descriptive message and requeue after 30 seconds. When the dependency becomes available, provisioning SHALL proceed.

**Validates: Requirements 7.1, 7.2, 7.3, 7.4**

### Property 8: Null-Safe Dependency Access

*For any* access to an injected dependency (such as TemplateCatalog), the controller SHALL verify the dependency is not nil before use. If nil, a descriptive initialization error SHALL be returned.

**Validates: Requirements 8.1, 8.2, 8.3**

### Property 9: Configurable Namespace References

*For any* namespace reference, the controller SHALL read the value from environment variables or flags. If not configured, sensible defaults SHALL be used and a warning SHALL be logged.

**Validates: Requirements 9.1, 9.2, 9.3**

### Property 10: Provisioning Failure Rollback

*For any* provisioning failure that occurs after resources have been created, the controller SHALL track created resources and attempt cleanup. The resource status SHALL indicate partial provisioning failure with details about any cleanup failures.

**Validates: Requirements 10.1, 10.2, 10.3, 10.4**

### Property 11: ArgoCD Object Validation

*For any* ArgoCD Application or AppProject creation, required fields SHALL be validated before cluster creation. If validation fails, a descriptive error SHALL be returned.

**Validates: Requirements 11.1, 11.2, 11.3**

### Property 12: Resource Quota Configuration Precedence

*For any* ResourceQuota creation, limits SHALL be read from RepoBinding spec first, then Organization defaults, then hardcoded defaults. The first available source SHALL be used.

**Validates: Requirements 14.1, 14.2, 14.3**

### Property 13: Exponential Backoff Rate Limiting

*For any* resource that fails reconciliation repeatedly, the backoff delay SHALL increase exponentially up to a maximum of 60 seconds.

**Validates: Requirements 17.3**

### Property 14: Validation Before Finalizer

*For any* new resource, validation SHALL occur before adding a finalizer. If validation fails, status SHALL be updated to failed without adding a finalizer. If validation passes, the finalizer SHALL be added.

**Validates: Requirements 18.1, 18.2, 18.3**

### Property 15: Webhook Validation

*For any* resource submitted to the admission webhook, required fields, field formats, and cross-field constraints SHALL be validated. Invalid resources SHALL be rejected with descriptive error messages.

**Validates: Requirements 19.2, 19.3**

### Property 16: Efficient Status Patching

*For any* status modification, Status().Patch() with merge patch semantics SHALL be used instead of Status().Update().

**Validates: Requirements 20.1, 20.2, 20.3**

### Property 17: Cluster-Scoped Resource Tracking

*For any* ClusterRole or ClusterRoleBinding creation, the resource SHALL be labeled with the owning resource reference. When the parent is deleted, labeled cluster-scoped resources SHALL be cleaned up.

**Validates: Requirements 22.1, 22.2, 22.3**

### Property 18: Health Check Accuracy

*For any* health check request, the controller SHALL report unhealthy if no successful reconciliation has occurred within the configured threshold.

**Validates: Requirements 23.1, 23.2**

### Property 19: Error Chain Preservation

*For any* wrapped error, the error chain SHALL be preserved such that errors.Is() and errors.As() work correctly. Wrapped errors SHALL include contextual information.

**Validates: Requirements 28.2, 28.3**

### Property 20: Complete Resource Management

*For any* custom resource creation, the controller SHALL create all required dependent resources (namespaces, RBAC, deployments, services, etc.) without requiring external intervention. All namespace-scoped dependent resources SHALL have owner references set for automatic garbage collection.

**Validates: Requirements 29.1, 29.2, 29.3, 29.4, 29.5**



## Error Handling

### Error Categories

The controller distinguishes between different error categories for appropriate handling:

1. **Transient Errors**: Network issues, API server unavailability, conflict errors
   - Action: Retry with exponential backoff
   - Status: Keep current phase, update message

2. **Validation Errors**: Invalid spec, missing required fields, constraint violations
   - Action: Do not retry, update status to Failed
   - Status: Phase = Failed, Message = validation error details

3. **Dependency Errors**: Missing Organization, unavailable external service
   - Action: Requeue after delay, update status to Pending
   - Status: Phase = Pending, Message = waiting for dependency

4. **Permission Errors**: RBAC violations, insufficient permissions
   - Action: Do not retry, update status to Failed
   - Status: Phase = Failed, Message = permission error details

5. **Timeout Errors**: Provisioning step exceeded timeout
   - Action: Cancel operation, update status to Failed
   - Status: Phase = Failed, Message = timeout details with step name

### Error Wrapping Pattern

All errors use consistent wrapping with `%w` verb:

```go
// Good: preserves error chain
return fmt.Errorf("failed to create namespace %s: %w", name, err)

// Bad: loses error chain
return fmt.Errorf("failed to create namespace %s: %s", name, err.Error())
```

### Status Update on Error

```go
func (r *Reconciler) handleError(
    ctx context.Context,
    obj client.Object,
    err error,
    step string,
) (ctrl.Result, error) {
    switch {
    case errors.IsConflict(err):
        // Transient: requeue immediately
        return ctrl.Result{Requeue: true}, nil
        
    case errors.IsNotFound(err) && isExternalDependency(err):
        // Dependency: requeue after delay
        r.updateStatusToPending(ctx, obj, "Waiting for dependency")
        return ctrl.Result{RequeueAfter: constants.DefaultDependencyWaitTime}, nil
        
    case isValidationError(err):
        // Validation: fail permanently
        r.updateStatusToFailed(ctx, obj, err.Error())
        return ctrl.Result{}, nil
        
    default:
        // Unknown: requeue with backoff
        return ctrl.Result{}, err
    }
}
```

## Testing Strategy

### Dual Testing Approach

The testing strategy combines unit tests for specific examples and edge cases with property-based tests for universal properties.

### Unit Tests

Unit tests focus on:
- Specific validation scenarios (valid/invalid inputs)
- Edge cases (empty strings, nil values, boundary conditions)
- Error conditions (network failures, permission errors)
- Integration points between components

### Property-Based Tests

Property-based tests verify universal properties using the `gopter` library for Go:

```go
// Example property test structure
func TestStatusUpdateRetryProperty(t *testing.T) {
    properties := gopter.NewProperties(nil)
    
    // Feature: platform-controller-production-hardening, Property 1: Status Update Retry on Conflict
    properties.Property("status updates retry on conflict up to 3 times", prop.ForAll(
        func(resourceState ResourceState, conflictCount int) bool {
            // Generate random resource state
            // Simulate conflict errors
            // Verify retry behavior
            return verifyRetryBehavior(resourceState, conflictCount)
        },
        genResourceState(),
        gen.IntRange(0, 5),
    ))
    
    properties.TestingRun(t)
}
```

### Test Configuration

- Minimum 100 iterations per property test
- Use `envtest` for controller integration tests
- Target 70% code coverage for controller logic

### Test Organization

Tests are organized in `.kiro/tests/` following the property-tests.md steering:

```
.kiro/tests/
├── status_helper_test.go      # Property tests for StatusHelper
├── finalizer_helper_test.go   # Property tests for FinalizerHelper
├── provisioner_test.go        # Property tests for provisioners
├── validation_test.go         # Property tests for validators
└── integration_test.go        # Integration tests using envtest
```

### Property Test Tags

Each property test includes a comment referencing the design property:

```go
// Feature: platform-controller-production-hardening, Property 1: Status Update Retry on Conflict
// For any status update operation that encounters a conflict error, the StatusHelper
// SHALL retry the operation up to 3 times, fetching fresh resource state before each retry.
func TestStatusUpdateRetryProperty(t *testing.T) {
    // ...
}
```

## Implementation Notes

### Key Dependencies

- `k8s.io/client-go/util/retry`: RetryOnConflict for status updates
- `sigs.k8s.io/controller-runtime`: Controller framework
- `github.com/prometheus/client_golang`: Metrics
- `github.com/leanovate/gopter`: Property-based testing

### Migration Strategy

1. **Phase 1**: Add shared components (StatusHelper, FinalizerHelper, Config)
2. **Phase 2**: Refactor RepoBinding controller (highest complexity)
3. **Phase 3**: Refactor remaining controllers
4. **Phase 4**: Add webhooks and metrics
5. **Phase 5**: Add comprehensive tests

### Backward Compatibility

- Existing CRD specs remain unchanged
- Status fields are additive (no breaking changes)
- Finalizer names remain unchanged to avoid orphaned resources

