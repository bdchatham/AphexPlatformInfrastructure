/**
 * Jest global teardown for integration tests
 * 
 * This runs once after all integration tests and cleans up the kind cluster.
 */

import { execSync } from 'child_process';

const CLUSTER_NAME = process.env.KIND_CLUSTER_NAME || 'arbiter-test';
const CLEANUP_CLUSTER = process.env.CLEANUP_INTEGRATION_CLUSTER !== 'false';

function exec(command: string): void {
  try {
    execSync(command, {
      encoding: 'utf-8',
      stdio: 'inherit',
    });
  } catch (error: any) {
    console.error(`Command failed: ${command}`);
    console.error(error.message);
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

async function teardown(): Promise<void> {
  // Skip if integration tests were skipped
  if (process.env.SKIP_INTEGRATION_TESTS === 'true') {
    return;
  }

  // Skip cleanup if explicitly disabled
  if (!CLEANUP_CLUSTER) {
    console.log(`\n⚠️  Keeping cluster '${CLUSTER_NAME}' (CLEANUP_INTEGRATION_CLUSTER=false)`);
    console.log(`   Clean up manually with: kind delete cluster --name ${CLUSTER_NAME}\n`);
    return;
  }

  console.log('\n🧹 Cleaning up integration test environment...\n');

  if (clusterExists()) {
    console.log(`🗑️  Deleting kind cluster '${CLUSTER_NAME}'...`);
    exec(`kind delete cluster --name ${CLUSTER_NAME}`);
    console.log('✅ Cluster deleted');
  } else {
    console.log(`ℹ️  Cluster '${CLUSTER_NAME}' does not exist, nothing to clean up`);
  }

  console.log('\n✨ Cleanup complete!\n');
}

export default teardown;
