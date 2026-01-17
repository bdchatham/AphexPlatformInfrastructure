# GitHub Actions Workflows

This directory contains CI/CD workflows for the Aphex Pipeline Infrastructure platform.

## Workflows

### Platform Controller Image Build

**File:** `platform-controller-image.yaml`

**Purpose:** Builds and pushes the platform controller Docker image to GitHub Container Registry (ghcr.io).

**Triggers:**
- Push to `main` or `develop` branches (when controller code changes)
- Pull requests to `main` (builds but doesn't push)
- Manual workflow dispatch

**Image Tags:**
- `latest` - Latest build from main branch
- `<branch-name>` - Branch-specific builds
- `<branch>-<sha>` - Commit-specific builds

**Registry:** `ghcr.io/<owner>/platform-controller`

**Permissions Required:**
- `contents: read` - Read repository code
- `packages: write` - Push to GitHub Container Registry

**Notes:**
- Uses Docker Buildx for efficient multi-stage builds
- Implements GitHub Actions cache for faster builds
- Only pushes images on push events (not PRs)
- Builds for linux/amd64 platform
