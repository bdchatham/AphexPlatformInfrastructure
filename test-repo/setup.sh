#!/bin/bash
set -euo pipefail

echo "Setting up test repository for pipeline execution..."

# Install dependencies
echo "Installing npm dependencies..."
npm install

# Generate CDKTF providers
echo "Generating CDKTF providers..."
npm run get

# Build TypeScript
echo "Building TypeScript..."
npm run build

# Test synth locally
echo "Testing CDKTF synth..."
npm run synth

echo ""
echo "Test repository setup complete!"
echo ""
echo "Next steps:"
echo "1. Initialize git repository: git init"
echo "2. Add files: git add ."
echo "3. Commit: git commit -m 'Initial commit'"
echo "4. Create GitHub repository and push"
echo "5. Create RepoBinding to onboard the repository"
