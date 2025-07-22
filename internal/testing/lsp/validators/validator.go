package validators

import (
	"encoding/json"
	"fmt"
	"strings"

	"lsp-gateway/internal/testing/lsp/cases"
	"lsp-gateway/internal/testing/lsp/config"
)

// ResponseValidator validates LSP responses against expected results
type ResponseValidator interface {
	ValidateResponse(testCase *cases.TestCase, response json.RawMessage) []*cases.ValidationResult
}

// LSPResponseValidator orchestrates validation for different LSP methods
type LSPResponseValidator struct {
	validators map[string]ResponseValidator
	config     *config.ValidationConfig
}

// NewLSPResponseValidator creates a new response validator
func NewLSPResponseValidator(validationConfig *config.ValidationConfig) *LSPResponseValidator {
	validator := &LSPResponseValidator{
		validators: make(map[string]ResponseValidator),
		config:     validationConfig,
	}

	// Register validators for each LSP method
	validator.validators[cases.LSPMethodDefinition] = NewDefinitionValidator(validationConfig)
	validator.validators[cases.LSPMethodReferences] = NewReferencesValidator(validationConfig)
	validator.validators[cases.LSPMethodHover] = NewHoverValidator(validationConfig)
	validator.validators[cases.LSPMethodDocumentSymbol] = NewDocumentSymbolValidator(validationConfig)
	validator.validators[cases.LSPMethodWorkspaceSymbol] = NewWorkspaceSymbolValidator(validationConfig)

	return validator
}

// ValidateTestCase validates a test case response
func (v *LSPResponseValidator) ValidateTestCase(testCase *cases.TestCase) error {
	if testCase.Response == nil {
		if testCase.Expected == nil || testCase.Expected.Success {
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
		return nil
	}

	// Get appropriate validator for the method
	validator, exists := v.validators[testCase.Method]
	if !exists {
		result := &cases.ValidationResult{
			Name:        "method_support",
			Description: "Check if method is supported for validation",
			Passed:      false,
			Message:     fmt.Sprintf("No validator available for method: %s", testCase.Method),
		}
		testCase.ValidationResults = append(testCase.ValidationResults, result)
		testCase.Status = cases.TestStatusError
		return fmt.Errorf("unsupported method for validation: %s", testCase.Method)
	}

	// Perform validation
	validationResults := validator.ValidateResponse(testCase, testCase.Response)
	testCase.ValidationResults = append(testCase.ValidationResults, validationResults...)

	// Perform common validations
	commonResults := v.performCommonValidations(testCase, testCase.Response)
	testCase.ValidationResults = append(testCase.ValidationResults, commonResults...)

	// Determine overall status
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

	return nil
}

// performCommonValidations performs validation checks common to all methods
func (v *LSPResponseValidator) performCommonValidations(testCase *cases.TestCase, response json.RawMessage) []*cases.ValidationResult {
	var results []*cases.ValidationResult

	// Basic JSON structure validation
	var responseObj interface{}
	if err := json.Unmarshal(response, &responseObj); err != nil {
		results = append(results, &cases.ValidationResult{
			Name:        "json_validity",
			Description: "Validate response is valid JSON",
			Passed:      false,
			Message:     fmt.Sprintf("Invalid JSON: %v", err),
		})
		return results
	}

	results = append(results, &cases.ValidationResult{
		Name:        "json_validity",
		Description: "Validate response is valid JSON",
		Passed:      true,
		Message:     "Response is valid JSON",
	})

	// Check if expected to be successful
	if testCase.Expected != nil {
		if testCase.Expected.Success {
			// Check for LSP error response
			if errorResult := v.checkForLSPError(response); errorResult != nil {
				results = append(results, errorResult)
			} else {
				results = append(results, &cases.ValidationResult{
					Name:        "success_expectation",
					Description: "Validate successful response expected",
					Passed:      true,
					Message:     "Response indicates success as expected",
				})
			}
		} else {
			// Expected error - check if we got one
			if errorResult := v.checkForLSPError(response); errorResult != nil && errorResult.Passed {
				results = append(results, &cases.ValidationResult{
					Name:        "error_expectation",
					Description: "Validate error response expected",
					Passed:      true,
					Message:     "Got expected error response",
				})
			} else {
				results = append(results, &cases.ValidationResult{
					Name:        "error_expectation",
					Description: "Validate error response expected",
					Passed:      false,
					Message:     "Expected error but got successful response",
				})
			}
		}

		// Check contains/excludes patterns
		if len(testCase.Expected.Contains) > 0 {
			results = append(results, v.validateContainsPatterns(response, testCase.Expected.Contains)...)
		}

		if len(testCase.Expected.Excludes) > 0 {
			results = append(results, v.validateExcludesPatterns(response, testCase.Expected.Excludes)...)
		}
	}

	return results
}

// checkForLSPError checks if the response contains an LSP error
func (v *LSPResponseValidator) checkForLSPError(response json.RawMessage) *cases.ValidationResult {
	var errorResponse struct {
		Error *struct {
			Code    int         `json:"code"`
			Message string      `json:"message"`
			Data    interface{} `json:"data,omitempty"`
		} `json:"error,omitempty"`
	}

	if err := json.Unmarshal(response, &errorResponse); err != nil {
		return &cases.ValidationResult{
			Name:        "error_check",
			Description: "Check for LSP error in response",
			Passed:      false,
			Message:     fmt.Sprintf("Failed to parse response for error check: %v", err),
		}
	}

	if errorResponse.Error != nil {
		return &cases.ValidationResult{
			Name:        "error_presence",
			Description: "Check for presence of error in response",
			Passed:      true, // Passed means we found the error as expected
			Message:     fmt.Sprintf("Found LSP error: %s (code: %d)", errorResponse.Error.Message, errorResponse.Error.Code),
			Details: map[string]interface{}{
				"error_code":    errorResponse.Error.Code,
				"error_message": errorResponse.Error.Message,
				"error_data":    errorResponse.Error.Data,
			},
		}
	}

	return nil
}

// validateContainsPatterns validates that response contains expected patterns
func (v *LSPResponseValidator) validateContainsPatterns(response json.RawMessage, patterns []string) []*cases.ValidationResult {
	var results []*cases.ValidationResult
	responseStr := string(response)

	for _, pattern := range patterns {
		found := strings.Contains(responseStr, pattern)
		results = append(results, &cases.ValidationResult{
			Name:        fmt.Sprintf("contains_%s", pattern),
			Description: fmt.Sprintf("Validate response contains pattern: %s", pattern),
			Passed:      found,
			Message:     fmt.Sprintf("Pattern '%s' found in response: %t", pattern, found),
		})
	}

	return results
}

// validateExcludesPatterns validates that response excludes certain patterns
func (v *LSPResponseValidator) validateExcludesPatterns(response json.RawMessage, patterns []string) []*cases.ValidationResult {
	var results []*cases.ValidationResult
	responseStr := string(response)

	for _, pattern := range patterns {
		found := strings.Contains(responseStr, pattern)
		results = append(results, &cases.ValidationResult{
			Name:        fmt.Sprintf("excludes_%s", pattern),
			Description: fmt.Sprintf("Validate response excludes pattern: %s", pattern),
			Passed:      !found,
			Message:     fmt.Sprintf("Pattern '%s' excluded from response: %t", pattern, !found),
		})
	}

	return results
}

// BaseValidator provides common validation functionality
type BaseValidator struct {
	config *config.ValidationConfig
}

// NewBaseValidator creates a new base validator
func NewBaseValidator(config *config.ValidationConfig) *BaseValidator {
	return &BaseValidator{config: config}
}

// validateURI validates URI format
func (v *BaseValidator) validateURI(uri string, fieldName string) *cases.ValidationResult {
	if !v.config.ValidateURIs {
		return &cases.ValidationResult{
			Name:        fmt.Sprintf("%s_uri_validation", fieldName),
			Description: fmt.Sprintf("Validate %s URI format (skipped)", fieldName),
			Passed:      true,
			Message:     "URI validation is disabled",
		}
	}

	if uri == "" {
		return &cases.ValidationResult{
			Name:        fmt.Sprintf("%s_uri_presence", fieldName),
			Description: fmt.Sprintf("Validate %s URI is present", fieldName),
			Passed:      false,
			Message:     fmt.Sprintf("%s URI is empty", fieldName),
		}
	}

	if !strings.HasPrefix(uri, "file://") {
		return &cases.ValidationResult{
			Name:        fmt.Sprintf("%s_uri_scheme", fieldName),
			Description: fmt.Sprintf("Validate %s URI has file scheme", fieldName),
			Passed:      false,
			Message:     fmt.Sprintf("%s URI does not have file:// scheme: %s", fieldName, uri),
		}
	}

	return &cases.ValidationResult{
		Name:        fmt.Sprintf("%s_uri_format", fieldName),
		Description: fmt.Sprintf("Validate %s URI format", fieldName),
		Passed:      true,
		Message:     fmt.Sprintf("%s URI format is valid: %s", fieldName, uri),
	}
}

// validatePosition validates LSP position format
func (v *BaseValidator) validatePosition(pos interface{}, fieldName string) *cases.ValidationResult {
	if !v.config.ValidatePositions {
		return &cases.ValidationResult{
			Name:        fmt.Sprintf("%s_position_validation", fieldName),
			Description: fmt.Sprintf("Validate %s position format (skipped)", fieldName),
			Passed:      true,
			Message:     "Position validation is disabled",
		}
	}

	posMap, ok := pos.(map[string]interface{})
	if !ok {
		return &cases.ValidationResult{
			Name:        fmt.Sprintf("%s_position_type", fieldName),
			Description: fmt.Sprintf("Validate %s position is an object", fieldName),
			Passed:      false,
			Message:     fmt.Sprintf("%s position is not an object", fieldName),
		}
	}

	line, hasLine := posMap["line"]
	character, hasCharacter := posMap["character"]

	if !hasLine || !hasCharacter {
		return &cases.ValidationResult{
			Name:        fmt.Sprintf("%s_position_fields", fieldName),
			Description: fmt.Sprintf("Validate %s position has required fields", fieldName),
			Passed:      false,
			Message:     fmt.Sprintf("%s position missing required fields (line: %t, character: %t)", fieldName, hasLine, hasCharacter),
		}
	}

	// Validate line and character are numbers
	if _, ok := line.(float64); !ok {
		return &cases.ValidationResult{
			Name:        fmt.Sprintf("%s_position_line_type", fieldName),
			Description: fmt.Sprintf("Validate %s position line is a number", fieldName),
			Passed:      false,
			Message:     fmt.Sprintf("%s position line is not a number: %v", fieldName, line),
		}
	}

	if _, ok := character.(float64); !ok {
		return &cases.ValidationResult{
			Name:        fmt.Sprintf("%s_position_character_type", fieldName),
			Description: fmt.Sprintf("Validate %s position character is a number", fieldName),
			Passed:      false,
			Message:     fmt.Sprintf("%s position character is not a number: %v", fieldName, character),
		}
	}

	return &cases.ValidationResult{
		Name:        fmt.Sprintf("%s_position_format", fieldName),
		Description: fmt.Sprintf("Validate %s position format", fieldName),
		Passed:      true,
		Message:     fmt.Sprintf("%s position format is valid", fieldName),
		Details: map[string]interface{}{
			"line":      line,
			"character": character,
		},
	}
}

// GetValidationSummary returns a summary of validation results
func GetValidationSummary(results []*cases.ValidationResult) map[string]interface{} {
	total := len(results)
	passed := 0
	failed := 0

	for _, result := range results {
		if result.Passed {
			passed++
		} else {
			failed++
		}
	}

	return map[string]interface{}{
		"total":      total,
		"passed":     passed,
		"failed":     failed,
		"pass_rate":  float64(passed) / float64(total) * 100,
		"all_passed": failed == 0,
	}
}
