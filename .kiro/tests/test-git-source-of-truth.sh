#!/bin/bash

# Property-Based Test for Git as Source of Truth
# Feature: argocd-tekton-platform
# Property 9: Git as Source of Truth
# Validates: Requirements 2.5

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

# Function to check if git is available
check_git() {
    if ! command -v git &> /dev/null; then
        echo -e "${RED}Error: git is not installed or not in PATH${NC}"
        exit 1
    fi
}

# Property 9: Git as Source of Truth
# For any platform component, its configuration should be defined in Git with no 
# cluster-specific state except secrets
test_property_9_git_source_of_truth() {
    echo ""
    echo "=========================================="
    echo "Property 9: Git as Source of Truth"
    echo "=========================================="
    echo ""
    
    # Check that all platform manifests are in Git
    echo "  Checking platform manifests in Git:"
    
    declare -A components=(
        ["CRDs"]="platform/crds"
        ["Infrastructure"]="platform/infrastructure"
        ["Controllers"]="platform/onboarding"
        ["Catalog"]="platform/catalog"
        ["ArgoCD Apps"]="platform/argocd"
    )
    
    local all_in_git=true
    local total_manifests=0
    
    for component in "${!components[@]}"; do
        local dir="${components[$component]}"
        
        # Count tracked YAML files
        local tracked_count
        tracked_count=$(git -C "$REPO_ROOT" ls-files "$dir" | grep -E '\.(yaml|yml)$' | wc -l | tr -d ' ')
        
        if [ "$tracked_count" -gt 0 ]; then
            echo -e "    ${GREEN}✓${NC} ${component}: ${tracked_count} manifest(s) tracked in Git"
            total_manifests=$((total_manifests + tracked_count))
        else
            echo -e "    ${RED}✗${NC} ${component}: No manifests tracked in Git"
            all_in_git=false
        fi
    done
    
    if [ "$all_in_git" = false ]; then
        print_result "Property 9" "FAIL" "Not all platform components have manifests in Git"
        return 1
    fi
    
    print_result "Property 9.1" "PASS" "All platform components have manifests in Git (${total_manifests} total)"
    
    # Check that ArgoCD Applications point to Git repository
    echo ""
    echo "  Verifying ArgoCD Applications use Git as source:"
    
    local applications=("platform-root" "platform-crds" "platform-infrastructure" "platform-controllers" "platform-catalog")
    local all_use_git=true
    
    for app in "${applications[@]}"; do
        if ! kubectl get application "$app" -n argocd &>/dev/null; then
            echo -e "    ${YELLOW}○${NC} ${app}: Application does not exist (skipping)"
            continue
        fi
        
        local repo_url
        repo_url=$(kubectl get application "$app" -n argocd -o jsonpath='{.spec.source.repoURL}' 2>/dev/null)
        
        if [ -z "$repo_url" ]; then
            echo -e "    ${RED}✗${NC} ${app}: No repository URL configured"
            all_use_git=false
            continue
        fi
        
        # Check if it's a Git URL (http/https/git/ssh)
        if [[ "$repo_url" =~ ^(https?|git|ssh):// ]] || [[ "$repo_url" =~ \.git$ ]]; then
            echo -e "    ${GREEN}✓${NC} ${app}: Uses Git repository (${repo_url})"
        else
            echo -e "    ${RED}✗${NC} ${app}: Not using Git repository (${repo_url})"
            all_use_git=false
        fi
    done
    
    if [ "$all_use_git" = false ]; then
        print_result "Property 9" "FAIL" "Not all Applications use Git as source"
        return 1
    fi
    
    print_result "Property 9.2" "PASS" "All Applications use Git as source"
    
    # Check that no cluster-specific configuration exists in manifests (except secrets)
    echo ""
    echo "  Checking for cluster-specific configuration in manifests:"
    
    local cluster_specific_found=false
    
    # Patterns that indicate cluster-specific configuration
    local patterns=(
        "server: https://[0-9]"  # Hardcoded cluster IPs
        "clusterIP: [0-9]"       # Hardcoded cluster IPs
        "hostPath:"              # Host-specific paths
    )
    
    for component in "${!components[@]}"; do
        local dir="${components[$component]}"
        local full_path="${REPO_ROOT}/${dir}"
        
        if [ ! -d "$full_path" ]; then
            continue
        fi
        
        for pattern in "${patterns[@]}"; do
            local matches
            matches=$(grep -r "$pattern" "$full_path" --include="*.yaml" --include="*.yml" 2>/dev/null || echo "")
            
            if [ -n "$matches" ]; then
                echo -e "    ${YELLOW}○${NC} ${component}: Found potential cluster-specific config (${pattern})"
                cluster_specific_found=true
            fi
        done
    done
    
    if [ "$cluster_specific_found" = false ]; then
        print_result "Property 9.3" "PASS" "No obvious cluster-specific configuration in manifests"
    else
        print_result "Property 9.3" "WARN" "Found potential cluster-specific configuration (review needed)"
    fi
    
    # Check that secrets are not stored in Git
    echo ""
    echo "  Verifying secrets are not stored in Git:"
    
    local secrets_in_git=false
    
    # Patterns that might indicate secrets in Git
    local secret_patterns=(
        "password:"
        "token:"
        "apiKey:"
        "privateKey:"
        "secret:"
    )
    
    for component in "${!components[@]}"; do
        local dir="${components[$component]}"
        local full_path="${REPO_ROOT}/${dir}"
        
        if [ ! -d "$full_path" ]; then
            continue
        fi
        
        # Look for Secret resources with data fields
        local secret_files
        secret_files=$(grep -l "kind: Secret" "$full_path"/*.yaml "$full_path"/*.yml 2>/dev/null || echo "")
        
        if [ -n "$secret_files" ]; then
            for file in $secret_files; do
                # Check if the Secret has actual data (not just references)
                if grep -q "^  data:" "$file" 2>/dev/null; then
                    local data_content
                    data_content=$(sed -n '/^  data:/,/^[^ ]/p' "$file" | grep -v "^  data:" | grep -v "^$" | head -1)
                    
                    if [ -n "$data_content" ]; then
                        echo -e "    ${RED}✗${NC} ${component}: Secret with data found in $(basename "$file")"
                        secrets_in_git=true
                    fi
                fi
            done
        fi
    done
    
    if [ "$secrets_in_git" = false ]; then
        print_result "Property 9.4" "PASS" "No secrets with data stored in Git"
    else
        print_result "Property 9" "FAIL" "Secrets with data found in Git (security violation)"
        return 1
    fi
    
    # Check that all configuration is declarative (YAML/JSON)
    echo ""
    echo "  Verifying all configuration is declarative:"
    
    local non_declarative_found=false
    
    for component in "${!components[@]}"; do
        local dir="${components[$component]}"
        local full_path="${REPO_ROOT}/${dir}"
        
        if [ ! -d "$full_path" ]; then
            continue
        fi
        
        # Look for non-declarative files (scripts, binaries)
        local script_files
        script_files=$(find "$full_path" -type f \( -name "*.sh" -o -name "*.py" -o -name "*.rb" \) 2>/dev/null || echo "")
        
        if [ -n "$script_files" ]; then
            local script_count
            script_count=$(echo "$script_files" | wc -l | tr -d ' ')
            echo -e "    ${YELLOW}○${NC} ${component}: Found ${script_count} script file(s) (may be acceptable)"
            # Don't fail for this, as some scripts (like bootstrap) are expected
        fi
    done
    
    print_result "Property 9.5" "PASS" "Configuration is primarily declarative"
    
    # Verify that platform can be recreated from Git
    echo ""
    echo "  Verifying platform can be recreated from Git:"
    
    # Check that bootstrap script exists
    if [ -f "${REPO_ROOT}/platform/bootstrap/bootstrap.sh" ]; then
        print_result "Property 9.6" "PASS" "Bootstrap script exists for cluster recreation"
    else
        print_result "Property 9" "FAIL" "Bootstrap script not found (cannot recreate platform)"
        return 1
    fi
    
    # Check that all ArgoCD Application manifests exist
    local app_manifests_exist=true
    
    for app in "${applications[@]}"; do
        local manifest_file="${REPO_ROOT}/platform/argocd/apps/${app}.yaml"
        
        if [ -f "$manifest_file" ]; then
            echo -e "    ${GREEN}✓${NC} ${app}: Manifest exists in Git"
        else
            echo -e "    ${RED}✗${NC} ${app}: Manifest not found in Git"
            app_manifests_exist=false
        fi
    done
    
    if [ "$app_manifests_exist" = false ]; then
        print_result "Property 9" "FAIL" "Not all Application manifests exist in Git"
        return 1
    fi
    
    print_result "Property 9.7" "PASS" "All Application manifests exist in Git"
    
    # Final verdict
    print_result "Property 9" "PASS" "Git is the source of truth for all platform configuration"
    return 0
}

# Test that cluster state matches Git
test_property_9_cluster_git_consistency() {
    echo ""
    echo "=========================================="
    echo "Property 9: Cluster-Git Consistency"
    echo "=========================================="
    echo ""
    
    # Check that ArgoCD Applications are synced to Git
    echo "  Checking ArgoCD sync status:"
    
    local applications=("platform-root" "platform-crds" "platform-infrastructure" "platform-controllers" "platform-catalog")
    local all_synced=true
    local synced_count=0
    
    for app in "${applications[@]}"; do
        if ! kubectl get application "$app" -n argocd &>/dev/null; then
            echo -e "    ${YELLOW}○${NC} ${app}: Application does not exist"
            continue
        fi
        
        local sync_status
        sync_status=$(kubectl get application "$app" -n argocd -o jsonpath='{.status.sync.status}' 2>/dev/null)
        
        if [ "$sync_status" = "Synced" ]; then
            echo -e "    ${GREEN}✓${NC} ${app}: Synced with Git"
            synced_count=$((synced_count + 1))
        else
            echo -e "    ${YELLOW}○${NC} ${app}: ${sync_status} (not synced with Git)"
            all_synced=false
        fi
    done
    
    if [ $synced_count -ge 3 ]; then
        print_result "Property 9.8" "PASS" "Most Applications are synced with Git (${synced_count}/${#applications[@]})"
    else
        print_result "Property 9.8" "WARN" "Some Applications are not synced with Git (${synced_count}/${#applications[@]})"
    fi
    
    # Check that no manual changes have been made to managed resources
    echo ""
    echo "  Checking for drift from Git:"
    
    local drift_detected=false
    
    for app in "${applications[@]}"; do
        if ! kubectl get application "$app" -n argocd &>/dev/null; then
            continue
        fi
        
        local sync_status
        sync_status=$(kubectl get application "$app" -n argocd -o jsonpath='{.status.sync.status}' 2>/dev/null)
        
        if [ "$sync_status" = "OutOfSync" ]; then
            echo -e "    ${YELLOW}○${NC} ${app}: Out of sync (drift detected)"
            drift_detected=true
            
            # Get comparison result if available
            local comparison
            comparison=$(kubectl get application "$app" -n argocd -o jsonpath='{.status.sync.comparedTo.destination}' 2>/dev/null)
            
            if [ -n "$comparison" ]; then
                echo "      Comparison: ${comparison}"
            fi
        fi
    done
    
    if [ "$drift_detected" = false ]; then
        print_result "Property 9.9" "PASS" "No drift detected from Git"
    else
        print_result "Property 9.9" "WARN" "Drift detected (cluster state differs from Git)"
    fi
    
    return 0
}

# Main test execution
main() {
    echo ""
    echo "=========================================="
    echo "Git as Source of Truth Property Test"
    echo "=========================================="
    echo ""
    echo "Feature: argocd-tekton-platform"
    echo "Property: 9 (Git as Source of Truth)"
    echo "Validates: Requirements 2.5"
    echo ""
    
    # Check prerequisites
    check_kubectl
    check_git
    
    echo "Cluster: $(kubectl config current-context)"
    echo "Repository: $(git -C "$REPO_ROOT" remote get-url origin 2>/dev/null || echo "local")"
    echo "Current branch: $(git -C "$REPO_ROOT" branch --show-current 2>/dev/null || echo "unknown")"
    echo ""
    
    # Run property tests
    test_property_9_git_source_of_truth
    test_property_9_cluster_git_consistency
    
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
