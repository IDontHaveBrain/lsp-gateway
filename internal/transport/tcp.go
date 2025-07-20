package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type TCPClient struct {
	config ClientConfig
	conn   net.Conn

	mu     sync.RWMutex
	active int32 // atomic flag for active state

	requests  map[string]chan json.RawMessage
	requestMu sync.RWMutex
	nextID    int64

	reader *bufio.Reader
	writer *bufio.Writer

	ctx    context.Context
	cancel context.CancelFunc
}

func NewTCPClient(config ClientConfig) (LSPClient, error) {
	if config.Transport != TransportTCP {
		return nil, fmt.Errorf("invalid transport for TCP client: %s", config.Transport)
	}

	return &TCPClient{
		config:   config,
		requests: make(map[string]chan json.RawMessage),
	}, nil
}

func (c *TCPClient) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if atomic.LoadInt32(&c.active) != 0 {
		return fmt.Errorf("TCP client already active")
	}

	c.ctx, c.cancel = context.WithCancel(ctx)

	address, err := c.parseAddress()
	if err != nil {
		return fmt.Errorf("failed to parse TCP address: %w", err)
	}

	conn, err := net.DialTimeout("tcp", address, 10*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to TCP server at %s: %w", address, err)
	}

	c.conn = conn
	c.reader = bufio.NewReader(conn)
	c.writer = bufio.NewWriter(conn)

	atomic.StoreInt32(&c.active, 1)

	go c.handleMessages()

	return nil
}

func (c *TCPClient) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if atomic.LoadInt32(&c.active) == 0 {
		return nil // Already stopped
	}

	if c.cancel != nil {
		c.cancel()
	}

	if c.conn != nil {
		_ = c.conn.Close()
	}

	c.requestMu.Lock()
	for id, ch := range c.requests {
		close(ch)
		delete(c.requests, id)
	}
	c.requestMu.Unlock()

	atomic.StoreInt32(&c.active, 0)

	return nil
}

func (c *TCPClient) SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	if atomic.LoadInt32(&c.active) == 0 {
		return nil, errors.New(ERROR_TCP_CLIENT_NOT_ACTIVE)
	}

	id := c.generateRequestID()

	respCh := make(chan json.RawMessage, 1)

	c.requestMu.Lock()
	c.requests[id] = respCh
	c.requestMu.Unlock()

	defer func() {
		c.requestMu.Lock()
		delete(c.requests, id)
		c.requestMu.Unlock()
		close(respCh)
	}()

	request := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	if err := c.sendMessage(request); err != nil {
		return nil, fmt.Errorf(ERROR_SEND_REQUEST, err)
	}

	select {
	case response := <-respCh:
		return response, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.ctx.Done():
		return nil, errors.New(ERROR_CLIENT_STOPPED)
	}
}

func (c *TCPClient) SendNotification(ctx context.Context, method string, params interface{}) error {
	if atomic.LoadInt32(&c.active) == 0 {
		return errors.New(ERROR_TCP_CLIENT_NOT_ACTIVE)
	}

	notification := JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}

	if err := c.sendMessage(notification); err != nil {
		return fmt.Errorf(ERROR_SEND_NOTIFICATION, err)
	}

	return nil
}

func (c *TCPClient) IsActive() bool {
	return atomic.LoadInt32(&c.active) != 0
}

func (c *TCPClient) parseAddress() (string, error) {
	address := c.config.Command

	if address == "" {
		return "", fmt.Errorf("TCP address not specified in command field")
	}

	if _, err := strconv.Atoi(address); err == nil {
		address = "localhost:" + address
	}

	if !strings.Contains(address, ":") {
		return "", fmt.Errorf("invalid TCP address format: %s (expected host:port)", address)
	}

	return address, nil
}

func (c *TCPClient) generateRequestID() string {
	id := atomic.AddInt64(&c.nextID, 1)
	return strconv.FormatInt(id, 10)
}

func (c *TCPClient) sendMessage(msg JSONRPCMessage) error {
	c.mu.RLock()
	writer := c.writer
	conn := c.conn
	c.mu.RUnlock()

	if writer == nil {
		return errors.New(ERROR_TCP_CONNECTION_NOT_ESTABLISHED)
	}

	if conn != nil {
		if err := conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
			return fmt.Errorf("failed to set write deadline: %w", err)
		}
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf(ERROR_MARSHAL_MESSAGE, err)
	}

	message := fmt.Sprintf(PROTOCOL_HEADER_FORMAT, len(data), string(data))

	if _, err := writer.WriteString(message); err != nil {
		return fmt.Errorf(ERROR_WRITE_MESSAGE, err)
	}

	if err := writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush message: %w", err)
	}

	return nil
}

func (c *TCPClient) handleMessages() {
	defer func() {
		atomic.StoreInt32(&c.active, 0)
	}()

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			msg, err := c.readMessage()
			if err != nil {
				if err != io.EOF && !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "deadline") {
					fmt.Printf("TCP client error reading message: %v\n", err)
				}
				return
			}

			c.handleMessage(msg)
		}
	}
}

func (c *TCPClient) readMessage() (*JSONRPCMessage, error) {
	c.mu.RLock()
	reader := c.reader
	conn := c.conn
	c.mu.RUnlock()

	if reader == nil {
		return nil, errors.New(ERROR_TCP_CONNECTION_NOT_ESTABLISHED)
	}

	if conn != nil {
		if err := conn.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
			return nil, fmt.Errorf("failed to set read deadline: %w", err)
		}
	}

	contentLength := 0
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("failed to read header: %w", err)
		}

		line = strings.TrimSpace(line)
		if line == "" {
			break
		}

		if strings.HasPrefix(line, PROTOCOL_CONTENT_LENGTH) {
			lengthStr := strings.TrimSpace(strings.TrimPrefix(line, PROTOCOL_CONTENT_LENGTH))
			var err error
			contentLength, err = strconv.Atoi(lengthStr)
			if err != nil {
				return nil, fmt.Errorf(ERROR_INVALID_CONTENT_LENGTH, lengthStr)
			}
		}
	}

	if contentLength == 0 {
		return nil, fmt.Errorf("missing Content-Length header")
	}

	body := make([]byte, contentLength)
	if _, err := io.ReadFull(reader, body); err != nil {
		return nil, fmt.Errorf(ERROR_READ_MESSAGE_BODY, err)
	}

	var msg JSONRPCMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		return nil, fmt.Errorf("failed to parse JSON-RPC message: %w", err)
	}

	return &msg, nil
}

func (c *TCPClient) handleMessage(msg *JSONRPCMessage) {
	if msg.ID != nil {
		idStr := fmt.Sprintf("%v", msg.ID)

		c.requestMu.RLock()
		respCh, exists := c.requests[idStr]
		c.requestMu.RUnlock()

		if exists {
			if msg.Result != nil {
				if result, err := json.Marshal(msg.Result); err == nil {
					select {
					case respCh <- result:
					default:
					}
				}
			} else if msg.Error != nil {
				if errorData, err := json.Marshal(msg.Error); err == nil {
					select {
					case respCh <- errorData:
					default:
					}
				}
			}
		}
	}

}
