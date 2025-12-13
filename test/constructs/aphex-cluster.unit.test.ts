/**
 * Unit tests for AphexCluster construct
 * 
 * These tests verify specific examples and edge cases for the AphexCluster construct.
 * They complement the property-based tests by testing concrete scenarios.
 * 
 * Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 2.1, 2.2, 2.3, 2.4, 8.1, 8.2, 8.3
 */

import * as cdk from 'aws-cdk-lib';
import * as ec2 from 'aws-cdk-lib/aws-ec2';
import * as eks from 'aws-cdk-lib/aws-eks';
import { Template, Match } from 'aws-cdk-lib/assertions';
import { AphexCluster } from '../../lib/constructs/aphex-cluster';

describe('AphexCluster Unit Tests', () => {
  describe('Cluster creation with default parameters', () => {
    it('should create a cluster with default configuration', () => {
      // Arrange
      const app = new cdk.App();
      const stack = new cdk.Stack(app, 'TestStack');

      // Act
      const cluster = new AphexCluster(stack, 'TestCluster');

      // Assert
      const template = Template.fromStack(stack);

      // Verify EKS cluster is created
      template.hasResourceProperties('Custom::AWSCDK-EKS-Cluster', {
        Config: Match.objectLike({
          name: 'arbiter-pipeline-cluster',
          version: '1.28',
        }),
      });

      // Verify node group with default settings
      template.hasResourceProperties('AWS::EKS::Nodegroup', {
        ScalingConfig: {
          MinSize: 2,
          MaxSize: 10,
          DesiredSize: 2,
        },
      });

      // Verify cluster properties
      expect(cluster.cluster).toBeDefined();
      expect(cluster.oidcProvider).toBeDefined();
      expect(cluster.kubectlRoleArn).toBeDefined();
    });

    it('should create a VPC when none is provided', () => {
      // Arrange
      const app = new cdk.App();
      const stack = new cdk.Stack(app, 'TestStack');

      // Act
      new AphexCluster(stack, 'TestCluster');

      // Assert
      const template = Template.fromStack(stack);

      // Verify VPC is created with correct configuration
      template.hasResourceProperties('AWS::EC2::VPC', {
        EnableDnsHostnames: true,
        EnableDnsSupport: true,
      });

      // Verify public and private subnets exist (at least 2 for multi-AZ)
      const subnets = template.findResources('AWS::EC2::Subnet');
      expect(Object.keys(subnets).length).toBeGreaterThanOrEqual(2);
      
      // Verify NAT gateway exists
      template.resourceCountIs('AWS::EC2::NatGateway', 1);
    });

    it('should install Argo Workflows in the default namespace', () => {
      // Arrange
      const app = new cdk.App();
      const stack = new cdk.Stack(app, 'TestStack');

      // Act
      new AphexCluster(stack, 'TestCluster');

      // Assert
      const template = Template.fromStack(stack);

      // Verify Argo namespace is created
      const kubernetesResources = template.findResources('Custom::AWSCDK-EKS-KubernetesResource');
      const argoNamespace = Object.values(kubernetesResources).find((resource: any) => {
        const manifest = resource.Properties?.Manifest;
        if (!manifest) return false;
        const manifestStr = typeof manifest === 'string' ? manifest : JSON.stringify(manifest);
        return manifestStr.includes('"kind":"Namespace"') && manifestStr.includes('"name":"argo"');
      });

      expect(argoNamespace).toBeDefined();

      // Verify Argo Workflows Helm chart is installed
      template.hasResourceProperties('Custom::AWSCDK-EKS-HelmChart', {
        Chart: 'argo-workflows',
        Repository: 'https://argoproj.github.io/argo-helm',
        Namespace: 'argo',
      });
    });

    it('should install Argo Events in the default namespace', () => {
      // Arrange
      const app = new cdk.App();
      const stack = new cdk.Stack(app, 'TestStack');

      // Act
      new AphexCluster(stack, 'TestCluster');

      // Assert
      const template = Template.fromStack(stack);

      // Verify Argo Events Helm chart is installed
      template.hasResourceProperties('Custom::AWSCDK-EKS-HelmChart', {
        Chart: 'argo-events',
        Repository: 'https://argoproj.github.io/argo-helm',
        Namespace: 'argo',
      });
    });

    it('should enable Container Insights by default', () => {
      // Arrange
      const app = new cdk.App();
      const stack = new cdk.Stack(app, 'TestStack');

      // Act
      new AphexCluster(stack, 'TestCluster');

      // Assert
      const template = Template.fromStack(stack);

      // Verify Container Insights addon is created
      template.hasResourceProperties('AWS::EKS::Addon', {
        AddonName: 'amazon-cloudwatch-observability',
      });
    });
  });

  describe('Cluster creation with custom VPC', () => {
    it('should use provided VPC instead of creating a new one', () => {
      // Arrange
      const app = new cdk.App();
      const stack = new cdk.Stack(app, 'TestStack');
      
      // Create a custom VPC
      const customVpc = new ec2.Vpc(stack, 'CustomVpc', {
        maxAzs: 2,
        ipAddresses: ec2.IpAddresses.cidr('10.0.0.0/16'),
      });

      // Act
      new AphexCluster(stack, 'TestCluster', {
        vpc: customVpc,
      });

      // Assert
      const template = Template.fromStack(stack);

      // Verify only one VPC exists (the custom one)
      template.resourceCountIs('AWS::EC2::VPC', 1);

      // Verify the VPC has the custom CIDR
      template.hasResourceProperties('AWS::EC2::VPC', {
        CidrBlock: '10.0.0.0/16',
      });
    });

    it('should create cluster with custom parameters', () => {
      // Arrange
      const app = new cdk.App();
      const stack = new cdk.Stack(app, 'TestStack');

      // Act
      new AphexCluster(stack, 'TestCluster', {
        clusterName: 'custom-cluster',
        minNodes: 3,
        maxNodes: 15,
        instanceType: ec2.InstanceType.of(ec2.InstanceClass.T3, ec2.InstanceSize.LARGE),
        argoNamespace: 'custom-argo',
        enableContainerInsights: false,
      });

      // Assert
      const template = Template.fromStack(stack);

      // Verify cluster name
      template.hasResourceProperties('Custom::AWSCDK-EKS-Cluster', {
        Config: Match.objectLike({
          name: 'custom-cluster',
        }),
      });

      // Verify node group scaling
      template.hasResourceProperties('AWS::EKS::Nodegroup', {
        ScalingConfig: {
          MinSize: 3,
          MaxSize: 15,
          DesiredSize: 3,
        },
      });

      // Verify Argo components use custom namespace
      template.hasResourceProperties('Custom::AWSCDK-EKS-HelmChart', {
        Chart: 'argo-workflows',
        Namespace: 'custom-argo',
      });

      // Verify Container Insights is not enabled
      template.resourceCountIs('AWS::EKS::Addon', 0);
    });
  });

  describe('CloudFormation export names', () => {
    it('should export cluster name with correct export name', () => {
      // Arrange
      const app = new cdk.App();
      const stack = new cdk.Stack(app, 'TestStack');

      // Act
      const cluster = new AphexCluster(stack, 'TestCluster', {
        clusterName: 'test-cluster',
      });

      // Assert
      const template = Template.fromStack(stack);

      // Verify export name property
      expect(cluster.clusterNameExport).toBe('ArbiterCluster-ClusterName');

      // Verify CloudFormation output with export exists
      const outputs = template.findOutputs('*');
      const clusterNameOutput = Object.values(outputs).find((output: any) => 
        output.Export?.Name === 'ArbiterCluster-ClusterName'
      );
      expect(clusterNameOutput).toBeDefined();
    });

    it('should export OIDC provider ARN with correct export name', () => {
      // Arrange
      const app = new cdk.App();
      const stack = new cdk.Stack(app, 'TestStack');

      // Act
      const cluster = new AphexCluster(stack, 'TestCluster');

      // Assert
      const template = Template.fromStack(stack);

      // Verify export name property
      expect(cluster.oidcProviderArnExport).toBe('ArbiterCluster-OIDCProviderArn');

      // Verify CloudFormation output with export exists
      const outputs = template.findOutputs('*');
      const oidcOutput = Object.values(outputs).find((output: any) => 
        output.Export?.Name === 'ArbiterCluster-OIDCProviderArn'
      );
      expect(oidcOutput).toBeDefined();
    });

    it('should export kubectl role ARN with correct export name', () => {
      // Arrange
      const app = new cdk.App();
      const stack = new cdk.Stack(app, 'TestStack');

      // Act
      new AphexCluster(stack, 'TestCluster');

      // Assert
      const template = Template.fromStack(stack);

      // Verify CloudFormation output with export exists
      const outputs = template.findOutputs('*');
      const kubectlOutput = Object.values(outputs).find((output: any) => 
        output.Export?.Name === 'ArbiterCluster-KubectlRoleArn'
      );
      expect(kubectlOutput).toBeDefined();
    });

    it('should export cluster security group ID with correct export name', () => {
      // Arrange
      const app = new cdk.App();
      const stack = new cdk.Stack(app, 'TestStack');

      // Act
      new AphexCluster(stack, 'TestCluster');

      // Assert
      const template = Template.fromStack(stack);

      // Verify CloudFormation output with export exists
      const outputs = template.findOutputs('*');
      const securityGroupOutput = Object.values(outputs).find((output: any) => 
        output.Export?.Name === 'ArbiterCluster-ClusterSecurityGroupId'
      );
      expect(securityGroupOutput).toBeDefined();
    });
  });


  describe('OIDC provider creation', () => {
    it('should create OIDC provider for IRSA', () => {
      // Arrange
      const app = new cdk.App();
      const stack = new cdk.Stack(app, 'TestStack');

      // Act
      const cluster = new AphexCluster(stack, 'TestCluster');

      // Assert
      // Verify OIDC provider is accessible
      expect(cluster.oidcProvider).toBeDefined();
      expect(cluster.oidcProvider.openIdConnectProviderArn).toBeDefined();

      const template = Template.fromStack(stack);

      // Verify OIDC provider resource exists
      // Note: EKS cluster automatically creates OIDC provider
      template.hasResourceProperties('Custom::AWSCDKOpenIdConnectProvider', {
        Url: Match.anyValue(),
      });
    });

    it('should allow service accounts to use IRSA', () => {
      // Arrange
      const app = new cdk.App();
      const stack = new cdk.Stack(app, 'TestStack');
      const cluster = new AphexCluster(stack, 'TestCluster');

      // Act
      const serviceAccount = cluster.createServiceAccountWithRole(
        'TestServiceAccount',
        'test-namespace',
        'test-sa',
        [
          new cdk.aws_iam.PolicyStatement({
            effect: cdk.aws_iam.Effect.ALLOW,
            actions: ['s3:GetObject'],
            resources: ['arn:aws:s3:::test-bucket/*'],
          }),
        ]
      );

      // Assert
      expect(serviceAccount).toBeDefined();
      expect(serviceAccount.role).toBeDefined();

      const template = Template.fromStack(stack);

      // Verify IAM role with OIDC trust policy
      template.hasResourceProperties('AWS::IAM::Role', {
        AssumeRolePolicyDocument: {
          Statement: Match.arrayWith([
            Match.objectLike({
              Action: 'sts:AssumeRoleWithWebIdentity',
              Principal: {
                Federated: Match.anyValue(),
              },
            }),
          ]),
        },
      });
    });
  });


  describe('Argo Workflows Helm chart installation', () => {
    it('should install Argo Workflows with correct configuration', () => {
      // Arrange
      const app = new cdk.App();
      const stack = new cdk.Stack(app, 'TestStack');

      // Act
      new AphexCluster(stack, 'TestCluster');

      // Assert
      const template = Template.fromStack(stack);

      // Verify Helm chart installation
      template.hasResourceProperties('Custom::AWSCDK-EKS-HelmChart', {
        Chart: 'argo-workflows',
        Repository: 'https://argoproj.github.io/argo-helm',
        Namespace: 'argo',
        Values: Match.stringLikeRegexp('.*server.*'),
      });
    });

    it('should configure RBAC for workflow execution', () => {
      // Arrange
      const app = new cdk.App();
      const stack = new cdk.Stack(app, 'TestStack');

      // Act
      new AphexCluster(stack, 'TestCluster');

      // Assert
      const template = Template.fromStack(stack);

      // Verify Helm chart has RBAC configuration in values
      const helmCharts = template.findResources('Custom::AWSCDK-EKS-HelmChart');
      const argoWorkflowsChart = Object.values(helmCharts).find((chart: any) => {
        return chart.Properties?.Chart === 'argo-workflows';
      });

      expect(argoWorkflowsChart).toBeDefined();
      
      // Verify values contain RBAC configuration
      const values = (argoWorkflowsChart as any).Properties?.Values;
      expect(values).toBeDefined();
      
      // Values are JSON stringified, so we check the string
      const valuesStr = typeof values === 'string' ? values : JSON.stringify(values);
      expect(valuesStr).toContain('rbac');
    });

    it('should create workflow service accounts', () => {
      // Arrange
      const app = new cdk.App();
      const stack = new cdk.Stack(app, 'TestStack');

      // Act
      new AphexCluster(stack, 'TestCluster');

      // Assert
      const template = Template.fromStack(stack);

      // Verify Helm chart values include service account configuration
      const helmCharts = template.findResources('Custom::AWSCDK-EKS-HelmChart');
      const argoWorkflowsChart = Object.values(helmCharts).find((chart: any) => {
        return chart.Properties?.Chart === 'argo-workflows';
      });

      const values = (argoWorkflowsChart as any).Properties?.Values;
      const valuesStr = typeof values === 'string' ? values : JSON.stringify(values);
      
      // Verify service account configuration
      expect(valuesStr).toContain('serviceAccount');
      expect(valuesStr).toContain('argo-workflow');
    });
  });


  describe('Argo Events Helm chart installation', () => {
    it('should install Argo Events with correct configuration', () => {
      // Arrange
      const app = new cdk.App();
      const stack = new cdk.Stack(app, 'TestStack');

      // Act
      new AphexCluster(stack, 'TestCluster');

      // Assert
      const template = Template.fromStack(stack);

      // Verify Helm chart installation
      template.hasResourceProperties('Custom::AWSCDK-EKS-HelmChart', {
        Chart: 'argo-events',
        Repository: 'https://argoproj.github.io/argo-helm',
        Namespace: 'argo',
      });
    });

    it('should configure event bus for webhook events', () => {
      // Arrange
      const app = new cdk.App();
      const stack = new cdk.Stack(app, 'TestStack');

      // Act
      new AphexCluster(stack, 'TestCluster');

      // Assert
      const template = Template.fromStack(stack);

      // Verify Helm chart has event bus configuration
      const helmCharts = template.findResources('Custom::AWSCDK-EKS-HelmChart');
      const argoEventsChart = Object.values(helmCharts).find((chart: any) => {
        return chart.Properties?.Chart === 'argo-events';
      });

      expect(argoEventsChart).toBeDefined();
      
      const values = (argoEventsChart as any).Properties?.Values;
      const valuesStr = typeof values === 'string' ? values : JSON.stringify(values);
      
      // Verify event bus configuration
      expect(valuesStr).toContain('eventBus');
    });

    it('should create EventBus resource', () => {
      // Arrange
      const app = new cdk.App();
      const stack = new cdk.Stack(app, 'TestStack');

      // Act
      new AphexCluster(stack, 'TestCluster');

      // Assert
      const template = Template.fromStack(stack);

      // Verify EventBus manifest exists
      const kubernetesResources = template.findResources('Custom::AWSCDK-EKS-KubernetesResource');
      const eventBus = Object.values(kubernetesResources).find((resource: any) => {
        const manifest = resource.Properties?.Manifest;
        if (!manifest) return false;
        const manifestStr = typeof manifest === 'string' ? manifest : JSON.stringify(manifest);
        return manifestStr.includes('"kind":"EventBus"') || manifestStr.includes('"kind": "EventBus"');
      });

      expect(eventBus).toBeDefined();
    });

    it('should create service accounts with appropriate permissions', () => {
      // Arrange
      const app = new cdk.App();
      const stack = new cdk.Stack(app, 'TestStack');

      // Act
      new AphexCluster(stack, 'TestCluster');

      // Assert
      const template = Template.fromStack(stack);

      // Verify Helm chart values include service account configuration
      const helmCharts = template.findResources('Custom::AWSCDK-EKS-HelmChart');
      const argoEventsChart = Object.values(helmCharts).find((chart: any) => {
        return chart.Properties?.Chart === 'argo-events';
      });

      const values = (argoEventsChart as any).Properties?.Values;
      const valuesStr = typeof values === 'string' ? values : JSON.stringify(values);
      
      // Verify service account configuration
      expect(valuesStr).toContain('serviceAccount');
      expect(valuesStr).toContain('rbac');
    });
  });

  describe('fromClusterAttributes static method', () => {
    it('should import existing cluster by attributes', () => {
      // Arrange
      const app = new cdk.App();
      const stack = new cdk.Stack(app, 'TestStack');

      const clusterAttributes = {
        clusterName: 'existing-cluster',
        oidcProviderArn: 'arn:aws:iam::123456789012:oidc-provider/oidc.eks.us-east-1.amazonaws.com/id/EXAMPLE',
        kubectlRoleArn: 'arn:aws:iam::123456789012:role/existing-kubectl-role',
      };

      // Act
      const importedCluster = AphexCluster.fromClusterAttributes(
        stack,
        'ImportedCluster',
        clusterAttributes
      );

      // Assert
      expect(importedCluster).toBeDefined();
      expect(importedCluster.cluster).toBeDefined();
      expect(importedCluster.oidcProvider).toBeDefined();
      expect(importedCluster.kubectlRoleArn).toBe(clusterAttributes.kubectlRoleArn);
      expect(importedCluster.clusterNameExport).toBe('ArbiterCluster-ClusterName');
      expect(importedCluster.oidcProviderArnExport).toBe('ArbiterCluster-OIDCProviderArn');
    });

    it('should allow creating service accounts on imported cluster', () => {
      // Arrange
      const app = new cdk.App();
      const stack = new cdk.Stack(app, 'TestStack');

      const clusterAttributes = {
        clusterName: 'existing-cluster',
        oidcProviderArn: 'arn:aws:iam::123456789012:oidc-provider/oidc.eks.us-east-1.amazonaws.com/id/EXAMPLE',
        kubectlRoleArn: 'arn:aws:iam::123456789012:role/existing-kubectl-role',
      };

      const importedCluster = AphexCluster.fromClusterAttributes(
        stack,
        'ImportedCluster',
        clusterAttributes
      );

      // Act
      const serviceAccount = importedCluster.createServiceAccountWithRole(
        'TestServiceAccount',
        'test-namespace',
        'test-sa',
        [
          new cdk.aws_iam.PolicyStatement({
            effect: cdk.aws_iam.Effect.ALLOW,
            actions: ['s3:GetObject'],
            resources: ['arn:aws:s3:::test-bucket/*'],
          }),
        ]
      );

      // Assert
      expect(serviceAccount).toBeDefined();
      expect(serviceAccount.role).toBeDefined();
    });

    it('should allow creating pipelines on imported cluster', () => {
      // Arrange
      const app = new cdk.App();
      const stack = new cdk.Stack(app, 'TestStack');

      const clusterAttributes = {
        clusterName: 'existing-cluster',
        oidcProviderArn: 'arn:aws:iam::123456789012:oidc-provider/oidc.eks.us-east-1.amazonaws.com/id/EXAMPLE',
        kubectlRoleArn: 'arn:aws:iam::123456789012:role/existing-kubectl-role',
      };

      const importedCluster = AphexCluster.fromClusterAttributes(
        stack,
        'ImportedCluster',
        clusterAttributes
      );

      // Act
      const pipeline = importedCluster.createPipeline({
        pipelineId: 'test-pipeline',
        policyStatements: [
          new cdk.aws_iam.PolicyStatement({
            effect: cdk.aws_iam.Effect.ALLOW,
            actions: ['s3:GetObject'],
            resources: ['arn:aws:s3:::test-bucket/*'],
          }),
        ],
      });

      // Assert
      expect(pipeline).toBeDefined();
      expect(pipeline.pipelineId).toBe('test-pipeline');
      expect(pipeline.namespace).toBe('pipeline-test-pipeline');
      expect(pipeline.serviceAccount).toBeDefined();
      expect(pipeline.roleArn).toBeDefined();
    });

    it('should allow deleting pipelines on imported cluster', () => {
      // Arrange
      const app = new cdk.App();
      const stack = new cdk.Stack(app, 'TestStack');

      const clusterAttributes = {
        clusterName: 'existing-cluster',
        oidcProviderArn: 'arn:aws:iam::123456789012:oidc-provider/oidc.eks.us-east-1.amazonaws.com/id/EXAMPLE',
        kubectlRoleArn: 'arn:aws:iam::123456789012:role/existing-kubectl-role',
      };

      const importedCluster = AphexCluster.fromClusterAttributes(
        stack,
        'ImportedCluster',
        clusterAttributes
      );

      const pipeline = importedCluster.createPipeline({
        pipelineId: 'test-pipeline',
        policyStatements: [
          new cdk.aws_iam.PolicyStatement({
            effect: cdk.aws_iam.Effect.ALLOW,
            actions: ['s3:GetObject'],
            resources: ['arn:aws:s3:::test-bucket/*'],
          }),
        ],
      });

      // Act
      importedCluster.deletePipeline('test-pipeline');

      // Assert
      // Verify pipeline is no longer tracked (would throw if we try to delete again)
      expect(() => importedCluster.deletePipeline('test-pipeline')).toThrow();
    });
  });
});
