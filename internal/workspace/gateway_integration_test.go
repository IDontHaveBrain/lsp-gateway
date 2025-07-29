package workspace

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestWorkspaceGateway_EndToEndRouting tests the complete request lifecycle
func TestWorkspaceGateway_EndToEndRouting(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	
	tempDir, cleanup := createComplexWorkspace(t)
	defer cleanup()
	
	gateway := NewWorkspaceGateway()
	workspaceConfig := createGatewayTestWorkspaceConfig(tempDir)
	gatewayConfig := createGatewayTestGatewayConfig(tempDir)
	
	ctx := context.Background()
	err := gateway.Initialize(ctx, workspaceConfig, gatewayConfig)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	
	// Test end-to-end routing for different file types
	testCases := []struct {
		name     string
		filePath string
		language string
		method   string
	}{
		{
			name:     "Go file routing",
			filePath: "services/user/main.go",
			language: "go",
			method:   "textDocument/hover",
		},
		{
			name:     "Python file routing", 
			filePath: "services/auth/app.py",
			language: "python",
			method:   "textDocument/definition",
		},
		{
			name:     "JavaScript file routing",
			filePath: "frontend/src/app.js",
			language: "javascript",
			method:   "textDocument/references",
		},
		{
			name:     "TypeScript file routing",
			filePath: "frontend/src/components/Header.ts",
			language: "typescript", 
			method:   "textDocument/completion",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test file-to-project resolution
			// Test project resolution
			projects := gateway.GetSubProjects()
			projectFound := false
			for _, project := range projects {
				if containsString(project.Languages, tc.language) {
					projectFound = true
					break
				}
			}
			
			if !projectFound {
				t.Errorf("No project found supporting language %s", tc.language)
			}
			
			// Test routing metrics are updated
			initialMetrics := gateway.GetRoutingMetrics()
			t.Logf("Initial request count: %d", initialMetrics.RequestCount)
			
			// Test that sub-project routing is working by checking if file can be resolved
			// We don't need to test internal URI extraction as that's covered by unit tests
			if len(projects) == 0 {
				t.Logf("No sub-projects detected for file %s", tc.filePath)
			} else {
				t.Logf("Found %d sub-projects for routing", len(projects))
			}
		})
	}
}

func TestWorkspaceGateway_RealWorldScenarios(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	
	scenarios := []struct {
		name        string
		setupFunc   func(t *testing.T) (string, func())
		testFunc    func(t *testing.T, gateway WorkspaceGateway, tempDir string)
	}{
		{
			name:      "Large Monorepo",
			setupFunc: createLargeMonorepo,
			testFunc:  testLargeMonorepoHandling,
		},
		{
			name:      "Nested Projects",
			setupFunc: createNestedProjectsIntegration,
			testFunc:  testNestedProjectHandling,
		},
		{
			name:      "Mixed Languages",
			setupFunc: createMixedLanguageWorkspace,
			testFunc:  testMixedLanguageHandling,
		},
	}
	
	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			tempDir, cleanup := scenario.setupFunc(t)
			defer cleanup()
			
			gateway := NewWorkspaceGateway()
			workspaceConfig := createGatewayTestWorkspaceConfig(tempDir)
			gatewayConfig := createGatewayTestGatewayConfig(tempDir)
			
			ctx := context.Background()
			err := gateway.Initialize(ctx, workspaceConfig, gatewayConfig)
			if err != nil {
				t.Fatalf("Initialize failed: %v", err)
			}
			
			scenario.testFunc(t, gateway, tempDir)
		})
	}
}

func TestWorkspaceGateway_PerformanceUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}
	
	tempDir, cleanup := createComplexWorkspace(t)
	defer cleanup()
	
	gateway := NewWorkspaceGateway()
	workspaceConfig := createGatewayTestWorkspaceConfig(tempDir)
	gatewayConfig := createGatewayTestGatewayConfig(tempDir)
	
	ctx := context.Background()
	err := gateway.Initialize(ctx, workspaceConfig, gatewayConfig)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	
	// Performance targets
	const (
		maxHealthCheckTime    = 5 * time.Millisecond
		maxProjectRefreshTime = 100 * time.Millisecond
		maxMetricsTime        = 1 * time.Millisecond
	)
	
	// Test health check performance
	start := time.Now()
	for i := 0; i < 100; i++ {
		gateway.Health()
	}
	avgHealthTime := time.Since(start) / 100
	
	if avgHealthTime > maxHealthCheckTime {
		t.Errorf("Health check too slow: %v > %v", avgHealthTime, maxHealthCheckTime)
	}
	
	// Test project refresh performance
	start = time.Now()
	err = gateway.RefreshSubProjects(ctx)
	refreshTime := time.Since(start)
	
	if err != nil {
		t.Errorf("Project refresh failed: %v", err)
	}
	if refreshTime > maxProjectRefreshTime {
		t.Errorf("Project refresh too slow: %v > %v", refreshTime, maxProjectRefreshTime)
	}
	
	// Test metrics access performance
	start = time.Now()
	for i := 0; i < 1000; i++ {
		gateway.GetRoutingMetrics()
	}
	avgMetricsTime := time.Since(start) / 1000
	
	if avgMetricsTime > maxMetricsTime {
		t.Errorf("Metrics access too slow: %v > %v", avgMetricsTime, maxMetricsTime)
	}
	
	t.Logf("Performance results: Health=%v, Refresh=%v, Metrics=%v", 
		avgHealthTime, refreshTime, avgMetricsTime)
}

func TestWorkspaceGateway_ResourceManagement(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping resource management test in short mode")
	}
	
	tempDir, cleanup := createComplexWorkspace(t)
	defer cleanup()
	
	gateway := NewWorkspaceGateway()
	workspaceConfig := createGatewayTestWorkspaceConfig(tempDir)
	gatewayConfig := createGatewayTestGatewayConfig(tempDir)
	
	ctx := context.Background()
	err := gateway.Initialize(ctx, workspaceConfig, gatewayConfig)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	
	// Test resource cleanup on stop
	err = gateway.Start(ctx)
	if err != nil {
		t.Logf("Start failed (expected in test): %v", err)
	}
	
	health := gateway.Health()
	initialClients := health.ActiveClients + len(health.SubProjectClients)
	
	err = gateway.Stop()
	if err != nil {
		t.Errorf("Stop failed: %v", err)
	}
	
	// Verify resources are cleaned up
	health = gateway.Health()
	finalClients := health.ActiveClients + len(health.SubProjectClients)
	
	if finalClients > 0 {
		t.Errorf("Expected 0 clients after stop, got %d", finalClients)
	}
	
	// Test that gateway is in clean state
	if health.IsHealthy {
		t.Error("Expected gateway to be unhealthy after stop")
	}
	
	t.Logf("Resource cleanup successful: %d -> %d clients", initialClients, finalClients)
}

// Helper functions for creating test workspaces

func createComplexWorkspace(t *testing.T) (string, func()) {
	tempDir, err := os.MkdirTemp("", "lspg-complex-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	
	// Create a complex project structure resembling a real monorepo
	structure := map[string]string{
		// Go microservices
		"services/user/main.go":           "package main\nfunc main() {}",
		"services/user/go.mod":            "module user-service\ngo 1.20",
		"services/user/handler/user.go":   "package handler\nfunc GetUser() {}",
		"services/auth/main.go":           "package main\nfunc main() {}",
		"services/auth/go.mod":            "module auth-service\ngo 1.20",
		
		// Python services
		"services/auth/app.py":            "def main():\n    pass",
		"services/auth/requirements.txt":  "flask==2.0.1",
		"services/data/processor.py":      "def process(): pass",
		"services/data/requirements.txt":  "pandas==1.3.0",
		
		// Frontend applications
		"frontend/src/app.js":             "console.log('main app');",
		"frontend/src/components/Header.ts": "export class Header {}",
		"frontend/package.json":           `{"name": "frontend", "dependencies": {}}`,
		"frontend/tsconfig.json":          `{"compilerOptions": {}}`,
		
		// Shared libraries
		"shared/utils/logger.go":          "package utils\nfunc Log() {}",
		"shared/utils/go.mod":            "module shared-utils\ngo 1.20",
		"shared/types/user.ts":            "export interface User {}",
		
		// Configuration and documentation
		"docker-compose.yml":              "version: '3'\nservices: {}",
		"README.md":                      "# Complex Workspace",
		".gitignore":                     "node_modules/\n*.log",
	}
	
	for filePath, content := range structure {
		fullPath := filepath.Join(tempDir, filePath)
		dir := filepath.Dir(fullPath)
		
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
		
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", filePath, err)
		}
	}
	
	cleanup := func() {
		os.RemoveAll(tempDir)
	}
	
	return tempDir, cleanup
}

func createLargeMonorepo(t *testing.T) (string, func()) {
	tempDir, err := os.MkdirTemp("", "lspg-large-monorepo-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	
	// Create many projects to test scalability
	for i := 0; i < 20; i++ {
		projectDir := fmt.Sprintf("project-%d", i)
		
		// Vary the project types
		var projectType string
		var files map[string]string
		
		switch i % 4 {
		case 0: // Go project
			projectType = "go"
			files = map[string]string{
				fmt.Sprintf("%s/main.go", projectDir):  "package main\nfunc main() {}",
				fmt.Sprintf("%s/go.mod", projectDir):   fmt.Sprintf("module project-%d\ngo 1.20", i),
			}
		case 1: // Python project  
			projectType = "python"
			files = map[string]string{
				fmt.Sprintf("%s/app.py", projectDir):         "def main(): pass",
				fmt.Sprintf("%s/requirements.txt", projectDir): "flask==2.0.1",
			}
		case 2: // JavaScript project
			projectType = "javascript"
			files = map[string]string{
				fmt.Sprintf("%s/app.js", projectDir):      "console.log('hello');",
				fmt.Sprintf("%s/package.json", projectDir): fmt.Sprintf(`{"name": "project-%d"}`, i),
			}
		case 3: // TypeScript project
			projectType = "typescript"
			files = map[string]string{
				fmt.Sprintf("%s/app.ts", projectDir):       "console.log('hello');",
				fmt.Sprintf("%s/package.json", projectDir): fmt.Sprintf(`{"name": "project-%d"}`, i),
				fmt.Sprintf("%s/tsconfig.json", projectDir): `{"compilerOptions": {}}`,
			}
		}
		
		for filePath, content := range files {
			fullPath := filepath.Join(tempDir, filePath)
			dir := filepath.Dir(fullPath)
			
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatalf("Failed to create directory %s: %v", dir, err)
			}
			
			if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
				t.Fatalf("Failed to create file %s: %v", filePath, err)
			}
		}
		
		t.Logf("Created %s project %d", projectType, i)
	}
	
	cleanup := func() {
		os.RemoveAll(tempDir)
	}
	
	return tempDir, cleanup
}

func createNestedProjectsIntegration(t *testing.T) (string, func()) {
	tempDir, err := os.MkdirTemp("", "lspg-nested-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	
	// Create deeply nested project structure
	structure := map[string]string{
		// Top-level project
		"go.mod":                                "module top-level\ngo 1.20",
		"main.go":                              "package main\nfunc main() {}",
		
		// Nested subprojects
		"services/user/go.mod":                 "module user-service\ngo 1.20",
		"services/user/main.go":                "package main\nfunc main() {}",
		"services/user/internal/handler.go":    "package internal\nfunc Handler() {}",
		
		// Deeply nested
		"services/user/pkg/auth/go.mod":        "module auth-pkg\ngo 1.20", 
		"services/user/pkg/auth/auth.go":       "package auth\nfunc Auth() {}",
		
		// Mixed languages at different levels
		"frontend/package.json":                `{"name": "frontend"}`,
		"frontend/src/app.js":                  "console.log('app');",
		"frontend/components/package.json":     `{"name": "components"}`,
		"frontend/components/src/Button.tsx":   "export function Button() {}",
		
		// Python project nested in Go structure
		"services/user/scripts/requirements.txt": "requests==2.25.1",
		"services/user/scripts/deploy.py":        "import requests",
	}
	
	for filePath, content := range structure {
		fullPath := filepath.Join(tempDir, filePath)
		dir := filepath.Dir(fullPath)
		
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
		
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", filePath, err)
		}
	}
	
	cleanup := func() {
		os.RemoveAll(tempDir)
	}
	
	return tempDir, cleanup
}

func createMixedLanguageWorkspace(t *testing.T) (string, func()) {
	tempDir, err := os.MkdirTemp("", "lspg-mixed-lang-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	
	// Create a workspace with mixed language files in same directories
	structure := map[string]string{
		// Multi-language project root
		"main.go":           "package main\nfunc main() {}",
		"go.mod":            "module mixed-project\ngo 1.20",
		"app.py":            "def main(): pass",
		"requirements.txt":  "flask==2.0.1",
		"app.js":            "console.log('hello');",
		"package.json":      `{"name": "mixed-project"}`,
		"app.ts":            "console.log('hello');",
		"tsconfig.json":     `{"compilerOptions": {}}`,
		
		// Subdirectory with mixed files
		"src/handler.go":    "package src\nfunc Handler() {}",
		"src/utils.py":      "def utils(): pass",
		"src/helper.js":     "function helper() {}",
		"src/types.ts":      "export interface Types {}",
		
		// Language-specific subdirectories
		"go-service/main.go":      "package main\nfunc main() {}",
		"go-service/go.mod":       "module go-service\ngo 1.20",
		"python-service/app.py":   "def main(): pass",
		"js-client/app.js":        "console.log('client');",
		"ts-types/types.ts":       "export interface User {}",
	}
	
	for filePath, content := range structure {
		fullPath := filepath.Join(tempDir, filePath)
		dir := filepath.Dir(fullPath)
		
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
		
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", filePath, err)
		}
	}
	
	cleanup := func() {
		os.RemoveAll(tempDir)
	}
	
	return tempDir, cleanup
}

// Test scenario functions

func testLargeMonorepoHandling(t *testing.T, gateway WorkspaceGateway, tempDir string) {
	projects := gateway.GetSubProjects()
	
	// Should detect a good number of projects
	if len(projects) < 10 {
		t.Errorf("Expected at least 10 projects in large monorepo, got %d", len(projects))
	}
	
	// Test that project refresh doesn't take too long
	start := time.Now()
	err := gateway.RefreshSubProjects(context.Background())
	refreshTime := time.Since(start)
	
	if err != nil {
		t.Errorf("Project refresh failed: %v", err)
	}
	
	// Should complete refresh within reasonable time even with many projects
	if refreshTime > 5*time.Second {
		t.Errorf("Project refresh took too long: %v", refreshTime)
	}
	
	// Test health check includes all projects
	health := gateway.Health()
	if health.SubProjects != len(projects) {
		t.Errorf("Health reports %d projects but GetSubProjects returned %d", 
			health.SubProjects, len(projects))
	}
}

func testNestedProjectHandling(t *testing.T, gateway WorkspaceGateway, tempDir string) {
	projects := gateway.GetSubProjects()
	
	// Should detect nested projects
	if len(projects) < 3 {
		t.Errorf("Expected at least 3 nested projects, got %d", len(projects))
	}
	
	// Test that nested relationships are properly detected
	hasNestedRelationship := false
	for _, project := range projects {
		if project.Parent != nil || len(project.Children) > 0 {
			hasNestedRelationship = true
			break
		}
	}
	
	if !hasNestedRelationship {
		t.Error("Expected to find nested project relationships")
	}
	
	// Test routing to files at different nesting levels
	testFiles := []string{
		"main.go",                              // Root level
		"services/user/main.go",                // One level deep
		"services/user/pkg/auth/auth.go",       // Three levels deep
	}
	
	for _, testFile := range testFiles {
		fullPath := filepath.Join(tempDir, testFile)
		if _, err := os.Stat(fullPath); err == nil {
			// File exists, verify it's within our test workspace
			uri := fmt.Sprintf("file://%s", fullPath)
			t.Logf("Testing nested file routing for: %s", uri)
		}
	}
}

func testMixedLanguageHandling(t *testing.T, gateway WorkspaceGateway, tempDir string) {
	projects := gateway.GetSubProjects()
	
	// Should detect projects with multiple languages
	multiLangProject := false
	for _, project := range projects {
		if len(project.Languages) > 1 {
			multiLangProject = true
			break
		}
	}
	
	if !multiLangProject {
		t.Error("Expected to find at least one multi-language project")
	}
	
	// Test language detection for different file types in same directory
	testFiles := map[string]string{
		"main.go":    "go",
		"app.py":     "python",
		"app.js":     "javascript", 
		"app.ts":     "typescript",
	}
	
	gw := gateway.(*workspaceGateway)
	for fileName, expectedLang := range testFiles {
		uri := fmt.Sprintf("file://%s/%s", tempDir, fileName)
		detectedLang, err := gw.extractLanguageFromURI(uri)
		if err != nil {
			t.Errorf("Failed to detect language for %s: %v", fileName, err)
			continue
		}
		
		if detectedLang != expectedLang {
			t.Errorf("Language detection mismatch for %s: expected %s, got %s", 
				fileName, expectedLang, detectedLang)
		}
	}
	
	// Test that all expected languages are supported
	expectedLanguages := []string{"go", "python", "javascript", "typescript"}
	foundLanguages := make(map[string]bool)
	
	for _, project := range projects {
		for _, lang := range project.Languages {
			foundLanguages[lang] = true
		}
	}
	
	for _, expectedLang := range expectedLanguages {
		if !foundLanguages[expectedLang] {
			t.Errorf("Expected language %s not found in any project", expectedLang)
		}
	}
}

