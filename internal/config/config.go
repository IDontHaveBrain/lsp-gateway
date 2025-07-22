package config

import (
	"fmt"
	"lsp-gateway/internal/transport"
	"strings"
	"unicode/utf8"
)

const (
	DefaultConfigFile = "config.yaml"
)

const (
	DefaultTransport = "stdio"
)

type ServerConfig struct {
	Name string `yaml:"name" json:"name"`

	Languages []string `yaml:"languages" json:"languages"`

	Command string `yaml:"command" json:"command"`

	Args []string `yaml:"args" json:"args"`

	Transport string `yaml:"transport" json:"transport"`
}

type GatewayConfig struct {
	Servers []ServerConfig `yaml:"servers" json:"servers"`

	Port int `yaml:"port" json:"port"`
}

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

func (c *GatewayConfig) Validate() error {
	if c.Port < 0 || c.Port > 65535 {
		return fmt.Errorf("invalid port: %d, must be between 0 and 65535", c.Port)
	}

	if len(c.Servers) == 0 {
		return fmt.Errorf("at least one server must be configured")
	}

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

func (s *ServerConfig) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("server name cannot be empty")
	}

	// Check for extremely long server names
	if len(s.Name) > 500 {
		return fmt.Errorf("server name too long: %d characters, maximum 500 allowed", len(s.Name))
	}

	if len(s.Languages) == 0 {
		return fmt.Errorf("server must support at least one language")
	}

	// Validate individual language strings
	for i, lang := range s.Languages {
		if strings.TrimSpace(lang) == "" {
			return fmt.Errorf("language at index %d cannot be empty or whitespace-only", i)
		}
	}

	if s.Command == "" {
		return fmt.Errorf("server command cannot be empty")
	}

	// Check for invalid characters in command
	if !utf8.ValidString(s.Command) {
		return fmt.Errorf("command contains invalid UTF-8 characters")
	}
	if strings.ContainsAny(s.Command, "\x00\x01\x02\x03\x04\x05\x06\x07\x08\x0b\x0c\x0e\x0f") {
		return fmt.Errorf("command contains invalid control characters")
	}

	if s.Transport == "" {
		return fmt.Errorf("server transport cannot be empty")
	}

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

func (c *GatewayConfig) GetServerByName(name string) (*ServerConfig, error) {
	for _, server := range c.Servers {
		if server.Name == name {
			return &server, nil
		}
	}
	return nil, fmt.Errorf("no server found with name: %s", name)
}
