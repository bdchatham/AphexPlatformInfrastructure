#!/bin/bash
set -euo pipefail

# Configuration
IMAGE_NAME="pipeline-runner"
IMAGE_TAG="${IMAGE_TAG:-latest}"
REGISTRY="${REGISTRY:-ghcr.io/bdchatham}"
FULL_IMAGE="${REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG}"

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "Building runner image..."
echo "Image: ${FULL_IMAGE}"

# Build the Docker image
docker build \
    -t "${IMAGE_NAME}:${IMAGE_TAG}" \
    -t "${FULL_IMAGE}" \
    "${SCRIPT_DIR}"

echo "Build complete!"
echo "Local tag: ${IMAGE_NAME}:${IMAGE_TAG}"
echo "Registry tag: ${FULL_IMAGE}"
