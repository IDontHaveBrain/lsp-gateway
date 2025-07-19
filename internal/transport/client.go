package transport

import (
	"context"
	"encoding/json"
	"fmt"
)

// JSONRPCMessage represents a JSON-RPC message
type JSONRPCMessage struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method,omitempty"`
	Params  interface{} `json:"params,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC error
type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ClientConfig defines configuration for LSP client
type ClientConfig struct {
	Command   string
	Args      []string
	Transport string
}

// LSPClient defines the interface for LSP server communication
type LSPClient interface {
	Start(ctx context.Context) error
	Stop() error
	SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error)
	SendNotification(ctx context.Context, method string, params interface{}) error
	IsActive() bool
}

const (
	TransportStdio = "stdio"
	TransportTCP   = "tcp"
	TransportHTTP  = "http"
)

// NewLSPClient creates a new LSP client based on transport type
func NewLSPClient(config ClientConfig) (LSPClient, error) {
	switch config.Transport {
	case TransportStdio:
		return NewStdioClient(config)
	case TransportTCP:
		return NewTCPClient(config)
	default:
		return nil, fmt.Errorf("unsupported transport: %s", config.Transport)
	}
}
