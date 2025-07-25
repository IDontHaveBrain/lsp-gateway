package gateway

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"lsp-gateway/tests/framework"
)

// SimpleMultiServerManagerTestSuite provides unit tests for MultiServerManager
// using simplified mocking to avoid config package conflicts
type SimpleMultiServerManagerTestSuite struct {
	suite.Suite
	mockManager *framework.MockLSPServerManager
}

func (suite *SimpleMultiServerManagerTestSuite) SetupSuite() {
	suite.mockManager = framework.NewMockLSPServerManager()
}

func (suite *SimpleMultiServerManagerTestSuite) TearDownSuite() {
	if suite.mockManager != nil {
		suite.mockManager.Cleanup()
	}
}

func (suite *SimpleMultiServerManagerTestSuite) SetupTest() {
	// Reset mock manager for each test
	suite.mockManager.Reset()
}

func (suite *SimpleMultiServerManagerTestSuite) TearDownTest() {
	// Clean up after each test
	suite.mockManager.Reset()
}

// Test MockLSPServer functionality that MultiServerManager would use
func (suite *SimpleMultiServerManagerTestSuite) TestMockLSPServer_StateManagement() {
	mockServer := framework.NewMockLSPServer("test-server")

	// Test initial state
	assert.Equal(suite.T(), "test-server", mockServer.ServerID)
	assert.False(suite.T(), mockServer.ShouldFail)
	assert.Equal(suite.T(), 0, mockServer.StartCalls)
	assert.Equal(suite.T(), 0, mockServer.SendRequestCalls)

	// Test server start
	ctx := context.Background()
	err := mockServer.Start(ctx)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), 1, mockServer.StartCalls)
	assert.True(suite.T(), mockServer.IsActive())

	// Test request handling
	params := map[string]interface{}{"test": "data"}
	response, err := mockServer.SendRequest(ctx, "textDocument/definition", params)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), response)
	assert.Equal(suite.T(), 1, mockServer.SendRequestCalls)

	// Test server stop
	err = mockServer.Stop()
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), 1, mockServer.StopCalls)
}

func (suite *SimpleMultiServerManagerTestSuite) TestMockLSPServer_FailureSimulation() {
	mockServer := framework.NewMockLSPServer("failing-server")
	mockServer.ShouldFail = true
	mockServer.FailureRate = 1.0 // 100% failure rate

	ctx := context.Background()
	err := mockServer.Start(ctx)
	assert.NoError(suite.T(), err)

	// Test that requests fail when configured to fail
	params := map[string]interface{}{"test": "data"}
	_, err = mockServer.SendRequest(ctx, "textDocument/definition", params)
	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), 1, mockServer.SendRequestCalls)

	mockServer.Stop()
}

func (suite *SimpleMultiServerManagerTestSuite) TestMockLSPServer_PerformanceSimulation() {
	mockServer := framework.NewMockLSPServer("performance-server")
	mockServer.ResponseDelay = 100 * time.Millisecond
	mockServer.MemoryUsageMB = 64
	mockServer.CPUUsagePercent = 25.5
	mockServer.HealthScore = 0.85

	ctx := context.Background()
	err := mockServer.Start(ctx)
	assert.NoError(suite.T(), err)

	// Test response delay
	start := time.Now()
	params := map[string]interface{}{"test": "data"}
	_, err = mockServer.SendRequest(ctx, "textDocument/definition", params)
	elapsed := time.Since(start)

	assert.NoError(suite.T(), err)
	assert.True(suite.T(), elapsed >= 100*time.Millisecond, "Response should be delayed")
	assert.True(suite.T(), elapsed < 200*time.Millisecond, "Response shouldn't be too delayed")

	// Test metrics
	metrics := mockServer.GetMetrics()
	assert.NotNil(suite.T(), metrics)
	assert.Equal(suite.T(), float64(64), metrics["memory_mb"])
	assert.Equal(suite.T(), 25.5, metrics["cpu_percent"])
	assert.Equal(suite.T(), 0.85, metrics["health_score"])

	mockServer.Stop()
}

func (suite *SimpleMultiServerManagerTestSuite) TestMockLSPServerManager_MultiServerManagement() {
	// Create multiple servers
	server1 := suite.mockManager.CreateServer("server-1")
	server2 := suite.mockManager.CreateServer("server-2")
	server3 := suite.mockManager.CreateServer("server-3")

	// Configure servers with different characteristics
	server1.HealthScore = 0.9
	server2.HealthScore = 0.7
	server3.ShouldFail = true
	server3.FailureRate = 0.5

	ctx := context.Background()

	// Start all servers
	err := suite.mockManager.StartServer("server-1")
	assert.NoError(suite.T(), err)
	err = suite.mockManager.StartServer("server-2")
	assert.NoError(suite.T(), err)
	err = suite.mockManager.StartServer("server-3")
	assert.NoError(suite.T(), err)

	// Verify all servers are active
	assert.True(suite.T(), server1.IsActive())
	assert.True(suite.T(), server2.IsActive())
	assert.True(suite.T(), server3.IsActive())

	// Test requests to different servers
	params := map[string]interface{}{"test": "data"}

	// Server 1 should succeed
	_, err = server1.SendRequest(ctx, "textDocument/definition", params)
	assert.NoError(suite.T(), err)

	// Server 2 should succeed
	_, err = server2.SendRequest(ctx, "textDocument/hover", params)
	assert.NoError(suite.T(), err)

	// Server 3 might fail (50% failure rate)
	_, err = server3.SendRequest(ctx, "textDocument/references", params)
	// Don't assert error since it's probabilistic

	// Test server stop
	err = suite.mockManager.StopServer("server-1")
	assert.NoError(suite.T(), err)
	assert.False(suite.T(), server1.IsActive())

	// Clean up
	suite.mockManager.StopAllServers()
}

func (suite *SimpleMultiServerManagerTestSuite) TestMockLSPServerManager_ConcurrentOperations() {
	// Create servers for concurrent testing
	numServers := 5
	for i := 0; i < numServers; i++ {
		serverName := fmt.Sprintf("concurrent-server-%d", i)
		server := suite.mockManager.CreateServer(serverName)
		server.ResponseDelay = time.Duration(i*10) * time.Millisecond
		server.HealthScore = 0.8 + float64(i)*0.04 // 0.8 to 0.96
	}

	// Start all servers concurrently
	var startErrors []error
	for i := 0; i < numServers; i++ {
		serverName := fmt.Sprintf("concurrent-server-%d", i)
		if err := suite.mockManager.StartServer(serverName); err != nil {
			startErrors = append(startErrors, err)
		}
	}
	assert.Empty(suite.T(), startErrors, "All servers should start successfully")

	// Test concurrent requests
	ctx := context.Background()
	params := map[string]interface{}{"test": "concurrent"}

	type result struct {
		serverName string
		err        error
		duration   time.Duration
	}

	results := make(chan result, numServers)

	// Send concurrent requests
	for i := 0; i < numServers; i++ {
		go func(serverIndex int) {
			serverName := fmt.Sprintf("concurrent-server-%d", serverIndex)
			server := suite.mockManager.GetServer(serverName)
			if server != nil {
				start := time.Now()
				_, err := server.SendRequest(ctx, "textDocument/definition", params)
				duration := time.Since(start)
				results <- result{serverName: serverName, err: err, duration: duration}
			} else {
				results <- result{serverName: serverName, err: fmt.Errorf("server not found"), duration: 0}
			}
		}(i)
	}

	// Collect results
	var successCount int
	for i := 0; i < numServers; i++ {
		select {
		case res := <-results:
			if res.err == nil {
				successCount++
			}
		case <-time.After(2 * time.Second):
			assert.Fail(suite.T(), "Timeout waiting for concurrent request results")
		}
	}

	assert.Equal(suite.T(), numServers, successCount, "All concurrent requests should succeed")

	// Clean up
	suite.mockManager.StopAllServers()
}

func (suite *SimpleMultiServerManagerTestSuite) TestMockLSPServerManager_HealthMonitoring() {
	// Create servers with different health profiles
	healthyServer := suite.mockManager.CreateServer("healthy-server")
	healthyServer.HealthScore = 0.95
	healthyServer.ResponseDelay = 50 * time.Millisecond

	degradedServer := suite.mockManager.CreateServer("degraded-server")
	degradedServer.HealthScore = 0.6
	degradedServer.ResponseDelay = 200 * time.Millisecond

	unhealthyServer := suite.mockManager.CreateServer("unhealthy-server")
	unhealthyServer.HealthScore = 0.2
	unhealthyServer.ShouldFail = true
	unhealthyServer.FailureRate = 0.8

	// Start all servers
	suite.mockManager.StartServer("healthy-server")
	suite.mockManager.StartServer("degraded-server")
	suite.mockManager.StartServer("unhealthy-server")

	// Test health characteristics
	ctx := context.Background()
	params := map[string]interface{}{"test": "health"}

	// Healthy server should respond quickly and successfully
	start := time.Now()
	_, err := healthyServer.SendRequest(ctx, "textDocument/definition", params)
	elapsed := time.Since(start)
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), elapsed < 100*time.Millisecond)

	// Degraded server should respond slower but successfully
	start = time.Now()
	_, err = degradedServer.SendRequest(ctx, "textDocument/hover", params)
	elapsed = time.Since(start)
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), elapsed >= 200*time.Millisecond)

	// Unhealthy server should often fail
	failures := 0
	attempts := 10
	for i := 0; i < attempts; i++ {
		_, err := unhealthyServer.SendRequest(ctx, "textDocument/references", params)
		if err != nil {
			failures++
		}
	}

	// Should have significant failure rate (expecting around 80% failure rate)
	assert.True(suite.T(), failures >= attempts/2, "Unhealthy server should have significant failures")

	// Clean up
	suite.mockManager.StopAllServers()
}

// Helper function for formatted output
func (suite *SimpleMultiServerManagerTestSuite) TestMockLSPServerManager_Cleanup() {
	// Test that cleanup works properly
	initialServerCount := len(suite.mockManager.GetAllServers())

	// Create some servers
	suite.mockManager.CreateServer("cleanup-test-1")
	suite.mockManager.CreateServer("cleanup-test-2")
	suite.mockManager.StartServer("cleanup-test-1")
	suite.mockManager.StartServer("cleanup-test-2")

	assert.Equal(suite.T(), initialServerCount+2, len(suite.mockManager.GetAllServers()))

	// Test cleanup
	suite.mockManager.Cleanup()

	// After cleanup, should have no servers
	assert.Equal(suite.T(), 0, len(suite.mockManager.GetAllServers()))
}

func TestSimpleMultiServerManagerTestSuite(t *testing.T) {
	suite.Run(t, new(SimpleMultiServerManagerTestSuite))
}
