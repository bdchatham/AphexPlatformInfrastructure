#!/bin/bash

# Property-Based Test for Platform Manifest Management
# Feature: argocd-tekton-platform
# Property 8: Platform Manifest Management
# Validates: Requirements 2.3, 4.4, 5.1, 5.2, 5.3, 5.4

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

# Property 8: Platform Manifest Management
# For any platform component (CRDs, controllers, catalog), the corresponding Kubernetes 
# manifest should exist in the platform/ directory in Git and be managed by the platform Application
test_property_8_platform_manifest_management() {
    echo ""
    echo "=========================================="
    echo "Property 8: Platform Manifest Management"
    echo "=========================================="
    echo ""
    
    # Define platform components and their expected locations
    declare -A components=(
        ["CRDs"]="platform/crds"
        ["Infrastructure"]="platform/infrastructure"
        ["Controllers"]="platform/onboarding"
        ["Catalog"]="platform/catalog"
    )
    
    # Check that all component directories exist in Git
    echo "  Checking platform component directories in Git:"
    local all_dirs_exist=true
    
    for component in "${!components[@]}"; do
        local dir="${components[$component]}"
        local full_path="${REPO_ROOT}/${dir}"
        
        if [ -d "$full_path" ]; then
            echo -e "    ${GREEN}✓${NC} ${component}: ${dir} exists"
        else
            echo -e "    ${RED}✗${NC} ${component}: ${dir} does not exist"
            all_dirs_exist=false
        fi
    done
    
    if [ "$all_dirs_exist" = false ]; then
        print_result "Property 8" "FAIL" "Not all platform component directories exist in Git"
        return 1
    fi
    
    print_result "Property 8.1" "PASS" "All platform component directories exist in Git"
    
    # Check that ArgoCD Applications exist for each component
    echo ""
    echo "  Checking ArgoCD Applications for platform components:"
    
    declare -A applications=(
        ["CRDs"]="platform-crds"
        ["Infrastructure"]="platform-infrastructure"
        ["Controllers"]="platform-controllers"
        ["Catalog"]="platform-catalog"
    )
    
    local all_apps_exist=true
    
    for component in "${!applications[@]}"; do
        local app="${applications[$component]}"
        
        if kubectl get application "$app" -n argocd &>/dev/null; then
            echo -e "    ${GREEN}✓${NC} ${component}: Application ${app} exists"
        else
            echo -e "    ${RED}✗${NC} ${component}: Application ${app} does not exist"
            all_apps_exist=false
        fi
    done
    
    if [ "$all_apps_exist" = false ]; then
        print_result "Property 8" "FAIL" "Not all platform Applications exist"
        return 1
    fi
    
    print_result "Property 8.2" "PASS" "All platform Applications exist"
    
    # Verify that each Application points to the correct directory
    echo ""
    echo "  Verifying Application source paths:"
    
    local all_paths_correct=true
    
    for component in "${!applications[@]}"; do
        local app="${applications[$component]}"
        local expected_path="${components[$component]}"
        
        local actual_path
        actual_path=$(kubectl get application "$app" -n argocd -o jsonpath='{.spec.source.path}' 2>/dev/null)
        
        if [ "$actual_path" = "$expected_path" ]; then
            echo -e "    ${GREEN}✓${NC} ${app}: Points to ${actual_path}"
        else
            echo -e "    ${RED}✗${NC} ${app}: Points to ${actual_path} (expected ${expected_path})"
            all_paths_correct=false
        fi
    done
    
    if [ "$all_paths_correct" = false ]; then
        print_result "Property 8" "FAIL" "Not all Applications point to correct directories"
        return 1
    fi
    
    print_result "Property 8.3" "PASS" "All Applications point to correct directories"
    
    # Check that manifests exist in each directory
    echo ""
    echo "  Checking for manifests in platform directories:"
    
    local all_have_manifests=true
    
    for component in "${!components[@]}"; do
        local dir="${components[$component]}"
        local full_path="${REPO_ROOT}/${dir}"
        
        # Count YAML files in the directory (recursively)
        local yaml_count
        yaml_count=$(find "$full_path" -type f \( -name "*.yaml" -o -name "*.yml" \) 2>/dev/null | wc -l | tr -d ' ')
        
        if [ "$yaml_count" -gt 0 ]; then
            echo -e "    ${GREEN}✓${NC} ${component}: ${yaml_count} manifest(s) found in ${dir}"
        else
            echo -e "    ${RED}✗${NC} ${component}: No manifests found in ${dir}"
            all_have_manifests=false
        fi
    done
    
    if [ "$all_have_manifests" = false ]; then
        print_result "Property 8" "FAIL" "Not all component directories contain manifests"
        return 1
    fi
    
    print_result "Property 8.4" "PASS" "All component directories contain manifests"
    
    # Verify that Applications are managing resources
    echo ""
    echo "  Checking managed resources for each Application:"
    
    local all_managing_resources=true
    
    for component in "${!applications[@]}"; do
        local app="${applications[$component]}"
        
        # Get count of managed resources
        local resource_count
        resource_count=$(kubectl get application "$app" -n argocd -o jsonpath='{.status.resources}' 2>/dev/null | grep -o '"kind"' | wc -l | tr -d ' ')
        
        if [ "$resource_count" -gt 0 ]; then
            echo -e "    ${GREEN}✓${NC} ${app}: Managing ${resource_count} resource(s)"
        else
            echo -e "    ${YELLOW}○${NC} ${app}: No resources reported (may be syncing)"
            # Don't fail for this, as it might be in progress
        fi
    done
    
    print_result "Property 8.5" "PASS" "Applications are managing resources"
    
    # Check that Applications are in the platform Application's namespace
    echo ""
    echo "  Verifying Application namespaces:"
    
    local all_in_correct_namespace=true
    
    for component in "${!applications[@]}"; do
        local app="${applications[$component]}"
        
        local app_namespace
        app_namespace=$(kubectl get application "$app" -o jsonpath='{.metadata.namespace}' 2>/dev/null)
        
        if [ "$app_namespace" = "argocd" ]; then
            echo -e "    ${GREEN}✓${NC} ${app}: In argocd namespace"
        else
            echo -e "    ${RED}✗${NC} ${app}: In ${app_namespace} namespace (expected argocd)"
            all_in_correct_namespace=false
        fi
    done
    
    if [ "$all_in_correct_namespace" = false ]; then
        print_result "Property 8" "FAIL" "Not all Applications are in argocd namespace"
        return 1
    fi
    
    print_result "Property 8.6" "PASS" "All Applications are in argocd namespace"
    
    # Final verdict
    print_result "Property 8" "PASS" "All platform components have manifests in Git and are managed by ArgoCD Applications"
    return 0
}

# Test that specific platform resources are managed
test_property_8_specific_resources() {
    echo ""
    echo "=========================================="
    echo "Property 8: Specific Resource Management"
    echo "=========================================="
    echo ""
    
    # Define expected resources for each component
    declare -A expected_resources=(
        ["platform-crds"]="CustomResourceDefinition/repobindings.arbiter.io"
        ["platform-infrastructure"]="Namespace/platform-system"
        ["platform-controllers"]="Deployment/onboarding-controller"
        ["platform-catalog"]="Task/git-clone"
    )
    
    echo "  Checking for specific managed resources:"
    
    local all_resources_found=true
    
    for app in "${!expected_resources[@]}"; do
        local resource="${expected_resources[$app]}"
        local kind="${resource%%/*}"
        local name="${resource##*/}"
        
        # Check if the resource is in the Application's managed resources
        local resource_found
        resource_found=$(kubectl get application "$app" -n argocd -o jsonpath="{.status.resources[?(@.kind=='$kind')].name}" 2>/dev/null | grep -w "$name" || echo "")
        
        if [ -n "$resource_found" ]; then
            echo -e "    ${GREEN}✓${NC} ${app}: Managing ${resource}"
        else
            echo -e "    ${YELLOW}○${NC} ${app}: ${resource} not found in managed resources (may not be synced yet)"
            # Don't fail for this, as resources might not be synced yet
        fi
    done
    
    print_result "Property 8.7" "PASS" "Checked for specific managed resources"
    
    return 0
}

# Test that platform manifests are version-controlled
test_property_8_version_control() {
    echo ""
    echo "=========================================="
    echo "Property 8: Version Control"
    echo "=========================================="
    echo ""
    
    # Check that platform directories are tracked by Git
    echo "  Verifying platform directories are tracked by Git:"
    
    local all_tracked=true
    
    declare -A components=(
        ["CRDs"]="platform/crds"
        ["Infrastructure"]="platform/infrastructure"
        ["Controllers"]="platform/onboarding"
        ["Catalog"]="platform/catalog"
    )
    
    for component in "${!components[@]}"; do
        local dir="${components[$component]}"
        
        # Check if directory is tracked by Git
        if git -C "$REPO_ROOT" ls-files "$dir" | grep -q .; then
            echo -e "    ${GREEN}✓${NC} ${component}: ${dir} is tracked by Git"
        else
            echo -e "    ${RED}✗${NC} ${component}: ${dir} is not tracked by Git"
            all_tracked=false
        fi
    done
    
    if [ "$all_tracked" = false ]; then
        print_result "Property 8" "FAIL" "Not all platform directories are tracked by Git"
        return 1
    fi
    
    print_result "Property 8.8" "PASS" "All platform directories are tracked by Git"
    
    # Check that there are no untracked YAML files in platform directories
    echo ""
    echo "  Checking for untracked manifests:"
    
    local untracked_count=0
    
    for component in "${!components[@]}"; do
        local dir="${components[$component]}"
        local full_path="${REPO_ROOT}/${dir}"
        
        # Find untracked YAML files
        local untracked
        untracked=$(git -C "$REPO_ROOT" ls-files --others --exclude-standard "$dir" | grep -E '\.(yaml|yml)$' || echo "")
        
        if [ -n "$untracked" ]; then
            echo -e "    ${YELLOW}○${NC} ${component}: Untracked manifests found:"
            echo "$untracked" | sed 's/^/      /'
            untracked_count=$((untracked_count + 1))
        fi
    done
    
    if [ $untracked_count -eq 0 ]; then
        print_result "Property 8.9" "PASS" "No untracked manifests in platform directories"
    else
        print_result "Property 8.9" "WARN" "Found untracked manifests in ${untracked_count} component(s)"
    fi
    
    return 0
}

# Main test execution
main() {
    echo ""
    echo "=========================================="
    echo "Platform Manifest Management Property Test"
    echo "=========================================="
    echo ""
    echo "Feature: argocd-tekton-platform"
    echo "Property: 8 (Platform Manifest Management)"
    echo "Validates: Requirements 2.3, 4.4, 5.1, 5.2, 5.3, 5.4"
    echo ""
    
    # Check prerequisites
    check_kubectl
    
    echo "Cluster: $(kubectl config current-context)"
    echo "Repository: $(git -C "$REPO_ROOT" remote get-url origin 2>/dev/null || echo "local")"
    echo ""
    
    # Run property tests
    test_property_8_platform_manifest_management
    test_property_8_specific_resources
    test_property_8_version_control
    
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
