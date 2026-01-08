#!/usr/bin/env bash

set -euo pipefail

# Arbiter Pipeline Infrastructure Bootstrap Script
# 
# This script provides zero-touch convergence for the authentication platform:
# 1. Creates/selects Kubernetes cluster
# 2. Generates ALL secrets automatically
# 3. Installs ArgoCD
# 4. Creates root Application (ArgoCD takes over from here)
# 5. Waits for Authentik and creates API token
# 6. Platform converges automatically via GitOps
#
# Usage:
#   ./bootstrap.sh [--cluster-name NAME] [--use-existing] [--show-secrets]
#
# Options:
#   --cluster-name NAME    Name for Kind cluster (default: platform-cluster)
#   --use-existing         Use existing kubecontext instead of creating cluster
#   --show-secrets         Display generated secrets (WARNING: not for production)

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
CLUSTER_NAME="${CLUSTER_NAME:-platform-cluster}"
USE_EXISTING=false
SHOW_SECRETS=false
ARGOCD_VERSION="v2.9.3"
ARGOCD_NAMESPACE="argocd"
AUTH_NAMESPACE="auth-system"

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --cluster-name)
      CLUSTER_NAME="$2"
      shift 2
      ;;
    --use-existing)
      USE_EXISTING=true
      shift
      ;;
    --show-secrets)
      SHOW_SECRETS=true
      shift
      ;;
    *)
      echo -e "${RED}Unknown option: $1${NC}"
      echo "Usage: $0 [--cluster-name NAME] [--use-existing] [--show-secrets]"
      exit 1
      ;;
  esac
done

# Helper functions
log_info() {
  echo -e "${BLUE}ℹ${NC} $1"
}

log_success() {
  echo -e "${GREEN}✓${NC} $1"
}

log_warning() {
  echo -e "${YELLOW}⚠${NC}  $1"
}

log_error() {
  echo -e "${RED}✗${NC} $1"
}

check_prerequisites() {
  log_info "Checking prerequisites..."
  
  local missing_tools=()
  
  if ! command -v kubectl &> /dev/null; then
    missing_tools+=("kubectl")
  fi
  
  if ! command -v openssl &> /dev/null; then
    missing_tools+=("openssl")
  fi
  
  if ! command -v base64 &> /dev/null; then
    missing_tools+=("base64")
  fi
  
  if ! command -v curl &> /dev/null; then
    missing_tools+=("curl")
  fi
  
  if ! command -v jq &> /dev/null; then
    missing_tools+=("jq")
  fi
  
  if [[ "$USE_EXISTING" == false ]] && ! command -v kind &> /dev/null; then
    missing_tools+=("kind")
  fi
  
  if [[ ${#missing_tools[@]} -gt 0 ]]; then
    log_error "Missing required tools: ${missing_tools[*]}"
    log_info "Please install missing tools and try again"
    exit 1
  fi
  
  log_success "All prerequisites satisfied"
}

create_or_select_cluster() {
  if [[ "$USE_EXISTING" == true ]]; then
    log_info "Using existing kubecontext: $(kubectl config current-context)"
    
    # Verify cluster is reachable
    if ! kubectl cluster-info &> /dev/null; then
      log_error "Cannot connect to cluster"
      exit 1
    fi
    
    log_success "Cluster is reachable"
  else
    log_info "Creating Kind cluster: $CLUSTER_NAME"
    
    # Check if cluster already exists
    if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
      log_warning "Cluster $CLUSTER_NAME already exists"
      log_info "Using existing cluster"
    else
      # Create cluster with config
      kind create cluster --name "$CLUSTER_NAME" --config platform/bootstrap/kind-cluster-config.yaml
      log_success "Created Kind cluster: $CLUSTER_NAME"
    fi
    
    # Set kubecontext
    kubectl config use-context "kind-${CLUSTER_NAME}"
    log_success "Switched to kubecontext: kind-${CLUSTER_NAME}"
  fi
  
  # Verify cluster connectivity
  log_info "Verifying cluster connectivity..."
  if ! kubectl cluster-info &> /dev/null; then
    log_error "Cannot connect to cluster"
    exit 1
  fi
  log_success "Cluster is reachable"
}

generate_secrets() {
  log_info "Generating secrets..."
  
  # Generate PostgreSQL credentials
  POSTGRES_PASSWORD=$(openssl rand -base64 32)
  POSTGRES_SUPERUSER_PASSWORD=$(openssl rand -base64 32)
  
  # Generate Authentik secrets
  AUTHENTIK_SECRET_KEY=$(openssl rand -base64 50)
  AUTHENTIK_ADMIN_PASSWORD=$(openssl rand -base64 32)
  AUTHENTIK_BOOTSTRAP_TOKEN=$(openssl rand -hex 32)
  
  # Generate Dex client secret
  DEX_CLIENT_SECRET=$(openssl rand -base64 32)
  
  # Generate ArgoCD, Tekton, and Kubernetes client secrets
  ARGOCD_CLIENT_SECRET=$(openssl rand -base64 32)
  TEKTON_CLIENT_SECRET=$(openssl rand -base64 32)
  KUBERNETES_CLIENT_SECRET=$(openssl rand -base64 32)
  
  log_success "Generated all secrets"
  
  if [[ "$SHOW_SECRETS" == true ]]; then
    log_warning "WARNING: Displaying secrets (--show-secrets enabled)"
    log_warning "Do not use in production or shared environments"
    echo ""
    echo "PostgreSQL password: $POSTGRES_PASSWORD"
    echo "PostgreSQL superuser password: $POSTGRES_SUPERUSER_PASSWORD"
    echo "Authentik secret key: $AUTHENTIK_SECRET_KEY"
    echo "Authentik admin password: $AUTHENTIK_ADMIN_PASSWORD"
    echo "Authentik bootstrap token: $AUTHENTIK_BOOTSTRAP_TOKEN"
    echo "Dex client secret (for Authentik): $DEX_CLIENT_SECRET"
    echo "ArgoCD OIDC client secret: $ARGOCD_CLIENT_SECRET"
    echo "Tekton Dashboard OIDC client secret: $TEKTON_CLIENT_SECRET"
    echo "Kubernetes API OIDC client secret: $KUBERNETES_CLIENT_SECRET"
    echo ""
  fi
}

create_namespaces() {
  log_info "Creating namespaces..."
  
  # Create auth-system namespace (required for secrets)
  if ! kubectl get namespace "$AUTH_NAMESPACE" &> /dev/null; then
    kubectl create namespace "$AUTH_NAMESPACE"
    log_success "Created namespace: $AUTH_NAMESPACE"
  else
    log_info "Namespace $AUTH_NAMESPACE already exists"
  fi
  
  # Create tekton-pipelines namespace (required for Tekton Dashboard secret)
  if ! kubectl get namespace "tekton-pipelines" &> /dev/null; then
    kubectl create namespace "tekton-pipelines"
    log_success "Created namespace: tekton-pipelines"
  else
    log_info "Namespace tekton-pipelines already exists"
  fi
}

create_auth_system_secrets() {
  log_info "Creating auth-system secrets..."
  
  # Check if secrets already exist (production override support)
  local secrets_exist=false
  
  # Auth-system namespace secrets
  if kubectl get secret authentik-postgresql -n "$AUTH_NAMESPACE" &> /dev/null; then
    log_info "Secret authentik-postgresql already exists (skipping)"
    secrets_exist=true
  else
    kubectl create secret generic authentik-postgresql \
      -n "$AUTH_NAMESPACE" \
      --from-literal=postgresql-password="$POSTGRES_PASSWORD" \
      --from-literal=postgresql-postgres-password="$POSTGRES_SUPERUSER_PASSWORD"
    log_success "Created secret: authentik-postgresql (in $AUTH_NAMESPACE)"
  fi
  
  if kubectl get secret authentik-secrets -n "$AUTH_NAMESPACE" &> /dev/null; then
    log_info "Secret authentik-secrets already exists (skipping)"
    secrets_exist=true
  else
    kubectl create secret generic authentik-secrets \
      -n "$AUTH_NAMESPACE" \
      --from-literal=secret-key="$AUTHENTIK_SECRET_KEY" \
      --from-literal=admin-password="$AUTHENTIK_ADMIN_PASSWORD" \
      --from-literal=bootstrap-token="$AUTHENTIK_BOOTSTRAP_TOKEN"
    log_success "Created secret: authentik-secrets (in $AUTH_NAMESPACE)"
  fi
  
  if kubectl get secret dex-secrets -n "$AUTH_NAMESPACE" &> /dev/null; then
    log_info "Secret dex-secrets already exists (skipping)"
    secrets_exist=true
  else
    kubectl create secret generic dex-secrets \
      -n "$AUTH_NAMESPACE" \
      --from-literal=client-secret="$DEX_CLIENT_SECRET" \
      --from-literal=authentik-client-secret="$DEX_CLIENT_SECRET" \
      --from-literal=argocd-client-secret="$ARGOCD_CLIENT_SECRET" \
      --from-literal=tekton-client-secret="$TEKTON_CLIENT_SECRET" \
      --from-literal=kubernetes-client-secret="$KUBERNETES_CLIENT_SECRET"
    log_success "Created secret: dex-secrets (in $AUTH_NAMESPACE)"
  fi
  
  if [[ "$secrets_exist" == true ]]; then
    log_warning "Some secrets already existed (production override mode)"
  fi
}

create_argocd_and_tekton_secrets() {
  log_info "Creating ArgoCD and Tekton secrets..."
  
  # ArgoCD namespace secret (must be in argocd namespace)
  # ArgoCD references $oidc.dex.clientSecret which looks for oidc.dex.clientSecret key in argocd-secret
  if kubectl get secret argocd-secret -n "$ARGOCD_NAMESPACE" &> /dev/null; then
    log_info "Secret argocd-secret exists, patching with OIDC client secret..."
    # Patch existing argocd-secret to add oidc.dex.clientSecret key
    kubectl patch secret argocd-secret -n "$ARGOCD_NAMESPACE" \
      --type merge \
      -p "{\"data\":{\"oidc.dex.clientSecret\":\"$(echo -n "$ARGOCD_CLIENT_SECRET" | base64)\"}}"
    log_success "Patched secret: argocd-secret with oidc.dex.clientSecret (in $ARGOCD_NAMESPACE)"
  else
    # Create argocd-secret if it doesn't exist (shouldn't happen, but handle it)
    kubectl create secret generic argocd-secret \
      -n "$ARGOCD_NAMESPACE" \
      --from-literal=oidc.dex.clientSecret="$ARGOCD_CLIENT_SECRET"
    log_success "Created secret: argocd-secret with oidc.dex.clientSecret (in $ARGOCD_NAMESPACE)"
  fi
  
  # Tekton namespace secret (must be in tekton-pipelines namespace)
  if kubectl get secret tekton-dashboard-oidc -n "tekton-pipelines" &> /dev/null; then
    log_info "Secret tekton-dashboard-oidc already exists (skipping)"
  else
    kubectl create secret generic tekton-dashboard-oidc \
      -n "tekton-pipelines" \
      --from-literal=client-secret="$TEKTON_CLIENT_SECRET"
    log_success "Created secret: tekton-dashboard-oidc (in tekton-pipelines)"
  fi
}

install_argocd() {
  log_info "Installing ArgoCD..."
  
  # Create ArgoCD namespace
  if ! kubectl get namespace "$ARGOCD_NAMESPACE" &> /dev/null; then
    kubectl create namespace "$ARGOCD_NAMESPACE"
    log_success "Created namespace: $ARGOCD_NAMESPACE"
  else
    log_info "Namespace $ARGOCD_NAMESPACE already exists"
  fi
  
  # Install ArgoCD
  if ! kubectl get deployment argocd-server -n "$ARGOCD_NAMESPACE" &> /dev/null; then
    kubectl apply -n "$ARGOCD_NAMESPACE" -f "https://raw.githubusercontent.com/argoproj/argo-cd/${ARGOCD_VERSION}/manifests/install.yaml"
    log_success "Applied ArgoCD manifests"
  else
    log_info "ArgoCD already installed"
  fi
  
  # Wait for ArgoCD to be ready
  log_info "Waiting for ArgoCD to be ready (this may take a few minutes)..."
  kubectl wait --for=condition=available --timeout=300s \
    deployment/argocd-server \
    deployment/argocd-repo-server \
    deployment/argocd-applicationset-controller \
    -n "$ARGOCD_NAMESPACE"
  
  log_success "ArgoCD is ready"
}

create_root_application() {
  log_info "Creating root ArgoCD Application..."
  
  # Apply root application
  kubectl apply -f platform/argocd/apps/platform-root.yaml
  
  log_success "Created root Application (app-of-apps pattern)"
  log_info "ArgoCD will now sync all platform Applications automatically"
}

wait_for_config_sync_job() {
  log_info "Config Sync Job will be created by ArgoCD (GitOps-managed)"
  log_info "Monitor with: kubectl logs -n auth-system job/auth-config-sync -f"
}

validate_oidc_configuration() {
  log_info "OIDC validation will be available after Dex deployment"
  echo ""
  echo "To validate OIDC configuration after platform converges:"
  echo ""
  echo "1. Verify Dex OIDC discovery endpoint:"
  echo "   curl https://dex.home.local/.well-known/openid-configuration"
  echo ""
  echo "2. Verify kube-apiserver can reach Dex (from within cluster):"
  echo "   kubectl run -it --rm debug --image=curlimages/curl --restart=Never -- \\"
  echo "     curl -v https://dex.home.local/.well-known/openid-configuration"
  echo ""
  echo "3. Verify JWKS endpoint:"
  echo "   curl https://dex.home.local/keys"
  echo ""
  echo "4. Test break-glass access:"
  echo "   kubectl --kubeconfig /etc/kubernetes/admin.conf get nodes"
  echo ""
}

print_access_instructions() {
  echo ""
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo -e "${GREEN}✓ Bootstrap Complete!${NC}"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo ""
  echo -e "${BLUE}ArgoCD is now managing all platform components via GitOps.${NC}"
  echo ""
  echo "The platform will converge automatically:"
  echo "  • ArgoCD syncs all Applications (Tekton, auth-system, controllers)"
  echo "  • Config Sync Job runs automatically (has API token)"
  echo "  • Dex scales from 0 to 1 replica after Authentik is configured"
  echo "  • Platform becomes fully functional"
  echo ""
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo -e "${YELLOW}Next Steps:${NC}"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo ""
  echo "1. Access ArgoCD UI:"
  echo "   kubectl port-forward svc/argocd-server -n argocd 8080:443"
  echo "   Open: https://localhost:8080"
  echo ""
  echo "2. Get ArgoCD admin password:"
  echo "   kubectl get secret argocd-initial-admin-secret -n argocd -o jsonpath='{.data.password}' | base64 -d"
  echo ""
  echo "3. Monitor ArgoCD Application sync status:"
  echo "   kubectl get applications -n argocd"
  echo ""
  echo "4. Check Config Sync Job status:"
  echo "   kubectl get job auth-config-sync -n auth-system"
  echo "   kubectl logs job/auth-config-sync -n auth-system"
  echo ""
  echo "5. Verify Dex scaled to 1 replica:"
  echo "   kubectl get deployment dex -n auth-system"
  echo ""
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo -e "${YELLOW}Retrieve Secrets (requires RBAC permissions):${NC}"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo ""
  echo "Authentik admin password:"
  echo "  kubectl get secret authentik-secrets -n auth-system -o jsonpath='{.data.admin-password}' | base64 -d"
  echo ""
  echo "Dex client secret:"
  echo "  kubectl get secret dex-secrets -n auth-system -o jsonpath='{.data.client-secret}' | base64 -d"
  echo ""
  echo "ArgoCD OIDC client secret:"
  echo "  kubectl get secret argocd-secret -n argocd -o jsonpath='{.data.oidc\.dex\.clientSecret}' | base64 -d"
  echo ""
  echo "Tekton Dashboard OIDC client secret:"
  echo "  kubectl get secret tekton-dashboard-oidc -n tekton-pipelines -o jsonpath='{.data.client-secret}' | base64 -d"
  echo ""
  echo "Authentik bootstrap token:"
  echo "  kubectl get secret authentik-secrets -n auth-system -o jsonpath='{.data.bootstrap-token}' | base64 -d"
  echo ""
  echo "PostgreSQL password:"
  echo "  kubectl get secret authentik-postgresql -n auth-system -o jsonpath='{.data.postgresql-password}' | base64 -d"
  echo ""
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo ""
}

main() {
  echo ""
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo -e "${BLUE}Arbiter Pipeline Infrastructure Bootstrap${NC}"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo ""
  
  check_prerequisites
  create_or_select_cluster
  generate_secrets
  create_namespaces
  create_auth_system_secrets
  install_argocd
  create_argocd_and_tekton_secrets
  create_root_application
  wait_for_config_sync_job
  validate_oidc_configuration
  print_access_instructions
}

main "$@"
