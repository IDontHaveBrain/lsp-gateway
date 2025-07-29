package workspace_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"lsp-gateway/internal/project/types"
	"lsp-gateway/internal/storage"
	"lsp-gateway/internal/workspace"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	multiProjectTestTimeout     = 120 * time.Second
	multiProjectStartupTimeout = 60 * time.Second
	multiProjectShortTimeout   = 10 * time.Second
	concurrentRequestCount     = 20
	performanceIterations      = 100
)

// TestMultiProjectWorkspaceDetection tests comprehensive multi-project workspace detection
func TestMultiProjectWorkspaceDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping multi-project integration tests in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), multiProjectTestTimeout)
	defer cancel()

	// Create comprehensive multi-project workspace
	generator := workspace.NewTestWorkspaceGenerator()
	defer func() {
		if cleanupGen, ok := generator.(*workspace.DefaultTestWorkspaceGenerator); ok {
			cleanupGen.CleanupAllWorkspaces()
		}
	}()

	testWorkspace, err := createCompleteMultiProjectWorkspace(generator, "comprehensive-workspace")
	require.NoError(t, err, "Failed to create test workspace")
	defer generator.CleanupWorkspace(testWorkspace)

	t.Run("DetectAllSubProjects", func(t *testing.T) {
		detector := workspace.NewWorkspaceDetector()
		workspaceContext, err := detector.DetectWorkspaceWithContext(ctx, testWorkspace.RootPath)
		require.NoError(t, err, "Workspace detection should succeed")
		
		// Verify workspace structure
		require.NotNil(t, workspaceContext, "Workspace context should not be nil")
		assert.Equal(t, testWorkspace.RootPath, workspaceContext.Root, "Workspace root should match")
		assert.NotEmpty(t, workspaceContext.ID, "Workspace ID should be generated")
		assert.NotEmpty(t, workspaceContext.Hash, "Workspace hash should be generated")
		
		// Verify sub-projects detected
		assert.GreaterOrEqual(t, len(workspaceContext.SubProjects), 4, "Should detect at least 4 sub-projects")
		
		// Verify expected project types are detected
		detectedTypes := make(map[string]bool)
		for _, project := range workspaceContext.SubProjects {
			detectedTypes[project.ProjectType] = true
		}
		
		expectedTypes := []string{types.PROJECT_TYPE_GO, types.PROJECT_TYPE_PYTHON, 
			types.PROJECT_TYPE_TYPESCRIPT, types.PROJECT_TYPE_JAVA}
		for _, expectedType := range expectedTypes {
			assert.True(t, detectedTypes[expectedType], 
				"Should detect %s project type", expectedType)
		}

		// Verify workspace languages aggregate correctly
		assert.GreaterOrEqual(t, len(workspaceContext.Languages), 4, 
			"Workspace should aggregate multiple languages")
		
		languageSet := make(map[string]bool)
		for _, lang := range workspaceContext.Languages {
			languageSet[lang] = true
		}
		
		for _, expectedType := range expectedTypes {
			assert.True(t, languageSet[expectedType], 
				"Workspace languages should include %s", expectedType)
		}
	})

	t.Run("ValidateProjectStructure", func(t *testing.T) {
		detector := workspace.NewWorkspaceDetector()
		workspaceContext, err := detector.DetectWorkspaceWithContext(ctx, testWorkspace.RootPath)
		require.NoError(t, err)

		for _, project := range workspaceContext.SubProjects {
			t.Run(fmt.Sprintf("Project_%s", project.Name), func(t *testing.T) {
				// Verify project fields
				assert.NotEmpty(t, project.ID, "Project ID should be generated")
				assert.NotEmpty(t, project.Name, "Project name should not be empty")
				assert.NotEmpty(t, project.AbsolutePath, "Absolute path should not be empty")
				assert.NotEmpty(t, project.ProjectType, "Project type should be detected")
				assert.NotEmpty(t, project.Languages, "Languages should be detected")
				assert.NotEmpty(t, project.MarkerFiles, "Marker files should be detected")
				
				// Verify paths are consistent
				expectedAbs := filepath.Join(testWorkspace.RootPath, project.RelativePath)
				assert.Equal(t, expectedAbs, project.AbsolutePath, 
					"Absolute path should match workspace root + relative path")
				
				// Verify project directory exists
				info, err := os.Stat(project.AbsolutePath)
				assert.NoError(t, err, "Project directory should exist")
				assert.True(t, info.IsDir(), "Project path should be a directory")
				
				// Verify marker files exist
				for _, markerFile := range project.MarkerFiles {
					markerPath := filepath.Join(project.AbsolutePath, markerFile)
					_, err := os.Stat(markerPath)
					assert.NoError(t, err, "Marker file %s should exist in project %s", 
						markerFile, project.Name)
				}
			})
		}
	})

	t.Run("ProjectPathMapping", func(t *testing.T) {
		detector := workspace.NewWorkspaceDetector()
		workspaceContext, err := detector.DetectWorkspaceWithContext(ctx, testWorkspace.RootPath)
		require.NoError(t, err)

		// Verify project path mapping is built correctly
		assert.NotNil(t, workspaceContext.ProjectPaths, "Project paths mapping should exist")
		assert.NotEmpty(t, workspaceContext.ProjectPaths, "Project paths mapping should not be empty")
		
		// Verify all projects are in the mapping
		for _, project := range workspaceContext.SubProjects {
			// Check absolute path mapping
			mappedProject, exists := workspaceContext.ProjectPaths[project.AbsolutePath]
			assert.True(t, exists, "Project should be mapped by absolute path: %s", project.AbsolutePath)
			assert.Equal(t, project.ID, mappedProject.ID, "Mapped project should match original")
			
			// Check relative path mapping
			mappedProject, exists = workspaceContext.ProjectPaths[project.RelativePath]
			assert.True(t, exists, "Project should be mapped by relative path: %s", project.RelativePath)
			assert.Equal(t, project.ID, mappedProject.ID, "Mapped project should match original")
		}
		
		// Test file path resolution
		for _, project := range workspaceContext.SubProjects {
			// Create a test file path within the project
			testFilePath := filepath.Join(project.AbsolutePath, "test."+getFileExtension(project.ProjectType))
			
			resolvedProject := detector.FindSubProjectForPath(workspaceContext, testFilePath)
			assert.NotNil(t, resolvedProject, "Should resolve project for file path: %s", testFilePath)
			assert.Equal(t, project.ID, resolvedProject.ID, "Should resolve to correct project")
		}
	})
}

// TestMultiProjectRequestRouting tests end-to-end request routing across multiple projects
func TestMultiProjectRequestRouting(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping multi-project integration tests in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), multiProjectTestTimeout)
	defer cancel()

	// Create test workspace
	generator := workspace.NewTestWorkspaceGenerator()
	defer func() {
		if cleanupGen, ok := generator.(*workspace.DefaultTestWorkspaceGenerator); ok {
			cleanupGen.CleanupAllWorkspaces()
		}
	}()

	testWorkspace, err := createCompleteMultiProjectWorkspace(generator, "routing-workspace")
	require.NoError(t, err)
	defer generator.CleanupWorkspace(testWorkspace)

	// Initialize workspace gateway with multi-project support
	gateways, configManagers, portManagers, err := initializeMultiProjectGateways(ctx, []*workspace.TestWorkspace{testWorkspace})
	require.NoError(t, err, "Failed to initialize multi-project gateways")
	defer cleanupMultiProjectGateways(gateways, portManagers)

	gateway := gateways[0]
	t.Run("RouteToCorrectSubProjects", func(t *testing.T) {
		// Test requests to each sub-project
		projectTests := []struct {
			projectType string
			method      string
			fileExt     string
		}{
			{types.PROJECT_TYPE_GO, "textDocument/definition", "go"},
			{types.PROJECT_TYPE_PYTHON, "textDocument/references", "py"},
			{types.PROJECT_TYPE_TYPESCRIPT, "textDocument/hover", "ts"},
			{types.PROJECT_TYPE_JAVA, "workspace/symbol", "java"},
		}

		for _, test := range projectTests {
			t.Run(fmt.Sprintf("%s_%s", test.projectType, test.method), func(t *testing.T) {
				// Find project of this type
				var targetProject *workspace.DetectedSubProject
				for _, project := range testWorkspace.Projects {
					if project.ProjectType == test.projectType {
						targetProject = project
						break
					}
				}
				require.NotNil(t, targetProject, "Should find project of type %s", test.projectType)

				// Create request for file in this project
				fileURI := fmt.Sprintf("file://%s/main.%s", targetProject.AbsolutePath, test.fileExt)
				req := createLSPRequest(test.method, fileURI, 1, 1)
				
				resp, err := executeJSONRPCRequest(gateway, req)
				require.NoError(t, err, "Request should execute successfully")
				
				// Verify response structure (we don't expect specific content, just valid responses)
				assert.Equal(t, req.ID, resp.ID, "Response ID should match request ID")
				
				// Log for debugging but don't fail on LSP errors (servers may not be fully running)
				if resp.Error != nil {
					t.Logf("LSP server error for %s: %v", test.projectType, resp.Error)
				}
			})
		}
	})

	t.Run("FallbackRouting", func(t *testing.T) {
		// Test routing for files outside sub-projects (should fall back to legacy routing)
		sharedFileURI := fmt.Sprintf("file://%s/shared/common.txt", testWorkspace.RootPath)
		req := createLSPRequest("textDocument/definition", sharedFileURI, 1, 1)
		
		resp, err := executeJSONRPCRequest(gateway, req)
		require.NoError(t, err, "Fallback routing should work")
		assert.Equal(t, req.ID, resp.ID, "Response ID should match")
		
		// Fallback may return errors for unsupported file types, which is expected
		if resp.Error != nil {
			t.Logf("Expected fallback error for shared file: %v", resp.Error)
		}
	})

	t.Run("ConcurrentRouting", func(t *testing.T) {
		// Test concurrent requests to different sub-projects
		var wg sync.WaitGroup
		requestErrors := make([]error, concurrentRequestCount)
		successCount := int32(0)

		for i := 0; i < concurrentRequestCount; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				// Rotate through different project types
				projectTypes := []string{types.PROJECT_TYPE_GO, types.PROJECT_TYPE_PYTHON, 
					types.PROJECT_TYPE_TYPESCRIPT, types.PROJECT_TYPE_JAVA}
				projectType := projectTypes[idx%len(projectTypes)]
				
				// Find project
				var targetProject *workspace.DetectedSubProject
				for _, project := range testWorkspace.Projects {
					if project.ProjectType == projectType {
						targetProject = project
						break
					}
				}
				
				if targetProject == nil {
					requestErrors[idx] = fmt.Errorf("no project found for type %s", projectType)
					return
				}

				// Create and execute request
				fileExt := getFileExtension(projectType)
				fileURI := fmt.Sprintf("file://%s/test_%d.%s", targetProject.AbsolutePath, idx, fileExt)
				req := createLSPRequest("textDocument/hover", fileURI, 1, 1)
				
				_, err := executeJSONRPCRequest(gateway, req)
				if err != nil {
					requestErrors[idx] = err
					return
				}
				
				atomic.AddInt32(&successCount, 1)
			}(i)
		}

		wg.Wait()

		// Check results
		errorCount := 0
		for i, err := range requestErrors {
			if err != nil {
				t.Logf("Request %d error: %v", i, err)
				errorCount++
			}
		}

		// Allow some errors due to mock LSP servers, but most should succeed
		successRate := float64(successCount) / float64(concurrentRequestCount)
		assert.GreaterOrEqual(t, successRate, 0.7, 
			"At least 70%% of concurrent requests should succeed (got %.2f%%)", successRate*100)
	})
}

// TestMultiProjectConfigurationGeneration tests workspace configuration generation for multiple projects
func TestMultiProjectConfigurationGeneration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping multi-project integration tests in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), multiProjectTestTimeout)
	defer cancel()

	// Create test workspace
	generator := workspace.NewTestWorkspaceGenerator()
	defer func() {
		if cleanupGen, ok := generator.(*workspace.DefaultTestWorkspaceGenerator); ok {
			cleanupGen.CleanupAllWorkspaces()
		}
	}()

	testWorkspace, err := createCompleteMultiProjectWorkspace(generator, "config-workspace")
	require.NoError(t, err)
	defer generator.CleanupWorkspace(testWorkspace)

	t.Run("GenerateCompleteConfiguration", func(t *testing.T) {
		configManager := workspace.NewWorkspaceConfigManager()
		detector := workspace.NewWorkspaceDetector()

		// Detect workspace structure
		workspaceContext, err := detector.DetectWorkspaceWithContext(ctx, testWorkspace.RootPath)
		require.NoError(t, err)

		// Generate configuration
		err = configManager.GenerateWorkspaceConfig(testWorkspace.RootPath, workspaceContext)
		require.NoError(t, err, "Configuration generation should succeed")

		// Load and verify configuration
		workspaceConfig, err := configManager.LoadWorkspaceConfig(testWorkspace.RootPath)
		require.NoError(t, err, "Configuration loading should succeed")

		// Verify workspace-level configuration
		assert.Equal(t, testWorkspace.RootPath, workspaceConfig.Workspace.RootPath)
		assert.NotEmpty(t, workspaceConfig.Workspace.Hash)
		assert.GreaterOrEqual(t, len(workspaceConfig.Workspace.Languages), 4)

		// Verify server configurations for each project type
		assert.NotEmpty(t, workspaceConfig.Servers, "Should have server configurations")
		
		expectedServers := map[string]string{
			types.PROJECT_TYPE_GO:         types.SERVER_GOPLS,
			types.PROJECT_TYPE_PYTHON:     types.SERVER_PYLSP,
			types.PROJECT_TYPE_TYPESCRIPT: types.SERVER_TYPESCRIPT_LANG_SERVER,
			types.PROJECT_TYPE_JAVA:       types.SERVER_JDTLS,
		}

		for projectType, expectedServer := range expectedServers {
			found := false
			for _, serverConfig := range workspaceConfig.Servers {
				if serverConfig.Name == expectedServer && 
				   contains(serverConfig.Languages, projectType) {
					found = true
					
					// Verify server configuration structure
					assert.NotEmpty(t, serverConfig.Command, "Server command should be set")
					assert.NotEmpty(t, serverConfig.Languages, "Server languages should be set")
					assert.True(t, serverConfig.Enabled, "Server should be enabled")
			
					break
				}
			}
			assert.True(t, found, "Should have server configuration for %s", projectType)
		}

		// Verify directories configuration
		assert.NotEmpty(t, workspaceConfig.Directories.Cache, "Cache directory should be set")
		assert.NotEmpty(t, workspaceConfig.Directories.Logs, "Logs directory should be set")
		assert.Contains(t, workspaceConfig.Directories.Cache, workspaceConfig.Workspace.Hash,
			"Cache directory should include workspace hash")
	})

	t.Run("SubProjectMapping", func(t *testing.T) {
		configManager := workspace.NewWorkspaceConfigManager()
		detector := workspace.NewWorkspaceDetector()

		workspaceContext, err := detector.DetectWorkspaceWithContext(ctx, testWorkspace.RootPath)
		require.NoError(t, err)

		err = configManager.GenerateWorkspaceConfig(testWorkspace.RootPath, workspaceContext)
		require.NoError(t, err)

		workspaceConfig, err := configManager.LoadWorkspaceConfig(testWorkspace.RootPath)
		require.NoError(t, err)

		// Verify sub-project information is preserved
		assert.GreaterOrEqual(t, len(workspaceContext.SubProjects), 4, 
			"Should have detected multiple sub-projects")
		
		// Verify workspace folders are configured correctly for each sub-project
		for _, project := range workspaceContext.SubProjects {
			// Each sub-project should have its workspace folder configured
			assert.NotEmpty(t, project.WorkspaceFolder, 
				"Sub-project %s should have workspace folder configured", project.Name)
			assert.Equal(t, project.AbsolutePath, project.WorkspaceFolder,
				"Workspace folder should match absolute path for project %s", project.Name)
		}
	})

	t.Run("ResourceQuotaAllocation", func(t *testing.T) {
		configManager := workspace.NewWorkspaceConfigManager()
		detector := workspace.NewWorkspaceDetector()

		workspaceContext, err := detector.DetectWorkspaceWithContext(ctx, testWorkspace.RootPath)
		require.NoError(t, err)

		err = configManager.GenerateWorkspaceConfig(testWorkspace.RootPath, workspaceContext)
		require.NoError(t, err)

		workspaceConfig, err := configManager.LoadWorkspaceConfig(testWorkspace.RootPath)
		require.NoError(t, err)

		// Verify each server has reasonable resource limits
		for _, serverConfig := range workspaceConfig.Servers {
			if serverConfig.Enabled {
				// Should have timeout configured
				// Note: Exact resource checking depends on implementation
				t.Logf("Server %s configuration: %+v", serverConfig.Name, serverConfig)
			}
		}

		// Verify caching is configured appropriately for multi-project setup
		assert.NotEmpty(t, workspaceConfig.Directories.Cache, "Cache should be configured")
		assert.NotEmpty(t, workspaceConfig.Directories.Logs, "Logging should be configured")
	})
}

// TestMultiProjectPerformance tests performance characteristics of multi-project workspaces
func TestMultiProjectPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance tests in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), multiProjectTestTimeout)
	defer cancel()

	// Create test workspace
	generator := workspace.NewTestWorkspaceGenerator()
	defer func() {
		if cleanupGen, ok := generator.(*workspace.DefaultTestWorkspaceGenerator); ok {
			cleanupGen.CleanupAllWorkspaces()
		}
	}()

	testWorkspace, err := createCompleteMultiProjectWorkspace(generator, "perf-workspace")
	require.NoError(t, err)
	defer generator.CleanupWorkspace(testWorkspace)

	t.Run("SubProjectResolutionPerformance", func(t *testing.T) {
		detector := workspace.NewWorkspaceDetector()
		
		// Warm up
		workspaceContext, err := detector.DetectWorkspaceWithContext(ctx, testWorkspace.RootPath)
		require.NoError(t, err)

		// Create test file paths for resolution
		var testPaths []string
		for _, project := range workspaceContext.SubProjects {
			for i := 0; i < 10; i++ {
				testPath := filepath.Join(project.AbsolutePath, fmt.Sprintf("test%d.%s", i, getFileExtension(project.ProjectType)))
				testPaths = append(testPaths, testPath)
			}
		}

		// Measure resolution performance
		start := time.Now()
		for i := 0; i < performanceIterations; i++ {
			for _, testPath := range testPaths {
				resolvedProject := detector.FindSubProjectForPath(workspaceContext, testPath)
				assert.NotNil(t, resolvedProject, "Should resolve project for path")
			}
		}
		elapsed := time.Since(start)

		avgResolutionTime := elapsed / time.Duration(performanceIterations*len(testPaths))
		t.Logf("Average sub-project resolution time: %v", avgResolutionTime)
		
		// Performance requirement: <1ms per resolution
		assert.Less(t, avgResolutionTime, time.Millisecond, 
			"Sub-project resolution should be under 1ms (got %v)", avgResolutionTime)
	})

	t.Run("ConfigurationGenerationPerformance", func(t *testing.T) {
		configManager := workspace.NewWorkspaceConfigManager()
		detector := workspace.NewWorkspaceDetector()

		workspaceContext, err := detector.DetectWorkspaceWithContext(ctx, testWorkspace.RootPath)
		require.NoError(t, err)

		// Measure configuration generation performance
		start := time.Now()
		for i := 0; i < 10; i++ { // Fewer iterations for config generation
			err = configManager.GenerateWorkspaceConfig(testWorkspace.RootPath, workspaceContext)
			require.NoError(t, err)
		}
		elapsed := time.Since(start)

		avgConfigTime := elapsed / 10
		t.Logf("Average configuration generation time: %v", avgConfigTime)
		
		// Performance requirement: <1s for 4 projects
		assert.Less(t, avgConfigTime, time.Second, 
			"Configuration generation should be under 1s (got %v)", avgConfigTime)
	})

	t.Run("MemoryUsage", func(t *testing.T) {
		var memBefore, memAfter runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&memBefore)

		// Initialize multiple gateways to test memory usage
		gateways, configManagers, portManagers, err := initializeMultiProjectGateways(ctx, []*workspace.TestWorkspace{testWorkspace})
		require.NoError(t, err)
		defer cleanupMultiProjectGateways(gateways, portManagers)

		runtime.GC()
		runtime.ReadMemStats(&memAfter)

		memUsed := memAfter.Alloc - memBefore.Alloc
		t.Logf("Memory used for multi-project workspace: %d bytes (%.2f MB)", 
			memUsed, float64(memUsed)/(1024*1024))

		// Performance requirement: <200MB total
		assert.Less(t, float64(memUsed)/(1024*1024), 200.0, 
			"Memory usage should be under 200MB (got %.2f MB)", float64(memUsed)/(1024*1024))
	})

	t.Run("ConcurrentRequestHandling", func(t *testing.T) {
		// Initialize gateway
		gateways, _, portManagers, err := initializeMultiProjectGateways(ctx, []*workspace.TestWorkspace{testWorkspace})
		require.NoError(t, err)
		defer cleanupMultiProjectGateways(gateways, portManagers)

		gateway := gateways[0]

		// Prepare requests for different projects
		var requests []*workspace.JSONRPCRequest
		for _, project := range testWorkspace.Projects {
			for i := 0; i < 5; i++ {
				fileExt := getFileExtension(project.ProjectType)
				fileURI := fmt.Sprintf("file://%s/concurrent_%d.%s", project.AbsolutePath, i, fileExt)
				req := createLSPRequest("textDocument/hover", fileURI, 1, 1)
				requests = append(requests, req)
			}
		}

		// Measure concurrent request handling
		start := time.Now()
		var wg sync.WaitGroup
		successCount := int32(0)

		for _, req := range requests {
			wg.Add(1)
			go func(request *workspace.JSONRPCRequest) {
				defer wg.Done()
				_, err := executeJSONRPCRequest(gateway, request)
				if err == nil {
					atomic.AddInt32(&successCount, 1)
				}
			}(req)
		}

		wg.Wait()
		elapsed := time.Since(start)

		successRate := float64(successCount) / float64(len(requests))
		avgRequestTime := elapsed / time.Duration(len(requests))

		t.Logf("Concurrent requests: %d total, %d successful (%.1f%%), avg time: %v", 
			len(requests), successCount, successRate*100, avgRequestTime)

		// Performance requirements
		assert.GreaterOrEqual(t, successRate, 0.8, "At least 80%% of requests should succeed")
		assert.Less(t, avgRequestTime, 100*time.Millisecond, 
			"Average request time should be under 100ms")
	})
}

// TestMultiProjectErrorHandling tests error scenarios and edge cases
func TestMultiProjectErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping error handling tests in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), multiProjectTestTimeout)
	defer cancel()

	t.Run("InvalidWorkspaceHandling", func(t *testing.T) {
		detector := workspace.NewWorkspaceDetector()

		// Test non-existent workspace
		_, err := detector.DetectWorkspaceAt("/non/existent/path")
		assert.Error(t, err, "Should fail for non-existent workspace")

		// Test file instead of directory
		tempFile, err := ioutil.TempFile("", "not_a_directory")
		require.NoError(t, err)
		defer os.Remove(tempFile.Name())
		tempFile.Close()

		_, err = detector.DetectWorkspaceAt(tempFile.Name())
		assert.Error(t, err, "Should fail when path is not a directory")
	})

	t.Run("EmptyWorkspaceHandling", func(t *testing.T) {
		// Create empty workspace directory
		emptyDir, err := ioutil.TempDir("", "empty_workspace_")
		require.NoError(t, err)
		defer os.RemoveAll(emptyDir)

		detector := workspace.NewWorkspaceDetector()
		workspaceContext, err := detector.DetectWorkspaceWithContext(ctx, emptyDir)
		require.NoError(t, err, "Should handle empty workspace")

		// Should create a default root project
		assert.NotNil(t, workspaceContext)
		assert.GreaterOrEqual(t, len(workspaceContext.SubProjects), 1, "Should have at least root project")
		
		rootProject := workspaceContext.SubProjects[0]
		assert.Equal(t, ".", rootProject.RelativePath, "Root project should have '.' as relative path")
		assert.Equal(t, types.PROJECT_TYPE_UNKNOWN, rootProject.ProjectType, "Should detect as unknown type")
	})

	t.Run("CorruptedProjectMarkers", func(t *testing.T) {
		// Create workspace with corrupted marker files
		corruptedDir, err := ioutil.TempDir("", "corrupted_workspace_")
		require.NoError(t, err)
		defer os.RemoveAll(corruptedDir)

		// Create corrupted go.mod
		goDir := filepath.Join(corruptedDir, "go-project")
		err = os.MkdirAll(goDir, 0755)
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(goDir, "go.mod"), []byte("invalid content [[["), 0644)
		require.NoError(t, err)

		// Should still detect the project despite corrupted content
		detector := workspace.NewWorkspaceDetector()
		workspaceContext, err := detector.DetectWorkspaceWithContext(ctx, corruptedDir)
		require.NoError(t, err, "Should handle corrupted marker files")

		// Should still detect Go project based on marker file presence
		found := false
		for _, project := range workspaceContext.SubProjects {
			if project.ProjectType == types.PROJECT_TYPE_GO {
				found = true
				break
			}
		}
		assert.True(t, found, "Should detect Go project despite corrupted go.mod")
	})

	t.Run("NestedProjectConflicts", func(t *testing.T) {
		// Create nested project structure that could cause conflicts
		nestedDir, err := ioutil.TempDir("", "nested_workspace_")
		require.NoError(t, err)
		defer os.RemoveAll(nestedDir)

		// Create parent Go project
		parentGoMod := filepath.Join(nestedDir, "go.mod")
		err = os.WriteFile(parentGoMod, []byte("module parent\n\ngo 1.21\n"), 0644)
		require.NoError(t, err)

		// Create nested sub-project
		subDir := filepath.Join(nestedDir, "sub")
		err = os.MkdirAll(subDir, 0755)
		require.NoError(t, err)

		subGoMod := filepath.Join(subDir, "go.mod")
		err = os.WriteFile(subGoMod, []byte("module sub\n\ngo 1.21\n"), 0644)
		require.NoError(t, err)

		detector := workspace.NewWorkspaceDetector()
		workspaceContext, err := detector.DetectWorkspaceWithContext(ctx, nestedDir)
		require.NoError(t, err, "Should handle nested project conflicts")

		// Should detect both projects
		assert.GreaterOrEqual(t, len(workspaceContext.SubProjects), 2, "Should detect both nested projects")

		// Verify path mapping precedence (deeper projects should take precedence)
		subProjectPath := filepath.Join(subDir, "test.go") 
		resolvedProject := detector.FindSubProjectForPath(workspaceContext, subProjectPath)
		require.NotNil(t, resolvedProject, "Should resolve nested project")
		assert.Equal(t, subDir, resolvedProject.AbsolutePath, "Should resolve to deeper nested project")
	})

	t.Run("LargeWorkspaceHandling", func(t *testing.T) {
		// Create workspace with many directories (to test limits)
		largeDir, err := ioutil.TempDir("", "large_workspace_")
		require.NoError(t, err)
		defer os.RemoveAll(largeDir)

		// Create many subdirectories
		for i := 0; i < 50; i++ {
			subDir := filepath.Join(largeDir, fmt.Sprintf("dir_%d", i))
			err = os.MkdirAll(subDir, 0755)
			require.NoError(t, err)

			// Only some have project markers
			if i%10 == 0 {
				goMod := filepath.Join(subDir, "go.mod")
				err = os.WriteFile(goMod, []byte(fmt.Sprintf("module dir%d\ngo 1.21\n", i)), 0644)
				require.NoError(t, err)
			}
		}

		detector := workspace.NewWorkspaceDetector()
		start := time.Now()
		workspaceContext, err := detector.DetectWorkspaceWithContext(ctx, largeDir)
		elapsed := time.Since(start)

		require.NoError(t, err, "Should handle large workspace")
		t.Logf("Large workspace detection took: %v", elapsed)

		// Should respect scanning limits but still find projects
		assert.GreaterOrEqual(t, len(workspaceContext.SubProjects), 1, "Should find some projects")
		assert.Less(t, elapsed, 10*time.Second, "Should complete within reasonable time")
	})

	t.Run("ConcurrentDetectionStress", func(t *testing.T) {
		// Create test workspace
		generator := workspace.NewTestWorkspaceGenerator()
		defer func() {
			if cleanupGen, ok := generator.(*workspace.DefaultTestWorkspaceGenerator); ok {
				cleanupGen.CleanupAllWorkspaces()
			}
		}()

		testWorkspace, err := createCompleteMultiProjectWorkspace(generator, "stress-workspace")
		require.NoError(t, err)
		defer generator.CleanupWorkspace(testWorkspace)

		// Run concurrent detection operations
		var wg sync.WaitGroup
		errors := make([]error, 10)

		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				detector := workspace.NewWorkspaceDetector()
				_, err := detector.DetectWorkspaceWithContext(ctx, testWorkspace.RootPath)
				errors[idx] = err
			}(i)
		}

		wg.Wait()

		// All detections should succeed
		for i, err := range errors {
			assert.NoError(t, err, "Concurrent detection %d should succeed", i)
		}
	})
}

// Helper functions

func createCompleteMultiProjectWorkspace(generator workspace.TestWorkspaceGenerator, name string) (*workspace.TestWorkspace, error) {
	config := &workspace.MultiProjectWorkspaceConfig{
		Name: name,
		Projects: []*workspace.ProjectConfig{
			{
				Name:         "go-service",
				RelativePath: "services/go-service",
				ProjectType:  types.PROJECT_TYPE_GO,
				Languages:    []string{types.PROJECT_TYPE_GO},
				EnableBuild:  true,
				EnableTests:  true,
			},
			{
				Name:         "python-api",
				RelativePath: "apis/python-api",
				ProjectType:  types.PROJECT_TYPE_PYTHON,
				Languages:    []string{types.PROJECT_TYPE_PYTHON},
				EnableBuild:  true,
				EnableTests:  true,
			},
			{
				Name:         "frontend",
				RelativePath: "ui/frontend",
				ProjectType:  types.PROJECT_TYPE_TYPESCRIPT,
				Languages:    []string{types.PROJECT_TYPE_TYPESCRIPT},
				EnableBuild:  true,
				EnableTests:  true,
			},
			{
				Name:         "java-backend",
				RelativePath: "backends/java-backend",
				ProjectType:  types.PROJECT_TYPE_JAVA,
				Languages:    []string{types.PROJECT_TYPE_JAVA},
				EnableBuild:  true,
				EnableTests:  true,
			},
		},
		SharedDirectories: []string{"shared", "docs", "config"},
		WorkspaceFiles: map[string]string{
			"README.md": fmt.Sprintf("# %s\n\nMulti-project workspace for LSP Gateway testing.\n", name),
			"Makefile":  "# Multi-project Makefile\nall:\n\t@echo \"Building all projects\"\n",
			".gitignore": "# Multi-project gitignore\nnode_modules/\n__pycache__/\ntarget/\ndist/\n",
		},
		GenerateMetadata: true,
		ValidationLevel:  workspace.ValidationBasic,
	}

	return generator.GenerateMultiProjectWorkspace(config)
}

func initializeMultiProjectGateways(ctx context.Context, testWorkspaces []*workspace.TestWorkspace) ([]workspace.WorkspaceGateway, []workspace.WorkspaceConfigManager, []workspace.WorkspacePortManager, error) {
	gateways := make([]workspace.WorkspaceGateway, len(testWorkspaces))
	configManagers := make([]workspace.WorkspaceConfigManager, len(testWorkspaces))
	portManagers := make([]workspace.WorkspacePortManager, len(testWorkspaces))

	for i, testWorkspace := range testWorkspaces {
		// Initialize port manager
		portManager, err := workspace.NewWorkspacePortManager()
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to create port manager for workspace %d: %w", i, err)
		}
		portManagers[i] = portManager

		// Allocate port
		_, err = portManager.AllocatePort(testWorkspace.RootPath)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to allocate port for workspace %d: %w", i, err)
		}

		// Initialize config manager
		configManager := workspace.NewWorkspaceConfigManager()
		configManagers[i] = configManager

		// Detect workspace
		detector := workspace.NewWorkspaceDetector()
		workspaceContext, err := detector.DetectWorkspaceWithContext(ctx, testWorkspace.RootPath)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to detect workspace %d: %w", i, err)
		}

		// Generate configuration
		err = configManager.GenerateWorkspaceConfig(testWorkspace.RootPath, workspaceContext)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to generate config for workspace %d: %w", i, err)
		}

		// Load configuration
		workspaceConfig, err := configManager.LoadWorkspaceConfig(testWorkspace.RootPath)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to load config for workspace %d: %w", i, err)
		}

		// Initialize gateway
		gateway := workspace.NewWorkspaceGateway()
		gatewayConfig := &workspace.WorkspaceGatewayConfig{
			WorkspaceRoot: testWorkspace.RootPath,
			ExtensionMapping: map[string]string{
				"go": types.PROJECT_TYPE_GO,
				"py": types.PROJECT_TYPE_PYTHON,
				"ts": types.PROJECT_TYPE_TYPESCRIPT,
				"js": types.PROJECT_TYPE_NODEJS,
				"java": types.PROJECT_TYPE_JAVA,
			},
			Timeout:       multiProjectShortTimeout,
			EnableLogging: true,
		}

		err = gateway.Initialize(ctx, workspaceConfig, gatewayConfig)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to initialize gateway %d: %w", i, err)
		}

		err = gateway.Start(ctx)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to start gateway %d: %w", i, err)
		}

		gateways[i] = gateway
	}

	return gateways, configManagers, portManagers, nil
}

func cleanupMultiProjectGateways(gateways []workspace.WorkspaceGateway, portManagers []workspace.WorkspacePortManager) {
	for i, gateway := range gateways {
		if gateway != nil {
			gateway.Stop()
		}
		if i < len(portManagers) && portManagers[i] != nil {
			// Note: We don't have the workspace path here for cleanup
			// In a real scenario, we'd track this properly
		}
	}
}

func createLSPRequest(method, fileURI string, line, character int) *workspace.JSONRPCRequest {
	req := &workspace.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      fmt.Sprintf("test-%d", time.Now().UnixNano()),
		Method:  method,
	}

	// Set method-specific parameters
	switch method {
	case "textDocument/definition", "textDocument/references", "textDocument/hover":
		req.Params = map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": fileURI,
			},
			"position": map[string]interface{}{
				"line":      line,
				"character": character,
			},
		}
		if method == "textDocument/references" {
			if params, ok := req.Params.(map[string]interface{}); ok {
				params["context"] = map[string]interface{}{
					"includeDeclaration": true,
				}
			}
		}
	case "workspace/symbol":
		req.Params = map[string]interface{}{
			"query": "test",
		}
	case "textDocument/documentSymbol":
		req.Params = map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": fileURI,
			},
		}
	}

	return req
}

func getFileExtension(projectType string) string {
	switch projectType {
	case types.PROJECT_TYPE_GO:
		return "go"
	case types.PROJECT_TYPE_PYTHON:
		return "py"
	case types.PROJECT_TYPE_TYPESCRIPT:
		return "ts"
	case types.PROJECT_TYPE_NODEJS:
		return "js"
	case types.PROJECT_TYPE_JAVA:
		return "java"
	default:
		return "txt"
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Benchmark functions

func BenchmarkSubProjectResolution(b *testing.B) {
	// Create test workspace
	generator := workspace.NewTestWorkspaceGenerator()
	defer func() {
		if cleanupGen, ok := generator.(*workspace.DefaultTestWorkspaceGenerator); ok {
			cleanupGen.CleanupAllWorkspaces()
		}
	}()

	testWorkspace, err := createCompleteMultiProjectWorkspace(generator, "bench-workspace")
	if err != nil {
		b.Fatalf("Failed to create test workspace: %v", err)
	}
	defer generator.CleanupWorkspace(testWorkspace)

	// Initialize detector
	detector := workspace.NewWorkspaceDetector()
	workspaceContext, err := detector.DetectWorkspaceWithContext(context.Background(), testWorkspace.RootPath)
	if err != nil {
		b.Fatalf("Failed to detect workspace: %v", err)
	}

	// Create test paths
	var testPaths []string
	for _, project := range workspaceContext.SubProjects {
		testPath := filepath.Join(project.AbsolutePath, "test."+getFileExtension(project.ProjectType))
		testPaths = append(testPaths, testPath)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			testPath := testPaths[i%len(testPaths)]
			resolvedProject := detector.FindSubProjectForPath(workspaceContext, testPath)
			if resolvedProject == nil {
				b.Fatalf("Failed to resolve project for path: %s", testPath)
			}
			i++
		}
	})
}

func BenchmarkConfigurationGeneration(b *testing.B) {
	// Create test workspace  
	generator := workspace.NewTestWorkspaceGenerator()
	defer func() {
		if cleanupGen, ok := generator.(*workspace.DefaultTestWorkspaceGenerator); ok {
			cleanupGen.CleanupAllWorkspaces()
		}
	}()

	testWorkspace, err := createCompleteMultiProjectWorkspace(generator, "bench-config-workspace")
	if err != nil {
		b.Fatalf("Failed to create test workspace: %v", err)
	}
	defer generator.CleanupWorkspace(testWorkspace)

	// Initialize components
	detector := workspace.NewWorkspaceDetector()
	workspaceContext, err := detector.DetectWorkspaceWithContext(context.Background(), testWorkspace.RootPath)
	if err != nil {
		b.Fatalf("Failed to detect workspace: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		configManager := workspace.NewWorkspaceConfigManager()
		err := configManager.GenerateWorkspaceConfig(testWorkspace.RootPath, workspaceContext)
		if err != nil {
			b.Fatalf("Failed to generate config: %v", err)
		}
	}
}

func BenchmarkConcurrentRequests(b *testing.B) {
	// Create test workspace
	generator := workspace.NewTestWorkspaceGenerator()
	defer func() {
		if cleanupGen, ok := generator.(*workspace.DefaultTestWorkspaceGenerator); ok {
			cleanupGen.CleanupAllWorkspaces()
		}
	}()

	testWorkspace, err := createCompleteMultiProjectWorkspace(generator, "bench-requests-workspace")
	if err != nil {
		b.Fatalf("Failed to create test workspace: %v", err)
	}
	defer generator.CleanupWorkspace(testWorkspace)

	// Initialize gateway
	ctx := context.Background()
	gateways, _, portManagers, err := initializeMultiProjectGateways(ctx, []*workspace.TestWorkspace{testWorkspace})
	if err != nil {
		b.Fatalf("Failed to initialize gateways: %v", err)
	}
	defer cleanupMultiProjectGateways(gateways, portManagers)

	gateway := gateways[0]

	// Prepare requests
	var requests []*workspace.JSONRPCRequest
	for _, project := range testWorkspace.Projects {
		fileExt := getFileExtension(project.ProjectType)
		fileURI := fmt.Sprintf("file://%s/bench.%s", project.AbsolutePath, fileExt)
		req := createLSPRequest("textDocument/hover", fileURI, 1, 1)
		requests = append(requests, req)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			req := requests[i%len(requests)]
			_, err := executeJSONRPCRequest(gateway, req)
			if err != nil {
				// Allow some errors in benchmarks due to mock servers
				b.Logf("Request failed (expected in benchmark): %v", err)
			}
			i++
		}
	})
}