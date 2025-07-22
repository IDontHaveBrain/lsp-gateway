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
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

type StdioClient struct {
	config ClientConfig
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	mu       sync.RWMutex
	active   bool
	requests map[string]chan json.RawMessage
	nextID   int

	stopCh chan struct{}
	done   chan struct{}

	// Circuit breaker fields
	errorCount    int
	lastErrorTime time.Time
	circuitOpen   bool
	maxRetries    int
	baseDelay     time.Duration
}

func NewStdioClient(config ClientConfig) (*StdioClient, error) {
	return &StdioClient{
		config:      config,
		requests:    make(map[string]chan json.RawMessage),
		stopCh:      make(chan struct{}),
		done:        make(chan struct{}),
		maxRetries:  3,
		baseDelay:   100 * time.Millisecond,
		circuitOpen: false,
	}, nil
}

func (c *StdioClient) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.active {
		return fmt.Errorf("client already active")
	}

	c.cmd = exec.CommandContext(ctx, c.config.Command, c.config.Args...)

	var err error
	c.stdin, err = c.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	c.stdout, err = c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	c.stderr, err = c.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start LSP server: %w", err)
	}

	c.active = true

	go c.handleResponses()

	go c.logStderr()

	return nil
}

func (c *StdioClient) SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	c.mu.RLock()
	if !c.active {
		c.mu.RUnlock()
		return nil, errors.New(ERROR_CLIENT_NOT_ACTIVE)
	}
	c.mu.RUnlock()

	c.mu.Lock()
	c.nextID++
	id := fmt.Sprintf("req_%d", c.nextID)
	c.mu.Unlock()

	respCh := make(chan json.RawMessage, 1)

	c.mu.Lock()
	c.requests[id] = respCh
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.requests, id)
		close(respCh)
		c.mu.Unlock()
	}()

	request := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	if err := c.writeMessage(request); err != nil {
		return nil, fmt.Errorf(ERROR_SEND_REQUEST, err)
	}

	select {
	case response := <-respCh:
		return response, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.stopCh:
		return nil, errors.New(ERROR_CLIENT_STOPPED)
	}
}

func (c *StdioClient) SendNotification(ctx context.Context, method string, params interface{}) error {
	c.mu.RLock()
	if !c.active {
		c.mu.RUnlock()
		return errors.New(ERROR_CLIENT_NOT_ACTIVE)
	}
	c.mu.RUnlock()

	notification := JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}

	if err := c.writeMessage(notification); err != nil {
		return fmt.Errorf(ERROR_SEND_NOTIFICATION, err)
	}

	return nil
}

func (c *StdioClient) Stop() error {
	c.mu.Lock()
	if !c.active {
		c.mu.Unlock()
		return nil
	}

	c.active = false
	select {
	case <-c.stopCh:
		// Channel is already closed
	default:
		close(c.stopCh)
	}

	if c.stdin != nil {
		_ = c.stdin.Close()
	}

	cmd := c.cmd
	c.mu.Unlock() // Release mutex before waiting for process

	if cmd != nil && cmd.Process != nil {
		done := make(chan error, 1)
		go func() {
			done <- cmd.Wait()
		}()

		// First, try graceful shutdown with extended timeout
		select {
		case err := <-done:
			if err != nil {
				log.Printf("LSP server exited with error: %v", err)
			}
			log.Println("LSP server exited gracefully")
		case <-time.After(10 * time.Second):
			log.Println("LSP server did not exit gracefully within 10 seconds, attempting termination")

			// Try SIGTERM first (more graceful than SIGKILL)
			if err := cmd.Process.Signal(os.Interrupt); err != nil {
				log.Printf("Failed to send interrupt signal: %v", err)
			}

			// Wait a bit for graceful termination
			select {
			case <-done:
				log.Println("LSP server terminated gracefully after interrupt")
			case <-time.After(3 * time.Second):
				log.Println("LSP server did not respond to interrupt, forcing kill")
				if err := cmd.Process.Kill(); err != nil {
					log.Printf("Failed to kill LSP server process: %v", err)
				}

				// Final wait for process cleanup with extended timeout
				select {
				case <-done:
					log.Println("LSP server process was successfully killed")
				case <-time.After(5 * time.Second):
					log.Println("Warning: Process cleanup may be incomplete after kill")
				}
			}
		}
	}

	if c.stdout != nil {
		_ = c.stdout.Close()
	}
	if c.stderr != nil {
		_ = c.stderr.Close()
	}

	<-c.done

	return nil
}

func (c *StdioClient) IsActive() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.active
}

func (c *StdioClient) handleResponses() {
	defer func() {
		select {
		case <-c.done:
			// Channel is already closed
		default:
			close(c.done)
		}
	}()

	reader := bufio.NewReader(c.stdout)
	consecutiveErrors := 0
	maxConsecutiveErrors := 10

	for {
		select {
		case <-c.stopCh:
			return
		default:
		}

		// Circuit breaker check
		if c.isCircuitOpen() {
			log.Println("Circuit breaker is open, stopping message handling")
			return
		}

		message, err := c.readMessage(reader)
		if err != nil {
			if err == io.EOF {
				log.Println("LSP server closed connection")
				return
			}

			consecutiveErrors++
			c.recordError()

			log.Printf("Error reading message (consecutive: %d/%d): %v", consecutiveErrors, maxConsecutiveErrors, err)

			// Stop if too many consecutive errors
			if consecutiveErrors >= maxConsecutiveErrors {
				log.Printf("Too many consecutive errors (%d), stopping message handling", consecutiveErrors)
				c.openCircuit()
				return
			}

			// Exponential backoff with jitter
			delay := c.calculateBackoff(consecutiveErrors)
			select {
			case <-time.After(delay):
			case <-c.stopCh:
				return
			}
			continue
		}

		// Reset error count on successful read
		consecutiveErrors = 0
		c.resetErrorCount()

		var jsonrpcMsg JSONRPCMessage
		if err := json.Unmarshal(message, &jsonrpcMsg); err != nil {
			log.Printf("Error parsing JSON-RPC message: %v", err)
			consecutiveErrors++
			continue
		}

		if jsonrpcMsg.ID != nil {
			c.handleResponse(jsonrpcMsg)
		} else {
			c.handleNotification(jsonrpcMsg)
		}
	}
}

func (c *StdioClient) handleResponse(msg JSONRPCMessage) {
	if msg.ID == nil {
		return
	}

	idStr := fmt.Sprintf("%v", msg.ID)

	// Use TryRLock to avoid deadlock during shutdown
	if !c.mu.TryRLock() {
		log.Printf("Skipping response handling for ID %v due to shutdown", msg.ID)
		return
	}
	respCh, exists := c.requests[idStr]
	c.mu.RUnlock()

	if !exists {
		log.Printf("Received response for unknown request ID: %v", msg.ID)
		return
	}

	var result json.RawMessage
	if msg.Error != nil {
		errorData, _ := json.Marshal(msg.Error)
		result = errorData
	} else {
		resultData, _ := json.Marshal(msg.Result)
		result = resultData
	}

	select {
	case respCh <- result:
	default:
	}
}

func (c *StdioClient) handleNotification(msg JSONRPCMessage) {
	log.Printf("Received notification: %s", msg.Method)

}

func (c *StdioClient) writeMessage(msg JSONRPCMessage) error {
	jsonData, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf(ERROR_MARSHAL_MESSAGE, err)
	}

	content := fmt.Sprintf(PROTOCOL_HEADER_FORMAT, len(jsonData), jsonData)

	_, err = c.stdin.Write([]byte(content))
	if err != nil {
		return fmt.Errorf(ERROR_WRITE_MESSAGE, err)
	}

	return nil
}

func (c *StdioClient) readMessage(reader *bufio.Reader) ([]byte, error) {
	var contentLength int

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			break
		}

		if strings.HasPrefix(line, PROTOCOL_CONTENT_LENGTH_PREFIX) {
			lengthStr := strings.TrimPrefix(line, PROTOCOL_CONTENT_LENGTH_PREFIX)
			contentLength, err = strconv.Atoi(lengthStr)
			if err != nil {
				return nil, fmt.Errorf(ERROR_INVALID_CONTENT_LENGTH, lengthStr)
			}
		}
	}

	if contentLength == 0 {
		return nil, fmt.Errorf("no Content-Length header found")
	}

	body := make([]byte, contentLength)
	_, err := io.ReadFull(reader, body)
	if err != nil {
		return nil, fmt.Errorf("failed to read message body: %w", err)
	}

	return body, nil
}

func (c *StdioClient) logStderr() {
	scanner := bufio.NewScanner(c.stderr)
	for scanner.Scan() {
		log.Printf("LSP stderr: %s", scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		log.Printf("Error reading stderr: %v", err)
	}
}

// Circuit breaker methods - using atomic operations to avoid deadlock
func (c *StdioClient) recordError() {
	// Use TryLock to avoid deadlock during shutdown
	if c.mu.TryLock() {
		c.errorCount++
		c.lastErrorTime = time.Now()
		c.mu.Unlock()
	}
}

func (c *StdioClient) resetErrorCount() {
	// Don't take lock if we're already in a critical section
	if c.mu.TryLock() {
		c.errorCount = 0
		c.circuitOpen = false
		c.mu.Unlock()
	}
}

func (c *StdioClient) openCircuit() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.circuitOpen = true
}

func (c *StdioClient) isCircuitOpen() bool {
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

func (c *StdioClient) calculateBackoff(attempt int) time.Duration {
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
