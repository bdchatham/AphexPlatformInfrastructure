#!/usr/bin/env bash

set -euo pipefail

# RBAC Authorization Validation Script
# Tests platform group permissions using kubectl auth can-i

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() {
  echo -e "${YELLOW}ℹ${NC} $1"
}

log_success() {
  echo -e "${GREEN}✓${NC} $1"
}

log_error() {
  echo -e "${RED}✗${NC} $1"
}

test_permission() {
  local user="$1"
  local group="$2"
  local verb="$3"
  local resource="$4"
  local namespace="${5:-}"
  local expected="$6"
  
  local cmd="kubectl auth can-i $verb $resource --as=$user --as-group=$group"
  if [ -n "$namespace" ]; then
    cmd="$cmd -n $namespace"
  fi
  
  if $cmd >/dev/null 2>&1; then
    actual="yes"
  else
    actual="no"
  fi
  
  if [ "$actual" = "$expected" ]; then
    log_success "$user ($group) can $verb $resource ${namespace:+in $namespace}: $actual (expected $expected)"
    return 0
  else
    log_error "$user ($group) can $verb $resource ${namespace:+in $namespace}: $actual (expected $expected)"
    return 1
  fi
}

validate_platform_admins() {
  log_info "Validating platform-admins permissions..."
  
  test_permission "admin@platform.local" "platform-admins" "create" "pipelines.platform.dev" "user-alice" "yes"
  test_permission "admin@platform.local" "platform-admins" "delete" "pipelines.platform.dev" "user-alice" "yes"
  test_permission "admin@platform.local" "platform-admins" "create" "pipelines.platform.dev" "auth-system" "yes"
  test_permission "admin@platform.local" "platform-admins" "create" "namespaces" "" "yes"
}

validate_platform_operators() {
  log_info "Validating platform-operators permissions..."
  
  test_permission "operator@platform.local" "platform-operators" "create" "pipelines.platform.dev" "user-alice" "yes"
  test_permission "operator@platform.local" "platform-operators" "delete" "pipelines.platform.dev" "user-alice" "yes"
  test_permission "operator@platform.local" "platform-operators" "get" "pods" "tekton-pipelines" "yes"
  test_permission "operator@platform.local" "platform-operators" "create" "namespaces" "" "no"
  test_permission "operator@platform.local" "platform-operators" "delete" "namespaces" "" "no"
}

validate_platform_engineering() {
  log_info "Validating platform-engineering permissions..."
  
  test_permission "alice@platform.local" "platform-engineering" "create" "pipelines.platform.dev" "user-alice" "yes"
  test_permission "alice@platform.local" "platform-engineering" "update" "pipelines.platform.dev" "user-alice" "yes"
  test_permission "alice@platform.local" "platform-engineering" "delete" "pipelines.platform.dev" "user-alice" "no"
  test_permission "alice@platform.local" "platform-engineering" "create" "pipelines.platform.dev" "auth-system" "no"
  test_permission "alice@platform.local" "platform-engineering" "create" "namespaces" "" "no"
}

main() {
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "RBAC Authorization Validation"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo ""
  
  local failed=0
  
  validate_platform_admins || failed=$((failed + 1))
  echo ""
  validate_platform_operators || failed=$((failed + 1))
  echo ""
  validate_platform_engineering || failed=$((failed + 1))
  
  echo ""
  if [ $failed -eq 0 ]; then
    log_success "All RBAC authorization validations passed"
  else
    log_error "$failed validation group(s) failed"
    exit 1
  fi
}

main "$@"
