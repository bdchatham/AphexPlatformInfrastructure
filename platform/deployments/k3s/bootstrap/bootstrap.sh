#!/usr/bin/env bash

set -euo pipefail

# Aphex Platform Bootstrap - K3s Deployment
#
# Prerequisites:
#   - K3s cluster already running
#   - kubectl configured to access cluster
#   - NVIDIA drivers installed on GPU nodes
#   - NVIDIA Container Toolkit installed
#
# Usage:
#   ./bootstrap.sh [--show-secrets]

echo "K3s bootstrap not yet implemented"
echo ""
echo "TODO:"
echo "  - Assume cluster exists (no creation)"
echo "  - Configure NVIDIA RuntimeClass for real GPUs"
echo "  - Use Traefik or bare-metal nginx ingress"
echo "  - Configure real domain TLS with Let's Encrypt"
echo "  - Set up local-path-provisioner or Longhorn storage"
exit 1
