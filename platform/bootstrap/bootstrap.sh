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

# Configuration
CLUSTER_NAME="${CLUSTER_NAME:-arbiter-infrastructure}"
CLUSTER_TYPE="${CLUSTER_TYPE:-kind}"
REPO_URL="${REPO_URL:-https://github.com/bdchatham/ArbiterPipelineInfrastructure}"
REPO_ORG="${REPO_ORG:-bdchatham}"
REPO_NAME="${REPO_NAME:-ArbiterPipelineInfrastructure}"
MIN_KUBECTL_VERSION="1.24.0"
MIN_HELM_VERSION="3.0.0"
MIN_K8S_VERSION="1.24.0"
TEKTON_VERSION="v0.56.0"

echo ""
echo "=========================================="
echo "  Jenkins X Platform Bootstrap"
echo "=========================================="
echo ""
echo "This script will:"
echo "  1. Create/verify Kubernetes cluster"
echo "  2. Install Tekton Pipelines"
echo "  3. Install Lighthouse"
echo "  4. Create platform namespaces"
echo "  5. Deploy onboarding controller"
echo "  6. Deploy pipeline catalog"
echo "  7. Register platform repository"
echo ""
echo "Configuration:"
echo "  Cluster: ${CLUSTER_NAME} (${CLUSTER_TYPE})"
echo "  Platform Repo: ${REPO_URL}"
echo ""

echo ""
echo "=========================================="
echo "  Step 1: Cluster Setup"
echo "=========================================="
echo ""

# Check if cluster creation is needed
echo -e "${BLUE}▸${NC} Checking for existing Kubernetes cluster..."
if ! kubectl cluster-info &> /dev/null; then
    echo -e "${YELLOW}  No accessible cluster found${NC}"
    echo ""
    
    # Check if kind is available
    if command -v kind &> /dev/null; then
        echo -e "${GREEN}  ✓${NC} Kind is available for local cluster creation"
        read -p "Would you like to create a local Kind cluster named '${CLUSTER_NAME}'? (y/n) " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            echo ""
            echo -e "${BLUE}▸${NC} Creating Kind cluster: ${CLUSTER_NAME}"
            
            # Check if cluster already exists
            if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
                echo -e "${YELLOW}  Cluster ${CLUSTER_NAME} already exists${NC}"
                read -p "Delete and recreate? (y/n) " -n 1 -r
                echo
                if [[ $REPLY =~ ^[Yy]$ ]]; then
                    echo -e "${BLUE}▸${NC} Deleting existing cluster..."
                    kind delete cluster --name "${CLUSTER_NAME}"
                else
                    echo -e "${GREEN}  ✓${NC} Using existing cluster"
                fi
            fi
            
            # Create cluster if it doesn't exist
            if ! kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
                echo -e "${BLUE}▸${NC} Creating Kind cluster with ingress support..."
                cat <<EOF | kind create cluster --name "${CLUSTER_NAME}" --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
    kubeadmConfigPatches:
      - |
        kind: InitConfiguration
        nodeRegistration:
          kubeletExtraArgs:
            node-labels: "ingress-ready=true"
    extraPortMappings:
      - containerPort: 80
        hostPort: 80
        protocol: TCP
      - containerPort: 443
        hostPort: 443
        protocol: TCP
      - containerPort: 5556
        hostPort: 5556
        protocol: TCP
EOF
                
                if [ $? -eq 0 ]; then
                    echo -e "${GREEN}  ✓${NC} Kind cluster created successfully"
                    
                    # Set kubectl context
                    kubectl config use-context "kind-${CLUSTER_NAME}"
                    echo -e "${GREEN}  ✓${NC} kubectl context set to kind-${CLUSTER_NAME}"
                else
                    echo -e "${RED}  ✗${NC} Failed to create Kind cluster"
                    exit 1
                fi
            fi
        else
            echo "Cluster creation skipped. Please configure kubectl to access your cluster."
            exit 1
        fi
    else
        echo -e "${RED}  ✗${NC} No cluster access and Kind is not installed"
        echo ""
        echo "Options:"
        echo "  1. Install Kind: https://kind.sigs.k8s.io/docs/user/quick-start/#installation"
        echo "  2. Configure kubectl to access an existing cluster"
        echo "  3. Install k3s for homelab: curl -sfL https://get.k3s.io | sh -"
        exit 1
    fi
else
    echo -e "${GREEN}  ✓${NC} Cluster access confirmed"
    CURRENT_CONTEXT=$(kubectl config current-context)
    echo "    Current context: ${CURRENT_CONTEXT}"
    echo ""
    read -p "Continue with this cluster? (y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Bootstrap cancelled. Please switch to the desired cluster context."
        exit 0
    fi
fi

echo ""
echo "=========================================="
echo "  Step 2: Prerequisites Check"
echo "=========================================="
echo ""

# Function to compare versions
version_ge() {
    [ "$(printf '%s\n' "$1" "$2" | sort -V | head -n1)" = "$2" ]
}

# Function to extract version number
extract_version() {
    echo "$1" | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -n1
}

# Check if kubectl is installed
echo -n -e "${BLUE}▸${NC} Checking kubectl installation... "
if ! command -v kubectl &> /dev/null; then
    echo -e "${RED}FAILED${NC}"
    echo "kubectl is not installed. Please install kubectl version ${MIN_KUBECTL_VERSION} or higher."
    echo "Visit: https://kubernetes.io/docs/tasks/tools/"
    exit 1
fi
echo -e "${GREEN}✓${NC}"

# Check kubectl version
echo -n -e "${BLUE}▸${NC} Checking kubectl version... "
KUBECTL_VERSION=$(kubectl version --client 2>/dev/null | grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+' | head -n1 | sed 's/v//')
if [ -z "$KUBECTL_VERSION" ]; then
    KUBECTL_VERSION=$(kubectl version --client --short 2>/dev/null | extract_version)
fi

if [ -z "$KUBECTL_VERSION" ]; then
    echo -e "${RED}FAILED${NC}"
    echo "Could not determine kubectl version"
    exit 1
fi

if version_ge "$KUBECTL_VERSION" "$MIN_KUBECTL_VERSION"; then
    echo -e "${GREEN}✓${NC} (v${KUBECTL_VERSION})"
else
    echo -e "${RED}FAILED${NC}"
    echo "kubectl version ${KUBECTL_VERSION} is too old. Minimum required: ${MIN_KUBECTL_VERSION}"
    exit 1
fi

# Check if helm is installed
echo -n -e "${BLUE}▸${NC} Checking helm installation... "
if ! command -v helm &> /dev/null; then
    echo -e "${RED}FAILED${NC}"
    echo "helm is not installed. Please install helm version ${MIN_HELM_VERSION} or higher."
    echo "Visit: https://helm.sh/docs/intro/install/"
    exit 1
fi
echo -e "${GREEN}✓${NC}"

# Check helm version
echo -n -e "${BLUE}▸${NC} Checking helm version... "
HELM_VERSION=$(helm version 2>/dev/null | grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+' | head -n1 | sed 's/v//')
if [ -z "$HELM_VERSION" ]; then
    echo -e "${RED}FAILED${NC}"
    echo "Could not determine helm version"
    exit 1
fi

if version_ge "$HELM_VERSION" "$MIN_HELM_VERSION"; then
    echo -e "${GREEN}✓${NC} (v${HELM_VERSION})"
else
    echo -e "${RED}FAILED${NC}"
    echo "helm version ${HELM_VERSION} is too old. Minimum required: ${MIN_HELM_VERSION}"
    exit 1
fi

# Check cluster access
echo -n -e "${BLUE}▸${NC} Checking cluster access... "
if ! kubectl cluster-info &> /dev/null; then
    echo -e "${RED}FAILED${NC}"
    echo "Cannot access Kubernetes cluster. Please check your kubeconfig."
    exit 1
fi
echo -e "${GREEN}✓${NC}"

# Check Kubernetes version
echo -n -e "${BLUE}▸${NC} Checking Kubernetes version... "
K8S_VERSION=$(kubectl version 2>/dev/null | grep "Server Version" | grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+' | sed 's/v//')
if [ -z "$K8S_VERSION" ]; then
    K8S_VERSION=$(kubectl version --short 2>/dev/null | grep "Server Version" | extract_version)
fi

if [ -z "$K8S_VERSION" ]; then
    echo -e "${YELLOW}WARNING${NC}"
    echo "Could not determine Kubernetes server version"
else
    if version_ge "$K8S_VERSION" "$MIN_K8S_VERSION"; then
        echo -e "${GREEN}✓${NC} (v${K8S_VERSION})"
    else
        echo -e "${RED}FAILED${NC}"
        echo "Kubernetes version ${K8S_VERSION} is too old. Minimum required: ${MIN_K8S_VERSION}"
        exit 1
    fi
fi

# Check for RBAC support
echo -n -e "${BLUE}▸${NC} Checking RBAC support... "
if kubectl api-versions | grep -q "rbac.authorization.k8s.io"; then
    echo -e "${GREEN}✓${NC}"
else
    echo -e "${RED}FAILED${NC}"
    echo "RBAC is not enabled on this cluster. RBAC is required for Jenkins X platform."
    exit 1
fi

# Check for NetworkPolicy support
echo -n -e "${BLUE}▸${NC} Checking NetworkPolicy support... "
if kubectl api-versions | grep -q "networking.k8s.io"; then
    echo -e "${GREEN}✓${NC}"
else
    echo -e "${YELLOW}WARNING${NC}"
    echo "NetworkPolicy API is not available. Network isolation may not work."
    echo "Consider installing a CNI plugin that supports NetworkPolicy (e.g., Calico, Cilium)."
fi

# Check cluster permissions
echo -n -e "${BLUE}▸${NC} Checking cluster admin permissions... "
if kubectl auth can-i create namespaces --all-namespaces &> /dev/null; then
    echo -e "${GREEN}✓${NC}"
else
    echo -e "${RED}FAILED${NC}"
    echo "Current user does not have cluster admin permissions."
    echo "Bootstrap requires the ability to create namespaces and cluster-wide resources."
    exit 1
fi

echo ""
echo -e "${GREEN}  ✓ All prerequisites checks passed!${NC}"

echo ""
echo "=========================================="
echo "  Step 3: Install Tekton Pipelines"
echo "=========================================="
echo ""

TEKTON_RELEASE_URL="https://github.com/tektoncd/pipeline/releases/download/${TEKTON_VERSION}/release.yaml"

echo -e "${BLUE}▸${NC} Installing Tekton Pipelines ${TEKTON_VERSION}..."
if curl -sL "${TEKTON_RELEASE_URL}" | \
   sed 's|gcr.io/tekton-releases|ghcr.io/tektoncd|g' | \
   kubectl apply -f - > /dev/null; then
    echo -e "${GREEN}  ✓${NC} Tekton Pipelines installed successfully"
else
    echo -e "${RED}  ✗${NC} Failed to install Tekton Pipelines"
    exit 1
fi

echo -e "${BLUE}▸${NC} Waiting for Tekton Pipelines namespace..."
timeout=60
elapsed=0
while ! kubectl get namespace tekton-pipelines &> /dev/null; do
    if [ $elapsed -ge $timeout ]; then
        echo -e "${RED}  ✗${NC} Timeout waiting for tekton-pipelines namespace"
        exit 1
    fi
    sleep 2
    elapsed=$((elapsed + 2))
done
echo -e "${GREEN}  ✓${NC} tekton-pipelines namespace created"

# Give pods time to be created
sleep 5

echo -e "${BLUE}▸${NC} Waiting for tekton-pipelines-controller..."
if kubectl wait --for=condition=ready pod \
    -l app.kubernetes.io/name=controller \
    -n tekton-pipelines \
    --timeout=300s > /dev/null 2>&1; then
    echo -e "${GREEN}  ✓${NC} tekton-pipelines-controller is ready"
else
    echo -e "${RED}  ✗${NC} tekton-pipelines-controller failed to become ready"
    echo "Checking pod status:"
    kubectl get pods -n tekton-pipelines -l app.kubernetes.io/name=controller
    exit 1
fi

echo -e "${BLUE}▸${NC} Waiting for tekton-pipelines-webhook..."
if kubectl wait --for=condition=ready pod \
    -l app.kubernetes.io/name=webhook \
    -n tekton-pipelines \
    --timeout=300s > /dev/null 2>&1; then
    echo -e "${GREEN}  ✓${NC} tekton-pipelines-webhook is ready"
else
    echo -e "${RED}  ✗${NC} tekton-pipelines-webhook failed to become ready"
    echo "Checking pod status:"
    kubectl get pods -n tekton-pipelines -l app.kubernetes.io/name=webhook
    exit 1
fi


echo ""
echo "=========================================="
echo "  Step 4: Install Lighthouse"
echo "=========================================="
echo ""

# Ensure pipeline-system namespace exists
echo -e "${BLUE}▸${NC} Creating pipeline-system namespace..."
if ! kubectl get namespace pipeline-system &> /dev/null; then
    kubectl create namespace pipeline-system
    echo -e "${GREEN}  ✓${NC} pipeline-system namespace created"
else
    echo -e "${GREEN}  ✓${NC} pipeline-system namespace already exists"
fi

# Add Jenkins X Helm repository
echo -e "${BLUE}▸${NC} Adding Jenkins X Helm repository..."
if helm repo add jx3 https://jenkins-x-charts.github.io/repo 2>&1 | grep -q "already exists"; then
    echo -e "${GREEN}  ✓${NC} Jenkins X Helm repository already exists"
else
    echo -e "${GREEN}  ✓${NC} Jenkins X Helm repository added"
fi

echo -e "${BLUE}▸${NC} Updating Helm repository cache..."
if helm repo update > /dev/null 2>&1; then
    echo -e "${GREEN}  ✓${NC} Helm repository cache updated"
else
    echo -e "${RED}  ✗${NC} Failed to update Helm repository cache"
    exit 1
fi

# Create Lighthouse configuration ConfigMaps
echo -e "${BLUE}▸${NC} Creating Lighthouse configuration..."
if kubectl apply -f "${REPO_ROOT}/platform/lighthouse/lighthouse-config.yaml" > /dev/null 2>&1; then
    echo -e "${GREEN}  ✓${NC} Lighthouse configuration ConfigMap created"
else
    echo -e "${RED}  ✗${NC} Failed to create Lighthouse configuration ConfigMap"
    exit 1
fi

if kubectl apply -f "${REPO_ROOT}/platform/lighthouse/repo-allowlist.yaml" > /dev/null 2>&1; then
    echo -e "${GREEN}  ✓${NC} Repository allowlist ConfigMap created"
else
    echo -e "${RED}  ✗${NC} Failed to create repository allowlist ConfigMap"
    exit 1
fi

# Check if GitHub App secret exists, if not prompt for credentials
if ! kubectl get secret lighthouse-github-app -n pipeline-system &> /dev/null; then
    echo ""
    echo -e "${YELLOW}GitHub App credentials required${NC}"
    echo "Please provide your GitHub App configuration:"
    echo ""
    
    # Check for GitHub App ID (environment variable or prompt)
    if [ -z "$GITHUB_APP_ID" ]; then
        read -p "GitHub App ID: " GITHUB_APP_ID
    else
        echo "Using GITHUB_APP_ID from environment: ${GITHUB_APP_ID}"
    fi
    if [ -z "$GITHUB_APP_ID" ]; then
        echo -e "${RED}  ✗${NC} GitHub App ID is required"
        exit 1
    fi
    
    # Check for GitHub App Installation ID (environment variable or prompt)
    if [ -z "$GITHUB_APP_INSTALLATION_ID" ]; then
        read -p "GitHub App Installation ID: " GITHUB_APP_INSTALLATION_ID
    else
        echo "Using GITHUB_APP_INSTALLATION_ID from environment: ${GITHUB_APP_INSTALLATION_ID}"
    fi
    if [ -z "$GITHUB_APP_INSTALLATION_ID" ]; then
        echo -e "${RED}  ✗${NC} GitHub App Installation ID is required"
        exit 1
    fi
    
    # Check for private key file path (environment variable or prompt)
    if [ -z "$GITHUB_APP_PRIVATE_KEY_FILE" ]; then
        read -p "Path to GitHub App Private Key (.pem file): " GITHUB_APP_PRIVATE_KEY_FILE
    else
        echo "Using GITHUB_APP_PRIVATE_KEY_FILE from environment: ${GITHUB_APP_PRIVATE_KEY_FILE}"
    fi
    if [ -z "$GITHUB_APP_PRIVATE_KEY_FILE" ]; then
        echo -e "${RED}  ✗${NC} Private Key file path is required"
        exit 1
    fi
    
    if [ ! -f "$GITHUB_APP_PRIVATE_KEY_FILE" ]; then
        echo -e "${RED}  ✗${NC} Private Key file not found: ${GITHUB_APP_PRIVATE_KEY_FILE}"
        exit 1
    fi
    
    # Generate webhook secret
    echo -e "${BLUE}▸${NC} Generating webhook secret..."
    GITHUB_WEBHOOK_SECRET=$(openssl rand -base64 32 | tr -d "=+/" | cut -c1-32)
    echo -e "${GREEN}  ✓${NC} Webhook secret generated"
    
    # Create the secret
    echo -e "${BLUE}▸${NC} Creating GitHub App secret..."
    if kubectl create secret generic lighthouse-github-app \
        --from-file=private-key="${GITHUB_APP_PRIVATE_KEY_FILE}" \
        --from-literal=app-id="${GITHUB_APP_ID}" \
        --from-literal=installation-id="${GITHUB_APP_INSTALLATION_ID}" \
        --from-literal=webhook-secret="${GITHUB_WEBHOOK_SECRET}" \
        -n pipeline-system > /dev/null 2>&1; then
        echo -e "${GREEN}  ✓${NC} GitHub App secret created"
    else
        echo -e "${RED}  ✗${NC} Failed to create GitHub App secret"
        exit 1
    fi
else
    # Secret exists, read credentials from it
    echo -e "${BLUE}▸${NC} Reading GitHub App credentials from existing secret..."
    GITHUB_APP_ID=$(kubectl get secret lighthouse-github-app -n pipeline-system -o jsonpath='{.data.app-id}' | base64 -d)
    GITHUB_APP_INSTALLATION_ID=$(kubectl get secret lighthouse-github-app -n pipeline-system -o jsonpath='{.data.installation-id}' | base64 -d)
    GITHUB_WEBHOOK_SECRET=$(kubectl get secret lighthouse-github-app -n pipeline-system -o jsonpath='{.data.webhook-secret}' | base64 -d)
    
    if [ -z "$GITHUB_APP_ID" ] || [ -z "$GITHUB_APP_INSTALLATION_ID" ] || [ -z "$GITHUB_WEBHOOK_SECRET" ]; then
        echo -e "${RED}  ✗${NC} Failed to read GitHub App credentials from secret"
        exit 1
    fi
    
    echo -e "${GREEN}  ✓${NC} GitHub App credentials loaded"
    echo "    App ID: ${GITHUB_APP_ID}"
    echo "    Installation ID: ${GITHUB_APP_INSTALLATION_ID}"
fi

# Install Lighthouse
echo -e "${BLUE}▸${NC} Installing Lighthouse..."
if helm list -n pipeline-system | grep -q "lighthouse"; then
    if helm upgrade lighthouse jx3/lighthouse -n pipeline-system \
        --set git.kind=github \
        --set githubApp.enabled=true \
        --set githubApp.username="jenkins-x[bot]" \
        --set configMaps.config="lighthouse-config" \
        --set configMaps.allowlist="repo-allowlist" \
        --set keeper.replicaCount=0 \
        --set env.LIGHTHOUSE_IN_REPO_CONFIG_ENABLED=true > /dev/null 2>&1; then
        echo -e "${GREEN}  ✓${NC} Lighthouse upgraded successfully"
    else
        echo -e "${RED}  ✗${NC} Failed to upgrade Lighthouse"
        exit 1
    fi
else
    if helm install lighthouse jx3/lighthouse -n pipeline-system \
        --set git.kind=github \
        --set githubApp.enabled=true \
        --set githubApp.username="jenkins-x[bot]" \
        --set configMaps.config="lighthouse-config" \
        --set configMaps.allowlist="repo-allowlist" \
        --set keeper.replicaCount=0 \
        --set env.LIGHTHOUSE_IN_REPO_CONFIG_ENABLED=true > /dev/null 2>&1; then
        echo -e "${GREEN}  ✓${NC} Lighthouse installed successfully"
    else
        echo -e "${RED}  ✗${NC} Failed to install Lighthouse"
        exit 1
    fi
fi

echo -e "${BLUE}▸${NC} Waiting for Lighthouse webhooks deployment..."
timeout=120
elapsed=0
while ! kubectl get deployment lighthouse-webhooks -n pipeline-system &> /dev/null; do
    if [ $elapsed -ge $timeout ]; then
        echo -e "${RED}  ✗${NC} Timeout waiting for Lighthouse webhooks deployment"
        exit 1
    fi
    sleep 2
    elapsed=$((elapsed + 2))
done

if kubectl wait --for=condition=available deployment/lighthouse-webhooks \
    -n pipeline-system \
    --timeout=300s > /dev/null 2>&1; then
    echo -e "${GREEN}  ✓${NC} Lighthouse webhooks is ready"
else
    echo -e "${RED}  ✗${NC} Lighthouse webhooks failed to become ready"
    kubectl get deployment lighthouse-webhooks -n pipeline-system
    kubectl get pods -n pipeline-system -l app=lighthouse-webhooks
    exit 1
fi

echo -e "${BLUE}▸${NC} Waiting for Lighthouse foghorn deployment..."
if kubectl wait --for=condition=available deployment/lighthouse-foghorn \
    -n pipeline-system \
    --timeout=300s > /dev/null 2>&1; then
    echo -e "${GREEN}  ✓${NC} Lighthouse foghorn is ready"
else
    echo -e "${RED}  ✗${NC} Lighthouse foghorn failed to become ready"
    kubectl get deployment lighthouse-foghorn -n pipeline-system
    kubectl get pods -n pipeline-system -l app=lighthouse-foghorn
    exit 1
fi

# Install nginx ingress controller for Kind
if [ "${CLUSTER_TYPE}" = "kind" ]; then
    echo ""
    echo -e "${BLUE}▸${NC} Installing nginx ingress controller..."
    if kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml > /dev/null 2>&1; then
        echo -e "${GREEN}  ✓${NC} Nginx ingress controller installed"
    else
        echo -e "${RED}  ✗${NC} Failed to install nginx ingress controller"
        exit 1
    fi
    
    echo -e "${BLUE}▸${NC} Waiting for ingress controller..."
    if kubectl wait --namespace ingress-nginx \
        --for=condition=ready pod \
        --selector=app.kubernetes.io/component=controller \
        --timeout=300s > /dev/null 2>&1; then
        echo -e "${GREEN}  ✓${NC} Ingress controller is ready"
    else
        echo -e "${RED}  ✗${NC} Ingress controller failed to become ready"
        exit 1
    fi
fi

# Create ingress for Lighthouse webhooks
echo -e "${BLUE}▸${NC} Creating Lighthouse webhook ingress..."
if kubectl apply -f "${REPO_ROOT}/platform/infrastructure/ingress/lighthouse-ingress.yaml" > /dev/null 2>&1; then
    echo -e "${GREEN}  ✓${NC} Lighthouse webhook ingress created"
else
    echo -e "${RED}  ✗${NC} Failed to create Lighthouse webhook ingress"
    exit 1
fi

echo ""
echo "=========================================="
echo "  Step 5: Deploy Onboarding Controller"
echo "=========================================="
echo ""

# Build and load onboarding controller image
echo -e "${BLUE}▸${NC} Building onboarding controller image..."
if (cd "${REPO_ROOT}/platform/onboarding/controller" && make docker-build IMG=ghcr.io/bdchatham/onboarding-controller:latest > /dev/null 2>&1); then
    echo -e "${GREEN}  ✓${NC} Onboarding controller image built"
else
    echo -e "${RED}  ✗${NC} Failed to build onboarding controller image"
    exit 1
fi

echo -e "${BLUE}▸${NC} Loading onboarding controller image into Kind cluster..."
if kind load docker-image ghcr.io/bdchatham/onboarding-controller:latest --name "${CLUSTER_NAME}" > /dev/null 2>&1; then
    echo -e "${GREEN}  ✓${NC} Onboarding controller image loaded"
else
    echo -e "${RED}  ✗${NC} Failed to load onboarding controller image"
    exit 1
fi

# Apply RepoBinding CRD
echo -e "${BLUE}▸${NC} Installing RepoBinding CRD..."
if kubectl apply -f "${REPO_ROOT}/platform/infrastructure/crds/repobinding-crd.yaml" > /dev/null 2>&1; then
    echo -e "${GREEN}  ✓${NC} RepoBinding CRD installed"
else
    echo -e "${RED}  ✗${NC} Failed to install RepoBinding CRD"
    exit 1
fi

# Deploy onboarding controller
echo -e "${BLUE}▸${NC} Deploying onboarding controller..."
if kubectl apply -f "${REPO_ROOT}/platform/onboarding/controller-service-account.yaml" > /dev/null 2>&1 && \
   kubectl apply -f "${REPO_ROOT}/platform/onboarding/controller-rbac.yaml" > /dev/null 2>&1 && \
   kubectl apply -f "${REPO_ROOT}/platform/onboarding/controller-deployment.yaml" > /dev/null 2>&1; then
    echo -e "${GREEN}  ✓${NC} Onboarding controller deployed"
else
    echo -e "${RED}  ✗${NC} Failed to deploy onboarding controller"
    exit 1
fi

# Wait for onboarding controller to be ready
echo -e "${BLUE}▸${NC} Waiting for onboarding controller..."
timeout=120
elapsed=0
while ! kubectl get deployment onboarding-controller -n pipeline-system &> /dev/null; do
    if [ $elapsed -ge $timeout ]; then
        echo -e "${RED}  ✗${NC} Timeout waiting for onboarding controller deployment"
        exit 1
    fi
    sleep 2
    elapsed=$((elapsed + 2))
done

if kubectl wait --for=condition=available deployment/onboarding-controller \
    -n pipeline-system \
    --timeout=300s > /dev/null 2>&1; then
    echo -e "${GREEN}  ✓${NC} Onboarding controller is ready"
else
    echo -e "${RED}  ✗${NC} Onboarding controller failed to become ready"
    kubectl get deployment onboarding-controller -n pipeline-system
    kubectl get pods -n pipeline-system -l app=onboarding-controller
    exit 1
fi

echo ""
echo "=========================================="
echo "  Step 6: Deploy Pipeline Catalog"
echo "=========================================="
echo ""

# Create pipeline-catalog namespace
echo -e "${BLUE}▸${NC} Creating pipeline-catalog namespace..."
kubectl create namespace pipeline-catalog --dry-run=client -o yaml | kubectl apply -f - > /dev/null 2>&1
echo -e "${GREEN}  ✓${NC} pipeline-catalog namespace created"

# Deploy catalog tasks
echo -e "${BLUE}▸${NC} Deploying pipeline catalog tasks..."
if kubectl apply -f "${REPO_ROOT}/platform/catalog/tasks/" -n pipeline-catalog > /dev/null 2>&1; then
    echo -e "${GREEN}  ✓${NC} Pipeline catalog tasks deployed"
else
    echo -e "${YELLOW}  ⚠${NC} Some catalog tasks may have failed to deploy"
fi

# Deploy catalog pipelines
echo -e "${BLUE}▸${NC} Deploying pipeline catalog pipelines..."
if kubectl apply -f "${REPO_ROOT}/platform/catalog/pipelines/" -n pipeline-catalog > /dev/null 2>&1; then
    echo -e "${GREEN}  ✓${NC} Pipeline catalog pipelines deployed"
else
    echo -e "${YELLOW}  ⚠${NC} Some catalog pipelines may have failed to deploy"
fi

echo ""
echo "=========================================="
echo "  Step 7: Register Platform Repository"
echo "=========================================="
echo ""

# Create platform RepoBinding
echo -e "${BLUE}▸${NC} Creating platform RepoBinding..."
set +e  # Temporarily disable exit on error
REPOBINDING_OUTPUT=$(cat <<EOF | kubectl apply -f - 2>&1
apiVersion: arbiter.io/v1alpha1
kind: RepoBinding
metadata:
  name: platform-binding
  namespace: pipeline-system
spec:
  repoOrg: "${REPO_ORG}"
  repoName: "${REPO_NAME}"
  tenantName: "tenant-platform-infra"
  permissionProfile: "elevated"
EOF
)
REPOBINDING_EXIT_CODE=$?
set -e  # Re-enable exit on error

if [ $REPOBINDING_EXIT_CODE -eq 0 ]; then
    echo -e "${GREEN}  ✓${NC} Platform RepoBinding created"
else
    echo -e "${RED}  ✗${NC} Platform RepoBinding creation failed:"
    echo "${REPOBINDING_OUTPUT}"
    exit 1
fi

# Wait for tenant-platform-infra namespace to be created by onboarding controller
echo -e "${BLUE}▸${NC} Waiting for onboarding controller to create tenant-platform-infra namespace..."
timeout=60
elapsed=0
while ! kubectl get namespace tenant-platform-infra &> /dev/null; do
    if [ $elapsed -ge $timeout ]; then
        echo -e "${RED}  ✗${NC} Timeout waiting for tenant-platform-infra namespace"
        echo "Checking onboarding controller logs:"
        kubectl logs -n pipeline-system -l app=onboarding-controller --tail=20
        exit 1
    fi
    sleep 2
    elapsed=$((elapsed + 2))
done
echo -e "${GREEN}  ✓${NC} tenant-platform-infra namespace created by onboarding controller"

# Deploy platform upgrade pipeline
echo -e "${BLUE}▸${NC} Deploying platform upgrade pipeline..."
if kubectl apply -f "${REPO_ROOT}/platform/catalog/pipelines/platform-upgrade-pipeline.yaml" -n tenant-platform-infra > /dev/null 2>&1; then
    echo -e "${GREEN}  ✓${NC} Platform upgrade pipeline deployed"
else
    echo -e "${YELLOW}  ⚠${NC} Failed to deploy platform upgrade pipeline"
fi

echo ""
echo "=========================================="
echo -e "${GREEN}  ✓ Bootstrap Complete!${NC}"
echo "=========================================="

echo ""
echo "Installed components:"
echo ""
echo "Tekton Pipelines:"
kubectl get pods -n tekton-pipelines
echo ""
echo "Lighthouse:"
kubectl get deployments -n pipeline-system -l app.kubernetes.io/name=lighthouse
echo ""
echo "=========================================="
echo "  GitHub Webhook Configuration"
echo "=========================================="
echo ""
echo -e "${GREEN}GitHub App Webhook Secret:${NC}"
echo ""
echo "  ${GITHUB_WEBHOOK_SECRET}"
echo ""
echo -e "${YELLOW}Configure GitHub App webhook:${NC}"
echo ""
echo "  1. Go to your GitHub App settings:"
echo "     https://github.com/settings/apps"
echo ""
echo "  2. Webhook URL:"
if [ "${CLUSTER_TYPE}" = "kind" ]; then
    echo "     http://localhost/hook"
    echo ""
    echo "     ${YELLOW}Note:${NC} For Kind clusters, use localhost since ports are mapped to your host"
else
    # Get LoadBalancer IP if available
    LIGHTHOUSE_ENDPOINT=$(kubectl get ingress lighthouse-webhooks -n pipeline-system -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null)
    if [ -z "$LIGHTHOUSE_ENDPOINT" ]; then
        LIGHTHOUSE_ENDPOINT=$(kubectl get ingress lighthouse-webhooks -n pipeline-system -o jsonpath='{.status.loadBalancer.ingress[0].hostname}' 2>/dev/null)
    fi
    
    if [ -n "$LIGHTHOUSE_ENDPOINT" ]; then
        echo "     http://${LIGHTHOUSE_ENDPOINT}/hook"
    else
        echo "     ${YELLOW}Waiting for LoadBalancer IP...${NC}"
        echo "     Run: kubectl get ingress lighthouse-webhooks -n pipeline-system"
    fi
fi
echo ""
echo "  3. Webhook secret:"
echo "     ${GITHUB_WEBHOOK_SECRET}"
echo ""
echo "  4. Subscribe to events:"
echo "     - Push"
echo "     - Pull request"
echo ""
echo "  5. Ensure webhook is Active"
echo ""
echo -e "${YELLOW}Note:${NC} The platform repository uses the GitHub App webhook above."
echo "Individual tenant repositories will have their own webhook secrets"
echo "generated by the onboarding controller when RepoBindings are created."
echo ""
echo "=========================================="
echo ""
echo "Next steps:"
echo "  1. Configure GitHub webhook (see instructions above)"
echo "  2. Create RepoBinding for your repositories"
echo "  3. Deploy pipeline catalog"
echo ""
