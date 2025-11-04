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

package metrics

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Metrics tracks all performance metrics for the load test
type Metrics struct {
	// Counters
	totalInserts atomic.Int64
	totalUpdates atomic.Int64
	totalErrors  atomic.Int64
	totalBytes   atomic.Int64

	// Data loss tracking
	insertedIDs      sync.Map // map[int64]bool - tracks all inserted IDs
	totalInsertedIDs atomic.Int64

	// Latency tracking
	insertLatencies []time.Duration
	updateLatencies []time.Duration
	latencyMutex    sync.RWMutex

	// Connection metrics
	activeConns    atomic.Int32
	maxConns       atomic.Int32
	availableConns atomic.Int32

	// Timing
	startTime      time.Time
	lastReportTime time.Time

	// For rate calculation
	lastInsertCount int64
	lastUpdateCount int64
	lastErrorCount  int64
	lastBytesCount  int64
}

// MetricsSnapshot represents metrics at a point in time
type MetricsSnapshot struct {
	Duration        time.Duration
	TotalInserts    int64
	TotalUpdates    int64
	TotalOperations int64
	TotalErrors     int64
	TotalBytes      int64

	// Data loss tracking
	TotalInsertedIDs int64
	LostRecords      int64
	DataLossPercent  float64

	InsertsPerSec float64
	UpdatesPerSec float64
	OpsPerSec     float64
	ErrorsPerSec  float64
	BytesPerSec   float64

	AvgInsertLatency time.Duration
	P95InsertLatency time.Duration
	P99InsertLatency time.Duration

	AvgUpdateLatency time.Duration
	P95UpdateLatency time.Duration
	P99UpdateLatency time.Duration

	ActiveConns    int32
	MaxConns       int32
	AvailableConns int32
}

// New creates a new Metrics instance
func New() *Metrics {
	return &Metrics{
		startTime:       time.Now(),
		lastReportTime:  time.Now(),
		insertLatencies: make([]time.Duration, 0, 10000),
		updateLatencies: make([]time.Duration, 0, 10000),
	}
}

// RecordInsert records a successful insert operation
func (m *Metrics) RecordInsert(latency time.Duration, bytesWritten int64) {
	m.totalInserts.Add(1)
	m.totalBytes.Add(bytesWritten)

	m.latencyMutex.Lock()
	m.insertLatencies = append(m.insertLatencies, latency)
	// Keep only last 10000 samples to prevent memory issues
	if len(m.insertLatencies) > 10000 {
		m.insertLatencies = m.insertLatencies[len(m.insertLatencies)-10000:]
	}
	m.latencyMutex.Unlock()
}

// RecordInsertedID records an inserted record ID for data loss tracking
func (m *Metrics) RecordInsertedID(id int64) {
	m.insertedIDs.Store(id, true)
	m.totalInsertedIDs.Add(1)
}

// GetInsertedIDs returns all inserted IDs as a slice
func (m *Metrics) GetInsertedIDs() []int64 {
	ids := make([]int64, 0)
	m.insertedIDs.Range(func(key, value interface{}) bool {
		if id, ok := key.(int64); ok {
			ids = append(ids, id)
		}
		return true
	})
	return ids
}

// RecordUpdate records a successful update operation
func (m *Metrics) RecordUpdate(latency time.Duration, bytesWritten int64) {
	m.totalUpdates.Add(1)
	m.totalBytes.Add(bytesWritten)

	m.latencyMutex.Lock()
	m.updateLatencies = append(m.updateLatencies, latency)
	// Keep only last 10000 samples
	if len(m.updateLatencies) > 10000 {
		m.updateLatencies = m.updateLatencies[len(m.updateLatencies)-10000:]
	}
	m.latencyMutex.Unlock()
}

// RecordError records an error
func (m *Metrics) RecordError() {
	m.totalErrors.Add(1)
}

// UpdateConnectionMetrics updates connection-related metrics
func (m *Metrics) UpdateConnectionMetrics(active, max, available int32) {
	m.activeConns.Store(active)
	m.maxConns.Store(max)
	m.availableConns.Store(available)
}

// GetSnapshot returns a snapshot of current metrics
func (m *Metrics) GetSnapshot() MetricsSnapshot {
	now := time.Now()
	duration := now.Sub(m.startTime)
	intervalDuration := now.Sub(m.lastReportTime)

	snapshot := MetricsSnapshot{
		Duration:       duration,
		TotalInserts:   m.totalInserts.Load(),
		TotalUpdates:   m.totalUpdates.Load(),
		TotalErrors:    m.totalErrors.Load(),
		TotalBytes:     m.totalBytes.Load(),
		ActiveConns:    m.activeConns.Load(),
		MaxConns:       m.maxConns.Load(),
		AvailableConns: m.availableConns.Load(),
	}

	snapshot.TotalOperations = snapshot.TotalInserts + snapshot.TotalUpdates

	// Calculate rates based on interval
	if intervalDuration.Seconds() > 0 {
		insertsDiff := snapshot.TotalInserts - m.lastInsertCount
		updatesDiff := snapshot.TotalUpdates - m.lastUpdateCount
		errorsDiff := snapshot.TotalErrors - m.lastErrorCount
		bytesDiff := snapshot.TotalBytes - m.lastBytesCount

		snapshot.InsertsPerSec = float64(insertsDiff) / intervalDuration.Seconds()
		snapshot.UpdatesPerSec = float64(updatesDiff) / intervalDuration.Seconds()
		snapshot.OpsPerSec = float64(insertsDiff+updatesDiff) / intervalDuration.Seconds()
		snapshot.ErrorsPerSec = float64(errorsDiff) / intervalDuration.Seconds()
		snapshot.BytesPerSec = float64(bytesDiff) / intervalDuration.Seconds()
	}

	// Calculate latency percentiles
	m.latencyMutex.RLock()
	if len(m.insertLatencies) > 0 {
		snapshot.AvgInsertLatency = calculateAvg(m.insertLatencies)
		snapshot.P95InsertLatency = calculatePercentile(m.insertLatencies, 95)
		snapshot.P99InsertLatency = calculatePercentile(m.insertLatencies, 99)
	}
	if len(m.updateLatencies) > 0 {
		snapshot.AvgUpdateLatency = calculateAvg(m.updateLatencies)
		snapshot.P95UpdateLatency = calculatePercentile(m.updateLatencies, 95)
		snapshot.P99UpdateLatency = calculatePercentile(m.updateLatencies, 99)
	}
	m.latencyMutex.RUnlock()

	// Update last counts for rate calculation
	m.lastInsertCount = snapshot.TotalInserts
	m.lastUpdateCount = snapshot.TotalUpdates
	m.lastErrorCount = snapshot.TotalErrors
	m.lastBytesCount = snapshot.TotalBytes
	m.lastReportTime = now

	return snapshot
}

// Print prints the metrics snapshot in a readable format
func (s *MetricsSnapshot) Print() {
	fmt.Println("=================================================================")
	fmt.Printf("Test Duration: %v\n", s.Duration.Round(time.Second))
	fmt.Println("-----------------------------------------------------------------")
	fmt.Println("Cumulative Statistics:")
	fmt.Printf("  Total Operations: %d (Inserts: %d, Updates: %d)\n",
		s.TotalOperations, s.TotalInserts, s.TotalUpdates)
	fmt.Printf("  Total Errors: %d\n", s.TotalErrors)
	fmt.Printf("  Total Data Written: %.2f MB\n", float64(s.TotalBytes)/(1024*1024))
	fmt.Println("-----------------------------------------------------------------")
	fmt.Println("Current Throughput (interval):")
	fmt.Printf("  Operations/sec: %.2f (Inserts: %.2f/s, Updates: %.2f/s)\n",
		s.OpsPerSec, s.InsertsPerSec, s.UpdatesPerSec)
	fmt.Printf("  Throughput: %.2f MB/s\n", s.BytesPerSec/(1024*1024))
	fmt.Printf("  Errors/sec: %.2f\n", s.ErrorsPerSec)
	fmt.Println("-----------------------------------------------------------------")
	fmt.Println("Latency Statistics:")
	if s.AvgInsertLatency > 0 {
		fmt.Printf("  Inserts - Avg: %v, P95: %v, P99: %v\n",
			s.AvgInsertLatency.Round(time.Microsecond),
			s.P95InsertLatency.Round(time.Microsecond),
			s.P99InsertLatency.Round(time.Microsecond))
	}
	if s.AvgUpdateLatency > 0 {
		fmt.Printf("  Updates - Avg: %v, P95: %v, P99: %v\n",
			s.AvgUpdateLatency.Round(time.Microsecond),
			s.P95UpdateLatency.Round(time.Microsecond),
			s.P99UpdateLatency.Round(time.Microsecond))
	}
	fmt.Println("-----------------------------------------------------------------")
	fmt.Println("Connection Pool:")
	fmt.Printf("  Active: %d, Max: %d, Available: %d\n",
		s.ActiveConns, s.MaxConns, s.AvailableConns)
	fmt.Println("=================================================================")
}

// Helper functions for percentile calculation
func calculateAvg(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	var sum time.Duration
	for _, d := range durations {
		sum += d
	}
	return sum / time.Duration(len(durations))
}

func calculatePercentile(durations []time.Duration, percentile int) time.Duration {
	if len(durations) == 0 {
		return 0
	}

	// Create a copy and sort it
	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)

	// Simple bubble sort (good enough for our sampling size)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	index := int(float64(len(sorted)) * float64(percentile) / 100.0)
	if index >= len(sorted) {
		index = len(sorted) - 1
	}

	return sorted[index]
}
