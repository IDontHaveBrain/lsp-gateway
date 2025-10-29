package client

import (
	"context"
	"io"
	"net"
	"time"

	"lsp-gateway/src/server/process"
)

type Transport interface {
	Connect(ctx context.Context) error
	Disconnect() error
	Reader() io.Reader
	Writer() io.Writer
	IsConnected() bool
}

type StdioTransport struct {
	processInfo *process.ProcessInfo
	connected   bool
}

func NewStdioTransport(processInfo *process.ProcessInfo) Transport {
	return &StdioTransport{
		processInfo: processInfo,
	}
}

func (t *StdioTransport) Connect(ctx context.Context) error {
	if t.processInfo == nil {
		return nil
	}
	t.connected = true
	return nil
}

func (t *StdioTransport) Disconnect() error {
	t.connected = false
	return nil
}

func (t *StdioTransport) Reader() io.Reader {
	if t.processInfo == nil {
		return nil
	}
	return t.processInfo.Stdout
}

func (t *StdioTransport) Writer() io.Writer {
	if t.processInfo == nil {
		return nil
	}
	return t.processInfo.Stdin
}

func (t *StdioTransport) IsConnected() bool {
	return t.connected && t.processInfo != nil
}

type SocketTransport struct {
	addr      string
	conn      net.Conn
	connected bool
}

func NewSocketTransport(addr string) Transport {
	return &SocketTransport{
		addr: addr,
	}
}

func (t *SocketTransport) Connect(ctx context.Context) error {
	maxRetries := 10
	retryDelay := 500 * time.Millisecond

	var lastErr error
	for i := 0; i < maxRetries; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		conn, err := net.DialTimeout("tcp", t.addr, 3*time.Second)
		if err == nil {
			t.conn = conn
			t.connected = true
			return nil
		}

		lastErr = err
		if i < maxRetries-1 {
			time.Sleep(retryDelay)
		}
	}

	return lastErr
}

func (t *SocketTransport) Disconnect() error {
	t.connected = false
	if t.conn != nil {
		return t.conn.Close()
	}
	return nil
}

func (t *SocketTransport) Reader() io.Reader {
	return t.conn
}

func (t *SocketTransport) Writer() io.Writer {
	return t.conn
}

func (t *SocketTransport) IsConnected() bool {
	return t.connected && t.conn != nil
}
