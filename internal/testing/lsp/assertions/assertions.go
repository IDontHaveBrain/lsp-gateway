package assertions

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"lsp-gateway/internal/testing/lsp/cases"
	lspconfig "lsp-gateway/internal/testing/lsp/config"
)

// AssertionContext provides context for assertions with configuration and helpers
type AssertionContext struct {
	config          *lspconfig.ValidationConfig
	testCase        *cases.TestCase
	failureMessages []string
}

// NewAssertionContext creates a new assertion context
func NewAssertionContext(testCase *cases.TestCase, config *lspconfig.ValidationConfig) *AssertionContext {
	if config == nil {
		defaultConfig := lspconfig.ValidationConfig{
			StrictMode:        false,
			ValidateTypes:     true,
			ValidatePositions: true,
			ValidateURIs:      true,
		}
		config = &defaultConfig
	}

	return &AssertionContext{
		config:          config,
		testCase:        testCase,
		failureMessages: make([]string, 0),
	}
}

// CreateValidationResult creates a validation result with the given parameters
func (ctx *AssertionContext) CreateValidationResult(name, description, message string, passed bool, details map[string]interface{}) *cases.ValidationResult {
	return &cases.ValidationResult{
		Name:        name,
		Description: description,
		Passed:      passed,
		Message:     message,
		Details:     details,
	}
}

// Position Assertion Helpers

// AssertPosition validates an LSP position structure
func (ctx *AssertionContext) AssertPosition(actual interface{}, expected *ExpectedPosition, fieldName string) *cases.ValidationResult {
	if !ctx.config.ValidatePositions {
		return ctx.CreateValidationResult(
			fmt.Sprintf("%s_position_validation", fieldName),
			fmt.Sprintf("Validate %s position format (skipped)", fieldName),
			"Position validation is disabled",
			true,
			nil,
		)
	}

	posMap, ok := actual.(map[string]interface{})
	if !ok {
		return ctx.CreateValidationResult(
			fmt.Sprintf("%s_position_type", fieldName),
			fmt.Sprintf("Validate %s position is an object", fieldName),
			fmt.Sprintf("%s position is not an object", fieldName),
			false,
			nil,
		)
	}

	line, hasLine := posMap["line"]
	character, hasCharacter := posMap["character"]

	if !hasLine || !hasCharacter {
		return ctx.CreateValidationResult(
			fmt.Sprintf("%s_position_fields", fieldName),
			fmt.Sprintf("Validate %s position has required fields", fieldName),
			fmt.Sprintf("%s position missing required fields (line: %t, character: %t)", fieldName, hasLine, hasCharacter),
			false,
			nil,
		)
	}

	// Type validation
	lineNum, lineOk := line.(float64)
	charNum, charOk := character.(float64)

	if !lineOk || !charOk {
		return ctx.CreateValidationResult(
			fmt.Sprintf("%s_position_types", fieldName),
			fmt.Sprintf("Validate %s position field types", fieldName),
			fmt.Sprintf("%s position fields are not numbers (line: %t, character: %t)", fieldName, lineOk, charOk),
			false,
			nil,
		)
	}

	// Expected value validation
	if expected != nil {
		if expected.Line != nil && int(lineNum) != *expected.Line {
			return ctx.CreateValidationResult(
				fmt.Sprintf("%s_position_line", fieldName),
				fmt.Sprintf("Validate %s position line matches expected", fieldName),
				fmt.Sprintf("Line %d does not match expected %d", int(lineNum), *expected.Line),
				false,
				map[string]interface{}{
					"actual_line":   int(lineNum),
					"expected_line": *expected.Line,
				},
			)
		}

		if expected.Character != nil && int(charNum) != *expected.Character {
			return ctx.CreateValidationResult(
				fmt.Sprintf("%s_position_character", fieldName),
				fmt.Sprintf("Validate %s position character matches expected", fieldName),
				fmt.Sprintf("Character %d does not match expected %d", int(charNum), *expected.Character),
				false,
				map[string]interface{}{
					"actual_character":   int(charNum),
					"expected_character": *expected.Character,
				},
			)
		}
	}

	return ctx.CreateValidationResult(
		fmt.Sprintf("%s_position_format", fieldName),
		fmt.Sprintf("Validate %s position format", fieldName),
		fmt.Sprintf("%s position format is valid", fieldName),
		true,
		map[string]interface{}{
			"line":      int(lineNum),
			"character": int(charNum),
		},
	)
}

// AssertRange validates an LSP range structure
func (ctx *AssertionContext) AssertRange(actual interface{}, expected *ExpectedRange, fieldName string) []*cases.ValidationResult {
	var results []*cases.ValidationResult

	rangeMap, ok := actual.(map[string]interface{})
	if !ok {
		results = append(results, ctx.CreateValidationResult(
			fmt.Sprintf("%s_range_type", fieldName),
			fmt.Sprintf("Validate %s range is an object", fieldName),
			fmt.Sprintf("%s range is not an object", fieldName),
			false,
			nil,
		))
		return results
	}

	start, hasStart := rangeMap["start"]
	end, hasEnd := rangeMap["end"]

	if !hasStart || !hasEnd {
		results = append(results, ctx.CreateValidationResult(
			fmt.Sprintf("%s_range_fields", fieldName),
			fmt.Sprintf("Validate %s range has required fields", fieldName),
			fmt.Sprintf("%s range missing required fields (start: %t, end: %t)", fieldName, hasStart, hasEnd),
			false,
			nil,
		))
		return results
	}

	// Validate start position
	var expectedStart *ExpectedPosition
	if expected != nil && expected.Start != nil {
		expectedStart = expected.Start
	}
	startResult := ctx.AssertPosition(start, expectedStart, fmt.Sprintf("%s_start", fieldName))
	results = append(results, startResult)

	// Validate end position
	var expectedEnd *ExpectedPosition
	if expected != nil && expected.End != nil {
		expectedEnd = expected.End
	}
	endResult := ctx.AssertPosition(end, expectedEnd, fmt.Sprintf("%s_end", fieldName))
	results = append(results, endResult)

	// Validate range order (start should be before or equal to end)
	if ctx.config.ValidatePositions && startResult.Passed && endResult.Passed {
		startMap := start.(map[string]interface{})
		endMap := end.(map[string]interface{})

		startLine := int(startMap["line"].(float64))
		startChar := int(startMap["character"].(float64))
		endLine := int(endMap["line"].(float64))
		endChar := int(endMap["character"].(float64))

		if startLine > endLine || (startLine == endLine && startChar > endChar) {
			results = append(results, ctx.CreateValidationResult(
				fmt.Sprintf("%s_order", fieldName),
				fmt.Sprintf("Validate %s start position is before end position", fieldName),
				fmt.Sprintf("Start position (%d,%d) is after end position (%d,%d)", startLine, startChar, endLine, endChar),
				false,
				map[string]interface{}{
					"start_line": startLine,
					"start_char": startChar,
					"end_line":   endLine,
					"end_char":   endChar,
				},
			))
		} else {
			results = append(results, ctx.CreateValidationResult(
				fmt.Sprintf("%s_order", fieldName),
				fmt.Sprintf("Validate %s position order", fieldName),
				"Range positions are in correct order",
				true,
				nil,
			))
		}
	}

	return results
}

// AssertLocation validates an LSP location structure
func (ctx *AssertionContext) AssertLocation(actual interface{}, expected *ExpectedLocation, fieldName string) []*cases.ValidationResult {
	var results []*cases.ValidationResult

	locationMap, ok := actual.(map[string]interface{})
	if !ok {
		results = append(results, ctx.CreateValidationResult(
			fmt.Sprintf("%s_location_type", fieldName),
			fmt.Sprintf("Validate %s location is an object", fieldName),
			fmt.Sprintf("%s location is not an object", fieldName),
			false,
			nil,
		))
		return results
	}

	uri, hasURI := locationMap["uri"]
	lspRange, hasRange := locationMap["range"]

	if !hasURI || !hasRange {
		results = append(results, ctx.CreateValidationResult(
			fmt.Sprintf("%s_location_fields", fieldName),
			fmt.Sprintf("Validate %s location has required fields", fieldName),
			fmt.Sprintf("%s location missing required fields (uri: %t, range: %t)", fieldName, hasURI, hasRange),
			false,
			nil,
		))
		return results
	}

	// Validate URI
	if uriStr, ok := uri.(string); ok {
		var expectedURI *ExpectedURI
		if expected != nil && expected.URI != nil {
			expectedURI = expected.URI
		}
		uriResult := ctx.AssertURI(uriStr, expectedURI, fmt.Sprintf("%s_location", fieldName))
		results = append(results, uriResult)
	} else {
		results = append(results, ctx.CreateValidationResult(
			fmt.Sprintf("%s_location_uri_type", fieldName),
			fmt.Sprintf("Validate %s location URI is a string", fieldName),
			fmt.Sprintf("%s location URI is not a string", fieldName),
			false,
			nil,
		))
	}

	// Validate range
	var expectedRange *ExpectedRange
	if expected != nil && expected.Range != nil {
		expectedRange = expected.Range
	}
	rangeResults := ctx.AssertRange(lspRange, expectedRange, fmt.Sprintf("%s_range", fieldName))
	results = append(results, rangeResults...)

	return results
}

// AssertURI validates a URI string
func (ctx *AssertionContext) AssertURI(actual string, expected *ExpectedURI, fieldName string) *cases.ValidationResult {
	if !ctx.config.ValidateURIs {
		return ctx.CreateValidationResult(
			fmt.Sprintf("%s_uri_validation", fieldName),
			fmt.Sprintf("Validate %s URI format (skipped)", fieldName),
			"URI validation is disabled",
			true,
			nil,
		)
	}

	if actual == "" {
		return ctx.CreateValidationResult(
			fmt.Sprintf("%s_uri_presence", fieldName),
			fmt.Sprintf("Validate %s URI is present", fieldName),
			fmt.Sprintf("%s URI is empty", fieldName),
			false,
			nil,
		)
	}

	// Basic scheme validation
	if !strings.HasPrefix(actual, "file://") {
		return ctx.CreateValidationResult(
			fmt.Sprintf("%s_uri_scheme", fieldName),
			fmt.Sprintf("Validate %s URI has file scheme", fieldName),
			fmt.Sprintf("%s URI does not have file:// scheme: %s", fieldName, actual),
			false,
			map[string]interface{}{
				"actual_uri": actual,
			},
		)
	}

	// Expected value validation
	if expected != nil {
		if expected.Exact != nil && actual != *expected.Exact {
			return ctx.CreateValidationResult(
				fmt.Sprintf("%s_uri_exact", fieldName),
				fmt.Sprintf("Validate %s URI matches exact expected value", fieldName),
				fmt.Sprintf("URI %s does not match expected %s", actual, *expected.Exact),
				false,
				map[string]interface{}{
					"actual_uri":   actual,
					"expected_uri": *expected.Exact,
				},
			)
		}

		if expected.Contains != nil && !strings.Contains(actual, *expected.Contains) {
			return ctx.CreateValidationResult(
				fmt.Sprintf("%s_uri_contains", fieldName),
				fmt.Sprintf("Validate %s URI contains expected substring", fieldName),
				fmt.Sprintf("URI %s does not contain expected substring %s", actual, *expected.Contains),
				false,
				map[string]interface{}{
					"actual_uri":         actual,
					"expected_substring": *expected.Contains,
				},
			)
		}

		if expected.Pattern != nil {
			if matched, err := regexp.MatchString(*expected.Pattern, actual); err != nil {
				return ctx.CreateValidationResult(
					fmt.Sprintf("%s_uri_pattern_error", fieldName),
					fmt.Sprintf("Validate %s URI matches pattern (error)", fieldName),
					fmt.Sprintf("Pattern matching error: %v", err),
					false,
					map[string]interface{}{
						"actual_uri": actual,
						"pattern":    *expected.Pattern,
						"error":      err.Error(),
					},
				)
			} else if !matched {
				return ctx.CreateValidationResult(
					fmt.Sprintf("%s_uri_pattern", fieldName),
					fmt.Sprintf("Validate %s URI matches pattern", fieldName),
					fmt.Sprintf("URI %s does not match pattern %s", actual, *expected.Pattern),
					false,
					map[string]interface{}{
						"actual_uri": actual,
						"pattern":    *expected.Pattern,
					},
				)
			}
		}
	}

	return ctx.CreateValidationResult(
		fmt.Sprintf("%s_uri_format", fieldName),
		fmt.Sprintf("Validate %s URI format", fieldName),
		fmt.Sprintf("%s URI format is valid: %s", fieldName, actual),
		true,
		map[string]interface{}{
			"uri": actual,
		},
	)
}

// Content Validation Helpers

// AssertContains validates that content contains expected patterns
func (ctx *AssertionContext) AssertContains(content string, patterns []string, fieldName string) []*cases.ValidationResult {
	var results []*cases.ValidationResult

	for i, pattern := range patterns {
		found := strings.Contains(content, pattern)
		results = append(results, ctx.CreateValidationResult(
			fmt.Sprintf("%s_contains_%d", fieldName, i),
			fmt.Sprintf("Validate %s contains pattern: %s", fieldName, pattern),
			fmt.Sprintf("Pattern '%s' found in %s: %t", pattern, fieldName, found),
			found,
			map[string]interface{}{
				"pattern": pattern,
				"found":   found,
			},
		))
	}

	return results
}

// AssertMatches validates content against regex patterns
func (ctx *AssertionContext) AssertMatches(content string, patterns []string, fieldName string) []*cases.ValidationResult {
	var results []*cases.ValidationResult

	for i, pattern := range patterns {
		matched, err := regexp.MatchString(pattern, content)
		if err != nil {
			results = append(results, ctx.CreateValidationResult(
				fmt.Sprintf("%s_pattern_error_%d", fieldName, i),
				fmt.Sprintf("Validate %s matches pattern: %s (error)", fieldName, pattern),
				fmt.Sprintf("Pattern matching error: %v", err),
				false,
				map[string]interface{}{
					"pattern": pattern,
					"error":   err.Error(),
				},
			))
		} else {
			results = append(results, ctx.CreateValidationResult(
				fmt.Sprintf("%s_matches_%d", fieldName, i),
				fmt.Sprintf("Validate %s matches pattern: %s", fieldName, pattern),
				fmt.Sprintf("Pattern '%s' matches %s: %t", pattern, fieldName, matched),
				matched,
				map[string]interface{}{
					"pattern": pattern,
					"matched": matched,
				},
			))
		}
	}

	return results
}

// AssertExcludes validates that content excludes certain patterns
func (ctx *AssertionContext) AssertExcludes(content string, patterns []string, fieldName string) []*cases.ValidationResult {
	var results []*cases.ValidationResult

	for i, pattern := range patterns {
		found := strings.Contains(content, pattern)
		results = append(results, ctx.CreateValidationResult(
			fmt.Sprintf("%s_excludes_%d", fieldName, i),
			fmt.Sprintf("Validate %s excludes pattern: %s", fieldName, pattern),
			fmt.Sprintf("Pattern '%s' excluded from %s: %t", pattern, fieldName, !found),
			!found,
			map[string]interface{}{
				"pattern":  pattern,
				"excluded": !found,
			},
		))
	}

	return results
}

// Array Validation Helpers

// AssertArrayLength validates array length against expected bounds
func (ctx *AssertionContext) AssertArrayLength(actual interface{}, expected *ExpectedArrayLength, fieldName string) *cases.ValidationResult {
	// Use reflection to handle both []interface{} and other slice types
	actualValue := reflect.ValueOf(actual)
	if actualValue.Kind() != reflect.Slice && actualValue.Kind() != reflect.Array {
		return ctx.CreateValidationResult(
			fmt.Sprintf("%s_array_type", fieldName),
			fmt.Sprintf("Validate %s is an array", fieldName),
			fmt.Sprintf("%s is not an array or slice", fieldName),
			false,
			nil,
		)
	}

	length := actualValue.Len()

	if expected == nil {
		return ctx.CreateValidationResult(
			fmt.Sprintf("%s_array_length", fieldName),
			fmt.Sprintf("Validate %s array length", fieldName),
			fmt.Sprintf("%s has %d items", fieldName, length),
			true,
			map[string]interface{}{
				"length": length,
			},
		)
	}

	// Validate exact length
	if expected.Exact != nil && length != *expected.Exact {
		return ctx.CreateValidationResult(
			fmt.Sprintf("%s_array_exact_length", fieldName),
			fmt.Sprintf("Validate %s array has exact length", fieldName),
			fmt.Sprintf("Array has %d items, expected exactly %d", length, *expected.Exact),
			false,
			map[string]interface{}{
				"actual_length":   length,
				"expected_length": *expected.Exact,
			},
		)
	}

	// Validate minimum length
	if expected.Min != nil && length < *expected.Min {
		return ctx.CreateValidationResult(
			fmt.Sprintf("%s_array_min_length", fieldName),
			fmt.Sprintf("Validate %s array has minimum length", fieldName),
			fmt.Sprintf("Array has %d items, expected at least %d", length, *expected.Min),
			false,
			map[string]interface{}{
				"actual_length": length,
				"expected_min":  *expected.Min,
			},
		)
	}

	// Validate maximum length
	if expected.Max != nil && length > *expected.Max {
		return ctx.CreateValidationResult(
			fmt.Sprintf("%s_array_max_length", fieldName),
			fmt.Sprintf("Validate %s array has maximum length", fieldName),
			fmt.Sprintf("Array has %d items, expected at most %d", length, *expected.Max),
			false,
			map[string]interface{}{
				"actual_length": length,
				"expected_max":  *expected.Max,
			},
		)
	}

	return ctx.CreateValidationResult(
		fmt.Sprintf("%s_array_length", fieldName),
		fmt.Sprintf("Validate %s array length", fieldName),
		fmt.Sprintf("%s array length is valid (%d items)", fieldName, length),
		true,
		map[string]interface{}{
			"length": length,
		},
	)
}

// JSON Parsing Helpers

// ParseJSONResponse safely parses a JSON response with detailed error reporting
func (ctx *AssertionContext) ParseJSONResponse(response json.RawMessage, target interface{}, fieldName string) *cases.ValidationResult {
	if err := json.Unmarshal(response, target); err != nil {
		return ctx.CreateValidationResult(
			fmt.Sprintf("%s_json_parse", fieldName),
			fmt.Sprintf("Parse %s JSON response", fieldName),
			fmt.Sprintf("Failed to parse JSON: %v", err),
			false,
			map[string]interface{}{
				"error":    err.Error(),
				"response": string(response),
			},
		)
	}

	return ctx.CreateValidationResult(
		fmt.Sprintf("%s_json_parse", fieldName),
		fmt.Sprintf("Parse %s JSON response", fieldName),
		fmt.Sprintf("%s JSON parsed successfully", fieldName),
		true,
		nil,
	)
}

// IsNullResponse checks if response is JSON null
func (ctx *AssertionContext) IsNullResponse(response json.RawMessage) bool {
	return strings.TrimSpace(string(response)) == "null"
}

// Aggregate result helpers

// AllPassed checks if all validation results passed
func AllPassed(results []*cases.ValidationResult) bool {
	for _, result := range results {
		if !result.Passed {
			return false
		}
	}
	return true
}

// CountFailures counts the number of failed validation results
func CountFailures(results []*cases.ValidationResult) int {
	count := 0
	for _, result := range results {
		if !result.Passed {
			count++
		}
	}
	return count
}

// GetFailureMessages extracts failure messages from validation results
func GetFailureMessages(results []*cases.ValidationResult) []string {
	var messages []string
	for _, result := range results {
		if !result.Passed {
			messages = append(messages, result.Message)
		}
	}
	return messages
}
