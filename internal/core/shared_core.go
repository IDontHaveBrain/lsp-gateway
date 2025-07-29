package core

import (
	"context"
	"fmt"
	"sync"
	"time"

	"lsp-gateway/internal/setup"
)

// DefaultSharedCore is the default implementation of SharedCore
type DefaultSharedCore struct {
	// Component references
	configMgr   ConfigManager
	registry    LSPServerRegistry  
	cache       SCIPCache
	projectMgr  ProjectManager
	
	// Configuration
	config      *SharedCoreConfig
	
	// State management
	mu          sync.RWMutex
	initialized bool
	started     bool
	
	// Monitoring
	healthStatus *HealthStatus
	metrics      *Metrics
	
	// Lifecycle management
	logger      *setup.SetupLogger
	stopCh      chan struct{}
	stopOnce    sync.Once
}

// SharedCoreConfig contains configuration for the shared core
type SharedCoreConfig struct {
	// Component settings
	EnableHealthChecks   bool          `json:"enable_health_checks"`
	HealthCheckInterval  time.Duration `json:"health_check_interval"`
	MetricsInterval      time.Duration `json:"metrics_interval"`
	
	// Resource management
	MaxMemoryUsage       int64         `json:"max_memory_usage"`
	MemoryCheckInterval  time.Duration `json:"memory_check_interval"`
	
	// Cleanup settings
	CleanupInterval      time.Duration `json:"cleanup_interval"`
	MaxIdleTime          time.Duration `json:"max_idle_time"`
	
	// Circuit breaker settings
	EnableCircuitBreaker bool          `json:"enable_circuit_breaker"`
	FailureThreshold     int           `json:"failure_threshold"`
	RecoveryTimeout      time.Duration `json:"recovery_timeout"`
	
	// Feature flags
	Features             map[string]bool `json:"features"`
}

// NewDefaultSharedCore creates a new instance of DefaultSharedCore
func NewDefaultSharedCore(
	configMgr ConfigManager,
	registry LSPServerRegistry,
	cache SCIPCache,
	projectMgr ProjectManager,
	configData interface{},
) (SharedCore, error) {
	
	config := defaultSharedCoreConfig()
	if configData != nil {
		// TODO: Unmarshal configData into config
	}
	
	core := &DefaultSharedCore{
		configMgr:    configMgr,
		registry:     registry,
		cache:        cache,
		projectMgr:   projectMgr,
		config:       config,
		logger:       setup.NewSetupLogger(nil),
		stopCh:       make(chan struct{}),
		healthStatus: &HealthStatus{
			Overall:    "unknown",
			Components: make(map[string]ComponentHealth),
			Timestamp:  time.Now(),
		},
		metrics: &Metrics{
			Timestamp: time.Now(),
		},
	}
	
	return core, nil
}

// Initialize initializes the shared core and all components
func (c *DefaultSharedCore) Initialize(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.initialized {
		return fmt.Errorf("shared core already initialized")
	}
	
	c.logger.Info("Initializing shared core")
	
	// Initialize components in dependency order
	if err := c.initializeComponents(ctx); err != nil {
		return fmt.Errorf("failed to initialize components: %w", err)
	}
	
	// Set up monitoring if enabled
	if c.config.EnableHealthChecks {
		c.startHealthChecking()
	}
	
	// Set up metrics collection
	c.startMetricsCollection()
	
	// Set up periodic cleanup
	c.startPeriodicCleanup()
	
	c.initialized = true
	c.logger.Info("Shared core initialized successfully")
	
	return nil
}

// Start starts the shared core
func (c *DefaultSharedCore) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if !c.initialized {
		return fmt.Errorf("shared core not initialized")
	}
	
	if c.started {
		return fmt.Errorf("shared core already started")
	}
	
	c.logger.Info("Starting shared core")
	
	// Start background processes
	go c.monitoringLoop()
	
	c.started = true
	c.logger.Info("Shared core started successfully")
	
	return nil
}

// Stop stops the shared core and all components
func (c *DefaultSharedCore) Stop() error {
	var err error
	
	c.stopOnce.Do(func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		
		if !c.started {
			return
		}
		
		c.logger.Info("Stopping shared core")
		
		// Signal stop to background processes
		close(c.stopCh)
		
		// Stop components in reverse order
		var errors []error
		
		// Stop registry first to prevent new connections
		if c.registry != nil {
			if stopErr := c.stopRegistry(); stopErr != nil {
				errors = append(errors, fmt.Errorf("failed to stop registry: %w", stopErr))
			}
		}
		
		// Stop cache
		if c.cache != nil {
			if stopErr := c.stopCache(); stopErr != nil {
				errors = append(errors, fmt.Errorf("failed to stop cache: %w", stopErr))
			}
		}
		
		// Stop project manager
		if c.projectMgr != nil {
			if stopErr := c.stopProjectManager(); stopErr != nil {
				errors = append(errors, fmt.Errorf("failed to stop project manager: %w", stopErr))
			}
		}
		
		c.started = false
		
		if len(errors) > 0 {
			err = fmt.Errorf("shutdown errors: %v", errors)
		} else {
			c.logger.Info("Shared core stopped successfully")
		}
	})
	
	return err
}

// Component access methods

func (c *DefaultSharedCore) Config() ConfigManager {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.configMgr
}

func (c *DefaultSharedCore) Registry() LSPServerRegistry {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.registry
}

func (c *DefaultSharedCore) Cache() SCIPCache {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cache
}

func (c *DefaultSharedCore) Projects() ProjectManager {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.projectMgr
}

// Health and metrics

func (c *DefaultSharedCore) Health() HealthStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	// Create a copy to avoid concurrent modification
	health := HealthStatus{
		Overall:    c.healthStatus.Overall,
		Components: make(map[string]ComponentHealth),
		Timestamp:  c.healthStatus.Timestamp,
	}
	
	for name, status := range c.healthStatus.Components {
		health.Components[name] = status
	}
	
	return health
}

func (c *DefaultSharedCore) Metrics() Metrics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	// Create a copy to avoid concurrent modification
	return *c.metrics
}

// Private helper methods

func (c *DefaultSharedCore) initializeComponents(ctx context.Context) error {
	// Components are already initialized by the container
	// This method can be used for additional initialization logic
	
	c.logger.Debug("All components initialized successfully")
	return nil
}

func (c *DefaultSharedCore) startHealthChecking() {
	c.logger.Debug("Starting health checking")
	// Health checking will be handled in the monitoring loop
}

func (c *DefaultSharedCore) startMetricsCollection() {
	c.logger.Debug("Starting metrics collection")
	// Metrics collection will be handled in the monitoring loop
}

func (c *DefaultSharedCore) startPeriodicCleanup() {
	c.logger.Debug("Starting periodic cleanup")
	// Cleanup will be handled in the monitoring loop
}

func (c *DefaultSharedCore) monitoringLoop() {
	healthTicker := time.NewTicker(c.config.HealthCheckInterval)
	metricsTicker := time.NewTicker(c.config.MetricsInterval)
	cleanupTicker := time.NewTicker(c.config.CleanupInterval)
	memoryTicker := time.NewTicker(c.config.MemoryCheckInterval)
	
	defer func() {
		healthTicker.Stop()
		metricsTicker.Stop()
		cleanupTicker.Stop()
		memoryTicker.Stop()
	}()
	
	for {
		select {
		case <-c.stopCh:
			c.logger.Debug("Monitoring loop stopped")
			return
			
		case <-healthTicker.C:
			if c.config.EnableHealthChecks {
				c.performHealthCheck()
			}
			
		case <-metricsTicker.C:
			c.collectMetrics()
			
		case <-cleanupTicker.C:
			c.performCleanup()
			
		case <-memoryTicker.C:
			c.checkMemoryUsage()
		}
	}
}

func (c *DefaultSharedCore) performHealthCheck() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	now := time.Now()
	components := make(map[string]ComponentHealth)
	overallHealthy := true
	
	// Check config manager
	components["config"] = c.checkConfigManagerHealth()
	if components["config"].Status != "healthy" {
		overallHealthy = false
	}
	
	// Check registry
	components["registry"] = c.checkRegistryHealth()
	if components["registry"].Status != "healthy" {
		overallHealthy = false
	}
	
	// Check cache
	components["cache"] = c.checkCacheHealth()
	if components["cache"].Status != "healthy" {
		overallHealthy = false
	}
	
	// Check project manager
	components["projects"] = c.checkProjectManagerHealth()
	if components["projects"].Status != "healthy" {
		overallHealthy = false
	}
	
	// Update overall status
	overall := "healthy"
	if !overallHealthy {
		overall = "degraded"
	}
	
	c.healthStatus = &HealthStatus{
		Overall:    overall,
		Components: components,
		Timestamp:  now,
	}
	
	if !overallHealthy {
		c.logger.WithField("health", c.healthStatus).Warn("System health degraded")
	}
}

func (c *DefaultSharedCore) checkConfigManagerHealth() ComponentHealth {
	// Basic health check - try to get global config
	_, err := c.configMgr.GetGlobalConfig()
	if err != nil {
		return ComponentHealth{
			Status:    "unhealthy",
			Message:   fmt.Sprintf("Config manager error: %v", err),
			LastCheck: time.Now(),
		}
	}
	
	return ComponentHealth{
		Status:    "healthy",
		LastCheck: time.Now(),
	}
}

func (c *DefaultSharedCore) checkRegistryHealth() ComponentHealth {
	// Check if registry is responding
	servers := c.registry.GetAllServers()
	
	return ComponentHealth{
		Status:    "healthy",
		Message:   fmt.Sprintf("Managing %d server instances", len(servers)),
		LastCheck: time.Now(),
	}
}

func (c *DefaultSharedCore) checkCacheHealth() ComponentHealth {
	// Check cache statistics
	stats := c.cache.GetCacheStats()
	
	status := "healthy"
	message := fmt.Sprintf("Hit rate: %.2f%%, Memory: %d entries", 
		stats.HitRate, stats.MemoryEntries)
	
	// Check if hit rate is too low
	if stats.HitRate < 0.5 && stats.MemoryHits+stats.MemoryMisses > 100 {
		status = "degraded"
		message += " (low hit rate)"
	}
	
	return ComponentHealth{
		Status:    status,
		Message:   message,
		LastCheck: time.Now(),
	}
}

func (c *DefaultSharedCore) checkProjectManagerHealth() ComponentHealth {
	// Check number of active workspaces
	workspaces := c.projectMgr.GetAllWorkspaces()
	
	return ComponentHealth{
		Status:    "healthy",
		Message:   fmt.Sprintf("Managing %d workspaces", len(workspaces)),
		LastCheck: time.Now(),
	}
}

func (c *DefaultSharedCore) collectMetrics() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	now := time.Now()
	
	// Collect server metrics
	allServers := c.registry.GetAllServers()
	serverCount := 0
	activeServers := 0
	
	for _, workspaceServers := range allServers {
		for range workspaceServers {
			serverCount++
			// Check if server is healthy (simplified)
			activeServers++
		}
	}
	
	// Collect cache metrics
	cacheStats := c.cache.GetCacheStats()
	
	// Collect workspace metrics
	workspaces := c.projectMgr.GetAllWorkspaces()
	workspaceCount := len(workspaces)
	activeWorkspaces := 0
	
	for _, workspace := range workspaces {
		if workspace.IsInitialized {
			activeWorkspaces++
		}
	}
	
	// Update metrics
	c.metrics = &Metrics{
		ServerCount:       serverCount,
		ActiveServers:     activeServers,
		FailedServers:     serverCount - activeServers,
		
		CacheHitRate:      cacheStats.HitRate,
		CacheSize:         cacheStats.MemorySize + cacheStats.DiskSize,
		CacheEvictions:    cacheStats.Evictions,
		
		WorkspaceCount:    workspaceCount,
		ActiveWorkspaces:  activeWorkspaces,
		
		// TODO: Collect actual performance metrics
		AvgResponseTime:   100 * time.Millisecond,
		RequestCount:      0,
		ErrorCount:        0,
		
		Timestamp:         now,
	}
}

func (c *DefaultSharedCore) performCleanup() {
	c.logger.Debug("Performing periodic cleanup")
	
	// Cleanup idle workspaces
	workspaces := c.projectMgr.GetAllWorkspaces()
	now := time.Now()
	
	for workspaceID, workspace := range workspaces {
		if now.Sub(workspace.LastAccessedAt) > c.config.MaxIdleTime {
			c.logger.WithField("workspace_id", workspaceID).Debug("Cleaning up idle workspace")
			
			// Remove idle workspace
			if err := c.projectMgr.RemoveWorkspace(workspaceID); err != nil {
				c.logger.WithError(err).WithField("workspace_id", workspaceID).
					Error("Failed to cleanup idle workspace")
			}
		}
	}
	
	// Cleanup cache if needed
	if err := c.cache.Compact(); err != nil {
		c.logger.WithError(err).Error("Failed to compact cache")
	}
}

func (c *DefaultSharedCore) checkMemoryUsage() {
	memoryUsage := c.cache.GetMemoryUsage()
	
	if memoryUsage > c.config.MaxMemoryUsage {
		c.logger.WithField("memory_usage", memoryUsage).
			WithField("max_memory", c.config.MaxMemoryUsage).
			Warn("Memory usage exceeds limit, forcing cache compaction")
		
		if err := c.cache.Compact(); err != nil {
			c.logger.WithError(err).Error("Failed to compact cache during memory pressure")
		}
	}
}

func (c *DefaultSharedCore) stopRegistry() error {
	// Stop all servers in the registry
	allServers := c.registry.GetAllServers()
	var errors []error
	
	for workspaceID := range allServers {
		if err := c.registry.RemoveWorkspace(workspaceID); err != nil {
			errors = append(errors, err)
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("registry shutdown errors: %v", errors)
	}
	
	return nil
}

func (c *DefaultSharedCore) stopCache() error {
	// Ensure cache is properly flushed
	if err := c.cache.Compact(); err != nil {
		return fmt.Errorf("failed to compact cache during shutdown: %w", err)
	}
	
	return nil
}

func (c *DefaultSharedCore) stopProjectManager() error {
	// Stop all workspace watchers
	workspaces := c.projectMgr.GetAllWorkspaces()
	var errors []error
	
	for workspaceID := range workspaces {
		if err := c.projectMgr.StopWatching(workspaceID); err != nil {
			errors = append(errors, err)
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("project manager shutdown errors: %v", errors)
	}
	
	return nil
}

func defaultSharedCoreConfig() *SharedCoreConfig {
	return &SharedCoreConfig{
		EnableHealthChecks:   true,
		HealthCheckInterval:  30 * time.Second,
		MetricsInterval:      10 * time.Second,
		
		MaxMemoryUsage:       512 * 1024 * 1024, // 512MB
		MemoryCheckInterval:  60 * time.Second,
		
		CleanupInterval:      5 * time.Minute,
		MaxIdleTime:          30 * time.Minute,
		
		EnableCircuitBreaker: true,
		FailureThreshold:     5,
		RecoveryTimeout:      60 * time.Second,
		
		Features:             make(map[string]bool),
	}
}