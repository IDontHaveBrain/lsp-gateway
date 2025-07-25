package transport

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// EnhancedConnectionPool manages TCP connections with dynamic sizing
type EnhancedConnectionPool struct {
	// Basic config
	address   string
	transport string

	// Dynamic sizing
	minSize     int
	maxSize     int
	currentSize int32
	targetSize  int32

	// Connection management
	connections       chan *PooledConnection
	activeConnections map[string]*PooledConnection
	connectionIndex   int64

	// Performance metrics
	requestCount int64

	// Dynamic sizing logic
	sizer *PoolSizer

	// Context and sync
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.RWMutex

	// Background goroutines
	scalingTicker *time.Ticker
	healthTicker  *time.Ticker
}

// PooledConnection represents a connection with tracking metadata
type PooledConnection struct {
	conn      net.Conn
	id        string
	createdAt time.Time
	lastUsed  time.Time
	useCount  int64
	isHealthy bool
	mu        sync.RWMutex
}

// PoolSizer handles dynamic sizing logic
type PoolSizer struct {
	minConnections     int
	maxConnections     int
	targetUtilization  float64 // 0.7 for 70%
	scaleUpThreshold   float64 // 0.85 for 85%
	scaleDownThreshold float64 // 0.5 for 50%

	scaleUpFactor   float64 // 1.5 for 50% increase
	scaleDownFactor float64 // 0.75 for 25% decrease
	cooldownPeriod  time.Duration
	lastScaleTime   time.Time

	mu sync.RWMutex
}

// EnhancedPoolStats provides statistics about the enhanced connection pool
type EnhancedPoolStats struct {
	TotalConnections     int
	ActiveConnections    int
	AvailableConnections int
	Utilization          float64
	RequestCount         int64
	MinSize              int
	MaxSize              int
	CurrentSize          int
	TargetSize           int
}

// NewEnhancedConnectionPool creates a new enhanced connection pool
func NewEnhancedConnectionPool(address string, minSize, maxSize int) *EnhancedConnectionPool {
	ctx, cancel := context.WithCancel(context.Background())

	if minSize < 1 {
		minSize = 1
	}
	if maxSize < minSize {
		maxSize = minSize * 2
	}

	sizer := &PoolSizer{
		minConnections:     minSize,
		maxConnections:     maxSize,
		targetUtilization:  0.7,
		scaleUpThreshold:   0.85,
		scaleDownThreshold: 0.5,
		scaleUpFactor:      1.5,
		scaleDownFactor:    0.75,
		cooldownPeriod:     30 * time.Second,
	}

	pool := &EnhancedConnectionPool{
		address:           address,
		transport:         "tcp",
		minSize:           minSize,
		maxSize:           maxSize,
		currentSize:       0,
		targetSize:        int32(minSize),
		connections:       make(chan *PooledConnection, maxSize),
		activeConnections: make(map[string]*PooledConnection),
		sizer:             sizer,
		ctx:               ctx,
		cancel:            cancel,
	}

	return pool
}

// Initialize starts the connection pool
func (ecp *EnhancedConnectionPool) Initialize() error {
	if err := ecp.warmUp(); err != nil {
		return fmt.Errorf("failed to warm up enhanced connection pool: %w", err)
	}

	// Start background scaling
	ecp.scalingTicker = time.NewTicker(5 * time.Second)
	go ecp.backgroundScaling()

	// Start health monitoring
	ecp.healthTicker = time.NewTicker(10 * time.Second)
	go ecp.backgroundHealthMonitoring()

	return nil
}

// GetConnection retrieves a connection from the pool
func (ecp *EnhancedConnectionPool) GetConnection() (*PooledConnection, error) {
	atomic.AddInt64(&ecp.requestCount, 1)

	select {
	case conn := <-ecp.connections:
		if ecp.isConnectionHealthy(conn) {
			conn.mu.Lock()
			conn.lastUsed = time.Now()
			conn.useCount++
			conn.mu.Unlock()

			ecp.mu.Lock()
			ecp.activeConnections[conn.id] = conn
			ecp.mu.Unlock()

			return conn, nil
		}
		// Connection unhealthy, close and create new one
		ecp.closeConnection(conn)
		return ecp.createNewConnection()
	default:
		// No available connections, create new one if under limit
		currentSize := atomic.LoadInt32(&ecp.currentSize)
		if int(currentSize) < ecp.maxSize {
			return ecp.createNewConnection()
		}

		// Wait briefly for a connection to become available
		select {
		case conn := <-ecp.connections:
			if ecp.isConnectionHealthy(conn) {
				conn.mu.Lock()
				conn.lastUsed = time.Now()
				conn.useCount++
				conn.mu.Unlock()

				ecp.mu.Lock()
				ecp.activeConnections[conn.id] = conn
				ecp.mu.Unlock()

				return conn, nil
			}
			ecp.closeConnection(conn)
			return ecp.createNewConnection()
		case <-time.After(100 * time.Millisecond):
			return nil, fmt.Errorf("no connections available and pool at maximum size")
		case <-ecp.ctx.Done():
			return nil, fmt.Errorf("pool closed")
		}
	}
}

// ReturnConnection returns a connection to the pool
func (ecp *EnhancedConnectionPool) ReturnConnection(conn *PooledConnection) error {
	if conn == nil {
		return fmt.Errorf("cannot return nil connection")
	}

	ecp.mu.Lock()
	delete(ecp.activeConnections, conn.id)
	ecp.mu.Unlock()

	if !ecp.isConnectionHealthy(conn) {
		ecp.closeConnection(conn)
		return nil
	}

	// Check if pool is closed
	select {
	case <-ecp.ctx.Done():
		ecp.closeConnection(conn)
		return nil
	default:
	}

	// Return to pool or close if pool is full
	select {
	case ecp.connections <- conn:
		return nil
	case <-ecp.ctx.Done():
		ecp.closeConnection(conn)
		return nil
	default:
		// Pool full, close connection to maintain size
		ecp.closeConnection(conn)
		return nil
	}
}

// ScalePool adjusts the pool size based on current utilization
func (ecp *EnhancedConnectionPool) ScalePool() error {
	utilization := ecp.calculateUtilization()
	shouldScale, scaleUp := ecp.sizer.ShouldScale(utilization)

	if !shouldScale {
		return nil
	}

	currentSize := int(atomic.LoadInt32(&ecp.currentSize))
	newSize := ecp.sizer.CalculateOptimalSize(utilization, currentSize)

	if scaleUp && newSize > currentSize {
		return ecp.scaleUp(newSize - currentSize)
	} else if !scaleUp && newSize < currentSize {
		return ecp.scaleDown(currentSize - newSize)
	}

	return nil
}

// GetStats returns current pool statistics
func (ecp *EnhancedConnectionPool) GetStats() *EnhancedPoolStats {
	ecp.mu.RLock()
	activeCount := len(ecp.activeConnections)
	ecp.mu.RUnlock()

	availableCount := len(ecp.connections)
	currentSize := int(atomic.LoadInt32(&ecp.currentSize))
	targetSize := int(atomic.LoadInt32(&ecp.targetSize))
	requestCount := atomic.LoadInt64(&ecp.requestCount)

	return &EnhancedPoolStats{
		TotalConnections:     currentSize,
		ActiveConnections:    activeCount,
		AvailableConnections: availableCount,
		Utilization:          ecp.calculateUtilization(),
		RequestCount:         requestCount,
		MinSize:              ecp.minSize,
		MaxSize:              ecp.maxSize,
		CurrentSize:          currentSize,
		TargetSize:           targetSize,
	}
}

// Close closes the connection pool and all connections
func (ecp *EnhancedConnectionPool) Close() error {
	ecp.cancel()

	if ecp.scalingTicker != nil {
		ecp.scalingTicker.Stop()
	}
	if ecp.healthTicker != nil {
		ecp.healthTicker.Stop()
	}

	// Close all pooled connections
	for {
		select {
		case conn := <-ecp.connections:
			ecp.closeConnection(conn)
		default:
			goto closeActive
		}
	}

closeActive:
	// Close all active connections
	ecp.mu.Lock()
	for _, conn := range ecp.activeConnections {
		ecp.closeConnection(conn)
	}
	ecp.activeConnections = make(map[string]*PooledConnection)
	ecp.mu.Unlock()

	return nil
}

// Internal methods

func (ecp *EnhancedConnectionPool) createNewConnection() (*PooledConnection, error) {
	conn, err := ecp.createTCPConnection()
	if err != nil {
		return nil, err
	}

	id := ecp.generateConnectionID()
	pooledConn := &PooledConnection{
		conn:      conn,
		id:        id,
		createdAt: time.Now(),
		lastUsed:  time.Now(),
		useCount:  1,
		isHealthy: true,
	}

	atomic.AddInt32(&ecp.currentSize, 1)

	ecp.mu.Lock()
	ecp.activeConnections[id] = pooledConn
	ecp.mu.Unlock()

	return pooledConn, nil
}

func (ecp *EnhancedConnectionPool) createTCPConnection() (net.Conn, error) {
	maxRetries := 3
	baseDelay := 100 * time.Millisecond

	for attempt := 0; attempt < maxRetries; attempt++ {
		conn, err := net.DialTimeout("tcp", ecp.address, 3*time.Second)
		if err == nil {
			return conn, nil
		}

		if attempt == maxRetries-1 {
			return nil, fmt.Errorf("failed to connect to %s after %d attempts: %w", ecp.address, maxRetries, err)
		}

		// Exponential backoff with jitter
		multiplier := math.Pow(1.5, float64(attempt))
		delay := time.Duration(float64(baseDelay) * multiplier)
		jitter := time.Duration(float64(delay) * (rand.Float64() - 0.5) * 0.3)
		finalDelay := delay + jitter

		if finalDelay > time.Second {
			finalDelay = time.Second
		}

		time.Sleep(finalDelay)
	}

	return nil, fmt.Errorf("failed to create connection after %d attempts", maxRetries)
}

func (ecp *EnhancedConnectionPool) closeConnection(conn *PooledConnection) {
	if conn != nil && conn.conn != nil {
		if err := conn.conn.Close(); err != nil {
			// Log close errors but don't fail the operation since this is cleanup
			// Note: In production, this would use a proper logger
			_ = err
		}
		atomic.AddInt32(&ecp.currentSize, -1)
	}
}

func (ecp *EnhancedConnectionPool) isConnectionHealthy(conn *PooledConnection) bool {
	if conn == nil || conn.conn == nil {
		return false
	}

	// Simple health check: try to set a deadline
	if err := conn.conn.SetDeadline(time.Now().Add(time.Millisecond)); err != nil {
		return false
	}
	// Reset deadline
	if err := conn.conn.SetDeadline(time.Time{}); err != nil {
		// Log deadline reset errors but continue since this is cleanup
		// Note: In production, this would use a proper logger
		_ = err
	}

	conn.mu.Lock()
	conn.isHealthy = true
	conn.mu.Unlock()

	return true
}

func (ecp *EnhancedConnectionPool) generateConnectionID() string {
	id := atomic.AddInt64(&ecp.connectionIndex, 1)
	return fmt.Sprintf("conn-%d", id)
}

func (ecp *EnhancedConnectionPool) calculateUtilization() float64 {
	ecp.mu.RLock()
	activeCount := len(ecp.activeConnections)
	ecp.mu.RUnlock()

	currentSize := int(atomic.LoadInt32(&ecp.currentSize))
	if currentSize == 0 {
		return 0.0
	}

	return float64(activeCount) / float64(currentSize)
}

func (ecp *EnhancedConnectionPool) warmUp() error {
	initialSize := ecp.minSize
	successCount := 0

	for i := 0; i < initialSize; i++ {
		conn, err := ecp.createTCPConnection()
		if err != nil {
			if successCount == 0 {
				return fmt.Errorf("failed to create initial connection: %w", err)
			}
			continue
		}

		id := ecp.generateConnectionID()
		pooledConn := &PooledConnection{
			conn:      conn,
			id:        id,
			createdAt: time.Now(),
			lastUsed:  time.Now(),
			isHealthy: true,
		}

		select {
		case ecp.connections <- pooledConn:
			atomic.AddInt32(&ecp.currentSize, 1)
			successCount++
		default:
			if err := conn.Close(); err != nil {
				// Log close errors but continue since this is cleanup
				// Note: In production, this would use a proper logger
				_ = err
			}
			successCount++
		}
	}

	if successCount == 0 {
		return fmt.Errorf("failed to create any initial connections")
	}

	atomic.StoreInt32(&ecp.targetSize, int32(successCount))
	return nil
}

func (ecp *EnhancedConnectionPool) scaleUp(count int) error {
	for i := 0; i < count; i++ {
		conn, err := ecp.createTCPConnection()
		if err != nil {
			continue // Skip failed connections
		}

		id := ecp.generateConnectionID()
		pooledConn := &PooledConnection{
			conn:      conn,
			id:        id,
			createdAt: time.Now(),
			lastUsed:  time.Now(),
			isHealthy: true,
		}

		select {
		case ecp.connections <- pooledConn:
			atomic.AddInt32(&ecp.currentSize, 1)
		default:
			if err := conn.Close(); err != nil {
				// Log close errors but continue since this is cleanup
				// Note: In production, this would use a proper logger
				_ = err
			}
		}
	}
	return nil
}

func (ecp *EnhancedConnectionPool) scaleDown(count int) error {
	for i := 0; i < count; i++ {
		select {
		case conn := <-ecp.connections:
			ecp.closeConnection(conn)
		default:
			return nil // No more connections to remove
		}
	}
	return nil
}

func (ecp *EnhancedConnectionPool) backgroundScaling() {
	defer ecp.scalingTicker.Stop()

	for {
		select {
		case <-ecp.ctx.Done():
			return
		case <-ecp.scalingTicker.C:
			if err := ecp.ScalePool(); err != nil {
				// Log scaling errors but don't stop the background goroutine
				// Note: In production, this would use a proper logger
				_ = err
			}
		}
	}
}

func (ecp *EnhancedConnectionPool) backgroundHealthMonitoring() {
	defer ecp.healthTicker.Stop()

	for {
		select {
		case <-ecp.ctx.Done():
			return
		case <-ecp.healthTicker.C:
			ecp.performHealthCheck()
		}
	}
}

func (ecp *EnhancedConnectionPool) performHealthCheck() {
	connCount := len(ecp.connections)
	maxCheck := 3
	if connCount < maxCheck {
		maxCheck = connCount
	}

	for i := 0; i < maxCheck; i++ {
		select {
		case conn := <-ecp.connections:
			if ecp.isConnectionHealthy(conn) {
				select {
				case ecp.connections <- conn:
				default:
					ecp.closeConnection(conn)
				}
			} else {
				ecp.closeConnection(conn)
			}
		default:
			return
		}
	}
}

// PoolSizer methods

func (ps *PoolSizer) CalculateOptimalSize(currentLoad float64, currentSize int) int {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	var optimalSize int

	if currentLoad > ps.scaleUpThreshold {
		optimalSize = int(float64(currentSize) * ps.scaleUpFactor)
	} else if currentLoad < ps.scaleDownThreshold {
		optimalSize = int(float64(currentSize) * ps.scaleDownFactor)
	} else {
		return currentSize // No change needed
	}

	// Enforce limits
	if optimalSize < ps.minConnections {
		optimalSize = ps.minConnections
	}
	if optimalSize > ps.maxConnections {
		optimalSize = ps.maxConnections
	}

	return optimalSize
}

func (ps *PoolSizer) ShouldScale(utilization float64) (bool, bool) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Check cooldown period
	if time.Since(ps.lastScaleTime) < ps.cooldownPeriod {
		return false, false
	}

	scaleUp := utilization > ps.scaleUpThreshold
	scaleDown := utilization < ps.scaleDownThreshold

	if scaleUp || scaleDown {
		ps.lastScaleTime = time.Now()
		return true, scaleUp
	}

	return false, false
}

// Factory function to create enhanced pool with configuration
func CreateEnhancedPool(address string, config map[string]interface{}) (*EnhancedConnectionPool, error) {
	minSize := 2
	maxSize := 10

	if min, ok := config["min_size"].(int); ok && min > 0 {
		minSize = min
	}
	if max, ok := config["max_size"].(int); ok && max > minSize {
		maxSize = max
	}

	pool := NewEnhancedConnectionPool(address, minSize, maxSize)
	return pool, pool.Initialize()
}
