# Kubernetes API Server OIDC Configuration

This document provides instructions for configuring the Kubernetes API server to use Dex as an OIDC provider.

## Prerequisites

- Dex must be deployed and accessible
- You need cluster admin access to modify API server configuration
- The Dex issuer URL must be accessible from the API server

## Configuration for k3s

For k3s clusters (common in homelab environments), add the following to `/etc/rancher/k3s/config.yaml`:

```yaml
kube-apiserver-arg:
  - "oidc-issuer-url=https://dex.homelab.local"
  - "oidc-client-id=kubernetes"
  - "oidc-username-claim=email"
  - "oidc-groups-claim=groups"
  # Optional: If using self-signed certificates
  # - "oidc-ca-file=/etc/ssl/certs/dex-ca.crt"
```

After updating the configuration, restart k3s:

```bash
sudo systemctl restart k3s
```

## Configuration for kubeadm Clusters

For kubeadm-based clusters, edit the kube-apiserver manifest at `/etc/kubernetes/manifests/kube-apiserver.yaml`:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: kube-apiserver
  namespace: kube-system
spec:
  containers:
    - command:
        - kube-apiserver
        # ... existing flags ...
        - --oidc-issuer-url=https://dex.homelab.local
        - --oidc-client-id=kubernetes
        - --oidc-username-claim=email
        - --oidc-groups-claim=groups
        # Optional: If using self-signed certificates
        # - --oidc-ca-file=/etc/ssl/certs/dex-ca.crt
      # ... rest of configuration ...
```

The kubelet will automatically restart the API server when the manifest changes.

## Configuration for Managed Kubernetes (EKS, GKE, AKS)

### Amazon EKS

For EKS, you need to configure OIDC through the cluster configuration:

```bash
aws eks update-cluster-config \
  --name your-cluster-name \
  --identity-provider-config \
    type=oidc,\
    identityProviderConfigName=dex,\
    issuerUrl=https://dex.homelab.local,\
    clientId=kubernetes,\
    usernameClaim=email,\
    groupsClaim=groups
```

### Google GKE

GKE uses Workload Identity and doesn't support custom OIDC providers directly. Consider using Google's identity provider or deploying on a different platform.

### Azure AKS

For AKS, configure OIDC through Azure AD integration or use a self-managed cluster.

## Verification

After configuring the API server, verify OIDC is enabled:

```bash
# Check API server logs for OIDC configuration
kubectl logs -n kube-system kube-apiserver-<node-name> | grep oidc

# You should see lines indicating OIDC is configured:
# "Using oidc issuer url: https://dex.homelab.local"
```

## Client Configuration

Users need to configure kubectl to use OIDC authentication. Install the kubelogin plugin:

```bash
# Install kubelogin (oidc-login)
kubectl krew install oidc-login
```

Configure kubectl context:

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

## Testing Authentication

Test OIDC authentication:

```bash
# This should open a browser for authentication
kubectl get pods

# If successful, you'll be prompted to authenticate via Dex
# After authentication, kubectl commands will work with your OIDC identity
```

## Troubleshooting

### API Server Can't Reach Dex

If the API server can't reach Dex, ensure:
- Dex service is running: `kubectl get pods -n auth-system`
- Dex is accessible from the API server node
- DNS resolution works for `dex.homelab.local`
- Firewall rules allow traffic on port 5556

### Certificate Errors

If using self-signed certificates:
1. Export the Dex CA certificate
2. Add it to the API server's trusted certificates
3. Specify `--oidc-ca-file` flag pointing to the CA certificate

### Authentication Fails

Check:
- Dex logs: `kubectl logs -n auth-system deployment/dex`
- API server logs for OIDC errors
- Verify the client ID and secret match between Dex config and kubectl config
- Ensure the issuer URL is exactly the same in all configurations

## Security Considerations

- Use HTTPS for the Dex issuer URL in production
- Rotate the client secret regularly
- Limit OIDC group claims to only necessary groups
- Monitor API server logs for authentication failures
- Consider using short-lived tokens with refresh tokens

## References

- [Kubernetes OIDC Authentication](https://kubernetes.io/docs/reference/access-authn-authz/authentication/#openid-connect-tokens)
- [Dex Documentation](https://dexidp.io/docs/)
- [kubelogin Plugin](https://github.com/int128/kubelogin)
