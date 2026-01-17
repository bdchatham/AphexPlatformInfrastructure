# Authentication System

This directory contains the authentication and authorization infrastructure for the Aphex Platform Infrastructure platform. The system provides centralized identity management through Authentik with Dex as an OIDC connector layer, enabling secure, role-based access to platform services including ArgoCD and Tekton Dashboard.

## Architecture Overview

The authentication system consists of:

- **Authentik**: Primary Identity Provider (IdP) with web UI for user and group management
- **Dex**: OIDC connector layer that acts as a proxy between Authentik and platform services
- **PostgreSQL**: Database for Authentik user data and configuration
- **Redis**: Cache for Authentik sessions (optional but recommended)
- **Config Sync Job**: Kubernetes Job that orchestrates Authentik-Dex integration
- **Ingress Resources**: Expose services to home network with real hostnames

### Component Relationships

```
User Browser
    ↓
Platform Service (ArgoCD/Tekton)
    ↓
Dex OIDC Connector (https://dex.home.local)
    ↓
Authentik IdP (https://auth.home.local)
    ↓
PostgreSQL Database
```

## GitOps Ownership

**All authentication system components are managed by ArgoCD**, not by the bootstrap script. This follows the platform's GitOps-first architecture:

### What ArgoCD Manages

The `platform-auth` ArgoCD Application manages all manifests in this directory:

- PostgreSQL StatefulSet and Service
- Redis Deployment and Service
- Authentik Server and Worker Deployments
- Authentik Blueprints ConfigMap
- Dex Deployment, Service, and ConfigMap
- Config Sync Job and RBAC
- Ingress resources for all services

### What Bootstrap Manages

The bootstrap script (`platform/bootstrap/bootstrap.sh`) handles ONLY out-of-cluster concerns:

1. **Cluster creation/selection**: Creates Kind cluster or selects existing cluster
2. **Secret generation**: Generates ALL secrets automatically (PostgreSQL, Authentik, Dex, API token)
3. **ArgoCD installation**: Installs ArgoCD itself (one-time kubectl apply)
4. **Root Application**: Creates the root ArgoCD Application pointing to Git
5. **Authentik API token**: Waits for Authentik (deployed by ArgoCD) and creates API token

**Bootstrap does NOT:**
- Deploy application workloads (ArgoCD does this)
- Create namespaces other than auth-system (ArgoCD does this)
- Apply manifests in `platform/auth/` (ArgoCD does this)
- Build or push container images (belongs in CI/dev workflow)

### GitOps Workflow

To make changes to the authentication system:

1. Edit manifests in `platform/auth/` or `platform/integrations/`
2. Commit and push to Git
3. ArgoCD detects changes (via webhook or polling)
4. ArgoCD syncs changes to cluster automatically
5. Pods restart with new configuration if needed

**No manual kubectl apply required** - ArgoCD handles all deployments and updates.

## Zero-Touch Bootstrap

The platform supports **zero-touch convergence**: run `bootstrap.sh` once and walk away. No manual configuration steps required.

### Automated Secret Generation

Bootstrap automatically generates ALL required secrets:

- **PostgreSQL credentials**: Database password and superuser password
- **Authentik secrets**: Secret key and admin password
- **Dex client secret**: OIDC client secret for Authentik-Dex integration
- **Authentik API token**: Created via Authentik API after Authentik is ready

**Secrets are never printed to stdout by default.** Use `--show-secrets` flag for debugging (with warning).

### Bootstrap Workflow

```bash
./platform/bootstrap/bootstrap.sh

# What happens:
# 1. Creates/selects Kubernetes cluster
# 2. Creates auth-system namespace
# 3. Generates and creates ALL secrets
# 4. Installs ArgoCD
# 5. Creates root ArgoCD Application
# 6. Waits for Authentik (deployed by ArgoCD)
# 7. Creates Authentik API token via API
# 8. Prints access instructions
# 9. Done - ArgoCD handles the rest
```

After bootstrap completes:
- ArgoCD syncs all Applications (Tekton, auth-system, etc.)
- Config Sync Job runs automatically (has API token)
- Platform is fully functional

**Zero manual steps. Zero UI configuration. Just works.**

## Homelab Network Access Strategy

This system is designed for **homelab environments** where services are accessed from the home network. Services are exposed via Ingress with real hostnames that user browsers can reach.

### External URL Requirements

**CRITICAL**: All browser-facing URLs must use external hostnames, NOT internal Kubernetes DNS names.

**Correct (browser-reachable):**
- Authentik: `https://auth.home.local`
- Dex: `https://dex.home.local`
- ArgoCD: `https://argocd.home.local`
- Tekton Dashboard: `https://tekton.home.local`

**Incorrect (browser cannot reach):**
- ❌ `http://authentik.auth-system.svc.cluster.local:9000`
- ❌ `http://dex.auth-system.svc.cluster.local:5556`
- ❌ `http://localhost:8080` (only works with port-forward)

### Why External URLs Matter

OIDC authentication requires redirect URIs that user browsers can reach:

1. User accesses ArgoCD at `https://argocd.home.local`
2. ArgoCD redirects to Dex at `https://dex.home.local`
3. Dex redirects to Authentik at `https://auth.home.local`
4. User logs in to Authentik
5. Authentik redirects back to Dex at `https://dex.home.local/callback`
6. Dex redirects back to ArgoCD at `https://argocd.home.local/auth/callback`

If any of these URLs use internal DNS names, the browser cannot reach them and authentication fails.

### DNS Configuration

You must configure DNS so that `*.home.local` resolves to your Ingress controller's IP address.

**Option 1: Router/Pi-hole DNS (Recommended)**

Add A records in your home router or Pi-hole:

```
auth.home.local     → 192.168.1.100
dex.home.local      → 192.168.1.100
argocd.home.local   → 192.168.1.100
tekton.home.local   → 192.168.1.100
```

Replace `192.168.1.100` with your Ingress controller's LoadBalancer IP or NodePort IP.

**Option 2: Hosts File**

Add entries to `/etc/hosts` on each device that needs access:

```bash
# /etc/hosts
192.168.1.100 auth.home.local
192.168.1.100 dex.home.local
192.168.1.100 argocd.home.local
192.168.1.100 tekton.home.local
```

**Option 3: mDNS/Avahi**

Use `.local` domain with mDNS for automatic discovery (works on macOS, Linux with Avahi, Windows with Bonjour).

### Finding Your Ingress Controller IP

```bash
# For nginx-ingress
kubectl get svc -n ingress-nginx ingress-nginx-controller

# For traefik
kubectl get svc -n traefik traefik

# Look for EXTERNAL-IP (LoadBalancer) or use NodePort IP
```

### TLS Certificate Options

**Option 1: Self-Signed Certificates (Simplest for homelab)**

- Use cert-manager with `selfsigned` ClusterIssuer
- Users accept certificate warnings in browser
- No external dependencies
- Works immediately

**Option 2: Let's Encrypt with DNS-01 Challenge**

- Use cert-manager with `letsencrypt` ClusterIssuer
- Requires DNS provider API access (Cloudflare, Route53, etc.)
- Provides valid, trusted certificates
- Works even if cluster is not publicly accessible

See `platform/auth/ingress/README.md` for detailed TLS setup instructions.

## Accessing Authentik UI

After bootstrap completes and ArgoCD syncs the auth system:

1. **Ensure DNS is configured** (see DNS Configuration above)
2. **Open Authentik UI**: `https://auth.home.local`
3. **Accept certificate warning** (if using self-signed certificates)
4. **Retrieve admin password**:
   ```bash
   kubectl get secret authentik-secrets -n auth-system \
     -o jsonpath='{.data.admin-password}' | base64 -d
   ```
5. **Login with**:
   - Username: `admin`
   - Password: (from step 4)

## Adding Users via UI

After initial login, you can manage users through the Authentik web UI:

### Creating a New User

1. Navigate to **Directory** → **Users**
2. Click **Create**
3. Fill in user details:
   - Username (required, unique)
   - Email (required, unique)
   - Name (display name)
   - Password (or send password reset email)
4. Assign user to groups:
   - `admins`: Full access to all platform services
   - `engineering`: Read-only access to platform services
5. Click **Create**

**User can authenticate immediately** - no pod restarts or configuration changes required.

### Managing Groups

1. Navigate to **Directory** → **Groups**
2. View existing groups (`admins`, `engineering`)
3. Create new groups as needed
4. Assign users to groups
5. Groups are automatically included in OIDC tokens

### Changing Admin Password

1. Navigate to **Directory** → **Users**
2. Click on `admin` user
3. Click **Set password**
4. Enter new password
5. Click **Update**

## Configuring External Identity Providers

Authentik supports integration with external identity providers for federated authentication.

### GitHub OAuth (Example)

1. **Create GitHub OAuth App**:
   - Go to GitHub Settings → Developer settings → OAuth Apps
   - Click **New OAuth App**
   - Application name: `Platform Services`
   - Homepage URL: `https://auth.home.local`
   - Authorization callback URL: `https://auth.home.local/source/oauth/callback/github/`
   - Copy Client ID and Client Secret

2. **Configure in Authentik UI**:
   - Navigate to **Directory** → **Federation & Social login**
   - Click **Create** → **GitHub**
   - Enter Client ID and Client Secret
   - Configure organization/team filtering (optional)
   - Map GitHub teams to Authentik groups
   - Click **Create**

3. **Test**:
   - Logout of Authentik
   - Click **Login with GitHub** on login page
   - Authorize application
   - User is created in Authentik with mapped groups

### LDAP Integration

1. Navigate to **Directory** → **Federation & Social login**
2. Click **Create** → **LDAP**
3. Configure LDAP server details:
   - Server URI: `ldap://ldap.example.com`
   - Bind DN and password
   - Base DN for user search
   - User and group filters
4. Map LDAP attributes to Authentik user fields
5. Click **Create**

### SAML Integration

1. Navigate to **Directory** → **Federation & Social login**
2. Click **Create** → **SAML**
3. Configure SAML IdP details:
   - Metadata URL or upload metadata XML
   - Entity ID
   - SSO URL
   - Attribute mappings
4. Click **Create**

## Role-Based Access Control (RBAC)

The authentication system provides two default roles:

### Admin Role

**Group**: `admins`

**Permissions**:
- **ArgoCD**: Full access to all applications, repositories, and clusters
- **Tekton**: Full access to all pipelines, tasks, and runs across all namespaces
- **Authentik**: Superuser access to manage users, groups, and configuration

**Use case**: Platform operators who need full control

### Engineering Role

**Group**: `engineering`

**Permissions**:
- **ArgoCD**: Read-only access to applications, repositories, and clusters
- **Tekton**: Read-only access to pipelines, tasks, and runs
- **Authentik**: No access to Authentik UI (can only authenticate)

**Use case**: Developers who need visibility but not modification rights

### Creating Custom Roles

1. **Create new group in Authentik UI**:
   - Navigate to **Directory** → **Groups**
   - Click **Create**
   - Enter group name (e.g., `developers`)
   - Click **Create**

2. **Update ArgoCD RBAC policy**:
   - Edit `platform/integrations/argocd-rbac-policy.yaml`
   - Add group mapping and permissions
   - Commit and push to Git
   - ArgoCD syncs changes automatically

3. **Update Tekton RBAC policy**:
   - Edit `platform/integrations/tekton-rbac.yaml`
   - Create ClusterRole with desired permissions
   - Create ClusterRoleBinding for new group
   - Commit and push to Git
   - ArgoCD syncs changes automatically

## Directory Structure

```
platform/auth/
├── README.md                      # This file
├── postgresql/                    # PostgreSQL database
│   ├── statefulset.yaml
│   ├── service.yaml
│   └── secret.yaml.example
├── redis/                         # Redis cache (optional)
│   ├── deployment.yaml
│   └── service.yaml
├── authentik/                     # Authentik IdP
│   ├── server-deployment.yaml
│   ├── worker-deployment.yaml
│   ├── service.yaml
│   ├── blueprints-configmap.yaml
│   ├── secret.yaml.example
│   └── README.md
├── dex/                           # Dex OIDC connector
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── serviceaccount.yaml
│   ├── rbac.yaml
│   ├── configmap.yaml
│   └── README.md
├── config-sync/                   # Configuration orchestration Job
│   ├── job.yaml
│   ├── serviceaccount.yaml
│   ├── rbac.yaml
│   ├── configmap.yaml
│   └── README.md
├── ingress/                       # Ingress resources
│   ├── authentik-ingress.yaml
│   ├── dex-ingress.yaml
│   ├── argocd-ingress.yaml
│   ├── tekton-ingress.yaml
│   └── README.md
└── secrets/                       # Secret management
    └── README.md
```

## Component Documentation

- **PostgreSQL**: See `postgresql/README.md` for database management
- **Authentik**: See `authentik/README.md` for Blueprint structure and customization
- **Dex**: See `dex/README.md` for OIDC connector configuration
- **Config Sync Job**: See `config-sync/README.md` for orchestration details
- **Ingress**: See `ingress/README.md` for DNS and TLS setup
- **Secrets**: See `secrets/README.md` for secret generation and rotation

## Troubleshooting

### Authentication Fails

**Symptom**: User cannot log in to ArgoCD or Tekton Dashboard

**Possible causes**:
1. DNS not configured - browser cannot reach `dex.home.local` or `auth.home.local`
2. Redirect URI mismatch - check Dex config and Authentik OIDC provider
3. User not in correct group - check user group membership in Authentik UI

**Resolution**:
```bash
# Check DNS resolution
nslookup auth.home.local
nslookup dex.home.local

# Check Authentik OIDC discovery
curl https://auth.home.local/application/o/dex/.well-known/openid-configuration

# Check Dex OIDC discovery
curl https://dex.home.local/.well-known/openid-configuration

# Check user groups in Authentik
# Login to Authentik UI → Directory → Users → Select user → Groups tab
```

### Dex Pod CrashLoopBackOff

**Symptom**: Dex pod fails to start repeatedly

**Possible causes**:
1. Authentik not ready yet - Dex should start with replicas=0
2. Invalid Dex configuration - check ConfigMap syntax
3. Missing RBAC permissions - check ServiceAccount and Role

**Resolution**:
```bash
# Check Dex logs
kubectl logs -n auth-system deployment/dex

# Verify Dex starts with replicas=0
kubectl get deployment dex -n auth-system -o jsonpath='{.spec.replicas}'

# Check Config Sync Job status
kubectl get job auth-config-sync -n auth-system
kubectl logs -n auth-system job/auth-config-sync
```

### ArgoCD Not Syncing

**Symptom**: Changes to Git are not applied to cluster

**Possible causes**:
1. ArgoCD auto-sync disabled
2. Application in error state
3. Invalid YAML syntax in manifests

**Resolution**:
```bash
# Check Application status
kubectl get applications -n argocd

# Check Application details
kubectl describe application platform-auth -n argocd

# Manually trigger sync
kubectl patch application platform-auth -n argocd \
  --type merge -p '{"operation":{"initiatedBy":{"username":"admin"},"sync":{}}}'

# Or use ArgoCD UI
# https://argocd.home.local → Applications → platform-auth → Sync
```

### Secrets Not Found

**Symptom**: Pods fail to start with "secret not found" errors

**Possible causes**:
1. Bootstrap did not complete successfully
2. Secrets created in wrong namespace
3. Secret names don't match manifest references

**Resolution**:
```bash
# Check if secrets exist
kubectl get secrets -n auth-system

# Re-run bootstrap to regenerate secrets
./platform/bootstrap/bootstrap.sh

# Or manually create missing secrets (see secrets/README.md)
```

## Security Considerations

### Secret Management

- **Secrets are never committed to Git** - only `.example` templates
- **Secrets are never printed by default** - use `--show-secrets` flag only for debugging
- **Authentik API token has minimal permissions** - scoped to OAuth2 provider update only
- **Secrets are namespace-scoped** - only accessible within auth-system namespace

### RBAC Permissions

- **Dex ServiceAccount**: Namespace-scoped, read/write on Secrets and ConfigMaps (required for Kubernetes storage backend)
- **Config Sync Job ServiceAccount**: Namespace-scoped, can only scale Dex deployment
- **Platform service RBAC**: Cluster-scoped, enforces group-based access control

### Network Security

- **All services use HTTPS** - TLS required for OIDC security
- **Ingress controller required** - services not exposed via NodePort or LoadBalancer
- **Internal pod-to-pod communication** - uses ClusterIP services and internal DNS

## Next Steps

1. **Configure DNS**: Set up DNS resolution for `*.home.local` domains
2. **Deploy Ingress controller**: Install nginx-ingress or traefik if not already present
3. **Run bootstrap**: Execute `./platform/bootstrap/bootstrap.sh` for zero-touch setup
4. **Access Authentik UI**: Login and change admin password
5. **Create users**: Add users and assign to groups via Authentik UI
6. **Test authentication**: Login to ArgoCD and Tekton Dashboard with new users
7. **Configure external IdPs**: Integrate GitHub, LDAP, or SAML if needed

## References

- **Requirements**: `.kiro/specs/dex-authentication-platform/requirements.md`
- **Design**: `.kiro/specs/dex-authentication-platform/design.md`
- **Tasks**: `.kiro/specs/dex-authentication-platform/tasks.md`
- **Authentik Documentation**: https://goauthentik.io/docs/
- **Dex Documentation**: https://dexidp.io/docs/
- **ArgoCD OIDC**: https://argo-cd.readthedocs.io/en/stable/operator-manual/user-management/
