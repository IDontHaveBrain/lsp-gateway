package transport

import (
	"sync"
	"sync/atomic"
	"time"
)

// PoolMetrics handles comprehensive metrics collection for connection pools
type PoolMetrics struct {
	// Request metrics (atomic for thread safety)
	requestCount      int64
	successCount      int64
	errorCount        int64
	totalResponseTime int64 // in nanoseconds

	// Connection metrics (atomic)
	connectionCreations int64
	connectionDestroyed int64
	connectionReused    int64
	connectionTimeouts  int64

	// Resource metrics (protected by mutex)
	peakMemoryUsage int64
	peakCPUUsage    float64
	peakConnections int32

	// Circuit breaker metrics (atomic)
	circuitOpenCount  int64
	circuitCloseCount int64

	// Pool size tracking (atomic)
	currentPoolSize int32
	minPoolSize     int32
	maxPoolSize     int32

	// Timing metrics
	createdAt  time.Time
	lastUpdate time.Time

	// Snapshot data (protected by mutex)
	lastSnapshot *PoolStats

	mu sync.RWMutex
}

// NewPoolMetrics creates a new metrics collector
func NewPoolMetrics() *PoolMetrics {
	now := time.Now()
	return &PoolMetrics{
		createdAt:  now,
		lastUpdate: now,
	}
}

// RecordRequest records a request with its response time and success status
func (pm *PoolMetrics) RecordRequest(responseTime time.Duration, success bool) {
	atomic.AddInt64(&pm.requestCount, 1)
	atomic.AddInt64(&pm.totalResponseTime, int64(responseTime))

	if success {
		atomic.AddInt64(&pm.successCount, 1)
	} else {
		atomic.AddInt64(&pm.errorCount, 1)
	}

	pm.updateLastUpdate()
}

// RecordConnection records connection lifecycle events
func (pm *PoolMetrics) RecordConnection(action string) {
	switch action {
	case "created":
		atomic.AddInt64(&pm.connectionCreations, 1)
	case "destroyed":
		atomic.AddInt64(&pm.connectionDestroyed, 1)
	case "reused":
		atomic.AddInt64(&pm.connectionReused, 1)
	case "timeout":
		atomic.AddInt64(&pm.connectionTimeouts, 1)
	}

	pm.updateLastUpdate()
}

// RecordResource records resource usage metrics
func (pm *PoolMetrics) RecordResource(memoryMB int64, cpuPercent float64, connections int32) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if memoryMB > pm.peakMemoryUsage {
		pm.peakMemoryUsage = memoryMB
	}

	if cpuPercent > pm.peakCPUUsage {
		pm.peakCPUUsage = cpuPercent
	}

	if connections > pm.peakConnections {
		pm.peakConnections = connections
	}

	atomic.StoreInt32(&pm.currentPoolSize, connections)
	pm.updateMinMax(connections)
	pm.lastUpdate = time.Now()
}

// RecordCircuitState records circuit breaker state changes
func (pm *PoolMetrics) RecordCircuitState(state string) {
	switch state {
	case "opened":
		atomic.AddInt64(&pm.circuitOpenCount, 1)
	case "closed":
		atomic.AddInt64(&pm.circuitCloseCount, 1)
	}

	pm.updateLastUpdate()
}

// GetSnapshot returns a complete snapshot of current metrics
func (pm *PoolMetrics) GetSnapshot() *PoolStats {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	now := time.Now()
	requestCount := atomic.LoadInt64(&pm.requestCount)
	successCount := atomic.LoadInt64(&pm.successCount)
	errorCount := atomic.LoadInt64(&pm.errorCount)
	totalResponseTime := atomic.LoadInt64(&pm.totalResponseTime)

	connectionCreations := atomic.LoadInt64(&pm.connectionCreations)
	connectionDestroyed := atomic.LoadInt64(&pm.connectionDestroyed)
	connectionReused := atomic.LoadInt64(&pm.connectionReused)
	connectionTimeouts := atomic.LoadInt64(&pm.connectionTimeouts)

	currentPoolSize := atomic.LoadInt32(&pm.currentPoolSize)
	minPoolSize := atomic.LoadInt32(&pm.minPoolSize)
	maxPoolSize := atomic.LoadInt32(&pm.maxPoolSize)

	circuitOpenCount := atomic.LoadInt64(&pm.circuitOpenCount)
	circuitCloseCount := atomic.LoadInt64(&pm.circuitCloseCount)

	// Calculate derived metrics
	var avgResponseTime time.Duration
	if requestCount > 0 {
		avgResponseTime = time.Duration(totalResponseTime / requestCount)
	}

	var errorRate float64
	if requestCount > 0 {
		errorRate = float64(errorCount) / float64(requestCount)
	}

	var requestsPerSecond float64
	elapsed := now.Sub(pm.createdAt)
	if elapsed > 0 {
		requestsPerSecond = float64(requestCount) / elapsed.Seconds()
	}

	var creationRate, destructionRate, reuseRate float64
	if elapsed > 0 {
		creationRate = float64(connectionCreations) / elapsed.Seconds()
		destructionRate = float64(connectionDestroyed) / elapsed.Seconds()
		reuseRate = float64(connectionReused) / elapsed.Seconds()
	}

	// Calculate average connection age
	var avgConnectionAge time.Duration
	if connectionCreations > 0 && connectionDestroyed > 0 {
		avgConnectionAge = elapsed / time.Duration(connectionCreations)
	}

	// Determine circuit state
	circuitState := "closed"
	if circuitOpenCount > circuitCloseCount {
		circuitState = "open"
	} else if circuitOpenCount > 0 {
		circuitState = "half-open"
	}

	// Health determination - consider healthy if error rate < 10% and no recent timeouts
	isHealthy := errorRate < 0.1 && connectionTimeouts == 0

	stats := &PoolStats{
		// Connection counts
		TotalConnections:  int(currentPoolSize),
		ActiveConnections: int(currentPoolSize), // This would be refined in actual implementation
		IdleConnections:   0,                    // This would be calculated in actual implementation

		// Performance metrics
		AverageResponseTime: avgResponseTime,
		RequestsPerSecond:   requestsPerSecond,
		ErrorRate:           errorRate,

		// Resource usage (would be measured in actual implementation)
		MemoryUsageMB:   pm.peakMemoryUsage,
		CPUUsagePercent: pm.peakCPUUsage,

		// Health status
		IsHealthy:            isHealthy,
		LastHealthCheck:      now,
		UnhealthyConnections: int(connectionTimeouts),

		// Circuit breaker status
		CircuitState: circuitState,
		FailureCount: errorCount,
		SuccessCount: successCount,

		// Pool-specific metrics
		CreationRate:      creationRate,
		DestructionRate:   destructionRate,
		ReuseRate:         reuseRate,
		AvgConnectionAge:  avgConnectionAge,
		PeakConnections:   int(pm.peakConnections),
		MinConnections:    int(minPoolSize),
		ConfiguredMaxSize: int(maxPoolSize),
		ConfiguredMinSize: int(minPoolSize),
	}

	pm.lastSnapshot = stats
	return stats
}

// Reset resets all metrics to their initial state
func (pm *PoolMetrics) Reset() {
	atomic.StoreInt64(&pm.requestCount, 0)
	atomic.StoreInt64(&pm.successCount, 0)
	atomic.StoreInt64(&pm.errorCount, 0)
	atomic.StoreInt64(&pm.totalResponseTime, 0)

	atomic.StoreInt64(&pm.connectionCreations, 0)
	atomic.StoreInt64(&pm.connectionDestroyed, 0)
	atomic.StoreInt64(&pm.connectionReused, 0)
	atomic.StoreInt64(&pm.connectionTimeouts, 0)

	atomic.StoreInt64(&pm.circuitOpenCount, 0)
	atomic.StoreInt64(&pm.circuitCloseCount, 0)

	atomic.StoreInt32(&pm.currentPoolSize, 0)
	atomic.StoreInt32(&pm.minPoolSize, 0)
	atomic.StoreInt32(&pm.maxPoolSize, 0)

	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.peakMemoryUsage = 0
	pm.peakCPUUsage = 0
	pm.peakConnections = 0

	now := time.Now()
	pm.createdAt = now
	pm.lastUpdate = now
	pm.lastSnapshot = nil
}

// GetRequestMetrics returns request-specific metrics
func (pm *PoolMetrics) GetRequestMetrics() (total, success, error int64, avgResponseTime time.Duration) {
	total = atomic.LoadInt64(&pm.requestCount)
	success = atomic.LoadInt64(&pm.successCount)
	error = atomic.LoadInt64(&pm.errorCount)

	totalTime := atomic.LoadInt64(&pm.totalResponseTime)
	if total > 0 {
		avgResponseTime = time.Duration(totalTime / total)
	}

	return
}

// GetConnectionMetrics returns connection-specific metrics
func (pm *PoolMetrics) GetConnectionMetrics() (created, destroyed, reused, timeouts int64) {
	created = atomic.LoadInt64(&pm.connectionCreations)
	destroyed = atomic.LoadInt64(&pm.connectionDestroyed)
	reused = atomic.LoadInt64(&pm.connectionReused)
	timeouts = atomic.LoadInt64(&pm.connectionTimeouts)

	return
}

// GetResourceMetrics returns resource usage metrics
func (pm *PoolMetrics) GetResourceMetrics() (peakMemoryMB int64, peakCPU float64, peakConns int32) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	return pm.peakMemoryUsage, pm.peakCPUUsage, pm.peakConnections
}

// GetCircuitMetrics returns circuit breaker metrics
func (pm *PoolMetrics) GetCircuitMetrics() (openCount, closeCount int64) {
	openCount = atomic.LoadInt64(&pm.circuitOpenCount)
	closeCount = atomic.LoadInt64(&pm.circuitCloseCount)

	return
}

// GetCurrentPoolSize returns the current pool size
func (pm *PoolMetrics) GetCurrentPoolSize() int32 {
	return atomic.LoadInt32(&pm.currentPoolSize)
}

// GetUptime returns how long the metrics have been collecting
func (pm *PoolMetrics) GetUptime() time.Duration {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	return time.Since(pm.createdAt)
}

// GetLastUpdate returns when metrics were last updated
func (pm *PoolMetrics) GetLastUpdate() time.Time {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	return pm.lastUpdate
}

// updateLastUpdate atomically updates the last update time
func (pm *PoolMetrics) updateLastUpdate() {
	pm.mu.Lock()
	pm.lastUpdate = time.Now()
	pm.mu.Unlock()
}

// updateMinMax updates the minimum and maximum pool sizes
func (pm *PoolMetrics) updateMinMax(current int32) {
	// Update minimum
	for {
		min := atomic.LoadInt32(&pm.minPoolSize)
		if min == 0 || current < min {
			if atomic.CompareAndSwapInt32(&pm.minPoolSize, min, current) {
				break
			}
		} else {
			break
		}
	}

	// Update maximum
	for {
		max := atomic.LoadInt32(&pm.maxPoolSize)
		if current > max {
			if atomic.CompareAndSwapInt32(&pm.maxPoolSize, max, current) {
				break
			}
		} else {
			break
		}
	}
}

// Export method for testing
func (pm *PoolMetrics) ExportMetricsForTesting() map[string]interface{} {
	return map[string]interface{}{
		"request_count":        atomic.LoadInt64(&pm.requestCount),
		"success_count":        atomic.LoadInt64(&pm.successCount),
		"error_count":          atomic.LoadInt64(&pm.errorCount),
		"connection_creations": atomic.LoadInt64(&pm.connectionCreations),
		"connection_destroyed": atomic.LoadInt64(&pm.connectionDestroyed),
		"connection_reused":    atomic.LoadInt64(&pm.connectionReused),
		"connection_timeouts":  atomic.LoadInt64(&pm.connectionTimeouts),
		"circuit_open_count":   atomic.LoadInt64(&pm.circuitOpenCount),
		"circuit_close_count":  atomic.LoadInt64(&pm.circuitCloseCount),
		"current_pool_size":    atomic.LoadInt32(&pm.currentPoolSize),
		"peak_memory_usage":    pm.peakMemoryUsage,
		"peak_cpu_usage":       pm.peakCPUUsage,
		"peak_connections":     pm.peakConnections,
		"created_at":           pm.createdAt,
		"last_update":          pm.lastUpdate,
	}
}
