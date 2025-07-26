package transport

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// PoolManagerRegistry provides centralized management of pool managers
type PoolManagerRegistry struct {
	pools map[string]PoolManager
	mu    sync.RWMutex
}

// NewPoolManagerRegistry creates a new pool manager registry
func NewPoolManagerRegistry() *PoolManagerRegistry {
	return &PoolManagerRegistry{
		pools: make(map[string]PoolManager),
	}
}

// RegisterPool registers a pool manager with a unique identifier
func (pmr *PoolManagerRegistry) RegisterPool(id string, pool PoolManager) error {
	pmr.mu.Lock()
	defer pmr.mu.Unlock()

	if _, exists := pmr.pools[id]; exists {
		return fmt.Errorf("pool with id %s already exists", id)
	}

	pmr.pools[id] = pool
	return nil
}

// GetPool retrieves a pool manager by its identifier
func (pmr *PoolManagerRegistry) GetPool(id string) (PoolManager, error) {
	pmr.mu.RLock()
	defer pmr.mu.RUnlock()

	pool, exists := pmr.pools[id]
	if !exists {
		return nil, fmt.Errorf("pool with id %s not found", id)
	}

	return pool, nil
}

// UnregisterPool removes a pool manager from the registry
func (pmr *PoolManagerRegistry) UnregisterPool(id string) error {
	pmr.mu.Lock()
	defer pmr.mu.Unlock()

	pool, exists := pmr.pools[id]
	if !exists {
		return fmt.Errorf("pool with id %s not found", id)
	}

	// Attempt to shutdown the pool gracefully
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := pool.Shutdown(ctx); err != nil {
		// Log the error but don't fail the unregistration
		log.Printf("Warning: error shutting down pool %s: %v\n", id, err)
	}

	delete(pmr.pools, id)
	return nil
}

// ListPools returns all registered pool identifiers
func (pmr *PoolManagerRegistry) ListPools() []string {
	pmr.mu.RLock()
	defer pmr.mu.RUnlock()

	ids := make([]string, 0, len(pmr.pools))
	for id := range pmr.pools {
		ids = append(ids, id)
	}
	return ids
}

// GetAllStats returns statistics for all registered pools
func (pmr *PoolManagerRegistry) GetAllStats() map[string]*PoolStats {
	pmr.mu.RLock()
	defer pmr.mu.RUnlock()

	stats := make(map[string]*PoolStats)
	for id, pool := range pmr.pools {
		stats[id] = pool.GetStats()
	}
	return stats
}

// ShutdownAll shuts down all registered pools
func (pmr *PoolManagerRegistry) ShutdownAll(ctx context.Context) error {
	pmr.mu.Lock()
	defer pmr.mu.Unlock()

	var errors []error
	for id, pool := range pmr.pools {
		if err := pool.Shutdown(ctx); err != nil {
			errors = append(errors, fmt.Errorf("failed to shutdown pool %s: %w", id, err))
		}
	}

	// Clear the registry
	pmr.pools = make(map[string]PoolManager)

	if len(errors) > 0 {
		return fmt.Errorf("multiple shutdown errors: %v", errors)
	}

	return nil
}

// EnhancedLSPClient wraps the standard LSPClient with enhanced pool management
type EnhancedLSPClient struct {
	LSPClient
	config      *EnhancedClientConfig
	poolManager PoolManager
	poolID      string
	registry    *PoolManagerRegistry
}

// NewEnhancedLSPClient creates a new LSP client with enhanced pool management
func NewEnhancedLSPClient(config *EnhancedClientConfig) (LSPClient, error) {
	if config == nil {
		return nil, fmt.Errorf("configuration cannot be nil")
	}

	// Validate and set defaults
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	if err := config.SetDefaults(); err != nil {
		return nil, fmt.Errorf("failed to set configuration defaults: %w", err)
	}

	// Create pool manager
	factory := NewEnhancedPoolFactory()
	poolManager, err := factory.CreatePool(config.Transport, config.PoolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create pool manager: %w", err)
	}

	// Create the underlying LSP client based on transport type
	basicConfig := config.ToBasicClientConfig()
	var underlyingClient LSPClient

	switch config.Transport {
	case TransportStdio:
		underlyingClient, err = NewStdioClient(*basicConfig)
	case TransportTCP:
		underlyingClient, err = NewTCPClient(*basicConfig)
	default:
		return nil, fmt.Errorf("unsupported transport type: %s", config.Transport)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create underlying client: %w", err)
	}

	// Generate a unique pool ID
	poolID := fmt.Sprintf("%s-%s-%d", config.ServerName, config.Transport, time.Now().UnixNano())

	// Register the pool manager
	registry := GetGlobalPoolRegistry()
	if err := registry.RegisterPool(poolID, poolManager); err != nil {
		return nil, fmt.Errorf("failed to register pool manager: %w", err)
	}

	enhancedClient := &EnhancedLSPClient{
		LSPClient:   underlyingClient,
		config:      config,
		poolManager: poolManager,
		poolID:      poolID,
		registry:    registry,
	}

	return enhancedClient, nil
}

// GetPoolManager returns the pool manager for the enhanced client
func (elc *EnhancedLSPClient) GetPoolManager() PoolManager {
	return elc.poolManager
}

// GetPoolStats returns current pool statistics
func (elc *EnhancedLSPClient) GetPoolStats() *PoolStats {
	return elc.poolManager.GetStats()
}

// UpdatePoolConfig updates the pool configuration dynamically
func (elc *EnhancedLSPClient) UpdatePoolConfig(newConfig *PoolConfig) error {
	if newConfig == nil {
		return fmt.Errorf("pool configuration cannot be nil")
	}

	factory := NewEnhancedPoolFactory()
	if err := factory.ValidateConfig(newConfig); err != nil {
		return fmt.Errorf("invalid pool configuration: %w", err)
	}

	return elc.poolManager.UpdateConfig(newConfig)
}

// Stop overrides the base client's Stop method to handle pool cleanup
func (elc *EnhancedLSPClient) Stop() error {
	// First stop the underlying client
	if err := elc.LSPClient.Stop(); err != nil {
		return fmt.Errorf("failed to stop underlying client: %w", err)
	}

	// Then shutdown the pool manager
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := elc.poolManager.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown pool manager: %w", err)
	}

	// Unregister from the global registry
	if err := elc.registry.UnregisterPool(elc.poolID); err != nil {
		// This is not a critical error, just log it
		log.Printf("Warning: failed to unregister pool %s: %v\n", elc.poolID, err)
	}

	return nil
}

// Global registry singleton
var (
	globalRegistry     *PoolManagerRegistry
	globalRegistryOnce sync.Once
)

// GetGlobalPoolRegistry returns the global pool manager registry
func GetGlobalPoolRegistry() *PoolManagerRegistry {
	globalRegistryOnce.Do(func() {
		globalRegistry = NewPoolManagerRegistry()
	})
	return globalRegistry
}

// Integration helper functions

// GetPoolManagerForClient retrieves the pool manager for an LSP client
func GetPoolManagerForClient(client LSPClient) (PoolManager, error) {
	if enhancedClient, ok := client.(*EnhancedLSPClient); ok {
		return enhancedClient.GetPoolManager(), nil
	}

	return nil, fmt.Errorf("client is not an enhanced LSP client")
}

// UpdateClientPoolConfig updates the pool configuration for an LSP client
func UpdateClientPoolConfig(client LSPClient, config *PoolConfig) error {
	if enhancedClient, ok := client.(*EnhancedLSPClient); ok {
		return enhancedClient.UpdatePoolConfig(config)
	}

	return fmt.Errorf("client is not an enhanced LSP client")
}

// GetClientPoolStats returns pool statistics for an LSP client
func GetClientPoolStats(client LSPClient) (*PoolStats, error) {
	if enhancedClient, ok := client.(*EnhancedLSPClient); ok {
		return enhancedClient.GetPoolStats(), nil
	}

	return nil, fmt.Errorf("client is not an enhanced LSP client")
}

// CreateClientFromBasicConfig creates an enhanced client from a basic configuration
func CreateClientFromBasicConfig(basicConfig *ClientConfig) (LSPClient, error) {
	enhancedConfig := FromBasicClientConfig(basicConfig)
	return NewEnhancedLSPClient(enhancedConfig)
}

// MigrateToEnhancedClient migrates an existing basic client to an enhanced client
func MigrateToEnhancedClient(basicClient LSPClient, poolConfig *PoolConfig) (LSPClient, error) {
	// This is a conceptual function - in practice, migration would be more complex
	// and would depend on the specific client implementation

	// Extract configuration from the basic client
	// This would need to be implemented based on the actual client interface
	config := &EnhancedClientConfig{
		Transport: TransportStdio, // Default, would need to be determined from the client
	}

	if poolConfig != nil {
		config.PoolConfig = poolConfig
	}

	// Stop the old client
	if err := basicClient.Stop(); err != nil {
		return nil, fmt.Errorf("failed to stop basic client: %w", err)
	}

	// Create the new enhanced client
	return NewEnhancedLSPClient(config)
}

// Utility functions for configuration management

// LoadEnhancedConfigFromYAML loads enhanced client configuration from YAML
func LoadEnhancedConfigFromYAML(yamlData []byte) (*EnhancedClientConfig, error) {
	var config EnhancedClientConfig
	if err := yaml.Unmarshal(yamlData, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal enhanced config: %w", err)
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	if err := config.SetDefaults(); err != nil {
		return nil, fmt.Errorf("failed to set defaults: %w", err)
	}

	return &config, nil
}

// CreatePoolConfigTemplate creates a template pool configuration for a given transport
func CreatePoolConfigTemplate(transport string) *PoolConfig {
	factory := NewEnhancedPoolFactory()
	return factory.GetDefaultConfig(transport)
}

// ValidateTransportSupport checks if a transport type is supported
func ValidateTransportSupport(transport string) error {
	factory := NewEnhancedPoolFactory()
	supportedTransports := factory.GetSupportedTransports()

	for _, supported := range supportedTransports {
		if transport == supported {
			return nil
		}
	}

	return fmt.Errorf("unsupported transport type: %s, supported types: %v", transport, supportedTransports)
}

// GetTransportCapabilities returns the capabilities and constraints for a transport type
func GetTransportCapabilities(transport string) (map[string]interface{}, error) {
	if err := ValidateTransportSupport(transport); err != nil {
		return nil, err
	}

	capabilities := make(map[string]interface{})

	switch transport {
	case TransportTCP:
		capabilities["pooling"] = true
		capabilities["connection_reuse"] = true
		capabilities["health_checks"] = true
		capabilities["circuit_breaker"] = true
		capabilities["max_recommended_pool_size"] = 20
		capabilities["supports_tls"] = true
		capabilities["supports_keep_alive"] = true

	case TransportStdio:
		capabilities["pooling"] = true
		capabilities["connection_reuse"] = false // Process-based, limited reuse
		capabilities["health_checks"] = true
		capabilities["circuit_breaker"] = true
		capabilities["max_recommended_pool_size"] = 5
		capabilities["supports_tls"] = false
		capabilities["supports_keep_alive"] = false
		capabilities["process_isolation"] = true

	case TransportHTTP:
		capabilities["pooling"] = true
		capabilities["connection_reuse"] = true
		capabilities["health_checks"] = true
		capabilities["circuit_breaker"] = true
		capabilities["max_recommended_pool_size"] = 50
		capabilities["supports_tls"] = true
		capabilities["supports_keep_alive"] = true
		capabilities["stateless"] = true

	}

	return capabilities, nil
}

// MonitorPoolHealth continuously monitors the health of all registered pools
func MonitorPoolHealth(ctx context.Context, interval time.Duration) error {
	registry := GetGlobalPoolRegistry()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			stats := registry.GetAllStats()
			for poolID, stat := range stats {
				if !stat.IsHealthy {
					log.Printf("Warning: Pool %s is unhealthy - Error rate: %.2f%%, Unhealthy connections: %d\n",
						poolID, stat.ErrorRate*100, stat.UnhealthyConnections)

					// Attempt to get the pool and run health check
					if pool, err := registry.GetPool(poolID); err == nil {
						if err := pool.HealthCheck(); err != nil {
							log.Printf("Health check failed for pool %s: %v\n", poolID, err)
						}
					}
				}
			}
		}
	}
}

// GetSystemStats returns comprehensive statistics for all pools and the system
func GetSystemStats() map[string]interface{} {
	registry := GetGlobalPoolRegistry()
	allStats := registry.GetAllStats()

	systemStats := map[string]interface{}{
		"total_pools":              len(allStats),
		"healthy_pools":            0,
		"unhealthy_pools":          0,
		"total_connections":        0,
		"total_active_connections": 0,
		"total_requests":           int64(0),
		"average_error_rate":       0.0,
		"pools":                    allStats,
	}

	var totalErrorRate float64
	healthyPools := 0

	for _, stats := range allStats {
		if stats.IsHealthy {
			healthyPools++
		}
		systemStats["total_connections"] = systemStats["total_connections"].(int) + stats.TotalConnections
		systemStats["total_active_connections"] = systemStats["total_active_connections"].(int) + stats.ActiveConnections
		totalErrorRate += stats.ErrorRate
	}

	systemStats["healthy_pools"] = healthyPools
	systemStats["unhealthy_pools"] = len(allStats) - healthyPools

	if len(allStats) > 0 {
		systemStats["average_error_rate"] = totalErrorRate / float64(len(allStats))
	}

	return systemStats
}

// CleanupOrphanedPools removes pools that are no longer active
func CleanupOrphanedPools(ctx context.Context) error {
	registry := GetGlobalPoolRegistry()
	poolIDs := registry.ListPools()

	var errors []error
	for _, poolID := range poolIDs {
		pool, err := registry.GetPool(poolID)
		if err != nil {
			continue
		}

		// Run health check to determine if pool is still viable
		if err := pool.HealthCheck(); err != nil {
			log.Printf("Removing orphaned pool %s due to health check failure: %v\n", poolID, err)
			if err := registry.UnregisterPool(poolID); err != nil {
				errors = append(errors, fmt.Errorf("failed to unregister pool %s: %w", poolID, err))
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("multiple cleanup errors: %v", errors)
	}

	return nil
}
