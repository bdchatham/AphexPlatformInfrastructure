# Lighthouse Configuration

This directory contains Lighthouse and GitHub App integration configuration.

## Contents

- `lighthouse-config.yaml` - Lighthouse configuration ConfigMap
- `repo-allowlist.yaml` - Repository allowlist ConfigMap (initially empty)

## Purpose

Configures Git event handling and webhook processing for the platform. Lighthouse receives webhook events from the GitHub App and triggers Tekton PipelineRuns based on the repository allowlist.

## Configuration Files

### lighthouse-config.yaml

Main Lighthouse configuration including:
- GitHub App credentials (references lighthouse-github-app secret)
- Trigger rules (push events on main/master branches)
- Plank configuration (PipelineRun creation settings)
- Resource limits and timeouts
- Logging and metrics configuration

**Note**: The GitHub App credentials (app_id, app_installation_id, webhook_secret) are referenced from the `lighthouse-github-app` secret. You must create this secret before deploying Lighthouse.

### repo-allowlist.yaml

Repository allowlist that controls which repositories can trigger pipelines. Initially empty - repositories are added by the onboarding controller when RepoBindings are created.

Format:
```yaml
repos:
  - org: "github-org-name"
    name: "repository-name"
    tenant: "tenant-namespace-name"
    enabled: true
```

## Deployment

1. Create the GitHub App secret (see `platform/bootstrap/github-app-setup.md`):
   ```bash
   ./platform/bootstrap/create-github-app-secret.sh
   ```

2. Apply the Lighthouse configuration:
   ```bash
   kubectl apply -f platform/lighthouse/lighthouse-config.yaml
   kubectl apply -f platform/lighthouse/repo-allowlist.yaml
   ```

3. Install Lighthouse via Helm (see task 6.5):
   ```bash
   helm install lighthouse jx3/lighthouse -n pipeline-system \
     --set github.appId="${GITHUB_APP_ID}" \
     --set github.appInstallationId="${GITHUB_APP_INSTALLATION_ID}"
   ```

## Verification

Check Lighthouse configuration:
```bash
# View Lighthouse config
kubectl get configmap lighthouse-config -n pipeline-system -o yaml

# View allowlist
kubectl get configmap repo-allowlist -n pipeline-system -o yaml

# Check Lighthouse logs
kubectl logs -n pipeline-system deployment/lighthouse -c lighthouse
```

## Updating Configuration

### Updating Lighthouse Config

Edit the ConfigMap and apply:
```bash
kubectl apply -f platform/lighthouse/lighthouse-config.yaml
```

Lighthouse will automatically reload the configuration.

### Updating Allowlist

The allowlist is managed by the onboarding controller. Manual updates:
```bash
kubectl edit configmap repo-allowlist -n pipeline-system
```

Lighthouse will automatically reload the allowlist when it changes.

## Troubleshooting

### Lighthouse Not Receiving Events

1. Check GitHub App webhook deliveries in GitHub settings
2. Verify webhook URL is accessible from GitHub
3. Check webhook secret matches:
   ```bash
   kubectl get secret lighthouse-github-app -n pipeline-system -o jsonpath='{.data.webhook-secret}' | base64 -d
   ```

### Events Not Triggering Pipelines

1. Check repository is in allowlist:
   ```bash
   kubectl get configmap repo-allowlist -n pipeline-system -o yaml | grep -A 3 "your-repo-name"
   ```

2. Check Lighthouse logs for allowlist rejections:
   ```bash
   kubectl logs -n pipeline-system deployment/lighthouse -c lighthouse | grep "allowlist"
   ```

3. Verify tenant namespace exists:
   ```bash
   kubectl get namespace YOUR_TENANT_NAMESPACE
   ```

## References

- [Lighthouse Documentation](https://github.com/jenkins-x/lighthouse)
- [GitHub App Setup Guide](../bootstrap/github-app-setup.md)
# Test commit to trigger webhook - Fri Jan  2 09:52:46 PST 2026
