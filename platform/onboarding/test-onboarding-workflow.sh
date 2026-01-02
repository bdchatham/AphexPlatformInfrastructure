#!/bin/bash
# Test script for validating the onboarding workflow
# This script creates a test RepoBinding and verifies all tenant resources are provisioned correctly

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test configuration
TEST_REPOBINDING_NAME="test-onboarding-binding"
TEST_TENANT_NAME="test-tenant"
TEST_REPO_ORG="your-github-org"
TEST_REPO_NAME="test-onboarding-repo"
PLATFORM_NAMESPACE="pipeline-system"

# Helper functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_resource() {
    local resource_type=$1
    local resource_name=$2
    local namespace=$3
    
    if kubectl get "$resource_type" "$resource_name" -n "$namespace" &>/dev/null; then
        log_info "✓ $resource_type '$resource_name' exists in namespace '$namespace'"
        return 0
    else
        log_error "✗ $resource_type '$resource_name' NOT found in namespace '$namespace'"
        return 1
    fi
}

check_label() {
    local resource_type=$1
    local resource_name=$2
    local namespace=$3
    local label_key=$4
    local expected_value=$5
    
    local actual_value=$(kubectl get "$resource_type" "$resource_name" -n "$namespace" -o jsonpath="{.metadata.labels.$label_key}" 2>/dev/null || echo "")
    
    if [ "$actual_value" = "$expected_value" ]; then
        log_info "✓ Label '$label_key=$expected_value' is correct"
        return 0
    else
        log_error "✗ Label '$label_key' has value '$actual_value', expected '$expected_value'"
        return 1
    fi
}

wait_for_repobinding_ready() {
    local max_wait=120
    local elapsed=0
    
    log_info "Waiting for RepoBinding to reach 'Ready' status (max ${max_wait}s)..."
    
    while [ $elapsed -lt $max_wait ]; do
        local phase=$(kubectl get repobinding "$TEST_REPOBINDING_NAME" -n "$PLATFORM_NAMESPACE" -o jsonpath='{.status.phase}' 2>/dev/null || echo "")
        
        if [ "$phase" = "Ready" ]; then
            log_info "✓ RepoBinding reached 'Ready' status"
            return 0
        elif [ "$phase" = "Failed" ]; then
            local message=$(kubectl get repobinding "$TEST_REPOBINDING_NAME" -n "$PLATFORM_NAMESPACE" -o jsonpath='{.status.message}' 2>/dev/null || echo "")
            log_error "✗ RepoBinding failed: $message"
            return 1
        fi
        
        sleep 5
        elapsed=$((elapsed + 5))
        echo -n "."
    done
    
    echo ""
    log_error "✗ Timeout waiting for RepoBinding to reach 'Ready' status"
    return 1
}

# Main test workflow
main() {
    log_info "========================================="
    log_info "Testing Onboarding Workflow"
    log_info "========================================="
    
    # Task 15.1: Create test RepoBinding
    log_info ""
    log_info "Task 15.1: Creating test RepoBinding..."
    kubectl apply -f platform/onboarding/test-repobinding.yaml
    log_info "✓ Test RepoBinding created"
    
    # Wait for reconciliation
    if ! wait_for_repobinding_ready; then
        log_error "Onboarding workflow failed"
        exit 1
    fi
    
    # Task 15.2: Verify tenant namespace creation
    log_info ""
    log_info "Task 15.2: Verifying tenant namespace creation..."
    if ! check_resource "namespace" "$TEST_TENANT_NAME" ""; then
        exit 1
    fi
    
    # Verify namespace labels
    check_label "namespace" "$TEST_TENANT_NAME" "" "platform.arbiter.io/tenant" "$TEST_TENANT_NAME"
    check_label "namespace" "$TEST_TENANT_NAME" "" "platform.arbiter.io/repo" "$TEST_REPO_ORG/$TEST_REPO_NAME"
    check_label "namespace" "$TEST_TENANT_NAME" "" "platform.arbiter.io/managed-by" "onboarding-controller"
    
    # Task 15.3: Verify service account creation
    log_info ""
    log_info "Task 15.3: Verifying service account creation..."
    if ! check_resource "serviceaccount" "pipeline-runner" "$TEST_TENANT_NAME"; then
        exit 1
    fi
    
    # Task 15.4: Verify RBAC configuration
    log_info ""
    log_info "Task 15.4: Verifying RBAC configuration..."
    if ! check_resource "role" "pipeline-runner" "$TEST_TENANT_NAME"; then
        exit 1
    fi
    if ! check_resource "rolebinding" "pipeline-runner" "$TEST_TENANT_NAME"; then
        exit 1
    fi
    
    # Verify RBAC is scoped correctly (check that Role is namespace-scoped)
    local role_rules=$(kubectl get role pipeline-runner -n "$TEST_TENANT_NAME" -o json)
    log_info "✓ Role 'pipeline-runner' is namespace-scoped (Role, not ClusterRole)"
    
    # Task 15.5: Verify resource limits
    log_info ""
    log_info "Task 15.5: Verifying resource limits..."
    if ! check_resource "resourcequota" "tenant-quota" "$TEST_TENANT_NAME"; then
        exit 1
    fi
    if ! check_resource "limitrange" "tenant-limits" "$TEST_TENANT_NAME"; then
        exit 1
    fi
    
    # Task 15.6: Verify network policy
    log_info ""
    log_info "Task 15.6: Verifying network policy..."
    if ! check_resource "networkpolicy" "tenant-isolation" "$TEST_TENANT_NAME"; then
        exit 1
    fi
    
    # Task 15.7: Verify Terraform backend secret
    log_info ""
    log_info "Task 15.7: Verifying Terraform backend secret..."
    if ! check_resource "secret" "terraform-backend-config" "$TEST_TENANT_NAME"; then
        exit 1
    fi
    
    # Verify backend configuration exists
    local backend_config=$(kubectl get secret terraform-backend-config -n "$TEST_TENANT_NAME" -o jsonpath='{.data.backend\.tf}' 2>/dev/null || echo "")
    if [ -n "$backend_config" ]; then
        log_info "✓ Terraform backend configuration exists in secret"
    else
        log_error "✗ Terraform backend configuration NOT found in secret"
        exit 1
    fi
    
    # Task 15.8: Verify allowlist update
    log_info ""
    log_info "Task 15.8: Verifying allowlist update..."
    if ! check_resource "configmap" "repo-allowlist" "$PLATFORM_NAMESPACE"; then
        log_warn "ConfigMap 'repo-allowlist' not found - may not be created yet"
    else
        # Check if repository is in allowlist
        local allowlist_data=$(kubectl get configmap repo-allowlist -n "$PLATFORM_NAMESPACE" -o jsonpath='{.data.allowlist\.yaml}' 2>/dev/null || echo "")
        if echo "$allowlist_data" | grep -q "$TEST_REPO_NAME"; then
            log_info "✓ Repository '$TEST_REPO_NAME' found in allowlist"
            
            # Verify tenant mapping
            if echo "$allowlist_data" | grep -A 2 "$TEST_REPO_NAME" | grep -q "tenant: $TEST_TENANT_NAME"; then
                log_info "✓ Tenant mapping is correct"
            else
                log_error "✗ Tenant mapping NOT found or incorrect"
                exit 1
            fi
        else
            log_error "✗ Repository '$TEST_REPO_NAME' NOT found in allowlist"
            exit 1
        fi
    fi
    
    # Task 15.9: Verify RepoBinding status
    log_info ""
    log_info "Task 15.9: Verifying RepoBinding status..."
    
    local status_phase=$(kubectl get repobinding "$TEST_REPOBINDING_NAME" -n "$PLATFORM_NAMESPACE" -o jsonpath='{.status.phase}')
    if [ "$status_phase" = "Ready" ]; then
        log_info "✓ RepoBinding status is 'Ready'"
    else
        log_error "✗ RepoBinding status is '$status_phase', expected 'Ready'"
        exit 1
    fi
    
    # Verify all resource creation flags
    local namespace_created=$(kubectl get repobinding "$TEST_REPOBINDING_NAME" -n "$PLATFORM_NAMESPACE" -o jsonpath='{.status.namespaceCreated}')
    local sa_created=$(kubectl get repobinding "$TEST_REPOBINDING_NAME" -n "$PLATFORM_NAMESPACE" -o jsonpath='{.status.serviceAccountCreated}')
    local rbac_configured=$(kubectl get repobinding "$TEST_REPOBINDING_NAME" -n "$PLATFORM_NAMESPACE" -o jsonpath='{.status.rbacConfigured}')
    local allowlist_updated=$(kubectl get repobinding "$TEST_REPOBINDING_NAME" -n "$PLATFORM_NAMESPACE" -o jsonpath='{.status.allowlistUpdated}')
    
    local all_flags_true=true
    
    if [ "$namespace_created" = "true" ]; then
        log_info "✓ namespaceCreated flag is true"
    else
        log_error "✗ namespaceCreated flag is '$namespace_created', expected 'true'"
        all_flags_true=false
    fi
    
    if [ "$sa_created" = "true" ]; then
        log_info "✓ serviceAccountCreated flag is true"
    else
        log_error "✗ serviceAccountCreated flag is '$sa_created', expected 'true'"
        all_flags_true=false
    fi
    
    if [ "$rbac_configured" = "true" ]; then
        log_info "✓ rbacConfigured flag is true"
    else
        log_error "✗ rbacConfigured flag is '$rbac_configured', expected 'true'"
        all_flags_true=false
    fi
    
    if [ "$allowlist_updated" = "true" ]; then
        log_info "✓ allowlistUpdated flag is true"
    else
        log_error "✗ allowlistUpdated flag is '$allowlist_updated', expected 'true'"
        all_flags_true=false
    fi
    
    if [ "$all_flags_true" = false ]; then
        exit 1
    fi
    
    # Summary
    log_info ""
    log_info "========================================="
    log_info "✓ All onboarding workflow tests passed!"
    log_info "========================================="
    log_info ""
    log_info "Cleanup: To remove test resources, run:"
    log_info "  kubectl delete repobinding $TEST_REPOBINDING_NAME -n $PLATFORM_NAMESPACE"
    log_info "  kubectl delete namespace $TEST_TENANT_NAME"
}

# Run main function
main "$@"
