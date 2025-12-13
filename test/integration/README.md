# Integration Tests

This directory contains integration tests for the Arbiter Pipeline Infrastructure. These tests verify end-to-end functionality using a local Kubernetes cluster (kind).

## Automatic Setup and Teardown

**NEW**: Integration tests now automatically set up and tear down the kind cluster! Just run `npm test` and Jest will:
1. Check if required tools are installed (kind, kubectl, helm, docker)
2. Create a kind cluster if needed
3. Install Argo Workflows and Argo Events
4. Run all tests (unit, property, and integration)
5. Clean up the cluster after tests complete

If prerequisites are missing, integration tests will be automatically skipped and only unit/property tests will run.

## Prerequisites

The following tools are required for integration tests:

### Automatic Installation (Recommended)

```bash
# Install all integration test tools automatically
make install-integration-tools

# Or check if they're already installed
make test-integration-check
```

This works on macOS (via Homebrew) and Linux (via direct downloads). For Windows, see manual installation below.

### Manual Installation

#### macOS (using Homebrew)
```bash
brew install kind kubectl helm
```

#### Linux
```bash
# kind
curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.20.0/kind-linux-amd64
chmod +x ./kind
sudo mv ./kind /usr/local/bin/kind

# kubectl
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
chmod +x kubectl
sudo mv kubectl /usr/local/bin/

# helm
curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
```

#### Windows
```powershell
# Using Chocolatey
choco install kind kubectl kubernetes-helm
```

## Quick Start

### Run All Tests (Recommended)

**Simplest approach - automatic setup and cleanup:**
```bash
npm test
```

This runs ALL tests (unit, property, and integration) with automatic kind cluster management:
- If prerequisites are installed: Sets up cluster, runs all tests, cleans up
- If prerequisites are missing: Skips integration tests, runs unit/property tests only

### Run Only Integration Tests

**Option 1: Automatic setup and cleanup**
```bash
npm run test:integration
```

**Option 2: Keep cluster after tests (for debugging)**
```bash
npm run test:integration:keep
```

**Option 3: Using Make**
```bash
make test-integration-full
```

**Option 4: Using convenience script**
```bash
bash test/integration/run-tests.sh
```

### Manual Setup and Testing (Advanced)

If you want full control over the cluster lifecycle:

#### 1. Setup the cluster (one time)
```bash
npm run test:integration:setup
```

This creates a kind cluster named `arbiter-test` and installs:
- Argo Workflows
- Argo Events

#### 2. Run integration tests (without auto-setup/cleanup)
```bash
# Disable automatic setup/teardown
SKIP_INTEGRATION_TESTS=false npm run test:integration
```

#### 3. Cleanup (when done)
```bash
npm run test:integration:cleanup
```

### Environment Variables

Control the automatic setup/teardown behavior:

- `CLEANUP_INTEGRATION_CLUSTER=false` - Keep cluster after tests (useful for debugging)
- `SKIP_INTEGRATION_TESTS=true` - Skip integration tests entirely
- `KIND_CLUSTER_NAME=my-cluster` - Use a custom cluster name
- `ARGO_NAMESPACE=my-argo` - Use a custom Argo namespace

## Test Structure

### `argo-workflows.integration.test.ts`
Tests Argo Workflows and Argo Events functionality:
- **Cluster Setup**: Verifies kind cluster and Argo components are running
- **Simple Workflow Execution**: Tests basic workflow execution
- **Multi-Step Workflows**: Tests sequential and parallel workflow steps
- **Workflow Artifacts**: Tests artifact passing between workflow steps
- **Argo Events**: Verifies Argo Events installation and CRDs

## Configuration

You can customize the cluster configuration using environment variables:

```bash
# Custom cluster name
export KIND_CLUSTER_NAME=my-test-cluster

# Custom Argo namespace
export ARGO_NAMESPACE=my-argo

# Run tests
npm run test:integration
```

## Troubleshooting

### Cluster won't start
```bash
# Check Docker is running
docker ps

# Check kind version
kind version

# Try recreating the cluster
npm run test:integration:cleanup
npm run test:integration:setup
```

### Tests are timing out
Integration tests have a 5-minute timeout. If tests are timing out:
1. Check your internet connection (Helm needs to download charts)
2. Check Docker resource limits (kind needs sufficient CPU/memory)
3. Check cluster logs: `kubectl logs -n argo -l app.kubernetes.io/name=argo-workflows-workflow-controller`

### Workflows are failing
```bash
# Check workflow status
kubectl get workflows -n argo

# Get workflow details
kubectl describe workflow <workflow-name> -n argo

# Check workflow logs
kubectl logs -n argo -l workflows.argoproj.io/workflow=<workflow-name>
```

### Clean up stuck resources
```bash
# Delete all workflows
kubectl delete workflows --all -n argo

# Force delete the cluster
kind delete cluster --name arbiter-test

# Clean up Docker resources
docker system prune -f
```

## CI/CD Integration

### GitHub Actions Example
```yaml
name: Integration Tests

on: [push, pull_request]

jobs:
  integration-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Setup Node.js
        uses: actions/setup-node@v3
        with:
          node-version: '20'
      
      - name: Install dependencies
        run: npm ci
      
      - name: Install kind
        run: |
          curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.20.0/kind-linux-amd64
          chmod +x ./kind
          sudo mv ./kind /usr/local/bin/kind
      
      - name: Install kubectl
        run: |
          curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
          chmod +x kubectl
          sudo mv kubectl /usr/local/bin/
      
      - name: Install helm
        run: |
          curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
      
      - name: Run integration tests
        run: npm run test:integration:full
```

## Performance

Typical test execution times:
- Cluster setup: ~90 seconds
- Simple workflow test: ~10 seconds
- Multi-step workflow test: ~20 seconds
- Artifact workflow test: ~20 seconds
- Total test suite: ~2-3 minutes
- Cleanup: ~5 seconds

## What's Not Tested

These integration tests focus on Argo Workflows/Events functionality. They do **not** test:
- AWS-specific features (EKS, IRSA, VPC)
- Container image builds
- S3 artifact storage
- CloudFormation deployments
- Cross-account deployments

These AWS-specific features are validated through:
- Unit tests (CDK template verification)
- Property-based tests (script logic)
- Manual deployment to dev environment

## Contributing

When adding new integration tests:
1. Keep tests focused and fast (< 30 seconds per test)
2. Clean up resources in `afterEach` hooks
3. Use descriptive test names
4. Add comments explaining what's being tested
5. Update this README if adding new test categories
