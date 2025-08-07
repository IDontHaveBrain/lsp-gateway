package server

// MCPRequest represents a JSON-RPC 2.0 request in the Model Context Protocol.
// Used for incoming tool calls and method invocations from MCP clients.
type MCPRequest struct {
	JSONRPC string      `json:"jsonrpc"`          // Must be "2.0" for JSON-RPC 2.0 compliance
	ID      interface{} `json:"id"`               // Request identifier, can be string, number, or null
	Method  string      `json:"method"`           // The method to be invoked
	Params  interface{} `json:"params,omitempty"` // Method parameters, structure varies by method
}

// MCPResponse represents a JSON-RPC 2.0 response in the Model Context Protocol.
// Contains either a successful result or an error, never both.
type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`          // Must be "2.0" for JSON-RPC 2.0 compliance
	ID      interface{} `json:"id"`               // Matches the request ID this response corresponds to
	Result  interface{} `json:"result,omitempty"` // Success result, omitted if error occurred
	Error   *MCPError   `json:"error,omitempty"`  // Error details, omitted if successful
}

// MCPError represents a JSON-RPC 2.0 error object in the Model Context Protocol.
// Follows standard JSON-RPC error code conventions.
type MCPError struct {
	Code    int         `json:"code"`           // Numeric error code (e.g., -32602 for invalid params)
	Message string      `json:"message"`        // Human-readable error description
	Data    interface{} `json:"data,omitempty"` // Additional error information
}

// ToolContent represents structured content returned by MCP tools.
// Supports different content types including text, JSON data, and binary content with MIME types.
type ToolContent struct {
	Type     string      `json:"type"`               // Content type identifier (e.g., "text", "json")
	Text     string      `json:"text,omitempty"`     // Text content for human-readable responses
	Data     interface{} `json:"data,omitempty"`     // Structured data content
	MimeType string      `json:"mimeType,omitempty"` // MIME type for binary or formatted content
}
