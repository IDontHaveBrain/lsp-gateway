package assertions

import (
	"encoding/json"
	"fmt"
	"strings"

	"lsp-gateway/internal/testing/lsp/cases"
	lspconfig "lsp-gateway/internal/testing/lsp/config"
)

// Example functions demonstrating how to use the assertion helpers
// These can be used as templates or reference implementations

// ExampleDefinitionValidation demonstrates how to validate a textDocument/definition response
func ExampleDefinitionValidation() {
	// Example test case with definition request
	testCase := &cases.TestCase{
		ID:          "def-001",
		Name:        "Go function definition",
		Method:      cases.LSPMethodDefinition,
		Description: "Find definition of calculateTotal function",
	}

	// Example response (single location)
	response := json.RawMessage(`{
		"uri": "file:///path/to/file.go",
		"range": {
			"start": {"line": 10, "character": 5},
			"end": {"line": 10, "character": 18}
		}
	}`)
	testCase.Response = response

	// Create expected result using builder pattern
	expected := NewExpectedResult().
		WithDefinition(true).
		WithDefinitionLocation("file:///path/to/file.go", 10, 5, 10, 18).
		Build()

	// Validate using comprehensive validator
	validator := NewComprehensiveValidator(nil)
	err := validator.ValidateTestCase(testCase, expected)
	if err != nil {
		fmt.Printf("Validation error: %v\n", err)
		return
	}

	// Check results
	summary := GetValidationSummary(testCase.ValidationResults, false)
	fmt.Printf("Definition validation: %d/%d passed (%.1f%%)\n", 
		summary.PassedValidations, summary.TotalValidations, summary.PassRate)
}

// ExampleReferencesValidation demonstrates how to validate a textDocument/references response
func ExampleReferencesValidation() {
	// Example test case with references request
	testCase := &cases.TestCase{
		ID:          "ref-001",
		Name:        "Go variable references",
		Method:      cases.LSPMethodReferences,
		Description: "Find references to userName variable",
	}

	// Example response (array of locations)
	response := json.RawMessage(`[
		{
			"uri": "file:///path/to/file1.go",
			"range": {
				"start": {"line": 15, "character": 8},
				"end": {"line": 15, "character": 16}
			}
		},
		{
			"uri": "file:///path/to/file2.go",
			"range": {
				"start": {"line": 23, "character": 12},
				"end": {"line": 23, "character": 20}
			}
		}
	]`)
	testCase.Response = response

	// Create expected result
	expected := NewExpectedResult().
		WithReferences(true).
		WithReferencesCount(2, 10). // Expect 2-10 references
		WithReferencesIncludeDeclaration(false).
		WithReferencesFromFile("file1.go").
		Build()

	// Validate
	validator := NewComprehensiveValidator(nil)
	err := validator.ValidateTestCase(testCase, expected)
	if err != nil {
		fmt.Printf("Validation error: %v\n", err)
		return
	}

	// Check results
	summary := GetValidationSummary(testCase.ValidationResults, false)
	fmt.Printf("References validation: %d/%d passed (%.1f%%)\n", 
		summary.PassedValidations, summary.TotalValidations, summary.PassRate)
}

// ExampleHoverValidation demonstrates how to validate a textDocument/hover response
func ExampleHoverValidation() {
	// Example test case with hover request
	testCase := &cases.TestCase{
		ID:          "hover-001",
		Name:        "Go function hover",
		Method:      cases.LSPMethodHover,
		Description: "Get hover information for fmt.Println",
	}

	// Example response with markdown content
	response := json.RawMessage(`{
		"contents": {
			"kind": "markdown",
			"value": "` + "```" + `go\nfunc Println(a ...interface{}) (n int, err error)\n` + "```" + `\n\nPrintln formats using the default formats for its operands and writes to standard output."
		},
		"range": {
			"start": {"line": 8, "character": 4},
			"end": {"line": 8, "character": 11}
		}
	}`)
	testCase.Response = response

	// Create expected result
	expected := NewExpectedResult().
		WithHover(true).
		WithHoverContent(true, "func Println", "standard output").
		WithHoverMarkupKind("markdown").
		WithContains("```go").
		Build()

	// Validate
	validator := NewComprehensiveValidator(nil)
	err := validator.ValidateTestCase(testCase, expected)
	if err != nil {
		fmt.Printf("Validation error: %v\n", err)
		return
	}

	// Check results
	summary := GetValidationSummary(testCase.ValidationResults, false)
	fmt.Printf("Hover validation: %d/%d passed (%.1f%%)\n", 
		summary.PassedValidations, summary.TotalValidations, summary.PassRate)
}

// ExampleDocumentSymbolValidation demonstrates how to validate a textDocument/documentSymbol response
func ExampleDocumentSymbolValidation() {
	// Example test case with document symbol request
	testCase := &cases.TestCase{
		ID:          "docsym-001",
		Name:        "Go file document symbols",
		Method:      cases.LSPMethodDocumentSymbol,
		Description: "Get document symbols for main.go",
	}

	// Example response with hierarchical symbols
	response := json.RawMessage(`[
		{
			"name": "main",
			"kind": 12,
			"location": {
				"uri": "file:///path/to/main.go",
				"range": {
					"start": {"line": 10, "character": 0},
					"end": {"line": 20, "character": 1}
				}
			},
			"children": [
				{
					"name": "userName",
					"kind": 13,
					"location": {
						"uri": "file:///path/to/main.go",
						"range": {
							"start": {"line": 11, "character": 4},
							"end": {"line": 11, "character": 12}
						}
					}
				}
			]
		}
	]`)
	testCase.Response = response

	// Create expected result
	expected := NewExpectedResult().
		WithDocumentSymbol(true).
		WithDocumentSymbolCount(1, 5).
		WithDocumentSymbolOfKind("main", SymbolKindFunction).
		Build()

	// Validate
	validator := NewComprehensiveValidator(nil)
	err := validator.ValidateTestCase(testCase, expected)
	if err != nil {
		fmt.Printf("Validation error: %v\n", err)
		return
	}

	// Check results
	summary := GetValidationSummary(testCase.ValidationResults, false)
	fmt.Printf("Document symbol validation: %d/%d passed (%.1f%%)\n", 
		summary.PassedValidations, summary.TotalValidations, summary.PassRate)
}

// ExampleWorkspaceSymbolValidation demonstrates how to validate a workspace/symbol response
func ExampleWorkspaceSymbolValidation() {
	// Example test case with workspace symbol request
	testCase := &cases.TestCase{
		ID:          "worksym-001",
		Name:        "Workspace symbol search",
		Method:      cases.LSPMethodWorkspaceSymbol,
		Description: "Search for 'User' symbols in workspace",
	}

	// Example response with multiple symbols
	response := json.RawMessage(`[
		{
			"name": "User",
			"kind": 5,
			"location": {
				"uri": "file:///path/to/user.go",
				"range": {
					"start": {"line": 5, "character": 5},
					"end": {"line": 5, "character": 9}
				}
			},
			"containerName": "models"
		},
		{
			"name": "UserService",
			"kind": 5,
			"location": {
				"uri": "file:///path/to/service.go",
				"range": {
					"start": {"line": 15, "character": 5},
					"end": {"line": 15, "character": 16}
				}
			},
			"containerName": "services"
		}
	]`)
	testCase.Response = response

	// Create expected result
	expected := NewExpectedResult().
		WithWorkspaceSymbol(true).
		WithWorkspaceSymbolCount(1, 10).
		WithWorkspaceSymbolOfKind("User", SymbolKindClass).
		WithContains("UserService").
		Build()

	// Validate
	validator := NewComprehensiveValidator(nil)
	err := validator.ValidateTestCase(testCase, expected)
	if err != nil {
		fmt.Printf("Validation error: %v\n", err)
		return
	}

	// Check results
	summary := GetValidationSummary(testCase.ValidationResults, false)
	fmt.Printf("Workspace symbol validation: %d/%d passed (%.1f%%)\n", 
		summary.PassedValidations, summary.TotalValidations, summary.PassRate)
}

// ExampleErrorValidation demonstrates how to validate error responses
func ExampleErrorValidation() {
	// Example test case that should result in an error
	testCase := &cases.TestCase{
		ID:          "error-001",
		Name:        "Invalid position error",
		Method:      cases.LSPMethodDefinition,
		Description: "Test error handling for invalid position",
	}

	// Example error response
	response := json.RawMessage(`{
		"error": {
			"code": -32602,
			"message": "Invalid position: line 1000 is out of bounds"
		}
	}`)
	testCase.Response = response

	// Create expected result for error case
	expected := NewExpectedResult().
		Success(false).
		WithError(true).
		WithErrorCode(-32602).
		WithErrorMessageContains("Invalid position", "out of bounds").
		Build()

	// Validate
	validator := NewComprehensiveValidator(nil)
	err := validator.ValidateTestCase(testCase, expected)
	if err != nil {
		fmt.Printf("Validation error: %v\n", err)
		return
	}

	// Check results
	summary := GetValidationSummary(testCase.ValidationResults, false)
	fmt.Printf("Error validation: %d/%d passed (%.1f%%)\n", 
		summary.PassedValidations, summary.TotalValidations, summary.PassRate)
}

// ExampleFlexibleMatching demonstrates flexible matching options
func ExampleFlexibleMatching() {
	// Create assertion context
	config := &lspconfig.ValidationConfig{
		StrictMode:        false,
		ValidateTypes:     true,
		ValidatePositions: true,
		ValidateURIs:      true,
	}
	
	testCase := &cases.TestCase{
		ID: "flexible-001",
		Name: "Flexible matching example",
	}
	
	ctx := NewAssertionContext(testCase, config)

	// Example: Flexible URI matching
	actualURI := "file:///home/user/project/src/main.go"
	
	// Exact match
	exactMatch := &ExpectedURI{
		Exact: &actualURI,
	}
	result1 := ctx.AssertURI(actualURI, exactMatch, "exact_uri")
	fmt.Printf("Exact match: %s\n", result1.Message)

	// Contains match
	containsStr := "project/src"
	containsMatch := &ExpectedURI{
		Contains: &containsStr,
	}
	result2 := ctx.AssertURI(actualURI, containsMatch, "contains_uri")
	fmt.Printf("Contains match: %s\n", result2.Message)

	// Pattern match
	pattern := `file:///.+/main\.go$`
	patternMatch := &ExpectedURI{
		Pattern: &pattern,
	}
	result3 := ctx.AssertURI(actualURI, patternMatch, "pattern_uri")
	fmt.Printf("Pattern match: %s\n", result3.Message)

	// Example: Flexible array length matching
	symbols := make([]interface{}, 5) // 5 symbols
	
	// Range matching
	rangeLength := &ExpectedArrayLength{
		Min: intPtr(3),
		Max: intPtr(10),
	}
	result4 := ctx.AssertArrayLength(symbols, rangeLength, "symbols_range")
	fmt.Printf("Range match: %s\n", result4.Message)

	// Exact length matching
	exactLength := &ExpectedArrayLength{
		Exact: intPtr(5),
	}
	result5 := ctx.AssertArrayLength(symbols, exactLength, "symbols_exact")
	fmt.Printf("Exact length match: %s\n", result5.Message)
}

// ExampleValidationComparison demonstrates how to compare validation results
func ExampleValidationComparison() {
	// Create baseline results
	baseline := []*cases.ValidationResult{
		{Name: "test1", Passed: true, Message: "Test 1 passed"},
		{Name: "test2", Passed: false, Message: "Test 2 failed"},
		{Name: "test3", Passed: true, Message: "Test 3 passed"},
	}

	// Create current results (with improvements and regressions)
	current := []*cases.ValidationResult{
		{Name: "test1", Passed: true, Message: "Test 1 passed"},
		{Name: "test2", Passed: true, Message: "Test 2 now passes"},  // Improved
		{Name: "test3", Passed: false, Message: "Test 3 now fails"}, // Regressed
		{Name: "test4", Passed: true, Message: "Test 4 is new"},     // Added
	}

	// Compare results
	comparison := CompareValidationResults(baseline, current)
	
	fmt.Printf("Validation comparison summary: %s\n", comparison.Summary())
	fmt.Printf("Improvements: %d\n", len(comparison.Improved))
	fmt.Printf("Regressions: %d\n", len(comparison.Regressed))
	fmt.Printf("New tests: %d\n", len(comparison.Added))
	fmt.Printf("Removed tests: %d\n", len(comparison.Removed))

	// Print detailed improvements and regressions
	for _, change := range comparison.Improved {
		fmt.Printf("✅ %s: %s -> %s\n", change.Name, change.Before.Message, change.After.Message)
	}
	
	for _, change := range comparison.Regressed {
		fmt.Printf("❌ %s: %s -> %s\n", change.Name, change.Before.Message, change.After.Message)
	}
}

// ExampleBuilderPatterns demonstrates advanced builder patterns
func ExampleBuilderPatterns() {
	// Complex definition expectation with multiple scenarios
	definitionBuilder := NewExpectedResult().
		WithDefinition(true).
		WithDefinitionArray(1, 3). // Expect 1-3 definitions
		WithMaxResponseTime(500).   // Max 500ms response time
		WithContains("func", "return").
		WithExcludes("error", "panic")

	// Add specific location expectations
	definitionBuilder.
		WithDefinitionLocation("file:///src/main.go", 10, 5, 10, 15).
		WithDefinitionLocation("file:///src/utils.go", 25, 8, 25, 20)

	definitionExpected := definitionBuilder.Build()

	// Complex hover expectation with content validation
	hoverExpected := NewExpectedResult().
		WithHover(true).
		WithHoverContent(true, "function signature", "documentation").
		WithHoverMarkupKind("markdown").
		WithMatches(`func \w+\(.*\)`, `returns? \w+`). // Regex patterns
		WithMaxResponseTime(200).
		Build()

	// Complex workspace symbol expectation
	workspaceExpected := NewExpectedResult().
		WithWorkspaceSymbol(true).
		WithWorkspaceSymbolCount(5, 50).
		WithWorkspaceSymbolOfKind("UserController", SymbolKindClass).
		WithWorkspaceSymbolOfKind("getUserById", SymbolKindMethod).
		WithContains("User", "Controller").
		WithExcludes("Test", "Mock").
		WithMaxResponseTime(1000).
		Build()

	fmt.Printf("Definition expectation created with %d expected properties\n", 
		len(definitionExpected.Properties))
	fmt.Printf("Hover expectation has content validation: %t\n", 
		hoverExpected.Hover.Content.HasContent)
	fmt.Printf("Workspace symbol expectation count range: %d-%d\n", 
		*workspaceExpected.WorkspaceSymbol.ArrayLength.Min,
		*workspaceExpected.WorkspaceSymbol.ArrayLength.Max)
}

// Helper function for creating int pointers
func intPtr(i int) *int {
	return &i
}

// Helper function for creating string pointers
func stringPtr(s string) *string {
	return &s
}

// ExampleCustomValidation demonstrates how to extend the validation system
func ExampleCustomValidation() {
	// Custom validation function that can be integrated
	customValidator := func(testCase *cases.TestCase, response json.RawMessage) *cases.ValidationResult {
		// Custom business logic validation
		responseStr := string(response)
		
		// Example: Validate response doesn't contain sensitive information
		sensitivePatterns := []string{"password", "secret", "token"}
		for _, pattern := range sensitivePatterns {
			if containsIgnoreCase(responseStr, pattern) {
				return &cases.ValidationResult{
					Name:        "security_check",
					Description: "Validate response doesn't contain sensitive information",
					Passed:      false,
					Message:     fmt.Sprintf("Response contains potentially sensitive information: %s", pattern),
					Details: map[string]interface{}{
						"sensitive_pattern": pattern,
					},
				}
			}
		}
		
		return &cases.ValidationResult{
			Name:        "security_check",
			Description: "Validate response doesn't contain sensitive information",
			Passed:      true,
			Message:     "No sensitive information detected in response",
		}
	}

	// Example usage
	testCase := &cases.TestCase{ID: "custom-001"}
	response := json.RawMessage(`{"result": "user data"}`)
	
	result := customValidator(testCase, response)
	fmt.Printf("Custom validation result: %s\n", result.Message)
}

// Helper function for case-insensitive contains check
func containsIgnoreCase(text, pattern string) bool {
	return strings.Contains(strings.ToLower(text), strings.ToLower(pattern))
}