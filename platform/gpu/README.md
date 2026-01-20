# GPU Infrastructure

This directory contains Kubernetes resources for enabling GPU workloads on the Aphex platform.

## RuntimeClass

The `nvidia` RuntimeClass enables pods to access NVIDIA GPUs via the nvidia container runtime handler.

### Prerequisites

Before deploying GPU workloads, ensure the following are installed on your cluster nodes:

1. **NVIDIA Drivers**: GPU drivers must be installed on the host
2. **NVIDIA Container Toolkit**: Enables container runtimes to access GPUs
3. **NVIDIA Device Plugin**: Exposes GPUs as schedulable resources in Kubernetes

### Installing NVIDIA Device Plugin

If not already deployed, install the NVIDIA Device Plugin:

```bash
kubectl create -f https://raw.githubusercontent.com/NVIDIA/k8s-device-plugin/v0.14.0/nvidia-device-plugin.yml
```

### Usage

To use GPU resources in a pod, specify the RuntimeClass and request GPU resources:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: gpu-workload
spec:
  runtimeClassName: nvidia
  containers:
  - name: gpu-container
    image: nvidia/cuda:12.0-base
    resources:
      requests:
        nvidia.com/gpu: "1"
      limits:
        nvidia.com/gpu: "1"
```

### Verification

Verify the RuntimeClass is available:

```bash
kubectl get runtimeclass nvidia
```

Verify GPU resources are available on nodes:

```bash
kubectl describe nodes | grep -A5 "Allocatable:" | grep nvidia
```

## ArgoCD Sync

This directory is synced by the `platform-gpu` ArgoCD Application with sync-wave 3, ensuring it's deployed before any GPU-dependent workloads.

**Source**
- `.kiro/specs/model-server-infrastructure/design.md` - Model server infrastructure design
- `.kiro/specs/model-server-infrastructure/requirements.md` - Requirements 4.1, 4.2, 4.4
