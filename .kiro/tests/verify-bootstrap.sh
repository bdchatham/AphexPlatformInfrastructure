#!/bin/bash

# Bootstrap Verification Script
# Verifies that all platform components are running and ArgoCD Applications are syncing
# Requirements: 13.5

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
ARGOCD_NAMESPACE="argocd"
TEKTON_NAMESPACE="tekton-pipelines"
PLATFORM_NAMESPACE="platform-system"

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

check_namespace() {
    local namespace=$1
    
    if kubectl get namespace "$namespace" &>/dev/null; then
        log_info "✓ Namespace '$namespace' exists"
        return 0
    else
        log_error "✗ Namespace '$namespace' NOT found"
        return 1
    fi
}

check_deployment_ready() {
    local deployment=$1
    local namespace=$2
    
    if ! kubectl get deployment "$deployment" -n "$namespace" &>/dev/null; then
        log_error "✗ Deployment '$deployment' NOT found in namespace '$namespace'"
        return 1
    fi
    
    local ready=$(kubectl get deployment "$deployment" -n "$namespace" -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
    local desired=$(kubectl get deployment "$deployment" -n "$namespace" -o jsonpath='{.spec.replicas}' 2>/dev/null || echo "1")
    
    if [ "$ready" = "$desired" ] && [ "$ready" != "0" ]; then
        log_info "✓ Deployment '$deployment' is ready ($ready/$desired replicas)"
        return 0
    else
        log_error "✗ Deployment '$deployment' is NOT ready ($ready/$desired replicas)"
        return 1
    fi
}

check_application_exists() {
    local app_name=$1
    
    if kubectl get application "$app_name" -n "$ARGOCD_NAMESPACE" &>/dev/null; then
        log_info "✓ ArgoCD Application '$app_name' exists"
        return 0
    else
        log_error "✗ ArgoCD Application '$app_name' NOT found"
        return 1
    fi
}

check_application_synced() {
    local app_name=$1
    
    local sync_status=$(kubectl get application "$app_name" -n "$ARGOCD_NAMESPACE" -o jsonpath='{.status.sync.status}' 2>/dev/null || echo "Unknown")
    local health_status=$(kubectl get application "$app_name" -n "$ARGOCD_NAMESPACE" -o jsonpath='{.status.health.status}' 2>/dev/null || echo "Unknown")
    
    if [ "$sync_status" = "Synced" ]; then
        log_info "✓ Application '$app_name' is synced (Health: $health_status)"
        return 0
    else
        log_warn "Application '$app_name' sync status: $sync_status (Health: $health_status)"
        return 1
    fi
}

# Main verification
main() {
    log_section "ArgoCD + Tekton Platform Bootstrap Verification"
    
    echo ""
    echo "This script verifies:"
    echo "  1. All platform namespaces exist"
    echo "  2. Tekton components are running"
    echo "  3. ArgoCD components are running"
    echo "  4. ArgoCD Applications exist and are syncing"
    echo ""
    
    # Check 1: Verify platform namespaces
    log_section "1. Verifying Platform Namespaces"
    
    check_namespace "$ARGOCD_NAMESPACE"
    check_namespace "$TEKTON_NAMESPACE"
    check_namespace "$PLATFORM_NAMESPACE"
    
    # Check 2: Verify Tekton components
    log_section "2. Verifying Tekton Components"
    
    log_info "Checking Tekton Pipelines..."
    check_deployment_ready "tekton-pipelines-controller" "$TEKTON_NAMESPACE"
    check_deployment_ready "tekton-pipelines-webhook" "$TEKTON_NAMESPACE"
    
    log_info ""
    log_info "Checking Tekton Triggers..."
    check_deployment_ready "tekton-triggers-controller" "$TEKTON_NAMESPACE"
    check_deployment_ready "tekton-triggers-webhook" "$TEKTON_NAMESPACE"
    
    # Check 3: Verify ArgoCD components
    log_section "3. Verifying ArgoCD Components"
    
    check_deployment_ready "argocd-server" "$ARGOCD_NAMESPACE"
    check_deployment_ready "argocd-application-controller" "$ARGOCD_NAMESPACE"
    check_deployment_ready "argocd-repo-server" "$ARGOCD_NAMESPACE"
    check_deployment_ready "argocd-dex-server" "$ARGOCD_NAMESPACE" || log_warn "Dex server not found (optional)"
    
    # Check 4: Verify ArgoCD Applications
    log_section "4. Verifying ArgoCD Applications"
    
    log_info "Checking root Application..."
    check_application_exists "platform-root"
    check_application_synced "platform-root" || log_warn "Root Application not yet synced"
    
    echo ""
    log_info "Checking child Applications..."
    
    local child_apps=("platform-crds" "platform-infrastructure" "platform-controllers" "platform-catalog")
    local all_synced=true
    
    for app in "${child_apps[@]}"; do
        if check_application_exists "$app"; then
            if ! check_application_synced "$app"; then
                all_synced=false
            fi
        else
            all_synced=false
        fi
    done
    
    # Summary
    log_section "Verification Summary"
    
    if [ $OVERALL_STATUS -eq 0 ]; then
        echo -e "${GREEN}✓ ALL CHECKS PASSED${NC}"
        echo ""
        echo "The platform is ready for use:"
        echo "  ✓ All namespaces exist"
        echo "  ✓ Tekton components are running"
        echo "  ✓ ArgoCD components are running"
        echo "  ✓ ArgoCD Applications are configured"
        echo ""
        echo "Next steps:"
        echo "  1. Access ArgoCD UI:"
        echo "     kubectl port-forward svc/argocd-server -n argocd 8080:443"
        echo "     Open: http://localhost:8080"
        echo ""
        echo "  2. Get ArgoCD admin password:"
        echo "     kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath=\"{.data.password}\" | base64 -d"
        echo ""
        echo "  3. Monitor Application sync:"
        echo "     kubectl get applications -n argocd"
        echo "     watch kubectl get applications -n argocd"
        echo ""
        echo "  4. Register a repository:"
        echo "     kubectl apply -f platform/crds/example-repobinding.yaml"
    else
        echo -e "${RED}✗ SOME CHECKS FAILED${NC}"
        echo ""
        echo "Please review the errors above and troubleshoot:"
        echo ""
        echo "Common issues:"
        echo "  1. Components not ready yet - wait a few minutes and re-run"
        echo "  2. Check pod status: kubectl get pods -n <namespace>"
        echo "  3. Check pod logs: kubectl logs -n <namespace> <pod-name>"
        echo "  4. Check Application status: kubectl describe application <app-name> -n argocd"
        echo ""
        echo "For ArgoCD sync issues:"
        echo "  kubectl get application platform-root -n argocd -o yaml"
        echo "  argocd app get platform-root"
    fi
    
    echo ""
    exit $OVERALL_STATUS
}

# Run main function
main "$@"
