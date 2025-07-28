package e2e_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"lsp-gateway/tests/e2e/testutils"
)

type LSPEdgeCasesTestSuite struct {
	suite.Suite
	httpClient     *testutils.HttpClient
	gatewayCmd     *exec.Cmd
	gatewayPort    int
	configPath     string
	tempDir        string
	projectRoot    string
	testTimeout    time.Duration
}

func (suite *LSPEdgeCasesTestSuite) SetupSuite() {
	suite.testTimeout = 15 * time.Second
	
	var err error
	suite.projectRoot, err = testutils.GetProjectRoot()
	suite.Require().NoError(err)
	
	suite.tempDir, err = os.MkdirTemp("", "lsp-edge-cases-test-*")
	suite.Require().NoError(err)
	
	suite.gatewayPort, err = testutils.FindAvailablePort()
	suite.Require().NoError(err)

	suite.createTestConfig()
}

func (suite *LSPEdgeCasesTestSuite) SetupTest() {
	config := testutils.HttpClientConfig{
		BaseURL:         fmt.Sprintf("http://localhost:%d", suite.gatewayPort),
		Timeout:         3 * time.Second,
		MaxRetries:      2,
		RetryDelay:      200 * time.Millisecond,
		EnableLogging:   true,
		EnableRecording: true,
		WorkspaceID:     "edge-cases-test-workspace",
		ProjectPath:     suite.tempDir,
	}
	suite.httpClient = testutils.NewHttpClient(config)
}

func (suite *LSPEdgeCasesTestSuite) TearDownTest() {
	if suite.httpClient != nil {
		suite.httpClient.Close()
	}
}

func (suite *LSPEdgeCasesTestSuite) TearDownSuite() {
	if suite.gatewayCmd != nil && suite.gatewayCmd.Process != nil {
		suite.gatewayCmd.Process.Signal(syscall.SIGTERM)
		suite.gatewayCmd.Wait()
	}
	if suite.tempDir != "" {
		os.RemoveAll(suite.tempDir)
	}
}

func (suite *LSPEdgeCasesTestSuite) createTestConfig() {
	configContent := fmt.Sprintf(`
servers:
- name: test-lsp
  languages:
  - go
  - python
  - typescript
  - java
  command: echo
  args: ["test-response"]
  transport: stdio
  root_markers:
  - go.mod
  - setup.py
  - package.json
  - pom.xml
  priority: 1
  weight: 1.0
port: %d
timeout: 5s
max_concurrent_requests: 50
`, suite.gatewayPort)

	var err error
	suite.configPath, _, err = testutils.CreateTempConfig(configContent)
	suite.Require().NoError(err)
}

func (suite *LSPEdgeCasesTestSuite) startGatewayServer() {
	binaryPath := filepath.Join(suite.projectRoot, "bin", "lsp-gateway")
	suite.gatewayCmd = exec.Command(binaryPath, "server", "--config", suite.configPath)
	suite.gatewayCmd.Dir = suite.projectRoot
	
	err := suite.gatewayCmd.Start()
	suite.Require().NoError(err)
	
	time.Sleep(2 * time.Second)
}

func (suite *LSPEdgeCasesTestSuite) stopGatewayServer() {
	if suite.gatewayCmd != nil && suite.gatewayCmd.Process != nil {
		suite.gatewayCmd.Process.Signal(syscall.SIGTERM)
		suite.gatewayCmd.Wait()
		suite.gatewayCmd = nil
	}
}

func (suite *LSPEdgeCasesTestSuite) waitForServerReady(ctx context.Context) {
	suite.Eventually(func() bool {
		err := suite.httpClient.HealthCheck(ctx)
		return err == nil
	}, 20*time.Second, 500*time.Millisecond, "Server should become ready")
}

func (suite *LSPEdgeCasesTestSuite) TestInvalidURIHandling() {
	suite.T().Run("InvalidFileURIs", func(t *testing.T) {
		suite.startGatewayServer()
		defer suite.stopGatewayServer()

		ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
		defer cancel()

		suite.waitForServerReady(ctx)

		invalidURIs := []string{
			"",                              // Empty URI
			"not-a-uri",                     // Not a URI at all
			"http://example.com/file.go",    // HTTP URI instead of file URI
			"file://",                       // Incomplete file URI
			"file:///",                      // Root only file URI
			"file:///nonexistent/path.go",   // Non-existent file
			"file://localhost/missing.py",  // File URI with host
			"file:///\x00invalid.ts",       // URI with null character
			"file:///very/deeply/nested/path/that/does/not/exist/anywhere/file.java", // Very long non-existent path
		}

		position := testutils.Position{Line: 1, Character: 1}

		for _, uri := range invalidURIs {
			suite.T().Run(fmt.Sprintf("URI_%s", strings.ReplaceAll(uri, "/", "_")), func(t *testing.T) {
				// Test Definition
				_, err := suite.httpClient.Definition(ctx, uri, position)
				suite.Error(err, "Definition should fail for invalid URI: %s", uri)

				// Test References
				_, err = suite.httpClient.References(ctx, uri, position, true)
				suite.Error(err, "References should fail for invalid URI: %s", uri)

				// Test Hover
				_, err = suite.httpClient.Hover(ctx, uri, position)
				suite.Error(err, "Hover should fail for invalid URI: %s", uri)

				// Test Document Symbols
				_, err = suite.httpClient.DocumentSymbol(ctx, uri)
				suite.Error(err, "DocumentSymbol should fail for invalid URI: %s", uri)

				// Test Completion
				_, err = suite.httpClient.Completion(ctx, uri, position)
				suite.Error(err, "Completion should fail for invalid URI: %s", uri)
			})
		}
	})
}

func (suite *LSPEdgeCasesTestSuite) TestInvalidPositionHandling() {
	suite.T().Run("InvalidPositions", func(t *testing.T) {
		suite.startGatewayServer()
		defer suite.stopGatewayServer()

		ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
		defer cancel()

		suite.waitForServerReady(ctx)

		// Create a valid test file
		testFile := filepath.Join(suite.tempDir, "test.go")
		testContent := "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n"
		err := os.WriteFile(testFile, []byte(testContent), 0644)
		suite.Require().NoError(err)
		
		validURI := fmt.Sprintf("file://%s", testFile)

		invalidPositions := []testutils.Position{
			{Line: -1, Character: 0},     // Negative line
			{Line: 0, Character: -1},     // Negative character
			{Line: -1, Character: -1},    // Both negative
			{Line: 1000, Character: 0},   // Line way beyond file end
			{Line: 0, Character: 1000},   // Character way beyond line end
			{Line: 1000, Character: 1000}, // Both way beyond bounds
		}

		for i, position := range invalidPositions {
			suite.T().Run(fmt.Sprintf("Position_%d_Line%d_Char%d", i, position.Line, position.Character), func(t *testing.T) {
				// Test Definition
				locations, err := suite.httpClient.Definition(ctx, validURI, position)
				if err == nil {
					suite.Empty(locations, "Definition should return empty for invalid position")
				} else {
					suite.T().Logf("Definition failed for invalid position (expected): %v", err)
				}

				// Test References
				refs, err := suite.httpClient.References(ctx, validURI, position, true)
				if err == nil {
					suite.Empty(refs, "References should return empty for invalid position")
				} else {
					suite.T().Logf("References failed for invalid position (expected): %v", err)
				}

				// Test Hover
				hover, err := suite.httpClient.Hover(ctx, validURI, position)
				if err == nil {
					suite.Nil(hover, "Hover should return nil for invalid position")
				} else {
					suite.T().Logf("Hover failed for invalid position (expected): %v", err)
				}

				// Test Completion
				completion, err := suite.httpClient.Completion(ctx, validURI, position)
				if err == nil {
					if completion != nil {
						suite.Empty(completion.Items, "Completion should return empty items for invalid position")
					}
				} else {
					suite.T().Logf("Completion failed for invalid position (expected): %v", err)
				}
			})
		}
	})
}

func (suite *LSPEdgeCasesTestSuite) TestWorkspaceSymbolEdgeCases() {
	suite.T().Run("WorkspaceSymbolQueries", func(t *testing.T) {
		suite.startGatewayServer()
		defer suite.stopGatewayServer()

		ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
		defer cancel()

		suite.waitForServerReady(ctx)

		edgeCaseQueries := []string{
			"",                    // Empty query
			" ",                   // Whitespace only
			"\t\n",                // Tabs and newlines
			"a",                   // Single character
			"nonexistentsymbol",   // Non-existent symbol
			"*",                   // Wildcard character
			"?",                   // Question mark
			"[",                   // Square bracket
			"(",                   // Parenthesis
			"<>",                  // Angle brackets
			"@#$%",                // Special characters
			"123",                 // Numbers only
			"very_long_symbol_name_that_probably_does_not_exist_anywhere_in_the_codebase", // Very long name
			strings.Repeat("a", 1000), // Extremely long query
		}

		for i, query := range edgeCaseQueries {
			suite.T().Run(fmt.Sprintf("Query_%d_%s", i, strings.ReplaceAll(query, " ", "_SPACE_")), func(t *testing.T) {
				symbols, err := suite.httpClient.WorkspaceSymbol(ctx, query)
				
				if err != nil {
					suite.T().Logf("Workspace symbol query '%s' failed: %v", query, err)
				} else {
					suite.T().Logf("Query '%s' returned %d symbols", query, len(symbols))
					
					// Validate symbol structure if any returned
					for j, symbol := range symbols {
						suite.NotEmpty(symbol.Name, fmt.Sprintf("Symbol %d name should not be empty", j))
						suite.Greater(symbol.Kind, 0, fmt.Sprintf("Symbol %d kind should be valid", j))
						suite.Contains(symbol.Location.URI, "file://", fmt.Sprintf("Symbol %d URI should be valid", j))
					}
				}
			})
		}
	})
}

func (suite *LSPEdgeCasesTestSuite) TestTimeoutScenarios() {
	suite.T().Run("RequestTimeouts", func(t *testing.T) {
		// Create a client with very short timeout
		shortTimeoutConfig := testutils.HttpClientConfig{
			BaseURL:         fmt.Sprintf("http://localhost:%d", suite.gatewayPort),
			Timeout:         50 * time.Millisecond, // Very short timeout
			MaxRetries:      1,
			RetryDelay:      10 * time.Millisecond,
			EnableLogging:   true,
			WorkspaceID:     "timeout-test-workspace",
			ProjectPath:     suite.tempDir,
		}
		shortTimeoutClient := testutils.NewHttpClient(shortTimeoutConfig)
		defer shortTimeoutClient.Close()

		suite.startGatewayServer()
		defer suite.stopGatewayServer()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		suite.waitForServerReady(ctx)

		// Test that very short timeouts cause failures
		testFile := filepath.Join(suite.tempDir, "timeout_test.go")
		testContent := "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n"
		err := os.WriteFile(testFile, []byte(testContent), 0644)
		suite.Require().NoError(err)
		
		uri := fmt.Sprintf("file://%s", testFile)
		position := testutils.Position{Line: 2, Character: 5}

		methods := []string{"Definition", "References", "Hover", "DocumentSymbol", "Completion", "WorkspaceSymbol"}
		
		for _, method := range methods {
			suite.T().Run(fmt.Sprintf("Timeout_%s", method), func(t *testing.T) {
				switch method {
				case "Definition":
					_, err := shortTimeoutClient.Definition(ctx, uri, position)
					suite.Error(err, "Definition should timeout with very short timeout")
					suite.Contains(err.Error(), "timeout", "Error should mention timeout")
				case "References":
					_, err := shortTimeoutClient.References(ctx, uri, position, true)
					suite.Error(err, "References should timeout with very short timeout")
					suite.Contains(err.Error(), "timeout", "Error should mention timeout")
				case "Hover":
					_, err := shortTimeoutClient.Hover(ctx, uri, position)
					suite.Error(err, "Hover should timeout with very short timeout")
					suite.Contains(err.Error(), "timeout", "Error should mention timeout")
				case "DocumentSymbol":
					_, err := shortTimeoutClient.DocumentSymbol(ctx, uri)
					suite.Error(err, "DocumentSymbol should timeout with very short timeout")
					suite.Contains(err.Error(), "timeout", "Error should mention timeout")
				case "Completion":
					_, err := shortTimeoutClient.Completion(ctx, uri, position)
					suite.Error(err, "Completion should timeout with very short timeout")
					suite.Contains(err.Error(), "timeout", "Error should mention timeout")
				case "WorkspaceSymbol":
					_, err := shortTimeoutClient.WorkspaceSymbol(ctx, "test")
					suite.Error(err, "WorkspaceSymbol should timeout with very short timeout")
					suite.Contains(err.Error(), "timeout", "Error should mention timeout")
				}
			})
		}

		// Check metrics recorded timeout errors
		metrics := shortTimeoutClient.GetMetrics()
		suite.Greater(metrics.TimeoutErrors, 0, "Should have recorded timeout errors")
		suite.Greater(metrics.FailedRequests, 0, "Should have recorded failed requests")
	})
}

func (suite *LSPEdgeCasesTestSuite) TestContextCancellation() {
	suite.T().Run("ContextCancellation", func(t *testing.T) {
		suite.startGatewayServer()
		defer suite.stopGatewayServer()

		parentCtx, parentCancel := context.WithTimeout(context.Background(), suite.testTimeout)
		defer parentCancel()

		suite.waitForServerReady(parentCtx)

		// Create a valid test file
		testFile := filepath.Join(suite.tempDir, "cancel_test.go")
		testContent := "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n"
		err := os.WriteFile(testFile, []byte(testContent), 0644)
		suite.Require().NoError(err)
		
		uri := fmt.Sprintf("file://%s", testFile)
		position := testutils.Position{Line: 2, Character: 5}

		// Test immediate cancellation
		immediateCtx, immediateCancel := context.WithCancel(parentCtx)
		immediateCancel() // Cancel immediately

		_, err = suite.httpClient.Definition(immediateCtx, uri, position)
		suite.Error(err, "Should fail with cancelled context")
		suite.Contains(err.Error(), "context canceled", "Error should mention context cancellation")

		// Test cancellation during request
		delayedCtx, delayedCancel := context.WithCancel(parentCtx)
		
		go func() {
			time.Sleep(10 * time.Millisecond) // Cancel after short delay
			delayedCancel()
		}()

		_, err = suite.httpClient.Definition(delayedCtx, uri, position)
		if err != nil {
			suite.T().Logf("Request cancelled as expected: %v", err)
		}
	})
}

func (suite *LSPEdgeCasesTestSuite) TestLargeResponseHandling() {
	suite.T().Run("LargeResponses", func(t *testing.T) {
		// Create client with very small response size limit
		smallResponseConfig := testutils.HttpClientConfig{
			BaseURL:         fmt.Sprintf("http://localhost:%d", suite.gatewayPort),
			Timeout:         10 * time.Second,
			MaxRetries:      2,
			RetryDelay:      100 * time.Millisecond,
			MaxResponseSize: 100, // Very small limit
			EnableLogging:   true,
			WorkspaceID:     "large-response-test",
			ProjectPath:     suite.tempDir,
		}
		smallResponseClient := testutils.NewHttpClient(smallResponseConfig)
		defer smallResponseClient.Close()

		suite.startGatewayServer()
		defer suite.stopGatewayServer()

		ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
		defer cancel()

		suite.waitForServerReady(ctx)

		// Create a large test file that might generate large responses
		largeContent := "package main\n\n"
		for i := 0; i < 100; i++ {
			largeContent += fmt.Sprintf("func Function%d() {\n\t// Function %d implementation\n}\n\n", i, i)
		}

		testFile := filepath.Join(suite.tempDir, "large_file.go")
		err := os.WriteFile(testFile, []byte(largeContent), 0644)
		suite.Require().NoError(err)
		
		uri := fmt.Sprintf("file://%s", testFile)

		// Test methods that might return large responses
		_, err = smallResponseClient.DocumentSymbol(ctx, uri)
		if err != nil {
			suite.T().Logf("DocumentSymbol failed with small response limit (expected): %v", err)
		}

		_, err = smallResponseClient.WorkspaceSymbol(ctx, "Function")
		if err != nil {
			suite.T().Logf("WorkspaceSymbol failed with small response limit (expected): %v", err)
		}
	})
}

func (suite *LSPEdgeCasesTestSuite) TestMalformedFileContent() {
	suite.T().Run("MalformedFiles", func(t *testing.T) {
		suite.startGatewayServer()
		defer suite.stopGatewayServer()

		ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
		defer cancel()

		suite.waitForServerReady(ctx)

		malformedFiles := map[string]string{
			"empty.go":            "",                                    // Empty file
			"binary.go":           "\x00\x01\x02\x03\x04\x05",          // Binary content
			"incomplete.go":       "package main\n\nfunc incomplete(",   // Incomplete syntax
			"unicode.go":          "package main\n\n// 测试 🚀 ñoño",       // Unicode characters
			"very_long_line.go":   fmt.Sprintf("package main\n\n// %s", strings.Repeat("a", 10000)), // Very long line
			"many_newlines.go":    fmt.Sprintf("package main%s", strings.Repeat("\n", 1000)),         // Many newlines
		}

		position := testutils.Position{Line: 1, Character: 1}

		for filename, content := range malformedFiles {
			suite.T().Run(fmt.Sprintf("File_%s", filename), func(t *testing.T) {
				testFile := filepath.Join(suite.tempDir, filename)
				err := os.WriteFile(testFile, []byte(content), 0644)
				suite.Require().NoError(err)
				
				uri := fmt.Sprintf("file://%s", testFile)

				// Test each LSP method with malformed content
				methods := []string{"Definition", "References", "Hover", "DocumentSymbol", "Completion"}
				
				for _, method := range methods {
					switch method {
					case "Definition":
						locations, err := suite.httpClient.Definition(ctx, uri, position)
						if err == nil {
							suite.T().Logf("Definition for %s returned %d locations", filename, len(locations))
						} else {
							suite.T().Logf("Definition for %s failed: %v", filename, err)
						}
					case "References":
						refs, err := suite.httpClient.References(ctx, uri, position, true)
						if err == nil {
							suite.T().Logf("References for %s returned %d references", filename, len(refs))
						} else {
							suite.T().Logf("References for %s failed: %v", filename, err)
						}
					case "Hover":
						hover, err := suite.httpClient.Hover(ctx, uri, position)
						if err == nil {
							if hover != nil {
								suite.T().Logf("Hover for %s returned content", filename)
							} else {
								suite.T().Logf("Hover for %s returned nil", filename)
							}
						} else {
							suite.T().Logf("Hover for %s failed: %v", filename, err)
						}
					case "DocumentSymbol":
						symbols, err := suite.httpClient.DocumentSymbol(ctx, uri)
						if err == nil {
							suite.T().Logf("DocumentSymbol for %s returned %d symbols", filename, len(symbols))
						} else {
							suite.T().Logf("DocumentSymbol for %s failed: %v", filename, err)
						}
					case "Completion":
						completion, err := suite.httpClient.Completion(ctx, uri, position)
						if err == nil {
							if completion != nil {
								suite.T().Logf("Completion for %s returned %d items", filename, len(completion.Items))
							} else {
								suite.T().Logf("Completion for %s returned nil", filename)
							}
						} else {
							suite.T().Logf("Completion for %s failed: %v", filename, err)
						}
					}
				}
			})
		}
	})
}

func (suite *LSPEdgeCasesTestSuite) TestConcurrentEdgeCases() {
	suite.T().Run("ConcurrentInvalidRequests", func(t *testing.T) {
		suite.startGatewayServer()
		defer suite.stopGatewayServer()

		ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
		defer cancel()

		suite.waitForServerReady(ctx)

		numConcurrent := 20
		done := make(chan bool, numConcurrent)

		// Mix of invalid requests running concurrently
		for i := 0; i < numConcurrent; i++ {
			go func(id int) {
				defer func() { done <- true }()
				
				invalidURI := fmt.Sprintf("file:///invalid%d.go", id)
				invalidPosition := testutils.Position{Line: -id, Character: -id}
				
				// Try different invalid operations
				switch id % 6 {
				case 0:
					suite.httpClient.Definition(ctx, invalidURI, invalidPosition)
				case 1:
					suite.httpClient.References(ctx, invalidURI, invalidPosition, true)
				case 2:
					suite.httpClient.Hover(ctx, invalidURI, invalidPosition)
				case 3:
					suite.httpClient.DocumentSymbol(ctx, invalidURI)
				case 4:
					suite.httpClient.Completion(ctx, invalidURI, invalidPosition)
				case 5:
					suite.httpClient.WorkspaceSymbol(ctx, fmt.Sprintf("invalid%d", id))
				}
			}(i)
		}

		completed := 0
		for completed < numConcurrent {
			select {
			case <-done:
				completed++
			case <-ctx.Done():
				suite.Fail("Timeout waiting for concurrent invalid requests")
				return
			}
		}

		suite.T().Logf("Completed %d concurrent invalid requests", completed)
		
		// Verify metrics recorded the errors appropriately
		metrics := suite.httpClient.GetMetrics()
		suite.T().Logf("Metrics after concurrent invalid requests: Total: %d, Failed: %d", 
			metrics.TotalRequests, metrics.FailedRequests)
	})
}

func TestLSPEdgeCasesTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping LSP edge cases tests in short mode")
	}
	suite.Run(t, new(LSPEdgeCasesTestSuite))
}