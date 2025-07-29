package workspace

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestWorkspaceGateway_GetSubProjectClient(t *testing.T) {
	tempDir, cleanup := createGatewayTestWorkspace(t)
	defer cleanup()
	
	gateway := NewWorkspaceGateway()
	workspaceConfig := createGatewayTestWorkspaceConfig(tempDir)
	gatewayConfig := createGatewayTestGatewayConfig(tempDir)
	
	ctx := context.Background()
	err := gateway.Initialize(ctx, workspaceConfig, gatewayConfig)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	
	// Test getting client for non-existent project
	_, err = gateway.GetSubProjectClient("non-existent", "go")
	if err == nil {
		t.Error("Expected error for non-existent project")
	}
	
	// Test getting client for non-existent language
	_, err = gateway.GetSubProjectClient("project1", "unknown")
	if err == nil {
		t.Error("Expected error for unknown language")
	}
}

func TestWorkspaceGateway_EnhancedHealth(t *testing.T) {
	tempDir, cleanup := createGatewayTestWorkspace(t)
	defer cleanup()
	
	gateway := NewWorkspaceGateway()
	workspaceConfig := createGatewayTestWorkspaceConfig(tempDir)
	gatewayConfig := createGatewayTestGatewayConfig(tempDir)
	
	ctx := context.Background()
	err := gateway.Initialize(ctx, workspaceConfig, gatewayConfig)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	
	health := gateway.Health()
	
	// Test enhanced health fields
	if health.SubProjectClients == nil {
		t.Error("Expected SubProjectClients to be initialized")
	}
	
	if health.RoutingMetrics == nil {
		t.Error("Expected RoutingMetrics to be included in health")
	}
	
	// Test sub-project count
	if health.SubProjects < 0 {
		t.Error("Expected SubProjects count to be non-negative")
	}
	
	// Health should consider sub-projects when determining overall health
	if health.SubProjects > 0 {
		// With sub-projects, gateway can be healthy even without legacy clients
		if !health.IsHealthy && len(health.Errors) == 0 {
			t.Error("Expected gateway to be healthy with detected sub-projects")
		}
	}
}

func TestWorkspaceGateway_FallbackStrategies(t *testing.T) {
	tempDir, cleanup := createGatewayTestWorkspace(t)
	defer cleanup()
	
	gateway := NewWorkspaceGateway().(*workspaceGateway)
	workspaceConfig := createGatewayTestWorkspaceConfig(tempDir)
	gatewayConfig := createGatewayTestGatewayConfig(tempDir)
	
	ctx := context.Background()
	err := gateway.Initialize(ctx, workspaceConfig, gatewayConfig)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	
	// Test fallback handler initialization
	if gateway.fallbackHandler == nil {
		t.Error("Expected fallback handler to be initialized")
	}
	
	// Test enhanced routing initialization
	if gateway.requestRouter == nil {
		t.Error("Expected request router to be initialized")
	}
	
	if gateway.subProjectResolver == nil {
		t.Error("Expected sub-project resolver to be initialized")
	}
	
	if gateway.clientManager == nil {
		t.Error("Expected client manager to be initialized")
	}
}

func TestWorkspaceGateway_JSONRPCHandling(t *testing.T) {
	tempDir, cleanup := createGatewayTestWorkspace(t)
	defer cleanup()
	
	gateway := NewWorkspaceGateway()
	workspaceConfig := createGatewayTestWorkspaceConfig(tempDir)
	gatewayConfig := createGatewayTestGatewayConfig(tempDir)
	
	ctx := context.Background()
	err := gateway.Initialize(ctx, workspaceConfig, gatewayConfig)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	
	tests := []struct {
		name           string
		method         string
		body           interface{}
		expectedStatus int
		expectedError  bool
	}{
		{
			name:           "Invalid method (GET)",
			method:         "GET",
			body:           nil,
			expectedStatus: 200, // JSON-RPC errors return 200 with error in body
			expectedError:  true,
		},
		{
			name:   "Valid JSON-RPC request",
			method: "POST",
			body: map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"method":  "textDocument/hover",
				"params": map[string]interface{}{
					"textDocument": map[string]interface{}{
						"uri": fmt.Sprintf("file://%s/project1/main.go", tempDir),
					},
					"position": map[string]interface{}{
						"line":      0,
						"character": 0,
					},
				},
			},
			expectedStatus: 200,
			expectedError:  false, // Might fail due to no actual LSP server, but request should be processed
		},
		{
			name:           "Malformed JSON",
			method:         "POST",
			body:           "invalid json",
			expectedStatus: 200,
			expectedError:  true,
		},
		{
			name:   "Missing method field",
			method: "POST",
			body: map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"params":  map[string]interface{}{},
			},
			expectedStatus: 200,
			expectedError:  true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body []byte
			if tt.body != nil {
				if str, ok := tt.body.(string); ok {
					body = []byte(str)
				} else {
					body, _ = json.Marshal(tt.body)
				}
			}
			
			req := httptest.NewRequest(tt.method, "/jsonrpc", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			
			w := httptest.NewRecorder()
			gateway.HandleJSONRPC(w, req)
			
			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
			
			var response map[string]interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				if !tt.expectedError {
					t.Errorf("Failed to parse response JSON: %v", err)
				}
				return
			}
			
			hasError := response["error"] != nil
			if hasError != tt.expectedError {
				t.Errorf("Expected error: %v, got error: %v", tt.expectedError, hasError)
			}
		})
	}
}

func TestWorkspaceGateway_ClientLifecycle(t *testing.T) {
	tempDir, cleanup := createGatewayTestWorkspace(t)
	defer cleanup()
	
	gateway := NewWorkspaceGateway()
	workspaceConfig := createGatewayTestWorkspaceConfig(tempDir)
	gatewayConfig := createGatewayTestGatewayConfig(tempDir)
	
	ctx := context.Background()
	
	// Test initialization
	err := gateway.Initialize(ctx, workspaceConfig, gatewayConfig)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	
	// Test start (note: will fail to actually start LSP servers, but should handle gracefully)
	err = gateway.Start(ctx)
	if err != nil {
		t.Logf("Start failed (expected in test environment): %v", err)
	}
	
	// Test health after start attempt
	health := gateway.Health()
	if health.WorkspaceRoot != tempDir {
		t.Errorf("Expected workspace root %s, got %s", tempDir, health.WorkspaceRoot)
	}
	
	// Test stop
	err = gateway.Stop()
	if err != nil {
		t.Errorf("Stop failed: %v", err)
	}
	
	// Test double stop (should be safe)
	err = gateway.Stop()
	if err != nil {
		t.Errorf("Double stop failed: %v", err)
	}
}

func TestWorkspaceGateway_ConcurrentAccess(t *testing.T) {
	tempDir, cleanup := createGatewayTestWorkspace(t)
	defer cleanup()
	
	gateway := NewWorkspaceGateway()
	workspaceConfig := createGatewayTestWorkspaceConfig(tempDir)
	gatewayConfig := createGatewayTestGatewayConfig(tempDir)
	
	ctx := context.Background()
	err := gateway.Initialize(ctx, workspaceConfig, gatewayConfig)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	
	// Test concurrent access to gateway methods
	numGoroutines := 10
	numOperations := 20
	
	var wg sync.WaitGroup
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				// Test concurrent health checks
				health := gateway.Health()
				if health.WorkspaceRoot != tempDir {
					t.Errorf("Concurrent health check failed")
				}
				
				// Test concurrent sub-project access
				projects := gateway.GetSubProjects()
				if projects == nil {
					t.Errorf("Concurrent GetSubProjects failed")
				}
				
				// Test concurrent metrics access
				metrics := gateway.GetRoutingMetrics()
				if metrics == nil {
					t.Errorf("Concurrent GetRoutingMetrics failed")
				}
				
				// Test concurrent legacy client access
				_, exists := gateway.GetClient("go")
				if exists {
					// Client exists, no error expected
				}
			}
		}()
	}
	
	wg.Wait()
}

func TestWorkspaceGateway_ErrorHandling(t *testing.T) {
	gateway := NewWorkspaceGateway()
	
	// Test operations without initialization
	health := gateway.Health()
	if health.IsHealthy {
		t.Error("Expected gateway to be unhealthy without initialization")
	}
	
	projects := gateway.GetSubProjects()
	if projects != nil {
		t.Error("Expected no projects without initialization")
	}
	
	metrics := gateway.GetRoutingMetrics()
	if metrics == nil {
		t.Error("Expected basic metrics even without initialization")
	}
	
	// Test operations with invalid workspace
	workspaceConfig := createGatewayTestWorkspaceConfig("/non/existent/path")
	gatewayConfig := createGatewayTestGatewayConfig("/non/existent/path")
	
	ctx := context.Background()
	err := gateway.Initialize(ctx, workspaceConfig, gatewayConfig)
	if err != nil {
		t.Logf("Initialize with invalid path failed (expected): %v", err)
	}
	
	// Test double initialization
	err = gateway.Initialize(ctx, workspaceConfig, gatewayConfig)
	if err == nil {
		t.Error("Expected error on double initialization")
	}
}

func TestWorkspaceGateway_BackwardCompatibility(t *testing.T) {
	tempDir, cleanup := createGatewayTestWorkspace(t)
	defer cleanup()
	
	gateway := NewWorkspaceGateway()
	workspaceConfig := createGatewayTestWorkspaceConfig(tempDir)
	gatewayConfig := createGatewayTestGatewayConfig(tempDir)
	
	ctx := context.Background()
	err := gateway.Initialize(ctx, workspaceConfig, gatewayConfig)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	
	// Test legacy GetClient method still works
	_, exists := gateway.GetClient("go")
	// Should not panic, existence depends on whether LSP server actually started
	_ = exists
	
	// Test legacy extension mapping still works
	gw := gateway.(*workspaceGateway)
	language, err := gw.extractLanguageFromURI(fmt.Sprintf("file://%s/project1/main.go", tempDir))
	if err != nil {
		t.Errorf("Legacy language extraction failed: %v", err)
	}
	if language != "go" {
		t.Errorf("Expected language 'go', got '%s'", language)
	}
	
	// Test legacy health response format
	health := gateway.Health()
	if health.WorkspaceRoot == "" {
		t.Error("Legacy health field WorkspaceRoot should be set")
	}
	if health.ClientStatuses == nil {
		t.Error("Legacy health field ClientStatuses should be initialized")
	}
	if health.LastCheck.IsZero() {
		t.Error("Legacy health field LastCheck should be set")
	}
}

// INTEGRATION TESTS

func TestWorkspaceGateway_MonorepoSupport(t *testing.T) {
	tempDir, cleanup := createGatewayTestWorkspace(t)
	defer cleanup()
	
	gateway := NewWorkspaceGateway()
	workspaceConfig := createGatewayTestWorkspaceConfig(tempDir)
	gatewayConfig := createGatewayTestGatewayConfig(tempDir)
	
	ctx := context.Background()
	err := gateway.Initialize(ctx, workspaceConfig, gatewayConfig)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	
	projects := gateway.GetSubProjects()
	
	// Should detect multiple projects in the monorepo structure
	if len(projects) < 2 {
		t.Errorf("Expected at least 2 sub-projects in monorepo, got %d", len(projects))
	}
	
	// Test that each project has proper metadata
	for _, project := range projects {
		if project.ID == "" {
			t.Error("Project should have an ID")
		}
		if project.Root == "" {
			t.Error("Project should have a root path")
		}
		if len(project.Languages) == 0 {
			t.Error("Project should have detected languages")
		}
	}
}

func TestWorkspaceGateway_LSPMethodSupport(t *testing.T) {
	tempDir, cleanup := createGatewayTestWorkspace(t)
	defer cleanup()
	
	gateway := NewWorkspaceGateway()
	workspaceConfig := createGatewayTestWorkspaceConfig(tempDir)
	gatewayConfig := createGatewayTestGatewayConfig(tempDir)
	
	ctx := context.Background()
	err := gateway.Initialize(ctx, workspaceConfig, gatewayConfig)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	
	// Test all 6 supported LSP methods with mock requests
	supportedMethods := []string{
		"textDocument/definition",
		"textDocument/references", 
		"textDocument/hover",
		"textDocument/documentSymbol",
		"workspace/symbol",
		"textDocument/completion",
	}
	
	for _, method := range supportedMethods {
		t.Run(method, func(t *testing.T) {
			var params interface{}
			
			switch method {
			case "workspace/symbol":
				params = map[string]interface{}{
					"query": "test",
				}
			default:
				params = map[string]interface{}{
					"textDocument": map[string]interface{}{
						"uri": fmt.Sprintf("file://%s/project1/main.go", tempDir),
					},
					"position": map[string]interface{}{
						"line":      0,
						"character": 0,
					},
				}
			}
			
			body := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      1,
				"method":  method,
				"params":  params,
			}
			
			bodyBytes, _ := json.Marshal(body)
			req := httptest.NewRequest("POST", "/jsonrpc", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			
			w := httptest.NewRecorder()
			gateway.HandleJSONRPC(w, req)
			
			// Should not return method not found error
			var response map[string]interface{}
			json.Unmarshal(w.Body.Bytes(), &response)
			
			if errorObj, exists := response["error"]; exists {
				if errorMap, ok := errorObj.(map[string]interface{}); ok {
					if code, ok := errorMap["code"].(float64); ok && code == -32601 {
						t.Errorf("Method %s should be supported but got method not found error", method)
					}
				}
			}
		})
	}
}

// PERFORMANCE TESTS

func BenchmarkWorkspaceGateway_Health(b *testing.B) {
	tempDir, cleanup := createGatewayTestWorkspace(&testing.T{}) 
	defer cleanup()
	
	gateway := NewWorkspaceGateway()
	workspaceConfig := createGatewayTestWorkspaceConfig(tempDir)
	gatewayConfig := createGatewayTestGatewayConfig(tempDir)
	
	ctx := context.Background()
	gateway.Initialize(ctx, workspaceConfig, gatewayConfig)
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		health := gateway.Health()
		_ = health.IsHealthy
	}
}

func BenchmarkWorkspaceGateway_GetSubProjects(b *testing.B) {
	tempDir, cleanup := createGatewayTestWorkspace(&testing.T{})
	defer cleanup()
	
	gateway := NewWorkspaceGateway()
	workspaceConfig := createGatewayTestWorkspaceConfig(tempDir)
	gatewayConfig := createGatewayTestGatewayConfig(tempDir)
	
	ctx := context.Background()
	gateway.Initialize(ctx, workspaceConfig, gatewayConfig)
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		projects := gateway.GetSubProjects()
		_ = len(projects)
	}
}

func BenchmarkWorkspaceGateway_GetRoutingMetrics(b *testing.B) {
	tempDir, cleanup := createGatewayTestWorkspace(&testing.T{})
	defer cleanup()
	
	gateway := NewWorkspaceGateway()
	workspaceConfig := createGatewayTestWorkspaceConfig(tempDir)
	gatewayConfig := createGatewayTestGatewayConfig(tempDir)
	
	ctx := context.Background()
	gateway.Initialize(ctx, workspaceConfig, gatewayConfig)
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		metrics := gateway.GetRoutingMetrics()
		_ = metrics.RequestCount
	}
}

func BenchmarkWorkspaceGateway_ConcurrentHealth(b *testing.B) {
	tempDir, cleanup := createGatewayTestWorkspace(&testing.T{})
	defer cleanup()
	
	gateway := NewWorkspaceGateway()
	workspaceConfig := createGatewayTestWorkspaceConfig(tempDir)
	gatewayConfig := createGatewayTestGatewayConfig(tempDir)
	
	ctx := context.Background()
	gateway.Initialize(ctx, workspaceConfig, gatewayConfig)
	
	b.ResetTimer()
	
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			health := gateway.Health()
			_ = health.IsHealthy
		}
	})
}

// TEST MEMORY USAGE

func TestWorkspaceGateway_MemoryUsage(t *testing.T) {
	tempDir, cleanup := createGatewayTestWorkspace(t)
	defer cleanup()
	
	gateway := NewWorkspaceGateway()
	workspaceConfig := createGatewayTestWorkspaceConfig(tempDir)
	gatewayConfig := createGatewayTestGatewayConfig(tempDir)
	
	ctx := context.Background()
	err := gateway.Initialize(ctx, workspaceConfig, gatewayConfig)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	
	// Perform many operations to test for memory leaks
	for i := 0; i < 1000; i++ {
		_ = gateway.Health()
		_ = gateway.GetSubProjects()
		_ = gateway.GetRoutingMetrics()
		
		// Refresh projects periodically
		if i%100 == 0 {
			gateway.RefreshSubProjects(ctx)
		}
	}
	
	// No direct memory measurement in this test, but ensures no panics
	// In real testing, you would use runtime.MemStats to check for leaks
	t.Log("Memory usage test completed without panics")
}