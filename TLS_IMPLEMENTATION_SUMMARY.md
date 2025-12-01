# TLS/SSL Implementation Summary

## Overview

Added complete TLS/SSL support for PostgreSQL load testing client with three authentication modes:

1. **Full Certificate Authentication** - Client cert + CA verification (most secure)
2. **CA + Password Authentication** - CA verification with password (scram/md5)
3. **Password-Only Authentication** - Password over encrypted connection

## Changes Made

### 1. Code Changes

#### `config/config.go`
- Added SSL configuration fields to `DBConfig`:
  - `SSLEnabled` - Enable/disable SSL
  - `ClientAuthMode` - Authentication mode (cert/scram/md5)
  - `SSLRootCert`, `SSLCert`, `SSLKey` - Certificate paths
- Enhanced `GetConnectionString()` with TLS logic:
  - Full certificate auth: `sslmode=verify-full` + all certs
  - CA + password: `sslmode=verify-full` + CA cert only
  - Password-only: `sslmode=require` + no certs
  - No SSL: `sslmode=disable`

### 2. Certificate Files

#### `clients/postgres/certificates/`
- `ca.crt` - Extracted from K8s secret `pg-ha-cluster-client-cert`
- `tls.crt` - Client certificate
- `tls.key` - Client private key (mode 600)

Command used to extract:
```bash
kubectl get secret pg-ha-cluster-client-cert -n demo -o yaml
# Decoded base64 values and saved to files
```

### 3. Configuration Files

#### Local Environment Files
- `.env` - Updated with SSL configuration options
- `.env.tls` - Full certificate authentication
- `.env.tls-ca-password` - CA + password authentication
- `.env.tls-password-only` - Password-only authentication

#### Kubernetes Manifests (`k8s-tls/`)
- `01-configmap.yaml` - Full cert auth config
- `02-secret.yaml` - Database credentials
- `03-job.yaml` - Job with TLS volume mounts
- `04-ca-password-mode.yaml` - CA + password mode
- `05-password-only-mode.yaml` - Password-only mode

### 4. Deployment Scripts

#### `deploy-k8s-tls.sh`
- One-command deployment for TLS mode
- Validates namespace and secret existence
- Cleans up existing resources
- Applies all manifests
- Shows monitoring commands

#### `cleanup-k8s-tls.sh`
- Cleans up TLS deployment resources
- Preserves PostgreSQL-managed certificates

### 5. Documentation

#### `TLS_AUTHENTICATION_GUIDE.md`
Comprehensive guide covering:
- Authentication mode comparison
- Environment variable reference
- SSL mode explanations
- Local testing instructions
- Kubernetes deployment guide
- Troubleshooting section
- Security best practices
- Decision tree for choosing mode

#### `k8s-tls/README.md`
Kubernetes-specific guide:
- Manifest descriptions
- Quick start commands
- Configuration reference
- Volume mount details
- Monitoring commands
- Debug procedures

#### `TLS_QUICK_REFERENCE.md`
Quick lookup guide:
- Mode comparison table
- Configuration snippets
- Command cheat sheet
- Common errors table
- File structure overview

## Authentication Modes

### Mode 1: Full Certificate Authentication
**Environment:**
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
sslmode=verify-full sslrootcert=... sslcert=... sslkey=...
```

**Use Case:** Production, mTLS, maximum security

---

### Mode 2: CA + Password Authentication
**Environment:**
```bash
SSL=ON
DB_SSL_MODE=verify-full
CLIENT_AUTH_MODE=scram
SSL_ROOT_CERT=clients/postgres/certificates/ca.crt
```

**Connection String:**
```
sslmode=verify-full sslrootcert=...
```

**Use Case:** Server verification + password auth

---

### Mode 3: Password-Only Authentication
**Environment:**
```bash
SSL=ON
DB_SSL_MODE=require
CLIENT_AUTH_MODE=md5
```

**Connection String:**
```
sslmode=require
```

**Use Case:** Encrypted connection without certificate verification

---

## Usage Examples

### Local Testing

**Full certificate:**
```bash
cp .env.tls .env
go build -o load-test-client-v2 .
./load-test-client-v2
```

**CA + password:**
```bash
cp .env.tls-ca-password .env
go build -o load-test-client-v2 .
./load-test-client-v2
```

**Password-only:**
```bash
cp .env.tls-password-only .env
go build -o load-test-client-v2 .
./load-test-client-v2
```

### Kubernetes Deployment

**Full certificate:**
```bash
./deploy-k8s-tls.sh
kubectl logs -f job/pg-load-test-tls -n demo
```

**CA + password:**
```bash
kubectl apply -f k8s-tls/02-secret.yaml
kubectl apply -f k8s-tls/04-ca-password-mode.yaml
kubectl logs -f job/pg-load-test-tls-ca-password -n demo
```

**Password-only:**
```bash
kubectl apply -f k8s-tls/02-secret.yaml
kubectl apply -f k8s-tls/05-password-only-mode.yaml
kubectl logs -f job/pg-load-test-tls-password-only -n demo
```

## Volume Mounts in Kubernetes

### Full Certificate Mode
Mounts all three certificate files from `pg-ha-cluster-client-cert` secret:
```yaml
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

### CA + Password Mode
Mounts only CA certificate:
```yaml
volumes:
- name: tls-ca-cert
  secret:
    secretName: pg-ha-cluster-client-cert
    items:
    - key: ca.crt
      path: ca.crt
```

### Password-Only Mode
No volume mounts required.

## Security Considerations

1. **Private Key Permissions**
   - Local: `chmod 600 clients/postgres/certificates/tls.key`
   - K8s: Set `mode: 0600` in volume mount

2. **Certificate Verification**
   - `verify-full` verifies server identity and hostname
   - `verify-ca` only verifies certificate chain
   - `require` encrypts but doesn't verify identity

3. **Authentication Methods**
   - `cert` - Strongest, requires client certificate
   - `scram` - Strong password hashing (recommended)
   - `md5` - Legacy, less secure

4. **Best Practices**
   - Use full certificate authentication in production
   - Rotate certificates regularly
   - Never commit passwords to version control
   - Use Kubernetes secrets for sensitive data

## Testing Verification

### Build Verification
```bash
go build -o load-test-client-v2 .
# ✅ Successful - no compilation errors
```

### Certificate Extraction
```bash
ls -la clients/postgres/certificates/
# ✅ ca.crt, tls.crt, tls.key exist
```

### Kubernetes Secret Validation
```bash
kubectl get secret pg-ha-cluster-client-cert -n demo
# ✅ Secret exists with ca.crt, tls.crt, tls.key keys
```

## Backward Compatibility

All changes are **backward compatible**:

- Default configuration remains `SSL=OFF`, `DB_SSL_MODE=disable`
- Existing `.env` file continues to work without SSL
- Non-TLS Kubernetes deployment (`deploy-k8s.sh`) unchanged
- Old behavior preserved when SSL is disabled

## File Structure

```
├── clients/postgres/certificates/
│   ├── ca.crt                      # NEW: CA certificate
│   ├── tls.crt                     # NEW: Client certificate
│   └── tls.key                     # NEW: Client private key
├── k8s-tls/                        # NEW: TLS manifests directory
│   ├── 01-configmap.yaml           # NEW: Full cert config
│   ├── 02-secret.yaml              # NEW: DB credentials
│   ├── 03-job.yaml                 # NEW: Full cert job
│   ├── 04-ca-password-mode.yaml    # NEW: CA + password
│   ├── 05-password-only-mode.yaml  # NEW: Password only
│   └── README.md                   # NEW: K8s TLS guide
├── config/config.go                # MODIFIED: Added SSL config
├── .env                            # MODIFIED: Added SSL options
├── .env.tls                        # NEW: Full cert example
├── .env.tls-ca-password            # NEW: CA + password example
├── .env.tls-password-only          # NEW: Password-only example
├── deploy-k8s-tls.sh               # NEW: TLS deployment script
├── cleanup-k8s-tls.sh              # NEW: TLS cleanup script
├── TLS_AUTHENTICATION_GUIDE.md     # NEW: Comprehensive guide
├── TLS_QUICK_REFERENCE.md          # NEW: Quick reference
└── TLS_IMPLEMENTATION_SUMMARY.md   # NEW: This file
```

## Next Steps

### For Users

1. **Choose authentication mode** based on security requirements
2. **Extract certificates** from PostgreSQL cluster (if using cert auth)
3. **Configure environment** with appropriate `.env` file
4. **Test locally** before Kubernetes deployment
5. **Deploy to Kubernetes** using `deploy-k8s-tls.sh`

### For Development

1. **Build Docker image** with certificates:
   ```bash
   docker build -t souravbiswassanto/pg-load-test:latest .
   ```

2. **Push to registry**:
   ```bash
   docker push souravbiswassanto/pg-load-test:latest
   ```

3. **Load to Kubernetes**:
   ```bash
   kind load docker-image souravbiswassanto/pg-load-test:latest
   # or
   minikube image load souravbiswassanto/pg-load-test:latest
   ```

## Support

For issues or questions:
- See `TLS_AUTHENTICATION_GUIDE.md` for detailed troubleshooting
- Check `TLS_QUICK_REFERENCE.md` for common solutions
- Review `k8s-tls/README.md` for Kubernetes-specific issues

## Summary

✅ **Complete TLS/SSL support implemented**
✅ **Three authentication modes available**
✅ **Backward compatible with existing deployments**
✅ **Comprehensive documentation provided**
✅ **Code compiles successfully**
✅ **Ready for testing and deployment**
