# GitHub App Setup Guide

This guide walks through creating and configuring a GitHub App for the Jenkins X platform.

## Overview

The GitHub App provides webhook integration at the organization level, eliminating the need to configure webhooks for each repository individually. Lighthouse receives events from the GitHub App and triggers pipelines based on the repository allowlist.

## Prerequisites

- GitHub organization admin access
- Access to the Kubernetes cluster where Lighthouse will be deployed
- Lighthouse webhook endpoint URL (will be available after Lighthouse installation)

## Step 1: Create GitHub App

1. Navigate to your GitHub organization settings:
   - Go to `https://github.com/organizations/YOUR_ORG/settings/apps`
   - Click "New GitHub App"

2. Configure the GitHub App:

   **Basic Information:**
   - **GitHub App name**: `jenkins-x-platform` (or your preferred name)
   - **Homepage URL**: `https://github.com/YOUR_ORG`
   - **Webhook URL**: `https://YOUR_LIGHTHOUSE_ENDPOINT/hook`
     - This will be your Lighthouse ingress URL
     - Example: `https://lighthouse.example.com/hook`
   - **Webhook secret**: Generate a strong random secret
     ```bash
     openssl rand -hex 32
     ```
     - Save this secret - you'll need it for Lighthouse configuration

   **Permissions:**
   
   Repository permissions:
   - **Contents**: Read-only (to clone repositories)
   - **Metadata**: Read-only (required)
   - **Pull requests**: Read & write (to update PR status)
   - **Checks**: Read & write (to create check runs)
   - **Commit statuses**: Read & write (to update commit status)

   Organization permissions:
   - **Members**: Read-only (to verify user membership)

   **Subscribe to events:**
   - [x] Check run
   - [x] Check suite
   - [x] Pull request
   - [x] Push
   - [x] Status

3. Click "Create GitHub App"

## Step 2: Generate Private Key

1. After creating the app, scroll down to "Private keys"
2. Click "Generate a private key"
3. Save the downloaded `.pem` file securely
4. You'll need to create a Kubernetes Secret with this key

## Step 3: Install GitHub App

1. In the GitHub App settings, click "Install App" in the left sidebar
2. Select your organization
3. Choose installation scope:
   - **All repositories** (recommended for platform-wide access)
   - Or select specific repositories
4. Click "Install"
5. Note the **Installation ID** from the URL:
   - URL format: `https://github.com/organizations/YOUR_ORG/settings/installations/INSTALLATION_ID`
   - Save this Installation ID - you'll need it for Lighthouse configuration

## Step 4: Record Configuration Values

You'll need these values for Lighthouse configuration:

```bash
# GitHub App ID (found in app settings)
GITHUB_APP_ID="123456"

# GitHub App Installation ID (from installation URL)
GITHUB_APP_INSTALLATION_ID="12345678"

# Webhook secret (generated in step 1)
GITHUB_WEBHOOK_SECRET="your-webhook-secret-here"

# Private key file path
GITHUB_APP_PRIVATE_KEY_FILE="path/to/your-app.private-key.pem"
```

## Step 5: Create Kubernetes Secret

Create a Kubernetes Secret with the GitHub App credentials:

```bash
# Create secret with private key
kubectl create secret generic lighthouse-github-app \
  --from-file=private-key=${GITHUB_APP_PRIVATE_KEY_FILE} \
  --from-literal=app-id=${GITHUB_APP_ID} \
  --from-literal=installation-id=${GITHUB_APP_INSTALLATION_ID} \
  --from-literal=webhook-secret=${GITHUB_WEBHOOK_SECRET} \
  -n pipeline-system
```

Verify the secret was created:

```bash
kubectl get secret lighthouse-github-app -n pipeline-system
kubectl describe secret lighthouse-github-app -n pipeline-system
```

## Step 6: Verify GitHub App Configuration

After Lighthouse is installed and configured:

1. Check that Lighthouse can authenticate with GitHub:
   ```bash
   kubectl logs -n pipeline-system deployment/lighthouse -c lighthouse
   ```
   - Look for successful authentication messages
   - Check for any GitHub API errors

2. Test webhook delivery:
   - Make a test commit to a repository in your organization
   - Check GitHub App "Advanced" tab → "Recent Deliveries"
   - Verify webhook was delivered successfully
   - Check Lighthouse logs for received event

3. Verify event processing:
   ```bash
   # Check Lighthouse logs for event processing
   kubectl logs -n pipeline-system deployment/lighthouse -c lighthouse | grep "processing event"
   ```

## Troubleshooting

### Webhook Delivery Failures

If webhooks are not being delivered:

1. Check the webhook URL is accessible from GitHub:
   ```bash
   curl -I https://YOUR_LIGHTHOUSE_ENDPOINT/hook
   ```

2. Verify the webhook secret matches:
   ```bash
   kubectl get secret lighthouse-github-app -n pipeline-system -o jsonpath='{.data.webhook-secret}' | base64 -d
   ```

3. Check GitHub App "Advanced" tab for delivery errors

### Authentication Failures

If Lighthouse cannot authenticate with GitHub:

1. Verify the private key is correct:
   ```bash
   kubectl get secret lighthouse-github-app -n pipeline-system -o jsonpath='{.data.private-key}' | base64 -d | head -n 1
   ```
   - Should show `-----BEGIN RSA PRIVATE KEY-----`

2. Verify App ID and Installation ID:
   ```bash
   kubectl get secret lighthouse-github-app -n pipeline-system -o jsonpath='{.data.app-id}' | base64 -d
   kubectl get secret lighthouse-github-app -n pipeline-system -o jsonpath='{.data.installation-id}' | base64 -d
   ```

3. Check Lighthouse logs for authentication errors:
   ```bash
   kubectl logs -n pipeline-system deployment/lighthouse -c lighthouse | grep -i "auth"
   ```

### Events Not Triggering Pipelines

If events are received but pipelines don't trigger:

1. Check the repository is in the allowlist:
   ```bash
   kubectl get configmap repo-allowlist -n pipeline-system -o yaml
   ```

2. Verify the tenant namespace exists:
   ```bash
   kubectl get namespace YOUR_TENANT_NAMESPACE
   ```

3. Check Lighthouse logs for allowlist rejections:
   ```bash
   kubectl logs -n pipeline-system deployment/lighthouse -c lighthouse | grep "allowlist"
   ```

## Security Considerations

1. **Private Key Security**:
   - Never commit the private key to version control
   - Store the private key in a secure secret management system
   - Rotate the private key periodically

2. **Webhook Secret**:
   - Use a strong random secret (at least 32 characters)
   - Never expose the webhook secret in logs or configuration files
   - Rotate the webhook secret periodically

3. **Permissions**:
   - Grant minimum required permissions to the GitHub App
   - Regularly review and audit GitHub App permissions
   - Monitor GitHub App activity in organization audit logs

## Next Steps

After creating the GitHub App:

1. Configure Lighthouse with GitHub App credentials (task 6.4)
2. Install Lighthouse via Helm (task 6.5)
3. Create repository allowlist (task 6.4)
4. Test webhook delivery and event processing (task 6.6)

## References

- [GitHub Apps Documentation](https://docs.github.com/en/developers/apps)
- [Creating a GitHub App](https://docs.github.com/en/developers/apps/building-github-apps/creating-a-github-app)
- [Lighthouse Documentation](https://github.com/jenkins-x/lighthouse)
