#!/bin/bash
set -e

# Container Image Build and Publish Script
# Builds all four Arbiter Pipeline Infrastructure container images
# and publishes them with semantic version, commit SHA, and latest tags

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
REGISTRY="${REGISTRY:-public.ecr.aws/arbiter}"
VERSION="${VERSION:-}"
COMMIT_SHA=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

# Container images to build
declare -a IMAGES=("builder" "deployer" "tester" "validator")

# Functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

usage() {
    cat << EOF
Usage: $0 [OPTIONS]

Build and publish Arbiter Pipeline Infrastructure container images.

OPTIONS:
    -v, --version VERSION    Semantic version (e.g., v1.0.0, v1.1.0)
    -r, --registry REGISTRY  Container registry (default: public.ecr.aws/arbiter)
    -p, --push              Push images to registry
    -h, --help              Show this help message

EXAMPLES:
    # Build images locally
    $0

    # Build and tag with version
    $0 --version v1.0.0

    # Build, tag, and push to registry
    $0 --version v1.0.0 --push

    # Use custom registry
    $0 --version v1.0.0 --registry my-registry.io/arbiter --push

EOF
}

validate_version() {
    local version=$1
    if [[ ! $version =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        log_error "Invalid version format: $version"
        log_error "Version must follow semantic versioning: v<major>.<minor>.<patch> (e.g., v1.0.0)"
        exit 1
    fi
}

build_image() {
    local image_name=$1
    local image_dir="containers/${image_name}"
    
    if [ ! -d "$image_dir" ]; then
        log_error "Image directory not found: $image_dir"
        return 1
    fi
    
    if [ ! -f "$image_dir/Dockerfile" ]; then
        log_error "Dockerfile not found in: $image_dir"
        return 1
    fi
    
    log_info "Building ${image_name} image for AMD64 architecture..."
    
    # Build the image for AMD64 (linux/amd64) to match EKS nodes
    docker buildx build \
        --platform linux/amd64 \
        --build-arg BUILD_DATE="$BUILD_DATE" \
        --build-arg VCS_REF="$COMMIT_SHA" \
        --build-arg VERSION="${VERSION:-dev}" \
        -t "${REGISTRY}/${image_name}:${COMMIT_SHA}" \
        --load \
        -f "$image_dir/Dockerfile" \
        "$image_dir"
    
    if [ $? -ne 0 ]; then
        log_error "Failed to build ${image_name} image"
        return 1
    fi
    
    log_info "Successfully built ${image_name} image for AMD64"
    return 0
}

tag_image() {
    local image_name=$1
    local base_tag="${REGISTRY}/${image_name}:${COMMIT_SHA}"
    
    log_info "Tagging ${image_name} image..."
    
    # Always tag with commit SHA (already done during build)
    log_info "  - ${REGISTRY}/${image_name}:${COMMIT_SHA}"
    
    # Tag with semantic version if provided
    if [ -n "$VERSION" ]; then
        docker tag "$base_tag" "${REGISTRY}/${image_name}:${VERSION}"
        log_info "  - ${REGISTRY}/${image_name}:${VERSION}"
    fi
    
    # Tag as latest
    docker tag "$base_tag" "${REGISTRY}/${image_name}:latest"
    log_info "  - ${REGISTRY}/${image_name}:latest"
}

push_image() {
    local image_name=$1
    
    log_info "Pushing ${image_name} image to registry..."
    
    # Push commit SHA tag
    docker push "${REGISTRY}/${image_name}:${COMMIT_SHA}"
    
    # Push semantic version tag if provided
    if [ -n "$VERSION" ]; then
        docker push "${REGISTRY}/${image_name}:${VERSION}"
    fi
    
    # Push latest tag
    docker push "${REGISTRY}/${image_name}:latest"
    
    log_info "Successfully pushed ${image_name} image"
}

# Parse command line arguments
PUSH=false

while [[ $# -gt 0 ]]; do
    case $1 in
        -v|--version)
            VERSION="$2"
            shift 2
            ;;
        -r|--registry)
            REGISTRY="$2"
            shift 2
            ;;
        -p|--push)
            PUSH=true
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

# Validate version if provided
if [ -n "$VERSION" ]; then
    validate_version "$VERSION"
fi

# Print configuration
log_info "Build Configuration:"
log_info "  Registry: $REGISTRY"
log_info "  Version: ${VERSION:-<not set>}"
log_info "  Commit SHA: $COMMIT_SHA"
log_info "  Build Date: $BUILD_DATE"
log_info "  Push: $PUSH"
log_info ""

# Check if Docker is available
if ! command -v docker &> /dev/null; then
    log_error "Docker is not installed or not in PATH"
    exit 1
fi

# Check if Docker daemon is running
if ! docker info &> /dev/null; then
    log_error "Docker daemon is not running"
    exit 1
fi

# Check if Docker buildx is available
if ! docker buildx version &> /dev/null; then
    log_error "Docker buildx is not available"
    log_error "Please enable Docker buildx or update Docker to a newer version"
    exit 1
fi

# Create and use a buildx builder if needed
if ! docker buildx inspect multiarch-builder &> /dev/null; then
    log_info "Creating buildx builder for multi-architecture builds..."
    docker buildx create --name multiarch-builder --use
else
    docker buildx use multiarch-builder
fi

# Build all images
log_info "Building container images..."
for image in "${IMAGES[@]}"; do
    if ! build_image "$image"; then
        log_error "Build failed for ${image}"
        exit 1
    fi
done

log_info ""
log_info "All images built successfully!"
log_info ""

# Tag all images
log_info "Tagging container images..."
for image in "${IMAGES[@]}"; do
    tag_image "$image"
done

log_info ""
log_info "All images tagged successfully!"
log_info ""

# Push images if requested
if [ "$PUSH" = true ]; then
    log_info "Pushing container images to registry..."
    
    # Check if logged in to registry (for ECR)
    if [[ $REGISTRY == *"ecr.aws"* ]]; then
        log_info "Detected ECR registry, checking authentication..."
        # Note: User should run aws ecr-public get-login-password | docker login ... before this
    fi
    
    for image in "${IMAGES[@]}"; do
        if ! push_image "$image"; then
            log_error "Push failed for ${image}"
            exit 1
        fi
    done
    
    log_info ""
    log_info "All images pushed successfully!"
else
    log_info "Skipping push (use --push to push images to registry)"
fi

log_info ""
log_info "Build complete!"
log_info ""
log_info "Built images:"
for image in "${IMAGES[@]}"; do
    log_info "  - ${REGISTRY}/${image}:${COMMIT_SHA}"
    if [ -n "$VERSION" ]; then
        log_info "  - ${REGISTRY}/${image}:${VERSION}"
    fi
    log_info "  - ${REGISTRY}/${image}:latest"
done
