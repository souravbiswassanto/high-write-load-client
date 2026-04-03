# Why "Minimum recovery ending location" Keeps Increasing on Standby

## 🔍 Your Specific Question

**You ran `pg_controldata` on the STANDBY and saw:**
```bash
pg_controldata | grep "Minimum recovery ending location"
Minimum recovery ending location:     22/3FC36E90

# Wait 10 seconds, run again:
Minimum recovery ending location:     22/40C36E90  # INCREASED!

# Wait another 10 seconds:
Minimum recovery ending location:     22/41C36E90  # INCREASED AGAIN!
```

## ⚠️ This Should NOT Happen Under Normal Circumstances

**The "Minimum recovery ending location" is supposed to be FIXED after backup completes!**

## 🔬 Root Causes - Why This Happens

### Cause 1: **pg_basebackup Still Running or Just Completed**

When you took the basebackup WHILE the primary was under heavy write load:

```
What pg_basebackup does:

1. Calls pg_start_backup('label') on primary
   → Primary records: Backup Start Checkpoint LSN
   
2. Copies all data files (THIS TAKES TIME - 2-5 minutes)
   → While copying, primary continues writing
   → New checkpoints happen on primary
   
3. During step 2, if a checkpoint occurs on primary:
   → pg_stop_backup() records a LATER checkpoint LSN
   → This becomes the "Minimum recovery ending location"
   
4. If another checkpoint happens during backup:
   → The "Minimum recovery ending location" moves forward AGAIN
   → This is recorded in the backup_label file
```

**Evidence this is happening:**
```bash
# On standby, check if backup_label file exists:
ls -la /var/pv/data/backup_label

# If it exists and standby is running, check its contents:
cat /var/pv/data/backup_label

# Look for:
START WAL LOCATION: 22/3FC36E90 (file 000000010000002200000003)
CHECKPOINT LOCATION: 22/3FC36E90
START TIME: 2025-11-05 13:30:00 UTC
LABEL: pg_basebackup base backup

# This LSN in backup_label determines the minimum recovery point
```

### Cause 2: **Continuous Checkpoints During pg_basebackup**

**This is THE KEY ISSUE in your case!**

```
Your settings:
- checkpoint_timeout = 300s (5 minutes)
- max_wal_size = 1GB
- Write rate = 10 MB/s

Timeline during pg_basebackup:

T=0s:    pg_basebackup starts
         pg_start_backup() called → Checkpoint forced
         "Minimum recovery ending location" = 22/3FC36E90
         
T=120s:  Max_wal_size reached (1GB at 10 MB/s = 100s)
         AUTOMATIC CHECKPOINT triggered
         pg_basebackup updates: "Minimum recovery ending location" = 22/40000000
         
T=240s:  Another max_wal_size checkpoint
         pg_basebackup updates: "Minimum recovery ending location" = 22/48000000
         
T=300s:  pg_basebackup completes
         pg_stop_backup() uses LATEST checkpoint
         Final "Minimum recovery ending location" = 22/48000000
```

**This is why it keeps increasing during backup!**

### Cause 3: **Standby in Crash Recovery Mode (Not Streaming Recovery)**

If the standby is in **crash recovery** instead of **streaming replication recovery**:

```bash
# Check standby PostgreSQL logs:
tail -100 /var/pv/data/log/postgresql-*.log | grep -i "recovery"

# Look for these messages:

# CRASH RECOVERY (BAD - causes increasing minimum recovery point):
LOG:  database system was interrupted; last known up at...
LOG:  entering standby mode
LOG:  redo starts at 22/3FC36E90
LOG:  consistent recovery state will be reached at 22/45000000  # THIS KEEPS UPDATING!

# vs.

# STREAMING RECOVERY (GOOD - fixed minimum recovery point):
LOG:  entering standby mode
LOG:  redo starts at 22/3FC36E90
LOG:  consistent recovery state reached at 22/45000000  # FIXED VALUE
LOG:  database system is ready to accept read-only connections
```

**Why crash recovery updates the minimum recovery point:**
- Crash recovery reads the backup_label file
- It processes each checkpoint record found in WAL
- Each checkpoint updates the "consistent recovery point"
- This appears as increasing "Minimum recovery ending location"

### Cause 4: **Missing or Incorrect standby.signal File**

```bash
# On standby, check if standby.signal exists:
ls -la /var/pv/data/standby.signal

# If MISSING, PostgreSQL thinks it's in crash recovery, not standby mode!
# Create it:
touch /var/pv/data/standby.signal

# Then restart:
pg_ctl restart
```

### Cause 5: **Primary Still Checkpointing While Standby Reads WAL**

Even AFTER pg_basebackup completes, if:
- Standby is reading archived WAL files (not streaming)
- Primary keeps checkpointing
- Standby processes these checkpoint records
- Updates its view of "minimum recovery ending location"

```
Flow:

1. pg_basebackup completes with minimum recovery = 22/3FC36E90
2. Standby starts, begins replaying WAL
3. Standby reads WAL file containing checkpoint record at 22/40000000
4. Standby updates: "minimum recovery ending location" = 22/40000000
5. Standby reads next WAL file with checkpoint at 22/42000000
6. Standby updates again: "minimum recovery ending location" = 22/42000000
7. This continues until standby catches up to primary's current position
```

**This is NORMAL during the initial catch-up phase!**

## 🎯 The Real Answer: Which Scenario Are You In?

### Scenario A: **pg_basebackup Still Running** ✓ LIKELY

```bash
# Check if pg_basebackup process is still running:
ps aux | grep pg_basebackup

# Check standby data directory size growing:
watch -n 5 'du -sh /var/pv/data'

# If size still increasing → backup still in progress
# The minimum recovery location updates as primary checkpoints during backup
```

**Solution:** Wait for pg_basebackup to complete!

---

### Scenario B: **Standby in Crash Recovery (Not Streaming Mode)** ✓ VERY LIKELY

```bash
# Check recovery mode:
psql -c "SELECT pg_is_in_recovery(), 
         pg_is_wal_replay_paused(),
         pg_last_wal_receive_lsn(),
         pg_last_wal_replay_lsn();"

# If pg_last_wal_receive_lsn() is NULL:
# → Standby is NOT streaming, just replaying archived WAL
# → This causes the behavior you're seeing
```

**Why this happens:**
```
After pg_basebackup with --wal-method=stream:
1. All WAL files from backup are in pg_wal/
2. Standby starts replaying these files
3. For each WAL file, it finds checkpoint records
4. Each checkpoint updates "minimum recovery ending location"
5. This continues until ALL backup WAL is replayed
6. Then standby switches to streaming mode
7. THEN "minimum recovery ending location" becomes fixed
```

**Solution:** This is NORMAL! Wait for crash recovery to finish.

---

### Scenario C: **Very Frequent Checkpoints on Primary** ✓ YOUR ISSUE

```
Your settings cause checkpoints every ~100 seconds:
- max_wal_size = 1GB
- Write rate = 10 MB/s
- Time to fill: 1024 MB / 10 MB/s = 102 seconds

During 5-minute pg_basebackup:
- 3 automatic checkpoints occur
- Each updates the minimum recovery point
- Final value is much higher than initial value
```

**Solution:** Increase max_wal_size BEFORE taking backup!

```sql
-- On primary, before taking backup:
ALTER SYSTEM SET max_wal_size = '8GB';
SELECT pg_reload_conf();

-- Force a checkpoint to apply:
CHECKPOINT;

-- Now take backup:
-- pg_basebackup will have fewer checkpoint updates
```

---

## 🔧 How to Fix Your Situation

### Step-by-Step Fix:

```bash
# === 1. Verify What's Actually Happening ===

# On standby, check control data:
pg_controldata | grep -A 5 "Minimum recovery ending location"

# Output example:
# Minimum recovery ending location:     22/3FC36E90
# Min recovery ending loc's timeline:   1
# Backup start location:                22/3FC36E90
# Backup end location:                  22/40000000  # Note if this is DIFFERENT
# End-of-backup record required:        yes

# === 2. Check if backup_label Still Exists ===

ls -la /var/pv/data/backup_label

# If EXISTS:
# - PostgreSQL is still in "restoring from backup" mode
# - This is why minimum recovery point updates
# - This is NORMAL until recovery reaches backup end point

# === 3. Check Standby Logs ===

tail -50 /var/pv/data/log/postgresql-*.log

# Look for:
# "redo in progress, elapsed time: XXX s, current LSN: 22/XXXXXXXX"
# This shows recovery is progressing

# === 4. Calculate How Much Longer ===

# Get current replay position:
REPLAY_LSN=$(psql -t -c "SELECT pg_last_wal_replay_lsn();" 2>/dev/null || echo "0/0")

# Get minimum recovery ending location:
MIN_RECOVERY=$(pg_controldata | grep "Minimum recovery ending location" | awk '{print $5}')

# Calculate remaining:
psql -c "SELECT pg_wal_lsn_diff('$MIN_RECOVERY'::pg_lsn, '$REPLAY_LSN'::pg_lsn) as remaining_bytes,
         pg_size_pretty(pg_wal_lsn_diff('$MIN_RECOVERY'::pg_lsn, '$REPLAY_LSN'::pg_lsn)) as remaining;"

# === 5. Wait for "Consistent Recovery State Reached" ===

# Monitor logs until you see:
tail -f /var/pv/data/log/postgresql-*.log

# Wait for these key messages:
# 1. "consistent recovery state reached at 22/XXXXXXXX"
# 2. "database system is ready to accept read-only connections"
# 3. "started streaming WAL from primary"

# === 6. After Recovery Completes ===

# The minimum recovery ending location will STOP increasing
# It becomes a historical value (where backup ended)

# Check replication status:
# On primary:
psql -c "SELECT * FROM pg_stat_replication;"
```

---

## 📊 What's Actually Happening (Technical Deep Dive)

### Understanding the Control File Fields:

```c
// From PostgreSQL source code (pg_control.h)

typedef struct ControlFileData
{
    ...
    XLogRecPtr  minRecoveryPoint;        // Minimum recovery ending location
    TimeLineID  minRecoveryPointTLI;     
    XLogRecPtr  backupStartPoint;        // Where backup started
    XLogRecPtr  backupEndPoint;          // Where backup ended
    bool        backupEndRequired;       // Must reach backupEndPoint
    ...
} ControlFileData;
```

**What happens during recovery:**

```c
// Simplified PostgreSQL recovery logic:

while (in_recovery) {
    record = ReadWALRecord();
    
    if (record->type == CHECKPOINT) {
        // Update minRecoveryPoint to this checkpoint
        ControlFile->minRecoveryPoint = record->lsn;
        UpdateControlFile();  // ← THIS is why you see it increase!
    }
    
    if (ReachedMinRecoveryPoint() && ReachedBackupEndPoint()) {
        // Recovery complete!
        RemoveBackupLabel();
        in_recovery = false;
    }
    
    ReplayWALRecord(record);
}
```

### Why It Increases:

```
During crash recovery with backup_label:

WAL Record 1: CHECKPOINT at 22/3FC36E90
→ minRecoveryPoint = 22/3FC36E90
→ pg_controldata shows 22/3FC36E90

WAL Record 100: CHECKPOINT at 22/40000000  
→ minRecoveryPoint = 22/40000000  ← UPDATED!
→ pg_controldata shows 22/40000000

WAL Record 200: CHECKPOINT at 22/42000000
→ minRecoveryPoint = 22/42000000  ← UPDATED AGAIN!
→ pg_controldata shows 22/42000000

This continues until:
→ minRecoveryPoint >= backupEndPoint
→ Then recovery can reach consistent state
```

---

## ✅ The Direct Answer to Your Question

### Why Does "Minimum recovery ending location" Keep Increasing on Standby?

**It increases because:**

1. **Your pg_basebackup took several minutes** while primary was writing at 10 MB/s
2. **During those minutes, 3-4 checkpoints occurred** on the primary (due to max_wal_size=1GB)
3. **Each checkpoint was captured in the WAL** streamed during backup
4. **Standby is now replaying those WAL files** in order
5. **Each time standby encounters a checkpoint record** in WAL, it updates the "minimum recovery ending location"
6. **This continues until standby reaches the "backup end point"** from backup_label

### Is This Normal?

**YES! This is completely normal behavior during the initial recovery phase after pg_basebackup!**

The "minimum recovery ending location" will:
- ✅ Keep increasing as standby replays WAL with checkpoint records
- ✅ Stop increasing once standby reaches "consistent recovery state"
- ✅ Become a fixed historical value after recovery completes
- ✅ Then standby starts streaming replication (pg_stat_replication shows it)

### When Will It Stop?

**It will stop increasing when:**

1. Standby replays all WAL up to the "backup end point"
2. Standby reaches "consistent recovery state"
3. Standby logs: "database system is ready to accept read-only connections"
4. Standby switches from crash recovery to streaming replication
5. PRIMARY's `pg_stat_replication` shows your standby

**Time estimate:**
```
WAL to replay = backupEndPoint - backupStartPoint
If 2GB of WAL was generated during backup:
Recovery time = 2048 MB / 6 MB/s (replay rate) = ~5-6 minutes
```

### What You Should See Eventually:

```bash
# After recovery completes:

# On standby:
pg_controldata | grep "Minimum recovery ending location"
# Minimum recovery ending location:     22/48000000  # FIXED, doesn't change anymore

psql -c "SELECT pg_is_in_recovery();"
# t  (still true, but in streaming mode now)

psql -c "SELECT pg_last_wal_receive_lsn(), pg_last_wal_replay_lsn();"
# Both should be similar and continuously increasing

# On primary:
psql -c "SELECT application_name FROM pg_stat_replication;"
# Should show your standby name!
```

---

## 🎯 Summary

**Direct Answer:** The "minimum recovery ending location" keeps increasing on your standby because:

1. ✅ You took pg_basebackup during heavy write load
2. ✅ Multiple checkpoints occurred during the backup (due to small max_wal_size)
3. ✅ Standby is now replaying all that WAL
4. ✅ Each checkpoint record updates the minimum recovery point
5. ✅ This is NORMAL and EXPECTED behavior
6. ✅ It will stop once standby reaches consistent state

**What to Do:** 
- ⏰ **Just wait!** Let the standby finish replaying WAL
- 📊 Monitor: `tail -f /var/pv/data/log/postgresql-*.log`
- ✅ Success when you see: "database system is ready to accept read-only connections"
- 🎯 Then: `pg_stat_replication` on primary will show your standby

**Prevention for Next Time:**
```sql
-- Before taking backup:
ALTER SYSTEM SET max_wal_size = '8GB';  -- Reduce checkpoint frequency
```

**Your scenario is completely normal! The standby WILL catch up and appear in pg_stat_replication, you just need to wait for the initial recovery to complete.**
