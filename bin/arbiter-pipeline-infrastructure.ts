#!/usr/bin/env node
import 'source-map-support/register';
import * as cdk from 'aws-cdk-lib';
import * as iam from 'aws-cdk-lib/aws-iam';
import { ArbiterPipelineInfrastructureStack } from '../lib/arbiter-pipeline-infrastructure-stack';

const app = new cdk.App();

// Get account and region from environment or CDK context
const account = process.env.CDK_DEFAULT_ACCOUNT || app.node.tryGetContext('account');
const region = process.env.CDK_DEFAULT_REGION || app.node.tryGetContext('region');

// For the simplified prototype approach, we'll use AccountPrincipal
// and create the operator user separately after stack creation
// This avoids circular dependency issues

const breakglassAdminPrincipals = [new iam.AccountPrincipal(account)];
const readOnlyPrincipals = [new iam.AccountPrincipal(account)];

console.log('⚠️  Using account-level access for cluster roles.');
console.log('   After deployment, create an operator user and configure access keys.');

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
