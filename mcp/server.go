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
	DefaultProtocolVersion     = "2025-06-18"
	MCPLogPrefix               = "[MCP] "
	ContentTypeApplicationJSON = "application/json"
	ContentLengthHeader        = "Content-Length"
	ContentTypeHeader          = "Content-Type"
)

type InitializationState int

const (
	NotInitialized InitializationState = iota
	Initializing
	Initialized
	Failed
)

func (s InitializationState) String() string {
	switch s {
	case NotInitialized:
		return "NotInitialized"
	case Initializing:
		return "Initializing"
	case Initialized:
		return "Initialized"
	case Failed:
		return "Failed"
	default:
		return "Unknown"
	}
}

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
	JSONRPCErrorCodeUnsupportedVersion = -32001
	JSONRPCErrorMessageUnsupportedVersion = "Unsupported protocol version"
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

// ProtocolFormat represents the detected message format
type ProtocolFormat int

const (
	ProtocolFormatUnknown ProtocolFormat = iota
	ProtocolFormatJSON    // Direct JSON (Claude Code format)
	ProtocolFormatLSP     // LSP Content-Length format
)

func (pf ProtocolFormat) String() string {
	switch pf {
	case ProtocolFormatJSON:
		return "JSON"
	case ProtocolFormatLSP:
		return "LSP"
	default:
		return "Unknown"
	}
}

// ConnectionContext holds per-connection protocol state
type ConnectionContext struct {
	mu             sync.RWMutex
	protocolFormat ProtocolFormat
	detected       bool
	buffer         []byte
}

// SetProtocolFormat safely sets the protocol format for this connection
func (cc *ConnectionContext) SetProtocolFormat(format ProtocolFormat) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	cc.protocolFormat = format
	cc.detected = true
}

// GetProtocolFormat safely gets the protocol format for this connection
func (cc *ConnectionContext) GetProtocolFormat() (ProtocolFormat, bool) {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	return cc.protocolFormat, cc.detected
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

	initStateMu sync.RWMutex
	initState   InitializationState

	// Initialization timeout and recovery
	initTimeout      time.Duration
	initDeadline     time.Time
	autoRecovery     bool
	recoveryAttempts int

	Logger          *log.Logger
	ProtocolLimits  *ProtocolConstants
	RecoveryContext *RecoveryContext
	
	// Per-connection protocol context (thread-safe)
	connContext *ConnectionContext
	
	// Track negotiated protocol version
	negotiatedVersionMu sync.RWMutex
	negotiatedVersion   string
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
		initState:       NotInitialized,
		Logger:          log.New(os.Stderr, MCPLogPrefix, log.LstdFlags|log.Lshortfile),
		ProtocolLimits:  protocolLimits,
		RecoveryContext: &RecoveryContext{},
		connContext:     &ConnectionContext{},
		// Initialize timeout and recovery settings
		initTimeout:  30 * time.Second,
		autoRecovery: true,
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
		initState:       NotInitialized,
		Logger:          log.New(os.Stderr, MCPLogPrefix, log.LstdFlags|log.Lshortfile),
		ProtocolLimits:  protocolLimits,
		RecoveryContext: &RecoveryContext{},
		connContext:     &ConnectionContext{},
		// Initialize timeout and recovery settings
		initTimeout:  30 * time.Second,
		autoRecovery: true,
	}, nil
}

func (s *Server) SetIO(input io.Reader, output io.Writer) {
	s.Input = input
	s.Output = output
}

func (s *Server) Start() error {
	// Debug: Log working directory and process info
	if cwd, err := os.Getwd(); err == nil {
		s.Logger.Printf("[DEBUG] MCP Server Working Directory: %s", cwd)
	}
	s.Logger.Printf("[DEBUG] Process Args: %v", os.Args)
	s.Logger.Printf("[DEBUG] Environment Variables: CLAUDE_PROJECT_ROOT=%s", os.Getenv("CLAUDE_PROJECT_ROOT"))
	
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

func (s *Server) GetInitializationState() InitializationState {
	s.initStateMu.RLock()
	defer s.initStateMu.RUnlock()
	return s.initState
}

func (s *Server) SetInitializationState(newState InitializationState) error {
	s.initStateMu.Lock()
	defer s.initStateMu.Unlock()

	if err := s.validateInitializationStateTransition(s.initState, newState); err != nil {
		return err
	}

	oldState := s.initState
	s.initState = newState
	
	// Reset negotiated version when transitioning to NotInitialized
	if newState == NotInitialized {
		s.setNegotiatedVersion("")
		s.Logger.Printf("Protocol version negotiation reset due to state transition: %s -> %s", oldState, newState)
	}
	
	s.logInitializationStateChange(oldState, newState, "")
	return nil
}

func (s *Server) IsInitialized() bool {
	return s.GetInitializationState() == Initialized
}

func (s *Server) isValidStateTransition(from, to InitializationState) bool {
	switch from {
	case NotInitialized:
		return to == Initializing || to == Failed
	case Initializing:
		return to == Initialized || to == Failed
	case Initialized:
		return to == NotInitialized
	case Failed:
		return to == NotInitialized || to == Initializing
	default:
		return false
	}
}

// sendNotInitializedError sends a standardized error response for requests made before initialization
func (s *Server) sendNotInitializedError(id interface{}) error {
	return s.SendError(id, JSONRPCErrorCodeServerNotInit, JSONRPCErrorMessageServerNotInit, ERROR_CALL_INITIALIZE_FIRST)
}

// sendInitializingError sends a standardized error response for requests made during initialization
func (s *Server) sendInitializingError(id interface{}) error {
	return s.SendError(id, JSONRPCErrorCodeInvalidRequest, JSONRPCErrorMessageInvalidRequest, "Server is currently initializing. Please wait for initialization to complete.")
}

// sendInitializationFailedError sends a standardized error response when initialization has failed
func (s *Server) sendInitializationFailedError(id interface{}) error {
	return s.SendError(id, JSONRPCErrorCodeInternalError, JSONRPCErrorMessageInternalError, "Server initialization failed. Please reinitialize the server.")
}

// validateInitializationStateTransition validates and logs a potential state transition
func (s *Server) validateInitializationStateTransition(from, to InitializationState) error {
	if !s.isValidStateTransition(from, to) {
		s.Logger.Printf("Invalid initialization state transition attempted: %s -> %s", from, to)
		return fmt.Errorf("invalid state transition from %s to %s", from, to)
	}
	return nil
}

// logInitializationStateChange logs initialization state changes with consistent formatting
func (s *Server) logInitializationStateChange(from, to InitializationState, context string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	if context != "" {
		s.Logger.Printf("[%s] Initialization state changed: %s -> %s (%s)", timestamp, from, to, context)
	} else {
		s.Logger.Printf("[%s] Initialization state changed: %s -> %s", timestamp, from, to)
	}
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
	
	// Reset negotiated version
	s.setNegotiatedVersion("")
	s.Logger.Printf("Protocol version negotiation reset on server stop")
	
	s.initStateMu.Lock()
	oldState := s.initState
	s.initState = NotInitialized
	s.initStateMu.Unlock()
	s.logInitializationStateChange(oldState, NotInitialized, "server stopping")
	s.cancel()
	s.wg.Wait()
	return nil
}

// checkInitializationTimeout checks if initialization has exceeded the timeout
func (s *Server) checkInitializationTimeout() bool {
	if s.initTimeout <= 0 {
		return false // No timeout set
	}
	
	s.initStateMu.RLock()
	state := s.initState
	deadline := s.initDeadline
	s.initStateMu.RUnlock()
	
	if state == Initializing && !deadline.IsZero() && time.Now().After(deadline) {
		s.Logger.Printf("Initialization timeout exceeded (%v), attempting recovery", s.initTimeout)
		return true
	}
	return false
}

// attemptInitializationRecovery attempts to recover from initialization timeout
func (s *Server) attemptInitializationRecovery() bool {
	if !s.autoRecovery {
		return false
	}
	
	s.initStateMu.Lock()
	defer s.initStateMu.Unlock()
	
	s.recoveryAttempts++
	s.Logger.Printf("Attempting initialization recovery (attempt %d)", s.recoveryAttempts)
	
	if s.recoveryAttempts > 3 {
		s.Logger.Printf("Maximum recovery attempts exceeded, marking as failed")
		s.initState = Failed
		return false
	}
	
	// Reset initialization with extended timeout
	s.initState = NotInitialized
	s.initDeadline = time.Time{} // Clear deadline
	s.Logger.Printf("Initialization recovery attempt %d: reset to NotInitialized", s.recoveryAttempts)
	return true
}

// isInitializationStable checks if the server is in a stable state for operations
func (s *Server) isInitializationStable() bool {
	s.initStateMu.RLock()
	defer s.initStateMu.RUnlock()
	
	// Consider initialization "stable enough" for tool operations if:
	// 1. Fully initialized, OR
	// 2. Currently initializing but tools are available (DirectLSP mode), OR  
	// 3. Failed but auto-recovery is enabled
	switch s.initState {
	case Initialized:
		return true
	case Initializing:
		// In DirectLSP mode, tools are available during initialization
		return s.Client == nil
	case Failed:
		return s.autoRecovery
	default:
		return false
	}
}

func (s *Server) messageLoop() {
	defer s.wg.Done()

	reader := bufio.NewReader(s.Input)
	consecutiveErrors := 0
	maxConsecutiveErrors := 10
	messagesProcessed := 0
	consecutiveEOFs := 0

	for {
		select {
		case <-s.ctx.Done():
			s.Logger.Println("Message loop terminated by context cancellation")
			return
		default:
		}

		// Check for initialization timeout and attempt recovery
		if s.checkInitializationTimeout() {
			if !s.attemptInitializationRecovery() {
				s.Logger.Println("Initialization recovery failed, continuing with limited functionality")
			}
		}

		msgCtx, msgCancel := context.WithTimeout(s.ctx, s.ProtocolLimits.MessageTimeout)

		message, err := s.ReadMessageWithRecovery(msgCtx, reader)
		if err != nil {
			msgCancel()

			if err == io.EOF {
				// If we've processed at least one message, exit gracefully
				if messagesProcessed > 0 {
					s.Logger.Println("Input stream closed gracefully")
					return
				}
				// If no messages processed yet, just log and continue
				// This handles the case where stdin is closed but the client expects us to stay alive
				if consecutiveEOFs == 0 {
					s.Logger.Println("EOF detected but no messages processed yet, continuing to run...")
				}
				consecutiveEOFs++
				// Sleep longer to avoid busy-waiting
				time.Sleep(1 * time.Second)
				continue
			}

			if s.isEOFError(err) {
				s.Logger.Println("EOF-related error detected, terminating gracefully")
				return
			}

			consecutiveErrors++
			s.Logger.Printf("Error reading message (attempt %d/%d): %v", consecutiveErrors, maxConsecutiveErrors, err)

			if s.shouldAttemptRecovery(err) && s.AttemptRecovery(msgCtx, reader, err) {
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
		consecutiveEOFs = 0
		messagesProcessed++

		go func(ctx context.Context, msg string) {
			defer msgCancel()

			if err := s.HandleMessageWithValidation(ctx, msg); err != nil {
				s.Logger.Printf("Error handling message: %v", err)
				_ = s.sendGenericError("Message handling failed: " + err.Error())
			}
		}(msgCtx, message)
	}
}

func (s *Server) ReadMessageWithRecovery(ctx context.Context, reader *bufio.Reader) (string, error) {
	return s.readMessageWithProtocolDetection(ctx, reader)
}

// readMessageWithProtocolDetection implements robust protocol detection with state machine
func (s *Server) readMessageWithProtocolDetection(ctx context.Context, reader *bufio.Reader) (string, error) {
	s.Logger.Printf("[DEBUG] readMessageWithProtocolDetection called")
	
	// Check if protocol is already detected for this connection
	if format, detected := s.connContext.GetProtocolFormat(); detected {
		s.Logger.Printf("[DEBUG] Protocol already detected: %v", format)
		switch format {
		case ProtocolFormatJSON:
			s.Logger.Printf("[DEBUG] Reading JSON message")
			return s.readJSONMessage(ctx, reader)
		case ProtocolFormatLSP:
			s.Logger.Printf("[DEBUG] Reading LSP message")
			return s.readLSPMessage(ctx, reader)
		default:
			s.Logger.Printf("[ERROR] Invalid protocol format: %v", format)
			return "", fmt.Errorf("invalid protocol format: %v", format)
		}
	}

	// Protocol not detected yet - perform detection with timeout
	s.Logger.Printf("[DEBUG] Protocol not detected yet, performing detection with 5s timeout")
	detectionCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result, err := s.detectAndReadMessage(detectionCtx, reader)
	if err != nil {
		s.Logger.Printf("[DEBUG] detectAndReadMessage failed: %v", err)
	} else {
		s.Logger.Printf("[DEBUG] detectAndReadMessage succeeded, result length: %d", len(result))
	}
	return result, err
}

// detectAndReadMessage performs protocol detection and reads the first message
func (s *Server) detectAndReadMessage(ctx context.Context, reader *bufio.Reader) (string, error) {
	s.Logger.Printf("[DEBUG] detectAndReadMessage called")
	
	// Buffer for protocol detection - peek more bytes for reliable detection
	peekSize := 32
	s.Logger.Printf("[DEBUG] Attempting to peek %d bytes for protocol detection", peekSize)
	
	headerBuffer, err := s.peekWithTimeout(ctx, reader, peekSize)
	if err != nil {
		s.Logger.Printf("[DEBUG] peekWithTimeout failed: %v", err)
		if err == io.EOF {
			s.Logger.Printf("[DEBUG] EOF encountered during peek")
			return "", err
		}
		s.Logger.Printf("[ERROR] Failed to peek for protocol detection: %v", err)
		return "", fmt.Errorf("failed to peek for protocol detection: %w", err)
	}
	
	s.Logger.Printf("[DEBUG] Successfully peeked %d bytes: %q", len(headerBuffer), string(headerBuffer))

	// Detect protocol format based on header content
	format, err := s.detectProtocolFormat(headerBuffer)
	if err != nil {
		return "", fmt.Errorf("protocol detection failed: %w", err)
	}

	// Set detected protocol format for this connection
	s.connContext.SetProtocolFormat(format)
	s.Logger.Printf("Detected protocol format: %s", format)

	// Read message using detected format
	switch format {
	case ProtocolFormatJSON:
		return s.readJSONMessage(ctx, reader)
	case ProtocolFormatLSP:
		return s.readLSPMessage(ctx, reader)
	default:
		return "", fmt.Errorf("unsupported protocol format: %v", format)
	}
}

// peekWithTimeout safely peeks data with context timeout
func (s *Server) peekWithTimeout(ctx context.Context, reader *bufio.Reader, size int) ([]byte, error) {
	type result struct {
		data []byte
		err  error
	}

	ch := make(chan result, 1)
	go func() {
		data, err := reader.Peek(size)
		ch <- result{data: data, err: err}
	}()

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("peek timeout: %w", ctx.Err())
	case res := <-ch:
		return res.data, res.err
	}
}

// detectProtocolFormat analyzes header data to determine protocol format
func (s *Server) detectProtocolFormat(headerData []byte) (ProtocolFormat, error) {
	if len(headerData) == 0 {
		return ProtocolFormatUnknown, fmt.Errorf("no data available for protocol detection")
	}

	// Convert to string for analysis
	headerStr := string(headerData)
	
	// Check for JSON format: starts with '{' and contains valid JSON structure
	if headerData[0] == '{' {
		// Additional validation: check if it looks like valid JSON start
		if strings.Contains(headerStr[:min(len(headerStr), 20)], "\"jsonrpc\"") ||
		   strings.Contains(headerStr[:min(len(headerStr), 20)], "\"method\"") ||
		   strings.Contains(headerStr[:min(len(headerStr), 20)], "\"id\"") {
			return ProtocolFormatJSON, nil
		}
	}

	// Check for LSP Content-Length format
	headerLower := strings.ToLower(headerStr)
	if strings.HasPrefix(headerLower, "content-length:") {
		return ProtocolFormatLSP, nil
	}

	// Look for Content-Length anywhere in the first few bytes (case insensitive)
	if strings.Contains(headerLower, "content-length:") {
		return ProtocolFormatLSP, nil
	}

	// If starts with printable ASCII and no clear JSON markers, likely LSP
	if headerData[0] >= 32 && headerData[0] <= 126 && headerData[0] != '{' {
		// Check if it looks like HTTP-style headers
		if strings.Contains(headerStr, ":") {
			return ProtocolFormatLSP, nil
		}
	}

	// Default to JSON if data starts with '{' but no clear validation
	if headerData[0] == '{' {
		return ProtocolFormatJSON, nil
	}

	return ProtocolFormatUnknown, fmt.Errorf("unable to detect protocol format from header: %q", 
		string(headerData[:min(len(headerData), 16)]))
}

// readJSONMessage reads a direct JSON message (Claude Code format)
func (s *Server) readJSONMessage(ctx context.Context, reader *bufio.Reader) (string, error) {
	s.Logger.Printf("[DEBUG] readJSONMessage called")
	
	line, err := s.readLineWithContext(ctx, reader)
	if err != nil {
		s.Logger.Printf("[ERROR] Failed to read JSON line: %v", err)
		return "", fmt.Errorf("failed to read JSON line: %w", err)
	}

	s.Logger.Printf("[DEBUG] Raw line read: %d bytes, content: %q", len(line), line)
	
	line = strings.TrimSpace(line)
	if !utf8.ValidString(line) {
		s.Logger.Printf("[ERROR] Invalid UTF-8 encoding in JSON message")
		return "", fmt.Errorf("invalid UTF-8 encoding in JSON message")
	}

	if len(line) > s.ProtocolLimits.MaxMessageSize {
		s.Logger.Printf("[ERROR] Message size too large: %d bytes (max %d)", len(line), s.ProtocolLimits.MaxMessageSize)
		return "", fmt.Errorf("message size too large: %d bytes (max %d)", len(line), s.ProtocolLimits.MaxMessageSize)
	}

	s.Logger.Printf("[DEBUG] Received direct JSON message: %d bytes, content: %s", len(line), line)
	return line, nil
}

// readLSPMessage reads an LSP Content-Length format message
func (s *Server) readLSPMessage(ctx context.Context, reader *bufio.Reader) (string, error) {
	var contentLength int
	headerLines := 0
	headerSize := 0

	// Read headers
	for {
		headerLines++
		if headerLines > s.ProtocolLimits.MaxHeaderLines {
			return "", fmt.Errorf("too many header lines (max %d)", s.ProtocolLimits.MaxHeaderLines)
		}

		line, err := s.readLineWithContext(ctx, reader)
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

	// Read message content
	content := make([]byte, contentLength)
	n, err := s.readFullWithContext(ctx, reader, content)
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

	s.Logger.Printf("Received LSP Content-Length message: %d bytes", len(contentStr))
	return contentStr, nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (s *Server) HandleMessageWithValidation(ctx context.Context, data string) error {
	s.Logger.Printf("[DEBUG] HandleMessageWithValidation called with data length: %d", len(data))
	s.Logger.Printf("[DEBUG] Message content: %s", data)
	
	if len(data) == 0 {
		s.Logger.Printf("[ERROR] Empty message received")
		return s.SendError(nil, JSONRPCErrorCodeParseError, JSONRPCErrorMessageParseError, "Empty message content")
	}

	if len(data) > s.ProtocolLimits.MaxMessageSize {
		s.Logger.Printf("[ERROR] Message too large: %d bytes", len(data))
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

	s.Logger.Printf("[DEBUG] Processing message: method=%s, id=%v", msg.Method, msg.ID)

	handlerCtx, cancel := context.WithTimeout(ctx, s.ProtocolLimits.MessageTimeout)
	defer cancel()

	switch msg.Method {
	case MCPMethodInitialize:
		s.Logger.Printf("[DEBUG] Handling initialize method")
		return s.handleInitializeWithValidation(handlerCtx, msg)
	case MCPMethodToolsList:
		s.Logger.Printf("[DEBUG] Handling tools/list method")
		return s.handleListToolsWithValidation(handlerCtx, msg)
	case MCPMethodToolsCall:
		s.Logger.Printf("[DEBUG] Handling tools/call method")
		return s.handleCallToolWithValidation(handlerCtx, msg)
	case MCPMethodPing:
		s.Logger.Printf("[DEBUG] Handling ping method")
		return s.handlePingWithValidation(handlerCtx, msg)
	case MCPMethodNotificationInit:
		s.Logger.Printf("[DEBUG] Received initialization notification")
		return nil
	default:
		s.Logger.Printf("[ERROR] Unknown method requested: %s", msg.Method)
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

	select {
	case <-s.ctx.Done():
		return fmt.Errorf("context cancelled while writing message")
	default:
	}

	// Use per-connection protocol format for response formatting
	format, detected := s.connContext.GetProtocolFormat()
	if !detected {
		// Default to LSP format if protocol not yet detected
		format = ProtocolFormatLSP
		s.Logger.Printf("Warning: Protocol format not detected, defaulting to LSP format")
	}

	switch format {
	case ProtocolFormatJSON:
		// Direct JSON (Claude Code format) - send JSON directly with newline
		contentWithNewline := content + "\n"
		if _, err := s.Output.Write([]byte(contentWithNewline)); err != nil {
			if s.isConnectionError(err) {
				s.Logger.Printf("Connection closed while writing JSON: %v", err)
				return fmt.Errorf("connection closed: %w", err)
			}
			s.Logger.Printf("Failed to write JSON response: %v", err)
			return fmt.Errorf("failed to write JSON: %w", err)
		}
		s.Logger.Printf("Sent JSON format message: id=%v, size=%d bytes", msg.ID, len(content))
		s.Logger.Printf("[DEBUG] JSON Response sent: %s", content)
		
	case ProtocolFormatLSP:
		// Standard LSP protocol with Content-Length headers
		header := ContentLengthHeader + ": " + strconv.Itoa(len(content)) + "\r\n\r\n"

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
		s.Logger.Printf("Sent LSP format message: id=%v, size=%d bytes", msg.ID, len(content))
		s.Logger.Printf("[DEBUG] LSP Response sent: %s", content)
		
	default:
		return fmt.Errorf("unsupported protocol format for sending: %v", format)
	}

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

func (s *Server) AttemptRecovery(ctx context.Context, reader *bufio.Reader, originalErr error) bool {
	s.Logger.Printf("Attempting recovery from error: %v", originalErr)

	if s.isEOFError(originalErr) {
		s.Logger.Println("EOF detected, no recovery possible")
		return false
	}

	buffer := make([]byte, 512)
	for i := 0; i < 5; i++ {
		select {
		case <-ctx.Done():
			s.Logger.Println("Recovery cancelled due to context")
			return false
		default:
		}

		n, err := s.readBufferWithContext(ctx, reader, buffer)
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
	currentState := s.GetInitializationState()
	if currentState == Initialized {
		return s.SendError(msg.ID, JSONRPCErrorCodeInvalidRequest, JSONRPCErrorMessageInvalidRequest, "Server already initialized")
	}
	if currentState == Initializing {
		return s.SendError(msg.ID, JSONRPCErrorCodeInvalidRequest, JSONRPCErrorMessageInvalidRequest, "Initialization already in progress")
	}

	s.initStateMu.Lock()
	if err := s.validateInitializationStateTransition(s.initState, Initializing); err != nil {
		s.initStateMu.Unlock()
		s.Logger.Printf("Failed to set initializing state: %v", err)
		return s.SendError(msg.ID, JSONRPCErrorCodeInternalError, JSONRPCErrorMessageInternalError, "Failed to update initialization state")
	}
	oldState := s.initState
	s.initState = Initializing
	// Set initialization deadline for timeout tracking
	if s.initTimeout > 0 {
		s.initDeadline = time.Now().Add(s.initTimeout)
	}
	s.initStateMu.Unlock()
	s.logInitializationStateChange(oldState, Initializing, "initialize request received")

	var params InitializeParams
	if msg.Params != nil {
		paramBytes, err := json.Marshal(msg.Params)
		if err != nil {
			s.initStateMu.Lock()
			oldState := s.initState
			s.initState = Failed
			s.initStateMu.Unlock()
			s.setNegotiatedVersion("") // Reset version on failure
			s.logInitializationStateChange(oldState, Failed, "parameter marshaling failed")
			return s.SendError(msg.ID, JSONRPCErrorCodeInternalError, JSONRPCErrorMessageInternalError, fmt.Sprintf("Failed to marshal initialize params: %v", err))
		}
		if err := json.Unmarshal(paramBytes, &params); err != nil {
			s.initStateMu.Lock()
			oldState := s.initState
			s.initState = Failed
			s.initStateMu.Unlock()
			s.setNegotiatedVersion("") // Reset version on failure
			s.logInitializationStateChange(oldState, Failed, "parameter parsing failed")
			return s.SendError(msg.ID, JSONRPCErrorCodeInvalidParams, JSONRPCErrorMessageInvalidParams, fmt.Sprintf("Failed to parse initialize params: %v", err))
		}
	}

	// Perform protocol version negotiation
	negotiatedVersion, err := s.negotiateProtocolVersion(params.ProtocolVersion)
	if err != nil {
		s.initStateMu.Lock()
		oldState := s.initState
		s.initState = Failed
		s.initStateMu.Unlock()
		s.setNegotiatedVersion("") // Reset version on failure
		s.logInitializationStateChange(oldState, Failed, "protocol version negotiation failed")
		s.Logger.Printf("Protocol version negotiation failed: client=%s, error=%v", params.ProtocolVersion, err)
		return s.SendError(msg.ID, JSONRPCErrorCodeUnsupportedVersion, JSONRPCErrorMessageUnsupportedVersion, err.Error())
	}
	
	// Store negotiated version
	s.setNegotiatedVersion(negotiatedVersion)
	s.Logger.Printf("Protocol version negotiation successful: client=%s, negotiated=%s", params.ProtocolVersion, negotiatedVersion)

	result := InitializeResult{
		ProtocolVersion: negotiatedVersion,
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

	s.initStateMu.Lock()
	if err := s.validateInitializationStateTransition(s.initState, Initialized); err != nil {
		s.Logger.Printf("Failed to set initialized state: %v", err)
		failedOldState := s.initState
		s.initState = Failed
		s.initStateMu.Unlock()
		s.setNegotiatedVersion("") // Reset version on failure
		s.logInitializationStateChange(failedOldState, Failed, "state validation failed during completion")
		return s.SendError(msg.ID, JSONRPCErrorCodeInternalError, JSONRPCErrorMessageInternalError, "Failed to complete initialization")
	}
	successOldState := s.initState
	s.initState = Initialized
	s.initStateMu.Unlock()
	s.logInitializationStateChange(successOldState, Initialized, "initialization completed successfully")

	s.Logger.Printf("MCP server initialized with client protocol version: %s", params.ProtocolVersion)
	return s.SendResponse(msg.ID, result)
}

func (s *Server) handleListToolsWithValidation(ctx context.Context, msg MCPMessage) error {
	currentState := s.GetInitializationState()
	switch currentState {
	case NotInitialized:
		return s.sendNotInitializedError(msg.ID)
	case Initializing:
		// Allow tools/list during initialization since tools are already registered
		s.Logger.Printf("Allowing tools/list during initialization phase")
	case Failed:
		return s.sendInitializationFailedError(msg.ID)
	case Initialized:
		// Continue with normal execution
	default:
		return s.SendError(msg.ID, JSONRPCErrorCodeInternalError, JSONRPCErrorMessageInternalError, "Unknown initialization state")
	}

	tools := s.ToolHandler.ListTools()
	result := map[string]interface{}{
		"tools": tools,
	}

	s.Logger.Printf("Listed %d tools (state: %s)", len(tools), currentState.String())
	return s.SendResponse(msg.ID, result)
}

func (s *Server) handleCallToolWithValidation(ctx context.Context, msg MCPMessage) error {
	s.Logger.Printf("[DEBUG] handleCallToolWithValidation called with msg.ID=%v", msg.ID)
	
	currentState := s.GetInitializationState()
	s.Logger.Printf("[DEBUG] Current initialization state: %s", currentState.String())
	
	switch currentState {
	case NotInitialized:
		s.Logger.Printf("[ERROR] Tool call rejected - not initialized")
		return s.sendNotInitializedError(msg.ID)
	case Initializing:
		// Allow tool calls during initialization since DirectLSP servers are already started
		s.Logger.Printf("[DEBUG] Allowing tool call during initialization phase")
	case Failed:
		s.Logger.Printf("[ERROR] Tool call rejected - initialization failed")
		return s.sendInitializationFailedError(msg.ID)
	case Initialized:
		s.Logger.Printf("[DEBUG] Tool call accepted - fully initialized")
	default:
		s.Logger.Printf("[ERROR] Unknown initialization state: %s", currentState.String())
		return s.SendError(msg.ID, JSONRPCErrorCodeInternalError, JSONRPCErrorMessageInternalError, "Unknown initialization state")
	}

	var call ToolCall
	if msg.Params != nil {
		s.Logger.Printf("[DEBUG] Parsing tool call params: %+v", msg.Params)
		paramBytes, err := json.Marshal(msg.Params)
		if err != nil {
			s.Logger.Printf("[ERROR] Failed to marshal tool call params: %v", err)
			return s.SendError(msg.ID, JSONRPCErrorCodeInternalError, JSONRPCErrorMessageInternalError, fmt.Sprintf("Failed to marshal tool call params: %v", err))
		}
		if err := json.Unmarshal(paramBytes, &call); err != nil {
			s.Logger.Printf("[ERROR] Failed to parse tool call params: %v", err)
			return s.SendError(msg.ID, JSONRPCErrorCodeInvalidParams, JSONRPCErrorMessageInvalidParams, fmt.Sprintf("Failed to parse tool call params: %v", err))
		}
	} else {
		s.Logger.Printf("[ERROR] Missing tool call parameters")
		return s.SendError(msg.ID, JSONRPCErrorCodeInvalidParams, JSONRPCErrorMessageInvalidParams, "Missing tool call parameters")
	}

	if call.Name == "" {
		s.Logger.Printf("[ERROR] Tool name is empty")
		return s.SendError(msg.ID, JSONRPCErrorCodeInvalidParams, JSONRPCErrorMessageInvalidParams, "Tool name is required")
	}

	s.Logger.Printf("[DEBUG] Calling tool: name=%s, args=%+v", call.Name, call.Arguments)
	result, err := s.ToolHandler.CallTool(ctx, call)

	if err != nil {
		s.Logger.Printf("[ERROR] Tool call failed: tool=%s, error=%v", call.Name, err)
		return s.SendError(msg.ID, JSONRPCErrorCodeInternalError, JSONRPCErrorMessageInternalError, err.Error())
	}

	s.Logger.Printf("[DEBUG] Tool call completed: tool=%s, success=%t, result_length=%d", call.Name, !result.IsError, len(result.Content))
	s.Logger.Printf("[DEBUG] Sending response for msg.ID=%v", msg.ID)
	return s.SendResponse(msg.ID, result)
}

func (s *Server) handlePingWithValidation(ctx context.Context, msg MCPMessage) error {
	result := map[string]interface{}{
		"pong": time.Now().Unix(),
	}
	return s.SendResponse(msg.ID, result)
}

// negotiateProtocolVersion performs MCP protocol version negotiation
// Returns the best compatible version between client and server capabilities
func (s *Server) negotiateProtocolVersion(clientVersion string) (string, error) {
	// If client doesn't specify version, use default
	if clientVersion == "" {
		s.Logger.Printf("Client did not specify protocol version, using default: %s", DefaultProtocolVersion)
		return DefaultProtocolVersion, nil
	}
	
	// Get server supported versions in preference order (newest first)
	supportedVersions := GetSupportedProtocolVersions()
	
	// Check if client version is directly supported
	for _, serverVersion := range supportedVersions {
		if clientVersion == serverVersion {
			s.Logger.Printf("Exact version match found: %s", clientVersion)
			return clientVersion, nil
		}
	}
	
	// Try to find compatible version using compatibility matrix
	compatibleVersion := findCompatibleVersion(clientVersion, supportedVersions)
	if compatibleVersion != "" {
		s.Logger.Printf("Compatible version found: client=%s, server=%s", clientVersion, compatibleVersion)
		return compatibleVersion, nil
	}
	
	// No compatible version found
	supportedStr := strings.Join(supportedVersions, ", ")
	return "", fmt.Errorf("Client version '%s' is not compatible with server versions [%s]", clientVersion, supportedStr)
}

// setNegotiatedVersion stores the negotiated protocol version thread-safely
func (s *Server) setNegotiatedVersion(version string) {
	s.negotiatedVersionMu.Lock()
	defer s.negotiatedVersionMu.Unlock()
	s.negotiatedVersion = version
}

// getNegotiatedVersion retrieves the negotiated protocol version thread-safely
func (s *Server) getNegotiatedVersion() string {
	s.negotiatedVersionMu.RLock()
	defer s.negotiatedVersionMu.RUnlock()
	if s.negotiatedVersion == "" {
		return DefaultProtocolVersion
	}
	return s.negotiatedVersion
}

// GetSupportedProtocolVersions returns supported MCP protocol versions in preference order
func GetSupportedProtocolVersions() []string {
	return []string{
		"2025-06-18", // Current default version
		"2024-11-05", // Previous stable version
		"2024-10-07", // Older stable version
		"2024-09-01", // Legacy version
	}
}

// findCompatibleVersion finds a compatible version using version compatibility matrix
func findCompatibleVersion(clientVersion string, serverVersions []string) string {
	// Version compatibility matrix - maps client versions to compatible server versions
	compatibilityMatrix := map[string][]string{
		"2025-06-18": {"2025-06-18", "2024-11-05"},
		"2024-11-05": {"2025-06-18", "2024-11-05", "2024-10-07"},
		"2024-10-07": {"2024-11-05", "2024-10-07", "2024-09-01"},
		"2024-09-01": {"2024-10-07", "2024-09-01"},
	}
	
	compatibleWithClient, exists := compatibilityMatrix[clientVersion]
	if !exists {
		return "" // Unknown client version
	}
	
	// Find first server version that is compatible with client
	for _, serverVersion := range serverVersions {
		for _, compatibleVersion := range compatibleWithClient {
			if serverVersion == compatibleVersion {
				return serverVersion
			}
		}
	}
	
	return "" // No compatible version found
}

func IsCompatibleProtocolVersion(version string) bool {
	supportedVersions := GetSupportedProtocolVersions()
	
	for _, v := range supportedVersions {
		if version == v {
			return true
		}
	}
	
	// Check compatibility matrix for indirect compatibility
	return findCompatibleVersion(version, supportedVersions) != ""
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

// GetVersionNegotiationInfo returns information about protocol version negotiation
func (s *Server) GetVersionNegotiationInfo() map[string]interface{} {
	negotiatedVersion := s.getNegotiatedVersion()
	supportedVersions := GetSupportedProtocolVersions()
	
	return map[string]interface{}{
		"negotiated_version": negotiatedVersion,
		"supported_versions": supportedVersions,
		"default_version":    DefaultProtocolVersion,
		"is_negotiated":      negotiatedVersion != "" && negotiatedVersion != DefaultProtocolVersion,
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

func (s *Server) readLineWithContext(ctx context.Context, reader *bufio.Reader) (string, error) {
	type result struct {
		line string
		err  error
	}

	ch := make(chan result, 1)
	go func() {
		line, err := reader.ReadString('\n')
		ch <- result{line: line, err: err}
	}()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case res := <-ch:
		return res.line, res.err
	}
}

func (s *Server) readFullWithContext(ctx context.Context, reader *bufio.Reader, buf []byte) (int, error) {
	type result struct {
		n   int
		err error
	}

	ch := make(chan result, 1)
	go func() {
		n, err := io.ReadFull(reader, buf)
		ch <- result{n: n, err: err}
	}()

	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case res := <-ch:
		return res.n, res.err
	}
}

func (s *Server) readBufferWithContext(ctx context.Context, reader *bufio.Reader, buf []byte) (int, error) {
	type result struct {
		n   int
		err error
	}

	ch := make(chan result, 1)
	go func() {
		n, err := reader.Read(buf)
		ch <- result{n: n, err: err}
	}()

	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case res := <-ch:
		return res.n, res.err
	}
}
