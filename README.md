# Aphex Platform Infrastructure

ArgoCD + Tekton GitOps platform for homelab deployment providing shared pipeline infrastructure with tenant isolation.

## Overview

This repository defines and operates a shared CI/CD platform cluster using ArgoCD for GitOps and Tekton for pipeline execution. Product teams can run pipelines on shared infrastructure with logical separation through namespaces, RBAC, and policies. The platform is self-managing through ArgoCD, which automatically syncs all platform components from Git.

## Key Features

- **GitOps-First Architecture**: ArgoCD manages ALL platform components declaratively from Git after bootstrap
- **Zero-Touch Bootstrap**: Automated secret generation and configuration - run once and walk away
- **Centralized Authentication**: Authentik identity provider with Dex OIDC connector for all platform services
- **Home Network Access**: Services exposed via Ingress with real hostnames for browser-based authentication
- **Role-Based Access Control**: Admin and engineering roles with group-based permissions
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

### Authentication System

The platform includes a centralized authentication system with zero-touch bootstrap:

- [Authentication Overview](platform/auth/README.md) - Architecture, GitOps ownership, and setup guide
- [Secret Management](platform/auth/secrets/README.md) - Automated secret generation and rotation
- [Authentik Configuration](platform/auth/authentik/README.md) - Identity provider and Blueprint setup
- [Ingress Configuration](platform/auth/ingress/README.md) - DNS and TLS setup for home network access

**Key Features:**
- **Zero-touch bootstrap**: Automated secret generation - no manual configuration required
- **GitOps-managed**: ArgoCD owns all authentication manifests after bootstrap
- **Home network access**: Services exposed via Ingress with real hostnames (e.g., `https://auth.home.local`)
- **OIDC integration**: Dex connector provides seamless authentication for ArgoCD and Tekton Dashboard
- **Role-based access**: Admin and engineering groups with granular permissions

## Quick Start

### Prerequisites

- Kubernetes cluster (1.24+) with RBAC enabled
- `kubectl` configured with cluster access
- Ingress controller (nginx-ingress or traefik) deployed and accessible from home network
- DNS configured for `*.home.local` (or your chosen domain) pointing to Ingress controller IP
- `helm` 3.x installed (optional, for Ingress controller installation)

**For home network access:**
- Configure DNS in your router/Pi-hole OR add entries to `/etc/hosts` on each device
- See [Ingress Configuration](platform/auth/ingress/README.md) for detailed DNS setup instructions

### Bootstrap the Platform

The bootstrap script provides **zero-touch convergence**: run once and walk away. No manual configuration steps required.

1. **Clone this repository:**
   ```bash
   git clone https://github.com/your-org/aphex-pipeline-infrastructure.git
   cd aphex-pipeline-infrastructure
   ```

2. **Deploy Ingress controller (if not already present):**
   ```bash
   # For Kind clusters
   kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml
   
   # Wait for controller to be ready
   kubectl wait --namespace ingress-nginx \
     --for=condition=ready pod \
     --selector=app.kubernetes.io/component=controller \
     --timeout=90s
   ```

3. **Configure DNS for home network access:**
   
   Find your Ingress controller IP:
   ```bash
   kubectl get svc -n ingress-nginx ingress-nginx-controller
   ```
   
   Add DNS entries (choose one method):
   
   **Option A: Router/Pi-hole DNS (Recommended)**
   - Add A records in your router or Pi-hole:
     ```
     auth.home.local     → <INGRESS_IP>
     dex.home.local      → <INGRESS_IP>
     argocd.home.local   → <INGRESS_IP>
     tekton.home.local   → <INGRESS_IP>
     ```
   
   **Option B: Hosts file (per-device)**
   - Add to `/etc/hosts` on each device:
     ```bash
     <INGRESS_IP> auth.home.local
     <INGRESS_IP> dex.home.local
     <INGRESS_IP> argocd.home.local
     <INGRESS_IP> tekton.home.local
     ```
   
   See [Ingress Configuration](platform/auth/ingress/README.md) for detailed DNS setup.

4. **Set Cloudflare API token (for organization webhooks):**
   ```bash
   export CLOUDFLARE_API_TOKEN="your-cloudflare-api-token"
   ```
   
   This is required for creating organization webhook tunnels. Get your token from the Cloudflare dashboard with permissions: Zone.DNS (Edit), Account.Cloudflare Tunnel (Edit).

5. **Run bootstrap:**
   ```bash
   cd platform/bootstrap
   ./bootstrap.sh
   ```

   **What bootstrap does:**
   - Creates/selects Kubernetes cluster
   - Generates ALL secrets automatically (PostgreSQL, Authentik, Dex, API token)
   - Installs ArgoCD
   - Creates root ArgoCD Application
   - Waits for Authentik (deployed by ArgoCD) and creates API token
   - Prints access instructions
   
   **What bootstrap does NOT do:**
   - Deploy application workloads (ArgoCD does this)
   - Print secrets to stdout (use `--show-secrets` flag for debugging only)
   - Build or push container images (belongs in CI/dev workflow)
   
   **Zero-touch convergence**: After bootstrap completes, ArgoCD syncs all Applications automatically. Walk away and it works.

5. **Verify installation:**
   ```bash
   # Check ArgoCD Applications
   kubectl get applications -n argocd
   
   # Check all platform components
   kubectl get pods -n auth-system
   kubectl get pods -n tekton-pipelines
   kubectl get pods -n argocd
   ```

6. **Access platform services via Ingress:**

   **Authentik UI (for user management):**
   - Open `https://auth.home.local` in browser
   - Accept certificate warning (if using self-signed certificates)
   - Retrieve admin password:
     ```bash
     kubectl get secret authentik-secrets -n auth-system \
       -o jsonpath='{.data.admin-password}' | base64 -d
     ```
   - Login with username `admin` and retrieved password

   **ArgoCD UI:**
   - Open `https://argocd.home.local` in browser
   - Click "Login via Dex"
   - Authenticate with Authentik credentials
   - Admin users have full access, engineering users have read-only access

   **Tekton Dashboard:**
   - Open `https://tekton.home.local` in browser
   - Authenticate with Authentik credentials via Dex
   - Admin users have full access, engineering users have read-only access

   **Note**: All services use HTTPS with TLS certificates (self-signed by default). See [Ingress Configuration](platform/auth/ingress/README.md) for Let's Encrypt setup.

### Onboard a Repository

1. **Create a RepoBinding:**
   ```yaml
   apiVersion: platform.aphex/v1alpha1
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

### Manage Users

After bootstrap, manage users through the Authentik web UI:

1. **Access Authentik UI:**
   - Open `https://auth.home.local` in browser
   - Login with admin credentials

2. **Create users:**
   - Navigate to **Directory** → **Users**
   - Click **Create** and fill in user details
   - Assign users to groups:
     - `admins`: Full access to all platform services
     - `engineering`: Read-only access to platform services

3. **Users can immediately log in** to ArgoCD and Tekton Dashboard with their credentials (no pod restarts required)

For detailed user management instructions, see [Authentication System Documentation](platform/auth/README.md).

## GitOps-First Architecture

The platform follows a **GitOps-first architecture** where ArgoCD owns all declarative infrastructure after bootstrap:

### Bootstrap Responsibilities (Minimal)

The bootstrap script handles ONLY out-of-cluster concerns:
- Creates/selects Kubernetes cluster
- Generates and creates ALL secrets (PostgreSQL, Authentik, Dex, API token)
- Installs ArgoCD
- Creates root ArgoCD Application
- Waits for Authentik and creates API token (enables zero-touch convergence)

### ArgoCD Responsibilities (Everything Else)

After bootstrap, ArgoCD manages ALL application manifests:
- **platform-auth Application**: PostgreSQL, Redis, Authentik, Dex, Ingress, Config Sync Job
- **platform-tekton Application**: Tekton Pipelines, Triggers, Dashboard
- **platform-controllers Application**: Custom controllers and CRDs
- **platform-catalog Application**: Pipeline and task catalog

### Making Changes

To update the platform:
1. Edit manifests in `platform/` directories
2. Commit and push to Git
3. ArgoCD detects changes (via webhook or polling)
4. ArgoCD syncs changes to cluster automatically
5. Pods restart with new configuration if needed

**No manual kubectl apply required** - ArgoCD handles all deployments and updates.

## Archon Integration

This repository participates in the Archon RAG system. All documentation under `.kiro/docs/` is ingested to provide context for automated agents.

See [CLAUDE.md](CLAUDE.md) for the complete documentation contract.

## License

[Add your license]
# Test webhook trigger
