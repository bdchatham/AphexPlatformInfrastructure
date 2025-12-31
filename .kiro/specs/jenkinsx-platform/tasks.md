# Implementation Plan: Jenkins X Platform

## Overview

This implementation plan breaks down the Jenkins X platform into discrete, incremental tasks. Each task builds on previous work and includes validation steps. The plan follows a bottom-up approach: infrastructure first, then platform components, then tenant provisioning, and finally end-to-end workflows.

## Tasks

- [x] 1. Set up repository structure and documentation
  - Create platform directory structure (bootstrap, crds, onboarding, catalog, tenancy, lighthouse)
  - Create initial documentation (architecture.md, onboarding.md, runbooks)
  - Create README with quick start guide
  - _Requirements: 1.7, 20.1_

- [x] 2. Create bootstrap prerequisites script
  - Write prerequisites.sh to verify kubectl, helm, and cluster access
  - Verify Kubernetes version compatibility (1.24+)
  - Check for required cluster features (RBAC, NetworkPolicy support)
  - _Requirements: 20.4_

- [x] 3. Create platform namespaces
  - [x] 3.1 Create pipeline-system namespace manifest
    - Define namespace with appropriate labels
    - _Requirements: 1.5_
  
  - [x] 3.2 Create pipeline-catalog namespace manifest
    - Define namespace with appropriate labels
    - _Requirements: 1.6, 13.1_
  
  - [x] 3.3 Create auth-system namespace manifest (for Dex)
    - Define namespace for OIDC provider
    - _Requirements: 4.1_

- [x] 4. Deploy self-hosted OIDC provider (Dex)
  - [x] 4.1 Create Dex ConfigMap with static users and GitHub connector
    - Configure issuer URL
    - Define static admin user with engineering group
    - Configure GitHub OAuth connector (optional)
    - _Requirements: 4.1, 4.2_
  
  - [x] 4.2 Create Dex Deployment and Service
    - Deploy Dex container
    - Expose Dex service
    - _Requirements: 4.1_
  
  - [x] 4.3 Configure Kubernetes API server for OIDC
    - Add OIDC flags to kube-apiserver (k3s or kubeadm)
    - Restart API server
    - _Requirements: 4.1_
  
  - [x] 4.4 Create RBAC for engineering group
    - Create ClusterRole for repo-onboarder
    - Create ClusterRoleBinding for engineering group
    - _Requirements: 4.2, 4.3_

- [x] 5. Install Tekton Pipelines
  - [x] 5.1 Apply Tekton Pipelines release manifest
    - Install Tekton controllers
    - Verify tekton-pipelines namespace is created
    - _Requirements: 1.2_
  
  - [x] 5.2 Wait for Tekton controllers to be ready
    - Check tekton-pipelines-controller pod status
    - Check tekton-pipelines-webhook pod status
    - _Requirements: 20.5_

- [x] 6. Install Jenkins X and Lighthouse
  - [x] 6.1 Add Jenkins X Helm repository
    - Add jx3 Helm repo
    - Update Helm repo cache
    - _Requirements: 1.1_
  
  - [x] 6.2 Install jx-build-controller
    - Deploy via Helm to pipeline-system namespace
    - _Requirements: 1.1_
  
  - [x] 6.3 Create GitHub App
    - Register GitHub App with webhook permissions
    - Generate and store private key
    - Install app at organization level
    - _Requirements: 2.1, 2.2_
  
  - [x] 6.4 Create Lighthouse configuration ConfigMap
    - Configure GitHub App ID and installation ID
    - Create initial empty allowlist
    - _Requirements: 2.2, 3.1_
  
  - [x] 6.5 Install Lighthouse via Helm
    - Deploy Lighthouse to pipeline-system namespace
    - Configure GitHub App credentials
    - _Requirements: 1.3, 2.2_
  
  - [x] 6.6 Wait for Lighthouse to be ready
    - Check Lighthouse pod status
    - Verify webhook endpoint is accessible
    - _Requirements: 20.5_

- [x] 7. Create RepoBinding CRD
  - [x] 7.1 Define RepoBinding CRD schema
    - Define spec fields (repoOrg, repoName, tenantName, permissionProfile)
    - Define status fields (phase, message, resource creation flags)
    - Add validation rules (patterns, enums)
    - _Requirements: 9.1, 9.2, 9.3, 9.4_
  
  - [x] 7.2 Apply RepoBinding CRD to cluster
    - Install CRD
    - Verify CRD is registered
    - _Requirements: 1.5_

- [ ] 8. Create tenant resource templates
  - [ ] 8.1 Create namespace template
    - Define namespace with tenant labels
    - _Requirements: 5.1, 5.2_
  
  - [ ] 8.2 Create service account template
    - Define pipeline-runner service account
    - _Requirements: 6.1_
  
  - [ ] 8.3 Create Role templates (standard and elevated profiles)
    - Define standard profile with namespace-scoped permissions
    - Define elevated profile with additional permissions
    - _Requirements: 6.2, 9.4_
  
  - [ ] 8.4 Create RoleBinding template
    - Bind service account to role
    - _Requirements: 6.3_
  
  - [ ] 8.5 Create ResourceQuota template
    - Define CPU, memory, and pod limits
    - _Requirements: 5.3, 14.2_
  
  - [ ] 8.6 Create LimitRange template
    - Define default container resource requests/limits
    - _Requirements: 5.4_
  
  - [ ] 8.7 Create NetworkPolicy template
    - Define namespace isolation rules
    - Allow DNS queries
    - Allow internet egress
    - Deny cross-namespace traffic
    - _Requirements: 5.5, 14.4_
  
  - [ ] 8.8 Create Terraform backend secret template (Kubernetes backend)
    - Define backend configuration for Kubernetes state storage
    - _Requirements: 7.1, 7.2, 7.3_

- [ ] 9. Implement onboarding controller
  - [ ] 9.1 Set up Go project structure
    - Initialize Go module
    - Add controller-runtime dependencies
    - Create main.go entry point
    - _Requirements: 4.5_
  
  - [ ] 9.2 Implement RepoBinding reconciler
    - Create reconciler struct
    - Implement Reconcile method
    - Handle RepoBinding create/update/delete events
    - _Requirements: 4.5, 9.5_
  
  - [ ] 9.3 Implement validation logic
    - Validate repository organization against approved list
    - Validate namespace name pattern
    - Reject privileged namespace names
    - Validate permission profile
    - _Requirements: 9.1, 9.2, 9.3, 9.4_
  
  - [ ] 9.4 Implement tenant namespace provisioning
    - Create namespace from template
    - Apply tenant labels
    - Update RepoBinding status
    - _Requirements: 5.1, 5.2_
  
  - [ ] 9.5 Implement service account provisioning
    - Create service account from template
    - Update RepoBinding status
    - _Requirements: 6.1_
  
  - [ ] 9.6 Implement RBAC provisioning
    - Create Role from template (based on permission profile)
    - Create RoleBinding from template
    - Update RepoBinding status
    - _Requirements: 6.2, 6.3_
  
  - [ ] 9.7 Implement resource limit provisioning
    - Create ResourceQuota from template
    - Create LimitRange from template
    - Update RepoBinding status
    - _Requirements: 5.3, 5.4_
  
  - [ ] 9.8 Implement network policy provisioning
    - Create NetworkPolicy from template
    - Update RepoBinding status
    - _Requirements: 5.5_
  
  - [ ] 9.9 Implement Terraform backend secret provisioning
    - Create backend secret from template
    - Update RepoBinding status
    - _Requirements: 7.1, 7.2, 7.3_
  
  - [ ] 9.10 Implement allowlist update
    - Read allowlist ConfigMap
    - Add repository entry with tenant mapping
    - Update ConfigMap
    - Update RepoBinding status
    - _Requirements: 8.1, 8.2, 8.3, 8.4, 8.5_
  
  - [ ] 9.11 Implement error handling and status updates
    - Update RepoBinding status on errors
    - Log errors with context
    - _Requirements: 16.5_

- [ ] 10. Create onboarding controller deployment manifests
  - [ ] 10.1 Create ServiceAccount for controller
    - Define service account in pipeline-system namespace
    - _Requirements: 4.5_
  
  - [ ] 10.2 Create ClusterRole for controller
    - Grant permissions to create namespaces, roles, rolebindings
    - Grant permissions to read/write ConfigMaps
    - Grant permissions to update RepoBinding status
    - _Requirements: 4.5_
  
  - [ ] 10.3 Create ClusterRoleBinding for controller
    - Bind controller service account to ClusterRole
    - _Requirements: 4.5_
  
  - [ ] 10.4 Create Deployment for controller
    - Define controller deployment
    - Configure resource requests/limits
    - _Requirements: 4.5_

- [ ] 11. Build and deploy onboarding controller
  - [ ] 11.1 Build controller Docker image
    - Create Dockerfile
    - Build image
    - _Requirements: 4.5_
  
  - [ ] 11.2 Push image to local registry
    - Tag image
    - Push to homelab registry
    - _Requirements: 4.5_
  
  - [ ] 11.3 Deploy controller to cluster
    - Apply controller manifests
    - Verify controller pod is running
    - _Requirements: 4.5, 20.5_

- [ ] 12. Create golden pipeline catalog
  - [ ] 12.1 Create git-clone Task
    - Define Task to clone repository at commit SHA
    - _Requirements: 10.1, 13.2_
  
  - [ ] 12.2 Create cdktf-synth Task
    - Define Task to run cdktf synth
    - _Requirements: 12.2, 13.2_
  
  - [ ] 12.3 Create cdktf-deploy Task
    - Define Task to run cdktf deploy with remote state
    - Configure Terraform backend from secret
    - _Requirements: 12.3, 12.4, 13.2_
  
  - [ ] 12.4 Create upload-artifacts Task (optional)
    - Define Task to upload logs/outputs to external storage
    - _Requirements: 12.5, 13.2_
  
  - [ ] 12.5 Create cdktf-deploy-pipeline Pipeline
    - Define Pipeline with git-clone, cdktf-synth, cdktf-deploy tasks
    - Configure workspaces
    - Configure parameters
    - _Requirements: 10.3, 13.3_
  
  - [ ] 12.6 Apply catalog resources to pipeline-catalog namespace
    - Apply all Tasks
    - Apply all Pipelines
    - Verify resources are created
    - _Requirements: 13.1_

- [ ] 13. Build runner container image
  - [ ] 13.1 Create Dockerfile for runner image
    - Base on node:20-alpine
    - Install git, terraform, cdktf-cli, kubectl
    - _Requirements: 13.4_
  
  - [ ] 13.2 Build runner image
    - Build Docker image
    - _Requirements: 13.4_
  
  - [ ] 13.3 Push runner image to local registry
    - Tag image
    - Push to homelab registry
    - _Requirements: 13.4_

- [ ] 14. Checkpoint - Verify platform bootstrap
  - Verify all platform components are running
  - Verify Dex is accessible
  - Verify Tekton controllers are healthy
  - Verify Lighthouse is healthy
  - Verify onboarding controller is healthy
  - Verify pipeline catalog is installed
  - Ask user if questions arise

- [ ] 15. Test onboarding workflow
  - [ ] 15.1 Create test RepoBinding
    - Create RepoBinding for test repository
    - _Requirements: 4.5_
  
  - [ ] 15.2 Verify tenant namespace creation
    - Check namespace exists
    - Verify labels are correct
    - _Requirements: 5.1, 5.2_
  
  - [ ] 15.3 Verify service account creation
    - Check service account exists
    - _Requirements: 6.1_
  
  - [ ] 15.4 Verify RBAC configuration
    - Check Role exists
    - Check RoleBinding exists
    - Verify permissions are scoped correctly
    - _Requirements: 6.2, 6.3, 6.4, 6.5_
  
  - [ ] 15.5 Verify resource limits
    - Check ResourceQuota exists
    - Check LimitRange exists
    - _Requirements: 5.3, 5.4_
  
  - [ ] 15.6 Verify network policy
    - Check NetworkPolicy exists
    - _Requirements: 5.5_
  
  - [ ] 15.7 Verify Terraform backend secret
    - Check secret exists
    - Verify backend configuration
    - _Requirements: 7.1, 7.2, 7.3_
  
  - [ ] 15.8 Verify allowlist update
    - Check repository is in allowlist ConfigMap
    - Verify tenant mapping
    - _Requirements: 8.1, 8.2, 8.3, 8.4, 8.5_
  
  - [ ] 15.9 Verify RepoBinding status
    - Check status is "Ready"
    - Verify all resource creation flags are true
    - _Requirements: 4.5_

- [ ] 16. Test pipeline execution workflow
  - [ ] 16.1 Create test repository with CDKTF code
    - Create repository with simple CDKTF stack
    - Add pipeline definition YAML
    - _Requirements: 10.1, 10.2, 10.3_
  
  - [ ] 16.2 Onboard test repository
    - Create RepoBinding
    - Wait for onboarding to complete
    - _Requirements: 4.5_
  
  - [ ] 16.3 Trigger pipeline via merge to main
    - Merge commit to main branch
    - Verify GitHub App delivers webhook
    - _Requirements: 11.1, 11.2_
  
  - [ ] 16.4 Verify PipelineRun creation
    - Check PipelineRun is created in tenant namespace
    - Verify PipelineRun uses tenant service account
    - _Requirements: 11.3, 11.4, 11.5_
  
  - [ ] 16.5 Verify pipeline execution
    - Check git-clone task completes
    - Check cdktf-synth task completes
    - Check cdktf-deploy task completes
    - _Requirements: 12.1, 12.2, 12.3_
  
  - [ ] 16.6 Verify Terraform state
    - Check state is stored in Kubernetes backend
    - Verify state is isolated to tenant
    - _Requirements: 12.4, 7.5_
  
  - [ ] 16.7 Verify logs are accessible
    - Check PipelineRun logs via kubectl
    - _Requirements: 16.1_

- [ ] 17. Test tenant isolation
  - [ ] 17.1 Create second tenant
    - Onboard second repository
    - Verify second tenant namespace is created
    - _Requirements: 13.1_
  
  - [ ] 17.2 Verify cross-namespace access is denied
    - Attempt to access resources from first tenant in second tenant namespace
    - Verify RBAC denies access
    - _Requirements: 14.1, 6.4_
  
  - [ ] 17.3 Verify network isolation
    - Attempt network connection from first tenant pod to second tenant pod
    - Verify NetworkPolicy blocks connection
    - _Requirements: 14.4_
  
  - [ ] 17.4 Verify resource quotas are enforced
    - Attempt to exceed ResourceQuota in tenant namespace
    - Verify quota enforcement
    - _Requirements: 14.2_

- [ ] 18. Test deployment serialization
  - [ ] 18.1 Trigger multiple concurrent deployments for same tenant
    - Merge multiple commits rapidly
    - Verify multiple PipelineRuns are created
    - _Requirements: 15.1_
  
  - [ ] 18.2 Verify deployments are serialized
    - Check only one cdktf-deploy runs at a time per tenant
    - Verify subsequent deployments queue
    - _Requirements: 15.2, 15.3, 15.4_
  
  - [ ] 18.3 Verify cross-tenant parallelism
    - Trigger deployments for different tenants
    - Verify they run in parallel
    - _Requirements: 15.5_

- [ ] 19. Checkpoint - Verify end-to-end workflows
  - Verify onboarding workflow completes successfully
  - Verify pipeline execution workflow completes successfully
  - Verify tenant isolation is enforced
  - Verify deployment serialization works correctly
  - Ask user if questions arise

- [ ] 20. Create operational documentation
  - [ ] 20.1 Write bootstrap runbook
    - Document prerequisites
    - Document bootstrap steps
    - Document verification steps
    - _Requirements: 1.7_
  
  - [ ] 20.2 Write onboarding runbook
    - Document how to create RepoBinding
    - Document how to verify onboarding
    - Document common onboarding errors
    - _Requirements: 1.7_
  
  - [ ] 20.3 Write pipeline troubleshooting runbook
    - Document how to debug pipeline failures
    - Document how to access logs
    - Document common pipeline errors
    - _Requirements: 16.2, 16.3, 16.4_
  
  - [ ] 20.4 Write Terraform state backend runbook
    - Document backend configuration
    - Document state migration
    - Document backup/restore procedures
    - _Requirements: 7.1, 7.2, 7.3_

- [ ] 21. Update .kiro/docs with platform information
  - [ ] 21.1 Update overview.md
    - Document platform purpose
    - Document key concepts
    - Document quick start
    - _Requirements: 1.7_
  
  - [ ] 21.2 Update architecture.md
    - Document component architecture
    - Document data flow
    - Document integration points
    - _Requirements: 1.7_
  
  - [ ] 21.3 Update operations.md
    - Document deployment procedures
    - Document monitoring
    - Document maintenance
    - _Requirements: 1.7_
  
  - [ ] 21.4 Update api.md
    - Document RepoBinding API
    - Document pipeline catalog API
    - Document tenant contracts
    - _Requirements: 1.7_
  
  - [ ] 21.5 Update data-models.md
    - Document RepoBinding schema
    - Document allowlist schema
    - Document pipeline parameters
    - _Requirements: 1.7_
  
  - [ ] 21.6 Update faq.md
    - Document common questions
    - Document troubleshooting tips
    - _Requirements: 1.7_

- [ ] 22. Final checkpoint - Platform ready for production
  - Verify all components are healthy
  - Verify all tests pass
  - Verify documentation is complete
  - Verify platform is ready for product teams
  - Ask user if questions arise

## Notes

- Tasks are organized in dependency order
- Each task includes validation steps
- Checkpoints ensure incremental progress
- Documentation tasks ensure platform is maintainable
- All tasks reference specific requirements for traceability

