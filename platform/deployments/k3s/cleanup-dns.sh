#!/usr/bin/env bash
set -euo pipefail

# Cleans up Cloudflare DNS records before K3s uninstall
# Run this BEFORE /usr/local/bin/k3s-uninstall.sh

echo "Deleting HTTPRoutes to trigger external-dns cleanup..."
k3s kubectl delete httproutes --all -A --wait=true

echo "Waiting 90s for external-dns to remove DNS records..."
sleep 90

echo "DNS cleanup complete. You can now run: sudo /usr/local/bin/k3s-uninstall.sh"
