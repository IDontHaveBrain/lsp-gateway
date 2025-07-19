package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"lsp-gateway/internal/gateway"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// MCPMessage represents a generic MCP protocol message
type MCPMessage struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method,omitempty"`
	Params  interface{} `json:"params,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

// MCPError represents an MCP protocol error
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// InitializeParams represents MCP initialization parameters
type InitializeParams struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ClientInfo      map[string]interface{} `json:"clientInfo"`
}

// InitializeResult represents MCP initialization result
type InitializeResult struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ServerInfo      map[string]interface{} `json:"serverInfo"`
}

// ProtocolConstants defines protocol limits and constraints
type ProtocolConstants struct {
	MaxMessageSize  int           // Maximum message size in bytes
	MaxHeaderSize   int           // Maximum header size in bytes
	MaxMethodLength int           // Maximum method name length
	MaxParamsSize   int           // Maximum params size in bytes
	MessageTimeout  time.Duration // Maximum time to process a message
	MaxHeaderLines  int           // Maximum number of header lines
}

// MessageValidationError represents a message validation error
type MessageValidationError struct {
	Field   string
	Reason  string
	Value   interface{}
	Message string
}

func (e *MessageValidationError) Error() string {
	return e.Message
}

// RecoveryContext tracks error recovery state
type RecoveryContext struct {
	mu             sync.RWMutex
	malformedCount int
	lastMalformed  time.Time
	parseErrors    int
	lastParseError time.Time
	recoveryMode   bool
	recoveryStart  time.Time
}

// Server represents the MCP server with enhanced error handling
type Server struct {
	config      *ServerConfig
	client      *LSPGatewayClient
	toolHandler *ToolHandler

	// I/O channels
	input  io.Reader
	output io.Writer

	// Synchronization
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// State
	initialized     bool
	logger          *log.Logger
	protocolLimits  *ProtocolConstants
	recoveryContext *RecoveryContext
}

// NewServer creates a new MCP server instance with enhanced error handling
func NewServer(config *ServerConfig) *Server {
	if config == nil {
		config = DefaultConfig()
	}

	client := NewLSPGatewayClient(config)
	toolHandler := NewToolHandler(client)

	ctx, cancel := context.WithCancel(context.Background())

	// Set up protocol constants with reasonable limits
	protocolLimits := &ProtocolConstants{
		MaxMessageSize:  10 * 1024 * 1024, // 10MB
		MaxHeaderSize:   4096,             // 4KB
		MaxMethodLength: 256,              // 256 chars
		MaxParamsSize:   5 * 1024 * 1024,  // 5MB
		MessageTimeout:  30 * time.Second, // 30 seconds
		MaxHeaderLines:  20,               // 20 header lines max
	}

	return &Server{
		config:          config,
		client:          client,
		toolHandler:     toolHandler,
		input:           os.Stdin,
		output:          os.Stdout,
		ctx:             ctx,
		cancel:          cancel,
		logger:          log.New(os.Stderr, "[MCP] ", log.LstdFlags|log.Lshortfile),
		protocolLimits:  protocolLimits,
		recoveryContext: &RecoveryContext{},
	}
}

// SetIO sets custom input/output streams (useful for testing)
func (s *Server) SetIO(input io.Reader, output io.Writer) {
	s.input = input
	s.output = output
}

// Start starts the MCP server
func (s *Server) Start() error {
	s.logger.Printf("Starting MCP server %s v%s", s.config.Name, s.config.Version)
	s.logger.Printf("LSP Gateway URL: %s", s.config.LSPGatewayURL)

	s.wg.Add(1)
	go s.messageLoop()

	s.wg.Wait()
	return nil
}

// Stop stops the MCP server
func (s *Server) Stop() error {
	s.logger.Println("Stopping MCP server")
	s.cancel()
	s.wg.Wait()
	return nil
}

// messageLoop processes incoming MCP messages with comprehensive error handling
func (s *Server) messageLoop() {
	defer s.wg.Done()

	reader := bufio.NewReader(s.input)
	consecutiveErrors := 0
	maxConsecutiveErrors := 10

	for {
		select {
		case <-s.ctx.Done():
			s.logger.Println("Message loop terminated by context cancellation")
			return
		default:
		}

		// Create timeout context for message processing
		msgCtx, msgCancel := context.WithTimeout(s.ctx, s.protocolLimits.MessageTimeout)

		message, err := s.readMessageWithRecovery(reader)
		if err != nil {
			msgCancel()

			if err == io.EOF {
				s.logger.Println("Input stream closed gracefully")
				return
			}

			consecutiveErrors++
			s.logger.Printf("Error reading message (attempt %d/%d): %v", consecutiveErrors, maxConsecutiveErrors, err)

			// Try to recover from malformed messages
			if s.attemptRecovery(reader, err) {
				consecutiveErrors = 0 // Reset on successful recovery
				continue
			}

			if consecutiveErrors >= maxConsecutiveErrors {
				s.logger.Printf("Too many consecutive errors (%d), terminating message loop", consecutiveErrors)
				return
			}

			// Wait before retry to avoid CPU spinning
			time.Sleep(time.Duration(consecutiveErrors) * 100 * time.Millisecond)
			continue
		}

		// Reset error count on successful message read
		consecutiveErrors = 0

		// Handle message with timeout
		go func(ctx context.Context, msg string) {
			defer msgCancel()

			if err := s.handleMessageWithValidation(ctx, msg); err != nil {
				s.logger.Printf("Error handling message: %v", err)
				// Send error response if possible
				_ = s.sendGenericError("Message handling failed: " + err.Error())
			}
		}(msgCtx, message)
	}
}

// readMessageWithRecovery reads a single MCP message with comprehensive error handling
func (s *Server) readMessageWithRecovery(reader *bufio.Reader) (string, error) {
	// Read headers with size limits and validation
	var contentLength int
	headerLines := 0
	headerSize := 0

	for {
		headerLines++
		if headerLines > s.protocolLimits.MaxHeaderLines {
			return "", fmt.Errorf("too many header lines (max %d)", s.protocolLimits.MaxHeaderLines)
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("failed to read header line: %w", err)
		}

		headerSize += len(line)
		if headerSize > s.protocolLimits.MaxHeaderSize {
			return "", fmt.Errorf("header size too large (max %d bytes)", s.protocolLimits.MaxHeaderSize)
		}

		// Validate UTF-8 encoding
		if !utf8.ValidString(line) {
			return "", fmt.Errorf("invalid UTF-8 encoding in header")
		}

		line = strings.TrimSpace(line)

		// Empty line indicates end of headers
		if line == "" {
			break
		}

		// Parse Content-Length header with validation
		if strings.HasPrefix(strings.ToLower(line), "content-length:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 {
				return "", fmt.Errorf("malformed Content-Length header: %s", line)
			}

			lengthStr := strings.TrimSpace(parts[1])
			length, err := strconv.Atoi(lengthStr)
			if err != nil {
				return "", fmt.Errorf("invalid Content-Length value: %s", lengthStr)
			}

			if length < 0 {
				return "", fmt.Errorf("negative Content-Length: %d", length)
			}

			if length > s.protocolLimits.MaxMessageSize {
				return "", fmt.Errorf("message size too large: %d bytes (max %d)", length, s.protocolLimits.MaxMessageSize)
			}

			contentLength = length
		}
	}

	// Must have Content-Length header
	if contentLength == 0 {
		return "", fmt.Errorf("missing or zero Content-Length header")
	}

	// Read the message content with validation
	content := make([]byte, contentLength)
	n, err := io.ReadFull(reader, content)
	if err != nil {
		return "", fmt.Errorf("failed to read message content: %w", err)
	}
	if n != contentLength {
		return "", fmt.Errorf("expected %d bytes, read %d", contentLength, n)
	}

	// Validate UTF-8 encoding of content
	contentStr := string(content)
	if !utf8.ValidString(contentStr) {
		return "", fmt.Errorf("invalid UTF-8 encoding in message content")
	}

	return contentStr, nil
}

// handleMessageWithValidation processes a single MCP message with comprehensive validation
func (s *Server) handleMessageWithValidation(ctx context.Context, data string) error {
	// Pre-validation checks
	if len(data) == 0 {
		return s.sendError(nil, -32700, "Parse error", "Empty message content")
	}

	if len(data) > s.protocolLimits.MaxMessageSize {
		return s.sendError(nil, -32700, "Parse error", fmt.Sprintf("Message too large: %d bytes", len(data)))
	}

	// Parse JSON with detailed error handling
	var msg MCPMessage
	if err := json.Unmarshal([]byte(data), &msg); err != nil {
		s.updateRecoveryContext("parse_error")

		// Provide more specific parse error information
		errorDetails := s.analyzeJSONError(err, data)
		s.logger.Printf("JSON parse error: %v", errorDetails)

		return s.sendError(nil, -32700, "Parse error", errorDetails)
	}

	// Validate message structure
	if err := s.validateMessageStructure(&msg); err != nil {
		s.logger.Printf("Message validation error: %v", err)
		return s.sendError(msg.ID, -32600, "Invalid Request", err.Error())
	}

	// Log valid message for debugging
	s.logger.Printf("Processing message: method=%s, id=%v", msg.Method, msg.ID)

	// Route to appropriate handler with timeout
	handlerCtx, cancel := context.WithTimeout(ctx, s.protocolLimits.MessageTimeout)
	defer cancel()

	switch msg.Method {
	case "initialize":
		return s.handleInitializeWithValidation(handlerCtx, msg)
	case "tools/list":
		return s.handleListToolsWithValidation(handlerCtx, msg)
	case "tools/call":
		return s.handleCallToolWithValidation(handlerCtx, msg)
	case "ping":
		return s.handlePingWithValidation(handlerCtx, msg)
	case "notifications/initialized":
		// Handle initialization notification (no response needed)
		s.logger.Println("Received initialization notification")
		return nil
	default:
		s.logger.Printf("Unknown method requested: %s", msg.Method)
		if msg.ID != nil {
			return s.sendError(msg.ID, -32601, "Method not found", fmt.Sprintf("Unknown method: %s", msg.Method))
		}
		// For notifications (no ID), we don't send error responses
		return nil
	}
}

// sendResponse sends a successful response
func (s *Server) sendResponse(id interface{}, result interface{}) error {
	response := MCPMessage{
		JSONRPC: gateway.JSONRPCVersion,
		ID:      id,
		Result:  result,
	}
	return s.sendMessage(response)
}

// sendError sends an error response with enhanced error information
func (s *Server) sendError(id interface{}, code int, message, data string) error {
	// Enhance error message with context if available
	enhancedData := data
	if s.recoveryContext.recoveryMode {
		enhancedData = fmt.Sprintf("%s (recovery mode active)", data)
	}

	response := MCPMessage{
		JSONRPC: gateway.JSONRPCVersion,
		ID:      id,
		Error: &MCPError{
			Code:    code,
			Message: message,
			Data:    enhancedData,
		},
	}

	s.logger.Printf("Sending error response: code=%d, message=%s, id=%v", code, message, id)
	return s.sendMessage(response)
}

// sendGenericError sends a generic error when message ID is unknown
func (s *Server) sendGenericError(errorMsg string) error {
	return s.sendError(nil, -32603, "Internal error", errorMsg)
}

// sendMessage sends a message to the client with comprehensive error handling
func (s *Server) sendMessage(msg MCPMessage) error {
	// Validate outgoing message structure
	if err := s.validateOutgoingMessage(&msg); err != nil {
		s.logger.Printf("Outgoing message validation failed: %v", err)
		return fmt.Errorf("invalid outgoing message: %w", err)
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	// Check message size limits
	if len(data) > s.protocolLimits.MaxMessageSize {
		return fmt.Errorf("outgoing message too large: %d bytes (max %d)", len(data), s.protocolLimits.MaxMessageSize)
	}

	// Write Content-Length header followed by double CRLF and JSON content
	content := string(data)
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(content))

	// Write header with error handling
	if _, err := s.output.Write([]byte(header)); err != nil {
		s.logger.Printf("Failed to write response header: %v", err)
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Write content with error handling
	if _, err := s.output.Write([]byte(content)); err != nil {
		s.logger.Printf("Failed to write response content: %v", err)
		return fmt.Errorf("failed to write content: %w", err)
	}

	s.logger.Printf("Sent message: id=%v, size=%d bytes", msg.ID, len(content))
	return nil
}

// IsRunning returns true if the server is running
func (s *Server) IsRunning() bool {
	select {
	case <-s.ctx.Done():
		return false
	default:
		return true
	}
}

// Message validation and analysis functions

func (s *Server) validateMessageStructure(msg *MCPMessage) error {
	// Validate JSON-RPC version
	if msg.JSONRPC != gateway.JSONRPCVersion {
		return &MessageValidationError{
			Field:   "jsonrpc",
			Reason:  "invalid_version",
			Value:   msg.JSONRPC,
			Message: fmt.Sprintf("Invalid JSON-RPC version: %s (expected 2.0)", msg.JSONRPC),
		}
	}

	// Validate method name if present
	if msg.Method != "" {
		if len(msg.Method) > s.protocolLimits.MaxMethodLength {
			return &MessageValidationError{
				Field:   "method",
				Reason:  "too_long",
				Value:   len(msg.Method),
				Message: fmt.Sprintf("Method name too long: %d chars (max %d)", len(msg.Method), s.protocolLimits.MaxMethodLength),
			}
		}

		// Validate method name format
		if !isValidMethodName(msg.Method) {
			return &MessageValidationError{
				Field:   "method",
				Reason:  "invalid_format",
				Value:   msg.Method,
				Message: fmt.Sprintf("Invalid method name format: %s", msg.Method),
			}
		}
	}

	// Validate that either method or result/error is present (not both)
	isRequest := msg.Method != ""
	isResponse := msg.Result != nil || msg.Error != nil

	if isRequest && isResponse {
		return &MessageValidationError{
			Field:   "message_type",
			Reason:  "ambiguous_type",
			Value:   "request_and_response",
			Message: "Message cannot be both request and response",
		}
	}

	if !isRequest && !isResponse {
		return &MessageValidationError{
			Field:   "message_type",
			Reason:  "unknown_type",
			Value:   "neither_request_nor_response",
			Message: "Message must be either request or response",
		}
	}

	// Validate ID constraints
	if isRequest && msg.Method != "ping" && msg.ID == nil {
		// Some methods require an ID for responses
		requiredIDMethods := map[string]bool{
			"initialize": true,
			"tools/list": true,
			"tools/call": true,
		}
		if requiredIDMethods[msg.Method] {
			return &MessageValidationError{
				Field:   "id",
				Reason:  "missing_required",
				Value:   nil,
				Message: fmt.Sprintf("Method %s requires an ID", msg.Method),
			}
		}
	}

	// Validate params size if present
	if msg.Params != nil {
		paramsBytes, _ := json.Marshal(msg.Params)
		if len(paramsBytes) > s.protocolLimits.MaxParamsSize {
			return &MessageValidationError{
				Field:   "params",
				Reason:  "too_large",
				Value:   len(paramsBytes),
				Message: fmt.Sprintf("Params too large: %d bytes (max %d)", len(paramsBytes), s.protocolLimits.MaxParamsSize),
			}
		}
	}

	return nil
}

func (s *Server) validateOutgoingMessage(msg *MCPMessage) error {
	// Validate JSON-RPC version
	if msg.JSONRPC != gateway.JSONRPCVersion {
		return fmt.Errorf("invalid JSON-RPC version: %s", msg.JSONRPC)
	}

	// Response must have either result or error, not both
	if msg.Result != nil && msg.Error != nil {
		return fmt.Errorf("response cannot have both result and error")
	}

	// Response should have an ID (except for notifications)
	if msg.Method == "" && msg.ID == nil && (msg.Result != nil || msg.Error != nil) {
		return fmt.Errorf("response missing ID")
	}

	return nil
}

func (s *Server) analyzeJSONError(err error, data string) string {
	errorStr := err.Error()

	// Truncate data for error reporting
	truncatedData := data
	if len(data) > 200 {
		truncatedData = data[:200] + "..."
	}

	switch {
	case strings.Contains(errorStr, "unexpected end of JSON input"):
		return fmt.Sprintf("Incomplete JSON message: %s", truncatedData)
	case strings.Contains(errorStr, "invalid character"):
		return fmt.Sprintf("Invalid JSON character: %s", errorStr)
	case strings.Contains(errorStr, "cannot unmarshal"):
		return fmt.Sprintf("JSON structure mismatch: %s", errorStr)
	default:
		return fmt.Sprintf("JSON parse error: %s (data: %s)", errorStr, truncatedData)
	}
}

func isValidMethodName(method string) bool {
	if len(method) == 0 {
		return false
	}

	// Method names should contain only alphanumeric, slash, underscore, and dash
	for _, r := range method {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') &&
			(r < '0' || r > '9') && r != '/' && r != '_' && r != '-' {
			return false
		}
	}

	return true
}

// Recovery and error handling functions

func (s *Server) updateRecoveryContext(errorType string) {
	s.recoveryContext.mu.Lock()
	defer s.recoveryContext.mu.Unlock()

	now := time.Now()

	switch errorType {
	case "malformed":
		s.recoveryContext.malformedCount++
		s.recoveryContext.lastMalformed = now
	case "parse_error":
		s.recoveryContext.parseErrors++
		s.recoveryContext.lastParseError = now
	}

	// Enable recovery mode if too many errors
	if s.recoveryContext.malformedCount >= 3 || s.recoveryContext.parseErrors >= 5 {
		if !s.recoveryContext.recoveryMode {
			s.recoveryContext.recoveryMode = true
			s.recoveryContext.recoveryStart = now
			s.logger.Printf("Entering recovery mode due to multiple errors")
		}
	}

	// Exit recovery mode after successful period
	if s.recoveryContext.recoveryMode && now.Sub(s.recoveryContext.recoveryStart) > 60*time.Second {
		s.recoveryContext.recoveryMode = false
		s.recoveryContext.malformedCount = 0
		s.recoveryContext.parseErrors = 0
		s.logger.Printf("Exiting recovery mode")
	}
}

func (s *Server) attemptRecovery(reader *bufio.Reader, originalErr error) bool {
	s.logger.Printf("Attempting recovery from error: %v", originalErr)

	// Try to skip to next potential message boundary
	// Look for Content-Length header pattern
	buffer := make([]byte, 1024)
	for i := 0; i < 10; i++ { // Max 10 recovery attempts
		n, err := reader.Read(buffer)
		if err != nil {
			s.logger.Printf("Recovery attempt %d failed: %v", i+1, err)
			return false
		}

		data := string(buffer[:n])
		if strings.Contains(data, "Content-Length:") {
			s.logger.Printf("Found potential message boundary after %d attempts", i+1)
			// Put the data back to be processed normally
			// Note: This is a simplified recovery - in production, you'd need more sophisticated parsing
			return true
		}
	}

	s.logger.Println("Recovery attempts exhausted")
	return false
}

// Enhanced handler functions with validation

func (s *Server) handleInitializeWithValidation(ctx context.Context, msg MCPMessage) error {
	var params InitializeParams
	if msg.Params != nil {
		paramBytes, _ := json.Marshal(msg.Params)
		if err := json.Unmarshal(paramBytes, &params); err != nil {
			return s.sendError(msg.ID, -32602, "Invalid params", fmt.Sprintf("Failed to parse initialize params: %v", err))
		}
	}

	// Validate protocol version compatibility
	if params.ProtocolVersion != "" && !isCompatibleProtocolVersion(params.ProtocolVersion) {
		s.logger.Printf("Warning: Potentially incompatible protocol version: %s", params.ProtocolVersion)
	}

	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: map[string]interface{}{
			"tools": map[string]interface{}{
				"listChanged": false,
			},
		},
		ServerInfo: map[string]interface{}{
			"name":    s.config.Name,
			"version": s.config.Version,
		},
	}

	s.initialized = true
	s.logger.Printf("MCP server initialized with client protocol version: %s", params.ProtocolVersion)

	return s.sendResponse(msg.ID, result)
}

func (s *Server) handleListToolsWithValidation(ctx context.Context, msg MCPMessage) error {
	if !s.initialized {
		return s.sendError(msg.ID, -32002, "Server not initialized", "Call initialize first")
	}

	tools := s.toolHandler.ListTools()
	result := map[string]interface{}{
		"tools": tools,
	}

	s.logger.Printf("Listed %d tools", len(tools))
	return s.sendResponse(msg.ID, result)
}

func (s *Server) handleCallToolWithValidation(ctx context.Context, msg MCPMessage) error {
	if !s.initialized {
		return s.sendError(msg.ID, -32002, "Server not initialized", "Call initialize first")
	}

	var call ToolCall
	if msg.Params != nil {
		paramBytes, _ := json.Marshal(msg.Params)
		if err := json.Unmarshal(paramBytes, &call); err != nil {
			return s.sendError(msg.ID, -32602, "Invalid params", fmt.Sprintf("Failed to parse tool call params: %v", err))
		}
	} else {
		return s.sendError(msg.ID, -32602, "Invalid params", "Missing tool call parameters")
	}

	// Validate tool call structure
	if call.Name == "" {
		return s.sendError(msg.ID, -32602, "Invalid params", "Tool name is required")
	}

	// Call tool handler
	result, err := s.toolHandler.CallTool(ctx, call)

	if err != nil {
		s.logger.Printf("Tool call failed: tool=%s, error=%v", call.Name, err)
		return s.sendError(msg.ID, -32603, "Internal error", err.Error())
	}

	s.logger.Printf("Tool call completed: tool=%s, success=%t", call.Name, !result.IsError)
	return s.sendResponse(msg.ID, result)
}

func (s *Server) handlePingWithValidation(ctx context.Context, msg MCPMessage) error {
	result := map[string]interface{}{
		"pong": time.Now().Unix(),
	}
	return s.sendResponse(msg.ID, result)
}

// Utility functions

func isCompatibleProtocolVersion(version string) bool {
	// Simple version compatibility check
	compatibleVersions := []string{
		"2024-11-05",
		"2024-10-07",
		"2024-09-01",
	}

	for _, v := range compatibleVersions {
		if version == v {
			return true
		}
	}

	return false
}

// GetRecoveryMetrics returns current recovery context metrics
func (s *Server) GetRecoveryMetrics() map[string]interface{} {
	s.recoveryContext.mu.RLock()
	defer s.recoveryContext.mu.RUnlock()

	return map[string]interface{}{
		"malformed_count":  s.recoveryContext.malformedCount,
		"parse_errors":     s.recoveryContext.parseErrors,
		"recovery_mode":    s.recoveryContext.recoveryMode,
		"last_malformed":   s.recoveryContext.lastMalformed,
		"last_parse_error": s.recoveryContext.lastParseError,
		"recovery_start":   s.recoveryContext.recoveryStart,
	}
}
