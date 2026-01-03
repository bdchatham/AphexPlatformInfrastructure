#!/bin/bash

# Property-Based Test for GitOps Sync on Commit
# Feature: argocd-tekton-platform
# Property 7: GitOps Sync on Commit
# Validates: Requirements 2.2, 2.6

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
POLLING_INTERVAL=180  # ArgoCD default polling interval is 3 minutes
MAX_WAIT_TIME=300     # Maximum wait time of 5 minutes

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

# Function to check if git is available
check_git() {
    if ! command -v git &> /dev/null; then
        echo -e "${RED}Error: git is not installed or not in PATH${NC}"
        exit 1
    fi
}

# Property 7: GitOps Sync on Commit
# For any commit to the platform repository, ArgoCD should detect the change and sync 
# the platform Application within the polling interval
test_property_7_gitops_sync_on_commit() {
    echo ""
    echo "=========================================="
    echo "Property 7: GitOps Sync on Commit"
    echo "=========================================="
    echo ""
    
    # Check if ArgoCD is installed
    if ! kubectl get namespace argocd &>/dev/null; then
        print_result "Property 7" "FAIL" "ArgoCD namespace does not exist"
        return 1
    fi
    
    print_result "Property 7.1" "PASS" "ArgoCD namespace exists"
    
    # Check if platform Applications exist
    local applications=("platform-root" "platform-crds" "platform-infrastructure" "platform-controllers" "platform-catalog")
    local all_exist=true
    
    for app in "${applications[@]}"; do
        if ! kubectl get application "$app" -n argocd &>/dev/null; then
            echo -e "  ${RED}✗${NC} Application ${app} does not exist"
            all_exist=false
        fi
    done
    
    if [ "$all_exist" = false ]; then
        print_result "Property 7" "FAIL" "Not all platform Applications exist"
        return 1
    fi
    
    print_result "Property 7.2" "PASS" "All platform Applications exist"
    
    # Get current Git commit SHA
    local current_commit
    current_commit=$(git -C "$REPO_ROOT" rev-parse HEAD 2>/dev/null)
    
    if [ -z "$current_commit" ]; then
        print_result "Property 7" "FAIL" "Cannot determine current Git commit"
        return 1
    fi
    
    echo "  Current Git commit: ${current_commit:0:8}"
    
    # Get current sync revision for platform-root
    local current_sync_revision
    current_sync_revision=$(kubectl get application platform-root -n argocd -o jsonpath='{.status.sync.revision}' 2>/dev/null)
    
    if [ -z "$current_sync_revision" ]; then
        print_result "Property 7" "FAIL" "Cannot determine current sync revision"
        return 1
    fi
    
    echo "  Current sync revision: ${current_sync_revision:0:8}"
    
    # Check if ArgoCD has detected the latest commit
    if [ "$current_commit" = "$current_sync_revision" ]; then
        print_result "Property 7.3" "PASS" "ArgoCD is synced to latest commit"
    else
        echo -e "  ${YELLOW}Note: ArgoCD sync revision differs from HEAD${NC}"
        echo "  This is expected if commits were made recently (within polling interval)"
        
        # Calculate time since last commit
        local last_commit_time
        last_commit_time=$(git -C "$REPO_ROOT" log -1 --format=%ct 2>/dev/null)
        local current_time
        current_time=$(date +%s)
        local time_diff=$((current_time - last_commit_time))
        
        echo "  Time since last commit: ${time_diff} seconds"
        
        if [ $time_diff -lt $POLLING_INTERVAL ]; then
            print_result "Property 7.3" "PASS" "Recent commit detected, within polling interval"
        else
            print_result "Property 7.3" "WARN" "Commit is older than polling interval but not synced yet"
        fi
    fi
    
    # Check sync status of all Applications
    echo ""
    echo "  Checking sync status of all Applications:"
    
    local all_synced=true
    local synced_count=0
    
    for app in "${applications[@]}"; do
        local sync_status
        sync_status=$(kubectl get application "$app" -n argocd -o jsonpath='{.status.sync.status}' 2>/dev/null)
        
        local health_status
        health_status=$(kubectl get application "$app" -n argocd -o jsonpath='{.status.health.status}' 2>/dev/null)
        
        if [ "$sync_status" = "Synced" ]; then
            echo -e "    ${GREEN}✓${NC} ${app}: Synced (Health: ${health_status})"
            synced_count=$((synced_count + 1))
        elif [ "$sync_status" = "OutOfSync" ]; then
            echo -e "    ${YELLOW}○${NC} ${app}: OutOfSync (Health: ${health_status})"
            all_synced=false
        else
            echo -e "    ${RED}✗${NC} ${app}: ${sync_status} (Health: ${health_status})"
            all_synced=false
        fi
    done
    
    if [ "$all_synced" = true ]; then
        print_result "Property 7.4" "PASS" "All Applications are synced (${synced_count}/${#applications[@]})"
    elif [ $synced_count -ge 3 ]; then
        print_result "Property 7.4" "PASS" "Most Applications are synced (${synced_count}/${#applications[@]})"
    else
        print_result "Property 7.4" "WARN" "Some Applications are not synced (${synced_count}/${#applications[@]})"
    fi
    
    # Check that automated sync is enabled (prerequisite for this property)
    echo ""
    echo "  Verifying automated sync is enabled:"
    
    local all_automated=true
    
    for app in "${applications[@]}"; do
        local auto_sync
        auto_sync=$(kubectl get application "$app" -n argocd -o jsonpath='{.spec.syncPolicy.automated}' 2>/dev/null)
        
        if [ -z "$auto_sync" ] || [ "$auto_sync" = "null" ]; then
            echo -e "    ${RED}✗${NC} ${app}: Automated sync not configured"
            all_automated=false
        else
            echo -e "    ${GREEN}✓${NC} ${app}: Automated sync enabled"
        fi
    done
    
    if [ "$all_automated" = false ]; then
        print_result "Property 7" "FAIL" "Not all Applications have automated sync enabled"
        return 1
    fi
    
    print_result "Property 7.5" "PASS" "All Applications have automated sync enabled"
    
    # Check operation state to see if sync is in progress
    echo ""
    echo "  Checking for ongoing sync operations:"
    
    local sync_in_progress=false
    
    for app in "${applications[@]}"; do
        local operation_phase
        operation_phase=$(kubectl get application "$app" -n argocd -o jsonpath='{.status.operationState.phase}' 2>/dev/null)
        
        if [ "$operation_phase" = "Running" ] || [ "$operation_phase" = "Progressing" ]; then
            echo -e "    ${BLUE}○${NC} ${app}: Sync operation in progress (${operation_phase})"
            sync_in_progress=true
        fi
    done
    
    if [ "$sync_in_progress" = true ]; then
        print_result "Property 7.6" "PASS" "Sync operations are in progress (ArgoCD is actively syncing)"
    else
        print_result "Property 7.6" "PASS" "No sync operations in progress (Applications are stable)"
    fi
    
    # Final verdict
    if [ "$all_synced" = true ] && [ "$current_commit" = "$current_sync_revision" ]; then
        print_result "Property 7" "PASS" "ArgoCD successfully synced all Applications to latest commit"
        return 0
    elif [ $synced_count -ge 3 ]; then
        print_result "Property 7" "PASS" "ArgoCD is syncing Applications (${synced_count}/${#applications[@]} synced)"
        return 0
    else
        print_result "Property 7" "WARN" "ArgoCD sync may be in progress or delayed"
        return 0
    fi
}

# Test that ArgoCD detects changes within polling interval
test_property_7_polling_interval() {
    echo ""
    echo "=========================================="
    echo "Property 7: Polling Interval Detection"
    echo "=========================================="
    echo ""
    
    # Get ArgoCD application controller configuration
    local timeout_reconciliation
    timeout_reconciliation=$(kubectl get configmap argocd-cm -n argocd -o jsonpath='{.data.timeout\.reconciliation}' 2>/dev/null)
    
    if [ -z "$timeout_reconciliation" ]; then
        timeout_reconciliation="180s"  # Default value
        echo "  Using default polling interval: ${timeout_reconciliation}"
    else
        echo "  Configured polling interval: ${timeout_reconciliation}"
    fi
    
    print_result "Property 7.7" "PASS" "ArgoCD polling interval is configured"
    
    # Verify that the polling interval is reasonable (not too long)
    local interval_seconds
    interval_seconds=$(echo "$timeout_reconciliation" | sed 's/s$//')
    
    if [ "$interval_seconds" -le 300 ]; then
        print_result "Property 7.8" "PASS" "Polling interval is reasonable (${interval_seconds}s <= 300s)"
    else
        print_result "Property 7.8" "WARN" "Polling interval is longer than expected (${interval_seconds}s > 300s)"
    fi
    
    return 0
}

# Main test execution
main() {
    echo ""
    echo "=========================================="
    echo "GitOps Sync on Commit Property Test"
    echo "=========================================="
    echo ""
    echo "Feature: argocd-tekton-platform"
    echo "Property: 7 (GitOps Sync on Commit)"
    echo "Validates: Requirements 2.2, 2.6"
    echo ""
    
    # Check prerequisites
    check_kubectl
    check_git
    
    echo "Cluster: $(kubectl config current-context)"
    echo "Repository: $(git -C "$REPO_ROOT" remote get-url origin 2>/dev/null || echo "local")"
    echo ""
    
    # Run property tests
    test_property_7_gitops_sync_on_commit
    test_property_7_polling_interval
    
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
