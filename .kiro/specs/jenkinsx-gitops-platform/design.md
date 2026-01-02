# Design Document: Jenkins X GitOps Platform

## Overview

This document describes the design of a GitOps-based Jenkins X platform for homelab Kubernetes clusters. The platform provides self-service repository registration with automated tenant provisioning and CDKTF deployment pipelines, managed entirely through ArgoCD.

### Key Design Principles

1. **GitOps-First**: All platform components are managed declaratively through Git
2. **Minimal Bootstrap**: Bootstrap script only creates cluster and installs ArgoCD
3. **Standard Tools**: Leverage ArgoCD, Helm, and Kustomize instead of custom scripts
4. **Declarative**: Everything is defined as Kubernetes resources
5. **Self-Healing**: ArgoCD automatically reconciles drift

### Architecture Philosophy

The platform follows a **"Bootstrap to GitOps"** pattern:

```mermaid
sequenceDiagram
    participant User
    participant Bootstrap as Bootstrap Script
    participant Cluster as Kubernetes Cluster
    participant ArgoCD
    participant Git as Git Repository
    
    User->>Bootstrap: Run bootstrap.sh
    Bootstrap->>Cluster: Create cluster (Kind/k3s)
    Bootstrap->>Cluster: Install ArgoCD
    Bootstrap->>ArgoCD: Create root Application
    ArgoCD->>Git: Watch repository
    Git-->>ArgoCD: Detect changes
    ArgoCD->>Cluster: Deploy all platform components
    ArgoCD->>Cluster: Continuously sync and reconcile
    
    Note over Bootstrap,ArgoCD: Bootstrap is one-time only
    Note over ArgoCD,Cluster: ArgoCD manages everything else
```

This eliminates the need for multiple installation scripts, manual kubectl commands, and imperative workflows. Once ArgoCD is installed, all platform management happens through Git commits.

## Architecture

### High-Level Architecture

```mermaid
graph TB
    subgraph Git["Git Repository (ArbiterPipelineInfrastructure)"]
        subgraph ArgoCD_Manifests["argocd/"]
            RootApp["root-app.yaml<br/>(App-of-Apps)"]
            InfraApps["infrastructure-apps.yaml"]
            PlatformApps["platform-apps.yaml"]
            TenantApps["tenant-apps.yaml"]
        end
        
        subgraph Platform_Manifests["platform/"]
            Infra["infrastructure/<br/>(Namespaces, CRDs, Dex)"]
            Tekton["tekton/<br/>(Tekton via Helm)"]
            Lighthouse["lighthouse/<br/>(Lighthouse via Helm)"]
            Registration["registration/<br/>(Controller + RBAC)"]
            Catalog["catalog/<br/>(Pipeline Tasks)"]
        end
    end
    
    Git -->|ArgoCD watches and syncs| Cluster
    
    subgraph Cluster["Kubernetes Cluster"]
        subgraph Platform_Services["Platform Services"]
            ArgoCDSvc["ArgoCD"]
            TektonSvc["Tekton"]
            LighthouseSvc["Lighthouse"]
            DexSvc["Dex"]
            RegistrationSvc["Registration<br/>Controller"]
            CatalogSvc["Catalog"]
        end
        
        subgraph Tenants["Tenant Namespaces"]
            Tenant1["tenant-1"]
            Tenant2["tenant-2"]
            TenantN["tenant-N"]
        end
    end
    
    style Git fill:#e1f5ff
    style Cluster fill:#fff4e1
    style Platform_Services fill:#e8f5e9
    style Tenants fill:#f3e5f5
```

### Component Layers

The platform is organized into three layers:

1. **Infrastructure Layer**: Core Kubernetes resources (namespaces, CRDs, RBAC, Dex)
2. **Platform Layer**: Platform services (Tekton, Lighthouse, Registration Controller, Catalog)
3. **Tenant Layer**: User namespaces and resources (provisioned by Registration Controller)

### ArgoCD Application Structure

```mermaid
graph TD
    Root["root-app<br/>(App-of-Apps)"]
    
    Root --> InfraApps["infrastructure-apps"]
    Root --> PlatformApps["platform-apps"]
    Root --> TenantApps["tenant-apps"]
    
    InfraApps --> Namespaces["namespaces"]
    InfraApps --> CRDs["crds"]
    InfraApps --> Dex["dex"]
    
    PlatformApps --> Tekton["tekton"]
    PlatformApps --> Lighthouse["lighthouse"]
    PlatformApps --> Registration["registration-controller"]
    PlatformApps --> Catalog["pipeline-catalog"]
    
    TenantApps -.->|dynamically created| TenantApp["tenant-*<br/>(created by controller)"]
    
    style Root fill:#ff9800
    style InfraApps fill:#2196f3
    style PlatformApps fill:#4caf50
    style TenantApps fill:#9c27b0
```

## Components and Interfaces

### 1. Bootstrap Script

**Purpose**: One-time initialization of cluster and ArgoCD

**Responsibilities**:
- Create Kubernetes cluster (Kind for local, configurable for others)
- Install ArgoCD using official manifests
- Create initial ArgoCD Application pointing to Git repository
- Configure ArgoCD to watch the platform repository
- Display ArgoCD admin credentials
- Display Lighthouse webhook URL for product teams
- No GitHub integration required (webhooks managed manually)

**Interface**:
```bash
./bootstrap.sh [OPTIONS]

Options:
  --cluster-name NAME    Name of the cluster (default: arbiter-infrastructure)
  --cluster-type TYPE    Type of cluster: kind, k3s, existing (default: kind)
  --repo-url URL         Git repository URL (default: current repo)
  --repo-branch BRANCH   Git branch to watch (default: mainline)
  --argocd-version VER   ArgoCD version to install (default: stable)
```

**Output**:
- Kubernetes cluster running
- ArgoCD installed and accessible
- Root Application created and syncing
- ArgoCD admin password displayed
- Lighthouse webhook URL displayed for product teams

**Configuration**: None (minimal script, no config files)

### 2. ArgoCD

**Purpose**: GitOps operator that manages all platform components

**Responsibilities**:
- Watch Git repository for changes
- Sync Kubernetes resources to match Git state
- Detect and report drift
- Provide UI and CLI for status and management
- Handle Application dependencies via sync waves

**Interface**:
- **UI**: `https://argocd.homelab.local` (or port-forward)
- **CLI**: `argocd` command-line tool
- **API**: Kubernetes CRDs (Application, AppProject)

**Configuration**:
```yaml
# argocd/root-app.yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: root-app
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/bdchatham/ArbiterPipelineInfrastructure
    targetRevision: mainline
    path: argocd/apps
  destination:
    server: https://kubernetes.default.svc
    namespace: argocd
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
```

### 3. Infrastructure Applications

#### 3.1 Namespaces

**Purpose**: Create platform and system namespaces

**Managed By**: ArgoCD Application using Kustomize

**Resources**:
- `platform-services`: Platform services namespace
- `platform-assets`: Shared pipeline assets namespace
- `auth-system`: Authentication services namespace (Dex)

**Configuration**:
```yaml
# platform/infrastructure/namespaces/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - namespace-platform-services.yaml
  - namespace-platform-assets.yaml
  - namespace-auth-system.yaml
```

#### 3.2 Custom Resource Definitions (CRDs)

**Purpose**: Install RepoBinding CRD

**Managed By**: ArgoCD Application using Kustomize

**Resources**:
- RepoBinding CRD with validation rules

**Configuration**:
```yaml
# platform/infrastructure/crds/repobinding-crd.yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: repobindings.arbiter.io
spec:
  group: arbiter.io
  names:
    kind: RepoBinding
    plural: repobindings
  scope: Cluster
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
              required: [repoOrg, repoName, tenantName]
              properties:
                repoOrg:
                  type: string
                  pattern: '^[a-z0-9-]+$'
                repoName:
                  type: string
                  pattern: '^[a-z0-9-]+$'
                tenantName:
                  type: string
                  pattern: '^tenant-[a-z0-9-]+$'
                permissionProfile:
                  type: string
                  enum: [standard, elevated]
                  default: standard
            status:
              type: object
              properties:
                phase:
                  type: string
                  enum: [Pending, Provisioning, Ready, Failed]
                message:
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
                allowlistUpdated:
                  type: boolean
```

#### 3.3 Dex (OIDC Provider)

**Purpose**: Provide OIDC authentication for Kubernetes API and webhooks

**Managed By**: ArgoCD Application using Kustomize

**Resources**:
- Dex Deployment
- Dex Service
- Dex ConfigMap
- Dex RBAC

**Configuration**:
```yaml
# platform/infrastructure/dex/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: auth-system
resources:
  - deployment.yaml
  - service.yaml
  - configmap.yaml
  - rbac.yaml
```

**Dex ConfigMap**:
```yaml
# platform/infrastructure/dex/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: dex-config
  namespace: auth-system
data:
  config.yaml: |
    issuer: https://dex.homelab.local
    storage:
      type: kubernetes
      config:
        inCluster: true
    web:
      http: 0.0.0.0:5556
    enablePasswordDB: true
    staticClients:
      - id: kubernetes
        name: Kubernetes
        secret: kubernetes-client-secret
        redirectURIs:
          - http://localhost:8000
    staticPasswords:
      - email: "admin@homelab.local"
        hash: "$2a$10$2b2cU8CPhOTaGrs1HRQuAueS7JTT5ZHsHSzYiFPm1leZck7Mc8T4W"
        username: "admin"
        userID: "admin"
        groups:
          - engineering
```

### 4. Platform Applications

#### 4.1 Tekton Pipelines

**Purpose**: Provide pipeline execution engine

**Managed By**: ArgoCD Application using Helm

**Configuration**:
```yaml
# argocd/apps/tekton.yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: tekton
  namespace: argocd
spec:
  project: default
  source:
    chart: tekton-pipelines
    repoURL: https://charts.tekton.dev
    targetRevision: 0.56.0
    helm:
      values: |
        controller:
          image:
            repository: ghcr.io/tektoncd/pipeline/controller
            tag: v0.56.0
        webhook:
          image:
            repository: ghcr.io/tektoncd/pipeline/webhook
            tag: v0.56.0
  destination:
    server: https://kubernetes.default.svc
    namespace: platform-services
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
      - CreateNamespace=true
  syncWave: 1
```

**Note**: If Tekton doesn't have an official Helm chart, we'll use a Kustomize-based Application that applies the release YAML with image patches for ghcr.io.

#### 4.2 Lighthouse

**Purpose**: Handle GitHub webhooks and trigger pipelines

**Managed By**: ArgoCD Application using Helm or Kustomize

**Configuration**:
```yaml
# argocd/apps/lighthouse.yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: lighthouse
  namespace: argocd
spec:
  project: default
  source:
    chart: lighthouse
    repoURL: https://jenkins-x-charts.github.io/repo
    targetRevision: 1.x.x
    helm:
      values: |
        webhook:
          enabled: true
        configMaps:
          config: "lighthouse-config"
          allowlist: "repo-allowlist"
  destination:
    server: https://kubernetes.default.svc
    namespace: platform-services
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
  syncWave: 2
```

**Webhook Management**:
- Webhooks created manually by product teams in GitHub
- Registration Controller generates webhook secrets
- Lighthouse validates webhook signatures using stored secrets
- Each repository has its own webhook secret stored in Kubernetes

#### 4.3 Registration Controller

**Purpose**: Provision tenant resources based on RepoBinding CRs

**Managed By**: ArgoCD Application using Kustomize

**Resources**:
- Controller Deployment
- Controller ServiceAccount
- Controller ClusterRole and ClusterRoleBinding
- Tenant resource templates (ConfigMaps)

**Configuration**:
```yaml
# platform/registration/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: platform-services
resources:
  - deployment.yaml
  - service-account.yaml
  - rbac.yaml
  - templates/
images:
  - name: registration-controller
    newName: ghcr.io/bdchatham/registration-controller
    newTag: latest
```

**Controller Logic**:
- Watch RepoBinding resources
- Validate spec (org, repo, tenant name)
- **Generate webhook secret** (cryptographically secure random string)
- Store webhook secret in Kubernetes Secret for Lighthouse
- Create namespace with labels
- Create service account
- Create Role/RoleBinding based on permission profile
- Create ResourceQuota and LimitRange
- Create NetworkPolicy
- Create Terraform backend secret
- Update Lighthouse allowlist ConfigMap with repo + secret reference
- Update RepoBinding status with webhook secret and URL
- **Return webhook secret to user** for manual GitHub webhook configuration

**Webhook Secret Management**:
- Secret generated using crypto/rand (e.g., `whsec_` + 32 random bytes base64)
- Stored in Secret: `webhook-<tenant-name>` in platform-services namespace
- Lighthouse reads secrets to validate incoming webhooks
- Secret displayed in RepoBinding status and CLI output

#### 4.4 Pipeline Catalog

**Purpose**: Provide shared Tekton Tasks and Pipelines

**Managed By**: ArgoCD Application using Kustomize

**Resources**:
- git-clone Task
- cdktf-synth Task
- cdktf-deploy Task
- cdktf-deploy-pipeline Pipeline

**Configuration**:
```yaml
# platform/catalog/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: platform-assets
resources:
  - tasks/git-clone.yaml
  - tasks/cdktf-synth.yaml
  - tasks/cdktf-deploy.yaml
  - pipelines/cdktf-deploy-pipeline.yaml
```

### 5. Tenant Applications

**Purpose**: Represent tenant namespaces in ArgoCD

**Managed By**: Registration Controller (creates ArgoCD Applications dynamically)

**Pattern**: When a RepoBinding is created with GitHub App credentials, the Registration Controller provisions all tenant resources including the Lighthouse secret.

```mermaid
sequenceDiagram
    participant Dev as Developer
    participant Git as Git Repository
    participant ArgoCD
    participant Controller as Registration Controller
    participant K8s as Kubernetes API
    participant Lighthouse
    
    Dev->>K8s: Create Secret with GitHub App private key
    Dev->>Git: Create RepoBinding YAML<br/>(includes GitHub App ID, Installation ID, secret ref)
    Dev->>Git: git commit & push
    ArgoCD->>Git: Detect change
    ArgoCD->>K8s: Apply RepoBinding
    K8s->>Controller: RepoBinding created event
    Controller->>K8s: Validate GitHub App credentials exist
    Controller->>K8s: Create namespace
    Controller->>K8s: Create Lighthouse secret<br/>(from GitHub App credentials)
    Controller->>K8s: Create ServiceAccount
    Controller->>K8s: Create RBAC
    Controller->>K8s: Create ResourceQuota
    Controller->>K8s: Create NetworkPolicy
    Controller->>K8s: Create Terraform secret
    Controller->>Lighthouse: Update allowlist ConfigMap
    Controller->>K8s: Update RepoBinding status
    Controller->>ArgoCD: Create tenant Application
    ArgoCD->>K8s: Sync tenant resources
    
    Note over Dev,ArgoCD: Product team provides all credentials upfront
    Note over Controller,K8s: Platform creates everything needed
```

**Configuration** (created by controller):
```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: tenant-example-repo
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/bdchatham/ArbiterPipelineInfrastructure
    targetRevision: mainline
    path: tenants/example-repo
  destination:
    server: https://kubernetes.default.svc
    namespace: tenant-example-repo
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
```

## Data Models

### RepoBinding Custom Resource

```yaml
apiVersion: arbiter.io/v1alpha1
kind: RepoBinding
metadata:
  name: example-repo-binding
spec:
  repoOrg: bdchatham
  repoName: example-repo
  tenantName: tenant-example-repo
  permissionProfile: standard  # or elevated
status:
  phase: Ready  # Pending, Provisioning, Ready, Failed
  message: "Tenant provisioned successfully. Configure webhook in GitHub."
  webhookSecret: whsec_abc123xyz456  # Generated by controller, use in GitHub
  webhookURL: https://lighthouse.homelab.local/hook
  namespaceCreated: true
  serviceAccountCreated: true
  rbacCreated: true
  quotasCreated: true
  networkPolicyCreated: true
  terraformSecretCreated: true
  webhookSecretCreated: true
  allowlistUpdated: true
```

**Webhook Setup Instructions** (displayed after registration):
```
Registration successful!

Next steps - Configure GitHub webhook:
1. Go to: https://github.com/bdchatham/example-repo/settings/hooks/new
2. Payload URL: https://lighthouse.homelab.local/hook
3. Content type: application/json
4. Secret: whsec_abc123xyz456
5. Events: Push events, Pull request events
6. Active: ✓
7. Click "Add webhook"
```

### ArgoCD Application (Root)

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: root-app
  namespace: argocd
  finalizers:
    - resources-finalizer.argocd.argoproj.io
spec:
  project: default
  source:
    repoURL: https://github.com/bdchatham/ArbiterPipelineInfrastructure
    targetRevision: mainline
    path: argocd/apps
  destination:
    server: https://kubernetes.default.svc
    namespace: argocd
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
      allowEmpty: false
    syncOptions:
      - CreateNamespace=false
```

### Lighthouse ConfigMap (Allowlist)

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: repo-allowlist
  namespace: platform-services
data:
  allowlist.yaml: |
    repositories:
      - org: bdchatham
        repo: example-repo
        tenant: tenant-example-repo
      - org: bdchatham
        repo: another-repo
        tenant: tenant-another-repo
```

## Repository Structure

```
ArbiterPipelineInfrastructure/
├── argocd/
│   ├── root-app.yaml                    # Root Application (App-of-Apps)
│   └── apps/
│       ├── infrastructure-apps.yaml     # Infrastructure layer apps
│       ├── platform-apps.yaml           # Platform layer apps
│       └── tenant-apps.yaml             # Tenant layer apps (optional)
├── platform/
│   ├── infrastructure/
│   │   ├── namespaces/
│   │   │   ├── kustomization.yaml
│   │   │   ├── namespace-platform-services.yaml
│   │   │   ├── namespace-platform-assets.yaml
│   │   │   └── namespace-auth-system.yaml
│   │   ├── crds/
│   │   │   ├── kustomization.yaml
│   │   │   └── repobinding-crd.yaml
│   │   └── dex/
│   │       ├── kustomization.yaml
│   │       ├── deployment.yaml
│   │       ├── service.yaml
│   │       ├── configmap.yaml
│   │       └── rbac.yaml
│   ├── tekton/
│   │   └── application.yaml             # ArgoCD App for Tekton
│   ├── lighthouse/
│   │   ├── application.yaml             # ArgoCD App for Lighthouse
│   │   └── config/
│   │       ├── lighthouse-config.yaml
│   │       └── repo-allowlist.yaml
│   ├── registration/
│   │   ├── kustomization.yaml
│   │   ├── deployment.yaml
│   │   ├── service-account.yaml
│   │   ├── rbac.yaml
│   │   └── templates/
│   │       ├── namespace-template.yaml
│   │       ├── serviceaccount-template.yaml
│   │       ├── role-standard-template.yaml
│   │       ├── role-elevated-template.yaml
│   │       ├── rolebinding-template.yaml
│   │       ├── resourcequota-template.yaml
│   │       ├── limitrange-template.yaml
│   │       ├── networkpolicy-template.yaml
│   │       └── terraform-secret-template.yaml
│   └── catalog/
│       ├── kustomization.yaml
│       ├── tasks/
│       │   ├── git-clone.yaml
│       │   ├── cdktf-synth.yaml
│       │   └── cdktf-deploy.yaml
│       └── pipelines/
│           └── cdktf-deploy-pipeline.yaml
├── tenants/
│   └── (tenant-specific resources, if any)
├── bootstrap/
│   ├── bootstrap.sh                     # Main bootstrap script
│   └── README.md                        # Bootstrap documentation
└── docs/
    └── .kiro/docs/                      # Platform documentation
```

## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system—essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.*

