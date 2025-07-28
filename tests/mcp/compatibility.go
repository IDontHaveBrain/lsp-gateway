package mcp

import (
	"time"

	"lsp-gateway/tests/e2e/mcp/transport"
)

// Transport alias for compatibility with existing code
type Transport = transport.Transport

// State constants for compatibility - map to existing ClientState values
const (
	StateDisconnected  = Disconnected
	StateConnecting    = Connecting
	StateConnected     = Connected
	StateReady         = Initialized
)

// ClientConfig represents the configuration for the MCP client
type ClientConfig struct {
	ServerURL         string
	TransportType     TransportType
	ConnectionTimeout time.Duration
	RequestTimeout    time.Duration
	MaxRetries        int
	RetryDelay        time.Duration
	BackoffMultiplier float64
	AutoReconnect     bool
	ProtocolVersion   string
	ClientInfo        map[string]interface{}
	Capabilities      map[string]interface{}
	LogLevel          string
	MetricsEnabled    bool
}

type TransportType int

const (
	TransportSTDIO TransportType = iota
	TransportTCP
	TransportHTTP
)

// DefaultClientConfig returns a default configuration for testing
func DefaultClientConfig() ClientConfig {
	return ClientConfig{
		ServerURL:         "localhost:8080",
		TransportType:     TransportSTDIO,
		ConnectionTimeout: 30 * time.Second,
		RequestTimeout:    10 * time.Second,
		MaxRetries:        3,
		RetryDelay:        time.Second,
		BackoffMultiplier: 2.0,
		AutoReconnect:     true,
		ProtocolVersion:   "2024-11-05",
		ClientInfo: map[string]interface{}{
			"name":    "mcp-test-client", 
			"version": "1.0.0",
		},
		Capabilities: map[string]interface{}{
			"tools":         true,
			"resources":     true,
			"prompts":       true,
			"notifications": true,
		},
		LogLevel:       "info",
		MetricsEnabled: true,
	}
}

// TestMCPClient is a simple mock implementation for compatibility
type TestMCPClient struct {
	stateManager *StateManager
	transport    Transport
	config       ClientConfig
}

// NewTestMCPClient creates a new test MCP client (placeholder implementation)
func NewTestMCPClient(config ClientConfig, transport Transport) *TestMCPClient {
	return &TestMCPClient{
		stateManager: NewStateManager(),
		transport:    transport,
		config:       config,
	}
}

func (c *TestMCPClient) GetState() ClientState {
	return c.stateManager.GetState()
}

func (c *TestMCPClient) WaitForState(expected ClientState, timeout time.Duration) error {
	return c.stateManager.WaitForState(expected, timeout)
}