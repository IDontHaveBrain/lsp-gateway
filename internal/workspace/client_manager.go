package workspace

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/transport"
	"lsp-gateway/mcp"
)

// SubProjectClientManager defines the interface for comprehensive LSP client lifecycle management
type SubProjectClientManager interface {
	GetClient(subProjectID, language string) (transport.LSPClient, error)
	CreateClient(ctx context.Context, subProject *SubProject, language string) (transport.LSPClient, error)
	RemoveClient(subProjectID, language string) error
	RemoveAllClients(subProjectID string) error
	GetClientStatus(subProjectID, language string) (*ExtendedClientStatus, error)
	GetAllClientStatuses() map[string]map[string]*ExtendedClientStatus
	HealthCheck(ctx context.Context) (*ClientManagerHealth, error)
	StartClients(ctx context.Context, subProject *SubProject) error
	StopClients(ctx context.Context, subProject *SubProject) error
	RefreshClients(ctx context.Context, subProject *SubProject) error
	Shutdown(ctx context.Context) error
	GetLoadBalancingInfo() *LoadBalancingInfo
	GetRegistry() *ClientRegistry
}

// ExtendedClientStatus represents enhanced status information for an LSP client
type ExtendedClientStatus struct {
	SubProjectID  string    `json:"sub_project_id"`
	Language      string    `json:"language"`
	IsActive      bool      `json:"is_active"`
	IsHealthy     bool      `json:"is_healthy"`
	LastUsed      time.Time `json:"last_used"`
	StartedAt     time.Time `json:"started_at"`
	RequestCount  int64     `json:"request_count"`
	ErrorCount    int64     `json:"error_count"`
	LastError     string    `json:"last_error,omitempty"`
	WorkspaceRoot string    `json:"workspace_root"`
	ServerCommand string    `json:"server_command,omitempty"`
	Transport     string    `json:"transport"`
	Health        *ClientHealthInfo `json:"health,omitempty"`
}

// ClientManagerHealth represents comprehensive health information for the client manager
type ClientManagerHealth struct {
	TotalClients     int                            `json:"total_clients"`
	ActiveClients    int                            `json:"active_clients"`
	HealthyClients   int                            `json:"healthy_clients"`
	FailedClients    int                            `json:"failed_clients"`
	ClientsByProject map[string]int                 `json:"clients_by_project"`
	ClientsByLanguage map[string]int                `json:"clients_by_language"`
	MemoryUsage      int64                          `json:"memory_usage_mb"`
	LastCheck        time.Time                      `json:"last_check"`
	RegistryHealth   *ClientRegistryHealth          `json:"registry_health"`
	LoadBalancing    *LoadBalancingInfo             `json:"load_balancing"`
	Errors           []string                       `json:"errors,omitempty"`
}

// LoadBalancingInfo provides information about client load distribution
type LoadBalancingInfo struct {
	TotalRequests         int64                     `json:"total_requests"`
	RequestsPerProject    map[string]int64          `json:"requests_per_project"`
	RequestsPerLanguage   map[string]int64          `json:"requests_per_language"`
	AverageResponseTime   time.Duration             `json:"average_response_time"`
	ResponseTimePerClient map[string]time.Duration  `json:"response_time_per_client"`
	LoadBalancingStrategy string                    `json:"load_balancing_strategy"`
	LastUpdated           time.Time                 `json:"last_updated"`
}

// ClientManagerConfig holds configuration for the client manager
type ClientManagerConfig struct {
	MaxClients              int           `yaml:"max_clients" json:"max_clients"`
	MaxClientsPerProject    int           `yaml:"max_clients_per_project" json:"max_clients_per_project"`
	MaxClientsPerLanguage   int           `yaml:"max_clients_per_language" json:"max_clients_per_language"`
	HealthCheckInterval     time.Duration `yaml:"health_check_interval" json:"health_check_interval"`
	ClientTimeout           time.Duration `yaml:"client_timeout" json:"client_timeout"`
	InitializationTimeout   time.Duration `yaml:"initialization_timeout" json:"initialization_timeout"`
	EnableLazyInitialization bool         `yaml:"enable_lazy_initialization" json:"enable_lazy_initialization"`
	EnableLoadBalancing     bool          `yaml:"enable_load_balancing" json:"enable_load_balancing"`
	EnableCircuitBreaker    bool          `yaml:"enable_circuit_breaker" json:"enable_circuit_breaker"`
	EnableAutoRestart       bool          `yaml:"enable_auto_restart" json:"enable_auto_restart"`
	IdleClientTimeout       time.Duration `yaml:"idle_client_timeout" json:"idle_client_timeout"`
	RestartBackoffStrategy  string        `yaml:"restart_backoff_strategy" json:"restart_backoff_strategy"`
	ResourceLimits          *ResourceLimits `yaml:"resource_limits,omitempty" json:"resource_limits,omitempty"`
}

// ResourceLimits defines resource constraints for client management
type ResourceLimits struct {
	MaxMemoryMB         int `yaml:"max_memory_mb" json:"max_memory_mb"`
	MaxCPUPercent       int `yaml:"max_cpu_percent" json:"max_cpu_percent"`
	MaxOpenFiles        int `yaml:"max_open_files" json:"max_open_files"`
	MaxConcurrentReqs   int `yaml:"max_concurrent_requests" json:"max_concurrent_requests"`
}

// DefaultClientManagerConfig returns a default configuration for the client manager
func DefaultClientManagerConfig() *ClientManagerConfig {
	return &ClientManagerConfig{
		MaxClients:              100,
		MaxClientsPerProject:    20,
		MaxClientsPerLanguage:   10,
		HealthCheckInterval:     30 * time.Second,
		ClientTimeout:           10 * time.Second,
		InitializationTimeout:   30 * time.Second,
		EnableLazyInitialization: true,
		EnableLoadBalancing:     true,
		EnableCircuitBreaker:    true,
		EnableAutoRestart:       true,
		IdleClientTimeout:       10 * time.Minute,
		RestartBackoffStrategy:  "exponential",
		ResourceLimits: &ResourceLimits{
			MaxMemoryMB:       1024,
			MaxCPUPercent:     80,
			MaxOpenFiles:      1000,
			MaxConcurrentReqs: 50,
		},
	}
}

// subProjectClientManager implements the SubProjectClientManager interface
type subProjectClientManager struct {
	// Core components
	registry        *ClientRegistry
	healthMonitor   *ClientHealthMonitor
	workspaceConfig *WorkspaceConfig
	config          *ClientManagerConfig
	logger          *mcp.StructuredLogger

	// Client selection and routing
	loadBalancer    *ClientLoadBalancer
	circuitBreaker  *CircuitBreakerManager
	clientSelector  *ClientSelector
	
	// Performance tracking
	requestCounter    int64
	successCounter    int64
	errorCounter      int64
	totalResponseTime int64
	
	// Lifecycle management
	ctx            context.Context
	cancel         context.CancelFunc
	healthTicker   *time.Ticker
	cleanupTicker  *time.Ticker
	isShutdown     int32
	
	// Synchronization
	mu             sync.RWMutex
	shutdownOnce   sync.Once
	startedClients map[string]map[string]bool // [subProjectID][language] -> started
}

// NewSubProjectClientManager creates a new client manager instance
func NewSubProjectClientManager(workspaceConfig *WorkspaceConfig, config *ClientManagerConfig, logger *mcp.StructuredLogger) SubProjectClientManager {
	if config == nil {
		config = DefaultClientManagerConfig()
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	manager := &subProjectClientManager{
		registry:        NewClientRegistry(config, logger),
		workspaceConfig: workspaceConfig,
		config:          config,
		logger:          logger,
		ctx:             ctx,
		cancel:          cancel,
		startedClients:  make(map[string]map[string]bool),
	}
	
	// Initialize sub-components
	manager.healthMonitor = NewClientHealthMonitor(manager.registry, config, logger)
	manager.loadBalancer = NewClientLoadBalancer(manager.registry, config, logger)
	manager.circuitBreaker = NewCircuitBreakerManager(config, logger)
	manager.clientSelector = NewClientSelector(manager.registry, manager.loadBalancer, config, logger)
	
	// Start background processes
	manager.startBackgroundProcesses()
	
	if logger != nil {
		logger.Info("SubProjectClientManager initialized successfully")
	}
	
	return manager
}

// startBackgroundProcesses starts health checking and cleanup processes
func (m *subProjectClientManager) startBackgroundProcesses() {
	if m.config.HealthCheckInterval > 0 {
		m.healthTicker = time.NewTicker(m.config.HealthCheckInterval)
		go m.runHealthChecks()
	}
	
	// Start cleanup process for idle clients
	if m.config.IdleClientTimeout > 0 {
		m.cleanupTicker = time.NewTicker(m.config.IdleClientTimeout / 2)
		go m.runIdleClientCleanup()
	}
}

// runHealthChecks performs periodic health checks on all clients
func (m *subProjectClientManager) runHealthChecks() {
	defer m.healthTicker.Stop()
	
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-m.healthTicker.C:
			if atomic.LoadInt32(&m.isShutdown) != 0 {
				return
			}
			
			ctx, cancel := context.WithTimeout(m.ctx, m.config.ClientTimeout)
			if err := m.performHealthCheck(ctx); err != nil && m.logger != nil {
				m.logger.WithError(err).Error("Health check failed")
			}
			cancel()
		}
	}
}

// runIdleClientCleanup removes clients that have been idle for too long
func (m *subProjectClientManager) runIdleClientCleanup() {
	defer m.cleanupTicker.Stop()
	
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-m.cleanupTicker.C:
			if atomic.LoadInt32(&m.isShutdown) != 0 {
				return
			}
			
			m.cleanupIdleClients()
		}
	}
}

// performHealthCheck conducts health checks on all registered clients
func (m *subProjectClientManager) performHealthCheck(ctx context.Context) error {
	return m.healthMonitor.PerformHealthCheck(ctx)
}

// cleanupIdleClients removes clients that haven't been used recently
func (m *subProjectClientManager) cleanupIdleClients() {
	if m.config.IdleClientTimeout == 0 {
		return
	}
	
	cutoff := time.Now().Add(-m.config.IdleClientTimeout)
	idleClients := m.registry.GetIdleClients(cutoff)
	
	for _, clientInfo := range idleClients {
		if err := m.RemoveClient(clientInfo.SubProjectID, clientInfo.Language); err != nil && m.logger != nil {
			m.logger.WithError(err).WithFields(map[string]interface{}{
				"sub_project_id": clientInfo.SubProjectID,
				"language":       clientInfo.Language,
			}).Warn("Failed to remove idle client")
		} else if m.logger != nil {
			m.logger.WithFields(map[string]interface{}{
				"sub_project_id": clientInfo.SubProjectID,
				"language":       clientInfo.Language,
				"idle_duration":  time.Since(clientInfo.LastUsed).String(),
			}).Info("Removed idle client")
		}
	}
}

// GetClient retrieves an LSP client for the specified sub-project and language
func (m *subProjectClientManager) GetClient(subProjectID, language string) (transport.LSPClient, error) {
	if atomic.LoadInt32(&m.isShutdown) != 0 {
		return nil, fmt.Errorf("client manager is shutdown")
	}
	
	// Update request metrics
	atomic.AddInt64(&m.requestCounter, 1)
	
	// Try to get existing client
	client, err := m.registry.GetClient(subProjectID, language)
	if err == nil && client != nil {
		// Update usage tracking
		m.registry.UpdateClientUsage(subProjectID, language)
		return client, nil
	}
	
	// Check if lazy initialization is enabled
	if !m.config.EnableLazyInitialization {
		return nil, fmt.Errorf("no client available for sub-project %s, language %s", subProjectID, language)
	}
	
	// Try to create client on-demand
	subProject := m.findSubProjectByID(subProjectID)
	if subProject == nil {
		return nil, fmt.Errorf("sub-project not found: %s", subProjectID)
	}
	
	return m.CreateClient(m.ctx, subProject, language)
}

// CreateClient creates a new LSP client for the specified sub-project and language
func (m *subProjectClientManager) CreateClient(ctx context.Context, subProject *SubProject, language string) (transport.LSPClient, error) {
	if atomic.LoadInt32(&m.isShutdown) != 0 {
		return nil, fmt.Errorf("client manager is shutdown")
	}
	
	// Check resource limits
	if err := m.checkResourceLimits(subProject.ID, language); err != nil {
		return nil, fmt.Errorf("resource limits exceeded: %w", err)
	}
	
	// Check circuit breaker
	if m.config.EnableCircuitBreaker {
		key := fmt.Sprintf("%s:%s", subProject.ID, language)
		if !m.circuitBreaker.AllowRequest(key) {
			return nil, fmt.Errorf("circuit breaker open for %s", key)
		}
	}
	
	// Find server configuration for the language
	serverConfig := m.findServerConfigForLanguage(language)
	if serverConfig == nil {
		return nil, fmt.Errorf("no server configuration found for language: %s", language)
	}
	
	// Create client configuration with workspace folder
	clientConfig := transport.ClientConfig{
		Command:   serverConfig.Command,
		Args:      m.buildClientArgs(serverConfig, subProject),
		Transport: serverConfig.Transport,
	}
	
	if clientConfig.Transport == "" {
		clientConfig.Transport = transport.TransportStdio
	}
	
	// Create the LSP client
	client, err := transport.NewLSPClient(clientConfig)
	if err != nil {
		atomic.AddInt64(&m.errorCounter, 1)
		if m.config.EnableCircuitBreaker {
			key := fmt.Sprintf("%s:%s", subProject.ID, language)
			m.circuitBreaker.RecordFailure(key)
		}
		return nil, fmt.Errorf("failed to create LSP client: %w", err)
	}
	
	// Start the client with timeout
	startCtx, cancel := context.WithTimeout(ctx, m.config.InitializationTimeout)
	defer cancel()
	
	if err := client.Start(startCtx); err != nil {
		atomic.AddInt64(&m.errorCounter, 1)
		if m.config.EnableCircuitBreaker {
			key := fmt.Sprintf("%s:%s", subProject.ID, language)
			m.circuitBreaker.RecordFailure(key)
		}
		return nil, fmt.Errorf("failed to start LSP client: %w", err)
	}
	
	// Initialize LSP workspace folders
	if err := m.initializeWorkspaceFolders(startCtx, client, subProject); err != nil {
		client.Stop() // Cleanup on failure
		atomic.AddInt64(&m.errorCounter, 1)
		if m.logger != nil {
			m.logger.WithError(err).WithFields(map[string]interface{}{
				"sub_project_id": subProject.ID,
				"language":       language,
			}).Warn("Failed to initialize workspace folders")
		}
		// Don't fail creation for workspace folder issues, just log
	}
	
	// Register the client
	clientInfo := &ClientInfo{
		Client:        client,
		SubProjectID:  subProject.ID,
		Language:      language,
		WorkspaceRoot: subProject.Root,
		ServerCommand: serverConfig.Command,
		Transport:     clientConfig.Transport,
		StartedAt:     time.Now(),
		LastUsed:      time.Now(),
	}
	
	if err := m.registry.RegisterClient(clientInfo); err != nil {
		client.Stop() // Cleanup on failure
		atomic.AddInt64(&m.errorCounter, 1)
		return nil, fmt.Errorf("failed to register client: %w", err)
	}
	
	// Record successful creation
	atomic.AddInt64(&m.successCounter, 1)
	if m.config.EnableCircuitBreaker {
		key := fmt.Sprintf("%s:%s", subProject.ID, language)
		m.circuitBreaker.RecordSuccess(key)
	}
	
	// Track started clients
	m.mu.Lock()
	if m.startedClients[subProject.ID] == nil {
		m.startedClients[subProject.ID] = make(map[string]bool)
	}
	m.startedClients[subProject.ID][language] = true
	m.mu.Unlock()
	
	if m.logger != nil {
		m.logger.WithFields(map[string]interface{}{
			"sub_project_id": subProject.ID,
			"language":       language,
			"workspace_root": subProject.Root,
			"transport":      clientConfig.Transport,
		}).Info("LSP client created and started successfully")
	}
	
	return client, nil
}

// RemoveClient removes an LSP client for the specified sub-project and language
func (m *subProjectClientManager) RemoveClient(subProjectID, language string) error {
	if atomic.LoadInt32(&m.isShutdown) != 0 {
		return fmt.Errorf("client manager is shutdown")
	}
	
	client, err := m.registry.GetClient(subProjectID, language)
	if err != nil {
		return err
	}
	
	if client == nil {
		return nil // Already removed
	}
	
	// Stop the client
	if err := client.Stop(); err != nil && m.logger != nil {
		m.logger.WithError(err).WithFields(map[string]interface{}{
			"sub_project_id": subProjectID,
			"language":       language,
		}).Warn("Error stopping LSP client")
	}
	
	// Unregister from registry
	if err := m.registry.UnregisterClient(subProjectID, language); err != nil {
		return fmt.Errorf("failed to unregister client: %w", err)
	}
	
	// Update tracking
	m.mu.Lock()
	if clients, exists := m.startedClients[subProjectID]; exists {
		delete(clients, language)
		if len(clients) == 0 {
			delete(m.startedClients, subProjectID)
		}
	}
	m.mu.Unlock()
	
	if m.logger != nil {
		m.logger.WithFields(map[string]interface{}{
			"sub_project_id": subProjectID,
			"language":       language,
		}).Info("LSP client removed successfully")
	}
	
	return nil
}

// RemoveAllClients removes all LSP clients for the specified sub-project
func (m *subProjectClientManager) RemoveAllClients(subProjectID string) error {
	if atomic.LoadInt32(&m.isShutdown) != 0 {
		return fmt.Errorf("client manager is shutdown")
	}
	
	clients := m.registry.GetAllClientsForProject(subProjectID)
	var errors []error
	
	for language := range clients {
		if err := m.RemoveClient(subProjectID, language); err != nil {
			errors = append(errors, fmt.Errorf("failed to remove %s client: %w", language, err))
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("errors removing clients: %v", errors)
	}
	
	if m.logger != nil {
		m.logger.WithField("sub_project_id", subProjectID).Info("All LSP clients removed for sub-project")
	}
	
	return nil
}

// GetClientStatus returns the status of a specific client
func (m *subProjectClientManager) GetClientStatus(subProjectID, language string) (*ExtendedClientStatus, error) {
	info := m.registry.GetClientInfo(subProjectID, language)
	if info == nil {
		return nil, fmt.Errorf("client not found: %s:%s", subProjectID, language)
	}
	
	isActive := info.Client.IsActive()
	health := m.healthMonitor.GetClientHealth(subProjectID, language)
	
	status := &ExtendedClientStatus{
		SubProjectID:  subProjectID,
		Language:      language,
		IsActive:      isActive,
		IsHealthy:     health != nil && health.IsHealthy,
		LastUsed:      info.LastUsed,
		StartedAt:     info.StartedAt,
		RequestCount:  info.RequestCount,
		ErrorCount:    info.ErrorCount,
		WorkspaceRoot: info.WorkspaceRoot,
		ServerCommand: info.ServerCommand,
		Transport:     info.Transport,
		Health:        health,
	}
	
	if health != nil && health.LastError != "" {
		status.LastError = health.LastError
	}
	
	return status, nil
}

// GetAllClientStatuses returns the status of all registered clients
func (m *subProjectClientManager) GetAllClientStatuses() map[string]map[string]*ExtendedClientStatus {
	result := make(map[string]map[string]*ExtendedClientStatus)
	
	allClients := m.registry.GetAllClients()
	for subProjectID, clients := range allClients {
		result[subProjectID] = make(map[string]*ExtendedClientStatus)
		for language := range clients {
			if status, err := m.GetClientStatus(subProjectID, language); err == nil {
				result[subProjectID][language] = status
			}
		}
	}
	
	return result
}

// HealthCheck performs a comprehensive health check of the client manager
func (m *subProjectClientManager) HealthCheck(ctx context.Context) (*ClientManagerHealth, error) {
	if atomic.LoadInt32(&m.isShutdown) != 0 {
		return nil, fmt.Errorf("client manager is shutdown")
	}
	
	registryHealth := m.registry.GetHealth()
	loadBalancing := m.loadBalancer.GetLoadBalancingInfo()
	
	totalClients := registryHealth.TotalClients
	activeClients := 0
	healthyClients := 0
	failedClients := 0
	clientsByProject := make(map[string]int)
	clientsByLanguage := make(map[string]int)
	var errors []string
	
	// Analyze client health
	allClients := m.registry.GetAllClients()
	for subProjectID, clients := range allClients {
		clientsByProject[subProjectID] = len(clients)
		
		for language, client := range clients {
			clientsByLanguage[language]++
			
			if client.IsActive() {
				activeClients++
			}
			
			health := m.healthMonitor.GetClientHealth(subProjectID, language)
			if health != nil {
				if health.IsHealthy {
					healthyClients++
				} else {
					failedClients++
					if health.LastError != "" {
						errors = append(errors, fmt.Sprintf("%s:%s - %s", subProjectID, language, health.LastError))
					}
				}
			}
		}
	}
	
	return &ClientManagerHealth{
		TotalClients:      totalClients,
		ActiveClients:     activeClients,
		HealthyClients:    healthyClients,
		FailedClients:     failedClients,
		ClientsByProject:  clientsByProject,
		ClientsByLanguage: clientsByLanguage,
		MemoryUsage:       registryHealth.MemoryUsageMB,
		LastCheck:         time.Now(),
		RegistryHealth:    registryHealth,
		LoadBalancing:     loadBalancing,
		Errors:            errors,
	}, nil
}

// StartClients starts all required LSP clients for a sub-project
func (m *subProjectClientManager) StartClients(ctx context.Context, subProject *SubProject) error {
	if atomic.LoadInt32(&m.isShutdown) != 0 {
		return fmt.Errorf("client manager is shutdown")
	}
	
	var errors []error
	
	// Start clients for each language in the sub-project
	for _, language := range subProject.Languages {
		if _, err := m.CreateClient(ctx, subProject, language); err != nil {
			errors = append(errors, fmt.Errorf("failed to start %s client: %w", language, err))
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("errors starting clients: %v", errors)
	}
	
	return nil
}

// StopClients stops all LSP clients for a sub-project
func (m *subProjectClientManager) StopClients(ctx context.Context, subProject *SubProject) error {
	return m.RemoveAllClients(subProject.ID)
}

// RefreshClients refreshes all LSP clients for a sub-project
func (m *subProjectClientManager) RefreshClients(ctx context.Context, subProject *SubProject) error {
	if atomic.LoadInt32(&m.isShutdown) != 0 {
		return fmt.Errorf("client manager is shutdown")
	}
	
	// Stop existing clients
	if err := m.StopClients(ctx, subProject); err != nil && m.logger != nil {
		m.logger.WithError(err).WithField("sub_project_id", subProject.ID).Warn("Error stopping clients during refresh")
	}
	
	// Start new clients
	return m.StartClients(ctx, subProject)
}

// Shutdown gracefully shuts down the client manager
func (m *subProjectClientManager) Shutdown(ctx context.Context) error {
	var shutdownErr error
	
	m.shutdownOnce.Do(func() {
		atomic.StoreInt32(&m.isShutdown, 1)
		
		if m.logger != nil {
			m.logger.Info("Shutting down SubProjectClientManager")
		}
		
		// Cancel context to stop background processes
		m.cancel()
		
		// Stop all clients
		allClients := m.registry.GetAllClients()
		var errors []error
		
		for subProjectID, clients := range allClients {
			for language := range clients {
				if err := m.RemoveClient(subProjectID, language); err != nil {
					errors = append(errors, fmt.Errorf("failed to remove client %s:%s: %w", subProjectID, language, err))
				}
			}
		}
		
		// Shutdown registry
		if err := m.registry.Shutdown(ctx); err != nil {
			errors = append(errors, fmt.Errorf("failed to shutdown registry: %w", err))
		}
		
		// Shutdown health monitor
		if err := m.healthMonitor.Shutdown(ctx); err != nil {
			errors = append(errors, fmt.Errorf("failed to shutdown health monitor: %w", err))
		}
		
		if len(errors) > 0 {
			shutdownErr = fmt.Errorf("errors during shutdown: %v", errors)
		}
		
		if m.logger != nil {
			if shutdownErr != nil {
				m.logger.WithError(shutdownErr).Error("SubProjectClientManager shutdown completed with errors")
			} else {
				m.logger.Info("SubProjectClientManager shutdown completed successfully")
			}
		}
	})
	
	return shutdownErr
}

// GetLoadBalancingInfo returns current load balancing information
func (m *subProjectClientManager) GetLoadBalancingInfo() *LoadBalancingInfo {
	return m.loadBalancer.GetLoadBalancingInfo()
}

// GetRegistry returns the client registry for advanced operations
func (m *subProjectClientManager) GetRegistry() *ClientRegistry {
	return m.registry
}

// Helper methods

func (m *subProjectClientManager) findSubProjectByID(subProjectID string) *SubProject {
	if m.workspaceConfig == nil || m.workspaceConfig.SubProjects == nil {
		return nil
	}
	
	for _, subProjectInfo := range m.workspaceConfig.SubProjects {
		if subProjectInfo.ID == subProjectID {
			// Convert SubProjectInfo to SubProject
			return &SubProject{
				ID:           subProjectInfo.ID,
				Name:         subProjectInfo.Name,
				Root:         subProjectInfo.AbsolutePath,
				RelativePath: subProjectInfo.RelativePath,
				ProjectType:  subProjectInfo.ProjectType,
				Languages:    subProjectInfo.Languages,
				PrimaryLang:  subProjectInfo.Languages[0], // Assume first is primary
				SourceDirs:   []string{subProjectInfo.AbsolutePath},
				ConfigFiles:  subProjectInfo.MarkerFiles,
				DetectedAt:   time.Now(),
				LastModified: time.Now(),
			}
		}
	}
	
	return nil
}

func (m *subProjectClientManager) findServerConfigForLanguage(language string) *config.ServerConfig {
	if m.workspaceConfig == nil || m.workspaceConfig.Servers == nil {
		return nil
	}
	
	for _, serverConfig := range m.workspaceConfig.Servers {
		for _, supportedLang := range serverConfig.Languages {
			if supportedLang == language {
				return serverConfig
			}
		}
	}
	
	return nil
}

func (m *subProjectClientManager) buildClientArgs(serverConfig *config.ServerConfig, subProject *SubProject) []string {
	args := make([]string, len(serverConfig.Args))
	copy(args, serverConfig.Args)
	
	// Replace workspace placeholders in arguments
	for i, arg := range args {
		args[i] = strings.ReplaceAll(arg, "${workspace_root}", subProject.Root)
		args[i] = strings.ReplaceAll(args[i], "${project_root}", subProject.Root)
		args[i] = strings.ReplaceAll(args[i], "${project_name}", subProject.Name)
	}
	
	return args
}

func (m *subProjectClientManager) initializeWorkspaceFolders(ctx context.Context, client transport.LSPClient, subProject *SubProject) error {
	// LSP Initialize request with workspace folders
	initParams := map[string]interface{}{
		"processId": nil,
		"clientInfo": map[string]interface{}{
			"name":    "lsp-gateway",
			"version": "1.0.0",
		},
		"rootUri": fmt.Sprintf("file://%s", subProject.Root),
		"capabilities": map[string]interface{}{
			"workspace": map[string]interface{}{
				"workspaceFolders": true,
			},
		},
		"workspaceFolders": []map[string]interface{}{
			{
				"uri":  fmt.Sprintf("file://%s", subProject.Root),
				"name": subProject.Name,
			},
		},
	}
	
	// Send initialize request
	_, err := client.SendRequest(ctx, "initialize", initParams)
	if err != nil {
		return fmt.Errorf("failed to send initialize request: %w", err)
	}
	
	// Send initialized notification
	return client.SendNotification(ctx, "initialized", map[string]interface{}{})
}

func (m *subProjectClientManager) checkResourceLimits(subProjectID, language string) error {
	if m.config.ResourceLimits == nil {
		return nil
	}
	
	registryHealth := m.registry.GetHealth()
	
	// Check global client limit
	if registryHealth.TotalClients >= m.config.MaxClients {
		return fmt.Errorf("maximum client limit reached: %d", m.config.MaxClients)
	}
	
	// Check per-project limit
	projectClients := len(m.registry.GetAllClientsForProject(subProjectID))
	if projectClients >= m.config.MaxClientsPerProject {
		return fmt.Errorf("maximum clients per project limit reached: %d", m.config.MaxClientsPerProject)
	}
	
	// Check per-language limit
	languageClients := 0
	allClients := m.registry.GetAllClients()
	for _, clients := range allClients {
		if _, exists := clients[language]; exists {
			languageClients++
		}
	}
	if languageClients >= m.config.MaxClientsPerLanguage {
		return fmt.Errorf("maximum clients per language limit reached: %d", m.config.MaxClientsPerLanguage)
	}
	
	// Check memory usage
	if m.config.ResourceLimits.MaxMemoryMB > 0 && 
		registryHealth.MemoryUsageMB > int64(m.config.ResourceLimits.MaxMemoryMB) {
		return fmt.Errorf("memory usage limit exceeded: %d MB", registryHealth.MemoryUsageMB)
	}
	
	return nil
}