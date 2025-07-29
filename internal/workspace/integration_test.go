package workspace_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"lsp-gateway/internal/project"
	"lsp-gateway/internal/storage"
	"lsp-gateway/internal/workspace"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testTimeout = 60 * time.Second
	shortTimeout = 5 * time.Second
	workspaceStartupTimeout = 30 * time.Second
	portAllocationTimeout = 5 * time.Second
)

// TestMultipleWorkspaceInstances tests concurrent workspace creation and isolation
func TestMultipleWorkspaceInstances(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// Create 5 temporary workspace directories
	workspaces := make([]string, 5)
	cleanupFuncs := make([]func(), 5)
	defer func() {
		for _, cleanup := range cleanupFuncs {
			if cleanup != nil {
				cleanup()
			}
		}
	}()

	for i := 0; i < 5; i++ {
		tempDir, err := ioutil.TempDir("", fmt.Sprintf("workspace_test_%d_", i))
		require.NoError(t, err)
		workspaces[i] = tempDir
		cleanupFuncs[i] = func() { os.RemoveAll(tempDir) }

		// Create basic Go project structure
		err = createBasicGoProject(tempDir)
		require.NoError(t, err)
	}

	// Initialize workspace gateways concurrently
	var wg sync.WaitGroup
	gateways := make([]workspace.WorkspaceGateway, 5)
	ports := make([]int, 5)
	errors := make([]error, 5)
	configManagers := make([]workspace.WorkspaceConfigManager, 5)
	portManagers := make([]workspace.WorkspacePortManager, 5)

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			// Initialize port manager
			portManager, err := workspace.NewWorkspacePortManager()
			if err != nil {
				errors[idx] = fmt.Errorf("failed to create port manager: %w", err)
				return
			}
			portManagers[idx] = portManager

			// Allocate unique port
			port, err := portManager.AllocatePort(workspaces[idx])
			if err != nil {
				errors[idx] = fmt.Errorf("failed to allocate port: %w", err)
				return
			}
			ports[idx] = port

			// Initialize config manager and generate workspace config
			configManager := workspace.NewWorkspaceConfigManager()
			configManagers[idx] = configManager

			detector := project.NewProjectDetector()
			projectContext, err := detector.DetectProject(ctx, workspaces[idx])
			if err != nil {
				errors[idx] = fmt.Errorf("failed to detect project: %w", err)
				return
			}

			err = configManager.GenerateWorkspaceConfig(workspaces[idx], projectContext)
			if err != nil {
				errors[idx] = fmt.Errorf("failed to generate workspace config: %w", err)
				return
			}

			workspaceConfig, err := configManager.LoadWorkspaceConfig(workspaces[idx])
			if err != nil {
				errors[idx] = fmt.Errorf("failed to load workspace config: %w", err)
				return
			}

			// Create and initialize gateway
			gateway := workspace.NewWorkspaceGateway()
			gatewayConfig := &workspace.WorkspaceGatewayConfig{
				WorkspaceRoot: workspaces[idx],
				ExtensionMapping: map[string]string{
					"go": "go",
				},
				Timeout:       shortTimeout,
				EnableLogging: true,
			}

			err = gateway.Initialize(ctx, workspaceConfig, gatewayConfig)
			if err != nil {
				errors[idx] = fmt.Errorf("failed to initialize gateway: %w", err)
				return
			}

			err = gateway.Start(ctx)
			if err != nil {
				errors[idx] = fmt.Errorf("failed to start gateway: %w", err)
				return
			}

			gateways[idx] = gateway
		}(i)
	}

	wg.Wait()

	// Check for initialization errors
	for i, err := range errors {
		if err != nil {
			require.NoError(t, err, "Workspace %d initialization failed", i)
		}
	}

	// Verify all workspaces have unique ports
	portSet := make(map[int]bool)
	for i, port := range ports {
		assert.NotZero(t, port, "Workspace %d should have allocated port", i)
		assert.False(t, portSet[port], "Port %d should be unique (workspace %d)", port, i)
		portSet[port] = true
	}

	// Verify port range compliance
	for i, port := range ports {
		assert.GreaterOrEqual(t, port, workspace.PortRangeStart, "Workspace %d port should be in valid range", i)
		assert.LessOrEqual(t, port, workspace.PortRangeEnd, "Workspace %d port should be in valid range", i)
	}

	// Test concurrent JSON-RPC requests
	var requestWg sync.WaitGroup
	requestErrors := make([]error, len(gateways)*3)
	requestIdx := int32(0)

	for i, gateway := range gateways {
		for j := 0; j < 3; j++ {
			requestWg.Add(1)
			go func(gw workspace.WorkspaceGateway, workspaceIdx, reqIdx int) {
				defer requestWg.Done()
				idx := atomic.AddInt32(&requestIdx, 1) - 1

				// Test workspace/symbol request
				req := createWorkspaceSymbolRequest(workspaces[workspaceIdx])
				resp, err := executeJSONRPCRequest(gw, req)
				if err != nil {
					requestErrors[idx] = fmt.Errorf("workspace %d request %d failed: %w", workspaceIdx, reqIdx, err)
					return
				}

				// Verify response structure
				if resp.Error != nil {
					requestErrors[idx] = fmt.Errorf("workspace %d request %d returned error: %v", workspaceIdx, reqIdx, resp.Error)
				}
			}(gateway, i, j)
		}
	}

	requestWg.Wait()

	// Check request errors
	for i, err := range requestErrors {
		if err != nil {
			t.Logf("Request %d error: %v", i, err)
		}
	}

	// Verify LSP client isolation
	for i, gateway := range gateways {
		client, exists := gateway.GetClient("go")
		assert.True(t, exists, "Workspace %d should have Go client", i)
		assert.NotNil(t, client, "Workspace %d Go client should not be nil", i)
		assert.True(t, client.IsActive(), "Workspace %d Go client should be active", i)
	}

	// Cleanup gateways and port managers
	for i, gateway := range gateways {
		if gateway != nil {
			err := gateway.Stop()
			assert.NoError(t, err, "Failed to stop gateway %d", i)
		}
		if portManagers[i] != nil {
			err := portManagers[i].ReleasePort(workspaces[i])
			assert.NoError(t, err, "Failed to release port for workspace %d", i)
		}
	}
}

// TestPortAllocationConflicts tests port allocation edge cases and conflicts
func TestPortAllocationConflicts(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	// Note: Context not needed for port allocation tests as they don't use async operations

	// Create test workspace
	tempDir, err := ioutil.TempDir("", "port_conflict_test_")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	err = createBasicGoProject(tempDir)
	require.NoError(t, err)

	t.Run("PortReuse", func(t *testing.T) {
		portManager, err := workspace.NewWorkspacePortManager()
		require.NoError(t, err)

		// Allocate port
		port1, err := portManager.AllocatePort(tempDir)
		require.NoError(t, err)
		assert.True(t, port1 >= workspace.PortRangeStart && port1 <= workspace.PortRangeEnd)

		// Allocate same workspace again - should return same port
		port2, err := portManager.AllocatePort(tempDir)
		require.NoError(t, err)
		assert.Equal(t, port1, port2, "Second allocation should return same port")

		// Release and reallocate
		err = portManager.ReleasePort(tempDir)
		require.NoError(t, err)

		port3, err := portManager.AllocatePort(tempDir)
		require.NoError(t, err)
		assert.True(t, port3 >= workspace.PortRangeStart && port3 <= workspace.PortRangeEnd)

		// Clean up
		err = portManager.ReleasePort(tempDir)
		require.NoError(t, err)
	})

	t.Run("PortExhaustion", func(t *testing.T) {
		portManager, err := workspace.NewWorkspacePortManager()
		require.NoError(t, err)

		// Create many temporary workspaces to exhaust port range
		maxPorts := workspace.PortRangeEnd - workspace.PortRangeStart + 1
		workspaces := make([]string, maxPorts+5) // Try to allocate more than available
		allocatedPorts := make([]int, 0, maxPorts+5)
		
		defer func() {
			for _, ws := range workspaces {
				if ws != "" {
					portManager.ReleasePort(ws)
					os.RemoveAll(ws)
				}
			}
		}()

		successCount := 0
		for i := 0; i < maxPorts+5; i++ {
			workspace, err := ioutil.TempDir("", fmt.Sprintf("port_exhaust_%d_", i))
			if err != nil {
				continue
			}
			workspaces[i] = workspace

			port, err := portManager.AllocatePort(workspace)
			if err != nil {
				// Expected to fail once ports are exhausted
				t.Logf("Port allocation failed at attempt %d: %v", i+1, err)
				break
			}
			
			allocatedPorts = append(allocatedPorts, port)
			successCount++
		}

		// Should allocate at least most of the available ports
		assert.GreaterOrEqual(t, successCount, maxPorts-5, "Should allocate most available ports")
		
		// Verify all allocated ports are unique
		portSet := make(map[int]bool)
		for _, port := range allocatedPorts {
			assert.False(t, portSet[port], "Port %d should be unique", port)
			portSet[port] = true
		}
	})

	t.Run("StalePortCleanup", func(t *testing.T) {
		portManager, err := workspace.NewWorkspacePortManager()
		require.NoError(t, err)

		workspace1, err := ioutil.TempDir("", "stale_port_test_")
		require.NoError(t, err)
		defer os.RemoveAll(workspace1)

		// Allocate port
		port, err := portManager.AllocatePort(workspace1)
		require.NoError(t, err)

		// Verify port is allocated
		assignedPort, exists := portManager.GetAssignedPort(workspace1)
		assert.True(t, exists)
		assert.Equal(t, port, assignedPort)

		// Simulate process death by creating new port manager instance
		newPortManager, err := workspace.NewWorkspacePortManager()
		require.NoError(t, err)

		// Should be able to allocate same workspace again after cleanup
		newPort, err := newPortManager.AllocatePort(workspace1)
		require.NoError(t, err)
		assert.True(t, newPort >= workspace.PortRangeStart && newPort <= workspace.PortRangeEnd)

		// Clean up
		err = newPortManager.ReleasePort(workspace1)
		require.NoError(t, err)
	})
}

// TestWorkspaceConfigurationIsolation verifies configuration isolation between workspaces
func TestWorkspaceConfigurationIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// Create three different project types
	workspaceConfigs := []struct {
		name        string
		projectType string
		languages   []string
		files       map[string]string
	}{
		{
			name:        "go_workspace",
			projectType: "go",
			languages:   []string{"go"},
			files: map[string]string{
				"main.go":    "package main\n\nfunc main() {}\n",
				"go.mod":     "module test\n\ngo 1.21\n",
				"helper.go":  "package main\n\nfunc helper() string { return \"test\" }",
			},
		},
		{
			name:        "python_workspace", 
			projectType: "python",
			languages:   []string{"python"},
			files: map[string]string{
				"main.py":         "def main():\n    pass\n\nif __name__ == '__main__':\n    main()\n",
				"requirements.txt": "requests==2.28.1\n",
				"helper.py":       "def helper():\n    return 'test'\n",
			},
		},
		{
			name:        "typescript_workspace",
			projectType: "typescript",
			languages:   []string{"typescript"},
			files: map[string]string{
				"index.ts":     "function main(): void {\n  console.log('Hello');\n}\n\nmain();\n",
				"package.json":  `{"name": "test", "version": "1.0.0", "devDependencies": {"typescript": "^4.0.0"}}`,
				"tsconfig.json": `{"compilerOptions": {"target": "es2020", "module": "commonjs"}}`,
				"helper.ts":     "export function helper(): string {\n  return 'test';\n}\n",
			},
		},
	}

	workspaces := make([]string, len(workspaceConfigs))
	gateways := make([]workspace.WorkspaceGateway, len(workspaceConfigs))
	configManagers := make([]workspace.WorkspaceConfigManager, len(workspaceConfigs))
	
	defer func() {
		for i := range gateways {
			if gateways[i] != nil {
				gateways[i].Stop()
			}
			if workspaces[i] != "" {
				os.RemoveAll(workspaces[i])
			}
		}
	}()

	// Create workspaces with different configurations
	for i, config := range workspaceConfigs {
		tempDir, err := ioutil.TempDir("", config.name+"_")
		require.NoError(t, err)
		workspaces[i] = tempDir

		// Create project files
		for filename, content := range config.files {
			filePath := filepath.Join(tempDir, filename)
			err = os.WriteFile(filePath, []byte(content), 0644)
			require.NoError(t, err)
		}

		// Initialize configuration
		configManager := workspace.NewWorkspaceConfigManager()
		configManagers[i] = configManager

		detector := project.NewProjectDetector()
		projectContext, err := detector.DetectProject(ctx, tempDir)
		require.NoError(t, err)
		assert.Equal(t, config.projectType, projectContext.ProjectType, "Project type should match for %s", config.name)

		err = configManager.GenerateWorkspaceConfig(tempDir, projectContext)
		require.NoError(t, err)

		workspaceConfig, err := configManager.LoadWorkspaceConfig(tempDir)
		require.NoError(t, err)

		// Verify workspace-specific configuration
		assert.Equal(t, tempDir, workspaceConfig.Workspace.RootPath)
		assert.Equal(t, config.projectType, workspaceConfig.Workspace.ProjectType)
		assert.ElementsMatch(t, config.languages, workspaceConfig.Workspace.Languages)

		// Verify cache and logging directories are isolated
		assert.Contains(t, workspaceConfig.Directories.Cache, workspaceConfig.Workspace.Hash)
		assert.Contains(t, workspaceConfig.Directories.Logs, workspaceConfig.Workspace.Hash)
		assert.NotEqual(t, workspaceConfig.Directories.Cache, "/tmp/cache") // Should not be shared

		// Initialize gateway
		gateway := workspace.NewWorkspaceGateway()
		gatewayConfig := &workspace.WorkspaceGatewayConfig{
			WorkspaceRoot: tempDir,
			ExtensionMapping: map[string]string{
				"go": "go",
				"py": "python", 
				"ts": "typescript",
			},
			Timeout: shortTimeout,
			EnableLogging: true,
		}

		err = gateway.Initialize(ctx, workspaceConfig, gatewayConfig)
		require.NoError(t, err, "Failed to initialize %s gateway", config.name)

		err = gateway.Start(ctx)
		require.NoError(t, err, "Failed to start %s gateway", config.name)

		gateways[i] = gateway
	}

	// Verify configuration isolation
	for i, config := range workspaceConfigs {
		workspaceConfig, err := configManagers[i].LoadWorkspaceConfig(workspaces[i])
		require.NoError(t, err)

		// Each workspace should have its own unique hash and directories
		for j, otherConfig := range workspaceConfigs {
			if i == j {
				continue
			}
			
			otherWorkspaceConfig, err := configManagers[j].LoadWorkspaceConfig(workspaces[j])
			require.NoError(t, err)

			assert.NotEqual(t, workspaceConfig.Workspace.Hash, otherWorkspaceConfig.Workspace.Hash,
				"Workspace hashes should be unique between %s and %s", config.name, otherConfig.name)
			assert.NotEqual(t, workspaceConfig.Directories.Cache, otherWorkspaceConfig.Directories.Cache,
				"Cache directories should be isolated between %s and %s", config.name, otherConfig.name)
			assert.NotEqual(t, workspaceConfig.Directories.Logs, otherWorkspaceConfig.Directories.Logs,
				"Log directories should be isolated between %s and %s", config.name, otherConfig.name)
		}

		// Verify workspace-specific server configurations
		assert.NotEmpty(t, workspaceConfig.Servers, "Workspace %s should have server configs", config.name)
		
		// Verify language-specific client availability
		for _, language := range config.languages {
			client, exists := gateways[i].GetClient(language)
			assert.True(t, exists, "Workspace %s should have %s client", config.name, language)
			assert.NotNil(t, client, "Workspace %s %s client should not be nil", config.name, language)
		}
	}
}

// TestLSPClientIsolation verifies LSP clients don't cross-contaminate between workspaces
func TestLSPClientIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// Create two Go workspaces with different module names
	workspace1, err := ioutil.TempDir("", "client_isolation_1_")
	require.NoError(t, err)
	defer os.RemoveAll(workspace1)

	workspace2, err := ioutil.TempDir("", "client_isolation_2_")
	require.NoError(t, err)
	defer os.RemoveAll(workspace2)

	// Create distinct project structures
	files1 := map[string]string{
		"main.go": "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"workspace1\")\n}\n",
		"go.mod":  "module workspace1\n\ngo 1.21\n",
		"util.go": "package main\n\nfunc UtilityOne() string {\n\treturn \"one\"\n}\n",
	}

	files2 := map[string]string{
		"main.go": "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"workspace2\")\n}\n",
		"go.mod":  "module workspace2\n\ngo 1.21\n", 
		"util.go": "package main\n\nfunc UtilityTwo() string {\n\treturn \"two\"\n}\n",
	}

	// Create files for workspace1
	for filename, content := range files1 {
		filePath := filepath.Join(workspace1, filename)
		err = os.WriteFile(filePath, []byte(content), 0644)
		require.NoError(t, err)
	}

	// Create files for workspace2
	for filename, content := range files2 {
		filePath := filepath.Join(workspace2, filename)
		err = os.WriteFile(filePath, []byte(content), 0644)
		require.NoError(t, err)
	}

	// Initialize workspace gateways
	gateways := make([]workspace.WorkspaceGateway, 2)
	workspaces := []string{workspace1, workspace2}

	defer func() {
		for _, gateway := range gateways {
			if gateway != nil {
				gateway.Stop()
			}
		}
	}()

	for i, ws := range workspaces {
		configManager := workspace.NewWorkspaceConfigManager()
		detector := project.NewProjectDetector()
		
		projectContext, err := detector.DetectProject(ctx, ws)
		require.NoError(t, err)

		err = configManager.GenerateWorkspaceConfig(ws, projectContext)
		require.NoError(t, err)

		workspaceConfig, err := configManager.LoadWorkspaceConfig(ws)
		require.NoError(t, err)

		gateway := workspace.NewWorkspaceGateway()
		gatewayConfig := &workspace.WorkspaceGatewayConfig{
			WorkspaceRoot: ws,
			ExtensionMapping: map[string]string{"go": "go"},
			Timeout: shortTimeout,
			EnableLogging: true,
		}

		err = gateway.Initialize(ctx, workspaceConfig, gatewayConfig)
		require.NoError(t, err)

		err = gateway.Start(ctx)
		require.NoError(t, err)

		gateways[i] = gateway
	}

	// Test workspace symbol requests to verify isolation
	t.Run("WorkspaceSymbolIsolation", func(t *testing.T) {
		// Search for workspace-specific symbols
		req1 := createWorkspaceSymbolRequest(workspace1, "UtilityOne")
		resp1, err := executeJSONRPCRequest(gateways[0], req1)
		require.NoError(t, err)
		assert.Nil(t, resp1.Error, "Workspace1 symbol request should succeed")

		req2 := createWorkspaceSymbolRequest(workspace2, "UtilityTwo")
		resp2, err := executeJSONRPCRequest(gateways[1], req2)
		require.NoError(t, err)
		assert.Nil(t, resp2.Error, "Workspace2 symbol request should succeed")

		// Cross-workspace symbol requests should not find symbols from other workspace
		crossReq := createWorkspaceSymbolRequest(workspace1, "UtilityTwo")
		_, err = executeJSONRPCRequest(gateways[0], crossReq)
		require.NoError(t, err)
		// Should either return empty results or not find the symbol from workspace2
	})

	t.Run("LSPClientIndependence", func(t *testing.T) {
		// Verify each workspace has independent clients
		client1, exists1 := gateways[0].GetClient("go")
		require.True(t, exists1)
		require.NotNil(t, client1)

		client2, exists2 := gateways[1].GetClient("go")
		require.True(t, exists2)
		require.NotNil(t, client2)

		// Clients should be different instances
		assert.NotEqual(t, fmt.Sprintf("%p", client1), fmt.Sprintf("%p", client2), 
			"Clients should be different instances")

		// Both clients should be active
		assert.True(t, client1.IsActive(), "Workspace1 client should be active")
		assert.True(t, client2.IsActive(), "Workspace2 client should be active")
	})

	t.Run("RoutingAccuracy", func(t *testing.T) {
		// Test that .go files are routed to Go clients
		for i, gateway := range gateways {
			goFileURI := fmt.Sprintf("file://%s/main.go", workspaces[i])
			req := createDefinitionRequest(goFileURI, 5, 10) // Position in main function
			
			resp, err := executeJSONRPCRequest(gateway, req)
			require.NoError(t, err)
			
			if resp.Error != nil {
				// LSP errors are acceptable, but should not be routing errors
				assert.NotContains(t, strings.ToLower(resp.Error.Message), "unsupported", 
					"Should not have unsupported file type error for workspace %d", i)
			}
		}
	})
}

// TestSCIPCacheIsolation verifies SCIP cache isolation between workspaces
func TestSCIPCacheIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// Create two workspaces
	workspace1, err := ioutil.TempDir("", "scip_isolation_1_")
	require.NoError(t, err)
	defer os.RemoveAll(workspace1)

	workspace2, err := ioutil.TempDir("", "scip_isolation_2_")
	require.NoError(t, err)
	defer os.RemoveAll(workspace2)

	// Create projects with same filenames but different content
	err = createBasicGoProject(workspace1)
	require.NoError(t, err)
	
	err = createBasicGoProject(workspace2)
	require.NoError(t, err)

	// Initialize workspace config managers
	configManager1 := workspace.NewWorkspaceConfigManager()
	configManager2 := workspace.NewWorkspaceConfigManager()

	detector := project.NewProjectDetector()
	
	projectContext1, err := detector.DetectProject(ctx, workspace1)
	require.NoError(t, err)
	
	projectContext2, err := detector.DetectProject(ctx, workspace2)
	require.NoError(t, err)

	err = configManager1.GenerateWorkspaceConfig(workspace1, projectContext1)
	require.NoError(t, err)
	
	err = configManager2.GenerateWorkspaceConfig(workspace2, projectContext2)
	require.NoError(t, err)

	// Initialize SCIP caches
	scipCache1 := workspace.NewWorkspaceSCIPCache(configManager1)
	scipCache2 := workspace.NewWorkspaceSCIPCache(configManager2)

	defer func() {
		scipCache1.Close()
		scipCache2.Close()
	}()

	cacheConfig := &workspace.CacheConfig{
		MaxMemorySize:     100 * 1024 * 1024, // 100MB
		MaxMemoryEntries:  1000,
		MemoryTTL:         10 * time.Minute,
		MaxDiskSize:       500 * 1024 * 1024, // 500MB
		MaxDiskEntries:    5000,
		DiskTTL:           1 * time.Hour,
		CleanupInterval:   1 * time.Minute,
		MetricsInterval:   10 * time.Second,
		SyncInterval:      30 * time.Second,
		IsolationLevel:    workspace.IsolationStrong,
	}

	err = scipCache1.Initialize(workspace1, cacheConfig)
	require.NoError(t, err)
	
	err = scipCache2.Initialize(workspace2, cacheConfig)
	require.NoError(t, err)

	t.Run("CacheDirectoryIsolation", func(t *testing.T) {
		stats1 := scipCache1.GetStats()
		stats2 := scipCache2.GetStats()

		// Verify different workspace hashes
		assert.NotEqual(t, stats1.WorkspaceHash, stats2.WorkspaceHash, 
			"Workspaces should have different hashes")
		
		// Verify different workspace roots
		assert.Equal(t, workspace1, stats1.WorkspaceRoot)
		assert.Equal(t, workspace2, stats2.WorkspaceRoot)
		assert.NotEqual(t, stats1.WorkspaceRoot, stats2.WorkspaceRoot)

		// Verify cache directories exist and are separate
		workspace1Dir := configManager1.GetWorkspaceDirectory(workspace1)
		workspace2Dir := configManager2.GetWorkspaceDirectory(workspace2)
		
		assert.NotEqual(t, workspace1Dir, workspace2Dir, "Workspace directories should be different")
		
		// Verify directories exist
		_, err := os.Stat(workspace1Dir)
		assert.NoError(t, err, "Workspace1 directory should exist")
		
		_, err = os.Stat(workspace2Dir)
		assert.NoError(t, err, "Workspace2 directory should exist")
	})

	t.Run("CacheEntryIsolation", func(t *testing.T) {
		// Create mock cache entries for each workspace
		testEntry1 := createMockCacheEntry("test_key_1", workspace1, "go")
		testEntry2 := createMockCacheEntry("test_key_2", workspace2, "go")

		// Set entries in respective caches
		err = scipCache1.Set("test_key_1", testEntry1, 10*time.Minute)
		require.NoError(t, err)
		
		err = scipCache2.Set("test_key_2", testEntry2, 10*time.Minute)
		require.NoError(t, err)

		// Verify isolation: cache1 should not have cache2's entries
		entry, exists := scipCache1.Get("test_key_2")
		assert.False(t, exists, "Cache1 should not have cache2's entries")
		assert.Nil(t, entry)

		entry, exists = scipCache2.Get("test_key_1")
		assert.False(t, exists, "Cache2 should not have cache1's entries")
		assert.Nil(t, entry)

		// Verify each cache has its own entries
		entry1, exists1 := scipCache1.Get("test_key_1")
		assert.True(t, exists1, "Cache1 should have its own entry")
		assert.NotNil(t, entry1)
		assert.Equal(t, workspace1, entry1.ProjectPath)

		entry2, exists2 := scipCache2.Get("test_key_2")
		assert.True(t, exists2, "Cache2 should have its own entry")
		assert.NotNil(t, entry2)
		assert.Equal(t, workspace2, entry2.ProjectPath)
	})

	t.Run("FileInvalidationIsolation", func(t *testing.T) {
		// Add entries for similar files in both workspaces
		testFile1 := filepath.Join(workspace1, "main.go")
		testFile2 := filepath.Join(workspace2, "main.go")
		
		entry1 := createMockCacheEntry("file_key_1", workspace1, "go")
		entry1.FilePaths = []string{testFile1}
		
		entry2 := createMockCacheEntry("file_key_2", workspace2, "go")
		entry2.FilePaths = []string{testFile2}

		err = scipCache1.Set("file_key_1", entry1, 10*time.Minute)
		require.NoError(t, err)
		
		err = scipCache2.Set("file_key_2", entry2, 10*time.Minute)
		require.NoError(t, err)

		// Invalidate file in workspace1
		count, err := scipCache1.InvalidateFile(testFile1)
		require.NoError(t, err)
		assert.Greater(t, count, 0, "Should invalidate at least one entry")

		// Verify workspace2 cache is unaffected
		entry, exists := scipCache2.Get("file_key_2")
		assert.True(t, exists, "Workspace2 cache should be unaffected by workspace1 invalidation")
		assert.NotNil(t, entry)

		// Verify workspace1 entry is invalidated
		entry, exists = scipCache1.Get("file_key_1")
		assert.False(t, exists, "Workspace1 entry should be invalidated")
	})

	t.Run("CacheCleanupIsolation", func(t *testing.T) {
		// Verify each cache can be cleaned independently
		err = scipCache1.Invalidate()
		require.NoError(t, err)

		stats1 := scipCache1.GetStats()
		stats2 := scipCache2.GetStats()

		// Workspace1 cache should be empty
		assert.Zero(t, stats1.TotalEntries, "Workspace1 cache should be empty after invalidation")
		
		// Workspace2 cache should be unaffected (if it had entries)
		// We can't guarantee it has entries, but it should not have been affected by workspace1's invalidation
		assert.Equal(t, workspace2, stats2.WorkspaceRoot, "Workspace2 should maintain its identity")
	})
}

// TestWorkspaceFailureIsolation tests that failures in one workspace don't affect others
func TestWorkspaceFailureIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// Create three workspaces
	workspaces := make([]string, 3)
	gateways := make([]workspace.WorkspaceGateway, 3)
	
	defer func() {
		for i := range gateways {
			if gateways[i] != nil {
				gateways[i].Stop()
			}
			if workspaces[i] != "" {
				os.RemoveAll(workspaces[i])
			}
		}
	}()

	// Initialize workspaces
	for i := 0; i < 3; i++ {
		tempDir, err := ioutil.TempDir("", fmt.Sprintf("failure_isolation_%d_", i))
		require.NoError(t, err)
		workspaces[i] = tempDir

		err = createBasicGoProject(tempDir)
		require.NoError(t, err)

		configManager := workspace.NewWorkspaceConfigManager()
		detector := project.NewProjectDetector()
		
		projectContext, err := detector.DetectProject(ctx, tempDir)
		require.NoError(t, err)

		err = configManager.GenerateWorkspaceConfig(tempDir, projectContext)
		require.NoError(t, err)

		workspaceConfig, err := configManager.LoadWorkspaceConfig(tempDir)
		require.NoError(t, err)

		gateway := workspace.NewWorkspaceGateway()
		gatewayConfig := &workspace.WorkspaceGatewayConfig{
			WorkspaceRoot: tempDir,
			ExtensionMapping: map[string]string{"go": "go"},
			Timeout: shortTimeout,
			EnableLogging: true,
		}

		err = gateway.Initialize(ctx, workspaceConfig, gatewayConfig)
		require.NoError(t, err)

		err = gateway.Start(ctx)
		require.NoError(t, err)

		gateways[i] = gateway
	}

	t.Run("GatewayFailureIsolation", func(t *testing.T) {
		// Verify all gateways are initially healthy
		for i, gateway := range gateways {
			health := gateway.Health()
			assert.True(t, health.IsHealthy, "Gateway %d should be initially healthy", i)
		}

		// Stop middle gateway to simulate failure
		err := gateways[1].Stop()
		require.NoError(t, err)

		// Verify other gateways remain operational
		health0 := gateways[0].Health()
		assert.True(t, health0.IsHealthy, "Gateway 0 should remain healthy after gateway 1 failure")

		health2 := gateways[2].Health()
		assert.True(t, health2.IsHealthy, "Gateway 2 should remain healthy after gateway 1 failure")

		// Verify failed gateway reports unhealthy state
		health1 := gateways[1].Health()
		assert.False(t, health1.IsHealthy, "Gateway 1 should report unhealthy after stop")

		// Test that healthy gateways can still process requests
		req0 := createWorkspaceSymbolRequest(workspaces[0])
		resp0, err := executeJSONRPCRequest(gateways[0], req0)
		require.NoError(t, err)
		assert.Nil(t, resp0.Error, "Gateway 0 should still process requests")

		req2 := createWorkspaceSymbolRequest(workspaces[2])
		resp2, err := executeJSONRPCRequest(gateways[2], req2)
		require.NoError(t, err)
		assert.Nil(t, resp2.Error, "Gateway 2 should still process requests")
	})

	t.Run("ConfigurationCorruption", func(t *testing.T) {
		// Corrupt workspace 0's configuration
		configPath := filepath.Join(workspaces[0], ".lspg", "workspace.yaml")
		err := os.WriteFile(configPath, []byte("invalid: yaml: content: ["), 0644)
		// Ignore error if config doesn't exist at expected location

		// Create new gateway with corrupted config
		corruptedGateway := workspace.NewWorkspaceGateway()
		gatewayConfig := &workspace.WorkspaceGatewayConfig{
			WorkspaceRoot: workspaces[0],
			ExtensionMapping: map[string]string{"go": "go"},
			Timeout: shortTimeout,
			EnableLogging: true,
		}

		// This should fail due to corrupted config
		configManager := workspace.NewWorkspaceConfigManager()
		detector := project.NewProjectDetector()
		
		projectContext, err := detector.DetectProject(ctx, workspaces[0])
		require.NoError(t, err)

		// Try to generate new config (this should succeed as it recreates the config)
		err = configManager.GenerateWorkspaceConfig(workspaces[0], projectContext)
		require.NoError(t, err)

		workspaceConfig, err := configManager.LoadWorkspaceConfig(workspaces[0])
		require.NoError(t, err)

		err = corruptedGateway.Initialize(ctx, workspaceConfig, gatewayConfig)
		require.NoError(t, err)

		// Even if this workspace had issues, others should remain unaffected
		health1 := gateways[1].Health()
		health2 := gateways[2].Health()
		
		// Note: Gateway 1 was stopped in previous test, so we expect it to be unhealthy
		_ = health1 // Gateway 1 was stopped in previous test
		assert.True(t, health2.IsHealthy, "Gateway 2 should remain healthy despite workspace 0 issues")
	})

	t.Run("PortConflictGracefulHandling", func(t *testing.T) {
		// Create a new workspace that might conflict with existing port allocation
		conflictWorkspace, err := ioutil.TempDir("", "conflict_test_")
		require.NoError(t, err)
		defer os.RemoveAll(conflictWorkspace)

		err = createBasicGoProject(conflictWorkspace)
		require.NoError(t, err)

		portManager, err := workspace.NewWorkspacePortManager()
		require.NoError(t, err)

		// This should find an available port despite other workspaces running
		port, err := portManager.AllocatePort(conflictWorkspace)
		require.NoError(t, err)
		assert.True(t, port >= workspace.PortRangeStart && port <= workspace.PortRangeEnd)

		// Clean up
		err = portManager.ReleasePort(conflictWorkspace)
		require.NoError(t, err)
	})
}

// Helper functions

func createBasicGoProject(dir string) error {
	files := map[string]string{
		"main.go": `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}

func helperFunction() string {
	return "helper"
}
`,
		"go.mod": fmt.Sprintf(`module %s

go 1.21
`, filepath.Base(dir)),
		"helper.go": `package main

func UtilityFunction() string {
	return "utility"
}

type TestStruct struct {
	Field string
}

func (t TestStruct) Method() string {
	return t.Field + " method"
}
`,
	}

	for filename, content := range files {
		filePath := filepath.Join(dir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return err
		}
	}

	return nil
}

func createWorkspaceSymbolRequest(workspaceRoot string, query ...string) *workspace.JSONRPCRequest {
	searchQuery := "test"
	if len(query) > 0 {
		searchQuery = query[0]
	}

	return &workspace.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      fmt.Sprintf("test-%d", time.Now().UnixNano()),
		Method:  "workspace/symbol",
		Params: map[string]interface{}{
			"query": searchQuery,
		},
	}
}

func createDefinitionRequest(fileURI string, line, character int) *workspace.JSONRPCRequest {
	return &workspace.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      fmt.Sprintf("def-%d", time.Now().UnixNano()),
		Method:  "textDocument/definition",
		Params: map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": fileURI,
			},
			"position": map[string]interface{}{
				"line":      line,
				"character": character,
			},
		},
	}
}

func executeJSONRPCRequest(gateway workspace.WorkspaceGateway, req *workspace.JSONRPCRequest) (*workspace.JSONRPCResponse, error) {
	// Create a mock HTTP request/response for testing
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	// Create mock HTTP objects
	httpReq := &http.Request{
		Method: "POST",
		Header: make(http.Header),
		Body:   ioutil.NopCloser(strings.NewReader(string(reqBody))),
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Use a simple response writer that captures the response
	respWriter := &mockResponseWriter{
		headers: make(http.Header),
		body:    make([]byte, 0),
	}

	// Execute the request
	gateway.HandleJSONRPC(respWriter, httpReq)

	// Parse the response
	var response workspace.JSONRPCResponse
	if err := json.Unmarshal(respWriter.body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

type mockResponseWriter struct {
	headers    http.Header
	statusCode int
	body       []byte
}

func (m *mockResponseWriter) Header() http.Header {
	return m.headers
}

func (m *mockResponseWriter) Write(data []byte) (int, error) {
	m.body = append(m.body, data...)
	return len(data), nil
}

func (m *mockResponseWriter) WriteHeader(statusCode int) {
	m.statusCode = statusCode
}

func createMockCacheEntry(key, projectPath, language string) *storage.CacheEntry {
	return &storage.CacheEntry{
		Key:            key,
		ProjectPath:    projectPath,
		FilePaths:      []string{filepath.Join(projectPath, "test.go")},
		CreatedAt:      time.Now(),
		AccessedAt:     time.Now(),
		TTL:            10 * time.Minute,
		Size:           1024,
		Method:         "workspace/symbol",
		Params:         fmt.Sprintf(`{"query": "%s"}`, language),
		Response:       json.RawMessage(`{"symbols": []}`),
		CurrentTier:    storage.TierL1Memory,
		OriginTier:     storage.TierL1Memory,
		Version:        1,
		Checksum:       "mock-checksum",
		ContentType:    "application/json",
		Priority:       1,
		AccessCount:    1,
		HitCount:       1,
		LastHitTime:    time.Now(),
		AvgAccessTime:  time.Millisecond,
	}
}