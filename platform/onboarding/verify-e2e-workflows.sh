#!/bin/bash

# Master verification script for end-to-end workflows
# This script verifies all critical workflows for the Jenkins X platform

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "=========================================="
echo "Jenkins X Platform E2E Workflow Verification"
echo "=========================================="
echo ""

# Color codes for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Track overall status
OVERALL_STATUS=0

# Function to run a verification and track status
run_verification() {
    local test_name="$1"
    local test_script="$2"
    
    echo ""
    echo "=========================================="
    echo "Running: $test_name"
    echo "=========================================="
    
    if bash "$SCRIPT_DIR/$test_script"; then
        echo -e "${GREEN}✓ PASSED: $test_name${NC}"
        return 0
    else
        echo -e "${RED}✗ FAILED: $test_name${NC}"
        OVERALL_STATUS=1
        return 1
    fi
}

# 1. Verify onboarding workflow completes successfully
echo ""
echo "=========================================="
echo "1. VERIFYING ONBOARDING WORKFLOW"
echo "=========================================="

run_verification "Onboarding Workflow" "test-onboarding-workflow.sh" || true

# 2. Verify pipeline execution workflow completes successfully
echo ""
echo "=========================================="
echo "2. VERIFYING PIPELINE EXECUTION WORKFLOW"
echo "=========================================="

run_verification "Pipeline Execution" "test-pipeline-execution.sh" || true

# 3. Verify tenant isolation is enforced
echo ""
echo "=========================================="
echo "3. VERIFYING TENANT ISOLATION"
echo "=========================================="

run_verification "Tenant Isolation" "test-tenant-isolation.sh" || true

# 4. Verify deployment serialization works correctly
echo ""
echo "=========================================="
echo "4. VERIFYING DEPLOYMENT SERIALIZATION"
echo "=========================================="

run_verification "Deployment Serialization" "test-deployment-serialization.sh" || true

# Final summary
echo ""
echo "=========================================="
echo "VERIFICATION SUMMARY"
echo "=========================================="

if [ $OVERALL_STATUS -eq 0 ]; then
    echo -e "${GREEN}✓ ALL WORKFLOWS VERIFIED SUCCESSFULLY${NC}"
    echo ""
    echo "The Jenkins X platform is ready for production use:"
    echo "  ✓ Onboarding workflow completes successfully"
    echo "  ✓ Pipeline execution workflow completes successfully"
    echo "  ✓ Tenant isolation is enforced"
    echo "  ✓ Deployment serialization works correctly"
    echo ""
    echo "Next steps:"
    echo "  1. Review operational documentation in platform/*/README.md"
    echo "  2. Set up monitoring and alerting"
    echo "  3. Onboard production repositories"
else
    echo -e "${RED}✗ SOME WORKFLOWS FAILED VERIFICATION${NC}"
    echo ""
    echo "Please review the failed tests above and address any issues."
    echo "Common troubleshooting steps:"
    echo "  1. Check component health: kubectl get pods -A"
    echo "  2. Review logs: kubectl logs -n pipeline-system <pod-name>"
    echo "  3. Verify RBAC: kubectl auth can-i --list --as=system:serviceaccount:tenant:pipeline-runner"
    echo "  4. Check allowlist: kubectl get configmap -n pipeline-system repo-allowlist -o yaml"
fi

echo ""
exit $OVERALL_STATUS
