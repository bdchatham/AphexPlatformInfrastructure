#!/usr/bin/env node
import 'source-map-support/register';
import * as cdk from 'aws-cdk-lib';
import { ArbiterPipelineInfrastructureStack } from '../lib/arbiter-pipeline-infrastructure-stack';

const app = new cdk.App();

new ArbiterPipelineInfrastructureStack(app, 'ArbiterPipelineInfrastructureStack', {
  env: {
    account: process.env.CDK_DEFAULT_ACCOUNT,
    region: process.env.CDK_DEFAULT_REGION,
  },
  description: 'Arbiter Pipeline Infrastructure - EKS cluster with Argo Workflows and Argo Events',
});

app.synth();
