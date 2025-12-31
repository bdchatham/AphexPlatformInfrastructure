# Dex OIDC Provider Deployment

This directory contains manifests for deploying Dex as a self-hosted OIDC provider for the Jenkins X platform.

## Overview

Dex provides OpenID Connect (OIDC) authentication for the Kubernetes cluster, enabling users in the `engineering` group to authenticate and onboard repositories without requiring cluster admin access.

## Components

### 1. Dex ConfigMap (`dex-config.yaml`)

Configures Dex with:
- **Issuer URL**: `https://dex.homelab.local` (customize for your environment)
- **Storage**: Kubernetes backend (stores data in CRDs)
- **Static Users**: Pre-configured admin and engineer users
- **GitHub Connector**: Optional GitHub OAuth integration
- **Static Client**: Kubernetes API server client configuration

### 2. Dex Deployment (`dex-deployment.yaml`)

Deploys Dex with:
- **Image**: `ghcr.io/dexidp/dex:v2.37.0`
- **Replicas**: 1 (sufficient for homelab)
- **Resources**: 100m CPU / 128Mi memory (requests), 500m CPU / 256Mi memory (limits)
- **Health Checks**: Liveness and readiness probes on `/healthz`
- **Service Account**: Dedicated service account with RBAC permissions

### 3. Dex Service

Exposes Dex on port 5556 within the cluster.

### 4. Engineering RBAC (`engineering-rbac.yaml`)

Creates RBAC for the `engineering` OIDC group:
- **repo-onboarder ClusterRole**: Allows creating and managing RepoBinding resources
- **pipeline-catalog-viewer ClusterRole**: Allows viewing shared Tasks and Pipelines
- **ClusterRoleBindings**: Binds roles to the `engineering` group
- **RoleBindings**: Allows viewing platform components in `pipeline-system` and `pipeline-catalog` namespaces

## Deployment Steps

### Step 1: Create auth-system Namespace

```bash
kubectl create namespace auth-system
```

### Step 2: Deploy Dex ConfigMap

```bash
kubectl apply -f dex-config.yaml
```

**Important**: Before deploying, customize the following in `dex-config.yaml`:
- Replace `https://dex.homelab.local` with your actual Dex URL
- Replace `your-github-org` with your GitHub organization name
- Set `$GITHUB_OAUTH_CLIENT_ID` and `$GITHUB_OAUTH_CLIENT_SECRET` environment variables if using GitHub authentication
- Update static user passwords (generate bcrypt hashes with `htpasswd`)

### Step 3: Deploy Dex

```bash
kubectl apply -f dex-deployment.yaml
```

### Step 4: Verify Dex is Running

```bash
# Check pod status
kubectl get pods -n auth-system

# Check logs
kubectl logs -n auth-system deployment/dex

# Verify service
kubectl get svc -n auth-system
```

Expected output:
```
NAME   TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)    AGE
dex    ClusterIP   10.43.xxx.xxx   <none>        5556/TCP   1m
```

### Step 5: Configure Kubernetes API Server for OIDC

Follow the instructions in `k8s-apiserver-oidc-config.md` to configure your Kubernetes API server to use Dex as an OIDC provider.

For k3s (most common in homelab):

```bash
# Edit /etc/rancher/k3s/config.yaml
sudo nano /etc/rancher/k3s/config.yaml

# Add:
kube-apiserver-arg:
  - "oidc-issuer-url=https://dex.homelab.local"
  - "oidc-client-id=kubernetes"
  - "oidc-username-claim=email"
  - "oidc-groups-claim=groups"

# Restart k3s
sudo systemctl restart k3s
```

### Step 6: Deploy Engineering RBAC

```bash
kubectl apply -f engineering-rbac.yaml
```

### Step 7: Verify RBAC

```bash
# Check ClusterRoles
kubectl get clusterrole repo-onboarder pipeline-catalog-viewer

# Check ClusterRoleBindings
kubectl get clusterrolebinding engineering-onboarders engineering-catalog-viewers

# Check RoleBindings
kubectl get rolebinding -n pipeline-system engineering-platform-viewers
kubectl get rolebinding -n pipeline-catalog engineering-catalog-viewers
```

## User Authentication

### Installing kubelogin

Users need to install the `kubelogin` plugin to authenticate via OIDC:

```bash
# Install via krew
kubectl krew install oidc-login
```

### Configuring kubectl

Users configure kubectl to use OIDC authentication:

```bash
kubectl config set-credentials oidc \
  --exec-api-version=client.authentication.k8s.io/v1beta1 \
  --exec-command=kubectl \
  --exec-arg=oidc-login \
  --exec-arg=get-token \
  --exec-arg=--oidc-issuer-url=https://dex.homelab.local \
  --exec-arg=--oidc-client-id=kubernetes \
  --exec-arg=--oidc-client-secret=kubernetes-client-secret

# Use OIDC credentials
kubectl config set-context --current --user=oidc
```

### Testing Authentication

```bash
# This should prompt for authentication
kubectl get pods

# Login with static user credentials:
# Email: admin@homelab.local
# Password: admin
# OR
# Email: engineer@homelab.local
# Password: engineer
```

## Static Users

The default configuration includes two static users:

| Email | Username | Password | Groups |
|-------|----------|----------|--------|
| admin@homelab.local | admin | admin | engineering, admins |
| engineer@homelab.local | engineer | engineer | engineering |

**Security Note**: Change these passwords in production! Generate bcrypt hashes:

```bash
# Generate bcrypt hash for a password
echo "your-password" | htpasswd -BinC 10 admin | cut -d: -f2
```

## GitHub OAuth (Optional)

To enable GitHub OAuth authentication:

1. Create a GitHub OAuth App:
   - Go to GitHub Settings → Developer settings → OAuth Apps
   - Click "New OAuth App"
   - Set Authorization callback URL to: `https://dex.homelab.local/callback`
   - Note the Client ID and Client Secret

2. Update `dex-config.yaml`:
   ```yaml
   connectors:
     - type: github
       id: github
       name: GitHub
       config:
         clientID: your-github-client-id
         clientSecret: your-github-client-secret
         redirectURI: https://dex.homelab.local/callback
         orgs:
           - name: your-github-org
             teams:
               - engineering
   ```

3. Redeploy Dex:
   ```bash
   kubectl apply -f dex-config.yaml
   kubectl rollout restart deployment/dex -n auth-system
   ```

## Exposing Dex

For production use, expose Dex via Ingress with TLS:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: dex
  namespace: auth-system
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
spec:
  tls:
    - hosts:
        - dex.homelab.local
      secretName: dex-tls
  rules:
    - host: dex.homelab.local
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: dex
                port:
                  number: 5556
```

## Troubleshooting

### Dex Pod Not Starting

Check logs:
```bash
kubectl logs -n auth-system deployment/dex
```

Common issues:
- ConfigMap not found: Ensure `dex-config.yaml` is applied
- Invalid YAML: Validate the config.yaml syntax
- RBAC permissions: Ensure the Dex service account has required permissions

### Authentication Fails

Check:
1. API server is configured with correct OIDC flags
2. Dex is accessible from the API server
3. Client ID and secret match between Dex and kubectl config
4. User exists in Dex (static users or GitHub)
5. User is in the `engineering` group

View Dex logs:
```bash
kubectl logs -n auth-system deployment/dex -f
```

### RBAC Denials

Verify user groups:
```bash
# Check user's groups
kubectl auth whoami

# Test permissions
kubectl auth can-i create repobindings.platform.arbiter.io
```

If the user is not in the `engineering` group, they won't have permissions to create RepoBindings.

## Security Considerations

1. **Change Default Passwords**: The static users have default passwords. Change them immediately.
2. **Use HTTPS**: In production, always use HTTPS for the Dex issuer URL.
3. **Rotate Secrets**: Regularly rotate the client secret and GitHub OAuth credentials.
4. **Limit Group Claims**: Only include necessary groups in OIDC claims.
5. **Monitor Logs**: Monitor Dex and API server logs for authentication failures.
6. **Network Policies**: Consider adding NetworkPolicies to restrict access to Dex.

## References

- [Dex Documentation](https://dexidp.io/docs/)
- [Kubernetes OIDC Authentication](https://kubernetes.io/docs/reference/access-authn-authz/authentication/#openid-connect-tokens)
- [kubelogin Plugin](https://github.com/int128/kubelogin)
- [Dex GitHub Connector](https://dexidp.io/docs/connectors/github/)
