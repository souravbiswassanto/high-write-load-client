/*
Copyright AppsCode Inc. and Contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package postgres

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/souravbiswassanto/high-write-load-client/config"
	"github.com/souravbiswassanto/high-write-load-client/metrics"
)

// LoadGeneratorV2 generates mixed read/write load on PostgreSQL
type LoadGeneratorV2 struct {
	cm        *ConnectionManager
	config    *config.Config
	metrics   *metrics.MetricsV2
	wg        sync.WaitGroup
	stopChan  chan struct{}
	stopOnce  sync.Once
	tableName string
	totalRows atomic.Int64 // Track approximate number of rows for efficient reads
}

// NewLoadGeneratorV2 creates a new enhanced load generator with read support
func NewLoadGeneratorV2(cm *ConnectionManager, cfg *config.Config, m *metrics.MetricsV2) *LoadGeneratorV2 {
	lg := &LoadGeneratorV2{
		cm:        cm,
		config:    cfg,
		metrics:   m,
		stopChan:  make(chan struct{}),
		tableName: cfg.Workload.TableName,
	}
	return lg
}

// Initialize sets up the test table and prepares the database
func (lg *LoadGeneratorV2) Initialize(ctx context.Context) error {
	fmt.Println("Initializing enhanced load generator with read support...")

	// Create table if it doesn't exist
	createTableSQL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id BIGSERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			email VARCHAR(255) NOT NULL,
			age INT NOT NULL,
			address TEXT,
			phone_number VARCHAR(20),
			created_at TIMESTAMP NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
			data TEXT,
			status VARCHAR(50) DEFAULT 'active',
			score INT DEFAULT 0
		)
	`, lg.tableName)

	_, err := lg.cm.GetDB().ExecContext(ctx, createTableSQL)
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	// Create indices for better read and update performance
	createIndexSQL := fmt.Sprintf(`
		CREATE INDEX IF NOT EXISTS idx_%s_email ON %s(email);
		CREATE INDEX IF NOT EXISTS idx_%s_created_at ON %s(created_at);
		CREATE INDEX IF NOT EXISTS idx_%s_status_score ON %s(status, score);
		CREATE INDEX IF NOT EXISTS idx_%s_name ON %s(name);
	`, lg.tableName, lg.tableName,
		lg.tableName, lg.tableName,
		lg.tableName, lg.tableName,
		lg.tableName, lg.tableName)

	_, err = lg.cm.GetDB().ExecContext(ctx, createIndexSQL)
	if err != nil {
		return fmt.Errorf("failed to create indices: %w", err)
	}

	// Check if table has data, if not, seed it
	var count int64
	err = lg.cm.GetDB().QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", lg.tableName)).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to count records: %w", err)
	}

	if count == 0 {
		fmt.Println("Seeding table with initial data...")
		if err := lg.seedInitialData(ctx, 50000); err != nil { // Seed more data for reads
			return fmt.Errorf("failed to seed initial data: %w", err)
		}
		count = 50000
		fmt.Printf("Seeded %d initial records\n", count)
	} else {
		fmt.Printf("Table already contains %d records\n", count)
	}

	lg.totalRows.Store(count)

	fmt.Println("Enhanced load generator initialized successfully")
	return nil
}

// seedInitialData inserts initial records
func (lg *LoadGeneratorV2) seedInitialData(ctx context.Context, count int) error {
	batchSize := 1000
	for i := 0; i < count; i += batchSize {
		remaining := count - i
		if remaining > batchSize {
			remaining = batchSize
		}

		records := make([]TestRecord, remaining)
		for j := 0; j < remaining; j++ {
			records[j] = lg.generateRecord()
		}

		if err := lg.batchInsert(ctx, records); err != nil {
			return err
		}
	}
	return nil
}

// Start starts the load generation with multiple workers
func (lg *LoadGeneratorV2) Start(ctx context.Context) {
	fmt.Printf("Starting %d concurrent workers with mixed read/write workload...\n", lg.config.Load.ConcurrentWriters)
	fmt.Printf("  Workload: %d%% Reads, %d%% Inserts, %d%% Updates\n",
		lg.config.Workload.ReadPercent,
		lg.config.Workload.InsertPercent,
		lg.config.Workload.UpdatePercent)

	for i := 0; i < lg.config.Load.ConcurrentWriters; i++ {
		lg.wg.Add(1)
		go lg.worker(ctx, i)
	}

	fmt.Println("All workers started successfully")
}

// worker is the main worker goroutine that performs mixed operations
func (lg *LoadGeneratorV2) worker(ctx context.Context, workerID int) {
	defer lg.wg.Done()

	// Random number generator for this worker
	rng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(workerID)))

	for {
		select {
		case <-ctx.Done():
			return
		case <-lg.stopChan:
			return
		default:
			// Decide operation type based on workload configuration
			roll := rng.Intn(100)

			if roll < lg.config.Workload.ReadPercent {
				// Perform read
				lg.performRead(ctx, rng)
			} else if roll < lg.config.Workload.ReadPercent+lg.config.Workload.InsertPercent {
				// Perform insert
				lg.performInsert(ctx, rng)
			} else {
				// Perform update
				lg.performUpdate(ctx, rng)
			}
		}
	}
}

// performRead executes a read/SELECT operation
func (lg *LoadGeneratorV2) performRead(ctx context.Context, rng *rand.Rand) {
	start := time.Now()

	// Various read patterns to simulate real-world scenarios
	readPattern := rng.Intn(4)
	var err error
	var bytesRead int64

	switch readPattern {
	case 0:
		// Read by ID range
		bytesRead, err = lg.readByIDRange(ctx, rng)
	case 1:
		// Read by status
		bytesRead, err = lg.readByStatus(ctx, rng)
	case 2:
		// Read recent records
		bytesRead, err = lg.readRecentRecords(ctx)
	case 3:
		// Read by name pattern
		bytesRead, err = lg.readByNamePattern(ctx, rng)
	}

	latency := time.Since(start)

	if err != nil {
		lg.metrics.RecordError()
		return
	}

	lg.metrics.RecordRead(latency, bytesRead)
}

// readByIDRange reads records within an ID range
func (lg *LoadGeneratorV2) readByIDRange(ctx context.Context, rng *rand.Rand) (int64, error) {
	totalRows := lg.totalRows.Load()
	if totalRows == 0 {
		return 0, fmt.Errorf("no rows to read")
	}

	startID := rng.Int63n(totalRows)
	limit := lg.config.Workload.ReadBatchSize

	query := fmt.Sprintf(`
		SELECT id, name, email, age, address, phone_number, status, score, data
		FROM %s
		WHERE id >= $1
		LIMIT $2
	`, lg.tableName)

	rows, err := lg.cm.GetDB().QueryContext(ctx, query, startID, limit)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var bytesRead int64
	for rows.Next() {
		var r TestRecord
		err := rows.Scan(&r.ID, &r.Name, &r.Email, &r.Age, &r.Address, &r.PhoneNumber, &r.Status, &r.Score, &r.Data)
		if err != nil {
			return bytesRead, err
		}
		bytesRead += int64(len(r.Data) + len(r.Name) + len(r.Email) + len(r.Address))
	}

	return bytesRead, rows.Err()
}

// readByStatus reads records with a specific status
func (lg *LoadGeneratorV2) readByStatus(ctx context.Context, rng *rand.Rand) (int64, error) {
	statuses := []string{"active", "inactive", "pending"}
	status := statuses[rng.Intn(len(statuses))]

	query := fmt.Sprintf(`
		SELECT id, name, email, score, data
		FROM %s
		WHERE status = $1
		ORDER BY score DESC
		LIMIT $2
	`, lg.tableName)

	rows, err := lg.cm.GetDB().QueryContext(ctx, query, status, lg.config.Workload.ReadBatchSize)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var bytesRead int64
	for rows.Next() {
		var id int64
		var name, email, data string
		var score int
		err := rows.Scan(&id, &name, &email, &score, &data)
		if err != nil {
			return bytesRead, err
		}
		bytesRead += int64(len(name) + len(email) + len(data))
	}

	return bytesRead, rows.Err()
}

// readRecentRecords reads the most recently created records
func (lg *LoadGeneratorV2) readRecentRecords(ctx context.Context) (int64, error) {
	query := fmt.Sprintf(`
		SELECT id, name, email, created_at, data
		FROM %s
		ORDER BY created_at DESC
		LIMIT $1
	`, lg.tableName)

	rows, err := lg.cm.GetDB().QueryContext(ctx, query, lg.config.Workload.ReadBatchSize)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var bytesRead int64
	for rows.Next() {
		var id int64
		var name, email, data string
		var createdAt time.Time
		err := rows.Scan(&id, &name, &email, &createdAt, &data)
		if err != nil {
			return bytesRead, err
		}
		bytesRead += int64(len(name) + len(email) + len(data))
	}

	return bytesRead, rows.Err()
}

// readByNamePattern reads records matching a name pattern
func (lg *LoadGeneratorV2) readByNamePattern(ctx context.Context, rng *rand.Rand) (int64, error) {
	firstNames := []string{"John", "Jane", "Michael", "Emily", "David", "Sarah", "Robert", "Lisa", "William", "Jennifer"}
	pattern := firstNames[rng.Intn(len(firstNames))] + "%"

	query := fmt.Sprintf(`
		SELECT id, name, email, data
		FROM %s
		WHERE name LIKE $1
		LIMIT $2
	`, lg.tableName)

	rows, err := lg.cm.GetDB().QueryContext(ctx, query, pattern, lg.config.Workload.ReadBatchSize)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var bytesRead int64
	for rows.Next() {
		var id int64
		var name, email, data string
		err := rows.Scan(&id, &name, &email, &data)
		if err != nil {
			return bytesRead, err
		}
		bytesRead += int64(len(name) + len(email) + len(data))
	}

	return bytesRead, rows.Err()
}

// performInsert executes a batch insert operation
func (lg *LoadGeneratorV2) performInsert(ctx context.Context, rng *rand.Rand) {
	start := time.Now()

	// Generate batch of records
	records := make([]TestRecord, lg.config.Load.BatchSize)
	for i := 0; i < lg.config.Load.BatchSize; i++ {
		records[i] = lg.generateRecord()
	}

	// Calculate approximate size
	bytesWritten := int64(len(records) * 600) // Rough estimate with new fields

	// Execute batch insert
	err := lg.batchInsert(ctx, records)
	latency := time.Since(start)

	if err != nil {
		lg.metrics.RecordError()
		return
	}

	// Update row count
	lg.totalRows.Add(int64(len(records)))
	lg.metrics.RecordInsert(latency, bytesWritten)
}

// batchInsert performs a batch insert using a single SQL statement
func (lg *LoadGeneratorV2) batchInsert(ctx context.Context, records []TestRecord) error {
	if len(records) == 0 {
		return nil
	}

	// Build bulk insert query
	valueStrings := make([]string, 0, len(records))
	valueArgs := make([]interface{}, 0, len(records)*9)

	for i, record := range records {
		valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
			i*9+1, i*9+2, i*9+3, i*9+4, i*9+5, i*9+6, i*9+7, i*9+8, i*9+9))

		valueArgs = append(valueArgs,
			record.Name,
			record.Email,
			record.Age,
			record.Address,
			record.PhoneNumber,
			record.CreatedAt,
			record.Data,
			record.Status,
			record.Score,
		)
	}

	query := fmt.Sprintf(`
		INSERT INTO %s (name, email, age, address, phone_number, created_at, data, status, score)
		VALUES %s
	`, lg.tableName, strings.Join(valueStrings, ","))

	_, err := lg.cm.GetDB().ExecContext(ctx, query, valueArgs...)
	return err
}

// performUpdate executes an update operation
func (lg *LoadGeneratorV2) performUpdate(ctx context.Context, rng *rand.Rand) {
	start := time.Now()

	totalRows := lg.totalRows.Load()
	if totalRows == 0 {
		return
	}

	// Get a random record ID to update
	randomID := rng.Int63n(totalRows) + 1

	// Update the record
	updateQuery := fmt.Sprintf(`
		UPDATE %s
		SET name = $1,
		    age = $2,
		    address = $3,
		    updated_at = $4,
		    score = $5,
		    data = $6
		WHERE id = $7
	`, lg.tableName)

	record := lg.generateRecord()
	_, err := lg.cm.GetDB().ExecContext(ctx, updateQuery,
		record.Name,
		record.Age,
		record.Address,
		time.Now(),
		record.Score,
		record.Data,
		randomID,
	)

	latency := time.Since(start)

	if err != nil {
		lg.metrics.RecordError()
		return
	}

	bytesWritten := int64(500) // Rough estimate for update
	lg.metrics.RecordUpdate(latency, bytesWritten)
}

// generateRecord creates a random test record
func (lg *LoadGeneratorV2) generateRecord() TestRecord {
	statuses := []string{"active", "inactive", "pending"}
	return TestRecord{
		Name:        generateRandomName(),
		Email:       generateRandomEmail(),
		Age:         rand.Intn(80) + 18,
		Address:     generateRandomAddress(),
		PhoneNumber: generateRandomPhone(),
		CreatedAt:   time.Now(),
		Data:        generateRandomData(1024), // 1KB of random data
		Status:      statuses[rand.Intn(len(statuses))],
		Score:       rand.Intn(1000),
	}
}

// Stop gracefully stops the load generator
func (lg *LoadGeneratorV2) Stop() {
	lg.stopOnce.Do(func() {
		close(lg.stopChan)
		lg.wg.Wait()
		fmt.Println("All workers stopped")
	})
}

// Cleanup removes the test table
func (lg *LoadGeneratorV2) Cleanup(ctx context.Context) error {
	fmt.Println("Cleaning up test table...")
	_, err := lg.cm.GetDB().ExecContext(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", lg.tableName))
	if err != nil {
		return fmt.Errorf("failed to drop table: %w", err)
	}
	fmt.Println("Cleanup completed")
	return nil
}
