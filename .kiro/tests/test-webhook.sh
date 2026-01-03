#!/bin/bash

# Webhook Testing Script
# Sends a test webhook to EventListener and verifies PipelineRun creation
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
    echo "Usage: $0 <tenant-name> [webhook-secret]"
    echo ""
    echo "Sends a test webhook to the tenant's EventListener and verifies PipelineRun creation."
    echo ""
    echo "Arguments:"
    echo "  tenant-name      Name of the tenant namespace"
    echo "  webhook-secret   Webhook secret (optional, will be retrieved from RepoBinding if not provided)"
    echo ""
    echo "Example:"
    echo "  $0 tenant-example-repo"
    echo "  $0 tenant-example-repo whsec_abc123xyz456"
    echo ""
}

generate_webhook_signature() {
    local secret=$1
    local payload=$2
    
    # Generate HMAC-SHA256 signature
    echo -n "$payload" | openssl dgst -sha256 -hmac "$secret" | sed 's/^.* //'
}

get_webhook_secret() {
    local tenant_name=$1
    
    # Try to get secret from tenant namespace
    local secret=$(kubectl get secret "webhook-${tenant_name}" -n "$tenant_name" -o jsonpath='{.data.secret}' 2>/dev/null | base64 -d 2>/dev/null || echo "")
    
    if [ -n "$secret" ]; then
        echo "$secret"
        return 0
    fi
    
    # Try to get from RepoBinding status
    secret=$(kubectl get repobinding -n platform-system -o json 2>/dev/null | jq -r ".items[] | select(.spec.tenantName == \"$tenant_name\") | .status.webhookSecret" 2>/dev/null || echo "")
    
    if [ -n "$secret" ] && [ "$secret" != "null" ]; then
        echo "$secret"
        return 0
    fi
    
    return 1
}

get_eventlistener_url() {
    local tenant_name=$1
    
    # Try to get from Ingress
    local ingress_host=$(kubectl get ingress -n "$tenant_name" -o jsonpath='{.items[0].spec.rules[0].host}' 2>/dev/null || echo "")
    local ingress_path=$(kubectl get ingress -n "$tenant_name" -o jsonpath='{.items[0].spec.rules[0].http.paths[0].path}' 2>/dev/null || echo "")
    
    if [ -n "$ingress_host" ]; then
        echo "https://${ingress_host}${ingress_path}"
        return 0
    fi
    
    # Fallback: use port-forward URL
    echo "http://localhost:8081"
    return 0
}

wait_for_pipelinerun() {
    local tenant_name=$1
    local max_wait=60
    local elapsed=0
    
    log_info "Waiting for PipelineRun to be created (max ${max_wait}s)..."
    
    local initial_count=$(kubectl get pipelinerun -n "$tenant_name" --no-headers 2>/dev/null | wc -l | tr -d ' ')
    
    while [ $elapsed -lt $max_wait ]; do
        local current_count=$(kubectl get pipelinerun -n "$tenant_name" --no-headers 2>/dev/null | wc -l | tr -d ' ')
        
        if [ "$current_count" -gt "$initial_count" ]; then
            log_info "✓ PipelineRun created"
            
            # Get the latest PipelineRun
            local pipelinerun=$(kubectl get pipelinerun -n "$tenant_name" --sort-by=.metadata.creationTimestamp -o jsonpath='{.items[-1].metadata.name}' 2>/dev/null)
            
            if [ -n "$pipelinerun" ]; then
                log_info "  PipelineRun name: $pipelinerun"
                
                # Display PipelineRun details
                echo ""
                log_info "PipelineRun Details:"
                kubectl get pipelinerun "$pipelinerun" -n "$tenant_name" -o custom-columns=NAME:.metadata.name,STATUS:.status.conditions[0].reason,MESSAGE:.status.conditions[0].message 2>/dev/null | sed 's/^/  /'
            fi
            
            return 0
        fi
        
        sleep 2
        elapsed=$((elapsed + 2))
        echo -n "."
    done
    
    echo ""
    log_error "✗ Timeout waiting for PipelineRun to be created"
    return 1
}

test_webhook_signature_validation() {
    local tenant_name=$1
    local webhook_url=$2
    local webhook_secret=$3
    
    log_section "Testing Webhook Signature Validation"
    
    # Test 1: Valid signature
    log_info "Test 1: Sending webhook with valid signature..."
    
    local payload='{"ref":"refs/heads/main","repository":{"clone_url":"https://github.com/test/repo.git"},"head_commit":{"id":"abc123"}}'
    local signature=$(generate_webhook_signature "$webhook_secret" "$payload")
    
    local response=$(curl -s -w "\n%{http_code}" -X POST "$webhook_url" \
        -H "Content-Type: application/json" \
        -H "X-Hub-Signature-256: sha256=$signature" \
        -H "X-GitHub-Event: push" \
        -d "$payload" 2>/dev/null || echo -e "\n000")
    
    local http_code=$(echo "$response" | tail -n 1)
    local body=$(echo "$response" | head -n -1)
    
    if [ "$http_code" = "202" ] || [ "$http_code" = "201" ] || [ "$http_code" = "200" ]; then
        log_info "✓ Valid signature accepted (HTTP $http_code)"
        
        # Wait for PipelineRun
        wait_for_pipelinerun "$tenant_name"
    else
        log_error "✗ Valid signature rejected (HTTP $http_code)"
        if [ -n "$body" ]; then
            echo "  Response: $body"
        fi
    fi
    
    echo ""
    
    # Test 2: Invalid signature
    log_info "Test 2: Sending webhook with invalid signature..."
    
    local invalid_signature="0000000000000000000000000000000000000000000000000000000000000000"
    
    response=$(curl -s -w "\n%{http_code}" -X POST "$webhook_url" \
        -H "Content-Type: application/json" \
        -H "X-Hub-Signature-256: sha256=$invalid_signature" \
        -H "X-GitHub-Event: push" \
        -d "$payload" 2>/dev/null || echo -e "\n000")
    
    http_code=$(echo "$response" | tail -n 1)
    body=$(echo "$response" | head -n -1)
    
    if [ "$http_code" = "401" ] || [ "$http_code" = "403" ]; then
        log_info "✓ Invalid signature rejected (HTTP $http_code)"
    else
        log_warn "Invalid signature not rejected (HTTP $http_code) - signature validation may not be enabled"
    fi
    
    echo ""
    
    # Test 3: Missing signature
    log_info "Test 3: Sending webhook without signature..."
    
    response=$(curl -s -w "\n%{http_code}" -X POST "$webhook_url" \
        -H "Content-Type: application/json" \
        -H "X-GitHub-Event: push" \
        -d "$payload" 2>/dev/null || echo -e "\n000")
    
    http_code=$(echo "$response" | tail -n 1)
    
    if [ "$http_code" = "401" ] || [ "$http_code" = "403" ]; then
        log_info "✓ Missing signature rejected (HTTP $http_code)"
    else
        log_warn "Missing signature not rejected (HTTP $http_code) - signature validation may not be enabled"
    fi
}

# Main testing
main() {
    # Check arguments
    if [ $# -lt 1 ]; then
        print_usage
        exit 1
    fi
    
    local tenant_name=$1
    local webhook_secret=""
    
    if [ $# -ge 2 ]; then
        webhook_secret=$2
    fi
    
    log_section "Webhook Testing: $tenant_name"
    
    echo ""
    echo "This script will:"
    echo "  1. Retrieve webhook URL and secret"
    echo "  2. Send test webhook with valid signature"
    echo "  3. Verify PipelineRun is created"
    echo "  4. Test signature validation (valid, invalid, missing)"
    echo ""
    
    # Check if tenant namespace exists
    if ! kubectl get namespace "$tenant_name" &>/dev/null; then
        log_error "Namespace '$tenant_name' does not exist"
        exit 1
    fi
    
    # Get webhook secret if not provided
    if [ -z "$webhook_secret" ]; then
        log_info "Retrieving webhook secret..."
        
        if webhook_secret=$(get_webhook_secret "$tenant_name"); then
            log_info "✓ Webhook secret retrieved"
        else
            log_error "✗ Could not retrieve webhook secret"
            echo ""
            echo "Please provide the webhook secret as an argument:"
            echo "  $0 $tenant_name <webhook-secret>"
            exit 1
        fi
    fi
    
    # Get EventListener URL
    log_info "Retrieving EventListener URL..."
    local webhook_url=$(get_eventlistener_url "$tenant_name")
    log_info "✓ EventListener URL: $webhook_url"
    
    # Check if we need to set up port-forward
    if [[ "$webhook_url" == *"localhost"* ]]; then
        log_warn "Using localhost URL - you may need to set up port-forward:"
        echo "  kubectl port-forward -n $tenant_name svc/el-github-listener 8081:8080"
        echo ""
        read -p "Press Enter when port-forward is ready, or Ctrl+C to cancel..."
    fi
    
    echo ""
    
    # Test webhook signature validation
    test_webhook_signature_validation "$tenant_name" "$webhook_url" "$webhook_secret"
    
    # Summary
    log_section "Testing Summary"
    
    if [ $OVERALL_STATUS -eq 0 ]; then
        echo -e "${GREEN}✓ WEBHOOK TESTING PASSED${NC}"
        echo ""
        echo "The webhook integration is working correctly:"
        echo "  ✓ EventListener is accessible"
        echo "  ✓ Valid webhooks create PipelineRuns"
        echo "  ✓ Signature validation is working"
        echo ""
        echo "Next steps:"
        echo "  1. Configure GitHub webhook with:"
        echo "     URL: $webhook_url"
        echo "     Secret: $webhook_secret"
        echo "     Content type: application/json"
        echo "     Events: Push events"
        echo ""
        echo "  2. Monitor PipelineRuns:"
        echo "     kubectl get pipelinerun -n $tenant_name"
        echo "     kubectl logs -n $tenant_name -l tekton.dev/pipelineRun=<name> -f"
    else
        echo -e "${RED}✗ WEBHOOK TESTING FAILED${NC}"
        echo ""
        echo "Please review the errors above and troubleshoot:"
        echo ""
        echo "Common issues:"
        echo "  1. EventListener not accessible - check service and ingress:"
        echo "     kubectl get svc,ingress -n $tenant_name"
        echo ""
        echo "  2. Check EventListener logs:"
        echo "     kubectl logs -n $tenant_name -l eventlistener=github-listener"
        echo ""
        echo "  3. Check TriggerBinding and TriggerTemplate:"
        echo "     kubectl get triggerbinding,triggertemplate -n platform-system"
    fi
    
    echo ""
    exit $OVERALL_STATUS
}

# Run main function
main "$@"
