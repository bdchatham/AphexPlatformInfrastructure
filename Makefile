.PHONY: help install install-deps install-integration-tools test test-unit test-integration test-integration-setup test-integration-cleanup test-integration-full build clean

# Default target
.DEFAULT_GOAL := help

# Detect OS
UNAME_S := $(shell uname -s)

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-30s\033[0m %s\n", $$1, $$2}'

install: install-deps ## Install all dependencies (Node.js + Python)
	@echo "✅ All dependencies installed"

install-deps: ## Install Node.js and Python dependencies
	@echo "📦 Installing Node.js dependencies..."
	npm install
	@echo "📦 Installing Python dependencies..."
	@if [ ! -d "venv" ]; then \
		python3 -m venv venv; \
	fi
	@. venv/bin/activate && pip install -r requirements-test.txt
	@echo "✅ Dependencies installed"

install-integration-tools: ## Install integration test prerequisites (kind, kubectl, helm)
	@echo "🔧 Installing integration test tools..."
ifeq ($(UNAME_S),Darwin)
	@echo "Detected macOS - using Homebrew"
	@if ! command -v brew >/dev/null 2>&1; then \
		echo "❌ Homebrew not found. Please install from https://brew.sh"; \
		exit 1; \
	fi
	@echo "Installing kind..."
	@brew list kind >/dev/null 2>&1 || brew install kind
	@echo "Installing kubectl..."
	@brew list kubectl >/dev/null 2>&1 || brew install kubectl
	@echo "Installing helm..."
	@brew list helm >/dev/null 2>&1 || brew install helm
	@echo "✅ Integration tools installed"
else ifeq ($(UNAME_S),Linux)
	@echo "Detected Linux - installing binaries"
	@echo "Installing kind..."
	@if ! command -v kind >/dev/null 2>&1; then \
		curl -Lo /tmp/kind https://kind.sigs.k8s.io/dl/v0.20.0/kind-linux-amd64; \
		chmod +x /tmp/kind; \
		sudo mv /tmp/kind /usr/local/bin/kind; \
	fi
	@echo "Installing kubectl..."
	@if ! command -v kubectl >/dev/null 2>&1; then \
		curl -LO "https://dl.k8s.io/release/$$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"; \
		chmod +x kubectl; \
		sudo mv kubectl /usr/local/bin/; \
	fi
	@echo "Installing helm..."
	@if ! command -v helm >/dev/null 2>&1; then \
		curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash; \
	fi
	@echo "✅ Integration tools installed"
else
	@echo "❌ Unsupported OS: $(UNAME_S)"
	@echo "Please install kind, kubectl, and helm manually:"
	@echo "  - kind: https://kind.sigs.k8s.io/docs/user/quick-start/#installation"
	@echo "  - kubectl: https://kubernetes.io/docs/tasks/tools/"
	@echo "  - helm: https://helm.sh/docs/intro/install/"
	@exit 1
endif
	@echo ""
	@echo "Verifying installation..."
	@npm run test:integration:check

build: ## Build TypeScript code
	@echo "🔨 Building TypeScript..."
	npm run build
	@echo "✅ Build complete"

test: ## Run all tests (unit + property-based)
	@echo "🧪 Running all tests..."
	npm test

test-unit: ## Run unit tests only
	@echo "🧪 Running unit tests..."
	npm run test:unit

test-integration-check: ## Check if integration test prerequisites are installed
	@npm run test:integration:check

test-integration-setup: ## Setup kind cluster for integration tests
	@echo "🚀 Setting up kind cluster..."
	npm run test:integration:setup

test-integration-cleanup: ## Cleanup kind cluster
	@echo "🧹 Cleaning up kind cluster..."
	npm run test:integration:cleanup

test-integration: ## Run integration tests (requires cluster setup first)
	@echo "🧪 Running integration tests..."
	npm run test:integration

test-integration-full: ## Run integration tests with automatic setup and cleanup
	@echo "🧪 Running full integration test suite..."
	npm run test:integration:full

test-all: test test-integration-full ## Run all tests including integration tests

clean: ## Clean build artifacts and test clusters
	@echo "🧹 Cleaning build artifacts..."
	rm -rf dist/
	rm -rf coverage/
	rm -rf node_modules/.cache/
	@echo "🧹 Cleaning test clusters..."
	@npm run test:integration:cleanup 2>/dev/null || true
	@echo "✅ Clean complete"

clean-all: clean ## Clean everything including node_modules and venv
	@echo "🧹 Removing node_modules..."
	rm -rf node_modules/
	@echo "🧹 Removing Python virtual environment..."
	rm -rf venv/
	@echo "✅ Deep clean complete"

# Docker check
check-docker: ## Check if Docker is running
	@if ! docker ps >/dev/null 2>&1; then \
		echo "❌ Docker is not running. Please start Docker Desktop."; \
		exit 1; \
	fi
	@echo "✅ Docker is running"

# Quick setup for new developers
setup: install install-integration-tools build ## Complete setup for new developers
	@echo ""
	@echo "✅ Setup complete! You can now run:"
	@echo "  make test              # Run unit tests"
	@echo "  make test-integration-full  # Run integration tests"
	@echo "  make help              # See all available commands"

# CI/CD target
ci: install build test ## Run CI/CD pipeline (install, build, test)
	@echo "✅ CI pipeline complete"

ci-full: install install-integration-tools build test-all ## Run full CI/CD pipeline with integration tests
	@echo "✅ Full CI pipeline complete"
