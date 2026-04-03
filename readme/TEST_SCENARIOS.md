# Test Scenarios and Expected Results

This document provides various test scenarios you can run against your PostgreSQL database.

## Quick Test Scenarios

### 1. Quick Validation (30 seconds)
```bash
export $(cat .env | grep -v '^#' | xargs)
export TEST_RUN_DURATION=30
export CONCURRENT_WRITERS=5
export BATCH_SIZE=50
./load-test-client
```
**Expected**: ~500-1000 operations, good for quick verification

### 2. Light Load - Development Testing (2 minutes)
```bash
export $(cat .env | grep -v '^#' | xargs)
export TEST_RUN_DURATION=120
export CONCURRENT_WRITERS=5
export BATCH_SIZE=100
export INSERT_PERCENT=70
export UPDATE_PERCENT=30
./load-test-client
```
**Expected**: ~2000-4000 operations, 50-100 MB written

### 3. Medium Load - Staging Testing (10 minutes)
```bash
export $(cat .env | grep -v '^#' | xargs)
export TEST_RUN_DURATION=600
export CONCURRENT_WRITERS=20
export BATCH_SIZE=200
export INSERT_PERCENT=70
export UPDATE_PERCENT=30
./load-test-client
```
**Expected**: ~20000-40000 operations, 500MB-1GB written

### 4. Heavy Load - Production Simulation (30 minutes)
```bash
export $(cat .env | grep -v '^#' | xargs)
export TEST_RUN_DURATION=1800
export CONCURRENT_WRITERS=50
export BATCH_SIZE=500
export INSERT_PERCENT=80
export UPDATE_PERCENT=20
./load-test-client
```
**Expected**: ~200000+ operations, 5-10GB written

### 5. Update-Heavy Workload (10 minutes)
```bash
export $(cat .env | grep -v '^#' | xargs)
export TEST_RUN_DURATION=600
export CONCURRENT_WRITERS=30
export BATCH_SIZE=100
export INSERT_PERCENT=20
export UPDATE_PERCENT=80
./load-test-client
```
**Expected**: Tests update performance, useful for databases with existing data

### 6. Maximum Throughput Test (15 minutes)
```bash
export $(cat .env | grep -v '^#' | xargs)
export TEST_RUN_DURATION=900
export CONCURRENT_WRITERS=100
export BATCH_SIZE=1000
export INSERT_PERCENT=90
export UPDATE_PERCENT=10
./load-test-client
```
**Expected**: Maximum write throughput, may stress database resources

### 7. Sustained Load Test (1 hour)
```bash
export $(cat .env | grep -v '^#' | xargs)
export TEST_RUN_DURATION=3600
export CONCURRENT_WRITERS=25
export BATCH_SIZE=200
export INSERT_PERCENT=70
export UPDATE_PERCENT=30
./load-test-client
```
**Expected**: Tests database stability under sustained load

### 8. Small Batch High Concurrency (5 minutes)
```bash
export $(cat .env | grep -v '^#' | xargs)
export TEST_RUN_DURATION=300
export CONCURRENT_WRITERS=50
export BATCH_SIZE=10
export INSERT_PERCENT=70
export UPDATE_PERCENT=30
./load-test-client
```
**Expected**: Tests many small transactions, higher connection overhead

## Using the Makefile

If you have `make` installed:

```bash
# Quick 30-second test
make test-short

# Medium 5-minute test
make test-medium

# Heavy 15-minute test
make test-heavy
```

## Monitoring During Tests

While the test is running, monitor your database:

### PostgreSQL Monitoring Queries

```sql
-- Check active connections
SELECT count(*) FROM pg_stat_activity WHERE state = 'active';

-- Check current connections by state
SELECT state, count(*) 
FROM pg_stat_activity 
GROUP BY state;

-- Check table size
SELECT pg_size_pretty(pg_total_relation_size('load_test_data'));

-- Check write activity
SELECT 
    schemaname,
    relname,
    n_tup_ins as inserts,
    n_tup_upd as updates,
    n_tup_del as deletes
FROM pg_stat_user_tables
WHERE relname = 'load_test_data';

-- Check lock status
SELECT locktype, mode, granted, count(*)
FROM pg_locks
GROUP BY locktype, mode, granted;
```

### System Monitoring

```bash
# Watch CPU and memory
top -p $(pgrep postgres)

# Watch disk I/O
iostat -x 2

# Watch network
iftop
```

## Interpreting Results

### Good Performance Indicators
- ✅ Latency P99 < 1 second for inserts
- ✅ Latency P99 < 500ms for updates
- ✅ Zero or very low error rate (<0.1%)
- ✅ Consistent throughput over time
- ✅ Stable connection count

### Performance Issues
- ❌ P99 latency > 5 seconds
- ❌ Error rate > 1%
- ❌ Declining throughput over time
- ❌ Growing connection count
- ❌ "Connection refused" errors

### Optimization Tips

If you see poor performance:

1. **High Latency**: 
   - Reduce `CONCURRENT_WRITERS`
   - Check disk I/O
   - Check database locks
   - Tune PostgreSQL configuration

2. **Low Throughput**:
   - Increase `BATCH_SIZE`
   - Increase `CONCURRENT_WRITERS`
   - Check network latency
   - Check database CPU usage

3. **Connection Issues**:
   - Increase `max_connections` in PostgreSQL
   - Reduce `DB_MAX_OPEN_CONNS`
   - Check for connection leaks

4. **Memory Issues**:
   - Reduce `CONCURRENT_WRITERS`
   - Reduce `BATCH_SIZE`
   - Check `work_mem` in PostgreSQL

## Production Load Simulation

To simulate a production environment with 500GB of data:

```bash
# Run for 24 hours with realistic load
export $(cat .env | grep -v '^#' | xargs)
export TEST_RUN_DURATION=86400  # 24 hours
export CONCURRENT_WRITERS=30
export BATCH_SIZE=200
export INSERT_PERCENT=70
export UPDATE_PERCENT=30
./load-test-client
```

This will write approximately 100-200 GB of data over 24 hours, depending on your database performance.

## Cleanup

After testing, you can clean up the test table:

```sql
-- Check table size first
SELECT pg_size_pretty(pg_total_relation_size('load_test_data'));

-- Drop the test table
DROP TABLE load_test_data CASCADE;

-- Vacuum to reclaim space
VACUUM FULL;
```

Or modify `main.go` to uncomment the cleanup section to automatically drop the table after each run.
