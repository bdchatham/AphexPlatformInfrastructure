# Requirements Document: Jenkins X Platform with Self-Service CI/CD

## Introduction

This document defines requirements for a Jenkins X platform for homelab Kubernetes clusters. The platform provides self-service repository registration with automated tenant provisioning, CDKTF deployment pipelines, and self-upgrade capabilities through its own CI/CD pipeline.

## Glossary

- **Bootstrap**: One-time initialization script that creates cluster, installs Tekton, JenkinsX, and Lighthouse
- **Tenant**: Isolated namespace with dedicated resources for a repository
- **RepoBinding**: Custom resource that declares repository-to-tenant mapping
- **Onboarding_Controller**: Kubernetes controller that provisions tenant resources based on RepoBindings
- **Platform_Catalog**: Shared Tekton Tasks and Pipelines for CDKTF deployments
- **Lighthouse**: GitHub webhook handler and pipeline trigger from Jenkins X
- **Platform_Pipeline**: Self-upgrade pipeline that manages platform component updates
- **Webhook_Secret**: Cryptographically secure secret for GitHub webhook validation

## Requirements

### Requirement 1: Platform Self-Upgrade via CI/CD

**User Story:** As a platform engineer, I want the platform to upgrade itself via CI/CD pipeline, so that platform changes are automatically deployed when I commit to the infrastructure repository.

#### Acceptance Criteria

1. THE Platform SHALL have a dedicated pipeline for self-upgrade
2. WHEN changes are committed to the platform repository, THEN Lighthouse SHALL trigger the platform upgrade pipeline
3. THE Platform_Pipeline SHALL apply Kubernetes manifests for platform components
4. THE Platform_Pipeline SHALL upgrade Helm releases for Tekton, Lighthouse, and other components
5. THE Platform SHALL maintain all component definitions in Git (version-controlled)

### Requirement 2: Single-Command Bootstrap

**User Story:** As a platform engineer, I want a single bootstrap command, so that I can quickly initialize new clusters without running multiple scripts.

#### Acceptance Criteria

1. THE Bootstrap_Script SHALL create a Kubernetes cluster (Kind for local, configurable for other providers)
2. THE Bootstrap_Script SHALL install Tekton Pipelines
3. THE Bootstrap_Script SHALL install Jenkins X and Lighthouse
4. THE Bootstrap_Script SHALL generate a webhook secret for the platform repository
5. WHEN bootstrap completes, THEN the script SHALL display the webhook secret and GitHub configuration instructions

### Requirement 3: Platform Repository Registration

**User Story:** As a platform engineer, I want the platform repository automatically registered during bootstrap, so that platform upgrades work immediately after installation.

#### Acceptance Criteria

1. THE Bootstrap_Script SHALL create a RepoBinding for the platform repository
2. THE RepoBinding SHALL provision a platform-infra tenant namespace
3. THE Platform SHALL deploy the platform upgrade pipeline to the platform-infra namespace
4. THE Platform SHALL configure Lighthouse to accept webhooks for the platform repository
5. THE Bootstrap_Script SHALL display webhook configuration instructions with the generated secret

### Requirement 4: Platform Upgrade Pipeline

**User Story:** As a platform engineer, I want a pipeline that upgrades platform components, so that I can deploy platform changes by committing to Git.

#### Acceptance Criteria

1. THE Platform_Pipeline SHALL apply CRDs and namespace manifests
2. THE Platform_Pipeline SHALL upgrade Tekton Pipelines via kubectl apply
3. THE Platform_Pipeline SHALL upgrade Lighthouse via Helm
4. THE Platform_Pipeline SHALL apply onboarding controller manifests
5. THE Platform_Pipeline SHALL apply pipeline catalog Tasks and Pipelines

### Requirement 5: Self-Service Repository Registration

**User Story:** As a developer, I want to register my repository by creating a RepoBinding, so that I get a fully configured tenant with webhook integration.

#### Acceptance Criteria

1. WHEN a developer creates a RepoBinding resource, THEN the Onboarding_Controller SHALL provision a tenant namespace
2. THE Onboarding_Controller SHALL generate a webhook secret for the repository
3. THE Onboarding_Controller SHALL create a service account with appropriate RBAC
4. THE Onboarding_Controller SHALL create ResourceQuota and LimitRange for the tenant
5. THE Onboarding_Controller SHALL create NetworkPolicy for tenant isolation
6. THE Onboarding_Controller SHALL update the Lighthouse allowlist to enable webhooks
7. THE Onboarding_Controller SHALL update RepoBinding status with webhook secret and configuration instructions

### Requirement 6: Tenant Isolation

**User Story:** As a platform engineer, I want tenants isolated from each other, so that one tenant cannot access or interfere with another tenant's resources.

#### Acceptance Criteria

1. THE Platform SHALL create separate namespaces for each tenant
2. THE Platform SHALL enforce RBAC to prevent cross-tenant access
3. THE Platform SHALL enforce NetworkPolicy to prevent cross-tenant network traffic
4. THE Platform SHALL enforce ResourceQuota to prevent resource exhaustion
5. WHEN a tenant attempts cross-namespace access, THEN Kubernetes SHALL deny the request

### Requirement 7: CDKTF Deployment Pipelines

**User Story:** As a developer, I want automated CDKTF deployment pipelines, so that infrastructure changes are deployed when I merge to main.

#### Acceptance Criteria

1. WHEN a commit is merged to main, THEN Lighthouse SHALL trigger a PipelineRun in the tenant namespace
2. THE Pipeline SHALL clone the repository at the commit SHA
3. THE Pipeline SHALL run cdktf synth to generate Terraform configuration
4. THE Pipeline SHALL run cdktf deploy to apply infrastructure changes
5. THE Pipeline SHALL use Terraform state stored in Kubernetes backend

### Requirement 8: Webhook Secret Management

**User Story:** As a platform engineer, I want secure webhook secret generation, so that GitHub webhooks are properly authenticated.

#### Acceptance Criteria

1. WHEN a repository is registered, THEN the Onboarding_Controller SHALL generate a cryptographically secure webhook secret
2. THE Onboarding_Controller SHALL store the webhook secret in a Kubernetes Secret
3. THE Onboarding_Controller SHALL update RepoBinding status with the webhook secret
4. WHEN a GitHub webhook is received, THEN Lighthouse SHALL validate the webhook signature
5. WHEN a webhook is for a disallowed repository, THEN Lighthouse SHALL reject it

### Requirement 9: Observability and Monitoring

**User Story:** As a platform engineer, I want visibility into platform health, so that I can detect and resolve issues quickly.

#### Acceptance Criteria

1. THE Platform SHALL expose Tekton Dashboard for pipeline visibility
2. THE Platform SHALL log all pipeline executions and errors
3. WHEN a PipelineRun fails, THEN Tekton SHALL report the error in the PipelineRun status
4. THE Platform SHALL use Kubernetes events for component status
5. THE Platform SHALL provide kubectl commands for troubleshooting in documentation

### Requirement 10: Secrets Management

**User Story:** As a platform engineer, I want secure secrets management, so that sensitive credentials are properly isolated per tenant.

#### Acceptance Criteria

1. THE Platform SHALL NOT store secrets in Git repository
2. THE Platform SHALL create tenant-specific webhook secrets during registration
3. THE Onboarding_Controller SHALL generate webhook secrets using cryptographic randomness
4. THE Platform SHALL store GitHub App credentials in Kubernetes Secrets
5. THE Platform SHALL isolate tenant secrets to their respective namespaces

### Requirement 11: Disaster Recovery

**User Story:** As a platform engineer, I want to recover from cluster failure, so that I can restore the platform quickly.

#### Acceptance Criteria

1. WHEN a cluster is lost, THEN running bootstrap on a new cluster SHALL restore all platform components
2. THE Platform SHALL store all configuration in Git (no cluster-specific state except secrets)
3. THE Platform SHALL document backup procedures for GitHub App credentials
4. WHEN bootstrap completes, THEN all platform components SHALL be ready
5. THE Platform SHALL support re-registering repositories by reapplying RepoBinding resources

### Requirement 12: Development Workflow

**User Story:** As a platform engineer, I want to test platform changes safely, so that I can validate changes before applying to production.

#### Acceptance Criteria

1. THE Platform SHALL support local Kind clusters for development
2. WHEN testing changes, THEN engineers SHALL use Git branches and test clusters
3. THE Platform SHALL document the change testing workflow
4. THE Platform SHALL support running the platform upgrade pipeline manually for testing
5. THE Platform SHALL provide verification scripts for testing platform components

### Requirement 13: Documentation and Runbooks

**User Story:** As a platform engineer, I want clear documentation, so that I can operate and troubleshoot the platform.

#### Acceptance Criteria

1. THE Platform SHALL document the bootstrap process
2. THE Platform SHALL document the platform self-upgrade workflow
3. THE Platform SHALL document common troubleshooting scenarios
4. THE Platform SHALL document the repository registration process
5. THE Platform SHALL maintain documentation in `.kiro/docs/`

### Requirement 14: Upgrade and Maintenance

**User Story:** As a platform engineer, I want to upgrade platform components safely, so that I can keep the platform current without downtime.

#### Acceptance Criteria

1. WHEN upgrading a component, THEN the engineer SHALL update the version in Git and commit
2. WHEN the platform upgrade pipeline runs, THEN components SHALL upgrade gracefully
3. THE Platform SHALL use versioned Tekton release manifests
4. THE Platform SHALL use Helm chart versions for Lighthouse
5. THE Platform SHALL support rollback via Git revert and pipeline re-run

### Requirement 15: Minimal Custom Code

**User Story:** As a platform engineer, I want minimal custom code, so that the platform is easier to maintain and understand.

#### Acceptance Criteria

1. THE Platform SHALL use a single bootstrap script for initialization
2. THE Platform SHALL use standard Kubernetes and Tekton patterns
3. THE Platform SHALL leverage existing tools (Helm, kubectl, Tekton) instead of custom scripts
4. THE Platform SHALL minimize the onboarding controller code to core logic only
5. THE Platform SHALL document any custom code with clear rationale
