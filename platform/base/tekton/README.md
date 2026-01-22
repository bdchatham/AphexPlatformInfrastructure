# Tekton Pipelines

This directory contains the Kustomize configuration for deploying Tekton Pipelines to the platform-services namespace.

## Overview

Tekton Pipelines provides the pipeline execution engine for the platform. It enables CI/CD workflows for tenant repositories through PipelineRuns triggered by Lighthouse webhooks.

## Configuration

The `kustomization.yaml` file:
- Pulls the latest Tekton Pipelines release from official storage
- Patches all Tekton images to use ghcr.io registry instead of gcr.io
- Deploys to the platform-services namespace

## Image Registry

All Tekton images are patched to use GitHub Container Registry (ghcr.io) instead of Google Container Registry (gcr.io) to avoid rate limiting and improve reliability for homelab deployments.

## Deployment

This component is deployed via ArgoCD. The ArgoCD Application is defined in `argocd/apps/platform-apps.yaml` with sync wave 1.

## Components

Tekton Pipelines includes:
- **Controller**: Manages Pipeline and PipelineRun resources
- **Webhook**: Validates and mutates Tekton resources
- **Resolvers**: Resolves remote Pipeline and Task references
- **Events**: Emits CloudEvents for Pipeline execution
- **Entrypoint**: Container entrypoint for Task execution
- **Sidecar Log Results**: Collects Task results from sidecars
- **Working Dir Init**: Initializes working directories for Tasks

## Version

Current version: v0.56.0

## Source

- `platform/tekton/kustomization.yaml` - Kustomize configuration
- `argocd/apps/platform-apps.yaml` - ArgoCD Application definition
