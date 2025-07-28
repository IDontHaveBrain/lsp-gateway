package client

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	mcp "lsp-gateway/tests/mcp"
)

type ConnectionManager struct {
	transport       mcp.Transport
	config          MCPTestConfig
	logger          *TestLogger
	
	connectionState atomic.Value
	healthChecker   *HealthChecker
	circuitBreaker  *CircuitBreaker
	reconnectPolicy *ReconnectPolicy
	metrics         *ConnectionMetrics
	
	mu               sync.RWMutex
	lastConnected    time.Time
	lastHealthCheck  time.Time
	reconnectCount   int64
	shutdown         chan struct{}
	shutdownOnce     sync.Once
}

type ConnectionState int

const (
	StateDisconnected ConnectionState = iota
	StateConnecting
	StateConnected
	StateReconnecting
	StateFailed
	StateClosed
)

func (s ConnectionState) String() string {
	switch s {
	case StateDisconnected:
		return "Disconnected"
	case StateConnecting:
		return "Connecting"
	case StateConnected:
		return "Connected"
	case StateReconnecting:
		return "Reconnecting"
	case StateFailed:
		return "Failed"
	case StateClosed:
		return "Closed"
	default:
		return "Unknown"
	}
}

type HealthChecker struct {
	config   *HealthCheckConfig
	logger   *TestLogger
	
	failureCount   int32
	successCount   int32
	lastCheck      time.Time
	lastHealthy    time.Time
	running        bool
	mu             sync.RWMutex
	stopCh         chan struct{}
}

type CircuitBreaker struct {
	config     *CircuitBreakerConfig
	logger     *TestLogger
	
	state         atomic.Value
	failureCount  int32
	successCount  int32
	lastFailure   time.Time
	lastSuccess   time.Time
	mu            sync.RWMutex
}

type CircuitBreakerState int

const (
	CircuitClosed CircuitBreakerState = iota
	CircuitOpen
	CircuitHalfOpen
)

func (s CircuitBreakerState) String() string {
	switch s {
	case CircuitClosed:
		return "Closed"
	case CircuitOpen:
		return "Open"
	case CircuitHalfOpen:
		return "HalfOpen"
	default:
		return "Unknown"
	}
}

type ReconnectPolicy struct {
	maxAttempts   int
	initialDelay  time.Duration
	maxDelay      time.Duration
	backoffFactor float64
	jitterEnabled bool
}

type ConnectionMetrics struct {
	TotalAttempts      int64
	SuccessfulConns    int64
	FailedConns        int64
	ReconnectAttempts  int64
	CircuitBreakerTrips int64
	HealthCheckFailures int64
	LastConnected      time.Time
	LastDisconnected   time.Time
	ConnectionDuration time.Duration
	mu                 sync.RWMutex
}

func NewConnectionManager(transport mcp.Transport, config MCPTestConfig, logger *TestLogger) *ConnectionManager {
	cm := &ConnectionManager{
		transport: transport,
		config:    config,
		logger:    logger,
		shutdown:  make(chan struct{}),
		metrics:   &ConnectionMetrics{},
	}
	
	cm.connectionState.Store(StateDisconnected)
	
	if config.HealthCheckConfig != nil && config.HealthCheckConfig.Enabled {
		cm.healthChecker = NewHealthChecker(config.HealthCheckConfig, logger)
	}
	
	if config.CircuitBreaker != nil {
		cm.circuitBreaker = NewCircuitBreaker(config.CircuitBreaker, logger)
	}
	
	cm.reconnectPolicy = &ReconnectPolicy{
		maxAttempts:   config.MaxRetries,
		initialDelay:  config.RetryDelay,
		maxDelay:      config.RetryDelay * time.Duration(config.MaxRetries),
		backoffFactor: config.BackoffMultiplier,
		jitterEnabled: true,
	}
	
	return cm
}

func (cm *ConnectionManager) Connect(ctx context.Context) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	if cm.getConnectionState() == StateConnected {
		return nil
	}
	
	if cm.circuitBreaker != nil && !cm.circuitBreaker.CanAttempt() {
		return fmt.Errorf("circuit breaker is open, cannot attempt connection")
	}
	
	cm.setConnectionState(StateConnecting)
	cm.recordConnectionAttempt()
	
	start := time.Now()
	err := cm.transport.Connect(ctx)
	duration := time.Since(start)
	
	if err != nil {
		cm.setConnectionState(StateDisconnected)
		cm.recordConnectionFailure(duration)
		if cm.circuitBreaker != nil {
			cm.circuitBreaker.RecordFailure()
		}
		cm.logger.Error("Connection attempt failed", "error", err, "duration", duration)
		return fmt.Errorf("transport connection failed: %w", err)
	}
	
	cm.setConnectionState(StateConnected)
	cm.recordConnectionSuccess(duration)
	cm.lastConnected = time.Now()
	
	if cm.circuitBreaker != nil {
		cm.circuitBreaker.RecordSuccess()
	}
	
	if cm.healthChecker != nil {
		go cm.healthChecker.Start(cm.transport)
	}
	
	cm.logger.Info("Connection established successfully", "duration", duration)
	return nil
}

func (cm *ConnectionManager) Disconnect() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	if cm.getConnectionState() == StateDisconnected {
		return nil
	}
	
	cm.setConnectionState(StateDisconnected)
	
	if cm.healthChecker != nil {
		cm.healthChecker.Stop()
	}
	
	err := cm.transport.Disconnect()
	cm.recordDisconnection()
	
	if err != nil {
		cm.logger.Error("Disconnect failed", "error", err)
		return fmt.Errorf("transport disconnect failed: %w", err)
	}
	
	cm.logger.Info("Disconnected successfully")
	return nil
}

func (cm *ConnectionManager) IsConnected() bool {
	return cm.getConnectionState() == StateConnected
}

func (cm *ConnectionManager) IsHealthy() bool {
	if cm.healthChecker == nil {
		return cm.IsConnected()
	}
	return cm.healthChecker.IsHealthy()
}

func (cm *ConnectionManager) AttemptReconnect(ctx context.Context) error {
	if !cm.config.AutoReconnect {
		return fmt.Errorf("auto-reconnect is disabled")
	}
	
	cm.setConnectionState(StateReconnecting)
	atomic.AddInt64(&cm.reconnectCount, 1)
	
	for attempt := 1; attempt <= cm.reconnectPolicy.maxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-cm.shutdown:
			return fmt.Errorf("connection manager shutdown")
		default:
		}
		
		cm.logger.Info("Attempting reconnection", "attempt", attempt, "maxAttempts", cm.reconnectPolicy.maxAttempts)
		
		if err := cm.Connect(ctx); err == nil {
			cm.logger.Info("Reconnection successful", "attempt", attempt)
			return nil
		}
		
		if attempt < cm.reconnectPolicy.maxAttempts {
			delay := cm.calculateBackoffDelay(attempt)
			cm.logger.Info("Reconnection failed, retrying", "attempt", attempt, "delay", delay)
			
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			case <-cm.shutdown:
				return fmt.Errorf("connection manager shutdown")
			}
		}
	}
	
	cm.setConnectionState(StateFailed)
	return fmt.Errorf("reconnection failed after %d attempts", cm.reconnectPolicy.maxAttempts)
}

func (cm *ConnectionManager) GetMetrics() *ConnectionMetrics {
	cm.metrics.mu.RLock()
	defer cm.metrics.mu.RUnlock()
	
	return &ConnectionMetrics{
		TotalAttempts:       cm.metrics.TotalAttempts,
		SuccessfulConns:     cm.metrics.SuccessfulConns,
		FailedConns:         cm.metrics.FailedConns,
		ReconnectAttempts:   atomic.LoadInt64(&cm.reconnectCount),
		CircuitBreakerTrips: cm.getCircuitBreakerTrips(),
		HealthCheckFailures: cm.getHealthCheckFailures(),
		LastConnected:       cm.lastConnected,
		LastDisconnected:    cm.metrics.LastDisconnected,
		ConnectionDuration:  cm.metrics.ConnectionDuration,
	}
}

func (cm *ConnectionManager) Close() error {
	cm.shutdownOnce.Do(func() {
		close(cm.shutdown)
	})
	
	var errors []error
	
	if cm.healthChecker != nil {
		if err := cm.healthChecker.Stop(); err != nil {
			errors = append(errors, fmt.Errorf("health checker stop failed: %w", err))
		}
	}
	
	if err := cm.Disconnect(); err != nil {
		errors = append(errors, fmt.Errorf("disconnect failed: %w", err))
	}
	
	cm.setConnectionState(StateClosed)
	
	if len(errors) > 0 {
		return fmt.Errorf("close errors: %v", errors)
	}
	
	return nil
}

func (cm *ConnectionManager) getConnectionState() ConnectionState {
	return cm.connectionState.Load().(ConnectionState)
}

func (cm *ConnectionManager) setConnectionState(state ConnectionState) {
	oldState := cm.getConnectionState()
	cm.connectionState.Store(state)
	cm.logger.Debug("Connection state changed", "from", oldState, "to", state)
}

func (cm *ConnectionManager) recordConnectionAttempt() {
	cm.metrics.mu.Lock()
	defer cm.metrics.mu.Unlock()
	atomic.AddInt64(&cm.metrics.TotalAttempts, 1)
}

func (cm *ConnectionManager) recordConnectionSuccess(duration time.Duration) {
	cm.metrics.mu.Lock()
	defer cm.metrics.mu.Unlock()
	atomic.AddInt64(&cm.metrics.SuccessfulConns, 1)
	cm.metrics.ConnectionDuration = duration
}

func (cm *ConnectionManager) recordConnectionFailure(duration time.Duration) {
	cm.metrics.mu.Lock()
	defer cm.metrics.mu.Unlock()
	atomic.AddInt64(&cm.metrics.FailedConns, 1)
}

func (cm *ConnectionManager) recordDisconnection() {
	cm.metrics.mu.Lock()
	defer cm.metrics.mu.Unlock()
	cm.metrics.LastDisconnected = time.Now()
}

func (cm *ConnectionManager) calculateBackoffDelay(attempt int) time.Duration {
	delay := float64(cm.reconnectPolicy.initialDelay)
	for i := 1; i < attempt; i++ {
		delay *= cm.reconnectPolicy.backoffFactor
	}
	
	if delay > float64(cm.reconnectPolicy.maxDelay) {
		delay = float64(cm.reconnectPolicy.maxDelay)
	}
	
	if cm.reconnectPolicy.jitterEnabled {
		jitter := delay * 0.1
		delay += (jitter * 2 * (float64(time.Now().UnixNano()%1000) / 1000)) - jitter
	}
	
	return time.Duration(delay)
}

func (cm *ConnectionManager) getCircuitBreakerTrips() int64 {
	if cm.circuitBreaker == nil {
		return 0
	}
	return cm.circuitBreaker.GetTrips()
}

func (cm *ConnectionManager) getHealthCheckFailures() int64 {
	if cm.healthChecker == nil {
		return 0
	}
	return cm.healthChecker.GetFailureCount()
}

func NewHealthChecker(config *HealthCheckConfig, logger *TestLogger) *HealthChecker {
	return &HealthChecker{
		config: config,
		logger: logger,
		stopCh: make(chan struct{}),
	}
}

func (hc *HealthChecker) Start(transport mcp.Transport) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	
	if hc.running {
		return
	}
	
	hc.running = true
	go hc.runHealthChecks(transport)
}

func (hc *HealthChecker) Stop() error {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	
	if !hc.running {
		return nil
	}
	
	hc.running = false
	close(hc.stopCh)
	return nil
}

func (hc *HealthChecker) IsHealthy() bool {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	
	failures := atomic.LoadInt32(&hc.failureCount)
	return failures < int32(hc.config.FailureCount)
}

func (hc *HealthChecker) GetFailureCount() int64 {
	return int64(atomic.LoadInt32(&hc.failureCount))
}

func (hc *HealthChecker) runHealthChecks(transport mcp.Transport) {
	ticker := time.NewTicker(hc.config.Interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			hc.performHealthCheck(transport)
		case <-hc.stopCh:
			return
		}
	}
}

func (hc *HealthChecker) performHealthCheck(transport mcp.Transport) {
	hc.mu.Lock()
	hc.lastCheck = time.Now()
	hc.mu.Unlock()
	
	isHealthy := transport.IsConnected()
	
	if isHealthy {
		successCount := atomic.AddInt32(&hc.successCount, 1)
		if successCount >= int32(hc.config.RecoveryCount) {
			atomic.StoreInt32(&hc.failureCount, 0)
			hc.mu.Lock()
			hc.lastHealthy = time.Now()
			hc.mu.Unlock()
		}
	} else {
		atomic.StoreInt32(&hc.successCount, 0)
		atomic.AddInt32(&hc.failureCount, 1)
		hc.logger.Warn("Health check failed", "failures", atomic.LoadInt32(&hc.failureCount))
	}
}

func NewCircuitBreaker(config *CircuitBreakerConfig, logger *TestLogger) *CircuitBreaker {
	cb := &CircuitBreaker{
		config: config,
		logger: logger,
	}
	cb.state.Store(CircuitClosed)
	return cb
}

func (cb *CircuitBreaker) CanAttempt() bool {
	state := cb.getState()
	
	switch state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		cb.mu.RLock()
		lastFailure := cb.lastFailure
		cb.mu.RUnlock()
		
		if time.Since(lastFailure) > cb.config.Timeout {
			cb.setState(CircuitHalfOpen)
			return true
		}
		return false
	case CircuitHalfOpen:
		return atomic.LoadInt32(&cb.successCount) < int32(cb.config.MaxRequests)
	default:
		return false
	}
}

func (cb *CircuitBreaker) RecordSuccess() {
	state := cb.getState()
	cb.mu.Lock()
	cb.lastSuccess = time.Now()
	cb.mu.Unlock()
	
	successCount := atomic.AddInt32(&cb.successCount, 1)
	
	if state == CircuitHalfOpen && successCount >= int32(cb.config.HealthThreshold) {
		cb.setState(CircuitClosed)
		atomic.StoreInt32(&cb.failureCount, 0)
		atomic.StoreInt32(&cb.successCount, 0)
		cb.logger.Info("Circuit breaker reset to closed state")
	}
}

func (cb *CircuitBreaker) RecordFailure() {
	atomic.StoreInt32(&cb.successCount, 0)
	failureCount := atomic.AddInt32(&cb.failureCount, 1)
	
	cb.mu.Lock()
	cb.lastFailure = time.Now()
	cb.mu.Unlock()
	
	if cb.getState() == CircuitClosed && failureCount >= int32(cb.config.MaxFailures) {
		cb.setState(CircuitOpen)
		cb.logger.Warn("Circuit breaker tripped to open state", "failures", failureCount)
	}
}

func (cb *CircuitBreaker) GetTrips() int64 {
	if cb.getState() == CircuitOpen {
		return 1
	}
	return 0
}

func (cb *CircuitBreaker) getState() CircuitBreakerState {
	return cb.state.Load().(CircuitBreakerState)
}

func (cb *CircuitBreaker) setState(newState CircuitBreakerState) {
	oldState := cb.getState()
	cb.state.Store(newState)
	cb.logger.Debug("Circuit breaker state changed", "from", oldState, "to", newState)
}