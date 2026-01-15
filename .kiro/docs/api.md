# API

## Overview

The Arbiter Pipeline Infrastructure provides a Kubernetes-native API for repository onboarding through Custom Resource Definitions (CRDs). Users interact with the platform by creating RepoBinding resources, which trigger automated provisioning of tenant infrastructure with complete isolation and security boundaries.

## Kubernetes API OIDC Authentication

The platform provides OIDC authentication for the Kubernetes API server, enabling kubectl and other Kubernetes clients to authenticate using the centralized Authentik identity provider via Dex.

### Configuration

**API Server OIDC Settings** (configured in Kind cluster):
- **Issuer URL**: `https://dex.home.local`
- **Client ID**: `kubernetes`
- **Username Claim**: `email`
- **Groups Claim**: `groups`

**Dex Client Configuration**:
- **Client ID**: `kubernetes`
- **Redirect URIs**: `http://localhost:8000/callback`, `http://localhost:18000/callback`
- **Authentication Flow**: Authentik → Dex → Kubernetes API

### Usage

Users authenticate to the Kubernetes API using the kubectl OIDC plugin:

```bash
# Install kubectl OIDC plugin
kubectl krew install oidc-login

# Configure OIDC authentication
kubectl oidc-login setup \
  --oidc-issuer-url=https://dex.home.local \
  --oidc-client-id=kubernetes

# Authenticate and access cluster
kubectl get pods --user=oidc
```

**Authentication Flow**:
1. User runs kubectl command with OIDC authentication
2. kubectl opens browser to Dex login page
3. Dex redirects to Authentik for authentication
4. User authenticates with Authentik credentials
5. Authentik returns to Dex with user info and groups
6. Dex issues JWT token to kubectl
7. kubectl uses JWT for Kubernetes API requests
8. API server validates JWT and maps groups to RBAC roles

**Source**: `platform/bootstrap/kind-cluster-config.yaml`, `platform/auth/dex/configmap.yaml`

## Organization API

### Organization Custom Resource

The primary API for creating multi-tenant organizations with isolated namespaces, public webhook endpoints, and Cloudflare tunnel integration.

**API Group**: `arbiter.io`  
**API Version**: `v1alpha1`  
**Kind**: `Organization`  
**Scope**: Namespaced (must be created in `platform-system`)

### Create Organization

```yaml
apiVersion: arbiter.io/v1alpha1
kind: Organization
metadata:
  name: acme-corp
  namespace: platform-system
spec:
  displayName: "ACME Corporation"
  adminUsers:
    - admin@acme-corp.com
  webhookSecret: ""  # Auto-generated if empty
```

**Behavior**:
1. Creates namespace: `org-acme-corp`
2. Creates Cloudflare tunnel via API
3. Creates DNS CNAME: `acme-corp.arbiter-dev.com → {tunnel-id}.cfargotunnel.com`
4. Deploys cloudflared tunnel pod
5. Creates EventListener with dedicated ServiceAccount and ClusterRoleBinding
6. Creates organization admin RBAC
7. Updates status with webhook URL

**Status Fields**:
- `namespace`: Organization namespace name
- `webhookURL`: Public webhook endpoint (e.g., `https://acme-corp.arbiter-dev.com`)
- `phase`: `Pending`, `Active`, or `Failed`

**Source**: `platform/crds/organization-crd.yaml`, `platform/onboarding/controller/controllers/organization_controller.go`

### Delete Organization

```bash
kubectl delete organization acme-corp -n platform-system
```

**Behavior**:
1. Deletes DNS CNAME record from Cloudflare
2. Cleans up tunnel connections via Cloudflare API
3. Deletes tunnel from Cloudflare
4. Deletes ClusterRoleBinding for EventListener
5. Deletes organization namespace (cascades all resources)

**Source**: `platform/onboarding/controller/controllers/organization_controller.go`

## RepoBinding API

### RepoBinding Custom Resource

The primary API for onboarding repositories to the platform with automated tenant provisioning.

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
  aphexOrg: <string>             # Required: Organization name (maps to org-{aphexOrg} namespace)
  repoOrg: <string>              # Required: GitHub organization
  repoName: <string>             # Required: Repository name
  pipelineName: <string>         # Required: Tekton Pipeline name to trigger
  templateRef: <string>          # Required: Dispatcher template (e.g., run-pipeline-v1)
  pipelineSpec: <string>         # Required: Raw YAML content of Tekton Pipeline to create
```

**Field Descriptions**:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `aphexOrg` | string | Yes | Organization name (maps to `org-{aphexOrg}` namespace) |
| `repoOrg` | string | Yes | GitHub organization name |
| `repoName` | string | Yes | Repository name |
| `pipelineName` | string | Yes | Tekton Pipeline name to trigger |
| `templateRef` | string | Yes | Dispatcher template name (e.g., `run-pipeline-v1`) |
| `pipelineSpec` | string | Yes | Raw YAML content of Tekton Pipeline resource to create |

**Validation Rules**:
- `aphexOrg` must reference an existing Organization resource
- `pipelineSpec` must contain valid Tekton Pipeline YAML (kind: Pipeline, apiVersion: tekton.dev/v1 or tekton.dev/v1beta1)
- `pipelineName` must reference an existing Tekton Pipeline
- `templateRef` must reference an existing dispatcher template (e.g., `run-pipeline-v1`)

For data model details, see [data-models.md](data-models.md#repobinding-data-model).

**Source**: `platform/crds/repobinding-crd.yaml`, `platform/onboarding/controller/controllers/repobinding_controller.go`

### RepoBinding Status

The onboarding controller updates the status to reflect provisioning progress and provide GitHub webhook configuration details.

```yaml
status:
  phase: <string>                    # "Pending" | "Provisioning" | "Ready" | "Failed"
  message: <string>                  # Human-readable status message
  conditions:                        # Detailed condition tracking
  - type: <string>                   # Condition type
    status: <string>                 # "True" | "False" | "Unknown"
    reason: <string>                 # Machine-readable reason
    message: <string>                # Human-readable message
    lastTransitionTime: <timestamp>  # When condition last changed
  webhookConfiguration:              # GitHub webhook setup information
    url: <string>                    # Webhook URL for GitHub configuration
    secret: <string>                 # Webhook secret for GitHub configuration
    events: <[]string>               # Supported webhook events
  provisionedResources:              # Status of provisioned resources
    namespace: <boolean>             # Tenant namespace created
    serviceAccount: <boolean>        # Service account created
    rbac: <boolean>                  # RBAC policies created
    resourceQuota: <boolean>         # Resource quotas created
    networkPolicy: <boolean>         # Network policies created
    eventListener: <boolean>         # Tekton EventListener created
    pipelineResources: <boolean>     # Pipeline resources created
```

**Phase Values**:
- `Pending`: RepoBinding created, validation in progress
- `Provisioning`: Controller is creating tenant resources
- `Ready`: All resources provisioned successfully, webhook ready
- `Failed`: Provisioning failed (check conditions for details)

**Condition Types**:
- `NamespaceReady`: Tenant namespace created and configured
- `RBACReady`: Service account and RBAC policies configured
- `NetworkPolicyReady`: Network isolation policies applied
- `EventListenerReady`: Tekton webhook handler configured
- `WebhookReady`: GitHub webhook configuration available

### Usage Examples

**Example 1: Standard Repository Onboarding**

```yaml
apiVersion: platform.arbiter.io/v1alpha1
kind: RepoBinding
metadata:
  name: my-app-binding
  namespace: platform-system
spec:
  repoOrg: "acme-corp"
  repoName: "my-application"
  tenantName: "my-app"
  permissionProfile: "standard"
```

**Example 2: Infrastructure Repository with Elevated Permissions**

```yaml
apiVersion: platform.arbiter.io/v1alpha1
kind: RepoBinding
metadata:
  name: infrastructure-binding
  namespace: platform-system
spec:
  repoOrg: "acme-corp"
  repoName: "infrastructure-as-code"
  tenantName: "infrastructure"
  permissionProfile: "elevated"
```

### Permission Profiles

**Standard Profile**:
- Namespace-scoped RBAC permissions
- Standard resource quotas (CPU: 2 cores, Memory: 4Gi, Storage: 10Gi)
- Network policies allowing ingress from ingress-system only
- Access to shared pipeline catalog
- EventListener for webhook handling

**Elevated Profile**:
- Additional cluster-scoped read permissions for infrastructure resources
- Higher resource quotas (CPU: 4 cores, Memory: 8Gi, Storage: 20Gi)
- Additional network policy exceptions for infrastructure access
- Access to infrastructure-specific pipeline tasks

### GitHub Webhook Configuration

After RepoBinding reaches `Ready` phase, configure the GitHub webhook:

**Step 1: Get Webhook Configuration**
```bash
kubectl get repobinding my-app-binding -n platform-system -o yaml
```

**Step 2: Configure in GitHub Repository**
1. Navigate to repository Settings → Webhooks → Add webhook
2. **Payload URL**: Use `status.webhookConfiguration.url`
3. **Content type**: `application/json`
4. **Secret**: Use `status.webhookConfiguration.secret`
5. **Events**: Select "Push events" (or use `status.webhookConfiguration.events`)
6. **Active**: ✓ Enabled

**Step 3: Verify Webhook**
```bash
# Check EventListener logs
kubectl logs -n <tenant-name> -l app.kubernetes.io/component=eventlistener

# Test webhook delivery in GitHub repository settings
```

## Onboarding Controller API

### Controller Behavior

The onboarding controller watches RepoBinding resources and provisions tenant infrastructure automatically.

**Reconciliation Process**:
1. **Validation**: Validates RepoBinding spec and checks Organization exists
2. **Namespace Creation**: Creates pipeline namespace (`{pipelineName}`) with proper labels
3. **Pipeline Creation**: Parses `pipelineSpec` and creates Tekton Pipeline resource in pipeline namespace
4. **RBAC Setup**: Creates ServiceAccount, Role, RoleBinding, ClusterRole, and ClusterRoleBinding
5. **Resource Quotas**: Applies resource limits (CPU, memory, pods, PVCs)
6. **Network Policies**: Configures network isolation with egress to internet and DNS
7. **Terraform Secret**: Creates Terraform backend configuration secret
8. **EventListener Update**: Adds pipeline namespace to organization EventListener's namespace selector
9. **TriggerTemplate**: Materializes dispatcher template from catalog in organization namespace
10. **Trigger**: Creates Trigger resource in organization namespace linking webhook to pipeline
11. **Webhook Configuration**: Updates status with webhook URL and secret
12. **Status Updates**: Updates RepoBinding status with provisioning progress

**Error Handling**:
- Validation errors result in `Failed` phase with descriptive error messages
- Transient errors trigger exponential backoff retry
- Resource conflicts are detected and resolved automatically
- Orphaned resources are cleaned up on RepoBinding deletion

**Source**
- `platform/crds/repobinding-crd.yaml` - RepoBinding CRD definition
- `platform/onboarding/controller/api/v1alpha1/repobinding_types.go` - RepoBinding Go types
- `platform/onboarding/controller/controllers/repobinding_controller.go` - Controller reconciliation logic
- `platform/onboarding/controller/controllers/repobinding_provisioners.go` - Resource provisioning functions

## Platform Services API

### ArgoCD Integration

**OIDC Authentication**:
- **Issuer URL**: `https://dex.home.local`
- **Client ID**: `argocd`
- **Scopes**: `openid profile email groups`
- **Group Claims**: Maps Authentik groups to ArgoCD roles

**Access Patterns**:
- **Admin Group**: Full access to all ArgoCD applications and settings
- **Engineering Group**: Read-only access to applications, no admin functions
- **Tenant-specific**: Future enhancement for tenant-scoped access

### Tekton Dashboard Integration

**OIDC Authentication**:
- **Issuer URL**: `https://dex.home.local`
- **Client ID**: `tekton-dashboard`
- **Scopes**: `openid profile email groups`
- **Group Claims**: Maps Authentik groups to Kubernetes RBAC

**Access Patterns**:
- **Admin Group**: Full access to all pipelines and resources
- **Engineering Group**: Read-only access to pipeline runs and logs
- **Tenant-scoped**: Access limited to tenant namespace resources

### Authentik Management API

**Admin Access**:
- **URL**: `https://auth.home.local`
- **Admin User**: `admin` (password from bootstrap)
- **API Access**: Token-based API for automation

**User Management**:
- **Groups**: `admins`, `engineering` with different platform permissions
- **External Providers**: GitHub OAuth, LDAP, SAML integration support
- **Self-Service**: Users can update profiles and passwords

**Source**
- `platform/auth/authentik/` - Authentik configuration and blueprints
- `platform/auth/dex/` - Dex OIDC connector configuration
- `platform/auth/ingress/` - Ingress resources for authentication services
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

## Authentication API

The authentication system provides OIDC-based authentication for platform services through Authentik (IdP) and Dex (OIDC connector).

### Authentik API

Authentik provides a REST API for managing users, groups, and OIDC providers.

#### Base URL

**Internal (pod-to-pod)**: `http://authentik.auth-system.svc.cluster.local:9000`  
**External (browser)**: `https://auth.home.local`

#### Authentication

All API requests require authentication via Bearer token:

```
Authorization: Bearer <api-token>
```

API tokens are created via Authentik UI or API and stored in Kubernetes Secrets.

#### Core Endpoints

**Health Check**

```
GET /api/v3/root/config/
```

Returns Authentik configuration and health status.

**Response (200 OK)**:
```json
{
  "branding_logo": "/static/dist/assets/icons/icon_left_brand.svg",
  "branding_title": "authentik",
  "ui_footer_links": [],
  "ui_theme": "automatic",
  "cache_timeout": 300,
  "cache_timeout_flows": 300,
  "cache_timeout_policies": 300,
  "cache_timeout_reputation": 300
}
```

**List Users**

```
GET /api/v3/core/users/
```

Returns list of all users.

**Response (200 OK)**:
```json
{
  "pagination": {
    "next": 0,
    "previous": 0,
    "count": 2,
    "current": 1,
    "total_pages": 1,
    "start_index": 1,
    "end_index": 2
  },
  "results": [
    {
      "pk": 1,
      "username": "admin",
      "name": "Admin User",
      "is_active": true,
      "last_login": "2024-01-05T10:00:00Z",
      "email": "admin@example.com",
      "groups": ["admins"]
    }
  ]
}
```

**List Groups**

```
GET /api/v3/core/groups/
```

Returns list of all groups.

**Response (200 OK)**:
```json
{
  "pagination": {
    "next": 0,
    "previous": 0,
    "count": 2,
    "current": 1,
    "total_pages": 1,
    "start_index": 1,
    "end_index": 2
  },
  "results": [
    {
      "pk": "abc123",
      "name": "admins",
      "is_superuser": false,
      "parent": null,
      "users": [1],
      "attributes": {}
    },
    {
      "pk": "def456",
      "name": "engineering",
      "is_superuser": false,
      "parent": null,
      "users": [],
      "attributes": {}
    }
  ]
}
```

**List OAuth2 Providers**

```
GET /api/v3/providers/oauth2/
```

Returns list of all OAuth2/OIDC providers.

**Response (200 OK)**:
```json
{
  "pagination": {
    "next": 0,
    "previous": 0,
    "count": 1,
    "current": 1,
    "total_pages": 1,
    "start_index": 1,
    "end_index": 1
  },
  "results": [
    {
      "pk": 1,
      "name": "Dex OIDC Provider",
      "authorization_flow": "abc123",
      "client_type": "confidential",
      "client_id": "dex-client",
      "client_secret": "***",
      "redirect_uris": "https://dex.home.local/callback",
      "signing_key": "def456"
    }
  ]
}
```

**Update OAuth2 Provider**

```
PATCH /api/v3/providers/oauth2/{id}/
Content-Type: application/json
```

**Request Body**:
```json
{
  "client_secret": "new-secret-value"
}
```

**Response (200 OK)**:
```json
{
  "pk": 1,
  "name": "Dex OIDC Provider",
  "authorization_flow": "abc123",
  "client_type": "confidential",
  "client_id": "dex-client",
  "client_secret": "***",
  "redirect_uris": "https://dex.home.local/callback",
  "signing_key": "def456"
}
```

#### OIDC Discovery Endpoint

**Authentik OIDC Discovery**

```
GET /application/o/dex/.well-known/openid-configuration
```

Returns OIDC discovery metadata for the Dex application.

**Response (200 OK)**:
```json
{
  "issuer": "https://auth.home.local/application/o/dex/",
  "authorization_endpoint": "https://auth.home.local/application/o/authorize/",
  "token_endpoint": "https://auth.home.local/application/o/token/",
  "userinfo_endpoint": "https://auth.home.local/application/o/userinfo/",
  "end_session_endpoint": "https://auth.home.local/application/o/dex/end-session/",
  "jwks_uri": "https://auth.home.local/application/o/dex/jwks/",
  "response_types_supported": ["code", "id_token", "id_token token", "code token"],
  "subject_types_supported": ["public"],
  "id_token_signing_alg_values_supported": ["RS256"],
  "scopes_supported": ["openid", "profile", "email", "groups"],
  "token_endpoint_auth_methods_supported": ["client_secret_basic", "client_secret_post"],
  "claims_supported": ["sub", "iss", "aud", "exp", "iat", "name", "email", "groups"]
}
```

### Dex API

Dex provides OIDC connector functionality between Authentik and platform services.

#### Base URL

**Internal (pod-to-pod)**: `http://dex.auth-system.svc.cluster.local:5556`  
**External (browser)**: `https://dex.home.local`

#### Health Check

```
GET /healthz
```

Returns Dex health status.

**Response (200 OK)**:
```
OK
```

#### OIDC Discovery Endpoint

```
GET /.well-known/openid-configuration
```

Returns OIDC discovery metadata for Dex.

**Response (200 OK)**:
```json
{
  "issuer": "https://dex.home.local",
  "authorization_endpoint": "https://dex.home.local/auth",
  "token_endpoint": "https://dex.home.local/token",
  "userinfo_endpoint": "https://dex.home.local/userinfo",
  "jwks_uri": "https://dex.home.local/keys",
  "response_types_supported": ["code"],
  "subject_types_supported": ["public"],
  "id_token_signing_alg_values_supported": ["RS256"],
  "scopes_supported": ["openid", "profile", "email", "groups", "offline_access"],
  "token_endpoint_auth_methods_supported": ["client_secret_basic", "client_secret_post"],
  "claims_supported": ["sub", "iss", "aud", "exp", "iat", "name", "email", "groups"]
}
```

#### Authorization Endpoint

```
GET /auth?client_id=<client_id>&redirect_uri=<redirect_uri>&response_type=code&scope=<scopes>&state=<state>
```

Initiates OIDC authorization flow. Redirects to Authentik for authentication.

**Query Parameters**:
- `client_id`: Client ID (e.g., "argocd", "tekton-dashboard")
- `redirect_uri`: Callback URL (e.g., "https://argocd.home.local/auth/callback")
- `response_type`: Must be "code"
- `scope`: Space-separated scopes (e.g., "openid profile email groups")
- `state`: Random state value for CSRF protection

**Response**: HTTP 302 redirect to Authentik login page

#### Token Endpoint

```
POST /token
Content-Type: application/x-www-form-urlencoded
```

Exchanges authorization code for access token and ID token.

**Request Body**:
```
grant_type=authorization_code
&code=<authorization_code>
&redirect_uri=<redirect_uri>
&client_id=<client_id>
&client_secret=<client_secret>
```

**Response (200 OK)**:
```json
{
  "access_token": "eyJhbGciOiJSUzI1NiIsImtpZCI6IjEyMyJ9...",
  "token_type": "Bearer",
  "expires_in": 86400,
  "refresh_token": "eyJhbGciOiJSUzI1NiIsImtpZCI6IjQ1NiJ9...",
  "id_token": "eyJhbGciOiJSUzI1NiIsImtpZCI6Ijc4OSJ9..."
}
```

#### UserInfo Endpoint

```
GET /userinfo
Authorization: Bearer <access_token>
```

Returns user information for the authenticated user.

**Response (200 OK)**:
```json
{
  "sub": "abc123",
  "name": "John Doe",
  "email": "john.doe@example.com",
  "groups": ["admins"]
}
```

### ArgoCD OIDC Configuration

ArgoCD is configured to use Dex as its OIDC provider.

#### OIDC Settings

**Issuer**: `https://dex.home.local`  
**Client ID**: `argocd`  
**Client Secret**: Stored in `argocd-secret` Secret  
**Redirect URI**: `https://argocd.home.local/auth/callback`  
**Scopes**: `openid`, `profile`, `email`, `groups`

#### Group Mapping

ArgoCD maps OIDC groups to ArgoCD roles:

- `admins` group → `role:admin` (full access)
- `engineering` group → `role:readonly` (read-only access)

#### Login Flow

1. User accesses ArgoCD UI at `https://argocd.home.local`
2. User clicks "Login via Dex"
3. ArgoCD redirects to Dex authorization endpoint
4. Dex redirects to Authentik login page
5. User logs in to Authentik
6. Authentik redirects back to Dex with authorization code
7. Dex exchanges code for tokens
8. Dex redirects back to ArgoCD with authorization code
9. ArgoCD exchanges code for tokens
10. ArgoCD validates ID token and extracts user info and groups
11. User is logged in to ArgoCD with appropriate role

### Tekton Dashboard OIDC Configuration

Tekton Dashboard is configured to use Dex as its OIDC provider.

#### OIDC Settings

**Issuer**: `https://dex.home.local`  
**Client ID**: `tekton-dashboard`  
**Client Secret**: Stored in `tekton-dashboard-oidc` Secret  
**Redirect URI**: `https://tekton.home.local/auth/callback`  
**Scopes**: `openid`, `profile`, `email`, `groups`

#### Group Mapping

Tekton Dashboard uses Kubernetes RBAC for authorization:

- `admins` group → ClusterRole with full access
- `engineering` group → ClusterRole with read-only access

#### Login Flow

1. User accesses Tekton Dashboard at `https://tekton.home.local`
2. Tekton Dashboard redirects to Dex authorization endpoint
3. Dex redirects to Authentik login page (if not already logged in)
4. User logs in to Authentik
5. Authentik redirects back to Dex with authorization code
6. Dex exchanges code for tokens
7. Dex redirects back to Tekton Dashboard with authorization code
8. Tekton Dashboard exchanges code for tokens
9. Tekton Dashboard validates ID token and extracts user info and groups
10. User is logged in to Tekton Dashboard with appropriate permissions

### OIDC Token Claims

ID tokens issued by Dex contain the following claims:

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

**Claim Descriptions**:

| Claim | Type | Description |
|-------|------|-------------|
| `iss` | string | Issuer (Dex URL) |
| `sub` | string | Subject (unique user ID) |
| `aud` | string | Audience (client ID) |
| `exp` | number | Expiration time (Unix timestamp) |
| `iat` | number | Issued at time (Unix timestamp) |
| `name` | string | User's display name |
| `email` | string | User's email address |
| `groups` | array | User's group memberships |
| `email_verified` | boolean | Whether email is verified |

### Authentication Error Responses

#### Authentik API Errors

**Unauthorized (401)**:
```json
{
  "detail": "Authentication credentials were not provided."
}
```

**Forbidden (403)**:
```json
{
  "detail": "You do not have permission to perform this action."
}
```

**Not Found (404)**:
```json
{
  "detail": "Not found."
}
```

#### Dex OIDC Errors

**Invalid Client (401)**:
```json
{
  "error": "invalid_client",
  "error_description": "Invalid client credentials."
}
```

**Invalid Grant (400)**:
```json
{
  "error": "invalid_grant",
  "error_description": "The provided authorization grant is invalid, expired, or revoked."
}
```

**Invalid Scope (400)**:
```json
{
  "error": "invalid_scope",
  "error_description": "The requested scope is invalid, unknown, or malformed."
}
```

**Source**
- `platform/auth/authentik/README.md`
- `platform/auth/dex/README.md`
- `platform/integrations/argocd-oidc-config.yaml`
- `platform/integrations/tekton-dashboard-oidc.yaml`
- `platform/auth/dex/configmap.yaml`

## AphexCLI Integration Contract

The AphexCLI provides OIDC-authenticated access to platform CRDs via the Kubernetes API.

### Kubeconfig Structure

After `aphex login`, kubeconfig contains:

```yaml
users:
  - name: aphex
    user:
      exec:
        apiVersion: client.authentication.k8s.io/v1beta1
        command: kubectl
        args:
          - oidc-login
          - get-token
          - --oidc-issuer-url=https://dex.home.local
          - --oidc-client-id=kubernetes
contexts:
  - name: aphex
    context:
      cluster: aphex
      user: aphex
```

### Capability Matrix

| Command | Resource | Verb | Namespace | Required Group |
|---------|----------|------|-----------|----------------|
| `aphex pipeline create` | pipelines.platform.dev | create | user-*, team-* | platform-engineering |
| `aphex pipeline list` | pipelines.platform.dev | list | user-*, team-* | platform-engineering |
| `aphex pipeline get` | pipelines.platform.dev | get | user-*, team-* | platform-engineering |
| `aphex pipeline delete` | pipelines.platform.dev | delete | user-*, team-* | platform-operators |

**Source**
- `platform/auth/dex/configmap.yaml`
- `platform/rbac/platform-rbac.yaml`

**Source**
- `.kiro/specs/argocd-tekton-platform/design.md`
- `.kiro/specs/argocd-tekton-platform/requirements.md`
- `platform/crds/repobinding-crd.yaml`
- `platform/onboarding/controller/`
- `platform/tenancy/templates/`
- `platform/argocd/apps/`
