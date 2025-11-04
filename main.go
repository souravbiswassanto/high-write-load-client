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

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/souravbiswassanto/high-write-load-client/clients/postgres"
	"github.com/souravbiswassanto/high-write-load-client/config"
	"github.com/souravbiswassanto/high-write-load-client/metrics"
)

func _main() {
	fmt.Println("=================================================================")
	fmt.Println("PostgreSQL High Write Load Testing Client")
	fmt.Println("=================================================================")

	// Load configuration from environment variables
	cfg, err := config.LoadFromEnv()
	if err != nil {
		fmt.Printf("Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\nConfiguration:")
	fmt.Printf("  Database: %s@%s:%d/%s\n", cfg.DB.User, cfg.DB.Host, cfg.DB.Port, cfg.DB.DBName)
	fmt.Printf("  Concurrent Writers: %d\n", cfg.Load.ConcurrentWriters)
	fmt.Printf("  Test Duration: %v\n", cfg.Load.Duration)
	fmt.Printf("  Batch Size: %d records\n", cfg.Load.BatchSize)
	fmt.Printf("  Workload: %d%% Inserts, %d%% Updates\n", cfg.Workload.InsertPercent, cfg.Workload.UpdatePercent)
	fmt.Printf("  Report Interval: %v\n", cfg.Load.ReportInterval)
	fmt.Println()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Load.Duration+30*time.Second)
	defer cancel()

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Initialize connection manager
	fmt.Println("Connecting to PostgreSQL...")
	cm, err := postgres.NewConnectionManager(&cfg.DB)
	if err != nil {
		fmt.Printf("Failed to create connection manager: %v\n", err)
		os.Exit(1)
	}
	defer cm.Close()

	// Initialize metrics
	m := metrics.New()

	// Initialize load generator
	lg := postgres.NewLoadGenerator(cm, cfg, m)
	if err := lg.Initialize(ctx); err != nil {
		fmt.Printf("Failed to initialize load generator: %v\n", err)
		os.Exit(1)
	}

	// Start connection monitoring in background
	monitorCtx, monitorCancel := context.WithCancel(ctx)
	defer monitorCancel()

	go cm.MonitorConnections(monitorCtx, 5*time.Second, func(stats *postgres.ConnectionStats) {
		m.UpdateConnectionMetrics(stats.CurrentConnections, stats.MaxConnections, stats.AvailableConnections)
	})

	// Start metrics reporting
	go func() {
		ticker := time.NewTicker(cfg.Load.ReportInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				snapshot := m.GetSnapshot()
				snapshot.Print()
			}
		}
	}()

	// Start load generation
	lg.Start(ctx)

	// Create a timer for test duration
	testTimer := time.NewTimer(cfg.Load.Duration)

	// Wait for either test duration to complete or interrupt signal
	fmt.Printf("\nStarting load test for %v...\n", cfg.Load.Duration)
	fmt.Println("Press Ctrl+C to stop early")
	fmt.Println()

	select {
	case <-testTimer.C:
		fmt.Println("\nTest duration completed")
	case <-sigChan:
		fmt.Println("\nReceived interrupt signal, stopping...")
	case <-ctx.Done():
		fmt.Println("\nContext cancelled, stopping...")
	}

	// Stop load generation
	lg.Stop()

	// Print final metrics
	fmt.Println("\nFinal Results:")
	finalSnapshot := m.GetSnapshot()
	finalSnapshot.Print()

	// Check for data loss before cleanup
	fmt.Println("\n=================================================================")
	fmt.Println("Checking for Data Loss...")
	fmt.Println("=================================================================")
	dataLossCtx, dataLossCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer dataLossCancel()

	totalInsertedIDs, lostRecords, err := lg.CheckDataLoss(dataLossCtx)
	if err != nil {
		fmt.Printf("Warning: Data loss check failed: %v\n", err)
	} else {
		dataLossPercent := 0.0
		if totalInsertedIDs > 0 {
			dataLossPercent = float64(lostRecords) * 100.0 / float64(totalInsertedIDs)
		}

		fmt.Println("\n=================================================================")
		fmt.Println("Data Loss Report:")
		fmt.Println("-----------------------------------------------------------------")
		fmt.Printf("  Total Records Inserted: %d\n", totalInsertedIDs)
		fmt.Printf("  Records Found in DB: %d\n", totalInsertedIDs-lostRecords)
		fmt.Printf("  Records Lost: %d\n", lostRecords)
		fmt.Printf("  Data Loss Percentage: %.2f%%\n", dataLossPercent)
		fmt.Println("=================================================================")

		if lostRecords > 0 {
			fmt.Printf("\n⚠️  WARNING: %d records were inserted but not found in database!\n", lostRecords)
			fmt.Println("This may indicate:")
			fmt.Println("  - Database crash/restart occurred during test")
			fmt.Println("  - pg_rewind was triggered due to network partition")
			fmt.Println("  - Transaction rollback due to replication issues")
		} else if totalInsertedIDs > 0 {
			fmt.Println("\n✅ No data loss detected - all inserted records are present in database")
		}
	}

	// Cleanup test data table after test completion
	fmt.Println("\nCleaning up test data...")
	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cleanupCancel()
	if err := lg.Cleanup(cleanupCtx); err != nil {
		fmt.Printf("Warning: Cleanup failed: %v\n", err)
	} else {
		fmt.Println("Test data table deleted successfully")
	}

	fmt.Println("\nTest completed successfully!")
}
