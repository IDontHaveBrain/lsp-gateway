package testutils

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

type CircuitBreakerState string

const (
	CircuitBreakerClosed   CircuitBreakerState = "CLOSED"
	CircuitBreakerOpen     CircuitBreakerState = "OPEN"
	CircuitBreakerHalfOpen CircuitBreakerState = "HALF_OPEN"
)

type CircuitBreaker struct {
	name             string
	maxFailures      int32
	timeout          time.Duration
	state            CircuitBreakerState
	failures         int32
	lastFailureTime  time.Time
	nextRetryTime    time.Time
	successCount     int32
	requestCount     int32
	mu               sync.RWMutex
}

func NewCircuitBreaker(name string, maxFailures int32, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		name:        name,
		maxFailures: maxFailures,
		timeout:     timeout,
		state:       CircuitBreakerClosed,
	}
}

func (cb *CircuitBreaker) Call(ctx context.Context, fn func() error) error {
	if !cb.allowRequest() {
		return fmt.Errorf("circuit breaker %s is open", cb.name)
	}

	atomic.AddInt32(&cb.requestCount, 1)
	
	err := fn()
	if err != nil {
		cb.RecordFailure()
		return err
	}

	cb.RecordSuccess()
	return nil
}

func (cb *CircuitBreaker) allowRequest() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case CircuitBreakerClosed:
		return true
	case CircuitBreakerOpen:
		return time.Now().After(cb.nextRetryTime)
	case CircuitBreakerHalfOpen:
		return true
	default:
		return false
	}
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	atomic.AddInt32(&cb.successCount, 1)

	if cb.state == CircuitBreakerHalfOpen {
		cb.state = CircuitBreakerClosed
		cb.failures = 0
	}
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	atomic.AddInt32(&cb.failures, 1)
	cb.lastFailureTime = time.Now()

	if cb.failures >= cb.maxFailures {
		cb.state = CircuitBreakerOpen
		cb.nextRetryTime = time.Now().Add(cb.timeout)
	} else if cb.state == CircuitBreakerHalfOpen {
		cb.state = CircuitBreakerOpen
		cb.nextRetryTime = time.Now().Add(cb.timeout)
	}
}

func (cb *CircuitBreaker) State() string {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if cb.state == CircuitBreakerOpen && time.Now().After(cb.nextRetryTime) {
		cb.mu.RUnlock()
		cb.mu.Lock()
		cb.state = CircuitBreakerHalfOpen
		cb.mu.Unlock()
		cb.mu.RLock()
	}

	return string(cb.state)
}

func (cb *CircuitBreaker) GetStats() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return map[string]interface{}{
		"name":          cb.name,
		"state":         string(cb.state),
		"failures":      atomic.LoadInt32(&cb.failures),
		"requests":      atomic.LoadInt32(&cb.requestCount),
		"successes":     atomic.LoadInt32(&cb.successCount),
		"max_failures":  cb.maxFailures,
		"timeout":       cb.timeout,
		"next_retry":    cb.nextRetryTime,
		"last_failure":  cb.lastFailureTime,
	}
}

type RecoveryStrategy func(ctx context.Context, err *PythonRepoError) error

type PythonPatternsErrorHandler struct {
	config           ErrorRecoveryConfig
	logger           *PythonPatternsLogger
	circuitBreakers  map[string]*CircuitBreaker
	retryStrategies  map[ErrorType]RecoveryStrategy
	errorCounts      map[ErrorType]int64
	mu               sync.RWMutex
}

func NewPythonPatternsErrorHandler(logger *PythonPatternsLogger) *PythonPatternsErrorHandler {
	handler := &PythonPatternsErrorHandler{
		config:          DefaultErrorRecoveryConfig(),
		logger:          logger,
		circuitBreakers: make(map[string]*CircuitBreaker),
		retryStrategies: make(map[ErrorType]RecoveryStrategy),
		errorCounts:     make(map[ErrorType]int64),
	}

	handler.initializeCircuitBreakers()
	handler.initializeRecoveryStrategies()

	return handler
}

func (eh *PythonPatternsErrorHandler) initializeCircuitBreakers() {
	eh.circuitBreakers["server_startup"] = NewCircuitBreaker("server_startup", 3, 30*time.Second)
	eh.circuitBreakers["lsp_requests"] = NewCircuitBreaker("lsp_requests", 5, 10*time.Second)
	eh.circuitBreakers["repository_operations"] = NewCircuitBreaker("repository_operations", 3, 60*time.Second)
	eh.circuitBreakers["network_operations"] = NewCircuitBreaker("network_operations", 5, 15*time.Second)
}

func (eh *PythonPatternsErrorHandler) initializeRecoveryStrategies() {
	eh.retryStrategies[ErrorTypeTimeout] = eh.handleTimeoutError
	eh.retryStrategies[ErrorTypeNetwork] = eh.handleNetworkError
	eh.retryStrategies[ErrorTypeGit] = eh.handleGitError
	eh.retryStrategies[ErrorTypeLSP] = eh.handleLSPError
	eh.retryStrategies[ErrorTypeResource] = eh.handleResourceError
	eh.retryStrategies[ErrorTypeConcurrency] = eh.handleConcurrencyError
}

func (eh *PythonPatternsErrorHandler) HandleError(ctx context.Context, err error, operation string) error {
	if err == nil {
		return nil
	}

	pythonErr, ok := err.(*PythonRepoError)
	if !ok {
		pythonErr = NewPythonRepoError(ErrorTypeValidation, operation, err)
	}

	eh.recordError(pythonErr.Type)

	eh.logger.Error("Error occurred during operation", map[string]interface{}{
		"operation":    operation,
		"error_type":   pythonErr.Type,
		"message":      pythonErr.Message,
		"retry_count":  pythonErr.RetryCount,
		"recoverable":  pythonErr.Recoverable,
		"context":      pythonErr.Context,
	})

	if !pythonErr.Recoverable || pythonErr.RetryCount >= eh.config.MaxRetries {
		eh.logger.Error("Error is not recoverable or max retries exceeded", map[string]interface{}{
			"error":         pythonErr,
			"max_retries":   eh.config.MaxRetries,
		})
		return pythonErr
	}

	circuitBreakerKey := eh.getCircuitBreakerKey(pythonErr.Type)
	circuitBreaker := eh.circuitBreakers[circuitBreakerKey]
	if circuitBreaker != nil && circuitBreaker.State() == string(CircuitBreakerOpen) {
		eh.logger.Warn("Circuit breaker is open, skipping recovery attempt", map[string]interface{}{
			"circuit_breaker": circuitBreakerKey,
			"error_type":      pythonErr.Type,
		})
		return fmt.Errorf("circuit breaker %s is open: %w", circuitBreakerKey, pythonErr)
	}

	return eh.attemptRecovery(ctx, pythonErr)
}

func (eh *PythonPatternsErrorHandler) attemptRecovery(ctx context.Context, err *PythonRepoError) error {
	strategy, exists := eh.retryStrategies[err.Type]
	if !exists {
		eh.logger.Warn("No recovery strategy found for error type", map[string]interface{}{
			"error_type": err.Type,
		})
		return err
	}

	retryDelay := eh.calculateRetryDelay(err.RetryCount)
	eh.logger.Info("Attempting error recovery", map[string]interface{}{
		"error_type":  err.Type,
		"retry_count": err.RetryCount,
		"retry_delay": retryDelay,
	})

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(retryDelay):
	}

	err.IncrementRetry()
	return strategy(ctx, err)
}

func (eh *PythonPatternsErrorHandler) calculateRetryDelay(retryCount int) time.Duration {
	if retryCount == 0 {
		return eh.config.RetryDelay
	}

	delay := float64(eh.config.RetryDelay) * math.Pow(eh.config.BackoffMultiplier, float64(retryCount))
	maxDelay := float64(eh.config.MaxRetryDelay)
	
	if delay > maxDelay {
		delay = maxDelay
	}

	return time.Duration(delay)
}

func (eh *PythonPatternsErrorHandler) handleTimeoutError(ctx context.Context, err *PythonRepoError) error {
	eh.logger.Debug("Handling timeout error", map[string]interface{}{
		"error": err,
	})

	newTimeout := time.Duration(float64(5*time.Second) * eh.config.TimeoutMultiplier)
	err.WithContext("adjusted_timeout", newTimeout)

	return fmt.Errorf("timeout error recovery attempted: %w", err)
}

func (eh *PythonPatternsErrorHandler) handleNetworkError(ctx context.Context, err *PythonRepoError) error {
	eh.logger.Debug("Handling network error", map[string]interface{}{
		"error": err,
	})

	circuitBreaker := eh.circuitBreakers["network_operations"]
	if circuitBreaker != nil {
		circuitBreaker.RecordFailure()
	}

	return fmt.Errorf("network error recovery attempted: %w", err)
}

func (eh *PythonPatternsErrorHandler) handleGitError(ctx context.Context, err *PythonRepoError) error {
	eh.logger.Debug("Handling git error", map[string]interface{}{
		"error": err,
	})

	circuitBreaker := eh.circuitBreakers["repository_operations"]
	if circuitBreaker != nil {
		circuitBreaker.RecordFailure()
	}

	return fmt.Errorf("git error recovery attempted: %w", err)
}

func (eh *PythonPatternsErrorHandler) handleLSPError(ctx context.Context, err *PythonRepoError) error {
	eh.logger.Debug("Handling LSP error", map[string]interface{}{
		"error": err,
	})

	circuitBreaker := eh.circuitBreakers["lsp_requests"]
	if circuitBreaker != nil {
		circuitBreaker.RecordFailure()
	}

	return fmt.Errorf("LSP error recovery attempted: %w", err)
}

func (eh *PythonPatternsErrorHandler) handleResourceError(ctx context.Context, err *PythonRepoError) error {
	eh.logger.Debug("Handling resource error", map[string]interface{}{
		"error": err,
	})

	return fmt.Errorf("resource error recovery attempted: %w", err)
}

func (eh *PythonPatternsErrorHandler) handleConcurrencyError(ctx context.Context, err *PythonRepoError) error {
	eh.logger.Debug("Handling concurrency error", map[string]interface{}{
		"error": err,
	})

	backoffDelay := time.Duration(float64(time.Second) * math.Pow(2, float64(err.RetryCount)))
	if backoffDelay > 10*time.Second {
		backoffDelay = 10 * time.Second
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(backoffDelay):
	}

	return fmt.Errorf("concurrency error recovery attempted: %w", err)
}

func (eh *PythonPatternsErrorHandler) recordError(errorType ErrorType) {
	eh.mu.Lock()
	defer eh.mu.Unlock()
	eh.errorCounts[errorType]++
}

func (eh *PythonPatternsErrorHandler) getCircuitBreakerKey(errorType ErrorType) string {
	switch errorType {
	case ErrorTypeNetwork:
		return "network_operations"
	case ErrorTypeGit, ErrorTypeRepository:
		return "repository_operations"
	case ErrorTypeLSP:
		return "lsp_requests"
	default:
		return "server_startup"
	}
}

func (eh *PythonPatternsErrorHandler) GetErrorStats() map[string]interface{} {
	eh.mu.RLock()
	defer eh.mu.RUnlock()

	stats := map[string]interface{}{
		"error_counts": make(map[string]int64),
		"circuit_breakers": make(map[string]interface{}),
	}

	for errorType, count := range eh.errorCounts {
		stats["error_counts"].(map[string]int64)[string(errorType)] = count
	}

	for name, cb := range eh.circuitBreakers {
		stats["circuit_breakers"].(map[string]interface{})[name] = cb.GetStats()
	}

	return stats
}

func (eh *PythonPatternsErrorHandler) IsCircuitBreakerOpen(operation string) bool {
	eh.mu.RLock()
	defer eh.mu.RUnlock()

	cb, exists := eh.circuitBreakers[operation]
	if !exists {
		return false
	}

	return cb.State() == string(CircuitBreakerOpen)
}

func (eh *PythonPatternsErrorHandler) ResetCircuitBreaker(name string) {
	eh.mu.Lock()
	defer eh.mu.Unlock()

	if cb, exists := eh.circuitBreakers[name]; exists {
		cb.mu.Lock()
		cb.state = CircuitBreakerClosed
		cb.failures = 0
		cb.mu.Unlock()

		eh.logger.Info("Circuit breaker reset", map[string]interface{}{
			"circuit_breaker": name,
		})
	}
}

func (eh *PythonPatternsErrorHandler) EmergencyFallback(ctx context.Context, operation string) error {
	if !eh.config.EnableEmergencyMode {
		return fmt.Errorf("emergency mode not enabled for operation: %s", operation)
	}

	eh.logger.Warn("Activating emergency fallback mode", map[string]interface{}{
		"operation": operation,
	})

	eh.resetAllCircuitBreakers()

	return nil
}

func (eh *PythonPatternsErrorHandler) resetAllCircuitBreakers() {
	eh.mu.Lock()
	defer eh.mu.Unlock()

	for name := range eh.circuitBreakers {
		eh.ResetCircuitBreaker(name)
	}
}

func IsRecoverableError(err error) bool {
	if err == nil {
		return false
	}

	errorStr := err.Error()

	recoverablePatterns := []string{
		"connection refused",
		"timeout",
		"temporary failure",
		"network",
		"dns",
		"i/o timeout",
		"context deadline exceeded",
		"connection reset",
		"broken pipe",
	}

	for _, pattern := range recoverablePatterns {
		if contains(errorStr, pattern) {
			return true
		}
	}

	return false
}

func contains(str, substr string) bool {
	return len(str) >= len(substr) && (str == substr || len(substr) == 0 || 
		(len(str) > len(substr) && (str[:len(substr)] == substr || 
		str[len(str)-len(substr):] == substr || 
		containsSubstring(str, substr))))
}

func containsSubstring(str, substr string) bool {
	for i := 0; i <= len(str)-len(substr); i++ {
		if str[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}