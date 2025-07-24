package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// HealthStatus represents the health status of a server
type HealthStatus struct {
	IsHealthy           bool          `json:"is_healthy"`
	ResponseTime        time.Duration `json:"response_time"`
	ErrorRate           float64       `json:"error_rate"`
	MemoryUsage         int64         `json:"memory_usage"`
	CPUUsage            float64       `json:"cpu_usage"`
	ConnectionCount     int32         `json:"connection_count"`
	LastHealthCheck     time.Time     `json:"last_health_check"`
	ConsecutiveFailures int           `json:"consecutive_failures"`
	RecoveryAttempts    int           `json:"recovery_attempts"`
	LastError           string        `json:"last_error"`
	Uptime              time.Duration `json:"uptime"`
}

// NewHealthStatus creates a new health status
func NewHealthStatus() *HealthStatus {
	return &HealthStatus{
		IsHealthy:           true,
		ResponseTime:        0,
		ErrorRate:           0.0,
		MemoryUsage:         0,
		CPUUsage:            0.0,
		ConnectionCount:     0,
		LastHealthCheck:     time.Now(),
		ConsecutiveFailures: 0,
		RecoveryAttempts:    0,
		LastError:           "",
		Uptime:              0,
	}
}

// Copy creates a copy of the health status
func (hs *HealthStatus) Copy() *HealthStatus {
	return &HealthStatus{
		IsHealthy:           hs.IsHealthy,
		ResponseTime:        hs.ResponseTime,
		ErrorRate:           hs.ErrorRate,
		MemoryUsage:         hs.MemoryUsage,
		CPUUsage:            hs.CPUUsage,
		ConnectionCount:     hs.ConnectionCount,
		LastHealthCheck:     hs.LastHealthCheck,
		ConsecutiveFailures: hs.ConsecutiveFailures,
		RecoveryAttempts:    hs.RecoveryAttempts,
		LastError:           hs.LastError,
		Uptime:              hs.Uptime,
	}
}

// HealthThresholds defines thresholds for health checking
type HealthThresholds struct {
	MaxResponseTime        time.Duration `json:"max_response_time"`
	MaxErrorRate           float64       `json:"max_error_rate"`
	MaxMemoryUsageMB       int64         `json:"max_memory_usage_mb"`
	MaxCPUUsage            float64       `json:"max_cpu_usage"`
	MaxConsecutiveFailures int           `json:"max_consecutive_failures"`
	MaxConnectionCount     int32         `json:"max_connection_count"`
}

// DefaultHealthThresholds returns default health thresholds
func DefaultHealthThresholds() *HealthThresholds {
	return &HealthThresholds{
		MaxResponseTime:        5 * time.Second,
		MaxErrorRate:           0.1, // 10%
		MaxMemoryUsageMB:       512,
		MaxCPUUsage:            80.0, // 80%
		MaxConsecutiveFailures: 3,
		MaxConnectionCount:     100,
	}
}

// RecoveryStrategy defines how to recover a failed server
type RecoveryStrategy int

const (
	RecoveryRestart RecoveryStrategy = iota
	RecoveryReconnect
	RecoveryReplace
	RecoveryIgnore
)

func (rs RecoveryStrategy) String() string {
	switch rs {
	case RecoveryRestart:
		return "restart"
	case RecoveryReconnect:
		return "reconnect"
	case RecoveryReplace:
		return "replace"
	case RecoveryIgnore:
		return "ignore"
	default:
		return "unknown"
	}
}

// ServerHealthChecker manages health checking for a single server
type ServerHealthChecker struct {
	server          *ServerInstance
	thresholds      *HealthThresholds
	recoveryStrategy RecoveryStrategy
	checkInterval   time.Duration
	ctx             context.Context
	cancel          context.CancelFunc
	mu              sync.RWMutex
}

// NewServerHealthChecker creates a new server health checker
func NewServerHealthChecker(server *ServerInstance, thresholds *HealthThresholds, recoveryStrategy RecoveryStrategy, checkInterval time.Duration) *ServerHealthChecker {
	ctx, cancel := context.WithCancel(context.Background())
	return &ServerHealthChecker{
		server:           server,
		thresholds:       thresholds,
		recoveryStrategy: recoveryStrategy,
		checkInterval:    checkInterval,
		ctx:              ctx,
		cancel:           cancel,
	}
}

// Start starts the health checker
func (shc *ServerHealthChecker) Start() {
	go shc.healthCheckLoop()
}

// Stop stops the health checker
func (shc *ServerHealthChecker) Stop() {
	shc.cancel()
}

// healthCheckLoop runs the health check loop
func (shc *ServerHealthChecker) healthCheckLoop() {
	ticker := time.NewTicker(shc.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-shc.ctx.Done():
			return
		case <-ticker.C:
			shc.performHealthCheck()
		}
	}
}

// performHealthCheck performs a single health check
func (shc *ServerHealthChecker) performHealthCheck() {
	shc.mu.Lock()
	defer shc.mu.Unlock()

	if shc.server.GetState() == ServerStateStopped || shc.server.GetState() == ServerStateStopping {
		return
	}

	start := time.Now()
	err := shc.checkServerHealth()
	duration := time.Since(start)

	shc.server.healthStatus.LastHealthCheck = time.Now()
	shc.server.healthStatus.ResponseTime = duration

	if err != nil {
		shc.server.healthStatus.IsHealthy = false
		shc.server.healthStatus.LastError = err.Error()
		shc.server.healthStatus.ConsecutiveFailures++

		// Check if we should trigger recovery
		if shc.shouldTriggerRecovery() {
			go shc.triggerRecovery()
		}
	} else {
		shc.server.healthStatus.IsHealthy = true
		shc.server.healthStatus.LastError = ""
		shc.server.healthStatus.ConsecutiveFailures = 0
		
		// Update server state if it was unhealthy
		if shc.server.GetState() == ServerStateUnhealthy {
			shc.server.SetState(ServerStateHealthy)
		}
	}
}

// checkServerHealth performs the actual health check
func (shc *ServerHealthChecker) checkServerHealth() error {
	// Check if client is active
	if !shc.server.client.IsActive() {
		return fmt.Errorf("LSP client is not active")
	}

	// Perform a simple LSP request to check server responsiveness
	ctx, cancel := context.WithTimeout(context.Background(), shc.thresholds.MaxResponseTime)
	defer cancel()

	// Send a simple initialize request or ping
	params := map[string]interface{}{
		"processId": nil,
		"clientInfo": map[string]interface{}{
			"name":    "lsp-gateway-health-check",
			"version": "1.0.0",
		},
		"capabilities": map[string]interface{}{},
	}

	start := time.Now()
	_, err := shc.server.client.SendRequest(ctx, "initialize", params)
	responseTime := time.Since(start)

	if err != nil {
		return fmt.Errorf("health check request failed: %w", err)
	}

	// Check response time threshold
	if responseTime > shc.thresholds.MaxResponseTime {
		return fmt.Errorf("response time %v exceeds threshold %v", responseTime, shc.thresholds.MaxResponseTime)
	}

	// Check error rate from metrics
	metrics := shc.server.GetMetrics()
	if metrics.GetErrorRate() > shc.thresholds.MaxErrorRate {
		return fmt.Errorf("error rate %.2f%% exceeds threshold %.2f%%", metrics.GetErrorRate()*100, shc.thresholds.MaxErrorRate*100)
	}

	// Check memory usage if available
	if shc.server.memoryUsage > shc.thresholds.MaxMemoryUsageMB*1024*1024 {
		return fmt.Errorf("memory usage %d MB exceeds threshold %d MB", shc.server.memoryUsage/1024/1024, shc.thresholds.MaxMemoryUsageMB)
	}

	// Update uptime
	shc.server.healthStatus.Uptime = time.Since(shc.server.startTime)

	return nil
}

// shouldTriggerRecovery determines if recovery should be triggered
func (shc *ServerHealthChecker) shouldTriggerRecovery() bool {
	return shc.server.healthStatus.ConsecutiveFailures >= shc.thresholds.MaxConsecutiveFailures
}

// triggerRecovery triggers the recovery process
func (shc *ServerHealthChecker) triggerRecovery() {
	shc.server.healthStatus.RecoveryAttempts++

	switch shc.recoveryStrategy {
	case RecoveryRestart:
		shc.restartServer()
	case RecoveryReconnect:
		shc.reconnectServer()
	case RecoveryReplace:
		shc.replaceServer()
	case RecoveryIgnore:
		// Do nothing, just log
	}
}

// restartServer restarts the server
func (shc *ServerHealthChecker) restartServer() {
	// Implementation would restart the server
	// This would typically involve stopping and starting the LSP client
	if err := shc.server.Stop(); err != nil {
		return
	}

	time.Sleep(1 * time.Second) // Brief pause

	if err := shc.server.Start(context.Background()); err != nil {
		shc.server.SetState(ServerStateFailed)
	}
}

// reconnectServer attempts to reconnect the server
func (shc *ServerHealthChecker) reconnectServer() {
	// Implementation would attempt to reconnect the LSP client
	// without fully restarting the server process
}

// replaceServer attempts to replace the server with a new instance
func (shc *ServerHealthChecker) replaceServer() {
	// Implementation would create a new server instance
	// and replace the current one in the pool
}

// HealthMonitor manages health checking for all servers
type HealthMonitor struct {
	checkInterval      time.Duration
	healthCheckers     map[string]*ServerHealthChecker
	alertThresholds    *HealthThresholds
	recoveryStrategies map[string]RecoveryStrategy
	ctx                context.Context
	cancel             context.CancelFunc
	mu                 sync.RWMutex
}

// NewHealthMonitor creates a new health monitor
func NewHealthMonitor(checkInterval time.Duration) *HealthMonitor {
	ctx, cancel := context.WithCancel(context.Background())
	return &HealthMonitor{
		checkInterval:      checkInterval,
		healthCheckers:     make(map[string]*ServerHealthChecker),
		alertThresholds:    DefaultHealthThresholds(),
		recoveryStrategies: make(map[string]RecoveryStrategy),
		ctx:                ctx,
		cancel:             cancel,
	}
}

// StartMonitoring starts health monitoring for all registered servers
func (hm *HealthMonitor) StartMonitoring(ctx context.Context) error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	// Update context
	hm.ctx = ctx

	// Start all health checkers
	for _, checker := range hm.healthCheckers {
		checker.ctx = ctx
		checker.Start()
	}

	return nil
}

// StopMonitoring stops health monitoring
func (hm *HealthMonitor) StopMonitoring() {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	hm.cancel()

	// Stop all health checkers
	for _, checker := range hm.healthCheckers {
		checker.Stop()
	}
}

// RegisterServer registers a server for health monitoring
func (hm *HealthMonitor) RegisterServer(server *ServerInstance, recoveryStrategy RecoveryStrategy) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	checker := NewServerHealthChecker(server, hm.alertThresholds, recoveryStrategy, hm.checkInterval)
	hm.healthCheckers[server.config.Name] = checker
	hm.recoveryStrategies[server.config.Name] = recoveryStrategy

	// Start the checker if monitoring is already running
	if hm.ctx != nil {
		checker.ctx = hm.ctx
		checker.Start()
	}
}

// UnregisterServer unregisters a server from health monitoring
func (hm *HealthMonitor) UnregisterServer(serverName string) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	if checker, exists := hm.healthCheckers[serverName]; exists {
		checker.Stop()
		delete(hm.healthCheckers, serverName)
		delete(hm.recoveryStrategies, serverName)
	}
}

// CheckServerHealth performs an immediate health check on a specific server
func (hm *HealthMonitor) CheckServerHealth(server *ServerInstance) *HealthStatus {
	hm.mu.RLock()
	checker, exists := hm.healthCheckers[server.config.Name]
	hm.mu.RUnlock()

	if !exists {
		// Create a temporary checker for immediate health check
		checker = NewServerHealthChecker(server, hm.alertThresholds, RecoveryIgnore, hm.checkInterval)
	}

	checker.performHealthCheck()
	return server.healthStatus.Copy()
}

// PerformHealthCheck performs a health check on a specific server by name
func (hm *HealthMonitor) PerformHealthCheck(serverName string) error {
	hm.mu.RLock()
	checker, exists := hm.healthCheckers[serverName]
	hm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("server %s not registered for health monitoring", serverName)
	}

	checker.performHealthCheck()
	return nil
}

// TriggerRecovery manually triggers recovery for a specific server
func (hm *HealthMonitor) TriggerRecovery(serverName string) error {
	hm.mu.RLock()
	checker, exists := hm.healthCheckers[serverName]
	hm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("server %s not registered for health monitoring", serverName)
	}

	go checker.triggerRecovery()
	return nil
}

// UpdateServerHealth manually updates the health status of a server
func (hm *HealthMonitor) UpdateServerHealth(serverName string, status *HealthStatus) error {
	hm.mu.RLock()
	checker, exists := hm.healthCheckers[serverName]
	hm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("server %s not registered for health monitoring", serverName)
	}

	checker.server.healthStatus = status
	return nil
}

// GetHealthStatus returns the health status of all monitored servers
func (hm *HealthMonitor) GetHealthStatus() map[string]*HealthStatus {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	status := make(map[string]*HealthStatus)
	for serverName, checker := range hm.healthCheckers {
		status[serverName] = checker.server.healthStatus.Copy()
	}
	return status
}

// GetUnhealthyServers returns a list of unhealthy servers
func (hm *HealthMonitor) GetUnhealthyServers() []string {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	var unhealthy []string
	for serverName, checker := range hm.healthCheckers {
		if !checker.server.healthStatus.IsHealthy {
			unhealthy = append(unhealthy, serverName)
		}
	}
	return unhealthy
}

// SetThresholds updates the health thresholds for all servers
func (hm *HealthMonitor) SetThresholds(thresholds *HealthThresholds) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	hm.alertThresholds = thresholds
	
	// Update all existing checkers
	for _, checker := range hm.healthCheckers {
		checker.thresholds = thresholds
	}
}

// SetRecoveryStrategy sets the recovery strategy for a specific server
func (hm *HealthMonitor) SetRecoveryStrategy(serverName string, strategy RecoveryStrategy) error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	checker, exists := hm.healthCheckers[serverName]
	if !exists {
		return fmt.Errorf("server %s not registered for health monitoring", serverName)
	}

	checker.recoveryStrategy = strategy
	hm.recoveryStrategies[serverName] = strategy
	return nil
}

// GetHealthSummary returns a summary of health status across all servers
func (hm *HealthMonitor) GetHealthSummary() *HealthSummary {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	summary := &HealthSummary{
		TotalServers:    len(hm.healthCheckers),
		HealthyServers:  0,
		UnhealthyServers: 0,
		FailedServers:   0,
		LastCheckTime:   time.Now(),
	}

	for _, checker := range hm.healthCheckers {
		switch {
		case checker.server.healthStatus.IsHealthy:
			summary.HealthyServers++
		case checker.server.GetState() == ServerStateFailed:
			summary.FailedServers++
		default:
			summary.UnhealthyServers++
		}
	}

	return summary
}

// HealthSummary provides a summary of health monitoring status
type HealthSummary struct {
	TotalServers     int       `json:"total_servers"`
	HealthyServers   int       `json:"healthy_servers"`
	UnhealthyServers int       `json:"unhealthy_servers"`
	FailedServers    int       `json:"failed_servers"`
	LastCheckTime    time.Time `json:"last_check_time"`
}

// ToJSON converts health summary to JSON
func (hs *HealthSummary) ToJSON() ([]byte, error) {
	return json.Marshal(hs)
}