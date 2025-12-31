# Requirements Document

## Introduction

The Arbiter Pipeline Infrastructure provides a shared CI/CD platform cluster for multiple product teams using Jenkins X, Lighthouse, and Tekton. The system enables product teams to run pipelines on shared infrastructure with logical separation through namespaces, RBAC, and policies. It provides self-service onboarding for approved users to register repositories and automatically provision the resources required for safe pipeline execution. The platform is designed for homelab deployment with high inspectability to enable automated agents (Arbiter/Archon) to reason about operations, failures, and remediation.

## Glossary

- **Jenkins X**: Kubernetes-native CI/CD platform built on Tekton
- **Lighthouse**: Git event handler component of Jenkins X that processes webhooks
- **Tekton**: Kubernetes-native pipeline execution framework
- **Tenant**: A product team with isolated namespace and pipeline resources
- **Tenant Namespace**: Kubernetes namespace dedicated to a single product team
- **Pipeline Runner**: Service account used for pipeline execution within a tenant namespace
- **Allowlist**: Security boundary defining which repositories can trigger pipelines
- **Golden Pipeline Catalog**: Shared, versioned Tekton Tasks and Pipelines maintained by platform team
- **Onboarding**: Process of provisioning tenant resources for a new repository
- **OIDC Group**: OpenID Connect group used for authentication and authorization
- **CDKTF**: Cloud Development Kit for Terraform - infrastructure as code framework
- **PipelineRun**: Tekton resource representing a single pipeline execution
- **GitHub App**: GitHub integration mechanism for receiving repository events
- **Remote State**: Terraform state stored externally (not locally) for persistence
- **RepoBinding**: Custom Resource Definition for repository onboarding requests
- **Platform Namespace**: Kubernetes namespace containing platform-owned components
- **Runner Image**: Container image with tools (node, cdktf, terraform, kubectl) for pipeline execution

## Requirements

### Requirement 1

**User Story:** As a platform engineer, I want to bootstrap the Jenkins X platform on a fresh cluster, so that the CI/CD infrastructure is ready for product teams.

#### Acceptance Criteria

1. WHEN the platform engineer runs the bootstrap command THEN the system SHALL install Jenkins X core components in the platform namespace
2. WHEN Jenkins X is installed THEN the system SHALL install Tekton controllers in the platform namespace
3. WHEN Tekton is installed THEN the system SHALL install Lighthouse for Git event handling
4. WHEN Lighthouse is installed THEN the system SHALL configure the GitHub App integration
5. WHEN the GitHub App is configured THEN the system SHALL create the platform namespaces (pipeline-system, pipeline-catalog)
6. WHEN platform namespaces are created THEN the system SHALL install the golden pipeline catalog with shared Tasks and Pipelines
7. WHEN the bootstrap completes THEN the system SHALL create platform documentation and runbooks

### Requirement 2

**User Story:** As a platform engineer, I want to configure GitHub App integration at the organization level, so that repository webhooks are managed centrally without per-repo configuration.

#### Acceptance Criteria

1. WHEN the platform is bootstrapped THEN the system SHALL support GitHub App installation at the organization level
2. WHEN the GitHub App is installed THEN the system SHALL configure Lighthouse to receive events from the GitHub App
3. WHEN a repository event occurs THEN the GitHub App SHALL deliver the event to Lighthouse
4. WHEN Lighthouse receives an event THEN the system SHALL validate the event signature
5. WHEN the event signature is valid THEN the system SHALL process the event according to trigger rules

### Requirement 3

**User Story:** As a platform engineer, I want to maintain a repository allowlist, so that only approved repositories can trigger pipelines on the platform.

#### Acceptance Criteria

1. WHEN the platform is configured THEN the system SHALL maintain an allowlist of approved repositories
2. WHEN the allowlist is updated THEN the system SHALL reload the configuration without requiring platform redeployment
3. WHEN Lighthouse receives an event from a repository THEN the system SHALL check if the repository is in the allowlist
4. WHEN a repository is not in the allowlist THEN the system SHALL reject the event and log the rejection
5. WHEN a repository is in the allowlist THEN the system SHALL process the event according to trigger rules

### Requirement 4

**User Story:** As a user in the engineering OIDC group, I want to request onboarding for my repository, so that pipeline resources are provisioned without requiring cluster admin access.

#### Acceptance Criteria

1. WHEN a user in the engineering OIDC group submits an onboarding request THEN the system SHALL authenticate the user via OIDC
2. WHEN the user is authenticated THEN the system SHALL validate the user belongs to the engineering group
3. WHEN the user is authorized THEN the system SHALL accept the onboarding request
4. WHEN a user not in the engineering group submits a request THEN the system SHALL reject the request
5. WHEN an onboarding request is accepted THEN the system SHALL provision tenant resources without requiring cluster admin privileges for the user

### Requirement 5

**User Story:** As a platform engineer, I want onboarding to provision tenant namespaces, so that each product team has isolated resources.

#### Acceptance Criteria

1. WHEN an onboarding request is processed for repository org/archon-agent THEN the system SHALL create a tenant namespace with name archon or tenant-archon
2. WHEN the tenant namespace is created THEN the system SHALL apply labels identifying the tenant and repository
3. WHEN the namespace is created THEN the system SHALL create a ResourceQuota to limit resource consumption
4. WHEN the ResourceQuota is created THEN the system SHALL create a LimitRange with default resource limits
5. WHEN resource limits are configured THEN the system SHALL create a NetworkPolicy for namespace isolation

### Requirement 6

**User Story:** As a platform engineer, I want onboarding to create tenant service accounts, so that pipelines execute with least privilege.

#### Acceptance Criteria

1. WHEN a tenant namespace is created THEN the system SHALL create a service account named pipeline-runner in the tenant namespace
2. WHEN the service account is created THEN the system SHALL create a Role with permissions scoped to the tenant namespace
3. WHEN the Role is created THEN the system SHALL create a RoleBinding associating the service account with the Role
4. WHEN RBAC is configured THEN the system SHALL verify the service account cannot access resources in other namespaces
5. WHEN RBAC is configured THEN the system SHALL verify the service account cannot access platform namespace resources

### Requirement 7

**User Story:** As a platform engineer, I want onboarding to configure Terraform backend references, so that tenants can use remote state without managing credentials.

#### Acceptance Criteria

1. WHEN a tenant namespace is created THEN the system SHALL create secret references for Terraform backend configuration
2. WHEN backend secrets are configured THEN the system SHALL use ExternalSecrets or equivalent for credential management
3. WHEN credentials are configured THEN the system SHALL ensure no secret material is stored in the onboarding request
4. WHEN backend configuration is complete THEN the system SHALL verify the tenant can access remote state
5. WHEN multiple tenants exist THEN the system SHALL ensure Terraform state is isolated per tenant

### Requirement 8

**User Story:** As a platform engineer, I want onboarding to update the repository allowlist, so that the newly onboarded repository can trigger pipelines.

#### Acceptance Criteria

1. WHEN onboarding completes for a repository THEN the system SHALL add the repository to the allowlist
2. WHEN the repository is added to the allowlist THEN the system SHALL configure Lighthouse to accept events from the repository
3. WHEN Lighthouse is configured THEN the system SHALL map the repository to the tenant namespace
4. WHEN the mapping is configured THEN the system SHALL map the repository to the tenant service account
5. WHEN all mappings are complete THEN the system SHALL verify events from the repository trigger PipelineRuns in the tenant namespace

### Requirement 9

**User Story:** As a platform engineer, I want onboarding to enforce guardrails, so that tenant requests cannot compromise platform security.

#### Acceptance Criteria

1. WHEN an onboarding request specifies a repository THEN the system SHALL validate the repository is from an approved organization
2. WHEN an onboarding request specifies a namespace THEN the system SHALL validate the namespace matches a safe pattern
3. WHEN a namespace is validated THEN the system SHALL reject privileged namespace names (kube-system, pipeline-system, etc.)
4. WHEN an onboarding request specifies permissions THEN the system SHALL use predefined permission profiles only
5. WHEN an onboarding request is resubmitted THEN the system SHALL converge idempotently without destructive changes

### Requirement 10

**User Story:** As a product team developer, I want to define pipelines in my repository, so that the platform knows how to build and deploy my application.

#### Acceptance Criteria

1. WHEN a product repository is onboarded THEN the repository SHALL contain pipeline definition YAML following Tekton conventions
2. WHEN a repository contains pipeline definitions THEN the definitions SHALL specify trigger rules (e.g., on merge to main)
3. WHEN a repository contains CDKTF code THEN the pipeline SHALL reference the golden pipeline catalog for CDKTF deployment
4. WHEN pipeline definitions are committed THEN the system SHALL validate the YAML syntax
5. WHEN pipeline definitions reference the catalog THEN the system SHALL verify the referenced Tasks and Pipelines exist

### Requirement 11

**User Story:** As a product team developer, I want to merge code to main and trigger a pipeline, so that my changes are automatically deployed.

#### Acceptance Criteria

1. WHEN a developer merges code to the main branch THEN the GitHub App SHALL deliver a push event to Lighthouse
2. WHEN Lighthouse receives the push event THEN the system SHALL validate the repository is allowlisted
3. WHEN the repository is allowlisted THEN the system SHALL evaluate trigger rules from the repository pipeline definitions
4. WHEN trigger rules match the event THEN the system SHALL create a Tekton PipelineRun in the tenant namespace
5. WHEN the PipelineRun is created THEN the system SHALL execute the pipeline using the tenant service account

### Requirement 12

**User Story:** As a product team developer, I want my pipeline to execute CDKTF deployment, so that infrastructure changes are applied automatically.

#### Acceptance Criteria

1. WHEN a PipelineRun executes THEN the pipeline SHALL checkout the repository at the commit SHA that triggered the event
2. WHEN the repository is checked out THEN the pipeline SHALL run cdktf synth to generate Terraform configuration
3. WHEN cdktf synth completes THEN the pipeline SHALL run cdktf deploy to apply infrastructure changes
4. WHEN cdktf deploy executes THEN the system SHALL use remote Terraform state configured for the tenant
5. WHEN deployment completes THEN the pipeline SHALL store outputs, logs, and artifacts externally for inspection

### Requirement 13

**User Story:** As a platform engineer, I want to provide a golden pipeline catalog, so that product teams can reference shared, versioned pipeline components.

#### Acceptance Criteria

1. WHEN the platform is bootstrapped THEN the system SHALL create a pipeline-catalog namespace
2. WHEN the catalog namespace is created THEN the system SHALL install shared Tekton Tasks for common operations (build, test, cdktf)
3. WHEN Tasks are installed THEN the system SHALL install shared Tekton Pipelines for standard workflows (cdktf deploy pipeline)
4. WHEN Pipelines are installed THEN the system SHALL publish runner images containing node, cdktf, terraform, and kubectl
5. WHEN catalog components are versioned THEN the system SHALL allow tenants to reference specific versions or latest

### Requirement 14

**User Story:** As a platform engineer, I want to enforce namespace isolation, so that tenants cannot access each other's resources.

#### Acceptance Criteria

1. WHEN multiple tenant namespaces exist THEN the system SHALL enforce RBAC preventing cross-namespace resource access
2. WHEN a tenant pipeline executes THEN the system SHALL enforce ResourceQuota limits to prevent resource exhaustion
3. WHEN a tenant pipeline executes THEN the system SHALL enforce LimitRange defaults for pod resource requests
4. WHEN a tenant pipeline executes THEN the system SHALL enforce NetworkPolicy rules preventing cross-namespace network access
5. WHEN a tenant attempts to access another tenant's resources THEN the system SHALL deny the request and log the attempt

### Requirement 15

**User Story:** As a platform engineer, I want to serialize CDKTF deployments per tenant, so that Terraform state is not corrupted by concurrent operations.

#### Acceptance Criteria

1. WHEN multiple PipelineRuns execute for the same tenant THEN the system SHALL serialize cdktf deploy operations
2. WHEN a cdktf deploy is in progress THEN the system SHALL queue subsequent deploy requests for the same tenant
3. WHEN a cdktf deploy completes THEN the system SHALL process the next queued deploy request
4. WHEN deployments are serialized THEN the system SHALL prevent Terraform state lock conflicts
5. WHEN different tenants deploy concurrently THEN the system SHALL allow parallel execution across tenants

### Requirement 16

**User Story:** As a platform engineer, I want comprehensive observability, so that I can debug pipeline failures quickly.

#### Acceptance Criteria

1. WHEN a PipelineRun executes THEN the system SHALL capture all logs and make them accessible via kubectl
2. WHEN a PipelineRun fails THEN the system SHALL provide clear error messages indicating the failure reason
3. WHEN a repository is not allowlisted THEN the system SHALL log the rejection with repository details
4. WHEN RBAC denies an operation THEN the system SHALL log the denial with service account and resource details
5. WHEN onboarding fails THEN the system SHALL update the onboarding request status with failure details

### Requirement 17

**User Story:** As an automated agent (Archon), I want to query why a repository isn't triggering, so that I can diagnose and fix configuration issues.

#### Acceptance Criteria

1. WHEN a repository is not triggering THEN the agent SHALL query the allowlist to verify the repository is included
2. WHEN the repository is allowlisted THEN the agent SHALL query Lighthouse configuration to verify event routing
3. WHEN event routing is configured THEN the agent SHALL query the tenant namespace to verify it exists
4. WHEN the namespace exists THEN the agent SHALL query RBAC to verify the service account has required permissions
5. WHEN all checks pass THEN the agent SHALL query recent events to verify the GitHub App is delivering webhooks

### Requirement 18

**User Story:** As an automated agent (Archon), I want to query why a PipelineRun failed, so that I can identify and resolve the root cause.

#### Acceptance Criteria

1. WHEN a PipelineRun fails THEN the agent SHALL query the PipelineRun status to retrieve the failure reason
2. WHEN the failure reason is retrieved THEN the agent SHALL query the PipelineRun logs to retrieve detailed error messages
3. WHEN logs are retrieved THEN the agent SHALL query the pod events to identify infrastructure issues
4. WHEN pod events are retrieved THEN the agent SHALL query RBAC to verify the service account has required permissions
5. WHEN all diagnostics are complete THEN the agent SHALL have sufficient information to determine remediation steps

### Requirement 19

**User Story:** As an automated agent (Archon), I want to verify tenant namespace provisioning, so that I can confirm onboarding completed successfully.

#### Acceptance Criteria

1. WHEN onboarding completes THEN the agent SHALL query the tenant namespace to verify it exists
2. WHEN the namespace exists THEN the agent SHALL query the service account to verify it was created
3. WHEN the service account exists THEN the agent SHALL query RBAC to verify Roles and RoleBindings are configured
4. WHEN RBAC is configured THEN the agent SHALL query ResourceQuota and LimitRange to verify resource limits
5. WHEN all resources are verified THEN the agent SHALL confirm the tenant is ready for pipeline execution

### Requirement 20

**User Story:** As a platform engineer, I want the platform to be reproducible, so that I can deploy identical environments reliably.

#### Acceptance Criteria

1. WHEN the platform is deployed THEN the system SHALL use deterministic configuration (Helm charts, Kustomize, or CDKTF)
2. WHEN the same configuration is applied twice THEN the system SHALL produce identical results
3. WHEN the platform is upgraded THEN the system SHALL apply changes without manual intervention
4. WHEN the platform is deployed to a new cluster THEN the system SHALL bootstrap successfully from the repository
5. WHEN the bootstrap completes THEN the system SHALL verify all components are healthy and ready

