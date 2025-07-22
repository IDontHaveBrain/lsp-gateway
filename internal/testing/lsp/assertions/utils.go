package assertions

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"lsp-gateway/internal/testing/lsp/cases"
	lspconfig "lsp-gateway/internal/testing/lsp/config"
)

// Utility functions and convenience methods for assertion helpers

// Quick assertion builders for common scenarios

// QuickDefinitionTest creates a simple definition test expectation
func QuickDefinitionTest(hasResult bool, uri string, line, char int) *ComprehensiveExpectedResult {
	builder := NewExpectedResult().WithDefinition(hasResult)

	if hasResult && uri != "" {
		builder = builder.WithDefinitionLocation(uri, line, char, line, char+10)
	}

	return builder.Build()
}

// QuickReferencesTest creates a simple references test expectation
func QuickReferencesTest(hasResult bool, minCount, maxCount int) *ComprehensiveExpectedResult {
	builder := NewExpectedResult().WithReferences(hasResult)

	if hasResult {
		builder = builder.WithReferencesCount(minCount, maxCount)
	}

	return builder.Build()
}

// QuickHoverTest creates a simple hover test expectation
func QuickHoverTest(hasResult bool, contentPatterns ...string) *ComprehensiveExpectedResult {
	builder := NewExpectedResult().WithHover(hasResult)

	if hasResult && len(contentPatterns) > 0 {
		builder = builder.WithHoverContent(true, contentPatterns...)
	}

	return builder.Build()
}

// QuickSymbolTest creates a simple document symbol test expectation
func QuickSymbolTest(hasResult bool, minCount, maxCount int) *ComprehensiveExpectedResult {
	builder := NewExpectedResult().WithDocumentSymbol(hasResult)

	if hasResult {
		builder = builder.WithDocumentSymbolCount(minCount, maxCount)
	}

	return builder.Build()
}

// QuickErrorTest creates a simple error test expectation
func QuickErrorTest(errorCode int, messagePatterns ...string) *ComprehensiveExpectedResult {
	builder := NewExpectedResult().
		Success(false).
		WithError(true).
		WithErrorCode(errorCode)

	if len(messagePatterns) > 0 {
		builder = builder.WithErrorMessageContains(messagePatterns...)
	}

	return builder.Build()
}

// Validation result analysis utilities

// FilterResults filters validation results by name patterns
func FilterResults(results []*cases.ValidationResult, patterns ...string) []*cases.ValidationResult {
	var filtered []*cases.ValidationResult

	for _, result := range results {
		for _, pattern := range patterns {
			if matched, _ := regexp.MatchString(pattern, result.Name); matched {
				filtered = append(filtered, result)
				break
			}
		}
	}

	return filtered
}

// GroupResultsByType groups validation results by their type (first part of name)
func GroupResultsByType(results []*cases.ValidationResult) map[string][]*cases.ValidationResult {
	groups := make(map[string][]*cases.ValidationResult)

	for _, result := range results {
		resultType := extractFailureType(result.Name)
		groups[resultType] = append(groups[resultType], result)
	}

	return groups
}

// GetFailedResults returns only the failed validation results
func GetFailedResults(results []*cases.ValidationResult) []*cases.ValidationResult {
	var failed []*cases.ValidationResult

	for _, result := range results {
		if !result.Passed {
			failed = append(failed, result)
		}
	}

	return failed
}

// GetPassedResults returns only the passed validation results
func GetPassedResults(results []*cases.ValidationResult) []*cases.ValidationResult {
	var passed []*cases.ValidationResult

	for _, result := range results {
		if result.Passed {
			passed = append(passed, result)
		}
	}

	return passed
}

// Response parsing utilities

// ExtractLocationFromResponse extracts location information from various LSP response formats
func ExtractLocationFromResponse(response json.RawMessage) ([]*LocationInfo, error) {
	var locations []*LocationInfo

	// Try single location first
	var singleLoc map[string]interface{}
	if err := json.Unmarshal(response, &singleLoc); err == nil {
		if _, hasURI := singleLoc["uri"]; hasURI {
			if loc := parseLocationFromMap(singleLoc); loc != nil {
				locations = append(locations, loc)
				return locations, nil
			}
		}
	}

	// Try array of locations
	var arrayLocs []interface{}
	if err := json.Unmarshal(response, &arrayLocs); err == nil {
		for _, locInterface := range arrayLocs {
			if locMap, ok := locInterface.(map[string]interface{}); ok {
				if loc := parseLocationFromMap(locMap); loc != nil {
					locations = append(locations, loc)
				}
			}
		}
		return locations, nil
	}

	return nil, fmt.Errorf("unable to extract location information from response")
}

// LocationInfo represents parsed location information
type LocationInfo struct {
	URI       string `json:"uri"`
	StartLine int    `json:"start_line"`
	StartChar int    `json:"start_char"`
	EndLine   int    `json:"end_line"`
	EndChar   int    `json:"end_char"`
	Filename  string `json:"filename"`
}

// parseLocationFromMap parses location from a map structure
func parseLocationFromMap(locMap map[string]interface{}) *LocationInfo {
	uri, hasURI := locMap["uri"]
	lspRange, hasRange := locMap["range"]

	if !hasURI || !hasRange {
		return nil
	}

	uriStr, ok := uri.(string)
	if !ok {
		return nil
	}

	rangeMap, ok := lspRange.(map[string]interface{})
	if !ok {
		return nil
	}

	start, hasStart := rangeMap["start"]
	end, hasEnd := rangeMap["end"]

	if !hasStart || !hasEnd {
		return nil
	}

	startMap, ok := start.(map[string]interface{})
	if !ok {
		return nil
	}

	endMap, ok := end.(map[string]interface{})
	if !ok {
		return nil
	}

	startLine, hasStartLine := startMap["line"]
	startChar, hasStartChar := startMap["character"]
	endLine, hasEndLine := endMap["line"]
	endChar, hasEndChar := endMap["character"]

	if !hasStartLine || !hasStartChar || !hasEndLine || !hasEndChar {
		return nil
	}

	// Convert to integers
	startLineInt := int(startLine.(float64))
	startCharInt := int(startChar.(float64))
	endLineInt := int(endLine.(float64))
	endCharInt := int(endChar.(float64))

	// Extract filename from URI
	filename := filepath.Base(strings.TrimPrefix(uriStr, "file://"))

	return &LocationInfo{
		URI:       uriStr,
		StartLine: startLineInt,
		StartChar: startCharInt,
		EndLine:   endLineInt,
		EndChar:   endCharInt,
		Filename:  filename,
	}
}

// ExtractHoverContentFromResponse extracts hover content from various formats
func ExtractHoverContentFromResponse(response json.RawMessage) (string, string, error) {
	var hover map[string]interface{}
	if err := json.Unmarshal(response, &hover); err != nil {
		return "", "", fmt.Errorf("failed to parse hover response: %w", err)
	}

	contents, hasContents := hover["contents"]
	if !hasContents {
		return "", "", fmt.Errorf("hover response missing contents field")
	}

	content, markupKind := extractHoverContentAndKind(contents)
	return content, markupKind, nil
}

// extractHoverContentAndKind extracts content string and markup kind from hover contents
func extractHoverContentAndKind(contents interface{}) (string, string) {
	switch v := contents.(type) {
	case string:
		return v, "plaintext"

	case map[string]interface{}:
		// MarkupContent format
		if value, hasValue := v["value"]; hasValue {
			if valueStr, ok := value.(string); ok {
				kind := "plaintext"
				if kindValue, hasKind := v["kind"]; hasKind {
					if kindStr, ok := kindValue.(string); ok {
						kind = kindStr
					}
				}
				return valueStr, kind
			}
		}
		return "", "plaintext"

	case []interface{}:
		// Array of content items
		var parts []string
		kind := "plaintext"

		for _, item := range v {
			if itemContent, itemKind := extractHoverContentAndKind(item); itemContent != "" {
				parts = append(parts, itemContent)
				if itemKind != "plaintext" {
					kind = itemKind
				}
			}
		}

		return strings.Join(parts, "\n"), kind

	default:
		return fmt.Sprintf("%v", v), "plaintext"
	}
}

// ExtractSymbolsFromResponse extracts symbol information from document/workspace symbol responses
func ExtractSymbolsFromResponse(response json.RawMessage) ([]*SymbolInfo, error) {
	var symbols []interface{}
	if err := json.Unmarshal(response, &symbols); err != nil {
		return nil, fmt.Errorf("failed to parse symbols response: %w", err)
	}

	var symbolInfos []*SymbolInfo
	for _, symbol := range symbols {
		if symbolMap, ok := symbol.(map[string]interface{}); ok {
			if info := parseSymbolFromMap(symbolMap); info != nil {
				symbolInfos = append(symbolInfos, info)
			}
		}
	}

	return symbolInfos, nil
}

// SymbolInfo represents parsed symbol information
type SymbolInfo struct {
	Name          string        `json:"name"`
	Kind          int           `json:"kind"`
	KindName      string        `json:"kind_name"`
	Location      *LocationInfo `json:"location,omitempty"`
	ContainerName string        `json:"container_name,omitempty"`
	Detail        string        `json:"detail,omitempty"`
	Children      []*SymbolInfo `json:"children,omitempty"`
}

// parseSymbolFromMap parses symbol from a map structure
func parseSymbolFromMap(symbolMap map[string]interface{}) *SymbolInfo {
	name, hasName := symbolMap["name"]
	kind, hasKind := symbolMap["kind"]

	if !hasName || !hasKind {
		return nil
	}

	nameStr, ok := name.(string)
	if !ok {
		return nil
	}

	kindNum, ok := kind.(float64)
	if !ok {
		return nil
	}

	kindInt := int(kindNum)
	symbolInfo := &SymbolInfo{
		Name:     nameStr,
		Kind:     kindInt,
		KindName: ExpectedSymbolKind(kindInt).String(),
	}

	// Optional location
	if location, hasLocation := symbolMap["location"]; hasLocation {
		if locMap, ok := location.(map[string]interface{}); ok {
			symbolInfo.Location = parseLocationFromMap(locMap)
		}
	}

	// Optional container name
	if containerName, hasContainer := symbolMap["containerName"]; hasContainer {
		if containerStr, ok := containerName.(string); ok {
			symbolInfo.ContainerName = containerStr
		}
	}

	// Optional detail
	if detail, hasDetail := symbolMap["detail"]; hasDetail {
		if detailStr, ok := detail.(string); ok {
			symbolInfo.Detail = detailStr
		}
	}

	// Optional children (for document symbols)
	if children, hasChildren := symbolMap["children"]; hasChildren {
		if childrenArray, ok := children.([]interface{}); ok {
			for _, child := range childrenArray {
				if childMap, ok := child.(map[string]interface{}); ok {
					if childInfo := parseSymbolFromMap(childMap); childInfo != nil {
						symbolInfo.Children = append(symbolInfo.Children, childInfo)
					}
				}
			}
		}
	}

	return symbolInfo
}

// URI utilities

// NormalizeURI normalizes a URI for comparison
func NormalizeURI(uri string) string {
	// Ensure file:// scheme
	if !strings.HasPrefix(uri, "file://") {
		if strings.HasPrefix(uri, "/") {
			return "file://" + uri
		}
		return "file:///" + uri
	}
	return uri
}

// URIToFilename extracts filename from URI
func URIToFilename(uri string) string {
	path := strings.TrimPrefix(uri, "file://")
	return filepath.Base(path)
}

// URIMatches checks if a URI matches any of the given patterns
func URIMatches(uri string, patterns ...string) bool {
	for _, pattern := range patterns {
		if strings.Contains(uri, pattern) {
			return true
		}
		if matched, _ := regexp.MatchString(pattern, uri); matched {
			return true
		}
	}
	return false
}

// Timing utilities

// MeasureValidationTime measures the time taken for validation
func MeasureValidationTime(fn func() []*cases.ValidationResult) (time.Duration, []*cases.ValidationResult) {
	start := time.Now()
	results := fn()
	duration := time.Since(start)
	return duration, results
}

// Performance validation helpers

// ValidateResponseTime validates if response time is within acceptable bounds
func ValidateResponseTime(actual time.Duration, maxMs int, testName string) *cases.ValidationResult {
	maxDuration := time.Duration(maxMs) * time.Millisecond
	actualMs := actual.Milliseconds()

	if actual > maxDuration {
		return &cases.ValidationResult{
			Name:        fmt.Sprintf("%s_response_time", testName),
			Description: "Validate response time is within expected bounds",
			Passed:      false,
			Message:     fmt.Sprintf("Response time %dms exceeds maximum expected %dms", actualMs, maxMs),
			Details: map[string]interface{}{
				"actual_ms":       actualMs,
				"expected_max_ms": maxMs,
				"exceeded_by_ms":  actualMs - int64(maxMs),
			},
		}
	}

	return &cases.ValidationResult{
		Name:        fmt.Sprintf("%s_response_time", testName),
		Description: "Validate response time is within expected bounds",
		Passed:      true,
		Message:     fmt.Sprintf("Response time %dms is within expected maximum %dms", actualMs, maxMs),
		Details: map[string]interface{}{
			"actual_ms":       actualMs,
			"expected_max_ms": maxMs,
		},
	}
}

// Configuration utilities

// DefaultValidationConfig returns a default validation configuration
func DefaultValidationConfig() *lspconfig.ValidationConfig {
	return &lspconfig.ValidationConfig{
		StrictMode:        false,
		ValidateTypes:     true,
		ValidatePositions: true,
		ValidateURIs:      true,
	}
}

// StrictValidationConfig returns a strict validation configuration
func StrictValidationConfig() *lspconfig.ValidationConfig {
	return &lspconfig.ValidationConfig{
		StrictMode:        true,
		ValidateTypes:     true,
		ValidatePositions: true,
		ValidateURIs:      true,
	}
}

// LenientValidationConfig returns a lenient validation configuration
func LenientValidationConfig() *lspconfig.ValidationConfig {
	return &lspconfig.ValidationConfig{
		StrictMode:        false,
		ValidateTypes:     false,
		ValidatePositions: false,
		ValidateURIs:      false,
	}
}

// Test case utilities

// CreateTestCase creates a test case with common defaults
func CreateTestCase(id, name, method string) *cases.TestCase {
	return &cases.TestCase{
		ID:                id,
		Name:              name,
		Method:            method,
		Status:            cases.TestStatusPending,
		ValidationResults: make([]*cases.ValidationResult, 0),
	}
}

// Batch validation utilities

// ValidateTestCaseBatch validates multiple test cases in batch
func ValidateTestCaseBatch(testCases []*cases.TestCase, expectations map[string]*ComprehensiveExpectedResult, config *lspconfig.ValidationConfig) *BatchValidationResult {
	validator := NewComprehensiveValidator(config)

	result := &BatchValidationResult{
		TotalCases:  len(testCases),
		StartTime:   time.Now(),
		CaseResults: make(map[string]*TestCaseResult),
	}

	for _, testCase := range testCases {
		caseResult := &TestCaseResult{
			TestCase:  testCase,
			StartTime: time.Now(),
		}

		expected := expectations[testCase.ID]
		err := validator.ValidateTestCase(testCase, expected)

		caseResult.EndTime = time.Now()
		caseResult.Duration = caseResult.EndTime.Sub(caseResult.StartTime)
		caseResult.Error = err

		switch testCase.Status {
		case cases.TestStatusPassed:
			result.PassedCases++
		case cases.TestStatusFailed:
			result.FailedCases++
		case cases.TestStatusSkipped:
			result.SkippedCases++
		default:
			result.ErrorCases++
		}

		result.CaseResults[testCase.ID] = caseResult
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	return result
}

// BatchValidationResult represents the result of batch validation
type BatchValidationResult struct {
	TotalCases   int                        `json:"total_cases"`
	PassedCases  int                        `json:"passed_cases"`
	FailedCases  int                        `json:"failed_cases"`
	SkippedCases int                        `json:"skipped_cases"`
	ErrorCases   int                        `json:"error_cases"`
	StartTime    time.Time                  `json:"start_time"`
	EndTime      time.Time                  `json:"end_time"`
	Duration     time.Duration              `json:"duration"`
	CaseResults  map[string]*TestCaseResult `json:"case_results"`
}

// TestCaseResult represents the result of a single test case validation
type TestCaseResult struct {
	TestCase  *cases.TestCase `json:"test_case"`
	StartTime time.Time       `json:"start_time"`
	EndTime   time.Time       `json:"end_time"`
	Duration  time.Duration   `json:"duration"`
	Error     error           `json:"error,omitempty"`
}

// Summary returns a summary of the batch validation
func (r *BatchValidationResult) Summary() string {
	passRate := float64(0)
	if r.TotalCases > 0 {
		passRate = float64(r.PassedCases) / float64(r.TotalCases) * 100
	}

	return fmt.Sprintf("Batch validation: %d total, %d passed (%.1f%%), %d failed, %d skipped, %d errors, duration: %v",
		r.TotalCases, r.PassedCases, passRate, r.FailedCases, r.SkippedCases, r.ErrorCases, r.Duration)
}

// GetFailedCases returns the test cases that failed validation
func (r *BatchValidationResult) GetFailedCases() []*cases.TestCase {
	var failed []*cases.TestCase
	for _, caseResult := range r.CaseResults {
		if caseResult.TestCase.Status == cases.TestStatusFailed {
			failed = append(failed, caseResult.TestCase)
		}
	}
	return failed
}
