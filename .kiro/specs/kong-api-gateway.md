# Kong API Gateway — Unified Platform Gateway

## Problem

The platform uses Traefik (bundled with k3s) as a reverse proxy for UI services. It provides L7 routing but no API Gateway features. This creates two issues:

1. **MCP servers are inaccessible** — only reachable via `kubectl port-forward`. The existing KB HTTPRoute is broken (`NoMatchingListenerHostname` because it uses `home.local` while the gateway listener accepts `*.arbiter-dev.com`).
2. **No gateway-level auth or rate limiting** — UI services handle auth individually (Dex/OIDC at the app level). API services (MCP, query) have no auth at all.

## Decision

Replace Traefik with Kong as the sole platform gateway. Kong implements the same Kubernetes Gateway API (HTTPRoute resources are unchanged) but adds plugin-based auth, rate limiting, and request transformation.

### Why Replace Rather Than Add Alongside

- One gateway is simpler to operate than two
- Auth can be standardized across UI and API traffic
- Existing HTTPRoutes work with Kong — same Gateway API, different implementation
- Traefik is k3s-bundled but can be disabled via `--disable=traefik` server flag

## Current State

```
Traefik (platform-gateway) — *.arbiter-dev.com
├── argocd.arbiter-dev.com   → argocd-server:443      (app-level OIDC via Dex)
├── auth.arbiter-dev.com     → authentik:9000          (identity provider)
├── dex.arbiter-dev.com      → dex:5556                (OIDC connector)
├── tekton.arbiter-dev.com   → tekton-dashboard:9097   (app-level OIDC via Dex)
└── archon-knowledge.home.local → BROKEN (hostname mismatch)
```

- Gateway: `platform-gateway` in `kube-system`, GatewayClass `traefik`
- TLS: Wildcard cert `wildcard-arbiter-dev-tls` via cert-manager + Let's Encrypt
- DNS: external-dns with Cloudflare provider, syncs from `gateway-httproute` source
- Traefik: k3s-bundled HelmChart, configured via `HelmChartConfig` to enable Gateway API provider
- Auth: Authentik → Dex → per-app OIDC. No gateway-level enforcement.

## Target State

```
Kong (platform-gateway) — *.arbiter-dev.com
│
├── UI Routes (OIDC auth via Kong OpenID Connect plugin)
│   ├── argocd.arbiter-dev.com   → argocd-server:443
│   ├── auth.arbiter-dev.com     → authentik:9000       (no auth — it IS the IdP)
│   ├── dex.arbiter-dev.com      → dex:5556             (no auth — it IS the OIDC provider)
│   └── tekton.arbiter-dev.com   → tekton-dashboard:9097
│
├── API Routes (key-auth + rate limiting)
│   ├── aphex.arbiter-dev.com/mcp/archon-knowledge   → mcp-server:3000
│   ├── aphex.arbiter-dev.com/mcp/zerg-knowledge     → mcp-server:3000
│   └── aphex.arbiter-dev.com/query/archon-knowledge → query-service:8080
│
└── Plugins
    ├── openid-connect (UI routes — Dex as IdP)
    ├── key-auth (API routes — per-org API keys)
    └── rate-limiting (API routes — 100 req/min per consumer)
```

## Design

### Kong Deployment

- **Helm chart** via ArgoCD Application (same pattern as `platform-gpu-operator`)
- **DB-less mode** — declarative config, no PostgreSQL dependency
- **Gateway API** enabled via `gateway.enabled: true` in Helm values
- **Namespace**: `kube-system` (replaces Traefik's position)

### k3s Configuration

Disable Traefik by adding `--disable=traefik` to the k3s server config:

```
# /etc/rancher/k3s/config.yaml
disable:
  - traefik
```

After restart, k3s stops managing Traefik. Kong takes over the same Gateway resource name (`platform-gateway`) and listener config.

### Gateway Resource

Unchanged from current — same name, listeners, and cert. Only `gatewayClassName` changes:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: platform-gateway
  namespace: kube-system
spec:
  gatewayClassName: kong          # was: traefik
  listeners:
    - name: https
      protocol: HTTPS
      port: 443                   # Kong default (was 8443 for Traefik)
      hostname: "*.arbiter-dev.com"
      tls:
        mode: Terminate
        certificateRefs:
          - name: wildcard-arbiter-dev-tls
      allowedRoutes:
        namespaces:
          from: All
```

### Existing HTTPRoutes

No changes needed. ArgoCD, Tekton, Auth, and Dex routes already reference `platform-gateway` by name. They'll automatically attach to Kong once the GatewayClass changes.

### New: API Listener

Add a second listener for API traffic on a dedicated hostname:

```yaml
    - name: api
      protocol: HTTPS
      port: 443
      hostname: "aphex.arbiter-dev.com"
      tls:
        mode: Terminate
        certificateRefs:
          - name: wildcard-arbiter-dev-tls
      allowedRoutes:
        namespaces:
          from: All
```

### New: Per-KB MCP Route (Created by Controller)

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: archon-knowledge-api
  namespace: kb-archon-knowledge
  annotations:
    konghq.com/strip-path: "true"
    konghq.com/plugins: "mcp-rate-limit,mcp-key-auth"
spec:
  parentRefs:
    - name: platform-gateway
      namespace: kube-system
      sectionName: api
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /mcp/archon-knowledge
      backendRefs:
        - name: mcp-server-archon-knowledge
          port: 3000
    - matches:
        - path:
            type: PathPrefix
            value: /query/archon-knowledge
      backendRefs:
        - name: archon-knowledge-query
          port: 8080
```

### Auth Model

**UI routes**: Kong OpenID Connect plugin pointing at Dex. Replaces per-app OIDC config — auth moves to the gateway layer. Authentik and Dex routes are excluded (they ARE the auth infrastructure).

**API routes**: Kong `key-auth` plugin. Each organization gets a consumer + API key. The KB controller provisions these when a KnowledgeBase is created.

**Future**: Migrate API auth to JWT via Dex for user-level access tokens.

### Rate Limiting

Kong `rate-limiting` plugin on API routes:

- 100 requests/minute per consumer (default)
- `policy: local` (single node, sufficient for homelab)
- Applied via KongPlugin CRD, referenced by HTTPRoute annotation

### Controller Changes

**AphexControllerRuntime** — Add `ExternalURL` to `MCPStatus`:

```go
type MCPStatus struct {
    Deployed      bool   `json:"deployed,omitempty"`
    ServiceName   string `json:"serviceName,omitempty"`
    ServiceURL    string `json:"serviceURL,omitempty"`
    ExternalURL   string `json:"externalURL,omitempty"`   // new
    ReadyReplicas int32  `json:"readyReplicas,omitempty"`
}
```

**AphexKnowledgeBaseController** — Replace `httproute_provisioner.go`:

- Build HTTPRoute targeting `platform-gateway` sectionName `api`
- Path prefix: `/mcp/<kb-name>` and `/query/<kb-name>`
- Annotate with Kong plugins
- Set `MCPStatus.ExternalURL` to `https://aphex.arbiter-dev.com/mcp/<kb-name>`
- Provision Kong consumer + key-auth credential per organization

### Discoverability

```bash
$ kubectl get kb archon-knowledge -n org-archon -o jsonpath='{.status.mcp.externalURL}'
https://aphex.arbiter-dev.com/mcp/archon-knowledge
```

### Kiro CLI Config

```json
{
  "mcpServers": {
    "archon": {
      "type": "sse",
      "url": "https://aphex.arbiter-dev.com/mcp/archon-knowledge/mcp",
      "headers": {
        "apikey": "<org-api-key>"
      }
    }
  }
}
```

## Implementation Tasks

### Phase 1: Kong Replaces Traefik

Repo: **AphexPlatformInfrastructure**

1. Add Kong Helm chart ArgoCD Application
   - `platform/deployments/k3s/argocd/apps/platform-kong.yaml`
   - DB-less mode, Gateway API enabled, KongPlugin CRDs installed
2. Add `api` listener to Gateway resource for `aphex.arbiter-dev.com`
3. Add platform-level KongPlugin resources (rate-limiting, key-auth)
4. Update `gatewayClassName: traefik` → `kong` in `gateway.yaml`
5. Remove `traefik-config.yaml` (HelmChartConfig)
6. Disable Traefik in k3s server config (`--disable=traefik`)
7. Verify all existing UI routes work through Kong (argocd, tekton, auth, dex)
8. Verify external-dns creates DNS records from Kong-backed HTTPRoutes

### Phase 2: MCP Route Provisioning

Repo: **AphexControllerRuntime**

9. Add `ExternalURL` field to `MCPStatus`
10. Regenerate CRD manifests, commit, push

Repo: **AphexKnowledgeBaseController**

11. Rewrite `httproute_provisioner.go` — target `api` sectionName, path prefix `/mcp/<kb-name>` + `/query/<kb-name>`, Kong annotations
12. Update `reconcileMCPServer` to set `MCPStatus.ExternalURL`
13. Add constants for API listener section name
14. Build, push controller image
15. Verify MCP accessible at `https://aphex.arbiter-dev.com/mcp/archon-knowledge/mcp`

### Phase 3: Auth + Rate Limiting

Repo: **AphexKnowledgeBaseController**

16. Add Kong consumer provisioning per organization
17. Generate API key credential, store in KB namespace Secret
18. Annotate MCP HTTPRoute with `mcp-key-auth` and `mcp-rate-limit` plugin refs
19. Verify: missing key → 401, valid key → 200, exceed rate → 429

### Phase 4: UI Auth Migration (Optional)

Repo: **AphexPlatformInfrastructure**

20. Add Kong OpenID Connect plugin pointing at Dex
21. Annotate ArgoCD and Tekton routes with OIDC plugin
22. Remove app-level OIDC config from ArgoCD and Tekton
23. Verify browser auth flow works through Kong → Dex → Authentik

## Risks

| Risk | Mitigation |
|------|------------|
| Kong migration breaks UI routes | Phase 1 validates all existing routes before proceeding. Rollback: re-enable Traefik. |
| k3s upgrade re-enables Traefik | `--disable=traefik` in config.yaml persists across upgrades. |
| Kong resource overhead | DB-less mode is lightweight. Single replica sufficient for homelab. |
| Path prefix stripping breaks MCP | Kong `strip-path` annotation is well-tested. Verify `/mcp` endpoint works after strip. |
| API key management UX | Phase 3 stores keys in Secrets. Self-service distribution is a future enhancement. |

## Open Questions

1. **Port**: Kong defaults to 443. Traefik uses 8443. Need to verify external-dns and cert-manager work with the port change, or keep 8443.
2. **UI auth timing**: Phase 4 (OIDC at gateway) is optional. Current app-level OIDC works. Worth doing only if we want to standardize.
3. **API key rotation**: How often? Manual or automated?
