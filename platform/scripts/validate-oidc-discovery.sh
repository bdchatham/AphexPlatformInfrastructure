#!/usr/bin/env bash

set -euo pipefail

# OIDC Discovery Validation Script
# Validates that Dex OIDC discovery endpoints are reachable

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

DEX_URL="${DEX_URL:-https://dex.home.local}"

log_info() {
  echo -e "${YELLOW}ℹ${NC} $1"
}

log_success() {
  echo -e "${GREEN}✓${NC} $1"
}

log_error() {
  echo -e "${RED}✗${NC} $1"
}

validate_dex_discovery() {
  log_info "Validating Dex OIDC discovery endpoint..."
  
  if curl -fsS "${DEX_URL}/.well-known/openid-configuration" >/dev/null; then
    log_success "Dex OIDC discovery endpoint is reachable"
  else
    log_error "Dex OIDC discovery endpoint is not reachable"
    return 1
  fi
}

validate_jwks_endpoint() {
  log_info "Validating Dex JWKS endpoint..."
  
  if curl -fsS "${DEX_URL}/keys" >/dev/null; then
    log_success "Dex JWKS endpoint is reachable"
  else
    log_error "Dex JWKS endpoint is not reachable"
    return 1
  fi
}

validate_from_cluster() {
  log_info "Validating Dex discovery from within cluster..."
  
  kubectl run oidc-test --image=curlimages/curl --rm -i --restart=Never -- \
    curl -fsS "${DEX_URL}/.well-known/openid-configuration" >/dev/null
  
  if [ $? -eq 0 ]; then
    log_success "Dex is reachable from within cluster"
  else
    log_error "Dex is not reachable from within cluster"
    return 1
  fi
}

main() {
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "OIDC Discovery Validation"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo ""
  
  validate_dex_discovery
  validate_jwks_endpoint
  validate_from_cluster
  
  echo ""
  log_success "All OIDC discovery validations passed"
}

main "$@"
