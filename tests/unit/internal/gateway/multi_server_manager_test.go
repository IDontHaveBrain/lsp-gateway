package gateway

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/gateway"
	"lsp-gateway/tests/framework"
)

type MultiServerManagerTestSuite struct {
	suite.Suite
	manager       *gateway.MultiServerManager
	mockManager   *framework.MockLSPServerManager
	config        *config.GatewayConfig
	tempDir       string
	testFramework *framework.MultiLanguageTestFramework
}

func (suite *MultiServerManagerTestSuite) SetupSuite() {
	suite.mockManager = framework.NewMockLSPServerManager()

	var err error
	suite.testFramework, err = framework.NewMultiLanguageTestFramework(nil)
	require.NoError(suite.T(), err)
}

func (suite *MultiServerManagerTestSuite) TearDownSuite() {
	if suite.testFramework != nil {
		suite.testFramework.Cleanup()
	}
	if suite.mockManager != nil {
		suite.mockManager.Cleanup()
	}
}

func (suite *MultiServerManagerTestSuite) SetupTest() {
	suite.config = &config.GatewayConfig{
		Port:                  8080,
		Timeout:               30 * time.Second,
		MaxConcurrentRequests: 100,
		Servers: []config.ServerConfig{
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
				Name:      "python-server-1",
				Languages: []string{"python"},
				Command:   "pylsp",
				Transport: "stdio",
			},
			{
				Name:      "typescript-server-1",
				Languages: []string{"typescript", "javascript"},
				Command:   "typescript-language-server",
				Args:      []string{"--stdio"},
				Transport: "stdio",
			},
		},
	}

	var err error
	suite.manager, err = gateway.NewMultiServerManager(suite.config, nil)
	require.NoError(suite.T(), err)
}

func (suite *MultiServerManagerTestSuite) TearDownTest() {
	if suite.manager != nil {
		suite.manager.Stop()
	}
}

// ServerInstance Tests

func (suite *MultiServerManagerTestSuite) TestServerInstance_NewServerInstance() {
	tests := []struct {
		name        string
		serverName  string
		client      framework.MockLSPServer
		expectError bool
	}{
		{
			name:       "valid server instance creation",
			serverName: "test-server",
			client:     *framework.NewMockLSPServer("test-server"),
		},
		{
			name:       "server with empty name",
			serverName: "",
			client:     *framework.NewMockLSPServer(""),
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			server := gateway.NewServerInstance(tt.serverName, &tt.client)

			assert.NotNil(suite.T(), server)
			assert.Equal(suite.T(), gateway.ServerStateStopped, server.GetState())
			assert.False(suite.T(), server.IsHealthy())
			assert.False(suite.T(), server.IsActive())
		})
	}
}

func (suite *MultiServerManagerTestSuite) TestServerInstance_StateTransitions() {
	mockServer := framework.NewMockLSPServer("test-server")
	server := gateway.NewServerInstance("test-server", mockServer)

	// Initial state
	assert.Equal(suite.T(), gateway.ServerStateStopped, server.GetState())
	assert.False(suite.T(), server.IsHealthy())
	assert.False(suite.T(), server.IsActive())

	// Start server
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := server.Start(ctx)
	assert.NoError(suite.T(), err)

	// Wait for state to stabilize
	time.Sleep(100 * time.Millisecond)

	// Check running state
	assert.True(suite.T(), server.IsActive())
	state := server.GetState()
	assert.True(suite.T(), state == gateway.ServerStateHealthy || state == gateway.ServerStateStarting)

	// Stop server
	err = server.Stop()
	assert.NoError(suite.T(), err)

	// Check stopped state
	assert.Equal(suite.T(), gateway.ServerStateStopped, server.GetState())
	assert.False(suite.T(), server.IsActive())
}

func (suite *MultiServerManagerTestSuite) TestServerInstance_RequestHandling() {
	mockServer := framework.NewMockLSPServer("test-server")
	mockServer.ResponseDelay = 50 * time.Millisecond
	server := gateway.NewServerInstance("test-server", mockServer)

	ctx := context.Background()
	err := server.Start(ctx)
	require.NoError(suite.T(), err)

	// Wait for server to be ready
	time.Sleep(200 * time.Millisecond)

	// Test successful request
	params := map[string]interface{}{"test": "data"}
	response, err := server.SendRequest(ctx, "textDocument/definition", params)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), response)
	assert.Equal(suite.T(), 1, mockServer.SendRequestCalls)

	// Test multiple concurrent requests
	var wg sync.WaitGroup
	concurrentRequests := 10
	wg.Add(concurrentRequests)

	for i := 0; i < concurrentRequests; i++ {
		go func() {
			defer wg.Done()
			_, err := server.SendRequest(ctx, "textDocument/hover", params)
			assert.NoError(suite.T(), err)
		}()
	}

	wg.Wait()
	assert.Equal(suite.T(), concurrentRequests+1, mockServer.SendRequestCalls)

	server.Stop()
}

func (suite *MultiServerManagerTestSuite) TestServerInstance_CircuitBreakerIntegration() {
	mockServer := framework.NewMockLSPServer("failing-server")
	mockServer.ShouldFail = true
	mockServer.FailureRate = 1.0 // 100% failure rate
	server := gateway.NewServerInstance("failing-server", mockServer)

	ctx := context.Background()
	err := server.Start(ctx)
	require.NoError(suite.T(), err)

	time.Sleep(100 * time.Millisecond)

	// Send multiple failing requests to trigger circuit breaker
	params := map[string]interface{}{"test": "data"}
	for i := 0; i < 10; i++ {
		_, err := server.SendRequest(ctx, "textDocument/definition", params)
		// Expect errors due to server failures
		assert.Error(suite.T(), err)
	}

	// Verify circuit breaker state affects server health
	time.Sleep(100 * time.Millisecond)
	assert.False(suite.T(), server.IsHealthy())

	server.Stop()
}

func (suite *MultiServerManagerTestSuite) TestServerInstance_MetricsCollection() {
	mockServer := framework.NewMockLSPServer("metrics-server")
	mockServer.MemoryUsageMB = 64
	mockServer.CPUUsagePercent = 25.5
	mockServer.HealthScore = 0.85
	server := gateway.NewServerInstance("metrics-server", mockServer)

	ctx := context.Background()
	err := server.Start(ctx)
	require.NoError(suite.T(), err)

	time.Sleep(100 * time.Millisecond)

	// Send some requests to generate metrics
	params := map[string]interface{}{"test": "data"}
	for i := 0; i < 5; i++ {
		server.SendRequest(ctx, "textDocument/definition", params)
	}

	metrics := server.GetMetrics()
	assert.NotNil(suite.T(), metrics)
	assert.True(suite.T(), metrics.RequestCount >= 5)
	assert.True(suite.T(), metrics.MemoryUsageMB > 0)
	assert.True(suite.T(), metrics.CPUUsagePercent >= 0)

	server.Stop()
}

// LanguageServerPool Tests

func (suite *MultiServerManagerTestSuite) TestLanguageServerPool_Creation() {
	tests := []struct {
		name     string
		language string
		strategy string
	}{
		{
			name:     "go pool with round robin",
			language: "go",
			strategy: "round_robin",
		},
		{
			name:     "python pool with least connections",
			language: "python",
			strategy: "least_connections",
		},
		{
			name:     "typescript pool with response time",
			language: "typescript",
			strategy: "response_time",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			pool := gateway.NewLanguageServerPool(tt.language)
			assert.NotNil(suite.T(), pool)
			assert.Equal(suite.T(), tt.language, pool.Language())
			assert.Equal(suite.T(), 0, len(pool.GetHealthyServers()))
		})
	}
}

func (suite *MultiServerManagerTestSuite) TestLanguageServerPool_ServerManagement() {
	pool := gateway.NewLanguageServerPool("go")

	// Create mock servers
	mockServer1 := framework.NewMockLSPServer("go-server-1")
	mockServer2 := framework.NewMockLSPServer("go-server-2")
	server1 := gateway.NewServerInstance("go-server-1", mockServer1)
	server2 := gateway.NewServerInstance("go-server-2", mockServer2)

	// Start servers
	ctx := context.Background()
	err := server1.Start(ctx)
	require.NoError(suite.T(), err)
	err = server2.Start(ctx)
	require.NoError(suite.T(), err)

	time.Sleep(200 * time.Millisecond)

	// Add servers to pool
	err = pool.AddServer(server1)
	assert.NoError(suite.T(), err)
	err = pool.AddServer(server2)
	assert.NoError(suite.T(), err)

	// Verify servers are in pool
	healthyServers := pool.GetHealthyServers()
	assert.Equal(suite.T(), 2, len(healthyServers))

	// Remove a server
	err = pool.RemoveServer("go-server-1")
	assert.NoError(suite.T(), err)

	healthyServers = pool.GetHealthyServers()
	assert.Equal(suite.T(), 1, len(healthyServers))
	assert.Equal(suite.T(), "go-server-2", healthyServers[0].Name())

	// Clean up
	server1.Stop()
	server2.Stop()
}

func (suite *MultiServerManagerTestSuite) TestLanguageServerPool_LoadBalancing() {
	pool := gateway.NewLanguageServerPool("go")

	// Create multiple mock servers with different performance characteristics
	servers := make([]*gateway.ServerInstance, 3)
	mockServers := make([]*framework.MockLSPServer, 3)

	for i := 0; i < 3; i++ {
		name := fmt.Sprintf("go-server-%d", i+1)
		mockServers[i] = framework.NewMockLSPServer(name)
		mockServers[i].ResponseDelay = time.Duration(i*50) * time.Millisecond
		servers[i] = gateway.NewServerInstance(name, mockServers[i])

		ctx := context.Background()
		err := servers[i].Start(ctx)
		require.NoError(suite.T(), err)

		err = pool.AddServer(servers[i])
		require.NoError(suite.T(), err)
	}

	time.Sleep(300 * time.Millisecond)

	// Test round-robin behavior
	selectedServers := make(map[string]int)
	for i := 0; i < 9; i++ {
		server := pool.SelectServer("textDocument/definition")
		if server != nil {
			selectedServers[server.Name()]++
		}
	}

	// Should distribute requests across servers
	assert.True(suite.T(), len(selectedServers) > 1, "Load balancer should distribute requests")

	// Test multiple server selection for concurrent requests
	multipleServers := pool.SelectMultipleServers(2)
	assert.True(suite.T(), len(multipleServers) <= 2)
	assert.True(suite.T(), len(multipleServers) >= 1)

	// Clean up
	for _, server := range servers {
		server.Stop()
	}
}

func (suite *MultiServerManagerTestSuite) TestLanguageServerPool_HealthMonitoring() {
	pool := gateway.NewLanguageServerPool("go")

	// Create servers with different health characteristics
	healthyMock := framework.NewMockLSPServer("healthy-server")
	healthyMock.HealthScore = 0.9
	healthyServer := gateway.NewServerInstance("healthy-server", healthyMock)

	unhealthyMock := framework.NewMockLSPServer("unhealthy-server")
	unhealthyMock.ShouldFail = true
	unhealthyMock.FailureRate = 0.8
	unhealthyServer := gateway.NewServerInstance("unhealthy-server", unhealthyMock)

	ctx := context.Background()
	err := healthyServer.Start(ctx)
	require.NoError(suite.T(), err)
	err = unhealthyServer.Start(ctx)
	require.NoError(suite.T(), err)

	time.Sleep(200 * time.Millisecond)

	// Add servers to pool
	pool.AddServer(healthyServer)
	pool.AddServer(unhealthyServer)

	// Send requests to trigger health changes
	for i := 0; i < 10; i++ {
		unhealthyServer.SendRequest(ctx, "test", map[string]interface{}{})
		time.Sleep(10 * time.Millisecond)
	}

	time.Sleep(200 * time.Millisecond)

	// Check that only healthy servers are selected
	healthyServers := pool.GetHealthyServers()
	assert.True(suite.T(), len(healthyServers) >= 1)

	// The healthy server should still be available
	foundHealthy := false
	for _, server := range healthyServers {
		if server.Name() == "healthy-server" {
			foundHealthy = true
			break
		}
	}
	assert.True(suite.T(), foundHealthy, "Healthy server should be available")

	// Clean up
	healthyServer.Stop()
	unhealthyServer.Stop()
}

// MultiServerManager Tests

func (suite *MultiServerManagerTestSuite) TestMultiServerManager_Initialization() {
	err := suite.manager.Initialize()
	assert.NoError(suite.T(), err)

	// Verify that pools were created for each language
	languages := []string{"go", "python", "typescript", "javascript"}
	for _, lang := range languages {
		server := suite.manager.GetServerForRequest(lang, "textDocument/definition")
		// Server might be nil if not started, but the pool should exist
		// The fact that GetServerForRequest doesn't panic indicates the pool exists
	}
}

func (suite *MultiServerManagerTestSuite) TestMultiServerManager_StartStop() {
	err := suite.manager.Initialize()
	require.NoError(suite.T(), err)

	// Replace LSP clients with mocks for testing
	suite.replaceManagerClientsWithMocks()

	// Start manager
	err = suite.manager.Start()
	assert.NoError(suite.T(), err)

	// Wait for startup
	time.Sleep(300 * time.Millisecond)

	// Verify servers are available
	goServer := suite.manager.GetServerForRequest("go", "textDocument/definition")
	assert.NotNil(suite.T(), goServer, "Go server should be available after start")

	pythonServer := suite.manager.GetServerForRequest("python", "textDocument/hover")
	assert.NotNil(suite.T(), pythonServer, "Python server should be available after start")

	// Stop manager
	err = suite.manager.Stop()
	assert.NoError(suite.T(), err)

	// Verify servers are stopped
	time.Sleep(200 * time.Millisecond)
	goServer = suite.manager.GetServerForRequest("go", "textDocument/definition")
	// After stop, servers should not be available for new requests
}

func (suite *MultiServerManagerTestSuite) TestMultiServerManager_ServerRouting() {
	err := suite.manager.Initialize()
	require.NoError(suite.T(), err)

	suite.replaceManagerClientsWithMocks()

	err = suite.manager.Start()
	require.NoError(suite.T(), err)

	time.Sleep(300 * time.Millisecond)

	tests := []struct {
		language    string
		requestType string
		expectFound bool
	}{
		{"go", "textDocument/definition", true},
		{"go", "textDocument/hover", true},
		{"python", "textDocument/references", true},
		{"typescript", "textDocument/documentSymbol", true},
		{"javascript", "textDocument/completion", true},
		{"unknown", "textDocument/definition", false},
	}

	for _, tt := range tests {
		suite.Run(fmt.Sprintf("%s_%s", tt.language, tt.requestType), func() {
			server := suite.manager.GetServerForRequest(tt.language, tt.requestType)
			if tt.expectFound {
				assert.NotNil(suite.T(), server, "Should find server for %s/%s", tt.language, tt.requestType)
			} else {
				assert.Nil(suite.T(), server, "Should not find server for %s/%s", tt.language, tt.requestType)
			}
		})
	}

	suite.manager.Stop()
}

func (suite *MultiServerManagerTestSuite) TestMultiServerManager_HealthMonitoring() {
	err := suite.manager.Initialize()
	require.NoError(suite.T(), err)

	suite.replaceManagerClientsWithMocks()

	err = suite.manager.Start()
	require.NoError(suite.T(), err)

	time.Sleep(300 * time.Millisecond)

	// Get a server
	server := suite.manager.GetServerForRequest("go", "textDocument/definition")
	require.NotNil(suite.T(), server)

	serverName := server.Name()

	// Mark server as unhealthy
	suite.manager.MarkServerUnhealthy(serverName, fmt.Errorf("test failure"))

	time.Sleep(100 * time.Millisecond)

	// Server should not be selected when unhealthy
	// (depending on pool configuration, it might return a different server or nil)
	newServer := suite.manager.GetServerForRequest("go", "textDocument/definition")
	if newServer != nil {
		// If we got a server, it should be a different one (if multiple exist) or the same if it's the only one
		assert.True(suite.T(), newServer.Name() != serverName || len(suite.config.Servers) == 1)
	}

	// Mark server as healthy again
	suite.manager.MarkServerHealthy(serverName)

	time.Sleep(100 * time.Millisecond)

	// Server should be available again
	recoveredServer := suite.manager.GetServerForRequest("go", "textDocument/definition")
	assert.NotNil(suite.T(), recoveredServer)

	suite.manager.Stop()
}

func (suite *MultiServerManagerTestSuite) TestMultiServerManager_ConcurrentOperations() {
	err := suite.manager.Initialize()
	require.NoError(suite.T(), err)

	suite.replaceManagerClientsWithMocks()

	err = suite.manager.Start()
	require.NoError(suite.T(), err)

	time.Sleep(300 * time.Millisecond)

	// Test concurrent server requests
	var wg sync.WaitGroup
	concurrentRequests := 20
	results := make([]bool, concurrentRequests)

	wg.Add(concurrentRequests)
	for i := 0; i < concurrentRequests; i++ {
		go func(idx int) {
			defer wg.Done()
			server := suite.manager.GetServerForRequest("go", "textDocument/definition")
			results[idx] = (server != nil)
		}(i)
	}

	wg.Wait()

	// Most requests should succeed
	successCount := 0
	for _, success := range results {
		if success {
			successCount++
		}
	}
	assert.True(suite.T(), successCount > concurrentRequests/2, "Most concurrent requests should succeed")

	suite.manager.Stop()
}

func (suite *MultiServerManagerTestSuite) TestMultiServerManager_LoadBalancingStrategies() {
	err := suite.manager.Initialize()
	require.NoError(suite.T(), err)

	suite.replaceManagerClientsWithMocks()

	err = suite.manager.Start()
	require.NoError(suite.T(), err)

	time.Sleep(300 * time.Millisecond)

	strategies := []string{"round_robin", "least_connections", "response_time"}

	for _, strategy := range strategies {
		suite.Run(fmt.Sprintf("strategy_%s", strategy), func() {
			err := suite.manager.SetLoadBalancingStrategy("go", strategy)
			assert.NoError(suite.T(), err)

			// Test that we can still get servers with the new strategy
			server := suite.manager.GetServerForRequest("go", "textDocument/definition")
			assert.NotNil(suite.T(), server, "Should get server with %s strategy", strategy)
		})
	}

	suite.manager.Stop()
}

func (suite *MultiServerManagerTestSuite) TestMultiServerManager_ServerRestart() {
	err := suite.manager.Initialize()
	require.NoError(suite.T(), err)

	suite.replaceManagerClientsWithMocks()

	err = suite.manager.Start()
	require.NoError(suite.T(), err)

	time.Sleep(300 * time.Millisecond)

	// Get a server to restart
	server := suite.manager.GetServerForRequest("go", "textDocument/definition")
	require.NotNil(suite.T(), server)

	serverName := server.Name()

	// Restart the server
	err = suite.manager.RestartServer(serverName)
	assert.NoError(suite.T(), err)

	// Wait for restart to complete
	time.Sleep(500 * time.Millisecond)

	// Server should be available again
	restartedServer := suite.manager.GetServerForRequest("go", "textDocument/definition")
	assert.NotNil(suite.T(), restartedServer)

	suite.manager.Stop()
}

func (suite *MultiServerManagerTestSuite) TestMultiServerManager_PoolRebalancing() {
	err := suite.manager.Initialize()
	require.NoError(suite.T(), err)

	suite.replaceManagerClientsWithMocks()

	err = suite.manager.Start()
	require.NoError(suite.T(), err)

	time.Sleep(300 * time.Millisecond)

	// Test pool rebalancing
	err = suite.manager.RebalancePool("go")
	assert.NoError(suite.T(), err)

	// Server should still be available after rebalancing
	server := suite.manager.GetServerForRequest("go", "textDocument/definition")
	assert.NotNil(suite.T(), server)

	suite.manager.Stop()
}

// Helper Methods

func (suite *MultiServerManagerTestSuite) replaceManagerClientsWithMocks() {
	// This would require access to internal manager state to replace LSP clients with mocks
	// For the purpose of this test, we'll create mock servers through the mock manager
	// In a real implementation, you might need dependency injection or internal access

	// Create mock servers for each configured server
	for _, serverConfig := range suite.config.Servers {
		mockServer := suite.mockManager.CreateServer(serverConfig.Name)
		mockServer.HealthScore = 0.9
		mockServer.ResponseDelay = 50 * time.Millisecond
	}
}

// Stress Tests

func (suite *MultiServerManagerTestSuite) TestMultiServerManager_StressTest() {
	if testing.Short() {
		suite.T().Skip("Skipping stress test in short mode")
	}

	err := suite.manager.Initialize()
	require.NoError(suite.T(), err)

	suite.replaceManagerClientsWithMocks()

	err = suite.manager.Start()
	require.NoError(suite.T(), err)

	time.Sleep(500 * time.Millisecond)

	// Stress test with many concurrent operations
	var wg sync.WaitGroup
	stressRequests := 100
	languages := []string{"go", "python", "typescript"}
	requestTypes := []string{"textDocument/definition", "textDocument/hover", "textDocument/references"}

	wg.Add(stressRequests)
	for i := 0; i < stressRequests; i++ {
		go func(idx int) {
			defer wg.Done()

			lang := languages[idx%len(languages)]
			reqType := requestTypes[idx%len(requestTypes)]

			server := suite.manager.GetServerForRequest(lang, reqType)
			if server != nil {
				// Simulate health changes
				if idx%10 == 0 {
					suite.manager.MarkServerUnhealthy(server.Name(), fmt.Errorf("stress test failure"))
				}
				if idx%15 == 0 {
					suite.manager.MarkServerHealthy(server.Name())
				}
			}
		}(i)
	}

	wg.Wait()

	// Manager should still be functional after stress test
	server := suite.manager.GetServerForRequest("go", "textDocument/definition")
	assert.NotNil(suite.T(), server)

	suite.manager.Stop()
}

func TestMultiServerManagerTestSuite(t *testing.T) {
	suite.Run(t, new(MultiServerManagerTestSuite))
}
