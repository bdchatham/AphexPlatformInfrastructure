# Platform Controller

The platform controller is a Kubernetes operator that automates tenant provisioning for the Aphex Platform Infrastructure. It watches for `RepoBinding` and `Organization` custom resources and provisions all necessary tenant infrastructure.

## Features

- **Validation**: Validates repository organization, namespace patterns, and permission profiles
- **Namespace Provisioning**: Creates tenant namespaces with appropriate labels
- **Service Account Creation**: Creates pipeline-runner service accounts for pipeline execution
- **RBAC Configuration**: Configures Roles and RoleBindings based on permission profiles (standard/elevated)
- **Resource Limits**: Applies ResourceQuotas and LimitRanges to prevent resource exhaustion
- **Network Isolation**: Creates NetworkPolicies for tenant isolation
- **Terraform Backend**: Provisions Kubernetes backend configuration for Terraform state
- **Allowlist Management**: Automatically updates the Lighthouse repository allowlist

## Building

```bash
# Build the controller binary
make build

# Build the Docker image
make docker-build IMG=localhost:5000/platform-controller:latest

# Push the Docker image
make docker-push IMG=localhost:5000/platform-controller:latest
```

## Running Locally

```bash
# Run against the configured Kubernetes cluster
make run
```

## Deploying to Cluster

```bash
# Deploy the controller to the cluster
make deploy
```

## Usage

Create a `RepoBinding` resource to onboard a repository:

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
  permissionProfile: "standard"  # or "elevated"
```

Apply the resource:

```bash
kubectl apply -f repobinding.yaml
```

Check the status:

```bash
kubectl get repobinding archon-binding -n pipeline-system -o yaml
```

The status will show:
- `phase`: Current state (Pending, Provisioning, Ready, Failed)
- `message`: Detailed status message
- `namespaceCreated`: Whether the namespace was created
- `serviceAccountCreated`: Whether the service account was created
- `rbacConfigured`: Whether RBAC was configured
- `allowlistUpdated`: Whether the allowlist was updated
- `lastReconcileTime`: Timestamp of last reconciliation

## Permission Profiles

### Standard Profile
- Manage pods, ConfigMaps, Secrets
- Create PipelineRuns and TaskRuns
- Read PersistentVolumeClaims

### Elevated Profile
- All standard permissions
- Manage PersistentVolumeClaims
- Manage Services and Deployments
- Read Events for debugging

## Configuration

The controller can be configured via environment variables or command-line flags:

- `--metrics-bind-address`: Address for metrics endpoint (default: `:8080`)
- `--health-probe-bind-address`: Address for health probe endpoint (default: `:8081`)
- `--leader-elect`: Enable leader election (default: `false`)

## Approved Organizations

Edit `controllers/validators.go` to configure approved GitHub organizations:

```go
approvedOrgs = []string{
    "your-github-org",
    "another-org",
}
```

## Architecture

The controller follows the Kubernetes operator pattern:

1. **Reconciler**: Main reconciliation loop that processes RepoBinding resources
2. **Validators**: Validates RepoBinding specs against security policies
3. **Provisioners**: Creates and updates Kubernetes resources for tenants

## Development

### Project Structure

```
controller/
├── api/v1alpha1/          # API types and CRD definitions
│   ├── repobinding_types.go
│   ├── groupversion_info.go
│   └── zz_generated.deepcopy.go
├── controllers/           # Controller logic
│   ├── repobinding_controller.go
│   ├── validators.go
│   └── provisioners.go
├── main.go               # Entry point
├── Dockerfile            # Container image definition
├── Makefile              # Build automation
└── go.mod                # Go module dependencies
```

### Adding New Validation Rules

Edit `controllers/validators.go` and add validation functions. Update the `ValidateRepoBinding` function to call your new validators.

### Adding New Provisioning Steps

1. Add a new function to `controllers/provisioners.go`
2. Add a status field to `api/v1alpha1/repobinding_types.go` if needed
3. Call the function from the reconciler in `controllers/repobinding_controller.go`
4. Update error handling and status updates

## Troubleshooting

### Controller Not Starting

Check the controller logs:

```bash
kubectl logs -n pipeline-system deployment/platform-controller
```

### RepoBinding Stuck in Provisioning

Check the RepoBinding status:

```bash
kubectl describe repobinding <name> -n pipeline-system
```

Check controller logs for errors:

```bash
kubectl logs -n pipeline-system deployment/platform-controller | grep ERROR
```

### Validation Failures

Common validation errors:
- **Organization not approved**: Add the organization to `approvedOrgs` in `validators.go`
- **Invalid namespace pattern**: Namespace must match `^[a-z0-9-]+$`
- **Privileged namespace**: Cannot use system namespace names (kube-system, pipeline-system, etc.)
- **Invalid permission profile**: Must be "standard" or "elevated"

## License

Copyright 2025 Aphex Platform Team
