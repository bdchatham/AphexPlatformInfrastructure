# API

## Overview

The Arbiter Pipeline Infrastructure provides two types of APIs:

1. **CDK Construct API**: TypeScript interfaces for creating and managing the infrastructure
2. **Execution Script API**: Command-line interfaces for the container execution scripts

## CDK Construct API

### AphexCluster

The main construct for creating an EKS cluster with Argo Workflows and Argo Events.

#### Constructor

```typescript
new AphexCluster(scope: Construct, id: string, props?: AphexClusterProps)
```

#### Properties (AphexClusterProps)

| Property | Type | Default | Description |
|----------|------|---------|-------------|
| `clusterName` | `string` | `'arbiter-pipeline-cluster'` | Name of the EKS cluster |
| `minNodes` | `number` | `2` | Minimum number of nodes |
| `maxNodes` | `number` | `10` | Maximum number of nodes |
| `instanceType` | `ec2.InstanceType` | `t3.medium` | Instance type for nodes |
| `kubernetesVersion` | `eks.KubernetesVersion` | `1.28` | Kubernetes version |
| `vpc` | `ec2.IVpc` | (new VPC) | VPC to use for the cluster |
| `argoNamespace` | `string` | `'argo'` | Namespace for Argo components |
| `enableContainerInsights` | `boolean` | `true` | Enable CloudWatch Container Insights |

#### Public Properties

| Property | Type | Description |
|----------|------|-------------|
| `cluster` | `eks.Cluster` | The EKS cluster |
| `oidcProvider` | `iam.IOpenIdConnectProvider` | OIDC provider for IRSA |
| `kubectlRoleArn` | `string` | kubectl IAM role ARN |
| `clusterNameExport` | `string` | CloudFormation export name for cluster name |
| `oidcProviderArnExport` | `string` | CloudFormation export name for OIDC provider ARN |

#### Methods

##### createServiceAccountWithRole()

Create a Kubernetes service account with an IAM role (IRSA).

```typescript
createServiceAccountWithRole(
  id: string,
  namespace: string,
  serviceAccountName: string,
  policyStatements: iam.PolicyStatement[]
): eks.ServiceAccount
```

**Parameters**:
- `id`: Construct ID
- `namespace`: Kubernetes namespace
- `serviceAccountName`: Name of the service account
- `policyStatements`: IAM policy statements to attach

**Returns**: The created service account

##### createPipeline()

Create an isolated pipeline with its own namespace and service account.

```typescript
createPipeline(config: PipelineConfig): PipelineResources
```

**Parameters** (PipelineConfig):
- `pipelineId`: Unique identifier for the pipeline
- `namespace`: (optional) Kubernetes namespace (defaults to `pipeline-{pipelineId}`)
- `policyStatements`: IAM policy statements for the pipeline's service account
- `labels`: (optional) Additional labels for pipeline resources

**Returns** (PipelineResources):
- `pipelineId`: The pipeline's unique identifier
- `namespace`: The pipeline's namespace
- `serviceAccount`: The pipeline's service account with IRSA
- `roleArn`: The IAM role ARN for the service account
- `labels`: Labels applied to pipeline resources

**Creates**:
- Kubernetes namespace with labels
- Service account with IRSA
- Resource quota to prevent resource exhaustion
- Network policy for namespace isolation
- Role binding for workflow execution

##### deletePipeline()

Delete a pipeline and its resources without affecting other pipelines.

```typescript
deletePipeline(pipelineId: string): void
```

**Parameters**:
- `pipelineId`: The pipeline ID to delete

**Note**: In CDK, this removes the pipeline from tracking. Actual deletion would be done via `kubectl delete namespace <namespace>`.

##### fromClusterAttributes() (static)

Import an existing cluster by its attributes.

```typescript
static fromClusterAttributes(
  scope: Construct,
  id: string,
  attrs: ClusterAttributes
): IAphexCluster
```

**Parameters** (ClusterAttributes):
- `clusterName`: EKS cluster name
- `oidcProviderArn`: OIDC provider ARN
- `kubectlRoleArn`: kubectl IAM role ARN

**Returns**: An imported cluster reference

#### Usage Example

```typescript
import { AphexCluster } from 'arbiter-pipeline-infrastructure';
import * as iam from 'aws-cdk-lib/aws-iam';

// Create the cluster
const cluster = new AphexCluster(this, 'MyCluster', {
  clusterName: 'my-pipeline-cluster',
  minNodes: 3,
  maxNodes: 20,
});

// Create an isolated pipeline
const pipeline = cluster.createPipeline({
  pipelineId: 'archon-prod',
  policyStatements: [
    new iam.PolicyStatement({
      actions: ['s3:GetObject', 's3:PutObject'],
      resources: ['arn:aws:s3:::my-artifact-bucket/*'],
    }),
    new iam.PolicyStatement({
      actions: ['cloudformation:*'],
      resources: ['*'],
    }),
  ],
});

console.log(`Pipeline created in namespace: ${pipeline.namespace}`);
console.log(`Service account role ARN: ${pipeline.roleArn}`);
```

## Execution Script APIs

All execution scripts follow a common pattern:
- Accept command-line arguments
- Read environment variables for configuration
- Output structured JSON to stdout (success) or stderr (failure)
- Exit with standard exit codes

### Exit Codes

| Code | Name | Description |
|------|------|-------------|
| 0 | SUCCESS | Operation completed successfully |
| 1 | INPUT_ERROR | Invalid parameters or missing arguments |
| 2 | AUTH_ERROR | AWS credential failures or IRSA issues |
| 3 | RESOURCE_ERROR | S3 access failures, repository clone failures |
| 4 | EXECUTION_ERROR | Build failures, deployment failures, test failures |
| 5 | TIMEOUT_ERROR | Operations exceeding time limits |

### Output Format

All scripts output structured JSON:

```json
{
  "success": true,
  "data": {
    "message": "Operation completed",
    ...
  },
  "timestamp": "2024-01-15T10:30:00.000Z"
}
```

### aphex-build

Clone a repository, run build commands, and upload artifacts to S3.

**Usage**:
```bash
aphex-build <repo-url> <commit-sha> <build-commands> <artifact-bucket>
```

**Arguments**:
- `repo-url`: Git repository URL
- `commit-sha`: Git commit SHA to build
- `build-commands`: Build commands to execute (semicolon-separated)
- `artifact-bucket`: S3 bucket for artifact upload

**Environment Variables**:
- `AWS_REGION`: AWS region for S3 operations (required)
- `AWS_ROLE_ARN`: IAM role ARN for IRSA (required)

**Output** (success):
```json
{
  "success": true,
  "data": {
    "message": "Build completed successfully",
    "artifact_path": "s3://bucket/artifacts/abc123-20240115.tar.gz",
    "commit_sha": "abc123...",
    "artifact_size": 1234567
  },
  "timestamp": "2024-01-15T10:30:00.000Z"
}
```

### aphex-deploy-pipeline

Deploy pipeline infrastructure (the cluster itself).

**Usage**:
```bash
aphex-deploy-pipeline <repo-url> <commit-sha> <stack-name> <artifact-bucket>
```

**Arguments**:
- `repo-url`: Git repository URL
- `commit-sha`: Git commit SHA to deploy
- `stack-name`: CDK stack name to deploy
- `artifact-bucket`: S3 bucket for artifacts (not used but required for consistency)

**Environment Variables**:
- `AWS_REGION`: Target AWS region (required)
- `AWS_ACCOUNT`: Target AWS account (required)
- `AWS_ROLE_ARN`: IAM role ARN for IRSA (required)

**Output** (success):
```json
{
  "success": true,
  "data": {
    "message": "Pipeline deployment completed successfully",
    "stack_name": "ArbiterPipelineInfrastructureStack",
    "commit_sha": "abc123...",
    "deployment_result": {
      "stack_name": "ArbiterPipelineInfrastructureStack",
      "outputs": {...},
      "status": "DEPLOYED"
    }
  },
  "timestamp": "2024-01-15T10:30:00.000Z"
}
```

### aphex-deploy-stack

Deploy application stacks from build artifacts.

**Usage**:
```bash
aphex-deploy-stack <repo-url> <commit-sha> <environment> <stack-list> <artifact-bucket>
```

**Arguments**:
- `repo-url`: Git repository URL
- `commit-sha`: Git commit SHA to deploy
- `environment`: Target environment (e.g., dev, staging, prod)
- `stack-list`: Comma-separated list of stack names
- `artifact-bucket`: S3 bucket containing build artifacts

**Environment Variables**:
- `AWS_REGION`: Target AWS region (required)
- `AWS_ACCOUNT`: Target AWS account (required)
- `AWS_ROLE_ARN`: IAM role ARN for IRSA (required)

**Output** (success):
```json
{
  "success": true,
  "data": {
    "message": "Stack deployment completed successfully",
    "environment": "prod",
    "commit_sha": "abc123...",
    "stacks_deployed": 3,
    "deployment_results": [
      {
        "stack_name": "MyStack1",
        "outputs": {...},
        "status": "DEPLOYED",
        "duration": 120.5
      },
      ...
    ]
  },
  "timestamp": "2024-01-15T10:30:00.000Z"
}
```

### aphex-test

Execute test commands and report results.

**Usage**:
```bash
aphex-test <test-commands>
```

**Arguments**:
- `test-commands`: Test commands to execute (semicolon-separated)

**Environment Variables**:
- `AWS_REGION`: AWS region (optional, for tests that interact with AWS)
- `AWS_ROLE_ARN`: IAM role ARN for IRSA (optional)

**Output** (success):
```json
{
  "success": true,
  "data": {
    "message": "Tests passed",
    "test_results": {
      "status": "pass",
      "exit_code": 0,
      "stdout": "...",
      "stderr": "...",
      "commands_executed": ["npm test", "pytest"]
    }
  },
  "timestamp": "2024-01-15T10:30:00.000Z"
}
```

### aphex-validate

Validate configuration files and prerequisites.

**Usage**:
```bash
aphex-validate <config-path>
```

**Arguments**:
- `config-path`: Path to configuration file (JSON or YAML)

**Environment Variables**:
- `AWS_REGION`: AWS region (optional)
- `AWS_ROLE_ARN`: IAM role ARN for IRSA (optional)

**Output** (success):
```json
{
  "success": true,
  "data": {
    "message": "All validations passed",
    "validation_results": {
      "schema_validation": {"passed": true, "error": ""},
      "aws_credentials": {"passed": true, "error": ""},
      "cdk_context": {"passed": true, "error": ""}
    }
  },
  "timestamp": "2024-01-15T10:30:00.000Z"
}
```

## CloudFormation Exports

Pipeline stacks can import cluster attributes using CloudFormation exports:

```typescript
import * as cdk from 'aws-cdk-lib';

// Import cluster name
const clusterName = cdk.Fn.importValue('ArbiterCluster-ClusterName');

// Import OIDC provider ARN
const oidcProviderArn = cdk.Fn.importValue('ArbiterCluster-OIDCProviderArn');

// Import kubectl role ARN
const kubectlRoleArn = cdk.Fn.importValue('ArbiterCluster-KubectlRoleArn');

// Import cluster security group ID
const securityGroupId = cdk.Fn.importValue('ArbiterCluster-ClusterSecurityGroupId');
```

## Error Handling

### CDK Construct Errors

The AphexCluster construct throws standard TypeScript errors:

```typescript
try {
  const pipeline = cluster.createPipeline({
    pipelineId: 'my-pipeline',
    policyStatements: [...],
  });
} catch (error) {
  if (error.message.includes('already exists')) {
    // Handle duplicate pipeline
  }
}
```

### Execution Script Errors

Scripts output structured error information to stderr:

```json
{
  "success": false,
  "data": {
    "error": "Failed to clone repository: Permission denied",
    "error_type": "ResourceError",
    "exit_code": 3
  },
  "timestamp": "2024-01-15T10:30:00.000Z"
}
```

**Source**
- `lib/constructs/aphex-cluster.ts`
- `containers/deployer/aphex-deploy-pipeline`
- `containers/deployer/aphex-deploy-stack`
- `containers/tester/aphex-test`
- `containers/validator/aphex-validate`
- `containers/common/aphex_common.py`
