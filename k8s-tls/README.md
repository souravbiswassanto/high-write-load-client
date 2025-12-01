# Kubernetes TLS Deployment Manifests

This directory contains Kubernetes manifests for deploying the PostgreSQL load testing client with TLS/SSL authentication.

## Files

### Core Manifests (Full Certificate Authentication)

- **01-configmap.yaml** - Configuration for full certificate authentication
- **02-secret.yaml** - Database credentials (user/password)
- **03-job.yaml** - Job definition with TLS certificate volume mounts

### Alternative Authentication Modes

- **04-ca-password-mode.yaml** - CA certificate + password authentication
- **05-password-only-mode.yaml** - Password-only authentication (no certificates)

## Quick Start

### Full Certificate Authentication (Recommended)

From the project root:
```bash
./deploy-k8s-tls.sh
```

Or manually:
```bash
kubectl apply -f k8s-tls/01-configmap.yaml
kubectl apply -f k8s-tls/02-secret.yaml
kubectl apply -f k8s-tls/03-job.yaml
```

Monitor the job:
```bash
kubectl logs -f job/pg-load-test-tls -n demo
```

### CA + Password Authentication

```bash
kubectl apply -f k8s-tls/02-secret.yaml
kubectl apply -f k8s-tls/04-ca-password-mode.yaml
kubectl logs -f job/pg-load-test-tls-ca-password -n demo
```

### Password-Only Authentication

```bash
kubectl apply -f k8s-tls/02-secret.yaml
kubectl apply -f k8s-tls/05-password-only-mode.yaml
kubectl logs -f job/pg-load-test-tls-password-only -n demo
```

## Prerequisites

1. **Namespace**: The `demo` namespace must exist
2. **PostgreSQL Cluster**: Running in the `demo` namespace
3. **TLS Secret**: `pg-ha-cluster-client-cert` must exist with these keys:
   - `ca.crt` - CA certificate
   - `tls.crt` - Client certificate
   - `tls.key` - Client private key

## Configuration

### ConfigMap Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DB_HOST` | `pg-ha-cluster.demo.svc.cluster.local` | PostgreSQL service hostname |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_NAME` | `postgres` | Database name |
| `SSL` | `ON` | Enable SSL/TLS |
| `DB_SSL_MODE` | `verify-full` | SSL verification mode |
| `CLIENT_AUTH_MODE` | `cert` | Authentication mode (cert/scram/md5) |
| `SSL_ROOT_CERT` | `/tls/certs/ca.crt` | Path to CA cert in container |
| `SSL_CERT` | `/tls/certs/tls.crt` | Path to client cert in container |
| `SSL_KEY` | `/tls/certs/tls.key` | Path to client key in container |
| `CONCURRENT_WRITERS` | `100` | Number of concurrent workers |
| `TEST_RUN_DURATION` | `500` | Test duration in seconds |
| `INSERT_PERCENT` | `60` | Percentage of insert operations |
| `UPDATE_PERCENT` | `25` | Percentage of update operations |
| `READ_PERCENT` | `15` | Percentage of read operations |

### Secret Variables

| Variable | Description |
|----------|-------------|
| `DB_USER` | PostgreSQL username |
| `DB_PASSWORD` | PostgreSQL password |

## Volume Mounts

### Full Certificate Authentication (03-job.yaml)

```yaml
volumeMounts:
- name: tls-certs
  mountPath: /tls/certs
  readOnly: true

volumes:
- name: tls-certs
  secret:
    secretName: pg-ha-cluster-client-cert
    items:
    - key: ca.crt
      path: ca.crt
    - key: tls.crt
      path: tls.crt
    - key: tls.key
      path: tls.key
      mode: 0600
```

### CA + Password Authentication (04-ca-password-mode.yaml)

Only CA certificate is mounted:
```yaml
volumeMounts:
- name: tls-ca-cert
  mountPath: /tls/certs
  readOnly: true

volumes:
- name: tls-ca-cert
  secret:
    secretName: pg-ha-cluster-client-cert
    items:
    - key: ca.crt
      path: ca.crt
```

### Password-Only Authentication (05-password-only-mode.yaml)

No volume mounts - password authentication only.

## Resource Limits

All jobs are configured with:
```yaml
resources:
  requests:
    memory: "2Gi"
    cpu: "1000m"
  limits:
    memory: "2Gi"
    cpu: "2000m"
```

Adjust based on your workload requirements.

## Job Settings

- **activeDeadlineSeconds**: 1800 (30 minutes max runtime)
- **backoffLimit**: 0 (no retries)
- **restartPolicy**: Never

## Cleanup

From the project root:
```bash
./cleanup-k8s-tls.sh
```

Or manually:
```bash
# Full cert auth
kubectl delete job pg-load-test-tls -n demo
kubectl delete configmap pg-load-test-config-tls -n demo

# CA + password auth
kubectl delete job pg-load-test-tls-ca-password -n demo
kubectl delete configmap pg-load-test-config-tls-ca-password -n demo

# Password-only auth
kubectl delete job pg-load-test-tls-password-only -n demo
kubectl delete configmap pg-load-test-config-tls-password-only -n demo

# Shared secret
kubectl delete secret pg-load-test-secret-tls -n demo
```

**Note:** The TLS certificates secret (`pg-ha-cluster-client-cert`) is managed by the PostgreSQL operator and should not be deleted.

## Monitoring

### View Job Status
```bash
kubectl get jobs -n demo
kubectl describe job pg-load-test-tls -n demo
```

### View Pod Status
```bash
kubectl get pods -n demo -l app=pg-load-test-tls
kubectl describe pod <pod-name> -n demo
```

### View Logs
```bash
# Follow logs
kubectl logs -f job/pg-load-test-tls -n demo

# Previous logs (if pod restarted)
kubectl logs -p job/pg-load-test-tls -n demo
```

### Debug Connection Issues
```bash
# Check if certificates are mounted correctly
kubectl exec -it <pod-name> -n demo -- ls -la /tls/certs

# Verify certificate contents
kubectl exec -it <pod-name> -n demo -- cat /tls/certs/ca.crt

# Check environment variables
kubectl exec -it <pod-name> -n demo -- env | grep -E '(SSL|DB_)'
```

## Troubleshooting

### Pod Not Starting
```bash
kubectl describe pod <pod-name> -n demo
```
Check for:
- Image pull errors
- Secret/ConfigMap not found
- Resource constraints

### TLS Connection Errors

**"x509: certificate signed by unknown authority"**
- Verify CA certificate is correct
- Check `SSL_ROOT_CERT` path

**"server requires client certificate"**
- Use full certificate authentication mode
- Ensure client cert/key are mounted

**"no pg_hba.conf entry"**
- PostgreSQL server doesn't accept this auth method
- Check PostgreSQL `pg_hba.conf` configuration

### Certificate Permission Errors

**"private key file has group or world access"**
- Set `mode: 0600` in secret volume mount (already configured)

## See Also

- [TLS_AUTHENTICATION_GUIDE.md](../TLS_AUTHENTICATION_GUIDE.md) - Comprehensive TLS setup guide
- [KUBERNETES_DEPLOYMENT.md](../KUBERNETES_DEPLOYMENT.md) - Non-TLS Kubernetes deployment
- [README.md](../README.md) - Project overview
