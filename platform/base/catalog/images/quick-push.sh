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
echo "  Quick Push: Pipeline Runner Image"
echo "=========================================="
echo ""
echo "Registry: ${GHCR_REGISTRY}"
echo "Organization: ${GITHUB_ORG}"
echo "Tag: ${IMAGE_TAG}"
echo ""

# Check if logged into GHCR
echo -e "${BLUE}▸${NC} Checking GHCR authentication..."
if ! docker info 2>/dev/null | grep -q "Username"; then
    echo -e "${YELLOW}  ⚠${NC} Not logged into Docker"
    echo ""
    echo "Please log in to GitHub Container Registry:"
    echo "  docker login ${GHCR_REGISTRY} -u ${GITHUB_ORG}"
    echo ""
    echo "You'll need a Personal Access Token or GitHub App token with 'write:packages' scope"
    echo "Create a PAT at: https://github.com/settings/tokens"
    echo ""
    exit 1
fi

# Build runner image
echo ""
echo -e "${BLUE}▸${NC} Building pipeline-runner image..."
RUNNER_IMAGE="${GHCR_REGISTRY}/${GITHUB_ORG}/pipeline-runner:${IMAGE_TAG}"

if docker build -t "${RUNNER_IMAGE}" "${SCRIPT_DIR}/runner"; then
    echo -e "${GREEN}  ✓${NC} Pipeline-runner image built: ${RUNNER_IMAGE}"
else
    echo -e "${RED}  ✗${NC} Failed to build pipeline-runner image"
    exit 1
fi

# Push runner image
echo ""
echo -e "${BLUE}▸${NC} Pushing pipeline-runner image to GHCR..."
if docker push "${RUNNER_IMAGE}"; then
    echo -e "${GREEN}  ✓${NC} Pipeline-runner image pushed: ${RUNNER_IMAGE}"
else
    echo -e "${RED}  ✗${NC} Failed to push pipeline-runner image"
    exit 1
fi

# Also tag and push as commit SHA if we're in a git repo
if git rev-parse --git-dir > /dev/null 2>&1; then
    COMMIT_SHA=$(git rev-parse --short HEAD)
    COMMIT_IMAGE="${GHCR_REGISTRY}/${GITHUB_ORG}/pipeline-runner:${COMMIT_SHA}"
    
    echo ""
    echo -e "${BLUE}▸${NC} Tagging with commit SHA: ${COMMIT_SHA}"
    docker tag "${RUNNER_IMAGE}" "${COMMIT_IMAGE}"
    
    echo -e "${BLUE}▸${NC} Pushing commit-tagged image..."
    if docker push "${COMMIT_IMAGE}"; then
        echo -e "${GREEN}  ✓${NC} Commit-tagged image pushed: ${COMMIT_IMAGE}"
    else
        echo -e "${YELLOW}  ⚠${NC} Failed to push commit-tagged image (non-fatal)"
    fi
fi

echo ""
echo "=========================================="
echo -e "${GREEN}  ✓ Image Built and Pushed!${NC}"
echo "=========================================="
echo ""
echo "Image available at:"
echo "  - ${RUNNER_IMAGE}"
if [ -n "${COMMIT_SHA}" ]; then
    echo "  - ${COMMIT_IMAGE}"
fi
echo ""
echo "To make the image public (if needed):"
echo "  1. Visit: https://github.com/users/${GITHUB_ORG}/packages/container/pipeline-runner/settings"
echo "  2. Scroll to 'Danger Zone'"
echo "  3. Click 'Change visibility' → 'Public'"
echo ""
echo "Next steps:"
echo "  1. Commit and push the new Tekton tasks and pipeline"
echo "  2. Create the platform-images RepoBinding"
echo "  3. Future image builds will happen automatically in the cluster!"
echo ""
