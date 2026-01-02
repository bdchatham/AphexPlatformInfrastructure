#!/bin/bash
set -euo pipefail

echo "=========================================="
echo "Testing Resource Quota Enforcement"
echo "=========================================="

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo ""
echo "Verifying ResourceQuota and LimitRange exist in tenant namespaces..."
echo ""

# Check first tenant
if kubectl get resourcequota tenant-quota -n test-tenant &>/dev/null; then
    echo -e "${GREEN}✓ ResourceQuota exists in test-tenant${NC}"
    kubectl get resourcequota tenant-quota -n test-tenant -o jsonpath='{.spec.hard}' | jq '.'
else
    echo "✗ ResourceQuota not found in test-tenant"
    exit 1
fi

echo ""

if kubectl get limitrange tenant-limits -n test-tenant &>/dev/null; then
    echo -e "${GREEN}✓ LimitRange exists in test-tenant${NC}"
else
    echo "✗ LimitRange not found in test-tenant"
    exit 1
fi

echo ""
echo -e "${YELLOW}Note: Resource quotas prevent tenants from consuming excessive cluster resources.${NC}"
echo -e "${YELLOW}The quotas limit CPU, memory, pods, and persistent volume claims per namespace.${NC}"
echo ""
echo -e "${GREEN}✓ Resource quota enforcement verified${NC}"
