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
	"database/sql"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/souravbiswassanto/high-write-load-client/config"
	"github.com/souravbiswassanto/high-write-load-client/metrics"
)

// LoadGenerator generates write load on PostgreSQL
type LoadGenerator struct {
	cm        *ConnectionManager
	config    *config.Config
	metrics   *metrics.Metrics
	wg        sync.WaitGroup
	stopChan  chan struct{}
	stopOnce  sync.Once
	tableName string
}

// TestRecord represents a sample record for load testing
type TestRecord struct {
	ID          int64
	Name        string
	Email       string
	Age         int
	Address     string
	PhoneNumber string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Data        string // Large text field for data volume
	Status      string // Status field for filtering
	Score       int    // Score field for sorting/filtering
}

// NewLoadGenerator creates a new load generator
func NewLoadGenerator(cm *ConnectionManager, cfg *config.Config, m *metrics.Metrics) *LoadGenerator {
	return &LoadGenerator{
		cm:        cm,
		config:    cfg,
		metrics:   m,
		stopChan:  make(chan struct{}),
		tableName: cfg.Workload.TableName,
	}
}

// Initialize sets up the test table and prepares the database
func (lg *LoadGenerator) Initialize(ctx context.Context) error {
	fmt.Println("Initializing load generator...")

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
			data TEXT
		)
	`, lg.tableName)

	_, err := lg.cm.GetDB().ExecContext(ctx, createTableSQL)
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	// Create indices for better update performance
	createIndexSQL := fmt.Sprintf(`
		CREATE INDEX IF NOT EXISTS idx_%s_email ON %s(email);
		CREATE INDEX IF NOT EXISTS idx_%s_created_at ON %s(created_at);
	`, lg.tableName, lg.tableName, lg.tableName, lg.tableName)

	_, err = lg.cm.GetDB().ExecContext(ctx, createIndexSQL)
	if err != nil {
		return fmt.Errorf("failed to create indices: %w", err)
	}

	// Check if table has data, if not, seed it with initial records for updates
	var count int64
	err = lg.cm.GetDB().QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", lg.tableName)).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to count records: %w", err)
	}

	if count == 0 {
		fmt.Println("Seeding table with initial data for update operations...")
		if err := lg.seedInitialData(ctx, 10000); err != nil {
			return fmt.Errorf("failed to seed initial data: %w", err)
		}
		fmt.Printf("Seeded %d initial records\n", 10000)
	} else {
		fmt.Printf("Table already contains %d records\n", count)
	}

	fmt.Println("Load generator initialized successfully")
	return nil
}

// seedInitialData inserts initial records for update operations
func (lg *LoadGenerator) seedInitialData(ctx context.Context, count int) error {
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
func (lg *LoadGenerator) Start(ctx context.Context) {
	fmt.Printf("Starting %d concurrent workers...\n", lg.config.Load.ConcurrentWriters)

	for i := 0; i < lg.config.Load.ConcurrentWriters; i++ {
		lg.wg.Add(1)
		go lg.worker(ctx, i)
	}

	fmt.Println("All workers started successfully")
}

// worker is the main worker goroutine that performs writes
func (lg *LoadGenerator) worker(ctx context.Context, workerID int) {
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
			if roll < lg.config.Workload.InsertPercent {
				// Perform insert
				lg.performInsert(ctx, rng)
			} else {
				// Perform update
				lg.performUpdate(ctx, rng)
			}
		}
	}
}

// performInsert executes a batch insert operation
func (lg *LoadGenerator) performInsert(ctx context.Context, rng *rand.Rand) {
	start := time.Now()

	// Generate batch of records
	records := make([]TestRecord, lg.config.Load.BatchSize)
	for i := 0; i < lg.config.Load.BatchSize; i++ {
		records[i] = lg.generateRecord()
	}

	// Calculate approximate size
	bytesWritten := int64(len(records) * 500) // Rough estimate

	// Execute batch insert
	err := lg.batchInsert(ctx, records)
	latency := time.Since(start)

	if err != nil {
		lg.metrics.RecordError()
		// Don't spam logs with errors, just record them
		return
	}

	lg.metrics.RecordInsert(latency, bytesWritten)
}

// batchInsert performs a batch insert using a single SQL statement and records inserted IDs
func (lg *LoadGenerator) batchInsert(ctx context.Context, records []TestRecord) error {
	if len(records) == 0 {
		return nil
	}

	// Build bulk insert query
	valueStrings := make([]string, 0, len(records))
	valueArgs := make([]interface{}, 0, len(records)*7)

	for i, record := range records {
		valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d)",
			i*7+1, i*7+2, i*7+3, i*7+4, i*7+5, i*7+6, i*7+7))

		valueArgs = append(valueArgs,
			record.Name,
			record.Email,
			record.Age,
			record.Address,
			record.PhoneNumber,
			record.CreatedAt,
			record.Data,
		)
	}

	// Use RETURNING id to get inserted IDs for data loss tracking
	query := fmt.Sprintf(`
		INSERT INTO %s (name, email, age, address, phone_number, created_at, data)
		VALUES %s
		RETURNING id
	`, lg.tableName, strings.Join(valueStrings, ","))

	rows, err := lg.cm.GetDB().QueryContext(ctx, query, valueArgs...)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Record all inserted IDs for data loss tracking
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return err
		}
		lg.metrics.RecordInsertedID(id)
	}

	return rows.Err()
}

// performUpdate executes an update operation
func (lg *LoadGenerator) performUpdate(ctx context.Context, rng *rand.Rand) {
	start := time.Now()

	// Get a random record ID to update
	var id int64
	query := fmt.Sprintf("SELECT id FROM %s ORDER BY RANDOM() LIMIT 1", lg.tableName)
	err := lg.cm.GetDB().QueryRowContext(ctx, query).Scan(&id)
	if err != nil {
		if err != sql.ErrNoRows {
			lg.metrics.RecordError()
		}
		return
	}

	// Update the record
	updateQuery := fmt.Sprintf(`
		UPDATE %s
		SET name = $1,
		    age = $2,
		    address = $3,
		    updated_at = $4,
		    data = $5
		WHERE id = $6
	`, lg.tableName)

	record := lg.generateRecord()
	_, err = lg.cm.GetDB().ExecContext(ctx, updateQuery,
		record.Name,
		record.Age,
		record.Address,
		time.Now(),
		record.Data,
		id,
	)

	latency := time.Since(start)

	if err != nil {
		lg.metrics.RecordError()
		return
	}

	bytesWritten := int64(400) // Rough estimate for update
	lg.metrics.RecordUpdate(latency, bytesWritten)
}

// generateRecord creates a random test record
func (lg *LoadGenerator) generateRecord() TestRecord {
	return TestRecord{
		Name:        generateRandomName(),
		Email:       generateRandomEmail(),
		Age:         rand.Intn(80) + 18,
		Address:     generateRandomAddress(),
		PhoneNumber: generateRandomPhone(),
		CreatedAt:   time.Now(),
		Data:        generateRandomData(1024), // 1KB of random data
	}
}

// Stop gracefully stops the load generator
func (lg *LoadGenerator) Stop() {
	lg.stopOnce.Do(func() {
		close(lg.stopChan)
		lg.wg.Wait()
		fmt.Println("All workers stopped")
	})
}

// CheckDataLoss verifies how many inserted records are actually in the database
func (lg *LoadGenerator) CheckDataLoss(ctx context.Context) (int64, int64, error) {
	// Get all IDs that were supposedly inserted
	insertedIDs := lg.metrics.GetInsertedIDs()
	if len(insertedIDs) == 0 {
		return 0, 0, nil
	}

	totalInserted := int64(len(insertedIDs))
	fmt.Printf("Checking data loss for %d inserted records...\n", totalInserted)

	// For very large datasets (>100K records), use a more efficient approach:
	// Instead of checking each ID, use a range query with NOT IN for missing IDs
	if len(insertedIDs) > 100000 {
		fmt.Println("  Using optimized range-based verification for large dataset...")
		return lg.checkDataLossOptimized(ctx, insertedIDs)
	}

	// For smaller datasets, use the batch IN query approach
	// Use larger batches for better performance (10000 IDs per query)
	batchSize := 10000
	found := int64(0)
	totalBatches := (len(insertedIDs) + batchSize - 1) / batchSize

	for i := 0; i < len(insertedIDs); i += batchSize {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return 0, 0, fmt.Errorf("data loss check cancelled: %w", ctx.Err())
		default:
		}

		end := i + batchSize
		if end > len(insertedIDs) {
			end = len(insertedIDs)
		}

		batch := insertedIDs[i:end]
		currentBatch := (i / batchSize) + 1

		// Show progress every 5 batches or on last batch
		if currentBatch%5 == 0 || currentBatch == totalBatches {
			fmt.Printf("  Progress: Checked %d/%d batches (%.1f%%)\n",
				currentBatch, totalBatches, float64(currentBatch)*100/float64(totalBatches))
		}

		// Build the IN clause
		var idStrs []string
		for _, id := range batch {
			idStrs = append(idStrs, fmt.Sprintf("%d", id))
		}

		query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE id IN (%s)",
			lg.tableName, strings.Join(idStrs, ","))

		var count int64
		err := lg.cm.GetDB().QueryRowContext(ctx, query).Scan(&count)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to check data loss (batch %d/%d): %w", currentBatch, totalBatches, err)
		}

		found += count
	}

	lost := totalInserted - found
	fmt.Printf("Data loss check complete: %d found, %d lost out of %d inserted\n",
		found, lost, totalInserted)

	return totalInserted, lost, nil
}

// checkDataLossOptimized uses a range-based approach for very large datasets
func (lg *LoadGenerator) checkDataLossOptimized(ctx context.Context, insertedIDs []int64) (int64, int64, error) {
	// Find min and max IDs
	if len(insertedIDs) == 0 {
		return 0, 0, nil
	}

	minID := insertedIDs[0]
	maxID := insertedIDs[0]
	for _, id := range insertedIDs {
		if id < minID {
			minID = id
		}
		if id > maxID {
			maxID = id
		}
	}

	fmt.Printf("  ID range: %d to %d\n", minID, maxID)

	// Check context cancellation
	select {
	case <-ctx.Done():
		return 0, 0, fmt.Errorf("data loss check cancelled: %w", ctx.Err())
	default:
	}

	// Count how many records exist in this range
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE id >= %d AND id <= %d", lg.tableName, minID, maxID)
	var foundInRange int64
	err := lg.cm.GetDB().QueryRowContext(ctx, query).Scan(&foundInRange)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to count records in range: %w", err)
	}

	fmt.Printf("  Records in range [%d-%d]: %d\n", minID, maxID, foundInRange)

	totalInserted := int64(len(insertedIDs))

	// If all expected IDs are found, we're done
	if foundInRange >= totalInserted {
		fmt.Printf("Data loss check complete: %d found, 0 lost out of %d inserted\n",
			totalInserted, totalInserted)
		return totalInserted, 0, nil
	}

	// Otherwise, estimate data loss
	// This is an approximation - some IDs in range might not be ours
	lost := totalInserted - foundInRange
	if lost < 0 {
		lost = 0
	}

	fmt.Printf("Data loss check complete (estimated): %d found, ~%d lost out of %d inserted\n",
		foundInRange, lost, totalInserted)
	fmt.Println("  Note: Using range-based estimation for large dataset")

	return totalInserted, lost, nil
}

// Cleanup removes the test table
func (lg *LoadGenerator) Cleanup(ctx context.Context) error {
	fmt.Println("Cleaning up test table...")
	_, err := lg.cm.GetDB().ExecContext(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", lg.tableName))
	if err != nil {
		return fmt.Errorf("failed to drop table: %w", err)
	}
	fmt.Println("Cleanup completed")
	return nil
}

// Helper functions to generate random data
func generateRandomName() string {
	firstNames := []string{"John", "Jane", "Michael", "Emily", "David", "Sarah", "Robert", "Lisa", "William", "Jennifer"}
	lastNames := []string{"Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller", "Davis", "Rodriguez", "Martinez"}
	return fmt.Sprintf("%s %s", firstNames[rand.Intn(len(firstNames))], lastNames[rand.Intn(len(lastNames))])
}

func generateRandomEmail() string {
	domains := []string{"example.com", "test.com", "email.com", "mail.com"}
	return fmt.Sprintf("user%d@%s", rand.Intn(1000000), domains[rand.Intn(len(domains))])
}

func generateRandomAddress() string {
	streets := []string{"Main St", "Oak Ave", "Maple Dr", "Cedar Ln", "Pine Rd"}
	cities := []string{"Springfield", "Riverside", "Madison", "Georgetown", "Franklin"}
	return fmt.Sprintf("%d %s, %s, %s %05d",
		rand.Intn(9999)+1,
		streets[rand.Intn(len(streets))],
		cities[rand.Intn(len(cities))],
		"CA",
		rand.Intn(99999),
	)
}

func generateRandomPhone() string {
	return fmt.Sprintf("+1-%03d-%03d-%04d",
		rand.Intn(900)+100,
		rand.Intn(900)+100,
		rand.Intn(9000)+1000,
	)
}

func generateRandomData(size int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 "
	b := make([]byte, size)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}
