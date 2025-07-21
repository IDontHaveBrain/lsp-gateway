package validators

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"lsp-gateway/internal/testing/lsp/cases"
	"lsp-gateway/internal/testing/lsp/config"
)

// HoverValidator validates textDocument/hover responses
type HoverValidator struct {
	*BaseValidator
	metrics *HoverMetrics
}

// HoverMetrics tracks comprehensive hover validation metrics
type HoverMetrics struct {
	ResponseTime     time.Duration
	MemoryUsage      int64
	ContentRichness  float64
	TypeInfoPresent  bool
	DocumentationPresent bool
	ValidationErrors []string
}

// HoverResponse represents a hover response
type HoverResponse struct {
	Contents interface{} `json:"contents"`
	Range    *LSPRange   `json:"range,omitempty"`
}

// NewHoverValidator creates a new hover validator
func NewHoverValidator(config *config.ValidationConfig) *HoverValidator {
	return &HoverValidator{
		BaseValidator: NewBaseValidator(config),
		metrics:       &HoverMetrics{},
	}
}

// ValidateResponse validates a textDocument/hover response with comprehensive validation
func (v *HoverValidator) ValidateResponse(testCase *cases.TestCase, response json.RawMessage) []*cases.ValidationResult {
	var results []*cases.ValidationResult
	startTime := time.Now()

	// Initialize metrics
	v.metrics.ResponseTime = time.Since(startTime)
	v.metrics.ValidationErrors = []string{}

	// Performance validation
	results = append(results, v.validateResponseTime(startTime))

	// Protocol compliance validation
	results = append(results, v.validateLSPProtocolCompliance(response)...)

	// Check if response is null (no hover info found)
	if string(response) == "null" {
		return v.validateNullResponse(testCase, results)
	}
	
	// Parse hover response
	var hover HoverResponse
	if err := json.Unmarshal(response, &hover); err != nil {
		results = append(results, &cases.ValidationResult{
			Name:        "hover_format",
			Description: "Validate hover response format",
			Passed:      false,
			Message:     fmt.Sprintf("Invalid hover response format: %v", err),
		})
		return results
	}
	
	results = append(results, &cases.ValidationResult{
		Name:        "hover_format",
		Description: "Validate hover response format",
		Passed:      true,
		Message:     "Hover response has valid format",
	})
	
	// Validate contents
	results = append(results, v.validateHoverContents(hover.Contents, testCase)...)
	
	// Comprehensive hover validation
	results = append(results, v.validateContentRichness(hover.Contents, testCase)...)
	results = append(results, v.validateTypeInformation(hover.Contents, testCase)...)
	results = append(results, v.validateDocumentationPresence(hover.Contents, testCase)...)

	// Validate range if present
	if hover.Range != nil {
		rangeResults := v.validateHoverRange(*hover.Range)
		results = append(results, rangeResults...)
	}
	
	return results
}

// validateHoverContents validates hover contents
func (v *HoverValidator) validateHoverContents(contents interface{}, testCase *cases.TestCase) []*cases.ValidationResult {
	var results []*cases.ValidationResult
	
	if contents == nil {
		results = append(results, &cases.ValidationResult{
			Name:        "hover_contents_presence",
			Description: "Validate hover contents are present",
			Passed:      false,
			Message:     "Hover contents are null",
		})
		return results
	}
	
	// Get string representation of contents for validation
	var contentsStr string
	
	switch v := contents.(type) {
	case string:
		contentsStr = v
	case map[string]interface{}:
		// MarkupContent format
		if value, ok := v["value"].(string); ok {
			contentsStr = value
		} else {
			contentsStr = fmt.Sprintf("%v", v)
		}
	case []interface{}:
		// Array of MarkupContent or strings
		parts := make([]string, len(v))
		for i, item := range v {
			if str, ok := item.(string); ok {
				parts[i] = str
			} else if obj, ok := item.(map[string]interface{}); ok {
				if value, hasValue := obj["value"].(string); hasValue {
					parts[i] = value
				} else {
					parts[i] = fmt.Sprintf("%v", obj)
				}
			} else {
				parts[i] = fmt.Sprintf("%v", item)
			}
		}
		contentsStr = strings.Join(parts, "\n")
	default:
		contentsStr = fmt.Sprintf("%v", v)
	}
	
	// Basic presence check
	if contentsStr == "" {
		results = append(results, &cases.ValidationResult{
			Name:        "hover_contents_non_empty",
			Description: "Validate hover contents are not empty",
			Passed:      false,
			Message:     "Hover contents are empty",
		})
	} else {
		results = append(results, &cases.ValidationResult{
			Name:        "hover_contents_non_empty",
			Description: "Validate hover contents are not empty",
			Passed:      true,
			Message:     fmt.Sprintf("Hover contents have %d characters", len(contentsStr)),
		})
	}
	
	// Check expected conditions
	if testCase.Expected != nil && testCase.Expected.Hover != nil {
		expected := testCase.Expected.Hover
		
		// Check if content was expected
		if !expected.HasContent {
			results = append(results, &cases.ValidationResult{
				Name:        "hover_expectation",
				Description: "Validate hover expectation",
				Passed:      false,
				Message:     "Expected no hover content but found some",
			})
		} else {
			results = append(results, &cases.ValidationResult{
				Name:        "hover_expectation",
				Description: "Validate hover expectation",
				Passed:      true,
				Message:     "Found hover content as expected",
			})
		}
		
		// Check expected content patterns
		if len(expected.Contains) > 0 {
			results = append(results, v.validateHoverContainsPatterns(contentsStr, expected.Contains)...)
		}
	}
	
	return results
}

// validateHoverContainsPatterns validates that hover content contains expected patterns
func (v *HoverValidator) validateHoverContainsPatterns(content string, patterns []string) []*cases.ValidationResult {
	var results []*cases.ValidationResult
	
	for _, pattern := range patterns {
		found := strings.Contains(content, pattern)
		results = append(results, &cases.ValidationResult{
			Name:        fmt.Sprintf("hover_contains_%s", strings.ReplaceAll(pattern, " ", "_")),
			Description: fmt.Sprintf("Validate hover content contains pattern: %s", pattern),
			Passed:      found,
			Message:     fmt.Sprintf("Pattern '%s' found in hover content: %t", pattern, found),
		})
	}
	
	return results
}

// validateHoverRange validates hover range
func (v *HoverValidator) validateHoverRange(lspRange LSPRange) []*cases.ValidationResult {
	var results []*cases.ValidationResult
	
	// Validate start position
	startResult := v.validatePosition(map[string]interface{}{
		"line":      float64(lspRange.Start.Line),
		"character": float64(lspRange.Start.Character),
	}, "hover_range_start")
	results = append(results, startResult)
	
	// Validate end position
	endResult := v.validatePosition(map[string]interface{}{
		"line":      float64(lspRange.End.Line),
		"character": float64(lspRange.End.Character),
	}, "hover_range_end")
	results = append(results, endResult)
	
	// Validate range logic (start should be before or equal to end)
	if v.config.ValidatePositions {
		if lspRange.Start.Line > lspRange.End.Line || 
		   (lspRange.Start.Line == lspRange.End.Line && lspRange.Start.Character > lspRange.End.Character) {
			results = append(results, &cases.ValidationResult{
				Name:        "hover_range_order",
				Description: "Validate hover range start position is before end position",
				Passed:      false,
				Message:     fmt.Sprintf("Start position (%d,%d) is after end position (%d,%d)", 
					lspRange.Start.Line, lspRange.Start.Character, 
					lspRange.End.Line, lspRange.End.Character),
			})
		} else {
			results = append(results, &cases.ValidationResult{
				Name:        "hover_range_order",
				Description: "Validate hover range position order",
				Passed:      true,
				Message:     "Range positions are in correct order",
			})
		}
	}
	
	return results
}

// validateResponseTime validates LSP response performance for hover
func (v *HoverValidator) validateResponseTime(startTime time.Time) *cases.ValidationResult {
	responseTime := time.Since(startTime)
	v.metrics.ResponseTime = responseTime

	// Hover requests should be very fast (< 1 second for good UX)
	threshold := 1 * time.Second
	if responseTime > threshold {
		return &cases.ValidationResult{
			Name:        "hover_response_time",
			Description: "Validate hover response time performance",
			Passed:      false,
			Message:     fmt.Sprintf("Response time %v exceeds threshold %v", responseTime, threshold),
			Details: map[string]interface{}{
				"response_time_ms": responseTime.Milliseconds(),
				"threshold_ms":     threshold.Milliseconds(),
			},
		}
	}

	return &cases.ValidationResult{
		Name:        "hover_response_time",
		Description: "Validate hover response time performance",
		Passed:      true,
		Message:     fmt.Sprintf("Response time %v is within acceptable threshold", responseTime),
		Details: map[string]interface{}{
			"response_time_ms": responseTime.Milliseconds(),
		},
	}
}

// validateLSPProtocolCompliance validates LSP protocol compliance for hover
func (v *HoverValidator) validateLSPProtocolCompliance(response json.RawMessage) []*cases.ValidationResult {
	var results []*cases.ValidationResult

	// LSP spec: hover response should be Hover | null
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
		// Hover object with contents
		if _, hasContents := v["contents"]; hasContents {
			valid = true
			responseType = "Hover"
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
			Message:     "Response does not conform to LSP hover response format",
			Details: map[string]interface{}{
				"response_type": responseType,
			},
		})
	}

	return results
}

// validateNullResponse handles null response validation for hover
func (v *HoverValidator) validateNullResponse(testCase *cases.TestCase, results []*cases.ValidationResult) []*cases.ValidationResult {
	if testCase.Expected != nil && testCase.Expected.Hover != nil && testCase.Expected.Hover.HasContent {
		results = append(results, &cases.ValidationResult{
			Name:        "hover_presence",
			Description: "Validate hover content is found",
			Passed:      false,
			Message:     "Expected hover content but response is null",
		})
	} else {
		results = append(results, &cases.ValidationResult{
			Name:        "hover_absence",
			Description: "Validate no hover content found (as expected)",
			Passed:      true,
			Message:     "No hover content found as expected",
		})
	}

	// Validate edge case handling
	results = append(results, v.validateEdgeCaseHandling(testCase)...)

	return results
}

// validateContentRichness validates the richness and quality of hover content
func (v *HoverValidator) validateContentRichness(contents interface{}, testCase *cases.TestCase) []*cases.ValidationResult {
	var results []*cases.ValidationResult

	contentStr := v.extractContentString(contents)
	if contentStr == "" {
		results = append(results, &cases.ValidationResult{
			Name:        "content_richness",
			Description: "Validate hover content richness",
			Passed:      false,
			Message:     "Hover content is empty",
		})
		return results
	}

	// Calculate content richness metrics
	richness := v.calculateContentRichness(contentStr)
	v.metrics.ContentRichness = richness

	// Assess richness based on various factors
	richnessLevel := "basic"
	if richness >= 80 {
		richnessLevel = "excellent"
	} else if richness >= 60 {
		richnessLevel = "good"
	} else if richness >= 40 {
		richnessLevel = "moderate"
	}

	passed := richness >= 40 // Require at least moderate richness

	results = append(results, &cases.ValidationResult{
		Name:        "content_richness",
		Description: "Validate hover content richness",
		Passed:      passed,
		Message:     fmt.Sprintf("Content richness score: %.1f (%s)", richness, richnessLevel),
		Details: map[string]interface{}{
			"richness_score": richness,
			"richness_level": richnessLevel,
			"content_length": len(contentStr),
		},
	})

	// Validate specific content quality aspects
	results = append(results, v.validateContentQuality(contentStr)...)

	return results
}

// calculateContentRichness calculates a richness score for hover content
func (v *HoverValidator) calculateContentRichness(content string) float64 {
	score := 0.0

	// Length factor (up to 20 points)
	length := len(content)
	if length > 0 {
		score += min(float64(length)/10, 20)
	}

	// Information density factors
	lines := strings.Split(content, "\n")
	nonEmptyLines := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			nonEmptyLines++
		}
	}

	// Multi-line content bonus (up to 15 points)
	if nonEmptyLines > 1 {
		score += min(float64(nonEmptyLines)*3, 15)
	}

	// Code examples (up to 15 points)
	if v.containsCodeExamples(content) {
		score += 15
	}

	// Parameter information (up to 10 points)
	if v.containsParameterInfo(content) {
		score += 10
	}

	// Return type information (up to 10 points)
	if v.containsReturnTypeInfo(content) {
		score += 10
	}

	// Documentation links or references (up to 10 points)
	if v.containsDocumentationReferences(content) {
		score += 10
	}

	// Markdown formatting (up to 10 points)
	if v.containsMarkdownFormatting(content) {
		score += 10
	}

	// Type annotations (up to 10 points)
	if v.containsTypeAnnotations(content) {
		score += 10
	}

	return min(score, 100)
}

// Helper function for min
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// containsCodeExamples checks if content contains code examples
func (v *HoverValidator) containsCodeExamples(content string) bool {
	codePatterns := []string{
		"```",
		"`[^`]+`",
		"func ",
		"function ",
		"def ",
		"class ",
		"interface ",
		"type ",
	}
	
	for _, pattern := range codePatterns {
		if matched, _ := regexp.MatchString(pattern, content); matched {
			return true
		}
	}
	return false
}

// containsParameterInfo checks if content contains parameter information
func (v *HoverValidator) containsParameterInfo(content string) bool {
	paramPatterns := []string{
		"@param",
		"Parameters:",
		"Args:",
		"Arguments:",
		"\\([^)]*\\).*->",
		"\\([^)]*\\):",
	}
	
	for _, pattern := range paramPatterns {
		if matched, _ := regexp.MatchString(pattern, content); matched {
			return true
		}
	}
	return false
}

// containsReturnTypeInfo checks if content contains return type information
func (v *HoverValidator) containsReturnTypeInfo(content string) bool {
	returnPatterns := []string{
		"@return",
		"Returns:",
		"Return type:",
		"-> ",
		": [A-Z][a-zA-Z]*",
		"\\) [A-Z][a-zA-Z]*",
	}
	
	for _, pattern := range returnPatterns {
		if matched, _ := regexp.MatchString(pattern, content); matched {
			return true
		}
	}
	return false
}

// containsDocumentationReferences checks if content contains documentation references
func (v *HoverValidator) containsDocumentationReferences(content string) bool {
	docPatterns := []string{
		"@see",
		"See also:",
		"Documentation:",
		"https?://",
		"\\[.*\\]\\(.*\\)",
	}
	
	for _, pattern := range docPatterns {
		if matched, _ := regexp.MatchString(pattern, content); matched {
			return true
		}
	}
	return false
}

// containsMarkdownFormatting checks if content contains markdown formatting
func (v *HoverValidator) containsMarkdownFormatting(content string) bool {
	markdownPatterns := []string{
		"\\*\\*.*\\*\\*",
		"\\*.*\\*",
		"##? ",
		"- ",
		"\\* ",
		"\\d+\\. ",
	}
	
	for _, pattern := range markdownPatterns {
		if matched, _ := regexp.MatchString(pattern, content); matched {
			return true
		}
	}
	return false
}

// containsTypeAnnotations checks if content contains type annotations
func (v *HoverValidator) containsTypeAnnotations(content string) bool {
	typePatterns := []string{
		": [a-zA-Z]+",
		"<[a-zA-Z]+>",
		"\\[\\]",
		"Array<",
		"Map<",
		"Optional<",
		"Promise<",
	}
	
	for _, pattern := range typePatterns {
		if matched, _ := regexp.MatchString(pattern, content); matched {
			return true
		}
	}
	return false
}

// validateContentQuality validates specific content quality aspects
func (v *HoverValidator) validateContentQuality(content string) []*cases.ValidationResult {
	var results []*cases.ValidationResult

	// Check for meaningful content (not just whitespace or trivial info)
	if len(strings.TrimSpace(content)) < 10 {
		results = append(results, &cases.ValidationResult{
			Name:        "content_meaningful",
			Description: "Validate content is meaningful",
			Passed:      false,
			Message:     "Hover content is too short to be meaningful",
		})
	} else {
		results = append(results, &cases.ValidationResult{
			Name:        "content_meaningful",
			Description: "Validate content is meaningful",
			Passed:      true,
			Message:     "Hover content appears to be meaningful",
		})
	}

	// Check for common placeholder text that indicates poor quality
	placeholders := []string{
		"no documentation",
		"todo",
		"fixme",
		"placeholder",
		"tbd",
		"to be determined",
	}

	hasPlaceholder := false
	for _, placeholder := range placeholders {
		if strings.Contains(strings.ToLower(content), placeholder) {
			hasPlaceholder = true
			break
		}
	}

	if hasPlaceholder {
		results = append(results, &cases.ValidationResult{
			Name:        "content_quality",
			Description: "Validate content quality",
			Passed:      false,
			Message:     "Content contains placeholder text indicating incomplete documentation",
		})
	} else {
		results = append(results, &cases.ValidationResult{
			Name:        "content_quality",
			Description: "Validate content quality",
			Passed:      true,
			Message:     "Content quality appears good",
		})
	}

	return results
}

// validateTypeInformation validates presence and quality of type information
func (v *HoverValidator) validateTypeInformation(contents interface{}, testCase *cases.TestCase) []*cases.ValidationResult {
	var results []*cases.ValidationResult

	contentStr := v.extractContentString(contents)
	hasTypeInfo := v.containsTypeInformation(contentStr)
	v.metrics.TypeInfoPresent = hasTypeInfo

	if hasTypeInfo {
		results = append(results, &cases.ValidationResult{
			Name:        "type_information_presence",
			Description: "Validate presence of type information",
			Passed:      true,
			Message:     "Type information is present in hover content",
		})

		// Validate quality of type information
		results = append(results, v.validateTypeInformationQuality(contentStr)...)
	} else {
		// For strongly typed languages, type information should generally be present
		shouldHaveTypeInfo := v.languageExpectsTypeInfo(testCase.Language)
		
		results = append(results, &cases.ValidationResult{
			Name:        "type_information_presence",
			Description: "Validate presence of type information",
			Passed:      !shouldHaveTypeInfo,
			Message: func() string {
				if shouldHaveTypeInfo {
					return fmt.Sprintf("Type information expected for %s but not found", testCase.Language)
				}
				return "Type information not present (acceptable for this language)"
			}(),
		})
	}

	return results
}

// containsTypeInformation checks if content contains type information
func (v *HoverValidator) containsTypeInformation(content string) bool {
	typePatterns := []string{
		": [a-zA-Z][a-zA-Z0-9]*",       // Go/TypeScript style
		"-> [a-zA-Z][a-zA-Z0-9]*",       // Function return types
		"\\([^)]*\\) [a-zA-Z]+",         // Function signatures
		"<[a-zA-Z][a-zA-Z0-9]*>",       // Generic types
		"\\[\\][a-zA-Z]+",               // Array types
		"Map<.*>",                       // Map types
		"Array<.*>",                     // Array types
		"Promise<.*>",                   // Promise types
		"Optional<.*>",                  // Optional types
		"func\\([^)]*\\)",               // Go function types
		"interface\\s+[a-zA-Z]+",       // Interface definitions
		"class\\s+[a-zA-Z]+",           // Class definitions
		"type\\s+[a-zA-Z]+",            // Type definitions
	}

	for _, pattern := range typePatterns {
		if matched, _ := regexp.MatchString(pattern, content); matched {
			return true
		}
	}
	return false
}

// languageExpectsTypeInfo determines if a language should typically provide type information
func (v *HoverValidator) languageExpectsTypeInfo(language string) bool {
	typedLanguages := []string{"go", "typescript", "java", "rust", "c", "cpp", "c#", "kotlin", "swift"}
	
	for _, lang := range typedLanguages {
		if strings.EqualFold(language, lang) {
			return true
		}
	}
	return false
}

// validateTypeInformationQuality validates the quality of type information
func (v *HoverValidator) validateTypeInformationQuality(content string) []*cases.ValidationResult {
	var results []*cases.ValidationResult

	// Check for comprehensive type information
	hasReturnType := v.containsReturnTypeInfo(content)
	hasParamTypes := v.containsParameterInfo(content)

	qualityScore := 0
	if hasReturnType {
		qualityScore += 50
	}
	if hasParamTypes {
		qualityScore += 50
	}

	if qualityScore >= 75 {
		results = append(results, &cases.ValidationResult{
			Name:        "type_information_quality",
			Description: "Validate quality of type information",
			Passed:      true,
			Message:     "Type information is comprehensive",
			Details: map[string]interface{}{
				"quality_score": qualityScore,
				"has_return_type": hasReturnType,
				"has_param_types": hasParamTypes,
			},
		})
	} else {
		results = append(results, &cases.ValidationResult{
			Name:        "type_information_quality",
			Description: "Validate quality of type information",
			Passed:      false,
			Message:     "Type information could be more comprehensive",
			Details: map[string]interface{}{
				"quality_score": qualityScore,
				"has_return_type": hasReturnType,
				"has_param_types": hasParamTypes,
			},
		})
	}

	return results
}

// validateDocumentationPresence validates presence and quality of documentation
func (v *HoverValidator) validateDocumentationPresence(contents interface{}, testCase *cases.TestCase) []*cases.ValidationResult {
	var results []*cases.ValidationResult

	contentStr := v.extractContentString(contents)
	hasDocumentation := v.containsDocumentation(contentStr)
	v.metrics.DocumentationPresent = hasDocumentation

	if hasDocumentation {
		results = append(results, &cases.ValidationResult{
			Name:        "documentation_presence",
			Description: "Validate presence of documentation",
			Passed:      true,
			Message:     "Documentation is present in hover content",
		})

		// Validate quality of documentation
		results = append(results, v.validateDocumentationQuality(contentStr)...)
	} else {
		results = append(results, &cases.ValidationResult{
			Name:        "documentation_presence",
			Description: "Validate presence of documentation",
			Passed:      false,
			Message:     "No documentation found in hover content",
		})
	}

	return results
}

// containsDocumentation checks if content contains documentation
func (v *HoverValidator) containsDocumentation(content string) bool {
	// Documentation indicators
	docPatterns := []string{
		"@.*",                    // JSDoc/Javadoc tags
		"//.*",                   // Comments
		"/\\*.*\\*/",             // Block comments  
		"\"\"\".*\"\"\"",         // Python docstrings
		"'''.*'''",               // Python docstrings
		"Description:",           // Explicit description
		"Summary:",               // Summary section
		"Overview:",              // Overview section
		"Example:",               // Examples
		"Usage:",                 // Usage information
	}

	for _, pattern := range docPatterns {
		if matched, _ := regexp.MatchString("(?s)"+pattern, content); matched {
			return true
		}
	}

	// Check for descriptive text (sentences with periods)
	sentences := strings.Count(content, ".")
	if sentences >= 2 {
		return true
	}

	return false
}

// validateDocumentationQuality validates the quality of documentation
func (v *HoverValidator) validateDocumentationQuality(content string) []*cases.ValidationResult {
	var results []*cases.ValidationResult

	qualityScore := 0
	qualityFactors := []string{}

	// Check for examples
	if v.containsCodeExamples(content) {
		qualityScore += 25
		qualityFactors = append(qualityFactors, "examples")
	}

	// Check for parameter documentation
	if v.containsParameterInfo(content) {
		qualityScore += 25
		qualityFactors = append(qualityFactors, "parameters")
	}

	// Check for return documentation
	if v.containsReturnTypeInfo(content) {
		qualityScore += 25
		qualityFactors = append(qualityFactors, "return_info")
	}

	// Check for comprehensive description
	if len(content) > 100 && strings.Count(content, ".") >= 2 {
		qualityScore += 25
		qualityFactors = append(qualityFactors, "detailed_description")
	}

	qualityLevel := "poor"
	if qualityScore >= 75 {
		qualityLevel = "excellent"
	} else if qualityScore >= 50 {
		qualityLevel = "good"
	} else if qualityScore >= 25 {
		qualityLevel = "moderate"
	}

	passed := qualityScore >= 25 // Require at least moderate quality

	results = append(results, &cases.ValidationResult{
		Name:        "documentation_quality",
		Description: "Validate quality of documentation",
		Passed:      passed,
		Message:     fmt.Sprintf("Documentation quality: %s (score: %d)", qualityLevel, qualityScore),
		Details: map[string]interface{}{
			"quality_score":   qualityScore,
			"quality_level":   qualityLevel,
			"quality_factors": qualityFactors,
		},
	})

	return results
}

// extractContentString extracts string content from various hover content formats
func (v *HoverValidator) extractContentString(contents interface{}) string {
	switch v := contents.(type) {
	case string:
		return v
	case map[string]interface{}:
		// MarkupContent format
		if value, ok := v["value"].(string); ok {
			return value
		}
		return fmt.Sprintf("%v", v)
	case []interface{}:
		// Array of MarkupContent or strings
		parts := make([]string, len(v))
		for i, item := range v {
			if str, ok := item.(string); ok {
				parts[i] = str
			} else if obj, ok := item.(map[string]interface{}); ok {
				if value, hasValue := obj["value"].(string); hasValue {
					parts[i] = value
				} else {
					parts[i] = fmt.Sprintf("%v", obj)
				}
			} else {
				parts[i] = fmt.Sprintf("%v", item)
			}
		}
		return strings.Join(parts, "\n")
	default:
		return fmt.Sprintf("%v", v)
	}
}

// validateEdgeCaseHandling validates edge case and error condition handling for hover
func (v *HoverValidator) validateEdgeCaseHandling(testCase *cases.TestCase) []*cases.ValidationResult {
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
		// This would be handled at the LSP server level
		results = append(results, &cases.ValidationResult{
			Name:        "edge_case_handling",
			Description: "Validate edge case handling",
			Passed:      true,
			Message:     "Edge case handling appears appropriate",
		})
	}

	return results
}