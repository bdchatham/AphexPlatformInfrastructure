# API

## Overview

The Arbiter Pipeline Infrastructure provides a Kubernetes-native API for repository onboarding through Custom Resource Definitions (CRDs). Users interact with the platform by creating RepoBinding resources, which trigger automated provisioning of tenant infrastructure.

## RepoBinding API

### RepoBinding Custom Resource

The primary API for onboarding repositories to the platform.

**API Group**: `arbiter.io`  
**API Version**: `v1alpha1`  
**Kind**: `RepoBinding`  
**Scope**: Namespaced (must be created in `platform-system` namespace)

### RepoBinding Spec

```yaml
apiVersion: arbiter.io/v1alpha1
kind: RepoBinding
metadata:
  name: <binding-name>
  namespace: platform-system
spec:
  repoOrg: <string>              # Required: GitHub organization
  repoName: <string>             # Required: Repository name
  tenantName: <string>           # Required: Tenant namespace name
  permissionProfile: <string>    # Optional: "standard" or "elevated" (default: "standard")
  ingressHost: <string>          # Optional: Ingress hostname for webhooks
```

**Field Descriptions**:

| Field | Type | Required | Description | Validation |
|-------|------|----------|-------------|------------|
| `repoOrg` | string | Yes | GitHub organization name | Must match pattern `^[a-z0-9-]+$` |
| `repoName` | string | Yes | Repository name | Must match pattern `^[a-z0-9-]+$` |
| `tenantName` | string | Yes | Tenant namespace name | Must match pattern `^[a-z0-9-]+$`, cannot be privileged namespace |
| `permissionProfile` | string | No | Permission level | Must be "standard" or "elevated" (default: "standard") |
| `ingressHost` | string | No | Ingress hostname | Valid hostname format |

**Validation Rules**:
- `tenantName` cannot be a privileged namespace (kube-system, platform-system, argocd, tekton-pipelines, etc.)
- `permissionProfile` must be one of the predefined profiles

### RepoBinding Status

The onboarding controller updates the status to reflect provisioning progress.

```yaml
status:
  phase: <string>                    # "Pending" | "Provisioning" | "Ready" | "Failed"
  message: <string>                  # Human-readable status message
  webhookURL: <string>               # Webhook URL for GitHub configuration
  webhookSecret: <string>            # Webhook secret for GitHub configuration
  namespaceCreated: <boolean>        # Whether tenant namespace was created
  serviceAccountCreated: <boolean>   # Whether service account was created
  rbacCreated: <boolean>             # Whether RBAC was configured
  quotasCreated: <boolean>           # Whether resource quotas were created
  networkPolicyCreated: <boolean>    # Whether network policy was created
  terraformSecretCreated: <boolean>  # Whether Terraform secret was created
  eventListenerCreated: <boolean>    # Whether EventListener was created
  ingressCreated: <boolean>          # Whether Ingress was created
```

**Phase Values**:
- `Pending`: RepoBinding created, waiting for reconciliation
- `Provisioning`: Onboarding controller is provisioning resources
- `Ready`: All resources provisioned successfully
- `Failed`: Provisioning failed (see message for details)

### Usage Examples

**Example 1: Standard Onboarding**

```yaml
apiVersion: arbiter.io/v1alpha1
kind: RepoBinding
metadata:
  name: archon-binding
  namespace: platform-system
spec:
  repoOrg: "your-github-org"
  repoName: "archon-agent"
  tenantName: "archon"
  permissionProfile: "standard"
  ingressHost: "webhooks.example.com"
```

**Example 2: Elevated Permissions**

```yaml
apiVersion: arbiter.io/v1alpha1
kind: RepoBinding
metadata:
  name: infrastructure-binding
  namespace: platform-system
spec:
  repoOrg: "your-github-org"
  repoName: "infrastructure-repo"
  tenantName: "infrastructure"
  permissionProfile: "elevated"
  ingressHost: "webhooks.example.com"
```

**Example 3: Check Status**

```bash
# Get RepoBinding status
kubectl get repobinding archon-binding -n platform-system

# Get detailed status
kubectl describe repobinding archon-binding -n platform-system

# Get status as YAML
kubectl get repobinding archon-binding -n platform-system -o yaml
```

**Example Status Output**:

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

## EventListener Webhook API

Each tenant gets a dedicated Tekton EventListener that receives GitHub webhooks.

### Webhook Endpoint

**URL Format**: `https://<ingressHost>/<tenantName>`

**Example**: `https://webhooks.example.com/archon`

### Webhook Request

**Method**: POST

**Headers**:
- `Content-Type: application/json`
- `X-GitHub-Event: push`
- `X-Hub-Signature-256: sha256=<signature>`

**Body** (GitHub Push Event):
```json
{
  "ref": "refs/heads/main",
  "after": "abc123def456...",
  "repository": {
    "clone_url": "https://github.com/org/repo.git",
    "name": "repo",
    "full_name": "org/repo"
  },
  "pusher": {
    "name": "username"
  }
}
```

### Webhook Response

**Success (200 OK)**:
```json
{
  "eventListener": "github-listener",
  "namespace": "archon",
  "eventID": "abc123"
}
```

**Unauthorized (401 Unauthorized)**:
```json
{
  "error": "Invalid webhook signature"
}
```

**Bad Request (400 Bad Request)**:
```json
{
  "error": "Invalid webhook payload"
}
```

### Webhook Signature Validation

EventListeners validate webhook signatures using HMAC-SHA256:

```
signature = HMAC-SHA256(secret, payload)
X-Hub-Signature-256 = "sha256=" + hex(signature)
```

The webhook secret is stored in a Kubernetes Secret and referenced by the EventListener.

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

### ClusterRole (Tekton Triggers Resources)

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: pipeline-runner-${TENANT_NAME}
  labels:
    platform.arbiter.io/tenant: "${TENANT_NAME}"
    platform.arbiter.io/managed-by: "onboarding-controller"
rules:
  - apiGroups: ["triggers.tekton.dev"]
    resources: ["clusterinterceptors", "clustertriggerbindings"]
    verbs: ["get", "list", "watch"]
```

**Purpose**: Grants read-only access to cluster-scoped Tekton Triggers resources. Required for EventListener pods to validate webhooks and create PipelineRuns.

### ClusterRoleBinding (Tekton Triggers Resources)

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: pipeline-runner-${TENANT_NAME}
  labels:
    platform.arbiter.io/tenant: "${TENANT_NAME}"
    platform.arbiter.io/managed-by: "onboarding-controller"
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: pipeline-runner-${TENANT_NAME}
subjects:
  - kind: ServiceAccount
    name: pipeline-runner
    namespace: ${TENANT_NAME}
```

**Purpose**: Binds the ClusterRole to the tenant's pipeline-runner ServiceAccount, granting cluster-scoped read permissions for Tekton Triggers resources.

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

### EventListener

```yaml
apiVersion: triggers.tekton.dev/v1beta1
kind: EventListener
metadata:
  name: github-listener
  namespace: ${TENANT_NAME}
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
                secretName: webhook-${TENANT_NAME}
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

### Ingress

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: github-webhook
  namespace: ${TENANT_NAME}
spec:
  rules:
    - host: ${INGRESS_HOST}
      http:
        paths:
          - path: /${TENANT_NAME}
            pathType: Prefix
            backend:
              service:
                name: el-github-listener
                port:
                  number: 8080
```

## ArgoCD Application API

The platform uses ArgoCD Applications to manage components via GitOps.

### Application Spec

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: platform-root
  namespace: argocd
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

```yaml
status:
  sync:
    status: Synced  # Synced | OutOfSync | Unknown
    revision: abc123def456
  health:
    status: Healthy  # Healthy | Progressing | Degraded | Suspended | Missing | Unknown
  conditions:
    - type: ComparisonError
      status: "False"
      message: ""
```

## Error Responses

### RepoBinding Validation Errors

**Invalid Namespace Pattern**:
```yaml
status:
  phase: Failed
  message: "Namespace name must match pattern ^[a-z0-9-]+$"
  namespaceCreated: false
  serviceAccountCreated: false
  rbacCreated: false
  quotasCreated: false
  networkPolicyCreated: false
  terraformSecretCreated: false
  eventListenerCreated: false
  ingressCreated: false
```

**Privileged Namespace**:
```yaml
status:
  phase: Failed
  message: "Cannot create namespace with privileged name"
  namespaceCreated: false
  serviceAccountCreated: false
  rbacCreated: false
  quotasCreated: false
  networkPolicyCreated: false
  terraformSecretCreated: false
  eventListenerCreated: false
  ingressCreated: false
```

### EventListener Webhook Rejections

**Invalid Webhook Signature**:
```json
{
  "level": "error",
  "msg": "Invalid webhook signature",
  "eventListener": "github-listener",
  "namespace": "archon",
  "timestamp": "2024-12-31T10:00:00Z"
}
```

**Invalid Event Type**:
```json
{
  "level": "warn",
  "msg": "Event type not supported",
  "eventType": "pull_request",
  "eventListener": "github-listener",
  "namespace": "archon",
  "timestamp": "2024-12-31T10:00:00Z"
}
```

**Branch Filter Not Matched**:
```json
{
  "level": "info",
  "msg": "Branch filter not matched",
  "ref": "refs/heads/feature-branch",
  "eventListener": "github-listener",
  "namespace": "archon",
  "timestamp": "2024-12-31T10:00:00Z"
}
```

**Source**
- `.kiro/specs/argocd-tekton-platform/design.md`
- `.kiro/specs/argocd-tekton-platform/requirements.md`
- `platform/crds/repobinding-crd.yaml`
- `platform/onboarding/controller/`
- `platform/tenancy/templates/`
- `platform/argocd/apps/`
