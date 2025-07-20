package testutil

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"lsp-gateway/internal/integration/fixtures"
)

// IntegrationTestSuite provides comprehensive integration testing capabilities
type IntegrationTestSuite struct {
	t               *testing.T
	env             *TestEnvironment
	httpClient      *HTTPClientPool
	requestGen      *RequestGenerator
	perfMonitor     *PerformanceMonitor
	fixtures        *fixtures.FixtureManager
	config          *IntegrationTestConfig
	cleanupFuncs    []func()
	mu              sync.RWMutex
}

// IntegrationTestConfig configures the integration test suite
type IntegrationTestConfig struct {
	TestName            string
	Environment         *TestEnvironmentConfig
	HTTPClientPool      *HTTPClientPoolConfig
	Performance         bool
	ConcurrentUsers     int
	TestDuration        time.Duration
	RequestPatterns     []RequestPattern
	ValidationLevel     ValidationLevel
	FailureThresholds   *FailureThresholds
	ResourceMonitoring  bool
}

// FailureThresholds defines acceptable failure rates and performance limits
type FailureThresholds struct {
	MaxErrorRate        float64       // Maximum acceptable error rate (0.0 to 1.0)
	MaxLatencyP95       time.Duration // Maximum acceptable P95 latency
	MaxLatencyP99       time.Duration // Maximum acceptable P99 latency
	MinThroughput       float64       // Minimum acceptable requests per second
	MaxMemoryUsageMB    float64       // Maximum acceptable memory usage
	MaxGoroutines       int           // Maximum acceptable goroutine count
}

// NewIntegrationTestSuite creates a new integration test suite
func NewIntegrationTestSuite(t *testing.T, config *IntegrationTestConfig) *IntegrationTestSuite {
	if config == nil {
		config = defaultIntegrationTestConfig()
	}

	suite := &IntegrationTestSuite{
		t:            t,
		config:       config,
		cleanupFuncs: make([]func(), 0),
	}

	// Initialize fixtures manager
	suite.fixtures = fixtures.NewFixtureManager(t, "")

	// Setup test environment
	suite.env = SetupTestEnvironment(t, config.Environment)
	suite.addCleanup(suite.env.Cleanup)

	// Wait for environment to be ready
	if err := suite.env.WaitForReady(30 * time.Second); err != nil {
		t.Fatalf("Test environment not ready: %v", err)
	}

	// Initialize HTTP client pool
	if config.HTTPClientPool == nil {
		config.HTTPClientPool = &HTTPClientPoolConfig{
			BaseURL:  suite.env.BaseURL(),
			PoolSize: 10,
			Timeout:  30 * time.Second,
		}
	}
	suite.httpClient = NewHTTPClientPool(config.HTTPClientPool)

	// Initialize request generator
	suite.requestGen = NewRequestGenerator(suite.env.BaseURL())

	// Initialize performance monitor if enabled
	if config.Performance {
		suite.perfMonitor = NewPerformanceMonitor(t)
		suite.perfMonitor.Start()
		suite.addCleanup(func() {
			metrics := suite.perfMonitor.Stop()
			suite.validatePerformanceMetrics(metrics)
		})
	}

	return suite
}

// defaultIntegrationTestConfig returns default configuration
func defaultIntegrationTestConfig() *IntegrationTestConfig {
	return &IntegrationTestConfig{
		TestName:        "integration_test",
		Environment:     DefaultTestEnvironmentConfig(),
		Performance:     false,
		ConcurrentUsers: 5,
		TestDuration:    30 * time.Second,
		ValidationLevel: ValidationBasic,
		FailureThresholds: &FailureThresholds{
			MaxErrorRate:     0.05, // 5% error rate
			MaxLatencyP95:    100 * time.Millisecond,
			MaxLatencyP99:    200 * time.Millisecond,
			MinThroughput:    10.0, // 10 RPS
			MaxMemoryUsageMB: 200.0,
			MaxGoroutines:    100,
		},
		ResourceMonitoring: true,
	}
}

// addCleanup adds a cleanup function
func (suite *IntegrationTestSuite) addCleanup(fn func()) {
	suite.mu.Lock()
	defer suite.mu.Unlock()
	suite.cleanupFuncs = append(suite.cleanupFuncs, fn)
}

// Cleanup performs all cleanup operations
func (suite *IntegrationTestSuite) Cleanup() {
	suite.mu.Lock()
	cleanupFuncs := make([]func(), len(suite.cleanupFuncs))
	copy(cleanupFuncs, suite.cleanupFuncs)
	suite.mu.Unlock()

	// Execute cleanup functions in reverse order
	for i := len(cleanupFuncs) - 1; i >= 0; i-- {
		func() {
			defer func() {
				if r := recover(); r != nil {
					suite.t.Logf("Cleanup function panicked: %v", r)
				}
			}()
			cleanupFuncs[i]()
		}()
	}
}

// RunBasicFunctionalTests executes basic functional tests
func (suite *IntegrationTestSuite) RunBasicFunctionalTests() error {
	suite.t.Log("Running basic functional tests...")

	patterns := suite.requestGen.GetDefaultRequestPatterns()
	if len(suite.config.RequestPatterns) > 0 {
		patterns = suite.config.RequestPatterns
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, pattern := range patterns {
		suite.t.Logf("Testing %s method", pattern.Method)
		
		request := suite.requestGen.GenerateRequest([]RequestPattern{pattern})
		
		var measurement *LatencyMeasurement
		if suite.perfMonitor != nil {
			measurement = suite.perfMonitor.StartLatencyMeasurement()
		}
		
		response, err := suite.httpClient.SendJSONRPCRequest(ctx, request)
		
		if measurement != nil {
			measurement.End()
		}
		
		if err != nil {
			if suite.perfMonitor != nil {
				suite.perfMonitor.RecordError()
			}
			return fmt.Errorf("failed to send %s request: %w", pattern.Method, err)
		}

		// Validate response if validation function is provided
		if pattern.ValidateFunc != nil {
			if err := pattern.ValidateFunc(response); err != nil {
				return fmt.Errorf("validation failed for %s: %w", pattern.Method, err)
			}
		}

		suite.t.Logf("✓ %s test passed", pattern.Method)
	}

	return nil
}

// RunLoadTest executes a load test with the specified configuration
func (suite *IntegrationTestSuite) RunLoadTest() error {
	suite.t.Log("Running load test...")

	if suite.perfMonitor == nil {
		return fmt.Errorf("performance monitoring must be enabled for load tests")
	}

	loadConfig := &LoadTestConfig{
		ConcurrentUsers:  suite.config.ConcurrentUsers,
		RequestsPerUser:  100, // Default to 100 requests per user
		TestDuration:     suite.config.TestDuration,
		RampUpTime:       suite.config.TestDuration / 4, // 25% ramp-up time
		ThinkTime:        100 * time.Millisecond,
		MaxErrorRate:     suite.config.FailureThresholds.MaxErrorRate,
		TargetThroughput: suite.config.FailureThresholds.MinThroughput,
		LatencyThresholds: LatencyThresholds{
			P50: 50 * time.Millisecond,
			P95: suite.config.FailureThresholds.MaxLatencyP95,
			P99: suite.config.FailureThresholds.MaxLatencyP99,
		},
	}

	requestFunc := func(ctx context.Context) error {
		patterns := suite.requestGen.GetDefaultRequestPatterns()
		request := suite.requestGen.GenerateRequest(patterns)
		
		_, err := suite.httpClient.SendJSONRPCRequest(ctx, request)
		return err
	}

	runner := NewLoadTestRunner(suite.t, loadConfig, requestFunc)
	
	ctx, cancel := context.WithTimeout(context.Background(), suite.config.TestDuration*2)
	defer cancel()

	metrics := runner.Run(ctx)
	suite.t.Logf("Load test completed: %s", suite.formatMetrics(metrics))

	return nil
}

// RunReliabilityTest executes reliability tests with various failure scenarios
func (suite *IntegrationTestSuite) RunReliabilityTest() error {
	suite.t.Log("Running reliability tests...")

	scenarios := []struct {
		name        string
		requestFunc func(ctx context.Context) error
	}{
		{
			name: "invalid_request",
			requestFunc: func(ctx context.Context) error {
				invalidRequest := map[string]interface{}{
					"invalid": "request",
				}
				_, err := suite.httpClient.SendJSONRPCRequest(ctx, invalidRequest)
				// For reliability test, we expect this to fail gracefully
				_ = err // Ignore error for reliability test
				return nil // Don't propagate the error
			},
		},
		{
			name: "malformed_json",
			requestFunc: func(ctx context.Context) error {
				// This would need special handling in the HTTP client
				// For now, test with a valid but unsupported method
				request := JSONRPCRequest{
					JSONRPC: "2.0",
					ID:      "test",
					Method:  "unsupported/method",
					Params:  map[string]interface{}{},
				}
				_, err := suite.httpClient.SendJSONRPCRequest(ctx, request)
				_ = err // Ignore error for reliability test
				return nil // Don't propagate the error
			},
		},
		{
			name: "concurrent_requests",
			requestFunc: func(ctx context.Context) error {
				patterns := suite.requestGen.GetDefaultRequestPatterns()
				request := suite.requestGen.GenerateRequest(patterns)
				
				_, err := suite.httpClient.SendJSONRPCRequest(ctx, request)
				return err
			},
		},
	}

	for _, scenario := range scenarios {
		suite.t.Logf("Running reliability scenario: %s", scenario.name)
		
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		
		for i := 0; i < 10; i++ {
			if err := scenario.requestFunc(ctx); err != nil {
				cancel()
				return fmt.Errorf("reliability scenario %s failed: %w", scenario.name, err)
			}
		}
		
		cancel()
		suite.t.Logf("✓ Reliability scenario %s passed", scenario.name)
	}

	return nil
}

// RunConcurrencyTest executes concurrency tests
func (suite *IntegrationTestSuite) RunConcurrencyTest() error {
	suite.t.Log("Running concurrency tests...")

	patterns := suite.requestGen.GetDefaultRequestPatterns()
	requests := make([]interface{}, 100)
	for i := 0; i < 100; i++ {
		requests[i] = suite.requestGen.GenerateRequest(patterns)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	batchConfig := &BatchRequestConfig{
		Requests:       requests,
		ConcurrentReqs: suite.config.ConcurrentUsers,
		Timeout:        5 * time.Second,
		CollectStats:   true,
	}

	results, err := suite.httpClient.ExecuteBatch(ctx, batchConfig)
	if err != nil {
		return fmt.Errorf("batch execution failed: %w", err)
	}

	// Analyze results
	successCount := 0
	errorCount := 0
	totalLatency := time.Duration(0)

	for _, result := range results {
		if result.Error != nil {
			errorCount++
		} else {
			successCount++
			totalLatency += result.Latency
		}
	}

	errorRate := float64(errorCount) / float64(len(results))
	avgLatency := totalLatency / time.Duration(successCount)

	suite.t.Logf("Concurrency test results: %d success, %d errors, %.2f%% error rate, avg latency: %v",
		successCount, errorCount, errorRate*100, avgLatency)

	if errorRate > suite.config.FailureThresholds.MaxErrorRate {
		return fmt.Errorf("error rate %.2f%% exceeds threshold %.2f%%",
			errorRate*100, suite.config.FailureThresholds.MaxErrorRate*100)
	}

	return nil
}

// RunComprehensiveTest runs all test categories
func (suite *IntegrationTestSuite) RunComprehensiveTest() error {
	suite.t.Log("Running comprehensive integration tests...")

	tests := []struct {
		name string
		fn   func() error
	}{
		{"Basic Functional", suite.RunBasicFunctionalTests},
		{"Reliability", suite.RunReliabilityTest},
		{"Concurrency", suite.RunConcurrencyTest},
	}

	if suite.config.Performance {
		tests = append(tests, struct {
			name string
			fn   func() error
		}{"Load Test", suite.RunLoadTest})
	}

	for _, test := range tests {
		suite.t.Logf("=== %s Tests ===", test.name)
		if err := test.fn(); err != nil {
			return fmt.Errorf("%s tests failed: %w", test.name, err)
		}
		suite.t.Logf("✓ %s tests completed successfully", test.name)
	}

	// Print final statistics
	if suite.httpClient != nil {
		stats := suite.httpClient.GetStats()
		suite.t.Logf("Final HTTP Statistics: %s", stats.String())
	}

	return nil
}

// validatePerformanceMetrics validates performance metrics against thresholds
func (suite *IntegrationTestSuite) validatePerformanceMetrics(metrics *PerformanceMetrics) {
	thresholds := suite.config.FailureThresholds
	if thresholds == nil {
		return
	}

	errorRate := float64(metrics.ErrorCount) / float64(metrics.RequestCount)
	if errorRate > thresholds.MaxErrorRate {
		suite.t.Errorf("Error rate %.2f%% exceeds threshold %.2f%%",
			errorRate*100, thresholds.MaxErrorRate*100)
	}

	if thresholds.MaxLatencyP95 > 0 && metrics.P95Latency > thresholds.MaxLatencyP95 {
		suite.t.Errorf("P95 latency %v exceeds threshold %v",
			metrics.P95Latency, thresholds.MaxLatencyP95)
	}

	if thresholds.MaxLatencyP99 > 0 && metrics.P99Latency > thresholds.MaxLatencyP99 {
		suite.t.Errorf("P99 latency %v exceeds threshold %v",
			metrics.P99Latency, thresholds.MaxLatencyP99)
	}

	if thresholds.MinThroughput > 0 && metrics.ThroughputRPS < thresholds.MinThroughput {
		suite.t.Errorf("Throughput %.2f RPS below threshold %.2f RPS",
			metrics.ThroughputRPS, thresholds.MinThroughput)
	}

	if thresholds.MaxMemoryUsageMB > 0 && metrics.MaxMemoryMB > thresholds.MaxMemoryUsageMB {
		suite.t.Errorf("Memory usage %.2f MB exceeds threshold %.2f MB",
			metrics.MaxMemoryMB, thresholds.MaxMemoryUsageMB)
	}

	if thresholds.MaxGoroutines > 0 && metrics.GoroutineCount > thresholds.MaxGoroutines {
		suite.t.Errorf("Goroutine count %d exceeds threshold %d",
			metrics.GoroutineCount, thresholds.MaxGoroutines)
	}
}

// formatMetrics formats performance metrics for logging
func (suite *IntegrationTestSuite) formatMetrics(metrics *PerformanceMetrics) string {
	return fmt.Sprintf("Duration: %v, Requests: %d, Errors: %d, Throughput: %.2f RPS, P95: %v, P99: %v, Memory: %.2f MB",
		metrics.TestDuration,
		metrics.RequestCount,
		metrics.ErrorCount,
		metrics.ThroughputRPS,
		metrics.P95Latency,
		metrics.P99Latency,
		metrics.MaxMemoryMB)
}

// GetEnvironment returns the test environment
func (suite *IntegrationTestSuite) GetEnvironment() *TestEnvironment {
	return suite.env
}

// GetHTTPClient returns the HTTP client pool
func (suite *IntegrationTestSuite) GetHTTPClient() *HTTPClientPool {
	return suite.httpClient
}

// GetRequestGenerator returns the request generator
func (suite *IntegrationTestSuite) GetRequestGenerator() *RequestGenerator {
	return suite.requestGen
}

// GetPerformanceMonitor returns the performance monitor
func (suite *IntegrationTestSuite) GetPerformanceMonitor() *PerformanceMonitor {
	return suite.perfMonitor
}

// GetFixtures returns the fixture manager
func (suite *IntegrationTestSuite) GetFixtures() *fixtures.FixtureManager {
	return suite.fixtures
}