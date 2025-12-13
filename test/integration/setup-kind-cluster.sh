#!/bin/bash
# Setup script for kind cluster with Argo Workflows and Argo Events
# This script is idempotent and can be run multiple times

set -e

CLUSTER_NAME="${KIND_CLUSTER_NAME:-arbiter-test}"
ARGO_NAMESPACE="${ARGO_NAMESPACE:-argo}"

echo "🚀 Setting up kind cluster for integration tests..."

# Check if kind is installed
if ! command -v kind &> /dev/null; then
    echo "❌ kind is not installed. Please install it first:"
    echo "   brew install kind  # macOS"
    echo "   or visit: https://kind.sigs.k8s.io/docs/user/quick-start/#installation"
    exit 1
fi

# Check if kubectl is installed
if ! command -v kubectl &> /dev/null; then
    echo "❌ kubectl is not installed. Please install it first:"
    echo "   brew install kubectl  # macOS"
    exit 1
fi

# Check if helm is installed
if ! command -v helm &> /dev/null; then
    echo "❌ helm is not installed. Please install it first:"
    echo "   brew install helm  # macOS"
    exit 1
fi

# Check if cluster already exists
if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
    echo "✅ kind cluster '${CLUSTER_NAME}' already exists"
else
    echo "📦 Creating kind cluster '${CLUSTER_NAME}'..."
    kind create cluster --name "${CLUSTER_NAME}" --wait 60s
    echo "✅ kind cluster created"
fi

# Set kubectl context
kubectl config use-context "kind-${CLUSTER_NAME}"

# Create argo namespace if it doesn't exist
if kubectl get namespace "${ARGO_NAMESPACE}" &> /dev/null; then
    echo "✅ Namespace '${ARGO_NAMESPACE}' already exists"
else
    echo "📦 Creating namespace '${ARGO_NAMESPACE}'..."
    kubectl create namespace "${ARGO_NAMESPACE}"
    echo "✅ Namespace created"
fi

# Add Argo Helm repository
echo "📦 Adding Argo Helm repository..."
helm repo add argo https://argoproj.github.io/argo-helm 2>/dev/null || true
helm repo update

# Install Argo Workflows if not already installed
if helm list -n "${ARGO_NAMESPACE}" | grep -q "argo-workflows"; then
    echo "✅ Argo Workflows already installed"
else
    echo "📦 Installing Argo Workflows..."
    helm install argo-workflows argo/argo-workflows \
        --namespace "${ARGO_NAMESPACE}" \
        --set server.enabled=true \
        --set controller.workflowDefaults.spec.serviceAccountName=argo-workflow \
        --set workflow.serviceAccount.create=true \
        --set workflow.serviceAccount.name=argo-workflow \
        --wait \
        --timeout 5m
    echo "✅ Argo Workflows installed"
fi

# Install Argo Events if not already installed
if helm list -n "${ARGO_NAMESPACE}" | grep -q "argo-events"; then
    echo "✅ Argo Events already installed"
else
    echo "📦 Installing Argo Events..."
    helm install argo-events argo/argo-events \
        --namespace "${ARGO_NAMESPACE}" \
        --wait \
        --timeout 5m
    echo "✅ Argo Events installed"
fi

# Wait for Argo Workflows controller to be ready
echo "⏳ Waiting for Argo Workflows controller to be ready..."
kubectl wait --for=condition=ready pod \
    -l app.kubernetes.io/name=argo-workflows-workflow-controller \
    -n "${ARGO_NAMESPACE}" \
    --timeout=120s

# Wait for Argo Workflows server to be ready
echo "⏳ Waiting for Argo Workflows server to be ready..."
kubectl wait --for=condition=ready pod \
    -l app.kubernetes.io/name=argo-workflows-server \
    -n "${ARGO_NAMESPACE}" \
    --timeout=120s

# Wait for Argo Events controller to be ready
echo "⏳ Waiting for Argo Events controller to be ready..."
kubectl wait --for=condition=ready pod \
    -l app.kubernetes.io/component=controller-manager \
    -n "${ARGO_NAMESPACE}" \
    --timeout=120s

echo "✅ kind cluster is ready for integration tests!"
echo ""
echo "Cluster name: ${CLUSTER_NAME}"
echo "Namespace: ${ARGO_NAMESPACE}"
echo "Context: kind-${CLUSTER_NAME}"
echo ""
echo "To interact with the cluster:"
echo "  kubectl config use-context kind-${CLUSTER_NAME}"
echo "  kubectl get pods -n ${ARGO_NAMESPACE}"
