# FAQ

## General Questions

### What is this repository for?

The Arbiter Pipeline Infrastructure provides a lightweight GitOps platform using ArgoCD and Tekton for homelab Kubernetes clusters. It enables self-service repository registration with automated tenant provisioning, CDKTF deployment pipelines, and self-upgrade capabilities through ArgoCD-based GitOps.

### How does this fit into the larger system?

This platform provides shared CI/CD infrastructure for the Arbiter agent suite and product teams. Teams can onboard their repositories, which automatically provisions isolated tenant resources and enables automated CDKTF deployments on merge to main.

### What is a tenant?

A tenant is a product team with an isolated namespace and dedicated pipeline resources. Each tenant gets:
- Dedicated Kubernetes namespace
- Service account with least-privilege RBAC
- Resource quotas to prevent exhaustion
- Network policies for isolation
- EventListener for webhook handling
- Ingress for webhook routing
- Terraform backend configuration

### Can I use this platform for non-CDKTF projects?

Yes! While the platform includes a CDKTF pipeline in the catalog, you can define custom pipelines in your repository for any build/deploy workflow. The platform provides the infrastructure and isolation; you define the pipeline steps.

## Development Questions

### How do I onboard my repository?

Create a RepoBinding resource:

```bash
kubectl apply -f - <<EOF
apiVersion: arbiter.io/v1alpha1
kind: RepoBinding
metadata:
  name: my-repo-binding
  namespace: platform-system
spec:
  repoOrg: "your-github-org"
  repoName: "your-repo"
  tenantName: "my-tenant"
  permissionProfile: "standard"
  ingressHost: "webhooks.example.com"
EOF
```

Then verify onboarding:

```bash
kubectl get repobinding my-repo-binding -n platform-system
kubectl get namespace my-tenant
```

### How do I configure the GitHub webhook?

After onboarding, get the webhook URL and secret from the RepoBinding status:

```bash
kubectl get repobinding my-repo-binding -n platform-system -o yaml
```

Then configure in GitHub:
1. Go to repository Settings → Webhooks → Add webhook
2. Payload URL: (from RepoBinding status.webhookURL)
3. Content type: application/json
4. Secret: (from RepoBinding status.webhookSecret)
5. Events: Push events
6. Active: ✓

### How do I view pipeline logs?

```bash
# List PipelineRuns
kubectl get pipelineruns -n my-tenant

# Get PipelineRun details
kubectl describe pipelinerun <name> -n my-tenant

# View logs
kubectl logs -n my-tenant -l tekton.dev/pipelineRun=<name>

# Stream logs
kubectl logs -n my-tenant -l tekton.dev/pipelineRun=<name> -f
```

### How do I test my pipeline locally?

You can create a PipelineRun manually:

```bash
kubectl create -f - <<EOF
apiVersion: tekton.dev/v1beta1
kind: PipelineRun
metadata:
  name: test-run
  namespace: my-tenant
spec:
  pipelineRef:
    name: cdktf-deploy-pipeline
    namespace: platform-system
  params:
    - name: repo-url
      value: "https://github.com/your-github-org/your-repo"
    - name: commit-sha
      value: "abc123..."
    - name: tenant-name
      value: "my-tenant"
  serviceAccountName: pipeline-runner
EOF
```

## Operational Questions

### What should I do if my repository isn't triggering pipelines?

**Diagnosis**:

```bash
# Check if EventListener exists
kubectl get eventlistener -n <tenant-namespace>

# Check EventListener logs for webhook events
kubectl logs -n <tenant-namespace> -l eventlistener=github-listener | grep "webhook"

# Check if Ingress exists
kubectl get ingress -n <tenant-namespace>

# Check RepoBinding status
kubectl describe repobinding <name> -n platform-system
```

**Common Issues**:
1. EventListener not running (check onboarding controller logs)
2. Ingress not configured correctly (check Ingress controller installation)
3. GitHub webhook not configured (check GitHub webhook settings)
4. Webhook secret mismatch (check RepoBinding status for correct secret)

### What should I do if my pipeline fails?

**Diagnosis**:

```bash
# Get PipelineRun status
kubectl get pipelinerun <name> -n <tenant-namespace>

# Get detailed status
kubectl describe pipelinerun <name> -n <tenant-namespace>

# Get pod logs
kubectl logs -n <tenant-namespace> -l tekton.dev/pipelineRun=<name>

# Check pod events
kubectl get events -n <tenant-namespace> --sort-by='.lastTimestamp'
```

**Common Issues**:
1. Git clone failure: Check repository access
2. CDKTF synth failure: Check Node.js dependencies and syntax
3. CDKTF deploy failure: Check Terraform state and permissions
4. RBAC denial: Check service account permissions

### How do I update platform components?

The platform upgrades itself via ArgoCD when manifests change in Git:

```bash
# Update component manifests in Git
vi platform/onboarding/controller-deployment.yaml  # Update image tag

# Commit changes
git add .
git commit -m "Update onboarding controller to v1.1.0"
git push

# ArgoCD will automatically sync and update the controller
# Watch sync status
kubectl get application -n argocd -w
```

### How do I grant my tenant elevated permissions?

Update your RepoBinding to use the elevated profile:

```bash
kubectl patch repobinding <name> -n platform-system --type merge -p '{"spec":{"permissionProfile":"elevated"}}'
```

The onboarding controller will reconcile and update the Role.

### How do I delete a tenant?

Delete the RepoBinding:

```bash
kubectl delete repobinding <name> -n platform-system
```

Or manually delete the namespace:

```bash
kubectl delete namespace <tenant-namespace>
```

## Architecture Questions

### Why use ArgoCD instead of Flux?

ArgoCD provides several advantages:
1. Better UI for visualizing sync status
2. More mature and widely adopted
3. Better support for App of Apps pattern
4. Easier to troubleshoot sync issues
5. Strong community support

### Why use Tekton instead of Argo Workflows?

Tekton is the standard pipeline engine for Kubernetes-native CI/CD and provides:
1. Native Kubernetes integration
2. Reusable Tasks and Pipelines
3. Strong community support
4. Cloud-native design
5. Better integration with Tekton Triggers for webhooks

### How does tenant isolation work?

Isolation is achieved through multiple layers:
1. **Kubernetes Namespaces**: Each tenant gets dedicated namespace
2. **RBAC**: Service accounts scoped to tenant namespace only
3. **Network Policies**: Restrict inter-namespace communication
4. **Resource Quotas**: Prevent resource exhaustion
5. **Terraform State**: Isolated per tenant

### How does the App of Apps pattern work?

The platform uses a root ArgoCD Application that manages child Applications for each component layer:
- **platform-root**: Manages all child Applications
- **platform-crds**: CRDs and foundational resources
- **platform-infrastructure**: Namespaces and RBAC
- **platform-controllers**: Onboarding controller
- **platform-catalog**: Tekton tasks, pipelines, triggers

This provides better separation of concerns, independent lifecycle management, and clearer troubleshooting.

### Can multiple clusters be deployed?

Yes! You can deploy separate clusters for different environments:
- Development cluster (smaller, fewer resources)
- Staging cluster (production-like)
- Production cluster (larger, more resources)

Each cluster is independent with its own tenants and ArgoCD Applications.

## Security Questions

### How are webhook secrets stored?

Webhook secrets are generated by the Onboarding Controller using cryptographic randomness and stored in Kubernetes Secrets in the tenant namespace. EventListeners reference these secrets for webhook signature validation.

### How are Terraform credentials managed?

Terraform backend credentials are stored in per-tenant Secrets. For Kubernetes backend, no external credentials are needed. The platform uses the Kubernetes backend by default for simplicity.

### Can tenants access other tenants' resources?

No. RBAC ensures service accounts can only access resources in their own namespace. Network policies prevent cross-namespace network access.

### Can tenants access platform namespaces?

No. RBAC denies access to platform namespaces (platform-system, argocd, tekton-pipelines, etc.). Only the onboarding controller has permissions to create resources in platform namespaces.

### How do I rotate webhook secrets?

```bash
# Delete existing secret
kubectl delete secret webhook-<tenant-name> -n <tenant-namespace>

# Delete and recreate RepoBinding to regenerate secret
kubectl delete repobinding <name> -n platform-system
kubectl apply -f repobinding.yaml

# Get new webhook secret from RepoBinding status
kubectl get repobinding <name> -n platform-system -o yaml

# Update GitHub webhook with new secret
```

### How do I audit tenant activity?

```bash
# View PipelineRuns for a tenant
kubectl get pipelineruns -n <tenant-namespace>

# View events for a tenant
kubectl get events -n <tenant-namespace> --sort-by='.lastTimestamp'

# View EventListener logs for a tenant
kubectl logs -n <tenant-namespace> -l eventlistener=github-listener
```

## Troubleshooting Questions

### Why is ArgoCD not syncing my changes?

**Common Issues**:
1. ArgoCD cannot access Git repository (check repo-server logs)
2. Sync policy not configured (check Application spec)
3. Manifest errors in Git (check Application status)
4. ArgoCD controller not running (check argocd namespace)

**Resolution**:
```bash
# Check Application sync status
kubectl get application platform-root -n argocd

# Check ArgoCD controller logs
kubectl logs -n argocd -l app.kubernetes.io/name=argocd-application-controller --tail=100

# Manually trigger sync
kubectl patch application platform-root -n argocd --type merge -p '{"operation":{"initiatedBy":{"username":"admin"},"sync":{"revision":"HEAD"}}}'
```

### Why is the onboarding controller not reconciling?

**Common Issues**:
1. Controller not running (check pod status)
2. Controller lacks RBAC permissions (check controller logs)
3. RepoBinding validation failed (check RepoBinding status)
4. Tekton Triggers not installed (check tekton-pipelines namespace)

**Resolution**:
```bash
# Check controller logs
kubectl logs -n platform-system -l app=onboarding-controller --tail=100

# Check controller pod status
kubectl get pods -n platform-system -l app=onboarding-controller

# Restart controller if needed
kubectl rollout restart deployment onboarding-controller -n platform-system
```

### Why is my webhook not being delivered?

**Common Issues**:
1. Ingress not accessible from GitHub (check Ingress configuration)
2. Webhook secret mismatch (check RepoBinding status)
3. EventListener not running (check pod status)
4. GitHub webhook not configured (check GitHub webhook settings)

**Resolution**:
```bash
# Check EventListener logs
kubectl logs -n <tenant-namespace> -l eventlistener=github-listener --tail=100

# Check Ingress configuration
kubectl get ingress -n <tenant-namespace> -o yaml

# Check GitHub webhook delivery logs
# Go to GitHub repository Settings → Webhooks → Recent Deliveries
```

## Archon-Specific Questions

### How is this repository ingested by Archon?

Archon reads all Markdown files under `.kiro/docs/` from this public GitHub repository. Documentation follows the contract defined in `CLAUDE.md`.

### How do I update documentation?

Update the relevant files under `.kiro/docs/` and ensure changes are grounded in code. Include "Source" references to relevant files. Follow the 6-file structure (overview, architecture, operations, api, data-models, faq).

### What documentation standards should I follow?

Follow the Archon documentation contract in `CLAUDE.md`:
1. Keep sections small and focused (400-800 tokens)
2. Use clear, direct language
3. Maintain provenance (reference source files)
4. No hallucinations (only document what exists)
5. Avoid duplication (link instead of repeating)
6. Use descriptive, specific headings

**Source**
- `CLAUDE.md`
- `.kiro/steering/archon-docs.md`
- `README.md`
- `.kiro/specs/argocd-tekton-platform/design.md`
- `.kiro/specs/argocd-tekton-platform/requirements.md`
- `.kiro/docs/operations.md`
- `.kiro/docs/api.md`
