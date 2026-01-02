#!/bin/bash
set -euo pipefail

# Trigger Test Pipeline
# This script helps trigger a pipeline execution by creating a test commit

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_REPO_DIR="${TEST_REPO_DIR:-../../test-repo}"
TEST_TENANT_NAME="${TEST_TENANT_NAME:-test-tenant}"

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
echo "Trigger Test Pipeline"
echo "========================================="
echo ""

# Check if test repo directory exists
if [ ! -d "$TEST_REPO_DIR" ]; then
    log_error "Test repository directory not found: $TEST_REPO_DIR"
    log_info "Set TEST_REPO_DIR environment variable to point to your test repository"
    exit 1
fi

cd "$TEST_REPO_DIR"

# Check if it's a git repository
if [ ! -d ".git" ]; then
    log_error "Not a git repository: $TEST_REPO_DIR"
    log_info "Initialize git repository first: git init"
    exit 1
fi

# Check if remote is configured
if ! git remote get-url origin &> /dev/null; then
    log_error "No git remote 'origin' configured"
    log_info "Add remote: git remote add origin <your-repo-url>"
    exit 1
fi

# Check current branch
current_branch=$(git branch --show-current)
log_info "Current branch: $current_branch"

if [ "$current_branch" != "main" ] && [ "$current_branch" != "master" ]; then
    log_warn "Not on main/master branch"
    read -p "Switch to main branch? (y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        git checkout main 2>/dev/null || git checkout -b main
    else
        log_error "Pipeline triggers only work on main/master branch"
        exit 1
    fi
fi

# Create a test commit
log_info "Creating test commit..."

timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
echo "" >> README.md
echo "Test pipeline trigger at $timestamp" >> README.md

git add README.md
git commit -m "Test: Trigger pipeline execution at $timestamp"

log_info "Test commit created"

# Push to trigger pipeline
log_info "Pushing to origin to trigger pipeline..."
git push origin $(git branch --show-current)

log_info "Push complete!"

echo ""
log_info "GitHub App should deliver webhook to Lighthouse"
log_info "Lighthouse will create a PipelineRun in the ${TEST_TENANT_NAME} namespace"

echo ""
log_info "Monitor PipelineRun creation:"
echo "  kubectl get pipelinerun -n ${TEST_TENANT_NAME} -w"

echo ""
log_info "View Lighthouse logs:"
echo "  kubectl logs -n pipeline-system -l app=lighthouse -f"

echo ""
log_info "Once PipelineRun is created, view logs:"
echo "  kubectl logs -n ${TEST_TENANT_NAME} -l tekton.dev/pipelineRun=<name> -f"
