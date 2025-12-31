# Arbiter Pipeline Infrastructure

Jenkins X-based CI/CD platform for homelab deployment providing shared pipeline infrastructure with tenant isolation.

## Overview

This repository defines and operates a shared CI/CD platform cluster using Jenkins X, Lighthouse, and Tekton. Product teams can run pipelines on shared infrastructure with logical separation through namespaces, RBAC, and policies.

## Key Features

- **Jenkins X Platform**: Kubernetes-native CI/CD with Tekton pipelines
- **Self-Service Onboarding**: OIDC-authenticated users can provision tenant resources
- **Tenant Isolation**: Namespace-per-team with RBAC and network policies
- **Golden Pipeline Catalog**: Shared, versioned Tekton tasks and pipelines
- **CDKTF Support**: Built-in support for infrastructure-as-code deployments
- **Agent-Friendly**: Highly inspectable for automated reasoning and remediation

## Documentation

Complete documentation is available in `.kiro/docs/`:

- [Overview](.kiro/docs/overview.md) - System purpose and key concepts
- [Architecture](.kiro/docs/architecture.md) - System design and components
- [Operations](.kiro/docs/operations.md) - Deployment and maintenance
- [API](.kiro/docs/api.md) - Onboarding API and contracts
- [Data Models](.kiro/docs/data-models.md) - Configuration structures
- [FAQ](.kiro/docs/faq.md) - Common questions and troubleshooting

## Quick Start

Documentation for bootstrap and deployment will be added as the platform is implemented.

## Archon Integration

This repository participates in the Archon RAG system. All documentation under `.kiro/docs/` is ingested to provide context for automated agents.

See [CLAUDE.md](CLAUDE.md) for the complete documentation contract.

## License

[Add your license]
