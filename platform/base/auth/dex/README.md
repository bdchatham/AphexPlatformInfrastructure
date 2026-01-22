# Dex OIDC Connector

This directory contains Kubernetes manifests for deploying Dex as an OIDC connector between Authentik and platform services (ArgoCD, Tekton Dashboard).

## Overview

Dex acts as an OIDC proxy that:
- Connects to Authentik as the upstream identity provider
- Provides a stable OIDC endpoint for platform services
- Translates Authentik OIDC tokens to service-specific tokens
- Simplifies service integration (services only configure Dex, not Authentik directly)

## Components

### ConfigMap (`configmap.yaml`)

Defines Dex configuration including:
- **Issuer URL**: `http://dex.auth-system.svc.cluster.local:5556`
- **Storage Backend**: Kubernetes CRDs for minimal state storage
- **Static Clients**: ArgoCD and Tekton Dashboard OIDC clients
- **Authentik Connector**: OIDC connector configuration for Authentik
- **Token Expiry**: ID tokens (24h), refresh tokens (90 days unused, 165 days absolute)

### Deployment (`deployment.yaml`)

Deploys Dex server with:
- **Image**: `ghcr.io/dexidp/dex:v2.37.0`
- **Replicas**: 1 (sufficient for homelab, can scale for production)
- **Resources**: 100m CPU / 128Mi memory (requests), 500m CPU / 256Mi memory (limits)
- **Health Checks**: Liveness and readiness probes on `/healthz` endpoint
- **Secrets**: Mounted from `dex-secrets` Secret for OIDC client credentials

### Service (`service.yaml`)

Exposes Dex within the cluster:
- **Type**: ClusterIP
- **Port**: 5556
- **DNS Name**: `dex.auth-system.svc.cluster.local`

### ServiceAccount and RBAC (`serviceaccount.yaml`, `rbac.yaml`)

Grants Dex permissions to:
- Create and manage Dex CRDs for storage
- Access Kubernetes API for CRD operations

### Secret Template (`secret.yaml.example`)

Example secret containing:
- `authentik-client-secret`: OIDC client secret for Authentik connector
- `argocd-client-secret`: OIDC client secret for ArgoCD
- `tekton-client-secret`: OIDC client secret for Tekton Dashboard

**Generate secrets with:**
```bash
openssl rand -base64 32
```

## Deployment

### Prerequisites

1. PostgreSQL must be running in `auth-system` namespace
2. Authentik must be deployed and configured with Dex OIDC provider
3. Authentik Blueprint must have created the Dex OIDC application with client ID `dex-client`

### Bootstrap Deployment

During bootstrap, Dex is deployed after Authentik:

```bash
# Create secrets
kubectl create secret generic dex-secrets \
  --namespace=auth-system \
  --from-literal=authentik-client-secret=$(openssl rand -base64 32) \
  --from-literal=argocd-client-secret=$(openssl rand -base64 32) \
  --from-literal=tekton-client-secret=$(openssl rand -base64 32)

# Deploy Dex
kubectl apply -f platform/auth/dex/serviceaccount.yaml
kubectl apply -f platform/auth/dex/rbac.yaml
kubectl apply -f platform/auth/dex/configmap.yaml
kubectl apply -f platform/auth/dex/deployment.yaml
kubectl apply -f platform/auth/dex/service.yaml

# Wait for Dex to be ready
kubectl wait --for=condition=available --timeout=300s deployment/dex -n auth-system
```

### GitOps Management

After bootstrap, Dex configuration is managed by ArgoCD through the `platform-auth` Application.

## Configuration

### Authentik Connector

The Authentik connector configuration in `configmap.yaml` must match the Authentik OIDC provider:

- **Issuer**: `http://authentik.auth-system.svc.cluster.local:9000/application/o/dex/`
- **Client ID**: `dex-client` (defined in Authentik Blueprint)
- **Client Secret**: Stored in `dex-secrets` Secret
- **Redirect URI**: `http://dex.auth-system.svc.cluster.local:5556/callback`
- **Scopes**: `openid`, `profile`, `email`, `groups`

### Static Clients

Platform services are configured as static clients in Dex:

**ArgoCD:**
- Client ID: `argocd`
- Redirect URIs: `http://localhost:8080/auth/callback`, `https://argocd.example.com/auth/callback`

**Tekton Dashboard:**
- Client ID: `tekton-dashboard`
- Redirect URIs: `http://localhost:9097/auth/callback`, `https://tekton.example.com/auth/callback`

Update redirect URIs to match your actual service URLs.

## Verification

### Check Dex Health

```bash
# Check pod status
kubectl get pods -n auth-system -l app=dex

# Check logs
kubectl logs -n auth-system -l app=dex

# Test health endpoint
kubectl exec -n auth-system deploy/dex -- wget -qO- http://localhost:5556/healthz
```

### Verify OIDC Discovery

```bash
# Check OIDC discovery endpoint
kubectl exec -n auth-system deploy/dex -- wget -qO- http://localhost:5556/.well-known/openid-configuration
```

### Test Authentik Connection

```bash
# Check Dex logs for Authentik connector initialization
kubectl logs -n auth-system -l app=dex | grep authentik
```

## Troubleshooting

### Dex Pod Fails to Start

**Symptoms**: Pod in CrashLoopBackOff or Error state

**Possible Causes**:
- Invalid configuration in ConfigMap
- Missing secrets
- Insufficient RBAC permissions

**Resolution**:
1. Check pod logs: `kubectl logs -n auth-system -l app=dex`
2. Verify ConfigMap syntax: `kubectl get configmap dex-config -n auth-system -o yaml`
3. Verify secrets exist: `kubectl get secret dex-secrets -n auth-system`
4. Verify ServiceAccount and RBAC: `kubectl get sa,clusterrole,clusterrolebinding -n auth-system | grep dex`

### Cannot Connect to Authentik

**Symptoms**: Dex logs show connection errors to Authentik

**Possible Causes**:
- Authentik not running
- Incorrect Authentik issuer URL
- Network connectivity issues

**Resolution**:
1. Verify Authentik is running: `kubectl get pods -n auth-system -l app=authentik`
2. Test DNS resolution: `kubectl exec -n auth-system deploy/dex -- nslookup authentik.auth-system.svc.cluster.local`
3. Test HTTP connectivity: `kubectl exec -n auth-system deploy/dex -- wget -qO- http://authentik.auth-system.svc.cluster.local:9000/application/o/dex/.well-known/openid-configuration`
4. Verify Authentik OIDC provider configuration in Authentik UI

### Authentication Fails

**Symptoms**: Users cannot authenticate through Dex

**Possible Causes**:
- Incorrect client secrets
- Authentik OIDC provider misconfigured
- Token validation failures

**Resolution**:
1. Check Dex logs for authentication errors: `kubectl logs -n auth-system -l app=dex | grep error`
2. Verify client secrets match between Dex ConfigMap and service configurations
3. Verify Authentik OIDC provider has correct redirect URI for Dex
4. Check Authentik logs: `kubectl logs -n auth-system -l app=authentik`

### Storage Backend Errors

**Symptoms**: Dex logs show errors accessing Kubernetes storage

**Possible Causes**:
- Insufficient RBAC permissions
- Kubernetes API unavailable

**Resolution**:
1. Verify ClusterRole permissions: `kubectl get clusterrole dex -o yaml`
2. Verify ClusterRoleBinding: `kubectl get clusterrolebinding dex -o yaml`
3. Check Dex can access CRDs: `kubectl auth can-i create customresourcedefinitions --as=system:serviceaccount:auth-system:dex`

## Security Considerations

- **Secrets**: All OIDC client secrets are stored in Kubernetes Secrets, never in Git
- **Network**: Dex is only accessible within the cluster via ClusterIP service
- **RBAC**: Dex ServiceAccount has minimal permissions (only CRD management)
- **Token Expiry**: ID tokens expire after 24 hours, refresh tokens after 90 days unused

## References

- [Dex Documentation](https://dexidp.io/docs/)
- [Dex OIDC Connector](https://dexidp.io/docs/connectors/oidc/)
- [Dex Kubernetes Storage](https://dexidp.io/docs/storage/#kubernetes-custom-resource-definitions-crds)
