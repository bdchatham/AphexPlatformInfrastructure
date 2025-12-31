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

**Source**
- `.kiro/specs/jenkinsx-platform/design.md`
- `.kiro/specs/jenkinsx-platform/requirements.md`
- `platform/bootstrap/README.md`
- `README.md`
