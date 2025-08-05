package protocol

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"lsp-gateway/src/internal/common"
)

// JSON-RPC protocol constants
const (
	JSONRPCVersion = "2.0"
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
	bufReader := bufio.NewReader(reader)

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
					return fmt.Errorf("LSP server connection closed unexpectedly (EOF)")
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
			sanitizedError := common.SanitizeErrorForLogging(msg.Error)
			common.LSPLogger.Warn("LSP response contains error: id=%v, error=%s", msg.ID, sanitizedError)
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
