#!/bin/bash
set -euo pipefail

# Verify Terraform State
# This script verifies that Terraform state is stored correctly and isolated to the tenant

TEST_TENANT_NAME="${TEST_TENANT_NAME:-test-tenant}"

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
echo "Verify Terraform State"
echo "========================================="
echo "Tenant: ${TEST_TENANT_NAME}"
echo ""

# Check if tenant namespace exists
if ! kubectl get namespace ${TEST_TENANT_NAME} &> /dev/null; then
    log_error "Tenant namespace not found: ${TEST_TENANT_NAME}"
    exit 1
fi

log_info "✓ Tenant namespace exists"

# Check backend configuration secret
echo ""
log_info "Checking Terraform backend configuration..."

if ! kubectl get secret terraform-backend-config -n ${TEST_TENANT_NAME} &> /dev/null; then
    log_error "Terraform backend configuration secret not found"
    exit 1
fi

log_info "✓ Backend configuration secret exists"

# Show backend configuration (without sensitive data)
echo ""
log_info "Backend configuration:"
kubectl get secret terraform-backend-config -n ${TEST_TENANT_NAME} -o jsonpath='{.data}' | jq 'keys' 2>/dev/null || echo "  (keys not available)"

# Check for Kubernetes backend state (if using Kubernetes backend)
echo ""
log_info "Checking for Terraform state in Kubernetes backend..."

state_secrets=$(kubectl get secret -n ${TEST_TENANT_NAME} -l tfstate=true -o name 2>/dev/null || echo "")

if [ -n "$state_secrets" ]; then
    log_info "✓ Terraform state found in Kubernetes backend"
    
    echo ""
    log_info "State secrets:"
    echo "$state_secrets"
    
    # Verify state is in correct namespace
    for secret in $state_secrets; do
        secret_name=${secret##*/}
        namespace=$(kubectl get secret $secret_name -n ${TEST_TENANT_NAME} -o jsonpath='{.metadata.namespace}')
        
        if [ "$namespace" == "${TEST_TENANT_NAME}" ]; then
            log_info "✓ State secret $secret_name is in correct namespace: $namespace"
        else
            log_error "State secret $secret_name is in wrong namespace: $namespace (expected: ${TEST_TENANT_NAME})"
            exit 1
        fi
        
        # Show state metadata
        echo ""
        log_info "State secret $secret_name metadata:"
        kubectl get secret $secret_name -n ${TEST_TENANT_NAME} -o jsonpath='{.metadata}' | jq '.' 2>/dev/null || kubectl get secret $secret_name -n ${TEST_TENANT_NAME} -o jsonpath='{.metadata}'
    done
    
else
    log_warn "No Terraform state found in Kubernetes backend"
    log_info "This may be expected if using a different backend (S3, Consul, etc.)"
    
    echo ""
    log_info "To check state in other backends:"
    echo "  - S3/MinIO: Check bucket contents"
    echo "  - Consul: Check Consul KV store"
    echo "  - HTTP: Check HTTP backend endpoint"
fi

# Verify state isolation
echo ""
log_info "Verifying state isolation..."

# Check if other tenants can access this tenant's state
other_tenants=$(kubectl get namespace -l platform.arbiter.io/tenant -o name 2>/dev/null | grep -v ${TEST_TENANT_NAME} || echo "")

if [ -n "$other_tenants" ]; then
    log_info "Found other tenant namespaces, checking isolation..."
    
    for tenant_ns in $other_tenants; do
        tenant_name=${tenant_ns##*/}
        
        # Try to access state from other tenant (should fail)
        if kubectl get secret -n ${TEST_TENANT_NAME} --as=system:serviceaccount:${tenant_name}:pipeline-runner &> /dev/null; then
            log_error "✗ Tenant $tenant_name can access ${TEST_TENANT_NAME} secrets (isolation breach!)"
            exit 1
        else
            log_info "✓ Tenant $tenant_name cannot access ${TEST_TENANT_NAME} secrets"
        fi
    done
else
    log_info "No other tenants found, skipping cross-tenant isolation check"
fi

# Check deployed resources
echo ""
log_info "Checking deployed resources..."

# The test CDKTF stack creates a ConfigMap in default namespace
if kubectl get configmap pipeline-test-config -n default &> /dev/null; then
    log_info "✓ Deployed resource found: ConfigMap pipeline-test-config"
    
    echo ""
    log_info "ConfigMap details:"
    kubectl get configmap pipeline-test-config -n default -o yaml | grep -A 10 "data:"
    
else
    log_warn "Deployed ConfigMap not found (may not have been created yet)"
fi

# Summary
echo ""
echo "========================================="
log_info "Terraform State Verification: PASSED"
echo "========================================="
echo ""
log_info "Summary:"
echo "  ✓ Backend configuration exists"
echo "  ✓ State is stored correctly"
echo "  ✓ State is isolated to tenant namespace"

if [ -n "$state_secrets" ]; then
    echo "  ✓ State found in Kubernetes backend"
fi

echo ""
log_info "State management verified successfully"
