package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func createTestConfig() *ServerConfig {
	return &ServerConfig{
		LSPGatewayURL: "http://localhost:8080",
		Timeout:       100 * time.Millisecond, // Reduced from 5s
		MaxRetries:    2,                      // Reduced from 3
	}
}

// createFastTestClient creates a client with very fast retry settings for tests
func createFastTestClient(config *ServerConfig) *LSPGatewayClient {
	client := NewLSPGatewayClient(config)
	// Override retry policy for much faster tests
	client.retryPolicy = &RetryPolicy{
		InitialBackoff: 1 * time.Millisecond, // Much faster than default 100ms
		MaxBackoff:     5 * time.Millisecond, // Much faster than default 2s
		BackoffFactor:  1.5,                  // Smaller factor
		JitterEnabled:  false,                // Disable jitter for predictable timing
		MaxRetries:     config.MaxRetries,
		RetryableErrors: map[ErrorCategory]bool{
			ErrorCategoryNetwork:   true,
			ErrorCategoryTimeout:   true,
			ErrorCategoryServer:    true,
			ErrorCategoryRateLimit: true,
		},
	}
	return client
}

func TestNewLSPGatewayClient(t *testing.T) {
	config := createTestConfig()
	client := NewLSPGatewayClient(config)

	if client == nil {
		t.Fatal("Expected client to be created, got nil")
	}

	if client.baseURL != config.LSPGatewayURL {
		t.Errorf("Expected baseURL %s, got %s", config.LSPGatewayURL, client.baseURL)
	}

	if client.timeout != config.Timeout {
		t.Errorf("Expected timeout %v, got %v", config.Timeout, client.timeout)
	}

	if client.maxRetries != config.MaxRetries {
		t.Errorf("Expected maxRetries %d, got %d", config.MaxRetries, client.maxRetries)
	}

	if client.circuitBreaker == nil {
		t.Fatal("Expected circuit breaker to be initialized")
	}

	if client.circuitBreaker.state != CircuitClosed {
		t.Errorf("Expected circuit breaker state to be closed, got %v", client.circuitBreaker.state)
	}

	if client.retryPolicy == nil {
		t.Fatal("Expected retry policy to be initialized")
	}

	if client.retryPolicy.MaxRetries != config.MaxRetries {
		t.Errorf("Expected retry policy max retries %d, got %d", config.MaxRetries, client.retryPolicy.MaxRetries)
	}

	if client.metrics == nil {
		t.Fatal("Expected metrics to be initialized")
	}
}

func TestCircuitBreakerStates(t *testing.T) {
	cb := &CircuitBreaker{
		state:       CircuitClosed,
		timeout:     10 * time.Millisecond, // Reduced from 1s
		maxFailures: 3,
		maxRequests: 2,
	}

	if !cb.AllowRequest() {
		t.Error("Expected closed circuit to allow requests")
	}

	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}

	if cb.state != CircuitOpen {
		t.Errorf("Expected circuit to be open after %d failures", cb.maxFailures)
	}

	if cb.AllowRequest() {
		t.Error("Expected open circuit to reject requests")
	}

	time.Sleep(15 * time.Millisecond) // Reduced wait time
	if !cb.AllowRequest() {
		t.Error("Expected circuit to allow request after timeout (half-open)")
	}

	if cb.state != CircuitHalfOpen {
		t.Error("Expected circuit to be half-open after timeout")
	}

	cb.RecordSuccess()
	cb.RecordSuccess() // Should close circuit after maxRequests successes

	if cb.state != CircuitClosed {
		t.Error("Expected circuit to close after successful requests in half-open")
	}
}

func TestCircuitBreakerConcurrency(t *testing.T) {
	cb := &CircuitBreaker{
		state:       CircuitClosed,
		timeout:     5 * time.Millisecond, // Reduced from 100ms
		maxFailures: 5,
		maxRequests: 3,
	}

	var wg sync.WaitGroup
	var successCount, failureCount int64

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if cb.AllowRequest() {
				atomic.AddInt64(&successCount, 1)
				if i%10 == 0 {
					cb.RecordFailure()
					atomic.AddInt64(&failureCount, 1)
				} else {
					cb.RecordSuccess()
				}
			}
		}()
	}

	wg.Wait()

	if successCount == 0 {
		t.Error("Expected some successful requests in concurrent test")
	}
}

func TestSendLSPRequestSuccess(t *testing.T) {
	expectedResponse := json.RawMessage(`{"result": "test success"}`)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST method, got %s", r.Method)
		}
		if r.URL.Path != JSON_RPC_ENDPOINT {
			t.Errorf("Expected /jsonrpc path, got %s", r.URL.Path)
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		w.Header().Set("Content-Type", "application/json")

		response := JSONRPCResponse{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Result:  expectedResponse,
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	config := createTestConfig()
	config.LSPGatewayURL = server.URL
	client := NewLSPGatewayClient(config)

	ctx := context.Background()
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": "file:///test.go"},
		"position":     map[string]interface{}{"line": 1, "character": 5},
	}

	result, err := client.SendLSPRequest(ctx, "textDocument/definition", params)
	if err != nil {
		t.Fatalf("Expected successful request, got error: %v", err)
	}

	// Normalize JSON for comparison by marshaling and unmarshaling both
	var expectedJSON, resultJSON interface{}
	if err := json.Unmarshal(expectedResponse, &expectedJSON); err != nil {
		t.Fatalf("Failed to unmarshal expected response: %v", err)
	}
	if err := json.Unmarshal(result, &resultJSON); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// Compare the parsed JSON structures
	expectedBytes, _ := json.Marshal(expectedJSON)
	resultBytes, _ := json.Marshal(resultJSON)
	if string(expectedBytes) != string(resultBytes) {
		t.Errorf("Expected result %s, got %s", expectedBytes, resultBytes)
	}

	metrics := client.GetMetrics()
	if metrics.totalRequests != 1 {
		t.Errorf("Expected 1 total request, got %d", metrics.totalRequests)
	}
	if metrics.successfulReqs != 1 {
		t.Errorf("Expected 1 successful request, got %d", metrics.successfulReqs)
	}
}

func TestSendLSPRequestJSONRPCError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := JSONRPCResponse{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Error: &JSONRPCError{
				Code:    -32601,
				Message: "Method not found",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	config := createTestConfig()
	config.LSPGatewayURL = server.URL
	client := NewLSPGatewayClient(config)

	ctx := context.Background()
	params := map[string]interface{}{}

	_, err := client.SendLSPRequest(ctx, "invalid/method", params)
	if err == nil {
		t.Fatal("Expected error for JSON-RPC error response")
	}

	if !strings.Contains(err.Error(), "method not found") {
		t.Errorf("Expected method not found error, got: %v", err)
	}

	metrics := client.GetMetrics()
	if metrics.failedRequests != 1 {
		t.Errorf("Expected 1 failed request, got %d", metrics.failedRequests)
	}
}

func TestSendLSPRequestHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := w.Write([]byte("Internal server error")); err != nil {
			t.Errorf("Failed to write error response: %v", err)
		}
	}))
	defer server.Close()

	config := createTestConfig()
	config.LSPGatewayURL = server.URL
	client := createFastTestClient(config) // Use fast client for quick retries

	ctx := context.Background()
	params := map[string]interface{}{}

	_, err := client.SendLSPRequest(ctx, "textDocument/definition", params)
	if err == nil {
		t.Fatal("Expected error for HTTP 500 response")
	}

	if !strings.Contains(err.Error(), "server error 500") {
		t.Errorf("Expected server error message, got: %v", err)
	}
}

func TestRetryLogicSuccess(t *testing.T) {
	var attemptCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := atomic.AddInt32(&attemptCount, 1)
		if attempt < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		response := JSONRPCResponse{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Result:  json.RawMessage(`{"success": true}`),
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	config := createTestConfig()
	config.LSPGatewayURL = server.URL
	config.MaxRetries = 3
	client := createFastTestClient(config) // Use fast client for quick retries

	ctx := context.Background()
	params := map[string]interface{}{}

	result, err := client.SendLSPRequest(ctx, "textDocument/definition", params)
	if err != nil {
		t.Fatalf("Expected successful request after retries, got error: %v", err)
	}

	if result == nil {
		t.Error("Expected result after successful retry")
	}

	if attemptCount != 3 {
		t.Errorf("Expected 3 attempts, got %d", attemptCount)
	}
}

func TestRetryLogicFailure(t *testing.T) {
	var attemptCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attemptCount, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	config := createTestConfig()
	config.LSPGatewayURL = server.URL
	config.MaxRetries = 2
	client := createFastTestClient(config) // Use fast client for quick retries

	ctx := context.Background()
	params := map[string]interface{}{}

	_, err := client.SendLSPRequest(ctx, "textDocument/definition", params)
	if err == nil {
		t.Fatal("Expected error after max retries exceeded")
	}

	expectedAttempts := int32(config.MaxRetries + 1) // Initial attempt + retries
	if attemptCount != expectedAttempts {
		t.Errorf("Expected %d attempts, got %d", expectedAttempts, attemptCount)
	}

	if !strings.Contains(err.Error(), fmt.Sprintf("failed after %d attempts", expectedAttempts)) {
		t.Errorf("Expected retry failure message, got: %v", err)
	}
}

func TestErrorCategorization(t *testing.T) {
	config := createTestConfig()
	client := NewLSPGatewayClient(config)

	tests := []struct {
		name     string
		err      error
		expected ErrorCategory
	}{
		{
			name:     "Network connection refused",
			err:      fmt.Errorf("connection refused"),
			expected: ErrorCategoryNetwork,
		},
		{
			name:     "Timeout error",
			err:      fmt.Errorf("timeout exceeded"),
			expected: ErrorCategoryTimeout,
		},
		{
			name:     "Context deadline exceeded",
			err:      context.DeadlineExceeded,
			expected: ErrorCategoryTimeout,
		},
		{
			name:     "Context canceled",
			err:      context.Canceled,
			expected: ErrorCategoryClient,
		},
		{
			name:     "HTTP 500 error",
			err:      fmt.Errorf("http error 500"),
			expected: ErrorCategoryServer,
		},
		{
			name:     "HTTP 429 rate limit",
			err:      fmt.Errorf("http error 429"),
			expected: ErrorCategoryRateLimit,
		},
		{
			name:     "HTTP 400 client error",
			err:      fmt.Errorf("http error 400"),
			expected: ErrorCategoryClient,
		},
		{
			name:     "JSON decode error",
			err:      fmt.Errorf("failed to decode json"),
			expected: ErrorCategoryProtocol,
		},
		{
			name:     "Unknown error",
			err:      fmt.Errorf("something unexpected"),
			expected: ErrorCategoryUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			category := client.categorizeError(tt.err)
			if category != tt.expected {
				t.Errorf("Expected category %v, got %v", tt.expected, category)
			}
		})
	}
}

func TestBackoffCalculation(t *testing.T) {
	retryPolicy := &RetryPolicy{
		InitialBackoff:  100 * time.Millisecond,
		MaxBackoff:      2 * time.Second,
		BackoffFactor:   2.0,
		JitterEnabled:   false,
		RetryableErrors: map[ErrorCategory]bool{},
	}

	client := &LSPGatewayClient{retryPolicy: retryPolicy}

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{1, 100 * time.Millisecond},
		{2, 200 * time.Millisecond},
		{3, 400 * time.Millisecond},
		{4, 800 * time.Millisecond},
		{5, 1600 * time.Millisecond},
		{6, 2 * time.Second}, // Should be capped at MaxBackoff
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("attempt_%d", tt.attempt), func(t *testing.T) {
			backoff := client.calculateBackoff(tt.attempt)
			if backoff != tt.expected {
				t.Errorf("Expected backoff %v, got %v", tt.expected, backoff)
			}
		})
	}
}

func TestBackoffWithJitter(t *testing.T) {
	retryPolicy := &RetryPolicy{
		InitialBackoff:  100 * time.Millisecond,
		MaxBackoff:      2 * time.Second,
		BackoffFactor:   2.0,
		JitterEnabled:   true,
		RetryableErrors: map[ErrorCategory]bool{},
	}

	client := &LSPGatewayClient{retryPolicy: retryPolicy}

	baseBackoff := client.calculateBackoff(2)

	backoffs := make([]time.Duration, 10)
	for i := 0; i < 10; i++ {
		backoffs[i] = client.calculateBackoff(2)
	}

	allSame := true
	for i := 1; i < len(backoffs); i++ {
		if backoffs[i] != backoffs[0] {
			allSame = false
			break
		}
	}

	if allSame {
		t.Error("Expected jitter to create variation in backoff times")
	}

	minExpected := time.Duration(float64(baseBackoff) * 0.9)
	maxExpected := time.Duration(float64(baseBackoff) * 1.1)

	for _, backoff := range backoffs {
		if backoff < minExpected || backoff > maxExpected {
			t.Errorf("Backoff %v outside expected range [%v, %v]", backoff, minExpected, maxExpected)
		}
	}
}

func TestTimeoutHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond) // Reduced from 2s
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := createTestConfig()
	config.LSPGatewayURL = server.URL
	config.Timeout = 10 * time.Millisecond // Much shorter timeout
	client := NewLSPGatewayClient(config)

	ctx := context.Background()
	params := map[string]interface{}{}

	_, err := client.SendLSPRequest(ctx, "textDocument/definition", params)
	if err == nil {
		t.Fatal("Expected timeout error")
	}

	category := client.categorizeError(err)
	if category != ErrorCategoryTimeout {
		t.Errorf("Expected timeout category, got %v", category)
	}
}

func TestContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond) // Reduced from 1s
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := createTestConfig()
	config.LSPGatewayURL = server.URL
	client := NewLSPGatewayClient(config)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(5 * time.Millisecond) // Reduced from 100ms
		cancel()
	}()

	params := map[string]interface{}{}
	_, err := client.SendLSPRequest(ctx, "textDocument/definition", params)
	if err == nil {
		t.Fatal("Expected context cancellation error")
	}

	// Check if the error is or contains context.Canceled
	if err != context.Canceled && !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("Expected context.Canceled or wrapped context canceled error, got: %v", err)
	}
}

func TestMetricsCollection(t *testing.T) {
	successCount := int32(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&successCount, 1)
		w.Header().Set("Content-Type", "application/json")
		response := JSONRPCResponse{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Result:  json.RawMessage(`{"success": true}`),
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	config := createTestConfig()
	config.LSPGatewayURL = server.URL
	client := NewLSPGatewayClient(config)

	ctx := context.Background()
	params := map[string]interface{}{}

	for i := 0; i < 5; i++ {
		_, err := client.SendLSPRequest(ctx, "textDocument/definition", params)
		if err != nil {
			t.Fatalf("Request %d failed: %v", i, err)
		}
	}

	metrics := client.GetMetrics()
	if metrics.totalRequests != 5 {
		t.Errorf("Expected 5 total requests, got %d", metrics.totalRequests)
	}
	if metrics.successfulReqs != 5 {
		t.Errorf("Expected 5 successful requests, got %d", metrics.successfulReqs)
	}
	if metrics.failedRequests != 0 {
		t.Errorf("Expected 0 failed requests, got %d", metrics.failedRequests)
	}
	if metrics.averageLatency == 0 {
		t.Error("Expected non-zero average latency")
	}
}

func TestHealthCheck(t *testing.T) {
	healthCheckCount := int32(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			atomic.AddInt32(&healthCheckCount, 1)
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	config := createTestConfig()
	config.LSPGatewayURL = server.URL
	client := NewLSPGatewayClient(config)

	ctx := context.Background()
	err := client.GetHealth(ctx)
	if err != nil {
		t.Fatalf("Expected successful health check, got error: %v", err)
	}

	if healthCheckCount != 1 {
		t.Errorf("Expected 1 health check call, got %d", healthCheckCount)
	}

	if !client.IsHealthy() {
		t.Error("Expected client to be healthy")
	}

	err = client.GetHealth(ctx) // Should be rate limited
	if err != nil {
		t.Fatalf("Expected successful health check (rate limited), got error: %v", err)
	}

	if healthCheckCount != 1 {
		t.Errorf("Expected health check to be rate limited, got %d calls", healthCheckCount)
	}
}

func TestHealthCheckFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	config := createTestConfig()
	config.LSPGatewayURL = server.URL
	client := NewLSPGatewayClient(config)

	ctx := context.Background()
	err := client.GetHealth(ctx)
	if err == nil {
		t.Fatal("Expected health check to fail")
	}

	if !strings.Contains(err.Error(), "status 500") {
		t.Errorf("Expected status 500 error, got: %v", err)
	}
}

func TestNetworkErrorHandling(t *testing.T) {
	config := createTestConfig()
	config.LSPGatewayURL = "http://localhost:99999" // Invalid port
	client := NewLSPGatewayClient(config)

	ctx := context.Background()
	params := map[string]interface{}{}

	_, err := client.SendLSPRequest(ctx, "textDocument/definition", params)
	if err == nil {
		t.Fatal("Expected connection error")
	}

	category := client.categorizeError(err)
	if category != ErrorCategoryNetwork {
		t.Errorf("Expected network error category, got %v", category)
	}
}

func TestInvalidJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte("invalid json response")); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	}))
	defer server.Close()

	config := createTestConfig()
	config.LSPGatewayURL = server.URL
	client := NewLSPGatewayClient(config)

	ctx := context.Background()
	params := map[string]interface{}{}

	_, err := client.SendLSPRequest(ctx, "textDocument/definition", params)
	if err == nil {
		t.Fatal("Expected JSON decode error")
	}

	if !strings.Contains(err.Error(), "failed to decode") {
		t.Errorf("Expected decode error, got: %v", err)
	}
}

func TestWrongContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		if _, err := w.Write([]byte("plain text response")); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	}))
	defer server.Close()

	config := createTestConfig()
	config.LSPGatewayURL = server.URL
	client := NewLSPGatewayClient(config)

	ctx := context.Background()
	params := map[string]interface{}{}

	_, err := client.SendLSPRequest(ctx, "textDocument/definition", params)
	if err == nil {
		t.Fatal("Expected content type error")
	}

	if !strings.Contains(err.Error(), "unexpected content type") {
		t.Errorf("Expected content type error, got: %v", err)
	}
}

func TestCircuitBreakerIntegration(t *testing.T) {
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.WriteHeader(http.StatusInternalServerError) // Always fail
	}))
	defer server.Close()

	config := createTestConfig()
	config.LSPGatewayURL = server.URL
	config.MaxRetries = 0 // No retries to test circuit breaker faster
	client := NewLSPGatewayClient(config)

	client.circuitBreaker.maxFailures = 3
	client.circuitBreaker.timeout = 5 * time.Millisecond // Reduced from 100ms

	ctx := context.Background()
	params := map[string]interface{}{}

	for i := 0; i < 5; i++ {
		_, err := client.SendLSPRequest(ctx, "textDocument/definition", params)
		if err != nil && strings.Contains(err.Error(), "circuit breaker is open") {
			break
		}
	}

	_, err := client.SendLSPRequest(ctx, "textDocument/definition", params)
	if err == nil || !strings.Contains(err.Error(), "circuit breaker is open") {
		t.Errorf("Expected circuit breaker to be open, got error: %v", err)
	}

	time.Sleep(10 * time.Millisecond) // Reduced from 150ms

	_, err = client.SendLSPRequest(ctx, "textDocument/definition", params)
	if err != nil && strings.Contains(err.Error(), "circuit breaker is open") {
		t.Error("Expected circuit to allow request in half-open state")
	}
}

func TestRequestIDGeneration(t *testing.T) {
	ids := make(map[interface{}]bool)

	for i := 0; i < 1000; i++ {
		id := generateRequestID()
		if ids[id] {
			t.Errorf("Duplicate request ID generated: %v", id)
		}
		ids[id] = true
	}
}

func TestHTTPClientConfiguration(t *testing.T) {
	config := createTestConfig()
	client := NewLSPGatewayClient(config)

	if client.httpClient == nil {
		t.Fatal("Expected HTTP client to be configured")
	}

	if client.httpClient.Timeout != config.Timeout {
		t.Errorf("Expected HTTP client timeout %v, got %v", config.Timeout, client.httpClient.Timeout)
	}

	transport, ok := client.httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatal("Expected HTTP transport to be configured")
	}

	if transport.MaxIdleConns != 100 {
		t.Errorf("Expected MaxIdleConns 100, got %d", transport.MaxIdleConns)
	}

	if transport.MaxIdleConnsPerHost != 10 {
		t.Errorf("Expected MaxIdleConnsPerHost 10, got %d", transport.MaxIdleConnsPerHost)
	}
}

func BenchmarkSendLSPRequest(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := JSONRPCResponse{
			JSONRPC: JSONRPCVersion,
			ID:      1,
			Result:  json.RawMessage(`{"benchmark": true}`),
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			b.Errorf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	config := createTestConfig()
	config.LSPGatewayURL = server.URL
	client := NewLSPGatewayClient(config)

	ctx := context.Background()
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": "file:///test.go"},
		"position":     map[string]interface{}{"line": 1, "character": 5},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.SendLSPRequest(ctx, "textDocument/definition", params)
		if err != nil {
			b.Fatalf("Request failed: %v", err)
		}
	}
}

func BenchmarkCircuitBreakerCheck(b *testing.B) {
	cb := &CircuitBreaker{
		state:       CircuitClosed,
		timeout:     1 * time.Second,
		maxFailures: 5,
		maxRequests: 3,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cb.AllowRequest()
	}
}

// TestProtocolCompliance tests invalid MCP message formats and protocol violations
func TestProtocolCompliance(t *testing.T) {
	tests := []struct {
		name            string
		responseBuilder func(w http.ResponseWriter, r *http.Request)
		expectedError   string
		shouldFail      bool
	}{
		{
			name: "InvalidJSONRPCVersion",
			responseBuilder: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				response := map[string]interface{}{
					"jsonrpc": "1.0", // Invalid version
					"id":      1,
					"result":  map[string]interface{}{"test": "result"},
				}
				json.NewEncoder(w).Encode(response)
			},
			expectedError: "invalid JSON-RPC version",
			shouldFail:    true,
		},
		{
			name: "MissingJSONRPCField",
			responseBuilder: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				response := map[string]interface{}{
					// Missing "jsonrpc" field
					"id":     1,
					"result": map[string]interface{}{"test": "result"},
				}
				json.NewEncoder(w).Encode(response)
			},
			expectedError: "invalid JSON-RPC version",
			shouldFail:    true,
		},
		{
			name: "InvalidContentType",
			responseBuilder: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/plain")
				w.Write([]byte("plain text response"))
			},
			expectedError: "unexpected content type",
			shouldFail:    true,
		},
		{
			name: "MalformedJSON",
			responseBuilder: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":malformed}`))
			},
			expectedError: "failed to decode",
			shouldFail:    true,
		},
		{
			name: "JSONRPCErrorResponse",
			responseBuilder: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				response := JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      1,
					Error: &JSONRPCError{
						Code:    -32601,
						Message: "Method not found",
					},
				}
				json.NewEncoder(w).Encode(response)
			},
			expectedError: "method not found",
			shouldFail:    true,
		},
		{
			name: "JSONRPCInvalidParams",
			responseBuilder: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				response := JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      1,
					Error: &JSONRPCError{
						Code:    -32602,
						Message: "Invalid params",
					},
				}
				json.NewEncoder(w).Encode(response)
			},
			expectedError: "invalid parameters",
			shouldFail:    true,
		},
		{
			name: "ValidResponse",
			responseBuilder: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				response := JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      1,
					Result:  json.RawMessage(`{"success": true}`),
				}
				json.NewEncoder(w).Encode(response)
			},
			expectedError: "",
			shouldFail:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.responseBuilder))
			defer server.Close()

			config := createTestConfig()
			config.LSPGatewayURL = server.URL
			client := NewLSPGatewayClient(config)

			ctx := context.Background()
			params := map[string]interface{}{
				"textDocument": map[string]interface{}{"uri": "file:///test.go"},
				"position":     map[string]interface{}{"line": 1, "character": 5},
			}

			_, err := client.SendLSPRequest(ctx, "textDocument/definition", params)

			if tt.shouldFail {
				if err == nil {
					t.Errorf("Expected error containing '%s', but got no error", tt.expectedError)
				} else if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.expectedError)) {
					t.Errorf("Expected error containing '%s', got: %v", tt.expectedError, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
			}
		})
	}
}

// TestParameterValidation tests parameter validation for MCP tools
func TestParameterValidation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      1,
			Error: &JSONRPCError{
				Code:    -32602,
				Message: "Invalid params",
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := createTestConfig()
	config.LSPGatewayURL = server.URL
	client := NewLSPGatewayClient(config)

	ctx := context.Background()

	tests := []struct {
		name     string
		method   string
		params   interface{}
		expected string
	}{
		{
			name:     "MissingTextDocument",
			method:   "textDocument/definition",
			params:   map[string]interface{}{"position": map[string]interface{}{"line": 1, "character": 5}},
			expected: "invalid parameters",
		},
		{
			name:     "MissingPosition",
			method:   "textDocument/definition",
			params:   map[string]interface{}{"textDocument": map[string]interface{}{"uri": "file:///test.go"}},
			expected: "invalid parameters",
		},
		{
			name:     "InvalidURI",
			method:   "textDocument/definition",
			params:   map[string]interface{}{"textDocument": map[string]interface{}{"uri": ""}, "position": map[string]interface{}{"line": 1, "character": 5}},
			expected: "invalid parameters",
		},
		{
			name:     "NilParams",
			method:   "textDocument/definition",
			params:   nil,
			expected: "invalid parameters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.SendLSPRequest(ctx, tt.method, tt.params)
			if err == nil {
				t.Errorf("Expected error for %s, but got none", tt.name)
			} else if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.expected)) {
				t.Errorf("Expected error containing '%s', got: %v", tt.expected, err)
			}
		})
	}
}

// TestEnhancedRetryLogic tests comprehensive retry behavior with different scenarios
func TestEnhancedRetryLogic(t *testing.T) {
	t.Run("AutomaticRetryOnNetworkFailures", func(t *testing.T) {
		var attemptCount int32

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempt := atomic.AddInt32(&attemptCount, 1)
			if attempt < 3 {
				// Simulate network failure by closing connection
				conn, _, _ := w.(http.Hijacker).Hijack()
				conn.Close()
				return
			}

			// Success after retries
			w.Header().Set("Content-Type", "application/json")
			response := JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      1,
				Result:  json.RawMessage(`{"success": true}`),
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		config := createTestConfig()
		config.LSPGatewayURL = server.URL
		config.MaxRetries = 3
		client := createFastTestClient(config)

		ctx := context.Background()
		params := map[string]interface{}{}

		result, err := client.SendLSPRequest(ctx, "textDocument/definition", params)
		if err != nil {
			t.Fatalf("Expected successful request after retries, got error: %v", err)
		}

		if result == nil {
			t.Error("Expected result after successful retry")
		}

		if attemptCount != 3 {
			t.Errorf("Expected 3 attempts, got %d", attemptCount)
		}
	})

	t.Run("ExponentialBackoffImplementation", func(t *testing.T) {
		retryPolicy := &RetryPolicy{
			InitialBackoff:  10 * time.Millisecond,
			MaxBackoff:      1 * time.Second,
			BackoffFactor:   2.0,
			JitterEnabled:   false,
			RetryableErrors: map[ErrorCategory]bool{ErrorCategoryServer: true},
		}

		client := &LSPGatewayClient{retryPolicy: retryPolicy}

		// Test exponential backoff calculation
		expectedBackoffs := []time.Duration{
			10 * time.Millisecond,  // Attempt 1
			20 * time.Millisecond,  // Attempt 2
			40 * time.Millisecond,  // Attempt 3
			80 * time.Millisecond,  // Attempt 4
			160 * time.Millisecond, // Attempt 5
			320 * time.Millisecond, // Attempt 6
			640 * time.Millisecond, // Attempt 7
			1 * time.Second,        // Attempt 8 (capped at MaxBackoff)
		}

		for i, expected := range expectedBackoffs {
			actual := client.calculateBackoff(i + 1)
			if actual != expected {
				t.Errorf("Attempt %d: expected backoff %v, got %v", i+1, expected, actual)
			}
		}
	})

	t.Run("JitterImplementation", func(t *testing.T) {
		retryPolicy := &RetryPolicy{
			InitialBackoff:  100 * time.Millisecond,
			MaxBackoff:      2 * time.Second,
			BackoffFactor:   2.0,
			JitterEnabled:   true,
			RetryableErrors: map[ErrorCategory]bool{ErrorCategoryServer: true},
		}

		client := &LSPGatewayClient{retryPolicy: retryPolicy}

		// Test that jitter creates variation
		backoffs := make([]time.Duration, 10)
		for i := 0; i < 10; i++ {
			backoffs[i] = client.calculateBackoff(2) // Fixed attempt for consistency
		}

		// Check that not all backoffs are the same (jitter working)
		allSame := true
		for i := 1; i < len(backoffs); i++ {
			if backoffs[i] != backoffs[0] {
				allSame = false
				break
			}
		}

		if allSame {
			t.Error("Expected jitter to create variation in backoff times, but all were the same")
		}

		// Check that jitter is within reasonable bounds (within 10% of base)
		baseBackoff := 200 * time.Millisecond // 100ms * 2^1
		for i, backoff := range backoffs {
			if backoff < baseBackoff || backoff > time.Duration(float64(baseBackoff)*1.1) {
				t.Errorf("Backoff %d (%v) outside expected jitter range [%v, %v]", i, backoff, baseBackoff, time.Duration(float64(baseBackoff)*1.1))
			}
		}
	})

	t.Run("MaximumRetryLimitEnforcement", func(t *testing.T) {
		var attemptCount int32

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&attemptCount, 1)
			w.WriteHeader(http.StatusInternalServerError) // Always fail
		}))
		defer server.Close()

		config := createTestConfig()
		config.LSPGatewayURL = server.URL
		config.MaxRetries = 2 // 1 initial + 2 retries = 3 total attempts
		client := createFastTestClient(config)

		ctx := context.Background()
		params := map[string]interface{}{}

		_, err := client.SendLSPRequest(ctx, "textDocument/definition", params)
		if err == nil {
			t.Fatal("Expected error after max retries exceeded")
		}

		expectedAttempts := int32(config.MaxRetries + 1) // 1 initial + 2 retries
		if attemptCount != expectedAttempts {
			t.Errorf("Expected %d attempts (1 initial + %d retries), got %d", expectedAttempts, config.MaxRetries, attemptCount)
		}

		if !strings.Contains(err.Error(), fmt.Sprintf("failed after %d attempts", expectedAttempts)) {
			t.Errorf("Expected error message to mention %d attempts, got: %v", expectedAttempts, err)
		}
	})

	t.Run("AutomaticRecoveryMechanisms", func(t *testing.T) {
		var (
			attemptCount int32
			failurePhase = true
		)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempt := atomic.AddInt32(&attemptCount, 1)

			// Fail first few attempts, then start succeeding
			if failurePhase && attempt <= 2 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			failurePhase = false // Switch to success phase
			w.Header().Set("Content-Type", "application/json")
			response := JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      1,
				Result:  json.RawMessage(`{"recovered": true}`),
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		config := createTestConfig()
		config.LSPGatewayURL = server.URL
		config.MaxRetries = 3
		client := createFastTestClient(config)

		ctx := context.Background()
		params := map[string]interface{}{}

		// First request should recover after retries
		result, err := client.SendLSPRequest(ctx, "textDocument/definition", params)
		if err != nil {
			t.Fatalf("Expected successful recovery, got error: %v", err)
		}

		if result == nil {
			t.Error("Expected result after recovery")
		}

		// Subsequent requests should succeed immediately
		atomic.StoreInt32(&attemptCount, 0)
		result2, err2 := client.SendLSPRequest(ctx, "textDocument/definition", params)
		if err2 != nil {
			t.Fatalf("Expected immediate success after recovery, got error: %v", err2)
		}

		if result2 == nil {
			t.Error("Expected result from subsequent request")
		}

		// Should only need one attempt for subsequent request
		if attemptCount != 1 {
			t.Errorf("Expected 1 attempt for subsequent request, got %d", attemptCount)
		}
	})

	t.Run("TimeoutHandlingWithRetries", func(t *testing.T) {
		var attemptCount int32

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempt := atomic.AddInt32(&attemptCount, 1)
			if attempt < 3 {
				// Simulate timeout by delaying response
				time.Sleep(50 * time.Millisecond)
			}

			w.Header().Set("Content-Type", "application/json")
			response := JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      1,
				Result:  json.RawMessage(`{"success": true}`),
			}
			json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		config := createTestConfig()
		config.LSPGatewayURL = server.URL
		config.Timeout = 20 * time.Millisecond // Short timeout to trigger timeouts
		config.MaxRetries = 3
		client := createFastTestClient(config)

		ctx := context.Background()
		params := map[string]interface{}{}

		result, err := client.SendLSPRequest(ctx, "textDocument/definition", params)
		if err != nil {
			t.Fatalf("Expected successful request after timeout retries, got error: %v", err)
		}

		if result == nil {
			t.Error("Expected result after timeout recovery")
		}

		if attemptCount < 3 {
			t.Errorf("Expected at least 3 attempts due to timeouts, got %d", attemptCount)
		}
	})
}
