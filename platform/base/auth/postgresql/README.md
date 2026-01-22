# PostgreSQL for Authentik

This directory contains the PostgreSQL database deployment for Authentik authentication system.

## Components

- **statefulset.yaml**: PostgreSQL StatefulSet with persistent storage
- **service.yaml**: ClusterIP service for database access
- **secret.yaml.example**: Example secret template (not committed to Git)

## Database Configuration

- **Image**: postgres:15-alpine
- **Database Name**: authentik
- **User**: authentik
- **Storage**: 10Gi persistent volume
- **Resources**: 250m CPU / 256Mi memory (requests), 1000m CPU / 512Mi memory (limits)

## Secret Management

PostgreSQL credentials are stored in a Kubernetes Secret named `authentik-postgresql`.

### Manual Secret Creation

Generate and create the secret manually:

```bash
kubectl create secret generic authentik-postgresql \
  --from-literal=postgresql-password=$(openssl rand -base64 32) \
  --from-literal=postgresql-postgres-password=$(openssl rand -base64 32) \
  --namespace=auth-system
```

### Bootstrap Script

The bootstrap script (`platform/bootstrap/bootstrap.sh`) automatically generates and creates this secret during initial cluster setup.

## Service DNS

The PostgreSQL service is accessible within the cluster at:

```
postgresql.auth-system.svc.cluster.local:5432
```

## Health Checks

- **Liveness Probe**: `pg_isready -U authentik` every 10 seconds
- **Readiness Probe**: `pg_isready -U authentik` every 5 seconds

## Deployment

This PostgreSQL instance is deployed via:

1. **Bootstrap**: Initial deployment during cluster setup
2. **ArgoCD**: Ongoing management and configuration sync

## Backup and Recovery

**Important**: This is a basic PostgreSQL deployment suitable for homelab/development environments. For production use, consider:

- Regular database backups using `pg_dump` or volume snapshots
- Point-in-time recovery configuration
- High availability with replication
- Automated backup scheduling

### Manual Backup

```bash
# Backup database
kubectl exec -n auth-system postgresql-0 -- \
  pg_dump -U authentik authentik > authentik-backup.sql

# Restore database
kubectl exec -i -n auth-system postgresql-0 -- \
  psql -U authentik authentik < authentik-backup.sql
```

## Troubleshooting

### Pod Not Starting

Check pod logs:
```bash
kubectl logs -n auth-system postgresql-0
```

Common issues:
- Missing secret `authentik-postgresql`
- Insufficient storage
- PVC not bound

### Connection Issues

Verify service is running:
```bash
kubectl get svc -n auth-system postgresql
kubectl get pods -n auth-system -l app=postgresql
```

Test connection from another pod:
```bash
kubectl run -it --rm debug --image=postgres:15-alpine --restart=Never -- \
  psql -h postgresql.auth-system.svc.cluster.local -U authentik -d authentik
```

### Data Persistence

Check PVC status:
```bash
kubectl get pvc -n auth-system
```

The data is stored in the PVC `postgresql-data-postgresql-0`.

## Source

- Design: `.kiro/specs/dex-authentication-platform/design.md`
- Requirements: `.kiro/specs/dex-authentication-platform/requirements.md` (Requirements 1.2, 14.1, 14.2, 14.4)
