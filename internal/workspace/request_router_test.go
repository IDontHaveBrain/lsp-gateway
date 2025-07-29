package workspace

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"lsp-gateway/internal/transport"
	"lsp-gateway/mcp"
)

// Mock implementations for testing

type mockSubProjectResolver struct {
	projects map[string]*SubProject
	err      error
}

func (m *mockSubProjectResolver) ResolveSubProject(fileURI string) (*SubProject, error) {
	if m.err != nil {
		return nil, m.err
	}
	
	// Simple URI to project mapping
	for _, project := range m.projects {
		if strings.Contains(fileURI, project.Root) {
			return project, nil
		}
	}
	
	// Return default project
	return &SubProject{
		ID:          "default",
		Name:        "Default Project",
		Root:        "/default",
		ProjectType: "default",
		Languages:   []string{"go"},
		PrimaryLang: "go",
	}, nil
}

func (m *mockSubProjectResolver) ResolveSubProjectCached(fileURI string) (*SubProject, error) {
	return m.ResolveSubProject(fileURI)
}

func (m *mockSubProjectResolver) RefreshProjects(ctx context.Context) error {
	return m.err
}

func (m *mockSubProjectResolver) GetAllSubProjects() []*SubProject {
	projects := make([]*SubProject, 0, len(m.projects))
	for _, p := range m.projects {
		projects = append(projects, p)
	}
	return projects
}

func (m *mockSubProjectResolver) InvalidateCache(projectPath string) {}

func (m *mockSubProjectResolver) GetCacheStats() *ResolverCacheStats {
	return &ResolverCacheStats{}
}

func (m *mockSubProjectResolver) GetMetrics() *ResolverMetrics {
	return &ResolverMetrics{}
}

func (m *mockSubProjectResolver) Close() error {
	return nil
}

type mockLSPClient struct {
	responses map[string]string
	errors    map[string]error
	active    bool
}

func (m *mockLSPClient) SendRequest(ctx context.Context, method string, params interface{}) ([]byte, error) {
	if err, exists := m.errors[method]; exists {
		return nil, err
	}
	
	if response, exists := m.responses[method]; exists {
		return []byte(response), nil
	}
	
	// Default response
	return []byte(`{"result": []}`), nil
}

func (m *mockLSPClient) SendNotification(ctx context.Context, method string, params interface{}) error {
	return nil
}

func (m *mockLSPClient) Start(ctx context.Context) error {
	m.active = true
	return nil
}

func (m *mockLSPClient) Stop() error {
	m.active = false
	return nil
}

func (m *mockLSPClient) IsActive() bool {
	return m.active
}

type mockClientManager struct {
	clients map[string]map[string]transport.LSPClient
	err     error
}

func (m *mockClientManager) GetClient(subProjectID, language string) (transport.LSPClient, error) {
	if m.err != nil {
		return nil, m.err
	}
	
	if projectClients, exists := m.clients[subProjectID]; exists {
		if client, exists := projectClients[language]; exists {
			return client, nil
		}
	}
	
	// Return mock client
	return &mockLSPClient{
		responses: make(map[string]string),
		errors:    make(map[string]error),
		active:    true,
	}, nil
}

func (m *mockClientManager) CreateClient(ctx context.Context, subProject *SubProject, language string) (transport.LSPClient, error) {
	return m.GetClient(subProject.ID, language)
}

func (m *mockClientManager) RemoveClient(subProjectID, language string) error {
	return m.err
}

func (m *mockClientManager) RemoveAllClients(subProjectID string) error {
	return m.err
}

func (m *mockClientManager) GetClientStatus(subProjectID, language string) (*ExtendedClientStatus, error) {
	return &ExtendedClientStatus{
		SubProjectID: subProjectID,
		Language:     language,
		IsActive:     true,
		IsHealthy:    true,
		LastUsed:     time.Now(),
		StartedAt:    time.Now(),
	}, nil
}

func (m *mockClientManager) GetAllClientStatuses() map[string]map[string]*ExtendedClientStatus {
	return make(map[string]map[string]*ExtendedClientStatus)
}

func (m *mockClientManager) HealthCheck(ctx context.Context) (*ClientManagerHealth, error) {
	return &ClientManagerHealth{}, nil
}

func (m *mockClientManager) StartClients(ctx context.Context, subProject *SubProject) error {
	return m.err
}

func (m *mockClientManager) StopClients(ctx context.Context, subProject *SubProject) error {
	return m.err
}

func (m *mockClientManager) RefreshClients(ctx context.Context, subProject *SubProject) error {
	return m.err
}

func (m *mockClientManager) Shutdown(ctx context.Context) error {
	return m.err
}

func (m *mockClientManager) GetLoadBalancingInfo() *LoadBalancingInfo {
	return &LoadBalancingInfo{}
}

func (m *mockClientManager) GetRegistry() *ClientRegistry {
	return nil
}

// Test helper functions

func createTestProject(id, root, projectType string, languages []string) *SubProject {
	return &SubProject{
		ID:          id,
		Name:        id + " project",
		Root:        root,
		ProjectType: projectType,
		Languages:   languages,
		PrimaryLang: languages[0],
		LSPClients:  make(map[string]*ClientRef),
	}
}

func createTestRequest(method, uri string) *JSONRPCRequest {
	var params interface{}
	
	switch method {
	case "textDocument/definition", "textDocument/hover", "textDocument/references":
		params = map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": uri,
			},
			"position": map[string]interface{}{
				"line":      0,
				"character": 0,
			},
		}
	case "textDocument/documentSymbol":
		params = map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": uri,
			},
		}
	case "workspace/symbol":
		params = map[string]interface{}{
			"query": "test",
		}
	default:
		params = map[string]interface{}{}
	}
	
	return &JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      1,
		Method:  method,
		Params:  params,
	}
}

// Test cases

func TestNewSubProjectRequestRouter(t *testing.T) {
	logger := mcp.NewStructuredLogger(&mcp.LoggerConfig{Level: mcp.LogLevelInfo})
	router := NewSubProjectRequestRouter(logger)
	
	if router == nil {
		t.Fatal("NewSubProjectRequestRouter returned nil")
	}
	
	// Test initial state
	metrics := router.GetRoutingMetrics()
	if metrics == nil {
		t.Error("Expected metrics to be initialized")
	}
	
	supportedMethods := router.GetSupportedMethods()
	if len(supportedMethods) == 0 {
		t.Error("Expected supported methods to be populated")
	}
	
	// Verify expected methods are supported
	expectedMethods := []string{
		"textDocument/definition",
		"textDocument/references",
		"textDocument/hover",
		"textDocument/documentSymbol",
		"textDocument/completion",
		"workspace/symbol",
	}
	
	for _, method := range expectedMethods {
		found := false
		for _, supported := range supportedMethods {
			if supported == method {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected method %s to be supported", method)
		}
	}
}

func TestRouteRequest_Success(t *testing.T) {
	// Setup mocks
	projects := map[string]*SubProject{
		"project1": createTestProject("project1", "/path/to/project1", "go", []string{"go"}),
	}
	
	resolver := &mockSubProjectResolver{projects: projects}
	
	clients := map[string]map[string]transport.LSPClient{
		"project1": {
			"go": &mockLSPClient{
				responses: map[string]string{
					"textDocument/definition": `[{"uri": "file:///test.go", "range": {"start": {"line": 0, "character": 0}, "end": {"line": 0, "character": 5}}}]`,
				},
				active: true,
			},
		},
	}
	
	clientManager := &mockClientManager{clients: clients}
	
	// Create router
	logger := mcp.NewStructuredLogger(&mcp.LoggerConfig{Level: mcp.LogLevelInfo})
	router := NewSubProjectRequestRouter(logger)
	router.SetResolver(resolver)
	router.SetClientManager(clientManager)
	
	// Create test request
	request := createTestRequest("textDocument/definition", "file:///path/to/project1/test.go")
	
	// Execute routing
	ctx := context.Background()
	decision, err := router.RouteRequest(ctx, request)
	
	if err != nil {
		t.Fatalf("RouteRequest failed: %v", err)
	}
	
	if decision == nil {
		t.Fatal("Expected routing decision, got nil")
	}
	
	// Verify decision properties
	if decision.Method != "textDocument/definition" {
		t.Errorf("Expected method textDocument/definition, got %s", decision.Method)
	}
	
	if decision.TargetProject == nil {
		t.Error("Expected target project to be set")
	} else if decision.TargetProject.ID != "project1" {
		t.Errorf("Expected project1, got %s", decision.TargetProject.ID)
	}
	
	if decision.PrimaryClient == nil {
		t.Error("Expected primary client to be set")
	}
	
	if decision.Strategy == nil {
		t.Error("Expected strategy to be set")
	}
}

func TestRouteRequest_InvalidURI(t *testing.T) {
	logger := mcp.NewStructuredLogger(&mcp.LoggerConfig{Level: mcp.LogLevelInfo})
	router := NewSubProjectRequestRouter(logger)
	
	// Create request with invalid parameters
	request := &JSONRPCRequest{
		JSONRPC: JSONRPCVersion,
		ID:      1,
		Method:  "textDocument/definition",
		Params:  "invalid", // Should be object, not string
	}
	
	ctx := context.Background()
	_, err := router.RouteRequest(ctx, request)
	
	if err == nil {
		t.Error("Expected error for invalid request, got nil")
	}
	
	// Check error type
	if routingErr, ok := err.(*RoutingError); ok {
		if routingErr.Type != ErrorInvalidRequest {
			t.Errorf("Expected ErrorInvalidRequest, got %s", routingErr.Type)
		}
	} else {
		t.Error("Expected RoutingError type")
	}
}

func TestRouteRequest_ProjectResolutionFailure(t *testing.T) {
	// Setup mock resolver that fails
	resolver := &mockSubProjectResolver{
		err: &RoutingError{
			Type:    ErrorProjectResolution,
			Message: "Project not found",
		},
	}
	
	logger := mcp.NewStructuredLogger(&mcp.LoggerConfig{Level: mcp.LogLevelInfo})
	router := NewSubProjectRequestRouter(logger)
	router.SetResolver(resolver)
	
	request := createTestRequest("textDocument/definition", "file:///nonexistent/test.go")
	
	ctx := context.Background()
	_, err := router.RouteRequest(ctx, request)
	
	if err == nil {
		t.Error("Expected error for project resolution failure, got nil")
	}
	
	if routingErr, ok := err.(*RoutingError); ok {
		if routingErr.Type != ErrorProjectResolution {
			t.Errorf("Expected ErrorProjectResolution, got %s", routingErr.Type)
		}
	} else {
		t.Error("Expected RoutingError type")
	}
}

func TestRouteRequest_ClientSelectionFailure(t *testing.T) {
	// Setup resolver with project but client manager that fails
	projects := map[string]*SubProject{
		"project1": createTestProject("project1", "/path/to/project1", "go", []string{"go"}),
	}
	
	resolver := &mockSubProjectResolver{projects: projects}
	clientManager := &mockClientManager{
		err: &RoutingError{
			Type:    ErrorClientSelection,
			Message: "No client available",
		},
	}
	
	logger := mcp.NewStructuredLogger(&mcp.LoggerConfig{Level: mcp.LogLevelInfo})
	router := NewSubProjectRequestRouter(logger)
	router.SetResolver(resolver)
	router.SetClientManager(clientManager)
	
	request := createTestRequest("textDocument/definition", "file:///path/to/project1/test.go")
	
	ctx := context.Background()
	_, err := router.RouteRequest(ctx, request)
	
	if err == nil {
		t.Error("Expected error for client selection failure, got nil")
	}
	
	if routingErr, ok := err.(*RoutingError); ok {
		if routingErr.Type != ErrorClientSelection {
			t.Errorf("Expected ErrorClientSelection, got %s", routingErr.Type)
		}
	} else {
		t.Error("Expected RoutingError type")
	}
}

func TestSelectRoutingStrategy(t *testing.T) {
	logger := mcp.NewStructuredLogger(&mcp.LoggerConfig{Level: mcp.LogLevelInfo})
	router := NewSubProjectRequestRouter(logger)
	
	project := createTestProject("test", "/test", "go", []string{"go"})
	
	testCases := []struct {
		method   string
		expected string
	}{
		{"textDocument/definition", "single_target"},
		{"textDocument/references", "single_target"},
		{"textDocument/hover", "single_target"},
		{"textDocument/documentSymbol", "single_target"},
		{"textDocument/completion", "single_target"},
		{"workspace/symbol", "single_target"},
		{"unsupported/method", ""},
	}
	
	for _, tc := range testCases {
		t.Run(tc.method, func(t *testing.T) {
			strategy := router.SelectRoutingStrategy(tc.method, project)
			
			if tc.expected == "" {
				if strategy != nil {
					t.Errorf("Expected no strategy for %s, got %s", tc.method, strategy.Name())
				}
			} else {
				if strategy == nil {
					t.Errorf("Expected strategy %s for %s, got nil", tc.expected, tc.method)
				} else if strategy.Name() != tc.expected {
					t.Errorf("Expected strategy %s for %s, got %s", tc.expected, tc.method, strategy.Name())
				}
			}
		})
	}
}

func TestHandleRoutingFailure(t *testing.T) {
	logger := mcp.NewStructuredLogger(&mcp.LoggerConfig{Level: mcp.LogLevelInfo})
	router := NewSubProjectRequestRouter(logger)
	
	request := createTestRequest("textDocument/definition", "file:///test/file.go")
	originalError := &RoutingError{
		Type:    ErrorClientCommunication,
		Message: "Client communication failed",
	}
	
	ctx := context.Background()
	response, err := router.HandleRoutingFailure(ctx, request, originalError)
	
	// Since we don't have a functioning fallback handler in this test,
	// we expect an error response to be created
	if err != nil {
		t.Errorf("Expected error response to be created, got error: %v", err)
	}
	
	if response == nil {
		t.Error("Expected response to be created")
	} else {
		if response.Error == nil {
			t.Error("Expected error response")
		}
		
		if response.ID != request.ID {
			t.Errorf("Expected response ID %v, got %v", request.ID, response.ID)
		}
	}
}

func TestGetRoutingMetrics(t *testing.T) {
	logger := mcp.NewStructuredLogger(&mcp.LoggerConfig{Level: mcp.LogLevelInfo})
	router := NewSubProjectRequestRouter(logger)
	
	metrics := router.GetRoutingMetrics()
	
	if metrics == nil {
		t.Fatal("Expected metrics to be returned")
	}
	
	// Test initial state
	if metrics.TotalRequests != 0 {
		t.Errorf("Expected TotalRequests to be 0, got %d", metrics.TotalRequests)
	}
	
	if metrics.SuccessfulRequests != 0 {
		t.Errorf("Expected SuccessfulRequests to be 0, got %d", metrics.SuccessfulRequests)
	}
	
	if metrics.SuccessRate != 0 {
		t.Errorf("Expected SuccessRate to be 0, got %f", metrics.SuccessRate)
	}
	
	if metrics.StrategyUsage == nil {
		t.Error("Expected StrategyUsage to be initialized")
	}
}

func TestRouterShutdown(t *testing.T) {
	logger := mcp.NewStructuredLogger(&mcp.LoggerConfig{Level: mcp.LogLevelInfo})
	router := NewSubProjectRequestRouter(logger)
	
	ctx := context.Background()
	err := router.Shutdown(ctx)
	
	if err != nil {
		t.Errorf("Expected no error during shutdown, got %v", err)
	}
	
	// Verify router is shutdown by trying to route a request
	request := createTestRequest("textDocument/definition", "file:///test.go")
	_, err = router.RouteRequest(ctx, request)
	
	if err == nil {
		t.Error("Expected error when routing after shutdown")
	}
	
	if !strings.Contains(err.Error(), "shutdown") {
		t.Errorf("Expected shutdown error message, got: %v", err)
	}
}

func TestExtractFileURI(t *testing.T) {
	logger := mcp.NewStructuredLogger(&mcp.LoggerConfig{Level: mcp.LogLevelInfo})
	router := NewSubProjectRequestRouter(logger)
	
	testCases := []struct {
		name     string
		request  *JSONRPCRequest
		expected string
		hasError bool
	}{
		{
			name:     "valid textDocument request",
			request:  createTestRequest("textDocument/definition", "file:///test.go"),
			expected: "file:///test.go",
			hasError: false,
		},
		{
			name:     "workspace symbol request",
			request:  createTestRequest("workspace/symbol", ""),
			expected: "",
			hasError: false,
		},
		{
			name: "invalid request",
			request: &JSONRPCRequest{
				Method: "textDocument/definition",
				Params: "invalid",
			},
			expected: "",
			hasError: true,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uri, err := router.ExtractFileURI(tc.request)
			
			if tc.hasError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
				
				if uri != tc.expected {
					t.Errorf("Expected URI %s, got %s", tc.expected, uri)
				}
			}
		})
	}
}

// Benchmark tests

func BenchmarkRouteRequest(b *testing.B) {
	// Setup
	projects := map[string]*SubProject{
		"project1": createTestProject("project1", "/path/to/project1", "go", []string{"go"}),
	}
	
	resolver := &mockSubProjectResolver{projects: projects}
	
	clients := map[string]map[string]transport.LSPClient{
		"project1": {
			"go": &mockLSPClient{
				responses: map[string]string{
					"textDocument/definition": `[]`,
				},
				active: true,
			},
		},
	}
	
	clientManager := &mockClientManager{clients: clients}
	
	logger := mcp.NewStructuredLogger(&mcp.LoggerConfig{Level: mcp.LogLevelError}) // Minimize logging
	router := NewSubProjectRequestRouter(logger)
	router.SetResolver(resolver)
	router.SetClientManager(clientManager)
	
	request := createTestRequest("textDocument/definition", "file:///path/to/project1/test.go")
	ctx := context.Background()
	
	// Benchmark
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_, err := router.RouteRequest(ctx, request)
		if err != nil {
			b.Fatalf("RouteRequest failed: %v", err)
		}
	}
}

func BenchmarkSelectRoutingStrategy(b *testing.B) {
	logger := mcp.NewStructuredLogger(&mcp.LoggerConfig{Level: mcp.LogLevelError})
	router := NewSubProjectRequestRouter(logger)
	
	project := createTestProject("test", "/test", "go", []string{"go"})
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		strategy := router.SelectRoutingStrategy("textDocument/definition", project)
		if strategy == nil {
			b.Fatal("Expected strategy, got nil")
		}
	}
}

func BenchmarkExtractFileURI(b *testing.B) {
	logger := mcp.NewStructuredLogger(&mcp.LoggerConfig{Level: mcp.LogLevelError})
	router := NewSubProjectRequestRouter(logger)
	
	request := createTestRequest("textDocument/definition", "file:///test.go")
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_, err := router.ExtractFileURI(request)
		if err != nil {
			b.Fatalf("ExtractFileURI failed: %v", err)
		}
	}
}