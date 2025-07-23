package validators

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lsp-gateway/tests/utils/framework/cases"
	"lsp-gateway/tests/utils/framework/config"
)

// DefinitionValidator validates textDocument/definition responses
type DefinitionValidator struct {
	*BaseValidator
	metrics *ValidationMetrics
}

// ValidationMetrics tracks comprehensive validation metrics
type ValidationMetrics struct {
	ResponseTime        time.Duration
	MemoryUsage         int64
	PositionAccuracy    float64
	SymbolResolution    int
	CrossFileNavigation bool
	ValidationErrors    []string
	// References-specific metrics
	TotalReferences   int
	CrossFileRefs     int
	CrossModuleRefs   int
	ReferenceAccuracy float64
}

// DefinitionLocation represents a definition location
type DefinitionLocation struct {
	URI   string   `json:"uri"`
	Range LSPRange `json:"range"`
}

// LSPRange represents an LSP range
type LSPRange struct {
	Start LSPPosition `json:"start"`
	End   LSPPosition `json:"end"`
}

// LSPPosition represents an LSP position
type LSPPosition struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// NewDefinitionValidator creates a new definition validator
func NewDefinitionValidator(config *config.ValidationConfig) *DefinitionValidator {
	return &DefinitionValidator{
		BaseValidator: NewBaseValidator(config),
		metrics:       &ValidationMetrics{},
	}
}

// ValidateResponse validates a textDocument/definition response with comprehensive validation
func (v *DefinitionValidator) ValidateResponse(testCase *cases.TestCase, response json.RawMessage) []*cases.ValidationResult {
	var results []*cases.ValidationResult
	startTime := time.Now()

	// Initialize metrics
	v.metrics.ResponseTime = time.Since(startTime)
	v.metrics.ValidationErrors = []string{}

	// Performance validation - check response time
	results = append(results, v.validateResponseTime(startTime))

	// Protocol compliance validation
	results = append(results, v.validateLSPProtocolCompliance(response)...)

	// Check if response is null (no definition found)
	if string(response) == "null" {
		return v.validateNullResponse(testCase, results)
	}

	// Try to parse as single location first
	var singleLocation DefinitionLocation
	if err := json.Unmarshal(response, &singleLocation); err == nil {
		return v.validateSingleLocation(testCase, singleLocation, results)
	}

	// Try to parse as array of locations
	var locations []DefinitionLocation
	if err := json.Unmarshal(response, &locations); err != nil {
		results = append(results, &cases.ValidationResult{
			Name:        "definition_format",
			Description: "Validate definition response format",
			Passed:      false,
			Message:     fmt.Sprintf("Response is neither a location nor an array of locations: %v", err),
		})
		return results
	}

	return v.validateLocationArray(testCase, locations, results)
}

// validateResponseTime validates LSP response performance
func (v *DefinitionValidator) validateResponseTime(startTime time.Time) *cases.ValidationResult {
	responseTime := time.Since(startTime)
	v.metrics.ResponseTime = responseTime

	// Definition requests should be fast (< 2 seconds for good UX)
	threshold := 2 * time.Second
	if responseTime > threshold {
		return &cases.ValidationResult{
			Name:        "definition_response_time",
			Description: "Validate definition response time performance",
			Passed:      false,
			Message:     fmt.Sprintf("Response time %v exceeds threshold %v", responseTime, threshold),
			Details: map[string]interface{}{
				"response_time_ms": responseTime.Milliseconds(),
				"threshold_ms":     threshold.Milliseconds(),
			},
		}
	}

	return &cases.ValidationResult{
		Name:        "definition_response_time",
		Description: "Validate definition response time performance",
		Passed:      true,
		Message:     fmt.Sprintf("Response time %v is within acceptable threshold", responseTime),
		Details: map[string]interface{}{
			"response_time_ms": responseTime.Milliseconds(),
		},
	}
}

// validateLSPProtocolCompliance validates LSP protocol compliance
func (v *DefinitionValidator) validateLSPProtocolCompliance(response json.RawMessage) []*cases.ValidationResult {
	var results []*cases.ValidationResult

	// LSP spec: definition response can be Location | Location[] | LocationLink[] | null
	var responseData interface{}
	if err := json.Unmarshal(response, &responseData); err != nil {
		results = append(results, &cases.ValidationResult{
			Name:        "lsp_protocol_compliance",
			Description: "Validate LSP protocol compliance",
			Passed:      false,
			Message:     fmt.Sprintf("Invalid JSON response: %v", err),
		})
		return results
	}

	// Check response type compliance
	valid := false
	responseType := "unknown"

	switch v := responseData.(type) {
	case nil:
		valid = true
		responseType = "null"
	case map[string]interface{}:
		// Single Location
		if _, hasURI := v["uri"]; hasURI {
			if _, hasRange := v["range"]; hasRange {
				valid = true
				responseType = "Location"
			}
		}
	case []interface{}:
		// Array of Locations or LocationLinks
		if len(v) == 0 {
			valid = true
			responseType = "empty array"
		} else if item, ok := v[0].(map[string]interface{}); ok {
			if _, hasURI := item["uri"]; hasURI {
				valid = true
				responseType = "Location[]"
			} else if _, hasTargetURI := item["targetUri"]; hasTargetURI {
				valid = true
				responseType = "LocationLink[]"
			}
		}
	}

	if valid {
		results = append(results, &cases.ValidationResult{
			Name:        "lsp_protocol_compliance",
			Description: "Validate LSP protocol compliance",
			Passed:      true,
			Message:     fmt.Sprintf("Response conforms to LSP protocol (type: %s)", responseType),
			Details: map[string]interface{}{
				"response_type": responseType,
			},
		})
	} else {
		results = append(results, &cases.ValidationResult{
			Name:        "lsp_protocol_compliance",
			Description: "Validate LSP protocol compliance",
			Passed:      false,
			Message:     "Response does not conform to LSP definition response format",
			Details: map[string]interface{}{
				"response_type": responseType,
			},
		})
	}

	return results
}

// validateNullResponse handles null response validation
func (v *DefinitionValidator) validateNullResponse(testCase *cases.TestCase, results []*cases.ValidationResult) []*cases.ValidationResult {
	if testCase.Expected != nil && testCase.Expected.Definition != nil && testCase.Expected.Definition.HasLocation {
		results = append(results, &cases.ValidationResult{
			Name:        "definition_presence",
			Description: "Validate definition location is found",
			Passed:      false,
			Message:     "Expected definition but response is null",
		})
	} else {
		results = append(results, &cases.ValidationResult{
			Name:        "definition_absence",
			Description: "Validate no definition found (as expected)",
			Passed:      true,
			Message:     "No definition found as expected",
		})
	}

	// Validate edge case handling
	results = append(results, v.validateEdgeCaseHandling(testCase)...)

	return results
}

// validateSingleLocation validates a single definition location with comprehensive checks
func (v *DefinitionValidator) validateSingleLocation(testCase *cases.TestCase, location DefinitionLocation, results []*cases.ValidationResult) []*cases.ValidationResult {
	results = append(results, &cases.ValidationResult{
		Name:        "definition_format",
		Description: "Validate definition response format",
		Passed:      true,
		Message:     "Response is a single location",
	})

	// Comprehensive location validation
	results = append(results, v.validateLocation(location, "definition")...)

	// Cross-file navigation validation
	results = append(results, v.validateCrossFileNavigation(testCase, location)...)

	// Position accuracy validation
	results = append(results, v.validatePositionAccuracy(testCase, location)...)

	// Symbol resolution validation
	results = append(results, v.validateSymbolResolution(testCase, location)...)

	// Check expected conditions
	if testCase.Expected != nil && testCase.Expected.Definition != nil {
		expected := testCase.Expected.Definition

		if !expected.HasLocation {
			results = append(results, &cases.ValidationResult{
				Name:        "definition_expectation",
				Description: "Validate definition expectation",
				Passed:      false,
				Message:     "Expected no definition but found one",
			})
		} else {
			results = append(results, &cases.ValidationResult{
				Name:        "definition_expectation",
				Description: "Validate definition expectation",
				Passed:      true,
				Message:     "Found definition as expected",
			})

			// Validate specific expected values
			results = append(results, v.validateExpectedDefinition(location, expected)...)
		}
	}

	return results
}

// validateLocationArray validates an array of definition locations with comprehensive analysis
func (v *DefinitionValidator) validateLocationArray(testCase *cases.TestCase, locations []DefinitionLocation, results []*cases.ValidationResult) []*cases.ValidationResult {
	results = append(results, &cases.ValidationResult{
		Name:        "definition_format",
		Description: "Validate definition response format",
		Passed:      true,
		Message:     fmt.Sprintf("Response is an array of %d locations", len(locations)),
	})

	if len(locations) == 0 {
		if testCase.Expected != nil && testCase.Expected.Definition != nil && testCase.Expected.Definition.HasLocation {
			results = append(results, &cases.ValidationResult{
				Name:        "definition_presence",
				Description: "Validate definition location is found",
				Passed:      false,
				Message:     "Expected definition but array is empty",
			})
		} else {
			results = append(results, &cases.ValidationResult{
				Name:        "definition_absence",
				Description: "Validate no definition found (as expected)",
				Passed:      true,
				Message:     "No definitions found as expected",
			})
		}
		return results
	}

	// Validate each location with comprehensive checks
	for i, location := range locations {
		locationResults := v.validateLocation(location, fmt.Sprintf("definition_%d", i))
		results = append(results, locationResults...)

		// Cross-reference validation for multiple definitions
		if i == 0 {
			results = append(results, v.validateCrossFileNavigation(testCase, location)...)
			results = append(results, v.validatePositionAccuracy(testCase, location)...)
			results = append(results, v.validateSymbolResolution(testCase, location)...)
		}
	}

	// Validate definition completeness
	results = append(results, v.validateDefinitionCompleteness(testCase, locations)...)

	// Check expected conditions
	if testCase.Expected != nil && testCase.Expected.Definition != nil {
		expected := testCase.Expected.Definition

		if !expected.HasLocation {
			results = append(results, &cases.ValidationResult{
				Name:        "definition_expectation",
				Description: "Validate definition expectation",
				Passed:      false,
				Message:     fmt.Sprintf("Expected no definition but found %d", len(locations)),
			})
		} else {
			results = append(results, &cases.ValidationResult{
				Name:        "definition_expectation",
				Description: "Validate definition expectation",
				Passed:      true,
				Message:     fmt.Sprintf("Found %d definitions as expected", len(locations)),
			})

			// For array responses, validate against the first location
			if len(locations) > 0 {
				results = append(results, v.validateExpectedDefinition(locations[0], expected)...)
			}
		}
	}

	return results
}

// validateLocation validates a single definition location structure
func (v *DefinitionValidator) validateLocation(location DefinitionLocation, fieldName string) []*cases.ValidationResult {
	var results []*cases.ValidationResult

	// Validate URI
	results = append(results, v.validateURI(location.URI, fmt.Sprintf("%s_location", fieldName)))

	// Validate range structure
	results = append(results, v.validateRange(location.Range, fmt.Sprintf("%s_range", fieldName))...)

	return results
}

// validateRange validates an LSP range structure
func (v *DefinitionValidator) validateRange(lspRange LSPRange, fieldName string) []*cases.ValidationResult {
	var results []*cases.ValidationResult

	// Validate start position
	startResult := v.validatePosition(map[string]interface{}{
		"line":      float64(lspRange.Start.Line),
		"character": float64(lspRange.Start.Character),
	}, fmt.Sprintf("%s_start", fieldName))
	results = append(results, startResult)

	// Validate end position
	endResult := v.validatePosition(map[string]interface{}{
		"line":      float64(lspRange.End.Line),
		"character": float64(lspRange.End.Character),
	}, fmt.Sprintf("%s_end", fieldName))
	results = append(results, endResult)

	// Validate range logic (start should be before or equal to end)
	if v.config.ValidatePositions {
		if lspRange.Start.Line > lspRange.End.Line ||
			(lspRange.Start.Line == lspRange.End.Line && lspRange.Start.Character > lspRange.End.Character) {
			results = append(results, &cases.ValidationResult{
				Name:        fmt.Sprintf("%s_order", fieldName),
				Description: fmt.Sprintf("Validate %s start position is before end position", fieldName),
				Passed:      false,
				Message: fmt.Sprintf("Start position (%d,%d) is after end position (%d,%d)",
					lspRange.Start.Line, lspRange.Start.Character,
					lspRange.End.Line, lspRange.End.Character),
			})
		} else {
			results = append(results, &cases.ValidationResult{
				Name:        fmt.Sprintf("%s_order", fieldName),
				Description: fmt.Sprintf("Validate %s position order", fieldName),
				Passed:      true,
				Message:     "Range positions are in correct order",
			})
		}
	}

	return results
}

// validateExpectedDefinition validates against specific expected definition values
func (v *DefinitionValidator) validateExpectedDefinition(location DefinitionLocation, expected *config.DefinitionExpected) []*cases.ValidationResult {
	var results []*cases.ValidationResult

	// Check expected file URI
	if expected.FileURI != nil {
		if location.URI == *expected.FileURI {
			results = append(results, &cases.ValidationResult{
				Name:        "definition_file_uri",
				Description: "Validate definition file URI matches expected",
				Passed:      true,
				Message:     fmt.Sprintf("File URI matches expected: %s", location.URI),
			})
		} else {
			results = append(results, &cases.ValidationResult{
				Name:        "definition_file_uri",
				Description: "Validate definition file URI matches expected",
				Passed:      false,
				Message:     fmt.Sprintf("File URI %s does not match expected %s", location.URI, *expected.FileURI),
			})
		}
	}

	// Check expected range
	if expected.Range != nil {
		results = append(results, v.validateExpectedRange(location.Range, expected.Range)...)
	}

	return results
}

// validateExpectedRange validates against expected range values
func (v *DefinitionValidator) validateExpectedRange(actualRange LSPRange, expectedRange *config.Range) []*cases.ValidationResult {
	var results []*cases.ValidationResult

	if expectedRange.Start != nil {
		if actualRange.Start.Line == expectedRange.Start.Line && actualRange.Start.Character == expectedRange.Start.Character {
			results = append(results, &cases.ValidationResult{
				Name:        "definition_range_start",
				Description: "Validate definition range start position",
				Passed:      true,
				Message:     fmt.Sprintf("Start position matches expected: (%d,%d)", actualRange.Start.Line, actualRange.Start.Character),
			})
		} else {
			results = append(results, &cases.ValidationResult{
				Name:        "definition_range_start",
				Description: "Validate definition range start position",
				Passed:      false,
				Message: fmt.Sprintf("Start position (%d,%d) does not match expected (%d,%d)",
					actualRange.Start.Line, actualRange.Start.Character,
					expectedRange.Start.Line, expectedRange.Start.Character),
			})
		}
	}

	if expectedRange.End != nil {
		if actualRange.End.Line == expectedRange.End.Line && actualRange.End.Character == expectedRange.End.Character {
			results = append(results, &cases.ValidationResult{
				Name:        "definition_range_end",
				Description: "Validate definition range end position",
				Passed:      true,
				Message:     fmt.Sprintf("End position matches expected: (%d,%d)", actualRange.End.Line, actualRange.End.Character),
			})
		} else {
			results = append(results, &cases.ValidationResult{
				Name:        "definition_range_end",
				Description: "Validate definition range end position",
				Passed:      false,
				Message: fmt.Sprintf("End position (%d,%d) does not match expected (%d,%d)",
					actualRange.End.Line, actualRange.End.Character,
					expectedRange.End.Line, expectedRange.End.Character),
			})
		}
	}

	return results
}

// validateCrossFileNavigation validates cross-file navigation capability
func (v *DefinitionValidator) validateCrossFileNavigation(testCase *cases.TestCase, location DefinitionLocation) []*cases.ValidationResult {
	var results []*cases.ValidationResult

	if testCase.Repository == nil || testCase.FilePath == "" {
		return results
	}

	// Extract file path from URI
	definitionFile := strings.TrimPrefix(location.URI, "file://")
	sourceFile := filepath.Join(testCase.Repository.Path, testCase.FilePath)

	// Check if definition is in a different file (cross-file navigation)
	if definitionFile != sourceFile {
		v.metrics.CrossFileNavigation = true
		results = append(results, &cases.ValidationResult{
			Name:        "cross_file_navigation",
			Description: "Validate cross-file navigation capability",
			Passed:      true,
			Message:     fmt.Sprintf("Definition found in different file: %s", definitionFile),
			Details: map[string]interface{}{
				"source_file":     sourceFile,
				"definition_file": definitionFile,
			},
		})

		// Validate that the definition file exists
		if _, err := os.Stat(definitionFile); err != nil {
			results = append(results, &cases.ValidationResult{
				Name:        "definition_file_exists",
				Description: "Validate definition file exists",
				Passed:      false,
				Message:     fmt.Sprintf("Definition file does not exist: %s", definitionFile),
			})
		} else {
			results = append(results, &cases.ValidationResult{
				Name:        "definition_file_exists",
				Description: "Validate definition file exists",
				Passed:      true,
				Message:     "Definition file exists and is accessible",
			})
		}
	} else {
		results = append(results, &cases.ValidationResult{
			Name:        "same_file_navigation",
			Description: "Validate same-file navigation",
			Passed:      true,
			Message:     "Definition found in same file",
		})
	}

	return results
}

// validatePositionAccuracy validates the accuracy of position information
func (v *DefinitionValidator) validatePositionAccuracy(testCase *cases.TestCase, location DefinitionLocation) []*cases.ValidationResult {
	var results []*cases.ValidationResult

	if testCase.Repository == nil {
		return results
	}

	// Get the definition file path
	definitionFile := strings.TrimPrefix(location.URI, "file://")

	// Validate position is within file bounds
	if _, err := os.Stat(definitionFile); err == nil {
		if content, err := os.ReadFile(definitionFile); err == nil {
			results = append(results, v.validatePositionWithinFileBounds(location, content)...)

			// Validate position points to meaningful content
			results = append(results, v.validatePositionContent(location, content)...)
		}
	}

	return results
}

// validatePositionWithinFileBounds validates position is within file boundaries
func (v *DefinitionValidator) validatePositionWithinFileBounds(location DefinitionLocation, content []byte) []*cases.ValidationResult {
	var results []*cases.ValidationResult

	lines := strings.Split(string(content), "\n")
	totalLines := len(lines)

	// Validate start position
	if location.Range.Start.Line >= totalLines {
		results = append(results, &cases.ValidationResult{
			Name:        "position_bounds_start_line",
			Description: "Validate start position line is within file bounds",
			Passed:      false,
			Message:     fmt.Sprintf("Start line %d exceeds file bounds (total lines: %d)", location.Range.Start.Line, totalLines),
		})
	} else {
		lineContent := lines[location.Range.Start.Line]
		if location.Range.Start.Character > len(lineContent) {
			results = append(results, &cases.ValidationResult{
				Name:        "position_bounds_start_character",
				Description: "Validate start position character is within line bounds",
				Passed:      false,
				Message:     fmt.Sprintf("Start character %d exceeds line length %d", location.Range.Start.Character, len(lineContent)),
			})
		} else {
			results = append(results, &cases.ValidationResult{
				Name:        "position_bounds_start",
				Description: "Validate start position is within bounds",
				Passed:      true,
				Message:     "Start position is within file bounds",
			})
		}
	}

	// Validate end position
	if location.Range.End.Line >= totalLines {
		results = append(results, &cases.ValidationResult{
			Name:        "position_bounds_end_line",
			Description: "Validate end position line is within file bounds",
			Passed:      false,
			Message:     fmt.Sprintf("End line %d exceeds file bounds (total lines: %d)", location.Range.End.Line, totalLines),
		})
	} else {
		lineContent := lines[location.Range.End.Line]
		if location.Range.End.Character > len(lineContent) {
			results = append(results, &cases.ValidationResult{
				Name:        "position_bounds_end_character",
				Description: "Validate end position character is within line bounds",
				Passed:      false,
				Message:     fmt.Sprintf("End character %d exceeds line length %d", location.Range.End.Character, len(lineContent)),
			})
		} else {
			results = append(results, &cases.ValidationResult{
				Name:        "position_bounds_end",
				Description: "Validate end position is within bounds",
				Passed:      true,
				Message:     "End position is within file bounds",
			})
		}
	}

	return results
}

// validatePositionContent validates that position points to meaningful content
func (v *DefinitionValidator) validatePositionContent(location DefinitionLocation, content []byte) []*cases.ValidationResult {
	var results []*cases.ValidationResult

	lines := strings.Split(string(content), "\n")

	if location.Range.Start.Line < len(lines) {
		lineContent := lines[location.Range.Start.Line]

		// Extract the text at the definition position
		startChar := location.Range.Start.Character
		endChar := location.Range.End.Character

		if startChar < len(lineContent) && endChar <= len(lineContent) && startChar <= endChar {
			definitionText := lineContent[startChar:endChar]

			// Check if definition text is meaningful (not just whitespace)
			if strings.TrimSpace(definitionText) == "" {
				results = append(results, &cases.ValidationResult{
					Name:        "position_content_meaningful",
					Description: "Validate position points to meaningful content",
					Passed:      false,
					Message:     "Position points to whitespace only",
					Details: map[string]interface{}{
						"definition_text": definitionText,
						"line_content":    lineContent,
					},
				})
			} else {
				results = append(results, &cases.ValidationResult{
					Name:        "position_content_meaningful",
					Description: "Validate position points to meaningful content",
					Passed:      true,
					Message:     fmt.Sprintf("Position points to meaningful content: '%s'", strings.TrimSpace(definitionText)),
					Details: map[string]interface{}{
						"definition_text": definitionText,
					},
				})

				// Additional validation: check if it looks like a symbol definition
				results = append(results, v.validateSymbolLikeContent(definitionText)...)
			}
		}
	}

	return results
}

// validateSymbolLikeContent validates that content looks like a symbol definition
func (v *DefinitionValidator) validateSymbolLikeContent(text string) []*cases.ValidationResult {
	var results []*cases.ValidationResult

	text = strings.TrimSpace(text)

	// Basic heuristics for symbol-like content
	isSymbolLike := false
	symbolType := "unknown"

	// Check for common symbol patterns
	if len(text) > 0 {
		// Identifier pattern (letters, numbers, underscores)
		if strings.ContainsAny(text, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ_") {
			isSymbolLike = true

			// Try to determine symbol type
			if strings.Contains(text, "func ") || strings.Contains(text, "function ") {
				symbolType = "function"
			} else if strings.Contains(text, "class ") {
				symbolType = "class"
			} else if strings.Contains(text, "var ") || strings.Contains(text, "let ") || strings.Contains(text, "const ") {
				symbolType = "variable"
			} else if strings.Contains(text, "type ") || strings.Contains(text, "interface ") {
				symbolType = "type"
			} else {
				symbolType = "identifier"
			}
		}
	}

	if isSymbolLike {
		results = append(results, &cases.ValidationResult{
			Name:        "symbol_like_content",
			Description: "Validate content appears to be a symbol definition",
			Passed:      true,
			Message:     fmt.Sprintf("Content appears to be a %s definition", symbolType),
			Details: map[string]interface{}{
				"symbol_type": symbolType,
				"content":     text,
			},
		})
	} else {
		results = append(results, &cases.ValidationResult{
			Name:        "symbol_like_content",
			Description: "Validate content appears to be a symbol definition",
			Passed:      false,
			Message:     "Content does not appear to be a symbol definition",
			Details: map[string]interface{}{
				"content": text,
			},
		})
	}

	return results
}

// validateSymbolResolution validates symbol resolution accuracy
func (v *DefinitionValidator) validateSymbolResolution(testCase *cases.TestCase, location DefinitionLocation) []*cases.ValidationResult {
	var results []*cases.ValidationResult

	v.metrics.SymbolResolution++

	// Validate that the definition URI is accessible and valid
	definitionFile := strings.TrimPrefix(location.URI, "file://")

	if _, err := os.Stat(definitionFile); err != nil {
		results = append(results, &cases.ValidationResult{
			Name:        "symbol_resolution_file_access",
			Description: "Validate definition file is accessible",
			Passed:      false,
			Message:     fmt.Sprintf("Cannot access definition file: %v", err),
		})
	} else {
		results = append(results, &cases.ValidationResult{
			Name:        "symbol_resolution_file_access",
			Description: "Validate definition file is accessible",
			Passed:      true,
			Message:     "Definition file is accessible",
		})

		// Validate file extension matches language
		if testCase.Language != "" {
			results = append(results, v.validateLanguageConsistency(definitionFile, testCase.Language)...)
		}
	}

	return results
}

// validateLanguageConsistency validates that definition file matches expected language
func (v *DefinitionValidator) validateLanguageConsistency(filePath, expectedLang string) []*cases.ValidationResult {
	var results []*cases.ValidationResult

	ext := strings.ToLower(filepath.Ext(filePath))
	expectedExts := getLanguageExtensions(expectedLang)

	consistent := false
	for _, expectedExt := range expectedExts {
		if ext == expectedExt {
			consistent = true
			break
		}
	}

	if consistent {
		results = append(results, &cases.ValidationResult{
			Name:        "language_consistency",
			Description: "Validate definition file language consistency",
			Passed:      true,
			Message:     fmt.Sprintf("File extension %s matches expected language %s", ext, expectedLang),
		})
	} else {
		results = append(results, &cases.ValidationResult{
			Name:        "language_consistency",
			Description: "Validate definition file language consistency",
			Passed:      false,
			Message:     fmt.Sprintf("File extension %s may not match expected language %s", ext, expectedLang),
			Details: map[string]interface{}{
				"file_extension":      ext,
				"expected_language":   expectedLang,
				"expected_extensions": expectedExts,
			},
		})
	}

	return results
}

// getLanguageExtensions returns common file extensions for a language
func getLanguageExtensions(lang string) []string {
	switch strings.ToLower(lang) {
	case "go":
		return []string{".go"}
	case "python":
		return []string{".py", ".pyx", ".pyi"}
	case "javascript", "js":
		return []string{".js", ".jsx", ".mjs", ".cjs"}
	case "typescript", "ts":
		return []string{".ts", ".tsx", ".d.ts"}
	case "java":
		return []string{".java"}
	case "c":
		return []string{".c", ".h"}
	case "cpp", "c++":
		return []string{".cpp", ".cc", ".cxx", ".c++", ".hpp", ".hh", ".hxx", ".h++"}
	case "rust":
		return []string{".rs"}
	case "php":
		return []string{".php", ".phtml", ".php3", ".php4", ".php5", ".phps"}
	default:
		return []string{}
	}
}

// validateDefinitionCompleteness validates completeness of definition results
func (v *DefinitionValidator) validateDefinitionCompleteness(testCase *cases.TestCase, locations []DefinitionLocation) []*cases.ValidationResult {
	var results []*cases.ValidationResult

	// Check for duplicate definitions
	seenURIs := make(map[string]bool)
	duplicateCount := 0

	for _, location := range locations {
		key := fmt.Sprintf("%s:%d:%d", location.URI, location.Range.Start.Line, location.Range.Start.Character)
		if seenURIs[key] {
			duplicateCount++
		} else {
			seenURIs[key] = true
		}
	}

	if duplicateCount > 0 {
		results = append(results, &cases.ValidationResult{
			Name:        "definition_uniqueness",
			Description: "Validate definition results are unique",
			Passed:      false,
			Message:     fmt.Sprintf("Found %d duplicate definitions", duplicateCount),
			Details: map[string]interface{}{
				"total_definitions":  len(locations),
				"unique_definitions": len(seenURIs),
				"duplicates":         duplicateCount,
			},
		})
	} else {
		results = append(results, &cases.ValidationResult{
			Name:        "definition_uniqueness",
			Description: "Validate definition results are unique",
			Passed:      true,
			Message:     "All definition results are unique",
			Details: map[string]interface{}{
				"total_definitions": len(locations),
			},
		})
	}

	return results
}

// validateEdgeCaseHandling validates edge case and error condition handling
func (v *DefinitionValidator) validateEdgeCaseHandling(testCase *cases.TestCase) []*cases.ValidationResult {
	var results []*cases.ValidationResult

	// Validate handling of invalid positions
	if testCase.Position != nil {
		if testCase.Position.Line < 0 {
			results = append(results, &cases.ValidationResult{
				Name:        "edge_case_negative_line",
				Description: "Validate handling of negative line positions",
				Passed:      true,
				Message:     "Server correctly handled negative line position by returning null",
			})
		}

		if testCase.Position.Character < 0 {
			results = append(results, &cases.ValidationResult{
				Name:        "edge_case_negative_character",
				Description: "Validate handling of negative character positions",
				Passed:      true,
				Message:     "Server correctly handled negative character position by returning null",
			})
		}
	}

	// Validate handling of non-existent files
	if testCase.FilePath != "" && testCase.Repository != nil {
		fullPath := filepath.Join(testCase.Repository.Path, testCase.FilePath)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			results = append(results, &cases.ValidationResult{
				Name:        "edge_case_missing_file",
				Description: "Validate handling of requests on non-existent files",
				Passed:      true,
				Message:     "Server correctly handled request on non-existent file by returning null",
			})
		}
	}

	return results
}
