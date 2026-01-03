# Requirements Document: ArgoCD + Tekton GitOps Platform

## Introduction

This document defines requirements for a lightweight GitOps platform using ArgoCD and Tekton for homelab Kubernetes clusters. The platform provides self-service repository registration with automated tenant provisioning, CDKTF deployment pipelines, and self-upgrade capabilities through GitOps.

## Glossary

- **Bootstrap**: One-time initialization script that creates cluster, installs Tekton, ArgoCD, and platform components
- **Tenant**: Isolated namespace with dedicated resources for a repository
- **RepoBinding**: Custom resource that declares repository-to-tenant mapping
- **Onboarding_Controller**: Kubernetes controller that provisions tenant resources based on RepoBindings
- **Platform_Catalog**: Shared Tekton Tasks and Pipelines for CDKTF deployments
- **ArgoCD**: GitOps continuous delivery tool that syncs Kubernetes resources from Git
- **Platform_Application**: ArgoCD Application that manages platform component upgrades
- **Webhook_Secret**: Cryptographically secure secret for GitHub webhook validation
- **EventListener**: Tekton Triggers component that receives GitHub webhooks and creates PipelineRuns

## Requirements

### Requirement 1: Platform Self-Upgrade via GitOps

**User Story:** As a platform engineer, I want the platform to upgrade itself via ArgoCD, so that platform changes are automatically deployed when I commit to the infrastructure repository.

#### Acceptance Criteria

1. THE Platform SHALL have an ArgoCD Application for self-management
2. WHEN changes are committed to the platform repository, THEN ArgoCD SHALL detect and sync the changes
3. THE Platform_Application SHALL manage Kubernetes manifests for platform components
4. THE Platform_Application SHALL use Helm charts where appropriate (ArgoCD, Tekton Dashboard)
5. THE Platform SHALL maintain all component definitions in Git (version-controlled)
6. THE Platform_Application SHALL sync automatically on Git commits (no manual intervention)

### Requirement 2: Single-Command Bootstrap

**User Story:** As a platform engineer, I want a single bootstrap command, so that I can quickly initialize new clusters without running multiple scripts.

#### Acceptance Criteria

1. THE Bootstrap_Script SHALL create a Kubernetes cluster (Kind for local, configurable for other providers)
2. THE Bootstrap_Script SHALL install Tekton Pipelines and Tekton Triggers
3. THE Bootstrap_Script SHALL install ArgoCD
4. THE Bootstrap_Script SHALL create platform namespaces (argocd, tekton-pipelines, platform-system)
5. THE Bootstrap_Script SHALL create the platform ArgoCD Application pointing to the platform repository
6. WHEN bootstrap completes, THEN the script SHALL display ArgoCD access information and next steps

### Requirement 3: Platform Repository Registration

**User Story:** As a platform engineer, I want the platform repository automatically registered during bootstrap, so that platform upgrades work immediately after installation.

#### Acceptance Criteria

1. THE Bootstrap_Script SHALL create an ArgoCD Application for the platform repository
2. THE Platform_Application SHALL point to the platform manifests directory in Git
3. THE Platform_Application SHALL sync automatically on Git commits
4. THE Platform_Application SHALL manage CRDs, namespaces, controllers, and catalog
5. THE Bootstrap_Script SHALL display ArgoCD UI access instructions

### Requirement 4: ArgoCD-Based Platform Management

**User Story:** As a platform engineer, I want ArgoCD to manage platform components, so that all platform changes are declarative and version-controlled.

#### Acceptance Criteria

1. THE Platform_Application SHALL manage the Onboarding Controller deployment
2. THE Platform_Application SHALL manage the Pipeline Catalog (Tasks and Pipelines)
3. THE Platform_Application SHALL manage platform CRDs (RepoBinding)
4. THE Platform_Application SHALL manage platform namespaces
5. WHEN manifests change in Git, THEN ArgoCD SHALL sync within 3 minutes (default polling interval)
6. THE Platform SHALL support manual sync via ArgoCD UI or CLI

### Requirement 5: Self-Service Repository Registration

**User Story:** As a developer, I want to register my repository by creating a RepoBinding, so that I get a fully configured tenant with webhook integration.

#### Acceptance Criteria

1. WHEN a developer creates a RepoBinding resource, THEN the Onboarding_Controller SHALL provision a tenant namespace
2. THE Onboarding_Controller SHALL generate a webhook secret for the repository
3. THE Onboarding_Controller SHALL create a service account with appropriate RBAC
4. THE Onboarding_Controller SHALL create ResourceQuota and LimitRange for the tenant
5. THE Onboarding_Controller SHALL create NetworkPolicy for tenant isolation
6. THE Onboarding_Controller SHALL create a Tekton EventListener for the tenant
7. THE Onboarding_Controller SHALL create an Ingress or Route for the EventListener webhook endpoint
8. THE Onboarding_Controller SHALL update RepoBinding status with webhook URL and secret

### Requirement 6: Tenant Isolation

**User Story:** As a platform engineer, I want tenants isolated from each other, so that one tenant cannot access or interfere with another tenant's resources.

#### Acceptance Criteria

1. THE Platform SHALL create separate namespaces for each tenant
2. THE Platform SHALL enforce RBAC to prevent cross-tenant access
3. THE Platform SHALL enforce NetworkPolicy to prevent cross-tenant network traffic
4. THE Platform SHALL enforce ResourceQuota to prevent resource exhaustion
5. WHEN a tenant attempts cross-namespace access, THEN Kubernetes SHALL deny the request

### Requirement 7: Webhook-Driven CDKTF Pipelines

**User Story:** As a developer, I want automated CDKTF deployment pipelines triggered by GitHub webhooks, so that infrastructure changes are deployed when I merge to main.

#### Acceptance Criteria

1. WHEN a GitHub webhook is received at the tenant EventListener, THEN Tekton Triggers SHALL validate the webhook signature
2. WHEN the webhook is valid, THEN Tekton Triggers SHALL create a PipelineRun in the tenant namespace
3. THE Pipeline SHALL clone the repository at the commit SHA
4. THE Pipeline SHALL run cdktf synth to generate Terraform configuration
5. THE Pipeline SHALL run cdktf deploy to apply infrastructure changes
6. THE Pipeline SHALL use Terraform state stored in Kubernetes backend

### Requirement 8: Webhook Secret Management

**User Story:** As a platform engineer, I want secure webhook secret generation, so that GitHub webhooks are properly authenticated.

#### Acceptance Criteria

1. WHEN a repository is registered, THEN the Onboarding_Controller SHALL generate a cryptographically secure webhook secret
2. THE Onboarding_Controller SHALL store the webhook secret in a Kubernetes Secret
3. THE Onboarding_Controller SHALL configure the EventListener to use the webhook secret
4. THE Onboarding_Controller SHALL update RepoBinding status with the webhook secret
5. WHEN a GitHub webhook is received, THEN the EventListener SHALL validate the webhook signature
6. WHEN a webhook signature is invalid, THEN the EventListener SHALL reject it with 401 Unauthorized

### Requirement 9: Observability and Monitoring

**User Story:** As a platform engineer, I want visibility into platform health, so that I can detect and resolve issues quickly.

#### Acceptance Criteria

1. THE Platform SHALL expose Tekton Dashboard for pipeline visibility
2. THE Platform SHALL expose ArgoCD UI for GitOps sync status
3. THE Platform SHALL log all pipeline executions and errors
4. WHEN a PipelineRun fails, THEN Tekton SHALL report the error in the PipelineRun status
5. WHEN an ArgoCD sync fails, THEN ArgoCD SHALL report the error in the Application status
6. THE Platform SHALL use Kubernetes events for component status

### Requirement 10: Secrets Management

**User Story:** As a platform engineer, I want secure secrets management, so that sensitive credentials are properly isolated per tenant.

#### Acceptance Criteria

1. THE Platform SHALL NOT store secrets in Git repository
2. THE Platform SHALL create tenant-specific webhook secrets during registration
3. THE Onboarding_Controller SHALL generate webhook secrets using cryptographic randomness
4. THE Platform SHALL store ArgoCD admin credentials in Kubernetes Secrets
5. THE Platform SHALL isolate tenant secrets to their respective namespaces

### Requirement 11: Disaster Recovery

**User Story:** As a platform engineer, I want to recover from cluster failure, so that I can restore the platform quickly.

#### Acceptance Criteria

1. WHEN a cluster is lost, THEN running bootstrap on a new cluster SHALL restore all platform components
2. THE Platform SHALL store all configuration in Git (no cluster-specific state except secrets)
3. THE Platform SHALL document backup procedures for ArgoCD credentials
4. WHEN bootstrap completes, THEN ArgoCD SHALL sync all platform components automatically
5. THE Platform SHALL support re-registering repositories by reapplying RepoBinding resources

### Requirement 12: Development Workflow

**User Story:** As a platform engineer, I want to test platform changes safely, so that I can validate changes before applying to production.

#### Acceptance Criteria

1. THE Platform SHALL support local Kind clusters for development
2. WHEN testing changes, THEN engineers SHALL use Git branches and test clusters
3. THE Platform SHALL document the change testing workflow
4. THE Platform SHALL support pointing ArgoCD Applications to different Git branches for testing
5. THE Platform SHALL provide verification scripts for testing platform components

### Requirement 13: Documentation and Runbooks

**User Story:** As a platform engineer, I want clear documentation, so that I can operate and troubleshoot the platform.

#### Acceptance Criteria

1. THE Platform SHALL document the bootstrap process
2. THE Platform SHALL document the ArgoCD-based upgrade workflow
3. THE Platform SHALL document common troubleshooting scenarios
4. THE Platform SHALL document the repository registration process
5. THE Platform SHALL maintain documentation in `.kiro/docs/`

### Requirement 14: Upgrade and Maintenance

**User Story:** As a platform engineer, I want to upgrade platform components safely, so that I can keep the platform current without downtime.

#### Acceptance Criteria

1. WHEN upgrading a component, THEN the engineer SHALL update the version in Git and commit
2. WHEN ArgoCD detects the change, THEN components SHALL upgrade gracefully
3. THE Platform SHALL use versioned Tekton release manifests
4. THE Platform SHALL use Helm chart versions for ArgoCD and Tekton Dashboard
5. THE Platform SHALL support rollback via Git revert and ArgoCD sync

### Requirement 15: Minimal Custom Code

**User Story:** As a platform engineer, I want minimal custom code, so that the platform is easier to maintain and understand.

#### Acceptance Criteria

1. THE Platform SHALL use a single bootstrap script for initialization
2. THE Platform SHALL use standard Kubernetes and Tekton patterns
3. THE Platform SHALL leverage ArgoCD for all GitOps operations (no custom sync logic)
4. THE Platform SHALL minimize the onboarding controller code to core logic only
5. THE Platform SHALL use Tekton Triggers instead of custom webhook handlers
6. THE Platform SHALL document any custom code with clear rationale

### Requirement 16: Ingress and Webhook Routing

**User Story:** As a developer, I want my repository webhooks to reach my tenant's EventListener, so that pipeline runs are triggered automatically.

#### Acceptance Criteria

1. THE Platform SHALL configure Ingress or LoadBalancer for EventListener endpoints
2. WHEN a RepoBinding is created, THEN the Onboarding_Controller SHALL create an Ingress rule for the tenant EventListener
3. THE Ingress SHALL route webhooks to the correct tenant EventListener based on path or hostname
4. THE Platform SHALL support both Ingress (for clusters with Ingress controller) and LoadBalancer (for cloud providers)
5. THE RepoBinding status SHALL display the full webhook URL including ingress hostname

### Requirement 17: ArgoCD Application Management

**User Story:** As a platform engineer, I want ArgoCD Applications managed declaratively, so that all platform configuration is in Git.

#### Acceptance Criteria

1. THE Platform SHALL define the platform ArgoCD Application as a YAML manifest in Git
2. THE Bootstrap_Script SHALL apply the platform Application manifest during initialization
3. THE Platform_Application SHALL use sync policies (automated sync, self-heal, prune)
4. THE Platform_Application SHALL use sync waves for ordered component deployment
5. THE Platform SHALL support multiple ArgoCD Applications for different platform layers (CRDs, controllers, catalog)
