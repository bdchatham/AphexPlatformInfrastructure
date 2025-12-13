#!/bin/bash
# Convenience script to run integration tests with proper error handling

set -e

echo "🧪 Arbiter Pipeline Infrastructure - Integration Tests"
echo "======================================================"
echo ""

# Check prerequisites first
echo "Step 1/4: Checking prerequisites..."
if ! bash test/integration/check-prerequisites.sh; then
    echo ""
    echo "❌ Prerequisites check failed. Please install missing tools."
    exit 1
fi

echo ""
echo "Step 2/4: Setting up kind cluster..."
if ! bash test/integration/setup-kind-cluster.sh; then
    echo ""
    echo "❌ Cluster setup failed."
    exit 1
fi

echo ""
echo "Step 3/4: Running integration tests..."
if npm run test:integration; then
    TEST_RESULT=0
    echo ""
    echo "✅ All integration tests passed!"
else
    TEST_RESULT=1
    echo ""
    echo "❌ Some integration tests failed."
fi

echo ""
echo "Step 4/4: Cleaning up..."
bash test/integration/cleanup-kind-cluster.sh

echo ""
if [ $TEST_RESULT -eq 0 ]; then
    echo "✅ Integration test run completed successfully!"
    exit 0
else
    echo "❌ Integration test run completed with failures."
    exit 1
fi
