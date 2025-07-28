package testutils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
}

// DefaultHttpClientConfig returns a default configuration
func DefaultHttpClientConfig() HttpClientConfig {
	return HttpClientConfig{
		BaseURL:            "http://localhost:8080",
		Timeout:            30 * time.Second,
		MaxRetries:         3,
		RetryDelay:         time.Second,
		EnableLogging:      true,
		EnableRecording:    false,
		MockMode:           false,
		WorkspaceID:        "test-workspace",
		ProjectPath:        "/tmp/test-project",
		UserAgent:          "LSP-Gateway-E2E-Test/1.0",
		MaxResponseSize:    10 * 1024 * 1024, // 10MB
		ConnectionPoolSize: 10,
		KeepAlive:          30 * time.Second,
	}
}

// HttpClient provides comprehensive HTTP client for testing real server connections
type HttpClient struct {
	config       HttpClientConfig
	client       *http.Client
	metrics      *RequestMetrics
	recordings   []RecordedRequest
	mockResponses map[string]interface{}
	mu           sync.RWMutex
}

// NewHttpClient creates a new HttpClient with the given configuration
func NewHttpClient(config HttpClientConfig) *HttpClient {
	// Set defaults if not provided
	if config.BaseURL == "" {
		config.BaseURL = "http://localhost:8080"
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
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
		config.KeepAlive = 30 * time.Second
	}

	// Create HTTP transport with connection pooling and keep-alive
	transport := &http.Transport{
		MaxIdleConns:        config.ConnectionPoolSize,
		MaxIdleConnsPerHost: config.ConnectionPoolSize,
		IdleConnTimeout:     config.KeepAlive,
		DisableKeepAlives:   false,
	}

	httpClient := &http.Client{
		Timeout:   config.Timeout,
		Transport: transport,
	}

	return &HttpClient{
		config:        config,
		client:        httpClient,
		metrics:       &RequestMetrics{},
		recordings:    make([]RecordedRequest, 0),
		mockResponses: make(map[string]interface{}),
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
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.updateMetrics(0, fmt.Errorf("health check returned status %d", resp.StatusCode), false)
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	c.updateMetrics(0, nil, false)
	return nil
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

	// Build JSON-RPC request with workspace extensions
	jsonRPCRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      requestID,
		"method":  method,
		"params":  params,
	}

	// Add workspace context if configured
	if c.config.WorkspaceID != "" || c.config.ProjectPath != "" {
		if paramsMap, ok := params.(map[string]interface{}); ok {
			if c.config.WorkspaceID != "" {
				paramsMap["workspace_id"] = c.config.WorkspaceID
			}
			if c.config.ProjectPath != "" {
				paramsMap["project_path"] = c.config.ProjectPath
			}
		}
	}

	requestBody, err := json.Marshal(jsonRPCRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/jsonrpc", c.config.BaseURL)
	
	var response json.RawMessage
	var lastErr error

	// Retry logic with exponential backoff
	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(c.config.RetryDelay * time.Duration(attempt)):
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
	defer resp.Body.Close()

	// Check for HTTP errors
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		duration := time.Since(startTime)
		err = fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
		c.updateMetrics(duration, err, false)
		return nil, err
	}

	// Read response with size limit
	limitReader := io.LimitReader(resp.Body, c.config.MaxResponseSize)
	responseBody, err := io.ReadAll(limitReader)
	if err != nil {
		duration := time.Since(startTime)
		c.updateMetrics(duration, err, false)
		return nil, fmt.Errorf("failed to read response: %w", err)
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
	c.metrics = &RequestMetrics{}
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

// Close gracefully closes the HTTP client and cleans up resources
func (c *HttpClient) Close() error {
	if transport, ok := c.client.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}
	return nil
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