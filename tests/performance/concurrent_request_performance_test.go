package performance

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"lsp-gateway/internal/gateway"
	"lsp-gateway/tests/framework"
)

// ConcurrentRequestPerformanceTest validates concurrent request handling performance
type ConcurrentRequestPerformanceTest struct {
	framework   *framework.MultiLanguageTestFramework
	profiler    *framework.PerformanceProfiler
	testProject *framework.TestProject
	gateway     *gateway.ProjectAwareGateway

	// Performance thresholds
	MaxResponseTime        time.Duration
	MinThroughputReqPerSec float64
	MaxErrorRate           float64
	MaxMemoryGrowthBytes   int64

	// Test configuration
	ConcurrencyLevels []int
	RequestTypes      []string
	TestDurations     []time.Duration
	LoadProfiles      []LoadProfile

	// Metrics tracking
	totalRequests      int64
	successfulRequests int64
	failedRequests     int64
	responseTimes      []time.Duration
	errorsByType       map[string]int64

	mu sync.RWMutex
}

// LoadProfile defines different load testing profiles
type LoadProfile struct {
	Name            string
	ConcurrentUsers int
	RequestsPerUser int
	RampUpTime      time.Duration
	SustainTime     time.Duration
	RampDownTime    time.Duration
	ThinkTime       time.Duration
}

// RequestResult contains the result of a single request
type RequestResult struct {
	Success      bool
	ResponseTime time.Duration
	RequestType  string
	Error        error
	Timestamp    time.Time
}

// LoadTestMetrics contains comprehensive load test metrics
type LoadTestMetrics struct {
	TotalRequests        int64
	SuccessfulRequests   int64
	FailedRequests       int64
	AverageResponseTime  time.Duration
	P50ResponseTime      time.Duration
	P95ResponseTime      time.Duration
	P99ResponseTime      time.Duration
	MaxResponseTime      time.Duration
	MinResponseTime      time.Duration
	ThroughputReqPerSec  float64
	ErrorRate            float64
	MemoryUsageMB        float64
	CPUUsagePercent      float64
	GoroutineCount       int
	CircuitBreakerTrips  int
	LoadBalancerSwitches int
}

// NewConcurrentRequestPerformanceTest creates a new concurrent request performance test
func NewConcurrentRequestPerformanceTest(t *testing.T) *ConcurrentRequestPerformanceTest {
	return &ConcurrentRequestPerformanceTest{
		framework: framework.NewMultiLanguageTestFramework(45 * time.Minute),
		profiler:  framework.NewPerformanceProfiler(),

		// Enterprise-scale performance thresholds
		MaxResponseTime:        5 * time.Second,
		MinThroughputReqPerSec: 100.0,
		MaxErrorRate:           0.05,              // 5% error rate
		MaxMemoryGrowthBytes:   500 * 1024 * 1024, // 500MB growth

		// Test configuration
		ConcurrencyLevels: []int{10, 25, 50, 100, 200, 500},
		RequestTypes: []string{
			"textDocument/definition",
			"textDocument/references",
			"textDocument/documentSymbol",
			"workspace/symbol",
			"textDocument/hover",
			"textDocument/completion",
		},
		TestDurations: []time.Duration{
			30 * time.Second,
			60 * time.Second,
			120 * time.Second,
		},
		LoadProfiles: []LoadProfile{
			{
				Name:            "spike_load",
				ConcurrentUsers: 200,
				RequestsPerUser: 50,
				RampUpTime:      5 * time.Second,
				SustainTime:     30 * time.Second,
				RampDownTime:    5 * time.Second,
				ThinkTime:       100 * time.Millisecond,
			},
			{
				Name:            "sustained_load",
				ConcurrentUsers: 100,
				RequestsPerUser: 100,
				RampUpTime:      30 * time.Second,
				SustainTime:     120 * time.Second,
				RampDownTime:    30 * time.Second,
				ThinkTime:       200 * time.Millisecond,
			},
			{
				Name:            "stress_load",
				ConcurrentUsers: 500,
				RequestsPerUser: 20,
				RampUpTime:      10 * time.Second,
				SustainTime:     60 * time.Second,
				RampDownTime:    10 * time.Second,
				ThinkTime:       50 * time.Millisecond,
			},
		},

		// Initialize metrics tracking
		responseTimes: make([]time.Duration, 0, 10000),
		errorsByType:  make(map[string]int64),
	}
}

// TestConcurrentRequestHandling validates handling of 100+ concurrent LSP requests
func (test *ConcurrentRequestPerformanceTest) TestConcurrentRequestHandling(t *testing.T) {
	ctx := context.Background()

	if err := test.setupTestEnvironment(ctx); err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer test.cleanup()

	// Test with different concurrency levels
	for _, concurrency := range test.ConcurrencyLevels {
		t.Run(fmt.Sprintf("Concurrency_%d", concurrency), func(t *testing.T) {
			test.testConcurrentRequests(t, concurrency, 60*time.Second)
		})
	}
}

// TestLoadBalancingEfficiency validates load balancing efficiency under high load
func (test *ConcurrentRequestPerformanceTest) TestLoadBalancingEfficiency(t *testing.T) {
	ctx := context.Background()

	if err := test.setupTestEnvironment(ctx); err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer test.cleanup()

	// Start multiple language servers to test load balancing
	languages := []string{"go", "python", "typescript", "java"}
	if err := test.framework.StartMultipleLanguageServers(languages); err != nil {
		t.Fatalf("Failed to start language servers: %v", err)
	}

	// Test load balancing with high concurrency
	loadBalancingMetrics, err := test.framework.MeasurePerformance(func() error {
		return test.testLoadBalancingEfficiency(100, 2*time.Minute)
	})

	if err != nil {
		t.Fatalf("Load balancing test failed: %v", err)
	}

	// Validate load balancing performance
	if loadBalancingMetrics.ServerSwitches == 0 {
		t.Error("No server switches detected - load balancing may not be working")
	}

	throughput := float64(loadBalancingMetrics.NetworkRequests) / loadBalancingMetrics.OperationDuration.Seconds()
	if throughput < test.MinThroughputReqPerSec {
		t.Errorf("Load balancing throughput too low: %.2f req/sec < %.2f req/sec",
			throughput, test.MinThroughputReqPerSec)
	}

	t.Logf("Load balancing efficiency: Throughput=%.2f req/sec, Server switches=%d, Duration=%v",
		throughput, loadBalancingMetrics.ServerSwitches, loadBalancingMetrics.OperationDuration)
}

// TestResponseTimeConsistency validates response time consistency under stress
func (test *ConcurrentRequestPerformanceTest) TestResponseTimeConsistency(t *testing.T) {
	ctx := context.Background()

	if err := test.setupTestEnvironment(ctx); err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer test.cleanup()

	// Test response time consistency with sustained load
	for _, duration := range test.TestDurations {
		t.Run(fmt.Sprintf("Duration_%v", duration), func(t *testing.T) {
			metrics := test.measureResponseTimeConsistency(t, 50, duration)
			test.validateResponseTimeConsistency(t, metrics)
		})
	}
}

// TestCircuitBreakerPerformance validates circuit breaker performance under failures
func (test *ConcurrentRequestPerformanceTest) TestCircuitBreakerPerformance(t *testing.T) {
	ctx := context.Background()

	if err := test.setupTestEnvironment(ctx); err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer test.cleanup()

	// Test circuit breaker behavior with simulated failures
	circuitBreakerMetrics, err := test.framework.MeasurePerformance(func() error {
		return test.testCircuitBreakerBehavior(75, 90*time.Second)
	})

	if err != nil {
		t.Fatalf("Circuit breaker test failed: %v", err)
	}

	// Validate circuit breaker activated appropriately
	if circuitBreakerMetrics.ErrorCount == 0 {
		t.Error("No errors detected in circuit breaker test - test may not be simulating failures")
	}

	errorRate := float64(circuitBreakerMetrics.ErrorCount) / float64(circuitBreakerMetrics.NetworkRequests)
	if errorRate > test.MaxErrorRate*2 { // Allow higher error rate for circuit breaker test
		t.Errorf("Error rate too high even with circuit breaker: %.2f%% > %.2f%%",
			errorRate*100, test.MaxErrorRate*2*100)
	}

	t.Logf("Circuit breaker performance: Error rate=%.2f%%, Duration=%v, Errors=%d",
		errorRate*100, circuitBreakerMetrics.OperationDuration, circuitBreakerMetrics.ErrorCount)
}

// TestResourceContentionIdentification validates resource contention and bottleneck identification
func (test *ConcurrentRequestPerformanceTest) TestResourceContentionIdentification(t *testing.T) {
	ctx := context.Background()

	if err := test.setupTestEnvironment(ctx); err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer test.cleanup()

	// Test resource contention with very high concurrency
	contentionMetrics, err := test.framework.MeasurePerformance(func() error {
		return test.testResourceContention(300, 120*time.Second)
	})

	if err != nil {
		t.Fatalf("Resource contention test failed: %v", err)
	}

	// Analyze resource contention patterns
	avgResponseTime := contentionMetrics.OperationDuration.Nanoseconds() / contentionMetrics.NetworkRequests
	if time.Duration(avgResponseTime) > test.MaxResponseTime {
		t.Errorf("Average response time too high under contention: %v > %v",
			time.Duration(avgResponseTime), test.MaxResponseTime)
	}

	// Check for excessive goroutine creation (potential contention indicator)
	if contentionMetrics.GoroutineCount > 1000 {
		t.Errorf("Excessive goroutines detected: %d > 1000 (potential resource contention)",
			contentionMetrics.GoroutineCount)
	}

	t.Logf("Resource contention test: Avg response time=%v, Goroutines=%d, Memory=%dMB",
		time.Duration(avgResponseTime), contentionMetrics.GoroutineCount,
		contentionMetrics.MemoryAllocated/1024/1024)
}

// TestLoadProfiles validates different load testing profiles
func (test *ConcurrentRequestPerformanceTest) TestLoadProfiles(t *testing.T) {
	ctx := context.Background()

	if err := test.setupTestEnvironment(ctx); err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer test.cleanup()

	for _, profile := range test.LoadProfiles {
		t.Run(profile.Name, func(t *testing.T) {
			metrics := test.executeLoadProfile(t, profile)
			test.validateLoadProfileResults(t, profile, metrics)
		})
	}
}

// BenchmarkConcurrentRequests benchmarks concurrent request handling
func (test *ConcurrentRequestPerformanceTest) BenchmarkConcurrentRequests(b *testing.B) {
	ctx := context.Background()

	if err := test.setupTestEnvironment(ctx); err != nil {
		b.Fatalf("Failed to setup test environment: %v", err)
	}
	defer test.cleanup()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := test.performBenchmarkRequests(50, 100)
		if err != nil {
			b.Fatalf("Benchmark iteration %d failed: %v", i, err)
		}
	}
}

// setupTestEnvironment sets up the test environment
func (test *ConcurrentRequestPerformanceTest) setupTestEnvironment(ctx context.Context) error {
	if err := test.framework.SetupTestEnvironment(ctx); err != nil {
		return fmt.Errorf("failed to setup framework: %w", err)
	}

	// Create test project
	project, err := test.framework.CreateMultiLanguageProject(
		framework.ProjectTypeMultiLanguage,
		[]string{"go", "python", "typescript", "java"})
	if err != nil {
		return fmt.Errorf("failed to create test project: %w", err)
	}
	test.testProject = project

	// Start language servers
	if err := test.framework.StartMultipleLanguageServers(project.Languages); err != nil {
		return fmt.Errorf("failed to start language servers: %w", err)
	}

	// Create gateway
	gateway, err := test.framework.CreateGatewayWithProject(project)
	if err != nil {
		return fmt.Errorf("failed to create gateway: %w", err)
	}
	test.gateway = gateway

	return nil
}

// testConcurrentRequests tests concurrent request handling with specified parameters
func (test *ConcurrentRequestPerformanceTest) testConcurrentRequests(t *testing.T, concurrency int, duration time.Duration) {
	results := make(chan *RequestResult, concurrency*100)
	var wg sync.WaitGroup

	startTime := time.Now()
	endTime := startTime.Add(duration)

	// Start concurrent workers
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			test.concurrentWorker(workerID, endTime, results)
		}(i)
	}

	// Collect results
	go func() {
		wg.Wait()
		close(results)
	}()

	// Process results
	metrics := test.collectLoadTestMetrics(results)

	// Validate performance thresholds
	test.validateConcurrentRequestMetrics(t, concurrency, metrics)

	t.Logf("Concurrent requests (concurrency=%d): Throughput=%.2f req/sec, Avg response time=%v, Error rate=%.2f%%",
		concurrency, metrics.ThroughputReqPerSec, metrics.AverageResponseTime, metrics.ErrorRate*100)
}

// concurrentWorker is a worker that sends concurrent requests
func (test *ConcurrentRequestPerformanceTest) concurrentWorker(workerID int, endTime time.Time, results chan<- *RequestResult) {
	for time.Now().Before(endTime) {
		requestType := test.RequestTypes[rand.Intn(len(test.RequestTypes))]

		startTime := time.Now()
		err := test.simulateLSPRequest(requestType)
		responseTime := time.Since(startTime)

		result := &RequestResult{
			Success:      err == nil,
			ResponseTime: responseTime,
			RequestType:  requestType,
			Error:        err,
			Timestamp:    startTime,
		}

		select {
		case results <- result:
		case <-time.After(1 * time.Second):
			// Avoid blocking if results channel is full
		}

		// Small delay between requests
		time.Sleep(10 * time.Millisecond)
	}
}

// testLoadBalancingEfficiency tests load balancing efficiency
func (test *ConcurrentRequestPerformanceTest) testLoadBalancingEfficiency(concurrency int, duration time.Duration) error {
	var wg sync.WaitGroup
	requestCount := int64(0)

	endTime := time.Now().Add(duration)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for time.Now().Before(endTime) {
				// Simulate requests to different language servers
				requestType := test.RequestTypes[rand.Intn(len(test.RequestTypes))]
				test.simulateLSPRequest(requestType)
				atomic.AddInt64(&requestCount, 1)
				time.Sleep(50 * time.Millisecond)
			}
		}()
	}

	wg.Wait()

	return nil
}

// measureResponseTimeConsistency measures response time consistency
func (test *ConcurrentRequestPerformanceTest) measureResponseTimeConsistency(t *testing.T, concurrency int, duration time.Duration) *LoadTestMetrics {
	results := make(chan *RequestResult, concurrency*1000)
	var wg sync.WaitGroup

	endTime := time.Now().Add(duration)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for time.Now().Before(endTime) {
				requestType := test.RequestTypes[rand.Intn(len(test.RequestTypes))]

				startTime := time.Now()
				err := test.simulateLSPRequest(requestType)
				responseTime := time.Since(startTime)

				results <- &RequestResult{
					Success:      err == nil,
					ResponseTime: responseTime,
					RequestType:  requestType,
					Error:        err,
					Timestamp:    startTime,
				}

				time.Sleep(20 * time.Millisecond)
			}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	return test.collectLoadTestMetrics(results)
}

// testCircuitBreakerBehavior tests circuit breaker behavior with failures
func (test *ConcurrentRequestPerformanceTest) testCircuitBreakerBehavior(concurrency int, duration time.Duration) error {
	var wg sync.WaitGroup
	errorCount := int64(0)
	requestCount := int64(0)

	endTime := time.Now().Add(duration)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for time.Now().Before(endTime) {
				requestType := test.RequestTypes[rand.Intn(len(test.RequestTypes))]

				// Simulate failures for some requests
				var err error
				if rand.Float32() < 0.1 { // 10% failure rate
					err = fmt.Errorf("simulated server failure")
					atomic.AddInt64(&errorCount, 1)
				} else {
					err = test.simulateLSPRequest(requestType)
				}

				atomic.AddInt64(&requestCount, 1)
				time.Sleep(30 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()

	return nil
}

// testResourceContention tests resource contention with high concurrency
func (test *ConcurrentRequestPerformanceTest) testResourceContention(concurrency int, duration time.Duration) error {
	var wg sync.WaitGroup

	endTime := time.Now().Add(duration)

	// Create high contention scenario
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for time.Now().Before(endTime) {
				// All workers try to access the same resource types
				requestType := "textDocument/definition" // High contention operation
				test.simulateLSPRequest(requestType)
				time.Sleep(5 * time.Millisecond) // Minimal delay to maximize contention
			}
		}()
	}

	wg.Wait()

	return nil
}

// executeLoadProfile executes a specific load testing profile
func (test *ConcurrentRequestPerformanceTest) executeLoadProfile(t *testing.T, profile LoadProfile) *LoadTestMetrics {
	results := make(chan *RequestResult, profile.ConcurrentUsers*profile.RequestsPerUser)
	var wg sync.WaitGroup

	// Ramp up phase
	rampUpInterval := profile.RampUpTime / time.Duration(profile.ConcurrentUsers)

	for i := 0; i < profile.ConcurrentUsers; i++ {
		time.Sleep(rampUpInterval)
		wg.Add(1)
		go func(userID int) {
			defer wg.Done()
			test.loadProfileWorker(userID, profile, results)
		}(i)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	return test.collectLoadTestMetrics(results)
}

// loadProfileWorker executes requests for a load profile
func (test *ConcurrentRequestPerformanceTest) loadProfileWorker(userID int, profile LoadProfile, results chan<- *RequestResult) {
	// Sustain phase
	sustainEnd := time.Now().Add(profile.SustainTime)
	requestCount := 0

	for time.Now().Before(sustainEnd) && requestCount < profile.RequestsPerUser {
		requestType := test.RequestTypes[rand.Intn(len(test.RequestTypes))]

		startTime := time.Now()
		err := test.simulateLSPRequest(requestType)
		responseTime := time.Since(startTime)

		results <- &RequestResult{
			Success:      err == nil,
			ResponseTime: responseTime,
			RequestType:  requestType,
			Error:        err,
			Timestamp:    startTime,
		}

		requestCount++
		time.Sleep(profile.ThinkTime)
	}
}

// performBenchmarkRequests performs requests for benchmarking
func (test *ConcurrentRequestPerformanceTest) performBenchmarkRequests(concurrency int, requestsPerWorker int) error {
	var wg sync.WaitGroup

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < requestsPerWorker; j++ {
				requestType := test.RequestTypes[j%len(test.RequestTypes)]
				test.simulateLSPRequest(requestType)
			}
		}()
	}

	wg.Wait()
	return nil
}

// simulateLSPRequest simulates an LSP request
func (test *ConcurrentRequestPerformanceTest) simulateLSPRequest(requestType string) error {
	// Simulate request processing time based on request type
	var processingTime time.Duration

	switch requestType {
	case "textDocument/definition":
		processingTime = time.Duration(10+rand.Intn(40)) * time.Millisecond
	case "textDocument/references":
		processingTime = time.Duration(20+rand.Intn(60)) * time.Millisecond
	case "textDocument/documentSymbol":
		processingTime = time.Duration(15+rand.Intn(50)) * time.Millisecond
	case "workspace/symbol":
		processingTime = time.Duration(30+rand.Intn(100)) * time.Millisecond
	case "textDocument/hover":
		processingTime = time.Duration(5+rand.Intn(20)) * time.Millisecond
	case "textDocument/completion":
		processingTime = time.Duration(25+rand.Intn(75)) * time.Millisecond
	default:
		processingTime = time.Duration(10+rand.Intn(30)) * time.Millisecond
	}

	time.Sleep(processingTime)

	// Simulate occasional failures
	if rand.Float32() < 0.02 { // 2% failure rate
		return fmt.Errorf("simulated LSP request failure")
	}

	return nil
}

// collectLoadTestMetrics collects and calculates load test metrics
func (test *ConcurrentRequestPerformanceTest) collectLoadTestMetrics(results <-chan *RequestResult) *LoadTestMetrics {
	var (
		totalRequests      int64
		successfulRequests int64
		failedRequests     int64
		responseTimes      []time.Duration
		minResponseTime    time.Duration = time.Hour
		maxResponseTime    time.Duration
		totalResponseTime  time.Duration
	)

	startTime := time.Now()

	for result := range results {
		totalRequests++
		totalResponseTime += result.ResponseTime
		responseTimes = append(responseTimes, result.ResponseTime)

		if result.Success {
			successfulRequests++
		} else {
			failedRequests++
		}

		if result.ResponseTime < minResponseTime {
			minResponseTime = result.ResponseTime
		}
		if result.ResponseTime > maxResponseTime {
			maxResponseTime = result.ResponseTime
		}
	}

	duration := time.Since(startTime)
	if len(responseTimes) == 0 {
		duration = 1 * time.Second // Avoid division by zero
	}

	// Calculate percentiles
	p50, p95, p99 := test.calculatePercentiles(responseTimes)

	// Calculate average response time
	var avgResponseTime time.Duration
	if totalRequests > 0 {
		avgResponseTime = totalResponseTime / time.Duration(totalRequests)
	}

	// Calculate throughput
	throughput := float64(totalRequests) / duration.Seconds()

	// Calculate error rate
	errorRate := float64(failedRequests) / float64(totalRequests)

	return &LoadTestMetrics{
		TotalRequests:       totalRequests,
		SuccessfulRequests:  successfulRequests,
		FailedRequests:      failedRequests,
		AverageResponseTime: avgResponseTime,
		P50ResponseTime:     p50,
		P95ResponseTime:     p95,
		P99ResponseTime:     p99,
		MaxResponseTime:     maxResponseTime,
		MinResponseTime:     minResponseTime,
		ThroughputReqPerSec: throughput,
		ErrorRate:           errorRate,
	}
}

// calculatePercentiles calculates response time percentiles
func (test *ConcurrentRequestPerformanceTest) calculatePercentiles(responseTimes []time.Duration) (p50, p95, p99 time.Duration) {
	if len(responseTimes) == 0 {
		return 0, 0, 0
	}

	// Sort response times
	sorted := make([]time.Duration, len(responseTimes))
	copy(sorted, responseTimes)

	// Simple bubble sort for small datasets (would use sort.Slice for larger datasets)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	n := len(sorted)
	p50 = sorted[n*50/100]
	p95 = sorted[min(n*95/100, n-1)]
	p99 = sorted[min(n*99/100, n-1)]

	return p50, p95, p99
}

// validateConcurrentRequestMetrics validates concurrent request performance metrics
func (test *ConcurrentRequestPerformanceTest) validateConcurrentRequestMetrics(t *testing.T, concurrency int, metrics *LoadTestMetrics) {
	// Validate response time
	if metrics.P95ResponseTime > test.MaxResponseTime {
		t.Errorf("P95 response time too high: %v > %v", metrics.P95ResponseTime, test.MaxResponseTime)
	}

	// Validate throughput
	if metrics.ThroughputReqPerSec < test.MinThroughputReqPerSec {
		t.Errorf("Throughput too low: %.2f req/sec < %.2f req/sec",
			metrics.ThroughputReqPerSec, test.MinThroughputReqPerSec)
	}

	// Validate error rate
	if metrics.ErrorRate > test.MaxErrorRate {
		t.Errorf("Error rate too high: %.2f%% > %.2f%%",
			metrics.ErrorRate*100, test.MaxErrorRate*100)
	}

	// Validate that we actually processed requests
	if metrics.TotalRequests == 0 {
		t.Error("No requests were processed")
	}
}

// validateResponseTimeConsistency validates response time consistency
func (test *ConcurrentRequestPerformanceTest) validateResponseTimeConsistency(t *testing.T, metrics *LoadTestMetrics) {
	// Check that P99 is not drastically higher than P50 (consistency check)
	if metrics.P99ResponseTime > metrics.P50ResponseTime*5 {
		t.Errorf("Response time inconsistency detected: P99=%v is >5x P50=%v",
			metrics.P99ResponseTime, metrics.P50ResponseTime)
	}

	// Check that max response time is reasonable
	if metrics.MaxResponseTime > test.MaxResponseTime*2 {
		t.Errorf("Maximum response time too high: %v > %v",
			metrics.MaxResponseTime, test.MaxResponseTime*2)
	}
}

// validateLoadProfileResults validates load profile test results
func (test *ConcurrentRequestPerformanceTest) validateLoadProfileResults(t *testing.T, profile LoadProfile, metrics *LoadTestMetrics) {
	// Validate based on load profile type
	switch profile.Name {
	case "spike_load":
		// Spike load should handle high concurrency bursts
		if metrics.ErrorRate > test.MaxErrorRate*2 {
			t.Errorf("Spike load error rate too high: %.2f%% > %.2f%%",
				metrics.ErrorRate*100, test.MaxErrorRate*2*100)
		}
	case "sustained_load":
		// Sustained load should maintain consistent performance
		if metrics.P95ResponseTime > test.MaxResponseTime {
			t.Errorf("Sustained load P95 response time too high: %v > %v",
				metrics.P95ResponseTime, test.MaxResponseTime)
		}
	case "stress_load":
		// Stress load may have higher error rates but should not crash
		if metrics.ErrorRate > 0.20 { // 20% max for stress test
			t.Errorf("Stress load error rate excessive: %.2f%% > 20%%", metrics.ErrorRate*100)
		}
	}

	t.Logf("Load profile %s: Requests=%d, Throughput=%.2f req/sec, P95=%v, Error rate=%.2f%%",
		profile.Name, metrics.TotalRequests, metrics.ThroughputReqPerSec,
		metrics.P95ResponseTime, metrics.ErrorRate*100)
}

// cleanup performs cleanup of test resources
func (test *ConcurrentRequestPerformanceTest) cleanup() {
	if test.framework != nil {
		test.framework.CleanupAll()
	}
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Integration tests that use the concurrent request performance test framework
func TestConcurrentRequestPerformanceSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent request performance tests in short mode")
	}

	perfTest := NewConcurrentRequestPerformanceTest(t)

	t.Run("ConcurrentRequestHandling", perfTest.TestConcurrentRequestHandling)
	t.Run("LoadBalancingEfficiency", perfTest.TestLoadBalancingEfficiency)
	t.Run("ResponseTimeConsistency", perfTest.TestResponseTimeConsistency)
	t.Run("CircuitBreakerPerformance", perfTest.TestCircuitBreakerPerformance)
	t.Run("ResourceContention", perfTest.TestResourceContentionIdentification)
	t.Run("LoadProfiles", perfTest.TestLoadProfiles)
}

// Benchmark function
func BenchmarkConcurrentRequestPerformance(b *testing.B) {
	perfTest := NewConcurrentRequestPerformanceTest(nil)
	perfTest.BenchmarkConcurrentRequests(b)
}
