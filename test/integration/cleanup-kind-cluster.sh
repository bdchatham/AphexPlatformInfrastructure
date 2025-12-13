#!/bin/bash
# Cleanup script for kind cluster
# Removes the test cluster and cleans up resources

set -e

CLUSTER_NAME="${KIND_CLUSTER_NAME:-arbiter-test}"

echo "🧹 Cleaning up kind cluster..."

# Check if kind is installed
if ! command -v kind &> /dev/null; then
    echo "⚠️  kind is not installed, nothing to clean up"
    exit 0
fi

# Check if cluster exists
if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
    echo "🗑️  Deleting kind cluster '${CLUSTER_NAME}'..."
    kind delete cluster --name "${CLUSTER_NAME}"
    echo "✅ Cluster deleted"
else
    echo "✅ Cluster '${CLUSTER_NAME}' does not exist, nothing to clean up"
fi

# Clean up any dangling Docker resources from kind
echo "🧹 Cleaning up Docker resources..."
docker system prune -f --filter "label=io.x-k8s.kind.cluster=${CLUSTER_NAME}" 2>/dev/null || true

echo "✅ Cleanup complete!"
