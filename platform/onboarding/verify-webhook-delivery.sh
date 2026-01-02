#!/bin/bash
set -euo pipefail

# Verify Webhook Delivery
# This script checks if the GitHub App is delivering webhooks to Lighthouse

TIMEOUT="${TIMEOUT:-60}"

# Color codes
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

function log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

function log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

function log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

echo "========================================="
echo "Verify Webhook Delivery"
echo "========================================="
echo ""

# Check if Lighthouse is running
log_info "Checking Lighthouse status..."
if ! kubectl get pods -n pipeline-system -l app=lighthouse &> /dev/null; then
    log_error "Lighthouse pods not found in pipeline-system namespace"
    exit 1
fi

lighthouse_pod=$(kubectl get pods -n pipeline-system -l app=lighthouse -o name | head -1)
if [ -z "$lighthouse_pod" ]; then
    log_error "No Lighthouse pods found"
    exit 1
fi

log_info "Lighthouse pod: $lighthouse_pod"

# Check pod status
pod_status=$(kubectl get $lighthouse_pod -n pipeline-system -o jsonpath='{.status.phase}')
if [ "$pod_status" != "Running" ]; then
    log_error "Lighthouse pod is not running: $pod_status"
    exit 1
fi

log_info "✓ Lighthouse is running"

# Check recent logs for webhook events
echo ""
log_info "Checking recent Lighthouse logs for webhook events..."
echo ""

# Get logs from last 5 minutes
kubectl logs $lighthouse_pod -n pipeline-system --since=5m | grep -i "webhook\|event\|push" | tail -20 || {
    log_warn "No recent webhook events found in logs"
}

echo ""
log_info "To monitor webhook delivery in real-time:"
echo "  kubectl logs $lighthouse_pod -n pipeline-system -f | grep -i webhook"

echo ""
log_info "To check GitHub App webhook delivery:"
echo "  1. Go to GitHub: Settings > Developer settings > GitHub Apps > Your App"
echo "  2. Click 'Advanced' tab"
echo "  3. Check 'Recent Deliveries' for webhook status"

echo ""
log_info "Common webhook delivery issues:"
echo "  - GitHub App not installed on repository"
echo "  - Webhook URL not accessible from GitHub"
echo "  - Webhook secret mismatch"
echo "  - Repository not in allowlist"
