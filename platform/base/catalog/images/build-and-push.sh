#!/bin/bash

set -e

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Configuration
GHCR_REGISTRY="ghcr.io"
GITHUB_ORG="${GITHUB_ORG:-bdchatham}"
IMAGE_TAG="${IMAGE_TAG:-latest}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo ""
echo "=========================================="
echo "  Build and Push Container Images"
echo "=========================================="
echo ""
echo "Registry: ${GHCR_REGISTRY}"
echo "Organization: ${GITHUB_ORG}"
echo "Tag: ${IMAGE_TAG}"
echo ""

# Check if logged into GHCR
echo -e "${BLUE}▸${NC} Checking GHCR authentication..."
if ! docker info 2>/dev/null | grep -q "${GHCR_REGISTRY}"; then
    echo -e "${YELLOW}  ⚠${NC} Not logged into GHCR"
    echo ""
    echo "Please log in to GitHub Container Registry:"
    echo "  docker login ${GHCR_REGISTRY} -u <username>"
    echo ""
    echo "You'll need a Personal Access Token with 'write:packages' scope"
    echo "Create one at: https://github.com/settings/tokens"
    echo ""
    read -p "Press Enter after logging in, or Ctrl+C to cancel..."
fi

# Build runner image
echo ""
echo -e "${BLUE}▸${NC} Building runner image..."
RUNNER_IMAGE="${GHCR_REGISTRY}/${GITHUB_ORG}/pipeline-runner:${IMAGE_TAG}"

if docker build -t "${RUNNER_IMAGE}" "${SCRIPT_DIR}/runner"; then
    echo -e "${GREEN}  ✓${NC} Runner image built: ${RUNNER_IMAGE}"
else
    echo -e "${RED}  ✗${NC} Failed to build runner image"
    exit 1
fi

# Push runner image
echo -e "${BLUE}▸${NC} Pushing runner image to GHCR..."
if docker push "${RUNNER_IMAGE}"; then
    echo -e "${GREEN}  ✓${NC} Runner image pushed: ${RUNNER_IMAGE}"
else
    echo -e "${RED}  ✗${NC} Failed to push runner image"
    exit 1
fi

echo ""
echo "=========================================="
echo -e "${GREEN}  ✓ Images Built and Pushed!${NC}"
echo "=========================================="
echo ""
echo "Images available at:"
echo "  - ${RUNNER_IMAGE}"
echo ""
echo "Update your Tekton tasks to use these images:"
echo "  runnerImage: ${RUNNER_IMAGE}"
echo ""
