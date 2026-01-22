#!/usr/bin/env bash

set -euo pipefail

# Aphex Platform Bootstrap Dispatcher
#
# Routes to deployment-specific bootstrap scripts based on --deployment flag.
#
# Usage:
#   ./bootstrap.sh --deployment kind [options]
#   ./bootstrap.sh --deployment k3s [options]

DEPLOYMENT=""
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Parse --deployment flag first
while [[ $# -gt 0 ]]; do
  case $1 in
    --deployment)
      DEPLOYMENT="$2"
      shift 2
      ;;
    *)
      break
      ;;
  esac
done

if [[ -z "$DEPLOYMENT" ]]; then
  echo "Usage: $0 --deployment <kind|k3s> [options]"
  echo ""
  echo "Deployments:"
  echo "  kind    Local Kind cluster (development)"
  echo "  k3s     K3s cluster with GPU support (production)"
  exit 1
fi

BOOTSTRAP_SCRIPT="$SCRIPT_DIR/platform/deployments/$DEPLOYMENT/bootstrap/bootstrap.sh"

if [[ ! -f "$BOOTSTRAP_SCRIPT" ]]; then
  echo "Error: Bootstrap script not found for deployment '$DEPLOYMENT'"
  echo "Expected: $BOOTSTRAP_SCRIPT"
  exit 1
fi

exec "$BOOTSTRAP_SCRIPT" "$@"
