package scip

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"lsp-gateway/internal/indexing"
)

// TestLSPMethodIntegration tests all LSP methods with SCIP backend integration
func TestLSPMethodIntegration(t *testing.T) {
	suite, err := NewSCIPTestSuite(DefaultSCIPTestConfig())
	if err != nil {
		t.Fatalf("Failed to create SCIP test suite: %v", err)
	}
	defer suite.Cleanup()

	t.Run("DefinitionIntegration", func(t *testing.T) {
		testDefinitionIntegration(t, suite)
	})

	t.Run("ReferencesIntegration", func(t *testing.T) {
		testReferencesIntegration(t, suite)
	})

	t.Run("HoverIntegration", func(t *testing.T) {
		testHoverIntegration(t, suite)
	})

	t.Run("DocumentSymbolIntegration", func(t *testing.T) {
		testDocumentSymbolIntegration(t, suite)
	})

	t.Run("WorkspaceSymbolIntegration", func(t *testing.T) {
		testWorkspaceSymbolIntegration(t, suite)
	})

	// Validate overall performance targets
	suite.ValidatePerformanceTargets(t)
}

// testDefinitionIntegration tests textDocument/definition integration
func testDefinitionIntegration(t *testing.T, suite *SCIPTestSuite) {
	testCases := []struct {
		name     string
		language string
		request  *LSPTestRequest
		validate func(t *testing.T, response *LSPTestResponse)
	}{
		{
			name:     "Go function definition",
			language: "go",
			request: &LSPTestRequest{
				Method: "textDocument/definition",
				URI:    "file://main.go",
				Position: &indexing.LSPPosition{
					Line:      65, // NewUserService call in main()
					Character: 15,
				},
				Expected: &indexing.LSPLocation{
					URI: "file://main.go",
					Range: indexing.LSPRange{
						Start: indexing.LSPPosition{Line: 27, Character: 5},
						End:   indexing.LSPPosition{Line: 27, Character: 18},
					},
				},
			},
			validate: validateDefinitionResponse,
		},
		{
			name:     "Go struct field definition",
			language: "go",
			request: &LSPTestRequest{
				Method: "textDocument/definition",
				URI:    "file://main.go",
				Position: &indexing.LSPPosition{
					Line:      34, // user.ID access in GetUser
					Character: 8,
				},
				Expected: &indexing.LSPLocation{
					URI: "file://main.go",
					Range: indexing.LSPRange{
						Start: indexing.LSPPosition{Line: 9, Character: 1},
						End:   indexing.LSPPosition{Line: 9, Character: 3},
					},
				},
			},
			validate: validateDefinitionResponse,
		},
		{
			name:     "TypeScript interface definition",
			language: "typescript",
			request: &LSPTestRequest{
				Method: "textDocument/definition",
				URI:    "file://index.ts",
				Position: &indexing.LSPPosition{
					Line:      47, // UserRepository parameter
					Character: 25,
				},
				Expected: &indexing.LSPLocation{
					URI: "file://index.ts",
					Range: indexing.LSPRange{
						Start: indexing.LSPPosition{Line: 8, Character: 17},
						End:   indexing.LSPPosition{Line: 8, Character: 31},
					},
				},
			},
			validate: validateDefinitionResponse,
		},
		{
			name:     "Python class method definition",
			language: "python",
			request: &LSPTestRequest{
				Method: "textDocument/definition",
				URI:    "file://user.py",
				Position: &indexing.LSPPosition{
					Line:      200, // service.create_new_user call in main()
					Character: 20,
				},
				Expected: &indexing.LSPLocation{
					URI: "file://user.py",
					Range: indexing.LSPRange{
						Start: indexing.LSPPosition{Line: 122, Character: 8},
						End:   indexing.LSPPosition{Line: 122, Character: 23},
					},
				},
			},
			validate: validateDefinitionResponse,
		},
		{
			name:     "Java method definition",
			language: "java",
			request: &LSPTestRequest{
				Method: "textDocument/definition",
				URI:    "file://src/main/java/com/example/user/UserService.java",
				Position: &indexing.LSPPosition{
					Line:      15, // userRepository field access
					Character: 12,
				},
				Expected: &indexing.LSPLocation{
					URI: "file://src/main/java/com/example/user/UserRepository.java",
					Range: indexing.LSPRange{
						Start: indexing.LSPPosition{Line: 8, Character: 17},
						End:   indexing.LSPPosition{Line: 8, Character: 31},
					},
				},
			},
			validate: validateDefinitionResponse,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			startTime := time.Now()

			// Execute LSP definition request through SCIP mapper
			params, err := suite.scipMapper.ParseLSPParams(tc.request.Method, map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": tc.request.URI,
				},
				"position": map[string]interface{}{
					"line":      tc.request.Position.Line,
					"character": tc.request.Position.Character,
				},
			})
			if err != nil {
				t.Fatalf("Failed to parse LSP params: %v", err)
				return
			}

			response, err := suite.scipMapper.MapDefinition(params)
			if err != nil {
				t.Errorf("Definition request failed: %v", err)
				suite.RecordTestResult(tc.name, false, time.Since(startTime))
				return
			}

			// Validate response
			testResponse := &LSPTestResponse{
				Method:       tc.request.Method,
				Response:     response,
				Found:        response != nil && string(response) != "[]",
				QueryTime:    time.Since(startTime),
				ValidatedFields: []string{"uri", "range"},
			}

			tc.validate(t, testResponse)

			// Record metrics
			suite.RecordQueryMetrics(testResponse.QueryTime, testResponse.CacheHit, !t.Failed())
			suite.RecordTestResult(tc.name, !t.Failed(), time.Since(startTime))
		})
	}
}

// testReferencesIntegration tests textDocument/references integration
func testReferencesIntegration(t *testing.T, suite *SCIPTestSuite) {
	testCases := []struct {
		name            string
		language        string
		request         *LSPTestRequest
		expectedMinRefs int
		validate        func(t *testing.T, response *LSPTestResponse, minRefs int)
	}{
		{
			name:     "Go function references",
			language: "go",
			request: &LSPTestRequest{
				Method: "textDocument/references",
				URI:    "file://main.go",
				Position: &indexing.LSPPosition{
					Line:      27, // NewUserService function definition
					Character: 10,
				},
			},
			expectedMinRefs: 2, // Definition + at least 1 call
			validate:        validateReferencesResponse,
		},
		{
			name:     "Go struct field references",
			language: "go",
			request: &LSPTestRequest{
				Method: "textDocument/references",
				URI:    "file://main.go",
				Position: &indexing.LSPPosition{
					Line:      9, // User.ID field definition
					Character: 2,
				},
			},
			expectedMinRefs: 3, // Multiple ID field accesses
			validate:        validateReferencesResponse,
		},
		{
			name:     "TypeScript interface references",
			language: "typescript",
			request: &LSPTestRequest{
				Method: "textDocument/references",
				URI:    "file://index.ts",
				Position: &indexing.LSPPosition{
					Line:      1, // User interface definition
					Character: 20,
				},
			},
			expectedMinRefs: 5, // Multiple User type usages
			validate:        validateReferencesResponse,
		},
		{
			name:     "Python class references",
			language: "python",
			request: &LSPTestRequest{
				Method: "textDocument/references",
				URI:    "file://user.py",
				Position: &indexing.LSPPosition{
					Line:      10, // User class definition
					Character: 8,
				},
			},
			expectedMinRefs: 10, // Many User class usages
			validate:        validateReferencesResponse,
		},
		{
			name:     "Java class references",
			language: "java",
			request: &LSPTestRequest{
				Method: "textDocument/references",
				URI:    "file://src/main/java/com/example/user/User.java",
				Position: &indexing.LSPPosition{
					Line:      8, // User class definition
					Character: 15,
				},
			},
			expectedMinRefs: 8, // Multiple User class usages
			validate:        validateReferencesResponse,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			startTime := time.Now()

			// Execute references request
			params, err := suite.scipMapper.ParseLSPParams(tc.request.Method, map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": tc.request.URI,
				},
				"position": map[string]interface{}{
					"line":      tc.request.Position.Line,
					"character": tc.request.Position.Character,
				},
				"context": map[string]interface{}{
					"includeDeclaration": false,
				},
			})
			if err != nil {
				t.Fatalf("Failed to parse LSP params: %v", err)
				return
			}

			response, err := suite.scipMapper.MapReferences(params)
			if err != nil {
				t.Errorf("References request failed: %v", err)
				suite.RecordTestResult(tc.name, false, time.Since(startTime))
				return
			}

			testResponse := &LSPTestResponse{
				Method:       tc.request.Method,
				Response:     response,
				Found:        response != nil && string(response) != "[]",
				QueryTime:    time.Since(startTime),
				ValidatedFields: []string{"references", "count"},
			}

			tc.validate(t, testResponse, tc.expectedMinRefs)

			suite.RecordQueryMetrics(testResponse.QueryTime, testResponse.CacheHit, !t.Failed())
			suite.RecordTestResult(tc.name, !t.Failed(), time.Since(startTime))
		})
	}
}

// testHoverIntegration tests textDocument/hover integration
func testHoverIntegration(t *testing.T, suite *SCIPTestSuite) {
	testCases := []struct {
		name        string
		language    string
		request     *LSPTestRequest
		expectHover bool
		validate    func(t *testing.T, response *LSPTestResponse)
	}{
		{
			name:     "Go function hover",
			language: "go",
			request: &LSPTestRequest{
				Method: "textDocument/hover",
				URI:    "file://main.go",
				Position: &indexing.LSPPosition{
					Line:      27, // NewUserService function
					Character: 10,
				},
			},
			expectHover: true,
			validate:    validateHoverResponse,
		},
		{
			name:     "Go struct hover",
			language: "go",
			request: &LSPTestRequest{
				Method: "textDocument/hover",
				URI:    "file://main.go",
				Position: &indexing.LSPPosition{
					Line:      8, // User struct
					Character: 7,
				},
			},
			expectHover: true,
			validate:    validateHoverResponse,
		},
		{
			name:     "TypeScript interface hover",
			language: "typescript",
			request: &LSPTestRequest{
				Method: "textDocument/hover",
				URI:    "file://index.ts",
				Position: &indexing.LSPPosition{
					Line:      1, // User interface
					Character: 20,
				},
			},
			expectHover: true,
			validate:    validateHoverResponse,
		},
		{
			name:     "Python class hover",
			language: "python",
			request: &LSPTestRequest{
				Method: "textDocument/hover",
				URI:    "file://user.py",
				Position: &indexing.LSPPosition{
					Line:      10, // User class
					Character: 8,
				},
			},
			expectHover: true,
			validate:    validateHoverResponse,
		},
		{
			name:     "Java method hover",
			language: "java",
			request: &LSPTestRequest{
				Method: "textDocument/hover",
				URI:    "file://src/main/java/com/example/user/UserService.java",
				Position: &indexing.LSPPosition{
					Line:      25, // createUser method
					Character: 15,
				},
			},
			expectHover: true,
			validate:    validateHoverResponse,
		},
		{
			name:     "Empty space hover (should return null)",
			language: "go",
			request: &LSPTestRequest{
				Method: "textDocument/hover",
				URI:    "file://main.go",
				Position: &indexing.LSPPosition{
					Line:      1, // Empty line
					Character: 0,
				},
			},
			expectHover: false,
			validate:    validateEmptyHoverResponse,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			startTime := time.Now()

			params, err := suite.scipMapper.ParseLSPParams(tc.request.Method, map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": tc.request.URI,
				},
				"position": map[string]interface{}{
					"line":      tc.request.Position.Line,
					"character": tc.request.Position.Character,
				},
			})
			if err != nil {
				t.Fatalf("Failed to parse LSP params: %v", err)
				return
			}

			response, err := suite.scipMapper.MapHover(params)
			if err != nil {
				t.Errorf("Hover request failed: %v", err)
				suite.RecordTestResult(tc.name, false, time.Since(startTime))
				return
			}

			testResponse := &LSPTestResponse{
				Method:       tc.request.Method,
				Response:     response,
				Found:        response != nil && string(response) != "null",
				QueryTime:    time.Since(startTime),
				ValidatedFields: []string{"contents", "range"},
			}

			tc.validate(t, testResponse)

			suite.RecordQueryMetrics(testResponse.QueryTime, testResponse.CacheHit, !t.Failed())
			suite.RecordTestResult(tc.name, !t.Failed(), time.Since(startTime))
		})
	}
}

// testDocumentSymbolIntegration tests textDocument/documentSymbol integration
func testDocumentSymbolIntegration(t *testing.T, suite *SCIPTestSuite) {
	testCases := []struct {
		name              string
		language          string
		uri               string
		expectedMinSymbols int
		validate          func(t *testing.T, response *LSPTestResponse, minSymbols int)
	}{
		{
			name:              "Go file symbols",
			language:          "go",
			uri:               "file://main.go",
			expectedMinSymbols: 8, // User, UserService, DefaultUserService, methods, etc.
			validate:          validateDocumentSymbolResponse,
		},
		{
			name:              "TypeScript file symbols",
			language:          "typescript",
			uri:               "file://index.ts",
			expectedMinSymbols: 6, // User, UserRepository, InMemoryUserRepository, etc.
			validate:          validateDocumentSymbolResponse,
		},
		{
			name:              "Python file symbols",
			language:          "python",
			uri:               "file://user.py",
			expectedMinSymbols: 10, // User, UserRepository, UserService, methods, etc.
			validate:          validateDocumentSymbolResponse,
		},
		{
			name:              "Java file symbols",
			language:          "java",
			uri:               "file://src/main/java/com/example/user/UserService.java",
			expectedMinSymbols: 5, // UserService class and methods
			validate:          validateDocumentSymbolResponse,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			startTime := time.Now()

			params, err := suite.scipMapper.ParseLSPParams("textDocument/documentSymbol", map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": tc.uri,
				},
			})
			if err != nil {
				t.Fatalf("Failed to parse LSP params: %v", err)
				return
			}

			response, err := suite.scipMapper.MapDocumentSymbol(params)
			if err != nil {
				t.Errorf("DocumentSymbol request failed: %v", err)
				suite.RecordTestResult(tc.name, false, time.Since(startTime))
				return
			}

			testResponse := &LSPTestResponse{
				Method:       "textDocument/documentSymbol",
				Response:     response,
				Found:        response != nil && string(response) != "[]",
				QueryTime:    time.Since(startTime),
				ValidatedFields: []string{"symbols", "count", "hierarchy"},
			}

			tc.validate(t, testResponse, tc.expectedMinSymbols)

			suite.RecordQueryMetrics(testResponse.QueryTime, testResponse.CacheHit, !t.Failed())
			suite.RecordTestResult(tc.name, !t.Failed(), time.Since(startTime))
		})
	}
}

// testWorkspaceSymbolIntegration tests workspace/symbol integration
func testWorkspaceSymbolIntegration(t *testing.T, suite *SCIPTestSuite) {
	testCases := []struct {
		name              string
		query             string
		expectedMinSymbols int
		validate          func(t *testing.T, response *LSPTestResponse, minSymbols int)
	}{
		{
			name:              "Search for User symbols",
			query:             "User",
			expectedMinSymbols: 4, // User symbols across all languages
			validate:          validateWorkspaceSymbolResponse,
		},
		{
			name:              "Search for Service symbols",
			query:             "Service",
			expectedMinSymbols: 2, // UserService classes
			validate:          validateWorkspaceSymbolResponse,
		},
		{
			name:              "Search for Repository symbols",
			query:             "Repository",
			expectedMinSymbols: 3, // Repository interfaces and implementations
			validate:          validateWorkspaceSymbolResponse,
		},
		{
			name:              "Search for createUser methods",
			query:             "createUser",
			expectedMinSymbols: 3, // createUser methods across languages
			validate:          validateWorkspaceSymbolResponse,
		},
		{
			name:              "Search for non-existent symbol",
			query:             "NonExistentSymbol",
			expectedMinSymbols: 0, // Should return empty
			validate:          validateEmptyWorkspaceSymbolResponse,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			startTime := time.Now()

			params, err := suite.scipMapper.ParseLSPParams("workspace/symbol", map[string]interface{}{
				"query": tc.query,
			})
			if err != nil {
				t.Fatalf("Failed to parse LSP params: %v", err)
				return
			}

			response, err := suite.scipMapper.MapWorkspaceSymbol(params)
			if err != nil {
				t.Errorf("WorkspaceSymbol request failed: %v", err)
				suite.RecordTestResult(tc.name, false, time.Since(startTime))
				return
			}

			testResponse := &LSPTestResponse{
				Method:       "workspace/symbol",
				Response:     response,
				Found:        response != nil && string(response) != "[]",
				QueryTime:    time.Since(startTime),
				ValidatedFields: []string{"symbols", "count", "locations"},
			}

			tc.validate(t, testResponse, tc.expectedMinSymbols)

			suite.RecordQueryMetrics(testResponse.QueryTime, testResponse.CacheHit, !t.Failed())
			suite.RecordTestResult(tc.name, !t.Failed(), time.Since(startTime))
		})
	}
}

// Validation functions

func validateDefinitionResponse(t *testing.T, response *LSPTestResponse) {
	if !response.Found {
		t.Error("Expected definition to be found")
		return
	}

	// Parse response to validate structure
	var locations []indexing.LSPLocation
	if err := json.Unmarshal(response.Response, &locations); err != nil {
		// Try single location format
		var location indexing.LSPLocation
		if err := json.Unmarshal(response.Response, &location); err != nil {
			t.Errorf("Failed to parse definition response: %v", err)
			return
		}
		locations = []indexing.LSPLocation{location}
	}

	if len(locations) == 0 {
		t.Error("Expected at least one definition location")
		return
	}

	// Validate location structure
	for _, loc := range locations {
		if loc.URI == "" {
			t.Error("Definition location missing URI")
		}
		if loc.Range.Start.Line < 0 || loc.Range.Start.Character < 0 {
			t.Error("Invalid definition range start position")
		}
		if loc.Range.End.Line < 0 || loc.Range.End.Character < 0 {
			t.Error("Invalid definition range end position")
		}
	}

	// Validate performance
	if response.QueryTime > 10*time.Millisecond {
		t.Logf("Definition query took %v (target: <10ms)", response.QueryTime)
	}
}

func validateReferencesResponse(t *testing.T, response *LSPTestResponse, minRefs int) {
	if !response.Found && minRefs > 0 {
		t.Error("Expected references to be found")
		return
	}

	var references []indexing.LSPLocation
	if err := json.Unmarshal(response.Response, &references); err != nil {
		t.Errorf("Failed to parse references response: %v", err)
		return
	}

	if len(references) < minRefs {
		t.Errorf("Expected at least %d references, got %d", minRefs, len(references))
	}

	// Validate reference structure
	for i, ref := range references {
		if ref.URI == "" {
			t.Errorf("Reference %d missing URI", i)
		}
		if ref.Range.Start.Line < 0 || ref.Range.Start.Character < 0 {
			t.Errorf("Reference %d has invalid range start position", i)
		}
	}
}

func validateHoverResponse(t *testing.T, response *LSPTestResponse) {
	if !response.Found {
		t.Error("Expected hover information to be found")
		return
	}

	var hover indexing.Hover
	if err := json.Unmarshal(response.Response, &hover); err != nil {
		t.Errorf("Failed to parse hover response: %v", err)
		return
	}

	if hover.Contents.Value == "" {
		t.Error("Hover contents should not be empty")
	}
}

func validateEmptyHoverResponse(t *testing.T, response *LSPTestResponse) {
	if string(response.Response) != "null" {
		t.Errorf("Expected null hover response, got: %s", string(response.Response))
	}
}

func validateDocumentSymbolResponse(t *testing.T, response *LSPTestResponse, minSymbols int) {
	if !response.Found && minSymbols > 0 {
		t.Error("Expected document symbols to be found")
		return
	}

	var symbols []indexing.DocumentSymbol
	if err := json.Unmarshal(response.Response, &symbols); err != nil {
		t.Errorf("Failed to parse document symbol response: %v", err)
		return
	}

	if len(symbols) < minSymbols {
		t.Errorf("Expected at least %d symbols, got %d", minSymbols, len(symbols))
	}

	// Validate symbol structure
	for i, symbol := range symbols {
		if symbol.Name == "" {
			t.Errorf("Symbol %d missing name", i)
		}
		if symbol.Kind == 0 {
			t.Errorf("Symbol %d missing kind", i)
		}
		if symbol.Range.Start.Line < 0 || symbol.Range.Start.Character < 0 {
			t.Errorf("Symbol %d has invalid range", i)
		}
	}
}

func validateWorkspaceSymbolResponse(t *testing.T, response *LSPTestResponse, minSymbols int) {
	if !response.Found && minSymbols > 0 {
		t.Error("Expected workspace symbols to be found")
		return
	}

	var symbols []indexing.SymbolInformation
	if err := json.Unmarshal(response.Response, &symbols); err != nil {
		t.Errorf("Failed to parse workspace symbol response: %v", err)
		return
	}

	if len(symbols) < minSymbols {
		t.Errorf("Expected at least %d symbols, got %d", minSymbols, len(symbols))
	}

	// Validate symbol structure
	for i, symbol := range symbols {
		if symbol.Name == "" {
			t.Errorf("Symbol %d missing name", i)
		}
		if symbol.Location.URI == "" {
			t.Errorf("Symbol %d missing location URI", i)
		}
	}
}

func validateEmptyWorkspaceSymbolResponse(t *testing.T, response *LSPTestResponse, minSymbols int) {
	var symbols []indexing.SymbolInformation
	if err := json.Unmarshal(response.Response, &symbols); err != nil {
		t.Errorf("Failed to parse workspace symbol response: %v", err)
		return
	}

	if len(symbols) != 0 {
		t.Errorf("Expected no symbols for non-existent query, got %d", len(symbols))
	}
}

// TestLSPMethodPerformance tests LSP method performance with SCIP
func TestLSPMethodPerformance(t *testing.T) {
	suite, err := NewSCIPTestSuite(DefaultSCIPTestConfig())
	if err != nil {
		t.Fatalf("Failed to create SCIP test suite: %v", err)
	}
	defer suite.Cleanup()

	// Performance test configuration
	testDuration := 1 * time.Minute
	maxConcurrentRequests := 10

	methods := []string{
		"textDocument/definition",
		"textDocument/references",
		"textDocument/hover",
		"textDocument/documentSymbol",
		"workspace/symbol",
	}

	for _, method := range methods {
		t.Run(fmt.Sprintf("Performance_%s", method), func(t *testing.T) {
			testMethodPerformance(t, suite, method, testDuration, maxConcurrentRequests)
		})
	}
}

func testMethodPerformance(t *testing.T, suite *SCIPTestSuite, method string, duration time.Duration, maxConcurrent int) {
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	// Create semaphore for concurrency control
	semaphore := make(chan struct{}, maxConcurrent)
	results := make(chan time.Duration, 1000)
	
	startTime := time.Now()
	requestCount := 0

	for {
		select {
		case <-ctx.Done():
			goto done
		case semaphore <- struct{}{}:
			go func() {
				defer func() { <-semaphore }()
				
				queryStart := time.Now()
				err := executeMethodRequest(suite, method)
				queryTime := time.Since(queryStart)
				
				if err == nil {
					results <- queryTime
				}
			}()
			requestCount++
		}
	}

done:
	close(results)

	// Collect results
	var queryTimes []time.Duration
	for queryTime := range results {
		queryTimes = append(queryTimes, queryTime)
	}

	totalTime := time.Since(startTime)
	successCount := len(queryTimes)
	
	if successCount == 0 {
		t.Errorf("No successful requests for method %s", method)
		return
	}

	// Calculate statistics
	var totalQueryTime time.Duration
	for _, qt := range queryTimes {
		totalQueryTime += qt
	}
	avgQueryTime := totalQueryTime / time.Duration(successCount)

	// Calculate throughput
	throughput := float64(successCount) / totalTime.Seconds()

	t.Logf("Method %s performance:", method)
	t.Logf("  Total requests: %d", requestCount)
	t.Logf("  Successful requests: %d", successCount)
	t.Logf("  Success rate: %.2f%%", float64(successCount)/float64(requestCount)*100)
	t.Logf("  Average query time: %v", avgQueryTime)
	t.Logf("  Throughput: %.2f req/sec", throughput)

	// Validate performance targets
	if avgQueryTime > 10*time.Millisecond {
		t.Errorf("Average query time %v exceeds target 10ms", avgQueryTime)
	}

	if throughput < 50.0 {
		t.Errorf("Throughput %.2f req/sec below target 50 req/sec", throughput)
	}
}

func executeMethodRequest(suite *SCIPTestSuite, method string) error {
	switch method {
	case "textDocument/definition":
		params, _ := suite.scipMapper.ParseLSPParams(method, map[string]interface{}{
			"textDocument": map[string]interface{}{"uri": "file://main.go"},
			"position":     map[string]interface{}{"line": 27, "character": 10},
		})
		_, err := suite.scipMapper.MapDefinition(params)
		return err

	case "textDocument/references":
		params, _ := suite.scipMapper.ParseLSPParams(method, map[string]interface{}{
			"textDocument": map[string]interface{}{"uri": "file://main.go"},
			"position":     map[string]interface{}{"line": 27, "character": 10},
			"context":      map[string]interface{}{"includeDeclaration": false},
		})
		_, err := suite.scipMapper.MapReferences(params)
		return err

	case "textDocument/hover":
		params, _ := suite.scipMapper.ParseLSPParams(method, map[string]interface{}{
			"textDocument": map[string]interface{}{"uri": "file://main.go"},
			"position":     map[string]interface{}{"line": 27, "character": 10},
		})
		_, err := suite.scipMapper.MapHover(params)
		return err

	case "textDocument/documentSymbol":
		params, _ := suite.scipMapper.ParseLSPParams(method, map[string]interface{}{
			"textDocument": map[string]interface{}{"uri": "file://main.go"},
		})
		_, err := suite.scipMapper.MapDocumentSymbol(params)
		return err

	case "workspace/symbol":
		params, _ := suite.scipMapper.ParseLSPParams(method, map[string]interface{}{
			"query": "User",
		})
		_, err := suite.scipMapper.MapWorkspaceSymbol(params)
		return err

	default:
		return fmt.Errorf("unsupported method: %s", method)
	}
}