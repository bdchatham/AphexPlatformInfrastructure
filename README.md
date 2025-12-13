# Arbiter Pipeline Infrastructure

CDK infrastructure package providing a shared execution environment for deploying CDK applications across the Arbiter agent suite.

## Overview

This package deploys and manages an EKS cluster with Argo Workflows and Argo Events, builds and publishes container images with execution scripts, and exports cluster references for pipeline packages to consume. The system enables multiple CDK applications to share a common deployment infrastructure while maintaining isolation and security.

## Project Structure

```
arbiter-pipeline-infrastructure/
├── bin/                          # CDK app entry point
├── lib/                          # CDK constructs and stacks
├── test/                         # Unit, property-based, and integration tests
├── containers/                   # Container images and Dockerfiles
│   ├── builder/                  # Build container with Node.js, Python, Git
│   ├── deployer/                 # Deploy container with CDK CLI, kubectl
│   ├── tester/                   # Test container with test frameworks
│   ├── validator/                # Validation container with schema tools
│   └── common/                   # Shared utilities for execution scripts
├── scripts/                      # Build and utility scripts
│   ├── build-images.sh           # Container image build and publish script
│   └── README.md                 # Build script documentation
├── .kiro/
│   ├── docs/                     # RAG-ready documentation
│   └── specs/                    # Feature specifications
├── package.json                  # Node.js dependencies
├── tsconfig.json                 # TypeScript configuration
├── jest.config.js                # Jest test configuration
├── cdk.json                      # CDK configuration
└── requirements-test.txt         # Python testing dependencies
```

## Getting Started

### Prerequisites

- Node.js 20.x or later
- Python 3.11 or later
- AWS CLI configured with appropriate credentials
- AWS CDK CLI (`npm install -g aws-cdk`)
- Docker (for integration tests)

### Quick Setup (Recommended)

```bash
# Complete setup for new developers (installs everything)
make setup

# Or step by step:
make install                    # Install Node.js and Python dependencies
make install-integration-tools  # Install kind, kubectl, helm (for integration tests)
make build                      # Build TypeScript
```

### Manual Installation

```bash
# Install Node.js dependencies
npm install

# Create Python virtual environment and install testing dependencies
python3 -m venv venv
source venv/bin/activate  # On Windows: venv\Scripts\activate
pip install -r requirements-test.txt

# Build the project
npm run build
```

### Available Make Targets

Run `make help` to see all available commands:
```bash
make help                       # Show all available commands
make install                    # Install all dependencies
make build                      # Build TypeScript
make test                       # Run unit tests
make test-integration-full      # Run integration tests (with setup/cleanup)
make clean                      # Clean build artifacts
```

See `MAKEFILE.md` for a complete reference of all make commands and workflows.

### Development

```bash
# Watch mode for TypeScript compilation
npm run watch

# Run tests
npm test

# Run tests in watch mode
npm run test:watch
```

### Building Container Images

The project includes four container images (builder, deployer, tester, validator) that are used by the pipeline infrastructure:

```bash
# Build all container images locally
npm run build:images

# Build with semantic version tag
./scripts/build-images.sh --version v1.0.0

# Build and push to registry
./scripts/build-images.sh --version v1.0.0 --push
```

See `scripts/README.md` for detailed build instructions.

### Deployment

```bash
# Bootstrap CDK (first time only)
cdk bootstrap

# Deploy the infrastructure
cdk deploy
```

## Documentation

This repository is configured for the **Archon** RAG system. Complete documentation is maintained under `.kiro/docs/`:

- `overview.md` - High-level purpose and context
- `architecture.md` - System design and components
- `operations.md` - Deployment, monitoring, runbooks
- `api.md` - API contracts and interfaces
- `data-models.md` - Data structures and schemas
- `faq.md` - Common questions and answers

See `CLAUDE.md` for the documentation contract and standards.

## Testing

The project uses a comprehensive testing approach:

### Unit Tests
```bash
# Run all unit tests
npm run test:unit

# Run specific test file
npm test -- test/constructs/aphex-cluster.unit.test.ts
```

### Property-Based Tests
Property-based tests use Hypothesis (Python) and fast-check (TypeScript) to verify correctness properties across many inputs:

```bash
# Run Python property tests
pytest test/

# Run TypeScript property tests
npm test -- test/constructs/aphex-cluster.property.test.ts
```

### Integration Tests
Integration tests use a local Kubernetes cluster (kind) to verify end-to-end functionality:

```bash
# Check prerequisites (kind, kubectl, helm, Docker)
npm run test:integration:check

# Setup cluster, run tests, and cleanup (all-in-one)
npm run test:integration:full

# Or run steps manually:
npm run test:integration:setup    # Create kind cluster with Argo
npm run test:integration          # Run integration tests
npm run test:integration:cleanup  # Delete cluster
```

See `test/integration/README.md` for detailed integration testing documentation.

All tests are located in the `test/` directory.

## License

MIT


## Examples

### Example 1: Basic Cluster Deployment

Deploy a cluster with default configuration:

```typescript
import * as cdk from 'aws-cdk-lib';
import { ArbiterPipelineInfrastructureStack } from 'arbiter-pipeline-infrastructure';

const app = new cdk.App();

new ArbiterPipelineInfrastructureStack(app, 'ArbiterPipelineInfrastructure', {
  env: {
    account: process.env.CDK_DEFAULT_ACCOUNT,
    region: process.env.CDK_DEFAULT_REGION,
  },
});

app.synth();
```

### Example 2: Custom Cluster Configuration

Deploy a cluster with custom sizing and configuration:

```typescript
import * as cdk from 'aws-cdk-lib';
import * as ec2 from 'aws-cdk-lib/aws-ec2';
import * as eks from 'aws-cdk-lib/aws-eks';
import { ArbiterPipelineInfrastructureStack } from 'arbiter-pipeline-infrastructure';

const app = new cdk.App();

new ArbiterPipelineInfrastructureStack(app, 'ArbiterPipelineInfrastructure', {
  clusterProps: {
    clusterName: 'my-pipeline-cluster',
    minNodes: 3,
    maxNodes: 20,
    instanceType: ec2.InstanceType.of(ec2.InstanceClass.T3, ec2.InstanceSize.LARGE),
    kubernetesVersion: eks.KubernetesVersion.V1_28,
    enableContainerInsights: true,
  },
  env: {
    account: process.env.CDK_DEFAULT_ACCOUNT,
    region: process.env.CDK_DEFAULT_REGION,
  },
});

app.synth();
```

### Example 3: Using Existing VPC

Deploy a cluster in an existing VPC:

```typescript
import * as cdk from 'aws-cdk-lib';
import * as ec2 from 'aws-cdk-lib/aws-ec2';
import { ArbiterPipelineInfrastructureStack } from 'arbiter-pipeline-infrastructure';

const app = new cdk.App();

const stack = new cdk.Stack(app, 'MyStack');

// Import existing VPC
const vpc = ec2.Vpc.fromLookup(stack, 'ExistingVpc', {
  vpcId: 'vpc-12345678',
});

new ArbiterPipelineInfrastructureStack(stack, 'ArbiterPipelineInfrastructure', {
  clusterProps: {
    vpc: vpc,
  },
  env: {
    account: process.env.CDK_DEFAULT_ACCOUNT,
    region: process.env.CDK_DEFAULT_REGION,
  },
});

app.synth();
```

### Example 4: Creating an Isolated Pipeline

Create a pipeline with its own namespace and AWS permissions:

```typescript
import * as cdk from 'aws-cdk-lib';
import * as iam from 'aws-cdk-lib/aws-iam';
import { AphexCluster } from 'arbiter-pipeline-infrastructure';

const app = new cdk.App();
const stack = new cdk.Stack(app, 'MyPipelineStack');

// Create or import cluster
const cluster = new AphexCluster(stack, 'Cluster', {
  clusterName: 'arbiter-pipeline-cluster',
});

// Create isolated pipeline
const pipeline = cluster.createPipeline({
  pipelineId: 'archon-prod',
  policyStatements: [
    // S3 permissions for artifacts
    new iam.PolicyStatement({
      actions: ['s3:GetObject', 's3:PutObject', 's3:ListBucket'],
      resources: [
        'arn:aws:s3:::my-artifact-bucket',
        'arn:aws:s3:::my-artifact-bucket/*',
      ],
    }),
    // CloudFormation permissions for deployment
    new iam.PolicyStatement({
      actions: [
        'cloudformation:CreateStack',
        'cloudformation:UpdateStack',
        'cloudformation:DescribeStacks',
        'cloudformation:DescribeStackEvents',
      ],
      resources: ['*'],
    }),
    // IAM permissions for CDK deployment
    new iam.PolicyStatement({
      actions: [
        'iam:CreateRole',
        'iam:AttachRolePolicy',
        'iam:PutRolePolicy',
      ],
      resources: ['*'],
    }),
  ],
  labels: {
    'environment': 'production',
    'team': 'archon',
  },
});

console.log(`Pipeline created in namespace: ${pipeline.namespace}`);
console.log(`Service account role ARN: ${pipeline.roleArn}`);

app.synth();
```

### Example 5: Importing Existing Cluster

Import an existing cluster in a pipeline stack:

```typescript
import * as cdk from 'aws-cdk-lib';
import * as iam from 'aws-cdk-lib/aws-iam';
import { AphexCluster } from 'arbiter-pipeline-infrastructure';

const app = new cdk.App();
const stack = new cdk.Stack(app, 'MyPipelineStack');

// Import cluster using CloudFormation exports
const clusterName = cdk.Fn.importValue('ArbiterCluster-ClusterName');
const oidcProviderArn = cdk.Fn.importValue('ArbiterCluster-OIDCProviderArn');
const kubectlRoleArn = cdk.Fn.importValue('ArbiterCluster-KubectlRoleArn');

const cluster = AphexCluster.fromClusterAttributes(stack, 'ImportedCluster', {
  clusterName: clusterName,
  oidcProviderArn: oidcProviderArn,
  kubectlRoleArn: kubectlRoleArn,
});

// Now use the cluster to create pipelines
const pipeline = cluster.createPipeline({
  pipelineId: 'my-app',
  policyStatements: [
    new iam.PolicyStatement({
      actions: ['s3:*'],
      resources: ['arn:aws:s3:::my-app-bucket/*'],
    }),
  ],
});

app.synth();
```

### Example 6: Argo Workflow Using Execution Scripts

Example Argo Workflow that uses the execution scripts:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  name: deploy-my-app
  namespace: pipeline-my-app
spec:
  entrypoint: deploy-pipeline
  serviceAccountName: my-app-sa
  
  templates:
    - name: deploy-pipeline
      steps:
        - - name: validate
            template: validate-config
        - - name: build
            template: build-artifacts
        - - name: deploy
            template: deploy-stacks
        - - name: test
            template: run-tests
    
    - name: validate-config
      container:
        image: public.ecr.aws/arbiter/validator:latest
        command: [aphex-validate]
        args: ["/config/pipeline-config.yaml"]
        env:
          - name: AWS_REGION
            value: "us-east-1"
    
    - name: build-artifacts
      container:
        image: public.ecr.aws/arbiter/builder:latest
        command: [aphex-build]
        args:
          - "https://github.com/myorg/myapp.git"
          - "{{workflow.parameters.commit-sha}}"
          - "npm install && npm run build"
          - "my-artifact-bucket"
        env:
          - name: AWS_REGION
            value: "us-east-1"
    
    - name: deploy-stacks
      container:
        image: public.ecr.aws/arbiter/deployer:latest
        command: [aphex-deploy-stack]
        args:
          - "https://github.com/myorg/myapp.git"
          - "{{workflow.parameters.commit-sha}}"
          - "prod"
          - "MyAppStack,MyDatabaseStack"
          - "my-artifact-bucket"
        env:
          - name: AWS_REGION
            value: "us-east-1"
          - name: AWS_ACCOUNT
            value: "123456789012"
    
    - name: run-tests
      container:
        image: public.ecr.aws/arbiter/tester:latest
        command: [aphex-test]
        args: ["npm run test:integration"]
        env:
          - name: AWS_REGION
            value: "us-east-1"
```

## CloudFormation Exports Reference

The infrastructure stack exports the following values for consumption by pipeline stacks:

| Export Name | Description | Example Usage |
|-------------|-------------|---------------|
| `ArbiterCluster-ClusterName` | EKS cluster name | `cdk.Fn.importValue('ArbiterCluster-ClusterName')` |
| `ArbiterCluster-OIDCProviderArn` | OIDC provider ARN for IRSA | `cdk.Fn.importValue('ArbiterCluster-OIDCProviderArn')` |
| `ArbiterCluster-KubectlRoleArn` | kubectl IAM role ARN | `cdk.Fn.importValue('ArbiterCluster-KubectlRoleArn')` |
| `ArbiterCluster-ClusterSecurityGroupId` | Cluster security group ID | `cdk.Fn.importValue('ArbiterCluster-ClusterSecurityGroupId')` |

## Container Image Reference

### Available Images

All images are published to a public container registry:

| Image | Tag Format | Description |
|-------|------------|-------------|
| `public.ecr.aws/arbiter/builder` | `latest`, `v1.0.0`, `abc1234` | Build container with Node.js, Python, Git, AWS CLI |
| `public.ecr.aws/arbiter/deployer` | `latest`, `v1.0.0`, `abc1234` | Deploy container with CDK CLI, kubectl, AWS CLI |
| `public.ecr.aws/arbiter/tester` | `latest`, `v1.0.0`, `abc1234` | Test container with Jest, Mocha, Pytest |
| `public.ecr.aws/arbiter/validator` | `latest`, `v1.0.0`, `abc1234` | Validation container with jsonschema, PyYAML |

### Image Tags

- `latest`: Most recent build (use for development)
- `v1.0.0`: Semantic version (use for production)
- `abc1234`: Git commit SHA (use for reproducibility)

## Contributing

Contributions are welcome! Please follow these guidelines:

1. **Code Style**: Follow existing code style (TypeScript/Python)
2. **Tests**: Add unit tests and property-based tests for new features
3. **Documentation**: Update `.kiro/docs/` to reflect changes
4. **Commits**: Use conventional commit messages

### Development Workflow

```bash
# Create feature branch
git checkout -b feature/my-feature

# Make changes and test
npm run build
npm test
pytest test/

# Run integration tests
npm run test:integration:full

# Commit changes
git commit -m "feat: add new feature"

# Push and create PR
git push origin feature/my-feature
```

## Troubleshooting

### Common Issues

**Issue**: `cdk deploy` fails with "Unable to resolve AWS account"

**Solution**: Ensure AWS CLI is configured with credentials:
```bash
aws configure
# Or set environment variables:
export AWS_ACCESS_KEY_ID=...
export AWS_SECRET_ACCESS_KEY=...
export AWS_DEFAULT_REGION=us-east-1
```

**Issue**: Container images fail to pull

**Solution**: Verify image name and tag are correct. For private registries, ensure image pull secrets are configured.

**Issue**: Workflows fail with IRSA errors

**Solution**: Verify service account has the correct role annotation and the IAM role trust policy allows the service account to assume it.

For more troubleshooting guidance, see `.kiro/docs/operations.md`.

## Support

For questions and support:
- **Documentation**: See `.kiro/docs/` for comprehensive documentation
- **Issues**: Open an issue on GitHub
- **Discussions**: Use GitHub Discussions for questions

