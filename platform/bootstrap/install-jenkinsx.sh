#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "=========================================="
echo "Installing Jenkins X and Lighthouse"
echo "=========================================="
echo ""

# Check if helm is installed
if ! command -v helm &> /dev/null; then
    echo -e "${RED}ERROR: helm is not installed${NC}"
    echo "Please install helm first: https://helm.sh/docs/intro/install/"
    exit 1
fi

# Check if kubectl is installed
if ! command -v kubectl &> /dev/null; then
    echo -e "${RED}ERROR: kubectl is not installed${NC}"
    echo "Please install kubectl first: https://kubernetes.io/docs/tasks/tools/"
    exit 1
fi

# Check cluster access
if ! kubectl cluster-info &> /dev/null; then
    echo -e "${RED}ERROR: Cannot access Kubernetes cluster${NC}"
    echo "Please check your kubeconfig"
    exit 1
fi

# Ensure pipeline-system namespace exists
echo "Checking pipeline-system namespace..."
if ! kubectl get namespace pipeline-system &> /dev/null; then
    echo "Creating pipeline-system namespace..."
    kubectl create namespace pipeline-system
    echo -e "${GREEN}pipeline-system namespace created${NC}"
else
    echo -e "${GREEN}pipeline-system namespace already exists${NC}"
fi

echo ""
echo "=========================================="
echo "Step 1: Add Jenkins X Helm Repository"
echo "=========================================="
echo ""

# Add Jenkins X Helm repository
echo "Adding Jenkins X Helm repository..."
if helm repo add jx3 https://jenkins-x-charts.github.io/repo 2>&1 | grep -q "already exists"; then
    echo -e "${YELLOW}Jenkins X Helm repository already exists${NC}"
else
    echo -e "${GREEN}Jenkins X Helm repository added${NC}"
fi

# Update Helm repository cache
echo ""
echo "Updating Helm repository cache..."
if helm repo update; then
    echo -e "${GREEN}Helm repository cache updated${NC}"
else
    echo -e "${RED}Failed to update Helm repository cache${NC}"
    exit 1
fi

# Verify jx3 repository is available
echo ""
echo "Verifying jx3 repository..."
if helm search repo jx3 | grep -q "jx3/"; then
    echo -e "${GREEN}jx3 repository is available${NC}"
    echo ""
    echo "Available charts:"
    helm search repo jx3 | head -n 10
else
    echo -e "${RED}jx3 repository not found${NC}"
    exit 1
fi

echo ""
echo "=========================================="
echo "Step 2: Install jx-build-controller"
echo "=========================================="
echo ""

# Install jx-build-controller
echo "Installing jx-build-controller to pipeline-system namespace..."
if helm list -n pipeline-system | grep -q "jx-build-controller"; then
    echo -e "${YELLOW}jx-build-controller is already installed${NC}"
    echo "Upgrading jx-build-controller..."
    if helm upgrade jx-build-controller jx3/jx-build-controller -n pipeline-system; then
        echo -e "${GREEN}jx-build-controller upgraded successfully${NC}"
    else
        echo -e "${RED}Failed to upgrade jx-build-controller${NC}"
        exit 1
    fi
else
    if helm install jx-build-controller jx3/jx-build-controller -n pipeline-system; then
        echo -e "${GREEN}jx-build-controller installed successfully${NC}"
    else
        echo -e "${RED}Failed to install jx-build-controller${NC}"
        exit 1
    fi
fi

# Wait for jx-build-controller to be ready
echo ""
echo "Waiting for jx-build-controller to be ready..."
timeout=120
elapsed=0
while ! kubectl get deployment jx-build-controller -n pipeline-system &> /dev/null; do
    if [ $elapsed -ge $timeout ]; then
        echo -e "${RED}Timeout waiting for jx-build-controller deployment${NC}"
        exit 1
    fi
    sleep 2
    elapsed=$((elapsed + 2))
    echo -n "."
done
echo ""

if kubectl wait --for=condition=available deployment/jx-build-controller \
    -n pipeline-system \
    --timeout=300s; then
    echo -e "${GREEN}jx-build-controller is ready${NC}"
else
    echo -e "${RED}jx-build-controller failed to become ready${NC}"
    echo "Checking deployment status:"
    kubectl get deployment jx-build-controller -n pipeline-system
    kubectl get pods -n pipeline-system -l app=jx-build-controller
    exit 1
fi

echo ""
echo "=========================================="
echo "Step 3: Create Lighthouse Configuration"
echo "=========================================="
echo ""

# Apply Lighthouse configuration ConfigMaps
echo "Creating Lighthouse configuration ConfigMaps..."
if kubectl apply -f platform/lighthouse/lighthouse-config.yaml; then
    echo -e "${GREEN}Lighthouse configuration ConfigMap created${NC}"
else
    echo -e "${RED}Failed to create Lighthouse configuration ConfigMap${NC}"
    exit 1
fi

if kubectl apply -f platform/lighthouse/repo-allowlist.yaml; then
    echo -e "${GREEN}Repository allowlist ConfigMap created${NC}"
else
    echo -e "${RED}Failed to create repository allowlist ConfigMap${NC}"
    exit 1
fi

echo ""
echo "=========================================="
echo "Step 4: Install Lighthouse"
echo "=========================================="
echo ""

# Check if GitHub App secret exists
if ! kubectl get secret lighthouse-github-app -n pipeline-system &> /dev/null; then
    echo -e "${RED}ERROR: GitHub App secret 'lighthouse-github-app' not found${NC}"
    echo ""
    echo "Please create the GitHub App secret first:"
    echo "  1. Follow the guide: platform/bootstrap/github-app-setup.md"
    echo "  2. Run: ./platform/bootstrap/create-github-app-secret.sh"
    echo ""
    exit 1
fi

# Get GitHub App credentials from secret
echo "Reading GitHub App credentials from secret..."
GITHUB_APP_ID=$(kubectl get secret lighthouse-github-app -n pipeline-system -o jsonpath='{.data.app-id}' | base64 -d)
GITHUB_APP_INSTALLATION_ID=$(kubectl get secret lighthouse-github-app -n pipeline-system -o jsonpath='{.data.installation-id}' | base64 -d)

if [ -z "$GITHUB_APP_ID" ] || [ -z "$GITHUB_APP_INSTALLATION_ID" ]; then
    echo -e "${RED}ERROR: Failed to read GitHub App credentials from secret${NC}"
    exit 1
fi

echo -e "${GREEN}GitHub App credentials loaded${NC}"
echo "  App ID: ${GITHUB_APP_ID}"
echo "  Installation ID: ${GITHUB_APP_INSTALLATION_ID}"

# Install Lighthouse
echo ""
echo "Installing Lighthouse to pipeline-system namespace..."
if helm list -n pipeline-system | grep -q "lighthouse"; then
    echo -e "${YELLOW}Lighthouse is already installed${NC}"
    echo "Upgrading Lighthouse..."
    if helm upgrade lighthouse jx3/lighthouse -n pipeline-system \
        --set github.appId="${GITHUB_APP_ID}" \
        --set github.appInstallationId="${GITHUB_APP_INSTALLATION_ID}" \
        --set github.secretName="lighthouse-github-app" \
        --set configMaps.config="lighthouse-config" \
        --set configMaps.allowlist="repo-allowlist"; then
        echo -e "${GREEN}Lighthouse upgraded successfully${NC}"
    else
        echo -e "${RED}Failed to upgrade Lighthouse${NC}"
        exit 1
    fi
else
    if helm install lighthouse jx3/lighthouse -n pipeline-system \
        --set github.appId="${GITHUB_APP_ID}" \
        --set github.appInstallationId="${GITHUB_APP_INSTALLATION_ID}" \
        --set github.secretName="lighthouse-github-app" \
        --set configMaps.config="lighthouse-config" \
        --set configMaps.allowlist="repo-allowlist"; then
        echo -e "${GREEN}Lighthouse installed successfully${NC}"
    else
        echo -e "${RED}Failed to install Lighthouse${NC}"
        exit 1
    fi
fi

echo ""
echo "=========================================="
echo "Step 5: Wait for Lighthouse to be Ready"
echo "=========================================="
echo ""

# Wait for Lighthouse deployment to be created
echo "Waiting for Lighthouse deployment to be created..."
timeout=120
elapsed=0
while ! kubectl get deployment lighthouse -n pipeline-system &> /dev/null; do
    if [ $elapsed -ge $timeout ]; then
        echo -e "${RED}Timeout waiting for Lighthouse deployment${NC}"
        exit 1
    fi
    sleep 2
    elapsed=$((elapsed + 2))
    echo -n "."
done
echo ""
echo -e "${GREEN}Lighthouse deployment created${NC}"

# Wait for Lighthouse to be ready
echo ""
echo "Waiting for Lighthouse to be ready..."
if kubectl wait --for=condition=available deployment/lighthouse \
    -n pipeline-system \
    --timeout=300s; then
    echo -e "${GREEN}Lighthouse is ready${NC}"
else
    echo -e "${RED}Lighthouse failed to become ready${NC}"
    echo ""
    echo "Checking deployment status:"
    kubectl get deployment lighthouse -n pipeline-system
    echo ""
    echo "Checking pod status:"
    kubectl get pods -n pipeline-system -l app=lighthouse
    echo ""
    echo "Checking pod logs:"
    kubectl logs -n pipeline-system -l app=lighthouse --tail=50
    exit 1
fi

# Verify webhook endpoint is accessible
echo ""
echo "Verifying Lighthouse webhook endpoint..."
if kubectl get service lighthouse -n pipeline-system &> /dev/null; then
    echo -e "${GREEN}Lighthouse service exists${NC}"
    kubectl get service lighthouse -n pipeline-system
    
    # Get service endpoint
    LIGHTHOUSE_ENDPOINT=$(kubectl get service lighthouse -n pipeline-system -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
    if [ -z "$LIGHTHOUSE_ENDPOINT" ]; then
        LIGHTHOUSE_ENDPOINT=$(kubectl get service lighthouse -n pipeline-system -o jsonpath='{.status.loadBalancer.ingress[0].hostname}')
    fi
    
    if [ -n "$LIGHTHOUSE_ENDPOINT" ]; then
        echo ""
        echo -e "${GREEN}Lighthouse webhook endpoint: http://${LIGHTHOUSE_ENDPOINT}/hook${NC}"
        echo ""
        echo "Configure this URL in your GitHub App webhook settings:"
        echo "  https://github.com/organizations/YOUR_ORG/settings/apps/YOUR_APP"
    else
        echo ""
        echo -e "${YELLOW}WARNING: LoadBalancer IP/hostname not yet assigned${NC}"
        echo "Run this command to get the endpoint once it's assigned:"
        echo "  kubectl get service lighthouse -n pipeline-system"
    fi
else
    echo -e "${YELLOW}WARNING: Lighthouse service not found${NC}"
    echo "This may be expected depending on the Helm chart configuration"
fi

echo ""
echo "=========================================="
echo -e "${GREEN}Jenkins X and Lighthouse installation complete!${NC}"
echo "=========================================="
echo ""
echo "Installed components:"
echo ""
echo "Jenkins X:"
kubectl get deployment jx-build-controller -n pipeline-system
echo ""
echo "Lighthouse:"
kubectl get deployment lighthouse -n pipeline-system
kubectl get pods -n pipeline-system -l app=lighthouse
echo ""
echo "Configuration:"
kubectl get configmap lighthouse-config -n pipeline-system
kubectl get configmap repo-allowlist -n pipeline-system
echo ""
echo "Secrets:"
kubectl get secret lighthouse-github-app -n pipeline-system
echo ""
echo "Next steps:"
echo "  1. Configure GitHub App webhook URL (see above)"
echo "  2. Create RepoBinding CRD (task 7)"
echo "  3. Deploy onboarding controller (tasks 9-11)"
echo "  4. Install pipeline catalog (task 12)"
