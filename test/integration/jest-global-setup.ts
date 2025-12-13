/**
 * Jest global setup for integration tests
 * 
 * This runs once before all integration tests and sets up the kind cluster
 * with Argo Workflows and Argo Events.
 */

import { execSync } from 'child_process';
import * as fs from 'fs';
import * as path from 'path';

const CLUSTER_NAME = process.env.KIND_CLUSTER_NAME || 'arbiter-test';
const ARGO_NAMESPACE = process.env.ARGO_NAMESPACE || 'argo';

function exec(command: string, options: { silent?: boolean } = {}): string {
  try {
    const result = execSync(command, {
      encoding: 'utf-8',
      stdio: options.silent ? 'pipe' : 'inherit',
    });
    return typeof result === 'string' ? result.trim() : '';
  } catch (error: any) {
    if (!options.silent) {
      console.error(`Command failed: ${command}`);
      console.error(error.message);
    }
    throw error;
  }
}

function clusterExists(): boolean {
  try {
    const clusters = execSync('kind get clusters', { encoding: 'utf-8', stdio: 'pipe' }).trim();
    return clusters.split('\n').includes(CLUSTER_NAME);
  } catch {
    return false;
  }
}

function checkPrerequisites(): void {
  const tools = ['kind', 'kubectl', 'helm', 'docker'];
  const missing: string[] = [];

  for (const tool of tools) {
    try {
      execSync(`which ${tool}`, { stdio: 'pipe' });
    } catch {
      missing.push(tool);
    }
  }

  if (missing.length > 0) {
    console.error('\n❌ Missing required tools for integration tests:');
    missing.forEach(tool => console.error(`   - ${tool}`));
    console.error('\nInstall them with: make install-integration-tools');
    console.error('Or skip integration tests with: npm test -- --testPathIgnorePatterns=integration\n');
    throw new Error(`Missing tools: ${missing.join(', ')}`);
  }

  // Check if Docker is running
  try {
    execSync('docker ps', { stdio: 'pipe' });
  } catch {
    console.error('\n❌ Docker is not running. Please start Docker and try again.\n');
    throw new Error('Docker is not running');
  }
}

async function setup(): Promise<void> {
  console.log('\n🚀 Setting up integration test environment...\n');

  // Check prerequisites
  try {
    checkPrerequisites();
  } catch (error) {
    console.log('⚠️  Skipping integration test setup due to missing prerequisites');
    // Set an environment variable to skip integration tests
    process.env.SKIP_INTEGRATION_TESTS = 'true';
    return;
  }

  // Check if cluster already exists
  if (clusterExists()) {
    console.log(`✅ kind cluster '${CLUSTER_NAME}' already exists`);
    
    // Verify Argo is installed
    try {
      exec(`helm list -n ${ARGO_NAMESPACE}`, { silent: true });
      console.log('✅ Argo Workflows already installed');
      console.log('\n✨ Integration test environment ready!\n');
      return;
    } catch {
      console.log('⚠️  Cluster exists but Argo not installed, reinstalling...');
      exec(`kind delete cluster --name ${CLUSTER_NAME}`);
    }
  }

  console.log(`📦 Creating kind cluster '${CLUSTER_NAME}'...`);
  
  // Run the setup script
  const setupScript = path.join(__dirname, 'setup-kind-cluster.sh');
  
  try {
    exec(`bash ${setupScript}`);
    console.log('\n✨ Integration test environment ready!\n');
  } catch (error) {
    console.error('\n❌ Failed to set up integration test environment');
    throw error;
  }
}

export default setup;
