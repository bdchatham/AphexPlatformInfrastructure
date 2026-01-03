# Arbiter Pipeline Infrastructure

ArgoCD + Tekton GitOps platform for homelab deployment providing shared pipeline infrastructure with tenant isolation.

## Overview

This repository defines and operates a shared CI/CD platform cluster using ArgoCD for GitOps and Tekton for pipeline execution. Product teams can run pipelines on shared infrastructure with logical separation through namespaces, RBAC, and policies. The platform is self-managing through ArgoCD, which automatically syncs all platform components from Git.

## Key Features

- **GitOps Native**: ArgoCD manages all platform components declaratively from Git
- **Self-Service Onboarding**: Kubernetes controller provisions tenant resources automatically
- **Tenant Isolation**: Namespace-per-team with RBAC and network policies
- **Pipeline Catalog**: Shared, versioned Tekton tasks and pipelines
- **CDKTF Support**: Built-in support for infrastructure-as-code deployments
- **Self-Upgrading**: Platform upgrades itself via ArgoCD when changes are committed to Git
- **Agent-Friendly**: Highly inspectable for automated reasoning and remediation

## Documentation

Complete documentation is available in `.kiro/docs/`:

- [Overview](.kiro/docs/overview.md) - System purpose and key concepts
- [Architecture](.kiro/docs/architecture.md) - System design and components
- [Operations](.kiro/docs/operations.md) - Deployment and maintenance
- [API](.kiro/docs/api.md) - Onboarding API and contracts
- [Data Models](.kiro/docs/data-models.md) - Configuration structures
- [FAQ](.kiro/docs/faq.md) - Common questions and troubleshooting

## Quick Start

### Prerequisites

- Kubernetes cluster (1.24+) with RBAC enabled
- `kubectl` configured with cluster access
- `helm` 3.x installed
- GitHub organization with admin access

### Bootstrap the Platform

1. **Clone this repository:**
   ```bash
   git clone https://github.com/your-org/arbiter-pipeline-infrastructure.git
   cd arbiter-pipeline-infrastructure
   ```

2. **Set up GitHub App:**
   - Create a GitHub App in your organization with webhook permissions
   - Note the App ID and Installation ID
   - Download the private key

3. **Configure environment:**
   ```bash
   export GITHUB_APP_ID="your-app-id"
   export GITHUB_APP_INSTALLATION_ID="your-installation-id"
   ```

4. **Run bootstrap:**
   ```bash
   cd platform/bootstrap
   ./bootstrap.sh
   ```

5. **Verify installation:**
   ```bash
   kubectl get pods -n pipeline-system
   kubectl get pods -n tekton-pipelines
   ```

### Onboard a Repository

1. **Create a RepoBinding:**
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

2. **Apply the RepoBinding:**
   ```bash
   kubectl apply -f repobinding.yaml
   ```

3. **Verify onboarding:**
   ```bash
   kubectl get repobinding my-repo-binding -n pipeline-system
   kubectl get namespace my-tenant
   ```

For detailed instructions, see [Operations Guide](.kiro/docs/operations.md).

## Archon Integration

This repository participates in the Archon RAG system. All documentation under `.kiro/docs/` is ingested to provide context for automated agents.

See [CLAUDE.md](CLAUDE.md) for the complete documentation contract.

## License

[Add your license]
# Test webhook trigger
