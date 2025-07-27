package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
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
	Config      *ServerConfig
	Client      *LSPGatewayClient
	ToolHandler *ToolHandler

	Input  io.Reader
	Output io.Writer

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	Initialized     bool
	Logger          *log.Logger
	ProtocolLimits  *ProtocolConstants
	RecoveryContext *RecoveryContext
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
		Config:          config,
		Client:          client,
		ToolHandler:     toolHandler,
		Input:           os.Stdin,
		Output:          os.Stdout,
		ctx:             ctx,
		cancel:          cancel,
		Initialized:     false,
		Logger:          log.New(os.Stderr, MCPLogPrefix, log.LstdFlags|log.Lshortfile),
		ProtocolLimits:  protocolLimits,
		RecoveryContext: &RecoveryContext{},
	}
}

// NewServerWithDirectLSP creates a new MCP server with DirectLSPManager instead of HTTP gateway client
func NewServerWithDirectLSP(config *ServerConfig, directLSPManager *DirectLSPManager) (*Server, error) {
	if config == nil {
		config = DefaultConfig()
	}

	if directLSPManager == nil {
		return nil, fmt.Errorf("DirectLSPManager cannot be nil")
	}

	// Create tool handler with DirectLSPManager instead of LSPGatewayClient
	toolHandler := NewToolHandlerWithDirectLSP(directLSPManager)

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
		Config:          config,
		Client:          nil, // No HTTP gateway client in DirectLSP mode
		ToolHandler:     toolHandler,
		Input:           os.Stdin,
		Output:          os.Stdout,
		ctx:             ctx,
		cancel:          cancel,
		Initialized:     false,
		Logger:          log.New(os.Stderr, MCPLogPrefix, log.LstdFlags|log.Lshortfile),
		ProtocolLimits:  protocolLimits,
		RecoveryContext: &RecoveryContext{},
	}, nil
}

func (s *Server) SetIO(input io.Reader, output io.Writer) {
	s.Input = input
	s.Output = output
}

func (s *Server) Start() error {
	s.Logger.Printf("Starting MCP server %s v%s", s.Config.Name, s.Config.Version)
	if s.Client != nil {
		s.Logger.Printf("LSP Gateway URL: %s", s.Config.LSPGatewayURL)
	} else {
		s.Logger.Printf("DirectLSP mode: Using direct LSP server connections")
	}

	// Start DirectLSP manager if present (detected by nil Client)
	if s.Client == nil && s.ToolHandler != nil {
		// Try to access DirectLSPManager through the tool handler
		if directLSPClient, ok := s.ToolHandler.Client.(*DirectLSPManager); ok {
			if err := directLSPClient.Start(s.ctx); err != nil {
				return fmt.Errorf("failed to start DirectLSP manager: %w", err)
			}
			s.Logger.Printf("DirectLSP manager started successfully")
		}
	}

	s.wg.Add(1)
	go s.messageLoop()

	s.wg.Wait()
	return nil
}

// Testing helper methods - public wrappers for internal methods
func (s *Server) IsConnectionError(err error) bool {
	// Basic connection error detection
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "connection") || strings.Contains(errStr, "EOF")
}

func (s *Server) ValidateOutgoingMessage(message *MCPMessage) error {
	// Basic message validation
	if message == nil {
		return errors.New("nil message")
	}
	if message.JSONRPC == "" {
		return errors.New("missing jsonrpc field")
	}
	return nil
}

func (s *Server) AnalyzeJSONError(err error, data string) string {
	// Basic JSON error analysis
	if err == nil {
		return "no error"
	}
	return fmt.Sprintf("JSON error: %v (data: %s)", err, data)
}

func (s *Server) IsEOFError(err error) bool {
	// EOF error detection
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "EOF")
}

func (s *Server) ShouldAttemptRecovery(err error) bool {
	// Basic recovery decision logic
	if err == nil {
		return false
	}
	// Don't recover from context cancellation
	if errors.Is(err, context.Canceled) {
		return false
	}
	return true
}

func (s *Server) Stop() error {
	s.Logger.Println("Stopping MCP server")
	
	// Stop DirectLSP manager if present
	if s.Client == nil && s.ToolHandler != nil {
		if directLSPClient, ok := s.ToolHandler.Client.(*DirectLSPManager); ok {
			if err := directLSPClient.Stop(); err != nil {
				s.Logger.Printf("Warning: Error stopping DirectLSP manager: %v", err)
			} else {
				s.Logger.Printf("DirectLSP manager stopped successfully")
			}
		}
	}
	
	s.cancel()
	s.wg.Wait()
	return nil
}

func (s *Server) messageLoop() {
	defer s.wg.Done()

	reader := bufio.NewReader(s.Input)
	consecutiveErrors := 0
	maxConsecutiveErrors := 10

	for {
		select {
		case <-s.ctx.Done():
			s.Logger.Println("Message loop terminated by context cancellation")
			return
		default:
		}

		msgCtx, msgCancel := context.WithTimeout(s.ctx, s.ProtocolLimits.MessageTimeout)

		message, err := s.ReadMessageWithRecovery(reader)
		if err != nil {
			msgCancel()

			if err == io.EOF {
				s.Logger.Println("Input stream closed gracefully")
				return
			}

			if s.isEOFError(err) {
				s.Logger.Println("EOF-related error detected, terminating gracefully")
				return
			}

			consecutiveErrors++
			s.Logger.Printf("Error reading message (attempt %d/%d): %v", consecutiveErrors, maxConsecutiveErrors, err)

			if s.shouldAttemptRecovery(err) && s.AttemptRecovery(reader, err) {
				consecutiveErrors = 0
				continue
			}

			if consecutiveErrors >= maxConsecutiveErrors {
				s.Logger.Printf("Too many consecutive errors (%d), terminating message loop", consecutiveErrors)
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

			if err := s.HandleMessageWithValidation(ctx, msg); err != nil {
				s.Logger.Printf("Error handling message: %v", err)
				_ = s.sendGenericError("Message handling failed: " + err.Error())
			}
		}(msgCtx, message)
	}
}

func (s *Server) ReadMessageWithRecovery(reader *bufio.Reader) (string, error) {
	var contentLength int
	headerLines := 0
	headerSize := 0

	for {
		headerLines++
		if headerLines > s.ProtocolLimits.MaxHeaderLines {
			return "", fmt.Errorf("too many header lines (max %d)", s.ProtocolLimits.MaxHeaderLines)
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return "", err
			}
			return "", fmt.Errorf("failed to read header line: %w", err)
		}

		headerSize += len(line)
		if headerSize > s.ProtocolLimits.MaxHeaderSize {
			return "", fmt.Errorf("header size too large (max %d bytes)", s.ProtocolLimits.MaxHeaderSize)
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

			if length > s.ProtocolLimits.MaxMessageSize {
				return "", fmt.Errorf("message size too large: %d bytes (max %d)", length, s.ProtocolLimits.MaxMessageSize)
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

func (s *Server) HandleMessageWithValidation(ctx context.Context, data string) error {
	if len(data) == 0 {
		return s.SendError(nil, JSONRPCErrorCodeParseError, JSONRPCErrorMessageParseError, "Empty message content")
	}

	if len(data) > s.ProtocolLimits.MaxMessageSize {
		return s.SendError(nil, JSONRPCErrorCodeParseError, JSONRPCErrorMessageParseError, fmt.Sprintf("Message too large: %d bytes", len(data)))
	}

	var msg MCPMessage
	if err := json.Unmarshal([]byte(data), &msg); err != nil {
		s.UpdateRecoveryContext(MCPRecoveryContextParseError)

		errorDetails := s.analyzeJSONError(err, data)
		s.Logger.Printf("JSON parse error: %v", errorDetails)

		return s.SendError(nil, JSONRPCErrorCodeParseError, JSONRPCErrorMessageParseError, errorDetails)
	}

	if err := s.ValidateMessageStructure(&msg); err != nil {
		s.Logger.Printf("Message validation error: %v", err)
		return s.SendError(msg.ID, JSONRPCErrorCodeInvalidRequest, JSONRPCErrorMessageInvalidRequest, err.Error())
	}

	s.Logger.Printf("Processing message: method=%s, id=%v", msg.Method, msg.ID)

	handlerCtx, cancel := context.WithTimeout(ctx, s.ProtocolLimits.MessageTimeout)
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
		s.Logger.Println("Received initialization notification")
		return nil
	default:
		s.Logger.Printf("Unknown method requested: %s", msg.Method)
		if msg.ID != nil {
			return s.SendError(msg.ID, JSONRPCErrorCodeMethodNotFound, JSONRPCErrorMessageMethodNotFound, fmt.Sprintf("Unknown method: %s", msg.Method))
		}
		return nil
	}
}

func (s *Server) SendResponse(id interface{}, result interface{}) error {
	response := MCPMessage{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Result:  result,
	}
	return s.SendMessage(response)
}

func (s *Server) SendError(id interface{}, code int, message, data string) error {
	enhancedData := data
	if s.RecoveryContext.recoveryMode {
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

	s.Logger.Printf("Sending error response: code=%d, message=%s, id=%v", code, message, id)
	return s.SendMessage(response)
}

func (s *Server) sendGenericError(errorMsg string) error {
	return s.SendError(nil, JSONRPCErrorCodeInternalError, JSONRPCErrorMessageInternalError, errorMsg)
}

func (s *Server) SendMessage(msg MCPMessage) error {
	if err := s.validateOutgoingMessage(&msg); err != nil {
		s.Logger.Printf("Outgoing message validation failed: %v", err)
		return fmt.Errorf("invalid outgoing message: %w", err)
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	if len(data) > s.ProtocolLimits.MaxMessageSize {
		return fmt.Errorf("outgoing message too large: %d bytes (max %d)", len(data), s.ProtocolLimits.MaxMessageSize)
	}

	content := string(data)
	header := ContentLengthHeader + ": " + strconv.Itoa(len(content)) + "\r\n\r\n"

	select {
	case <-s.ctx.Done():
		return fmt.Errorf("context cancelled while writing message")
	default:
	}

	if _, err := s.Output.Write([]byte(header)); err != nil {
		if s.isConnectionError(err) {
			s.Logger.Printf("Connection closed while writing header: %v", err)
			return fmt.Errorf("connection closed: %w", err)
		}
		s.Logger.Printf("Failed to write response header: %v", err)
		return fmt.Errorf("failed to write header: %w", err)
	}

	if _, err := s.Output.Write([]byte(content)); err != nil {
		if s.isConnectionError(err) {
			s.Logger.Printf("Connection closed while writing content: %v", err)
			return fmt.Errorf("connection closed: %w", err)
		}
		s.Logger.Printf("Failed to write response content: %v", err)
		return fmt.Errorf("failed to write content: %w", err)
	}

	s.Logger.Printf("Sent message: id=%v, size=%d bytes", msg.ID, len(content))
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

func (s *Server) ValidateMessageStructure(msg *MCPMessage) error {
	if msg.JSONRPC != JSONRPCVersion {
		return &MessageValidationError{
			Field:   MCPValidationFieldJSONRPC,
			Reason:  MCPValidationReasonInvalidVersion,
			Value:   msg.JSONRPC,
			Message: fmt.Sprintf("Invalid JSON-RPC version: %s (expected %s)", msg.JSONRPC, JSONRPCVersion),
		}
	}

	if msg.Method != "" {
		if len(msg.Method) > s.ProtocolLimits.MaxMethodLength {
			return &MessageValidationError{
				Field:   MCPValidationFieldMethod,
				Reason:  MCPValidationReasonTooLong,
				Value:   len(msg.Method),
				Message: fmt.Sprintf("Method name too long: %d chars (max %d)", len(msg.Method), s.ProtocolLimits.MaxMethodLength),
			}
		}

		if !IsValidMethodName(msg.Method) {
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
		paramsBytes, err := json.Marshal(msg.Params)
		if err != nil {
			return &MessageValidationError{
				Field:   MCPValidationFieldParams,
				Reason:  MCPValidationReasonInvalidFormat,
				Value:   "unmarshalable",
				Message: fmt.Sprintf("Failed to marshal params: %v", err),
			}
		}
		if len(paramsBytes) > s.ProtocolLimits.MaxParamsSize {
			return &MessageValidationError{
				Field:   MCPValidationFieldParams,
				Reason:  MCPValidationReasonTooLarge,
				Value:   len(paramsBytes),
				Message: fmt.Sprintf("Params too large: %d bytes (max %d)", len(paramsBytes), s.ProtocolLimits.MaxParamsSize),
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

func IsValidMethodName(method string) bool {
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

func (s *Server) UpdateRecoveryContext(errorType string) {
	s.RecoveryContext.mu.Lock()
	defer s.RecoveryContext.mu.Unlock()

	now := time.Now()

	switch errorType {
	case MCPRecoveryContextMalformed:
		s.RecoveryContext.malformedCount++
		s.RecoveryContext.lastMalformed = now
	case MCPRecoveryContextParseError:
		s.RecoveryContext.parseErrors++
		s.RecoveryContext.lastParseError = now
	}

	if s.RecoveryContext.malformedCount >= 3 || s.RecoveryContext.parseErrors >= 5 {
		if !s.RecoveryContext.recoveryMode {
			s.RecoveryContext.recoveryMode = true
			s.RecoveryContext.recoveryStart = now
			s.Logger.Printf("Entering recovery mode due to multiple errors")
		}
	}

	if s.RecoveryContext.recoveryMode && now.Sub(s.RecoveryContext.recoveryStart) > 60*time.Second {
		s.RecoveryContext.recoveryMode = false
		s.RecoveryContext.malformedCount = 0
		s.RecoveryContext.parseErrors = 0
		s.Logger.Printf("Exiting recovery mode")
	}
}

func (s *Server) AttemptRecovery(reader *bufio.Reader, originalErr error) bool {
	s.Logger.Printf("Attempting recovery from error: %v", originalErr)

	if s.isEOFError(originalErr) {
		s.Logger.Println("EOF detected, no recovery possible")
		return false
	}

	buffer := make([]byte, 512)
	for i := 0; i < 5; i++ {
		select {
		case <-s.ctx.Done():
			s.Logger.Println("Recovery cancelled due to context")
			return false
		default:
		}

		n, err := reader.Read(buffer)
		if err != nil {
			if err == io.EOF {
				s.Logger.Printf("EOF encountered during recovery attempt %d", i+1)
				return false
			}
			s.Logger.Printf("Recovery attempt %d failed: %v", i+1, err)
			return false
		}

		data := string(buffer[:n])
		if strings.Contains(data, ContentLengthHeader+":") {
			s.Logger.Printf("Found potential message boundary after %d attempts", i+1)
			return true
		}

		time.Sleep(50 * time.Millisecond)
	}

	s.Logger.Println("Recovery attempts exhausted")
	return false
}

func (s *Server) handleInitializeWithValidation(ctx context.Context, msg MCPMessage) error {
	var params InitializeParams
	if msg.Params != nil {
		paramBytes, err := json.Marshal(msg.Params)
		if err != nil {
			return s.SendError(msg.ID, JSONRPCErrorCodeInternalError, JSONRPCErrorMessageInternalError, fmt.Sprintf("Failed to marshal initialize params: %v", err))
		}
		if err := json.Unmarshal(paramBytes, &params); err != nil {
			return s.SendError(msg.ID, JSONRPCErrorCodeInvalidParams, JSONRPCErrorMessageInvalidParams, fmt.Sprintf("Failed to parse initialize params: %v", err))
		}
	}

	if params.ProtocolVersion != "" && !IsCompatibleProtocolVersion(params.ProtocolVersion) {
		s.Logger.Printf("Warning: Potentially incompatible protocol version: %s", params.ProtocolVersion)
	}

	result := InitializeResult{
		ProtocolVersion: ProtocolVersion,
		Capabilities: map[string]interface{}{
			"tools": map[string]interface{}{
				"listChanged": false,
			},
		},
		ServerInfo: map[string]interface{}{
			"name":    s.Config.Name,
			"version": s.Config.Version,
		},
	}

	s.Initialized = true
	s.Logger.Printf("MCP server initialized with client protocol version: %s", params.ProtocolVersion)

	return s.SendResponse(msg.ID, result)
}

func (s *Server) handleListToolsWithValidation(ctx context.Context, msg MCPMessage) error {
	if !s.Initialized {
		return s.SendError(msg.ID, JSONRPCErrorCodeServerNotInit, JSONRPCErrorMessageServerNotInit, ERROR_CALL_INITIALIZE_FIRST)
	}

	tools := s.ToolHandler.ListTools()
	result := map[string]interface{}{
		"tools": tools,
	}

	s.Logger.Printf("Listed %d tools", len(tools))
	return s.SendResponse(msg.ID, result)
}

func (s *Server) handleCallToolWithValidation(ctx context.Context, msg MCPMessage) error {
	if !s.Initialized {
		return s.SendError(msg.ID, JSONRPCErrorCodeServerNotInit, JSONRPCErrorMessageServerNotInit, ERROR_CALL_INITIALIZE_FIRST)
	}

	var call ToolCall
	if msg.Params != nil {
		paramBytes, err := json.Marshal(msg.Params)
		if err != nil {
			return s.SendError(msg.ID, JSONRPCErrorCodeInternalError, JSONRPCErrorMessageInternalError, fmt.Sprintf("Failed to marshal tool call params: %v", err))
		}
		if err := json.Unmarshal(paramBytes, &call); err != nil {
			return s.SendError(msg.ID, JSONRPCErrorCodeInvalidParams, JSONRPCErrorMessageInvalidParams, fmt.Sprintf("Failed to parse tool call params: %v", err))
		}
	} else {
		return s.SendError(msg.ID, JSONRPCErrorCodeInvalidParams, JSONRPCErrorMessageInvalidParams, "Missing tool call parameters")
	}

	if call.Name == "" {
		return s.SendError(msg.ID, JSONRPCErrorCodeInvalidParams, JSONRPCErrorMessageInvalidParams, "Tool name is required")
	}

	result, err := s.ToolHandler.CallTool(ctx, call)

	if err != nil {
		s.Logger.Printf("Tool call failed: tool=%s, error=%v", call.Name, err)
		return s.SendError(msg.ID, JSONRPCErrorCodeInternalError, JSONRPCErrorMessageInternalError, err.Error())
	}

	s.Logger.Printf("Tool call completed: tool=%s, success=%t", call.Name, !result.IsError)
	return s.SendResponse(msg.ID, result)
}

func (s *Server) handlePingWithValidation(ctx context.Context, msg MCPMessage) error {
	result := map[string]interface{}{
		"pong": time.Now().Unix(),
	}
	return s.SendResponse(msg.ID, result)
}

func IsCompatibleProtocolVersion(version string) bool {
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
	s.RecoveryContext.mu.RLock()
	defer s.RecoveryContext.mu.RUnlock()

	return map[string]interface{}{
		"malformed_count":  s.RecoveryContext.malformedCount,
		"parse_errors":     s.RecoveryContext.parseErrors,
		"recovery_mode":    s.RecoveryContext.recoveryMode,
		"last_malformed":   s.RecoveryContext.lastMalformed,
		"last_parse_error": s.RecoveryContext.lastParseError,
		"recovery_start":   s.RecoveryContext.recoveryStart,
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
