# Platform Controllers

This directory contains Kubernetes manifests for deploying the Aphex platform controllers.

## Architecture

The platform uses four separate controllers, each managing a specific CRD:

| Controller | CRD | Repository |
|------------|-----|------------|
| repobinding-controller | RepoBinding | AphexRepoBindingController |
| organization-controller | Organization | AphexOrganizationController |
| knowledgebase-controller | KnowledgeBase | AphexKnowledgeBaseController |
| agent-controller | Agent | AphexAgentController |

All controllers share common types and utilities from `AphexControllerRuntime`.

## Manifests

- `repobinding-controller.yaml` - RepoBinding controller deployment, RBAC, and service account
- `organization-controller.yaml` - Organization controller deployment, RBAC, and service account
- `knowledgebase-controller.yaml` - KnowledgeBase controller deployment, RBAC, and service account
- `agent-controller.yaml` - Agent controller deployment, RBAC, and service account
- `webhook-configuration.yaml` - Validating webhook configuration (optional)

## Deployment

These manifests are deployed via ArgoCD through the `platform-controllers` Application.

## Images

Controller images are built from their respective repositories and published to:
- `ghcr.io/bdchatham/repobinding-controller:latest`
- `ghcr.io/bdchatham/organization-controller:latest`
- `ghcr.io/bdchatham/knowledgebase-controller:latest`
- `ghcr.io/bdchatham/agent-controller:latest`
