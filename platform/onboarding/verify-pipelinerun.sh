#!/bin/bash
set -euo pipefail

# Verify PipelineRun Creation
# This script verifies that a PipelineRun was created in the tenant namespace
# and that it uses the correct service account

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
echo "Verify PipelineRun Creation"
echo "========================================="
echo "Tenant: ${TEST_TENANT_NAME}"
echo ""

# Check if tenant namespace exists
if ! kubectl get namespace ${TEST_TENANT_NAME} &> /dev/null; then
    log_error "Tenant namespace not found: ${TEST_TENANT_NAME}"
    log_info "Run onboarding first: ./onboard-test-repo.sh"
    exit 1
fi

log_info "✓ Tenant namespace exists"

# Wait for PipelineRun to be created
log_info "Waiting for PipelineRun to be created..."

elapsed=0
interval=5
pipelinerun=""

while [ $elapsed -lt $TIMEOUT ]; do
    pipelinerun=$(kubectl get pipelinerun -n ${TEST_TENANT_NAME} --sort-by=.metadata.creationTimestamp -o name 2>/dev/null | tail -1)
    
    if [ -n "$pipelinerun" ]; then
        echo ""
        log_info "✓ PipelineRun found: $pipelinerun"
        break
    fi
    
    echo -n "."
    sleep $interval
    elapsed=$((elapsed + interval))
done

if [ -z "$pipelinerun" ]; then
    echo ""
    log_error "No PipelineRun found after ${TIMEOUT}s"
    
    echo ""
    log_info "Troubleshooting steps:"
    echo "  1. Check if webhook was delivered:"
    echo "     ./verify-webhook-delivery.sh"
    echo ""
    echo "  2. Check Lighthouse logs:"
    echo "     kubectl logs -n pipeline-system -l app=lighthouse --tail=50"
    echo ""
    echo "  3. Check if repository is in allowlist:"
    echo "     kubectl get configmap repo-allowlist -n pipeline-system -o yaml"
    echo ""
    echo "  4. Manually trigger pipeline:"
    echo "     ./trigger-test-pipeline.sh"
    
    exit 1
fi

# Get PipelineRun details
pipelinerun_name=${pipelinerun##*/}

echo ""
log_info "PipelineRun Details:"
kubectl get $pipelinerun -n ${TEST_TENANT_NAME} -o wide

# Verify service account
echo ""
log_info "Verifying service account..."

service_account=$(kubectl get $pipelinerun -n ${TEST_TENANT_NAME} -o jsonpath='{.spec.serviceAccountName}')

if [ "$service_account" == "pipeline-runner" ]; then
    log_info "✓ PipelineRun uses correct service account: $service_account"
else
    log_error "PipelineRun uses incorrect service account: $service_account (expected: pipeline-runner)"
    exit 1
fi

# Verify namespace
echo ""
log_info "Verifying namespace..."

namespace=$(kubectl get $pipelinerun -n ${TEST_TENANT_NAME} -o jsonpath='{.metadata.namespace}')

if [ "$namespace" == "${TEST_TENANT_NAME}" ]; then
    log_info "✓ PipelineRun is in correct namespace: $namespace"
else
    log_error "PipelineRun is in wrong namespace: $namespace (expected: ${TEST_TENANT_NAME})"
    exit 1
fi

# Show PipelineRun spec
echo ""
log_info "PipelineRun Specification:"
kubectl get $pipelinerun -n ${TEST_TENANT_NAME} -o jsonpath='{.spec}' | jq '.' 2>/dev/null || kubectl get $pipelinerun -n ${TEST_TENANT_NAME} -o jsonpath='{.spec}'

# Show PipelineRun status
echo ""
log_info "PipelineRun Status:"
kubectl get $pipelinerun -n ${TEST_TENANT_NAME} -o jsonpath='{.status}' | jq '.' 2>/dev/null || kubectl get $pipelinerun -n ${TEST_TENANT_NAME} -o jsonpath='{.status}'

# Check if PipelineRun is running or completed
echo ""
status=$(kubectl get $pipelinerun -n ${TEST_TENANT_NAME} -o jsonpath='{.status.conditions[?(@.type=="Succeeded")].status}' 2>/dev/null || echo "Unknown")
reason=$(kubectl get $pipelinerun -n ${TEST_TENANT_NAME} -o jsonpath='{.status.conditions[?(@.type=="Succeeded")].reason}' 2>/dev/null || echo "Unknown")

log_info "PipelineRun Status: $status ($reason)"

if [ "$status" == "True" ]; then
    log_info "✓ PipelineRun completed successfully"
elif [ "$status" == "False" ]; then
    log_error "PipelineRun failed"
    
    echo ""
    log_info "Showing recent logs:"
    kubectl logs -n ${TEST_TENANT_NAME} -l tekton.dev/pipelineRun=$pipelinerun_name --tail=50 || true
elif [ "$status" == "Unknown" ] || [ "$reason" == "Running" ] || [ "$reason" == "Started" ]; then
    log_info "PipelineRun is still running"
    
    echo ""
    log_info "Monitor progress:"
    echo "  kubectl get pipelinerun $pipelinerun_name -n ${TEST_TENANT_NAME} -w"
    echo ""
    log_info "View logs:"
    echo "  kubectl logs -n ${TEST_TENANT_NAME} -l tekton.dev/pipelineRun=$pipelinerun_name -f"
fi

echo ""
echo "========================================="
log_info "PipelineRun Verification Complete"
echo "========================================="
echo ""
log_info "Summary:"
echo "  ✓ PipelineRun created in tenant namespace"
echo "  ✓ PipelineRun uses tenant service account"
echo "  Status: $status ($reason)"
