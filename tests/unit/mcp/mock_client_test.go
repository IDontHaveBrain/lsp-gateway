package mcp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"lsp-gateway/mcp"
	"lsp-gateway/tests/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockMcpClient_SendLSPRequest_DefaultBehavior(t *testing.T) {
	mockClient := mocks.NewMockMcpClient()

	ctx := context.Background()
	method := mcp.LSP_METHOD_WORKSPACE_SYMBOL
	params := map[string]interface{}{"query": "test"}

	result, err := mockClient.SendLSPRequest(ctx, method, params)

	require.NoError(t, err, "SendLSPRequest should not return an error by default")
	assert.NotNil(t, result, "Result should not be nil")

	var response map[string]interface{}
	err = json.Unmarshal(result, &response)
	require.NoError(t, err, "Result should be valid JSON")

	symbols, exists := response["symbols"]
	assert.True(t, exists, "Response should contain symbols")
	assert.NotNil(t, symbols, "Symbols should not be nil")

	assert.Len(t, mockClient.SendLSPRequestCalls, 1, "Should record one call")
	assert.Equal(t, method, mockClient.SendLSPRequestCalls[0].Method, "Method should match")
}

func TestMockMcpClient_SendLSPRequest_CustomFunction(t *testing.T) {
	mockClient := mocks.NewMockMcpClient()

	expectedResult := json.RawMessage(`{"custom": "response"}`)
	mockClient.SendLSPRequestFunc = func(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
		return expectedResult, nil
	}

	ctx := context.Background()
	result, err := mockClient.SendLSPRequest(ctx, "test/method", nil)

	require.NoError(t, err)
	assert.Equal(t, expectedResult, result, "Should return custom response")
}

func TestMockMcpClient_SendLSPRequest_QueuedResponses(t *testing.T) {
	mockClient := mocks.NewMockMcpClient()

	response1 := json.RawMessage(`{"response": 1}`)
	response2 := json.RawMessage(`{"response": 2}`)

	mockClient.QueueResponse(response1)
	mockClient.QueueResponse(response2)

	ctx := context.Background()

	result1, err1 := mockClient.SendLSPRequest(ctx, "test1", nil)
	require.NoError(t, err1)
	assert.Equal(t, response1, result1, "Should return first queued response")

	result2, err2 := mockClient.SendLSPRequest(ctx, "test2", nil)
	require.NoError(t, err2)
	assert.Equal(t, response2, result2, "Should return second queued response")
}

func TestMockMcpClient_SendLSPRequest_QueuedErrors(t *testing.T) {
	mockClient := mocks.NewMockMcpClient()

	expectedError := fmt.Errorf("mock error")
	mockClient.QueueError(expectedError)

	ctx := context.Background()
	result, err := mockClient.SendLSPRequest(ctx, "test", nil)

	assert.Nil(t, result, "Result should be nil on error")
	assert.Equal(t, expectedError, err, "Should return queued error")

	metrics := mockClient.GetMetrics()
	assert.Equal(t, int64(1), metrics.FailedRequests, "Should increment failed requests")
}

func TestMockMcpClient_GetMetrics(t *testing.T) {
	mockClient := mocks.NewMockMcpClient()

	mockClient.UpdateMetrics(10, 8, 2, 1, 0, time.Millisecond*150)

	metrics := mockClient.GetMetrics()

	assert.Equal(t, int64(10), metrics.TotalRequests, "Total requests should match")
	assert.Equal(t, int64(8), metrics.SuccessfulReqs, "Successful requests should match")
	assert.Equal(t, int64(2), metrics.FailedRequests, "Failed requests should match")
	assert.Equal(t, int64(1), metrics.TimeoutCount, "Timeout count should match")
	assert.Equal(t, time.Millisecond*150, metrics.AverageLatency, "Average latency should match")

	assert.Len(t, mockClient.GetMetricsCalls, 1, "Should record metrics call")
}

func TestMockMcpClient_HealthCheck(t *testing.T) {
	mockClient := mocks.NewMockMcpClient()

	assert.True(t, mockClient.IsHealthy(), "Should be healthy by default")

	ctx := context.Background()
	err := mockClient.GetHealth(ctx)
	assert.NoError(t, err, "Health check should pass by default")

	mockClient.SetHealthy(false)
	assert.False(t, mockClient.IsHealthy(), "Should be unhealthy after setting")

	err = mockClient.GetHealth(ctx)
	assert.Error(t, err, "Health check should fail when unhealthy")

	assert.Len(t, mockClient.IsHealthyCalls, 2, "Should record IsHealthy calls")
	assert.Len(t, mockClient.GetHealthCalls, 2, "Should record GetHealth calls")
}

func TestMockMcpClient_CircuitBreaker(t *testing.T) {
	mockClient := mocks.NewMockMcpClient()

	state := mockClient.GetCircuitBreakerState()
	assert.Equal(t, mcp.CircuitClosed, state, "Circuit breaker should be closed by default")

	mockClient.SetCircuitBreakerState(mcp.CircuitOpen)
	state = mockClient.GetCircuitBreakerState()
	assert.Equal(t, mcp.CircuitOpen, state, "Circuit breaker state should be updated")

	mockClient.SetCircuitBreakerConfig(10, time.Minute)
	cb := mockClient.GetCircuitBreaker()
	assert.Equal(t, 10, cb.MaxFailures, "Max failures should be updated")
	assert.Equal(t, time.Minute, cb.Timeout, "Timeout should be updated")

	assert.Len(t, mockClient.GetCircuitBreakerStateCalls, 2, "Should record state calls")
	assert.Len(t, mockClient.SetCircuitBreakerConfigCalls, 1, "Should record config calls")
}

func TestMockMcpClient_RetryPolicy(t *testing.T) {
	mockClient := mocks.NewMockMcpClient()

	newPolicy := &mcp.RetryPolicy{
		MaxRetries:     5,
		InitialBackoff: time.Second,
		MaxBackoff:     time.Minute,
		BackoffFactor:  1.5,
		JitterEnabled:  false,
	}

	mockClient.SetRetryPolicy(newPolicy)
	retrievedPolicy := mockClient.GetRetryPolicy()

	assert.Equal(t, 5, retrievedPolicy.MaxRetries, "Max retries should match")
	assert.Equal(t, time.Second, retrievedPolicy.InitialBackoff, "Initial backoff should match")
	assert.Equal(t, time.Minute, retrievedPolicy.MaxBackoff, "Max backoff should match")
	assert.Equal(t, 1.5, retrievedPolicy.BackoffFactor, "Backoff factor should match")
	assert.False(t, retrievedPolicy.JitterEnabled, "Jitter enabled should match")

	assert.Len(t, mockClient.SetRetryPolicyCalls, 1, "Should record set policy call")
	assert.Len(t, mockClient.GetRetryPolicyCalls, 1, "Should record get policy call")
}

func TestMockMcpClient_ErrorCategorization(t *testing.T) {
	mockClient := mocks.NewMockMcpClient()

	tests := []struct {
		error    error
		expected mcp.ErrorCategory
	}{
		{fmt.Errorf("network error"), mcp.ErrorCategoryNetwork},
		{fmt.Errorf("timeout"), mcp.ErrorCategoryTimeout},
		{fmt.Errorf("server error"), mcp.ErrorCategoryServer},
		{fmt.Errorf("client error"), mcp.ErrorCategoryClient},
		{fmt.Errorf("rate limit"), mcp.ErrorCategoryRateLimit},
		{fmt.Errorf("protocol error"), mcp.ErrorCategoryProtocol},
		{fmt.Errorf("unknown error"), mcp.ErrorCategoryUnknown},
		{nil, mcp.ErrorCategoryUnknown},
	}

	for _, test := range tests {
		category := mockClient.CategorizeError(test.error)
		assert.Equal(t, test.expected, category, "Error category should match for: %v", test.error)
	}

	assert.Len(t, mockClient.CategorizeErrorCalls, len(tests), "Should record all categorize calls")
}

func TestMockMcpClient_BackoffCalculation(t *testing.T) {
	mockClient := mocks.NewMockMcpClient()

	backoff0 := mockClient.CalculateBackoff(0)
	assert.Equal(t, time.Duration(0), backoff0, "Backoff for attempt 0 should be 0")

	backoff1 := mockClient.CalculateBackoff(1)
	assert.Equal(t, 500*time.Millisecond, backoff1, "Backoff for attempt 1 should be 500ms")

	backoff2 := mockClient.CalculateBackoff(2)
	assert.Equal(t, time.Second, backoff2, "Backoff for attempt 2 should be 1s")

	backoff100 := mockClient.CalculateBackoff(100)
	assert.Equal(t, 30*time.Second, backoff100, "Backoff should max out at 30s")

	assert.Len(t, mockClient.CalculateBackoffCalls, 4, "Should record all backoff calls")
}

func TestMockMcpClient_Configuration(t *testing.T) {
	mockClient := mocks.NewMockMcpClient()

	assert.Equal(t, "http://localhost:8080", mockClient.GetBaseURL(), "Default base URL should match")
	assert.Equal(t, 30*time.Second, mockClient.GetTimeout(), "Default timeout should match")
	assert.Equal(t, 3, mockClient.GetMaxRetries(), "Default max retries should match")

	mockClient.SetBaseURL("http://example.com:9090")
	mockClient.SetTimeout(time.Minute)
	mockClient.SetMaxRetries(5)

	assert.Equal(t, "http://example.com:9090", mockClient.GetBaseURL(), "Base URL should be updated")
	assert.Equal(t, time.Minute, mockClient.GetTimeout(), "Timeout should be updated")
	assert.Equal(t, 5, mockClient.GetMaxRetries(), "Max retries should be updated")
}

func TestMockMcpClient_CallTracking(t *testing.T) {
	mockClient := mocks.NewMockMcpClient()

	ctx := context.Background()

	_, _ = mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_WORKSPACE_SYMBOL, nil)
	_, _ = mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION, nil)
	_, _ = mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_WORKSPACE_SYMBOL, nil)

	assert.Equal(t, 2, mockClient.GetCallCount(mcp.LSP_METHOD_WORKSPACE_SYMBOL), "Should count workspace symbol calls")
	assert.Equal(t, 1, mockClient.GetCallCount(mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION), "Should count definition calls")

	lastCall := mockClient.GetLastCall()
	require.NotNil(t, lastCall, "Last call should not be nil")
	assert.Equal(t, mcp.LSP_METHOD_WORKSPACE_SYMBOL, lastCall.Method, "Last call method should match")

	symbolCalls := mockClient.GetCallsForMethod(mcp.LSP_METHOD_WORKSPACE_SYMBOL)
	assert.Len(t, symbolCalls, 2, "Should return all calls for workspace symbol")
}

func TestMockMcpClient_DefaultResponses(t *testing.T) {
	mockClient := mocks.NewMockMcpClient()
	ctx := context.Background()

	tests := []struct {
		method   string
		expected string
	}{
		{mcp.LSP_METHOD_WORKSPACE_SYMBOL, "symbols"},
		{mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION, "uri"},
		{mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER, "contents"},
		{"custom/method", "result"},
	}

	for _, test := range tests {
		result, err := mockClient.SendLSPRequest(ctx, test.method, nil)
		require.NoError(t, err, "Should not error for method: %s", test.method)

		var response map[string]interface{}
		err = json.Unmarshal(result, &response)
		require.NoError(t, err, "Should unmarshal response for: %s", test.method)

		_, exists := response[test.expected]
		assert.True(t, exists, "Response should contain %s for method %s", test.expected, test.method)
	}
}

func TestMockMcpClient_Reset(t *testing.T) {
	mockClient := mocks.NewMockMcpClient()

	ctx := context.Background()
	_, _ = mockClient.SendLSPRequest(ctx, "test", nil)
	mockClient.GetMetrics()
	mockClient.IsHealthy()

	assert.Len(t, mockClient.SendLSPRequestCalls, 1, "Should have calls before reset")
	assert.Len(t, mockClient.GetMetricsCalls, 1, "Should have metrics calls before reset")
	assert.Len(t, mockClient.IsHealthyCalls, 1, "Should have health calls before reset")

	mockClient.Reset()

	assert.Len(t, mockClient.SendLSPRequestCalls, 0, "Should clear calls after reset")
	assert.Len(t, mockClient.GetMetricsCalls, 0, "Should clear metrics calls after reset")
	assert.Len(t, mockClient.IsHealthyCalls, 0, "Should clear health calls after reset")

	result, err := mockClient.SendLSPRequest(ctx, "test", nil)
	require.NoError(t, err, "Should work after reset")
	assert.NotNil(t, result, "Should return result after reset")
}

func TestMockMcpClient_ThreadSafety(t *testing.T) {
	mockClient := mocks.NewMockMcpClient()

	ctx := context.Background()
	done := make(chan bool)

	// Run concurrent operations
	for i := 0; i < 10; i++ {
		go func(i int) {
			defer func() { done <- true }()

			method := fmt.Sprintf("test/method/%d", i)
			_, _ = mockClient.SendLSPRequest(ctx, method, nil)
			mockClient.GetMetrics()
			mockClient.IsHealthy()
			mockClient.SetHealthy(i%2 == 0)
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	assert.Equal(t, 10, len(mockClient.SendLSPRequestCalls), "Should record all concurrent calls")
	assert.Equal(t, 10, len(mockClient.GetMetricsCalls), "Should record all metrics calls")
	assert.Equal(t, 10, len(mockClient.IsHealthyCalls), "Should record all health calls")
}
