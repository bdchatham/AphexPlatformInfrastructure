# Requirements Document: Jenkins X GitOps Platform

## Introduction

This document defines requirements for a GitOps-based Jenkins X platform for homelab Kubernetes clusters. The platform provides self-service repository onboarding with automated tenant provisioning and CDKTF deployment pipelines, managed entirely through GitOps principles using ArgoCD.

## Glossary

- **GitOps**: Declarative infrastructure and application management using Git as the single source of truth
- **ArgoCD**: Kubernetes-native GitOps continuous delivery tool
- **Bootstrap**: One-time initialization process that creates cluster and installs ArgoCD
- **Application**: ArgoCD resource that defines what to deploy and where
- **Sync**: Process of reconciling cluster state with Git repository state
- **Tenant**: Isolated namespace with dedicated resources for a repository
- **RepoBinding**: Custom resource that declares repository-to-tenant mapping
- **Onboarding_Controller**: Kubernetes controller that provisions tenant resources based on RepoBindings
- **Platform_Assets**: Shared Tekton Tasks and Pipelines for CDKTF deployments
- **Lighthouse**: GitHub webhook handler and pipeline trigger

## Requirements

### Requirement 1: GitOps-Based Platform Management

**User Story:** As a platform engineer, I want all platform components managed via GitOps, so that infrastructure changes are version-controlled, auditable, and automatically applied.

#### Acceptance Criteria

1. THE Platform SHALL use ArgoCD as the GitOps operator
2. WHEN platform manifests are committed to Git, THEN ArgoCD SHALL automatically sync changes to the cluster
3. THE Platform SHALL maintain all component definitions in Git (no manual kubectl apply)
4. WHEN cluster state drifts from Git, THEN ArgoCD SHALL detect and report the drift
5. THE Platform SHALL support rollback via Git revert operations

### Requirement 2: Minimal Bootstrap Process

**User Story:** As a platform engineer, I want a minimal bootstrap process, so that I can quickly initialize new clusters without complex scripts.

#### Acceptance Criteria

1. THE Bootstrap_Script SHALL create a Kubernetes cluster (Kind for local, configurable for other providers)
2. THE Bootstrap_Script SHALL install ArgoCD
3. THE Bootstrap_Script SHALL configure ArgoCD to watch the platform repository
4. THE Bootstrap_Script SHALL NOT install platform components directly (ArgoCD handles this)
5. WHEN bootstrap completes, THEN ArgoCD SHALL automatically deploy all platform components

### Requirement 3: ArgoCD Application Structure

**User Story:** As a platform engineer, I want a clear ArgoCD Application structure, so that I can understand and manage component dependencies.

#### Acceptance Criteria

1. THE Platform SHALL define an "app-of-apps" pattern with a root Application
2. THE Root_Application SHALL manage child Applications for each platform component
3. WHEN a child Application is added to Git, THEN ArgoCD SHALL automatically create and sync it
4. THE Platform SHALL organize Applications by concern (infrastructure, platform, tenants)
5. THE Platform SHALL define sync waves to control deployment order

### Requirement 4: Declarative Component Management

**User Story:** As a platform engineer, I want all components defined declaratively, so that I can manage them through Git without custom scripts.

#### Acceptance Criteria

1. THE Platform SHALL use ArgoCD Applications for Helm-based components (Tekton, Lighthouse)
2. THE Platform SHALL use Kustomize for custom components (CRDs, controllers, Dex)
3. THE Platform SHALL NOT require custom installation scripts for components
4. WHEN component configuration changes in Git, THEN ArgoCD SHALL apply the changes automatically
5. THE Platform SHALL use HelmRelease or Application resources instead of helm CLI commands

### Requirement 5: Self-Service Repository Onboarding

**User Story:** As a developer, I want to onboard my repository by creating a RepoBinding, so that I get an isolated tenant namespace with pipeline access.

#### Acceptance Criteria

1. WHEN a developer creates a RepoBinding resource, THEN the Onboarding_Controller SHALL provision a tenant namespace
2. THE Onboarding_Controller SHALL create a service account with appropriate RBAC
3. THE Onboarding_Controller SHALL create ResourceQuota and LimitRange for the tenant
4. THE Onboarding_Controller SHALL create NetworkPolicy for tenant isolation
5. THE Onboarding_Controller SHALL update the Lighthouse allowlist to enable webhooks

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

### Requirement 8: GitHub Webhook Integration

**User Story:** As a developer, I want GitHub webhooks to trigger pipelines, so that deployments happen automatically on merge.

#### Acceptance Criteria

1. THE Platform SHALL deploy Lighthouse to handle GitHub webhooks
2. WHEN a GitHub webhook is received, THEN Lighthouse SHALL validate the signature
3. WHEN a webhook is for an allowed repository, THEN Lighthouse SHALL create a PipelineRun
4. WHEN a webhook is for a disallowed repository, THEN Lighthouse SHALL reject it
5. THE Platform SHALL maintain an allowlist of repositories in a ConfigMap

### Requirement 9: Observability and Monitoring

**User Story:** As a platform engineer, I want visibility into platform health, so that I can detect and resolve issues quickly.

#### Acceptance Criteria

1. THE Platform SHALL expose ArgoCD UI for GitOps status
2. THE Platform SHALL log all sync operations and errors
3. WHEN an Application fails to sync, THEN ArgoCD SHALL report the error
4. THE Platform SHALL expose Tekton Dashboard for pipeline visibility
5. THE Platform SHALL use Kubernetes events for component status

### Requirement 10: Secrets Management

**User Story:** As a platform engineer, I want secure secrets management, so that sensitive credentials are not stored in Git.

#### Acceptance Criteria

1. THE Platform SHALL NOT store secrets in Git repository
2. THE Platform SHALL use Kubernetes Secrets for sensitive data
3. THE Platform SHALL document manual secret creation steps
4. WHERE sealed secrets or external secrets operator is configured, THE Platform SHALL support automated secret management
5. THE Bootstrap_Script SHALL prompt for required secrets during initialization

### Requirement 11: Disaster Recovery

**User Story:** As a platform engineer, I want to recover from cluster failure, so that I can restore the platform quickly.

#### Acceptance Criteria

1. WHEN a cluster is lost, THEN running bootstrap on a new cluster SHALL restore all platform components
2. THE Platform SHALL store all configuration in Git (no cluster-specific state)
3. THE Platform SHALL document backup procedures for secrets
4. WHEN ArgoCD syncs after bootstrap, THEN all Applications SHALL reach healthy state
5. THE Platform SHALL support exporting and importing RepoBinding resources

### Requirement 12: Development Workflow

**User Story:** As a platform engineer, I want to test platform changes safely, so that I can validate changes before applying to production.

#### Acceptance Criteria

1. THE Platform SHALL support multiple ArgoCD Applications for different environments
2. WHEN testing changes, THEN engineers SHALL use Git branches and ArgoCD app-of-apps
3. THE Platform SHALL support local Kind clusters for development
4. THE Platform SHALL document the change testing workflow
5. THE Platform SHALL use ArgoCD sync policies (manual vs automatic) appropriately

### Requirement 13: Documentation and Runbooks

**User Story:** As a platform engineer, I want clear documentation, so that I can operate and troubleshoot the platform.

#### Acceptance Criteria

1. THE Platform SHALL document the bootstrap process
2. THE Platform SHALL document the GitOps workflow for making changes
3. THE Platform SHALL document common troubleshooting scenarios
4. THE Platform SHALL document the ArgoCD Application structure
5. THE Platform SHALL maintain documentation in `.kiro/docs/`

### Requirement 14: Upgrade and Maintenance

**User Story:** As a platform engineer, I want to upgrade platform components safely, so that I can keep the platform current without downtime.

#### Acceptance Criteria

1. WHEN upgrading a component, THEN the engineer SHALL update the version in Git
2. WHEN ArgoCD syncs the change, THEN the component SHALL upgrade gracefully
3. THE Platform SHALL use Helm chart versions for third-party components
4. THE Platform SHALL use image tags for custom components
5. THE Platform SHALL support rollback via Git revert and ArgoCD rollback

### Requirement 15: Minimal Custom Code

**User Story:** As a platform engineer, I want minimal custom code, so that the platform is easier to maintain and understand.

#### Acceptance Criteria

1. THE Platform SHALL eliminate custom installation scripts (except bootstrap)
2. THE Platform SHALL use standard Kubernetes and ArgoCD patterns
3. THE Platform SHALL leverage existing tools (Helm, Kustomize) instead of custom scripts
4. THE Platform SHALL minimize the onboarding controller code to core logic only
5. THE Platform SHALL document any custom code with clear rationale
