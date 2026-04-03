# PostgreSQL High Write Load Testing Client

A high-performance load testing client for PostgreSQL designed to simulate production-level write workloads. This tool helps you test database performance under heavy write loads with configurable concurrent writers, batch operations, and mixed workloads (inserts and updates).

## Features

- ✅ **Connection Safety**: Checks `max_connections` and active connections before starting, ensuring at least N connections remain available
- ✅ **Mixed Workload**: Configurable distribution of INSERT and UPDATE operations
- ✅ **Batch Operations**: Efficient bulk inserts for maximum throughput
- ✅ **Concurrent Workers**: Multiple goroutines simulating parallel write operations
- ✅ **Real-time Metrics**: Track throughput, latency (avg, P95, P99), errors, and connection pool stats
- ✅ **Kubernetes & Non-Kubernetes**: Works both inside and outside Kubernetes using environment variables
- ✅ **Graceful Shutdown**: Handles SIGINT/SIGTERM for clean test termination

## Architecture

```
.
├── config/             # Configuration management
│   └── config.go       # Environment-based config loader
├── metrics/            # Performance metrics tracking
│   └── metrics.go      # Real-time metrics collection and reporting
├── clients/
│   └── postgres/       # PostgreSQL specific implementation
│       ├── client.go              # Original client structs
│       ├── connection_manager.go  # Safe connection management
│       └── load_generator.go      # Load generation engine
└── main.go            # Application orchestrator
```

## Quick Start

### Prerequisites

- Go 1.24 or higher
- PostgreSQL database (local or remote)
- Database credentials

### Installation

```bash
# Clone the repository
git clone https://github.com/souravbiswassanto/high-write-load-client.git
cd high-write-load-client

# Install dependencies
go mod download
```

### Configuration

All configuration is done via environment variables:

#### Database Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `DB_HOST` | PostgreSQL host | `localhost` |
| `DB_PORT` | PostgreSQL port | `5432` |
| `DB_USER` | Database user | `postgres` |
| `DB_PASSWORD` | Database password | `` |
| `DB_NAME` | Database name | `testdb` |
| `DB_SSL_MODE` | SSL mode (disable/require/verify-ca/verify-full) | `disable` |
| `DB_MAX_OPEN_CONNS` | Maximum open connections in pool | `50` |
| `DB_MAX_IDLE_CONNS` | Maximum idle connections in pool | `10` |
| `DB_MIN_FREE_CONNS` | Minimum free connections to leave available | `5` |

#### Load Test Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `CONCURRENT_WRITERS` | Number of concurrent worker goroutines | `10` |
| `TEST_RUN_DURATION` | Test duration in seconds | `300` (5 minutes) |
| `BATCH_SIZE` | Number of records per batch insert | `100` |
| `REPORT_INTERVAL` | Metrics reporting interval in seconds | `10` |

#### Workload Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `INSERT_PERCENT` | Percentage of insert operations (0-100) | `70` |
| `UPDATE_PERCENT` | Percentage of update operations (0-100) | `30` |
| `TABLE_NAME` | Name of the test table | `load_test_data` |

**Note**: `INSERT_PERCENT + UPDATE_PERCENT` must equal 100.

### Running the Load Test

#### Option 1: Using Environment Variables

```bash
export DB_HOST=localhost
export DB_PORT=5432
export DB_USER=postgres
export DB_PASSWORD=yourpassword
export DB_NAME=testdb
export CONCURRENT_WRITERS=20
export TEST_RUN_DURATION=600
export BATCH_SIZE=200

go run main.go
```

#### Option 2: Inline Environment Variables

```bash
DB_HOST=localhost DB_USER=postgres DB_PASSWORD=yourpassword DB_NAME=testdb \
CONCURRENT_WRITERS=20 TEST_RUN_DURATION=300 BATCH_SIZE=100 \
INSERT_PERCENT=70 UPDATE_PERCENT=30 \
go run main.go
```

#### Option 3: Build and Run

```bash
go build -o load-client
./load-client
```

### Running in Kubernetes

Create a ConfigMap and Secret:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: postgres-credentials
type: Opaque
stringData:
  DB_PASSWORD: "yourpassword"
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: load-test-config
data:
  DB_HOST: "postgres-service"
  DB_PORT: "5432"
  DB_USER: "postgres"
  DB_NAME: "testdb"
  DB_SSL_MODE: "disable"
  CONCURRENT_WRITERS: "20"
  TEST_RUN_DURATION: "600"
  BATCH_SIZE: "200"
  INSERT_PERCENT: "70"
  UPDATE_PERCENT: "30"
```

Create a Job or Deployment:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: postgres-load-test
spec:
  template:
    spec:
      containers:
      - name: load-client
        image: your-registry/high-write-load-client:latest
        envFrom:
        - configMapRef:
            name: load-test-config
        - secretRef:
            name: postgres-credentials
      restartPolicy: Never
  backoffLimit: 1
```

## Output Example

```
=================================================================
PostgreSQL High Write Load Testing Client
=================================================================

Configuration:
  Database: postgres@localhost:5432/testdb
  Concurrent Writers: 20
  Test Duration: 5m0s
  Batch Size: 100 records
  Workload: 70% Inserts, 30% Updates
  Report Interval: 10s

Connection Manager initialized successfully
  Max connections in DB: 100
  Current active connections: 3
  Available connections: 97
  Client pool size: 50 (max open), 10 (max idle)

Initializing load generator...
Table already contains 10000 records
Load generator initialized successfully
Starting 20 concurrent workers...
All workers started successfully

Starting load test for 5m0s...
Press Ctrl+C to stop early

=================================================================
Test Duration: 10s
-----------------------------------------------------------------
Cumulative Statistics:
  Total Operations: 45230 (Inserts: 31661, Updates: 13569)
  Total Errors: 12
  Total Data Written: 22.65 MB
-----------------------------------------------------------------
Current Throughput (interval):
  Operations/sec: 4523.00 (Inserts: 3166.10/s, Updates: 1356.90/s)
  Throughput: 2.27 MB/s
  Errors/sec: 1.20
-----------------------------------------------------------------
Latency Statistics:
  Inserts - Avg: 2.156ms, P95: 8.234ms, P99: 15.432ms
  Updates - Avg: 1.234ms, P95: 4.567ms, P99: 9.876ms
-----------------------------------------------------------------
Connection Pool:
  Active: 21, Max: 100, Available: 79
=================================================================
```

## Performance Tuning

### For Maximum Throughput

```bash
export CONCURRENT_WRITERS=50
export BATCH_SIZE=500
export DB_MAX_OPEN_CONNS=100
export INSERT_PERCENT=100
export UPDATE_PERCENT=0
```

### For Mixed Workload Simulation

```bash
export CONCURRENT_WRITERS=30
export BATCH_SIZE=200
export INSERT_PERCENT=50
export UPDATE_PERCENT=50
```

### For High Connection Count Testing

```bash
export CONCURRENT_WRITERS=100
export BATCH_SIZE=50
export DB_MAX_OPEN_CONNS=200
```

## Database Preparation

The client automatically creates the test table and seeds initial data. No manual setup required!

The test table schema:

```sql
CREATE TABLE load_test_data (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL,
    age INT NOT NULL,
    address TEXT,
    phone_number VARCHAR(20),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    data TEXT
);

CREATE INDEX idx_load_test_data_email ON load_test_data(email);
CREATE INDEX idx_load_test_data_created_at ON load_test_data(created_at);
```

## Cleanup

By default, the test table is **NOT** dropped after the test completes, allowing you to inspect the data.

To enable automatic cleanup, uncomment these lines in `main.go`:

```go
cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cleanupCancel()
if err := lg.Cleanup(cleanupCtx); err != nil {
    fmt.Printf("Warning: Cleanup failed: %v\n", err)
}
```

Or manually drop the table:

```sql
DROP TABLE IF EXISTS load_test_data CASCADE;
```

## Metrics Explained

- **Total Operations**: Sum of all successful inserts and updates
- **Operations/sec**: Throughput during the last reporting interval
- **Throughput (MB/s)**: Data written per second (approximate)
- **Avg Latency**: Average operation duration
- **P95/P99 Latency**: 95th and 99th percentile latencies (worst-case scenarios)
- **Active Connections**: Current connections being used by the client
- **Available Connections**: Connections still available in the database

## Safety Features

1. **Connection Check**: Verifies sufficient connections are available before starting
2. **Min Free Connections**: Ensures at least N connections remain free for other clients
3. **Graceful Shutdown**: Handles interrupt signals cleanly
4. **Error Tracking**: Records and reports errors without stopping the test
5. **Connection Monitoring**: Continuous monitoring of connection pool health

## Troubleshooting

### "insufficient available connections"

- Increase `max_connections` in PostgreSQL
- Reduce `DB_MAX_OPEN_CONNS` or `CONCURRENT_WRITERS`
- Reduce `DB_MIN_FREE_CONNS` (not recommended in production)

### High error rate

- Check network latency
- Verify database performance (CPU, memory, disk I/O)
- Reduce `CONCURRENT_WRITERS` or `BATCH_SIZE`
- Check PostgreSQL logs for errors

### Low throughput

- Increase `CONCURRENT_WRITERS`
- Increase `BATCH_SIZE`
- Increase `DB_MAX_OPEN_CONNS`
- Optimize PostgreSQL settings (shared_buffers, work_mem, etc.)

## Future Enhancements

- [ ] Support for MySQL, MongoDB, and other databases
- [ ] Custom workload patterns (read operations, deletes)
- [ ] Prometheus metrics export
- [ ] Grafana dashboard templates
- [ ] Transaction testing
- [ ] Prepared statement support
- [ ] Connection pooling with pgbouncer
- [ ] Result export (CSV, JSON)

## License

Apache License 2.0

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
