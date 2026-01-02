#!/bin/bash
set -euo pipefail

echo "=========================================="
echo "Testing Cross-Namespace Access Denial"
echo "=========================================="

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test configuration
FIRST_TENANT="test-tenant"
SECOND_TENANT="test-tenant-2"

echo ""
echo "Test Configuration:"
echo "  First Tenant: ${FIRST_TENANT}"
echo "  Second Tenant: ${SECOND_TENANT}"
echo ""

# Function to print success
success() {
    echo -e "${GREEN}✓ $1${NC}"
}

# Function to print error
error() {
    echo -e "${RED}✗ $1${NC}"
}

# Function to print info
info() {
    echo -e "${YELLOW}ℹ $1${NC}"
}

# Step 1: Create a test pod in first tenant namespace
echo "=========================================="
echo "Step 1: Creating test pod in first tenant"
echo "=========================================="

cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: test-pod-tenant1
  namespace: ${FIRST_TENANT}
spec:
  serviceAccountName: pipeline-runner
  containers:
  - name: kubectl
    image: bitnami/kubectl:latest
    command: ["sleep", "3600"]
  restartPolicy: Never
EOF

# Wait for pod to be ready
kubectl wait --for=condition=ready pod/test-pod-tenant1 -n ${FIRST_TENANT} --timeout=60s
success "Test pod created in ${FIRST_TENANT}"

# Step 2: Try to access resources in second tenant namespace
echo ""
echo "=========================================="
echo "Step 2: Attempting to access second tenant resources"
echo "=========================================="

info "Attempting to list pods in ${SECOND_TENANT} from ${FIRST_TENANT} pod..."

# Try to list pods in second tenant namespace (should fail)
OUTPUT=$(kubectl exec test-pod-tenant1 -n ${FIRST_TENANT} -- kubectl get pods -n ${SECOND_TENANT} 2>&1 || true)

if echo "$OUTPUT" | grep -qE "(Forbidden|forbidden)"; then
    success "Access to ${SECOND_TENANT} pods denied (as expected)"
else
    error "Access to ${SECOND_TENANT} pods was NOT denied (security issue!)"
    echo "Output: $OUTPUT"
    kubectl delete pod test-pod-tenant1 -n ${FIRST_TENANT}
    exit 1
fi

# Step 3: Try to access secrets in second tenant namespace
echo ""
echo "=========================================="
echo "Step 3: Attempting to access second tenant secrets"
echo "=========================================="

info "Attempting to list secrets in ${SECOND_TENANT} from ${FIRST_TENANT} pod..."

OUTPUT=$(kubectl exec test-pod-tenant1 -n ${FIRST_TENANT} -- kubectl get secrets -n ${SECOND_TENANT} 2>&1 || true)

if echo "$OUTPUT" | grep -qE "(Forbidden|forbidden)"; then
    success "Access to ${SECOND_TENANT} secrets denied (as expected)"
else
    error "Access to ${SECOND_TENANT} secrets was NOT denied (security issue!)"
    echo "Output: $OUTPUT"
    kubectl delete pod test-pod-tenant1 -n ${FIRST_TENANT}
    exit 1
fi

# Step 4: Try to access configmaps in second tenant namespace
echo ""
echo "=========================================="
echo "Step 4: Attempting to access second tenant configmaps"
echo "=========================================="

info "Attempting to list configmaps in ${SECOND_TENANT} from ${FIRST_TENANT} pod..."

OUTPUT=$(kubectl exec test-pod-tenant1 -n ${FIRST_TENANT} -- kubectl get configmaps -n ${SECOND_TENANT} 2>&1 || true)

if echo "$OUTPUT" | grep -qE "(Forbidden|forbidden)"; then
    success "Access to ${SECOND_TENANT} configmaps denied (as expected)"
else
    error "Access to ${SECOND_TENANT} configmaps was NOT denied (security issue!)"
    echo "Output: $OUTPUT"
    kubectl delete pod test-pod-tenant1 -n ${FIRST_TENANT}
    exit 1
fi

# Step 5: Verify access to own namespace still works
echo ""
echo "=========================================="
echo "Step 5: Verifying access to own namespace"
echo "=========================================="

info "Attempting to list pods in ${FIRST_TENANT} from ${FIRST_TENANT} pod..."

if kubectl exec test-pod-tenant1 -n ${FIRST_TENANT} -- kubectl get pods -n ${FIRST_TENANT} 2>&1 | grep -q "test-pod-tenant1"; then
    success "Access to own namespace ${FIRST_TENANT} works correctly"
else
    error "Access to own namespace ${FIRST_TENANT} failed (unexpected)"
    kubectl exec test-pod-tenant1 -n ${FIRST_TENANT} -- kubectl get pods -n ${FIRST_TENANT} 2>&1
    kubectl delete pod test-pod-tenant1 -n ${FIRST_TENANT}
    exit 1
fi

# Cleanup
echo ""
echo "=========================================="
echo "Cleanup"
echo "=========================================="

kubectl delete pod test-pod-tenant1 -n ${FIRST_TENANT}
success "Test pod deleted"

echo ""
echo "=========================================="
echo -e "${GREEN}✓ Cross-namespace access denial verified${NC}"
echo "=========================================="
echo ""
echo "Summary:"
echo "  - First tenant cannot access second tenant pods"
echo "  - First tenant cannot access second tenant secrets"
echo "  - First tenant cannot access second tenant configmaps"
echo "  - First tenant can access its own namespace resources"
echo ""
