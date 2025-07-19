package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// TCPClient implements LSP communication over TCP
type TCPClient struct {
	config ClientConfig
	conn   net.Conn
	
	// Connection management
	mu       sync.RWMutex
	active   int32 // atomic flag for active state
	
	// Request handling
	requests map[string]chan json.RawMessage
	requestMu sync.RWMutex
	nextID   int64
	
	// I/O handling
	reader *bufio.Reader
	writer *bufio.Writer
	
	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc
}

// NewTCPClient creates a new TCP-based LSP client
func NewTCPClient(config ClientConfig) (LSPClient, error) {
	if config.Transport != TransportTCP {
		return nil, fmt.Errorf("invalid transport for TCP client: %s", config.Transport)
	}
	
	return &TCPClient{
		config:   config,
		requests: make(map[string]chan json.RawMessage),
	}, nil
}

// Start establishes the TCP connection and starts the LSP server
func (c *TCPClient) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if atomic.LoadInt32(&c.active) != 0 {
		return fmt.Errorf("TCP client already active")
	}
	
	// Create context for this client
	c.ctx, c.cancel = context.WithCancel(ctx)
	
	// Parse TCP address from command and args
	address, err := c.parseAddress()
	if err != nil {
		return fmt.Errorf("failed to parse TCP address: %w", err)
	}
	
	// Establish TCP connection
	conn, err := net.DialTimeout("tcp", address, 10*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to TCP server at %s: %w", address, err)
	}
	
	c.conn = conn
	c.reader = bufio.NewReader(conn)
	c.writer = bufio.NewWriter(conn)
	
	// Mark as active
	atomic.StoreInt32(&c.active, 1)
	
	// Start message handler
	go c.handleMessages()
	
	return nil
}

// Stop closes the TCP connection and cleans up resources
func (c *TCPClient) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if atomic.LoadInt32(&c.active) == 0 {
		return nil // Already stopped
	}
	
	// Cancel context to stop message handler
	if c.cancel != nil {
		c.cancel()
	}
	
	// Close connection
	if c.conn != nil {
		_ = c.conn.Close()
	}
	
	// Clear pending requests
	c.requestMu.Lock()
	for id, ch := range c.requests {
		close(ch)
		delete(c.requests, id)
	}
	c.requestMu.Unlock()
	
	// Mark as inactive
	atomic.StoreInt32(&c.active, 0)
	
	return nil
}

// SendRequest sends a JSON-RPC request and waits for response
func (c *TCPClient) SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	if atomic.LoadInt32(&c.active) == 0 {
		return nil, fmt.Errorf("TCP client not active")
	}
	
	// Generate request ID
	id := c.generateRequestID()
	
	// Create response channel
	respCh := make(chan json.RawMessage, 1)
	
	// Register request
	c.requestMu.Lock()
	c.requests[id] = respCh
	c.requestMu.Unlock()
	
	// Clean up on exit
	defer func() {
		c.requestMu.Lock()
		delete(c.requests, id)
		c.requestMu.Unlock()
		close(respCh)
	}()
	
	// Create request message
	request := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}
	
	// Send request
	if err := c.sendMessage(request); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	
	// Wait for response
	select {
	case response := <-respCh:
		return response, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.ctx.Done():
		return nil, fmt.Errorf("client stopped")
	}
}

// SendNotification sends a JSON-RPC notification (no response expected)
func (c *TCPClient) SendNotification(ctx context.Context, method string, params interface{}) error {
	if atomic.LoadInt32(&c.active) == 0 {
		return fmt.Errorf("TCP client not active")
	}
	
	// Create notification message (no ID)
	notification := JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
	
	// Send notification
	if err := c.sendMessage(notification); err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}
	
	return nil
}

// IsActive returns true if the client is active and connected
func (c *TCPClient) IsActive() bool {
	return atomic.LoadInt32(&c.active) != 0
}

// parseAddress extracts the TCP address from the command and args
func (c *TCPClient) parseAddress() (string, error) {
	// For TCP clients, the "command" field should contain the address
	// Format: host:port or just port (defaults to localhost)
	address := c.config.Command
	
	if address == "" {
		return "", fmt.Errorf("TCP address not specified in command field")
	}
	
	// If it's just a port number, prepend localhost
	if _, err := strconv.Atoi(address); err == nil {
		address = "localhost:" + address
	}
	
	// Validate address format
	if !strings.Contains(address, ":") {
		return "", fmt.Errorf("invalid TCP address format: %s (expected host:port)", address)
	}
	
	return address, nil
}

// generateRequestID generates a unique request ID
func (c *TCPClient) generateRequestID() string {
	id := atomic.AddInt64(&c.nextID, 1)
	return strconv.FormatInt(id, 10)
}

// sendMessage sends a JSON-RPC message over TCP
func (c *TCPClient) sendMessage(msg JSONRPCMessage) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	if c.writer == nil {
		return fmt.Errorf("TCP connection not established")
	}
	
	// Marshal message to JSON
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}
	
	// Create LSP message with Content-Length header
	message := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(data), string(data))
	
	// Write message
	if _, err := c.writer.WriteString(message); err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}
	
	// Flush buffer
	if err := c.writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush message: %w", err)
	}
	
	return nil
}

// handleMessages handles incoming messages from the TCP connection
func (c *TCPClient) handleMessages() {
	defer func() {
		// Clean up on exit
		_ = c.Stop()
	}()
	
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			// Read message
			msg, err := c.readMessage()
			if err != nil {
				if err != io.EOF {
					// Log error but continue
					fmt.Printf("TCP client error reading message: %v\n", err)
				}
				return
			}
			
			// Handle message
			c.handleMessage(msg)
		}
	}
}

// readMessage reads a single LSP message from the TCP connection
func (c *TCPClient) readMessage() (*JSONRPCMessage, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	if c.reader == nil {
		return nil, fmt.Errorf("TCP connection not established")
	}
	
	// Read Content-Length header
	contentLength := 0
	for {
		line, err := c.reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("failed to read header: %w", err)
		}
		
		line = strings.TrimSpace(line)
		if line == "" {
			// Empty line indicates end of headers
			break
		}
		
		if strings.HasPrefix(line, "Content-Length:") {
			lengthStr := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
			var err error
			contentLength, err = strconv.Atoi(lengthStr)
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Length: %s", lengthStr)
			}
		}
	}
	
	if contentLength == 0 {
		return nil, fmt.Errorf("missing Content-Length header")
	}
	
	// Read message body
	body := make([]byte, contentLength)
	if _, err := io.ReadFull(c.reader, body); err != nil {
		return nil, fmt.Errorf("failed to read message body: %w", err)
	}
	
	// Parse JSON-RPC message
	var msg JSONRPCMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		return nil, fmt.Errorf("failed to parse JSON-RPC message: %w", err)
	}
	
	return &msg, nil
}

// handleMessage processes an incoming JSON-RPC message
func (c *TCPClient) handleMessage(msg *JSONRPCMessage) {
	// Handle responses (messages with ID)
	if msg.ID != nil {
		idStr := fmt.Sprintf("%v", msg.ID)
		
		c.requestMu.RLock()
		respCh, exists := c.requests[idStr]
		c.requestMu.RUnlock()
		
		if exists {
			// Send response to waiting request
			if msg.Result != nil {
				if result, err := json.Marshal(msg.Result); err == nil {
					select {
					case respCh <- result:
					default:
						// Channel full or closed, ignore
					}
				}
			} else if msg.Error != nil {
				// For errors, we might want to send the error as JSON
				if errorData, err := json.Marshal(msg.Error); err == nil {
					select {
					case respCh <- errorData:
					default:
						// Channel full or closed, ignore
					}
				}
			}
		}
	}
	
	// Handle notifications (messages without ID)
	// For now, we just ignore them, but they could be logged or processed
}