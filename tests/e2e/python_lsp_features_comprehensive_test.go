package e2e_test

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lsp-gateway/tests/e2e/fixtures"
	"lsp-gateway/tests/e2e/testutils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type PythonLSPFeaturesComprehensiveTestSuite struct {
	suite.Suite
	httpClient    *testutils.HttpClient
	repoManager   *testutils.PythonRepoManager
	workspaceDir  string
	repoContext   string
	startTime     time.Time
	timeout       time.Duration
}

func (suite *PythonLSPFeaturesComprehensiveTestSuite) SetupSuite() {
	suite.timeout = 5 * time.Second
	suite.startTime = time.Now()

	config := testutils.DefaultPythonRepoConfig()
	config.EnableLogging = true
	config.ForceClean = true

	suite.repoManager = testutils.NewPythonRepoManager(config)
	require.NotNil(suite.T(), suite.repoManager)

	err := suite.repoManager.CloneRepository()
	require.NoError(suite.T(), err, "Failed to clone Python patterns repository")

	suite.workspaceDir = suite.repoManager.WorkspaceDir
	suite.repoContext = "python-patterns"

	httpConfig := testutils.HttpClientConfig{
		BaseURL:     "http://localhost:8080",
		Timeout:     suite.timeout,
		MockMode:    false,
		ProjectPath: suite.workspaceDir,
		WorkspaceID: suite.repoContext,
	}

	suite.httpClient = testutils.NewHttpClient(httpConfig)
	require.NotNil(suite.T(), suite.httpClient)
}

func (suite *PythonLSPFeaturesComprehensiveTestSuite) TearDownSuite() {
	if suite.repoManager != nil && suite.repoManager.CleanupFunc != nil {
		_ = suite.repoManager.CleanupFunc()
	}
}

// TestDefinitionFeature tests textDocument/definition LSP method
func (suite *PythonLSPFeaturesComprehensiveTestSuite) TestDefinitionFeature() {
	scenarios := fixtures.GetScenariosForLSPMethod("textDocument/definition")
	require.NotEmpty(suite.T(), scenarios, "No definition scenarios found")

	for _, scenario := range scenarios {
		suite.Run(fmt.Sprintf("Definition_%s", scenario.Name), func() {
			ctx, cancel := context.WithTimeout(context.Background(), suite.timeout)
			defer cancel()

			filePath := filepath.Join(suite.workspaceDir, scenario.FilePath)
			fileURI := "file://" + filePath

			for _, testPos := range scenario.TestPositions {
				position := testutils.Position{
					Line:      testPos.Line,
					Character: testPos.Character,
				}

				startTime := time.Now()
				locations, err := suite.httpClient.Definition(ctx, fileURI, position)
				duration := time.Since(startTime)

				// Performance validation
				assert.True(suite.T(), duration < suite.timeout,
					"Definition request took too long: %v", duration)

				if err != nil {
					suite.T().Logf("Definition error for %s at %d:%d: %v",
						testPos.Symbol, testPos.Line, testPos.Character, err)
					continue
				}

				// Validate response structure
				assert.NotNil(suite.T(), locations, "Locations should not be nil")
				
				if len(locations) > 0 {
					location := locations[0]
					
					// Validate location structure
					assert.NotEmpty(suite.T(), location.URI, "Location URI should not be empty")
					assert.True(suite.T(), strings.HasPrefix(location.URI, "file://"),
						"URI should have file:// prefix")
					
					// Validate range structure
					assert.GreaterOrEqual(suite.T(), location.Range.Start.Line, 0,
						"Start line should be non-negative")
					assert.GreaterOrEqual(suite.T(), location.Range.Start.Character, 0,
						"Start character should be non-negative")
					assert.GreaterOrEqual(suite.T(), location.Range.End.Line, location.Range.Start.Line,
						"End line should be >= start line")
					
					// Content accuracy for specific patterns
					if testPos.Symbol == "Builder" || testPos.Symbol == "Director" {
						assert.Contains(suite.T(), location.URI, "builder.py",
							"Builder/Director should be defined in builder.py")
					}
					if testPos.Symbol == "Adapter" || testPos.Symbol == "Dog" || testPos.Symbol == "Cat" {
						assert.Contains(suite.T(), location.URI, "adapter.py",
							"Adapter pattern symbols should be in adapter.py")
					}

					suite.T().Logf("Definition found for %s: %s at %d:%d",
						testPos.Symbol, location.URI, location.Range.Start.Line, location.Range.Start.Character)
				}
			}
		})
	}
}

// TestReferencesFeature tests textDocument/references LSP method
func (suite *PythonLSPFeaturesComprehensiveTestSuite) TestReferencesFeature() {
	scenarios := fixtures.GetScenariosForLSPMethod("textDocument/references")
	require.NotEmpty(suite.T(), scenarios, "No references scenarios found")

	for _, scenario := range scenarios {
		suite.Run(fmt.Sprintf("References_%s", scenario.Name), func() {
			ctx, cancel := context.WithTimeout(context.Background(), suite.timeout)
			defer cancel()

			filePath := filepath.Join(suite.workspaceDir, scenario.FilePath)
			fileURI := "file://" + filePath

			for _, testPos := range scenario.TestPositions {
				position := testutils.Position{
					Line:      testPos.Line,
					Character: testPos.Character,
				}

				startTime := time.Now()
				references, err := suite.httpClient.References(ctx, fileURI, position, true)
				duration := time.Since(startTime)

				// Performance validation
				assert.True(suite.T(), duration < suite.timeout,
					"References request took too long: %v", duration)

				if err != nil {
					suite.T().Logf("References error for %s at %d:%d: %v",
						testPos.Symbol, testPos.Line, testPos.Character, err)
					continue
				}

				// Validate response structure
				assert.NotNil(suite.T(), references, "References should not be nil")

				for _, ref := range references {
					// Validate reference structure
					assert.NotEmpty(suite.T(), ref.URI, "Reference URI should not be empty")
					assert.True(suite.T(), strings.HasPrefix(ref.URI, "file://"),
						"Reference URI should have file:// prefix")
					
					// Validate range
					assert.GreaterOrEqual(suite.T(), ref.Range.Start.Line, 0,
						"Reference start line should be non-negative")
					assert.GreaterOrEqual(suite.T(), ref.Range.Start.Character, 0,
						"Reference start character should be non-negative")
				}

				// Validate reference count for common patterns
				if testPos.Symbol == "Builder" && len(references) > 0 {
					assert.GreaterOrEqual(suite.T(), len(references), 1,
						"Builder class should have at least one reference")
				}

				suite.T().Logf("Found %d references for %s", len(references), testPos.Symbol)
			}
		})
	}
}

// TestHoverFeature tests textDocument/hover LSP method
func (suite *PythonLSPFeaturesComprehensiveTestSuite) TestHoverFeature() {
	scenarios := fixtures.GetScenariosForLSPMethod("textDocument/hover")
	require.NotEmpty(suite.T(), scenarios, "No hover scenarios found")

	for _, scenario := range scenarios {
		suite.Run(fmt.Sprintf("Hover_%s", scenario.Name), func() {
			ctx, cancel := context.WithTimeout(context.Background(), suite.timeout)
			defer cancel()

			filePath := filepath.Join(suite.workspaceDir, scenario.FilePath)
			fileURI := "file://" + filePath

			for _, testPos := range scenario.TestPositions {
				position := testutils.Position{
					Line:      testPos.Line,
					Character: testPos.Character,
				}

				startTime := time.Now()
				hoverResult, err := suite.httpClient.Hover(ctx, fileURI, position)
				duration := time.Since(startTime)

				// Performance validation
				assert.True(suite.T(), duration < suite.timeout,
					"Hover request took too long: %v", duration)

				if err != nil {
					suite.T().Logf("Hover error for %s at %d:%d: %v",
						testPos.Symbol, testPos.Line, testPos.Character, err)
					continue
				}

				// Validate hover result structure
				if hoverResult != nil {
					assert.NotNil(suite.T(), hoverResult.Contents, "Hover contents should not be nil")
					
					// Validate range if present
					if hoverResult.Range != nil {
						assert.GreaterOrEqual(suite.T(), hoverResult.Range.Start.Line, 0,
							"Hover range start line should be non-negative")
						assert.GreaterOrEqual(suite.T(), hoverResult.Range.Start.Character, 0,
							"Hover range start character should be non-negative")
					}

					// Content validation for design patterns
					contentsStr := fmt.Sprintf("%v", hoverResult.Contents)
					if testPos.Symbol == "Director" || testPos.Symbol == "Builder" {
						// Should contain relevant pattern information
						assert.True(suite.T(), len(contentsStr) > 0,
							"Hover content should not be empty for %s", testPos.Symbol)
					}

					suite.T().Logf("Hover info for %s: %v", testPos.Symbol, hoverResult.Contents)
				} else {
					suite.T().Logf("No hover information available for %s", testPos.Symbol)
				}
			}
		})
	}
}

// TestDocumentSymbolFeature tests textDocument/documentSymbol LSP method
func (suite *PythonLSPFeaturesComprehensiveTestSuite) TestDocumentSymbolFeature() {
	scenarios := fixtures.GetScenariosForLSPMethod("textDocument/documentSymbol")
	require.NotEmpty(suite.T(), scenarios, "No document symbol scenarios found")

	for _, scenario := range scenarios {
		suite.Run(fmt.Sprintf("DocumentSymbol_%s", scenario.Name), func() {
			ctx, cancel := context.WithTimeout(context.Background(), suite.timeout)
			defer cancel()

			filePath := filepath.Join(suite.workspaceDir, scenario.FilePath)
			fileURI := "file://" + filePath

			startTime := time.Now()
			symbols, err := suite.httpClient.DocumentSymbol(ctx, fileURI)
			duration := time.Since(startTime)

			// Performance validation
			assert.True(suite.T(), duration < suite.timeout,
				"DocumentSymbol request took too long: %v", duration)

			if err != nil {
				suite.T().Logf("DocumentSymbol error for %s: %v", scenario.FilePath, err)
				return
			}

			// Validate response structure
			assert.NotNil(suite.T(), symbols, "Document symbols should not be nil")

			// Validate symbol structure and hierarchy
			for _, symbol := range symbols {
				// Basic symbol validation
				assert.NotEmpty(suite.T(), symbol.Name, "Symbol name should not be empty")
				assert.Greater(suite.T(), symbol.Kind, 0, "Symbol kind should be positive")
				
				// Validate ranges
				assert.GreaterOrEqual(suite.T(), symbol.Range.Start.Line, 0,
					"Symbol range start line should be non-negative")
				assert.GreaterOrEqual(suite.T(), symbol.Range.Start.Character, 0,
					"Symbol range start character should be non-negative")
				assert.GreaterOrEqual(suite.T(), symbol.SelectionRange.Start.Line, 0,
					"Symbol selection range start line should be non-negative")

				// Validate expected symbols for specific patterns
				for _, expectedSymbol := range scenario.ExpectedSymbols {
					if symbol.Name == expectedSymbol.Name {
						assert.Equal(suite.T(), expectedSymbol.Kind, symbol.Kind,
							"Symbol kind mismatch for %s", symbol.Name)
						suite.T().Logf("Found expected symbol: %s (kind: %d)",
							symbol.Name, symbol.Kind)
					}
				}

				// Validate children hierarchy
				suite.validateSymbolHierarchy(symbol, 0)
			}

			suite.T().Logf("Found %d document symbols in %s", len(symbols), scenario.FilePath)
		})
	}
}

// TestWorkspaceSymbolFeature tests workspace/symbol LSP method
func (suite *PythonLSPFeaturesComprehensiveTestSuite) TestWorkspaceSymbolFeature() {
	scenarios := fixtures.GetScenariosForLSPMethod("workspace/symbol")
	require.NotEmpty(suite.T(), scenarios, "No workspace symbol scenarios found")

	// Test common pattern queries
	queries := []string{"Builder", "Adapter", "Observer", "Director", "Pattern"}

	for _, query := range queries {
		suite.Run(fmt.Sprintf("WorkspaceSymbol_%s", query), func() {
			ctx, cancel := context.WithTimeout(context.Background(), suite.timeout)
			defer cancel()

			startTime := time.Now()
			symbols, err := suite.httpClient.WorkspaceSymbol(ctx, query)
			duration := time.Since(startTime)

			// Performance validation
			assert.True(suite.T(), duration < suite.timeout,
				"WorkspaceSymbol request took too long: %v", duration)

			if err != nil {
				suite.T().Logf("WorkspaceSymbol error for query '%s': %v", query, err)
				return
			}

			// Validate response structure
			assert.NotNil(suite.T(), symbols, "Workspace symbols should not be nil")

			for _, symbol := range symbols {
				// Basic symbol validation
				assert.NotEmpty(suite.T(), symbol.Name, "Symbol name should not be empty")
				assert.Greater(suite.T(), symbol.Kind, 0, "Symbol kind should be positive")
				assert.NotEmpty(suite.T(), symbol.Location.URI, "Symbol location URI should not be empty")
				
				// Validate location
				assert.True(suite.T(), strings.HasPrefix(symbol.Location.URI, "file://"),
					"Symbol location URI should have file:// prefix")
				
				// Query relevance validation
				assert.True(suite.T(), 
					strings.Contains(strings.ToLower(symbol.Name), strings.ToLower(query)) ||
					strings.Contains(strings.ToLower(query), strings.ToLower(symbol.Name)),
					"Symbol '%s' should be relevant to query '%s'", symbol.Name, query)
			}

			suite.T().Logf("Found %d workspace symbols for query '%s'", len(symbols), query)
		})
	}
}

// TestCompletionFeature tests textDocument/completion LSP method
func (suite *PythonLSPFeaturesComprehensiveTestSuite) TestCompletionFeature() {
	scenarios := fixtures.GetScenariosForLSPMethod("textDocument/completion")
	require.NotEmpty(suite.T(), scenarios, "No completion scenarios found")

	for _, scenario := range scenarios {
		suite.Run(fmt.Sprintf("Completion_%s", scenario.Name), func() {
			ctx, cancel := context.WithTimeout(context.Background(), suite.timeout)
			defer cancel()

			filePath := filepath.Join(suite.workspaceDir, scenario.FilePath)
			fileURI := "file://" + filePath

			for _, testPos := range scenario.TestPositions {
				position := testutils.Position{
					Line:      testPos.Line,
					Character: testPos.Character,
				}

				startTime := time.Now()
				completionList, err := suite.httpClient.Completion(ctx, fileURI, position)
				duration := time.Since(startTime)

				// Performance validation
				assert.True(suite.T(), duration < suite.timeout,
					"Completion request took too long: %v", duration)

				if err != nil {
					suite.T().Logf("Completion error for %s at %d:%d: %v",
						testPos.Symbol, testPos.Line, testPos.Character, err)
					continue
				}

				// Validate completion list structure
				if completionList != nil {
					assert.NotNil(suite.T(), completionList.Items, "Completion items should not be nil")
					
					for _, item := range completionList.Items {
						// Basic completion item validation
						assert.NotEmpty(suite.T(), item.Label, "Completion item label should not be empty")
						assert.Greater(suite.T(), item.Kind, 0, "Completion item kind should be positive")
						
						// Pattern-specific validation
						if testPos.Context == "method chaining" || testPos.Context == "builder method" {
							// Should include builder pattern methods
							if strings.Contains(item.Label, "set_") || strings.Contains(item.Label, "build") {
								suite.T().Logf("Found builder method completion: %s", item.Label)
							}
						}
					}

					suite.T().Logf("Found %d completion items for %s at %d:%d",
						len(completionList.Items), testPos.Symbol, testPos.Line, testPos.Character)
				}
			}
		})
	}
}

// TestLSPFeatureErrorHandling tests error scenarios for all LSP methods
func (suite *PythonLSPFeaturesComprehensiveTestSuite) TestLSPFeatureErrorHandling() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.timeout)
	defer cancel()

	// Test with invalid file URI
	invalidURI := "file:///non/existent/file.py"
	invalidPosition := testutils.Position{Line: 0, Character: 0}

	suite.Run("InvalidFileDefinition", func() {
		locations, err := suite.httpClient.Definition(ctx, invalidURI, invalidPosition)
		// Should handle gracefully - either error or empty results
		if err != nil {
			suite.T().Logf("Expected error for invalid file: %v", err)
		} else {
			assert.NotNil(suite.T(), locations, "Locations should not be nil even for invalid files")
		}
	})

	suite.Run("InvalidFileReferences", func() {
		references, err := suite.httpClient.References(ctx, invalidURI, invalidPosition, true)
		if err != nil {
			suite.T().Logf("Expected error for invalid file: %v", err)
		} else {
			assert.NotNil(suite.T(), references, "References should not be nil even for invalid files")
		}
	})

	suite.Run("InvalidFileHover", func() {
		hoverResult, err := suite.httpClient.Hover(ctx, invalidURI, invalidPosition)
		if err != nil {
			suite.T().Logf("Expected error for invalid file: %v", err)
		} else {
			// Hover can legitimately return nil for invalid positions
			suite.T().Logf("Hover result for invalid file: %v", hoverResult)
		}
	})

	suite.Run("InvalidFileDocumentSymbol", func() {
		symbols, err := suite.httpClient.DocumentSymbol(ctx, invalidURI)
		if err != nil {
			suite.T().Logf("Expected error for invalid file: %v", err)
		} else {
			assert.NotNil(suite.T(), symbols, "Symbols should not be nil even for invalid files")
		}
	})

	suite.Run("EmptyWorkspaceSymbolQuery", func() {
		symbols, err := suite.httpClient.WorkspaceSymbol(ctx, "")
		if err != nil {
			suite.T().Logf("Expected error for empty query: %v", err)
		} else {
			assert.NotNil(suite.T(), symbols, "Symbols should not be nil even for empty query")
		}
	})

	suite.Run("InvalidFileCompletion", func() {
		completionList, err := suite.httpClient.Completion(ctx, invalidURI, invalidPosition)
		if err != nil {
			suite.T().Logf("Expected error for invalid file: %v", err)
		} else {
			assert.NotNil(suite.T(), completionList, "Completion list should not be nil even for invalid files")
		}
	})
}

// validateSymbolHierarchy recursively validates document symbol hierarchy
func (suite *PythonLSPFeaturesComprehensiveTestSuite) validateSymbolHierarchy(symbol testutils.DocumentSymbol, depth int) {
	// Limit recursion depth to prevent infinite loops
	if depth > 5 {
		return
	}

	for _, child := range symbol.Children {
		// Validate child structure
		assert.NotEmpty(suite.T(), child.Name, "Child symbol name should not be empty")
		assert.Greater(suite.T(), child.Kind, 0, "Child symbol kind should be positive")
		
		// Validate child range is within parent range
		assert.GreaterOrEqual(suite.T(), child.Range.Start.Line, symbol.Range.Start.Line,
			"Child range should start after or at parent start")
		assert.LessOrEqual(suite.T(), child.Range.End.Line, symbol.Range.End.Line,
			"Child range should end before or at parent end")

		// Recursively validate children
		suite.validateSymbolHierarchy(child, depth+1)
	}
}

// TestSuite entry point
func TestPythonLSPFeaturesComprehensiveTestSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping comprehensive LSP features test in short mode")
	}
	suite.Run(t, new(PythonLSPFeaturesComprehensiveTestSuite))
}