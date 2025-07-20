package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	JSONRPCFieldName           = "jsonrpc"
	ProtocolVersion            = "2024-11-05"
	MCPLogPrefix               = "[MCP] "
	ContentTypeApplicationJSON = "application/json"
	ContentLengthHeader        = "Content-Length"
	ContentTypeHeader          = "Content-Type"
)

const (
	JSONRPCErrorCodeParseError     = -32700
	JSONRPCErrorCodeInvalidRequest = -32600
	JSONRPCErrorCodeMethodNotFound = -32601
	JSONRPCErrorCodeInvalidParams  = -32602
	JSONRPCErrorCodeInternalError  = -32603
	JSONRPCErrorCodeServerNotInit  = -32002
)

const (
	JSONRPCErrorMessageParseError     = "Parse error"
	JSONRPCErrorMessageInvalidRequest = "Invalid Request"
	JSONRPCErrorMessageMethodNotFound = "Method not found"
	JSONRPCErrorMessageInvalidParams  = "Invalid params"
	JSONRPCErrorMessageInternalError  = "Internal error"
	JSONRPCErrorMessageServerNotInit  = "Server not initialized"
)

const (
	MCPMethodInitialize       = "initialize"
	MCPMethodToolsList        = "tools/list"
	MCPMethodToolsCall        = "tools/call"
	MCPMethodPing             = "ping"
	MCPMethodNotificationInit = "notifications/initialized"
)

const (
	MCPRecoveryContextMalformed  = "malformed"
	MCPRecoveryContextParseError = "parse_error"
)

const (
	MCPValidationFieldJSONRPC     = "jsonrpc"
	MCPValidationFieldMethod      = "method"
	MCPValidationFieldID          = "id"
	MCPValidationFieldParams      = "params"
	MCPValidationFieldMessageType = "message_type"
)

const (
	MCPValidationReasonInvalidVersion  = "invalid_version"
	MCPValidationReasonTooLong         = "too_long"
	MCPValidationReasonInvalidFormat   = "invalid_format"
	MCPValidationReasonMissingRequired = "missing_required"
	MCPValidationReasonTooLarge        = "too_large"
	MCPValidationReasonAmbiguousType   = "ambiguous_type"
	MCPValidationReasonUnknownType     = "unknown_type"
)

const (
	MCPValidationValueRequestAndResponse    = "request_and_response"
	MCPValidationValueNeitherRequestNorResp = "neither_request_nor_response"
)

type MCPMessage struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method,omitempty"`
	Params  interface{} `json:"params,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type InitializeParams struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ClientInfo      map[string]interface{} `json:"clientInfo"`
}

type InitializeResult struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ServerInfo      map[string]interface{} `json:"serverInfo"`
}

type ProtocolConstants struct {
	MaxMessageSize  int
	MaxHeaderSize   int
	MaxMethodLength int
	MaxParamsSize   int
	MessageTimeout  time.Duration
	MaxHeaderLines  int
}

type MessageValidationError struct {
	Field   string
	Reason  string
	Value   interface{}
	Message string
}

func (e *MessageValidationError) Error() string {
	return e.Message
}

type RecoveryContext struct {
	mu             sync.RWMutex
	malformedCount int
	lastMalformed  time.Time
	parseErrors    int
	lastParseError time.Time
	recoveryMode   bool
	recoveryStart  time.Time
}

type Server struct {
	config      *ServerConfig
	client      *LSPGatewayClient
	toolHandler *ToolHandler

	input  io.Reader
	output io.Writer

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	initialized     bool
	logger          *log.Logger
	protocolLimits  *ProtocolConstants
	recoveryContext *RecoveryContext
}

func NewServer(config *ServerConfig) *Server {
	if config == nil {
		config = DefaultConfig()
	}

	client := NewLSPGatewayClient(config)
	toolHandler := NewToolHandler(client)

	ctx, cancel := context.WithCancel(context.Background())

	protocolLimits := &ProtocolConstants{
		MaxMessageSize:  10 * 1024 * 1024,
		MaxHeaderSize:   4096,
		MaxMethodLength: 256,
		MaxParamsSize:   5 * 1024 * 1024,
		MessageTimeout:  30 * time.Second,
		MaxHeaderLines:  20,
	}

	return &Server{
		config:          config,
		client:          client,
		toolHandler:     toolHandler,
		input:           os.Stdin,
		output:          os.Stdout,
		ctx:             ctx,
		cancel:          cancel,
		logger:          log.New(os.Stderr, MCPLogPrefix, log.LstdFlags|log.Lshortfile),
		protocolLimits:  protocolLimits,
		recoveryContext: &RecoveryContext{},
	}
}

func (s *Server) SetIO(input io.Reader, output io.Writer) {
	s.input = input
	s.output = output
}

func (s *Server) Start() error {
	s.logger.Printf("Starting MCP server %s v%s", s.config.Name, s.config.Version)
	s.logger.Printf("LSP Gateway URL: %s", s.config.LSPGatewayURL)

	s.wg.Add(1)
	go s.messageLoop()

	s.wg.Wait()
	return nil
}

func (s *Server) Stop() error {
	s.logger.Println("Stopping MCP server")
	s.cancel()
	s.wg.Wait()
	return nil
}

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

		msgCtx, msgCancel := context.WithTimeout(s.ctx, s.protocolLimits.MessageTimeout)

		message, err := s.readMessageWithRecovery(reader)
		if err != nil {
			msgCancel()

			if err == io.EOF {
				s.logger.Println("Input stream closed gracefully")
				return
			}

			if s.isEOFError(err) {
				s.logger.Println("EOF-related error detected, terminating gracefully")
				return
			}

			consecutiveErrors++
			s.logger.Printf("Error reading message (attempt %d/%d): %v", consecutiveErrors, maxConsecutiveErrors, err)

			if s.shouldAttemptRecovery(err) && s.attemptRecovery(reader, err) {
				consecutiveErrors = 0
				continue
			}

			if consecutiveErrors >= maxConsecutiveErrors {
				s.logger.Printf("Too many consecutive errors (%d), terminating message loop", consecutiveErrors)
				return
			}

			backoffDuration := time.Duration(consecutiveErrors) * 100 * time.Millisecond
			if backoffDuration > 2*time.Second {
				backoffDuration = 2 * time.Second
			}

			select {
			case <-s.ctx.Done():
				return
			case <-time.After(backoffDuration):
			}
			continue
		}

		consecutiveErrors = 0

		go func(ctx context.Context, msg string) {
			defer msgCancel()

			if err := s.handleMessageWithValidation(ctx, msg); err != nil {
				s.logger.Printf("Error handling message: %v", err)
				_ = s.sendGenericError("Message handling failed: " + err.Error())
			}
		}(msgCtx, message)
	}
}

func (s *Server) readMessageWithRecovery(reader *bufio.Reader) (string, error) {
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
			if err == io.EOF {
				return "", err
			}
			return "", fmt.Errorf("failed to read header line: %w", err)
		}

		headerSize += len(line)
		if headerSize > s.protocolLimits.MaxHeaderSize {
			return "", fmt.Errorf("header size too large (max %d bytes)", s.protocolLimits.MaxHeaderSize)
		}

		if !utf8.ValidString(line) {
			return "", fmt.Errorf("invalid UTF-8 encoding in header")
		}

		line = strings.TrimSpace(line)

		if line == "" {
			break
		}

		if strings.HasPrefix(strings.ToLower(line), strings.ToLower(ContentLengthHeader)+":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 {
				return "", fmt.Errorf("malformed %s header: %s", ContentLengthHeader, line)
			}

			lengthStr := strings.TrimSpace(parts[1])
			length, err := strconv.Atoi(lengthStr)
			if err != nil {
				return "", fmt.Errorf("invalid %s value: %s", ContentLengthHeader, lengthStr)
			}

			if length < 0 {
				return "", fmt.Errorf("negative %s: %d", ContentLengthHeader, length)
			}

			if length > s.protocolLimits.MaxMessageSize {
				return "", fmt.Errorf("message size too large: %d bytes (max %d)", length, s.protocolLimits.MaxMessageSize)
			}

			contentLength = length
		}
	}

	if contentLength == 0 {
		return "", fmt.Errorf("missing or zero %s header", ContentLengthHeader)
	}

	content := make([]byte, contentLength)
	n, err := io.ReadFull(reader, content)
	if err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return "", err
		}
		return "", fmt.Errorf("failed to read message content: %w", err)
	}
	if n != contentLength {
		return "", fmt.Errorf("expected %d bytes, read %d", contentLength, n)
	}

	contentStr := string(content)
	if !utf8.ValidString(contentStr) {
		return "", fmt.Errorf("invalid UTF-8 encoding in message content")
	}

	return contentStr, nil
}

func (s *Server) handleMessageWithValidation(ctx context.Context, data string) error {
	if len(data) == 0 {
		return s.sendError(nil, JSONRPCErrorCodeParseError, JSONRPCErrorMessageParseError, "Empty message content")
	}

	if len(data) > s.protocolLimits.MaxMessageSize {
		return s.sendError(nil, JSONRPCErrorCodeParseError, JSONRPCErrorMessageParseError, fmt.Sprintf("Message too large: %d bytes", len(data)))
	}

	var msg MCPMessage
	if err := json.Unmarshal([]byte(data), &msg); err != nil {
		s.updateRecoveryContext(MCPRecoveryContextParseError)

		errorDetails := s.analyzeJSONError(err, data)
		s.logger.Printf("JSON parse error: %v", errorDetails)

		return s.sendError(nil, JSONRPCErrorCodeParseError, JSONRPCErrorMessageParseError, errorDetails)
	}

	if err := s.validateMessageStructure(&msg); err != nil {
		s.logger.Printf("Message validation error: %v", err)
		return s.sendError(msg.ID, JSONRPCErrorCodeInvalidRequest, JSONRPCErrorMessageInvalidRequest, err.Error())
	}

	s.logger.Printf("Processing message: method=%s, id=%v", msg.Method, msg.ID)

	handlerCtx, cancel := context.WithTimeout(ctx, s.protocolLimits.MessageTimeout)
	defer cancel()

	switch msg.Method {
	case MCPMethodInitialize:
		return s.handleInitializeWithValidation(handlerCtx, msg)
	case MCPMethodToolsList:
		return s.handleListToolsWithValidation(handlerCtx, msg)
	case MCPMethodToolsCall:
		return s.handleCallToolWithValidation(handlerCtx, msg)
	case MCPMethodPing:
		return s.handlePingWithValidation(handlerCtx, msg)
	case MCPMethodNotificationInit:
		s.logger.Println("Received initialization notification")
		return nil
	default:
		s.logger.Printf("Unknown method requested: %s", msg.Method)
		if msg.ID != nil {
			return s.sendError(msg.ID, JSONRPCErrorCodeMethodNotFound, JSONRPCErrorMessageMethodNotFound, fmt.Sprintf("Unknown method: %s", msg.Method))
		}
		return nil
	}
}

func (s *Server) sendResponse(id interface{}, result interface{}) error {
	response := MCPMessage{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Result:  result,
	}
	return s.sendMessage(response)
}

func (s *Server) sendError(id interface{}, code int, message, data string) error {
	enhancedData := data
	if s.recoveryContext.recoveryMode {
		enhancedData = data + " (recovery mode active)"
	}

	response := MCPMessage{
		JSONRPC: JSONRPCVersion,
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

func (s *Server) sendGenericError(errorMsg string) error {
	return s.sendError(nil, JSONRPCErrorCodeInternalError, JSONRPCErrorMessageInternalError, errorMsg)
}

func (s *Server) sendMessage(msg MCPMessage) error {
	if err := s.validateOutgoingMessage(&msg); err != nil {
		s.logger.Printf("Outgoing message validation failed: %v", err)
		return fmt.Errorf("invalid outgoing message: %w", err)
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	if len(data) > s.protocolLimits.MaxMessageSize {
		return fmt.Errorf("outgoing message too large: %d bytes (max %d)", len(data), s.protocolLimits.MaxMessageSize)
	}

	content := string(data)
	header := ContentLengthHeader + ": " + strconv.Itoa(len(content)) + "\r\n\r\n"

	select {
	case <-s.ctx.Done():
		return fmt.Errorf("context cancelled while writing message")
	default:
	}

	if _, err := s.output.Write([]byte(header)); err != nil {
		if s.isConnectionError(err) {
			s.logger.Printf("Connection closed while writing header: %v", err)
			return fmt.Errorf("connection closed: %w", err)
		}
		s.logger.Printf("Failed to write response header: %v", err)
		return fmt.Errorf("failed to write header: %w", err)
	}

	if _, err := s.output.Write([]byte(content)); err != nil {
		if s.isConnectionError(err) {
			s.logger.Printf("Connection closed while writing content: %v", err)
			return fmt.Errorf("connection closed: %w", err)
		}
		s.logger.Printf("Failed to write response content: %v", err)
		return fmt.Errorf("failed to write content: %w", err)
	}

	s.logger.Printf("Sent message: id=%v, size=%d bytes", msg.ID, len(content))
	return nil
}

func (s *Server) IsRunning() bool {
	select {
	case <-s.ctx.Done():
		return false
	default:
		return true
	}
}

func (s *Server) validateMessageStructure(msg *MCPMessage) error {
	if msg.JSONRPC != JSONRPCVersion {
		return &MessageValidationError{
			Field:   MCPValidationFieldJSONRPC,
			Reason:  MCPValidationReasonInvalidVersion,
			Value:   msg.JSONRPC,
			Message: fmt.Sprintf("Invalid JSON-RPC version: %s (expected %s)", msg.JSONRPC, JSONRPCVersion),
		}
	}

	if msg.Method != "" {
		if len(msg.Method) > s.protocolLimits.MaxMethodLength {
			return &MessageValidationError{
				Field:   MCPValidationFieldMethod,
				Reason:  MCPValidationReasonTooLong,
				Value:   len(msg.Method),
				Message: fmt.Sprintf("Method name too long: %d chars (max %d)", len(msg.Method), s.protocolLimits.MaxMethodLength),
			}
		}

		if !isValidMethodName(msg.Method) {
			return &MessageValidationError{
				Field:   MCPValidationFieldMethod,
				Reason:  MCPValidationReasonInvalidFormat,
				Value:   msg.Method,
				Message: fmt.Sprintf("Invalid method name format: %s", msg.Method),
			}
		}
	}

	isRequest := msg.Method != ""
	isResponse := msg.Result != nil || msg.Error != nil

	if isRequest && isResponse {
		return &MessageValidationError{
			Field:   MCPValidationFieldMessageType,
			Reason:  MCPValidationReasonAmbiguousType,
			Value:   MCPValidationValueRequestAndResponse,
			Message: "Message cannot be both request and response",
		}
	}

	if !isRequest && !isResponse {
		return &MessageValidationError{
			Field:   MCPValidationFieldMessageType,
			Reason:  MCPValidationReasonUnknownType,
			Value:   MCPValidationValueNeitherRequestNorResp,
			Message: "Message must be either request or response",
		}
	}

	if isRequest && msg.Method != MCPMethodPing && msg.ID == nil {
		requiredIDMethods := map[string]bool{
			MCPMethodInitialize: true,
			MCPMethodToolsList:  true,
			MCPMethodToolsCall:  true,
		}
		if requiredIDMethods[msg.Method] {
			return &MessageValidationError{
				Field:   MCPValidationFieldID,
				Reason:  MCPValidationReasonMissingRequired,
				Value:   nil,
				Message: fmt.Sprintf("Method %s requires an ID", msg.Method),
			}
		}
	}

	if msg.Params != nil {
		paramsBytes, _ := json.Marshal(msg.Params)
		if len(paramsBytes) > s.protocolLimits.MaxParamsSize {
			return &MessageValidationError{
				Field:   MCPValidationFieldParams,
				Reason:  MCPValidationReasonTooLarge,
				Value:   len(paramsBytes),
				Message: fmt.Sprintf("Params too large: %d bytes (max %d)", len(paramsBytes), s.protocolLimits.MaxParamsSize),
			}
		}
	}

	return nil
}

func (s *Server) validateOutgoingMessage(msg *MCPMessage) error {
	if msg.JSONRPC != JSONRPCVersion {
		return fmt.Errorf(ERROR_INVALID_JSON_RPC, msg.JSONRPC)
	}

	if msg.Result != nil && msg.Error != nil {
		return fmt.Errorf("response cannot have both result and error")
	}

	if msg.Method == "" && msg.ID == nil && (msg.Result != nil || msg.Error != nil) {
		return fmt.Errorf("response missing ID")
	}

	return nil
}

func (s *Server) analyzeJSONError(err error, data string) string {
	errorStr := err.Error()

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

	for _, r := range method {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') &&
			(r < '0' || r > '9') && r != '/' && r != '_' && r != '-' {
			return false
		}
	}

	return true
}

func (s *Server) updateRecoveryContext(errorType string) {
	s.recoveryContext.mu.Lock()
	defer s.recoveryContext.mu.Unlock()

	now := time.Now()

	switch errorType {
	case MCPRecoveryContextMalformed:
		s.recoveryContext.malformedCount++
		s.recoveryContext.lastMalformed = now
	case MCPRecoveryContextParseError:
		s.recoveryContext.parseErrors++
		s.recoveryContext.lastParseError = now
	}

	if s.recoveryContext.malformedCount >= 3 || s.recoveryContext.parseErrors >= 5 {
		if !s.recoveryContext.recoveryMode {
			s.recoveryContext.recoveryMode = true
			s.recoveryContext.recoveryStart = now
			s.logger.Printf("Entering recovery mode due to multiple errors")
		}
	}

	if s.recoveryContext.recoveryMode && now.Sub(s.recoveryContext.recoveryStart) > 60*time.Second {
		s.recoveryContext.recoveryMode = false
		s.recoveryContext.malformedCount = 0
		s.recoveryContext.parseErrors = 0
		s.logger.Printf("Exiting recovery mode")
	}
}

func (s *Server) attemptRecovery(reader *bufio.Reader, originalErr error) bool {
	s.logger.Printf("Attempting recovery from error: %v", originalErr)

	if s.isEOFError(originalErr) {
		s.logger.Println("EOF detected, no recovery possible")
		return false
	}

	buffer := make([]byte, 512)
	for i := 0; i < 5; i++ {
		select {
		case <-s.ctx.Done():
			s.logger.Println("Recovery cancelled due to context")
			return false
		default:
		}

		n, err := reader.Read(buffer)
		if err != nil {
			if err == io.EOF {
				s.logger.Printf("EOF encountered during recovery attempt %d", i+1)
				return false
			}
			s.logger.Printf("Recovery attempt %d failed: %v", i+1, err)
			return false
		}

		data := string(buffer[:n])
		if strings.Contains(data, ContentLengthHeader+":") {
			s.logger.Printf("Found potential message boundary after %d attempts", i+1)
			return true
		}

		time.Sleep(50 * time.Millisecond)
	}

	s.logger.Println("Recovery attempts exhausted")
	return false
}

func (s *Server) handleInitializeWithValidation(ctx context.Context, msg MCPMessage) error {
	var params InitializeParams
	if msg.Params != nil {
		paramBytes, _ := json.Marshal(msg.Params)
		if err := json.Unmarshal(paramBytes, &params); err != nil {
			return s.sendError(msg.ID, JSONRPCErrorCodeInvalidParams, JSONRPCErrorMessageInvalidParams, fmt.Sprintf("Failed to parse initialize params: %v", err))
		}
	}

	if params.ProtocolVersion != "" && !isCompatibleProtocolVersion(params.ProtocolVersion) {
		s.logger.Printf("Warning: Potentially incompatible protocol version: %s", params.ProtocolVersion)
	}

	result := InitializeResult{
		ProtocolVersion: ProtocolVersion,
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
		return s.sendError(msg.ID, JSONRPCErrorCodeServerNotInit, JSONRPCErrorMessageServerNotInit, ERROR_CALL_INITIALIZE_FIRST)
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
		return s.sendError(msg.ID, JSONRPCErrorCodeServerNotInit, JSONRPCErrorMessageServerNotInit, ERROR_CALL_INITIALIZE_FIRST)
	}

	var call ToolCall
	if msg.Params != nil {
		paramBytes, _ := json.Marshal(msg.Params)
		if err := json.Unmarshal(paramBytes, &call); err != nil {
			return s.sendError(msg.ID, JSONRPCErrorCodeInvalidParams, JSONRPCErrorMessageInvalidParams, fmt.Sprintf("Failed to parse tool call params: %v", err))
		}
	} else {
		return s.sendError(msg.ID, JSONRPCErrorCodeInvalidParams, JSONRPCErrorMessageInvalidParams, "Missing tool call parameters")
	}

	if call.Name == "" {
		return s.sendError(msg.ID, JSONRPCErrorCodeInvalidParams, JSONRPCErrorMessageInvalidParams, "Tool name is required")
	}

	result, err := s.toolHandler.CallTool(ctx, call)

	if err != nil {
		s.logger.Printf("Tool call failed: tool=%s, error=%v", call.Name, err)
		return s.sendError(msg.ID, JSONRPCErrorCodeInternalError, JSONRPCErrorMessageInternalError, err.Error())
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

func isCompatibleProtocolVersion(version string) bool {
	compatibleVersions := []string{
		ProtocolVersion,
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

func (s *Server) isEOFError(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := strings.ToLower(err.Error())
	return err == io.EOF || 
		err == io.ErrUnexpectedEOF ||
		strings.Contains(errStr, "eof") ||
		strings.Contains(errStr, "broken pipe") ||
		strings.Contains(errStr, "connection reset")
}

func (s *Server) shouldAttemptRecovery(err error) bool {
	if s.isEOFError(err) {
		return false
	}
	
	errStr := strings.ToLower(err.Error())
	
	nonRecoverableErrors := []string{
		"context canceled",
		"context deadline exceeded",
		"connection closed",
		"use of closed network connection",
		"broken pipe",
	}
	
	for _, nonRecoverable := range nonRecoverableErrors {
		if strings.Contains(errStr, nonRecoverable) {
			return false
		}
	}
	
	return true
}

func (s *Server) isConnectionError(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "broken pipe") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "connection closed") ||
		strings.Contains(errStr, "use of closed network connection")
}
