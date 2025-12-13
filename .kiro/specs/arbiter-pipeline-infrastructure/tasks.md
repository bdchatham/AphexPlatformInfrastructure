# Implementation Plan

- [x] 1. Set up project structure and dependencies
  - Create CDK TypeScript project structure with lib/, bin/, and test/ directories
  - Configure package.json with CDK dependencies (@aws-cdk/aws-eks, @aws-cdk/aws-ec2, @aws-cdk/aws-iam)
  - Set up TypeScript configuration with strict mode
  - Configure Jest for testing with ts-jest
  - Install Hypothesis for Python property-based testing
  - _Requirements: All_

- [x] 2. Implement AphexCluster CDK construct
  - Define AphexClusterProps interface with cluster configuration options
  - Implement AphexCluster construct class that creates VPC (or accepts existing VPC)
  - Add EKS cluster creation with managed node groups and autoscaling
  - Configure OIDC provider for IRSA support
  - _Requirements: 1.1, 1.2, 1.3, 2.4, 9.1, 9.2, 9.3_

- [x] 2.1 Write property test for autoscaling configuration
  - **Property 11: Autoscaling configuration**
  - **Validates: Requirements 9.3**

- [x] 3. Add Argo Workflows and Argo Events installation
  - Install Argo Workflows using Helm chart via CDK
  - Install Argo Events using Helm chart via CDK
  - Configure RBAC permissions for workflow execution
  - Configure event bus for webhook events
  - Create necessary service accounts
  - _Requirements: 1.1, 1.4, 2.1, 2.2, 2.3_

- [x] 4. Implement CloudFormation exports
  - Export cluster name with predictable export name (ArbiterCluster-ClusterName)
  - Export OIDC provider ARN with predictable export name
  - Export kubectl role ARN with predictable export name
  - Export cluster security group ID
  - Add static method fromClusterAttributes for importing existing clusters
  - _Requirements: 1.5, 2.4, 8.1, 8.2, 8.3_

- [x] 4.1 Write property test for export resolution
  - **Property 10: CloudFormation export resolution**
  - **Validates: Requirements 8.4**

- [x] 5. Create Builder container image
  - Write Dockerfile based on node:20-alpine
  - Install Node.js 20.x, Python 3.11, Git, AWS CLI v2, and build tools
  - Add aphex-build script to /usr/local/bin/
  - Configure container entrypoint and working directory
  - _Requirements: 3.1, 3.5_

- [x] 6. Implement aphex-build execution script
  - Create Python script that accepts repo URL, commit SHA, build commands, and artifact bucket
  - Implement repository cloning at specific commit using Git
  - Implement build command execution with output capture
  - Implement artifact packaging with commit SHA and timestamp tagging
  - Implement S3 upload with artifact path output
  - Add structured JSON output for success/failure
  - Add error handling with appropriate exit codes
  - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5, 4.6, 4.7, 10.1, 10.2, 10.3, 10.4, 10.5_

- [x] 6.1 Write property test for repository cloning
  - **Property 2: Repository cloning at specific commits**
  - **Validates: Requirements 4.2**

- [x] 6.2 Write property test for build command execution
  - **Property 3: Build command execution**
  - **Validates: Requirements 4.3**

- [x] 6.3 Write property test for artifact upload
  - **Property 4: Artifact upload with metadata**
  - **Validates: Requirements 4.4, 4.5, 4.6, 4.7**

- [x] 7. Create Deployer container image
  - Write Dockerfile based on node:20-alpine
  - Install Node.js 20.x, Python 3.11, AWS CDK CLI, AWS CLI v2, kubectl, and Git
  - Add aphex-deploy-pipeline script to /usr/local/bin/
  - Add aphex-deploy-stack script to /usr/local/bin/
  - Configure container entrypoint and working directory
  - _Requirements: 3.2, 3.5_

- [x] 8. Implement aphex-deploy-pipeline execution script
  - Create Python script that accepts repo URL, commit SHA, stack name, and artifact bucket
  - Implement repository cloning at specific commit
  - Implement CDK stack synthesis using cdk synth
  - Implement CloudFormation deployment using cdk deploy
  - Add structured JSON output with deployment results
  - Add error handling with appropriate exit codes
  - _Requirements: 5.1, 5.2, 5.3, 10.1, 10.2, 10.3, 10.4, 10.5_

- [x] 8.1 Write property test for CDK synthesis and deployment
  - **Property 5: CDK stack synthesis and deployment**
  - **Validates: Requirements 5.2, 5.3**

- [x] 9. Implement aphex-deploy-stack execution script
  - Create Python script that accepts repo URL, commit SHA, environment, stack list, and artifact bucket
  - Implement artifact download from S3
  - Implement CDK stack synthesis for target environment
  - Implement dependency-ordered stack deployment
  - Implement stack output capture and structured output
  - Add error handling with appropriate exit codes
  - _Requirements: 5.4, 5.5, 5.6, 5.7, 5.8, 10.1, 10.2, 10.3, 10.4, 10.5_

- [x] 9.1 Write property test for artifact-based deployment
  - **Property 6: Artifact-based deployment**
  - **Validates: Requirements 5.5, 5.6, 5.8**

- [x] 9.2 Write property test for dependency-ordered deployment
  - **Property 7: Dependency-ordered deployment**
  - **Validates: Requirements 5.7**

- [x] 10. Create Tester container image
  - Write Dockerfile based on node:20-alpine
  - Install Node.js 20.x, Python 3.11, Jest, Mocha, Pytest, and AWS CLI v2
  - Add aphex-test script to /usr/local/bin/
  - Configure container entrypoint and working directory
  - _Requirements: 3.3, 3.5_

- [x] 11. Implement aphex-test execution script
  - Create Python script that accepts test commands
  - Implement test command execution with stdout/stderr capture
  - Implement exit code capture
  - Implement pass/fail status determination based on exit code
  - Add structured JSON output with test results
  - Add error handling with appropriate exit codes
  - _Requirements: 6.1, 6.2, 6.3, 6.4, 6.5, 10.1, 10.2, 10.3, 10.4, 10.5_

- [x] 11.1 Write property test for test execution
  - **Property 8: Test execution and result capture**
  - **Validates: Requirements 6.2, 6.3, 6.4, 6.5**

- [x] 12. Create Validator container image
  - Write Dockerfile based on python:3.11-slim
  - Install Python 3.11, jsonschema, PyYAML, and AWS CLI v2
  - Add aphex-validate script to /usr/local/bin/
  - Configure container entrypoint and working directory
  - _Requirements: 3.4, 3.5_

- [x] 13. Implement aphex-validate execution script
  - Create Python script that accepts configuration file path
  - Implement JSON schema validation
  - Implement AWS credentials validation
  - Implement CDK context validation
  - Implement validation sequence with success output only if all pass
  - Add structured JSON output with validation results
  - Add error handling with appropriate exit codes
  - _Requirements: 7.1, 7.2, 7.3, 7.4, 7.5, 10.1, 10.2, 10.3, 10.4, 10.5_

- [x] 13.1 Write property test for configuration validation
  - **Property 9: Configuration validation sequence**
  - **Validates: Requirements 7.2, 7.3, 7.4, 7.5**

- [x] 14. Implement common script utilities
  - Create shared Python module for structured output formatting
  - Create shared module for CloudWatch logging setup
  - Create shared module for AWS credential handling via IRSA
  - Create shared module for error handling and exit codes
  - _Requirements: 10.1, 10.2, 10.3, 10.4, 10.5_

- [x] 14.1 Write property test for script exit codes
  - **Property 12: Script exit code behavior**
  - **Validates: Requirements 10.1, 10.4**

- [x] 14.2 Write property test for structured output
  - **Property 13: Structured script output**
  - **Validates: Requirements 10.2, 10.5**

- [x] 14.3 Write property test for error logging
  - **Property 14: Error logging**
  - **Validates: Requirements 10.3**

- [x] 15. Implement container image build and publish pipeline
  - Create build script that builds all four container images
  - Implement semantic versioning tagging (v1.0.0, v1.1.0, etc.)
  - Implement Git commit SHA tagging
  - Implement "latest" tag for most recent image
  - Configure push to public container registry (ECR or Docker Hub)
  - _Requirements: 3.5, 12.1, 12.2, 12.3_

- [x] 15.1 Write property test for image tagging
  - **Property 17: Container image tagging**
  - **Validates: Requirements 12.1, 12.2, 12.3**

- [x] 15.2 Write property test for image tag resolution
  - **Property 18: Image tag resolution**
  - **Validates: Requirements 12.4**

- [x] 16. Implement IRSA configuration
  - Add OIDC provider configuration to AphexCluster construct
  - Create service account creation utilities
  - Implement IAM role creation with trust policy for IRSA
  - Add role assumption logic to execution scripts
  - Ensure no long-lived credentials are used
  - _Requirements: 11.1, 11.2_

- [x] 16.1 Write property test for IRSA credentials
  - **Property 15: IRSA credential provisioning**
  - **Validates: Requirements 11.1, 11.2**

- [x] 17. Implement multi-pipeline isolation
  - Add namespace or label-based resource isolation to cluster configuration
  - Create separate service accounts per pipeline
  - Implement IAM permission isolation per pipeline
  - Add pipeline deletion logic that preserves other pipelines
  - _Requirements: 11.4, 13.1, 13.3_

- [x] 17.1 Write property test for pipeline permission isolation
  - **Property 16: Pipeline permission isolation**
  - **Validates: Requirements 11.4**

- [x] 17.2 Write property test for resource isolation
  - **Property 19: Pipeline resource isolation**
  - **Validates: Requirements 13.1**

- [x] 17.3 Write property test for deletion isolation
  - **Property 20: Pipeline deletion isolation**
  - **Validates: Requirements 13.3**

- [x] 18. Write unit tests for AphexCluster construct
  - Test cluster creation with default parameters
  - Test cluster creation with custom VPC
  - Test CloudFormation export names are correct
  - Test OIDC provider is created
  - Test Argo Workflows Helm chart is installed
  - Test Argo Events Helm chart is installed
  - Test fromClusterAttributes static method
  - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 2.1, 2.2, 2.3, 2.4, 8.1, 8.2, 8.3_

- [x] 19. Write unit tests for container images
  - Test Builder image has required tools installed
  - Test Deployer image has required tools installed
  - Test Tester image has required tools installed
  - Test Validator image has required tools installed
  - Test scripts are executable
  - Test base images are correct
  - _Requirements: 3.1, 3.2, 3.3, 3.4_

- [x] 20. Write integration tests
  - Test end-to-end cluster deployment
  - Test container image build and publish
  - Test workflow execution with all stages
  - _Requirements: All_

- [x] 21. Create documentation
  - Write README with getting started guide
  - Document AphexCluster construct API
  - Document container image specifications
  - Document execution script interfaces
  - Create examples for common use cases
  - Document CloudFormation exports
  - _Requirements: All_

- [x] 22. Checkpoint - Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.
