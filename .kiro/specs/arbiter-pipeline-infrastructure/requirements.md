# Requirements Document

## Introduction

The Arbiter Pipeline Infrastructure provides the complete execution environment for CDK deployment pipelines across all Arbiter agent products (including Archon and future agents). This infrastructure package deploys and manages an EKS cluster with Argo Workflows and Argo Events, builds and publishes container images with execution scripts, and exports cluster references for pipeline packages to consume. The system enables multiple CDK applications to share a common deployment infrastructure while maintaining isolation and security.

## Glossary

- **Arbiter Pipeline Infrastructure**: The infrastructure package providing the execution environment for CDK deployment pipelines
- **EKS Cluster**: Amazon Elastic Kubernetes Service cluster that hosts the Argo Workflows and Argo Events
- **Argo Workflows**: Kubernetes-native workflow engine for orchestrating parallel jobs
- **Argo Events**: Event-driven workflow automation framework for Kubernetes
- **Container Image**: Docker container image published to a registry containing execution scripts and tools
- **Execution Script**: Shell or Python script embedded in container images that performs specific pipeline tasks
- **CloudFormation Export**: Named output value from a CloudFormation stack that can be referenced by other stacks
- **IRSA**: IAM Roles for Service Accounts - mechanism for granting AWS permissions to Kubernetes pods
- **WorkflowTemplate**: Argo Workflows resource defining a reusable workflow specification
- **Builder Container**: Container image containing build tools for compiling and packaging CDK applications
- **Deployer Container**: Container image containing CDK CLI and deployment tools
- **Tester Container**: Container image containing test execution frameworks
- **Validator Container**: Container image containing configuration validation tools
- **Artifact Bucket**: S3 bucket used to store build artifacts between pipeline stages
- **OIDC Provider**: OpenID Connect identity provider for EKS cluster enabling IRSA

## Requirements

### Requirement 1

**User Story:** As a platform engineer, I want to deploy a shared EKS cluster with Argo Workflows, so that multiple CDK applications can use a common deployment infrastructure.

#### Acceptance Criteria

1. WHEN a user deploys the Arbiter Pipeline Infrastructure stack THEN the system SHALL create an EKS cluster with Argo Workflows installed
2. WHEN the EKS cluster is created THEN the system SHALL configure VPC networking with public and private subnets
3. WHEN the EKS cluster is created THEN the system SHALL configure node groups with autoscaling capabilities
4. WHEN Argo Workflows is installed THEN the system SHALL configure RBAC permissions for workflow execution
5. WHEN the cluster is deployed THEN the system SHALL export the cluster name via CloudFormation exports

### Requirement 2

**User Story:** As a platform engineer, I want Argo Events installed and configured, so that GitHub webhooks can trigger deployment workflows automatically.

#### Acceptance Criteria

1. WHEN the Arbiter Pipeline Infrastructure is deployed THEN the system SHALL install Argo Events on the EKS cluster
2. WHEN Argo Events is installed THEN the system SHALL configure the event bus for receiving webhook events
3. WHEN Argo Events is configured THEN the system SHALL create necessary service accounts with appropriate permissions
4. WHEN the cluster is deployed THEN the system SHALL export the OIDC provider ARN via CloudFormation exports

### Requirement 3

**User Story:** As a pipeline developer, I want pre-built container images with execution scripts, so that I can reference them in my WorkflowTemplates without building custom containers.

#### Acceptance Criteria

1. WHEN the Arbiter Pipeline Infrastructure is built THEN the system SHALL create a Builder container image with Node.js, Python, Git, and AWS CLI
2. WHEN the Arbiter Pipeline Infrastructure is built THEN the system SHALL create a Deployer container image with CDK CLI, kubectl, and AWS CLI
3. WHEN the Arbiter Pipeline Infrastructure is built THEN the system SHALL create a Tester container image with test execution frameworks
4. WHEN the Arbiter Pipeline Infrastructure is built THEN the system SHALL create a Validator container image with configuration validation tools
5. WHEN container images are built THEN the system SHALL publish them to a public container registry

### Requirement 4

**User Story:** As a pipeline developer, I want a build execution script in the Builder container, so that workflows can clone repositories, run build commands, and upload artifacts to S3.

#### Acceptance Criteria

1. WHEN the Builder container includes the aphex-build script THEN the script SHALL accept repository URL, commit SHA, and build commands as parameters
2. WHEN aphex-build executes THEN the script SHALL clone the specified repository at the specified commit
3. WHEN aphex-build executes THEN the script SHALL run the provided build commands in the cloned repository
4. WHEN build commands complete successfully THEN the script SHALL package the build artifacts
5. WHEN artifacts are packaged THEN the script SHALL tag them with commit SHA and timestamp
6. WHEN artifacts are tagged THEN the script SHALL upload them to the specified S3 bucket
7. WHEN the upload completes THEN the script SHALL output the artifact S3 path

### Requirement 5

**User Story:** As a pipeline developer, I want deployment execution scripts in the Deployer container, so that workflows can synthesize and deploy CDK stacks to AWS accounts.

#### Acceptance Criteria

1. WHEN the Deployer container includes aphex-deploy-pipeline script THEN the script SHALL accept repository URL, commit SHA, and stack name as parameters
2. WHEN aphex-deploy-pipeline executes THEN the script SHALL clone the repository and synthesize the specified CDK stack
3. WHEN synthesis completes THEN the script SHALL deploy the stack to CloudFormation
4. WHEN the Deployer container includes aphex-deploy-stack script THEN the script SHALL accept repository URL, commit SHA, environment name, and stack list as parameters
5. WHEN aphex-deploy-stack executes THEN the script SHALL download build artifacts from S3
6. WHEN artifacts are downloaded THEN the script SHALL synthesize the specified CDK stacks for the target environment
7. WHEN synthesis completes THEN the script SHALL deploy the stacks to CloudFormation in dependency order
8. WHEN deployment completes THEN the script SHALL capture and output stack outputs

### Requirement 6

**User Story:** As a pipeline developer, I want a test execution script in the Tester container, so that workflows can run automated tests after deployments.

#### Acceptance Criteria

1. WHEN the Tester container includes the aphex-test script THEN the script SHALL accept test commands as parameters
2. WHEN aphex-test executes THEN the script SHALL run the provided test commands
3. WHEN test commands execute THEN the script SHALL capture stdout and stderr output
4. WHEN test commands complete THEN the script SHALL capture the exit code
5. WHEN tests complete THEN the script SHALL output pass or fail status based on exit code

### Requirement 7

**User Story:** As a pipeline developer, I want a validation script in the Validator container, so that workflows can validate configuration and prerequisites before deployment.

#### Acceptance Criteria

1. WHEN the Validator container includes the aphex-validate script THEN the script SHALL accept a configuration file path as a parameter
2. WHEN aphex-validate executes THEN the script SHALL validate the configuration against a JSON schema
3. WHEN configuration is valid THEN the script SHALL validate AWS credentials are available
4. WHEN AWS credentials are valid THEN the script SHALL validate required CDK context values are present
5. WHEN all validations pass THEN the script SHALL output a success status

### Requirement 8

**User Story:** As a pipeline developer, I want the cluster to export references via CloudFormation, so that pipeline stacks can discover and connect to the cluster.

#### Acceptance Criteria

1. WHEN the Arbiter Pipeline Infrastructure stack deploys THEN the system SHALL export the cluster name with a predictable export name
2. WHEN the Arbiter Pipeline Infrastructure stack deploys THEN the system SHALL export the OIDC provider ARN with a predictable export name
3. WHEN the Arbiter Pipeline Infrastructure stack deploys THEN the system SHALL export the kubectl role ARN with a predictable export name
4. WHEN pipeline stacks reference these exports THEN the system SHALL resolve the exported values correctly

### Requirement 9

**User Story:** As a platform engineer, I want to configure cluster sizing and autoscaling, so that the infrastructure can handle varying workload demands efficiently.

#### Acceptance Criteria

1. WHEN creating the Arbiter Pipeline Infrastructure THEN the system SHALL accept minimum node count as a configuration parameter
2. WHEN creating the Arbiter Pipeline Infrastructure THEN the system SHALL accept maximum node count as a configuration parameter
3. WHEN the cluster is deployed THEN the system SHALL configure node autoscaling between the specified minimum and maximum
4. WHEN workload demand increases THEN the cluster SHALL automatically scale up nodes within the maximum limit
5. WHEN workload demand decreases THEN the cluster SHALL automatically scale down nodes to the minimum limit

### Requirement 10

**User Story:** As a pipeline developer, I want execution scripts to handle errors gracefully, so that workflow failures are clear and actionable.

#### Acceptance Criteria

1. WHEN any execution script encounters an error THEN the script SHALL exit with a non-zero exit code
2. WHEN any execution script encounters an error THEN the script SHALL output structured error information to stderr
3. WHEN any execution script encounters an error THEN the script SHALL log error details to CloudWatch Logs
4. WHEN an execution script succeeds THEN the script SHALL exit with exit code 0
5. WHEN an execution script succeeds THEN the script SHALL output structured success information to stdout

### Requirement 11

**User Story:** As a security engineer, I want execution scripts to use IRSA for AWS permissions, so that pods have least-privilege access to AWS resources without long-lived credentials.

#### Acceptance Criteria

1. WHEN a workflow pod executes THEN the system SHALL use IRSA to provide AWS credentials to the pod
2. WHEN IRSA credentials are used THEN the system SHALL not require long-lived AWS access keys
3. WHEN a pod assumes an IAM role via IRSA THEN the system SHALL enforce the role's permission boundaries
4. WHEN multiple pipelines share the cluster THEN the system SHALL isolate IAM permissions per pipeline using separate service accounts

### Requirement 12

**User Story:** As a pipeline developer, I want container images to be versioned, so that I can pin to specific versions or use the latest version.

#### Acceptance Criteria

1. WHEN container images are published THEN the system SHALL tag them with semantic version numbers
2. WHEN container images are published THEN the system SHALL tag them with the Git commit SHA
3. WHEN container images are published THEN the system SHALL tag the most recent image as "latest"
4. WHEN a pipeline references a container image THEN the system SHALL support both version-specific and latest tags

### Requirement 13

**User Story:** As a platform engineer, I want the cluster to support multiple concurrent pipelines, so that different applications can deploy independently without interference.

#### Acceptance Criteria

1. WHEN multiple pipelines are deployed to the cluster THEN the system SHALL isolate each pipeline's Kubernetes resources using namespaces or labels
2. WHEN multiple workflows execute concurrently THEN the system SHALL schedule them independently without blocking
3. WHEN one pipeline is destroyed THEN the system SHALL not affect other pipelines running on the cluster
4. WHEN the cluster reaches capacity THEN the system SHALL queue additional workflows until resources are available
