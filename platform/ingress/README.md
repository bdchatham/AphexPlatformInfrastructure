# Ingress Resources

This directory contains Ingress resources for exposing authentication system services to the home network. Ingress resources enable access to services via real hostnames that user browsers can reach, which is required for OIDC authentication flows.

## Overview

The authentication system exposes four services via Ingress:

- **Authentik**: `https://auth.home.local` - Identity Provider web UI
- **Dex**: `https://dex.home.local` - OIDC connector for platform services
- **ArgoCD**: `https://argocd.home.local` - GitOps continuous delivery UI
- **Tekton Dashboard**: `https://tekton.home.local` - CI/CD pipeline UI

All services use HTTPS with TLS certificates (self-signed or Let's Encrypt).

## Manifests

- `authentik-ingress.yaml`: Ingress for Authentik IdP
- `dex-ingress.yaml`: Ingress for Dex OIDC connector
- `argocd-ingress.yaml`: Ingress for ArgoCD UI
- `tekton-ingress.yaml`: Ingress for Tekton Dashboard

## Ingress Controller Requirements

**An Ingress controller must be deployed before the authentication system.** The Ingress controller is responsible for:

- Routing HTTP/HTTPS traffic to services based on hostnames
- TLS termination (decrypting HTTPS traffic)
- Load balancing across service pods

### Supported Ingress Controllers

**nginx-ingress (Recommended for homelab)**:
```bash
# For Kind clusters
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml

# For bare-metal clusters
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/baremetal/deploy.yaml

# Wait for controller to be ready
kubectl wait --namespace ingress-nginx \
  --for=condition=ready pod \
  --selector=app.kubernetes.io/component=controller \
  --timeout=90s
```

**Traefik**:
```bash
helm repo add traefik https://traefik.github.io/charts
helm repo update
helm install traefik traefik/traefik -n traefik --create-namespace
```

### Verifying Ingress Controller

```bash
# For nginx-ingress
kubectl get pods -n ingress-nginx
kubectl get svc -n ingress-nginx

# For traefik
kubectl get pods -n traefik
kubectl get svc -n traefik

# Check for LoadBalancer or NodePort service
# This is the IP address you'll use for DNS configuration
```

## DNS Configuration

**DNS configuration is required** for user browsers to reach services via hostnames. You must configure DNS so that `*.home.local` (or your chosen domain) resolves to your Ingress controller's IP address.

### Finding Your Ingress Controller IP

**For LoadBalancer service** (cloud providers, MetalLB):
```bash
# nginx-ingress
kubectl get svc -n ingress-nginx ingress-nginx-controller

# traefik
kubectl get svc -n traefik traefik

# Look for EXTERNAL-IP column
# Example output:
# NAME                       TYPE           EXTERNAL-IP     PORT(S)
# ingress-nginx-controller   LoadBalancer   192.168.1.100   80:30080/TCP,443:30443/TCP
```

**For NodePort service** (Kind, bare-metal without LoadBalancer):
```bash
# Get any node IP
kubectl get nodes -o wide

# Use node IP with NodePort
# Example: If node IP is 192.168.1.50 and NodePort is 30080/30443
# Use 192.168.1.50 for DNS configuration
```

**For Kind clusters** (local development):
```bash
# Kind exposes ports on localhost
# Use 127.0.0.1 or your machine's IP for DNS configuration
```

### DNS Configuration Options

#### Option 1: Router/Pi-hole DNS (Recommended)

Configure DNS records in your home router or Pi-hole:

**A Records**:
```
auth.home.local     → 192.168.1.100
dex.home.local      → 192.168.1.100
argocd.home.local   → 192.168.1.100
tekton.home.local   → 192.168.1.100
```

Replace `192.168.1.100` with your Ingress controller's IP address.

**Advantages**:
- Works for all devices on home network
- No per-device configuration required
- Persistent across device reboots

**Configuration steps**:
1. Login to your router's admin interface (usually `192.168.1.1` or `192.168.0.1`)
2. Navigate to DNS settings or DHCP settings
3. Add custom DNS entries (exact location varies by router)
4. Save and restart router if required

**For Pi-hole**:
1. Login to Pi-hole admin interface
2. Navigate to **Local DNS** → **DNS Records**
3. Add A records for each hostname
4. Click **Add**

#### Option 2: Hosts File (Per-Device)

Add entries to `/etc/hosts` on each device that needs access:

**Linux/macOS**:
```bash
sudo nano /etc/hosts

# Add these lines:
192.168.1.100 auth.home.local
192.168.1.100 dex.home.local
192.168.1.100 argocd.home.local
192.168.1.100 tekton.home.local
```

**Windows**:
```powershell
# Run as Administrator
notepad C:\Windows\System32\drivers\etc\hosts

# Add these lines:
192.168.1.100 auth.home.local
192.168.1.100 dex.home.local
192.168.1.100 argocd.home.local
192.168.1.100 tekton.home.local
```

**Advantages**:
- No router configuration required
- Works immediately
- No external dependencies

**Disadvantages**:
- Must configure each device separately
- Requires admin/root access
- Not persistent across OS reinstalls

#### Option 3: mDNS/Avahi (Automatic Discovery)

Use `.local` domain with mDNS for automatic discovery:

**Requirements**:
- macOS: Built-in (Bonjour)
- Linux: Install Avahi (`sudo apt install avahi-daemon`)
- Windows: Install Bonjour Print Services

**Configuration**:
- No manual DNS configuration required
- Services advertise themselves via mDNS
- Works automatically on local network

**Limitations**:
- Only works on local network
- May not work across VLANs
- Requires mDNS support on all devices

### Verifying DNS Configuration

```bash
# Test DNS resolution
nslookup auth.home.local
nslookup dex.home.local
nslookup argocd.home.local
nslookup tekton.home.local

# Expected output:
# Name:   auth.home.local
# Address: 192.168.1.100

# Test HTTP connectivity
curl -k https://auth.home.local
curl -k https://dex.home.local/.well-known/openid-configuration
```

## TLS Certificate Options

All services use HTTPS with TLS certificates. You can choose between self-signed certificates (simplest) or Let's Encrypt (trusted certificates).

### Option 1: Self-Signed Certificates (Simplest)

**Advantages**:
- No external dependencies
- Works immediately
- No domain ownership required
- No rate limits

**Disadvantages**:
- Browser certificate warnings
- Users must manually accept certificates
- Not trusted by default

**Setup**:

1. **Install cert-manager**:
   ```bash
   kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml
   
   # Wait for cert-manager to be ready
   kubectl wait --for=condition=ready pod \
     -l app.kubernetes.io/instance=cert-manager \
     -n cert-manager \
     --timeout=90s
   ```

2. **Create self-signed ClusterIssuer**:
   ```bash
   cat <<EOF | kubectl apply -f -
   apiVersion: cert-manager.io/v1
   kind: ClusterIssuer
   metadata:
     name: selfsigned-issuer
   spec:
     selfSigned: {}
   EOF
   ```

3. **Ingress resources reference this issuer**:
   ```yaml
   metadata:
     annotations:
       cert-manager.io/cluster-issuer: "selfsigned-issuer"
   ```

4. **Accept certificate warnings in browser**:
   - Chrome: Click "Advanced" → "Proceed to auth.home.local (unsafe)"
   - Firefox: Click "Advanced" → "Accept the Risk and Continue"
   - Safari: Click "Show Details" → "visit this website"

### Option 2: Let's Encrypt with DNS-01 Challenge

**Advantages**:
- Trusted certificates (no browser warnings)
- Automatic renewal
- Works even if cluster is not publicly accessible

**Disadvantages**:
- Requires DNS provider API access
- More complex setup
- Rate limits (50 certificates per week)

**Requirements**:
- Domain name (e.g., `example.com`)
- DNS provider with API support (Cloudflare, Route53, Google Cloud DNS, etc.)
- API credentials for DNS provider

**Setup (Cloudflare example)**:

1. **Install cert-manager** (same as Option 1)

2. **Create Cloudflare API token**:
   - Login to Cloudflare dashboard
   - Navigate to **My Profile** → **API Tokens**
   - Click **Create Token**
   - Use **Edit zone DNS** template
   - Select your domain
   - Copy token

3. **Create Cloudflare API token secret**:
   ```bash
   kubectl create secret generic cloudflare-api-token \
     -n cert-manager \
     --from-literal=api-token=YOUR_TOKEN_HERE
   ```

4. **Create Let's Encrypt ClusterIssuer**:
   ```bash
   cat <<EOF | kubectl apply -f -
   apiVersion: cert-manager.io/v1
   kind: ClusterIssuer
   metadata:
     name: letsencrypt-dns
   spec:
     acme:
       server: https://acme-v02.api.letsencrypt.org/directory
       email: your-email@example.com
       privateKeySecretRef:
         name: letsencrypt-dns-key
       solvers:
         - dns01:
             cloudflare:
               apiTokenSecretRef:
                 name: cloudflare-api-token
                 key: api-token
   EOF
   ```

5. **Update Ingress annotations**:
   ```yaml
   metadata:
     annotations:
       cert-manager.io/cluster-issuer: "letsencrypt-dns"
   ```

6. **Update hostnames in Ingress resources**:
   ```yaml
   spec:
     tls:
       - hosts:
           - auth.example.com  # Replace with your domain
         secretName: authentik-tls
     rules:
       - host: auth.example.com  # Replace with your domain
   ```

7. **Commit and push to Git**:
   - ArgoCD syncs changes automatically
   - cert-manager requests certificates from Let's Encrypt
   - Certificates are automatically renewed before expiration

**For other DNS providers**, see cert-manager documentation: https://cert-manager.io/docs/configuration/acme/dns01/

### Verifying TLS Certificates

```bash
# Check certificate status
kubectl get certificate -n auth-system

# Check certificate details
kubectl describe certificate authentik-tls -n auth-system

# Test HTTPS connection
curl -v https://auth.home.local 2>&1 | grep -i "subject\|issuer"

# For self-signed:
# subject: CN=auth.home.local
# issuer: CN=auth.home.local

# For Let's Encrypt:
# subject: CN=auth.example.com
# issuer: CN=R3, O=Let's Encrypt
```

## Example `/etc/hosts` Entries

For quick testing or when router DNS configuration is not available:

```bash
# Replace 192.168.1.100 with your Ingress controller IP
192.168.1.100 auth.home.local
192.168.1.100 dex.home.local
192.168.1.100 argocd.home.local
192.168.1.100 tekton.home.local
```

**To find your Ingress controller IP**:
```bash
# For LoadBalancer
kubectl get svc -n ingress-nginx ingress-nginx-controller -o jsonpath='{.status.loadBalancer.ingress[0].ip}'

# For NodePort (use any node IP)
kubectl get nodes -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}'
```

## Customizing Hostnames

To use different hostnames (e.g., `auth.example.com` instead of `auth.home.local`):

1. **Update Ingress resources**:
   - Edit `authentik-ingress.yaml`, `dex-ingress.yaml`, etc.
   - Replace `home.local` with your domain
   - Commit and push to Git

2. **Update Dex configuration**:
   - Edit `platform/auth/dex/configmap.yaml`
   - Update `issuer` URL
   - Update `redirectURIs` for all clients
   - Commit and push to Git

3. **Update Authentik Blueprints**:
   - Edit `platform/auth/authentik/blueprints-configmap.yaml`
   - Update `redirect_uris` in OIDC provider
   - Commit and push to Git

4. **Update ArgoCD and Tekton configurations**:
   - Edit `platform/integrations/argocd-oidc-config.yaml`
   - Edit `platform/integrations/tekton-dashboard-oidc.yaml`
   - Update all URLs to use new domain
   - Commit and push to Git

5. **ArgoCD syncs changes automatically**

6. **Update DNS configuration** to point new hostnames to Ingress controller IP

## Common Ingress Troubleshooting

### Service Not Accessible

**Symptom**: Browser shows "This site can't be reached" or "Connection refused"

**Possible causes**:
1. DNS not configured
2. Ingress controller not running
3. Service not ready

**Resolution**:
```bash
# Check DNS resolution
nslookup auth.home.local

# Check Ingress controller is running
kubectl get pods -n ingress-nginx

# Check Ingress resource exists
kubectl get ingress -n auth-system

# Check service exists and has endpoints
kubectl get svc -n auth-system
kubectl get endpoints -n auth-system

# Check Ingress controller logs
kubectl logs -n ingress-nginx deployment/ingress-nginx-controller
```

### Certificate Errors

**Symptom**: Browser shows "Your connection is not private" or "NET::ERR_CERT_AUTHORITY_INVALID"

**For self-signed certificates**:
- This is expected behavior
- Click "Advanced" and proceed to site
- Or add certificate to browser's trusted certificates

**For Let's Encrypt certificates**:
```bash
# Check certificate status
kubectl get certificate -n auth-system

# Check cert-manager logs
kubectl logs -n cert-manager deployment/cert-manager

# Check certificate details
kubectl describe certificate authentik-tls -n auth-system

# Common issues:
# - DNS-01 challenge failed (check DNS provider API credentials)
# - Rate limit exceeded (wait 1 hour and try again)
# - Domain validation failed (check domain ownership)
```

### 404 Not Found

**Symptom**: Browser shows "404 Not Found" or "default backend - 404"

**Possible causes**:
1. Ingress path doesn't match service
2. Service selector doesn't match pods
3. Backend service not ready

**Resolution**:
```bash
# Check Ingress configuration
kubectl describe ingress authentik -n auth-system

# Check service selector matches pods
kubectl get svc authentik -n auth-system -o yaml | grep selector
kubectl get pods -n auth-system -l app=authentik

# Check service endpoints
kubectl get endpoints authentik -n auth-system

# Check pod logs
kubectl logs -n auth-system deployment/authentik-server
```

### Redirect Loop

**Symptom**: Browser shows "Too many redirects" or keeps redirecting

**Possible causes**:
1. Ingress and service both doing TLS termination
2. Incorrect redirect URI configuration
3. OIDC configuration mismatch

**Resolution**:
```bash
# Check Ingress TLS configuration
kubectl get ingress authentik -n auth-system -o yaml | grep -A 5 tls

# Check service is using HTTP (not HTTPS)
kubectl get svc authentik -n auth-system -o yaml | grep port

# For ArgoCD, check if SSL passthrough is needed
# Add annotation to argocd-ingress.yaml:
# nginx.ingress.kubernetes.io/ssl-passthrough: "true"
```

### Ingress Controller Not Getting IP

**Symptom**: Ingress controller service shows `<pending>` for EXTERNAL-IP

**For cloud providers**:
- LoadBalancer provisioning may take a few minutes
- Check cloud provider console for load balancer status

**For bare-metal/homelab**:
- Install MetalLB for LoadBalancer support:
  ```bash
  kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/v0.13.12/config/manifests/metallb-native.yaml
  
  # Configure IP address pool
  cat <<EOF | kubectl apply -f -
  apiVersion: metallb.io/v1beta1
  kind: IPAddressPool
  metadata:
    name: default
    namespace: metallb-system
  spec:
    addresses:
      - 192.168.1.200-192.168.1.250
  ---
  apiVersion: metallb.io/v1beta1
  kind: L2Advertisement
  metadata:
    name: default
    namespace: metallb-system
  EOF
  ```

- Or use NodePort instead of LoadBalancer:
  ```bash
  # Edit Ingress controller service
  kubectl edit svc -n ingress-nginx ingress-nginx-controller
  
  # Change type from LoadBalancer to NodePort
  # Use node IP + NodePort for DNS configuration
  ```

## Security Considerations

### TLS Best Practices

- **Always use HTTPS** for authentication services (required for OIDC)
- **Use Let's Encrypt** for production environments (trusted certificates)
- **Use self-signed** only for homelab/dev environments
- **Enable HSTS** (HTTP Strict Transport Security) for production:
  ```yaml
  metadata:
    annotations:
      nginx.ingress.kubernetes.io/force-ssl-redirect: "true"
      nginx.ingress.kubernetes.io/hsts: "true"
      nginx.ingress.kubernetes.io/hsts-max-age: "31536000"
  ```

### Network Security

- **Restrict Ingress to home network** (if possible):
  ```yaml
  metadata:
    annotations:
      nginx.ingress.kubernetes.io/whitelist-source-range: "192.168.1.0/24"
  ```

- **Use strong TLS ciphers**:
  ```yaml
  metadata:
    annotations:
      nginx.ingress.kubernetes.io/ssl-ciphers: "ECDHE-RSA-AES128-GCM-SHA256:ECDHE-RSA-AES256-GCM-SHA384"
      nginx.ingress.kubernetes.io/ssl-protocols: "TLSv1.2 TLSv1.3"
  ```

### Rate Limiting

Protect against brute-force attacks:

```yaml
metadata:
  annotations:
    nginx.ingress.kubernetes.io/limit-rps: "10"
    nginx.ingress.kubernetes.io/limit-connections: "5"
```

## References

- **nginx-ingress Documentation**: https://kubernetes.github.io/ingress-nginx/
- **Traefik Documentation**: https://doc.traefik.io/traefik/
- **cert-manager Documentation**: https://cert-manager.io/docs/
- **Let's Encrypt**: https://letsencrypt.org/docs/
- **Requirements**: `.kiro/specs/dex-authentication-platform/requirements.md` (Requirement 12.4, 17.5)
- **Design**: `.kiro/specs/dex-authentication-platform/design.md` (External URLs and Redirect URIs section)
