# Quick Reference Guide

## One-Line Commands

### Build and Run
```bash
# Build
go build -o load-test-client .

# Run with .env file
export $(cat .env | grep -v '^#' | xargs) && ./load-test-client
```

### Quick Tests
```bash
# 30 second test
export $(cat .env | xargs) && export TEST_RUN_DURATION=30 CONCURRENT_WRITERS=5 && ./load-test-client

# 5 minute test
export $(cat .env | xargs) && export TEST_RUN_DURATION=300 CONCURRENT_WRITERS=10 && ./load-test-client

# Heavy load test
export $(cat .env | xargs) && export TEST_RUN_DURATION=900 CONCURRENT_WRITERS=50 BATCH_SIZE=500 && ./load-test-client
```

## Environment Variables Quick Reference

| Variable | Default | Description |
|----------|---------|-------------|
| `DB_HOST` | `localhost` | Database host |
| `DB_PORT` | `5432` | Database port |
| `DB_USER` | `postgres` | Database user |
| `DB_PASSWORD` | - | Database password |
| `DB_NAME` | `testdb` | Database name |
| `CONCURRENT_WRITERS` | `10` | Number of workers |
| `TEST_RUN_DURATION` | `300` | Test duration (seconds) |
| `BATCH_SIZE` | `100` | Records per insert |
| `INSERT_PERCENT` | `70` | Insert % (0-100) |
| `UPDATE_PERCENT` | `30` | Update % (0-100) |

## Common Scenarios

### Development Testing
```bash
CONCURRENT_WRITERS=5 TEST_RUN_DURATION=60 BATCH_SIZE=50
```

### Staging Testing
```bash
CONCURRENT_WRITERS=20 TEST_RUN_DURATION=600 BATCH_SIZE=200
```

### Production Simulation
```bash
CONCURRENT_WRITERS=50 TEST_RUN_DURATION=3600 BATCH_SIZE=500
```

## Monitoring Queries

```sql
-- Active connections
SELECT count(*) FROM pg_stat_activity WHERE state = 'active';

-- Table size
SELECT pg_size_pretty(pg_total_relation_size('load_test_data'));

-- Write stats
SELECT n_tup_ins, n_tup_upd FROM pg_stat_user_tables WHERE relname = 'load_test_data';
```

## Troubleshooting

| Issue | Solution |
|-------|----------|
| Connection refused | Check DB_HOST, DB_PORT, network |
| Insufficient connections | Increase max_connections or reduce workers |
| High latency | Reduce workers/batch size, check DB resources |
| Out of memory | Reduce workers and batch size |

## File Structure
```
.
├── main.go                    # Main application
├── config/
│   └── config.go              # Configuration management
├── clients/postgres/
│   ├── client.go              # Basic client (legacy)
│   ├── connection_manager.go  # Connection management
│   └── load_generator.go      # Load generation
├── metrics/
│   └── metrics.go             # Metrics tracking
├── .env.example               # Example configuration
├── Dockerfile                 # Docker build
├── k8s-example.yaml           # Kubernetes manifests
├── README.md                  # Main documentation
├── ARCHITECTURE.md            # Architecture details
└── TEST_SCENARIOS.md          # Test examples
```

## Quick Metrics Interpretation

### Good Performance
- Ops/sec: > 50
- P99 latency: < 1s (inserts), < 500ms (updates)
- Errors: 0
- Available connections: > 5

### Poor Performance
- Ops/sec: < 10
- P99 latency: > 5s
- Errors: > 1%
- Available connections: < 2

## Docker Commands

```bash
# Build
docker build -t load-test-client .

# Run
docker run --rm --env-file .env load-test-client

# Run with overrides
docker run --rm --env-file .env -e TEST_RUN_DURATION=60 load-test-client
```

## Kubernetes Commands

```bash
# Deploy
kubectl apply -f k8s-example.yaml

# Check status
kubectl get jobs
kubectl logs job/postgres-load-test

# Delete
kubectl delete -f k8s-example.yaml
```

## Cleanup

```sql
-- Drop test table
DROP TABLE load_test_data CASCADE;

-- Reclaim space
VACUUM FULL;
```

Or uncomment cleanup section in `main.go` for automatic cleanup.
