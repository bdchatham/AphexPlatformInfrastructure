#!/usr/bin/env node
import 'source-map-support/register';
import * as cdk from 'aws-cdk-lib';
import * as iam from 'aws-cdk-lib/aws-iam';
import { ArbiterPipelineInfrastructureStack } from '../lib/arbiter-pipeline-infrastructure-stack';

const app = new cdk.App();

// Get account and region from environment or CDK context
const account = process.env.CDK_DEFAULT_ACCOUNT || app.node.tryGetContext('account');
const region = process.env.CDK_DEFAULT_REGION || app.node.tryGetContext('region');

// Define IAM principals for cluster access using AWS SSO (Identity Center)
// The AWSReservedSSO role is created automatically when you assign a permission set
// Pattern: arn:aws:iam::<account>:role/aws-reserved/sso.amazonaws.com/<region>/AWSReservedSSO_<PermissionSetName>_<suffix>

// For ArbiterDevs group with ArbiterDevsBase permission set
// You can find the exact role name with: aws iam list-roles | grep AWSReservedSSO_ArbiterDevsBase
const ssoRolePattern = `arn:aws:iam::${account}:role/aws-reserved/sso.amazonaws.com/*/AWSReservedSSO_ArbiterDevsBase_*`;

const breakglassAdminPrincipals = [
  // AWS SSO role for ArbiterDevs group (emergency admin access)
  new iam.ArnPrincipal(ssoRolePattern),
];

const readOnlyPrincipals = [
  // AWS SSO role for ArbiterDevs group (daily read-only access)
  new iam.ArnPrincipal(ssoRolePattern),
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
