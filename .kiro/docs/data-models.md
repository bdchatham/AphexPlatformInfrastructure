# Data Models

## Overview

The Arbiter Pipeline Infrastructure uses Kubernetes Custom Resource Definitions (CRDs) and standard Kubernetes resources to define data structures. All state is managed by Kubernetes (cluster state) and the onboarding controller (tenant provisioning state).

Data flows through the system in three main forms:
1. **RepoBinding Resources**: Custom resources for onboarding requests
2. **Kubernetes Resources**: YAML manifests for tenant infrastructure
3. **ArgoCD Applications**: GitOps application definitions

## RepoBinding Data Model

### RepoBinding Spec

```typescript
interface RepoBindingSpec {
  repoOrg: string;              // GitHub organization (e.g., "your-github-org")
  repoName: string;             // Repository name (e.g., "archon-agent")
  tenantName: string;           // Tenant namespace name (e.g., "archon")
  permissionProfile: "standard" | "elevated";  // Permission level (default: "standard")
  ingressHost?: string;         // Optional ingress hostname (e.g., "webhooks.example.com")
}
```

**Validation Rules**:
- `repoOrg`: Must match pattern `^[a-z0-9-]+$`
- `repoName`: Must match pattern `^[a-z0-9-]+$`
- `tenantName`: Must match pattern `^[a-z0-9-]+$`, cannot be privileged namespace
- `permissionProfile`: Must be "standard" or "elevated"
- `ingressHost`: Must be valid hostname format (if provided)

**Example**:
```yaml
spec:
  repoOrg: "your-github-org"
  repoName: "archon-agent"
  tenantName: "archon"
  permissionProfile: "standard"
  ingressHost: "webhooks.example.com"
```

### RepoBinding Status

```typescript
interface RepoBindingStatus {
  phase: "Pending" | "Provisioning" | "Ready" | "Failed";
  message: string;
  webhookURL: string;
  webhookSecret: string;
  namespaceCreated: boolean;
  serviceAccountCreated: boolean;
  rbacCreated: boolean;
  quotasCreated: boolean;
  networkPolicyCreated: boolean;
  terraformSecretCreated: boolean;
  eventListenerCreated: boolean;
  ingressCreated: boolean;
}
```

**Phase Transitions**:
```
Pending → Provisioning → Ready
                      ↓
                    Failed
```

**Example**:
```yaml
status:
  phase: Ready
  message: "All resources provisioned successfully"
  webhookURL: "https://webhooks.example.com/archon"
  webhookSecret: "whsec_abc123xyz456"
  namespaceCreated: true
  serviceAccountCreated: true
  rbacCreated: true
  quotasCreated: true
  networkPolicyCreated: true
  terraformSecretCreated: true
  eventListenerCreated: true
  ingressCreated: true
```

## ArgoCD Application Data Model

### Application Spec

```typescript
interface ApplicationSpec {
  project: string;              // ArgoCD project (default: "default")
  source: {
    repoURL: string;            // Git repository URL
    targetRevision: string;     // Git branch/tag/commit (e.g., "main")
    path: string;               // Path within repository
  };
  destination: {
    server: string;             // Kubernetes API server URL
    namespace: string;          // Target namespace
  };
  syncPolicy: {
    automated?: {
      prune: boolean;           // Delete resources not in Git
      selfHeal: boolean;        // Revert manual changes
    };
    retry?: {
      limit: number;            // Max retry attempts
      backoff: {
        duration: string;       // Initial backoff duration
        factor: number;         // Backoff multiplier
        maxDuration: string;    // Max backoff duration
      };
    };
  };
}
```

**Example**:
```yaml
spec:
  project: default
  source:
    repoURL: https://github.com/bdchatham/ArbiterPipelineInfrastructure
    targetRevision: main
    path: platform/argocd/apps
  destination:
    server: https://kubernetes.default.svc
    namespace: argocd
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    retry:
      limit: 5
      backoff:
        duration: 5s
        factor: 2
        maxDuration: 3m
```

### Application Status

```typescript
interface ApplicationStatus {
  sync: {
    status: "Synced" | "OutOfSync" | "Unknown";
    revision: string;           // Git commit SHA
  };
  health: {
    status: "Healthy" | "Progressing" | "Degraded" | "Suspended" | "Missing" | "Unknown";
  };
  conditions: Array<{
    type: string;
    status: string;
    message: string;
  }>;
}
```

**Example**:
```yaml
status:
  sync:
    status: Synced
    revision: abc123def456
  health:
    status: Healthy
  conditions:
    - type: ComparisonError
      status: "False"
      message: ""
```

## EventListener Configuration Data Model

### EventListener Spec

```typescript
interface EventListenerSpec {
  serviceAccountName: string;   // Service account for pipeline execution
  triggers: Array<{
    name: string;               // Trigger name
    interceptors: Array<{
      ref: {
        name: string;           // Interceptor type (e.g., "github", "cel")
      };
      params: Array<{
        name: string;
        value: any;
      }>;
    }>;
    bindings: Array<{
      ref: string;              // TriggerBinding name
    }>;
    template: {
      ref: string;              // TriggerTemplate name
    };
  }>;
}
```

**Example**:
```yaml
spec:
  serviceAccountName: pipeline-runner
  triggers:
    - name: github-push
      interceptors:
        - ref:
            name: github
          params:
            - name: secretRef
              value:
                secretName: webhook-archon
                secretKey: secret
            - name: eventTypes
              value:
                - push
        - ref:
            name: cel
          params:
            - name: filter
              value: "body.ref == 'refs/heads/main'"
      bindings:
        - ref: github-push-binding
      template:
        ref: cdktf-deploy-trigger-template
```

## Pipeline Parameters Data Model

### Pipeline Parameters

```typescript
interface PipelineParams {
  repoUrl: string;       // Git repository URL
  commitSha: string;     // Git commit SHA to build
  tenantName: string;    // Tenant namespace for RBAC context
}
```

**Example**:
```yaml
params:
  - name: repo-url
    value: "https://github.com/your-github-org/archon-agent"
  - name: commit-sha
    value: "abc123def456..."
  - name: tenant-name
    value: "archon"
```

## Terraform Backend Configuration Data Model

### Terraform Backend Config (Kubernetes Backend)

```typescript
interface TerraformBackendConfig {
  backend: "kubernetes";
  secretSuffix: string;      // Tenant name for state isolation
  namespace: string;         // Tenant namespace
  inClusterConfig: boolean;  // Use in-cluster credentials
}
```

**Example**:
```yaml
terraform {
  backend "kubernetes" {
    secret_suffix    = "archon"
    namespace        = "archon"
    in_cluster_config = true
  }
}
```

## Tenant Resource Data Models

### Namespace Labels

```typescript
interface NamespaceLabels {
  "platform.arbiter.io/tenant": string;      // Tenant name
  "platform.arbiter.io/repo": string;        // Repository (org/name)
  "platform.arbiter.io/managed-by": string;  // "onboarding-controller"
}
```

**Example**:
```yaml
labels:
  platform.arbiter.io/tenant: "archon"
  platform.arbiter.io/repo: "your-github-org/archon-agent"
  platform.arbiter.io/managed-by: "onboarding-controller"
```

### Resource Quota Spec

```typescript
interface ResourceQuotaSpec {
  hard: {
    "requests.cpu": string;           // e.g., "4"
    "requests.memory": string;        // e.g., "8Gi"
    "limits.cpu": string;             // e.g., "8"
    "limits.memory": string;          // e.g., "16Gi"
    "persistentvolumeclaims": string; // e.g., "5"
    "pods": string;                   // e.g., "20"
  };
}
```

**Example**:
```yaml
spec:
  hard:
    requests.cpu: "4"
    requests.memory: "8Gi"
    limits.cpu: "8"
    limits.memory: "16Gi"
    persistentvolumeclaims: "5"
    pods: "20"
```

### LimitRange Spec

```typescript
interface LimitRangeSpec {
  limits: Array<{
    type: "Container";
    default: {
      cpu: string;      // e.g., "500m"
      memory: string;   // e.g., "512Mi"
    };
    defaultRequest: {
      cpu: string;      // e.g., "100m"
      memory: string;   // e.g., "128Mi"
    };
  }>;
}
```

**Example**:
```yaml
spec:
  limits:
    - type: Container
      default:
        cpu: "500m"
        memory: "512Mi"
      defaultRequest:
        cpu: "100m"
        memory: "128Mi"
```

### NetworkPolicy Spec

```typescript
interface NetworkPolicySpec {
  podSelector: {};  // Empty selector matches all pods
  policyTypes: ["Ingress", "Egress"];
  ingress: Array<{
    from: Array<{
      podSelector?: {};
      namespaceSelector?: {
        matchLabels: { [key: string]: string };
      };
    }>;
    ports?: Array<{
      protocol: string;
      port: number;
    }>;
  }>;
  egress: Array<{
    to: Array<{
      podSelector?: {};
      namespaceSelector?: {
        matchLabels: { [key: string]: string };
      };
      ipBlock?: {
        cidr: string;
        except?: string[];
      };
    }>;
    ports?: Array<{
      protocol: string;
      port: number;
    }>;
  }>;
}
```

**Example**:
```yaml
spec:
  podSelector: {}
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
        - podSelector: {}
  egress:
    - to:
        - podSelector: {}
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: kube-system
      ports:
        - protocol: UDP
          port: 53
    - to:
        - ipBlock:
            cidr: 0.0.0.0/0
```

## Permission Profile Data Models

### Standard Profile

```typescript
interface StandardProfile {
  role: {
    rules: Array<{
      apiGroups: string[];
      resources: string[];
      verbs: string[];
    }>;
  };
}
```

**Example**:
```yaml
rules:
  - apiGroups: [""]
    resources: ["pods", "pods/log", "configmaps", "secrets"]
    verbs: ["get", "list", "create", "update", "delete"]
  - apiGroups: ["tekton.dev"]
    resources: ["pipelineruns", "taskruns"]
    verbs: ["get", "list", "create"]
```

### Elevated Profile

```typescript
interface ElevatedProfile {
  role: {
    rules: Array<{
      apiGroups: string[];
      resources: string[];
      verbs: string[];
    }>;
  };
}
```

**Example**:
```yaml
rules:
  - apiGroups: [""]
    resources: ["pods", "pods/log", "configmaps", "secrets", "services", "persistentvolumeclaims"]
    verbs: ["get", "list", "create", "update", "delete"]
  - apiGroups: ["tekton.dev"]
    resources: ["pipelineruns", "taskruns", "pipelines", "tasks"]
    verbs: ["get", "list", "create", "update", "delete"]
  - apiGroups: ["apps"]
    resources: ["deployments", "statefulsets"]
    verbs: ["get", "list", "create", "update", "delete"]
```

## Data Flow

### Onboarding Flow

```
User creates RepoBinding
    ↓
Onboarding Controller reconciles
    ↓
Validate spec (org, repo, namespace, profile)
    ↓
Generate webhook secret
    ↓
Create Namespace with labels
    ↓
Create ServiceAccount
    ↓
Create Role (based on profile)
    ↓
Create RoleBinding
    ↓
Create ResourceQuota
    ↓
Create LimitRange
    ↓
Create NetworkPolicy
    ↓
Create Terraform backend secret
    ↓
Create EventListener
    ↓
Create Ingress
    ↓
Update RepoBinding status to Ready
```

### Pipeline Execution Flow

```
GitHub webhook event
    ↓
Ingress routes to EventListener
    ↓
EventListener validates webhook signature
    ↓
EventListener checks CEL filter (main branch)
    ↓
EventListener creates PipelineRun in tenant namespace
    ↓
PipelineRun executes as tenant ServiceAccount
    ↓
Pipeline pods run with RBAC constraints
    ↓
Pipeline accesses Terraform state via secret
    ↓
Pipeline completes, logs stored
```

### GitOps Sync Flow

```
Engineer commits platform changes to Git
    ↓
ArgoCD polls Git repository (every 3 minutes)
    ↓
ArgoCD detects changes
    ↓
ArgoCD compares Git state with cluster state
    ↓
ArgoCD applies changes to cluster
    ↓
ArgoCD updates Application status
```

## Validation Rules

### RepoBinding Validation

- `repoOrg`: Must match `^[a-z0-9-]+$`
- `repoName`: Must match `^[a-z0-9-]+$`
- `tenantName`: Must match `^[a-z0-9-]+$`, cannot be privileged namespace
- `permissionProfile`: Must be "standard" or "elevated"
- `ingressHost`: Must be valid hostname format (if provided)

### Namespace Validation

- Name must be valid DNS label (lowercase alphanumeric and hyphens)
- Name cannot be privileged (kube-system, platform-system, argocd, tekton-pipelines, etc.)
- Name must be unique in cluster

### Pipeline Parameters Validation

- `repoUrl`: Must be valid Git URL
- `commitSha`: Must be valid Git commit hash (40 hex characters)
- `tenantName`: Must be existing namespace

## Authentication Data Models

### Platform Groups

The authentication system defines three platform groups with specific permissions:

```yaml
# platform-admins group
name: platform-admins
is_superuser: true
permissions:
  - Full CRUD access to all platform CRDs
  - Create and delete namespaces
  - Read logs and events in all namespaces

# platform-operators group  
name: platform-operators
is_superuser: false
permissions:
  - Full CRUD access to platform CRDs (no namespace management)
  - Read logs and events in all namespaces
  - Cannot create or delete namespaces

# platform-engineering group
name: platform-engineering
is_superuser: false
permissions:
  - Create, read, update platform CRDs (no delete)
  - Restricted to user-* and team-* namespaces only
  - Cannot access platform system namespaces
```

### JWT Token Claims

Dex-issued JWT tokens contain the following claims structure:

```typescript
interface JWTClaims {
  iss: string;                  // Issuer: "https://dex.home.local"
  sub: string;                  // Subject: unique user identifier
  aud: string;                  // Audience: "kubernetes" | "argocd" | "tekton-dashboard"
  exp: number;                  // Expiration time (Unix timestamp)
  iat: number;                  // Issued at time (Unix timestamp)
  email: string;                // User's email address (used as username)
  email_verified: boolean;      // Whether email is verified
  name: string;                 // User's display name
  groups: string[];             // User's group memberships from Authentik
}
```

**Example Kubernetes API Token**:
```json
{
  "iss": "https://dex.home.local",
  "sub": "CgVhbGljZRIEbW9jaw",
  "aud": "kubernetes",
  "exp": 1704153600,
  "iat": 1704067200,
  "email": "alice@platform.local",
  "email_verified": true,
  "name": "Alice Developer",
  "groups": ["platform-engineering"]
}
```

### Authentik User Schema

```typescript
interface AuthentikUser {
  pk: number;                   // Primary key (unique user ID)
  username: string;             // Username (unique, lowercase)
  name: string;                 // Display name
  email: string;                // Email address (unique)
  is_active: boolean;           // Whether user is active
  is_superuser: boolean;        // Whether user has superuser permissions
  last_login: string;           // ISO 8601 timestamp of last login
  groups: string[];             // Array of group names
  attributes: {                 // Custom user attributes
    [key: string]: any;
  };
}
```

**Example**:
```json
{
  "pk": 1,
  "username": "admin",
  "name": "Admin User",
  "email": "admin@example.com",
  "is_active": true,
  "is_superuser": true,
  "last_login": "2024-01-05T10:00:00Z",
  "groups": ["admins"],
  "attributes": {}
}
```

### Authentik Group Schema

```typescript
interface AuthentikGroup {
  pk: string;                   // Primary key (UUID)
  name: string;                 // Group name (unique)
  is_superuser: boolean;        // Whether group has superuser permissions
  parent: string | null;        // Parent group UUID (null if top-level)
  users: number[];              // Array of user PKs
  attributes: {                 // Custom group attributes
    [key: string]: any;
  };
}
```

**Example**:
```json
{
  "pk": "abc123-def456-ghi789",
  "name": "admins",
  "is_superuser": false,
  "parent": null,
  "users": [1, 2],
  "attributes": {}
}
```

### Authentik OAuth2 Provider Schema

```typescript
interface AuthentikOAuth2Provider {
  pk: number;                   // Primary key
  name: string;                 // Provider name
  authorization_flow: string;   // Authorization flow UUID
  client_type: "confidential" | "public";
  client_id: string;            // OAuth2 client ID
  client_secret: string;        // OAuth2 client secret (masked in responses)
  redirect_uris: string;        // Newline-separated redirect URIs
  signing_key: string;          // Signing key UUID
  access_code_validity: string; // Access code validity duration (e.g., "minutes=1")
  access_token_validity: string;// Access token validity duration (e.g., "minutes=5")
  refresh_token_validity: string;// Refresh token validity duration (e.g., "days=30")
  include_claims_in_id_token: boolean;
  issuer_mode: "global" | "per_provider";
  sub_mode: "hashed_user_id" | "user_id" | "user_username" | "user_email";
}
```

**Example**:
```json
{
  "pk": 1,
  "name": "Dex OIDC Provider",
  "authorization_flow": "abc123-def456",
  "client_type": "confidential",
  "client_id": "dex-client",
  "client_secret": "***",
  "redirect_uris": "https://dex.home.local/callback",
  "signing_key": "ghi789-jkl012",
  "access_code_validity": "minutes=1",
  "access_token_validity": "minutes=5",
  "refresh_token_validity": "days=30",
  "include_claims_in_id_token": true,
  "issuer_mode": "per_provider",
  "sub_mode": "hashed_user_id"
}
```

### Dex Configuration Schema

```typescript
interface DexConfig {
  issuer: string;               // Dex issuer URL (e.g., "https://dex.home.local")
  storage: {
    type: "kubernetes";
    config: {
      inCluster: boolean;       // Use in-cluster Kubernetes credentials
    };
  };
  web: {
    http: string;               // HTTP listen address (e.g., "0.0.0.0:5556")
  };
  logger: {
    level: "debug" | "info" | "warn" | "error";
    format: "json" | "text";
  };
  staticClients: Array<{
    id: string;                 // Client ID
    name: string;               // Client display name
    secretEnv: string;          // Environment variable containing client secret
    redirectURIs: string[];     // Allowed redirect URIs
  }>;
  connectors: Array<{
    type: "oidc";
    id: string;                 // Connector ID
    name: string;               // Connector display name
    config: {
      issuer: string;           // Upstream OIDC issuer URL
      clientID: string;         // Client ID for upstream provider
      clientSecret: string;     // Client secret for upstream provider
      redirectURI: string;      // Dex callback URL
      scopes: string[];         // Requested scopes
      getUserInfo: boolean;     // Fetch user info from userinfo endpoint
      insecureSkipEmailVerified: boolean;
      insecureEnableGroups: boolean;
      claimMapping: {
        groups: string;         // Claim name for groups
      };
    };
  }>;
  expiry: {
    signingKeys: string;        // Signing key rotation interval (e.g., "6h")
    idTokens: string;           // ID token validity (e.g., "24h")
    refreshTokens: {
      validIfNotUsedFor: string;// Refresh token idle timeout (e.g., "2160h")
      absoluteLifetime: string; // Refresh token absolute lifetime (e.g., "3960h")
    };
  };
}
```

**Example**:
```yaml
issuer: https://dex.home.local
storage:
  type: kubernetes
  config:
    inCluster: true
web:
  http: 0.0.0.0:5556
logger:
  level: info
  format: json
staticClients:
  - id: argocd
    name: ArgoCD
    secretEnv: ARGOCD_CLIENT_SECRET
    redirectURIs:
      - https://argocd.home.local/auth/callback
  - id: tekton-dashboard
    name: Tekton Dashboard
    secretEnv: TEKTON_CLIENT_SECRET
    redirectURIs:
      - https://tekton.home.local/auth/callback
connectors:
  - type: oidc
    id: authentik
    name: Authentik
    config:
      issuer: http://authentik.auth-system.svc.cluster.local:9000/application/o/platform-services/
      clientID: dex-client
      clientSecret: $AUTHENTIK_CLIENT_SECRET
      redirectURI: https://dex.home.local/callback
      scopes:
        - openid
        - profile
        - email
        - groups
      getUserInfo: true
      insecureSkipEmailVerified: true
      insecureEnableGroups: true
      claimMapping:
        groups: groups
expiry:
  signingKeys: "6h"
  idTokens: "24h"
  refreshTokens:
    validIfNotUsedFor: "2160h"
    absoluteLifetime: "3960h"
```

### OIDC Token Claims Schema

```typescript
interface OIDCTokenClaims {
  iss: string;                  // Issuer (Dex URL)
  sub: string;                  // Subject (unique user ID)
  aud: string;                  // Audience (client ID)
  exp: number;                  // Expiration time (Unix timestamp)
  iat: number;                  // Issued at time (Unix timestamp)
  name: string;                 // User's display name
  email: string;                // User's email address
  groups: string[];             // User's group memberships
  email_verified: boolean;      // Whether email is verified
}
```

**Example**:
```json
{
  "iss": "https://dex.home.local",
  "sub": "abc123",
  "aud": "argocd",
  "exp": 1704542400,
  "iat": 1704456000,
  "name": "John Doe",
  "email": "john.doe@example.com",
  "groups": ["admins"],
  "email_verified": true
}
```

### ArgoCD RBAC Policy Model

```typescript
interface ArgoCDRBACPolicy {
  policy: {
    csv: string;                // CSV-formatted RBAC policy
  };
  scopes: string;               // RBAC scopes (e.g., "[groups]")
}
```

**Policy CSV Format**:
```
p, role:admin, applications, *, */*, allow
p, role:admin, clusters, *, *, allow
p, role:admin, repositories, *, *, allow
p, role:readonly, applications, get, */*, allow
p, role:readonly, clusters, get, *, allow
p, role:readonly, repositories, get, *, allow
g, admins, role:admin
g, engineering, role:readonly
```

**Policy Rules**:
- `p`: Permission rule (role, resource, action, object, effect)
- `g`: Group mapping (group, role)

**Example**:
```yaml
policy.csv: |
  p, role:admin, applications, *, */*, allow
  p, role:admin, clusters, *, *, allow
  p, role:admin, repositories, *, *, allow
  p, role:readonly, applications, get, */*, allow
  p, role:readonly, clusters, get, *, allow
  p, role:readonly, repositories, get, *, allow
  g, admins, role:admin
  g, engineering, role:readonly
scopes: "[groups]"
```

### Tekton RBAC Model

```typescript
interface TektonRBACModel {
  clusterRole: {
    rules: Array<{
      apiGroups: string[];
      resources: string[];
      verbs: string[];
    }>;
  };
  clusterRoleBinding: {
    subjects: Array<{
      kind: "Group";
      name: string;             // Group name from OIDC token
      apiGroup: "rbac.authorization.k8s.io";
    }>;
    roleRef: {
      kind: "ClusterRole";
      name: string;
      apiGroup: "rbac.authorization.k8s.io";
    };
  };
}
```

**Admin Role Example**:
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: tekton-admin
rules:
  - apiGroups: ["tekton.dev"]
    resources: ["*"]
    verbs: ["*"]
  - apiGroups: [""]
    resources: ["pods", "pods/log"]
    verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: tekton-admin-binding
subjects:
  - kind: Group
    name: admins
    apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: tekton-admin
  apiGroup: rbac.authorization.k8s.io
```

**Read-Only Role Example**:
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: tekton-readonly
rules:
  - apiGroups: ["tekton.dev"]
    resources: ["*"]
    verbs: ["get", "list", "watch"]
  - apiGroups: [""]
    resources: ["pods", "pods/log"]
    verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: tekton-readonly-binding
subjects:
  - kind: Group
    name: engineering
    apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: tekton-readonly
  apiGroup: rbac.authorization.k8s.io
```

### Authentication Secrets Schema

```typescript
interface AuthenticationSecrets {
  "authentik-postgresql": {
    "postgresql-password": string;        // Database password (base64)
    "postgresql-postgres-password": string;// Superuser password (base64)
  };
  "authentik-secrets": {
    "secret-key": string;                 // Authentik secret key (base64)
    "admin-password": string;             // Admin password (base64)
  };
  "dex-secrets": {
    "client-secret": string;              // Dex client secret (base64)
  };
  "authentik-api-token": {
    "token": string;                      // API token (base64)
  };
}
```

**Example**:
```yaml
# authentik-postgresql Secret
apiVersion: v1
kind: Secret
metadata:
  name: authentik-postgresql
  namespace: auth-system
type: Opaque
data:
  postgresql-password: YWJjMTIzZGVmNDU2  # base64 encoded
  postgresql-postgres-password: eHl6Nzg5Z2hpMDEy  # base64 encoded

---
# authentik-secrets Secret
apiVersion: v1
kind: Secret
metadata:
  name: authentik-secrets
  namespace: auth-system
type: Opaque
data:
  secret-key: ZGVmNDU2amtsNzg5bW5vMzQ1cHFyOTAxc3R1MjM0dnd4NTY3eXphYjY3OGNkZTkwMQ==  # base64 encoded
  admin-password: Z2hpNzg5bW5vMzQ1  # base64 encoded

---
# dex-secrets Secret
apiVersion: v1
kind: Secret
metadata:
  name: dex-secrets
  namespace: auth-system
type: Opaque
data:
  client-secret: amtsMTIzbW5vNDU2  # base64 encoded

---
# authentik-api-token Secret
apiVersion: v1
kind: Secret
metadata:
  name: authentik-api-token
  namespace: auth-system
type: Opaque
data:
  token: cHFyOTAxc3R1MjM0dnd4NTY3  # base64 encoded
```

## Authentication Data Flow

### User Authentication Flow

```
User accesses ArgoCD/Tekton Dashboard
    ↓
Service redirects to Dex authorization endpoint
    ↓
Dex redirects to Authentik login page
    ↓
User enters credentials in Authentik
    ↓
Authentik validates credentials against PostgreSQL
    ↓
Authentik returns authorization code to Dex
    ↓
Dex exchanges code for tokens from Authentik
    ↓
Dex returns authorization code to service
    ↓
Service exchanges code for tokens from Dex
    ↓
Service validates ID token and extracts claims
    ↓
Service grants access based on groups claim
```

### Config Sync Job Data Flow

```
Bootstrap creates Authentik API token
    ↓
Bootstrap stores token in authentik-api-token Secret
    ↓
Config Sync Job reads token from Secret
    ↓
Job waits for Authentik to be ready
    ↓
Job fetches OAuth2 providers from Authentik API
    ↓
Job finds Dex OIDC provider by name/client_id/redirect_uris
    ↓
Job reads Dex client secret from dex-secrets Secret
    ↓
Job updates Dex OIDC provider with client secret via API
    ↓
Job verifies Authentik OIDC discovery endpoint
    ↓
Job scales Dex deployment to 1 replica
    ↓
Job waits for Dex to be ready
    ↓
Job verifies Dex OIDC discovery endpoint
    ↓
Authentication system is operational
```

### Secret Generation Flow

```
Bootstrap script starts
    ↓
Generate PostgreSQL password (32 bytes random)
    ↓
Generate Authentik secret key (50 bytes random)
    ↓
Generate Authentik admin password (32 bytes random)
    ↓
Generate Dex client secret (32 bytes random)
    ↓
Create Kubernetes Secrets in auth-system namespace
    ↓
ArgoCD deploys Authentik with secrets
    ↓
Bootstrap waits for Authentik to be ready
    ↓
Bootstrap authenticates to Authentik API
    ↓
Bootstrap creates API token via Authentik API
    ↓
Bootstrap stores API token in authentik-api-token Secret
    ↓
Config Sync Job uses API token to configure Authentik
```

## Validation Rules

### Authentik User Validation

- `username`: Must be unique, lowercase, alphanumeric and hyphens
- `email`: Must be unique, valid email format
- `name`: Required, non-empty string
- `groups`: Must reference existing group names

### Authentik Group Validation

- `name`: Must be unique, non-empty string
- `parent`: Must reference existing group UUID (if not null)

### Dex Configuration Validation

- `issuer`: Must be valid HTTPS URL (or HTTP for internal)
- `staticClients[].id`: Must be unique across all clients
- `staticClients[].redirectURIs`: Must be valid HTTPS URLs
- `connectors[].config.issuer`: Must be valid URL
- `connectors[].config.clientID`: Required, non-empty string
- `connectors[].config.clientSecret`: Required, non-empty string

### OIDC Token Claims Validation

- `iss`: Must match Dex issuer URL
- `aud`: Must match client ID
- `exp`: Must be future timestamp
- `iat`: Must be past timestamp
- `email`: Must be valid email format
- `groups`: Must be array of strings

**Source**
- `.kiro/specs/dex-authentication-platform/design.md`
- `.kiro/specs/dex-authentication-platform/requirements.md`
- `platform/auth/authentik/blueprints-configmap.yaml`
- `platform/auth/dex/configmap.yaml`
- `platform/integrations/argocd-rbac-policy.yaml`
- `platform/integrations/tekton-rbac.yaml`
- `platform/auth/secrets/README.md`

**Source**
- `.kiro/specs/argocd-tekton-platform/design.md`
- `.kiro/specs/argocd-tekton-platform/requirements.md`
- `platform/crds/repobinding-crd.yaml`
- `platform/onboarding/controller/`
- `platform/tenancy/templates/`
- `platform/argocd/apps/`
