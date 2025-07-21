package assertions

import (
	"encoding/json"
	"testing"

	"lsp-gateway/internal/testing/lsp/cases"
)

// Test Position Assertions

func TestAssertPosition(t *testing.T) {
	config := DefaultValidationConfig()
	testCase := CreateTestCase("pos-001", "Position test", cases.LSPMethodDefinition)
	ctx := NewAssertionContext(testCase, config)

	// Test valid position
	validPosition := map[string]interface{}{
		"line":      float64(10),
		"character": float64(5),
	}
	
	expected := &ExpectedPosition{
		Line:      intPtr(10),
		Character: intPtr(5),
	}
	
	result := ctx.AssertPosition(validPosition, expected, "test_position")
	if !result.Passed {
		t.Errorf("Valid position should pass: %s", result.Message)
	}

	// Test invalid position - wrong line
	wrongExpected := &ExpectedPosition{
		Line:      intPtr(15),
		Character: intPtr(5),
	}
	
	result2 := ctx.AssertPosition(validPosition, wrongExpected, "test_position")
	if result2.Passed {
		t.Errorf("Wrong line should fail validation")
	}

	// Test missing fields
	invalidPosition := map[string]interface{}{
		"line": float64(10),
		// missing character
	}
	
	result3 := ctx.AssertPosition(invalidPosition, nil, "test_position")
	if result3.Passed {
		t.Errorf("Missing character field should fail validation")
	}
}

func TestAssertRange(t *testing.T) {
	config := DefaultValidationConfig()
	testCase := CreateTestCase("range-001", "Range test", cases.LSPMethodDefinition)
	ctx := NewAssertionContext(testCase, config)

	// Test valid range
	validRange := map[string]interface{}{
		"start": map[string]interface{}{
			"line":      float64(10),
			"character": float64(5),
		},
		"end": map[string]interface{}{
			"line":      float64(10),
			"character": float64(15),
		},
	}
	
	expected := &ExpectedRange{
		Start: &ExpectedPosition{Line: intPtr(10), Character: intPtr(5)},
		End:   &ExpectedPosition{Line: intPtr(10), Character: intPtr(15)},
	}
	
	results := ctx.AssertRange(validRange, expected, "test_range")
	allPassed := true
	for _, result := range results {
		if !result.Passed {
			allPassed = false
			t.Errorf("Range validation failed: %s", result.Message)
		}
	}
	if !allPassed {
		t.Errorf("Valid range should pass all validations")
	}

	// Test invalid range order (start after end)
	invalidRange := map[string]interface{}{
		"start": map[string]interface{}{
			"line":      float64(10),
			"character": float64(20),
		},
		"end": map[string]interface{}{
			"line":      float64(10),
			"character": float64(15),
		},
	}
	
	results2 := ctx.AssertRange(invalidRange, nil, "test_range")
	orderValidationPassed := true
	for _, result := range results2 {
		if result.Name == "test_range_order" && !result.Passed {
			orderValidationPassed = false
			break
		}
	}
	if orderValidationPassed {
		t.Errorf("Invalid range order should fail validation")
	}
}

func TestAssertLocation(t *testing.T) {
	config := DefaultValidationConfig()
	testCase := CreateTestCase("loc-001", "Location test", cases.LSPMethodDefinition)
	ctx := NewAssertionContext(testCase, config)

	// Test valid location
	validLocation := map[string]interface{}{
		"uri": "file:///path/to/file.go",
		"range": map[string]interface{}{
			"start": map[string]interface{}{
				"line":      float64(10),
				"character": float64(5),
			},
			"end": map[string]interface{}{
				"line":      float64(10),
				"character": float64(15),
			},
		},
	}
	
	expected := &ExpectedLocation{
		URI: &ExpectedURI{
			Contains: stringPtr("file.go"),
		},
		Range: &ExpectedRange{
			Start: &ExpectedPosition{Line: intPtr(10)},
		},
	}
	
	results := ctx.AssertLocation(validLocation, expected, "test_location")
	for _, result := range results {
		if !result.Passed {
			t.Errorf("Location validation failed: %s", result.Message)
		}
	}
}

func TestAssertURI(t *testing.T) {
	config := DefaultValidationConfig()
	testCase := CreateTestCase("uri-001", "URI test", cases.LSPMethodDefinition)
	ctx := NewAssertionContext(testCase, config)

	testURI := "file:///home/user/project/src/main.go"

	// Test exact match
	exactExpected := &ExpectedURI{Exact: &testURI}
	result := ctx.AssertURI(testURI, exactExpected, "test_uri")
	if !result.Passed {
		t.Errorf("Exact URI match should pass: %s", result.Message)
	}

	// Test contains match
	containsStr := "project/src"
	containsExpected := &ExpectedURI{Contains: &containsStr}
	result2 := ctx.AssertURI(testURI, containsExpected, "test_uri")
	if !result2.Passed {
		t.Errorf("Contains URI match should pass: %s", result2.Message)
	}

	// Test pattern match
	pattern := `file:///.+/main\.go$`
	patternExpected := &ExpectedURI{Pattern: &pattern}
	result3 := ctx.AssertURI(testURI, patternExpected, "test_uri")
	if !result3.Passed {
		t.Errorf("Pattern URI match should pass: %s", result3.Message)
	}

	// Test invalid URI (no file scheme)
	invalidURI := "/home/user/file.go"
	result4 := ctx.AssertURI(invalidURI, nil, "test_uri")
	if result4.Passed {
		t.Errorf("Invalid URI scheme should fail validation")
	}
}

// Test Content Assertions

func TestAssertContains(t *testing.T) {
	config := DefaultValidationConfig()
	testCase := CreateTestCase("contains-001", "Contains test", cases.LSPMethodHover)
	ctx := NewAssertionContext(testCase, config)

	content := "This is a function that returns a value"
	patterns := []string{"function", "returns", "value"}

	results := ctx.AssertContains(content, patterns, "hover_content")
	for _, result := range results {
		if !result.Passed {
			t.Errorf("Contains validation should pass: %s", result.Message)
		}
	}

	// Test missing pattern
	missingPatterns := []string{"missing", "pattern"}
	results2 := ctx.AssertContains(content, missingPatterns, "hover_content")
	for _, result := range results2 {
		if result.Passed {
			t.Errorf("Missing pattern should fail validation: %s", result.Message)
		}
	}
}

func TestAssertMatches(t *testing.T) {
	config := DefaultValidationConfig()
	testCase := CreateTestCase("matches-001", "Matches test", cases.LSPMethodHover)
	ctx := NewAssertionContext(testCase, config)

	content := "func calculateTotal(items []Item) (int, error)"
	patterns := []string{`func \w+\(.*\)`, `\(.*\).*error`}

	results := ctx.AssertMatches(content, patterns, "hover_content")
	for _, result := range results {
		if !result.Passed {
			t.Errorf("Pattern match should pass: %s", result.Message)
		}
	}

	// Test invalid regex
	invalidPatterns := []string{"[invalid"}
	results2 := ctx.AssertMatches(content, invalidPatterns, "hover_content")
	for _, result := range results2 {
		if result.Passed {
			t.Errorf("Invalid regex should fail validation: %s", result.Message)
		}
	}
}

func TestAssertExcludes(t *testing.T) {
	config := DefaultValidationConfig()
	testCase := CreateTestCase("excludes-001", "Excludes test", cases.LSPMethodHover)
	ctx := NewAssertionContext(testCase, config)

	content := "This is production code"
	patterns := []string{"test", "mock", "debug"}

	results := ctx.AssertExcludes(content, patterns, "hover_content")
	for _, result := range results {
		if !result.Passed {
			t.Errorf("Excludes validation should pass: %s", result.Message)
		}
	}

	// Test pattern that should be excluded but is present
	content2 := "This is test code"
	results2 := ctx.AssertExcludes(content2, []string{"test"}, "hover_content")
	for _, result := range results2 {
		if result.Passed {
			t.Errorf("Excluded pattern present should fail validation: %s", result.Message)
		}
	}
}

// Test Array Assertions

func TestAssertArrayLength(t *testing.T) {
	config := DefaultValidationConfig()
	testCase := CreateTestCase("array-001", "Array test", cases.LSPMethodReferences)
	ctx := NewAssertionContext(testCase, config)

	// Test array with exact length expectation
	array := []interface{}{1, 2, 3, 4, 5}
	exactLength := &ExpectedArrayLength{Exact: intPtr(5)}
	
	result := ctx.AssertArrayLength(array, exactLength, "references_array")
	if !result.Passed {
		t.Errorf("Exact length validation should pass: %s", result.Message)
	}

	// Test array with range expectation
	rangeLength := &ExpectedArrayLength{Min: intPtr(3), Max: intPtr(10)}
	result2 := ctx.AssertArrayLength(array, rangeLength, "references_array")
	if !result2.Passed {
		t.Errorf("Range length validation should pass: %s", result2.Message)
	}

	// Test array exceeding max length
	shortMax := &ExpectedArrayLength{Max: intPtr(3)}
	result3 := ctx.AssertArrayLength(array, shortMax, "references_array")
	if result3.Passed {
		t.Errorf("Array exceeding max length should fail validation")
	}

	// Test non-array input
	notArray := "not an array"
	result4 := ctx.AssertArrayLength(notArray, nil, "test_array")
	if result4.Passed {
		t.Errorf("Non-array input should fail validation")
	}
}

// Test Method-Specific Assertions

func TestAssertDefinitionResponse(t *testing.T) {
	config := DefaultValidationConfig()
	testCase := CreateTestCase("def-001", "Definition test", cases.LSPMethodDefinition)
	ctx := NewAssertionContext(testCase, config)

	// Test single location response
	singleLocationResponse := json.RawMessage(`{
		"uri": "file:///path/to/file.go",
		"range": {
			"start": {"line": 10, "character": 5},
			"end": {"line": 10, "character": 15}
		}
	}`)

	expected := &ExpectedDefinitionResult{
		HasResult: true,
		FirstLocation: &ExpectedLocation{
			URI: &ExpectedURI{Contains: stringPtr("file.go")},
		},
	}

	results := ctx.AssertDefinitionResponse(singleLocationResponse, expected)
	for _, result := range results {
		if !result.Passed {
			t.Errorf("Definition validation failed: %s", result.Message)
		}
	}

	// Test null response
	nullResponse := json.RawMessage(`null`)
	expectedNull := &ExpectedDefinitionResult{HasResult: false}
	
	results2 := ctx.AssertDefinitionResponse(nullResponse, expectedNull)
	for _, result := range results2 {
		if !result.Passed {
			t.Errorf("Null definition validation failed: %s", result.Message)
		}
	}

	// Test array response
	arrayResponse := json.RawMessage(`[{
		"uri": "file:///path/to/file.go",
		"range": {
			"start": {"line": 10, "character": 5},
			"end": {"line": 10, "character": 15}
		}
	}]`)

	isArray := true
	expectedArray := &ExpectedDefinitionResult{
		HasResult: true,
		IsArray:   &isArray,
		ArrayLength: &ExpectedArrayLength{Exact: intPtr(1)},
	}

	results3 := ctx.AssertDefinitionResponse(arrayResponse, expectedArray)
	for _, result := range results3 {
		if !result.Passed {
			t.Errorf("Array definition validation failed: %s", result.Message)
		}
	}
}

func TestAssertHoverResponse(t *testing.T) {
	config := DefaultValidationConfig()
	testCase := CreateTestCase("hover-001", "Hover test", cases.LSPMethodHover)
	ctx := NewAssertionContext(testCase, config)

	// Test hover response with markdown content
	hoverResponse := json.RawMessage(`{
		"contents": {
			"kind": "markdown",
			"value": "` + "```" + `go\nfunc calculateTotal() int\n` + "```" + `\n\nCalculates the total value"
		},
		"range": {
			"start": {"line": 8, "character": 4},
			"end": {"line": 8, "character": 16}
		}
	}`)

	expected := &ExpectedHoverResult{
		HasResult: true,
		Content: &ExpectedHoverContent{
			HasContent:   true,
			Contains:     []string{"func calculateTotal", "total value"},
			MarkupKind:   stringPtr("markdown"),
		},
	}

	results := ctx.AssertHoverResponse(hoverResponse, expected)
	for _, result := range results {
		if !result.Passed {
			t.Errorf("Hover validation failed: %s", result.Message)
		}
	}

	// Test null hover response
	nullResponse := json.RawMessage(`null`)
	expectedNull := &ExpectedHoverResult{HasResult: false}
	
	results2 := ctx.AssertHoverResponse(nullResponse, expectedNull)
	for _, result := range results2 {
		if !result.Passed {
			t.Errorf("Null hover validation failed: %s", result.Message)
		}
	}
}

func TestAssertReferencesResponse(t *testing.T) {
	config := DefaultValidationConfig()
	testCase := CreateTestCase("ref-001", "References test", cases.LSPMethodReferences)
	ctx := NewAssertionContext(testCase, config)

	// Test references response with multiple locations
	referencesResponse := json.RawMessage(`[
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

	expected := &ExpectedReferencesResult{
		HasResult:   true,
		ArrayLength: &ExpectedArrayLength{Min: intPtr(1), Max: intPtr(5)},
		ContainsFile: stringPtr("file1.go"),
	}

	results := ctx.AssertReferencesResponse(referencesResponse, expected)
	for _, result := range results {
		if !result.Passed {
			t.Errorf("References validation failed: %s", result.Message)
		}
	}
}

// Test Builder Pattern

func TestExpectedResultBuilder(t *testing.T) {
	// Test complete builder chain
	expected := NewExpectedResult().
		WithDefinition(true).
		WithDefinitionLocation("file:///test.go", 10, 5, 10, 15).
		WithReferences(true).
		WithReferencesCount(2, 10).
		WithHover(true).
		WithHoverContent(true, "function", "test").
		WithDocumentSymbol(true).
		WithDocumentSymbolCount(1, 5).
		WithWorkspaceSymbol(true).
		WithWorkspaceSymbolCount(3, 20).
		WithContains("test").
		WithExcludes("error").
		WithMaxResponseTime(500).
		Build()

	// Verify all fields are set
	if expected.Definition == nil || !expected.Definition.HasResult {
		t.Error("Definition expectation not set correctly")
	}
	
	if expected.References == nil || !expected.References.HasResult {
		t.Error("References expectation not set correctly")
	}
	
	if expected.Hover == nil || !expected.Hover.HasResult {
		t.Error("Hover expectation not set correctly")
	}
	
	if expected.DocumentSymbol == nil || !expected.DocumentSymbol.HasResult {
		t.Error("Document symbol expectation not set correctly")
	}
	
	if expected.WorkspaceSymbol == nil || !expected.WorkspaceSymbol.HasResult {
		t.Error("Workspace symbol expectation not set correctly")
	}
	
	if len(expected.Contains) != 1 || expected.Contains[0] != "test" {
		t.Error("Contains patterns not set correctly")
	}
	
	if len(expected.Excludes) != 1 || expected.Excludes[0] != "error" {
		t.Error("Excludes patterns not set correctly")
	}
	
	if expected.ResponseTime == nil || *expected.ResponseTime != 500 {
		t.Error("Response time not set correctly")
	}
}

// Test Error Handling

func TestErrorValidation(t *testing.T) {
	config := DefaultValidationConfig()
	testCase := CreateTestCase("error-001", "Error test", cases.LSPMethodDefinition)

	// Test error response validation
	errorResponse := json.RawMessage(`{
		"error": {
			"code": -32602,
			"message": "Invalid params: position out of range"
		}
	}`)
	
	expected := NewExpectedResult().
		Success(false).
		WithError(true).
		WithErrorCode(-32602).
		WithErrorMessageContains("Invalid params", "out of range").
		Build()

	validator := NewComprehensiveValidator(config)
	testCase.Response = errorResponse
	
	err := validator.ValidateTestCase(testCase, expected)
	if err != nil {
		t.Errorf("Error validation should not fail: %v", err)
	}

	// Check that error validation passed
	foundErrorValidation := false
	for _, result := range testCase.ValidationResults {
		if result.Name == "error_validation" && result.Passed {
			foundErrorValidation = true
			break
		}
	}
	
	if !foundErrorValidation {
		t.Error("Error validation should have passed")
	}
}

// Test Comprehensive Validator

func TestComprehensiveValidator(t *testing.T) {
	config := DefaultValidationConfig()
	validator := NewComprehensiveValidator(config)

	// Test successful definition validation
	testCase := CreateTestCase("comprehensive-001", "Comprehensive test", cases.LSPMethodDefinition)
	testCase.Response = json.RawMessage(`{
		"uri": "file:///test/main.go",
		"range": {
			"start": {"line": 10, "character": 5},
			"end": {"line": 10, "character": 15}
		}
	}`)

	expected := NewExpectedResult().
		WithDefinition(true).
		WithDefinitionLocation("file:///test/main.go", 10, 5, 10, 15).
		WithContains("main.go").
		Build()

	err := validator.ValidateTestCase(testCase, expected)
	if err != nil {
		t.Errorf("Comprehensive validation should not fail: %v", err)
	}

	if testCase.Status != cases.TestStatusPassed {
		t.Errorf("Test case should have passed status, got: %v", testCase.Status)
	}

	summary := GetValidationSummary(testCase.ValidationResults, false)
	if !summary.AllPassed {
		t.Errorf("All validations should have passed. Summary: %+v", summary)
		
		// Print failed validations for debugging
		for _, result := range testCase.ValidationResults {
			if !result.Passed {
				t.Logf("Failed validation: %s - %s", result.Name, result.Message)
			}
		}
	}
}

// Test Utility Functions

func TestQuickBuilders(t *testing.T) {
	// Test quick definition test
	defTest := QuickDefinitionTest(true, "file:///test.go", 10, 5)
	if defTest.Definition == nil || !defTest.Definition.HasResult {
		t.Error("Quick definition test not created correctly")
	}

	// Test quick references test
	refTest := QuickReferencesTest(true, 1, 10)
	if refTest.References == nil || !refTest.References.HasResult {
		t.Error("Quick references test not created correctly")
	}

	// Test quick hover test
	hoverTest := QuickHoverTest(true, "function", "test")
	if hoverTest.Hover == nil || !hoverTest.Hover.HasResult {
		t.Error("Quick hover test not created correctly")
	}

	// Test quick error test
	errorTest := QuickErrorTest(-32602, "invalid", "error")
	if errorTest.Error == nil || !errorTest.Error.HasError {
		t.Error("Quick error test not created correctly")
	}
}

func TestValidationSummary(t *testing.T) {
	// Create test results
	results := []*cases.ValidationResult{
		{Name: "test1", Passed: true, Message: "Test 1 passed"},
		{Name: "test2", Passed: false, Message: "Test 2 failed"},
		{Name: "test3", Passed: true, Message: "Test 3 passed"},
		{Name: "test4", Passed: false, Message: "Test 4 failed"},
	}

	summary := GetValidationSummary(results, false)
	
	if summary.TotalValidations != 4 {
		t.Errorf("Expected 4 total validations, got %d", summary.TotalValidations)
	}
	
	if summary.PassedValidations != 2 {
		t.Errorf("Expected 2 passed validations, got %d", summary.PassedValidations)
	}
	
	if summary.FailedValidations != 2 {
		t.Errorf("Expected 2 failed validations, got %d", summary.FailedValidations)
	}
	
	if summary.PassRate != 50.0 {
		t.Errorf("Expected 50%% pass rate, got %.1f%%", summary.PassRate)
	}
	
	if summary.AllPassed {
		t.Error("Summary should not indicate all passed when there are failures")
	}
}

// Test helper functions - using the ones defined in examples.go