package assertions

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"lsp-gateway/internal/testing/lsp/cases"
	lspconfig "lsp-gateway/internal/testing/lsp/config"
)

// ComprehensiveValidator provides high-level validation using assertion helpers
type ComprehensiveValidator struct {
	config *lspconfig.ValidationConfig
}

// NewComprehensiveValidator creates a new comprehensive validator
func NewComprehensiveValidator(validationConfig *lspconfig.ValidationConfig) *ComprehensiveValidator {
	if validationConfig == nil {
		validationConfig = &lspconfig.ValidationConfig{
			StrictMode:        false,
			ValidateTypes:     true,
			ValidatePositions: true,
			ValidateURIs:      true,
		}
	}
	
	return &ComprehensiveValidator{
		config: validationConfig,
	}
}

// ValidateTestCase validates a test case using comprehensive assertion helpers
func (v *ComprehensiveValidator) ValidateTestCase(testCase *cases.TestCase, expected *ComprehensiveExpectedResult) error {
	if testCase.Response == nil {
		return v.handleNullResponse(testCase, expected)
	}

	// Create assertion context
	ctx := NewAssertionContext(testCase, v.config)
	
	// Start timing validation
	startTime := time.Now()
	
	// Perform common validations first
	commonResults := v.performCommonValidations(ctx, testCase.Response, expected)
	testCase.ValidationResults = append(testCase.ValidationResults, commonResults...)
	
	// Perform method-specific validation
	methodResults := v.performMethodValidation(ctx, testCase, expected)
	testCase.ValidationResults = append(testCase.ValidationResults, methodResults...)
	
	// Check response time if specified
	if expected != nil && expected.ResponseTime != nil {
		duration := time.Since(startTime)
		timeResult := v.validateResponseTime(duration, *expected.ResponseTime)
		testCase.ValidationResults = append(testCase.ValidationResults, timeResult)
	}
	
	// Determine overall status
	v.updateTestCaseStatus(testCase)
	
	return nil
}

// handleNullResponse handles cases where no response was received
func (v *ComprehensiveValidator) handleNullResponse(testCase *cases.TestCase, expected *ComprehensiveExpectedResult) error {
	if expected == nil || expected.Success {
		// Expected success but got no response
		result := &cases.ValidationResult{
			Name:        "response_presence",
			Description: "Validate response is present",
			Passed:      false,
			Message:     "Expected response but got none",
		}
		testCase.ValidationResults = append(testCase.ValidationResults, result)
		testCase.Status = cases.TestStatusFailed
		return fmt.Errorf("no response received")
	}
	
	// No response expected and none received - this is success for error cases
	result := &cases.ValidationResult{
		Name:        "expected_error",
		Description: "Validate expected error condition",
		Passed:      true,
		Message:     "Expected no response and got none",
	}
	testCase.ValidationResults = append(testCase.ValidationResults, result)
	testCase.Status = cases.TestStatusPassed
	return nil
}

// performCommonValidations performs validation checks common to all methods
func (v *ComprehensiveValidator) performCommonValidations(ctx *AssertionContext, response json.RawMessage, expected *ComprehensiveExpectedResult) []*cases.ValidationResult {
	var results []*cases.ValidationResult
	
	// Basic JSON structure validation
	var responseObj interface{}
	if err := json.Unmarshal(response, &responseObj); err != nil {
		results = append(results, ctx.CreateValidationResult(
			"json_validity",
			"Validate response is valid JSON",
			fmt.Sprintf("Invalid JSON: %v", err),
			false,
			map[string]interface{}{
				"error": err.Error(),
			},
		))
		return results
	}
	
	results = append(results, ctx.CreateValidationResult(
		"json_validity",
		"Validate response is valid JSON",
		"Response is valid JSON",
		true,
		nil,
	))
	
	if expected == nil {
		return results
	}
	
	// Check success expectation
	if !expected.Success {
		// Expected error - check for LSP error response
		if errorResult := v.checkForLSPError(ctx, response, expected.Error); errorResult != nil {
			results = append(results, errorResult)
		}
	} else {
		// Expected success - make sure there's no error
		if errorResult := v.checkForLSPError(ctx, response, nil); errorResult != nil && errorResult.Passed {
			// Found error but expected success
			results = append(results, ctx.CreateValidationResult(
				"success_expectation",
				"Validate successful response expected",
				"Expected success but found error in response",
				false,
				nil,
			))
		} else {
			results = append(results, ctx.CreateValidationResult(
				"success_expectation",
				"Validate successful response expected",
				"Response indicates success as expected",
				true,
				nil,
			))
		}
	}
	
	// Check contains patterns
	if len(expected.Contains) > 0 {
		containsResults := ctx.AssertContains(string(response), expected.Contains, "response")
		results = append(results, containsResults...)
	}
	
	// Check regex matches
	if len(expected.Matches) > 0 {
		matchResults := ctx.AssertMatches(string(response), expected.Matches, "response")
		results = append(results, matchResults...)
	}
	
	// Check excludes patterns
	if len(expected.Excludes) > 0 {
		excludeResults := ctx.AssertExcludes(string(response), expected.Excludes, "response")
		results = append(results, excludeResults...)
	}
	
	return results
}

// checkForLSPError checks if the response contains an LSP error
func (v *ComprehensiveValidator) checkForLSPError(ctx *AssertionContext, response json.RawMessage, expectedError *ExpectedErrorResult) *cases.ValidationResult {
	var errorResponse struct {
		Error *struct {
			Code    int         `json:"code"`
			Message string      `json:"message"`
			Data    interface{} `json:"data,omitempty"`
		} `json:"error,omitempty"`
	}
	
	if err := json.Unmarshal(response, &errorResponse); err != nil {
		return ctx.CreateValidationResult(
			"error_check",
			"Check for LSP error in response",
			fmt.Sprintf("Failed to parse response for error check: %v", err),
			false,
			map[string]interface{}{
				"parse_error": err.Error(),
			},
		)
	}
	
	if errorResponse.Error != nil {
		// Found an error - validate against expectations
		if expectedError != nil {
			return v.validateErrorDetails(ctx, errorResponse.Error, expectedError)
		}
		
		return ctx.CreateValidationResult(
			"error_presence",
			"Check for presence of error in response",
			fmt.Sprintf("Found LSP error: %s (code: %d)", errorResponse.Error.Message, errorResponse.Error.Code),
			true,
			map[string]interface{}{
				"error_code":    errorResponse.Error.Code,
				"error_message": errorResponse.Error.Message,
				"error_data":    errorResponse.Error.Data,
			},
		)
	}
	
	// No error found
	if expectedError != nil && expectedError.HasError {
		return ctx.CreateValidationResult(
			"error_expectation",
			"Validate error response expected",
			"Expected error but none found",
			false,
			nil,
		)
	}
	
	return nil
}

// validateErrorDetails validates error details against expectations
func (v *ComprehensiveValidator) validateErrorDetails(ctx *AssertionContext, actualError *struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}, expected *ExpectedErrorResult) *cases.ValidationResult {
	
	// Check error code
	if expected.Code != nil && actualError.Code != *expected.Code {
		return ctx.CreateValidationResult(
			"error_code",
			"Validate error code matches expected",
			fmt.Sprintf("Error code %d does not match expected %d", actualError.Code, *expected.Code),
			false,
			map[string]interface{}{
				"actual_code":   actualError.Code,
				"expected_code": *expected.Code,
			},
		)
	}
	
	// Check error message
	if expected.Message != nil && actualError.Message != *expected.Message {
		return ctx.CreateValidationResult(
			"error_message",
			"Validate error message matches expected",
			fmt.Sprintf("Error message '%s' does not match expected '%s'", actualError.Message, *expected.Message),
			false,
			map[string]interface{}{
				"actual_message":   actualError.Message,
				"expected_message": *expected.Message,
			},
		)
	}
	
	// Check message contains patterns
	if len(expected.MessageContains) > 0 {
		for _, pattern := range expected.MessageContains {
			if !ctx.AssertContains(actualError.Message, []string{pattern}, "error_message")[0].Passed {
				return ctx.CreateValidationResult(
					"error_message_contains",
					"Validate error message contains expected pattern",
					fmt.Sprintf("Error message '%s' does not contain expected pattern '%s'", actualError.Message, pattern),
					false,
					map[string]interface{}{
						"actual_message":    actualError.Message,
						"expected_pattern":  pattern,
					},
				)
			}
		}
	}
	
	// Check message matches patterns
	if len(expected.MessageMatches) > 0 {
		for _, pattern := range expected.MessageMatches {
			if !ctx.AssertMatches(actualError.Message, []string{pattern}, "error_message")[0].Passed {
				return ctx.CreateValidationResult(
					"error_message_matches",
					"Validate error message matches expected pattern",
					fmt.Sprintf("Error message '%s' does not match expected pattern '%s'", actualError.Message, pattern),
					false,
					map[string]interface{}{
						"actual_message":    actualError.Message,
						"expected_pattern":  pattern,
					},
				)
			}
		}
	}
	
	return ctx.CreateValidationResult(
		"error_validation",
		"Validate error details",
		fmt.Sprintf("Error details validated successfully: %s (code: %d)", actualError.Message, actualError.Code),
		true,
		map[string]interface{}{
			"error_code":    actualError.Code,
			"error_message": actualError.Message,
		},
	)
}

// performMethodValidation performs method-specific validation
func (v *ComprehensiveValidator) performMethodValidation(ctx *AssertionContext, testCase *cases.TestCase, expected *ComprehensiveExpectedResult) []*cases.ValidationResult {
	if expected == nil {
		return []*cases.ValidationResult{}
	}
	
	switch testCase.Method {
	case cases.LSPMethodDefinition:
		if expected.Definition != nil {
			return ctx.AssertDefinitionResponse(testCase.Response, expected.Definition)
		}
	case cases.LSPMethodReferences:
		if expected.References != nil {
			return ctx.AssertReferencesResponse(testCase.Response, expected.References)
		}
	case cases.LSPMethodHover:
		if expected.Hover != nil {
			return ctx.AssertHoverResponse(testCase.Response, expected.Hover)
		}
	case cases.LSPMethodDocumentSymbol:
		if expected.DocumentSymbol != nil {
			return ctx.AssertDocumentSymbolResponse(testCase.Response, expected.DocumentSymbol)
		}
	case cases.LSPMethodWorkspaceSymbol:
		if expected.WorkspaceSymbol != nil {
			return ctx.AssertWorkspaceSymbolResponse(testCase.Response, expected.WorkspaceSymbol)
		}
	}
	
	// No method-specific validation performed
	return []*cases.ValidationResult{
		ctx.CreateValidationResult(
			"method_validation",
			"Perform method-specific validation",
			fmt.Sprintf("No specific validation configured for method: %s", testCase.Method),
			true,
			map[string]interface{}{
				"method": testCase.Method,
			},
		),
	}
}

// validateResponseTime validates response time against expected maximum
func (v *ComprehensiveValidator) validateResponseTime(actualDuration time.Duration, maxMs int) *cases.ValidationResult {
	maxDuration := time.Duration(maxMs) * time.Millisecond
	actualMs := actualDuration.Milliseconds()
	
	if actualDuration > maxDuration {
		return &cases.ValidationResult{
			Name:        "response_time",
			Description: "Validate response time is within expected bounds",
			Passed:      false,
			Message:     fmt.Sprintf("Response time %dms exceeds maximum expected %dms", actualMs, maxMs),
			Details: map[string]interface{}{
				"actual_ms":   actualMs,
				"expected_max_ms": maxMs,
				"exceeded_by_ms":  actualMs - int64(maxMs),
			},
		}
	}
	
	return &cases.ValidationResult{
		Name:        "response_time",
		Description: "Validate response time is within expected bounds",
		Passed:      true,
		Message:     fmt.Sprintf("Response time %dms is within expected maximum %dms", actualMs, maxMs),
		Details: map[string]interface{}{
			"actual_ms":     actualMs,
			"expected_max_ms": maxMs,
		},
	}
}

// updateTestCaseStatus updates the test case status based on validation results
func (v *ComprehensiveValidator) updateTestCaseStatus(testCase *cases.TestCase) {
	allPassed := true
	for _, result := range testCase.ValidationResults {
		if !result.Passed {
			allPassed = false
			break
		}
	}
	
	if allPassed {
		testCase.Status = cases.TestStatusPassed
	} else {
		testCase.Status = cases.TestStatusFailed
	}
}

// ValidationSummary provides a summary of validation results
type ValidationSummary struct {
	TotalValidations int                        `json:"total_validations"`
	PassedValidations int                       `json:"passed_validations"`
	FailedValidations int                       `json:"failed_validations"`
	PassRate         float64                    `json:"pass_rate"`
	AllPassed        bool                       `json:"all_passed"`
	FailuresByType   map[string]int             `json:"failures_by_type"`
	Details          []*cases.ValidationResult  `json:"details,omitempty"`
}

// GetValidationSummary returns a comprehensive summary of validation results
func GetValidationSummary(results []*cases.ValidationResult, includeDetails bool) *ValidationSummary {
	total := len(results)
	passed := 0
	failed := 0
	failuresByType := make(map[string]int)
	
	for _, result := range results {
		if result.Passed {
			passed++
		} else {
			failed++
			// Extract failure type from result name
			failureType := extractFailureType(result.Name)
			failuresByType[failureType]++
		}
	}
	
	passRate := float64(0)
	if total > 0 {
		passRate = float64(passed) / float64(total) * 100
	}
	
	summary := &ValidationSummary{
		TotalValidations:  total,
		PassedValidations: passed,
		FailedValidations: failed,
		PassRate:         passRate,
		AllPassed:        failed == 0,
		FailuresByType:   failuresByType,
	}
	
	if includeDetails {
		summary.Details = results
	}
	
	return summary
}

// extractFailureType extracts the type of failure from the result name
func extractFailureType(name string) string {
	// Extract the general category from the validation name
	parts := strings.Split(name, "_")
	if len(parts) > 0 {
		return parts[0]
	}
	return "unknown"
}

// CompareValidationResults compares two sets of validation results
func CompareValidationResults(baseline, current []*cases.ValidationResult) *ValidationComparison {
	baselineMap := make(map[string]*cases.ValidationResult)
	for _, result := range baseline {
		baselineMap[result.Name] = result
	}
	
	currentMap := make(map[string]*cases.ValidationResult)
	for _, result := range current {
		currentMap[result.Name] = result
	}
	
	comparison := &ValidationComparison{
		Added:     make([]*cases.ValidationResult, 0),
		Removed:   make([]*cases.ValidationResult, 0),
		Improved:  make([]*ValidationChange, 0),
		Regressed: make([]*ValidationChange, 0),
		Unchanged: make([]*cases.ValidationResult, 0),
	}
	
	// Check for added and improved/regressed
	for name, currentResult := range currentMap {
		if baselineResult, exists := baselineMap[name]; exists {
			if baselineResult.Passed != currentResult.Passed {
				change := &ValidationChange{
					Name:     name,
					Before:   baselineResult,
					After:    currentResult,
					Improved: !baselineResult.Passed && currentResult.Passed,
				}
				
				if change.Improved {
					comparison.Improved = append(comparison.Improved, change)
				} else {
					comparison.Regressed = append(comparison.Regressed, change)
				}
			} else {
				comparison.Unchanged = append(comparison.Unchanged, currentResult)
			}
		} else {
			comparison.Added = append(comparison.Added, currentResult)
		}
	}
	
	// Check for removed
	for name, baselineResult := range baselineMap {
		if _, exists := currentMap[name]; !exists {
			comparison.Removed = append(comparison.Removed, baselineResult)
		}
	}
	
	return comparison
}

// ValidationComparison represents a comparison between two sets of validation results
type ValidationComparison struct {
	Added     []*cases.ValidationResult `json:"added"`
	Removed   []*cases.ValidationResult `json:"removed"`
	Improved  []*ValidationChange       `json:"improved"`
	Regressed []*ValidationChange       `json:"regressed"`
	Unchanged []*cases.ValidationResult `json:"unchanged"`
}

// ValidationChange represents a change in validation result
type ValidationChange struct {
	Name     string                    `json:"name"`
	Before   *cases.ValidationResult   `json:"before"`
	After    *cases.ValidationResult   `json:"after"`
	Improved bool                     `json:"improved"`
}

// Summary returns a summary of the comparison
func (c *ValidationComparison) Summary() string {
	return fmt.Sprintf("Added: %d, Removed: %d, Improved: %d, Regressed: %d, Unchanged: %d",
		len(c.Added), len(c.Removed), len(c.Improved), len(c.Regressed), len(c.Unchanged))
}