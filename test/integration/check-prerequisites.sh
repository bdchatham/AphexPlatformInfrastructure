#!/bin/bash
# Check if all prerequisites for integration tests are installed

set -e

echo "🔍 Checking integration test prerequisites..."
echo ""

ALL_INSTALLED=true

# Check kind
if command -v kind &> /dev/null; then
    KIND_VERSION=$(kind version | head -n 1)
    echo "✅ kind is installed: $KIND_VERSION"
else
    echo "❌ kind is NOT installed"
    echo "   Install: brew install kind  # macOS"
    echo "   Or visit: https://kind.sigs.k8s.io/docs/user/quick-start/#installation"
    ALL_INSTALLED=false
fi

# Check kubectl
if command -v kubectl &> /dev/null; then
    KUBECTL_VERSION=$(kubectl version --client --short 2>/dev/null || kubectl version --client 2>/dev/null | head -n 1)
    echo "✅ kubectl is installed: $KUBECTL_VERSION"
else
    echo "❌ kubectl is NOT installed"
    echo "   Install: brew install kubectl  # macOS"
    ALL_INSTALLED=false
fi

# Check helm
if command -v helm &> /dev/null; then
    HELM_VERSION=$(helm version --short)
    echo "✅ helm is installed: $HELM_VERSION"
else
    echo "❌ helm is NOT installed"
    echo "   Install: brew install helm  # macOS"
    ALL_INSTALLED=false
fi

# Check Docker
if command -v docker &> /dev/null; then
    if docker ps &> /dev/null; then
        DOCKER_VERSION=$(docker version --format '{{.Server.Version}}' 2>/dev/null)
        echo "✅ Docker is installed and running: $DOCKER_VERSION"
    else
        echo "⚠️  Docker is installed but not running"
        echo "   Please start Docker Desktop"
        ALL_INSTALLED=false
    fi
else
    echo "❌ Docker is NOT installed"
    echo "   Install: https://docs.docker.com/get-docker/"
    ALL_INSTALLED=false
fi

echo ""

if [ "$ALL_INSTALLED" = true ]; then
    echo "✅ All prerequisites are installed!"
    echo ""
    echo "You can now run integration tests:"
    echo "  npm run test:integration:setup    # Setup cluster"
    echo "  npm run test:integration          # Run tests"
    echo "  npm run test:integration:cleanup  # Cleanup"
    echo ""
    echo "Or run everything at once:"
    echo "  npm run test:integration:full"
    exit 0
else
    echo "❌ Some prerequisites are missing. Please install them and try again."
    exit 1
fi
