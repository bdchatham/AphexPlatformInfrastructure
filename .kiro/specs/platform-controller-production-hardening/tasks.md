# Implementation Plan: Platform Controller Production Hardening

## Overview

This implementation plan refactors the Aphex Platform Controller to production-ready quality by addressing 28 audit issues. The work is organized into phases: shared components first, then controller refactoring, followed by webhooks/metrics, and finally comprehensive testing.

## Tasks

- [x] 1. Create constants and configuration infrastructure
  - [x] 1.1 Create constants package with all magic strings
    - Create `controllers/constants/constants.go`
    - Define finalizer names, service account names, namespace defaults
    - Define label keys, annotation keys, phase constants
    - Define timeout and rate limiting defaults
    - _Requirements: 26.1, 26.2, 26.3_
  
  - [x] 1.2 Create ConfigManager component
    - Create `controllers/config/config.go`
    - Implement Config struct with all configurable values
    - Implement LoadFromEnvironment() to read env vars
    - Implement GetConfig() to return loaded configuration
    - Add validation for required configuration
    - _Requirements: 9.1, 9.2, 9.3, 24.1, 24.2, 24.3_

- [x] 2. Implement StatusHelper component
  - [x] 2.1 Create StatusHelper with retry logic
    - Create `controllers/helpers/status_helper.go`
    - Implement UpdateStatusWithRetry using client-go retry.RetryOnConflict
    - Implement PatchStatus using merge patch semantics
    - Add structured logging for retry attempts
    - _Requirements: 1.1, 1.2, 1.3, 20.1, 20.2, 20.3_
  
  - [ ]* 2.2 Write property test for StatusHelper retry behavior
    - **Property 1: Status Update Retry on Conflict**
    - **Validates: Requirements 1.1, 1.2, 1.3**

- [x] 3. Implement FinalizerHelper component
  - [x] 3.1 Create FinalizerHelper with verified cleanup
    - Create `controllers/helpers/finalizer_helper.go`
    - Implement EnsureFinalizer (adds finalizer after validation)
    - Implement HandleDeletion (cleanup then remove finalizer)
    - Only remove finalizer after all cleanup succeeds
    - Add structured logging for cleanup progress
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 18.1, 18.2, 18.3_
  
  - [ ]* 3.2 Write property test for FinalizerHelper cleanup verification
    - **Property 2: Verified Finalizer Cleanup**
    - **Validates: Requirements 2.1, 2.2, 2.3**

- [x] 4. Implement provisioning infrastructure
  - [x] 4.1 Create Provisioner interface and ProvisioningContext
    - Create `controllers/provisioners/provisioner.go`
    - Define Provisioner interface with Provision, Cleanup, Name methods
    - Implement ProvisioningContext for tracking created resources
    - Add helper for idempotent create-or-update with spec comparison
    - _Requirements: 3.1, 3.2, 3.3, 3.4_
  
  - [x] 4.2 Implement provisioning timeout and context cancellation
    - Add context timeout wrapper for provisioning steps
    - Add context cancellation checks between steps
    - Implement timeout error handling with step information
    - _Requirements: 5.1, 5.2, 5.3, 5.4, 16.1, 16.2, 16.3_
  
  - [x] 4.3 Implement rollback on provisioning failure
    - Track created resources in ProvisioningContext
    - Implement cleanup of created resources on failure
    - Handle cleanup failures with logging and status update
    - _Requirements: 10.1, 10.2, 10.3, 10.4_
  
  - [ ]* 4.4 Write property test for idempotent provisioning
    - **Property 3: Idempotent Resource Provisioning**
    - **Validates: Requirements 3.1, 3.2, 3.3, 3.4**
  
  - [ ]* 4.5 Write property test for provisioning timeout
    - **Property 5: Provisioning Timeout and Context Cancellation**
    - **Validates: Requirements 5.1, 5.2, 5.3, 5.4, 16.1, 16.2, 16.3**

- [ ] 5. Checkpoint - Ensure shared components work
  - Ensure all tests pass, ask the user if questions arise.



- [x] 6. Implement validation components
  - [x] 6.1 Create YAML parsing validator
    - Create `controllers/validators/yaml_validator.go`
    - Validate decoded objects have expected Kind and APIVersion
    - Return descriptive validation errors
    - _Requirements: 6.1, 6.2, 6.3_
  
  - [x] 6.2 Create RBAC permission validator
    - Create `controllers/validators/rbac_validator.go`
    - Use SubjectAccessReview to check grantable permissions
    - Return descriptive errors for missing permissions
    - _Requirements: 4.1, 4.2, 4.3_
  
  - [x] 6.3 Create ArgoCD object validator
    - Create `controllers/validators/argocd_validator.go`
    - Validate required fields for Application and AppProject
    - Return descriptive validation errors
    - _Requirements: 11.1, 11.2, 11.3_
  
  - [ ]* 6.4 Write property test for YAML validation
    - **Property 6: Safe YAML Parsing**
    - **Validates: Requirements 6.1, 6.2, 6.3**
  
  - [ ]* 6.5 Write property test for RBAC validation
    - **Property 4: RBAC Permission Validation**
    - **Validates: Requirements 4.1, 4.2, 4.3**

- [x] 7. Implement dependency waiting and null-safety
  - [x] 7.1 Create dependency resolver with graceful waiting
    - Create `controllers/helpers/dependency_resolver.go`
    - Implement graceful waiting for missing dependencies
    - Update status to Pending with descriptive message
    - Return RequeueAfter result for dependency waiting
    - _Requirements: 7.1, 7.2, 7.3, 7.4_
  
  - [x] 7.2 Add null-safety checks for injected dependencies
    - Add nil checks for TemplateCatalog and other dependencies
    - Return descriptive initialization errors on nil
    - _Requirements: 8.1, 8.2, 8.3_
  
  - [ ]* 7.3 Write property test for dependency waiting
    - **Property 7: Graceful Dependency Waiting**
    - **Validates: Requirements 7.1, 7.2, 7.3, 7.4**

- [x] 8. Refactor RepoBinding controller
  - [x] 8.1 Integrate StatusHelper into RepoBinding controller
    - Replace direct Status().Update() calls with StatusHelper
    - Use PatchStatus for efficient updates
    - _Requirements: 1.1, 1.2, 1.3, 20.1, 20.2, 20.3_
  
  - [x] 8.2 Integrate FinalizerHelper into RepoBinding controller
    - Replace finalizer logic with FinalizerHelper
    - Ensure validation before finalizer addition
    - Verify cleanup before finalizer removal
    - _Requirements: 2.1, 2.2, 2.3, 18.1, 18.2, 18.3_
  
  - [x] 8.3 Refactor provisioners to use Provisioner interface
    - Implement idempotent provisioning for all resource types
    - Add timeout and context cancellation handling
    - Implement rollback on failure
    - _Requirements: 3.1, 3.2, 3.3, 3.4, 5.1, 5.2, 10.1, 10.2_
  
  - [x] 8.4 Add dependency waiting for Organization lookup
    - Use dependency resolver for Organization lookup
    - Update status to Pending when Organization not found
    - _Requirements: 7.1, 7.2, 7.3_
  
  - [x] 8.5 Add YAML and RBAC validation
    - Validate Pipeline YAML before parsing
    - Validate RBAC permissions before creating Roles
    - _Requirements: 4.1, 6.1, 11.1_
  
  - [x] 8.6 Replace hardcoded namespaces with ConfigManager
    - Use ConfigManager for allowlist namespace
    - Use ConfigManager for platform system namespace
    - _Requirements: 9.1, 9.2, 9.3_
  
  - [x] 8.7 Add cluster-scoped resource tracking
    - Label ClusterRoles and ClusterRoleBindings with owner reference
    - Clean up labeled cluster-scoped resources on deletion
    - _Requirements: 22.1, 22.2, 22.3_
  
  - [x] 8.8 Implement configurable ResourceQuota limits
    - Read limits from RepoBinding spec if provided
    - Fall back to Organization defaults
    - Fall back to hardcoded defaults
    - _Requirements: 14.1, 14.2, 14.3_

- [ ] 9. Checkpoint - Ensure RepoBinding controller works
  - Ensure all tests pass, ask the user if questions arise.

- [x] 10. Refactor Organization controller
  - [x] 10.1 Integrate shared components into Organization controller
    - Use StatusHelper for status updates
    - Use FinalizerHelper for finalizer management
    - Add null-safety checks
    - _Requirements: 1.1, 2.1, 8.1_
  
  - [x] 10.2 Ensure Organization creates all required resources
    - Create organization namespace
    - Create required RBAC resources
    - Set owner references on dependent resources
    - _Requirements: 29.2, 29.5_

- [x] 11. Refactor Agent controller
  - [x] 11.1 Integrate shared components into Agent controller
    - Use StatusHelper for status updates
    - Use FinalizerHelper for finalizer management
    - Add null-safety checks
    - _Requirements: 1.1, 2.1, 8.1_
  
  - [x] 11.2 Ensure Agent creates all required resources
    - Create deployments, services, and configurations
    - Set owner references on dependent resources
    - _Requirements: 29.3, 29.5_

- [x] 12. Refactor KnowledgeBase controller
  - [x] 12.1 Integrate shared components into KnowledgeBase controller
    - Use StatusHelper for status updates
    - Use FinalizerHelper for finalizer management
    - Add null-safety checks
    - _Requirements: 1.1, 2.1, 8.1_
  
  - [x] 12.2 Ensure KnowledgeBase creates all required resources
    - Create storage and processing resources
    - Set owner references on dependent resources
    - _Requirements: 29.4, 29.5_

- [ ] 13. Checkpoint - Ensure all controllers work
  - Ensure all tests pass, ask the user if questions arise.



- [x] 14. Implement observability components
  - [x] 14.1 Create MetricsCollector component
    - Create `controllers/metrics/metrics.go`
    - Implement reconciliation duration histogram
    - Implement reconciliation error counter
    - Implement provisioning step counter
    - Register metrics with controller-runtime
    - _Requirements: 13.1, 13.2, 13.3, 13.4_
  
  - [x] 14.2 Integrate metrics into all controllers
    - Record reconciliation duration in each controller
    - Record errors with error type labels
    - _Requirements: 13.1, 13.2_
  
  - [x] 14.3 Implement meaningful health checks
    - Create HealthState tracker
    - Record successful reconciliations
    - Report unhealthy if no recent success
    - _Requirements: 23.1, 23.2, 23.3_
  
  - [ ]* 14.4 Write property test for health check accuracy
    - **Property 18: Health Check Accuracy**
    - **Validates: Requirements 23.1, 23.2**

- [x] 15. Implement admission webhooks
  - [x] 15.1 Create ValidatingWebhookConfiguration
    - Create webhook infrastructure for all CRDs
    - Implement validation for RepoBinding
    - Implement validation for Organization
    - Implement validation for Agent
    - Implement validation for KnowledgeBase
    - _Requirements: 19.1, 19.2, 19.3_
  
  - [ ]* 15.2 Write property test for webhook validation
    - **Property 15: Webhook Validation**
    - **Validates: Requirements 19.2, 19.3**

- [x] 16. Update main.go with production configuration
  - [x] 16.1 Configure rate limiting
    - Add exponential backoff rate limiter
    - Configure max concurrent reconciles
    - _Requirements: 17.1, 17.2, 17.3_
  
  - [x] 16.2 Configure leader election
    - Enable leader election by default
    - Add flag to disable for development
    - _Requirements: 15.1, 15.2, 15.3_
  
  - [x] 16.3 Configure development mode
    - Read development mode from environment
    - Default to production mode
    - Configure logging based on mode
    - _Requirements: 24.1, 24.2, 24.3_
  
  - [x] 16.4 Implement graceful shutdown
    - Handle SIGTERM signal
    - Wait for in-progress reconciliations
    - Log shutdown progress
    - _Requirements: 25.1, 25.2, 25.3_

- [ ] 17. Checkpoint - Ensure observability and webhooks work
  - Ensure all tests pass, ask the user if questions arise.

- [x] 18. Fix error handling and logging
  - [x] 18.1 Fix inconsistent error wrapping
    - Replace %s with %w for error wrapping
    - Ensure error chains are preserved
    - Add contextual information to errors
    - _Requirements: 28.1, 28.2, 28.3_
  
  - [x] 18.2 Update logging to use structured fields
    - Replace string concatenation with structured fields
    - Include resource name, namespace, controller in all logs
    - Use appropriate log levels (DEBUG for routine, INFO for state changes)
    - _Requirements: 12.1, 12.2, 12.3, 12.4, 21.1, 21.2, 21.3_
  
  - [ ]* 18.3 Write property test for error chain preservation
    - **Property 19: Error Chain Preservation**
    - **Validates: Requirements 28.2, 28.3**

- [ ] 19. Write comprehensive tests
  - [ ]* 19.1 Write property tests for remaining properties
    - **Property 8: Null-Safe Dependency Access**
    - **Property 9: Configurable Namespace References**
    - **Property 10: Provisioning Failure Rollback**
    - **Property 11: ArgoCD Object Validation**
    - **Property 12: Resource Quota Configuration Precedence**
    - **Property 14: Validation Before Finalizer**
    - **Property 16: Efficient Status Patching**
    - **Property 17: Cluster-Scoped Resource Tracking**
    - **Property 20: Complete Resource Management**
  
  - [ ]* 19.2 Write unit tests for provisioner functions
    - Test namespace provisioner
    - Test pipeline provisioner
    - Test RBAC provisioner
    - Test ArgoCD provisioner
    - _Requirements: 27.1_
  
  - [ ]* 19.3 Write unit tests for validation functions
    - Test RepoBinding validation
    - Test Organization validation
    - Test Agent validation
    - Test KnowledgeBase validation
    - _Requirements: 27.2_
  
  - [ ]* 19.4 Write integration tests using envtest
    - Test full reconciliation flow for RepoBinding
    - Test full reconciliation flow for Organization
    - Test deletion and cleanup flow
    - _Requirements: 27.3_

- [ ] 20. Final checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.
  - Verify code coverage meets 70% target
  - _Requirements: 27.4_

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests validate universal correctness properties
- Unit tests validate specific examples and edge cases
