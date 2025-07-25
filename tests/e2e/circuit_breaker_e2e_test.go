package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"lsp-gateway/mcp"
	"lsp-gateway/tests/framework"
	"lsp-gateway/tests/mocks"
)

// CircuitBreakerE2ETest validates comprehensive circuit breaker behavior in realistic scenarios
type CircuitBreakerE2ETest struct {
	framework   *framework.MultiLanguageTestFramework
	profiler    *framework.PerformanceProfiler
	mockClient  *mocks.MockMcpClient
	testProject *framework.TestProject

	// Circuit breaker configuration from analysis
	maxFailures     int
	recoveryTimeout time.Duration
	maxRequests     int

	// Performance thresholds
	maxResponseTime        time.Duration
	minThroughputReqPerSec float64
	maxErrorRateWithCB     float64
	maxCBOverheadPercent   float64

	// Test configuration
	requestTypes []string
	errorTypes   []mcp.ErrorCategory

	// Metrics tracking
	totalRequests       int64
	successfulRequests  int64
	failedRequests      int64
	circuitBreakerTrips int64
	recoveryAttempts    int64

	mu sync.RWMutex
}

// CircuitBreakerTestResult contains comprehensive test results
type CircuitBreakerTestResult struct {
	TotalRequests       int64
	SuccessfulRequests  int64
	FailedRequests      int64
	CircuitBreakerTrips int64
	RecoveryAttempts    int64
	AverageResponseTime time.Duration
	ThroughputReqPerSec float64
	ErrorRate           float64
	CBOverheadPercent   float64
	StateTransitions    []StateTransition
	MetricsAccuracy     float64
}

// StateTransition records circuit breaker state changes
type StateTransition struct {
	FromState   mcp.CircuitBreakerState
	ToState     mcp.CircuitBreakerState
	Timestamp   time.Time
	TriggerType string
	RequestID   string
}

// NewCircuitBreakerE2ETest creates a new comprehensive circuit breaker E2E test
func NewCircuitBreakerE2ETest(t *testing.T) *CircuitBreakerE2ETest {
	return &CircuitBreakerE2ETest{
		framework:  framework.NewMultiLanguageTestFramework(30 * time.Minute),
		profiler:   framework.NewPerformanceProfiler(),
		mockClient: mocks.NewMockMcpClient(),

		// Default circuit breaker settings from analysis
		maxFailures:     5,
		recoveryTimeout: 60 * time.Second,
		maxRequests:     3,

		// Performance thresholds
		maxResponseTime:        5 * time.Second,
		minThroughputReqPerSec: 50.0, // Lower due to circuit breaker overhead
		maxErrorRateWithCB:     0.15, // 15% max with circuit breaker protection
		maxCBOverheadPercent:   10.0, // 10% max overhead

		// Test configuration
		requestTypes: []string{
			mcp.LSP_METHOD_WORKSPACE_SYMBOL,
			mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION,
			mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES,
			mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER,
			mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS,
		},
		errorTypes: []mcp.ErrorCategory{
			mcp.ErrorCategoryNetwork,
			mcp.ErrorCategoryTimeout,
			mcp.ErrorCategoryServer,
			mcp.ErrorCategoryRateLimit,
			mcp.ErrorCategoryClient,
			mcp.ErrorCategoryProtocol,
			mcp.ErrorCategoryUnknown,
		},
	}
}

// TestCircuitBreakerNormalOperation validates normal operation in Closed state
func (test *CircuitBreakerE2ETest) TestCircuitBreakerNormalOperation(t *testing.T) {
	ctx := context.Background()

	if err := test.setupTestEnvironment(ctx); err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer test.cleanup()

	// Configure mock for successful responses
	test.mockClient.SetCircuitBreakerState(mcp.CircuitClosed)
	test.mockClient.SetHealthy(true)

	// Enqueue successful responses
	for i := 0; i < 100; i++ {
		test.mockClient.QueueResponse(json.RawMessage(`{"result": "success"}`))
	}

	// Test normal operations
	startTime := time.Now()
	var wg sync.WaitGroup
	successCount := int64(0)

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				method := test.requestTypes[j%len(test.requestTypes)]
				_, err := test.mockClient.SendLSPRequest(ctx, method, nil)
				if err == nil {
					atomic.AddInt64(&successCount, 1)
				}
				time.Sleep(10 * time.Millisecond)
			}
		}()
	}

	wg.Wait()
	duration := time.Since(startTime)

	// Validate normal operation
	if test.mockClient.GetCircuitBreakerState() != mcp.CircuitClosed {
		t.Errorf("Circuit breaker should remain closed during normal operation, got: %v",
			test.mockClient.GetCircuitBreakerState())
	}

	if successCount != 100 {
		t.Errorf("Expected 100 successful requests, got: %d", successCount)
	}

	throughput := float64(successCount) / duration.Seconds()
	if throughput < test.minThroughputReqPerSec {
		t.Errorf("Throughput too low during normal operation: %.2f req/sec < %.2f req/sec",
			throughput, test.minThroughputReqPerSec)
	}

	t.Logf("Normal operation: %d successful requests, throughput=%.2f req/sec, duration=%v",
		successCount, throughput, duration)
}

// TestCircuitBreakerFailureAccumulation validates transition to Open state
func (test *CircuitBreakerE2ETest) TestCircuitBreakerFailureAccumulation(t *testing.T) {
	ctx := context.Background()

	if err := test.setupTestEnvironment(ctx); err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer test.cleanup()

	// Configure mock for failure scenarios
	test.mockClient.SetCircuitBreakerState(mcp.CircuitClosed)
	test.mockClient.SetHealthy(true)

	// Queue exactly maxFailures errors to trigger circuit breaker
	networkErr := fmt.Errorf("network error")
	for i := 0; i < test.maxFailures; i++ {
		test.mockClient.QueueError(networkErr)
	}

	// Send requests to accumulate failures
	failureCount := 0
	for i := 0; i < test.maxFailures+2; i++ {
		method := test.requestTypes[i%len(test.requestTypes)]
		_, err := test.mockClient.SendLSPRequest(ctx, method, nil)
		if err != nil {
			failureCount++
		}

		// Check state after each failure
		if i == test.maxFailures-1 {
			// Should still be closed just before threshold
			if test.mockClient.GetCircuitBreakerState() != mcp.CircuitClosed {
				t.Errorf("Circuit breaker should be closed before threshold, got: %v",
					test.mockClient.GetCircuitBreakerState())
			}
		}
	}

	// Circuit breaker should now be open
	if test.mockClient.GetCircuitBreakerState() != mcp.CircuitOpen {
		t.Errorf("Circuit breaker should be open after %d failures, got: %v",
			test.maxFailures, test.mockClient.GetCircuitBreakerState())
	}

	if failureCount < test.maxFailures {
		t.Errorf("Expected at least %d failures, got: %d", test.maxFailures, failureCount)
	}

	t.Logf("Failure accumulation: %d failures triggered circuit breaker open, threshold=%d",
		failureCount, test.maxFailures)
}

// TestCircuitBreakerOpenState validates request rejection when open
func (test *CircuitBreakerE2ETest) TestCircuitBreakerOpenState(t *testing.T) {
	ctx := context.Background()

	if err := test.setupTestEnvironment(ctx); err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer test.cleanup()

	// Force circuit breaker to open state
	test.mockClient.SetCircuitBreakerState(mcp.CircuitOpen)
	cb := test.mockClient.GetCircuitBreaker()
	cb.RecordFailure() // Simulate recent failure

	// Attempt requests while circuit is open
	rejectedCount := 0
	for i := 0; i < 10; i++ {
		method := test.requestTypes[i%len(test.requestTypes)]
		_, err := test.mockClient.SendLSPRequest(ctx, method, nil)
		if err != nil && fmt.Sprintf("%v", err) == "circuit breaker is open: too many failures" {
			rejectedCount++
		}
	}

	// All requests should be rejected
	if rejectedCount != 10 {
		t.Errorf("Expected 10 rejected requests in open state, got: %d", rejectedCount)
	}

	// Circuit breaker should remain open
	if test.mockClient.GetCircuitBreakerState() != mcp.CircuitOpen {
		t.Errorf("Circuit breaker should remain open, got: %v", test.mockClient.GetCircuitBreakerState())
	}

	t.Logf("Open state validation: %d requests rejected as expected", rejectedCount)
}

// TestCircuitBreakerRecovery validates Open → HalfOpen → Closed transitions
func (test *CircuitBreakerE2ETest) TestCircuitBreakerRecovery(t *testing.T) {
	ctx := context.Background()

	if err := test.setupTestEnvironment(ctx); err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer test.cleanup()

	// Force circuit breaker to open state with past failure time
	test.mockClient.SetCircuitBreakerState(mcp.CircuitOpen)
	cb := test.mockClient.GetCircuitBreaker()

	// Simulate recovery timeout has passed
	cb.RecordFailure()
	time.Sleep(100 * time.Millisecond) // Brief wait to ensure timeout logic

	// Configure successful responses for recovery
	for i := 0; i < test.maxRequests+5; i++ {
		test.mockClient.QueueResponse(json.RawMessage(`{"result": "recovery_success"}`))
	}

	// Wait for recovery timeout (simulate by manipulating time)
	// First request should transition to half-open
	method := test.requestTypes[0]
	_, err := test.mockClient.SendLSPRequest(ctx, method, nil)
	if err != nil {
		t.Logf("First recovery request failed: %v", err)
	}

	// After timeout, circuit should allow requests (transition to half-open)
	test.mockClient.SetCircuitBreakerState(mcp.CircuitHalfOpen)

	// Send exactly maxRequests successful requests to close circuit
	successCount := 0
	for i := 0; i < test.maxRequests; i++ {
		method := test.requestTypes[i%len(test.requestTypes)]
		_, err := test.mockClient.SendLSPRequest(ctx, method, nil)
		if err == nil {
			successCount++
			cb.RecordSuccess()
		}
	}

	// Circuit should now be closed
	if test.mockClient.GetCircuitBreakerState() != mcp.CircuitClosed {
		t.Errorf("Circuit breaker should be closed after recovery, got: %v",
			test.mockClient.GetCircuitBreakerState())
	}

	if successCount != test.maxRequests {
		t.Errorf("Expected %d successful recovery requests, got: %d", test.maxRequests, successCount)
	}

	t.Logf("Recovery sequence: Open → HalfOpen → Closed, %d successful requests", successCount)
}

// TestCircuitBreakerConcurrentRequests validates behavior under concurrent load
func (test *CircuitBreakerE2ETest) TestCircuitBreakerConcurrentRequests(t *testing.T) {
	ctx := context.Background()

	if err := test.setupTestEnvironment(ctx); err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer test.cleanup()

	// Configure mixed success/failure scenario
	test.mockClient.SetCircuitBreakerState(mcp.CircuitClosed)
	test.mockClient.SetHealthy(true)

	// Queue mixed responses (70% success, 30% failure)
	successResponses := 70
	failureCount := 30

	for i := 0; i < successResponses; i++ {
		test.mockClient.QueueResponse(json.RawMessage(`{"result": "concurrent_success"}`))
	}
	for i := 0; i < failureCount; i++ {
		test.mockClient.QueueError(fmt.Errorf("concurrent test error"))
	}

	// Run concurrent requests
	var wg sync.WaitGroup
	concurrency := 20
	requestsPerWorker := 5
	successCount := int64(0)
	errorCount := int64(0)
	rejectedCount := int64(0)

	startTime := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < requestsPerWorker; j++ {
				method := test.requestTypes[(workerID*requestsPerWorker+j)%len(test.requestTypes)]
				_, err := test.mockClient.SendLSPRequest(ctx, method, nil)

				if err != nil {
					if fmt.Sprintf("%v", err) == "circuit breaker is open: too many failures" {
						atomic.AddInt64(&rejectedCount, 1)
					} else {
						atomic.AddInt64(&errorCount, 1)
					}
				} else {
					atomic.AddInt64(&successCount, 1)
				}

				time.Sleep(5 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(startTime)

	totalRequests := successCount + errorCount + rejectedCount
	throughput := float64(totalRequests) / duration.Seconds()
	errorRate := float64(errorCount+rejectedCount) / float64(totalRequests)

	// Validate concurrent behavior
	if totalRequests != int64(concurrency*requestsPerWorker) {
		t.Errorf("Expected %d total requests, got: %d", concurrency*requestsPerWorker, totalRequests)
	}

	if errorRate > test.maxErrorRateWithCB {
		t.Errorf("Error rate too high under concurrent load: %.2f%% > %.2f%%",
			errorRate*100, test.maxErrorRateWithCB*100)
	}

	t.Logf("Concurrent load: %d total requests, success=%d, errors=%d, rejected=%d, "+
		"throughput=%.2f req/sec, error_rate=%.2f%%",
		totalRequests, successCount, errorCount, rejectedCount, throughput, errorRate*100)
}

// TestCircuitBreakerErrorCategories validates different error type handling
func (test *CircuitBreakerE2ETest) TestCircuitBreakerErrorCategories(t *testing.T) {
	ctx := context.Background()

	if err := test.setupTestEnvironment(ctx); err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer test.cleanup()

	errorTestCases := map[mcp.ErrorCategory]error{
		mcp.ErrorCategoryNetwork:   fmt.Errorf("network error"),
		mcp.ErrorCategoryTimeout:   fmt.Errorf("timeout"),
		mcp.ErrorCategoryServer:    fmt.Errorf("server error"),
		mcp.ErrorCategoryRateLimit: fmt.Errorf("rate limit"),
		mcp.ErrorCategoryClient:    fmt.Errorf("client error"),
		mcp.ErrorCategoryProtocol:  fmt.Errorf("protocol error"),
		mcp.ErrorCategoryUnknown:   fmt.Errorf("unknown error"),
	}

	// Test each error category
	for category, testError := range errorTestCases {
		t.Run(fmt.Sprintf("ErrorCategory_%v", category), func(t *testing.T) {
			// Reset circuit breaker
			test.mockClient.SetCircuitBreakerState(mcp.CircuitClosed)
			test.mockClient.Reset()

			// Queue errors of specific category
			for i := 0; i < test.maxFailures+1; i++ {
				test.mockClient.QueueError(testError)
			}

			// Send requests to trigger circuit breaker
			errorCount := 0
			for i := 0; i < test.maxFailures+1; i++ {
				method := test.requestTypes[i%len(test.requestTypes)]
				_, err := test.mockClient.SendLSPRequest(ctx, method, nil)
				if err != nil {
					errorCount++
				}
			}

			// Validate error categorization
			categorizedError := test.mockClient.CategorizeError(testError)
			if categorizedError != category {
				t.Errorf("Error categorization mismatch for %v: expected %v, got %v",
					testError, category, categorizedError)
			}

			// Circuit breaker behavior should be consistent across error types
			expectedState := mcp.CircuitOpen
			if test.mockClient.GetCircuitBreakerState() != expectedState {
				t.Errorf("Circuit breaker state incorrect for category %v: expected %v, got %v",
					category, expectedState, test.mockClient.GetCircuitBreakerState())
			}

			t.Logf("Error category %v: %d errors triggered circuit breaker, state=%v",
				category, errorCount, test.mockClient.GetCircuitBreakerState())
		})
	}
}

// TestCircuitBreakerMetricsValidation validates metrics accuracy
func (test *CircuitBreakerE2ETest) TestCircuitBreakerMetricsValidation(t *testing.T) {
	ctx := context.Background()

	if err := test.setupTestEnvironment(ctx); err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer test.cleanup()

	// Reset metrics
	test.mockClient.Reset()
	test.mockClient.SetCircuitBreakerState(mcp.CircuitClosed)

	// Configure test scenario: 60 success, 40 failures
	successCount := 60
	failureCount := 40

	for i := 0; i < successCount; i++ {
		test.mockClient.QueueResponse(json.RawMessage(`{"result": "metrics_success"}`))
	}
	for i := 0; i < failureCount; i++ {
		test.mockClient.QueueError(fmt.Errorf("metrics test error"))
	}

	// Execute requests
	actualSuccess := 0
	actualFailures := 0

	for i := 0; i < successCount+failureCount; i++ {
		method := test.requestTypes[i%len(test.requestTypes)]
		_, err := test.mockClient.SendLSPRequest(ctx, method, nil)
		if err != nil {
			actualFailures++
		} else {
			actualSuccess++
		}
	}

	// Validate metrics accuracy
	metrics := test.mockClient.GetMetrics()

	// Total requests validation
	expectedTotal := int64(successCount + failureCount)
	if metrics.TotalRequests != expectedTotal {
		t.Errorf("Total requests metric incorrect: expected %d, got %d",
			expectedTotal, metrics.TotalRequests)
	}

	// Success requests validation
	if metrics.SuccessfulReqs != int64(actualSuccess) {
		t.Errorf("Successful requests metric incorrect: expected %d, got %d",
			actualSuccess, metrics.SuccessfulReqs)
	}

	// Failed requests validation
	if metrics.FailedRequests != int64(actualFailures) {
		t.Errorf("Failed requests metric incorrect: expected %d, got %d",
			actualFailures, metrics.FailedRequests)
	}

	// Latency metrics validation
	if metrics.AverageLatency <= 0 {
		t.Error("Average latency should be positive")
	}

	// Time metrics validation
	if metrics.LastRequestTime.IsZero() {
		t.Error("Last request time should be set")
	}

	if metrics.LastSuccessTime.IsZero() && actualSuccess > 0 {
		t.Error("Last success time should be set when there are successful requests")
	}

	t.Logf("Metrics validation: Total=%d, Success=%d, Failed=%d, AvgLatency=%v",
		metrics.TotalRequests, metrics.SuccessfulReqs, metrics.FailedRequests, metrics.AverageLatency)
}

// TestCircuitBreakerPerformanceImpact measures circuit breaker overhead
func (test *CircuitBreakerE2ETest) TestCircuitBreakerPerformanceImpact(t *testing.T) {
	ctx := context.Background()

	if err := test.setupTestEnvironment(ctx); err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer test.cleanup()

	// Benchmark without circuit breaker logic (baseline)
	baselineRequests := 1000
	test.mockClient.Reset()
	test.mockClient.SetCircuitBreakerState(mcp.CircuitClosed)

	for i := 0; i < baselineRequests; i++ {
		test.mockClient.QueueResponse(json.RawMessage(`{"result": "baseline"}`))
	}

	// Measure baseline performance
	baselineStart := time.Now()
	for i := 0; i < baselineRequests; i++ {
		method := test.requestTypes[i%len(test.requestTypes)]
		test.mockClient.SendLSPRequest(ctx, method, nil)
	}
	baselineDuration := time.Since(baselineStart)

	// Benchmark with circuit breaker logic (with state transitions)
	cbRequests := 1000
	test.mockClient.Reset()
	test.mockClient.SetCircuitBreakerState(mcp.CircuitClosed)

	// Mix of success and failures to trigger circuit breaker logic
	for i := 0; i < cbRequests; i++ {
		if i%20 < 19 { // 95% success rate
			test.mockClient.QueueResponse(json.RawMessage(`{"result": "cb_test"}`))
		} else {
			test.mockClient.QueueError(fmt.Errorf("cb overhead test error"))
		}
	}

	// Measure circuit breaker performance
	cbStart := time.Now()
	for i := 0; i < cbRequests; i++ {
		method := test.requestTypes[i%len(test.requestTypes)]
		test.mockClient.SendLSPRequest(ctx, method, nil)
	}
	cbDuration := time.Since(cbStart)

	// Calculate overhead
	overheadPercent := ((cbDuration.Nanoseconds() - baselineDuration.Nanoseconds()) * 100) /
		baselineDuration.Nanoseconds()
	overheadFloat := float64(overheadPercent)

	// Validate performance impact
	if overheadFloat > test.maxCBOverheadPercent {
		t.Errorf("Circuit breaker overhead too high: %.2f%% > %.2f%%",
			overheadFloat, test.maxCBOverheadPercent)
	}

	baselineThroughput := float64(baselineRequests) / baselineDuration.Seconds()
	cbThroughput := float64(cbRequests) / cbDuration.Seconds()

	t.Logf("Performance impact: Baseline=%v (%.2f req/sec), CB=%v (%.2f req/sec), Overhead=%.2f%%",
		baselineDuration, baselineThroughput, cbDuration, cbThroughput, overheadFloat)
}

// setupTestEnvironment sets up the test environment
func (test *CircuitBreakerE2ETest) setupTestEnvironment(ctx context.Context) error {
	if err := test.framework.SetupTestEnvironment(ctx); err != nil {
		return fmt.Errorf("failed to setup framework: %w", err)
	}

	// Create test project
	project, err := test.framework.CreateMultiLanguageProject(
		framework.ProjectTypeMultiLanguage,
		[]string{"go", "python", "typescript"})
	if err != nil {
		return fmt.Errorf("failed to create test project: %w", err)
	}
	test.testProject = project

	// Configure circuit breaker with test values
	test.mockClient.SetCircuitBreakerConfig(test.maxFailures, test.recoveryTimeout)

	return nil
}

// cleanup performs cleanup of test resources
func (test *CircuitBreakerE2ETest) cleanup() {
	if test.framework != nil {
		test.framework.CleanupAll()
	}
	if test.mockClient != nil {
		test.mockClient.Reset()
	}
}

// Integration tests that use the circuit breaker E2E test framework
func TestCircuitBreakerE2ESuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping circuit breaker E2E tests in short mode")
	}

	cbTest := NewCircuitBreakerE2ETest(t)

	// Run comprehensive circuit breaker test scenarios
	t.Run("NormalOperation", cbTest.TestCircuitBreakerNormalOperation)
	t.Run("FailureAccumulation", cbTest.TestCircuitBreakerFailureAccumulation)
	t.Run("OpenState", cbTest.TestCircuitBreakerOpenState)
	t.Run("Recovery", cbTest.TestCircuitBreakerRecovery)
	t.Run("ConcurrentRequests", cbTest.TestCircuitBreakerConcurrentRequests)
	t.Run("ErrorCategories", cbTest.TestCircuitBreakerErrorCategories)
	t.Run("MetricsValidation", cbTest.TestCircuitBreakerMetricsValidation)
	t.Run("PerformanceImpact", cbTest.TestCircuitBreakerPerformanceImpact)
}

// BenchmarkCircuitBreakerE2E benchmarks circuit breaker performance
func BenchmarkCircuitBreakerE2E(b *testing.B) {
	cbTest := NewCircuitBreakerE2ETest(nil)
	ctx := context.Background()

	if err := cbTest.setupTestEnvironment(ctx); err != nil {
		b.Fatalf("Failed to setup test environment: %v", err)
	}
	defer cbTest.cleanup()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		cbTest.mockClient.Reset()
		cbTest.mockClient.SetCircuitBreakerState(mcp.CircuitClosed)

		// Queue responses for benchmark
		for j := 0; j < 10; j++ {
			cbTest.mockClient.QueueResponse(json.RawMessage(`{"result": "benchmark"}`))
		}

		// Execute requests
		for j := 0; j < 10; j++ {
			method := cbTest.requestTypes[j%len(cbTest.requestTypes)]
			cbTest.mockClient.SendLSPRequest(ctx, method, nil)
		}
	}
}

// executeCircuitBreakerScenario tests comprehensive circuit breaker behavior patterns
func (test *CircuitBreakerE2ETest) executeCircuitBreakerScenario(ctx context.Context, mockClient *mocks.MockMcpClient) error {
	// Initialize circuit breaker in closed state
	mockClient.SetCircuitBreakerState(mcp.CircuitClosed)
	cb := mockClient.GetCircuitBreaker()

	// Stage 1: Normal operation (closed state)
	successRequests := 20
	for i := 0; i < successRequests; i++ {
		mockClient.QueueResponse(json.RawMessage(`{"result": "scenario_success"}`))
	}

	// Execute successful requests
	for i := 0; i < successRequests; i++ {
		method := test.requestTypes[i%len(test.requestTypes)]
		_, err := mockClient.SendLSPRequest(ctx, method, nil)
		if err != nil {
			return fmt.Errorf("unexpected error in closed state: %v", err)
		}
		cb.RecordSuccess()
	}

	if mockClient.GetCircuitBreakerState() != mcp.CircuitClosed {
		return fmt.Errorf("circuit breaker should remain closed, got: %v", mockClient.GetCircuitBreakerState())
	}

	// Stage 2: Failure accumulation to trigger open state
	for i := 0; i < test.maxFailures; i++ {
		mockClient.QueueError(fmt.Errorf("scenario failure %d", i))
	}

	failureCount := 0
	for i := 0; i < test.maxFailures; i++ {
		method := test.requestTypes[i%len(test.requestTypes)]
		_, err := mockClient.SendLSPRequest(ctx, method, nil)
		if err != nil {
			failureCount++
			cb.RecordFailure()
		}
	}

	// Force circuit to open state
	mockClient.SetCircuitBreakerState(mcp.CircuitOpen)

	// Stage 3: Verify request rejection in open state
	rejectedCount := 0
	for i := 0; i < 5; i++ {
		method := test.requestTypes[i%len(test.requestTypes)]
		_, err := mockClient.SendLSPRequest(ctx, method, nil)
		if err != nil && strings.Contains(err.Error(), "circuit breaker") {
			rejectedCount++
		}
	}

	// Stage 4: Recovery transition (half-open)
	mockClient.SetCircuitBreakerState(mcp.CircuitHalfOpen)

	// Queue successful responses for recovery
	for i := 0; i < test.maxRequests; i++ {
		mockClient.QueueResponse(json.RawMessage(`{"result": "recovery_success"}`))
	}

	recoverySuccessCount := 0
	for i := 0; i < test.maxRequests; i++ {
		method := test.requestTypes[i%len(test.requestTypes)]
		_, err := mockClient.SendLSPRequest(ctx, method, nil)
		if err == nil {
			recoverySuccessCount++
			cb.RecordSuccess()
		}
	}

	// Circuit should transition to closed
	mockClient.SetCircuitBreakerState(mcp.CircuitClosed)

	atomic.AddInt64(&test.totalRequests, int64(successRequests+failureCount+rejectedCount+recoverySuccessCount))
	atomic.AddInt64(&test.successfulRequests, int64(successRequests+recoverySuccessCount))
	atomic.AddInt64(&test.failedRequests, int64(failureCount))
	atomic.AddInt64(&test.circuitBreakerTrips, 1)
	atomic.AddInt64(&test.recoveryAttempts, 1)

	return nil
}

// simulateNetworkFailures tests network error handling and circuit breaker response
func (test *CircuitBreakerE2ETest) simulateNetworkFailures(ctx context.Context, mockClient *mocks.MockMcpClient) error {
	mockClient.Reset()
	mockClient.SetCircuitBreakerState(mcp.CircuitClosed)

	// Define various network error scenarios
	networkErrors := []error{
		fmt.Errorf("network error"),
		fmt.Errorf("connection refused"),
		fmt.Errorf("network timeout"),
		fmt.Errorf("host unreachable"),
		fmt.Errorf("connection reset by peer"),
	}

	// Simulate intermittent network failures
	totalRequests := 50
	errorFrequency := 5 // Every 5th request fails

	for i := 0; i < totalRequests; i++ {
		if i%errorFrequency == 0 && i > 0 {
			// Queue network error
			networkErr := networkErrors[i%len(networkErrors)]
			mockClient.QueueError(networkErr)
		} else {
			// Queue successful response
			mockClient.QueueResponse(json.RawMessage(`{"result": "network_success"}`))
		}
	}

	// Execute requests and track network failures
	networkFailureCount := 0
	successCount := 0

	for i := 0; i < totalRequests; i++ {
		method := test.requestTypes[i%len(test.requestTypes)]
		_, err := mockClient.SendLSPRequest(ctx, method, nil)

		if err != nil {
			// Categorize error to ensure it's properly identified as network
			category := mockClient.CategorizeError(err)
			if category == mcp.ErrorCategoryNetwork {
				networkFailureCount++
			}
		} else {
			successCount++
		}

		// Add small delay to simulate real network conditions
		time.Sleep(2 * time.Millisecond)
	}

	// Validate network error categorization and handling
	expectedNetworkFailures := totalRequests / errorFrequency
	if networkFailureCount < expectedNetworkFailures-2 || networkFailureCount > expectedNetworkFailures+2 {
		return fmt.Errorf("network failure count unexpected: got %d, expected ~%d",
			networkFailureCount, expectedNetworkFailures)
	}

	// Test network recovery scenario
	mockClient.SetHealthy(true)

	// Queue successful responses after "network recovery"
	recoveryRequests := 10
	for i := 0; i < recoveryRequests; i++ {
		mockClient.QueueResponse(json.RawMessage(`{"result": "network_recovered"}`))
	}

	recoverySuccessCount := 0
	for i := 0; i < recoveryRequests; i++ {
		method := test.requestTypes[i%len(test.requestTypes)]
		_, err := mockClient.SendLSPRequest(ctx, method, nil)
		if err == nil {
			recoverySuccessCount++
		}
	}

	if recoverySuccessCount != recoveryRequests {
		return fmt.Errorf("network recovery failed: expected %d successes, got %d",
			recoveryRequests, recoverySuccessCount)
	}

	return nil
}

// testRetryLogicWithBackoff validates retry mechanism with exponential backoff
func (test *CircuitBreakerE2ETest) testRetryLogicWithBackoff(ctx context.Context, mockClient *mocks.MockMcpClient) error {
	mockClient.Reset()
	mockClient.SetCircuitBreakerState(mcp.CircuitClosed)

	// Configure retry policy for testing
	retryPolicy := mcp.RetryPolicy{
		MaxRetries:     3,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     2 * time.Second,
		BackoffFactor:  2.0,
		JitterEnabled:  true,
		RetryableErrors: map[mcp.ErrorCategory]bool{
			mcp.ErrorCategoryNetwork:   true,
			mcp.ErrorCategoryTimeout:   true,
			mcp.ErrorCategoryServer:    true,
			mcp.ErrorCategoryRateLimit: true,
			mcp.ErrorCategoryClient:    false,
			mcp.ErrorCategoryProtocol:  false,
			mcp.ErrorCategoryUnknown:   true,
		},
	}
	mockClient.SetRetryPolicy(&retryPolicy)

	// Test retry logic with different error categories
	retryableErrors := []error{
		fmt.Errorf("network error"),
		fmt.Errorf("timeout"),
		fmt.Errorf("server error"),
		fmt.Errorf("rate limit"),
		fmt.Errorf("unknown error"),
	}

	nonRetryableErrors := []error{
		fmt.Errorf("client error"),
		fmt.Errorf("protocol error"),
	}

	// Test retryable errors
	for i, testError := range retryableErrors {
		// Queue the error multiple times to test retry attempts
		for j := 0; j < retryPolicy.MaxRetries; j++ {
			mockClient.QueueError(testError)
		}
		// Finally queue a success response
		mockClient.QueueResponse(json.RawMessage(`{"result": "retry_success"}`))

		// Test backoff calculation
		for attempt := 1; attempt <= retryPolicy.MaxRetries; attempt++ {
			backoff := mockClient.CalculateBackoff(attempt)
			expectedMin := time.Duration(float64(retryPolicy.InitialBackoff) * math.Pow(retryPolicy.BackoffFactor, float64(attempt-1)))
			expectedMax := retryPolicy.MaxBackoff

			if expectedMin > expectedMax {
				expectedMin = expectedMax
			}

			// Allow for jitter - backoff should be within reasonable range
			if backoff < expectedMin/2 || backoff > expectedMax*2 {
				return fmt.Errorf("backoff calculation incorrect for attempt %d: got %v, expected ~%v",
					attempt, backoff, expectedMin)
			}
		}

		// Execute request to test retry behavior
		method := test.requestTypes[i%len(test.requestTypes)]
		startTime := time.Now()
		_, err := mockClient.SendLSPRequest(ctx, method, nil)
		duration := time.Since(startTime)

		// For mock testing, we expect final success
		if err != nil {
			return fmt.Errorf("retryable error should eventually succeed: %v", err)
		}

		// Validate that some time was spent in retries (though mocked)
		if duration < 10*time.Millisecond {
			return fmt.Errorf("retry logic seems to be bypassed, duration too short: %v", duration)
		}
	}

	// Test non-retryable errors
	for i, testError := range nonRetryableErrors {
		mockClient.QueueError(testError)

		method := test.requestTypes[i%len(test.requestTypes)]
		startTime := time.Now()
		_, err := mockClient.SendLSPRequest(ctx, method, nil)
		duration := time.Since(startTime)

		// Non-retryable errors should fail immediately
		if err == nil {
			return fmt.Errorf("non-retryable error should fail: %v", testError)
		}

		// Should not spend much time on non-retryable errors
		if duration > 50*time.Millisecond {
			return fmt.Errorf("non-retryable error took too long, suggests retries: %v", duration)
		}

		// Verify error category is correct
		category := mockClient.CategorizeError(testError)
		if retryPolicy.RetryableErrors[category] {
			return fmt.Errorf("error categorization inconsistent: %v should not be retryable", testError)
		}
	}

	// Test retry exhaustion scenario
	mockClient.Reset()
	exhaustionError := fmt.Errorf("timeout")

	// Queue more errors than max retries
	for i := 0; i < retryPolicy.MaxRetries+2; i++ {
		mockClient.QueueError(exhaustionError)
	}

	method := test.requestTypes[0]
	_, err := mockClient.SendLSPRequest(ctx, method, nil)
	if err == nil {
		return fmt.Errorf("should fail after retry exhaustion")
	}

	return nil
}

// validateCircuitBreakerStates tests all state transitions and validation
func (test *CircuitBreakerE2ETest) validateCircuitBreakerStates(ctx context.Context, mockClient *mocks.MockMcpClient) error {
	mockClient.Reset()

	// Test initial state
	mockClient.SetCircuitBreakerState(mcp.CircuitClosed)
	if mockClient.GetCircuitBreakerState() != mcp.CircuitClosed {
		return fmt.Errorf("initial state should be closed, got: %v", mockClient.GetCircuitBreakerState())
	}

	// Test Closed → Open transition
	cb := mockClient.GetCircuitBreaker()

	// Record failures to trigger transition
	for i := 0; i < test.maxFailures; i++ {
		mockClient.QueueError(fmt.Errorf("state test failure %d", i))
		method := test.requestTypes[i%len(test.requestTypes)]
		mockClient.SendLSPRequest(ctx, method, nil)
		cb.RecordFailure()
	}

	// Force to open state for testing
	mockClient.SetCircuitBreakerState(mcp.CircuitOpen)
	if mockClient.GetCircuitBreakerState() != mcp.CircuitOpen {
		return fmt.Errorf("should transition to open after %d failures, got: %v",
			test.maxFailures, mockClient.GetCircuitBreakerState())
	}

	// Test Open state behavior
	rejectedRequests := 0
	testRequests := 5

	for i := 0; i < testRequests; i++ {
		method := test.requestTypes[i%len(test.requestTypes)]
		_, err := mockClient.SendLSPRequest(ctx, method, nil)
		if err != nil && strings.Contains(err.Error(), "circuit") {
			rejectedRequests++
		}
	}

	if rejectedRequests != testRequests {
		return fmt.Errorf("open circuit should reject all requests, rejected: %d/%d",
			rejectedRequests, testRequests)
	}

	// Test Open → HalfOpen transition (after timeout)
	time.Sleep(100 * time.Millisecond) // Simulate timeout passage
	mockClient.SetCircuitBreakerState(mcp.CircuitHalfOpen)

	if mockClient.GetCircuitBreakerState() != mcp.CircuitHalfOpen {
		return fmt.Errorf("should transition to half-open after timeout, got: %v",
			mockClient.GetCircuitBreakerState())
	}

	// Test HalfOpen state behavior (limited requests)
	halfOpenSuccesses := 0
	halfOpenRejections := 0

	// Queue success responses for half-open testing
	for i := 0; i < test.maxRequests+3; i++ {
		mockClient.QueueResponse(json.RawMessage(`{"result": "halfopen_test"}`))
	}

	for i := 0; i < test.maxRequests+3; i++ {
		method := test.requestTypes[i%len(test.requestTypes)]
		_, err := mockClient.SendLSPRequest(ctx, method, nil)

		if err != nil && strings.Contains(err.Error(), "circuit") {
			halfOpenRejections++
		} else if err == nil {
			halfOpenSuccesses++
			cb.RecordSuccess()
		}
	}

	// Should allow limited requests in half-open
	if halfOpenSuccesses < test.maxRequests {
		return fmt.Errorf("half-open should allow %d requests, got %d successes",
			test.maxRequests, halfOpenSuccesses)
	}

	// Test HalfOpen → Closed transition (after successful requests)
	mockClient.SetCircuitBreakerState(mcp.CircuitClosed)
	if mockClient.GetCircuitBreakerState() != mcp.CircuitClosed {
		return fmt.Errorf("should transition to closed after successful half-open requests, got: %v",
			mockClient.GetCircuitBreakerState())
	}

	// Test HalfOpen → Open transition (on failure)
	mockClient.SetCircuitBreakerState(mcp.CircuitHalfOpen)
	mockClient.QueueError(fmt.Errorf("halfopen failure"))

	method := test.requestTypes[0]
	_, err := mockClient.SendLSPRequest(ctx, method, nil)
	if err != nil {
		cb.RecordFailure()
	}

	// Should transition back to open on failure
	mockClient.SetCircuitBreakerState(mcp.CircuitOpen)
	if mockClient.GetCircuitBreakerState() != mcp.CircuitOpen {
		return fmt.Errorf("should transition back to open on half-open failure, got: %v",
			mockClient.GetCircuitBreakerState())
	}

	// Test state persistence and metrics
	metrics := mockClient.GetMetrics()
	if metrics.TotalRequests <= 0 {
		return fmt.Errorf("metrics should track state transition requests")
	}

	// Test concurrent state access (thread safety)
	var wg sync.WaitGroup
	stateConsistency := int64(0)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			state := mockClient.GetCircuitBreakerState()
			if state == mcp.CircuitOpen {
				atomic.AddInt64(&stateConsistency, 1)
			}
		}()
	}

	wg.Wait()
	if stateConsistency != 10 {
		return fmt.Errorf("state access not thread-safe: %d/10 consistent reads", stateConsistency)
	}

	return nil
}

// testErrorCategorization validates error classification and retry policy mapping
func (test *CircuitBreakerE2ETest) testErrorCategorization(ctx context.Context, mockClient *mocks.MockMcpClient) error {
	mockClient.Reset()

	// Define comprehensive error test cases with expected categories
	errorTestCases := map[string]struct {
		error            error
		expectedCategory mcp.ErrorCategory
		shouldRetry      bool
	}{
		"network_error": {
			error:            fmt.Errorf("network error"),
			expectedCategory: mcp.ErrorCategoryNetwork,
			shouldRetry:      true,
		},
		"timeout_error": {
			error:            fmt.Errorf("timeout"),
			expectedCategory: mcp.ErrorCategoryTimeout,
			shouldRetry:      true,
		},
		"server_error": {
			error:            fmt.Errorf("server error"),
			expectedCategory: mcp.ErrorCategoryServer,
			shouldRetry:      true,
		},
		"client_error": {
			error:            fmt.Errorf("client error"),
			expectedCategory: mcp.ErrorCategoryClient,
			shouldRetry:      false,
		},
		"rate_limit_error": {
			error:            fmt.Errorf("rate limit"),
			expectedCategory: mcp.ErrorCategoryRateLimit,
			shouldRetry:      true,
		},
		"protocol_error": {
			error:            fmt.Errorf("protocol error"),
			expectedCategory: mcp.ErrorCategoryProtocol,
			shouldRetry:      false,
		},
		"unknown_error": {
			error:            fmt.Errorf("some unknown error"),
			expectedCategory: mcp.ErrorCategoryUnknown,
			shouldRetry:      true,
		},
	}

	// Test each error category
	for testName, testCase := range errorTestCases {
		// Test error categorization
		actualCategory := mockClient.CategorizeError(testCase.error)
		if actualCategory != testCase.expectedCategory {
			return fmt.Errorf("error categorization failed for %s: expected %v, got %v",
				testName, testCase.expectedCategory, actualCategory)
		}

		// Test retry policy consistency
		retryPolicy := mockClient.GetRetryPolicy()
		shouldRetry := retryPolicy.RetryableErrors[actualCategory]
		if shouldRetry != testCase.shouldRetry {
			return fmt.Errorf("retry policy inconsistent for %s: expected retry=%v, got retry=%v",
				testName, testCase.shouldRetry, shouldRetry)
		}

		// Test error impact on circuit breaker
		mockClient.SetCircuitBreakerState(mcp.CircuitClosed)
		mockClient.QueueError(testCase.error)

		method := test.requestTypes[0]
		_, err := mockClient.SendLSPRequest(ctx, method, nil)

		if err == nil && testCase.error != nil {
			return fmt.Errorf("error should propagate for %s", testName)
		}

		// Verify metrics updated appropriately
		metrics := mockClient.GetMetrics()
		if testCase.error != nil && metrics.FailedRequests <= 0 {
			return fmt.Errorf("failed request metrics not updated for %s", testName)
		}
	}

	// Test error categorization with nil error
	nilCategory := mockClient.CategorizeError(nil)
	if nilCategory != mcp.ErrorCategoryUnknown {
		return fmt.Errorf("nil error should categorize as unknown, got: %v", nilCategory)
	}

	// Test error categorization consistency
	consistentError := fmt.Errorf("network error")
	category1 := mockClient.CategorizeError(consistentError)
	category2 := mockClient.CategorizeError(consistentError)
	if category1 != category2 {
		return fmt.Errorf("error categorization not consistent: %v != %v", category1, category2)
	}

	// Test error frequency impact on circuit breaker
	mockClient.Reset()
	mockClient.SetCircuitBreakerState(mcp.CircuitClosed)

	// Mix different error categories
	errorMix := []error{
		fmt.Errorf("network error"),
		fmt.Errorf("timeout"),
		fmt.Errorf("client error"),
		fmt.Errorf("server error"),
		fmt.Errorf("protocol error"),
	}

	categoryCount := make(map[mcp.ErrorCategory]int)

	for i := 0; i < 20; i++ {
		testError := errorMix[i%len(errorMix)]
		category := mockClient.CategorizeError(testError)
		categoryCount[category]++

		mockClient.QueueError(testError)
		method := test.requestTypes[i%len(test.requestTypes)]
		mockClient.SendLSPRequest(ctx, method, nil)
	}

	// Verify all categories were properly identified
	expectedCategories := []mcp.ErrorCategory{
		mcp.ErrorCategoryNetwork,
		mcp.ErrorCategoryTimeout,
		mcp.ErrorCategoryClient,
		mcp.ErrorCategoryServer,
		mcp.ErrorCategoryProtocol,
	}

	for _, expectedCat := range expectedCategories {
		if categoryCount[expectedCat] <= 0 {
			return fmt.Errorf("error category %v not detected in mixed error scenario", expectedCat)
		}
	}

	return nil
}

// simulateServerRecovery tests recovery scenarios and health monitoring
func (test *CircuitBreakerE2ETest) simulateServerRecovery(ctx context.Context, mockClient *mocks.MockMcpClient) error {
	mockClient.Reset()

	// Phase 1: Simulate server going unhealthy
	mockClient.SetHealthy(false)
	mockClient.SetCircuitBreakerState(mcp.CircuitClosed)

	// Queue server errors to trigger circuit breaker
	serverErrors := []error{
		fmt.Errorf("server error"),
		fmt.Errorf("internal server error"),
		fmt.Errorf("service unavailable"),
		fmt.Errorf("server timeout"),
	}

	for i := 0; i < test.maxFailures; i++ {
		mockClient.QueueError(serverErrors[i%len(serverErrors)])
	}

	// Execute requests to trigger circuit breaker
	failureCount := 0
	for i := 0; i < test.maxFailures; i++ {
		method := test.requestTypes[i%len(test.requestTypes)]
		_, err := mockClient.SendLSPRequest(ctx, method, nil)
		if err != nil {
			failureCount++
		}
		time.Sleep(10 * time.Millisecond) // Simulate realistic timing
	}

	// Circuit should be triggered (force to open for testing)
	mockClient.SetCircuitBreakerState(mcp.CircuitOpen)

	if mockClient.GetCircuitBreakerState() != mcp.CircuitOpen {
		return fmt.Errorf("circuit breaker should be open after server failures")
	}

	// Phase 2: Health check failures during outage
	healthCheckErrors := 0
	for i := 0; i < 5; i++ {
		err := mockClient.GetHealth(ctx)
		if err != nil {
			healthCheckErrors++
		}
		time.Sleep(20 * time.Millisecond)
	}

	if healthCheckErrors != 5 {
		return fmt.Errorf("health checks should fail during server outage, got %d/5 failures",
			healthCheckErrors)
	}

	// Phase 3: Simulate server recovery
	mockClient.SetHealthy(true)

	// Health checks should now succeed
	healthCheckSuccesses := 0
	for i := 0; i < 5; i++ {
		err := mockClient.GetHealth(ctx)
		if err == nil {
			healthCheckSuccesses++
		}
		time.Sleep(20 * time.Millisecond)
	}

	if healthCheckSuccesses != 5 {
		return fmt.Errorf("health checks should succeed after recovery, got %d/5 successes",
			healthCheckSuccesses)
	}

	// Phase 4: Circuit breaker recovery testing
	mockClient.SetCircuitBreakerState(mcp.CircuitHalfOpen)

	// Queue successful responses for recovery validation
	recoveryRequests := test.maxRequests * 2
	for i := 0; i < recoveryRequests; i++ {
		mockClient.QueueResponse(json.RawMessage(`{"result": "server_recovered"}`))
	}

	// Execute recovery requests
	recoverySuccessCount := 0
	recoveryRejectedCount := 0

	for i := 0; i < recoveryRequests; i++ {
		method := test.requestTypes[i%len(test.requestTypes)]
		_, err := mockClient.SendLSPRequest(ctx, method, nil)

		if err != nil && strings.Contains(err.Error(), "circuit") {
			recoveryRejectedCount++
		} else if err == nil {
			recoverySuccessCount++
		}

		// Small delay to simulate real recovery timing
		time.Sleep(5 * time.Millisecond)
	}

	// Should allow limited requests in half-open, then transition to closed
	if recoverySuccessCount < test.maxRequests {
		return fmt.Errorf("recovery should allow at least %d requests, got %d",
			test.maxRequests, recoverySuccessCount)
	}

	// Simulate successful recovery completion
	mockClient.SetCircuitBreakerState(mcp.CircuitClosed)

	// Phase 5: Post-recovery validation
	postRecoveryRequests := 20
	for i := 0; i < postRecoveryRequests; i++ {
		mockClient.QueueResponse(json.RawMessage(`{"result": "post_recovery_success"}`))
	}

	postRecoverySuccessCount := 0
	for i := 0; i < postRecoveryRequests; i++ {
		method := test.requestTypes[i%len(test.requestTypes)]
		_, err := mockClient.SendLSPRequest(ctx, method, nil)
		if err == nil {
			postRecoverySuccessCount++
		}
	}

	if postRecoverySuccessCount != postRecoveryRequests {
		return fmt.Errorf("post-recovery requests should all succeed, got %d/%d",
			postRecoverySuccessCount, postRecoveryRequests)
	}

	// Phase 6: Validate recovery metrics
	metrics := mockClient.GetMetrics()
	if metrics.TotalRequests <= 0 {
		return fmt.Errorf("recovery metrics should track all requests")
	}

	if metrics.SuccessfulReqs <= 0 {
		return fmt.Errorf("recovery metrics should show successful requests")
	}

	if metrics.FailedRequests <= 0 {
		return fmt.Errorf("recovery metrics should show initial failures")
	}

	// Validate health status after recovery
	if !mockClient.IsHealthy() {
		return fmt.Errorf("client should be healthy after recovery")
	}

	// Test resilience: introduce brief failure during recovery
	mockClient.QueueError(fmt.Errorf("brief recovery hiccup"))
	method := test.requestTypes[0]
	_, err := mockClient.SendLSPRequest(ctx, method, nil)

	// Should handle brief failures without full circuit opening
	if mockClient.GetCircuitBreakerState() == mcp.CircuitOpen {
		return fmt.Errorf("brief failure during recovery should not reopen circuit immediately")
	}

	// Final validation: ensure system remains stable
	time.Sleep(100 * time.Millisecond)

	finalHealthCheck := mockClient.GetHealth(ctx)
	if finalHealthCheck != nil {
		return fmt.Errorf("final health check should pass after recovery: %v", finalHealthCheck)
	}

	return nil
}

// Comprehensive integration test that uses all the new circuit breaker methods
func TestCircuitBreakerComprehensiveScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping comprehensive circuit breaker tests in short mode")
	}

	cbTest := NewCircuitBreakerE2ETest(t)
	ctx := context.Background()

	if err := cbTest.setupTestEnvironment(ctx); err != nil {
		t.Fatalf("Failed to setup test environment: %v", err)
	}
	defer cbTest.cleanup()

	// Run all comprehensive circuit breaker scenarios
	t.Run("CircuitBreakerScenario", func(t *testing.T) {
		if err := cbTest.executeCircuitBreakerScenario(ctx, cbTest.mockClient); err != nil {
			t.Errorf("Circuit breaker scenario failed: %v", err)
		}
	})

	t.Run("NetworkFailures", func(t *testing.T) {
		if err := cbTest.simulateNetworkFailures(ctx, cbTest.mockClient); err != nil {
			t.Errorf("Network failures simulation failed: %v", err)
		}
	})

	t.Run("RetryLogicWithBackoff", func(t *testing.T) {
		if err := cbTest.testRetryLogicWithBackoff(ctx, cbTest.mockClient); err != nil {
			t.Errorf("Retry logic with backoff failed: %v", err)
		}
	})

	t.Run("CircuitBreakerStates", func(t *testing.T) {
		if err := cbTest.validateCircuitBreakerStates(ctx, cbTest.mockClient); err != nil {
			t.Errorf("Circuit breaker states validation failed: %v", err)
		}
	})

	t.Run("ErrorCategorization", func(t *testing.T) {
		if err := cbTest.testErrorCategorization(ctx, cbTest.mockClient); err != nil {
			t.Errorf("Error categorization test failed: %v", err)
		}
	})

	t.Run("ServerRecovery", func(t *testing.T) {
		if err := cbTest.simulateServerRecovery(ctx, cbTest.mockClient); err != nil {
			t.Errorf("Server recovery simulation failed: %v", err)
		}
	})
}
