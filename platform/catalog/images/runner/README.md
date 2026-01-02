# Pipeline Runner Image

This directory contains the Dockerfile and build scripts for the pipeline runner image used by Tekton pipelines in the Jenkins X platform.

## Overview

The runner image is based on `node:20-alpine` and includes all the tools needed to execute CDKTF deployment pipelines:

- **Node.js 20**: JavaScript runtime for CDKTF
- **Git**: Repository cloning
- **Terraform**: Infrastructure provisioning
- **CDKTF CLI**: Cloud Development Kit for Terraform
- **kubectl**: Kubernetes CLI for cluster operations

## Building the Image

To build the runner image:

```bash
./build.sh
```

This will create two tags:
- `pipeline-runner:latest` (local)
- `ghcr.io/bdchatham/pipeline-runner:latest` (registry-ready)

### Custom Configuration

You can customize the build with environment variables:

```bash
# Use a different tag
IMAGE_TAG=v1.0.0 ./build.sh

# Use a different registry
REGISTRY=my-registry.com ./build.sh

# Combine both
IMAGE_TAG=v1.0.0 REGISTRY=my-registry.com ./build.sh
```

## Pushing to Registry

To push the image to a registry:

```bash
./push.sh
```

### Setting Up a Local Registry

If you don't have a local registry running, you can start one with:

```bash
docker run -d -p 5001:5000 --restart=always --name registry registry:2
```

Then push the image:

```bash
./push.sh
```

### Pushing to a Different Registry

To push to a different registry:

```bash
REGISTRY=your-registry.com ./push.sh
```

## Verifying the Image

To verify the image was built correctly:

```bash
# Check the image exists
docker images | grep pipeline-runner

# Run the image interactively
docker run -it --rm pipeline-runner:latest /bin/bash

# Inside the container, verify tools are installed
node --version
npm --version
git --version
terraform version
cdktf --version
kubectl version --client
```

## Using in Tekton Pipelines

Reference the image in your Tekton Task definitions:

```yaml
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: cdktf-deploy
spec:
  steps:
    - name: deploy
      image: localhost:5000/pipeline-runner:latest
      script: |
        #!/bin/bash
        set -euo pipefail
        
        cd $(workspaces.source.path)
        cdktf deploy --auto-approve
```

## Image Size

The image is approximately 973MB, which includes:
- Alpine Linux base
- Node.js runtime and npm packages
- Terraform binary
- CDKTF CLI and dependencies
- kubectl binary
- Git and other system tools

## Updating the Image

When updating the image:

1. Modify the `Dockerfile` as needed
2. Build the new image: `./build.sh`
3. Test the image locally
4. Tag with a version: `IMAGE_TAG=v1.1.0 ./build.sh`
5. Push to registry: `IMAGE_TAG=v1.1.0 ./push.sh`
6. Update pipeline definitions to use the new tag

## Troubleshooting

### Build Fails

If the build fails, check:
- Docker daemon is running
- Internet connectivity (for downloading packages)
- Sufficient disk space

### Push Fails

If the push fails, check:
- You are authenticated to GHCR: `echo $GITHUB_TOKEN | docker login ghcr.io -u USERNAME --password-stdin`
- You have push permissions to the `bdchatham` organization
- Network connectivity to ghcr.io

### Tools Not Working

If tools don't work in the container:
- Verify installation in Dockerfile
- Check tool versions are compatible
- Review build logs for errors

## Security Considerations

- The image runs as root by default (required for some operations)
- Consider using a non-root user for production deployments
- Regularly update base image and dependencies for security patches
- Scan image for vulnerabilities: `docker scout quickview pipeline-runner:latest`

## Source

This image is part of the Arbiter Pipeline Infrastructure platform.

**Requirements**: 13.4
