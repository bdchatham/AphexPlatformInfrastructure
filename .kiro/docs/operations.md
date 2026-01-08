# Operations

## Deployment

### Prerequisites

Before deploying the ArgoCD + Tekton platform, ensure you have:

1. **Kubernetes Cluster**: Version 1.24+ with RBAC enabled
2. **kubectl**: Configured with cluster access
3. **GitHub Organization**: With admin access for webhook configuration
4. **Cluster Features**: RBAC and NetworkPolicy support

### Bootstrap Process

The bootstrap process installs all platform components in the correct order. After bootstrap completes, ArgoCD takes over and manages all platform components via GitOps.

**Step 1: Run Bootstrap Script**

```bash
# Clone the repository
git clone https://github.com/bdchatham/ArbiterPipelineInfrastructure.git
cd ArbiterPipelineInfrastructure

# Run bootstrap
cd platform/bootstrap
./bootstrap.sh --cluster-name arbiter-platform --repo-url https://github.com/bdchatham/ArbiterPipelineInfrastructure
```

**Bootstrap Script Actions**:
1. Detects and cleans up existing JenkinsX installations (if present)
2. Creates Kubernetes cluster (Kind for local, configurable for others)
3. **Generates and creates ALL secrets (PostgreSQL, Authentik, Dex)**
4. **Creates auth-system namespace**
5. Installs Tekton Pipelines v0.65.0
6. Installs Tekton Triggers v0.29.0
7. Installs Tekton Triggers Core Interceptors v0.29.0
8. Installs ArgoCD
9. Creates platform namespaces (argocd, tekton-pipelines, platform-system)
10. Creates platform root ArgoCD Application
11. **Waits for Authentik to be ready (deployed by ArgoCD)**
12. **Creates Authentik API token via Authentik API**
13. **Stores API token in Kubernetes Secret**
14. Displays ArgoCD credentials and access instructions
15. **Prints commands to retrieve secrets (NOT the secrets themselves)**

**Note**: After bootstrap, all platform components (Tekton, auth-system, etc.) are managed by ArgoCD via GitOps. Bootstrap handles cluster setup, secret generation, and ArgoCD installation. ArgoCD handles everything else.

**Step 2: Verify Bootstrap**

```bash
# Check all platform components
kubectl get pods -n argocd
kubectl get pods -n tekton-pipelines
kubectl get pods -n platform-system

# Verify ArgoCD Application
kubectl get application platform-root -n argocd

# Check ArgoCD sync status
kubectl get application -n argocd
```

**Expected Output**:
- All pods in argocd namespace running
- All pods in tekton-pipelines namespace running
- platform-root Application syncing
- Child Applications (platform-crds, platform-infrastructure, platform-controllers, platform-catalog) created

**Step 3: Access ArgoCD UI**

```bash
# Get ArgoCD admin password
kubectl get secret argocd-initial-admin-secret -n argocd -o jsonpath='{.data.password}' | base64 -d

# Port-forward to ArgoCD server
kubectl port-forward svc/argocd-server -n argocd 8080:443

# Access UI at http://localhost:8080
# Username: admin
# Password: (from above command)
```

**Source**
- `platform/bootstrap/bootstrap.sh`

## Authentication System Validation

After bootstrap completes and ArgoCD syncs the authentication system, validate OIDC functionality:

### Validate OIDC Discovery

```bash
# Run OIDC discovery validation script
platform/scripts/validate-oidc-discovery.sh

# Manual validation
curl https://dex.home.local/.well-known/openid-configuration
curl https://dex.home.local/keys
```

### Validate RBAC Authorization

```bash
# Run RBAC validation script
platform/scripts/validate-rbac.sh

# Manual RBAC checks
kubectl auth can-i create pipelines.platform.dev --as=admin@platform.local --as-group=platform-admins
kubectl auth can-i create pipelines.platform.dev --as=alice@platform.local --as-group=platform-engineering -n user-alice
kubectl auth can-i create pipelines.platform.dev --as=alice@platform.local --as-group=platform-engineering -n auth-system
```

### Test Break-Glass Access

```bash
# Verify certificate-based admin access works
kubectl --kubeconfig /etc/kubernetes/admin.conf get nodes

# Test when OIDC is unavailable
kubectl scale deployment/dex --replicas=0 -n auth-system
kubectl --kubeconfig /etc/kubernetes/admin.conf get pods -n auth-system
kubectl scale deployment/dex --replicas=1 -n auth-system
```

### Access Authentication Services

**Authentik UI (User Management)**:
```bash
# Get admin password
kubectl get secret authentik-secrets -n auth-system -o jsonpath='{.data.admin-password}' | base64 -d

# Access at https://auth.home.local
# Username: admin
# Password: (from above command)
```

**ArgoCD with OIDC**:
```bash
# Access at https://argocd.home.local
# Click "Login via Dex"
# Authenticate with Authentik credentials
```

**Tekton Dashboard with OIDC**:
```bash
# Access at https://tekton.home.local
# Authenticate with Authentik credentials via Dex
```

**Source**
- `platform/scripts/validate-oidc-discovery.sh`
- `platform/scripts/validate-rbac.sh`
- `platform/auth/ingress/`

## Repository Registration

### Create RepoBinding

To onboard a repository to the platform, create a RepoBinding resource:

```bash
# Create RepoBinding
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
```

**Field Descriptions**:
- `repoOrg`: GitHub organization name
- `repoName`: Repository name
- `tenantName`: Kubernetes namespace name for this tenant (lowercase, alphanumeric, hyphens)
- `permissionProfile`: Permission level (`standard` or `elevated`)
- `ingressHost`: Hostname for webhook Ingress (optional, defaults to cluster ingress)

### Verify Onboarding

```bash
# Check RepoBinding status
kubectl get repobinding my-repo-binding -n platform-system
kubectl describe repobinding my-repo-binding -n platform-system

# Verify tenant namespace
kubectl get namespace my-tenant

# Verify service account
kubectl get serviceaccount pipeline-runner -n my-tenant

# Verify RBAC
kubectl get role,rolebinding -n my-tenant

# Verify resource limits
kubectl get resourcequota,limitrange -n my-tenant

# Verify network policy
kubectl get networkpolicy -n my-tenant

# Verify EventListener
kubectl get eventlistener -n my-tenant

# Verify Ingress
kubectl get ingress -n my-tenant
```

### Configure GitHub Webhook

After onboarding, configure the webhook in GitHub:

```bash
# Get webhook URL and secret from RepoBinding status
kubectl get repobinding my-repo-binding -n platform-system -o yaml

# Look for status.webhookURL and status.webhookSecret
```

**Configure in GitHub**:
1. Go to repository Settings → Webhooks → Add webhook
2. Payload URL: (from RepoBinding status.webhookURL)
3. Content type: application/json
4. Secret: (from RepoBinding status.webhookSecret)
5. Events: Push events
6. Active: ✓
7. Click "Add webhook"

**Source**
- `platform/crds/repobinding-crd.yaml`
- `platform/onboarding/controller/`

## Authentication System Operations

The authentication system provides centralized identity management through Authentik with Dex as an OIDC connector layer. All authentication components are managed by ArgoCD via GitOps.

### Accessing Authentik UI

After bootstrap completes and ArgoCD syncs the auth system:

**Step 1: Ensure DNS is configured**

Configure DNS so that `*.home.local` resolves to your Ingress controller's IP address.

```bash
# Find Ingress controller IP
kubectl get svc -n ingress-nginx ingress-nginx-controller

# Add DNS records (router/Pi-hole) or /etc/hosts entries:
# 192.168.1.100 auth.home.local
# 192.168.1.100 dex.home.local
# 192.168.1.100 argocd.home.local
# 192.168.1.100 tekton.home.local
```

**Step 2: Retrieve admin password**

```bash
kubectl get secret authentik-secrets -n auth-system \
  -o jsonpath='{.data.admin-password}' | base64 -d
```

**Step 3: Access Authentik UI**

- Open `https://auth.home.local` in browser
- Accept certificate warning (if using self-signed certificates)
- Login with username `admin` and password from Step 2

### User Management

#### Creating Users

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

Users can authenticate immediately - no pod restarts or configuration changes required.

#### Managing Groups

1. Navigate to **Directory** → **Groups**
2. View existing groups (`admins`, `engineering`)
3. Create new groups as needed
4. Assign users to groups
5. Groups are automatically included in OIDC tokens

#### Changing Admin Password

1. Navigate to **Directory** → **Users**
2. Click on `admin` user
3. Click **Set password**
4. Enter new password
5. Click **Update**

### DNS Configuration

DNS configuration is required for user browsers to reach services via hostnames. OIDC authentication requires redirect URIs that browsers can reach.

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

Add entries to `/etc/hosts` on each device:

```bash
# Linux/macOS
sudo nano /etc/hosts

# Add these lines:
192.168.1.100 auth.home.local
192.168.1.100 dex.home.local
192.168.1.100 argocd.home.local
192.168.1.100 tekton.home.local
```

**Verify DNS configuration**:

```bash
nslookup auth.home.local
nslookup dex.home.local
curl -k https://auth.home.local
```

### TLS Certificate Setup

All services use HTTPS with TLS certificates. Choose between self-signed (simplest) or Let's Encrypt (trusted certificates).

#### Self-Signed Certificates (Simplest)

1. **Install cert-manager**:
   ```bash
   kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml
   
   kubectl wait --for=condition=ready pod \
     -l app.kubernetes.io/instance=cert-manager \
     -n cert-manager \
     --timeout=90s
   ```

2. **Create self-signed ClusterIssuer**:
   ```bash
   cat <<EOF | kubectl apply -f -
   apiVersion: cert-manager.io/v1
   kind: ClusterIssuer
   metadata:
     name: selfsigned-issuer
   spec:
     selfSigned: {}
   EOF
   ```

3. **Accept certificate warnings in browser**:
   - Chrome: Click "Advanced" → "Proceed to auth.home.local (unsafe)"
   - Firefox: Click "Advanced" → "Accept the Risk and Continue"

#### Let's Encrypt with DNS-01 Challenge

Provides trusted certificates without browser warnings. Requires DNS provider API access.

1. **Install cert-manager** (same as above)

2. **Create DNS provider API token secret** (Cloudflare example):
   ```bash
   kubectl create secret generic cloudflare-api-token \
     -n cert-manager \
     --from-literal=api-token=YOUR_TOKEN_HERE
   ```

3. **Create Let's Encrypt ClusterIssuer**:
   ```bash
   cat <<EOF | kubectl apply -f -
   apiVersion: cert-manager.io/v1
   kind: ClusterIssuer
   metadata:
     name: letsencrypt-dns
   spec:
     acme:
       server: https://acme-v02.api.letsencrypt.org/directory
       email: your-email@example.com
       privateKeySecretRef:
         name: letsencrypt-dns-key
       solvers:
         - dns01:
             cloudflare:
               apiTokenSecretRef:
                 name: cloudflare-api-token
                 key: api-token
   EOF
   ```

4. **Update Ingress resources** to use Let's Encrypt issuer and your domain

5. **Commit and push to Git** - ArgoCD syncs changes automatically

**Verify certificates**:

```bash
kubectl get certificate -n auth-system
kubectl describe certificate authentik-tls -n auth-system
```

### Secret Rotation

Secrets should be rotated periodically for security. The authentication system supports secret rotation without downtime.

#### Rotating Authentik Admin Password

**Via Authentik UI (Recommended)**:

1. Login to Authentik UI
2. Navigate to **Directory** → **Users** → `admin`
3. Click **Set password**
4. Enter new password
5. Click **Update**
6. Update Kubernetes Secret:
   ```bash
   kubectl create secret generic authentik-secrets \
     -n auth-system \
     --from-literal=secret-key="$(kubectl get secret authentik-secrets -n auth-system -o jsonpath='{.data.secret-key}' | base64 -d)" \
     --from-literal=admin-password="NEW_PASSWORD" \
     --dry-run=client -o yaml | kubectl apply -f -
   ```

#### Rotating Dex Client Secret

1. **Generate new secret**:
   ```bash
   NEW_SECRET=$(openssl rand -base64 32)
   ```

2. **Update Kubernetes Secret**:
   ```bash
   kubectl create secret generic dex-secrets \
     -n auth-system \
     --from-literal=client-secret="$NEW_SECRET" \
     --dry-run=client -o yaml | kubectl apply -f -
   ```

3. **Update Authentik OIDC provider**:
   - Login to Authentik UI
   - Navigate to **Applications** → **Providers** → **Dex OIDC Provider**
   - Update **Client Secret** field
   - Click **Update**

4. **Restart Dex**:
   ```bash
   kubectl rollout restart deployment/dex -n auth-system
   ```

#### Rotating Authentik API Token

1. **Create new token via Authentik UI**:
   - Navigate to **Directory** → **Tokens** → **Create**
   - Set identifier: "config-sync-job-new"
   - Set intent: "API"
   - Copy token value

2. **Update Kubernetes Secret**:
   ```bash
   kubectl create secret generic authentik-api-token \
     -n auth-system \
     --from-literal=token="NEW_TOKEN_HERE" \
     --dry-run=client -o yaml | kubectl apply -f -
   ```

3. **Revoke old token**:
   - Navigate to **Directory** → **Tokens**
   - Find old token → **Delete**

### Config Sync Job Management

The Config Sync Job orchestrates Authentik-Dex integration. It runs automatically during bootstrap but can be manually triggered.

#### Check Job Status

```bash
# Check Job status
kubectl get job auth-config-sync -n auth-system

# Check Job pod status
kubectl get pods -n auth-system -l app=auth-config-sync

# View Job logs
kubectl logs -n auth-system -l app=auth-config-sync
```

#### Manually Trigger Job

To re-run the Job (e.g., after fixing a configuration issue):

```bash
# Delete existing Job
kubectl delete job auth-config-sync -n auth-system

# ArgoCD will recreate the Job automatically
# Or manually apply:
kubectl apply -f platform/auth/config-sync/job.yaml

# Watch Job progress
kubectl logs -n auth-system -l app=auth-config-sync -f
```

#### Verify Job Completion

```bash
# Check if Job completed successfully
kubectl get job auth-config-sync -n auth-system -o jsonpath='{.status.conditions[?(@.type=="Complete")].status}'
# Should output: True

# Get detailed Job status
kubectl describe job auth-config-sync -n auth-system
```

### GitOps Workflow for Authentication Changes

All authentication system components are managed by ArgoCD. To make changes:

**Step 1: Update manifests in Git**

```bash
# Clone repository
git clone https://github.com/bdchatham/ArbiterPipelineInfrastructure.git
cd ArbiterPipelineInfrastructure

# Update authentication manifests
# Example: Update Authentik image version
vi platform/auth/authentik/server-deployment.yaml

# Commit changes
git add .
git commit -m "Update Authentik to v2024.1.0"
git push
```

**Step 2: ArgoCD detects and syncs changes**

ArgoCD polls Git every 3 minutes and detects changes automatically.

```bash
# Watch ArgoCD sync status
kubectl get application platform-auth -n argocd -w

# Or view in ArgoCD UI
# https://argocd.home.local
```

**Step 3: Verify changes**

```bash
# Check pod status
kubectl get pods -n auth-system

# Check ArgoCD sync status
kubectl get application platform-auth -n argocd

# View sync details
kubectl describe application platform-auth -n argocd
```

**No manual kubectl apply required** - ArgoCD handles all deployments and updates.

### Authentication Troubleshooting

#### Authentication Fails

**Symptom**: User cannot log in to ArgoCD or Tekton Dashboard

**Diagnosis**:

```bash
# Check DNS resolution
nslookup auth.home.local
nslookup dex.home.local

# Check Authentik OIDC discovery
curl https://auth.home.local/application/o/dex/.well-known/openid-configuration

# Check Dex OIDC discovery
curl https://dex.home.local/.well-known/openid-configuration

# Check user groups in Authentik UI
# Login → Directory → Users → Select user → Groups tab
```

**Resolution**:
- Configure DNS if resolution fails
- Verify redirect URI configuration in Dex and Authentik
- Verify user is in correct group (`admins` or `engineering`)

#### Dex Pod CrashLoopBackOff

**Symptom**: Dex pod fails to start repeatedly

**Diagnosis**:

```bash
# Check Dex logs
kubectl logs -n auth-system deployment/dex

# Verify Dex starts with replicas=0
kubectl get deployment dex -n auth-system -o jsonpath='{.spec.replicas}'

# Check Config Sync Job status
kubectl get job auth-config-sync -n auth-system
kubectl logs -n auth-system job/auth-config-sync
```

**Resolution**:
- Verify Authentik is running and healthy
- Check Dex ConfigMap for syntax errors
- Verify Config Sync Job completed successfully
- Re-run Config Sync Job if needed

#### ArgoCD Not Syncing Auth Changes

**Symptom**: Changes to Git are not applied to cluster

**Diagnosis**:

```bash
# Check Application status
kubectl get application platform-auth -n argocd

# Check Application details
kubectl describe application platform-auth -n argocd

# Check for sync errors
kubectl get application platform-auth -n argocd -o json | \
  jq '.status.conditions[] | select(.type=="SyncError")'
```

**Resolution**:
- Verify ArgoCD auto-sync is enabled
- Check for invalid YAML syntax in manifests
- Manually trigger sync:
  ```bash
  kubectl patch application platform-auth -n argocd \
    --type merge -p '{"operation":{"initiatedBy":{"username":"admin"},"sync":{}}}'
  ```

#### Secrets Not Found

**Symptom**: Pods fail to start with "secret not found" errors

**Diagnosis**:

```bash
# Check if secrets exist
kubectl get secrets -n auth-system

# Expected secrets:
# - authentik-postgresql
# - authentik-secrets
# - dex-secrets
# - authentik-api-token
```

**Resolution**:
- Re-run bootstrap to regenerate secrets:
  ```bash
  ./platform/bootstrap/bootstrap.sh
  ```
- Or manually create missing secrets (see `platform/auth/secrets/README.md`)

**Source**
- `platform/auth/README.md`
- `platform/auth/secrets/README.md`
- `platform/auth/ingress/README.md`
- `platform/auth/config-sync/README.md`
- `platform/bootstrap/bootstrap.sh`

## ArgoCD-Based Upgrade Workflow

The platform upgrades itself via ArgoCD when manifests change in Git. No manual kubectl apply or custom upgrade scripts are needed.

### Update Platform Components

**Step 1: Update Manifests in Git**

```bash
# Clone platform repository
git clone https://github.com/bdchatham/ArbiterPipelineInfrastructure.git
cd ArbiterPipelineInfrastructure

# Update component manifests
# Example: Update controller image
vi platform/onboarding/controller-deployment.yaml  # Update image tag

# Commit changes
git add .
git commit -m "Update onboarding controller to v1.1.0"
git push
```

**Step 2: ArgoCD Detects Changes**

ArgoCD polls Git every 3 minutes (default) and detects changes automatically.

```bash
# Watch ArgoCD sync status
kubectl get application -n argocd -w

# Or view in ArgoCD UI
# http://localhost:8080
```

**Step 3: ArgoCD Syncs Changes**

ArgoCD automatically syncs changes based on sync policy:
- Automated sync: Changes applied automatically
- Self-heal: Drift corrected automatically
- Prune: Removed resources deleted automatically

```bash
# Check sync status
kubectl get application platform-root -n argocd

# View sync details
kubectl describe application platform-root -n argocd

# Check child Applications
kubectl get application -n argocd
```

**Step 4: Verify Upgrade**

```bash
# Check component versions
kubectl get deployment -n platform-system -o wide

# Check pod status
kubectl get pods -n platform-system

# Check ArgoCD sync status
kubectl get application -n argocd
```

### Manual Sync (If Needed)

If automatic sync is disabled or you want to sync immediately:

```bash
# Sync via kubectl
kubectl patch application platform-root -n argocd --type merge -p '{"operation":{"initiatedBy":{"username":"admin"},"sync":{"revision":"HEAD"}}}'

# Or sync via ArgoCD CLI
argocd app sync platform-root

# Or sync via ArgoCD UI
# Click "Sync" button in UI
```

**Source**
- `platform/argocd/apps/platform-root.yaml`

## Monitoring

### Platform Health Checks

```bash
# Check ArgoCD
kubectl get pods -n argocd

# Check Tekton controllers
kubectl get pods -n tekton-pipelines

# Check onboarding controller
kubectl get pods -n platform-system -l app=onboarding-controller

# Check ArgoCD Applications
kubectl get application -n argocd
```

### ArgoCD Sync Status

```bash
# List all Applications
kubectl get application -n argocd

# Get Application sync status
kubectl get application platform-root -n argocd -o jsonpath='{.status.sync.status}'

# View sync details
kubectl describe application platform-root -n argocd

# Check for sync errors
kubectl get application -n argocd -o json | jq '.items[] | select(.status.sync.status != "Synced") | {name: .metadata.name, status: .status.sync.status, message: .status.conditions[0].message}'
```

### Pipeline Execution Monitoring

```bash
# List all PipelineRuns
kubectl get pipelineruns --all-namespaces

# Get PipelineRun details
kubectl describe pipelinerun <name> -n <tenant-namespace>

# View PipelineRun logs
kubectl logs -n <tenant-namespace> -l tekton.dev/pipelineRun=<name>

# Watch PipelineRun status
kubectl get pipelinerun <name> -n <tenant-namespace> -w
```

### EventListener Logs

```bash
# View EventListener logs for a tenant
kubectl logs -n <tenant-namespace> -l eventlistener=github-listener --tail=100

# Stream EventListener logs
kubectl logs -n <tenant-namespace> -l eventlistener=github-listener -f

# Search for specific webhook events
kubectl logs -n <tenant-namespace> -l eventlistener=github-listener | grep "webhook"
```

### Onboarding Controller Logs

```bash
# View controller logs
kubectl logs -n platform-system -l app=onboarding-controller --tail=100

# Stream controller logs
kubectl logs -n platform-system -l app=onboarding-controller -f

# Search for specific RepoBinding
kubectl logs -n platform-system -l app=onboarding-controller | grep "repobinding-name"
```

**Source**
- `platform/argocd/apps/`
- `platform/onboarding/controller-deployment.yaml`

## Troubleshooting

### EventListener CrashLoopBackOff

**Symptoms**: EventListener pod crashes with "empty caBundle in clusterInterceptor spec" error.

**Diagnosis**:

```bash
# Check EventListener pod status
kubectl get pods -n <tenant-namespace> -l eventlistener=github-listener

# Check EventListener logs
kubectl logs -n <tenant-namespace> -l eventlistener=github-listener --tail=50

# Check if ClusterInterceptors exist
kubectl get clusterinterceptors

# Check if Core Interceptors deployment exists
kubectl get deployment tekton-triggers-core-interceptors -n tekton-pipelines
```

**Resolution**:

This error occurs when Tekton Triggers Core Interceptors are not installed. The Core Interceptors provide ClusterInterceptor resources (github, gitlab, cel, etc.) that EventListeners need.

```bash
# Install Core Interceptors
kubectl apply -f https://github.com/tektoncd/triggers/releases/download/v0.29.0/interceptors.yaml

# Verify ClusterInterceptors are created
kubectl get clusterinterceptors

# Delete EventListener pod to restart
kubectl delete pod -n <tenant-namespace> -l eventlistener=github-listener

# Verify EventListener is running
kubectl get pods -n <tenant-namespace> -l eventlistener=github-listener
```

**Prevention**: Ensure bootstrap script installs Core Interceptors, or ensure `platform-tekton` ArgoCD Application includes interceptors.yaml.

### EventListener RBAC Permission Errors

**Symptoms**: EventListener pod logs show "cannot list resource clusterinterceptors" or "cannot list resource clustertriggerbindings" errors.

**Diagnosis**:

```bash
# Check EventListener logs
kubectl logs -n <tenant-namespace> -l eventlistener=github-listener --tail=50

# Check if ClusterRole exists for tenant
kubectl get clusterrole pipeline-runner-<tenant-name>

# Check if ClusterRoleBinding exists for tenant
kubectl get clusterrolebinding pipeline-runner-<tenant-name>
```

**Resolution**:

EventListener pods need cluster-scoped read permissions for ClusterInterceptor and ClusterTriggerBinding resources. The onboarding controller should provision these automatically.

```bash
# Check if controller provisioned cluster-scoped RBAC
kubectl describe clusterrole pipeline-runner-<tenant-name>
kubectl describe clusterrolebinding pipeline-runner-<tenant-name>

# If missing, delete and recreate RepoBinding to trigger reprovisioning
kubectl delete repobinding <name> -n platform-system
kubectl apply -f repobinding.yaml

# Verify cluster-scoped RBAC was created
kubectl get clusterrole pipeline-runner-<tenant-name>
kubectl get clusterrolebinding pipeline-runner-<tenant-name>

# Delete EventListener pod to restart with new permissions
kubectl delete pod -n <tenant-namespace> -l eventlistener=github-listener
```

**Prevention**: Ensure onboarding controller has permissions to create ClusterRoles and ClusterRoleBindings (check `platform/onboarding/controller-rbac.yaml`).

### Repository Not Triggering Pipelines

**Diagnosis**:

```bash
# Check if EventListener exists
kubectl get eventlistener -n <tenant-namespace>

# Check EventListener logs for webhook events
kubectl logs -n <tenant-namespace> -l eventlistener=github-listener | grep "webhook"

# Check if Ingress exists
kubectl get ingress -n <tenant-namespace>

# Check if webhook secret exists
kubectl get secret webhook-<tenant-name> -n <tenant-namespace>
```

**Resolution**:
1. Verify EventListener is running
2. Verify Ingress is configured correctly
3. Verify GitHub webhook is configured with correct URL and secret
4. Check EventListener logs for error messages

### PipelineRun Failures

**Diagnosis**:

```bash
# Get PipelineRun status
kubectl get pipelinerun <name> -n <tenant-namespace>

# Get detailed status
kubectl describe pipelinerun <name> -n <tenant-namespace>

# Get pod logs
kubectl logs -n <tenant-namespace> -l tekton.dev/pipelineRun=<name>

# Check pod events
kubectl get events -n <tenant-namespace> --sort-by='.lastTimestamp'
```

**Common Issues**:

1. **Git clone failure**: Check repository access
2. **CDKTF synth failure**: Check Node.js dependencies and syntax errors
3. **CDKTF deploy failure**: Check Terraform state and permissions
4. **RBAC denial**: Check service account permissions

### Onboarding Failures

**Diagnosis**:

```bash
# Check RepoBinding status
kubectl get repobinding <name> -n platform-system
kubectl describe repobinding <name> -n platform-system

# Check controller logs
kubectl logs -n platform-system -l app=onboarding-controller | grep "<name>"
```

**Common Issues**:

1. **Invalid namespace pattern**: Namespace name doesn't match pattern
2. **RBAC failure**: Controller lacks permissions to create resources
3. **EventListener creation failed**: Check Tekton Triggers installation
4. **Ingress creation failed**: Check Ingress controller installation

### ArgoCD Sync Failures

**Diagnosis**:

```bash
# Check Application sync status
kubectl get application -n argocd

# Get sync error details
kubectl describe application <name> -n argocd

# View Application events
kubectl get events -n argocd --field-selector involvedObject.name=<name>

# Check ArgoCD controller logs
kubectl logs -n argocd -l app.kubernetes.io/name=argocd-application-controller
```

**Common Issues**:

1. **Invalid manifest**: YAML syntax errors in Git
2. **Resource conflicts**: Resource already exists with different configuration
3. **RBAC denial**: ArgoCD lacks permissions to create resources
4. **Git connection failure**: ArgoCD cannot access Git repository

**Source**
- `platform/argocd/apps/`
- `platform/onboarding/controller/`
- `platform/catalog/`

## Disaster Recovery

### Backup Platform Configuration

```bash
# Backup RepoBindings
kubectl get repobindings -n platform-system -o yaml > repobindings-backup.yaml

# Backup ArgoCD Applications
kubectl get applications -n argocd -o yaml > applications-backup.yaml

# Backup platform manifests (already in Git)
# No backup needed - Git is the source of truth
```

### Restore Platform

**Step 1: Re-run Bootstrap**

```bash
# Run bootstrap on new cluster
cd platform/bootstrap
./bootstrap.sh --cluster-name arbiter-platform --repo-url https://github.com/bdchatham/ArbiterPipelineInfrastructure
```

**Step 2: Wait for ArgoCD to Sync**

ArgoCD will automatically sync all platform components from Git.

```bash
# Watch ArgoCD sync
kubectl get application -n argocd -w

# Verify all Applications are synced
kubectl get application -n argocd
```

**Step 3: Restore RepoBindings**

```bash
# Restore RepoBindings
kubectl apply -f repobindings-backup.yaml

# Verify onboarding
kubectl get repobindings -n platform-system
kubectl get namespaces -l platform.arbiter.io/managed-by=onboarding-controller
```

**Step 4: Verify Platform**

```bash
# Check all components
kubectl get pods -n argocd
kubectl get pods -n tekton-pipelines
kubectl get pods -n platform-system

# Check tenant namespaces
kubectl get namespaces -l platform.arbiter.io/managed-by=onboarding-controller

# Check ArgoCD sync status
kubectl get application -n argocd
```

**Source**
- `platform/bootstrap/bootstrap.sh`
- `.kiro/specs/argocd-tekton-platform/requirements.md` (Requirement 12.1, 12.4)

## Common Troubleshooting Scenarios

### Webhook Not Delivered

**Symptoms**: No PipelineRun created after merge to main.

**Diagnosis**:

```bash
# Check EventListener logs
kubectl logs -n <tenant-namespace> -l eventlistener=github-listener --tail=100

# Check Ingress configuration
kubectl get ingress -n <tenant-namespace> -o yaml

# Check webhook secret
kubectl get secret webhook-<tenant-name> -n <tenant-namespace>

# Check GitHub webhook delivery logs
# Go to GitHub repository Settings → Webhooks → Recent Deliveries
```

**Resolution**:
1. Verify Ingress is accessible from GitHub
2. Verify webhook secret matches GitHub configuration
3. Verify EventListener is running
4. Check GitHub webhook delivery logs for errors

### ArgoCD Not Syncing

**Symptoms**: Changes committed to Git but ArgoCD not syncing.

**Diagnosis**:

```bash
# Check Application sync status
kubectl get application platform-root -n argocd

# Check ArgoCD controller logs
kubectl logs -n argocd -l app.kubernetes.io/name=argocd-application-controller --tail=100

# Check Git repository connectivity
kubectl exec -n argocd -it <argocd-repo-server-pod> -- git ls-remote https://github.com/bdchatham/ArbiterPipelineInfrastructure
```

**Resolution**:
1. Verify ArgoCD can access Git repository
2. Verify sync policy is configured (automated sync enabled)
3. Manually trigger sync if needed
4. Check for manifest errors in Git

### Onboarding Controller Not Reconciling

**Symptoms**: RepoBinding created but status remains "Pending".

**Diagnosis**:

```bash
# Check controller logs
kubectl logs -n platform-system -l app=onboarding-controller --tail=100

# Check controller pod status
kubectl get pods -n platform-system -l app=onboarding-controller

# Check RepoBinding status
kubectl describe repobinding <name> -n platform-system
```

**Resolution**:
1. Verify controller is running
2. Check controller logs for errors
3. Verify controller has RBAC permissions
4. Restart controller if needed: `kubectl rollout restart deployment onboarding-controller -n platform-system`

**Source**
- `platform/onboarding/controller/`
- `platform/argocd/apps/`
- `platform/tenancy/templates/`

## Maintenance

### Update Tekton

Tekton is managed by ArgoCD after bootstrap. To update Tekton versions:

```bash
# Update Tekton versions in kustomization
vi platform/tekton/kustomization.yaml

# Update resource URLs to new versions
# Example: Update to Tekton Pipelines v0.66.0
resources:
  - https://github.com/tektoncd/pipeline/releases/download/v0.66.0/release.yaml
  - https://github.com/tektoncd/triggers/releases/download/v0.30.0/release.yaml
  - https://github.com/tektoncd/triggers/releases/download/v0.30.0/interceptors.yaml

# Commit changes
git add platform/tekton/kustomization.yaml
git commit -m "Update Tekton to v0.66.0"
git push

# ArgoCD will automatically sync and update Tekton
# Watch sync status
kubectl get application platform-tekton -n argocd -w

# Verify updates
kubectl get pods -n tekton-pipelines
```

**Note**: The bootstrap script installs Tekton initially, but ArgoCD manages updates from Git. This enables GitOps-based Tekton upgrades.

### Update ArgoCD

```bash
# Update ArgoCD version in bootstrap script
vi platform/bootstrap/bootstrap.sh

# Update version URL
# ArgoCD: https://raw.githubusercontent.com/argoproj/argo-cd/v2.9.0/manifests/install.yaml

# Commit changes
git add platform/bootstrap/bootstrap.sh
git commit -m "Update ArgoCD to v2.9.0"
git push

# ArgoCD will NOT automatically update itself (installed by bootstrap)
# To update ArgoCD, manually apply new manifests:
kubectl apply -f https://raw.githubusercontent.com/argoproj/argo-cd/v2.9.0/manifests/install.yaml
```

### Update Onboarding Controller

```bash
# Build new version
cd platform/onboarding/controller
docker build -t your-registry/onboarding-controller:v1.1.0 .
docker push your-registry/onboarding-controller:v1.1.0

# Update deployment manifest
vi platform/onboarding/controller-deployment.yaml
# Update image tag to v1.1.0

# Commit changes
git add platform/onboarding/controller-deployment.yaml
git commit -m "Update onboarding controller to v1.1.0"
git push

# ArgoCD will automatically sync and update the controller
# Watch sync status
kubectl get application platform-controllers -n argocd -w
```

### Update Pipeline Catalog

```bash
# Update catalog resources
vi platform/catalog/tasks/cdktf-deploy.yaml
# Make changes

# Commit changes
git add platform/catalog/
git commit -m "Update CDKTF deploy task"
git push

# ArgoCD will automatically sync and update the catalog
# Watch sync status
kubectl get application platform-catalog -n argocd -w

# Verify updates
kubectl get tasks -n platform-system
```

### Clean Up Old PipelineRuns

```bash
# List old PipelineRuns
kubectl get pipelineruns --all-namespaces --sort-by=.metadata.creationTimestamp

# Delete PipelineRuns older than 30 days
kubectl get pipelineruns --all-namespaces -o json | \
  jq -r '.items[] | select(.metadata.creationTimestamp < "'$(date -d '30 days ago' -Iseconds)'") | "\(.metadata.namespace) \(.metadata.name)"' | \
  xargs -n2 kubectl delete pipelinerun -n
```

**Source**
- `platform/bootstrap/bootstrap.sh`
- `platform/onboarding/controller-deployment.yaml`
- `platform/catalog/`

## Security

### Rotate Webhook Secrets

Webhook secrets are generated by the Onboarding Controller and stored in tenant namespaces. To rotate:

```bash
# Delete existing secret
kubectl delete secret webhook-<tenant-name> -n <tenant-namespace>

# Delete and recreate RepoBinding to regenerate secret
kubectl delete repobinding <name> -n platform-system
kubectl apply -f repobinding.yaml

# Get new webhook secret from RepoBinding status
kubectl get repobinding <name> -n platform-system -o yaml

# Update GitHub webhook with new secret
```

### Audit RBAC Permissions

```bash
# List all Roles in tenant namespaces
kubectl get roles --all-namespaces | grep -v "kube-"

# Review specific Role
kubectl get role pipeline-runner -n <tenant-namespace> -o yaml

# Check what a service account can do
kubectl auth can-i --list --as=system:serviceaccount:<tenant-namespace>:pipeline-runner -n <tenant-namespace>
```

### Review Network Policies

```bash
# List all NetworkPolicies
kubectl get networkpolicies --all-namespaces

# Review specific NetworkPolicy
kubectl get networkpolicy tenant-isolation -n <tenant-namespace> -o yaml

# Test network connectivity
kubectl run -it --rm debug --image=busybox --restart=Never -n <tenant-namespace> -- wget -O- http://<service>.<other-namespace>.svc.cluster.local
```

**Source**
- `platform/onboarding/controller/`
- `platform/tenancy/templates/`

## Runbooks

### Bootstrap Runbook

**Prerequisites**:
- Kubernetes cluster (1.24+) with RBAC enabled
- kubectl configured with cluster admin access
- GitHub organization with admin access

**Bootstrap Steps**:

1. **Clone Repository**:
   ```bash
   git clone https://github.com/bdchatham/ArbiterPipelineInfrastructure.git
   cd ArbiterPipelineInfrastructure
   ```

2. **Run Bootstrap**:
   ```bash
   cd platform/bootstrap
   ./bootstrap.sh --cluster-name arbiter-platform --repo-url https://github.com/bdchatham/ArbiterPipelineInfrastructure
   ```

3. **Verify Bootstrap**:
   ```bash
   # Check all components
   kubectl get pods -n argocd
   kubectl get pods -n tekton-pipelines
   kubectl get pods -n platform-system

   # Check ArgoCD Applications
   kubectl get application -n argocd
   ```

4. **Access ArgoCD UI**:
   ```bash
   # Get admin password
   kubectl get secret argocd-initial-admin-secret -n argocd -o jsonpath='{.data.password}' | base64 -d

   # Port-forward
   kubectl port-forward svc/argocd-server -n argocd 8080:443

   # Access at http://localhost:8080
   ```

**Source**
- `platform/bootstrap/bootstrap.sh`

### Onboarding Runbook

**Prerequisites**:
- Platform bootstrapped and running
- kubectl configured with cluster access
- GitHub repository ready for onboarding

**Onboarding Steps**:

1. **Create RepoBinding**:
   ```bash
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
   ```

2. **Verify Onboarding**:
   ```bash
   # Check RepoBinding status
   kubectl get repobinding my-repo-binding -n platform-system

   # Verify tenant namespace
   kubectl get namespace my-tenant

   # Verify all resources
   kubectl get all -n my-tenant
   ```

3. **Configure GitHub Webhook**:
   ```bash
   # Get webhook URL and secret
   kubectl get repobinding my-repo-binding -n platform-system -o yaml

   # Configure in GitHub repository Settings → Webhooks
   ```

4. **Test Pipeline**:
   ```bash
   # Merge a commit to main branch
   # Watch for PipelineRun creation
   kubectl get pipelineruns -n my-tenant -w
   ```

**Source**
- `platform/crds/repobinding-crd.yaml`
- `platform/onboarding/controller/`

**Source**
- `.kiro/specs/argocd-tekton-platform/design.md`
- `.kiro/specs/argocd-tekton-platform/requirements.md`
- `platform/bootstrap/bootstrap.sh`
- `platform/argocd/apps/`
- `platform/onboarding/controller/`
- `platform/catalog/`
- `platform/tenancy/templates/`
