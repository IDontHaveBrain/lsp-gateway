package client

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"

	mcp "lsp-gateway/tests/mcp"
)

type EnhancedMCPTestClient struct {
	transport         mcp.Transport
	connectionMgr     *ConnectionManager
	messageHandler    *MessageHandler
	requestTracker    *RequestTracker
	validator         *ResponseValidator
	performanceMonitor *PerformanceMonitor
	config            MCPTestConfig
	logger            *TestLogger
	
	state            atomic.Value
	shutdown         chan struct{}
	shutdownOnce     sync.Once
	mu               sync.RWMutex
	initialized      bool
	toolCache        []mcp.Tool
}

type MCPTestConfig struct {
	ServerURL         string
	TransportType     TransportType
	ConnectionTimeout time.Duration
	RequestTimeout    time.Duration
	MaxRetries        int
	RetryDelay        time.Duration
	BackoffMultiplier float64
	AutoReconnect     bool
	HealthCheckConfig *HealthCheckConfig
	CircuitBreaker    *CircuitBreakerConfig
	ProtocolVersion   string
	ClientInfo        map[string]interface{}
	Capabilities      map[string]interface{}
	LogLevel          string
	MetricsEnabled    bool
}

type TransportType int

const (
	TransportSTDIO TransportType = iota
	TransportTCP
	TransportHTTP
)

type HealthCheckConfig struct {
	Enabled        bool
	Interval       time.Duration
	Timeout        time.Duration
	FailureCount   int
	RecoveryCount  int
}

type CircuitBreakerConfig struct {
	MaxFailures     int
	Timeout         time.Duration
	MaxRequests     int
	HealthThreshold int
}

type TestLogger struct {
	*log.Logger
	level    LogLevel
	mu       sync.RWMutex
	metrics  *LogMetrics
}

type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

type LogMetrics struct {
	DebugCount int64
	InfoCount  int64
	WarnCount  int64
	ErrorCount int64
}

func NewEnhancedMCPTestClient(config MCPTestConfig, transport mcp.Transport) *EnhancedMCPTestClient {
	logger := NewTestLogger(config.LogLevel, "[EnhancedMCPClient]")
	
	client := &EnhancedMCPTestClient{
		transport:   transport,
		config:     config,
		logger:     logger,
		shutdown:   make(chan struct{}),
		toolCache:  make([]mcp.Tool, 0),
	}
	
	client.state.Store(mcp.Disconnected)
	
	client.connectionMgr = NewConnectionManager(transport, config, logger)
	client.requestTracker = NewRequestTracker(config.RequestTimeout, logger)
	client.messageHandler = NewMessageHandler(client.requestTracker, logger)
	client.validator = NewResponseValidator(logger)
	client.performanceMonitor = NewPerformanceMonitor(config.MetricsEnabled, logger)
	
	return client
}

func DefaultMCPTestConfig() MCPTestConfig {
	return MCPTestConfig{
		ServerURL:         "http://localhost:8080",
		TransportType:     TransportSTDIO,
		ConnectionTimeout: 30 * time.Second,
		RequestTimeout:    10 * time.Second,
		MaxRetries:        3,
		RetryDelay:        1 * time.Second,
		BackoffMultiplier: 2.0,
		AutoReconnect:     true,
		HealthCheckConfig: &HealthCheckConfig{
			Enabled:       true,
			Interval:      30 * time.Second,
			Timeout:       5 * time.Second,
			FailureCount:  3,
			RecoveryCount: 2,
		},
		CircuitBreaker: &CircuitBreakerConfig{
			MaxFailures:     5,
			Timeout:         60 * time.Second,
			MaxRequests:     3,
			HealthThreshold: 2,
		},
		ProtocolVersion: mcp.ProtocolVersion,
		ClientInfo: map[string]interface{}{
			"name":    "enhanced-mcp-test-client",
			"version": "1.0.0",
		},
		Capabilities: map[string]interface{}{
			"tools": true,
		},
		LogLevel:       "info",
		MetricsEnabled: true,
	}
}

func (c *EnhancedMCPTestClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.GetState() == mcp.Connected || c.GetState() == mcp.Initialized {
		return fmt.Errorf("client already connected")
	}
	
	c.setState(mcp.Connecting)
	c.logger.Info("Attempting to connect to MCP server")
	
	start := time.Now()
	err := c.connectionMgr.Connect(ctx)
	connectionDuration := time.Since(start)
	
	c.performanceMonitor.RecordConnectionAttempt(err == nil, connectionDuration)
	
	if err != nil {
		c.setState(mcp.Disconnected)
		c.logger.Error("Failed to connect", "error", err, "duration", connectionDuration)
		return fmt.Errorf("connection failed: %w", err)
	}
	
	c.setState(mcp.Connected)
	c.logger.Info("Successfully connected to MCP server", "duration", connectionDuration)
	
	if err := c.initialize(ctx); err != nil {
		c.setState(mcp.Disconnected)
		c.logger.Error("Initialization failed", "error", err)
		return fmt.Errorf("initialization failed: %w", err)
	}
	
	c.setState(mcp.Initialized)
	c.initialized = true
	c.logger.Info("MCP client fully initialized")
	
	return nil
}

func (c *EnhancedMCPTestClient) initialize(ctx context.Context) error {
	initParams := mcp.InitializeParams{
		ProtocolVersion: c.config.ProtocolVersion,
		Capabilities:    c.config.Capabilities,
		ClientInfo:      c.config.ClientInfo,
	}
	
	resp, err := c.SendRequest(ctx, "initialize", initParams)
	if err != nil {
		return fmt.Errorf("initialize request failed: %w", err)
	}
	
	var initResult mcp.InitializeResult
	if err := json.Unmarshal(resp, &initResult); err != nil {
		return fmt.Errorf("failed to parse initialize response: %w", err)
	}
	
	c.logger.Info("Received initialize response", "serverVersion", initResult.ProtocolVersion)
	
	return nil
}

func (c *EnhancedMCPTestClient) SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	if c.GetState() != mcp.Initialized {
		return nil, fmt.Errorf("client not initialized, current state: %s", c.GetState())
	}
	
	requestID := c.generateRequestID()
	request := &mcp.MCPRequest{
		JSONRPC: mcp.JSONRPCVersion,
		ID:      requestID,
		Method:  method,
		Params:  params,
	}
	
	start := time.Now()
	c.performanceMonitor.RecordRequestStart(method)
	
	response, err := c.messageHandler.SendRequest(ctx, request)
	duration := time.Since(start)
	
	c.performanceMonitor.RecordRequestComplete(method, err == nil, duration)
	
	if err != nil {
		c.logger.Error("Request failed", "method", method, "error", err, "duration", duration)
		return nil, err
	}
	
	if err := c.validator.ValidateResponse(response); err != nil {
		c.logger.Warn("Response validation failed", "method", method, "error", err)
	}
	
	c.logger.Debug("Request completed successfully", "method", method, "duration", duration)
	return response.Result.(json.RawMessage), nil
}

func (c *EnhancedMCPTestClient) SendNotification(ctx context.Context, method string, params interface{}) error {
	if c.GetState() != mcp.Initialized {
		return fmt.Errorf("client not initialized, current state: %s", c.GetState())
	}
	
	notification := &mcp.MCPMessage{
		JSONRPC: mcp.JSONRPCVersion,
		Method:  method,
		Params:  params,
	}
	
	return c.messageHandler.SendNotification(ctx, notification)
}

func (c *EnhancedMCPTestClient) ListTools(ctx context.Context) ([]mcp.Tool, error) {
	c.mu.RLock()
	if len(c.toolCache) > 0 {
		tools := make([]mcp.Tool, len(c.toolCache))
		copy(tools, c.toolCache)
		c.mu.RUnlock()
		return tools, nil
	}
	c.mu.RUnlock()
	
	resp, err := c.SendRequest(ctx, "tools/list", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}
	
	var result mcp.ListToolsResult
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse tools list: %w", err)
	}
	
	tools := make([]mcp.Tool, len(result.Tools))
	for i, toolInfo := range result.Tools {
		tools[i] = mcp.Tool{
			Name:        toolInfo.Name,
			Description: toolInfo.Description,
			InputSchema: toolInfo.InputSchema.(map[string]interface{}),
		}
	}
	
	c.mu.Lock()
	c.toolCache = tools
	c.mu.Unlock()
	
	return tools, nil
}

func (c *EnhancedMCPTestClient) CallTool(ctx context.Context, toolName string, params map[string]interface{}) (*mcp.ToolResult, error) {
	toolCall := mcp.ToolCall{
		Tool:   toolName,
		Params: params,
	}
	
	resp, err := c.SendRequest(ctx, "tools/call", toolCall)
	if err != nil {
		return nil, fmt.Errorf("tool call failed: %w", err)
	}
	
	var result mcp.ToolResult
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to parse tool result: %w", err)
	}
	
	return &result, nil
}

func (c *EnhancedMCPTestClient) GetState() mcp.ClientState {
	return c.state.Load().(mcp.ClientState)
}

func (c *EnhancedMCPTestClient) setState(newState mcp.ClientState) {
	oldState := c.GetState()
	c.state.Store(newState)
	c.logger.Debug("State changed", "from", oldState, "to", newState)
}

func (c *EnhancedMCPTestClient) IsConnected() bool {
	state := c.GetState()
	return state == mcp.Connected || state == mcp.Initialized
}

func (c *EnhancedMCPTestClient) IsHealthy() bool {
	return c.connectionMgr.IsHealthy()
}

func (c *EnhancedMCPTestClient) GetMetrics() *ClientMetrics {
	return &ClientMetrics{
		Connection:  c.connectionMgr.GetMetrics(),
		Performance: c.performanceMonitor.GetMetrics(),
		Messages:    c.messageHandler.GetMetrics(),
		Requests:    c.requestTracker.GetMetrics(),
		Validation:  c.validator.GetMetrics(),
	}
}

func (c *EnhancedMCPTestClient) WaitForState(target mcp.ClientState, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	checkInterval := 50 * time.Millisecond
	
	for {
		if c.GetState() == target {
			return nil
		}
		
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for state %s, current: %s", target, c.GetState())
		}
		
		select {
		case <-c.shutdown:
			return fmt.Errorf("client shutdown while waiting for state %s", target)
		default:
			time.Sleep(checkInterval)
		}
	}
}

func (c *EnhancedMCPTestClient) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.GetState() == mcp.Disconnected {
		return nil
	}
	
	c.setState(mcp.Disconnected)
	c.initialized = false
	
	return c.connectionMgr.Disconnect()
}

func (c *EnhancedMCPTestClient) Close() error {
	c.shutdownOnce.Do(func() {
		close(c.shutdown)
	})
	
	var errors []error
	
	if err := c.Disconnect(); err != nil {
		errors = append(errors, fmt.Errorf("disconnect failed: %w", err))
	}
	
	if c.connectionMgr != nil {
		if err := c.connectionMgr.Close(); err != nil {
			errors = append(errors, fmt.Errorf("connection manager close failed: %w", err))
		}
	}
	
	if c.messageHandler != nil {
		if err := c.messageHandler.Close(); err != nil {
			errors = append(errors, fmt.Errorf("message handler close failed: %w", err))
		}
	}
	
	if c.requestTracker != nil {
		if err := c.requestTracker.Close(); err != nil {
			errors = append(errors, fmt.Errorf("request tracker close failed: %w", err))
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("close errors: %v", errors)
	}
	
	c.logger.Info("Enhanced MCP client closed successfully")
	return nil
}

func (c *EnhancedMCPTestClient) generateRequestID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return fmt.Sprintf("%x", bytes)
}

type ClientMetrics struct {
	Connection  *ConnectionMetrics
	Performance *PerformanceMetrics
	Messages    *MessageMetrics
	Requests    *RequestMetrics
	Validation  *ValidationMetrics
}

func NewTestLogger(levelStr string, prefix string) *TestLogger {
	level := LogLevelInfo
	switch levelStr {
	case "debug":
		level = LogLevelDebug
	case "warn":
		level = LogLevelWarn
	case "error":
		level = LogLevelError
	}
	
	return &TestLogger{
		Logger:  log.New(os.Stdout, prefix+" ", log.LstdFlags|log.Lmicroseconds),
		level:   level,
		metrics: &LogMetrics{},
	}
}

func (l *TestLogger) Debug(msg string, args ...interface{}) {
	if l.level <= LogLevelDebug {
		l.output("DEBUG", msg, args...)
		atomic.AddInt64(&l.metrics.DebugCount, 1)
	}
}

func (l *TestLogger) Info(msg string, args ...interface{}) {
	if l.level <= LogLevelInfo {
		l.output("INFO", msg, args...)
		atomic.AddInt64(&l.metrics.InfoCount, 1)
	}
}

func (l *TestLogger) Warn(msg string, args ...interface{}) {
	if l.level <= LogLevelWarn {
		l.output("WARN", msg, args...)
		atomic.AddInt64(&l.metrics.WarnCount, 1)
	}
}

func (l *TestLogger) Error(msg string, args ...interface{}) {
	if l.level <= LogLevelError {
		l.output("ERROR", msg, args...)
		atomic.AddInt64(&l.metrics.ErrorCount, 1)
	}
}

func (l *TestLogger) output(level, msg string, args ...interface{}) {
	if len(args) == 0 {
		l.Printf("[%s] %s", level, msg)
		return
	}
	
	formattedArgs := make([]string, 0, len(args)/2)
	for i := 0; i < len(args); i += 2 {
		if i+1 < len(args) {
			formattedArgs = append(formattedArgs, fmt.Sprintf("%v=%v", args[i], args[i+1]))
		}
	}
	
	if len(formattedArgs) > 0 {
		l.Printf("[%s] %s %s", level, msg, fmt.Sprintf("(%s)", fmt.Sprintf("%s", formattedArgs)))
	} else {
		l.Printf("[%s] %s", level, msg)
	}
}

func (l *TestLogger) GetMetrics() *LogMetrics {
	return &LogMetrics{
		DebugCount: atomic.LoadInt64(&l.metrics.DebugCount),
		InfoCount:  atomic.LoadInt64(&l.metrics.InfoCount),
		WarnCount:  atomic.LoadInt64(&l.metrics.WarnCount),
		ErrorCount: atomic.LoadInt64(&l.metrics.ErrorCount),
	}
}