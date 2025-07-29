package testutils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"lsp-gateway/mcp"
)

// Position represents an LSP position
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range represents an LSP range
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Location represents an LSP location
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// SymbolInformation represents an LSP symbol
type SymbolInformation struct {
	Name     string   `json:"name"`
	Kind     int      `json:"kind"`
	Location Location `json:"location"`
}

// DocumentSymbol represents an LSP document symbol
type DocumentSymbol struct {
	Name           string           `json:"name"`
	Kind           int              `json:"kind"`
	Range          Range            `json:"range"`
	SelectionRange Range            `json:"selectionRange"`
	Children       []DocumentSymbol `json:"children,omitempty"`
}

// HoverResult represents an LSP hover response
type HoverResult struct {
	Contents interface{} `json:"contents"`
	Range    *Range      `json:"range,omitempty"`
}

// CompletionItem represents an LSP completion item
type CompletionItem struct {
	Label         string      `json:"label"`
	Kind          int         `json:"kind"`
	Detail        string      `json:"detail,omitempty"`
	Documentation interface{} `json:"documentation,omitempty"`
}

// CompletionList represents an LSP completion response
type CompletionList struct {
	IsIncomplete bool             `json:"isIncomplete"`
	Items        []CompletionItem `json:"items"`
}

// SubProjectMetrics tracks performance metrics for a specific sub-project
type SubProjectMetrics struct {
	Language           string        `json:"language"`
	TotalRequests      int           `json:"total_requests"`
	SuccessfulRequests int           `json:"successful_requests"`
	FailedRequests     int           `json:"failed_requests"`
	AverageLatency     time.Duration `json:"average_latency"`
	RoutingErrors      int           `json:"routing_errors"`
	TotalLatency       time.Duration `json:"total_latency"`
	mu                 sync.RWMutex
}

// RequestMetrics tracks performance metrics for requests
type RequestMetrics struct {
	TotalRequests     int           `json:"total_requests"`
	SuccessfulReqs    int           `json:"successful_requests"`
	FailedRequests    int           `json:"failed_requests"`
	AverageLatency    time.Duration `json:"average_latency"`
	MinLatency        time.Duration `json:"min_latency"`
	MaxLatency        time.Duration `json:"max_latency"`
	TotalLatency      time.Duration `json:"total_latency"`
	ResponseSizes     []int         `json:"response_sizes"`
	ConnectionErrors  int           `json:"connection_errors"`
	TimeoutErrors     int           `json:"timeout_errors"`
	SubProjectMetrics map[string]*SubProjectMetrics `json:"sub_project_metrics"`
	CrossProjectRequests int       `json:"cross_project_requests"`
	mu                sync.RWMutex
}

// RecordedRequest stores a request for validation
type RecordedRequest struct {
	Method    string                 `json:"method"`
	URL       string                 `json:"url"`
	Headers   map[string]string      `json:"headers"`
	Body      interface{}            `json:"body"`
	Timestamp time.Time              `json:"timestamp"`
	Response  interface{}            `json:"response,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Duration  time.Duration          `json:"duration"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// HttpClientConfig configures the HttpClient
type HttpClientConfig struct {
	BaseURL            string
	Timeout            time.Duration
	MaxRetries         int
	RetryDelay         time.Duration
	EnableLogging      bool
	EnableRecording    bool
	MockMode           bool
	WorkspaceID        string
	ProjectPath        string
	UserAgent          string
	MaxResponseSize    int64
	ConnectionPoolSize int
	KeepAlive          time.Duration
	WorkspaceRoot      string
	SubProjects        map[string]*SubProjectInfo
	EnableSubProjectMetrics bool
}

// DefaultHttpClientConfig returns a default configuration optimized for test environments
func DefaultHttpClientConfig() HttpClientConfig {
	return HttpClientConfig{
		BaseURL:            "http://localhost:8080",
		Timeout:            10 * time.Second, // Increased for test stability
		MaxRetries:         3,
		RetryDelay:         500 * time.Millisecond, // Faster retries for tests
		EnableLogging:      true,
		EnableRecording:    false,
		MockMode:           false,
		WorkspaceID:        "test-workspace",
		ProjectPath:        "/tmp/test-project",
		UserAgent:          "LSP-Gateway-E2E-Test/1.0",
		MaxResponseSize:    10 * 1024 * 1024, // 10MB
		ConnectionPoolSize: 20, // Increased for concurrent tests
		KeepAlive:          30 * time.Second, // Longer keep-alive for test efficiency
	}
}

// HttpClient provides comprehensive HTTP client for testing real server connections
type HttpClient struct {
	config       HttpClientConfig
	client       *http.Client
	metrics      *RequestMetrics
	recordings   []RecordedRequest
	mockResponses map[string]interface{}
	workspaceRoot string
	mu           sync.RWMutex
}

// NewHttpClient creates a new HttpClient with the given configuration
func NewHttpClient(config HttpClientConfig) *HttpClient {
	// Set defaults if not provided
	if config.BaseURL == "" {
		config.BaseURL = "http://localhost:8080"
	}
	if config.Timeout == 0 {
		config.Timeout = 5 * time.Second
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = time.Second
	}
	if config.UserAgent == "" {
		config.UserAgent = "LSP-Gateway-E2E-Test/1.0"
	}
	if config.MaxResponseSize == 0 {
		config.MaxResponseSize = 10 * 1024 * 1024 // 10MB
	}
	if config.ConnectionPoolSize == 0 {
		config.ConnectionPoolSize = 10
	}
	if config.KeepAlive == 0 {
		config.KeepAlive = 15 * time.Second
	}

	// Create HTTP transport optimized for test environment with proper connection management
	transport := &http.Transport{
		// Connection pool settings
		MaxIdleConns:        config.ConnectionPoolSize * 2,           // Allow more idle connections
		MaxIdleConnsPerHost: config.ConnectionPoolSize,                // Per-host limit
		MaxConnsPerHost:     config.ConnectionPoolSize * 3,           // Total connections per host
		IdleConnTimeout:     config.KeepAlive,                        // How long idle connections stay open
		
		// Timeout settings for connection establishment
		DialContext: (&net.Dialer{
			Timeout:   3 * time.Second,  // Connection establishment timeout
			KeepAlive: config.KeepAlive, // TCP keep-alive
		}).DialContext,
		
		// TLS and response timeouts
		TLSHandshakeTimeout:   5 * time.Second,  // TLS handshake timeout
		ResponseHeaderTimeout: 5 * time.Second,  // Response header timeout
		ExpectContinueTimeout: 1 * time.Second,  // 100-continue timeout
		
		// Connection reuse and keep-alive
		DisableKeepAlives:     false,            // Enable keep-alive
		DisableCompression:    false,            // Enable compression
		ForceAttemptHTTP2:     false,            // Stick to HTTP/1.1 for simplicity
		
		// Important: Enable connection draining for better cleanup
		WriteBufferSize: 4096,  // Write buffer size
		ReadBufferSize:  4096,  // Read buffer size
	}

	httpClient := &http.Client{
		Timeout:   config.Timeout,
		Transport: transport,
	}

	metrics := &RequestMetrics{
		SubProjectMetrics: make(map[string]*SubProjectMetrics),
	}

	return &HttpClient{
		config:        config,
		client:        httpClient,
		metrics:       metrics,
		recordings:    make([]RecordedRequest, 0),
		mockResponses: make(map[string]interface{}),
		workspaceRoot: config.WorkspaceRoot,
	}
}

// HealthCheck performs a health check against the /health endpoint
func (c *HttpClient) HealthCheck(ctx context.Context) error {
	url := fmt.Sprintf("%s/health", c.config.BaseURL)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	c.setStandardHeaders(req)

	resp, err := c.client.Do(req)
	if err != nil {
		c.updateMetrics(0, err, true)
		return fmt.Errorf("health check failed: %w", err)
	}
	defer func() {
		// Ensure connection cleanup with proper draining
		c.drainAndCloseResponse(resp)
	}()

	if resp.StatusCode != http.StatusOK {
		c.updateMetrics(0, fmt.Errorf("health check returned status %d", resp.StatusCode), false)
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	c.updateMetrics(0, nil, false)
	return nil
}

// FastHealthCheck performs a lightweight HEAD-based health check for faster server detection
func (c *HttpClient) FastHealthCheck(ctx context.Context) error {
	url := fmt.Sprintf("%s/health", c.config.BaseURL)
	
	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create fast health check request: %w", err)
	}

	c.setStandardHeaders(req)

	resp, err := c.client.Do(req)
	if err != nil {
		c.updateMetrics(0, err, true)
		return fmt.Errorf("fast health check failed: %w", err)
	}
	defer func() {
		// Ensure connection cleanup with proper draining
		c.drainAndCloseResponse(resp)
	}()

	// Accept 2xx and 3xx status codes for HEAD requests
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		c.updateMetrics(0, nil, false)
		return nil
	}

	c.updateMetrics(0, fmt.Errorf("fast health check returned status %d", resp.StatusCode), false)
	return fmt.Errorf("fast health check returned status %d", resp.StatusCode)
}

// Definition sends a textDocument/definition request
func (c *HttpClient) Definition(ctx context.Context, fileURI string, position Position) ([]Location, error) {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": fileURI,
		},
		"position": position,
	}

	response, err := c.sendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION, params)
	if err != nil {
		return nil, err
	}

	var locations []Location
	if err := json.Unmarshal(response, &locations); err != nil {
		// Try parsing as single location
		var singleLocation Location
		if err2 := json.Unmarshal(response, &singleLocation); err2 != nil {
			return nil, fmt.Errorf("failed to parse definition response: %w", err)
		}
		locations = []Location{singleLocation}
	}

	return locations, nil
}

// References sends a textDocument/references request
func (c *HttpClient) References(ctx context.Context, fileURI string, position Position, includeDeclaration bool) ([]Location, error) {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": fileURI,
		},
		"position": position,
		"context": map[string]interface{}{
			"includeDeclaration": includeDeclaration,
		},
	}

	response, err := c.sendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES, params)
	if err != nil {
		return nil, err
	}

	var locations []Location
	if err := json.Unmarshal(response, &locations); err != nil {
		return nil, fmt.Errorf("failed to parse references response: %w", err)
	}

	return locations, nil
}

// DocumentSymbol sends a textDocument/documentSymbol request
func (c *HttpClient) DocumentSymbol(ctx context.Context, fileURI string) ([]DocumentSymbol, error) {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": fileURI,
		},
	}

	response, err := c.sendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS, params)
	if err != nil {
		return nil, err
	}

	var symbols []DocumentSymbol
	if err := json.Unmarshal(response, &symbols); err != nil {
		// Try parsing as SymbolInformation array (fallback)
		var symbolInfo []SymbolInformation
		if err2 := json.Unmarshal(response, &symbolInfo); err2 != nil {
			return nil, fmt.Errorf("failed to parse document symbols response: %w", err)
		}
		// Convert SymbolInformation to DocumentSymbol
		symbols = make([]DocumentSymbol, len(symbolInfo))
		for i, si := range symbolInfo {
			symbols[i] = DocumentSymbol{
				Name:           si.Name,
				Kind:           si.Kind,
				Range:          si.Location.Range,
				SelectionRange: si.Location.Range,
			}
		}
	}

	return symbols, nil
}

// WorkspaceSymbol sends a workspace/symbol request
func (c *HttpClient) WorkspaceSymbol(ctx context.Context, query string) ([]SymbolInformation, error) {
	params := map[string]interface{}{
		"query": query,
	}

	response, err := c.sendLSPRequest(ctx, mcp.LSP_METHOD_WORKSPACE_SYMBOL, params)
	if err != nil {
		return nil, err
	}

	var symbols []SymbolInformation
	if err := json.Unmarshal(response, &symbols); err != nil {
		return nil, fmt.Errorf("failed to parse workspace symbols response: %w", err)
	}

	return symbols, nil
}

// Hover sends a textDocument/hover request
func (c *HttpClient) Hover(ctx context.Context, fileURI string, position Position) (*HoverResult, error) {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": fileURI,
		},
		"position": position,
	}

	response, err := c.sendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER, params)
	if err != nil {
		return nil, err
	}

	// Handle null response
	if bytes.Equal(response, []byte("null")) {
		return nil, nil
	}

	var hoverResult HoverResult
	if err := json.Unmarshal(response, &hoverResult); err != nil {
		return nil, fmt.Errorf("failed to parse hover response: %w", err)
	}

	return &hoverResult, nil
}

// Completion sends a textDocument/completion request
func (c *HttpClient) Completion(ctx context.Context, fileURI string, position Position) (*CompletionList, error) {
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": fileURI,
		},
		"position": position,
	}

	response, err := c.sendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_COMPLETION, params)
	if err != nil {
		return nil, err
	}

	var completionList CompletionList
	if err := json.Unmarshal(response, &completionList); err != nil {
		// Try parsing as array of CompletionItem
		var items []CompletionItem
		if err2 := json.Unmarshal(response, &items); err2 != nil {
			return nil, fmt.Errorf("failed to parse completion response: %w", err)
		}
		completionList = CompletionList{
			IsIncomplete: false,
			Items:        items,
		}
	}

	return &completionList, nil
}

// sendLSPRequest sends a JSON-RPC request to the LSP gateway
func (c *HttpClient) sendLSPRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	// Check mock mode first
	if c.config.MockMode {
		if mockResponse, exists := c.mockResponses[method]; exists {
			data, _ := json.Marshal(mockResponse)
			return json.RawMessage(data), nil
		}
		return json.RawMessage("[]"), nil
	}

	startTime := time.Now()
	requestID := fmt.Sprintf("req-%d", time.Now().UnixNano())

	// Build JSON-RPC request compliant with JSON-RPC 2.0 specification
	jsonRPCRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      requestID,
		"method":  method,
		"params":  params,
	}

	requestBody, err := json.Marshal(jsonRPCRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/jsonrpc", c.config.BaseURL)
	
	var response json.RawMessage
	var lastErr error

	// Retry logic with exponential backoff and connection pool management
	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff with jitter for connection pool recovery
			backoffDelay := c.config.RetryDelay * time.Duration(attempt)
			
			// Add jitter to prevent thundering herd
			jitter := time.Duration(attempt) * 50 * time.Millisecond
			backoffDelay += jitter
			
			// If connection error, give connection pool time to recover
			if lastErr != nil && c.isConnectionPoolError(lastErr) {
				backoffDelay = backoffDelay * 2 // Double delay for pool errors
			}
			
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoffDelay):
			}
		}

		response, lastErr = c.executeRequest(ctx, url, requestBody, method, startTime)
		if lastErr == nil {
			break
		}

		// Don't retry on context cancellation or timeout
		if ctx.Err() != nil {
			break
		}
		
		// Don't retry on non-retriable errors (e.g., 4xx HTTP status codes)
		if !c.isRetriableError(lastErr) {
			break
		}
	}

	if lastErr != nil {
		return nil, lastErr
	}

	return response, nil
}

// executeRequest executes a single HTTP request
func (c *HttpClient) executeRequest(ctx context.Context, url string, requestBody []byte, method string, startTime time.Time) (json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setStandardHeaders(req)

	// Record request if enabled
	if c.config.EnableRecording {
		c.recordRequest(req, requestBody, method, startTime)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		duration := time.Since(startTime)
		c.updateMetrics(duration, err, true)
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		// Ensure proper connection cleanup with draining in all paths
		c.drainAndCloseResponse(resp)
	}()

	// Check for HTTP errors - drain response body even on HTTP errors to enable connection reuse
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		duration := time.Since(startTime)
		err = fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
		c.updateMetrics(duration, err, false)
		
		// Drain response body on HTTP error to enable connection reuse
		c.drainResponseBody(resp)
		return nil, err
	}

	// Read response with size limit and proper error handling
	limitReader := io.LimitReader(resp.Body, c.config.MaxResponseSize)
	responseBody, err := io.ReadAll(limitReader)
	if err != nil {
		duration := time.Since(startTime)
		c.updateMetrics(duration, err, false)
		
		// Drain remaining response body on read error to enable connection reuse
		c.drainResponseBody(resp)
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	// Drain any remaining bytes if we hit the size limit to enable connection reuse
	if int64(len(responseBody)) == c.config.MaxResponseSize {
		c.drainResponseBody(resp)
	}

	// Update metrics
	duration := time.Since(startTime)
	c.updateMetrics(duration, nil, false)

	// Parse JSON-RPC response
	var jsonRPCResponse struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      string          `json:"id"`
		Result  json.RawMessage `json:"result"`
		Error   *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(responseBody, &jsonRPCResponse); err != nil {
		return nil, fmt.Errorf("failed to parse JSON-RPC response: %w", err)
	}

	if jsonRPCResponse.Error != nil {
		return nil, fmt.Errorf("LSP error %d: %s", jsonRPCResponse.Error.Code, jsonRPCResponse.Error.Message)
	}

	// Record response if enabled
	if c.config.EnableRecording {
		c.recordResponse(method, jsonRPCResponse.Result, duration)
	}

	return jsonRPCResponse.Result, nil
}

// setStandardHeaders sets the standard headers for LSP requests
func (c *HttpClient) setStandardHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.config.UserAgent)
	req.Header.Set("X-Request-ID", fmt.Sprintf("test-%d", time.Now().UnixNano()))
	
	if c.config.WorkspaceID != "" {
		req.Header.Set("X-Workspace-ID", c.config.WorkspaceID)
	}
	if c.config.ProjectPath != "" {
		req.Header.Set("X-Project-Path", c.config.ProjectPath)
	}
}

// updateMetrics updates the performance metrics
func (c *HttpClient) updateMetrics(duration time.Duration, err error, isConnectionError bool) {
	c.metrics.mu.Lock()
	defer c.metrics.mu.Unlock()

	c.metrics.TotalRequests++
	
	if err != nil {
		c.metrics.FailedRequests++
		if isConnectionError {
			c.metrics.ConnectionErrors++
		}
		if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline exceeded") {
			c.metrics.TimeoutErrors++
		}
		return
	}

	c.metrics.SuccessfulReqs++
	c.metrics.TotalLatency += duration

	if c.metrics.MinLatency == 0 || duration < c.metrics.MinLatency {
		c.metrics.MinLatency = duration
	}
	if duration > c.metrics.MaxLatency {
		c.metrics.MaxLatency = duration
	}

	if c.metrics.SuccessfulReqs > 0 {
		c.metrics.AverageLatency = c.metrics.TotalLatency / time.Duration(c.metrics.SuccessfulReqs)
	}
}

// recordRequest records a request for validation
func (c *HttpClient) recordRequest(req *http.Request, body []byte, method string, startTime time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	headers := make(map[string]string)
	for k, v := range req.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	var parsedBody interface{}
	json.Unmarshal(body, &parsedBody)

	recording := RecordedRequest{
		Method:    req.Method,
		URL:       req.URL.String(),
		Headers:   headers,
		Body:      parsedBody,
		Timestamp: startTime,
		Metadata: map[string]interface{}{
			"lsp_method": method,
		},
	}

	c.recordings = append(c.recordings, recording)
}

// recordResponse records a response for validation
func (c *HttpClient) recordResponse(method string, response json.RawMessage, duration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.recordings) > 0 {
		lastIdx := len(c.recordings) - 1
		c.recordings[lastIdx].Response = response
		c.recordings[lastIdx].Duration = duration
	}
}

// GetMetrics returns the current performance metrics
func (c *HttpClient) GetMetrics() RequestMetrics {
	c.metrics.mu.RLock()
	defer c.metrics.mu.RUnlock()
	return *c.metrics
}

// GetRecordings returns all recorded requests
func (c *HttpClient) GetRecordings() []RecordedRequest {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	recordings := make([]RecordedRequest, len(c.recordings))
	copy(recordings, c.recordings)
	return recordings
}

// ClearRecordings clears all recorded requests
func (c *HttpClient) ClearRecordings() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.recordings = c.recordings[:0]
}

// ClearMetrics resets all performance metrics
func (c *HttpClient) ClearMetrics() {
	c.metrics.mu.Lock()
	defer c.metrics.mu.Unlock()
	c.metrics = &RequestMetrics{
		SubProjectMetrics: make(map[string]*SubProjectMetrics),
	}
}

// SetMockResponse sets a mock response for the given LSP method
func (c *HttpClient) SetMockResponse(method string, response interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.mockResponses[method] = response
}

// ClearMockResponses clears all mock responses
func (c *HttpClient) ClearMockResponses() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.mockResponses = make(map[string]interface{})
}

// drainResponseBody drains remaining response body to enable connection reuse
func (c *HttpClient) drainResponseBody(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	
	// Drain up to 64KB of remaining response body with timeout
	const maxDrainBytes = 64 * 1024
	drained := int64(0)
	buf := make([]byte, 1024)
	
	// Set a short deadline for draining to prevent hanging
	if conn, ok := resp.Body.(interface{ SetReadDeadline(time.Time) error }); ok {
		conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	}
	
	for drained < maxDrainBytes {
		n, err := resp.Body.Read(buf)
		drained += int64(n)
		if err != nil {
			break // EOF or other error - done draining
		}
	}
}

// drainAndCloseResponse properly drains and closes response body for connection reuse
func (c *HttpClient) drainAndCloseResponse(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	
	// First drain any remaining body content
	c.drainResponseBody(resp)
	
	// Then close the body
	resp.Body.Close()
}

// Close gracefully closes the HTTP client and cleans up all resources
func (c *HttpClient) Close() error {
	if transport, ok := c.client.Transport.(*http.Transport); ok {
		// Close all idle connections
		transport.CloseIdleConnections()
		
		// Wait a brief moment for connections to close cleanly
		time.Sleep(10 * time.Millisecond)
	}
	
	// Clear internal state
	c.mu.Lock()
	c.recordings = c.recordings[:0]
	c.mockResponses = make(map[string]interface{})
	c.mu.Unlock()
	
	c.ClearMetrics()
	
	return nil
}

// isConnectionPoolError checks if the error is related to connection pool exhaustion
func (c *HttpClient) isConnectionPoolError(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := err.Error()
	return strings.Contains(errStr, "connection refused") ||
		   strings.Contains(errStr, "too many open files") ||
		   strings.Contains(errStr, "connection reset") ||
		   strings.Contains(errStr, "connection pool") ||
		   strings.Contains(errStr, "no such host") ||
		   strings.Contains(errStr, "network is unreachable")
}

// isRetriableError determines if an error should trigger a retry
func (c *HttpClient) isRetriableError(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := err.Error()
	
	// Don't retry client errors (4xx) 
	if strings.Contains(errStr, "HTTP 4") {
		return false
	}
	
	// Don't retry on parse errors or malformed requests
	if strings.Contains(errStr, "failed to parse") ||
	   strings.Contains(errStr, "failed to marshal") ||
	   strings.Contains(errStr, "invalid") {
		return false
	}
	
	// Retry on server errors (5xx), connection errors, and timeouts
	return strings.Contains(errStr, "HTTP 5") ||
		   strings.Contains(errStr, "connection") ||
		   strings.Contains(errStr, "timeout") ||
		   strings.Contains(errStr, "deadline exceeded") ||
		   strings.Contains(errStr, "temporary") ||
		   strings.Contains(errStr, "request failed")
}

// ValidateConnection performs a comprehensive connection validation
func (c *HttpClient) ValidateConnection(ctx context.Context) error {
	// Perform health check
	if err := c.HealthCheck(ctx); err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	// Test basic LSP functionality with a simple workspace/symbol request
	_, err := c.WorkspaceSymbol(ctx, "test")
	if err != nil {
		return fmt.Errorf("basic LSP functionality test failed: %w", err)
	}

	return nil
}

// Multi-Project Support Methods

// SetWorkspaceRoot sets the workspace root directory for multi-project support
func (c *HttpClient) SetWorkspaceRoot(workspaceRoot string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.workspaceRoot = workspaceRoot
	c.config.WorkspaceRoot = workspaceRoot
}

// GetWorkspaceRoot returns the current workspace root directory
func (c *HttpClient) GetWorkspaceRoot() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.workspaceRoot
}

// DefinitionInSubProject sends a textDocument/definition request scoped to a specific sub-project
func (c *HttpClient) DefinitionInSubProject(ctx context.Context, subProject *SubProjectInfo, fileName string, position Position) ([]Location, error) {
	if subProject == nil {
		return nil, fmt.Errorf("subProject cannot be nil")
	}

	startTime := time.Now()
	fileURI, err := c.buildSubProjectURI(subProject, fileName)
	if err != nil {
		c.updateSubProjectMetrics(subProject, time.Since(startTime), err)
		return nil, fmt.Errorf("failed to build file URI: %w", err)
	}

	if err := c.validateSubProjectFile(subProject, fileName); err != nil {
		c.updateSubProjectMetrics(subProject, time.Since(startTime), err)
		return nil, fmt.Errorf("file validation failed: %w", err)
	}

	locations, err := c.Definition(ctx, fileURI, position)
	c.updateSubProjectMetrics(subProject, time.Since(startTime), err)

	if err != nil {
		return nil, fmt.Errorf("definition request failed in %s project: %w", subProject.Language, err)
	}

	return locations, nil
}

// ReferencesInSubProject sends a textDocument/references request scoped to a specific sub-project
func (c *HttpClient) ReferencesInSubProject(ctx context.Context, subProject *SubProjectInfo, fileName string, position Position, includeDeclaration bool) ([]Location, error) {
	if subProject == nil {
		return nil, fmt.Errorf("subProject cannot be nil")
	}

	startTime := time.Now()
	fileURI, err := c.buildSubProjectURI(subProject, fileName)
	if err != nil {
		c.updateSubProjectMetrics(subProject, time.Since(startTime), err)
		return nil, fmt.Errorf("failed to build file URI: %w", err)
	}

	if err := c.validateSubProjectFile(subProject, fileName); err != nil {
		c.updateSubProjectMetrics(subProject, time.Since(startTime), err)
		return nil, fmt.Errorf("file validation failed: %w", err)
	}

	locations, err := c.References(ctx, fileURI, position, includeDeclaration)
	c.updateSubProjectMetrics(subProject, time.Since(startTime), err)

	if err != nil {
		return nil, fmt.Errorf("references request failed in %s project: %w", subProject.Language, err)
	}

	return locations, nil
}

// WorkspaceSymbolInSubProject sends a workspace/symbol request scoped to a specific sub-project
func (c *HttpClient) WorkspaceSymbolInSubProject(ctx context.Context, subProject *SubProjectInfo, query string) ([]SymbolInformation, error) {
	if subProject == nil {
		return nil, fmt.Errorf("subProject cannot be nil")
	}

	startTime := time.Now()
	
	// For workspace symbol requests, we need to set appropriate headers to scope the request
	originalProjectPath := c.config.ProjectPath
	c.config.ProjectPath = subProject.ProjectPath
	defer func() {
		c.config.ProjectPath = originalProjectPath
	}()

	symbols, err := c.WorkspaceSymbol(ctx, query)
	c.updateSubProjectMetrics(subProject, time.Since(startTime), err)

	if err != nil {
		return nil, fmt.Errorf("workspace symbol request failed in %s project: %w", subProject.Language, err)
	}

	// Track cross-project requests if symbols from other projects are returned
	c.trackCrossProjectRequest(subProject, symbols)

	return symbols, nil
}

// Helper functions for URI handling

// buildSubProjectURI constructs a file:// URI for a file within a sub-project
func (c *HttpClient) buildSubProjectURI(subProject *SubProjectInfo, fileName string) (string, error) {
	if subProject == nil {
		return "", fmt.Errorf("subProject cannot be nil")
	}

	if fileName == "" {
		return "", fmt.Errorf("fileName cannot be empty")
	}

	// If already a file:// URI, return as is
	if strings.HasPrefix(fileName, "file://") {
		return fileName, nil
	}

	// Handle both absolute and relative paths
	var absolutePath string
	if filepath.IsAbs(fileName) {
		absolutePath = fileName
	} else {
		absolutePath = filepath.Join(subProject.ProjectPath, fileName)
	}

	// Clean the path and convert to file:// URI
	cleanPath := filepath.Clean(absolutePath)
	return fmt.Sprintf("file://%s", cleanPath), nil
}

// validateSubProjectFile validates that a file exists within the sub-project boundaries
func (c *HttpClient) validateSubProjectFile(subProject *SubProjectInfo, fileName string) error {
	if subProject == nil {
		return fmt.Errorf("subProject cannot be nil")
	}

	if fileName == "" {
		return fmt.Errorf("fileName cannot be empty")
	}

	// Convert to absolute path
	var absolutePath string
	if strings.HasPrefix(fileName, "file://") {
		absolutePath = strings.TrimPrefix(fileName, "file://")
	} else if filepath.IsAbs(fileName) {
		absolutePath = fileName
	} else {
		absolutePath = filepath.Join(subProject.ProjectPath, fileName)
	}

	// Check if file is within sub-project boundaries
	relPath, err := filepath.Rel(subProject.ProjectPath, absolutePath)
	if err != nil {
		return fmt.Errorf("failed to resolve relative path: %w", err)
	}

	if strings.HasPrefix(relPath, "..") {
		return fmt.Errorf("file is outside sub-project boundaries: %s", fileName)
	}

	// Check if file exists (optional validation)
	if _, err := os.Stat(absolutePath); err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist - this might be OK for some LSP operations
			// Return a warning but don't fail the validation
			return nil
		}
		return fmt.Errorf("failed to stat file: %w", err)
	}

	return nil
}

// updateSubProjectMetrics updates metrics for a specific sub-project
func (c *HttpClient) updateSubProjectMetrics(subProject *SubProjectInfo, duration time.Duration, err error) {
	if !c.config.EnableSubProjectMetrics || subProject == nil {
		return
	}

	c.metrics.mu.Lock()
	defer c.metrics.mu.Unlock()

	// Get or create sub-project metrics
	subMetrics, exists := c.metrics.SubProjectMetrics[subProject.Language]
	if !exists {
		subMetrics = &SubProjectMetrics{
			Language: subProject.Language,
		}
		c.metrics.SubProjectMetrics[subProject.Language] = subMetrics
	}

	subMetrics.mu.Lock()
	defer subMetrics.mu.Unlock()

	subMetrics.TotalRequests++

	if err != nil {
		subMetrics.FailedRequests++
		// Check if it's a routing error
		if strings.Contains(err.Error(), "routing") || strings.Contains(err.Error(), "not found") {
			subMetrics.RoutingErrors++
		}
		return
	}

	subMetrics.SuccessfulRequests++
	subMetrics.TotalLatency += duration

	if subMetrics.SuccessfulRequests > 0 {
		subMetrics.AverageLatency = subMetrics.TotalLatency / time.Duration(subMetrics.SuccessfulRequests)
	}
}

// trackCrossProjectRequest tracks requests that return results from multiple projects
func (c *HttpClient) trackCrossProjectRequest(sourceProject *SubProjectInfo, symbols []SymbolInformation) {
	if !c.config.EnableSubProjectMetrics || sourceProject == nil {
		return
	}

	// Check if any symbols are from different projects
	sourceProjectPath := sourceProject.ProjectPath
	crossProjectFound := false

	for _, symbol := range symbols {
		filePath := strings.TrimPrefix(symbol.Location.URI, "file://")
		if !strings.HasPrefix(filePath, sourceProjectPath) {
			crossProjectFound = true
			break
		}
	}

	if crossProjectFound {
		c.metrics.mu.Lock()
		c.metrics.CrossProjectRequests++
		c.metrics.mu.Unlock()
	}
}

// GetSubProjectMetrics returns metrics for a specific sub-project
func (c *HttpClient) GetSubProjectMetrics(language string) (*SubProjectMetrics, bool) {
	c.metrics.mu.RLock()
	defer c.metrics.mu.RUnlock()
	
	metrics, exists := c.metrics.SubProjectMetrics[language]
	if !exists {
		return nil, false
	}
	
	// Return a copy to prevent external modification
	metrics.mu.RLock()
	defer metrics.mu.RUnlock()
	
	return &SubProjectMetrics{
		Language:           metrics.Language,
		TotalRequests:      metrics.TotalRequests,
		SuccessfulRequests: metrics.SuccessfulRequests,
		FailedRequests:     metrics.FailedRequests,
		AverageLatency:     metrics.AverageLatency,
		RoutingErrors:      metrics.RoutingErrors,
		TotalLatency:       metrics.TotalLatency,
	}, true
}

// GetAllSubProjectMetrics returns metrics for all sub-projects
func (c *HttpClient) GetAllSubProjectMetrics() map[string]*SubProjectMetrics {
	c.metrics.mu.RLock()
	defer c.metrics.mu.RUnlock()
	
	result := make(map[string]*SubProjectMetrics)
	for language, metrics := range c.metrics.SubProjectMetrics {
		metrics.mu.RLock()
		result[language] = &SubProjectMetrics{
			Language:           metrics.Language,
			TotalRequests:      metrics.TotalRequests,
			SuccessfulRequests: metrics.SuccessfulRequests,
			FailedRequests:     metrics.FailedRequests,
			AverageLatency:     metrics.AverageLatency,
			RoutingErrors:      metrics.RoutingErrors,
			TotalLatency:       metrics.TotalLatency,
		}
		metrics.mu.RUnlock()
	}
	
	return result
}

// GetCrossProjectRequestCount returns the number of cross-project requests made
func (c *HttpClient) GetCrossProjectRequestCount() int {
	c.metrics.mu.RLock()
	defer c.metrics.mu.RUnlock()
	return c.metrics.CrossProjectRequests
}

// ClearSubProjectMetrics clears metrics for a specific sub-project
func (c *HttpClient) ClearSubProjectMetrics(language string) {
	c.metrics.mu.Lock()
	defer c.metrics.mu.Unlock()
	
	if metrics, exists := c.metrics.SubProjectMetrics[language]; exists {
		metrics.mu.Lock()
		metrics.TotalRequests = 0
		metrics.SuccessfulRequests = 0
		metrics.FailedRequests = 0
		metrics.AverageLatency = 0
		metrics.RoutingErrors = 0
		metrics.TotalLatency = 0
		metrics.mu.Unlock()
	}
}

// EnableSubProjectMetrics enables or disables sub-project metrics collection
func (c *HttpClient) EnableSubProjectMetrics(enable bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.config.EnableSubProjectMetrics = enable
}

// IsSubProjectMetricsEnabled returns whether sub-project metrics collection is enabled
func (c *HttpClient) IsSubProjectMetricsEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.config.EnableSubProjectMetrics
}