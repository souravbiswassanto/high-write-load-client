# TLS/SSL Authentication Guide

This guide explains how to use the PostgreSQL load testing client with different TLS/SSL authentication modes.

## Overview

The client supports **three authentication modes** when connecting to PostgreSQL:

1. **Full Certificate Authentication** - Client cert + CA verification (most secure)
2. **CA + Password Authentication** - CA verification with password auth (scram/md5)
3. **Password-Only Authentication** - Password over encrypted connection (no CA verification)

## Certificate Files

The TLS certificates are stored in `clients/postgres/certificates/`:

- `ca.crt` - Certificate Authority (CA) certificate
- `tls.crt` - Client certificate
- `tls.key` - Client private key

These certificates are extracted from the Kubernetes secret `pg-ha-cluster-client-cert` in the `demo` namespace.

## Configuration

### Environment Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `SSL` | Enable/disable SSL | `ON` or `OFF` |
| `DB_SSL_MODE` | SSL verification mode | `disable`, `require`, `verify-ca`, `verify-full` |
| `CLIENT_AUTH_MODE` | Authentication method | `cert`, `scram`, `md5`, or empty |
| `SSL_ROOT_CERT` | Path to CA certificate | `clients/postgres/certificates/ca.crt` |
| `SSL_CERT` | Path to client certificate | `clients/postgres/certificates/tls.crt` |
| `SSL_KEY` | Path to client private key | `clients/postgres/certificates/tls.key` |

### SSL Modes Explained

- **`disable`** - No SSL encryption (default)
- **`require`** - SSL encryption without identity verification
- **`verify-ca`** - SSL encryption + verify server has trusted certificate
- **`verify-full`** - SSL encryption + verify server identity matches hostname

## Authentication Modes

### 1. Full Certificate Authentication

**Most secure option** - Client authenticates with certificate + CA verification.

**Environment Configuration:**
```bash
SSL=ON
DB_SSL_MODE=verify-full
CLIENT_AUTH_MODE=cert
SSL_ROOT_CERT=clients/postgres/certificates/ca.crt
SSL_CERT=clients/postgres/certificates/tls.crt
SSL_KEY=clients/postgres/certificates/tls.key
```

**Connection String:**
```
host=... port=5432 user=postgres password=... dbname=postgres 
connect_timeout=30 sslmode=verify-full 
sslrootcert=clients/postgres/certificates/ca.crt 
sslcert=clients/postgres/certificates/tls.crt 
sslkey=clients/postgres/certificates/tls.key
```

**Use Case:**
- Production environments
- Mutual TLS (mTLS) requirements
- Maximum security

**Example File:** `.env.tls`

---

### 2. CA + Password Authentication

**Moderate security** - Password authentication over SSL with CA verification.

**Environment Configuration:**
```bash
SSL=ON
DB_SSL_MODE=verify-full
CLIENT_AUTH_MODE=scram  # or md5
SSL_ROOT_CERT=clients/postgres/certificates/ca.crt
SSL_CERT=
SSL_KEY=
```

**Connection String:**
```
host=... port=5432 user=postgres password=... dbname=postgres 
connect_timeout=30 sslmode=verify-full 
sslrootcert=clients/postgres/certificates/ca.crt
```

**Use Case:**
- Environments where client certificates are not available
- Server identity verification is required
- Standard password authentication

**Example File:** `.env.tls-ca-password`

---

### 3. Password-Only Authentication

**Basic security** - Password over encrypted connection without CA verification.

**Environment Configuration:**
```bash
SSL=ON
DB_SSL_MODE=require
CLIENT_AUTH_MODE=md5  # or scram
SSL_ROOT_CERT=
SSL_CERT=
SSL_KEY=
```

**Connection String:**
```
host=... port=5432 user=postgres password=... dbname=postgres 
connect_timeout=30 sslmode=require
```

**Use Case:**
- Development/testing environments
- Encryption required but certificate management is impractical
- Self-signed certificates without CA

**Example File:** `.env.tls-password-only`

---

## Local Testing

### 1. Full Certificate Authentication
```bash
# Copy the TLS environment file
cp .env.tls .env

# Build and run
go build -o load-test-client-v2 .
./load-test-client-v2
```

### 2. CA + Password Authentication
```bash
cp .env.tls-ca-password .env
go build -o load-test-client-v2 .
./load-test-client-v2
```

### 3. Password-Only Authentication
```bash
cp .env.tls-password-only .env
go build -o load-test-client-v2 .
./load-test-client-v2
```

### 4. No SSL (Development)
```bash
# Use default .env with SSL=OFF
SSL=OFF
DB_SSL_MODE=disable
```

---

## Kubernetes Deployment

### Prerequisites
1. PostgreSQL cluster running in `demo` namespace
2. Secret `pg-ha-cluster-client-cert` exists with TLS certificates

### Full Certificate Authentication

**One-command deployment:**
```bash
./deploy-k8s-tls.sh
```

**Manual deployment:**
```bash
kubectl apply -f k8s-tls/01-configmap.yaml
kubectl apply -f k8s-tls/02-secret.yaml
kubectl apply -f k8s-tls/03-job.yaml
```

**Monitor:**
```bash
kubectl logs -f job/pg-load-test-tls -n demo
```

---

### CA + Password Authentication

```bash
kubectl apply -f k8s-tls/02-secret.yaml
kubectl apply -f k8s-tls/04-ca-password-mode.yaml
kubectl logs -f job/pg-load-test-tls-ca-password -n demo
```

---

### Password-Only Authentication

```bash
kubectl apply -f k8s-tls/02-secret.yaml
kubectl apply -f k8s-tls/05-password-only-mode.yaml
kubectl logs -f job/pg-load-test-tls-password-only -n demo
```

---

## Cleanup

### TLS Deployment
```bash
./cleanup-k8s-tls.sh
```

### Manual Cleanup
```bash
kubectl delete job pg-load-test-tls -n demo
kubectl delete job pg-load-test-tls-ca-password -n demo
kubectl delete job pg-load-test-tls-password-only -n demo
kubectl delete configmap pg-load-test-config-tls -n demo
kubectl delete configmap pg-load-test-config-tls-ca-password -n demo
kubectl delete configmap pg-load-test-config-tls-password-only -n demo
kubectl delete secret pg-load-test-secret-tls -n demo
```

**Note:** The secret `pg-ha-cluster-client-cert` is preserved as it's managed by the PostgreSQL cluster.

---

## Troubleshooting

### Certificate Verification Failed
```
Error: x509: certificate signed by unknown authority
```
**Solution:** Ensure `SSL_ROOT_CERT` points to the correct CA certificate.

### Client Certificate Required
```
Error: server requires client certificate
```
**Solution:** Use `CLIENT_AUTH_MODE=cert` and provide client certificate paths.

### SSL Connection Failed
```
Error: SSL connection failed
```
**Solution:** Check that PostgreSQL server has SSL enabled and accepts the specified SSL mode.

### Certificate Permissions
```
Error: private key file has group or world access
```
**Solution:** Set proper permissions on key file:
```bash
chmod 600 clients/postgres/certificates/tls.key
```

### Wrong Authentication Mode
```
Error: no pg_hba.conf entry for host
```
**Solution:** Verify that PostgreSQL `pg_hba.conf` allows the authentication method (cert, scram, md5).

---

## Security Best Practices

1. **Use Full Certificate Authentication in Production**
   - Provides strongest security
   - Prevents man-in-the-middle attacks
   - Verifies both client and server identity

2. **Protect Private Keys**
   ```bash
   chmod 600 clients/postgres/certificates/tls.key
   ```

3. **Use Environment Variables for Secrets**
   - Never commit passwords to version control
   - Use Kubernetes secrets for sensitive data

4. **Rotate Certificates Regularly**
   - Certificates have expiration dates
   - Plan for certificate renewal

5. **Verify Server Identity**
   - Use `verify-full` in production
   - Ensures connection to legitimate server

6. **Use Strong Password Authentication**
   - Prefer SCRAM-SHA-256 over MD5
   - Set `CLIENT_AUTH_MODE=scram`

---

## Connection String Decision Tree

```
Is SSL Required?
├── NO  → sslmode=disable
└── YES
    ├── Do you have client certificates?
    │   ├── YES
    │   │   └── Full Cert Auth
    │   │       sslmode=verify-full + sslcert + sslkey + sslrootcert
    │   └── NO
    │       ├── Do you have CA certificate?
    │       │   ├── YES
    │       │   │   └── CA + Password Auth
    │       │   │       sslmode=verify-full + sslrootcert
    │       │   └── NO
    │       │       └── Password-Only Auth
    │       │           sslmode=require (no certs)
```

---

## Example Output

Successful TLS connection:
```
🔗 Connecting to PostgreSQL...
Database: postgres@pg-ha-cluster.demo.svc.cluster.local:5432
SSL Mode: verify-full
Auth Mode: cert
✅ Successfully connected to database
Connection pool: max_open=60, max_idle=10, min_free=5
TLS: Enabled with client certificate authentication
```

---

## References

- [PostgreSQL SSL Documentation](https://www.postgresql.org/docs/current/ssl-tcp.html)
- [libpq SSL Support](https://www.postgresql.org/docs/current/libpq-ssl.html)
- [Go pq Driver SSL](https://pkg.go.dev/github.com/lib/pq#hdr-Connection_String_Parameters)
