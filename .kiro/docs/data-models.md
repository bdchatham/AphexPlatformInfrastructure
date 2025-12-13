# Data Models

## Overview

The Arbiter Pipeline Infrastructure uses TypeScript interfaces and Python dataclasses to define data structures. There are no persistent databases; all state is managed by Kubernetes (cluster state) and CloudFormation (infrastructure state).

Data flows through the system in three main forms:
1. **CDK Constructs**: TypeScript interfaces for infrastructure configuration
2. **Kubernetes Resources**: YAML manifests for cluster resources
3. **Script I/O**: JSON structures for execution script input/output

## CDK Construct Interfaces

### AphexClusterProps

Configuration for creating an AphexCluster.

```typescript
interface AphexClusterProps {
  clusterName?: string;           // Default: 'arbiter-pipeline-cluster'
  minNodes?: number;               // Default: 2
  maxNodes?: number;               // Default: 10
  instanceType?: ec2.InstanceType; // Default: t3.medium
  kubernetesVersion?: eks.KubernetesVersion; // Default: 1.28
  vpc?: ec2.IVpc;                  // Default: new VPC created
  argoNamespace?: string;          // Default: 'argo'
  enableContainerInsights?: boolean; // Default: true
}
```

**Validation Rules**:
- `minNodes` must be ≥ 1
- `maxNodes` must be ≥ `minNodes`
- `clusterName` must be valid EKS cluster name (alphanumeric and hyphens)
- `kubernetesVersion` must be a supported EKS version

### ClusterAttributes

Attributes for importing an existing cluster.

```typescript
interface ClusterAttributes {
  clusterName: string;      // EKS cluster name
  oidcProviderArn: string;  // OIDC provider ARN
  kubectlRoleArn: string;   // kubectl IAM role ARN
}
```

**Validation Rules**:
- All fields are required
- ARNs must be valid AWS ARN format

### PipelineConfig

Configuration for creating an isolated pipeline.

```typescript
interface PipelineConfig {
  pipelineId: string;                    // Unique identifier
  namespace?: string;                    // Default: 'pipeline-{pipelineId}'
  policyStatements: iam.PolicyStatement[]; // IAM permissions
  labels?: { [key: string]: string };    // Additional labels
}
```

**Validation Rules**:
- `pipelineId` must be unique within the cluster
- `pipelineId` must be valid Kubernetes namespace name (lowercase alphanumeric and hyphens)
- `namespace` must be valid Kubernetes namespace name
- `policyStatements` must be non-empty array

### PipelineResources

Resources created for a pipeline.

```typescript
interface PipelineResources {
  pipelineId: string;                 // Pipeline identifier
  namespace: string;                  // Kubernetes namespace
  serviceAccount: eks.ServiceAccount; // Service account with IRSA
  roleArn: string;                    // IAM role ARN
  labels: { [key: string]: string };  // Applied labels
}
```

## Kubernetes Resource Schemas

### Namespace

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: pipeline-{pipelineId}
  labels:
    arbiter.pipeline/id: {pipelineId}
    arbiter.pipeline/managed-by: aphex-cluster
```

### Service Account (with IRSA)

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {pipelineId}-sa
  namespace: pipeline-{pipelineId}
  annotations:
    eks.amazonaws.com/role-arn: arn:aws:iam::{account}:role/{role-name}
```

### Resource Quota

```yaml
apiVersion: v1
kind: ResourceQuota
metadata:
  name: pipeline-quota
  namespace: pipeline-{pipelineId}
spec:
  hard:
    requests.cpu: "10"
    requests.memory: "20Gi"
    limits.cpu: "20"
    limits.memory: "40Gi"
    persistentvolumeclaims: "10"
```

### Network Policy

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: pipeline-isolation
  namespace: pipeline-{pipelineId}
spec:
  podSelector: {}
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
        - podSelector: {}  # Allow from same namespace
  egress:
    - to:
        - podSelector: {}  # Allow to same namespace
    - to:  # Allow DNS
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: kube-system
          podSelector:
            matchLabels:
              k8s-app: kube-dns
      ports:
        - protocol: UDP
          port: 53
    - to:  # Allow internet (except metadata service)
        - ipBlock:
            cidr: 0.0.0.0/0
            except:
              - 169.254.169.254/32
```

## Script I/O Structures

### Script Result (Output)

All execution scripts output this structure:

```typescript
interface ScriptResult {
  success: boolean;           // Operation success status
  data: {                     // Result data or error information
    [key: string]: any;
  };
  timestamp: string;          // ISO 8601 timestamp
}
```

**Example (Success)**:
```json
{
  "success": true,
  "data": {
    "message": "Operation completed",
    "artifact_path": "s3://bucket/artifacts/abc123.tar.gz"
  },
  "timestamp": "2024-01-15T10:30:00.000Z"
}
```

**Example (Failure)**:
```json
{
  "success": false,
  "data": {
    "error": "Failed to clone repository",
    "error_type": "ResourceError",
    "exit_code": 3
  },
  "timestamp": "2024-01-15T10:30:00.000Z"
}
```

### Build Artifact Metadata

Metadata stored with build artifacts in S3:

```typescript
interface ArtifactMetadata {
  commitSha: string;        // Git commit SHA
  timestamp: string;        // Build timestamp (ISO 8601)
  s3Bucket: string;         // S3 bucket name
  s3Key: string;            // S3 object key
  buildCommands: string[];  // Commands executed
  artifactSize: number;     // Size in bytes
}
```

**S3 Object Naming Convention**:
```
s3://{bucket}/artifacts/{commitSha[:8]}-{timestamp}.tar.gz
```

**Example**:
```
s3://my-artifacts/artifacts/abc12345-20240115T103000Z.tar.gz
```

### Deployment Result

Result from deploying a CDK stack:

```typescript
interface DeploymentResult {
  stackName: string;                    // CloudFormation stack name
  stackId: string;                      // CloudFormation stack ID
  outputs: { [key: string]: string };   // Stack outputs
  status: 'CREATE_COMPLETE' | 'UPDATE_COMPLETE'; // Deployment status
  duration: number;                     // Duration in seconds
}
```

**Example**:
```json
{
  "stackName": "MyApplicationStack",
  "stackId": "arn:aws:cloudformation:us-east-1:123456789012:stack/MyApplicationStack/...",
  "outputs": {
    "ApiEndpoint": "https://api.example.com",
    "BucketName": "my-app-bucket"
  },
  "status": "UPDATE_COMPLETE",
  "duration": 120.5
}
```

### Test Result

Result from executing tests:

```typescript
interface TestResult {
  status: 'pass' | 'fail';      // Test status
  exit_code: number;            // Exit code from test command
  stdout: string;               // Standard output
  stderr: string;               // Standard error
  commands_executed: string[];  // Commands that were run
}
```

**Example**:
```json
{
  "status": "pass",
  "exit_code": 0,
  "stdout": "All tests passed\n",
  "stderr": "",
  "commands_executed": ["npm test", "pytest"]
}
```

### Validation Result

Result from validating configuration:

```typescript
interface ValidationResult {
  schema_validation: {
    passed: boolean;
    error: string;
  };
  aws_credentials: {
    passed: boolean;
    error: string;
  };
  cdk_context: {
    passed: boolean;
    error: string;
  };
}
```

**Example**:
```json
{
  "schema_validation": {
    "passed": true,
    "error": ""
  },
  "aws_credentials": {
    "passed": true,
    "error": ""
  },
  "cdk_context": {
    "passed": true,
    "error": ""
  }
}
```

## Data Flow

### Infrastructure Deployment Flow

```
User Input (AphexClusterProps)
    ↓
CDK Synthesis
    ↓
CloudFormation Template
    ↓
AWS Resources (EKS, VPC, IAM)
    ↓
CloudFormation Exports
    ↓
Pipeline Stacks (Import Exports)
```

### Pipeline Creation Flow

```
User Input (PipelineConfig)
    ↓
AphexCluster.createPipeline()
    ↓
Kubernetes Manifests (Namespace, ServiceAccount, etc.)
    ↓
Kubernetes API
    ↓
PipelineResources (returned to user)
```

### Build and Deploy Flow

```
GitHub Webhook
    ↓
Argo Events (EventSource)
    ↓
Argo Workflows (Workflow)
    ↓
Builder Container (aphex-build)
    ↓
Build Artifacts → S3
    ↓
Deployer Container (aphex-deploy-stack)
    ↓
CloudFormation Stacks
    ↓
Tester Container (aphex-test)
    ↓
Test Results → CloudWatch Logs
```

### Script Execution Flow

```
Workflow Step
    ↓
Container Start (with IRSA)
    ↓
Script Execution (aphex-*)
    ↓
AWS API Calls (using IRSA credentials)
    ↓
Structured JSON Output (stdout/stderr)
    ↓
CloudWatch Logs
```

## Validation Rules

### CDK Construct Validation

Validation is performed by AWS CDK and CloudFormation:
- Resource names must follow AWS naming conventions
- IAM policies must be valid JSON
- VPC CIDR blocks must not overlap
- EKS version must be supported

### Kubernetes Resource Validation

Validation is performed by Kubernetes API server:
- Resource names must be valid DNS labels (lowercase alphanumeric and hyphens)
- Namespaces must be unique
- Service accounts must be in valid namespaces
- Resource quotas must have valid units

### Script Input Validation

Validation is performed by execution scripts:
- Repository URLs must be valid Git URLs
- Commit SHAs must be valid Git commit hashes (40 hex characters)
- S3 bucket names must follow S3 naming rules
- Environment variables must be set (AWS_REGION, AWS_ACCOUNT, etc.)

### Script Output Validation

All scripts must output valid JSON:
- `success` field must be boolean
- `data` field must be object
- `timestamp` field must be ISO 8601 format
- Exit code must match success status (0 for success, non-zero for failure)

**Source**
- `lib/constructs/aphex-cluster.ts`
- `containers/common/aphex_common.py`
- `containers/deployer/aphex-deploy-pipeline`
- `containers/deployer/aphex-deploy-stack`
- `containers/tester/aphex-test`
- `containers/validator/aphex-validate`
