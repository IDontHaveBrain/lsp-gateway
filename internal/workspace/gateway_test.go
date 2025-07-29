package workspace

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lsp-gateway/internal/config"
)

func TestWorkspaceGateway_NewWorkspaceGateway(t *testing.T) {
	gateway := NewWorkspaceGateway()
	if gateway == nil {
		t.Fatal("NewWorkspaceGateway returned nil")
	}
}

func TestWorkspaceGateway_Initialize(t *testing.T) {
	gateway := NewWorkspaceGateway()
	
	workspaceConfig := &WorkspaceConfig{
		Workspace: WorkspaceInfo{
			RootPath: "/test/workspace",
		},
		Servers: map[string]*config.ServerConfig{
			"gopls": {
				Name:      "gopls",
				Languages: []string{"go"},
				Command:   "gopls",
				Args:      []string{"serve"},
				Transport: "stdio",
			},
		},
		Logging: LoggingConfig{
			Level: "info",
		},
	}
	
	gatewayConfig := &WorkspaceGatewayConfig{
		WorkspaceRoot: "/test/workspace",
		EnableLogging: true,
		ExtensionMapping: map[string]string{
			"go": "go",
		},
	}
	
	ctx := context.Background()
	err := gateway.Initialize(ctx, workspaceConfig, gatewayConfig)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
}

func TestWorkspaceGateway_ExtractLanguageFromURI(t *testing.T) {
	gateway := NewWorkspaceGateway().(*workspaceGateway)
	
	// Initialize with default extension mappings
	gateway.addDefaultExtensionMappings()
	
	tests := []struct {
		name     string
		uri      string
		expected string
		hasError bool
	}{
		{
			name:     "Go file",
			uri:      "file:///path/to/main.go",
			expected: "go",
			hasError: false,
		},
		{
			name:     "Python file",
			uri:      "file:///path/to/script.py",
			expected: "python",
			hasError: false,
		},
		{
			name:     "TypeScript file",
			uri:      "file:///path/to/component.ts",
			expected: "typescript",
			hasError: false,
		},
		{
			name:     "No extension",
			uri:      "file:///path/to/README",
			expected: "",
			hasError: true,
		},
		{
			name:     "Unsupported extension",
			uri:      "file:///path/to/file.xyz",
			expected: "",
			hasError: true,
		},
		{
			name:     "Empty URI",
			uri:      "",
			expected: "",
			hasError: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := gateway.extractLanguageFromURI(tt.uri)
			
			if tt.hasError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("Expected %s, got %s", tt.expected, result)
				}
			}
		})
	}
}

func TestWorkspaceGateway_ParseLogLevel(t *testing.T) {
	gateway := NewWorkspaceGateway().(*workspaceGateway)
	
	tests := []struct {
		input    string
		expected int // Using int for easier comparison
	}{
		{"trace", 0},
		{"debug", 1},
		{"info", 2},
		{"warn", 3},
		{"error", 4},
		{"fatal", 5},
		{"INFO", 2},  // Case insensitive
		{"DEBUG", 1}, // Case insensitive
		{"unknown", 2}, // Default to info
		{"", 2},        // Default to info
	}
	
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := gateway.parseLogLevel(tt.input)
			if int(result) != tt.expected {
				t.Errorf("parseLogLevel(%s) = %d, expected %d", tt.input, int(result), tt.expected)
			}
		})
	}
}

func TestWorkspaceGateway_Health(t *testing.T) {
	gateway := NewWorkspaceGateway()
	
	workspaceConfig := &WorkspaceConfig{
		Workspace: WorkspaceInfo{
			RootPath: "/test/workspace",
		},
		Servers: map[string]*config.ServerConfig{},
		Logging: LoggingConfig{
			Level: "info",
		},
	}
	
	gatewayConfig := &WorkspaceGatewayConfig{
		WorkspaceRoot: "/test/workspace",
		EnableLogging: false,
	}
	
	ctx := context.Background()
	err := gateway.Initialize(ctx, workspaceConfig, gatewayConfig)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)  
	}
	
	health := gateway.Health()
	
	if health.WorkspaceRoot != "/test/workspace" {
		t.Errorf("Expected workspace root '/test/workspace', got '%s'", health.WorkspaceRoot)
	}
	
	if health.ActiveClients != 0 {
		t.Errorf("Expected 0 active clients, got %d", health.ActiveClients)
	}
	
	if health.IsHealthy {
		t.Error("Expected gateway to be unhealthy with no clients")
	}
	
	if health.LastCheck.IsZero() {
		t.Error("Expected LastCheck to be set")
	}
}

func TestWorkspaceGateway_DefaultExtensionMappings(t *testing.T) {
	gateway := NewWorkspaceGateway().(*workspaceGateway)
	gateway.addDefaultExtensionMappings()
	
	expectedMappings := map[string]string{
		"go":   "go",
		"py":   "python",
		"js":   "javascript",
		"ts":   "typescript",
		"java": "java",
		"rs":   "rust",
	}
	
	for ext, expectedLang := range expectedMappings {
		if actualLang, exists := gateway.extensionMapping[ext]; !exists {
			t.Errorf("Extension %s not found in mapping", ext)
		} else if actualLang != expectedLang {
			t.Errorf("Extension %s mapped to %s, expected %s", ext, actualLang, expectedLang)
		}
	}
}

// Test helpers for creating mock configurations
func createGatewayTestWorkspaceConfig(tempDir string) *WorkspaceConfig {
	return &WorkspaceConfig{
		Workspace: WorkspaceInfo{
			RootPath: tempDir,
		},
		Servers: map[string]*config.ServerConfig{
			"gopls": {
				Name:      "gopls",
				Languages: []string{"go"},
				Command:   "gopls",
				Args:      []string{"serve"},
				Transport: "stdio",
			},
			"pylsp": {
				Name:      "pylsp",
				Languages: []string{"python"},
				Command:   "pylsp",
				Args:      []string{},
				Transport: "stdio",
			},
		},
		Logging: LoggingConfig{
			Level: "info",
		},
	}
}

func createGatewayTestGatewayConfig(tempDir string) *WorkspaceGatewayConfig {
	return &WorkspaceGatewayConfig{
		WorkspaceRoot: tempDir,
		EnableLogging: true,
		Timeout:       30 * time.Second,
		ExtensionMapping: map[string]string{
			"go": "go",
			"py": "python",
			"js": "javascript",
			"ts": "typescript",
		},
	}
}

func createGatewayTestWorkspace(t *testing.T) (string, func()) {
	tempDir, err := os.MkdirTemp("", "lspg-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	
	// Create sample project structure
	projectDirs := []string{
		"project1",
		"project1/src", 
		"project2/backend",
		"project2/frontend",
		"shared/utils",
	}
	
	for _, dir := range projectDirs {
		if err := os.MkdirAll(filepath.Join(tempDir, dir), 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}
	
	// Create sample files
	testFiles := map[string]string{
		"project1/main.go":              "package main\nfunc main() {}",
		"project1/go.mod":               "module project1\ngo 1.20",
		"project1/src/handler.go":       "package src\nfunc Handler() {}",
		"project2/backend/app.py":       "def main():\n    pass",
		"project2/backend/requirements.txt": "flask==2.0.1",
		"project2/frontend/app.js":      "console.log('hello');",
		"project2/frontend/package.json": `{"name": "frontend"}`,
		"shared/utils/helper.py":        "def helper(): pass",
		"README.md":                     "# Test Workspace",
	}
	
	for filePath, content := range testFiles {
		fullPath := filepath.Join(tempDir, filePath)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", filePath, err)
		}
	}
	
	cleanup := func() {
		os.RemoveAll(tempDir)
	}
	
	return tempDir, cleanup
}

// COMPREHENSIVE UNIT TESTS FOR SUB-PROJECT ROUTING

func TestWorkspaceGateway_SubProjectRouting(t *testing.T) {
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
	
	// Test GetSubProjects
	projects := gateway.GetSubProjects()
	if len(projects) == 0 {
		t.Error("Expected sub-projects to be detected")
	}
	
	// Test RefreshSubProjects
	err = gateway.RefreshSubProjects(ctx)
	if err != nil {
		t.Errorf("RefreshSubProjects failed: %v", err)
	}
	
	// Test GetRoutingMetrics
	metrics := gateway.GetRoutingMetrics()
	if metrics == nil {
		t.Error("Expected routing metrics to be available")
	}
	
	// Test that metrics are properly initialized
	if metrics.StrategyUsage == nil {
		t.Error("Expected strategy usage metrics to be initialized")
	}
}

// TestEnhancedWorkspaceGateway_SubProjectRouting tests the new sub-project routing functionality
func TestEnhancedWorkspaceGateway_SubProjectRouting(t *testing.T) {
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
	
	// Test EnableSubProjectRouting
	gateway.EnableSubProjectRouting(true)
	
	// Test GetSubProjects (should not panic even if empty)
	projects := gateway.GetSubProjects()
	t.Logf("Detected %d sub-projects", len(projects))
	
	// Test GetRoutingMetrics (should not panic)
	metrics := gateway.GetRoutingMetrics()
	if metrics == nil {
		t.Error("GetRoutingMetrics returned nil")
	}
	
	// Test Health method includes sub-project info
	health := gateway.Health()
	if health.SubProjects < 0 {
		t.Error("Health reported negative sub-project count")
	}
}

// TestEnhancedWorkspaceGateway_FallbackRouting tests fallback from sub-project to legacy routing
func TestEnhancedWorkspaceGateway_FallbackRouting(t *testing.T) {
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
	
	// Enable sub-project routing (will fail gracefully to legacy)
	gateway.EnableSubProjectRouting(true)
	
	// Create a mock HTTP request for textDocument/hover
	reqBody := `{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "textDocument/hover",
		"params": {
			"textDocument": {"uri": "file://` + tempDir + `/project1/main.go"},
			"position": {"line": 0, "character": 0}
		}
	}`
	
	req := httptest.NewRequest("POST", "/jsonrpc", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	
	w := httptest.NewRecorder()
	
	// This should attempt sub-project routing, fail gracefully, then use legacy routing
	gateway.HandleJSONRPC(w, req)
	
	// Should get proper JSON-RPC response (either success or method not found)
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
	
	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected application/json content type, got %s", contentType)
	}
}

// TestEnhancedWorkspaceGateway_BackwardCompatibility tests that existing functionality still works
func TestEnhancedWorkspaceGateway_BackwardCompatibility(t *testing.T) {
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
	client, exists := gateway.GetClient("go")
	t.Logf("Legacy GetClient('go'): exists=%v, client=%v", exists, client != nil)
	
	// Test with sub-project routing disabled
	gateway.EnableSubProjectRouting(false)
	
	// Should work exactly like before
	reqBody := `{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "textDocument/hover",  
		"params": {
			"textDocument": {"uri": "file://` + tempDir + `/project1/main.go"},
			"position": {"line": 0, "character": 0}
		}
	}`
	
	req := httptest.NewRequest("POST", "/jsonrpc", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	
	w := httptest.NewRecorder()
	gateway.HandleJSONRPC(w, req)
	
	// Should work with legacy routing only
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}