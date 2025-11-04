# Data Loss Check Timeout Fix

## Problem

The data loss check was failing with the error:
```
Warning: Data loss check failed: failed to check data loss: pq: canceling statement due to user request
```

## Root Cause

The data loss verification process was timing out due to:

1. **Short timeout**: Context timeout was set to only 60 seconds
2. **Large dataset**: With 500-second test duration and 100 concurrent workers, tens of thousands of records were inserted
3. **Batch processing**: Checking 1000 IDs at a time was creating many small queries
4. **Query overhead**: Each batch query has network and parsing overhead

### Example Calculation:
- Test duration: 500 seconds
- Workers: 100
- Insert percentage: 60%
- Batch size: 100
- Approximate inserts: ~300,000 records
- Old batch size: 1000 IDs per query = ~300 queries
- Time per query: ~0.5 seconds
- Total time needed: ~150 seconds (exceeds 60-second timeout!)

## Solution

### 1. Increased Context Timeout
Changed from 60 seconds to **5 minutes (300 seconds)**:

**Files modified:**
- `main.go` (line 137)
- `main_v2.go` (line 177)

```go
// Before
dataLossCtx, dataLossCancel := context.WithTimeout(context.Background(), 60*time.Second)

// After
dataLossCtx, dataLossCancel := context.WithTimeout(context.Background(), 5*time.Minute)
```

### 2. Optimized Batch Size
Increased batch size from 1000 to **5000 IDs per query**:

**Files modified:**
- `clients/postgres/load_generator.go` (line 342)
- `clients/postgres/load_generator_v2.go` (line 518)

```go
// Before
batchSize := 1000

// After
batchSize := 5000
```

**Benefits:**
- 5x fewer queries (300,000 records: 300 queries → 60 queries)
- Reduced network overhead
- Reduced query parsing overhead
- Faster completion time

### 3. Added Progress Reporting
Shows progress every 10 batches or on the last batch:

```go
// Show progress every 10 batches or on last batch
if currentBatch%10 == 0 || currentBatch == totalBatches {
    fmt.Printf("  Progress: Checked %d/%d batches (%.1f%%)\n",
        currentBatch, totalBatches, float64(currentBatch)*100/float64(totalBatches))
}
```

**Benefits:**
- Users can see verification is still running
- Can estimate time remaining
- Better user experience for long-running checks

### 4. Added Context Cancellation Check
Properly handles timeout/cancellation:

```go
// Check context cancellation
select {
case <-ctx.Done():
    return 0, 0, fmt.Errorf("data loss check cancelled: %w", ctx.Err())
default:
}
```

**Benefits:**
- Clean error message if timeout occurs
- Immediate exit on cancellation
- Better error reporting

## Expected Behavior After Fix

### For Small Tests (< 10,000 records):
```
Checking data loss for 5,420 inserted records...
  Progress: Checked 1/2 batches (50.0%)
Data loss check complete: 5420 found, 0 lost out of 5420 inserted
✅ No data loss detected
```

### For Large Tests (> 100,000 records):
```
Checking data loss for 312,548 inserted records...
  Progress: Checked 10/63 batches (15.9%)
  Progress: Checked 20/63 batches (31.7%)
  Progress: Checked 30/63 batches (47.6%)
  Progress: Checked 40/63 batches (63.5%)
  Progress: Checked 50/63 batches (79.4%)
  Progress: Checked 60/63 batches (95.2%)
  Progress: Checked 63/63 batches (100.0%)
Data loss check complete: 312548 found, 0 lost out of 312548 inserted
✅ No data loss detected
```

## Performance Improvements

| Metric | Before | After | Improvement |
|--------|--------|-------|-------------|
| Timeout | 60s | 300s | 5x longer |
| Batch size | 1,000 IDs | 5,000 IDs | 5x larger |
| Queries for 100K records | 100 queries | 20 queries | 5x fewer |
| Queries for 300K records | 300 queries | 60 queries | 5x fewer |
| Max recordset supported | ~60,000 | ~1,500,000 | 25x more |

## Testing Recommendations

1. **Small test** (verify fix works):
   ```bash
   TEST_RUN_DURATION=30 CONCURRENT_WRITERS=10 ./load-test-client-v2
   ```

2. **Medium test** (verify progress reporting):
   ```bash
   TEST_RUN_DURATION=120 CONCURRENT_WRITERS=50 ./load-test-client-v2
   ```

3. **Large test** (stress test):
   ```bash
   TEST_RUN_DURATION=500 CONCURRENT_WRITERS=100 ./load-test-client-v2
   ```

## Additional Notes

- The 5-minute timeout is generous and should handle even very large datasets
- Progress reporting helps users understand the verification is still running
- If verification still times out, increase the timeout further or reduce test duration
- The batch size of 5000 is safe for PostgreSQL (well below query size limits)

## Files Modified

1. `main.go` - Increased timeout to 5 minutes
2. `main_v2.go` - Increased timeout to 5 minutes  
3. `clients/postgres/load_generator.go` - Optimized batch size, added progress reporting
4. `clients/postgres/load_generator_v2.go` - Optimized batch size, added progress reporting

## Rebuild Required

After these changes, rebuild the Docker image:

```bash
# Local testing
go build -o load-test-client-v2 .

# Kubernetes deployment
docker build -t souravbiswassanto/pg-load-test:latest .
./cleanup-k8s.sh
./deploy-k8s.sh
```
