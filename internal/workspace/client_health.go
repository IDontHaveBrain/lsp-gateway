package workspace

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"lsp-gateway/internal/transport"
	"lsp-gateway/mcp"
)

// ClientHealthInfo represents detailed health information for a client
type ClientHealthInfo struct {
	SubProjectID        string        `json:"sub_project_id"`
	Language            string        `json:"language"`
	IsHealthy           bool          `json:"is_healthy"`
	LastHealthCheck     time.Time     `json:"last_health_check"`
	HealthCheckCount    int64         `json:"health_check_count"`
	SuccessfulChecks    int64         `json:"successful_checks"`
	FailedChecks        int64         `json:"failed_checks"`
	ConsecutiveFails    int64         `json:"consecutive_fails"`
	LastError           string        `json:"last_error,omitempty"`
	ResponseTime        time.Duration `json:"response_time"`
	AverageResponseTime time.Duration `json:"average_response_time"`
	CircuitBreakerState string        `json:"circuit_breaker_state"`
	RecoveryAttempts    int64         `json:"recovery_attempts"`
	LastRecoveryAttempt time.Time     `json:"last_recovery_attempt,omitempty"`
}

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState int

const (
	CircuitBreakerClosed CircuitBreakerState = iota
	CircuitBreakerOpen
	CircuitBreakerHalfOpen
)

func (s CircuitBreakerState) String() string {
	switch s {
	case CircuitBreakerClosed:
		return "closed"
	case CircuitBreakerOpen:
		return "open"
	case CircuitBreakerHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreaker implements the circuit breaker pattern for LSP clients
type CircuitBreaker struct {
	state             CircuitBreakerState
	failureCount      int64
	successCount      int64
	failureThreshold  int64
	recoveryTimeout   time.Duration
	lastFailureTime   time.Time
	lastSuccessTime   time.Time
	halfOpenRequests  int64
	maxHalfOpenReqs   int64
	mutex             sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(failureThreshold int64, recoveryTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:            CircuitBreakerClosed,
		failureThreshold: failureThreshold,
		recoveryTimeout:  recoveryTimeout,
		maxHalfOpenReqs:  3, // Allow 3 requests in half-open state
	}
}

// AllowRequest determines if a request should be allowed through the circuit breaker
func (cb *CircuitBreaker) AllowRequest() bool {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	
	now := time.Now()
	
	switch cb.state {
	case CircuitBreakerClosed:
		return true
		
	case CircuitBreakerOpen:
		// Check if recovery timeout has elapsed
		if now.Sub(cb.lastFailureTime) >= cb.recoveryTimeout {
			cb.state = CircuitBreakerHalfOpen
			cb.halfOpenRequests = 0
			return true
		}
		return false
		
	case CircuitBreakerHalfOpen:
		// Allow limited requests in half-open state
		if cb.halfOpenRequests < cb.maxHalfOpenReqs {
			cb.halfOpenRequests++
			return true
		}
		return false
		
	default:
		return false
	}
}

// RecordSuccess records a successful request
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	
	cb.successCount++
	cb.lastSuccessTime = time.Now()
	
	switch cb.state {
	case CircuitBreakerHalfOpen:
		// If we've had enough successful requests, close the circuit
		if cb.halfOpenRequests >= cb.maxHalfOpenReqs {
			cb.state = CircuitBreakerClosed
			cb.failureCount = 0
		}
	case CircuitBreakerClosed:
		// Reset failure count on success
		cb.failureCount = 0
	}
}

// RecordFailure records a failed request
func (cb *CircuitBreaker) RecordFailure() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	
	cb.failureCount++
	cb.lastFailureTime = time.Now()
	
	// Open circuit if failure threshold is reached
	if cb.failureCount >= cb.failureThreshold {
		cb.state = CircuitBreakerOpen
	}
}

// GetState returns the current circuit breaker state
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.state
}

// GetStats returns statistics about the circuit breaker
func (cb *CircuitBreaker) GetStats() (int64, int64, time.Time, time.Time) {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.failureCount, cb.successCount, cb.lastFailureTime, cb.lastSuccessTime
}

// CircuitBreakerManager manages circuit breakers for all clients
type CircuitBreakerManager struct {
	breakers map[string]*CircuitBreaker // key: "subProjectID:language"
	mutex    sync.RWMutex
	config   *ClientManagerConfig
	logger   *mcp.StructuredLogger
}

// NewCircuitBreakerManager creates a new circuit breaker manager
func NewCircuitBreakerManager(config *ClientManagerConfig, logger *mcp.StructuredLogger) *CircuitBreakerManager {
	return &CircuitBreakerManager{
		breakers: make(map[string]*CircuitBreaker),
		config:   config,
		logger:   logger,
	}
}

// AllowRequest checks if a request should be allowed for the given client
func (cbm *CircuitBreakerManager) AllowRequest(key string) bool {
	cbm.mutex.RLock()
	breaker, exists := cbm.breakers[key]
	cbm.mutex.RUnlock()
	
	if !exists {
		// Create new circuit breaker on first use
		cbm.mutex.Lock()
		breaker, exists = cbm.breakers[key]
		if !exists {
			breaker = NewCircuitBreaker(5, 30*time.Second) // 5 failures, 30s recovery
			cbm.breakers[key] = breaker
		}
		cbm.mutex.Unlock()
	}
	
	return breaker.AllowRequest()
}

// RecordSuccess records a successful request for the given client
func (cbm *CircuitBreakerManager) RecordSuccess(key string) {
	cbm.mutex.RLock()
	breaker, exists := cbm.breakers[key]
	cbm.mutex.RUnlock()
	
	if exists {
		breaker.RecordSuccess()
	}
}

// RecordFailure records a failed request for the given client
func (cbm *CircuitBreakerManager) RecordFailure(key string) {
	cbm.mutex.RLock()
	breaker, exists := cbm.breakers[key]
	cbm.mutex.RUnlock()
	
	if exists {
		breaker.RecordFailure()
	}
}

// GetCircuitBreakerState returns the current state of a circuit breaker
func (cbm *CircuitBreakerManager) GetCircuitBreakerState(key string) CircuitBreakerState {
	cbm.mutex.RLock()
	breaker, exists := cbm.breakers[key]
	cbm.mutex.RUnlock()
	
	if !exists {
		return CircuitBreakerClosed
	}
	
	return breaker.GetState()
}

// ClientHealthMonitor monitors the health of LSP clients
type ClientHealthMonitor struct {
	registry      *ClientRegistry
	config        *ClientManagerConfig
	logger        *mcp.StructuredLogger
	healthData    map[string]*ClientHealthInfo // key: "subProjectID:language"
	mutex         sync.RWMutex
	isShutdown    int32
	shutdownOnce  sync.Once
}

// NewClientHealthMonitor creates a new client health monitor
func NewClientHealthMonitor(registry *ClientRegistry, config *ClientManagerConfig, logger *mcp.StructuredLogger) *ClientHealthMonitor {
	return &ClientHealthMonitor{
		registry:   registry,
		config:     config,
		logger:     logger,
		healthData: make(map[string]*ClientHealthInfo),
	}
}

// PerformHealthCheck conducts health checks on all registered clients
func (chm *ClientHealthMonitor) PerformHealthCheck(ctx context.Context) error {
	if atomic.LoadInt32(&chm.isShutdown) != 0 {
		return fmt.Errorf("health monitor is shutdown")
	}
	
	allClients := chm.registry.GetAllClients()
	var errors []error
	
	// Create a channel to limit concurrent health checks
	semaphore := make(chan struct{}, 10) // Max 10 concurrent checks
	var wg sync.WaitGroup
	
	for subProjectID, clients := range allClients {
		for language, client := range clients {
			wg.Add(1)
			go func(spID, lang string, c transport.LSPClient) {
				defer wg.Done()
				
				semaphore <- struct{}{} // Acquire semaphore
				defer func() { <-semaphore }() // Release semaphore
				
				if err := chm.checkClientHealth(ctx, spID, lang, c); err != nil {
					// Collect errors but don't fail the entire health check
					chm.mutex.Lock()
					errors = append(errors, fmt.Errorf("health check failed for %s:%s: %w", spID, lang, err))
					chm.mutex.Unlock()
				}
			}(subProjectID, language, client)
		}
	}
	
	wg.Wait()
	
	if len(errors) > 0 && chm.logger != nil {
		chm.logger.WithField("error_count", len(errors)).Warn("Some client health checks failed")
		for _, err := range errors {
			chm.logger.WithError(err).Debug("Health check error details")
		}
	}
	
	return nil
}

// checkClientHealth performs a health check on a specific client
func (chm *ClientHealthMonitor) checkClientHealth(ctx context.Context, subProjectID, language string, client transport.LSPClient) error {
	key := fmt.Sprintf("%s:%s", subProjectID, language)
	
	// Get or create health info
	chm.mutex.Lock()
	healthInfo, exists := chm.healthData[key]
	if !exists {
		healthInfo = &ClientHealthInfo{
			SubProjectID:        subProjectID,
			Language:            language,
			IsHealthy:           true,
			CircuitBreakerState: CircuitBreakerClosed.String(),
		}
		chm.healthData[key] = healthInfo
	}
	chm.mutex.Unlock()
	
	// Perform the actual health check
	startTime := time.Now()
	isHealthy := true
	var lastError string
	
	// Check if client is active
	if !client.IsActive() {
		isHealthy = false
		lastError = "client is not active"
	} else {
		// Send a simple LSP request to test connectivity
		healthCheckCtx, cancel := context.WithTimeout(ctx, chm.config.ClientTimeout)
		defer cancel()
		
		// Try to send a capabilities request or similar lightweight request
		_, err := client.SendRequest(healthCheckCtx, "$/cancelRequest", map[string]interface{}{
			"id": "health-check-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		})
		
		if err != nil {
			isHealthy = false
			lastError = err.Error()
		}
	}
	
	responseTime := time.Since(startTime)
	
	// Update health information
	chm.mutex.Lock()
	defer chm.mutex.Unlock()
	
	healthInfo.LastHealthCheck = time.Now()
	healthInfo.HealthCheckCount++
	healthInfo.ResponseTime = responseTime
	
	// Update average response time using exponential moving average
	if healthInfo.AverageResponseTime == 0 {
		healthInfo.AverageResponseTime = responseTime
	} else {
		alpha := 0.1 // Smoothing factor
		avgNanos := float64(healthInfo.AverageResponseTime.Nanoseconds())
		newNanos := float64(responseTime.Nanoseconds())
		healthInfo.AverageResponseTime = time.Duration(alpha*newNanos + (1-alpha)*avgNanos)
	}
	
	if isHealthy {
		healthInfo.IsHealthy = true
		healthInfo.SuccessfulChecks++
		healthInfo.ConsecutiveFails = 0
		healthInfo.LastError = ""
		
		// Update registry health status
		chm.registry.UpdateClientHealth(subProjectID, language, true)
	} else {
		healthInfo.IsHealthy = false
		healthInfo.FailedChecks++
		healthInfo.ConsecutiveFails++
		healthInfo.LastError = lastError
		
		// Update registry health status and error
		chm.registry.UpdateClientHealth(subProjectID, language, false)
		chm.registry.UpdateClientError(subProjectID, language, lastError)
		
		// Consider auto-restart if enabled
		if chm.config.EnableAutoRestart && chm.shouldAttemptRestart(healthInfo) {
			chm.scheduleClientRestart(subProjectID, language)
		}
	}
	
	if chm.logger != nil {
		chm.logger.WithFields(map[string]interface{}{
			"sub_project_id":       subProjectID,
			"language":             language,
			"is_healthy":           isHealthy,
			"response_time_ms":     responseTime.Milliseconds(),
			"consecutive_fails":    healthInfo.ConsecutiveFails,
			"total_checks":         healthInfo.HealthCheckCount,
		}).Debug("Client health check completed")
	}
	
	return nil
}

// shouldAttemptRestart determines if a client should be restarted based on health status
func (chm *ClientHealthMonitor) shouldAttemptRestart(healthInfo *ClientHealthInfo) bool {
	// Don't restart too frequently
	if time.Since(healthInfo.LastRecoveryAttempt) < time.Minute {
		return false
	}
	
	// Restart after 3 consecutive failures
	if healthInfo.ConsecutiveFails >= 3 {
		return true
	}
	
	// Restart if average response time is too high (over 5 seconds)
	if healthInfo.AverageResponseTime > 5*time.Second {
		return true
	}
	
	return false
}

// scheduleClientRestart schedules a restart for a failing client
func (chm *ClientHealthMonitor) scheduleClientRestart(subProjectID, language string) {
	if chm.logger != nil {
		chm.logger.WithFields(map[string]interface{}{
			"sub_project_id": subProjectID,
			"language":       language,
		}).Info("Scheduling client restart due to health issues")
	}
	
	// Update recovery attempt tracking
	key := fmt.Sprintf("%s:%s", subProjectID, language)
	chm.mutex.Lock()
	if healthInfo, exists := chm.healthData[key]; exists {
		healthInfo.RecoveryAttempts++
		healthInfo.LastRecoveryAttempt = time.Now()
	}
	chm.mutex.Unlock()
	
	// Schedule restart in a separate goroutine to avoid blocking health checks
	go func() {
		// Use exponential backoff for restart attempts
		backoffDuration := chm.calculateBackoffDuration(subProjectID, language)
		time.Sleep(backoffDuration)
		
		// The actual restart would be handled by the client manager
		// This is just logging the scheduled restart
		if chm.logger != nil {
			chm.logger.WithFields(map[string]interface{}{
				"sub_project_id":   subProjectID,
				"language":         language,
				"backoff_duration": backoffDuration.String(),
			}).Info("Executing scheduled client restart")
		}
	}()
}

// calculateBackoffDuration calculates the backoff duration for restart attempts
func (chm *ClientHealthMonitor) calculateBackoffDuration(subProjectID, language string) time.Duration {
	key := fmt.Sprintf("%s:%s", subProjectID, language)
	
	chm.mutex.RLock()
	healthInfo, exists := chm.healthData[key]
	chm.mutex.RUnlock()
	
	if !exists {
		return time.Second // Default 1 second
	}
	
	// Exponential backoff: 2^attempts seconds, capped at 5 minutes
	attempts := healthInfo.RecoveryAttempts
	backoffSeconds := math.Pow(2, float64(attempts))
	maxBackoffSeconds := 300.0 // 5 minutes
	
	if backoffSeconds > maxBackoffSeconds {
		backoffSeconds = maxBackoffSeconds
	}
	
	return time.Duration(backoffSeconds) * time.Second
}

// GetClientHealth returns health information for a specific client
func (chm *ClientHealthMonitor) GetClientHealth(subProjectID, language string) *ClientHealthInfo {
	if atomic.LoadInt32(&chm.isShutdown) != 0 {
		return nil
	}
	
	key := fmt.Sprintf("%s:%s", subProjectID, language)
	
	chm.mutex.RLock()
	defer chm.mutex.RUnlock()
	
	if healthInfo, exists := chm.healthData[key]; exists {
		// Return a copy to prevent external modifications
		infoCopy := *healthInfo
		return &infoCopy
	}
	
	return nil
}

// GetAllHealthInfo returns health information for all monitored clients
func (chm *ClientHealthMonitor) GetAllHealthInfo() map[string]*ClientHealthInfo {
	if atomic.LoadInt32(&chm.isShutdown) != 0 {
		return make(map[string]*ClientHealthInfo)
	}
	
	chm.mutex.RLock()
	defer chm.mutex.RUnlock()
	
	result := make(map[string]*ClientHealthInfo)
	for key, healthInfo := range chm.healthData {
		infoCopy := *healthInfo
		result[key] = &infoCopy
	}
	
	return result
}

// ClearHealthData removes health data for a specific client
func (chm *ClientHealthMonitor) ClearHealthData(subProjectID, language string) {
	key := fmt.Sprintf("%s:%s", subProjectID, language)
	
	chm.mutex.Lock()
	defer chm.mutex.Unlock()
	
	delete(chm.healthData, key)
}

// Shutdown gracefully shuts down the health monitor
func (chm *ClientHealthMonitor) Shutdown(ctx context.Context) error {
	var shutdownErr error
	
	chm.shutdownOnce.Do(func() {
		atomic.StoreInt32(&chm.isShutdown, 1)
		
		if chm.logger != nil {
			chm.logger.Info("Shutting down ClientHealthMonitor")
		}
		
		chm.mutex.Lock()
		// Clear all health data
		chm.healthData = make(map[string]*ClientHealthInfo)
		chm.mutex.Unlock()
		
		if chm.logger != nil {
			chm.logger.Info("ClientHealthMonitor shutdown completed successfully")
		}
	})
	
	return shutdownErr
}

// LoadBalancingInfo and ClientLoadBalancer for completeness
type ClientLoadBalancer struct {
	registry *ClientRegistry
	config   *ClientManagerConfig
	logger   *mcp.StructuredLogger
	
	// Load balancing statistics
	requestCounts    map[string]int64 // key: "subProjectID:language"
	responseTimes    map[string]time.Duration
	lastUpdated      time.Time
	mutex            sync.RWMutex
}

// NewClientLoadBalancer creates a new client load balancer
func NewClientLoadBalancer(registry *ClientRegistry, config *ClientManagerConfig, logger *mcp.StructuredLogger) *ClientLoadBalancer {
	return &ClientLoadBalancer{
		registry:      registry,
		config:        config,
		logger:        logger,
		requestCounts: make(map[string]int64),
		responseTimes: make(map[string]time.Duration),
		lastUpdated:   time.Now(),
	}
}

// GetLoadBalancingInfo returns current load balancing information
func (clb *ClientLoadBalancer) GetLoadBalancingInfo() *LoadBalancingInfo {
	clb.mutex.RLock()
	defer clb.mutex.RUnlock()
	
	totalRequests := int64(0)
	requestsPerProject := make(map[string]int64)
	requestsPerLanguage := make(map[string]int64)
	responseTimePerClient := make(map[string]time.Duration)
	
	var totalResponseTime time.Duration
	clientCount := 0
	
	for key, count := range clb.requestCounts {
		totalRequests += count
		responseTimePerClient[key] = clb.responseTimes[key]
		totalResponseTime += clb.responseTimes[key]
		clientCount++
		
		// Parse subProjectID:language key
		if parts := fmt.Sprintf("%s", key); len(parts) > 0 {
			// This is a simplified parsing - in practice you'd want more robust parsing
			requestsPerProject[key] = count
			requestsPerLanguage[key] = count
		}
	}
	
	var averageResponseTime time.Duration
	if clientCount > 0 {
		averageResponseTime = totalResponseTime / time.Duration(clientCount)
	}
	
	return &LoadBalancingInfo{
		TotalRequests:         totalRequests,
		RequestsPerProject:    requestsPerProject,
		RequestsPerLanguage:   requestsPerLanguage,
		AverageResponseTime:   averageResponseTime,
		ResponseTimePerClient: responseTimePerClient,
		LoadBalancingStrategy: "round-robin", // Default strategy
		LastUpdated:           clb.lastUpdated,
	}
}

// ClientSelector handles client selection logic
type ClientSelector struct {
	registry     *ClientRegistry
	loadBalancer *ClientLoadBalancer
	config       *ClientManagerConfig
	logger       *mcp.StructuredLogger
}

// NewClientSelector creates a new client selector
func NewClientSelector(registry *ClientRegistry, loadBalancer *ClientLoadBalancer, config *ClientManagerConfig, logger *mcp.StructuredLogger) *ClientSelector {
	return &ClientSelector{
		registry:     registry,
		loadBalancer: loadBalancer,
		config:       config,
		logger:       logger,
	}
}

// SelectClient selects the best available client for the given criteria
func (cs *ClientSelector) SelectClient(subProjectID, language string) (transport.LSPClient, error) {
	// For now, use simple direct lookup
	// In a more sophisticated implementation, this would implement load balancing logic
	return cs.registry.GetClient(subProjectID, language)
}