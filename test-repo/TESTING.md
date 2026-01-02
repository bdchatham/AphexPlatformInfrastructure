# Pipeline Execution Testing Guide

This document describes how to use this test repository to validate the Jenkins X pipeline execution workflow.

## Prerequisites

- Kubernetes cluster with Jenkins X platform installed
- kubectl configured to access the cluster
- GitHub repository created for this test code
- GitHub App configured and connected to Lighthouse

## Test Repository Setup

### 1. Prepare the Repository

```bash
cd test-repo

# Install dependencies and build
./setup.sh

# Initialize git repository
git init
git add .
git commit -m "Initial commit: Test CDKTF stack for pipeline validation"

# Add GitHub remote (replace with your repository URL)
git remote add origin https://github.com/your-org/test-pipeline-execution.git

# Push to GitHub
git branch -M main
git push -u origin main
```

### 2. Onboard the Repository

Create a RepoBinding to onboard the test repository:

```bash
kubectl apply -f - <<EOF
apiVersion: platform.arbiter.io/v1alpha1
kind: RepoBinding
metadata:
  name: test-tenant-binding
  namespace: pipeline-system
spec:
  repoOrg: "your-github-org"
  repoName: "test-pipeline-execution"
  tenantName: "test-tenant"
  permissionProfile: "standard"
EOF
```

Wait for onboarding to complete:

```bash
kubectl get repobinding test-tenant-binding -n pipeline-system -w
```

### 3. Trigger the Pipeline

Merge a commit to the main branch to trigger the pipeline:

```bash
# Make a change
echo "# Test change" >> README.md
git add README.md
git commit -m "Test: Trigger pipeline execution"
git push origin main
```

The GitHub App will deliver a webhook to Lighthouse, which will create a PipelineRun.

### 4. Monitor Pipeline Execution

Watch the PipelineRun:

```bash
kubectl get pipelinerun -n test-tenant -w
```

View logs:

```bash
# Get the PipelineRun name
PIPELINERUN=$(kubectl get pipelinerun -n test-tenant --sort-by=.metadata.creationTimestamp -o name | tail -1)

# View logs
kubectl logs -n test-tenant -l tekton.dev/pipelineRun=${PIPELINERUN##*/} -f
```

## Automated Testing

Use the automated test script to run the complete workflow:

```bash
cd platform/onboarding

# Set environment variables
export TEST_REPO_ORG="your-github-org"
export TEST_REPO_NAME="test-pipeline-execution"
export TEST_TENANT_NAME="test-tenant"

# Run the test
./test-pipeline-execution.sh
```

The script will:
1. Create RepoBinding
2. Wait for onboarding to complete
3. Verify tenant resources are created
4. Verify repository is in allowlist
5. Prompt you to trigger the pipeline
6. Wait for PipelineRun creation
7. Verify PipelineRun uses correct service account
8. Wait for pipeline completion
9. Verify task execution (clone, synth, deploy)
10. Verify Terraform state management
11. Verify logs are accessible

## What the Pipeline Does

The test pipeline:

1. **Clone**: Clones the repository at the specific commit SHA
2. **Synth**: Runs `cdktf synth` to generate Terraform configuration
3. **Deploy**: Runs `cdktf deploy` to apply the infrastructure

The CDKTF stack creates a simple Kubernetes ConfigMap in the default namespace as a test resource.

## Verification Steps

### Verify PipelineRun Creation

```bash
kubectl get pipelinerun -n test-tenant
```

Expected: PipelineRun exists with status "Running" or "Succeeded"

### Verify Service Account Usage

```bash
kubectl get pipelinerun <pipelinerun-name> -n test-tenant -o jsonpath='{.spec.serviceAccountName}'
```

Expected: `pipeline-runner`

### Verify Task Execution

```bash
kubectl get taskrun -n test-tenant -l tekton.dev/pipelineRun=<pipelinerun-name>
```

Expected: TaskRuns for `clone`, `synth`, and `deploy` tasks

### Verify Terraform State

```bash
# For Kubernetes backend
kubectl get secret -n test-tenant -l tfstate=true
```

Expected: Secret containing Terraform state, isolated to test-tenant namespace

### Verify Deployed Resources

```bash
kubectl get configmap pipeline-test-config -n default
```

Expected: ConfigMap created by the CDKTF stack

### Verify Logs

```bash
kubectl logs -n test-tenant -l tekton.dev/pipelineRun=<pipelinerun-name>
```

Expected: Logs showing git clone, cdktf synth, and cdktf deploy execution

## Troubleshooting

### PipelineRun Not Created

Check Lighthouse logs:

```bash
kubectl logs -n pipeline-system -l app=lighthouse
```

Verify repository is in allowlist:

```bash
kubectl get configmap repo-allowlist -n pipeline-system -o yaml
```

### Pipeline Fails at Clone

Check service account has access to repository:

```bash
kubectl get secret -n test-tenant
```

Verify deploy key or GitHub App credentials are configured.

### Pipeline Fails at Deploy

Check Terraform backend configuration:

```bash
kubectl get secret terraform-backend-config -n test-tenant -o yaml
```

Check RBAC permissions:

```bash
kubectl auth can-i create configmaps --as=system:serviceaccount:test-tenant:pipeline-runner -n default
```

### Logs Not Accessible

Check pod status:

```bash
kubectl get pods -n test-tenant
```

Check RBAC for log access:

```bash
kubectl auth can-i get pods/log --as=system:serviceaccount:test-tenant:pipeline-runner -n test-tenant
```

## Cleanup

Delete the RepoBinding to clean up tenant resources:

```bash
kubectl delete repobinding test-tenant-binding -n pipeline-system
```

This will trigger the onboarding controller to clean up:
- Tenant namespace
- Service account
- RBAC resources
- Resource quotas
- Network policies
- Allowlist entry

Delete the test ConfigMap:

```bash
kubectl delete configmap pipeline-test-config -n default
```

## Requirements Validated

This test validates the following requirements:

- **10.1, 10.2, 10.3**: Pipeline definitions in repository
- **11.1, 11.2**: GitHub App webhook delivery and Lighthouse processing
- **11.3, 11.4, 11.5**: PipelineRun creation in tenant namespace with tenant service account
- **12.1**: Repository checkout at commit SHA
- **12.2**: CDKTF synth execution
- **12.3**: CDKTF deploy execution
- **12.4**: Remote Terraform state usage
- **7.5**: Terraform state isolation per tenant
- **16.1**: Log accessibility via kubectl
