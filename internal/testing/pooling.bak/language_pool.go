package pooling

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"lsp-gateway/internal/transport"
)

// LanguagePool manages a pool of LSP servers for a specific language
type LanguagePool struct {
	// Configuration
	language string
	config   *LanguagePoolConfig
	
	// Server management
	servers      []*PooledServer
	available    chan *PooledServer
	clientFactory func() (transport.LSPClient, error)
	
	// State management
	mu              sync.RWMutex
	isRunning       bool
	ctx             context.Context
	cancel          context.CancelFunc
	
	// Metrics and health
	totalAllocations int64
	failedAllocations int64
	creationErrors   int64
	lastActivity     time.Time
	
	// Allocation strategy state
	roundRobinIndex int
	
	// Background tasks
	healthCheckTicker *time.Ticker
	maintenanceTicker *time.Ticker
}

// NewLanguagePool creates a new language-specific server pool
func NewLanguagePool(language string, config *LanguagePoolConfig, clientFactory func() (transport.LSPClient, error)) *LanguagePool {
	ctx, cancel := context.WithCancel(context.Background())
	
	lp := &LanguagePool{
		language:      language,
		config:        config,
		servers:       make([]*PooledServer, 0, config.MaxServers),
		available:     make(chan *PooledServer, config.MaxServers),
		clientFactory: clientFactory,
		ctx:           ctx,
		cancel:        cancel,
		lastActivity:  time.Now(),
	}
	
	return lp
}

// Start initializes the language pool and creates initial servers
func (lp *LanguagePool) Start(ctx context.Context) error {
	lp.mu.Lock()
	defer lp.mu.Unlock()
	
	if lp.isRunning {
		return fmt.Errorf("language pool for %s is already running", lp.language)
	}
	
	// Create initial servers
	if err := lp.warmupServersUnsafe(ctx); err != nil {
		return fmt.Errorf("failed to warmup servers for %s: %w", lp.language, err)
	}
	
	// Start background tasks
	lp.startBackgroundTasksUnsafe()
	
	lp.isRunning = true
	return nil
}

// Stop gracefully stops the language pool
func (lp *LanguagePool) Stop() error {
	lp.mu.Lock()
	defer lp.mu.Unlock()
	
	if !lp.isRunning {
		return nil
	}
	
	lp.isRunning = false
	
	// Cancel background tasks
	if lp.cancel != nil {
		lp.cancel()
	}
	
	// Stop tickers
	if lp.healthCheckTicker != nil {
		lp.healthCheckTicker.Stop()
	}
	if lp.maintenanceTicker != nil {
		lp.maintenanceTicker.Stop()
	}
	
	// Stop all servers
	var errors []error
	for _, server := range lp.servers {
		if err := server.Stop(); err != nil {
			errors = append(errors, err)
		}
	}
	
	// Clear the pool
	lp.servers = nil
	lp.drainAvailableChannel()
	
	if len(errors) > 0 {
		return fmt.Errorf("failed to stop some servers: %v", errors)
	}
	
	return nil
}

// GetServer allocates a server from the pool
func (lp *LanguagePool) GetServer(ctx context.Context, workspace string) (*PooledServer, error) {
	if !lp.isRunning {
		return nil, fmt.Errorf("language pool for %s is not running", lp.language)
	}
	
	lp.mu.Lock()
	lp.totalAllocations++
	lp.lastActivity = time.Now()
	lp.mu.Unlock()
	
	// Try to get an available server with timeout
	timeout := lp.config.WarmupTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	
	select {
	case server := <-lp.available:
		if err := server.Acquire(workspace); err != nil {
			// Server is not usable, try to create a new one
			lp.mu.Lock()
			lp.failedAllocations++
			lp.mu.Unlock()
			
			return lp.createServerOnDemand(ctx, workspace)
		}
		return server, nil
		
	case <-time.After(timeout):
		// No server available, try to create one
		return lp.createServerOnDemand(ctx, workspace)
		
	case <-ctx.Done():
		lp.mu.Lock()
		lp.failedAllocations++
		lp.mu.Unlock()
		return nil, ctx.Err()
	}
}

// ReturnServer returns a server to the pool
func (lp *LanguagePool) ReturnServer(server *PooledServer) error {
	if server == nil {
		return fmt.Errorf("cannot return nil server")
	}
	
	if server.language != lp.language {
		return fmt.Errorf("server language %s does not match pool language %s", server.language, lp.language)
	}
	
	// Release the server
	server.Release()
	
	// Check if server should be replaced
	if server.ShouldReplace() {
		lp.replaceServer(server)
		return nil
	}
	
	// Return to available pool
	select {
	case lp.available <- server:
		return nil
	default:
		// Channel is full, server will be garbage collected
		return fmt.Errorf("available channel is full, server discarded")
	}
}

// GetHealth returns health information for the language pool
func (lp *LanguagePool) GetHealth() *LanguageHealth {
	lp.mu.RLock()
	defer lp.mu.RUnlock()
	
	health := &LanguageHealth{
		Language:         lp.language,
		TotalServers:     len(lp.servers),
		HealthyServers:   0,
		ActiveServers:    0,
		IdleServers:      0,
		FailedServers:    0,
		AverageUseCount:  0,
		LastUsed:         lp.lastActivity,
		CreationErrors:   int(lp.creationErrors),
		AllocationErrors: int(lp.failedAllocations),
	}
	
	var totalUseCount int
	for _, server := range lp.servers {
		info := server.GetInfo()
		
		switch info.HealthStatus {
		case HealthStatusHealthy:
			health.HealthyServers++
		case HealthStatusUnhealthy:
			health.FailedServers++
		}
		
		switch info.State {
		case ServerStateBusy:
			health.ActiveServers++
		case ServerStateIdle, ServerStateReady:
			health.IdleServers++
		}
		
		totalUseCount += info.UseCount
	}
	
	if len(lp.servers) > 0 {
		health.AverageUseCount = float64(totalUseCount) / float64(len(lp.servers))
	}
	
	return health
}

// GetServerInfo returns information about all servers in the pool
func (lp *LanguagePool) GetServerInfo() []*PooledServerInfo {
	lp.mu.RLock()
	defer lp.mu.RUnlock()
	
	info := make([]*PooledServerInfo, len(lp.servers))
	for i, server := range lp.servers {
		info[i] = server.GetInfo()
	}
	
	return info
}

// warmupServersUnsafe creates initial servers (must be called with lock held)
func (lp *LanguagePool) warmupServersUnsafe(ctx context.Context) error {
	warmupCount := lp.config.WarmupServers
	if warmupCount > lp.config.MaxServers {
		warmupCount = lp.config.MaxServers
	}
	
	for i := 0; i < warmupCount; i++ {
		server, err := lp.createServerUnsafe(ctx)
		if err != nil {
			lp.creationErrors++
			// Continue creating other servers even if one fails
			continue
		}
		
		lp.servers = append(lp.servers, server)
		
		// Make the server available
		select {
		case lp.available <- server:
		default:
			// Channel is full, this shouldn't happen during warmup
		}
	}
	
	return nil
}

// createServerUnsafe creates a new pooled server (must be called with lock held)
func (lp *LanguagePool) createServerUnsafe(ctx context.Context) (*PooledServer, error) {
	// Create the underlying LSP client
	client, err := lp.clientFactory()
	if err != nil {
		return nil, fmt.Errorf("failed to create LSP client: %w", err)
	}
	
	// Create the pooled server wrapper
	server := NewPooledServer(lp.language, client, lp.config)
	
	// Start the server
	if err := server.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start pooled server: %w", err)
	}
	
	return server, nil
}

// createServerOnDemand creates a server when the pool is empty
func (lp *LanguagePool) createServerOnDemand(ctx context.Context, workspace string) (*PooledServer, error) {
	lp.mu.Lock()
	defer lp.mu.Unlock()
	
	// Check if we've hit the max server limit
	if len(lp.servers) >= lp.config.MaxServers {
		lp.failedAllocations++
		return nil, fmt.Errorf("maximum servers reached for language %s", lp.language)
	}
	
	// Create a new server
	server, err := lp.createServerUnsafe(ctx)
	if err != nil {
		lp.creationErrors++
		lp.failedAllocations++
		return nil, err
	}
	
	// Add to the pool
	lp.servers = append(lp.servers, server)
	
	// Acquire it for the requested workspace
	if err := server.Acquire(workspace); err != nil {
		lp.failedAllocations++
		return nil, err
	}
	
	return server, nil
}

// replaceServer replaces a server that should no longer be used
func (lp *LanguagePool) replaceServer(oldServer *PooledServer) {
	go func() {
		// Stop the old server
		if err := oldServer.Stop(); err != nil {
			// Log error but continue
		}
		
		lp.mu.Lock()
		defer lp.mu.Unlock()
		
		// Remove from servers list
		for i, server := range lp.servers {
			if server == oldServer {
				lp.servers = append(lp.servers[:i], lp.servers[i+1:]...)
				break
			}
		}
		
		// Create a replacement if we're below minimum
		if len(lp.servers) < lp.config.MinServers {
			ctx, cancel := context.WithTimeout(lp.ctx, lp.config.WarmupTimeout)
			defer cancel()
			
			newServer, err := lp.createServerUnsafe(ctx)
			if err != nil {
				lp.creationErrors++
				return
			}
			
			lp.servers = append(lp.servers, newServer)
			
			// Make it available
			select {
			case lp.available <- newServer:
			default:
				// Channel is full, server will be available for next request
			}
		}
	}()
}

// startBackgroundTasksUnsafe starts health checking and maintenance tasks
func (lp *LanguagePool) startBackgroundTasksUnsafe() {
	// Health check ticker
	if lp.config != nil && lp.config.IdleTimeout > 0 {
		lp.healthCheckTicker = time.NewTicker(lp.config.IdleTimeout / 4)
		go lp.healthCheckLoop()
	}
	
	// Maintenance ticker
	lp.maintenanceTicker = time.NewTicker(60 * time.Second)
	go lp.maintenanceLoop()
}

// healthCheckLoop performs periodic health checks on servers
func (lp *LanguagePool) healthCheckLoop() {
	for {
		select {
		case <-lp.healthCheckTicker.C:
			lp.performHealthChecks()
		case <-lp.ctx.Done():
			return
		}
	}
}

// maintenanceLoop performs periodic maintenance tasks
func (lp *LanguagePool) maintenanceLoop() {
	for {
		select {
		case <-lp.maintenanceTicker.C:
			lp.performMaintenance()
		case <-lp.ctx.Done():
			return
		}
	}
}

// performHealthChecks checks the health of all servers
func (lp *LanguagePool) performHealthChecks() {
	lp.mu.RLock()
	servers := make([]*PooledServer, len(lp.servers))
	copy(servers, lp.servers)
	lp.mu.RUnlock()
	
	ctx, cancel := context.WithTimeout(lp.ctx, 5*time.Second)
	defer cancel()
	
	for _, server := range servers {
		if err := server.HealthCheck(ctx); err != nil {
			// Server is unhealthy, consider replacing it
			if server.ShouldReplace() {
				lp.replaceServer(server)
			}
		}
	}
}

// performMaintenance performs periodic maintenance tasks
func (lp *LanguagePool) performMaintenance() {
	lp.mu.Lock()
	defer lp.mu.Unlock()
	
	// Remove servers that should be replaced
	var serversToReplace []*PooledServer
	filteredServers := make([]*PooledServer, 0, len(lp.servers))
	
	for _, server := range lp.servers {
		if server.ShouldReplace() {
			serversToReplace = append(serversToReplace, server)
		} else {
			filteredServers = append(filteredServers, server)
		}
	}
	
	lp.servers = filteredServers
	
	// Replace servers asynchronously
	for _, server := range serversToReplace {
		lp.replaceServer(server)
	}
}

// drainAvailableChannel drains the available server channel
func (lp *LanguagePool) drainAvailableChannel() {
	for {
		select {
		case <-lp.available:
			// Drain the channel
		default:
			return
		}
	}
}

// selectServerByStrategy selects a server using the configured allocation strategy
func (lp *LanguagePool) selectServerByStrategy(servers []*PooledServer) *PooledServer {
	if len(servers) == 0 {
		return nil
	}
	
	switch lp.config.AllocationStrategy {
	case AllocationStrategyLeastUsed:
		return lp.selectLeastUsedServer(servers)
	case AllocationStrategyMostRecent:
		return lp.selectMostRecentServer(servers)
	case AllocationStrategyRandom:
		return servers[rand.Intn(len(servers))]
	case AllocationStrategyRoundRobin:
		fallthrough
	default:
		return lp.selectRoundRobinServer(servers)
	}
}

// selectLeastUsedServer selects the server with the lowest use count
func (lp *LanguagePool) selectLeastUsedServer(servers []*PooledServer) *PooledServer {
	if len(servers) == 0 {
		return nil
	}
	
	leastUsed := servers[0]
	leastUsedInfo := leastUsed.GetInfo()
	
	for _, server := range servers[1:] {
		info := server.GetInfo()
		if info.UseCount < leastUsedInfo.UseCount {
			leastUsed = server
			leastUsedInfo = info
		}
	}
	
	return leastUsed
}

// selectMostRecentServer selects the most recently used server
func (lp *LanguagePool) selectMostRecentServer(servers []*PooledServer) *PooledServer {
	if len(servers) == 0 {
		return nil
	}
	
	mostRecent := servers[0]
	mostRecentInfo := mostRecent.GetInfo()
	
	for _, server := range servers[1:] {
		info := server.GetInfo()
		if info.LastUsed.After(mostRecentInfo.LastUsed) {
			mostRecent = server
			mostRecentInfo = info
		}
	}
	
	return mostRecent
}

// selectRoundRobinServer selects the next server in round-robin order
func (lp *LanguagePool) selectRoundRobinServer(servers []*PooledServer) *PooledServer {
	if len(servers) == 0 {
		return nil
	}
	
	server := servers[lp.roundRobinIndex%len(servers)]
	lp.roundRobinIndex++
	
	return server
}