#!/bin/bash

set -e

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# Get the repository root (two levels up from script directory)
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default configuration
DEFAULT_CLUSTER_NAME="arbiter-platform"
DEFAULT_CLUSTER_TYPE="kind"
DEFAULT_REPO_URL="https://github.com/bdchatham/ArbiterPipelineInfrastructure"
DEFAULT_GITHUB_ORG="bdchatham"

# Parse command-line arguments
CLUSTER_NAME="${DEFAULT_CLUSTER_NAME}"
CLUSTER_TYPE="${DEFAULT_CLUSTER_TYPE}"
REPO_URL="${DEFAULT_REPO_URL}"
SKIP_ARGOCD_INSTALL="false"
GITHUB_ORG="${DEFAULT_GITHUB_ORG}"
GHCR_PAT=""

print_usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --cluster-name NAME       Name of the cluster (default: ${DEFAULT_CLUSTER_NAME})"
    echo "  --cluster-type TYPE       Type of cluster: kind, k3s, existing (default: ${DEFAULT_CLUSTER_TYPE})"
    echo "  --repo-url URL            Platform repository URL (default: ${DEFAULT_REPO_URL})"
    echo "  --github-org ORG          GitHub organization/username (default: ${DEFAULT_GITHUB_ORG})"
    echo "  --ghcr-pat TOKEN          GitHub Personal Access Token for GHCR (required)"
    echo "  --skip-argocd-install     Skip ArgoCD installation (use existing)"
    echo "  -h, --help                Display this help message"
    echo ""
    echo "Required:"
    echo "  --ghcr-pat: A GitHub PAT with 'packages:write' permission"
    echo "              Create at: https://github.com/settings/tokens?type=beta"
    echo ""
}

while [[ $# -gt 0 ]]; do
    case $1 in
        --cluster-name)
            CLUSTER_NAME="$2"
            shift 2
            ;;
        --cluster-type)
            CLUSTER_TYPE="$2"
            shift 2
            ;;
        --repo-url)
            REPO_URL="$2"
            shift 2
            ;;
        --github-org)
            GITHUB_ORG="$2"
            shift 2
            ;;
        --ghcr-pat)
            GHCR_PAT="$2"
            shift 2
            ;;
        --skip-argocd-install)
            SKIP_ARGOCD_INSTALL="true"
            shift
            ;;
        -h|--help)
            print_usage
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            print_usage
            exit 1
            ;;
    esac
done

# Validate required parameters
if [ -z "${GHCR_PAT}" ]; then
    echo -e "${RED}Error: --ghcr-pat is required${NC}"
    echo ""
    print_usage
    exit 1
fi

echo ""
echo "=========================================="
echo "  ArgoCD + Tekton Platform Bootstrap"
echo "=========================================="
echo ""
echo "This script will:"
echo "  1. Create/verify Kubernetes cluster"
echo "  2. Install Tekton Pipelines and Triggers"
echo "  3. Install ArgoCD"
echo "  4. Create platform namespaces"
echo "  5. Setup GHCR credentials and push runner image"
echo "  6. Create platform ArgoCD Application"
echo ""
echo "Configuration:"
echo "  Cluster: ${CLUSTER_NAME} (${CLUSTER_TYPE})"
echo "  Platform Repo: ${REPO_URL}"
echo "  GitHub Org: ${GITHUB_ORG}"
echo "  GHCR PAT: ${GHCR_PAT:0:10}... (provided)"
echo ""

# Function to create Kind cluster
create_kind_cluster() {
    local cluster_name="$1"
    
    echo -e "${BLUE}▸${NC} Creating Kind cluster: ${cluster_name}"
    
    # Check if cluster already exists (shouldn't happen, but safety check)
    if kind get clusters 2>/dev/null | grep -q "^${cluster_name}$"; then
        echo -e "${RED}  ✗${NC} Cluster ${cluster_name} already exists"
        echo "  Please delete it first: kind delete cluster --name ${cluster_name}"
        exit 1
    fi
    
    # Create cluster
    echo -e "${BLUE}▸${NC} Creating Kind cluster with ingress support..."
    local kind_config="${SCRIPT_DIR}/kind-cluster-config.yaml"
    
    if ! kind create cluster --name "${cluster_name}" --config="${kind_config}"; then
        echo -e "${RED}  ✗${NC} Failed to create Kind cluster"
        exit 1
    fi
    
    echo -e "${GREEN}  ✓${NC} Kind cluster created successfully"
    
    # Set kubectl context
    kubectl config use-context "kind-${cluster_name}"
    echo -e "${GREEN}  ✓${NC} kubectl context set to kind-${cluster_name}"
}

# Function to verify cluster accessibility
verify_cluster_access() {
    echo -e "${BLUE}▸${NC} Verifying cluster accessibility..."
    
    if ! kubectl cluster-info &> /dev/null; then
        echo -e "${RED}  ✗${NC} Cannot access Kubernetes cluster"
        return 1
    fi
    
    echo -e "${GREEN}  ✓${NC} Cluster access confirmed"
    
    # Display cluster info
    CURRENT_CONTEXT=$(kubectl config current-context)
    echo "    Current context: ${CURRENT_CONTEXT}"
    
    # Get cluster version
    K8S_VERSION=$(kubectl version --short 2>/dev/null | grep "Server Version" | awk '{print $3}')
    if [ -n "$K8S_VERSION" ]; then
        echo "    Kubernetes version: ${K8S_VERSION}"
    fi
    
    return 0
}

echo ""
echo "=========================================="
echo "  Step 1: Cluster Setup"
echo "=========================================="
echo ""

# Check if cluster already exists
echo -e "${BLUE}▸${NC} Checking for existing Kubernetes cluster..."
if kubectl cluster-info &> /dev/null; then
    CURRENT_CONTEXT=$(kubectl config current-context)
    echo -e "${YELLOW}  ✓ Cluster already exists: ${CURRENT_CONTEXT}${NC}"
    echo ""
    echo "A Kubernetes cluster is already configured and accessible."
    echo "This bootstrap script is designed to create a fresh cluster."
    echo ""
    echo "If you want to bootstrap this existing cluster, please ensure:"
    echo "  1. The cluster is empty or you're okay with installing platform components"
    echo "  2. You have appropriate permissions"
    echo ""
    echo "To create a fresh Kind cluster, first delete the existing one:"
    echo "  kind delete cluster --name ${CLUSTER_NAME}"
    echo ""
    exit 0
fi

echo -e "${BLUE}▸${NC} No existing cluster found. Creating new cluster..."
echo ""

if [ "${CLUSTER_TYPE}" = "kind" ]; then
    # Check if kind is available
    if ! command -v kind &> /dev/null; then
        echo -e "${RED}  ✗${NC} Kind is not installed"
        echo ""
        echo "Please install Kind: https://kind.sigs.k8s.io/docs/user/quick-start/#installation"
        echo ""
        echo "On macOS: brew install kind"
        echo "On Linux: curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.20.0/kind-linux-amd64 && chmod +x ./kind && sudo mv ./kind /usr/local/bin/kind"
        exit 1
    fi
    
    echo -e "${GREEN}  ✓${NC} Kind is available"
    create_kind_cluster "${CLUSTER_NAME}"
    
elif [ "${CLUSTER_TYPE}" = "k3s" ]; then
    echo -e "${RED}  ✗${NC} k3s cluster type specified but no cluster access"
    echo "Please install k3s: curl -sfL https://get.k3s.io | sh -"
    exit 1
    
elif [ "${CLUSTER_TYPE}" = "existing" ]; then
    echo -e "${RED}  ✗${NC} Existing cluster type specified but no cluster access"
    echo "Please configure kubectl to access your cluster"
    exit 1
    
else
    echo -e "${RED}  ✗${NC} Unknown cluster type: ${CLUSTER_TYPE}"
    exit 1
fi

# Verify cluster access
verify_cluster_access

echo ""
echo -e "${GREEN}  ✓ Cluster setup complete!${NC}"
echo ""

echo ""
echo "=========================================="
echo "  Step 2: Tekton Installation"
echo "=========================================="
echo ""

# Tekton versions
TEKTON_PIPELINES_VERSION="v0.65.0"
TEKTON_TRIGGERS_VERSION="v0.29.0"

# Function to install Tekton Pipelines
install_tekton_pipelines() {
    echo -e "${BLUE}▸${NC} Installing Tekton Pipelines ${TEKTON_PIPELINES_VERSION}..."
    
    # Check if already installed
    if kubectl get namespace tekton-pipelines &>/dev/null; then
        echo -e "${GREEN}  ✓${NC} Tekton Pipelines already installed, skipping"
        return 0
    fi
    
    # Download and apply Tekton Pipelines manifest
    local manifest_url="https://github.com/tektoncd/pipeline/releases/download/${TEKTON_PIPELINES_VERSION}/release.yaml"
    echo "  Downloading manifest from: ${manifest_url}"
    
    # Download manifest and replace gcr.io with ghcr.io to avoid 403 errors
    local temp_manifest="/tmp/tekton-pipelines-${TEKTON_PIPELINES_VERSION}.yaml"
    if ! curl -sL "${manifest_url}" | sed -e 's,gcr.io/tekton-releases,ghcr.io/tektoncd,g' > "${temp_manifest}"; then
        echo -e "${RED}  ✗${NC} Failed to download Tekton Pipelines manifest"
        return 1
    fi
    
    echo "  Applying manifest (using ghcr.io registry)..."
    if ! kubectl apply -f "${temp_manifest}"; then
        echo -e "${RED}  ✗${NC} Failed to install Tekton Pipelines"
        rm -f "${temp_manifest}"
        return 1
    fi
    
    rm -f "${temp_manifest}"
    echo -e "${GREEN}  ✓${NC} Tekton Pipelines manifest applied"
    
    # Wait for Tekton Pipelines to be ready
    echo "  Waiting for Tekton Pipelines controller to be ready..."
    local max_wait=60
    local elapsed=0
    while [ $elapsed -lt $max_wait ]; do
        if kubectl get pods -n tekton-pipelines -l app.kubernetes.io/name=controller 2>/dev/null | grep -q "controller"; then
            break
        fi
        sleep 5
        elapsed=$((elapsed + 5))
    done
    
    if ! kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=controller -n tekton-pipelines --timeout=300s 2>/dev/null; then
        echo -e "${RED}  ✗${NC} Tekton Pipelines controller failed to become ready"
        echo "  Checking pod status:"
        kubectl get pods -n tekton-pipelines -l app.kubernetes.io/name=controller
        kubectl describe pods -n tekton-pipelines -l app.kubernetes.io/name=controller | tail -20
        return 1
    fi
    
    echo "  Waiting for Tekton Pipelines webhook to be ready..."
    if ! kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=webhook -n tekton-pipelines --timeout=300s 2>/dev/null; then
        echo -e "${RED}  ✗${NC} Tekton Pipelines webhook failed to become ready"
        echo "  Checking pod status:"
        kubectl get pods -n tekton-pipelines -l app.kubernetes.io/name=webhook
        kubectl describe pods -n tekton-pipelines -l app.kubernetes.io/name=webhook | tail -20
        return 1
    fi
    
    echo -e "${GREEN}  ✓${NC} Tekton Pipelines ${TEKTON_PIPELINES_VERSION} installed successfully"
    return 0
}

# Install Tekton Pipelines
install_tekton_pipelines

echo ""
echo -e "${GREEN}  ✓ Tekton Pipelines installation complete!${NC}"
echo ""

# Function to install Tekton Triggers
install_tekton_triggers() {
    echo -e "${BLUE}▸${NC} Installing Tekton Triggers ${TEKTON_TRIGGERS_VERSION}..."
    
    # Check if already installed
    if kubectl get deployment tekton-triggers-controller -n tekton-pipelines &>/dev/null; then
        echo -e "${GREEN}  ✓${NC} Tekton Triggers already installed, skipping"
        return 0
    fi
    
    # Download and apply Tekton Triggers manifest
    local manifest_url="https://github.com/tektoncd/triggers/releases/download/${TEKTON_TRIGGERS_VERSION}/release.yaml"
    echo "  Downloading manifest from: ${manifest_url}"
    
    # Download manifest and replace gcr.io with ghcr.io to avoid 403 errors
    local temp_manifest="/tmp/tekton-triggers-${TEKTON_TRIGGERS_VERSION}.yaml"
    if ! curl -sL "${manifest_url}" | sed -e 's,gcr.io/tekton-releases,ghcr.io/tektoncd,g' > "${temp_manifest}"; then
        echo -e "${RED}  ✗${NC} Failed to download Tekton Triggers manifest"
        return 1
    fi
    
    echo "  Applying manifest (using ghcr.io registry)..."
    if ! kubectl apply -f "${temp_manifest}"; then
        echo -e "${RED}  ✗${NC} Failed to install Tekton Triggers"
        rm -f "${temp_manifest}"
        return 1
    fi
    
    rm -f "${temp_manifest}"
    echo -e "${GREEN}  ✓${NC} Tekton Triggers manifest applied"
    
    # Wait for Tekton Triggers to be ready
    echo "  Waiting for Tekton Triggers controller to be ready..."
    local max_wait=60
    local elapsed=0
    while [ $elapsed -lt $max_wait ]; do
        if kubectl get pods -n tekton-pipelines -l app.kubernetes.io/name=controller,app.kubernetes.io/part-of=tekton-triggers 2>/dev/null | grep -q "controller"; then
            break
        fi
        sleep 5
        elapsed=$((elapsed + 5))
    done
    
    if ! kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=controller,app.kubernetes.io/part-of=tekton-triggers -n tekton-pipelines --timeout=300s 2>/dev/null; then
        echo -e "${RED}  ✗${NC} Tekton Triggers controller failed to become ready"
        echo "  Checking pod status:"
        kubectl get pods -n tekton-pipelines -l app.kubernetes.io/name=controller,app.kubernetes.io/part-of=tekton-triggers
        kubectl describe pods -n tekton-pipelines -l app.kubernetes.io/name=controller,app.kubernetes.io/part-of=tekton-triggers | tail -20
        return 1
    fi
    
    echo "  Waiting for Tekton Triggers webhook to be ready..."
    if ! kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=webhook,app.kubernetes.io/part-of=tekton-triggers -n tekton-pipelines --timeout=300s 2>/dev/null; then
        echo -e "${RED}  ✗${NC} Tekton Triggers webhook failed to become ready"
        echo "  Checking pod status:"
        kubectl get pods -n tekton-pipelines -l app.kubernetes.io/name=webhook,app.kubernetes.io/part-of=tekton-triggers
        kubectl describe pods -n tekton-pipelines -l app.kubernetes.io/name=webhook,app.kubernetes.io/part-of=tekton-triggers | tail -20
        return 1
    fi
    
    echo -e "${GREEN}  ✓${NC} Tekton Triggers ${TEKTON_TRIGGERS_VERSION} installed successfully"
    return 0
}

# Install Tekton Triggers
if ! install_tekton_triggers; then
    echo ""
    echo -e "${YELLOW}  ⚠ Tekton Triggers installation failed${NC}"
    echo "  This is likely due to GCR image pull issues (403 Forbidden)"
    echo "  The platform can still function without Triggers for basic pipeline execution"
    echo "  You can manually install Triggers later or use alternative trigger mechanisms"
    echo ""
else
    echo ""
    echo -e "${GREEN}  ✓ Tekton Triggers installation complete!${NC}"
    echo ""
fi

echo "=========================================="
echo "  Step 3: ArgoCD Installation"
echo "=========================================="
echo ""

# ArgoCD version
ARGOCD_VERSION="stable"

# Function to install ArgoCD
install_argocd() {
    if [ "${SKIP_ARGOCD_INSTALL}" = "true" ]; then
        echo -e "${YELLOW}  Skipping ArgoCD installation (--skip-argocd-install flag)${NC}"
        return 0
    fi
    
    echo -e "${BLUE}▸${NC} Installing ArgoCD ${ARGOCD_VERSION}..."
    
    # Create argocd namespace if it doesn't exist
    if ! kubectl get namespace argocd &>/dev/null; then
        echo "  Creating argocd namespace..."
        kubectl create namespace argocd
    else
        echo -e "${GREEN}  ✓${NC} ArgoCD already installed, skipping"
        return 0
    fi
    
    # Download and apply ArgoCD manifest
    local manifest_url="https://raw.githubusercontent.com/argoproj/argo-cd/${ARGOCD_VERSION}/manifests/install.yaml"
    echo "  Downloading manifest from: ${manifest_url}"
    
    if ! kubectl apply -n argocd -f "${manifest_url}"; then
        echo -e "${RED}  ✗${NC} Failed to install ArgoCD"
        return 1
    fi
    
    echo -e "${GREEN}  ✓${NC} ArgoCD manifest applied"
    
    # Patch argocd-cmd-params-cm for insecure mode (homelab)
    echo "  Configuring ArgoCD for insecure mode (homelab)..."
    kubectl patch configmap argocd-cmd-params-cm -n argocd \
        --type merge \
        -p '{"data":{"server.insecure":"true"}}' 2>/dev/null || \
    kubectl create configmap argocd-cmd-params-cm -n argocd \
        --from-literal=server.insecure=true \
        --dry-run=client -o yaml | kubectl apply -f -
    
    # Restart argocd-server to pick up the config change
    echo "  Restarting ArgoCD server to apply configuration..."
    kubectl rollout restart deployment argocd-server -n argocd
    
    # Wait for ArgoCD components to be ready
    echo "  Waiting for ArgoCD server to be ready..."
    if ! kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=argocd-server -n argocd --timeout=300s; then
        echo -e "${RED}  ✗${NC} ArgoCD server failed to become ready"
        return 1
    fi
    
    echo "  Waiting for ArgoCD application controller to be ready..."
    if ! kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=argocd-application-controller -n argocd --timeout=300s; then
        echo -e "${RED}  ✗${NC} ArgoCD application controller failed to become ready"
        return 1
    fi
    
    echo "  Waiting for ArgoCD repo server to be ready..."
    if ! kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=argocd-repo-server -n argocd --timeout=300s; then
        echo -e "${RED}  ✗${NC} ArgoCD repo server failed to become ready"
        return 1
    fi
    
    echo -e "${GREEN}  ✓${NC} ArgoCD ${ARGOCD_VERSION} installed successfully"
    
    return 0
}

# Install ArgoCD
install_argocd

echo ""
echo -e "${GREEN}  ✓ ArgoCD installation complete!${NC}"
echo ""

echo "=========================================="
echo "  Step 4: Platform Namespace Creation"
echo "=========================================="
echo ""

# Function to create platform namespaces
create_platform_namespaces() {
    echo -e "${BLUE}▸${NC} Creating platform namespaces..."
    
    # argocd namespace (should already exist from ArgoCD installation)
    if ! kubectl get namespace argocd &>/dev/null; then
        echo "  Creating argocd namespace..."
        kubectl create namespace argocd
        echo -e "${GREEN}  ✓${NC} argocd namespace created"
    else
        echo -e "${GREEN}  ✓${NC} argocd namespace already exists"
    fi
    
    # tekton-pipelines namespace (should already exist from Tekton installation)
    if ! kubectl get namespace tekton-pipelines &>/dev/null; then
        echo "  Creating tekton-pipelines namespace..."
        kubectl create namespace tekton-pipelines
        echo -e "${GREEN}  ✓${NC} tekton-pipelines namespace created"
    else
        echo -e "${GREEN}  ✓${NC} tekton-pipelines namespace already exists"
    fi
    
    # platform-system namespace (new namespace for platform components)
    if ! kubectl get namespace platform-system &>/dev/null; then
        echo "  Creating platform-system namespace..."
        kubectl create namespace platform-system
        echo -e "${GREEN}  ✓${NC} platform-system namespace created"
    else
        echo -e "${GREEN}  ✓${NC} platform-system namespace already exists"
    fi
    
    echo -e "${GREEN}  ✓${NC} All platform namespaces verified"
    return 0
}

# Create platform namespaces
create_platform_namespaces

echo ""
echo -e "${GREEN}  ✓ Platform namespace creation complete!${NC}"
echo ""

echo "=========================================="
echo "  Step 5: GHCR Setup and Image Push"
echo "=========================================="
echo ""

# Function to create GHCR secret
create_ghcr_secret() {
    echo -e "${BLUE}▸${NC} Creating GHCR push secret..."
    
    # Check if secret already exists
    if kubectl get secret ghcr-push-secret -n platform-system &>/dev/null; then
        echo -e "${GREEN}  ✓${NC} GHCR push secret already exists"
        return 0
    fi
    
    # Create docker-registry secret
    if kubectl create secret docker-registry ghcr-push-secret \
        --docker-server=ghcr.io \
        --docker-username="${GITHUB_ORG}" \
        --docker-password="${GHCR_PAT}" \
        --namespace=platform-system; then
        echo -e "${GREEN}  ✓${NC} GHCR push secret created"
    else
        echo -e "${RED}  ✗${NC} Failed to create GHCR push secret"
        return 1
    fi
    
    return 0
}

# Function to build and push pipeline-runner image
push_pipeline_runner_image() {
    echo -e "${BLUE}▸${NC} Building and pushing pipeline-runner image..."
    
    local runner_image="ghcr.io/${GITHUB_ORG}/pipeline-runner:latest"
    local dockerfile_path="${REPO_ROOT}/platform/catalog/images/runner"
    
    # Check if Docker is available
    if ! command -v docker &> /dev/null; then
        echo -e "${YELLOW}  ⚠${NC} Docker not found, skipping image push"
        echo "  You'll need to manually build and push the image later"
        return 0
    fi
    
    # Check if logged into Docker
    if ! docker info &> /dev/null; then
        echo -e "${YELLOW}  ⚠${NC} Docker daemon not running, skipping image push"
        echo "  You'll need to manually build and push the image later"
        return 0
    fi
    
    echo "  Building image: ${runner_image}"
    if docker build -t "${runner_image}" "${dockerfile_path}" > /dev/null 2>&1; then
        echo -e "${GREEN}  ✓${NC} Image built successfully"
    else
        echo -e "${YELLOW}  ⚠${NC} Failed to build image, skipping push"
        echo "  You can manually build later with:"
        echo "    docker build -t ${runner_image} ${dockerfile_path}"
        return 0
    fi
    
    echo "  Pushing image to GHCR..."
    if docker push "${runner_image}" > /dev/null 2>&1; then
        echo -e "${GREEN}  ✓${NC} Image pushed successfully"
        
        # Also tag and push with commit SHA if in git repo
        if git rev-parse --git-dir > /dev/null 2>&1; then
            local commit_sha=$(git rev-parse --short HEAD)
            local commit_image="ghcr.io/${GITHUB_ORG}/pipeline-runner:${commit_sha}"
            docker tag "${runner_image}" "${commit_image}"
            if docker push "${commit_image}" > /dev/null 2>&1; then
                echo -e "${GREEN}  ✓${NC} Commit-tagged image pushed: ${commit_sha}"
            fi
        fi
    else
        echo -e "${YELLOW}  ⚠${NC} Failed to push image"
        echo "  You can manually push later with:"
        echo "    docker push ${runner_image}"
        return 0
    fi
    
    return 0
}

# Create GHCR secret
create_ghcr_secret

# Build and push pipeline-runner image
push_pipeline_runner_image

echo ""
echo -e "${GREEN}  ✓ GHCR setup complete!${NC}"
echo ""

echo "=========================================="
echo "  Step 6: Platform ArgoCD Application"
echo "=========================================="
echo ""

# Function to create platform root ArgoCD Application (App of Apps)
create_platform_application() {
    echo -e "${BLUE}▸${NC} Creating platform root ArgoCD Application (App of Apps)..."
    
    # Path to platform-root.yaml
    local app_manifest="${REPO_ROOT}/platform/argocd/apps/platform-root.yaml"
    
    if [ ! -f "${app_manifest}" ]; then
        echo -e "${RED}  ✗${NC} Platform root Application manifest not found: ${app_manifest}"
        echo "  Expected location: ${app_manifest}"
        return 1
    fi
    
    # Check if root Application already exists
    if kubectl get application platform-root -n argocd &>/dev/null; then
        echo -e "${GREEN}  ✓${NC} Platform root Application already exists, skipping"
        return 0
    fi
    
    # Apply platform root Application manifest
    echo "  Applying platform root Application manifest..."
    if ! kubectl apply -f "${app_manifest}"; then
        echo -e "${RED}  ✗${NC} Failed to create platform root Application"
        return 1
    fi
    
    echo -e "${GREEN}  ✓${NC} Platform root Application created"
    
    # Wait for root Application to sync
    echo "  Waiting for root Application to sync..."
    local max_wait=180
    local elapsed=0
    
    while [ $elapsed -lt $max_wait ]; do
        sync_status=$(kubectl get application platform-root -n argocd -o jsonpath='{.status.sync.status}' 2>/dev/null || echo "Unknown")
        health_status=$(kubectl get application platform-root -n argocd -o jsonpath='{.status.health.status}' 2>/dev/null || echo "Unknown")
        
        if [ "$sync_status" = "Synced" ]; then
            echo -e "${GREEN}  ✓${NC} Root Application synced successfully"
            echo "    Health status: ${health_status}"
            break
        fi
        
        echo "    Sync status: ${sync_status}, Health status: ${health_status} (${elapsed}s elapsed)"
        sleep 10
        elapsed=$((elapsed + 10))
    done
    
    if [ "$sync_status" != "Synced" ]; then
        echo -e "${YELLOW}  Root Application sync is taking longer than expected${NC}"
    fi
    
    # Wait for child Applications to be created
    echo ""
    echo "  Waiting for child Applications to be created..."
    local child_apps=("platform-crds" "platform-controllers" "platform-catalog")
    local all_created=false
    elapsed=0
    max_wait=120
    
    while [ $elapsed -lt $max_wait ]; do
        all_created=true
        for app in "${child_apps[@]}"; do
            if ! kubectl get application "$app" -n argocd &>/dev/null; then
                all_created=false
                break
            fi
        done
        
        if [ "$all_created" = true ]; then
            echo -e "${GREEN}  ✓${NC} All child Applications created"
            break
        fi
        
        echo "    Waiting for child Applications... (${elapsed}s elapsed)"
        sleep 10
        elapsed=$((elapsed + 10))
    done
    
    if [ "$all_created" = false ]; then
        echo -e "${YELLOW}  Some child Applications not yet created${NC}"
        echo "  This is normal - they will be created as the root Application syncs"
    fi
    
    # Display child Application status
    echo ""
    echo "  Child Application Status:"
    for app in "${child_apps[@]}"; do
        if kubectl get application "$app" -n argocd &>/dev/null; then
            sync_status=$(kubectl get application "$app" -n argocd -o jsonpath='{.status.sync.status}' 2>/dev/null || echo "Unknown")
            health_status=$(kubectl get application "$app" -n argocd -o jsonpath='{.status.health.status}' 2>/dev/null || echo "Unknown")
            echo "    ${app}: Sync=${sync_status}, Health=${health_status}"
        else
            echo "    ${app}: Not yet created"
        fi
    done
    
    echo ""
    echo -e "${GREEN}  ✓${NC} Platform Applications configured"
    echo ""
    echo "  Monitor Application sync status:"
    echo "    kubectl get applications -n argocd"
    echo "    argocd app list"
    
    return 0
}

# Create platform Application
create_platform_application

echo ""
echo -e "${GREEN}  ✓ Platform Application creation complete!${NC}"
echo ""

echo "=========================================="
echo "  Bootstrap Complete!"
echo "=========================================="
echo ""
echo -e "${GREEN}✓ Platform bootstrap completed successfully!${NC}"
echo ""

# Display ArgoCD access information
echo "=========================================="
echo "  ArgoCD Access Information"
echo "=========================================="
echo ""

# Retrieve admin password
admin_password=$(kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" 2>/dev/null | base64 -d)

if [ -n "$admin_password" ]; then
    echo "  Username: admin"
    echo "  Password: ${admin_password}"
    echo ""
else
    echo "  Username: admin"
    echo "  Password: (retrieve with command below)"
    echo ""
fi

echo "  To access ArgoCD UI:"
echo "    kubectl port-forward svc/argocd-server -n argocd 8080:443"
echo "    Then open: http://localhost:8080"
echo ""

if [ -z "$admin_password" ]; then
    echo "  To retrieve admin password:"
    echo "    kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath=\"{.data.password}\" | base64 -d"
    echo ""
fi

echo "  Or install ArgoCD CLI and login:"
if [ -n "$admin_password" ]; then
    echo "    argocd login localhost:8080 --username admin --password '${admin_password}' --insecure"
else
    echo "    argocd login localhost:8080 --username admin --insecure"
fi
echo ""
echo "=========================================="
echo ""

echo "Next steps:"
echo ""
echo "1. Monitor platform Applications:"
echo "   kubectl get applications -n argocd"
echo "   argocd app list"
echo ""
echo "2. View platform components:"
echo "   kubectl get all -n platform-system"
echo ""
echo "3. Check child Application sync status:"
echo "   kubectl get application platform-crds -n argocd"
echo "   kubectl get application platform-controllers -n argocd"
echo "   kubectl get application platform-catalog -n argocd"
echo ""
echo "4. Register a repository by creating a RepoBinding:"
echo "   kubectl apply -f platform/crds/example-repobinding.yaml"
echo ""
echo "=========================================="
echo ""
