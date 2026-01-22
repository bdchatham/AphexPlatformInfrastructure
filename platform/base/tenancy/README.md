# Tenant Resource Templates

This directory contains Kubernetes resource templates for provisioning tenant namespaces. These templates are used by the onboarding controller to create isolated environments for each product team.

## Templates

### namespace.yaml
Creates a tenant namespace with appropriate labels for tracking and management.

**Variables:**
- `${TENANT_NAME}`: Name of the tenant namespace (e.g., "archon")
- `${REPO_ORG}`: GitHub organization (e.g., "your-github-org")
- `${REPO_NAME}`: Repository name (e.g., "archon-agent")

**Requirements:** 5.1, 5.2

### service-account.yaml
Creates the `pipeline-runner` service account used for pipeline execution within the tenant namespace.

**Variables:**
- `${TENANT_NAME}`: Name of the tenant namespace

**Requirements:** 6.1

### role-standard.yaml
Defines the standard permission profile with namespace-scoped permissions for pipeline execution.

**Permissions:**
- Manage pods and pod logs
- Manage ConfigMaps and Secrets
- Create and view Tekton PipelineRuns and TaskRuns
- Read PersistentVolumeClaims

**Variables:**
- `${TENANT_NAME}`: Name of the tenant namespace

**Requirements:** 6.2, 9.4

### role-elevated.yaml
Defines the elevated permission profile with additional permissions for advanced use cases.

**Additional Permissions (beyond standard):**
- Manage PersistentVolumeClaims
- Manage Services
- Manage Deployments
- Read Events

**Variables:**
- `${TENANT_NAME}`: Name of the tenant namespace

**Requirements:** 6.2, 9.4

### rolebinding.yaml
Binds the pipeline-runner service account to the appropriate Role (standard or elevated).

**Variables:**
- `${TENANT_NAME}`: Name of the tenant namespace

**Requirements:** 6.3

### resourcequota.yaml
Defines resource limits for the tenant namespace to prevent resource exhaustion.

**Limits:**
- CPU requests: 4 cores
- CPU limits: 8 cores
- Memory requests: 8Gi
- Memory limits: 16Gi
- PersistentVolumeClaims: 5
- Pods: 20

**Variables:**
- `${TENANT_NAME}`: Name of the tenant namespace

**Requirements:** 5.3, 14.2

### limitrange.yaml
Defines default resource requests and limits for containers in the tenant namespace.

**Defaults:**
- Default CPU limit: 500m
- Default memory limit: 512Mi
- Default CPU request: 100m
- Default memory request: 128Mi
- Max CPU: 2 cores
- Max memory: 4Gi
- Min CPU: 50m
- Min memory: 64Mi

**Variables:**
- `${TENANT_NAME}`: Name of the tenant namespace

**Requirements:** 5.4

### networkpolicy.yaml
Enforces network isolation for the tenant namespace.

**Rules:**
- Allow ingress from same namespace only
- Allow egress to same namespace
- Allow DNS queries to kube-system
- Allow internet egress (for git clone, terraform providers)
- Block cross-namespace traffic (private IP ranges)

**Variables:**
- `${TENANT_NAME}`: Name of the tenant namespace

**Requirements:** 5.5, 14.4

### terraform-backend-secret.yaml
Configures Terraform backend for remote state storage using Kubernetes backend.

**Configuration:**
- Backend type: Kubernetes
- State stored in Kubernetes Secret
- Scoped to tenant namespace
- In-cluster authentication

**Variables:**
- `${TENANT_NAME}`: Name of the tenant namespace

**Requirements:** 7.1, 7.2, 7.3

## Usage

These templates are used by the onboarding controller during the reconciliation process. The controller:

1. Reads the template files
2. Substitutes variables with values from the RepoBinding resource
3. Applies the rendered manifests to the cluster
4. Updates the RepoBinding status to track provisioning progress

## Variable Substitution

The onboarding controller performs simple string replacement for template variables:

```go
template := strings.ReplaceAll(templateContent, "${TENANT_NAME}", tenantName)
template = strings.ReplaceAll(template, "${REPO_ORG}", repoOrg)
template = strings.ReplaceAll(template, "${REPO_NAME}", repoName)
```

## Permission Profiles

The onboarding controller selects the appropriate Role template based on the `permissionProfile` field in the RepoBinding:

- `standard`: Uses `role-standard.yaml` (default)
- `elevated`: Uses `role-elevated.yaml`

## Example RepoBinding

```yaml
apiVersion: aphex.io/v1alpha1
kind: RepoBinding
metadata:
  name: archon-binding
  namespace: pipeline-system
spec:
  repoOrg: "your-github-org"
  repoName: "archon-agent"
  tenantName: "archon"
  permissionProfile: "standard"
```

This will provision:
- Namespace: `archon`
- ServiceAccount: `archon/pipeline-runner`
- Role: `archon/pipeline-runner` (standard profile)
- RoleBinding: `archon/pipeline-runner`
- ResourceQuota: `archon/tenant-quota`
- LimitRange: `archon/tenant-limits`
- NetworkPolicy: `archon/tenant-isolation`
- Secret: `archon/terraform-backend-config`

## Testing

To test template rendering manually:

```bash
# Set variables
export TENANT_NAME="test-tenant"
export REPO_ORG="your-github-org"
export REPO_NAME="test-repo"

# Render namespace template
envsubst < templates/namespace.yaml

# Render all templates
for template in templates/*.yaml; do
  echo "--- $template ---"
  envsubst < "$template"
  echo
done
```

## Source

These templates are defined in the design document at `.kiro/specs/jenkinsx-platform/design.md` under the "Tenant Namespace Resources" section.
