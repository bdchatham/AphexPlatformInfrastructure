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

  constructor(scope: Construct, id: string, props?: ArbiterPipelineInfrastructureStackProps) {
    super(scope, id, props);

    // Create the AphexCluster
    this.cluster = new AphexCluster(this, 'AphexCluster', props?.clusterProps);
  }
}
