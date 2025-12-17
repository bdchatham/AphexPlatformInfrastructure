#!/bin/bash
set -e

# Push Arbiter Pipeline Infrastructure container images to ECR
# This script creates ECR repositories if needed and pushes all container images

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
AWS_REGION="${AWS_REGION:-us-east-1}"
AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
ECR_REGISTRY="${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com"
VERSION="${VERSION:-v1.0.0}"

# Container images
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

log_step() {
    echo -e "${BLUE}[STEP]${NC} $1"
}

usage() {
    cat << EOF
Usage: $0 [OPTIONS]

Push Arbiter Pipeline Infrastructure container images to Amazon ECR.

This script will:
1. Create ECR repositories if they don't exist
2. Authenticate Docker with ECR
3. Build container images
4. Push images to ECR with version and latest tags

OPTIONS:
    -v, --version VERSION    Semantic version (default: v1.0.0)
    -r, --region REGION      AWS region (default: us-east-1)
    -h, --help              Show this help message

EXAMPLES:
    # Push with default version
    $0

    # Push with specific version
    $0 --version v1.2.3

    # Push to different region
    $0 --region us-west-2

ENVIRONMENT VARIABLES:
    AWS_REGION    AWS region (default: us-east-1)
    VERSION       Image version (default: v1.0.0)

EOF
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -v|--version)
            VERSION="$2"
            shift 2
            ;;
        -r|--region)
            AWS_REGION="$2"
            ECR_REGISTRY="${AWS_ACCOUNT_ID}.dkr.ecr.${AWS_REGION}.amazonaws.com"
            shift 2
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

# Print configuration
log_info "Configuration:"
log_info "  AWS Account: $AWS_ACCOUNT_ID"
log_info "  AWS Region: $AWS_REGION"
log_info "  ECR Registry: $ECR_REGISTRY"
log_info "  Version: $VERSION"
echo ""

# Check prerequisites
log_step "Checking prerequisites..."

if ! command -v docker &> /dev/null; then
    log_error "Docker is not installed or not in PATH"
    exit 1
fi

if ! docker info &> /dev/null; then
    log_error "Docker daemon is not running"
    exit 1
fi

if ! command -v aws &> /dev/null; then
    log_error "AWS CLI is not installed or not in PATH"
    exit 1
fi

log_info "✓ All prerequisites met"
echo ""

# Step 1: Create ECR repositories
log_step "Step 1: Creating ECR repositories..."

for image in "${IMAGES[@]}"; do
    REPO_NAME="arbiter-pipeline-${image}"
    
    if aws ecr describe-repositories --repository-names "$REPO_NAME" --region "$AWS_REGION" &> /dev/null; then
        log_info "✓ Repository already exists: $REPO_NAME"
    else
        log_info "Creating repository: $REPO_NAME"
        aws ecr create-repository \
            --repository-name "$REPO_NAME" \
            --region "$AWS_REGION" \
            --image-scanning-configuration scanOnPush=true \
            --encryption-configuration encryptionType=AES256 \
            > /dev/null
        log_info "✓ Created repository: $REPO_NAME"
    fi
done

echo ""

# Step 2: Authenticate Docker with ECR
log_step "Step 2: Authenticating Docker with ECR..."

aws ecr get-login-password --region "$AWS_REGION" | \
    docker login --username AWS --password-stdin "$ECR_REGISTRY" > /dev/null 2>&1

if [ $? -eq 0 ]; then
    log_info "✓ Successfully authenticated with ECR"
else
    log_error "Failed to authenticate with ECR"
    exit 1
fi

echo ""

# Step 3: Build and push images
log_step "Step 3: Building and pushing container images..."

# Build images using the existing script
log_info "Building images..."
./scripts/build-images.sh --version "$VERSION" --registry "$ECR_REGISTRY/arbiter-pipeline"

if [ $? -ne 0 ]; then
    log_error "Failed to build images"
    exit 1
fi

echo ""

# Push each image
log_info "Pushing images to ECR..."

COMMIT_SHA=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

for image in "${IMAGES[@]}"; do
    log_info "Pushing ${image}..."
    
    # Push commit SHA tag
    docker push "${ECR_REGISTRY}/arbiter-pipeline-${image}:${COMMIT_SHA}"
    
    # Push version tag
    docker push "${ECR_REGISTRY}/arbiter-pipeline-${image}:${VERSION}"
    
    # Push latest tag
    docker push "${ECR_REGISTRY}/arbiter-pipeline-${image}:latest"
    
    log_info "✓ Pushed ${image}"
done

echo ""

# Step 4: Summary
log_step "Step 4: Summary"
echo ""
log_info "Successfully pushed all images to ECR!"
echo ""
log_info "Images available at:"
for image in "${IMAGES[@]}"; do
    echo "  ${ECR_REGISTRY}/arbiter-pipeline-${image}:${VERSION}"
    echo "  ${ECR_REGISTRY}/arbiter-pipeline-${image}:${COMMIT_SHA}"
    echo "  ${ECR_REGISTRY}/arbiter-pipeline-${image}:latest"
    echo ""
done

log_info "To use these images in your pipeline:"
echo "  export BUILDER_IMAGE=${ECR_REGISTRY}/arbiter-pipeline-builder:${VERSION}"
echo "  export DEPLOYER_IMAGE=${ECR_REGISTRY}/arbiter-pipeline-deployer:${VERSION}"
echo "  export TESTER_IMAGE=${ECR_REGISTRY}/arbiter-pipeline-tester:${VERSION}"
echo "  export VALIDATOR_IMAGE=${ECR_REGISTRY}/arbiter-pipeline-validator:${VERSION}"
echo ""

log_info "Done!"
