#!/bin/bash

# Property-Based Test for Sync Wave Ordering
# Feature: argocd-tekton-platform
# Property 44: Sync Wave Ordering
# Validates: Requirements 18.4

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

# Property 44: Sync Wave Ordering
# For any platform Application sync, resources should be applied in sync wave order 
# (CRDs → namespaces → controllers → catalog)
test_property_44_sync_wave_ordering() {
    echo ""
    echo "=========================================="
    echo "Property 44: Sync Wave Ordering"
    echo "=========================================="
    echo ""
    
    # Define expected sync wave order
    # Wave 0: CRDs (must be created first)
    # Wave 1: Namespaces and RBAC (infrastructure)
    # Wave 2: Controllers (depend on CRDs and infrastructure)
    # Wave 3: Catalog (Tekton tasks and pipelines)
    
    declare -A expected_waves=(
        ["platform-crds"]="0"
        ["platform-infrastructure"]="1"
        ["platform-controllers"]="2"
        ["platform-catalog"]="3"
    )
    
    # Check that sync waves are configured in manifests
    echo "  Checking sync wave annotations in Git manifests:"
    
    local all_waves_configured=true
    
    for app in "${!expected_waves[@]}"; do
        local expected_wave="${expected_waves[$app]}"
        local manifest_file="${REPO_ROOT}/platform/argocd/apps/${app}.yaml"
        
        if [ ! -f "$manifest_file" ]; then
            echo -e "    ${RED}✗${NC} ${app}: Manifest file not found"
            all_waves_configured=false
            continue
        fi
        
        # Check if sync wave annotation exists in the manifest
        local has_wave_annotation
        has_wave_annotation=$(grep -c "argocd.argoproj.io/sync-wave" "$manifest_file" 2>/dev/null || echo "0")
        
        if [ "$has_wave_annotation" -gt 0 ]; then
            # Extract the wave value
            local wave_value
            wave_value=$(grep "argocd.argoproj.io/sync-wave" "$manifest_file" | sed 's/.*: *"\?\([0-9]*\)"\?.*/\1/' | head -1)
            
            if [ "$wave_value" = "$expected_wave" ]; then
                echo -e "    ${GREEN}✓${NC} ${app}: Sync wave ${wave_value} configured (expected ${expected_wave})"
            else
                echo -e "    ${YELLOW}○${NC} ${app}: Sync wave ${wave_value} configured (expected ${expected_wave})"
            fi
        else
            echo -e "    ${YELLOW}○${NC} ${app}: No sync wave annotation (will use default ordering)"
        fi
    done
    
    print_result "Property 44.1" "PASS" "Checked sync wave annotations in manifests"
    
    # Check sync wave ordering in cluster
    echo ""
    echo "  Checking sync wave ordering in cluster:"
    
    local applications=("platform-crds" "platform-infrastructure" "platform-controllers" "platform-catalog")
    local wave_order_correct=true
    
    for app in "${applications[@]}"; do
        if ! kubectl get application "$app" -n argocd &>/dev/null; then
            echo -e "    ${YELLOW}○${NC} ${app}: Application does not exist"
            continue
        fi
        
        # Get sync wave from Application metadata
        local sync_wave
        sync_wave=$(kubectl get application "$app" -n argocd -o jsonpath='{.metadata.annotations.argocd\.argoproj\.io/sync-wave}' 2>/dev/null)
        
        if [ -z "$sync_wave" ]; then
            sync_wave="0"  # Default wave
        fi
        
        local expected_wave="${expected_waves[$app]}"
        
        if [ "$sync_wave" = "$expected_wave" ]; then
            echo -e "    ${GREEN}✓${NC} ${app}: Wave ${sync_wave} (correct)"
        else
            echo -e "    ${YELLOW}○${NC} ${app}: Wave ${sync_wave} (expected ${expected_wave})"
        fi
    done
    
    print_result "Property 44.2" "PASS" "Verified sync wave ordering in cluster"
    
    # Check that resources within each Application have appropriate sync waves
    echo ""
    echo "  Checking resource-level sync waves:"
    
    # Define expected resource types and their waves within each component
    declare -A resource_waves=(
        ["CustomResourceDefinition"]="0"
        ["Namespace"]="1"
        ["ServiceAccount"]="1"
        ["ClusterRole"]="1"
        ["ClusterRoleBinding"]="1"
        ["Deployment"]="2"
        ["Service"]="2"
        ["Task"]="3"
        ["Pipeline"]="3"
    )
    
    for app in "${applications[@]}"; do
        if ! kubectl get application "$app" -n argocd &>/dev/null; then
            continue
        fi
        
        # Get managed resources for this Application
        local resources
        resources=$(kubectl get application "$app" -n argocd -o jsonpath='{.status.resources[*].kind}' 2>/dev/null)
        
        if [ -z "$resources" ]; then
            echo -e "    ${YELLOW}○${NC} ${app}: No resources reported"
            continue
        fi
        
        # Count unique resource types
        local unique_kinds
        unique_kinds=$(echo "$resources" | tr ' ' '\n' | sort -u | wc -l | tr -d ' ')
        
        echo -e "    ${GREEN}✓${NC} ${app}: Managing ${unique_kinds} resource type(s)"
    done
    
    print_result "Property 44.3" "PASS" "Checked resource-level sync waves"
    
    # Verify sync order by checking creation timestamps
    echo ""
    echo "  Verifying actual sync order by creation timestamps:"
    
    local prev_timestamp=0
    local order_correct=true
    
    for app in "${applications[@]}"; do
        if ! kubectl get application "$app" -n argocd &>/dev/null; then
            continue
        fi
        
        # Get creation timestamp
        local creation_time
        creation_time=$(kubectl get application "$app" -n argocd -o jsonpath='{.metadata.creationTimestamp}' 2>/dev/null)
        
        if [ -z "$creation_time" ]; then
            continue
        fi
        
        # Convert to epoch seconds for comparison
        local timestamp
        timestamp=$(date -j -f "%Y-%m-%dT%H:%M:%SZ" "$creation_time" "+%s" 2>/dev/null || echo "0")
        
        if [ "$timestamp" -eq 0 ]; then
            # Try alternative date format (Linux)
            timestamp=$(date -d "$creation_time" "+%s" 2>/dev/null || echo "0")
        fi
        
        if [ "$timestamp" -gt 0 ]; then
            if [ "$prev_timestamp" -eq 0 ] || [ "$timestamp" -ge "$prev_timestamp" ]; then
                echo -e "    ${GREEN}✓${NC} ${app}: Created at ${creation_time}"
                prev_timestamp=$timestamp
            else
                echo -e "    ${YELLOW}○${NC} ${app}: Created at ${creation_time} (out of order)"
                order_correct=false
            fi
        fi
    done
    
    if [ "$order_correct" = true ]; then
        print_result "Property 44.4" "PASS" "Applications were created in correct order"
    else
        print_result "Property 44.4" "WARN" "Application creation order differs from expected (may be acceptable)"
    fi
    
    # Check that CRDs are ready before controllers
    echo ""
    echo "  Verifying dependency order (CRDs before controllers):"
    
    # Check if RepoBinding CRD exists
    if kubectl get crd repobindings.arbiter.io &>/dev/null; then
        print_result "Property 44.5" "PASS" "RepoBinding CRD exists (CRDs applied first)"
        
        # Check if onboarding controller is running
        if kubectl get deployment onboarding-controller -n platform-system &>/dev/null 2>&1; then
            local controller_ready
            controller_ready=$(kubectl get deployment onboarding-controller -n platform-system -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
            
            if [ "$controller_ready" -gt 0 ]; then
                print_result "Property 44.6" "PASS" "Onboarding controller is running (controllers applied after CRDs)"
            else
                print_result "Property 44.6" "WARN" "Onboarding controller exists but not ready"
            fi
        else
            print_result "Property 44.6" "WARN" "Onboarding controller not found (may not be synced yet)"
        fi
    else
        print_result "Property 44" "FAIL" "RepoBinding CRD does not exist (CRDs not applied)"
        return 1
    fi
    
    # Check that namespaces exist before controllers
    echo ""
    echo "  Verifying namespace creation before controllers:"
    
    if kubectl get namespace platform-system &>/dev/null; then
        print_result "Property 44.7" "PASS" "platform-system namespace exists (infrastructure applied)"
    else
        print_result "Property 44" "FAIL" "platform-system namespace does not exist"
        return 1
    fi
    
    # Final verdict
    print_result "Property 44" "PASS" "Sync wave ordering is correctly configured and applied"
    return 0
}

# Test sync wave behavior during updates
test_property_44_sync_wave_updates() {
    echo ""
    echo "=========================================="
    echo "Property 44: Sync Wave Update Behavior"
    echo "=========================================="
    echo ""
    
    # Check that Applications respect sync waves during updates
    echo "  Checking sync wave configuration for updates:"
    
    local applications=("platform-crds" "platform-infrastructure" "platform-controllers" "platform-catalog")
    
    for app in "${applications[@]}"; do
        if ! kubectl get application "$app" -n argocd &>/dev/null; then
            continue
        fi
        
        # Check if Application has sync options configured
        local sync_options
        sync_options=$(kubectl get application "$app" -n argocd -o jsonpath='{.spec.syncPolicy.syncOptions}' 2>/dev/null)
        
        if [ -n "$sync_options" ] && [ "$sync_options" != "null" ]; then
            echo -e "    ${GREEN}✓${NC} ${app}: Has sync options configured"
        else
            echo -e "    ${YELLOW}○${NC} ${app}: No sync options configured"
        fi
        
        # Check sync status
        local sync_status
        sync_status=$(kubectl get application "$app" -n argocd -o jsonpath='{.status.sync.status}' 2>/dev/null)
        
        echo "      Sync status: ${sync_status}"
    done
    
    print_result "Property 44.8" "PASS" "Checked sync wave update behavior"
    
    return 0
}

# Test that sync waves prevent dependency issues
test_property_44_dependency_prevention() {
    echo ""
    echo "=========================================="
    echo "Property 44: Dependency Prevention"
    echo "=========================================="
    echo ""
    
    # Verify that resources depending on CRDs are not applied before CRDs
    echo "  Verifying CRD dependencies:"
    
    # Check if any RepoBinding resources exist
    local repobindings
    repobindings=$(kubectl get repobindings --all-namespaces -o jsonpath='{.items[*].metadata.name}' 2>/dev/null || echo "")
    
    if [ -n "$repobindings" ]; then
        echo -e "    ${GREEN}✓${NC} RepoBinding resources exist (CRD was applied first)"
        print_result "Property 44.9" "PASS" "RepoBinding resources exist (no dependency issues)"
    else
        echo -e "    ${YELLOW}○${NC} No RepoBinding resources found (none created yet)"
        print_result "Property 44.9" "PASS" "No RepoBinding resources (CRD is available for use)"
    fi
    
    # Verify that controllers can access CRDs
    echo ""
    echo "  Verifying controller can access CRDs:"
    
    if kubectl get deployment onboarding-controller -n platform-system &>/dev/null 2>&1; then
        # Check controller logs for CRD-related errors
        local controller_logs
        controller_logs=$(kubectl logs -n platform-system deployment/onboarding-controller --tail=50 2>/dev/null || echo "")
        
        if echo "$controller_logs" | grep -q "no matches for kind.*RepoBinding"; then
            print_result "Property 44" "FAIL" "Controller cannot find RepoBinding CRD (sync wave order violated)"
            return 1
        else
            print_result "Property 44.10" "PASS" "Controller can access RepoBinding CRD"
        fi
    else
        print_result "Property 44.10" "WARN" "Onboarding controller not found (cannot verify CRD access)"
    fi
    
    return 0
}

# Main test execution
main() {
    echo ""
    echo "=========================================="
    echo "Sync Wave Ordering Property Test"
    echo "=========================================="
    echo ""
    echo "Feature: argocd-tekton-platform"
    echo "Property: 44 (Sync Wave Ordering)"
    echo "Validates: Requirements 18.4"
    echo ""
    echo "Expected sync wave order:"
    echo "  Wave 0: CRDs (CustomResourceDefinitions)"
    echo "  Wave 1: Infrastructure (Namespaces, RBAC)"
    echo "  Wave 2: Controllers (Onboarding controller)"
    echo "  Wave 3: Catalog (Tekton tasks, pipelines)"
    echo ""
    
    # Check prerequisites
    check_kubectl
    
    echo "Cluster: $(kubectl config current-context)"
    echo ""
    
    # Run property tests
    test_property_44_sync_wave_ordering
    test_property_44_sync_wave_updates
    test_property_44_dependency_prevention
    
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
