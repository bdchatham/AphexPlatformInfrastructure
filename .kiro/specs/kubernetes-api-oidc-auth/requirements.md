# Requirements Document

## Introduction

This specification defines OIDC-based authentication for the Kubernetes API server, enabling developers to authenticate using Dex-issued tokens backed by Authentik as the identity provider. The system implements group-based Kubernetes RBAC to govern access to platform CRDs and supporting resources. The AphexCLI will provide a seamless developer experience by abstracting the permissions model, allowing users to authenticate with `aphex login` and create platform resources with commands like `aphex pipeline create`. This specification focuses on the infrastructure work required to support the AphexCLI contract, while the CLI implementation itself is out of scope.

## Glossary

- **Authentik**: Identity Provider (IdP) that manages users, groups, and authentication
- **Dex**: OIDC issuer that acts as a stable authentication proxy between Authentik and platform consumers
- **OIDC**: OpenID Connect, an authentication protocol built on OAuth 2.0
- **kube-apiserver**: Kubernetes API server that validates JWT tokens and enforces RBAC
- **RBAC**: Role-Based Access Control, Kubernetes authorization mechanism
- **AphexCLI**: Platform CLI tool that abstracts permissions and provides developer-friendly commands
- **Platform_CRD**: Custom Resource Definitions for platform resources (pipelines, workspaces, etc.)
- **JWT**: JSON Web Token, used for authentication
- **Claims**: Data embedded in JWT tokens (iss, aud, groups, username)
- **Exec_Plugin**: Kubernetes credential plugin mechanism (e.g., kubelogin) for interactive authentication
- **Break_Glass_Access**: Emergency admin access path that works even if OIDC is misconfigured
- **Kubeconfig**: Kubernetes client configuration file containing cluster and authentication details
- **Platform_Groups**: Authentik groups that map to Kubernetes RBAC roles (platform-admins, platform-operators, platform-engineering)
- **Namespace_Scoping**: Pattern that restricts resource creation to specific namespaces based on group membership
- **Preflight_Check**: Authorization validation performed before executing commands to provide early feedback

## Requirements

### Requirement 1: Kubernetes API Server OIDC Trust Configuration

**User Story:** As a platform operator, I want the Kubernetes API server to trust Dex-issued OIDC tokens, so that developers can authenticate to the cluster using their platform credentials.

#### Acceptance Criteria

1. WHEN the cluster is provisioned, THE kube-apiserver SHALL be configured with OIDC issuer URL pointing to Dex
2. THE kube-apiserver SHALL be configured with OIDC client ID matching the Dex Kubernetes client
3. THE kube-apiserver SHALL be configured to extract username from the email claim
4. THE kube-apiserver SHALL be configured to extract groups from the groups claim
5. WHERE Dex uses a private or self-signed CA, THE kube-apiserver SHALL be configured with the CA certificate for JWKS validation

### Requirement 2: Dex Kubernetes OIDC Client Configuration

**User Story:** As a platform operator, I want Dex configured with a Kubernetes OIDC client, so that developers can obtain tokens for Kubernetes API authentication.

#### Acceptance Criteria

1. THE Dex SHALL define a static OIDC client with ID "kubernetes"
2. WHEN the Kubernetes client is configured, THE Dex SHALL include redirect URIs compatible with local exec plugin callback patterns
3. THE Dex SHALL configure the Kubernetes client with scopes including openid, profile, email, and groups
4. WHEN Dex issues tokens for the Kubernetes client, THE Dex SHALL include the groups claim populated from Authentik
5. THE Dex SHALL ensure the groups claim mapping is correct and stable

### Requirement 3: OIDC Claims Contract

**User Story:** As a platform developer, I want Dex-issued tokens to contain consistent, predictable claims, so that authentication and authorization work reliably.

#### Acceptance Criteria

1. WHEN Dex issues a token for Kubernetes, THE Dex SHALL include the iss claim with the external Dex issuer URL
2. THE Dex SHALL include the aud claim with value "kubernetes"
3. THE Dex SHALL include the groups claim as a list of group strings from Authentik
4. THE Dex SHALL include the email claim as the username identifier
5. THE Dex SHALL include standard OIDC claims (sub, exp, iat) for token validation

### Requirement 4: Platform Group Definitions

**User Story:** As a platform operator, I want standardized platform groups defined in Authentik, so that group membership determines user permissions consistently.

#### Acceptance Criteria

1. THE Auth_System SHALL define a "platform-admins" group in Authentik
2. THE Auth_System SHALL define a "platform-operators" group in Authentik
3. THE Auth_System SHALL define a "platform-engineering" group in Authentik
4. WHEN users are assigned to groups in Authentik, THE Auth_System SHALL include those group names in Dex-issued tokens
5. THE Auth_System SHALL document the purpose and permissions associated with each platform group

### Requirement 5: RBAC ClusterRole for Platform Admins

**User Story:** As a platform admin, I want full access to platform CRDs and necessary supporting resources, so that I can administer the platform effectively.

#### Acceptance Criteria

1. THE RBAC_System SHALL define a ClusterRole named "platform-admin"
2. THE ClusterRole SHALL grant full CRUD access to all platform CRDs
3. THE ClusterRole SHALL grant access to manage namespaces used by the platform
4. THE ClusterRole SHALL grant access to read logs and events in platform namespaces
5. THE ClusterRole SHALL grant access to necessary core Kubernetes resources for platform administration

### Requirement 6: RBAC ClusterRole for Platform Operators

**User Story:** As a platform operator, I want elevated access to platform namespaces and CRDs, so that I can manage team-scoped resources and troubleshoot issues.

#### Acceptance Criteria

1. THE RBAC_System SHALL define a ClusterRole named "platform-operator"
2. THE ClusterRole SHALL grant elevated access to platform namespaces
3. THE ClusterRole SHALL grant access to manage team-scoped platform CRDs
4. THE ClusterRole SHALL grant access to read logs and events in platform namespaces for troubleshooting
5. THE ClusterRole SHALL grant access to manage select resources within defined boundaries

### Requirement 7: RBAC ClusterRole for Platform Engineers

**User Story:** As a platform engineer, I want to create and manage allowed CRDs in allowed namespaces, so that I can use platform features without requiring admin privileges.

#### Acceptance Criteria

1. THE RBAC_System SHALL define a ClusterRole named "platform-engineer"
2. THE ClusterRole SHALL grant create, read, and update access to allowed platform CRDs
3. THE ClusterRole SHALL restrict write access to allowed namespaces only
4. THE ClusterRole SHALL grant read-only access to cluster-scoped objects
5. THE ClusterRole SHALL prevent write access to restricted platform resources

### Requirement 8: RBAC Group-to-Role Bindings

**User Story:** As a platform operator, I want Kubernetes RBAC roles bound to Authentik groups, so that group membership automatically determines permissions.

#### Acceptance Criteria

1. THE RBAC_System SHALL create a ClusterRoleBinding mapping "platform-admins" group to "platform-admin" ClusterRole
2. THE RBAC_System SHALL create a ClusterRoleBinding mapping "platform-operators" group to "platform-operator" ClusterRole
3. THE RBAC_System SHALL create a ClusterRoleBinding mapping "platform-engineering" group to "platform-engineer" ClusterRole
4. WHEN a user authenticates with groups in their token, THE kube-apiserver SHALL apply the corresponding ClusterRole permissions
5. THE RBAC_System SHALL use the groups claim values exactly as emitted by Dex

### Requirement 9: Namespace Scoping Policy

**User Story:** As a platform operator, I want engineers restricted to creating resources in specific namespaces, so that platform namespaces remain protected.

#### Acceptance Criteria

1. THE RBAC_System SHALL implement a namespace scoping policy for platform engineers
2. THE RBAC_System SHALL allow engineers to create CRDs only in user-* or team-* namespaces
3. THE RBAC_System SHALL prevent engineers from creating resources in platform system namespaces
4. THE RBAC_System SHALL allow admins and operators to create resources in all namespaces
5. THE RBAC_System SHALL document the chosen namespace scoping pattern

### Requirement 10: Break-Glass Admin Access

**User Story:** As a platform operator, I want a non-OIDC admin access path, so that I can recover from OIDC misconfigurations without being locked out.

#### Acceptance Criteria

1. THE Bootstrap_System SHALL maintain a non-OIDC authentication method for cluster admin access
2. WHEN OIDC is misconfigured or Dex is unavailable, THE Bootstrap_System SHALL allow operators to access the cluster using break-glass credentials
3. THE Bootstrap_System SHALL document the break-glass access procedure
4. THE Bootstrap_System SHALL ensure break-glass access is available during initial cluster setup
5. THE Bootstrap_System SHALL NOT require OIDC to be functional for break-glass access

### Requirement 11: AphexCLI Authentication Contract

**User Story:** As a developer, I want `aphex login` to configure my local credentials seamlessly, so that I can authenticate to the platform cluster without manual kubeconfig editing.

#### Acceptance Criteria

1. WHEN a developer runs `aphex login`, THE AphexCLI SHALL ensure a local credential mechanism is installed or available
2. THE AphexCLI SHALL write or update a kubeconfig context named "aphex" using an exec credential plugin
3. THE AphexCLI SHALL configure the exec plugin with the platform Dex issuer URL
4. THE AphexCLI SHALL configure the exec plugin with client ID "kubernetes"
5. WHEN login completes, THE AphexCLI SHALL verify cluster connectivity and display success output with context details

### Requirement 12: AphexCLI Permissions Abstraction

**User Story:** As a developer, I want clear, actionable error messages when I lack permissions, so that I understand what access I need without learning Kubernetes RBAC.

#### Acceptance Criteria

1. WHEN a developer lacks required permissions, THE AphexCLI SHALL provide a friendly error message explaining the missing access
2. THE AphexCLI SHALL perform preflight authorization checks before executing commands
3. WHEN authorization fails, THE AphexCLI SHALL infer the missing group membership from RBAC expectations
4. THE AphexCLI SHALL guide users to request access by joining the appropriate Authentik group
5. THE AphexCLI SHALL NOT require users to understand Kubernetes RBAC objects or run kubectl auth can-i manually

### Requirement 13: AphexCLI Resource Creation Contract

**User Story:** As a developer, I want `aphex pipeline create` to use my authenticated credentials to create platform CRDs, so that I can provision platform resources through the CLI.

#### Acceptance Criteria

1. WHEN a developer runs `aphex pipeline create`, THE AphexCLI SHALL use standard kubeconfig loading to obtain credentials
2. THE AphexCLI SHALL automatically benefit from exec plugin token refresh
3. THE AphexCLI SHALL create the relevant CRD instance for pipeline creation
4. WHEN authorization fails, THE AphexCLI SHALL provide clear error messages with actionable guidance
5. THE AphexCLI SHALL perform preflight checks to fail early with clear messaging before attempting resource creation

### Requirement 14: Stable OIDC Issuer and Audience

**User Story:** As a platform operator, I want stable OIDC issuer and audience values, so that clients and the API server have a consistent authentication contract.

#### Acceptance Criteria

1. THE Dex SHALL expose a stable external issuer URL (e.g., https://dex.platform-domain)
2. THE Dex SHALL use "kubernetes" as the client ID and audience value
3. THE kube-apiserver SHALL be configured to accept tokens with the exact issuer URL
4. THE kube-apiserver SHALL be configured to accept tokens with audience "kubernetes"
5. THE Auth_System SHALL document the issuer URL and audience values for client configuration

### Requirement 15: Kubeconfig Context Naming Convention

**User Story:** As a developer, I want a predictable kubeconfig context name, so that I can reference the platform cluster consistently.

#### Acceptance Criteria

1. THE AphexCLI SHALL create or update a kubeconfig context named "aphex"
2. THE AphexCLI SHALL configure the "aphex" context to use the exec credential plugin
3. WHEN developers switch contexts, THE kubectl SHALL use the "aphex" context for platform cluster access
4. THE Auth_System SHALL document the "aphex" context name as the standard platform context
5. THE AphexCLI SHALL NOT create multiple contexts with different names for the same cluster

### Requirement 16: TLS and Certificate Management

**User Story:** As a platform operator, I want Dex accessible via valid TLS, so that OIDC validation succeeds and credentials are transmitted securely.

#### Acceptance Criteria

1. THE Dex SHALL be accessible on the external issuer URL with valid TLS
2. WHERE Dex uses a self-signed certificate, THE kube-apiserver SHALL be configured with the CA certificate
3. THE Dex SHALL use a browser-reachable hostname, not cluster-internal service names
4. THE Auth_System SHALL ensure TLS certificates are valid and not expired
5. THE Auth_System SHALL document certificate management and renewal procedures

### Requirement 17: Bootstrap and GitOps Ownership Boundaries

**User Story:** As a platform operator, I want clear ownership boundaries between bootstrap and GitOps, so that I understand which system manages each component.

#### Acceptance Criteria

1. THE Bootstrap_System SHALL own kube-apiserver OIDC configuration
2. THE Bootstrap_System SHALL own break-glass admin access path configuration
3. THE GitOps_System SHALL own Dex deployment, configuration, and ingress
4. THE GitOps_System SHALL own Authentik deployment and configuration
5. THE GitOps_System SHALL own RBAC manifests for groups, roles, and bindings

### Requirement 18: Avoiding Circular Dependencies

**User Story:** As a platform operator, I want bootstrap to complete successfully even if OIDC is not yet functional, so that I can provision the cluster without circular dependencies.

#### Acceptance Criteria

1. THE Bootstrap_System SHALL NOT require OIDC to be functional to complete bootstrap
2. THE Bootstrap_System SHALL configure kube-apiserver OIDC settings during provisioning
3. WHEN Dex is not yet deployed, THE kube-apiserver SHALL remain accessible via break-glass access
4. THE Bootstrap_System SHALL allow Dex to start after Authentik without blocking cluster provisioning
5. WHEN Dex issuer and TLS are healthy, THE OIDC authentication SHALL become available

### Requirement 19: Developer Login UX Infrastructure Requirements

**User Story:** As a platform operator, I want the infrastructure to support seamless local developer login, so that developers can authenticate without manual configuration.

#### Acceptance Criteria

1. THE Auth_System SHALL ensure Dex is reachable on the external issuer URL from developer machines
2. THE Auth_System SHALL ensure Dex has valid TLS certificates for browser-based authentication
3. THE Auth_System SHALL document the recommended exec credential plugin (e.g., kubelogin)
4. THE Auth_System SHALL provide documentation for the platform login workflow
5. THE Auth_System SHALL ensure redirect URIs support localhost callback patterns for local authentication

### Requirement 20: Capability Matrix Documentation

**User Story:** As a CLI developer, I want a documented mapping of CLI commands to required RBAC permissions, so that I can implement accurate preflight checks and error messages.

#### Acceptance Criteria

1. THE Auth_System SHALL document a capability matrix mapping AphexCLI commands to required RBAC verbs and resources
2. THE capability matrix SHALL specify which group membership is required for each command
3. THE capability matrix SHALL specify which namespaces each command can operate in
4. THE Auth_System SHALL document the expected RBAC checks for preflight validation
5. THE Auth_System SHALL keep the capability matrix synchronized with RBAC policy changes

### Requirement 21: Authentication Validation

**User Story:** As a platform operator, I want to validate that OIDC authentication is working correctly, so that I can verify the system before developers use it.

#### Acceptance Criteria

1. WHEN OIDC is configured, THE kube-apiserver SHALL accept tokens from the configured Dex issuer
2. THE kube-apiserver SHALL reject tokens from other issuers
3. THE kube-apiserver SHALL reject tokens with incorrect audience values
4. WHEN a valid token is presented, THE kube-apiserver SHALL extract username and groups claims
5. THE Auth_System SHALL provide validation procedures to test OIDC authentication

### Requirement 22: Authorization Validation

**User Story:** As a platform operator, I want to validate that RBAC is working correctly, so that I can verify permissions before developers use the system.

#### Acceptance Criteria

1. WHEN a user in platform-admins authenticates, THE kube-apiserver SHALL grant full access to platform CRDs
2. WHEN a user in platform-operators authenticates, THE kube-apiserver SHALL grant elevated access within defined boundaries
3. WHEN a user in platform-engineering authenticates, THE kube-apiserver SHALL grant access to create CRDs in allowed namespaces
4. WHEN a user in platform-engineering attempts to create resources in restricted namespaces, THE kube-apiserver SHALL deny the request
5. THE Auth_System SHALL provide validation procedures using kubectl auth can-i checks

### Requirement 23: Issuer Discovery and JWKS Reachability

**User Story:** As a platform operator, I want to verify that the kube-apiserver can reach the Dex OIDC discovery endpoint and JWKS, so that token validation works correctly.

#### Acceptance Criteria

1. THE kube-apiserver SHALL be able to reach the Dex OIDC discovery endpoint at /.well-known/openid-configuration
2. THE kube-apiserver SHALL be able to fetch the JWKS from the Dex JWKS endpoint
3. WHEN the kube-apiserver cannot reach Dex, THE kube-apiserver SHALL log errors indicating OIDC validation failures
4. THE Auth_System SHALL provide validation procedures to test issuer discovery from the apiserver network path
5. THE Auth_System SHALL document troubleshooting steps for OIDC discovery failures

### Requirement 24: Group Claim Validation

**User Story:** As a platform operator, I want to verify that tokens include the expected groups claim, so that RBAC authorization works correctly.

#### Acceptance Criteria

1. WHEN a user authenticates via Dex, THE token SHALL include a groups claim
2. THE groups claim SHALL contain the user's Authentik group memberships
3. THE groups claim values SHALL match the group names defined in Authentik exactly
4. THE Auth_System SHALL provide procedures to inspect token claims for validation
5. WHEN groups are missing from tokens, THE Auth_System SHALL provide troubleshooting guidance

### Requirement 25: Failure Mode Testing

**User Story:** As a platform operator, I want to verify that break-glass access works when OIDC fails, so that I can recover from authentication system failures.

#### Acceptance Criteria

1. WHEN Dex is unavailable, THE kube-apiserver SHALL remain accessible via break-glass admin credentials
2. WHEN Authentik is unavailable, THE kube-apiserver SHALL remain accessible via break-glass admin credentials
3. THE Auth_System SHALL provide procedures to test break-glass access
4. THE Auth_System SHALL document the break-glass recovery workflow
5. WHEN OIDC is restored, THE kube-apiserver SHALL resume accepting OIDC tokens without requiring restart

### Requirement 26: Documentation Deliverables

**User Story:** As a platform operator and developer, I want comprehensive documentation for the OIDC authentication system, so that I can operate, troubleshoot, and use it effectively.

#### Acceptance Criteria

1. THE Auth_System SHALL document the OIDC issuer URL, client ID, and claims contract
2. THE Auth_System SHALL document the platform groups and their associated permissions
3. THE Auth_System SHALL document the developer login setup steps using the exec credential plugin
4. THE Auth_System SHALL document the capability matrix mapping CLI commands to RBAC permissions
5. THE Auth_System SHALL document troubleshooting procedures for common authentication and authorization issues
