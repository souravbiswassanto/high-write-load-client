# High Concurrency Configuration Guide (10,000+ Concurrent Users)

This guide explains how to configure the load testing client to simulate 10,000+ concurrent users performing read and write operations.

## Key Architectural Changes for High Concurrency

### 1. Workload Distribution
With 10,000 concurrent users, the workload now supports three operation types:
- **READ**: SELECT queries (e.g., 60%)
- **INSERT**: Bulk inserts (e.g., 20%)
- **UPDATE**: Individual updates (e.g., 20%)

### 2. Connection Pooling Strategy

For 10,000 concurrent users, you need to optimize connection pooling:

```bash
# PostgreSQL Configuration (postgresql.conf)
max_connections = 1000              # Increase from default (100)
shared_buffers = 8GB               # 25% of total RAM
effective_cache_size = 24GB        # 75% of total RAM
work_mem = 16MB                    # Per operation
maintenance_work_mem = 2GB
checkpoint_completion_target = 0.9
wal_buffers = 16MB
default_statistics_target = 100
random_page_cost = 1.1             # For SSD
effective_io_concurrency = 200     # For SSD

# Client Connection Pool
DB_MAX_OPEN_CONNS=500             # More connections
DB_MAX_IDLE_CONNS=100             # Keep more idle connections
DB_MIN_FREE_CONNS=50              # Safety buffer
```

## Configuration for 10,000 Concurrent Users

### Example 1: Read-Heavy Workload (Typical Web Application)
```bash
# .env configuration
DB_HOST=127.0.0.1
DB_PORT=5678
DB_USER=postgres
DB_PASSWORD='SVuwzFvw!HJf;vLm'
DB_NAME=postgres
DB_SSL_MODE=disable

# Connection Pool - Optimized for high concurrency
DB_MAX_OPEN_CONNS=500
DB_MAX_IDLE_CONNS=100
DB_MIN_FREE_CONNS=50

# Load Test Configuration - 10K users
CONCURRENT_WRITERS=10000          # 10,000 concurrent goroutines
TEST_RUN_DURATION=600             # 10 minutes
BATCH_SIZE=100                    # Inserts per batch
READ_BATCH_SIZE=10                # Records per read query
REPORT_INTERVAL=10                # Report every 10 seconds

# Workload - Read-heavy (typical web app)
READ_PERCENT=60                   # 60% reads
INSERT_PERCENT=20                 # 20% inserts
UPDATE_PERCENT=20                 # 20% updates
TABLE_NAME=load_test_data
```

### Example 2: Balanced Workload
```bash
# Balanced read/write
CONCURRENT_WRITERS=10000
READ_PERCENT=50
INSERT_PERCENT=30
UPDATE_PERCENT=20
```

### Example 3: Write-Heavy Workload (Data Processing)
```bash
# Write-heavy workload
CONCURRENT_WRITERS=5000           # Fewer concurrent users
READ_PERCENT=20
INSERT_PERCENT=50
UPDATE_PERCENT=30
BATCH_SIZE=500                    # Larger batches for inserts
```

### Example 4: Maximum Read Load (Analytics/Reporting)
```bash
# Maximum read operations
CONCURRENT_WRITERS=15000          # Even more concurrent readers
READ_PERCENT=90
INSERT_PERCENT=5
UPDATE_PERCENT=5
READ_BATCH_SIZE=50                # Fetch more records per query
```

## Running the Test

```bash
# Build
go build -o load-test-client .

# Run with 10K concurrent users (read-heavy)
export $(cat .env | grep -v '^#' | xargs)
export CONCURRENT_WRITERS=10000
export READ_PERCENT=60
export INSERT_PERCENT=20
export UPDATE_PERCENT=20
./load-test-client
```

## Performance Expectations

### With 10,000 Concurrent Users:

#### Read-Heavy Workload (60% read, 20% insert, 20% update)
- **Expected Throughput**: 50,000 - 150,000 ops/sec
- **Reads**: 30,000 - 90,000/sec
- **Inserts**: 10,000 - 30,000/sec  
- **Updates**: 10,000 - 30,000/sec
- **Latency**: 
  - Reads: 10-50ms avg, 100-500ms P99
  - Inserts: 50-200ms avg, 500ms-2s P99
  - Updates: 20-100ms avg, 200ms-1s P99

#### Write-Heavy Workload (20% read, 50% insert, 30% update)
- **Expected Throughput**: 30,000 - 80,000 ops/sec
- **Higher latencies** due to write contention
- **More database CPU** usage

## System Requirements

### Database Server
- **CPU**: 16+ cores
- **RAM**: 32+ GB
- **Storage**: NVMe SSD with 10,000+ IOPS
- **Network**: 10 Gbps+

### Client Machine
- **CPU**: 8+ cores
- **RAM**: 8+ GB
- **Network**: 1 Gbps+

## Monitoring During Test

### PostgreSQL Queries

```sql
-- Monitor connection count
SELECT count(*), state 
FROM pg_stat_activity 
GROUP BY state;

-- Check for connection limits
SELECT count(*) as used_connections, 
       current_setting('max_connections')::int as max_connections,
       current_setting('max_connections')::int - count(*) as available
FROM pg_stat_activity;

-- Monitor lock contention
SELECT locktype, mode, granted, count(*) 
FROM pg_locks 
GROUP BY locktype, mode, granted 
ORDER BY count(*) DESC;

-- Check table bloat
SELECT schemaname, tablename,
       pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) as size
FROM pg_tables
WHERE tablename = 'load_test_data';

-- Monitor cache hit ratio (should be > 99%)
SELECT 
  sum(heap_blks_read) as heap_read,
  sum(heap_blks_hit) as heap_hit,
  sum(heap_blks_hit) / (sum(heap_blks_hit) + sum(heap_blks_read)) as ratio
FROM pg_statio_user_tables
WHERE relname = 'load_test_data';
```

### System Monitoring

```bash
# CPU usage per PostgreSQL process
top -H -p $(pgrep -f postgres) -n 1

# I/O stats
iostat -x 1

# Network stats
iftop -i eth0

# PostgreSQL stats
pg_stat_statements  # Enable this extension for query-level stats
```

## Optimization Tips

### 1. Connection Pooling
Use PgBouncer for connection pooling:
```bash
# Install PgBouncer
sudo apt-get install pgbouncer

# Configure pgbouncer.ini
[databases]
postgres = host=127.0.0.1 port=5432 dbname=postgres

[pgbouncer]
listen_addr = 0.0.0.0
listen_port = 6432
auth_type = md5
pool_mode = transaction  # or session
max_client_conn = 10000
default_pool_size = 100
reserve_pool_size = 25
```

Then connect to PgBouncer:
```bash
DB_PORT=6432  # PgBouncer port instead of 5432
```

### 2. Prepared Statements
The enhanced load generator uses prepared statements for better performance with high concurrency.

### 3. Batch Operations
- Increase `BATCH_SIZE` for inserts (500-1000)
- Balance with memory usage

### 4. Index Optimization
```sql
-- Add indices for common read patterns
CREATE INDEX CONCURRENTLY idx_load_test_data_created_at_id 
ON load_test_data(created_at, id);

CREATE INDEX CONCURRENTLY idx_load_test_data_email_name 
ON load_test_data(email, name);
```

### 5. Vacuum and Analyze
```sql
-- Schedule regular maintenance
VACUUM ANALYZE load_test_data;

-- For production, tune autovacuum
ALTER TABLE load_test_data SET (autovacuum_vacuum_scale_factor = 0.01);
ALTER TABLE load_test_data SET (autovacuum_analyze_scale_factor = 0.005);
```

## Troubleshooting

### Issue: "Too many connections"
**Solution**:
1. Increase `max_connections` in PostgreSQL
2. Use PgBouncer for connection pooling
3. Reduce `CONCURRENT_WRITERS`
4. Reduce `DB_MAX_OPEN_CONNS`

### Issue: High latency (P99 > 5 seconds)
**Solution**:
1. Check database CPU - scale vertically if needed
2. Check disk I/O - upgrade to faster storage
3. Reduce `CONCURRENT_WRITERS`
4. Add appropriate indices
5. Check for lock contention

### Issue: Low throughput
**Solution**:
1. Increase `CONCURRENT_WRITERS`
2. Increase `BATCH_SIZE` for inserts
3. Optimize PostgreSQL configuration
4. Check network latency
5. Use connection pooler (PgBouncer)

### Issue: Out of memory (client or server)
**Solution**:
1. Reduce `CONCURRENT_WRITERS`
2. Reduce `BATCH_SIZE`
3. Reduce `READ_BATCH_SIZE`
4. Tune PostgreSQL `work_mem`

### Issue: CPU exhaustion
**Solution**:
1. Scale database server vertically
2. Reduce concurrent operations
3. Optimize queries
4. Consider read replicas for read-heavy workloads

## Graduated Load Testing

Don't jump directly to 10,000 users. Gradually increase:

```bash
# Stage 1: Baseline (100 users)
export CONCURRENT_WRITERS=100
export TEST_RUN_DURATION=300
./load-test-client

# Stage 2: Medium (1,000 users)
export CONCURRENT_WRITERS=1000
export TEST_RUN_DURATION=600
./load-test-client

# Stage 3: High (5,000 users)
export CONCURRENT_WRITERS=5000
export TEST_RUN_DURATION=600
./load-test-client

# Stage 4: Maximum (10,000 users)
export CONCURRENT_WRITERS=10000
export TEST_RUN_DURATION=600
./load-test-client

# Stage 5: Stress test (15,000+ users)
export CONCURRENT_WRITERS=15000
export TEST_RUN_DURATION=300
./load-test-client
```

Monitor metrics at each stage to identify bottlenecks.

## Expected Results

With proper configuration, you should achieve:

✅ **10,000+ concurrent operations**
✅ **100,000+ total operations per second**
✅ **P99 latency < 1 second** for most operations
✅ **< 0.1% error rate**
✅ **Stable performance** over extended periods

## Real-World Simulation

For realistic production simulation:

```bash
# E-commerce site simulation
READ_PERCENT=70      # Browse products, view details
INSERT_PERCENT=15    # New orders, user registrations
UPDATE_PERCENT=15    # Cart updates, inventory changes

# Social media simulation
READ_PERCENT=80      # Feed scrolling, profile views
INSERT_PERCENT=15    # New posts, comments
UPDATE_PERCENT=5     # Likes, edits

# Financial/Trading system
READ_PERCENT=40      # Price checks, balance queries
INSERT_PERCENT=30    # New transactions
UPDATE_PERCENT=30    # Transaction updates, status changes
```

## Next Steps

1. Start with your current configuration
2. Gradually increase concurrency
3. Monitor database metrics
4. Optimize based on bottlenecks
5. Test with realistic workload mix
6. Run extended tests (hours) to verify stability
