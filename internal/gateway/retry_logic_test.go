package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestExponentialBackoffImplementation tests exponential backoff with proper jitter calculation
func TestExponentialBackoffImplementation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		baseDelay       time.Duration
		maxDelay        time.Duration
		jitterPercent   float64
		attempts        int
		expectedPattern string
	}{
		{
			name:            "standard exponential backoff",
			baseDelay:       100 * time.Millisecond,
			maxDelay:        5 * time.Second,
			jitterPercent:   0.25,
			attempts:        5,
			expectedPattern: "exponential_growth",
		},
		{
			name:            "aggressive backoff with high jitter",
			baseDelay:       50 * time.Millisecond,
			maxDelay:        2 * time.Second,
			jitterPercent:   0.5,
			attempts:        6,
			expectedPattern: "exponential_with_jitter",
		},
		{
			name:            "conservative backoff with cap",
			baseDelay:       200 * time.Millisecond,
			maxDelay:        1 * time.Second,
			jitterPercent:   0.1,
			attempts:        8,
			expectedPattern: "capped_exponential",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backoffCalculator := NewExponentialBackoffCalculator(tt.baseDelay, tt.maxDelay, tt.jitterPercent)

			delays := make([]time.Duration, tt.attempts)
			for i := 0; i < tt.attempts; i++ {
				delays[i] = backoffCalculator.CalculateDelay(i + 1)
			}

			// Validate backoff pattern
			switch tt.expectedPattern {
			case "exponential_growth":
				validateExponentialGrowth(t, delays, tt.baseDelay, tt.maxDelay)
			case "exponential_with_jitter":
				validateExponentialWithJitter(t, delays, tt.baseDelay, tt.maxDelay, tt.jitterPercent)
			case "capped_exponential":
				validateCappedExponential(t, delays, tt.baseDelay, tt.maxDelay)
			}

			// Log calculated delays for inspection
			t.Logf("Backoff delays for %s:", tt.name)
			for i, delay := range delays {
				t.Logf("  Attempt %d: %v", i+1, delay)
			}
		})
	}
}

// TestMaximumRetryLimitEnforcement tests maximum retry limit enforcement and circuit breaker activation
func TestMaximumRetryLimitEnforcement(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                      string
		maxRetries                int
		failureType               string
		expectedCircuitActivation bool
		timeoutDuration           time.Duration
	}{
		{
			name:                      "circuit breaker on network errors",
			maxRetries:                3,
			failureType:               "network_error",
			expectedCircuitActivation: true,
			timeoutDuration:           10 * time.Second,
		},
		{
			name:                      "circuit breaker on timeout errors",
			maxRetries:                5,
			failureType:               "timeout_error",
			expectedCircuitActivation: true,
			timeoutDuration:           15 * time.Second,
		},
		{
			name:                      "no circuit breaker on application errors",
			maxRetries:                3,
			failureType:               "application_error",
			expectedCircuitActivation: false,
			timeoutDuration:           8 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := CreateRetryTestServer(t, tt.failureType, tt.maxRetries+2)
			defer server.Close()

			retryHandler := NewRetryHandler(RetryConfig{
				MaxRetries:    tt.maxRetries,
				BaseDelay:     50 * time.Millisecond,
				MaxDelay:      1 * time.Second,
				JitterPercent: 0.25,
			})

			ctx, cancel := context.WithTimeout(context.Background(), tt.timeoutDuration)
			defer cancel()

			// Track retry attempts
			attemptCount := 0
			circuitBreakerActivated := false

			// Create request that will trigger retries
			request := &RetryableRequest{
				Method: "textDocument/definition",
				Params: map[string]interface{}{
					"textDocument": map[string]interface{}{"uri": "file:///test.go"},
					"position":     map[string]interface{}{"line": 10, "character": 5},
				},
				URL: server.URL,
			}

			result, err := retryHandler.ExecuteWithRetry(ctx, request, func(ctx context.Context, req *RetryableRequest) (*RetryResult, error) {
				attemptCount++

				// Simulate different failure types
				switch tt.failureType {
				case "network_error":
					if attemptCount <= tt.maxRetries {
						return nil, &NetworkError{Message: fmt.Sprintf("network failure attempt %d", attemptCount)}
					}
				case "timeout_error":
					if attemptCount <= tt.maxRetries {
						return nil, &TimeoutError{Message: fmt.Sprintf("timeout failure attempt %d", attemptCount)}
					}
				case "application_error":
					if attemptCount <= tt.maxRetries {
						return nil, &ApplicationError{Message: fmt.Sprintf("application failure attempt %d", attemptCount)}
					}
				}

				// Success case
				return &RetryResult{
					Success: true,
					Data:    json.RawMessage(`{"result": "success"}`),
				}, nil
			})

			// Validate retry behavior
			t.Logf("Completed after %d attempts", attemptCount)

			if tt.expectedCircuitActivation {
				if attemptCount <= tt.maxRetries {
					t.Errorf("Expected retry limit to be enforced, but got %d attempts (max: %d)", attemptCount, tt.maxRetries)
				}

				if err == nil {
					t.Error("Expected error when circuit breaker activates")
				}

				if retryHandler.IsCircuitOpen() {
					circuitBreakerActivated = true
				}

				if !circuitBreakerActivated {
					t.Error("Circuit breaker should have been activated")
				}
			} else {
				if attemptCount > tt.maxRetries+1 {
					t.Errorf("Retry count exceeded limit for non-retriable errors: %d > %d", attemptCount, tt.maxRetries+1)
				}

				if tt.failureType == "application_error" && result == nil {
					t.Error("Application errors should not trigger circuit breaker")
				}
			}
		})
	}
}

// TestRequestDeduplicationDuringRetries tests request deduplication during retry scenarios
func TestRequestDeduplicationDuringRetries(t *testing.T) {
	t.Parallel()

	t.Run("duplicate request prevention", func(t *testing.T) {
		server := CreateDeduplicationTestServer(t)
		defer server.Close()

		deduplicator := NewRequestDeduplicator(1 * time.Minute)
		retryHandler := NewRetryHandler(RetryConfig{
			MaxRetries:    3,
			BaseDelay:     100 * time.Millisecond,
			MaxDelay:      1 * time.Second,
			JitterPercent: 0.25,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Create identical requests
		request1 := &RetryableRequest{
			Method: "textDocument/definition",
			Params: map[string]interface{}{
				"textDocument": map[string]interface{}{"uri": "file:///test.go"},
				"position":     map[string]interface{}{"line": 10, "character": 5},
			},
			URL: server.URL,
		}

		request2 := &RetryableRequest{
			Method: "textDocument/definition",
			Params: map[string]interface{}{
				"textDocument": map[string]interface{}{"uri": "file:///test.go"},
				"position":     map[string]interface{}{"line": 10, "character": 5},
			},
			URL: server.URL,
		}

		// Execute requests concurrently
		var wg sync.WaitGroup
		results := make([]*RetryResult, 2)
		errors := make([]error, 2)

		executeRequest := func(idx int, req *RetryableRequest) {
			defer wg.Done()

			result, err := deduplicator.ExecuteWithDeduplication(ctx, req, func(ctx context.Context, r *RetryableRequest) (*RetryResult, error) {
				return retryHandler.ExecuteWithRetry(ctx, r, func(ctx context.Context, r *RetryableRequest) (*RetryResult, error) {
					// Simulate server request
					response, err := http.Post(r.URL, "application/json", strings.NewReader(`{"test": "request"}`))
					if err != nil {
						return nil, err
					}
					defer response.Body.Close()

					return &RetryResult{
						Success: true,
						Data:    json.RawMessage(`{"result": "deduplicated"}`),
					}, nil
				})
			})

			results[idx] = result
			errors[idx] = err
		}

		wg.Add(2)
		go executeRequest(0, request1)
		go executeRequest(1, request2)

		wg.Wait()

		// Validate deduplication
		serverRequestCount := server.GetRequestCount()
		t.Logf("Server received %d requests for 2 duplicate requests", serverRequestCount)

		if serverRequestCount > 1 {
			t.Errorf("Expected 1 server request due to deduplication, got %d", serverRequestCount)
		}

		// Both requests should have results
		for i, result := range results {
			if result == nil && errors[i] != nil {
				t.Errorf("Request %d failed: %v", i+1, errors[i])
			}
		}
	})

	t.Run("deduplication cache expiration", func(t *testing.T) {
		server := CreateDeduplicationTestServer(t)
		defer server.Close()

		deduplicator := NewRequestDeduplicator(200 * time.Millisecond) // Short expiration

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		request := &RetryableRequest{
			Method: "textDocument/definition",
			Params: map[string]interface{}{
				"textDocument": map[string]interface{}{"uri": "file:///test.go"},
				"position":     map[string]interface{}{"line": 10, "character": 5},
			},
			URL: server.URL,
		}

		// First request
		_, err := deduplicator.ExecuteWithDeduplication(ctx, request, func(ctx context.Context, r *RetryableRequest) (*RetryResult, error) {
			response, err := http.Post(r.URL, "application/json", strings.NewReader(`{"test": "request1"}`))
			if err != nil {
				return nil, err
			}
			defer response.Body.Close()

			return &RetryResult{Success: true, Data: json.RawMessage(`{"result": "first"}`)}, nil
		})

		if err != nil {
			t.Fatalf("First request failed: %v", err)
		}

		// Wait for cache expiration
		time.Sleep(300 * time.Millisecond)

		// Second request (after expiration)
		_, err = deduplicator.ExecuteWithDeduplication(ctx, request, func(ctx context.Context, r *RetryableRequest) (*RetryResult, error) {
			response, err := http.Post(r.URL, "application/json", strings.NewReader(`{"test": "request2"}`))
			if err != nil {
				return nil, err
			}
			defer response.Body.Close()

			return &RetryResult{Success: true, Data: json.RawMessage(`{"result": "second"}`)}, nil
		})

		if err != nil {
			t.Fatalf("Second request failed: %v", err)
		}

		// Should have received 2 requests due to cache expiration
		serverRequestCount := server.GetRequestCount()
		if serverRequestCount != 2 {
			t.Errorf("Expected 2 server requests after cache expiration, got %d", serverRequestCount)
		}
	})
}

// TestRetryBehaviorForDifferentErrorTypes tests retry behavior for different error types
func TestRetryBehaviorForDifferentErrorTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		errorType        string
		shouldRetry      bool
		maxRetries       int
		expectedAttempts int
	}{
		{
			name:             "network connection error",
			errorType:        "network_connection",
			shouldRetry:      true,
			maxRetries:       3,
			expectedAttempts: 4, // initial + 3 retries
		},
		{
			name:             "timeout error",
			errorType:        "timeout",
			shouldRetry:      true,
			maxRetries:       2,
			expectedAttempts: 3, // initial + 2 retries
		},
		{
			name:             "HTTP 500 server error",
			errorType:        "http_500",
			shouldRetry:      true,
			maxRetries:       3,
			expectedAttempts: 4,
		},
		{
			name:             "HTTP 429 rate limit",
			errorType:        "http_429",
			shouldRetry:      true,
			maxRetries:       5,
			expectedAttempts: 6,
		},
		{
			name:             "HTTP 400 bad request",
			errorType:        "http_400",
			shouldRetry:      false,
			maxRetries:       3,
			expectedAttempts: 1, // no retries for client errors
		},
		{
			name:             "JSON-RPC parse error",
			errorType:        "jsonrpc_parse",
			shouldRetry:      false,
			maxRetries:       3,
			expectedAttempts: 1,
		},
		{
			name:             "LSP method not found",
			errorType:        "method_not_found",
			shouldRetry:      false,
			maxRetries:       3,
			expectedAttempts: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := CreateErrorTypeTestServer(t, tt.errorType)
			defer server.Close()

			retryHandler := NewRetryHandler(RetryConfig{
				MaxRetries:    tt.maxRetries,
				BaseDelay:     50 * time.Millisecond,
				MaxDelay:      500 * time.Millisecond,
				JitterPercent: 0.1,
			})

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			request := &RetryableRequest{
				Method: "textDocument/definition",
				Params: map[string]interface{}{
					"textDocument": map[string]interface{}{"uri": "file:///test.go"},
					"position":     map[string]interface{}{"line": 10, "character": 5},
				},
				URL: server.URL,
			}

			attemptCount := 0
			_, err := retryHandler.ExecuteWithRetry(ctx, request, func(ctx context.Context, req *RetryableRequest) (*RetryResult, error) {
				attemptCount++
				return server.SimulateError(tt.errorType)
			})

			// Validate retry behavior
			if attemptCount != tt.expectedAttempts {
				t.Errorf("Expected %d attempts for %s, got %d", tt.expectedAttempts, tt.errorType, attemptCount)
			}

			if tt.shouldRetry && err == nil {
				t.Errorf("Expected error for retriable failure type %s", tt.errorType)
			}

			if !tt.shouldRetry && attemptCount > 1 {
				t.Errorf("Should not retry for error type %s, but got %d attempts", tt.errorType, attemptCount)
			}

			t.Logf("Error type %s: %d attempts (expected %d), error: %v", tt.errorType, attemptCount, tt.expectedAttempts, err)
		})
	}
}

// TestConcurrentRequestRetryHandling tests concurrent request retry handling without interference
func TestConcurrentRequestRetryHandling(t *testing.T) {
	t.Parallel()

	t.Run("concurrent retry isolation", func(t *testing.T) {
		server := CreateConcurrentRetryTestServer(t)
		defer server.Close()

		retryHandler := NewRetryHandler(RetryConfig{
			MaxRetries:    3,
			BaseDelay:     100 * time.Millisecond,
			MaxDelay:      1 * time.Second,
			JitterPercent: 0.25,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		// Test concurrent requests with different failure patterns
		const numConcurrentRequests = 10
		var wg sync.WaitGroup
		results := make([]*RetryResult, numConcurrentRequests)
		errors := make([]error, numConcurrentRequests)
		attemptCounts := make([]int32, numConcurrentRequests)

		for i := 0; i < numConcurrentRequests; i++ {
			wg.Add(1)
			go func(requestID int) {
				defer wg.Done()

				request := &RetryableRequest{
					Method: "textDocument/definition",
					Params: map[string]interface{}{
						"textDocument": map[string]interface{}{"uri": fmt.Sprintf("file:///test%d.go", requestID)},
						"position":     map[string]interface{}{"line": requestID, "character": 5},
					},
					URL: server.URL,
				}

				result, err := retryHandler.ExecuteWithRetry(ctx, request, func(ctx context.Context, req *RetryableRequest) (*RetryResult, error) {
					count := atomic.AddInt32(&attemptCounts[requestID], 1)

					// Different failure patterns for different requests
					if requestID%3 == 0 {
						// Always succeed immediately
						return &RetryResult{Success: true, Data: json.RawMessage(fmt.Sprintf(`{"requestID": %d}`, requestID))}, nil
					} else if requestID%3 == 1 {
						// Fail first 2 attempts, then succeed
						if count <= 2 {
							return nil, &NetworkError{Message: fmt.Sprintf("network error for request %d attempt %d", requestID, count)}
						}
						return &RetryResult{Success: true, Data: json.RawMessage(fmt.Sprintf(`{"requestID": %d}`, requestID))}, nil
					} else {
						// Always fail
						return nil, &NetworkError{Message: fmt.Sprintf("persistent error for request %d attempt %d", requestID, count)}
					}
				})

				results[requestID] = result
				errors[requestID] = err
			}(i)
		}

		wg.Wait()

		// Validate concurrent behavior
		successCount := 0
		failureCount := 0

		for i := 0; i < numConcurrentRequests; i++ {
			attempts := atomic.LoadInt32(&attemptCounts[i])
			t.Logf("Request %d: %d attempts, success: %v, error: %v", i, attempts, results[i] != nil, errors[i])

			if results[i] != nil {
				successCount++
			} else {
				failureCount++
			}

			// Validate retry isolation
			if i%3 == 0 && attempts != 1 {
				t.Errorf("Request %d should succeed immediately, got %d attempts", i, attempts)
			} else if i%3 == 1 && attempts != 3 {
				t.Errorf("Request %d should succeed after 3 attempts, got %d attempts", i, attempts)
			} else if i%3 == 2 && attempts != 4 {
				t.Errorf("Request %d should fail after 4 attempts, got %d attempts", i, attempts)
			}
		}

		t.Logf("Concurrent retry test: %d successes, %d failures", successCount, failureCount)

		// Expected: 4 immediate successes, 3 eventual successes, 3 failures
		expectedSuccesses := 7
		expectedFailures := 3

		if successCount != expectedSuccesses {
			t.Errorf("Expected %d successes, got %d", expectedSuccesses, successCount)
		}

		if failureCount != expectedFailures {
			t.Errorf("Expected %d failures, got %d", expectedFailures, failureCount)
		}
	})
}

// TestTransientNetworkFailureRecovery tests retry behavior for transient network failures with recovery
func TestTransientNetworkFailureRecovery(t *testing.T) {
	t.Parallel()

	t.Run("transient failure recovery patterns", func(t *testing.T) {
		server := CreateTransientFailureTestServer(t)
		defer server.Close()

		retryHandler := NewRetryHandler(RetryConfig{
			MaxRetries:    5,
			BaseDelay:     100 * time.Millisecond,
			MaxDelay:      2 * time.Second,
			JitterPercent: 0.25,
		})

		// Test different transient failure patterns
		failurePatterns := []struct {
			name                string
			failureCount        int
			recoveryDelay       time.Duration
			expectedSuccess     bool
			maxExpectedAttempts int
		}{
			{
				name:                "single transient failure",
				failureCount:        1,
				recoveryDelay:       200 * time.Millisecond,
				expectedSuccess:     true,
				maxExpectedAttempts: 2,
			},
			{
				name:                "multiple transient failures",
				failureCount:        3,
				recoveryDelay:       500 * time.Millisecond,
				expectedSuccess:     true,
				maxExpectedAttempts: 4,
			},
			{
				name:                "extended transient failure",
				failureCount:        6,
				recoveryDelay:       1 * time.Second,
				expectedSuccess:     false,
				maxExpectedAttempts: 6,
			},
		}

		for _, pattern := range failurePatterns {
			t.Run(pattern.name, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()

				server.SetFailurePattern(pattern.failureCount, pattern.recoveryDelay)

				request := &RetryableRequest{
					Method: "textDocument/definition",
					Params: map[string]interface{}{
						"textDocument": map[string]interface{}{"uri": "file:///test.go"},
						"position":     map[string]interface{}{"line": 10, "character": 5},
					},
					URL: server.URL,
				}

				attemptCount := 0
				startTime := time.Now()

				result, err := retryHandler.ExecuteWithRetry(ctx, request, func(ctx context.Context, req *RetryableRequest) (*RetryResult, error) {
					attemptCount++
					return server.HandleTransientFailure()
				})

				duration := time.Since(startTime)

				// Validate recovery behavior
				t.Logf("Pattern %s: %d attempts in %v, success: %v", pattern.name, attemptCount, duration, result != nil)

				if pattern.expectedSuccess {
					if result == nil {
						t.Errorf("Expected success for pattern %s, got error: %v", pattern.name, err)
					}
					if attemptCount > pattern.maxExpectedAttempts {
						t.Errorf("Too many attempts for pattern %s: %d > %d", pattern.name, attemptCount, pattern.maxExpectedAttempts)
					}
				} else {
					if result != nil {
						t.Errorf("Expected failure for pattern %s, got success", pattern.name)
					}
					if attemptCount != pattern.maxExpectedAttempts {
						t.Errorf("Expected exactly %d attempts for pattern %s, got %d", pattern.maxExpectedAttempts, pattern.name, attemptCount)
					}
				}
			})
		}
	})
}

// Helper types and functions for retry testing

type RetryableRequest struct {
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params"`
	URL    string                 `json:"url"`
}

type RetryResult struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
}

type RetryConfig struct {
	MaxRetries    int           `json:"maxRetries"`
	BaseDelay     time.Duration `json:"baseDelay"`
	MaxDelay      time.Duration `json:"maxDelay"`
	JitterPercent float64       `json:"jitterPercent"`
}

// Error types for testing different retry behaviors
type NetworkError struct {
	Message string
}

func (e *NetworkError) Error() string {
	return e.Message
}

func (e *NetworkError) IsRetriable() bool {
	return true
}

type TimeoutError struct {
	Message string
}

func (e *TimeoutError) Error() string {
	return e.Message
}

func (e *TimeoutError) IsRetriable() bool {
	return true
}

type ApplicationError struct {
	Message string
}

func (e *ApplicationError) Error() string {
	return e.Message
}

func (e *ApplicationError) IsRetriable() bool {
	return false
}

// ExponentialBackoffCalculator implements exponential backoff with jitter
type ExponentialBackoffCalculator struct {
	baseDelay     time.Duration
	maxDelay      time.Duration
	jitterPercent float64
}

func NewExponentialBackoffCalculator(baseDelay, maxDelay time.Duration, jitterPercent float64) *ExponentialBackoffCalculator {
	return &ExponentialBackoffCalculator{
		baseDelay:     baseDelay,
		maxDelay:      maxDelay,
		jitterPercent: jitterPercent,
	}
}

func (e *ExponentialBackoffCalculator) CalculateDelay(attempt int) time.Duration {
	// Exponential backoff: baseDelay * 2^(attempt-1)
	backoff := float64(e.baseDelay) * math.Pow(2, float64(attempt-1))

	// Add jitter (±jitterPercent)
	jitter := 1.0 + (rand.Float64()-0.5)*e.jitterPercent*2
	delay := time.Duration(backoff * jitter)

	// Cap at maxDelay
	if delay > e.maxDelay {
		delay = e.maxDelay
	}

	return delay
}

// RetryHandler implements retry logic with exponential backoff
type RetryHandler struct {
	config      RetryConfig
	circuitOpen bool
	mu          sync.RWMutex
}

func NewRetryHandler(config RetryConfig) *RetryHandler {
	return &RetryHandler{
		config: config,
	}
}

func (r *RetryHandler) ExecuteWithRetry(ctx context.Context, request *RetryableRequest, executor func(context.Context, *RetryableRequest) (*RetryResult, error)) (*RetryResult, error) {
	calculator := NewExponentialBackoffCalculator(r.config.BaseDelay, r.config.MaxDelay, r.config.JitterPercent)

	var lastErr error
	for attempt := 1; attempt <= r.config.MaxRetries+1; attempt++ {
		result, err := executor(ctx, request)
		if err == nil {
			return result, nil
		}

		lastErr = err

		// Check if error is retriable
		if retriable, ok := err.(interface{ IsRetriable() bool }); ok && !retriable.IsRetriable() {
			return nil, err
		}

		// Check if we've reached max retries
		if attempt > r.config.MaxRetries {
			r.openCircuit()
			break
		}

		// Calculate backoff delay
		delay := calculator.CalculateDelay(attempt)

		// Wait with context cancellation support
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return nil, lastErr
}

func (r *RetryHandler) IsCircuitOpen() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.circuitOpen
}

func (r *RetryHandler) openCircuit() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.circuitOpen = true
}

// RequestDeduplicator prevents duplicate requests
type RequestDeduplicator struct {
	cache      map[string]*pendingRequest
	mu         sync.RWMutex
	expiration time.Duration
}

type pendingRequest struct {
	result    *RetryResult
	err       error
	done      chan struct{}
	timestamp time.Time
}

func NewRequestDeduplicator(expiration time.Duration) *RequestDeduplicator {
	return &RequestDeduplicator{
		cache:      make(map[string]*pendingRequest),
		expiration: expiration,
	}
}

func (d *RequestDeduplicator) ExecuteWithDeduplication(ctx context.Context, request *RetryableRequest, executor func(context.Context, *RetryableRequest) (*RetryResult, error)) (*RetryResult, error) {
	key := d.generateRequestKey(request)

	d.mu.Lock()
	if pending, exists := d.cache[key]; exists && time.Since(pending.timestamp) < d.expiration {
		d.mu.Unlock()
		// Wait for existing request to complete
		select {
		case <-pending.done:
			return pending.result, pending.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// Create new pending request
	pending := &pendingRequest{
		done:      make(chan struct{}),
		timestamp: time.Now(),
	}
	d.cache[key] = pending
	d.mu.Unlock()

	// Execute request
	go func() {
		defer close(pending.done)
		pending.result, pending.err = executor(ctx, request)
	}()

	// Wait for completion
	select {
	case <-pending.done:
		return pending.result, pending.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (d *RequestDeduplicator) generateRequestKey(request *RetryableRequest) string {
	data, _ := json.Marshal(request)
	return fmt.Sprintf("%x", data)
}

// Test servers for different scenarios

func CreateRetryTestServer(t *testing.T, failureType string, failureCount int) *httptest.Server {
	requestCount := 0
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		if requestCount <= failureCount {
			switch failureType {
			case "network_error":
				http.Error(w, "Network error", http.StatusBadGateway)
			case "timeout_error":
				time.Sleep(2 * time.Second)
				http.Error(w, "Timeout", http.StatusRequestTimeout)
			case "application_error":
				http.Error(w, "Application error", http.StatusBadRequest)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"result": "success"}`))
	}))
}

func CreateDeduplicationTestServer(t *testing.T) *DeduplicationTestServer {
	server := &DeduplicationTestServer{}
	server.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&server.requestCount, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"result": "deduplicated"}`))
	}))
	return server
}

type DeduplicationTestServer struct {
	*httptest.Server
	requestCount int32
}

func (s *DeduplicationTestServer) GetRequestCount() int32 {
	return atomic.LoadInt32(&s.requestCount)
}

func CreateErrorTypeTestServer(t *testing.T, errorType string) *ErrorTypeTestServer {
	return &ErrorTypeTestServer{
		errorType: errorType,
	}
}

type ErrorTypeTestServer struct {
	errorType string
	URL       string
}

func (s *ErrorTypeTestServer) SimulateError(errorType string) (*RetryResult, error) {
	switch errorType {
	case "network_connection":
		return nil, &NetworkError{Message: "connection refused"}
	case "timeout":
		return nil, &TimeoutError{Message: "request timeout"}
	case "http_500":
		return nil, &NetworkError{Message: "internal server error"}
	case "http_429":
		return nil, &NetworkError{Message: "rate limit exceeded"}
	case "http_400":
		return nil, &ApplicationError{Message: "bad request"}
	case "jsonrpc_parse":
		return nil, &ApplicationError{Message: "JSON-RPC parse error"}
	case "method_not_found":
		return nil, &ApplicationError{Message: "method not found"}
	default:
		return &RetryResult{Success: true, Data: json.RawMessage(`{"result": "success"}`)}, nil
	}
}

func (s *ErrorTypeTestServer) Close() {
	// No-op for this mock server
}

func CreateConcurrentRetryTestServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"result": "concurrent"}`))
	}))
}

func CreateTransientFailureTestServer(t *testing.T) *TransientFailureTestServer {
	server := &TransientFailureTestServer{}
	server.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result, err := server.HandleTransientFailure()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}))
	return server
}

type TransientFailureTestServer struct {
	*httptest.Server
	failureCount  int
	recoveryDelay time.Duration
	currentCount  int32
	mu            sync.RWMutex
}

func (s *TransientFailureTestServer) SetFailurePattern(failureCount int, recoveryDelay time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failureCount = failureCount
	s.recoveryDelay = recoveryDelay
	s.currentCount = 0
}

func (s *TransientFailureTestServer) HandleTransientFailure() (*RetryResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.currentCount++

	if int(s.currentCount) <= s.failureCount {
		return nil, &NetworkError{Message: fmt.Sprintf("transient failure %d", s.currentCount)}
	}

	return &RetryResult{Success: true, Data: json.RawMessage(`{"result": "recovered"}`)}, nil
}

func (s *TransientFailureTestServer) Close() {
	// No-op for this mock server
}

// Validation functions for backoff patterns

func validateExponentialGrowth(t *testing.T, delays []time.Duration, baseDelay, maxDelay time.Duration) {
	for i := 1; i < len(delays); i++ {
		if delays[i] < delays[i-1] {
			t.Errorf("Delay should increase exponentially: %v < %v at attempt %d", delays[i], delays[i-1], i+1)
		}
	}

	if delays[0] < baseDelay/2 {
		t.Errorf("First delay %v should be close to base delay %v", delays[0], baseDelay)
	}

	for _, delay := range delays {
		if delay > maxDelay {
			t.Errorf("Delay %v should not exceed max delay %v", delay, maxDelay)
		}
	}
}

func validateExponentialWithJitter(t *testing.T, delays []time.Duration, baseDelay, maxDelay time.Duration, jitterPercent float64) {
	// Check that jitter is applied (delays shouldn't be identical for exponential sequence)
	identical := 0
	for i := 1; i < len(delays); i++ {
		if delays[i] == delays[i-1] {
			identical++
		}
	}

	if identical > 1 {
		t.Errorf("Too many identical delays (%d), jitter should add variance", identical)
	}

	// Validate jitter bounds
	for i, delay := range delays {
		expectedBase := float64(baseDelay) * math.Pow(2, float64(i))
		if expectedBase > float64(maxDelay) {
			expectedBase = float64(maxDelay)
		}

		minExpected := time.Duration(expectedBase * (1 - jitterPercent))
		maxExpected := time.Duration(expectedBase * (1 + jitterPercent))

		if delay < minExpected || delay > maxExpected {
			t.Errorf("Delay %v at attempt %d outside jitter bounds [%v, %v]", delay, i+1, minExpected, maxExpected)
		}
	}
}

func validateCappedExponential(t *testing.T, delays []time.Duration, baseDelay, maxDelay time.Duration) {
	// Find where capping should occur
	cappingPoint := -1
	for i := 0; i < len(delays); i++ {
		expectedDelay := float64(baseDelay) * math.Pow(2, float64(i))
		if expectedDelay >= float64(maxDelay) {
			cappingPoint = i
			break
		}
	}

	if cappingPoint > 0 {
		// Check that delays are capped after the capping point
		for i := cappingPoint; i < len(delays); i++ {
			if delays[i] > maxDelay*11/10 { // Allow 10% tolerance
				t.Errorf("Delay %v at attempt %d should be capped at %v", delays[i], i+1, maxDelay)
			}
		}
	}
}
