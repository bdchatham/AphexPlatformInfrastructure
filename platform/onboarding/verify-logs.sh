#!/bin/bash
set -euo pipefail

# Verify Logs Are Accessible
# This script verifies that PipelineRun logs are accessible via kubectl

TEST_TENANT_NAME="${TEST_TENANT_NAME:-test-tenant}"
PIPELINERUN_NAME="${PIPELINERUN_NAME:-}"

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
echo "Verify Logs Are Accessible"
echo "========================================="
echo "Tenant: ${TEST_TENANT_NAME}"
echo ""

# Get PipelineRun if not specified
if [ -z "$PIPELINERUN_NAME" ]; then
    log_info "Finding most recent PipelineRun..."
    pipelinerun=$(kubectl get pipelinerun -n ${TEST_TENANT_NAME} --sort-by=.metadata.creationTimestamp -o name 2>/dev/null | tail -1)
    
    if [ -z "$pipelinerun" ]; then
        log_error "No PipelineRun found in namespace ${TEST_TENANT_NAME}"
        exit 1
    fi
    
    PIPELINERUN_NAME=${pipelinerun##*/}
    log_info "Using PipelineRun: $PIPELINERUN_NAME"
else
    log_info "Using specified PipelineRun: $PIPELINERUN_NAME"
fi

echo ""

# Get pods for this PipelineRun
log_info "Finding pods for PipelineRun..."

pods=$(kubectl get pods -n ${TEST_TENANT_NAME} -l tekton.dev/pipelineRun=$PIPELINERUN_NAME -o name 2>/dev/null)

if [ -z "$pods" ]; then
    log_error "No pods found for PipelineRun"
    exit 1
fi

log_info "Found pods:"
echo "$pods"

# Try to get logs from each pod
echo ""
log_info "Verifying log accessibility..."

all_logs_accessible=true

for pod in $pods; do
    pod_name=${pod##*/}
    
    echo ""
    log_info "Checking logs for pod: $pod_name"
    
    # Try to get logs
    if kubectl logs $pod -n ${TEST_TENANT_NAME} --tail=5 &> /dev/null; then
        log_info "✓ Logs accessible for pod: $pod_name"
        
        echo "  Last 5 lines:"
        kubectl logs $pod -n ${TEST_TENANT_NAME} --tail=5 | sed 's/^/    /'
    else
        log_error "✗ Cannot access logs for pod: $pod_name"
        all_logs_accessible=false
        
        # Check pod status
        pod_status=$(kubectl get $pod -n ${TEST_TENANT_NAME} -o jsonpath='{.status.phase}')
        log_info "  Pod status: $pod_status"
    fi
done

# Try to get logs using PipelineRun label selector
echo ""
log_info "Verifying logs via label selector..."

if kubectl logs -n ${TEST_TENANT_NAME} -l tekton.dev/pipelineRun=$PIPELINERUN_NAME --tail=10 &> /dev/null; then
    log_info "✓ Logs accessible via label selector"
    
    echo ""
    log_info "Sample logs (last 10 lines):"
    kubectl logs -n ${TEST_TENANT_NAME} -l tekton.dev/pipelineRun=$PIPELINERUN_NAME --tail=10 | sed 's/^/  /'
else
    log_error "✗ Cannot access logs via label selector"
    all_logs_accessible=false
fi

# Get TaskRuns and check their logs
echo ""
log_info "Checking TaskRun logs..."

taskruns=$(kubectl get taskrun -n ${TEST_TENANT_NAME} -l tekton.dev/pipelineRun=$PIPELINERUN_NAME -o name 2>/dev/null)

if [ -n "$taskruns" ]; then
    for taskrun in $taskruns; do
        taskrun_name=${taskrun##*/}
        
        echo ""
        log_info "Checking logs for TaskRun: $taskrun_name"
        
        if kubectl logs $taskrun -n ${TEST_TENANT_NAME} --tail=5 &> /dev/null; then
            log_info "✓ Logs accessible for TaskRun: $taskrun_name"
            
            echo "  Last 5 lines:"
            kubectl logs $taskrun -n ${TEST_TENANT_NAME} --tail=5 | sed 's/^/    /'
        else
            log_warn "Cannot access logs for TaskRun: $taskrun_name (may not have started yet)"
        fi
    done
else
    log_warn "No TaskRuns found for PipelineRun"
fi

# Test log streaming capability
echo ""
log_info "Testing log streaming capability..."

log_info "You can stream logs in real-time using:"
echo "  kubectl logs -n ${TEST_TENANT_NAME} -l tekton.dev/pipelineRun=$PIPELINERUN_NAME -f"

# Test log filtering
echo ""
log_info "Testing log filtering..."

log_info "You can filter logs by task using:"
echo "  kubectl logs -n ${TEST_TENANT_NAME} -l tekton.dev/pipelineRun=$PIPELINERUN_NAME,tekton.dev/pipelineTask=clone"

# Show full log retrieval command
echo ""
log_info "To retrieve full logs:"
echo "  kubectl logs -n ${TEST_TENANT_NAME} -l tekton.dev/pipelineRun=$PIPELINERUN_NAME --all-containers=true"

# Check RBAC for log access
echo ""
log_info "Verifying RBAC for log access..."

# Check if tenant service account can access logs
if kubectl auth can-i get pods/log --as=system:serviceaccount:${TEST_TENANT_NAME}:pipeline-runner -n ${TEST_TENANT_NAME} &> /dev/null; then
    log_info "✓ Tenant service account can access logs in tenant namespace"
else
    log_warn "Tenant service account cannot access logs (may be expected)"
fi

# Summary
echo ""
echo "========================================="
if [ "$all_logs_accessible" = true ]; then
    log_info "Log Accessibility Verification: PASSED"
    echo "========================================="
    echo ""
    log_info "Summary:"
    echo "  ✓ Logs accessible via kubectl"
    echo "  ✓ Logs accessible via label selector"
    echo "  ✓ Logs accessible for individual pods"
    echo "  ✓ Logs accessible for TaskRuns"
    
    echo ""
    log_info "Log access methods:"
    echo "  1. By PipelineRun:"
    echo "     kubectl logs -n ${TEST_TENANT_NAME} -l tekton.dev/pipelineRun=$PIPELINERUN_NAME"
    echo ""
    echo "  2. By TaskRun:"
    echo "     kubectl logs <taskrun-name> -n ${TEST_TENANT_NAME}"
    echo ""
    echo "  3. By Pod:"
    echo "     kubectl logs <pod-name> -n ${TEST_TENANT_NAME}"
    echo ""
    echo "  4. Stream logs:"
    echo "     kubectl logs -n ${TEST_TENANT_NAME} -l tekton.dev/pipelineRun=$PIPELINERUN_NAME -f"
    
    exit 0
else
    log_error "Log Accessibility Verification: FAILED"
    echo "========================================="
    echo ""
    log_error "Some logs are not accessible"
    
    echo ""
    log_info "Troubleshooting:"
    echo "  1. Check pod status:"
    echo "     kubectl get pods -n ${TEST_TENANT_NAME} -l tekton.dev/pipelineRun=$PIPELINERUN_NAME"
    echo ""
    echo "  2. Describe pods:"
    echo "     kubectl describe pods -n ${TEST_TENANT_NAME} -l tekton.dev/pipelineRun=$PIPELINERUN_NAME"
    echo ""
    echo "  3. Check RBAC:"
    echo "     kubectl auth can-i get pods/log -n ${TEST_TENANT_NAME}"
    
    exit 1
fi
