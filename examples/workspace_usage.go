package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"lsp-gateway/internal/project"
	"lsp-gateway/internal/workspace"
	"lsp-gateway/mcp"
)

const (
	ExampleTimeout = 30 * time.Second
	HTTPTimeout    = 10 * time.Second
)

func main() {
	log.Println("=== LSP Gateway Workspace Usage Examples ===")
	log.Println("Demonstrating workspace architecture capabilities and integration patterns")
	
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("Starting Workspace Usage Examples")
	fmt.Println(strings.Repeat("=", 80))
	
	ExampleBasicWorkspaceSetup()
	ExampleMultipleWorkspaces()
	ExampleIDEClientConnection()
	ExampleMCPClientUsage()
	ExampleConfigurationHierarchy()
	ExamplePerformanceMonitoring()
	
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("All Workspace Usage Examples Completed Successfully!")
	fmt.Println(strings.Repeat("=", 80))
}

// ExampleBasicWorkspaceSetup demonstrates workspace detection, config generation, gateway startup
func ExampleBasicWorkspaceSetup() {
	fmt.Println("\n🔧 Example 1: Basic Workspace Setup")
	fmt.Println(strings.Repeat("-", 50))
	
	ctx, cancel := context.WithTimeout(context.Background(), ExampleTimeout)
	defer cancel()
	
	startTime := time.Now()
	
	// Step 1: Create workspace detector
	fmt.Println("Step 1: Creating workspace detector...")
	detector := workspace.NewWorkspaceDetector()
	
	// Step 2: Detect current workspace
	fmt.Println("Step 2: Detecting workspace...")
	workspaceContext, err := detector.DetectWorkspace()
	if err != nil {
		log.Printf("⚠️  Failed to detect workspace: %v", err)
		return
	}
	
	fmt.Printf("✅ Workspace detected:\n")
	fmt.Printf("   ID: %s\n", workspaceContext.ID)
	fmt.Printf("   Root: %s\n", workspaceContext.Root)
	fmt.Printf("   Project Type: %s\n", workspaceContext.ProjectType)
	fmt.Printf("   Languages: %v\n", workspaceContext.Languages)
	fmt.Printf("   Hash: %s\n", workspaceContext.Hash)
	
	// Step 3: Create config manager
	fmt.Println("\nStep 3: Creating workspace configuration manager...")
	configManager := workspace.NewWorkspaceConfigManager()
	
	// Step 4: Create project context for configuration
	fmt.Println("Step 4: Creating project context...")
	projectContext := &project.ProjectContext{
		ProjectType: workspaceContext.ProjectType,
		Languages:   workspaceContext.Languages,
		RootPath:    workspaceContext.Root,
	}
	
	// Step 5: Generate workspace configuration
	fmt.Println("Step 5: Generating workspace configuration...")
	if err := configManager.GenerateWorkspaceConfig(workspaceContext.Root, projectContext); err != nil {
		log.Printf("⚠️  Failed to generate workspace config: %v", err)
		return
	}
	
	// Step 6: Load workspace configuration
	fmt.Println("Step 6: Loading workspace configuration...")
	workspaceConfig, err := configManager.LoadWorkspaceConfig(workspaceContext.Root)
	if err != nil {
		log.Printf("⚠️  Failed to load workspace config: %v", err)
		return
	}
	
	// Step 7: Create port manager
	fmt.Println("Step 7: Creating port manager...")
	portManager, err := workspace.NewWorkspacePortManager()
	if err != nil {
		log.Printf("⚠️  Failed to create port manager: %v", err)
		return
	}
	
	// Step 8: Allocate port for workspace
	fmt.Println("Step 8: Allocating port for workspace...")
	port, err := portManager.AllocatePort(workspaceContext.Root)
	if err != nil {
		log.Printf("⚠️  Failed to allocate port: %v", err)
		return
	}
	fmt.Printf("✅ Port allocated: %d\n", port)
	
	// Step 9: Create workspace gateway
	fmt.Println("Step 9: Creating and initializing workspace gateway...")
	gateway := workspace.NewWorkspaceGateway()
	
	gatewayConfig := &workspace.WorkspaceGatewayConfig{
		WorkspaceRoot: workspaceContext.Root,
		ExtensionMapping: map[string]string{
			".go":   "go",
			".py":   "python",
			".js":   "javascript",
			".ts":   "typescript",
			".java": "java",
		},
		Timeout:       30 * time.Second,
		EnableLogging: true,
	}
	
	if err := gateway.Initialize(ctx, workspaceConfig, gatewayConfig); err != nil {
		log.Printf("⚠️  Failed to initialize gateway: %v", err)
		return
	}
	
	// Step 10: Start workspace gateway
	fmt.Println("Step 10: Starting workspace gateway...")
	if err := gateway.Start(ctx); err != nil {
		log.Printf("⚠️  Failed to start gateway: %v", err)
		return
	}
	
	// Step 11: Check gateway health
	fmt.Println("Step 11: Checking gateway health...")
	health := gateway.Health()
	fmt.Printf("✅ Gateway health status:\n")
	fmt.Printf("   Healthy: %t\n", health.IsHealthy)
	fmt.Printf("   Active Clients: %d\n", health.ActiveClients)
	fmt.Printf("   Client Statuses: %d\n", len(health.ClientStatuses))
	for lang, status := range health.ClientStatuses {
		fmt.Printf("     - %s: active=%t\n", lang, status.IsActive)
	}
	
	// Step 12: Cleanup
	fmt.Println("Step 12: Cleaning up resources...")
	if err := gateway.Stop(); err != nil {
		log.Printf("⚠️  Error stopping gateway: %v", err)
	}
	
	if err := portManager.ReleasePort(workspaceContext.Root); err != nil {
		log.Printf("⚠️  Error releasing port: %v", err)
	}
	
	duration := time.Since(startTime)
	fmt.Printf("✅ Basic workspace setup completed in %v\n", duration)
}

// ExampleMultipleWorkspaces demonstrates managing multiple workspace instances
func ExampleMultipleWorkspaces() {
	fmt.Println("\n🏢 Example 2: Multiple Workspace Management")
	fmt.Println(strings.Repeat("-", 50))
	
	ctx, cancel := context.WithTimeout(context.Background(), ExampleTimeout)
	defer cancel()
	
	startTime := time.Now()
	
	// Create temporary workspace directories for demonstration
	tempDir := os.TempDir()
	workspace1 := filepath.Join(tempDir, "workspace1")
	workspace2 := filepath.Join(tempDir, "workspace2")
	workspace3 := filepath.Join(tempDir, "workspace3")
	
	workspaces := []string{workspace1, workspace2, workspace3}
	
	// Step 1: Create workspace directories with different project types
	fmt.Println("Step 1: Creating multiple workspace directories...")
	for i, ws := range workspaces {
		if err := os.MkdirAll(ws, 0755); err != nil {
			log.Printf("⚠️  Failed to create workspace %s: %v", ws, err)
			continue
		}
		
		// Create different project markers
		switch i {
		case 0: // Go project
			createFile(filepath.Join(ws, "go.mod"), "module workspace1\n\ngo 1.21\n")
			createFile(filepath.Join(ws, "main.go"), "package main\n\nfunc main() {}\n")
		case 1: // Python project
			createFile(filepath.Join(ws, "setup.py"), "from setuptools import setup\n\nsetup(name='workspace2')\n")
			createFile(filepath.Join(ws, "main.py"), "def main():\n    pass\n")
		case 2: // TypeScript project
			createFile(filepath.Join(ws, "tsconfig.json"), `{"compilerOptions": {"target": "es2020"}}`)
			createFile(filepath.Join(ws, "index.ts"), "function main(): void {}\n")
		}
		
		fmt.Printf("   ✅ Created workspace: %s\n", ws)
	}
	
	// Step 2: Initialize components
	fmt.Println("\nStep 2: Initializing workspace components...")
	detector := workspace.NewWorkspaceDetector()
	configManager := workspace.NewWorkspaceConfigManager()
	portManager, err := workspace.NewWorkspacePortManager()
	if err != nil {
		log.Printf("⚠️  Failed to create port manager: %v", err)
		return
	}
	
	// Step 3: Process workspaces concurrently
	fmt.Println("Step 3: Processing workspaces concurrently...")
	var wg sync.WaitGroup
	results := make(chan WorkspaceResult, len(workspaces))
	
	for _, ws := range workspaces {
		wg.Add(1)
		go func(workspacePath string) {
			defer wg.Done()
			processWorkspace(ctx, workspacePath, detector, configManager, portManager, results)
		}(ws)
	}
	
	wg.Wait()
	close(results)
	
	// Step 4: Collect and display results
	fmt.Println("\nStep 4: Workspace processing results:")
	var successCount, failureCount int
	var totalPorts []int
	
	for result := range results {
		if result.Error != nil {
			failureCount++
			fmt.Printf("   ❌ %s: %v\n", result.WorkspacePath, result.Error)
		} else {
			successCount++
			totalPorts = append(totalPorts, result.Port)
			fmt.Printf("   ✅ %s: %s project, port %d, %d clients\n", 
				result.WorkspacePath, result.ProjectType, result.Port, result.ActiveClients)
		}
	}
	
	// Step 5: Demonstrate port management
	fmt.Println("\nStep 5: Port allocation summary:")
	fmt.Printf("   Allocated ports: %v\n", totalPorts)
	fmt.Printf("   Port range utilization: %d/%d\n", len(totalPorts), 
		workspace.PortRangeEnd-workspace.PortRangeStart+1)
	
	// Step 6: Cleanup
	fmt.Println("\nStep 6: Cleaning up workspaces...")
	for i, ws := range workspaces {
		if err := portManager.ReleasePort(ws); err != nil {
			log.Printf("⚠️  Error releasing port for %s: %v", ws, err)
		}
		if err := configManager.CleanupWorkspace(ws); err != nil {
			log.Printf("⚠️  Error cleaning up config for %s: %v", ws, err)
		}
		if err := os.RemoveAll(ws); err != nil {
			log.Printf("⚠️  Error removing directory %s: %v", ws, err)
		}
		fmt.Printf("   ✅ Cleaned up workspace %d\n", i+1)
	}
	
	duration := time.Since(startTime)
	fmt.Printf("\n✅ Multiple workspace management completed in %v\n", duration)
	fmt.Printf("   Success: %d, Failures: %d\n", successCount, failureCount)
}

// ExampleIDEClientConnection demonstrates how IDEs connect to workspace HTTP endpoints
func ExampleIDEClientConnection() {
	fmt.Println("\n💻 Example 3: IDE Client Integration")
	fmt.Println(strings.Repeat("-", 50))
	
	ctx, cancel := context.WithTimeout(context.Background(), ExampleTimeout)
	defer cancel()
	
	startTime := time.Now()
	
	// Step 1: Setup workspace
	fmt.Println("Step 1: Setting up workspace for IDE connection...")
	detector := workspace.NewWorkspaceDetector()
	workspaceContext, err := detector.DetectWorkspace()
	if err != nil {
		log.Printf("⚠️  Failed to detect workspace: %v", err)
		return
	}
	
	configManager := workspace.NewWorkspaceConfigManager()
	projectContext := &project.ProjectContext{
		ProjectType: workspaceContext.ProjectType,
		Languages:   workspaceContext.Languages,
		RootPath:    workspaceContext.Root,
	}
	
	if err := configManager.GenerateWorkspaceConfig(workspaceContext.Root, projectContext); err != nil {
		log.Printf("⚠️  Failed to generate workspace config: %v", err)
		return
	}
	
	workspaceConfig, err := configManager.LoadWorkspaceConfig(workspaceContext.Root)
	if err != nil {
		log.Printf("⚠️  Failed to load workspace config: %v", err)
		return
	}
	
	// Step 2: Start HTTP server
	fmt.Println("Step 2: Starting workspace HTTP server...")
	portManager, err := workspace.NewWorkspacePortManager()
	if err != nil {
		log.Printf("⚠️  Failed to create port manager: %v", err)
		return
	}
	
	port, err := portManager.AllocatePort(workspaceContext.Root)
	if err != nil {
		log.Printf("⚠️  Failed to allocate port: %v", err)
		return
	}
	
	gateway := workspace.NewWorkspaceGateway()
	gatewayConfig := &workspace.WorkspaceGatewayConfig{
		WorkspaceRoot:    workspaceContext.Root,
		ExtensionMapping: getDefaultExtensionMapping(),
		Timeout:          30 * time.Second,
		EnableLogging:    true,
	}
	
	if err := gateway.Initialize(ctx, workspaceConfig, gatewayConfig); err != nil {
		log.Printf("⚠️  Failed to initialize gateway: %v", err)
		return
	}
	
	if err := gateway.Start(ctx); err != nil {
		log.Printf("⚠️  Failed to start gateway: %v", err)
		return
	}
	
	// Step 3: Start HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/jsonrpc", gateway.HandleJSONRPC)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		health := gateway.Health()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(health)
	})
	
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  HTTPTimeout,
		WriteTimeout: HTTPTimeout,
	}
	
	serverErrChan := make(chan error, 1)
	go func() {
		fmt.Printf("✅ HTTP server listening on port %d\n", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrChan <- err
		}
	}()
	
	// Wait for server to start
	time.Sleep(1 * time.Second)
	
	// Step 4: Simulate IDE client connections
	fmt.Println("\nStep 3: Simulating IDE client JSON-RPC requests...")
	
	baseURL := fmt.Sprintf("http://localhost:%d", port)
	
	// Test different LSP methods with various file types
	testCases := []struct {
		name     string
		method   string
		fileExt  string
		language string
	}{
		{"Go Hover", "textDocument/hover", ".go", "go"},
		{"Python Symbols", "textDocument/documentSymbol", ".py", "python"},
		{"TypeScript Definition", "textDocument/definition", ".ts", "typescript"},
		{"JavaScript References", "textDocument/references", ".js", "javascript"},
		{"Java Completion", "textDocument/completion", ".java", "java"},
	}
	
	for _, tc := range testCases {
		fmt.Printf("   Testing %s...", tc.name)
		
		// Create JSON-RPC request
		request := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  tc.method,
			"params": map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": fmt.Sprintf("file:///example/test%s", tc.fileExt),
				},
				"position": map[string]interface{}{
					"line":      0,
					"character": 0,
				},
			},
		}
		
		// Send request
		success := sendJSONRPCRequest(baseURL+"/jsonrpc", request)
		if success {
			fmt.Printf(" ✅\n")
		} else {
			fmt.Printf(" ❌\n")
		}
	}
	
	// Step 5: Test health endpoint
	fmt.Println("\nStep 4: Testing health endpoint...")
	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		fmt.Printf("   ❌ Health check failed: %v\n", err)
	} else {
		defer resp.Body.Close()
		var health workspace.GatewayHealth
		if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
			fmt.Printf("   ❌ Failed to decode health response: %v\n", err)
		} else {
			fmt.Printf("   ✅ Health check passed: healthy=%t, clients=%d\n", 
				health.IsHealthy, health.ActiveClients)
		}
	}
	
	// Step 6: Cleanup
	fmt.Println("\nStep 5: Shutting down server...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("⚠️  Error shutting down server: %v", err)
	}
	
	if err := gateway.Stop(); err != nil {
		log.Printf("⚠️  Error stopping gateway: %v", err)
	}
	
	if err := portManager.ReleasePort(workspaceContext.Root); err != nil {
		log.Printf("⚠️  Error releasing port: %v", err)
	}
	
	duration := time.Since(startTime)
	fmt.Printf("✅ IDE client integration completed in %v\n", duration)
}

// ExampleMCPClientUsage demonstrates MCP client connecting to workspace-aware MCP server
func ExampleMCPClientUsage() {
	fmt.Println("\n🤖 Example 4: MCP Client Integration")
	fmt.Println(strings.Repeat("-", 50))
	
	ctx, cancel := context.WithTimeout(context.Background(), ExampleTimeout)
	defer cancel()
	
	startTime := time.Now()
	
	// Step 1: Setup workspace context
	fmt.Println("Step 1: Setting up workspace context for MCP...")
	detector := workspace.NewWorkspaceDetector()
	workspaceContext, err := detector.DetectWorkspace()
	if err != nil {
		log.Printf("⚠️  Failed to detect workspace: %v", err)
		return
	}
	
	fmt.Printf("✅ Workspace context for MCP:\n")
	fmt.Printf("   Root: %s\n", workspaceContext.Root)
	fmt.Printf("   Languages: %v\n", workspaceContext.Languages)
	fmt.Printf("   Project Type: %s\n", workspaceContext.ProjectType)
	
	// Step 2: Create MCP server with workspace awareness
	fmt.Println("\nStep 2: Creating workspace-aware MCP server...")
	
	// Create a mock gateway for MCP integration
	gateway := workspace.NewWorkspaceGateway()
	configManager := workspace.NewWorkspaceConfigManager()
	
	projectContext := &project.ProjectContext{
		ProjectType: workspaceContext.ProjectType,
		Languages:   workspaceContext.Languages,
		RootPath:    workspaceContext.Root,
	}
	
	if err := configManager.GenerateWorkspaceConfig(workspaceContext.Root, projectContext); err != nil {
		log.Printf("⚠️  Failed to generate workspace config: %v", err)
		return
	}
	
	workspaceConfig, err := configManager.LoadWorkspaceConfig(workspaceContext.Root)
	if err != nil {
		log.Printf("⚠️  Failed to load workspace config: %v", err)
		return
	}
	
	gatewayConfig := &workspace.WorkspaceGatewayConfig{
		WorkspaceRoot:    workspaceContext.Root,
		ExtensionMapping: getDefaultExtensionMapping(),
		Timeout:          30 * time.Second,
		EnableLogging:    true,
	}
	
	if err := gateway.Initialize(ctx, workspaceConfig, gatewayConfig); err != nil {
		log.Printf("⚠️  Failed to initialize gateway: %v", err)
		return
	}
	
	// Step 3: Create MCP tools with workspace context
	fmt.Println("Step 3: Creating MCP tools with workspace context...")
	
	workspaceTools := []mcp.Tool{
		{
			Name:        "workspace_info",
			Description: "Get current workspace information and status",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"include_health": map[string]interface{}{
						"type":    "boolean",
						"default": true,
					},
				},
			},
		},
		{
			Name:        "workspace_symbol_search",
			Description: "Search for symbols across the workspace",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "Symbol name or pattern to search for",
					},
					"language": map[string]interface{}{
						"type":        "string",
						"description": "Language to filter by (optional)",
					},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "workspace_hover_info",
			Description: "Get hover information for a symbol in the workspace",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"file_path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the file",
					},
					"line": map[string]interface{}{
						"type":        "number",
						"description": "Line number (0-based)",
					},
					"character": map[string]interface{}{
						"type":        "number", 
						"description": "Character position (0-based)",
					},
				},
				"required": []string{"file_path", "line", "character"},
			},
		},
	}
	
	fmt.Printf("✅ Created %d workspace-aware MCP tools\n", len(workspaceTools))
	
	// Step 4: Simulate MCP client tool calls
	fmt.Println("\nStep 4: Simulating MCP client tool calls...")
	
	// Simulate workspace_info call
	fmt.Println("   Testing workspace_info tool...")
	workspaceInfoArgs := map[string]interface{}{
		"include_health": true,
	}
	
	workspaceInfo := simulateWorkspaceInfoTool(workspaceContext, gateway, workspaceInfoArgs)
	fmt.Printf("   ✅ Workspace info retrieved: %d languages, %s type\n", 
		len(workspaceInfo["languages"].([]string)), workspaceInfo["project_type"])
	
	// Simulate workspace_symbol_search call
	fmt.Println("   Testing workspace_symbol_search tool...")
	symbolSearchArgs := map[string]interface{}{
		"query":    "main",
		"language": "go",
	}
	
	symbolResults := simulateSymbolSearchTool(workspaceContext, symbolSearchArgs)
	fmt.Printf("   ✅ Symbol search completed: found %d results\n", len(symbolResults))
	
	// Simulate workspace_hover_info call
	fmt.Println("   Testing workspace_hover_info tool...")
	hoverArgs := map[string]interface{}{
		"file_path": filepath.Join(workspaceContext.Root, "main.go"),
		"line":      0,
		"character": 0,
	}
	
	hoverInfo := simulateHoverInfoTool(workspaceContext, gateway, hoverArgs)
	fmt.Printf("   ✅ Hover info retrieved: %s\n", hoverInfo["status"])
	
	// Step 5: Demonstrate workspace-specific MCP features
	fmt.Println("\nStep 5: Demonstrating workspace-specific MCP features...")
	
	features := map[string]string{
		"Project-aware tool routing":    "Tools automatically route to appropriate LSP servers based on file extensions",
		"Workspace context persistence": "MCP maintains workspace state across tool calls",
		"Multi-language support":        "Single MCP session handles multiple programming languages",
		"Performance optimization":      "Workspace-level caching and connection pooling",
		"Configuration hierarchy":       "Global → workspace → environment config merging",
	}
	
	for feature, description := range features {
		fmt.Printf("   ✅ %s: %s\n", feature, description)
	}
	
	// Step 6: Cleanup
	fmt.Println("\nStep 6: Cleaning up MCP resources...")
	if err := gateway.Stop(); err != nil {
		log.Printf("⚠️  Error stopping gateway: %v", err)
	}
	
	duration := time.Since(startTime)
	fmt.Printf("✅ MCP client integration completed in %v\n", duration)
}

// ExampleConfigurationHierarchy demonstrates global → workspace → environment config merging
func ExampleConfigurationHierarchy() {
	fmt.Println("\n⚙️  Example 5: Configuration Management Hierarchy")
	fmt.Println(strings.Repeat("-", 50))
	
	startTime := time.Now()
	
	// Step 1: Create temporary config files for demonstration
	fmt.Println("Step 1: Creating configuration hierarchy...")
	
	tempDir := os.TempDir()
	globalConfigPath := filepath.Join(tempDir, "global-config.yaml")
	
	// Create global configuration
	globalConfig := `
servers:
  - name: "global-go-server"
    languages: ["go"]
    command: "gopls"
    transport: "stdio"
    settings:
      gopls:
        usePlaceholders: true
        completeUnimported: true

performance_config:
  enabled: true
  profile: "development"
  auto_tuning: true
  caching:
    enabled: true
    global_ttl: "15m"
    max_memory_usage: 1024
    
logging:
  level: "info"
  max_size_mb: 50
`
	
	if err := os.WriteFile(globalConfigPath, []byte(globalConfig), 0644); err != nil {
		log.Printf("⚠️  Failed to create global config: %v", err)
		return
	}
	
	fmt.Printf("✅ Created global configuration: %s\n", globalConfigPath)
	
	// Step 2: Setup workspace
	fmt.Println("\nStep 2: Setting up workspace configuration...")
	detector := workspace.NewWorkspaceDetector()
	workspaceContext, err := detector.DetectWorkspace()
	if err != nil {
		log.Printf("⚠️  Failed to detect workspace: %v", err)
		return
	}
	
	configManager := workspace.NewWorkspaceConfigManager()
	projectContext := &project.ProjectContext{
		ProjectType: workspaceContext.ProjectType,
		Languages:   workspaceContext.Languages,
		RootPath:    workspaceContext.Root,
	}
	
	// Generate workspace-specific configuration
	if err := configManager.GenerateWorkspaceConfig(workspaceContext.Root, projectContext); err != nil {
		log.Printf("⚠️  Failed to generate workspace config: %v", err)
		return
	}
	
	workspaceConfigPath := configManager.GetWorkspaceConfigPath(workspaceContext.Root)
	fmt.Printf("✅ Generated workspace configuration: %s\n", workspaceConfigPath)
	
	// Step 3: Demonstrate configuration loading hierarchy
	fmt.Println("\nStep 3: Loading configuration hierarchy...")
	
	// Load with hierarchy merging
	mergedConfig, err := configManager.LoadWithHierarchy(workspaceContext.Root, globalConfigPath)
	if err != nil {
		log.Printf("⚠️  Failed to load configuration hierarchy: %v", err)
		return
	}
	
	fmt.Printf("✅ Configuration hierarchy loaded successfully\n")
	
	// Step 4: Display configuration merging results
	fmt.Println("\nStep 4: Configuration merging analysis...")
	
	fmt.Printf("Workspace Information:\n")
	fmt.Printf("   ID: %s\n", mergedConfig.Workspace.WorkspaceID)
	fmt.Printf("   Root: %s\n", mergedConfig.Workspace.RootPath)
	fmt.Printf("   Languages: %v\n", mergedConfig.Workspace.Languages)
	fmt.Printf("   Project Type: %s\n", mergedConfig.Workspace.ProjectType)
	
	fmt.Printf("\nServer Configurations (%d total):\n", len(mergedConfig.Servers))
	for name, server := range mergedConfig.Servers {
		fmt.Printf("   - %s: languages=%v, command=%s\n", 
			name, server.Languages, server.Command)
	}
	
	fmt.Printf("\nPerformance Configuration:\n")
	fmt.Printf("   Enabled: %t\n", mergedConfig.Performance.Enabled)
	fmt.Printf("   Profile: %s\n", mergedConfig.Performance.Profile)
	fmt.Printf("   Auto Tuning: %t\n", mergedConfig.Performance.AutoTuning)
	if mergedConfig.Performance.Caching != nil {
		fmt.Printf("   Caching: enabled=%t, memory=%dMB\n", 
			mergedConfig.Performance.Caching.Enabled, 
			mergedConfig.Performance.Caching.MaxMemoryUsage)
	}
	
	fmt.Printf("\nDirectory Structure:\n")
	fmt.Printf("   Root: %s\n", mergedConfig.Directories.Root)
	fmt.Printf("   Cache: %s\n", mergedConfig.Directories.Cache)
	fmt.Printf("   Logs: %s\n", mergedConfig.Directories.Logs)
	fmt.Printf("   Index: %s\n", mergedConfig.Directories.Index)
	
	// Step 5: Demonstrate environment overrides
	fmt.Println("\nStep 5: Demonstrating environment overrides...")
	
	// Set environment variables for testing
	originalLogLevel := os.Getenv("LSPG_LOG_LEVEL")
	originalCacheSize := os.Getenv("LSPG_CACHE_SIZE")
	
	os.Setenv("LSPG_LOG_LEVEL", "debug")
	os.Setenv("LSPG_CACHE_SIZE", "2048")
	
	fmt.Printf("Set environment overrides:\n")
	fmt.Printf("   LSPG_LOG_LEVEL=debug\n")
	fmt.Printf("   LSPG_CACHE_SIZE=2048\n")
	
	// Reload with environment overrides
	reloadedConfig, err := configManager.LoadWithHierarchy(workspaceContext.Root, globalConfigPath)
	if err != nil {
		log.Printf("⚠️  Failed to reload with environment overrides: %v", err)
	} else {
		fmt.Printf("✅ Configuration reloaded with environment overrides\n")
		fmt.Printf("   Effective log level: %s\n", reloadedConfig.Logging.Level)
	}
	
	// Restore environment
	if originalLogLevel != "" {
		os.Setenv("LSPG_LOG_LEVEL", originalLogLevel)
	} else {
		os.Unsetenv("LSPG_LOG_LEVEL")
	}
	if originalCacheSize != "" {
		os.Setenv("LSPG_CACHE_SIZE", originalCacheSize)
	} else {
		os.Unsetenv("LSPG_CACHE_SIZE")
	}
	
	// Step 6: Cleanup
	fmt.Println("\nStep 6: Cleaning up configuration files...")
	os.Remove(globalConfigPath)
	configManager.CleanupWorkspace(workspaceContext.Root)
	
	duration := time.Since(startTime)
	fmt.Printf("✅ Configuration hierarchy demonstration completed in %v\n", duration)
}

// ExamplePerformanceMonitoring demonstrates workspace health monitoring and SCIP cache performance
func ExamplePerformanceMonitoring() {
	fmt.Println("\n📊 Example 6: Performance Monitoring")
	fmt.Println(strings.Repeat("-", 50))
	
	ctx, cancel := context.WithTimeout(context.Background(), ExampleTimeout)
	defer cancel()
	
	startTime := time.Now()
	
	// Step 1: Setup workspace with performance monitoring
	fmt.Println("Step 1: Setting up workspace with performance monitoring...")
	detector := workspace.NewWorkspaceDetector()
	workspaceContext, err := detector.DetectWorkspace()
	if err != nil {
		log.Printf("⚠️  Failed to detect workspace: %v", err)
		return
	}
	
	configManager := workspace.NewWorkspaceConfigManager()
	projectContext := &project.ProjectContext{
		ProjectType: workspaceContext.ProjectType,
		Languages:   workspaceContext.Languages,
		RootPath:    workspaceContext.Root,
	}
	
	if err := configManager.GenerateWorkspaceConfig(workspaceContext.Root, projectContext); err != nil {
		log.Printf("⚠️  Failed to generate workspace config: %v", err)
		return
	}
	
	workspaceConfig, err := configManager.LoadWorkspaceConfig(workspaceContext.Root)
	if err != nil {
		log.Printf("⚠️  Failed to load workspace config: %v", err)
		return
	}
	
	// Step 2: Initialize performance monitoring components
	fmt.Println("Step 2: Initializing performance monitoring components...")
	
	portManager, err := workspace.NewWorkspacePortManager()
	if err != nil {
		log.Printf("⚠️  Failed to create port manager: %v", err)
		return
	}
	
	port, err := portManager.AllocatePort(workspaceContext.Root)
	if err != nil {
		log.Printf("⚠️  Failed to allocate port: %v", err)
		return
	}
	
	gateway := workspace.NewWorkspaceGateway()
	gatewayConfig := &workspace.WorkspaceGatewayConfig{
		WorkspaceRoot:    workspaceContext.Root,
		ExtensionMapping: getDefaultExtensionMapping(),
		Timeout:          30 * time.Second,
		EnableLogging:    true,
	}
	
	if err := gateway.Initialize(ctx, workspaceConfig, gatewayConfig); err != nil {
		log.Printf("⚠️  Failed to initialize gateway: %v", err)
		return
	}
	
	if err := gateway.Start(ctx); err != nil {
		log.Printf("⚠️  Failed to start gateway: %v", err)
		return
	}
	
	fmt.Printf("✅ Performance monitoring initialized on port %d\n", port)
	
	// Step 3: Collect initial performance metrics
	fmt.Println("\nStep 3: Collecting baseline performance metrics...")
	
	initialHealth := gateway.Health()
	fmt.Printf("Initial Health Status:\n")
	fmt.Printf("   Healthy: %t\n", initialHealth.IsHealthy)
	fmt.Printf("   Active Clients: %d\n", initialHealth.ActiveClients)
	fmt.Printf("   Last Check: %s\n", initialHealth.LastCheck.Format(time.RFC3339))
	
	// Step 4: Simulate workload and monitor performance
	fmt.Println("\nStep 4: Simulating workload and monitoring performance...")
	
	performanceMetrics := &PerformanceMetrics{
		StartTime:      time.Now(),
		TotalRequests:  0,
		SuccessfulRequests: 0,
		FailedRequests: 0,
		AverageLatency: 0,
		MemoryUsage:    0,
		PortUtilization: float64(1) / float64(workspace.PortRangeEnd-workspace.PortRangeStart+1) * 100,
	}
	
	// Simulate multiple health checks over time
	for i := 0; i < 5; i++ {
		checkStart := time.Now()
		health := gateway.Health()
		checkDuration := time.Since(checkStart)
		
		performanceMetrics.TotalRequests++
		if health.IsHealthy {
			performanceMetrics.SuccessfulRequests++
		} else {
			performanceMetrics.FailedRequests++
		}
		
		// Update average latency
		performanceMetrics.AverageLatency = (performanceMetrics.AverageLatency*time.Duration(i) + checkDuration) / time.Duration(i+1)
		
		fmt.Printf("   Health check %d: healthy=%t, latency=%v, clients=%d\n", 
			i+1, health.IsHealthy, checkDuration, health.ActiveClients)
		
		time.Sleep(200 * time.Millisecond)
	}
	
	// Step 5: Monitor port allocation performance
	fmt.Println("\nStep 5: Monitoring port allocation performance...")
	
	portAllocationStart := time.Now()
	testWorkspace := filepath.Join(os.TempDir(), "perf-test-workspace")
	os.MkdirAll(testWorkspace, 0755)
	
	testPort, err := portManager.AllocatePort(testWorkspace)
	portAllocationDuration := time.Since(portAllocationStart)
	
	if err != nil {
		fmt.Printf("   ❌ Port allocation failed: %v\n", err)
	} else {
		fmt.Printf("   ✅ Port allocation: %d (took %v)\n", testPort, portAllocationDuration)
		
		// Check port availability
		isAvailable := portManager.IsPortAvailable(testPort + 1)
		fmt.Printf("   Next port (%d) available: %t\n", testPort+1, isAvailable)
		
		// Release test port
		portManager.ReleasePort(testWorkspace)
		os.RemoveAll(testWorkspace)
	}
	
	// Step 6: Display comprehensive performance report
	fmt.Println("\nStep 6: Performance monitoring report...")
	
	totalDuration := time.Since(performanceMetrics.StartTime)
	performanceMetrics.RequestsPerSecond = float64(performanceMetrics.TotalRequests) / totalDuration.Seconds()
	
	fmt.Printf("Performance Summary:\n")
	fmt.Printf("   Total Duration: %v\n", totalDuration)
	fmt.Printf("   Total Requests: %d\n", performanceMetrics.TotalRequests)
	fmt.Printf("   Successful: %d (%.1f%%)\n", 
		performanceMetrics.SuccessfulRequests,
		float64(performanceMetrics.SuccessfulRequests)/float64(performanceMetrics.TotalRequests)*100)
	fmt.Printf("   Failed: %d (%.1f%%)\n", 
		performanceMetrics.FailedRequests,
		float64(performanceMetrics.FailedRequests)/float64(performanceMetrics.TotalRequests)*100)
	fmt.Printf("   Average Latency: %v\n", performanceMetrics.AverageLatency)
	fmt.Printf("   Requests/Second: %.2f\n", performanceMetrics.RequestsPerSecond)
	fmt.Printf("   Port Utilization: %.2f%%\n", performanceMetrics.PortUtilization)
	
	// SCIP Cache Performance (simulated)
	fmt.Printf("\nSCIP Cache Performance (simulated):\n")
	fmt.Printf("   Cache Hit Rate: 87.3%%\n")
	fmt.Printf("   Average Cache Lookup: 8.2ms\n")
	fmt.Printf("   Cache Size: 245MB / 512MB\n")
	fmt.Printf("   Index Refresh Rate: every 10 minutes\n")
	
	// Step 7: Cleanup
	fmt.Println("\nStep 7: Cleaning up performance monitoring...")
	if err := gateway.Stop(); err != nil {
		log.Printf("⚠️  Error stopping gateway: %v", err)
	}
	
	if err := portManager.ReleasePort(workspaceContext.Root); err != nil {
		log.Printf("⚠️  Error releasing port: %v", err)
	}
	
	duration := time.Since(startTime)
	fmt.Printf("✅ Performance monitoring completed in %v\n", duration)
}

// Helper types and functions

type WorkspaceResult struct {
	WorkspacePath string
	ProjectType   string
	Port          int
	ActiveClients int
	Error         error
}

type PerformanceMetrics struct {
	StartTime          time.Time
	TotalRequests      int
	SuccessfulRequests int
	FailedRequests     int
	AverageLatency     time.Duration
	RequestsPerSecond  float64
	MemoryUsage        int64
	PortUtilization    float64
}

func processWorkspace(ctx context.Context, workspacePath string, detector workspace.WorkspaceDetector, 
	configManager workspace.WorkspaceConfigManager, portManager workspace.WorkspacePortManager, 
	results chan<- WorkspaceResult) {
	
	result := WorkspaceResult{WorkspacePath: workspacePath}
	
	// Detect workspace
	workspaceContext, err := detector.DetectWorkspaceAt(workspacePath)
	if err != nil {
		result.Error = fmt.Errorf("detection failed: %w", err)
		results <- result
		return
	}
	
	result.ProjectType = workspaceContext.ProjectType
	
	// Allocate port
	port, err := portManager.AllocatePort(workspacePath)
	if err != nil {
		result.Error = fmt.Errorf("port allocation failed: %w", err)
		results <- result
		return
	}
	
	result.Port = port
	
	// Generate and load config
	projectContext := &project.ProjectContext{
		ProjectType: workspaceContext.ProjectType,
		Languages:   workspaceContext.Languages,
		RootPath:    workspaceContext.Root,
	}
	
	if err := configManager.GenerateWorkspaceConfig(workspacePath, projectContext); err != nil {
		result.Error = fmt.Errorf("config generation failed: %w", err)
		results <- result
		return
	}
	
	workspaceConfig, err := configManager.LoadWorkspaceConfig(workspacePath)
	if err != nil {
		result.Error = fmt.Errorf("config loading failed: %w", err)
		results <- result
		return
	}
	
	// Create and initialize gateway
	gateway := workspace.NewWorkspaceGateway()
	gatewayConfig := &workspace.WorkspaceGatewayConfig{
		WorkspaceRoot:    workspacePath,
		ExtensionMapping: getDefaultExtensionMapping(),
		Timeout:          10 * time.Second,
		EnableLogging:    false, // Disable for batch processing
	}
	
	if err := gateway.Initialize(ctx, workspaceConfig, gatewayConfig); err != nil {
		result.Error = fmt.Errorf("gateway initialization failed: %w", err)
		results <- result
		return
	}
	
	if err := gateway.Start(ctx); err != nil {
		result.Error = fmt.Errorf("gateway start failed: %w", err)
		results <- result
		return
	}
	
	// Get health info
	health := gateway.Health()
	result.ActiveClients = health.ActiveClients
	
	// Cleanup
	gateway.Stop()
	
	results <- result
}

func createFile(path, content string) {
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		log.Printf("⚠️  Failed to create file %s: %v", path, err)
	}
}

func getDefaultExtensionMapping() map[string]string {
	return map[string]string{
		".go":   "go",
		".py":   "python",
		".pyx":  "python",  
		".pyi":  "python",
		".js":   "javascript",
		".jsx":  "javascript",
		".mjs":  "javascript",
		".cjs":  "javascript",
		".ts":   "typescript",
		".tsx":  "typescript",
		".java": "java",
		".kt":   "kotlin",
		".rs":   "rust",
		".c":    "c",
		".cpp":  "cpp",
		".cc":   "cpp", 
		".cxx":  "cpp",
		".h":    "c",
		".hpp":  "cpp",
	}
}

func sendJSONRPCRequest(url string, request map[string]interface{}) bool {
	jsonData, err := json.Marshal(request)
	if err != nil {
		return false
	}
	
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	
	return resp.StatusCode == http.StatusOK
}

func simulateWorkspaceInfoTool(workspaceContext *workspace.WorkspaceContext, 
	gateway workspace.WorkspaceGateway, args map[string]interface{}) map[string]interface{} {
	
	result := map[string]interface{}{
		"workspace_id":   workspaceContext.ID,
		"root_path":      workspaceContext.Root,
		"project_type":   workspaceContext.ProjectType,
		"languages":      workspaceContext.Languages,
		"created_at":     workspaceContext.CreatedAt.Format(time.RFC3339),
	}
	
	if includeHealth, ok := args["include_health"].(bool); ok && includeHealth {
		health := gateway.Health()
		result["health"] = map[string]interface{}{
			"is_healthy":      health.IsHealthy,
			"active_clients":  health.ActiveClients,
			"client_statuses": health.ClientStatuses,
		}
	}
	
	return result
}

func simulateSymbolSearchTool(workspaceContext *workspace.WorkspaceContext, 
	args map[string]interface{}) []map[string]interface{} {
	
	query := args["query"].(string)
	
	// Simulate symbol search results
	results := []map[string]interface{}{
		{
			"name":     query,
			"kind":     "function",
			"location": map[string]interface{}{
				"file": filepath.Join(workspaceContext.Root, "main.go"),
				"line": 5,
				"character": 0,
			},
		},
		{
			"name":     query + "Test",
			"kind":     "function", 
			"location": map[string]interface{}{
				"file": filepath.Join(workspaceContext.Root, "main_test.go"),
				"line": 10,
				"character": 0,
			},
		},
	}
	
	return results
}

func simulateHoverInfoTool(workspaceContext *workspace.WorkspaceContext, 
	gateway workspace.WorkspaceGateway, args map[string]interface{}) map[string]interface{} {
	
	filePath := args["file_path"].(string)
	line := int(args["line"].(float64))
	character := int(args["character"].(float64))
	
	// Simulate hover info result
	result := map[string]interface{}{
		"status": "success",
		"hover_info": map[string]interface{}{
			"file":      filePath,
			"line":      line,
			"character": character,
			"content":   "func main()",
			"language":  "go",
		},
	}
	
	return result
}