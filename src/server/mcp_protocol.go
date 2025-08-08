package server

import "lsp-gateway/src/server/protocol"

// MCP types using shared JSON-RPC protocol structures
// These are aliases for backward compatibility and MCP-specific documentation

// MCPRequest represents a JSON-RPC 2.0 request in the Model Context Protocol.
// Used for incoming tool calls and method invocations from MCP clients.
type MCPRequest = protocol.JSONRPCRequest

// MCPResponse represents a JSON-RPC 2.0 response in the Model Context Protocol.
// Contains either a successful result or an error, never both.
type MCPResponse = protocol.JSONRPCResponse

// MCPError represents a JSON-RPC 2.0 error object in the Model Context Protocol.
// Follows standard JSON-RPC error code conventions.
type MCPError = protocol.RPCError

// ToolContent represents structured content returned by MCP tools.
// Supports different content types including text, JSON data, and binary content with MIME types.
type ToolContent struct {
	Type     string      `json:"type"`               // Content type identifier (e.g., "text", "json")
	Text     string      `json:"text,omitempty"`     // Text content for human-readable responses
	Data     interface{} `json:"data,omitempty"`     // Structured data content
	MimeType string      `json:"mimeType,omitempty"` // MIME type for binary or formatted content
}
