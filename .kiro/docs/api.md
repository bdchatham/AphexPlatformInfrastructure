# API

## Overview

The Arbiter Pipeline Infrastructure provides a Kubernetes-native API for repository onboarding through Custom Resource Definitions (CRDs). Users interact with the platform by creating RepoBinding resources, which trigger automated provisioning of tenant infrastructure.

## RepoBinding API

### RepoBinding Custom Resource

The primary API for onboarding repositories to the platform.

**API Group**: `platform.arbiter.io`  
**API Version**: `v1alpha1`  
**Kind**: `RepoBinding`  
**Scope**: Namespaced (must be created in `pipeline-system` namespace)

### RepoBinding Spec

```yaml
apiVersion: platform.arbiter.io/v1alpha1
kind: RepoBinding
metadata:
  name: <binding-name>
  namespace: pipeline-system
spec:
  repoOrg: <string>              # Required: GitHub organization
  repoName: <string>             # Required: Repository name
  tenantName: <string>           # Required: Tenant namespace name
  permissionProfile: <string>    # Optional: "standard" or "elevated" (default: "standard")
```

**Field Descriptions**:

| Field | Type | Required | Description | Validation |
|-------|------|----------|-------------|------------|
| `repoOrg` | string | Yes | GitHub organization name | Must match pattern `^[a-z0-9-]+$` |
| `repoName` | string | Yes | Repository name | Must match pattern `^[a-z0-9-]+$` |
| `tenantName` | string | Yes | Tenant namespace name | Must match pattern `^[a-z0-9-]+$`, cannot be privileged namespace |
| `permissionProfile` | string | No | Permission level | Must be "standard" or "elevated" (default: "standard") |

**Validation Rules**:
- `repoOrg` must be in the approved organization list
- `tenantName` cannot be a privileged namespace (kube-system, pipeline-system, etc.)
- `permissionProfile` must be one of the predefined profiles

### RepoBinding Status

The onboarding controller updates the status to reflect provisioning progress.

```yaml
status:
  phase: <string>                    # "Pending" | "Provisioning" | "Ready" | "Failed"
  message: <string>                  # Human-readable status message
  namespaceCreated: <boolean>        # Whether tenant namespace was created
  serviceAccountCreated: <boolean>   # Whether service account was created
  rbacConfigured: <boolean>          # Whether RBAC was configured
  allowlistUpdated: <boolean>        # Whether allowlist was updated
  lastReconcileTime: <string>        # ISO 8601 timestamp of last reconciliation
```

**Phase Values**:
- `Pending`: RepoBinding created, waiting for reconciliation
- `Provisioning`: Onboarding controller is provisioning resources
- `Ready`: All resources provisioned successfully
- `Failed`: Provisioning failed (see message for details)

### Usage Examples

**Example 1: Standard Onboarding**

```yaml
apiVersion: platform.arbiter.io/v1alpha1
kind: RepoBinding
metadata:
  name: archon-binding
  namespace: pipeline-system
spec:
  repoOrg: "your-github-org"
  repoName: "archon-agent"
  tenantName: "archon"
  permissionProfile: "standard"
```

**Example 2: Elevated Permissions**

```yaml
apiVersion: platform.arbiter.io/v1alpha1
kind: RepoBinding
metadata:
  name: infrastructure-binding
  namespace: pipeline-system
spec:
  repoOrg: "your-github-org"
  repoName: "infrastructure-repo"
  tenantName: "infrastructure"
  permissionProfile: "elevated"
```

**Example 3: Check Status**

```bash
# Get RepoBinding status
kubectl get repobinding archon-binding -n pipeline-system

# Get detailed status
kubectl describe repobinding archon-binding -n pipeline-system

# Get status as YAML
kubectl get repobinding archon-binding -n pipeline-system -o yaml
```

**Example Status Output**:

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

## Repository Allowlist API

The allowlist is managed through a ConfigMap in the `pipeline-system` namespace.

### Allowlist ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: repo-allowlist
  namespace: pipeline-system
data:
  allowlist.yaml: |
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

**Field Descriptions**:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `org` | string | Yes | GitHub organization |
| `name` | string | Yes | Repository name |
| `tenant` | string | Yes | Tenant namespace |
| `enabled` | boolean | No | Whether triggers are active (default: true) |

**Usage**:

```bash
# View allowlist
kubectl get configmap repo-allowlist -n pipeline-system -o yaml

# Edit allowlist (manual - not recommended)
kubectl edit configmap repo-allowlist -n pipeline-system

# Lighthouse automatically reloads on ConfigMap changes
```

**Note**: The onboarding controller automatically updates the allowlist when RepoBinding resources are created. Manual editing is not recommended.

## Lighthouse Configuration API

Lighthouse configuration is managed through a ConfigMap.

### Lighthouse ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: lighthouse-config
  namespace: pipeline-system
data:
  config.yaml: |
    github:
      app_id: "${GITHUB_APP_ID}"
      app_installation_id: "${GITHUB_APP_INSTALLATION_ID}"
    allowlist:
      - org: "your-github-org"
        repos:
          - "archon-agent"
          - "another-repo"
```

**Configuration Fields**:

| Field | Type | Description |
|-------|------|-------------|
| `github.app_id` | string | GitHub App ID |
| `github.app_installation_id` | string | GitHub App Installation ID |
| `allowlist` | array | List of allowed organizations and repositories |

## Tenant Resources API

When a RepoBinding is created, the onboarding controller provisions these Kubernetes resources:

### Namespace

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: ${TENANT_NAME}
  labels:
    platform.arbiter.io/tenant: "${TENANT_NAME}"
    platform.arbiter.io/repo: "${REPO_ORG}/${REPO_NAME}"
    platform.arbiter.io/managed-by: "onboarding-controller"
```

### Service Account

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: pipeline-runner
  namespace: ${TENANT_NAME}
```

### Role (Standard Profile)

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: pipeline-runner
  namespace: ${TENANT_NAME}
rules:
  - apiGroups: [""]
    resources: ["pods", "pods/log", "configmaps", "secrets"]
    verbs: ["get", "list", "create", "update", "delete"]
  - apiGroups: ["tekton.dev"]
    resources: ["pipelineruns", "taskruns"]
    verbs: ["get", "list", "create"]
```

### Role (Elevated Profile)

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: pipeline-runner
  namespace: ${TENANT_NAME}
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

### RoleBinding

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: pipeline-runner
  namespace: ${TENANT_NAME}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: pipeline-runner
subjects:
  - kind: ServiceAccount
    name: pipeline-runner
    namespace: ${TENANT_NAME}
```

### ResourceQuota

```yaml
apiVersion: v1
kind: ResourceQuota
metadata:
  name: tenant-quota
  namespace: ${TENANT_NAME}
spec:
  hard:
    requests.cpu: "4"
    requests.memory: "8Gi"
    limits.cpu: "8"
    limits.memory: "16Gi"
    persistentvolumeclaims: "5"
    pods: "20"
```

### LimitRange

```yaml
apiVersion: v1
kind: LimitRange
metadata:
  name: tenant-limits
  namespace: ${TENANT_NAME}
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

### NetworkPolicy

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: tenant-isolation
  namespace: ${TENANT_NAME}
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

## Pipeline Catalog API

Tenants reference shared Tekton Tasks and Pipelines from the `pipeline-catalog` namespace.

### Referencing Catalog Tasks

```yaml
apiVersion: tekton.dev/v1beta1
kind: PipelineRun
metadata:
  name: my-deployment
  namespace: my-tenant
spec:
  pipelineRef:
    name: cdktf-deploy-pipeline
    namespace: pipeline-catalog
  params:
    - name: repo-url
      value: "https://github.com/org/repo"
    - name: commit-sha
      value: "abc123..."
    - name: tenant-name
      value: "my-tenant"
```

### Available Catalog Tasks

| Task Name | Description | Parameters |
|-----------|-------------|------------|
| `git-clone` | Clone repository at commit SHA | `url`, `revision` |
| `cdktf-synth` | Run cdktf synth | None |
| `cdktf-deploy` | Run cdktf deploy with remote state | `tenant-name` |
| `upload-artifacts` | Upload logs/outputs to external storage | `bucket`, `key` |

### Available Catalog Pipelines

| Pipeline Name | Description | Parameters |
|---------------|-------------|------------|
| `cdktf-deploy-pipeline` | Complete CDKTF deployment workflow | `repo-url`, `commit-sha`, `tenant-name` |

## Error Responses

### RepoBinding Validation Errors

**Invalid Organization**:
```yaml
status:
  phase: Failed
  message: "Repository organization not in approved list"
  namespaceCreated: false
  serviceAccountCreated: false
  rbacConfigured: false
  allowlistUpdated: false
```

**Invalid Namespace Pattern**:
```yaml
status:
  phase: Failed
  message: "Namespace name must match pattern ^[a-z0-9-]+$"
  namespaceCreated: false
  serviceAccountCreated: false
  rbacConfigured: false
  allowlistUpdated: false
```

**Privileged Namespace**:
```yaml
status:
  phase: Failed
  message: "Cannot create namespace with privileged name"
  namespaceCreated: false
  serviceAccountCreated: false
  rbacConfigured: false
  allowlistUpdated: false
```

### Lighthouse Event Rejections

**Repository Not in Allowlist**:
```json
{
  "level": "warn",
  "msg": "Repository not in allowlist",
  "repo": "org/unauthorized-repo",
  "event": "push",
  "timestamp": "2024-12-31T10:00:00Z"
}
```

**Invalid Webhook Signature**:
```json
{
  "level": "error",
  "msg": "Invalid webhook signature",
  "repo": "org/repo",
  "event": "push",
  "timestamp": "2024-12-31T10:00:00Z"
}
```

## Authentication

### OIDC Authentication

Users authenticate via OIDC to create RepoBinding resources.

**kubectl Configuration**:

```bash
# Configure kubectl with OIDC
kubectl config set-credentials oidc \
  --exec-api-version=client.authentication.k8s.io/v1beta1 \
  --exec-command=kubectl \
  --exec-arg=oidc-login \
  --exec-arg=get-token \
  --exec-arg=--oidc-issuer-url=https://dex.homelab.local \
  --exec-arg=--oidc-client-id=kubernetes \
  --exec-arg=--oidc-client-secret=kubernetes-client-secret

# Use OIDC credentials
kubectl config set-context --current --user=oidc
```

**RBAC for Onboarding**:

Users in the `engineering` OIDC group can create RepoBinding resources:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: repo-onboarder
rules:
  - apiGroups: ["platform.arbiter.io"]
    resources: ["repobindings"]
    verbs: ["create", "get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: engineering-onboarders
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: repo-onboarder
subjects:
  - kind: Group
    name: "system:authenticated:engineering"
    apiGroup: rbac.authorization.k8s.io
```

**Source**
- `.kiro/specs/jenkinsx-platform/design.md`
- `.kiro/specs/jenkinsx-platform/requirements.md`
- `platform/crds/README.md`
- `platform/onboarding/README.md`
- `platform/lighthouse/README.md`
