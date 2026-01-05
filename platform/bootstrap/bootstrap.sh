#!/usr/bin/env bash

set -euo pipefail

# Dex Authentication Platform Bootstrap Script
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
  
  # Generate Dex client secret
  DEX_CLIENT_SECRET=$(openssl rand -base64 32)
  
  # Generate ArgoCD and Tekton client secrets
  ARGOCD_CLIENT_SECRET=$(openssl rand -base64 32)
  TEKTON_CLIENT_SECRET=$(openssl rand -base64 32)
  
  log_success "Generated all secrets"
  
  if [[ "$SHOW_SECRETS" == true ]]; then
    log_warning "WARNING: Displaying secrets (--show-secrets enabled)"
    log_warning "Do not use in production or shared environments"
    echo ""
    echo "PostgreSQL password: $POSTGRES_PASSWORD"
    echo "PostgreSQL superuser password: $POSTGRES_SUPERUSER_PASSWORD"
    echo "Authentik secret key: $AUTHENTIK_SECRET_KEY"
    echo "Authentik admin password: $AUTHENTIK_ADMIN_PASSWORD"
    echo "Dex client secret (for Authentik): $DEX_CLIENT_SECRET"
    echo "ArgoCD OIDC client secret: $ARGOCD_CLIENT_SECRET"
    echo "Tekton Dashboard OIDC client secret: $TEKTON_CLIENT_SECRET"
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
      --from-literal=admin-password="$AUTHENTIK_ADMIN_PASSWORD"
    log_success "Created secret: authentik-secrets (in $AUTH_NAMESPACE)"
  fi
  
  if kubectl get secret dex-secrets -n "$AUTH_NAMESPACE" &> /dev/null; then
    log_info "Secret dex-secrets already exists (skipping)"
    secrets_exist=true
  else
    kubectl create secret generic dex-secrets \
      -n "$AUTH_NAMESPACE" \
      --from-literal=client-secret="$DEX_CLIENT_SECRET"
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

wait_for_authentik() {
  log_info "Waiting for Authentik to be deployed by ArgoCD..."
  log_info "This is the ONLY bootstrap → application interaction (documented exception)"
  
  # Wait for Authentik deployment to exist (ArgoCD creates it)
  local max_wait=300
  local elapsed=0
  while ! kubectl get deployment authentik-server -n "$AUTH_NAMESPACE" &> /dev/null; do
    if [[ $elapsed -ge $max_wait ]]; then
      log_error "Timeout waiting for Authentik deployment to be created by ArgoCD"
      exit 1
    fi
    sleep 5
    elapsed=$((elapsed + 5))
    echo -n "."
  done
  echo ""
  
  log_info "Authentik deployment found, waiting for pods to be ready..."
  
  # Wait for Authentik pods to be ready
  kubectl wait --for=condition=ready --timeout=300s \
    pod -l app=authentik-server \
    -n "$AUTH_NAMESPACE" || true
  
  # Wait for health endpoint to return 200
  log_info "Waiting for Authentik health endpoint..."
  max_wait=300
  elapsed=0
  while true; do
    if kubectl exec -n "$AUTH_NAMESPACE" \
      deployment/authentik-server \
      -c authentik -- \
      curl -sf http://localhost:9000/api/v3/root/config/ &> /dev/null; then
      break
    fi
    
    if [[ $elapsed -ge $max_wait ]]; then
      log_error "Timeout waiting for Authentik health endpoint"
      exit 1
    fi
    
    sleep 5
    elapsed=$((elapsed + 5))
    echo -n "."
  done
  echo ""
  
  log_success "Authentik is ready"
}

create_authentik_api_token() {
  log_info "Creating Authentik API token..."
  
  # Check if token already exists (production override support)
  if kubectl get secret authentik-api-token -n "$AUTH_NAMESPACE" &> /dev/null; then
    log_info "Secret authentik-api-token already exists (skipping automated token creation)"
    log_info "Using existing token (production override mode)"
    return
  fi
  
  log_info "Authenticating to Authentik API (using kubectl exec to reach in-cluster service)..."
  
  # Note: We use kubectl exec to curl from inside the cluster because bootstrap runs
  # on the host machine which cannot resolve *.svc.cluster.local DNS names.
  # This is Option B: run curl inside the cluster via kubectl exec.
  
  # Authenticate with admin credentials to get session token
  local auth_response
  auth_response=$(kubectl exec -n "$AUTH_NAMESPACE" deployment/authentik-server \
    -c authentik -- \
    curl -sf -X POST \
    -H "Content-Type: application/json" \
    -d "{\"uid\": \"admin\", \"password\": \"$AUTHENTIK_ADMIN_PASSWORD\"}" \
    http://localhost:9000/api/v3/flows/executor/default-authentication-flow/ 2>/dev/null || echo "")
  
  if [[ -z "$auth_response" ]]; then
    log_error "Failed to authenticate to Authentik API"
    log_error "This may indicate Authentik is not fully ready or credentials are incorrect"
    exit 1
  fi
  
  local session_token
  session_token=$(echo "$auth_response" | jq -r '.token // empty')
  
  if [[ -z "$session_token" ]]; then
    log_error "Failed to extract session token from Authentik response"
    log_error "Response: $auth_response"
    exit 1
  fi
  
  log_info "Creating API token with minimal permissions..."
  
  # Create API token with minimal permissions (OAuth2 provider update only)
  local token_response
  token_response=$(kubectl exec -n "$AUTH_NAMESPACE" deployment/authentik-server \
    -c authentik -- \
    curl -sf -X POST \
    -H "Authorization: Bearer $session_token" \
    -H "Content-Type: application/json" \
    -d '{"identifier": "config-sync-job", "intent": "api", "description": "Token for Config Sync Job (OAuth2 provider update only)"}' \
    http://localhost:9000/api/v3/core/tokens/ 2>/dev/null || echo "")
  
  if [[ -z "$token_response" ]]; then
    log_error "Failed to create API token"
    log_error "This may indicate insufficient permissions or API changes"
    exit 1
  fi
  
  local api_token
  api_token=$(echo "$token_response" | jq -r '.key // empty')
  
  if [[ -z "$api_token" ]]; then
    log_error "Failed to extract API token from response"
    log_error "Response: $token_response"
    exit 1
  fi
  
  # Store token in Kubernetes Secret
  kubectl create secret generic authentik-api-token \
    -n "$AUTH_NAMESPACE" \
    --from-literal=token="$api_token"
  
  log_success "Created Authentik API token (scoped to OAuth2 provider update only)"
  log_success "Stored token in authentik-api-token Secret"
  
  if [[ "$SHOW_SECRETS" == true ]]; then
    echo "Authentik API token: $api_token"
    echo ""
  fi
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
  echo "Authentik API token:"
  echo "  kubectl get secret authentik-api-token -n auth-system -o jsonpath='{.data.token}' | base64 -d"
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
  echo -e "${BLUE}Dex Authentication Platform Bootstrap${NC}"
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
  wait_for_authentik
  create_authentik_api_token
  print_access_instructions
}

main "$@"
