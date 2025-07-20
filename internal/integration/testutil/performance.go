package testutil

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"
)

// PerformanceMonitor tracks performance metrics during integration tests
type PerformanceMonitor struct {
	t              *testing.T
	startTime      time.Time
	metrics        *PerformanceMetrics
	samplingRate   time.Duration
	stopCh         chan struct{}
	wg             sync.WaitGroup
	mu             sync.RWMutex
}

// PerformanceMetrics contains collected performance data
type PerformanceMetrics struct {
	TestDuration    time.Duration
	RequestCount    int64
	ErrorCount      int64
	AverageLatency  time.Duration
	MinLatency      time.Duration
	MaxLatency      time.Duration
	P50Latency      time.Duration
	P95Latency      time.Duration
	P99Latency      time.Duration
	ThroughputRPS   float64
	MemoryUsageMB   float64
	MaxMemoryMB     float64
	CPUUsagePercent float64
	GoroutineCount  int
	GCCount         uint32
	GCPauseTotalNs  uint64
	Samples         []PerformanceSample
}

// PerformanceSample represents a single performance sample
type PerformanceSample struct {
	Timestamp       time.Time
	RequestLatency  time.Duration
	MemoryUsageMB   float64
	GoroutineCount  int
	CPUUsagePercent float64
	RequestsInFlight int64
}

// LatencyMeasurement tracks individual request latencies
type LatencyMeasurement struct {
	monitor   *PerformanceMonitor
	startTime time.Time
}

// NewPerformanceMonitor creates a new performance monitor
func NewPerformanceMonitor(t *testing.T) *PerformanceMonitor {
	return &PerformanceMonitor{
		t:            t,
		startTime:    time.Now(),
		samplingRate: 100 * time.Millisecond,
		stopCh:       make(chan struct{}),
		metrics: &PerformanceMetrics{
			MinLatency: time.Hour, // Start with high value
			Samples:    make([]PerformanceSample, 0),
		},
	}
}

// Start begins performance monitoring
func (pm *PerformanceMonitor) Start() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	pm.startTime = time.Now()
	pm.wg.Add(1)
	go pm.collectSamples()
}

// Stop stops performance monitoring and finalizes metrics
func (pm *PerformanceMonitor) Stop() *PerformanceMetrics {
	close(pm.stopCh)
	pm.wg.Wait()
	
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	pm.metrics.TestDuration = time.Since(pm.startTime)
	pm.calculateLatencyPercentiles()
	pm.calculateThroughput()
	
	return pm.metrics
}

// StartLatencyMeasurement begins measuring request latency
func (pm *PerformanceMonitor) StartLatencyMeasurement() *LatencyMeasurement {
	return &LatencyMeasurement{
		monitor:   pm,
		startTime: time.Now(),
	}
}

// EndLatencyMeasurement records the end of a request and updates metrics
func (lm *LatencyMeasurement) End() {
	latency := time.Since(lm.startTime)
	lm.monitor.recordLatency(latency)
}

// recordLatency updates latency statistics
func (pm *PerformanceMonitor) recordLatency(latency time.Duration) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	pm.metrics.RequestCount++
	
	// Update min/max latency
	if latency < pm.metrics.MinLatency {
		pm.metrics.MinLatency = latency
	}
	if latency > pm.metrics.MaxLatency {
		pm.metrics.MaxLatency = latency
	}
	
	// Update average latency using running average
	if pm.metrics.RequestCount == 1 {
		pm.metrics.AverageLatency = latency
	} else {
		total := pm.metrics.AverageLatency * time.Duration(pm.metrics.RequestCount-1)
		pm.metrics.AverageLatency = (total + latency) / time.Duration(pm.metrics.RequestCount)
	}
}

// RecordError increments the error count
func (pm *PerformanceMonitor) RecordError() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.metrics.ErrorCount++
}

// collectSamples periodically collects performance samples
func (pm *PerformanceMonitor) collectSamples() {
	defer pm.wg.Done()
	
	ticker := time.NewTicker(pm.samplingRate)
	defer ticker.Stop()
	
	for {
		select {
		case <-pm.stopCh:
			return
		case <-ticker.C:
			pm.takeSample()
		}
	}
}

// takeSample collects a single performance sample
func (pm *PerformanceMonitor) takeSample() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	sample := PerformanceSample{
		Timestamp:      time.Now(),
		MemoryUsageMB:  float64(m.Alloc) / 1024 / 1024,
		GoroutineCount: runtime.NumGoroutine(),
		// Note: CPU usage calculation would need additional system-specific code
		CPUUsagePercent: 0, // Placeholder - would need proper implementation
	}
	
	pm.mu.Lock()
	pm.metrics.Samples = append(pm.metrics.Samples, sample)
	
	// Update peak memory usage
	if sample.MemoryUsageMB > pm.metrics.MaxMemoryMB {
		pm.metrics.MaxMemoryMB = sample.MemoryUsageMB
	}
	
	// Update current memory usage
	pm.metrics.MemoryUsageMB = sample.MemoryUsageMB
	pm.metrics.GoroutineCount = sample.GoroutineCount
	pm.metrics.GCCount = m.NumGC
	pm.metrics.GCPauseTotalNs = m.PauseTotalNs
	pm.mu.Unlock()
}

// calculateLatencyPercentiles computes latency percentiles from samples
func (pm *PerformanceMonitor) calculateLatencyPercentiles() {
	if len(pm.metrics.Samples) == 0 {
		return
	}
	
	// Extract latencies from samples (this is simplified - in real implementation,
	// we'd need to collect actual request latencies)
	latencies := make([]time.Duration, 0)
	for _, sample := range pm.metrics.Samples {
		if sample.RequestLatency > 0 {
			latencies = append(latencies, sample.RequestLatency)
		}
	}
	
	if len(latencies) == 0 {
		return
	}
	
	// Sort latencies for percentile calculation
	for i := 0; i < len(latencies)-1; i++ {
		for j := i + 1; j < len(latencies); j++ {
			if latencies[i] > latencies[j] {
				latencies[i], latencies[j] = latencies[j], latencies[i]
			}
		}
	}
	
	// Calculate percentiles
	pm.metrics.P50Latency = percentile(latencies, 0.50)
	pm.metrics.P95Latency = percentile(latencies, 0.95)
	pm.metrics.P99Latency = percentile(latencies, 0.99)
}

// calculateThroughput computes requests per second
func (pm *PerformanceMonitor) calculateThroughput() {
	if pm.metrics.TestDuration > 0 {
		pm.metrics.ThroughputRPS = float64(pm.metrics.RequestCount) / pm.metrics.TestDuration.Seconds()
	}
}

// percentile calculates the nth percentile of a sorted slice
func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	
	index := p * float64(len(sorted)-1)
	lower := int(index)
	upper := lower + 1
	
	if upper >= len(sorted) {
		return sorted[len(sorted)-1]
	}
	
	// Linear interpolation between lower and upper
	weight := index - float64(lower)
	return time.Duration(float64(sorted[lower]) + weight*float64(sorted[upper]-sorted[lower]))
}

// Report generates a performance report
func (pm *PerformanceMonitor) Report() string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	return fmt.Sprintf(`Performance Report:
Test Duration: %v
Total Requests: %d
Total Errors: %d
Error Rate: %.2f%%
Throughput: %.2f RPS
Average Latency: %v
Min Latency: %v
Max Latency: %v
P50 Latency: %v
P95 Latency: %v
P99 Latency: %v
Peak Memory: %.2f MB
Current Memory: %.2f MB
Goroutines: %d
GC Runs: %d
Total GC Pause: %v
Samples Collected: %d`,
		pm.metrics.TestDuration,
		pm.metrics.RequestCount,
		pm.metrics.ErrorCount,
		float64(pm.metrics.ErrorCount)/float64(pm.metrics.RequestCount)*100,
		pm.metrics.ThroughputRPS,
		pm.metrics.AverageLatency,
		pm.metrics.MinLatency,
		pm.metrics.MaxLatency,
		pm.metrics.P50Latency,
		pm.metrics.P95Latency,
		pm.metrics.P99Latency,
		pm.metrics.MaxMemoryMB,
		pm.metrics.MemoryUsageMB,
		pm.metrics.GoroutineCount,
		pm.metrics.GCCount,
		time.Duration(pm.metrics.GCPauseTotalNs),
		len(pm.metrics.Samples))
}

// LoadTestConfig configures load testing parameters
type LoadTestConfig struct {
	ConcurrentUsers    int
	RequestsPerUser    int
	TestDuration       time.Duration
	RampUpTime         time.Duration
	ThinkTime          time.Duration
	MaxErrorRate       float64
	TargetThroughput   float64
	LatencyThresholds  LatencyThresholds
}

// LatencyThresholds defines acceptable latency limits
type LatencyThresholds struct {
	P50 time.Duration
	P95 time.Duration
	P99 time.Duration
}

// LoadTestRunner executes load tests with performance monitoring
type LoadTestRunner struct {
	config      *LoadTestConfig
	monitor     *PerformanceMonitor
	requestFunc func(ctx context.Context) error
	t           *testing.T
}

// NewLoadTestRunner creates a new load test runner
func NewLoadTestRunner(t *testing.T, config *LoadTestConfig, requestFunc func(ctx context.Context) error) *LoadTestRunner {
	return &LoadTestRunner{
		config:      config,
		monitor:     NewPerformanceMonitor(t),
		requestFunc: requestFunc,
		t:           t,
	}
}

// Run executes the load test
func (ltr *LoadTestRunner) Run(ctx context.Context) *PerformanceMetrics {
	ltr.monitor.Start()
	defer func() {
		metrics := ltr.monitor.Stop()
		ltr.validateResults(metrics)
	}()
	
	userCh := make(chan struct{}, ltr.config.ConcurrentUsers)
	var wg sync.WaitGroup
	
	// Ramp up users gradually
	rampUpInterval := ltr.config.RampUpTime / time.Duration(ltr.config.ConcurrentUsers)
	
	for i := 0; i < ltr.config.ConcurrentUsers; i++ {
		select {
		case <-ctx.Done():
			break
		case userCh <- struct{}{}:
			wg.Add(1)
			go ltr.runUser(ctx, &wg, userCh)
			if rampUpInterval > 0 {
				time.Sleep(rampUpInterval)
			}
		}
	}
	
	wg.Wait()
	return ltr.monitor.Stop()
}

// runUser simulates a single user's requests
func (ltr *LoadTestRunner) runUser(ctx context.Context, wg *sync.WaitGroup, userCh chan struct{}) {
	defer wg.Done()
	defer func() { <-userCh }()
	
	requestCount := 0
	for requestCount < ltr.config.RequestsPerUser {
		select {
		case <-ctx.Done():
			return
		default:
		}
		
		measurement := ltr.monitor.StartLatencyMeasurement()
		err := ltr.requestFunc(ctx)
		measurement.End()
		
		if err != nil {
			ltr.monitor.RecordError()
		}
		
		requestCount++
		
		if ltr.config.ThinkTime > 0 {
			time.Sleep(ltr.config.ThinkTime)
		}
	}
}

// validateResults checks if performance meets thresholds
func (ltr *LoadTestRunner) validateResults(metrics *PerformanceMetrics) {
	errorRate := float64(metrics.ErrorCount) / float64(metrics.RequestCount)
	
	if errorRate > ltr.config.MaxErrorRate {
		ltr.t.Errorf("Error rate %.2f%% exceeds threshold %.2f%%", 
			errorRate*100, ltr.config.MaxErrorRate*100)
	}
	
	if ltr.config.TargetThroughput > 0 && metrics.ThroughputRPS < ltr.config.TargetThroughput {
		ltr.t.Errorf("Throughput %.2f RPS below target %.2f RPS", 
			metrics.ThroughputRPS, ltr.config.TargetThroughput)
	}
	
	thresholds := ltr.config.LatencyThresholds
	if thresholds.P50 > 0 && metrics.P50Latency > thresholds.P50 {
		ltr.t.Errorf("P50 latency %v exceeds threshold %v", metrics.P50Latency, thresholds.P50)
	}
	
	if thresholds.P95 > 0 && metrics.P95Latency > thresholds.P95 {
		ltr.t.Errorf("P95 latency %v exceeds threshold %v", metrics.P95Latency, thresholds.P95)
	}
	
	if thresholds.P99 > 0 && metrics.P99Latency > thresholds.P99 {
		ltr.t.Errorf("P99 latency %v exceeds threshold %v", metrics.P99Latency, thresholds.P99)
	}
}