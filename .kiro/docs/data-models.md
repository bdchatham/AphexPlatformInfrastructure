# Data Models

## Overview

The Arbiter Pipeline Infrastructure uses Kubernetes Custom Resource Definitions (CRDs), standard Kubernetes resources, and configuration data structures to manage platform state. All data is stored in Kubernetes etcd with GitOps-managed configuration.

Data flows through the system in four main forms:
1. **Organization Resources**: Multi-tenant organization management with webhook infrastructure
2. **RepoBinding Resources**: Custom resources for repository-to-pipeline integration
3. **ArgoCD Applications**: GitOps application definitions with sync waves  
4. **Authentication Data**: User accounts, groups, and OIDC configuration
5. **Certificate Resources**: TLS certificates and issuers managed by cert-manager

For detailed architecture, see [architecture.md](architecture.md).
For operational procedures, see [operations.md](operations.md).

## Organization Data Model

### Organization Spec

```typescript
interface OrganizationSpec {
  displayName: string;          // Human-readable organization name
  adminUsers: string[];         // List of admin email addresses
  webhookSecret?: string;       // GitHub webhook secret (auto-generated if not provided)
}
```

**Validation Rules**:
- `displayName`: Required, must match pattern `^[a-zA-Z0-9\s\-\.]+$`
- `adminUsers`: Required array, each email must match pattern `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
- `webhookSecret`: Optional, must match pattern `^[a-zA-Z0-9_-]+$` if provided

**Example**:
```yaml
apiVersion: arbiter.io/v1alpha1
kind: Organization
metadata:
  name: acme-corp
  namespace: platform-system
spec:
  displayName: "ACME Corporation"
  adminUsers:
    - "admin@acme-corp.com"
    - "ops@acme-corp.com"
  webhookSecret: "my-custom-secret"  # Optional
```

### Organization Status

```typescript
interface OrganizationStatus {
  namespace: string;            // Organization namespace (org-{name})
  webhookURL: string;           // Public webhook URL via Cloudflare tunnel
  phase: "Pending" | "Active" | "Failed";
  message?: string;             // Human-readable status message
}
```

**Example**:
```yaml
status:
  namespace: "org-acme-corp"
  webhookURL: "https://acme-corp.arbiter-dev.com"
  phase: "Active"
```

For operational procedures on creating and managing organizations, see [operations.md](operations.md#bootstrap-organization).

### Cloudflare Tunnel Credentials

Stored as a Secret in the organization namespace:

```typescript
interface TunnelCredentials {
  AccountTag: string;           // Cloudflare account ID
  TunnelID: string;             // Unique tunnel identifier
  TunnelSecret: string;         // Base64-encoded tunnel secret
}
```

**Secret Name**: `cloudflared-credentials-{org-name}`

**Example**:
```json
{
  "AccountTag": "65671c123fa015f89bfb9f110d0000fd",
  "TunnelID": "1280a396-767b-4e77-9425-735765b5f1ae",
  "TunnelSecret": "Wl0r8cioWrFIUVb625PfPDOG6rYAgkBkRxzA3QoqGCo="
}
```

### DNS Record Model

Automatically created in Cloudflare for each organization:

```typescript
interface DNSRecord {
  type: "CNAME";
  name: string;                 // {org-name}.arbiter-dev.com
  content: string;              // {tunnel-id}.cfargotunnel.com
  proxied: boolean;             // true (enables Cloudflare edge features)
}
```

**Example**:
```json
{
  "type": "CNAME",
  "name": "acme-corp.arbiter-dev.com",
  "content": "1280a396-767b-4e77-9425-735765b5f1ae.cfargotunnel.com",
  "proxied": true
}
```

**Source**
- `platform/onboarding/controller/api/v1alpha1/organization_types.go` - Go type definition
- `platform/crds/organization-crd.yaml` - CRD definition
- `platform/onboarding/controller/controllers/organization_controller.go` - Status management and tunnel provisioning

## RepoBinding Data Model

### RepoBinding Spec

```typescript
interface RepoBindingSpec {
  aphexOrg: string;             // Organization name (references Organization resource)
  repoOrg: string;              // GitHub organization (e.g., "acme-corp")
  repoName: string;             // Repository name (e.g., "my-application")
  pipelineName: string;         // Pipeline name to trigger (e.g., "cdktf-deploy-pipeline")
  templateRef: string;          // Dispatcher template name (e.g., "run-pipeline-v1")
  pipelineSpec: string;         // Raw YAML content of Tekton Pipeline to create
}
```

**Validation Rules**:
- `aphexOrg`: Required, must match pattern `^[a-z0-9-]+$`
- `repoOrg`: Required, must match pattern `^[a-z0-9-]+$`
- `repoName`: Required, must match pattern `^[a-z0-9-]+$`
- `pipelineName`: Required, must match pattern `^[a-z0-9-]+$`
- `templateRef`: Required, non-empty string
- `pipelineSpec`: Required, must contain valid Tekton Pipeline YAML

**Example**:
```yaml
spec:
  aphexOrg: "acme-corp"
  repoOrg: "acme-corp"
  repoName: "my-application"
  pipelineName: "cdktf-deploy-pipeline"
  templateRef: "run-pipeline-v1"
  pipelineSpec: |
    apiVersion: tekton.dev/v1
    kind: Pipeline
    metadata:
      name: cdktf-deploy-pipeline
    spec:
      params:
        - name: git-url
        - name: git-revision
      tasks:
        - name: deploy
          taskRef:
            name: cdktf-deploy
          params:
            - name: git-url
              value: $(params.git-url)
            - name: git-revision
              value: $(params.git-revision)
```

### RepoBinding Status

```typescript
interface RepoBindingStatus {
  phase: "Pending" | "Provisioning" | "Ready" | "Failed";
  message: string;
  conditions: Condition[];
  webhookConfiguration: {
    url: string;
    secret: string;
    events: string[];
  };
  provisionedResources: {
    namespace: boolean;
    serviceAccount: boolean;
    rbac: boolean;
    resourceQuota: boolean;
    networkPolicy: boolean;
    eventListener: boolean;
    pipelineResources: boolean;
  };
}

interface Condition {
  type: string;
  status: "True" | "False" | "Unknown";
  reason: string;
  message: string;
  lastTransitionTime: string;
}
```

**Phase Transitions**:
```
Pending → Provisioning → Ready
                      ↓
                    Failed
```

**Condition Types**:
- `NamespaceReady`: Tenant namespace created and configured
- `RBACReady`: Service account and RBAC policies configured
- `NetworkPolicyReady`: Network isolation policies applied
- `EventListenerReady`: Tekton webhook handler configured
- `WebhookReady`: GitHub webhook configuration available

For operational procedures on creating and managing repo bindings, see [operations.md](operations.md#create-repobinding).
For API details on RepoBinding resources, see [api.md](api.md#repobinding-api).

## ArgoCD Application Data Model

### Application Spec with Sync Waves

```typescript
interface ApplicationSpec {
  project: string;              // ArgoCD project (default: "default")
  source: {
    repoURL: string;            // Git repository URL
    targetRevision: string;     // Git branch/tag/commit (e.g., "HEAD")
    path: string;               // Path within repository
  };
  destination: {
    server: string;             // Kubernetes API server URL
    namespace?: string;         // Target namespace (optional)
  };
  syncPolicy: {
    automated: {
      prune: boolean;           // Delete resources not in Git
      selfHeal: boolean;        // Correct drift automatically
    };
    syncOptions: string[];      // Additional sync options
    retry: {
      limit: number;            // Max retry attempts
      backoff: {
        duration: string;       // Initial backoff duration
        factor: number;         // Backoff multiplier
        maxDuration: string;    // Maximum backoff duration
      };
    };
  };
}
```

### Sync Wave Annotations

```typescript
interface SyncWaveAnnotations {
  "argocd.argoproj.io/sync-wave": string;  // Deployment order (e.g., "10", "20", "30")
}
```

**Platform Sync Waves**:
- **Wave 0**: `platform-ingress-controller` - Ingress controller deployment
- **Wave 1**: `platform-tekton` - Tekton Pipelines and Triggers
- **Wave 5**: `platform-crds`, `platform-rbac` - CRDs and RBAC policies
- **Wave 10**: `platform-cert-manager`, `platform-auth` - cert-manager and authentication
- **Wave 20**: `platform-cert-foundation`, `platform-controllers`, `platform-catalog` - Certificates and controllers
- **Wave 30**: `platform-ingress` - Ingress resources with TLS

## Authentication Data Model

### Authentik User Data

```typescript
interface AuthentikUser {
  pk: number;                   // Primary key
  username: string;             // Username
  email: string;                // Email address
  name: string;                 // Display name
  is_active: boolean;           // Account active status
  groups: number[];             // Group membership (by group PK)
  attributes: Record<string, any>; // Custom attributes
}
```

### Authentik Group Data

```typescript
interface AuthentikGroup {
  pk: number;                   // Primary key
  name: string;                 // Group name (e.g., "admins", "engineering")
  is_superuser: boolean;        // Superuser privileges
  users: number[];              // User membership (by user PK)
  attributes: Record<string, any>; // Custom attributes
}
```

**Platform Groups**:
- `admins`: Full access to all platform services
- `engineering`: Read-only access to platform services

### OIDC Provider Configuration

```typescript
interface OIDCProvider {
  name: string;                 // Provider name (e.g., "dex")
  client_id: string;            // OIDC client ID
  client_secret: string;        // OIDC client secret
  authorization_url: string;    // Authorization endpoint
  access_token_url: string;     // Token endpoint
  profile_url: string;          // User info endpoint
  oidc_jwks_url: string;        // JWKS endpoint
  issuer: string;               // OIDC issuer URL
}
```

### Dex Configuration

```typescript
interface DexConfig {
  issuer: string;               // Dex issuer URL (e.g., "https://dex.home.local")
  storage: {
    type: "kubernetes";
    config: {
      inCluster: boolean;
    };
  };
  web: {
    http: string;               // HTTP listen address
    tlsCert?: string;           // TLS certificate path
    tlsKey?: string;            // TLS key path
  };
  connectors: DexConnector[];
  staticClients: DexClient[];
  oauth2: {
    skipApprovalScreen: boolean;
  };
}

interface DexConnector {
  type: "oidc";
  id: string;                   // Connector ID
  name: string;                 // Display name
  config: {
    issuer: string;             // Authentik issuer URL
    clientID: string;           // OIDC client ID
    clientSecret: string;       // OIDC client secret
    redirectURI: string;        // Redirect URI
    scopes: string[];           // OIDC scopes
    claimsMapping: {
      groups: string;           // Groups claim name
    };
  };
}

interface DexClient {
  id: string;                   // Client ID (e.g., "argocd", "tekton-dashboard")
  redirectURIs: string[];       // Allowed redirect URIs
  name: string;                 // Client display name
  secret: string;               // Client secret
}
```

## Certificate Data Model

### Certificate Resource

```typescript
interface Certificate {
  apiVersion: "cert-manager.io/v1";
  kind: "Certificate";
  metadata: {
    name: string;               // Certificate name (e.g., "dex-tls")
    namespace: string;          // Target namespace
  };
  spec: {
    secretName: string;         // Secret name for certificate storage
    issuerRef: {
      name: string;             // Issuer name (e.g., "selfsigned-issuer")
      kind: "ClusterIssuer" | "Issuer";
    };
    dnsNames: string[];         // DNS names for certificate
    duration?: string;          // Certificate validity duration
    renewBefore?: string;       // Renewal threshold
  };
  status?: {
    conditions: CertificateCondition[];
    renewalTime?: string;
  };
}

interface CertificateCondition {
  type: "Ready" | "Issuing";
  status: "True" | "False" | "Unknown";
  reason: string;
  message: string;
  lastTransitionTime: string;
}
```

### ClusterIssuer Resource

```typescript
interface ClusterIssuer {
  apiVersion: "cert-manager.io/v1";
  kind: "ClusterIssuer";
  metadata: {
    name: string;               // Issuer name (e.g., "selfsigned-issuer")
  };
  spec: {
    selfSigned?: {};            // Self-signed issuer configuration
    acme?: {                    // ACME/Let's Encrypt configuration
      server: string;           // ACME server URL
      email: string;            // Contact email
      privateKeySecretRef: {
        name: string;           // Secret for ACME private key
      };
      solvers: ACMESolver[];
    };
  };
  status?: {
    conditions: IssuerCondition[];
    acme?: {
      uri: string;
      lastRegisteredEmail: string;
    };
  };
}
```

**Platform Certificates**:
- `dex-tls`: TLS certificate for Dex OIDC connector
- `authentik-tls`: TLS certificate for Authentik identity provider
- `argocd-tls`: TLS certificate for ArgoCD UI
- `tekton-tls`: TLS certificate for Tekton Dashboard

## Configuration Data Flow

### Bootstrap Secret Generation

```typescript
interface GeneratedSecrets {
  postgresql: {
    password: string;           // PostgreSQL user password
    postgresPassword: string;   // PostgreSQL superuser password
  };
  authentik: {
    secretKey: string;          // Authentik SECRET_KEY
    adminPassword: string;      // Admin user password
    bootstrapToken: string;     // API token for automation
  };
  dex: {
    clientSecret: string;       // OIDC client secret
  };
  argocd: {
    oidcClientSecret: string;   // ArgoCD OIDC client secret
  };
  tekton: {
    oidcClientSecret: string;   // Tekton Dashboard OIDC client secret
  };
}
```

### Config Sync Job Data

```typescript
interface ConfigSyncJobData {
  authentikConfig: {
    baseUrl: string;            // Authentik base URL
    token: string;              // API token
    providerId: number;         // OIDC provider ID
  };
  dexConfig: {
    clientSecret: string;       // Client secret to update
    namespace: string;          // Dex deployment namespace
    deploymentName: string;     // Dex deployment name
  };
  validation: {
    authentikDiscovery: string; // Authentik OIDC discovery URL
    dexDiscovery: string;       // Dex OIDC discovery URL
  };
}
```

**Source**
- `platform/crds/repobinding-crd.yaml` - RepoBinding CRD definition
- `platform/argocd/apps/` - ArgoCD application definitions
- `platform/auth/authentik/blueprints-configmap.yaml` - Authentik configuration
- `platform/auth/dex/dex-config.yaml` - Dex configuration
- `platform/cert-foundation/certificates.yaml` - Certificate definitions
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
  "arbiter.io/tenant": string;      // Tenant name
  "arbiter.io/repo": string;        // Repository (org/name)
  "arbiter.io/managed-by": string;  // "onboarding-controller"
}
```

**Example**:
```yaml
labels:
  arbiter.io/tenant: "archon"
  arbiter.io/repo: "your-github-org/archon-agent"
  arbiter.io/managed-by: "onboarding-controller"
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

- `aphexOrg`: Must match `^[a-z0-9-]+$`
- `repoOrg`: Must match `^[a-z0-9-]+$`
- `repoName`: Must match `^[a-z0-9-]+$`
- `pipelineName`: Must match `^[a-z0-9-]+$`
- `templateRef`: Required, non-empty string

### Namespace Validation

- Name must be valid DNS label (lowercase alphanumeric and hyphens)
- Name cannot be privileged (kube-system, pipeline-system, argocd, tekton-pipelines, etc.)
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
