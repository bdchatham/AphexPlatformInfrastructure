#!/bin/bash
set -euo pipefail

# Verify Pipeline Task Execution
# This script verifies that all pipeline tasks (git-clone, cdktf-synth, cdktf-deploy) complete successfully

TEST_TENANT_NAME="${TEST_TENANT_NAME:-test-tenant}"
PIPELINERUN_NAME="${PIPELINERUN_NAME:-}"
TIMEOUT="${TIMEOUT:-600}"

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
echo "Verify Pipeline Task Execution"
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

# Wait for PipelineRun to complete
log_info "Waiting for PipelineRun to complete..."

elapsed=0
interval=10

while [ $elapsed -lt $TIMEOUT ]; do
    status=$(kubectl get pipelinerun $PIPELINERUN_NAME -n ${TEST_TENANT_NAME} -o jsonpath='{.status.conditions[?(@.type=="Succeeded")].status}' 2>/dev/null || echo "Unknown")
    reason=$(kubectl get pipelinerun $PIPELINERUN_NAME -n ${TEST_TENANT_NAME} -o jsonpath='{.status.conditions[?(@.type=="Succeeded")].reason}' 2>/dev/null || echo "Unknown")
    
    if [ "$status" == "True" ]; then
        echo ""
        log_info "✓ PipelineRun completed successfully"
        break
    elif [ "$status" == "False" ]; then
        echo ""
        log_error "PipelineRun failed: $reason"
        
        echo ""
        log_info "Showing failure logs:"
        kubectl logs -n ${TEST_TENANT_NAME} -l tekton.dev/pipelineRun=$PIPELINERUN_NAME --tail=100 || true
        
        exit 1
    fi
    
    echo -n "."
    sleep $interval
    elapsed=$((elapsed + interval))
done

if [ "$status" != "True" ]; then
    echo ""
    log_error "PipelineRun did not complete within ${TIMEOUT}s"
    log_info "Current status: $status ($reason)"
    exit 1
fi

# Get TaskRuns for this PipelineRun
echo ""
log_info "Getting TaskRuns for PipelineRun..."

taskruns=$(kubectl get taskrun -n ${TEST_TENANT_NAME} -l tekton.dev/pipelineRun=$PIPELINERUN_NAME -o name 2>/dev/null)

if [ -z "$taskruns" ]; then
    log_error "No TaskRuns found for PipelineRun"
    exit 1
fi

log_info "Found TaskRuns:"
echo "$taskruns"

# Verify each expected task
echo ""
log_info "Verifying task execution..."

expected_tasks=("clone" "synth" "deploy")
all_tasks_passed=true

for task in "${expected_tasks[@]}"; do
    echo ""
    log_info "Checking task: $task"
    
    # Find TaskRun for this task
    taskrun=$(echo "$taskruns" | grep "$task" | head -1 || echo "")
    
    if [ -z "$taskrun" ]; then
        log_error "✗ Task $task: TaskRun not found"
        all_tasks_passed=false
        continue
    fi
    
    taskrun_name=${taskrun##*/}
    
    # Get TaskRun status
    task_status=$(kubectl get $taskrun -n ${TEST_TENANT_NAME} -o jsonpath='{.status.conditions[?(@.type=="Succeeded")].status}' 2>/dev/null || echo "Unknown")
    task_reason=$(kubectl get $taskrun -n ${TEST_TENANT_NAME} -o jsonpath='{.status.conditions[?(@.type=="Succeeded")].reason}' 2>/dev/null || echo "Unknown")
    
    if [ "$task_status" == "True" ]; then
        log_info "✓ Task $task: Completed successfully"
        
        # Show task details
        start_time=$(kubectl get $taskrun -n ${TEST_TENANT_NAME} -o jsonpath='{.status.startTime}')
        completion_time=$(kubectl get $taskrun -n ${TEST_TENANT_NAME} -o jsonpath='{.status.completionTime}')
        
        echo "  Start: $start_time"
        echo "  Completion: $completion_time"
        
        # Show last few log lines
        echo "  Last 5 log lines:"
        kubectl logs $taskrun -n ${TEST_TENANT_NAME} --tail=5 2>/dev/null | sed 's/^/    /' || echo "    (logs not available)"
        
    elif [ "$task_status" == "False" ]; then
        log_error "✗ Task $task: Failed ($task_reason)"
        all_tasks_passed=false
        
        echo "  Showing failure logs:"
        kubectl logs $taskrun -n ${TEST_TENANT_NAME} --tail=20 2>/dev/null | sed 's/^/    /' || echo "    (logs not available)"
        
    else
        log_warn "? Task $task: Status unknown ($task_status, $task_reason)"
        all_tasks_passed=false
    fi
done

# Summary
echo ""
echo "========================================="
if [ "$all_tasks_passed" = true ]; then
    log_info "Pipeline Task Verification: PASSED"
    echo "========================================="
    echo ""
    log_info "All tasks completed successfully:"
    echo "  ✓ git-clone: Repository cloned at commit SHA"
    echo "  ✓ cdktf-synth: Terraform configuration generated"
    echo "  ✓ cdktf-deploy: Infrastructure deployed"
    
    exit 0
else
    log_error "Pipeline Task Verification: FAILED"
    echo "========================================="
    echo ""
    log_error "Some tasks did not complete successfully"
    
    echo ""
    log_info "View full logs:"
    echo "  kubectl logs -n ${TEST_TENANT_NAME} -l tekton.dev/pipelineRun=$PIPELINERUN_NAME"
    
    echo ""
    log_info "Describe PipelineRun:"
    echo "  kubectl describe pipelinerun $PIPELINERUN_NAME -n ${TEST_TENANT_NAME}"
    
    exit 1
fi
