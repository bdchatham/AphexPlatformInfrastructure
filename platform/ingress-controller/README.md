# Ingress Controller

This directory contains the nginx-ingress controller configuration for the platform.

## Purpose

The Ingress controller provides external access to platform services via HTTP/HTTPS with:
- TLS termination
- Host-based routing
- Load balancing

## Deployment

The Ingress controller is deployed automatically by ArgoCD as part of the platform bootstrap.

**Sync Wave: 0** (deployed first, before all other platform components)

## Configuration

The Ingress controller is deployed using the official nginx-ingress manifests optimized for Kind clusters.

For production deployments, consider:
- Using a cloud provider's load balancer integration
- Configuring cert-manager for automatic TLS certificate management
- Adjusting resource limits based on traffic patterns

## Verification

Check Ingress controller status:
```bash
kubectl get pods -n ingress-nginx
kubectl get svc -n ingress-nginx
```

Get Ingress controller IP (for DNS configuration):
```bash
kubectl get svc ingress-nginx-controller -n ingress-nginx -o jsonpath='{.status.loadBalancer.ingress[0].ip}'
```

## Source

- Official nginx-ingress: https://kubernetes.github.io/ingress-nginx/
- Kind-specific deployment: https://kind.sigs.k8s.io/docs/user/ingress/
