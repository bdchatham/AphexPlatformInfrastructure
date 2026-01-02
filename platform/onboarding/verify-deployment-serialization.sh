#!/bin/bash
set -euo pipefail

# Verify Deployment Serialization
# This script verifies that only one cdktf-deploy task runs at a time per tenant
# and that subsequent deployments queue properly

TEST_TENANT_NAME="${TEST_TENANT_NAME:-test-serialization-1}"
TIMEOUT="${TIMEOUT:-300}"
CHECK_INTERVAL="${CHECK_INTERVAL:-5}"

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
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
echo "Verify Deployment Serialization"
echo "========================================="
echo "Tenant: ${TEST_TENANT_NAME}"
echo ""

function check_prerequisites() {
    log_info "Checking prerequisites..."
    
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl not found"
        exit 1
    fi
    
    if ! kubectl get namespace ${TEST_TENANT_NAME} &> /dev/null; then
        log_error "Tenant namespace not found: ${TEST_TENANT_NAME}"
        log_info "Run test-deployment-serialization.sh first"
        exit 1
    fi
    
    log_info "Prerequisites check passed"
}

function get_deploy_taskruns() {
    # Get all cdktf-deploy TaskRuns for this tenant
    kubectl get taskrun -n ${TEST_TENANT_NAME} \
        -o json 2>/dev/null | \
        jq -r '.items[] | select(.metadata.name | contains("deploy")) | 
               {name: .metadata.name, 
                status: (.status.conditions[]? | select(.type=="Succeeded") | .status),
                reason: (.status.conditions[]? | select(.type=="Succeeded") | .reason),
                startTime: .status.startTime,
                completionTime: .status.completionTime} | 
               @json'
}

function get_running_deploy_count() {
    kubectl get taskrun -n ${TEST_TENANT_NAME} \
        -o json 2>/dev/null | \
        jq -r '.items[] | 
               select(.metadata.name | contains("deploy")) | 
               select(.status.conditions[]? | select(.type=="Succeeded" and .status=="Unknown")) | 
               .metadata.name' | \
        wc -l | tr -d ' '
}

function get_pending_pipelineruns() {
    # Get PipelineRuns that haven't started yet
    kubectl get pipelinerun -n ${TEST_TENANT_NAME} \
        -o json 2>/dev/null | \
        jq -r '.items[] | 
               select(.status.conditions == null or (.status.conditions | length == 0)) | 
               .metadata.name' | \
        wc -l | tr -d ' '
}

function monitor_serialization() {
    log_info "Monitoring deployment serialization..."
    
    local elapsed=0
    local max_concurrent=0
    local samples=0
    local concurrent_violations=0
    
    echo ""
    echo "Time | Running | Pending | Max Concurrent"
    echo "-----+----------+---------+---------------"
    
    while [ $elapsed -lt $TIMEOUT ]; do
        local running=$(get_running_deploy_count)
        local pending=$(get_pending_pipelineruns)
        
        if [ $running -gt $max_concurrent ]; then
            max_concurrent=$running
        fi
        
        if [ $running -gt 1 ]; then
            concurrent_violations=$((concurrent_violations + 1))
            echo -e "${elapsed}s | ${RED}${running}${NC} | ${pending} | ${max_concurrent}"
        else
            echo "${elapsed}s | ${running} | ${pending} | ${max_concurrent}"
        fi
        
        samples=$((samples + 1))
        sleep $CHECK_INTERVAL
        elapsed=$((elapsed + CHECK_INTERVAL))
        
        # Stop if no more running or pending
        if [ $running -eq 0 ] && [ $pending -eq 0 ]; then
            log_info "No more running or pending deployments"
            break
        fi
    done
    
    echo ""
    log_info "Monitoring complete"
    log_info "Samples taken: $samples"
    log_info "Maximum concurrent cdktf-deploy tasks: $max_concurrent"
    log_info "Concurrent violations: $concurrent_violations"
    
    if [ $max_concurrent -le 1 ]; then
        log_info "✓ Deployments are properly serialized"
        return 0
    else
        log_error "✗ Deployments are NOT properly serialized"
        log_error "Found $max_concurrent concurrent cdktf-deploy tasks"
        return 1
    fi
}

function show_deployment_timeline() {
    log_info "Deployment timeline:"
    
    echo ""
    echo "TaskRun | Status | Start Time | Completion Time"
    echo "--------+--------+------------+----------------"
    
    get_deploy_taskruns | while read -r taskrun; do
        local name=$(echo $taskrun | jq -r '.name' | cut -d'-' -f1-3)
        local status=$(echo $taskrun | jq -r '.status // "Pending"')
        local reason=$(echo $taskrun | jq -r '.reason // "N/A"')
        local start=$(echo $taskrun | jq -r '.startTime // "N/A"' | cut -d'T' -f2 | cut -d'Z' -f1)
        local completion=$(echo $taskrun | jq -r '.completionTime // "N/A"' | cut -d'T' -f2 | cut -d'Z' -f1)
        
        echo "$name | $status ($reason) | $start | $completion"
    done
}

function verify_queuing() {
    log_info "Verifying deployment queuing..."
    
    # Check if there are any pending PipelineRuns
    local pending=$(get_pending_pipelineruns)
    
    if [ $pending -gt 0 ]; then
        log_info "✓ Found $pending pending PipelineRuns (queued)"
        
        # Show pending PipelineRuns
        echo ""
        log_info "Pending PipelineRuns:"
        kubectl get pipelinerun -n ${TEST_TENANT_NAME} \
            -o json 2>/dev/null | \
            jq -r '.items[] | 
                   select(.status.conditions == null or (.status.conditions | length == 0)) | 
                   .metadata.name'
    else
        log_info "No pending PipelineRuns found (all may have started)"
    fi
}

function check_state_lock_conflicts() {
    log_info "Checking for Terraform state lock conflicts..."
    
    # Look for state lock errors in TaskRun logs
    local lock_errors=0
    
    kubectl get taskrun -n ${TEST_TENANT_NAME} \
        -o json 2>/dev/null | \
        jq -r '.items[] | select(.metadata.name | contains("deploy")) | .metadata.name' | \
        while read -r taskrun; do
            if kubectl logs -n ${TEST_TENANT_NAME} $taskrun 2>/dev/null | grep -q "state lock"; then
                log_warn "Found state lock reference in $taskrun"
                lock_errors=$((lock_errors + 1))
            fi
        done
    
    if [ $lock_errors -eq 0 ]; then
        log_info "✓ No state lock conflicts detected"
    else
        log_warn "Found $lock_errors TaskRuns with state lock references"
    fi
}

function show_pipelinerun_summary() {
    log_info "PipelineRun summary:"
    
    local total=$(kubectl get pipelinerun -n ${TEST_TENANT_NAME} -o name 2>/dev/null | wc -l | tr -d ' ')
    local succeeded=$(kubectl get pipelinerun -n ${TEST_TENANT_NAME} -o json 2>/dev/null | \
        jq -r '.items[] | select(.status.conditions[]? | select(.type=="Succeeded" and .status=="True")) | .metadata.name' | \
        wc -l | tr -d ' ')
    local failed=$(kubectl get pipelinerun -n ${TEST_TENANT_NAME} -o json 2>/dev/null | \
        jq -r '.items[] | select(.status.conditions[]? | select(.type=="Succeeded" and .status=="False")) | .metadata.name' | \
        wc -l | tr -d ' ')
    local running=$(kubectl get pipelinerun -n ${TEST_TENANT_NAME} -o json 2>/dev/null | \
        jq -r '.items[] | select(.status.conditions[]? | select(.type=="Succeeded" and .status=="Unknown")) | .metadata.name' | \
        wc -l | tr -d ' ')
    
    echo "  Total: $total"
    echo "  Succeeded: $succeeded"
    echo "  Failed: $failed"
    echo "  Running: $running"
}

function main() {
    check_prerequisites
    
    echo ""
    log_info "Step 1: Monitor serialization"
    if ! monitor_serialization; then
        log_error "Serialization verification failed"
        
        echo ""
        show_deployment_timeline
        
        echo ""
        show_pipelinerun_summary
        
        exit 1
    fi
    
    echo ""
    log_info "Step 2: Verify queuing behavior"
    verify_queuing
    
    echo ""
    log_info "Step 3: Check for state lock conflicts"
    check_state_lock_conflicts
    
    echo ""
    log_info "Step 4: Show deployment timeline"
    show_deployment_timeline
    
    echo ""
    log_info "Step 5: Show PipelineRun summary"
    show_pipelinerun_summary
    
    echo ""
    echo "========================================="
    log_info "Serialization Verification: PASSED"
    echo "========================================="
}

# Run main function
main
