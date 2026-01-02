#!/bin/bash
set -euo pipefail

echo "=========================================="
echo "Testing Tenant Isolation"
echo "=========================================="

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test configuration
FIRST_TENANT="test-tenant"
SECOND_TENANT="test-tenant-2"
SECOND_REPO_ORG="your-github-org"
SECOND_REPO_NAME="test-pipeline-execution-2"

echo ""
echo "Test Configuration:"
echo "  First Tenant: ${FIRST_TENANT}"
echo "  Second Tenant: ${SECOND_TENANT}"
echo "  Second Repo: ${SECOND_REPO_ORG}/${SECOND_REPO_NAME}"
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

# Step 1: Create second tenant RepoBinding
echo "=========================================="
echo "Step 1: Creating second tenant RepoBinding"
echo "=========================================="

cat <<EOF | kubectl apply -f -
apiVersion: platform.arbiter.io/v1alpha1
kind: RepoBinding
metadata:
  name: test-tenant-2-binding
  namespace: pipeline-system
  labels:
    test: "true"
    purpose: "tenant-isolation-test"
spec:
  repoOrg: "${SECOND_REPO_ORG}"
  repoName: "${SECOND_REPO_NAME}"
  tenantName: "${SECOND_TENANT}"
  permissionProfile: "standard"
EOF

if [ $? -eq 0 ]; then
    success "Created RepoBinding for second tenant"
else
    error "Failed to create RepoBinding for second tenant"
    exit 1
fi

# Step 2: Wait for second tenant namespace to be created
echo ""
echo "=========================================="
echo "Step 2: Waiting for second tenant namespace"
echo "=========================================="

info "Waiting for RepoBinding to be reconciled..."
sleep 5

# Wait up to 60 seconds for namespace to be created
TIMEOUT=60
ELAPSED=0
while [ $ELAPSED -lt $TIMEOUT ]; do
    if kubectl get namespace ${SECOND_TENANT} &>/dev/null; then
        success "Second tenant namespace '${SECOND_TENANT}' created"
        break
    fi
    sleep 2
    ELAPSED=$((ELAPSED + 2))
done

if [ $ELAPSED -ge $TIMEOUT ]; then
    error "Timeout waiting for second tenant namespace"
    exit 1
fi

# Step 3: Verify second tenant namespace labels
echo ""
echo "=========================================="
echo "Step 3: Verifying second tenant namespace labels"
echo "=========================================="

TENANT_LABEL=$(kubectl get namespace ${SECOND_TENANT} -o jsonpath='{.metadata.labels.platform\.arbiter\.io/tenant}')
REPO_LABEL=$(kubectl get namespace ${SECOND_TENANT} -o jsonpath='{.metadata.labels.platform\.arbiter\.io/repo}')
EXPECTED_REPO_LABEL="${SECOND_REPO_ORG}-${SECOND_REPO_NAME}"

if [ "${TENANT_LABEL}" == "${SECOND_TENANT}" ]; then
    success "Tenant label is correct: ${TENANT_LABEL}"
else
    error "Tenant label is incorrect. Expected: ${SECOND_TENANT}, Got: ${TENANT_LABEL}"
    exit 1
fi

if [ "${REPO_LABEL}" == "${EXPECTED_REPO_LABEL}" ]; then
    success "Repo label is correct: ${REPO_LABEL}"
else
    error "Repo label is incorrect. Expected: ${EXPECTED_REPO_LABEL}, Got: ${REPO_LABEL}"
    exit 1
fi

# Step 4: Verify second tenant service account
echo ""
echo "=========================================="
echo "Step 4: Verifying second tenant service account"
echo "=========================================="

if kubectl get serviceaccount pipeline-runner -n ${SECOND_TENANT} &>/dev/null; then
    success "Service account 'pipeline-runner' exists in ${SECOND_TENANT}"
else
    error "Service account 'pipeline-runner' not found in ${SECOND_TENANT}"
    exit 1
fi

# Step 5: Verify second tenant RBAC
echo ""
echo "=========================================="
echo "Step 5: Verifying second tenant RBAC"
echo "=========================================="

if kubectl get role pipeline-runner -n ${SECOND_TENANT} &>/dev/null; then
    success "Role 'pipeline-runner' exists in ${SECOND_TENANT}"
else
    error "Role 'pipeline-runner' not found in ${SECOND_TENANT}"
    exit 1
fi

if kubectl get rolebinding pipeline-runner -n ${SECOND_TENANT} &>/dev/null; then
    success "RoleBinding 'pipeline-runner' exists in ${SECOND_TENANT}"
else
    error "RoleBinding 'pipeline-runner' not found in ${SECOND_TENANT}"
    exit 1
fi

# Step 6: Verify second tenant resource limits
echo ""
echo "=========================================="
echo "Step 6: Verifying second tenant resource limits"
echo "=========================================="

if kubectl get resourcequota tenant-quota -n ${SECOND_TENANT} &>/dev/null; then
    success "ResourceQuota 'tenant-quota' exists in ${SECOND_TENANT}"
else
    error "ResourceQuota 'tenant-quota' not found in ${SECOND_TENANT}"
    exit 1
fi

if kubectl get limitrange tenant-limits -n ${SECOND_TENANT} &>/dev/null; then
    success "LimitRange 'tenant-limits' exists in ${SECOND_TENANT}"
else
    error "LimitRange 'tenant-limits' not found in ${SECOND_TENANT}"
    exit 1
fi

# Step 7: Verify second tenant network policy
echo ""
echo "=========================================="
echo "Step 7: Verifying second tenant network policy"
echo "=========================================="

if kubectl get networkpolicy tenant-isolation -n ${SECOND_TENANT} &>/dev/null; then
    success "NetworkPolicy 'tenant-isolation' exists in ${SECOND_TENANT}"
else
    error "NetworkPolicy 'tenant-isolation' not found in ${SECOND_TENANT}"
    exit 1
fi

# Step 8: Verify RepoBinding status
echo ""
echo "=========================================="
echo "Step 8: Verifying RepoBinding status"
echo "=========================================="

PHASE=$(kubectl get repobinding test-tenant-2-binding -n pipeline-system -o jsonpath='{.status.phase}')

if [ "${PHASE}" == "Ready" ]; then
    success "RepoBinding status is 'Ready'"
else
    error "RepoBinding status is not 'Ready'. Current status: ${PHASE}"
    kubectl get repobinding test-tenant-2-binding -n pipeline-system -o yaml
    exit 1
fi

echo ""
echo "=========================================="
echo -e "${GREEN}✓ Second tenant created successfully${NC}"
echo "=========================================="
echo ""
echo "Summary:"
echo "  - Second tenant namespace: ${SECOND_TENANT}"
echo "  - Service account: pipeline-runner"
echo "  - RBAC configured: Role + RoleBinding"
echo "  - Resource limits: ResourceQuota + LimitRange"
echo "  - Network isolation: NetworkPolicy"
echo "  - RepoBinding status: Ready"
echo ""
