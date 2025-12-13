# Operations

## Deployment

### Prerequisites

Before deploying the Arbiter Pipeline Infrastructure, ensure you have:

1. **AWS Account**: An AWS account with appropriate permissions
2. **AWS CLI**: Configured with credentials (`aws configure`)
3. **AWS CDK CLI**: Installed globally (`npm install -g aws-cdk`)
4. **Node.js**: Version 20.x or later
5. **Python**: Version 3.11 or later
6. **Docker**: For building container images (optional)

### Initial Setup

```bash
# Clone the repository
git clone <repo-url>
cd arbiter-pipeline-infrastructure

# Install dependencies
make install

# Build the project
make build
```

### Bootstrap CDK (First Time Only)

```bash
# Bootstrap CDK in your AWS account and region
cdk bootstrap aws://<account-id>/<region>

# Example:
cdk bootstrap aws://123456789012/us-east-1
```

### Deploy the Infrastructure

```bash
# Deploy with default configuration
cdk deploy

# Deploy with custom cluster name
cdk deploy --context clusterName=my-cluster

# Deploy with custom node configuration
cdk deploy \
  --context minNodes=3 \
  --context maxNodes=20 \
  --context instanceType=t3.large
```

### Verify Deployment

```bash
# Get cluster name from CloudFormation exports
aws cloudformation list-exports \
  --query "Exports[?Name=='ArbiterCluster-ClusterName'].Value" \
  --output text

# Update kubeconfig
aws eks update-kubeconfig \
  --name <cluster-name> \
  --region <region>

# Verify cluster is accessible
kubectl get nodes

# Verify Argo Workflows is running
kubectl get pods -n argo

# Verify Argo Events is running
kubectl get pods -n argo | grep events
```

### Build and Publish Container Images

```bash
# Build all container images locally
npm run build:images

# Build with version tag
./scripts/build-images.sh --version v1.0.0

# Build and push to registry (requires registry credentials)
./scripts/build-images.sh --version v1.0.0 --push --registry <registry-url>
```

## Monitoring

### CloudWatch Metrics

The cluster automatically sends metrics to CloudWatch:

**EKS Cluster Metrics**:
- Cluster status
- Node count
- Pod count
- API server request latency

**Container Insights Metrics** (if enabled):
- CPU utilization (cluster, node, pod)
- Memory utilization (cluster, node, pod)
- Network throughput
- Disk I/O

**Custom Metrics** (from execution scripts):
- Workflow execution count
- Workflow success/failure rate
- Workflow duration
- Build artifact size

### CloudWatch Logs

All execution script output is captured in CloudWatch Logs:

**Log Groups**:
- `/aws/eks/<cluster-name>/cluster`: EKS control plane logs
- `/aws/containerinsights/<cluster-name>/application`: Application logs
- `/aws/containerinsights/<cluster-name>/host`: Node logs
- `/aws/containerinsights/<cluster-name>/dataplane`: Data plane logs

**Viewing Logs**:
```bash
# View recent logs from a specific pod
kubectl logs <pod-name> -n <namespace>

# Stream logs in real-time
kubectl logs -f <pod-name> -n <namespace>

# View logs in CloudWatch
aws logs tail /aws/eks/<cluster-name>/cluster --follow
```

### Argo Workflows UI

Access the Argo Workflows UI to monitor workflow execution:

```bash
# Port-forward to Argo Workflows server
kubectl port-forward -n argo svc/argo-workflows-server 2746:2746

# Open browser to http://localhost:2746
```

### Kubernetes Dashboard (Optional)

```bash
# Install Kubernetes Dashboard
kubectl apply -f https://raw.githubusercontent.com/kubernetes/dashboard/v2.7.0/aio/deploy/recommended.yaml

# Create admin service account
kubectl create serviceaccount dashboard-admin -n kubernetes-dashboard
kubectl create clusterrolebinding dashboard-admin \
  --clusterrole=cluster-admin \
  --serviceaccount=kubernetes-dashboard:dashboard-admin

# Get access token
kubectl -n kubernetes-dashboard create token dashboard-admin

# Port-forward to dashboard
kubectl port-forward -n kubernetes-dashboard svc/kubernetes-dashboard 8443:443

# Open browser to https://localhost:8443
```

## Alerting

### CloudWatch Alarms

Create CloudWatch alarms for critical metrics:

```bash
# High workflow failure rate
aws cloudwatch put-metric-alarm \
  --alarm-name arbiter-high-workflow-failure-rate \
  --alarm-description "Alert when workflow failure rate exceeds 10%" \
  --metric-name WorkflowFailureRate \
  --namespace Arbiter/Pipelines \
  --statistic Average \
  --period 300 \
  --evaluation-periods 2 \
  --threshold 10 \
  --comparison-operator GreaterThanThreshold

# Cluster at maximum capacity
aws cloudwatch put-metric-alarm \
  --alarm-name arbiter-cluster-at-max-capacity \
  --alarm-description "Alert when cluster reaches maximum node count" \
  --metric-name NodeCount \
  --namespace AWS/EKS \
  --statistic Maximum \
  --period 60 \
  --evaluation-periods 1 \
  --threshold <max-nodes> \
  --comparison-operator GreaterThanOrEqualToThreshold

# Pod crash loop detected
aws cloudwatch put-metric-alarm \
  --alarm-name arbiter-pod-crash-loop \
  --alarm-description "Alert when pods are crash looping" \
  --metric-name PodRestartCount \
  --namespace ContainerInsights \
  --statistic Sum \
  --period 300 \
  --evaluation-periods 2 \
  --threshold 5 \
  --comparison-operator GreaterThanThreshold
```

### SNS Notifications

Configure SNS topics for alarm notifications:

```bash
# Create SNS topic
aws sns create-topic --name arbiter-pipeline-alerts

# Subscribe email to topic
aws sns subscribe \
  --topic-arn arn:aws:sns:<region>:<account>:arbiter-pipeline-alerts \
  --protocol email \
  --notification-endpoint your-email@example.com

# Update alarms to send to SNS
aws cloudwatch put-metric-alarm \
  --alarm-name arbiter-high-workflow-failure-rate \
  --alarm-actions arn:aws:sns:<region>:<account>:arbiter-pipeline-alerts \
  ...
```

## Runbooks

### Common Issues

#### Issue: Pods stuck in Pending state

**Symptoms**: Pods remain in Pending state and never start

**Diagnosis**:
```bash
# Check pod status
kubectl describe pod <pod-name> -n <namespace>

# Check node capacity
kubectl describe nodes

# Check resource quotas
kubectl describe resourcequota -n <namespace>
```

**Resolution**:
1. If nodes are at capacity, increase `maxNodes` in cluster configuration
2. If resource quota is exceeded, adjust quota or reduce pod resource requests
3. If no nodes available, check autoscaling configuration

#### Issue: Workflow fails with "permission denied"

**Symptoms**: Workflow fails with AWS permission errors

**Diagnosis**:
```bash
# Check service account
kubectl get serviceaccount <sa-name> -n <namespace> -o yaml

# Check IAM role
aws iam get-role --role-name <role-name>

# Check role policy
aws iam list-attached-role-policies --role-name <role-name>
```

**Resolution**:
1. Verify IRSA is configured correctly (service account has `eks.amazonaws.com/role-arn` annotation)
2. Verify IAM role trust policy allows the service account to assume it
3. Verify IAM role has necessary permissions
4. Check that `AWS_ROLE_ARN` environment variable is set in pod

#### Issue: Container image pull failures

**Symptoms**: Pods fail with "ImagePullBackOff" or "ErrImagePull"

**Diagnosis**:
```bash
# Check pod events
kubectl describe pod <pod-name> -n <namespace>

# Check image exists
docker pull <image-name>:<tag>
```

**Resolution**:
1. Verify image name and tag are correct
2. Verify image exists in registry
3. If using private registry, verify image pull secrets are configured
4. Check node IAM role has ECR permissions (if using ECR)

#### Issue: Cluster autoscaling not working

**Symptoms**: Pods remain pending but cluster doesn't scale up

**Diagnosis**:
```bash
# Check cluster autoscaler logs
kubectl logs -n kube-system deployment/cluster-autoscaler

# Check node group configuration
aws eks describe-nodegroup \
  --cluster-name <cluster-name> \
  --nodegroup-name <nodegroup-name>
```

**Resolution**:
1. Verify node group has autoscaling enabled
2. Verify min/max node counts are correct
3. Check cluster autoscaler has necessary IAM permissions
4. Verify pods have resource requests set (required for autoscaling)

### Troubleshooting

#### Debug a failing workflow

```bash
# Get workflow status
kubectl get workflow <workflow-name> -n <namespace>

# Get workflow details
kubectl describe workflow <workflow-name> -n <namespace>

# Get workflow logs
kubectl logs <workflow-pod-name> -n <namespace>

# Get workflow as YAML
kubectl get workflow <workflow-name> -n <namespace> -o yaml
```

#### Debug IRSA issues

```bash
# Check service account annotations
kubectl get serviceaccount <sa-name> -n <namespace> -o jsonpath='{.metadata.annotations}'

# Check pod environment variables
kubectl exec <pod-name> -n <namespace> -- env | grep AWS

# Check token file exists
kubectl exec <pod-name> -n <namespace> -- ls -la /var/run/secrets/eks.amazonaws.com/serviceaccount/

# Test AWS credentials
kubectl exec <pod-name> -n <namespace> -- aws sts get-caller-identity
```

#### Debug network connectivity

```bash
# Test DNS resolution
kubectl run -it --rm debug --image=busybox --restart=Never -- nslookup kubernetes.default

# Test external connectivity
kubectl run -it --rm debug --image=busybox --restart=Never -- wget -O- https://www.google.com

# Test pod-to-pod connectivity
kubectl run -it --rm debug --image=busybox --restart=Never -- wget -O- http://<service-name>.<namespace>.svc.cluster.local
```

## Maintenance

### Regular Maintenance Tasks

#### Update Kubernetes Version

```bash
# Check current version
kubectl version --short

# Update cluster version (via CDK)
# Edit lib/constructs/aphex-cluster.ts:
# kubernetesVersion: eks.KubernetesVersion.V1_29

# Deploy update
cdk deploy

# Update node group AMI
aws eks update-nodegroup-version \
  --cluster-name <cluster-name> \
  --nodegroup-name <nodegroup-name>
```

#### Update Argo Workflows

```bash
# Check current version
helm list -n argo

# Update Helm chart (via CDK)
# The Helm chart version is managed by CDK
# To update, modify the chart version in lib/constructs/aphex-cluster.ts

# Deploy update
cdk deploy
```

#### Rotate IRSA Credentials

IRSA credentials are automatically rotated by Kubernetes. No manual rotation required.

#### Clean Up Old Artifacts

```bash
# List artifacts in S3
aws s3 ls s3://<artifact-bucket>/artifacts/

# Delete artifacts older than 30 days
aws s3 ls s3://<artifact-bucket>/artifacts/ --recursive | \
  awk '{if ($1 < "'$(date -d '30 days ago' +%Y-%m-%d)'") print $4}' | \
  xargs -I {} aws s3 rm s3://<artifact-bucket>/{}
```

#### Update Container Images

```bash
# Build new images with updated version
./scripts/build-images.sh --version v1.1.0 --push

# Update WorkflowTemplates to use new version
kubectl edit workflowtemplate <template-name> -n <namespace>
# Change image tag from v1.0.0 to v1.1.0
```

### Backup and Recovery

#### Backup Cluster Configuration

```bash
# Export cluster configuration
kubectl get all --all-namespaces -o yaml > cluster-backup.yaml

# Export Argo Workflows templates
kubectl get workflowtemplate -n argo -o yaml > workflow-templates-backup.yaml

# Export Argo Events resources
kubectl get eventsource,sensor -n argo -o yaml > argo-events-backup.yaml
```

#### Disaster Recovery

If the cluster is lost, redeploy using CDK:

```bash
# Deploy new cluster
cdk deploy

# Restore Argo Workflows templates
kubectl apply -f workflow-templates-backup.yaml

# Restore Argo Events resources
kubectl apply -f argo-events-backup.yaml

# Recreate pipelines
# Use the createPipeline() method for each pipeline
```

### Cost Optimization

#### Monitor Costs

```bash
# View EKS cluster costs
aws ce get-cost-and-usage \
  --time-period Start=2024-01-01,End=2024-01-31 \
  --granularity MONTHLY \
  --metrics BlendedCost \
  --filter file://eks-filter.json

# eks-filter.json:
{
  "Tags": {
    "Key": "aws:eks:cluster-name",
    "Values": ["<cluster-name>"]
  }
}
```

#### Reduce Costs

1. **Use Spot Instances**: Add spot instance node groups for non-critical workloads
2. **Right-size Nodes**: Use smaller instance types if workloads allow
3. **Reduce Min Nodes**: Lower `minNodes` during off-peak hours
4. **Enable Cluster Autoscaler**: Ensure autoscaling is working to scale down unused nodes
5. **Clean Up Unused Resources**: Delete unused pipelines and namespaces

**Source**
- `lib/constructs/aphex-cluster.ts`
- `lib/arbiter-pipeline-infrastructure-stack.ts`
- `scripts/build-images.sh`
- `README.md`
- `.kiro/specs/arbiter-pipeline-infrastructure/design.md`
