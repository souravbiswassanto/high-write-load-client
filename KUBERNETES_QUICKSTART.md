# Kubernetes Deployment - Quick Start

## 🚀 One Command Deploy

```bash
./deploy-k8s.sh
```

That's it! The script will:
1. ✅ Build Docker image
2. ✅ Encode credentials from `.env`
3. ✅ Deploy to Kubernetes
4. ✅ Show logs automatically

## 📋 What Gets Deployed

Your test configuration from `run.sh`:
- **Test Duration:** 500 seconds (~8 minutes)
- **Concurrent Workers:** 100
- **Workload:** 60% Reads, 25% Inserts, 15% Updates
- **Read Batch Size:** 20 records

## 📁 Files Created

```
k8s/
├── 00-namespace.yaml          # Creates pg-load-test namespace
├── 01-configmap.yaml          # Test parameters (duration, concurrency, etc.)
├── 02-secret.yaml             # Database credentials (base64 encoded)
├── 03-job.yaml                # Kubernetes Job to run the test
├── 04-pvc.yaml                # (Optional) Persistent storage
└── README.md                  # Detailed documentation
```

## 🔧 Before You Deploy

### 1. Build Docker Image (Optional - script does this)

```bash
docker build -t pg-load-test:latest .
```

### 2. Configure Database Credentials

**Option A: Using `.env` file (Automatic)**

Your existing `.env` file will be used automatically by the script:
```bash
./deploy-k8s.sh
```

**Option B: Manual Configuration**

Encode credentials:
```bash
echo -n "127.0.0.1" | base64      # Your DB host
echo -n "5678" | base64           # Your DB port
echo -n "postgres" | base64
echo -n "SVuwzFvw!HJf;vLm" | base64
echo -n "postgres" | base64
```

Update `k8s/02-secret.yaml` with the base64 values.

## 🎯 Deploy Options

### Option 1: Automated Script (Recommended)

```bash
./deploy-k8s.sh
```

**What it does:**
- Checks prerequisites (kubectl, docker)
- Builds Docker image
- Encodes credentials from `.env`
- Deploys all resources
- Shows you how to follow logs

### Option 2: Manual Kubernetes Deploy

```bash
# Deploy all resources
kubectl apply -f k8s/

# Follow logs
kubectl logs -f -n pg-load-test job/pg-load-test-job
```

### Option 3: Step-by-Step

```bash
kubectl apply -f k8s/00-namespace.yaml
kubectl apply -f k8s/02-secret.yaml
kubectl apply -f k8s/01-configmap.yaml
kubectl apply -f k8s/03-job.yaml
kubectl logs -f -n pg-load-test job/pg-load-test-job
```

## 📊 Monitoring

### Watch Test Progress

```bash
# Follow logs (real-time)
kubectl logs -f -n pg-load-test job/pg-load-test-job

# Check job status
kubectl get jobs -n pg-load-test

# Check pod status
kubectl get pods -n pg-load-test

# View all resources
kubectl get all -n pg-load-test
```

### View Results

```bash
# Get complete output
kubectl logs -n pg-load-test job/pg-load-test-job

# Save to file
kubectl logs -n pg-load-test job/pg-load-test-job > test-results.log

# Check data loss report
kubectl logs -n pg-load-test job/pg-load-test-job | grep -A 10 "Data Loss Report"
```

## 🔄 Re-run Test

```bash
# Delete existing job
kubectl delete job pg-load-test-job -n pg-load-test

# (Optional) Update configuration
kubectl apply -f k8s/01-configmap.yaml

# Deploy again
kubectl apply -f k8s/03-job.yaml
```

## ⚙️ Customize Configuration

Edit `k8s/01-configmap.yaml`:

```yaml
data:
  # Your current configuration
  TEST_RUN_DURATION: "500"
  CONCURRENT_WRITERS: "100"
  READ_PERCENT: "60"
  INSERT_PERCENT: "25"
  UPDATE_PERCENT: "15"
  READ_BATCH_SIZE: "20"
  
  # Other available options
  BATCH_SIZE: "100"              # Insert batch size
  MAX_OPEN_CONNS: "50"           # Connection pool size
  REPORT_INTERVAL: "10s"         # How often to show metrics
```

Apply changes:
```bash
kubectl apply -f k8s/01-configmap.yaml
kubectl delete job pg-load-test-job -n pg-load-test
kubectl apply -f k8s/03-job.yaml
```

## 🧹 Cleanup

```bash
# Delete everything
kubectl delete namespace pg-load-test

# Or delete only the job
kubectl delete job pg-load-test-job -n pg-load-test
```

## 🐛 Troubleshooting

### Pod Not Starting

```bash
# Check pod events
kubectl describe pod -n pg-load-test -l app=pg-load-test

# Check logs
kubectl logs -n pg-load-test -l app=pg-load-test
```

### Image Pull Errors

```bash
# If using local image with kind
kind load docker-image pg-load-test:latest

# If using minikube
minikube image load pg-load-test:latest
```

### Connection Issues

```bash
# Test database connectivity
kubectl run -it --rm debug --image=postgres:15 -n pg-load-test -- \
  psql -h YOUR_HOST -p YOUR_PORT -U postgres
```

### View Events

```bash
kubectl get events -n pg-load-test --sort-by='.lastTimestamp'
```

## 📈 Expected Output

After deployment, you'll see:

```
=================================================================
PostgreSQL High Concurrency Load Testing Client v2
Supports Read + Write Operations for 10,000+ Concurrent Users
=================================================================

Configuration:
  Database: postgres@your-host:5432/postgres
  Concurrent Workers: 100
  Test Duration: 8m20s
  Batch Size: 100 records (inserts), 20 records (reads)
  Workload: 60% Reads, 25% Inserts, 15% Updates

Connecting to PostgreSQL...
Connection Manager initialized successfully
  Max connections in DB: 100
  Current active connections: 5
  Available connections: 95

Starting 100 concurrent workers...
All workers started successfully

Starting load test for 8m20s...

=================================================================
Test Duration: 10s
-----------------------------------------------------------------
Cumulative Statistics:
  Total Operations: 12,450 (Reads: 7,470, Inserts: 3,112, Updates: 1,868)
  Total Errors: 0
  Total Data Transferred: 325.50 MB
-----------------------------------------------------------------
Current Throughput (interval):
  Operations/sec: 1,245.00 (Reads: 747.00/s, Inserts: 311.20/s, Updates: 186.80/s)
=================================================================

... (continues until test completes)

=================================================================
Data Loss Report:
-----------------------------------------------------------------
  Total Records Inserted: 15,560
  Records Found in DB: 15,560
  Records Lost: 0
  Data Loss Percentage: 0.00%
=================================================================

✅ No data loss detected - all inserted records are present in database

Cleaning up test data...
Test data table deleted successfully

Test completed successfully!
```

## 🎓 Advanced Usage

### High Concurrency Testing

For 1000+ workers, edit `k8s/03-job.yaml`:

```yaml
resources:
  requests:
    memory: "2Gi"
    cpu: "2000m"
  limits:
    memory: "4Gi"
    cpu: "4000m"
```

And `k8s/01-configmap.yaml`:
```yaml
CONCURRENT_WRITERS: "1000"
```

### Scheduled Testing

Create a CronJob for daily tests:

```bash
# Copy 03-job.yaml and modify to CronJob
kubectl apply -f k8s/05-cronjob.yaml
```

### Network Chaos Testing

Use with Chaos Mesh to simulate network failures during test:

```bash
# Install Chaos Mesh
curl -sSL https://mirrors.chaos-mesh.org/latest/install.sh | bash

# Apply network chaos during test
kubectl apply -f chaos-network.yaml
```

## 📚 Documentation

- **[KUBERNETES_DEPLOYMENT.md](KUBERNETES_DEPLOYMENT.md)** - Complete deployment guide
- **[k8s/README.md](k8s/README.md)** - Kubernetes manifests reference
- **[DATA_LOSS_TRACKING.md](DATA_LOSS_TRACKING.md)** - Data loss feature details
- **[HIGH_CONCURRENCY_GUIDE.md](HIGH_CONCURRENCY_GUIDE.md)** - Scaling guide

## ✅ Checklist

Before deploying:
- [ ] Kubernetes cluster accessible (`kubectl cluster-info`)
- [ ] Docker installed and running
- [ ] `.env` file with database credentials (or manually update `k8s/02-secret.yaml`)
- [ ] Sufficient cluster resources (1Gi memory, 1 CPU minimum)

Deploy:
- [ ] Run `./deploy-k8s.sh` or `kubectl apply -f k8s/`
- [ ] Follow logs to monitor progress
- [ ] Save results if needed
- [ ] Cleanup when done

## 🎯 Quick Commands

```bash
# Deploy
./deploy-k8s.sh

# Watch logs
kubectl logs -f -n pg-load-test job/pg-load-test-job

# Check status
kubectl get all -n pg-load-test

# Save results
kubectl logs -n pg-load-test job/pg-load-test-job > results.log

# Cleanup
kubectl delete namespace pg-load-test
```

---

**Ready to deploy?** Run: `./deploy-k8s.sh` 🚀

Your test will run with exactly the same configuration as your `run.sh`:
- 500 seconds duration
- 100 concurrent workers  
- 60% reads, 25% inserts, 15% updates
- 20 records per read batch
- Automatic data loss tracking
