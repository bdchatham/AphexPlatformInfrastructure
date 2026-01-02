# Testing Platform Upgrade Tasks

This directory contains test TaskRuns for validating the platform upgrade Tasks.

## Prerequisites

- Kubernetes cluster with Tekton Pipelines installed
- kubectl configured to access the cluster
- Tasks deployed to the `pipeline-catalog` namespace

## Deploy Tasks

First, deploy all the Tasks to the cluster:

```bash
kubectl apply -f kubectl-apply.yaml
kubectl apply -f helm-upgrade.yaml
kubectl apply -f upgrade-tekton.yaml
```

## Test kubectl-apply Task

The kubectl-apply Task applies Kubernetes manifests from a directory.

```bash
# Run the test
kubectl apply -f test-kubectl-apply.yaml

# Check the TaskRun status
kubectl get taskrun test-kubectl-apply -n pipeline-catalog

# View logs
kubectl logs -n pipeline-catalog -l tekton.dev/taskRun=test-kubectl-apply

# Verify the ConfigMap was created
kubectl get configmap test-config -n default
```

Expected output:
- TaskRun completes successfully
- ConfigMap `test-config` exists in default namespace

## Test helm-upgrade Task

The helm-upgrade Task upgrades Helm releases.

**Note**: This test requires Helm repositories to be configured. You may need to adjust the chart name or add a repository first.

```bash
# Add a Helm repository (if needed)
helm repo add stable https://charts.helm.sh/stable
helm repo update

# Run the test
kubectl apply -f test-helm-upgrade.yaml

# Check the TaskRun status
kubectl get taskrun test-helm-upgrade -n pipeline-catalog

# View logs
kubectl logs -n pipeline-catalog -l tekton.dev/taskRun=test-helm-upgrade

# Verify the release was created
helm list -n default
```

Expected output:
- TaskRun completes successfully
- Helm release `test-nginx` exists in default namespace

## Test upgrade-tekton Task

The upgrade-tekton Task upgrades Tekton Pipelines.

**Warning**: This will upgrade Tekton Pipelines in your cluster. Only run this in a test environment.

```bash
# Run the test
kubectl apply -f test-upgrade-tekton.yaml

# Check the TaskRun status
kubectl get taskrun test-upgrade-tekton -n pipeline-catalog

# View logs
kubectl logs -n pipeline-catalog -l tekton.dev/taskRun=test-upgrade-tekton

# Verify Tekton version
kubectl get deployment tekton-pipelines-controller -n tekton-pipelines -o jsonpath='{.spec.template.spec.containers[0].image}'
```

Expected output:
- TaskRun completes successfully
- Tekton Pipelines upgraded to v0.56.0
- All Tekton pods are running

## Cleanup

Remove test resources:

```bash
# Delete TaskRuns
kubectl delete taskrun test-kubectl-apply -n pipeline-catalog
kubectl delete taskrun test-helm-upgrade -n pipeline-catalog
kubectl delete taskrun test-upgrade-tekton -n pipeline-catalog

# Delete test ConfigMap
kubectl delete configmap test-config -n default

# Delete test Helm release (if created)
helm uninstall test-nginx -n default
```

## Validation Checklist

- [ ] kubectl-apply Task successfully applies manifests
- [ ] kubectl-apply Task supports recursive directory application
- [ ] helm-upgrade Task successfully upgrades Helm releases
- [ ] helm-upgrade Task supports values files
- [ ] helm-upgrade Task supports inline values
- [ ] upgrade-tekton Task successfully downloads release YAML
- [ ] upgrade-tekton Task patches images from gcr.io to ghcr.io
- [ ] upgrade-tekton Task applies manifests and waits for readiness
- [ ] All Tasks have appropriate error handling
- [ ] All Tasks produce clear log output
