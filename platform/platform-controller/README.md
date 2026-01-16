# Platform Controller

This directory contains the platform controller implementation and deployment manifests.

## Contents

- Go source code for the RepoBinding and Organization controllers
- Kubernetes deployment manifests (ServiceAccount, ClusterRole, Deployment)
- Tenant resource templates (namespace, RBAC, quotas, network policies)

## Purpose

The platform controller reconciles RepoBinding and Organization resources and provisions tenant infrastructure automatically.
