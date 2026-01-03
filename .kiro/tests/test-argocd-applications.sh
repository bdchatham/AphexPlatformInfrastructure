#!/bin/bash

# Property-Based Tests for ArgoCD Application Configuration
# Feature: argocd-tekton-platform
# Property 6: Platform Application Existence
# Property 10: Automated Sync Policy
# Validates: Requirements 2.1, 2.6, 3.5, 18.3

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test configuration
MIN_ITERATIONS=100
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Test counters
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

# Function to print test results
print_result() {
    local test_name="$1"
    local result="$2"
    local message="$3"
    
    TESTS_RUN=$((TESTS_RUN + 1))
    
    if [ "$result" = "PASS" ]; then
        echo -e "${GREEN}✓${NC} ${test_name}: ${message}"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        echo -e "${RED}✗${NC} ${test_name}: ${message}"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
}

# Function to check if kubectl is available
check_kubectl() {
    if ! command -v kubectl &> /dev/null; then
        echo -e "${RED}Error: kubectl is not installed or not in PATH${NC}"
        exit 1
    fi
    
    if ! kubectl cluster-info &> /dev/null; then
        echo -e "${RED}Error: Cannot access Kubernetes cluster${NC}"
        echo "Please ensure kubectl is configured and a cluster is accessible"
        exit 1
    fi
}

# Property 6: Platform Application Existence
# For any successful bootstrap execution, an ArgoCD Application named "platform-root" 
# should exist in the argocd namespace pointing to the platform repository
test_property_6_platform_application_existence() {
    echo ""
    echo "=========================================="
    echo "Property 6: Platform Application Existence"
    echo "=========================================="
    echo ""
    
    # Check if argocd namespace exists
    if ! kubectl get namespace argocd &>/dev/null; then
        print_result "Property 6" "FAIL" "argocd namespace does not exist"
        return 1
    fi
    
    print_result "Property 6.1" "PASS" "argocd namespace exists"
    
    # Check if platform-root Application exists
    if ! kubectl get application platform-root -n argocd &>/dev/null; then
        print_result "Property 6" "FAIL" "platform-root Application does not exist"
        return 1
    fi
    
    print_result "Property 6.2" "PASS" "platform-root Application exists"
    
    # Verify Application points to correct repository
    local repo_url
    repo_url=$(kubectl get application platform-root -n argocd -o jsonpath='{.spec.source.repoURL}' 2>/dev/null)
    
    if [ -z "$repo_url" ]; then
        print_result "Property 6" "FAIL" "Cannot retrieve repository URL from Application"
        return 1
    fi
    
    if [[ ! "$repo_url" =~ "ArbiterPipelineInfrastructure" ]]; then
        print_result "Property 6" "FAIL" "Application points to wrong repository: ${repo_url}"
        return 1
    fi
    
    print_result "Property 6.3" "PASS" "Application points to correct repository: ${repo_url}"
    
    # Verify Application points to correct path
    local source_path
    source_path=$(kubectl get application platform-root -n argocd -o jsonpath='{.spec.source.path}' 2>/dev/null)
    
    if [ "$source_path" != "platform/argocd/apps" ]; then
        print_result "Property 6" "FAIL" "Application points to wrong path: ${source_path}"
        return 1
    fi
    
    print_result "Property 6.4" "PASS" "Application points to correct path: ${source_path}"
    
    # Verify Application targets correct namespace
    local dest_namespace
    dest_namespace=$(kubectl get application platform-root -n argocd -o jsonpath='{.spec.destination.namespace}' 2>/dev/null)
    
    if [ "$dest_namespace" != "argocd" ]; then
        print_result "Property 6" "FAIL" "Application targets wrong namespace: ${dest_namespace}"
        return 1
    fi
    
    print_result "Property 6.5" "PASS" "Application targets correct namespace: ${dest_namespace}"
    
    print_result "Property 6" "PASS" "Platform Application exists with correct configuration"
    return 0
}

# Property 10: Automated Sync Policy
# For any platform Application, the syncPolicy should have automated sync, self-heal, and prune enabled
test_property_10_automated_sync_policy() {
    echo ""
    echo "=========================================="
    echo "Property 10: Automated Sync Policy"
    echo "=========================================="
    echo ""
    
    local applications=("platform-root" "platform-crds" "platform-infrastructure" "platform-controllers" "platform-catalog")
    local all_valid=true
    
    for app in "${applications[@]}"; do
        echo "Testing Application: ${app}"
        
        # Check if Application exists
        if ! kubectl get application "$app" -n argocd &>/dev/null; then
            print_result "Property 10 (${app})" "FAIL" "Application does not exist"
            all_valid=false
            continue
        fi
        
        # Check automated sync
        local auto_sync
        auto_sync=$(kubectl get application "$app" -n argocd -o jsonpath='{.spec.syncPolicy.automated}' 2>/dev/null)
        
        if [ -z "$auto_sync" ] || [ "$auto_sync" = "null" ]; then
            print_result "Property 10 (${app})" "FAIL" "Automated sync not configured"
            all_valid=false
            continue
        fi
        
        # Check prune enabled
        local prune
        prune=$(kubectl get application "$app" -n argocd -o jsonpath='{.spec.syncPolicy.automated.prune}' 2>/dev/null)
        
        if [ "$prune" != "true" ]; then
            print_result "Property 10 (${app})" "FAIL" "Prune not enabled (value: ${prune})"
            all_valid=false
            continue
        fi
        
        # Check selfHeal enabled
        local self_heal
        self_heal=$(kubectl get application "$app" -n argocd -o jsonpath='{.spec.syncPolicy.automated.selfHeal}' 2>/dev/null)
        
        if [ "$self_heal" != "true" ]; then
            print_result "Property 10 (${app})" "FAIL" "SelfHeal not enabled (value: ${self_heal})"
            all_valid=false
            continue
        fi
        
        # Check retry policy exists
        local retry_limit
        retry_limit=$(kubectl get application "$app" -n argocd -o jsonpath='{.spec.syncPolicy.retry.limit}' 2>/dev/null)
        
        if [ -z "$retry_limit" ] || [ "$retry_limit" = "null" ]; then
            print_result "Property 10 (${app})" "FAIL" "Retry policy not configured"
            all_valid=false
            continue
        fi
        
        print_result "Property 10 (${app})" "PASS" "Automated sync policy correctly configured (prune=true, selfHeal=true, retry.limit=${retry_limit})"
    done
    
    if [ "$all_valid" = true ]; then
        print_result "Property 10" "PASS" "All Applications have correct automated sync policies"
        return 0
    else
        print_result "Property 10" "FAIL" "Some Applications have incorrect sync policies"
        return 1
    fi
}

# Main test execution
main() {
    echo ""
    echo "=========================================="
    echo "ArgoCD Application Property Tests"
    echo "=========================================="
    echo ""
    echo "Feature: argocd-tekton-platform"
    echo "Properties: 6 (Platform Application Existence), 10 (Automated Sync Policy)"
    echo "Validates: Requirements 2.1, 2.6, 3.5, 18.3"
    echo ""
    
    # Check prerequisites
    check_kubectl
    
    echo "Cluster: $(kubectl config current-context)"
    echo ""
    
    # Run property tests
    test_property_6_platform_application_existence
    test_property_10_automated_sync_policy
    
    # Print summary
    echo ""
    echo "=========================================="
    echo "Test Summary"
    echo "=========================================="
    echo ""
    echo "Tests run: ${TESTS_RUN}"
    echo -e "${GREEN}Tests passed: ${TESTS_PASSED}${NC}"
    if [ $TESTS_FAILED -gt 0 ]; then
        echo -e "${RED}Tests failed: ${TESTS_FAILED}${NC}"
    else
        echo "Tests failed: ${TESTS_FAILED}"
    fi
    echo ""
    
    if [ $TESTS_FAILED -gt 0 ]; then
        echo -e "${RED}Some tests failed${NC}"
        exit 1
    else
        echo -e "${GREEN}All tests passed${NC}"
        exit 0
    fi
}

# Run main function
main "$@"
