package testutils

import "encoding/json"

// MCPMessage represents an MCP protocol message used across multiple test files
type MCPMessage struct {
	Jsonrpc string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method,omitempty"`
	Params  interface{} `json:"params,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

// LSPPosition represents a position in a text document
// Shared across all language-specific E2E tests
type LSPPosition struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// MCPRequest represents a generic MCP request structure
type MCPRequest struct {
	Jsonrpc string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// MCPResponse represents a generic MCP response structure
type MCPResponse struct {
	Jsonrpc string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

// ParseMCPMessage parses a JSON byte slice into an MCPMessage
func ParseMCPMessage(data []byte) (*MCPMessage, error) {
	var msg MCPMessage
	err := json.Unmarshal(data, &msg)
	return &msg, err
}

// ToJSON converts an MCPMessage to JSON bytes
func (m *MCPMessage) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}