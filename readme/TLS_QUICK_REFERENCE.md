# TLS Quick Reference

## Authentication Modes Comparison

| Mode | Security | Certificates Required | Use Case |
|------|----------|----------------------|----------|
| **Full Certificate** | ⭐⭐⭐⭐⭐ | CA + Client Cert + Key | Production, mTLS |
| **CA + Password** | ⭐⭐⭐⭐ | CA only | Server verification + password |
| **Password-Only** | ⭐⭐⭐ | None | Encrypted connection, no verification |
| **No SSL** | ⭐ | None | Development only |

## Configuration Quick Reference

### Full Certificate Authentication
```bash
SSL=ON
DB_SSL_MODE=verify-full
CLIENT_AUTH_MODE=cert
SSL_ROOT_CERT=clients/postgres/certificates/ca.crt
SSL_CERT=clients/postgres/certificates/tls.crt
SSL_KEY=clients/postgres/certificates/tls.key
```

### CA + Password Authentication
```bash
SSL=ON
DB_SSL_MODE=verify-full
CLIENT_AUTH_MODE=scram
SSL_ROOT_CERT=clients/postgres/certificates/ca.crt
SSL_CERT=
SSL_KEY=
```

### Password-Only Authentication
```bash
SSL=ON
DB_SSL_MODE=require
CLIENT_AUTH_MODE=md5
SSL_ROOT_CERT=
SSL_CERT=
SSL_KEY=
```

### No SSL
```bash
SSL=OFF
DB_SSL_MODE=disable
```

## Kubernetes Deployment Commands

### Full Certificate
```bash
./deploy-k8s-tls.sh
kubectl logs -f job/pg-load-test-tls -n demo
./cleanup-k8s-tls.sh
```

### CA + Password
```bash
kubectl apply -f k8s-tls/02-secret.yaml
kubectl apply -f k8s-tls/04-ca-password-mode.yaml
kubectl logs -f job/pg-load-test-tls-ca-password -n demo
```

### Password-Only
```bash
kubectl apply -f k8s-tls/02-secret.yaml
kubectl apply -f k8s-tls/05-password-only-mode.yaml
kubectl logs -f job/pg-load-test-tls-password-only -n demo
```

## Local Testing Commands

```bash
# Full certificate
cp .env.tls .env && go build -o load-test-client-v2 . && ./load-test-client-v2

# CA + password
cp .env.tls-ca-password .env && go build -o load-test-client-v2 . && ./load-test-client-v2

# Password-only
cp .env.tls-password-only .env && go build -o load-test-client-v2 . && ./load-test-client-v2
```

## SSL Mode Values

| Mode | Encryption | CA Verification | Hostname Verification |
|------|------------|-----------------|----------------------|
| `disable` | ❌ | ❌ | ❌ |
| `require` | ✅ | ❌ | ❌ |
| `verify-ca` | ✅ | ✅ | ❌ |
| `verify-full` | ✅ | ✅ | ✅ |

## Troubleshooting Checklist

- [ ] Check SSL is enabled: `SSL=ON`
- [ ] Verify SSL mode: `DB_SSL_MODE=verify-full`
- [ ] Confirm certificates exist:
  ```bash
  ls -la clients/postgres/certificates/
  ```
- [ ] Check certificate permissions:
  ```bash
  chmod 600 clients/postgres/certificates/tls.key
  ```
- [ ] Verify K8s secret exists:
  ```bash
  kubectl get secret pg-ha-cluster-client-cert -n demo
  ```
- [ ] Check connection string in logs:
  ```bash
  kubectl logs job/pg-load-test-tls -n demo | grep -i ssl
  ```

## Common Errors

| Error | Cause | Solution |
|-------|-------|----------|
| `x509: certificate signed by unknown authority` | CA cert missing/invalid | Check `SSL_ROOT_CERT` path |
| `server requires client certificate` | Client cert not provided | Use `CLIENT_AUTH_MODE=cert` |
| `no pg_hba.conf entry` | Auth method not allowed | Update PostgreSQL `pg_hba.conf` |
| `private key has group access` | Wrong file permissions | `chmod 600 tls.key` |

## File Structure

```
clients/postgres/certificates/
├── ca.crt       # Certificate Authority
├── tls.crt      # Client certificate
└── tls.key      # Client private key (mode 600)

k8s-tls/
├── 01-configmap.yaml              # Full cert config
├── 02-secret.yaml                 # DB credentials
├── 03-job.yaml                    # Full cert job
├── 04-ca-password-mode.yaml       # CA + password
├── 05-password-only-mode.yaml     # Password only
└── README.md

.env.tls                  # Full cert local config
.env.tls-ca-password      # CA + password local config
.env.tls-password-only    # Password-only local config
deploy-k8s-tls.sh         # Deploy script
cleanup-k8s-tls.sh        # Cleanup script
```

## Connection String Examples

### Full Certificate
```
host=pg-ha-cluster.demo.svc.cluster.local port=5432 
user=postgres password=*** dbname=postgres connect_timeout=30 
sslmode=verify-full 
sslrootcert=/tls/certs/ca.crt 
sslcert=/tls/certs/tls.crt 
sslkey=/tls/certs/tls.key
```

### CA + Password
```
host=pg-ha-cluster.demo.svc.cluster.local port=5432 
user=postgres password=*** dbname=postgres connect_timeout=30 
sslmode=verify-full 
sslrootcert=/tls/certs/ca.crt
```

### Password-Only
```
host=pg-ha-cluster.demo.svc.cluster.local port=5432 
user=postgres password=*** dbname=postgres connect_timeout=30 
sslmode=require
```

## See Full Documentation

- **[TLS_AUTHENTICATION_GUIDE.md](TLS_AUTHENTICATION_GUIDE.md)** - Complete guide
- **[k8s-tls/README.md](k8s-tls/README.md)** - Kubernetes manifests reference
