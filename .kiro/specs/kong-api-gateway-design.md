# Kong API Gateway — Design

File-level changes for each phase from the [spec](kong-api-gateway.md).

---

## Phase 1: Deploy Kong, Remove Traefik

All changes in **AphexPlatformInfrastructure**.

### Kong ArgoCD Application
New: `platform/deployments/k3s/argocd/apps/platform-kong.yaml`
- Helm chart `kong` v2.47.0 from `charts.konghq.com`
- DB-less mode, Gateway API enabled, sync wave `"4"`
- Deploys to `kong-system` namespace

### Gateway Resource
Modify: `platform/deployments/k3s/gateway/gateway.yaml`
- `gatewayClassName`: `traefik` → `kong`
- `namespace`: `kube-system` → `kong-system`
- Ports: `8443`/`8000` → `443`
- Removed plain HTTP listener
- Added `api` listener for `aphex.arbiter-dev.com`

### KongPlugin CRDs
New: `platform/deployments/k3s/gateway/plugins/rate-limit.yaml` — 100 req/min per consumer
New: `platform/deployments/k3s/gateway/plugins/key-auth.yaml` — `apikey` header auth

### Route Migration
Modify: `routes/argocd.yaml`, `routes/auth.yaml`, `routes/dex.yaml`, `routes/tekton.yaml`
- parentRef namespace `kube-system` → `kong-system`

### Kustomization
Modify: `platform/deployments/k3s/gateway/kustomization.yaml`
- Removed `traefik-config.yaml`, added `plugins/*.yaml`

### Traefik Removal
Delete: `platform/deployments/k3s/gateway/traefik-config.yaml`
Manual: Add `disable: [traefik]` to `/etc/rancher/k3s/config.yaml`, restart k3s

### Wildcard Cert
Modify: `platform/deployments/k3s/cert-foundation/certificate.yaml`
- namespace `kube-system` → `kong-system`

---

## Phase 2: Controller MCP Route Provisioning

### CRD — AphexControllerRuntime
Modify: `api/v1alpha1/knowledgebase_types.go` — added `ExternalURL` to `MCPStatus`
Regenerated CRD manifests via `make manifests`

### Constants — AphexKnowledgeBaseController
Modify: `controller/infra_constants.go`
- `PlatformGatewayNamespace` → `kong-system`
- Added `PlatformAPISectionName = "api"`
- Added `PlatformAPIDomain = "aphex.arbiter-dev.com"`

### HTTPRoute Provisioner — AphexKnowledgeBaseController
Rewrite: `controller/httproute_provisioner.go`
- Kong annotations: `strip-path`, `plugins` (rate-limit + key-auth)
- Targets `api` listener section on `aphex.arbiter-dev.com`
- Two rules: `/mcp/<kb-name>` → mcp-server, `/query/<kb-name>` → query service
- MCP port from `kb.Spec.MCP.Port`

### ExternalURL — AphexKnowledgeBaseController
Modify: `controller/knowledgebase_controller.go`
- Sets `kb.Status.MCP.ExternalURL = "https://aphex.arbiter-dev.com/mcp/<kb-name>"`

---

## Phase 3: Auth Enforcement (Future)
- KongConsumer provisioning per org
- API key Secret creation
- Plugin annotations already in place from Phase 2

---

## File Change Summary

| Repo | File | Action |
|------|------|--------|
| PlatformInfra | `argocd/apps/platform-kong.yaml` | Create |
| PlatformInfra | `gateway/gateway.yaml` | Modify |
| PlatformInfra | `gateway/plugins/rate-limit.yaml` | Create |
| PlatformInfra | `gateway/plugins/key-auth.yaml` | Create |
| PlatformInfra | `gateway/kustomization.yaml` | Modify |
| PlatformInfra | `gateway/traefik-config.yaml` | Delete |
| PlatformInfra | `gateway/routes/*.yaml` (×4) | Modify |
| PlatformInfra | `cert-foundation/certificate.yaml` | Modify |
| ControllerRuntime | `api/v1alpha1/knowledgebase_types.go` | Modify |
| KBController | `controller/infra_constants.go` | Modify |
| KBController | `controller/httproute_provisioner.go` | Rewrite |
| KBController | `controller/knowledgebase_controller.go` | Modify |
