#!/bin/bash
set -euo pipefail

# Onboard Test Repository
# This script creates a RepoBinding for the test repository and waits for onboarding to complete

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_REPO_ORG="${TEST_REPO_ORG:-your-github-org}"
TEST_REPO_NAME="${TEST_REPO_NAME:-test-pipeline-execution}"
TEST_TENANT_NAME="${TEST_TENANT_NAME:-test-tenant}"
TIMEOUT="${TIMEOUT:-300}"

# Color codes
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

function log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

function log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

function log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

echo "========================================="
echo "Onboarding Test Repository"
echo "========================================="
echo "Repository: ${TEST_REPO_ORG}/${TEST_REPO_NAME}"
echo "Tenant: ${TEST_TENANT_NAME}"
echo ""

# Check if RepoBinding already exists
if kubectl get repobinding ${TEST_TENANT_NAME}-binding -n pipeline-system &> /dev/null; then
    log_warn "RepoBinding already exists"
    read -p "Delete and recreate? (y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        log_info "Deleting existing RepoBinding..."
        kubectl delete repobinding ${TEST_TENANT_NAME}-binding -n pipeline-system
        
        # Wait for namespace to be deleted
        log_info "Waiting for tenant namespace to be cleaned up..."
        local elapsed=0
        while kubectl get namespace ${TEST_TENANT_NAME} &> /dev/null && [ $elapsed -lt 60 ]; do
            echo -n "."
            sleep 2
            elapsed=$((elapsed + 2))
        done
        echo ""
    else
        log_info "Using existing RepoBinding"
        exit 0
    fi
fi

# Create RepoBinding
log_info "Creating RepoBinding..."

cat <<EOF | kubectl apply -f -
apiVersion: platform.arbiter.io/v1alpha1
kind: RepoBinding
metadata:
  name: ${TEST_TENANT_NAME}-binding
  namespace: pipeline-system
  labels:
    test: "true"
    purpose: "pipeline-execution-test"
spec:
  repoOrg: "${TEST_REPO_ORG}"
  repoName: "${TEST_REPO_NAME}"
  tenantName: "${TEST_TENANT_NAME}"
  permissionProfile: "standard"
EOF

log_info "RepoBinding created, waiting for onboarding to complete..."

# Wait for onboarding
elapsed=0
interval=5

while [ $elapsed -lt $TIMEOUT ]; do
    phase=$(kubectl get repobinding ${TEST_TENANT_NAME}-binding -n pipeline-system -o jsonpath='{.status.phase}' 2>/dev/null || echo "")
    
    if [ "$phase" == "Ready" ]; then
        echo ""
        log_info "Onboarding completed successfully!"
        
        # Show status
        echo ""
        log_info "RepoBinding Status:"
        kubectl get repobinding ${TEST_TENANT_NAME}-binding -n pipeline-system -o yaml | grep -A 20 "status:"
        
        echo ""
        log_info "Tenant Resources:"
        echo "  Namespace: $(kubectl get namespace ${TEST_TENANT_NAME} -o name 2>/dev/null || echo 'NOT FOUND')"
        echo "  ServiceAccount: $(kubectl get serviceaccount pipeline-runner -n ${TEST_TENANT_NAME} -o name 2>/dev/null || echo 'NOT FOUND')"
        echo "  Role: $(kubectl get role pipeline-runner -n ${TEST_TENANT_NAME} -o name 2>/dev/null || echo 'NOT FOUND')"
        echo "  RoleBinding: $(kubectl get rolebinding pipeline-runner -n ${TEST_TENANT_NAME} -o name 2>/dev/null || echo 'NOT FOUND')"
        echo "  ResourceQuota: $(kubectl get resourcequota tenant-quota -n ${TEST_TENANT_NAME} -o name 2>/dev/null || echo 'NOT FOUND')"
        echo "  LimitRange: $(kubectl get limitrange tenant-limits -n ${TEST_TENANT_NAME} -o name 2>/dev/null || echo 'NOT FOUND')"
        echo "  NetworkPolicy: $(kubectl get networkpolicy tenant-isolation -n ${TEST_TENANT_NAME} -o name 2>/dev/null || echo 'NOT FOUND')"
        echo "  Terraform Secret: $(kubectl get secret terraform-backend-config -n ${TEST_TENANT_NAME} -o name 2>/dev/null || echo 'NOT FOUND')"
        
        echo ""
        log_info "Repository is ready for pipeline execution!"
        exit 0
    elif [ "$phase" == "Failed" ]; then
        echo ""
        message=$(kubectl get repobinding ${TEST_TENANT_NAME}-binding -n pipeline-system -o jsonpath='{.status.message}')
        log_error "Onboarding failed: $message"
        
        echo ""
        log_info "RepoBinding Status:"
        kubectl get repobinding ${TEST_TENANT_NAME}-binding -n pipeline-system -o yaml | grep -A 20 "status:"
        
        exit 1
    elif [ "$phase" == "Provisioning" ]; then
        echo -n "."
    else
        echo -n "."
    fi
    
    sleep $interval
    elapsed=$((elapsed + interval))
done

echo ""
log_error "Onboarding timed out after ${TIMEOUT}s"
log_info "Current status:"
kubectl get repobinding ${TEST_TENANT_NAME}-binding -n pipeline-system -o yaml | grep -A 20 "status:"
exit 1
