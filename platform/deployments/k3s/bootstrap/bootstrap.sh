#!/usr/bin/env bash

set -euo pipefail

# Aphex Platform Bootstrap - K3s Deployment
#
# Prerequisites (must be installed manually):
#   - Ubuntu 22.04+ with NVIDIA drivers
#   - NVIDIA Container Toolkit (nvidia-container-runtime)
#
# This script will:
#   - Install K3s (if not present)
#   - Configure containerd for NVIDIA runtime
#   - Install ArgoCD
#   - Deploy platform via GitOps
#
# Usage:
#   ./bootstrap.sh [--show-secrets]

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
ARGOCD_VERSION="v2.9.3"
SHOW_SECRETS=false

# Parse arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --show-secrets)
      SHOW_SECRETS=true
      shift
      ;;
    *)
      echo -e "${RED}Unknown option: $1${NC}"
      exit 1
      ;;
  esac
done

log_info() { echo -e "${BLUE}ℹ${NC} $1"; }
log_success() { echo -e "${GREEN}✓${NC} $1"; }
log_warning() { echo -e "${YELLOW}⚠${NC}  $1"; }
log_error() { echo -e "${RED}✗${NC} $1"; }

check_prerequisites() {
  log_info "Checking prerequisites..."
  
  # Fail if K3s already installed
  if command -v k3s &> /dev/null; then
    log_error "K3s is already installed. Uninstall first with: /usr/local/bin/k3s-uninstall.sh"
    exit 1
  fi
  
  # Check for Cloudflare API token
  if [[ -z "${CLOUDFLARE_API_TOKEN:-}" ]]; then
    log_error "CLOUDFLARE_API_TOKEN environment variable not set"
    exit 1
  fi
  

  
  local missing=()
  
  # Check for root/sudo
  if [[ $EUID -ne 0 ]]; then
    log_error "This script must be run as root or with sudo"
    exit 1
  fi
  
  # Check NVIDIA driver
  if ! command -v nvidia-smi &> /dev/null; then
    missing+=("nvidia-driver (nvidia-smi not found)")
  else
    if ! nvidia-smi &> /dev/null; then
      missing+=("nvidia-driver (nvidia-smi failed)")
    fi
  fi
  
  # Check NVIDIA container runtime
  if ! command -v nvidia-container-runtime &> /dev/null; then
    missing+=("nvidia-container-toolkit")
  fi
  
  # Check basic tools
  for tool in curl openssl; do
    if ! command -v $tool &> /dev/null; then
      missing+=("$tool")
    fi
  done
  
  if [[ ${#missing[@]} -gt 0 ]]; then
    log_error "Missing prerequisites:"
    for item in "${missing[@]}"; do
      log_error "  - $item"
    done
    echo ""
    echo "Install NVIDIA Container Toolkit:"
    echo "  curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey | sudo gpg --dearmor -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg"
    echo "  curl -s -L https://nvidia.github.io/libnvidia-container/stable/deb/nvidia-container-toolkit.list | sed 's#deb https://#deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' | sudo tee /etc/apt/sources.list.d/nvidia-container-toolkit.list"
    echo "  sudo apt-get update && sudo apt-get install -y nvidia-container-toolkit"
    exit 1
  fi
  
  log_success "All prerequisites satisfied"
}

install_k3s() {
  log_info "Installing K3s..."
  curl -sfL https://get.k3s.io | sh -
  
  # Wait for K3s to be ready
  log_info "Waiting for K3s to be ready..."
  until k3s kubectl get nodes &> /dev/null; do
    sleep 2
  done
  
  log_success "K3s installed successfully"
}

configure_nvidia_runtime() {
  local k3s_containerd_config="/var/lib/rancher/k3s/agent/etc/containerd/config.toml.tmpl"
  
  mkdir -p "$(dirname "$k3s_containerd_config")"
  
  if ! nvidia-ctk runtime configure \
    --runtime=containerd \
    --config="$k3s_containerd_config" \
    --set-as-default=false 2>/dev/null; then
    log_warning "nvidia-ctk configure skipped (may already be configured)"
  fi
  log_success "NVIDIA runtime configured for k3s containerd"
}

generate_secrets() {
  log_info "Generating secrets..."
  
  POSTGRES_PASSWORD=$(openssl rand -base64 32)
  AUTHENTIK_SECRET_KEY=$(openssl rand -base64 50)
  AUTHENTIK_ADMIN_PASSWORD=$(openssl rand -base64 32)
  AUTHENTIK_BOOTSTRAP_TOKEN=$(openssl rand -hex 32)
  DEX_CLIENT_SECRET=$(openssl rand -base64 32)
  ARGOCD_CLIENT_SECRET=$(openssl rand -base64 32)
  TEKTON_CLIENT_SECRET=$(openssl rand -base64 32)
  KUBERNETES_CLIENT_SECRET=$(openssl rand -base64 32)
  
  log_success "Generated all secrets"
  
  if [[ "$SHOW_SECRETS" == true ]]; then
    log_warning "Secrets (--show-secrets enabled):"
    echo "  PostgreSQL: $POSTGRES_PASSWORD"
    echo "  Authentik admin: $AUTHENTIK_ADMIN_PASSWORD"
    echo "  Dex client: $DEX_CLIENT_SECRET"
  fi
}

create_namespaces() {
  log_info "Creating namespaces..."
  
  for ns in auth-system tekton-pipelines platform-system argocd cert-manager; do
    if k3s kubectl get namespace "$ns" &> /dev/null; then
      log_info "Namespace $ns already exists"
    else
      k3s kubectl create namespace "$ns"
      log_success "Created namespace: $ns"
    fi
  done
}

create_secrets() {
  log_info "Creating secrets..."
  
  # Cloudflare API token (cert-manager for DNS-01, kube-system for external-dns, platform-system for org controller)
  if ! k3s kubectl get secret cloudflare-api-token -n cert-manager &> /dev/null; then
    k3s kubectl create secret generic cloudflare-api-token -n cert-manager \
      --from-literal=api-token="$CLOUDFLARE_API_TOKEN"
    log_success "Created secret: cloudflare-api-token (cert-manager)"
  fi
  
  if ! k3s kubectl get secret cloudflare-api-token -n kube-system &> /dev/null; then
    k3s kubectl create secret generic cloudflare-api-token -n kube-system \
      --from-literal=api-token="$CLOUDFLARE_API_TOKEN"
    log_success "Created secret: cloudflare-api-token (kube-system)"
  fi
  
  if ! k3s kubectl get secret cloudflare-api-token -n platform-system &> /dev/null; then
    k3s kubectl create secret generic cloudflare-api-token -n platform-system \
      --from-literal=token="$CLOUDFLARE_API_TOKEN"
    log_success "Created secret: cloudflare-api-token (platform-system)"
  fi
  
  # PostgreSQL secret
  if ! k3s kubectl get secret authentik-postgresql -n auth-system &> /dev/null; then
    k3s kubectl create secret generic authentik-postgresql -n auth-system \
      --from-literal=postgresql-password="$POSTGRES_PASSWORD" \
      --from-literal=postgresql-postgres-password="$POSTGRES_PASSWORD"
    log_success "Created secret: authentik-postgresql"
  fi
  
  # Authentik secrets
  if ! k3s kubectl get secret authentik-secrets -n auth-system &> /dev/null; then
    k3s kubectl create secret generic authentik-secrets -n auth-system \
      --from-literal=secret-key="$AUTHENTIK_SECRET_KEY" \
      --from-literal=admin-password="$AUTHENTIK_ADMIN_PASSWORD" \
      --from-literal=bootstrap-token="$AUTHENTIK_BOOTSTRAP_TOKEN"
    log_success "Created secret: authentik-secrets"
  fi
  
  # Dex secrets
  if ! k3s kubectl get secret dex-secrets -n auth-system &> /dev/null; then
    k3s kubectl create secret generic dex-secrets -n auth-system \
      --from-literal=client-secret="$DEX_CLIENT_SECRET" \
      --from-literal=authentik-client-secret="$DEX_CLIENT_SECRET" \
      --from-literal=argocd-client-secret="$ARGOCD_CLIENT_SECRET" \
      --from-literal=tekton-client-secret="$TEKTON_CLIENT_SECRET" \
      --from-literal=kubernetes-client-secret="$KUBERNETES_CLIENT_SECRET"
    log_success "Created secret: dex-secrets"
  fi
  
  # ArgoCD OIDC secret
  if k3s kubectl get secret argocd-secret -n argocd &> /dev/null; then
    k3s kubectl patch secret argocd-secret -n argocd \
      --type merge -p "{\"stringData\":{\"oidc.dex.clientSecret\":\"$ARGOCD_CLIENT_SECRET\"}}"
    log_success "Patched secret: argocd-secret"
  fi
  
  # Tekton Dashboard OIDC
  if ! k3s kubectl get secret tekton-dashboard-oidc -n tekton-pipelines &> /dev/null; then
    k3s kubectl create secret generic tekton-dashboard-oidc -n tekton-pipelines \
      --from-literal=client-secret="$TEKTON_CLIENT_SECRET"
    log_success "Created secret: tekton-dashboard-oidc"
  fi
}

install_argocd() {
  log_info "Installing ArgoCD..."
  
  if k3s kubectl get deployment argocd-server -n argocd &> /dev/null; then
    log_info "ArgoCD already installed"
  else
    k3s kubectl apply -n argocd -f "https://raw.githubusercontent.com/argoproj/argo-cd/${ARGOCD_VERSION}/manifests/install.yaml"
    log_success "Applied ArgoCD manifests"
  fi
  
  log_info "Waiting for ArgoCD to be ready..."
  k3s kubectl wait --for=condition=available --timeout=300s deployment/argocd-server -n argocd
  k3s kubectl wait --for=condition=available --timeout=300s deployment/argocd-repo-server -n argocd
  
  log_success "ArgoCD is ready"
}

deploy_platform() {
  log_info "Deploying platform root application..."
  
  # Apply the root application that bootstraps everything else
  # For K3s, we use the k3s-specific app-of-apps
  k3s kubectl apply -f - <<'EOF'
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: platform-root
  namespace: argocd
  finalizers:
    - argoproj.io/finalizer
spec:
  project: default
  source:
    repoURL: https://github.com/bdchatham/AphexPlatformInfrastructure.git
    targetRevision: HEAD
    path: platform/deployments/k3s/argocd/apps
  destination:
    server: https://kubernetes.default.svc
    namespace: argocd
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    retry:
      limit: 5
      backoff:
        duration: 5s
        factor: 2
        maxDuration: 3m
EOF
  
  log_success "Platform root application deployed"
  log_info "ArgoCD will now sync all platform components"
}

print_summary() {
  echo ""
  echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
  echo -e "${GREEN}✓ K3s Bootstrap Complete!${NC}"
  echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
  echo ""
  echo "Kubeconfig location: /etc/rancher/k3s/k3s.yaml"
  echo ""
  echo "To access from remote machine:"
  echo "  1. Copy /etc/rancher/k3s/k3s.yaml to your machine"
  echo "  2. Replace 127.0.0.1 with this server's IP"
  echo "  3. export KUBECONFIG=~/.kube/k3s-config"
  echo ""
  echo "ArgoCD admin password:"
  echo "  k3s kubectl get secret argocd-initial-admin-secret -n argocd -o jsonpath='{.data.password}' | base64 -d"
  echo ""
  echo "Monitor sync status:"
  echo "  k3s kubectl get applications -n argocd"
  echo ""
}

# Main
echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}Aphex Platform Bootstrap - K3s Deployment${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

check_prerequisites
install_k3s
configure_nvidia_runtime
generate_secrets
create_namespaces
install_argocd
create_secrets
deploy_platform
print_summary
