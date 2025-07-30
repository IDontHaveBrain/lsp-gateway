package pooling

import (
	"context"
	"fmt"
	"sync"
	"time"

	"lsp-gateway/internal/transport"
)

// ServerPoolManager manages pools of LSP servers for different languages
type ServerPoolManager struct {
	// Configuration
	config *PoolConfig
	
	// Language pools
	pools map[string]*LanguagePool
	
	// Health monitoring
	healthMonitor   *PoolHealthMonitor
	workspaceSwitcher *WorkspaceSwitcher
	metrics         *PoolMetrics
	
	// State management
	mu        sync.RWMutex
	isRunning bool
	ctx       context.Context
	cancel    context.CancelFunc
	
	// Client factories for each language
	clientFactories map[string]func() (transport.LSPClient, error)
	
	// Background tasks
	metricsCollector *time.Ticker
	healthChecker    *time.Ticker
}

// NewServerPoolManager creates a new server pool manager
func NewServerPoolManager(config *PoolConfig) *ServerPoolManager {
	if config == nil {
		config = DefaultPoolConfig()
	}
	
	// Validate configuration
	if err := config.ValidateConfig(); err != nil {
		// Use defaults for invalid config
		config = DefaultPoolConfig()
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	spm := &ServerPoolManager{
		config:          config,
		pools:           make(map[string]*LanguagePool),
		metrics:         &PoolMetrics{},
		mu:              sync.RWMutex{},
		ctx:             ctx,
		cancel:          cancel,
		clientFactories: make(map[string]func() (transport.LSPClient, error)),
	}
	
	// Initialize health monitor and workspace switcher stubs
	spm.healthMonitor = NewPoolHealthMonitor(spm)
	spm.workspaceSwitcher = NewWorkspaceSwitcher(spm)
	
	return spm
}

// RegisterClientFactory registers a factory function for creating LSP clients for a specific language
func (spm *ServerPoolManager) RegisterClientFactory(language string, factory func() (transport.LSPClient, error)) error {
	spm.mu.Lock()
	defer spm.mu.Unlock()
	
	if factory == nil {
		return fmt.Errorf("client factory cannot be nil for language %s", language)
	}
	
	spm.clientFactories[language] = factory
	return nil
}

// Start initializes and starts the server pool manager
func (spm *ServerPoolManager) Start(ctx context.Context) error {
	spm.mu.Lock()
	defer spm.mu.Unlock()
	
	if spm.isRunning {
		return fmt.Errorf("server pool manager is already running")
	}
	
	if !spm.config.Enabled {
		return fmt.Errorf("server pooling is disabled in configuration")
	}
	
	// Create language pools
	for language, langConfig := range spm.config.Languages {
		if err := spm.createLanguagePoolUnsafe(language, langConfig); err != nil {
			return fmt.Errorf("failed to create pool for language %s: %w", language, err)
		}
	}
	
	// Start all language pools
	for language, pool := range spm.pools {
		if err := pool.Start(ctx); err != nil {
			return fmt.Errorf("failed to start pool for language %s: %w", language, err)
		}
	}
	
	// Start background tasks
	spm.startBackgroundTasksUnsafe()
	
	spm.isRunning = true
	return nil
}

// Stop gracefully stops the server pool manager
func (spm *ServerPoolManager) Stop() error {
	spm.mu.Lock()
	defer spm.mu.Unlock()
	
	if !spm.isRunning {
		return nil
	}
	
	spm.isRunning = false
	
	// Cancel background tasks
	if spm.cancel != nil {
		spm.cancel()
	}
	
	// Stop tickers
	if spm.metricsCollector != nil {
		spm.metricsCollector.Stop()
	}
	if spm.healthChecker != nil {
		spm.healthChecker.Stop()
	}
	
	// Stop all language pools
	var errors []error
	for language, pool := range spm.pools {
		if err := pool.Stop(); err != nil {
			errors = append(errors, fmt.Errorf("failed to stop pool for %s: %w", language, err))
		}
	}
	
	// Clear pools
	spm.pools = make(map[string]*LanguagePool)
	
	if len(errors) > 0 {
		return fmt.Errorf("errors stopping pools: %v", errors)
	}
	
	return nil
}

// GetServer allocates a server from the appropriate language pool
func (spm *ServerPoolManager) GetServer(language string) (*PooledServer, error) {
	return spm.GetServerWithContext(context.Background(), language, "")
}

// GetServerWithContext allocates a server with context and workspace
func (spm *ServerPoolManager) GetServerWithContext(ctx context.Context, language string, workspace string) (*PooledServer, error) {
	if !spm.isRunning {
		// Fallback to creating a fresh client
		return spm.createFallbackServer(ctx, language, workspace)
	}
	
	spm.mu.RLock()
	pool, exists := spm.pools[language]
	spm.mu.RUnlock()
	
	if !exists {
		// Language not configured for pooling, fallback
		return spm.createFallbackServer(ctx, language, workspace)
	}
	
	// Try to get from pool with timeout
	allocCtx, cancel := context.WithTimeout(ctx, spm.config.AllocationTimeout)
	defer cancel()
	
	server, err := pool.GetServer(allocCtx, workspace)
	if err != nil {
		// Pool allocation failed, try fallback
		if spm.config.EnableFallback {
			spm.updateMetricsFallback()
			return spm.createFallbackServer(ctx, language, workspace)
		}
		return nil, fmt.Errorf("failed to allocate server from pool for %s: %w", language, err)
	}
	
	spm.updateMetricsAllocation(true)
	return server, nil
}

// ReturnServer returns a server to the appropriate pool
func (spm *ServerPoolManager) ReturnServer(server *PooledServer) error {
	if server == nil {
		return fmt.Errorf("cannot return nil server")
	}
	
	if !spm.isRunning {
		// If pool is not running, just stop the server
		return server.Stop()
	}
	
	spm.mu.RLock()
	pool, exists := spm.pools[server.language]
	spm.mu.RUnlock()
	
	if !exists {
		// No pool for this language, stop the server
		return server.Stop()
	}
	
	return pool.ReturnServer(server)
}

// Health returns aggregate health information for all pools
func (spm *ServerPoolManager) Health() *PoolHealth {
	spm.mu.RLock()
	defer spm.mu.RUnlock()
	
	health := &PoolHealth{
		TotalServers:     0,
		HealthyServers:   0,
		DegradedServers:  0,
		UnhealthyServers: 0,
		ActiveServers:    0,
		IdleServers:      0,
		AverageUseCount:  0,
		LastHealthCheck:  time.Now(),
		Languages:        make(map[string]*LanguageHealth),
	}
	
	var totalUseCount float64
	var totalLanguages int
	
	for language, pool := range spm.pools {
		langHealth := pool.GetHealth()
		health.Languages[language] = langHealth
		
		health.TotalServers += langHealth.TotalServers
		health.HealthyServers += langHealth.HealthyServers
		health.ActiveServers += langHealth.ActiveServers
		health.IdleServers += langHealth.IdleServers
		health.UnhealthyServers += langHealth.FailedServers
		
		totalUseCount += langHealth.AverageUseCount
		totalLanguages++
	}
	
	if totalLanguages > 0 {
		health.AverageUseCount = totalUseCount / float64(totalLanguages)
	}
	
	return health
}

// GetMetrics returns detailed metrics about pool performance
func (spm *ServerPoolManager) GetMetrics() *PoolMetrics {
	spm.mu.RLock()
	defer spm.mu.RUnlock()
	
	// Return a copy of current metrics
	metricsCopy := *spm.metrics
	return &metricsCopy
}

// GetLanguagePoolInfo returns information about servers in a specific language pool
func (spm *ServerPoolManager) GetLanguagePoolInfo(language string) ([]*PooledServerInfo, error) {
	spm.mu.RLock()
	pool, exists := spm.pools[language]
	spm.mu.RUnlock()
	
	if !exists {
		return nil, fmt.Errorf("no pool found for language %s", language)
	}
	
	return pool.GetServerInfo(), nil
}

// SetConfig updates the pool configuration (for dynamic reconfiguration)
func (spm *ServerPoolManager) SetConfig(config *PoolConfig) error {
	if config == nil {
		return fmt.Errorf("configuration cannot be nil")
	}
	
	if err := config.ValidateConfig(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}
	
	spm.mu.Lock()
	defer spm.mu.Unlock()
	
	spm.config = config
	
	// TODO: In a full implementation, this would update running pools
	// For now, changes require restart
	
	return nil
}

// IsRunning returns whether the pool manager is currently running
func (spm *ServerPoolManager) IsRunning() bool {
	spm.mu.RLock()
	defer spm.mu.RUnlock()
	return spm.isRunning
}

// createLanguagePoolUnsafe creates a language pool (must be called with lock held)
func (spm *ServerPoolManager) createLanguagePoolUnsafe(language string, config *LanguagePoolConfig) error {
	factory, exists := spm.clientFactories[language]
	if !exists {
		return fmt.Errorf("no client factory registered for language %s", language)
	}
	
	pool := NewLanguagePool(language, config, factory)
	spm.pools[language] = pool
	
	return nil
}

// createFallbackServer creates a server when pooling is not available
func (spm *ServerPoolManager) createFallbackServer(ctx context.Context, language string, workspace string) (*PooledServer, error) {
	factory, exists := spm.clientFactories[language]
	if !exists {
		return nil, fmt.Errorf("no client factory registered for language %s", language)
	}
	
	// Get language config or use defaults
	var langConfig *LanguagePoolConfig
	if spm.config != nil && spm.config.Languages != nil {
		if lc, exists := spm.config.Languages[language]; exists {
			langConfig = lc
		}
	}
	
	if langConfig == nil {
		// Use default config
		langConfig = &LanguagePoolConfig{
			WarmupTimeout:      30 * time.Second,
			MaxUseCount:        1, // Single use for fallback
			FailureThreshold:   1,
			RestartThreshold:   1,
			TransportType:      "stdio",
			ConcurrentRequests: 1,
		}
	}
	
	// Create client
	client, err := factory()
	if err != nil {
		return nil, fmt.Errorf("failed to create fallback client for %s: %w", language, err)
	}
	
	// Create pooled server wrapper
	server := NewPooledServer(language, client, langConfig)
	
	// Start the server
	startCtx, cancel := context.WithTimeout(ctx, langConfig.WarmupTimeout)
	defer cancel()
	
	if err := server.Start(startCtx); err != nil {
		return nil, fmt.Errorf("failed to start fallback server for %s: %w", language, err)
	}
	
	// Acquire for the workspace
	if err := server.Acquire(workspace); err != nil {
		server.Stop()
		return nil, fmt.Errorf("failed to acquire fallback server for %s: %w", language, err)
	}
	
	return server, nil
}

// startBackgroundTasksUnsafe starts background monitoring tasks
func (spm *ServerPoolManager) startBackgroundTasksUnsafe() {
	if spm.config.EnableMetrics && spm.config.MetricsInterval > 0 {
		spm.metricsCollector = time.NewTicker(spm.config.MetricsInterval)
		go spm.metricsCollectionLoop()
	}
	
	if spm.config.HealthCheckInterval > 0 {
		spm.healthChecker = time.NewTicker(spm.config.HealthCheckInterval)
		go spm.healthCheckLoop()
	}
}

// metricsCollectionLoop collects metrics periodically
func (spm *ServerPoolManager) metricsCollectionLoop() {
	for {
		select {
		case <-spm.metricsCollector.C:
			spm.collectMetrics()
		case <-spm.ctx.Done():
			return
		}
	}
}

// healthCheckLoop performs periodic health checks
func (spm *ServerPoolManager) healthCheckLoop() {
	for {
		select {
		case <-spm.healthChecker.C:
			spm.performHealthCheck()
		case <-spm.ctx.Done():
			return
		}
	}
}

// collectMetrics gathers metrics from all pools
func (spm *ServerPoolManager) collectMetrics() {
	spm.mu.RLock()
	defer spm.mu.RUnlock()
	
	// This is a stub implementation
	// In a full implementation, this would gather detailed metrics
	// from all language pools and update spm.metrics
}

// performHealthCheck checks the health of all pools
func (spm *ServerPoolManager) performHealthCheck() {
	// This is a stub implementation
	// In a full implementation, this would trigger health checks
	// across all language pools and take corrective actions
}

// updateMetricsAllocation updates allocation metrics
func (spm *ServerPoolManager) updateMetricsAllocation(success bool) {
	spm.mu.Lock()
	defer spm.mu.Unlock()
	
	spm.metrics.TotalAllocations++
	if success {
		spm.metrics.SuccessfulAllocs++
	} else {
		spm.metrics.FailedAllocs++
	}
}

// updateMetricsFallback updates fallback metrics
func (spm *ServerPoolManager) updateMetricsFallback() {
	spm.mu.Lock()
	defer spm.mu.Unlock()
	
	// Calculate fallback rate
	if spm.metrics.TotalAllocations > 0 {
		fallbacks := spm.metrics.TotalAllocations - spm.metrics.SuccessfulAllocs
		spm.metrics.FallbackRate = float64(fallbacks) / float64(spm.metrics.TotalAllocations)
	}
}

// Placeholder for health monitor - this would be implemented in a separate task
type PoolHealthMonitor struct {
	manager *ServerPoolManager
}

func NewPoolHealthMonitor(manager *ServerPoolManager) *PoolHealthMonitor {
	return &PoolHealthMonitor{manager: manager}
}

// Placeholder for workspace switcher - this would be implemented in a separate task
type WorkspaceSwitcher struct {
	manager *ServerPoolManager
}

func NewWorkspaceSwitcher(manager *ServerPoolManager) *WorkspaceSwitcher {
	return &WorkspaceSwitcher{manager: manager}
}