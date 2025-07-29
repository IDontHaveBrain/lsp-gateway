package workspace

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// LoadTestScenario defines a specific load testing scenario
type LoadTestScenario struct {
	Name              string        `json:"name"`
	WorkspaceCount    int           `json:"workspace_count"`
	RequestsPerSecond int           `json:"requests_per_second"`
	Duration          time.Duration `json:"duration"`
	Languages         []string      `json:"languages"`
	ConcurrentUsers   int           `json:"concurrent_users"`
	RampUpDuration    time.Duration `json:"ramp_up_duration"`
	RampDownDuration  time.Duration `json:"ramp_down_duration"`
	RequestPattern    RequestPattern `json:"request_pattern"`
	FailureThreshold  float64       `json:"failure_threshold"`
}

// RequestPattern defines how requests are distributed
type RequestPattern int

const (
	PatternUniform RequestPattern = iota
	PatternBurst
	PatternGradual
	PatternSpike
)

// LoadTestResults contains comprehensive load test results
type LoadTestResults struct {
	Scenario           *LoadTestScenario             `json:"scenario"`
	StartTime          time.Time                     `json:"start_time"`
	EndTime            time.Time                     `json:"end_time"`
	Duration           time.Duration                 `json:"duration"`
	TotalRequests      int64                         `json:"total_requests"`
	SuccessfulRequests int64                         `json:"successful_requests"`
	FailedRequests     int64                         `json:"failed_requests"`
	RequestsPerSecond  float64                       `json:"requests_per_second"`
	AverageLatency     time.Duration                 `json:"average_latency"`
	MedianLatency      time.Duration                 `json:"median_latency"`
	P95Latency         time.Duration                 `json:"p95_latency"`
	P99Latency         time.Duration                 `json:"p99_latency"`
	MinLatency         time.Duration                 `json:"min_latency"`
	MaxLatency         time.Duration                 `json:"max_latency"`
	ErrorRate          float64                       `json:"error_rate"`
	ThroughputMBps     float64                       `json:"throughput_mbps"`
	PeakMemoryUsage    int64                         `json:"peak_memory_usage"`
	CPUUsagePercent    float64                       `json:"cpu_usage_percent"`
	WorkspaceMetrics   map[string]*LoadTestWSMetrics `json:"workspace_metrics"`
	TimeSeriesData     []*LoadTestDataPoint          `json:"time_series_data"`
	Errors             []LoadTestError               `json:"errors"`
	Grade              string                        `json:"grade"`
	Passed             bool                          `json:"passed"`
}

// LoadTestWSMetrics contains per-workspace load test metrics
type LoadTestWSMetrics struct {
	WorkspaceName      string        `json:"workspace_name"`
	RequestCount       int64         `json:"request_count"`
	ErrorCount         int64         `json:"error_count"`
	AverageLatency     time.Duration `json:"average_latency"`
	MaxLatency         time.Duration `json:"max_latency"`
	MemoryUsage        int64         `json:"memory_usage"`
	ConnectionCount    int           `json:"connection_count"`
	ThroughputRPS      float64       `json:"throughput_rps"`
}

// LoadTestDataPoint represents a single measurement point during load testing
type LoadTestDataPoint struct {
	Timestamp         time.Time     `json:"timestamp"`
	RequestsPerSecond float64       `json:"requests_per_second"`
	AverageLatency    time.Duration `json:"average_latency"`
	ErrorRate         float64       `json:"error_rate"`
	MemoryUsage       int64         `json:"memory_usage"`
	ActiveConnections int           `json:"active_connections"`
}

// LoadTestError represents an error that occurred during load testing
type LoadTestError struct {
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	Count     int64     `json:"count"`
}

// TestConcurrentWorkspaceLoad tests performance under concurrent workspace load
func TestConcurrentWorkspaceLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	scenario := &LoadTestScenario{
		Name:              "concurrent_workspace_load",
		WorkspaceCount:    5,
		RequestsPerSecond: 100,
		Duration:          2 * time.Minute,
		Languages:         []string{"go", "python"},
		ConcurrentUsers:   20,
		RampUpDuration:    30 * time.Second,
		RampDownDuration:  15 * time.Second,
		RequestPattern:    PatternUniform,
		FailureThreshold:  0.05, // 5% failure rate
	}

	results := runLoadTestScenario(t, scenario)
	
	// Validate results
	if results.ErrorRate > scenario.FailureThreshold {
		t.Errorf("Error rate %.2f%% exceeds threshold %.2f%%", 
			results.ErrorRate*100, scenario.FailureThreshold*100)
	}

	// Performance requirements validation
	if results.P95Latency > 50*time.Millisecond {
		t.Errorf("P95 latency %v exceeds 50ms requirement", results.P95Latency)
	}

	if results.RequestsPerSecond < float64(scenario.RequestsPerSecond)*0.8 {
		t.Errorf("Actual RPS %.2f is less than 80%% of target %d", 
			results.RequestsPerSecond, scenario.RequestsPerSecond)
	}

	t.Logf("Load test completed: %s", results.Grade)
	t.Logf("RPS: %.2f, P95 Latency: %v, Error Rate: %.2f%%", 
		results.RequestsPerSecond, results.P95Latency, results.ErrorRate*100)
}

// TestHighFrequencyRequests tests handling of high-frequency requests
func TestHighFrequencyRequests(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	scenario := &LoadTestScenario{
		Name:              "high_frequency_requests",
		WorkspaceCount:    3,
		RequestsPerSecond: 500,
		Duration:          1 * time.Minute,
		Languages:         []string{"go"},
		ConcurrentUsers:   50,
		RampUpDuration:    10 * time.Second,
		RampDownDuration:  5 * time.Second,
		RequestPattern:    PatternBurst,
		FailureThreshold:  0.10, // Allow higher failure rate for stress test
	}

	results := runLoadTestScenario(t, scenario)
	
	// Stress test validation - focus on stability rather than perfection
	if results.ErrorRate > scenario.FailureThreshold {
		t.Errorf("Error rate %.2f%% exceeds stress test threshold %.2f%%", 
			results.ErrorRate*100, scenario.FailureThreshold*100)
	}

	// Check that system didn't completely fail
	if results.SuccessfulRequests == 0 {
		t.Fatalf("No successful requests - system completely failed")
	}

	t.Logf("High frequency test: %d successful requests out of %d total", 
		results.SuccessfulRequests, results.TotalRequests)
}

// TestWorkspaceResourceContention tests resource contention between workspaces
func TestWorkspaceResourceContention(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	scenario := &LoadTestScenario{
		Name:              "resource_contention",
		WorkspaceCount:    8,
		RequestsPerSecond: 200,
		Duration:          90 * time.Second,
		Languages:         []string{"go", "python", "javascript"},
		ConcurrentUsers:   40,
		RampUpDuration:    20 * time.Second,
		RampDownDuration:  10 * time.Second,
		RequestPattern:    PatternUniform,
		FailureThreshold:  0.08,
	}

	results := runLoadTestScenario(t, scenario)
	
	// Analyze resource contention
	memoryPerWorkspace := results.PeakMemoryUsage / int64(scenario.WorkspaceCount)
	maxMemoryPerWorkspace := int64(150 * 1024 * 1024) // 150MB per workspace under contention
	
	if memoryPerWorkspace > maxMemoryPerWorkspace {
		t.Errorf("Memory per workspace %d MB exceeds limit %d MB under contention", 
			memoryPerWorkspace/(1024*1024), maxMemoryPerWorkspace/(1024*1024))
	}

	// Check fairness - no workspace should be starved
	for wsName, wsMetrics := range results.WorkspaceMetrics {
		if wsMetrics.RequestCount == 0 {
			t.Errorf("Workspace %s received no requests - possible starvation", wsName)
		}
		
		expectedMinRequests := results.TotalRequests / int64(scenario.WorkspaceCount) / 10 // At least 10%
		if wsMetrics.RequestCount < expectedMinRequests {
			t.Errorf("Workspace %s received too few requests (%d) - possible contention issue", 
				wsName, wsMetrics.RequestCount)
		}
	}

	t.Logf("Resource contention test completed: %d workspaces, %.2f MB memory per workspace", 
		scenario.WorkspaceCount, float64(memoryPerWorkspace)/(1024*1024))
}

// TestPortExhaustionRecovery tests port exhaustion and recovery scenarios
func TestPortExhaustionRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	portManager, err := NewWorkspacePortManager()
	if err != nil {
		t.Fatalf("Failed to create port manager: %v", err)
	}

	// Test port allocation under pressure
	const maxWorkspaces = MaxConcurrentPorts // Use all available ports
	workspaceRoots := make([]string, maxWorkspaces)
	allocatedPorts := make([]int, maxWorkspaces)
	
	// Allocate all available ports
	for i := 0; i < maxWorkspaces; i++ {
		workspaceRoot := fmt.Sprintf("/tmp/port-exhaustion-test-%d", i)
		workspaceRoots[i] = workspaceRoot
		
		port, err := portManager.AllocatePort(workspaceRoot)
		if err != nil {
			t.Fatalf("Failed to allocate port %d: %v", i, err)
		}
		allocatedPorts[i] = port
	}

	// Try to allocate one more - should fail
	extraWorkspace := "/tmp/port-exhaustion-extra"
	_, err = portManager.AllocatePort(extraWorkspace)
	if err == nil {
		t.Errorf("Expected port allocation to fail when exhausted, but it succeeded")
	}

	// Release half the ports
	for i := 0; i < maxWorkspaces/2; i++ {
		err := portManager.ReleasePort(workspaceRoots[i])
		if err != nil {
			t.Errorf("Failed to release port for workspace %d: %v", i, err)
		}
	}

	// Should be able to allocate again
	_, err = portManager.AllocatePort(extraWorkspace)
	if err != nil {
		t.Errorf("Expected port allocation to succeed after releasing ports, but it failed: %v", err)
	}

	// Cleanup
	portManager.ReleasePort(extraWorkspace)
	for i := maxWorkspaces/2; i < maxWorkspaces; i++ {
		portManager.ReleasePort(workspaceRoots[i])
	}

	t.Logf("Port exhaustion recovery test completed: handled %d concurrent ports", maxWorkspaces)
}

// TestLongRunningWorkspaces tests workspace stability over extended periods
func TestLongRunningWorkspaces(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running test in short mode")
	}

	scenario := &LoadTestScenario{
		Name:              "long_running_workspaces",
		WorkspaceCount:    3,
		RequestsPerSecond: 50,
		Duration:          5 * time.Minute, // Extended duration
		Languages:         []string{"go", "python"},
		ConcurrentUsers:   10,
		RampUpDuration:    30 * time.Second,
		RampDownDuration:  30 * time.Second,
		RequestPattern:    PatternGradual,
		FailureThreshold:  0.02, // Very low failure rate for stability test
	}

	results := runLoadTestScenario(t, scenario)
	
	// Check for stability - error rate should remain low throughout
	if results.ErrorRate > scenario.FailureThreshold {
		t.Errorf("Long-running stability test failed: error rate %.2f%%", results.ErrorRate*100)
	}

	// Check for memory leaks - analyze time series data
	if len(results.TimeSeriesData) > 10 {
		startMemory := results.TimeSeriesData[5].MemoryUsage // Skip initial ramp-up
		endMemory := results.TimeSeriesData[len(results.TimeSeriesData)-5].MemoryUsage // Skip ramp-down
		
		memoryGrowth := float64(endMemory-startMemory) / float64(startMemory)
		if memoryGrowth > 0.5 { // 50% memory growth indicates potential leak
			t.Errorf("Potential memory leak detected: memory grew by %.2f%% during test", memoryGrowth*100)
		}
	}

	// Check latency stability - shouldn't degrade significantly over time
	if len(results.TimeSeriesData) > 10 {
		startLatency := results.TimeSeriesData[5].AverageLatency
		endLatency := results.TimeSeriesData[len(results.TimeSeriesData)-5].AverageLatency
		
		latencyDegradation := float64(endLatency-startLatency) / float64(startLatency)
		if latencyDegradation > 1.0 { // 100% latency increase indicates performance degradation
			t.Errorf("Performance degradation detected: latency increased by %.2f%%", latencyDegradation*100)
		}
	}

	t.Logf("Long-running test completed: %.2f%% error rate, stable performance", results.ErrorRate*100)
}

// RunLoadTestScenario executes a specific load test scenario
func RunLoadTestScenario(scenario *LoadTestScenario) (*LoadTestResults, error) {
	testSuite := NewWorkspaceTestSuite()
	defer testSuite.Cleanup()

	return executeLoadTest(testSuite, scenario)
}

// Helper functions

func runLoadTestScenario(t *testing.T, scenario *LoadTestScenario) *LoadTestResults {
	testSuite := NewWorkspaceTestSuite()
	defer testSuite.Cleanup()

	results, err := executeLoadTest(testSuite, scenario)
	if err != nil {
		t.Fatalf("Load test execution failed: %v", err)
	}

	return results
}

func executeLoadTest(testSuite *WorkspaceTestSuite, scenario *LoadTestScenario) (*LoadTestResults, error) {
	metricsCollector := NewMetricsCollector()
	metricsCollector.StartCollection()
	defer metricsCollector.StopCollection()

	results := &LoadTestResults{
		Scenario:         scenario,
		StartTime:        time.Now(),
		WorkspaceMetrics: make(map[string]*LoadTestWSMetrics),
		TimeSeriesData:   make([]*LoadTestDataPoint, 0),
		Errors:           make([]LoadTestError, 0),
	}

	// Setup workspaces
	workspaceNames := make([]string, scenario.WorkspaceCount)
	for i := 0; i < scenario.WorkspaceCount; i++ {
		workspaceName := fmt.Sprintf("%s-ws-%d", scenario.Name, i)
		workspaceNames[i] = workspaceName
		
		err := testSuite.SetupWorkspace(workspaceName, scenario.Languages)
		if err != nil {
			return nil, fmt.Errorf("failed to setup workspace %s: %w", workspaceName, err)
		}
		
		err = testSuite.StartWorkspace(workspaceName)
		if err != nil {
			return nil, fmt.Errorf("failed to start workspace %s: %w", workspaceName, err)
		}
		
		err = testSuite.WaitForWorkspaceReady(workspaceName, 30*time.Second)
		if err != nil {
			return nil, fmt.Errorf("workspace %s not ready: %w", workspaceName, err)
		}

		// Initialize workspace metrics
		results.WorkspaceMetrics[workspaceName] = &LoadTestWSMetrics{
			WorkspaceName: workspaceName,
		}
	}

	// Initialize counters
	var (
		totalRequests      atomic.Int64
		successfulRequests atomic.Int64
		failedRequests     atomic.Int64
		latencySum         atomic.Int64
		minLatency         atomic.Int64
		maxLatency         atomic.Int64
	)
	
	latencies := make([]time.Duration, 0, 10000)
	var latencyMu sync.Mutex

	// Time series data collection
	dataPointTicker := time.NewTicker(5 * time.Second)
	defer dataPointTicker.Stop()
	
	ctx, cancel := context.WithTimeout(context.Background(), 
		scenario.RampUpDuration+scenario.Duration+scenario.RampDownDuration)
	defer cancel()

	// Background metrics collection
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-dataPointTicker.C:
				dataPoint := collectDataPoint(metricsCollector, totalRequests.Load(), 
					successfulRequests.Load(), latencies, latencyMu)
				results.TimeSeriesData = append(results.TimeSeriesData, dataPoint)
			}
		}
	}()

	// Execute load test phases
	err := executeLoadTestPhases(ctx, testSuite, scenario, workspaceNames, 
		&totalRequests, &successfulRequests, &failedRequests, 
		&latencySum, &minLatency, &maxLatency, &latencies, &latencyMu,
		metricsCollector, results)
	
	if err != nil {
		return nil, fmt.Errorf("load test execution failed: %w", err)
	}

	// Calculate final results
	results.EndTime = time.Now()
	results.Duration = results.EndTime.Sub(results.StartTime)
	results.TotalRequests = totalRequests.Load()
	results.SuccessfulRequests = successfulRequests.Load()
	results.FailedRequests = failedRequests.Load()
	results.RequestsPerSecond = float64(results.TotalRequests) / results.Duration.Seconds()
	
	if results.TotalRequests > 0 {
		results.ErrorRate = float64(results.FailedRequests) / float64(results.TotalRequests)
		results.AverageLatency = time.Duration(latencySum.Load() / results.TotalRequests)
	}

	// Calculate latency percentiles
	if len(latencies) > 0 {
		sortedLatencies := make([]time.Duration, len(latencies))
		copy(sortedLatencies, latencies)
		sortDurations(sortedLatencies)
		
		results.MedianLatency = calculatePercentile(sortedLatencies, 0.50)
		results.P95Latency = calculatePercentile(sortedLatencies, 0.95)
		results.P99Latency = calculatePercentile(sortedLatencies, 0.99)
		results.MinLatency = sortedLatencies[0]
		results.MaxLatency = sortedLatencies[len(sortedLatencies)-1]
	}

	// Get peak memory usage
	systemMetrics := metricsCollector.GetSystemMetrics()
	results.PeakMemoryUsage = systemMetrics.MemoryUsage
	results.CPUUsagePercent = systemMetrics.CPUUsage

	// Calculate per-workspace metrics
	for _, workspaceName := range workspaceNames {
		if wsMetrics, exists := results.WorkspaceMetrics[workspaceName]; exists {
			if wsMetrics.RequestCount > 0 {
				wsMetrics.ThroughputRPS = float64(wsMetrics.RequestCount) / results.Duration.Seconds()
			}
		}
	}

	// Grade the results
	results.Grade = calculateLoadTestGrade(results, scenario)
	results.Passed = results.Grade != "F" && results.ErrorRate <= scenario.FailureThreshold

	return results, nil
}

func executeLoadTestPhases(ctx context.Context, testSuite *WorkspaceTestSuite, 
	scenario *LoadTestScenario, workspaceNames []string,
	totalRequests, successfulRequests, failedRequests, latencySum, minLatency, maxLatency *atomic.Int64,
	latencies *[]time.Duration, latencyMu *sync.Mutex,
	metricsCollector *MetricsCollector, results *LoadTestResults) error {

	// Phase 1: Ramp-up
	if scenario.RampUpDuration > 0 {
		err := executeRampUpPhase(ctx, testSuite, scenario, workspaceNames,
			totalRequests, successfulRequests, failedRequests, latencySum, minLatency, maxLatency,
			latencies, latencyMu, metricsCollector, results)
		if err != nil {
			return fmt.Errorf("ramp-up phase failed: %w", err)
		}
	}

	// Phase 2: Sustained load
	err := executeSustainedLoadPhase(ctx, testSuite, scenario, workspaceNames,
		totalRequests, successfulRequests, failedRequests, latencySum, minLatency, maxLatency,
		latencies, latencyMu, metricsCollector, results)
	if err != nil {
		return fmt.Errorf("sustained load phase failed: %w", err)
	}

	// Phase 3: Ramp-down
	if scenario.RampDownDuration > 0 {
		err := executeRampDownPhase(ctx, testSuite, scenario, workspaceNames,
			totalRequests, successfulRequests, failedRequests, latencySum, minLatency, maxLatency,
			latencies, latencyMu, metricsCollector, results)
		if err != nil {
			return fmt.Errorf("ramp-down phase failed: %w", err)
		}
	}

	return nil
}

func executeRampUpPhase(ctx context.Context, testSuite *WorkspaceTestSuite,
	scenario *LoadTestScenario, workspaceNames []string,
	totalRequests, successfulRequests, failedRequests, latencySum, minLatency, maxLatency *atomic.Int64,
	latencies *[]time.Duration, latencyMu *sync.Mutex,
	metricsCollector *MetricsCollector, results *LoadTestResults) error {

	rampCtx, rampCancel := context.WithTimeout(ctx, scenario.RampUpDuration)
	defer rampCancel()

	// Gradually increase load
	maxUsers := scenario.ConcurrentUsers
	increment := maxUsers / 5 // 5 steps in ramp-up
	if increment < 1 {
		increment = 1
	}

	for users := increment; users <= maxUsers; users += increment {
		stepCtx, stepCancel := context.WithTimeout(rampCtx, scenario.RampUpDuration/5)
		
		executeLoadStep(stepCtx, testSuite, scenario, workspaceNames, users,
			totalRequests, successfulRequests, failedRequests, latencySum, minLatency, maxLatency,
			latencies, latencyMu, metricsCollector, results)
		
		stepCancel()
		
		select {
		case <-rampCtx.Done():
			return nil
		default:
		}
	}

	return nil
}

func executeSustainedLoadPhase(ctx context.Context, testSuite *WorkspaceTestSuite,
	scenario *LoadTestScenario, workspaceNames []string,
	totalRequests, successfulRequests, failedRequests, latencySum, minLatency, maxLatency *atomic.Int64,
	latencies *[]time.Duration, latencyMu *sync.Mutex,
	metricsCollector *MetricsCollector, results *LoadTestResults) error {

	loadCtx, loadCancel := context.WithTimeout(ctx, scenario.Duration)
	defer loadCancel()

	executeLoadStep(loadCtx, testSuite, scenario, workspaceNames, scenario.ConcurrentUsers,
		totalRequests, successfulRequests, failedRequests, latencySum, minLatency, maxLatency,
		latencies, latencyMu, metricsCollector, results)

	return nil
}

func executeRampDownPhase(ctx context.Context, testSuite *WorkspaceTestSuite,
	scenario *LoadTestScenario, workspaceNames []string,
	totalRequests, successfulRequests, failedRequests, latencySum, minLatency, maxLatency *atomic.Int64,
	latencies *[]time.Duration, latencyMu *sync.Mutex,
	metricsCollector *MetricsCollector, results *LoadTestResults) error {

	rampCtx, rampCancel := context.WithTimeout(ctx, scenario.RampDownDuration)
	defer rampCancel()

	// Gradually decrease load
	maxUsers := scenario.ConcurrentUsers
	decrement := maxUsers / 3 // 3 steps in ramp-down
	if decrement < 1 {
		decrement = 1
	}

	for users := maxUsers - decrement; users > 0; users -= decrement {
		stepCtx, stepCancel := context.WithTimeout(rampCtx, scenario.RampDownDuration/3)
		
		executeLoadStep(stepCtx, testSuite, scenario, workspaceNames, users,
			totalRequests, successfulRequests, failedRequests, latencySum, minLatency, maxLatency,
			latencies, latencyMu, metricsCollector, results)
		
		stepCancel()
		
		select {
		case <-rampCtx.Done():
			return nil
		default:
		}
	}

	return nil
}

func executeLoadStep(ctx context.Context, testSuite *WorkspaceTestSuite,
	scenario *LoadTestScenario, workspaceNames []string, concurrentUsers int,
	totalRequests, successfulRequests, failedRequests, latencySum, minLatency, maxLatency *atomic.Int64,
	latencies *[]time.Duration, latencyMu *sync.Mutex,
	metricsCollector *MetricsCollector, results *LoadTestResults) {

	var wg sync.WaitGroup
	
	for i := 0; i < concurrentUsers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			workspaceIndex := workerID % len(workspaceNames)
			workspaceName := workspaceNames[workspaceIndex]
			
			for {
				select {
				case <-ctx.Done():
					return
				default:
					// Send request
					start := time.Now()
					_, err := testSuite.SendJSONRPCRequest(workspaceName, "workspace/symbol", 
						map[string]interface{}{"query": "test"})
					latency := time.Since(start)
					
					totalRequests.Add(1)
					metricsCollector.RecordRequestLatency(workspaceName, latency)
					
					// Update workspace metrics
					if wsMetrics, exists := results.WorkspaceMetrics[workspaceName]; exists {
						atomic.AddInt64(&wsMetrics.RequestCount, 1)
						if latency > time.Duration(atomic.LoadInt64((*int64)(&wsMetrics.MaxLatency))) {
							atomic.StoreInt64((*int64)(&wsMetrics.MaxLatency), int64(latency))
						}
					}
					
					if err != nil {
						failedRequests.Add(1)
						if wsMetrics, exists := results.WorkspaceMetrics[workspaceName]; exists {
							atomic.AddInt64(&wsMetrics.ErrorCount, 1)
						}
					} else {
						successfulRequests.Add(1)
						
						// Update global latency stats
						latencySum.Add(latency.Nanoseconds())
						
						// Update min/max latency
						for {
							currentMin := minLatency.Load()
							if currentMin == 0 || latency.Nanoseconds() < currentMin {
								if minLatency.CompareAndSwap(currentMin, latency.Nanoseconds()) {
									break
								}
							} else {
								break
							}
						}
						
						for {
							currentMax := maxLatency.Load()
							if latency.Nanoseconds() > currentMax {
								if maxLatency.CompareAndSwap(currentMax, latency.Nanoseconds()) {
									break
								}
							} else {
								break
							}
						}
						
						// Store latency for percentile calculation
						latencyMu.Lock()
						*latencies = append(*latencies, latency)
						latencyMu.Unlock()
					}
					
					// Apply request pattern delays
					applyRequestPatternDelay(scenario.RequestPattern)
				}
			}
		}(i)
	}
	
	wg.Wait()
}

func collectDataPoint(metricsCollector *MetricsCollector, totalRequests, successfulRequests int64,
	latencies []time.Duration, latencyMu sync.RWMutex) *LoadTestDataPoint {

	systemMetrics := metricsCollector.GetSystemMetrics()
	
	latencyMu.RLock()
	avgLatency := time.Duration(0)
	if len(latencies) > 0 {
		var sum int64
		for _, lat := range latencies {
			sum += lat.Nanoseconds()
		}
		avgLatency = time.Duration(sum / int64(len(latencies)))
	}
	latencyMu.RUnlock()
	
	errorRate := 0.0
	if totalRequests > 0 {
		errorRate = float64(totalRequests-successfulRequests) / float64(totalRequests)
	}

	return &LoadTestDataPoint{
		Timestamp:         time.Now(),
		RequestsPerSecond: float64(totalRequests) / time.Since(time.Now().Add(-5*time.Second)).Seconds(),
		AverageLatency:    avgLatency,
		ErrorRate:         errorRate,
		MemoryUsage:       systemMetrics.MemoryUsage,
		ActiveConnections: systemMetrics.NetworkConns,
	}
}

func applyRequestPatternDelay(pattern RequestPattern) {
	switch pattern {
	case PatternUniform:
		time.Sleep(10 * time.Millisecond)
	case PatternBurst:
		// Send bursts with pauses
		if time.Now().UnixNano()%1000 < 200 { // 20% of time
			time.Sleep(1 * time.Millisecond) // Fast
		} else {
			time.Sleep(50 * time.Millisecond) // Slow
		}
	case PatternGradual:
		time.Sleep(25 * time.Millisecond)
	case PatternSpike:
		// Occasional spikes
		if time.Now().UnixNano()%1000 < 50 { // 5% of time
			// No delay for spike
		} else {
			time.Sleep(20 * time.Millisecond)
		}
	}
}

func calculateLoadTestGrade(results *LoadTestResults, scenario *LoadTestScenario) string {
	score := 100.0

	// Deduct points for errors
	if results.ErrorRate > scenario.FailureThreshold {
		score -= (results.ErrorRate - scenario.FailureThreshold) * 200 // Heavy penalty for exceeding threshold
	}

	// Deduct points for high latency
	if results.P95Latency > 50*time.Millisecond {
		score -= 20
	}

	// Deduct points for low throughput
	expectedRPS := float64(scenario.RequestsPerSecond) * 0.8 // Allow 20% variance
	if results.RequestsPerSecond < expectedRPS {
		score -= 25
	}

	// Performance grading
	switch {
	case score >= 90:
		return "A"
	case score >= 80:
		return "B"
	case score >= 70:
		return "C"
	case score >= 60:
		return "D"
	default:
		return "F"
	}
}

