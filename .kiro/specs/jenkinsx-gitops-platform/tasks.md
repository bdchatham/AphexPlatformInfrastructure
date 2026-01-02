# Implementation Plan: Jenkins X Platform with Self-Service CI/CD

## Overview

This implementation plan focuses on creating a batteries-included Jenkins X platform that bootstraps with a single command and manages its own upgrades through CI/CD. The platform eliminates manual kubectl commands after bootstrap by using webhooks to trigger automated upgrades.

## Tasks

- [x] 1. Consolidate bootstrap script
  - [x] 1.1 Merge bootstrap.sh and install-jenkinsx.sh into single script
    - Combine cluster creation, Tekton installation, and JenkinsX installation
    - Add progress indicators and clear output
    - _Requirements: 2.1, 2.2, 2.3_
  
  - [x] 1.2 Add platform RepoBinding creation to bootstrap
    - Generate webhook secret for platform repository
    - Create RepoBinding for platform-infra tenant
    - Store webhook secret in Kubernetes Secret
    - _Requirements: 3.1, 3.2_
  
  - [x] 1.3 Add webhook secret display to bootstrap output
    - Display generated webhook secret
    - Display GitHub webhook configuration instructions
    - Display Lighthouse webhook URL
    - _Requirements: 2.5, 3.5_
  
  - [x] 1.4 Add platform pipeline deployment to bootstrap
    - Apply platform-upgrade-pipeline to platform-infra namespace
    - Verify pipeline is ready
    - _Requirements: 3.3_

- [x] 2. Create platform upgrade pipeline Tasks
  - [x] 2.1 Create kubectl-apply Task
    - Task to apply Kubernetes manifests from directory
    - Support for recursive directory application
    - _Requirements: 4.1_
  
  - [x] 2.2 Create helm-upgrade Task
    - Task to upgrade Helm releases
    - Support for values files and inline values
    - _Requirements: 4.3_
  
  - [x] 2.3 Create upgrade-tekton Task
    - Task to download and apply Tekton release YAML
    - Patch images from gcr.io to ghcr.io
    - _Requirements: 4.2_
  
  - [x] 2.4 Test platform upgrade Tasks
    - Create test manifests
    - Run Tasks individually
    - Verify output
    - _Requirements: 4.1, 4.2, 4.3_

- [x] 3. Create platform-upgrade-pipeline
  - [x] 3.1 Define pipeline structure
    - Define parameters (repo-url, revision)
    - Define workspaces (source)
    - Define task sequence
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5_
  
  - [x] 3.2 Add git-clone task
    - Clone platform repository at commit SHA
    - _Requirements: 4.1_
  
  - [x] 3.3 Add apply-crds task
    - Apply RepoBinding CRD
    - _Requirements: 4.1_
  
  - [x] 3.4 Add apply-namespaces task
    - Apply platform namespace manifests
    - _Requirements: 4.1_
  
  - [x] 3.5 Add upgrade-tekton task
    - Upgrade Tekton Pipelines
    - _Requirements: 4.2_
  
  - [x] 3.6 Add upgrade-lighthouse task
    - Upgrade Lighthouse via Helm
    - _Requirements: 4.3_
  
  - [x] 3.7 Add apply-onboarding-controller task
    - Apply onboarding controller manifests
    - _Requirements: 4.4_
  
  - [x] 3.8 Add apply-catalog task
    - Apply pipeline catalog Tasks and Pipelines
    - _Requirements: 4.5_

- [x] 4. Update onboarding controller for webhook secret generation
  - [x] 4.1 Implement webhook secret generation
    - Generate cryptographically secure random secret
    - Format as whsec_ prefix + base64 random bytes
    - _Requirements: 5.2, 8.1_
  
  - [x] 4.2 Store webhook secret in Kubernetes Secret
    - Create Secret in pipeline-system namespace
    - Name format: webhook-<tenant-name>
    - _Requirements: 5.2, 8.2_
  
  - [x] 4.3 Update RepoBinding status with webhook secret
    - Add webhookSecret field to status
    - Add webhookURL field to status
    - Add configuration instructions to message
    - _Requirements: 5.7, 8.3_
  
  - [x] 4.4 Update allowlist with webhook secret reference
    - Add webhookSecretRef to allowlist entry
    - Update Lighthouse allowlist ConfigMap
    - _Requirements: 5.6_

- [x] 5. Test bootstrap process
  - [x] 5.1 Test cluster creation
    - Run bootstrap with Kind
    - Verify cluster created
    - _Requirements: 2.1_
  
  - [x] 5.2 Test Tekton installation
    - Verify Tekton pods running
    - Verify Tekton version correct
    - _Requirements: 2.2_
  
  - [x] 5.3 Test Lighthouse installation
    - Verify Lighthouse pods running
    - Verify Lighthouse configuration
    - _Requirements: 2.3_
  
  - [x] 5.4 Test platform RepoBinding creation
    - Verify RepoBinding created
    - Verify webhook secret generated
    - Verify status updated
    - _Requirements: 3.1, 3.2_
  
  - [x] 5.5 Test webhook secret display
    - Verify secret displayed in output
    - Verify instructions displayed
    - _Requirements: 2.5, 3.5_
  
  - [x] 5.6 Test platform pipeline deployment
    - Verify pipeline created in platform-infra namespace
    - Verify pipeline is ready
    - _Requirements: 3.3_

- [ ] 6. Test platform self-upgrade workflow
  - [x] 6.1 Configure GitHub webhook
    - Add webhook to platform repository
    - Use webhook secret from bootstrap output
    - Point to Lighthouse URL (Note: Use /hooks endpoint, not /hook)
    - _Requirements: 3.5_
    - _Status: Webhook configured and delivering successfully (200 response)_
  
  - [x] 6.2 Trigger platform upgrade
    - Make test change to platform manifests
    - Commit and push to platform repository
    - Verify webhook delivered
    - _Requirements: 1.2_
    - _Status: Added environment variable support to bootstrap script, webhook delivered_
  
  - [ ] 6.3 Verify pipeline execution
    - Check PipelineRun created in platform-infra namespace
    - Verify all tasks complete successfully
    - _Requirements: 1.3, 1.4, 1.5_
    - _Status: Blocked by Lighthouse configAgent issue (see task 15)_
  
  - [ ] 6.4 Verify component upgrades
    - Check Tekton version updated
    - Check Lighthouse version updated
    - Check onboarding controller updated
    - Check catalog updated
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5_
    - _Status: Blocked by Lighthouse configAgent issue (see task 15)_

- [ ] 7. Test tenant registration workflow
  - [ ] 7.1 Create test RepoBinding
    - Apply RepoBinding manifest
    - _Requirements: 5.1_
  
  - [ ] 7.2 Verify tenant namespace creation
    - Check namespace exists
    - Verify labels are correct
    - _Requirements: 5.1_
  
  - [ ] 7.3 Verify webhook secret generation
    - Check webhook secret in RepoBinding status
    - Verify secret stored in Kubernetes
    - Verify secret format (whsec_ prefix)
    - _Requirements: 5.2, 8.1_
  
  - [ ] 7.4 Verify service account creation
    - Check service account exists
    - _Requirements: 5.3_
  
  - [ ] 7.5 Verify RBAC configuration
    - Check Role exists
    - Check RoleBinding exists
    - Verify permissions are scoped correctly
    - _Requirements: 5.3, 6.2_
  
  - [ ] 7.6 Verify resource limits
    - Check ResourceQuota exists
    - Check LimitRange exists
    - _Requirements: 5.4_
  
  - [ ] 7.7 Verify network policy
    - Check NetworkPolicy exists
    - _Requirements: 5.5_
  
  - [ ] 7.8 Verify Terraform backend secret
    - Check secret exists
    - Verify backend configuration
    - _Requirements: 7.5_
  
  - [ ] 7.9 Verify allowlist update
    - Check repository is in allowlist ConfigMap
    - Verify webhook secret reference
    - _Requirements: 5.6_
  
  - [ ] 7.10 Verify RepoBinding status
    - Check status shows webhook secret
    - Check status shows webhook URL
    - Check status shows configuration instructions
    - _Requirements: 5.7_

- [ ] 8. Test webhook and pipeline workflow
  - [ ] 8.1 Configure GitHub webhook for tenant
    - Create webhook in test repository
    - Use webhook secret from RepoBinding status
    - Point to Lighthouse URL
    - _Requirements: 8.3_
  
  - [ ] 8.2 Trigger webhook
    - Push commit to test repository
    - Verify webhook delivered
    - _Requirements: 7.1_
  
  - [ ] 8.3 Verify PipelineRun creation
    - Check PipelineRun created in tenant namespace
    - Verify PipelineRun uses tenant service account
    - _Requirements: 7.1_
  
  - [ ] 8.4 Verify pipeline execution
    - Check git-clone task completes
    - Check cdktf-synth task completes
    - Check cdktf-deploy task completes
    - _Requirements: 7.2, 7.3, 7.4_
  
  - [ ] 8.5 Verify Terraform state
    - Check state stored in Kubernetes backend
    - Verify state isolated to tenant
    - _Requirements: 7.5_

- [ ] 9. Test tenant isolation
  - [ ] 9.1 Create second tenant
    - Register second repository
    - Verify second tenant namespace created
    - _Requirements: 6.1_
  
  - [ ] 9.2 Test RBAC isolation
    - Attempt to access resources from first tenant in second tenant namespace
    - Verify RBAC denies access
    - _Requirements: 6.2_
  
  - [ ] 9.3 Test network isolation
    - Attempt network connection from first tenant pod to second tenant pod
    - Verify NetworkPolicy blocks connection
    - _Requirements: 6.3_
  
  - [ ] 9.4 Test resource quota enforcement
    - Attempt to exceed ResourceQuota in tenant namespace
    - Verify quota enforcement
    - _Requirements: 6.4_

- [ ] 10. Test webhook security
  - [ ] 10.1 Test invalid webhook signature
    - Send webhook with invalid signature
    - Verify Lighthouse rejects it
    - _Requirements: 8.4_
  
  - [ ] 10.2 Test disallowed repository
    - Send webhook for repository not in allowlist
    - Verify Lighthouse rejects it
    - _Requirements: 8.5_
  
  - [ ] 10.3 Test webhook secret isolation
    - Attempt to access webhook secret from tenant
    - Verify RBAC denies access
    - _Requirements: 10.5_

- [ ] 11. Test disaster recovery
  - [ ] 11.1 Destroy cluster
    - Delete Kind cluster
    - _Requirements: 11.1_
  
  - [ ] 11.2 Bootstrap new cluster
    - Run bootstrap script
    - Verify all components install
    - _Requirements: 11.1, 11.4_
  
  - [ ] 11.3 Verify platform restored
    - Check all platform components running
    - Verify platform RepoBinding exists
    - Verify webhook secret generated
    - _Requirements: 11.1, 11.4_
  
  - [ ] 11.4 Re-register tenants
    - Reapply RepoBinding resources
    - Verify tenants provisioned
    - _Requirements: 11.5_

- [ ] 12. Test upgrade and rollback
  - [ ] 12.1 Test component version upgrade
    - Update Tekton version in platform repo
    - Commit and push
    - Verify platform upgrade pipeline runs
    - Verify Tekton upgraded
    - _Requirements: 14.1, 14.2_
  
  - [ ] 12.2 Test rollback via Git revert
    - Revert version change commit
    - Verify platform upgrade pipeline runs
    - Verify Tekton rolled back
    - _Requirements: 14.5_

- [ ] 13. Documentation
  - [ ] 13.1 Update .kiro/docs/overview.md
    - Document platform purpose
    - Document self-upgrade approach
    - Document quick start
    - _Requirements: 13.1_
  
  - [ ] 13.2 Update .kiro/docs/architecture.md
    - Document platform architecture
    - Document component relationships
    - Document data flow
    - _Requirements: 13.1_
  
  - [ ] 13.3 Update .kiro/docs/operations.md
    - Document bootstrap procedure
    - Document platform self-upgrade workflow
    - Document registration workflow
    - Document webhook configuration
    - Document troubleshooting
    - _Requirements: 13.2, 13.3_
  
  - [ ] 13.4 Update .kiro/docs/api.md
    - Document RepoBinding API
    - Document webhook API
    - _Requirements: 13.1_
  
  - [ ] 13.5 Update .kiro/docs/data-models.md
    - Document RepoBinding schema
    - Document allowlist schema
    - Document pipeline parameters
    - _Requirements: 13.1_
  
  - [ ] 13.6 Update .kiro/docs/faq.md
    - Add common questions about bootstrap
    - Add common questions about self-upgrade
    - Add common questions about webhook configuration
    - _Requirements: 13.3_

- [ ] 14. Cleanup and polish
  - [ ] 14.1 Remove ArgoCD references
    - Delete argocd/ directory
    - Remove ArgoCD-related manifests
    - _Requirements: 15.2_
  
  - [ ] 14.2 Update README.md
    - Document single-command bootstrap
    - Document self-upgrade workflow
    - Document webhook configuration
    - _Requirements: 13.1_
  
  - [ ] 14.3 Add verification scripts
    - Script to verify bootstrap completed successfully
    - Script to verify platform components healthy
    - Script to verify tenant provisioning
    - _Requirements: 12.5_
  
  - [ ] 14.4 Polish bootstrap script output
    - Add color-coded output
    - Add progress indicators
    - Add clear success/failure messages
    - _Requirements: 2.5_

- [-] 15. Fix Lighthouse configAgent for pipeline triggering
  - [x] 15.1 Investigate Lighthouse configAgent configuration
    - Research how Lighthouse reads .lighthouse/jenkins-x/ trigger configs from repositories
    - Determine what configAgent settings are needed
    - Review Lighthouse Helm chart values for configAgent options
    - _Context: Webhooks are being delivered successfully (200 response) but PipelineRuns are not being created_
    - _Context: Lighthouse logs show "no configAgent configuration" warning_
    - _Context: Repository has .lighthouse/jenkins-x/triggers.yaml and release.yaml configured_
  
  - [x] 15.2 Configure Lighthouse configAgent
    - Update Lighthouse Helm values or ConfigMap with configAgent settings
    - Configure how Lighthouse fetches repository configuration
    - Ensure Lighthouse can read .lighthouse directories from GitHub
    - _Requirements: 1.2, 1.3_
  
  - [x] 15.3 Test webhook to PipelineRun flow
    - Push a test commit to trigger webhook
    - Verify Lighthouse creates PipelineRun in tenant-platform-infra namespace
    - Check Lighthouse logs for successful trigger processing
    - _Requirements: 1.2, 1.3_
  
  - [ ] 15.4 Verify platform upgrade pipeline execution
    - Confirm PipelineRun executes all tasks (git-clone, apply-crds, upgrade-tekton, etc.)
    - Verify platform components are updated based on commit changes
    - Check that changes from Git are applied to the cluster
    - _Requirements: 1.3, 1.4, 1.5, 4.1, 4.2, 4.3, 4.4, 4.5_

## Notes

- Tasks are organized in dependency order
- Bootstrap is the only manual script - everything else is automated via webhooks
- Platform manages its own upgrades through CI/CD
- Testing validates both bootstrap and self-upgrade workflows
- Documentation ensures platform is maintainable and understandable
- Cleanup removes all ArgoCD references from previous design
- Task 15 addresses the Lighthouse configAgent issue discovered during task 6 testing
