package transport

import "context"

// Transport interface for MCP communication
type Transport interface {
	Connect(ctx context.Context) error
	Disconnect() error
	Send(ctx context.Context, data []byte) error
	Receive(ctx context.Context) ([]byte, error)
	IsConnected() bool
	Close() error
}