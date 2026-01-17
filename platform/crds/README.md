# Custom Resource Definitions (CRDs)

This directory contains Custom Resource Definitions for the Aphex Pipeline Infrastructure.

## RepoBinding CRD

The `RepoBinding` CRD defines the schema for repository onboarding requests. When a user in the engineering OIDC group creates a RepoBinding resource, the onboarding controller provisions all necessary tenant resources.

### Prerequisites

- Kubernetes cluster (1.24+) with RBAC enabled
- kubectl configured with cluster access
- Appropriate permissions to create CRDs (cluster-admin or equivalent)

### Spec Fields

- **repoOrg** (required): GitHub organization name (pattern: `^[a-z0-9-]+$`)
- **repoName** (required): GitHub repository name (pattern: `^[a-z0-9-]+$`)
- **tenantName** (required): Tenant namespace name (pattern: `^[a-z0-9-]+$`)
- **permissionProfile** (optional): Permission profile for tenant service account (`standard` or `elevated`, default: `standard`)

### Status Fields

- **phase**: Current phase of onboarding (`Pending`, `Provisioning`, `Ready`, `Failed`)
- **message**: Human-readable status message
- **namespaceCreated**: Whether tenant namespace was created
- **serviceAccountCreated**: Whether pipeline-runner service account was created
- **rbacConfigured**: Whether RBAC (Role + RoleBinding) was configured
- **allowlistUpdated**: Whether repository was added to allowlist
- **resourceQuotaCreated**: Whether ResourceQuota was created
- **limitRangeCreated**: Whether LimitRange was created
- **networkPolicyCreated**: Whether NetworkPolicy was created
- **backendSecretCreated**: Whether Terraform backend secret was created
- **lastReconcileTime**: Timestamp of last reconciliation

### Validation Rules

The CRD enforces the following validation rules:

1. **Organization validation**: `repoOrg` must match pattern `^[a-z0-9-]+$` (Requirements 9.1)
2. **Namespace pattern validation**: `tenantName` must match pattern `^[a-z0-9-]+$` (Requirements 9.2)
3. **Permission profile validation**: `permissionProfile` must be either `standard` or `elevated` (Requirements 9.4)

Additional validation is performed by the onboarding controller:
- Repository organization must be in approved list
- Tenant namespace must not be a privileged name (e.g., `kube-system`, `pipeline-system`)

### Example Usage

See `example-repobinding.yaml` for complete examples.

Basic example:

```yaml
apiVersion: platform.aphex/v1alpha1
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

Apply the RepoBinding:

```bash
kubectl apply -f example-repobinding.yaml
```

### Installation

#### Manual Installation

Apply the CRD to your cluster:

```bash
kubectl apply -f repobinding.yaml
```

Verify the CRD is registered:

```bash
kubectl get crd repobindings.platform.aphex
```

Wait for the CRD to be established:

```bash
kubectl wait --for condition=established --timeout=60s crd/repobindings.platform.aphex
```

#### Automated Installation and Verification

Use the provided verification script:

```bash
chmod +x verify-crd.sh
./verify-crd.sh
```

This script will:
1. Check cluster access
2. Apply the RepoBinding CRD
3. Wait for the CRD to be established
4. Verify the CRD is registered
5. Test RepoBinding creation with a dry-run
6. Display CRD details and API resources

### Viewing RepoBindings

List all RepoBindings:

```bash
kubectl get repobindings -n pipeline-system
# or use the short name
kubectl get rb -n pipeline-system
```

Get details of a specific RepoBinding:

```bash
kubectl describe repobinding archon-binding -n pipeline-system
```

### Status Phases

- **Pending**: RepoBinding has been created but not yet processed
- **Provisioning**: Onboarding controller is provisioning tenant resources
- **Ready**: All tenant resources have been successfully provisioned
- **Failed**: Onboarding failed (check `message` field for details)

**Source**
- `.kiro/specs/jenkinsx-platform/design.md` (Onboarding API section)
- `.kiro/specs/jenkinsx-platform/requirements.md` (Requirements 9.1-9.5)
