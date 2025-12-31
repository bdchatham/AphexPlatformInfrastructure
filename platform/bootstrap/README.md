# Bootstrap

This directory contains scripts and manifests for bootstrapping the Jenkins X platform on a fresh Kubernetes cluster.

## Contents

- `prerequisites.sh` - Verifies cluster prerequisites (kubectl, helm, cluster access)
- `bootstrap.sh` - Main bootstrap script (runs prerequisites check and installs Tekton Pipelines)
- `install-jenkinsx.sh` - Installs Jenkins X and Lighthouse (task 6)
- `create-github-app-secret.sh` - Helper script to create GitHub App secret
- `github-app-setup.md` - Comprehensive GitHub App setup guide
- Platform namespace manifests:
  - `namespace-auth-system.yaml` - Auth system namespace for Dex
  - `namespace-pipeline-catalog.yaml` - Pipeline catalog namespace
  - `namespace-pipeline-system.yaml` - Pipeline system namespace
- Dex OIDC provider manifests:
  - `dex-config.yaml` - Dex ConfigMap with static users and GitHub connector
  - `dex-deployment.yaml` - Dex Deployment, Service, and RBAC
  - `dex-README.md` - Comprehensive Dex deployment guide
  - `k8s-apiserver-oidc-config.md` - API server OIDC configuration instructions
- RBAC manifests:
  - `engineering-rbac.yaml` - RBAC for engineering group (repo onboarding permissions)

## Bootstrap Order

1. **Prerequisites Check**: Run `prerequisites.sh` to verify cluster readiness
2. **Create Namespaces**: Apply namespace manifests for auth-system, pipeline-system, and pipeline-catalog
3. **Deploy Dex**: Deploy Dex OIDC provider for authentication (see `dex-README.md`)
4. **Configure API Server**: Configure Kubernetes API server for OIDC (see `k8s-apiserver-oidc-config.md`)
5. **Deploy RBAC**: Apply engineering group RBAC for repository onboarding
6. **Install Tekton**: Install Tekton Pipelines (task 5) - run `bootstrap.sh`
7. **Create GitHub App**: Create GitHub App for webhook integration (see `github-app-setup.md`)
8. **Install Jenkins X and Lighthouse**: Install Jenkins X components (task 6) - run `install-jenkinsx.sh`
9. **Deploy Onboarding Controller**: Deploy the custom onboarding controller (task 9-11)
10. **Install Pipeline Catalog**: Deploy shared Tasks and Pipelines (task 12)

## Quick Start

### Automated Installation (Recommended)

```bash
# Step 1: Run the bootstrap script - it will check prerequisites and install Tekton
./bootstrap.sh

# Step 2: Create GitHub App and secret
# Follow the guide in github-app-setup.md to create the GitHub App
# Then run the helper script to create the Kubernetes secret:
./create-github-app-secret.sh

# Step 3: Install Jenkins X and Lighthouse
./install-jenkinsx.sh
```

The scripts will:
1. **bootstrap.sh**:
   - Verify all prerequisites (kubectl, helm, cluster access, RBAC, NetworkPolicy)
   - Prompt for confirmation
   - Install Tekton Pipelines v0.53.0
   - Wait for Tekton controllers to be ready
   - Display installation status

2. **create-github-app-secret.sh**:
   - Prompt for GitHub App credentials
   - Create Kubernetes secret with GitHub App private key and configuration
   - Verify secret creation

3. **install-jenkinsx.sh**:
   - Add Jenkins X Helm repository
   - Install jx-build-controller
   - Create Lighthouse configuration ConfigMaps
   - Install Lighthouse with GitHub App integration
   - Wait for all components to be ready
   - Display webhook endpoint URL for GitHub App configuration

### Manual Installation

If you prefer to install components manually:

```bash
# 1. Verify prerequisites
./prerequisites.sh

# 2. Create namespaces
kubectl apply -f namespace-auth-system.yaml
kubectl apply -f namespace-pipeline-system.yaml
kubectl apply -f namespace-pipeline-catalog.yaml

# 3. Deploy Dex
kubectl apply -f dex-config.yaml
kubectl apply -f dex-deployment.yaml

# 4. Configure API server for OIDC
# Follow instructions in k8s-apiserver-oidc-config.md

# 5. Deploy engineering RBAC
kubectl apply -f engineering-rbac.yaml

# 6. Install Tekton Pipelines
kubectl apply -f https://storage.googleapis.com/tekton-releases/pipeline/previous/v0.53.0/release.yaml

# 7. Wait for Tekton to be ready
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=controller -n tekton-pipelines --timeout=300s
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=webhook -n tekton-pipelines --timeout=300s

# 8. Verify Tekton installation
kubectl get pods -n tekton-pipelines

# 9. Create GitHub App (follow github-app-setup.md)

# 10. Create GitHub App secret
./create-github-app-secret.sh

# 11. Add Jenkins X Helm repository
helm repo add jx3 https://jenkins-x-charts.github.io/repo
helm repo update

# 12. Install jx-build-controller
helm install jx-build-controller jx3/jx-build-controller -n pipeline-system

# 13. Create Lighthouse configuration
kubectl apply -f ../lighthouse/lighthouse-config.yaml
kubectl apply -f ../lighthouse/repo-allowlist.yaml

# 14. Install Lighthouse
GITHUB_APP_ID=$(kubectl get secret lighthouse-github-app -n pipeline-system -o jsonpath='{.data.app-id}' | base64 -d)
GITHUB_APP_INSTALLATION_ID=$(kubectl get secret lighthouse-github-app -n pipeline-system -o jsonpath='{.data.installation-id}' | base64 -d)

helm install lighthouse jx3/lighthouse -n pipeline-system \
  --set github.appId="${GITHUB_APP_ID}" \
  --set github.appInstallationId="${GITHUB_APP_INSTALLATION_ID}" \
  --set github.secretName="lighthouse-github-app" \
  --set configMaps.config="lighthouse-config" \
  --set configMaps.allowlist="repo-allowlist"

# 15. Wait for Lighthouse to be ready
kubectl wait --for=condition=available deployment/lighthouse -n pipeline-system --timeout=300s

# 16. Verify installation
kubectl get pods -n pipeline-system
```

## Verification

After running the bootstrap and installation scripts, verify the installation:

```bash
# Check Tekton namespace
kubectl get namespace tekton-pipelines

# Check Tekton pods
kubectl get pods -n tekton-pipelines

# Check Tekton version
kubectl get deployment -n tekton-pipelines tekton-pipelines-controller -o jsonpath='{.spec.template.spec.containers[0].image}'

# Check pipeline-system namespace
kubectl get namespace pipeline-system

# Check Jenkins X components
kubectl get deployment jx-build-controller -n pipeline-system
kubectl get pods -n pipeline-system -l app=jx-build-controller

# Check Lighthouse
kubectl get deployment lighthouse -n pipeline-system
kubectl get pods -n pipeline-system -l app=lighthouse
kubectl logs -n pipeline-system deployment/lighthouse

# Check Lighthouse configuration
kubectl get configmap lighthouse-config -n pipeline-system -o yaml
kubectl get configmap repo-allowlist -n pipeline-system -o yaml

# Check GitHub App secret
kubectl get secret lighthouse-github-app -n pipeline-system
kubectl describe secret lighthouse-github-app -n pipeline-system

# Check Lighthouse webhook endpoint
kubectl get service lighthouse -n pipeline-system

# Check Dex (if deployed)
kubectl get pods -n auth-system
kubectl logs -n auth-system deployment/dex
```

## Usage

Run the bootstrap process to set up the platform infrastructure before deploying components. See individual README files for detailed instructions on each component.
