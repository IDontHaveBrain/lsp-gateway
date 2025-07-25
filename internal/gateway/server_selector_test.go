package gateway

import (
	"fmt"
	"testing"
	"time"

	"lsp-gateway/internal/config"
)

// createTestPool creates a test language server pool with mock servers
func createTestPool(t *testing.T, strategy string) *LanguageServerPool {
	lbConfig := &config.LoadBalancingConfig{
		Strategy:        strategy,
		HealthThreshold: 0.8,
	}

	pool := NewLanguageServerPoolWithConfig("go", nil, lbConfig, nil)

	// Add mock servers
	for i := 1; i <= 3; i++ {
		serverConfig := &config.ServerConfig{
			Name:     fmt.Sprintf("server%d", i),
			Languages: []string{"go"},
			Command:  "gopls",
		}
		
		server := &ServerInstance{
			config:    serverConfig,
			state:     ServerStateHealthy,
			startTime: time.Now(),
			metrics:   NewServerMetrics(),
		}

		err := pool.AddServer(server)
		if err != nil {
			t.Fatalf("Failed to add server to pool: %v", err)
		}
	}

	return pool
}

func TestRoundRobinSelector(t *testing.T) {
	pool := createTestPool(t, "round_robin")
	selector := NewRoundRobinSelector()

	context := &SelectionRequestContext{
		Method:   "textDocument/definition",
		Priority: PriorityNormal,
		Timeout:  30 * time.Second,
	}

	// Test multiple selections to verify round-robin behavior
	selectedServers := make(map[string]int)
	for i := 0; i < 9; i++ {
		server, err := selector.SelectServer(pool, "textDocument/definition", context)
		if err != nil {
			t.Fatalf("Failed to select server: %v", err)
		}
		selectedServers[server.config.Name]++
	}

	// Each server should be selected 3 times (9 requests / 3 servers)
	for serverName, count := range selectedServers {
		if count != 3 {
			t.Errorf("Server %s was selected %d times, expected 3", serverName, count)
		}
	}
}

func TestLeastConnectionsSelector(t *testing.T) {
	pool := createTestPool(t, "least_connections")
	selector := NewLeastConnectionsSelector()

	context := &SelectionRequestContext{
		Method:   "textDocument/definition",
		Priority: PriorityNormal,
		Timeout:  30 * time.Second,
	}

	// First selection should pick any server
	server1, err := selector.SelectServer(pool, "textDocument/definition", context)
	if err != nil {
		t.Fatalf("Failed to select server: %v", err)
	}

	// Second selection should pick a different server (since first has 1 connection)
	server2, err := selector.SelectServer(pool, "textDocument/definition", context)
	if err != nil {
		t.Fatalf("Failed to select server: %v", err)
	}

	if server1.config.Name == server2.config.Name {
		t.Errorf("Expected different servers, got same server: %s", server1.config.Name)
	}

	// Simulate request completion for server1
	selector.UpdateServerMetrics(server1.config.Name, 100*time.Millisecond, true)

	// Next selection should prefer server1 again (now has 0 connections)
	server3, err := selector.SelectServer(pool, "textDocument/definition", context)
	if err != nil {
		t.Fatalf("Failed to select server: %v", err)
	}

	if server3.config.Name != server1.config.Name {
		t.Errorf("Expected server %s (least connections), got server %s", server1.config.Name, server3.config.Name)
	}
}

func TestResponseTimeSelector(t *testing.T) {
	pool := createTestPool(t, "response_time")
	selector := NewResponseTimeSelector()

	context := &SelectionRequestContext{
		Method:   "textDocument/definition",
		Priority: PriorityNormal,
		Timeout:  30 * time.Second,
	}

	// Update metrics for servers with different response times
	selector.UpdateServerMetrics("server1", 100*time.Millisecond, true)
	selector.UpdateServerMetrics("server2", 200*time.Millisecond, true)
	selector.UpdateServerMetrics("server3", 50*time.Millisecond, true)

	// Should select server3 (fastest)
	server, err := selector.SelectServer(pool, "textDocument/definition", context)
	if err != nil {
		t.Fatalf("Failed to select server: %v", err)
	}

	if server.config.Name != "server3" {
		t.Errorf("Expected server3 (fastest), got server %s", server.config.Name)
	}
}

func TestPerformanceBasedSelector(t *testing.T) {
	weights := &WeightFactors{
		ResponseTimeWeight: 0.5,
		AvailabilityWeight: 0.3,
		ResourceWeight:     0.2,
	}

	selector := NewPerformanceBasedSelector(weights)
	pool := createTestPool(t, "performance")

	context := &SelectionRequestContext{
		Method:   "textDocument/definition",
		Priority: PriorityNormal,
		Timeout:  30 * time.Second,
	}

	// Set up server metrics for performance scoring
	for _, server := range pool.GetHealthyServers() {
		server.metrics.UpdateMetrics(100*time.Millisecond, true)
		server.metrics.healthScore = 0.9
	}

	server, err := selector.SelectServer(pool, "textDocument/definition", context)
	if err != nil {
		t.Fatalf("Failed to select server: %v", err)
	}

	if server == nil {
		t.Errorf("Expected a server to be selected")
	}
}

func TestFeatureBasedSelector(t *testing.T) {
	selector := NewFeatureBasedSelector()
	pool := createTestPool(t, "feature")

	context := &SelectionRequestContext{
		Method:           "textDocument/definition",
		Priority:         PriorityNormal,
		RequiredFeatures: []string{"basic"},
		Timeout:          30 * time.Second,
	}

	server, err := selector.SelectServer(pool, "textDocument/definition", context)
	if err != nil {
		t.Fatalf("Failed to select server: %v", err)
	}

	if server == nil {
		t.Errorf("Expected a server to be selected")
	}
}

func TestMultiServerSelector(t *testing.T) {
	pool := createTestPool(t, "multi_strategy")

	primary := NewRoundRobinSelector()
	fallbacks := []ServerSelector{
		NewLeastConnectionsSelector(),
		NewResponseTimeSelector(),
	}

	selector := NewMultiServerSelector(primary, fallbacks)

	context := &SelectionRequestContext{
		Method:   "textDocument/definition",
		Priority: PriorityNormal,
		Timeout:  30 * time.Second,
	}

	// Test single server selection
	server, err := selector.SelectServer(pool, "textDocument/definition", context)
	if err != nil {
		t.Fatalf("Failed to select server: %v", err)
	}

	if server == nil {
		t.Errorf("Expected a server to be selected")
	}

	// Test multiple server selection
	servers, err := selector.SelectMultipleServers(pool, "textDocument/definition", 2)
	if err != nil {
		t.Fatalf("Failed to select multiple servers: %v", err)
	}

	if len(servers) != 2 {
		t.Errorf("Expected 2 servers, got %d", len(servers))
	}

	// Ensure no duplicates
	if servers[0].config.Name == servers[1].config.Name {
		t.Errorf("Expected different servers, got duplicates: %s", servers[0].config.Name)
	}
}

func TestHealthAwareSelector(t *testing.T) {
	baseSelector := NewRoundRobinSelector()
	selector := NewHealthAwareSelector(baseSelector, 0.8, 0.3)
	pool := createTestPool(t, "health_aware")

	// Set one server as unhealthy
	servers := pool.GetHealthyServers()
	if len(servers) > 0 {
		servers[0].metrics.healthScore = 0.5 // Below threshold
	}

	context := &SelectionRequestContext{
		Method:   "textDocument/definition",
		Priority: PriorityNormal,
		Timeout:  30 * time.Second,
	}

	// Should select a healthy server
	server, err := selector.SelectServer(pool, "textDocument/definition", context)
	if err != nil {
		t.Fatalf("Failed to select server: %v", err)
	}

	if server == nil {
		t.Errorf("Expected a server to be selected")
	}

	// Verify selected server is healthy
	if server.metrics.healthScore < 0.8 {
		t.Errorf("Selected unhealthy server with score %f", server.metrics.healthScore)
	}
}

func TestNewServerSelector(t *testing.T) {
	testCases := []struct {
		name     string
		config   *config.LoadBalancingConfig
		expected string
	}{
		{
			name: "round_robin",
			config: &config.LoadBalancingConfig{
				Strategy: "round_robin",
			},
			expected: "round_robin",
		},
		{
			name: "least_connections",
			config: &config.LoadBalancingConfig{
				Strategy: "least_connections",
			},
			expected: "least_connections",
		},
		{
			name: "response_time",
			config: &config.LoadBalancingConfig{
				Strategy: "response_time",
			},
			expected: "response_time",
		},
		{
			name: "performance",
			config: &config.LoadBalancingConfig{
				Strategy: "performance",
				WeightFactors: map[string]float64{
					"response_time": 0.4,
					"availability":  0.4,
					"resource":      0.2,
				},
			},
			expected: "performance",
		},
		{
			name: "feature",
			config: &config.LoadBalancingConfig{
				Strategy: "feature",
			},
			expected: "feature",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			selector, err := NewServerSelector(tc.config)
			if err != nil {
				t.Fatalf("Failed to create selector: %v", err)
			}

			if selector.GetName() != tc.expected {
				t.Errorf("Expected selector name %s, got %s", tc.expected, selector.GetName())
			}
		})
	}
}

func TestLoadBalancerIntegration(t *testing.T) {
	lbConfig := &config.LoadBalancingConfig{
		Strategy:        "round_robin",
		HealthThreshold: 0.8,
	}

	pool := NewLanguageServerPoolWithConfig("go", nil, lbConfig, nil)

	// Add test servers
	for i := 1; i <= 3; i++ {
		serverConfig := &config.ServerConfig{
			Name:     fmt.Sprintf("server%d", i),
			Languages: []string{"go"},
			Command:  "gopls",
		}
		
		server := &ServerInstance{
			config:    serverConfig,
			state:     ServerStateHealthy,
			startTime: time.Now(),
			metrics:   NewServerMetrics(),
		}

		err := pool.AddServer(server)
		if err != nil {
			t.Fatalf("Failed to add server to pool: %v", err)
		}
	}

	context := &SelectionRequestContext{
		Method:   "textDocument/definition",
		Priority: PriorityNormal,
		Timeout:  30 * time.Second,
	}

	// Test server selection through pool
	server, err := pool.SelectServerWithContext("textDocument/definition", context)
	if err != nil {
		t.Fatalf("Failed to select server through pool: %v", err)
	}

	if server == nil {
		t.Errorf("Expected a server to be selected")
	}

	// Test multiple server selection
	servers, err := pool.SelectMultipleServersWithType("textDocument/definition", 2)
	if err != nil {
		t.Fatalf("Failed to select multiple servers: %v", err)
	}

	if len(servers) != 2 {
		t.Errorf("Expected 2 servers, got %d", len(servers))
	}
}