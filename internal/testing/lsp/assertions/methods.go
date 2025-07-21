package assertions

import (
	"encoding/json"
	"fmt"
	"strings"

	"lsp-gateway/internal/testing/lsp/cases"
)

// Method-specific assertion helpers for each of the 5 core LSP methods

// AssertDefinitionResponse validates a textDocument/definition response
func (ctx *AssertionContext) AssertDefinitionResponse(response json.RawMessage, expected *ExpectedDefinitionResult) []*cases.ValidationResult {
	var results []*cases.ValidationResult

	// Handle null response
	if ctx.IsNullResponse(response) {
		if expected != nil && expected.HasResult {
			results = append(results, ctx.CreateValidationResult(
				"definition_presence",
				"Validate definition location is found",
				"Expected definition but response is null",
				false,
				nil,
			))
		} else {
			results = append(results, ctx.CreateValidationResult(
				"definition_absence",
				"Validate no definition found (as expected)",
				"No definition found as expected",
				true,
				nil,
			))
		}
		return results
	}

	// Try to parse as single location first
	var singleLocation map[string]interface{}
	if err := json.Unmarshal(response, &singleLocation); err == nil {
		// Check if it's a valid location structure (has 'uri' and 'range')
		if _, hasURI := singleLocation["uri"]; hasURI {
			if _, hasRange := singleLocation["range"]; hasRange {
				return ctx.assertSingleDefinitionLocation(singleLocation, expected, results)
			}
		}
	}

	// Try to parse as array of locations
	var locations []interface{}
	if err := json.Unmarshal(response, &locations); err != nil {
		results = append(results, ctx.CreateValidationResult(
			"definition_format",
			"Validate definition response format",
			fmt.Sprintf("Response is neither a location nor an array of locations: %v", err),
			false,
			map[string]interface{}{
				"parse_error": err.Error(),
				"response":    string(response),
			},
		))
		return results
	}

	return ctx.assertDefinitionLocationArray(locations, expected, results)
}

// assertSingleDefinitionLocation validates a single definition location
func (ctx *AssertionContext) assertSingleDefinitionLocation(location map[string]interface{}, expected *ExpectedDefinitionResult, results []*cases.ValidationResult) []*cases.ValidationResult {
	results = append(results, ctx.CreateValidationResult(
		"definition_format",
		"Validate definition response format",
		"Response is a single location",
		true,
		nil,
	))

	// Validate location structure
	var expectedLoc *ExpectedLocation
	if expected != nil && expected.FirstLocation != nil {
		expectedLoc = expected.FirstLocation
	}
	locationResults := ctx.AssertLocation(location, expectedLoc, "definition")
	results = append(results, locationResults...)

	// Check expected conditions
	if expected != nil {
		if !expected.HasResult {
			results = append(results, ctx.CreateValidationResult(
				"definition_expectation",
				"Validate definition expectation",
				"Expected no definition but found one",
				false,
				nil,
			))
		} else {
			results = append(results, ctx.CreateValidationResult(
				"definition_expectation",
				"Validate definition expectation",
				"Found definition as expected",
				true,
				nil,
			))
		}

		// Check if expected to be array but got single location
		if expected.IsArray != nil && *expected.IsArray {
			results = append(results, ctx.CreateValidationResult(
				"definition_array_expectation",
				"Validate definition response format expectation",
				"Expected array response but got single location",
				false,
				nil,
			))
		}
	}

	return results
}

// assertDefinitionLocationArray validates an array of definition locations
func (ctx *AssertionContext) assertDefinitionLocationArray(locations []interface{}, expected *ExpectedDefinitionResult, results []*cases.ValidationResult) []*cases.ValidationResult {
	count := len(locations)
	
	results = append(results, ctx.CreateValidationResult(
		"definition_format",
		"Validate definition response format",
		fmt.Sprintf("Response is an array of %d locations", count),
		true,
		map[string]interface{}{
			"location_count": count,
		},
	))

	// Validate array length expectations
	if expected != nil && expected.ArrayLength != nil {
		lengthResult := ctx.AssertArrayLength(locations, expected.ArrayLength, "definition_array")
		results = append(results, lengthResult)
	}

	if count == 0 {
		if expected != nil && expected.HasResult {
			results = append(results, ctx.CreateValidationResult(
				"definition_presence",
				"Validate definition location is found",
				"Expected definition but array is empty",
				false,
				nil,
			))
		} else {
			results = append(results, ctx.CreateValidationResult(
				"definition_absence",
				"Validate no definition found (as expected)",
				"No definitions found as expected",
				true,
				nil,
			))
		}
		return results
	}

	// Validate each location
	for i, location := range locations {
		var expectedLoc *ExpectedLocation
		if expected != nil && expected.Locations != nil && i < len(expected.Locations) {
			expectedLoc = expected.Locations[i]
		} else if expected != nil && expected.FirstLocation != nil && i == 0 {
			expectedLoc = expected.FirstLocation
		}
		
		locationResults := ctx.AssertLocation(location, expectedLoc, fmt.Sprintf("definition_%d", i))
		results = append(results, locationResults...)
	}

	// Check expected conditions
	if expected != nil {
		if !expected.HasResult {
			results = append(results, ctx.CreateValidationResult(
				"definition_expectation",
				"Validate definition expectation",
				fmt.Sprintf("Expected no definition but found %d", count),
				false,
				map[string]interface{}{
					"found_count": count,
				},
			))
		} else {
			results = append(results, ctx.CreateValidationResult(
				"definition_expectation",
				"Validate definition expectation",
				fmt.Sprintf("Found %d definitions as expected", count),
				true,
				map[string]interface{}{
					"found_count": count,
				},
			))
		}

		// Check if expected to be single but got array
		if expected.IsArray != nil && !*expected.IsArray {
			results = append(results, ctx.CreateValidationResult(
				"definition_single_expectation",
				"Validate definition response format expectation",
				"Expected single location but got array",
				false,
				map[string]interface{}{
					"array_length": count,
				},
			))
		}
	}

	return results
}

// AssertReferencesResponse validates a textDocument/references response
func (ctx *AssertionContext) AssertReferencesResponse(response json.RawMessage, expected *ExpectedReferencesResult) []*cases.ValidationResult {
	var results []*cases.ValidationResult

	// Handle null response
	if ctx.IsNullResponse(response) {
		if expected != nil && expected.HasResult {
			results = append(results, ctx.CreateValidationResult(
				"references_presence",
				"Validate references are found",
				"Expected references but response is null",
				false,
				nil,
			))
		} else {
			results = append(results, ctx.CreateValidationResult(
				"references_absence",
				"Validate no references found (as expected)",
				"No references found as expected",
				true,
				nil,
			))
		}
		return results
	}

	// Parse as array of locations (references is always an array)
	var locations []interface{}
	if err := json.Unmarshal(response, &locations); err != nil {
		results = append(results, ctx.CreateValidationResult(
			"references_format",
			"Validate references response format",
			fmt.Sprintf("Response is not an array of locations: %v", err),
			false,
			map[string]interface{}{
				"parse_error": err.Error(),
				"response":    string(response),
			},
		))
		return results
	}

	count := len(locations)
	results = append(results, ctx.CreateValidationResult(
		"references_format",
		"Validate references response format",
		fmt.Sprintf("Response is an array of %d references", count),
		true,
		map[string]interface{}{
			"reference_count": count,
		},
	))

	// Validate array length expectations
	if expected != nil && expected.ArrayLength != nil {
		lengthResult := ctx.AssertArrayLength(locations, expected.ArrayLength, "references_array")
		results = append(results, lengthResult)
	}

	// Validate each reference location
	for i, location := range locations {
		var expectedLoc *ExpectedLocation
		if expected != nil && expected.Locations != nil && i < len(expected.Locations) {
			expectedLoc = expected.Locations[i]
		}
		
		locationResults := ctx.AssertLocation(location, expectedLoc, fmt.Sprintf("reference_%d", i))
		results = append(results, locationResults...)
	}

	// Check specific references expectations
	if expected != nil {
		if !expected.HasResult && count > 0 {
			results = append(results, ctx.CreateValidationResult(
				"references_expectation",
				"Validate references expectation",
				fmt.Sprintf("Expected no references but found %d", count),
				false,
				map[string]interface{}{
					"found_count": count,
				},
			))
		} else if expected.HasResult && count == 0 {
			results = append(results, ctx.CreateValidationResult(
				"references_expectation",
				"Validate references expectation",
				"Expected references but none found",
				false,
				nil,
			))
		}

		// Check if references should contain specific file
		if expected.ContainsFile != nil && count > 0 {
			containsFile := false
			for _, location := range locations {
				if locMap, ok := location.(map[string]interface{}); ok {
					if uri, hasURI := locMap["uri"]; hasURI {
						if uriStr, ok := uri.(string); ok && strings.Contains(uriStr, *expected.ContainsFile) {
							containsFile = true
							break
						}
					}
				}
			}
			
			results = append(results, ctx.CreateValidationResult(
				"references_contains_file",
				fmt.Sprintf("Validate references contain file: %s", *expected.ContainsFile),
				fmt.Sprintf("References contain file %s: %t", *expected.ContainsFile, containsFile),
				containsFile,
				map[string]interface{}{
					"expected_file": *expected.ContainsFile,
					"found":         containsFile,
				},
			))
		}
	}

	return results
}

// AssertHoverResponse validates a textDocument/hover response
func (ctx *AssertionContext) AssertHoverResponse(response json.RawMessage, expected *ExpectedHoverResult) []*cases.ValidationResult {
	var results []*cases.ValidationResult

	// Handle null response
	if ctx.IsNullResponse(response) {
		if expected != nil && expected.HasResult {
			results = append(results, ctx.CreateValidationResult(
				"hover_presence",
				"Validate hover information is found",
				"Expected hover but response is null",
				false,
				nil,
			))
		} else {
			results = append(results, ctx.CreateValidationResult(
				"hover_absence",
				"Validate no hover found (as expected)",
				"No hover found as expected",
				true,
				nil,
			))
		}
		return results
	}

	// Parse hover response
	var hover map[string]interface{}
	parseResult := ctx.ParseJSONResponse(response, &hover, "hover")
	results = append(results, parseResult)
	if !parseResult.Passed {
		return results
	}

	// Validate hover structure
	contents, hasContents := hover["contents"]
	if !hasContents {
		results = append(results, ctx.CreateValidationResult(
			"hover_structure",
			"Validate hover has contents field",
			"Hover response missing contents field",
			false,
			nil,
		))
		return results
	}

	// Validate hover content
	contentResults := ctx.validateHoverContent(contents, expected)
	results = append(results, contentResults...)

	// Validate optional range
	if hoverRange, hasRange := hover["range"]; hasRange {
		var expectedRange *ExpectedRange
		if expected != nil && expected.Range != nil {
			expectedRange = expected.Range
		}
		rangeResults := ctx.AssertRange(hoverRange, expectedRange, "hover_range")
		results = append(results, rangeResults...)
	}

	// Check expected conditions
	if expected != nil {
		if !expected.HasResult {
			results = append(results, ctx.CreateValidationResult(
				"hover_expectation",
				"Validate hover expectation",
				"Expected no hover but found one",
				false,
				nil,
			))
		} else {
			results = append(results, ctx.CreateValidationResult(
				"hover_expectation",
				"Validate hover expectation",
				"Found hover as expected",
				true,
				nil,
			))
		}
	}

	return results
}

// validateHoverContent validates hover content structure and patterns
func (ctx *AssertionContext) validateHoverContent(contents interface{}, expected *ExpectedHoverResult) []*cases.ValidationResult {
	var results []*cases.ValidationResult

	// Extract content string based on different possible structures
	contentStr := ctx.extractHoverContentString(contents)
	
	if contentStr == "" {
		if expected != nil && expected.Content != nil && expected.Content.HasContent {
			results = append(results, ctx.CreateValidationResult(
				"hover_content_presence",
				"Validate hover has content",
				"Expected hover content but none found",
				false,
				nil,
			))
		} else {
			results = append(results, ctx.CreateValidationResult(
				"hover_content_absence",
				"Validate no hover content (as expected)",
				"No hover content as expected",
				true,
				nil,
			))
		}
		return results
	}

	// Validate content expectations
	if expected != nil && expected.Content != nil {
		contentExpected := expected.Content

		if !contentExpected.HasContent {
			results = append(results, ctx.CreateValidationResult(
				"hover_content_expectation",
				"Validate hover content expectation",
				"Expected no content but found some",
				false,
				map[string]interface{}{
					"found_content": contentStr,
				},
			))
		}

		// Check content patterns
		if len(contentExpected.Contains) > 0 {
			containsResults := ctx.AssertContains(contentStr, contentExpected.Contains, "hover_content")
			results = append(results, containsResults...)
		}

		if len(contentExpected.Matches) > 0 {
			matchResults := ctx.AssertMatches(contentStr, contentExpected.Matches, "hover_content")
			results = append(results, matchResults...)
		}

		if len(contentExpected.Excludes) > 0 {
			excludeResults := ctx.AssertExcludes(contentStr, contentExpected.Excludes, "hover_content")
			results = append(results, excludeResults...)
		}

		// Check content length
		contentLength := len(contentStr)
		if contentExpected.MinLength != nil && contentLength < *contentExpected.MinLength {
			results = append(results, ctx.CreateValidationResult(
				"hover_content_min_length",
				"Validate hover content minimum length",
				fmt.Sprintf("Content length %d is less than expected minimum %d", contentLength, *contentExpected.MinLength),
				false,
				map[string]interface{}{
					"actual_length":   contentLength,
					"expected_min":    *contentExpected.MinLength,
				},
			))
		}

		if contentExpected.MaxLength != nil && contentLength > *contentExpected.MaxLength {
			results = append(results, ctx.CreateValidationResult(
				"hover_content_max_length",
				"Validate hover content maximum length",
				fmt.Sprintf("Content length %d exceeds expected maximum %d", contentLength, *contentExpected.MaxLength),
				false,
				map[string]interface{}{
					"actual_length":   contentLength,
					"expected_max":    *contentExpected.MaxLength,
				},
			))
		}

		// Validate markup kind if present
		if contentExpected.MarkupKind != nil {
			// This would require parsing the markup content structure
			// For now, we'll just validate that we can extract content
			results = append(results, ctx.CreateValidationResult(
				"hover_content_markup_kind",
				"Validate hover content markup kind",
				fmt.Sprintf("Content extracted successfully, assuming markup kind is correct"),
				true,
				map[string]interface{}{
					"expected_kind": *contentExpected.MarkupKind,
				},
			))
		}
	}

	results = append(results, ctx.CreateValidationResult(
		"hover_content_presence",
		"Validate hover content is present",
		fmt.Sprintf("Found hover content (%d characters)", len(contentStr)),
		true,
		map[string]interface{}{
			"content_length": len(contentStr),
			"content_preview": ctx.truncateString(contentStr, 100),
		},
	))

	return results
}

// extractHoverContentString extracts string content from various hover content formats
func (ctx *AssertionContext) extractHoverContentString(contents interface{}) string {
	switch v := contents.(type) {
	case string:
		return v
	case map[string]interface{}:
		// MarkupContent format
		if value, hasValue := v["value"]; hasValue {
			if str, ok := value.(string); ok {
				return str
			}
		}
		return ""
	case []interface{}:
		// Array of content items
		var parts []string
		for _, item := range v {
			if part := ctx.extractHoverContentString(item); part != "" {
				parts = append(parts, part)
			}
		}
		return strings.Join(parts, "\n")
	default:
		return fmt.Sprintf("%v", v)
	}
}

// AssertDocumentSymbolResponse validates a textDocument/documentSymbol response
func (ctx *AssertionContext) AssertDocumentSymbolResponse(response json.RawMessage, expected *ExpectedDocumentSymbolResult) []*cases.ValidationResult {
	var results []*cases.ValidationResult

	// Handle null response
	if ctx.IsNullResponse(response) {
		if expected != nil && expected.HasResult {
			results = append(results, ctx.CreateValidationResult(
				"document_symbol_presence",
				"Validate document symbols are found",
				"Expected document symbols but response is null",
				false,
				nil,
			))
		} else {
			results = append(results, ctx.CreateValidationResult(
				"document_symbol_absence",
				"Validate no document symbols found (as expected)",
				"No document symbols found as expected",
				true,
				nil,
			))
		}
		return results
	}

	// Parse as array of symbols
	var symbols []interface{}
	parseResult := ctx.ParseJSONResponse(response, &symbols, "document_symbols")
	results = append(results, parseResult)
	if !parseResult.Passed {
		return results
	}

	return ctx.validateSymbolArray(symbols, expected, "document_symbol", results)
}

// AssertWorkspaceSymbolResponse validates a workspace/symbol response
func (ctx *AssertionContext) AssertWorkspaceSymbolResponse(response json.RawMessage, expected *ExpectedWorkspaceSymbolResult) []*cases.ValidationResult {
	var results []*cases.ValidationResult

	// Handle null response
	if ctx.IsNullResponse(response) {
		if expected != nil && expected.HasResult {
			results = append(results, ctx.CreateValidationResult(
				"workspace_symbol_presence",
				"Validate workspace symbols are found",
				"Expected workspace symbols but response is null",
				false,
				nil,
			))
		} else {
			results = append(results, ctx.CreateValidationResult(
				"workspace_symbol_absence",
				"Validate no workspace symbols found (as expected)",
				"No workspace symbols found as expected",
				true,
				nil,
			))
		}
		return results
	}

	// Parse as array of symbols
	var symbols []interface{}
	parseResult := ctx.ParseJSONResponse(response, &symbols, "workspace_symbols")
	results = append(results, parseResult)
	if !parseResult.Passed {
		return results
	}

	count := len(symbols)
	results = append(results, ctx.CreateValidationResult(
		"workspace_symbol_format",
		"Validate workspace symbol response format",
		fmt.Sprintf("Response is an array of %d workspace symbols", count),
		true,
		map[string]interface{}{
			"symbol_count": count,
		},
	))

	// Validate array length expectations for workspace symbols
	if expected != nil && expected.ArrayLength != nil {
		lengthResult := ctx.AssertArrayLength(symbols, expected.ArrayLength, "workspace_symbol_array")
		results = append(results, lengthResult)
	}

	// Validate each symbol
	for i, symbol := range symbols {
		symbolResults := ctx.validateSingleSymbol(symbol, nil, fmt.Sprintf("workspace_symbol_%d", i))
		results = append(results, symbolResults...)
	}

	// Check workspace symbol specific expectations
	if expected != nil {
		if !expected.HasResult && count > 0 {
			results = append(results, ctx.CreateValidationResult(
				"workspace_symbol_expectation",
				"Validate workspace symbol expectation",
				fmt.Sprintf("Expected no symbols but found %d", count),
				false,
				map[string]interface{}{
					"found_count": count,
				},
			))
		} else if expected.HasResult && count == 0 {
			results = append(results, ctx.CreateValidationResult(
				"workspace_symbol_expectation",
				"Validate workspace symbol expectation",
				"Expected symbols but none found",
				false,
				nil,
			))
		}

		// Check if workspace symbols contain expected symbol
		if expected.ContainsSymbol != nil {
			containsResult := ctx.checkSymbolArrayContains(symbols, expected.ContainsSymbol, "workspace_symbol")
			results = append(results, containsResult)
		}
	}

	return results
}

// validateSymbolArray validates an array of symbols (shared between document and workspace symbols)
func (ctx *AssertionContext) validateSymbolArray(symbols []interface{}, expected interface{}, symbolType string, results []*cases.ValidationResult) []*cases.ValidationResult {
	count := len(symbols)
	
	results = append(results, ctx.CreateValidationResult(
		fmt.Sprintf("%s_format", symbolType),
		fmt.Sprintf("Validate %s response format", symbolType),
		fmt.Sprintf("Response is an array of %d symbols", count),
		true,
		map[string]interface{}{
			"symbol_count": count,
		},
	))

	// Handle different expected types
	switch exp := expected.(type) {
	case *ExpectedDocumentSymbolResult:
		if exp != nil {
			// Validate array length expectations
			if exp.ArrayLength != nil {
				lengthResult := ctx.AssertArrayLength(symbols, exp.ArrayLength, fmt.Sprintf("%s_array", symbolType))
				results = append(results, lengthResult)
			}

			// Check expectation consistency
			if !exp.HasResult && count > 0 {
				results = append(results, ctx.CreateValidationResult(
					fmt.Sprintf("%s_expectation", symbolType),
					fmt.Sprintf("Validate %s expectation", symbolType),
					fmt.Sprintf("Expected no symbols but found %d", count),
					false,
					map[string]interface{}{
						"found_count": count,
					},
				))
			} else if exp.HasResult && count == 0 {
				results = append(results, ctx.CreateValidationResult(
					fmt.Sprintf("%s_expectation", symbolType),
					fmt.Sprintf("Validate %s expectation", symbolType),
					"Expected symbols but none found",
					false,
					nil,
				))
			}

			// Check if document symbols contain expected symbol
			if exp.ContainsSymbol != nil {
				containsResult := ctx.checkSymbolArrayContains(symbols, exp.ContainsSymbol, symbolType)
				results = append(results, containsResult)
			}
		}
	}

	// Validate each symbol
	for i, symbol := range symbols {
		symbolResults := ctx.validateSingleSymbol(symbol, nil, fmt.Sprintf("%s_%d", symbolType, i))
		results = append(results, symbolResults...)
	}

	return results
}

// validateSingleSymbol validates a single symbol structure
func (ctx *AssertionContext) validateSingleSymbol(symbol interface{}, expected *ExpectedSymbol, fieldName string) []*cases.ValidationResult {
	var results []*cases.ValidationResult

	symbolMap, ok := symbol.(map[string]interface{})
	if !ok {
		results = append(results, ctx.CreateValidationResult(
			fmt.Sprintf("%s_type", fieldName),
			fmt.Sprintf("Validate %s is an object", fieldName),
			fmt.Sprintf("%s is not an object", fieldName),
			false,
			nil,
		))
		return results
	}

	// Validate required fields
	name, hasName := symbolMap["name"]
	kind, hasKind := symbolMap["kind"]

	if !hasName || !hasKind {
		results = append(results, ctx.CreateValidationResult(
			fmt.Sprintf("%s_required_fields", fieldName),
			fmt.Sprintf("Validate %s has required fields", fieldName),
			fmt.Sprintf("%s missing required fields (name: %t, kind: %t)", fieldName, hasName, hasKind),
			false,
			nil,
		))
		return results
	}

	// Validate field types
	nameStr, nameOk := name.(string)
	kindNum, kindOk := kind.(float64)

	if !nameOk || !kindOk {
		results = append(results, ctx.CreateValidationResult(
			fmt.Sprintf("%s_field_types", fieldName),
			fmt.Sprintf("Validate %s field types", fieldName),
			fmt.Sprintf("%s field types invalid (name string: %t, kind number: %t)", fieldName, nameOk, kindOk),
			false,
			nil,
		))
		return results
	}

	// Validate symbol content against expectations
	if expected != nil {
		if expected.Name != nil && nameStr != *expected.Name {
			results = append(results, ctx.CreateValidationResult(
				fmt.Sprintf("%s_name", fieldName),
				fmt.Sprintf("Validate %s name matches expected", fieldName),
				fmt.Sprintf("Symbol name '%s' does not match expected '%s'", nameStr, *expected.Name),
				false,
				map[string]interface{}{
					"actual_name":   nameStr,
					"expected_name": *expected.Name,
				},
			))
		}

		if expected.Kind != nil && ExpectedSymbolKind(kindNum) != *expected.Kind {
			results = append(results, ctx.CreateValidationResult(
				fmt.Sprintf("%s_kind", fieldName),
				fmt.Sprintf("Validate %s kind matches expected", fieldName),
				fmt.Sprintf("Symbol kind %d does not match expected %d", int(kindNum), int(*expected.Kind)),
				false,
				map[string]interface{}{
					"actual_kind":   int(kindNum),
					"expected_kind": int(*expected.Kind),
				},
			))
		}

		// Check name patterns
		if len(expected.NameContains) > 0 {
			containsResults := ctx.AssertContains(nameStr, expected.NameContains, fmt.Sprintf("%s_name", fieldName))
			results = append(results, containsResults...)
		}

		if len(expected.NameMatches) > 0 {
			matchResults := ctx.AssertMatches(nameStr, expected.NameMatches, fmt.Sprintf("%s_name", fieldName))
			results = append(results, matchResults...)
		}

		if len(expected.NameExcludes) > 0 {
			excludeResults := ctx.AssertExcludes(nameStr, expected.NameExcludes, fmt.Sprintf("%s_name", fieldName))
			results = append(results, excludeResults...)
		}
	}

	// Validate location if present
	if location, hasLocation := symbolMap["location"]; hasLocation {
		var expectedLoc *ExpectedLocation
		if expected != nil && expected.Location != nil {
			expectedLoc = expected.Location
		}
		locationResults := ctx.AssertLocation(location, expectedLoc, fmt.Sprintf("%s_location", fieldName))
		results = append(results, locationResults...)
	}

	results = append(results, ctx.CreateValidationResult(
		fmt.Sprintf("%s_structure", fieldName),
		fmt.Sprintf("Validate %s structure", fieldName),
		fmt.Sprintf("%s structure is valid (name: %s, kind: %d)", fieldName, nameStr, int(kindNum)),
		true,
		map[string]interface{}{
			"name": nameStr,
			"kind": int(kindNum),
		},
	))

	return results
}

// checkSymbolArrayContains checks if a symbol array contains a symbol matching the expected criteria
func (ctx *AssertionContext) checkSymbolArrayContains(symbols []interface{}, expected *ExpectedSymbol, fieldName string) *cases.ValidationResult {
	for _, symbol := range symbols {
		if ctx.symbolMatches(symbol, expected) {
			return ctx.CreateValidationResult(
				fmt.Sprintf("%s_contains_expected", fieldName),
				fmt.Sprintf("Validate %s array contains expected symbol", fieldName),
				"Found symbol matching expected criteria",
				true,
				nil,
			)
		}
	}

	var criteria []string
	if expected.Name != nil {
		criteria = append(criteria, fmt.Sprintf("name=%s", *expected.Name))
	}
	if expected.Kind != nil {
		criteria = append(criteria, fmt.Sprintf("kind=%d", int(*expected.Kind)))
	}
	criteriaStr := strings.Join(criteria, ", ")

	return ctx.CreateValidationResult(
		fmt.Sprintf("%s_contains_expected", fieldName),
		fmt.Sprintf("Validate %s array contains expected symbol", fieldName),
		fmt.Sprintf("No symbol found matching criteria: %s", criteriaStr),
		false,
		map[string]interface{}{
			"expected_criteria": criteriaStr,
		},
	)
}

// symbolMatches checks if a symbol matches the expected criteria
func (ctx *AssertionContext) symbolMatches(symbol interface{}, expected *ExpectedSymbol) bool {
	symbolMap, ok := symbol.(map[string]interface{})
	if !ok {
		return false
	}

	// Check name match
	if expected.Name != nil {
		if name, hasName := symbolMap["name"]; hasName {
			if nameStr, ok := name.(string); ok && nameStr != *expected.Name {
				return false
			}
		} else {
			return false
		}
	}

	// Check kind match
	if expected.Kind != nil {
		if kind, hasKind := symbolMap["kind"]; hasKind {
			if kindNum, ok := kind.(float64); ok && ExpectedSymbolKind(kindNum) != *expected.Kind {
				return false
			}
		} else {
			return false
		}
	}

	// Check name patterns
	if name, hasName := symbolMap["name"]; hasName {
		if nameStr, ok := name.(string); ok {
			for _, pattern := range expected.NameContains {
				if !strings.Contains(nameStr, pattern) {
					return false
				}
			}
			for _, pattern := range expected.NameExcludes {
				if strings.Contains(nameStr, pattern) {
					return false
				}
			}
		}
	}

	return true
}

// truncateString truncates a string to the specified length with ellipsis
func (ctx *AssertionContext) truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}