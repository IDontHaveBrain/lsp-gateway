package mcp

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"lsp-gateway/internal/transport"
)

const JSONRPCVersion = "2.0"

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

type RetryPolicy struct {
	MaxRetries      int
	InitialBackoff  time.Duration
	MaxBackoff      time.Duration
	BackoffFactor   float64
	JitterEnabled   bool
	RetryableErrors map[ErrorCategory]bool
}

type CircuitBreakerState int

const (
	CircuitClosed CircuitBreakerState = iota
	CircuitOpen
	CircuitHalfOpen
)

type CircuitBreaker struct {
	mu              sync.RWMutex
	State           CircuitBreakerState
	failureCount    int
	lastFailureTime time.Time
	successCount    int
	Timeout         time.Duration
	MaxFailures     int
	MaxRequests     int
}

type ConnectionMetrics struct {
	mu               sync.RWMutex
	TotalRequests    int64
	SuccessfulReqs   int64
	FailedRequests   int64
	TimeoutCount     int64
	ConnectionErrors int64
	AverageLatency   time.Duration
	LastRequestTime  time.Time
	LastSuccessTime  time.Time
}

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

type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func NewLSPGatewayClient(config *ServerConfig) *LSPGatewayClient {
	logger := log.New(log.Writer(), "[LSPClient] ", log.LstdFlags|log.Lshortfile)

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

	circuitBreaker := &CircuitBreaker{
		State:       CircuitClosed,
		Timeout:     60 * time.Second,
		MaxFailures: 5,
		MaxRequests: 3,
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

func (c *LSPGatewayClient) SendLSPRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	start := time.Now()

	c.metrics.mu.Lock()
	c.metrics.TotalRequests++
	c.metrics.LastRequestTime = start
	c.metrics.mu.Unlock()

	if !c.circuitBreaker.AllowRequest() {
		c.logger.Printf("Circuit breaker is open, rejecting request for method: %s", method)
		c.updateFailureMetrics()
		return nil, fmt.Errorf("circuit breaker is open: too many failures")
	}

	request := JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      GenerateRequestID(),
		Method:  method,
		Params:  params,
	}

	c.logger.Printf("Sending LSP request: method=%s, id=%v", method, request.ID)

	var response JSONRPCResponse
	err := c.sendRequestWithRetry(ctx, request, &response)

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

func (c *LSPGatewayClient) sendRequestWithRetry(ctx context.Context, request JSONRPCRequest, response *JSONRPCResponse) error {
	var lastErr error

	for attempt := 0; attempt <= c.retryPolicy.MaxRetries; attempt++ {
		if attempt > 0 {
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

		if !c.shouldRetryError(errorCategory) {
			c.logger.Printf("Not retrying due to error category: %v", errorCategory)
			break
		}

		if attempt >= c.retryPolicy.MaxRetries {
			break
		}
	}

	c.logger.Printf("Request failed after %d attempts: method=%s, final_error=%v", c.retryPolicy.MaxRetries+1, request.Method, lastErr)
	return fmt.Errorf("request failed after %d attempts: %w", c.retryPolicy.MaxRetries+1, lastErr)
}

func (c *LSPGatewayClient) sendSingleRequest(ctx context.Context, request JSONRPCRequest, response *JSONRPCResponse) error {
	requestBody, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := c.baseURL + "/jsonrpc"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(requestBody))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set(HTTP_HEADER_CONTENT_TYPE, transport.HTTP_CONTENT_TYPE_JSON)
	httpReq.Header.Set("Accept", transport.HTTP_CONTENT_TYPE_JSON)
	httpReq.Header.Set("User-Agent", "LSP-Gateway-MCP-Client/1.0")
	httpReq.Header.Set("Content-Length", strconv.Itoa(len(requestBody)))

	requestID := fmt.Sprintf("%v", request.ID)
	httpReq.Header.Set("X-Request-ID", requestID)

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

	contentType := httpResp.Header.Get(HTTP_HEADER_CONTENT_TYPE)
	if !strings.Contains(contentType, transport.HTTP_CONTENT_TYPE_JSON) {
		return fmt.Errorf("unexpected content type: %s, expected %s", contentType, transport.HTTP_CONTENT_TYPE_JSON)
	}

	limitedReader := io.LimitReader(httpResp.Body, 10*1024*1024) // 10MB limit
	if err := json.NewDecoder(limitedReader).Decode(response); err != nil {
		return fmt.Errorf("failed to decode JSON response: %w", err)
	}

	if response.JSONRPC != JSONRPCVersion {
		return fmt.Errorf("invalid JSON-RPC version: %s", response.JSONRPC)
	}

	return nil
}

func (c *LSPGatewayClient) shouldRetryError(category ErrorCategory) bool {
	retryable, exists := c.retryPolicy.RetryableErrors[category]
	if !exists {
		return false
	}
	return retryable
}

func GenerateRequestID() interface{} {
	// Use crypto/rand for security-sensitive request ID generation
	randomBytes := make([]byte, 4)
	_, err := rand.Read(randomBytes)
	if err != nil {
		// Fallback to timestamp-only on crypto/rand failure
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}

	// Convert bytes to uint32 for better distribution
	randomValue := binary.BigEndian.Uint32(randomBytes)
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), randomValue%10000)
}

func (c *LSPGatewayClient) categorizeError(err error) ErrorCategory {
	if err == nil {
		return ErrorCategoryUnknown
	}

	errorStr := strings.ToLower(err.Error())

	if strings.Contains(errorStr, "connection refused") ||
		strings.Contains(errorStr, "connection reset") ||
		strings.Contains(errorStr, "network is unreachable") ||
		strings.Contains(errorStr, "no route to host") ||
		strings.Contains(errorStr, "connection timeout") ||
		strings.Contains(errorStr, "eof") ||
		strings.Contains(errorStr, "connection lost") ||
		strings.Contains(errorStr, "dial tcp") ||
		strings.Contains(errorStr, "invalid port") {
		return ErrorCategoryNetwork
	}

	if strings.Contains(errorStr, "timeout") ||
		strings.Contains(errorStr, "deadline exceeded") ||
		err == context.DeadlineExceeded {
		return ErrorCategoryTimeout
	}

	if err == context.Canceled {
		return ErrorCategoryClient
	}

	if strings.Contains(errorStr, "http error 5") ||
		strings.Contains(errorStr, "server error 5") {
		return ErrorCategoryServer
	}
	if strings.Contains(errorStr, "http error 429") ||
		strings.Contains(errorStr, "too many requests") {
		return ErrorCategoryRateLimit
	}
	if strings.Contains(errorStr, "http error 4") {
		return ErrorCategoryClient
	}

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

	if c.retryPolicy.JitterEnabled {
		// Use crypto/rand for security-sensitive jitter calculation
		randomBytes := make([]byte, 8)
		_, err := rand.Read(randomBytes)
		if err != nil {
			// Fallback to minimal jitter on crypto/rand failure
			jitter := time.Duration(float64(backoff) * 0.05) // 5% fixed jitter
			backoff += jitter
		} else {
			// Convert bytes to float64 in range [0.0, 1.0)
			randomValue := binary.BigEndian.Uint64(randomBytes)
			randomFloat := float64(randomValue) / float64(^uint64(0))
			jitter := time.Duration(randomFloat * float64(backoff) * 0.1) // 10% jitter
			backoff += jitter
		}
	}

	return backoff
}

func (c *LSPGatewayClient) wrapHTTPError(err error, url string, duration time.Duration) error {
	if err == nil {
		return nil
	}

	if netErr, ok := err.(net.Error); ok {
		if netErr.Timeout() {
			return fmt.Errorf("HTTP request timeout after %v to %s: %w", duration, url, err)
		}
	}

	if urlErr, ok := err.(*net.OpError); ok {
		if urlErr.Timeout() {
			return fmt.Errorf("URL timeout after %v to %s: %w", duration, url, err)
		}
		if opErr, ok := urlErr.Err.(*net.OpError); ok {
			if opErr.Op == "dial" {
				return fmt.Errorf("connection failed to %s: %w", url, err)
			}
		}
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

func (cb *CircuitBreaker) AllowRequest() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()

	switch cb.State {
	case CircuitClosed:
		return true
	case CircuitOpen:
		if now.Sub(cb.lastFailureTime) > cb.Timeout {
			cb.State = CircuitHalfOpen
			cb.successCount = 0
			return true
		}
		return false
	case CircuitHalfOpen:
		return cb.successCount < cb.MaxRequests
	default:
		return false
	}
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureCount = 0

	if cb.State == CircuitHalfOpen {
		cb.successCount++
		if cb.successCount >= cb.MaxRequests {
			cb.State = CircuitClosed
		}
	}
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureCount++
	cb.lastFailureTime = time.Now()

	if cb.failureCount >= cb.MaxFailures {
		cb.State = CircuitOpen
	}
}

func (c *LSPGatewayClient) updateSuccessMetrics() {
	c.metrics.mu.Lock()
	defer c.metrics.mu.Unlock()

	c.metrics.SuccessfulReqs++
	c.metrics.LastSuccessTime = time.Now()
}

func (c *LSPGatewayClient) updateFailureMetrics() {
	c.metrics.mu.Lock()
	defer c.metrics.mu.Unlock()

	c.metrics.FailedRequests++
}

func (c *LSPGatewayClient) updateLatencyMetrics(latency time.Duration) {
	c.metrics.mu.Lock()
	defer c.metrics.mu.Unlock()

	if c.metrics.AverageLatency == 0 {
		c.metrics.AverageLatency = latency
	} else {
		c.metrics.AverageLatency = time.Duration((int64(c.metrics.AverageLatency)*9 + int64(latency)) / 10)
	}
}

func (c *LSPGatewayClient) GetMetrics() ConnectionMetrics {
	c.metrics.mu.RLock()
	defer c.metrics.mu.RUnlock()

	return ConnectionMetrics{
		TotalRequests:    c.metrics.TotalRequests,
		SuccessfulReqs:   c.metrics.SuccessfulReqs,
		FailedRequests:   c.metrics.FailedRequests,
		TimeoutCount:     c.metrics.TimeoutCount,
		ConnectionErrors: c.metrics.ConnectionErrors,
		AverageLatency:   c.metrics.AverageLatency,
		LastRequestTime:  c.metrics.LastRequestTime,
		LastSuccessTime:  c.metrics.LastSuccessTime,
	}
}

func (c *LSPGatewayClient) GetHealth(ctx context.Context) error {
	c.healthCheckMu.Lock()
	defer c.healthCheckMu.Unlock()

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

func (c *LSPGatewayClient) IsHealthy() bool {
	c.circuitBreaker.mu.RLock()
	defer c.circuitBreaker.mu.RUnlock()

	return c.circuitBreaker.State != CircuitOpen
}

// SetRetryPolicy updates the client's retry policy
func (c *LSPGatewayClient) SetRetryPolicy(policy *RetryPolicy) {
	c.retryPolicy = policy
}

// GetBaseURL returns the base URL of the LSP Gateway
func (c *LSPGatewayClient) GetBaseURL() string {
	return c.baseURL
}

// GetTimeout returns the client timeout
func (c *LSPGatewayClient) GetTimeout() time.Duration {
	return c.timeout
}

// GetMaxRetries returns the maximum number of retries
func (c *LSPGatewayClient) GetMaxRetries() int {
	return c.maxRetries
}

// GetCircuitBreakerState returns the current circuit breaker state
func (c *LSPGatewayClient) GetCircuitBreakerState() CircuitBreakerState {
	c.circuitBreaker.mu.RLock()
	defer c.circuitBreaker.mu.RUnlock()
	return c.circuitBreaker.State
}

// SetCircuitBreakerConfig allows setting circuit breaker parameters for testing
func (c *LSPGatewayClient) SetCircuitBreakerConfig(maxFailures int, timeout time.Duration) {
	c.circuitBreaker.mu.Lock()
	defer c.circuitBreaker.mu.Unlock()
	c.circuitBreaker.MaxFailures = maxFailures
	c.circuitBreaker.Timeout = timeout
}

// GetRetryPolicy returns a copy of the retry policy
func (c *LSPGatewayClient) GetRetryPolicy() RetryPolicy {
	if c.retryPolicy == nil {
		return RetryPolicy{}
	}
	return *c.retryPolicy
}

// GetCircuitBreaker returns the circuit breaker for testing purposes
func (c *LSPGatewayClient) GetCircuitBreaker() *CircuitBreaker {
	return c.circuitBreaker
}

// GetMetricsPointer returns a pointer to the metrics for testing
func (c *LSPGatewayClient) GetMetricsPointer() *ConnectionMetrics {
	return c.metrics
}

// CategorizeError exposes error categorization for testing
func (c *LSPGatewayClient) CategorizeError(err error) ErrorCategory {
	return c.categorizeError(err)
}

// CalculateBackoff exposes backoff calculation for testing
func (c *LSPGatewayClient) CalculateBackoff(attempt int) time.Duration {
	return c.calculateBackoff(attempt)
}

// GetHTTPClient returns the HTTP client for testing
func (c *LSPGatewayClient) GetHTTPClient() *http.Client {
	return c.httpClient
}
