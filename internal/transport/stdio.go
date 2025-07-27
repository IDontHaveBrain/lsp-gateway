package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"lsp-gateway/internal/installer"
	"lsp-gateway/internal/types"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// pendingRequest stores context for pending LSP requests to enable SCIP indexing
type pendingRequest struct {
	respCh chan json.RawMessage
	method string
	params interface{}
}

type StdioClient struct {
	config ClientConfig
	cmd    *exec.Cmd
	Stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	mu       sync.RWMutex
	active   bool
	requests map[string]*pendingRequest
	nextID   int

	stopCh chan struct{}
	Done   chan struct{}

	// Circuit breaker fields
	errorCount    int
	lastErrorTime time.Time
	circuitOpen   bool
	maxRetries    int
	baseDelay     time.Duration

	// SCIP indexer for background response caching
	scipIndexer *SafeIndexerWrapper
}

func NewStdioClient(config ClientConfig) (*StdioClient, error) {
	client, err := NewStdioClientWithSCIP(config, nil)
	if err != nil {
		return nil, err
	}
	return client.(*StdioClient), nil
}

// NewStdioClientWithSCIP creates a new STDIO client with optional SCIP indexer support.
// Pass nil for scipIndexer to disable SCIP indexing.
func NewStdioClientWithSCIP(config ClientConfig, scipIndexer SCIPIndexer) (LSPClient, error) {
	var scipWrapper *SafeIndexerWrapper
	if scipIndexer != nil {
		scipWrapper = NewSafeIndexerWrapper(scipIndexer, 10) // Default max 10 concurrent indexing goroutines
	}

	return &StdioClient{
		config:      config,
		requests:    make(map[string]*pendingRequest),
		stopCh:      make(chan struct{}),
		Done:        make(chan struct{}),
		maxRetries:  5,
		baseDelay:   100 * time.Millisecond,
		circuitOpen: false,
		scipIndexer: scipWrapper,
	}, nil
}

func (c *StdioClient) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.active {
		return fmt.Errorf("client already active")
	}

	// Check if this is JDTLS and needs auto-installation
	if c.isJDTLSCommand() && !c.isJDTLSInstalled() {
		log.Printf("[INFO] JDTLS not found at %s, attempting auto-installation...", c.config.Command)
		if err := c.installJDTLS(); err != nil {
			return fmt.Errorf("failed to auto-install JDTLS: %w", err)
		}
		log.Printf("[INFO] JDTLS installation completed successfully")
	}

	c.cmd = exec.CommandContext(ctx, c.config.Command, c.config.Args...)

	var err error
	c.Stdin, err = c.cmd.StdinPipe()
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

	// Store request context for SCIP indexing
	pending := &pendingRequest{
		respCh: respCh,
		method: method,
		params: params,
	}

	c.mu.Lock()
	c.requests[id] = pending
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

	if err := c.WriteMessage(request); err != nil {
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

	if err := c.WriteMessage(notification); err != nil {
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

	if c.Stdin != nil {
		_ = c.Stdin.Close()
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
		case <-time.After(8 * time.Second):
			log.Println("LSP server did not exit gracefully within 8 seconds, attempting termination")

			// Try SIGTERM first (more graceful than SIGKILL)
			if err := cmd.Process.Signal(os.Interrupt); err != nil {
				log.Printf("Failed to send interrupt signal: %v", err)
			}

			// Wait a bit for graceful termination
			select {
			case <-done:
				log.Println("LSP server terminated gracefully after interrupt")
			case <-time.After(2 * time.Second):
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

	// Clean up SCIP indexer if present
	if c.scipIndexer != nil {
		if err := c.scipIndexer.Close(); err != nil {
			log.Printf("Error closing SCIP indexer: %v", err)
		}
	}

	<-c.Done

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
		case <-c.Done:
			// Channel is already closed
		default:
			close(c.Done)
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

		message, err := c.ReadMessage(reader)
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

	// Use regular RLock with timeout to handle potential deadlocks
	done := make(chan struct{})
	var pending *pendingRequest
	var exists bool

	go func() {
		defer close(done)
		c.mu.RLock()
		defer c.mu.RUnlock()
		pending, exists = c.requests[idStr]
	}()

	// Wait for lock acquisition with timeout
	select {
	case <-done:
		// Lock acquired successfully
	case <-time.After(50 * time.Millisecond):
		log.Printf("Timeout acquiring lock for response ID %v, skipping", msg.ID)
		return
	case <-c.stopCh:
		log.Printf("Client stopping, skipping response for ID %v", msg.ID)
		return
	}

	if !exists || pending == nil {
		log.Printf("Received response for unknown request ID: %v", msg.ID)
		return
	}

	var result json.RawMessage
	if msg.Error != nil {
		errorData, err := json.Marshal(msg.Error)
		if err != nil {
			log.Printf("Failed to marshal error response for ID %v: %v", msg.ID, err)
			// Create fallback error response
			fallbackError := map[string]interface{}{
				"code":    -32603,
				"message": "Internal error: failed to marshal response",
			}
			if errorData, fallbackErr := json.Marshal(fallbackError); fallbackErr == nil {
				result = errorData
			} else {
				result = []byte(`{"code":-32603,"message":"Internal error"}`)
			}
		} else {
			result = errorData
		}
	} else {
		resultData, err := json.Marshal(msg.Result)
		if err != nil {
			log.Printf("Failed to marshal result response for ID %v: %v", msg.ID, err)
			// Create fallback error response
			fallbackError := map[string]interface{}{
				"code":    -32603,
				"message": "Internal error: failed to marshal result",
			}
			if errorData, fallbackErr := json.Marshal(fallbackError); fallbackErr == nil {
				result = errorData
			} else {
				result = []byte(`{"code":-32603,"message":"Internal error"}`)
			}
		} else {
			result = resultData
		}
	}

	select {
	case pending.respCh <- result:
		// SCIP indexing hook - non-blocking background processing
		if c.scipIndexer != nil && IsCacheableMethod(pending.method) {
			c.scipIndexer.SafeIndexResponse(pending.method, pending.params, result, idStr)
		}
	default:
		// Channel full or closed - request may have timed out
	}
}

func (c *StdioClient) handleNotification(msg JSONRPCMessage) {
	log.Printf("Received notification: %s", msg.Method)

}

func (c *StdioClient) WriteMessage(msg JSONRPCMessage) error {
	jsonData, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf(ERROR_MARSHAL_MESSAGE, err)
	}

	content := fmt.Sprintf(PROTOCOL_HEADER_FORMAT, len(jsonData), jsonData)

	_, err = c.Stdin.Write([]byte(content))
	if err != nil {
		return fmt.Errorf(ERROR_WRITE_MESSAGE, err)
	}

	return nil
}

func (c *StdioClient) ReadMessage(reader *bufio.Reader) ([]byte, error) {
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

// Circuit breaker methods - using timeout-based locking to avoid deadlock
func (c *StdioClient) recordError() {
	// Use timeout-based locking to avoid deadlock during shutdown
	done := make(chan struct{})

	go func() {
		defer close(done)
		c.mu.Lock()
		c.errorCount++
		c.lastErrorTime = time.Now()
		c.mu.Unlock()
	}()

	select {
	case <-done:
		// Lock acquired and error recorded
	case <-time.After(50 * time.Millisecond):
		// Timeout - log but don't block
		log.Printf("Timeout recording error, continuing")
	case <-c.stopCh:
		// Client stopping
		return
	}
}

func (c *StdioClient) resetErrorCount() {
	// Use timeout-based locking to avoid deadlock
	done := make(chan struct{})

	go func() {
		defer close(done)
		c.mu.Lock()
		c.errorCount = 0
		c.circuitOpen = false
		c.mu.Unlock()
	}()

	select {
	case <-done:
		// Lock acquired and count reset
	case <-time.After(50 * time.Millisecond):
		// Timeout - log but don't block
		log.Printf("Timeout resetting error count, continuing")
	case <-c.stopCh:
		// Client stopping
		return
	}
}

func (c *StdioClient) openCircuit() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.circuitOpen = true
	c.lastErrorTime = time.Now()
}

func (c *StdioClient) isCircuitOpen() bool {
	// Use timeout-based locking for consistency
	done := make(chan bool, 1)

	go func() {
		c.mu.RLock()
		defer c.mu.RUnlock()

		// Circuit breaker: open if too many errors in short time
		if c.circuitOpen {
			// Try to close circuit after 3 seconds for better recovery balance
			if time.Since(c.lastErrorTime) > 3*time.Second {
				c.circuitOpen = false
				c.errorCount = 0
			}
		}

		done <- c.circuitOpen || c.errorCount > c.maxRetries
	}()

	select {
	case result := <-done:
		return result
	case <-time.After(50 * time.Millisecond):
		// Timeout - assume circuit is closed to allow operation
		log.Printf("Timeout checking circuit breaker state, assuming closed")
		return false
	case <-c.stopCh:
		// Client stopping - circuit is effectively open
		return true
	}
}

func (c *StdioClient) calculateBackoff(attempt int) time.Duration {
	// Less aggressive exponential backoff with better jitter
	// Use base of 1.5 instead of 2 for more gradual increase
	backoff := float64(c.baseDelay) * math.Pow(1.5, float64(attempt-1))

	// Add better jitter (±30% for more variance)
	jitter := 1.0 + (rand.Float64()-0.5)*0.6
	delay := time.Duration(backoff * jitter)

	// Cap at 3 seconds for faster recovery
	if delay > 3*time.Second {
		delay = 3 * time.Second
	}

	return delay
}

// Test-only exported methods for circuit breaker testing
// These methods are provided to allow integration tests to verify
// circuit breaker behavior without exposing internal implementation details
// in the production API.

// RecordErrorForTesting records an error for testing purposes
func (c *StdioClient) RecordErrorForTesting() {
	c.recordError()
}

// ResetErrorCountForTesting resets the error count for testing purposes
func (c *StdioClient) ResetErrorCountForTesting() {
	c.resetErrorCount()
}

// IsCircuitOpenForTesting checks if the circuit is open for testing purposes
func (c *StdioClient) IsCircuitOpenForTesting() bool {
	return c.isCircuitOpen()
}

// OpenCircuitForTesting opens the circuit for testing purposes
func (c *StdioClient) OpenCircuitForTesting() {
	c.openCircuit()
}

// SetMaxRetriesForTesting sets the max retries for testing purposes
func (c *StdioClient) SetMaxRetriesForTesting(retries int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.maxRetries = retries
}

// GetErrorCountForTesting gets the error count for testing purposes
func (c *StdioClient) GetErrorCountForTesting() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.errorCount
}

// SetLastErrorTimeForTesting sets the last error time for testing purposes
func (c *StdioClient) SetLastErrorTimeForTesting(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastErrorTime = t
}

// CalculateBackoffForTesting calculates backoff for testing purposes
func (c *StdioClient) CalculateBackoffForTesting(attempt int) time.Duration {
	return c.calculateBackoff(attempt)
}

// GetProcessPIDForTesting returns the process PID for testing purposes
func (c *StdioClient) GetProcessPIDForTesting() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.cmd != nil && c.cmd.Process != nil {
		return c.cmd.Process.Pid
	}
	return -1
}

// isJDTLSCommand checks if this client is for JDTLS
func (c *StdioClient) isJDTLSCommand() bool {
	// Check if the command contains "jdtls" in the path
	return strings.Contains(c.config.Command, "jdtls")
}

// isJDTLSInstalled checks if JDTLS is already installed
func (c *StdioClient) isJDTLSInstalled() bool {
	// Check if the command path exists
	_, err := os.Stat(c.config.Command)
	return err == nil
}

// installJDTLS installs JDTLS and Java 21 runtime if needed
func (c *StdioClient) installJDTLS() error {
	// Create universal server strategy for installation
	strategy := installer.NewUniversalServerStrategy()
	
	// Install JDTLS with auto-installation of Java 21
	options := types.ServerInstallOptions{
		Force:   false, // Don't force reinstall if already present
		Timeout: 5 * time.Minute, // Give enough time for download
	}
	
	result, err := strategy.InstallServer(installer.ServerJDTLS, options)
	if err != nil {
		return fmt.Errorf("JDTLS installation failed: %w", err)
	}
	
	if !result.Success {
		return fmt.Errorf("JDTLS installation failed: %v", result.Errors)
	}
	
	log.Printf("[INFO] JDTLS installed successfully at: %s", result.Path)
	return nil
}
