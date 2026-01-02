#!/bin/bash

# Platform Bootstrap Verification Script
# This script verifies all components of the Jenkins X platform are properly deployed

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo "=========================================="
echo "Jenkins X Platform Verification"
echo "=========================================="
echo ""

# Track overall status
ERRORS=0
WARNINGS=0

# Function to check component
check_component() {
    local name=$1
    local check_command=$2
    local required=$3  # "required" or "optional"
    
    echo -n "Checking ${name}... "
    if eval "$check_command" &> /dev/null; then
        echo -e "${GREEN}✓ OK${NC}"
        return 0
    else
        if [ "$required" = "required" ]; then
            echo -e "${RED}✗ FAILED${NC}"
            ERRORS=$((ERRORS + 1))
            return 1
        else
            echo -e "${YELLOW}⚠ NOT FOUND (optional)${NC}"
            WARNINGS=$((WARNINGS + 1))
            return 1
        fi
    fi
}

# Function to check pod health
check_pods() {
    local namespace=$1
    local label=$2
    local name=$3
    local required=$4
    
    echo -n "Checking ${name} pods... "
    if kubectl get pods -n "$namespace" -l "$label" 2>/dev/null | grep -q "Running"; then
        local ready_count=$(kubectl get pods -n "$namespace" -l "$label" -o jsonpath='{.items[*].status.containerStatuses[*].ready}' 2>/dev/null | grep -o "true" | wc -l)
        local total_count=$(kubectl get pods -n "$namespace" -l "$label" --no-headers 2>/dev/null | wc -l)
        echo -e "${GREEN}✓ OK${NC} (${ready_count}/${total_count} ready)"
        return 0
    else
        if [ "$required" = "required" ]; then
            echo -e "${RED}✗ FAILED${NC}"
            ERRORS=$((ERRORS + 1))
            return 1
        else
            echo -e "${YELLOW}⚠ NOT FOUND (optional)${NC}"
            WARNINGS=$((WARNINGS + 1))
            return 1
        fi
    fi
}

echo "=========================================="
echo "1. Namespaces"
echo "=========================================="
echo ""

check_component "pipeline-system namespace" "kubectl get namespace pipeline-system" "required"
check_component "pipeline-catalog namespace" "kubectl get namespace pipeline-catalog" "required"
check_component "auth-system namespace" "kubectl get namespace auth-system" "optional" || true
check_component "tekton-pipelines namespace" "kubectl get namespace tekton-pipelines" "required"

echo ""
echo "=========================================="
echo "2. Tekton Pipelines"
echo "=========================================="
echo ""

check_pods "tekton-pipelines" "app.kubernetes.io/name=controller" "tekton-pipelines-controller" "required" || true
check_pods "tekton-pipelines" "app.kubernetes.io/name=webhook" "tekton-pipelines-webhook" "required" || true
check_pods "tekton-pipelines" "app.kubernetes.io/name=events-controller" "tekton-events-controller" "optional" || true

echo ""
echo "=========================================="
echo "3. Dex (OIDC Provider)"
echo "=========================================="
echo ""

check_component "Dex deployment" "kubectl get deployment dex -n auth-system" "optional" || true
check_pods "auth-system" "app=dex" "Dex" "optional" || true
check_component "Dex service" "kubectl get service dex -n auth-system" "optional" || true

if kubectl get namespace auth-system &> /dev/null; then
    echo ""
    echo "Dex configuration:"
    kubectl get configmap dex-config -n auth-system &> /dev/null && echo -e "${GREEN}✓ dex-config ConfigMap exists${NC}" || echo -e "${YELLOW}⚠ dex-config ConfigMap not found${NC}"
fi

echo ""
echo "=========================================="
echo "4. Jenkins X and Lighthouse"
echo "=========================================="
echo ""

check_component "jx-build-controller deployment" "kubectl get deployment jx-build-controller -n pipeline-system" "optional" || true
check_pods "pipeline-system" "app=jx-build-controller" "jx-build-controller" "optional" || true

check_component "Lighthouse deployment" "kubectl get deployment lighthouse -n pipeline-system" "optional" || true
check_pods "pipeline-system" "app=lighthouse" "Lighthouse" "optional" || true

if kubectl get deployment lighthouse -n pipeline-system &> /dev/null; then
    echo ""
    echo "Lighthouse configuration:"
    kubectl get configmap lighthouse-config -n pipeline-system &> /dev/null && echo -e "${GREEN}✓ lighthouse-config ConfigMap exists${NC}" || echo -e "${YELLOW}⚠ lighthouse-config ConfigMap not found${NC}"
    kubectl get configmap repo-allowlist -n pipeline-system &> /dev/null && echo -e "${GREEN}✓ repo-allowlist ConfigMap exists${NC}" || echo -e "${YELLOW}⚠ repo-allowlist ConfigMap not found${NC}"
    kubectl get secret lighthouse-github-app -n pipeline-system &> /dev/null && echo -e "${GREEN}✓ lighthouse-github-app Secret exists${NC}" || echo -e "${YELLOW}⚠ lighthouse-github-app Secret not found${NC}"
fi

echo ""
echo "=========================================="
echo "5. Onboarding Controller"
echo "=========================================="
echo ""

check_component "RepoBinding CRD" "kubectl get crd repobindings.platform.arbiter.io" "required" || true
check_component "Onboarding controller deployment" "kubectl get deployment onboarding-controller -n pipeline-system" "required" || true
check_pods "pipeline-system" "app=onboarding-controller" "Onboarding controller" "required" || true

echo ""
echo "Onboarding controller RBAC:"
kubectl get serviceaccount onboarding-controller -n pipeline-system &> /dev/null && echo -e "${GREEN}✓ ServiceAccount exists${NC}" || echo -e "${RED}✗ ServiceAccount not found${NC}"
kubectl get clusterrole onboarding-controller &> /dev/null && echo -e "${GREEN}✓ ClusterRole exists${NC}" || echo -e "${RED}✗ ClusterRole not found${NC}"
kubectl get clusterrolebinding onboarding-controller &> /dev/null && echo -e "${GREEN}✓ ClusterRoleBinding exists${NC}" || echo -e "${RED}✗ ClusterRoleBinding not found${NC}"

echo ""
echo "=========================================="
echo "6. Pipeline Catalog"
echo "=========================================="
echo ""

echo "Checking Tekton Tasks in pipeline-catalog namespace:"
TASK_COUNT=$(kubectl get tasks -n pipeline-catalog --no-headers 2>/dev/null | wc -l)
if [ "$TASK_COUNT" -gt 0 ]; then
    echo -e "${GREEN}✓ ${TASK_COUNT} Tasks found${NC}"
    kubectl get tasks -n pipeline-catalog --no-headers | awk '{print "  - " $1}'
else
    echo -e "${RED}✗ No Tasks found${NC}"
    ERRORS=$((ERRORS + 1))
fi

echo ""
echo "Checking Tekton Pipelines in pipeline-catalog namespace:"
PIPELINE_COUNT=$(kubectl get pipelines -n pipeline-catalog --no-headers 2>/dev/null | wc -l)
if [ "$PIPELINE_COUNT" -gt 0 ]; then
    echo -e "${GREEN}✓ ${PIPELINE_COUNT} Pipelines found${NC}"
    kubectl get pipelines -n pipeline-catalog --no-headers | awk '{print "  - " $1}'
else
    echo -e "${RED}✗ No Pipelines found${NC}"
    ERRORS=$((ERRORS + 1))
fi

echo ""
echo "=========================================="
echo "7. Runner Image"
echo "=========================================="
echo ""

# Check if runner image exists in local registry
echo "Checking runner image availability:"
echo -e "${YELLOW}⚠ Manual verification required${NC}"
echo "  Run: docker images | grep runner"
echo "  Expected: ghcr.io/bdchatham/pipeline-runner:latest or similar"

echo ""
echo "=========================================="
echo "Summary"
echo "=========================================="
echo ""

if [ $ERRORS -eq 0 ] && [ $WARNINGS -eq 0 ]; then
    echo -e "${GREEN}✓ All components verified successfully!${NC}"
    echo ""
    echo "Platform is ready for use."
    exit 0
elif [ $ERRORS -eq 0 ]; then
    echo -e "${YELLOW}⚠ Verification completed with ${WARNINGS} warnings${NC}"
    echo ""
    echo "Core components are operational, but some optional components are missing:"
    echo "  - Dex (OIDC provider) - Required for OIDC authentication"
    echo "  - Lighthouse - Required for Git webhook handling"
    echo "  - jx-build-controller - Required for Jenkins X integration"
    echo ""
    echo "To install missing components:"
    echo "  - Dex: kubectl apply -f platform/bootstrap/namespace-auth-system.yaml"
    echo "         kubectl apply -f platform/bootstrap/dex-deployment.yaml"
    echo "  - Lighthouse: ./platform/bootstrap/install-jenkinsx.sh"
    exit 0
else
    echo -e "${RED}✗ Verification failed with ${ERRORS} errors and ${WARNINGS} warnings${NC}"
    echo ""
    echo "Please review the errors above and fix the issues."
    echo ""
    echo "Common issues:"
    echo "  - Missing namespaces: kubectl apply -f platform/bootstrap/namespace-*.yaml"
    echo "  - Missing Tekton: ./platform/bootstrap/bootstrap.sh"
    echo "  - Missing onboarding controller: kubectl apply -f platform/onboarding/"
    echo "  - Missing pipeline catalog: kubectl apply -f platform/catalog/"
    exit 1
fi
