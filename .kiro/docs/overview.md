# Overview

## Purpose

The **Arbiter Pipeline Infrastructure** provides a shared CI/CD platform cluster for multiple product teams using Jenkins X, Lighthouse, and Tekton. The system enables product teams to run pipelines on shared infrastructure with logical separation through namespaces, RBAC, and policies.

This repository contains:

1. **Platform Bootstrap**: Scripts and manifests for deploying Jenkins X on Kubernetes
2. **Onboarding Controller**: Kubernetes controller for self-service repository onboarding
3. **Golden Pipeline Catalog**: Shared, versioned Tekton Tasks and Pipelines
4. **Tenant Isolation**: Namespace-per-team with RBAC, resource quotas, and network policies
5. **CDKTF Support**: Built-in support for infrastructure-as-code deployments

## Archon Integration

This repository is ingested by the **Archon** RAG system, which reads all Markdown files under `.kiro/docs/` to build mental models for sourcing code and architectural information.

Documentation in this repository follows the Archon documentation contract defined in `CLAUDE.md` at the repo root.

## Key Concepts

### Jenkins X Platform
A Kubernetes-native CI/CD platform built on Tekton that provides Git-driven automation for building and deploying applications.

### Lighthouse
Git event handler component of Jenkins X that processes webhooks from GitHub and triggers pipeline executions based on repository events.

### Tekton
Kubernetes-native pipeline execution framework that runs containerized build and deployment steps as Kubernetes pods.

### Tenant
A product team with an isolated namespace and dedicated pipeline resources. Each tenant gets their own namespace with RBAC boundaries and resource limits.

### Repository Allowlist
Security boundary defining which repositories can trigger pipelines on the platform. Only allowlisted repositories can execute pipelines.

### Golden Pipeline Catalog
Shared, versioned Tekton Tasks and Pipelines maintained by the platform team. Tenants reference catalog components in their pipeline definitions.

### RepoBinding
Custom Resource Definition (CRD) for repository onboarding requests. Users create RepoBinding resources to provision tenant infrastructure.

### OIDC Authentication
OpenID Connect authentication for self-service onboarding. Users in the engineering OIDC group can create RepoBinding resources without cluster admin access.

### CDKTF Pipelines
Cloud Development Kit for Terraform pipelines that automatically deploy infrastructure changes when code is merged to main.

## Design Principles

1. **Self-Service**: OIDC-authenticated users can onboard repositories without admin access
2. **Isolation**: Each tenant gets dedicated namespace with least-privilege service account
3. **Inspectability**: Comprehensive logging and status reporting for automated agent reasoning
4. **Reproducibility**: Deterministic bootstrap and upgrade processes
5. **Security**: Allowlist-based access control, RBAC boundaries, and remote state management

## Quick Start

### Prerequisites
- Kubernetes cluster (1.24+) with RBAC enabled
- `kubectl` configured with cluster access
- `helm` 3.x installed
- GitHub organization with admin access

### Bootstrap the Platform

```bash
# Clone the repository
git clone https://github.com/your-org/arbiter-pipeline-infrastructure.git
cd arbiter-pipeline-infrastructure

# Set up GitHub App
export GITHUB_APP_ID="your-app-id"
export GITHUB_APP_INSTALLATION_ID="your-installation-id"

# Run bootstrap
cd platform/bootstrap
./bootstrap.sh
```

### Onboard a Repository

```bash
# Create a RepoBinding
kubectl apply -f - <<EOF
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
EOF

# Verify onboarding
kubectl get repobinding my-repo-binding -n pipeline-system
kubectl get namespace my-tenant
```

## Related Systems

This platform is designed for homelab deployment and provides CI/CD infrastructure for:
- **Archon**: Automated agent for repository management
- **Product Teams**: Multiple teams sharing the platform with isolation
- **Infrastructure-as-Code**: CDKTF deployments with remote state management

**Source**
- `CLAUDE.md`
- `.kiro/steering/archon-docs.md`
- `README.md`
- `.kiro/specs/jenkinsx-platform/requirements.md`
- `.kiro/specs/jenkinsx-platform/design.md`
