# Redis Cache for Authentik

Redis provides caching and session storage for Authentik.

## Deployment

Redis is deployed as a single-replica Deployment with a ClusterIP Service.

**Deployment Specification:**
- Namespace: `auth-system`
- Image: `redis:7-alpine`
- Replicas: 1
- Resource Requests: 100m CPU, 128Mi memory
- Resource Limits: 500m CPU, 256Mi memory

**Service Specification:**
- Type: ClusterIP
- Port: 6379
- DNS Name: `redis.auth-system.svc.cluster.local`

## Configuration

Authentik connects to Redis using the `AUTHENTIK_REDIS__HOST` environment variable:

```yaml
env:
  - name: AUTHENTIK_REDIS__HOST
    value: redis.auth-system.svc.cluster.local
```

## Notes

- Redis is required by Authentik (not optional)
- No persistence configured (cache data is ephemeral)
- For production, consider Redis Sentinel or Redis Cluster for high availability
