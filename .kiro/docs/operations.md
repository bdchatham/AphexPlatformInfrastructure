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
3. Installs Tekton Pipelines and Tekton Triggers
4. Installs ArgoCD
5. Creates platform namespaces (argocd, tekton-pipelines, platform-system)
6. Creates platform root ArgoCD Application
7. Displays ArgoCD credentials and access instructions

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

```bash
# Update Tekton version in bootstrap script
vi platform/bootstrap/bootstrap.sh

# Update version URLs
# Tekton Pipelines: https://github.com/tektoncd/pipeline/releases/download/v0.57.0/release.yaml
# Tekton Triggers: https://github.com/tektoncd/triggers/releases/download/v0.26.0/release.yaml

# Commit changes
git add platform/bootstrap/bootstrap.sh
git commit -m "Update Tekton to v0.57.0"
git push

# ArgoCD will NOT automatically update Tekton (installed by bootstrap)
# To update Tekton, manually apply new manifests:
kubectl apply -f https://github.com/tektoncd/pipeline/releases/download/v0.57.0/release.yaml
kubectl apply -f https://github.com/tektoncd/triggers/releases/download/v0.26.0/release.yaml
```

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
