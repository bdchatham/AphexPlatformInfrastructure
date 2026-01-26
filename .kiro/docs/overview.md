# Overview

## Purpose

The **Aphex Pipeline Infrastructure** provides a production-ready GitOps platform using ArgoCD and Tekton for Kubernetes clusters. The system enables self-service repository onboarding with automated tenant provisioning, centralized authentication, and bulletproof certificate management through a layered cert-manager architecture.

This repository contains:

1. **Deployment Type Support**: Flexible bootstrap system supporting multiple deployment targets (Kind for development, K3s for production with GPU support)
2. **Bootstrap Script**: Zero-touch initialization that creates cluster, installs core components, generates all secrets, and achieves complete platform convergence automatically
3. **Layered cert-manager Architecture**: Wave-based deployment with webhook validation that eliminates manual intervention and timing issues
4. **External Secrets Operator**: Enables customers to create secrets in their organization namespace and reference them across their systems via ExternalSecret resources
5. **Platform Applications (App of Apps)**: Root ArgoCD Application managing child Applications with proper dependency ordering via sync waves
6. **Authentication System**: Authentik Identity Provider with Dex OIDC connector providing centralized SSO for all platform services
7. **Onboarding Controller**: Kubernetes controller enabling self-service repository onboarding via CRDs with AppProject isolation
8. **Pipeline Catalog**: Shared, versioned Tekton Tasks and Pipelines for CI/CD workflows
9. **Multi-Tenant Isolation**: Organization-level and pipeline-level namespaces with RBAC, network policies, resource quotas, and ArgoCD AppProject boundaries

## Archon Integration

This repository participates in the **Archon** RAG system, which ingests all Markdown files under `.kiro/docs/` to build mental models for automated agents and engineers.

Documentation follows the Archon contract defined in `CLAUDE.md` with exactly 6 stable documentation files optimized for RAG retrieval.

## Key Concepts

### GitOps Architecture
ArgoCD manages all platform components declaratively from Git using the app-of-apps pattern. After bootstrap, the platform is entirely self-managing with automatic drift correction and self-healing capabilities.

### Deployment Types
The platform supports multiple deployment targets with a unified bootstrap interface:

**Kind Deployment (Development)**:
- Local Kubernetes cluster via Kind
- Simulated GPU support with RuntimeClass
- nginx-ingress-controller for ingress
- Suitable for development and testing
- Bootstrap: `./bootstrap.sh --deployment kind`

**K3s Deployment (Production)**:
- Lightweight Kubernetes for production
- Real NVIDIA GPU support via GPU Operator
- Gateway API with external DNS
- Optimized for bare-metal and edge deployments
- Bootstrap: `./bootstrap.sh --deployment k3s`

**Deployment-Specific Resources**:
- Each deployment has its own directory: `platform/deployments/{kind|k3s}/`
- Deployment-specific ArgoCD Applications
- Deployment-specific bootstrap scripts
- Deployment-specific infrastructure (GPU, ingress, DNS)

**Source**: `bootstrap.sh`, `platform/deployments/kind/`, `platform/deployments/k3s/`

### External Secrets Management
Organizations automatically receive a ClusterSecretStore that enables customers to create secrets in their organization namespace and reference them across their systems.

**How it works**:
1. Organization controller provisions `ClusterSecretStore` named `org-{name}-store`
2. Customers create a Secret named `org-secrets` in their organization namespace
3. Customers create `ExternalSecret` resources in any namespace labeled with their organization
4. External Secrets Operator syncs secrets from `org-secrets` to target namespaces

**Example usage** (from ArchonKnowledgeBaseInfrastructure):
```yaml
apiVersion: external-secrets.io/v1
kind: ExternalSecret
metadata:
  name: knowledge-base-secrets
spec:
  secretStoreRef:
    name: org-archon-store
    kind: ClusterSecretStore
  target:
    name: knowledge-base-secrets
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

**Source**: `platform/platform-controller/controller/controllers/organization_controller.go` (provisionSecretStore), `platform/base/external-secrets/`

### AppProject Isolation
Each pipeline automatically receives an ArgoCD AppProject that enforces logical security boundaries for customer team resources.

**Scoping rules**:
- **Destinations**: Only `{pipelineName}` and `{pipelineName}-*` namespaces
- **Source repositories**: Only the specific GitHub repository for that pipeline
- **Cluster resources**: None (empty whitelist)
- **Namespace resources**: All resources allowed within scoped namespaces

**Benefits**:
- Prevents cross-pipeline Application deployments
- Enforces namespace boundaries
- Restricts source repositories
- Prevents cluster-scoped resource creation
- Logical isolation for customer teams

**Lifecycle**:
- Created during RepoBinding provisioning
- Updated if spec changes
- Deleted when RepoBinding is deleted (with finalizer cleanup)

**Source**: `platform/platform-controller/controller/controllers/repobinding_provisioners.go` (provisionArgoCDAppProject)

### Organizations
Organizations provide multi-tenant isolation with dedicated namespaces (`org-{name}`), EventListeners, and public webhook endpoints. Each organization gets a unique subdomain under arbiter-dev.com for GitHub webhook delivery through Cloudflare tunnels. Multiple pipelines can belong to a single organization, sharing the webhook infrastructure.

**Source**: `platform/platform-controller/controller/controllers/organization_controller.go`, `platform/crds/organization-crd.yaml`

### Public vs Local Domains
The platform uses two domain strategies:
- **arbiter-dev.com** (public): Organization webhook endpoints accessible from the internet via Cloudflare tunnels
- **home.local** (local): Authentication and platform services accessible only within the home network

### Layered cert-manager Deployment
Revolutionary approach that eliminates the "webhook chicken-and-egg" problem through sync waves and PostSync validation. See [architecture.md](architecture.md#layered-cert-manager-architecture) for detailed design.

### Zero-Touch Bootstrap
One-time initialization script that generates all secrets automatically and achieves complete platform convergence without manual intervention. See [operations.md](operations.md#deployment) for deployment steps.

### Centralized Authentication
Authentik Identity Provider with Dex OIDC connector provides SSO for all platform services. See [architecture.md](architecture.md#authentication-system-architecture) for authentication flow details.

### Multi-Tenant Isolation
Organizations (tenants) receive dedicated namespaces (`org-{name}`) with shared webhook infrastructure. Each pipeline within an organization gets its own namespace (`{pipeline-name}`) with RBAC boundaries, resource quotas, network policies, and isolated pipeline execution. See [api.md](api.md#repobinding-api) for onboarding details.

**Source**: `platform/platform-controller/controller/controllers/organization_controller.go`, `platform/platform-controller/controller/controllers/repobinding_controller.go`

### Self-Service Onboarding
Users create Organization or RepoBinding resources to automatically provision organization and pipeline infrastructure. The platform controller reconciles these CRDs and creates all necessary Kubernetes resources.

For operational procedures, see [operations.md](operations.md#organization-and-repository-registration).

**Source**: `platform/platform-controller/controller/controllers/organization_controller.go`, `platform/platform-controller/controller/controllers/repobinding_controller.go`

## Design Principles

1. **GitOps Native**: All platform components managed declaratively via ArgoCD
2. **Zero Manual Intervention**: Bootstrap achieves complete convergence automatically
3. **Bulletproof Dependencies**: Sync waves and validation hooks ensure proper component ordering
4. **Self-Healing**: ArgoCD automatically corrects configuration drift
5. **Production Ready**: Robust error handling, validation, and monitoring
6. **Maintainable**: Standard components with minimal customization
7. **Upgradeable**: Version-pinned components with clear upgrade paths
8. **Multi-Tenant Isolation**: Strong security boundaries between organizations and pipelines
9. **Self-Service**: Users manage repositories and authentication independently

## Architecture Highlights

### App-of-Apps Pattern
```
platform-root (ArgoCD Application)
├── Wave 0: platform-ingress-controller
├── Wave 1: platform-tekton
├── Wave 3: platform-gpu (k3s only)
├── Wave 5: platform-crds, platform-rbac
├── Wave 10: platform-cert-manager
├── Wave 15: platform-external-secrets
├── Wave 20: platform-cert-foundation, platform-auth, platform-controllers, platform-catalog
└── Wave 30: platform-ingress, platform-pipeline-resources, platform-gateway (k3s only), platform-external-dns (k3s only)
```

**Source**: `platform/base/argocd/apps/platform-*.yaml`, `platform/deployments/k3s/argocd/apps/platform-*.yaml`

### Authentication Flow
```
User → ArgoCD UI → Dex → Authentik → OIDC Token → ArgoCD Access
```

**Source**: `platform/auth/dex/configmap.yaml`, `platform/auth/authentik/blueprints-configmap.yaml`

### Certificate Management Flow
```
cert-manager (Wave 10) → Webhook Validation → Certificates (Wave 20) → Ingress (Wave 30)
```

**Source**: `platform/argocd/apps/platform-cert-manager.yaml`, `platform/argocd/apps/platform-cert-foundation.yaml`, `platform/argocd/apps/platform-ingress.yaml`

For detailed architecture, see [architecture.md](architecture.md).

## Quick Start

### Prerequisites
- Kubernetes cluster (1.24+) with RBAC enabled
- `kubectl` configured with cluster access
- Ingress controller deployed and accessible
- DNS configured for `*.home.local` (or your domain)

### Bootstrap the Platform

```bash
git clone https://github.com/bdchatham/AphexPlatformInfrastructure.git
cd AphexPlatformInfrastructure
./platform/bootstrap/bootstrap.sh
```

The bootstrap script will:
1. Create Kind cluster (or use existing)
2. Generate all secrets automatically
3. Install ArgoCD
4. Create root Application
5. Wait for complete platform convergence
6. Display access instructions

**Source**: `platform/bootstrap/bootstrap.sh`

### Access Platform Services

After bootstrap completes:

**ArgoCD UI**: `https://argocd.home.local`
- Click "Login via Dex" → Authenticate with Authentik

**Authentik UI**: `https://auth.home.local`
- Login with admin credentials (displayed by bootstrap)

**Tekton Dashboard**: `https://tekton.home.local`
- Authenticate via Dex/Authentik

### Onboard an Organization

```yaml
apiVersion: aphex.io/v1alpha1
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

This creates:
- Dedicated namespace: `org-acme-corp`
- Public webhook endpoint: `https://acme-corp.arbiter-dev.com`
- Cloudflare tunnel with DNS record
- EventListener for GitHub webhooks
- Organization admin RBAC

For detailed onboarding procedures, see [operations.md](operations.md#bootstrap-organization).
For Organization API details, see [api.md](api.md#organization-api).

**Source**: `platform/crds/organization-crd.yaml`, `platform/platform-controller/controller/controllers/organization_controller.go`

## System Benefits

### For Platform Engineers
- **Zero-touch deployment**: Bootstrap handles everything automatically
- **Self-healing**: ArgoCD corrects drift and failures
- **Maintainable**: Standard components with clear upgrade paths
- **Observable**: Complete visibility into all platform components

### For Product Teams
- **Self-service onboarding**: Create RepoBinding to get started
- **Isolated environments**: Secure namespace boundaries
- **Shared infrastructure**: Common pipeline catalog and authentication
- **Browser-based access**: No CLI configuration required

### For Operations
- **Production ready**: Robust error handling and validation
- **Scalable**: Supports multiple teams and repositories
- **Secure**: RBAC, network policies, and centralized authentication
- **Upgradeable**: Version-controlled with rollback capabilities

**Source**
- `CLAUDE.md` - Documentation contract and standards
- `README.md` - High-level project description
- `platform/bootstrap/bootstrap.sh` - Bootstrap implementation
- `platform/cert-manager/` - Layered cert-manager architecture
- `platform/auth/` - Authentication system components
- `platform/argocd/apps/` - ArgoCD application definitions
