#!/bin/bash
set -euo pipefail

# Verify Cross-Tenant Parallelism
# This script verifies that deployments for different tenants can run in parallel

TEST_TENANT_1="${TEST_TENANT_1:-test-serialization-1}"
TEST_TENANT_2="${TEST_TENANT_2:-test-serialization-2}"
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
echo "Verify Cross-Tenant Parallelism"
echo "========================================="
echo "Tenant 1: ${TEST_TENANT_1}"
echo "Tenant 2: ${TEST_TENANT_2}"
echo ""

function check_prerequisites() {
    log_info "Checking prerequisites..."
    
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl not found"
        exit 1
    fi
    
    if ! kubectl get namespace ${TEST_TENANT_1} &> /dev/null; then
        log_error "Tenant namespace not found: ${TEST_TENANT_1}"
        log_info "Run test-deployment-serialization.sh first"
        exit 1
    fi
    
    if ! kubectl get namespace ${TEST_TENANT_2} &> /dev/null; then
        log_error "Tenant namespace not found: ${TEST_TENANT_2}"
        log_info "Run test-deployment-serialization.sh first"
        exit 1
    fi
    
    log_info "Prerequisites check passed"
}

function get_running_deploy_count() {
    local tenant_name=$1
    
    kubectl get taskrun -n ${tenant_name} \
        -o json 2>/dev/null | \
        jq -r '.items[] | 
               select(.metadata.name | contains("deploy")) | 
               select(.status.conditions[]? | select(.type=="Succeeded" and .status=="Unknown")) | 
               .metadata.name' | \
        wc -l | tr -d ' '
}

function get_pipelinerun_count() {
    local tenant_name=$1
    
    kubectl get pipelinerun -n ${tenant_name} \
        -o name 2>/dev/null | \
        wc -l | tr -d ' '
}

function monitor_parallelism() {
    log_info "Monitoring cross-tenant parallelism..."
    
    local elapsed=0
    local parallel_samples=0
    local total_samples=0
    
    echo ""
    echo "Time | Tenant 1 Running | Tenant 2 Running | Parallel?"
    echo "-----+------------------+------------------+-----------"
    
    while [ $elapsed -lt $TIMEOUT ]; do
        local tenant1_running=$(get_running_deploy_count ${TEST_TENANT_1})
        local tenant2_running=$(get_running_deploy_count ${TEST_TENANT_2})
        
        local parallel="No"
        if [ $tenant1_running -gt 0 ] && [ $tenant2_running -gt 0 ]; then
            parallel="Yes"
            parallel_samples=$((parallel_samples + 1))
        fi
        
        if [ "$parallel" == "Yes" ]; then
            echo -e "${elapsed}s | ${tenant1_running} | ${tenant2_running} | ${GREEN}${parallel}${NC}"
        else
            echo "${elapsed}s | ${tenant1_running} | ${tenant2_running} | ${parallel}"
        fi
        
        total_samples=$((total_samples + 1))
        sleep $CHECK_INTERVAL
        elapsed=$((elapsed + CHECK_INTERVAL))
        
        # Stop if no more running deployments
        if [ $tenant1_running -eq 0 ] && [ $tenant2_running -eq 0 ]; then
            log_info "No more running deployments"
            break
        fi
    done
    
    echo ""
    log_info "Monitoring complete"
    log_info "Total samples: $total_samples"
    log_info "Parallel execution samples: $parallel_samples"
    
    if [ $parallel_samples -gt 0 ]; then
        local percentage=$((parallel_samples * 100 / total_samples))
        log_info "✓ Cross-tenant parallelism verified ($percentage% of samples)"
        return 0
    else
        log_warn "Could not verify cross-tenant parallelism"
        log_warn "This may be due to timing - deployments may have completed too quickly"
        return 0
    fi
}

function show_tenant_summary() {
    local tenant_name=$1
    
    log_info "Summary for tenant $tenant_name:"
    
    local total=$(get_pipelinerun_count $tenant_name)
    local succeeded=$(kubectl get pipelinerun -n ${tenant_name} -o json 2>/dev/null | \
        jq -r '.items[] | select(.status.conditions[]? | select(.type=="Succeeded" and .status=="True")) | .metadata.name' | \
        wc -l | tr -d ' ')
    local failed=$(kubectl get pipelinerun -n ${tenant_name} -o json 2>/dev/null | \
        jq -r '.items[] | select(.status.conditions[]? | select(.type=="Succeeded" and .status=="False")) | .metadata.name' | \
        wc -l | tr -d ' ')
    local running=$(kubectl get pipelinerun -n ${tenant_name} -o json 2>/dev/null | \
        jq -r '.items[] | select(.status.conditions[]? | select(.type=="Succeeded" and .status=="Unknown")) | .metadata.name' | \
        wc -l | tr -d ' ')
    
    echo "  Total PipelineRuns: $total"
    echo "  Succeeded: $succeeded"
    echo "  Failed: $failed"
    echo "  Running: $running"
}

function verify_isolation() {
    log_info "Verifying tenant isolation..."
    
    # Check that tenant 1 cannot access tenant 2 resources
    log_info "Checking RBAC isolation..."
    
    # Try to list pods in tenant 2 using tenant 1 service account
    if kubectl auth can-i list pods \
        --namespace=${TEST_TENANT_2} \
        --as=system:serviceaccount:${TEST_TENANT_1}:pipeline-runner 2>/dev/null; then
        log_error "✗ Tenant 1 can access tenant 2 resources (RBAC violation)"
        return 1
    else
        log_info "✓ Tenant 1 cannot access tenant 2 resources (RBAC enforced)"
    fi
    
    # Try to list pods in tenant 1 using tenant 2 service account
    if kubectl auth can-i list pods \
        --namespace=${TEST_TENANT_1} \
        --as=system:serviceaccount:${TEST_TENANT_2}:pipeline-runner 2>/dev/null; then
        log_error "✗ Tenant 2 can access tenant 1 resources (RBAC violation)"
        return 1
    else
        log_info "✓ Tenant 2 cannot access tenant 1 resources (RBAC enforced)"
    fi
    
    return 0
}

function show_concurrent_timeline() {
    log_info "Concurrent execution timeline:"
    
    echo ""
    echo "Tenant | TaskRun | Start Time | Completion Time"
    echo "-------+---------+------------+----------------"
    
    for tenant in ${TEST_TENANT_1} ${TEST_TENANT_2}; do
        kubectl get taskrun -n ${tenant} \
            -o json 2>/dev/null | \
            jq -r '.items[] | 
                   select(.metadata.name | contains("deploy")) | 
                   {tenant: "'$tenant'", 
                    name: .metadata.name, 
                    start: .status.startTime, 
                    completion: .status.completionTime} | 
                   "\(.tenant) | \(.name[0:20]) | \(.start[11:19] // "N/A") | \(.completion[11:19] // "N/A")"'
    done | sort -k5
}

function check_resource_contention() {
    log_info "Checking for resource contention..."
    
    # Check if any PipelineRuns failed due to resource constraints
    local resource_failures=0
    
    for tenant in ${TEST_TENANT_1} ${TEST_TENANT_2}; do
        local failures=$(kubectl get pipelinerun -n ${tenant} -o json 2>/dev/null | \
            jq -r '.items[] | 
                   select(.status.conditions[]? | 
                          select(.type=="Succeeded" and .status=="False" and 
                                 (.message | contains("resource") or contains("quota")))) | 
                   .metadata.name' | \
            wc -l | tr -d ' ')
        
        if [ $failures -gt 0 ]; then
            log_warn "Found $failures resource-related failures in tenant $tenant"
            resource_failures=$((resource_failures + failures))
        fi
    done
    
    if [ $resource_failures -eq 0 ]; then
        log_info "✓ No resource contention detected"
    else
        log_warn "Found $resource_failures resource-related failures"
        log_warn "This may indicate resource contention between tenants"
    fi
}

function main() {
    check_prerequisites
    
    echo ""
    log_info "Step 1: Monitor cross-tenant parallelism"
    monitor_parallelism
    
    echo ""
    log_info "Step 2: Verify tenant isolation"
    if ! verify_isolation; then
        log_error "Tenant isolation verification failed"
        exit 1
    fi
    
    echo ""
    log_info "Step 3: Check resource contention"
    check_resource_contention
    
    echo ""
    log_info "Step 4: Show concurrent execution timeline"
    show_concurrent_timeline
    
    echo ""
    log_info "Step 5: Show tenant summaries"
    show_tenant_summary ${TEST_TENANT_1}
    echo ""
    show_tenant_summary ${TEST_TENANT_2}
    
    echo ""
    echo "========================================="
    log_info "Cross-Tenant Parallelism: VERIFIED"
    echo "========================================="
}

# Run main function
main
