# Makefile Quick Reference

This project uses a Makefile to simplify common development tasks. Run `make help` to see all available commands.

## Quick Start

```bash
# Complete setup for new developers
make setup

# Run unit tests
make test

# Run integration tests
make test-integration-full
```

## Installation Commands

| Command | Description |
|---------|-------------|
| `make install` | Install all dependencies (Node.js + Python) |
| `make install-deps` | Install Node.js and Python dependencies |
| `make install-integration-tools` | Install kind, kubectl, helm for integration tests |
| `make setup` | Complete setup for new developers (all of the above + build) |

## Build Commands

| Command | Description |
|---------|-------------|
| `make build` | Build TypeScript code |
| `make clean` | Clean build artifacts and test clusters |
| `make clean-all` | Clean everything including node_modules and venv |

## Testing Commands

| Command | Description |
|---------|-------------|
| `make test` | Run all unit tests |
| `make test-unit` | Run unit tests only |
| `make test-integration-check` | Check if integration test prerequisites are installed |
| `make test-integration-setup` | Setup kind cluster for integration tests |
| `make test-integration` | Run integration tests (requires cluster setup first) |
| `make test-integration-cleanup` | Cleanup kind cluster |
| `make test-integration-full` | Run integration tests with automatic setup and cleanup |
| `make test-all` | Run all tests including integration tests |

## CI/CD Commands

| Command | Description |
|---------|-------------|
| `make ci` | Run CI/CD pipeline (install, build, test) |
| `make ci-full` | Run full CI/CD pipeline with integration tests |

## Utility Commands

| Command | Description |
|---------|-------------|
| `make help` | Show all available commands |
| `make check-docker` | Check if Docker is running |

## Common Workflows

### New Developer Setup
```bash
make setup
```

### Daily Development
```bash
# Make code changes...
make build
make test
```

### Before Committing
```bash
make test-all
```

### Integration Test Development
```bash
# Setup cluster once
make test-integration-setup

# Run tests multiple times
make test-integration

# Cleanup when done
make test-integration-cleanup
```

### Clean Slate
```bash
make clean-all
make setup
```

## Platform Support

The Makefile automatically detects your operating system:

- **macOS**: Uses Homebrew to install tools
- **Linux**: Downloads and installs binaries directly
- **Windows**: Manual installation required (see test/integration/README.md)

## Prerequisites

### For Basic Development
- Node.js 20.x or later
- Python 3.11 or later
- make (usually pre-installed on macOS/Linux)

### For Integration Tests
- Docker (must be running)
- kind, kubectl, helm (can be installed via `make install-integration-tools`)

### For macOS
- Homebrew (for `make install-integration-tools`)

## Troubleshooting

### "make: command not found"
- **macOS/Linux**: make should be pre-installed. Try `xcode-select --install` on macOS.
- **Windows**: Use WSL or install make via Chocolatey: `choco install make`

### "Homebrew not found" (macOS)
Install Homebrew first: https://brew.sh

### "Docker is not running"
Start Docker Desktop before running integration tests.

### Integration tools won't install
Try manual installation following the guide in `test/integration/README.md`

## Alternative: npm Scripts

If you prefer npm scripts over make:

```bash
npm install                      # Instead of: make install-deps
npm run build                    # Instead of: make build
npm test                         # Instead of: make test
npm run test:integration:full    # Instead of: make test-integration-full
```

See `package.json` for all available npm scripts.
