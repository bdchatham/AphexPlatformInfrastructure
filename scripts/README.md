# Build Scripts

This directory contains scripts for building and publishing the Arbiter Pipeline Infrastructure container images.

## build-images.sh

Builds all four container images (builder, deployer, tester, validator) and publishes them with proper versioning.

### Usage

```bash
# Build images locally
./scripts/build-images.sh

# Build and tag with semantic version
./scripts/build-images.sh --version v1.0.0

# Build, tag, and push to registry
./scripts/build-images.sh --version v1.0.0 --push

# Use custom registry
./scripts/build-images.sh --version v1.0.0 --registry my-registry.io/arbiter --push
```

### Options

- `-v, --version VERSION`: Semantic version (e.g., v1.0.0, v1.1.0)
- `-r, --registry REGISTRY`: Container registry (default: public.ecr.aws/arbiter)
- `-p, --push`: Push images to registry
- `-h, --help`: Show help message

### Image Tagging

The script applies three types of tags to each image:

1. **Commit SHA tag**: `<registry>/<image>:<commit-sha>` (e.g., `public.ecr.aws/arbiter/builder:a1b2c3d`)
2. **Semantic version tag**: `<registry>/<image>:<version>` (e.g., `public.ecr.aws/arbiter/builder:v1.0.0`)
3. **Latest tag**: `<registry>/<image>:latest` (e.g., `public.ecr.aws/arbiter/builder:latest`)

### NPM Scripts

You can also use the npm scripts defined in package.json:

```bash
# Build images locally
npm run build:images

# Build and push images (requires VERSION environment variable)
VERSION=v1.0.0 npm run build:images:push
```

### Prerequisites

- Docker installed and running
- For ECR: AWS CLI configured and authenticated
  ```bash
  aws ecr-public get-login-password --region us-east-1 | \
    docker login --username AWS --password-stdin public.ecr.aws
  ```

### Examples

**Local development build:**
```bash
./scripts/build-images.sh
```

**Release build:**
```bash
./scripts/build-images.sh --version v1.2.3 --push
```

**CI/CD pipeline:**
```bash
# Authenticate with ECR
aws ecr-public get-login-password --region us-east-1 | \
  docker login --username AWS --password-stdin public.ecr.aws

# Build and push with version from git tag
VERSION=$(git describe --tags --abbrev=0)
./scripts/build-images.sh --version "$VERSION" --push
```
