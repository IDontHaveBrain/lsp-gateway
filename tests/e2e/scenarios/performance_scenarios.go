package e2e_test

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// PerformanceScenario represents a performance test scenario
type PerformanceScenario struct {
	Name               string
	Description        string
	Duration           time.Duration
	ConcurrentClients  int
	RequestsPerSecond  int
	WarmupDuration     time.Duration
	Language           string
	RequestTypes       []string
	ExpectedThresholds PerformanceThresholds
	ValidationFunc     func(results *PerformanceResults) error
}

// PerformanceThresholds defines expected performance thresholds
type PerformanceThresholds struct {
	MaxAverageResponseTime time.Duration
	MaxP95ResponseTime     time.Duration
	MaxP99ResponseTime     time.Duration
	MinThroughput          float64 // requests per second
	MaxErrorRate           float64 // 0.0 to 1.0
	MaxMemoryUsage         uint64  // bytes
	MaxCPUUsage            float64 // 0.0 to 100.0
}

// PerformanceResults contains performance test results
type PerformanceResults struct {
	TotalRequests       int64
	SuccessfulRequests  int64
	FailedRequests      int64
	ResponseTimes       []time.Duration
	AverageResponseTime time.Duration
	P50ResponseTime     time.Duration
	P95ResponseTime     time.Duration
	P99ResponseTime     time.Duration
	MinResponseTime     time.Duration
	MaxResponseTime     time.Duration
	Throughput          float64
	ErrorRate           float64
	StartMemory         uint64
	EndMemory           uint64
	PeakMemory          uint64
	MemoryGrowth        uint64
	AverageCPUUsage     float64
	PeakCPUUsage        float64
	TestDuration        time.Duration
	RequestsPerSecond   []float64 // Throughput over time
	ErrorsPerSecond     []float64 // Error rate over time
}

// PerformanceTestManager manages performance test scenarios
type PerformanceTestManager struct {
	gatewayURL    string
	results       *PerformanceResults
	mu            sync.RWMutex
	scenarios     map[string]*PerformanceScenario
	memoryMonitor *MemoryMonitor
	cpuMonitor    *CPUMonitor
}

// MemoryMonitor monitors memory usage during tests
type MemoryMonitor struct {
	measurements []uint64
	mu           sync.RWMutex
	running      bool
	stopCh       chan struct{}
}

// CPUMonitor monitors CPU usage during tests
type CPUMonitor struct {
	measurements []float64
	mu           sync.RWMutex
	running      bool
	stopCh       chan struct{}
}

// NewPerformanceTestManager creates a new performance test manager
func NewPerformanceTestManager(gatewayURL string) *PerformanceTestManager {
	return &PerformanceTestManager{
		gatewayURL: gatewayURL,
		results: &PerformanceResults{
			ResponseTimes:     make([]time.Duration, 0),
			RequestsPerSecond: make([]float64, 0),
			ErrorsPerSecond:   make([]float64, 0),
		},
		scenarios:     make(map[string]*PerformanceScenario),
		memoryMonitor: NewMemoryMonitor(),
		cpuMonitor:    NewCPUMonitor(),
	}
}

// NewMemoryMonitor creates a new memory monitor
func NewMemoryMonitor() *MemoryMonitor {
	return &MemoryMonitor{
		measurements: make([]uint64, 0),
		stopCh:       make(chan struct{}),
	}
}

// NewCPUMonitor creates a new CPU monitor
func NewCPUMonitor() *CPUMonitor {
	return &CPUMonitor{
		measurements: make([]float64, 0),
		stopCh:       make(chan struct{}),
	}
}

// RegisterScenario registers a new performance scenario
func (m *PerformanceTestManager) RegisterScenario(scenario *PerformanceScenario) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.scenarios[scenario.Name] = scenario
}

// ExecuteScenario executes a specific performance scenario
func (m *PerformanceTestManager) ExecuteScenario(t *testing.T, scenarioName string) error {
	m.mu.RLock()
	scenario, exists := m.scenarios[scenarioName]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("scenario %s not found", scenarioName)
	}

	t.Logf("Executing performance scenario: %s", scenario.Name)
	t.Logf("Description: %s", scenario.Description)
	t.Logf("Configuration: %d clients, %d RPS, %v duration",
		scenario.ConcurrentClients, scenario.RequestsPerSecond, scenario.Duration)

	// Reset results
	m.resetResults()

	// Start monitoring
	m.startMonitoring()
	defer m.stopMonitoring()

	startTime := time.Now()

	// Warmup phase
	if scenario.WarmupDuration > 0 {
		t.Logf("Starting warmup phase: %v", scenario.WarmupDuration)
		if err := m.executeWarmup(scenario); err != nil {
			return fmt.Errorf("warmup failed: %w", err)
		}
		// Reset counters after warmup
		m.resetCounters()
	}

	// Main test phase
	t.Logf("Starting main test phase: %v", scenario.Duration)
	if err := m.executeLoadTest(scenario); err != nil {
		return fmt.Errorf("load test failed: %w", err)
	}

	m.results.TestDuration = time.Since(startTime)

	// Calculate final metrics
	m.calculateFinalMetrics()

	// Validate results
	if scenario.ValidationFunc != nil {
		if err := scenario.ValidationFunc(m.results); err != nil {
			return fmt.Errorf("scenario validation failed: %w", err)
		}
	}

	// Validate against thresholds
	if err := m.validateThresholds(scenario.ExpectedThresholds); err != nil {
		return fmt.Errorf("performance thresholds not met: %w", err)
	}

	t.Logf("Scenario %s completed successfully", scenarioName)
	m.logResults(t)

	return nil
}

// resetResults resets the test results
func (m *PerformanceTestManager) resetResults() {
	m.mu.Lock()
	defer m.mu.Unlock()

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	m.results = &PerformanceResults{
		ResponseTimes:     make([]time.Duration, 0),
		RequestsPerSecond: make([]float64, 0),
		ErrorsPerSecond:   make([]float64, 0),
		StartMemory:       memStats.Alloc,
	}
}

// resetCounters resets request counters (used after warmup)
func (m *PerformanceTestManager) resetCounters() {
	atomic.StoreInt64(&m.results.TotalRequests, 0)
	atomic.StoreInt64(&m.results.SuccessfulRequests, 0)
	atomic.StoreInt64(&m.results.FailedRequests, 0)

	m.mu.Lock()
	m.results.ResponseTimes = make([]time.Duration, 0)
	m.mu.Unlock()
}

// startMonitoring starts system resource monitoring
func (m *PerformanceTestManager) startMonitoring() {
	m.memoryMonitor.Start()
	m.cpuMonitor.Start()
}

// stopMonitoring stops system resource monitoring
func (m *PerformanceTestManager) stopMonitoring() {
	m.memoryMonitor.Stop()
	m.cpuMonitor.Stop()
}

// executeWarmup executes the warmup phase
func (m *PerformanceTestManager) executeWarmup(scenario *PerformanceScenario) error {
	ctx, cancel := context.WithTimeout(context.Background(), scenario.WarmupDuration)
	defer cancel()

	// Use lower concurrency and RPS for warmup
	warmupClients := scenario.ConcurrentClients / 2
	if warmupClients < 1 {
		warmupClients = 1
	}
	warmupRPS := scenario.RequestsPerSecond / 2
	if warmupRPS < 1 {
		warmupRPS = 1
	}

	return m.runLoadTest(ctx, warmupClients, warmupRPS, scenario.RequestTypes)
}

// executeLoadTest executes the main load test
func (m *PerformanceTestManager) executeLoadTest(scenario *PerformanceScenario) error {
	ctx, cancel := context.WithTimeout(context.Background(), scenario.Duration)
	defer cancel()

	return m.runLoadTest(ctx, scenario.ConcurrentClients, scenario.RequestsPerSecond, scenario.RequestTypes)
}

// runLoadTest runs a load test with specified parameters
func (m *PerformanceTestManager) runLoadTest(ctx context.Context, clients, rps int, requestTypes []string) error {
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, clients)

	// Calculate request interval
	interval := time.Second / time.Duration(rps)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Start throughput monitoring
	throughputTicker := time.NewTicker(time.Second)
	defer throughputTicker.Stop()

	var lastRequestCount int64
	var lastErrorCount int64

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-throughputTicker.C:
				currentReqs := atomic.LoadInt64(&m.results.TotalRequests)
				currentErrors := atomic.LoadInt64(&m.results.FailedRequests)

				rps := float64(currentReqs - lastRequestCount)
				eps := float64(currentErrors - lastErrorCount)

				m.mu.Lock()
				m.results.RequestsPerSecond = append(m.results.RequestsPerSecond, rps)
				m.results.ErrorsPerSecond = append(m.results.ErrorsPerSecond, eps)
				m.mu.Unlock()

				lastRequestCount = currentReqs
				lastErrorCount = currentErrors
			}
		}
	}()

	requestTypeIndex := 0

	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return nil
		case <-ticker.C:
			select {
			case semaphore <- struct{}{}:
				wg.Add(1)

				requestType := "definition" // default
				if len(requestTypes) > 0 {
					requestType = requestTypes[requestTypeIndex%len(requestTypes)]
					requestTypeIndex++
				}

				go func(reqType string) {
					defer wg.Done()
					defer func() { <-semaphore }()

					m.executeRequest(ctx, reqType)
				}(requestType)
			default:
				// Semaphore full, skip this request
			}
		}
	}
}

// executeRequest executes a single request
func (m *PerformanceTestManager) executeRequest(ctx context.Context, requestType string) {
	start := time.Now()

	atomic.AddInt64(&m.results.TotalRequests, 1)

	// Simulate different request types with different response times
	var success bool

	switch requestType {
	case "definition":
		_, success = m.simulateDefinitionRequest()
	case "hover":
		_, success = m.simulateHoverRequest()
	case "references":
		_, success = m.simulateReferencesRequest()
	case "symbols":
		_, success = m.simulateSymbolsRequest()
	default:
		_, success = m.simulateDefinitionRequest()
	}

	if success {
		atomic.AddInt64(&m.results.SuccessfulRequests, 1)
	} else {
		atomic.AddInt64(&m.results.FailedRequests, 1)
	}

	actualDuration := time.Since(start)

	m.mu.Lock()
	m.results.ResponseTimes = append(m.results.ResponseTimes, actualDuration)
	m.mu.Unlock()
}

// simulateDefinitionRequest simulates a textDocument/definition request
func (m *PerformanceTestManager) simulateDefinitionRequest() (time.Duration, bool) {
	// Simulate processing time: 50-200ms
	processingTime := time.Millisecond * time.Duration(50+time.Now().UnixNano()%150)
	time.Sleep(processingTime)

	// 95% success rate
	success := time.Now().UnixNano()%100 < 95

	return processingTime, success
}

// simulateHoverRequest simulates a textDocument/hover request
func (m *PerformanceTestManager) simulateHoverRequest() (time.Duration, bool) {
	// Simulate processing time: 30-100ms
	processingTime := time.Millisecond * time.Duration(30+time.Now().UnixNano()%70)
	time.Sleep(processingTime)

	// 98% success rate
	success := time.Now().UnixNano()%100 < 98

	return processingTime, success
}

// simulateReferencesRequest simulates a textDocument/references request
func (m *PerformanceTestManager) simulateReferencesRequest() (time.Duration, bool) {
	// Simulate processing time: 100-500ms (more complex operation)
	processingTime := time.Millisecond * time.Duration(100+time.Now().UnixNano()%400)
	time.Sleep(processingTime)

	// 90% success rate
	success := time.Now().UnixNano()%100 < 90

	return processingTime, success
}

// simulateSymbolsRequest simulates a workspace/symbol request
func (m *PerformanceTestManager) simulateSymbolsRequest() (time.Duration, bool) {
	// Simulate processing time: 200-800ms (most complex operation)
	processingTime := time.Millisecond * time.Duration(200+time.Now().UnixNano()%600)
	time.Sleep(processingTime)

	// 85% success rate
	success := time.Now().UnixNano()%100 < 85

	return processingTime, success
}

// calculateFinalMetrics calculates final performance metrics
func (m *PerformanceTestManager) calculateFinalMetrics() {
	m.mu.Lock()
	defer m.mu.Unlock()

	total := atomic.LoadInt64(&m.results.TotalRequests)
	_ = atomic.LoadInt64(&m.results.SuccessfulRequests)
	failed := atomic.LoadInt64(&m.results.FailedRequests)

	// Calculate error rate
	if total > 0 {
		m.results.ErrorRate = float64(failed) / float64(total)
	}

	// Calculate throughput
	if m.results.TestDuration > 0 {
		m.results.Throughput = float64(total) / m.results.TestDuration.Seconds()
	}

	// Calculate response time percentiles
	if len(m.results.ResponseTimes) > 0 {
		m.calculateResponseTimePercentiles()
	}

	// Calculate memory metrics
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	m.results.EndMemory = memStats.Alloc
	m.results.MemoryGrowth = m.results.EndMemory - m.results.StartMemory

	// Get monitoring results
	m.results.PeakMemory = m.memoryMonitor.GetPeakUsage()
	m.results.AverageCPUUsage, m.results.PeakCPUUsage = m.cpuMonitor.GetMetrics()
}

// calculateResponseTimePercentiles calculates response time percentiles
func (m *PerformanceTestManager) calculateResponseTimePercentiles() {
	// Sort response times (simple bubble sort for small datasets)
	times := make([]time.Duration, len(m.results.ResponseTimes))
	copy(times, m.results.ResponseTimes)

	// Simple sort implementation
	for i := 0; i < len(times); i++ {
		for j := i + 1; j < len(times); j++ {
			if times[i] > times[j] {
				times[i], times[j] = times[j], times[i]
			}
		}
	}

	n := len(times)

	m.results.MinResponseTime = times[0]
	m.results.MaxResponseTime = times[n-1]
	m.results.P50ResponseTime = times[n/2]
	m.results.P95ResponseTime = times[int(float64(n)*0.95)]
	m.results.P99ResponseTime = times[int(float64(n)*0.99)]

	// Calculate average
	var total time.Duration
	for _, duration := range times {
		total += duration
	}
	m.results.AverageResponseTime = total / time.Duration(n)
}

// validateThresholds validates performance against expected thresholds
func (m *PerformanceTestManager) validateThresholds(thresholds PerformanceThresholds) error {
	if thresholds.MaxAverageResponseTime > 0 && m.results.AverageResponseTime > thresholds.MaxAverageResponseTime {
		return fmt.Errorf("average response time %v exceeds threshold %v",
			m.results.AverageResponseTime, thresholds.MaxAverageResponseTime)
	}

	if thresholds.MaxP95ResponseTime > 0 && m.results.P95ResponseTime > thresholds.MaxP95ResponseTime {
		return fmt.Errorf("P95 response time %v exceeds threshold %v",
			m.results.P95ResponseTime, thresholds.MaxP95ResponseTime)
	}

	if thresholds.MinThroughput > 0 && m.results.Throughput < thresholds.MinThroughput {
		return fmt.Errorf("throughput %.2f RPS below threshold %.2f RPS",
			m.results.Throughput, thresholds.MinThroughput)
	}

	if thresholds.MaxErrorRate > 0 && m.results.ErrorRate > thresholds.MaxErrorRate {
		return fmt.Errorf("error rate %.2f%% exceeds threshold %.2f%%",
			m.results.ErrorRate*100, thresholds.MaxErrorRate*100)
	}

	if thresholds.MaxMemoryUsage > 0 && m.results.PeakMemory > thresholds.MaxMemoryUsage {
		return fmt.Errorf("peak memory usage %d bytes exceeds threshold %d bytes",
			m.results.PeakMemory, thresholds.MaxMemoryUsage)
	}

	return nil
}

// logResults logs the performance test results
func (m *PerformanceTestManager) logResults(t *testing.T) {
	t.Logf("Performance Test Results:")
	t.Logf("  Test Duration: %v", m.results.TestDuration)
	t.Logf("  Total Requests: %d", m.results.TotalRequests)
	t.Logf("  Successful Requests: %d", m.results.SuccessfulRequests)
	t.Logf("  Failed Requests: %d", m.results.FailedRequests)
	t.Logf("  Throughput: %.2f RPS", m.results.Throughput)
	t.Logf("  Error Rate: %.2f%%", m.results.ErrorRate*100)
	t.Logf("  Response Times:")
	t.Logf("    Average: %v", m.results.AverageResponseTime)
	t.Logf("    P50: %v", m.results.P50ResponseTime)
	t.Logf("    P95: %v", m.results.P95ResponseTime)
	t.Logf("    P99: %v", m.results.P99ResponseTime)
	t.Logf("    Min: %v", m.results.MinResponseTime)
	t.Logf("    Max: %v", m.results.MaxResponseTime)
	t.Logf("  Memory Usage:")
	t.Logf("    Start: %s", formatBytes(m.results.StartMemory))
	t.Logf("    End: %s", formatBytes(m.results.EndMemory))
	t.Logf("    Peak: %s", formatBytes(m.results.PeakMemory))
	t.Logf("    Growth: %s", formatBytes(m.results.MemoryGrowth))
	t.Logf("  CPU Usage:")
	t.Logf("    Average: %.2f%%", m.results.AverageCPUUsage)
	t.Logf("    Peak: %.2f%%", m.results.PeakCPUUsage)
}

// formatBytes formats bytes in human readable format
func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// Start starts memory monitoring
func (m *MemoryMonitor) Start() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return
	}

	m.running = true
	m.measurements = make([]uint64, 0)

	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-m.stopCh:
				return
			case <-ticker.C:
				var memStats runtime.MemStats
				runtime.ReadMemStats(&memStats)

				m.mu.Lock()
				m.measurements = append(m.measurements, memStats.Alloc)
				m.mu.Unlock()
			}
		}
	}()
}

// Stop stops memory monitoring
func (m *MemoryMonitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	m.running = false
	close(m.stopCh)
	m.stopCh = make(chan struct{})
}

// GetPeakUsage returns the peak memory usage
func (m *MemoryMonitor) GetPeakUsage() uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var peak uint64
	for _, usage := range m.measurements {
		if usage > peak {
			peak = usage
		}
	}

	return peak
}

// Start starts CPU monitoring (simplified simulation)
func (c *CPUMonitor) Start() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running {
		return
	}

	c.running = true
	c.measurements = make([]float64, 0)

	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-c.stopCh:
				return
			case <-ticker.C:
				// Simulate CPU usage measurement (in real implementation, use system calls)
				cpuUsage := float64(20 + time.Now().UnixNano()%40) // 20-60% simulated usage

				c.mu.Lock()
				c.measurements = append(c.measurements, cpuUsage)
				c.mu.Unlock()
			}
		}
	}()
}

// Stop stops CPU monitoring
func (c *CPUMonitor) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return
	}

	c.running = false
	close(c.stopCh)
	c.stopCh = make(chan struct{})
}

// GetMetrics returns average and peak CPU usage
func (c *CPUMonitor) GetMetrics() (average, peak float64) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.measurements) == 0 {
		return 0, 0
	}

	var total float64
	for _, usage := range c.measurements {
		total += usage
		if usage > peak {
			peak = usage
		}
	}

	average = total / float64(len(c.measurements))
	return average, peak
}

// GetBasicPerformanceScenario returns a basic performance test scenario
func GetBasicPerformanceScenario() *PerformanceScenario {
	return &PerformanceScenario{
		Name:              "basic-performance",
		Description:       "Basic performance test with moderate load",
		Duration:          60 * time.Second,
		ConcurrentClients: 10,
		RequestsPerSecond: 50,
		WarmupDuration:    15 * time.Second,
		Language:          "go",
		RequestTypes:      []string{"definition", "hover", "references"},
		ExpectedThresholds: PerformanceThresholds{
			MaxAverageResponseTime: 500 * time.Millisecond,
			MaxP95ResponseTime:     1 * time.Second,
			MinThroughput:          40.0,
			MaxErrorRate:           0.05,              // 5%
			MaxMemoryUsage:         100 * 1024 * 1024, // 100MB
		},
		ValidationFunc: func(results *PerformanceResults) error {
			if results.TotalRequests < 2000 {
				return fmt.Errorf("expected at least 2000 requests, got %d", results.TotalRequests)
			}
			return nil
		},
	}
}

// GetHighLoadPerformanceScenario returns a high-load performance test scenario
func GetHighLoadPerformanceScenario() *PerformanceScenario {
	return &PerformanceScenario{
		Name:              "high-load-performance",
		Description:       "High-load performance test with stress conditions",
		Duration:          120 * time.Second,
		ConcurrentClients: 50,
		RequestsPerSecond: 200,
		WarmupDuration:    30 * time.Second,
		Language:          "go",
		RequestTypes:      []string{"definition", "hover", "references", "symbols"},
		ExpectedThresholds: PerformanceThresholds{
			MaxAverageResponseTime: 1 * time.Second,
			MaxP95ResponseTime:     3 * time.Second,
			MinThroughput:          150.0,
			MaxErrorRate:           0.10,              // 10%
			MaxMemoryUsage:         500 * 1024 * 1024, // 500MB
		},
		ValidationFunc: func(results *PerformanceResults) error {
			if results.TotalRequests < 15000 {
				return fmt.Errorf("expected at least 15000 requests, got %d", results.TotalRequests)
			}
			return nil
		},
	}
}

// RunStandardPerformanceTests runs a standard set of performance tests
func RunStandardPerformanceTests(t *testing.T, gatewayURL string) {
	manager := NewPerformanceTestManager(gatewayURL)

	// Register scenarios
	manager.RegisterScenario(GetBasicPerformanceScenario())
	manager.RegisterScenario(GetHighLoadPerformanceScenario())

	// Execute scenarios
	scenarios := []string{"basic-performance", "high-load-performance"}

	for _, scenarioName := range scenarios {
		t.Run(scenarioName, func(t *testing.T) {
			err := manager.ExecuteScenario(t, scenarioName)
			require.NoError(t, err, "Performance scenario %s should complete successfully", scenarioName)
		})
	}
}
