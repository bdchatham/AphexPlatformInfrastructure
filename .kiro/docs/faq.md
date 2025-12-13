# FAQ

## General Questions

### What is this repository for?

The Arbiter Pipeline Infrastructure provides a shared execution environment for deploying CDK applications across the Arbiter agent suite. It creates an EKS cluster with Argo Workflows and Argo Events, provides pre-built container images with execution scripts, and exports cluster references for pipeline packages to consume.

### How does this fit into the larger system?

This infrastructure package is the foundation for all Arbiter agent deployment pipelines. Archon (the first Arbiter agent) and future agents will use this infrastructure to deploy their CDK applications. The infrastructure is shared across multiple agents while maintaining isolation through Kubernetes namespaces and IRSA.

### What is the difference between AphexCluster and Arbiter Pipeline Infrastructure?

AphexCluster is the CDK construct that creates the EKS cluster. Arbiter Pipeline Infrastructure is the complete package that includes the construct, container images, execution scripts, and documentation. Think of AphexCluster as the core component, and Arbiter Pipeline Infrastructure as the full solution.

### Can I use this infrastructure for non-Arbiter projects?

Yes! The infrastructure is generic and can be used for any CDK deployment pipeline. The container images and execution scripts work with any CDK project. However, it's designed with Arbiter's specific needs in mind (multi-agent isolation, IRSA, etc.).

### What is IRSA and why is it important?

IRSA (IAM Roles for Service Accounts) allows Kubernetes pods to assume AWS IAM roles without long-lived credentials. This is critical for security because:
1. No AWS access keys stored in containers or environment variables
2. Credentials are automatically rotated by Kubernetes
3. Each pipeline can have isolated AWS permissions
4. Follows AWS security best practices

## Development Questions

### How do I set up my development environment?

```bash
# Quick setup (recommended)
make setup

# Or step by step:
make install                    # Install dependencies
make install-integration-tools  # Install kind, kubectl, helm
make build                      # Build TypeScript
```

See `README.md` for detailed setup instructions.

### How do I run tests?

```bash
# Unit tests (TypeScript)
npm run test:unit

# Property-based tests (Python)
pytest test/

# Property-based tests (TypeScript)
npm test -- test/constructs/aphex-cluster.property.test.ts

# Integration tests (requires Docker)
npm run test:integration:full

# All tests
make test
```

### How do I build container images locally?

```bash
# Build all images
npm run build:images

# Build with version tag
./scripts/build-images.sh --version v1.0.0

# Build and push to registry
./scripts/build-images.sh --version v1.0.0 --push --registry <registry-url>
```

### How do I test changes to execution scripts?

```bash
# Build the container image
docker build -t test-builder containers/builder/

# Run the script in the container
docker run --rm \
  -e AWS_REGION=us-east-1 \
  -e AWS_ROLE_ARN=arn:aws:iam::123456789012:role/test-role \
  test-builder \
  aphex-build <repo-url> <commit-sha> "npm install && npm run build" <bucket>
```

### How do I add a new execution script?

1. Create the script in the appropriate container directory (e.g., `containers/builder/my-script`)
2. Make it executable: `chmod +x containers/builder/my-script`
3. Add it to the Dockerfile: `COPY my-script /usr/local/bin/`
4. Write unit tests in `test/scripts/`
5. Write property-based tests if applicable
6. Document the script interface in `.kiro/docs/api.md`

## Operational Questions

### How do I deploy changes?

```bash
# Deploy infrastructure changes
cdk deploy

# Deploy container image changes
./scripts/build-images.sh --version v1.1.0 --push
# Then update WorkflowTemplates to use new version
```

### What should I do if pods are stuck in Pending state?

Check the pod description for details:
```bash
kubectl describe pod <pod-name> -n <namespace>
```

Common causes:
- Insufficient cluster capacity (increase `maxNodes`)
- Resource quota exceeded (adjust quota or reduce requests)
- No nodes available (check autoscaling configuration)

See the "Common Issues" section in `.kiro/docs/operations.md` for detailed troubleshooting.

### What should I do if workflows fail with permission errors?

Verify IRSA is configured correctly:
```bash
# Check service account has role annotation
kubectl get serviceaccount <sa-name> -n <namespace> -o yaml

# Check IAM role exists and has correct trust policy
aws iam get-role --role-name <role-name>

# Test credentials in pod
kubectl exec <pod-name> -n <namespace> -- aws sts get-caller-identity
```

### How do I create a new pipeline?

```typescript
import { AphexCluster } from 'arbiter-pipeline-infrastructure';
import * as iam from 'aws-cdk-lib/aws-iam';

const cluster = AphexCluster.fromClusterAttributes(this, 'Cluster', {
  clusterName: 'arbiter-pipeline-cluster',
  oidcProviderArn: 'arn:aws:iam::123456789012:oidc-provider/...',
  kubectlRoleArn: 'arn:aws:iam::123456789012:role/...',
});

const pipeline = cluster.createPipeline({
  pipelineId: 'my-pipeline',
  policyStatements: [
    new iam.PolicyStatement({
      actions: ['s3:*'],
      resources: ['arn:aws:s3:::my-bucket/*'],
    }),
  ],
});
```

### How do I delete a pipeline?

```bash
# Delete the namespace (cascades to all resources)
kubectl delete namespace pipeline-<pipeline-id>

# Or use the CDK method (removes from tracking)
cluster.deletePipeline('my-pipeline');
```

### How do I monitor workflow execution?

```bash
# Port-forward to Argo Workflows UI
kubectl port-forward -n argo svc/argo-workflows-server 2746:2746

# Open browser to http://localhost:2746

# Or use kubectl
kubectl get workflows -n <namespace>
kubectl describe workflow <workflow-name> -n <namespace>
kubectl logs <workflow-pod-name> -n <namespace>
```

### How do I view logs from execution scripts?

```bash
# View logs from a specific pod
kubectl logs <pod-name> -n <namespace>

# Stream logs in real-time
kubectl logs -f <pod-name> -n <namespace>

# View logs in CloudWatch
aws logs tail /aws/containerinsights/<cluster-name>/application --follow
```

## Architecture Questions

### Why use Argo Workflows instead of AWS Step Functions?

Argo Workflows provides several advantages:
1. Runs on Kubernetes (same environment as execution)
2. Better integration with container-based workflows
3. More flexible workflow definitions
4. Open source and self-hosted (no AWS service costs)
5. Better support for parallel execution and DAGs

### Why separate container images for each stage?

Separation provides:
1. Smaller image sizes (only install needed tools)
2. Clearer separation of concerns
3. Easier to update individual stages
4. Better security (each stage has minimal tools)
5. Reusability (can use builder without deployer, etc.)

### How does multi-pipeline isolation work?

Isolation is achieved through multiple layers:
1. **Kubernetes Namespaces**: Each pipeline gets its own namespace
2. **Network Policies**: Restrict inter-namespace communication
3. **Resource Quotas**: Prevent resource exhaustion
4. **RBAC**: Separate service accounts and roles
5. **IRSA**: Isolated AWS permissions per pipeline

### Why use CloudFormation exports instead of SSM parameters?

CloudFormation exports provide:
1. Native CDK integration (Fn.importValue)
2. Automatic dependency tracking
3. Prevents deletion of exported stacks while in use
4. No additional AWS service costs
5. Simpler for CDK-to-CDK communication

### Can multiple clusters be deployed?

Yes! You can deploy multiple clusters for different environments:
```typescript
// Development cluster
const devCluster = new AphexCluster(this, 'DevCluster', {
  clusterName: 'arbiter-dev-cluster',
  minNodes: 1,
  maxNodes: 5,
});

// Production cluster
const prodCluster = new AphexCluster(this, 'ProdCluster', {
  clusterName: 'arbiter-prod-cluster',
  minNodes: 3,
  maxNodes: 20,
});
```

## Testing Questions

### What is property-based testing?

Property-based testing verifies that properties (rules) hold across many randomly generated inputs, rather than testing specific examples. For example, instead of testing "adding task 'foo' increases list length by 1", we test "adding any valid task increases list length by 1" with 100+ random tasks.

### Why use both unit tests and property-based tests?

They complement each other:
- **Unit tests**: Verify specific examples and edge cases
- **Property tests**: Verify general correctness across many inputs

Together they provide comprehensive coverage: unit tests catch concrete bugs, property tests verify general correctness.

### How do I run integration tests locally?

```bash
# Full integration test (setup, test, cleanup)
npm run test:integration:full

# Or step by step:
npm run test:integration:setup    # Create kind cluster
npm run test:integration          # Run tests
npm run test:integration:cleanup  # Delete cluster
```

Integration tests require Docker to be running.

### What testing frameworks are used?

- **TypeScript**: Jest (unit tests), fast-check (property tests)
- **Python**: Pytest (unit tests), Hypothesis (property tests)
- **Integration**: Jest with kind (local Kubernetes cluster)

## Archon-Specific Questions

### How is this repository ingested by Archon?

Archon reads all Markdown files under `.kiro/docs/` from this public GitHub repository. Documentation follows the contract defined in `CLAUDE.md`.

### How do I update documentation?

Update the relevant files under `.kiro/docs/` and ensure changes are grounded in code. Include "Source" references to relevant files.

### What documentation standards should I follow?

Follow the Archon documentation contract in `CLAUDE.md`:
1. Keep sections small and focused (400-800 tokens)
2. Use clear, direct language
3. Maintain provenance (reference source files)
4. No hallucinations (only document what exists in code)
5. Avoid duplication (link instead of repeating)

**Source**
- `CLAUDE.md`
- `.kiro/steering/archon-docs.md`
- `README.md`
- `lib/constructs/aphex-cluster.ts`
- `.kiro/docs/operations.md`
- `.kiro/docs/api.md`
