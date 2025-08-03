package e2e_test

import (
	"fmt"
	"strings"

	lsp "lsp-gateway/internal/models/lsp"
)

// LSPValidationResult contains validation results for an LSP method
type LSPValidationResult struct {
	Method   string
	IsValid  bool
	Details  map[string]interface{}
	Warnings []string
	Errors   []string
}

// ValidateDefinitionResponse validates textDocument/definition responses
func ValidateDefinitionResponse(locations []lsp.Location) *LSPValidationResult {
	result := &LSPValidationResult{
		Method:   "textDocument/definition",
		IsValid:  true,
		Details:  map[string]interface{}{"location_count": len(locations)},
		Warnings: []string{},
		Errors:   []string{},
	}

	if len(locations) == 0 {
		result.Warnings = append(result.Warnings, "No definition locations found")
		return result
	}

	for i, location := range locations {
		if !strings.HasPrefix(location.URI, "file://") {
			result.Errors = append(result.Errors, fmt.Sprintf("Location %d: Invalid URI format: %s", i, location.URI))
			result.IsValid = false
		}
		if location.Range.Start.Line < 0 || location.Range.Start.Character < 0 {
			result.Errors = append(result.Errors, fmt.Sprintf("Location %d: Invalid range start position", i))
			result.IsValid = false
		}
	}

	return result
}

// ValidateReferencesResponse validates textDocument/references responses
func ValidateReferencesResponse(references []lsp.Location) *LSPValidationResult {
	result := &LSPValidationResult{
		Method:   "textDocument/references",
		IsValid:  true,
		Details:  map[string]interface{}{"reference_count": len(references)},
		Warnings: []string{},
		Errors:   []string{},
	}

	if len(references) == 0 {
		result.Warnings = append(result.Warnings, "No references found")
		return result
	}

	for i, ref := range references {
		if !strings.HasPrefix(ref.URI, "file://") {
			result.Errors = append(result.Errors, fmt.Sprintf("Reference %d: Invalid URI format: %s", i, ref.URI))
			result.IsValid = false
		}
	}

	return result
}

// ValidateHoverResponse validates textDocument/hover responses
func ValidateHoverResponse(hoverResult *lsp.Hover) *LSPValidationResult {
	result := &LSPValidationResult{
		Method:   "textDocument/hover",
		IsValid:  true,
		Details:  map[string]interface{}{},
		Warnings: []string{},
		Errors:   []string{},
	}

	if hoverResult == nil {
		result.Warnings = append(result.Warnings, "No hover information found")
		return result
	}

	if hoverResult.Contents == nil {
		result.Errors = append(result.Errors, "Hover contents is nil")
		result.IsValid = false
	} else {
		contentStr := fmt.Sprintf("%v", hoverResult.Contents)
		result.Details["content_length"] = len(contentStr)
	}

	return result
}

// ValidateDocumentSymbolResponse validates textDocument/documentSymbol responses
func ValidateDocumentSymbolResponse(symbols []lsp.DocumentSymbol) *LSPValidationResult {
	result := &LSPValidationResult{
		Method:   "textDocument/documentSymbol",
		IsValid:  true,
		Details:  map[string]interface{}{"symbol_count": len(symbols)},
		Warnings: []string{},
		Errors:   []string{},
	}

	if len(symbols) == 0 {
		result.Warnings = append(result.Warnings, "No document symbols found")
		return result
	}

	symbolNames := collectSymbolNames(symbols)
	result.Details["symbol_names"] = symbolNames

	return result
}

// ValidateWorkspaceSymbolResponse validates workspace/symbol responses
func ValidateWorkspaceSymbolResponse(symbols []lsp.SymbolInformation) *LSPValidationResult {
	result := &LSPValidationResult{
		Method:   "workspace/symbol",
		IsValid:  true,
		Details:  map[string]interface{}{"symbol_count": len(symbols)},
		Warnings: []string{},
		Errors:   []string{},
	}

	if len(symbols) == 0 {
		result.Warnings = append(result.Warnings, "No workspace symbols found")
	}

	return result
}

// ValidateCompletionResponse validates textDocument/completion responses
func ValidateCompletionResponse(completionList *lsp.CompletionList) *LSPValidationResult {
	completionCount := 0
	if completionList != nil {
		completionCount = len(completionList.Items)
	}

	result := &LSPValidationResult{
		Method:   "textDocument/completion",
		IsValid:  true,
		Details:  map[string]interface{}{"completion_count": completionCount},
		Warnings: []string{},
		Errors:   []string{},
	}

	if completionCount == 0 {
		result.Warnings = append(result.Warnings, "No completion items found")
	}

	return result
}

// collectSymbolNames extracts symbol names recursively from DocumentSymbol hierarchy
func collectSymbolNames(symbols []lsp.DocumentSymbol) []string {
	var names []string
	for _, symbol := range symbols {
		names = append(names, symbol.Name)
		if len(symbol.Children) > 0 {
			var childSymbols []lsp.DocumentSymbol
			for _, child := range symbol.Children {
				if child != nil {
					childSymbols = append(childSymbols, *child)
				}
			}
			names = append(names, collectSymbolNames(childSymbols)...)
		}
	}
	return names
}
