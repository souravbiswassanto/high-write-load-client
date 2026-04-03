# Upgrade Guide: Adding Read Operations & 10K+ Concurrent User Support

## What's New in V2

The enhanced version adds support for:

1. **Read Operations** - SELECT queries with various patterns
2. **Higher Concurrency** - Tested with 10,000+ concurrent workers  
3. **Mixed Workload** - Realistic read/write ratio
4. **Better Connection Management** - Optimized for high concurrency

## Files Added

- `config/config.go` - Enhanced with READ_PERCENT and READ_BATCH_SIZE
- `metrics/metrics_v2.go` - Tracks read operations separately
- `clients/postgres/load_generator_v2.go` - Load generator with read support
- `main_v2.go` - New main program for v2
- `build-v2.sh` - Build script for v2 version

## New Configuration Options

### Added to .env.example

```bash
# New configuration options
READ_PERCENT=60              # Percentage of read operations (0-100)
READ_BATCH_SIZE=10           # Records to fetch per read operation

# Note: READ_PERCENT + INSERT_PERCENT + UPDATE_PERCENT must equal 100
```

## Building and Running V2

### Option 1: Using Build Script

```bash
# Build v2 version
./build-v2.sh

# Run with high concurrency
export $(cat .env | xargs)
export CONCURRENT_WRITERS=10000
export READ_PERCENT=60
export INSERT_PERCENT=20
export UPDATE_PERCENT=20
./load-test-client-v2
```

### Option 2: Manual Build

```bash
# Backup original main
mv main.go main_v1.go

# Use v2 main
mv main_v2.go main.go

# Build
go build -o load-test-client-v2 .

# Restore original (optional)
mv main.go main_v2.go
mv main_v1.go main.go
```

## Migration Guide

### From V1 to V2

1. **Update .env file**:
   ```bash
   # Add new variables
   READ_PERCENT=60
   READ_BATCH_SIZE=10
   
   # Adjust existing (must total 100)
   INSERT_PERCENT=20
   UPDATE_PERCENT=20
   ```

2. **Increase connection pool** (for high concurrency):
   ```bash
   DB_MAX_OPEN_CONNS=500      # Up from 50
   DB_MAX_IDLE_CONNS=100      # Up from 10
   DB_MIN_FREE_CONNS=50       # Up from 5
   ```

3. **PostgreSQL tuning** (for 10K+ users):
   ```sql
   -- In postgresql.conf
   max_connections = 1000
   shared_buffers = 8GB
   effective_cache_size = 24GB
   work_mem = 16MB
   ```

## Test Scenarios

### Scenario 1: Read-Heavy (Typical Web App)
Simulates 10,000 concurrent users browsing a website.

```bash
export CONCURRENT_WRITERS=10000
export READ_PERCENT=70
export INSERT_PERCENT=20
export UPDATE_PERCENT=10
export TEST_RUN_DURATION=600
export READ_BATCH_SIZE=20
```

**Expected Results:**
- 80,000-150,000 ops/sec
- Read latency: 10-50ms avg, 200-500ms P99
- Insert latency: 50-200ms avg
- Update latency: 30-150ms avg

### Scenario 2: Balanced Workload
Simulates a social media or e-commerce platform.

```bash
export CONCURRENT_WRITERS=5000
export READ_PERCENT=50
export INSERT_PERCENT=30
export UPDATE_PERCENT=20
export BATCH_SIZE=200
```

**Expected Results:**
- 40,000-80,000 ops/sec
- Balanced latencies across all operations

### Scenario 3: Write-Heavy (Data Processing)
Simulates data ingestion or processing workload.

```bash
export CONCURRENT_WRITERS=3000
export READ_PERCENT=20
export INSERT_PERCENT=50
export UPDATE_PERCENT=30
export BATCH_SIZE=500
```

**Expected Results:**
- 30,000-60,000 ops/sec
- Higher write latencies due to contention

### Scenario 4: Maximum Read Load (Analytics)
Simulates reporting or analytics workload.

```bash
export CONCURRENT_WORKERS=15000
export READ_PERCENT=90
export INSERT_PERCENT=5
export UPDATE_PERCENT=5
export READ_BATCH_SIZE=50
```

**Expected Results:**
- 100,000+ ops/sec
- Very low read latencies

## Read Operation Patterns

The V2 load generator implements 4 realistic read patterns:

### 1. Read by ID Range
```sql
SELECT * FROM table WHERE id >= $1 LIMIT $2
```
Simulates: Pagination, sequential scans

### 2. Read by Status
```sql
SELECT * FROM table WHERE status = $1 ORDER BY score DESC LIMIT $2
```
Simulates: Filtered lists, sorted results

### 3. Read Recent Records
```sql
SELECT * FROM table ORDER BY created_at DESC LIMIT $1
```
Simulates: Activity feeds, recent items

### 4. Read by Name Pattern
```sql
SELECT * FROM table WHERE name LIKE $1 LIMIT $2
```
Simulates: Search functionality

## Performance Comparison

### V1 (Write-Only)
- Max concurrent workers: ~100
- Operations: Inserts + Updates only
- Typical throughput: 5,000-10,000 ops/sec
- Use case: Write load testing

### V2 (Read + Write)
- Max concurrent workers: 10,000+
- Operations: Reads + Inserts + Updates
- Typical throughput: 50,000-150,000 ops/sec
- Use case: Production simulation

## Monitoring & Debugging

### New Metrics in V2

```
=================================================================
Test Duration: 5m0s
-----------------------------------------------------------------
Cumulative Statistics:
  Total Operations: 4,523,000 (Reads: 2,713,800, Inserts: 904,600, Updates: 904,600)
  Total Errors: 12
  Total Data Transferred: 2.5 GB
-----------------------------------------------------------------
Current Throughput (interval):
  Operations/sec: 15,076.67 (Reads: 9,046.00/s, Inserts: 3,015.33/s, Updates: 3,015.33/s)
  Throughput: 8.5 MB/s
  Errors/sec: 0.04
-----------------------------------------------------------------
Latency Statistics:
  Reads   - Avg: 15ms, P95: 45ms, P99: 120ms
  Inserts - Avg: 85ms, P95: 250ms, P99: 600ms
  Updates - Avg: 35ms, P95: 95ms, P99: 250ms
-----------------------------------------------------------------
Connection Pool:
  Active: 523, Max: 1000, Available: 477
=================================================================
```

### PostgreSQL Monitoring

```sql
-- Check read vs write activity
SELECT
  sum(tup_returned) as rows_read,
  sum(tup_inserted) as rows_inserted,
  sum(tup_updated) as rows_updated
FROM pg_stat_user_tables
WHERE relname = 'load_test_data';

-- Check index usage
SELECT
  schemaname,
  tablename,
  indexname,
  idx_scan,
  idx_tup_read,
  idx_tup_fetch
FROM pg_stat_user_indexes
WHERE tablename = 'load_test_data';

-- Cache hit ratio (should be > 99% for read-heavy)
SELECT
  sum(heap_blks_read) as heap_read,
  sum(heap_blks_hit) as heap_hit,
  round(sum(heap_blks_hit) * 100.0 / 
    (sum(heap_blks_hit) + sum(heap_blks_read)), 2) as cache_hit_ratio
FROM pg_statio_user_tables
WHERE relname = 'load_test_data';
```

## Troubleshooting

### Issue: Low Read Throughput

**Symptoms**: Read operations < 10,000/sec with 10K workers

**Solutions**:
1. Check cache hit ratio (should be > 99%)
2. Add appropriate indices
3. Increase `shared_buffers` and `effective_cache_size`
4. Check disk I/O

### Issue: High Read Latency

**Symptoms**: P99 read latency > 500ms

**Solutions**:
1. Add missing indices
2. Increase `work_mem`
3. Run `VACUUM ANALYZE`
4. Consider table partitioning

### Issue: Connection Pool Exhaustion

**Symptoms**: "No connections available" errors

**Solutions**:
1. Use PgBouncer for connection pooling
2. Increase `max_connections`
3. Reduce `CONCURRENT_WRITERS`
4. Increase `DB_MAX_OPEN_CONNS`

## Best Practices for High Concurrency

1. **Start Small**: Begin with 100 workers, gradually increase
2. **Use Connection Pooler**: Deploy PgBouncer for 5K+ workers
3. **Monitor Everything**: Watch CPU, memory, disk I/O, network
4. **Tune PostgreSQL**: Optimize for your workload
5. **Test Incrementally**: 100 → 1K → 5K → 10K workers
6. **Prepare Infrastructure**: Ensure adequate resources

## When to Use V1 vs V2

### Use V1 When:
- Testing pure write performance
- Simple INSERT/UPDATE load testing
- Lower concurrency (< 1000 users)
- No read operations needed

### Use V2 When:
- Simulating realistic user behavior
- Testing read performance
- High concurrency (1000+ users)
- Mixed workload testing
- Production environment simulation

## Future Enhancements

Planned improvements for V3:
- [ ] DELETE operations
- [ ] Transaction support
- [ ] Custom query patterns
- [ ] Distributed testing
- [ ] Real-time metrics dashboard
- [ ] Support for other databases (MySQL, MongoDB)

## Getting Help

If you encounter issues:

1. Check HIGH_CONCURRENCY_GUIDE.md for detailed tuning
2. Review TEST_SCENARIOS.md for examples
3. Monitor PostgreSQL logs for errors
4. Use the provided SQL queries for debugging
5. Start with lower concurrency and work up

## Summary

V2 adds comprehensive read support and enables testing with 10,000+ concurrent users, making it suitable for realistic production workload simulation. The architecture remains extensible for future database support and additional operation types.
