package mcp

import (
	"encoding/json"
	"fmt"
)

const (
	JSONRPCVersion = "2.0"
	ProtocolVersion = "2025-06-18"
)

// MCPMessage represents an MCP protocol message matching server format
type MCPMessage struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method,omitempty"`
	Params  interface{} `json:"params,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

// MCPError represents an error in MCP protocol
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// MCPRequest represents a request message
type MCPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// MCPResponse represents a response message
type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

// InitializeParams represents parameters for the initialize method
type InitializeParams struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ClientInfo      map[string]interface{} `json:"clientInfo"`
}

// InitializeResult represents the result of the initialize method
type InitializeResult struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ServerInfo      map[string]interface{} `json:"serverInfo"`
}

// ToolCall represents parameters for a tool call
type ToolCall struct {
	Tool   string                 `json:"tool"`
	Params map[string]interface{} `json:"params,omitempty"`
}

// ToolResult represents the result of a tool call (matching server structure)
type ToolResult struct {
	Content []ContentBlock    `json:"content"`
	IsError bool              `json:"isError,omitempty"`
	Error   *StructuredError  `json:"error,omitempty"`
	Meta    *ResponseMetadata `json:"meta,omitempty"`
}

// ContentBlock represents a content block in tool results
type ContentBlock struct {
	Type        string                 `json:"type"`
	Text        string                 `json:"text,omitempty"`
	Data        interface{}            `json:"data,omitempty"`
	Annotations map[string]interface{} `json:"annotations,omitempty"`
}

// ResponseMetadata represents metadata for a tool response
type ResponseMetadata struct {
	Timestamp   string                 `json:"timestamp"`
	Duration    string                 `json:"duration,omitempty"`
	LSPMethod   string                 `json:"lspMethod"`
	CacheHit    bool                   `json:"cacheHit,omitempty"`
	Warnings    []string               `json:"warnings,omitempty"`
	RequestInfo map[string]interface{} `json:"requestInfo,omitempty"`
}

// StructuredError represents a structured error response
type StructuredError struct {
	Code        int                    `json:"code"`
	Message     string                 `json:"message"`
	Details     string                 `json:"details,omitempty"`
	Context     map[string]interface{} `json:"context,omitempty"`
	Suggestions []string               `json:"suggestions,omitempty"`
	Retryable   bool                   `json:"retryable,omitempty"`
}

// ListToolsResult represents the result of listing available tools
type ListToolsResult struct {
	Tools []ToolInfo `json:"tools"`
}

// ToolInfo represents information about a tool
type ToolInfo struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema,omitempty"`
}

// Tool represents a tool with its schema
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema,omitempty"`
}

// Common MCP error codes
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
)

// NewMCPError creates a new MCP error with the given code and message
func NewMCPError(code int, message string, data interface{}) *MCPError {
	return &MCPError{
		Code:    code,
		Message: message,
		Data:    data,
	}
}

// IsRequest checks if the message is a request
func (m *MCPMessage) IsRequest() bool {
	return m.ID != nil && m.Method != ""
}

// IsResponse checks if the message is a response
func (m *MCPMessage) IsResponse() bool {
	return m.ID != nil && m.Method == "" && (m.Result != nil || m.Error != nil)
}

// IsNotification checks if the message is a notification
func (m *MCPMessage) IsNotification() bool {
	return m.ID == nil && m.Method != ""
}

// Validate performs basic validation on the message
func (m *MCPMessage) Validate() error {
	if m.JSONRPC != JSONRPCVersion {
		return fmt.Errorf("invalid jsonrpc version: %s", m.JSONRPC)
	}

	if m.IsRequest() {
		if m.Result != nil || m.Error != nil {
			return fmt.Errorf("request cannot have result or error fields")
		}
	} else if m.IsResponse() {
		if m.Params != nil {
			return fmt.Errorf("response cannot have params field")
		}
		if m.Result != nil && m.Error != nil {
			return fmt.Errorf("response cannot have both result and error")
		}
	}

	return nil
}

// ToJSON converts the message to JSON bytes
func (m *MCPMessage) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

// ParseMCPMessage parses JSON bytes into an MCPMessage
func ParseMCPMessage(data []byte) (*MCPMessage, error) {
	var msg MCPMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// ToRequest converts MCPMessage to MCPRequest
func (m *MCPMessage) ToRequest() (*MCPRequest, error) {
	if !m.IsRequest() {
		return nil, fmt.Errorf("message is not a request")
	}
	return &MCPRequest{
		JSONRPC: m.JSONRPC,
		ID:      m.ID,
		Method:  m.Method,
		Params:  m.Params,
	}, nil
}

// ToResponse converts MCPMessage to MCPResponse
func (m *MCPMessage) ToResponse() (*MCPResponse, error) {
	if !m.IsResponse() {
		return nil, fmt.Errorf("message is not a response")
	}
	return &MCPResponse{
		JSONRPC: m.JSONRPC,
		ID:      m.ID,
		Result:  m.Result,
		Error:   m.Error,
	}, nil
}