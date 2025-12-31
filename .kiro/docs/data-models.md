# Data Models

## Overview

The Arbiter Pipeline Infrastructure uses Kubernetes Custom Resource Definitions (CRDs) and ConfigMaps to define data structures. All state is managed by Kubernetes (cluster state) and the onboarding controller (tenant provisioning state).

Data flows through the system in three main forms:
1. **RepoBinding Resources**: Custom resources for onboarding requests
2. **Kubernetes Resources**: YAML manifests for tenant infrastructure
3. **Configuration Data**: ConfigMaps for allowlist and Lighthouse configuration

## RepoBinding Data Model

### RepoBinding Spec

```typescript
interface RepoBindingSpec {
  repoOrg: string;              // GitHub organization (e.g., "your-github-org")
  repoName: string;             // Repository name (e.g., "archon-agent")
  tenantName: string;           // Tenant namespace name (e.g., "archon")
  permissionProfile: "standard" | "elevated";  // Permission level (default: "standard")
}
```

**Validation Rules**:
- `repoOrg`: Must match pattern `^[a-z0-9-]+$`, must be in approved organization list
- `repoName`: Must match pattern `^[a-z0-9-]+$`
- `tenantName`: Must match pattern `^[a-z0-9-]+$`, cannot be privileged namespace
- `permissionProfile`: Must be "standard" or "elevated"

**Example**:
```yaml
spec:
  repoOrg: "your-github-org"
  repoName: "archon-agent"
  tenantName: "archon"
  permissionProfile: "standard"
```

### RepoBinding Status

```typescript
interface RepoBindingStatus {
  phase: "Pending" | "Provisioning" | "Ready" | "Failed";
  message: string;
  namespaceCreated: boolean;
  serviceAccountCreated: boolean;
  rbacConfigured: boolean;
  allowlistUpdated: boolean;
  lastReconcileTime: string;  // ISO 8601 timestamp
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
  namespaceCreated: true
  serviceAccountCreated: true
  rbacConfigured: true
  allowlistUpdated: true
  lastReconcileTime: "2024-12-31T10:00:00Z"
```

## Allowlist Data Model

### Allowlist Entry

```typescript
interface AllowlistEntry {
  org: string;           // GitHub organization
  name: string;          // Repository name
  tenant: string;        // Tenant namespace
  enabled: boolean;      // Whether triggers are active (default: true)
}
```

**Example**:
```yaml
repos:
  - org: "your-github-org"
    name: "archon-agent"
    tenant: "archon"
    enabled: true
  - org: "your-github-org"
    name: "another-repo"
    tenant: "another"
    enabled: true
```

**Storage**: ConfigMap `repo-allowlist` in `pipeline-system` namespace

## Pipeline Parameters Data Model

### Pipeline Parameters

```typescript
interface PipelineParams {
  repoUrl: string;       // Git repository URL
  commitSha: string;     // Git commit SHA to build
  tenantName: string;    // Tenant namespace for RBAC context
  branch: string;        // Git branch (for reference)
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
  - name: branch
    value: "main"
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

### Terraform Backend Config (MinIO/S3 Backend)

```typescript
interface TerraformBackendConfigS3 {
  backend: "s3";
  bucket: string;        // State bucket name
  key: string;           // State file key
  region: string;        // AWS region or equivalent
  endpoint: string;      // S3-compatible endpoint URL
  skipCredentialsValidation: boolean;
  skipMetadataApiCheck: boolean;
  skipRegionValidation: boolean;
  forcePathStyle: boolean;
}
```

**Example**:
```yaml
terraform {
  backend "s3" {
    bucket = "terraform-state-archon"
    key    = "state.tfstate"
    region = "us-east-1"
    endpoint = "http://minio.storage-system.svc.cluster.local:9000"
    skip_credentials_validation = true
    skip_metadata_api_check = true
    skip_region_validation = true
    force_path_style = true
  }
}
```

## Lighthouse Configuration Data Model

### Lighthouse Config

```typescript
interface LighthouseConfig {
  github: {
    appId: string;              // GitHub App ID
    appInstallationId: string;  // GitHub App Installation ID
  };
  allowlist: Array<{
    org: string;                // GitHub organization
    repos: string[];            // List of repository names
  }>;
}
```

**Example**:
```yaml
github:
  app_id: "123456"
  app_installation_id: "78901234"
allowlist:
  - org: "your-github-org"
    repos:
      - "archon-agent"
      - "another-repo"
```

**Storage**: ConfigMap `lighthouse-config` in `pipeline-system` namespace

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
Validate spec (org, namespace, profile)
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
Update allowlist ConfigMap
    ↓
Update RepoBinding status to Ready
```

### Pipeline Execution Flow

```
GitHub webhook event
    ↓
Lighthouse receives event
    ↓
Check allowlist ConfigMap
    ↓
Lookup tenant namespace from allowlist
    ↓
Create PipelineRun in tenant namespace
    ↓
PipelineRun executes as tenant ServiceAccount
    ↓
Pipeline pods run with RBAC constraints
    ↓
Pipeline accesses Terraform state via secret
    ↓
Pipeline completes, logs stored
```

### Configuration Update Flow

```
User updates RepoBinding
    ↓
Onboarding Controller detects change
    ↓
Reconcile resources (idempotent)
    ↓
Update status
```

## Validation Rules

### RepoBinding Validation

- `repoOrg`: Must match `^[a-z0-9-]+$`, must be in approved list
- `repoName`: Must match `^[a-z0-9-]+$`
- `tenantName`: Must match `^[a-z0-9-]+$`, cannot be privileged namespace
- `permissionProfile`: Must be "standard" or "elevated"

### Namespace Validation

- Name must be valid DNS label (lowercase alphanumeric and hyphens)
- Name cannot be privileged (kube-system, pipeline-system, tekton-pipelines, etc.)
- Name must be unique in cluster

### Allowlist Validation

- Organization must be string
- Repository name must be string
- Tenant must be valid namespace name
- Enabled must be boolean (default: true)

### Pipeline Parameters Validation

- `repoUrl`: Must be valid Git URL
- `commitSha`: Must be valid Git commit hash (40 hex characters)
- `tenantName`: Must be existing namespace
- `branch`: Must be string

**Source**
- `.kiro/specs/jenkinsx-platform/design.md`
- `.kiro/specs/jenkinsx-platform/requirements.md`
- `platform/crds/README.md`
- `platform/onboarding/README.md`
- `platform/tenancy/README.md`
