# Operations

## Deployment

### Prerequisites

Before deploying the Jenkins X platform, ensure you have:

1. **Kubernetes Cluster**: Version 1.24+ with RBAC enabled
2. **kubectl**: Configured with cluster access
3. **helm**: Version 3.x installed
4. **GitHub Organization**: With admin access for creating GitHub App
5. **Cluster Features**: RBAC and NetworkPolicy support

### Bootstrap Process

The bootstrap process installs all platform components in the correct order.

**Step 1: Verify Prerequisites**

```bash
# Run prerequisites check
cd platform/bootstrap
./prerequisites.sh
```

**Step 2: Create Platform Namespaces**

```bash
# Create pipeline-system namespace
kubectl create namespace pipeline-system --dry-run=client -o yaml | kubectl apply -f -

# Create pipeline-catalog namespace
kubectl create namespace pipeline-catalog --dry-run=client -o yaml | kubectl apply -f -

# Create auth-system namespace (for Dex)
kubectl create namespace auth-system --dry-run=client -o yaml | kubectl apply -f -
```

**Step 3: Deploy OIDC Provider (Dex)**

```bash
# Apply Dex configuration
kubectl apply -f platform/auth/dex-config.yaml
kubectl apply -f platform/auth/dex-deployment.yaml

# Wait for Dex to be ready
kubectl wait --for=condition=ready pod -l app=dex -n auth-system --timeout=300s

# Configure Kubernetes API server for OIDC
# For k3s: Edit /etc/rancher/k3s/config.yaml
# For kubeadm: Edit /etc/kubernetes/manifests/kube-apiserver.yaml
# Add OIDC flags and restart API server
```

**Step 4: Install Tekton Pipelines**

```bash
# Install Tekton
kubectl apply -f https://storage.googleapis.com/tekton-releases/pipeline/latest/release.yaml

# Wait for Tekton controllers
kubectl wait --for=condition=ready pod -l app=tekton-pipelines-controller -n tekton-pipelines --timeout=300s
```

**Step 5: Install Jenkins X and Lighthouse**

```bash
# Add Jenkins X Helm repository
helm repo add jx3 https://jenkins-x-charts.github.io/repo
helm repo update

# Install jx-build-controller
helm install jx jx3/jx-build-controller -n pipeline-system

# Create GitHub App (manual step - see GitHub App Setup section)

# Create Lighthouse configuration
kubectl apply -f platform/lighthouse/config.yaml

# Install Lighthouse
helm install lighthouse jx3/lighthouse -n pipeline-system \
  --set github.appId="${GITHUB_APP_ID}" \
  --set github.appInstallationId="${GITHUB_APP_INSTALLATION_ID}"

# Wait for Lighthouse
kubectl wait --for=condition=ready pod -l app=lighthouse -n pipeline-system --timeout=300s
```

**Step 6: Install RepoBinding CRD**

```bash
# Apply CRD
kubectl apply -f platform/crds/repobinding.yaml

# Verify CRD is registered
kubectl get crd repobindings.platform.arbiter.io
```

**Step 7: Deploy Onboarding Controller**

```bash
# Build controller (if not using pre-built image)
cd platform/onboarding
go build -o controller ./cmd/controller

# Build and push Docker image
docker build -t your-registry/onboarding-controller:latest .
docker push your-registry/onboarding-controller:latest

# Deploy controller
kubectl apply -f platform/onboarding/controller-rbac.yaml
kubectl apply -f platform/onboarding/controller-deployment.yaml

# Wait for controller
kubectl wait --for=condition=ready pod -l app=onboarding-controller -n pipeline-system --timeout=300s
```

**Step 8: Install Pipeline Catalog**

```bash
# Apply catalog resources
kubectl apply -f platform/catalog/tasks/
kubectl apply -f platform/catalog/pipelines/

# Verify catalog installation
kubectl get tasks -n pipeline-catalog
kubectl get pipelines -n pipeline-catalog
```

**Step 9: Build and Push Runner Image**

```bash
# Build runner image
cd platform/catalog/images/runner
docker build -t your-registry/pipeline-runner:latest .
docker push your-registry/pipeline-runner:latest
```

**Step 10: Verify Bootstrap**

```bash
# Check all platform components
kubectl get pods -n pipeline-system
kubectl get pods -n tekton-pipelines
kubectl get pods -n auth-system

# Verify CRD
kubectl get crd repobindings.platform.arbiter.io

# Verify catalog
kubectl get tasks,pipelines -n pipeline-catalog
```

### GitHub App Setup

**Create GitHub App**:

1. Go to GitHub Organization Settings → Developer settings → GitHub Apps
2. Click "New GitHub App"
3. Configure:
   - **Name**: Jenkins X Platform
   - **Homepage URL**: https://your-platform.example.com
   - **Webhook URL**: https://lighthouse.your-platform.example.com/hook
   - **Webhook secret**: Generate a secure secret
4. Set permissions:
   - Repository: Read access to code
   - Repository: Read and write access to pull requests
   - Repository: Read and write access to checks
   - Organization: Read access to members
5. Subscribe to events:
   - Push
   - Pull request
   - Check run
   - Check suite
6. Click "Create GitHub App"
7. Note the **App ID**
8. Generate and download **private key**
9. Install app at organization level

**Configure Lighthouse with GitHub App**:

```bash
# Create secret with GitHub App private key
kubectl create secret generic github-app-secret \
  --from-file=private-key=path/to/private-key.pem \
  -n pipeline-system

# Update Lighthouse configuration
kubectl apply -f - <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: lighthouse-config
  namespace: pipeline-system
data:
  config.yaml: |
    github:
      app_id: "${GITHUB_APP_ID}"
      app_installation_id: "${GITHUB_APP_INSTALLATION_ID}"
    allowlist:
      - org: "your-github-org"
        repos: []
EOF
```

## Onboarding Repositories

### Create RepoBinding

```bash
# Create RepoBinding for a repository
kubectl apply -f - <<EOF
apiVersion: platform.arbiter.io/v1alpha1
kind: RepoBinding
metadata:
  name: archon-binding
  namespace: pipeline-system
spec:
  repoOrg: "your-github-org"
  repoName: "archon-agent"
  tenantName: "archon"
  permissionProfile: "standard"
EOF
```

### Verify Onboarding

```bash
# Check RepoBinding status
kubectl get repobinding archon-binding -n pipeline-system
kubectl describe repobinding archon-binding -n pipeline-system

# Verify tenant namespace
kubectl get namespace archon

# Verify service account
kubectl get serviceaccount pipeline-runner -n archon

# Verify RBAC
kubectl get role,rolebinding -n archon

# Verify resource limits
kubectl get resourcequota,limitrange -n archon

# Verify network policy
kubectl get networkpolicy -n archon

# Verify allowlist update
kubectl get configmap repo-allowlist -n pipeline-system -o yaml
```

## Monitoring

### Platform Health Checks

```bash
# Check Tekton controllers
kubectl get pods -n tekton-pipelines

# Check Lighthouse
kubectl get pods -n pipeline-system -l app=lighthouse

# Check onboarding controller
kubectl get pods -n pipeline-system -l app=onboarding-controller

# Check Dex
kubectl get pods -n auth-system -l app=dex
```

### Pipeline Execution Monitoring

```bash
# List all PipelineRuns
kubectl get pipelineruns --all-namespaces

# Get PipelineRun details
kubectl describe pipelinerun <name> -n <tenant-namespace>

# View PipelineRun logs
kubectl logs <pod-name> -n <tenant-namespace>

# Watch PipelineRun status
kubectl get pipelinerun <name> -n <tenant-namespace> -w
```

### Lighthouse Event Logs

```bash
# View Lighthouse logs
kubectl logs -n pipeline-system -l app=lighthouse --tail=100

# Stream Lighthouse logs
kubectl logs -n pipeline-system -l app=lighthouse -f

# Search for specific repository events
kubectl logs -n pipeline-system -l app=lighthouse | grep "repo-name"
```

### Onboarding Controller Logs

```bash
# View controller logs
kubectl logs -n pipeline-system -l app=onboarding-controller --tail=100

# Stream controller logs
kubectl logs -n pipeline-system -l app=onboarding-controller -f

# Search for specific RepoBinding
kubectl logs -n pipeline-system -l app=onboarding-controller | grep "repobinding-name"
```

## Troubleshooting

### Repository Not Triggering Pipelines

**Diagnosis**:

```bash
# Check if repository is in allowlist
kubectl get configmap repo-allowlist -n pipeline-system -o yaml | grep "repo-name"

# Check Lighthouse logs for webhook events
kubectl logs -n pipeline-system -l app=lighthouse | grep "repo-name"

# Check if tenant namespace exists
kubectl get namespace <tenant-name>

# Check if service account exists
kubectl get serviceaccount pipeline-runner -n <tenant-name>
```

**Resolution**:
1. Verify repository is in allowlist ConfigMap
2. Verify GitHub App is installed and delivering webhooks
3. Verify tenant namespace was created by onboarding
4. Check Lighthouse logs for error messages

### PipelineRun Failures

**Diagnosis**:

```bash
# Get PipelineRun status
kubectl get pipelinerun <name> -n <tenant-namespace>

# Get detailed status
kubectl describe pipelinerun <name> -n <tenant-namespace>

# Get pod logs
kubectl logs <pod-name> -n <tenant-namespace>

# Check pod events
kubectl get events -n <tenant-namespace> --sort-by='.lastTimestamp'
```

**Common Issues**:

1. **Git clone failure**: Check repository access and credentials
2. **CDKTF synth failure**: Check Node.js dependencies and syntax errors
3. **CDKTF deploy failure**: Check Terraform state and AWS permissions
4. **RBAC denial**: Check service account permissions

### Onboarding Failures

**Diagnosis**:

```bash
# Check RepoBinding status
kubectl get repobinding <name> -n pipeline-system
kubectl describe repobinding <name> -n pipeline-system

# Check controller logs
kubectl logs -n pipeline-system -l app=onboarding-controller | grep "<name>"
```

**Common Issues**:

1. **Invalid organization**: Repository org not in approved list
2. **Invalid namespace pattern**: Namespace name doesn't match pattern
3. **Privileged namespace**: Attempting to create privileged namespace
4. **RBAC failure**: Controller lacks permissions to create resources

### RBAC Permission Errors

**Diagnosis**:

```bash
# Check service account
kubectl get serviceaccount pipeline-runner -n <tenant-namespace> -o yaml

# Check role
kubectl get role pipeline-runner -n <tenant-namespace> -o yaml

# Check rolebinding
kubectl get rolebinding pipeline-runner -n <tenant-namespace> -o yaml

# Test permissions
kubectl auth can-i create pods --as=system:serviceaccount:<tenant-namespace>:pipeline-runner -n <tenant-namespace>
```

**Resolution**:
1. Verify Role has necessary permissions
2. Verify RoleBinding associates service account with Role
3. Verify service account is used by PipelineRun

## Maintenance

### Update Platform Components

**Update Tekton**:

```bash
# Check current version
kubectl get deployment tekton-pipelines-controller -n tekton-pipelines -o jsonpath='{.spec.template.spec.containers[0].image}'

# Update to latest
kubectl apply -f https://storage.googleapis.com/tekton-releases/pipeline/latest/release.yaml
```

**Update Lighthouse**:

```bash
# Check current version
helm list -n pipeline-system

# Update Helm chart
helm upgrade lighthouse jx3/lighthouse -n pipeline-system
```

**Update Onboarding Controller**:

```bash
# Build new version
cd platform/onboarding
docker build -t your-registry/onboarding-controller:v1.1.0 .
docker push your-registry/onboarding-controller:v1.1.0

# Update deployment
kubectl set image deployment/onboarding-controller \
  controller=your-registry/onboarding-controller:v1.1.0 \
  -n pipeline-system
```

### Update Pipeline Catalog

```bash
# Update catalog resources
kubectl apply -f platform/catalog/tasks/
kubectl apply -f platform/catalog/pipelines/

# Verify updates
kubectl get tasks,pipelines -n pipeline-catalog
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

### Backup and Recovery

**Backup Platform Configuration**:

```bash
# Backup RepoBindings
kubectl get repobindings -n pipeline-system -o yaml > repobindings-backup.yaml

# Backup allowlist
kubectl get configmap repo-allowlist -n pipeline-system -o yaml > allowlist-backup.yaml

# Backup catalog
kubectl get tasks,pipelines -n pipeline-catalog -o yaml > catalog-backup.yaml
```

**Restore Platform**:

```bash
# Re-run bootstrap
cd platform/bootstrap
./bootstrap.sh

# Restore RepoBindings
kubectl apply -f repobindings-backup.yaml

# Restore allowlist
kubectl apply -f allowlist-backup.yaml

# Restore catalog
kubectl apply -f catalog-backup.yaml
```

## Security

### Rotate GitHub App Credentials

```bash
# Generate new private key in GitHub App settings

# Update secret
kubectl create secret generic github-app-secret \
  --from-file=private-key=path/to/new-private-key.pem \
  -n pipeline-system \
  --dry-run=client -o yaml | kubectl apply -f -

# Restart Lighthouse
kubectl rollout restart deployment lighthouse -n pipeline-system
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

## Runbooks

### Bootstrap Runbook

This runbook provides step-by-step instructions for bootstrapping the Jenkins X platform on a fresh Kubernetes cluster.

**Prerequisites**

Before starting the bootstrap process, verify the following prerequisites are met:

1. **Kubernetes Cluster**:
   - Version 1.24 or higher
   - RBAC enabled
   - NetworkPolicy support enabled
   - Sufficient resources (minimum 4 CPU cores, 8GB RAM)

2. **Local Tools**:
   - `kubectl` installed and configured with cluster admin access
   - `helm` version 3.x installed
   - `docker` installed (for building images)
   - `git` installed

3. **GitHub Access**:
   - Admin access to GitHub organization
   - Ability to create GitHub Apps

4. **Container Registry**:
   - Access to a container registry (Docker Hub, GitHub Container Registry, or private registry)
   - Credentials configured for pushing images

**Verification Script**:

```bash
cd platform/bootstrap
./prerequisites.sh
```

This script checks:
- Kubernetes cluster connectivity
- Kubernetes version compatibility
- kubectl configuration
- helm installation
- Required cluster features (RBAC, NetworkPolicy)

**Bootstrap Steps**

**Step 1: Create Platform Namespaces**

Create the three core platform namespaces:

```bash
# Create pipeline-system namespace (core platform components)
kubectl apply -f platform/bootstrap/namespace-pipeline-system.yaml

# Create pipeline-catalog namespace (shared pipeline resources)
kubectl apply -f platform/bootstrap/namespace-pipeline-catalog.yaml

# Create auth-system namespace (OIDC provider)
kubectl apply -f platform/bootstrap/namespace-auth-system.yaml

# Verify namespaces
kubectl get namespaces | grep -E "pipeline-system|pipeline-catalog|auth-system"
```

Expected output: All three namespaces should be in "Active" status.

**Step 2: Deploy OIDC Provider (Dex)**

Deploy Dex for OIDC authentication:

```bash
# Apply Dex configuration
kubectl apply -f platform/bootstrap/dex-config.yaml

# Deploy Dex
kubectl apply -f platform/bootstrap/dex-deployment.yaml

# Wait for Dex to be ready
kubectl wait --for=condition=ready pod -l app=dex -n auth-system --timeout=300s

# Verify Dex is running
kubectl get pods -n auth-system
```

Expected output: Dex pod should be in "Running" status with 1/1 ready.

**Step 3: Configure Kubernetes API Server for OIDC**

Configure the Kubernetes API server to use Dex for authentication:

**For k3s clusters**:

```bash
# Edit k3s configuration
sudo vi /etc/rancher/k3s/config.yaml

# Add OIDC configuration (see platform/bootstrap/k8s-apiserver-oidc-config.md)
# Restart k3s
sudo systemctl restart k3s
```

**For kubeadm clusters**:

```bash
# Edit kube-apiserver manifest
sudo vi /etc/kubernetes/manifests/kube-apiserver.yaml

# Add OIDC flags (see platform/bootstrap/k8s-apiserver-oidc-config.md)
# API server will restart automatically
```

Verify OIDC configuration:

```bash
# Check API server logs for OIDC initialization
kubectl logs -n kube-system kube-apiserver-<node-name> | grep oidc
```

**Step 4: Configure RBAC for Engineering Group**

Create RBAC for the engineering OIDC group:

```bash
# Apply engineering RBAC
kubectl apply -f platform/bootstrap/engineering-rbac.yaml

# Verify ClusterRole and ClusterRoleBinding
kubectl get clusterrole repo-onboarder
kubectl get clusterrolebinding engineering-onboarders
```

**Step 5: Install Tekton Pipelines**

Install Tekton as the pipeline execution engine:

```bash
# Install Tekton Pipelines
kubectl apply -f https://storage.googleapis.com/tekton-releases/pipeline/latest/release.yaml

# Wait for Tekton controllers to be ready
kubectl wait --for=condition=ready pod -l app=tekton-pipelines-controller -n tekton-pipelines --timeout=300s
kubectl wait --for=condition=ready pod -l app=tekton-pipelines-webhook -n tekton-pipelines --timeout=300s

# Verify Tekton installation
kubectl get pods -n tekton-pipelines
```

Expected output: All Tekton pods should be in "Running" status.

**Step 6: Create GitHub App**

Create a GitHub App for webhook delivery:

1. Navigate to GitHub Organization Settings → Developer settings → GitHub Apps
2. Click "New GitHub App"
3. Follow the instructions in `platform/bootstrap/github-app-setup.md`
4. Note the App ID and Installation ID
5. Download the private key

Store credentials:

```bash
# Create secret with GitHub App private key
kubectl create secret generic github-app-secret \
  --from-file=private-key=path/to/private-key.pem \
  -n pipeline-system

# Verify secret
kubectl get secret github-app-secret -n pipeline-system
```

**Step 7: Install Jenkins X and Lighthouse**

Install Jenkins X components:

```bash
# Add Jenkins X Helm repository
helm repo add jx3 https://jenkins-x-charts.github.io/repo
helm repo update

# Install jx-build-controller
helm install jx jx3/jx-build-controller -n pipeline-system

# Wait for jx-build-controller
kubectl wait --for=condition=ready pod -l app=jx-build-controller -n pipeline-system --timeout=300s
```

Configure and install Lighthouse:

```bash
# Create Lighthouse configuration
kubectl apply -f platform/lighthouse/lighthouse-config.yaml

# Create initial allowlist
kubectl apply -f platform/lighthouse/repo-allowlist.yaml

# Install Lighthouse
helm install lighthouse jx3/lighthouse -n pipeline-system \
  --set github.appId="${GITHUB_APP_ID}" \
  --set github.appInstallationId="${GITHUB_APP_INSTALLATION_ID}"

# Wait for Lighthouse
kubectl wait --for=condition=ready pod -l app=lighthouse -n pipeline-system --timeout=300s

# Verify Lighthouse
kubectl get pods -n pipeline-system -l app=lighthouse
```

**Step 8: Install RepoBinding CRD**

Install the custom resource definition for repository onboarding:

```bash
# Apply RepoBinding CRD
kubectl apply -f platform/crds/repobinding.yaml

# Verify CRD registration
kubectl get crd repobindings.platform.arbiter.io

# Test CRD with example
kubectl apply -f platform/crds/example-repobinding.yaml
kubectl delete repobinding example-binding -n pipeline-system
```

**Step 9: Deploy Onboarding Controller**

Build and deploy the onboarding controller:

```bash
# Build controller image
cd platform/onboarding/controller
docker build -t your-registry/onboarding-controller:latest .
docker push your-registry/onboarding-controller:latest

# Update image reference in deployment manifest
# Edit platform/onboarding/controller-deployment.yaml

# Deploy controller RBAC
kubectl apply -f platform/onboarding/controller-service-account.yaml
kubectl apply -f platform/onboarding/controller-rbac.yaml

# Deploy controller
kubectl apply -f platform/onboarding/controller-deployment.yaml

# Wait for controller
kubectl wait --for=condition=ready pod -l app=onboarding-controller -n pipeline-system --timeout=300s

# Verify controller
kubectl get pods -n pipeline-system -l app=onboarding-controller
kubectl logs -n pipeline-system -l app=onboarding-controller --tail=20
```

**Step 10: Install Pipeline Catalog**

Install shared pipeline resources:

```bash
# Apply catalog Tasks
kubectl apply -f platform/catalog/tasks/

# Apply catalog Pipelines
kubectl apply -f platform/catalog/pipelines/

# Verify catalog installation
kubectl get tasks -n pipeline-catalog
kubectl get pipelines -n pipeline-catalog
```

Expected output: All Tasks and Pipelines should be created successfully.

**Step 11: Build and Push Runner Image**

Build the container image used for pipeline execution:

```bash
# Build runner image
cd platform/catalog/images/runner
./build.sh

# Push runner image
./push.sh

# Verify image is accessible
docker pull your-registry/pipeline-runner:latest
```

**Step 12: Verify Bootstrap**

Run the verification script to ensure all components are healthy:

```bash
cd platform/bootstrap
./verify-platform.sh
```

This script checks:
- All platform namespaces exist
- All platform pods are running
- Tekton controllers are healthy
- Lighthouse is healthy
- Onboarding controller is healthy
- RepoBinding CRD is registered
- Pipeline catalog is installed

**Verification Checklist**:

- [ ] All platform namespaces created (pipeline-system, pipeline-catalog, auth-system)
- [ ] Dex is running and accessible
- [ ] Kubernetes API server configured for OIDC
- [ ] Engineering RBAC configured
- [ ] Tekton controllers running
- [ ] GitHub App created and credentials stored
- [ ] Lighthouse running and configured
- [ ] RepoBinding CRD registered
- [ ] Onboarding controller running
- [ ] Pipeline catalog installed
- [ ] Runner image built and pushed

**Common Bootstrap Issues**

**Issue: Tekton installation fails**

Symptoms: Tekton pods fail to start or remain in "Pending" status.

Resolution:
1. Check cluster resources: `kubectl top nodes`
2. Check pod events: `kubectl describe pod <pod-name> -n tekton-pipelines`
3. Verify cluster has sufficient CPU and memory
4. Check for image pull errors

**Issue: Lighthouse cannot connect to GitHub**

Symptoms: Lighthouse logs show authentication errors.

Resolution:
1. Verify GitHub App credentials: `kubectl get secret github-app-secret -n pipeline-system`
2. Verify App ID and Installation ID in Lighthouse config
3. Check GitHub App permissions and webhook configuration
4. Verify webhook URL is accessible from GitHub

**Issue: Onboarding controller fails to start**

Symptoms: Controller pod crashes or fails to start.

Resolution:
1. Check controller logs: `kubectl logs -n pipeline-system -l app=onboarding-controller`
2. Verify RBAC permissions: `kubectl get clusterrole onboarding-controller`
3. Verify CRD is registered: `kubectl get crd repobindings.platform.arbiter.io`
4. Check for image pull errors

**Issue: OIDC authentication not working**

Symptoms: Users cannot authenticate with kubectl.

Resolution:
1. Verify Dex is running: `kubectl get pods -n auth-system`
2. Check API server OIDC configuration
3. Verify Dex issuer URL is accessible
4. Check Dex logs: `kubectl logs -n auth-system -l app=dex`
5. Verify user is in engineering group

**Post-Bootstrap Steps**

After successful bootstrap:

1. **Test onboarding workflow**: Create a test RepoBinding to verify the onboarding process
2. **Configure monitoring**: Set up CloudWatch or Prometheus for platform monitoring
3. **Document platform details**: Update `.kiro/docs/` with platform-specific details (URLs, credentials location)
4. **Train users**: Provide documentation and training for engineering team on onboarding process

**Source**
- `platform/bootstrap/bootstrap.sh`
- `platform/bootstrap/prerequisites.sh`
- `platform/bootstrap/verify-platform.sh`
- `platform/bootstrap/github-app-setup.md`
- `.kiro/specs/jenkinsx-platform/requirements.md` (Requirement 1.7, 20.1, 20.4)

### Onboarding Runbook

This runbook provides step-by-step instructions for onboarding a new repository to the Jenkins X platform.

**Overview**

Repository onboarding provisions all resources required for a product team to run pipelines:
- Tenant namespace with isolation policies
- Service account with least-privilege RBAC
- Resource quotas and limits
- Network policies
- Terraform backend configuration
- Repository allowlist entry

**Prerequisites**

Before onboarding a repository:

1. **User Authentication**:
   - User must be authenticated via OIDC
   - User must be member of the engineering group
   - User must have kubectl configured with OIDC credentials

2. **Repository Requirements**:
   - Repository must exist in an approved GitHub organization
   - Repository must contain pipeline definition YAML (`.lighthouse/jenkins-x/`)
   - Repository must be ready to receive webhooks

3. **Naming Requirements**:
   - Tenant name must match pattern: `^[a-z0-9-]+$` (lowercase alphanumeric and hyphens)
   - Tenant name must not be a privileged namespace (kube-system, pipeline-system, etc.)

**How to Create RepoBinding**

**Step 1: Prepare RepoBinding Manifest**

Create a YAML file with the RepoBinding specification:

```yaml
apiVersion: platform.arbiter.io/v1alpha1
kind: RepoBinding
metadata:
  name: <repository-name>-binding
  namespace: pipeline-system
spec:
  repoOrg: "<github-organization>"
  repoName: "<repository-name>"
  tenantName: "<tenant-namespace-name>"
  permissionProfile: "standard"  # or "elevated"
```

**Field Descriptions**:
- `metadata.name`: Unique name for this RepoBinding (convention: `<repo>-binding`)
- `spec.repoOrg`: GitHub organization name (must be in approved list)
- `spec.repoName`: GitHub repository name
- `spec.tenantName`: Kubernetes namespace name for this tenant (lowercase, alphanumeric, hyphens)
- `spec.permissionProfile`: Permission level (`standard` or `elevated`)

**Permission Profiles**:
- **standard**: Namespace-scoped permissions for pods, configmaps, secrets, PipelineRuns
- **elevated**: Additional permissions for advanced use cases (use sparingly)

**Example RepoBinding**:

```yaml
apiVersion: platform.arbiter.io/v1alpha1
kind: RepoBinding
metadata:
  name: archon-agent-binding
  namespace: pipeline-system
spec:
  repoOrg: "your-github-org"
  repoName: "archon-agent"
  tenantName: "archon"
  permissionProfile: "standard"
```

**Step 2: Apply RepoBinding**

Apply the RepoBinding to the cluster:

```bash
# Apply RepoBinding
kubectl apply -f repobinding.yaml

# Verify RepoBinding was created
kubectl get repobinding <name> -n pipeline-system
```

Expected output: RepoBinding should be created with status "Pending" or "Provisioning".

**Step 3: Monitor Onboarding Progress**

Watch the RepoBinding status as the onboarding controller provisions resources:

```bash
# Watch RepoBinding status
kubectl get repobinding <name> -n pipeline-system -w

# View detailed status
kubectl describe repobinding <name> -n pipeline-system
```

The onboarding controller will:
1. Validate the request (organization, namespace pattern, permission profile)
2. Create tenant namespace
3. Create service account
4. Create RBAC (Role and RoleBinding)
5. Create ResourceQuota and LimitRange
6. Create NetworkPolicy
7. Create Terraform backend secret
8. Update repository allowlist
9. Update RepoBinding status to "Ready"

**How to Verify Onboarding**

After the RepoBinding status shows "Ready", verify all resources were created correctly.

**Verification Checklist**:

**1. Verify RepoBinding Status**

```bash
# Check RepoBinding status
kubectl get repobinding <name> -n pipeline-system

# Expected output: STATUS should be "Ready"
# Check detailed status
kubectl describe repobinding <name> -n pipeline-system
```

Expected status fields:
- `phase: Ready`
- `namespaceCreated: true`
- `serviceAccountCreated: true`
- `rbacConfigured: true`
- `allowlistUpdated: true`

**2. Verify Tenant Namespace**

```bash
# Check namespace exists
kubectl get namespace <tenant-name>

# Verify namespace labels
kubectl get namespace <tenant-name> -o yaml | grep -A 5 labels
```

Expected labels:
- `platform.arbiter.io/tenant: <tenant-name>`
- `platform.arbiter.io/repo: <org>/<repo>`
- `platform.arbiter.io/managed-by: onboarding-controller`

**3. Verify Service Account**

```bash
# Check service account exists
kubectl get serviceaccount pipeline-runner -n <tenant-name>

# Verify service account details
kubectl describe serviceaccount pipeline-runner -n <tenant-name>
```

Expected output: Service account should exist with no errors.

**4. Verify RBAC Configuration**

```bash
# Check Role exists
kubectl get role pipeline-runner -n <tenant-name>

# Check RoleBinding exists
kubectl get rolebinding pipeline-runner -n <tenant-name>

# Verify Role permissions
kubectl get role pipeline-runner -n <tenant-name> -o yaml

# Test service account permissions
kubectl auth can-i create pods \
  --as=system:serviceaccount:<tenant-name>:pipeline-runner \
  -n <tenant-name>
```

Expected output: Service account should have permissions to create pods, configmaps, secrets, and PipelineRuns in the tenant namespace only.

**5. Verify Resource Limits**

```bash
# Check ResourceQuota exists
kubectl get resourcequota tenant-quota -n <tenant-name>

# Check LimitRange exists
kubectl get limitrange tenant-limits -n <tenant-name>

# View ResourceQuota details
kubectl describe resourcequota tenant-quota -n <tenant-name>

# View LimitRange details
kubectl describe limitrange tenant-limits -n <tenant-name>
```

Expected ResourceQuota limits:
- CPU requests: 4 cores
- Memory requests: 8Gi
- CPU limits: 8 cores
- Memory limits: 16Gi
- Pods: 20

**6. Verify Network Policy**

```bash
# Check NetworkPolicy exists
kubectl get networkpolicy tenant-isolation -n <tenant-name>

# View NetworkPolicy details
kubectl describe networkpolicy tenant-isolation -n <tenant-name>
```

Expected policy:
- Ingress: Allow from same namespace only
- Egress: Allow to same namespace, DNS (kube-system), and internet

**7. Verify Terraform Backend Secret**

```bash
# Check secret exists
kubectl get secret terraform-backend-config -n <tenant-name>

# Verify secret contains backend configuration
kubectl get secret terraform-backend-config -n <tenant-name> -o yaml
```

Expected secret data:
- `backend.tf`: Terraform backend configuration
- `credentials`: Backend credentials (if using S3/MinIO)

**8. Verify Allowlist Update**

```bash
# Check repository is in allowlist
kubectl get configmap repo-allowlist -n pipeline-system -o yaml | grep "<repo-name>"

# View full allowlist
kubectl get configmap repo-allowlist -n pipeline-system -o yaml
```

Expected entry:
```yaml
- org: "<github-org>"
  name: "<repo-name>"
  tenant: "<tenant-name>"
  enabled: true
```

**9. Verify Cross-Namespace Isolation**

Test that the tenant service account cannot access other namespaces:

```bash
# Test access to another tenant namespace (should fail)
kubectl auth can-i get pods \
  --as=system:serviceaccount:<tenant-name>:pipeline-runner \
  -n <other-tenant-name>

# Expected output: "no"

# Test access to platform namespace (should fail)
kubectl auth can-i get pods \
  --as=system:serviceaccount:<tenant-name>:pipeline-runner \
  -n pipeline-system

# Expected output: "no"
```

**Complete Verification Script**:

```bash
#!/bin/bash
# verify-onboarding.sh

TENANT_NAME=$1
REPO_NAME=$2

echo "Verifying onboarding for tenant: $TENANT_NAME"

# Check RepoBinding status
echo "1. Checking RepoBinding status..."
kubectl get repobinding ${REPO_NAME}-binding -n pipeline-system

# Check namespace
echo "2. Checking namespace..."
kubectl get namespace $TENANT_NAME

# Check service account
echo "3. Checking service account..."
kubectl get serviceaccount pipeline-runner -n $TENANT_NAME

# Check RBAC
echo "4. Checking RBAC..."
kubectl get role,rolebinding -n $TENANT_NAME

# Check resource limits
echo "5. Checking resource limits..."
kubectl get resourcequota,limitrange -n $TENANT_NAME

# Check network policy
echo "6. Checking network policy..."
kubectl get networkpolicy -n $TENANT_NAME

# Check Terraform backend secret
echo "7. Checking Terraform backend secret..."
kubectl get secret terraform-backend-config -n $TENANT_NAME

# Check allowlist
echo "8. Checking allowlist..."
kubectl get configmap repo-allowlist -n pipeline-system -o yaml | grep $REPO_NAME

echo "Verification complete!"
```

**Common Onboarding Errors**

**Error: Invalid Organization**

Symptoms: RepoBinding status shows "Failed" with message "Repository organization not in approved list".

Resolution:
1. Verify the GitHub organization name is correct
2. Check the approved organization list in the onboarding controller configuration
3. Contact platform team to add organization to approved list if needed

**Error: Invalid Namespace Pattern**

Symptoms: RepoBinding status shows "Failed" with message "Namespace name must match pattern".

Resolution:
1. Verify tenant name uses only lowercase letters, numbers, and hyphens
2. Verify tenant name doesn't start or end with a hyphen
3. Update RepoBinding with a valid tenant name

**Error: Privileged Namespace**

Symptoms: RepoBinding status shows "Failed" with message "Cannot create privileged namespace".

Resolution:
1. Verify tenant name is not a reserved namespace (kube-system, pipeline-system, etc.)
2. Choose a different tenant name
3. Update RepoBinding with the new name

**Error: Namespace Already Exists**

Symptoms: RepoBinding status shows "Failed" with message "Namespace already exists".

Resolution:
1. Check if namespace was created by a previous onboarding: `kubectl get namespace <tenant-name>`
2. If namespace exists but RepoBinding is missing, delete and recreate RepoBinding
3. If namespace exists from another source, choose a different tenant name

**Error: RBAC Creation Failed**

Symptoms: RepoBinding status shows "Provisioning" but never reaches "Ready", controller logs show RBAC errors.

Resolution:
1. Check controller logs: `kubectl logs -n pipeline-system -l app=onboarding-controller`
2. Verify controller has ClusterRole permissions to create Roles and RoleBindings
3. Check for conflicting Role or RoleBinding names in the tenant namespace

**Error: Allowlist Update Failed**

Symptoms: RepoBinding status shows "Ready" but `allowlistUpdated: false`.

Resolution:
1. Check controller logs for allowlist update errors
2. Verify allowlist ConfigMap exists: `kubectl get configmap repo-allowlist -n pipeline-system`
3. Verify controller has permissions to update ConfigMaps in pipeline-system namespace
4. Manually update allowlist if needed and restart controller

**Post-Onboarding Steps**

After successful onboarding:

1. **Configure Repository Pipeline**:
   - Add pipeline definition YAML to repository (`.lighthouse/jenkins-x/`)
   - Configure trigger rules (e.g., on merge to main)
   - Reference golden pipeline catalog for CDKTF deployment

2. **Test Pipeline Execution**:
   - Merge a commit to main branch
   - Verify GitHub App delivers webhook to Lighthouse
   - Verify PipelineRun is created in tenant namespace
   - Monitor pipeline execution and logs

3. **Configure Monitoring**:
   - Set up alerts for pipeline failures
   - Configure log aggregation for tenant namespace
   - Document monitoring procedures for the team

4. **Train Team**:
   - Provide documentation on pipeline usage
   - Explain resource limits and quotas
   - Document troubleshooting procedures

**Offboarding a Repository**

To remove a repository from the platform:

```bash
# Delete RepoBinding
kubectl delete repobinding <name> -n pipeline-system

# Manually clean up tenant namespace (if desired)
kubectl delete namespace <tenant-name>

# Remove from allowlist (manual step)
kubectl edit configmap repo-allowlist -n pipeline-system
# Remove repository entry and save
```

**Note**: The onboarding controller does not automatically delete tenant namespaces when RepoBindings are deleted. This is intentional to prevent accidental data loss. Namespaces must be manually deleted if desired.

**Source**
- `platform/onboarding/controller/controllers/repobinding_controller.go`
- `platform/crds/repobinding.yaml`
- `platform/tenancy/templates/`
- `.kiro/specs/jenkinsx-platform/requirements.md` (Requirement 1.7, 4.5, 9.1-9.5)

### Pipeline Troubleshooting Runbook

This runbook provides systematic procedures for debugging pipeline failures and common pipeline issues.

**Overview**

Pipeline failures can occur at multiple stages:
- Webhook delivery from GitHub to Lighthouse
- PipelineRun creation by Lighthouse
- Pipeline execution (git clone, build, deploy)
- Terraform state management
- RBAC permission issues

This runbook provides step-by-step diagnosis and resolution procedures for each failure mode.

**How to Debug Pipeline Failures**

**Step 1: Identify the Failure Stage**

Determine where in the pipeline lifecycle the failure occurred:

```bash
# Check if PipelineRun was created
kubectl get pipelineruns -n <tenant-namespace> --sort-by=.metadata.creationTimestamp

# If no PipelineRun exists, the issue is in webhook delivery or Lighthouse
# If PipelineRun exists, check its status
kubectl get pipelinerun <name> -n <tenant-namespace>
```

**PipelineRun Status Values**:
- **Pending**: PipelineRun created but not started (check pod scheduling)
- **Running**: Pipeline is executing (check task logs)
- **Succeeded**: Pipeline completed successfully
- **Failed**: Pipeline failed (check task logs and events)
- **Cancelled**: Pipeline was cancelled by user or system

**Step 2: Check Webhook Delivery (No PipelineRun Created)**

If no PipelineRun was created after a merge to main:

```bash
# Check Lighthouse logs for webhook events
kubectl logs -n pipeline-system -l app=lighthouse --tail=100 | grep "<repo-name>"

# Check for webhook delivery from GitHub
# Look for log entries like: "Received webhook for repo: <org>/<repo>"

# Check if repository is in allowlist
kubectl get configmap repo-allowlist -n pipeline-system -o yaml | grep "<repo-name>"
```

**Common Issues**:

1. **Repository not in allowlist**: Add repository to allowlist via RepoBinding
2. **GitHub App not delivering webhooks**: Check GitHub App webhook delivery logs
3. **Lighthouse not processing events**: Check Lighthouse pod status and logs
4. **Trigger rules not matching**: Check repository pipeline definition YAML

**Step 3: Check PipelineRun Creation (PipelineRun Exists but Failed)**

If PipelineRun was created but failed:

```bash
# Get PipelineRun status
kubectl get pipelinerun <name> -n <tenant-namespace>

# Get detailed status with conditions
kubectl describe pipelinerun <name> -n <tenant-namespace>

# Check PipelineRun YAML for error messages
kubectl get pipelinerun <name> -n <tenant-namespace> -o yaml
```

Look for:
- `status.conditions[].message`: Error message describing failure
- `status.conditions[].reason`: Failure reason code
- `status.taskRuns`: Status of individual tasks

**Step 4: Check Task Execution (Task Failed)**

If a specific task failed:

```bash
# List all TaskRuns for the PipelineRun
kubectl get taskruns -n <tenant-namespace> -l tekton.dev/pipelineRun=<pipelinerun-name>

# Get TaskRun status
kubectl describe taskrun <taskrun-name> -n <tenant-namespace>

# Get pod name for the TaskRun
kubectl get pods -n <tenant-namespace> -l tekton.dev/taskRun=<taskrun-name>

# View task logs
kubectl logs <pod-name> -n <tenant-namespace>

# View logs for specific step
kubectl logs <pod-name> -n <tenant-namespace> -c <step-name>
```

**Common Task Failures**:

1. **git-clone failure**: Repository access issues, invalid commit SHA
2. **cdktf-synth failure**: Node.js dependency issues, syntax errors
3. **cdktf-deploy failure**: Terraform errors, AWS permission issues, state lock conflicts
4. **Image pull failure**: Container registry access issues

**How to Access Logs**

**Access PipelineRun Logs**:

```bash
# Get all logs for a PipelineRun
kubectl logs -n <tenant-namespace> -l tekton.dev/pipelineRun=<pipelinerun-name>

# Get logs for specific task
kubectl logs -n <tenant-namespace> -l tekton.dev/pipelineRun=<pipelinerun-name>,tekton.dev/pipelineTask=<task-name>

# Stream logs in real-time
kubectl logs -n <tenant-namespace> -l tekton.dev/pipelineRun=<pipelinerun-name> -f

# Get logs from all containers in a pod
kubectl logs <pod-name> -n <tenant-namespace> --all-containers=true
```

**Access Logs for Specific Steps**:

Each Tekton Task has multiple steps. To view logs for a specific step:

```bash
# List all containers in the pod
kubectl get pod <pod-name> -n <tenant-namespace> -o jsonpath='{.spec.containers[*].name}'

# View logs for specific step
kubectl logs <pod-name> -n <tenant-namespace> -c step-<step-name>

# Example: View git-clone step logs
kubectl logs <pod-name> -n <tenant-namespace> -c step-clone
```

**Access Historical Logs**:

```bash
# List all PipelineRuns (including completed)
kubectl get pipelineruns -n <tenant-namespace> --sort-by=.metadata.creationTimestamp

# View logs from completed PipelineRun
kubectl logs -n <tenant-namespace> -l tekton.dev/pipelineRun=<pipelinerun-name>

# Note: Logs are retained based on pod retention policy
# Old pods may be garbage collected
```

**Export Logs for Analysis**:

```bash
# Export all logs for a PipelineRun to file
kubectl logs -n <tenant-namespace> -l tekton.dev/pipelineRun=<pipelinerun-name> > pipelinerun-logs.txt

# Export logs with timestamps
kubectl logs -n <tenant-namespace> -l tekton.dev/pipelineRun=<pipelinerun-name> --timestamps > pipelinerun-logs.txt

# Export logs from all tasks
for taskrun in $(kubectl get taskruns -n <tenant-namespace> -l tekton.dev/pipelineRun=<pipelinerun-name> -o name); do
  echo "=== $taskrun ===" >> pipelinerun-logs.txt
  kubectl logs -n <tenant-namespace> -l tekton.dev/taskRun=$(basename $taskrun) >> pipelinerun-logs.txt
done
```

**Common Pipeline Errors**

**Error: Git Clone Failure**

Symptoms: git-clone task fails with authentication or access errors.

Diagnosis:

```bash
# Check git-clone task logs
kubectl logs <pod-name> -n <tenant-namespace> -c step-clone

# Check for error messages like:
# - "Authentication failed"
# - "Repository not found"
# - "Permission denied"
```

Resolution:

1. **Public repository**: Verify repository URL is correct and repository is public
2. **Private repository**: Verify GitHub App has access to the repository
3. **Invalid commit SHA**: Verify the commit SHA exists in the repository
4. **Network issues**: Check NetworkPolicy allows egress to GitHub

**Error: CDKTF Synth Failure**

Symptoms: cdktf-synth task fails with Node.js or TypeScript errors.

Diagnosis:

```bash
# Check cdktf-synth task logs
kubectl logs <pod-name> -n <tenant-namespace> -c step-synth

# Look for error messages like:
# - "npm install failed"
# - "TypeScript compilation error"
# - "Module not found"
```

Resolution:

1. **Dependency issues**: Verify `package.json` has correct dependencies
2. **Syntax errors**: Fix TypeScript syntax errors in CDKTF code
3. **Missing dependencies**: Add missing dependencies to `package.json`
4. **Node.js version**: Verify runner image has compatible Node.js version

**Error: CDKTF Deploy Failure**

Symptoms: cdktf-deploy task fails with Terraform errors.

Diagnosis:

```bash
# Check cdktf-deploy task logs
kubectl logs <pod-name> -n <tenant-namespace> -c step-deploy

# Look for error messages like:
# - "Error acquiring state lock"
# - "Error creating resource"
# - "Insufficient permissions"
# - "Resource already exists"
```

Resolution:

1. **State lock conflict**: Wait for other deployment to complete, or manually unlock state
2. **AWS permission errors**: Verify service account has correct AWS credentials
3. **Resource conflicts**: Resolve resource naming conflicts or import existing resources
4. **Terraform syntax errors**: Fix Terraform configuration errors

**Error: State Lock Conflict**

Symptoms: cdktf-deploy fails with "Error acquiring state lock" message.

Diagnosis:

```bash
# Check if another PipelineRun is running for the same tenant
kubectl get pipelineruns -n <tenant-namespace> --field-selector=status.conditions[0].status=Unknown

# Check Terraform state backend for lock information
# For Kubernetes backend:
kubectl get lease -n <tenant-namespace> | grep terraform
```

Resolution:

1. **Wait for concurrent deployment**: Wait for the other deployment to complete
2. **Manual unlock** (if deployment crashed):
   ```bash
   # For Kubernetes backend
   kubectl delete lease <lease-name> -n <tenant-namespace>
   ```
3. **Verify serialization**: Ensure only one cdktf-deploy runs at a time per tenant

**Error: RBAC Permission Denied**

Symptoms: Pipeline fails with "Forbidden" or "permission denied" errors.

Diagnosis:

```bash
# Check pod events for RBAC errors
kubectl get events -n <tenant-namespace> --sort-by='.lastTimestamp' | grep -i forbidden

# Check service account permissions
kubectl auth can-i <verb> <resource> \
  --as=system:serviceaccount:<tenant-namespace>:pipeline-runner \
  -n <tenant-namespace>

# Example: Check if service account can create pods
kubectl auth can-i create pods \
  --as=system:serviceaccount:<tenant-namespace>:pipeline-runner \
  -n <tenant-namespace>
```

Resolution:

1. **Missing permissions**: Update Role to include required permissions
2. **Wrong service account**: Verify PipelineRun uses correct service account
3. **Cross-namespace access**: Verify pipeline doesn't attempt cross-namespace access

**Error: Image Pull Failure**

Symptoms: Pipeline pod fails to start with "ImagePullBackOff" or "ErrImagePull" status.

Diagnosis:

```bash
# Check pod status
kubectl get pods -n <tenant-namespace>

# Check pod events
kubectl describe pod <pod-name> -n <tenant-namespace> | grep -A 10 Events

# Look for error messages like:
# - "Failed to pull image"
# - "Authentication required"
# - "Image not found"
```

Resolution:

1. **Image doesn't exist**: Verify runner image was built and pushed
2. **Registry authentication**: Verify cluster has credentials for private registry
3. **Image name typo**: Verify image name in pipeline definition is correct
4. **Network issues**: Check NetworkPolicy allows egress to container registry

**Error: Resource Quota Exceeded**

Symptoms: Pipeline pod fails to start with "Forbidden: exceeded quota" error.

Diagnosis:

```bash
# Check ResourceQuota status
kubectl describe resourcequota tenant-quota -n <tenant-namespace>

# Check current resource usage
kubectl top pods -n <tenant-namespace>

# Check pod events
kubectl get events -n <tenant-namespace> --sort-by='.lastTimestamp' | grep -i quota
```

Resolution:

1. **Clean up old PipelineRuns**: Delete completed PipelineRuns to free resources
2. **Reduce resource requests**: Adjust pipeline resource requests if too high
3. **Increase quota**: Contact platform team to increase ResourceQuota if needed

**Error: Webhook Not Delivered**

Symptoms: No PipelineRun created after merge to main.

Diagnosis:

```bash
# Check Lighthouse logs for webhook events
kubectl logs -n pipeline-system -l app=lighthouse --tail=100

# Check GitHub App webhook delivery logs
# Go to GitHub App settings → Advanced → Recent Deliveries

# Check if repository is in allowlist
kubectl get configmap repo-allowlist -n pipeline-system -o yaml | grep "<repo-name>"
```

Resolution:

1. **Repository not in allowlist**: Create RepoBinding to add repository
2. **GitHub App not installed**: Install GitHub App at organization level
3. **Webhook URL incorrect**: Verify webhook URL in GitHub App settings
4. **Lighthouse not running**: Check Lighthouse pod status
5. **Network issues**: Verify Lighthouse is accessible from GitHub

**Diagnostic Scripts**

**Script: Diagnose Pipeline Failure**

```bash
#!/bin/bash
# diagnose-pipeline.sh

TENANT_NAMESPACE=$1
PIPELINERUN_NAME=$2

echo "Diagnosing PipelineRun: $PIPELINERUN_NAME in namespace: $TENANT_NAMESPACE"

# Check PipelineRun status
echo "=== PipelineRun Status ==="
kubectl get pipelinerun $PIPELINERUN_NAME -n $TENANT_NAMESPACE

# Check PipelineRun conditions
echo "=== PipelineRun Conditions ==="
kubectl get pipelinerun $PIPELINERUN_NAME -n $TENANT_NAMESPACE -o jsonpath='{.status.conditions[*]}' | jq

# Check TaskRuns
echo "=== TaskRuns ==="
kubectl get taskruns -n $TENANT_NAMESPACE -l tekton.dev/pipelineRun=$PIPELINERUN_NAME

# Check pods
echo "=== Pods ==="
kubectl get pods -n $TENANT_NAMESPACE -l tekton.dev/pipelineRun=$PIPELINERUN_NAME

# Check events
echo "=== Recent Events ==="
kubectl get events -n $TENANT_NAMESPACE --sort-by='.lastTimestamp' | tail -20

# Get logs from failed tasks
echo "=== Logs from Failed Tasks ==="
for taskrun in $(kubectl get taskruns -n $TENANT_NAMESPACE -l tekton.dev/pipelineRun=$PIPELINERUN_NAME -o name); do
  status=$(kubectl get $taskrun -n $TENANT_NAMESPACE -o jsonpath='{.status.conditions[0].status}')
  if [ "$status" != "True" ]; then
    echo "--- Logs for $taskrun ---"
    kubectl logs -n $TENANT_NAMESPACE -l tekton.dev/taskRun=$(basename $taskrun) --tail=50
  fi
done

echo "Diagnosis complete!"
```

**Script: Check Repository Webhook Delivery**

```bash
#!/bin/bash
# check-webhook-delivery.sh

REPO_ORG=$1
REPO_NAME=$2

echo "Checking webhook delivery for: $REPO_ORG/$REPO_NAME"

# Check if repository is in allowlist
echo "=== Allowlist Check ==="
kubectl get configmap repo-allowlist -n pipeline-system -o yaml | grep -A 2 "$REPO_NAME"

# Check Lighthouse logs for recent events
echo "=== Recent Lighthouse Events ==="
kubectl logs -n pipeline-system -l app=lighthouse --tail=100 | grep "$REPO_NAME"

# Check Lighthouse pod status
echo "=== Lighthouse Status ==="
kubectl get pods -n pipeline-system -l app=lighthouse

echo "Check complete!"
echo "If no events found, verify GitHub App webhook delivery in GitHub settings"
```

**Escalation Procedures**

When to escalate to platform team:

1. **Platform component failures**: Lighthouse, Tekton, or onboarding controller not running
2. **Cluster-level issues**: Node failures, network issues, storage issues
3. **RBAC issues**: Controller lacks permissions to create resources
4. **Quota increases**: Tenant needs higher resource limits
5. **Allowlist updates**: Need to add new organization to approved list

**Escalation Information to Provide**:

- Tenant namespace name
- Repository name and organization
- PipelineRun name (if applicable)
- Error messages from logs
- Output from diagnostic scripts
- Steps already attempted to resolve

**Source**
- `platform/catalog/tasks/`
- `platform/catalog/pipelines/`
- `.kiro/specs/jenkinsx-platform/requirements.md` (Requirement 16.1, 16.2, 16.3, 16.4)
- `.kiro/specs/jenkinsx-platform/design.md` (Error Handling section)

### Terraform State Backend Runbook

This runbook provides procedures for managing Terraform state backends, including configuration, migration, backup, and recovery.

**Overview**

The Jenkins X platform uses remote Terraform state backends to:
- Persist infrastructure state across pipeline executions
- Enable state locking to prevent concurrent modifications
- Isolate state per tenant for security and reliability
- Enable state backup and recovery

**Supported Backend Types**:
- **Kubernetes Backend** (recommended for homelab): Stores state in Kubernetes Secrets
- **S3-Compatible Backend** (MinIO): Stores state in self-hosted object storage

**Backend Configuration**

**Kubernetes Backend Configuration**

The Kubernetes backend stores Terraform state in Kubernetes Secrets within the tenant namespace.

**Backend Secret Template**:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: terraform-backend-config
  namespace: <tenant-namespace>
type: Opaque
stringData:
  backend.tf: |
    terraform {
      backend "kubernetes" {
        secret_suffix    = "<tenant-name>"
        namespace        = "<tenant-namespace>"
        in_cluster_config = true
      }
    }
```

**Advantages**:
- No external dependencies
- Native state locking via Kubernetes leases
- Simple configuration
- Automatic cleanup when namespace is deleted

**Disadvantages**:
- No built-in versioning
- Limited to Kubernetes cluster storage
- Backup requires Kubernetes backup solution

**S3-Compatible Backend Configuration (MinIO)**

The S3-compatible backend stores Terraform state in MinIO (self-hosted S3-compatible storage).

**Backend Secret Template**:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: terraform-backend-config
  namespace: <tenant-namespace>
type: Opaque
stringData:
  backend.tf: |
    terraform {
      backend "s3" {
        bucket = "terraform-state-<tenant-name>"
        key    = "state.tfstate"
        region = "us-east-1"
        endpoint = "http://minio.storage-system.svc.cluster.local:9000"
        skip_credentials_validation = true
        skip_metadata_api_check = true
        skip_region_validation = true
        force_path_style = true
      }
    }
  credentials: |
    [default]
    aws_access_key_id = <minio-access-key>
    aws_secret_access_key = <minio-secret-key>
```

**Advantages**:
- Built-in versioning
- Separate storage from Kubernetes
- Standard S3 API for backup/restore
- Can be backed up independently

**Disadvantages**:
- Requires MinIO deployment
- No native state locking (requires pipeline serialization)
- More complex configuration

**Verify Backend Configuration**

```bash
# Check backend secret exists
kubectl get secret terraform-backend-config -n <tenant-namespace>

# View backend configuration
kubectl get secret terraform-backend-config -n <tenant-namespace> -o jsonpath='{.data.backend\.tf}' | base64 -d

# For S3 backend, verify credentials
kubectl get secret terraform-backend-config -n <tenant-namespace> -o jsonpath='{.data.credentials}' | base64 -d
```

**State Migration**

**Migrate from Local State to Remote Backend**

If a repository was using local Terraform state and needs to migrate to remote backend:

**Step 1: Backup Local State**

```bash
# Clone repository
git clone <repository-url>
cd <repository>

# Backup local state file
cp terraform.tfstate terraform.tfstate.backup
```

**Step 2: Configure Remote Backend**

```bash
# Get backend configuration from secret
kubectl get secret terraform-backend-config -n <tenant-namespace> -o jsonpath='{.data.backend\.tf}' | base64 -d > backend.tf

# For S3 backend, get credentials
kubectl get secret terraform-backend-config -n <tenant-namespace> -o jsonpath='{.data.credentials}' | base64 -d > ~/.aws/credentials
```

**Step 3: Initialize Backend**

```bash
# Initialize Terraform with new backend
terraform init -migrate-state

# Terraform will prompt: "Do you want to copy existing state to the new backend?"
# Answer: yes

# Verify state was migrated
terraform state list
```

**Step 4: Verify Migration**

```bash
# For Kubernetes backend, check secret
kubectl get secret tfstate-default-<tenant-name> -n <tenant-namespace>

# For S3 backend, check MinIO
# Use MinIO console or mc client to verify state file exists
```

**Step 5: Remove Local State**

```bash
# Remove local state files
rm terraform.tfstate
rm terraform.tfstate.backup

# Commit backend configuration
git add backend.tf
git commit -m "Migrate to remote Terraform backend"
git push
```

**Migrate Between Backend Types**

To migrate from Kubernetes backend to S3 backend (or vice versa):

**Step 1: Export Current State**

```bash
# Pull current state
terraform state pull > current-state.json

# Backup current state
cp current-state.json state-backup-$(date +%Y%m%d-%H%M%S).json
```

**Step 2: Update Backend Configuration**

```bash
# Update backend.tf with new backend configuration
# For Kubernetes → S3: Replace kubernetes backend with s3 backend
# For S3 → Kubernetes: Replace s3 backend with kubernetes backend
```

**Step 3: Reinitialize Backend**

```bash
# Reinitialize with new backend
terraform init -migrate-state -force-copy

# Verify state was migrated
terraform state list
```

**Step 4: Verify Migration**

```bash
# Verify state in new backend
terraform state pull

# Compare with backup
diff <(cat current-state.json | jq -S .) <(terraform state pull | jq -S .)
```

**Backup and Restore Procedures**

**Backup Kubernetes Backend State**

```bash
# Backup all Terraform state secrets in a namespace
kubectl get secrets -n <tenant-namespace> -l app.kubernetes.io/managed-by=terraform -o yaml > terraform-state-backup-<tenant-name>-$(date +%Y%m%d).yaml

# Backup specific state secret
kubectl get secret tfstate-default-<tenant-name> -n <tenant-namespace> -o yaml > tfstate-backup-$(date +%Y%m%d).yaml

# Backup all tenant namespaces (includes state)
kubectl get namespaces -l platform.arbiter.io/managed-by=onboarding-controller -o yaml > all-tenant-namespaces-$(date +%Y%m%d).yaml
```

**Restore Kubernetes Backend State**

```bash
# Restore state secret
kubectl apply -f tfstate-backup-<date>.yaml

# Verify state was restored
kubectl get secret tfstate-default-<tenant-name> -n <tenant-namespace>

# Verify state contents
kubectl get secret tfstate-default-<tenant-name> -n <tenant-namespace> -o jsonpath='{.data.tfstate}' | base64 -d | jq .
```

**Backup S3 Backend State (MinIO)**

```bash
# Using MinIO client (mc)
mc alias set minio http://minio.storage-system.svc.cluster.local:9000 <access-key> <secret-key>

# Backup single tenant state
mc cp minio/terraform-state-<tenant-name>/state.tfstate ./backups/state-<tenant-name>-$(date +%Y%m%d).tfstate

# Backup all tenant states
mc mirror minio/terraform-state-* ./backups/terraform-states-$(date +%Y%m%d)/

# Enable versioning on bucket (recommended)
mc version enable minio/terraform-state-<tenant-name>
```

**Restore S3 Backend State (MinIO)**

```bash
# Restore single tenant state
mc cp ./backups/state-<tenant-name>-<date>.tfstate minio/terraform-state-<tenant-name>/state.tfstate

# Restore all tenant states
mc mirror ./backups/terraform-states-<date>/ minio/

# Restore specific version (if versioning enabled)
mc cp --version-id <version-id> minio/terraform-state-<tenant-name>/state.tfstate ./restored-state.tfstate
```

**Automated Backup Strategy**

**Daily Backup CronJob (Kubernetes Backend)**:

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: terraform-state-backup
  namespace: pipeline-system
spec:
  schedule: "0 2 * * *"  # Daily at 2 AM
  jobTemplate:
    spec:
      template:
        spec:
          serviceAccountName: backup-service-account
          containers:
          - name: backup
            image: bitnami/kubectl:latest
            command:
            - /bin/bash
            - -c
            - |
              # Backup all Terraform state secrets
              for ns in $(kubectl get namespaces -l platform.arbiter.io/managed-by=onboarding-controller -o jsonpath='{.items[*].metadata.name}'); do
                kubectl get secrets -n $ns -l app.kubernetes.io/managed-by=terraform -o yaml > /backups/terraform-state-$ns-$(date +%Y%m%d).yaml
              done
            volumeMounts:
            - name: backup-storage
              mountPath: /backups
          volumes:
          - name: backup-storage
            persistentVolumeClaim:
              claimName: backup-pvc
          restartPolicy: OnFailure
```

**State Lock Management**

**Check State Lock Status (Kubernetes Backend)**

```bash
# List all Terraform state locks (leases)
kubectl get leases -n <tenant-namespace> | grep terraform

# View lock details
kubectl describe lease <lease-name> -n <tenant-namespace>

# Check lock holder and acquisition time
kubectl get lease <lease-name> -n <tenant-namespace> -o jsonpath='{.spec.holderIdentity}'
```

**Manually Unlock State (Kubernetes Backend)**

**Warning**: Only unlock state if you are certain no deployment is running.

```bash
# Delete the lease to unlock state
kubectl delete lease <lease-name> -n <tenant-namespace>

# Verify lock is released
kubectl get leases -n <tenant-namespace> | grep terraform
```

**Check State Lock Status (S3 Backend)**

S3 backend does not have native locking. State locking is enforced by pipeline serialization.

```bash
# Check if multiple PipelineRuns are running
kubectl get pipelineruns -n <tenant-namespace> --field-selector=status.conditions[0].status=Unknown

# If multiple PipelineRuns are running, wait for them to complete
```

**Troubleshooting State Issues**

**Issue: State Lock Timeout**

Symptoms: Pipeline fails with "Error acquiring state lock" after timeout.

Diagnosis:

```bash
# Check for active locks
kubectl get leases -n <tenant-namespace> | grep terraform

# Check if another PipelineRun is running
kubectl get pipelineruns -n <tenant-namespace> --field-selector=status.conditions[0].status=Unknown

# Check lock holder
kubectl get lease <lease-name> -n <tenant-namespace> -o jsonpath='{.spec.holderIdentity}'
```

Resolution:

1. **Wait for concurrent deployment**: If another deployment is running, wait for it to complete
2. **Check for crashed deployment**: If lock holder is from a crashed pod, manually unlock
3. **Verify serialization**: Ensure pipeline serialization is configured correctly

**Issue: State Corruption**

Symptoms: Terraform reports state corruption or inconsistency errors.

Diagnosis:

```bash
# Pull current state
terraform state pull > current-state.json

# Validate state JSON
cat current-state.json | jq . > /dev/null

# Check for error messages
```

Resolution:

1. **Restore from backup**: Restore state from most recent backup
2. **Manually fix state**: Edit state JSON to fix corruption (advanced)
3. **Rebuild state**: Use `terraform import` to rebuild state from actual resources

**Issue: State Drift**

Symptoms: Terraform detects changes that were made outside of Terraform.

Diagnosis:

```bash
# Check for drift
terraform plan

# Identify drifted resources
terraform show
```

Resolution:

1. **Accept drift**: Run `terraform apply` to update state to match reality
2. **Revert manual changes**: Manually revert changes to match Terraform state
3. **Import changes**: Use `terraform import` to bring manual changes into state

**Issue: Missing State**

Symptoms: Terraform reports "No state file found" or empty state.

Diagnosis:

```bash
# Check if state secret exists
kubectl get secret tfstate-default-<tenant-name> -n <tenant-namespace>

# For S3 backend, check if state file exists
mc ls minio/terraform-state-<tenant-name>/
```

Resolution:

1. **Restore from backup**: Restore state from most recent backup
2. **Initialize new state**: If no backup exists, run `terraform init` and `terraform import` to rebuild state
3. **Check backend configuration**: Verify backend configuration is correct

**State Inspection and Debugging**

**Inspect State Contents**:

```bash
# Pull state to local file
terraform state pull > state.json

# View state in readable format
cat state.json | jq .

# List all resources in state
terraform state list

# Show specific resource
terraform state show <resource-address>
```

**Verify State Integrity**:

```bash
# Validate state JSON structure
terraform state pull | jq . > /dev/null && echo "State is valid JSON"

# Check state version
terraform state pull | jq .version

# Check Terraform version used
terraform state pull | jq .terraform_version
```

**Compare State with Reality**:

```bash
# Check for drift
terraform plan -detailed-exitcode

# Exit codes:
# 0 = no changes
# 1 = error
# 2 = changes detected (drift)

# Refresh state from actual infrastructure
terraform refresh
```

**State Maintenance Best Practices**

1. **Regular Backups**: Automate daily backups of all tenant states
2. **Version Control**: Never commit state files to git (use remote backend)
3. **State Locking**: Always use backends with native locking (Kubernetes backend)
4. **Serialization**: Ensure only one deployment runs at a time per tenant
5. **Monitoring**: Monitor state lock duration and alert on long-running locks
6. **Documentation**: Document backend configuration and backup procedures
7. **Testing**: Test backup and restore procedures regularly
8. **Isolation**: Keep state isolated per tenant (never share state across tenants)

**Emergency Recovery Procedures**

**Scenario: Complete State Loss**

If state is completely lost and no backup exists:

1. **Assess Impact**: Determine what infrastructure was managed by the lost state
2. **Document Resources**: List all resources that need to be imported
3. **Initialize New State**: Run `terraform init` to create new empty state
4. **Import Resources**: Use `terraform import` to import each resource:
   ```bash
   terraform import <resource-type>.<resource-name> <resource-id>
   ```
5. **Verify State**: Run `terraform plan` to verify state matches reality
6. **Create Backup**: Immediately backup the recovered state

**Scenario: State Locked by Crashed Pod**

If state is locked by a pod that crashed:

1. **Verify Pod is Crashed**: Check pod status and logs
2. **Identify Lock Holder**: Get lock holder identity from lease
3. **Manually Unlock**: Delete the lease to release lock
4. **Retry Deployment**: Trigger new deployment to verify lock is released

**Source**
- `platform/tenancy/templates/terraform-backend-secret.yaml`
- `platform/catalog/tasks/cdktf-deploy.yaml`
- `.kiro/specs/jenkinsx-platform/requirements.md` (Requirement 7.1, 7.2, 7.3, 7.5)
- `.kiro/specs/jenkinsx-platform/design.md` (Terraform State Backend section)
