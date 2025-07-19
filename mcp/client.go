package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"lsp-gateway/internal/gateway"
)

// ErrorCategory represents different types of errors for retry logic
type ErrorCategory int

const (
	ErrorCategoryNetwork ErrorCategory = iota
	ErrorCategoryTimeout
	ErrorCategoryServer
	ErrorCategoryClient
	ErrorCategoryRateLimit
	ErrorCategoryProtocol
	ErrorCategoryUnknown
)

// RetryPolicy defines retry behavior for different error categories
type RetryPolicy struct {
	MaxRetries      int
	InitialBackoff  time.Duration
	MaxBackoff      time.Duration
	BackoffFactor   float64
	JitterEnabled   bool
	RetryableErrors map[ErrorCategory]bool
}

// CircuitBreakerState represents the state of the circuit breaker
type CircuitBreakerState int

const (
	CircuitClosed CircuitBreakerState = iota
	CircuitOpen
	CircuitHalfOpen
)

// CircuitBreaker implements circuit breaker pattern for connection failures
type CircuitBreaker struct {
	mu              sync.RWMutex
	state           CircuitBreakerState
	failureCount    int
	lastFailureTime time.Time
	successCount    int
	timeout         time.Duration
	maxFailures     int
	maxRequests     int
}

// ConnectionMetrics tracks connection health metrics
type ConnectionMetrics struct {
	mu               sync.RWMutex
	totalRequests    int64
	successfulReqs   int64
	failedRequests   int64
	timeoutCount     int64
	connectionErrors int64
	averageLatency   time.Duration
	lastRequestTime  time.Time
	lastSuccessTime  time.Time
}

// LSPGatewayClient handles communication with the LSP Gateway HTTP server
type LSPGatewayClient struct {
	baseURL         string
	httpClient      *http.Client
	timeout         time.Duration
	maxRetries      int
	retryPolicy     *RetryPolicy
	circuitBreaker  *CircuitBreaker
	metrics         *ConnectionMetrics
	logger          *log.Logger
	heathCheckURL   string
	lastHealthCheck time.Time
	healthCheckMu   sync.RWMutex
}

// JSONRPCRequest represents a JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC 2.0 error
type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// NewLSPGatewayClient creates a new LSP Gateway HTTP client with enhanced error handling
func NewLSPGatewayClient(config *ServerConfig) *LSPGatewayClient {
	logger := log.New(log.Writer(), "[LSPClient] ", log.LstdFlags|log.Lshortfile)

	// Configure HTTP client with optimized settings
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       60 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   config.Timeout,
	}

	// Configure retry policy
	retryPolicy := &RetryPolicy{
		MaxRetries:     config.MaxRetries,
		InitialBackoff: 500 * time.Millisecond,
		MaxBackoff:     30 * time.Second,
		BackoffFactor:  2.0,
		JitterEnabled:  true,
		RetryableErrors: map[ErrorCategory]bool{
			ErrorCategoryNetwork:   true,
			ErrorCategoryTimeout:   true,
			ErrorCategoryServer:    true,
			ErrorCategoryRateLimit: true,
			ErrorCategoryClient:    false,
			ErrorCategoryProtocol:  false,
			ErrorCategoryUnknown:   true,
		},
	}

	// Configure circuit breaker
	circuitBreaker := &CircuitBreaker{
		state:       CircuitClosed,
		timeout:     60 * time.Second,
		maxFailures: 5,
		maxRequests: 3,
	}

	return &LSPGatewayClient{
		baseURL:        config.LSPGatewayURL,
		httpClient:     httpClient,
		timeout:        config.Timeout,
		maxRetries:     config.MaxRetries,
		retryPolicy:    retryPolicy,
		circuitBreaker: circuitBreaker,
		metrics:        &ConnectionMetrics{},
		logger:         logger,
		heathCheckURL:  config.LSPGatewayURL + "/health",
	}
}

// SendLSPRequest sends a JSON-RPC request to the LSP Gateway with enhanced error handling
func (c *LSPGatewayClient) SendLSPRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	start := time.Now()

	// Update metrics
	c.metrics.mu.Lock()
	c.metrics.totalRequests++
	c.metrics.lastRequestTime = start
	c.metrics.mu.Unlock()

	// Check circuit breaker
	if !c.circuitBreaker.AllowRequest() {
		c.logger.Printf("Circuit breaker is open, rejecting request for method: %s", method)
		c.updateFailureMetrics()
		return nil, fmt.Errorf("circuit breaker is open: too many failures")
	}

	request := JSONRPCRequest{
		JSONRPC: gateway.JSONRPCVersion,
		ID:      generateRequestID(),
		Method:  method,
		Params:  params,
	}

	c.logger.Printf("Sending LSP request: method=%s, id=%v", method, request.ID)

	var response JSONRPCResponse
	err := c.sendRequestWithRetry(ctx, request, &response)

	// Update latency metrics
	latency := time.Since(start)
	c.updateLatencyMetrics(latency)

	if err != nil {
		c.logger.Printf("LSP request failed: method=%s, error=%v, latency=%v", method, err, latency)
		c.circuitBreaker.RecordFailure()
		c.updateFailureMetrics()
		return nil, c.enhanceError(fmt.Errorf("failed to send LSP request: %w", err), method)
	}

	if response.Error != nil {
		c.logger.Printf("LSP error response: method=%s, code=%d, message=%s", method, response.Error.Code, response.Error.Message)
		c.circuitBreaker.RecordFailure()
		c.updateFailureMetrics()
		return nil, c.enhanceJSONRPCError(response.Error, method)
	}

	c.logger.Printf("LSP request successful: method=%s, latency=%v", method, latency)
	c.circuitBreaker.RecordSuccess()
	c.updateSuccessMetrics()

	return response.Result, nil
}

// sendRequestWithRetry performs the actual HTTP request with sophisticated retry logic
func (c *LSPGatewayClient) sendRequestWithRetry(ctx context.Context, request JSONRPCRequest, response *JSONRPCResponse) error {
	var lastErr error

	for attempt := 0; attempt <= c.retryPolicy.MaxRetries; attempt++ {
		if attempt > 0 {
			// Calculate backoff with exponential backoff and jitter
			waitTime := c.calculateBackoff(attempt)
			c.logger.Printf("Retrying request (attempt %d/%d) after %v: method=%s", attempt+1, c.retryPolicy.MaxRetries+1, waitTime, request.Method)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(waitTime):
			}
		}

		err := c.sendSingleRequest(ctx, request, response)
		if err == nil {
			if attempt > 0 {
				c.logger.Printf("Request succeeded after %d retries: method=%s", attempt, request.Method)
			}
			return nil
		}

		lastErr = err
		errorCategory := c.categorizeError(err)

		c.logger.Printf("Request attempt %d failed: method=%s, error=%v, category=%v", attempt+1, request.Method, err, errorCategory)

		// Check if we should retry based on error category
		if !c.shouldRetryError(errorCategory) {
			c.logger.Printf("Not retrying due to error category: %v", errorCategory)
			break
		}

		// Don't retry if this is the last attempt
		if attempt >= c.retryPolicy.MaxRetries {
			break
		}
	}

	c.logger.Printf("Request failed after %d attempts: method=%s, final_error=%v", c.retryPolicy.MaxRetries+1, request.Method, lastErr)
	return fmt.Errorf("request failed after %d attempts: %w", c.retryPolicy.MaxRetries+1, lastErr)
}

// sendSingleRequest performs a single HTTP request attempt with comprehensive error handling
func (c *LSPGatewayClient) sendSingleRequest(ctx context.Context, request JSONRPCRequest, response *JSONRPCResponse) error {
	// Marshal request body
	requestBody, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	url := c.baseURL + "/jsonrpc"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(requestBody))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", "LSP-Gateway-MCP-Client/1.0")
	httpReq.Header.Set("Content-Length", strconv.Itoa(len(requestBody)))

	// Add request tracing
	requestID := fmt.Sprintf("%v", request.ID)
	httpReq.Header.Set("X-Request-ID", requestID)

	// Send request with timeout context
	requestStart := time.Now()
	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return c.wrapHTTPError(err, url, time.Since(requestStart))
	}
	defer func() {
		if closeErr := httpResp.Body.Close(); closeErr != nil {
			c.logger.Printf("Failed to close response body: %v", closeErr)
		}
	}()

	// Check status code with detailed error handling
	if httpResp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(httpResp.Body)
		if readErr != nil {
			body = []byte(fmt.Sprintf("failed to read error response: %v", readErr))
		}

		errorMsg := string(body)
		if len(errorMsg) > 500 {
			errorMsg = errorMsg[:500] + "..."
		}

		return c.createHTTPStatusError(httpResp.StatusCode, errorMsg, url)
	}

	// Validate Content-Type
	contentType := httpResp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		return fmt.Errorf("unexpected content type: %s, expected application/json", contentType)
	}

	// Parse response with size limit
	limitedReader := io.LimitReader(httpResp.Body, 10*1024*1024) // 10MB limit
	if err := json.NewDecoder(limitedReader).Decode(response); err != nil {
		return fmt.Errorf("failed to decode JSON response: %w", err)
	}

	// Validate JSON-RPC response structure
	if response.JSONRPC != gateway.JSONRPCVersion {
		return fmt.Errorf("invalid JSON-RPC version: %s", response.JSONRPC)
	}

	return nil
}

// shouldRetryError determines if an error category is retryable
func (c *LSPGatewayClient) shouldRetryError(category ErrorCategory) bool {
	retryable, exists := c.retryPolicy.RetryableErrors[category]
	if !exists {
		return false
	}
	return retryable
}

// generateRequestID generates a unique request ID
func generateRequestID() interface{} {
	// Use nanosecond timestamp with random component for uniqueness
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), rand.Intn(1000))
}

// Error handling and categorization functions

func (c *LSPGatewayClient) categorizeError(err error) ErrorCategory {
	if err == nil {
		return ErrorCategoryUnknown
	}

	errorStr := strings.ToLower(err.Error())

	// Network errors
	if strings.Contains(errorStr, "connection refused") ||
		strings.Contains(errorStr, "connection reset") ||
		strings.Contains(errorStr, "network is unreachable") ||
		strings.Contains(errorStr, "no route to host") ||
		strings.Contains(errorStr, "connection timeout") {
		return ErrorCategoryNetwork
	}

	// Timeout errors
	if strings.Contains(errorStr, "timeout") ||
		strings.Contains(errorStr, "deadline exceeded") ||
		err == context.DeadlineExceeded {
		return ErrorCategoryTimeout
	}

	// Context cancellation
	if err == context.Canceled {
		return ErrorCategoryClient
	}

	// HTTP status code based categorization
	if strings.Contains(errorStr, "http error 5") {
		return ErrorCategoryServer
	}
	if strings.Contains(errorStr, "http error 429") ||
		strings.Contains(errorStr, "too many requests") {
		return ErrorCategoryRateLimit
	}
	if strings.Contains(errorStr, "http error 4") {
		return ErrorCategoryClient
	}

	// Protocol errors
	if strings.Contains(errorStr, "failed to decode") ||
		strings.Contains(errorStr, "failed to marshal") ||
		strings.Contains(errorStr, "invalid json") {
		return ErrorCategoryProtocol
	}

	return ErrorCategoryUnknown
}

func (c *LSPGatewayClient) calculateBackoff(attempt int) time.Duration {
	backoff := time.Duration(float64(c.retryPolicy.InitialBackoff) * math.Pow(c.retryPolicy.BackoffFactor, float64(attempt-1)))

	if backoff > c.retryPolicy.MaxBackoff {
		backoff = c.retryPolicy.MaxBackoff
	}

	// Add jitter to prevent thundering herd
	if c.retryPolicy.JitterEnabled {
		jitter := time.Duration(rand.Float64() * float64(backoff) * 0.1) // 10% jitter
		backoff += jitter
	}

	return backoff
}

func (c *LSPGatewayClient) wrapHTTPError(err error, url string, duration time.Duration) error {
	if err == nil {
		return nil
	}

	// Check for specific network errors
	if netErr, ok := err.(net.Error); ok {
		if netErr.Timeout() {
			return fmt.Errorf("HTTP request timeout after %v to %s: %w", duration, url, err)
		}
	}

	// Check for URL errors
	if urlErr, ok := err.(*net.OpError); ok {
		if urlErr.Timeout() {
			return fmt.Errorf("URL timeout after %v to %s: %w", duration, url, err)
		}
		// Check for connection refused
		if opErr, ok := urlErr.Err.(*net.OpError); ok {
			if opErr.Op == "dial" {
				return fmt.Errorf("connection failed to %s: %w", url, err)
			}
		}
		// Check for syscall errors
		if syscallErr, ok := urlErr.Err.(*net.OpError); ok {
			if errno, ok := syscallErr.Err.(syscall.Errno); ok {
				switch errno {
				case syscall.ECONNREFUSED:
					return fmt.Errorf("connection refused to %s: %w", url, err)
				case syscall.EHOSTUNREACH:
					return fmt.Errorf("host unreachable %s: %w", url, err)
				case syscall.ENETUNREACH:
					return fmt.Errorf("network unreachable %s: %w", url, err)
				}
			}
		}
	}

	return fmt.Errorf("HTTP request failed to %s: %w", url, err)
}

func (c *LSPGatewayClient) createHTTPStatusError(statusCode int, body, url string) error {
	switch {
	case statusCode >= 500:
		return fmt.Errorf("server error %d from %s: %s", statusCode, url, body)
	case statusCode == 429:
		return fmt.Errorf("rate limit exceeded %d from %s: %s", statusCode, url, body)
	case statusCode >= 400:
		return fmt.Errorf("client error %d from %s: %s", statusCode, url, body)
	default:
		return fmt.Errorf("HTTP error %d from %s: %s", statusCode, url, body)
	}
}

func (c *LSPGatewayClient) enhanceError(err error, method string) error {
	category := c.categorizeError(err)

	switch category {
	case ErrorCategoryNetwork:
		return fmt.Errorf("network error for LSP method %s: %w (check LSP Gateway connectivity)", method, err)
	case ErrorCategoryTimeout:
		return fmt.Errorf("timeout error for LSP method %s: %w (consider increasing timeout)", method, err)
	case ErrorCategoryServer:
		return fmt.Errorf("server error for LSP method %s: %w (check LSP Gateway status)", method, err)
	case ErrorCategoryRateLimit:
		return fmt.Errorf("rate limit error for LSP method %s: %w (reduce request frequency)", method, err)
	case ErrorCategoryProtocol:
		return fmt.Errorf("protocol error for LSP method %s: %w (check request format)", method, err)
	default:
		return fmt.Errorf("error for LSP method %s: %w", method, err)
	}
}

func (c *LSPGatewayClient) enhanceJSONRPCError(rpcErr *JSONRPCError, method string) error {
	switch rpcErr.Code {
	case -32700:
		return fmt.Errorf("JSON-RPC parse error for method %s: %s", method, rpcErr.Message)
	case -32600:
		return fmt.Errorf("JSON-RPC invalid request for method %s: %s", method, rpcErr.Message)
	case -32601:
		return fmt.Errorf("JSON-RPC method not found %s: %s (LSP server may not support this method)", method, rpcErr.Message)
	case -32602:
		return fmt.Errorf("JSON-RPC invalid parameters for method %s: %s", method, rpcErr.Message)
	case -32603:
		return fmt.Errorf("JSON-RPC internal error for method %s: %s", method, rpcErr.Message)
	default:
		if rpcErr.Code <= -32000 && rpcErr.Code >= -32099 {
			return fmt.Errorf("JSON-RPC server error %d for method %s: %s", rpcErr.Code, method, rpcErr.Message)
		}
		return fmt.Errorf("JSON-RPC error %d for method %s: %s", rpcErr.Code, method, rpcErr.Message)
	}
}

// Circuit Breaker implementation

func (cb *CircuitBreaker) AllowRequest() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()

	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		if now.Sub(cb.lastFailureTime) > cb.timeout {
			cb.state = CircuitHalfOpen
			cb.successCount = 0
			return true
		}
		return false
	case CircuitHalfOpen:
		return cb.successCount < cb.maxRequests
	default:
		return false
	}
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureCount = 0

	if cb.state == CircuitHalfOpen {
		cb.successCount++
		if cb.successCount >= cb.maxRequests {
			cb.state = CircuitClosed
		}
	}
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureCount++
	cb.lastFailureTime = time.Now()

	if cb.failureCount >= cb.maxFailures {
		cb.state = CircuitOpen
	}
}

// Metrics and monitoring functions

func (c *LSPGatewayClient) updateSuccessMetrics() {
	c.metrics.mu.Lock()
	defer c.metrics.mu.Unlock()

	c.metrics.successfulReqs++
	c.metrics.lastSuccessTime = time.Now()
}

func (c *LSPGatewayClient) updateFailureMetrics() {
	c.metrics.mu.Lock()
	defer c.metrics.mu.Unlock()

	c.metrics.failedRequests++
}

func (c *LSPGatewayClient) updateLatencyMetrics(latency time.Duration) {
	c.metrics.mu.Lock()
	defer c.metrics.mu.Unlock()

	// Simple moving average for latency
	if c.metrics.averageLatency == 0 {
		c.metrics.averageLatency = latency
	} else {
		c.metrics.averageLatency = time.Duration((int64(c.metrics.averageLatency)*9 + int64(latency)) / 10)
	}
}

// GetMetrics returns current connection metrics
func (c *LSPGatewayClient) GetMetrics() ConnectionMetrics {
	c.metrics.mu.RLock()
	defer c.metrics.mu.RUnlock()

	// Create a copy without the mutex to avoid copy lock value issue
	return ConnectionMetrics{
		totalRequests:    c.metrics.totalRequests,
		successfulReqs:   c.metrics.successfulReqs,
		failedRequests:   c.metrics.failedRequests,
		timeoutCount:     c.metrics.timeoutCount,
		connectionErrors: c.metrics.connectionErrors,
		averageLatency:   c.metrics.averageLatency,
		lastRequestTime:  c.metrics.lastRequestTime,
		lastSuccessTime:  c.metrics.lastSuccessTime,
	}
}

// GetHealth returns connection health status
func (c *LSPGatewayClient) GetHealth(ctx context.Context) error {
	c.healthCheckMu.Lock()
	defer c.healthCheckMu.Unlock()

	// Rate limit health checks
	if time.Since(c.lastHealthCheck) < 30*time.Second {
		return nil
	}

	healthCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(healthCtx, "GET", c.heathCheckURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	c.lastHealthCheck = time.Now()
	return nil
}

// IsHealthy returns true if the client is healthy
func (c *LSPGatewayClient) IsHealthy() bool {
	c.circuitBreaker.mu.RLock()
	defer c.circuitBreaker.mu.RUnlock()

	return c.circuitBreaker.state != CircuitOpen
}
