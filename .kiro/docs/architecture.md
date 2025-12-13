# Architecture

## System Design

The Arbiter Pipeline Infrastructure follows a three-layer architecture:

```
┌─────────────────────────────────────────────────────────────┐
│                    Infrastructure Layer                      │
│  (EKS Cluster + Argo Workflows + Argo Events)               │
│  Created by AphexCluster CDK Construct                      │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│                      Container Layer                         │
│  (Pre-built Docker images with execution scripts)           │
│  Builder | Deployer | Tester | Validator                    │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│                    Integration Layer                         │
│  (CloudFormation Exports + IRSA Configuration)              │
│  Consumed by pipeline packages                              │
└─────────────────────────────────────────────────────────────┘
```

### Design Principles

1. **Separation of Concerns**: Infrastructure, orchestration, and execution are cleanly separated
2. **Multi-Tenancy**: Multiple pipelines share the cluster with strong isolation guarantees
3. **Security First**: IRSA for AWS permissions, no long-lived credentials, network policies
4. **Reusability**: Generic container images work with any CDK project
5. **Declarative**: CDK constructs define infrastructure as code

## Components

### 1. AphexCluster Construct

The main CDK construct that creates the complete infrastructure stack.

**Location**: `lib/constructs/aphex-cluster.ts`

**Responsibilities**:
- Create or reference VPC with public/private subnets
- Provision EKS cluster with managed node groups
- Configure cluster autoscaling (min/max nodes)
- Install Argo Workflows via Helm chart
- Install Argo Events via Helm chart
- Create OIDC provider for IRSA
- Export cluster attributes via CloudFormation
- Provide methods for creating isolated pipelines

**Key Methods**:
- `createServiceAccountWithRole()`: Create a service account with IRSA
- `createPipeline()`: Create an isolated pipeline with namespace and service account
- `deletePipeline()`: Remove a pipeline without affecting others
- `fromClusterAttributes()`: Import an existing cluster

### 2. Container Images

Four specialized container images, each with specific tools and execution scripts.

#### Builder Image
**Base**: `node:20-alpine`  
**Location**: `containers/builder/`  
**Tools**: Node.js 20, Python 3.11, Git, AWS CLI v2, build tools  
**Script**: `/usr/local/bin/aphex-build`  
**Purpose**: Clone repos, run build commands, upload artifacts to S3

#### Deployer Image
**Base**: `node:20-alpine`  
**Location**: `containers/deployer/`  
**Tools**: Node.js 20, Python 3.11, AWS CDK CLI, kubectl, AWS CLI v2, Git  
**Scripts**: `/usr/local/bin/aphex-deploy-pipeline`, `/usr/local/bin/aphex-deploy-stack`  
**Purpose**: Synthesize and deploy CDK stacks to CloudFormation

#### Tester Image
**Base**: `node:20-alpine`  
**Location**: `containers/tester/`  
**Tools**: Node.js 20, Python 3.11, Jest, Mocha, Pytest, AWS CLI v2  
**Script**: `/usr/local/bin/aphex-test`  
**Purpose**: Execute test commands and report results

#### Validator Image
**Base**: `python:3.11-slim`  
**Location**: `containers/validator/`  
**Tools**: Python 3.11, jsonschema, PyYAML, AWS CLI v2  
**Script**: `/usr/local/bin/aphex-validate`  
**Purpose**: Validate configuration files and prerequisites

### 3. Common Utilities

Shared Python module providing consistent functionality across all execution scripts.

**Location**: `containers/common/aphex_common.py`

**Provides**:
- Structured JSON output formatting
- CloudWatch logging setup
- AWS credential handling via IRSA
- Error handling with standard exit codes
- Input validation utilities

### 4. CloudFormation Exports

The stack exports key attributes for pipeline consumption:

| Export Name | Value | Purpose |
|-------------|-------|---------|
| `ArbiterCluster-ClusterName` | EKS cluster name | Pipeline stacks reference cluster |
| `ArbiterCluster-OIDCProviderArn` | OIDC provider ARN | Pipeline stacks create IRSA roles |
| `ArbiterCluster-KubectlRoleArn` | kubectl IAM role ARN | Pipeline stacks interact with cluster |
| `ArbiterCluster-ClusterSecurityGroupId` | Cluster security group ID | Pipeline stacks configure networking |

## Technology Stack

### Infrastructure
- **AWS CDK**: Infrastructure as code framework (TypeScript)
- **Amazon EKS**: Managed Kubernetes service
- **Amazon EC2**: Compute instances for EKS nodes
- **Amazon VPC**: Network isolation
- **AWS IAM**: Identity and access management
- **CloudFormation**: AWS resource provisioning

### Orchestration
- **Argo Workflows**: Kubernetes-native workflow engine
- **Argo Events**: Event-driven workflow automation
- **Helm**: Kubernetes package manager

### Container Runtime
- **Docker**: Container image format
- **Kubernetes**: Container orchestration
- **containerd**: Container runtime

### Development Tools
- **TypeScript**: Primary language for CDK constructs
- **Python**: Execution scripts and property-based tests
- **Node.js**: JavaScript runtime
- **Jest**: TypeScript testing framework
- **Hypothesis**: Python property-based testing
- **fast-check**: TypeScript property-based testing

### CI/CD
- **GitHub Actions**: Continuous integration (planned)
- **AWS ECR**: Container registry (planned)

## Architectural Patterns

### 1. Construct Pattern (CDK)
The AphexCluster follows the AWS CDK construct pattern, encapsulating complex infrastructure into a reusable component with a clean API.

### 2. Sidecar Pattern
Execution scripts run as containers in Kubernetes pods, with IRSA providing AWS credentials via a sidecar token file.

### 3. Event-Driven Architecture
Argo Events listens for GitHub webhooks and triggers Argo Workflows, enabling automated deployments on code changes.

### 4. Multi-Tenancy with Isolation
Multiple pipelines share the cluster but are isolated through:
- Kubernetes namespaces (one per pipeline)
- Network policies (restrict inter-namespace traffic)
- Resource quotas (prevent resource exhaustion)
- RBAC (separate service accounts and roles)
- IRSA (isolated AWS permissions per pipeline)

### 5. Immutable Infrastructure
Container images are versioned and immutable. Infrastructure changes are deployed through CDK, not manual modifications.

### 6. Structured Logging
All execution scripts output structured JSON to stdout/stderr, which is captured by CloudWatch Logs for monitoring and debugging.

## Dependencies

### Upstream Dependencies

**AWS Services**:
- Amazon EKS (cluster management)
- Amazon EC2 (compute instances)
- Amazon VPC (networking)
- AWS IAM (permissions)
- Amazon S3 (artifact storage)
- CloudWatch Logs (logging)

**Third-Party Services**:
- Argo Workflows (workflow orchestration)
- Argo Events (event processing)
- Helm (package management)

**Development Dependencies**:
- Node.js 20.x
- Python 3.11
- AWS CDK CLI
- Docker

### Downstream Dependencies

**Consuming Packages**:
- Archon deployment pipelines (planned)
- Future Arbiter agent pipelines (planned)

**Integration Points**:
- Pipeline packages import cluster via CloudFormation exports
- Pipeline packages create IRSA roles using the OIDC provider
- Pipeline packages reference container images from registry
- Pipeline packages create WorkflowTemplates that use execution scripts

**Source**
- `lib/constructs/aphex-cluster.ts`
- `lib/arbiter-pipeline-infrastructure-stack.ts`
- `containers/common/aphex_common.py`
- `containers/builder/Dockerfile`
- `containers/deployer/Dockerfile`
- `containers/tester/Dockerfile`
- `containers/validator/Dockerfile`
- `.kiro/specs/arbiter-pipeline-infrastructure/design.md`
