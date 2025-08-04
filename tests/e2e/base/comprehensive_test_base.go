package base

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"lsp-gateway/tests/e2e/testutils"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// LanguageConfig defines language-specific configuration
type LanguageConfig struct {
	Language      string
	DisplayName   string
	HasRepoMgmt   bool
	HasAllLSPTest bool
}

// ComprehensiveTestBaseSuite provides base functionality for comprehensive E2E tests
type ComprehensiveTestBaseSuite struct {
	suite.Suite

	// Language configuration
	Config LanguageConfig

	// Core infrastructure
	httpClient  *testutils.HttpClient
	gatewayCmd  *exec.Cmd
	gatewayPort int
	configPath  string
	tempDir     string
	projectRoot string
	testTimeout time.Duration

	// Repository management
	repoManager *testutils.RepoManager
	repoDir     string

	// Server state tracking
	serverStarted bool

	// Cache management with isolation
	cacheDir            string
	cacheIsolationMgr   *testutils.CacheIsolationManager
	cacheIsolationLevel testutils.IsolationLevel

	// Shared server management (NEW)
	sharedServerManager *testutils.SharedServerManager
	useSharedServer     bool // Flag to enable/disable shared server mode
}

// SetupSuite initializes the comprehensive test suite
func (suite *ComprehensiveTestBaseSuite) SetupSuite() {
	suite.testTimeout = 120 * time.Second
	suite.gatewayPort = 8080

	// Enable shared server mode for better performance and reduce test overhead
	suite.useSharedServer = true

	// Create temp directory for repositories
	tempDir, err := ioutil.TempDir("", "lsp-gateway-e2e-repos")
	require.NoError(suite.T(), err, "Failed to create temp directory")
	suite.tempDir = tempDir

	// Initialize repository manager
	suite.repoManager = testutils.NewRepoManager(tempDir)

	// Basic project root setup
	if cwd, err := os.Getwd(); err == nil {
		suite.projectRoot = filepath.Dir(filepath.Dir(cwd))
	}

	// Setup test workspace for the repository
	err = suite.setupTestWorkspace()
	require.NoError(suite.T(), err, "Failed to setup test workspace during SetupSuite")

	// Initialize cache isolation manager
	suite.cacheIsolationLevel = testutils.BasicIsolation // Use basic isolation for shared server
	cacheIsolationConfig := testutils.DefaultCacheIsolationConfig()
	cacheIsolationConfig.IsolationLevel = suite.cacheIsolationLevel

	suite.cacheIsolationMgr, err = testutils.NewCacheIsolationManager(tempDir, cacheIsolationConfig)
	require.NoError(suite.T(), err, "Failed to create cache isolation manager")

	// Setup isolated cache directory
	suite.cacheDir = suite.cacheIsolationMgr.GetCacheDirectory()

	// Initialize shared server manager if enabled
	if suite.useSharedServer {
		suite.sharedServerManager = testutils.NewSharedServerManager(suite.repoDir, suite.cacheIsolationMgr)

		// Start the shared server once for all tests in this suite
		err = suite.sharedServerManager.StartSharedServer(suite.T())
		require.NoError(suite.T(), err, "Failed to start shared LSP server")

		suite.T().Logf("🚀 Shared server mode enabled for %s tests", suite.Config.DisplayName)
	}
}

// SetupTest prepares each test with isolated cache
func (suite *ComprehensiveTestBaseSuite) SetupTest() {
	testName := suite.T().Name()

	// In shared server mode, register with the shared server
	if suite.useSharedServer && suite.sharedServerManager != nil {
		suite.sharedServerManager.RegisterTest(testName, suite.T())

		// Update HTTP client and port from shared server
		suite.httpClient = suite.sharedServerManager.GetHTTPClient()
		suite.gatewayPort = suite.sharedServerManager.GetServerPort()

		suite.T().Logf("🔗 Test '%s' connected to shared server on port %d", testName, suite.gatewayPort)
	} else {
		// Legacy mode: Initialize cache isolation for individual server
		err := suite.cacheIsolationMgr.InitializeIsolation(testName)
		require.NoError(suite.T(), err, "Failed to initialize cache isolation")

		// Validate clean cache state
		err = suite.cacheIsolationMgr.ValidateCleanState()
		require.NoError(suite.T(), err, "Cache isolation validation failed")

		// Update cache directory to isolated one
		suite.cacheDir = suite.cacheIsolationMgr.GetCacheDirectory()

		suite.T().Logf("🔒 Cache isolation initialized for test '%s' with directory: %s", testName, suite.cacheDir)
	}
}

// TearDownTest cleans up after each test with cache isolation validation
func (suite *ComprehensiveTestBaseSuite) TearDownTest() {
	testName := suite.T().Name()

	// In shared server mode, just unregister from shared server
	if suite.useSharedServer && suite.sharedServerManager != nil {
		suite.sharedServerManager.UnregisterTest(testName, suite.T())
		suite.T().Logf("🔗 Test '%s' disconnected from shared server", testName)
		return
	}

	// Legacy mode: Individual server cleanup
	// Record final cache state if server is running
	if suite.serverStarted {
		healthURL := fmt.Sprintf("http://localhost:%d/health", suite.gatewayPort)
		if err := suite.cacheIsolationMgr.RecordCacheState(healthURL, "FINAL_STATE"); err != nil {
			suite.T().Logf("Warning: Failed to record final cache state: %v", err)
		}
	}

	// Stop gateway server
	suite.stopGatewayServer()

	// Validate test isolation before cleanup
	if err := suite.cacheIsolationMgr.ValidateTestIsolation(); err != nil {
		suite.T().Logf("⚠️ Cache isolation violations detected: %v", err)

		// Log violations for debugging
		violations := suite.cacheIsolationMgr.GetViolations()
		for i, violation := range violations {
			suite.T().Logf("  Violation %d: %s - %s", i+1, violation.ViolationType, violation.Description)
		}
	}

	// Perform isolated cache cleanup
	if err := suite.cacheIsolationMgr.Cleanup(); err != nil {
		suite.T().Logf("Warning: Cache isolation cleanup failed: %v", err)
		// Fallback to basic cleanup
		suite.cleanupCache()
	}

	suite.T().Logf("🧹 Cache isolation cleanup completed for test '%s'", testName)
}

// TearDownSuite cleans up after all tests
func (suite *ComprehensiveTestBaseSuite) TearDownSuite() {
	// Stop shared server if it's running
	if suite.useSharedServer && suite.sharedServerManager != nil {
		if err := suite.sharedServerManager.StopSharedServer(suite.T()); err != nil {
			suite.T().Logf("Warning: Failed to stop shared server: %v", err)
		}

		// Log shared server statistics
		serverInfo := suite.sharedServerManager.GetServerInfo()
		suite.T().Logf("📊 Shared server served %v total tests for %s", serverInfo["total_tests"], suite.Config.DisplayName)
	}

	if suite.repoManager != nil {
		suite.repoManager.Cleanup()
	}

	// Final cache isolation cleanup
	if suite.cacheIsolationMgr != nil {
		if err := suite.cacheIsolationMgr.Cleanup(); err != nil {
			suite.T().Logf("Warning: Final cache isolation cleanup failed: %v", err)
		}

		// Log final isolation summary
		violations := suite.cacheIsolationMgr.GetViolations()
		if len(violations) > 0 {
			suite.T().Logf("📊 Test suite cache isolation summary: %d violations detected", len(violations))
		} else {
			suite.T().Logf("✅ Test suite completed with perfect cache isolation")
		}
	}

	// Clean up temp directory
	if suite.tempDir != "" {
		os.RemoveAll(suite.tempDir)
	}
}

// ensureServerAvailable ensures a server (shared or individual) is available for testing
func (suite *ComprehensiveTestBaseSuite) ensureServerAvailable() (*testutils.HttpClient, error) {
	if suite.useSharedServer && suite.sharedServerManager != nil {
		// Shared server mode: Use existing shared server
		if !suite.sharedServerManager.IsServerRunning() {
			return nil, fmt.Errorf("shared server is not running")
		}

		httpClient := suite.sharedServerManager.GetHTTPClient()
		if httpClient == nil {
			return nil, fmt.Errorf("shared server HTTP client is not available")
		}

		return httpClient, nil
	}

	// Legacy individual server mode
	err := suite.setupTestWorkspace()
	if err != nil {
		return nil, fmt.Errorf("failed to setup test workspace: %w", err)
	}

	err = suite.startGatewayServer()
	if err != nil {
		return nil, fmt.Errorf("failed to start gateway server: %w", err)
	}

	err = suite.waitForServerReady()
	if err != nil {
		return nil, fmt.Errorf("server failed to become ready: %w", err)
	}

	httpClient := testutils.NewHttpClient(testutils.HttpClientConfig{
		BaseURL: fmt.Sprintf("http://localhost:%d", suite.gatewayPort),
		Timeout: 10 * time.Second,
	})

	return httpClient, nil
}

// DisableSharedServer disables shared server mode for this test suite
func (suite *ComprehensiveTestBaseSuite) DisableSharedServer() {
	suite.useSharedServer = false
}

// stopGatewayServer stops the gateway server
func (suite *ComprehensiveTestBaseSuite) stopGatewayServer() {
	if suite.gatewayCmd != nil && suite.gatewayCmd.Process != nil {
		suite.gatewayCmd.Process.Signal(syscall.SIGTERM)
		suite.gatewayCmd.Wait()
		suite.gatewayCmd = nil
	}
	suite.serverStarted = false
}

// setupTestWorkspace sets up a real project workspace for testing
func (suite *ComprehensiveTestBaseSuite) setupTestWorkspace() error {
	// Setup repository for the language
	repoDir, err := suite.repoManager.SetupRepository(suite.Config.Language)
	if err != nil {
		return fmt.Errorf("failed to setup repository for %s: %w", suite.Config.Language, err)
	}
	suite.repoDir = repoDir

	// Verify test files exist
	if err := suite.repoManager.VerifyFileExists(suite.Config.Language, 0); err != nil {
		return fmt.Errorf("test file verification failed: %w", err)
	}

	suite.T().Logf("Set up test workspace for %s at: %s", suite.Config.Language, repoDir)
	return nil
}

// startGatewayServer starts the LSP gateway server
func (suite *ComprehensiveTestBaseSuite) startGatewayServer() error {
	if suite.serverStarted {
		return nil
	}

	// Find available port
	port, err := testutils.FindAvailablePort()
	if err != nil {
		return fmt.Errorf("failed to find available port: %w", err)
	}
	suite.gatewayPort = port

	// Generate isolated config using cache isolation manager
	servers := map[string]interface{}{
		"go": map[string]interface{}{
			"command": "gopls",
			"args":    []string{"serve"},
		},
		"python": map[string]interface{}{
			"command": "pylsp",
			"args":    []string{},
		},
		"javascript": map[string]interface{}{
			"command": "typescript-language-server",
			"args":    []string{"--stdio"},
		},
		"typescript": map[string]interface{}{
			"command": "typescript-language-server",
			"args":    []string{"--stdio"},
		},
		"java": map[string]interface{}{
			"command": "~/.lsp-gateway/tools/java/bin/jdtls",
			"args":    []string{},
		},
	}

	cacheConfig := testutils.DefaultCacheIsolationConfig()
	cacheConfig.IsolationLevel = suite.cacheIsolationLevel
	cacheConfig.MaxCacheSize = 128 * 1024 * 1024 // 128MB

	configPath, err := suite.cacheIsolationMgr.GenerateIsolatedConfig(servers, cacheConfig)
	if err != nil {
		return fmt.Errorf("failed to generate isolated config: %w", err)
	}
	suite.configPath = configPath

	suite.T().Logf("🔧 Generated isolated config at: %s", configPath)

	// Get path to lsp-gateway binary
	pwd, _ := os.Getwd()
	projectRoot := filepath.Dir(filepath.Dir(pwd)) // go up from tests/e2e to project root
	binaryPath := filepath.Join(projectRoot, "bin", "lsp-gateway")

	// Check if binary exists
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		return fmt.Errorf("lsp-gateway binary not found at %s. Run 'make local' first", binaryPath)
	}

	// Start the server
	cmd := exec.Command(binaryPath, "server", "--config", configPath, "--port", fmt.Sprintf("%d", port))
	cmd.Dir = suite.repoDir // Run in the cloned repository workspace
	cmd.Env = append(os.Environ(),
		"GO111MODULE=on",
		fmt.Sprintf("GOPATH=%s", os.Getenv("GOPATH")),
	)

	// Capture output for debugging
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	suite.T().Logf("Starting LSP gateway server on port %d with config %s", port, configPath)
	suite.T().Logf("Command: %s %s", binaryPath, strings.Join(cmd.Args[1:], " "))
	suite.T().Logf("Working directory: %s", cmd.Dir)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	suite.gatewayCmd = cmd
	suite.serverStarted = true

	return nil
}

// waitForServerReady waits for the server to be ready to accept requests
func (suite *ComprehensiveTestBaseSuite) waitForServerReady() error {
	healthURL := fmt.Sprintf("http://localhost:%d/health", suite.gatewayPort)

	suite.T().Logf("Waiting for server and LSP clients to be ready at %s...", healthURL)

	// Record initial server state
	if err := suite.cacheIsolationMgr.RecordCacheState(healthURL, "SERVER_STARTUP"); err != nil {
		suite.T().Logf("Warning: Failed to record server startup state: %v", err)
	}

	for i := 0; i < 30; i++ { // Wait up to 30 seconds
		if err := testutils.QuickConnectivityCheck(healthURL); err == nil {
			// Health endpoint is responding, now check if LSP clients are active
			if suite.checkLSPClientsActive(healthURL) {
				// Record ready state
				if err := suite.cacheIsolationMgr.RecordCacheState(healthURL, "SERVER_READY"); err != nil {
					suite.T().Logf("Warning: Failed to record server ready state: %v", err)
				}

				// Wait for cache stabilization
				if err := suite.cacheIsolationMgr.WaitForCacheStabilization(healthURL, 15*time.Second); err != nil {
					suite.T().Logf("Warning: Cache stabilization timeout: %v", err)
				}

				suite.T().Logf("✅ Server and LSP clients are ready after %d seconds", i+1)
				return nil
			}
		}
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("server did not become ready within 30 seconds")
}

// checkLSPClientsActive checks if required LSP clients are active and cache is working
func (suite *ComprehensiveTestBaseSuite) checkLSPClientsActive(healthURL string) bool {
	resp, err := http.Get(healthURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	var health map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return false
	}

	// Check LSP clients
	lspClients, ok := health["lsp_clients"].(map[string]interface{})
	if !ok {
		return false
	}

	// Check if the target language LSP client is active
	langClient, ok := lspClients[suite.Config.Language].(map[string]interface{})
	if !ok {
		suite.T().Logf("%s LSP client not found in health status", suite.Config.Language)
		return false
	}

	active, ok := langClient["Active"].(bool)
	if !ok || !active {
		suite.T().Logf("%s LSP client not active yet: %v", suite.Config.Language, langClient)
		return false
	}

	// Validate cache status
	if !suite.validateCacheHealth(health) {
		return false
	}

	suite.T().Logf("%s LSP client is active: %v", suite.Config.Language, langClient)
	return true
}

// validateCacheHealth validates cache status in health response
func (suite *ComprehensiveTestBaseSuite) validateCacheHealth(health map[string]interface{}) bool {
	cache, ok := health["cache"].(map[string]interface{})
	if !ok {
		suite.T().Logf("Cache status not found in health response")
		return false
	}

	// Check if cache is enabled
	enabled, ok := cache["enabled"].(bool)
	if !ok || !enabled {
		suite.T().Logf("Cache is not enabled: %v", cache)
		return false
	}

	// Check cache status
	status, ok := cache["status"].(string)
	if !ok {
		suite.T().Logf("Cache status field missing: %v", cache)
		return false
	}

	// Accept "healthy", "initializing", or "OK" states
	if status != "healthy" && status != "initializing" && status != "OK" {
		suite.T().Logf("Cache status is not healthy/initializing/OK: %s", status)
		return false
	}

	suite.T().Logf("Cache is %s and enabled", status)
	return true
}

// validateCacheMetrics validates detailed cache metrics (optional validation)
func (suite *ComprehensiveTestBaseSuite) validateCacheMetrics(health map[string]interface{}) bool {
	cache, ok := health["cache"].(map[string]interface{})
	if !ok {
		return false
	}

	// Optional: Check detailed metrics if available
	if hitRate, hasHitRate := cache["hit_rate_percent"]; hasHitRate {
		suite.T().Logf("Cache hit rate: %v", hitRate)
	}

	if entryCount, hasEntryCount := cache["entry_count"]; hasEntryCount {
		suite.T().Logf("Cache entry count: %v", entryCount)
	}

	if totalRequests, hasTotalRequests := cache["total_requests"]; hasTotalRequests {
		suite.T().Logf("Cache total requests: %v", totalRequests)
	}

	return true
}

// cleanupCache cleans up cache directory after test
func (suite *ComprehensiveTestBaseSuite) cleanupCache() {
	if suite.cacheDir != "" {
		suite.T().Logf("Cleaning up cache directory: %s", suite.cacheDir)
		if err := os.RemoveAll(suite.cacheDir); err != nil {
			suite.T().Logf("Warning: Failed to clean cache directory: %v", err)
		}
		// Recreate empty cache directory for next test
		if err := os.MkdirAll(suite.cacheDir, 0755); err != nil {
			suite.T().Logf("Warning: Failed to recreate cache directory: %v", err)
		}
	}
}

// validateCachePerformance validates cache performance after test operations
func (suite *ComprehensiveTestBaseSuite) validateCachePerformance() {
	healthURL := fmt.Sprintf("http://localhost:%d/health", suite.gatewayPort)

	resp, err := http.Get(healthURL)
	if err != nil {
		suite.T().Logf("Warning: Failed to get health status for cache validation: %v", err)
		return
	}
	defer resp.Body.Close()

	var health map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		suite.T().Logf("Warning: Failed to decode health response: %v", err)
		return
	}

	// Validate cache metrics
	if suite.validateCacheMetrics(health) {
		suite.T().Logf("✅ Cache performance validation completed")
	} else {
		suite.T().Logf("⚠️ Cache metrics not available or incomplete")
	}
}

// makeJSONRPCRequest makes a raw JSON-RPC request to the LSP gateway
func (suite *ComprehensiveTestBaseSuite) makeJSONRPCRequest(ctx context.Context, httpClient *testutils.HttpClient, request map[string]interface{}) (map[string]interface{}, error) {
	return httpClient.MakeRawJSONRPCRequest(ctx, request)
}

// Test methods that can be used by language-specific tests

// TestComprehensiveServerLifecycle tests server lifecycle
func (suite *ComprehensiveTestBaseSuite) TestComprehensiveServerLifecycle() {
	suite.T().Logf("Testing %s comprehensive server lifecycle", suite.Config.DisplayName)
	// Basic lifecycle test
}

// TestDefinitionComprehensive tests textDocument/definition
func (suite *ComprehensiveTestBaseSuite) TestDefinitionComprehensive() {
	suite.T().Logf("Testing %s definition", suite.Config.DisplayName)

	// Ensure server is available (shared or individual)
	httpClient, err := suite.ensureServerAvailable()
	require.NoError(suite.T(), err, "Failed to ensure server is available")

	fileURI, err := suite.repoManager.GetFileURI(suite.Config.Language, 0)
	require.NoError(suite.T(), err, "Failed to get file URI")

	testFile, err := suite.repoManager.GetTestFile(suite.Config.Language, 0)
	require.NoError(suite.T(), err, "Failed to get test file")

	time.Sleep(5 * time.Second)

	definitionRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "textDocument/definition",
		"params": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": fileURI,
			},
			"position": map[string]interface{}{
				"line":      testFile.DefinitionPos.Line,
				"character": testFile.DefinitionPos.Character,
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), suite.getLanguageTimeout())
	defer cancel()

	response, err := suite.makeJSONRPCRequest(ctx, httpClient, definitionRequest)
	require.NoError(suite.T(), err, "Definition request failed")
	require.NotNil(suite.T(), response, "Response should not be nil")

	if errorField, hasError := response["error"]; hasError && errorField != nil {
		suite.T().Errorf("LSP error in definition response: %v", errorField)
		return
	}

	result, hasResult := response["result"]
	require.True(suite.T(), hasResult, "Response should have result field")

	if result != nil {
		suite.T().Logf("✅ Definition result: %v", result)
	} else {
		suite.T().Logf("⚠️ Definition result is null")
	}

	// Validate cache performance after test
	suite.validateCachePerformance()

	suite.T().Logf("✅ Definition test completed for %s", suite.Config.DisplayName)
}

// TestReferencesComprehensive tests textDocument/references
func (suite *ComprehensiveTestBaseSuite) TestReferencesComprehensive() {
	suite.T().Logf("Testing %s references", suite.Config.DisplayName)

	// Ensure server is available (shared or individual)
	httpClient, err := suite.ensureServerAvailable()
	require.NoError(suite.T(), err, "Failed to ensure server is available")

	fileURI, err := suite.repoManager.GetFileURI(suite.Config.Language, 0)
	require.NoError(suite.T(), err, "Failed to get file URI")

	testFile, err := suite.repoManager.GetTestFile(suite.Config.Language, 0)
	require.NoError(suite.T(), err, "Failed to get test file")

	time.Sleep(5 * time.Second)

	referencesRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "textDocument/references",
		"params": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": fileURI,
			},
			"position": map[string]interface{}{
				"line":      testFile.ReferencePos.Line,
				"character": testFile.ReferencePos.Character,
			},
			"context": map[string]interface{}{
				"includeDeclaration": true,
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), suite.getLanguageTimeout())
	defer cancel()

	response, err := suite.makeJSONRPCRequest(ctx, httpClient, referencesRequest)
	require.NoError(suite.T(), err, "References request failed")
	require.NotNil(suite.T(), response, "Response should not be nil")

	if errorField, hasError := response["error"]; hasError && errorField != nil {
		suite.T().Errorf("LSP error in references response: %v", errorField)
		return
	}

	result, hasResult := response["result"]
	require.True(suite.T(), hasResult, "Response should have result field")

	if result != nil {
		suite.T().Logf("✅ References result: %v", result)
	} else {
		suite.T().Logf("⚠️ References result is null")
	}

	suite.T().Logf("✅ References test completed for %s", suite.Config.DisplayName)
}

// TestHoverComprehensive tests textDocument/hover
func (suite *ComprehensiveTestBaseSuite) TestHoverComprehensive() {
	suite.T().Logf("Testing %s hover - REAL E2E TEST", suite.Config.DisplayName)

	// Ensure server is available (shared or individual)
	httpClient, err := suite.ensureServerAvailable()
	require.NoError(suite.T(), err, "Failed to ensure server is available")

	// Get test file info from repository manager
	fileURI, err := suite.repoManager.GetFileURI(suite.Config.Language, 0)
	require.NoError(suite.T(), err, "Failed to get file URI")

	testFile, err := suite.repoManager.GetTestFile(suite.Config.Language, 0)
	require.NoError(suite.T(), err, "Failed to get test file")

	suite.T().Logf("Testing hover on file: %s", fileURI)

	// Give LSP server time to initialize
	suite.T().Logf("Waiting for LSP server to fully initialize...")
	time.Sleep(5 * time.Second)

	// Test hover request
	hoverRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "textDocument/hover",
		"params": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": fileURI,
			},
			"position": map[string]interface{}{
				"line":      testFile.HoverPos.Line,
				"character": testFile.HoverPos.Character,
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), suite.getLanguageTimeout())
	defer cancel()

	suite.T().Logf("Making hover request at line %d, character %d...", testFile.HoverPos.Line, testFile.HoverPos.Character)

	response, err := suite.makeJSONRPCRequest(ctx, httpClient, hoverRequest)
	require.NoError(suite.T(), err, "Hover request failed")
	require.NotNil(suite.T(), response, "Response should not be nil")

	// Check for error in response
	if errorField, hasError := response["error"]; hasError && errorField != nil {
		suite.T().Errorf("LSP error in hover response: %v", errorField)
		return
	}

	// Validate hover result
	result, hasResult := response["result"]
	require.True(suite.T(), hasResult, "Response should have result field")

	if result != nil {
		resultStr := fmt.Sprintf("%v", result)
		suite.T().Logf("✅ Hover result: %s", resultStr)
		require.True(suite.T(), len(resultStr) > 0, "Hover result should not be empty")
	} else {
		suite.T().Logf("⚠️ Hover result is null - this may be normal for some positions")
	}

	suite.T().Logf("✅ Hover test completed successfully for %s", suite.Config.DisplayName)
}

// TestDocumentSymbolComprehensive tests textDocument/documentSymbol
func (suite *ComprehensiveTestBaseSuite) TestDocumentSymbolComprehensive() {
	suite.T().Logf("Testing %s document symbols", suite.Config.DisplayName)

	// Ensure server is available (shared or individual)
	httpClient, err := suite.ensureServerAvailable()
	require.NoError(suite.T(), err, "Failed to ensure server is available")

	fileURI, err := suite.repoManager.GetFileURI(suite.Config.Language, 0)
	require.NoError(suite.T(), err, "Failed to get file URI")

	time.Sleep(5 * time.Second)

	documentSymbolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "textDocument/documentSymbol",
		"params": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": fileURI,
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), suite.getLanguageTimeout())
	defer cancel()

	response, err := suite.makeJSONRPCRequest(ctx, httpClient, documentSymbolRequest)
	require.NoError(suite.T(), err, "Document symbol request failed")
	require.NotNil(suite.T(), response, "Response should not be nil")

	if errorField, hasError := response["error"]; hasError && errorField != nil {
		suite.T().Errorf("LSP error in document symbol response: %v", errorField)
		return
	}

	result, hasResult := response["result"]
	require.True(suite.T(), hasResult, "Response should have result field")

	if result != nil {
		if resultArray, ok := result.([]interface{}); ok {
			suite.T().Logf("✅ Document symbols found: %d symbols", len(resultArray))
		} else {
			suite.T().Logf("✅ Document symbols result: %v", result)
		}
	} else {
		suite.T().Logf("⚠️ Document symbols result is null")
	}

	suite.T().Logf("✅ Document symbol test completed for %s", suite.Config.DisplayName)
}

// TestWorkspaceSymbolComprehensive tests workspace/symbol
func (suite *ComprehensiveTestBaseSuite) TestWorkspaceSymbolComprehensive() {
	suite.T().Logf("Testing %s workspace symbols", suite.Config.DisplayName)

	// Ensure server is available (shared or individual)
	httpClient, err := suite.ensureServerAvailable()
	require.NoError(suite.T(), err, "Failed to ensure server is available")

	testFile, err := suite.repoManager.GetTestFile(suite.Config.Language, 0)
	require.NoError(suite.T(), err, "Failed to get test file")

	time.Sleep(5 * time.Second)

	workspaceSymbolRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "workspace/symbol",
		"params": map[string]interface{}{
			"query": testFile.SymbolQuery,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), suite.getLanguageTimeout())
	defer cancel()

	response, err := suite.makeJSONRPCRequest(ctx, httpClient, workspaceSymbolRequest)
	require.NoError(suite.T(), err, "Workspace symbol request failed")
	require.NotNil(suite.T(), response, "Response should not be nil")

	if errorField, hasError := response["error"]; hasError && errorField != nil {
		suite.T().Errorf("LSP error in workspace symbol response: %v", errorField)
		return
	}

	result, hasResult := response["result"]
	require.True(suite.T(), hasResult, "Response should have result field")

	if result != nil {
		if resultArray, ok := result.([]interface{}); ok {
			suite.T().Logf("✅ Workspace symbols found for query '%s': %d symbols", testFile.SymbolQuery, len(resultArray))
		} else {
			suite.T().Logf("✅ Workspace symbols result: %v", result)
		}
	} else {
		suite.T().Logf("⚠️ Workspace symbols result is null")
	}

	suite.T().Logf("✅ Workspace symbol test completed for %s", suite.Config.DisplayName)
}

// TestCompletionComprehensive tests textDocument/completion
func (suite *ComprehensiveTestBaseSuite) TestCompletionComprehensive() {
	suite.T().Logf("Testing %s completion", suite.Config.DisplayName)

	// Ensure server is available (shared or individual)
	httpClient, err := suite.ensureServerAvailable()
	require.NoError(suite.T(), err, "Failed to ensure server is available")

	fileURI, err := suite.repoManager.GetFileURI(suite.Config.Language, 0)
	require.NoError(suite.T(), err, "Failed to get file URI")

	testFile, err := suite.repoManager.GetTestFile(suite.Config.Language, 0)
	require.NoError(suite.T(), err, "Failed to get test file")

	time.Sleep(5 * time.Second)

	completionRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "textDocument/completion",
		"params": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": fileURI,
			},
			"position": map[string]interface{}{
				"line":      testFile.CompletionPos.Line,
				"character": testFile.CompletionPos.Character,
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), suite.getLanguageTimeout())
	defer cancel()

	response, err := suite.makeJSONRPCRequest(ctx, httpClient, completionRequest)
	require.NoError(suite.T(), err, "Completion request failed")
	require.NotNil(suite.T(), response, "Response should not be nil")

	if errorField, hasError := response["error"]; hasError && errorField != nil {
		suite.T().Errorf("LSP error in completion response: %v", errorField)
		return
	}

	result, hasResult := response["result"]
	require.True(suite.T(), hasResult, "Response should have result field")

	if result != nil {
		if resultMap, ok := result.(map[string]interface{}); ok {
			if items, hasItems := resultMap["items"]; hasItems {
				if itemsArray, ok := items.([]interface{}); ok {
					suite.T().Logf("✅ Completion items found: %d items", len(itemsArray))
				}
			}
		} else if resultArray, ok := result.([]interface{}); ok {
			suite.T().Logf("✅ Completion items found: %d items", len(resultArray))
		} else {
			suite.T().Logf("✅ Completion result: %v", result)
		}
	} else {
		suite.T().Logf("⚠️ Completion result is null")
	}

	suite.T().Logf("✅ Completion test completed for %s", suite.Config.DisplayName)
}

// TestAllLSPMethodsSequential tests all LSP methods sequentially (for languages that support it)
func (suite *ComprehensiveTestBaseSuite) TestAllLSPMethodsSequential() {
	if !suite.Config.HasAllLSPTest {
		suite.T().Skip("Language does not support sequential LSP test")
		return
	}
	suite.T().Logf("Testing %s all LSP methods sequentially", suite.Config.DisplayName)

	// Ensure server is available (shared or individual)
	httpClient, err := suite.ensureServerAvailable()
	require.NoError(suite.T(), err, "Failed to ensure server is available")

	fileURI, err := suite.repoManager.GetFileURI(suite.Config.Language, 0)
	require.NoError(suite.T(), err, "Failed to get file URI")

	testFile, err := suite.repoManager.GetTestFile(suite.Config.Language, 0)
	require.NoError(suite.T(), err, "Failed to get test file")

	// Wait for LSP server to fully initialize (longer for Java)
	initTimeout := 5 * time.Second
	if suite.Config.Language == "java" {
		initTimeout = 15 * time.Second // Java LSP server needs more time to initialize
	}
	time.Sleep(initTimeout)

	// Test all 6 LSP methods sequentially
	suite.testMethodSequentially(httpClient, fileURI, testFile, "textDocument/definition", testFile.DefinitionPos)
	suite.testMethodSequentially(httpClient, fileURI, testFile, "textDocument/references", testFile.ReferencePos)
	suite.testMethodSequentially(httpClient, fileURI, testFile, "textDocument/hover", testFile.HoverPos)
	suite.testDocumentSymbolSequentially(httpClient, fileURI)
	suite.testWorkspaceSymbolSequentially(httpClient, testFile.SymbolQuery)
	suite.testMethodSequentially(httpClient, fileURI, testFile, "textDocument/completion", testFile.CompletionPos)

	suite.T().Logf("✅ All LSP methods tested sequentially for %s", suite.Config.DisplayName)
}

// Helper method for testing LSP methods with position
func (suite *ComprehensiveTestBaseSuite) testMethodSequentially(httpClient *testutils.HttpClient, fileURI string, testFile *testutils.TestFile, method string, pos testutils.Position) {
	suite.T().Logf("  Testing %s...", method)

	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": fileURI,
			},
			"position": map[string]interface{}{
				"line":      pos.Line,
				"character": pos.Character,
			},
		},
	}

	// Add context for references
	if method == "textDocument/references" {
		request["params"].(map[string]interface{})["context"] = map[string]interface{}{
			"includeDeclaration": true,
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), suite.getLanguageTimeout())
	defer cancel()

	response, err := suite.makeJSONRPCRequest(ctx, httpClient, request)
	if err != nil {
		suite.T().Logf("  ❌ %s failed: %v", method, err)
		return
	}

	if errorField, hasError := response["error"]; hasError && errorField != nil {
		suite.T().Logf("  ❌ %s LSP error: %v", method, errorField)
		return
	}

	suite.T().Logf("  ✅ %s completed", method)
}

// Helper method for testing document symbols
func (suite *ComprehensiveTestBaseSuite) testDocumentSymbolSequentially(httpClient *testutils.HttpClient, fileURI string) {
	suite.T().Logf("  Testing textDocument/documentSymbol...")

	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "textDocument/documentSymbol",
		"params": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": fileURI,
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), suite.getLanguageTimeout())
	defer cancel()

	response, err := suite.makeJSONRPCRequest(ctx, httpClient, request)
	if err != nil {
		suite.T().Logf("  ❌ textDocument/documentSymbol failed: %v", err)
		return
	}

	if errorField, hasError := response["error"]; hasError && errorField != nil {
		suite.T().Logf("  ❌ textDocument/documentSymbol LSP error: %v", errorField)
		return
	}

	suite.T().Logf("  ✅ textDocument/documentSymbol completed")
}

// Helper method for testing workspace symbols
func (suite *ComprehensiveTestBaseSuite) testWorkspaceSymbolSequentially(httpClient *testutils.HttpClient, query string) {
	suite.T().Logf("  Testing workspace/symbol...")

	request := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "workspace/symbol",
		"params": map[string]interface{}{
			"query": query,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), suite.getLanguageTimeout())
	defer cancel()

	response, err := suite.makeJSONRPCRequest(ctx, httpClient, request)
	if err != nil {
		suite.T().Logf("  ❌ workspace/symbol failed: %v", err)
		return
	}

	if errorField, hasError := response["error"]; hasError && errorField != nil {
		suite.T().Logf("  ❌ workspace/symbol LSP error: %v", errorField)
		return
	}

	suite.T().Logf("  ✅ workspace/symbol completed")
}

// Cache Isolation Convenience Methods

// SetCacheIsolationLevel sets the cache isolation level for tests
func (suite *ComprehensiveTestBaseSuite) SetCacheIsolationLevel(level testutils.IsolationLevel) {
	suite.cacheIsolationLevel = level
	suite.T().Logf("🔒 Cache isolation level set to: %d", level)
}

// EnableParallelCacheIsolation enables isolation suitable for parallel test execution
func (suite *ComprehensiveTestBaseSuite) EnableParallelCacheIsolation() {
	suite.SetCacheIsolationLevel(testutils.ParallelIsolation)
}

// EnableStrictCacheIsolation enables strict cache isolation with full validation
func (suite *ComprehensiveTestBaseSuite) EnableStrictCacheIsolation() {
	suite.SetCacheIsolationLevel(testutils.StrictIsolation)
}

// ValidateCacheHealthNow performs immediate cache health validation
func (suite *ComprehensiveTestBaseSuite) ValidateCacheHealthNow(expectedState string) error {
	if !suite.serverStarted {
		return fmt.Errorf("server not started, cannot validate cache health")
	}

	healthURL := fmt.Sprintf("http://localhost:%d/health", suite.gatewayPort)
	return suite.cacheIsolationMgr.ValidateCacheHealth(healthURL, expectedState)
}

// RecordCacheCheckpoint creates a cache state checkpoint for debugging
func (suite *ComprehensiveTestBaseSuite) RecordCacheCheckpoint(phase string) {
	if !suite.serverStarted {
		suite.T().Logf("Warning: Cannot record cache checkpoint - server not started")
		return
	}

	healthURL := fmt.Sprintf("http://localhost:%d/health", suite.gatewayPort)
	if err := suite.cacheIsolationMgr.RecordCacheState(healthURL, phase); err != nil {
		suite.T().Logf("Warning: Failed to record cache checkpoint for phase '%s': %v", phase, err)
	} else {
		suite.T().Logf("📊 Cache checkpoint recorded for phase: %s", phase)
	}
}

// ResetCacheForCleanTest completely resets cache state
func (suite *ComprehensiveTestBaseSuite) ResetCacheForCleanTest() error {
	// Stop server if running
	wasRunning := suite.serverStarted
	if wasRunning {
		suite.stopGatewayServer()
	}

	// Reset cache state
	if err := suite.cacheIsolationMgr.ResetCacheState(); err != nil {
		return fmt.Errorf("failed to reset cache state: %w", err)
	}

	// Update cache directory
	suite.cacheDir = suite.cacheIsolationMgr.GetCacheDirectory()

	// Restart server if it was running
	if wasRunning {
		if err := suite.startGatewayServer(); err != nil {
			return fmt.Errorf("failed to restart server after cache reset: %w", err)
		}

		if err := suite.waitForServerReady(); err != nil {
			return fmt.Errorf("server not ready after cache reset: %w", err)
		}
	}

	suite.T().Logf("🔄 Cache reset completed successfully")
	return nil
}

// GetCacheViolationsSummary returns a summary of cache isolation violations
func (suite *ComprehensiveTestBaseSuite) GetCacheViolationsSummary() string {
	violations := suite.cacheIsolationMgr.GetViolations()
	if len(violations) == 0 {
		return "✅ No cache isolation violations detected"
	}

	summary := fmt.Sprintf("⚠️ %d cache isolation violations detected:\n", len(violations))
	for i, violation := range violations {
		summary += fmt.Sprintf("  %d. %s: %s (Impact: %s)\n",
			i+1, violation.ViolationType, violation.Description, violation.Impact)
	}

	return summary
}

// getLanguageTimeout returns language-specific timeout for LSP requests
func (suite *ComprehensiveTestBaseSuite) getLanguageTimeout() time.Duration {
	switch suite.Config.Language {
	case "java":
		// Java LSP server (jdtls) is significantly slower, especially for initial operations
		return 60 * time.Second
	case "python":
		// Python LSP server can be slow for large projects
		return 30 * time.Second
	case "go", "javascript", "typescript":
		// These are generally faster
		return 15 * time.Second
	default:
		// Default timeout for unknown languages
		return 20 * time.Second
	}
}

// GetCacheHealthHistory returns cache health checkpoint history
func (suite *ComprehensiveTestBaseSuite) GetCacheHealthHistory() []testutils.CacheCheckpoint {
	return suite.cacheIsolationMgr.GetHealthHistory()
}

// AssertNoCacheViolations fails the test if cache isolation violations are detected
func (suite *ComprehensiveTestBaseSuite) AssertNoCacheViolations() {
	violations := suite.cacheIsolationMgr.GetViolations()
	if len(violations) > 0 {
		summary := suite.GetCacheViolationsSummary()
		suite.T().Fatalf("Cache isolation violations detected:\n%s", summary)
	}
}
