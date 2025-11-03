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
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver
	"github.com/souravbiswassanto/high-write-load-client/config"
)

// ConnectionManager manages PostgreSQL connections with safety checks
type ConnectionManager struct {
	db     *sql.DB
	config *config.DBConfig
}

// ConnectionStats represents the current connection state
type ConnectionStats struct {
	MaxConnections       int32
	CurrentConnections   int32
	AvailableConnections int32
	CanConnect           bool
}

// NewConnectionManager creates a new connection manager
func NewConnectionManager(cfg *config.DBConfig) (*ConnectionManager, error) {
	cm := &ConnectionManager{
		config: cfg,
	}

	// First, create a connection to check max_connections
	connStr := cfg.GetConnectionString()
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	cm.db = db

	// Check if we can safely connect
	stats, err := cm.GetConnectionStats(ctx)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to get connection stats: %w", err)
	}

	if !stats.CanConnect {
		db.Close()
		return nil, fmt.Errorf(
			"insufficient available connections: max=%d, current=%d, available=%d, required_free=%d",
			stats.MaxConnections,
			stats.CurrentConnections,
			stats.AvailableConnections,
			cfg.MinFreeConns,
		)
	}

	// Configure connection pool
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(time.Hour)
	db.SetConnMaxIdleTime(15 * time.Minute)

	fmt.Printf("Connection Manager initialized successfully\n")
	fmt.Printf("  Max connections in DB: %d\n", stats.MaxConnections)
	fmt.Printf("  Current active connections: %d\n", stats.CurrentConnections)
	fmt.Printf("  Available connections: %d\n", stats.AvailableConnections)
	fmt.Printf("  Client pool size: %d (max open), %d (max idle)\n", cfg.MaxOpenConns, cfg.MaxIdleConns)

	return cm, nil
}

// GetConnectionStats retrieves current connection statistics from PostgreSQL
func (cm *ConnectionManager) GetConnectionStats(ctx context.Context) (*ConnectionStats, error) {
	stats := &ConnectionStats{}

	// Get max_connections setting
	var maxConns int32
	err := cm.db.QueryRowContext(ctx, "SHOW max_connections").Scan(&maxConns)
	if err != nil {
		return nil, fmt.Errorf("failed to get max_connections: %w", err)
	}
	stats.MaxConnections = maxConns

	// Get current number of active connections
	var currentConns int32
	query := `SELECT count(*) FROM pg_stat_activity WHERE state != 'idle' OR state IS NULL`
	err = cm.db.QueryRowContext(ctx, query).Scan(&currentConns)
	if err != nil {
		return nil, fmt.Errorf("failed to get current connections: %w", err)
	}
	stats.CurrentConnections = currentConns

	// Calculate available connections
	stats.AvailableConnections = stats.MaxConnections - stats.CurrentConnections

	// Check if we have enough free connections
	stats.CanConnect = stats.AvailableConnections >= int32(cm.config.MinFreeConns)

	return stats, nil
}

// GetDB returns the underlying database connection
func (cm *ConnectionManager) GetDB() *sql.DB {
	return cm.db
}

// Close closes the database connection
func (cm *ConnectionManager) Close() error {
	if cm.db != nil {
		return cm.db.Close()
	}
	return nil
}

// GetDBStats returns database/sql connection pool stats
func (cm *ConnectionManager) GetDBStats() sql.DBStats {
	return cm.db.Stats()
}

// MonitorConnections periodically monitors connection stats
func (cm *ConnectionManager) MonitorConnections(ctx context.Context, interval time.Duration, callback func(*ConnectionStats)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			stats, err := cm.GetConnectionStats(ctx)
			if err != nil {
				fmt.Printf("Error getting connection stats: %v\n", err)
				continue
			}
			callback(stats)
		}
	}
}

// HealthCheck performs a health check on the connection
func (cm *ConnectionManager) HealthCheck(ctx context.Context) error {
	// Simple ping
	if err := cm.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	// Check connection stats
	stats, err := cm.GetConnectionStats(ctx)
	if err != nil {
		return fmt.Errorf("failed to get connection stats: %w", err)
	}

	if !stats.CanConnect {
		return fmt.Errorf(
			"insufficient connections available: max=%d, current=%d, available=%d",
			stats.MaxConnections,
			stats.CurrentConnections,
			stats.AvailableConnections,
		)
	}

	return nil
}
