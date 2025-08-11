package protocol

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/constants"
	"lsp-gateway/src/internal/errors"
)

// JSON-RPC protocol constants
const (
	JSONRPCVersion = "2.0"
)

// JSON-RPC error codes (RFC 7309)
const (
	ParseError     = -32700 // Invalid JSON was received by the server
	InvalidRequest = -32600 // The JSON sent is not a valid Request object
	MethodNotFound = -32601 // The method does not exist / is not available
	InvalidParams  = -32602 // Invalid method parameter(s)
	InternalError  = -32603 // Internal JSON-RPC error
)

// JSONRPCMessage represents a JSON-RPC 2.0 message
type JSONRPCMessage struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method,omitempty"`
	Params  interface{} `json:"params,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

// JSONRPCRequest represents a JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC 2.0 response
type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC error
type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// JSONRPCProtocol defines the interface for JSON-RPC protocol handling
type JSONRPCProtocol interface {
	WriteMessage(writer io.Writer, msg JSONRPCMessage) error
	HandleMessage(data []byte, messageHandler MessageHandler) error
	HandleResponses(reader io.Reader, messageHandler MessageHandler, stopCh <-chan struct{}) error
}

// MessageHandler defines the interface for handling different types of JSON-RPC messages
type MessageHandler interface {
	HandleRequest(method string, id interface{}, params interface{}) error
	HandleResponse(id interface{}, result json.RawMessage, err *RPCError) error
	HandleNotification(method string, params interface{}) error
}

// LSPJSONRPCProtocol implements JSON-RPC protocol handling for LSP communication
type LSPJSONRPCProtocol struct {
	language string // Language identifier for logging context
}

// NewLSPJSONRPCProtocol creates a new LSP JSON-RPC protocol handler
func NewLSPJSONRPCProtocol(language string) *LSPJSONRPCProtocol {
	return &LSPJSONRPCProtocol{
		language: language,
	}
}

// WriteMessage sends a JSON-RPC message with proper Content-Length header formatting
func (p *LSPJSONRPCProtocol) WriteMessage(writer io.Writer, msg JSONRPCMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// Format with Content-Length header according to LSP protocol
	content := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(data), data)

	_, err = writer.Write([]byte(content))
	return err
}

// HandleResponses processes responses from the LSP server using Content-Length header parsing
func (p *LSPJSONRPCProtocol) HandleResponses(reader io.Reader, messageHandler MessageHandler, stopCh <-chan struct{}) error {
	// Use a larger buffer size (1MB) to handle large LSP responses, especially workspace/symbol
	// which can return many results. This fixes Windows CI failures with truncated JSON.
	bufReader := bufio.NewReaderSize(reader, constants.LSPResponseBufferSize)

	for {
		select {
		case <-stopCh:
			return nil
		default:
		}

		// Read Content-Length header (LSP protocol format)
		var contentLength int

		// Read headers synchronously with proper timeout
		for {
			line, err := bufReader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					// EOF is expected during shutdown
					return nil
				}
				return err
			}

			line = strings.TrimSpace(line)
			if line == "" {
				// Empty line indicates end of headers
				break
			}

			if strings.HasPrefix(line, "Content-Length:") {
				lengthStr := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
				length, err := strconv.Atoi(lengthStr)
				if err != nil {
					common.LSPLogger.Debug("Failed to parse Content-Length: %s", lengthStr)
					continue
				}
				contentLength = length
			}
		}

		if contentLength > 0 {
			// Read the JSON body
			body := make([]byte, contentLength)
			_, err := io.ReadFull(bufReader, body)
			if err != nil {
				return err
			}

			if err := p.HandleMessage(body, messageHandler); err != nil {
				common.LSPLogger.Error("Error handling message: %v", err)
				// Continue processing other messages
			}
		}
	}
}

// HandleMessage processes a single JSON-RPC message and routes it to the appropriate handler
func (p *LSPJSONRPCProtocol) HandleMessage(data []byte, messageHandler MessageHandler) error {
	var msg JSONRPCMessage
	err := json.Unmarshal(data, &msg)
	if err != nil {
		common.LSPLogger.Error("Failed to unmarshal JSON from %s: %v", p.language, err)
		return err
	}

	// Check for server-initiated messages FIRST (before treating as client responses)
	if msg.Method != "" {
		if msg.ID != nil {
			// Server-initiated request (has both method and ID) - must respond
			common.LSPLogger.Debug("Received server request: method=%s, id=%v from %s", msg.Method, msg.ID, p.language)
			return messageHandler.HandleRequest(msg.Method, msg.ID, msg.Params)
		} else {
			// Server-initiated notification (has method, no ID)
			common.LSPLogger.Debug("Received server notification: method=%s from %s", msg.Method, p.language)
			return messageHandler.HandleNotification(msg.Method, msg.Params)
		}
	} else if msg.ID != nil {
		// Handle client response (has ID but no method)
		var result json.RawMessage
		var rpcError *RPCError

		if msg.Error != nil {
			rpcError = msg.Error
			// Suppress expected errors during indexing operations
			if !IsExpectedSuppressibleError(rpcError) {
				sanitizedError := common.SanitizeErrorForLogging(msg.Error)
				common.LSPLogger.Warn("LSP response contains error: id=%v, error=%s", msg.ID, sanitizedError)
			}
		} else if msg.Result != nil {
			result, _ = json.Marshal(msg.Result)
		}

		return messageHandler.HandleResponse(msg.ID, result, rpcError)
	} else {
		// Handle malformed messages (no ID and no method)
		common.LSPLogger.Warn("Received malformed message (no ID and no method) from %s", p.language)
		return fmt.Errorf("malformed JSON-RPC message: no ID and no method")
	}
}

// CreateMessage creates a JSON-RPC message with the specified parameters
func CreateMessage(method string, id interface{}, params interface{}) JSONRPCMessage {
	return JSONRPCMessage{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Method:  method,
		Params:  params,
	}
}

// CreateNotification creates a JSON-RPC notification (no ID)
func CreateNotification(method string, params interface{}) JSONRPCMessage {
	return JSONRPCMessage{
		JSONRPC: JSONRPCVersion,
		Method:  method,
		Params:  params,
	}
}

// CreateResponse creates a JSON-RPC response message
func CreateResponse(id interface{}, result interface{}, err *RPCError) JSONRPCMessage {
	return JSONRPCMessage{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Result:  result,
		Error:   err,
	}
}

// Helper functions for creating error responses

// NewRPCError creates a new RPCError with the specified code and message
func NewRPCError(code int, message string, data interface{}) *RPCError {
	return &RPCError{
		Code:    code,
		Message: message,
		Data:    data,
	}
}

// NewParseError creates a parse error (-32700)
func NewParseError(data interface{}) *RPCError {
	return NewRPCError(ParseError, "Parse error", data)
}

// NewInvalidRequestError creates an invalid request error (-32600)
func NewInvalidRequestError(data interface{}) *RPCError {
	return NewRPCError(InvalidRequest, "Invalid Request", data)
}

// NewMethodNotFoundError creates a method not found error (-32601)
func NewMethodNotFoundError(data interface{}) *RPCError {
	return NewRPCError(MethodNotFound, "Method not found", data)
}

// NewInternalError creates an internal error (-32603)
func NewInternalError(data interface{}) *RPCError {
	return NewRPCError(InternalError, "Internal error", data)
}

// IsExpectedSuppressibleError checks if an error should be suppressed based on its structure and message
func IsExpectedSuppressibleError(err interface{}) bool {
	if err == nil {
		return false
	}

	// Handle RPCError types
	if rpcErr, ok := err.(*RPCError); ok {
		return isRPCErrorSuppressible(rpcErr)
	}

	// Handle regular errors by checking error message
	if regularErr, ok := err.(error); ok {
		return isErrorMessageSuppressible(regularErr.Error())
	}

	// Handle any other types by converting to string
	errMsg := fmt.Sprintf("%v", err)
	return isErrorMessageSuppressible(errMsg)
}

// isRPCErrorSuppressible checks if an RPCError should be suppressed
func isRPCErrorSuppressible(rpcError *RPCError) bool {
	if rpcError == nil {
		return false
	}
	return isErrorMessageSuppressible(rpcError.Message)
}

// isErrorMessageSuppressible checks if an error message contains suppression patterns
func isErrorMessageSuppressible(message string) bool {
	msg := strings.ToLower(message)

	// Common patterns to suppress during indexing operations
	suppressPatterns := []string{
		"no identifier found",
		"identifier not found",
		"symbol not found",
		"no symbol at position",
		"position out of range",
		"bad line number",
		"line number",
	}

	for _, pattern := range suppressPatterns {
		if strings.Contains(msg, pattern) {
			return true
		}
	}

	return false
}

// Unified error system integration functions

// NewUnifiedRPCError creates an RPCError from a unified error
func NewUnifiedRPCError(err error) *RPCError {
	if err == nil {
		return nil
	}

	// Handle unified error types
	if lspErr, ok := err.(*errors.LSPError); ok {
		return &RPCError{
			Code:    lspErr.Code,
			Message: lspErr.Message,
			Data:    lspErr.Data,
		}
	}

	if valErr, ok := err.(*errors.ValidationError); ok {
		return NewRPCError(InvalidParams, valErr.Error(), map[string]string{
			"parameter": valErr.Parameter,
		})
	}

	if connErr, ok := err.(*errors.ConnectionError); ok {
		return NewRPCError(errors.ConnectionFailure, connErr.Error(), map[string]string{
			"language": connErr.Language,
			"type":     connErr.Type,
		})
	}

	if timeoutErr, ok := err.(*errors.TimeoutError); ok {
		return NewRPCError(errors.OperationTimeout, timeoutErr.Error(), map[string]string{
			"operation": timeoutErr.Operation,
			"language":  timeoutErr.Language,
		})
	}

	if methodErr, ok := err.(*errors.MethodNotSupportedError); ok {
		return NewRPCError(MethodNotFound, methodErr.Error(), map[string]string{
			"server":     methodErr.Server,
			"method":     methodErr.Method,
			"suggestion": methodErr.Suggestion,
		})
	}

	if procErr, ok := err.(*errors.ProcessError); ok {
		var code int
		switch procErr.Type {
		case "start":
			code = errors.ProcessStartFailure
		case "stop":
			code = errors.ProcessStopFailure
		default:
			code = errors.CommunicationError
		}
		return NewRPCError(code, procErr.Error(), map[string]string{
			"language": procErr.Language,
			"command":  procErr.Command,
			"type":     procErr.Type,
		})
	}

	// Default to internal error for unknown types
	return NewInternalError(err.Error())
}

// CreateUnifiedErrorResponse creates a JSON-RPC error response from a unified error
func CreateUnifiedErrorResponse(id interface{}, err error) JSONRPCMessage {
	rpcError := NewUnifiedRPCError(err)
	return CreateResponse(id, nil, rpcError)
}

// NewValidationRPCError creates an RPCError for parameter validation failures
func NewValidationRPCError(parameter, message string) *RPCError {
	return NewRPCError(InvalidParams,
		fmt.Sprintf("Invalid parameter '%s': %s", parameter, message),
		map[string]string{"parameter": parameter})
}

// NewConnectionRPCError creates an RPCError for connection failures
func NewConnectionRPCError(language string, cause error) *RPCError {
	message := fmt.Sprintf("Connection failed to %s server", language)
	if cause != nil {
		message = fmt.Sprintf("%s: %v", message, cause)
	}
	return NewRPCError(errors.ConnectionFailure, message, map[string]string{
		"language": language,
	})
}

// NewTimeoutRPCError creates an RPCError for timeout errors
func NewTimeoutRPCError(operation, language string) *RPCError {
	message := fmt.Sprintf("Operation timeout: %s", operation)
	if language != "" {
		message = fmt.Sprintf("%s (language: %s)", message, language)
	}
	return NewRPCError(errors.OperationTimeout, message, map[string]string{
		"operation": operation,
		"language":  language,
	})
}

// IsUnifiedError checks if an error is from the unified error system
func IsUnifiedError(err error) bool {
	if err == nil {
		return false
	}

	return errors.IsConnectionError(err) ||
		errors.IsValidationError(err) ||
		errors.IsTimeoutError(err) ||
		errors.IsMethodNotSupportedError(err) ||
		errors.IsProcessError(err)
}
