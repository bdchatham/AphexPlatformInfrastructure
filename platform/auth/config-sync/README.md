# Configuration Sync Job

## Overview

The Configuration Sync Job orchestrates the integration between Authentik and Dex by performing cross-system configuration that cannot be handled declaratively through Blueprints alone. This Job provides deterministic, observable, and retryable convergence without relying on timing-based hacks.

## Purpose

The Job solves a critical bootstrapping problem: Dex needs a client secret to connect to Authentik, but that secret must be stored in Authentik's OIDC provider configuration. The Job:

1. Waits for Authentik to be fully ready
2. Reads the Dex client secret from Kubernetes Secrets
3. Updates Authentik's OIDC provider with the client secret via API
4. Verifies Authentik's OIDC discovery endpoint is working
5. Scales Dex from 0 to 1 replica (starts Dex now that Authentik is ready)
6. Waits for Dex to be fully ready
7. Verifies Dex's OIDC discovery endpoint is working

## Why a Job Instead of Blueprint-Only?

**Deterministic**: The Job waits for actual readiness signals (HTTP 200 responses) rather than arbitrary sleep periods.

**Observable**: Job status in Kubernetes clearly shows success or failure. Logs provide detailed progress information.

**Retryable**: Failed jobs can be safely re-run. All operations are idempotent.

**No timing hacks**: No "sleep 45 seconds and hope things converge" approaches.

**Avoids CrashLoopBackOff**: Dex starts with replicas=0 and only scales to 1 after Authentik is properly configured.

**Auditable**: Job logs show exactly what configuration was applied and when.

## Workflow

```
┌─────────────────────────────────────────────────────────────────┐
│ Bootstrap Script                                                 │
├─────────────────────────────────────────────────────────────────┤
│ 1. Deploy PostgreSQL                                             │
│ 2. Deploy Authentik with Blueprints (creates OIDC provider      │
│    structure without client secret)                              │
│ 3. Create Authentik API token                                    │
│ 4. Generate Dex client secret                                    │
│ 5. Store secrets in Kubernetes Secrets                           │
│ 6. Deploy Dex with replicas=0 (does not start)                  │
│ 7. Deploy Ingress resources                                      │
│ 8. Create and run Configuration Sync Job                         │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ Configuration Sync Job                                           │
├─────────────────────────────────────────────────────────────────┤
│ [1/9] Read secrets from mounted volumes                          │
│       - Dex client secret                                        │
│       - Authentik API token                                      │
│                                                                  │
│ [2/9] Wait for Authentik health check (200)                     │
│       - Polls /api/v3/root/config/ endpoint                      │
│       - Max 60 attempts, 5 seconds between attempts              │
│                                                                  │
│ [3/9] Lookup OIDC provider in Authentik                         │
│       - Fetch all OAuth2 providers via API                       │
│       - Find provider matching:                                  │
│         * name = "Dex OIDC Provider"                             │
│         * client_id = "dex-client"                               │
│         * redirect_uris contains "https://dex.home.local/callback"│
│       - Fail if 0 or >1 matches found                            │
│                                                                  │
│ [4/9] Update OIDC provider with client secret                   │
│       - PATCH /api/v3/providers/oauth2/{id}/                     │
│       - Set client_secret field                                  │
│                                                                  │
│ [5/9] Verify Authentik OIDC discovery endpoint                  │
│       - GET /application/o/dex/.well-known/openid-configuration  │
│       - Verify HTTP 200 response                                 │
│       - Verify issuer = "https://auth.home.local/application/o/dex/"│
│                                                                  │
│ [6/9] Scale Dex deployment to 1 replica                         │
│       - kubectl scale deployment/dex --replicas=1                │
│       - Idempotent: no-op if already scaled                      │
│                                                                  │
│ [7/9] Wait for Dex health check (200)                           │
│       - Polls /healthz endpoint                                  │
│       - Max 60 attempts, 5 seconds between attempts              │
│                                                                  │
│ [8/9] Verify Dex OIDC discovery endpoint                        │
│       - GET /.well-known/openid-configuration                    │
│       - Verify HTTP 200 response                                 │
│       - Verify issuer = "https://dex.home.local"                 │
│                                                                  │
│ [9/9] Configuration sync complete                               │
│       - Authentication system is fully operational               │
└─────────────────────────────────────────────────────────────────┘
```

## Deterministic Convergence Approach

The Job uses **deterministic convergence** rather than timing-based hacks:

**What we DON'T do:**
- ❌ Sleep for arbitrary periods hoping things converge
- ❌ Assume services are ready after a fixed delay
- ❌ Retry indefinitely without clear failure conditions
- ❌ Ignore errors and hope for the best

**What we DO:**
- ✅ Wait for actual readiness signals (HTTP 200 responses)
- ✅ Verify configuration at each step before proceeding
- ✅ Fail fast with clear error messages if something is wrong
- ✅ Make all operations idempotent and safe to re-run
- ✅ Provide detailed logs showing exactly what happened

## Idempotency Guarantees

The Job is designed to be safely re-runnable:

**Idempotent operations:**
- Reading secrets: Always reads current values from Kubernetes
- Waiting for Authentik: Polls until ready, no side effects
- Looking up OIDC provider: Read-only API call
- Updating OIDC provider: PATCH is idempotent (sets to desired state)
- Verifying discovery endpoints: Read-only HTTP GET
- Scaling Dex: kubectl scale is idempotent (no-op if already at desired replicas)
- Waiting for Dex: Polls until ready, no side effects

**Safe to re-run when:**
- Job failed due to transient network issues
- Job failed because Authentik wasn't ready yet
- Job failed because OIDC provider wasn't created by Blueprint
- You want to verify the configuration is still correct
- You manually changed something and want to re-sync

**Not safe to re-run when:**
- You've manually changed the OIDC provider in Authentik UI and want to keep those changes (Job will overwrite with Dex client secret)

## Manual Triggering

### Check Current Job Status

```bash
# Check if Job exists and its status
kubectl get job auth-config-sync -n auth-system

# Check Job pod status
kubectl get pods -n auth-system -l app=auth-config-sync

# View Job logs
kubectl logs -n auth-system -l app=auth-config-sync
```

### Re-run the Job

To manually trigger the Job (e.g., after fixing a configuration issue):

```bash
# Delete the existing Job (if it exists)
kubectl delete job auth-config-sync -n auth-system

# Re-create the Job
kubectl apply -f platform/auth/config-sync/job.yaml
kubectl apply -f platform/auth/config-sync/configmap.yaml

# Watch the Job progress
kubectl logs -n auth-system -l app=auth-config-sync -f
```

### Check Job Completion

```bash
# Check if Job completed successfully
kubectl get job auth-config-sync -n auth-system -o jsonpath='{.status.conditions[?(@.type=="Complete")].status}'
# Should output: True

# Check if Job failed
kubectl get job auth-config-sync -n auth-system -o jsonpath='{.status.conditions[?(@.type=="Failed")].status}'
# Should output: (empty) or False

# Get detailed Job status
kubectl describe job auth-config-sync -n auth-system
```

## Troubleshooting

### Job Fails at Step 2: Waiting for Authentik

**Symptom:** Job logs show "Authentik not ready, waiting..." repeatedly, then fails.

**Possible causes:**
- Authentik pod is not running
- Authentik is crashing or in CrashLoopBackOff
- PostgreSQL is not available
- Network connectivity issues

**Diagnosis:**
```bash
# Check Authentik pod status
kubectl get pods -n auth-system -l app=authentik

# Check Authentik logs
kubectl logs -n auth-system -l app=authentik

# Check PostgreSQL status
kubectl get pods -n auth-system -l app=postgresql

# Test Authentik health endpoint manually
kubectl run -it --rm debug --image=curlimages/curl --restart=Never -- \
  curl -v http://authentik.auth-system.svc.cluster.local:9000/api/v3/root/config/
```

**Resolution:**
- Fix Authentik deployment issues
- Ensure PostgreSQL is running and healthy
- Re-run the Job after fixing issues

### Job Fails at Step 3: OIDC Provider Not Found

**Symptom:** Job logs show "ERROR: No OIDC provider found matching criteria"

**Possible causes:**
- Authentik Blueprint did not create the OIDC provider
- OIDC provider was created with wrong name, client_id, or redirect_uris
- Blueprint was not applied (Authentik didn't auto-apply it)

**Diagnosis:**
```bash
# Check if Blueprint ConfigMap exists
kubectl get configmap authentik-blueprints -n auth-system

# Check Authentik logs for Blueprint application
kubectl logs -n auth-system -l app=authentik | grep -i blueprint

# Check OIDC providers in Authentik via API
kubectl run -it --rm debug --image=curlimages/curl --restart=Never -- \
  curl -H "Authorization: Bearer $(kubectl get secret authentik-api-token -n auth-system -o jsonpath='{.data.token}' | base64 -d)" \
  http://authentik.auth-system.svc.cluster.local:9000/api/v3/providers/oauth2/
```

**Resolution:**
- Verify Blueprint ConfigMap is correctly mounted in Authentik deployment
- Check Blueprint YAML syntax and content
- Manually create OIDC provider in Authentik UI if needed
- Re-run the Job after fixing Blueprint

### Job Fails at Step 3: Multiple OIDC Providers Found

**Symptom:** Job logs show "ERROR: Multiple OIDC providers found matching criteria"

**Possible causes:**
- Blueprint was applied multiple times, creating duplicates
- Manual OIDC provider creation in Authentik UI created duplicates

**Diagnosis:**
```bash
# List all OIDC providers
kubectl run -it --rm debug --image=curlimages/curl --restart=Never -- \
  curl -H "Authorization: Bearer $(kubectl get secret authentik-api-token -n auth-system -o jsonpath='{.data.token}' | base64 -d)" \
  http://authentik.auth-system.svc.cluster.local:9000/api/v3/providers/oauth2/
```

**Resolution:**
- Log into Authentik UI
- Navigate to Admin Interface → Applications → Providers
- Delete duplicate "Dex OIDC Provider" entries
- Keep only one provider with correct configuration
- Re-run the Job

### Job Fails at Step 4: Failed to Update OIDC Provider

**Symptom:** Job logs show "ERROR: Failed to update OIDC provider (HTTP 403)" or similar

**Possible causes:**
- Authentik API token has insufficient permissions
- Authentik API token is invalid or expired
- OIDC provider is read-only or protected

**Diagnosis:**
```bash
# Verify API token exists
kubectl get secret authentik-api-token -n auth-system

# Test API token manually
kubectl run -it --rm debug --image=curlimages/curl --restart=Never -- \
  curl -H "Authorization: Bearer $(kubectl get secret authentik-api-token -n auth-system -o jsonpath='{.data.token}' | base64 -d)" \
  http://authentik.auth-system.svc.cluster.local:9000/api/v3/core/users/me/
```

**Resolution:**
- Recreate Authentik API token with correct permissions
- Ensure token has `authentik_providers_oauth2.change_oauth2provider` permission
- Update `authentik-api-token` Secret with new token
- Re-run the Job

### Job Fails at Step 5: Authentik OIDC Discovery Endpoint Error

**Symptom:** Job logs show "ERROR: Authentik OIDC discovery endpoint returned HTTP 404" or issuer mismatch

**Possible causes:**
- OIDC provider application is not properly configured
- Authentik application slug is wrong
- Issuer URL in Blueprint doesn't match expected value

**Diagnosis:**
```bash
# Test discovery endpoint manually
kubectl run -it --rm debug --image=curlimages/curl --restart=Never -- \
  curl -v http://authentik.auth-system.svc.cluster.local:9000/application/o/dex/.well-known/openid-configuration

# Check Authentik application configuration
# (requires logging into Authentik UI)
```

**Resolution:**
- Verify Authentik application slug is "dex"
- Verify OIDC provider is linked to application
- Check Blueprint application configuration
- Re-run the Job after fixing configuration

### Job Fails at Step 6: Failed to Scale Dex

**Symptom:** Job logs show "ERROR: Failed to scale Dex deployment"

**Possible causes:**
- Insufficient RBAC permissions for Job ServiceAccount
- Dex deployment doesn't exist
- Kubernetes API server issues

**Diagnosis:**
```bash
# Check if Dex deployment exists
kubectl get deployment dex -n auth-system

# Check Job ServiceAccount permissions
kubectl auth can-i update deployments/scale --as=system:serviceaccount:auth-system:auth-config-sync -n auth-system

# Check Job pod logs for detailed error
kubectl logs -n auth-system -l app=auth-config-sync
```

**Resolution:**
- Verify Dex deployment exists
- Verify RBAC Role and RoleBinding are correctly configured
- Re-apply RBAC manifests if needed
- Re-run the Job

### Job Fails at Step 7: Dex Not Ready

**Symptom:** Job logs show "Dex not ready, waiting..." repeatedly, then fails

**Possible causes:**
- Dex pod is crashing or in CrashLoopBackOff
- Dex configuration is invalid
- Dex cannot connect to Authentik
- Network connectivity issues

**Diagnosis:**
```bash
# Check Dex pod status
kubectl get pods -n auth-system -l app=dex

# Check Dex logs
kubectl logs -n auth-system -l app=dex

# Test Dex health endpoint manually
kubectl run -it --rm debug --image=curlimages/curl --restart=Never -- \
  curl -v http://dex.auth-system.svc.cluster.local:5556/healthz
```

**Resolution:**
- Check Dex logs for configuration errors
- Verify Dex ConfigMap has correct Authentik connector configuration
- Ensure Authentik is accessible from Dex pod
- Fix Dex configuration and re-run the Job

### Job Fails at Step 8: Dex OIDC Discovery Endpoint Error

**Symptom:** Job logs show "ERROR: Dex OIDC discovery endpoint returned HTTP 500" or issuer mismatch

**Possible causes:**
- Dex cannot connect to Authentik OIDC discovery endpoint
- Dex configuration has wrong issuer URL
- Network connectivity issues between Dex and Authentik

**Diagnosis:**
```bash
# Test Dex discovery endpoint manually
kubectl run -it --rm debug --image=curlimages/curl --restart=Never -- \
  curl -v http://dex.auth-system.svc.cluster.local:5556/.well-known/openid-configuration

# Check Dex logs for connector errors
kubectl logs -n auth-system -l app=dex | grep -i authentik
```

**Resolution:**
- Verify Dex ConfigMap issuer matches expected value
- Verify Dex can reach Authentik OIDC discovery endpoint
- Check Dex connector configuration
- Re-run the Job after fixing configuration

## Security Considerations

### Secret Handling

The Job uses **projected volumes** to mount secrets, which provides several security benefits:

- **No kubectl get secret permissions needed**: Job ServiceAccount doesn't need permission to read secrets via kubectl
- **Read-only access**: Secrets are mounted read-only, preventing accidental modification
- **Automatic updates**: If secrets are rotated, new Job runs will automatically use new values
- **Minimal RBAC**: Job only needs permissions to scale Deployments, nothing else

### RBAC Permissions

The Job ServiceAccount has **minimal RBAC permissions**:

```yaml
rules:
  # Only permission to scale the Dex deployment
  - apiGroups: ["apps"]
    resources: ["deployments/scale"]
    resourceNames: ["dex"]
    verbs: ["get", "patch", "update"]
  
  # Only permission to get Dex deployment status
  - apiGroups: ["apps"]
    resources: ["deployments"]
    resourceNames: ["dex"]
    verbs: ["get"]
```

**What the Job CANNOT do:**
- Read or modify other Deployments
- Read or modify Secrets
- Read or modify ConfigMaps
- Access other namespaces
- Perform cluster-wide operations

### API Token Permissions

The Authentik API token should have **minimal permissions**:

- Only `authentik_providers_oauth2.change_oauth2provider` permission
- No admin or superuser permissions
- Long-lived but rotatable via Authentik UI

## Files

- `job.yaml`: Kubernetes Job manifest
- `configmap.yaml`: ConfigMap containing the sync script
- `serviceaccount.yaml`: ServiceAccount for the Job
- `rbac.yaml`: Role and RoleBinding with minimal permissions
- `sync-script.sh`: Standalone sync script (for reference, actual script is in ConfigMap)
- `README.md`: This documentation

## Related Documentation

- [Authentik Documentation](../authentik/README.md)
- [Dex Documentation](../dex/README.md)
- [Secret Management](../secrets/README.md)
- [Bootstrap Process](../../bootstrap/README.md)
