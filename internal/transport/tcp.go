package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
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

	// Circuit breaker fields
	errorCount    int
	lastErrorTime time.Time
	circuitOpen   bool
	maxRetries    int
	baseDelay     time.Duration
}

func NewTCPClient(config ClientConfig) (LSPClient, error) {
	if config.Transport != TransportTCP {
		return nil, fmt.Errorf("invalid transport for TCP client: %s", config.Transport)
	}

	return &TCPClient{
		config:      config,
		requests:    make(map[string]chan json.RawMessage),
		maxRetries:  3,
		baseDelay:   100 * time.Millisecond,
		circuitOpen: false,
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

	// Signal shutdown first
	if c.cancel != nil {
		c.cancel()
	}

	// Set inactive to prevent new requests
	atomic.StoreInt32(&c.active, 0)

	// Clean up connection with deadline
	if c.conn != nil {
		// Set a short deadline for cleanup
		if err := c.conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
			// Log but continue with cleanup
			fmt.Printf("Failed to set connection deadline during shutdown: %v\n", err)
		}

		// Close connection
		if err := c.conn.Close(); err != nil {
			fmt.Printf("Failed to close TCP connection: %v\n", err)
		}
	}

	// Clean up pending requests with timeout
	done := make(chan struct{})
	go func() {
		c.requestMu.Lock()
		for id, ch := range c.requests {
			if ch != nil {
				select {
				case <-ch:
					// Channel is already closed
				default:
					close(ch)
				}
			}
			delete(c.requests, id)
		}
		c.requestMu.Unlock()
		close(done)
	}()

	// Wait for request cleanup with timeout
	select {
	case <-done:
		// Cleanup completed successfully
	case <-time.After(1 * time.Second):
		// Timeout - force cleanup
		fmt.Println("Warning: TCP client request cleanup timed out")
	}

	return nil
}

func (c *TCPClient) SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	if atomic.LoadInt32(&c.active) == 0 {
		return nil, errors.New(ERROR_TCP_CLIENT_NOT_ACTIVE)
	}

	id := c.generateRequestID()
	respCh := make(chan json.RawMessage, 1)

	// Register request with timeout protection
	registered := make(chan struct{})
	go func() {
		c.requestMu.Lock()
		c.requests[id] = respCh
		c.requestMu.Unlock()
		close(registered)
	}()

	select {
	case <-registered:
		// Successfully registered
	case <-time.After(100 * time.Millisecond):
		close(respCh)
		return nil, errors.New("failed to register request - client may be shutting down")
	case <-ctx.Done():
		close(respCh)
		return nil, ctx.Err()
	case <-c.ctx.Done():
		close(respCh)
		return nil, errors.New(ERROR_CLIENT_STOPPED)
	}

	// Cleanup function with proper error handling
	cleanup := func() {
		done := make(chan struct{})
		go func() {
			c.requestMu.Lock()
			if ch, exists := c.requests[id]; exists {
				delete(c.requests, id)
				if ch != nil && ch != respCh {
					// Channel was replaced, close the old one safely
					select {
					case <-ch:
						// Channel is already closed
					default:
						close(ch)
					}
				}
			}
			c.requestMu.Unlock()
			close(done)
		}()

		select {
		case <-done:
			// Cleanup completed
		case <-time.After(100 * time.Millisecond):
			// Timeout during cleanup - continue anyway
		}

		// Always close the response channel
		if respCh != nil {
			close(respCh)
		}
	}
	defer cleanup()

	request := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	if err := c.sendMessage(request); err != nil {
		return nil, fmt.Errorf(ERROR_SEND_REQUEST, err)
	}

	// Wait for response with improved timeout handling
	select {
	case response, ok := <-respCh:
		if !ok {
			return nil, errors.New("response channel was closed")
		}
		return response, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.ctx.Done():
		return nil, errors.New(ERROR_CLIENT_STOPPED)
	case <-time.After(30 * time.Second):
		// Fallback timeout to prevent hanging forever
		return nil, errors.New("request timeout: no response received within 30 seconds")
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
		if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
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

	consecutiveErrors := 0
	maxConsecutiveErrors := 10

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			// Circuit breaker check
			if c.isCircuitOpen() {
				fmt.Println("TCP client circuit breaker is open, stopping message handling")
				return
			}

			msg, err := c.readMessage()
			if err != nil {
				if err == io.EOF {
					fmt.Println("TCP connection closed")
					return
				}

				consecutiveErrors++
				c.recordError()

				if !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "deadline") {
					fmt.Printf("TCP client error reading message (consecutive: %d/%d): %v\n", consecutiveErrors, maxConsecutiveErrors, err)
				}

				// Stop if too many consecutive errors
				if consecutiveErrors >= maxConsecutiveErrors {
					fmt.Printf("TCP client: too many consecutive errors (%d), stopping message handling\n", consecutiveErrors)
					c.openCircuit()
					return
				}

				// Exponential backoff with jitter
				delay := c.calculateBackoff(consecutiveErrors)
				select {
				case <-time.After(delay):
				case <-c.ctx.Done():
					return
				}
				continue
			}

			// Reset error count on successful read
			consecutiveErrors = 0
			c.resetErrorCount()

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
		if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
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

		// Use proper synchronization with timeout to prevent deadlock
		done := make(chan struct{})
		var respCh chan json.RawMessage
		var exists bool

		go func() {
			c.requestMu.RLock()
			respCh, exists = c.requests[idStr]
			c.requestMu.RUnlock()
			close(done)
		}()

		// Wait for lock acquisition with timeout
		if c.ctx != nil {
			select {
			case <-done:
				// Lock acquired successfully
			case <-time.After(100 * time.Millisecond):
				// Timeout to avoid hanging during shutdown
				return
			case <-c.ctx.Done():
				// Client is shutting down
				return
			}
		} else {
			// No context, just wait for done or timeout
			select {
			case <-done:
				// Lock acquired successfully
			case <-time.After(100 * time.Millisecond):
				// Timeout to avoid hanging during shutdown
				return
			}
		}

		if exists && respCh != nil {
			if msg.Result != nil {
				result, err := json.Marshal(msg.Result)
				if err != nil {
					log.Printf("Failed to marshal result for TCP response: %v", err)
					// Create fallback error response
					result = []byte(`{"code":-32603,"message":"Internal error: failed to marshal result"}`)
				}

				if c.ctx != nil {
					select {
					case respCh <- result:
					case <-time.After(100 * time.Millisecond):
						// Channel may be closed or full
					case <-c.ctx.Done():
						// Client is shutting down
						return
					}
				} else {
					// No context, simpler select
					select {
					case respCh <- result:
					case <-time.After(100 * time.Millisecond):
						// Channel may be closed or full
					}
				}
			}
		} else if msg.Error != nil {
			errorData, err := json.Marshal(msg.Error)
			if err != nil {
				log.Printf("Failed to marshal error for TCP response: %v", err)
				// Create fallback error response
				errorData = []byte(`{"code":-32603,"message":"Internal error: failed to marshal error"}`)
			}

			if c.ctx != nil {
				select {
				case respCh <- errorData:
				case <-time.After(100 * time.Millisecond):
					// Channel may be closed or full
				case <-c.ctx.Done():
					// Client is shutting down
					return
				}
			} else {
				// No context, simpler select
				select {
				case respCh <- errorData:
				case <-time.After(100 * time.Millisecond):
					// Channel may be closed or full
				}
			}
		}
	}
}

// Circuit breaker methods for TCP client - using atomic operations and timeout-based locking
func (c *TCPClient) recordError() {
	// Use goroutine with timeout to avoid deadlock during shutdown
	done := make(chan struct{})
	go func() {
		c.mu.Lock()
		c.errorCount++
		c.lastErrorTime = time.Now()
		c.mu.Unlock()
		close(done)
	}()

	if c.ctx != nil {
		select {
		case <-done:
			// Successfully recorded error
		case <-time.After(50 * time.Millisecond):
			// Timeout to prevent hanging during shutdown
		case <-c.ctx.Done():
			// Client is shutting down
		}
	} else {
		select {
		case <-done:
			// Successfully recorded error
		case <-time.After(50 * time.Millisecond):
			// Timeout to prevent hanging during shutdown
		}
	}
}

func (c *TCPClient) resetErrorCount() {
	// Use goroutine with timeout to avoid deadlock during shutdown
	done := make(chan struct{})
	go func() {
		c.mu.Lock()
		c.errorCount = 0
		c.circuitOpen = false
		c.mu.Unlock()
		close(done)
	}()

	if c.ctx != nil {
		select {
		case <-done:
			// Successfully reset error count
		case <-time.After(50 * time.Millisecond):
			// Timeout to prevent hanging during shutdown
		case <-c.ctx.Done():
			// Client is shutting down
		}
	} else {
		select {
		case <-done:
			// Successfully reset error count
		case <-time.After(50 * time.Millisecond):
			// Timeout to prevent hanging during shutdown
		}
	}
}

func (c *TCPClient) openCircuit() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.circuitOpen = true
}

func (c *TCPClient) isCircuitOpen() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Circuit breaker: open if too many errors in short time
	if c.circuitOpen {
		// Try to close circuit after 30 seconds
		if time.Since(c.lastErrorTime) > 30*time.Second {
			c.circuitOpen = false
			c.errorCount = 0
		}
	}

	return c.circuitOpen || c.errorCount > c.maxRetries
}

func (c *TCPClient) calculateBackoff(attempt int) time.Duration {
	// Exponential backoff with jitter
	backoff := float64(c.baseDelay) * math.Pow(2, float64(attempt-1))

	// Add jitter (±25%)
	jitter := 1.0 + (rand.Float64()-0.5)*0.5
	delay := time.Duration(backoff * jitter)

	// Cap at 5 seconds
	if delay > 5*time.Second {
		delay = 5 * time.Second
	}

	return delay
}
