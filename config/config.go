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

package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the load testing client
type Config struct {
	// Database connection settings
	DB DBConfig

	// Load test settings
	Load LoadConfig

	// Workload distribution
	Workload WorkloadConfig
}

// DBConfig contains database connection information
type DBConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string

	// Connection pool settings
	MaxOpenConns int
	MaxIdleConns int
	MinFreeConns int // Minimum connections that must remain free
}

// LoadConfig contains load testing parameters
type LoadConfig struct {
	ConcurrentWriters int           // Number of concurrent worker goroutines
	Duration          time.Duration // Test duration
	BatchSize         int           // Number of records per batch insert
	ReportInterval    time.Duration // How often to report metrics
}

// WorkloadConfig defines the workload distribution
type WorkloadConfig struct {
	ReadPercent   int    // Percentage of read/SELECT operations (0-100)
	InsertPercent int    // Percentage of insert operations (0-100)
	UpdatePercent int    // Percentage of update operations (0-100)
	TableName     string // Test table name

	// Read operation settings
	ReadBatchSize int // Number of records to fetch per read operation
}

// LoadFromEnv loads configuration from environment variables
func LoadFromEnv() (*Config, error) {
	cfg := &Config{}

	// Database configuration
	cfg.DB.Host = getEnv("DB_HOST", "localhost")
	cfg.DB.Port = getEnvAsInt("DB_PORT", 5432)
	cfg.DB.User = getEnv("DB_USER", "postgres")
	cfg.DB.Password = getEnv("DB_PASSWORD", "")
	cfg.DB.DBName = getEnv("DB_NAME", "testdb")
	cfg.DB.SSLMode = getEnv("DB_SSL_MODE", "disable")
	cfg.DB.MaxOpenConns = getEnvAsInt("DB_MAX_OPEN_CONNS", 50)
	cfg.DB.MaxIdleConns = getEnvAsInt("DB_MAX_IDLE_CONNS", 10)
	cfg.DB.MinFreeConns = getEnvAsInt("DB_MIN_FREE_CONNS", 5)

	// Load test configuration
	cfg.Load.ConcurrentWriters = getEnvAsInt("CONCURRENT_WRITERS", 10)
	durationSecs := getEnvAsInt("TEST_RUN_DURATION", 300) // Default 5 minutes
	cfg.Load.Duration = time.Duration(durationSecs) * time.Second
	cfg.Load.BatchSize = getEnvAsInt("BATCH_SIZE", 100)
	reportIntervalSecs := getEnvAsInt("REPORT_INTERVAL", 10) // Default 10 seconds
	cfg.Load.ReportInterval = time.Duration(reportIntervalSecs) * time.Second

	// Workload configuration
	cfg.Workload.ReadPercent = getEnvAsInt("READ_PERCENT", 0)
	cfg.Workload.InsertPercent = getEnvAsInt("INSERT_PERCENT", 70)
	cfg.Workload.UpdatePercent = getEnvAsInt("UPDATE_PERCENT", 30)
	cfg.Workload.TableName = getEnv("TABLE_NAME", "load_test_data")
	cfg.Workload.ReadBatchSize = getEnvAsInt("READ_BATCH_SIZE", 10)

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.DB.Host == "" {
		return fmt.Errorf("DB_HOST cannot be empty")
	}
	if c.DB.User == "" {
		return fmt.Errorf("DB_USER cannot be empty")
	}
	if c.DB.DBName == "" {
		return fmt.Errorf("DB_NAME cannot be empty")
	}
	if c.DB.MinFreeConns < 1 {
		return fmt.Errorf("DB_MIN_FREE_CONNS must be at least 1")
	}
	if c.Load.ConcurrentWriters < 1 {
		return fmt.Errorf("CONCURRENT_WRITERS must be at least 1")
	}
	if c.Load.Duration < time.Second {
		return fmt.Errorf("TEST_RUN_DURATION must be at least 1 second")
	}
	if c.Load.BatchSize < 1 {
		return fmt.Errorf("BATCH_SIZE must be at least 1")
	}

	// Validate workload percentages
	totalPercent := c.Workload.ReadPercent + c.Workload.InsertPercent + c.Workload.UpdatePercent
	if totalPercent != 100 {
		return fmt.Errorf("READ_PERCENT + INSERT_PERCENT + UPDATE_PERCENT must equal 100, got %d + %d + %d = %d",
			c.Workload.ReadPercent, c.Workload.InsertPercent, c.Workload.UpdatePercent, totalPercent)
	}

	if c.Workload.ReadPercent < 0 || c.Workload.ReadPercent > 100 {
		return fmt.Errorf("READ_PERCENT must be between 0 and 100, got %d", c.Workload.ReadPercent)
	}
	if c.Workload.InsertPercent < 0 || c.Workload.InsertPercent > 100 {
		return fmt.Errorf("INSERT_PERCENT must be between 0 and 100, got %d", c.Workload.InsertPercent)
	}
	if c.Workload.UpdatePercent < 0 || c.Workload.UpdatePercent > 100 {
		return fmt.Errorf("UPDATE_PERCENT must be between 0 and 100, got %d", c.Workload.UpdatePercent)
	}

	if c.Workload.ReadBatchSize < 1 {
		return fmt.Errorf("READ_BATCH_SIZE must be at least 1")
	}

	return nil
}

// GetConnectionString returns the PostgreSQL connection string
func (c *DBConfig) GetConnectionString() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s connect_timeout=30",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode,
	)
}

// Helper functions
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}
