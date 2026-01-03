#!/bin/bash

# Validation script for templates and CRDs
# This script validates that all required templates and CRDs exist and are properly structured

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test counters
CHECKS_RUN=0
CHECKS_PASSED=0
CHECKS_FAILED=0

# Function to print check results
print_result() {
    local check_name="$1"
    local result="$2"
    local message="$3"
    
    CHECKS_RUN=$((CHECKS_RUN + 1))
    
    if [ "$result" = "PASS" ]; then
        echo -e "${GREEN}✓${NC} ${check_name}: ${message}"
        CHECKS_PASSED=$((CHECKS_PASSED + 1))
    else
        echo -e "${RED}✗${NC} ${check_name}: ${message}"
        CHECKS_FAILED=$((CHECKS_FAILED + 1))
    fi
}

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

echo ""
echo "=========================================="
echo "Template and CRD Validation"
echo "=========================================="
echo ""
echo "Repository: ${REPO_ROOT}"
echo ""

# Check if yq is available for YAML validation
if command -v yq &> /dev/null; then
    HAS_YQ=true
    echo "Using yq for YAML validation"
else
    HAS_YQ=false
    echo -e "${YELLOW}Warning: yq not found, skipping detailed YAML validation${NC}"
fi

echo ""

# Validate RepoBinding CRD
echo "Checking RepoBinding CRD..."
CRD_FILE="${REPO_ROOT}/platform/crds/repobinding-crd.yaml"

if [ ! -f "$CRD_FILE" ]; then
    print_result "CRD File" "FAIL" "RepoBinding CRD file does not exist"
else
    print_result "CRD File" "PASS" "RepoBinding CRD file exists"
    
    # Check for required fields in CRD
    if grep -q "kind: CustomResourceDefinition" "$CRD_FILE"; then
        print_result "CRD Kind" "PASS" "CRD has correct kind"
    else
        print_result "CRD Kind" "FAIL" "CRD missing kind: CustomResourceDefinition"
    fi
    
    if grep -q "group: arbiter.io" "$CRD_FILE"; then
        print_result "CRD Group" "PASS" "CRD has correct group (arbiter.io)"
    else
        print_result "CRD Group" "FAIL" "CRD missing or incorrect group"
    fi
    
    if grep -q "kind: RepoBinding" "$CRD_FILE"; then
        print_result "CRD Resource Kind" "PASS" "CRD defines RepoBinding kind"
    else
        print_result "CRD Resource Kind" "FAIL" "CRD missing RepoBinding kind"
    fi
    
    # Check for required spec fields
    if grep -q "repoOrg:" "$CRD_FILE"; then
        print_result "CRD Spec" "PASS" "CRD defines repoOrg field"
    else
        print_result "CRD Spec" "FAIL" "CRD missing repoOrg field"
    fi
    
    if grep -q "repoName:" "$CRD_FILE"; then
        print_result "CRD Spec" "PASS" "CRD defines repoName field"
    else
        print_result "CRD Spec" "FAIL" "CRD missing repoName field"
    fi
    
    if grep -q "tenantName:" "$CRD_FILE"; then
        print_result "CRD Spec" "PASS" "CRD defines tenantName field"
    else
        print_result "CRD Spec" "FAIL" "CRD missing tenantName field"
    fi
    
    # Check for status fields
    if grep -q "webhookURL:" "$CRD_FILE"; then
        print_result "CRD Status" "PASS" "CRD defines webhookURL status field"
    else
        print_result "CRD Status" "FAIL" "CRD missing webhookURL status field"
    fi
    
    if grep -q "webhookSecret:" "$CRD_FILE"; then
        print_result "CRD Status" "PASS" "CRD defines webhookSecret status field"
    else
        print_result "CRD Status" "FAIL" "CRD missing webhookSecret status field"
    fi
    
    # Check for sync-wave annotation
    if grep -q "argocd.argoproj.io/sync-wave" "$CRD_FILE"; then
        print_result "CRD Annotations" "PASS" "CRD has sync-wave annotation"
    else
        print_result "CRD Annotations" "FAIL" "CRD missing sync-wave annotation"
    fi
fi

echo ""

# Validate tenant templates
echo "Checking tenant templates..."
TEMPLATE_DIR="${REPO_ROOT}/platform/tenancy/templates"

REQUIRED_TEMPLATES=(
    "namespace.yaml"
    "service-account.yaml"
    "role-standard.yaml"
    "role-elevated.yaml"
    "rolebinding.yaml"
    "resourcequota.yaml"
    "limitrange.yaml"
    "networkpolicy.yaml"
    "eventlistener.yaml"
    "ingress.yaml"
    "terraform-backend-secret.yaml"
)

for template in "${REQUIRED_TEMPLATES[@]}"; do
    template_file="${TEMPLATE_DIR}/${template}"
    
    if [ ! -f "$template_file" ]; then
        print_result "Template" "FAIL" "${template} does not exist"
    else
        print_result "Template" "PASS" "${template} exists"
        
        # Check for placeholder variables
        if grep -q "{{TENANT_NAME}}" "$template_file"; then
            print_result "Template Placeholder" "PASS" "${template} has TENANT_NAME placeholder"
        else
            print_result "Template Placeholder" "FAIL" "${template} missing TENANT_NAME placeholder"
        fi
        
        # Check for arbiter.io labels
        if grep -q "arbiter.io/tenant" "$template_file"; then
            print_result "Template Labels" "PASS" "${template} has arbiter.io/tenant label"
        else
            print_result "Template Labels" "FAIL" "${template} missing arbiter.io/tenant label"
        fi
    fi
done

echo ""

# Validate example RepoBinding
echo "Checking example RepoBinding..."
EXAMPLE_FILE="${REPO_ROOT}/platform/crds/example-repobinding.yaml"

if [ ! -f "$EXAMPLE_FILE" ]; then
    print_result "Example" "FAIL" "Example RepoBinding file does not exist"
else
    print_result "Example" "PASS" "Example RepoBinding file exists"
    
    if grep -q "kind: RepoBinding" "$EXAMPLE_FILE"; then
        print_result "Example Kind" "PASS" "Example has correct kind"
    else
        print_result "Example Kind" "FAIL" "Example missing kind: RepoBinding"
    fi
    
    if grep -q "repoOrg:" "$EXAMPLE_FILE"; then
        print_result "Example Spec" "PASS" "Example defines repoOrg"
    else
        print_result "Example Spec" "FAIL" "Example missing repoOrg"
    fi
fi

echo ""

# Validate ArgoCD Applications
echo "Checking ArgoCD Application manifests..."
ARGOCD_APPS_DIR="${REPO_ROOT}/platform/argocd/apps"

REQUIRED_APPS=(
    "platform-root.yaml"
    "platform-crds.yaml"
    "platform-infrastructure.yaml"
    "platform-controllers.yaml"
    "platform-catalog.yaml"
)

for app in "${REQUIRED_APPS[@]}"; do
    app_file="${ARGOCD_APPS_DIR}/${app}"
    
    if [ ! -f "$app_file" ]; then
        print_result "ArgoCD App" "FAIL" "${app} does not exist"
    else
        print_result "ArgoCD App" "PASS" "${app} exists"
        
        # Check for automated sync policy
        if grep -q "automated:" "$app_file"; then
            print_result "App Sync Policy" "PASS" "${app} has automated sync policy"
        else
            print_result "App Sync Policy" "FAIL" "${app} missing automated sync policy"
        fi
        
        # Check for prune and selfHeal
        if grep -q "prune: true" "$app_file"; then
            print_result "App Prune" "PASS" "${app} has prune enabled"
        else
            print_result "App Prune" "FAIL" "${app} missing prune: true"
        fi
        
        if grep -q "selfHeal: true" "$app_file"; then
            print_result "App SelfHeal" "PASS" "${app} has selfHeal enabled"
        else
            print_result "App SelfHeal" "FAIL" "${app} missing selfHeal: true"
        fi
    fi
done

echo ""

# Print summary
echo "=========================================="
echo "Validation Summary"
echo "=========================================="
echo ""
echo "Checks run: ${CHECKS_RUN}"
echo -e "${GREEN}Checks passed: ${CHECKS_PASSED}${NC}"
if [ $CHECKS_FAILED -gt 0 ]; then
    echo -e "${RED}Checks failed: ${CHECKS_FAILED}${NC}"
else
    echo "Checks failed: ${CHECKS_FAILED}"
fi
echo ""

if [ $CHECKS_FAILED -gt 0 ]; then
    echo -e "${RED}Validation failed - some templates or CRDs have issues${NC}"
    exit 1
else
    echo -e "${GREEN}All validation checks passed${NC}"
    exit 0
fi
