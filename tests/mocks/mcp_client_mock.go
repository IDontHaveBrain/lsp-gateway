package mocks

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"lsp-gateway/mcp"
)

type MockMcpClient struct {
	mu                         sync.RWMutex
	requests                   []MCPRequest
	queuedResponses            []json.RawMessage
	queuedErrors               []error
	callCounts                 map[string]int
	isHealthy                  bool
	SendLSPRequestCalls        []MCPRequestCall
	UpdateMetricsCalls         []UpdateMetricsCall
	SetCircuitBreakerConfigCalls []CircuitBreakerConfigCall
	
	// Customizable function for SendLSPRequest behavior
	SendLSPRequestFunc         func(ctx context.Context, method string, params interface{}) (json.RawMessage, error)
	
	// Additional call tracking
	GetMetricsCalls            []interface{}
	IsHealthyCalls             []interface{}
	GetHealthCalls             []GetHealthCall
	GetCircuitBreakerStateCalls []interface{}
	SetRetryPolicyCalls        []SetRetryPolicyCall
	GetRetryPolicyCalls        []interface{}
	CategorizeErrorCalls       []CategorizeErrorCall
	CalculateBackoffCalls      []CalculateBackoffCall
	
	// Configuration state
	baseURL           string
	timeout           time.Duration
	maxRetries        int
	circuitBreakerState mcp.CircuitBreakerState
	retryPolicy       *mcp.RetryPolicy
	metrics           *mcp.ConnectionMetrics
}

type MCPRequestCall struct {
	Method string      `json:"method"`
	Params interface{} `json:"params"`
	Ctx    context.Context
}

type MCPRequest struct {
	Method string      `json:"method"`
	Params interface{} `json:"params"`
}

type UpdateMetricsCall struct {
	TotalRequests    int
	SuccessfulRequests int
	FailedRequests   int
	ErrorCount       int
	WarningCount     int
	AverageLatency   time.Duration
}

type CircuitBreakerConfigCall struct {
	ErrorThreshold   int
	TimeoutDuration  time.Duration
}

type GetHealthCall struct {
	Ctx context.Context
}

type SetRetryPolicyCall struct {
	Policy *mcp.RetryPolicy
}

type CategorizeErrorCall struct {
	Error error
}

type CalculateBackoffCall struct {
	Attempt int
}

func NewMockMcpClient() *MockMcpClient {
	return &MockMcpClient{
		requests:                   make([]MCPRequest, 0),
		queuedResponses:            make([]json.RawMessage, 0),
		queuedErrors:               make([]error, 0),
		callCounts:                 make(map[string]int),
		isHealthy:                  true,
		SendLSPRequestCalls:        make([]MCPRequestCall, 0),
		UpdateMetricsCalls:         make([]UpdateMetricsCall, 0),
		SetCircuitBreakerConfigCalls: make([]CircuitBreakerConfigCall, 0),
		
		// Initialize additional call tracking
		GetMetricsCalls:            make([]interface{}, 0),
		IsHealthyCalls:             make([]interface{}, 0),
		GetHealthCalls:             make([]GetHealthCall, 0),
		GetCircuitBreakerStateCalls: make([]interface{}, 0),
		SetRetryPolicyCalls:        make([]SetRetryPolicyCall, 0),
		GetRetryPolicyCalls:        make([]interface{}, 0),
		CategorizeErrorCalls:       make([]CategorizeErrorCall, 0),
		CalculateBackoffCalls:      make([]CalculateBackoffCall, 0),
		
		// Default configuration
		baseURL:           "http://localhost:8080",
		timeout:           30 * time.Second,
		maxRetries:        3,
		circuitBreakerState: mcp.CircuitClosed,
		retryPolicy: &mcp.RetryPolicy{
			MaxRetries:     3,
			InitialBackoff: 100 * time.Millisecond,
			MaxBackoff:     30 * time.Second,
			BackoffFactor:  2.0,
			JitterEnabled:  true,
		},
		metrics: &mcp.ConnectionMetrics{
			TotalRequests:  0,
			SuccessfulReqs: 0,
			FailedRequests: 0,
			TimeoutCount:   0,
		},
	}
}

func (m *MockMcpClient) SendLSPRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Record the request
	m.requests = append(m.requests, MCPRequest{Method: method, Params: params})
	m.SendLSPRequestCalls = append(m.SendLSPRequestCalls, MCPRequestCall{
		Method: method,
		Params: params,
		Ctx:    ctx,
	})

	// Increment call count
	m.callCounts[method]++

	// Use custom function if provided
	if m.SendLSPRequestFunc != nil {
		return m.SendLSPRequestFunc(ctx, method, params)
	}

	// Check if we have queued errors first
	if len(m.queuedErrors) > 0 {
		err := m.queuedErrors[0]
		m.queuedErrors = m.queuedErrors[1:]
		m.metrics.FailedRequests++
		return nil, err
	}

	// Check if we have queued responses
	if len(m.queuedResponses) > 0 {
		response := m.queuedResponses[0]
		m.queuedResponses = m.queuedResponses[1:]  // Always consume the response
		// Return queued response regardless of compatibility
		m.metrics.SuccessfulReqs++
		return response, nil
	}

	// Fall back to default responses
	m.metrics.SuccessfulReqs++
	switch method {
	case "textDocument/definition":
		return json.RawMessage(`{
			"uri": "file:///workspace/test.go",
			"range": {
				"start": {"line": 10, "character": 0},
				"end": {"line": 10, "character": 10}
			}
		}`), nil
	case "textDocument/hover":
		return json.RawMessage(`{
			"contents": {
				"kind": "markdown",
				"value": "Test hover content with documentation."
			},
			"range": {
				"start": {"line": 10, "character": 0},
				"end": {"line": 10, "character": 10}
			}
		}`), nil
	case "textDocument/references":
		return json.RawMessage(`[
			{
				"uri": "file:///workspace/test.go",
				"range": {
					"start": {"line": 5, "character": 0},
					"end": {"line": 5, "character": 10}
				}
			},
			{
				"uri": "file:///workspace/other.go",
				"range": {
					"start": {"line": 15, "character": 5},
					"end": {"line": 15, "character": 15}
				}
			}
		]`), nil
	case "textDocument/documentSymbol":
		return json.RawMessage(`[
			{
				"name": "TestClass",
				"kind": 5,
				"detail": "class",
				"range": {
					"start": {"line": 1, "character": 0},
					"end": {"line": 10, "character": 1}
				},
				"selectionRange": {
					"start": {"line": 1, "character": 6},
					"end": {"line": 1, "character": 15}
				},
				"children": [
					{
						"name": "testMethod",
						"kind": 6,
						"detail": "method",
						"range": {
							"start": {"line": 3, "character": 4},
							"end": {"line": 5, "character": 5}
						},
						"selectionRange": {
							"start": {"line": 3, "character": 4},
							"end": {"line": 3, "character": 14}
						}
					},
					{
						"name": "testProperty",
						"kind": 7,
						"detail": "field",
						"range": {
							"start": {"line": 2, "character": 4},
							"end": {"line": 2, "character": 16}
						},
						"selectionRange": {
							"start": {"line": 2, "character": 4},
							"end": {"line": 2, "character": 16}
						}
					}
				]
			},
			{
				"name": "TestFunction",
				"kind": 12,
				"detail": "function",
				"range": {
					"start": {"line": 12, "character": 0},
					"end": {"line": 15, "character": 1}
				},
				"selectionRange": {
					"start": {"line": 12, "character": 9},
					"end": {"line": 12, "character": 21}
				}
			},
			{
				"name": "TestInterface",
				"kind": 11,
				"detail": "interface",
				"range": {
					"start": {"line": 17, "character": 0},
					"end": {"line": 20, "character": 1}
				},
				"selectionRange": {
					"start": {"line": 17, "character": 10},
					"end": {"line": 17, "character": 23}
				}
			},
			{
				"name": "TestEnum",
				"kind": 10,
				"detail": "enum",
				"range": {
					"start": {"line": 22, "character": 0},
					"end": {"line": 26, "character": 1}
				},
				"selectionRange": {
					"start": {"line": 22, "character": 5},
					"end": {"line": 22, "character": 13}
				}
			}
		]`), nil
	case "workspace/symbol":
		return json.RawMessage(`[
			{
				"name": "WorkspaceSymbol",
				"kind": 5,
				"location": {
					"uri": "file:///workspace/test.go",
					"range": {
						"start": {"line": 1, "character": 0},
						"end": {"line": 1, "character": 15}
					}
				},
				"containerName": "TestPackage"
			}
		]`), nil
	case "textDocument/completion":
		return json.RawMessage(`{
			"isIncomplete": false,
			"items": [
				{
					"label": "testCompletion",
					"kind": 1,
					"detail": "test completion item",
					"documentation": {
						"kind": "markdown",
						"value": "Test completion documentation"
					},
					"insertText": "testCompletion",
					"insertTextFormat": 1
				}
			]
		}`), nil
	default:
		return json.RawMessage(`{"result":"success"}`), nil
	}
}

func (m *MockMcpClient) GetRequests() []MCPRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]MCPRequest{}, m.requests...)
}

func (m *MockMcpClient) ClearRequests() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests = m.requests[:0]
}

// SetHealthy sets the health status of the mock client
func (m *MockMcpClient) SetHealthy(healthy bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.isHealthy = healthy
}

// IsHealthy returns the current health status
func (m *MockMcpClient) IsHealthy() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.IsHealthyCalls = append(m.IsHealthyCalls, struct{}{})
	return m.isHealthy
}

// Reset clears all state of the mock client
func (m *MockMcpClient) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requests = m.requests[:0]
	m.queuedResponses = m.queuedResponses[:0]
	m.queuedErrors = m.queuedErrors[:0]
	m.callCounts = make(map[string]int)
	m.SendLSPRequestCalls = m.SendLSPRequestCalls[:0]
	m.UpdateMetricsCalls = m.UpdateMetricsCalls[:0]
	m.SetCircuitBreakerConfigCalls = m.SetCircuitBreakerConfigCalls[:0]
	
	// Reset additional call tracking
	m.GetMetricsCalls = m.GetMetricsCalls[:0]
	m.IsHealthyCalls = m.IsHealthyCalls[:0]
	m.GetHealthCalls = m.GetHealthCalls[:0]
	m.GetCircuitBreakerStateCalls = m.GetCircuitBreakerStateCalls[:0]
	m.SetRetryPolicyCalls = m.SetRetryPolicyCalls[:0]
	m.GetRetryPolicyCalls = m.GetRetryPolicyCalls[:0]
	m.CategorizeErrorCalls = m.CategorizeErrorCalls[:0]
	m.CalculateBackoffCalls = m.CalculateBackoffCalls[:0]
	
	// Reset state
	m.isHealthy = true
	m.SendLSPRequestFunc = nil
	m.metrics = &mcp.ConnectionMetrics{
		TotalRequests:  0,
		SuccessfulReqs: 0,
		FailedRequests: 0,
		TimeoutCount:   0,
	}
}

// QueueResponse adds a response to the queue that will be returned by SendLSPRequest
func (m *MockMcpClient) QueueResponse(response json.RawMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queuedResponses = append(m.queuedResponses, response)
}

// GetCallCount returns the number of times a specific method was called
func (m *MockMcpClient) GetCallCount(method string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.callCounts[method]
}

// GetLastCall returns the last call made to SendLSPRequest
func (m *MockMcpClient) GetLastCall() *MCPRequestCall {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.SendLSPRequestCalls) == 0 {
		return nil
	}
	return &m.SendLSPRequestCalls[len(m.SendLSPRequestCalls)-1]
}

// GetQueuedResponsesCount returns the number of queued responses remaining
func (m *MockMcpClient) GetQueuedResponsesCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.queuedResponses)
}

// GetTotalCallCount returns the total number of calls made
func (m *MockMcpClient) GetTotalCallCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.SendLSPRequestCalls)
}


// QueueError adds an error to the queue that will be returned by SendLSPRequest
func (m *MockMcpClient) QueueError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queuedErrors = append(m.queuedErrors, err)
}

// UpdateMetrics simulates updating performance metrics
func (m *MockMcpClient) UpdateMetrics(totalRequests, successfulRequests, failedRequests, errorCount, warningCount int, averageLatency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Record the call for testing purposes
	m.UpdateMetricsCalls = append(m.UpdateMetricsCalls, UpdateMetricsCall{
		TotalRequests:      totalRequests,
		SuccessfulRequests: successfulRequests,
		FailedRequests:     failedRequests,
		ErrorCount:         errorCount,
		WarningCount:       warningCount,
		AverageLatency:     averageLatency,
	})
	
	// Actually update the metrics values
	m.metrics.TotalRequests = int64(totalRequests)
	m.metrics.SuccessfulReqs = int64(successfulRequests)
	m.metrics.FailedRequests = int64(failedRequests)
	m.metrics.TimeoutCount = int64(errorCount)
	m.metrics.AverageLatency = averageLatency
}

// SetCircuitBreakerConfig simulates setting circuit breaker configuration
func (m *MockMcpClient) SetCircuitBreakerConfig(errorThreshold int, timeoutDuration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SetCircuitBreakerConfigCalls = append(m.SetCircuitBreakerConfigCalls, CircuitBreakerConfigCall{
		ErrorThreshold:  errorThreshold,
		TimeoutDuration: timeoutDuration,
	})
}

// GetMetrics returns the current connection metrics
func (m *MockMcpClient) GetMetrics() mcp.ConnectionMetrics {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.GetMetricsCalls = append(m.GetMetricsCalls, struct{}{})
	return *m.metrics
}

// GetHealth performs a health check
func (m *MockMcpClient) GetHealth(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.GetHealthCalls = append(m.GetHealthCalls, GetHealthCall{Ctx: ctx})
	if !m.isHealthy {
		return fmt.Errorf("health check failed: service is unhealthy")
	}
	return nil
}

// GetCircuitBreakerState returns the current circuit breaker state
func (m *MockMcpClient) GetCircuitBreakerState() mcp.CircuitBreakerState {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.GetCircuitBreakerStateCalls = append(m.GetCircuitBreakerStateCalls, struct{}{})
	return m.circuitBreakerState
}

// SetCircuitBreakerState sets the circuit breaker state
func (m *MockMcpClient) SetCircuitBreakerState(state mcp.CircuitBreakerState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.circuitBreakerState = state
}

// GetCircuitBreaker returns the circuit breaker configuration
func (m *MockMcpClient) GetCircuitBreaker() *mcp.CircuitBreaker {
	m.mu.Lock()
	defer m.mu.Unlock()
	return &mcp.CircuitBreaker{
		MaxFailures: 10,
		Timeout:     time.Minute,
	}
}

// SetRetryPolicy sets the retry policy
func (m *MockMcpClient) SetRetryPolicy(policy *mcp.RetryPolicy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SetRetryPolicyCalls = append(m.SetRetryPolicyCalls, SetRetryPolicyCall{Policy: policy})
	m.retryPolicy = policy
}

// GetRetryPolicy returns the current retry policy
func (m *MockMcpClient) GetRetryPolicy() *mcp.RetryPolicy {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.GetRetryPolicyCalls = append(m.GetRetryPolicyCalls, struct{}{})
	return m.retryPolicy
}

// CategorizeError categorizes an error by type
func (m *MockMcpClient) CategorizeError(err error) mcp.ErrorCategory {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CategorizeErrorCalls = append(m.CategorizeErrorCalls, CategorizeErrorCall{Error: err})
	
	if err == nil {
		return mcp.ErrorCategoryUnknown
	}
	
	errStr := err.Error()
	switch {
	case strings.Contains(errStr, "network"):
		return mcp.ErrorCategoryNetwork
	case strings.Contains(errStr, "timeout"):
		return mcp.ErrorCategoryTimeout
	case strings.Contains(errStr, "server error"):
		return mcp.ErrorCategoryServer
	case strings.Contains(errStr, "client error"):
		return mcp.ErrorCategoryClient
	case strings.Contains(errStr, "rate limit"):
		return mcp.ErrorCategoryRateLimit
	case strings.Contains(errStr, "protocol error"):
		return mcp.ErrorCategoryProtocol
	default:
		return mcp.ErrorCategoryUnknown
	}
}

// CalculateBackoff calculates backoff time for the given attempt
func (m *MockMcpClient) CalculateBackoff(attempt int) time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CalculateBackoffCalls = append(m.CalculateBackoffCalls, CalculateBackoffCall{Attempt: attempt})
	
	if attempt == 0 {
		return 0
	}
	if attempt == 1 {
		return 500 * time.Millisecond
	}
	if attempt == 2 {
		return time.Second
	}
	// Max out at 30 seconds
	return 30 * time.Second
}

// Configuration methods
func (m *MockMcpClient) GetBaseURL() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.baseURL
}

func (m *MockMcpClient) SetBaseURL(url string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.baseURL = url
}

func (m *MockMcpClient) GetTimeout() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.timeout
}

func (m *MockMcpClient) SetTimeout(timeout time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.timeout = timeout
}

func (m *MockMcpClient) GetMaxRetries() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.maxRetries
}

func (m *MockMcpClient) SetMaxRetries(retries int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.maxRetries = retries
}

// GetCallsForMethod returns all calls for a specific method
func (m *MockMcpClient) GetCallsForMethod(method string) []MCPRequestCall {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var calls []MCPRequestCall
	for _, call := range m.SendLSPRequestCalls {
		if call.Method == method {
			calls = append(calls, call)
		}
	}
	return calls
}