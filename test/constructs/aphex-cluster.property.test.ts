/**
 * Property-based tests for AphexCluster construct
 * 
 * Feature: arbiter-pipeline-infrastructure
 */

import * as cdk from 'aws-cdk-lib';
import * as ec2 from 'aws-cdk-lib/aws-ec2';
import * as eks from 'aws-cdk-lib/aws-eks';
import * as iam from 'aws-cdk-lib/aws-iam';
import { Template } from 'aws-cdk-lib/assertions';
import * as fc from 'fast-check';
import { AphexCluster } from '../../lib/constructs/aphex-cluster';

describe('AphexCluster Property-Based Tests', () => {
  /**
   * Feature: arbiter-pipeline-infrastructure, Property 11: Autoscaling configuration
   * 
   * For any valid minimum and maximum node count values, the deployed cluster 
   * should have autoscaling configured with those bounds
   * 
   * Validates: Requirements 9.3
   */
  it('should configure autoscaling with specified min and max node counts', () => {
    fc.assert(
      fc.property(
        // Generate valid min/max pairs where min <= max and both are positive
        fc.integer({ min: 1, max: 5 }),
        fc.integer({ min: 1, max: 20 }),
        (minNodes, maxNodesOffset) => {
          // Ensure maxNodes >= minNodes
          const maxNodes = minNodes + maxNodesOffset;
          
          // Create a stack and cluster with the generated values
          const app = new cdk.App();
          const stack = new cdk.Stack(app, 'TestStack');
          
          const cluster = new AphexCluster(stack, 'TestCluster', {
            minNodes,
            maxNodes,
            clusterName: `test-cluster-${minNodes}-${maxNodes}`,
          });

          // Synthesize the CloudFormation template
          const template = Template.fromStack(stack);

          // Verify that a node group exists with the correct autoscaling configuration
          template.hasResourceProperties('AWS::EKS::Nodegroup', {
            ScalingConfig: {
              MinSize: minNodes,
              MaxSize: maxNodes,
              DesiredSize: minNodes,
            },
          });

          // Property: The cluster should have autoscaling configured
          // with the exact min and max values we specified
          expect(cluster.cluster).toBeDefined();
        }
      ),
      { numRuns: 100 } // Run 100 iterations as specified in the design
    );
  });

  /**
   * Feature: arbiter-pipeline-infrastructure, Property 10: CloudFormation export resolution
   * 
   * For any pipeline stack that references the cluster's CloudFormation exports, 
   * the exports should resolve to the correct cluster attribute values
   * 
   * Validates: Requirements 8.4
   */
  it('should export cluster attributes that can be imported by pipeline stacks', () => {
    fc.assert(
      fc.property(
        // Generate random cluster names to ensure exports work with any name
        fc.stringMatching(/^[a-z][a-z0-9-]{3,20}$/),
        fc.integer({ min: 2, max: 10 }),
        (clusterName, minNodes) => {
          // Create the infrastructure stack with the cluster
          const app = new cdk.App();
          const infraStack = new cdk.Stack(app, 'InfraStack', {
            env: { account: '123456789012', region: 'us-east-1' },
          });
          
          const cluster = new AphexCluster(infraStack, 'AphexCluster', {
            clusterName,
            minNodes,
            maxNodes: minNodes + 5,
          });

          // Create a pipeline stack that imports the cluster attributes
          const pipelineStack = new cdk.Stack(app, 'PipelineStack', {
            env: { account: '123456789012', region: 'us-east-1' },
          });

          // Import cluster attributes using CloudFormation exports
          const importedClusterName = cdk.Fn.importValue('ArbiterCluster-ClusterName');
          const importedOidcProviderArn = cdk.Fn.importValue('ArbiterCluster-OIDCProviderArn');
          const importedKubectlRoleArn = cdk.Fn.importValue('ArbiterCluster-KubectlRoleArn');
          const importedSecurityGroupId = cdk.Fn.importValue('ArbiterCluster-ClusterSecurityGroupId');

          // Create outputs in the pipeline stack to verify the imports work
          new cdk.CfnOutput(pipelineStack, 'ImportedClusterName', {
            value: importedClusterName,
          });
          new cdk.CfnOutput(pipelineStack, 'ImportedOidcProviderArn', {
            value: importedOidcProviderArn,
          });
          new cdk.CfnOutput(pipelineStack, 'ImportedKubectlRoleArn', {
            value: importedKubectlRoleArn,
          });
          new cdk.CfnOutput(pipelineStack, 'ImportedSecurityGroupId', {
            value: importedSecurityGroupId,
          });

          // Synthesize both stacks
          const infraTemplate = Template.fromStack(infraStack);
          const pipelineTemplate = Template.fromStack(pipelineStack);

          // Verify that the infrastructure stack exports the correct values
          const infraOutputs = infraTemplate.findOutputs('*');
          
          // Check that all required exports exist
          const exportNames = Object.values(infraOutputs)
            .map((output: any) => output.Export?.Name)
            .filter(Boolean);
          
          expect(exportNames).toContain('ArbiterCluster-ClusterName');
          expect(exportNames).toContain('ArbiterCluster-OIDCProviderArn');
          expect(exportNames).toContain('ArbiterCluster-KubectlRoleArn');
          expect(exportNames).toContain('ArbiterCluster-ClusterSecurityGroupId');

          // Verify that the pipeline stack can reference these exports
          const pipelineOutputs = pipelineTemplate.findOutputs('*');
          
          // Check that the pipeline stack has outputs that use Fn::ImportValue
          const hasImportedValues = Object.values(pipelineOutputs).some((output: any) => {
            const value = output.Value;
            return value && typeof value === 'object' && 'Fn::ImportValue' in value;
          });
          
          expect(hasImportedValues).toBe(true);

          // Property: The exports should be resolvable and contain the correct structure
          expect(cluster.clusterNameExport).toBe('ArbiterCluster-ClusterName');
          expect(cluster.oidcProviderArnExport).toBe('ArbiterCluster-OIDCProviderArn');
        }
      ),
      { numRuns: 100 } // Run 100 iterations as specified in the design
    );
  });

  /**
   * Feature: arbiter-pipeline-infrastructure, Property 16: Pipeline permission isolation
   * 
   * For any set of pipelines sharing the cluster, each pipeline should have 
   * isolated IAM permissions via separate service accounts
   * 
   * Validates: Requirements 11.4
   */
  it('should isolate IAM permissions per pipeline using separate service accounts', () => {
    fc.assert(
      fc.property(
        // Generate 2-5 pipeline IDs that are valid DNS labels (RFC 1123)
        // Must start with letter, contain only lowercase letters, numbers, and hyphens
        // Cannot end with hyphen
        fc.array(
          fc.stringMatching(/^[a-z][a-z0-9]{2,10}[a-z0-9]$/),
          { minLength: 2, maxLength: 5 }
        ).map(ids => Array.from(new Set(ids))), // Ensure unique IDs
        (pipelineIds) => {
          // Skip if we don't have at least 2 unique IDs
          if (pipelineIds.length < 2) {
            return true;
          }

          // Create a stack and cluster
          const app = new cdk.App();
          const stack = new cdk.Stack(app, 'TestStack');
          
          const cluster = new AphexCluster(stack, 'TestCluster', {
            clusterName: 'test-cluster',
          });

          // Create multiple pipelines with different permissions
          const pipelines = pipelineIds.map((pipelineId, index) => {
            // Each pipeline gets different S3 bucket permissions
            const bucketName = `pipeline-${pipelineId}-bucket`;
            
            return cluster.createPipeline({
              pipelineId,
              policyStatements: [
                new cdk.aws_iam.PolicyStatement({
                  effect: cdk.aws_iam.Effect.ALLOW,
                  actions: ['s3:GetObject', 's3:PutObject'],
                  resources: [`arn:aws:s3:::${bucketName}/*`],
                }),
              ],
            });
          });

          // Synthesize the CloudFormation template
          const template = Template.fromStack(stack);

          // Property 1: Each pipeline should have its own service account with unique role ARN
          const serviceAccountRoleArns = new Set(pipelines.map(p => p.roleArn));
          expect(serviceAccountRoleArns.size).toBe(pipelineIds.length);

          // Property 2: Each pipeline should have its own namespace
          const namespaces = new Set(pipelines.map(p => p.namespace));
          expect(namespaces.size).toBe(pipelineIds.length);

          // Property 3: Each pipeline should have the correct pipeline ID
          pipelines.forEach((pipeline, index) => {
            expect(pipeline.pipelineId).toBe(pipelineIds[index]);
          });

          // Property 4: Each pipeline should have labels with the pipeline ID
          pipelines.forEach((pipeline) => {
            expect(pipeline.labels['arbiter.pipeline/id']).toBe(pipeline.pipelineId);
            expect(pipeline.labels['arbiter.pipeline/managed-by']).toBe('aphex-cluster');
          });

          // Property 5: Verify that IAM roles exist with OIDC trust policies
          const roles = template.findResources('AWS::IAM::Role');
          
          // Find all roles that match our pipeline service accounts
          const pipelineRoles = Object.entries(roles).filter(([logicalId, role]: [string, any]) => {
            // The logical ID pattern for service account roles created by CDK
            // typically includes "ServiceAccount" and "Role"
            const hasServiceAccountPattern = logicalId.includes('ServiceAccount') && logicalId.includes('Role');
            
            if (!hasServiceAccountPattern) return false;
            
            // Check if this role belongs to one of our pipelines
            const belongsToPipeline = pipelineIds.some(id => {
              // CDK sanitizes IDs, so we need to check for the pipeline ID in various forms
              const sanitizedId = id.replace(/-/g, '');
              return logicalId.includes(id) || logicalId.includes(sanitizedId);
            });
            
            if (!belongsToPipeline) return false;
            
            // Verify it has OIDC trust policy
            const assumeRolePolicy = role.Properties?.AssumeRolePolicyDocument;
            if (!assumeRolePolicy) return false;
            
            const statements = assumeRolePolicy.Statement || [];
            return statements.some((stmt: any) => {
              // Check for Federated principal (OIDC provider)
              return stmt.Principal?.Federated !== undefined;
            });
          });

          // We should have exactly one role per pipeline
          expect(pipelineRoles.length).toBe(pipelineIds.length);

          // Property 6: Each role should have isolated permissions
          // CDK creates separate AWS::IAM::Policy resources that reference the roles
          const policies = template.findResources('AWS::IAM::Policy');
          
          // Find policies that belong to our pipeline service accounts
          const pipelinePolicies = Object.entries(policies).filter(([logicalId, policy]: [string, any]) => {
            // Check if this policy belongs to one of our pipeline roles
            const roles = policy.Properties?.Roles || [];
            return roles.some((roleRef: any) => {
              // The role reference could be a Ref to one of our pipeline roles
              if (typeof roleRef === 'object' && 'Ref' in roleRef) {
                const refId = roleRef.Ref;
                return pipelineIds.some(id => {
                  const sanitizedId = id.replace(/-/g, '');
                  return refId.includes(id) || refId.includes(sanitizedId);
                });
              }
              return false;
            });
          });

          // We should have at least one policy per pipeline (for the S3 permissions)
          expect(pipelinePolicies.length).toBeGreaterThanOrEqual(pipelineIds.length);

          // Each policy should have different policy documents (isolated permissions)
          const policyDocuments = pipelinePolicies.map(([_, policy]: [string, any]) => {
            return JSON.stringify(policy.Properties?.PolicyDocument);
          });

          const uniquePolicyDocs = new Set(policyDocuments);
          // At least some policies should be different (different S3 buckets)
          expect(uniquePolicyDocs.size).toBeGreaterThan(1);

          return true;
        }
      ),
      { numRuns: 100 } // Run 100 iterations as specified in the design
    );
  });

  /**
   * Feature: arbiter-pipeline-infrastructure, Property 19: Pipeline resource isolation
   * 
   * For any set of pipelines deployed to the cluster, each pipeline's Kubernetes 
   * resources should be isolated via namespaces or labels
   * 
   * Validates: Requirements 13.1
   */
  it('should isolate pipeline resources using namespaces and labels', () => {
    fc.assert(
      fc.property(
        // Generate 2-4 pipeline IDs that are valid DNS labels
        fc.array(
          fc.stringMatching(/^[a-z][a-z0-9]{2,10}[a-z0-9]$/),
          { minLength: 2, maxLength: 4 }
        ).map(ids => Array.from(new Set(ids))), // Ensure unique IDs
        (pipelineIds) => {
          // Skip if we don't have at least 2 unique IDs
          if (pipelineIds.length < 2) {
            return true;
          }

          // Create a stack and cluster
          const app = new cdk.App();
          const stack = new cdk.Stack(app, 'TestStack');
          
          const cluster = new AphexCluster(stack, 'TestCluster', {
            clusterName: 'test-cluster',
          });

          // Create multiple pipelines
          const pipelines = pipelineIds.map((pipelineId) => {
            return cluster.createPipeline({
              pipelineId,
              policyStatements: [
                new iam.PolicyStatement({
                  effect: iam.Effect.ALLOW,
                  actions: ['s3:GetObject'],
                  resources: [`arn:aws:s3:::bucket-${pipelineId}/*`],
                }),
              ],
              labels: {
                'custom-label': `value-${pipelineId}`,
              },
            });
          });

          // Synthesize the CloudFormation template
          const template = Template.fromStack(stack);

          // Property 1: Each pipeline should have its own unique namespace
          const namespaces = pipelines.map(p => p.namespace);
          const uniqueNamespaces = new Set(namespaces);
          expect(uniqueNamespaces.size).toBe(pipelineIds.length);

          // Property 2: Each namespace should follow the pattern "pipeline-{id}"
          pipelines.forEach((pipeline) => {
            expect(pipeline.namespace).toBe(`pipeline-${pipeline.pipelineId}`);
          });

          // Property 3: Each pipeline should have labels with the pipeline ID
          pipelines.forEach((pipeline) => {
            expect(pipeline.labels['arbiter.pipeline/id']).toBe(pipeline.pipelineId);
            expect(pipeline.labels['arbiter.pipeline/managed-by']).toBe('aphex-cluster');
            expect(pipeline.labels['custom-label']).toBe(`value-${pipeline.pipelineId}`);
          });

          // Property 4: Verify that namespace manifests exist with proper labels
          const kubernetesResources = template.findResources('Custom::AWSCDK-EKS-KubernetesResource');
          
          // Find namespace manifests
          const namespaceManifests = Object.entries(kubernetesResources).filter(([_, resource]: [string, any]) => {
            const manifest = resource.Properties?.Manifest;
            if (!manifest) return false;
            
            try {
              const manifestStr = typeof manifest === 'string' ? manifest : JSON.stringify(manifest);
              return manifestStr.includes('"kind":"Namespace"') || manifestStr.includes('"kind": "Namespace"');
            } catch {
              return false;
            }
          });

          // We should have at least as many namespace manifests as pipelines
          // (there might be additional namespaces like 'argo')
          expect(namespaceManifests.length).toBeGreaterThanOrEqual(pipelineIds.length);

          // Property 5: Verify that each namespace manifest has the correct labels
          const pipelineNamespaceManifests = namespaceManifests.filter(([_, resource]: [string, any]) => {
            const manifest = resource.Properties?.Manifest;
            if (!manifest) return false;
            
            try {
              const manifestStr = typeof manifest === 'string' ? manifest : JSON.stringify(manifest);
              // Check if this namespace belongs to one of our pipelines
              return pipelineIds.some(id => manifestStr.includes(`pipeline-${id}`));
            } catch {
              return false;
            }
          });

          // We should have exactly one namespace manifest per pipeline
          expect(pipelineNamespaceManifests.length).toBe(pipelineIds.length);

          // Property 6: Verify that resource quotas exist for each pipeline namespace
          const resourceQuotaManifests = Object.entries(kubernetesResources).filter(([_, resource]: [string, any]) => {
            const manifest = resource.Properties?.Manifest;
            if (!manifest) return false;
            
            try {
              const manifestStr = typeof manifest === 'string' ? manifest : JSON.stringify(manifest);
              return manifestStr.includes('"kind":"ResourceQuota"') || manifestStr.includes('"kind": "ResourceQuota"');
            } catch {
              return false;
            }
          });

          // We should have at least one resource quota per pipeline
          expect(resourceQuotaManifests.length).toBeGreaterThanOrEqual(pipelineIds.length);

          // Property 7: Verify that network policies exist for each pipeline namespace
          const networkPolicyManifests = Object.entries(kubernetesResources).filter(([_, resource]: [string, any]) => {
            const manifest = resource.Properties?.Manifest;
            if (!manifest) return false;
            
            try {
              const manifestStr = typeof manifest === 'string' ? manifest : JSON.stringify(manifest);
              return manifestStr.includes('"kind":"NetworkPolicy"') || manifestStr.includes('"kind": "NetworkPolicy"');
            } catch {
              return false;
            }
          });

          // We should have at least one network policy per pipeline
          expect(networkPolicyManifests.length).toBeGreaterThanOrEqual(pipelineIds.length);

          return true;
        }
      ),
      { numRuns: 100 } // Run 100 iterations as specified in the design
    );
  });

  /**
   * Feature: arbiter-pipeline-infrastructure, Property 20: Pipeline deletion isolation
   * 
   * For any pipeline deletion, other pipelines running on the cluster should 
   * remain unaffected and continue operating normally
   * 
   * Validates: Requirements 13.3
   */
  it('should preserve other pipelines when deleting a pipeline', () => {
    fc.assert(
      fc.property(
        // Generate 3-5 pipeline IDs so we can delete one and verify others remain
        fc.array(
          fc.stringMatching(/^[a-z][a-z0-9]{2,10}[a-z0-9]$/),
          { minLength: 3, maxLength: 5 }
        ).map(ids => Array.from(new Set(ids))), // Ensure unique IDs
        fc.integer({ min: 0, max: 10 }), // Index of pipeline to delete (will be modulo'd)
        (pipelineIds, deleteIndex) => {
          // Skip if we don't have at least 3 unique IDs
          if (pipelineIds.length < 3) {
            return true;
          }

          // Create a stack and cluster
          const app = new cdk.App();
          const stack = new cdk.Stack(app, 'TestStack');
          
          const cluster = new AphexCluster(stack, 'TestCluster', {
            clusterName: 'test-cluster',
          });

          // Create multiple pipelines
          const pipelines = pipelineIds.map((pipelineId) => {
            return cluster.createPipeline({
              pipelineId,
              policyStatements: [
                new iam.PolicyStatement({
                  effect: iam.Effect.ALLOW,
                  actions: ['s3:GetObject'],
                  resources: [`arn:aws:s3:::bucket-${pipelineId}/*`],
                }),
              ],
            });
          });

          // Select a pipeline to delete (use modulo to ensure valid index)
          const pipelineToDeleteIndex = deleteIndex % pipelineIds.length;
          const pipelineToDelete = pipelineIds[pipelineToDeleteIndex];

          // Get the list of pipelines before deletion
          const pipelinesBeforeDeletion = cluster.listPipelines();
          expect(pipelinesBeforeDeletion).toContain(pipelineToDelete);
          expect(pipelinesBeforeDeletion.length).toBe(pipelineIds.length);

          // Delete the selected pipeline
          cluster.deletePipeline(pipelineToDelete);

          // Property 1: The deleted pipeline should no longer be in the list
          const pipelinesAfterDeletion = cluster.listPipelines();
          expect(pipelinesAfterDeletion).not.toContain(pipelineToDelete);

          // Property 2: All other pipelines should still be in the list
          const remainingPipelineIds = pipelineIds.filter(id => id !== pipelineToDelete);
          remainingPipelineIds.forEach(id => {
            expect(pipelinesAfterDeletion).toContain(id);
          });

          // Property 3: The number of pipelines should be reduced by exactly 1
          expect(pipelinesAfterDeletion.length).toBe(pipelineIds.length - 1);

          // Property 4: Attempting to get the deleted pipeline should return undefined
          const deletedPipeline = cluster.getPipeline(pipelineToDelete);
          expect(deletedPipeline).toBeUndefined();

          // Property 5: Other pipelines should still be accessible with all their properties
          remainingPipelineIds.forEach((id, index) => {
            const pipeline = cluster.getPipeline(id);
            expect(pipeline).toBeDefined();
            expect(pipeline?.pipelineId).toBe(id);
            expect(pipeline?.namespace).toBe(`pipeline-${id}`);
            expect(pipeline?.labels['arbiter.pipeline/id']).toBe(id);
          });

          // Property 6: Attempting to delete the same pipeline again should throw an error
          expect(() => cluster.deletePipeline(pipelineToDelete)).toThrow();

          // Property 7: We can still create new pipelines after deletion
          const newPipelineId = 'newpipe123';
          const newPipeline = cluster.createPipeline({
            pipelineId: newPipelineId,
            policyStatements: [
              new iam.PolicyStatement({
                effect: iam.Effect.ALLOW,
                actions: ['s3:GetObject'],
                resources: [`arn:aws:s3:::bucket-${newPipelineId}/*`],
              }),
            ],
          });

          expect(newPipeline.pipelineId).toBe(newPipelineId);
          expect(cluster.listPipelines()).toContain(newPipelineId);
          expect(cluster.listPipelines().length).toBe(pipelineIds.length); // Back to original count

          return true;
        }
      ),
      { numRuns: 100 } // Run 100 iterations as specified in the design
    );
  });
});
