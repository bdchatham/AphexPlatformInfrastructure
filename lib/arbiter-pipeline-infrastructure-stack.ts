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
  }
}
