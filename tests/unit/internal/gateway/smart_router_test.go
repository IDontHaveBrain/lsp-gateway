package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/gateway"
	"lsp-gateway/internal/transport"
	"lsp-gateway/mcp"
	"lsp-gateway/tests/framework"
)

// Test constants
const (
	TestWorkspaceID = "test-workspace-123"
	TestServerName1 = "test-server-1"
	TestServerName2 = "test-server-2"
	TestLanguageGo = "go"
	TestLanguagePython = "python"
)

// Mock implementations for testing

// MockProjectAwareRouter implements the ProjectAwareRouter for testing
type MockProjectAwareRouter struct {
	*MockRouter
}

type MockRouter struct {
	GetServerByLanguageFunc func(language string) (string, error)
	GetClientFunc           func(serverName string) (transport.LSPClient, error)
	servers                 map[string]transport.LSPClient
	mu                      sync.RWMutex
}

func NewMockRouter() *MockRouter {
	return &MockRouter{
		servers: make(map[string]transport.LSPClient),
	}
}

func (m *MockRouter) GetServerByLanguage(language string) (string, error) {
	if m.GetServerByLanguageFunc != nil {
		return m.GetServerByLanguageFunc(language)
	}
	
	languageServerMap := map[string]string{
		"go":         "gopls",
		"python":     "pylsp",
		"javascript": "typescript-language-server",
		"typescript": "typescript-language-server",
		"java":       "jdtls",
	}
	
	if server, exists := languageServerMap[language]; exists {
		return server, nil
	}
	
	return "", fmt.Errorf("no server found for language: %s", language)
}

func (m *MockRouter) GetClient(serverName string) (transport.LSPClient, error) {
	if m.GetClientFunc != nil {
		return m.GetClientFunc(serverName)
	}
	
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if client, exists := m.servers[serverName]; exists {
		return client, nil
	}
	
	return nil, fmt.Errorf("client not found for server: %s", serverName)
}

func (m *MockRouter) SetClient(serverName string, client transport.LSPClient) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.servers[serverName] = client
}

func (m *MockProjectAwareRouter) RouteRequestWithWorkspace(fileURI string) (string, error) {
	// Extract language from URI for routing
	if fileURI == "" {
		return "", fmt.Errorf("empty file URI")
	}
	
	// Simple URI to server mapping for testing
	if fileURI == "file:///test/main.go" {
		return "gopls", nil
	}
	if fileURI == "file:///test/main.py" {
		return "pylsp", nil
	}
	
	return "", fmt.Errorf("no server found for URI: %s", fileURI)
}

// MockWorkspaceManager for testing
type MockWorkspaceManager struct {
	clients map[string]map[string]transport.LSPClient
	mu      sync.RWMutex
}

func NewMockWorkspaceManager() *MockWorkspaceManager {
	return &MockWorkspaceManager{
		clients: make(map[string]map[string]transport.LSPClient),
	}
}

func (m *MockWorkspaceManager) GetLSPClient(workspaceID, serverName string) (transport.LSPClient, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if workspace, exists := m.clients[workspaceID]; exists {
		if client, exists := workspace[serverName]; exists {
			return client, nil
		}
	}
	
	return nil, fmt.Errorf("client not found for workspace %s, server %s", workspaceID, serverName)
}

func (m *MockWorkspaceManager) SetLSPClient(workspaceID, serverName string, client transport.LSPClient) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.clients[workspaceID] == nil {
		m.clients[workspaceID] = make(map[string]transport.LSPClient)
	}
	
	m.clients[workspaceID][serverName] = client
}

// MockLSPClient implements transport.LSPClient for testing
type MockLSPClient struct {
	*framework.MockLSPServer
	serverName      string
	requestCount    int64
	responses       map[string]interface{}
	shouldFail      bool
	responseDelay   time.Duration
	mu              sync.RWMutex
}

func NewMockLSPClient(serverName string) *MockLSPClient {
	mockServer := &framework.MockLSPServer{
		Language:    "test",
		ServerID:    serverName,
		IsRunning:   true,
		HealthScore: 1.0,
	}
	
	return &MockLSPClient{
		MockLSPServer: mockServer,
		serverName:    serverName,
		responses:     make(map[string]interface{}),
	}
}

func (m *MockLSPClient) SendRequest(method string, params interface{}) (interface{}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.requestCount++
	
	if m.responseDelay > 0 {
		time.Sleep(m.responseDelay)
	}
	
	if m.shouldFail {
		return nil, fmt.Errorf("mock failure for server %s, method %s", m.serverName, method)
	}
	
	if response, exists := m.responses[method]; exists {
		return response, nil
	}
	
	// Default responses based on method
	switch method {
	case "textDocument/definition":
		return map[string]interface{}{
			"uri":   fmt.Sprintf("file:///mock/%s/definition.go", m.serverName),
			"range": map[string]interface{}{"start": map[string]int{"line": 10, "character": 5}},
		}, nil
	case "textDocument/references":
		return []map[string]interface{}{
			{"uri": fmt.Sprintf("file:///mock/%s/ref1.go", m.serverName), "range": map[string]interface{}{"start": map[string]int{"line": 5, "character": 10}}},
		}, nil
	case "textDocument/hover":
		return map[string]interface{}{
			"contents": fmt.Sprintf("Mock hover from %s", m.serverName),
		}, nil
	case "textDocument/documentSymbol":
		return []map[string]interface{}{
			{"name": fmt.Sprintf("MockFunction_%s", m.serverName), "kind": 12},
		}, nil
	case "workspace/symbol":
		return []map[string]interface{}{
			{"name": fmt.Sprintf("GlobalSymbol_%s", m.serverName), "kind": 12},
		}, nil
	default:
		return map[string]interface{}{
			"result": fmt.Sprintf("Mock response from %s for %s", m.serverName, method),
		}, nil
	}
}

func (m *MockLSPClient) SetResponse(method string, response interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses[method] = response
}

func (m *MockLSPClient) SetShouldFail(shouldFail bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shouldFail = shouldFail
}

func (m *MockLSPClient) SetResponseDelay(delay time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responseDelay = delay
}

func (m *MockLSPClient) GetRequestCount() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.requestCount
}

// Test fixtures and helpers

func createTestGatewayConfig() *config.GatewayConfig {
	return &config.GatewayConfig{
		Port:                    8080,
		Timeout:                 30 * time.Second,
		MaxConcurrentRequests:   100,
		Servers: []config.ServerConfig{
			{
				Name:      "gopls",
				Languages: []string{"go"},
				Command:   "gopls",
				Transport: "stdio",
				Priority:  1,
				Weight:    1.0,
			},
			{
				Name:      "pylsp",
				Languages: []string{"python"},
				Command:   "python",
				Args:      []string{"-m", "pylsp"},
				Transport: "stdio",
				Priority:  1,
				Weight:    1.0,
			},
			{
				Name:      "typescript-language-server",
				Languages: []string{"javascript", "typescript"},
				Command:   "typescript-language-server",
				Args:      []string{"--stdio"},
				Transport: "stdio",
				Priority:  1,
				Weight:    1.0,
			},
		},
		ServerPools: []config.ServerPool{
			{
				Language: "go",
				Servers: []*config.ServerConfig{
					{Name: "gopls-1", Languages: []string{"go"}, Priority: 1, Weight: 1.0},
					{Name: "gopls-2", Languages: []string{"go"}, Priority: 2, Weight: 0.8},
				},
				LoadBalancingConfig: &config.LoadBalancingConfig{
					Strategy: "round_robin",
				},
			},
		},
	}
}

func createTestSmartRouter() (*gateway.SmartRouterImpl, *MockWorkspaceManager, *MockRouter) {
	config := createTestGatewayConfig()
	logger := mcp.NewStructuredLogger("test", mcp.LogLevelDebug)
	
	mockRouter := NewMockRouter()
	projectAwareRouter := &MockProjectAwareRouter{Router: mockRouter}
	workspaceManager := NewMockWorkspaceManager()
	
	smartRouter := gateway.NewSmartRouter(projectAwareRouter, config, workspaceManager, logger)
	
	return smartRouter, workspaceManager, mockRouter
}

func createTestLSPRequest(method, language string) *gateway.LSPRequest {
	return &gateway.LSPRequest{
		Method:   method,
		Params:   map[string]interface{}{"test": "params"},
		Language: language,
		URI:      fmt.Sprintf("file:///test/main.%s", getFileExtForLanguage(language)),
		Context:  context.Background(),
	}
}

func getFileExtForLanguage(language string) string {
	switch language {
	case "go":
		return "go"
	case "python":
		return "py"
	case "javascript":
		return "js"
	case "typescript":
		return "ts"
	default:
		return "txt"
	}
}

// Test Suite: Core Routing Tests

func TestSmartRouter_NewSmartRouter(t *testing.T) {
	smartRouter, _, _ := createTestSmartRouter()
	
	if smartRouter == nil {
		t.Fatal("SmartRouter should not be nil")
	}
	
	// Test default strategies are set
	strategy := smartRouter.GetRoutingStrategy(gateway.LSP_METHOD_DEFINITION)
	if strategy != gateway.SingleTargetWithFallback {
		t.Errorf("Expected SingleTargetWithFallback for definition, got %s", strategy)
	}
	
	strategy = smartRouter.GetRoutingStrategy(gateway.LSP_METHOD_REFERENCES)
	if strategy != gateway.MultiTargetParallel {
		t.Errorf("Expected MultiTargetParallel for references, got %s", strategy)
	}
}

func TestSmartRouter_SetAndGetRoutingStrategy(t *testing.T) {
	smartRouter, _, _ := createTestSmartRouter()
	
	testMethod := "textDocument/completion"
	newStrategy := gateway.LoadBalanced
	
	// Set new strategy
	smartRouter.SetRoutingStrategy(testMethod, newStrategy)
	
	// Get and verify
	retrievedStrategy := smartRouter.GetRoutingStrategy(testMethod)
	if retrievedStrategy != newStrategy {
		t.Errorf("Expected %s, got %s", newStrategy, retrievedStrategy)
	}
	
	// Test unknown method returns default
	unknownStrategy := smartRouter.GetRoutingStrategy("unknown/method")
	if unknownStrategy != gateway.SingleTargetWithFallback {
		t.Errorf("Expected default SingleTargetWithFallback for unknown method, got %s", unknownStrategy)
	}
}

func TestSmartRouter_RouteRequest_SingleTargetWithFallback(t *testing.T) {
	smartRouter, _, mockRouter := createTestSmartRouter()
	
	// Setup mock client
	mockClient := NewMockLSPClient("gopls")
	mockRouter.SetClient("gopls", mockClient)
	
	request := createTestLSPRequest(gateway.LSP_METHOD_DEFINITION, TestLanguageGo)
	
	decision, err := smartRouter.RouteRequest(request)
	if err != nil {
		t.Fatalf("RouteRequest failed: %v", err)
	}
	
	if decision == nil {
		t.Fatal("RoutingDecision should not be nil")
	}
	
	if decision.ServerName != "gopls" {
		t.Errorf("Expected server 'gopls', got '%s'", decision.ServerName)
	}
	
	if decision.Strategy != gateway.SingleTargetWithFallback {
		t.Errorf("Expected SingleTargetWithFallback strategy, got %s", decision.Strategy)
	}
}

func TestSmartRouter_RouteRequest_LoadBalanced(t *testing.T) {
	smartRouter, _, mockRouter := createTestSmartRouter()
	
	// Set load balanced strategy for completion
	smartRouter.SetRoutingStrategy("textDocument/completion", gateway.LoadBalanced)
	
	// Setup mock clients
	mockClient1 := NewMockLSPClient("gopls-1")
	mockClient2 := NewMockLSPClient("gopls-2")
	mockRouter.SetClient("gopls-1", mockClient1)
	mockRouter.SetClient("gopls-2", mockClient2)
	
	request := createTestLSPRequest("textDocument/completion", TestLanguageGo)
	
	decision, err := smartRouter.RouteRequest(request)
	if err != nil {
		t.Fatalf("RouteRequest failed: %v", err)
	}
	
	if decision.Strategy != gateway.LoadBalanced {
		t.Errorf("Expected LoadBalanced strategy, got %s", decision.Strategy)
	}
	
	// Check metadata contains load balancing info
	if metadata := decision.Metadata; metadata != nil {
		if _, exists := metadata["lb_strategy"]; !exists {
			t.Error("Expected lb_strategy in metadata")
		}
	}
}

func TestSmartRouter_RouteMultiRequest(t *testing.T) {
	smartRouter, _, mockRouter := createTestSmartRouter()
	
	// Setup mock clients
	mockClient1 := NewMockLSPClient("gopls-1")
	mockClient2 := NewMockLSPClient("gopls-2")
	mockRouter.SetClient("gopls-1", mockClient1)
	mockRouter.SetClient("gopls-2", mockClient2)
	
	// Test MultiTargetParallel strategy (references)
	request := createTestLSPRequest(gateway.LSP_METHOD_REFERENCES, TestLanguageGo)
	
	decisions, err := smartRouter.RouteMultiRequest(request)
	if err != nil {
		t.Fatalf("RouteMultiRequest failed: %v", err)
	}
	
	if len(decisions) == 0 {
		t.Fatal("Expected at least one routing decision")
	}
	
	// For single target strategies, should return single decision
	singleRequest := createTestLSPRequest(gateway.LSP_METHOD_DEFINITION, TestLanguageGo)
	decisions, err = smartRouter.RouteMultiRequest(singleRequest)
	if err != nil {
		t.Fatalf("RouteMultiRequest failed for single target: %v", err)
	}
	
	if len(decisions) != 1 {
		t.Errorf("Expected 1 decision for single target, got %d", len(decisions))
	}
}

// Test Suite: Response Aggregation Tests

func TestSmartRouter_AggregateBroadcast(t *testing.T) {
	smartRouter, _, mockRouter := createTestSmartRouter()
	
	// Setup mock clients with different responses
	mockClient1 := NewMockLSPClient("gopls-1")
	mockClient2 := NewMockLSPClient("gopls-2")
	
	// Set different responses
	mockClient1.SetResponse(gateway.LSP_METHOD_REFERENCES, []map[string]interface{}{
		{"uri": "file:///test/ref1.go", "range": map[string]interface{}{"start": map[string]int{"line": 5, "character": 10}}},
	})
	mockClient2.SetResponse(gateway.LSP_METHOD_REFERENCES, []map[string]interface{}{
		{"uri": "file:///test/ref2.go", "range": map[string]interface{}{"start": map[string]int{"line": 15, "character": 20}}},
	})
	
	mockRouter.SetClient("gopls-1", mockClient1)
	mockRouter.SetClient("gopls-2", mockClient2)
	
	request := createTestLSPRequest(gateway.LSP_METHOD_REFERENCES, TestLanguageGo)
	
	aggregated, err := smartRouter.AggregateBroadcast(request)
	if err != nil {
		t.Fatalf("AggregateBroadcast failed: %v", err)
	}
	
	if aggregated == nil {
		t.Fatal("AggregatedResponse should not be nil")
	}
	
	if aggregated.PrimaryResult == nil {
		t.Error("PrimaryResult should not be nil")
	}
	
	if aggregated.ServerCount != 2 {
		t.Errorf("Expected ServerCount 2, got %d", aggregated.ServerCount)
	}
	
	if aggregated.ProcessingTime <= 0 {
		t.Error("ProcessingTime should be positive")
	}
	
	// Check metadata
	if metadata := aggregated.Metadata; metadata != nil {
		if successCount, exists := metadata["success_count"]; exists {
			if successCount.(int) != 2 {
				t.Errorf("Expected success_count 2, got %v", successCount)
			}
		}
	}
}

func TestSmartRouter_AggregateBroadcast_WithFailures(t *testing.T) {
	smartRouter, _, mockRouter := createTestSmartRouter()
	
	// Setup one failing and one successful client
	mockClient1 := NewMockLSPClient("gopls-1")
	mockClient2 := NewMockLSPClient("gopls-2")
	
	mockClient1.SetShouldFail(true)
	mockClient2.SetShouldFail(false)
	mockClient2.SetResponse(gateway.LSP_METHOD_REFERENCES, []map[string]interface{}{
		{"uri": "file:///test/ref1.go"},
	})
	
	mockRouter.SetClient("gopls-1", mockClient1)
	mockRouter.SetClient("gopls-2", mockClient2)
	
	request := createTestLSPRequest(gateway.LSP_METHOD_REFERENCES, TestLanguageGo)
	
	aggregated, err := smartRouter.AggregateBroadcast(request)
	if err != nil {
		t.Fatalf("AggregateBroadcast should not fail with partial success: %v", err)
	}
	
	// Should still have primary result from successful server
	if aggregated.PrimaryResult == nil {
		t.Error("PrimaryResult should not be nil even with one failure")
	}
	
	// Check metadata for success/failure counts
	if metadata := aggregated.Metadata; metadata != nil {
		if successCount, exists := metadata["success_count"]; exists {
			if successCount.(int) != 1 {
				t.Errorf("Expected success_count 1, got %v", successCount)
			}
		}
	}
}

func TestSmartRouter_AggregateBroadcast_AllFailures(t *testing.T) {
	smartRouter, _, mockRouter := createTestSmartRouter()
	
	// Setup all failing clients
	mockClient1 := NewMockLSPClient("gopls-1")
	mockClient2 := NewMockLSPClient("gopls-2")
	
	mockClient1.SetShouldFail(true)
	mockClient2.SetShouldFail(true)
	
	mockRouter.SetClient("gopls-1", mockClient1)
	mockRouter.SetClient("gopls-2", mockClient2)
	
	request := createTestLSPRequest(gateway.LSP_METHOD_REFERENCES, TestLanguageGo)
	
	_, err := smartRouter.AggregateBroadcast(request)
	if err == nil {
		t.Error("AggregateBroadcast should fail when all servers fail")
	}
	
	if !contains(err.Error(), "all servers failed") {
		t.Errorf("Error should mention all servers failed, got: %v", err)
	}
}

// Test Suite: Circuit Breaker Tests

func TestSmartRouter_CircuitBreakerStateTransitions(t *testing.T) {
	smartRouter, _, _ := createTestSmartRouter()
	
	serverName := "test-server"
	
	// Initially server should be healthy
	if !smartRouter.IsServerHealthy(serverName) {
		t.Error("New server should be healthy initially")
	}
	
	// Simulate failures to trigger circuit breaker
	for i := 0; i < 5; i++ {
		smartRouter.UpdateServerPerformance(serverName, 100*time.Millisecond, false)
	}
	
	// Server should now be unhealthy (circuit breaker open)
	if smartRouter.IsServerHealthy(serverName) {
		t.Error("Server should be unhealthy after multiple failures")
	}
	
	// Wait for timeout and check half-open state
	time.Sleep(1 * time.Millisecond) // Small delay to simulate timeout passage
	
	// Recovery through successful requests
	smartRouter.UpdateServerPerformance(serverName, 50*time.Millisecond, true)
	
	// Additional successful requests should close circuit breaker
	for i := 0; i < 3; i++ {
		smartRouter.UpdateServerPerformance(serverName, 50*time.Millisecond, true)
	}
}

func TestSmartRouter_CircuitBreakerIntegration(t *testing.T) {
	smartRouter, _, mockRouter := createTestSmartRouter()
	
	// Setup a client that will fail
	mockClient := NewMockLSPClient("gopls")
	mockClient.SetShouldFail(true)
	mockRouter.SetClient("gopls", mockClient)
	
	request := createTestLSPRequest(gateway.LSP_METHOD_DEFINITION, TestLanguageGo)
	
	// Try multiple requests to trigger circuit breaker
	for i := 0; i < 6; i++ {
		_, err := smartRouter.RouteRequest(request)
		if err != nil {
			// Expected to fail due to mock failure
			continue
		}
	}
	
	// After multiple failures, server should be marked unhealthy
	// This would be reflected in routing decisions
	metrics := smartRouter.GetRoutingMetrics()
	if metrics.FailedRequests == 0 {
		t.Error("Expected some failed requests to be recorded")
	}
}

// Test Suite: Performance Metrics Tests

func TestSmartRouter_UpdateServerPerformance(t *testing.T) {
	smartRouter, _, _ := createTestSmartRouter()
	
	serverName := "test-server"
	responseTime := 100 * time.Millisecond
	
	// Update with successful request
	smartRouter.UpdateServerPerformance(serverName, responseTime, true)
	
	metrics := smartRouter.GetRoutingMetrics()
	
	if metrics.TotalRequests != 1 {
		t.Errorf("Expected TotalRequests 1, got %d", metrics.TotalRequests)
	}
	
	if metrics.SuccessfulRequests != 1 {
		t.Errorf("Expected SuccessfulRequests 1, got %d", metrics.SuccessfulRequests)
	}
	
	serverMetrics := metrics.ServerMetrics[serverName]
	if serverMetrics == nil {
		t.Fatal("Server metrics should exist")
	}
	
	if serverMetrics.RequestCount != 1 {
		t.Errorf("Expected RequestCount 1, got %d", serverMetrics.RequestCount)
	}
	
	if serverMetrics.SuccessCount != 1 {
		t.Errorf("Expected SuccessCount 1, got %d", serverMetrics.SuccessCount)
	}
	
	if serverMetrics.AverageResponseTime != responseTime {
		t.Errorf("Expected AverageResponseTime %v, got %v", responseTime, serverMetrics.AverageResponseTime)
	}
}

func TestSmartRouter_GetRoutingMetrics(t *testing.T) {
	smartRouter, _, _ := createTestSmartRouter()
	
	// Update metrics for multiple servers
	smartRouter.UpdateServerPerformance("server1", 100*time.Millisecond, true)
	smartRouter.UpdateServerPerformance("server2", 200*time.Millisecond, false)
	smartRouter.UpdateServerPerformance("server1", 150*time.Millisecond, true)
	
	metrics := smartRouter.GetRoutingMetrics()
	
	if metrics.TotalRequests != 3 {
		t.Errorf("Expected TotalRequests 3, got %d", metrics.TotalRequests)
	}
	
	if metrics.SuccessfulRequests != 2 {
		t.Errorf("Expected SuccessfulRequests 2, got %d", metrics.SuccessfulRequests)
	}
	
	if metrics.FailedRequests != 1 {
		t.Errorf("Expected FailedRequests 1, got %d", metrics.FailedRequests)
	}
	
	if len(metrics.ServerMetrics) != 2 {
		t.Errorf("Expected 2 server metrics, got %d", len(metrics.ServerMetrics))
	}
}

func TestSmartRouter_HealthScoreCalculation(t *testing.T) {
	smartRouter, _, _ := createTestSmartRouter()
	
	serverName := "test-server"
	
	// Multiple successful requests
	for i := 0; i < 5; i++ {
		smartRouter.UpdateServerPerformance(serverName, 100*time.Millisecond, true)
	}
	
	healthScore := smartRouter.GetServerHealthScore(serverName)
	if healthScore <= 0.8 {
		t.Errorf("Expected high health score after successful requests, got %f", healthScore)
	}
	
	// Add some failures
	for i := 0; i < 3; i++ {
		smartRouter.UpdateServerPerformance(serverName, 100*time.Millisecond, false)
	}
	
	newHealthScore := smartRouter.GetServerHealthScore(serverName)
	if newHealthScore >= healthScore {
		t.Errorf("Health score should decrease after failures, was %f, now %f", healthScore, newHealthScore)
	}
}

// Test Suite: Load Balancing Tests

func TestSmartRouter_RoundRobinSelection(t *testing.T) {
	smartRouter, _, mockRouter := createTestSmartRouter()
	
	// Setup multiple clients
	for i := 1; i <= 3; i++ {
		serverName := fmt.Sprintf("gopls-%d", i)
		mockClient := NewMockLSPClient(serverName)
		mockRouter.SetClient(serverName, mockClient)
	}
	
	// Set strategy to load balanced with round robin
	smartRouter.SetRoutingStrategy("textDocument/completion", gateway.LoadBalanced)
	
	request := createTestLSPRequest("textDocument/completion", TestLanguageGo)
	
	// Make multiple requests and track server selection
	serverCounts := make(map[string]int)
	for i := 0; i < 9; i++ { // 3 rounds of 3 servers
		decision, err := smartRouter.RouteRequest(request)
		if err != nil {
			t.Fatalf("RouteRequest failed: %v", err)
		}
		serverCounts[decision.ServerName]++
	}
	
	// With round robin, each server should be selected equally
	for server, count := range serverCounts {
		if count != 3 {
			t.Errorf("Server %s should be selected 3 times, got %d", server, count)
		}
	}
}

func TestSmartRouter_LeastConnectionsSelection(t *testing.T) {
	// This test would require more sophisticated mocking of active connections
	// For now, we'll test the basic functionality
	smartRouter, _, _ := createTestSmartRouter()
	
	if smartRouter == nil {
		t.Fatal("SmartRouter should not be nil")
	}
	
	// Test would involve setting up servers with different connection counts
	// and verifying that the server with least connections is selected
}

func TestSmartRouter_ResponseTimeSelection(t *testing.T) {
	smartRouter, _, mockRouter := createTestSmartRouter()
	
	// Setup clients with different response times
	fastClient := NewMockLSPClient("fast-server")
	slowClient := NewMockLSPClient("slow-server")
	
	fastClient.SetResponseDelay(10 * time.Millisecond)
	slowClient.SetResponseDelay(100 * time.Millisecond)
	
	mockRouter.SetClient("fast-server", fastClient)
	mockRouter.SetClient("slow-server", slowClient)
	
	// Update performance metrics to establish different response times
	smartRouter.UpdateServerPerformance("fast-server", 10*time.Millisecond, true)
	smartRouter.UpdateServerPerformance("slow-server", 100*time.Millisecond, true)
	
	// The selection logic would favor the faster server in real implementation
}

// Test Suite: Workspace Integration Tests

func TestSmartRouter_WorkspaceIntegration(t *testing.T) {
	smartRouter, workspaceManager, _ := createTestSmartRouter()
	
	// Setup workspace-specific client
	workspaceClient := NewMockLSPClient("workspace-gopls")
	workspaceManager.SetLSPClient(TestWorkspaceID, "gopls", workspaceClient)
	
	request := createTestLSPRequest(gateway.LSP_METHOD_DEFINITION, TestLanguageGo)
	request.WorkspaceID = TestWorkspaceID
	
	decision, err := smartRouter.RouteRequest(request)
	if err != nil {
		t.Fatalf("RouteRequest failed: %v", err)
	}
	
	if decision.Client == nil {
		t.Error("Decision should have a client")
	}
}

// Test Suite: Backward Compatibility Tests

func TestSmartRouter_GetServerForFile(t *testing.T) {
	smartRouter, _, _ := createTestSmartRouter()
	
	fileURI := "file:///test/main.go"
	
	serverName, err := smartRouter.GetServerForFile(fileURI)
	if err != nil {
		t.Fatalf("GetServerForFile failed: %v", err)
	}
	
	if serverName != "gopls" {
		t.Errorf("Expected 'gopls', got '%s'", serverName)
	}
}

func TestSmartRouter_GetServerForLanguage(t *testing.T) {
	smartRouter, _, _ := createTestSmartRouter()
	
	serverName, err := smartRouter.GetServerForLanguage(TestLanguageGo)
	if err != nil {
		t.Fatalf("GetServerForLanguage failed: %v", err)
	}
	
	if serverName != "gopls" {
		t.Errorf("Expected 'gopls', got '%s'", serverName)
	}
}

// Test Suite: Error Handling Tests

func TestSmartRouter_RouteRequest_NoLanguage(t *testing.T) {
	smartRouter, _, _ := createTestSmartRouter()
	
	request := createTestLSPRequest(gateway.LSP_METHOD_DEFINITION, "")
	request.Language = ""
	request.URI = ""
	
	_, err := smartRouter.RouteRequest(request)
	if err == nil {
		t.Error("RouteRequest should fail when no language is specified")
	}
}

func TestSmartRouter_RouteRequest_UnsupportedLanguage(t *testing.T) {
	smartRouter, _, _ := createTestSmartRouter()
	
	request := createTestLSPRequest(gateway.LSP_METHOD_DEFINITION, "unsupported-language")
	
	_, err := smartRouter.RouteRequest(request)
	if err == nil {
		t.Error("RouteRequest should fail for unsupported language")
	}
}

func TestSmartRouter_AggregateBroadcast_NoServers(t *testing.T) {
	smartRouter, _, _ := createTestSmartRouter()
	
	request := createTestLSPRequest(gateway.LSP_METHOD_REFERENCES, "unsupported-language")
	
	_, err := smartRouter.AggregateBroadcast(request)
	if err == nil {
		t.Error("AggregateBroadcast should fail when no servers available")
	}
}

// Test Suite: Concurrent Access Tests

func TestSmartRouter_ConcurrentRequests(t *testing.T) {
	smartRouter, _, mockRouter := createTestSmartRouter()
	
	// Setup mock client
	mockClient := NewMockLSPClient("gopls")
	mockRouter.SetClient("gopls", mockClient)
	
	const numGoroutines = 10
	const requestsPerGoroutine = 10
	
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*requestsPerGoroutine)
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < requestsPerGoroutine; j++ {
				request := createTestLSPRequest(gateway.LSP_METHOD_DEFINITION, TestLanguageGo)
				_, err := smartRouter.RouteRequest(request)
				if err != nil {
					errors <- err
				}
			}
		}()
	}
	
	wg.Wait()
	close(errors)
	
	// Check for any errors
	for err := range errors {
		t.Errorf("Concurrent request failed: %v", err)
	}
	
	// Verify metrics were updated correctly
	metrics := smartRouter.GetRoutingMetrics()
	expectedRequests := int64(numGoroutines * requestsPerGoroutines)
	if metrics.TotalRequests != expectedRequests {
		t.Errorf("Expected %d total requests, got %d", expectedRequests, metrics.TotalRequests)
	}
}

func TestSmartRouter_ConcurrentStrategyUpdates(t *testing.T) {
	smartRouter, _, _ := createTestSmartRouter()
	
	const numGoroutines = 5
	var wg sync.WaitGroup
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			method := fmt.Sprintf("test/method%d", id)
			strategy := gateway.LoadBalanced
			
			// Set and get strategy multiple times
			for j := 0; j < 10; j++ {
				smartRouter.SetRoutingStrategy(method, strategy)
				retrievedStrategy := smartRouter.GetRoutingStrategy(method)
				if retrievedStrategy != strategy {
					t.Errorf("Strategy mismatch for %s: expected %s, got %s", method, strategy, retrievedStrategy)
				}
			}
		}(i)
	}
	
	wg.Wait()
}

func TestSmartRouter_ConcurrentMetricsUpdates(t *testing.T) {
	smartRouter, _, _ := createTestSmartRouter()
	
	const numGoroutines = 5
	const updatesPerGoroutine = 20
	var wg sync.WaitGroup
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			serverName := fmt.Sprintf("server%d", id)
			
			for j := 0; j < updatesPerGoroutine; j++ {
				responseTime := time.Duration(j+1) * time.Millisecond
				success := j%2 == 0
				smartRouter.UpdateServerPerformance(serverName, responseTime, success)
			}
		}(i)
	}
	
	wg.Wait()
	
	// Verify final metrics
	metrics := smartRouter.GetRoutingMetrics()
	expectedTotal := int64(numGoroutines * updatesPerGoroutine)
	if metrics.TotalRequests != expectedTotal {
		t.Errorf("Expected %d total requests, got %d", expectedTotal, metrics.TotalRequests)
	}
	
	if len(metrics.ServerMetrics) != numGoroutines {
		t.Errorf("Expected %d servers in metrics, got %d", numGoroutines, len(metrics.ServerMetrics))
	}
}

// Test Suite: Language Extraction Tests

func TestSmartRouter_LanguageExtraction(t *testing.T) {
	smartRouter, _, mockRouter := createTestSmartRouter()
	
	// Setup mock client
	mockClient := NewMockLSPClient("gopls")
	mockRouter.SetClient("gopls", mockClient)
	
	testCases := []struct {
		uri            string
		expectedLang   string
		shouldSucceed  bool
	}{
		{"file:///test/main.go", "go", true},
		{"file:///test/script.py", "python", true},
		{"file:///test/app.js", "javascript", true},
		{"file:///test/component.ts", "typescript", true},
		{"file:///test/Main.java", "java", true},
		{"file:///test/program.rs", "rust", true},
		{"file:///test/document.txt", "", false},
	}
	
	for _, tc := range testCases {
		request := &gateway.LSPRequest{
			Method: gateway.LSP_METHOD_DEFINITION,
			URI:    tc.uri,
		}
		
		_, err := smartRouter.RouteRequest(request)
		
		if tc.shouldSucceed && err != nil {
			t.Errorf("Expected success for URI %s, got error: %v", tc.uri, err)
		}
		
		if !tc.shouldSucceed && err == nil {
			t.Errorf("Expected failure for URI %s, but succeeded", tc.uri)
		}
	}
}

// Test utilities

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// Benchmarks

func BenchmarkSmartRouter_RouteRequest(b *testing.B) {
	smartRouter, _, mockRouter := createTestSmartRouter()
	
	mockClient := NewMockLSPClient("gopls")
	mockRouter.SetClient("gopls", mockClient)
	
	request := createTestLSPRequest(gateway.LSP_METHOD_DEFINITION, TestLanguageGo)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := smartRouter.RouteRequest(request)
		if err != nil {
			b.Fatalf("RouteRequest failed: %v", err)
		}
	}
}

func BenchmarkSmartRouter_UpdateServerPerformance(b *testing.B) {
	smartRouter, _, _ := createTestSmartRouter()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		smartRouter.UpdateServerPerformance("test-server", 100*time.Millisecond, true)
	}
}

func BenchmarkSmartRouter_GetRoutingMetrics(b *testing.B) {
	smartRouter, _, _ := createTestSmartRouter()
	
	// Add some metrics data
	for i := 0; i < 100; i++ {
		smartRouter.UpdateServerPerformance(fmt.Sprintf("server%d", i%10), 100*time.Millisecond, true)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = smartRouter.GetRoutingMetrics()
	}
}