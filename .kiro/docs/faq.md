# FAQ

## General Questions

### What is this repository for?

The Arbiter Pipeline Infrastructure provides a shared CI/CD platform cluster for multiple product teams using Jenkins X, Lighthouse, and Tekton. It enables self-service repository onboarding with strong tenant isolation through Kubernetes namespaces, RBAC, and network policies.

### How does this fit into the larger system?

This platform provides shared CI/CD infrastructure for the Arbiter agent suite and product teams. Teams can onboard their repositories, which automatically provisions isolated tenant resources and enables automated CDKTF deployments on merge to main.

### What is the difference between Jenkins X and this platform?

Jenkins X is the underlying CI/CD framework. This platform is a complete solution built on Jenkins X that adds:
- Self-service onboarding via RepoBinding CRD
- Automated tenant provisioning
- OIDC authentication
- Repository allowlist security
- Golden pipeline catalog
- Terraform state management

### Can I use this platform for non-CDKTF projects?

Yes! While the platform includes a CDKTF pipeline in the catalog, you can define custom pipelines in your repository for any build/deploy workflow. The platform provides the infrastructure and isolation; you define the pipeline steps.

### What is a tenant?

A tenant is a product team with an isolated namespace and dedicated pipeline resources. Each tenant gets:
- Dedicated Kubernetes namespace
- Service account with least-privilege RBAC
- Resource quotas to prevent exhaustion
- Network policies for isolation
- Terraform backend configuration

## Development Questions

### How do I onboard my repository?

Create a RepoBinding resource:

```bash
kubectl apply -f - <<EOF
apiVersion: platform.arbiter.io/v1alpha1
kind: RepoBinding
metadata:
  name: my-repo-binding
  namespace: pipeline-system
spec:
  repoOrg: "your-github-org"
  repoName: "your-repo"
  tenantName: "my-tenant"
  permissionProfile: "standard"
EOF
```

Then verify onboarding:

```bash
kubectl get repobinding my-repo-binding -n pipeline-system
kubectl get namespace my-tenant
```

### How do I define a pipeline in my repository?

Create a `.lighthouse/jenkins-x/` directory in your repository with pipeline definitions:

```yaml
# .lighthouse/jenkins-x/triggers.yaml
apiVersion: config.lighthouse.jenkins-x.io/v1alpha1
kind: TriggerConfig
spec:
  presubmits:
    - name: pr-build
      context: pr-build
      always_run: true
      pipeline_run_spec:
        pipelineRef:
          name: cdktf-deploy-pipeline
          namespace: pipeline-catalog
        params:
          - name: repo-url
            value: $(body.repository.clone_url)
          - name: commit-sha
            value: $(body.pull_request.head.sha)
          - name: tenant-name
            value: my-tenant
  postsubmits:
    - name: deploy
      context: deploy
      branches:
        - main
      pipeline_run_spec:
        pipelineRef:
          name: cdktf-deploy-pipeline
          namespace: pipeline-catalog
        params:
          - name: repo-url
            value: $(body.repository.clone_url)
          - name: commit-sha
            value: $(body.after)
          - name: tenant-name
            value: my-tenant
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
    namespace: pipeline-catalog
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

### How do I view pipeline logs?

```bash
# List PipelineRuns
kubectl get pipelineruns -n my-tenant

# Get PipelineRun details
kubectl describe pipelinerun <name> -n my-tenant

# View logs
kubectl logs <pod-name> -n my-tenant

# Stream logs
kubectl logs -f <pod-name> -n my-tenant
```

### How do I add custom Tekton Tasks?

You can define custom Tasks in your repository and reference them in your pipeline:

```yaml
# my-repo/.lighthouse/jenkins-x/tasks/my-custom-task.yaml
apiVersion: tekton.dev/v1beta1
kind: Task
metadata:
  name: my-custom-task
spec:
  steps:
    - name: custom-step
      image: alpine:latest
      script: |
        #!/bin/sh
        echo "Running custom task"
```

Then reference it in your pipeline definition.

## Operational Questions

### How do I check if my repository is allowlisted?

```bash
# View allowlist
kubectl get configmap repo-allowlist -n pipeline-system -o yaml | grep "your-repo"

# Or check RepoBinding status
kubectl get repobinding <name> -n pipeline-system
```

### What should I do if my repository isn't triggering pipelines?

**Diagnosis**:

```bash
# Check if repository is in allowlist
kubectl get configmap repo-allowlist -n pipeline-system -o yaml | grep "your-repo"

# Check Lighthouse logs for webhook events
kubectl logs -n pipeline-system -l app=lighthouse | grep "your-repo"

# Check if tenant namespace exists
kubectl get namespace <tenant-name>

# Check RepoBinding status
kubectl describe repobinding <name> -n pipeline-system
```

**Common Issues**:
1. Repository not in allowlist (check RepoBinding status)
2. GitHub App not delivering webhooks (check GitHub App settings)
3. Tenant namespace not created (check onboarding controller logs)
4. Invalid pipeline definition (check Lighthouse logs)

### What should I do if my pipeline fails?

**Diagnosis**:

```bash
# Get PipelineRun status
kubectl get pipelinerun <name> -n <tenant-namespace>

# Get detailed status
kubectl describe pipelinerun <name> -n <tenant-namespace>

# Get pod logs
kubectl logs <pod-name> -n <tenant-namespace>

# Check pod events
kubectl get events -n <tenant-namespace> --sort-by='.lastTimestamp'
```

**Common Issues**:
1. Git clone failure: Check repository access
2. CDKTF synth failure: Check Node.js dependencies and syntax
3. CDKTF deploy failure: Check Terraform state and permissions
4. RBAC denial: Check service account permissions

### How do I update my tenant's resource quota?

Resource quotas are managed by the onboarding controller. To update, you need to modify the tenant resource templates in `platform/tenancy/templates/resourcequota.yaml` and redeploy the controller.

For immediate changes, you can manually edit:

```bash
kubectl edit resourcequota tenant-quota -n <tenant-namespace>
```

### How do I grant my tenant elevated permissions?

Update your RepoBinding to use the elevated profile:

```bash
kubectl patch repobinding <name> -n pipeline-system --type merge -p '{"spec":{"permissionProfile":"elevated"}}'
```

The onboarding controller will reconcile and update the Role.

### How do I delete a tenant?

Delete the RepoBinding (this will trigger cleanup):

```bash
kubectl delete repobinding <name> -n pipeline-system
```

Or manually delete the namespace:

```bash
kubectl delete namespace <tenant-namespace>
```

## Architecture Questions

### Why use Jenkins X instead of GitHub Actions?

Jenkins X provides several advantages for homelab deployment:
1. Self-hosted (no GitHub Actions minutes costs)
2. Runs on Kubernetes (same environment as deployment target)
3. Better integration with Kubernetes resources
4. More flexible pipeline definitions
5. Supports multiple Git providers

### Why use Tekton instead of Argo Workflows?

Tekton is the standard pipeline engine for Jenkins X and provides:
1. Native Kubernetes integration
2. Reusable Tasks and Pipelines
3. Strong community support
4. Cloud-native design
5. Better integration with Lighthouse

### How does tenant isolation work?

Isolation is achieved through multiple layers:
1. **Kubernetes Namespaces**: Each tenant gets dedicated namespace
2. **RBAC**: Service accounts scoped to tenant namespace only
3. **Network Policies**: Restrict inter-namespace communication
4. **Resource Quotas**: Prevent resource exhaustion
5. **Terraform State**: Isolated per tenant

### Why use OIDC authentication?

OIDC provides:
1. No long-lived credentials
2. Integration with existing identity providers
3. Group-based authorization
4. Standard protocol
5. Self-service onboarding without cluster admin access

### Can multiple clusters be deployed?

Yes! You can deploy separate clusters for different environments:
- Development cluster (smaller, fewer resources)
- Staging cluster (production-like)
- Production cluster (larger, more resources)

Each cluster is independent with its own tenants and allowlist.

### How does the allowlist work?

The allowlist is a ConfigMap that defines which repositories can trigger pipelines. When Lighthouse receives a webhook:
1. Check if repository is in allowlist
2. If yes, lookup tenant namespace from allowlist
3. Create PipelineRun in tenant namespace
4. If no, reject event and log rejection

The onboarding controller automatically updates the allowlist when RepoBinding resources are created.

## Security Questions

### How are GitHub App credentials stored?

GitHub App private key is stored in a Kubernetes Secret in the `pipeline-system` namespace. Lighthouse reads the secret to authenticate with GitHub.

### How are Terraform credentials managed?

Terraform backend credentials are stored in per-tenant Secrets. For Kubernetes backend, no external credentials are needed. For MinIO/S3 backend, credentials are stored in tenant namespace secrets.

### Can tenants access other tenants' resources?

No. RBAC ensures service accounts can only access resources in their own namespace. Network policies prevent cross-namespace network access.

### Can tenants access platform namespaces?

No. RBAC denies access to platform namespaces (pipeline-system, tekton-pipelines, etc.). Only the onboarding controller has permissions to create resources in platform namespaces.

### How do I rotate GitHub App credentials?

```bash
# Generate new private key in GitHub App settings

# Update secret
kubectl create secret generic github-app-secret \
  --from-file=private-key=path/to/new-private-key.pem \
  -n pipeline-system \
  --dry-run=client -o yaml | kubectl apply -f -

# Restart Lighthouse
kubectl rollout restart deployment lighthouse -n pipeline-system
```

### How do I audit tenant activity?

```bash
# View PipelineRuns for a tenant
kubectl get pipelineruns -n <tenant-namespace>

# View events for a tenant
kubectl get events -n <tenant-namespace> --sort-by='.lastTimestamp'

# View Lighthouse logs for a repository
kubectl logs -n pipeline-system -l app=lighthouse | grep "repo-name"
```

## Testing Questions

### How do I test the onboarding process?

Create a test RepoBinding and verify resources are created:

```bash
# Create test RepoBinding
kubectl apply -f test-repobinding.yaml

# Verify namespace
kubectl get namespace test-tenant

# Verify service account
kubectl get serviceaccount pipeline-runner -n test-tenant

# Verify RBAC
kubectl get role,rolebinding -n test-tenant

# Verify resource limits
kubectl get resourcequota,limitrange -n test-tenant

# Verify network policy
kubectl get networkpolicy -n test-tenant

# Clean up
kubectl delete repobinding test-binding -n pipeline-system
kubectl delete namespace test-tenant
```

### How do I test pipeline execution?

Create a test PipelineRun manually:

```bash
kubectl create -f test-pipelinerun.yaml

# Watch status
kubectl get pipelinerun test-run -n test-tenant -w

# View logs
kubectl logs <pod-name> -n test-tenant
```

### How do I test network isolation?

```bash
# Try to access another tenant's service
kubectl run -it --rm debug --image=busybox --restart=Never -n tenant1 -- \
  wget -O- http://service.tenant2.svc.cluster.local

# Should fail with connection timeout or refused
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
- `.kiro/specs/jenkinsx-platform/design.md`
- `.kiro/specs/jenkinsx-platform/requirements.md`
- `.kiro/docs/operations.md`
- `.kiro/docs/api.md`
