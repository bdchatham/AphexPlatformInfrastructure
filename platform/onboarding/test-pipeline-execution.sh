#!/bin/bash
set -euo pipefail

# Test Pipeline Execution Workflow
# This script tests the complete pipeline execution workflow including:
# - Repository onboarding
# - Pipeline triggering
# - PipelineRun creation and execution
# - Terraform state management
# - Log accessibility

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_REPO_ORG="${TEST_REPO_ORG:-your-github-org}"
TEST_REPO_NAME="${TEST_REPO_NAME:-test-pipeline-execution}"
TEST_TENANT_NAME="${TEST_TENANT_NAME:-test-tenant}"
TIMEOUT="${TIMEOUT:-600}"

echo "========================================="
echo "Pipeline Execution Workflow Test"
echo "========================================="
echo "Repository: ${TEST_REPO_ORG}/${TEST_REPO_NAME}"
echo "Tenant: ${TEST_TENANT_NAME}"
echo "Timeout: ${TIMEOUT}s"
echo ""

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

function log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

function log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

function log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

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

function create_repobinding() {
    log_info "Creating RepoBinding for test repository..."
    
    cat <<EOF | kubectl apply -f -
apiVersion: platform.arbiter.io/v1alpha1
kind: RepoBinding
metadata:
  name: ${TEST_TENANT_NAME}-binding
  namespace: pipeline-system
spec:
  repoOrg: "${TEST_REPO_ORG}"
  repoName: "${TEST_REPO_NAME}"
  tenantName: "${TEST_TENANT_NAME}"
  permissionProfile: "standard"
EOF
    
    log_info "RepoBinding created"
}

function wait_for_onboarding() {
    log_info "Waiting for onboarding to complete..."
    
    local elapsed=0
    local interval=5
    
    while [ $elapsed -lt $TIMEOUT ]; do
        local phase=$(kubectl get repobinding ${TEST_TENANT_NAME}-binding -n pipeline-system -o jsonpath='{.status.phase}' 2>/dev/null || echo "")
        
        if [ "$phase" == "Ready" ]; then
            log_info "Onboarding completed successfully"
            return 0
        elif [ "$phase" == "Failed" ]; then
            local message=$(kubectl get repobinding ${TEST_TENANT_NAME}-binding -n pipeline-system -o jsonpath='{.status.message}')
            log_error "Onboarding failed: $message"
            return 1
        fi
        
        echo -n "."
        sleep $interval
        elapsed=$((elapsed + interval))
    done
    
    log_error "Onboarding timed out after ${TIMEOUT}s"
    return 1
}

function verify_tenant_resources() {
    log_info "Verifying tenant resources..."
    
    # Check namespace
    if ! kubectl get namespace ${TEST_TENANT_NAME} &> /dev/null; then
        log_error "Tenant namespace not found"
        return 1
    fi
    log_info "✓ Namespace exists"
    
    # Check service account
    if ! kubectl get serviceaccount pipeline-runner -n ${TEST_TENANT_NAME} &> /dev/null; then
        log_error "Service account not found"
        return 1
    fi
    log_info "✓ Service account exists"
    
    # Check RBAC
    if ! kubectl get role pipeline-runner -n ${TEST_TENANT_NAME} &> /dev/null; then
        log_error "Role not found"
        return 1
    fi
    log_info "✓ Role exists"
    
    if ! kubectl get rolebinding pipeline-runner -n ${TEST_TENANT_NAME} &> /dev/null; then
        log_error "RoleBinding not found"
        return 1
    fi
    log_info "✓ RoleBinding exists"
    
    # Check resource limits
    if ! kubectl get resourcequota tenant-quota -n ${TEST_TENANT_NAME} &> /dev/null; then
        log_error "ResourceQuota not found"
        return 1
    fi
    log_info "✓ ResourceQuota exists"
    
    if ! kubectl get limitrange tenant-limits -n ${TEST_TENANT_NAME} &> /dev/null; then
        log_error "LimitRange not found"
        return 1
    fi
    log_info "✓ LimitRange exists"
    
    # Check network policy
    if ! kubectl get networkpolicy tenant-isolation -n ${TEST_TENANT_NAME} &> /dev/null; then
        log_error "NetworkPolicy not found"
        return 1
    fi
    log_info "✓ NetworkPolicy exists"
    
    # Check Terraform backend secret
    if ! kubectl get secret terraform-backend-config -n ${TEST_TENANT_NAME} &> /dev/null; then
        log_error "Terraform backend secret not found"
        return 1
    fi
    log_info "✓ Terraform backend secret exists"
    
    log_info "All tenant resources verified"
}

function verify_allowlist() {
    log_info "Verifying repository is in allowlist..."
    
    local allowlist=$(kubectl get configmap repo-allowlist -n pipeline-system -o jsonpath='{.data.allowlist\.yaml}' 2>/dev/null || echo "")
    
    if echo "$allowlist" | grep -q "${TEST_REPO_ORG}/${TEST_REPO_NAME}"; then
        log_info "✓ Repository found in allowlist"
        return 0
    else
        log_error "Repository not found in allowlist"
        return 1
    fi
}

function simulate_webhook() {
    log_warn "Manual step required: Trigger pipeline by merging to main branch"
    log_info "You can simulate this by:"
    log_info "1. Push a commit to the main branch of ${TEST_REPO_ORG}/${TEST_REPO_NAME}"
    log_info "2. Or manually create a PipelineRun for testing"
    echo ""
    read -p "Press Enter once you've triggered the pipeline..."
}

function wait_for_pipelinerun() {
    log_info "Waiting for PipelineRun to be created..."
    
    local elapsed=0
    local interval=5
    
    while [ $elapsed -lt $TIMEOUT ]; do
        local pipelinerun=$(kubectl get pipelinerun -n ${TEST_TENANT_NAME} --sort-by=.metadata.creationTimestamp -o name 2>/dev/null | tail -1)
        
        if [ -n "$pipelinerun" ]; then
            log_info "PipelineRun found: $pipelinerun"
            echo "$pipelinerun"
            return 0
        fi
        
        echo -n "."
        sleep $interval
        elapsed=$((elapsed + interval))
    done
    
    log_error "No PipelineRun found after ${TIMEOUT}s"
    return 1
}

function verify_pipelinerun_identity() {
    local pipelinerun=$1
    log_info "Verifying PipelineRun uses tenant service account..."
    
    local sa=$(kubectl get $pipelinerun -n ${TEST_TENANT_NAME} -o jsonpath='{.spec.serviceAccountName}')
    
    if [ "$sa" == "pipeline-runner" ]; then
        log_info "✓ PipelineRun uses correct service account: $sa"
        return 0
    else
        log_error "PipelineRun uses incorrect service account: $sa (expected: pipeline-runner)"
        return 1
    fi
}

function wait_for_pipeline_completion() {
    local pipelinerun=$1
    log_info "Waiting for pipeline to complete..."
    
    local elapsed=0
    local interval=10
    
    while [ $elapsed -lt $TIMEOUT ]; do
        local status=$(kubectl get $pipelinerun -n ${TEST_TENANT_NAME} -o jsonpath='{.status.conditions[?(@.type=="Succeeded")].status}' 2>/dev/null || echo "")
        local reason=$(kubectl get $pipelinerun -n ${TEST_TENANT_NAME} -o jsonpath='{.status.conditions[?(@.type=="Succeeded")].reason}' 2>/dev/null || echo "")
        
        if [ "$status" == "True" ]; then
            log_info "Pipeline completed successfully"
            return 0
        elif [ "$status" == "False" ]; then
            log_error "Pipeline failed: $reason"
            return 1
        fi
        
        echo -n "."
        sleep $interval
        elapsed=$((elapsed + interval))
    done
    
    log_error "Pipeline timed out after ${TIMEOUT}s"
    return 1
}

function verify_task_execution() {
    local pipelinerun=$1
    log_info "Verifying task execution..."
    
    # Get TaskRuns for this PipelineRun
    local taskruns=$(kubectl get taskrun -n ${TEST_TENANT_NAME} -l tekton.dev/pipelineRun=${pipelinerun##*/} -o name 2>/dev/null)
    
    if [ -z "$taskruns" ]; then
        log_error "No TaskRuns found for PipelineRun"
        return 1
    fi
    
    log_info "Found TaskRuns:"
    echo "$taskruns"
    
    # Check each task
    local expected_tasks=("clone" "synth" "deploy")
    for task in "${expected_tasks[@]}"; do
        if echo "$taskruns" | grep -q "$task"; then
            local taskrun=$(echo "$taskruns" | grep "$task" | head -1)
            local status=$(kubectl get $taskrun -n ${TEST_TENANT_NAME} -o jsonpath='{.status.conditions[?(@.type=="Succeeded")].status}' 2>/dev/null || echo "")
            
            if [ "$status" == "True" ]; then
                log_info "✓ Task $task completed successfully"
            else
                log_warn "Task $task status: $status"
            fi
        else
            log_warn "Task $task not found in TaskRuns"
        fi
    done
}

function verify_terraform_state() {
    log_info "Verifying Terraform state..."
    
    # Check if state secret exists (for Kubernetes backend)
    local state_secret=$(kubectl get secret -n ${TEST_TENANT_NAME} -l tfstate=true -o name 2>/dev/null | head -1)
    
    if [ -n "$state_secret" ]; then
        log_info "✓ Terraform state found: $state_secret"
        
        # Verify state is isolated to tenant
        local namespace=$(kubectl get $state_secret -n ${TEST_TENANT_NAME} -o jsonpath='{.metadata.namespace}')
        if [ "$namespace" == "${TEST_TENANT_NAME}" ]; then
            log_info "✓ State is isolated to tenant namespace"
        else
            log_error "State is in wrong namespace: $namespace"
            return 1
        fi
    else
        log_warn "Terraform state secret not found (may be using different backend)"
    fi
}

function verify_logs() {
    local pipelinerun=$1
    log_info "Verifying logs are accessible..."
    
    # Try to get logs from PipelineRun
    if kubectl logs -n ${TEST_TENANT_NAME} -l tekton.dev/pipelineRun=${pipelinerun##*/} --tail=10 &> /dev/null; then
        log_info "✓ Logs are accessible via kubectl"
        
        echo ""
        log_info "Sample logs (last 10 lines):"
        kubectl logs -n ${TEST_TENANT_NAME} -l tekton.dev/pipelineRun=${pipelinerun##*/} --tail=10
    else
        log_error "Cannot access logs"
        return 1
    fi
}

function cleanup() {
    log_info "Cleaning up test resources..."
    
    # Delete RepoBinding (this should trigger cleanup of tenant resources)
    kubectl delete repobinding ${TEST_TENANT_NAME}-binding -n pipeline-system --ignore-not-found=true
    
    # Wait for namespace to be deleted
    local elapsed=0
    while kubectl get namespace ${TEST_TENANT_NAME} &> /dev/null && [ $elapsed -lt 60 ]; do
        echo -n "."
        sleep 2
        elapsed=$((elapsed + 2))
    done
    
    log_info "Cleanup complete"
}

function main() {
    check_prerequisites
    
    echo ""
    log_info "Step 1: Create RepoBinding"
    create_repobinding
    
    echo ""
    log_info "Step 2: Wait for onboarding"
    if ! wait_for_onboarding; then
        log_error "Onboarding failed"
        exit 1
    fi
    
    echo ""
    log_info "Step 3: Verify tenant resources"
    if ! verify_tenant_resources; then
        log_error "Tenant resource verification failed"
        exit 1
    fi
    
    echo ""
    log_info "Step 4: Verify allowlist"
    if ! verify_allowlist; then
        log_error "Allowlist verification failed"
        exit 1
    fi
    
    echo ""
    log_info "Step 5: Trigger pipeline"
    simulate_webhook
    
    echo ""
    log_info "Step 6: Wait for PipelineRun"
    pipelinerun=$(wait_for_pipelinerun)
    if [ -z "$pipelinerun" ]; then
        log_error "PipelineRun creation failed"
        exit 1
    fi
    
    echo ""
    log_info "Step 7: Verify PipelineRun identity"
    if ! verify_pipelinerun_identity "$pipelinerun"; then
        log_error "PipelineRun identity verification failed"
        exit 1
    fi
    
    echo ""
    log_info "Step 8: Wait for pipeline completion"
    if ! wait_for_pipeline_completion "$pipelinerun"; then
        log_error "Pipeline execution failed"
        
        echo ""
        log_info "Showing recent logs for debugging:"
        kubectl logs -n ${TEST_TENANT_NAME} -l tekton.dev/pipelineRun=${pipelinerun##*/} --tail=50 || true
        
        exit 1
    fi
    
    echo ""
    log_info "Step 9: Verify task execution"
    verify_task_execution "$pipelinerun"
    
    echo ""
    log_info "Step 10: Verify Terraform state"
    verify_terraform_state
    
    echo ""
    log_info "Step 11: Verify logs"
    if ! verify_logs "$pipelinerun"; then
        log_error "Log verification failed"
        exit 1
    fi
    
    echo ""
    echo "========================================="
    log_info "Pipeline Execution Test: PASSED"
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
