# Implementation Plan: Kubernetes API OIDC Authentication

## Overview

This implementation plan breaks down the Kubernetes API OIDC authentication feature into discrete, incremental tasks. The plan follows a logical sequence: configure infrastructure (kube-apiserver, Dex), implement RBAC, document AphexCLI integration contracts, and validate the complete system.

## Tasks

- [ ] 1. Configure kube-apiserver OIDC Trust
  - Update cluster provisioning configuration to set kube-apiserver OIDC flags
  - Configure --oidc-issuer-url, --oidc-client-id, --oidc-username-claim, --oidc-groups-claim
  - Add --oidc-ca-file configuration for self-signed certificates (conditional)
  - Verify break-glass admin access remains functional
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 10.1, 10.4_

- [ ] 1.1 Write property test for break-glass access independence
  - **Property 6: Break-Glass Access Independence**
  - **Validates: Requirements 10.2, 10.5**

- [ ] 2. Configure Dex Kubernetes Client
  - [ ] 2.1 Generate Kubernetes client secret
    - Create secret generation script in bootstrap
    - Store secret in Kubernetes Secret (dex-kubernetes-client)
    - _Requirements: 14.2_

  - [ ] 2.2 Update Dex ConfigMap with Kubernetes client
    - Add static client with id "kubernetes"
    - Configure redirect URIs for localhost callback (8000, 18000)
    - Set client as confidential (not public)
    - _Requirements: 2.1, 2.2, 2.3, 14.1, 14.2_

  - [ ] 2.3 Write property test for token claims completeness
    - **Property 1: Token Claims Completeness**
    - **Validates: Requirements 3.1, 3.2, 3.3, 3.4, 3.5**

  - [ ] 2.4 Write property test for group claim propagation
    - **Property 2: Group Claim Propagation**
    - **Validates: Requirements 2.4, 2.5, 4.4**

- [ ] 3. Implement Platform Groups in Authentik
  - [ ] 3.1 Update Authentik Blueprints with platform groups
    - Add platform-admins group definition
    - Add platform-operators group definition
    - Add platform-engineering group definition
    - _Requirements: 4.1, 4.2, 4.3_

  - [ ] 3.2 Verify groups appear in Dex tokens
    - Test authentication flow with test users
    - Decode JWT tokens and verify groups claim
    - _Requirements: 4.4_

- [ ] 4. Implement Kubernetes RBAC
  - [ ] 4.1 Create ClusterRole for platform-admins
    - Define full CRUD access to platform CRDs
    - Grant namespace management permissions
    - Grant log and event read access
    - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5_

  - [ ] 4.2 Create ClusterRole for platform-operators
    - Define elevated platform CRD access
    - Grant log and event read access
    - Restrict namespace creation/deletion
    - _Requirements: 6.1, 6.2, 6.3, 6.4, 6.5_

  - [ ] 4.3 Create ClusterRole for platform-engineers
    - Define create/read/update access to platform CRDs
    - Grant read-only access to cluster-scoped resources
    - Prevent delete operations
    - _Requirements: 7.1, 7.2, 7.3, 7.4, 7.5_

  - [ ] 4.4 Create ClusterRoleBindings for groups
    - Bind platform-admins group to platform-admin ClusterRole
    - Bind platform-operators group to platform-operator ClusterRole
    - Bind platform-engineering group to platform-engineer ClusterRole
    - _Requirements: 8.1, 8.2, 8.3_

  - [ ] 4.5 Write property test for RBAC group authorization
    - **Property 3: RBAC Group Authorization**
    - **Validates: Requirements 8.4, 8.5**

  - [ ] 4.6 Write property test for admin full access
    - **Property 5: Admin Full Access**
    - **Validates: Requirements 22.1**

- [ ] 5. Implement Namespace Scoping for Engineers
  - [ ] 5.1 Create RoleBindings for user-* namespaces
    - Create template RoleBinding for user namespaces
    - Bind platform-engineer ClusterRole to user namespaces
    - Document namespace creation procedure
    - _Requirements: 9.1, 9.2_

  - [ ] 5.2 Create RoleBindings for team-* namespaces
    - Create template RoleBinding for team namespaces
    - Bind platform-engineer ClusterRole to team namespaces
    - _Requirements: 9.1, 9.2_

  - [ ] 5.3 Write property test for namespace scoping
    - **Property 4: Namespace Scoping for Engineers**
    - **Validates: Requirements 9.2, 9.3**

- [ ] 6. Checkpoint - Verify OIDC and RBAC Configuration
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 7. Document AphexCLI Integration Contract
  - [ ] 7.1 Document kubeconfig structure for exec plugin
    - Document "aphex" context configuration
    - Document exec plugin configuration (issuer URL, client ID)
    - Provide example kubeconfig YAML
    - _Requirements: 11.2, 11.3, 11.4, 15.1, 15.2_

  - [ ] 7.2 Document capability matrix for AphexCLI
    - Create capability matrix mapping commands to RBAC permissions
    - Document required groups for each command
    - Document namespace scoping rules
    - _Requirements: 20.1, 20.2, 20.3, 20.4, 20.5_

  - [ ] 7.3 Document expected AphexCLI behaviors
    - Document login workflow expectations
    - Document preflight check expectations
    - Document error message format expectations
    - _Requirements: 11.1, 11.5, 12.1, 12.2, 12.3, 12.4_

  - [ ] 7.4 Document authentication flow for CLI developers
    - Document exec plugin integration pattern
    - Document token refresh behavior
    - Document SelfSubjectAccessReview usage
    - _Requirements: 13.1, 13.2, 13.5_

- [ ] 8. Checkpoint - Verify OIDC, RBAC, and Documentation
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 9. Implement TLS and Certificate Management
  - [ ] 9.1 Create Ingress resource for Dex
    - Define Ingress with external hostname
    - Configure TLS with cert-manager
    - _Requirements: 16.1, 16.3, 19.1_

  - [ ] 9.2 Configure cert-manager for Dex certificates
    - Create Certificate resource for Dex
    - Choose issuer (selfsigned or letsencrypt)
    - _Requirements: 16.1, 16.4, 19.2_

  - [ ] 9.3 Update kube-apiserver CA configuration (if self-signed)
    - Extract CA certificate from Secret
    - Mount CA in apiserver pod
    - Update --oidc-ca-file flag
    - _Requirements: 16.2_

  - [ ] 9.4 Write property test for Dex external accessibility
    - **Property 13: Dex External Accessibility**
    - **Validates: Requirements 16.1, 16.4, 19.1, 19.2**

- [ ] 10. Update Bootstrap Script
  - [ ] 10.1 Add kube-apiserver OIDC configuration
    - Update Kind cluster config with OIDC flags
    - Update kubeadm config with OIDC flags
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 18.2_

  - [ ] 10.2 Add Dex client secret generation
    - Generate random client secret
    - Create Kubernetes Secret
    - _Requirements: 2.1_

  - [ ] 10.3 Verify break-glass access
    - Test certificate-based auth works
    - Document break-glass procedure
    - _Requirements: 10.1, 10.2, 10.3, 10.4_

  - [ ] 10.4 Add OIDC validation steps
    - Document how to verify OIDC discovery
    - Document how to test authentication
    - _Requirements: 21.5, 23.4_

  - [ ] 10.5 Write property test for bootstrap OIDC independence
    - **Property 14: Bootstrap OIDC Independence**
    - **Validates: Requirements 18.1**

- [ ] 11. Implement Validation and Testing Procedures
  - [ ] 11.1 Create OIDC discovery validation script
    - Test Dex discovery endpoint reachability
    - Test JWKS endpoint reachability
    - _Requirements: 23.1, 23.2_

  - [ ] 11.2 Create authentication validation script
    - Test token claims with kubectl
    - Verify token issuer and audience
    - Test kubectl with OIDC
    - _Requirements: 21.1, 21.4_

  - [ ] 11.3 Create authorization validation script
    - Test platform-admins permissions
    - Test platform-operators permissions
    - Test platform-engineering permissions with namespace scoping
    - _Requirements: 22.1, 22.2, 22.3, 22.4_

  - [ ] 11.4 Create break-glass validation script
    - Simulate Dex failure
    - Verify certificate auth works
    - Restore Dex and verify OIDC works
    - _Requirements: 25.1, 25.2, 25.3_

  - [ ] 11.5 Write property test for token issuer validation
    - **Property 9: Token Issuer Validation**
    - **Validates: Requirements 21.1, 21.2**

  - [ ] 11.6 Write property test for token audience validation
    - **Property 10: Token Audience Validation**
    - **Validates: Requirements 21.3**

  - [ ] 11.7 Write property test for username and groups extraction
    - **Property 11: Username and Groups Extraction**
    - **Validates: Requirements 21.4**

  - [ ] 11.8 Write property test for OIDC discovery reachability
    - **Property 12: OIDC Discovery Reachability**
    - **Validates: Requirements 23.1, 23.2**

- [ ] 12. Create Documentation
  - [ ] 12.1 Document OIDC configuration
    - Document issuer URL and client ID
    - Document claims contract
    - Document kube-apiserver configuration
    - _Requirements: 14.5, 26.1_

  - [ ] 12.2 Document platform groups and permissions
    - Document group definitions and purposes
    - Document ClusterRole permissions
    - Document namespace scoping policy
    - _Requirements: 4.5, 9.5, 26.2_

  - [ ] 12.3 Document AphexCLI integration contract
    - Document kubeconfig structure for exec plugin
    - Document capability matrix (command-to-permission mappings)
    - Document expected authentication flow
    - Document error handling expectations
    - _Requirements: 11.2, 11.3, 11.4, 15.1, 15.2, 19.3, 19.4, 20.5, 26.3, 26.4_

  - [ ] 12.4 Document troubleshooting procedures
    - Document common authentication issues
    - Document common authorization issues
    - Document break-glass recovery
    - Document TLS certificate issues
    - _Requirements: 23.5, 24.4, 24.5, 25.3, 25.4, 26.5_

- [ ] 13. Final Integration Testing
  - [ ] 13.1 Test complete authentication flow
    - Obtain OIDC token via Dex
    - Configure kubectl with OIDC token
    - Verify kubectl commands work
    - _Requirements: 21.1, 21.4_

  - [ ] 13.2 Test RBAC enforcement
    - Authenticate as admin, operator, engineer
    - Verify each user has correct permissions
    - Verify namespace scoping works
    - _Requirements: 22.1, 22.2, 22.3, 22.4_

  - [ ] 13.3 Test break-glass recovery
    - Stop Dex pod
    - Verify OIDC fails
    - Verify certificate auth works
    - Restart Dex
    - Verify OIDC works again
    - _Requirements: 25.1, 25.2, 25.5_

  - [ ] 13.4 Write property test for OIDC availability after Dex deployment
    - **Property 15: OIDC Availability After Dex Deployment**
    - **Validates: Requirements 18.5**

  - [ ] 13.5 Write property test for OIDC restoration without restart
    - **Property 19: OIDC Restoration Without Restart**
    - **Validates: Requirements 25.5**

  - [ ] 13.6 Write property test for operator elevated access
    - **Property 18: Operator Elevated Access**
    - **Validates: Requirements 22.2**

- [ ] 14. Final Checkpoint - Complete System Validation
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- All tasks are required for comprehensive implementation
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation
- Property tests validate universal correctness properties
- Unit tests validate specific examples and edge cases
- The implementation builds on existing Dex/Authentik infrastructure
- **AphexCLI implementation is OUT OF SCOPE** - this spec only documents the contracts that AphexCLI will consume
- RBAC manifests are managed by GitOps (ArgoCD)
- Bootstrap script handles kube-apiserver configuration
- Task 7 documents the AphexCLI integration contract (kubeconfig structure, capability matrix, authentication flow) but does not implement the CLI itself
