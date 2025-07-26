package gateway

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState int32

const (
	CircuitBreakerStateClosed CircuitBreakerState = iota
	CircuitBreakerStateOpen
	CircuitBreakerStateHalfOpen
)

const (
	StateStringClosed   = "closed"
	StateStringOpen     = "open"
	StateStringHalfOpen = "half-open"
	StateStringUnknown  = "unknown"
)

func (s CircuitBreakerState) String() string {
	switch s {
	case CircuitBreakerStateClosed:
		return StateStringClosed
	case CircuitBreakerStateOpen:
		return StateStringOpen
	case CircuitBreakerStateHalfOpen:
		return StateStringHalfOpen
	default:
		return StateStringUnknown
	}
}

// CircuitBreakerConfig contains configuration for a circuit breaker
type CircuitBreakerConfig struct {
	ErrorThreshold        int           `json:"error_threshold"`
	TimeoutDuration       time.Duration `json:"timeout_duration"`
	MaxHalfOpenRequests   int32         `json:"max_half_open_requests"`
	SuccessThreshold      int32         `json:"success_threshold"`
	MinRequestsToTrip     int32         `json:"min_requests_to_trip"`
	RollingWindowDuration time.Duration `json:"rolling_window_duration"`
}

// DefaultCircuitBreakerConfig returns default circuit breaker configuration
func DefaultCircuitBreakerConfig() *CircuitBreakerConfig {
	return &CircuitBreakerConfig{
		ErrorThreshold:        10,
		TimeoutDuration:       30 * time.Second,
		MaxHalfOpenRequests:   5,
		SuccessThreshold:      3,
		MinRequestsToTrip:     5,
		RollingWindowDuration: 60 * time.Second,
	}
}

// CircuitBreakerMetrics tracks metrics for a circuit breaker
type CircuitBreakerMetrics struct {
	TotalRequests       int64         `json:"total_requests"`
	SuccessfulRequests  int64         `json:"successful_requests"`
	FailedRequests      int64         `json:"failed_requests"`
	RejectedRequests    int64         `json:"rejected_requests"`
	LastStateChange     time.Time     `json:"last_state_change"`
	StateChangeCount    int64         `json:"state_change_count"`
	AverageResponseTime time.Duration `json:"average_response_time"`
	LastFailureTime     time.Time     `json:"last_failure_time"`
	LastSuccessTime     time.Time     `json:"last_success_time"`
}

// Copy creates a copy of the circuit breaker metrics
func (cbm *CircuitBreakerMetrics) Copy() *CircuitBreakerMetrics {
	return &CircuitBreakerMetrics{
		TotalRequests:       atomic.LoadInt64(&cbm.TotalRequests),
		SuccessfulRequests:  atomic.LoadInt64(&cbm.SuccessfulRequests),
		FailedRequests:      atomic.LoadInt64(&cbm.FailedRequests),
		RejectedRequests:    atomic.LoadInt64(&cbm.RejectedRequests),
		LastStateChange:     cbm.LastStateChange,
		StateChangeCount:    atomic.LoadInt64(&cbm.StateChangeCount),
		AverageResponseTime: cbm.AverageResponseTime,
		LastFailureTime:     cbm.LastFailureTime,
		LastSuccessTime:     cbm.LastSuccessTime,
	}
}

// GetErrorRate returns the current error rate
func (cbm *CircuitBreakerMetrics) GetErrorRate() float64 {
	total := atomic.LoadInt64(&cbm.TotalRequests)
	if total == 0 {
		return 0.0
	}
	failed := atomic.LoadInt64(&cbm.FailedRequests)
	return float64(failed) / float64(total)
}

// GetSuccessRate returns the current success rate
func (cbm *CircuitBreakerMetrics) GetSuccessRate() float64 {
	total := atomic.LoadInt64(&cbm.TotalRequests)
	if total == 0 {
		return 0.0
	}
	successful := atomic.LoadInt64(&cbm.SuccessfulRequests)
	return float64(successful) / float64(total)
}

// CircuitBreaker implements the circuit breaker pattern for fault tolerance
type CircuitBreaker struct {
	config               *CircuitBreakerConfig
	state                CircuitBreakerState
	errorCount           int32
	successCount         int32
	lastFailureTime      time.Time
	halfOpenSuccessCount int32
	requestCount         int32
	metrics              *CircuitBreakerMetrics
	mu                   sync.RWMutex

	// Rolling window tracking
	requestTimes   []time.Time
	requestResults []bool // true for success, false for failure
	windowMu       sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker with default configuration
func NewCircuitBreaker(errorThreshold int, timeoutDuration time.Duration, maxHalfOpenRequests int32) *CircuitBreaker {
	config := &CircuitBreakerConfig{
		ErrorThreshold:        errorThreshold,
		TimeoutDuration:       timeoutDuration,
		MaxHalfOpenRequests:   maxHalfOpenRequests,
		SuccessThreshold:      3,
		MinRequestsToTrip:     5,
		RollingWindowDuration: 60 * time.Second,
	}

	return NewCircuitBreakerWithConfig(config)
}

// NewCircuitBreakerWithConfig creates a new circuit breaker with custom configuration
func NewCircuitBreakerWithConfig(config *CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		config:       config,
		state:        CircuitBreakerStateClosed,
		errorCount:   0,
		successCount: 0,
		requestCount: 0,
		metrics: &CircuitBreakerMetrics{
			LastStateChange: time.Now(),
		},
		requestTimes:   make([]time.Time, 0),
		requestResults: make([]bool, 0),
	}
}

// CanExecute checks if a request can be executed through the circuit breaker
func (cb *CircuitBreaker) CanExecute() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	atomic.AddInt64(&cb.metrics.TotalRequests, 1)

	switch cb.state {
	case CircuitBreakerStateClosed:
		return true
	case CircuitBreakerStateOpen:
		// Check if timeout has passed to transition to half-open
		if time.Since(cb.lastFailureTime) >= cb.config.TimeoutDuration {
			cb.mu.RUnlock()
			cb.mu.Lock()
			// Double-check after acquiring write lock
			if cb.state == CircuitBreakerStateOpen && time.Since(cb.lastFailureTime) >= cb.config.TimeoutDuration {
				cb.transitionToHalfOpen()
			}
			cb.mu.Unlock()
			cb.mu.RLock()
			return cb.state == CircuitBreakerStateHalfOpen
		}
		atomic.AddInt64(&cb.metrics.RejectedRequests, 1)
		return false
	case CircuitBreakerStateHalfOpen:
		// Allow limited requests in half-open state
		if atomic.LoadInt32(&cb.halfOpenSuccessCount) < cb.config.MaxHalfOpenRequests {
			return true
		}
		atomic.AddInt64(&cb.metrics.RejectedRequests, 1)
		return false
	default:
		atomic.AddInt64(&cb.metrics.RejectedRequests, 1)
		return false
	}
}

// RecordSuccess records a successful request
func (cb *CircuitBreaker) RecordSuccess() {
	cb.recordResult(true)

	cb.mu.Lock()
	defer cb.mu.Unlock()

	atomic.AddInt64(&cb.metrics.SuccessfulRequests, 1)
	cb.metrics.LastSuccessTime = time.Now()

	switch cb.state {
	case CircuitBreakerStateClosed:
		// Reset error count on success
		atomic.StoreInt32(&cb.errorCount, 0)
	case CircuitBreakerStateHalfOpen:
		// Increment success count in half-open state
		count := atomic.AddInt32(&cb.halfOpenSuccessCount, 1)
		if count >= cb.config.SuccessThreshold {
			cb.transitionToClosed()
		}
	}
}

// RecordFailure records a failed request
func (cb *CircuitBreaker) RecordFailure() {
	cb.recordResult(false)

	cb.mu.Lock()
	defer cb.mu.Unlock()

	atomic.AddInt64(&cb.metrics.FailedRequests, 1)
	cb.lastFailureTime = time.Now()
	cb.metrics.LastFailureTime = cb.lastFailureTime

	switch cb.state {
	case CircuitBreakerStateClosed:
		errorCount := atomic.AddInt32(&cb.errorCount, 1)
		requestCount := atomic.LoadInt32(&cb.requestCount)

		// Check if we should trip the circuit breaker
		if requestCount >= cb.config.MinRequestsToTrip &&
			int(errorCount) >= cb.config.ErrorThreshold {
			cb.transitionToOpen()
		}
	case CircuitBreakerStateHalfOpen:
		// Any failure in half-open state transitions back to open
		cb.transitionToOpen()
	}
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetMetrics returns a copy of the circuit breaker metrics
func (cb *CircuitBreaker) GetMetrics() *CircuitBreakerMetrics {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.metrics.Copy()
}

// ForceOpen forces the circuit breaker to open state
func (cb *CircuitBreaker) ForceOpen() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.transitionToOpen()
}

// Reset resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.transitionToClosed()
}

// IsOpen returns true if the circuit breaker is in open state
func (cb *CircuitBreaker) IsOpen() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state == CircuitBreakerStateOpen
}

// IsHalfOpen returns true if the circuit breaker is in half-open state
func (cb *CircuitBreaker) IsHalfOpen() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state == CircuitBreakerStateHalfOpen
}

// IsClosed returns true if the circuit breaker is in closed state
func (cb *CircuitBreaker) IsClosed() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state == CircuitBreakerStateClosed
}

// recordResult records the result of a request for rolling window analysis
func (cb *CircuitBreaker) recordResult(success bool) {
	cb.windowMu.Lock()
	defer cb.windowMu.Unlock()

	now := time.Now()
	cb.requestTimes = append(cb.requestTimes, now)
	cb.requestResults = append(cb.requestResults, success)

	atomic.AddInt32(&cb.requestCount, 1)

	// Clean up old entries outside the rolling window
	cutoff := now.Add(-cb.config.RollingWindowDuration)
	validIndex := 0

	for i, t := range cb.requestTimes {
		if t.After(cutoff) {
			cb.requestTimes[validIndex] = cb.requestTimes[i]
			cb.requestResults[validIndex] = cb.requestResults[i]
			validIndex++
		}
	}

	cb.requestTimes = cb.requestTimes[:validIndex]
	cb.requestResults = cb.requestResults[:validIndex]
}

// transitionToOpen transitions the circuit breaker to open state
func (cb *CircuitBreaker) transitionToOpen() {
	if cb.state != CircuitBreakerStateOpen {
		cb.state = CircuitBreakerStateOpen
		cb.metrics.LastStateChange = time.Now()
		atomic.AddInt64(&cb.metrics.StateChangeCount, 1)
		atomic.StoreInt32(&cb.halfOpenSuccessCount, 0)
	}
}

// transitionToHalfOpen transitions the circuit breaker to half-open state
func (cb *CircuitBreaker) transitionToHalfOpen() {
	if cb.state != CircuitBreakerStateHalfOpen {
		cb.state = CircuitBreakerStateHalfOpen
		cb.metrics.LastStateChange = time.Now()
		atomic.AddInt64(&cb.metrics.StateChangeCount, 1)
		atomic.StoreInt32(&cb.halfOpenSuccessCount, 0)
	}
}

// transitionToClosed transitions the circuit breaker to closed state
func (cb *CircuitBreaker) transitionToClosed() {
	if cb.state != CircuitBreakerStateClosed {
		cb.state = CircuitBreakerStateClosed
		cb.metrics.LastStateChange = time.Now()
		atomic.AddInt64(&cb.metrics.StateChangeCount, 1)
		atomic.StoreInt32(&cb.errorCount, 0)
		atomic.StoreInt32(&cb.halfOpenSuccessCount, 0)
		atomic.StoreInt32(&cb.requestCount, 0)
	}
}

// GetRollingWindowStats returns statistics for the current rolling window
func (cb *CircuitBreaker) GetRollingWindowStats() *RollingWindowStats {
	cb.windowMu.RLock()
	defer cb.windowMu.RUnlock()

	now := time.Now()
	cutoff := now.Add(-cb.config.RollingWindowDuration)

	var totalRequests, successfulRequests, failedRequests int

	for i, t := range cb.requestTimes {
		if t.After(cutoff) {
			totalRequests++
			if cb.requestResults[i] {
				successfulRequests++
			} else {
				failedRequests++
			}
		}
	}

	stats := &RollingWindowStats{
		WindowDuration:     cb.config.RollingWindowDuration,
		TotalRequests:      totalRequests,
		SuccessfulRequests: successfulRequests,
		FailedRequests:     failedRequests,
		ErrorRate:          0.0,
		SuccessRate:        0.0,
	}

	if totalRequests > 0 {
		stats.ErrorRate = float64(failedRequests) / float64(totalRequests)
		stats.SuccessRate = float64(successfulRequests) / float64(totalRequests)
	}

	return stats
}

// RollingWindowStats contains statistics for a rolling window
type RollingWindowStats struct {
	WindowDuration     time.Duration `json:"window_duration"`
	TotalRequests      int           `json:"total_requests"`
	SuccessfulRequests int           `json:"successful_requests"`
	FailedRequests     int           `json:"failed_requests"`
	ErrorRate          float64       `json:"error_rate"`
	SuccessRate        float64       `json:"success_rate"`
}

// CircuitBreakerManager manages multiple circuit breakers
type CircuitBreakerManager struct {
	circuitBreakers map[string]*CircuitBreaker
	defaultConfig   *CircuitBreakerConfig
	mu              sync.RWMutex
}

// NewCircuitBreakerManager creates a new circuit breaker manager
func NewCircuitBreakerManager() *CircuitBreakerManager {
	return &CircuitBreakerManager{
		circuitBreakers: make(map[string]*CircuitBreaker),
		defaultConfig:   DefaultCircuitBreakerConfig(),
	}
}

// GetCircuitBreaker gets or creates a circuit breaker for the given key
func (cbm *CircuitBreakerManager) GetCircuitBreaker(key string) *CircuitBreaker {
	cbm.mu.RLock()
	cb, exists := cbm.circuitBreakers[key]
	cbm.mu.RUnlock()

	if exists {
		return cb
	}

	cbm.mu.Lock()
	defer cbm.mu.Unlock()

	// Double-check after acquiring write lock
	if cb, exists := cbm.circuitBreakers[key]; exists {
		return cb
	}

	// Create new circuit breaker
	cb = NewCircuitBreakerWithConfig(cbm.defaultConfig)
	cbm.circuitBreakers[key] = cb
	return cb
}

// CreateCircuitBreaker creates a circuit breaker with custom configuration
func (cbm *CircuitBreakerManager) CreateCircuitBreaker(key string, config *CircuitBreakerConfig) *CircuitBreaker {
	cbm.mu.Lock()
	defer cbm.mu.Unlock()

	cb := NewCircuitBreakerWithConfig(config)
	cbm.circuitBreakers[key] = cb
	return cb
}

// RemoveCircuitBreaker removes a circuit breaker
func (cbm *CircuitBreakerManager) RemoveCircuitBreaker(key string) {
	cbm.mu.Lock()
	defer cbm.mu.Unlock()
	delete(cbm.circuitBreakers, key)
}

// GetAllCircuitBreakers returns all circuit breakers
func (cbm *CircuitBreakerManager) GetAllCircuitBreakers() map[string]*CircuitBreaker {
	cbm.mu.RLock()
	defer cbm.mu.RUnlock()

	result := make(map[string]*CircuitBreaker)
	for k, v := range cbm.circuitBreakers {
		result[k] = v
	}
	return result
}

// ResetAll resets all circuit breakers
func (cbm *CircuitBreakerManager) ResetAll() {
	cbm.mu.RLock()
	defer cbm.mu.RUnlock()

	for _, cb := range cbm.circuitBreakers {
		cb.Reset()
	}
}

// GetHealthyCircuitBreakers returns circuit breakers that are not in open state
func (cbm *CircuitBreakerManager) GetHealthyCircuitBreakers() map[string]*CircuitBreaker {
	cbm.mu.RLock()
	defer cbm.mu.RUnlock()

	result := make(map[string]*CircuitBreaker)
	for k, v := range cbm.circuitBreakers {
		if !v.IsOpen() {
			result[k] = v
		}
	}
	return result
}

// GetOpenCircuitBreakers returns circuit breakers that are in open state
func (cbm *CircuitBreakerManager) GetOpenCircuitBreakers() map[string]*CircuitBreaker {
	cbm.mu.RLock()
	defer cbm.mu.RUnlock()

	result := make(map[string]*CircuitBreaker)
	for k, v := range cbm.circuitBreakers {
		if v.IsOpen() {
			result[k] = v
		}
	}
	return result
}

// GetMetrics returns metrics for all circuit breakers
func (cbm *CircuitBreakerManager) GetMetrics() map[string]*CircuitBreakerMetrics {
	cbm.mu.RLock()
	defer cbm.mu.RUnlock()

	result := make(map[string]*CircuitBreakerMetrics)
	for k, v := range cbm.circuitBreakers {
		result[k] = v.GetMetrics()
	}
	return result
}

// SetDefaultConfig sets the default configuration for new circuit breakers
func (cbm *CircuitBreakerManager) SetDefaultConfig(config *CircuitBreakerConfig) {
	cbm.mu.Lock()
	defer cbm.mu.Unlock()
	cbm.defaultConfig = config
}

// ExecuteWithCircuitBreaker executes a function with circuit breaker protection
func (cbm *CircuitBreakerManager) ExecuteWithCircuitBreaker(key string, fn func() error) error {
	cb := cbm.GetCircuitBreaker(key)

	if !cb.CanExecute() {
		return fmt.Errorf("circuit breaker %s is open", key)
	}

	err := fn()
	if err != nil {
		cb.RecordFailure()
		return err
	}

	cb.RecordSuccess()
	return nil
}
