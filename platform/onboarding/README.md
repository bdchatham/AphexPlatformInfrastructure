# Onboarding Controller

This directory contains the onboarding controller implementation and deployment manifests.

## Contents

- Go source code for the RepoBinding controller
- Kubernetes deployment manifests (ServiceAccount, ClusterRole, Deployment)
- Tenant resource templates (namespace, RBAC, quotas, network policies)

## Purpose

The onboarding controller reconciles RepoBinding resources and provisions tenant infrastructure automatically.
