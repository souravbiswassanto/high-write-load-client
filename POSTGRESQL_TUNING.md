# PostgreSQL Performance Tuning for High-Write Load

## Current Issues Identified

Based on your `pg_settings.tx`, several critical configurations are limiting performance:

### 1. **Shared Buffers - CRITICALLY LOW** ⚠️
```
Current: shared_buffers = 256 MB (32768 * 8kB)
Problem: WAY too small for a high-write workload
```

### 2. **WAL Buffers - TOO SMALL** ⚠️
```
Current: wal_buffers = 8 MB (1024 * 8kB)
Problem: Insufficient for high transaction rate
```

### 3. **Work Memory - TOO SMALL** ⚠️
```
Current: work_mem = 4 MB (4096 kB)
Problem: Limits sort and hash operations
```

### 4. **Maintenance Work Memory - TOO SMALL** ⚠️
```
Current: maintenance_work_mem = 64 MB
Problem: Slows down index creation and maintenance
```

### 5. **Checkpoint Settings - SUBOPTIMAL**
```
Current: 
  - checkpoint_timeout = 300s (5 minutes) - OK
  - max_wal_size = 1 GB - TOO SMALL for high-write load
  - min_wal_size = 80 MB - TOO SMALL
```

### 6. **Synchronous Commit - ENABLED** (if you can tolerate some data loss)
```
Current: synchronous_commit = on
Impact: Every commit waits for WAL to be written to disk
```

## 🔧 Recommended Tuning for High-Write Workload

### Step 1: Calculate Available Memory
First, determine your system resources:
```bash
# Check total RAM
free -h

# Check current PostgreSQL memory usage
kubectl exec -it <postgres-pod> -n demo -- top
```

### Step 2: Apply These Settings

Create a ConfigMap or edit `postgresql.conf`:

```sql
-- ============================================
-- MEMORY SETTINGS (Assume 8GB RAM available)
-- ============================================

-- Shared Buffers: 25% of RAM for dedicated DB server
-- For 8GB RAM: 2GB, For 16GB RAM: 4GB, For 32GB RAM: 8GB
shared_buffers = 2GB

-- Work Memory: For sorts/hashes per operation
-- Formula: (Total RAM - shared_buffers) / (max_connections * 3)
-- For 100 connections: ~20MB per connection
work_mem = 20MB

-- Maintenance Work Memory: For VACUUM, CREATE INDEX
-- 5-10% of RAM
maintenance_work_mem = 512MB

-- ============================================
-- WAL SETTINGS (Critical for Write Performance)
-- ============================================

-- WAL Buffers: 16MB is good for high-write
-- Auto-sized to 3% of shared_buffers, min 64kB, max 16MB
wal_buffers = 16MB

-- Maximum WAL size before checkpoint
-- Larger = fewer checkpoints = better write performance
-- Recommended: 4-8GB for high-write workload
max_wal_size = 4GB

-- Minimum WAL size to keep
min_wal_size = 1GB

-- Checkpoint completion target
-- 0.9 = spread checkpoint I/O over 90% of checkpoint interval
checkpoint_completion_target = 0.9

-- Checkpoint timeout
-- Keep at 5-10 minutes for high-write
checkpoint_timeout = 600s

-- ============================================
-- WRITE PERFORMANCE OPTIMIZATIONS
-- ============================================

-- Synchronous Commit (CRITICAL FOR SPEED)
-- Options:
--   on        = Wait for WAL write to disk (SLOWEST, safest)
--   remote_write = Wait for WAL sent to standby (medium)
--   local     = Wait for WAL in OS cache (FASTER, some risk)
--   off       = Don't wait at all (FASTEST, risk of data loss)
--
-- For load testing: Use 'off' or 'local'
-- For production with HA: Use 'remote_write' or 'on'
synchronous_commit = off  -- Or 'local' for slightly more safety

-- Full Page Writes (can be disabled if using checksums)
-- Keeping 'on' is safer, but 'off' improves write performance
full_page_writes = on  -- Keep on for safety

-- WAL Compression (helps reduce WAL size)
wal_compression = on

-- WAL Writer Delay
-- Lower = more frequent WAL flushes = lower latency
wal_writer_delay = 10ms  -- Default is 200ms

-- ============================================
-- CHECKPOINT TUNING
-- ============================================

-- Background writer settings
bgwriter_delay = 10ms              -- Default: 200ms
bgwriter_lru_maxpages = 1000       -- Default: 100
bgwriter_lru_multiplier = 10.0     -- Default: 2.0

-- Flush pages to disk more aggressively
-- Helps reduce checkpoint I/O spikes
bgwriter_flush_after = 512kB       -- Default: 512kB (64 * 8kB)

-- ============================================
-- CONNECTION POOLING
-- ============================================

-- Max connections (you already have 100 - good)
max_connections = 100

-- Reserved for superuser
superuser_reserved_connections = 3

-- ============================================
-- AUTOVACUUM TUNING (Important for sustained writes)
-- ============================================

-- Make autovacuum more aggressive
autovacuum_naptime = 10s                      -- Default: 60s
autovacuum_vacuum_scale_factor = 0.05         -- Default: 0.2
autovacuum_analyze_scale_factor = 0.05        -- Default: 0.1
autovacuum_vacuum_cost_limit = 2000           -- Default: -1 (uses vacuum_cost_limit)
autovacuum_max_workers = 4                    -- Default: 3

-- ============================================
-- QUERY PLANNER
-- ============================================

-- Effective cache size: Total available memory for caching
-- Should be ~50-75% of total RAM
effective_cache_size = 6GB  -- For 8GB system

-- Random page cost (lower for SSD)
random_page_cost = 1.1  -- Default: 4 (for HDD), use 1.1 for SSD

-- Effective I/O concurrency (for SSD/RAID)
effective_io_concurrency = 200  -- Default: 1

-- ============================================
-- LOGGING (for monitoring WAL generation)
-- ============================================

-- Log checkpoints to monitor WAL generation rate
log_checkpoints = on

-- Log autovacuum runs
log_autovacuum_min_duration = 0  -- Log all autovacuum runs

-- Log slow queries
log_min_duration_statement = 1000  -- Log queries > 1 second
```

## 📊 Expected Impact

### Before Tuning:
- **WAL Generation**: 1 file per 2 seconds = ~8 files/min = 480 files/hour
- **WAL Size**: 16MB per file = 128 MB/min = 7.5 GB/hour
- **Checkpoints**: Frequent (every ~2 minutes due to max_wal_size=1GB)
- **Write Throughput**: Limited by small buffers and frequent checkpoints

### After Tuning:
- **WAL Generation**: Reduced to 1 file per 10-30 seconds (depending on load)
- **Checkpoints**: Every 8-10 minutes (controlled by checkpoint_timeout)
- **Write Throughput**: 2-3x improvement
- **Latency**: 30-50% reduction

## 🚀 How to Apply (Kubernetes)

### Option 1: Using ConfigMap (Recommended)

Create `postgres-tuning-configmap.yaml`:
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: postgres-tuning
  namespace: demo
data:
  custom.conf: |
    # High-Write Performance Tuning
    
    # Memory
    shared_buffers = 2GB
    work_mem = 20MB
    maintenance_work_mem = 512MB
    effective_cache_size = 6GB
    
    # WAL
    wal_buffers = 16MB
    max_wal_size = 4GB
    min_wal_size = 1GB
    checkpoint_timeout = 600s
    checkpoint_completion_target = 0.9
    wal_compression = on
    wal_writer_delay = 10ms
    
    # Write Performance
    synchronous_commit = off  # or 'local'
    
    # Background Writer
    bgwriter_delay = 10ms
    bgwriter_lru_maxpages = 1000
    bgwriter_lru_multiplier = 10.0
    
    # Autovacuum
    autovacuum_naptime = 10s
    autovacuum_vacuum_scale_factor = 0.05
    autovacuum_analyze_scale_factor = 0.05
    autovacuum_vacuum_cost_limit = 2000
    autovacuum_max_workers = 4
    
    # Planner
    random_page_cost = 1.1
    effective_io_concurrency = 200
    
    # Logging
    log_checkpoints = on
    log_autovacuum_min_duration = 0
```

Apply it:
```bash
kubectl apply -f postgres-tuning-configmap.yaml

# Update your PostgreSQL deployment to mount this ConfigMap
# and include it in postgresql.conf with:
# include = '/path/to/custom.conf'

# Restart PostgreSQL to apply
kubectl rollout restart statefulset pg-ha-cluster -n demo
```

### Option 2: Direct ALTER SYSTEM (Quick Test)

Connect to PostgreSQL and run:
```sql
-- Memory
ALTER SYSTEM SET shared_buffers = '2GB';
ALTER SYSTEM SET work_mem = '20MB';
ALTER SYSTEM SET maintenance_work_mem = '512MB';
ALTER SYSTEM SET effective_cache_size = '6GB';

-- WAL
ALTER SYSTEM SET wal_buffers = '16MB';
ALTER SYSTEM SET max_wal_size = '4GB';
ALTER SYSTEM SET min_wal_size = '1GB';
ALTER SYSTEM SET checkpoint_timeout = '600s';
ALTER SYSTEM SET wal_compression = 'on';
ALTER SYSTEM SET wal_writer_delay = '10ms';

-- Write Performance (CRITICAL!)
ALTER SYSTEM SET synchronous_commit = 'off';  -- Or 'local'

-- Background Writer
ALTER SYSTEM SET bgwriter_delay = '10ms';
ALTER SYSTEM SET bgwriter_lru_maxpages = 1000;
ALTER SYSTEM SET bgwriter_lru_multiplier = 10.0;

-- Autovacuum
ALTER SYSTEM SET autovacuum_naptime = '10s';
ALTER SYSTEM SET autovacuum_vacuum_scale_factor = 0.05;
ALTER SYSTEM SET autovacuum_vacuum_cost_limit = 2000;

-- Planner
ALTER SYSTEM SET random_page_cost = 1.1;
ALTER SYSTEM SET effective_io_concurrency = 200;

-- Apply changes (requires restart)
SELECT pg_reload_conf();
```

**Note**: Some settings like `shared_buffers` require a full restart:
```bash
kubectl rollout restart statefulset pg-ha-cluster -n demo
```

## 🎯 Priority Changes (If Limited by Resources)

If you can't increase all settings, prioritize these:

### TOP 3 CRITICAL CHANGES:
1. **`synchronous_commit = off`** - Immediate 2-3x write speedup
2. **`max_wal_size = 4GB`** - Reduces checkpoint frequency
3. **`shared_buffers = 2GB`** - More data cached in memory

### Next 3 Important:
4. **`wal_buffers = 16MB`** - Better WAL buffering
5. **`work_mem = 20MB`** - Better query performance
6. **`checkpoint_timeout = 600s`** - Spread out checkpoints

## 📈 Monitoring After Changes

### Check WAL Generation Rate:
```sql
-- Monitor WAL generation
SELECT 
    pg_current_wal_lsn(),
    pg_size_pretty(
        pg_wal_lsn_diff(pg_current_wal_lsn(), '0/0')
    ) as total_wal_generated;

-- Check checkpoint stats
SELECT * FROM pg_stat_bgwriter;

-- Check WAL activity
SELECT * FROM pg_stat_wal;
```

### Check WAL Files:
```bash
# From inside the pod
ls -lh /var/pv/data/pg_wal/ | wc -l
```

### Monitor Performance:
```sql
-- Check current buffer usage
SELECT * FROM pg_buffercache_summary();

-- Check table bloat
SELECT schemaname, tablename, 
       pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) as size
FROM pg_tables 
WHERE tablename = 'load_test_data';
```

## ⚠️ Important Notes

### 1. Data Loss Risk with `synchronous_commit = off`
- **What it means**: Commits return success before WAL is written to disk
- **Risk**: Power failure could lose last ~1 second of commits
- **Safe for**: Load testing, analytics, non-critical data
- **NOT safe for**: Financial transactions, critical production data

### 2. Memory Requirements
- The tuning above assumes **8GB+ RAM** available
- Adjust proportionally for your system:
  - 4GB RAM: Use 1GB shared_buffers, 2GB max_wal_size
  - 16GB RAM: Use 4GB shared_buffers, 8GB max_wal_size
  - 32GB RAM: Use 8GB shared_buffers, 16GB max_wal_size

### 3. Storage I/O
- **SSD strongly recommended** for WAL and data directories
- For HDD: Keep `random_page_cost = 4`
- Consider separate volumes for WAL (`pg_wal` directory)

## 🔬 Testing the Impact

After applying changes, run your load test and compare:

```bash
# Before tuning - Monitor WAL
watch -n 1 'ls /var/pv/data/pg_wal/ | wc -l'

# Run your load test
kubectl apply -f k8s/03-job.yaml

# Check performance metrics
kubectl logs -f -n demo <load-test-pod>
```

Expected improvements:
- ✅ **WAL files**: From 1/2s to 1/10-30s
- ✅ **Throughput**: 2-3x increase
- ✅ **Latency P95**: 30-50% reduction
- ✅ **Checkpoint frequency**: From every 2min to every 8-10min

## 📚 Additional Resources

- [PostgreSQL WAL Configuration](https://www.postgresql.org/docs/16/wal-configuration.html)
- [PostgreSQL Performance Tuning](https://wiki.postgresql.org/wiki/Tuning_Your_PostgreSQL_Server)
- [pgtune](https://pgtune.leopard.in.ua/) - Automated configuration generator
