# Overview

## Purpose

The **Arbiter Pipeline Infrastructure** provides a lightweight GitOps platform using ArgoCD and Tekton for homelab Kubernetes clusters. The system enables self-service repository registration with automated tenant provisioning, CDKTF deployment pipelines, and self-upgrade capabilities through ArgoCD-based GitOps.

This repository contains:

1. **Bootstrap Script**: One-time initialization that creates cluster, installs core components (Tekton, ArgoCD), and generates all secrets for zero-touch convergence
2. **Platform Applications (App of Apps)**: Root ArgoCD Application that manages child Applications for each component layer
3. **Platform Manifests**: GitOps-managed platform configuration organized by layer (CRDs, Infrastructure, Controllers, Catalog, Authentication)
4. **Authentication System**: Authentik Identity Provider with Dex OIDC connector for centralized authentication and role-based access control
5. **Onboarding Controller**: Kubernetes controller for self-service repository onboarding
6. **Pipeline Catalog**: Shared, versioned Tekton Tasks and Pipelines for CDKTF deployments
7. **Tenant Templates**: Resource templates for tenant provisioning

## Archon Integration

This repository is ingested by the **Archon** RAG system, which reads all Markdown files under `.kiro/docs/` to build mental models for sourcing code and architectural information.

Documentation in this repository follows the Archon documentation contract defined in `CLAUDE.md` at the repo root.

## Key Concepts

### ArgoCD
GitOps continuous delivery tool that syncs Kubernetes resources from Git. ArgoCD manages all platform components declaratively, enabling self-upgrade capabilities.

### App of Apps Pattern
Architectural pattern where a root ArgoCD Application manages multiple child Applications. The platform uses this to organize components into layers (CRDs, Infrastructure, Controllers, Catalog).

### Tekton
Kubernetes-native pipeline execution framework that runs containerized build and deployment steps as Kubernetes pods.

### Bootstrap
One-time initialization script that creates the Kubernetes cluster, installs Tekton and ArgoCD, generates all secrets (PostgreSQL, Authentik, Dex, API tokens), and creates the platform root Application. After bootstrap, ArgoCD takes over all platform management. Bootstrap follows a zero-touch philosophy: run once and walk away.

### Authentication System
Centralized authentication and authorization infrastructure using Authentik as the Identity Provider (IdP) with Dex as an OIDC connector layer. Provides secure, role-based access to platform services (ArgoCD, Tekton Dashboard) with support for internal users and external identity providers (GitHub OAuth, LDAP, SAML).

### Authentik
Modern, self-hosted Identity Provider with web UI for user and group management. Provides OIDC authentication, supports external identity providers, and enables self-service user management without editing configuration files.

### Dex
OpenID Connect (OIDC) connector that acts as a proxy between Authentik and platform services. Provides a stable OIDC endpoint for service integration, simplifying the addition of new services without reconfiguring Authentik.

### Role-Based Access Control (RBAC)
Authorization model that grants permissions based on user group membership. The platform defines two primary roles: admins (full access to all platform services) and engineering (read-only access to platform services).

### Home Network Access
Platform services are exposed via Ingress resources with real hostnames (e.g., `https://auth.home.local`, `https://argocd.home.local`) for access from the home network. DNS configuration options include router/Pi-hole, hosts file, or mDNS.

### Config Sync Job
Kubernetes Job that orchestrates cross-system configuration between Authentik and Dex. Waits for Authentik to be ready, updates OIDC provider configuration via Authentik API, and scales Dex deployment. Provides deterministic convergence without timing-based hacks.

### Tenant
A product team with an isolated namespace and dedicated pipeline resources. Each tenant gets their own namespace with RBAC boundaries, resource limits, and network isolation.

### RepoBinding
Custom Resource Definition (CRD) for repository onboarding requests. Users create RepoBinding resources to provision tenant infrastructure.

### Onboarding Controller
Kubernetes controller that reconciles RepoBinding resources and provisions tenant namespaces, service accounts, RBAC, resource quotas, network policies, EventListeners, and Ingress.

### EventListener
Tekton Triggers component that receives GitHub webhooks and creates PipelineRuns. Each tenant gets their own EventListener with webhook secret validation.

### Pipeline Catalog
Shared Tekton Tasks and Pipelines maintained by the platform team in the `platform-system` namespace. Tenants reference catalog components in their pipeline definitions.

### CDKTF Pipelines
Cloud Development Kit for Terraform pipelines that automatically deploy infrastructure changes when code is merged to main.

### Webhook Secret
Cryptographically secure secret generated by the Onboarding Controller for GitHub webhook validation. Each tenant gets a unique webhook secret.

## Design Principles

1. **GitOps Native**: ArgoCD manages all platform components declaratively from Git
2. **Bootstrap Once, GitOps Forever**: One-time bootstrap script generates secrets and installs ArgoCD, then ArgoCD handles all application deployments and updates
3. **Zero-Touch Convergence**: Bootstrap generates all required secrets automatically and creates Authentik API token for deterministic platform convergence without manual steps
4. **Minimal Custom Code**: Leverage existing tools (ArgoCD, Tekton Triggers, Authentik, Dex) instead of custom webhook handlers or authentication systems
5. **Self-Service**: Users can onboard repositories by creating RepoBinding resources and manage users via Authentik web UI
6. **Tenant Isolation**: Strong RBAC, network, and resource isolation between tenants
7. **Webhook-Driven**: GitHub webhooks trigger pipelines via Tekton EventListeners
8. **Centralized Authentication**: Single sign-on (SSO) for all platform services via Authentik and Dex with role-based access control

## Quick Start

### Prerequisites
- Kubernetes cluster (1.24+) with RBAC enabled
- `kubectl` configured with cluster access
- GitHub organization with admin access

### Bootstrap the Platform

```bash
# Clone the repository
git clone https://github.com/bdchatham/ArbiterPipelineInfrastructure.git
cd ArbiterPipelineInfrastructure

# Run bootstrap
cd platform/bootstrap
./bootstrap.sh --cluster-name arbiter-platform --repo-url https://github.com/bdchatham/ArbiterPipelineInfrastructure
```

### Onboard a Repository

```bash
# Create a RepoBinding
kubectl apply -f - <<EOF
apiVersion: arbiter.io/v1alpha1
kind: RepoBinding
metadata:
  name: my-repo-binding
  namespace: platform-system
spec:
  repoOrg: "your-github-org"
  repoName: "your-repo"
  tenantName: "my-tenant"
  permissionProfile: "standard"
  ingressHost: "webhooks.example.com"
EOF

# Verify onboarding
kubectl get repobinding my-repo-binding -n platform-system
kubectl get namespace my-tenant
```

### Configure GitHub Webhook

After onboarding, configure the webhook in GitHub:

```bash
# Get webhook URL and secret from RepoBinding status
kubectl get repobinding my-repo-binding -n platform-system -o yaml

# Configure in GitHub:
# 1. Go to repository Settings → Webhooks → Add webhook
# 2. Payload URL: https://webhooks.example.com/my-tenant
# 3. Content type: application/json
# 4. Secret: (from RepoBinding status)
# 5. Events: Push events
# 6. Active: ✓
```

## Related Systems

This platform is designed for homelab deployment and provides CI/CD infrastructure for:
- **Archon**: Automated agent for repository management
- **Product Teams**: Multiple teams sharing the platform with isolation
- **Infrastructure-as-Code**: CDKTF deployments with Kubernetes backend for Terraform state

**Source**
- `CLAUDE.md`
- `.kiro/steering/archon-docs.md`
- `README.md`
- `.kiro/specs/argocd-tekton-platform/requirements.md`
- `.kiro/specs/argocd-tekton-platform/design.md`
- `.kiro/specs/dex-authentication-platform/requirements.md`
- `.kiro/specs/dex-authentication-platform/design.md`
- `platform/auth/README.md`
