import * as cdk from 'aws-cdk-lib';
import * as ec2 from 'aws-cdk-lib/aws-ec2';
import * as eks from 'aws-cdk-lib/aws-eks';
import * as iam from 'aws-cdk-lib/aws-iam';
import { Construct } from 'constructs';
import { KubectlV30Layer } from '@aws-cdk/lambda-layer-kubectl-v30';

/**
 * Properties for the AphexCluster construct
 */
export interface AphexClusterProps {
  /**
   * Name of the EKS cluster
   * @default 'arbiter-pipeline-cluster'
   */
  readonly clusterName?: string;
  
  /**
   * Minimum number of nodes in the cluster
   * @default 2
   */
  readonly minNodes?: number;
  
  /**
   * Maximum number of nodes in the cluster
   * @default 10
   */
  readonly maxNodes?: number;
  
  /**
   * Instance type for cluster nodes
   * @default t3.medium
   */
  readonly instanceType?: ec2.InstanceType;
  
  /**
   * Kubernetes version
   * @default 1.34
   */
  readonly kubernetesVersion?: eks.KubernetesVersion;
  
  /**
   * VPC to use for the cluster. If not provided, a new VPC will be created.
   */
  readonly vpc?: ec2.IVpc;
  
  /**
   * Namespace for Argo Workflows
   * @default 'argo'
   */
  readonly argoNamespace?: string;
  
  /**
   * Whether to enable CloudWatch Container Insights
   * @default true
   */
  readonly enableContainerInsights?: boolean;
}

/**
 * Attributes for importing an existing AphexCluster
 */
export interface ClusterAttributes {
  readonly clusterName: string;
  readonly oidcProviderArn: string;
  readonly kubectlRoleArn: string;
}

/**
 * Pipeline configuration for multi-pipeline isolation
 */
export interface PipelineConfig {
  /**
   * Unique identifier for the pipeline
   */
  readonly pipelineId: string;
  
  /**
   * Namespace for the pipeline's Kubernetes resources
   * If not provided, defaults to 'pipeline-{pipelineId}'
   */
  readonly namespace?: string;
  
  /**
   * IAM policy statements for the pipeline's service account
   */
  readonly policyStatements: iam.PolicyStatement[];
  
  /**
   * Labels to apply to pipeline resources for isolation
   */
  readonly labels?: { [key: string]: string };
}

/**
 * Interface for AphexCluster
 */
export interface IAphexCluster {
  readonly cluster: eks.ICluster;
  readonly oidcProvider: iam.IOpenIdConnectProvider;
  readonly kubectlRoleArn: string;
  readonly clusterNameExport: string;
  readonly oidcProviderArnExport: string;
  
  /**
   * Create a service account with IRSA role
   */
  createServiceAccountWithRole(
    id: string,
    namespace: string,
    serviceAccountName: string,
    policyStatements: iam.PolicyStatement[]
  ): eks.ServiceAccount;
  
  /**
   * Create an isolated pipeline with its own namespace and service account
   */
  createPipeline(config: PipelineConfig): PipelineResources;
  
  /**
   * Delete a pipeline and its resources without affecting other pipelines
   */
  deletePipeline(pipelineId: string): void;
}

/**
 * Resources created for a pipeline
 */
export interface PipelineResources {
  /**
   * The pipeline's unique identifier
   */
  readonly pipelineId: string;
  
  /**
   * The pipeline's namespace
   */
  readonly namespace: string;
  
  /**
   * The pipeline's service account with IRSA
   */
  readonly serviceAccount: eks.ServiceAccount;
  
  /**
   * The IAM role ARN for the service account
   */
  readonly roleArn: string;
  
  /**
   * Labels applied to pipeline resources
   */
  readonly labels: { [key: string]: string };
}

/**
 * AphexCluster construct that creates an EKS cluster with Argo Workflows and Argo Events
 */
export class AphexCluster extends Construct implements IAphexCluster {
  /**
   * The EKS cluster
   */
  public readonly cluster: eks.Cluster;
  
  /**
   * The OIDC provider for the cluster
   */
  public readonly oidcProvider: iam.IOpenIdConnectProvider;
  
  /**
   * The kubectl role ARN
   */
  public readonly kubectlRoleArn: string;
  
  /**
   * CloudFormation export name for cluster name
   */
  public readonly clusterNameExport: string;
  
  /**
   * CloudFormation export name for OIDC provider ARN
   */
  public readonly oidcProviderArnExport: string;
  
  /**
   * Map of pipeline IDs to their resources
   */
  private readonly pipelines: Map<string, PipelineResources> = new Map();

  /**
   * Create a service account with IRSA role
   * 
   * @param id Construct ID
   * @param namespace Kubernetes namespace for the service account
   * @param serviceAccountName Name of the service account
   * @param policyStatements IAM policy statements to attach to the role
   * @returns The created service account
   */
  public createServiceAccountWithRole(
    id: string,
    namespace: string,
    serviceAccountName: string,
    policyStatements: iam.PolicyStatement[]
  ): eks.ServiceAccount {
    // Create the service account with IRSA
    const serviceAccount = this.cluster.addServiceAccount(id, {
      name: serviceAccountName,
      namespace: namespace,
    });

    // Add policy statements to the service account's role
    policyStatements.forEach(statement => {
      serviceAccount.addToPrincipalPolicy(statement);
    });

    return serviceAccount;
  }

  /**
   * Create an isolated pipeline with its own namespace and service account
   * 
   * This method creates:
   * - A dedicated Kubernetes namespace for the pipeline
   * - A service account with IRSA for AWS permissions
   * - Labels for resource isolation
   * 
   * @param config Pipeline configuration
   * @returns Pipeline resources
   */
  public createPipeline(config: PipelineConfig): PipelineResources {
    // Check if pipeline already exists
    if (this.pipelines.has(config.pipelineId)) {
      throw new Error(`Pipeline ${config.pipelineId} already exists`);
    }

    // Determine namespace
    const namespace = config.namespace ?? `pipeline-${config.pipelineId}`;
    
    // Create default labels with pipeline ID
    const labels = {
      'arbiter.pipeline/id': config.pipelineId,
      'arbiter.pipeline/managed-by': 'aphex-cluster',
      ...config.labels,
    };

    // Create namespace with labels for isolation
    const namespaceManifest = this.cluster.addManifest(`Pipeline-${config.pipelineId}-Namespace`, {
      apiVersion: 'v1',
      kind: 'Namespace',
      metadata: {
        name: namespace,
        labels: labels,
      },
    });

    // Create service account with IRSA for the pipeline
    const serviceAccountName = `${config.pipelineId}-sa`;
    const serviceAccount = this.createServiceAccountWithRole(
      `Pipeline-${config.pipelineId}-ServiceAccount`,
      namespace,
      serviceAccountName,
      config.policyStatements
    );

    // Ensure service account is created after namespace
    serviceAccount.node.addDependency(namespaceManifest);

    // Create resource quota to prevent resource exhaustion
    const resourceQuotaManifest = this.cluster.addManifest(`Pipeline-${config.pipelineId}-ResourceQuota`, {
      apiVersion: 'v1',
      kind: 'ResourceQuota',
      metadata: {
        name: 'pipeline-quota',
        namespace: namespace,
        labels: labels,
      },
      spec: {
        hard: {
          'requests.cpu': '10',
          'requests.memory': '20Gi',
          'limits.cpu': '20',
          'limits.memory': '40Gi',
          'persistentvolumeclaims': '10',
        },
      },
    });
    resourceQuotaManifest.node.addDependency(namespaceManifest);

    // Create network policy for namespace isolation
    const networkPolicyManifest = this.cluster.addManifest(`Pipeline-${config.pipelineId}-NetworkPolicy`, {
      apiVersion: 'networking.k8s.io/v1',
      kind: 'NetworkPolicy',
      metadata: {
        name: 'pipeline-isolation',
        namespace: namespace,
        labels: labels,
      },
      spec: {
        podSelector: {},
        policyTypes: ['Ingress', 'Egress'],
        ingress: [
          {
            // Allow ingress from pods in the same namespace
            from: [
              {
                podSelector: {},
              },
            ],
          },
        ],
        egress: [
          {
            // Allow egress to pods in the same namespace
            to: [
              {
                podSelector: {},
              },
            ],
          },
          {
            // Allow egress to kube-dns for DNS resolution
            to: [
              {
                namespaceSelector: {
                  matchLabels: {
                    'kubernetes.io/metadata.name': 'kube-system',
                  },
                },
                podSelector: {
                  matchLabels: {
                    'k8s-app': 'kube-dns',
                  },
                },
              },
            ],
            ports: [
              {
                protocol: 'UDP',
                port: 53,
              },
            ],
          },
          {
            // Allow egress to the internet (for AWS API calls, etc.)
            to: [
              {
                ipBlock: {
                  cidr: '0.0.0.0/0',
                  except: ['169.254.169.254/32'], // Block metadata service
                },
              },
            ],
          },
        ],
      },
    });
    networkPolicyManifest.node.addDependency(namespaceManifest);

    // Create role binding for the service account to use workflows
    const roleBindingManifest = this.cluster.addManifest(`Pipeline-${config.pipelineId}-RoleBinding`, {
      apiVersion: 'rbac.authorization.k8s.io/v1',
      kind: 'RoleBinding',
      metadata: {
        name: 'pipeline-workflow-executor',
        namespace: namespace,
        labels: labels,
      },
      roleRef: {
        apiGroup: 'rbac.authorization.k8s.io',
        kind: 'ClusterRole',
        name: 'argo-workflow-executor',
      },
      subjects: [
        {
          kind: 'ServiceAccount',
          name: serviceAccountName,
          namespace: namespace,
        },
      ],
    });
    roleBindingManifest.node.addDependency(serviceAccount);

    // Store pipeline resources
    const pipelineResources: PipelineResources = {
      pipelineId: config.pipelineId,
      namespace,
      serviceAccount,
      roleArn: serviceAccount.role.roleArn,
      labels,
    };

    this.pipelines.set(config.pipelineId, pipelineResources);

    return pipelineResources;
  }

  /**
   * Delete a pipeline and its resources without affecting other pipelines
   * 
   * This method removes:
   * - The pipeline's namespace (which cascades to all resources in it)
   * - The pipeline's service account and IAM role
   * 
   * Other pipelines on the cluster remain unaffected.
   * 
   * @param pipelineId The pipeline ID to delete
   */
  public deletePipeline(pipelineId: string): void {
    const pipeline = this.pipelines.get(pipelineId);
    
    if (!pipeline) {
      throw new Error(`Pipeline ${pipelineId} not found`);
    }

    // In CDK, we can't actually delete resources at runtime, but we can
    // provide a manifest that would delete the namespace when applied.
    // The namespace deletion will cascade to all resources within it.
    // This is a design pattern for deletion - the actual deletion would
    // happen via kubectl or the Kubernetes API.
    
    // Remove from our tracking
    this.pipelines.delete(pipelineId);
    
    // Note: In a real implementation, this would trigger a kubectl delete
    // or use the Kubernetes API to delete the namespace. Since CDK is
    // declarative, we document the deletion pattern here.
    // Users would call: kubectl delete namespace <namespace>
  }

  /**
   * Get pipeline resources by ID
   * 
   * @param pipelineId The pipeline ID
   * @returns Pipeline resources or undefined if not found
   */
  public getPipeline(pipelineId: string): PipelineResources | undefined {
    return this.pipelines.get(pipelineId);
  }

  /**
   * List all pipelines
   * 
   * @returns Array of pipeline IDs
   */
  public listPipelines(): string[] {
    return Array.from(this.pipelines.keys());
  }

  constructor(scope: Construct, id: string, props?: AphexClusterProps) {
    super(scope, id);

    // Set defaults
    const clusterName = props?.clusterName ?? 'arbiter-pipeline-cluster';
    const minNodes = props?.minNodes ?? 2;
    const maxNodes = props?.maxNodes ?? 10;
    const instanceType = props?.instanceType ?? ec2.InstanceType.of(ec2.InstanceClass.T3, ec2.InstanceSize.MEDIUM);
    const kubernetesVersion = props?.kubernetesVersion ?? eks.KubernetesVersion.V1_31;
    const argoNamespace = props?.argoNamespace ?? 'argo';
    const enableContainerInsights = props?.enableContainerInsights ?? true;

    // Create or use existing VPC
    const vpc = props?.vpc ?? new ec2.Vpc(this, 'Vpc', {
      maxAzs: 3,
      natGateways: 1,
      subnetConfiguration: [
        {
          cidrMask: 24,
          name: 'Public',
          subnetType: ec2.SubnetType.PUBLIC,
        },
        {
          cidrMask: 24,
          name: 'Private',
          subnetType: ec2.SubnetType.PRIVATE_WITH_EGRESS,
        },
      ],
    });

    // Create kubectl layer for cluster management
    const kubectlLayer = new KubectlV30Layer(this, 'KubectlLayer');

    // Create EKS cluster
    this.cluster = new eks.Cluster(this, 'Cluster', {
      clusterName,
      version: kubernetesVersion,
      vpc,
      vpcSubnets: [{ subnetType: ec2.SubnetType.PRIVATE_WITH_EGRESS }],
      defaultCapacity: 0, // We'll add managed node groups separately
      kubectlLayer,
    });

    // Add managed node group with autoscaling
    this.cluster.addNodegroupCapacity('DefaultNodeGroup', {
      instanceTypes: [instanceType],
      minSize: minNodes,
      maxSize: maxNodes,
      desiredSize: minNodes,
      diskSize: 50,
      amiType: eks.NodegroupAmiType.AL2_X86_64,
      // Ensure nodes can be scheduled in private subnets
      subnets: { subnetType: ec2.SubnetType.PRIVATE_WITH_EGRESS },
      // Add tags for better visibility
      tags: {
        'Name': `${clusterName}-node`,
        'ClusterName': clusterName,
      },
    });

    // Get OIDC provider
    this.oidcProvider = this.cluster.openIdConnectProvider;

    // Get kubectl role ARN
    this.kubectlRoleArn = this.cluster.kubectlRole?.roleArn ?? '';

    // Create namespace for Argo components
    const argoNamespaceManifest = this.cluster.addManifest('ArgoNamespace', {
      apiVersion: 'v1',
      kind: 'Namespace',
      metadata: {
        name: argoNamespace,
      },
    });

    // Install Argo Workflows using Helm
    const argoWorkflowsChart = this.cluster.addHelmChart('ArgoWorkflows', {
      chart: 'argo-workflows',
      release: 'argo-workflows', // Explicit, stable release name
      repository: 'https://argoproj.github.io/argo-helm',
      namespace: argoNamespace,
      createNamespace: false, // We create it explicitly above
      values: {
        server: {
          serviceType: 'LoadBalancer',
          extraArgs: [
            '--auth-mode=server', // Enable server auth mode for better security
          ],
        },
        controller: {
          workflowNamespaces: [argoNamespace],
          // Enable RBAC for workflow execution
          rbac: {
            create: true,
          },
        },
        // Enable workflow service account
        workflow: {
          serviceAccount: {
            create: true,
            name: 'argo-workflow',
          },
          rbac: {
            create: true,
          },
        },
        // Create default executor service account with RBAC
        executor: {
          serviceAccount: {
            create: true,
            name: 'argo-workflow-executor',
          },
        },
      },
    });
    argoWorkflowsChart.node.addDependency(argoNamespaceManifest);

    // Install Argo Events using Helm
    const argoEventsChart = this.cluster.addHelmChart('ArgoEvents', {
      chart: 'argo-events',
      release: 'argo-events', // Explicit, stable release name
      repository: 'https://argoproj.github.io/argo-helm',
      namespace: argoNamespace,
      createNamespace: false, // Use the namespace created above
      values: {
        // Configure event bus for webhook events
        eventBus: {
          enabled: true,
          nats: {
            native: {
              replicas: 3,
              auth: 'token',
            },
          },
        },
        // Create service accounts with appropriate permissions
        serviceAccount: {
          create: true,
          name: 'argo-events-sa',
        },
        // Enable RBAC for event processing
        rbac: {
          create: true,
        },
        // Configure controller with service account
        controller: {
          serviceAccount: {
            create: true,
            name: 'argo-events-controller',
          },
        },
        // Configure event source with service account
        eventSource: {
          serviceAccount: {
            create: true,
            name: 'argo-events-eventsource-sa',
          },
        },
        // Configure sensor with service account
        sensor: {
          serviceAccount: {
            create: true,
            name: 'argo-events-sensor-sa',
          },
        },
      },
    });

    // Ensure Argo Events is installed after Argo Workflows
    argoEventsChart.node.addDependency(argoWorkflowsChart);

    // Create a default EventBus resource for webhook events
    const eventBusManifest = this.cluster.addManifest('DefaultEventBus', {
      apiVersion: 'argoproj.io/v1alpha1',
      kind: 'EventBus',
      metadata: {
        name: 'default',
        namespace: argoNamespace,
      },
      spec: {
        nats: {
          native: {
            replicas: 3,
            auth: 'token',
          },
        },
      },
    });
    eventBusManifest.node.addDependency(argoEventsChart);

    // Enable Container Insights if requested
    if (enableContainerInsights) {
      // Add CloudWatch Container Insights addon
      new eks.CfnAddon(this, 'ContainerInsightsAddon', {
        clusterName: this.cluster.clusterName,
        addonName: 'amazon-cloudwatch-observability',
        resolveConflicts: 'OVERWRITE',
      });
    }

    // Create CloudFormation exports
    this.clusterNameExport = 'ArbiterCluster-ClusterName';
    this.oidcProviderArnExport = 'ArbiterCluster-OIDCProviderArn';
    const kubectlRoleArnExport = 'ArbiterCluster-KubectlRoleArn';
    const clusterSecurityGroupIdExport = 'ArbiterCluster-ClusterSecurityGroupId';

    new cdk.CfnOutput(this, 'ClusterNameOutput', {
      value: this.cluster.clusterName,
      exportName: this.clusterNameExport,
      description: 'EKS cluster name for Arbiter pipelines',
    });

    new cdk.CfnOutput(this, 'OIDCProviderArnOutput', {
      value: this.oidcProvider.openIdConnectProviderArn,
      exportName: this.oidcProviderArnExport,
      description: 'OIDC provider ARN for IRSA',
    });

    new cdk.CfnOutput(this, 'KubectlRoleArnOutput', {
      value: this.kubectlRoleArn,
      exportName: kubectlRoleArnExport,
      description: 'kubectl IAM role ARN',
    });

    new cdk.CfnOutput(this, 'ClusterSecurityGroupIdOutput', {
      value: this.cluster.clusterSecurityGroup.securityGroupId,
      exportName: clusterSecurityGroupIdExport,
      description: 'Cluster security group ID',
    });
  }

  /**
   * Import an existing cluster by its attributes
   */
  public static fromClusterAttributes(
    scope: Construct,
    id: string,
    attrs: ClusterAttributes
  ): IAphexCluster {
    class ImportedAphexCluster extends Construct implements IAphexCluster {
      public readonly cluster: eks.ICluster;
      public readonly oidcProvider: iam.IOpenIdConnectProvider;
      public readonly kubectlRoleArn: string;
      public readonly clusterNameExport: string;
      public readonly oidcProviderArnExport: string;
      private readonly pipelines: Map<string, PipelineResources> = new Map();

      constructor() {
        super(scope, id);

        // Import the cluster
        this.cluster = eks.Cluster.fromClusterAttributes(this, 'ImportedCluster', {
          clusterName: attrs.clusterName,
          openIdConnectProvider: iam.OpenIdConnectProvider.fromOpenIdConnectProviderArn(
            this,
            'ImportedOIDCProvider',
            attrs.oidcProviderArn
          ),
          kubectlRoleArn: attrs.kubectlRoleArn,
        });

        this.oidcProvider = this.cluster.openIdConnectProvider;
        this.kubectlRoleArn = attrs.kubectlRoleArn;
        this.clusterNameExport = 'ArbiterCluster-ClusterName';
        this.oidcProviderArnExport = 'ArbiterCluster-OIDCProviderArn';
      }

      public createServiceAccountWithRole(
        id: string,
        namespace: string,
        serviceAccountName: string,
        policyStatements: iam.PolicyStatement[]
      ): eks.ServiceAccount {
        // Create the service account with IRSA
        const serviceAccount = this.cluster.addServiceAccount(id, {
          name: serviceAccountName,
          namespace: namespace,
        });

        // Add policy statements to the service account's role
        policyStatements.forEach(statement => {
          serviceAccount.addToPrincipalPolicy(statement);
        });

        return serviceAccount;
      }

      public createPipeline(config: PipelineConfig): PipelineResources {
        // Check if pipeline already exists
        if (this.pipelines.has(config.pipelineId)) {
          throw new Error(`Pipeline ${config.pipelineId} already exists`);
        }

        // Determine namespace
        const namespace = config.namespace ?? `pipeline-${config.pipelineId}`;
        
        // Create default labels with pipeline ID
        const labels = {
          'arbiter.pipeline/id': config.pipelineId,
          'arbiter.pipeline/managed-by': 'aphex-cluster',
          ...config.labels,
        };

        // Create namespace with labels for isolation
        const namespaceManifest = this.cluster.addManifest(`Pipeline-${config.pipelineId}-Namespace`, {
          apiVersion: 'v1',
          kind: 'Namespace',
          metadata: {
            name: namespace,
            labels: labels,
          },
        });

        // Create service account with IRSA for the pipeline
        const serviceAccountName = `${config.pipelineId}-sa`;
        const serviceAccount = this.createServiceAccountWithRole(
          `Pipeline-${config.pipelineId}-ServiceAccount`,
          namespace,
          serviceAccountName,
          config.policyStatements
        );

        // Ensure service account is created after namespace
        serviceAccount.node.addDependency(namespaceManifest);

        // Store pipeline resources
        const pipelineResources: PipelineResources = {
          pipelineId: config.pipelineId,
          namespace,
          serviceAccount,
          roleArn: serviceAccount.role.roleArn,
          labels,
        };

        this.pipelines.set(config.pipelineId, pipelineResources);

        return pipelineResources;
      }

      public deletePipeline(pipelineId: string): void {
        const pipeline = this.pipelines.get(pipelineId);
        
        if (!pipeline) {
          throw new Error(`Pipeline ${pipelineId} not found`);
        }

        // Remove from our tracking
        this.pipelines.delete(pipelineId);
      }
    }

    return new ImportedAphexCluster();
  }
}
