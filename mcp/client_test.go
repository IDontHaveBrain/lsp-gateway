package mcp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestCalculateBackoff(t *testing.T) {
	config := &ServerConfig{
		LSPGatewayURL: "http://localhost:8080",
		Timeout:       30 * time.Second,
		MaxRetries:    3,
	}
	client := NewLSPGatewayClient(config)

	tests := []struct {
		name           string
		attempt        int
		jitterEnabled  bool
		initialBackoff time.Duration
		maxBackoff     time.Duration
		backoffFactor  float64
		expectMin      time.Duration
		expectMax      time.Duration
	}{
		{
			name:           "First attempt no jitter",
			attempt:        1,
			jitterEnabled:  false,
			initialBackoff: 500 * time.Millisecond,
			maxBackoff:     30 * time.Second,
			backoffFactor:  2.0,
			expectMin:      500 * time.Millisecond,
			expectMax:      500 * time.Millisecond,
		},
		{
			name:           "Second attempt no jitter",
			attempt:        2,
			jitterEnabled:  false,
			initialBackoff: 500 * time.Millisecond,
			maxBackoff:     30 * time.Second,
			backoffFactor:  2.0,
			expectMin:      1000 * time.Millisecond,
			expectMax:      1000 * time.Millisecond,
		},
		{
			name:           "Third attempt no jitter",
			attempt:        3,
			jitterEnabled:  false,
			initialBackoff: 500 * time.Millisecond,
			maxBackoff:     30 * time.Second,
			backoffFactor:  2.0,
			expectMin:      2000 * time.Millisecond,
			expectMax:      2000 * time.Millisecond,
		},
		{
			name:           "Backoff capped at max",
			attempt:        10,
			jitterEnabled:  false,
			initialBackoff: 500 * time.Millisecond,
			maxBackoff:     5 * time.Second,
			backoffFactor:  2.0,
			expectMin:      5 * time.Second,
			expectMax:      5 * time.Second,
		},
		{
			name:           "With jitter enabled",
			attempt:        1,
			jitterEnabled:  true,
			initialBackoff: 500 * time.Millisecond,
			maxBackoff:     30 * time.Second,
			backoffFactor:  2.0,
			expectMin:      500 * time.Millisecond,
			expectMax:      550 * time.Millisecond, // 10% jitter
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client.retryPolicy.JitterEnabled = tt.jitterEnabled
			client.retryPolicy.InitialBackoff = tt.initialBackoff
			client.retryPolicy.MaxBackoff = tt.maxBackoff
			client.retryPolicy.BackoffFactor = tt.backoffFactor

			backoff := client.CalculateBackoff(tt.attempt)

			if backoff < tt.expectMin || backoff > tt.expectMax {
				t.Errorf("calculateBackoff(%d) = %v, expected between %v and %v", 
					tt.attempt, backoff, tt.expectMin, tt.expectMax)
			}

			// Test multiple times for jitter variance when enabled
			if tt.jitterEnabled {
				results := make([]time.Duration, 10)
				for i := 0; i < 10; i++ {
					results[i] = client.CalculateBackoff(tt.attempt)
				}
				
				// Verify some variance exists (not all identical)
				hasVariance := false
				for i := 1; i < len(results); i++ {
					if results[i] != results[0] {
						hasVariance = true
						break
					}
				}
				
				if !hasVariance {
					t.Error("Expected jitter variance, but all backoff values were identical")
				}
			}
		})
	}
}

func TestCategorizeError(t *testing.T) {
	config := &ServerConfig{
		LSPGatewayURL: "http://localhost:8080",
		Timeout:       30 * time.Second,
		MaxRetries:    3,
	}
	client := NewLSPGatewayClient(config)

	tests := []struct {
		name     string
		err      error
		expected ErrorCategory
	}{
		{
			name:     "Nil error",
			err:      nil,
			expected: ErrorCategoryUnknown,
		},
		{
			name:     "Connection refused",
			err:      errors.New("connection refused"),
			expected: ErrorCategoryNetwork,
		},
		{
			name:     "Connection reset",
			err:      errors.New("connection reset by peer"),
			expected: ErrorCategoryNetwork,
		},
		{
			name:     "Network unreachable",
			err:      errors.New("network is unreachable"),
			expected: ErrorCategoryNetwork,
		},
		{
			name:     "No route to host",
			err:      errors.New("no route to host"),
			expected: ErrorCategoryNetwork,
		},
		{
			name:     "Connection timeout",
			err:      errors.New("connection timeout"),
			expected: ErrorCategoryNetwork,
		},
		{
			name:     "EOF error",
			err:      errors.New("EOF"),
			expected: ErrorCategoryNetwork,
		},
		{
			name:     "Dial TCP error",
			err:      errors.New("dial tcp: operation timed out"),
			expected: ErrorCategoryNetwork,
		},
		{
			name:     "Timeout error",
			err:      errors.New("timeout exceeded"),
			expected: ErrorCategoryTimeout,
		},
		{
			name:     "Deadline exceeded",
			err:      errors.New("deadline exceeded"),
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
			err:      errors.New("HTTP error 500"),
			expected: ErrorCategoryServer,
		},
		{
			name:     "Server error 503",
			err:      errors.New("server error 503"),
			expected: ErrorCategoryServer,
		},
		{
			name:     "HTTP 429 rate limit",
			err:      errors.New("HTTP error 429"),
			expected: ErrorCategoryRateLimit,
		},
		{
			name:     "Too many requests",
			err:      errors.New("too many requests"),
			expected: ErrorCategoryRateLimit,
		},
		{
			name:     "HTTP 400 client error",
			err:      errors.New("HTTP error 400"),
			expected: ErrorCategoryClient,
		},
		{
			name:     "HTTP 404 not found",
			err:      errors.New("HTTP error 404"),
			expected: ErrorCategoryClient,
		},
		{
			name:     "Failed to decode JSON",
			err:      errors.New("failed to decode response"),
			expected: ErrorCategoryProtocol,
		},
		{
			name:     "Failed to marshal",
			err:      errors.New("failed to marshal request"),
			expected: ErrorCategoryProtocol,
		},
		{
			name:     "Invalid JSON",
			err:      errors.New("invalid json format"),
			expected: ErrorCategoryProtocol,
		},
		{
			name:     "Unknown error",
			err:      errors.New("some unknown error"),
			expected: ErrorCategoryUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			category := client.CategorizeError(tt.err)
			if category != tt.expected {
				t.Errorf("categorizeError(%v) = %v, expected %v", tt.err, category, tt.expected)
			}
		})
	}
}

func TestShouldRetryError(t *testing.T) {
	config := &ServerConfig{
		LSPGatewayURL: "http://localhost:8080",
		Timeout:       30 * time.Second,
		MaxRetries:    3,
	}
	client := NewLSPGatewayClient(config)

	tests := []struct {
		name     string
		category ErrorCategory
		expected bool
	}{
		{
			name:     "Network errors should retry",
			category: ErrorCategoryNetwork,
			expected: true,
		},
		{
			name:     "Timeout errors should retry",
			category: ErrorCategoryTimeout,
			expected: true,
		},
		{
			name:     "Server errors should retry",
			category: ErrorCategoryServer,
			expected: true,
		},
		{
			name:     "Rate limit errors should retry",
			category: ErrorCategoryRateLimit,
			expected: true,
		},
		{
			name:     "Client errors should not retry",
			category: ErrorCategoryClient,
			expected: false,
		},
		{
			name:     "Protocol errors should not retry",
			category: ErrorCategoryProtocol,
			expected: false,
		},
		{
			name:     "Unknown errors should retry",
			category: ErrorCategoryUnknown,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldRetry := client.shouldRetryError(tt.category)
			if shouldRetry != tt.expected {
				t.Errorf("shouldRetryError(%v) = %v, expected %v", tt.category, shouldRetry, tt.expected)
			}
		})
	}
}

func TestCircuitBreakerAllowRequest(t *testing.T) {
	tests := []struct {
		name         string
		state        CircuitBreakerState
		failureTime  time.Time
		successCount int
		timeout      time.Duration
		maxRequests  int
		expected     bool
	}{
		{
			name:     "Closed circuit allows requests",
			state:    CircuitClosed,
			expected: true,
		},
		{
			name:        "Open circuit within timeout rejects requests",
			state:       CircuitOpen,
			failureTime: time.Now().Add(-30 * time.Second),
			timeout:     60 * time.Second,
			expected:    false,
		},
		{
			name:        "Open circuit past timeout transitions to half-open",
			state:       CircuitOpen,
			failureTime: time.Now().Add(-70 * time.Second),
			timeout:     60 * time.Second,
			expected:    true,
		},
		{
			name:         "Half-open under max requests allows",
			state:        CircuitHalfOpen,
			successCount: 1,
			maxRequests:  3,
			expected:     true,
		},
		{
			name:         "Half-open at max requests rejects",
			state:        CircuitHalfOpen,
			successCount: 3,
			maxRequests:  3,
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb := &CircuitBreaker{
				State:           tt.state,
				lastFailureTime: tt.failureTime,
				successCount:    tt.successCount,
				Timeout:         tt.timeout,
				MaxRequests:     tt.maxRequests,
			}

			result := cb.AllowRequest()
			if result != tt.expected {
				t.Errorf("AllowRequest() = %v, expected %v", result, tt.expected)
			}

			// Test state transition for open -> half-open case
			if tt.state == CircuitOpen && !tt.failureTime.IsZero() && 
				time.Since(tt.failureTime) > tt.timeout && tt.expected {
				if cb.State != CircuitHalfOpen {
					t.Errorf("Expected state transition to CircuitHalfOpen, got %v", cb.State)
				}
			}
		})
	}
}

func TestCircuitBreakerRecordSuccess(t *testing.T) {
	tests := []struct {
		name           string
		initialState   CircuitBreakerState
		successCount   int
		maxRequests    int
		expectedState  CircuitBreakerState
		expectedCount  int
		expectedFailures int
	}{
		{
			name:             "Success in closed state",
			initialState:     CircuitClosed,
			successCount:     0,
			maxRequests:      3,
			expectedState:    CircuitClosed,
			expectedCount:    0,
			expectedFailures: 0,
		},
		{
			name:             "Success in half-open under max",
			initialState:     CircuitHalfOpen,
			successCount:     1,
			maxRequests:      3,
			expectedState:    CircuitHalfOpen,
			expectedCount:    2,
			expectedFailures: 0,
		},
		{
			name:             "Success in half-open reaching max",
			initialState:     CircuitHalfOpen,
			successCount:     2,
			maxRequests:      3,
			expectedState:    CircuitClosed,
			expectedCount:    3,
			expectedFailures: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb := &CircuitBreaker{
				State:        tt.initialState,
				successCount: tt.successCount,
				MaxRequests:  tt.maxRequests,
				failureCount: 5, // Should be reset to 0
			}

			cb.RecordSuccess()

			if cb.State != tt.expectedState {
				t.Errorf("Expected state %v, got %v", tt.expectedState, cb.State)
			}
			if cb.successCount != tt.expectedCount {
				t.Errorf("Expected success count %d, got %d", tt.expectedCount, cb.successCount)
			}
			if cb.failureCount != tt.expectedFailures {
				t.Errorf("Expected failure count %d, got %d", tt.expectedFailures, cb.failureCount)
			}
		})
	}
}

func TestCircuitBreakerRecordFailure(t *testing.T) {
	tests := []struct {
		name          string
		initialState  CircuitBreakerState
		failureCount  int
		maxFailures   int
		expectedState CircuitBreakerState
		expectedCount int
	}{
		{
			name:          "Failure under threshold",
			initialState:  CircuitClosed,
			failureCount:  2,
			maxFailures:   5,
			expectedState: CircuitClosed,
			expectedCount: 3,
		},
		{
			name:          "Failure reaching threshold",
			initialState:  CircuitClosed,
			failureCount:  4,
			maxFailures:   5,
			expectedState: CircuitOpen,
			expectedCount: 5,
		},
		{
			name:          "Failure in half-open",
			initialState:  CircuitHalfOpen,
			failureCount:  0,
			maxFailures:   3,
			expectedState: CircuitHalfOpen,
			expectedCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb := &CircuitBreaker{
				State:        tt.initialState,
				failureCount: tt.failureCount,
				MaxFailures:  tt.maxFailures,
			}

			beforeTime := time.Now()
			cb.RecordFailure()
			afterTime := time.Now()

			if cb.State != tt.expectedState {
				t.Errorf("Expected state %v, got %v", tt.expectedState, cb.State)
			}
			if cb.failureCount != tt.expectedCount {
				t.Errorf("Expected failure count %d, got %d", tt.expectedCount, cb.failureCount)
			}

			// Verify lastFailureTime was updated
			if cb.lastFailureTime.Before(beforeTime) || cb.lastFailureTime.After(afterTime) {
				t.Errorf("lastFailureTime not properly updated")
			}
		})
	}
}

func TestWrapHTTPError(t *testing.T) {
	config := &ServerConfig{
		LSPGatewayURL: "http://localhost:8080",
		Timeout:       30 * time.Second,
		MaxRetries:    3,
	}
	client := NewLSPGatewayClient(config)

	testURL := "http://localhost:8080/jsonrpc"
	duration := 5 * time.Second

	tests := []struct {
		name        string
		err         error
		expectedMsg string
	}{
		{
			name:        "Nil error returns nil",
			err:         nil,
			expectedMsg: "",
		},
		{
			name:        "Network timeout error",
			err:         &net.OpError{Op: "read", Net: "tcp", Err: &timeoutError{}},
			expectedMsg: "HTTP request timeout after",
		},
		{
			name:        "URL timeout error",
			err:         &net.OpError{Op: "dial", Net: "tcp", Err: &timeoutError{}},
			expectedMsg: "HTTP request timeout after",
		},
		{
			name:        "Connection refused error",
			err:         &net.OpError{Op: "dial", Err: &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}},
			expectedMsg: "connection failed to",
		},
		{
			name:        "Host unreachable error",
			err:         &net.OpError{Op: "dial", Err: &net.OpError{Op: "dial", Err: syscall.EHOSTUNREACH}},
			expectedMsg: "connection failed to",
		},
		{
			name:        "Network unreachable error",
			err:         &net.OpError{Op: "dial", Err: &net.OpError{Op: "dial", Err: syscall.ENETUNREACH}},
			expectedMsg: "connection failed to",
		},
		{
			name:        "Generic HTTP error",
			err:         errors.New("generic http error"),
			expectedMsg: "HTTP request failed to",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.wrapHTTPError(tt.err, testURL, duration)

			if tt.err == nil {
				if result != nil {
					t.Errorf("Expected nil for nil error, got %v", result)
				}
				return
			}

			if result == nil {
				t.Errorf("Expected non-nil error, got nil")
				return
			}

			if !strings.Contains(result.Error(), tt.expectedMsg) {
				t.Errorf("Expected error message to contain '%s', got '%s'", tt.expectedMsg, result.Error())
			}
		})
	}
}

// timeoutError implements net.Error with Timeout() returning true
type timeoutError struct{}

func (e *timeoutError) Error() string   { return "timeout" }
func (e *timeoutError) Timeout() bool   { return true }
func (e *timeoutError) Temporary() bool { return false }

func TestCreateHTTPStatusError(t *testing.T) {
	config := &ServerConfig{
		LSPGatewayURL: "http://localhost:8080",
		Timeout:       30 * time.Second,
		MaxRetries:    3,
	}
	client := NewLSPGatewayClient(config)

	testURL := "http://localhost:8080/jsonrpc"
	testBody := "error response body"

	tests := []struct {
		name        string
		statusCode  int
		expectedMsg string
	}{
		{
			name:        "500 server error",
			statusCode:  500,
			expectedMsg: "server error 500",
		},
		{
			name:        "502 bad gateway",
			statusCode:  502,
			expectedMsg: "server error 502",
		},
		{
			name:        "429 rate limit",
			statusCode:  429,
			expectedMsg: "rate limit exceeded 429",
		},
		{
			name:        "400 bad request",
			statusCode:  400,
			expectedMsg: "client error 400",
		},
		{
			name:        "404 not found",
			statusCode:  404,
			expectedMsg: "client error 404",
		},
		{
			name:        "301 redirect",
			statusCode:  301,
			expectedMsg: "HTTP error 301",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.createHTTPStatusError(tt.statusCode, testBody, testURL)

			if err == nil {
				t.Errorf("Expected non-nil error")
				return
			}

			errorMsg := err.Error()
			if !strings.Contains(errorMsg, tt.expectedMsg) {
				t.Errorf("Expected error message to contain '%s', got '%s'", tt.expectedMsg, errorMsg)
			}

			if !strings.Contains(errorMsg, testURL) {
				t.Errorf("Expected error message to contain URL '%s', got '%s'", testURL, errorMsg)
			}

			if !strings.Contains(errorMsg, testBody) {
				t.Errorf("Expected error message to contain body '%s', got '%s'", testBody, errorMsg)
			}
		})
	}
}

func TestEnhanceError(t *testing.T) {
	config := &ServerConfig{
		LSPGatewayURL: "http://localhost:8080",
		Timeout:       30 * time.Second,
		MaxRetries:    3,
	}
	client := NewLSPGatewayClient(config)

	testMethod := "textDocument/definition"
	originalErr := errors.New("original error")

	tests := []struct {
		name        string
		inputErr    error
		expectedMsg string
	}{
		{
			name:        "Network error enhancement",
			inputErr:    errors.New("connection refused"),
			expectedMsg: "network error for LSP method textDocument/definition",
		},
		{
			name:        "Timeout error enhancement",
			inputErr:    errors.New("timeout exceeded"),
			expectedMsg: "timeout error for LSP method textDocument/definition",
		},
		{
			name:        "Server error enhancement",
			inputErr:    errors.New("HTTP error 500"),
			expectedMsg: "server error for LSP method textDocument/definition",
		},
		{
			name:        "Rate limit error enhancement",
			inputErr:    errors.New("HTTP error 429"),
			expectedMsg: "rate limit error for LSP method textDocument/definition",
		},
		{
			name:        "Protocol error enhancement",
			inputErr:    errors.New("failed to decode"),
			expectedMsg: "protocol error for LSP method textDocument/definition",
		},
		{
			name:        "Unknown error enhancement",
			inputErr:    originalErr,
			expectedMsg: "error for LSP method textDocument/definition",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enhanced := client.enhanceError(tt.inputErr, testMethod)

			if enhanced == nil {
				t.Errorf("Expected non-nil enhanced error")
				return
			}

			errorMsg := enhanced.Error()
			if !strings.Contains(errorMsg, tt.expectedMsg) {
				t.Errorf("Expected error message to contain '%s', got '%s'", tt.expectedMsg, errorMsg)
			}

			// Verify original error is wrapped
			if !errors.Is(enhanced, tt.inputErr) {
				t.Errorf("Expected enhanced error to wrap original error")
			}
		})
	}
}

func TestEnhanceJSONRPCError(t *testing.T) {
	config := &ServerConfig{
		LSPGatewayURL: "http://localhost:8080",
		Timeout:       30 * time.Second,
		MaxRetries:    3,
	}
	client := NewLSPGatewayClient(config)

	testMethod := "textDocument/definition"

	tests := []struct {
		name        string
		rpcError    *JSONRPCError
		expectedMsg string
	}{
		{
			name: "Parse error",
			rpcError: &JSONRPCError{
				Code:    -32700,
				Message: "Parse error",
			},
			expectedMsg: "JSON-RPC parse error for method textDocument/definition",
		},
		{
			name: "Invalid request",
			rpcError: &JSONRPCError{
				Code:    -32600,
				Message: "Invalid Request",
			},
			expectedMsg: "JSON-RPC invalid request for method textDocument/definition",
		},
		{
			name: "Method not found",
			rpcError: &JSONRPCError{
				Code:    -32601,
				Message: "Method not found",
			},
			expectedMsg: "JSON-RPC method not found textDocument/definition",
		},
		{
			name: "Invalid parameters",
			rpcError: &JSONRPCError{
				Code:    -32602,
				Message: "Invalid params",
			},
			expectedMsg: "JSON-RPC invalid parameters for method textDocument/definition",
		},
		{
			name: "Internal error",
			rpcError: &JSONRPCError{
				Code:    -32603,
				Message: "Internal error",
			},
			expectedMsg: "JSON-RPC internal error for method textDocument/definition",
		},
		{
			name: "Server error in range",
			rpcError: &JSONRPCError{
				Code:    -32001,
				Message: "Server error",
			},
			expectedMsg: "JSON-RPC server error -32001 for method textDocument/definition",
		},
		{
			name: "Custom error",
			rpcError: &JSONRPCError{
				Code:    1001,
				Message: "Custom error",
			},
			expectedMsg: "JSON-RPC error 1001 for method textDocument/definition",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enhanced := client.enhanceJSONRPCError(tt.rpcError, testMethod)

			if enhanced == nil {
				t.Errorf("Expected non-nil enhanced error")
				return
			}

			errorMsg := enhanced.Error()
			if !strings.Contains(errorMsg, tt.expectedMsg) {
				t.Errorf("Expected error message to contain '%s', got '%s'", tt.expectedMsg, errorMsg)
			}

			if !strings.Contains(errorMsg, tt.rpcError.Message) {
				t.Errorf("Expected error message to contain original message '%s', got '%s'", tt.rpcError.Message, errorMsg)
			}
		})
	}
}

func TestGenerateRequestID(t *testing.T) {
	// Test that GenerateRequestID produces unique IDs
	ids := make(map[interface{}]bool)
	iterations := 100

	for i := 0; i < iterations; i++ {
		id := GenerateRequestID()
		
		if id == nil {
			t.Errorf("GenerateRequestID() returned nil")
			continue
		}

		if ids[id] {
			t.Errorf("GenerateRequestID() produced duplicate ID: %v", id)
		}
		ids[id] = true

		// Verify ID format (should be string with timestamp-randomValue)
		idStr, ok := id.(string)
		if !ok {
			t.Errorf("GenerateRequestID() returned non-string: %T", id)
			continue
		}

		if !strings.Contains(idStr, "-") {
			t.Errorf("GenerateRequestID() returned invalid format: %s", idStr)
		}
	}
}

func TestRetryPolicyIntegration(t *testing.T) {
	// Test server that returns errors for first N requests then succeeds
	var requestCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, `{"jsonrpc":"2.0","error":{"code":-32603,"message":"Internal error"},"id":1}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"jsonrpc":"2.0","result":{"success":true},"id":1}`)
	}))
	defer server.Close()

	config := &ServerConfig{
		LSPGatewayURL: server.URL,
		Timeout:       5 * time.Second,
		MaxRetries:    3,
	}
	
	client := NewLSPGatewayClient(config)
	
	// Configure shorter backoff for faster testing
	client.retryPolicy.InitialBackoff = 10 * time.Millisecond
	client.retryPolicy.MaxBackoff = 100 * time.Millisecond
	client.retryPolicy.JitterEnabled = false

	ctx := context.Background()
	result, err := client.SendLSPRequest(ctx, "textDocument/definition", map[string]interface{}{
		"textDocument": map[string]string{"uri": "file:///test.go"},
		"position":     map[string]int{"line": 0, "character": 0},
	})

	if err != nil {
		t.Errorf("Expected request to succeed after retries, got error: %v", err)
	}

	if result == nil {
		t.Errorf("Expected non-nil result")
	}

	// Verify that retries actually happened
	if requestCount != 3 {
		t.Errorf("Expected 3 requests (2 failures + 1 success), got %d", requestCount)
	}
}

func TestCircuitBreakerIntegration(t *testing.T) {
	// Test server that always returns errors
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Internal Server Error")
	}))
	defer server.Close()

	config := &ServerConfig{
		LSPGatewayURL: server.URL,
		Timeout:       1 * time.Second,
		MaxRetries:    1,
	}
	
	client := NewLSPGatewayClient(config)
	
	// Configure circuit breaker to open quickly
	client.SetCircuitBreakerConfig(2, 5*time.Second)
	
	// Configure faster retries
	client.retryPolicy.InitialBackoff = 10 * time.Millisecond
	client.retryPolicy.MaxBackoff = 50 * time.Millisecond
	client.retryPolicy.JitterEnabled = false

	ctx := context.Background()
	
	// Make enough requests to trigger circuit breaker
	for i := 0; i < 3; i++ {
		_, err := client.SendLSPRequest(ctx, "textDocument/definition", map[string]interface{}{})
		if err == nil {
			t.Errorf("Expected error on request %d", i+1)
		}
	}

	// Verify circuit breaker is open
	if client.GetCircuitBreakerState() != CircuitOpen {
		t.Errorf("Expected circuit breaker to be open, got %v", client.GetCircuitBreakerState())
	}

	// Next request should be immediately rejected
	_, err := client.SendLSPRequest(ctx, "textDocument/definition", map[string]interface{}{})
	if err == nil {
		t.Errorf("Expected immediate rejection when circuit breaker is open")
	}
	
	if !strings.Contains(err.Error(), "circuit breaker is open") {
		t.Errorf("Expected circuit breaker error message, got: %v", err)
	}
}