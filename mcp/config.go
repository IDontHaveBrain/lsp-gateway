package mcp

import (
	"fmt"
	"time"

	"lsp-gateway/internal/gateway"
	"lsp-gateway/internal/transport"
)

// ServerConfig represents the configuration for the MCP server
type ServerConfig struct {
	// Name is the identifier for this MCP server instance
	Name string `yaml:"name" json:"name"`

	// Description provides details about this MCP server's purpose
	Description string `yaml:"description" json:"description"`

	// Version of the MCP server implementation
	Version string `yaml:"version" json:"version"`

	// LSPGatewayURL is the URL to the LSP Gateway instance
	LSPGatewayURL string `yaml:"lsp_gateway_url" json:"lsp_gateway_url"`

	// Transport specifies the MCP transport method (stdio, websocket, etc.)
	Transport string `yaml:"transport" json:"transport"`

	// Timeout for LSP Gateway requests
	Timeout time.Duration `yaml:"timeout" json:"timeout"`

	// MaxRetries for failed LSP Gateway requests
	MaxRetries int `yaml:"max_retries" json:"max_retries"`
}

// ToolConfig represents configuration for individual MCP tools
type ToolConfig struct {
	// Name of the MCP tool
	Name string `yaml:"name" json:"name"`

	// Description of what this tool does
	Description string `yaml:"description" json:"description"`

	// LSPMethod is the corresponding LSP method this tool maps to
	LSPMethod string `yaml:"lsp_method" json:"lsp_method"`

	// Enabled controls whether this tool is available
	Enabled bool `yaml:"enabled" json:"enabled"`
}

// DefaultConfig returns a default MCP server configuration
func DefaultConfig() *ServerConfig {
	return &ServerConfig{
		Name:          "lsp-gateway-mcp",
		Description:   "MCP server providing LSP functionality through LSP Gateway",
		Version:       "0.1.0",
		LSPGatewayURL: "http://localhost:8080",
		Transport:     transport.TransportStdio,
		Timeout:       30 * time.Second,
		MaxRetries:    3,
	}
}

// Validate checks if the server configuration is valid
func (c *ServerConfig) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("server name cannot be empty")
	}

	if c.LSPGatewayURL == "" {
		return fmt.Errorf("LSP Gateway URL cannot be empty")
	}

	if c.Transport == "" {
		return fmt.Errorf("transport cannot be empty")
	}

	// Validate transport type
	validTransports := map[string]bool{
		transport.TransportStdio: true,
		"websocket":              true,
		transport.TransportHTTP:  true,
	}

	if !validTransports[c.Transport] {
		return fmt.Errorf("invalid transport type: %s", c.Transport)
	}

	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}

	if c.MaxRetries < 0 {
		return fmt.Errorf("max retries cannot be negative")
	}

	return nil
}

// DefaultTools returns the default set of MCP tools mapped to LSP methods
func DefaultTools() []ToolConfig {
	return []ToolConfig{
		{
			Name:        "goto_definition",
			Description: "Navigate to the definition of a symbol",
			LSPMethod:   "textDocument/definition",
			Enabled:     true,
		},
		{
			Name:        "find_references",
			Description: "Find all references to a symbol",
			LSPMethod:   "textDocument/references",
			Enabled:     true,
		},
		{
			Name:        "get_hover_info",
			Description: "Get hover information for a symbol",
			LSPMethod:   gateway.LSPMethodHover,
			Enabled:     true,
		},
		{
			Name:        "get_document_symbols",
			Description: "Get all symbols in a document",
			LSPMethod:   "textDocument/documentSymbol",
			Enabled:     true,
		},
		{
			Name:        "search_workspace_symbols",
			Description: "Search for symbols in the workspace",
			LSPMethod:   "workspace/symbol",
			Enabled:     true,
		},
	}
}
