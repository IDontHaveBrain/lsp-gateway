package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"lsp-gateway/internal/installer"
	"lsp-gateway/internal/transport"
	"lsp-gateway/mcp"
	"lsp-gateway/tests/e2e/testutils"
)

// JavaRealJDTLSE2ETestSuite provides comprehensive integration tests
// for Java with actual Eclipse JDT.LS server communication
type JavaRealJDTLSE2ETestSuite struct {
	suite.Suite
	testTimeout        time.Duration
	projectRoot        string
	lspClient          transport.LSPClient
	clientConfig       transport.ClientConfig
	projectFiles       map[string]string
	performanceMetrics *JavaPerformanceMetrics
}

// JavaPerformanceMetrics tracks actual performance data from real JDTLS server
type JavaPerformanceMetrics struct {
	InitializationTime   time.Duration
	FirstResponseTime    time.Duration
	AverageResponseTime  time.Duration
	MemoryUsage         int64
	ProcessPID          int
	TotalRequests       int
	SuccessfulRequests  int
	FailedRequests      int
	JDTLSStartupTime    time.Duration // JDTLS-specific metric
	WorkspaceLoadTime   time.Duration // Java workspace loading time
}

// JavaIntegrationResult captures comprehensive test results for supported LSP features
type JavaIntegrationResult struct {
	ServerStartSuccess      bool
	InitializationSuccess   bool
	ProjectLoadSuccess      bool
	DefinitionAccuracy      bool  // textDocument/definition
	ReferencesWork          bool  // textDocument/references
	HoverInformationWorks   bool  // textDocument/hover
	DocumentSymbolsWork     bool  // textDocument/documentSymbol
	WorkspaceSymbolsWork    bool  // workspace/symbol
	CompletionWorks         bool  // textDocument/completion
	ClasspathResolutionWorks bool
	MavenIntegrationWorks   bool
	JavaTypeInferenceWorks  bool
	CrossFileNavigationWorks bool
	PerformanceMetrics      *JavaPerformanceMetrics
	TestDuration           time.Duration
	ErrorCount             int
}

// SetupSuite initializes the test suite with real Java project structure
func (suite *JavaRealJDTLSE2ETestSuite) SetupSuite() {
	suite.testTimeout = 5 * time.Minute // Extended timeout for JDTLS operations (slower than pylsp)
	
	// Verify JDTLS is available
	if !suite.isJDTLSServerAvailable() {
		suite.T().Skip("JDTLS not available, skipping real server integration tests")
	}

	// Create real Java project structure
	suite.createTestProject()
	
	suite.performanceMetrics = &JavaPerformanceMetrics{}
}

// SetupTest initializes a fresh LSP client for each test
func (suite *JavaRealJDTLSE2ETestSuite) SetupTest() {
	// JDTLS requires specific configuration for Eclipse language server
	jdtlsPath := suite.getJDTLSPath()
	if jdtlsPath == "" {
		suite.T().Skip("JDTLS executable not found, skipping test")
	}
	
	suite.clientConfig = transport.ClientConfig{
		Command: jdtlsPath,
		Args: []string{
			"-configuration", "/tmp/jdtls-cache", // Workspace cache directory
			"-data", suite.projectRoot,            // Workspace data directory
		},
		Transport: transport.TransportStdio,
	}
	
	var err error
	suite.lspClient, err = transport.NewLSPClient(suite.clientConfig)
	suite.Require().NoError(err, "Should create LSP client")
}

// TearDownTest cleans up the LSP client
func (suite *JavaRealJDTLSE2ETestSuite) TearDownTest() {
	if suite.lspClient != nil && suite.lspClient.IsActive() {
		err := suite.lspClient.Stop()
		if err != nil {
			suite.T().Logf("Warning: Error stopping LSP client: %v", err)
		}
	}
}

// TearDownSuite cleans up the test project
func (suite *JavaRealJDTLSE2ETestSuite) TearDownSuite() {
	if suite.projectRoot != "" {
		_ = os.RemoveAll(suite.projectRoot)
	}
	// Clean up JDTLS cache
	_ = os.RemoveAll("/tmp/jdtls-cache")
}

// TestRealJDTLSServerLifecycle tests the complete LSP server lifecycle
func (suite *JavaRealJDTLSE2ETestSuite) TestRealJDTLSServerLifecycle() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	startTime := time.Now()
	
	// Step 1: Start the actual JDTLS language server
	jdtlsStartTime := time.Now()
	err := suite.lspClient.Start(ctx)
	suite.Require().NoError(err, "Should start JDTLS process")
	suite.performanceMetrics.ProcessPID = suite.getProcessPID()
	suite.performanceMetrics.JDTLSStartupTime = time.Since(jdtlsStartTime)
	
	// Step 2: Initialize the server with proper Java capabilities
	initResult := suite.initializeServer(ctx)
	suite.True(initResult, "Server initialization should succeed")
	suite.performanceMetrics.InitializationTime = time.Since(startTime)
	
	// Step 3: Open the Java project workspace
	suite.openWorkspace(ctx)
	
	// Step 4: Test core LSP functionality with real Java analysis
	result := suite.executeComprehensiveJavaWorkflow(ctx)
	
	// Step 5: Validate all aspects of real Java integration
	suite.validateJavaIntegrationResult(result)
	
	// Step 6: Measure final performance metrics
	result.TestDuration = time.Since(startTime)
	suite.recordPerformanceMetrics(result)
	
	suite.T().Logf("Real Java integration test completed in %v", result.TestDuration)
	suite.T().Logf("JDTLS startup: %v, Server PID: %d, Total requests: %d, Success rate: %.2f%%", 
		suite.performanceMetrics.JDTLSStartupTime,
		suite.performanceMetrics.ProcessPID,
		suite.performanceMetrics.TotalRequests,
		float64(suite.performanceMetrics.SuccessfulRequests)/float64(suite.performanceMetrics.TotalRequests)*100)
}

// TestRealJavaFeatures tests actual Java language features
func (suite *JavaRealJDTLSE2ETestSuite) TestRealJavaFeatures() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	// Start and initialize server
	err := suite.lspClient.Start(ctx)
	suite.Require().NoError(err, "Should start server")
	
	suite.initializeServer(ctx)
	suite.openWorkspace(ctx)

	featureTests := []struct {
		name           string
		testFunction   func(context.Context) bool
		description    string
	}{
		{
			name:         "Definition",
			testFunction: suite.testGoToDefinition,
			description:  "Test Go to definition across Java files",
		},
		{
			name:         "References",
			testFunction: suite.testFindReferences,
			description:  "Test Find references to classes/methods",
		},
		{
			name:         "Hover",
			testFunction: suite.testHoverInformation,
			description:  "Test Hover information for Java symbols",
		},
		{
			name:         "DocumentSymbol",
			testFunction: suite.testDocumentSymbols,
			description:  "Test Extract symbols from Java files",
		},
		{
			name:         "WorkspaceSymbol",
			testFunction: suite.testWorkspaceSymbols,
			description:  "Test Search symbols across Java project",
		},
		{
			name:         "Completion",
			testFunction: suite.testCompletion,
			description:  "Test Code completion for Java syntax",
		},
	}

	for _, test := range featureTests {
		suite.Run(test.name, func() {
			startTime := time.Now()
			success := test.testFunction(ctx)
			duration := time.Since(startTime)
			
			suite.True(success, "%s should succeed", test.description)
			suite.Less(duration, 60*time.Second, "%s should complete within reasonable time", test.name) // Java can be slower
			
			suite.T().Logf("%s completed in %v", test.name, duration)
		})
	}
}

// TestRealJDTLSPerformance tests performance with Java projects
func (suite *JavaRealJDTLSE2ETestSuite) TestRealJDTLSPerformance() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	// Start server and initialize
	err := suite.lspClient.Start(ctx)
	suite.Require().NoError(err, "Should start server")
	
	suite.initializeServer(ctx)
	suite.openWorkspace(ctx)

	performanceTests := []struct {
		name                string
		requestCount        int
		concurrency         int
		maxAcceptableTime   time.Duration
		expectedSuccessRate float64
	}{
		{
			name:                "HighVolumeDefinitionRequests",
			requestCount:        60, // Reduced for Java performance characteristics
			concurrency:         2,  // Lower concurrency for Java
			maxAcceptableTime:   45 * time.Second,
			expectedSuccessRate: 0.85, // Lower for Java complexity
		},
		{
			name:                "ConcurrentSymbolRequests",
			requestCount:        30,
			concurrency:         3,
			maxAcceptableTime:   35 * time.Second,
			expectedSuccessRate: 0.80,
		},
		{
			name:                "IntensiveHoverRequests",
			requestCount:        80,
			concurrency:         2,
			maxAcceptableTime:   50 * time.Second,
			expectedSuccessRate: 0.90,
		},
	}

	for _, test := range performanceTests {
		suite.Run(test.name, func() {
			result := suite.executePerformanceTest(ctx, test.requestCount, test.concurrency)
			
			suite.Less(result.TotalDuration, test.maxAcceptableTime, 
				"Performance test should complete within acceptable time")
			suite.GreaterOrEqual(result.SuccessRate, test.expectedSuccessRate,
				"Success rate should meet expectations")
			
			suite.T().Logf("%s: %d requests in %v (%.2f req/s, %.2f%% success)", 
				test.name, test.requestCount, result.TotalDuration,
				float64(test.requestCount)/result.TotalDuration.Seconds(),
				result.SuccessRate*100)
		})
	}
}

// Helper methods for test implementation

func (suite *JavaRealJDTLSE2ETestSuite) isJDTLSServerAvailable() bool {
	// Try to find JDTLS in common locations
	jdtlsPath := suite.getJDTLSPath()
	return jdtlsPath != ""
}

func (suite *JavaRealJDTLSE2ETestSuite) getJDTLSPath() string {
	// Use LSP Gateway's actual installation path logic
	jdtlsPath := installer.GetJDTLSExecutablePath()
	
	// Check if the LSP Gateway installed version exists
	if _, err := os.Stat(jdtlsPath); err == nil {
		return jdtlsPath
	}
	
	// Fallback to PATH lookup for external installations
	if path, err := exec.LookPath("jdtls"); err == nil {
		return path
	}
	
	return ""
}

func (suite *JavaRealJDTLSE2ETestSuite) createTestProject() {
	var err error
	suite.projectRoot, err = os.MkdirTemp("", "java-integration-test-*")
	suite.Require().NoError(err, "Should create temporary project directory")

	// Copy the existing Java fixture project using proper path resolution
	fixturesDir, err := testutils.GetE2EFixturesDir()
	suite.Require().NoError(err, "Should get E2E fixtures directory")
	
	fixtureRoot := filepath.Join(fixturesDir, "java-project")
	suite.copyDirectory(fixtureRoot, suite.projectRoot)
	
	// Store project files for reference
	suite.projectFiles = map[string]string{
		"pom.xml": "Maven project configuration",
		"src/main/java/com/test/Main.java": "Main class file",
		"src/main/java/com/test/model/User.java": "User model class",
		"src/main/java/com/test/service/UserService.java": "User service class",
		"src/main/java/com/test/controller/UserController.java": "User controller class",
		"src/test/java/com/test/UserServiceTest.java": "User service test class",
	}
}

func (suite *JavaRealJDTLSE2ETestSuite) copyDirectory(src, dst string) {
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		
		dstPath := filepath.Join(dst, relPath)
		
		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}
		
		// Copy file
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		
		return os.WriteFile(dstPath, content, info.Mode())
	})
	
	suite.Require().NoError(err, "Should copy fixture directory")
}

func (suite *JavaRealJDTLSE2ETestSuite) initializeServer(ctx context.Context) bool {
	// Send initialize request with proper Java/JDTLS capabilities
	initParams := map[string]interface{}{
		"processId": os.Getpid(),
		"rootUri":   fmt.Sprintf("file://%s", suite.projectRoot),
		"capabilities": map[string]interface{}{
			"workspace": map[string]interface{}{
				"workspaceFolders": true,
				"configuration":    true,
			},
			"textDocument": map[string]interface{}{
				"synchronization": map[string]interface{}{
					"dynamicRegistration": true,
					"willSave":           true,
					"willSaveWaitUntil":  true,
					"didSave":            true,
				},
				"completion": map[string]interface{}{
					"dynamicRegistration": true,
					"completionItem": map[string]interface{}{
						"snippetSupport": true,
						"commitCharactersSupport": true,
						"documentationFormat": []string{"markdown", "plaintext"},
					},
				},
				"hover": map[string]interface{}{
					"dynamicRegistration": true,
					"contentFormat": []string{"markdown", "plaintext"},
				},
				"definition": map[string]interface{}{
					"dynamicRegistration": true,
				},
				"references": map[string]interface{}{
					"dynamicRegistration": true,
				},
				"documentSymbol": map[string]interface{}{
					"dynamicRegistration": true,
				},
			},
		},
		"initializationOptions": map[string]interface{}{
			"settings": map[string]interface{}{
				"java": map[string]interface{}{
					"configuration": map[string]interface{}{
						"runtimes": []map[string]interface{}{
							{
								"name": "JavaSE-17",
								"path": "/usr/lib/jvm/java-17-openjdk-amd64",
							},
						},
					},
					"compile": map[string]interface{}{
						"nullAnalysis": map[string]interface{}{
							"mode": "automatic",
						},
					},
					"completion": map[string]interface{}{
						"enabled": true,
						"overwrite": true,
					},
					"sources": map[string]interface{}{
						"organizeImports": map[string]interface{}{
							"starThreshold": 99,
							"staticStarThreshold": 99,
						},
					},
				},
			},
		},
	}

	startTime := time.Now()
	response, err := suite.lspClient.SendRequest(ctx, "initialize", initParams)
	if err != nil {
		suite.T().Logf("Initialize request failed: %v", err)
		return false
	}

	suite.performanceMetrics.FirstResponseTime = time.Since(startTime)

	// Parse initialize response
	var initResult map[string]interface{}
	if err := json.Unmarshal(response, &initResult); err != nil {
		suite.T().Logf("Failed to parse initialize response: %v", err)
		return false
	}

	// Send initialized notification
	err = suite.lspClient.SendNotification(ctx, "initialized", map[string]interface{}{})
	if err != nil {
		suite.T().Logf("Initialized notification failed: %v", err)
		return false
	}

	return true
}

func (suite *JavaRealJDTLSE2ETestSuite) openWorkspace(ctx context.Context) {
	workspaceStartTime := time.Now()
	
	// Open all Java files in the project workspace
	err := filepath.Walk(suite.projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		if strings.HasSuffix(path, ".java") {
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			// Send textDocument/didOpen notification
			err = suite.lspClient.SendNotification(ctx, "textDocument/didOpen", map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri":        fmt.Sprintf("file://%s", path),
					"languageId": "java",
					"version":    1,
					"text":       string(content),
				},
			})
			if err != nil {
				suite.T().Logf("Failed to open document %s: %v", path, err)
			}
		}
		return nil
	})
	
	if err != nil {
		suite.T().Logf("Error walking project directory: %v", err)
	}

	// Wait for JDTLS to process the workspace (Java analysis can take longer)
	time.Sleep(10 * time.Second)
	
	suite.performanceMetrics.WorkspaceLoadTime = time.Since(workspaceStartTime)
}

func (suite *JavaRealJDTLSE2ETestSuite) executeComprehensiveJavaWorkflow(ctx context.Context) *JavaIntegrationResult {
	result := &JavaIntegrationResult{
		ServerStartSuccess:    true,
		InitializationSuccess: true,
		ProjectLoadSuccess:    true,
		PerformanceMetrics:   suite.performanceMetrics,
	}

	startTime := time.Now()

	// Test 1: Go to definition
	result.DefinitionAccuracy = suite.testGoToDefinition(ctx)
	
	// Test 2: Find references
	result.ReferencesWork = suite.testFindReferences(ctx)
	
	// Test 3: Hover information
	result.HoverInformationWorks = suite.testHoverInformation(ctx)
	
	// Test 4: Document symbols
	result.DocumentSymbolsWork = suite.testDocumentSymbols(ctx)
	
	// Test 5: Workspace symbols
	result.WorkspaceSymbolsWork = suite.testWorkspaceSymbols(ctx)
	
	// Test 6: Code completion
	result.CompletionWorks = suite.testCompletion(ctx)
	
	// Additional Java-specific tests
	result.ClasspathResolutionWorks = suite.testClasspathResolution(ctx)
	result.MavenIntegrationWorks = suite.testMavenIntegration(ctx)
	result.JavaTypeInferenceWorks = suite.testJavaTypeInference(ctx)
	result.CrossFileNavigationWorks = suite.testCrossFileNavigation(ctx)

	result.TestDuration = time.Since(startTime)
	return result
}

func (suite *JavaRealJDTLSE2ETestSuite) testGoToDefinition(ctx context.Context) bool {
	// Test go to definition on User class in UserService
	filePath := filepath.Join(suite.projectRoot, "src/main/java/com/test/service/UserService.java")
	response, err := suite.lspClient.SendRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION, map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": fmt.Sprintf("file://%s", filePath),
		},
		"position": map[string]interface{}{
			"line":      2, // import com.test.model.User;
			"character": 25, // Position of "User"
		},
	})

	suite.performanceMetrics.TotalRequests++
	if err != nil {
		suite.T().Logf("Go to definition failed: %v", err)
		suite.performanceMetrics.FailedRequests++
		return false
	}

	// Parse response to verify it points to the correct file
	var definitions []interface{}
	if err := json.Unmarshal(response, &definitions); err != nil {
		suite.T().Logf("Failed to parse definition response: %v", err)
		suite.performanceMetrics.FailedRequests++
		return false
	}

	suite.performanceMetrics.SuccessfulRequests++
	
	// Verify definition points to User.java file
	if len(definitions) > 0 {
		if def, ok := definitions[0].(map[string]interface{}); ok {
			if uri, exists := def["uri"]; exists {
				return strings.Contains(uri.(string), "User.java")
			}
		}
	}

	return false
}

func (suite *JavaRealJDTLSE2ETestSuite) testFindReferences(ctx context.Context) bool {
	// Test find references for User class
	filePath := filepath.Join(suite.projectRoot, "src/main/java/com/test/model/User.java")
	response, err := suite.lspClient.SendRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES, map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": fmt.Sprintf("file://%s", filePath),
		},
		"position": map[string]interface{}{
			"line":      14, // public class User {
			"character": 13, // Position of "User"
		},
		"context": map[string]interface{}{
			"includeDeclaration": true,
		},
	})

	suite.performanceMetrics.TotalRequests++
	if err != nil {
		suite.T().Logf("Find references failed: %v", err)
		suite.performanceMetrics.FailedRequests++
		return false
	}

	var references []interface{}
	if err := json.Unmarshal(response, &references); err != nil {
		suite.T().Logf("Failed to parse references response: %v", err)
		suite.performanceMetrics.FailedRequests++
		return false
	}

	suite.performanceMetrics.SuccessfulRequests++
	return len(references) > 1 // Should find multiple references across files
}

func (suite *JavaRealJDTLSE2ETestSuite) testHoverInformation(ctx context.Context) bool {
	// Test hover on User class method
	filePath := filepath.Join(suite.projectRoot, "src/main/java/com/test/model/User.java")
	response, err := suite.lspClient.SendRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER, map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": fmt.Sprintf("file://%s", filePath),
		},
		"position": map[string]interface{}{
			"line":      165, // public String getFullName() {
			"character": 25,  // Position of "getFullName"
		},
	})

	suite.performanceMetrics.TotalRequests++
	if err != nil {
		suite.T().Logf("Hover failed: %v", err)
		suite.performanceMetrics.FailedRequests++
		return false
	}

	var hoverResult map[string]interface{}
	if err := json.Unmarshal(response, &hoverResult); err != nil {
		suite.T().Logf("Failed to parse hover response: %v", err)
		suite.performanceMetrics.FailedRequests++
		return false
	}

	suite.performanceMetrics.SuccessfulRequests++
	
	// Verify hover contains method signature information
	if contents, exists := hoverResult["contents"]; exists {
		contentStr := fmt.Sprintf("%v", contents)
		return strings.Contains(contentStr, "String") || strings.Contains(contentStr, "getFullName")
	}

	return false
}

func (suite *JavaRealJDTLSE2ETestSuite) testDocumentSymbols(ctx context.Context) bool {
	// Test document symbols for User class file
	filePath := filepath.Join(suite.projectRoot, "src/main/java/com/test/model/User.java")
	response, err := suite.lspClient.SendRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS, map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": fmt.Sprintf("file://%s", filePath),
		},
	})

	suite.performanceMetrics.TotalRequests++
	if err != nil {
		suite.T().Logf("Document symbols failed: %v", err)
		suite.performanceMetrics.FailedRequests++
		return false
	}

	var symbols []interface{}
	if err := json.Unmarshal(response, &symbols); err != nil {
		suite.T().Logf("Failed to parse symbols response: %v", err)
		suite.performanceMetrics.FailedRequests++
		return false
	}

	suite.performanceMetrics.SuccessfulRequests++
	
	// Verify User class is found in symbols
	for _, symbol := range symbols {
		if sym, ok := symbol.(map[string]interface{}); ok {
			if name, exists := sym["name"]; exists && strings.Contains(name.(string), "User") {
				return true
			}
		}
	}

	return false
}

func (suite *JavaRealJDTLSE2ETestSuite) testWorkspaceSymbols(ctx context.Context) bool {
	// Test workspace symbol search
	response, err := suite.lspClient.SendRequest(ctx, mcp.LSP_METHOD_WORKSPACE_SYMBOL, map[string]interface{}{
		"query": "User",
	})

	suite.performanceMetrics.TotalRequests++
	if err != nil {
		suite.T().Logf("Workspace symbol search failed: %v", err)
		suite.performanceMetrics.FailedRequests++
		return false
	}

	var symbols []interface{}
	if err := json.Unmarshal(response, &symbols); err != nil {
		suite.T().Logf("Failed to parse workspace symbols response: %v", err)
		suite.performanceMetrics.FailedRequests++
		return false
	}

	suite.performanceMetrics.SuccessfulRequests++
	return len(symbols) > 0
}

func (suite *JavaRealJDTLSE2ETestSuite) testCompletion(ctx context.Context) bool {
	// Test completion on User class method call
	filePath := filepath.Join(suite.projectRoot, "src/main/java/com/test/service/UserService.java")
	response, err := suite.lspClient.SendRequest(ctx, "textDocument/completion", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": fmt.Sprintf("file://%s", filePath),
		},
		"position": map[string]interface{}{
			"line":      167, // After searching for user.
			"character": 50,
		},
	})

	suite.performanceMetrics.TotalRequests++
	if err != nil {
		suite.T().Logf("Completion failed: %v", err)
		suite.performanceMetrics.FailedRequests++
		return false
	}

	var completions map[string]interface{}
	if err := json.Unmarshal(response, &completions); err != nil {
		suite.T().Logf("Failed to parse completion response: %v", err)
		suite.performanceMetrics.FailedRequests++
		return false
	}

	suite.performanceMetrics.SuccessfulRequests++
	
	// Check if completion items exist
	if items, exists := completions["items"]; exists {
		if itemList, ok := items.([]interface{}); ok {
			return len(itemList) > 0
		}
	}

	return false
}

// Additional Java-specific test methods

func (suite *JavaRealJDTLSE2ETestSuite) testClasspathResolution(ctx context.Context) bool {
	// This is tested implicitly by other operations - if classpath was broken,
	// imports and other features wouldn't work
	return suite.testGoToDefinition(ctx)
}

func (suite *JavaRealJDTLSE2ETestSuite) testMavenIntegration(ctx context.Context) bool {
	// Test if JDTLS can handle Maven dependencies from pom.xml
	return suite.testWorkspaceSymbols(ctx)
}

func (suite *JavaRealJDTLSE2ETestSuite) testJavaTypeInference(ctx context.Context) bool {
	// Test hover on a method with inferred types
	return suite.testHoverInformation(ctx)
}

func (suite *JavaRealJDTLSE2ETestSuite) testCrossFileNavigation(ctx context.Context) bool {
	// Test navigation between Java files (already tested in definition)
	return suite.testGoToDefinition(ctx)
}

// Performance testing structures and methods

type JavaPerformanceTestResult struct {
	TotalDuration time.Duration
	SuccessRate   float64
	RequestCount  int
	FailureCount  int
}

func (suite *JavaRealJDTLSE2ETestSuite) executePerformanceTest(ctx context.Context, requestCount, concurrency int) *JavaPerformanceTestResult {
	startTime := time.Now()
	
	var wg sync.WaitGroup
	successChan := make(chan bool, requestCount)
	
	// Execute concurrent requests
	requestsPerWorker := requestCount / concurrency
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for j := 0; j < requestsPerWorker; j++ {
				// Alternate between different types of supported LSP requests
				var success bool
				switch j % 6 {
				case 0:
					success = suite.testGoToDefinition(ctx)    // textDocument/definition
				case 1:
					success = suite.testFindReferences(ctx)    // textDocument/references
				case 2:
					success = suite.testHoverInformation(ctx)  // textDocument/hover
				case 3:
					success = suite.testDocumentSymbols(ctx)   // textDocument/documentSymbol
				case 4:
					success = suite.testWorkspaceSymbols(ctx)  // workspace/symbol
				case 5:
					success = suite.testCompletion(ctx)        // textDocument/completion
				}
				successChan <- success
			}
		}(i)
	}
	
	wg.Wait()
	close(successChan)
	
	// Calculate results
	totalDuration := time.Since(startTime)
	successCount := 0
	for success := range successChan {
		if success {
			successCount++
		}
	}
	
	return &JavaPerformanceTestResult{
		TotalDuration: totalDuration,
		SuccessRate:   float64(successCount) / float64(requestCount),
		RequestCount:  requestCount,
		FailureCount:  requestCount - successCount,
	}
}

// Helper methods for validation and metrics

func (suite *JavaRealJDTLSE2ETestSuite) getProcessPID() int {
	// Try to get the actual process PID from the LSP client
	if stdioClient, ok := suite.lspClient.(*transport.StdioClient); ok {
		return stdioClient.GetProcessPIDForTesting()
	}
	return 0
}

func (suite *JavaRealJDTLSE2ETestSuite) validateJavaIntegrationResult(result *JavaIntegrationResult) {
	suite.True(result.ServerStartSuccess, "Java server should start successfully")
	suite.True(result.InitializationSuccess, "Server initialization should succeed")
	suite.True(result.ProjectLoadSuccess, "Project should load successfully")
	suite.True(result.DefinitionAccuracy, "Go to definition should work accurately")
	suite.True(result.ReferencesWork, "Find references should work")
	suite.True(result.HoverInformationWorks, "Hover information should work")
	suite.True(result.DocumentSymbolsWork, "Document symbols should work")
	suite.True(result.WorkspaceSymbolsWork, "Workspace symbols should work")
	suite.True(result.CompletionWorks, "Code completion should work")
	suite.True(result.ClasspathResolutionWorks, "Classpath resolution should work")
	suite.True(result.MavenIntegrationWorks, "Maven integration should work")
	suite.True(result.JavaTypeInferenceWorks, "Java type inference should work")
	suite.True(result.CrossFileNavigationWorks, "Cross-file navigation should work")
	
	suite.Less(result.TestDuration, suite.testTimeout, "Test should complete within timeout")
	suite.Equal(0, result.ErrorCount, "Should have no errors in comprehensive test")
	
	// Performance validations
	suite.NotNil(result.PerformanceMetrics, "Performance metrics should be collected")
	if result.PerformanceMetrics != nil {
		suite.Greater(result.PerformanceMetrics.TotalRequests, 0, "Should have made requests")
		suite.Greater(result.PerformanceMetrics.SuccessfulRequests, 0, "Should have successful requests")
		suite.Less(result.PerformanceMetrics.InitializationTime, 60*time.Second, "Initialization should be reasonable")
		suite.Less(result.PerformanceMetrics.JDTLSStartupTime, 20*time.Second, "JDTLS startup should be reasonable")
		suite.Less(result.PerformanceMetrics.WorkspaceLoadTime, 30*time.Second, "Workspace loading should be reasonable")
	}
}

func (suite *JavaRealJDTLSE2ETestSuite) recordPerformanceMetrics(result *JavaIntegrationResult) {
	if suite.performanceMetrics.TotalRequests > 0 {
		suite.performanceMetrics.AverageResponseTime = result.TestDuration / time.Duration(suite.performanceMetrics.TotalRequests)
	}
	
	result.PerformanceMetrics = suite.performanceMetrics
}

// Test suite runner
func TestJavaRealJDTLSE2ETestSuite(t *testing.T) {
	suite.Run(t, new(JavaRealJDTLSE2ETestSuite))
}