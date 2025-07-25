package config

import (
	"testing"
	"time"
)

func TestMultiServerConfig(t *testing.T) {
	// Test multi-server configuration creation and validation
	config := &GatewayConfig{
		Port:                            8080,
		EnableConcurrentServers:         true,
		MaxConcurrentServersPerLanguage: 3,
		GlobalMultiServerConfig: &MultiServerConfig{
			SelectionStrategy:   SelectionStrategyLoadBalance,
			ConcurrentLimit:     3,
			ResourceSharing:     true,
			HealthCheckInterval: 30 * time.Second,
			MaxRetries:          3,
		},
		LanguagePools: []LanguageServerPool{
			{
				Language:      "go",
				DefaultServer: "gopls-primary",
				Servers: map[string]*ServerConfig{
					"gopls-primary": {
						Name:                  "gopls-primary",
						Languages:             []string{"go"},
						Command:               "gopls",
						Transport:             "stdio",
						Priority:              2,
						Weight:                2.0,
						MaxConcurrentRequests: 100,
					},
					"gopls-secondary": {
						Name:                  "gopls-secondary",
						Languages:             []string{"go"},
						Command:               "gopls",
						Args:                  []string{"--mode=lightweight"},
						Transport:             "stdio",
						Priority:              1,
						Weight:                1.0,
						MaxConcurrentRequests: 50,
					},
				},
				LoadBalancingConfig: &LoadBalancingConfig{
					Strategy:        LoadBalanceRoundRobin,
					HealthThreshold: 0.8,
					WeightFactors: map[string]float64{
						"gopls-primary":   2.0,
						"gopls-secondary": 1.0,
					},
				},
				ResourceLimits: &ResourceLimits{
					MaxMemoryMB:           2048,
					MaxConcurrentRequests: 200,
					MaxProcesses:          10,
					RequestTimeoutSeconds: 60,
				},
			},
		},
	}

	// Test validation
	if err := config.ValidateMultiServerConfig(); err != nil {
		t.Errorf("Multi-server config validation failed: %v", err)
	}

	// Test GetServerPoolByLanguage
	pool, err := config.GetServerPoolByLanguage("go")
	if err != nil {
		t.Errorf("Failed to get server pool for Go: %v", err)
	}
	if pool.Language != "go" {
		t.Errorf("Expected language 'go', got '%s'", pool.Language)
	}

	// Test GetServersForLanguage
	servers, err := config.GetServersForLanguage("go", 2)
	if err != nil {
		t.Errorf("Failed to get servers for Go: %v", err)
	}
	if len(servers) != 2 {
		t.Errorf("Expected 2 servers, got %d", len(servers))
	}

	// Test IsMultiServerEnabled
	if !config.IsMultiServerEnabled("go") {
		t.Error("Expected multi-server to be enabled for Go")
	}

	// Test GetResourceLimits
	limits := config.GetResourceLimits("go")
	if limits == nil {
		t.Error("Expected resource limits, got nil")
	}
	if limits.MaxMemoryMB != 2048 {
		t.Errorf("Expected max memory 2048, got %d", limits.MaxMemoryMB)
	}
}

func TestLanguageServerPool(t *testing.T) {
	pool := CreateLanguageServerPool("python")

	// Test adding servers
	server1 := &ServerConfig{
		Name:      "pylsp-1",
		Languages: []string{"python"},
		Command:   "pylsp",
		Transport: "stdio",
		Priority:  2,
		Weight:    2.0,
	}

	server2 := &ServerConfig{
		Name:      "pylsp-2",
		Languages: []string{"python"},
		Command:   "pylsp",
		Transport: "stdio",
		Priority:  1,
		Weight:    1.0,
	}

	if err := pool.AddServerToPool(server1); err != nil {
		t.Errorf("Failed to add server1: %v", err)
	}

	if err := pool.AddServerToPool(server2); err != nil {
		t.Errorf("Failed to add server2: %v", err)
	}

	// Test server selection
	selected, err := pool.SelectServerByStrategy(SelectionStrategyPerformance)
	if err != nil {
		t.Errorf("Failed to select server by performance: %v", err)
	}
	if selected.Name != "pylsp-1" {
		t.Errorf("Expected pylsp-1 (higher priority), got %s", selected.Name)
	}

	// Test removing server
	if err := pool.RemoveServerFromPool("pylsp-2"); err != nil {
		t.Errorf("Failed to remove server: %v", err)
	}

	if len(pool.Servers) != 1 {
		t.Errorf("Expected 1 server after removal, got %d", len(pool.Servers))
	}
}

func TestBackwardCompatibility(t *testing.T) {
	// Test that existing single-server configurations still work
	config := &GatewayConfig{
		Port: 8080,
		Servers: []ServerConfig{
			{
				Name:      "go-lsp",
				Languages: []string{"go"},
				Command:   "gopls",
				Transport: "stdio",
				Priority:  1,
				Weight:    1.0,
			},
		},
		EnableConcurrentServers: false,
	}

	// Ensure defaults are applied
	config.EnsureMultiServerDefaults()

	// Test that GetServerByLanguage still works
	server, err := config.GetServerByLanguage("go")
	if err != nil {
		t.Errorf("Backward compatibility failed: %v", err)
	}
	if server.Name != "go-lsp" {
		t.Errorf("Expected 'go-lsp', got '%s'", server.Name)
	}

	// Test that multi-server is not enabled
	if config.IsMultiServerEnabled("go") {
		t.Error("Multi-server should not be enabled for backward compatibility")
	}
}

func TestValidation(t *testing.T) {
	// Test invalid selection strategy
	config := &MultiServerConfig{
		SelectionStrategy: "invalid_strategy",
	}

	if err := config.Validate(); err == nil {
		t.Error("Expected validation error for invalid selection strategy")
	}

	// Test invalid resource limits
	limits := &ResourceLimits{
		MaxMemoryMB: -100,
	}

	if err := limits.Validate(); err == nil {
		t.Error("Expected validation error for negative memory limit")
	}

	// Test invalid load balancing config
	lbConfig := &LoadBalancingConfig{
		Strategy:        "invalid_strategy",
		HealthThreshold: 1.5, // Invalid threshold > 1.0
	}

	if err := lbConfig.Validate(); err == nil {
		t.Error("Expected validation error for invalid load balancing config")
	}
}

func TestMigration(t *testing.T) {
	// Test migrating existing servers to pools
	config := &GatewayConfig{
		Servers: []ServerConfig{
			{
				Name:      "go-server-1",
				Languages: []string{"go"},
				Command:   "gopls",
				Transport: "stdio",
			},
			{
				Name:      "go-server-2",
				Languages: []string{"go"},
				Command:   "gopls",
				Transport: "stdio",
			},
			{
				Name:      "python-server",
				Languages: []string{"python"},
				Command:   "pylsp",
				Transport: "stdio",
			},
		},
	}

	if err := config.MigrateServersToPool(); err != nil {
		t.Errorf("Migration failed: %v", err)
	}

	// Should have 2 language pools (go and python)
	if len(config.LanguagePools) != 2 {
		t.Errorf("Expected 2 language pools, got %d", len(config.LanguagePools))
	}

	// Go pool should have 2 servers
	goPool, err := config.GetServerPoolByLanguage("go")
	if err != nil {
		t.Errorf("Failed to get Go pool: %v", err)
	}
	if len(goPool.Servers) != 2 {
		t.Errorf("Expected 2 servers in Go pool, got %d", len(goPool.Servers))
	}

	// Python pool should have 1 server
	pythonPool, err := config.GetServerPoolByLanguage("python")
	if err != nil {
		t.Errorf("Failed to get Python pool: %v", err)
	}
	if len(pythonPool.Servers) != 1 {
		t.Errorf("Expected 1 server in Python pool, got %d", len(pythonPool.Servers))
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	// Test that defaults are properly set
	if config.MaxConcurrentServersPerLanguage != DEFAULT_MAX_CONCURRENT_SERVERS_PER_LANG {
		t.Errorf("Expected max concurrent servers per language %d, got %d",
			DEFAULT_MAX_CONCURRENT_SERVERS_PER_LANG, config.MaxConcurrentServersPerLanguage)
	}

	if config.GlobalMultiServerConfig == nil {
		t.Error("Expected global multi-server config to be set")
	}

	if config.GlobalMultiServerConfig.SelectionStrategy != SelectionStrategyLoadBalance {
		t.Errorf("Expected default selection strategy %s, got %s",
			SelectionStrategyLoadBalance, config.GlobalMultiServerConfig.SelectionStrategy)
	}

	// Test validation passes
	if err := config.Validate(); err != nil {
		t.Errorf("Default config validation failed: %v", err)
	}
}
