package config

import (
	"fmt"
	"sort"
	"time"
)

// Multi-server selection strategies
const (
	SelectionStrategyPerformance = "performance"
	SelectionStrategyFeature     = "feature"
	SelectionStrategyLoadBalance = "load_balance"
	SelectionStrategyRandom      = "random"
)

// Load balancing strategies
const (
	LoadBalanceRoundRobin       = "round_robin"
	LoadBalanceLeastConnections = "least_connections"
	LoadBalanceResponseTime     = "response_time"
	LoadBalanceResourceUsage    = "resource_usage"
)

// DefaultMultiServerConfig returns default multi-server configuration
func DefaultMultiServerConfig() *MultiServerConfig {
	return &MultiServerConfig{
		SelectionStrategy:   SelectionStrategyLoadBalance,
		ConcurrentLimit:     3,
		ResourceSharing:     true,
		HealthCheckInterval: 30 * time.Second,
		MaxRetries:          3,
	}
}

// DefaultLoadBalancingConfig returns default load balancing configuration
func DefaultLoadBalancingConfig() *LoadBalancingConfig {
	return &LoadBalancingConfig{
		Strategy:        LoadBalanceRoundRobin,
		HealthThreshold: 0.8,
		WeightFactors:   make(map[string]float64),
	}
}

// DefaultResourceLimits returns default resource limits
func DefaultResourceLimits() *ResourceLimits {
	return &ResourceLimits{
		MaxMemoryMB:           1024,
		MaxConcurrentRequests: 50,
		MaxProcesses:          5,
		RequestTimeoutSeconds: 30,
	}
}

// CreateLanguageServerPool creates a new language server pool with defaults
func CreateLanguageServerPool(language string) *LanguageServerPool {
	return &LanguageServerPool{
		Language:            language,
		Servers:             make(map[string]*ServerConfig),
		LoadBalancingConfig: DefaultLoadBalancingConfig(),
		ResourceLimits:      DefaultResourceLimits(),
	}
}

// AddServerToPool adds a server to a language server pool
func (lsp *LanguageServerPool) AddServerToPool(server *ServerConfig) error {
	if server == nil {
		return fmt.Errorf("server configuration cannot be nil")
	}

	if server.Name == "" {
		return fmt.Errorf("server name cannot be empty")
	}

	// Check if server supports the pool's language
	supports := false
	for _, lang := range server.Languages {
		if lang == lsp.Language {
			supports = true
			break
		}
	}

	if !supports {
		return fmt.Errorf("server %s does not support language %s", server.Name, lsp.Language)
	}

	lsp.Servers[server.Name] = server

	// Set as default if this is the first server
	if lsp.DefaultServer == "" {
		lsp.DefaultServer = server.Name
	}

	return nil
}

// RemoveServerFromPool removes a server from a language server pool
func (lsp *LanguageServerPool) RemoveServerFromPool(serverName string) error {
	if _, exists := lsp.Servers[serverName]; !exists {
		return fmt.Errorf("server %s not found in pool", serverName)
	}

	delete(lsp.Servers, serverName)

	// Update default server if it was removed
	if lsp.DefaultServer == serverName {
		lsp.DefaultServer = ""
		// Set a new default if servers remain
		for name := range lsp.Servers {
			lsp.DefaultServer = name
			break
		}
	}

	return nil
}

// GetAvailableServers returns servers sorted by priority/weight
func (lsp *LanguageServerPool) GetAvailableServers() []*ServerConfig {
	var servers []*ServerConfig
	for _, server := range lsp.Servers {
		servers = append(servers, server)
	}

	// Sort by priority (higher priority first), then by weight (higher weight first)
	sort.Slice(servers, func(i, j int) bool {
		if servers[i].Priority != servers[j].Priority {
			return servers[i].Priority > servers[j].Priority
		}
		return servers[i].Weight > servers[j].Weight
	})

	return servers
}

// SelectServerByStrategy selects a server based on the configured strategy
func (lsp *LanguageServerPool) SelectServerByStrategy(strategy string) (*ServerConfig, error) {
	if len(lsp.Servers) == 0 {
		return nil, fmt.Errorf("no servers available in pool for language %s", lsp.Language)
	}

	switch strategy {
	case SelectionStrategyPerformance:
		return lsp.selectByPerformance()
	case SelectionStrategyFeature:
		return lsp.selectByFeature()
	case SelectionStrategyLoadBalance:
		return lsp.selectByLoadBalance()
	case SelectionStrategyRandom:
		return lsp.selectRandom()
	default:
		// Default to first available server
		for _, server := range lsp.Servers {
			return server, nil
		}
	}

	return nil, fmt.Errorf("no server could be selected for language %s", lsp.Language)
}

// selectByPerformance selects server with highest weight
func (lsp *LanguageServerPool) selectByPerformance() (*ServerConfig, error) {
	servers := lsp.GetAvailableServers()
	if len(servers) > 0 {
		return servers[0], nil // First server is highest priority/weight due to sorting
	}
	return nil, fmt.Errorf("no servers available for performance selection")
}

// selectByFeature selects server with most features (highest priority)
func (lsp *LanguageServerPool) selectByFeature() (*ServerConfig, error) {
	var bestServer *ServerConfig
	highestPriority := -1

	for _, server := range lsp.Servers {
		if server.Priority > highestPriority {
			highestPriority = server.Priority
			bestServer = server
		}
	}

	if bestServer != nil {
		return bestServer, nil
	}

	// Fallback to first available
	for _, server := range lsp.Servers {
		return server, nil
	}

	return nil, fmt.Errorf("no servers available for feature selection")
}

// selectByLoadBalance selects server based on load balancing configuration
func (lsp *LanguageServerPool) selectByLoadBalance() (*ServerConfig, error) {
	if lsp.LoadBalancingConfig == nil {
		// Fallback to default server
		if lsp.DefaultServer != "" {
			if server := lsp.Servers[lsp.DefaultServer]; server != nil {
				return server, nil
			}
		}
		// Return first available
		for _, server := range lsp.Servers {
			return server, nil
		}
		return nil, fmt.Errorf("no servers available for load balancing")
	}

	switch lsp.LoadBalancingConfig.Strategy {
	case LoadBalanceRoundRobin:
		return lsp.selectRoundRobin()
	case LoadBalanceLeastConnections:
		return lsp.selectLeastConnections()
	case LoadBalanceResponseTime:
		return lsp.selectByResponseTime()
	case LoadBalanceResourceUsage:
		return lsp.selectByResourceUsage()
	default:
		// Fallback to weighted selection
		return lsp.selectByWeight()
	}
}

// selectRandom selects a random server
func (lsp *LanguageServerPool) selectRandom() (*ServerConfig, error) {
	servers := lsp.GetAvailableServers()
	if len(servers) == 0 {
		return nil, fmt.Errorf("no servers available for random selection")
	}

	// Simple approach - return first server (in real implementation, use random)
	return servers[0], nil
}

// Helper methods for load balancing strategies
func (lsp *LanguageServerPool) selectRoundRobin() (*ServerConfig, error) {
	// Simplified round-robin: return servers in order by name
	servers := lsp.GetAvailableServers()
	if len(servers) > 0 {
		return servers[0], nil
	}
	return nil, fmt.Errorf("no servers available for round-robin selection")
}

func (lsp *LanguageServerPool) selectLeastConnections() (*ServerConfig, error) {
	// For now, select by priority/weight (in real implementation, track connections)
	return lsp.selectByPerformance()
}

func (lsp *LanguageServerPool) selectByResponseTime() (*ServerConfig, error) {
	// For now, select by priority/weight (in real implementation, track response times)
	return lsp.selectByPerformance()
}

func (lsp *LanguageServerPool) selectByResourceUsage() (*ServerConfig, error) {
	// For now, select by priority/weight (in real implementation, monitor resource usage)
	return lsp.selectByPerformance()
}

func (lsp *LanguageServerPool) selectByWeight() (*ServerConfig, error) {
	if lsp.LoadBalancingConfig == nil || len(lsp.LoadBalancingConfig.WeightFactors) == 0 {
		return lsp.selectByPerformance()
	}

	var bestServer *ServerConfig
	highestWeight := 0.0

	for name, server := range lsp.Servers {
		if weight, exists := lsp.LoadBalancingConfig.WeightFactors[name]; exists {
			if weight > highestWeight {
				highestWeight = weight
				bestServer = server
			}
		}
	}

	if bestServer != nil {
		return bestServer, nil
	}

	// Fallback to default selection
	return lsp.selectByPerformance()
}

// MigrateServersToPool converts existing servers to language pools
func (c *GatewayConfig) MigrateServersToPool() error {
	if len(c.LanguagePools) > 0 {
		// Already has pools configured
		return nil
	}

	languageMap := make(map[string]*LanguageServerPool)

	// Group servers by language
	for _, server := range c.Servers {
		for _, language := range server.Languages {
			if pool, exists := languageMap[language]; exists {
				if err := pool.AddServerToPool(&server); err != nil {
					return fmt.Errorf("failed to add server %s to existing pool for language %s: %w", server.Name, language, err)
				}
			} else {
				pool := CreateLanguageServerPool(language)
				if err := pool.AddServerToPool(&server); err != nil {
					return fmt.Errorf("failed to add server %s to new pool for language %s: %w", server.Name, language, err)
				}
				languageMap[language] = pool
			}
		}
	}

	// Convert map to slice
	for _, pool := range languageMap {
		c.LanguagePools = append(c.LanguagePools, *pool)
	}

	return nil
}

// EnsureDefaults sets default values for multi-server configuration
func (c *GatewayConfig) EnsureMultiServerDefaults() {
	if c.GlobalMultiServerConfig == nil {
		c.GlobalMultiServerConfig = DefaultMultiServerConfig()
	}

	if c.MaxConcurrentServersPerLanguage == 0 {
		c.MaxConcurrentServersPerLanguage = 3
	}

	// Ensure all pools have default configurations
	for i := range c.LanguagePools {
		pool := &c.LanguagePools[i]
		if pool.LoadBalancingConfig == nil {
			pool.LoadBalancingConfig = DefaultLoadBalancingConfig()
		}
		if pool.ResourceLimits == nil {
			pool.ResourceLimits = DefaultResourceLimits()
		}
	}
}
