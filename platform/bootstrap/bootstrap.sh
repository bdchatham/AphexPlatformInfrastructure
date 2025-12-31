#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
CLUSTER_NAME="arbiter-infrastructure"
MIN_KUBECTL_VERSION="1.24.0"
MIN_HELM_VERSION="3.0.0"
MIN_K8S_VERSION="1.24.0"

echo "=========================================="
echo "Jenkins X Platform Bootstrap"
echo "=========================================="
echo ""

# Check if cluster creation is needed
echo "Checking for existing Kubernetes cluster..."
if ! kubectl cluster-info &> /dev/null; then
    echo -e "${YELLOW}No accessible cluster found${NC}"
    echo ""
    
    # Check if kind is available
    if command -v kind &> /dev/null; then
        echo "Kind is available for local cluster creation"
        read -p "Would you like to create a local Kind cluster named '${CLUSTER_NAME}'? (y/n) " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            echo ""
            echo "=========================================="
            echo "Creating Kind Cluster: ${CLUSTER_NAME}"
            echo "=========================================="
            echo ""
            
            # Check if cluster already exists
            if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
                echo -e "${YELLOW}Cluster ${CLUSTER_NAME} already exists${NC}"
                read -p "Delete and recreate? (y/n) " -n 1 -r
                echo
                if [[ $REPLY =~ ^[Yy]$ ]]; then
                    echo "Deleting existing cluster..."
                    kind delete cluster --name "${CLUSTER_NAME}"
                else
                    echo "Using existing cluster..."
                fi
            fi
            
            # Create cluster if it doesn't exist
            if ! kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
                echo "Creating Kind cluster with ingress support..."
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
                    echo -e "${GREEN}Kind cluster created successfully${NC}"
                    
                    # Set kubectl context
                    kubectl config use-context "kind-${CLUSTER_NAME}"
                    echo -e "${GREEN}kubectl context set to kind-${CLUSTER_NAME}${NC}"
                else
                    echo -e "${RED}Failed to create Kind cluster${NC}"
                    exit 1
                fi
            fi
        else
            echo "Cluster creation skipped. Please configure kubectl to access your cluster."
            exit 1
        fi
    else
        echo -e "${RED}No cluster access and Kind is not installed${NC}"
        echo ""
        echo "Options:"
        echo "  1. Install Kind: https://kind.sigs.k8s.io/docs/user/quick-start/#installation"
        echo "  2. Configure kubectl to access an existing cluster"
        echo "  3. Install k3s for homelab: curl -sfL https://get.k3s.io | sh -"
        exit 1
    fi
else
    echo -e "${GREEN}Cluster access confirmed${NC}"
    CURRENT_CONTEXT=$(kubectl config current-context)
    echo "Current context: ${CURRENT_CONTEXT}"
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
echo "Prerequisites Check"
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
echo -n "Checking kubectl installation... "
if ! command -v kubectl &> /dev/null; then
    echo -e "${RED}FAILED${NC}"
    echo "kubectl is not installed. Please install kubectl version ${MIN_KUBECTL_VERSION} or higher."
    echo "Visit: https://kubernetes.io/docs/tasks/tools/"
    exit 1
fi
echo -e "${GREEN}OK${NC}"

# Check kubectl version
echo -n "Checking kubectl version... "
KUBECTL_VERSION=$(kubectl version --client 2>/dev/null | grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+' | head -n1 | sed 's/v//')
if [ -z "$KUBECTL_VERSION" ]; then
    # Fallback for older kubectl versions
    KUBECTL_VERSION=$(kubectl version --client --short 2>/dev/null | extract_version)
fi

if [ -z "$KUBECTL_VERSION" ]; then
    echo -e "${RED}FAILED${NC}"
    echo "Could not determine kubectl version"
    exit 1
fi

if version_ge "$KUBECTL_VERSION" "$MIN_KUBECTL_VERSION"; then
    echo -e "${GREEN}OK${NC} (v${KUBECTL_VERSION})"
else
    echo -e "${RED}FAILED${NC}"
    echo "kubectl version ${KUBECTL_VERSION} is too old. Minimum required: ${MIN_KUBECTL_VERSION}"
    exit 1
fi

# Check if helm is installed
echo -n "Checking helm installation... "
if ! command -v helm &> /dev/null; then
    echo -e "${RED}FAILED${NC}"
    echo "helm is not installed. Please install helm version ${MIN_HELM_VERSION} or higher."
    echo "Visit: https://helm.sh/docs/intro/install/"
    exit 1
fi
echo -e "${GREEN}OK${NC}"

# Check helm version
echo -n "Checking helm version... "
HELM_VERSION=$(helm version 2>/dev/null | grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+' | head -n1 | sed 's/v//')
if [ -z "$HELM_VERSION" ]; then
    echo -e "${RED}FAILED${NC}"
    echo "Could not determine helm version"
    exit 1
fi

if version_ge "$HELM_VERSION" "$MIN_HELM_VERSION"; then
    echo -e "${GREEN}OK${NC} (v${HELM_VERSION})"
else
    echo -e "${RED}FAILED${NC}"
    echo "helm version ${HELM_VERSION} is too old. Minimum required: ${MIN_HELM_VERSION}"
    exit 1
fi

# Check cluster access
echo -n "Checking cluster access... "
if ! kubectl cluster-info &> /dev/null; then
    echo -e "${RED}FAILED${NC}"
    echo "Cannot access Kubernetes cluster. Please check your kubeconfig."
    exit 1
fi
echo -e "${GREEN}OK${NC}"

# Check Kubernetes version
echo -n "Checking Kubernetes version... "
K8S_VERSION=$(kubectl version 2>/dev/null | grep "Server Version" | grep -oE 'v[0-9]+\.[0-9]+\.[0-9]+' | sed 's/v//')
if [ -z "$K8S_VERSION" ]; then
    # Fallback for older kubectl versions
    K8S_VERSION=$(kubectl version --short 2>/dev/null | grep "Server Version" | extract_version)
fi

if [ -z "$K8S_VERSION" ]; then
    echo -e "${YELLOW}WARNING${NC}"
    echo "Could not determine Kubernetes server version"
else
    if version_ge "$K8S_VERSION" "$MIN_K8S_VERSION"; then
        echo -e "${GREEN}OK${NC} (v${K8S_VERSION})"
    else
        echo -e "${RED}FAILED${NC}"
        echo "Kubernetes version ${K8S_VERSION} is too old. Minimum required: ${MIN_K8S_VERSION}"
        exit 1
    fi
fi

# Check for RBAC support
echo -n "Checking RBAC support... "
if kubectl api-versions | grep -q "rbac.authorization.k8s.io"; then
    echo -e "${GREEN}OK${NC}"
else
    echo -e "${RED}FAILED${NC}"
    echo "RBAC is not enabled on this cluster. RBAC is required for Jenkins X platform."
    exit 1
fi

# Check for NetworkPolicy support
echo -n "Checking NetworkPolicy support... "
if kubectl api-versions | grep -q "networking.k8s.io"; then
    echo -e "${GREEN}OK${NC}"
else
    echo -e "${YELLOW}WARNING${NC}"
    echo "NetworkPolicy API is not available. Network isolation may not work."
    echo "Consider installing a CNI plugin that supports NetworkPolicy (e.g., Calico, Cilium)."
fi

# Check cluster permissions
echo -n "Checking cluster admin permissions... "
if kubectl auth can-i create namespaces --all-namespaces &> /dev/null; then
    echo -e "${GREEN}OK${NC}"
else
    echo -e "${RED}FAILED${NC}"
    echo "Current user does not have cluster admin permissions."
    echo "Bootstrap requires the ability to create namespaces and cluster-wide resources."
    exit 1
fi

echo ""
echo "=========================================="
echo -e "${GREEN}All prerequisites checks passed!${NC}"
echo "=========================================="
echo ""

# Ask user if they want to proceed with installation
read -p "Do you want to proceed with Tekton Pipelines installation? (y/n) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Installation cancelled."
    exit 0
fi

echo ""
echo "=========================================="
echo "Installing Tekton Pipelines"
echo "=========================================="
echo ""

# Install Tekton Pipelines
# Note: Tekton migrated from gcr.io to ghcr.io in 2025
TEKTON_VERSION="v0.56.0"
TEKTON_RELEASE_URL="https://github.com/tektoncd/pipeline/releases/download/${TEKTON_VERSION}/release.yaml"

echo "Installing Tekton Pipelines ${TEKTON_VERSION}..."
# Download and patch the manifest to use ghcr.io instead of gcr.io
if curl -sL "${TEKTON_RELEASE_URL}" | \
   sed 's|gcr.io/tekton-releases|ghcr.io/tektoncd|g' | \
   kubectl apply -f -; then
    echo -e "${GREEN}Tekton Pipelines installed successfully${NC}"
else
    echo -e "${RED}Failed to install Tekton Pipelines${NC}"
    exit 1
fi

echo ""
echo "Waiting for Tekton Pipelines namespace to be created..."
timeout=60
elapsed=0
while ! kubectl get namespace tekton-pipelines &> /dev/null; do
    if [ $elapsed -ge $timeout ]; then
        echo -e "${RED}Timeout waiting for tekton-pipelines namespace${NC}"
        exit 1
    fi
    sleep 2
    elapsed=$((elapsed + 2))
    echo -n "."
done
echo -e "\n${GREEN}tekton-pipelines namespace created${NC}"

echo ""
echo "=========================================="
echo "Waiting for Tekton Controllers"
echo "=========================================="
echo ""

# Wait for tekton-pipelines-controller
echo "Waiting for tekton-pipelines-controller to be ready..."
if kubectl wait --for=condition=ready pod \
    -l app.kubernetes.io/name=controller \
    -n tekton-pipelines \
    --timeout=300s; then
    echo -e "${GREEN}tekton-pipelines-controller is ready${NC}"
else
    echo -e "${RED}tekton-pipelines-controller failed to become ready${NC}"
    echo "Checking pod status:"
    kubectl get pods -n tekton-pipelines -l app.kubernetes.io/name=controller
    exit 1
fi

# Wait for tekton-pipelines-webhook
echo ""
echo "Waiting for tekton-pipelines-webhook to be ready..."
if kubectl wait --for=condition=ready pod \
    -l app.kubernetes.io/name=webhook \
    -n tekton-pipelines \
    --timeout=300s; then
    echo -e "${GREEN}tekton-pipelines-webhook is ready${NC}"
else
    echo -e "${RED}tekton-pipelines-webhook failed to become ready${NC}"
    echo "Checking pod status:"
    kubectl get pods -n tekton-pipelines -l app.kubernetes.io/name=webhook
    exit 1
fi

echo ""
echo "=========================================="
echo -e "${GREEN}Tekton Pipelines installation complete!${NC}"
echo "=========================================="
echo ""
echo "Installed components:"
kubectl get pods -n tekton-pipelines
echo ""
echo "Next steps:"
echo "  1. Install Jenkins X and Lighthouse (task 6)"
echo "  2. Create RepoBinding CRD (task 7)"
echo "  3. Deploy onboarding controller (task 9-11)"
