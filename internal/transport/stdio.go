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
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
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
	
	// Response handler supervision
	responseHandlerActive bool
	responseHandlerMu     sync.Mutex
	lastResponseTime      time.Time
	
	// JDTLS installation state
	jdtlsInstalling    bool
	jdtlsInstallDone   chan struct{}
	jdtlsInstallError  error
	jdtlsInstallMu     sync.RWMutex
	
	// Workspace loading tracking for gopls and similar servers
	workspaceReady    bool
	workspaceReadyCh  chan struct{}
	workspaceReadyMu  sync.RWMutex
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
		config:         config,
		requests:       make(map[string]*pendingRequest),
		stopCh:         make(chan struct{}),
		Done:           make(chan struct{}),
		maxRetries:     5,
		baseDelay:      100 * time.Millisecond,
		circuitOpen:    false,
		scipIndexer:    scipWrapper,
		jdtlsInstallDone: make(chan struct{}),
		workspaceReadyCh: make(chan struct{}),
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
		log.Printf("[INFO] JDTLS not found at %s, starting background auto-installation...", c.config.Command)
		c.jdtlsInstallMu.Lock()
		c.jdtlsInstalling = true
		c.jdtlsInstallMu.Unlock()
		
		// Start JDTLS installation in background to avoid blocking Start()
		go c.installJDTLSAsync()
		
		// For now, try to proceed with startup - requests will wait for installation if needed
		log.Printf("[INFO] JDTLS installation started in background, proceeding with LSP server startup")
	} else {
		// Mark installation as done if not needed
		close(c.jdtlsInstallDone)
	}

	c.cmd = exec.CommandContext(ctx, c.config.Command, c.config.Args...)

	// Create pipes with timeout protection to prevent blocking
	var err error
	if err = c.createPipesWithTimeout(5 * time.Second); err != nil {
		return fmt.Errorf("failed to create pipes with timeout: %w", err)
	}

	// Start LSP server process with timeout protection
	if err := c.startProcessWithTimeout(10 * time.Second); err != nil {
		return fmt.Errorf("failed to start LSP server with timeout: %w", err)
	}

	log.Printf("[DEBUG-STDIO] LSP server process started: %s %v (PID: %d)", c.config.Command, c.config.Args, c.cmd.Process.Pid)
	c.active = true

	// Initialize response handler supervision fields
	c.responseHandlerMu.Lock()
	c.responseHandlerActive = false
	c.lastResponseTime = time.Now()
	c.responseHandlerMu.Unlock()
	
	// Start supervised response handler
	go c.superviseResponseHandler()
	
	// Start process monitoring
	go c.monitorProcess()

	go c.logStderr()

	// Initialize LSP server in a separate goroutine to avoid blocking Start()
	go func() {
		// Give the response handler some time to start
		time.Sleep(100 * time.Millisecond)
		
		// Create a context with timeout for the entire initialization
		initCtx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
		defer cancel()
		
		if err := c.initializeLSPServer(initCtx); err != nil {
			log.Printf("LSP server initialization failed: %v", err)
			// Don't stop the client - it can still be used for basic operations
		} else {
			log.Printf("LSP server initialization completed successfully")
		}
	}()

	return nil
}

func (c *StdioClient) SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	log.Printf("[DEBUG-STDIO] SendRequest ENTRY: method=%s, active=%v", method, c.active)
	
	// Wait for JDTLS installation if needed before proceeding
	if err := c.waitForJDTLSInstallation(ctx); err != nil {
		log.Printf("[WARN-STDIO] JDTLS installation issue: %v, proceeding anyway", err)
		// Continue with request even if JDTLS installation failed
	}
	
	// Wait for workspace indexing to complete for workspace-dependent methods
	if err := c.waitForWorkspaceReady(ctx, method); err != nil {
		log.Printf("[ERROR-STDIO] Workspace not ready for method %s: %v", method, err)
		return nil, fmt.Errorf("workspace not ready: %w", err)
	}
	
	c.mu.RLock()
	if !c.active {
		c.mu.RUnlock()
		log.Printf("[ERROR-STDIO] Client not active")
		return nil, errors.New(ERROR_CLIENT_NOT_ACTIVE)
	}
	c.mu.RUnlock()

	c.mu.Lock()
	c.nextID++
	// Use a more unique ID format to avoid conflicts
	id := fmt.Sprintf("mcp_req_%d_%d", time.Now().UnixNano()%1000000, c.nextID)
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

	log.Printf("[DEBUG-STDIO] About to send message to LSP server, id=%s", id)
	if err := c.WriteMessage(request); err != nil {
		log.Printf("[ERROR-STDIO] Failed to write message: %v", err)
		return nil, fmt.Errorf(ERROR_SEND_REQUEST, err)
	}
	log.Printf("[DEBUG-STDIO] Message sent successfully, now waiting for response...")

	log.Printf("[DEBUG-STDIO] Starting select wait - infinite wait may occur here")
	select {
	case response := <-respCh:
		log.Printf("[DEBUG-STDIO] Received response from respCh: %d bytes", len(response))
		return response, nil
	case <-ctx.Done():
		log.Printf("[DEBUG-STDIO] Context timeout/cancellation: %v", ctx.Err())
		return nil, ctx.Err()
	case <-c.stopCh:
		log.Printf("[DEBUG-STDIO] Client stopped")
		return nil, errors.New(ERROR_CLIENT_STOPPED)
	case <-time.After(35 * time.Second):
		log.Printf("[ERROR-STDIO] HARD TIMEOUT: No response after 35 seconds - LSP server may not be responding")
		return nil, fmt.Errorf("LSP request timeout: no response after 35 seconds (method: %s)", method)
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
	log.Printf("[DEBUG-STDIO] handleResponse: received response for ID %s", idStr)

	// Simplified lock handling - use direct mutex access
	c.mu.RLock()
	pending, exists := c.requests[idStr]
	c.mu.RUnlock()

	if !exists || pending == nil {
		log.Printf("[ERROR-STDIO] Received response for unknown request ID: %v", msg.ID)
		return
	}
	log.Printf("[DEBUG-STDIO] Found matching pending request for ID %s", idStr)

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

	// Critical Fix: Use blocking delivery with timeout to prevent response dropping
	select {
	case pending.respCh <- result:
		// SCIP indexing hook - non-blocking background processing
		if c.scipIndexer != nil && IsCacheableMethod(pending.method) {
			c.scipIndexer.SafeIndexResponse(pending.method, pending.params, result, idStr)
		}
		
		// Update response handler activity timestamp for supervision
		c.responseHandlerMu.Lock()
		c.lastResponseTime = time.Now()
		c.responseHandlerMu.Unlock()
		
		log.Printf("[DEBUG-STDIO] Successfully delivered response for ID %s", idStr)
	case <-time.After(3 * time.Second):
		log.Printf("[ERROR-STDIO] Failed to deliver response for ID %s: channel blocked for 3 seconds", idStr)
		// Response not delivered - this prevents infinite waiting
		// The requesting goroutine will eventually timeout via its own mechanisms
	}
}

func (c *StdioClient) handleNotification(msg JSONRPCMessage) {
	log.Printf("Received notification: %s", msg.Method)

	// Handle workspace loading completion for gopls
	if msg.Method == "window/showMessage" {
		log.Printf("[DEBUG-WORKSPACE] Processing window/showMessage notification")
		if params, ok := msg.Params.(map[string]interface{}); ok {
			if message, ok := params["message"].(string); ok {
				log.Printf("[DEBUG-WORKSPACE] window/showMessage: %s", message)
				
				// Detect when gopls finishes loading packages
				if message == "Finished loading packages." {
					log.Printf("[DEBUG-WORKSPACE] Detected 'Finished loading packages' - marking workspace ready")
					c.workspaceReadyMu.Lock()
					if !c.workspaceReady {
						c.workspaceReady = true
						close(c.workspaceReadyCh)
						log.Printf("[INFO-WORKSPACE] Workspace indexing completed - server ready for workspace operations")
					} else {
						log.Printf("[DEBUG-WORKSPACE] Workspace was already marked ready")
					}
					c.workspaceReadyMu.Unlock()
				}
			} else {
				log.Printf("[DEBUG-WORKSPACE] Could not extract message string from params")
			}
		} else {
			log.Printf("[DEBUG-WORKSPACE] Could not cast params to map[string]interface{}")
		}
	}
}

// waitForWorkspaceReady waits for workspace indexing to complete for methods that require it
func (c *StdioClient) waitForWorkspaceReady(ctx context.Context, method string) error {
	// Methods that require workspace indexing
	workspaceMethods := map[string]bool{
		"workspace/symbol": true,
		"workspace/executeCommand": true,
	}
	
	if !workspaceMethods[method] {
		return nil // No need to wait for non-workspace methods
	}
	
	// Check if already ready
	c.workspaceReadyMu.RLock()
	if c.workspaceReady {
		c.workspaceReadyMu.RUnlock()
		return nil
	}
	c.workspaceReadyMu.RUnlock()
	
	log.Printf("[DEBUG-WORKSPACE] Waiting for workspace indexing to complete for method: %s", method)
	
	// Wait for workspace ready with timeout
	select {
	case <-c.workspaceReadyCh:
		log.Printf("[DEBUG-WORKSPACE] Workspace ready - proceeding with method: %s", method)
		return nil
	case <-ctx.Done():
		return fmt.Errorf("workspace loading timeout for method %s: %w", method, ctx.Err())
	case <-time.After(15 * time.Second):
		// If gopls doesn't send the "Finished loading packages" message within 15s,
		// assume it's ready anyway (fallback for older versions or different servers)
		log.Printf("[WARN-WORKSPACE] Workspace loading timeout for %s, proceeding anyway", method)
		c.workspaceReadyMu.Lock()
		if !c.workspaceReady {
			c.workspaceReady = true
			close(c.workspaceReadyCh)
		}
		c.workspaceReadyMu.Unlock()
		return nil
	}
}

func (c *StdioClient) WriteMessage(msg JSONRPCMessage) error {
	jsonData, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf(ERROR_MARSHAL_MESSAGE, err)
	}

	content := fmt.Sprintf(PROTOCOL_HEADER_FORMAT, len(jsonData), jsonData)

	// Write with timeout protection to prevent blocking on LSP server hangs
	return c.writeWithTimeout([]byte(content), 10*time.Second)
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

// initializeLSPServer performs the LSP initialization handshake
func (c *StdioClient) initializeLSPServer(ctx context.Context) error {
	log.Printf("[DEBUG-STDIO] Starting LSP initialization for command: %s", c.config.Command)
	
	// Get current working directory for rootUri
	wd, err := os.Getwd()
	if err != nil {
		log.Printf("[WARN] Failed to get working directory: %v", err)
		wd = "/tmp" // fallback
	}

	// Create initialize request with basic client capabilities
	initializeParams := map[string]interface{}{
		"processId": os.Getpid(),
		"clientInfo": map[string]interface{}{
			"name":    "lsp-gateway",
			"version": "0.0.1",
		},
		"rootUri": fmt.Sprintf("file://%s", wd),
		"capabilities": map[string]interface{}{
			"workspace": map[string]interface{}{
				"applyEdit":                true,
				"workspaceEdit":            map[string]interface{}{"documentChanges": true},
				"didChangeConfiguration":   map[string]interface{}{"dynamicRegistration": true},
				"didChangeWatchedFiles":    map[string]interface{}{"dynamicRegistration": true},
				"symbol":                   map[string]interface{}{"dynamicRegistration": true},
				"executeCommand":           map[string]interface{}{"dynamicRegistration": true},
				"configuration":            true,
				"workspaceFolders":         true,
			},
			"textDocument": map[string]interface{}{
				"publishDiagnostics": map[string]interface{}{
					"relatedInformation": true,
					"versionSupport":     false,
					"tagSupport":         map[string]interface{}{"valueSet": []int{1, 2}},
				},
				"synchronization": map[string]interface{}{
					"dynamicRegistration": true,
					"willSave":            true,
					"willSaveWaitUntil":   true,
					"didSave":             true,
				},
				"completion": map[string]interface{}{
					"dynamicRegistration": true,
					"contextSupport":      true,
					"completionItem": map[string]interface{}{
						"snippetSupport":          true,
						"commitCharactersSupport": true,
						"documentationFormat":     []string{"markdown", "plaintext"},
						"deprecatedSupport":       true,
						"preselectSupport":        true,
					},
					"completionItemKind": map[string]interface{}{
						"valueSet": []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25},
					},
				},
				"hover": map[string]interface{}{
					"dynamicRegistration": true,
					"contentFormat":       []string{"markdown", "plaintext"},
				},
				"signatureHelp": map[string]interface{}{
					"dynamicRegistration": true,
					"signatureInformation": map[string]interface{}{
						"documentationFormat": []string{"markdown", "plaintext"},
					},
				},
				"definition": map[string]interface{}{
					"dynamicRegistration": true,
				},
				"references": map[string]interface{}{
					"dynamicRegistration": true,
				},
				"documentHighlight": map[string]interface{}{
					"dynamicRegistration": true,
				},
				"documentSymbol": map[string]interface{}{
					"dynamicRegistration": true,
					"symbolKind": map[string]interface{}{
						"valueSet": []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26},
					},
				},
				"codeAction": map[string]interface{}{
					"dynamicRegistration": true,
				},
				"codeLens": map[string]interface{}{
					"dynamicRegistration": true,
				},
				"formatting": map[string]interface{}{
					"dynamicRegistration": true,
				},
				"rangeFormatting": map[string]interface{}{
					"dynamicRegistration": true,
				},
				"onTypeFormatting": map[string]interface{}{
					"dynamicRegistration": true,
				},
				"rename": map[string]interface{}{
					"dynamicRegistration": true,
				},
			},
		},
		"initializationOptions": map[string]interface{}{},
		"workspaceFolders": []map[string]interface{}{
			{
				"uri":  fmt.Sprintf("file://%s", wd),
				"name": filepath.Base(wd),
			},
		},
	}

	// Send initialize request with timeout
	initCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	log.Printf("[DEBUG] Sending LSP initialize request")
	response, err := c.SendRequest(initCtx, "initialize", initializeParams)
	if err != nil {
		return fmt.Errorf("initialize request failed: %w", err)
	}

	log.Printf("[DEBUG] LSP initialize response: %s", string(response))

	// Send initialized notification
	log.Printf("[DEBUG] Sending LSP initialized notification")
	if err := c.SendNotification(ctx, "initialized", map[string]interface{}{}); err != nil {
		return fmt.Errorf("initialized notification failed: %w", err)
	}

	// Send workspace/didChangeWorkspaceFolders notification to ensure workspace is properly initialized for symbol search
	log.Printf("[DEBUG] Sending workspace/didChangeWorkspaceFolders notification to activate workspace features")
	err = c.SendNotification(ctx, "workspace/didChangeWorkspaceFolders", map[string]interface{}{
		"event": map[string]interface{}{
			"added": []map[string]interface{}{
				{
					"uri":  fmt.Sprintf("file://%s", wd),
					"name": filepath.Base(wd),
				},
			},
			"removed": []map[string]interface{}{},
		},
	})
	if err != nil {
		log.Printf("[WARN] Failed to send workspace folder notification (non-critical): %v", err)
	}

	log.Printf("[INFO] LSP server initialization completed successfully")
	return nil
}

// superviseResponseHandler monitors and restarts the response handler goroutine if it fails
func (c *StdioClient) superviseResponseHandler() {
	log.Printf("[DEBUG-STDIO] Response handler supervisor started")
	
	for {
		select {
		case <-c.stopCh:
			log.Printf("[DEBUG-STDIO] Response handler supervisor stopping")
			return
		default:
		}
		
		// Start or restart the response handler
		c.responseHandlerMu.Lock()
		if !c.responseHandlerActive {
			log.Printf("[DEBUG-STDIO] Starting/restarting response handler")
			c.responseHandlerActive = true
			c.lastResponseTime = time.Now()
			go c.monitoredHandleResponses()
		}
		c.responseHandlerMu.Unlock()
		
		// Check health every 5 seconds
		select {
		case <-c.stopCh:
			return
		case <-time.After(5 * time.Second):
			c.checkResponseHandlerHealth()
		}
	}
}

// checkResponseHandlerHealth verifies the response handler is still functioning
func (c *StdioClient) checkResponseHandlerHealth() {
	c.responseHandlerMu.Lock()
	defer c.responseHandlerMu.Unlock()
	
	if !c.responseHandlerActive {
		return
	}
	
	// If no response activity for 60 seconds, assume handler is stuck
	if time.Since(c.lastResponseTime) > 60*time.Second {
		log.Printf("[WARN-STDIO] Response handler appears stuck, will restart on next cycle")
		c.responseHandlerActive = false
	}
}

// monitoredHandleResponses wraps handleResponses with monitoring and recovery
func (c *StdioClient) monitoredHandleResponses() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[ERROR-STDIO] Response handler panicked: %v", r)
		}
		
		// Mark handler as inactive so supervisor will restart it
		c.responseHandlerMu.Lock()
		c.responseHandlerActive = false
		c.responseHandlerMu.Unlock()
		
		log.Printf("[WARN-STDIO] Response handler stopped, supervisor will restart")
	}()
	
	// Update activity timestamp
	c.responseHandlerMu.Lock()
	c.lastResponseTime = time.Now()  
	c.responseHandlerMu.Unlock()
	
	// Call the original handler
	c.handleResponses()
}

// monitorProcess continuously monitors the LSP server process health
func (c *StdioClient) monitorProcess() {
	log.Printf("[DEBUG-STDIO] Process monitor started for PID: %d", c.cmd.Process.Pid)
	
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-c.stopCh:
			log.Printf("[DEBUG-STDIO] Process monitor stopping")
			return
		case <-ticker.C:
			if err := c.checkProcessHealth(); err != nil {
				log.Printf("[ERROR-STDIO] Process health check failed: %v", err)
				c.handleProcessDeath()
				return
			}
		}
	}
}

// checkProcessHealth verifies the LSP server process is still alive and responsive
func (c *StdioClient) checkProcessHealth() error {
	// Check if process has exited
	if c.cmd.ProcessState != nil {
		return fmt.Errorf("LSP server process has exited: %v", c.cmd.ProcessState)
	}
	
	// Check if process is still alive by sending signal 0 (no-op signal)
	if err := c.cmd.Process.Signal(syscall.Signal(0)); err != nil {
		return fmt.Errorf("LSP server process is not responding to signals: %v", err)
	}
	
	return nil
}

// handleProcessDeath handles LSP server process termination
func (c *StdioClient) handleProcessDeath() {
	log.Printf("[ERROR-STDIO] LSP server process died, cleaning up")
	
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Mark client as inactive
	c.active = false
	
	// Notify all pending requests that the server is dead
	for id, req := range c.requests {
		log.Printf("[WARN-STDIO] Notifying request %s of server death", id)
		select {
		case req.respCh <- json.RawMessage(`{"error":{"code":-32603,"message":"LSP server process died"}}`):
		default:
		}
		close(req.respCh)
		delete(c.requests, id)
	}
	
	// Mark response handler as inactive
	c.responseHandlerMu.Lock()
	c.responseHandlerActive = false
	c.responseHandlerMu.Unlock()
	
	// Trigger client stop to clean up all resources
	select {
	case <-c.stopCh:
		// Already stopped
	default:
		close(c.stopCh)
	}
}

// createPipesWithTimeout creates stdin, stdout, and stderr pipes with timeout protection
func (c *StdioClient) createPipesWithTimeout(timeout time.Duration) error {
	type pipeResult struct {
		stdin  io.WriteCloser
		stdout io.ReadCloser
		stderr io.ReadCloser
		err    error
	}

	resultCh := make(chan pipeResult, 1)

	// Create pipes in goroutine to enable timeout
	go func() {
		var result pipeResult
		
		result.stdin, result.err = c.cmd.StdinPipe()
		if result.err != nil {
			result.err = fmt.Errorf("failed to create stdin pipe: %w", result.err)
			resultCh <- result
			return
		}

		result.stdout, result.err = c.cmd.StdoutPipe()
		if result.err != nil {
			result.err = fmt.Errorf("failed to create stdout pipe: %w", result.err)
			resultCh <- result
			return
		}

		result.stderr, result.err = c.cmd.StderrPipe()
		if result.err != nil {
			result.err = fmt.Errorf("failed to create stderr pipe: %w", result.err)
			resultCh <- result
			return
		}

		resultCh <- result
	}()

	// Wait for pipes creation or timeout
	select {
	case result := <-resultCh:
		if result.err != nil {
			return result.err
		}
		c.Stdin = result.stdin
		c.stdout = result.stdout
		c.stderr = result.stderr
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("pipe creation timed out after %v", timeout)
	}
}

// writeWithTimeout writes data to stdin with timeout protection to prevent blocking
func (c *StdioClient) writeWithTimeout(data []byte, timeout time.Duration) error {
	type writeResult struct {
		n   int
		err error
	}

	resultCh := make(chan writeResult, 1)

	// Perform write in goroutine to enable timeout
	go func() {
		n, err := c.Stdin.Write(data)
		resultCh <- writeResult{n: n, err: err}
	}()

	// Wait for write completion or timeout
	select {
	case result := <-resultCh:
		if result.err != nil {
			return fmt.Errorf(ERROR_WRITE_MESSAGE, result.err)
		}
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("write operation timed out after %v - LSP server may not be responsive", timeout)
	case <-c.stopCh:
		return fmt.Errorf("client stopped during write operation")
	}
}

// installJDTLSAsync handles JDTLS installation in background to prevent blocking startup
func (c *StdioClient) installJDTLSAsync() {
	defer func() {
		c.jdtlsInstallMu.Lock()
		c.jdtlsInstalling = false
		c.jdtlsInstallMu.Unlock()
		
		// Signal installation completion
		select {
		case <-c.jdtlsInstallDone:
			// Already closed
		default:
			close(c.jdtlsInstallDone)
		}
	}()

	log.Printf("[INFO] Starting JDTLS auto-installation...")
	
	if err := c.installJDTLS(); err != nil {
		c.jdtlsInstallMu.Lock()
		c.jdtlsInstallError = fmt.Errorf("failed to auto-install JDTLS: %w", err)
		c.jdtlsInstallMu.Unlock()
		
		log.Printf("[ERROR] JDTLS auto-installation failed: %v", err)
		log.Printf("[WARN] LSP server may not function properly without JDTLS")
		return
	}
	
	log.Printf("[INFO] JDTLS installation completed successfully")
}

// waitForJDTLSInstallation waits for JDTLS installation to complete if needed
func (c *StdioClient) waitForJDTLSInstallation(ctx context.Context) error {
	// Only wait if this is a JDTLS command
	if !c.isJDTLSCommand() {
		return nil
	}

	// Check if installation is needed and in progress
	c.jdtlsInstallMu.RLock()
	installing := c.jdtlsInstalling
	installError := c.jdtlsInstallError
	c.jdtlsInstallMu.RUnlock()

	// If not installing and no error, JDTLS is ready
	if !installing && installError == nil {
		return nil
	}

	// If there was an installation error, return it
	if installError != nil {
		return installError
	}

	// Wait for installation to complete with timeout
	select {
	case <-c.jdtlsInstallDone:
		// Check for installation error after completion
		c.jdtlsInstallMu.RLock()
		installError := c.jdtlsInstallError
		c.jdtlsInstallMu.RUnlock()
		
		if installError != nil {
			return installError
		}
		
		log.Printf("[DEBUG] JDTLS installation completed, proceeding with request")
		return nil
		
	case <-ctx.Done():
		return fmt.Errorf("timeout waiting for JDTLS installation: %w", ctx.Err())
		
	case <-time.After(5 * time.Minute):
		return fmt.Errorf("JDTLS installation timeout - proceeding anyway")
	}
}

// startProcessWithTimeout starts the LSP server process with timeout protection
func (c *StdioClient) startProcessWithTimeout(timeout time.Duration) error {
	resultCh := make(chan error, 1)

	// Start process in goroutine to enable timeout
	go func() {
		err := c.cmd.Start()
		resultCh <- err
	}()

	// Wait for process start or timeout
	select {
	case err := <-resultCh:
		if err != nil {
			return fmt.Errorf("failed to start LSP server process: %w", err)
		}
		return nil
	case <-time.After(timeout):
		// Process start timed out - try to kill the command if it exists
		if c.cmd.Process != nil {
			_ = c.cmd.Process.Kill()
		}
		return fmt.Errorf("LSP server process start timed out after %v", timeout)
	case <-c.stopCh:
		return fmt.Errorf("client stopped during process start")
	}
}
