package base

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"lsp-gateway/src/internal/types"
)

// Test request/response types for generic testing
type testRequest struct {
	Query   string                 `json:"query"`
	Options map[string]interface{} `json:"options,omitempty"`
}

type testResponse struct {
	Results []string `json:"results"`
	Count   int      `json:"count"`
}

// Enhanced mockLSPClient with configurable behavior
type mockLSPClient struct {
	mu             sync.Mutex
	language       string
	shouldFail     bool
	responseDelay  time.Duration
	response       interface{}
	customError    error
	supportsMethod bool
	isActive       bool
	callCount      int32
	startCalled    bool
	stopCalled     bool
}

func newMockLSPClient(language string) *mockLSPClient {
	return &mockLSPClient{
		language:       language,
		shouldFail:     false,
		responseDelay:  0,
		response:       &testResponse{Results: []string{fmt.Sprintf("%s-result", language)}, Count: 1},
		supportsMethod: true,
		isActive:       true,
	}
}

func (m *mockLSPClient) withFailure(err error) *mockLSPClient {
	m.shouldFail = true
	m.customError = err
	return m
}

func (m *mockLSPClient) withDelay(delay time.Duration) *mockLSPClient {
	m.responseDelay = delay
	return m
}

func (m *mockLSPClient) withResponse(response interface{}) *mockLSPClient {
	m.response = response
	return m
}

func (m *mockLSPClient) withInactive() *mockLSPClient {
	m.isActive = false
	return m
}

func (m *mockLSPClient) withUnsupported() *mockLSPClient {
	m.supportsMethod = false
	return m
}

func (m *mockLSPClient) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startCalled = true
	return nil
}

func (m *mockLSPClient) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopCalled = true
	return nil
}

func (m *mockLSPClient) SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	atomic.AddInt32(&m.callCount, 1)

	// Simulate response delay
	if m.responseDelay > 0 {
		select {
		case <-time.After(m.responseDelay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if m.shouldFail {
		if m.customError != nil {
			return nil, m.customError
		}
		return nil, fmt.Errorf("mock client %s failed", m.language)
	}

	responseBytes, err := json.Marshal(m.response)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(responseBytes), nil
}

func (m *mockLSPClient) SendNotification(ctx context.Context, method string, params interface{}) error {
	return nil
}

func (m *mockLSPClient) IsActive() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.isActive
}

func (m *mockLSPClient) Supports(method string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.supportsMethod
}

func (m *mockLSPClient) getCallCount() int32 {
	return atomic.LoadInt32(&m.callCount)
}

// Test executor function that converts generic request/response
func testExecutor(ctx context.Context, client types.LSPClient, request testRequest) (testResponse, error) {
	rawResponse, err := client.SendRequest(ctx, "test/method", request)
	if err != nil {
		return testResponse{}, err
	}

	var response testResponse
	if err := json.Unmarshal(rawResponse, &response); err != nil {
		return testResponse{}, err
	}

	return response, nil
}

// Helper function to create test clients map
func createTestClients(languages []string) map[string]types.LSPClient {
	clients := make(map[string]types.LSPClient)
	for _, lang := range languages {
		clients[lang] = newMockLSPClient(lang)
	}
	return clients
}

func TestNewParallelAggregator(t *testing.T) {
	individualTimeout := 5 * time.Second
	overallTimeout := 10 * time.Second

	aggregator := NewParallelAggregator[testRequest, testResponse](individualTimeout, overallTimeout)

	assert.NotNil(t, aggregator)
	assert.Equal(t, individualTimeout, aggregator.individualTimeout)
	assert.Equal(t, overallTimeout, aggregator.overallTimeout)
}

func TestParallelAggregator_Execute_AllClientsSucceed(t *testing.T) {
	aggregator := NewParallelAggregator[testRequest, testResponse](
		100*time.Millisecond, 500*time.Millisecond)

	languages := []string{"go", "python", "javascript", "typescript", "java", "rust"}
	clients := createTestClients(languages)

	request := testRequest{Query: "test-query"}

	results, errors := aggregator.Execute(context.Background(), clients, request, testExecutor)

	// All clients should succeed
	assert.Len(t, results, len(languages), "Should have results from all clients")
	assert.Empty(t, errors, "Should have no errors")

	// Verify each language has a result
	for _, lang := range languages {
		result, exists := results[lang]
		assert.True(t, exists, "Should have result for language: %s", lang)
		assert.Equal(t, fmt.Sprintf("%s-result", lang), result.Results[0])
		assert.Equal(t, 1, result.Count)
	}
}

func TestParallelAggregator_Execute_SomeClientsFail(t *testing.T) {
	aggregator := NewParallelAggregator[testRequest, testResponse](
		100*time.Millisecond, 500*time.Millisecond)

	clients := map[string]types.LSPClient{
		"go":         newMockLSPClient("go"),
		"python":     newMockLSPClient("python").withFailure(errors.New("python failed")),
		"javascript": newMockLSPClient("javascript"),
		"java":       newMockLSPClient("java").withFailure(errors.New("java timeout")),
	}

	request := testRequest{Query: "test-query"}

	results, errs := aggregator.Execute(context.Background(), clients, request, testExecutor)

	// Should have partial results
	assert.Len(t, results, 2, "Should have results from 2 successful clients")
	assert.Len(t, errs, 2, "Should have 2 errors")

	// Check successful results
	assert.Contains(t, results, "go")
	assert.Contains(t, results, "javascript")

	// Check error messages contain failed languages
	errorStr := fmt.Sprintf("%v", errs)
	assert.Contains(t, errorStr, "python")
	assert.Contains(t, errorStr, "java")
}

func TestParallelAggregator_Execute_AllClientsFail(t *testing.T) {
	aggregator := NewParallelAggregator[testRequest, testResponse](
		100*time.Millisecond, 500*time.Millisecond)

	clients := map[string]types.LSPClient{
		"go":     newMockLSPClient("go").withFailure(errors.New("go failed")),
		"python": newMockLSPClient("python").withFailure(errors.New("python failed")),
	}

	request := testRequest{Query: "test-query"}

	results, errs := aggregator.Execute(context.Background(), clients, request, testExecutor)

	assert.Empty(t, results, "Should have no successful results")
	assert.Len(t, errs, 2, "Should have errors from all clients")
}

func TestParallelAggregator_Execute_IndividualTimeout(t *testing.T) {
	// Use a very short individual timeout to force timeouts
	aggregator := NewParallelAggregator[testRequest, testResponse](
		10*time.Millisecond, 1*time.Second)

	clients := map[string]types.LSPClient{
		"fast": newMockLSPClient("fast").withDelay(5 * time.Millisecond),
		"slow": newMockLSPClient("slow").withDelay(50 * time.Millisecond), // Will timeout
	}

	request := testRequest{Query: "test-query"}

	results, errs := aggregator.Execute(context.Background(), clients, request, testExecutor)

	// Fast client should succeed, slow client should timeout
	assert.Len(t, results, 1, "Should have one successful result")
	assert.Len(t, errs, 1, "Should have one timeout error")
	assert.Contains(t, results, "fast")

	// Check that slow client got a timeout error
	errorStr := fmt.Sprintf("%v", errs)
	assert.Contains(t, errorStr, "slow")
}

func TestParallelAggregator_Execute_OverallTimeout(t *testing.T) {
	// Set overall timeout very short to test overall timeout behavior
	aggregator := NewParallelAggregator[testRequest, testResponse](
		1*time.Second, 20*time.Millisecond)

	clients := map[string]types.LSPClient{
		"slow1": newMockLSPClient("slow1").withDelay(50 * time.Millisecond),
		"slow2": newMockLSPClient("slow2").withDelay(50 * time.Millisecond),
	}

	request := testRequest{Query: "test-query"}

	start := time.Now()
	results, errs := aggregator.Execute(context.Background(), clients, request, testExecutor)
	elapsed := time.Since(start)

	// Should finish around the overall timeout time
	assert.Less(t, elapsed, 100*time.Millisecond, "Should finish due to overall timeout")
	assert.Empty(t, results, "Should have no completed results due to overall timeout")
	assert.Empty(t, errs, "Should have no client errors due to overall timeout")
}

func TestParallelAggregator_Execute_ContextCancellation(t *testing.T) {
	aggregator := NewParallelAggregator[testRequest, testResponse](
		1*time.Second, 5*time.Second)

	clients := map[string]types.LSPClient{
		"go": newMockLSPClient("go").withDelay(100 * time.Millisecond),
	}

	request := testRequest{Query: "test-query"}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	results, errs := aggregator.Execute(ctx, clients, request, testExecutor)

	assert.Empty(t, results, "Should have no results due to context cancellation")
	assert.Len(t, errs, 1, "Should have context cancellation error")
	assert.Contains(t, fmt.Sprintf("%v", errs), "context cancelled")
}

func TestParallelAggregator_Execute_EmptyClients(t *testing.T) {
	aggregator := NewParallelAggregator[testRequest, testResponse](
		100*time.Millisecond, 500*time.Millisecond)

	clients := make(map[string]types.LSPClient)
	request := testRequest{Query: "test-query"}

	results, errs := aggregator.Execute(context.Background(), clients, request, testExecutor)

	assert.Empty(t, results, "Should have no results for empty clients")
	assert.Len(t, errs, 1, "Should have one error for empty clients")
	assert.Contains(t, fmt.Sprintf("%v", errs), "no clients provided")
}

func TestParallelAggregator_ExecuteAll_AllSucceed(t *testing.T) {
	aggregator := NewParallelAggregator[testRequest, testResponse](
		100*time.Millisecond, 500*time.Millisecond)

	clients := map[string]types.LSPClient{
		"go":   newMockLSPClient("go"),
		"java": newMockLSPClient("java"),
	}

	request := testRequest{Query: "test-query"}

	results, err := aggregator.ExecuteAll(context.Background(), clients, request, testExecutor)

	assert.NoError(t, err, "Should succeed when all clients succeed")
	assert.Len(t, results, 2, "Should have results from both clients")
	assert.Contains(t, results, "go")
	assert.Contains(t, results, "java")
}

func TestParallelAggregator_ExecuteAll_OneClientFails(t *testing.T) {
	aggregator := NewParallelAggregator[testRequest, testResponse](
		100*time.Millisecond, 500*time.Millisecond)

	clients := map[string]types.LSPClient{
		"go":   newMockLSPClient("go"),
		"java": newMockLSPClient("java").withFailure(errors.New("java failed")),
	}

	request := testRequest{Query: "test-query"}

	results, err := aggregator.ExecuteAll(context.Background(), clients, request, testExecutor)

	assert.Error(t, err, "Should fail when any client fails")
	assert.Nil(t, results, "Should return nil results on failure")
	assert.Contains(t, err.Error(), "all clients must succeed")
	assert.Contains(t, err.Error(), "java")
}

func TestParallelAggregator_ExecuteAtLeastOne_AllSucceed(t *testing.T) {
	aggregator := NewParallelAggregator[testRequest, testResponse](
		100*time.Millisecond, 500*time.Millisecond)

	clients := map[string]types.LSPClient{
		"go":   newMockLSPClient("go"),
		"java": newMockLSPClient("java"),
	}

	request := testRequest{Query: "test-query"}

	results, err := aggregator.ExecuteAtLeastOne(context.Background(), clients, request, testExecutor)

	assert.NoError(t, err, "Should succeed when all clients succeed")
	assert.Len(t, results, 2, "Should have results from both clients")
}

func TestParallelAggregator_ExecuteAtLeastOne_OneSucceeds(t *testing.T) {
	aggregator := NewParallelAggregator[testRequest, testResponse](
		100*time.Millisecond, 500*time.Millisecond)

	clients := map[string]types.LSPClient{
		"go":   newMockLSPClient("go"),
		"java": newMockLSPClient("java").withFailure(errors.New("java failed")),
	}

	request := testRequest{Query: "test-query"}

	results, err := aggregator.ExecuteAtLeastOne(context.Background(), clients, request, testExecutor)

	assert.NoError(t, err, "Should succeed when at least one client succeeds")
	assert.Len(t, results, 1, "Should have result from one client")
	assert.Contains(t, results, "go")
}

func TestParallelAggregator_ExecuteAtLeastOne_AllFail(t *testing.T) {
	aggregator := NewParallelAggregator[testRequest, testResponse](
		100*time.Millisecond, 500*time.Millisecond)

	clients := map[string]types.LSPClient{
		"go":   newMockLSPClient("go").withFailure(errors.New("go failed")),
		"java": newMockLSPClient("java").withFailure(errors.New("java failed")),
	}

	request := testRequest{Query: "test-query"}

	results, err := aggregator.ExecuteAtLeastOne(context.Background(), clients, request, testExecutor)

	assert.Error(t, err, "Should fail when all clients fail")
	assert.Nil(t, results, "Should return nil results when all fail")
	assert.Contains(t, err.Error(), "all clients failed")
}

func TestParallelAggregator_ExecuteWithLanguageTimeouts_Success(t *testing.T) {
	aggregator := NewParallelAggregator[testRequest, testResponse](
		100*time.Millisecond, 1*time.Second) // Individual timeout will be overridden

	clients := map[string]types.LSPClient{
		"go":   newMockLSPClient("go").withDelay(20 * time.Millisecond),
		"java": newMockLSPClient("java").withDelay(40 * time.Millisecond),
	}

	request := testRequest{Query: "test-query"}

	// Language-specific timeout function
	timeoutFunc := func(language string) time.Duration {
		switch language {
		case "java":
			return 90 * time.Millisecond // Long timeout for Java
		default:
			return 30 * time.Millisecond // Short timeout for others
		}
	}

	results, errs := aggregator.ExecuteWithLanguageTimeouts(
		context.Background(), clients, request, testExecutor, timeoutFunc)

	assert.Len(t, results, 2, "Should have results from both clients")
	assert.Empty(t, errs, "Should have no errors")
}

func TestParallelAggregator_ExecuteWithLanguageTimeouts_LanguageSpecificTimeout(t *testing.T) {
	aggregator := NewParallelAggregator[testRequest, testResponse](
		100*time.Millisecond, 1*time.Second)

	clients := map[string]types.LSPClient{
		"go":   newMockLSPClient("go").withDelay(20 * time.Millisecond),
		"java": newMockLSPClient("java").withDelay(60 * time.Millisecond),
	}

	request := testRequest{Query: "test-query"}

	// Timeout function that gives Go very short timeout but Java longer timeout
	timeoutFunc := func(language string) time.Duration {
		switch language {
		case "java":
			return 100 * time.Millisecond // Java gets enough time
		default:
			return 10 * time.Millisecond // Go will timeout
		}
	}

	results, errs := aggregator.ExecuteWithLanguageTimeouts(
		context.Background(), clients, request, testExecutor, timeoutFunc)

	// Java should succeed, Go should timeout
	assert.Len(t, results, 1, "Should have one successful result")
	assert.Len(t, errs, 1, "Should have one timeout error")
	assert.Contains(t, results, "java")
}

func TestParallelAggregator_Execute_ConcurrentExecution(t *testing.T) {
	aggregator := NewParallelAggregator[testRequest, testResponse](
		200*time.Millisecond, 1*time.Second)

	// Create many clients to test concurrent execution
	numClients := 20
	clients := make(map[string]types.LSPClient)
	for i := 0; i < numClients; i++ {
		lang := fmt.Sprintf("lang-%d", i)
		clients[lang] = newMockLSPClient(lang).withDelay(50 * time.Millisecond)
	}

	request := testRequest{Query: "concurrent-test"}

	start := time.Now()
	results, errs := aggregator.Execute(context.Background(), clients, request, testExecutor)
	elapsed := time.Since(start)

	// All should succeed
	assert.Len(t, results, numClients, "Should have results from all clients")
	assert.Empty(t, errs, "Should have no errors")

	// Should complete in parallel (not sequentially)
	// 20 clients * 50ms each = 1000ms sequentially, but should be ~50ms in parallel
	assert.Less(t, elapsed, 300*time.Millisecond, "Should complete concurrently, not sequentially")

	// Verify all expected results are present
	for i := 0; i < numClients; i++ {
		lang := fmt.Sprintf("lang-%d", i)
		result, exists := results[lang]
		assert.True(t, exists, "Should have result for %s", lang)
		assert.Equal(t, fmt.Sprintf("%s-result", lang), result.Results[0])
	}
}

func TestParallelAggregator_Execute_ThreadSafety(t *testing.T) {
	aggregator := NewParallelAggregator[testRequest, testResponse](
		100*time.Millisecond, 500*time.Millisecond)

	clients := map[string]types.LSPClient{
		"go":   newMockLSPClient("go"),
		"java": newMockLSPClient("java"),
	}

	request := testRequest{Query: "thread-safety-test"}

	// Run multiple parallel aggregations concurrently to test thread safety
	var wg sync.WaitGroup
	numGoroutines := 10
	results := make([]map[string]testResponse, numGoroutines)
	errors := make([][]error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			res, errs := aggregator.Execute(context.Background(), clients, request, testExecutor)
			results[index] = res
			errors[index] = errs
		}(i)
	}

	wg.Wait()

	// All executions should succeed
	for i := 0; i < numGoroutines; i++ {
		assert.Len(t, results[i], 2, "Goroutine %d should have 2 results", i)
		assert.Empty(t, errors[i], "Goroutine %d should have no errors", i)
	}
}

func TestParallelAggregator_Execute_LargeResponsePayload(t *testing.T) {
	aggregator := NewParallelAggregator[testRequest, testResponse](
		200*time.Millisecond, 1*time.Second)

	// Create response with large payload
	largeResults := make([]string, 1000)
	for i := range largeResults {
		largeResults[i] = fmt.Sprintf("large-result-%d", i)
	}
	largeResponse := &testResponse{Results: largeResults, Count: 1000}

	clients := map[string]types.LSPClient{
		"go": newMockLSPClient("go").withResponse(largeResponse),
	}

	request := testRequest{Query: "large-payload-test"}

	results, errs := aggregator.Execute(context.Background(), clients, request, testExecutor)

	assert.Len(t, results, 1, "Should handle large response")
	assert.Empty(t, errs, "Should have no errors")

	result := results["go"]
	assert.Len(t, result.Results, 1000, "Should preserve large response payload")
	assert.Equal(t, 1000, result.Count)
}

func TestParallelAggregator_Execute_EmptyResponse(t *testing.T) {
	aggregator := NewParallelAggregator[testRequest, testResponse](
		100*time.Millisecond, 500*time.Millisecond)

	emptyResponse := &testResponse{Results: []string{}, Count: 0}

	clients := map[string]types.LSPClient{
		"go": newMockLSPClient("go").withResponse(emptyResponse),
	}

	request := testRequest{Query: "empty-response-test"}

	results, errs := aggregator.Execute(context.Background(), clients, request, testExecutor)

	assert.Len(t, results, 1, "Should handle empty response")
	assert.Empty(t, errs, "Should have no errors")

	result := results["go"]
	assert.Empty(t, result.Results, "Should preserve empty results array")
	assert.Equal(t, 0, result.Count)
}

// Benchmark tests
func BenchmarkParallelAggregator_Execute_FewClients(b *testing.B) {
	aggregator := NewParallelAggregator[testRequest, testResponse](
		100*time.Millisecond, 500*time.Millisecond)

	clients := createTestClients([]string{"go", "python", "javascript"})
	request := testRequest{Query: "benchmark-test"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = aggregator.Execute(context.Background(), clients, request, testExecutor)
	}
}

func BenchmarkParallelAggregator_Execute_ManyClients(b *testing.B) {
	aggregator := NewParallelAggregator[testRequest, testResponse](
		100*time.Millisecond, 500*time.Millisecond)

	// Create 10 mock clients
	languages := make([]string, 10)
	for i := 0; i < 10; i++ {
		languages[i] = fmt.Sprintf("lang-%d", i)
	}
	clients := createTestClients(languages)
	request := testRequest{Query: "benchmark-test"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = aggregator.Execute(context.Background(), clients, request, testExecutor)
	}
}

func BenchmarkParallelAggregator_ExecuteAll(b *testing.B) {
	aggregator := NewParallelAggregator[testRequest, testResponse](
		100*time.Millisecond, 500*time.Millisecond)

	clients := createTestClients([]string{"go", "python", "javascript"})
	request := testRequest{Query: "benchmark-test"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = aggregator.ExecuteAll(context.Background(), clients, request, testExecutor)
	}
}

// Table-driven tests for comprehensive edge case coverage
func TestParallelAggregator_Execute_EdgeCases(t *testing.T) {
	tests := []struct {
		name                 string
		individualTimeout    time.Duration
		overallTimeout       time.Duration
		clients              map[string]types.LSPClient
		expectedResults      int
		expectedErrors       int
		shouldFailExecuteAll bool
		shouldFailAtLeastOne bool
	}{
		{
			name:              "Single successful client",
			individualTimeout: 100 * time.Millisecond,
			overallTimeout:    500 * time.Millisecond,
			clients: map[string]types.LSPClient{
				"go": newMockLSPClient("go"),
			},
			expectedResults:      1,
			expectedErrors:       0,
			shouldFailExecuteAll: false,
			shouldFailAtLeastOne: false,
		},
		{
			name:              "Single failing client",
			individualTimeout: 100 * time.Millisecond,
			overallTimeout:    500 * time.Millisecond,
			clients: map[string]types.LSPClient{
				"go": newMockLSPClient("go").withFailure(errors.New("test error")),
			},
			expectedResults:      0,
			expectedErrors:       1,
			shouldFailExecuteAll: true,
			shouldFailAtLeastOne: true,
		},
		{
			name:              "Mixed success and failure",
			individualTimeout: 100 * time.Millisecond,
			overallTimeout:    500 * time.Millisecond,
			clients: map[string]types.LSPClient{
				"go":     newMockLSPClient("go"),
				"java":   newMockLSPClient("java").withFailure(errors.New("test error")),
				"python": newMockLSPClient("python"),
			},
			expectedResults:      2,
			expectedErrors:       1,
			shouldFailExecuteAll: true,
			shouldFailAtLeastOne: false,
		},
		{
			name:              "Zero timeout values",
			individualTimeout: 0,
			overallTimeout:    0,
			clients: map[string]types.LSPClient{
				"go": newMockLSPClient("go"),
			},
			expectedResults:      0,     // Should timeout immediately
			expectedErrors:       0,     // Overall timeout, not client errors
			shouldFailExecuteAll: false, // ExecuteAll succeeds with empty results (no client errors)
			shouldFailAtLeastOne: true,  // ExecuteAtLeastOne fails with empty results
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aggregator := NewParallelAggregator[testRequest, testResponse](
				tt.individualTimeout, tt.overallTimeout)

			request := testRequest{Query: "edge-case-test"}

			// Test Execute
			results, errs := aggregator.Execute(context.Background(), tt.clients, request, testExecutor)
			assert.Len(t, results, tt.expectedResults, "Execute results count mismatch")
			assert.Len(t, errs, tt.expectedErrors, "Execute errors count mismatch")

			// Test ExecuteAll
			allResults, allErr := aggregator.ExecuteAll(context.Background(), tt.clients, request, testExecutor)
			if tt.shouldFailExecuteAll {
				assert.Error(t, allErr, "ExecuteAll should fail")
				assert.Nil(t, allResults, "ExecuteAll should return nil on failure")
			} else {
				assert.NoError(t, allErr, "ExecuteAll should succeed")
				assert.NotNil(t, allResults, "ExecuteAll should return results on success")
			}

			// Test ExecuteAtLeastOne
			oneResults, oneErr := aggregator.ExecuteAtLeastOne(context.Background(), tt.clients, request, testExecutor)
			if tt.shouldFailAtLeastOne {
				assert.Error(t, oneErr, "ExecuteAtLeastOne should fail")
				assert.Nil(t, oneResults, "ExecuteAtLeastOne should return nil on failure")
			} else {
				assert.NoError(t, oneErr, "ExecuteAtLeastOne should succeed")
				assert.NotNil(t, oneResults, "ExecuteAtLeastOne should return results on success")
			}
		})
	}
}
