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

	"lsp-gateway/internal/gateway"
)

// Test server configurations
func createTestConfig() *ServerConfig {
	return &ServerConfig{
		LSPGatewayURL: "http://localhost:8080",
		Timeout:       5 * time.Second,
		MaxRetries:    3,
	}
}

// Test LSPGatewayClient creation and configuration
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

	// Test circuit breaker initialization
	if client.circuitBreaker == nil {
		t.Fatal("Expected circuit breaker to be initialized")
	}

	if client.circuitBreaker.state != CircuitClosed {
		t.Errorf("Expected circuit breaker state to be closed, got %v", client.circuitBreaker.state)
	}

	// Test retry policy initialization
	if client.retryPolicy == nil {
		t.Fatal("Expected retry policy to be initialized")
	}

	if client.retryPolicy.MaxRetries != config.MaxRetries {
		t.Errorf("Expected retry policy max retries %d, got %d", config.MaxRetries, client.retryPolicy.MaxRetries)
	}

	// Test metrics initialization
	if client.metrics == nil {
		t.Fatal("Expected metrics to be initialized")
	}
}

// Test Circuit Breaker functionality
func TestCircuitBreakerStates(t *testing.T) {
	cb := &CircuitBreaker{
		state:       CircuitClosed,
		timeout:     1 * time.Second,
		maxFailures: 3,
		maxRequests: 2,
	}

	// Test closed state allows requests
	if !cb.AllowRequest() {
		t.Error("Expected closed circuit to allow requests")
	}

	// Test failure recording
	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}

	// Should be open now
	if cb.state != CircuitOpen {
		t.Errorf("Expected circuit to be open after %d failures", cb.maxFailures)
	}

	// Test open state rejects requests
	if cb.AllowRequest() {
		t.Error("Expected open circuit to reject requests")
	}

	// Test half-open transition after timeout
	time.Sleep(1100 * time.Millisecond) // Wait for timeout
	if !cb.AllowRequest() {
		t.Error("Expected circuit to allow request after timeout (half-open)")
	}

	if cb.state != CircuitHalfOpen {
		t.Error("Expected circuit to be half-open after timeout")
	}

	// Test success recording in half-open state
	cb.RecordSuccess()
	cb.RecordSuccess() // Should close circuit after maxRequests successes

	if cb.state != CircuitClosed {
		t.Error("Expected circuit to close after successful requests in half-open")
	}
}

func TestCircuitBreakerConcurrency(t *testing.T) {
	cb := &CircuitBreaker{
		state:       CircuitClosed,
		timeout:     100 * time.Millisecond,
		maxFailures: 5,
		maxRequests: 3,
	}

	var wg sync.WaitGroup
	var successCount, failureCount int64

	// Concurrent access test
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

// Test HTTP request functionality
func TestSendLSPRequestSuccess(t *testing.T) {
	expectedResponse := json.RawMessage(`{"result": "test success"}`)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Validate request method and path
		if r.Method != "POST" {
			t.Errorf("Expected POST method, got %s", r.Method)
		}
		if r.URL.Path != "/jsonrpc" {
			t.Errorf("Expected /jsonrpc path, got %s", r.URL.Path)
		}

		// Validate headers
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		// Set the correct content type in response
		w.Header().Set("Content-Type", "application/json")

		response := JSONRPCResponse{
			JSONRPC: gateway.JSONRPCVersion,
			ID:      1,
			Result:  expectedResponse,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
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

	if string(result) != string(expectedResponse) {
		t.Errorf("Expected result %s, got %s", expectedResponse, result)
	}

	// Check metrics
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
			JSONRPC: gateway.JSONRPCVersion,
			ID:      1,
			Error: &JSONRPCError{
				Code:    -32601,
				Message: "Method not found",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
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

	// Check metrics
	metrics := client.GetMetrics()
	if metrics.failedRequests != 1 {
		t.Errorf("Expected 1 failed request, got %d", metrics.failedRequests)
	}
}

func TestSendLSPRequestHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal server error"))
	}))
	defer server.Close()

	config := createTestConfig()
	config.LSPGatewayURL = server.URL
	client := NewLSPGatewayClient(config)

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

// Test retry logic
func TestRetryLogicSuccess(t *testing.T) {
	var attemptCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := atomic.AddInt32(&attemptCount, 1)
		if attempt < 3 {
			// Fail first two attempts
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Succeed on third attempt
		w.Header().Set("Content-Type", "application/json")
		response := JSONRPCResponse{
			JSONRPC: gateway.JSONRPCVersion,
			ID:      1,
			Result:  json.RawMessage(`{"success": true}`),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := createTestConfig()
	config.LSPGatewayURL = server.URL
	config.MaxRetries = 3
	client := NewLSPGatewayClient(config)

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
	client := NewLSPGatewayClient(config)

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

// Test error categorization
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

// Test backoff calculation
func TestBackoffCalculation(t *testing.T) {
	retryPolicy := &RetryPolicy{
		InitialBackoff:  100 * time.Millisecond,
		MaxBackoff:      2 * time.Second,
		BackoffFactor:   2.0,
		JitterEnabled:   false,
		RetryableErrors: map[ErrorCategory]bool{},
	}

	client := &LSPGatewayClient{retryPolicy: retryPolicy}

	// Test exponential backoff
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

	// Test that jitter adds some variation
	baseBackoff := client.calculateBackoff(2)

	// Calculate multiple backoffs and ensure they're not all identical
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

	// Ensure backoff is within reasonable range (base ± 10%)
	minExpected := time.Duration(float64(baseBackoff) * 0.9)
	maxExpected := time.Duration(float64(baseBackoff) * 1.1)

	for _, backoff := range backoffs {
		if backoff < minExpected || backoff > maxExpected {
			t.Errorf("Backoff %v outside expected range [%v, %v]", backoff, minExpected, maxExpected)
		}
	}
}

// Test timeout handling
func TestTimeoutHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow server
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := createTestConfig()
	config.LSPGatewayURL = server.URL
	config.Timeout = 500 * time.Millisecond // Short timeout
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
		time.Sleep(1 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := createTestConfig()
	config.LSPGatewayURL = server.URL
	client := NewLSPGatewayClient(config)

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context after short delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	params := map[string]interface{}{}
	_, err := client.SendLSPRequest(ctx, "textDocument/definition", params)
	if err == nil {
		t.Fatal("Expected context cancellation error")
	}

	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got: %v", err)
	}
}

// Test metrics collection
func TestMetricsCollection(t *testing.T) {
	successCount := int32(0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&successCount, 1)
		w.Header().Set("Content-Type", "application/json")
		response := JSONRPCResponse{
			JSONRPC: gateway.JSONRPCVersion,
			ID:      1,
			Result:  json.RawMessage(`{"success": true}`),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := createTestConfig()
	config.LSPGatewayURL = server.URL
	client := NewLSPGatewayClient(config)

	ctx := context.Background()
	params := map[string]interface{}{}

	// Make multiple requests
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

// Test health checking
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

	// Test rate limiting of health checks
	err = client.GetHealth(ctx) // Should be rate limited
	if err != nil {
		t.Fatalf("Expected successful health check (rate limited), got error: %v", err)
	}

	// Should not increment counter due to rate limiting
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

// Test network error handling
func TestNetworkErrorHandling(t *testing.T) {
	// Test connection refused
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

// Test invalid JSON response handling
func TestInvalidJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("invalid json response"))
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

// Test wrong content type handling
func TestWrongContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("plain text response"))
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

// Test circuit breaker integration with client
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

	// Override circuit breaker settings for faster testing
	client.circuitBreaker.maxFailures = 3
	client.circuitBreaker.timeout = 100 * time.Millisecond

	ctx := context.Background()
	params := map[string]interface{}{}

	// Make requests until circuit opens
	for i := 0; i < 5; i++ {
		_, err := client.SendLSPRequest(ctx, "textDocument/definition", params)
		if err != nil && strings.Contains(err.Error(), "circuit breaker is open") {
			break
		}
	}

	// Circuit should be open now
	_, err := client.SendLSPRequest(ctx, "textDocument/definition", params)
	if err == nil || !strings.Contains(err.Error(), "circuit breaker is open") {
		t.Errorf("Expected circuit breaker to be open, got error: %v", err)
	}

	// Wait for circuit to go half-open
	time.Sleep(150 * time.Millisecond)

	// Should allow one request in half-open state
	_, err = client.SendLSPRequest(ctx, "textDocument/definition", params)
	if err != nil && strings.Contains(err.Error(), "circuit breaker is open") {
		t.Error("Expected circuit to allow request in half-open state")
	}
}

// Test request ID generation
func TestRequestIDGeneration(t *testing.T) {
	ids := make(map[interface{}]bool)

	// Generate multiple IDs and ensure they're unique
	for i := 0; i < 1000; i++ {
		id := generateRequestID()
		if ids[id] {
			t.Errorf("Duplicate request ID generated: %v", id)
		}
		ids[id] = true
	}
}

// Test HTTP client configuration
func TestHTTPClientConfiguration(t *testing.T) {
	config := createTestConfig()
	client := NewLSPGatewayClient(config)

	if client.httpClient == nil {
		t.Fatal("Expected HTTP client to be configured")
	}

	if client.httpClient.Timeout != config.Timeout {
		t.Errorf("Expected HTTP client timeout %v, got %v", config.Timeout, client.httpClient.Timeout)
	}

	// Test transport configuration
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

// Benchmark tests for performance validation
func BenchmarkSendLSPRequest(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := JSONRPCResponse{
			JSONRPC: gateway.JSONRPCVersion,
			ID:      1,
			Result:  json.RawMessage(`{"benchmark": true}`),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
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
