package mcp

import (
	"fmt"
	"time"

	"lsp-gateway/internal/transport"
	"lsp-gateway/internal/version"
)

type ServerConfig struct {
	Name string `yaml:"name" json:"name"`

	Description string `yaml:"description" json:"description"`

	Version string `yaml:"version" json:"version"`

	LSPGatewayURL string `yaml:"lsp_gateway_url" json:"lsp_gateway_url"`

	Transport string `yaml:"transport" json:"transport"`

	Timeout time.Duration `yaml:"timeout" json:"timeout"`

	MaxRetries int `yaml:"max_retries" json:"max_retries"`
}

type ToolConfig struct {
	Name string `yaml:"name" json:"name"`

	Description string `yaml:"description" json:"description"`

	LSPMethod string `yaml:"lsp_method" json:"lsp_method"`

	Enabled bool `yaml:"enabled" json:"enabled"`
}

func DefaultConfig() *ServerConfig {
	return &ServerConfig{
		Name:          "lspg-mcp",
		Description:   "MCP server providing LSP functionality through LSP Gateway",
		Version:       version.Version,
		LSPGatewayURL: "http://localhost:8080",
		Transport:     transport.TransportStdio,
		Timeout:       30 * time.Second,
		MaxRetries:    3,
	}
}

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

	validTransports := map[string]bool{
		transport.TransportStdio: true,
		transport.TransportTCP:   true,
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
