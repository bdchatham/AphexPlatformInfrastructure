# Container Images

This directory contains Dockerfiles and build scripts for custom container images used in the pipeline platform.

## Images

### Pipeline Runner (`ghcr.io/bdchatham/pipeline-runner:latest`)

A multi-tool container image that includes:
- Node.js 20 (Alpine)
- Git
- Terraform 1.6.0
- CDKTF CLI
- kubectl
- Python 3 with pip
- AWS CLI tools

Used by Tekton tasks for:
- CDKTF synthesis and deployment
- Artifact uploads
- Infrastructure operations

**Source:** `runner/Dockerfile`

## Building and Pushing Images

### Prerequisites

1. Docker installed and running
2. GitHub Personal Access Token with `write:packages` scope
3. Logged into GHCR:
   ```bash
   docker login ghcr.io -u <your-github-username>
   # Use your PAT as the password
   ```

### Build and Push

Run the build script from anywhere in the repository:

```bash
./platform/catalog/images/build-and-push.sh
```

Or with custom settings:

```bash
GITHUB_ORG=myorg IMAGE_TAG=v1.0.0 ./platform/catalog/images/build-and-push.sh
```

### Environment Variables

- `GITHUB_ORG`: GitHub organization/username (default: `bdchatham`)
- `IMAGE_TAG`: Image tag (default: `latest`)

## Using Images in Tekton Tasks

All Tekton tasks that use the runner image have been configured with the default:

```yaml
params:
  - name: runnerImage
    description: Container image with required tools
    type: string
    default: "ghcr.io/bdchatham/pipeline-runner:latest"
```

You can override this in PipelineRuns if needed:

```yaml
params:
  - name: runnerImage
    value: "ghcr.io/myorg/pipeline-runner:v1.0.0"
```

## Image Visibility

Images pushed to GHCR are private by default. To make them public:

1. Go to https://github.com/users/bdchatham/packages/container/pipeline-runner/settings
2. Scroll to "Danger Zone"
3. Click "Change visibility" → "Public"

Or use the GitHub CLI:

```bash
gh api \
  --method PATCH \
  -H "Accept: application/vnd.github+json" \
  /user/packages/container/pipeline-runner \
  -f visibility='public'
```
