#!/bin/bash
set -euo pipefail

# Configuration
IMAGE_NAME="pipeline-runner"
IMAGE_TAG="${IMAGE_TAG:-latest}"
REGISTRY="${REGISTRY:-ghcr.io/bdchatham}"
FULL_IMAGE="${REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG}"

echo "Pushing runner image to registry..."
echo "Image: ${FULL_IMAGE}"

# Check if authenticated to GHCR
echo "Checking GHCR authentication..."
if ! docker login ghcr.io --username dummy --password dummy 2>&1 | grep -q "unauthorized\|denied\|Login Succeeded"; then
    echo ""
    echo "=========================================="
    echo "WARNING: Not authenticated to GitHub Container Registry"
    echo "=========================================="
    echo ""
    echo "The image has been built successfully and is available locally as:"
    echo "  - ${IMAGE_NAME}:${IMAGE_TAG}"
    echo "  - ${FULL_IMAGE}"
    echo ""
    echo "To push to GHCR, authenticate first:"
    echo ""
    echo "1. Create a GitHub Personal Access Token with 'write:packages' scope:"
    echo "   https://github.com/settings/tokens/new"
    echo ""
    echo "2. Authenticate to GHCR:"
    echo "   echo \$GITHUB_TOKEN | docker login ghcr.io -u YOUR_USERNAME --password-stdin"
    echo ""
    echo "3. Then run this script again:"
    echo "   ./push.sh"
    echo ""
    echo "4. Or push to a different registry:"
    echo "   REGISTRY=your-registry.com ./push.sh"
    echo ""
    exit 0
fi

# Push the image
echo "Pushing image to GHCR..."
docker push "${FULL_IMAGE}"

echo ""
echo "=========================================="
echo "Push complete!"
echo "=========================================="
echo "Image available at: ${FULL_IMAGE}"
