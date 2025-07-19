package config

import (
	"fmt"
	"lsp-gateway/internal/transport"
)

// Configuration file constants
const (
	DefaultConfigFile = "config.yaml"
)

// Default transport constants
const (
	DefaultTransport = "stdio"
)

// ServerConfig represents the configuration for a single LSP server
type ServerConfig struct {
	// Name is the unique identifier for this LSP server
	Name string `yaml:"name" json:"name"`

	// Languages is the list of programming languages this server supports
	Languages []string `yaml:"languages" json:"languages"`

	// Command is the executable command to start the LSP server
	Command string `yaml:"command" json:"command"`

	// Args are the command-line arguments passed to the LSP server
	Args []string `yaml:"args" json:"args"`

	// Transport specifies the communication method (stdio, tcp, etc.)
	Transport string `yaml:"transport" json:"transport"`
}

// GatewayConfig represents the main configuration for the LSP Gateway
type GatewayConfig struct {
	// Servers is the list of LSP servers to manage
	Servers []ServerConfig `yaml:"servers" json:"servers"`

	// Port is the port number the gateway will listen on
	Port int `yaml:"port" json:"port"`
}

// DefaultConfig returns a default configuration with a Go LSP server
func DefaultConfig() *GatewayConfig {
	return &GatewayConfig{
		Port: 8080,
		Servers: []ServerConfig{
			{
				Name:      "go-lsp",
				Languages: []string{"go"},
				Command:   "gopls",
				Args:      []string{},
				Transport: DefaultTransport,
			},
		},
	}
}

// Validate checks if the configuration is valid
func (c *GatewayConfig) Validate() error {
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("invalid port: %d, must be between 1 and 65535", c.Port)
	}

	if len(c.Servers) == 0 {
		return fmt.Errorf("at least one server must be configured")
	}

	// Check for duplicate server names
	names := make(map[string]bool)
	for _, server := range c.Servers {
		if err := server.Validate(); err != nil {
			return fmt.Errorf("server %s: %w", server.Name, err)
		}

		if names[server.Name] {
			return fmt.Errorf("duplicate server name: %s", server.Name)
		}
		names[server.Name] = true
	}

	return nil
}

// Validate checks if the server configuration is valid
func (s *ServerConfig) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("server name cannot be empty")
	}

	if len(s.Languages) == 0 {
		return fmt.Errorf("server must support at least one language")
	}

	if s.Command == "" {
		return fmt.Errorf("server command cannot be empty")
	}

	if s.Transport == "" {
		return fmt.Errorf("server transport cannot be empty")
	}

	// Validate transport type
	validTransports := map[string]bool{
		transport.TransportStdio: true,
		transport.TransportTCP:   true,
		transport.TransportHTTP:  true,
	}

	if !validTransports[s.Transport] {
		return fmt.Errorf("invalid transport type: %s, must be one of: stdio, tcp, http", s.Transport)
	}

	return nil
}

// GetServerByLanguage returns the first server that supports the given language
func (c *GatewayConfig) GetServerByLanguage(language string) (*ServerConfig, error) {
	for _, server := range c.Servers {
		for _, lang := range server.Languages {
			if lang == language {
				return &server, nil
			}
		}
	}
	return nil, fmt.Errorf("no server found for language: %s", language)
}

// GetServerByName returns the server with the given name
func (c *GatewayConfig) GetServerByName(name string) (*ServerConfig, error) {
	for _, server := range c.Servers {
		if server.Name == name {
			return &server, nil
		}
	}
	return nil, fmt.Errorf("no server found with name: %s", name)
}
