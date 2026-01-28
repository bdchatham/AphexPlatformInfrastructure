# Requirements Document

## Introduction

This document specifies the requirements for refactoring the Aphex Platform Controller to production-ready quality. The Platform Controller manages four Custom Resource Definitions (CRDs): Organization, RepoBinding, Agent, and KnowledgeBase. A comprehensive audit identified 28 issues across critical, high, medium, and low priority categories that must be addressed to achieve production readiness.

The refactoring must follow Kubernetes controller best practices, meet CLAUDE.md code quality standards, and ensure controllers fully manage all dependent resources for users.

## Glossary

- **Platform_Controller**: The Kubernetes controller application that reconciles Aphex CRDs
- **Reconciler**: A controller component that handles reconciliation loops for a specific CRD
- **RepoBinding_Controller**: The reconciler that manages RepoBinding custom resources
- **Organization_Controller**: The reconciler that manages Organization custom resources
- **Agent_Controller**: The reconciler that manages Agent custom resources
- **KnowledgeBase_Controller**: The reconciler that manages KnowledgeBase custom resources
- **Provisioner**: A component that creates dependent Kubernetes resources during reconciliation
- **Finalizer**: A Kubernetes mechanism to ensure cleanup before resource deletion
- **Status_Update**: An operation that modifies the status subresource of a custom resource
- **Conflict_Error**: An error indicating concurrent modification of a resource (HTTP 409)
- **Owner_Reference**: A Kubernetes mechanism linking child resources to parent resources for garbage collection
- **Rate_Limiter**: A mechanism to control the frequency of reconciliation attempts
- **Leader_Election**: A mechanism ensuring only one controller replica is active at a time

## Requirements

### Requirement 1: Conflict-Safe Status Updates

**User Story:** As a platform operator, I want status updates to handle concurrent modifications gracefully, so that reconciliation does not fail randomly under load.

#### Acceptance Criteria

1. WHEN the Reconciler performs a Status_Update AND a Conflict_Error occurs, THEN THE Platform_Controller SHALL retry the operation up to 3 times with fresh resource state
2. WHEN all retry attempts for a Status_Update fail, THEN THE Platform_Controller SHALL return an error and requeue the reconciliation
3. THE Platform_Controller SHALL use optimistic concurrency control for all status updates across all controllers

### Requirement 2: Verified Finalizer Cleanup

**User Story:** As a platform operator, I want finalizers to only be removed after successful cleanup, so that dependent resources are never orphaned.

#### Acceptance Criteria

1. WHEN a resource with a Finalizer is deleted, THEN THE Platform_Controller SHALL verify all cleanup operations succeeded before removing the Finalizer
2. IF cleanup of any dependent resource fails, THEN THE Platform_Controller SHALL retain the Finalizer and requeue the reconciliation
3. WHEN cleanup succeeds for all dependent resources, THEN THE Platform_Controller SHALL remove the Finalizer and update the resource
4. THE Platform_Controller SHALL log cleanup progress with structured fields indicating which resources were cleaned up

### Requirement 3: Idempotent Resource Provisioning

**User Story:** As a platform operator, I want resource provisioning to be idempotent, so that running pipelines are not disrupted by reconciliation.

#### Acceptance Criteria

1. WHEN the Provisioner creates or updates a Pipeline, THEN THE Platform_Controller SHALL compare the desired spec with the existing spec before updating
2. IF the Pipeline spec has not changed, THEN THE Platform_Controller SHALL skip the update operation
3. WHEN the Pipeline spec has changed, THEN THE Platform_Controller SHALL update the resource and log the change with structured fields
4. THE Platform_Controller SHALL apply idempotency checks to all resource types created by provisioners

### Requirement 4: RBAC Permission Validation

**User Story:** As a platform operator, I want the controller to validate RBAC permissions before creating roles, so that provisioning does not fail with cryptic permission errors.

#### Acceptance Criteria

1. WHEN the Provisioner creates a Role or ClusterRole, THEN THE Platform_Controller SHALL validate it can grant the specified permissions
2. IF the Platform_Controller lacks permission to grant a rule, THEN THE Platform_Controller SHALL return a descriptive error indicating the missing permission
3. WHEN RBAC validation fails, THEN THE Platform_Controller SHALL update the resource status with a clear error message

### Requirement 5: Provisioning Timeouts

**User Story:** As a platform operator, I want provisioning steps to have timeouts, so that reconciliation does not hang indefinitely.

#### Acceptance Criteria

1. WHEN a provisioning step begins, THEN THE Platform_Controller SHALL apply a context timeout of 5 minutes
2. IF a provisioning step exceeds the timeout, THEN THE Platform_Controller SHALL cancel the operation and return a timeout error
3. WHEN a timeout occurs, THEN THE Platform_Controller SHALL update the resource status to indicate the timeout and which step failed
4. THE Platform_Controller SHALL check context cancellation between provisioning steps



### Requirement 6: Safe YAML Parsing

**User Story:** As a platform operator, I want YAML parsing to validate decoded objects, so that malicious or malformed input does not cause unexpected behavior.

#### Acceptance Criteria

1. WHEN the Provisioner parses Pipeline YAML, THEN THE Platform_Controller SHALL validate the decoded object has the expected Kind and APIVersion
2. IF the decoded object has an unexpected Kind or APIVersion, THEN THE Platform_Controller SHALL return a descriptive validation error
3. WHEN validation fails, THEN THE Platform_Controller SHALL update the resource status with the validation error details

### Requirement 7: Graceful Dependency Waiting

**User Story:** As a platform operator, I want RepoBindings to wait gracefully for Organizations to exist, so that resources can be created in any order.

#### Acceptance Criteria

1. WHEN a RepoBinding references an Organization that does not exist, THEN THE Platform_Controller SHALL update the status to "Pending" with a descriptive message
2. WHEN waiting for a dependency, THEN THE Platform_Controller SHALL requeue the reconciliation after 30 seconds
3. WHEN the dependency becomes available, THEN THE Platform_Controller SHALL proceed with provisioning on the next reconciliation
4. THE Platform_Controller SHALL apply graceful dependency waiting to all cross-resource references

### Requirement 8: Null-Safe Template Catalog

**User Story:** As a platform operator, I want the controller to handle uninitialized dependencies safely, so that it does not panic on startup failures.

#### Acceptance Criteria

1. WHEN the Reconciler accesses the Template_Catalog, THEN THE Platform_Controller SHALL verify it is not nil before use
2. IF the Template_Catalog is nil, THEN THE Platform_Controller SHALL return a descriptive initialization error
3. THE Platform_Controller SHALL apply null-safety checks to all injected dependencies

### Requirement 9: Configurable Namespace References

**User Story:** As a platform operator, I want hardcoded namespace references to be configurable, so that the controller works in different deployment configurations.

#### Acceptance Criteria

1. THE Platform_Controller SHALL read the allowlist ConfigMap namespace from an environment variable or controller flag
2. THE Platform_Controller SHALL read the platform system namespace from an environment variable or controller flag
3. WHEN namespace configuration is missing, THEN THE Platform_Controller SHALL use sensible defaults and log a warning

### Requirement 10: Provisioning Failure Rollback

**User Story:** As a platform operator, I want partial provisioning to be cleaned up on failure, so that orphaned resources do not accumulate.

#### Acceptance Criteria

1. WHEN a provisioning step fails, THEN THE Platform_Controller SHALL record which resources were created in earlier steps
2. IF provisioning fails after creating resources, THEN THE Platform_Controller SHALL attempt to clean up the created resources
3. WHEN rollback cleanup fails, THEN THE Platform_Controller SHALL log the orphaned resources and update status with cleanup failure details
4. THE Platform_Controller SHALL update the resource status to indicate partial provisioning failure

### Requirement 11: Validated ArgoCD Objects

**User Story:** As a platform operator, I want ArgoCD objects to be validated at runtime, so that invalid configurations are caught early.

#### Acceptance Criteria

1. WHEN the Provisioner creates an ArgoCD Application or AppProject, THEN THE Platform_Controller SHALL validate required fields are present
2. IF an ArgoCD object is invalid, THEN THE Platform_Controller SHALL return a descriptive validation error
3. THE Platform_Controller SHALL validate ArgoCD objects before attempting to create them in the cluster

### Requirement 12: Appropriate Log Levels

**User Story:** As a platform operator, I want logs to use appropriate severity levels, so that routine operations do not flood log aggregators.

#### Acceptance Criteria

1. THE Platform_Controller SHALL use DEBUG level for routine reconciliation progress
2. THE Platform_Controller SHALL use INFO level only for state changes and significant events
3. THE Platform_Controller SHALL use WARN level for recoverable issues and degraded operation
4. THE Platform_Controller SHALL use ERROR level only for failures requiring attention

### Requirement 13: Prometheus Metrics

**User Story:** As a platform operator, I want the controller to expose Prometheus metrics, so that I can monitor reconciliation health and performance.

#### Acceptance Criteria

1. THE Platform_Controller SHALL expose a histogram metric for reconciliation duration per controller
2. THE Platform_Controller SHALL expose a counter metric for reconciliation errors per controller
3. THE Platform_Controller SHALL expose a gauge metric for current queue depth per controller
4. THE Platform_Controller SHALL expose metrics on the standard /metrics endpoint

### Requirement 14: Configurable Resource Limits

**User Story:** As a platform operator, I want ResourceQuota limits to be configurable, so that I can customize resource allocation per pipeline.

#### Acceptance Criteria

1. WHEN the Provisioner creates a ResourceQuota, THEN THE Platform_Controller SHALL read limits from the RepoBinding spec if provided
2. IF the RepoBinding spec does not specify limits, THEN THE Platform_Controller SHALL use Organization defaults if available
3. IF no limits are specified, THEN THE Platform_Controller SHALL use sensible hardcoded defaults

### Requirement 15: Leader Election

**User Story:** As a platform operator, I want leader election enabled by default, so that running multiple replicas does not cause conflicts.

#### Acceptance Criteria

1. THE Platform_Controller SHALL enable leader election by default in production deployments
2. WHEN leader election is enabled, THEN THE Platform_Controller SHALL only reconcile resources on the leader replica
3. THE Platform_Controller SHALL support disabling leader election via flag for development environments



### Requirement 16: Context Cancellation Handling

**User Story:** As a platform operator, I want long-running operations to respect context cancellation, so that graceful shutdown works correctly.

#### Acceptance Criteria

1. WHEN a provisioning operation runs, THEN THE Platform_Controller SHALL check context cancellation between steps
2. IF the context is cancelled during provisioning, THEN THE Platform_Controller SHALL stop processing and return the context error
3. THE Platform_Controller SHALL propagate context cancellation to all downstream operations

### Requirement 17: Reconciliation Rate Limiting

**User Story:** As a platform operator, I want reconciliation to be rate-limited, so that flapping resources do not overwhelm the API server.

#### Acceptance Criteria

1. THE Platform_Controller SHALL configure exponential backoff rate limiting for failed reconciliations
2. THE Platform_Controller SHALL limit maximum concurrent reconciliations per controller
3. WHEN a resource fails reconciliation repeatedly, THEN THE Platform_Controller SHALL increase the backoff delay up to a maximum of 60 seconds

### Requirement 18: Validation Before Finalizer

**User Story:** As a platform operator, I want resources to be validated before adding finalizers, so that invalid resources do not get stuck.

#### Acceptance Criteria

1. WHEN a new resource is created, THEN THE Platform_Controller SHALL validate the spec before adding a Finalizer
2. IF validation fails, THEN THE Platform_Controller SHALL update status to failed without adding a Finalizer
3. WHEN validation passes, THEN THE Platform_Controller SHALL add the Finalizer and proceed with provisioning

### Requirement 19: Validating Admission Webhook

**User Story:** As a platform operator, I want invalid resources rejected at admission time, so that bad configurations never enter the cluster.

#### Acceptance Criteria

1. THE Platform_Controller SHALL implement a ValidatingWebhookConfiguration for all CRDs
2. WHEN an invalid resource is submitted, THEN THE Platform_Controller SHALL reject it with a descriptive error message
3. THE Platform_Controller SHALL validate required fields, field formats, and cross-field constraints at admission

### Requirement 20: Efficient Status Patching

**User Story:** As a platform operator, I want status updates to use patch operations, so that updates are more efficient and less prone to conflicts.

#### Acceptance Criteria

1. THE Platform_Controller SHALL use Status().Patch() instead of Status().Update() for status modifications
2. WHEN patching status, THEN THE Platform_Controller SHALL use merge patch semantics
3. THE Platform_Controller SHALL apply efficient patching to all controllers

### Requirement 21: Structured Logging

**User Story:** As a platform operator, I want logs to use structured fields, so that log aggregation and querying is effective.

#### Acceptance Criteria

1. THE Platform_Controller SHALL use structured logging with key-value fields instead of string concatenation
2. THE Platform_Controller SHALL include resource name, namespace, and controller name in all log entries
3. THE Platform_Controller SHALL include relevant context fields such as step name, error type, and duration

### Requirement 22: Cluster-Scoped Resource Tracking

**User Story:** As a platform operator, I want cluster-scoped resources to be tracked for cleanup, so that they are garbage collected when parent resources are deleted.

#### Acceptance Criteria

1. WHEN the Provisioner creates ClusterRoles or ClusterRoleBindings, THEN THE Platform_Controller SHALL label them with the owning resource reference
2. WHEN a parent resource is deleted, THEN THE Platform_Controller SHALL clean up labeled cluster-scoped resources
3. THE Platform_Controller SHALL use consistent labeling conventions for resource tracking

### Requirement 23: Meaningful Health Checks

**User Story:** As a platform operator, I want health checks to reflect actual controller health, so that unhealthy controllers are restarted.

#### Acceptance Criteria

1. THE Platform_Controller SHALL implement a health check that verifies recent successful reconciliation
2. IF no successful reconciliation has occurred within a threshold, THEN THE Platform_Controller SHALL report unhealthy
3. THE Platform_Controller SHALL expose health status on the /healthz endpoint

### Requirement 24: Configurable Development Mode

**User Story:** As a platform operator, I want development mode to be configurable, so that production deployments use production logging settings.

#### Acceptance Criteria

1. THE Platform_Controller SHALL read development mode setting from an environment variable or flag
2. WHEN development mode is disabled, THEN THE Platform_Controller SHALL use production logging configuration
3. THE Platform_Controller SHALL default to production mode when not explicitly configured

### Requirement 25: Graceful Shutdown

**User Story:** As a platform operator, I want the controller to shut down gracefully, so that in-progress reconciliations complete cleanly.

#### Acceptance Criteria

1. WHEN the Platform_Controller receives SIGTERM, THEN THE Platform_Controller SHALL stop accepting new reconciliations
2. WHEN shutting down, THEN THE Platform_Controller SHALL wait for in-progress reconciliations to complete up to a timeout
3. THE Platform_Controller SHALL log shutdown progress and any reconciliations that were interrupted

### Requirement 26: Named Constants

**User Story:** As a developer, I want magic strings replaced with named constants, so that the code is maintainable and consistent.

#### Acceptance Criteria

1. THE Platform_Controller SHALL define constants for all repeated string literals
2. THE Platform_Controller SHALL use constants for service account names, namespace names, and label keys
3. THE Platform_Controller SHALL organize constants in a dedicated constants file or section

### Requirement 27: Comprehensive Test Coverage

**User Story:** As a developer, I want provisioners to have unit tests, so that changes can be made with confidence.

#### Acceptance Criteria

1. THE Platform_Controller SHALL have unit tests for all provisioner functions
2. THE Platform_Controller SHALL have unit tests for all validation functions
3. THE Platform_Controller SHALL use envtest for controller integration tests
4. THE Platform_Controller SHALL achieve at least 70% code coverage for controller logic

### Requirement 28: Consistent Error Wrapping

**User Story:** As a developer, I want errors to be wrapped consistently, so that error chains are preserved for debugging.

#### Acceptance Criteria

1. THE Platform_Controller SHALL use fmt.Errorf with %w verb for all error wrapping
2. THE Platform_Controller SHALL preserve error chains to enable errors.Is() and errors.As() checks
3. THE Platform_Controller SHALL include contextual information in wrapped error messages

### Requirement 29: Complete Resource Management

**User Story:** As a platform user, I want controllers to manage all dependent resources, so that I only need to create the custom resource itself.

#### Acceptance Criteria

1. WHEN a RepoBinding is created, THEN THE RepoBinding_Controller SHALL create the target namespace if it does not exist
2. WHEN an Organization is created, THEN THE Organization_Controller SHALL create all required namespaces and RBAC resources
3. WHEN an Agent is created, THEN THE Agent_Controller SHALL create all required deployments, services, and configurations
4. WHEN a KnowledgeBase is created, THEN THE KnowledgeBase_Controller SHALL create all required storage and processing resources
5. THE Platform_Controller SHALL set Owner_References on all namespace-scoped dependent resources for automatic garbage collection
