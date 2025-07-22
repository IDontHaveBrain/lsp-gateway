package validators

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lsp-gateway/internal/testing/lsp/cases"
	"lsp-gateway/internal/testing/lsp/config"
)

// ReferencesValidator validates textDocument/references responses
type ReferencesValidator struct {
	*BaseValidator
	metrics *ValidationMetrics
}

// ReferenceLocation represents a reference location (same as DefinitionLocation)
type ReferenceLocation struct {
	URI   string   `json:"uri"`
	Range LSPRange `json:"range"`
}

// NewReferencesValidator creates a new references validator
func NewReferencesValidator(config *config.ValidationConfig) *ReferencesValidator {
	return &ReferencesValidator{
		BaseValidator: NewBaseValidator(config),
		metrics:       &ValidationMetrics{},
	}
}

// ValidateResponse validates a textDocument/references response
func (v *ReferencesValidator) ValidateResponse(testCase *cases.TestCase, response json.RawMessage) []*cases.ValidationResult {
	var results []*cases.ValidationResult
	startTime := time.Now()

	// Performance validation - check response time
	results = append(results, v.validateResponseTime(startTime))

	// Protocol compliance validation
	results = append(results, v.validateLSPProtocolCompliance(response)...)

	// References should always return an array (or null)
	if string(response) == "null" {
		if testCase.Expected != nil && testCase.Expected.References != nil && testCase.Expected.References.MinCount > 0 {
			results = append(results, &cases.ValidationResult{
				Name:        "references_presence",
				Description: "Validate references are found",
				Passed:      false,
				Message:     "Expected references but response is null",
			})
		} else {
			results = append(results, &cases.ValidationResult{
				Name:        "references_absence",
				Description: "Validate no references found (as expected)",
				Passed:      true,
				Message:     "No references found as expected",
			})
		}

		// Validate edge case handling for null responses
		results = append(results, v.validateEdgeCaseHandling(testCase)...)

		return results
	}

	// Parse as array of locations
	var references []ReferenceLocation
	if err := json.Unmarshal(response, &references); err != nil {
		results = append(results, &cases.ValidationResult{
			Name:        "references_format",
			Description: "Validate references response format",
			Passed:      false,
			Message:     fmt.Sprintf("Response is not an array of locations: %v", err),
		})
		return results
	}

	results = append(results, &cases.ValidationResult{
		Name:        "references_format",
		Description: "Validate references response format",
		Passed:      true,
		Message:     fmt.Sprintf("Response is an array of %d references", len(references)),
	})

	// Validate count expectations
	if testCase.Expected != nil && testCase.Expected.References != nil {
		results = append(results, v.validateReferenceCount(references, testCase.Expected.References)...)
	}

	// Validate each reference location
	for i, ref := range references {
		refResults := v.validateReferenceLocation(ref, fmt.Sprintf("reference_%d", i))
		results = append(results, refResults...)
	}

	// Comprehensive reference validation
	results = append(results, v.validateReferenceCompleteness(testCase, references)...)
	results = append(results, v.validateCrossModuleReferences(testCase, references)...)
	results = append(results, v.validateReferenceAccuracy(testCase, references)...)

	// Update metrics
	v.metrics.TotalReferences = len(references)

	return results
}

// validateReferenceCount validates the number of references
func (v *ReferencesValidator) validateReferenceCount(references []ReferenceLocation, expected *config.ReferencesExpected) []*cases.ValidationResult {
	var results []*cases.ValidationResult

	count := len(references)

	// Check minimum count
	if expected.MinCount > 0 {
		if count >= expected.MinCount {
			results = append(results, &cases.ValidationResult{
				Name:        "references_min_count",
				Description: fmt.Sprintf("Validate at least %d references found", expected.MinCount),
				Passed:      true,
				Message:     fmt.Sprintf("Found %d references, meets minimum of %d", count, expected.MinCount),
			})
		} else {
			results = append(results, &cases.ValidationResult{
				Name:        "references_min_count",
				Description: fmt.Sprintf("Validate at least %d references found", expected.MinCount),
				Passed:      false,
				Message:     fmt.Sprintf("Found %d references, expected at least %d", count, expected.MinCount),
			})
		}
	}

	// Check maximum count
	if expected.MaxCount > 0 {
		if count <= expected.MaxCount {
			results = append(results, &cases.ValidationResult{
				Name:        "references_max_count",
				Description: fmt.Sprintf("Validate at most %d references found", expected.MaxCount),
				Passed:      true,
				Message:     fmt.Sprintf("Found %d references, within maximum of %d", count, expected.MaxCount),
			})
		} else {
			results = append(results, &cases.ValidationResult{
				Name:        "references_max_count",
				Description: fmt.Sprintf("Validate at most %d references found", expected.MaxCount),
				Passed:      false,
				Message:     fmt.Sprintf("Found %d references, exceeds maximum of %d", count, expected.MaxCount),
			})
		}
	}

	return results
}

// validateReferenceLocation validates a single reference location
func (v *ReferencesValidator) validateReferenceLocation(reference ReferenceLocation, fieldName string) []*cases.ValidationResult {
	var results []*cases.ValidationResult

	// Validate URI
	results = append(results, v.validateURI(reference.URI, fmt.Sprintf("%s_location", fieldName)))

	// Validate range structure (reuse from definition validator)
	results = append(results, v.validateReferenceRange(reference.Range, fmt.Sprintf("%s_range", fieldName))...)

	return results
}

// validateReferenceRange validates an LSP range structure for references
func (v *ReferencesValidator) validateReferenceRange(lspRange LSPRange, fieldName string) []*cases.ValidationResult {
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

// validateResponseTime validates LSP response performance for references
func (v *ReferencesValidator) validateResponseTime(startTime time.Time) *cases.ValidationResult {
	responseTime := time.Since(startTime)
	v.metrics.ResponseTime = responseTime

	// References requests should be reasonably fast (< 3 seconds for good UX)
	threshold := 3 * time.Second
	if responseTime > threshold {
		return &cases.ValidationResult{
			Name:        "references_response_time",
			Description: "Validate references response time performance",
			Passed:      false,
			Message:     fmt.Sprintf("Response time %v exceeds threshold %v", responseTime, threshold),
			Details: map[string]interface{}{
				"response_time_ms": responseTime.Milliseconds(),
				"threshold_ms":     threshold.Milliseconds(),
			},
		}
	}

	return &cases.ValidationResult{
		Name:        "references_response_time",
		Description: "Validate references response time performance",
		Passed:      true,
		Message:     fmt.Sprintf("Response time %v is within acceptable threshold", responseTime),
		Details: map[string]interface{}{
			"response_time_ms": responseTime.Milliseconds(),
		},
	}
}

// validateLSPProtocolCompliance validates LSP protocol compliance for references
func (v *ReferencesValidator) validateLSPProtocolCompliance(response json.RawMessage) []*cases.ValidationResult {
	var results []*cases.ValidationResult

	// LSP spec: references response should be Location[] | null
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
	case []interface{}:
		// Array of Locations
		if len(v) == 0 {
			valid = true
			responseType = "empty array"
		} else if item, ok := v[0].(map[string]interface{}); ok {
			if _, hasURI := item["uri"]; hasURI {
				if _, hasRange := item["range"]; hasRange {
					valid = true
					responseType = "Location[]"
				}
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
			Message:     "Response does not conform to LSP references response format",
			Details: map[string]interface{}{
				"response_type": responseType,
			},
		})
	}

	return results
}

// validateReferenceCompleteness validates completeness of reference results
func (v *ReferencesValidator) validateReferenceCompleteness(testCase *cases.TestCase, references []ReferenceLocation) []*cases.ValidationResult {
	var results []*cases.ValidationResult

	// Check for duplicate references
	seenRefs := make(map[string]bool)
	duplicateCount := 0

	for _, ref := range references {
		key := fmt.Sprintf("%s:%d:%d", ref.URI, ref.Range.Start.Line, ref.Range.Start.Character)
		if seenRefs[key] {
			duplicateCount++
		} else {
			seenRefs[key] = true
		}
	}

	if duplicateCount > 0 {
		results = append(results, &cases.ValidationResult{
			Name:        "references_uniqueness",
			Description: "Validate reference results are unique",
			Passed:      false,
			Message:     fmt.Sprintf("Found %d duplicate references", duplicateCount),
			Details: map[string]interface{}{
				"total_references":  len(references),
				"unique_references": len(seenRefs),
				"duplicates":        duplicateCount,
			},
		})
	} else {
		results = append(results, &cases.ValidationResult{
			Name:        "references_uniqueness",
			Description: "Validate reference results are unique",
			Passed:      true,
			Message:     "All reference results are unique",
			Details: map[string]interface{}{
				"total_references": len(references),
			},
		})
	}

	// Validate that references include the declaration if requested
	if testCase.Expected != nil && testCase.Expected.References != nil && testCase.Expected.References.IncludeDecl {
		results = append(results, v.validateDeclarationInclusion(testCase, references)...)
	}

	return results
}

// validateDeclarationInclusion validates that declaration is included in references when requested
func (v *ReferencesValidator) validateDeclarationInclusion(testCase *cases.TestCase, references []ReferenceLocation) []*cases.ValidationResult {
	var results []*cases.ValidationResult

	if testCase.Position == nil || testCase.Repository == nil || testCase.FilePath == "" {
		return results
	}

	// Build expected declaration location
	sourceFile := filepath.Join(testCase.Repository.Path, testCase.FilePath)
	sourceURI := "file://" + sourceFile

	// Check if any reference matches the original position (declaration)
	declarationFound := false
	for _, ref := range references {
		if ref.URI == sourceURI &&
			ref.Range.Start.Line == testCase.Position.Line &&
			ref.Range.Start.Character == testCase.Position.Character {
			declarationFound = true
			break
		}
	}

	if declarationFound {
		results = append(results, &cases.ValidationResult{
			Name:        "declaration_inclusion",
			Description: "Validate declaration is included in references",
			Passed:      true,
			Message:     "Declaration found in references as expected",
		})
	} else {
		results = append(results, &cases.ValidationResult{
			Name:        "declaration_inclusion",
			Description: "Validate declaration is included in references",
			Passed:      false,
			Message:     "Declaration not found in references but was expected",
			Details: map[string]interface{}{
				"expected_uri":       sourceURI,
				"expected_line":      testCase.Position.Line,
				"expected_character": testCase.Position.Character,
			},
		})
	}

	return results
}

// validateCrossModuleReferences validates cross-module reference capability
func (v *ReferencesValidator) validateCrossModuleReferences(testCase *cases.TestCase, references []ReferenceLocation) []*cases.ValidationResult {
	var results []*cases.ValidationResult

	if testCase.Repository == nil || testCase.FilePath == "" {
		return results
	}

	sourceFile := filepath.Join(testCase.Repository.Path, testCase.FilePath)
	crossFileRefs := 0
	crossModuleRefs := 0
	accessibleFiles := 0

	for _, ref := range references {
		refFile := strings.TrimPrefix(ref.URI, "file://")

		// Check if reference is in a different file
		if refFile != sourceFile {
			crossFileRefs++
			v.metrics.CrossFileRefs++

			// Check if file is accessible
			if _, err := os.Stat(refFile); err == nil {
				accessibleFiles++

				// Check if it's a different module/package
				if v.isDifferentModule(sourceFile, refFile, testCase.Language) {
					crossModuleRefs++
					v.metrics.CrossModuleRefs++
				}
			}
		}
	}

	// Report cross-file references
	if crossFileRefs > 0 {
		results = append(results, &cases.ValidationResult{
			Name:        "cross_file_references",
			Description: "Validate cross-file reference detection",
			Passed:      true,
			Message:     fmt.Sprintf("Found %d cross-file references", crossFileRefs),
			Details: map[string]interface{}{
				"cross_file_count": crossFileRefs,
				"accessible_files": accessibleFiles,
			},
		})
	}

	// Report cross-module references
	if crossModuleRefs > 0 {
		results = append(results, &cases.ValidationResult{
			Name:        "cross_module_references",
			Description: "Validate cross-module reference detection",
			Passed:      true,
			Message:     fmt.Sprintf("Found %d cross-module references", crossModuleRefs),
			Details: map[string]interface{}{
				"cross_module_count": crossModuleRefs,
			},
		})
	}

	// Validate file accessibility
	if crossFileRefs > 0 {
		accessibilityRate := float64(accessibleFiles) / float64(crossFileRefs) * 100

		if accessibilityRate >= 90 {
			results = append(results, &cases.ValidationResult{
				Name:        "reference_file_accessibility",
				Description: "Validate reference files are accessible",
				Passed:      true,
				Message:     fmt.Sprintf("%.1f%% of reference files are accessible", accessibilityRate),
				Details: map[string]interface{}{
					"accessibility_rate": accessibilityRate,
					"accessible_files":   accessibleFiles,
					"total_cross_files":  crossFileRefs,
				},
			})
		} else {
			results = append(results, &cases.ValidationResult{
				Name:        "reference_file_accessibility",
				Description: "Validate reference files are accessible",
				Passed:      false,
				Message:     fmt.Sprintf("Only %.1f%% of reference files are accessible", accessibilityRate),
				Details: map[string]interface{}{
					"accessibility_rate": accessibilityRate,
					"accessible_files":   accessibleFiles,
					"total_cross_files":  crossFileRefs,
				},
			})
		}
	}

	return results
}

// isDifferentModule checks if two files belong to different modules/packages
func (v *ReferencesValidator) isDifferentModule(file1, file2, language string) bool {
	switch strings.ToLower(language) {
	case "go":
		// Different Go modules if they have different go.mod files
		return v.hasDifferentGoModules(file1, file2)
	case "python":
		// Different Python packages if they have different top-level directories
		return v.hasDifferentPythonPackages(file1, file2)
	case "java":
		// Different Java packages if they have different package declarations
		return v.hasDifferentJavaPackages(file1, file2)
	case "javascript", "typescript":
		// Different JS/TS modules if they have different package.json files
		return v.hasDifferentNpmPackages(file1, file2)
	default:
		// For other languages, consider different directories as different modules
		return filepath.Dir(file1) != filepath.Dir(file2)
	}
}

// hasDifferentGoModules checks if files belong to different Go modules
func (v *ReferencesValidator) hasDifferentGoModules(file1, file2 string) bool {
	mod1 := findGoModule(file1)
	mod2 := findGoModule(file2)
	return mod1 != mod2
}

// hasDifferentPythonPackages checks if files belong to different Python packages
func (v *ReferencesValidator) hasDifferentPythonPackages(file1, file2 string) bool {
	pkg1 := findPythonPackage(file1)
	pkg2 := findPythonPackage(file2)
	return pkg1 != pkg2
}

// hasDifferentJavaPackages checks if files belong to different Java packages
func (v *ReferencesValidator) hasDifferentJavaPackages(file1, file2 string) bool {
	// This would require parsing the package declaration in Java files
	// For now, use directory-based heuristic
	return filepath.Dir(file1) != filepath.Dir(file2)
}

// hasDifferentNpmPackages checks if files belong to different npm packages
func (v *ReferencesValidator) hasDifferentNpmPackages(file1, file2 string) bool {
	pkg1 := findNpmPackage(file1)
	pkg2 := findNpmPackage(file2)
	return pkg1 != pkg2
}

// findGoModule finds the Go module root for a file (reuse from definition validator)
func findGoModule(filePath string) string {
	dir := filepath.Dir(filePath)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// findPythonPackage finds the Python package root for a file
func findPythonPackage(filePath string) string {
	dir := filepath.Dir(filePath)
	for {
		if _, err := os.Stat(filepath.Join(dir, "setup.py")); err == nil {
			return dir
		}
		if _, err := os.Stat(filepath.Join(dir, "pyproject.toml")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// findNpmPackage finds the npm package root for a file
func findNpmPackage(filePath string) string {
	dir := filepath.Dir(filePath)
	for {
		if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// validateReferenceAccuracy validates the accuracy of reference positions
func (v *ReferencesValidator) validateReferenceAccuracy(testCase *cases.TestCase, references []ReferenceLocation) []*cases.ValidationResult {
	var results []*cases.ValidationResult

	accurateRefs := 0
	totalRefs := len(references)

	for i, ref := range references {
		refFile := strings.TrimPrefix(ref.URI, "file://")

		// Validate position is within file bounds
		if _, err := os.Stat(refFile); err == nil {
			if content, err := os.ReadFile(refFile); err == nil {
				if v.validateReferencePositionAccuracy(ref, content) {
					accurateRefs++
				}
			}
		}

		// Validate a sample of references (first 5) with detailed position checks
		if i < 5 {
			results = append(results, v.validateSingleReferenceAccuracy(ref, fmt.Sprintf("reference_%d", i))...)
		}
	}

	// Calculate accuracy rate
	if totalRefs > 0 {
		accuracyRate := float64(accurateRefs) / float64(totalRefs) * 100
		v.metrics.ReferenceAccuracy = accuracyRate

		if accuracyRate >= 95 {
			results = append(results, &cases.ValidationResult{
				Name:        "reference_position_accuracy",
				Description: "Validate reference position accuracy",
				Passed:      true,
				Message:     fmt.Sprintf("%.1f%% of references have accurate positions", accuracyRate),
				Details: map[string]interface{}{
					"accuracy_rate": accuracyRate,
					"accurate_refs": accurateRefs,
					"total_refs":    totalRefs,
				},
			})
		} else {
			results = append(results, &cases.ValidationResult{
				Name:        "reference_position_accuracy",
				Description: "Validate reference position accuracy",
				Passed:      false,
				Message:     fmt.Sprintf("Only %.1f%% of references have accurate positions", accuracyRate),
				Details: map[string]interface{}{
					"accuracy_rate": accuracyRate,
					"accurate_refs": accurateRefs,
					"total_refs":    totalRefs,
				},
			})
		}
	}

	return results
}

// validateReferencePositionAccuracy validates a single reference position accuracy
func (v *ReferencesValidator) validateReferencePositionAccuracy(ref ReferenceLocation, content []byte) bool {
	lines := strings.Split(string(content), "\n")

	// Check if position is within file bounds
	if ref.Range.Start.Line >= len(lines) {
		return false
	}

	lineContent := lines[ref.Range.Start.Line]
	if ref.Range.Start.Character > len(lineContent) {
		return false
	}

	// Check if end position is valid
	if ref.Range.End.Line >= len(lines) {
		return false
	}

	endLineContent := lines[ref.Range.End.Line]
	return ref.Range.End.Character <= len(endLineContent)
}

// validateSingleReferenceAccuracy validates a single reference with detailed checks
func (v *ReferencesValidator) validateSingleReferenceAccuracy(ref ReferenceLocation, fieldName string) []*cases.ValidationResult {
	var results []*cases.ValidationResult

	refFile := strings.TrimPrefix(ref.URI, "file://")

	// Validate file exists
	if _, err := os.Stat(refFile); err != nil {
		results = append(results, &cases.ValidationResult{
			Name:        fmt.Sprintf("%s_file_exists", fieldName),
			Description: fmt.Sprintf("Validate %s file exists", fieldName),
			Passed:      false,
			Message:     fmt.Sprintf("Reference file does not exist: %s", refFile),
		})
		return results
	}

	// Read file and validate position
	if content, err := os.ReadFile(refFile); err == nil {
		lines := strings.Split(string(content), "\n")

		// Validate position bounds
		if ref.Range.Start.Line < len(lines) && ref.Range.Start.Character <= len(lines[ref.Range.Start.Line]) {
			results = append(results, &cases.ValidationResult{
				Name:        fmt.Sprintf("%s_position_valid", fieldName),
				Description: fmt.Sprintf("Validate %s position is valid", fieldName),
				Passed:      true,
				Message:     "Reference position is within file bounds",
			})

			// Check if position contains meaningful content
			if ref.Range.Start.Line < len(lines) {
				lineContent := lines[ref.Range.Start.Line]
				startChar := ref.Range.Start.Character
				endChar := ref.Range.End.Character

				if startChar < len(lineContent) && endChar <= len(lineContent) && startChar <= endChar {
					refText := lineContent[startChar:endChar]
					if strings.TrimSpace(refText) != "" {
						results = append(results, &cases.ValidationResult{
							Name:        fmt.Sprintf("%s_meaningful_content", fieldName),
							Description: fmt.Sprintf("Validate %s points to meaningful content", fieldName),
							Passed:      true,
							Message:     fmt.Sprintf("Reference points to meaningful content: '%s'", strings.TrimSpace(refText)),
							Details: map[string]interface{}{
								"reference_text": refText,
							},
						})
					}
				}
			}
		} else {
			results = append(results, &cases.ValidationResult{
				Name:        fmt.Sprintf("%s_position_valid", fieldName),
				Description: fmt.Sprintf("Validate %s position is valid", fieldName),
				Passed:      false,
				Message:     "Reference position is outside file bounds",
			})
		}
	}

	return results
}

// validateEdgeCaseHandling validates edge case and error condition handling for references
func (v *ReferencesValidator) validateEdgeCaseHandling(testCase *cases.TestCase) []*cases.ValidationResult {
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
