#!/bin/bash

# Tenant Verification Script
# Verifies that all tenant resources exist and are properly configured
# Requirements: 13.5

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Track overall status
OVERALL_STATUS=0

# Helper functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
    OVERALL_STATUS=1
}

log_section() {
    echo ""
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}========================================${NC}"
}

print_usage() {
    echo "Usage: $0 <tenant-name>"
    echo ""
    echo "Verifies that all resources for a tenant are properly configured."
    echo ""
    echo "Arguments:"
    echo "  tenant-name    Name of the tenant namespace to verify"
    echo ""
    echo "Example:"
    echo "  $0 tenant-example-repo"
    echo ""
}

check_resource() {
    local resource_type=$1
    local resource_name=$2
    local namespace=$3
    
    if kubectl get "$resource_type" "$resource_name" -n "$namespace" &>/dev/null; then
        log_info "✓ $resource_type '$resource_name' exists"
        return 0
    else
        log_error "✗ $resource_type '$resource_name' NOT found"
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

check_eventlistener_ready() {
    local tenant_name=$1
    
    # EventListener creates a deployment with name el-<eventlistener-name>
    local el_name="github-listener"
    local deployment_name="el-${el_name}"
    
    if ! kubectl get deployment "$deployment_name" -n "$tenant_name" &>/dev/null; then
        log_error "✗ EventListener deployment '$deployment_name' NOT found"
        return 1
    fi
    
    local ready=$(kubectl get deployment "$deployment_name" -n "$tenant_name" -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
    local desired=$(kubectl get deployment "$deployment_name" -n "$tenant_name" -o jsonpath='{.spec.replicas}' 2>/dev/null || echo "1")
    
    if [ "$ready" = "$desired" ] && [ "$ready" != "0" ]; then
        log_info "✓ EventListener is ready ($ready/$desired replicas)"
        return 0
    else
        log_error "✗ EventListener is NOT ready ($ready/$desired replicas)"
        return 1
    fi
}

check_ingress_configured() {
    local tenant_name=$1
    
    if ! kubectl get ingress -n "$tenant_name" &>/dev/null; then
        log_error "✗ No Ingress found in namespace"
        return 1
    fi
    
    local ingress_name=$(kubectl get ingress -n "$tenant_name" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
    
    if [ -z "$ingress_name" ]; then
        log_error "✗ No Ingress found in namespace"
        return 1
    fi
    
    log_info "✓ Ingress '$ingress_name' exists"
    
    # Check if Ingress has rules configured
    local rules=$(kubectl get ingress "$ingress_name" -n "$tenant_name" -o jsonpath='{.spec.rules}' 2>/dev/null || echo "")
    
    if [ -n "$rules" ] && [ "$rules" != "null" ]; then
        log_info "✓ Ingress has routing rules configured"
        
        # Display webhook URL
        local host=$(kubectl get ingress "$ingress_name" -n "$tenant_name" -o jsonpath='{.spec.rules[0].host}' 2>/dev/null || echo "")
        local path=$(kubectl get ingress "$ingress_name" -n "$tenant_name" -o jsonpath='{.spec.rules[0].http.paths[0].path}' 2>/dev/null || echo "")
        
        if [ -n "$host" ]; then
            log_info "  Webhook URL: https://${host}${path}"
        fi
        
        return 0
    else
        log_error "✗ Ingress has no routing rules configured"
        return 1
    fi
}

# Main verification
main() {
    # Check arguments
    if [ $# -lt 1 ]; then
        print_usage
        exit 1
    fi
    
    local tenant_name=$1
    
    log_section "Tenant Verification: $tenant_name"
    
    echo ""
    echo "This script verifies:"
    echo "  1. Tenant namespace exists"
    echo "  2. Service account and RBAC are configured"
    echo "  3. Resource quotas and limits are set"
    echo "  4. Network policy is configured"
    echo "  5. EventListener is ready"
    echo "  6. Ingress is configured"
    echo ""
    
    # Check 1: Verify tenant namespace
    log_section "1. Verifying Tenant Namespace"
    
    if ! kubectl get namespace "$tenant_name" &>/dev/null; then
        log_error "Namespace '$tenant_name' does not exist"
        echo ""
        echo "Please ensure the tenant has been provisioned via RepoBinding."
        exit 1
    fi
    
    log_info "✓ Namespace '$tenant_name' exists"
    
    # Check namespace labels
    check_label "namespace" "$tenant_name" "" "platform.arbiter.io/tenant" "$tenant_name" || true
    check_label "namespace" "$tenant_name" "" "platform.arbiter.io/managed-by" "onboarding-controller" || true
    
    # Check 2: Verify service account and RBAC
    log_section "2. Verifying Service Account and RBAC"
    
    check_resource "serviceaccount" "pipeline-runner" "$tenant_name"
    check_resource "role" "pipeline-runner" "$tenant_name" || check_resource "role" "pipeline-runner-standard" "$tenant_name" || check_resource "role" "pipeline-runner-elevated" "$tenant_name"
    check_resource "rolebinding" "pipeline-runner" "$tenant_name"
    
    # Check 3: Verify resource quotas and limits
    log_section "3. Verifying Resource Quotas and Limits"
    
    check_resource "resourcequota" "tenant-quota" "$tenant_name"
    check_resource "limitrange" "tenant-limits" "$tenant_name"
    
    # Display quota details
    echo ""
    log_info "Resource Quota Details:"
    kubectl get resourcequota tenant-quota -n "$tenant_name" -o custom-columns=RESOURCE:.spec.hard 2>/dev/null | tail -n +2 | sed 's/^/  /'
    
    # Check 4: Verify network policy
    log_section "4. Verifying Network Policy"
    
    check_resource "networkpolicy" "tenant-isolation" "$tenant_name"
    
    # Check 5: Verify EventListener
    log_section "5. Verifying EventListener"
    
    check_resource "eventlistener" "github-listener" "$tenant_name"
    check_eventlistener_ready "$tenant_name"
    
    # Check EventListener service
    local el_service="el-github-listener"
    check_resource "service" "$el_service" "$tenant_name"
    
    # Check 6: Verify Ingress
    log_section "6. Verifying Ingress Configuration"
    
    check_ingress_configured "$tenant_name"
    
    # Summary
    log_section "Verification Summary"
    
    if [ $OVERALL_STATUS -eq 0 ]; then
        echo -e "${GREEN}✓ ALL CHECKS PASSED${NC}"
        echo ""
        echo "Tenant '$tenant_name' is fully configured:"
        echo "  ✓ Namespace exists with proper labels"
        echo "  ✓ Service account and RBAC configured"
        echo "  ✓ Resource quotas and limits set"
        echo "  ✓ Network policy configured"
        echo "  ✓ EventListener is ready"
        echo "  ✓ Ingress is configured"
        echo ""
        echo "Next steps:"
        echo "  1. Get webhook URL and secret from RepoBinding status:"
        echo "     kubectl get repobinding -n platform-system -o yaml | grep -A 10 'tenantName: $tenant_name'"
        echo ""
        echo "  2. Configure GitHub webhook with the URL and secret"
        echo ""
        echo "  3. Test webhook delivery:"
        echo "     .kiro/tests/test-webhook.sh $tenant_name"
    else
        echo -e "${RED}✗ SOME CHECKS FAILED${NC}"
        echo ""
        echo "Please review the errors above and troubleshoot:"
        echo ""
        echo "Common issues:"
        echo "  1. Tenant not fully provisioned - check RepoBinding status:"
        echo "     kubectl get repobinding -n platform-system"
        echo ""
        echo "  2. Check onboarding controller logs:"
        echo "     kubectl logs -n platform-system -l app=onboarding-controller"
        echo ""
        echo "  3. Check EventListener logs:"
        echo "     kubectl logs -n $tenant_name -l eventlistener=github-listener"
    fi
    
    echo ""
    exit $OVERALL_STATUS
}

# Run main function
main "$@"
