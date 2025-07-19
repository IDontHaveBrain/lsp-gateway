package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// StdioClient implements LSP communication over STDIO
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
}

// NewStdioClient creates a new STDIO-based LSP client
func NewStdioClient(config ClientConfig) (*StdioClient, error) {
	return &StdioClient{
		config:   config,
		requests: make(map[string]chan json.RawMessage),
		stopCh:   make(chan struct{}),
		done:     make(chan struct{}),
	}, nil
}

// Start starts the LSP server process
func (c *StdioClient) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.active {
		return fmt.Errorf("client already active")
	}

	// Start LSP server process
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

	// Start response handler
	go c.handleResponses()

	// Start stderr logger
	go c.logStderr()

	return nil
}

// SendRequest sends a JSON-RPC request and waits for response
func (c *StdioClient) SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	c.mu.RLock()
	if !c.active {
		c.mu.RUnlock()
		return nil, fmt.Errorf("client not active")
	}
	c.mu.RUnlock()

	// Generate unique request ID
	c.mu.Lock()
	c.nextID++
	id := fmt.Sprintf("req_%d", c.nextID)
	c.mu.Unlock()

	// Create response channel
	respCh := make(chan json.RawMessage, 1)

	c.mu.Lock()
	c.requests[id] = respCh
	c.mu.Unlock()

	// Cleanup on function exit
	defer func() {
		c.mu.Lock()
		delete(c.requests, id)
		close(respCh)
		c.mu.Unlock()
	}()

	// Create JSON-RPC request
	request := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	// Send request
	if err := c.writeMessage(request); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Wait for response
	select {
	case response := <-respCh:
		return response, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.stopCh:
		return nil, fmt.Errorf("client stopped")
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("request timeout")
	}
}

// SendNotification sends a JSON-RPC notification (no response expected)
func (c *StdioClient) SendNotification(ctx context.Context, method string, params interface{}) error {
	c.mu.RLock()
	if !c.active {
		c.mu.RUnlock()
		return fmt.Errorf("client not active")
	}
	c.mu.RUnlock()

	// Create JSON-RPC notification (no ID)
	notification := JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}

	// Send notification
	if err := c.writeMessage(notification); err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}

	return nil
}

// Stop shuts down the LSP server process
func (c *StdioClient) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.active {
		return nil
	}

	c.active = false

	// Signal stop to goroutines
	close(c.stopCh)

	// Close stdin to signal the process to stop
	if c.stdin != nil {
		_ = c.stdin.Close()
	}

	// Wait for process to exit or kill it
	if c.cmd != nil && c.cmd.Process != nil {
		done := make(chan error, 1)
		go func() {
			done <- c.cmd.Wait()
		}()

		select {
		case err := <-done:
			if err != nil {
				log.Printf("LSP server exited with error: %v", err)
			}
		case <-time.After(5 * time.Second):
			log.Println("LSP server did not exit gracefully, killing process")
			if err := c.cmd.Process.Kill(); err != nil {
				log.Printf("Failed to kill LSP server process: %v", err)
			}
		}
	}

	// Close pipes
	if c.stdout != nil {
		_ = c.stdout.Close()
	}
	if c.stderr != nil {
		_ = c.stderr.Close()
	}

	// Wait for goroutines to finish
	<-c.done

	return nil
}

// IsActive returns whether the client is active
func (c *StdioClient) IsActive() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.active
}

// handleResponses handles incoming messages from the LSP server
func (c *StdioClient) handleResponses() {
	defer close(c.done)

	reader := bufio.NewReader(c.stdout)

	for {
		select {
		case <-c.stopCh:
			return
		default:
		}

		// Read LSP message (Content-Length header + JSON body)
		message, err := c.readMessage(reader)
		if err != nil {
			if err == io.EOF {
				log.Println("LSP server closed connection")
				return
			}
			log.Printf("Error reading message: %v", err)
			continue
		}

		// Parse JSON-RPC message
		var jsonrpcMsg JSONRPCMessage
		if err := json.Unmarshal(message, &jsonrpcMsg); err != nil {
			log.Printf("Error parsing JSON-RPC message: %v", err)
			continue
		}

		// Handle response or notification
		if jsonrpcMsg.ID != nil {
			// This is a response to a request
			c.handleResponse(jsonrpcMsg)
		} else {
			// This is a notification from the server
			c.handleNotification(jsonrpcMsg)
		}
	}
}

// handleResponse handles a response to a request
func (c *StdioClient) handleResponse(msg JSONRPCMessage) {
	if msg.ID == nil {
		return
	}

	idStr := fmt.Sprintf("%v", msg.ID)

	c.mu.RLock()
	respCh, exists := c.requests[idStr]
	c.mu.RUnlock()

	if !exists {
		log.Printf("Received response for unknown request ID: %v", msg.ID)
		return
	}

	// Send response to waiting goroutine
	var result json.RawMessage
	if msg.Error != nil {
		// Convert error to JSON
		errorData, _ := json.Marshal(msg.Error)
		result = errorData
	} else {
		// Convert result to JSON
		resultData, _ := json.Marshal(msg.Result)
		result = resultData
	}

	select {
	case respCh <- result:
	default:
		// Channel is full or closed
	}
}

// handleNotification handles a notification from the server
func (c *StdioClient) handleNotification(msg JSONRPCMessage) {
	// Log notifications for debugging
	log.Printf("Received notification: %s", msg.Method)

	// In a full implementation, you might want to handle specific notifications
	// like textDocument/publishDiagnostics, window/showMessage, etc.
}

// writeMessage writes a JSON-RPC message to the LSP server
func (c *StdioClient) writeMessage(msg JSONRPCMessage) error {
	// Marshal message to JSON
	jsonData, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Create LSP message with Content-Length header
	content := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(jsonData), jsonData)

	// Write to stdin
	_, err = c.stdin.Write([]byte(content))
	if err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	return nil
}

// readMessage reads a complete LSP message from the reader
func (c *StdioClient) readMessage(reader *bufio.Reader) ([]byte, error) {
	// Read Content-Length header
	var contentLength int

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			// Empty line indicates end of headers
			break
		}

		if strings.HasPrefix(line, "Content-Length: ") {
			lengthStr := strings.TrimPrefix(line, "Content-Length: ")
			contentLength, err = strconv.Atoi(lengthStr)
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Length: %s", lengthStr)
			}
		}
	}

	if contentLength == 0 {
		return nil, fmt.Errorf("no Content-Length header found")
	}

	// Read message body
	body := make([]byte, contentLength)
	_, err := io.ReadFull(reader, body)
	if err != nil {
		return nil, fmt.Errorf("failed to read message body: %w", err)
	}

	return body, nil
}

// logStderr logs stderr output from the LSP server
func (c *StdioClient) logStderr() {
	scanner := bufio.NewScanner(c.stderr)
	for scanner.Scan() {
		log.Printf("LSP stderr: %s", scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		log.Printf("Error reading stderr: %v", err)
	}
}
