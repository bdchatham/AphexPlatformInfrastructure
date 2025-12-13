# Deployer Container

The Deployer container provides CDK deployment capabilities for the Arbiter Pipeline Infrastructure. It contains tools and scripts for synthesizing and deploying CDK stacks to AWS CloudFormation.

## Base Image

- `node:20-alpine`

## Installed Tools

- Node.js 20.x and npm
- Python 3.11 and pip
- AWS CDK CLI (latest)
- AWS CLI v2
- kubectl (latest stable)
- Git 2.x

## Execution Scripts

### aphex-deploy-pipeline

Deploys pipeline infrastructure by cloning a repository, synthesizing a CDK stack, and deploying it to CloudFormation.

**Usage:**
```bash
aphex-deploy-pipeline <repo-url> <commit-sha> <stack-name> <artifact-bucket>
```

**Arguments:**
- `repo-url`: Git repository URL
- `commit-sha`: Git commit SHA to deploy
- `stack-name`: Name of the CDK stack to deploy
- `artifact-bucket`: S3 bucket for artifacts (for consistency with other scripts)

**Environment Variables:**
- `AWS_REGION`: Target AWS region (required)
- `AWS_ACCOUNT`: Target AWS account (required)
- `AWS_ROLE_ARN`: IAM role to assume for deployment (via IRSA)

**Output:**
Structured JSON with deployment results:
```json
{
  "success": true,
  "data": {
    "message": "Pipeline deployment completed successfully",
    "stack_name": "MyPipelineStack",
    "commit_sha": "abc123...",
    "deployment_result": {
      "stack_name": "MyPipelineStack",
      "outputs": {...},
      "status": "DEPLOYED"
    }
  },
  "timestamp": "2024-01-01T12:00:00.000000"
}
```

**Exit Codes:**
- `0`: Success
- `1`: Invalid arguments
- `2`: Missing environment variables
- `4`: Deployment failure

### aphex-deploy-stack

Deploys application stacks by downloading build artifacts from S3, synthesizing CDK stacks for a target environment, and deploying them in dependency order.

**Usage:**
```bash
aphex-deploy-stack <repo-url> <commit-sha> <environment> <stack-list> <artifact-bucket>
```

**Arguments:**
- `repo-url`: Git repository URL
- `commit-sha`: Git commit SHA to deploy
- `environment`: Target environment name (e.g., dev, staging, prod)
- `stack-list`: Comma-separated list of stack names to deploy
- `artifact-bucket`: S3 bucket containing build artifacts

**Environment Variables:**
- `AWS_REGION`: Target AWS region (required)
- `AWS_ACCOUNT`: Target AWS account (required)
- `AWS_ROLE_ARN`: IAM role to assume for deployment (via IRSA)

**Output:**
Structured JSON with deployment results:
```json
{
  "success": true,
  "data": {
    "message": "Stack deployment completed successfully",
    "environment": "prod",
    "commit_sha": "abc123...",
    "stacks_deployed": 2,
    "deployment_results": [
      {
        "stack_name": "AppStack1",
        "outputs": {...},
        "status": "DEPLOYED",
        "duration": 120.5
      },
      {
        "stack_name": "AppStack2",
        "outputs": {...},
        "status": "DEPLOYED",
        "duration": 95.3
      }
    ]
  },
  "timestamp": "2024-01-01T12:00:00.000000"
}
```

**Exit Codes:**
- `0`: Success
- `1`: Invalid arguments
- `2`: Missing environment variables
- `4`: Deployment failure

## Building the Image

```bash
docker build -t arbiter/deployer:latest .
```

## Running the Container

### Deploy Pipeline Infrastructure

```bash
docker run --rm \
  -e AWS_REGION=us-east-1 \
  -e AWS_ACCOUNT=123456789012 \
  arbiter/deployer:latest \
  aphex-deploy-pipeline \
  https://github.com/example/repo.git \
  abc123def456 \
  MyPipelineStack \
  my-artifact-bucket
```

### Deploy Application Stacks

```bash
docker run --rm \
  -e AWS_REGION=us-east-1 \
  -e AWS_ACCOUNT=123456789012 \
  arbiter/deployer:latest \
  aphex-deploy-stack \
  https://github.com/example/repo.git \
  abc123def456 \
  prod \
  "Stack1,Stack2,Stack3" \
  my-artifact-bucket
```

## Features

- **Repository Cloning**: Clones Git repositories at specific commits
- **Dependency Installation**: Automatically installs npm dependencies
- **CDK Synthesis**: Synthesizes CDK stacks with environment context
- **CloudFormation Deployment**: Deploys stacks with automatic approval
- **Dependency Ordering**: Deploys stacks in dependency order (for aphex-deploy-stack)
- **Output Capture**: Captures and returns stack outputs
- **Artifact Support**: Downloads and extracts build artifacts from S3
- **Structured Output**: Returns JSON-formatted results
- **Error Handling**: Comprehensive error handling with appropriate exit codes
- **CloudWatch Logging**: All operations logged for monitoring

## Security

- Uses IRSA (IAM Roles for Service Accounts) for AWS credentials
- No long-lived credentials stored in the container
- Runs with least-privilege IAM permissions
- Temporary working directories cleaned up after execution

## Requirements Validation

This container satisfies the following requirements:
- **3.2**: Deployer container with CDK CLI, kubectl, and AWS CLI
- **3.5**: Published to public container registry
- **5.1**: aphex-deploy-pipeline accepts required parameters
- **5.2**: Clones repository and synthesizes CDK stack
- **5.3**: Deploys stack to CloudFormation
- **5.4**: aphex-deploy-stack accepts required parameters
- **5.5**: Downloads build artifacts from S3
- **5.6**: Synthesizes CDK stacks for target environment
- **5.7**: Deploys stacks in dependency order
- **5.8**: Captures and outputs stack outputs
