# Kubernetes Manifests

This directory contains all Kubernetes manifests for deploying the PostgreSQL Load Test Client.

## Files

| File | Description |
|------|-------------|
| `00-namespace.yaml` | Creates `pg-load-test` namespace |
| `01-configmap.yaml` | Test configuration (duration, concurrency, workload) |
| `02-secret.yaml` | Database credentials (base64 encoded) |
| `03-job.yaml` | Kubernetes Job to run the load test |
| `04-pvc.yaml` | (Optional) Persistent volume for results |

## Quick Deploy

### Option 1: Use Deployment Script (Recommended)

```bash
# From project root
./deploy-k8s.sh
```

This script will:
- Build Docker image
- Encode your `.env` credentials
- Deploy all resources
- Show logs automatically

### Option 2: Manual Deploy

```bash
# From project root
kubectl apply -f k8s/
```

Then follow logs:

```bash
kubectl logs -f -n pg-load-test job/pg-load-test-job
```

## Configuration

### Update Test Parameters

Edit `01-configmap.yaml`:

```yaml
data:
  TEST_RUN_DURATION: "500"    # Test duration in seconds
  CONCURRENT_WRITERS: "100"   # Number of concurrent workers
  READ_PERCENT: "60"          # Percentage of read operations
  INSERT_PERCENT: "25"        # Percentage of insert operations
  UPDATE_PERCENT: "15"        # Percentage of update operations
  READ_BATCH_SIZE: "20"       # Number of records per read
```

### Update Database Credentials

#### Method 1: Using `.env` file (Recommended)

If you have a `.env` file in project root:

```bash
./deploy-k8s.sh
```

The script automatically encodes credentials from `.env`.

#### Method 2: Manual Encoding

Encode your credentials:

```bash
echo -n "your-host" | base64
echo -n "5432" | base64
echo -n "postgres" | base64
echo -n "your-password" | base64
echo -n "postgres" | base64
```

Update `02-secret.yaml` with the base64 values.

## Monitoring

### Watch Job Progress

```bash
# Get job status
kubectl get jobs -n pg-load-test

# Get pod status
kubectl get pods -n pg-load-test

# Follow logs
kubectl logs -f -n pg-load-test job/pg-load-test-job
```

### Check Results

```bash
# View complete logs
kubectl logs -n pg-load-test job/pg-load-test-job

# Save to file
kubectl logs -n pg-load-test job/pg-load-test-job > results.log
```

## Re-run Test

```bash
# Delete job
kubectl delete job pg-load-test-job -n pg-load-test

# Update config if needed
kubectl apply -f k8s/01-configmap.yaml

# Re-deploy job
kubectl apply -f k8s/03-job.yaml
```

## Cleanup

```bash
# Delete everything
kubectl delete namespace pg-load-test

# Or delete specific resources
kubectl delete job,configmap,secret,pvc -n pg-load-test --all
```

## Troubleshooting

### Pod Not Starting

```bash
# Check pod events
kubectl describe pod -n pg-load-test -l app=pg-load-test

# Common issues:
# - ImagePullBackOff: Check image name in 03-job.yaml
# - CrashLoopBackOff: Check database connectivity
# - Pending: Check resource availability
```

### Connection Issues

```bash
# Test database connectivity
kubectl run -it --rm debug --image=postgres:15 -n pg-load-test -- \
  psql -h YOUR_HOST -U postgres -d postgres
```

### View Pod Logs

```bash
# Get pod name
POD_NAME=$(kubectl get pods -n pg-load-test -l app=pg-load-test -o name)

# View logs
kubectl logs -n pg-load-test $POD_NAME

# Follow logs
kubectl logs -f -n pg-load-test $POD_NAME
```

## Advanced Configuration

### High Concurrency

For 1000+ workers, update `03-job.yaml`:

```yaml
spec:
  template:
    spec:
      containers:
      - name: load-test
        resources:
          requests:
            memory: "2Gi"
            cpu: "2000m"
          limits:
            memory: "4Gi"
            cpu: "4000m"
```

Update `01-configmap.yaml`:

```yaml
data:
  CONCURRENT_WRITERS: "1000"
```

### Scheduled Tests (CronJob)

Create `05-cronjob.yaml`:

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: pg-load-test-cron
  namespace: pg-load-test
spec:
  schedule: "0 2 * * *"  # Daily at 2 AM
  jobTemplate:
    spec:
      # Copy from 03-job.yaml
```

### Custom Image Registry

Update `03-job.yaml`:

```yaml
spec:
  template:
    spec:
      containers:
      - name: load-test
        image: your-registry.com/pg-load-test:v1.0
        imagePullPolicy: Always
      imagePullSecrets:
      - name: registry-credentials
```

## Environment Variables Reference

### From ConfigMap

| Variable | Default | Description |
|----------|---------|-------------|
| `TEST_RUN_DURATION` | `500` | Test duration in seconds |
| `CONCURRENT_WRITERS` | `100` | Number of concurrent workers |
| `READ_PERCENT` | `60` | Read operations percentage |
| `INSERT_PERCENT` | `25` | Insert operations percentage |
| `UPDATE_PERCENT` | `15` | Update operations percentage |
| `BATCH_SIZE` | `100` | Insert batch size |
| `READ_BATCH_SIZE` | `20` | Read batch size |
| `TABLE_NAME` | `load_test_data` | Database table name |
| `MAX_OPEN_CONNS` | `50` | Max open connections |
| `MAX_IDLE_CONNS` | `10` | Max idle connections |
| `REPORT_INTERVAL` | `10s` | Metrics report interval |

### From Secret

| Variable | Description |
|----------|-------------|
| `DB_HOST` | PostgreSQL host |
| `DB_PORT` | PostgreSQL port |
| `DB_USER` | Database user |
| `DB_PASSWORD` | Database password |
| `DB_NAME` | Database name |

## Examples

### Run with Different Configurations

**High Read Workload:**
```yaml
# In 01-configmap.yaml
READ_PERCENT: "80"
INSERT_PERCENT: "15"
UPDATE_PERCENT: "5"
```

**High Write Workload:**
```yaml
# In 01-configmap.yaml
READ_PERCENT: "20"
INSERT_PERCENT: "60"
UPDATE_PERCENT: "20"
```

**Short Test:**
```yaml
# In 01-configmap.yaml
TEST_RUN_DURATION: "60"
CONCURRENT_WRITERS: "10"
```

**Long Test:**
```yaml
# In 01-configmap.yaml
TEST_RUN_DURATION: "3600"
CONCURRENT_WRITERS: "200"
```

## Quick Reference

```bash
# Deploy
kubectl apply -f k8s/

# Watch logs
kubectl logs -f -n pg-load-test job/pg-load-test-job

# Check status
kubectl get all -n pg-load-test

# Cleanup
kubectl delete namespace pg-load-test
```

---

For more details, see [KUBERNETES_DEPLOYMENT.md](../KUBERNETES_DEPLOYMENT.md) in the project root.
