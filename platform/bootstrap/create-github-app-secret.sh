#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "=========================================="
echo "Create GitHub App Secret for Lighthouse"
echo "=========================================="
echo ""

# Check if kubectl is installed
if ! command -v kubectl &> /dev/null; then
    echo -e "${RED}ERROR: kubectl is not installed${NC}"
    echo "Please install kubectl first: https://kubernetes.io/docs/tasks/tools/"
    exit 1
fi

# Check cluster access
if ! kubectl cluster-info &> /dev/null; then
    echo -e "${RED}ERROR: Cannot access Kubernetes cluster${NC}"
    echo "Please check your kubeconfig"
    exit 1
fi

# Ensure pipeline-system namespace exists
if ! kubectl get namespace pipeline-system &> /dev/null; then
    echo -e "${RED}ERROR: pipeline-system namespace does not exist${NC}"
    echo "Please create the namespace first: kubectl create namespace pipeline-system"
    exit 1
fi

# Prompt for GitHub App configuration
echo "Please provide the following GitHub App configuration values:"
echo ""

# GitHub App ID
read -p "GitHub App ID: " GITHUB_APP_ID
if [ -z "$GITHUB_APP_ID" ]; then
    echo -e "${RED}ERROR: GitHub App ID is required${NC}"
    exit 1
fi

# GitHub App Installation ID
read -p "GitHub App Installation ID: " GITHUB_APP_INSTALLATION_ID
if [ -z "$GITHUB_APP_INSTALLATION_ID" ]; then
    echo -e "${RED}ERROR: GitHub App Installation ID is required${NC}"
    exit 1
fi

# Webhook Secret
read -sp "Webhook Secret: " GITHUB_WEBHOOK_SECRET
echo ""
if [ -z "$GITHUB_WEBHOOK_SECRET" ]; then
    echo -e "${RED}ERROR: Webhook Secret is required${NC}"
    exit 1
fi

# Private Key File
read -p "Path to GitHub App Private Key (.pem file): " GITHUB_APP_PRIVATE_KEY_FILE
if [ -z "$GITHUB_APP_PRIVATE_KEY_FILE" ]; then
    echo -e "${RED}ERROR: Private Key file path is required${NC}"
    exit 1
fi

if [ ! -f "$GITHUB_APP_PRIVATE_KEY_FILE" ]; then
    echo -e "${RED}ERROR: Private Key file not found: ${GITHUB_APP_PRIVATE_KEY_FILE}${NC}"
    exit 1
fi

# Verify private key format
if ! head -n 1 "$GITHUB_APP_PRIVATE_KEY_FILE" | grep -q "BEGIN RSA PRIVATE KEY"; then
    echo -e "${YELLOW}WARNING: Private key file does not appear to be in PEM format${NC}"
    read -p "Continue anyway? (y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Aborted."
        exit 1
    fi
fi

echo ""
echo "=========================================="
echo "Creating Kubernetes Secret"
echo "=========================================="
echo ""

# Check if secret already exists
if kubectl get secret lighthouse-github-app -n pipeline-system &> /dev/null; then
    echo -e "${YELLOW}Secret 'lighthouse-github-app' already exists in pipeline-system namespace${NC}"
    read -p "Do you want to delete and recreate it? (y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "Deleting existing secret..."
        kubectl delete secret lighthouse-github-app -n pipeline-system
        echo -e "${GREEN}Existing secret deleted${NC}"
    else
        echo "Aborted."
        exit 1
    fi
fi

# Create the secret
echo "Creating secret 'lighthouse-github-app' in pipeline-system namespace..."
if kubectl create secret generic lighthouse-github-app \
    --from-file=private-key="${GITHUB_APP_PRIVATE_KEY_FILE}" \
    --from-literal=app-id="${GITHUB_APP_ID}" \
    --from-literal=installation-id="${GITHUB_APP_INSTALLATION_ID}" \
    --from-literal=webhook-secret="${GITHUB_WEBHOOK_SECRET}" \
    -n pipeline-system; then
    echo -e "${GREEN}Secret created successfully${NC}"
else
    echo -e "${RED}Failed to create secret${NC}"
    exit 1
fi

echo ""
echo "=========================================="
echo "Verifying Secret"
echo "=========================================="
echo ""

# Verify secret was created
kubectl get secret lighthouse-github-app -n pipeline-system

echo ""
echo "Secret contents (keys only):"
kubectl describe secret lighthouse-github-app -n pipeline-system | grep -A 10 "Data"

echo ""
echo "=========================================="
echo -e "${GREEN}GitHub App secret created successfully!${NC}"
echo "=========================================="
echo ""
echo "Next steps:"
echo "  1. Create Lighthouse configuration ConfigMap (task 6.4)"
echo "  2. Install Lighthouse via Helm (task 6.5)"
echo ""
echo "To verify the secret values:"
echo "  kubectl get secret lighthouse-github-app -n pipeline-system -o jsonpath='{.data.app-id}' | base64 -d"
echo "  kubectl get secret lighthouse-github-app -n pipeline-system -o jsonpath='{.data.installation-id}' | base64 -d"
