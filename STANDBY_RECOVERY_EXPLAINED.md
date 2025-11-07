# PostgreSQL Standby Recovery Issue - "Minimum recovery ending location" Keeps Increasing

## 🔍 What's Happening

### Your Situation:
1. **Primary + 2 Standbys** running normally
2. **Load test running** (high write activity)
3. **Standby shutdown** during load test
4. **Data directory removed** and **pg_basebackup** taken
5. **Standby restarted** but stuck in recovery
6. Error: "the database system is not yet accepting connections"
7. `pg_controldata` shows: **"Minimum recovery ending location keeps increasing"**

### The Root Cause

**This is NORMAL behavior, but you're experiencing a "catch-up" problem!**

## 📊 Understanding "Minimum recovery ending location"

### What is it?

```
Minimum recovery ending location = LSN (Log Sequence Number)

LSN = Write-Ahead Log position (like a bookmark in the WAL stream)
```

**What it means:**
- The **LSN that the standby must reach** before accepting connections
- During recovery, PostgreSQL replays WAL from the basebackup point to this LSN
- This LSN is **always the end of the last complete checkpoint** in the backup

### Why Does It Keep Increasing?

**Because you took a basebackup WHILE the primary was still receiving writes!**

```
Timeline of events:

T0: Standby crashes/removed
T1: pg_basebackup starts (takes 2 minutes to copy data)
    Primary LSN at start: 22/00000000
    
T2: During backup (primary still receiving writes!)
    Primary LSN now: 22/10000000  (+256MB of WAL)
    Load test is generating ~10 MB/s WAL
    
T3: pg_basebackup finishes
    "Minimum recovery ending location" set to: 22/20000000
    This is the LSN at the END of backup
    
T4: Standby starts recovery
    Must replay WAL from backup point to: 22/20000000
    But Primary has already moved to: 22/30000000
    
T5: While standby is replaying...
    Primary keeps writing (load test still running!)
    Primary LSN: 22/40000000, 22/50000000, 22/60000000...
    
    Standby is replaying: 22/21000000, 22/22000000, 22/23000000...
    
    The gap keeps GROWING because:
    - Primary writes at 10 MB/s
    - Standby replays at 5-8 MB/s (slower due to disk I/O)
    
Result: Standby can NEVER catch up while load test is running!
```

## 🎯 The Problem: "Chase the Dragon"

### Why Standby Can't Catch Up

```
Recovery Speed Calculation:

Primary write rate:     10 MB/s  (your load test)
Standby replay rate:    5-8 MB/s (typically 50-80% of write speed)

Gap increase rate:      10 - 6 = 4 MB/s
Time to catch up:       INFINITE (gap keeps growing!)
```

**Analogy:**
Imagine you're reading a book, but the author is writing new pages faster than you can read. You'll never finish!

## 🔥 Why This Happens with pg_basebackup During Load

### Normal pg_basebackup Process:

1. **Start backup**: `SELECT pg_start_backup()`
2. **Copy all data files** (takes time!)
3. **Copy WAL generated during backup**
4. **End backup**: `SELECT pg_stop_backup()`
5. **Backup includes**: 
   - Data files as of start
   - All WAL generated during backup
   - Minimum recovery point = end of backup

### The Issue During High Load:

```
Backup Duration:  2 minutes (typical)
WAL Generation:   10 MB/s × 120s = 1,200 MB = 75 WAL files

During backup:
- Primary generates 75 WAL files
- Basebackup includes these 75 files
- But by the time standby starts...
- Primary has generated 100 MORE files
- And keeps generating more!

Standby recovery:
- Must replay 75 files from backup
- Takes ~3 minutes to replay (slower than real-time)
- Meanwhile, primary generates 180 MORE files
- Gap: 180 new files - 75 replayed = 105 files behind
- This gap keeps GROWING!
```

## ✅ Solutions (Multiple Approaches)

### Solution 1: Stop Load Test During Recovery (Simplest)

```bash
# 1. Stop the load test
kubectl delete job pg-load-test-job -n demo

# 2. Wait for standby to catch up
# Monitor on standby:
watch -n 1 'tail -n 5 /var/pv/data/log/postgresql-*.log'

# 3. Check when recovery completes
# On standby:
psql -c "SELECT pg_is_in_recovery();"
# Should return 'f' (false) when ready

# 4. Verify replication status
# On primary:
psql -c "SELECT application_name, state, sync_state, 
         pg_wal_lsn_diff(pg_current_wal_lsn(), replay_lsn) as lag_bytes 
         FROM pg_stat_replication;"
```

**Why this works:**
- Primary stops generating new WAL
- Standby can catch up at its own pace
- Once caught up, switches to streaming replication

---

### Solution 2: Use Lower Recovery Speed Threshold (During Backup)

**Modify standby postgresql.conf BEFORE taking backup:**

```sql
-- On standby (before restart):
# postgresql.conf
recovery_min_apply_delay = 0          # No artificial delay
max_standby_streaming_delay = 30s     # Shorter delay
hot_standby = on                       # Allow connections during recovery
```

**Then take backup with better timing:**

```bash
# 1. On primary, create a checkpoint (makes backup faster)
psql -c "CHECKPOINT;"

# 2. Take backup with higher compression (faster transfer)
pg_basebackup -h primary-host -U postgres -D /var/pv/data \
  --wal-method=stream \
  --checkpoint=fast \
  --compress=9 \
  --progress

# 3. Start standby
pg_ctl start
```

---

### Solution 3: Use Pause/Resume Recovery (PostgreSQL 13+)

**Best solution for your use case!**

```bash
# 1. Take the basebackup as usual
pg_basebackup -h pg-ha-cluster -U postgres -D /var/pv/data --wal-method=stream

# 2. Configure standby to pause recovery automatically
echo "recovery_target = 'immediate'" >> /var/pv/data/postgresql.conf
echo "recovery_target_action = 'pause'" >> /var/pv/data/postgresql.conf

# 3. Start standby
pg_ctl start

# 4. Let it catch up to a good point, then resume
# On standby:
psql -c "SELECT pg_wal_replay_resume();"

# 5. Monitor recovery progress
psql -c "SELECT pg_last_wal_replay_lsn(), 
         pg_wal_lsn_diff(pg_last_wal_replay_lsn(), '0/0') as replay_bytes;"

# Compare with primary's current position
# On primary:
psql -c "SELECT pg_current_wal_lsn();"

# 6. When close enough (< 100MB), let it catch up
# Wait until lag is small, then recovery will complete
```

---

### Solution 4: Temporarily Reduce Load Test Intensity

**Adjust your load test to allow standby to catch up:**

```bash
# Edit ConfigMap to reduce write intensity temporarily
kubectl edit configmap pg-load-test-config -n demo

# Change:
CONCURRENT_WRITERS: "50"    # Was 100
INSERT_PERCENT: "40"         # Was 60
BATCH_SIZE: "50"             # Was 100

# Redeploy job
kubectl delete job pg-load-test-job -n demo
kubectl apply -f k8s/03-job.yaml

# This reduces WAL generation from 10 MB/s to ~4 MB/s
# Standby can now catch up (replays at 5-8 MB/s)
```

---

### Solution 5: Use pg_rewind Instead (Faster Alternative)

**If standby was running before and you just want to resync:**

```bash
# Instead of removing data directory and taking basebackup,
# use pg_rewind (much faster!)

# 1. Stop standby
pg_ctl stop -D /var/pv/data

# 2. Run pg_rewind (syncs differences only)
pg_rewind --target-pgdata=/var/pv/data \
          --source-server='host=pg-ha-cluster user=postgres password=XXX'

# 3. Update recovery configuration
cat > /var/pv/data/standby.signal << EOF
# Empty file to mark as standby
EOF

# 4. Start standby
pg_ctl start -D /var/pv/data

# This is MUCH faster because:
# - Only copies changed blocks (not entire database)
# - Takes seconds instead of minutes
# - Standby catches up quickly
```

---

## 🔬 Monitoring and Debugging

### Check Recovery Progress on Standby:

```sql
-- Current replay position
SELECT pg_last_wal_replay_lsn();

-- How much WAL has been replayed
SELECT pg_size_pretty(
  pg_wal_lsn_diff(pg_last_wal_replay_lsn(), '0/0')
);

-- Recovery status
SELECT pg_is_in_recovery();
```

### Check Primary WAL Position:

```sql
-- Current WAL position on primary
SELECT pg_current_wal_lsn();

-- Calculate lag (run on primary, comparing to standby's replay_lsn)
SELECT application_name,
       pg_size_pretty(pg_wal_lsn_diff(pg_current_wal_lsn(), replay_lsn)) as lag
FROM pg_stat_replication;
```

### Calculate Recovery Time Estimate:

```bash
# On standby, check pg_controldata
pg_controldata | grep "Minimum recovery ending location"

# Example output:
# Minimum recovery ending location: 22/3FC36E90

# Get current replay position
psql -c "SELECT pg_last_wal_replay_lsn();"

# Example output:
# 22/2FC36E90

# Calculate bytes remaining
psql -c "SELECT pg_wal_lsn_diff('22/3FC36E90'::pg_lsn, '22/2FC36E90'::pg_lsn);"

# Example output: 268435456 (256 MB remaining)

# Estimate time (assuming 6 MB/s replay rate):
echo "256 MB / 6 MB/s = 42 seconds remaining"
```

### Watch Recovery in Real-Time:

```bash
# On standby:
watch -n 2 "psql -c \"SELECT 
  pg_is_in_recovery() as recovering,
  pg_last_wal_replay_lsn() as replay_lsn,
  pg_size_pretty(pg_wal_lsn_diff(pg_last_wal_replay_lsn(), '0/0')) as replayed,
  now() - pg_last_xact_replay_timestamp() as replay_lag_time;\""
```

---

## 📋 Step-by-Step Fix for Your Situation

### Recommended Approach (Combines Best Practices):

```bash
# === Step 1: Stop the Load Test (Critical!) ===
kubectl delete job pg-load-test-job -n demo

# === Step 2: Check Current Lag ===
# On primary:
psql -U postgres -c "SELECT pg_current_wal_lsn();"
# Note this LSN (e.g., 22/5FC36E90)

# On standby:
psql -U postgres -c "SELECT pg_last_wal_replay_lsn();"
# Note this LSN (e.g., 22/1FC36E90)

# Calculate lag:
psql -U postgres -c "SELECT pg_wal_lsn_diff('22/5FC36E90'::pg_lsn, '22/1FC36E90'::pg_lsn);"
# If > 1GB, standby is far behind

# === Step 3: Let Standby Catch Up ===
# Monitor standby logs:
tail -f /var/pv/data/log/postgresql-*.log

# Watch for messages like:
# "consistent recovery state reached"
# "database system is ready to accept read-only connections"

# This may take 5-30 minutes depending on lag

# === Step 4: Verify Replication Status ===
# On primary:
psql -U postgres -c "
SELECT application_name, state, sync_state,
       pg_wal_lsn_diff(pg_current_wal_lsn(), replay_lsn) as lag_bytes,
       pg_size_pretty(pg_wal_lsn_diff(pg_current_wal_lsn(), replay_lsn)) as lag
FROM pg_stat_replication
ORDER BY application_name;
"

# Should now show your standby!

# === Step 5: Restart Load Test (Optional) ===
kubectl apply -f k8s/03-job.yaml

# === Step 6: Monitor Ongoing Replication ===
watch -n 5 "psql -U postgres -c 'SELECT * FROM pg_stat_replication;'"
```

---

## 🎓 Understanding the "Minimum recovery ending location"

### What Determines This Value?

```
When pg_basebackup runs:

1. Calls pg_start_backup() on primary
   → Records: "Backup Start LSN" = 22/00000000

2. Copies all data files (takes 2 minutes)
   → Primary continues writing (now at 22/10000000)

3. Calls pg_stop_backup() on primary
   → Records: "Backup End LSN" = 22/10000000
   → This becomes "Minimum recovery ending location"

4. Copies all WAL files generated during backup
   → Ensures standby can replay to "Backup End LSN"

5. Standby starts with:
   → Data files as of "Backup Start LSN" (22/00000000)
   → Must replay WAL to "Backup End LSN" (22/10000000)
   → "Minimum recovery ending location" = 22/10000000
```

### Why It Increases (During Your Observation):

**It SHOULDN'T increase after standby starts!**

If you're seeing it increase, it means:

```
Possibility 1: You're checking pg_controldata on the PRIMARY (not standby)
→ On primary, this value updates continuously

Possibility 2: Standby is still copying/processing the backup
→ As it extracts WAL from backup, this value updates

Possibility 3: You restarted standby multiple times
→ Each restart with hot_standby=on updates this value
```

### The Correct Behavior:

```
On Standby:
1. "Minimum recovery ending location" = Fixed value from backup
2. "Redo location" or "Replay location" = Continuously increasing
3. When "Replay location" >= "Minimum recovery ending location"
   → Recovery reaches consistent state
   → Standby accepts connections
```

---

## ⚡ Quick Reference

### Is Standby Catching Up?

```bash
# Run on standby every 10 seconds:
while true; do
  psql -c "SELECT pg_last_wal_replay_lsn();" | grep -v "pg_last"
  sleep 10
done

# If the LSN keeps increasing → Catching up ✓
# If the LSN is stuck → Problem ✗
```

### How Much Longer Until Ready?

```bash
# On standby:
STANDBY_LSN=$(psql -t -c "SELECT pg_last_wal_replay_lsn();")
PRIMARY_LSN=$(psql -h pg-ha-cluster -t -c "SELECT pg_current_wal_lsn();")

# Calculate lag
psql -c "SELECT 
  pg_size_pretty(pg_wal_lsn_diff('$PRIMARY_LSN'::pg_lsn, '$STANDBY_LSN'::pg_lsn)) as lag,
  pg_wal_lsn_diff('$PRIMARY_LSN'::pg_lsn, '$STANDBY_LSN'::pg_lsn) / (1024*1024) as lag_mb;"

# Estimate time (assume 6 MB/s replay rate):
# Time = lag_mb / 6 seconds
```

---

## 🎯 Summary

### Your Issue:
- ✅ **Behavior is NATURAL** (not a bug)
- ✅ **Root cause**: Taking basebackup during high-write load
- ✅ **Standby CAN'T catch up** while load test generates WAL faster than replay rate

### Solutions:
1. **✅ BEST**: Stop load test, let standby catch up, restart load test
2. **✅ GOOD**: Use pg_rewind instead of full basebackup
3. **✅ OK**: Reduce load test intensity temporarily
4. **⚠️ ADVANCED**: Use recovery pause/resume

### Prevention:
- Take basebackup during LOW activity periods
- Use pg_rewind for re-synchronization (faster)
- Consider using backup slots to preserve WAL
- Monitor replication lag continuously

### Key Takeaway:
```
Primary Write Speed > Standby Replay Speed = Standby Never Catches Up
Primary Write Speed < Standby Replay Speed = Standby Catches Up Eventually
```

**For your pg_rewind testing, this is expected and not a problem with your setup!**
