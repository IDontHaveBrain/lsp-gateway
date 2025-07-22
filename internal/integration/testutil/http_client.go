package testutil

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// JSONRPCRequest represents a JSON-RPC request
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

// JSONRPCResponse represents a JSON-RPC response
type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

// HTTPClientPool manages a pool of HTTP clients for concurrent testing
type HTTPClientPool struct {
	baseURL     string
	poolSize    int
	clients     []*http.Client
	nextClient  int64
	stats       *HTTPStats
	timeout     time.Duration
	retryConfig *RetryConfig
}

// HTTPStats tracks HTTP request statistics
type HTTPStats struct {
	TotalRequests   int64
	SuccessRequests int64
	FailedRequests  int64
	TotalLatency    int64 // nanoseconds
	MinLatency      int64
	MaxLatency      int64
	StatusCodes     map[int]int64
	mu              sync.RWMutex
}

// RetryConfig defines retry behavior for failed requests
type RetryConfig struct {
	MaxRetries      int
	InitialBackoff  time.Duration
	MaxBackoff      time.Duration
	BackoffFactor   float64
	RetryableStatus []int
}

// HTTPClientPoolConfig configures the HTTP client pool
type HTTPClientPoolConfig struct {
	BaseURL     string
	PoolSize    int
	Timeout     time.Duration
	RetryConfig *RetryConfig
}

// NewHTTPClientPool creates a new HTTP client pool
func NewHTTPClientPool(config *HTTPClientPoolConfig) *HTTPClientPool {
	if config.PoolSize <= 0 {
		config.PoolSize = 10
	}
	if config.Timeout <= 0 {
		config.Timeout = 30 * time.Second
	}

	pool := &HTTPClientPool{
		baseURL:     config.BaseURL,
		poolSize:    config.PoolSize,
		clients:     make([]*http.Client, config.PoolSize),
		timeout:     config.Timeout,
		retryConfig: config.RetryConfig,
		stats: &HTTPStats{
			StatusCodes: make(map[int]int64),
			MinLatency:  int64(time.Hour), // Start with high value
		},
	}

	// Initialize HTTP clients
	for i := 0; i < config.PoolSize; i++ {
		pool.clients[i] = &http.Client{
			Timeout: config.Timeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		}
	}

	return pool
}

// getClient returns the next available client using round-robin
func (pool *HTTPClientPool) getClient() *http.Client {
	index := atomic.AddInt64(&pool.nextClient, 1) % int64(pool.poolSize)
	return pool.clients[index]
}

// SendJSONRPCRequest sends a JSON-RPC request and returns the response
func (pool *HTTPClientPool) SendJSONRPCRequest(ctx context.Context, request interface{}) (*JSONRPCResponse, error) {
	return pool.SendJSONRPCRequestWithStats(ctx, request, true)
}

// SendJSONRPCRequestWithStats sends a request and optionally records statistics
func (pool *HTTPClientPool) SendJSONRPCRequestWithStats(ctx context.Context, request interface{}, recordStats bool) (*JSONRPCResponse, error) {
	startTime := time.Now()

	requestBody, err := json.Marshal(request)
	if err != nil {
		if recordStats {
			pool.recordFailure()
		}
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	var response *JSONRPCResponse
	var lastErr error

	// Implement retry logic if configured
	maxAttempts := 1
	if pool.retryConfig != nil {
		maxAttempts = pool.retryConfig.MaxRetries + 1
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			// Apply backoff
			backoff := pool.calculateBackoff(attempt)
			select {
			case <-ctx.Done():
				if recordStats {
					pool.recordFailure()
				}
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		response, lastErr = pool.doRequest(ctx, requestBody)
		if lastErr == nil {
			break
		}

		// Check if error is retryable
		if pool.retryConfig != nil && !pool.isRetryableError(lastErr) {
			break
		}
	}

	if recordStats {
		latency := time.Since(startTime)
		if lastErr != nil {
			pool.recordFailure()
		} else {
			pool.recordSuccess(latency, 200) // Assume 200 for successful JSON-RPC
		}
	}

	return response, lastErr
}

// doRequest performs a single HTTP request
func (pool *HTTPClientPool) doRequest(ctx context.Context, requestBody []byte) (*JSONRPCResponse, error) {
	client := pool.getClient()

	req, err := http.NewRequestWithContext(ctx, "POST", pool.baseURL+"/jsonrpc", bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "lsp-gateway-integration-test")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// Log error but don't fail the request
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request failed with status: %d", resp.StatusCode)
	}

	var response JSONRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

// calculateBackoff calculates the backoff duration for retry attempts
func (pool *HTTPClientPool) calculateBackoff(attempt int) time.Duration {
	if pool.retryConfig == nil {
		return time.Second
	}

	backoff := pool.retryConfig.InitialBackoff
	for i := 1; i < attempt; i++ {
		backoff = time.Duration(float64(backoff) * pool.retryConfig.BackoffFactor)
		if backoff > pool.retryConfig.MaxBackoff {
			backoff = pool.retryConfig.MaxBackoff
			break
		}
	}

	return backoff
}

// isRetryableError determines if an error should trigger a retry
func (pool *HTTPClientPool) isRetryableError(err error) bool {
	if pool.retryConfig == nil {
		return false
	}

	// For now, only retry on specific conditions
	// This could be enhanced to check for specific error types
	return false
}

// recordSuccess records a successful request
func (pool *HTTPClientPool) recordSuccess(latency time.Duration, statusCode int) {
	pool.stats.mu.Lock()
	defer pool.stats.mu.Unlock()

	atomic.AddInt64(&pool.stats.TotalRequests, 1)
	atomic.AddInt64(&pool.stats.SuccessRequests, 1)

	latencyNs := latency.Nanoseconds()
	atomic.AddInt64(&pool.stats.TotalLatency, latencyNs)

	// Update min/max latency
	for {
		currentMin := atomic.LoadInt64(&pool.stats.MinLatency)
		if latencyNs >= currentMin || atomic.CompareAndSwapInt64(&pool.stats.MinLatency, currentMin, latencyNs) {
			break
		}
	}

	for {
		currentMax := atomic.LoadInt64(&pool.stats.MaxLatency)
		if latencyNs <= currentMax || atomic.CompareAndSwapInt64(&pool.stats.MaxLatency, currentMax, latencyNs) {
			break
		}
	}

	pool.stats.StatusCodes[statusCode]++
}

// recordFailure records a failed request
func (pool *HTTPClientPool) recordFailure() {
	atomic.AddInt64(&pool.stats.TotalRequests, 1)
	atomic.AddInt64(&pool.stats.FailedRequests, 1)
}

// GetStats returns a copy of the current statistics
func (pool *HTTPClientPool) GetStats() HTTPStatsSnapshot {
	pool.stats.mu.RLock()
	defer pool.stats.mu.RUnlock()

	totalReq := atomic.LoadInt64(&pool.stats.TotalRequests)
	successReq := atomic.LoadInt64(&pool.stats.SuccessRequests)
	failedReq := atomic.LoadInt64(&pool.stats.FailedRequests)
	totalLatency := atomic.LoadInt64(&pool.stats.TotalLatency)
	minLatency := atomic.LoadInt64(&pool.stats.MinLatency)
	maxLatency := atomic.LoadInt64(&pool.stats.MaxLatency)

	statusCodes := make(map[int]int64)
	for code, count := range pool.stats.StatusCodes {
		statusCodes[code] = count
	}

	var avgLatency time.Duration
	if successReq > 0 {
		avgLatency = time.Duration(totalLatency / successReq)
	}

	return HTTPStatsSnapshot{
		TotalRequests:   totalReq,
		SuccessRequests: successReq,
		FailedRequests:  failedReq,
		AverageLatency:  avgLatency,
		MinLatency:      time.Duration(minLatency),
		MaxLatency:      time.Duration(maxLatency),
		StatusCodes:     statusCodes,
		SuccessRate:     float64(successReq) / float64(totalReq),
	}
}

// HTTPStatsSnapshot provides a point-in-time view of HTTP statistics
type HTTPStatsSnapshot struct {
	TotalRequests   int64
	SuccessRequests int64
	FailedRequests  int64
	AverageLatency  time.Duration
	MinLatency      time.Duration
	MaxLatency      time.Duration
	StatusCodes     map[int]int64
	SuccessRate     float64
}

// Reset clears all statistics
func (pool *HTTPClientPool) Reset() {
	pool.stats.mu.Lock()
	defer pool.stats.mu.Unlock()

	atomic.StoreInt64(&pool.stats.TotalRequests, 0)
	atomic.StoreInt64(&pool.stats.SuccessRequests, 0)
	atomic.StoreInt64(&pool.stats.FailedRequests, 0)
	atomic.StoreInt64(&pool.stats.TotalLatency, 0)
	atomic.StoreInt64(&pool.stats.MinLatency, int64(time.Hour))
	atomic.StoreInt64(&pool.stats.MaxLatency, 0)

	pool.stats.StatusCodes = make(map[int]int64)
}

// ConcurrentRequestExecutor executes multiple requests concurrently
type ConcurrentRequestExecutor struct {
	pool         *HTTPClientPool
	maxWorkers   int
	requestQueue chan ConcurrentRequest
	results      chan ConcurrentResult
	wg           sync.WaitGroup
}

// ConcurrentRequest represents a request to be executed concurrently
type ConcurrentRequest struct {
	ID      string
	Request interface{}
	Context context.Context
}

// ConcurrentResult represents the result of a concurrent request
type ConcurrentResult struct {
	ID       string
	Response *JSONRPCResponse
	Error    error
	Latency  time.Duration
}

// NewConcurrentRequestExecutor creates a new concurrent request executor
func NewConcurrentRequestExecutor(pool *HTTPClientPool, maxWorkers int) *ConcurrentRequestExecutor {
	return &ConcurrentRequestExecutor{
		pool:         pool,
		maxWorkers:   maxWorkers,
		requestQueue: make(chan ConcurrentRequest, maxWorkers*2),
		results:      make(chan ConcurrentResult, maxWorkers*2),
	}
}

// Start begins the concurrent execution workers
func (exe *ConcurrentRequestExecutor) Start() {
	for i := 0; i < exe.maxWorkers; i++ {
		exe.wg.Add(1)
		go exe.worker()
	}
}

// Stop stops all workers and waits for completion
func (exe *ConcurrentRequestExecutor) Stop() {
	close(exe.requestQueue)
	exe.wg.Wait()
	close(exe.results)
}

// Submit submits a request for concurrent execution
func (exe *ConcurrentRequestExecutor) Submit(id string, request interface{}, ctx context.Context) {
	select {
	case exe.requestQueue <- ConcurrentRequest{ID: id, Request: request, Context: ctx}:
	case <-ctx.Done():
		// Request cancelled before submission
	}
}

// Results returns the channel for reading results
func (exe *ConcurrentRequestExecutor) Results() <-chan ConcurrentResult {
	return exe.results
}

// worker processes requests from the queue
func (exe *ConcurrentRequestExecutor) worker() {
	defer exe.wg.Done()

	for req := range exe.requestQueue {
		startTime := time.Now()
		response, err := exe.pool.SendJSONRPCRequestWithStats(req.Context, req.Request, false)
		latency := time.Since(startTime)

		result := ConcurrentResult{
			ID:       req.ID,
			Response: response,
			Error:    err,
			Latency:  latency,
		}

		select {
		case exe.results <- result:
		case <-req.Context.Done():
			// Context cancelled, skip result
		}
	}
}

// BatchRequestConfig configures batch request execution
type BatchRequestConfig struct {
	Requests       []interface{}
	ConcurrentReqs int
	Timeout        time.Duration
	CollectStats   bool
}

// ExecuteBatch executes a batch of requests with specified concurrency
func (pool *HTTPClientPool) ExecuteBatch(ctx context.Context, config *BatchRequestConfig) ([]ConcurrentResult, error) {
	if len(config.Requests) == 0 {
		return nil, nil
	}

	maxWorkers := config.ConcurrentReqs
	if maxWorkers <= 0 {
		maxWorkers = 10
	}

	executor := NewConcurrentRequestExecutor(pool, maxWorkers)
	executor.Start()
	defer executor.Stop()

	// Submit all requests
	for i, req := range config.Requests {
		reqCtx := ctx
		if config.Timeout > 0 {
			var cancel context.CancelFunc
			reqCtx, cancel = context.WithTimeout(ctx, config.Timeout)
			defer cancel()
		}

		executor.Submit(fmt.Sprintf("req-%d", i), req, reqCtx)
	}

	// Collect results
	results := make([]ConcurrentResult, 0, len(config.Requests))
	for i := 0; i < len(config.Requests); i++ {
		select {
		case result := <-executor.Results():
			results = append(results, result)

			// Record stats if enabled
			if config.CollectStats {
				if result.Error != nil {
					pool.recordFailure()
				} else {
					pool.recordSuccess(result.Latency, 200)
				}
			}
		case <-ctx.Done():
			return results, ctx.Err()
		}
	}

	return results, nil
}

// HealthCheck performs a simple health check against the service
func (pool *HTTPClientPool) HealthCheck(ctx context.Context) error {
	healthRequest := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "health-check",
		Method:  "textDocument/definition", // Use a simple method for health check
		Params: map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": "file:///health-check.go",
			},
			"position": map[string]interface{}{
				"line": 0, "character": 0,
			},
		},
	}

	_, err := pool.SendJSONRPCRequestWithStats(ctx, healthRequest, false)
	return err
}

// String returns a string representation of the pool statistics
func (snapshot HTTPStatsSnapshot) String() string {
	return fmt.Sprintf("HTTP Stats: Total=%d, Success=%d, Failed=%d, Success Rate=%.2f%%, Avg Latency=%v, Min=%v, Max=%v",
		snapshot.TotalRequests,
		snapshot.SuccessRequests,
		snapshot.FailedRequests,
		snapshot.SuccessRate*100,
		snapshot.AverageLatency,
		snapshot.MinLatency,
		snapshot.MaxLatency)
}
