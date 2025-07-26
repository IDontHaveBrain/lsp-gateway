package transport

import (
	"context"
	"encoding/json"
	"fmt"
)

type JSONRPCMessage struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method,omitempty"`
	Params  interface{} `json:"params,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type ClientConfig struct {
	Command   string
	Args      []string
	Transport string
}

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

const (
	JSONRPCVersion = "2.0"
)

const (
	ErrorUnsupportedTransport = "unsupported transport"
)

func NewLSPClient(config ClientConfig) (LSPClient, error) {
	return NewLSPClientWithSCIP(config, nil)
}

// NewLSPClientWithSCIP creates a new LSP client with optional SCIP indexer support.
// Pass nil for scipIndexer to disable SCIP indexing.
func NewLSPClientWithSCIP(config ClientConfig, scipIndexer SCIPIndexer) (LSPClient, error) {
	switch config.Transport {
	case TransportStdio:
		return NewStdioClientWithSCIP(config, scipIndexer)
	case TransportTCP:
		return NewTCPClientWithSCIP(config, scipIndexer)
	default:
		return nil, fmt.Errorf("%s: %s", ErrorUnsupportedTransport, config.Transport)
	}
}
