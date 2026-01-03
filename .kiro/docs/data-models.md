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

**Source**
- `.kiro/specs/argocd-tekton-platform/design.md`
- `.kiro/specs/argocd-tekton-platform/requirements.md`
- `platform/crds/repobinding-crd.yaml`
- `platform/onboarding/controller/`
- `platform/tenancy/templates/`
- `platform/argocd/apps/`
