#!/usr/bin/env node
import 'source-map-support/register';
import * as cdk from 'aws-cdk-lib';
import * as iam from 'aws-cdk-lib/aws-iam';
import { ArbiterPipelineInfrastructureStack } from '../lib/arbiter-pipeline-infrastructure-stack';

const app = new cdk.App();

// Get account and region from environment or CDK context
const account = process.env.CDK_DEFAULT_ACCOUNT || app.node.tryGetContext('account');
const region = process.env.CDK_DEFAULT_REGION || app.node.tryGetContext('region');

// Define IAM principals for cluster access
// IMPORTANT: Update these ARNs with your actual IAM users/roles
// You can get your current identity with: aws sts get-caller-identity
const breakglassAdminPrincipals = [
  // Example: Specific IAM user
  // new iam.ArnPrincipal('arn:aws:iam::123456789012:user/alice'),
  
  // Example: Specific IAM role
  // new iam.ArnPrincipal('arn:aws:iam::123456789012:role/AdminRole'),
  
  // Example: AWS SSO role (replace with your actual SSO role ARN)
  // new iam.ArnPrincipal('arn:aws:iam::123456789012:role/aws-reserved/sso.amazonaws.com/*/AWSReservedSSO_AdministratorAccess_*'),
  
  // Temporary: Allow any IAM principal in the account (remove in production)
  new iam.AccountPrincipal(account),
];

const readOnlyPrincipals = [
  // Example: Specific IAM user for read-only access
  // new iam.ArnPrincipal('arn:aws:iam::123456789012:user/bob'),
  
  // Example: Developer role
  // new iam.ArnPrincipal('arn:aws:iam::123456789012:role/DeveloperRole'),
  
  // Temporary: Allow any IAM principal in the account (remove in production)
  new iam.AccountPrincipal(account),
];

new ArbiterPipelineInfrastructureStack(app, 'ArbiterPipelineInfrastructureStack', {
  env: {
    account,
    region,
  },
  description: 'Arbiter Pipeline Infrastructure - EKS cluster with Argo Workflows and Argo Events',
  clusterProps: {
    breakglassAdminPrincipals,
    readOnlyPrincipals,
  },
});

app.synth();
