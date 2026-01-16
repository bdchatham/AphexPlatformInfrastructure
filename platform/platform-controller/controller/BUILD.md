# Building and Publishing the Platform Controller

## Automated Builds (GitHub Actions)

The platform controller image is automatically built and pushed to GitHub Container Registry when changes are pushed to the repository.

### Automatic Triggers

The workflow runs automatically when:
- Code is pushed to `main` or `develop` branches
- Changes are made to files under `platform/platform-controller/controller/`
- Pull requests are opened against `main` (builds but doesn't push)

### Manual Trigger

You can manually trigger a build from the GitHub Actions UI:
1. Go to the repository on GitHub
2. Click "Actions" tab
3. Select "Build and Push Platform Controller Image"
4. Click "Run workflow"

### Image Tags

Images are tagged with:
- `latest` - Most recent build from main branch
- `main` - Latest main branch build
- `develop` - Latest develop branch build
- `main-<sha>` - Specific commit from main
- `develop-<sha>` - Specific commit from develop

### Registry Location

Images are published to:
```
ghcr.io/<github-username>/platform-controller:<tag>
```

## Local Development Builds

### Build Locally

```bash
cd platform/platform-controller/controller
make build
```

This creates a binary at `bin/manager`.

### Build Docker Image Locally

```bash
cd platform/platform-controller/controller
docker build -t platform-controller:local .
```

### Run Locally

```bash
cd platform/platform-controller/controller
make run
```

## Deployment

The controller deployment manifest references the image:
```yaml
image: ghcr.io/bdchatham/platform-controller:latest
```

ArgoCD automatically syncs the deployment when the manifest changes. To use a specific image version, update the tag in `platform/platform-controller/controller-deployment.yaml`.

## Troubleshooting

### Build Failures

Check the GitHub Actions logs:
1. Go to "Actions" tab in GitHub
2. Click on the failed workflow run
3. Review the build logs

Common issues:
- **Go compilation errors**: Fix syntax errors in Go code
- **Docker build failures**: Check Dockerfile syntax
- **Permission errors**: Ensure repository has packages write permission

### Image Not Available

If the image isn't available in ghcr.io:
1. Verify the workflow completed successfully
2. Check that the workflow had `packages: write` permission
3. Ensure the repository visibility allows package publishing
4. Verify you're using the correct image name and tag

### Using Private Images

If the repository is private, you'll need to authenticate to pull images:

```bash
# Create a GitHub personal access token with read:packages scope
kubectl create secret docker-registry ghcr-secret \
  --docker-server=ghcr.io \
  --docker-username=<github-username> \
  --docker-password=<github-token> \
  --namespace=platform-system

# Reference in deployment
spec:
  imagePullSecrets:
  - name: ghcr-secret
```
