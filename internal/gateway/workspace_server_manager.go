package gateway

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/transport"
)

// WorkspaceServerManager manages server pools for a specific workspace
// It provides workspace-specific server management with resource limits and multi-language support
type WorkspaceServerManager struct {
	workspaceID              string
	config                   *config.GatewayConfig
	globalMultiServerManager *MultiServerManager

	// Workspace-specific server pools
	languageServerPools map[string]*WorkspaceLanguagePool // language -> pool
	serverInstances     map[string][]*ServerInstance      // language -> instances

	// Workspace resource limits
	maxServersPerLanguage int
	totalMemoryLimitMB    int64
	currentMemoryUsage    int64

	// Request tracking
	activeRequests int32
	requestHistory []RequestRecord
	maxHistorySize int

	// Lifecycle management
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.RWMutex
	logger *log.Logger

	// Metrics and monitoring
	lastResourceCheck time.Time
	createdAt         time.Time
	lastUsed          time.Time
}

// WorkspaceLanguagePool manages servers for a specific language within a workspace
type WorkspaceLanguagePool struct {
	workspaceID string
	language    string
	servers     []*ServerInstance
	maxServers  int
	minServers  int

	// Simple load balancing
	roundRobinIndex int32

	// Basic metrics
	requestCount int64
	successCount int64
	errorCount   int64

	// Server health tracking
	healthyServers  []*ServerInstance
	lastHealthCheck time.Time

	mu sync.RWMutex
}

// WorkspaceResourceUsage tracks resource usage for a workspace
type WorkspaceResourceUsage struct {
	WorkspaceID    string         `json:"workspace_id"`
	MemoryUsageMB  int64          `json:"memory_usage_mb"`
	ServerCount    map[string]int `json:"server_count"` // language -> count
	ActiveRequests int32          `json:"active_requests"`
	LastUpdated    time.Time      `json:"last_updated"`
	TotalServers   int            `json:"total_servers"`
	HealthyServers int            `json:"healthy_servers"`
}

// RequestRecord tracks individual requests for analysis
type RequestRecord struct {
	RequestID  string        `json:"request_id"`
	Language   string        `json:"language"`
	Method     string        `json:"method"`
	StartTime  time.Time     `json:"start_time"`
	EndTime    time.Time     `json:"end_time"`
	Duration   time.Duration `json:"duration"`
	Success    bool          `json:"success"`
	ServerName string        `json:"server_name"`
	Error      string        `json:"error,omitempty"`
}

// Default configuration constants
const (
	DefaultMaxServersPerLanguage = 3
	DefaultTotalMemoryLimitMB    = 1024
	DefaultMaxHistorySize        = 100
	DefaultMinServersPerLanguage = 1
	HealthCheckInterval          = 30 * time.Second
)

// NewWorkspaceServerManager creates a new workspace server manager
func NewWorkspaceServerManager(workspaceID string, config *config.GatewayConfig, globalManager *MultiServerManager) *WorkspaceServerManager {
	ctx, cancel := context.WithCancel(context.Background())

	// Set default resource limits if not configured
	maxServersPerLanguage := DefaultMaxServersPerLanguage
	totalMemoryLimitMB := int64(DefaultTotalMemoryLimitMB)

	// Use global configuration if available
	if config != nil && config.ResourceLimits != nil {
		if config.ResourceLimits.MaxServersPerLanguage > 0 {
			maxServersPerLanguage = config.ResourceLimits.MaxServersPerLanguage
		}
		if config.ResourceLimits.TotalMemoryLimitMB > 0 {
			totalMemoryLimitMB = config.ResourceLimits.TotalMemoryLimitMB
		}
	}

	logger := log.Default()
	if globalManager != nil && globalManager.logger != nil {
		logger = globalManager.logger
	}

	return &WorkspaceServerManager{
		workspaceID:              workspaceID,
		config:                   config,
		globalMultiServerManager: globalManager,
		languageServerPools:      make(map[string]*WorkspaceLanguagePool),
		serverInstances:          make(map[string][]*ServerInstance),
		maxServersPerLanguage:    maxServersPerLanguage,
		totalMemoryLimitMB:       totalMemoryLimitMB,
		maxHistorySize:           DefaultMaxHistorySize,
		ctx:                      ctx,
		cancel:                   cancel,
		logger:                   logger,
		createdAt:                time.Now(),
		lastUsed:                 time.Now(),
	}
}

// Initialize initializes the workspace server manager
func (wsm *WorkspaceServerManager) Initialize() error {
	wsm.mu.Lock()
	defer wsm.mu.Unlock()

	wsm.logger.Printf("Initializing workspace server manager for workspace %s", wsm.workspaceID)

	// Start background health checking
	go wsm.healthCheckRoutine()

	// Start resource monitoring
	go wsm.resourceMonitorRoutine()

	wsm.logger.Printf("Workspace server manager initialized for workspace %s", wsm.workspaceID)
	return nil
}

// Shutdown shuts down the workspace server manager and all its servers
func (wsm *WorkspaceServerManager) Shutdown() error {
	wsm.mu.Lock()
	defer wsm.mu.Unlock()

	wsm.logger.Printf("Shutting down workspace server manager for workspace %s", wsm.workspaceID)

	// Cancel context to stop background routines
	wsm.cancel()

	// Stop all servers in all language pools
	var errors []error
	for language, pool := range wsm.languageServerPools {
		if err := wsm.shutdownLanguagePool(language, pool); err != nil {
			errors = append(errors, fmt.Errorf("failed to shutdown pool for language %s: %w", language, err))
		}
	}

	// Clear all data structures
	wsm.languageServerPools = make(map[string]*WorkspaceLanguagePool)
	wsm.serverInstances = make(map[string][]*ServerInstance)
	wsm.requestHistory = nil

	if len(errors) > 0 {
		return fmt.Errorf("errors during shutdown: %v", errors)
	}

	wsm.logger.Printf("Workspace server manager shut down successfully for workspace %s", wsm.workspaceID)
	return nil
}

// GetOrCreateLanguagePool gets or creates a language pool for the workspace
func (wsm *WorkspaceServerManager) GetOrCreateLanguagePool(language string) (*WorkspaceLanguagePool, error) {
	wsm.mu.Lock()
	defer wsm.mu.Unlock()

	// Check if pool already exists
	if pool, exists := wsm.languageServerPools[language]; exists {
		return pool, nil
	}

	// Create new language pool
	pool := &WorkspaceLanguagePool{
		workspaceID:     wsm.workspaceID,
		language:        language,
		servers:         make([]*ServerInstance, 0),
		maxServers:      wsm.maxServersPerLanguage,
		minServers:      DefaultMinServersPerLanguage,
		healthyServers:  make([]*ServerInstance, 0),
		lastHealthCheck: time.Now(),
	}

	wsm.languageServerPools[language] = pool
	wsm.serverInstances[language] = make([]*ServerInstance, 0)

	wsm.logger.Printf("Created language pool for %s in workspace %s", language, wsm.workspaceID)
	return pool, nil
}

// GetServerForRequest gets the best server for a specific request
func (wsm *WorkspaceServerManager) GetServerForRequest(language string, requestType string) (*ServerInstance, error) {
	// Ensure we have a language pool
	pool, err := wsm.GetOrCreateLanguagePool(language)
	if err != nil {
		return nil, fmt.Errorf("failed to get language pool: %w", err)
	}

	// Update last used time
	atomic.StoreInt64((*int64)(&wsm.lastUsed), time.Now().UnixNano())

	// Get server from pool
	server, err := wsm.selectServerFromPool(pool, requestType)
	if err != nil {
		// Try to start a new server if we're under limits
		if wsm.canStartNewServer(language) {
			newServer, startErr := wsm.StartServerForLanguage(language)
			if startErr != nil {
				return nil, fmt.Errorf("failed to get existing server and failed to start new server: %w, %w", err, startErr)
			}
			return newServer, nil
		}
		return nil, err
	}

	// Record request tracking
	atomic.AddInt32(&wsm.activeRequests, 1)

	return server, nil
}

// GetServersForRequest gets multiple servers for concurrent processing
func (wsm *WorkspaceServerManager) GetServersForRequest(language string, maxServers int) ([]*ServerInstance, error) {
	pool, err := wsm.GetOrCreateLanguagePool(language)
	if err != nil {
		return nil, fmt.Errorf("failed to get language pool: %w", err)
	}

	pool.mu.RLock()
	defer pool.mu.RUnlock()

	// Update last used time
	atomic.StoreInt64((*int64)(&wsm.lastUsed), time.Now().UnixNano())

	// Get available healthy servers
	availableServers := make([]*ServerInstance, 0)
	for _, server := range pool.healthyServers {
		if server.IsHealthy() && server.IsActive() {
			availableServers = append(availableServers, server)
		}
	}

	if len(availableServers) == 0 {
		return nil, fmt.Errorf("no healthy servers available for language %s in workspace %s", language, wsm.workspaceID)
	}

	// Limit to requested number or available servers
	requestedCount := maxServers
	if requestedCount > len(availableServers) {
		requestedCount = len(availableServers)
	}
	if requestedCount <= 0 {
		requestedCount = 1
	}

	// Return the requested number of servers
	selectedServers := availableServers[:requestedCount]

	// Update active request count
	atomic.AddInt32(&wsm.activeRequests, int32(len(selectedServers)))

	return selectedServers, nil
}

// StartServerForLanguage starts a new server for a specific language
func (wsm *WorkspaceServerManager) StartServerForLanguage(language string) (*ServerInstance, error) {
	wsm.mu.Lock()
	defer wsm.mu.Unlock()

	// Check resource limits
	if !wsm.checkResourceLimitsUnsafe(language) {
		return nil, fmt.Errorf("resource limits exceeded for language %s in workspace %s", language, wsm.workspaceID)
	}

	// Get server configuration from global config
	serverConfig, err := wsm.config.GetServerByLanguage(language)
	if err != nil {
		return nil, fmt.Errorf("no server configuration found for language %s: %w", language, err)
	}

	// Create LSP client
	clientConfig := transport.ClientConfig{
		Command:   serverConfig.Command,
		Args:      serverConfig.Args,
		Transport: serverConfig.Transport,
	}

	client, err := transport.NewLSPClient(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create LSP client for language %s: %w", language, err)
	}

	// Create server instance
	serverName := fmt.Sprintf("%s_%s_%d", wsm.workspaceID, language, time.Now().UnixNano())
	serverConfig.Name = serverName

	server := NewServerInstance(serverConfig, client)

	// Start the server
	ctx, cancel := context.WithTimeout(wsm.ctx, 30*time.Second)
	defer cancel()

	if err := server.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start server for language %s: %w", language, err)
	}

	// Add to workspace pools
	pool, err := wsm.GetOrCreateLanguagePool(language)
	if err != nil {
		server.Stop()
		return nil, fmt.Errorf("failed to get language pool: %w", err)
	}

	// Add server to pool
	pool.mu.Lock()
	pool.servers = append(pool.servers, server)
	if server.IsHealthy() {
		pool.healthyServers = append(pool.healthyServers, server)
	}
	pool.mu.Unlock()

	// Add to server instances
	wsm.serverInstances[language] = append(wsm.serverInstances[language], server)

	wsm.logger.Printf("Started new server %s for language %s in workspace %s", serverName, language, wsm.workspaceID)
	return server, nil
}

// StopServerForLanguage stops a specific server for a language
func (wsm *WorkspaceServerManager) StopServerForLanguage(language string, serverName string) error {
	wsm.mu.Lock()
	defer wsm.mu.Unlock()

	pool, exists := wsm.languageServerPools[language]
	if !exists {
		return fmt.Errorf("no language pool found for %s in workspace %s", language, wsm.workspaceID)
	}

	pool.mu.Lock()
	defer pool.mu.Unlock()

	// Find and remove server from pool
	serverIndex := -1
	var targetServer *ServerInstance

	for i, server := range pool.servers {
		if server.config.Name == serverName {
			serverIndex = i
			targetServer = server
			break
		}
	}

	if serverIndex == -1 {
		return fmt.Errorf("server %s not found in language %s pool for workspace %s", serverName, language, wsm.workspaceID)
	}

	// Stop the server
	if err := targetServer.Stop(); err != nil {
		wsm.logger.Printf("Error stopping server %s: %v", serverName, err)
	}

	// Remove from pool.servers
	pool.servers = append(pool.servers[:serverIndex], pool.servers[serverIndex+1:]...)

	// Remove from healthy servers if present
	for i, server := range pool.healthyServers {
		if server.config.Name == serverName {
			pool.healthyServers = append(pool.healthyServers[:i], pool.healthyServers[i+1:]...)
			break
		}
	}

	// Remove from workspace server instances
	if instances, exists := wsm.serverInstances[language]; exists {
		for i, server := range instances {
			if server.config.Name == serverName {
				wsm.serverInstances[language] = append(instances[:i], instances[i+1:]...)
				break
			}
		}
	}

	wsm.logger.Printf("Stopped server %s for language %s in workspace %s", serverName, language, wsm.workspaceID)
	return nil
}

// RestartServerForLanguage restarts a specific server for a language
func (wsm *WorkspaceServerManager) RestartServerForLanguage(language string, serverName string) error {
	// Stop the server first
	if err := wsm.StopServerForLanguage(language, serverName); err != nil {
		return fmt.Errorf("failed to stop server %s: %w", serverName, err)
	}

	// Start a new server
	_, err := wsm.StartServerForLanguage(language)
	if err != nil {
		return fmt.Errorf("failed to start replacement server for language %s: %w", language, err)
	}

	wsm.logger.Printf("Restarted server for language %s in workspace %s", language, wsm.workspaceID)
	return nil
}

// CheckResourceLimits checks if resource limits allow for new servers
func (wsm *WorkspaceServerManager) CheckResourceLimits(language string) bool {
	wsm.mu.RLock()
	defer wsm.mu.RUnlock()
	return wsm.checkResourceLimitsUnsafe(language)
}

// checkResourceLimitsUnsafe checks resource limits without locking (must be called with lock held)
func (wsm *WorkspaceServerManager) checkResourceLimitsUnsafe(language string) bool {
	// Check per-language server limit
	if instances, exists := wsm.serverInstances[language]; exists {
		if len(instances) >= wsm.maxServersPerLanguage {
			return false
		}
	}

	// Check total memory limit (simplified check)
	totalServers := 0
	for _, instances := range wsm.serverInstances {
		totalServers += len(instances)
	}

	// Estimate memory usage (simplified)
	estimatedMemoryMB := int64(totalServers * 128) // Assume 128MB per server
	if estimatedMemoryMB >= wsm.totalMemoryLimitMB {
		return false
	}

	return true
}

// canStartNewServer checks if we can start a new server for a language
func (wsm *WorkspaceServerManager) canStartNewServer(language string) bool {
	return wsm.CheckResourceLimits(language)
}

// GetResourceUsage returns current resource usage for the workspace
func (wsm *WorkspaceServerManager) GetResourceUsage() *WorkspaceResourceUsage {
	wsm.mu.RLock()
	defer wsm.mu.RUnlock()

	serverCount := make(map[string]int)
	totalServers := 0
	healthyServers := 0

	for language, instances := range wsm.serverInstances {
		count := len(instances)
		serverCount[language] = count
		totalServers += count

		// Count healthy servers
		for _, server := range instances {
			if server.IsHealthy() {
				healthyServers++
			}
		}
	}

	// Estimate current memory usage
	estimatedMemoryMB := int64(totalServers * 128) // Simplified estimation

	return &WorkspaceResourceUsage{
		WorkspaceID:    wsm.workspaceID,
		MemoryUsageMB:  estimatedMemoryMB,
		ServerCount:    serverCount,
		ActiveRequests: atomic.LoadInt32(&wsm.activeRequests),
		LastUpdated:    time.Now(),
		TotalServers:   totalServers,
		HealthyServers: healthyServers,
	}
}

// selectServerFromPool selects the best server from a language pool
func (wsm *WorkspaceServerManager) selectServerFromPool(pool *WorkspaceLanguagePool, requestType string) (*ServerInstance, error) {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	if len(pool.healthyServers) == 0 {
		return nil, fmt.Errorf("no healthy servers available for language %s in workspace %s", pool.language, wsm.workspaceID)
	}

	// Simple round-robin selection
	index := atomic.AddInt32(&pool.roundRobinIndex, 1) % int32(len(pool.healthyServers))
	selectedServer := pool.healthyServers[index]

	// Update pool metrics
	atomic.AddInt64(&pool.requestCount, 1)

	return selectedServer, nil
}

// shutdownLanguagePool shuts down all servers in a language pool
func (wsm *WorkspaceServerManager) shutdownLanguagePool(language string, pool *WorkspaceLanguagePool) error {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	var errors []error
	for _, server := range pool.servers {
		if err := server.Stop(); err != nil {
			errors = append(errors, fmt.Errorf("failed to stop server %s: %w", server.config.Name, err))
		}
	}

	// Clear pool data
	pool.servers = nil
	pool.healthyServers = nil

	if len(errors) > 0 {
		return fmt.Errorf("errors shutting down language pool %s: %v", language, errors)
	}

	return nil
}

// healthCheckRoutine performs periodic health checks on all servers
func (wsm *WorkspaceServerManager) healthCheckRoutine() {
	ticker := time.NewTicker(HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-wsm.ctx.Done():
			return
		case <-ticker.C:
			wsm.performHealthCheck()
		}
	}
}

// performHealthCheck checks the health of all servers and updates pools
func (wsm *WorkspaceServerManager) performHealthCheck() {
	wsm.mu.RLock()
	defer wsm.mu.RUnlock()

	for language, pool := range wsm.languageServerPools {
		pool.mu.Lock()

		// Rebuild healthy servers list
		healthyServers := make([]*ServerInstance, 0)
		for _, server := range pool.servers {
			if server.IsHealthy() && server.IsActive() {
				healthyServers = append(healthyServers, server)
			}
		}

		pool.healthyServers = healthyServers
		pool.lastHealthCheck = time.Now()

		wsm.logger.Printf("Health check for workspace %s language %s: %d/%d servers healthy",
			wsm.workspaceID, language, len(healthyServers), len(pool.servers))

		pool.mu.Unlock()
	}
}

// resourceMonitorRoutine monitors resource usage
func (wsm *WorkspaceServerManager) resourceMonitorRoutine() {
	ticker := time.NewTicker(60 * time.Second) // Check every minute
	defer ticker.Stop()

	for {
		select {
		case <-wsm.ctx.Done():
			return
		case <-ticker.C:
			wsm.checkResourceUsage()
		}
	}
}

// checkResourceUsage monitors and logs resource usage
func (wsm *WorkspaceServerManager) checkResourceUsage() {
	usage := wsm.GetResourceUsage()
	wsm.lastResourceCheck = time.Now()

	// Log if usage is high
	memoryPercent := float64(usage.MemoryUsageMB) / float64(wsm.totalMemoryLimitMB) * 100
	if memoryPercent > 80 {
		wsm.logger.Printf("High memory usage in workspace %s: %.1f%% (%dMB/%dMB)",
			wsm.workspaceID, memoryPercent, usage.MemoryUsageMB, wsm.totalMemoryLimitMB)
	}
}

// RecordRequest records a request for tracking and analysis
func (wsm *WorkspaceServerManager) RecordRequest(record RequestRecord) {
	wsm.mu.Lock()
	defer wsm.mu.Unlock()

	// Add to history
	wsm.requestHistory = append(wsm.requestHistory, record)

	// Trim history if too long
	if len(wsm.requestHistory) > wsm.maxHistorySize {
		wsm.requestHistory = wsm.requestHistory[len(wsm.requestHistory)-wsm.maxHistorySize:]
	}

	// Update metrics
	if pool, exists := wsm.languageServerPools[record.Language]; exists {
		pool.mu.Lock()
		if record.Success {
			atomic.AddInt64(&pool.successCount, 1)
		} else {
			atomic.AddInt64(&pool.errorCount, 1)
		}
		pool.mu.Unlock()
	}

	// Decrement active requests
	atomic.AddInt32(&wsm.activeRequests, -1)
}

// UpdateServerMetrics method to ServerInstance (if not already present)
func (si *ServerInstance) UpdateMetrics(responseTime time.Duration, success bool) {
	si.mu.Lock()
	defer si.mu.Unlock()

	if si.metrics != nil {
		si.metrics.RecordRequest(responseTime, success)
	}

	if success {
		si.lastUsed = time.Now()
	}
}

// GetRequestHistory returns recent request history
func (wsm *WorkspaceServerManager) GetRequestHistory() []RequestRecord {
	wsm.mu.RLock()
	defer wsm.mu.RUnlock()

	// Return a copy to avoid race conditions
	history := make([]RequestRecord, len(wsm.requestHistory))
	copy(history, wsm.requestHistory)
	return history
}

// WorkspaceRequestRouter handles routing within a workspace
type WorkspaceRequestRouter struct {
	workspaceServerManager *WorkspaceServerManager
	routeToMultipleServers map[string]bool // method -> should use multiple servers
	maxServersPerMethod    map[string]int  // method -> max servers

	mu sync.RWMutex
}

// NewWorkspaceRequestRouter creates a new workspace request router
func NewWorkspaceRequestRouter(workspaceServerManager *WorkspaceServerManager) *WorkspaceRequestRouter {
	// Configure methods that benefit from multiple servers
	routeToMultipleServers := map[string]bool{
		"workspace/symbol":            true,
		"textDocument/references":     true,
		"textDocument/documentSymbol": false,
		"textDocument/definition":     false,
		"textDocument/hover":          false,
		"textDocument/completion":     false,
	}

	// Configure max servers per method
	maxServersPerMethod := map[string]int{
		"workspace/symbol":            3,
		"textDocument/references":     2,
		"textDocument/documentSymbol": 1,
		"textDocument/definition":     1,
		"textDocument/hover":          1,
		"textDocument/completion":     1,
	}

	return &WorkspaceRequestRouter{
		workspaceServerManager: workspaceServerManager,
		routeToMultipleServers: routeToMultipleServers,
		maxServersPerMethod:    maxServersPerMethod,
	}
}

// RouteRequest routes a request to appropriate servers within the workspace
func (wrr *WorkspaceRequestRouter) RouteRequest(language string, method string, preferMultiple bool) ([]*ServerInstance, error) {
	wrr.mu.RLock()
	shouldUseMultiple := wrr.routeToMultipleServers[method]
	maxServers := wrr.maxServersPerMethod[method]
	wrr.mu.RUnlock()

	// Default to 1 server if not configured
	if maxServers == 0 {
		maxServers = 1
	}

	// Use multiple servers if configured and requested
	if (preferMultiple || shouldUseMultiple) && maxServers > 1 {
		servers, err := wrr.workspaceServerManager.GetServersForRequest(language, maxServers)
		if err != nil {
			// Fallback to single server
			server, fallbackErr := wrr.workspaceServerManager.GetServerForRequest(language, method)
			if fallbackErr != nil {
				return nil, fmt.Errorf("failed to get multiple servers and fallback failed: %w, %w", err, fallbackErr)
			}
			return []*ServerInstance{server}, nil
		}
		return servers, nil
	}

	// Use single server
	server, err := wrr.workspaceServerManager.GetServerForRequest(language, method)
	if err != nil {
		return nil, err
	}
	return []*ServerInstance{server}, nil
}

// SetMultipleServerConfig configures whether a method should use multiple servers
func (wrr *WorkspaceRequestRouter) SetMultipleServerConfig(method string, useMultiple bool, maxServers int) {
	wrr.mu.Lock()
	defer wrr.mu.Unlock()

	wrr.routeToMultipleServers[method] = useMultiple
	if maxServers > 0 {
		wrr.maxServersPerMethod[method] = maxServers
	}
}

// GetLanguagePoolMetrics returns metrics for a specific language pool
func (wsm *WorkspaceServerManager) GetLanguagePoolMetrics(language string) map[string]interface{} {
	wsm.mu.RLock()
	defer wsm.mu.RUnlock()

	pool, exists := wsm.languageServerPools[language]
	if !exists {
		return nil
	}

	pool.mu.RLock()
	defer pool.mu.RUnlock()

	return map[string]interface{}{
		"language":          language,
		"workspace_id":      wsm.workspaceID,
		"total_servers":     len(pool.servers),
		"healthy_servers":   len(pool.healthyServers),
		"max_servers":       pool.maxServers,
		"min_servers":       pool.minServers,
		"request_count":     atomic.LoadInt64(&pool.requestCount),
		"success_count":     atomic.LoadInt64(&pool.successCount),
		"error_count":       atomic.LoadInt64(&pool.errorCount),
		"last_health_check": pool.lastHealthCheck,
	}
}
