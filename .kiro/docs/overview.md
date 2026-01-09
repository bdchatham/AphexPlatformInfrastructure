# Overview

## Purpose

The **Arbiter Pipeline Infrastructure** provides a production-ready GitOps platform using ArgoCD and Tekton for Kubernetes clusters. The system enables self-service repository onboarding with automated tenant provisioning, centralized authentication, and bulletproof certificate management through a layered cert-manager architecture.

This repository contains:

1. **Bootstrap Script**: Zero-touch initialization that creates cluster, installs core components, generates all secrets, and achieves complete platform convergence automatically
2. **Layered cert-manager Architecture**: Wave-based deployment with webhook validation that eliminates manual intervention and timing issues
3. **Platform Applications (App of Apps)**: Root ArgoCD Application managing child Applications with proper dependency ordering via sync waves
4. **Authentication System**: Authentik Identity Provider with Dex OIDC connector providing centralized SSO for all platform services
5. **Onboarding Controller**: Kubernetes controller enabling self-service repository onboarding via CRDs
6. **Pipeline Catalog**: Shared, versioned Tekton Tasks and Pipelines for CI/CD workflows
7. **Tenant Isolation**: Namespace-per-team with RBAC, network policies, and resource quotas

## Archon Integration

This repository participates in the **Archon** RAG system, which ingests all Markdown files under `.kiro/docs/` to build mental models for automated agents and engineers.

Documentation follows the Archon contract defined in `CLAUDE.md` with exactly 6 stable documentation files optimized for RAG retrieval.

## Key Concepts

### GitOps Architecture
ArgoCD manages all platform components declaratively from Git using the app-of-apps pattern. After bootstrap, the platform is entirely self-managing with automatic drift correction and self-healing capabilities.

### Layered cert-manager Deployment
Revolutionary approach that eliminates the "webhook chicken-and-egg" problem through sync waves and PostSync validation. See [architecture.md](architecture.md) for detailed design.

### Zero-Touch Bootstrap
One-time initialization script that generates all secrets automatically and achieves complete platform convergence without manual intervention. See [operations.md](operations.md) for deployment steps.

### Centralized Authentication
Authentik Identity Provider with Dex OIDC connector provides SSO for all platform services. See [architecture.md](architecture.md) for authentication flow details.

### Tenant Isolation
Each product team receives a dedicated namespace with RBAC boundaries, resource quotas, and isolated pipeline execution. See [api.md](api.md) for onboarding details.

### Self-Service Onboarding
Users create RepoBinding resources to automatically provision tenant infrastructure. See [operations.md](operations.md) for registration procedures.

## Design Principles

1. **GitOps Native**: All platform components managed declaratively via ArgoCD
2. **Zero Manual Intervention**: Bootstrap achieves complete convergence automatically
3. **Bulletproof Dependencies**: Sync waves and validation hooks ensure proper component ordering
4. **Self-Healing**: ArgoCD automatically corrects configuration drift
5. **Production Ready**: Robust error handling, validation, and monitoring
6. **Maintainable**: Standard components with minimal customization
7. **Upgradeable**: Version-pinned components with clear upgrade paths
8. **Tenant Isolation**: Strong security boundaries between teams
9. **Self-Service**: Users manage repositories and authentication independently

## Architecture Highlights

### App-of-Apps Pattern
```
platform-root (ArgoCD Application)
├── Wave 0: platform-ingress-controller
├── Wave 1: platform-tekton
├── Wave 5: platform-crds, platform-rbac
├── Wave 10: platform-cert-manager, platform-auth
├── Wave 20: platform-cert-foundation, platform-controllers, platform-catalog
└── Wave 30: platform-ingress
```

### Authentication Flow
```
User → ArgoCD UI → Dex → Authentik → OIDC Token → ArgoCD Access
```

### Certificate Management Flow
```
cert-manager (Wave 10) → Webhook Validation → Certificates (Wave 20) → Ingress (Wave 30)
```

For detailed architecture, see [architecture.md](architecture.md).

## Quick Start

### Prerequisites
- Kubernetes cluster (1.24+) with RBAC enabled
- `kubectl` configured with cluster access
- Ingress controller deployed and accessible
- DNS configured for `*.home.local` (or your domain)

### Bootstrap the Platform

```bash
git clone https://github.com/bdchatham/ArbiterPipelineInfrastructure.git
cd ArbiterPipelineInfrastructure
./platform/bootstrap/bootstrap.sh
```

The bootstrap script will:
1. Create Kind cluster (or use existing)
2. Generate all secrets automatically
3. Install ArgoCD
4. Create root Application
5. Wait for complete platform convergence
6. Display access instructions

### Access Platform Services

After bootstrap completes:

**ArgoCD UI**: `https://argocd.home.local`
- Click "Login via Dex" → Authenticate with Authentik

**Authentik UI**: `https://auth.home.local`
- Login with admin credentials (displayed by bootstrap)

**Tekton Dashboard**: `https://tekton.home.local`
- Authenticate via Dex/Authentik

### Onboard a Repository

```yaml
apiVersion: platform.arbiter.io/v1alpha1
kind: RepoBinding
metadata:
  name: my-repo-binding
  namespace: pipeline-system
spec:
  repoOrg: "your-github-org"
  repoName: "your-repo"
  tenantName: "my-tenant"
  permissionProfile: "standard"
```

For detailed onboarding procedures, see [operations.md](operations.md).

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
