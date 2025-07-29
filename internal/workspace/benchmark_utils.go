package workspace

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// WorkspaceMetrics contains comprehensive performance metrics for a workspace
type WorkspaceMetrics struct {
	WorkspaceID        string            `json:"workspace_id"`
	WorkspaceRoot      string            `json:"workspace_root"`
	StartupTime        time.Duration     `json:"startup_time"`
	MemoryUsage        int64             `json:"memory_usage_bytes"`
	ActiveConnections  int               `json:"active_connections"`
	RequestLatencies   []time.Duration   `json:"request_latencies"`
	CacheHitRatio      float64           `json:"cache_hit_ratio"`
	ErrorRate          float64           `json:"error_rate"`
	ThroughputRPS      float64           `json:"throughput_rps"`
	LastUpdated        time.Time         `json:"last_updated"`
	LanguageBreakdown  map[string]int64  `json:"language_breakdown"`
	PortNumber         int               `json:"port_number"`
	ConfigLoadTime     time.Duration     `json:"config_load_time"`
	HealthScore        float64           `json:"health_score"`
}

// BenchmarkResults contains results from performance benchmarking
type BenchmarkResults struct {
	TestName           string                     `json:"test_name"`
	StartTime          time.Time                  `json:"start_time"`
	Duration           time.Duration              `json:"duration"`
	WorkspaceCount     int                        `json:"workspace_count"`
	ConcurrentUsers    int                        `json:"concurrent_users"`
	TotalRequests      int64                      `json:"total_requests"`
	SuccessfulRequests int64                      `json:"successful_requests"`
	FailedRequests     int64                      `json:"failed_requests"`
	AverageLatency     time.Duration              `json:"average_latency"`
	MinLatency         time.Duration              `json:"min_latency"`
	MaxLatency         time.Duration              `json:"max_latency"`
	P95Latency         time.Duration              `json:"p95_latency"`
	P99Latency         time.Duration              `json:"p99_latency"`
	ThroughputRPS      float64                    `json:"throughput_rps"`
	ErrorRate          float64                    `json:"error_rate"`
	MemoryUsed         int64                      `json:"memory_used_bytes"`
	CPUUsage           float64                    `json:"cpu_usage_percent"`
	Errors             []string                   `json:"errors,omitempty"`
	WorkspaceMetrics   map[string]*WorkspaceMetrics `json:"workspace_metrics"`
	PerformanceGrade   string                     `json:"performance_grade"`
	Recommendations    []string                   `json:"recommendations"`
}

// PerformanceBenchmark provides comprehensive workspace performance benchmarking
type PerformanceBenchmark struct {
	testSuite       *WorkspaceTestSuite
	metricsCollector *MetricsCollector
	config          *BenchmarkConfig
	results         map[string]*BenchmarkResults
	mu              sync.RWMutex
	isRunning       atomic.Bool
}

// BenchmarkConfig configures benchmark behavior
type BenchmarkConfig struct {
	WorkspaceCount      int           `json:"workspace_count"`
	ConcurrentUsers     int           `json:"concurrent_users"`  
	TestDuration        time.Duration `json:"test_duration"`
	WarmupDuration      time.Duration `json:"warmup_duration"`
	Languages           []string      `json:"languages"`
	EnableCaching       bool          `json:"enable_caching"`
	EnableMetrics       bool          `json:"enable_metrics"`
	SampleInterval      time.Duration `json:"sample_interval"`
	StressTestEnabled   bool          `json:"stress_test_enabled"`
	MemoryLimitMB       int64         `json:"memory_limit_mb"`
	RequestTimeoutMs    int           `json:"request_timeout_ms"`
	FailureThreshold    float64       `json:"failure_threshold"`
}

// DefaultBenchmarkConfig returns optimized default benchmark configuration
func DefaultBenchmarkConfig() *BenchmarkConfig {
	return &BenchmarkConfig{
		WorkspaceCount:      5,
		ConcurrentUsers:     10,
		TestDuration:        2 * time.Minute,
		WarmupDuration:      30 * time.Second,
		Languages:           []string{"go", "python", "javascript"},
		EnableCaching:       true,
		EnableMetrics:       true,
		SampleInterval:      1 * time.Second,
		StressTestEnabled:   false,
		MemoryLimitMB:       500,
		RequestTimeoutMs:    5000,
		FailureThreshold:    0.05, // 5% failure rate threshold
	}
}

// NewPerformanceBenchmark creates a new performance benchmark instance
func NewPerformanceBenchmark() *PerformanceBenchmark {
	return NewPerformanceBenchmarkWithConfig(DefaultBenchmarkConfig())
}

// NewPerformanceBenchmarkWithConfig creates a benchmark with custom configuration
func NewPerformanceBenchmarkWithConfig(config *BenchmarkConfig) *PerformanceBenchmark {
	testSuite := NewWorkspaceTestSuite()
	metricsCollector := NewMetricsCollector()
	
	return &PerformanceBenchmark{
		testSuite:        testSuite,
		metricsCollector: metricsCollector,
		config:           config,
		results:          make(map[string]*BenchmarkResults),
	}
}

// RunStartupBenchmark measures workspace startup performance
func (pb *PerformanceBenchmark) RunStartupBenchmark() *BenchmarkResults {
	if !pb.isRunning.CompareAndSwap(false, true) {
		return &BenchmarkResults{
			TestName: "startup_benchmark",
			Errors:   []string{"benchmark already running"},
		}
	}
	defer pb.isRunning.Store(false)

	pb.metricsCollector.StartCollection()
	defer pb.metricsCollector.StopCollection()

	result := &BenchmarkResults{
		TestName:         "startup_benchmark",
		StartTime:        time.Now(),
		WorkspaceCount:   pb.config.WorkspaceCount,
		WorkspaceMetrics: make(map[string]*WorkspaceMetrics),
	}

	// Capture initial memory
	runtime.GC()
	var initialMem runtime.MemStats
	runtime.ReadMemStats(&initialMem)

	startupTimes := make([]time.Duration, 0, pb.config.WorkspaceCount)
	
	// Benchmark workspace startup
	for i := 0; i < pb.config.WorkspaceCount; i++ {
		workspaceName := fmt.Sprintf("startup-bench-%d", i)
		
		startupStart := time.Now()
		
		// Setup workspace
		err := pb.testSuite.SetupWorkspace(workspaceName, pb.config.Languages)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("setup failed for %s: %v", workspaceName, err))
			result.FailedRequests++
			continue
		}
		
		// Start workspace
		err = pb.testSuite.StartWorkspace(workspaceName)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("start failed for %s: %v", workspaceName, err))
			result.FailedRequests++
			continue
		}
		
		// Wait for ready state
		err = pb.testSuite.WaitForWorkspaceReady(workspaceName, 30*time.Second)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("ready timeout for %s: %v", workspaceName, err))
			result.FailedRequests++
			continue
		}
		
		startupDuration := time.Since(startupStart)
		startupTimes = append(startupTimes, startupDuration)
		
		// Record metrics
		pb.metricsCollector.RecordWorkspaceStartup(workspaceName, startupDuration)
		result.SuccessfulRequests++
		
		// Collect workspace metrics
		wsMetrics := pb.collectWorkspaceMetrics(workspaceName, startupDuration)
		result.WorkspaceMetrics[workspaceName] = wsMetrics
	}

	// Calculate statistics
	result.Duration = time.Since(result.StartTime)
	result.TotalRequests = int64(pb.config.WorkspaceCount)
	
	if len(startupTimes) > 0 {
		result.AverageLatency = pb.calculateAverage(startupTimes)
		result.MinLatency = pb.calculateMin(startupTimes)
		result.MaxLatency = pb.calculateMax(startupTimes)
		result.P95Latency = pb.calculatePercentile(startupTimes, 0.95)
		result.P99Latency = pb.calculatePercentile(startupTimes, 0.99)
	}

	// Memory usage
	runtime.GC()
	var finalMem runtime.MemStats
	runtime.ReadMemStats(&finalMem)
	result.MemoryUsed = int64(finalMem.Alloc - initialMem.Alloc)

	// Performance grading
	result.PerformanceGrade = pb.calculatePerformanceGrade(result)
	result.Recommendations = pb.generateRecommendations(result)

	pb.mu.Lock()
	pb.results["startup_benchmark"] = result
	pb.mu.Unlock()

	return result
}

// RunThroughputBenchmark measures workspace request throughput
func (pb *PerformanceBenchmark) RunThroughputBenchmark() *BenchmarkResults {
	if !pb.isRunning.CompareAndSwap(false, true) {
		return &BenchmarkResults{
			TestName: "throughput_benchmark",
			Errors:   []string{"benchmark already running"},
		}
	}
	defer pb.isRunning.Store(false)

	pb.metricsCollector.StartCollection()
	defer pb.metricsCollector.StopCollection()

	result := &BenchmarkResults{
		TestName:         "throughput_benchmark",
		StartTime:        time.Now(),
		WorkspaceCount:   pb.config.WorkspaceCount,
		ConcurrentUsers:  pb.config.ConcurrentUsers,
		WorkspaceMetrics: make(map[string]*WorkspaceMetrics),
	}

	// Setup workspaces
	workspaceNames := make([]string, pb.config.WorkspaceCount)
	for i := 0; i < pb.config.WorkspaceCount; i++ {
		workspaceName := fmt.Sprintf("throughput-bench-%d", i)
		workspaceNames[i] = workspaceName
		
		err := pb.testSuite.SetupWorkspace(workspaceName, pb.config.Languages)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("setup failed: %v", err))
			continue
		}
		
		err = pb.testSuite.StartWorkspace(workspaceName)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("start failed: %v", err))
			continue
		}
		
		err = pb.testSuite.WaitForWorkspaceReady(workspaceName, 30*time.Second)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("ready failed: %v", err))
			continue
		}
	}

	// Warmup period
	if pb.config.WarmupDuration > 0 {
		pb.runWarmup(workspaceNames)
	}

	// Run throughput test
	latencies := make([]time.Duration, 0, 1000)
	var latencyMu sync.Mutex
	var requestCount atomic.Int64
	var errorCount atomic.Int64

	ctx, cancel := context.WithTimeout(context.Background(), pb.config.TestDuration)
	defer cancel()

	var wg sync.WaitGroup
	
	// Start concurrent workers
	for i := 0; i < pb.config.ConcurrentUsers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			for {
				select {
				case <-ctx.Done():
					return
				default:
					// Round-robin through workspaces
					workspaceName := workspaceNames[int(requestCount.Load())%len(workspaceNames)]
					
					start := time.Now()
					_, err := pb.testSuite.SendJSONRPCRequest(workspaceName, "workspace/symbol", map[string]interface{}{
						"query": "test",
					})
					latency := time.Since(start)
					
					requestCount.Add(1)
					pb.metricsCollector.RecordRequestLatency(workspaceName, latency)
					
					if err != nil {
						errorCount.Add(1)
					} else {
						latencyMu.Lock()
						latencies = append(latencies, latency)
						latencyMu.Unlock()
					}
				}
			}
		}()
	}

	wg.Wait()

	// Calculate results
	result.Duration = time.Since(result.StartTime)
	result.TotalRequests = requestCount.Load()
	result.FailedRequests = errorCount.Load()
	result.SuccessfulRequests = result.TotalRequests - result.FailedRequests
	result.ErrorRate = float64(result.FailedRequests) / float64(result.TotalRequests)
	result.ThroughputRPS = float64(result.TotalRequests) / result.Duration.Seconds()

	if len(latencies) > 0 {
		result.AverageLatency = pb.calculateAverage(latencies)
		result.MinLatency = pb.calculateMin(latencies)
		result.MaxLatency = pb.calculateMax(latencies)
		result.P95Latency = pb.calculatePercentile(latencies, 0.95)
		result.P99Latency = pb.calculatePercentile(latencies, 0.99)
	}

	// Collect workspace metrics
	for _, workspaceName := range workspaceNames {
		wsMetrics := pb.collectWorkspaceMetrics(workspaceName, 0)
		result.WorkspaceMetrics[workspaceName] = wsMetrics
	}

	result.PerformanceGrade = pb.calculatePerformanceGrade(result)
	result.Recommendations = pb.generateRecommendations(result)

	pb.mu.Lock()
	pb.results["throughput_benchmark"] = result
	pb.mu.Unlock()

	return result
}

// RunMemoryBenchmark measures memory usage patterns
func (pb *PerformanceBenchmark) RunMemoryBenchmark() *BenchmarkResults {
	if !pb.isRunning.CompareAndSwap(false, true) {
		return &BenchmarkResults{
			TestName: "memory_benchmark",
			Errors:   []string{"benchmark already running"},
		}
	}
	defer pb.isRunning.Store(false)

	result := &BenchmarkResults{
		TestName:         "memory_benchmark",
		StartTime:        time.Now(),
		WorkspaceCount:   pb.config.WorkspaceCount,
		WorkspaceMetrics: make(map[string]*WorkspaceMetrics),
	}

	// Baseline memory measurement
	runtime.GC()
	var baseMem runtime.MemStats
	runtime.ReadMemStats(&baseMem)

	workspaceNames := make([]string, pb.config.WorkspaceCount)
	memorySnapshots := make([]int64, 0, pb.config.WorkspaceCount)

	// Create workspaces and measure memory incrementally
	for i := 0; i < pb.config.WorkspaceCount; i++ {
		workspaceName := fmt.Sprintf("memory-bench-%d", i)
		workspaceNames[i] = workspaceName
		
		err := pb.testSuite.SetupWorkspace(workspaceName, pb.config.Languages)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("setup failed: %v", err))
			continue
		}
		
		err = pb.testSuite.StartWorkspace(workspaceName)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("start failed: %v", err))
			continue
		}

		// Force GC and measure memory
		runtime.GC()
		var currentMem runtime.MemStats
		runtime.ReadMemStats(&currentMem)
		
		memoryUsed := int64(currentMem.Alloc - baseMem.Alloc)
		memorySnapshots = append(memorySnapshots, memoryUsed)
		
		pb.metricsCollector.RecordMemoryUsage(workspaceName, memoryUsed)
		
		result.SuccessfulRequests++
	}

	// Calculate memory statistics
	if len(memorySnapshots) > 0 {
		result.MemoryUsed = memorySnapshots[len(memorySnapshots)-1] // Total memory used
		
		// Calculate memory per workspace
		avgMemoryPerWorkspace := result.MemoryUsed / int64(len(memorySnapshots))
		
		// Collect detailed workspace metrics
		for i, workspaceName := range workspaceNames {
			if i < len(memorySnapshots) {
				wsMetrics := pb.collectWorkspaceMetrics(workspaceName, 0)
				wsMetrics.MemoryUsage = memorySnapshots[i]
				result.WorkspaceMetrics[workspaceName] = wsMetrics
			}
		}

		// Memory efficiency analysis
		result.Recommendations = pb.generateMemoryRecommendations(avgMemoryPerWorkspace)
	}

	result.Duration = time.Since(result.StartTime)
	result.TotalRequests = int64(pb.config.WorkspaceCount)
	result.PerformanceGrade = pb.calculateMemoryGrade(result)

	pb.mu.Lock()
	pb.results["memory_benchmark"] = result
	pb.mu.Unlock()

	return result
}

// RunScalabilityTest measures how performance scales with workspace count
func (pb *PerformanceBenchmark) RunScalabilityTest() *BenchmarkResults {
	if !pb.isRunning.CompareAndSwap(false, true) {
		return &BenchmarkResults{
			TestName: "scalability_test",
			Errors:   []string{"benchmark already running"},
		}
	}
	defer pb.isRunning.Store(false)

	result := &BenchmarkResults{
		TestName:         "scalability_test",
		StartTime:        time.Now(),
		WorkspaceCount:   pb.config.WorkspaceCount,
		WorkspaceMetrics: make(map[string]*WorkspaceMetrics),
	}

	scalabilityData := make(map[int]time.Duration) // workspace count -> startup time
	
	// Test scaling from 1 to configured workspace count
	for wsCount := 1; wsCount <= pb.config.WorkspaceCount; wsCount++ {
		scaleStart := time.Now()
		
		workspaceName := fmt.Sprintf("scale-%d", wsCount)
		err := pb.testSuite.SetupWorkspace(workspaceName, pb.config.Languages)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("scale setup failed at %d: %v", wsCount, err))
			continue
		}
		
		err = pb.testSuite.StartWorkspace(workspaceName)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("scale start failed at %d: %v", wsCount, err))
			continue
		}
		
		scaleTime := time.Since(scaleStart)
		scalabilityData[wsCount] = scaleTime
		
		result.SuccessfulRequests++
		
		// Test isolation at each scale level
		err = pb.testSuite.ValidateWorkspaceIsolation()
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("isolation failed at %d workspaces: %v", wsCount, err))
		}
	}

	// Analyze scaling behavior
	if len(scalabilityData) > 1 {
		// Check for linear vs exponential scaling
		firstTime := scalabilityData[1]
		lastTime := scalabilityData[len(scalabilityData)]
		
		expectedLinearTime := time.Duration(int64(firstTime) * int64(len(scalabilityData)))
		actualRatio := float64(lastTime) / float64(expectedLinearTime)
		
		if actualRatio > 2.0 {
			result.Recommendations = append(result.Recommendations, 
				"Scaling is worse than linear - consider optimizing resource allocation")
		}
	}

	result.Duration = time.Since(result.StartTime)
	result.TotalRequests = int64(pb.config.WorkspaceCount)
	result.PerformanceGrade = pb.calculateScalabilityGrade(scalabilityData)

	pb.mu.Lock()
	pb.results["scalability_test"] = result
	pb.mu.Unlock()

	return result
}

// CollectMetrics gathers comprehensive performance metrics
func (pb *PerformanceBenchmark) CollectMetrics() *WorkspaceMetrics {
	systemMetrics := pb.metricsCollector.GetSystemMetrics()
	
	return &WorkspaceMetrics{
		LastUpdated:    time.Now(),
		MemoryUsage:    systemMetrics.MemoryUsage,
		HealthScore:    1.0, // Default healthy score
	}
}

// GenerateReport creates a comprehensive performance report
func (pb *PerformanceBenchmark) GenerateReport() string {
	pb.mu.RLock()
	defer pb.mu.RUnlock()

	report := "# Workspace Performance Benchmark Report\n\n"
	report += fmt.Sprintf("Generated: %s\n", time.Now().Format(time.RFC3339))
	report += fmt.Sprintf("Configuration: %d workspaces, %d concurrent users\n\n", 
		pb.config.WorkspaceCount, pb.config.ConcurrentUsers)

	for testName, result := range pb.results {
		report += fmt.Sprintf("## %s\n", testName)
		report += fmt.Sprintf("- Duration: %v\n", result.Duration)
		report += fmt.Sprintf("- Total Requests: %d\n", result.TotalRequests)
		report += fmt.Sprintf("- Success Rate: %.2f%%\n", 
			float64(result.SuccessfulRequests)/float64(result.TotalRequests)*100)
		
		if result.AverageLatency > 0 {
			report += fmt.Sprintf("- Average Latency: %v\n", result.AverageLatency)
			report += fmt.Sprintf("- P95 Latency: %v\n", result.P95Latency)
		}
		
		if result.ThroughputRPS > 0 {
			report += fmt.Sprintf("- Throughput: %.2f RPS\n", result.ThroughputRPS)
		}
		
		if result.MemoryUsed > 0 {
			report += fmt.Sprintf("- Memory Used: %d MB\n", result.MemoryUsed/(1024*1024))
		}
		
		report += fmt.Sprintf("- Performance Grade: %s\n", result.PerformanceGrade)
		
		if len(result.Errors) > 0 {
			report += fmt.Sprintf("- Errors: %d\n", len(result.Errors))
		}
		
		if len(result.Recommendations) > 0 {
			report += "- Recommendations:\n"
			for _, rec := range result.Recommendations {
				report += fmt.Sprintf("  - %s\n", rec)
			}
		}
		
		report += "\n"
	}

	return report
}

// Close cleans up benchmark resources
func (pb *PerformanceBenchmark) Close() error {
	if pb.testSuite != nil {
		pb.testSuite.Cleanup()
	}
	
	if pb.metricsCollector != nil {
		pb.metricsCollector.StopCollection()
	}
	
	return nil
}

// Helper methods

func (pb *PerformanceBenchmark) collectWorkspaceMetrics(workspaceName string, startupTime time.Duration) *WorkspaceMetrics {
	metrics := &WorkspaceMetrics{
		WorkspaceID:   workspaceName,
		StartupTime:   startupTime,
		LastUpdated:   time.Now(),
		HealthScore:   1.0,
	}

	// Get workspace metrics if available
	if wsMetrics, err := pb.testSuite.GetWorkspaceMetrics(workspaceName); err == nil {
		if httpMetrics, ok := wsMetrics["http_client"].(map[string]interface{}); ok {
			if totalReqs, ok := httpMetrics["total_requests"].(int64); ok {
				if avgLatency, ok := httpMetrics["average_latency"].(time.Duration); ok {
					if totalReqs > 0 {
						metrics.ThroughputRPS = float64(totalReqs) / time.Since(metrics.LastUpdated).Seconds()
					}
					if metrics.RequestLatencies == nil {
						metrics.RequestLatencies = make([]time.Duration, 0)
					}
					metrics.RequestLatencies = append(metrics.RequestLatencies, avgLatency)
				}
			}
		}
	}

	return metrics
}

func (pb *PerformanceBenchmark) runWarmup(workspaceNames []string) {
	ctx, cancel := context.WithTimeout(context.Background(), pb.config.WarmupDuration)
	defer cancel()

	var wg sync.WaitGroup
	for i := 0; i < pb.config.ConcurrentUsers/2; i++ { // Use half the users for warmup
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					workspaceName := workspaceNames[0] // Use first workspace for warmup
					pb.testSuite.SendJSONRPCRequest(workspaceName, "workspace/symbol", map[string]interface{}{
						"query": "warmup",
					})
					time.Sleep(100 * time.Millisecond)
				}
			}
		}()
	}
	wg.Wait()
}

func (pb *PerformanceBenchmark) calculateAverage(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	
	var total int64
	for _, d := range durations {
		total += d.Nanoseconds()
	}
	
	return time.Duration(total / int64(len(durations)))
}

func (pb *PerformanceBenchmark) calculateMin(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	
	min := durations[0]
	for _, d := range durations[1:] {
		if d < min {
			min = d
		}
	}
	
	return min
}

func (pb *PerformanceBenchmark) calculateMax(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	
	max := durations[0]
	for _, d := range durations[1:] {
		if d > max {
			max = d
		}
	}
	
	return max
}

func (pb *PerformanceBenchmark) calculatePercentile(durations []time.Duration, percentile float64) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	
	// Simple percentile calculation - sort and find index
	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)
	
	// Basic insertion sort for simplicity
	for i := 1; i < len(sorted); i++ {
		key := sorted[i]
		j := i - 1
		for j >= 0 && sorted[j] > key {
			sorted[j+1] = sorted[j]
			j--
		}
		sorted[j+1] = key
	}
	
	index := int(float64(len(sorted)-1) * percentile)
	return sorted[index]
}

func (pb *PerformanceBenchmark) calculatePerformanceGrade(result *BenchmarkResults) string {
	score := 100.0

	// Deduct points for errors
	if result.TotalRequests > 0 {
		errorRate := float64(result.FailedRequests) / float64(result.TotalRequests)
		score -= errorRate * 50 // Up to 50 points for errors
	}

	// Deduct points for high latency
	if result.P95Latency > 50*time.Millisecond {
		score -= 20 // 20 points for high latency
	}

	// Deduct points for low throughput
	if result.ThroughputRPS < 100 {
		score -= 15 // 15 points for low throughput
	}

	// Grade based on score
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

func (pb *PerformanceBenchmark) calculateMemoryGrade(result *BenchmarkResults) string {
	if result.WorkspaceCount == 0 {
		return "N/A"
	}

	avgMemoryPerWorkspace := result.MemoryUsed / int64(result.WorkspaceCount)
	maxMemoryPerWorkspace := int64(100 * 1024 * 1024) // 100MB

	if avgMemoryPerWorkspace <= maxMemoryPerWorkspace {
		return "A"
	} else if avgMemoryPerWorkspace <= maxMemoryPerWorkspace*2 {
		return "B"
	} else if avgMemoryPerWorkspace <= maxMemoryPerWorkspace*3 {
		return "C"
	} else {
		return "F"
	}
}

func (pb *PerformanceBenchmark) calculateScalabilityGrade(scalabilityData map[int]time.Duration) string {
	if len(scalabilityData) < 2 {
		return "N/A"
	}

	// Check if scaling is roughly linear
	firstTime := scalabilityData[1]
	lastCount := len(scalabilityData)
	lastTime := scalabilityData[lastCount]

	expectedLinearTime := time.Duration(int64(firstTime) * int64(lastCount))
	actualRatio := float64(lastTime) / float64(expectedLinearTime)

	switch {
	case actualRatio <= 1.2: // Within 20% of linear
		return "A"
	case actualRatio <= 1.5: // Within 50% of linear
		return "B"
	case actualRatio <= 2.0: // Within 100% of linear
		return "C"
	default:
		return "F"
	}
}

func (pb *PerformanceBenchmark) generateRecommendations(result *BenchmarkResults) []string {
	var recommendations []string

	// Error rate recommendations
	if result.TotalRequests > 0 {
		errorRate := float64(result.FailedRequests) / float64(result.TotalRequests)
		if errorRate > pb.config.FailureThreshold {
			recommendations = append(recommendations, 
				fmt.Sprintf("High error rate (%.2f%%) - investigate failed requests", errorRate*100))
		}
	}

	// Latency recommendations
	if result.P95Latency > 50*time.Millisecond {
		recommendations = append(recommendations, 
			"High P95 latency detected - consider optimizing request processing")
	}

	// Throughput recommendations
	if result.ThroughputRPS < 100 {
		recommendations = append(recommendations, 
			"Low throughput detected - consider connection pooling or request batching")
	}

	// Memory recommendations
	if result.MemoryUsed > pb.config.MemoryLimitMB*1024*1024 {
		recommendations = append(recommendations, 
			"Memory usage exceeds limit - investigate memory leaks or reduce cache sizes")
	}

	return recommendations
}

func (pb *PerformanceBenchmark) generateMemoryRecommendations(avgMemoryPerWorkspace int64) []string {
	var recommendations []string

	maxMemoryPerWorkspace := int64(100 * 1024 * 1024) // 100MB
	
	if avgMemoryPerWorkspace > maxMemoryPerWorkspace {
		recommendations = append(recommendations, 
			fmt.Sprintf("Average memory per workspace (%d MB) exceeds target (100 MB)", 
				avgMemoryPerWorkspace/(1024*1024)))
		
		recommendations = append(recommendations, 
			"Consider optimizing cache sizes or reducing memory footprint")
	}

	if avgMemoryPerWorkspace < maxMemoryPerWorkspace/2 {
		recommendations = append(recommendations, 
			"Memory usage is very efficient - consider increasing cache sizes for better performance")
	}

	return recommendations
}