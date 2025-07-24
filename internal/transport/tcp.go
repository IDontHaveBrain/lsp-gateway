package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type CircuitBreakerState int32

const (
	CircuitClosed CircuitBreakerState = iota
	CircuitOpen
	CircuitHalfOpen
)

type TCPClient struct {
	config ClientConfig

	// Connection management with pooling
	connPool *ConnectionPool
	active   int32 // atomic flag for active state

	// Request tracking with optimized synchronization
	requests  map[string]chan json.RawMessage
	requestMu sync.RWMutex
	nextID    int64

	ctx    context.Context
	cancel context.CancelFunc

	// High-performance circuit breaker with atomic operations
	errorCount      int64 // atomic counter
	lastErrorTime   int64 // atomic timestamp (unix nano)
	circuitState    int32 // atomic CircuitBreakerState
	successCount    int64 // atomic counter for half-open state
	maxRetries      int64
	baseDelay       time.Duration
	circuitTimeout  time.Duration // time to wait before trying half-open
	healthThreshold int64         // consecutive successes needed to close circuit
}

// ConnectionPool manages TCP connections with health checking
type ConnectionPool struct {
	address      string
	maxSize      int
	connections  chan net.Conn
	activeConns  int64
	totalCreated int64
	ctx          context.Context
	cancel       context.CancelFunc
	health       *ConnectionHealth
}

// ConnectionHealth tracks connection health metrics
type ConnectionHealth struct {
	lastCheckTime  int64
	healthCheckInt time.Duration
}

func NewTCPClient(config ClientConfig) (LSPClient, error) {
	if config.Transport != TransportTCP {
		return nil, fmt.Errorf("invalid transport for TCP client: %s", config.Transport)
	}

	address, err := parseAddress(config.Command)
	if err != nil {
		return nil, fmt.Errorf("failed to parse TCP address: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	connPool := NewConnectionPool(address, 5, ctx) // Pool of 5 connections

	return &TCPClient{
		config:          config,
		connPool:        connPool,
		requests:        make(map[string]chan json.RawMessage),
		maxRetries:      5,
		baseDelay:       100 * time.Millisecond,
		circuitTimeout:  3 * time.Second,
		healthThreshold: 1,
		ctx:             ctx,
		cancel:          cancel,
	}, nil
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(address string, maxSize int, ctx context.Context) *ConnectionPool {
	poolCtx, cancel := context.WithCancel(ctx)
	return &ConnectionPool{
		address:     address,
		maxSize:     maxSize,
		connections: make(chan net.Conn, maxSize),
		ctx:         poolCtx,
		cancel:      cancel,
		health: &ConnectionHealth{
			healthCheckInt: 10 * time.Second,
		},
	}
}

// parseAddress parses and validates TCP address
func parseAddress(address string) (string, error) {
	if address == "" {
		return "", fmt.Errorf("TCP address not specified")
	}

	if _, err := strconv.Atoi(address); err == nil {
		address = "localhost:" + address
	}

	if !strings.Contains(address, ":") {
		return "", fmt.Errorf("invalid TCP address format: %s (expected host:port)", address)
	}

	return address, nil
}

func (c *TCPClient) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&c.active, 0, 1) {
		return fmt.Errorf("TCP client already active")
	}

	// Start connection pool health monitoring
	go c.connPool.startHealthMonitoring()

	// Initialize connection pool with initial connections
	if err := c.connPool.warmUp(); err != nil {
		// Reset active state on failure
		atomic.StoreInt32(&c.active, 0)
		return fmt.Errorf("failed to warm up connection pool: %w", err)
	}

	// Start message handling goroutines (one per connection for efficiency)
	for i := 0; i < c.connPool.maxSize; i++ {
		go c.handleMessages()
	}

	return nil
}

func (c *TCPClient) Stop() error {
	if !atomic.CompareAndSwapInt32(&c.active, 1, 0) {
		return nil // Already stopped or not started
	}

	// Signal shutdown
	if c.cancel != nil {
		c.cancel()
	}

	// Stop connection pool (closes all connections)
	c.connPool.Close()

	// Clean up pending requests efficiently
	c.requestMu.Lock()
	for id, ch := range c.requests {
		select {
		case <-ch:
			// Channel already closed
		default:
			close(ch)
		}
		delete(c.requests, id)
	}
	c.requestMu.Unlock()

	return nil
}

func (c *TCPClient) SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	if atomic.LoadInt32(&c.active) == 0 {
		return nil, errors.New(ERROR_TCP_CLIENT_NOT_ACTIVE)
	}

	// Check circuit breaker state
	if c.isCircuitOpen() {
		return nil, errors.New("circuit breaker is open")
	}

	id := c.generateRequestID()
	respCh := make(chan json.RawMessage, 1)

	// Register request efficiently
	c.requestMu.Lock()
	c.requests[id] = respCh
	c.requestMu.Unlock()

	// Cleanup function without goroutines
	defer func() {
		c.requestMu.Lock()
		if ch, exists := c.requests[id]; exists {
			delete(c.requests, id)
			if ch != nil {
				select {
				case <-ch:
					// Already closed
				default:
					close(ch)
				}
			}
		}
		c.requestMu.Unlock()
	}()

	request := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	if err := c.sendMessage(request); err != nil {
		c.recordError()
		return nil, fmt.Errorf(ERROR_SEND_REQUEST, err)
	}

	// Wait for response with context timeout
	select {
	case response, ok := <-respCh:
		if !ok {
			return nil, errors.New("response channel was closed")
		}
		c.recordSuccess()
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

	// Check circuit breaker state
	if c.isCircuitOpen() {
		return errors.New("circuit breaker is open")
	}

	notification := JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}

	if err := c.sendMessage(notification); err != nil {
		c.recordError()
		return fmt.Errorf(ERROR_SEND_NOTIFICATION, err)
	}

	c.recordSuccess()
	return nil
}

func (c *TCPClient) IsActive() bool {
	return atomic.LoadInt32(&c.active) != 0
}

func (c *TCPClient) generateRequestID() string {
	id := atomic.AddInt64(&c.nextID, 1)
	return strconv.FormatInt(id, 10)
}

func (c *TCPClient) sendMessage(msg JSONRPCMessage) error {
	// Get connection from pool
	conn, err := c.connPool.getConnection()
	if err != nil {
		return fmt.Errorf("failed to get connection: %w", err)
	}
	defer c.connPool.releaseConnection(conn)

	// Set write deadline for safety
	if err := conn.SetWriteDeadline(time.Now().Add(3 * time.Second)); err != nil {
		return fmt.Errorf("failed to set write deadline: %w", err)
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf(ERROR_MARSHAL_MESSAGE, err)
	}

	message := fmt.Sprintf(PROTOCOL_HEADER_FORMAT, len(data), string(data))

	// Write directly to connection without buffering for better performance
	if _, err := conn.Write([]byte(message)); err != nil {
		return fmt.Errorf(ERROR_WRITE_MESSAGE, err)
	}

	return nil
}

func (c *TCPClient) handleMessages() {
	consecutiveErrors := 0
	maxConsecutiveErrors := 10

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			// Circuit breaker check (lock-free)
			if c.isCircuitOpen() {
				time.Sleep(500 * time.Millisecond) // Brief pause before rechecking
				continue
			}

			msg, err := c.readMessage()
			if err != nil {
				if err == io.EOF {
					return
				}

				consecutiveErrors++
				c.recordError()

				// Stop if too many consecutive errors
				if consecutiveErrors >= maxConsecutiveErrors {
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
			c.recordSuccess()

			c.handleMessage(msg)
		}
	}
}

func (c *TCPClient) readMessage() (*JSONRPCMessage, error) {
	// Get connection from pool
	conn, err := c.connPool.getConnection()
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}
	defer c.connPool.releaseConnection(conn)

	// Set read deadline for safety
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return nil, fmt.Errorf("failed to set read deadline: %w", err)
	}

	reader := bufio.NewReader(conn)

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
	if msg.ID == nil {
		return // Notification, no response expected
	}

	idStr := fmt.Sprintf("%v", msg.ID)

	// Get response channel efficiently
	c.requestMu.RLock()
	respCh, exists := c.requests[idStr]
	c.requestMu.RUnlock()

	if !exists || respCh == nil {
		return // No pending request for this ID
	}

	var responseData json.RawMessage
	var err error

	if msg.Result != nil {
		responseData, err = json.Marshal(msg.Result)
		if err != nil {
			// Create fallback error response
			responseData = json.RawMessage(`{"code":-32603,"message":"Internal error: failed to marshal result"}`)
		}
	} else if msg.Error != nil {
		responseData, err = json.Marshal(msg.Error)
		if err != nil {
			// Create fallback error response
			responseData = json.RawMessage(`{"code":-32603,"message":"Internal error: failed to marshal error"}`)
		}
	} else {
		return // No result or error
	}

	// Send response without timeout (channel should not block)
	select {
	case respCh <- responseData:
		// Successfully sent
	default:
		// Channel full or closed - request may have timed out
	}
}

// High-performance circuit breaker using atomic operations
func (c *TCPClient) recordError() {
	atomic.AddInt64(&c.errorCount, 1)
	atomic.StoreInt64(&c.lastErrorTime, time.Now().UnixNano())

	// Check if we should open the circuit
	errorCount := atomic.LoadInt64(&c.errorCount)
	currentState := CircuitBreakerState(atomic.LoadInt32(&c.circuitState))

	if errorCount > c.maxRetries {
		if currentState == CircuitClosed {
			c.openCircuit()
		} else if currentState == CircuitHalfOpen {
			// Reset success count and go back to open
			atomic.StoreInt64(&c.successCount, 0)
			c.openCircuit()
		}
	}
}

func (c *TCPClient) recordSuccess() {
	currentState := CircuitBreakerState(atomic.LoadInt32(&c.circuitState))

	if currentState == CircuitHalfOpen {
		// Increment success count in half-open state
		successCount := atomic.AddInt64(&c.successCount, 1)
		if successCount >= c.healthThreshold {
			// Close the circuit
			atomic.StoreInt32(&c.circuitState, int32(CircuitClosed))
			atomic.StoreInt64(&c.errorCount, 0)
			atomic.StoreInt64(&c.successCount, 0)
		}
	} else if currentState == CircuitClosed {
		// Reset error count on success in closed state
		atomic.StoreInt64(&c.errorCount, 0)
	} else if currentState == CircuitOpen {
		// If we somehow get a success in open state, transition to half-open
		if atomic.CompareAndSwapInt32(&c.circuitState, int32(CircuitOpen), int32(CircuitHalfOpen)) {
			atomic.StoreInt64(&c.successCount, 1)
			atomic.StoreInt64(&c.errorCount, 0)
		}
	}
}

func (c *TCPClient) openCircuit() {
	atomic.StoreInt32(&c.circuitState, int32(CircuitOpen))
	atomic.StoreInt64(&c.lastErrorTime, time.Now().UnixNano())
}

func (c *TCPClient) isCircuitOpen() bool {
	currentState := CircuitBreakerState(atomic.LoadInt32(&c.circuitState))

	switch currentState {
	case CircuitClosed:
		return false
	case CircuitOpen:
		// Check if circuit should transition to half-open
		lastErrorTime := atomic.LoadInt64(&c.lastErrorTime)
		if time.Since(time.Unix(0, lastErrorTime)) > c.circuitTimeout {
			// Try to transition to half-open
			if atomic.CompareAndSwapInt32(&c.circuitState, int32(CircuitOpen), int32(CircuitHalfOpen)) {
				atomic.StoreInt64(&c.successCount, 0)
				return false // Allow one request through
			}
		}
		return true
	case CircuitHalfOpen:
		return false // Allow requests through in half-open state
	default:
		return false
	}
}

func (c *TCPClient) calculateBackoff(attempt int) time.Duration {
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

// Connection Pool Implementation

// warmUp initializes the connection pool with initial connections
func (pool *ConnectionPool) warmUp() error {
	minConnections := 1                                // At least one connection to verify connectivity
	initialSize := max(minConnections, pool.maxSize/3) // Start with 1/3 capacity

	successCount := 0
	for i := 0; i < initialSize; i++ {
		conn, err := pool.createConnectionWithRetry()
		if err != nil {
			// If we can't create even one connection, fail
			if successCount == 0 {
				return fmt.Errorf("failed to create initial connection: %w", err)
			}
			// Otherwise, continue with partial pool
			continue
		}

		select {
		case pool.connections <- conn:
			atomic.AddInt64(&pool.totalCreated, 1)
			successCount++
		default:
			// Pool full, close connection
			conn.Close()
			successCount++
		}
	}

	if successCount == 0 {
		return fmt.Errorf("failed to create any initial connections")
	}

	return nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// getConnection retrieves a connection from the pool or creates a new one
func (pool *ConnectionPool) getConnection() (net.Conn, error) {
	select {
	case conn := <-pool.connections:
		// Test connection health
		if pool.isConnectionHealthy(conn) {
			atomic.AddInt64(&pool.activeConns, 1)
			return conn, nil
		}
		// Connection unhealthy, close and create new one
		conn.Close()
		// Create new connection with retry
		conn, err := pool.createConnectionWithRetry()
		if err != nil {
			return nil, err
		}
		atomic.AddInt64(&pool.activeConns, 1)
		atomic.AddInt64(&pool.totalCreated, 1)
		return conn, nil
	default:
		// No available connections, create new one with retry
		conn, err := pool.createConnectionWithRetry()
		if err != nil {
			return nil, err
		}
		atomic.AddInt64(&pool.activeConns, 1)
		atomic.AddInt64(&pool.totalCreated, 1)
		return conn, nil
	}
}

// releaseConnection returns a connection to the pool
func (pool *ConnectionPool) releaseConnection(conn net.Conn) {
	atomic.AddInt64(&pool.activeConns, -1)

	if !pool.isConnectionHealthy(conn) {
		conn.Close()
		return
	}

	// Check if pool is closed
	select {
	case <-pool.ctx.Done():
		// Pool is closed, just close the connection
		conn.Close()
		return
	default:
	}

	select {
	case pool.connections <- conn:
		// Successfully returned to pool
	case <-pool.ctx.Done():
		// Pool closed while waiting, close connection
		conn.Close()
	default:
		// Pool full, close connection
		conn.Close()
	}
}

// createConnection creates a new TCP connection
func (pool *ConnectionPool) createConnection() (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", pool.address, 3*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", pool.address, err)
	}
	return conn, nil
}

// createConnectionWithRetry creates a new TCP connection with retry logic
func (pool *ConnectionPool) createConnectionWithRetry() (net.Conn, error) {
	maxRetries := 5
	baseDelay := 100 * time.Millisecond

	for attempt := 0; attempt < maxRetries; attempt++ {
		conn, err := pool.createConnection()
		if err == nil {
			return conn, nil
		}

		// If this is the last attempt, return the error
		if attempt == maxRetries-1 {
			return nil, err
		}

		// Less aggressive exponential backoff with better jitter
		// Use base of 1.5 instead of 2 for more gradual increase
		multiplier := math.Pow(1.5, float64(attempt))
		delay := time.Duration(float64(baseDelay) * multiplier)

		// Add jitter (±30% for more variance)
		jitter := time.Duration(float64(delay) * (rand.Float64() - 0.5) * 0.6)
		finalDelay := delay + jitter

		// Cap at 1 second for faster retry cycles
		if finalDelay > time.Second {
			finalDelay = time.Second
		}

		time.Sleep(finalDelay)
	}

	return nil, fmt.Errorf("failed to create connection after %d attempts", maxRetries)
}

// isConnectionHealthy checks if a connection is still healthy
func (pool *ConnectionPool) isConnectionHealthy(conn net.Conn) bool {
	// Simple health check: try to set a deadline
	if err := conn.SetDeadline(time.Now().Add(time.Millisecond)); err != nil {
		return false
	}
	// Reset deadline
	conn.SetDeadline(time.Time{})
	return true
}

// startHealthMonitoring starts background health monitoring
func (pool *ConnectionPool) startHealthMonitoring() {
	ticker := time.NewTicker(pool.health.healthCheckInt)
	defer ticker.Stop()

	for {
		select {
		case <-pool.ctx.Done():
			return
		case <-ticker.C:
			pool.healthCheck()
		}
	}
}

// healthCheck performs periodic health checks on pooled connections
func (pool *ConnectionPool) healthCheck() {
	connCount := len(pool.connections)
	unhealthyCount := 0

	// Check up to 3 connections per health check cycle
	maxCheck := min(connCount, 3)

	for i := 0; i < maxCheck; i++ {
		select {
		case conn := <-pool.connections:
			if pool.isConnectionHealthy(conn) {
				// Return healthy connection to pool
				select {
				case pool.connections <- conn:
				default:
					conn.Close()
				}
			} else {
				// Close unhealthy connection
				conn.Close()
				unhealthyCount++
			}
		default:
			return // No connections to check
		}
	}

	// Update health metrics
	atomic.StoreInt64(&pool.health.lastCheckTime, time.Now().UnixNano())
}

// Close closes the connection pool and all connections
func (pool *ConnectionPool) Close() {
	pool.cancel()

	// Drain and close all connections without closing the channel
	for {
		select {
		case conn := <-pool.connections:
			conn.Close()
		default:
			// No more connections to drain
			return
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Testing methods for circuit breaker functionality

// SetMaxRetriesForTesting sets the max retries for testing purposes
func (c *TCPClient) SetMaxRetriesForTesting(retries int) {
	c.maxRetries = int64(retries)
}

// IsCircuitOpenForTesting checks if the circuit is open for testing purposes
func (c *TCPClient) IsCircuitOpenForTesting() bool {
	return c.isCircuitOpen()
}

// RecordErrorForTesting records an error for testing purposes
func (c *TCPClient) RecordErrorForTesting() {
	c.recordError()
}

// RecordSuccessForTesting records a success for testing purposes
func (c *TCPClient) RecordSuccessForTesting() {
	c.recordSuccess()
}

// OpenCircuitForTesting opens the circuit for testing purposes
func (c *TCPClient) OpenCircuitForTesting() {
	c.openCircuit()
}

// GetErrorCountForTesting gets the error count for testing purposes
func (c *TCPClient) GetErrorCountForTesting() int64 {
	return atomic.LoadInt64(&c.errorCount)
}

// GetCircuitStateForTesting gets the circuit state for testing purposes
func (c *TCPClient) GetCircuitStateForTesting() CircuitBreakerState {
	return CircuitBreakerState(atomic.LoadInt32(&c.circuitState))
}

// SetLastErrorTimeForTesting sets the last error time for testing purposes
func (c *TCPClient) SetLastErrorTimeForTesting(t time.Time) {
	atomic.StoreInt64(&c.lastErrorTime, t.UnixNano())
}

// ResetCircuitForTesting resets the circuit state for testing purposes
func (c *TCPClient) ResetCircuitForTesting() {
	atomic.StoreInt32(&c.circuitState, int32(CircuitClosed))
	atomic.StoreInt64(&c.errorCount, 0)
	atomic.StoreInt64(&c.successCount, 0)
}

// CalculateBackoffForTesting calculates backoff for testing purposes
func (c *TCPClient) CalculateBackoffForTesting(attempt int) time.Duration {
	return c.calculateBackoff(attempt)
}

// GetConnectionPoolStatsForTesting gets connection pool statistics for testing
func (c *TCPClient) GetConnectionPoolStatsForTesting() (active, total int64, poolSize int) {
	return atomic.LoadInt64(&c.connPool.activeConns),
		atomic.LoadInt64(&c.connPool.totalCreated),
		len(c.connPool.connections)
}
