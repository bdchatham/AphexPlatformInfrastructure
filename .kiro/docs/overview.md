# Overview

## Purpose

The **Arbiter Pipeline Infrastructure** is a CDK-based infrastructure package that provides a shared execution environment for deploying CDK applications across the Arbiter agent suite (including Archon and future agents). This repository contains:

1. **AphexCluster Construct**: A CDK construct that creates an EKS cluster with Argo Workflows and Argo Events
2. **Container Images**: Pre-built Docker images (builder, deployer, tester, validator) with execution scripts
3. **Integration Layer**: CloudFormation exports and IRSA configuration for pipeline consumption

The system enables multiple CDK applications to share a common deployment infrastructure while maintaining isolation and security through Kubernetes namespaces, RBAC, and IAM Roles for Service Accounts (IRSA).

## Archon Integration

This repository is ingested by the **Archon** RAG system, which reads all Markdown files under `.kiro/docs/` to build mental models for sourcing code and architectural information.

Documentation in this repository follows the Archon documentation contract defined in `CLAUDE.md` at the repo root.

## Quick Start

### Prerequisites
- Node.js 20.x or later
- Python 3.11 or later
- AWS CLI configured with appropriate credentials
- AWS CDK CLI (`npm install -g aws-cdk`)
- Docker (for building container images and integration tests)

### Installation

```bash
# Complete setup (recommended for new developers)
make setup

# Or step by step:
make install                    # Install Node.js and Python dependencies
make install-integration-tools  # Install kind, kubectl, helm
make build                      # Build TypeScript
```

### Deploy the Infrastructure

```bash
# Bootstrap CDK (first time only)
cdk bootstrap

# Deploy the cluster
cdk deploy
```

### Run Tests

```bash
# Unit tests
npm run test:unit

# Property-based tests
pytest test/

# Integration tests (requires Docker)
npm run test:integration:full
```

## Key Concepts

### AphexCluster
A CDK construct that creates and manages an EKS cluster with Argo Workflows and Argo Events. It provides methods for creating isolated pipelines with their own namespaces and service accounts.

### Container Images
Four pre-built container images that contain execution scripts:
- **Builder**: Clones repos, runs build commands, uploads artifacts to S3
- **Deployer**: Synthesizes and deploys CDK stacks to CloudFormation
- **Tester**: Executes test commands and reports results
- **Validator**: Validates configuration files and prerequisites

### IRSA (IAM Roles for Service Accounts)
Kubernetes service accounts are mapped to IAM roles, allowing pods to assume AWS permissions without long-lived credentials. Each pipeline gets its own service account with isolated permissions.

### Multi-Pipeline Isolation
Multiple pipelines can run on the same cluster with isolation through:
- Kubernetes namespaces (one per pipeline)
- Network policies (restrict inter-namespace communication)
- Resource quotas (prevent resource exhaustion)
- RBAC (separate service accounts and roles)

### CloudFormation Exports
The cluster exports key attributes (cluster name, OIDC provider ARN, kubectl role ARN) via CloudFormation, allowing pipeline stacks to discover and connect to the cluster.

## Related Repositories

This infrastructure package is designed to be consumed by:
- **Archon**: The first Arbiter agent, which will use this infrastructure for its deployment pipelines
- **Future Arbiter Agents**: Additional agents in the Arbiter suite that need CDK deployment capabilities

**Source**
- `CLAUDE.md`
- `.kiro/steering/archon-docs.md`
- `README.md`
- `lib/constructs/aphex-cluster.ts`
- `lib/arbiter-pipeline-infrastructure-stack.ts`
