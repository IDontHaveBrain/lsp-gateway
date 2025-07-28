package mcp

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)


// TestMCPClient is a test client for MCP protocol testing
type TestMCPClient struct {
	// Connection state management
	state *StateManager
	
	// Message handling
	messageID       int64
	pendingRequests map[string]*PendingRequest
	requestsMu      sync.RWMutex
	
	// Transport interface for sending/receiving messages
	transport Transport
	
	// Configuration
	config ClientConfig
	
	// Logging
	logger *log.Logger
	
	// Metrics
	metrics ClientMetrics
	
	// Tool cache
	toolCache   []Tool
	toolCacheMu sync.RWMutex
	
	// Shutdown
	shutdown     chan struct{}
	shutdownOnce sync.Once
}

// PendingRequest tracks an outgoing request waiting for a response
type PendingRequest struct {
	Request    *MCPRequest
	ResponseCh chan *MCPResponse
	ErrorCh    chan error
	Timestamp  time.Time
	Timeout    time.Duration
}

// ClientConfig holds configuration for the test client
type ClientConfig struct {
	// Connection settings
	ServerURL       string
	ConnectionTimeout time.Duration
	RequestTimeout    time.Duration
	
	// Retry settings
	MaxRetries      int
	RetryDelay      time.Duration
	RetryMultiplier float64
	
	// Protocol settings
	ProtocolVersion string
	ClientInfo      map[string]interface{}
	Capabilities    map[string]interface{}
	
	// Logging
	LogLevel   string
	LogPrefix  string
	LogOutput  *os.File
}

// ClientMetrics tracks client performance metrics
type ClientMetrics struct {
	mu              sync.RWMutex
	RequestsSent    int64
	ResponsesReceived int64
	ErrorsReceived   int64
	AverageLatency   time.Duration
	LastRequestTime  time.Time
	ConnectionTime   time.Time
}

// DefaultClientConfig returns a default configuration
func DefaultClientConfig() ClientConfig {
	return ClientConfig{
		ServerURL:         "http://localhost:8080/mcp",
		ConnectionTimeout: 30 * time.Second,
		RequestTimeout:    10 * time.Second,
		MaxRetries:        3,
		RetryDelay:        1 * time.Second,
		RetryMultiplier:   2.0,
		ProtocolVersion:   ProtocolVersion,
		ClientInfo: map[string]interface{}{
			"name":    "test-mcp-client",
			"version": "1.0.0",
		},
		Capabilities: map[string]interface{}{
			"tools": true,
		},
		LogLevel:  "info",
		LogPrefix: "[TestMCPClient] ",
		LogOutput: os.Stdout,
	}
}

// NewTestMCPClient creates a new test MCP client
func NewTestMCPClient(config ClientConfig, transport Transport) *TestMCPClient {
	logger := log.New(config.LogOutput, config.LogPrefix, log.LstdFlags|log.Lmicroseconds)
	
	client := &TestMCPClient{
		state:           NewStateManager(),
		pendingRequests: make(map[string]*PendingRequest),
		config:          config,
		transport:       transport,
		logger:          logger,
		shutdown:        make(chan struct{}),
	}
	
	// Add state change listener for logging
	client.state.AddStateListener(func(oldState, newState ClientState) {
		logger.Printf("State changed from %s to %s", oldState, newState)
	})
	
	return client
}

// GenerateRequestID generates a unique request ID
func (c *TestMCPClient) GenerateRequestID() interface{} {
	// Use atomic increment combined with random value for uniqueness
	counter := atomic.AddInt64(&c.messageID, 1)
	
	// Add random component
	randomBytes := make([]byte, 4)
	_, err := rand.Read(randomBytes)
	if err != nil {
		// Fallback to timestamp only
		return fmt.Sprintf("%d-%d", time.Now().UnixNano(), counter)
	}
	
	randomValue := binary.BigEndian.Uint32(randomBytes)
	return fmt.Sprintf("%d-%d-%d", time.Now().UnixNano(), counter, randomValue%10000)
}

// CreateRequest creates a new MCP request with a unique ID
func (c *TestMCPClient) CreateRequest(method string, params interface{}) *MCPRequest {
	return &MCPRequest{
		JSONRPC: JSONRPCVersion,
		ID:      c.GenerateRequestID(),
		Method:  method,
		Params:  params,
	}
}

// RegisterPendingRequest registers a request waiting for response
func (c *TestMCPClient) RegisterPendingRequest(req *MCPRequest) (*PendingRequest, error) {
	if req.ID == nil {
		return nil, fmt.Errorf("request must have an ID")
	}
	
	idStr := fmt.Sprintf("%v", req.ID)
	
	pending := &PendingRequest{
		Request:    req,
		ResponseCh: make(chan *MCPResponse, 1),
		ErrorCh:    make(chan error, 1),
		Timestamp:  time.Now(),
		Timeout:    c.config.RequestTimeout,
	}
	
	c.requestsMu.Lock()
	if _, exists := c.pendingRequests[idStr]; exists {
		c.requestsMu.Unlock()
		return nil, fmt.Errorf("request with ID %s already pending", idStr)
	}
	c.pendingRequests[idStr] = pending
	c.requestsMu.Unlock()
	
	// Start timeout timer
	go c.handleRequestTimeout(idStr, pending)
	
	return pending, nil
}

// handleRequestTimeout handles request timeout
func (c *TestMCPClient) handleRequestTimeout(id string, pending *PendingRequest) {
	timer := time.NewTimer(pending.Timeout)
	defer timer.Stop()
	
	select {
	case <-timer.C:
		c.requestsMu.Lock()
		if p, exists := c.pendingRequests[id]; exists && p == pending {
			delete(c.pendingRequests, id)
			c.requestsMu.Unlock()
			pending.ErrorCh <- fmt.Errorf("request timeout after %v", pending.Timeout)
		} else {
			c.requestsMu.Unlock()
		}
	case <-c.shutdown:
		return
	}
}

// GetPendingRequest retrieves a pending request by ID
func (c *TestMCPClient) GetPendingRequest(id interface{}) (*PendingRequest, bool) {
	idStr := fmt.Sprintf("%v", id)
	
	c.requestsMu.RLock()
	defer c.requestsMu.RUnlock()
	
	pending, exists := c.pendingRequests[idStr]
	return pending, exists
}

// RemovePendingRequest removes a pending request
func (c *TestMCPClient) RemovePendingRequest(id interface{}) {
	idStr := fmt.Sprintf("%v", id)
	
	c.requestsMu.Lock()
	defer c.requestsMu.Unlock()
	
	delete(c.pendingRequests, idStr)
}

// UpdateMetrics updates client metrics
func (c *TestMCPClient) UpdateMetrics(event string, latency time.Duration) {
	c.metrics.mu.Lock()
	defer c.metrics.mu.Unlock()
	
	switch event {
	case "request_sent":
		c.metrics.RequestsSent++
		c.metrics.LastRequestTime = time.Now()
	case "response_received":
		c.metrics.ResponsesReceived++
		if c.metrics.AverageLatency == 0 {
			c.metrics.AverageLatency = latency
		} else {
			// Weighted average
			c.metrics.AverageLatency = time.Duration(
				(int64(c.metrics.AverageLatency)*9 + int64(latency)) / 10,
			)
		}
	case "error_received":
		c.metrics.ErrorsReceived++
	case "connected":
		c.metrics.ConnectionTime = time.Now()
	}
}

// GetMetrics returns current client metrics
func (c *TestMCPClient) GetMetrics() ClientMetrics {
	c.metrics.mu.RLock()
	defer c.metrics.mu.RUnlock()
	
	// Return a copy to avoid race conditions
	return ClientMetrics{
		RequestsSent:      c.metrics.RequestsSent,
		ResponsesReceived: c.metrics.ResponsesReceived,
		ErrorsReceived:    c.metrics.ErrorsReceived,
		AverageLatency:    c.metrics.AverageLatency,
		LastRequestTime:   c.metrics.LastRequestTime,
		ConnectionTime:    c.metrics.ConnectionTime,
	}
}

// GetState returns the current connection state
func (c *TestMCPClient) GetState() ClientState {
	return c.state.GetState()
}

// IsConnected returns true if the client is connected
func (c *TestMCPClient) IsConnected() bool {
	return c.state.IsConnected()
}

// IsInitialized returns true if the client is initialized
func (c *TestMCPClient) IsInitialized() bool {
	return c.state.IsInitialized()
}

// WaitForState waits for the client to reach a specific state
func (c *TestMCPClient) WaitForState(state ClientState, timeout time.Duration) error {
	return c.state.WaitForState(state, timeout)
}

// Shutdown gracefully shuts down the client
func (c *TestMCPClient) Shutdown() {
	c.shutdownOnce.Do(func() {
		c.logger.Printf("Shutting down client")
		
		// Cancel all pending requests
		c.requestsMu.Lock()
		for id, pending := range c.pendingRequests {
			pending.ErrorCh <- fmt.Errorf("client shutdown")
			delete(c.pendingRequests, id)
		}
		c.requestsMu.Unlock()
		
		// Close shutdown channel
		close(c.shutdown)
		
		// Update state
		_ = c.state.SetState(Disconnected)
	})
}

// GetPendingRequestCount returns the number of pending requests
func (c *TestMCPClient) GetPendingRequestCount() int {
	c.requestsMu.RLock()
	defer c.requestsMu.RUnlock()
	return len(c.pendingRequests)
}

// GetLogger returns the client logger
func (c *TestMCPClient) GetLogger() *log.Logger {
	return c.logger
}

// GetConfig returns a copy of the client configuration
func (c *TestMCPClient) GetConfig() ClientConfig {
	return c.config
}

// sendRequest sends a request and waits for response
func (c *TestMCPClient) sendRequest(method string, params interface{}) (*MCPMessage, error) {
	// Check if connected
	if !c.transport.IsConnected() {
		return nil, fmt.Errorf("transport not connected")
	}
	
	// Create request
	req := c.CreateRequest(method, params)
	
	// Register pending request
	pending, err := c.RegisterPendingRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to register request: %w", err)
	}
	
	// Convert to MCPMessage
	msg := &MCPMessage{
		JSONRPC: req.JSONRPC,
		ID:      req.ID,
		Method:  req.Method,
		Params:  req.Params,
	}
	
	// Serialize message
	data, err := msg.ToJSON()
	if err != nil {
		c.RemovePendingRequest(req.ID)
		return nil, fmt.Errorf("failed to serialize request: %w", err)
	}
	
	// Send request
	c.logger.Printf("Sending request: %s (id: %v)", method, req.ID)
	if err := c.transport.Send(data); err != nil {
		c.RemovePendingRequest(req.ID)
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	
	// Update metrics
	c.UpdateMetrics("request_sent", 0)
	
	// Wait for response
	return c.waitForResponse(fmt.Sprintf("%v", req.ID), pending.Timeout)
}

// waitForResponse waits for a response with the given request ID
func (c *TestMCPClient) waitForResponse(requestID string, timeout time.Duration) (*MCPMessage, error) {
	pending, exists := c.GetPendingRequest(requestID)
	if !exists {
		return nil, fmt.Errorf("no pending request with ID %s", requestID)
	}
	
	select {
	case resp := <-pending.ResponseCh:
		latency := time.Since(pending.Timestamp)
		c.UpdateMetrics("response_received", latency)
		c.RemovePendingRequest(requestID)
		
		// Convert to MCPMessage
		msg := &MCPMessage{
			JSONRPC: resp.JSONRPC,
			ID:      resp.ID,
			Result:  resp.Result,
			Error:   resp.Error,
		}
		
		if resp.Error != nil {
			c.UpdateMetrics("error_received", 0)
			return msg, fmt.Errorf("server error: %s (code: %d)", resp.Error.Message, resp.Error.Code)
		}
		
		return msg, nil
		
	case err := <-pending.ErrorCh:
		c.UpdateMetrics("error_received", 0)
		c.RemovePendingRequest(requestID)
		return nil, err
		
	case <-time.After(timeout):
		c.UpdateMetrics("error_received", 0)
		c.RemovePendingRequest(requestID)
		return nil, fmt.Errorf("request timeout after %v", timeout)
		
	case <-c.shutdown:
		c.RemovePendingRequest(requestID)
		return nil, fmt.Errorf("client shutdown")
	}
}

// Initialize sends the initialize request to the server
func (c *TestMCPClient) Initialize(clientInfo map[string]interface{}, capabilities map[string]interface{}) (*InitializeResult, error) {
	c.logger.Printf("Initializing MCP connection")
	
	// Check state
	state := c.GetState()
	if state != Connected {
		return nil, fmt.Errorf("invalid state for initialization: %s", state)
	}
	
	// Prepare parameters
	params := InitializeParams{
		ProtocolVersion: c.config.ProtocolVersion,
		Capabilities:    capabilities,
		ClientInfo:      clientInfo,
	}
	
	if params.Capabilities == nil {
		params.Capabilities = c.config.Capabilities
	}
	if params.ClientInfo == nil {
		params.ClientInfo = c.config.ClientInfo
	}
	
	// Send initialize request
	resp, err := c.sendRequest("initialize", params)
	if err != nil {
		return nil, fmt.Errorf("initialize request failed: %w", err)
	}
	
	// Parse result
	if resp.Result == nil {
		return nil, fmt.Errorf("initialize response has no result")
	}
	
	// Convert result to InitializeResult
	resultBytes, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}
	
	var result InitializeResult
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to parse initialize result: %w", err)
	}
	
	// Update state to initialized
	if err := c.state.SetState(Initialized); err != nil {
		return nil, fmt.Errorf("failed to update state: %w", err)
	}
	
	c.logger.Printf("Successfully initialized with protocol version %s", result.ProtocolVersion)
	return &result, nil
}

// ListTools retrieves the list of available tools from the server
func (c *TestMCPClient) ListTools() ([]Tool, error) {
	c.logger.Printf("Listing available tools")
	
	// Check state
	if !c.IsInitialized() {
		return nil, fmt.Errorf("client not initialized")
	}
	
	// Send tools/list request
	resp, err := c.sendRequest("tools/list", nil)
	if err != nil {
		return nil, fmt.Errorf("tools/list request failed: %w", err)
	}
	
	// Parse result
	if resp.Result == nil {
		return nil, fmt.Errorf("tools/list response has no result")
	}
	
	// Convert result to ListToolsResult
	resultBytes, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}
	
	var listResult ListToolsResult
	if err := json.Unmarshal(resultBytes, &listResult); err != nil {
		return nil, fmt.Errorf("failed to parse tools list result: %w", err)
	}
	
	// Convert ToolInfo to Tool
	tools := make([]Tool, len(listResult.Tools))
	for i, info := range listResult.Tools {
		// Convert InputSchema if needed
		var schema map[string]interface{}
		if info.InputSchema != nil {
			if s, ok := info.InputSchema.(map[string]interface{}); ok {
				schema = s
			} else {
				// Try to convert via JSON marshaling
				schemaBytes, err := json.Marshal(info.InputSchema)
				if err == nil {
					json.Unmarshal(schemaBytes, &schema)
				}
			}
		}
		
		tools[i] = Tool{
			Name:        info.Name,
			Description: info.Description,
			InputSchema: schema,
		}
	}
	
	// Update tool cache
	c.toolCacheMu.Lock()
	c.toolCache = tools
	c.toolCacheMu.Unlock()
	
	c.logger.Printf("Found %d available tools", len(tools))
	return tools, nil
}

// CallTool executes a tool with the given parameters
func (c *TestMCPClient) CallTool(toolName string, params map[string]interface{}) (*ToolResult, error) {
	c.logger.Printf("Calling tool: %s", toolName)
	
	// Check state
	if !c.IsInitialized() {
		return nil, fmt.Errorf("client not initialized")
	}
	
	// Prepare tool call parameters
	toolCall := ToolCall{
		Tool:   toolName,
		Params: params,
	}
	
	// Send tools/call request
	resp, err := c.sendRequest("tools/call", toolCall)
	if err != nil {
		return nil, fmt.Errorf("tools/call request failed: %w", err)
	}
	
	// Parse result
	if resp.Result == nil {
		return nil, fmt.Errorf("tools/call response has no result")
	}
	
	// Convert result to ToolResult
	resultBytes, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}
	
	var toolResult ToolResult
	if err := json.Unmarshal(resultBytes, &toolResult); err != nil {
		return nil, fmt.Errorf("failed to parse tool result: %w", err)
	}
	
	// Check if the tool call resulted in an error
	if toolResult.IsError && toolResult.Error != nil {
		c.logger.Printf("Tool %s returned error: %s", toolName, toolResult.Error.Message)
		return &toolResult, fmt.Errorf("tool error: %s", toolResult.Error.Message)
	}
	
	c.logger.Printf("Tool %s completed successfully", toolName)
	return &toolResult, nil
}

// Ping sends a ping request to check connection health
func (c *TestMCPClient) Ping() error {
	c.logger.Printf("Sending ping")
	
	// Check if connected
	if !c.transport.IsConnected() {
		return fmt.Errorf("transport not connected")
	}
	
	// Send ping request with a shorter timeout
	oldTimeout := c.config.RequestTimeout
	c.config.RequestTimeout = 5 * time.Second
	defer func() { c.config.RequestTimeout = oldTimeout }()
	
	resp, err := c.sendRequest("ping", nil)
	if err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}
	
	// Ping should return an empty result
	if resp.Result != nil {
		// Some servers might return a result, which is fine
		c.logger.Printf("Ping successful (with result)")
	} else {
		c.logger.Printf("Ping successful")
	}
	
	return nil
}

// Connect establishes the transport connection
func (c *TestMCPClient) Connect(ctx context.Context) error {
	c.logger.Printf("Connecting to MCP server")
	
	// Check current state
	state := c.GetState()
	if state != Disconnected {
		return fmt.Errorf("invalid state for connection: %s", state)
	}
	
	// Set state to Connecting first
	if err := c.state.SetState(Connecting); err != nil {
		return fmt.Errorf("failed to set connecting state: %w", err)
	}
	
	// Connect transport
	if err := c.transport.Connect(ctx); err != nil {
		c.state.SetState(Disconnected) // Revert on failure
		return fmt.Errorf("transport connection failed: %w", err)
	}
	
	// Update state to Connected
	if err := c.state.SetState(Connected); err != nil {
		c.transport.Disconnect()
		c.state.SetState(Disconnected) // Revert on failure
		return fmt.Errorf("failed to update state: %w", err)
	}
	
	// Update metrics
	c.UpdateMetrics("connected", 0)
	
	// Start message receiver
	go c.receiveMessages()
	
	c.logger.Printf("Successfully connected")
	return nil
}

// Disconnect closes the transport connection
func (c *TestMCPClient) Disconnect() error {
	c.logger.Printf("Disconnecting from MCP server")
	
	// Check if connected
	if !c.transport.IsConnected() {
		return fmt.Errorf("not connected")
	}
	
	// Disconnect transport
	if err := c.transport.Disconnect(); err != nil {
		return fmt.Errorf("transport disconnection failed: %w", err)
	}
	
	// Update state
	if err := c.state.SetState(Disconnected); err != nil {
		return fmt.Errorf("failed to update state: %w", err)
	}
	
	c.logger.Printf("Successfully disconnected")
	return nil
}

// receiveMessages handles incoming messages from the transport
func (c *TestMCPClient) receiveMessages() {
	c.logger.Printf("Starting message receiver")
	defer c.logger.Printf("Message receiver stopped")
	
	for {
		select {
		case <-c.shutdown:
			return
		default:
			// Try to receive a message
			data, err := c.transport.Receive()
			if err != nil {
				// Check if we're still connected
				if !c.transport.IsConnected() {
					c.logger.Printf("Transport disconnected, stopping receiver")
					c.state.SetState(Disconnected)
					return
				}
				// Small delay before retry on error
				time.Sleep(10 * time.Millisecond)
				continue
			}
			
			// Parse message
			msg, err := ParseMCPMessage(data)
			if err != nil {
				c.logger.Printf("Failed to parse message: %v", err)
				continue
			}
			
			// Handle response
			if msg.IsResponse() {
				c.handleResponse(msg)
			} else if msg.IsNotification() {
				c.handleNotification(msg)
			} else if msg.IsRequest() {
				// Test client doesn't handle incoming requests
				c.logger.Printf("Received unexpected request: %s", msg.Method)
			}
		}
	}
}

// handleResponse processes incoming response messages
func (c *TestMCPClient) handleResponse(msg *MCPMessage) {
	if msg.ID == nil {
		c.logger.Printf("Received response without ID")
		return
	}
	
	idStr := fmt.Sprintf("%v", msg.ID)
	pending, exists := c.GetPendingRequest(idStr)
	if !exists {
		c.logger.Printf("Received response for unknown request ID: %s", idStr)
		return
	}
	
	// Convert to MCPResponse
	resp := &MCPResponse{
		JSONRPC: msg.JSONRPC,
		ID:      msg.ID,
		Result:  msg.Result,
		Error:   msg.Error,
	}
	
	// Send response to waiting goroutine
	select {
	case pending.ResponseCh <- resp:
		c.logger.Printf("Delivered response for request ID: %s", idStr)
	default:
		c.logger.Printf("Failed to deliver response for request ID: %s (channel full)", idStr)
	}
}

// handleNotification processes incoming notification messages
func (c *TestMCPClient) handleNotification(msg *MCPMessage) {
	c.logger.Printf("Received notification: %s", msg.Method)
	// Test client doesn't do anything special with notifications
	// In a real implementation, you might want to handle specific notifications
}

// GetCachedTools returns the cached list of tools
func (c *TestMCPClient) GetCachedTools() []Tool {
	c.toolCacheMu.RLock()
	defer c.toolCacheMu.RUnlock()
	
	// Return a copy to avoid race conditions
	tools := make([]Tool, len(c.toolCache))
	copy(tools, c.toolCache)
	return tools
}

// HasTool checks if a tool with the given name exists in the cache
func (c *TestMCPClient) HasTool(toolName string) bool {
	c.toolCacheMu.RLock()
	defer c.toolCacheMu.RUnlock()
	
	for _, tool := range c.toolCache {
		if tool.Name == toolName {
			return true
		}
	}
	return false
}

// GetTool returns a tool by name from the cache
func (c *TestMCPClient) GetTool(toolName string) (*Tool, bool) {
	c.toolCacheMu.RLock()
	defer c.toolCacheMu.RUnlock()
	
	for _, tool := range c.toolCache {
		if tool.Name == toolName {
			// Return a copy
			toolCopy := tool
			return &toolCopy, true
		}
	}
	return nil, false
}

// ValidateToolParams validates parameters against a tool's schema
func (c *TestMCPClient) ValidateToolParams(toolName string, params map[string]interface{}) error {
	tool, exists := c.GetTool(toolName)
	if !exists {
		return fmt.Errorf("tool %s not found", toolName)
	}
	
	// Basic validation - check if schema exists
	if tool.InputSchema == nil {
		// No schema means any params are valid
		return nil
	}
	
	// Check for required fields if schema specifies them
	if required, ok := tool.InputSchema["required"].([]interface{}); ok {
		for _, req := range required {
			if fieldName, ok := req.(string); ok {
				if _, exists := params[fieldName]; !exists {
					return fmt.Errorf("missing required parameter: %s", fieldName)
				}
			}
		}
	}
	
	// Additional schema validation could be added here
	// For now, we just check required fields
	
	return nil
}

// ClearToolCache clears the tool cache
func (c *TestMCPClient) ClearToolCache() {
	c.toolCacheMu.Lock()
	defer c.toolCacheMu.Unlock()
	
	c.toolCache = nil
}

// SetTransport sets a new transport (useful for testing)
func (c *TestMCPClient) SetTransport(transport Transport) {
	c.transport = transport
}

// GetTransport returns the current transport
func (c *TestMCPClient) GetTransport() Transport {
	return c.transport
}

// CallToolWithTimeout executes a tool with a custom timeout
func (c *TestMCPClient) CallToolWithTimeout(toolName string, params map[string]interface{}, timeout time.Duration) (*ToolResult, error) {
	// Temporarily override timeout
	oldTimeout := c.config.RequestTimeout
	c.config.RequestTimeout = timeout
	defer func() { c.config.RequestTimeout = oldTimeout }()
	
	return c.CallTool(toolName, params)
}

// SendRequestWithContext sends a request with context support for cancellation
func (c *TestMCPClient) SendRequestWithContext(ctx context.Context, method string, params interface{}) (*MCPMessage, error) {
	// Check if connected
	if !c.transport.IsConnected() {
		return nil, fmt.Errorf("transport not connected")
	}
	
	// Create request
	req := c.CreateRequest(method, params)
	
	// Register pending request
	pending, err := c.RegisterPendingRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to register request: %w", err)
	}
	
	// Convert to MCPMessage
	msg := &MCPMessage{
		JSONRPC: req.JSONRPC,
		ID:      req.ID,
		Method:  req.Method,
		Params:  req.Params,
	}
	
	// Serialize message
	data, err := msg.ToJSON()
	if err != nil {
		c.RemovePendingRequest(req.ID)
		return nil, fmt.Errorf("failed to serialize request: %w", err)
	}
	
	// Send request
	c.logger.Printf("Sending request with context: %s (id: %v)", method, req.ID)
	if err := c.transport.Send(data); err != nil {
		c.RemovePendingRequest(req.ID)
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	
	// Update metrics
	c.UpdateMetrics("request_sent", 0)
	
	// Wait for response with context
	select {
	case resp := <-pending.ResponseCh:
		latency := time.Since(pending.Timestamp)
		c.UpdateMetrics("response_received", latency)
		c.RemovePendingRequest(req.ID)
		
		// Convert to MCPMessage
		response := &MCPMessage{
			JSONRPC: resp.JSONRPC,
			ID:      resp.ID,
			Result:  resp.Result,
			Error:   resp.Error,
		}
		
		if resp.Error != nil {
			c.UpdateMetrics("error_received", 0)
			return response, fmt.Errorf("server error: %s (code: %d)", resp.Error.Message, resp.Error.Code)
		}
		
		return response, nil
		
	case err := <-pending.ErrorCh:
		c.UpdateMetrics("error_received", 0)
		c.RemovePendingRequest(req.ID)
		return nil, err
		
	case <-ctx.Done():
		c.UpdateMetrics("error_received", 0)
		c.RemovePendingRequest(req.ID)
		return nil, ctx.Err()
		
	case <-c.shutdown:
		c.RemovePendingRequest(req.ID)
		return nil, fmt.Errorf("client shutdown")
	}
}

// StartHeartbeat starts a background heartbeat using ping
func (c *TestMCPClient) StartHeartbeat(interval time.Duration) chan struct{} {
	stop := make(chan struct{})
	
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				if err := c.Ping(); err != nil {
					c.logger.Printf("Heartbeat ping failed: %v", err)
					// Could trigger reconnection logic here
				}
			case <-stop:
				c.logger.Printf("Stopping heartbeat")
				return
			case <-c.shutdown:
				return
			}
		}
	}()
	
	return stop
}

// WaitForConnection waits for the client to be connected and initialized
func (c *TestMCPClient) WaitForConnection(timeout time.Duration) error {
	// First wait for connected state
	if err := c.WaitForState(Connected, timeout); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	
	// Initialize if not already initialized
	if !c.IsInitialized() {
		_, err := c.Initialize(nil, nil)
		if err != nil {
			return fmt.Errorf("failed to initialize: %w", err)
		}
	}
	
	return nil
}

// ExtractTextFromToolResult extracts all text content from a tool result
func ExtractTextFromToolResult(result *ToolResult) string {
	if result == nil || len(result.Content) == 0 {
		return ""
	}
	
	var texts []string
	for _, block := range result.Content {
		if block.Type == "text" && block.Text != "" {
			texts = append(texts, block.Text)
		}
	}
	
	return strings.Join(texts, "\n")
}

// ExtractDataFromToolResult extracts data content from a tool result
func ExtractDataFromToolResult(result *ToolResult) []interface{} {
	if result == nil || len(result.Content) == 0 {
		return nil
	}
	
	var data []interface{}
	for _, block := range result.Content {
		if block.Type == "data" && block.Data != nil {
			data = append(data, block.Data)
		}
	}
	
	return data
}