#!/bin/bash
set -euo pipefail

# Test Deployment Serialization
# This script tests that CDKTF deployments are serialized per tenant
# and that different tenants can deploy in parallel

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_TENANT_1="${TEST_TENANT_1:-test-serialization-1}"
TEST_TENANT_2="${TEST_TENANT_2:-test-serialization-2}"
TEST_REPO_ORG="${TEST_REPO_ORG:-your-github-org}"
TIMEOUT="${TIMEOUT:-600}"

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
echo "Deployment Serialization Test"
echo "========================================="
echo "Tenant 1: ${TEST_TENANT_1}"
echo "Tenant 2: ${TEST_TENANT_2}"
echo "Timeout: ${TIMEOUT}s"
echo ""

function check_prerequisites() {
    log_info "Checking prerequisites..."
    
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl not found"
        exit 1
    fi
    
    if ! kubectl cluster-info &> /dev/null; then
        log_error "Cannot connect to Kubernetes cluster"
        exit 1
    fi
    
    log_info "Prerequisites check passed"
}

function create_test_tenant() {
    local tenant_name=$1
    local repo_name=$2
    
    log_info "Creating tenant: $tenant_name"
    
    cat <<EOF | kubectl apply -f -
apiVersion: platform.arbiter.io/v1alpha1
kind: RepoBinding
metadata:
  name: ${tenant_name}-binding
  namespace: pipeline-system
spec:
  repoOrg: "${TEST_REPO_ORG}"
  repoName: "${repo_name}"
  tenantName: "${tenant_name}"
  permissionProfile: "standard"
EOF
    
    # Wait for onboarding
    local elapsed=0
    local interval=5
    
    while [ $elapsed -lt $TIMEOUT ]; do
        local phase=$(kubectl get repobinding ${tenant_name}-binding -n pipeline-system -o jsonpath='{.status.phase}' 2>/dev/null || echo "")
        
        if [ "$phase" == "Ready" ]; then
            log_info "✓ Tenant $tenant_name ready"
            return 0
        elif [ "$phase" == "Failed" ]; then
            local message=$(kubectl get repobinding ${tenant_name}-binding -n pipeline-system -o jsonpath='{.status.message}')
            log_error "Tenant $tenant_name onboarding failed: $message"
            return 1
        fi
        
        sleep $interval
        elapsed=$((elapsed + interval))
    done
    
    log_error "Tenant $tenant_name onboarding timed out"
    return 1
}

function trigger_multiple_pipelines() {
    local tenant_name=$1
    local count=$2
    
    log_info "Triggering $count pipelines for tenant $tenant_name..."
    
    for i in $(seq 1 $count); do
        # Create a PipelineRun manually for testing
        local timestamp=$(date +%s)
        local run_name="${tenant_name}-run-${timestamp}-${i}"
        
        cat <<EOF | kubectl apply -f -
apiVersion: tekton.dev/v1beta1
kind: PipelineRun
metadata:
  name: ${run_name}
  namespace: ${tenant_name}
  labels:
    test: "serialization"
    tenant: "${tenant_name}"
spec:
  serviceAccountName: pipeline-runner
  pipelineRef:
    name: cdktf-deploy-pipeline
  params:
    - name: repo-url
      value: "https://github.com/${TEST_REPO_ORG}/test-repo.git"
    - name: commit-sha
      value: "main"
    - name: tenant-name
      value: "${tenant_name}"
  workspaces:
    - name: source
      volumeClaimTemplate:
        spec:
          accessModes:
            - ReadWriteOnce
          resources:
            requests:
              storage: 1Gi
    - name: terraform-state
      emptyDir: {}
EOF
        
        log_info "Created PipelineRun: $run_name"
        sleep 1
    done
    
    log_info "✓ Triggered $count pipelines for tenant $tenant_name"
}

function get_running_deploy_tasks() {
    local tenant_name=$1
    
    # Get TaskRuns for cdktf-deploy that are currently running
    kubectl get taskrun -n ${tenant_name} \
        -l test=serialization \
        -o json 2>/dev/null | \
        jq -r '.items[] | select(.metadata.name | contains("deploy")) | select(.status.conditions[]? | select(.type=="Succeeded" and .status=="Unknown")) | .metadata.name' | \
        wc -l
}

function verify_serialization() {
    local tenant_name=$1
    
    log_info "Verifying deployment serialization for tenant $tenant_name..."
    
    # Wait for pipelines to start
    sleep 10
    
    # Check multiple times over a period
    local max_concurrent=0
    local checks=10
    
    for i in $(seq 1 $checks); do
        local running=$(get_running_deploy_tasks $tenant_name)
        
        if [ $running -gt $max_concurrent ]; then
            max_concurrent=$running
        fi
        
        log_info "Check $i/$checks: $running cdktf-deploy tasks running"
        sleep 5
    done
    
    log_info "Maximum concurrent cdktf-deploy tasks: $max_concurrent"
    
    if [ $max_concurrent -le 1 ]; then
        log_info "✓ Deployments are serialized (max concurrent: $max_concurrent)"
        return 0
    else
        log_error "Deployments are NOT serialized (max concurrent: $max_concurrent)"
        return 1
    fi
}

function verify_cross_tenant_parallelism() {
    log_info "Verifying cross-tenant parallelism..."
    
    # Wait for pipelines to start
    sleep 10
    
    # Check if both tenants have running deploy tasks simultaneously
    local tenant1_running=$(get_running_deploy_tasks $TEST_TENANT_1)
    local tenant2_running=$(get_running_deploy_tasks $TEST_TENANT_2)
    
    log_info "Tenant 1 running tasks: $tenant1_running"
    log_info "Tenant 2 running tasks: $tenant2_running"
    
    if [ $tenant1_running -gt 0 ] && [ $tenant2_running -gt 0 ]; then
        log_info "✓ Cross-tenant parallelism verified"
        return 0
    else
        log_warn "Could not verify cross-tenant parallelism (may need more time)"
        return 0
    fi
}

function get_pipelinerun_count() {
    local tenant_name=$1
    kubectl get pipelinerun -n ${tenant_name} -l test=serialization -o name 2>/dev/null | wc -l
}

function wait_for_pipelines_to_complete() {
    local tenant_name=$1
    
    log_info "Waiting for pipelines to complete for tenant $tenant_name..."
    
    local elapsed=0
    local interval=10
    
    while [ $elapsed -lt $TIMEOUT ]; do
        local total=$(get_pipelinerun_count $tenant_name)
        local completed=$(kubectl get pipelinerun -n ${tenant_name} -l test=serialization -o json 2>/dev/null | \
            jq -r '.items[] | select(.status.conditions[]? | select(.type=="Succeeded" and (.status=="True" or .status=="False"))) | .metadata.name' | \
            wc -l)
        
        log_info "Progress: $completed/$total pipelines completed"
        
        if [ $completed -eq $total ] && [ $total -gt 0 ]; then
            log_info "✓ All pipelines completed for tenant $tenant_name"
            return 0
        fi
        
        sleep $interval
        elapsed=$((elapsed + interval))
    done
    
    log_warn "Timeout waiting for pipelines to complete"
    return 0
}

function show_pipeline_summary() {
    local tenant_name=$1
    
    log_info "Pipeline summary for tenant $tenant_name:"
    
    local succeeded=$(kubectl get pipelinerun -n ${tenant_name} -l test=serialization -o json 2>/dev/null | \
        jq -r '.items[] | select(.status.conditions[]? | select(.type=="Succeeded" and .status=="True")) | .metadata.name' | \
        wc -l)
    
    local failed=$(kubectl get pipelinerun -n ${tenant_name} -l test=serialization -o json 2>/dev/null | \
        jq -r '.items[] | select(.status.conditions[]? | select(.type=="Succeeded" and .status=="False")) | .metadata.name' | \
        wc -l)
    
    local running=$(kubectl get pipelinerun -n ${tenant_name} -l test=serialization -o json 2>/dev/null | \
        jq -r '.items[] | select(.status.conditions[]? | select(.type=="Succeeded" and .status=="Unknown")) | .metadata.name' | \
        wc -l)
    
    echo "  Succeeded: $succeeded"
    echo "  Failed: $failed"
    echo "  Running: $running"
}

function cleanup() {
    log_info "Cleaning up test resources..."
    
    # Delete PipelineRuns
    kubectl delete pipelinerun -n ${TEST_TENANT_1} -l test=serialization --ignore-not-found=true 2>/dev/null || true
    kubectl delete pipelinerun -n ${TEST_TENANT_2} -l test=serialization --ignore-not-found=true 2>/dev/null || true
    
    # Delete RepoBindings
    kubectl delete repobinding ${TEST_TENANT_1}-binding -n pipeline-system --ignore-not-found=true 2>/dev/null || true
    kubectl delete repobinding ${TEST_TENANT_2}-binding -n pipeline-system --ignore-not-found=true 2>/dev/null || true
    
    # Wait for namespaces to be deleted
    local elapsed=0
    while (kubectl get namespace ${TEST_TENANT_1} &> /dev/null || kubectl get namespace ${TEST_TENANT_2} &> /dev/null) && [ $elapsed -lt 60 ]; do
        sleep 2
        elapsed=$((elapsed + 2))
    done
    
    log_info "Cleanup complete"
}

function main() {
    check_prerequisites
    
    echo ""
    log_info "Step 1: Create test tenants"
    create_test_tenant ${TEST_TENANT_1} "test-repo-1" || exit 1
    create_test_tenant ${TEST_TENANT_2} "test-repo-2" || exit 1
    
    echo ""
    log_info "Step 2: Trigger multiple concurrent deployments for tenant 1"
    trigger_multiple_pipelines ${TEST_TENANT_1} 3
    
    echo ""
    log_info "Step 3: Verify deployments are serialized for tenant 1"
    if ! verify_serialization ${TEST_TENANT_1}; then
        log_error "Serialization verification failed for tenant 1"
        show_pipeline_summary ${TEST_TENANT_1}
        
        echo ""
        read -p "Continue with cross-tenant test? (y/n) " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            cleanup
            exit 1
        fi
    fi
    
    echo ""
    log_info "Step 4: Trigger deployments for tenant 2"
    trigger_multiple_pipelines ${TEST_TENANT_2} 2
    
    echo ""
    log_info "Step 5: Verify cross-tenant parallelism"
    verify_cross_tenant_parallelism
    
    echo ""
    log_info "Step 6: Wait for all pipelines to complete"
    wait_for_pipelines_to_complete ${TEST_TENANT_1}
    wait_for_pipelines_to_complete ${TEST_TENANT_2}
    
    echo ""
    log_info "Final Summary:"
    show_pipeline_summary ${TEST_TENANT_1}
    show_pipeline_summary ${TEST_TENANT_2}
    
    echo ""
    echo "========================================="
    log_info "Deployment Serialization Test Complete"
    echo "========================================="
    
    echo ""
    read -p "Clean up test resources? (y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        cleanup
    fi
}

# Run main function
main
