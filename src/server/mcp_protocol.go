package server

// ToolContent represents structured content returned by MCP tools.
// Supports different content types including text, JSON data, and binary content with MIME types.
type ToolContent struct {
    Type     string      `json:"type"`               // Content type identifier (e.g., "text", "json")
    Text     string      `json:"text,omitempty"`     // Text content for human-readable responses
    Data     interface{} `json:"data,omitempty"`     // Structured data content
    MimeType string      `json:"mimeType,omitempty"` // MIME type for binary or formatted content
}
