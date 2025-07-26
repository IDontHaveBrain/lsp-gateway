package mocks

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"lsp-gateway/mcp"
)

type MockMcpClient struct {
	mu sync.RWMutex

	SendLSPRequestFunc          func(ctx context.Context, method string, params interface{}) (json.RawMessage, error)
	GetMetricsFunc              func() mcp.ConnectionMetrics
	GetHealthFunc               func(ctx context.Context) error
	IsHealthyFunc               func() bool
	SetRetryPolicyFunc          func(policy *mcp.RetryPolicy)
	GetBaseURLFunc              func() string
	GetTimeoutFunc              func() time.Duration
	GetMaxRetriesFunc           func() int
	GetCircuitBreakerStateFunc  func() mcp.CircuitBreakerState
	SetCircuitBreakerConfigFunc func(maxFailures int, timeout time.Duration)
	GetRetryPolicyFunc          func() mcp.RetryPolicy
	GetCircuitBreakerFunc       func() *mcp.CircuitBreaker
	GetMetricsPointerFunc       func() *mcp.ConnectionMetrics
	CategorizeErrorFunc         func(err error) mcp.ErrorCategory
	CalculateBackoffFunc        func(attempt int) time.Duration
	GetHTTPClientFunc           func() *http.Client

	SendLSPRequestCalls          []SendLSPRequestCall
	GetMetricsCalls              []interface{}
	GetHealthCalls               []context.Context
	IsHealthyCalls               []interface{}
	SetRetryPolicyCalls          []*mcp.RetryPolicy
	GetBaseURLCalls              []interface{}
	GetTimeoutCalls              []interface{}
	GetMaxRetriesCalls           []interface{}
	GetCircuitBreakerStateCalls  []interface{}
	SetCircuitBreakerConfigCalls []CircuitBreakerConfigCall
	GetRetryPolicyCalls          []interface{}
	GetCircuitBreakerCalls       []interface{}
	GetMetricsPointerCalls       []interface{}
	CategorizeErrorCalls         []error
	CalculateBackoffCalls        []int
	GetHTTPClientCalls           []interface{}

	mockMetrics        mcp.ConnectionMetrics
	mockCircuitBreaker *mcp.CircuitBreaker
	mockRetryPolicy    *mcp.RetryPolicy
	mockBaseURL        string
	mockTimeout        time.Duration
	mockMaxRetries     int
	mockHTTPClient     *http.Client
	mockHealthy        bool
	mockLogger         *log.Logger

	responseQueue []json.RawMessage
	errorQueue    []error
	responseIndex int
	errorIndex    int
}

type SendLSPRequestCall struct {
	Ctx    context.Context
	Method string
	Params interface{}
}

type CircuitBreakerConfigCall struct {
	MaxFailures int
	Timeout     time.Duration
}

func NewMockMcpClient() *MockMcpClient {
	defaultMetrics := mcp.ConnectionMetrics{
		TotalRequests:    0,
		SuccessfulReqs:   0,
		FailedRequests:   0,
		TimeoutCount:     0,
		ConnectionErrors: 0,
		AverageLatency:   time.Millisecond * 100,
		LastRequestTime:  time.Now(),
		LastSuccessTime:  time.Now(),
	}

	defaultCircuitBreaker := &mcp.CircuitBreaker{
		State:       mcp.CircuitClosed,
		Timeout:     60 * time.Second,
		MaxFailures: 5,
		MaxRequests: 3,
	}

	defaultRetryPolicy := &mcp.RetryPolicy{
		MaxRetries:     3,
		InitialBackoff: 500 * time.Millisecond,
		MaxBackoff:     30 * time.Second,
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

	mockHTTPClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	return &MockMcpClient{
		SendLSPRequestCalls:          make([]SendLSPRequestCall, 0),
		GetMetricsCalls:              make([]interface{}, 0),
		GetHealthCalls:               make([]context.Context, 0),
		IsHealthyCalls:               make([]interface{}, 0),
		SetRetryPolicyCalls:          make([]*mcp.RetryPolicy, 0),
		GetBaseURLCalls:              make([]interface{}, 0),
		GetTimeoutCalls:              make([]interface{}, 0),
		GetMaxRetriesCalls:           make([]interface{}, 0),
		GetCircuitBreakerStateCalls:  make([]interface{}, 0),
		SetCircuitBreakerConfigCalls: make([]CircuitBreakerConfigCall, 0),
		GetRetryPolicyCalls:          make([]interface{}, 0),
		GetCircuitBreakerCalls:       make([]interface{}, 0),
		GetMetricsPointerCalls:       make([]interface{}, 0),
		CategorizeErrorCalls:         make([]error, 0),
		CalculateBackoffCalls:        make([]int, 0),
		GetHTTPClientCalls:           make([]interface{}, 0),

		mockMetrics:        mcp.ConnectionMetrics{
			TotalRequests:    defaultMetrics.TotalRequests,
			SuccessfulReqs:   defaultMetrics.SuccessfulReqs,
			FailedRequests:   defaultMetrics.FailedRequests,
			TimeoutCount:     defaultMetrics.TimeoutCount,
			ConnectionErrors: defaultMetrics.ConnectionErrors,
			AverageLatency:   defaultMetrics.AverageLatency,
			LastRequestTime:  defaultMetrics.LastRequestTime,
			LastSuccessTime:  defaultMetrics.LastSuccessTime,
		},
		mockCircuitBreaker: defaultCircuitBreaker,
		mockRetryPolicy:    defaultRetryPolicy,
		mockBaseURL:        "http://localhost:8080",
		mockTimeout:        30 * time.Second,
		mockMaxRetries:     3,
		mockHTTPClient:     mockHTTPClient,
		mockHealthy:        true,
		mockLogger:         log.New(log.Writer(), "[MockMCPClient] ", log.LstdFlags|log.Lshortfile),

		responseQueue: make([]json.RawMessage, 0),
		errorQueue:    make([]error, 0),
		responseIndex: 0,
		errorIndex:    0,
	}
}

func (m *MockMcpClient) SendLSPRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	call := SendLSPRequestCall{
		Ctx:    ctx,
		Method: method,
		Params: params,
	}
	m.SendLSPRequestCalls = append(m.SendLSPRequestCalls, call)

	if m.SendLSPRequestFunc != nil {
		return m.SendLSPRequestFunc(ctx, method, params)
	}

	if m.errorIndex < len(m.errorQueue) {
		err := m.errorQueue[m.errorIndex]
		m.errorIndex++
		m.mockMetrics.FailedRequests++
		return nil, err
	}

	if m.responseIndex < len(m.responseQueue) {
		response := m.responseQueue[m.responseIndex]
		m.responseIndex++
		m.mockMetrics.SuccessfulReqs++
		return response, nil
	}

	m.mockMetrics.SuccessfulReqs++
	return m.getDefaultResponse(method), nil
}

func (m *MockMcpClient) GetMetrics() mcp.ConnectionMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.GetMetricsCalls = append(m.GetMetricsCalls, struct{}{})
	if m.GetMetricsFunc != nil {
		return m.GetMetricsFunc()
	}
	return mcp.ConnectionMetrics{
		TotalRequests:    m.mockMetrics.TotalRequests,
		SuccessfulReqs:   m.mockMetrics.SuccessfulReqs,
		FailedRequests:   m.mockMetrics.FailedRequests,
		TimeoutCount:     m.mockMetrics.TimeoutCount,
		ConnectionErrors: m.mockMetrics.ConnectionErrors,
		AverageLatency:   m.mockMetrics.AverageLatency,
		LastRequestTime:  m.mockMetrics.LastRequestTime,
		LastSuccessTime:  m.mockMetrics.LastSuccessTime,
	}
}

func (m *MockMcpClient) GetHealth(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.GetHealthCalls = append(m.GetHealthCalls, ctx)
	if m.GetHealthFunc != nil {
		return m.GetHealthFunc(ctx)
	}
	if !m.mockHealthy {
		return fmt.Errorf("mock health check failed")
	}
	return nil
}

func (m *MockMcpClient) IsHealthy() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.IsHealthyCalls = append(m.IsHealthyCalls, struct{}{})
	if m.IsHealthyFunc != nil {
		return m.IsHealthyFunc()
	}
	return m.mockHealthy
}

func (m *MockMcpClient) SetRetryPolicy(policy *mcp.RetryPolicy) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SetRetryPolicyCalls = append(m.SetRetryPolicyCalls, policy)
	if m.SetRetryPolicyFunc != nil {
		m.SetRetryPolicyFunc(policy)
		return
	}
	m.mockRetryPolicy = policy
}

func (m *MockMcpClient) GetBaseURL() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.GetBaseURLCalls = append(m.GetBaseURLCalls, struct{}{})
	if m.GetBaseURLFunc != nil {
		return m.GetBaseURLFunc()
	}
	return m.mockBaseURL
}

func (m *MockMcpClient) GetTimeout() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.GetTimeoutCalls = append(m.GetTimeoutCalls, struct{}{})
	if m.GetTimeoutFunc != nil {
		return m.GetTimeoutFunc()
	}
	return m.mockTimeout
}

func (m *MockMcpClient) GetMaxRetries() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.GetMaxRetriesCalls = append(m.GetMaxRetriesCalls, struct{}{})
	if m.GetMaxRetriesFunc != nil {
		return m.GetMaxRetriesFunc()
	}
	return m.mockMaxRetries
}

func (m *MockMcpClient) GetCircuitBreakerState() mcp.CircuitBreakerState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.GetCircuitBreakerStateCalls = append(m.GetCircuitBreakerStateCalls, struct{}{})
	if m.GetCircuitBreakerStateFunc != nil {
		return m.GetCircuitBreakerStateFunc()
	}
	return m.mockCircuitBreaker.State
}

func (m *MockMcpClient) SetCircuitBreakerConfig(maxFailures int, timeout time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	call := CircuitBreakerConfigCall{
		MaxFailures: maxFailures,
		Timeout:     timeout,
	}
	m.SetCircuitBreakerConfigCalls = append(m.SetCircuitBreakerConfigCalls, call)

	if m.SetCircuitBreakerConfigFunc != nil {
		m.SetCircuitBreakerConfigFunc(maxFailures, timeout)
		return
	}

	m.mockCircuitBreaker.MaxFailures = maxFailures
	m.mockCircuitBreaker.Timeout = timeout
}

func (m *MockMcpClient) GetRetryPolicy() mcp.RetryPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.GetRetryPolicyCalls = append(m.GetRetryPolicyCalls, struct{}{})
	if m.GetRetryPolicyFunc != nil {
		return m.GetRetryPolicyFunc()
	}
	if m.mockRetryPolicy == nil {
		return mcp.RetryPolicy{}
	}
	return *m.mockRetryPolicy
}

func (m *MockMcpClient) GetCircuitBreaker() *mcp.CircuitBreaker {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.GetCircuitBreakerCalls = append(m.GetCircuitBreakerCalls, struct{}{})
	if m.GetCircuitBreakerFunc != nil {
		return m.GetCircuitBreakerFunc()
	}
	return m.mockCircuitBreaker
}

func (m *MockMcpClient) GetMetricsPointer() *mcp.ConnectionMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.GetMetricsPointerCalls = append(m.GetMetricsPointerCalls, struct{}{})
	if m.GetMetricsPointerFunc != nil {
		return m.GetMetricsPointerFunc()
	}
	return &m.mockMetrics
}

func (m *MockMcpClient) CategorizeError(err error) mcp.ErrorCategory {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CategorizeErrorCalls = append(m.CategorizeErrorCalls, err)
	if m.CategorizeErrorFunc != nil {
		return m.CategorizeErrorFunc(err)
	}

	if err == nil {
		return mcp.ErrorCategoryUnknown
	}

	switch err.Error() {
	case "network error":
		return mcp.ErrorCategoryNetwork
	case "timeout":
		return mcp.ErrorCategoryTimeout
	case "server error":
		return mcp.ErrorCategoryServer
	case "client error":
		return mcp.ErrorCategoryClient
	case "rate limit":
		return mcp.ErrorCategoryRateLimit
	case "protocol error":
		return mcp.ErrorCategoryProtocol
	default:
		return mcp.ErrorCategoryUnknown
	}
}

func (m *MockMcpClient) CalculateBackoff(attempt int) time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CalculateBackoffCalls = append(m.CalculateBackoffCalls, attempt)
	if m.CalculateBackoffFunc != nil {
		return m.CalculateBackoffFunc(attempt)
	}

	if attempt <= 0 {
		return 0
	}

	backoff := time.Duration(500*attempt) * time.Millisecond
	if backoff > 30*time.Second {
		backoff = 30 * time.Second
	}
	return backoff
}

func (m *MockMcpClient) GetHTTPClient() *http.Client {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.GetHTTPClientCalls = append(m.GetHTTPClientCalls, struct{}{})
	if m.GetHTTPClientFunc != nil {
		return m.GetHTTPClientFunc()
	}
	return m.mockHTTPClient
}

func (m *MockMcpClient) QueueResponse(response json.RawMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responseQueue = append(m.responseQueue, response)
}

func (m *MockMcpClient) QueueError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errorQueue = append(m.errorQueue, err)
}

func (m *MockMcpClient) SetHealthy(healthy bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mockHealthy = healthy
}

func (m *MockMcpClient) SetBaseURL(url string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mockBaseURL = url
}

func (m *MockMcpClient) SetTimeout(timeout time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mockTimeout = timeout
}

func (m *MockMcpClient) SetMaxRetries(maxRetries int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mockMaxRetries = maxRetries
}

func (m *MockMcpClient) SetCircuitBreakerState(state mcp.CircuitBreakerState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mockCircuitBreaker.State = state
}

func (m *MockMcpClient) UpdateMetrics(totalReq, successReq, failedReq, timeoutCount, connErrors int64, avgLatency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.mockMetrics.TotalRequests = totalReq
	m.mockMetrics.SuccessfulReqs = successReq
	m.mockMetrics.FailedRequests = failedReq
	m.mockMetrics.TimeoutCount = timeoutCount
	m.mockMetrics.ConnectionErrors = connErrors
	m.mockMetrics.AverageLatency = avgLatency
	m.mockMetrics.LastRequestTime = time.Now()
	if successReq > 0 {
		m.mockMetrics.LastSuccessTime = time.Now()
	}
}

func (m *MockMcpClient) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.SendLSPRequestCalls = make([]SendLSPRequestCall, 0)
	m.GetMetricsCalls = make([]interface{}, 0)
	m.GetHealthCalls = make([]context.Context, 0)
	m.IsHealthyCalls = make([]interface{}, 0)
	m.SetRetryPolicyCalls = make([]*mcp.RetryPolicy, 0)
	m.GetBaseURLCalls = make([]interface{}, 0)
	m.GetTimeoutCalls = make([]interface{}, 0)
	m.GetMaxRetriesCalls = make([]interface{}, 0)
	m.GetCircuitBreakerStateCalls = make([]interface{}, 0)
	m.SetCircuitBreakerConfigCalls = make([]CircuitBreakerConfigCall, 0)
	m.GetRetryPolicyCalls = make([]interface{}, 0)
	m.GetCircuitBreakerCalls = make([]interface{}, 0)
	m.GetMetricsPointerCalls = make([]interface{}, 0)
	m.CategorizeErrorCalls = make([]error, 0)
	m.CalculateBackoffCalls = make([]int, 0)
	m.GetHTTPClientCalls = make([]interface{}, 0)

	m.responseQueue = make([]json.RawMessage, 0)
	m.errorQueue = make([]error, 0)
	m.responseIndex = 0
	m.errorIndex = 0

	m.SendLSPRequestFunc = nil
	m.GetMetricsFunc = nil
	m.GetHealthFunc = nil
	m.IsHealthyFunc = nil
	m.SetRetryPolicyFunc = nil
	m.GetBaseURLFunc = nil
	m.GetTimeoutFunc = nil
	m.GetMaxRetriesFunc = nil
	m.GetCircuitBreakerStateFunc = nil
	m.SetCircuitBreakerConfigFunc = nil
	m.GetRetryPolicyFunc = nil
	m.GetCircuitBreakerFunc = nil
	m.GetMetricsPointerFunc = nil
	m.CategorizeErrorFunc = nil
	m.CalculateBackoffFunc = nil
	m.GetHTTPClientFunc = nil
}

func (m *MockMcpClient) getDefaultResponse(method string) json.RawMessage {
	switch method {
	case mcp.LSP_METHOD_WORKSPACE_SYMBOL:
		return json.RawMessage(`{"symbols": [{"name": "MockSymbol", "kind": 1}]}`)
	case mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION:
		return json.RawMessage(`{"uri": "file://mock.go", "range": {"start": {"line": 0, "character": 0}, "end": {"line": 0, "character": 10}}}`)
	case mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES:
		return json.RawMessage(`[{"uri": "file://mock.go", "range": {"start": {"line": 0, "character": 0}, "end": {"line": 0, "character": 10}}}]`)
	case mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER:
		return json.RawMessage(`{"contents": {"kind": "markdown", "value": "Mock hover content"}}`)
	case mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS:
		return json.RawMessage(`[{"name": "MockDocumentSymbol", "kind": 12, "range": {"start": {"line": 0, "character": 0}, "end": {"line": 0, "character": 10}}}]`)
	default:
		return json.RawMessage(`{"result": "mock_success", "method": "` + method + `"}`)
	}
}

func (m *MockMcpClient) GetCallCount(method string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, call := range m.SendLSPRequestCalls {
		if call.Method == method {
			count++
		}
	}
	return count
}

func (m *MockMcpClient) GetLastCall() *SendLSPRequestCall {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.SendLSPRequestCalls) == 0 {
		return nil
	}
	return &m.SendLSPRequestCalls[len(m.SendLSPRequestCalls)-1]
}

func (m *MockMcpClient) GetCallsForMethod(method string) []SendLSPRequestCall {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var calls []SendLSPRequestCall
	for _, call := range m.SendLSPRequestCalls {
		if call.Method == method {
			calls = append(calls, call)
		}
	}
	return calls
}
