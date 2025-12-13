/**
 * Integration tests for Argo Workflows and Argo Events
 * 
 * These tests verify end-to-end functionality using a local kind cluster.
 * They test that Argo Workflows can be installed and execute workflows successfully.
 * 
 * Prerequisites:
 * - kind installed (brew install kind)
 * - kubectl installed (brew install kubectl)
 * - helm installed (brew install helm)
 * 
 * Setup:
 * Run `npm run test:integration:setup` before running these tests
 * 
 * Cleanup:
 * Run `npm run test:integration:cleanup` after running tests
 * 
 * Requirements: All
 */

import { execSync } from 'child_process';
import * as fs from 'fs';
import * as path from 'path';

// Increase timeout for integration tests
jest.setTimeout(300000); // 5 minutes

const CLUSTER_NAME = process.env.KIND_CLUSTER_NAME || 'arbiter-test';
const ARGO_NAMESPACE = process.env.ARGO_NAMESPACE || 'argo';

/**
 * Execute a shell command and return the output
 */
function exec(command: string): string {
  try {
    return execSync(command, {
      encoding: 'utf-8',
      stdio: ['pipe', 'pipe', 'pipe'],
    }).trim();
  } catch (error: any) {
    throw new Error(`Command failed: ${command}\n${error.message}\n${error.stdout}\n${error.stderr}`);
  }
}

/**
 * Check if kind cluster exists
 */
function clusterExists(): boolean {
  try {
    const clusters = exec('kind get clusters');
    return clusters.split('\n').includes(CLUSTER_NAME);
  } catch {
    return false;
  }
}

/**
 * Wait for a workflow to complete
 */
function waitForWorkflow(workflowName: string, timeoutSeconds: number = 120): string {
  const startTime = Date.now();
  const timeoutMs = timeoutSeconds * 1000;

  while (Date.now() - startTime < timeoutMs) {
    try {
      const status = exec(
        `kubectl get workflow ${workflowName} -n ${ARGO_NAMESPACE} -o jsonpath='{.status.phase}'`
      );

      if (status === 'Succeeded') {
        return 'Succeeded';
      } else if (status === 'Failed' || status === 'Error') {
        // Get workflow details for debugging
        const details = exec(`kubectl get workflow ${workflowName} -n ${ARGO_NAMESPACE} -o yaml`);
        throw new Error(`Workflow failed with status: ${status}\n${details}`);
      }

      // Wait a bit before checking again
      execSync('sleep 2');
    } catch (error: any) {
      if (error.message.includes('NotFound')) {
        // Workflow not created yet, wait
        execSync('sleep 2');
        continue;
      }
      throw error;
    }
  }

  throw new Error(`Workflow ${workflowName} did not complete within ${timeoutSeconds} seconds`);
}

// Check if we should skip integration tests
const shouldSkip = process.env.SKIP_INTEGRATION_TESTS === 'true';

// Use describe.skip if prerequisites are missing
const describeIntegration = shouldSkip ? describe.skip : describe;

describeIntegration('Argo Workflows Integration Tests', () => {
  beforeAll(() => {

    // Verify cluster is set up
    if (!clusterExists()) {
      throw new Error(
        `kind cluster '${CLUSTER_NAME}' does not exist. ` +
        `This should have been created by jest-global-setup.`
      );
    }

    // Set kubectl context
    exec(`kubectl config use-context kind-${CLUSTER_NAME}`);

    // Verify Argo Workflows is installed
    try {
      exec(`helm list -n ${ARGO_NAMESPACE} | grep argo-workflows`);
    } catch {
      throw new Error(
        `Argo Workflows is not installed. ` +
        `This should have been installed by jest-global-setup.`
      );
    }
  });

  describe('Cluster Setup', () => {
    it('should have kind cluster running', () => {
      const clusters = exec('kind get clusters');
      expect(clusters).toContain(CLUSTER_NAME);
    });

    it('should have argo namespace created', () => {
      const namespaces = exec('kubectl get namespaces -o name');
      expect(namespaces).toContain(`namespace/${ARGO_NAMESPACE}`);
    });

    it('should have Argo Workflows controller running', () => {
      const pods = exec(
        `kubectl get pods -n ${ARGO_NAMESPACE} -l app.kubernetes.io/name=argo-workflows-workflow-controller -o name`
      );
      expect(pods).toContain('pod/');
    });

    it('should have Argo Workflows server running', () => {
      const pods = exec(
        `kubectl get pods -n ${ARGO_NAMESPACE} -l app.kubernetes.io/name=argo-workflows-server -o name`
      );
      expect(pods).toContain('pod/');
    });

    it('should have Argo Events controller running', () => {
      const pods = exec(
        `kubectl get pods -n ${ARGO_NAMESPACE} -l app.kubernetes.io/component=controller-manager -o name`
      );
      expect(pods).toContain('pod/');
    });
  });

  describe('Simple Workflow Execution', () => {
    const workflowName = 'hello-world-test';

    afterEach(() => {
      // Clean up workflow
      try {
        exec(`kubectl delete workflow ${workflowName} -n ${ARGO_NAMESPACE} --ignore-not-found=true`);
      } catch {
        // Ignore cleanup errors
      }
    });

    it('should execute a simple hello world workflow', () => {
      // Create a simple workflow
      const workflow = `
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  name: ${workflowName}
  namespace: ${ARGO_NAMESPACE}
spec:
  entrypoint: hello
  templates:
  - name: hello
    container:
      image: alpine:latest
      command: [echo]
      args: ["Hello, World!"]
`;

      // Write workflow to temp file
      const tempFile = path.join('/tmp', `${workflowName}.yaml`);
      fs.writeFileSync(tempFile, workflow);

      try {
        // Submit workflow
        exec(`kubectl apply -f ${tempFile}`);

        // Wait for workflow to complete
        const status = waitForWorkflow(workflowName, 120);
        expect(status).toBe('Succeeded');

        // Verify workflow output
        const logs = exec(
          `kubectl logs -n ${ARGO_NAMESPACE} -l workflows.argoproj.io/workflow=${workflowName} --tail=-1`
        );
        expect(logs).toContain('Hello, World!');
      } finally {
        // Clean up temp file
        fs.unlinkSync(tempFile);
      }
    });
  });

  describe('Multi-Step Workflow Execution', () => {
    const workflowName = 'multi-step-test';

    afterEach(() => {
      // Clean up workflow
      try {
        exec(`kubectl delete workflow ${workflowName} -n ${ARGO_NAMESPACE} --ignore-not-found=true`);
      } catch {
        // Ignore cleanup errors
      }
    });

    it('should execute a workflow with multiple sequential steps', () => {
      // Create a multi-step workflow
      const workflow = `
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  name: ${workflowName}
  namespace: ${ARGO_NAMESPACE}
spec:
  entrypoint: main
  templates:
  - name: main
    steps:
    - - name: step1
        template: echo-step
        arguments:
          parameters:
          - name: message
            value: "Step 1"
    - - name: step2
        template: echo-step
        arguments:
          parameters:
          - name: message
            value: "Step 2"
    - - name: step3
        template: echo-step
        arguments:
          parameters:
          - name: message
            value: "Step 3"
  - name: echo-step
    inputs:
      parameters:
      - name: message
    container:
      image: alpine:latest
      command: [echo]
      args: ["{{inputs.parameters.message}}"]
`;

      // Write workflow to temp file
      const tempFile = path.join('/tmp', `${workflowName}.yaml`);
      fs.writeFileSync(tempFile, workflow);

      try {
        // Submit workflow
        exec(`kubectl apply -f ${tempFile}`);

        // Wait for workflow to complete
        const status = waitForWorkflow(workflowName, 180);
        expect(status).toBe('Succeeded');

        // Verify all steps completed
        const workflowStatus = exec(
          `kubectl get workflow ${workflowName} -n ${ARGO_NAMESPACE} -o json`
        );
        const workflowObj = JSON.parse(workflowStatus);
        
        // Check that we have nodes for all steps
        const nodes = workflowObj.status.nodes;
        const nodeNames = Object.values(nodes).map((node: any) => node.displayName);
        
        expect(nodeNames).toContain('step1');
        expect(nodeNames).toContain('step2');
        expect(nodeNames).toContain('step3');
      } finally {
        // Clean up temp file
        fs.unlinkSync(tempFile);
      }
    });

    it('should execute a workflow with parallel steps', () => {
      const parallelWorkflowName = 'parallel-test';
      
      // Create a workflow with parallel steps
      const workflow = `
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  name: ${parallelWorkflowName}
  namespace: ${ARGO_NAMESPACE}
spec:
  entrypoint: main
  templates:
  - name: main
    steps:
    - - name: parallel1
        template: echo-step
        arguments:
          parameters:
          - name: message
            value: "Parallel 1"
      - name: parallel2
        template: echo-step
        arguments:
          parameters:
          - name: message
            value: "Parallel 2"
      - name: parallel3
        template: echo-step
        arguments:
          parameters:
          - name: message
            value: "Parallel 3"
  - name: echo-step
    inputs:
      parameters:
      - name: message
    container:
      image: alpine:latest
      command: [echo]
      args: ["{{inputs.parameters.message}}"]
`;

      // Write workflow to temp file
      const tempFile = path.join('/tmp', `${parallelWorkflowName}.yaml`);
      fs.writeFileSync(tempFile, workflow);

      try {
        // Submit workflow
        exec(`kubectl apply -f ${tempFile}`);

        // Wait for workflow to complete
        const status = waitForWorkflow(parallelWorkflowName, 180);
        expect(status).toBe('Succeeded');

        // Verify all parallel steps completed
        const workflowStatus = exec(
          `kubectl get workflow ${parallelWorkflowName} -n ${ARGO_NAMESPACE} -o json`
        );
        const workflowObj = JSON.parse(workflowStatus);
        
        // Check that we have nodes for all parallel steps
        const nodes = workflowObj.status.nodes;
        const nodeNames = Object.values(nodes).map((node: any) => node.displayName);
        
        expect(nodeNames).toContain('parallel1');
        expect(nodeNames).toContain('parallel2');
        expect(nodeNames).toContain('parallel3');
      } finally {
        // Clean up temp file
        fs.unlinkSync(tempFile);
        
        // Clean up workflow
        try {
          exec(`kubectl delete workflow ${parallelWorkflowName} -n ${ARGO_NAMESPACE} --ignore-not-found=true`);
        } catch {
          // Ignore cleanup errors
        }
      }
    });
  });

  describe('Workflow with Artifacts', () => {
    const workflowName = 'artifact-test';

    afterEach(() => {
      // Clean up workflow
      try {
        exec(`kubectl delete workflow ${workflowName} -n ${ARGO_NAMESPACE} --ignore-not-found=true`);
      } catch {
        // Ignore cleanup errors
      }
    });

    // Skip this test as it requires artifact storage (S3, Minio, etc.) which is not configured in kind
    it.skip('should execute a workflow that produces and consumes artifacts', () => {
      // Create a workflow with artifact passing
      const workflow = `
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  name: ${workflowName}
  namespace: ${ARGO_NAMESPACE}
spec:
  entrypoint: main
  templates:
  - name: main
    steps:
    - - name: generate
        template: generate-artifact
    - - name: consume
        template: consume-artifact
        arguments:
          artifacts:
          - name: message
            from: "{{steps.generate.outputs.artifacts.message}}"
  - name: generate-artifact
    container:
      image: alpine:latest
      command: [sh, -c]
      args: ["echo 'Hello from artifact' > /tmp/message.txt"]
    outputs:
      artifacts:
      - name: message
        path: /tmp/message.txt
  - name: consume-artifact
    inputs:
      artifacts:
      - name: message
        path: /tmp/message.txt
    container:
      image: alpine:latest
      command: [cat]
      args: ["/tmp/message.txt"]
`;

      // Write workflow to temp file
      const tempFile = path.join('/tmp', `${workflowName}.yaml`);
      fs.writeFileSync(tempFile, workflow);

      try {
        // Submit workflow
        exec(`kubectl apply -f ${tempFile}`);

        // Wait for workflow to complete
        const status = waitForWorkflow(workflowName, 180);
        expect(status).toBe('Succeeded');

        // Verify workflow completed both steps
        const workflowStatus = exec(
          `kubectl get workflow ${workflowName} -n ${ARGO_NAMESPACE} -o json`
        );
        const workflowObj = JSON.parse(workflowStatus);
        
        const nodes = workflowObj.status.nodes;
        const nodeNames = Object.values(nodes).map((node: any) => node.displayName);
        
        expect(nodeNames).toContain('generate');
        expect(nodeNames).toContain('consume');
      } finally {
        // Clean up temp file
        fs.unlinkSync(tempFile);
      }
    });
  });

  describe('Argo Events Installation', () => {
    it('should have Argo Events installed', () => {
      const helmList = exec(`helm list -n ${ARGO_NAMESPACE}`);
      expect(helmList).toContain('argo-events');
    });

    it('should have EventBus CRD installed', () => {
      const crds = exec('kubectl get crd');
      expect(crds).toContain('eventbus.argoproj.io');
    });

    it('should have EventSource CRD installed', () => {
      const crds = exec('kubectl get crd');
      expect(crds).toContain('eventsources.argoproj.io');
    });

    it('should have Sensor CRD installed', () => {
      const crds = exec('kubectl get crd');
      expect(crds).toContain('sensors.argoproj.io');
    });
  });
});
