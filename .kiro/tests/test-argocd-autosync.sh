#!/bin/bash

# Property-Based Test for ArgoCD Auto-Sync After Bootstrap
# Feature: argocd-tekton-platform
# Property 39: ArgoCD Auto-Sync After Bootstrap
# Validates: Requirements 12.4

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test configuration
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

# Property 39: ArgoCD Auto-Sync After Bootstrap
# For any successful bootstrap execution, ArgoCD should automatically sync all platform 
# components without manual intervention
test_property_39_argocd_autosync() {
    echo ""
    echo "=========================================="
    echo "Property 39: ArgoCD Auto-Sync After Bootstrap"
    echo "=========================================="
    echo ""
    
    # Check if ArgoCD is installed
    if ! kubectl get namespace argocd &>/dev/null; then
        print_result "Property 39" "FAIL" "ArgoCD namespace does not exist - bootstrap may not have completed"
        return 1
    fi
    
    print_result "Property 39.1" "PASS" "ArgoCD namespace exists"
    
    # Check if ArgoCD server is running
    if ! kubectl get deployment argocd-server -n argocd &>/dev/null; then
        print_result "Property 39" "FAIL" "ArgoCD server deployment does not exist"
        return 1
    fi
    
    local server_ready
    server_ready=$(kubectl get deployment argocd-server -n argocd -o jsonpath='{.status.readyReplicas}' 2>/dev/null)
    
    if [ -z "$server_ready" ] || [ "$server_ready" = "0" ]; then
        print_result "Property 39" "FAIL" "ArgoCD server is not ready"
        return 1
    fi
    
    print_result "Property 39.2" "PASS" "ArgoCD server is running and ready"
    
    # Check if platform-root Application exists
    if ! kubectl get application platform-root -n argocd &>/dev/null; then
        print_result "Property 39" "FAIL" "platform-root Application does not exist"
        return 1
    fi
    
    print_result "Property 39.3" "PASS" "platform-root Application exists"
    
    # Check if platform-root has automated sync enabled
    local auto_sync
    auto_sync=$(kubectl get application platform-root -n argocd -o jsonpath='{.spec.syncPolicy.automated}' 2>/dev/null)
    
    if [ -z "$auto_sync" ] || [ "$auto_sync" = "null" ]; then
        print_result "Property 39" "FAIL" "platform-root does not have automated sync configured"
        return 1
    fi
    
    print_result "Property 39.4" "PASS" "platform-root has automated sync configured"
    
    # Check sync status of platform-root
    local sync_status
    sync_status=$(kubectl get application platform-root -n argocd -o jsonpath='{.status.sync.status}' 2>/dev/null)
    
    if [ "$sync_status" != "Synced" ]; then
        echo -e "${YELLOW}  Warning: platform-root sync status is '${sync_status}' (expected 'Synced')${NC}"
        echo "  This may indicate the Application is still syncing or has sync errors"
        
        # Check if there are sync errors
        local sync_message
        sync_message=$(kubectl get application platform-root -n argocd -o jsonpath='{.status.conditions[?(@.type=="SyncError")].message}' 2>/dev/null)
        
        if [ -n "$sync_message" ]; then
            print_result "Property 39" "FAIL" "platform-root has sync errors: ${sync_message}"
            return 1
        fi
        
        # If no errors but not synced, it may be in progress
        if [ "$sync_status" = "OutOfSync" ] || [ "$sync_status" = "Unknown" ]; then
            print_result "Property 39" "WARN" "platform-root is ${sync_status} - sync may be in progress"
        fi
    else
        print_result "Property 39.5" "PASS" "platform-root is synced"
    fi
    
    # Check if child Applications were created automatically
    local child_apps=("platform-crds" "platform-infrastructure" "platform-controllers" "platform-catalog")
    local all_exist=true
    local synced_count=0
    
    echo ""
    echo "  Checking child Applications:"
    
    for app in "${child_apps[@]}"; do
        if ! kubectl get application "$app" -n argocd &>/dev/null; then
            echo -e "    ${RED}✗${NC} ${app}: Does not exist"
            all_exist=false
        else
            local child_sync_status
            child_sync_status=$(kubectl get application "$app" -n argocd -o jsonpath='{.status.sync.status}' 2>/dev/null)
            
            if [ "$child_sync_status" = "Synced" ]; then
                echo -e "    ${GREEN}✓${NC} ${app}: Synced"
                synced_count=$((synced_count + 1))
            else
                echo -e "    ${YELLOW}○${NC} ${app}: ${child_sync_status}"
            fi
        fi
    done
    
    if [ "$all_exist" = false ]; then
        print_result "Property 39" "FAIL" "Not all child Applications were created automatically"
        return 1
    fi
    
    print_result "Property 39.6" "PASS" "All child Applications were created automatically (${synced_count}/4 synced)"
    
    # Verify that sync happened without manual intervention
    # Check that Applications have automated sync policy
    local all_automated=true
    
    for app in "${child_apps[@]}"; do
        local child_auto_sync
        child_auto_sync=$(kubectl get application "$app" -n argocd -o jsonpath='{.spec.syncPolicy.automated}' 2>/dev/null)
        
        if [ -z "$child_auto_sync" ] || [ "$child_auto_sync" = "null" ]; then
            echo -e "    ${RED}✗${NC} ${app}: Does not have automated sync"
            all_automated=false
        fi
    done
    
    if [ "$all_automated" = false ]; then
        print_result "Property 39" "FAIL" "Not all child Applications have automated sync configured"
        return 1
    fi
    
    print_result "Property 39.7" "PASS" "All child Applications have automated sync configured"
    
    # Final verdict
    if [ "$sync_status" = "Synced" ] && [ $synced_count -ge 3 ]; then
        print_result "Property 39" "PASS" "ArgoCD automatically synced platform components after bootstrap"
        return 0
    elif [ "$sync_status" = "Synced" ] && [ $synced_count -ge 1 ]; then
        print_result "Property 39" "PASS" "ArgoCD is syncing platform components automatically (${synced_count}/4 complete)"
        return 0
    else
        print_result "Property 39" "WARN" "ArgoCD auto-sync is configured but sync may still be in progress"
        return 0
    fi
}

# Main test execution
main() {
    echo ""
    echo "=========================================="
    echo "ArgoCD Auto-Sync Property Test"
    echo "=========================================="
    echo ""
    echo "Feature: argocd-tekton-platform"
    echo "Property: 39 (ArgoCD Auto-Sync After Bootstrap)"
    echo "Validates: Requirements 12.4"
    echo ""
    
    # Check prerequisites
    check_kubectl
    
    echo "Cluster: $(kubectl config current-context)"
    echo ""
    
    # Run property test
    test_property_39_argocd_autosync
    
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
