# FAQ

## General Questions

### What is this repository for?

The Aphex Pipeline Infrastructure provides a production-ready GitOps platform using ArgoCD and Tekton with revolutionary layered cert-manager architecture. It enables zero-touch deployment, centralized authentication, self-service repository onboarding, and bulletproof certificate management.

### How does this fit into the larger system?

This platform provides shared CI/CD infrastructure with complete tenant isolation. Teams can onboard repositories through RepoBinding CRDs, which automatically provisions isolated namespaces with RBAC, network policies, and pipeline resources. The platform serves as the foundation for automated deployments and infrastructure management.

### What makes the cert-manager architecture special?

The platform implements a layered cert-manager deployment that eliminates the classic "webhook chicken-and-egg" problem through sync waves and PostSync validation. This eliminates manual intervention and timing-related failures.

For detailed architecture, see [architecture.md](architecture.md).

### What is zero-touch bootstrap?

The bootstrap script achieves complete platform convergence automatically by generating all secrets, creating the cluster, and waiting for full platform functionality without manual steps.

For deployment procedures, see [operations.md](operations.md).

### What is a tenant?

A tenant is a product team with complete isolation and dedicated resources:
- **Namespace**: Isolated Kubernetes namespace with RBAC boundaries
- **Service Account**: Least-privilege access with role-based permissions
- **Resource Quotas**: CPU, memory, and storage limits
- **Network Policies**: Traffic isolation with ingress exceptions
- **EventListener**: Tekton webhook handler for GitHub integration
- **Pipeline Resources**: Access to shared catalog and custom pipelines

## Authentication Questions

### How do I access platform services?

All platform services use centralized authentication via Authentik and Dex:

**ArgoCD UI**: `https://argocd.home.local` - Click "Login via Dex"
**Tekton Dashboard**: `https://tekton.home.local` - Authenticate via Dex/Authentik  
**Authentik UI**: `https://auth.home.local` - Direct login with admin credentials

For detailed authentication procedures, see [operations.md](operations.md).

### How do I manage users?

Use the Authentik web UI at `https://auth.home.local`. Create users and assign to groups (`admins` or `engineering`). Users can authenticate immediately without pod restarts.

## Organization and Webhook Questions

### Why does the platform use two different domains?

The platform uses two domain strategies for different purposes:

**arbiter-dev.com (Public Domain)**:
- Organization webhook endpoints accessible from the internet
- GitHub webhooks can reach these endpoints
- SSL/TLS termination at Cloudflare edge
- Example: `acme-corp.arbiter-dev.com`

**home.local (Local Domain)**:
- Authentication and platform services accessible only within home network
- ArgoCD, Authentik, Dex, Tekton Dashboard
- Example: `argocd.home.local`, `auth.home.local`

This separation ensures webhook endpoints are publicly accessible while keeping platform administration services private.

### How do webhooks reach my local cluster?

Webhooks use Cloudflare Tunnels to reach the cluster without exposing ports. The cloudflared pod maintains an outbound connection to Cloudflare, so no inbound ports are opened on your network.

For detailed webhook flow and networking architecture, see [architecture.md](architecture.md#webhook-path-detail).

### What happens when I delete an organization?

Organization deletion follows this sequence:

1. Delete DNS CNAME record from Cloudflare
2. Cleanup active tunnel connections via Cloudflare API
3. Delete tunnel from Cloudflare
4. Delete ClusterRoleBinding for EventListener
5. Delete organization namespace (cascades all resources)

This ensures complete cleanup with no orphaned resources in Cloudflare or Kubernetes.

### How do I troubleshoot webhook delivery issues?

For detailed webhook troubleshooting procedures, see [operations.md](operations.md#test-webhook-endpoint) and [operations.md](operations.md#webhook-not-delivered).

For user management procedures, see [operations.md](operations.md).

### How do I get admin credentials?

```bash
kubectl get secret authentik-secrets -n auth-system \
  -o jsonpath='{.data.admin-password}' | base64 -d
```

Bootstrap never prints secrets to stdout for security.

### Why can't I access services at localhost URLs?

The platform uses real hostnames for OIDC authentication:
- Browser redirects require reachable URLs
- Internal Kubernetes DNS (`*.svc.cluster.local`) won't work
- Configure DNS for `*.home.local` in your router or hosts file

## Development Questions

### How do I onboard my repository?

Create a RepoBinding resource:

```yaml
apiVersion: aphex.io/v1alpha1
kind: RepoBinding
metadata:
  name: my-repo-binding
  namespace: platform-system
spec:
  aphexOrg: "acme-corp"
  repoOrg: "acme-corp"
  repoName: "my-application"
  pipelineName: "cdktf-deploy-pipeline"
  templateRef: "cdktf-deploy-trigger-template"
```

For detailed onboarding procedures, see [operations.md](operations.md).

### How do I configure the GitHub webhook?

After RepoBinding reaches `Ready` phase, get webhook configuration from RepoBinding status and configure in GitHub repository Settings → Webhooks.

For webhook configuration details, see [operations.md](operations.md).

### How do I view pipeline logs?

```bash
# List recent PipelineRuns
kubectl get pipelineruns -n my-app --sort-by=.metadata.creationTimestamp

# Get PipelineRun details
kubectl describe pipelinerun <name> -n my-app

# View logs for all tasks
kubectl logs -n my-app -l tekton.dev/pipelineRun=<name>

# Stream logs in real-time
kubectl logs -n my-app -l tekton.dev/pipelineRun=<name> -f
```

### How do I test pipelines without GitHub webhooks?

Create a PipelineRun manually:

```yaml
apiVersion: tekton.dev/v1beta1
kind: PipelineRun
metadata:
  name: test-run
  namespace: my-app
spec:
  pipelineRef:
    name: my-pipeline
  params:
  - name: git-url
    value: "https://github.com/acme-corp/my-application"
  - name: git-revision
    value: "main"
  workspaces:
  - name: shared-data
    volumeClaimTemplate:
      spec:
        accessModes:
        - ReadWriteOnce
        resources:
          requests:
            storage: 1Gi
```

## Operational Questions

### How do I upgrade the platform?

The platform is self-upgrading via GitOps:
1. Update component versions in `platform/` manifests
2. Commit and push to Git
3. ArgoCD detects changes and syncs automatically
4. Monitor application health in ArgoCD UI

### How do I troubleshoot cert-manager issues?

Check the layered deployment status:

```bash
# Wave 10: cert-manager installation
kubectl get pods -n cert-manager
kubectl get job cert-manager-webhook-readiness -n cert-manager
kubectl logs job/cert-manager-webhook-readiness -n cert-manager

# Wave 20: Certificate creation
kubectl get clusterissuer selfsigned-issuer
kubectl get certificates -A

# Wave 30: Ingress resources
kubectl get ingress -A
```

### Why are my certificates not ready?

Check the cert-manager webhook validation:

```bash
# Check webhook readiness job
kubectl get job cert-manager-webhook-readiness -n cert-manager
kubectl logs job/cert-manager-webhook-readiness -n cert-manager

# Check webhook endpoints
kubectl get endpoints cert-manager-webhook -n cert-manager

# Check webhook configuration
kubectl get validatingwebhookconfiguration cert-manager-webhook \
  -o jsonpath='{.webhooks[0].clientConfig.caBundle}' | base64 -d | openssl x509 -text -noout
```

### How do I check ArgoCD application sync status?

```bash
# List all applications with status
kubectl get applications -n argocd

# Get detailed application status
kubectl describe application platform-cert-manager -n argocd

# Check sync waves and ordering
kubectl get applications -n argocd \
  -o custom-columns="NAME:.metadata.name,WAVE:.metadata.annotations.argocd\.argoproj\.io/sync-wave,STATUS:.status.sync.status,HEALTH:.status.health.status"
```

### How do I access ArgoCD when authentication is broken?

Use port-forward to bypass ingress and authentication:

```bash
# Port-forward to ArgoCD server
kubectl port-forward svc/argocd-server -n argocd 8080:443

# Get admin password
kubectl get secret argocd-initial-admin-secret -n argocd \
  -o jsonpath='{.data.password}' | base64 -d

# Access at https://localhost:8080
# Username: admin, Password: (from above)
```

### How do I reset the authentication system?

If authentication is completely broken:

```bash
# Delete auth-system pods to restart
kubectl delete pods -n auth-system --all

# Check Config Sync Job status
kubectl get job auth-config-sync -n auth-system
kubectl logs job/auth-config-sync -n auth-system

# Verify Dex scaling
kubectl get deployment dex -n auth-system
```

### How do I migrate from Kind to a real cluster?

The platform is designed for portability:

1. **Deploy ingress controller** to real cluster
2. **Configure real DNS** for `*.home.local` (or your domain)
3. **Update ClusterIssuer** for Let's Encrypt (optional)
4. **Run bootstrap script** with `--use-existing` flag
5. **Platform converges identically** to Kind deployment

The only differences are ingress controller and DNS configuration.

## Troubleshooting

### Bootstrap fails with "authentication required" error

This usually means ArgoCD can't access the Git repository:
- Verify repository URL is correct and accessible
- Check if repository is private (may need access tokens)
- Ensure network connectivity from cluster to GitHub

### Certificates stuck in "Progressing" state

Check the layered cert-manager deployment:
- Verify cert-manager pods are Running
- Check webhook readiness job completed successfully
- Verify ClusterIssuer is ready before certificates

### Authentication redirects fail with "invalid redirect URI"

Verify OIDC configuration:
- Ensure hostnames are reachable from browser
- Check DNS configuration for `*.home.local`
- Verify ingress controller is accessible
- Never use `*.svc.cluster.local` URLs for browser redirects

### Platform services return 503 errors

Check ingress and TLS configuration:
- Verify ingress controller is running
- Check certificate status (`kubectl get certificates -A`)
- Verify DNS resolution from your device
- Check ingress resource configuration

**Source**
- `platform/bootstrap/bootstrap.sh` - Bootstrap implementation and troubleshooting
- `platform/cert-manager/webhook-readiness-hook.yaml` - cert-manager validation
- `platform/auth/` - Authentication system components
- `platform/argocd/apps/` - ArgoCD application definitions
  pipelineRef:
    name: cdktf-deploy-pipeline
    namespace: pipeline-system
  params:
    - name: repo-url
      value: "https://github.com/your-github-org/your-repo"
    - name: commit-sha
      value: "abc123..."
    - name: tenant-name
      value: "my-tenant"
  serviceAccountName: pipeline-runner
EOF
```

## Operational Questions

### What should I do if my repository isn't triggering pipelines?

**Diagnosis**:

```bash
# Check if EventListener exists
kubectl get eventlistener -n <tenant-namespace>

# Check EventListener logs for webhook events
kubectl logs -n <tenant-namespace> -l eventlistener=github-listener | grep "webhook"

# Check if Ingress exists
kubectl get ingress -n <tenant-namespace>

# Check RepoBinding status
kubectl describe repobinding <name> -n pipeline-system
```

**Common Issues**:
1. EventListener not running (check onboarding controller logs)
2. Ingress not configured correctly (check Ingress controller installation)
3. GitHub webhook not configured (check GitHub webhook settings)
4. Webhook secret mismatch (check RepoBinding status for correct secret)

### What should I do if my pipeline fails?

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
1. Git clone failure: Check repository access
2. CDKTF synth failure: Check Node.js dependencies and syntax
3. CDKTF deploy failure: Check Terraform state and permissions
4. RBAC denial: Check service account permissions

### How do I update platform components?

The platform upgrades itself via ArgoCD when manifests change in Git:

```bash
# Update component manifests in Git
vi platform/platform-controller/controller-deployment.yaml  # Update image tag

# Commit changes
git add .
git commit -m "Update onboarding controller to v1.1.0"
git push

# ArgoCD will automatically sync and update the controller
# Watch sync status
kubectl get application -n argocd -w
```

### How do I delete a tenant?

Delete the RepoBinding:

```bash
kubectl delete repobinding <name> -n pipeline-system
```

Or manually delete the namespace:

```bash
kubectl delete namespace <tenant-namespace>
```

## Architecture Questions

### Why use ArgoCD instead of Flux?

ArgoCD provides several advantages:
1. Better UI for visualizing sync status
2. More mature and widely adopted
3. Better support for App of Apps pattern
4. Easier to troubleshoot sync issues
5. Strong community support

### Why use Tekton instead of Argo Workflows?

Tekton is the standard pipeline engine for Kubernetes-native CI/CD and provides:
1. Native Kubernetes integration
2. Reusable Tasks and Pipelines
3. Strong community support
4. Cloud-native design
5. Better integration with Tekton Triggers for webhooks

### How does tenant isolation work?

Isolation is achieved through multiple layers:
1. **Kubernetes Namespaces**: Each tenant gets dedicated namespace
2. **RBAC**: Service accounts scoped to tenant namespace only
3. **Network Policies**: Restrict inter-namespace communication
4. **Resource Quotas**: Prevent resource exhaustion
5. **Terraform State**: Isolated per tenant

### How does the App of Apps pattern work?

The platform uses a root ArgoCD Application that manages child Applications for each component layer:
- **platform-root**: Manages all child Applications
- **platform-crds**: CRDs and foundational resources
- **platform-infrastructure**: Namespaces and RBAC
- **platform-controllers**: Onboarding controller
- **platform-catalog**: Tekton tasks, pipelines, triggers

This provides better separation of concerns, independent lifecycle management, and clearer troubleshooting.

### Can multiple clusters be deployed?

Yes! You can deploy separate clusters for different environments:
- Development cluster (smaller, fewer resources)
- Staging cluster (production-like)
- Production cluster (larger, more resources)

Each cluster is independent with its own tenants and ArgoCD Applications.

## Security Questions

### How are webhook secrets stored?

Webhook secrets are generated by the Onboarding Controller using cryptographic randomness and stored in Kubernetes Secrets in the tenant namespace. EventListeners reference these secrets for webhook signature validation.

### How are Terraform credentials managed?

Terraform backend credentials are stored in per-tenant Secrets. For Kubernetes backend, no external credentials are needed. The platform uses the Kubernetes backend by default for simplicity.

### Can tenants access other tenants' resources?

No. RBAC ensures service accounts can only access resources in their own namespace. Network policies prevent cross-namespace network access.

### Can tenants access platform namespaces?

No. RBAC denies access to platform namespaces (platform-system, argocd, tekton-pipelines, etc.). Only the onboarding controller has permissions to create resources in platform namespaces.

### How do I rotate webhook secrets?

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

### How do I audit tenant activity?

```bash
# View PipelineRuns for a tenant
kubectl get pipelineruns -n <tenant-namespace>

# View events for a tenant
kubectl get events -n <tenant-namespace> --sort-by='.lastTimestamp'

# View EventListener logs for a tenant
kubectl logs -n <tenant-namespace> -l eventlistener=github-listener
```

## Troubleshooting Questions

### Why is ArgoCD not syncing my changes?

**Common Issues**:
1. ArgoCD cannot access Git repository (check repo-server logs)
2. Sync policy not configured (check Application spec)
3. Manifest errors in Git (check Application status)
4. ArgoCD controller not running (check argocd namespace)

**Resolution**:
```bash
# Check Application sync status
kubectl get application platform-root -n argocd

# Check ArgoCD controller logs
kubectl logs -n argocd -l app.kubernetes.io/name=argocd-application-controller --tail=100

# Manually trigger sync
kubectl patch application platform-root -n argocd --type merge -p '{"operation":{"initiatedBy":{"username":"admin"},"sync":{"revision":"HEAD"}}}'
```

### Why is the onboarding controller not reconciling?

**Common Issues**:
1. Controller not running (check pod status)
2. Controller lacks RBAC permissions (check controller logs)
3. RepoBinding validation failed (check RepoBinding status)
4. Tekton Triggers not installed (check tekton-pipelines namespace)

**Resolution**:
```bash
# Check controller logs
kubectl logs -n platform-system -l app=platform-controller --tail=100

# Check controller pod status
kubectl get pods -n platform-system -l app=platform-controller

# Restart controller if needed
kubectl rollout restart deployment platform-controller -n platform-system
```

### Why is my EventListener pod crashing?

**Common Issues**:

1. **Missing Core Interceptors**: EventListener needs ClusterInterceptors (github, gitlab, cel, etc.) to validate webhooks
   ```bash
   # Check if ClusterInterceptors exist
   kubectl get clusterinterceptors
   
   # If missing, install Core Interceptors
   kubectl apply -f https://infra.tekton.dev/tekton-releases/triggers/previous/v0.34.0/interceptors.yaml
   ```

2. **Missing Cluster-scoped RBAC**: EventListener needs read permissions for ClusterInterceptor and ClusterTriggerBinding
   ```bash
   # Check if ClusterRole exists
   kubectl get clusterrole pipeline-runner-<tenant-name>
   
   # If missing, delete and recreate RepoBinding
   kubectl delete repobinding <name> -n platform-system
   kubectl apply -f repobinding.yaml
   ```

3. **Webhook Secret Missing**: EventListener needs webhook secret for signature validation
   ```bash
   # Check if secret exists
   kubectl get secret webhook-<tenant-name> -n <tenant-namespace>
   ```

### What are ClusterInterceptors?

ClusterInterceptors are cluster-scoped Tekton Triggers resources that provide webhook validation and filtering capabilities. The Core Interceptors include:
- **github**: Validates GitHub webhook signatures and filters events
- **gitlab**: Validates GitLab webhook signatures and filters events
- **cel**: Evaluates CEL expressions for custom filtering
- **bitbucket**: Validates Bitbucket webhook signatures
- **slack**: Validates Slack webhook signatures

EventListeners reference ClusterInterceptors to validate incoming webhooks before creating PipelineRuns.

### Why does my tenant need cluster-scoped RBAC?

EventListener pods run with the tenant's `pipeline-runner` ServiceAccount and need to read cluster-scoped Tekton Triggers resources (ClusterInterceptor, ClusterTriggerBinding). These resources are cluster-scoped and cannot be accessed via namespace-scoped Roles.

The onboarding controller provisions a ClusterRole with read-only permissions for these resources and binds it to the tenant's ServiceAccount. This follows the principle of least privilege - tenants can only read cluster-scoped Tekton Triggers resources, not modify them.

### Why is my webhook not being delivered?

**Common Issues**:
1. Ingress not accessible from GitHub (check Ingress configuration)
2. Webhook secret mismatch (check RepoBinding status)
3. EventListener not running (check pod status)
4. GitHub webhook not configured (check GitHub webhook settings)

**Resolution**:
```bash
# Check EventListener logs
kubectl logs -n <tenant-namespace> -l eventlistener=github-listener --tail=100

# Check Ingress configuration
kubectl get ingress -n <tenant-namespace> -o yaml

# Check GitHub webhook delivery logs
# Go to GitHub repository Settings → Webhooks → Recent Deliveries
```

## Authentication Questions

### How do I access the Authentik UI?

After bootstrap completes and ArgoCD syncs the auth system:

1. **Ensure DNS is configured** so that `auth.home.local` resolves to your Ingress controller's IP
2. **Open Authentik UI**: `https://auth.home.local`
3. **Accept certificate warning** (if using self-signed certificates)
4. **Retrieve admin password**:
   ```bash
   kubectl get secret authentik-secrets -n auth-system \
     -o jsonpath='{.data.admin-password}' | base64 -d
   ```
5. **Login with username `admin`** and the password from step 4

### How do I create a new user?

1. Login to Authentik UI at `https://auth.home.local`
2. Navigate to **Directory** → **Users**
3. Click **Create**
4. Fill in user details (username, email, name, password)
5. Assign user to groups (`admins` or `engineering`)
6. Click **Create**

Users can authenticate immediately - no pod restarts or configuration changes required.

### What's the difference between platform groups?

**platform-admins group**:
- Full access to ArgoCD (can create, update, delete applications)
- Full access to Tekton Dashboard (can create, update, delete pipelines)
- Full CRUD access to all platform CRDs in all namespaces
- Can create and delete namespaces
- Superuser access to Authentik UI (can manage users and groups)

**platform-operators group**:
- Full access to platform CRDs (cannot create/delete namespaces)
- Can read logs and events in all namespaces for troubleshooting
- Read-only access to ArgoCD and Tekton Dashboard

**platform-engineering group**:
- Can create, read, update platform CRDs (no delete permissions)
- Restricted to user-* and team-* namespaces only
- Cannot access platform system namespaces
- Read-only access to ArgoCD and Tekton Dashboard

### How do I troubleshoot OIDC authentication issues?

**Symptom: "Cannot reach Dex" during login**

1. **Check Dex pod status**:
   ```bash
   kubectl get pods -n auth-system -l app=dex
   kubectl logs -n auth-system -l app=dex
   ```

2. **Verify Dex Ingress**:
   ```bash
   kubectl get ingress -n auth-system dex
   curl -v https://dex.home.local/.well-known/openid-configuration
   ```

3. **Check DNS resolution**:
   ```bash
   nslookup dex.home.local
   ```

**Symptom: "Permission Denied" after successful login**

1. **Check user's group membership in Authentik UI**
2. **Verify RBAC permissions**:
   ```bash
   kubectl auth can-i create pipelines.platform.dev --as=user@platform.local --as-group=platform-engineering -n user-alice
   ```

3. **Check token claims**:
   ```bash
   # Decode JWT token to verify groups claim
   # (Use jwt.io or similar tool to decode token)
   ```

**Symptom: Break-glass access needed**

```bash
# Use certificate-based admin access
kubectl --kubeconfig /etc/kubernetes/admin.conf get pods -n auth-system

# Fix OIDC issues using admin access
kubectl --kubeconfig /etc/kubernetes/admin.conf rollout restart deployment/dex -n auth-system
```

### How do I configure DNS for authentication services?

DNS configuration is required for user browsers to reach services via hostnames. OIDC authentication requires redirect URIs that browsers can reach.

**Option 1: Router/Pi-hole DNS (Recommended)**

Add A records in your home router or Pi-hole:
```
auth.home.local     → 192.168.1.100
dex.home.local      → 192.168.1.100
argocd.home.local   → 192.168.1.100
tekton.home.local   → 192.168.1.100
```

Replace `192.168.1.100` with your Ingress controller's IP address.

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

**Find your Ingress controller IP**:
```bash
kubectl get svc -n ingress-nginx ingress-nginx-controller
```

### Why am I getting certificate warnings?

If you're using self-signed certificates (the simplest option for homelab), browsers will show certificate warnings. This is expected behavior.

**To proceed**:
- Chrome: Click "Advanced" → "Proceed to auth.home.local (unsafe)"
- Firefox: Click "Advanced" → "Accept the Risk and Continue"
- Safari: Click "Show Details" → "visit this website"

**To avoid warnings**, use Let's Encrypt with DNS-01 challenge (requires DNS provider API access). See `.kiro/docs/operations.md` for setup instructions.

### Why can't I log in to ArgoCD or Tekton Dashboard?

**Common Issues**:

1. **DNS not configured**: Browser cannot reach `dex.home.local` or `auth.home.local`
   ```bash
   # Test DNS resolution
   nslookup auth.home.local
   nslookup dex.home.local
   ```

2. **User not in correct group**: Check user group membership in Authentik UI
   - Navigate to **Directory** → **Users** → Select user → **Groups** tab
   - Ensure user is in `admins` or `engineering` group

3. **Redirect URI mismatch**: Check Dex config and Authentik OIDC provider
   ```bash
   # Check Authentik OIDC discovery
   curl https://auth.home.local/application/o/dex/.well-known/openid-configuration
   
   # Check Dex OIDC discovery
   curl https://dex.home.local/.well-known/openid-configuration
   ```

4. **Services not ready**: Check pod status
   ```bash
   kubectl get pods -n auth-system
   kubectl get pods -n argocd
   kubectl get pods -n tekton-pipelines
   ```

### Why is Dex pod crashing?

**Common Issues**:

1. **Authentik not ready yet**: Dex should start with replicas=0 and only scale to 1 after Authentik is configured
   ```bash
   # Check Dex replicas
   kubectl get deployment dex -n auth-system -o jsonpath='{.spec.replicas}'
   
   # Should be 0 initially, then 1 after Config Sync Job completes
   ```

2. **Invalid Dex configuration**: Check ConfigMap syntax
   ```bash
   kubectl get configmap dex-config -n auth-system -o yaml
   ```

3. **Missing RBAC permissions**: Check ServiceAccount and Role
   ```bash
   kubectl get serviceaccount dex -n auth-system
   kubectl get role dex -n auth-system
   ```

**Resolution**:
```bash
# Check Dex logs
kubectl logs -n auth-system deployment/dex

# Check Config Sync Job status
kubectl get job auth-config-sync -n auth-system
kubectl logs -n auth-system job/auth-config-sync
```

### What is the Config Sync Job?

The Config Sync Job orchestrates the integration between Authentik and Dex by:

1. Waiting for Authentik to be fully ready
2. Reading the Dex client secret from Kubernetes Secrets
3. Updating Authentik's OIDC provider with the client secret via API
4. Verifying Authentik's OIDC discovery endpoint is working
5. Scaling Dex from 0 to 1 replica (starts Dex now that Authentik is ready)
6. Waiting for Dex to be fully ready
7. Verifying Dex's OIDC discovery endpoint is working

This provides deterministic, observable, and retryable convergence without relying on timing-based hacks.

### How do I manually trigger the Config Sync Job?

```bash
# Delete existing Job
kubectl delete job auth-config-sync -n auth-system

# ArgoCD will recreate the Job automatically
# Or manually apply:
kubectl apply -f platform/auth/config-sync/job.yaml

# Watch Job progress
kubectl logs -n auth-system -l app=auth-config-sync -f
```

### How do I rotate the Authentik admin password?

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

### How do I rotate the Dex client secret?

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

### How do I add a custom role?

1. **Create new group in Authentik UI**:
   - Navigate to **Directory** → **Groups** → **Create**
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

### How do I integrate with GitHub OAuth?

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

### Why are secrets not printed during bootstrap?

Bootstrap **never prints secret values to stdout or logs by default**. This prevents accidental exposure in CI/CD logs, terminal history, or shared screens.

**To retrieve secrets after bootstrap**:
```bash
# Authentik admin password
kubectl get secret authentik-secrets -n auth-system \
  -o jsonpath='{.data.admin-password}' | base64 -d

# Dex client secret
kubectl get secret dex-secrets -n auth-system \
  -o jsonpath='{.data.client-secret}' | base64 -d

# Authentik API token
kubectl get secret authentik-api-token -n auth-system \
  -o jsonpath='{.data.token}' | base64 -d
```

**For local debugging only**, use `--show-secrets` flag:
```bash
./platform/bootstrap/bootstrap.sh --show-secrets
```

**Never use `--show-secrets` in production, CI/CD, or shared environments.**

### How do I make changes to the authentication system?

All authentication system components are managed by ArgoCD via GitOps. To make changes:

1. **Update manifests in Git**:
   ```bash
   # Example: Update Authentik image version
   vi platform/auth/authentik/server-deployment.yaml
   
   # Commit changes
   git add .
   git commit -m "Update Authentik to v2024.2.2"
   git push
   ```

2. **ArgoCD detects and syncs changes automatically**:
   ```bash
   # Watch ArgoCD sync status
   kubectl get application platform-auth -n argocd -w
   ```

3. **Verify changes**:
   ```bash
   kubectl get pods -n auth-system
   kubectl get application platform-auth -n argocd
   ```

**No manual kubectl apply required** - ArgoCD handles all deployments and updates.

### Why do I need an Ingress controller?

An Ingress controller is required for OIDC authentication flows because:

1. **Browser-reachable URLs**: OIDC requires redirect URIs that user browsers can reach (e.g., `https://dex.home.local/callback`)
2. **TLS termination**: OIDC requires HTTPS for security
3. **Hostname-based routing**: Different services need different hostnames (auth.home.local, dex.home.local, argocd.home.local)

**Install nginx-ingress** (recommended for homelab):
```bash
# For Kind clusters
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml

# For bare-metal clusters
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/baremetal/deploy.yaml
```

### What happens if ArgoCD is not syncing auth changes?

**Common Issues**:

1. **ArgoCD auto-sync disabled**: Check Application sync policy
   ```bash
   kubectl get application platform-auth -n argocd -o yaml | grep -A 5 syncPolicy
   ```

2. **Application in error state**: Check Application status
   ```bash
   kubectl describe application platform-auth -n argocd
   ```

3. **Invalid YAML syntax**: Check for manifest errors in Git
   ```bash
   kubectl get application platform-auth -n argocd -o json | \
     jq '.status.conditions[] | select(.type=="SyncError")'
   ```

**Resolution**:
```bash
# Manually trigger sync
kubectl patch application platform-auth -n argocd \
  --type merge -p '{"operation":{"initiatedBy":{"username":"admin"},"sync":{}}}'

# Or use ArgoCD UI
# https://argocd.home.local → Applications → platform-auth → Sync
```

### How do I troubleshoot Config Sync Job failures?

**Check Job status**:
```bash
kubectl get job auth-config-sync -n auth-system
kubectl logs -n auth-system -l app=auth-config-sync
```

**Common Issues**:

1. **Authentik not ready**: Job waits for Authentik to be ready before proceeding
   ```bash
   kubectl get pods -n auth-system -l app=authentik
   kubectl logs -n auth-system -l app=authentik
   ```

2. **OIDC provider not found**: Authentik Blueprint did not create the OIDC provider
   ```bash
   # Check if Blueprint ConfigMap exists
   kubectl get configmap authentik-blueprints -n auth-system
   
   # Check Authentik logs for Blueprint application
   kubectl logs -n auth-system -l app=authentik | grep -i blueprint
   ```

3. **API token invalid**: Authentik API token has insufficient permissions
   ```bash
   # Verify API token exists
   kubectl get secret authentik-api-token -n auth-system
   
   # Re-run bootstrap to recreate token
   ./platform/bootstrap/bootstrap.sh
   ```

**Resolution**: Fix the underlying issue and re-run the Job:
```bash
kubectl delete job auth-config-sync -n auth-system
# ArgoCD will recreate the Job automatically
```

## Archon-Specific Questions

### How is this repository ingested by Archon?

Archon reads all Markdown files under `.kiro/docs/` from this public GitHub repository. Documentation follows the contract defined in `CLAUDE.md`.

### How do I update documentation?

Update the relevant files under `.kiro/docs/` and ensure changes are grounded in code. Include "Source" references to relevant files. Follow the 6-file structure (overview, architecture, operations, api, data-models, faq).

### What documentation standards should I follow?

Follow the Archon documentation contract in `CLAUDE.md`:
1. Keep sections small and focused (400-800 tokens)
2. Use clear, direct language
3. Maintain provenance (reference source files)
4. No hallucinations (only document what exists)
5. Avoid duplication (link instead of repeating)
6. Use descriptive, specific headings

**Source**
- `CLAUDE.md`
- `.kiro/steering/archon-docs.md`
- `README.md`
- `.kiro/specs/argocd-tekton-platform/design.md`
- `.kiro/specs/argocd-tekton-platform/requirements.md`
- `.kiro/specs/dex-authentication-platform/design.md`
- `.kiro/specs/dex-authentication-platform/requirements.md`
- `.kiro/docs/operations.md`
- `.kiro/docs/api.md`
- `platform/auth/README.md`
- `platform/auth/secrets/README.md`
- `platform/auth/ingress/README.md`
- `platform/auth/config-sync/README.md`
