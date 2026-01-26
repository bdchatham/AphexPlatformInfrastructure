# Architecture

## System Design

The Aphex Pipeline Infrastructure is a production-ready GitOps platform built on ArgoCD and Tekton with a revolutionary layered cert-manager architecture. The system provides bulletproof certificate management, centralized authentication, self-service repository onboarding, and complete platform automation.

The platform follows a **"Bootstrap Once, GitOps Forever"** pattern with **zero-touch convergence**: a one-time bootstrap script achieves complete platform deployment automatically, then ArgoCD manages all components declaratively with self-healing capabilities.

## High-Level Architecture

```mermaid
graph TB
    subgraph Git["Git Repository"]
        subgraph Platform_Manifests["platform/"]
            Bootstrap["bootstrap/<br/>(Zero-touch Script)"]
            ArgoCD_Apps["argocd/apps/<br/>(App of Apps)"]
            CertManager["cert-manager/<br/>(Layered Architecture)"]
            Auth["auth/<br/>(Authentik + Dex)"]
            Onboarding["onboarding/<br/>(Controller)"]
            Catalog["catalog/<br/>(Pipeline Tasks)"]
        end
    end
    
    Git -->|ArgoCD syncs with waves| ArgoCD_Svc
    Git -->|Webhooks trigger| EventListeners
    
    subgraph Cluster["Kubernetes Cluster"]
        subgraph ArgoCD_NS["argocd namespace"]
            ArgoCD_Svc["ArgoCD Server<br/>(OIDC Integration)"]
            ArgoCD_Controller["ArgoCD Controller<br/>(GitOps Engine)"]
        end
        
        subgraph CertManager_NS["cert-manager namespace"]
            CertManager_Controller["cert-manager<br/>(Wave 10)"]
            CertManager_Webhook["Webhook<br/>(Validated)"]
            CertManager_CAInjector["CA Injector<br/>(Fixed RBAC)"]
        end
        
        subgraph Auth_NS["auth-system namespace"]
            Authentik["Authentik<br/>(Identity Provider)"]
            Dex["Dex<br/>(OIDC Connector)"]
            PostgreSQL["PostgreSQL<br/>(Authentik DB)"]
        end
        
        subgraph Pipeline_System["pipeline-system namespace"]
            Onboarding_Ctrl["Onboarding Controller<br/>(RepoBinding CRD)"]
            Catalog_Tasks["Shared Pipeline Catalog<br/>(Versioned Tasks)"]
        end
        
        subgraph Tenants["Tenant Namespaces"]
            subgraph Tenant1["tenant-1"]
                EL1["EventListener<br/>(Webhook Handler)"]
                Pipeline1["Pipelines<br/>(Isolated Execution)"]
            end
            subgraph Tenant2["tenant-2"]
                EL2["EventListener<br/>(Webhook Handler)"]
                Pipeline2["Pipelines<br/>(Isolated Execution)"]
            end
        end
        
        subgraph Ingress_Layer["ingress-system namespace"]
            Ingress["Ingress Controller<br/>(TLS Termination)"]
        end
    end
    
    ArgoCD_Controller -->|Sync Wave 10| CertManager_Controller
    ArgoCD_Controller -->|Sync Wave 20| Auth_NS
    ArgoCD_Controller -->|Sync Wave 30| Ingress
    ArgoCD_Controller -->|Manages| Platform_System
    ArgoCD_Controller -->|Provisions| Tenants
    
    CertManager_Controller -->|Issues certificates| Auth_NS
    CertManager_Controller -->|Issues certificates| Ingress
    
    Ingress -->|TLS termination| ArgoCD_Svc
    Ingress -->|TLS termination| Authentik
    Ingress -->|Routes webhooks| EL1
    Ingress -->|Routes webhooks| EL2
    
    EL1 -->|Creates| Pipeline1
    EL2 -->|Creates| Pipeline2
    
    style Git fill:#e1f5ff
    style Cluster fill:#fff4e1
    style ArgoCD_NS fill:#e8f5e9
    style CertManager_NS fill:#ffe8e8
    style Auth_NS fill:#f0e8ff
    style Platform_System fill:#fff9c4
    style Tenants fill:#f3e5f5
```

## Product Team Onboarding Workflow

This section explains how product teams use the platform to manage their organizations and pipelines.

### Complete Onboarding Flow

```mermaid
sequenceDiagram
    participant PlatformAdmin as Platform Admin
    participant K8s as Kubernetes API
    participant OrgCtrl as Organization Controller
    participant Cloudflare as Cloudflare API
    participant ProductTeam as Product Team
    participant RepoCtrl as RepoBinding Controller
    participant GitHub as GitHub
    participant Webhook as Webhook Endpoint
    participant Pipeline as Tekton Pipeline
    
    Note over PlatformAdmin,Cloudflare: Step 1: Organization Provisioning
    PlatformAdmin->>K8s: Create Organization CRD
    K8s->>OrgCtrl: Organization created event
    OrgCtrl->>K8s: Create namespace (org-{name})
    OrgCtrl->>K8s: Generate webhook secret
    OrgCtrl->>Cloudflare: Create tunnel
    OrgCtrl->>Cloudflare: Create DNS CNAME record
    OrgCtrl->>K8s: Deploy cloudflared pod
    OrgCtrl->>K8s: Create EventListener ServiceAccount
    OrgCtrl->>K8s: Update Organization status
    Note over OrgCtrl: Status: webhookURL = https://{org}.arbiter-dev.com
    
    Note over ProductTeam,Pipeline: Step 2: Repository Onboarding
    ProductTeam->>K8s: Create RepoBinding CRD
    K8s->>RepoCtrl: RepoBinding created event
    RepoCtrl->>K8s: Validate Organization exists
    RepoCtrl->>K8s: Create ServiceAccount (pipeline-runner)
    RepoCtrl->>K8s: Create RBAC (Role, RoleBinding)
    RepoCtrl->>K8s: Create ResourceQuota, NetworkPolicy
    RepoCtrl->>K8s: Create TriggerBinding, TriggerTemplate
    RepoCtrl->>K8s: Reference Organization webhook secret
    RepoCtrl->>K8s: Update RepoBinding status
    Note over RepoCtrl: Status: webhookURL from Organization
    
    Note over ProductTeam,Pipeline: Step 3: GitHub Webhook Configuration
    ProductTeam->>ProductTeam: Read webhookURL from RepoBinding status
    ProductTeam->>ProductTeam: Read webhookSecret from Organization
    ProductTeam->>GitHub: Configure webhook in repo settings
    Note over GitHub: Payload URL: https://{org}.arbiter-dev.com<br/>Secret: {webhookSecret}<br/>Events: push
    
    Note over GitHub,Pipeline: Step 4: Pipeline Execution
    ProductTeam->>GitHub: Push code to main branch
    GitHub->>Webhook: POST webhook event
    Webhook->>Pipeline: EventListener creates PipelineRun
    Pipeline->>GitHub: Clone repository
    Pipeline->>Pipeline: Execute pipeline tasks
    Pipeline->>K8s: Update PipelineRun status
```

**Key Points**:
- **Organizations** provide webhook infrastructure (tunnel, DNS, EventListener)
- **RepoBindings** provision pipeline resources and reference Organization webhooks
- **Product teams** only need to configure GitHub webhook once
- **Pipelines** execute automatically on code changes

### Organization and RepoBinding Relationship

```mermaid
graph TB
    subgraph Organization["Organization: acme-corp"]
        OrgNS["Namespace: org-acme-corp"]
        Tunnel["Cloudflared Tunnel"]
        DNS["DNS: acme-corp.arbiter-dev.com"]
        WebhookSecret["Webhook Secret"]
        EventListener["EventListener ServiceAccount"]
    end
    
    subgraph RepoBinding1["RepoBinding: app1"]
        RB1["References: acme-corp"]
        SA1["ServiceAccount: pipeline-runner"]
        RBAC1["RBAC: Role + RoleBinding"]
        Trigger1["TriggerBinding + TriggerTemplate"]
    end
    
    subgraph RepoBinding2["RepoBinding: app2"]
        RB2["References: acme-corp"]
        SA2["ServiceAccount: pipeline-runner"]
        RBAC2["RBAC: Role + RoleBinding"]
        Trigger2["TriggerBinding + TriggerTemplate"]
    end
    
    subgraph GitHub_Repos["GitHub Repositories"]
        Repo1["acme-corp/app1<br/>Webhook: acme-corp.arbiter-dev.com"]
        Repo2["acme-corp/app2<br/>Webhook: acme-corp.arbiter-dev.com"]
    end
    
    Organization -->|Provides webhook infrastructure| RepoBinding1
    Organization -->|Provides webhook infrastructure| RepoBinding2
    
    Repo1 -->|Sends webhooks to| DNS
    Repo2 -->|Sends webhooks to| DNS
    
    DNS -->|Routes through| Tunnel
    Tunnel -->|Delivers to| EventListener
    
    EventListener -->|Triggers| Trigger1
    EventListener -->|Triggers| Trigger2
    
    style Organization fill:#e8f5e9
    style RepoBinding1 fill:#fff4e1
    style RepoBinding2 fill:#fff4e1
    style GitHub_Repos fill:#e1f5ff
```

**Benefits of this Model**:
- **Single webhook endpoint per organization**: All repos in an organization use the same webhook URL
- **Shared infrastructure**: Tunnel, DNS, and EventListener are shared across repos
- **Independent pipeline resources**: Each RepoBinding gets isolated RBAC and quotas
- **Simplified management**: Add new repos without creating new tunnels

For operational procedures on creating organizations and repo bindings, see [operations.md](operations.md#organization-and-repository-registration).

**Source**
- `platform/platform-controller/controller/controllers/organization_controller.go` - Organization provisioning
- `platform/platform-controller/controller/controllers/repobinding_controller.go` - RepoBinding provisioning
- `platform/crds/organization-crd.yaml` - Organization CRD
- `platform/crds/repobinding-crd.yaml` - RepoBinding CRD

#### Controller Implementation

The platform controller uses typed Kubernetes client libraries for type safety and proper resource handling:

**Typed Tekton Pipeline Processing**:
- Uses `tektonv1.Pipeline` structs instead of unstructured data
- Kubernetes YAML decoder (`k8s.io/apimachinery/pkg/util/yaml`) handles Tekton's custom JSON unmarshaling
- Registered Tekton v1 types in controller runtime scheme
- Provides compile-time type safety and IDE support

**Benefits**:
- Compile-time validation of Pipeline structure
- Proper handling of Tekton-specific types (e.g., `ParamValue`)
- Better error messages and debugging
- Clearer code that's easier to maintain

**Source**
- `platform/platform-controller/controller/controllers/repobinding_provisioners.go` - Typed Pipeline parsing
- `platform/platform-controller/controller/main.go` - Scheme registration

## Networking Architecture

The platform uses a dual-domain strategy to separate public webhook endpoints from private platform services.

### Network Topology

```mermaid
graph TB
    subgraph Internet["Internet"]
        GitHub["GitHub<br/>(github.com)"]
        CloudflareDNS["Cloudflare DNS<br/>(arbiter-dev.com)"]
        CloudflareEdge["Cloudflare Edge<br/>(SSL Termination)"]
    end
    
    subgraph HomeNetwork["Home Network / Data Center"]
        subgraph K8s["Kubernetes Cluster"]
            subgraph OrgNS["org-acme-corp namespace"]
                Cloudflared["cloudflared Pod<br/>(Outbound Tunnel)"]
                EL["EventListener<br/>(el-github-listener:8080)"]
            end
            
            subgraph IngressNS["ingress-system namespace"]
                IngressCtrl["Ingress Controller<br/>(nginx)"]
            end
            
            subgraph AuthNS["auth-system namespace"]
                Authentik["Authentik<br/>(auth.home.local)"]
                Dex["Dex<br/>(dex.home.local)"]
            end
            
            subgraph ArgoNS["argocd namespace"]
                ArgoCD["ArgoCD<br/>(argocd.home.local)"]
            end
        end
        
        subgraph LocalDevices["Local Devices"]
            Browser["Developer Browser"]
            Router["Home Router<br/>(DNS: *.home.local)"]
        end
    end
    
    GitHub -->|1. Webhook POST| CloudflareDNS
    CloudflareDNS -->|2. Resolve CNAME| CloudflareEdge
    CloudflareEdge -->|3. Route through tunnel| Cloudflared
    Cloudflared -->|4. Forward to Service| EL
    EL -->|5. Create PipelineRun| K8s
    
    Browser -->|6. Access platform UI| Router
    Router -->|7. Resolve *.home.local| IngressCtrl
    IngressCtrl -->|8. Route to service| ArgoCD
    IngressCtrl -->|Route to service| Authentik
    IngressCtrl -->|Route to service| Dex
    
    ArgoCD -->|9. OIDC auth| Dex
    Dex -->|10. OIDC auth| Authentik
    
    style Internet fill:#e1f5ff
    style HomeNetwork fill:#fff4e1
    style K8s fill:#f0f0f0
    style OrgNS fill:#e8f5e9
    style IngressNS fill:#ffe8e8
    style AuthNS fill:#f0e8ff
    style ArgoNS fill:#fff9c4
```

### Webhook Path Detail

```mermaid
sequenceDiagram
    participant Dev as Developer
    participant GitHub as GitHub
    participant DNS as Cloudflare DNS
    participant Edge as Cloudflare Edge
    participant Tunnel as Cloudflared Pod
    participant EL as EventListener
    participant Tekton as Tekton Controller
    participant Pod as Pipeline Pod
    
    Dev->>GitHub: git push origin main
    GitHub->>DNS: Webhook POST to acme-corp.arbiter-dev.com
    Note over GitHub,DNS: Payload: push event JSON<br/>Header: X-Hub-Signature-256
    
    DNS->>Edge: Resolve CNAME to tunnel
    Note over DNS,Edge: CNAME: {tunnel-id}.cfargotunnel.com
    
    Edge->>Edge: SSL/TLS termination
    Note over Edge: Full TLS mode<br/>Cloudflare certificate
    
    Edge->>Tunnel: Route through tunnel
    Note over Edge,Tunnel: Outbound connection<br/>No inbound ports required
    
    Tunnel->>EL: Forward to el-github-listener:8080
    Note over Tunnel,EL: Internal cluster networking<br/>Service: el-github-listener
    
    EL->>EL: Validate webhook signature
    Note over EL: Uses webhook secret from Organization
    
    EL->>EL: Check CEL filter
    Note over EL: Filter: body.ref == 'refs/heads/main'
    
    EL->>Tekton: Create PipelineRun
    Note over EL,Tekton: TriggerTemplate instantiation
    
    Tekton->>Pod: Start pipeline pod
    Note over Tekton,Pod: ServiceAccount: pipeline-runner<br/>Namespace: org-acme-corp
    
    Pod->>GitHub: Clone repository
    Pod->>Pod: Execute pipeline tasks
    Pod->>Tekton: Report completion
```

### Domain Strategy

**Public Domain (arbiter-dev.com)**:
- **Purpose**: Organization webhook endpoints accessible from internet
- **DNS**: Managed by Cloudflare
- **SSL/TLS**: Terminated at Cloudflare edge (automatic)
- **Access**: GitHub webhooks, external CI/CD triggers
- **Example**: `acme-corp.arbiter-dev.com`, `engineering.arbiter-dev.com`

**Local Domain (home.local)**:
- **Purpose**: Platform services accessible only within home network
- **DNS**: Managed by home router or Pi-hole
- **SSL/TLS**: Terminated at Ingress controller (self-signed or Let's Encrypt)
- **Access**: Platform administrators, developers on local network
- **Example**: `argocd.home.local`, `auth.home.local`, `dex.home.local`

**Benefits**:
- **Security**: Platform administration services not exposed to internet
- **Simplicity**: No port forwarding or firewall rules required
- **Reliability**: Cloudflare provides DDoS protection and global edge network
- **Flexibility**: Can use different domains for different environments

### Cloudflare Tunnel Architecture

```mermaid
graph LR
    subgraph Cloudflare["Cloudflare Infrastructure"]
        DNS["DNS Zone<br/>arbiter-dev.com"]
        Edge["Edge Network<br/>(Global PoPs)"]
        TunnelService["Tunnel Service<br/>(Cloudflare API)"]
    end
    
    subgraph Cluster["Kubernetes Cluster"]
        subgraph Org1["org-acme-corp"]
            CF1["cloudflared-acme-corp<br/>(Deployment)"]
            Creds1["Tunnel Credentials<br/>(Secret)"]
            Config1["Tunnel Config<br/>(ConfigMap)"]
            EL1["EventListener"]
        end
        
        subgraph Org2["org-engineering"]
            CF2["cloudflared-engineering<br/>(Deployment)"]
            Creds2["Tunnel Credentials<br/>(Secret)"]
            Config2["Tunnel Config<br/>(ConfigMap)"]
            EL2["EventListener"]
        end
    end
    
    DNS -->|CNAME record| Edge
    Edge -->|Route by hostname| TunnelService
    
    TunnelService <-->|Outbound connection| CF1
    TunnelService <-->|Outbound connection| CF2
    
    CF1 -->|Forward requests| EL1
    CF2 -->|Forward requests| EL2
    
    Creds1 -.->|Mounted as volume| CF1
    Config1 -.->|Mounted as volume| CF1
    Creds2 -.->|Mounted as volume| CF2
    Config2 -.->|Mounted as volume| CF2
    
    style Cloudflare fill:#f96854
    style Cluster fill:#fff4e1
    style Org1 fill:#e8f5e9
    style Org2 fill:#e8f5e9
```

**Tunnel Lifecycle**:
1. **Creation**: Organization controller calls Cloudflare API to create tunnel
2. **DNS**: Controller creates CNAME record pointing to tunnel
3. **Credentials**: Controller stores tunnel credentials in Secret
4. **Configuration**: Controller creates ConfigMap with tunnel routing rules
5. **Deployment**: Controller deploys cloudflared pod with credentials and config
6. **Connection**: cloudflared establishes outbound connection to Cloudflare
7. **Routing**: Cloudflare routes requests to tunnel based on hostname

**No Inbound Ports Required**:
- cloudflared maintains outbound connection to Cloudflare
- No firewall rules or port forwarding needed
- Works behind NAT and restrictive firewalls
- Automatic reconnection on network changes

For troubleshooting webhook delivery issues, see [operations.md](operations.md#test-webhook-endpoint).

**Source**
- `platform/platform-controller/controller/controllers/organization_controller.go` - Tunnel provisioning
- Cloudflare Tunnel documentation: https://developers.cloudflare.com/cloudflare-one/connections/connect-apps/

## Layered cert-manager Architecture

The platform implements a revolutionary **layered cert-manager architecture** that eliminates the classic "webhook chicken-and-egg" problem through proper dependency ordering and validation.

### cert-manager Deployment Waves

```mermaid
graph LR
    subgraph Wave10["Wave 10: cert-manager Installation"]
        CertManager[cert-manager Controller<br/>+ Webhook + CA Injector]
        PostSync[PostSync Hook<br/>Webhook Validation]
        CertManager --> PostSync
    end
    
    subgraph Wave20["Wave 20: Certificate Foundation"]
        ClusterIssuer[ClusterIssuer<br/>selfsigned-issuer]
        Certificates[Certificates<br/>dex-tls, argocd-tls, etc.]
        ClusterIssuer --> Certificates
    end
    
    subgraph Wave30["Wave 30: Ingress Resources"]
        Ingress[Ingress Resources<br/>TLS Configuration]
    end
    
    Wave10 -->|Webhook Ready| Wave20
    Wave20 -->|Certificates Ready| Wave30
    
    style Wave10 fill:#ffe8e8
    style Wave20 fill:#fff4e1
    style Wave30 fill:#e8f5e9
```

### PostSync Webhook Validation

The PostSync hook validates cert-manager webhook functionality before allowing certificate creation:

**Validation Checks:**
- Webhook Service has ready endpoints
- ValidatingWebhookConfiguration has non-empty caBundle
- MutatingWebhookConfiguration has non-empty caBundle
- CA injection process completed successfully

**Benefits:**
- Eliminates manual webhook restarts
- Prevents timing-related certificate failures
- Ensures deterministic deployment ordering
- Provides clear failure diagnostics

### RBAC Fix Implementation

The platform fixes cert-manager's default RBAC configuration using Kustomize patches:

```yaml
# Fix leader election namespace for both components
patchesJson6902:
  - target:
      kind: Deployment
      name: cert-manager-cainjector
    patch: |-
      - op: replace
        path: /spec/template/spec/containers/0/args/1
        value: --leader-election-namespace=cert-manager
  - target:
      kind: Deployment
      name: cert-manager
    patch: |-
      - op: replace
        path: /spec/template/spec/containers/0/args/2
        value: --leader-election-namespace=cert-manager
```

For operational procedures on verifying cert-manager deployment, see [operations.md](operations.md#layered-cert-manager-deployment).

**Source**
- `platform/cert-manager/kustomization.yaml` - Kustomize patches for RBAC fix
- `platform/cert-manager/webhook-readiness-hook.yaml` - PostSync validation hook
- `platform/argocd/apps/platform-cert-manager.yaml` - ArgoCD Application with sync wave 10
- `platform/argocd/apps/platform-cert-foundation.yaml` - ArgoCD Application with sync wave 20

## Authentication System Architecture

The authentication system provides centralized SSO for all platform services using Authentik as the Identity Provider with Dex as an OIDC connector layer.

### Component Overview

```mermaid
graph TB
    Users[Platform Users] --> Ingress[TLS Ingress]
    Ingress --> ArgoCD[ArgoCD UI]
    Ingress --> Tekton[Tekton Dashboard]
    Ingress --> Authentik[Authentik UI]
    Ingress --> Dex[Dex OIDC]
    
    ArgoCD --> Dex
    Tekton --> Dex
    Dex --> Authentik
    Authentik --> PostgreSQL[(PostgreSQL)]
    
    style Authentik fill:#f0e8ff
    style Dex fill:#e8f5e9
    style Ingress fill:#fff4e1
```

### Authentication Flow

```mermaid
sequenceDiagram
    participant User
    participant Service as ArgoCD/Tekton
    participant Dex
    participant Authentik
    
    User->>Service: Access UI
    Service->>Dex: Redirect to OIDC auth
    Dex->>Authentik: Redirect to login
    Authentik->>User: Show login page
    User->>Authentik: Submit credentials
    Authentik->>Dex: Return auth code
    Dex->>Service: Return auth code
    Service->>User: Grant access with permissions
```

### Component Responsibilities

**Authentik Server:**
- Web UI for user and group management
- User authentication and OIDC token issuance
- Integration with external identity providers
- Auto-applies Blueprint configurations on startup

**Dex OIDC Connector:**
- OIDC proxy between Authentik and platform services
- Provides stable OIDC endpoint for service integration
- Translates Authentik tokens to service-specific tokens

**Config Sync Job:**
- Orchestrates Authentik-Dex integration during bootstrap
- Updates Authentik OIDC provider configuration via API
- Scales Dex deployment after Authentik is ready

For detailed authentication operations, see [operations.md](operations.md#authentication-system-operations).
For authentication data models, see [data-models.md](data-models.md#authentication-data-models).
For authentication API details, see [api.md](api.md#authentication-api).

**Source**
- `platform/auth/authentik/server-deployment.yaml` - Authentik server deployment
- `platform/auth/dex/deployment.yaml` - Dex deployment
- `platform/auth/config-sync/job.yaml` - Config Sync Job
- `platform/auth/ingress/` - Ingress resources for auth services
- `platform/argocd/apps/platform-auth.yaml` - ArgoCD Application with sync wave 20

## Components

### 1. Bootstrap Script

**Purpose**: One-time initialization of cluster and platform components with deployment type support

**Location**: `bootstrap.sh` (dispatcher), `platform/deployments/{kind|k3s}/bootstrap/bootstrap.sh` (deployment-specific)

**Architecture**: The bootstrap system uses a dispatcher pattern to route to deployment-specific scripts:

```bash
bootstrap.sh (root dispatcher)
├── --deployment kind → platform/deployments/kind/bootstrap/bootstrap.sh
└── --deployment k3s → platform/deployments/k3s/bootstrap/bootstrap.sh
```

**Responsibilities**:
- Route to deployment-specific bootstrap script
- Create or use existing Kubernetes cluster
- Generate ALL secrets automatically (PostgreSQL, Authentik, Dex, API tokens)
- Install ArgoCD
- Create platform namespaces (argocd, auth-system, tekton-pipelines, platform-system, external-secrets)
- Create deployment-specific platform-root ArgoCD Application
- Wait for Authentik deployment and create API token
- Achieve complete platform convergence automatically

**Interface**:
```bash
./bootstrap.sh --deployment <kind|k3s> [OPTIONS]

Required:
  --deployment TYPE      Deployment type: kind (development) or k3s (production)

Kind Options:
  --cluster-name NAME    Name for Kind cluster (default: platform-cluster)
  --use-existing         Use existing kubecontext instead of creating cluster
  --show-secrets         Display generated secrets (WARNING: not for production)

K3s Options:
  --show-secrets         Display generated secrets (WARNING: not for production)
```

**Deployment Types**:

**Kind (Development)**:
- Creates local Kind cluster with Docker
- Simulated GPU support via RuntimeClass
- nginx-ingress-controller
- Uses `platform/base/argocd/apps` for Applications
- Apps reference `platform/deployments/kind/*` for resources

**K3s (Production)**:
- Installs K3s with NVIDIA container runtime
- Real GPU support via NVIDIA GPU Operator
- Gateway API with external DNS
- Uses `platform/deployments/k3s/argocd/apps` for Applications
- Apps reference `platform/deployments/k3s/*` for resources

**Output**:
- Kubernetes cluster running
- All secrets generated and stored in cluster
- ArgoCD installed and accessible
- Platform root Application created and syncing
- Complete platform convergence achieved
- Access instructions displayed

**Source**
- `bootstrap.sh` (dispatcher)
- `platform/deployments/kind/bootstrap/bootstrap.sh` (Kind deployment)
- `platform/deployments/k3s/bootstrap/bootstrap.sh` (K3s deployment)

### 2. ArgoCD

**Purpose**: GitOps continuous delivery tool that manages platform components

**Namespace**: `argocd`

**Installation**: Installed via kubectl during bootstrap from ArgoCD release manifests

**Configuration**:
```yaml
# Patch for insecure mode (homelab)
apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cmd-params-cm
  namespace: argocd
data:
  server.insecure: "true"  # For homelab without TLS
```

**Access**:
- UI: `http://localhost:8080` (port-forward) or via Ingress
- CLI: `argocd login <server>`
- Initial admin password: Retrieved from Secret `argocd-initial-admin-secret`

**Components**:
- argocd-server: Web UI and API server
- argocd-application-controller: Syncs Applications from Git
- argocd-repo-server: Manages Git repository connections
- argocd-dex-server: SSO and authentication (optional)

**Source**
- `platform/bootstrap/bootstrap.sh` (installation)
- `platform/argocd/argocd-cm-patch.yaml` (configuration)

### 3. Platform ArgoCD Applications (App of Apps Pattern)

**Purpose**: Manage all platform components via GitOps using the App of Apps pattern

**Architecture**: The platform uses a root Application that manages child Applications for each component layer.

**Root Application** (platform-root):
```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: platform-root
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/bdchatham/AphexPlatformInfrastructure
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

**Child Applications**:

1. **platform-crds** (CRDs and foundational resources)
2. **platform-infrastructure** (Namespaces, RBAC, base resources)
3. **platform-controllers** (Onboarding controller)
4. **platform-catalog** (Tekton tasks, pipelines, triggers)
5. **platform-tekton** (Tekton Pipelines, Triggers, and Core Interceptors)

**Sync Order**: ArgoCD automatically syncs Applications in dependency order using sync waves:
1. platform-crds (wave 0 - CRDs must exist first)
2. platform-tekton (wave 1 - Tekton must be installed before controllers)
3. platform-infrastructure (wave 1 - Namespaces and RBAC)
4. platform-controllers (wave 2 - Controllers depend on CRDs and infrastructure)
5. platform-catalog (wave 3 - Catalog depends on Tekton being installed)

**Benefits of App of Apps**:
- Independent lifecycle management for each component
- Clearer separation of concerns
- Easier troubleshooting (each Application has its own sync status)
- Can have different sync policies per component
- Better visibility in ArgoCD UI

**Source**
- `platform/argocd/apps/platform-root.yaml`
- `platform/argocd/apps/platform-crds.yaml`
- `platform/argocd/apps/platform-infrastructure.yaml`
- `platform/argocd/apps/platform-controllers.yaml`
- `platform/argocd/apps/platform-catalog.yaml`

### 4. Tekton Pipelines, Triggers, and Core Interceptors

**Purpose**: Pipeline execution engine, webhook handling, and interceptor services

**Namespace**: `tekton-pipelines`

**Installation**: Applied via kubectl during bootstrap, then managed by ArgoCD from `platform/tekton/`

**Configuration**:
```yaml
# Tekton Pipelines v0.65.0
# https://github.com/tektoncd/pipeline/releases/download/v0.65.0/release.yaml

# Tekton Triggers v0.34.0
# https://infra.tekton.dev/tekton-releases/triggers/previous/v0.34.0/release.yaml

# Tekton Triggers Core Interceptors v0.34.0
# https://infra.tekton.dev/tekton-releases/triggers/previous/v0.34.0/interceptors.yaml

# Tekton Dashboard v0.64.0
# https://github.com/tektoncd/dashboard/releases/download/v0.64.0/release.yaml
```

**Components**:
- tekton-pipelines-controller: Manages PipelineRun execution
- tekton-pipelines-webhook: Validates and mutates Tekton resources
- tekton-triggers-controller: Manages EventListeners and Triggers
- tekton-triggers-webhook: Validates Trigger resources
- tekton-triggers-core-interceptors: Provides ClusterInterceptors (github, gitlab, cel, bitbucket, slack)
- tekton-dashboard: Web UI for viewing and managing pipelines

**ClusterInterceptors**:
- github: Validates GitHub webhook signatures and filters events
- gitlab: Validates GitLab webhook signatures and filters events
- cel: Evaluates CEL expressions for custom filtering
- bitbucket: Validates Bitbucket webhook signatures
- slack: Validates Slack webhook signatures

**ArgoCD Management**:
After bootstrap, Tekton is managed by the `platform-tekton` ArgoCD Application. Updates to Tekton versions are made by updating `platform/tekton/kustomization.yaml` and committing to Git. ArgoCD automatically syncs changes.

**Source**
- `platform/bootstrap/bootstrap.sh` (initial installation)
- `platform/tekton/kustomization.yaml` (ArgoCD management)
- `platform/argocd/apps/platform-tekton.yaml` (ArgoCD Application)

### 5. External Secrets Operator

**Purpose**: Enable customers to create secrets in their organization namespace and reference them across their systems

**Namespace**: `external-secrets`

**Version**: v1.3.1

**Installation**: Managed by ArgoCD from `platform/base/external-secrets/`

**Sync Wave**: 15 (after cert-manager at wave 10, before auth at wave 20)

**Architecture**: Each organization receives a ClusterSecretStore that enables secret synchronization across namespaces.

**Per-Organization Resources** (created by Organization controller):
1. **ServiceAccount**: `eso-secrets-reader` in organization namespace
2. **Role**: Read access to `org-secrets` Secret
3. **RoleBinding**: Binds ServiceAccount to Role
4. **ClusterSecretStore**: `org-{name}-store` with Kubernetes provider

**ClusterSecretStore Configuration**:
```yaml
apiVersion: external-secrets.io/v1
kind: ClusterSecretStore
metadata:
  name: org-{name}-store
spec:
  conditions:
    - namespaceSelector:
        matchLabels:
          aphex.dev/org: {name}
  provider:
    kubernetes:
      remoteNamespace: org-{name}
      server:
        caProvider:
          type: ConfigMap
          name: kube-root-ca.crt
          key: ca.crt
      auth:
        serviceAccount:
          name: eso-secrets-reader
          namespace: org-{name}
```

**Customer Usage Pattern**:
1. Create Secret `org-secrets` in organization namespace with all secrets
2. Create ExternalSecret resources in any namespace labeled with organization
3. External Secrets Operator syncs secrets from `org-secrets` to target namespaces

**Example** (from ArchonKnowledgeBaseInfrastructure):
```yaml
apiVersion: external-secrets.io/v1
kind: ExternalSecret
metadata:
  name: knowledge-base-secrets
spec:
  refreshInterval: 1h
  secretStoreRef:
    name: org-archon-store
    kind: ClusterSecretStore
  target:
    name: knowledge-base-secrets
    creationPolicy: Owner
  data:
    - secretKey: github_token
      remoteRef:
        key: org-secrets
        property: github-token
```

**Benefits**:
- Centralized secret management per organization
- Secrets can be referenced across multiple namespaces
- No need to duplicate secrets
- Automatic synchronization and rotation support
- Namespace isolation via label selectors

**Patches Applied**:
- cert-controller: Fixed service and secret namespace to `external-secrets`
- webhook: Fixed DNS name to `external-secrets-webhook.external-secrets.svc`

**Source**
- `platform/base/external-secrets/kustomization.yaml` (operator installation)
- `platform/base/argocd/apps/platform-external-secrets.yaml` (ArgoCD Application)
- `platform/platform-controller/controller/controllers/organization_controller.go` (provisionSecretStore)

### 6. Organization and Onboarding System

**Purpose**: Multi-tenant organization management with automated webhook infrastructure and External Secrets provisioning

**Namespace**: `platform-system` (controllers), `org-{name}` (tenant resources)

**Installation**: Managed by ArgoCD from `platform/platform-controller/`

#### Organization Controller

**Resources**:
- Organization CRD and Controller
- Per-organization namespace provisioning
- Cloudflare tunnel with DNS record management
- EventListener with dedicated ServiceAccount and ClusterRoleBinding
- Organization-scoped webhook secrets and RBAC
- External Secrets ClusterSecretStore for secret management

**Controller Logic**:
1. Watch Organization resources in platform-system namespace
2. Create organization namespace: `org-{name}`
3. Generate webhook secret (cryptographically secure random)
4. Create Cloudflare tunnel via API
5. Create DNS CNAME record: `{org}.arbiter-dev.com → {tunnel-id}.cfargotunnel.com`
6. Create Cloudflared tunnel ConfigMap and Deployment
7. Create EventListener ServiceAccount with ClusterRoleBinding to eventlistener-access ClusterRole
8. Create organization admin RBAC
9. **Provision External Secrets infrastructure**:
   - Create ServiceAccount `eso-secrets-reader`
   - Create Role with read access to `org-secrets` Secret
   - Create RoleBinding
   - Create ClusterSecretStore `org-{name}-store`
10. Update Organization status with webhook URL: `https://{org}.arbiter-dev.com`

**Deletion Logic**:
1. Delete DNS CNAME record from Cloudflare
2. Cleanup tunnel connections via Cloudflare API
3. Delete tunnel from Cloudflare
4. Delete ClusterRoleBinding for EventListener
5. Delete ClusterSecretStore (cluster-scoped)
6. Delete organization namespace (cascades all resources)

**Source**
- `platform/platform-controller/controller/controllers/organization_controller.go` - Controller implementation
- `platform/platform-controller/controller/api/v1alpha1/organization_types.go` - CRD definition
- `platform/rbac/eventlistener-rbac.yaml` - EventListener ClusterRole

#### RepoBinding Controller

**Resources**:
- RepoBinding CRD and Controller
- Tekton webhook infrastructure per repository
- Pipeline namespace discovery and cross-namespace references

**Controller Logic**:
1. Watch RepoBinding resources
2. Validate spec (org, repo, tenant name, pipeline name)
3. Discover pipeline namespace automatically across cluster
4. Create namespace-scoped resources in pipeline namespace:
   - ServiceAccount (`pipeline-runner`)
   - RBAC (Role, RoleBinding, ClusterRole, ClusterRoleBinding, ArgoCD RoleBinding)
   - **ArgoCD AppProject for pipeline isolation**
   - ResourceQuota and LimitRange
   - NetworkPolicy
   - Terraform backend secret
5. Create Tekton webhook resources in organization namespace:
   - TriggerBinding (`github-push-binding`)
   - TriggerTemplate (`{tenant}-trigger-template`)
   - Trigger (`{pipeline-name}-trigger`)
6. Reference Organization-managed webhook secret
7. Update RepoBinding status with webhook configuration

**AppProject Provisioning**:
- **Name**: `{pipelineName}`
- **Namespace**: `argocd`
- **Destinations**: Only `{pipelineName}` and `{pipelineName}-*` namespaces
- **Source Repositories**: Only the specific GitHub repository
- **Cluster Resources**: None (empty whitelist)
- **Namespace Resources**: All resources allowed within scoped namespaces

**Deletion Logic**:
1. Delete Trigger from organization namespace
2. Delete TriggerTemplate from organization namespace
3. **Delete ArgoCD AppProject**
4. **Delete ArgoCD Applications with label `platform.aphex/pipeline`**
5. Finalizer cleanup

#### Per-Organization Webhook Infrastructure

Each organization gets isolated webhook infrastructure with public internet accessibility:

**Cloudflare Tunnel**:
- Unique subdomain: `{org}.arbiter-dev.com` (publicly resolvable)
- Dedicated tunnel deployment in organization namespace
- Routes directly to EventListener: `el-github-listener:8080`
- DNS CNAME record automatically created in Cloudflare
- SSL/TLS termination at Cloudflare edge (Full TLS mode)
- No inbound ports required on local network

**EventListener RBAC**:
- Dedicated ServiceAccount per organization: `eventlistener`
- ClusterRoleBinding: `eventlistener-{org}` → `eventlistener-access` ClusterRole
- Permissions for Tekton Triggers resources (namespace and cluster-scoped)
- Isolated from default ServiceAccount

**Webhook Flow**:
```
GitHub → {org}.arbiter-dev.com → Cloudflare DNS → Cloudflare Edge (SSL) → Tunnel → EventListener → TriggerTemplate → PipelineRun
```

**Benefits**:
- Complete isolation between organizations
- Simple GitHub webhook setup (unique URL per org)
- Public internet accessibility without exposing local network
- Automatic SSL certificate management via Cloudflare
- No manual DNS configuration required

**Source**
- `platform/platform-controller/controller/controllers/organization_controller.go` - Tunnel and DNS provisioning
- `platform/rbac/eventlistener-rbac.yaml` - EventListener ClusterRole definition
- No router port forwarding required
- Independent scaling per organization

#### KnowledgeBase Controller

**Resources**:
- KnowledgeBase CRD and Controller
- Documentation repository tracking for Archon RAG system
- Validation of repository URLs and documentation paths

**Controller Logic**:
1. Watch KnowledgeBase resources across all namespaces
2. Validate spec on creation or update:
   - Repository URLs must start with `https://github.com/`
   - Branch names must contain only valid Git characters
   - Documentation paths must start with `.kiro/docs`
3. Set status phase to Pending on first reconciliation
4. Update status phase to Ready when validation succeeds
5. Update status phase to Failed with descriptive error message when validation fails
6. Update lastReconcileTime on each reconciliation
7. Requeue every 5 minutes for periodic validation

**Status Phases**:
- **Pending**: Initial state, validation not yet complete
- **Ready**: Validation successful, repositories are valid
- **Failed**: Validation failed, see status message for details

**Validation Rules**:
- At least one repository must be specified
- Repository URLs must be GitHub HTTPS URLs
- Branch names must be valid Git branch names
- Documentation paths must be within `.kiro/docs` directory

**Integration with Archon**:
- Archon agent monitors KnowledgeBase resources
- Documentation changes in tracked repositories trigger re-ingestion
- Vector store is updated with latest documentation content
- RAG queries use ingested documentation for context

**Source**
- `platform/platform-controller/controller/controllers/knowledgebase_controller.go` - Controller implementation
- `platform/platform-controller/controller/api/v1alpha1/knowledgebase_types.go` - CRD definition
- `platform/crds/aphex_knowledgebases.yaml` - CRD manifest
- `platform/platform-controller/controller/` (Go source code)
- `platform/platform-controller/controller-deployment.yaml`
- `platform/platform-controller/controller-rbac.yaml`
- `platform/platform-controller/controller-service-account.yaml`

### 7. Tekton EventListener (Per Tenant)

**Purpose**: Receive GitHub webhooks and create PipelineRuns

**Namespace**: Tenant namespace (e.g., `tenant-example`)

**Created By**: Onboarding Controller when RepoBinding is created

**Definition**:
```yaml
apiVersion: triggers.tekton.dev/v1beta1
kind: EventListener
metadata:
  name: github-listener
  namespace: tenant-example
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
                secretName: webhook-tenant-example
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

**Service and Ingress**:
```yaml
# Service created automatically by EventListener
apiVersion: v1
kind: Service
metadata:
  name: el-github-listener
  namespace: tenant-example
spec:
  ports:
    - port: 8080
      targetPort: 8080
  selector:
    eventlistener: github-listener

---
# Ingress created by Onboarding Controller
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: github-webhook
  namespace: tenant-example
spec:
  rules:
    - host: webhooks.example.com
      http:
        paths:
          - path: /tenant-example
            pathType: Prefix
            backend:
              service:
                name: el-github-listener
                port:
                  number: 8080
```

**Source**
- `platform/tenancy/templates/eventlistener-template.yaml`
- `platform/tenancy/templates/ingress-template.yaml`

### 8. Pipeline Catalog

**Purpose**: Provide shared Tekton Tasks and Pipelines

**Namespace**: `pipeline-system`

**Installation**: Managed by ArgoCD from `platform/catalog/`

**Resources**:
- git-clone Task
- cdktf-synth Task
- cdktf-deploy Task
- cdktf-deploy-pipeline Pipeline
- TriggerBindings and TriggerTemplates

**Configuration**:
```yaml
# Tasks and Pipelines deployed to pipeline-system namespace
# Referenced by tenants using namespace-qualified names
```

**Source**
- `platform/catalog/tasks/git-clone.yaml`
- `platform/catalog/tasks/cdktf-synth.yaml`
- `platform/catalog/tasks/cdktf-deploy.yaml`
- `platform/catalog/pipelines/cdktf-deploy-pipeline.yaml`
- `platform/catalog/triggers/github-push-binding.yaml`
- `platform/catalog/triggers/cdktf-deploy-trigger-template.yaml`

### 9. RepoBinding Custom Resource Definition

**Purpose**: Define repository onboarding requests

**Namespace**: `pipeline-system` (CRD is cluster-scoped, instances are namespaced)

**Installation**: Managed by ArgoCD from `platform/crds/`

**CRD Definition**:
```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: repobindings.aphex
spec:
  group: aphex
  names:
    kind: RepoBinding
    plural: repobindings
    singular: repobinding
  scope: Namespaced
  versions:
    - name: v1alpha1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              required:
                - aphexOrg
                - repoOrg
                - repoName
                - pipelineName
                - templateRef
              properties:
                aphexOrg:
                  type: string
                repoOrg:
                  type: string
                repoName:
                  type: string
                pipelineName:
                  type: string
                templateRef:
                  type: string
            status:
              type: object
              properties:
                phase:
                  type: string
                  enum: ["Pending", "Provisioning", "Ready", "Failed"]
                message:
                  type: string
                webhookURL:
                  type: string
                webhookSecret:
                  type: string
                namespaceCreated:
                  type: boolean
                serviceAccountCreated:
                  type: boolean
                rbacCreated:
                  type: boolean
                quotasCreated:
                  type: boolean
                networkPolicyCreated:
                  type: boolean
                terraformSecretCreated:
                  type: boolean
                eventListenerCreated:
                  type: boolean
                ingressCreated:
                  type: boolean
```

**Source**
- `platform/crds/repobinding-crd.yaml`
- `platform/crds/example-repobinding.yaml`

## Data Flow

### Git to Kubernetes (GitOps Sync)

```mermaid
sequenceDiagram
    participant Engineer as Platform Engineer
    participant Git as Git Repository
    participant ArgoCD
    participant K8s as Kubernetes Cluster
    
    Engineer->>Git: Commit platform changes
    ArgoCD->>Git: Poll for changes (every 3 minutes)
    ArgoCD->>ArgoCD: Detect changes
    ArgoCD->>K8s: Sync manifests
    ArgoCD->>K8s: Apply updates
    K8s-->>ArgoCD: Sync status
    ArgoCD-->>Engineer: Display sync status in UI
```

**Flow Description**:
1. Platform engineer commits changes to platform manifests in Git
2. ArgoCD polls Git repository every 3 minutes (default)
3. ArgoCD detects changes and compares with cluster state
4. ArgoCD applies changes to Kubernetes cluster
5. ArgoCD reports sync status in UI

**Key Points**:
- All platform configuration is stored in Git (version-controlled)
- ArgoCD automatically syncs changes (no manual kubectl apply)
- Sync policies: automated sync, self-heal, prune
- Retry policy with exponential backoff for transient failures

### GitHub Webhook to PipelineRun

```mermaid
sequenceDiagram
    participant Dev as Developer
    participant GitHub
    participant Ingress
    participant EventListener
    participant Tekton
    participant Pipeline Pod
    
    Dev->>GitHub: Merge to main
    GitHub->>Ingress: Push event webhook
    Ingress->>EventListener: Route to tenant EventListener
    EventListener->>EventListener: Validate webhook signature
    EventListener->>EventListener: Check CEL filter (main branch)
    EventListener->>Tekton: Create PipelineRun
    Tekton->>Pipeline Pod: Start pipeline (as tenant SA)
    Pipeline Pod->>GitHub: Clone repo at commit SHA
    Pipeline Pod->>Pipeline Pod: cdktf synth
    Pipeline Pod->>Pipeline Pod: cdktf deploy (remote state)
    Pipeline Pod-->>Tekton: Pipeline complete
```

**Flow Description**:
1. Developer merges code to main branch
2. GitHub sends push event webhook to Ingress
3. Ingress routes webhook to tenant EventListener based on path
4. EventListener validates webhook signature using tenant secret
5. EventListener checks CEL filter (only main branch pushes)
6. EventListener creates PipelineRun in tenant namespace
7. Tekton starts pipeline pod using tenant service account
8. Pipeline clones repository at specific commit SHA
9. Pipeline runs cdktf synth to generate Terraform config
10. Pipeline runs cdktf deploy to apply infrastructure changes
11. Pipeline completes and reports status

**Key Points**:
- Each tenant has dedicated EventListener with unique webhook secret
- Webhook signature validation prevents unauthorized triggers
- CEL filters enable branch-specific triggering
- Pipelines run with tenant service account (RBAC isolation)
- Terraform state stored in Kubernetes backend (per-tenant isolation)

### Tenant Provisioning Flow

```mermaid
sequenceDiagram
    participant Dev as Developer
    participant K8s as Kubernetes API
    participant Controller as Onboarding Controller
    participant GitHub
    
    Dev->>K8s: Create RepoBinding YAML
    K8s->>Controller: RepoBinding created event
    Controller->>Controller: Validate spec
    Controller->>Controller: Generate webhook secret
    Controller->>K8s: Create namespace
    Controller->>K8s: Create webhook Secret
    Controller->>K8s: Create ServiceAccount
    Controller->>K8s: Create RBAC
    Controller->>K8s: Create ResourceQuota
    Controller->>K8s: Create NetworkPolicy
    Controller->>K8s: Create Terraform secret
    Controller->>K8s: Create EventListener
    Controller->>K8s: Create Ingress
    Controller->>K8s: Update RepoBinding status
    
    Note over Dev,K8s: RepoBinding status shows webhook URL and secret
    
    Dev->>Dev: Read webhook URL and secret from status
    Dev->>GitHub: Configure webhook with URL and secret
    
    Note over Dev,K8s: Tenant is ready for webhooks
```

**Flow Description**:
1. Developer creates RepoBinding resource
2. Kubernetes API notifies Onboarding Controller
3. Controller validates request (org, namespace pattern, permission profile)
4. Controller generates cryptographically secure webhook secret
5. Controller creates tenant namespace with labels
6. Controller creates webhook Secret in tenant namespace
7. Controller creates ServiceAccount for pipeline execution
8. Controller creates Role and RoleBinding based on permission profile
9. Controller creates ResourceQuota and LimitRange
10. Controller creates NetworkPolicy for tenant isolation
11. Controller creates Terraform backend secret
12. Controller creates EventListener for webhook handling
13. Controller creates Ingress for webhook routing
14. Controller updates RepoBinding status with webhook URL and secret
15. Developer reads webhook URL and secret from RepoBinding status
16. Developer configures webhook in GitHub repository settings

**Key Points**:
- Fully automated tenant provisioning (no manual steps)
- Webhook secret generated and stored securely
- RBAC enforces least-privilege access
- Resource quotas prevent resource exhaustion
- Network policies enforce tenant isolation
- Terraform state isolated per tenant

## Technology Stack

### Infrastructure
- **Kubernetes**: Container orchestration platform (1.24+)
- **kubectl**: Kubernetes CLI
- **Kind**: Kubernetes in Docker (for local development)

### GitOps Platform
- **ArgoCD**: GitOps continuous delivery tool
- **Tekton Pipelines**: Pipeline execution engine
- **Tekton Triggers**: Event-driven triggering

### Development Tools
- **Go**: Onboarding controller implementation
- **CDKTF**: Cloud Development Kit for Terraform
- **Terraform**: Infrastructure as code

### Container Runtime
- **Docker**: Container image format
- **containerd**: Container runtime

## Architectural Patterns

### 1. GitOps
All configuration is stored in Git. ArgoCD syncs changes automatically, enabling declarative infrastructure management and self-upgrade capabilities.

### 2. App of Apps
Root ArgoCD Application manages child Applications for each component layer, providing better separation of concerns and independent lifecycle management.

### 3. Event-Driven Architecture
Tekton EventListeners receive GitHub webhooks and trigger pipelines, enabling automated deployments on code changes.

### 4. Multi-Tenancy with Isolation
Multiple tenants share the cluster but are isolated through:
- Kubernetes namespaces (one per tenant)
- Network policies (restrict inter-namespace traffic)
- Resource quotas (prevent resource exhaustion)
- RBAC (separate service accounts and roles)

### 5. Operator Pattern
The onboarding controller follows the Kubernetes operator pattern, reconciling RepoBinding resources to provision tenant infrastructure.

### 6. Immutable Infrastructure
Container images are versioned and immutable. Infrastructure changes are deployed through GitOps, not manual modifications.

**Source**
- `.kiro/specs/argocd-tekton-platform/design.md`
- `.kiro/specs/argocd-tekton-platform/requirements.md`
- `.kiro/specs/dex-authentication-platform/design.md`
- `.kiro/specs/dex-authentication-platform/requirements.md`
- `platform/bootstrap/bootstrap.sh`
- `platform/argocd/apps/`
- `platform/auth/`
- `platform/auth/config-sync/`
- `platform/auth/ingress/`
- `platform/crds/repobinding-crd.yaml`
- `platform/platform-controller/controller/`
- `platform/catalog/`
- `platform/tenancy/templates/`
