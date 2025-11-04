# Kubernetes Deployment Guide

## Overview

Deploy the PostgreSQL High Write Load Testing Client on Kubernetes with one command. This guide includes ConfigMap for environment variables, Job for test execution, and optional persistent storage for results.

## Quick Start - One Command Deploy

```bash
kubectl apply -f k8s/
```

That's it! The client will deploy and start testing automatically.

## Architecture

```
┌─────────────────────────────────────────────┐
│          Kubernetes Cluster                 │
│                                             │
│  ┌──────────────────────────────────┐      │
│  │     ConfigMap (Environment)      │      │
│  │  - Database credentials          │      │
│  │  - Test parameters               │      │
│  │  - Workload distribution         │      │
│  └──────────────────────────────────┘      │
│                    │                        │
│                    ▼                        │
│  ┌──────────────────────────────────┐      │
│  │         Job (Load Test)          │      │
│  │  - Runs main_v2.go               │      │
│  │  - 100 concurrent workers        │      │
│  │  - 500s duration                 │      │
│  │  - Data loss tracking            │      │
│  └──────────────────────────────────┘      │
│                    │                        │
│                    ▼                        │
│  ┌──────────────────────────────────┐      │
│  │    PostgreSQL Database           │      │
│  │  (External or in-cluster)        │      │
│  └──────────────────────────────────┘      │
└─────────────────────────────────────────────┘
```

## Prerequisites

1. **Kubernetes cluster** (v1.20+)
2. **kubectl configured** to access your cluster
3. **PostgreSQL database** accessible from cluster
4. **Docker image** of the load test client (we'll build this)

## Step 1: Build Docker Image

### Dockerfile

Already created in project root. Build and push:

```bash
# Build the image
docker build -t your-registry/pg-load-test:latest .

# Push to registry
docker push your-registry/pg-load-test:latest

# Or for local testing with kind/minikube
docker build -t pg-load-test:latest .
kind load docker-image pg-load-test:latest  # If using kind
```

## Step 2: Create Kubernetes Manifests

All manifests are in the `k8s/` directory:

```
k8s/
├── 00-namespace.yaml          # Namespace for isolation
├── 01-configmap.yaml          # Environment variables
├── 02-secret.yaml             # Database credentials
├── 03-job.yaml                # Load test job
└── 04-pvc.yaml                # (Optional) Persistent storage
```

## Configuration

### Environment Variables (ConfigMap)

Edit `k8s/01-configmap.yaml` to customize your test:

```yaml
# Test Duration
TEST_RUN_DURATION: "500"          # 500 seconds (~8 minutes)

# Concurrency
CONCURRENT_WRITERS: "100"         # 100 concurrent workers

# Workload Distribution
READ_PERCENT: "60"                # 60% reads
INSERT_PERCENT: "25"              # 25% inserts
UPDATE_PERCENT: "15"              # 15% updates

# Batch Sizes
BATCH_SIZE: "100"                 # Insert batch size
READ_BATCH_SIZE: "20"             # Read batch size
```

### Database Credentials (Secret)

Edit `k8s/02-secret.yaml`:

```yaml
# Base64 encoded values
DB_HOST: <base64-encoded-host>
DB_PORT: <base64-encoded-port>
DB_USER: <base64-encoded-user>
DB_PASSWORD: <base64-encoded-password>
DB_NAME: <base64-encoded-dbname>
```

**Encode your credentials:**

```bash
echo -n "your-postgres-host" | base64
echo -n "5432" | base64
echo -n "postgres" | base64
echo -n "your-password" | base64
echo -n "postgres" | base64
```

## Deployment

### Option 1: One Command Deploy (Recommended)

```bash
# Deploy everything
kubectl apply -f k8s/

# Watch the job progress
kubectl logs -f -n pg-load-test job/pg-load-test-job
```

### Option 2: Step-by-Step Deploy

```bash
# 1. Create namespace
kubectl apply -f k8s/00-namespace.yaml

# 2. Create secret
kubectl apply -f k8s/02-secret.yaml

# 3. Create configmap
kubectl apply -f k8s/01-configmap.yaml

# 4. Deploy job
kubectl apply -f k8s/03-job.yaml

# 5. Monitor
kubectl logs -f -n pg-load-test job/pg-load-test-job
```

## Monitoring

### Check Job Status

```bash
# Get job status
kubectl get jobs -n pg-load-test

# Expected output:
# NAME                 COMPLETIONS   DURATION   AGE
# pg-load-test-job     1/1           8m45s      10m
```

### View Logs (Real-time)

```bash
# Follow logs
kubectl logs -f -n pg-load-test job/pg-load-test-job

# You'll see:
# - Configuration summary
# - Connection status
# - Real-time metrics (every 10s)
# - Final performance summary
# - Data loss report
```

### Check Pod Status

```bash
# Get pod status
kubectl get pods -n pg-load-test

# Describe pod for details
kubectl describe pod -n pg-load-test -l job-name=pg-load-test-job
```

### View Final Results

```bash
# Get complete logs (after completion)
kubectl logs -n pg-load-test job/pg-load-test-job

# Or save to file
kubectl logs -n pg-load-test job/pg-load-test-job > test-results.log
```

## Advanced Configuration

### High Concurrency Setup

For testing with 1000+ concurrent workers:

```yaml
# In 03-job.yaml
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
        env:
        - name: CONCURRENT_WRITERS
          value: "1000"
```

### Multiple Test Runs

Run multiple tests with different configurations:

```bash
# Deploy test with custom name
kubectl apply -f k8s/03-job.yaml

# Wait for completion
kubectl wait --for=condition=complete --timeout=600s job/pg-load-test-job -n pg-load-test

# Delete and re-run with different config
kubectl delete job pg-load-test-job -n pg-load-test
# Edit configmap
kubectl apply -f k8s/01-configmap.yaml
kubectl apply -f k8s/03-job.yaml
```

### CronJob for Scheduled Tests

For periodic testing:

```yaml
# k8s/05-cronjob.yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: pg-load-test-cron
  namespace: pg-load-test
spec:
  schedule: "0 2 * * *"  # Run daily at 2 AM
  jobTemplate:
    spec:
      template:
        # ... same as job template
```

Deploy:

```bash
kubectl apply -f k8s/05-cronjob.yaml
```

## Persistent Results Storage

Store test results in persistent volume:

```yaml
# Already configured in k8s/04-pvc.yaml
# Mounts to /results in container
```

Save results:

```bash
# Copy results from pod
kubectl cp -n pg-load-test \
  pg-load-test-job-xxxxx:/results/test-output.log \
  ./local-results.log
```

## Network Testing Scenarios

### Scenario 1: Test with Network Policies

Simulate network restrictions:

```yaml
# k8s/06-network-policy.yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: postgres-access
  namespace: pg-load-test
spec:
  podSelector:
    matchLabels:
      app: pg-load-test
  policyTypes:
  - Egress
  egress:
  - to:
    - podSelector:
        matchLabels:
          app: postgres
    ports:
    - protocol: TCP
      port: 5432
```

### Scenario 2: Chaos Testing

Use Chaos Mesh to inject failures:

```bash
# Install Chaos Mesh
curl -sSL https://mirrors.chaos-mesh.org/latest/install.sh | bash

# Create network chaos
kubectl apply -f k8s/07-chaos-network.yaml
```

## Troubleshooting

### Job Fails to Start

```bash
# Check pod events
kubectl describe pod -n pg-load-test -l job-name=pg-load-test-job

# Common issues:
# - Image pull error: Check registry credentials
# - OOMKilled: Increase memory limits
# - CrashLoopBackOff: Check database connectivity
```

### Database Connection Issues

```bash
# Test connectivity from pod
kubectl run -it --rm debug --image=postgres:15 -n pg-load-test -- \
  psql -h your-postgres-host -U postgres -d postgres

# Check DNS resolution
kubectl run -it --rm debug --image=busybox -n pg-load-test -- \
  nslookup your-postgres-host
```

### High Memory Usage

```bash
# Check pod resource usage
kubectl top pod -n pg-load-test

# Adjust limits if needed
# Edit k8s/03-job.yaml and reapply
```

### Job Doesn't Complete

```bash
# Check if job is stuck
kubectl get jobs -n pg-load-test

# Check pod logs for errors
kubectl logs -n pg-load-test job/pg-load-test-job

# Force delete if needed
kubectl delete job pg-load-test-job -n pg-load-test --grace-period=0 --force
```

## Cleanup

### Delete Job Only

```bash
kubectl delete job pg-load-test-job -n pg-load-test
```

### Delete Everything

```bash
kubectl delete namespace pg-load-test
```

### Keep Namespace, Delete Resources

```bash
kubectl delete job,configmap,secret,pvc -n pg-load-test --all
```

## Production Best Practices

### 1. Resource Limits

Always set resource limits:

```yaml
resources:
  requests:
    memory: "1Gi"
    cpu: "1000m"
  limits:
    memory: "2Gi"
    cpu: "2000m"
```

### 2. Secrets Management

Use external secrets manager:

```bash
# Example with Sealed Secrets
kubeseal --format=yaml < k8s/02-secret.yaml > k8s/02-sealed-secret.yaml
kubectl apply -f k8s/02-sealed-secret.yaml
```

### 3. Monitoring Integration

Add Prometheus annotations:

```yaml
metadata:
  annotations:
    prometheus.io/scrape: "true"
    prometheus.io/port: "8080"
    prometheus.io/path: "/metrics"
```

### 4. Multi-Cluster Deployment

Use GitOps (ArgoCD/Flux):

```yaml
# argocd-application.yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: pg-load-test
spec:
  source:
    repoURL: https://github.com/your-org/pg-load-test
    path: k8s
    targetRevision: main
  destination:
    server: https://kubernetes.default.svc
    namespace: pg-load-test
```

## Example Output

After deploying, you'll see:

```
=================================================================
PostgreSQL High Concurrency Load Testing Client v2
Supports Read + Write Operations for 10,000+ Concurrent Users
=================================================================

Configuration:
  Database: postgres@postgres-service:5432/postgres
  Concurrent Workers: 100
  Test Duration: 8m20s
  Workload: 60% Reads, 25% Inserts, 15% Updates

Connecting to PostgreSQL...
Connection Manager initialized successfully
  Max connections in DB: 100
  Current active connections: 5
  Available connections: 95

Starting load test...

=================================================================
Test Duration: 10s
-----------------------------------------------------------------
Total Operations: 12,450 (7,470 reads, 3,112 inserts, 1,868 updates)
Average Throughput: 1,245 ops/sec
=================================================================

... (continues for 500 seconds)

=================================================================
Data Loss Report:
-----------------------------------------------------------------
  Total Records Inserted: 15,560
  Records Found in DB: 15,560
  Records Lost: 0
  Data Loss Percentage: 0.00%
=================================================================

✅ No data loss detected - all inserted records are present in database

Test completed successfully!
```

## Quick Reference

| Command | Description |
|---------|-------------|
| `kubectl apply -f k8s/` | Deploy everything |
| `kubectl logs -f -n pg-load-test job/pg-load-test-job` | Watch logs |
| `kubectl get jobs -n pg-load-test` | Check job status |
| `kubectl delete job pg-load-test-job -n pg-load-test` | Delete job |
| `kubectl delete namespace pg-load-test` | Delete everything |

## Next Steps

1. ✅ Deploy with one command: `kubectl apply -f k8s/`
2. 📊 Monitor logs: `kubectl logs -f -n pg-load-test job/pg-load-test-job`
3. 📈 Review results after completion
4. 🔄 Adjust configuration and re-run
5. 🚀 Scale to higher concurrency
6. 🧪 Add chaos testing with network failures

## Support

For issues or questions:
- Check logs: `kubectl logs -n pg-load-test job/pg-load-test-job`
- Check events: `kubectl get events -n pg-load-test`
- Review documentation: `DATA_LOSS_TRACKING.md`, `HIGH_CONCURRENCY_GUIDE.md`

---

**Ready to deploy?** Run: `kubectl apply -f k8s/` 🚀
