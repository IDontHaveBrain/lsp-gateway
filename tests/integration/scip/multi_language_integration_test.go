package scip

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"lsp-gateway/internal/indexing"
)

// TestMultiLanguageIntegration tests SCIP integration across multiple programming languages
func TestMultiLanguageIntegration(t *testing.T) {
	config := DefaultSCIPTestConfig()
	config.CrossLanguageReferences = true
	config.EnabledLanguages = []string{"go", "typescript", "python", "java"}
	config.EnableWorkspaceTests = true
	config.WorkspaceIsolationTests = true

	suite, err := NewSCIPTestSuite(config)
	if err != nil {
		t.Fatalf("Failed to create SCIP test suite: %v", err)
	}
	defer suite.Cleanup()

	t.Run("LanguageSpecificIntegration", func(t *testing.T) {
		testLanguageSpecificIntegration(t, suite)
	})

	t.Run("CrossLanguageReferences", func(t *testing.T) {
		testCrossLanguageReferences(t, suite)
	})

	t.Run("CrossLanguageSymbolSearch", func(t *testing.T) {
		testCrossLanguageSymbolSearch(t, suite)
	})

	t.Run("MultiLanguagePerformance", func(t *testing.T) {
		testMultiLanguagePerformance(t, suite)
	})

	t.Run("LanguageCoverageValidation", func(t *testing.T) {
		testLanguageCoverageValidation(t, suite)
	})

	// Add workspace-aware multi-language tests
	if config.EnableWorkspaceTests {
		t.Run("WorkspaceMultiLanguageIntegration", func(t *testing.T) {
			testWorkspaceMultiLanguageIntegration(t, suite)
		})

		t.Run("WorkspaceLanguageIsolation", func(t *testing.T) {
			testWorkspaceLanguageIsolation(t, suite)
		})
	}

	// Validate overall performance
	suite.ValidatePerformanceTargets(t)
}

// testLanguageSpecificIntegration tests SCIP integration for each language individually
func testLanguageSpecificIntegration(t *testing.T, suite *SCIPTestSuite) {
	languages := []struct {
		name           string
		testFile       string
		symbolPosition *indexing.LSPPosition
		expectedSymbol string
	}{
		{
			name:     "Go",
			testFile: "file://main.go",
			symbolPosition: &indexing.LSPPosition{
				Line:      8,  // User struct
				Character: 7,
			},
			expectedSymbol: "User",
		},
		{
			name:     "TypeScript",
			testFile: "file://index.ts",
			symbolPosition: &indexing.LSPPosition{
				Line:      1,  // User interface
				Character: 20,
			},
			expectedSymbol: "User",
		},
		{
			name:     "Python",
			testFile: "file://user.py",
			symbolPosition: &indexing.LSPPosition{
				Line:      10, // User class
				Character: 8,
			},
			expectedSymbol: "User",
		},
		{
			name:     "Java",
			testFile: "file://src/main/java/com/example/user/User.java",
			symbolPosition: &indexing.LSPPosition{
				Line:      8,  // User class
				Character: 15,
			},
			expectedSymbol: "User",
		},
	}

	for _, lang := range languages {
		t.Run(lang.name, func(t *testing.T) {
			testLanguageIntegration(t, suite, lang.name, lang.testFile, lang.symbolPosition, lang.expectedSymbol)
		})
	}
}

func testLanguageIntegration(t *testing.T, suite *SCIPTestSuite, language, testFile string, symbolPos *indexing.LSPPosition, expectedSymbol string) {
	// Test definition lookup
	t.Run("Definition", func(t *testing.T) {
		startTime := time.Now()

		params, err := suite.scipMapper.ParseLSPParams("textDocument/definition", map[string]interface{}{
			"textDocument": map[string]interface{}{"uri": testFile},
			"position":     map[string]interface{}{"line": symbolPos.Line, "character": symbolPos.Character},
		})
		if err != nil {
			t.Fatalf("Failed to parse LSP params: %v", err)
		}

		response, err := suite.scipMapper.MapDefinition(params)
		if err != nil {
			t.Errorf("Definition lookup failed for %s: %v", language, err)
			return
		}

		if response == nil || string(response) == "[]" {
			t.Errorf("No definition found for %s symbol", language)
			return
		}

		queryTime := time.Since(startTime)
		suite.RecordQueryMetrics(queryTime, false, true)

		// Update language coverage
		suite.metrics.mutex.Lock()
		suite.metrics.LanguageCoverage[language]++
		suite.metrics.mutex.Unlock()

		t.Logf("%s definition lookup: %v", language, queryTime)
	})

	// Test references lookup
	t.Run("References", func(t *testing.T) {
		startTime := time.Now()

		params, err := suite.scipMapper.ParseLSPParams("textDocument/references", map[string]interface{}{
			"textDocument": map[string]interface{}{"uri": testFile},
			"position":     map[string]interface{}{"line": symbolPos.Line, "character": symbolPos.Character},
			"context":      map[string]interface{}{"includeDeclaration": false},
		})
		if err != nil {
			t.Fatalf("Failed to parse LSP params: %v", err)
		}

		response, err := suite.scipMapper.MapReferences(params)
		if err != nil {
			t.Errorf("References lookup failed for %s: %v", language, err)
			return
		}

		queryTime := time.Since(startTime)
		suite.RecordQueryMetrics(queryTime, false, true)

		// Validate references structure
		var references []indexing.LSPLocation
		if err := json.Unmarshal(response, &references); err != nil {
			t.Errorf("Failed to parse references response for %s: %v", language, err)
			return
		}

		t.Logf("%s references lookup: %v, found %d references", language, queryTime, len(references))
	})

	// Test hover information
	t.Run("Hover", func(t *testing.T) {
		startTime := time.Now()

		params, err := suite.scipMapper.ParseLSPParams("textDocument/hover", map[string]interface{}{
			"textDocument": map[string]interface{}{"uri": testFile},
			"position":     map[string]interface{}{"line": symbolPos.Line, "character": symbolPos.Character},
		})
		if err != nil {
			t.Fatalf("Failed to parse LSP params: %v", err)
		}

		response, err := suite.scipMapper.MapHover(params)
		if err != nil {
			t.Errorf("Hover lookup failed for %s: %v", language, err)
			return
		}

		queryTime := time.Since(startTime)
		suite.RecordQueryMetrics(queryTime, false, true)

		// Check if hover response is available
		hasHover := response != nil && string(response) != "null"
		t.Logf("%s hover lookup: %v, has hover: %v", language, queryTime, hasHover)
	})

	// Test document symbols
	t.Run("DocumentSymbols", func(t *testing.T) {
		startTime := time.Now()

		params, err := suite.scipMapper.ParseLSPParams("textDocument/documentSymbol", map[string]interface{}{
			"textDocument": map[string]interface{}{"uri": testFile},
		})
		if err != nil {
			t.Fatalf("Failed to parse LSP params: %v", err)
		}

		response, err := suite.scipMapper.MapDocumentSymbol(params)
		if err != nil {
			t.Errorf("Document symbols lookup failed for %s: %v", language, err)
			return
		}

		queryTime := time.Since(startTime)
		suite.RecordQueryMetrics(queryTime, false, true)

		// Validate symbols structure
		var symbols []indexing.DocumentSymbol
		if err := json.Unmarshal(response, &symbols); err != nil {
			t.Errorf("Failed to parse document symbols response for %s: %v", language, err)
			return
		}

		if len(symbols) == 0 {
			t.Errorf("No document symbols found for %s", language)
			return
		}

		t.Logf("%s document symbols: %v, found %d symbols", language, queryTime, len(symbols))
	})
}

// testCrossLanguageReferences tests cross-language symbol references
func testCrossLanguageReferences(t *testing.T, suite *SCIPTestSuite) {
	// Test cases for cross-language references
	testCases := []struct {
		name           string
		sourceLanguage string
		targetLanguage string
		symbolName     string
		description    string
	}{
		{
			name:           "Go to TypeScript User reference",
			sourceLanguage: "go",
			targetLanguage: "typescript",
			symbolName:     "User",
			description:    "Go service calling TypeScript user service",
		},
		{
			name:           "TypeScript to Python User reference",
			sourceLanguage: "typescript",
			targetLanguage: "python",
			symbolName:     "User",
			description:    "TypeScript frontend calling Python backend",
		},
		{
			name:           "Java to Go Service reference",
			sourceLanguage: "java",
			targetLanguage: "go",
			symbolName:     "UserService",
			description:    "Java service calling Go microservice",
		},
		{
			name:           "Python to Java Repository reference",
			sourceLanguage: "python",
			targetLanguage: "java",
			symbolName:     "UserRepository",
			description:    "Python service using Java data layer",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			startTime := time.Now()

			// Search for the symbol across all languages
			params, err := suite.scipMapper.ParseLSPParams("workspace/symbol", map[string]interface{}{
				"query": tc.symbolName,
			})
			if err != nil {
				t.Fatalf("Failed to parse LSP params: %v", err)
			}

			response, err := suite.scipMapper.MapWorkspaceSymbol(params)
			if err != nil {
				t.Errorf("Cross-language symbol search failed: %v", err)
				return
			}

			queryTime := time.Since(startTime)
			suite.RecordQueryMetrics(queryTime, false, true)

			// Validate that we found symbols from multiple languages
			var symbols []indexing.SymbolInformation
			if err := json.Unmarshal(response, &symbols); err != nil {
				t.Errorf("Failed to parse workspace symbols response: %v", err)
				return
			}

			// Check for cross-language symbols
			languageMap := make(map[string]bool)
			for _, symbol := range symbols {
				// Determine language from URI
				uri := symbol.Location.URI
				if isLanguageFile(uri, tc.sourceLanguage) {
					languageMap[tc.sourceLanguage] = true
				}
				if isLanguageFile(uri, tc.targetLanguage) {
					languageMap[tc.targetLanguage] = true
				}
			}

			// Record cross-language query
			suite.metrics.mutex.Lock()
			suite.metrics.CrossLanguageQueries++
			suite.metrics.mutex.Unlock()

			foundCrossLang := len(languageMap) > 1
			if !foundCrossLang {
				t.Logf("Warning: No cross-language references found for %s (%s)", tc.symbolName, tc.description)
			} else {
				t.Logf("Found cross-language references for %s: %v", tc.symbolName, languageMap)
			}

			t.Logf("Cross-language search (%s): %v, found %d symbols", tc.name, queryTime, len(symbols))
		})
	}
}

// testCrossLanguageSymbolSearch tests searching for symbols across multiple languages
func testCrossLanguageSymbolSearch(t *testing.T, suite *SCIPTestSuite) {
	searchQueries := []struct {
		query           string
		expectedLangs   []string
		minMatches      int
		description     string
	}{
		{
			query:         "User",
			expectedLangs: []string{"go", "typescript", "python", "java"},
			minMatches:    4,
			description:   "User symbol should exist in all languages",
		},
		{
			query:         "Service",
			expectedLangs: []string{"go", "typescript", "python", "java"},
			minMatches:    2,
			description:   "Service symbols should exist in multiple languages",
		},
		{
			query:         "Repository",
			expectedLangs: []string{"typescript", "python", "java"},
			minMatches:    2,
			description:   "Repository pattern in multiple languages",
		},
		{
			query:         "create",
			expectedLangs: []string{"go", "typescript", "python", "java"},
			minMatches:    3,
			description:   "Create methods across languages",
		},
	}

	for _, searchQuery := range searchQueries {
		t.Run(fmt.Sprintf("Search_%s", searchQuery.query), func(t *testing.T) {
			startTime := time.Now()

			params, err := suite.scipMapper.ParseLSPParams("workspace/symbol", map[string]interface{}{
				"query": searchQuery.query,
			})
			if err != nil {
				t.Fatalf("Failed to parse LSP params: %v", err)
			}

			response, err := suite.scipMapper.MapWorkspaceSymbol(params)
			if err != nil {
				t.Errorf("Workspace symbol search failed: %v", err)
				return
			}

			queryTime := time.Since(startTime)
			suite.RecordQueryMetrics(queryTime, false, true)

			var symbols []indexing.SymbolInformation
			if err := json.Unmarshal(response, &symbols); err != nil {
				t.Errorf("Failed to parse workspace symbols response: %v", err)
				return
			}

			// Analyze language coverage
			languageCoverage := make(map[string]int)
			for _, symbol := range symbols {
				for _, lang := range searchQuery.expectedLangs {
					if isLanguageFile(symbol.Location.URI, lang) {
						languageCoverage[lang]++
						break
					}
				}
			}

			// Validate minimum matches
			totalMatches := len(symbols)
			if totalMatches < searchQuery.minMatches {
				t.Errorf("Expected at least %d matches for '%s', got %d", 
					searchQuery.minMatches, searchQuery.query, totalMatches)
			}

			// Validate language coverage
			for _, expectedLang := range searchQuery.expectedLangs {
				if languageCoverage[expectedLang] == 0 {
					t.Logf("Warning: No matches found in %s for query '%s'", expectedLang, searchQuery.query)
				}
			}

			t.Logf("Search '%s': %v, found %d symbols across %d languages", 
				searchQuery.query, queryTime, totalMatches, len(languageCoverage))
			t.Logf("Language coverage: %v", languageCoverage)
		})
	}
}

// testMultiLanguagePerformance tests performance across multiple languages
func testMultiLanguagePerformance(t *testing.T, suite *SCIPTestSuite) {
	languages := []string{"go", "typescript", "python", "java"}
	methods := []string{
		"textDocument/definition",
		"textDocument/references", 
		"textDocument/documentSymbol",
	}

	// Test concurrent multi-language requests
	t.Run("ConcurrentMultiLanguage", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		var wg sync.WaitGroup
		results := make(chan PerformanceResult, 100)
		semaphore := make(chan struct{}, 20) // Limit concurrency

		for {
			select {
			case <-ctx.Done():
				goto done
			case semaphore <- struct{}{}:
				wg.Add(1)
				go func() {
					defer wg.Done()
					defer func() { <-semaphore }()

					// Pick random language and method
					lang := languages[time.Now().Nanosecond()%len(languages)]
					method := methods[time.Now().Nanosecond()%len(methods)]

					result := executeMultiLanguageRequest(suite, lang, method)
					results <- result
				}()
			}
		}

	done:
		go func() {
			wg.Wait()
			close(results)
		}()

		// Collect and analyze results
		var allResults []PerformanceResult
		for result := range results {
			allResults = append(allResults, result)
		}

		analyzeMultiLanguagePerformance(t, allResults)
	})

	// Test language-specific performance
	for _, lang := range languages {
		t.Run(fmt.Sprintf("Language_%s_Performance", lang), func(t *testing.T) {
			testLanguageSpecificPerformance(t, suite, lang)
		})
	}
}

type PerformanceResult struct {
	Language    string
	Method      string
	Duration    time.Duration
	Success     bool
	Error       string
}

func executeMultiLanguageRequest(suite *SCIPTestSuite, language, method string) PerformanceResult {
	startTime := time.Now()
	
	var uri string
	var position *indexing.LSPPosition

	// Get language-specific test data
	switch language {
	case "go":
		uri = "file://main.go"
		position = &indexing.LSPPosition{Line: 27, Character: 10}
	case "typescript":
		uri = "file://index.ts"
		position = &indexing.LSPPosition{Line: 46, Character: 15}
	case "python":
		uri = "file://user.py"
		position = &indexing.LSPPosition{Line: 107, Character: 10}
	case "java":
		uri = "file://src/main/java/com/example/user/UserService.java"
		position = &indexing.LSPPosition{Line: 25, Character: 15}
	}

	var err error
	switch method {
	case "textDocument/definition":
		params, parseErr := suite.scipMapper.ParseLSPParams(method, map[string]interface{}{
			"textDocument": map[string]interface{}{"uri": uri},
			"position":     map[string]interface{}{"line": position.Line, "character": position.Character},
		})
		if parseErr != nil {
			err = parseErr
		} else {
			_, err = suite.scipMapper.MapDefinition(params)
		}

	case "textDocument/references":
		params, parseErr := suite.scipMapper.ParseLSPParams(method, map[string]interface{}{
			"textDocument": map[string]interface{}{"uri": uri},
			"position":     map[string]interface{}{"line": position.Line, "character": position.Character},
			"context":      map[string]interface{}{"includeDeclaration": false},
		})
		if parseErr != nil {
			err = parseErr
		} else {
			_, err = suite.scipMapper.MapReferences(params)
		}

	case "textDocument/documentSymbol":
		params, parseErr := suite.scipMapper.ParseLSPParams(method, map[string]interface{}{
			"textDocument": map[string]interface{}{"uri": uri},
		})
		if parseErr != nil {
			err = parseErr
		} else {
			_, err = suite.scipMapper.MapDocumentSymbol(params)
		}
	}

	duration := time.Since(startTime)
	success := err == nil

	errorMsg := ""
	if err != nil {
		errorMsg = err.Error()
	}

	return PerformanceResult{
		Language: language,
		Method:   method,
		Duration: duration,
		Success:  success,
		Error:    errorMsg,
	}
}

func analyzeMultiLanguagePerformance(t *testing.T, results []PerformanceResult) {
	if len(results) == 0 {
		t.Error("No performance results to analyze")
		return
	}

	// Group by language and method
	langStats := make(map[string][]time.Duration)
	methodStats := make(map[string][]time.Duration)
	successCount := 0
	totalCount := len(results)

	for _, result := range results {
		if result.Success {
			successCount++
			langStats[result.Language] = append(langStats[result.Language], result.Duration)
			methodStats[result.Method] = append(methodStats[result.Method], result.Duration)
		}
	}

	successRate := float64(successCount) / float64(totalCount) * 100

	t.Logf("Multi-language performance analysis:")
	t.Logf("  Total requests: %d", totalCount)
	t.Logf("  Success rate: %.2f%%", successRate)

	// Language performance analysis
	t.Logf("  Language performance:")
	for lang, durations := range langStats {
		if len(durations) > 0 {
			avg := calculateAverage(durations)
			p95 := calculatePercentile(durations, 95)
			t.Logf("    %s: avg=%v, p95=%v, samples=%d", lang, avg, p95, len(durations))
		}
	}

	// Method performance analysis
	t.Logf("  Method performance:")
	for method, durations := range methodStats {
		if len(durations) > 0 {
			avg := calculateAverage(durations)
			p95 := calculatePercentile(durations, 95)
			t.Logf("    %s: avg=%v, p95=%v, samples=%d", method, avg, p95, len(durations))
		}
	}

	// Validate targets
	if successRate < 95.0 {
		t.Errorf("Success rate %.2f%% below target 95%%", successRate)
	}
}

func testLanguageSpecificPerformance(t *testing.T, suite *SCIPTestSuite, language string) {
	testDuration := 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), testDuration)
	defer cancel()

	results := make([]time.Duration, 0)
	requestCount := 0

	for {
		select {
		case <-ctx.Done():
			goto done
		default:
			result := executeMultiLanguageRequest(suite, language, "textDocument/definition")
			requestCount++
			if result.Success {
				results = append(results, result.Duration)
			}
		}
	}

done:
	if len(results) == 0 {
		t.Errorf("No successful requests for language %s", language)
		return
	}

	avgTime := calculateAverage(results)
	p95Time := calculatePercentile(results, 95)
	successRate := float64(len(results)) / float64(requestCount) * 100

	t.Logf("Language %s performance:", language)
	t.Logf("  Total requests: %d", requestCount)
	t.Logf("  Success rate: %.2f%%", successRate)
	t.Logf("  Average time: %v", avgTime)
	t.Logf("  P95 time: %v", p95Time)

	// Language-specific performance targets
	if avgTime > 15*time.Millisecond {
		t.Errorf("Language %s average time %v exceeds target 15ms", language, avgTime)
	}
}

// testLanguageCoverageValidation validates that all enabled languages are properly tested
func testLanguageCoverageValidation(t *testing.T, suite *SCIPTestSuite) {
	metrics := suite.GetMetrics()

	enabledLanguages := suite.config.EnabledLanguages
	
	t.Logf("Language coverage validation:")
	for _, lang := range enabledLanguages {
		coverage := metrics.LanguageCoverage[lang]
		t.Logf("  %s: %d queries", lang, coverage)
		
		if coverage == 0 {
			t.Errorf("No queries executed for enabled language: %s", lang)
		}
	}

	// Validate cross-language functionality
	if suite.config.CrossLanguageReferences && metrics.CrossLanguageQueries == 0 {
		t.Error("Cross-language references enabled but no cross-language queries executed")
	}

	t.Logf("Cross-language queries: %d", metrics.CrossLanguageQueries)
}

// Helper functions

func isLanguageFile(uri, language string) bool {
	switch language {
	case "go":
		return uri == "file://main.go" || uri == "file://main_test.go"
	case "typescript":
		return uri == "file://index.ts" || uri == "file://user.test.ts"
	case "python":
		return uri == "file://user.py" || uri == "file://test_user.py"
	case "java":
		return uri == "file://src/main/java/com/example/user/User.java" ||
			   uri == "file://src/main/java/com/example/user/UserService.java" ||
			   uri == "file://src/main/java/com/example/user/UserRepository.java" ||
			   uri == "file://src/test/java/com/example/user/UserServiceTest.java"
	}
	return false
}

func calculateAverage(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	
	var total time.Duration
	for _, d := range durations {
		total += d
	}
	return total / time.Duration(len(durations))
}

func calculatePercentile(durations []time.Duration, percentile int) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	
	// Simple percentile calculation (would use proper sorting in production)
	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)
	
	// Basic bubble sort for simplicity
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	
	index := (percentile * len(sorted)) / 100
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	
	return sorted[index]
}

// TestMultiLanguageEdgeCases tests edge cases in multi-language scenarios
func TestMultiLanguageEdgeCases(t *testing.T) {
	suite, err := NewSCIPTestSuite(DefaultSCIPTestConfig())
	if err != nil {
		t.Fatalf("Failed to create SCIP test suite: %v", err)
	}
	defer suite.Cleanup()

	t.Run("EmptyQuery", func(t *testing.T) {
		testEmptyQueries(t, suite)
	})

	t.Run("InvalidURIs", func(t *testing.T) {
		testInvalidURIs(t, suite)
	})

	t.Run("InvalidPositions", func(t *testing.T) {
		testInvalidPositions(t, suite)
	})

	t.Run("UnsupportedLanguages", func(t *testing.T) {
		testUnsupportedLanguages(t, suite)
	})
}

func testEmptyQueries(t *testing.T, suite *SCIPTestSuite) {
	// Test empty workspace symbol query
	params, err := suite.scipMapper.ParseLSPParams("workspace/symbol", map[string]interface{}{
		"query": "",
	})
	if err != nil {
		t.Fatalf("Failed to parse LSP params: %v", err)
	}

	response, err := suite.scipMapper.MapWorkspaceSymbol(params)
	if err != nil {
		t.Errorf("Empty query should not fail: %v", err)
		return
	}

	// Should return empty array
	if string(response) != "[]" {
		t.Errorf("Expected empty array for empty query, got: %s", string(response))
	}
}

func testInvalidURIs(t *testing.T, suite *SCIPTestSuite) {
	invalidURIs := []string{
		"file://nonexistent.go",
		"http://invalid.com/file.go",
		"",
		"invalid-uri",
	}

	for _, uri := range invalidURIs {
		t.Run(fmt.Sprintf("URI_%s", uri), func(t *testing.T) {
			params, err := suite.scipMapper.ParseLSPParams("textDocument/definition", map[string]interface{}{
				"textDocument": map[string]interface{}{"uri": uri},
				"position":     map[string]interface{}{"line": 0, "character": 0},
			})
			
			if err == nil {
				_, err = suite.scipMapper.MapDefinition(params)
			}

			// Invalid URIs should either fail parsing or return empty results
			// But should not crash
			if err != nil {
				t.Logf("Expected error for invalid URI %s: %v", uri, err)
			}
		})
	}
}

func testInvalidPositions(t *testing.T, suite *SCIPTestSuite) {
	invalidPositions := []struct {
		line      int
		character int
		desc      string
	}{
		{-1, 0, "negative line"},
		{0, -1, "negative character"},
		{999999, 0, "line out of bounds"},
		{0, 999999, "character out of bounds"},
	}

	for _, pos := range invalidPositions {
		t.Run(pos.desc, func(t *testing.T) {
			params, err := suite.scipMapper.ParseLSPParams("textDocument/definition", map[string]interface{}{
				"textDocument": map[string]interface{}{"uri": "file://main.go"},
				"position":     map[string]interface{}{"line": pos.line, "character": pos.character},
			})
			
			if err == nil {
				_, err = suite.scipMapper.MapDefinition(params)
			}

			// Invalid positions should fail gracefully
			if err != nil {
				t.Logf("Expected error for invalid position (%d, %d): %v", pos.line, pos.character, err)
			}
		})
	}
}

func testUnsupportedLanguages(t *testing.T, suite *SCIPTestSuite) {
	// Test with file extensions not in the test suite
	unsupportedFiles := []string{
		"file://test.cpp",
		"file://test.rb", 
		"file://test.php",
		"file://test.cs",
	}

	for _, uri := range unsupportedFiles {
		t.Run(fmt.Sprintf("Language_%s", uri), func(t *testing.T) {
			params, err := suite.scipMapper.ParseLSPParams("textDocument/definition", map[string]interface{}{
				"textDocument": map[string]interface{}{"uri": uri},
				"position":     map[string]interface{}{"line": 0, "character": 0},
			})
			
			if err == nil {
				response, err := suite.scipMapper.MapDefinition(params)
				if err == nil && string(response) == "[]" {
					t.Logf("Unsupported language file %s correctly returned empty result", uri)
				}
			}
		})
	}
}

// testWorkspaceMultiLanguageIntegration tests multi-language support within workspaces
func testWorkspaceMultiLanguageIntegration(t *testing.T, suite *SCIPTestSuite) {
	// Create workspace with multiple language projects
	workspaceRoot := filepath.Join(suite.tempDir, "multi-lang-workspace")
	
	configManager := &TestWorkspaceConfigManager{
		workspaceRoot: workspaceRoot,
	}
	
	scipFactory := workspace.NewWorkspaceSCIPFactory(configManager)
	
	scipConfig := &indexing.SCIPConfig{
		CacheConfig: indexing.CacheConfig{
			Enabled: true,
			MaxSize: 5000,
			TTL:     20 * time.Minute,
		},
		Performance: indexing.PerformanceConfig{
			QueryTimeout:         15 * time.Millisecond,
			MaxConcurrentQueries: 8,
			IndexLoadTimeout:     1 * time.Minute,
		},
	}
	
	store, err := scipFactory.CreateStoreWithConfig(workspaceRoot, scipConfig)
	if err != nil {
		t.Fatalf("Failed to create workspace SCIP store: %v", err)
	}
	defer store.Close()

	// Test multi-language queries within single workspace
	languageTests := []struct {
		language string
		fileName string
		symbol   string
	}{
		{"go", "main.go", "UserService"},
		{"typescript", "index.ts", "UserComponent"},
		{"python", "user.py", "UserModel"},
		{"java", "User.java", "UserRepository"},
	}

	for _, test := range languageTests {
		t.Run(fmt.Sprintf("%s_in_workspace", test.language), func(t *testing.T) {
			uri := fmt.Sprintf("file://%s/%s/%s", workspaceRoot, test.language, test.fileName)
			
			// Test definition lookup
			defParams := map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": uri,
				},
				"position": map[string]interface{}{
					"line":      10,
					"character": 5,
				},
			}
			
			startTime := time.Now()
			result := store.Query("textDocument/definition", defParams)
			queryTime := time.Since(startTime)
			
			// Verify workspace context
			if result.Metadata != nil {
				if wsRoot, ok := result.Metadata["workspace_root"]; ok {
					if wsRoot != workspaceRoot {
						t.Errorf("Workspace context error: expected %s, got %s", workspaceRoot, wsRoot)
					}
				}
			}
			
			// Record metrics
			suite.recordWorkspaceQueryMetrics(workspaceRoot, queryTime, result.CacheHit, result.Found)
			
			t.Logf("Multi-language workspace query (%s): %v, found=%v, cache_hit=%v",
				test.language, queryTime, result.Found, result.CacheHit)
		})
	}

	// Test workspace symbol search across languages
	t.Run("workspace_symbol_search", func(t *testing.T) {
		searchQueries := []string{"User", "Service", "Component", "Repository"}
		
		for _, query := range searchQueries {
			params := map[string]interface{}{
				"query": query,
			}
			
			startTime := time.Now()
			result := store.Query("workspace/symbol", params)
			queryTime := time.Since(startTime)
			
			// Verify workspace isolation
			if result.Metadata != nil {
				if wsRoot, ok := result.Metadata["workspace_root"]; ok {
					if wsRoot != workspaceRoot {
						t.Errorf("Workspace symbol search isolation error: expected %s, got %s", 
							workspaceRoot, wsRoot)
					}
				}
			}
			
			suite.recordWorkspaceQueryMetrics(workspaceRoot, queryTime, result.CacheHit, result.Found)
			
			t.Logf("Workspace symbol search '%s': %v, found=%v, cache_hit=%v",
				query, queryTime, result.Found, result.CacheHit)
		}
	})
}

// testWorkspaceLanguageIsolation tests language-specific isolation within workspaces
func testWorkspaceLanguageIsolation(t *testing.T, suite *SCIPTestSuite) {
	// Create separate workspaces for each language to test isolation
	languages := []string{"go", "typescript", "python", "java"}
	workspaceStores := make(map[string]indexing.SCIPStore)

	// Setup isolated workspaces
	for _, lang := range languages {
		workspaceRoot := filepath.Join(suite.tempDir, fmt.Sprintf("%s-workspace", lang))
		
		configManager := &TestWorkspaceConfigManager{
			workspaceRoot: workspaceRoot,
		}
		
		scipFactory := workspace.NewWorkspaceSCIPFactory(configManager)
		
		scipConfig := &indexing.SCIPConfig{
			CacheConfig: indexing.CacheConfig{
				Enabled: true,
				MaxSize: 2000,
				TTL:     15 * time.Minute,
			},
			Performance: indexing.PerformanceConfig{
				QueryTimeout:         10 * time.Millisecond,
				MaxConcurrentQueries: 5,
				IndexLoadTimeout:     30 * time.Second,
			},
		}
		
		store, err := scipFactory.CreateStoreWithConfig(workspaceRoot, scipConfig)
		if err != nil {
			t.Fatalf("Failed to create %s workspace store: %v", lang, err)
		}
		
		workspaceStores[lang] = store
		defer store.Close()
	}

	// Test isolation between language workspaces
	testData := map[string]string{
		"go":         "GoUserService",
		"typescript": "TSUserComponent", 
		"python":     "PyUserModel",
		"java":       "JavaUserRepository",
	}

	// Cache language-specific data in each workspace
	for lang, symbolData := range testData {
		store := workspaceStores[lang]
		workspaceStore := store.(*workspace.WorkspaceSCIPStore)
		
		// Cache definition data
		defResponse := []byte(fmt.Sprintf(`[{"uri": "file:///%s-workspace/main.%s", "range": {"start": {"line": 5, "character": 0}, "end": {"line": 5, "character": %d}}}]`,
			lang, getFileExtension(lang), len(symbolData)))
		
		params := map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": fmt.Sprintf("file:///%s-workspace/main.%s", lang, getFileExtension(lang)),
			},
			"position": map[string]interface{}{
				"line":      5,
				"character": 0,
			},
		}
		
		if err := workspaceStore.CacheResponse("textDocument/definition", params, defResponse); err != nil {
			t.Errorf("Failed to cache definition for %s: %v", lang, err)
		}
	}

	// Verify isolation: each workspace should only access its own data
	for lang, expectedData := range testData {
		store := workspaceStores[lang]
		
		params := map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": fmt.Sprintf("file:///%s-workspace/main.%s", lang, getFileExtension(lang)),
			},
			"position": map[string]interface{}{
				"line":      5,
				"character": 0,
			},
		}
		
		result := store.Query("textDocument/definition", params)
		
		if !result.Found {
			t.Errorf("Language %s data not found in its own workspace", lang)
			continue
		}
		
		if !result.CacheHit {
			t.Errorf("Expected cache hit for %s workspace data", lang)
		}
		
		// Verify workspace context
		if result.Metadata != nil {
			if wsRoot, ok := result.Metadata["workspace_root"]; ok {
				expectedWsRoot := filepath.Join(suite.tempDir, fmt.Sprintf("%s-workspace", lang))
				if wsRoot != expectedWsRoot {
					t.Errorf("Workspace isolation error for %s: expected %s, got %s",
						lang, expectedWsRoot, wsRoot)
				}
			}
		}
		
		t.Logf("Language isolation verified for %s: cache_hit=%v, workspace_isolated=%v",
			lang, result.CacheHit, result.Metadata != nil)
	}

	// Test cross-workspace isolation (negative test)
	for sourceLang, sourceStore := range workspaceStores {
		for otherLang := range testData {
			if sourceLang == otherLang {
				continue
			}
			
			// Try to query other language's data from this workspace
			params := map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": fmt.Sprintf("file:///%s-workspace/main.%s", otherLang, getFileExtension(otherLang)),
				},
				"position": map[string]interface{}{
					"line":      5,
					"character": 0,
				},
			}
			
			result := sourceStore.Query("textDocument/definition", params)
			
			// Should not find data from other workspace
			if result.Found && result.CacheHit {
				t.Errorf("Isolation violation: %s workspace found cached data from %s workspace",
					sourceLang, otherLang)
			}
			
			// Verify workspace context remains correct
			if result.Metadata != nil {
				if wsRoot, ok := result.Metadata["workspace_root"]; ok {
					expectedWsRoot := filepath.Join(suite.tempDir, fmt.Sprintf("%s-workspace", sourceLang))
					if wsRoot != expectedWsRoot {
						t.Errorf("Cross-workspace context error: %s query from %s workspace shows wrong context",
							otherLang, sourceLang)
					}
				}
			}
		}
	}
	
	t.Logf("Language workspace isolation verification completed for %d languages", len(languages))
}

// Helper types for workspace tests
type TestWorkspaceConfigManager struct {
	workspaceRoot string
}

func (m *TestWorkspaceConfigManager) GetWorkspaceDirectory(workspaceRoot string) string {
	return filepath.Join(workspaceRoot, ".lsp-gateway")
}

func (m *TestWorkspaceConfigManager) LoadWorkspaceConfig(workspaceRoot string) (*WorkspaceConfig, error) {
	return &WorkspaceConfig{
		Workspace: workspace.WorkspaceInfo{
			WorkspaceID: fmt.Sprintf("test-ws-%s", filepath.Base(workspaceRoot)),
			Name:        filepath.Base(workspaceRoot),
			RootPath:    workspaceRoot,
			Hash:        fmt.Sprintf("hash-%s", filepath.Base(workspaceRoot)),
			CreatedAt:   time.Now(),
			LastUpdated: time.Now(),
			Version:     "1.0.0",
		},
	}, nil
}

func getFileExtension(language string) string {
	switch language {
	case "go":
		return "go"
	case "typescript":
		return "ts"
	case "python":
		return "py"
	case "java":  
		return "java"
	default:
		return "txt"
	}
}

func (suite *SCIPTestSuite) recordWorkspaceQueryMetrics(workspaceRoot string, queryTime time.Duration, cacheHit, success bool) {
	suite.metrics.mutex.Lock()
	defer suite.metrics.mutex.Unlock()
	
	suite.metrics.WorkspaceQueries++
	
	if cacheHit {
		suite.metrics.WorkspaceCacheHits++
	}
	
	// Update or create workspace-specific metrics
	if suite.metrics.WorkspaceStats == nil {
		suite.metrics.WorkspaceStats = make(map[string]*WorkspaceMetrics)
	}
	
	wsMetrics, exists := suite.metrics.WorkspaceStats[workspaceRoot]
	if !exists {
		wsMetrics = &WorkspaceMetrics{
			WorkspaceRoot:  workspaceRoot,
			IsolationLevel: "strong",
		}
		suite.metrics.WorkspaceStats[workspaceRoot] = wsMetrics
	}
	
	wsMetrics.QueryCount++
	wsMetrics.LastQuery = time.Now()
	
	// Update average latency (simple moving average)
	if wsMetrics.AverageLatency == 0 {
		wsMetrics.AverageLatency = queryTime
	} else {
		wsMetrics.AverageLatency = (wsMetrics.AverageLatency + queryTime) / 2
	}
	
	// Update cache hit rate
	if cacheHit {
		hits := float64(wsMetrics.QueryCount-1) * wsMetrics.CacheHitRate + 1
		wsMetrics.CacheHitRate = hits / float64(wsMetrics.QueryCount)
	} else {
		hits := float64(wsMetrics.QueryCount-1) * wsMetrics.CacheHitRate  
		wsMetrics.CacheHitRate = hits / float64(wsMetrics.QueryCount)
	}
}