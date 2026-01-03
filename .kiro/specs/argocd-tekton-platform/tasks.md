# Implementation Plan: ArgoCD + Tekton GitOps Platform

## Overview

This implementation plan breaks down the ArgoCD + Tekton platform into discrete coding tasks. The approach follows a layered implementation:

1. **Manual Jenkins X file cleanup** - Remove legacy Jenkins X files from the repository
2. **Bootstrap script** - Manual one-time operation to create Kind cluster and install core components (Tekton, ArgoCD)
3. **Platform Applications (App of Apps)** - Root ArgoCD Application that manages child Applications for each component layer
4. **Platform manifests** - GitOps-managed platform configuration organized by layer:
   - CRDs (RepoBinding)
   - Infrastructure (namespaces, RBAC)
   - Controllers (Onboarding controller)
   - Catalog (Tekton tasks, pipelines, triggers)
5. **Tenant templates** - Resource templates for tenant provisioning

After bootstrap completes, ArgoCD takes over and manages all platform components via GitOps using the App of Apps pattern. The bootstrap script is a manual operation an engineer performs before the pipeline begins managing itself.

**App of Apps Architecture**: The platform uses a root Application (`platform-root`) that manages four child Applications:
- `platform-crds`: CRDs and foundational resources
- `platform-infrastructure`: Namespaces and RBAC
- `platform-controllers`: Onboarding controller
- `platform-catalog`: Tekton tasks, pipelines, and triggers

This provides better separation of concerns, independent lifecycle management, and clearer troubleshooting.

## Tasks

- [x] 1. Manual JenkinsX file cleanup (remove legacy files from repository)
  - [x] 1.1 Remove Lighthouse configuration files
    - Delete .lighthouse/ directory and all contents
    - Delete any lighthouse-related configuration files
    - _Requirements: 1.2_
  
  - [x] 1.2 Remove Jenkins X bootstrap scripts
    - Delete platform/bootstrap/install-jenkinsx.sh
    - Delete platform/bootstrap/expose-lighthouse.sh
    - Delete platform/bootstrap/github-app-setup.md (if Jenkins X specific)
    - _Requirements: 1.2_
  
  - [x] 1.3 Remove Jenkins X infrastructure manifests
    - Review and remove Jenkins X-specific files from platform/infrastructure/
    - Preserve any files that will be reused for ArgoCD/Tekton
    - _Requirements: 1.4_
  
  - [x] 1.4 Remove old Jenkins X spec folder
    - Delete .kiro/specs/jenkinsx-gitops-platform/ directory and all contents
    - _Requirements: 1.5_
  
  - [x] 1.5 Clean up documentation references
    - Remove or update any Jenkins X references in README.md
    - Remove or update any Jenkins X references in platform documentation
    - _Requirements: 1.5_

- [-] 2. Create bootstrap script (manual operation before GitOps takes over)
  - [x] 2.1 Implement Kind cluster creation
    - Create Kind cluster with specified name
    - Parse command-line options (--cluster-name, --repo-url)
    - Configure Kind with appropriate settings for homelab
    - Verify cluster accessibility via kubectl
    - _Requirements: 3.1_
  
  - [x] 2.2 Write property test for cluster creation
    - **Property 11: Cluster Creation**
    - **Validates: Requirements 3.1**

- [x] 3. Implement Tekton and ArgoCD installation in bootstrap script
  - [x] 3.1 Implement Tekton Pipelines installation
    - Download Tekton Pipelines release manifest (v0.56.0)
    - Apply manifest via kubectl
    - Wait for tekton-pipelines-controller and webhook to be ready
    - _Requirements: 3.2_
  
  - [x] 3.2 Implement Tekton Triggers installation
    - Download Tekton Triggers release manifest (v0.25.0)
    - Apply manifest via kubectl
    - Wait for tekton-triggers-controller and webhook to be ready
    - _Requirements: 3.2_
  
  - [x] 3.3 Write property test for Tekton installation
    - **Property 12: Tekton Installation**
    - **Validates: Requirements 3.2**
  
  - [x] 3.4 Implement ArgoCD installation
    - Download ArgoCD release manifest (stable)
    - Apply manifest via kubectl
    - Patch argocd-cmd-params-cm for insecure mode (homelab)
    - Wait for argocd-server, argocd-controller, argocd-repo-server to be ready
    - Retrieve admin password from argocd-initial-admin-secret
    - _Requirements: 3.3_
  
  - [x] 3.5 Write property test for ArgoCD installation
    - **Property 13: ArgoCD Installation**
    - **Validates: Requirements 3.3**
  
  - [x] 3.6 Implement platform namespace creation
    - Create argocd namespace (if not exists)
    - Create tekton-pipelines namespace (if not exists)
    - Create platform-system namespace
    - _Requirements: 3.4_
  
  - [x] 3.7 Write property test for namespace creation
    - **Property 14: Platform Namespace Creation**
    - **Validates: Requirements 3.4**

- [x] 4. Checkpoint - Verify bootstrap core components
  - Ensure all tests pass, ask the user if questions arise.

- [x] 5. Create platform ArgoCD Applications (App of Apps pattern)
  - [x] 5.1 Create root Application manifest
    - Create platform/argocd/apps/platform-root.yaml
    - Point to platform/argocd/apps directory
    - Configure automated sync policy (prune, selfHeal)
    - Configure retry policy with exponential backoff
    - _Requirements: 2.1, 3.5, 4.2_
  
  - [x] 5.2 Create CRDs Application manifest
    - Create platform/argocd/apps/platform-crds.yaml
    - Point to platform/crds directory
    - Configure automated sync policy
    - _Requirements: 2.1, 3.5_
  
  - [x] 5.3 Create Infrastructure Application manifest
    - Create platform/argocd/apps/platform-infrastructure.yaml
    - Point to platform/infrastructure directory
    - Configure automated sync policy
    - _Requirements: 2.1, 3.5_
  
  - [x] 5.4 Create Controllers Application manifest
    - Create platform/argocd/apps/platform-controllers.yaml
    - Point to platform/onboarding directory
    - Configure automated sync policy
    - _Requirements: 2.1, 3.5_
  
  - [x] 5.5 Create Catalog Application manifest
    - Create platform/argocd/apps/platform-catalog.yaml
    - Point to platform/catalog directory
    - Configure automated sync policy
    - _Requirements: 2.1, 3.5_
  
  - [x] 5.6 Write property tests for Application configuration
    - **Property 6: Platform Application Existence**
    - **Property 10: Automated Sync Policy**
    - **Validates: Requirements 2.1, 2.6, 3.5, 18.3**
  
  - [x] 5.7 Implement root Application creation in bootstrap script
    - Apply platform-root.yaml via kubectl
    - Wait for root Application to sync
    - Wait for child Applications to be created
    - Display ArgoCD UI access information
    - Display admin password
    - _Requirements: 3.5, 3.6_
  
  - [x] 5.8 Write property test for ArgoCD auto-sync
    - **Property 39: ArgoCD Auto-Sync After Bootstrap**
    - **Validates: Requirements 12.4**

- [x] 6. Create RepoBinding CRD
  - [x] 6.1 Define RepoBinding CRD YAML
    - Create platform/crds/repobinding-crd.yaml
    - Define spec fields (repoOrg, repoName, tenantName, permissionProfile, ingressHost)
    - Define status fields (phase, message, webhookURL, webhookSecret, resource creation flags)
    - Add sync-wave annotation (wave 0 for CRDs)
    - _Requirements: 5.3_
  
  - [x] 6.2 Create example RepoBinding
    - Create platform/crds/example-repobinding.yaml
    - Document usage and fields
    - _Requirements: 5.3_

- [x] 7. Create tenant resource templates
  - [x] 7.1 Create namespace template
    - Create platform/tenancy/templates/namespace-template.yaml
    - Add labels for tenant identification
    - _Requirements: 6.1_
  
  - [x] 7.2 Create service account template
    - Create platform/tenancy/templates/serviceaccount-template.yaml
    - Define pipeline-runner service account
    - _Requirements: 6.3_
  
  - [x] 7.3 Create RBAC templates
    - Create platform/tenancy/templates/role-standard.yaml (standard permissions)
    - Create platform/tenancy/templates/role-elevated.yaml (elevated permissions)
    - Create platform/tenancy/templates/rolebinding-template.yaml
    - _Requirements: 6.3_
  
  - [x] 7.4 Create ResourceQuota and LimitRange templates
    - Create platform/tenancy/templates/resourcequota-template.yaml
    - Create platform/tenancy/templates/limitrange-template.yaml
    - Define reasonable limits for homelab (CPU, memory, pods)
    - _Requirements: 6.4_
  
  - [x] 7.5 Create NetworkPolicy template
    - Create platform/tenancy/templates/networkpolicy-template.yaml
    - Deny ingress from other tenant namespaces
    - Allow ingress from ingress controller
    - _Requirements: 6.5_
  
  - [x] 7.6 Create EventListener template
    - Create platform/tenancy/templates/eventlistener-template.yaml
    - Configure GitHub interceptor with secret reference
    - Configure CEL filter for main branch pushes
    - Reference TriggerBinding and TriggerTemplate
    - _Requirements: 6.6_
  
  - [x] 7.7 Create Ingress template
    - Create platform/tenancy/templates/ingress-template.yaml
    - Route webhooks to EventListener service
    - Support path-based routing (/tenant-name)
    - _Requirements: 6.7_
  
  - [x] 7.8 Create Terraform backend secret template
    - Create platform/tenancy/templates/terraform-secret-template.yaml
    - Configure Kubernetes backend for Terraform state
    - _Requirements: 6.4_

- [x] 8. Checkpoint - Verify templates and CRDs
  - Ensure all tests pass, ask the user if questions arise.

- [x] 8. Implement Onboarding Controller
  - [x] 8.1 Set up Go controller project structure
    - Initialize Go module in platform/onboarding/controller/
    - Add dependencies (controller-runtime, client-go)
    - Create main.go entry point
    - _Requirements: 5.1_
  
  - [x] 8.2 Implement RepoBinding controller reconciliation logic
    - Watch RepoBinding resources
    - Implement Reconcile function
    - Handle create, update, delete events
    - _Requirements: 6.1_
  
  - [x] 8.3 Implement RepoBinding validation
    - Validate repoOrg is not empty
    - Validate repoName is not empty
    - Validate tenantName matches Kubernetes naming rules
    - Validate permissionProfile is "standard" or "elevated"
    - Update status with validation errors
    - _Requirements: 6.1_
  
  - [x] 8.4 Write unit tests for RepoBinding validation
    - Test valid RepoBindings are accepted
    - Test invalid RepoBindings are rejected with clear errors
    - _Requirements: 6.1_
  
  - [x] 8.5 Implement webhook secret generation
    - Generate cryptographically secure random bytes (32 bytes)
    - Format as "whsec_" + base64(random bytes)
    - Store in Kubernetes Secret (webhook-<tenant-name>)
    - _Requirements: 6.2, 9.1, 9.2_
  
  - [x] 8.6 Write property test for webhook secret generation
    - **Property 16: Webhook Secret Generation**
    - **Validates: Requirements 6.2, 9.1, 9.2**
  
  - [x] 8.7 Implement tenant resource provisioning
    - Render namespace template with tenant name
    - Render service account template
    - Render RBAC templates based on permission profile
    - Render ResourceQuota and LimitRange templates
    - Render NetworkPolicy template
    - Render Terraform secret template
    - Apply all resources via Kubernetes client
    - _Requirements: 6.1, 6.3, 6.4, 6.5_
  
  - [x] 8.8 Write property tests for tenant provisioning
    - **Property 15: Tenant Namespace Provisioning**
    - **Property 17: Service Account and RBAC Creation**
    - **Property 18: Resource Quota Creation**
    - **Property 19: Network Policy Creation**
    - **Validates: Requirements 6.1, 6.3, 6.4, 6.5**
  
  - [x] 8.9 Implement EventListener provisioning
    - Render EventListener template with tenant name and webhook secret reference
    - Apply EventListener via Kubernetes client
    - Wait for EventListener service to be created
    - _Requirements: 6.6_
  
  - [x] 8.10 Write property test for EventListener creation
    - **Property 20: EventListener Creation**
    - **Validates: Requirements 6.6**
  
  - [x] 8.11 Implement Ingress provisioning
    - Render Ingress template with tenant name and EventListener service
    - Determine ingress hostname (from RepoBinding spec or default)
    - Apply Ingress via Kubernetes client
    - _Requirements: 6.7_
  
  - [x] 8.12 Write property test for Ingress creation
    - **Property 21: Ingress Creation**
    - **Validates: Requirements 6.7**
  
  - [x] 8.13 Implement RepoBinding status updates
    - Set phase to "Provisioning" at start
    - Update resource creation flags as resources are created
    - Set webhookURL from Ingress hostname and path
    - Set webhookSecret from generated secret
    - Set phase to "Ready" on success
    - Set phase to "Failed" with error message on failure
    - _Requirements: 6.8_
  
  - [x] 8.14 Write property test for RepoBinding status
    - **Property 22: RepoBinding Status Completeness**
    - **Validates: Requirements 6.8**
  
  - [x] 8.15 Implement error handling and rollback
    - Rollback partial provisioning on failure
    - Log errors with context
    - Retry transient errors with exponential backoff
    - _Requirements: 6.1_

- [x] 9. Create Onboarding Controller deployment manifests
  - [x] 9.1 Create controller Dockerfile
    - Create platform/onboarding/controller/Dockerfile
    - Multi-stage build (build + runtime)
    - Use distroless base image for security
    - _Requirements: 5.1_
  
  - [x] 9.2 Create controller deployment YAML
    - Create platform/onboarding/controller-deployment.yaml
    - Configure resource limits
    - Configure health checks
    - Add sync-wave annotation (wave 2 for controllers)
    - _Requirements: 5.1_
  
  - [x] 9.3 Create controller RBAC
    - Create platform/onboarding/controller-rbac.yaml
    - Grant permissions to watch RepoBindings
    - Grant permissions to create namespaces, service accounts, RBAC, quotas, network policies, EventListeners, Ingresses, Secrets
    - Add sync-wave annotation (wave 1 for RBAC)
    - _Requirements: 5.1_
  
  - [x] 9.4 Create controller service account
    - Create platform/onboarding/controller-service-account.yaml
    - Add sync-wave annotation (wave 1)
    - _Requirements: 5.1_

- [x] 10. Checkpoint - Verify controller implementation
  - Ensure all tests pass, ask the user if questions arise.

- [x] 11. Create Pipeline Catalog
  - [x] 11.1 Create git-clone Task
    - Create platform/catalog/tasks/git-clone.yaml
    - Define parameters (url, revision)
    - Define workspace (output)
    - Use git image to clone repository
    - Add sync-wave annotation (wave 3 for catalog)
    - _Requirements: 7.3, 8.3_
  
  - [x] 11.2 Create cdktf-synth Task
    - Create platform/catalog/tasks/cdktf-synth.yaml
    - Define workspace (source)
    - Install Node.js and cdktf dependencies
    - Run cdktf synth
    - Add sync-wave annotation (wave 3)
    - _Requirements: 7.4, 8.4_
  
  - [x] 11.3 Create cdktf-deploy Task
    - Create platform/catalog/tasks/cdktf-deploy.yaml
    - Define workspace (source)
    - Configure Terraform Kubernetes backend
    - Run cdktf deploy with auto-approve
    - Add sync-wave annotation (wave 3)
    - _Requirements: 7.5, 8.5, 8.6_
  
  - [x] 11.4 Create cdktf-deploy Pipeline
    - Create platform/catalog/pipelines/cdktf-deploy-pipeline.yaml
    - Chain tasks: git-clone → cdktf-synth → cdktf-deploy
    - Define parameters (repo-url, revision)
    - Define workspace (source)
    - Add sync-wave annotation (wave 3)
    - _Requirements: 7.1, 7.2, 7.3, 7.4, 7.5_
  
  - [x] 11.5 Create TriggerBinding for GitHub push events
    - Create platform/catalog/triggers/github-push-binding.yaml
    - Extract repo URL from webhook payload
    - Extract commit SHA from webhook payload
    - Add sync-wave annotation (wave 3)
    - _Requirements: 8.1, 8.2_
  
  - [x] 11.6 Create TriggerTemplate for CDKTF pipeline
    - Create platform/catalog/triggers/cdktf-deploy-trigger-template.yaml
    - Create PipelineRun from cdktf-deploy-pipeline
    - Pass parameters from TriggerBinding
    - Add sync-wave annotation (wave 3)
    - _Requirements: 8.2_

- [x] 12. Implement tenant isolation tests
  - [x] 12.1 Write property test for RBAC isolation
    - **Property 23: Cross-Tenant RBAC Isolation**
    - **Validates: Requirements 7.2**
  
  - [x] 12.2 Write property test for network isolation
    - **Property 24: Cross-Tenant Network Isolation**
    - **Validates: Requirements 7.3**
  
  - [x] 12.3 Write property test for resource quota enforcement
    - **Property 25: Resource Quota Enforcement**
    - **Validates: Requirements 7.4**

- [x] 13. Implement webhook and pipeline tests
  - [x] 13.1 Write property test for webhook signature validation
    - **Property 26: Webhook Signature Validation**
    - **Validates: Requirements 8.1, 9.5, 9.6**
  
  - [x] 13.2 Write property test for webhook to PipelineRun
    - **Property 27: Webhook to PipelineRun Creation**
    - **Validates: Requirements 8.2**
  
  - [x] 13.3 Write property test for git clone at commit SHA
    - **Property 28: Git Clone at Commit SHA**
    - **Validates: Requirements 8.3**
  
  - [x] 13.4 Write property test for CDKTF synth
    - **Property 29: CDKTF Synth Execution**
    - **Validates: Requirements 8.4**
  
  - [x] 13.5 Write property test for CDKTF deploy
    - **Property 30: CDKTF Deploy Execution**
    - **Validates: Requirements 8.5**
  
  - [x] 13.6 Write property test for Terraform state persistence
    - **Property 31: Terraform State Persistence**
    - **Validates: Requirements 8.6**

- [x] 14. Implement ArgoCD GitOps tests
  - [x] 14.1 Write property test for GitOps sync on commit
    - **Property 7: GitOps Sync on Commit**
    - **Validates: Requirements 2.2, 2.6**
  
  - [x] 14.2 Write property test for platform manifest management
    - **Property 8: Platform Manifest Management**
    - **Validates: Requirements 2.3, 4.4, 5.1, 5.2, 5.3, 5.4**
  
  - [x] 14.3 Write property test for Git as source of truth
    - **Property 9: Git as Source of Truth**
    - **Validates: Requirements 2.5**
  
  - [x] 14.4 Write property test for sync wave ordering
    - **Property 44: Sync Wave Ordering**
    - **Validates: Requirements 18.4**

- [x] 15. Implement observability and monitoring tests
  - [x] 15.1 Write property test for pipeline execution logging
    - **Property 32: Pipeline Execution Logging**
    - **Validates: Requirements 10.3**
  
  - [x] 15.2 Write property test for PipelineRun failure reporting
    - **Property 33: PipelineRun Failure Reporting**
    - **Validates: Requirements 10.4**
  
  - [x] 15.3 Write property test for ArgoCD sync failure reporting
    - **Property 34: ArgoCD Sync Failure Reporting**
    - **Validates: Requirements 10.5**
  
  - [x] 15.4 Write property test for Kubernetes event generation
    - **Property 35: Kubernetes Event Generation**
    - **Validates: Requirements 10.6**

- [ ] 16. Implement security and secrets tests
  - [x] 16.1 Write property test for no secrets in Git
    - **Property 36: No Secrets in Git**
    - **Validates: Requirements 11.1**
  
  - [x] 16.2 Write property test for tenant secret isolation
    - **Property 37: Tenant Secret Isolation**
    - **Validates: Requirements 11.5**

- [x] 17. Implement disaster recovery and upgrade tests
  - [x] 17.1 Write property test for disaster recovery via bootstrap
    - **Property 38: Disaster Recovery via Bootstrap**
    - **Validates: Requirements 12.1**
  
  - [x] 17.2 Write property test for RepoBinding idempotency
    - **Property 40: RepoBinding Idempotency**
    - **Validates: Requirements 12.5**
  
  - [x] 17.3 Write property test for graceful component upgrades
    - **Property 41: Graceful Component Upgrades**
    - **Validates: Requirements 15.2**
  
  - [x] 17.4 Write property test for rollback via Git revert
    - **Property 42: Rollback via Git Revert**
    - **Validates: Requirements 15.5**

- [x] 18. Implement ingress routing tests
  - [x] 18.1 Write property test for ingress webhook routing
    - **Property 43: Ingress Webhook Routing**
    - **Validates: Requirements 17.3**

- [x] 19. Create documentation
  - [x] 19.1 Update .kiro/docs/overview.md
    - Document platform purpose and capabilities
    - Document ArgoCD + Tekton architecture
    - Document migration from JenkinsX
    - _Requirements: 14.1, 14.6_
  
  - [x] 19.2 Update .kiro/docs/architecture.md
    - Document component architecture
    - Document data flow (Git → ArgoCD → Kubernetes)
    - Document webhook flow (GitHub → EventListener → PipelineRun)
    - Add architecture diagrams
    - _Requirements: 14.1_
  
  - [x] 19.3 Update .kiro/docs/operations.md
    - Document bootstrap process
    - Document repository registration process
    - Document ArgoCD-based upgrade workflow
    - Document common troubleshooting scenarios
    - Document disaster recovery procedures
    - _Requirements: 14.1, 14.2, 14.3, 14.4_
  
  - [x] 19.4 Update .kiro/docs/api.md
    - Document RepoBinding CRD API
    - Document EventListener webhook API
    - Document ArgoCD Application API
    - _Requirements: 14.4_
  
  - [x] 19.5 Update .kiro/docs/data-models.md
    - Document RepoBinding spec and status
    - Document ArgoCD Application spec
    - Document EventListener configuration
    - _Requirements: 14.4_
  
  - [x] 19.6 Update .kiro/docs/faq.md
    - Document common issues and solutions
    - Document JenkinsX migration questions
    - Document webhook troubleshooting
    - Document ArgoCD sync issues
    - _Requirements: 14.3_

- [x] 20. Create verification and testing scripts
  - [x] 20.1 Create bootstrap verification script
    - Create platform/bootstrap/verify-bootstrap.sh
    - Check all components are running
    - Check ArgoCD Application is syncing
    - Check platform namespaces exist
    - _Requirements: 13.5_
  
  - [x] 20.2 Create tenant verification script
    - Create platform/onboarding/verify-tenant.sh
    - Check tenant namespace exists
    - Check all tenant resources exist
    - Check EventListener is ready
    - Check Ingress is configured
    - _Requirements: 13.5_
  
  - [x] 20.3 Create webhook testing script
    - Create platform/onboarding/test-webhook.sh
    - Send test webhook to EventListener
    - Verify PipelineRun is created
    - Verify webhook signature validation
    - _Requirements: 13.5_

- [x] 21. Final checkpoint - End-to-end validation
  - [x] 21.1 Verify no JenkinsX leftovers in repository
    - Scan repository for JenkinsX-related files and references
    - Verify all files relate to ArgoCD + Tekton architecture
    - Check for lighthouse, jx-git-operator, or other JenkinsX artifacts
    - Verify documentation has no JenkinsX references
    - _Requirements: 1.2, 1.5_
  
  - [x] 21.2 Run end-to-end validation
    - Ensure all tests pass, ask the user if questions arise.

## Notes

- All tasks are required for comprehensive implementation
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests validate universal correctness properties
- Unit tests validate specific examples and edge cases
- The implementation follows a layered approach: bootstrap → manifests → controller → catalog → tests → documentation
