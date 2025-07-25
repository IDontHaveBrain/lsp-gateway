package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/transport"
)

// ServerState represents the current state of a server instance
type ServerState int32

const (
	ServerStateStarting ServerState = iota
	ServerStateHealthy
	ServerStateUnhealthy
	ServerStateFailed
	ServerStateStopping
	ServerStateStopped
)

func (s ServerState) String() string {
	switch s {
	case ServerStateStarting:
		return "starting"
	case ServerStateHealthy:
		return "healthy"
	case ServerStateUnhealthy:
		return "unhealthy"
	case ServerStateFailed:
		return "failed"
	case ServerStateStopping:
		return "stopping"
	case ServerStateStopped:
		return "stopped"
	default:
		return "unknown"
	}
}

// ServerInstance represents a single LSP server instance
type ServerInstance struct {
	config          *config.ServerConfig
	client          transport.LSPClient
	healthStatus    *HealthStatus
	metrics         *ServerMetrics
	circuitBreaker  *CircuitBreaker
	startTime       time.Time
	lastUsed        time.Time
	processID       int
	memoryUsage     int64
	connectionCount int32
	state           ServerState
	mu              sync.RWMutex
}

// NewServerInstance creates a new server instance
func NewServerInstance(serverConfig *config.ServerConfig, client transport.LSPClient) *ServerInstance {
	return &ServerInstance{
		config:         serverConfig,
		client:         client,
		healthStatus:   NewHealthStatus(),
		metrics:        NewServerMetrics(),
		circuitBreaker: NewCircuitBreaker(10, 30*time.Second, 5),
		startTime:      time.Now(),
		lastUsed:       time.Now(),
		state:          ServerStateStarting,
	}
}

// Start starts the server instance
func (si *ServerInstance) Start(ctx context.Context) error {
	si.mu.Lock()
	defer si.mu.Unlock()

	if si.state != ServerStateStarting {
		return fmt.Errorf("server %s is in state %s, cannot start", si.config.Name, si.state)
	}

	if err := si.client.Start(ctx); err != nil {
		si.state = ServerStateFailed
		return fmt.Errorf("failed to start LSP client for server %s: %w", si.config.Name, err)
	}

	si.state = ServerStateHealthy
	si.startTime = time.Now()
	return nil
}

// Stop stops the server instance
func (si *ServerInstance) Stop() error {
	si.mu.Lock()
	defer si.mu.Unlock()

	if si.state == ServerStateStopped || si.state == ServerStateStopping {
		return nil
	}

	si.state = ServerStateStopping
	if err := si.client.Stop(); err != nil {
		si.state = ServerStateFailed
		return fmt.Errorf("failed to stop LSP client for server %s: %w", si.config.Name, err)
	}

	si.state = ServerStateStopped
	return nil
}

// IsHealthy returns true if the server is healthy
func (si *ServerInstance) IsHealthy() bool {
	si.mu.RLock()
	defer si.mu.RUnlock()
	return si.state == ServerStateHealthy && si.healthStatus.IsHealthy
}

// IsActive returns true if the server is active
func (si *ServerInstance) IsActive() bool {
	si.mu.RLock()
	defer si.mu.RUnlock()
	return si.client.IsActive() && (si.state == ServerStateHealthy || si.state == ServerStateUnhealthy)
}

// SendRequest sends a request to the server instance
func (si *ServerInstance) SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	if !si.circuitBreaker.CanExecute() {
		return nil, fmt.Errorf("circuit breaker is open for server %s", si.config.Name)
	}

	start := time.Now()
	result, err := si.client.SendRequest(ctx, method, params)
	responseTime := time.Since(start)

	atomic.StoreInt64((*int64)(&si.lastUsed), time.Now().UnixNano())

	if err != nil {
		si.circuitBreaker.RecordFailure()
		si.metrics.RecordRequest(responseTime, false)
		return nil, err
	}

	si.circuitBreaker.RecordSuccess()
	si.metrics.RecordRequest(responseTime, true)
	return result, nil
}

// GetMetrics returns the server metrics
func (si *ServerInstance) GetMetrics() *ServerMetrics {
	si.mu.RLock()
	defer si.mu.RUnlock()
	return si.metrics.Copy()
}

// GetState returns the current server state
func (si *ServerInstance) GetState() ServerState {
	si.mu.RLock()
	defer si.mu.RUnlock()
	return si.state
}

// SetState sets the server state
func (si *ServerInstance) SetState(state ServerState) {
	si.mu.Lock()
	defer si.mu.Unlock()
	si.state = state
}

// LanguageServerPool manages multiple servers for a single language
type LanguageServerPool struct {
	language          string
	servers           map[string]*ServerInstance
	activeServers     []*ServerInstance
	loadBalancer      *LoadBalancer
	healthMonitor     *HealthMonitor
	resourceLimits    *config.ResourceLimits
	selectionStrategy string
	metrics           *PoolMetrics
	mu                sync.RWMutex
}

// NewLanguageServerPool creates a new language server pool
func NewLanguageServerPool(language string, resourceLimits *config.ResourceLimits, strategy string) *LanguageServerPool {
	return NewLanguageServerPoolWithConfig(language, resourceLimits, &config.LoadBalancingConfig{
		Strategy:        strategy,
		HealthThreshold: 0.8,
	}, nil)
}

// NewLanguageServerPoolWithConfig creates a new language server pool with full configuration
func NewLanguageServerPoolWithConfig(language string, resourceLimits *config.ResourceLimits, lbConfig *config.LoadBalancingConfig, logger *log.Logger) *LanguageServerPool {
	if lbConfig == nil {
		lbConfig = &config.LoadBalancingConfig{
			Strategy:        "round_robin",
			HealthThreshold: 0.8,
		}
	}

	return &LanguageServerPool{
		language:          language,
		servers:           make(map[string]*ServerInstance),
		activeServers:     make([]*ServerInstance, 0),
		loadBalancer:      NewLoadBalancerWithConfig(lbConfig, logger),
		healthMonitor:     NewHealthMonitor(30 * time.Second),
		resourceLimits:    resourceLimits,
		selectionStrategy: lbConfig.Strategy,
		metrics:           NewPoolMetrics(),
	}
}

// AddServer adds a server to the pool
func (lsp *LanguageServerPool) AddServer(server *ServerInstance) error {
	lsp.mu.Lock()
	defer lsp.mu.Unlock()

	if _, exists := lsp.servers[server.config.Name]; exists {
		return fmt.Errorf("server %s already exists in pool for language %s", server.config.Name, lsp.language)
	}

	lsp.servers[server.config.Name] = server
	lsp.rebuildActiveServers()
	return nil
}

// RemoveServer removes a server from the pool
func (lsp *LanguageServerPool) RemoveServer(serverName string) error {
	lsp.mu.Lock()
	defer lsp.mu.Unlock()

	server, exists := lsp.servers[serverName]
	if !exists {
		return fmt.Errorf("server %s not found in pool for language %s", serverName, lsp.language)
	}

	if err := server.Stop(); err != nil {
		return fmt.Errorf("failed to stop server %s: %w", serverName, err)
	}

	delete(lsp.servers, serverName)
	lsp.rebuildActiveServers()
	return nil
}

// GetHealthyServers returns all healthy servers
func (lsp *LanguageServerPool) GetHealthyServers() []*ServerInstance {
	lsp.mu.RLock()
	defer lsp.mu.RUnlock()

	var healthy []*ServerInstance
	for _, server := range lsp.servers {
		if server.IsHealthy() {
			healthy = append(healthy, server)
		}
	}
	return healthy
}

// SelectServer selects the best server for a request
func (lsp *LanguageServerPool) SelectServer(requestType string) (*ServerInstance, error) {
	return lsp.SelectServerWithContext(requestType, &ServerSelectionContext{
		Method:   requestType,
		Priority: PriorityNormal,
		Timeout:  30 * time.Second,
	})
}

// SelectServerWithContext selects the best server for a request with context
func (lsp *LanguageServerPool) SelectServerWithContext(requestType string, context *ServerSelectionContext) (*ServerInstance, error) {
	lsp.mu.RLock()
	defer lsp.mu.RUnlock()

	if len(lsp.activeServers) == 0 {
		return nil, fmt.Errorf("no active servers available for language %s", lsp.language)
	}

	if context == nil {
		context = &ServerSelectionContext{
			Method:   requestType,
			Priority: PriorityNormal,
			Timeout:  30 * time.Second,
		}
	}

	server, err := lsp.loadBalancer.SelectServer(lsp, requestType, context)
	if err != nil {
		return nil, err
	}

	// Update pool metrics
	lsp.metrics.RecordServerSelection(server.config.Name, 0, true)
	return server, nil
}

// SelectMultipleServers selects multiple servers for concurrent requests
func (lsp *LanguageServerPool) SelectMultipleServers(maxServers int) ([]*ServerInstance, error) {
	return lsp.SelectMultipleServersWithType("", maxServers)
}

// SelectMultipleServersWithType selects multiple servers for concurrent requests with request type
func (lsp *LanguageServerPool) SelectMultipleServersWithType(requestType string, maxServers int) ([]*ServerInstance, error) {
	lsp.mu.RLock()
	defer lsp.mu.RUnlock()

	if len(lsp.activeServers) == 0 {
		return nil, fmt.Errorf("no active servers available for language %s", lsp.language)
	}

	servers, err := lsp.loadBalancer.SelectMultipleServers(lsp, requestType, maxServers)
	if err != nil {
		return nil, err
	}

	// Update pool metrics for each selected server
	for _, server := range servers {
		lsp.metrics.RecordServerSelection(server.config.Name, 0, true)
	}

	return servers, nil
}

// rebuildActiveServers rebuilds the active servers list (must be called with lock held)
func (lsp *LanguageServerPool) rebuildActiveServers() {
	lsp.activeServers = lsp.activeServers[:0]
	for _, server := range lsp.servers {
		if server.IsActive() {
			lsp.activeServers = append(lsp.activeServers, server)
		}
	}

	// Sort by performance for consistent ordering
	sort.Slice(lsp.activeServers, func(i, j int) bool {
		return lsp.activeServers[i].metrics.GetAverageResponseTime() < lsp.activeServers[j].metrics.GetAverageResponseTime()
	})
}

// GetMetrics returns pool metrics
func (lsp *LanguageServerPool) GetMetrics() *PoolMetrics {
	lsp.mu.RLock()
	defer lsp.mu.RUnlock()
	return lsp.metrics.Copy()
}

// MultiServerManager manages multiple LSP server pools
type MultiServerManager struct {
	config           *config.GatewayConfig
	serverPools      map[string]*LanguageServerPool
	healthMonitor    *HealthMonitor
	loadBalancer     *LoadBalancer
	metrics          *ManagerMetrics
	circuitBreakers  map[string]*CircuitBreaker
	resourceMonitor  *ResourceMonitor
	smartRouter      *SmartRouterImpl
	workspaceManager *WorkspaceManager
	ctx              context.Context
	cancel           context.CancelFunc
	mu               sync.RWMutex
	logger           *log.Logger
}

// NewMultiServerManager creates a new multi-server manager
func NewMultiServerManager(config *config.GatewayConfig, logger *log.Logger) *MultiServerManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &MultiServerManager{
		config:          config,
		serverPools:     make(map[string]*LanguageServerPool),
		healthMonitor:   NewHealthMonitor(30 * time.Second),
		loadBalancer:    NewLoadBalancer("round_robin"),
		metrics:         NewManagerMetrics(),
		circuitBreakers: make(map[string]*CircuitBreaker),
		resourceMonitor: NewResourceMonitor(),
		ctx:             ctx,
		cancel:          cancel,
		logger:          logger,
	}
}

// Initialize initializes the multi-server manager
func (msm *MultiServerManager) Initialize() error {
	msm.mu.Lock()
	defer msm.mu.Unlock()

	if msm.config.LanguagePools == nil {
		return fmt.Errorf("no language pools configured")
	}

	// Initialize server pools for each language
	for _, poolConfig := range msm.config.LanguagePools {
		lbConfig := poolConfig.LoadBalancingConfig
		if lbConfig == nil {
			lbConfig = &config.LoadBalancingConfig{
				Strategy:        "round_robin",
				HealthThreshold: 0.8,
			}
		}

		pool := NewLanguageServerPoolWithConfig(poolConfig.Language, poolConfig.ResourceLimits, lbConfig, msm.logger)
		msm.serverPools[poolConfig.Language] = pool

		// Initialize servers in the pool
		for serverName, serverConfig := range poolConfig.Servers {
			client, err := msm.createLSPClient(serverConfig)
			if err != nil {
				msm.logger.Printf("Failed to create LSP client for server %s: %v", serverName, err)
				continue
			}

			server := NewServerInstance(serverConfig, client)
			if err := pool.AddServer(server); err != nil {
				msm.logger.Printf("Failed to add server %s to pool %s: %v", serverName, poolConfig.Language, err)
				continue
			}

			// Create circuit breaker for this server
			msm.circuitBreakers[serverName] = NewCircuitBreaker(10, 30*time.Second, 5)
		}
	}

	return nil
}

// Start starts all server pools
func (msm *MultiServerManager) Start() error {
	msm.mu.Lock()
	defer msm.mu.Unlock()

	// Start health monitoring
	if err := msm.healthMonitor.StartMonitoring(msm.ctx); err != nil {
		return fmt.Errorf("failed to start health monitoring: %w", err)
	}

	// Start resource monitoring
	if err := msm.resourceMonitor.StartMonitoring(msm.ctx); err != nil {
		return fmt.Errorf("failed to start resource monitoring: %w", err)
	}

	// Start all servers in all pools concurrently
	var wg sync.WaitGroup
	errChan := make(chan error, len(msm.serverPools)*10) // Buffer for potential errors

	for language, pool := range msm.serverPools {
		wg.Add(1)
		go func(lang string, p *LanguageServerPool) {
			defer wg.Done()
			if err := msm.startServerPool(lang, p); err != nil {
				errChan <- fmt.Errorf("failed to start server pool %s: %w", lang, err)
			}
		}(language, pool)
	}

	wg.Wait()
	close(errChan)

	// Collect any errors
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to start some server pools: %v", errors)
	}

	msm.logger.Printf("MultiServerManager started successfully with %d language pools", len(msm.serverPools))
	return nil
}

// Stop stops all server pools
func (msm *MultiServerManager) Stop() error {
	msm.mu.Lock()
	defer msm.mu.Unlock()

	msm.cancel() // Cancel context to stop monitoring

	var wg sync.WaitGroup
	for language, pool := range msm.serverPools {
		wg.Add(1)
		go func(lang string, p *LanguageServerPool) {
			defer wg.Done()
			msm.stopServerPool(lang, p)
		}(language, pool)
	}

	wg.Wait()
	msm.logger.Printf("MultiServerManager stopped successfully")
	return nil
}

// GetServerForRequest gets the best server for a specific request
func (msm *MultiServerManager) GetServerForRequest(language string, requestType string) (*ServerInstance, error) {
	return msm.GetServerForRequestWithContext(language, requestType, nil)
}

// GetServerForRequestWithContext gets the best server for a specific request with context
func (msm *MultiServerManager) GetServerForRequestWithContext(language string, requestType string, context *ServerSelectionContext) (*ServerInstance, error) {
	msm.mu.RLock()
	defer msm.mu.RUnlock()

	pool, exists := msm.serverPools[language]
	if !exists {
		return nil, fmt.Errorf("no server pool found for language %s", language)
	}

	if context == nil {
		context = &ServerSelectionContext{
			Method:   requestType,
			Priority: PriorityNormal,
			Timeout:  30 * time.Second,
		}
	}

	startTime := time.Now()
	server, err := pool.SelectServerWithContext(requestType, context)
	if err != nil {
		msm.metrics.RecordRequest(time.Since(startTime), false)
		return nil, fmt.Errorf("failed to select server for language %s: %w", language, err)
	}

	msm.metrics.RecordRequest(time.Since(startTime), true)
	return server, nil
}

// GetServersForConcurrentRequest gets multiple servers for concurrent requests
func (msm *MultiServerManager) GetServersForConcurrentRequest(language string, maxServers int) ([]*ServerInstance, error) {
	return msm.GetServersForConcurrentRequestWithType(language, "", maxServers)
}

// GetServersForConcurrentRequestWithType gets multiple servers for concurrent requests with request type
func (msm *MultiServerManager) GetServersForConcurrentRequestWithType(language string, requestType string, maxServers int) ([]*ServerInstance, error) {
	msm.mu.RLock()
	defer msm.mu.RUnlock()

	pool, exists := msm.serverPools[language]
	if !exists {
		return nil, fmt.Errorf("no server pool found for language %s", language)
	}

	startTime := time.Now()
	servers, err := pool.SelectMultipleServersWithType(requestType, maxServers)
	if err != nil {
		msm.metrics.RecordRequest(time.Since(startTime), false)
		return nil, fmt.Errorf("failed to select multiple servers for language %s: %w", language, err)
	}

	msm.metrics.RecordRequest(time.Since(startTime), true)
	return servers, nil
}

// GetHealthyServers gets all healthy servers for a language
func (msm *MultiServerManager) GetHealthyServers(language string) ([]*ServerInstance, error) {
	msm.mu.RLock()
	defer msm.mu.RUnlock()

	pool, exists := msm.serverPools[language]
	if !exists {
		return nil, fmt.Errorf("no server pool found for language %s", language)
	}

	return pool.GetHealthyServers(), nil
}

// GetServerPool gets the server pool for a language
func (msm *MultiServerManager) GetServerPool(language string) (*LanguageServerPool, error) {
	msm.mu.RLock()
	defer msm.mu.RUnlock()

	pool, exists := msm.serverPools[language]
	if !exists {
		return nil, fmt.Errorf("no server pool found for language %s", language)
	}

	return pool, nil
}

// RestartServer restarts a specific server
func (msm *MultiServerManager) RestartServer(serverName string) error {
	msm.mu.Lock()
	defer msm.mu.Unlock()

	// Find the server in all pools
	for _, pool := range msm.serverPools {
		if server, exists := pool.servers[serverName]; exists {
			msm.logger.Printf("Restarting server %s", serverName)

			// Stop the server
			if err := server.Stop(); err != nil {
				return fmt.Errorf("failed to stop server %s: %w", serverName, err)
			}

			// Create new client
			client, err := msm.createLSPClient(server.config)
			if err != nil {
				return fmt.Errorf("failed to create new LSP client for server %s: %w", serverName, err)
			}

			// Replace client and restart
			server.client = client
			server.state = ServerStateStarting
			server.startTime = time.Now()

			if err := server.Start(msm.ctx); err != nil {
				return fmt.Errorf("failed to restart server %s: %w", serverName, err)
			}

			// Reset circuit breaker
			if cb, exists := msm.circuitBreakers[serverName]; exists {
				cb.Reset()
			}

			msm.logger.Printf("Server %s restarted successfully", serverName)
			return nil
		}
	}

	return fmt.Errorf("server %s not found", serverName)
}

// MarkServerUnhealthy marks a server as unhealthy
func (msm *MultiServerManager) MarkServerUnhealthy(serverName string, reason string) error {
	msm.mu.Lock()
	defer msm.mu.Unlock()

	for _, pool := range msm.serverPools {
		if server, exists := pool.servers[serverName]; exists {
			server.SetState(ServerStateUnhealthy)
			server.healthStatus.IsHealthy = false
			server.healthStatus.LastError = reason
			server.healthStatus.LastHealthCheck = time.Now()
			server.healthStatus.ConsecutiveFailures++

			msm.logger.Printf("Server %s marked as unhealthy: %s", serverName, reason)
			pool.rebuildActiveServers()
			return nil
		}
	}

	return fmt.Errorf("server %s not found", serverName)
}

// MarkServerHealthy marks a server as healthy
func (msm *MultiServerManager) MarkServerHealthy(serverName string) error {
	msm.mu.Lock()
	defer msm.mu.Unlock()

	for _, pool := range msm.serverPools {
		if server, exists := pool.servers[serverName]; exists {
			server.SetState(ServerStateHealthy)
			server.healthStatus.IsHealthy = true
			server.healthStatus.LastError = ""
			server.healthStatus.LastHealthCheck = time.Now()
			server.healthStatus.ConsecutiveFailures = 0

			msm.logger.Printf("Server %s marked as healthy", serverName)
			pool.rebuildActiveServers()
			return nil
		}
	}

	return fmt.Errorf("server %s not found", serverName)
}

// GetMetrics returns overall manager metrics
func (msm *MultiServerManager) GetMetrics() *ManagerMetrics {
	msm.mu.RLock()
	defer msm.mu.RUnlock()
	return msm.metrics.Copy()
}

// UpdateServerMetrics updates performance metrics for a specific server
func (msm *MultiServerManager) UpdateServerMetrics(serverName string, responseTime time.Duration, success bool) error {
	msm.mu.RLock()
	defer msm.mu.RUnlock()

	// Find the server and update its metrics
	for _, pool := range msm.serverPools {
		if server, exists := pool.servers[serverName]; exists {
			server.UpdateMetrics(responseTime, success)
			
			// Update load balancer metrics
			pool.loadBalancer.UpdateServerMetrics(serverName, responseTime, success)
			pool.metrics.RecordServerSelection(serverName, responseTime, success)
			
			// Update manager metrics
			msm.metrics.RecordRequest(responseTime, success)
			
			return nil
		}
	}

	return fmt.Errorf("server %s not found in any pool", serverName)
}

// RebalancePool rebalances a specific language server pool
func (msm *MultiServerManager) RebalancePool(language string) error {
	msm.mu.Lock()
	defer msm.mu.Unlock()

	pool, exists := msm.serverPools[language]
	if !exists {
		return fmt.Errorf("no server pool found for language %s", language)
	}

	if err := pool.loadBalancer.RebalancePool(pool); err != nil {
		return fmt.Errorf("failed to rebalance pool for language %s: %w", language, err)
	}

	// Update pool metrics after rebalancing
	healthy := len(pool.GetHealthyServers())
	total := len(pool.servers)
	active := len(pool.activeServers)
	pool.metrics.UpdatePoolStatus(language, total, active, healthy, pool.loadBalancer.GetStrategy())

	msm.logger.Printf("Successfully rebalanced pool for language %s (strategy: %s, healthy: %d/%d)", 
		language, pool.loadBalancer.GetStrategy(), healthy, total)

	return nil
}

// RebalanceAllPools rebalances all language server pools
func (msm *MultiServerManager) RebalanceAllPools() error {
	msm.mu.Lock()
	defer msm.mu.Unlock()

	var errors []error
	for language, pool := range msm.serverPools {
		if err := pool.loadBalancer.RebalancePool(pool); err != nil {
			errors = append(errors, fmt.Errorf("failed to rebalance pool for language %s: %w", language, err))
		} else {
			// Update pool metrics after rebalancing
			healthy := len(pool.GetHealthyServers())
			total := len(pool.servers)
			active := len(pool.activeServers)
			pool.metrics.UpdatePoolStatus(language, total, active, healthy, pool.loadBalancer.GetStrategy())
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to rebalance some pools: %v", errors)
	}

	msm.logger.Printf("Successfully rebalanced all %d language server pools", len(msm.serverPools))
	return nil
}

// SetLoadBalancingStrategy changes the load balancing strategy for a specific language pool
func (msm *MultiServerManager) SetLoadBalancingStrategy(language string, strategy string) error {
	msm.mu.Lock()
	defer msm.mu.Unlock()

	pool, exists := msm.serverPools[language]
	if !exists {
		return fmt.Errorf("no server pool found for language %s", language)
	}

	config := &config.LoadBalancingConfig{
		Strategy:        strategy,
		HealthThreshold: 0.8,
	}

	if err := pool.loadBalancer.SetStrategy(strategy, config); err != nil {
		return fmt.Errorf("failed to set strategy %s for language %s: %w", strategy, language, err)
	}

	pool.selectionStrategy = strategy
	pool.metrics.UpdatePoolStatus(language, len(pool.servers), len(pool.activeServers), len(pool.GetHealthyServers()), strategy)

	msm.logger.Printf("Changed load balancing strategy for language %s to %s", language, strategy)
	return nil
}

// GetLoadBalancingStats returns comprehensive load balancing statistics
func (msm *MultiServerManager) GetLoadBalancingStats() map[string]*LoadBalancingStats {
	msm.mu.RLock()
	defer msm.mu.RUnlock()

	stats := make(map[string]*LoadBalancingStats)
	for language, pool := range msm.serverPools {
		stats[language] = pool.loadBalancer.GetLoadBalancingStats()
	}

	return stats
}

// createLSPClient creates an LSP client for the given server configuration
func (msm *MultiServerManager) createLSPClient(serverConfig *config.ServerConfig) (transport.LSPClient, error) {
	clientConfig := transport.ClientConfig{
		Command:   serverConfig.Command,
		Args:      serverConfig.Args,
		Transport: serverConfig.Transport,
	}

	client, err := transport.NewLSPClient(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create LSP client for server %s: %w", serverConfig.Name, err)
	}

	return client, nil
}

// startServerPool starts all servers in a pool
func (msm *MultiServerManager) startServerPool(language string, pool *LanguageServerPool) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(pool.servers))

	for serverName, server := range pool.servers {
		wg.Add(1)
		go func(name string, srv *ServerInstance) {
			defer wg.Done()
			if err := srv.Start(msm.ctx); err != nil {
				errChan <- fmt.Errorf("failed to start server %s: %w", name, err)
			} else {
				msm.logger.Printf("Server %s started successfully", name)
			}
		}(serverName, server)
	}

	wg.Wait()
	close(errChan)

	// Collect errors
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	pool.rebuildActiveServers()

	if len(errors) > 0 {
		return fmt.Errorf("failed to start some servers in pool %s: %v", language, errors)
	}

	return nil
}

// stopServerPool stops all servers in a pool
func (msm *MultiServerManager) stopServerPool(language string, pool *LanguageServerPool) {
	var wg sync.WaitGroup

	for serverName, server := range pool.servers {
		wg.Add(1)
		go func(name string, srv *ServerInstance) {
			defer wg.Done()
			if err := srv.Stop(); err != nil {
				msm.logger.Printf("Error stopping server %s: %v", name, err)
			} else {
				msm.logger.Printf("Server %s stopped successfully", name)
			}
		}(serverName, server)
	}

	wg.Wait()
}

// ServerManager interface for gateway integration
type ServerManager interface {
	GetServerForRequest(language string, requestType string) (transport.LSPClient, error)
	GetServersForConcurrentRequest(language string, maxServers int) ([]transport.LSPClient, error)
	GetHealthyServers(language string) ([]transport.LSPClient, error)
	StartAll() error
	StopAll() error
	GetMetrics() *ManagerMetrics
}

// Ensure MultiServerManager implements ServerManager interface
var _ ServerManager = (*MultiServerManager)(nil)

// Implementation of ServerManager interface methods that return transport.LSPClient
func (msm *MultiServerManager) StartAll() error {
	return msm.Start()
}

func (msm *MultiServerManager) StopAll() error {
	return msm.Stop()
}
