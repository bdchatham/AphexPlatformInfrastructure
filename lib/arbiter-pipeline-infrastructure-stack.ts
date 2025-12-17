import * as cdk from 'aws-cdk-lib';
import { Construct } from 'constructs';
import { AphexCluster, AphexClusterProps } from './constructs/aphex-cluster';

/**
 * Properties for the Arbiter Pipeline Infrastructure Stack
 */
export interface ArbiterPipelineInfrastructureStackProps extends cdk.StackProps {
  /**
   * Configuration for the AphexCluster
   */
  readonly clusterProps?: AphexClusterProps;
}

/**
 * Main stack for Arbiter Pipeline Infrastructure
 * 
 * This stack contains the AphexCluster construct and related resources.
 */
export class ArbiterPipelineInfrastructureStack extends cdk.Stack {
  /**
   * The AphexCluster construct
   */
  public readonly cluster: AphexCluster;

  /**
   * The operator IAM user for kubectl access
   */
  public readonly operatorUser: cdk.aws_iam.User;

  constructor(scope: Construct, id: string, props?: ArbiterPipelineInfrastructureStackProps) {
    super(scope, id, props);

    // Create the AphexCluster
    this.cluster = new AphexCluster(this, 'AphexCluster', props?.clusterProps);

    // Create operator IAM user for kubectl access
    this.operatorUser = new cdk.aws_iam.User(this, 'OperatorUser', {
      userName: 'arbiter-operator',
    });

    // Grant the operator user permission to describe EKS clusters
    this.operatorUser.addToPolicy(new cdk.aws_iam.PolicyStatement({
      effect: cdk.aws_iam.Effect.ALLOW,
      actions: [
        'eks:DescribeCluster',
        'eks:ListClusters',
      ],
      resources: ['*'],
    }));

    // Grant the operator user permission to assume the cluster roles
    this.operatorUser.addToPolicy(new cdk.aws_iam.PolicyStatement({
      effect: cdk.aws_iam.Effect.ALLOW,
      actions: ['sts:AssumeRole'],
      resources: [
        this.cluster.breakglassAdminRole.roleArn,
        this.cluster.readOnlyRole.roleArn,
      ],
    }));

    // Grant the operator user permission to read CloudFormation stack outputs
    this.operatorUser.addToPolicy(new cdk.aws_iam.PolicyStatement({
      effect: cdk.aws_iam.Effect.ALLOW,
      actions: ['cloudformation:DescribeStacks'],
      resources: [this.stackId],
    }));

    // Create PipelineCreatorRole for external pipeline stacks
    const pipelineCreatorRole = new cdk.aws_iam.Role(this, 'PipelineCreatorRole', {
      roleName: 'arbiter-pipeline-creator',
      description: 'Role for pipeline stacks to create pipelines on Arbiter cluster',
      assumedBy: new cdk.aws_iam.CompositePrincipal(
        // Allow Lambda functions (for CDK custom resources)
        new cdk.aws_iam.ServicePrincipal('lambda.amazonaws.com'),
        // Allow pipeline stacks in the same account
        new cdk.aws_iam.AccountPrincipal(this.account)
      ),
      managedPolicies: [
        cdk.aws_iam.ManagedPolicy.fromAwsManagedPolicyName('service-role/AWSLambdaBasicExecutionRole'),
      ],
    });

    // Grant permissions needed for pipeline creation
    pipelineCreatorRole.addToPolicy(new cdk.aws_iam.PolicyStatement({
      effect: cdk.aws_iam.Effect.ALLOW,
      actions: [
        // S3 permissions for artifacts
        's3:CreateBucket',
        's3:PutBucketPolicy',
        's3:PutBucketVersioning',
        's3:PutLifecycleConfiguration',
        's3:PutEncryptionConfiguration',
        // IAM permissions for workflow execution roles
        'iam:CreateRole',
        'iam:PutRolePolicy',
        'iam:AttachRolePolicy',
        'iam:GetRole',
        'iam:PassRole',
        'iam:TagRole',
        // Secrets Manager for GitHub tokens
        'secretsmanager:GetSecretValue',
        'secretsmanager:DescribeSecret',
      ],
      resources: ['*'],
    }));

    // Update kubectl role trust policy to trust the pipeline creator role
    // Note: We do this manually instead of using grantAssumeRole() to avoid circular dependencies
    const kubectlRole = this.cluster.cluster.kubectlRole!;
    const kubectlRoleCfn = kubectlRole.node.defaultChild as cdk.aws_iam.CfnRole;
    const existingPolicy = kubectlRoleCfn.assumeRolePolicyDocument as any;
    
    // Get existing statements or initialize empty array
    const existingStatements = Array.isArray(existingPolicy?.Statement) 
      ? existingPolicy.Statement 
      : [];
    
    kubectlRoleCfn.assumeRolePolicyDocument = {
      Version: '2012-10-17',
      Statement: [
        ...existingStatements,
        {
          Effect: 'Allow',
          Principal: {
            AWS: pipelineCreatorRole.roleArn,
          },
          Action: 'sts:AssumeRole',
        },
      ],
    };

    // Output operator user information
    new cdk.CfnOutput(this, 'OperatorUserArn', {
      value: this.operatorUser.userArn,
      description: 'IAM user ARN for cluster operator (use this identity for kubectl access)',
    });

    new cdk.CfnOutput(this, 'OperatorUserName', {
      value: this.operatorUser.userName,
      description: 'IAM user name for cluster operator',
    });

    new cdk.CfnOutput(this, 'CreateAccessKeyCommand', {
      value: `aws iam create-access-key --user-name ${this.operatorUser.userName}`,
      description: 'Command to create access keys for the operator user (run with root/admin credentials)',
    });

    // Output pipeline creator role information
    new cdk.CfnOutput(this, 'PipelineCreatorRoleArn', {
      value: pipelineCreatorRole.roleArn,
      exportName: 'ArbiterCluster-PipelineCreatorRoleArn',
      description: 'Role ARN for creating pipelines on Arbiter cluster',
    });

    new cdk.CfnOutput(this, 'PipelineCreatorRoleName', {
      value: pipelineCreatorRole.roleName,
      exportName: 'ArbiterCluster-PipelineCreatorRoleName',
      description: 'Role name for creating pipelines on Arbiter cluster',
    });
  }
}
