package e2e_test

import (
	"fmt"
	"strings"
	"testing"

	"lsp-gateway/tests/e2e/fixtures"
	"lsp-gateway/tests/e2e/testutils"

)

// LSPValidationContext provides context for LSP response validation
type LSPValidationContext struct {
	T             *testing.T
	Scenario      fixtures.PythonPatternScenario
	WorkspaceDir  string
	ExpectedFiles []string
}

// LSPValidationMetrics tracks validation metrics
type LSPValidationMetrics struct {
	DefinitionAccuracy   float64
	ReferenceCompleteness float64
	HoverContentQuality  float64
	SymbolHierarchyDepth int
	CompletionRelevance  float64
	ErrorHandlingRobustness float64
}

// ValidateDefinitionResponse validates textDocument/definition responses
func ValidateDefinitionResponse(ctx *LSPValidationContext, locations []testutils.Location, testPos fixtures.PythonTestPosition) *LSPValidationResult {
	result := &LSPValidationResult{
		Method:    "textDocument/definition",
		Symbol:    testPos.Symbol,
		IsValid:   true,
		Details:   make(map[string]interface{}),
		Warnings:  []string{},
		Errors:    []string{},
	}

	if len(locations) == 0 {
		result.Warnings = append(result.Warnings, "No definition locations found")
		result.Details["location_count"] = 0
		return result
	}

	result.Details["location_count"] = len(locations)
	
	for i, location := range locations {
		// Validate URI format
		if !strings.HasPrefix(location.URI, "file://") {
			result.Errors = append(result.Errors, fmt.Sprintf("Location %d: Invalid URI format: %s", i, location.URI))
			result.IsValid = false
			continue
		}

		// Validate range
		if location.Range.Start.Line < 0 || location.Range.Start.Character < 0 {
			result.Errors = append(result.Errors, fmt.Sprintf("Location %d: Invalid range start position", i))
			result.IsValid = false
		}

		if location.Range.End.Line < location.Range.Start.Line ||
			(location.Range.End.Line == location.Range.Start.Line && location.Range.End.Character < location.Range.Start.Character) {
			result.Errors = append(result.Errors, fmt.Sprintf("Location %d: Invalid range end position", i))
			result.IsValid = false
		}

		// Pattern-specific validation
		result.Details[fmt.Sprintf("location_%d_uri", i)] = location.URI
		result.Details[fmt.Sprintf("location_%d_line", i)] = location.Range.Start.Line
		
		// Validate file relevance for specific patterns
		if testPos.Symbol == "Builder" || testPos.Symbol == "Director" {
			if !strings.Contains(location.URI, "builder.py") && !strings.Contains(location.URI, "creational") {
				result.Warnings = append(result.Warnings, fmt.Sprintf("Builder pattern symbol found in unexpected file: %s", location.URI))
			}
		}
	}

	return result
}

// ValidateReferencesResponse validates textDocument/references responses
func ValidateReferencesResponse(ctx *LSPValidationContext, references []testutils.Location, testPos fixtures.PythonTestPosition) *LSPValidationResult {
	result := &LSPValidationResult{
		Method:    "textDocument/references",
		Symbol:    testPos.Symbol,
		IsValid:   true,
		Details:   make(map[string]interface{}),
		Warnings:  []string{},
		Errors:    []string{},
	}

	result.Details["reference_count"] = len(references)

	if len(references) == 0 {
		result.Warnings = append(result.Warnings, "No references found")
		return result
	}

	// Track unique files and lines
	uniqueFiles := make(map[string]bool)
	uniqueLines := make(map[string]bool)

	for i, ref := range references {
		// Validate URI format
		if !strings.HasPrefix(ref.URI, "file://") {
			result.Errors = append(result.Errors, fmt.Sprintf("Reference %d: Invalid URI format: %s", i, ref.URI))
			result.IsValid = false
			continue
		}

		// Track statistics
		uniqueFiles[ref.URI] = true
		lineKey := fmt.Sprintf("%s:%d", ref.URI, ref.Range.Start.Line)
		uniqueLines[lineKey] = true

		// Validate range
		if ref.Range.Start.Line < 0 || ref.Range.Start.Character < 0 {
			result.Errors = append(result.Errors, fmt.Sprintf("Reference %d: Invalid range position", i))
			result.IsValid = false
		}
	}

	result.Details["unique_files"] = len(uniqueFiles)
	result.Details["unique_locations"] = len(uniqueLines)

	// Pattern-specific validation
	if testPos.Symbol == "Builder" && len(references) > 0 {
		result.Details["builder_pattern_detected"] = true
		if len(references) < 2 {
			result.Warnings = append(result.Warnings, "Builder pattern typically has multiple references")
		}
	}

	return result
}

// ValidateHoverResponse validates textDocument/hover responses
func ValidateHoverResponse(ctx *LSPValidationContext, hoverResult *testutils.HoverResult, testPos fixtures.PythonTestPosition) *LSPValidationResult {
	result := &LSPValidationResult{
		Method:    "textDocument/hover",
		Symbol:    testPos.Symbol,
		IsValid:   true,
		Details:   make(map[string]interface{}),
		Warnings:  []string{},
		Errors:    []string{},
	}

	if hoverResult == nil {
		result.Details["has_content"] = false
		result.Warnings = append(result.Warnings, "No hover information available")
		return result
	}

	result.Details["has_content"] = true

	// Validate contents
	if hoverResult.Contents == nil {
		result.Errors = append(result.Errors, "Hover contents should not be nil")
		result.IsValid = false
		return result
	}

	// Convert contents to string for analysis
	contentsStr := fmt.Sprintf("%v", hoverResult.Contents)
	result.Details["content_length"] = len(contentsStr)
	result.Details["content_preview"] = truncateString(contentsStr, 100)

	// Validate range if present
	if hoverResult.Range != nil {
		if hoverResult.Range.Start.Line < 0 || hoverResult.Range.Start.Character < 0 {
			result.Errors = append(result.Errors, "Hover range has invalid start position")
			result.IsValid = false
		}
		result.Details["has_range"] = true
	} else {
		result.Details["has_range"] = false
	}

	// Content quality analysis
	if len(contentsStr) > 10 {
		result.Details["content_quality"] = "good"
	} else if len(contentsStr) > 0 {
		result.Details["content_quality"] = "minimal"
	} else {
		result.Details["content_quality"] = "empty"
		result.Warnings = append(result.Warnings, "Hover content is empty")
	}

	return result
}

// ValidateDocumentSymbolResponse validates textDocument/documentSymbol responses
func ValidateDocumentSymbolResponse(ctx *LSPValidationContext, symbols []testutils.DocumentSymbol) *LSPValidationResult {
	result := &LSPValidationResult{
		Method:    "textDocument/documentSymbol",
		Symbol:    "document_symbols",
		IsValid:   true,
		Details:   make(map[string]interface{}),
		Warnings:  []string{},
		Errors:    []string{},
	}

	result.Details["symbol_count"] = len(symbols)

	if len(symbols) == 0 {
		result.Warnings = append(result.Warnings, "No document symbols found")
		return result
	}

	// Analyze symbol hierarchy
	symbolsByKind := make(map[int]int)
	maxDepth := 0
	totalSymbols := 0

	for _, symbol := range symbols {
		if err := validateDocumentSymbol(symbol, 0, &maxDepth, &totalSymbols, symbolsByKind); err != nil {
			result.Errors = append(result.Errors, err.Error())
			result.IsValid = false
		}
	}

	result.Details["max_hierarchy_depth"] = maxDepth
	result.Details["total_symbols"] = totalSymbols
	result.Details["symbols_by_kind"] = symbolsByKind

	// Pattern-specific validation
	expectedSymbols := ctx.Scenario.ExpectedSymbols
	if len(expectedSymbols) > 0 {
		foundSymbols := make(map[string]bool)
		for _, symbol := range symbols {
			collectSymbolNames(symbol, foundSymbols)
		}

		matchedCount := 0
		for _, expected := range expectedSymbols {
			if foundSymbols[expected.Name] {
				matchedCount++
			}
		}

		result.Details["expected_symbols"] = len(expectedSymbols)
		result.Details["matched_symbols"] = matchedCount
		result.Details["match_ratio"] = float64(matchedCount) / float64(len(expectedSymbols))

		if matchedCount == 0 {
			result.Warnings = append(result.Warnings, "No expected symbols found in document")
		} else if matchedCount < len(expectedSymbols) {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Only %d/%d expected symbols found", matchedCount, len(expectedSymbols)))
		}
	}

	return result
}

// ValidateWorkspaceSymbolResponse validates workspace/symbol responses
func ValidateWorkspaceSymbolResponse(ctx *LSPValidationContext, symbols []testutils.SymbolInformation, query string) *LSPValidationResult {
	result := &LSPValidationResult{
		Method:    "workspace/symbol",
		Symbol:    query,
		IsValid:   true,
		Details:   make(map[string]interface{}),
		Warnings:  []string{},
		Errors:    []string{},
	}

	result.Details["symbol_count"] = len(symbols)
	result.Details["query"] = query

	if len(symbols) == 0 {
		if query != "" {
			result.Warnings = append(result.Warnings, "No workspace symbols found for query")
		}
		return result
	}

	// Analyze symbols
	symbolsByKind := make(map[int]int)
	uniqueFiles := make(map[string]bool)
	relevantSymbols := 0

	for i, symbol := range symbols {
		// Validate symbol structure
		if symbol.Name == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("Symbol %d: Empty name", i))
			result.IsValid = false
			continue
		}

		if symbol.Kind <= 0 {
			result.Errors = append(result.Errors, fmt.Sprintf("Symbol %d: Invalid kind: %d", i, symbol.Kind))
			result.IsValid = false
		}

		if !strings.HasPrefix(symbol.Location.URI, "file://") {
			result.Errors = append(result.Errors, fmt.Sprintf("Symbol %d: Invalid URI format: %s", i, symbol.Location.URI))
			result.IsValid = false
		}

		// Track statistics
		symbolsByKind[symbol.Kind]++
		uniqueFiles[symbol.Location.URI] = true

		// Query relevance check
		if query != "" {
			if strings.Contains(strings.ToLower(symbol.Name), strings.ToLower(query)) ||
				strings.Contains(strings.ToLower(query), strings.ToLower(symbol.Name)) {
				relevantSymbols++
			}
		}
	}

	result.Details["symbols_by_kind"] = symbolsByKind
	result.Details["unique_files"] = len(uniqueFiles)
	result.Details["relevant_symbols"] = relevantSymbols

	if query != "" && relevantSymbols == 0 {
		result.Warnings = append(result.Warnings, "No symbols appear relevant to the query")
	}

	relevanceRatio := float64(relevantSymbols) / float64(len(symbols))
	result.Details["relevance_ratio"] = relevanceRatio

	if relevanceRatio < 0.5 {
		result.Warnings = append(result.Warnings, "Low query relevance ratio")
	}

	return result
}

// ValidateCompletionResponse validates textDocument/completion responses
func ValidateCompletionResponse(ctx *LSPValidationContext, completionList *testutils.CompletionList, testPos fixtures.PythonTestPosition) *LSPValidationResult {
	result := &LSPValidationResult{
		Method:    "textDocument/completion",
		Symbol:    testPos.Symbol,
		IsValid:   true,
		Details:   make(map[string]interface{}),
		Warnings:  []string{},
		Errors:    []string{},
	}

	if completionList == nil {
		result.Errors = append(result.Errors, "Completion list should not be nil")
		result.IsValid = false
		return result
	}

	result.Details["item_count"] = len(completionList.Items)
	result.Details["is_incomplete"] = completionList.IsIncomplete

	if len(completionList.Items) == 0 {
		result.Warnings = append(result.Warnings, "No completion items found")
		return result
	}

	// Analyze completion items
	itemsByKind := make(map[int]int)
	relevantItems := 0

	for i, item := range completionList.Items {
		// Validate item structure
		if item.Label == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("Completion item %d: Empty label", i))
			result.IsValid = false
			continue
		}

		if item.Kind <= 0 {
			result.Errors = append(result.Errors, fmt.Sprintf("Completion item %d: Invalid kind: %d", i, item.Kind))
			result.IsValid = false
		}

		itemsByKind[item.Kind]++

		// Context-specific relevance
		if testPos.Context == "method chaining" || testPos.Context == "builder method" {
			if strings.Contains(item.Label, "set_") || strings.Contains(item.Label, "build") ||
				strings.Contains(item.Label, "add_") || strings.Contains(item.Label, "with_") {
				relevantItems++
			}
		}
		
		if testPos.Context == "class definition" || testPos.Context == "class usage" {
			if item.Kind == 5 { // Class kind
				relevantItems++
			}
		}
	}

	result.Details["items_by_kind"] = itemsByKind
	result.Details["relevant_items"] = relevantItems

	relevanceRatio := float64(relevantItems) / float64(len(completionList.Items))
	result.Details["relevance_ratio"] = relevanceRatio

	if relevantItems == 0 && testPos.Context != "" {
		result.Warnings = append(result.Warnings, "No contextually relevant completion items found")
	}

	return result
}

// LSPValidationResult represents the result of validating an LSP response
type LSPValidationResult struct {
	Method     string                 `json:"method"`
	Symbol     string                 `json:"symbol"`
	IsValid    bool                   `json:"is_valid"`
	Details    map[string]interface{} `json:"details"`
	Warnings   []string               `json:"warnings"`
	Errors     []string               `json:"errors"`
	Timestamp  int64                  `json:"timestamp"`
}

// Helper functions

func validateDocumentSymbol(symbol testutils.DocumentSymbol, depth int, maxDepth *int, totalCount *int, kindCount map[int]int) error {
	*totalCount++
	if depth > *maxDepth {
		*maxDepth = depth
	}
	
	kindCount[symbol.Kind]++

	// Validate basic structure
	if symbol.Name == "" {
		return fmt.Errorf("symbol has empty name")
	}
	
	if symbol.Kind <= 0 {
		return fmt.Errorf("symbol %s has invalid kind: %d", symbol.Name, symbol.Kind)
	}

	// Validate ranges
	if symbol.Range.Start.Line < 0 || symbol.Range.Start.Character < 0 {
		return fmt.Errorf("symbol %s has invalid range start", symbol.Name)
	}

	if symbol.SelectionRange.Start.Line < 0 || symbol.SelectionRange.Start.Character < 0 {
		return fmt.Errorf("symbol %s has invalid selection range start", symbol.Name)
	}

	// Validate children recursively
	for _, child := range symbol.Children {
		if err := validateDocumentSymbol(child, depth+1, maxDepth, totalCount, kindCount); err != nil {
			return err
		}

		// Validate child is within parent range
		if child.Range.Start.Line < symbol.Range.Start.Line || child.Range.End.Line > symbol.Range.End.Line {
			return fmt.Errorf("child symbol %s is outside parent %s range", child.Name, symbol.Name)
		}
	}

	return nil
}

func collectSymbolNames(symbol testutils.DocumentSymbol, names map[string]bool) {
	names[symbol.Name] = true
	for _, child := range symbol.Children {
		collectSymbolNames(child, names)
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// CalculateLSPValidationMetrics calculates overall validation metrics
func CalculateLSPValidationMetrics(results []*LSPValidationResult) *LSPValidationMetrics {
	if len(results) == 0 {
		return &LSPValidationMetrics{}
	}

	metrics := &LSPValidationMetrics{}
	
	validResults := 0
	totalDefinitionAccuracy := 0.0
	totalReferenceCompleteness := 0.0
	totalHoverQuality := 0.0
	totalCompletionRelevance := 0.0
	maxHierarchyDepth := 0

	for _, result := range results {
		if result.IsValid {
			validResults++
		}

		switch result.Method {
		case "textDocument/definition":
			if count, ok := result.Details["location_count"].(int); ok && count > 0 {
				totalDefinitionAccuracy += 1.0
			}
		case "textDocument/references":
			if count, ok := result.Details["reference_count"].(int); ok && count > 0 {
				totalReferenceCompleteness += 1.0
			}
		case "textDocument/hover":
			if hasContent, ok := result.Details["has_content"].(bool); ok && hasContent {
				totalHoverQuality += 1.0
			}
		case "textDocument/documentSymbol":
			if depth, ok := result.Details["max_hierarchy_depth"].(int); ok && depth > maxHierarchyDepth {
				maxHierarchyDepth = depth
			}
		case "textDocument/completion":
			if ratio, ok := result.Details["relevance_ratio"].(float64); ok {
				totalCompletionRelevance += ratio
			}
		}
	}

	total := float64(len(results))
	metrics.DefinitionAccuracy = totalDefinitionAccuracy / total
	metrics.ReferenceCompleteness = totalReferenceCompleteness / total
	metrics.HoverContentQuality = totalHoverQuality / total
	metrics.CompletionRelevance = totalCompletionRelevance / total
	metrics.SymbolHierarchyDepth = maxHierarchyDepth
	metrics.ErrorHandlingRobustness = float64(validResults) / total

	return metrics
}