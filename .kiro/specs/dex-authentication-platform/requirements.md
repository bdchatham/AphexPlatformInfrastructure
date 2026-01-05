# Requirements Document

## Introduction

This specification defines an authentication and authorization system for the Arbiter Pipeline Infrastructure platform. The system will integrate Authentik as the primary Identity Provider (IdP) with Dex as an OIDC connector layer to provide centralized authentication for platform services including ArgoCD and Tekton Dashboard. The implementation will follow the existing GitOps patterns using ArgoCD for declarative management and bootstrap scripts for initial setup.

## Glossary

- **Authentik**: Modern, self-hosted Identity Provider with web UI for user and group management
- **Authentik_Blueprint**: Declarative YAML configuration file that Authentik auto-applies on startup
- **Dex**: OpenID Connect (OIDC) connector that acts as a proxy between Authentik and platform services
- **OIDC**: OpenID Connect, an authentication protocol built on OAuth 2.0
- **IdP**: Identity Provider, a system that creates, maintains, and manages identity information
- **RBAC**: Role-Based Access Control, a method of regulating access based on user roles
- **ArgoCD**: GitOps continuous delivery tool for Kubernetes
- **Tekton**: Cloud-native CI/CD pipeline framework
- **Platform_Services**: ArgoCD UI, Tekton Dashboard, and future platform service UIs
- **Auth_System**: The authentication and authorization infrastructure including Authentik, Dex, PostgreSQL, RBAC policies, and service integrations
- **Internal_User**: A user defined in Authentik (via blueprints or web UI)
- **Connector**: An external identity provider integrated with Authentik (e.g., GitHub, LDAP, SAML)
- **Service_Account**: Kubernetes ServiceAccount used by Dex for cluster operations
- **PostgreSQL**: Relational database used by Authentik for storing user data

## Requirements

### Requirement 1: Authentik Identity Provider Deployment

**User Story:** As a platform operator, I want Authentik deployed as the central identity provider with web UI, so that I can manage users and groups without editing configuration files.

#### Acceptance Criteria

1. THE Auth_System SHALL deploy Authentik in the auth-system namespace
2. THE Auth_System SHALL deploy PostgreSQL database for Authentik data storage
3. WHEN Authentik is deployed, THE Auth_System SHALL configure Authentik with OIDC provider capabilities
4. THE Auth_System SHALL configure Authentik with appropriate resource limits and health checks
5. THE Auth_System SHALL expose Authentik web UI on port 9000 within the cluster

### Requirement 2: Dex OIDC Connector Deployment

**User Story:** As a platform operator, I want Dex deployed as an OIDC connector between Authentik and platform services, so that services have a stable integration point.

#### Acceptance Criteria

1. THE Auth_System SHALL deploy Dex in the auth-system namespace
2. WHEN Dex is deployed, THE Auth_System SHALL configure Dex to use Authentik as OIDC connector
3. WHEN Dex starts, THE Auth_System SHALL use Kubernetes storage backend for minimal state
4. THE Auth_System SHALL configure Dex with appropriate resource limits and health checks
5. THE Auth_System SHALL expose Dex service on port 5556 within the cluster

### Requirement 3: Automated Initial Configuration

**User Story:** As a platform operator, I want Authentik configured automatically during bootstrap using Blueprints and in-cluster Jobs, so that the system is ready to use without manual UI configuration or exposing secrets outside the cluster.

#### Acceptance Criteria

1. THE Auth_System SHALL define Authentik configuration as Blueprints in Git
2. WHEN Authentik starts, THE Auth_System SHALL auto-apply Blueprint configuration
3. THE Auth_System SHALL create initial admin user via Blueprint
4. THE Auth_System SHALL create initial groups (admins, engineering) via Blueprint
5. THE Auth_System SHALL use a Kubernetes Job to configure Authentik OIDC provider with Dex client secret

### Requirement 4: Web-Based User Management

**User Story:** As a platform operator, I want to manage users and groups through Authentik web UI after initial setup, so that I can add, modify, and remove users without editing configuration files or restarting pods.

#### Acceptance Criteria

1. THE Auth_System SHALL provide Authentik web UI for user management
2. WHEN a user is created in Authentik UI, THE Auth_System SHALL store user data in PostgreSQL
3. WHEN a user is assigned to groups, THE Auth_System SHALL update group memberships immediately
4. THE Auth_System SHALL allow users to authenticate immediately after creation
5. THE Auth_System SHALL NOT require configuration file changes or pod restarts for user management

### Requirement 5: Internal User Authentication

**User Story:** As a platform operator, I want to define internal users in Authentik, so that I can provide platform access without external identity providers.

#### Acceptance Criteria

1. THE Auth_System SHALL support internal user authentication in Authentik
2. WHEN an internal user is defined, THE Auth_System SHALL store credentials securely in PostgreSQL
3. THE Auth_System SHALL define at least two user roles: admin and engineer
4. WHEN a user authenticates, THE Auth_System SHALL include user groups in the OIDC token
5. THE Auth_System SHALL configure password policies and session expiry

### Requirement 5: External Identity Provider Integration

**User Story:** As a platform operator, I want to integrate external identity providers with Authentik, so that users can authenticate using existing organizational credentials.

#### Acceptance Criteria

1. THE Auth_System SHALL support GitHub OAuth connector configuration in Authentik
2. WHEN a GitHub connector is configured, THE Auth_System SHALL validate organization membership
3. WHERE GitHub teams are specified, THE Auth_System SHALL map teams to user groups
4. THE Auth_System SHALL support adding additional connectors (LDAP, SAML) via Authentik UI
5. THE Auth_System SHALL handle connector authentication failures gracefully

### Requirement 6: ArgoCD OIDC Integration

**User Story:** As a platform operator, I want to integrate external identity providers with Authentik, so that users can authenticate using existing organizational credentials.

#### Acceptance Criteria

1. THE Auth_System SHALL support GitHub OAuth connector configuration in Authentik
2. WHEN a GitHub connector is configured, THE Auth_System SHALL validate organization membership
3. WHERE GitHub teams are specified, THE Auth_System SHALL map teams to user groups
4. THE Auth_System SHALL support adding additional connectors (LDAP, SAML) via Authentik UI
5. THE Auth_System SHALL handle connector authentication failures gracefully

### Requirement 6: ArgoCD OIDC Integration

**User Story:** As a platform user, I want to authenticate to ArgoCD using Authentik via Dex, so that I can access the ArgoCD UI with my platform credentials.

#### Acceptance Criteria

1. WHEN ArgoCD is deployed, THE Auth_System SHALL configure ArgoCD with Dex OIDC settings
2. THE Auth_System SHALL configure ArgoCD RBAC policies based on user groups from Authentik
3. WHEN a user with admin group authenticates, THE Auth_System SHALL grant full ArgoCD access
4. WHEN a user with engineer group authenticates, THE Auth_System SHALL grant read-only ArgoCD access
5. THE Auth_System SHALL configure ArgoCD to use OIDC for both UI and CLI authentication

### Requirement 7: Tekton Dashboard OIDC Integration

**User Story:** As a platform user, I want to authenticate to Tekton Dashboard using Authentik via Dex, so that I can view and manage pipelines with my platform credentials.

#### Acceptance Criteria

1. WHEN Tekton Dashboard is deployed, THE Auth_System SHALL configure it with Dex OIDC settings
2. THE Auth_System SHALL configure Tekton Dashboard RBAC based on user groups from Authentik
3. WHEN a user authenticates, THE Auth_System SHALL enforce namespace-based access control
4. THE Auth_System SHALL allow users to view pipelines in their authorized namespaces
5. THE Auth_System SHALL prevent users from accessing pipelines in unauthorized namespaces

### Requirement 8: GitOps Configuration Management

**User Story:** As a platform operator, I want authentication infrastructure managed through GitOps, so that changes are version-controlled and automatically applied.

#### Acceptance Criteria

1. THE Auth_System SHALL define Authentik and Dex configuration in Git-managed manifests
2. WHEN Authentik or Dex configuration changes in Git, THE Auth_System SHALL sync changes via ArgoCD
3. THE Auth_System SHALL define RBAC policies in Git-managed manifests
4. THE Auth_System SHALL organize authentication manifests in the platform directory structure
5. WHEN authentication manifests are updated, THE Auth_System SHALL apply changes without manual intervention

### Requirement 9: Bootstrap Integration

**User Story:** As a platform operator, I want authentication components bootstrapped automatically during initial cluster setup, so that the platform is secure and ready to use without manual configuration.

#### Acceptance Criteria

1. WHEN bootstrap script runs, THE Auth_System SHALL create the auth-system namespace
2. THE Auth_System SHALL deploy PostgreSQL during bootstrap before Authentik
3. THE Auth_System SHALL deploy Authentik with Blueprints ConfigMap during bootstrap
4. THE Auth_System SHALL deploy Dex during bootstrap configured to use Authentik
5. WHEN bootstrap completes, THE Auth_System SHALL have fully configured authentication without manual UI steps

### Requirement 10: RBAC Policy Management

**User Story:** As a platform operator, I want to define and manage RBAC policies for platform services, so that users have appropriate access based on their roles.

#### Acceptance Criteria

1. THE Auth_System SHALL define RBAC policies for ArgoCD based on user groups from Authentik
2. THE Auth_System SHALL define RBAC policies for Tekton based on user groups from Authentik
3. WHEN a new user group is added in Authentik, THE Auth_System SHALL support adding corresponding RBAC policies
4. THE Auth_System SHALL enforce least-privilege access by default
5. THE Auth_System SHALL document RBAC policy structure and customization options

### Requirement 11: Secret Management

**User Story:** As a platform operator, I want sensitive authentication credentials managed securely, so that secrets are not exposed in Git or logs.

#### Acceptance Criteria

1. THE Auth_System SHALL store PostgreSQL credentials in Kubernetes Secrets
2. THE Auth_System SHALL store Authentik secret key in Kubernetes Secrets
3. THE Auth_System SHALL store OIDC client secrets in Kubernetes Secrets
4. THE Auth_System SHALL NOT commit plaintext secrets to Git
5. THE Auth_System SHALL provide documentation for secret generation and rotation

### Requirement 12: Service Discovery and Networking

**User Story:** As a platform operator, I want services to discover Authentik and Dex automatically, so that authentication works without manual configuration of endpoints.

#### Acceptance Criteria

1. THE Auth_System SHALL expose Authentik via Kubernetes Service with stable DNS name
2. THE Auth_System SHALL expose Dex via Kubernetes Service with stable DNS name
3. WHEN services configure OIDC, THE Auth_System SHALL use cluster-internal service URLs
4. WHERE external access is needed, THE Auth_System SHALL support Ingress configuration
5. THE Auth_System SHALL configure appropriate network policies for auth-system namespace

### Requirement 13: Monitoring and Observability

**User Story:** As a platform operator, I want to monitor authentication system health, so that I can detect and resolve issues proactively.

#### Acceptance Criteria

1. THE Auth_System SHALL expose Authentik health check endpoint
2. THE Auth_System SHALL expose Dex health check endpoint
3. WHEN Authentik or Dex is unhealthy, THE Auth_System SHALL restart pods automatically
4. THE Auth_System SHALL configure Authentik and Dex to log authentication events
5. THE Auth_System SHALL integrate logs with cluster logging infrastructure

### Requirement 14: Database Management

**User Story:** As a platform operator, I want PostgreSQL database managed reliably, so that user data is persisted and available.

#### Acceptance Criteria

1. THE Auth_System SHALL deploy PostgreSQL as StatefulSet with persistent storage
2. WHEN PostgreSQL is deployed, THE Auth_System SHALL configure appropriate resource limits
3. THE Auth_System SHALL configure PostgreSQL with health checks
4. THE Auth_System SHALL persist PostgreSQL data across pod restarts
5. THE Auth_System SHALL document database backup and restore procedures

### Requirement 16: In-Cluster Configuration Orchestration

**User Story:** As a platform operator, I want cross-system configuration managed by in-cluster Jobs, so that secrets never leave the cluster and configuration is automated without manual API calls.

#### Acceptance Criteria

1. THE Auth_System SHALL use a Kubernetes Job to orchestrate Authentik and Dex configuration
2. WHEN the Job runs, THE Auth_System SHALL read Dex client secret from Kubernetes Secret
3. THE Auth_System SHALL update Authentik OIDC provider via Authentik API with the Dex client secret
4. WHEN Authentik configuration is updated, THE Auth_System SHALL trigger Dex rollout to pick up changes
5. THE Auth_System SHALL make the Job idempotent and re-runnable for safe retries

### Requirement 17: Documentation and Operational Procedures

**User Story:** As a platform operator, I want comprehensive documentation for the authentication system, so that I can operate and troubleshoot it effectively.

#### Acceptance Criteria

1. THE Auth_System SHALL document Authentik web UI usage for user management
2. THE Auth_System SHALL document procedures for adding new users and groups
3. THE Auth_System SHALL document procedures for integrating external identity providers
4. THE Auth_System SHALL document procedures for integrating new platform services with Dex
5. THE Auth_System SHALL document common troubleshooting scenarios and resolutions
