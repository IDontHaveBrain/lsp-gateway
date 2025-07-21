package validators

import (
	"encoding/json"
	"fmt"
	"strings"

	"lsp-gateway/internal/testing/lsp/cases"
	"lsp-gateway/internal/testing/lsp/config"
)

// DocumentSymbolValidator validates textDocument/documentSymbol responses
type DocumentSymbolValidator struct {
	*BaseValidator
}

// WorkspaceSymbolValidator validates workspace/symbol responses
type WorkspaceSymbolValidator struct {
	*BaseValidator
}

// DocumentSymbol represents a document symbol (hierarchical)
type DocumentSymbol struct {
	Name           string            `json:"name"`
	Detail         string            `json:"detail,omitempty"`
	Kind           int               `json:"kind"`
	Deprecated     bool              `json:"deprecated,omitempty"`
	Range          LSPRange          `json:"range"`
	SelectionRange LSPRange          `json:"selectionRange"`
	Children       []DocumentSymbol  `json:"children,omitempty"`
}

// SymbolInformation represents a symbol information (flat)
type SymbolInformation struct {
	Name          string       `json:"name"`
	Kind          int          `json:"kind"`
	Deprecated    bool         `json:"deprecated,omitempty"`
	Location      SymbolLocation `json:"location"`
	ContainerName string       `json:"containerName,omitempty"`
}

// SymbolLocation represents a symbol location
type SymbolLocation struct {
	URI   string   `json:"uri"`
	Range LSPRange `json:"range"`
}

// WorkspaceSymbol represents a workspace symbol
type WorkspaceSymbol struct {
	Name          string          `json:"name"`
	Kind          int             `json:"kind"`
	Deprecated    bool            `json:"deprecated,omitempty"`
	Location      *SymbolLocation `json:"location,omitempty"`
	ContainerName string          `json:"containerName,omitempty"`
}

// Symbol kinds as defined in LSP specification
const (
	SymbolKindFile          = 1
	SymbolKindModule        = 2
	SymbolKindNamespace     = 3
	SymbolKindPackage       = 4
	SymbolKindClass         = 5
	SymbolKindMethod        = 6
	SymbolKindProperty      = 7
	SymbolKindField         = 8
	SymbolKindConstructor   = 9
	SymbolKindEnum          = 10
	SymbolKindInterface     = 11
	SymbolKindFunction      = 12
	SymbolKindVariable      = 13
	SymbolKindConstant      = 14
	SymbolKindString        = 15
	SymbolKindNumber        = 16
	SymbolKindBoolean       = 17
	SymbolKindArray         = 18
	SymbolKindObject        = 19
	SymbolKindKey           = 20
	SymbolKindNull          = 21
	SymbolKindEnumMember    = 22
	SymbolKindStruct        = 23
	SymbolKindEvent         = 24
	SymbolKindOperator      = 25
	SymbolKindTypeParameter = 26
)

// NewDocumentSymbolValidator creates a new document symbol validator
func NewDocumentSymbolValidator(config *config.ValidationConfig) *DocumentSymbolValidator {
	return &DocumentSymbolValidator{
		BaseValidator: NewBaseValidator(config),
	}
}

// NewWorkspaceSymbolValidator creates a new workspace symbol validator
func NewWorkspaceSymbolValidator(config *config.ValidationConfig) *WorkspaceSymbolValidator {
	return &WorkspaceSymbolValidator{
		BaseValidator: NewBaseValidator(config),
	}
}

// ValidateResponse validates a textDocument/documentSymbol response
func (v *DocumentSymbolValidator) ValidateResponse(testCase *cases.TestCase, response json.RawMessage) []*cases.ValidationResult {
	var results []*cases.ValidationResult
	
	// Check if response is null or empty
	if string(response) == "null" || string(response) == "[]" {
		if testCase.Expected != nil && testCase.Expected.Symbols != nil && testCase.Expected.Symbols.MinCount > 0 {
			results = append(results, &cases.ValidationResult{
				Name:        "document_symbols_presence",
				Description: "Validate document symbols are found",
				Passed:      false,
				Message:     "Expected document symbols but response is null or empty",
			})
		} else {
			results = append(results, &cases.ValidationResult{
				Name:        "document_symbols_absence",
				Description: "Validate no document symbols found (as expected)",
				Passed:      true,
				Message:     "No document symbols found as expected",
			})
		}
		return results
	}
	
	// Try to parse as hierarchical DocumentSymbol array first
	var docSymbols []DocumentSymbol
	if err := json.Unmarshal(response, &docSymbols); err == nil {
		return v.validateDocumentSymbols(docSymbols, testCase, results)
	}
	
	// Try to parse as flat SymbolInformation array
	var symbolInfos []SymbolInformation
	if err := json.Unmarshal(response, &symbolInfos); err != nil {
		results = append(results, &cases.ValidationResult{
			Name:        "document_symbols_format",
			Description: "Validate document symbols response format",
			Passed:      false,
			Message:     fmt.Sprintf("Response is neither DocumentSymbol[] nor SymbolInformation[]: %v", err),
		})
		return results
	}
	
	return v.validateSymbolInformations(symbolInfos, testCase, results)
}

// ValidateResponse validates a workspace/symbol response
func (v *WorkspaceSymbolValidator) ValidateResponse(testCase *cases.TestCase, response json.RawMessage) []*cases.ValidationResult {
	var results []*cases.ValidationResult
	
	// Check if response is null or empty
	if string(response) == "null" || string(response) == "[]" {
		if testCase.Expected != nil && testCase.Expected.Symbols != nil && testCase.Expected.Symbols.MinCount > 0 {
			results = append(results, &cases.ValidationResult{
				Name:        "workspace_symbols_presence",
				Description: "Validate workspace symbols are found",
				Passed:      false,
				Message:     "Expected workspace symbols but response is null or empty",
			})
		} else {
			results = append(results, &cases.ValidationResult{
				Name:        "workspace_symbols_absence",
				Description: "Validate no workspace symbols found (as expected)",
				Passed:      true,
				Message:     "No workspace symbols found as expected",
			})
		}
		return results
	}
	
	// Parse as WorkspaceSymbol array
	var workspaceSymbols []WorkspaceSymbol
	if err := json.Unmarshal(response, &workspaceSymbols); err != nil {
		results = append(results, &cases.ValidationResult{
			Name:        "workspace_symbols_format",
			Description: "Validate workspace symbols response format",
			Passed:      false,
			Message:     fmt.Sprintf("Response is not a WorkspaceSymbol[]: %v", err),
		})
		return results
	}
	
	return v.validateWorkspaceSymbols(workspaceSymbols, testCase, results)
}

// validateDocumentSymbols validates hierarchical document symbols
func (v *DocumentSymbolValidator) validateDocumentSymbols(symbols []DocumentSymbol, testCase *cases.TestCase, results []*cases.ValidationResult) []*cases.ValidationResult {
	results = append(results, &cases.ValidationResult{
		Name:        "document_symbols_format",
		Description: "Validate document symbols response format",
		Passed:      true,
		Message:     fmt.Sprintf("Response is a DocumentSymbol array with %d symbols", len(symbols)),
	})
	
	// Count all symbols including children
	totalCount := v.countDocumentSymbols(symbols)
	
	// Validate count expectations
	if testCase.Expected != nil && testCase.Expected.Symbols != nil {
		results = append(results, v.validateSymbolCount(totalCount, testCase.Expected.Symbols, "document")...)
	}
	
	// Validate each symbol
	for i, symbol := range symbols {
		symbolResults := v.validateDocumentSymbol(symbol, fmt.Sprintf("document_symbol_%d", i))
		results = append(results, symbolResults...)
	}
	
	// Validate symbol types if expected
	if testCase.Expected != nil && testCase.Expected.Symbols != nil && len(testCase.Expected.Symbols.Types) > 0 {
		results = append(results, v.validateDocumentSymbolTypes(symbols, testCase.Expected.Symbols.Types)...)
	}
	
	return results
}

// validateSymbolInformations validates flat symbol information
func (v *DocumentSymbolValidator) validateSymbolInformations(symbols []SymbolInformation, testCase *cases.TestCase, results []*cases.ValidationResult) []*cases.ValidationResult {
	results = append(results, &cases.ValidationResult{
		Name:        "document_symbols_format",
		Description: "Validate document symbols response format",
		Passed:      true,
		Message:     fmt.Sprintf("Response is a SymbolInformation array with %d symbols", len(symbols)),
	})
	
	// Validate count expectations
	if testCase.Expected != nil && testCase.Expected.Symbols != nil {
		results = append(results, v.validateSymbolCount(len(symbols), testCase.Expected.Symbols, "document")...)
	}
	
	// Validate each symbol
	for i, symbol := range symbols {
		symbolResults := v.validateSymbolInformation(symbol, fmt.Sprintf("symbol_info_%d", i))
		results = append(results, symbolResults...)
	}
	
	return results
}

// validateWorkspaceSymbols validates workspace symbols
func (v *WorkspaceSymbolValidator) validateWorkspaceSymbols(symbols []WorkspaceSymbol, testCase *cases.TestCase, results []*cases.ValidationResult) []*cases.ValidationResult {
	results = append(results, &cases.ValidationResult{
		Name:        "workspace_symbols_format",
		Description: "Validate workspace symbols response format",
		Passed:      true,
		Message:     fmt.Sprintf("Response is a WorkspaceSymbol array with %d symbols", len(symbols)),
	})
	
	// Validate count expectations
	if testCase.Expected != nil && testCase.Expected.Symbols != nil {
		results = append(results, v.validateSymbolCount(len(symbols), testCase.Expected.Symbols, "workspace")...)
	}
	
	// Validate each symbol
	for i, symbol := range symbols {
		symbolResults := v.validateWorkspaceSymbol(symbol, fmt.Sprintf("workspace_symbol_%d", i))
		results = append(results, symbolResults...)
	}
	
	return results
}

// countDocumentSymbols recursively counts all document symbols including children
func (v *DocumentSymbolValidator) countDocumentSymbols(symbols []DocumentSymbol) int {
	count := len(symbols)
	for _, symbol := range symbols {
		count += v.countDocumentSymbols(symbol.Children)
	}
	return count
}

// validateSymbolCount validates symbol count against expectations
func (v *BaseValidator) validateSymbolCount(count int, expected *config.SymbolsExpected, symbolType string) []*cases.ValidationResult {
	var results []*cases.ValidationResult
	
	// Check minimum count
	if expected.MinCount > 0 {
		if count >= expected.MinCount {
			results = append(results, &cases.ValidationResult{
				Name:        fmt.Sprintf("%s_symbols_min_count", symbolType),
				Description: fmt.Sprintf("Validate at least %d %s symbols found", expected.MinCount, symbolType),
				Passed:      true,
				Message:     fmt.Sprintf("Found %d %s symbols, meets minimum of %d", count, symbolType, expected.MinCount),
			})
		} else {
			results = append(results, &cases.ValidationResult{
				Name:        fmt.Sprintf("%s_symbols_min_count", symbolType),
				Description: fmt.Sprintf("Validate at least %d %s symbols found", expected.MinCount, symbolType),
				Passed:      false,
				Message:     fmt.Sprintf("Found %d %s symbols, expected at least %d", count, symbolType, expected.MinCount),
			})
		}
	}
	
	// Check maximum count
	if expected.MaxCount > 0 {
		if count <= expected.MaxCount {
			results = append(results, &cases.ValidationResult{
				Name:        fmt.Sprintf("%s_symbols_max_count", symbolType),
				Description: fmt.Sprintf("Validate at most %d %s symbols found", expected.MaxCount, symbolType),
				Passed:      true,
				Message:     fmt.Sprintf("Found %d %s symbols, within maximum of %d", count, symbolType, expected.MaxCount),
			})
		} else {
			results = append(results, &cases.ValidationResult{
				Name:        fmt.Sprintf("%s_symbols_max_count", symbolType),
				Description: fmt.Sprintf("Validate at most %d %s symbols found", expected.MaxCount, symbolType),
				Passed:      false,
				Message:     fmt.Sprintf("Found %d %s symbols, exceeds maximum of %d", count, symbolType, expected.MaxCount),
			})
		}
	}
	
	return results
}

// validateDocumentSymbol validates a single document symbol
func (v *DocumentSymbolValidator) validateDocumentSymbol(symbol DocumentSymbol, fieldName string) []*cases.ValidationResult {
	var results []*cases.ValidationResult
	
	// Validate required fields
	if symbol.Name == "" {
		results = append(results, &cases.ValidationResult{
			Name:        fmt.Sprintf("%s_name", fieldName),
			Description: fmt.Sprintf("Validate %s has name", fieldName),
			Passed:      false,
			Message:     "Symbol name is empty",
		})
	} else {
		results = append(results, &cases.ValidationResult{
			Name:        fmt.Sprintf("%s_name", fieldName),
			Description: fmt.Sprintf("Validate %s has name", fieldName),
			Passed:      true,
			Message:     fmt.Sprintf("Symbol name: %s", symbol.Name),
		})
	}
	
	// Validate symbol kind
	if symbol.Kind < 1 || symbol.Kind > 26 {
		results = append(results, &cases.ValidationResult{
			Name:        fmt.Sprintf("%s_kind", fieldName),
			Description: fmt.Sprintf("Validate %s kind is valid", fieldName),
			Passed:      false,
			Message:     fmt.Sprintf("Invalid symbol kind: %d", symbol.Kind),
		})
	} else {
		results = append(results, &cases.ValidationResult{
			Name:        fmt.Sprintf("%s_kind", fieldName),
			Description: fmt.Sprintf("Validate %s kind is valid", fieldName),
			Passed:      true,
			Message:     fmt.Sprintf("Symbol kind: %d (%s)", symbol.Kind, v.getSymbolKindName(symbol.Kind)),
		})
	}
	
	// Validate ranges
	results = append(results, v.validateSymbolRange(symbol.Range, fmt.Sprintf("%s_range", fieldName))...)
	results = append(results, v.validateSymbolRange(symbol.SelectionRange, fmt.Sprintf("%s_selection_range", fieldName))...)
	
	// Validate children if present
	for i, child := range symbol.Children {
		childResults := v.validateDocumentSymbol(child, fmt.Sprintf("%s_child_%d", fieldName, i))
		results = append(results, childResults...)
	}
	
	return results
}

// validateSymbolInformation validates a single symbol information
func (v *DocumentSymbolValidator) validateSymbolInformation(symbol SymbolInformation, fieldName string) []*cases.ValidationResult {
	var results []*cases.ValidationResult
	
	// Validate required fields
	if symbol.Name == "" {
		results = append(results, &cases.ValidationResult{
			Name:        fmt.Sprintf("%s_name", fieldName),
			Description: fmt.Sprintf("Validate %s has name", fieldName),
			Passed:      false,
			Message:     "Symbol name is empty",
		})
	} else {
		results = append(results, &cases.ValidationResult{
			Name:        fmt.Sprintf("%s_name", fieldName),
			Description: fmt.Sprintf("Validate %s has name", fieldName),
			Passed:      true,
			Message:     fmt.Sprintf("Symbol name: %s", symbol.Name),
		})
	}
	
	// Validate symbol kind
	if symbol.Kind < 1 || symbol.Kind > 26 {
		results = append(results, &cases.ValidationResult{
			Name:        fmt.Sprintf("%s_kind", fieldName),
			Description: fmt.Sprintf("Validate %s kind is valid", fieldName),
			Passed:      false,
			Message:     fmt.Sprintf("Invalid symbol kind: %d", symbol.Kind),
		})
	} else {
		results = append(results, &cases.ValidationResult{
			Name:        fmt.Sprintf("%s_kind", fieldName),
			Description: fmt.Sprintf("Validate %s kind is valid", fieldName),
			Passed:      true,
			Message:     fmt.Sprintf("Symbol kind: %d", symbol.Kind),
		})
	}
	
	// Validate location
	results = append(results, v.validateURI(symbol.Location.URI, fmt.Sprintf("%s_location", fieldName)))
	results = append(results, v.validateSymbolRange(symbol.Location.Range, fmt.Sprintf("%s_location_range", fieldName))...)
	
	return results
}

// validateWorkspaceSymbol validates a single workspace symbol
func (v *WorkspaceSymbolValidator) validateWorkspaceSymbol(symbol WorkspaceSymbol, fieldName string) []*cases.ValidationResult {
	var results []*cases.ValidationResult
	
	// Validate required fields
	if symbol.Name == "" {
		results = append(results, &cases.ValidationResult{
			Name:        fmt.Sprintf("%s_name", fieldName),
			Description: fmt.Sprintf("Validate %s has name", fieldName),
			Passed:      false,
			Message:     "Symbol name is empty",
		})
	} else {
		results = append(results, &cases.ValidationResult{
			Name:        fmt.Sprintf("%s_name", fieldName),
			Description: fmt.Sprintf("Validate %s has name", fieldName),
			Passed:      true,
			Message:     fmt.Sprintf("Symbol name: %s", symbol.Name),
		})
	}
	
	// Validate symbol kind
	if symbol.Kind < 1 || symbol.Kind > 26 {
		results = append(results, &cases.ValidationResult{
			Name:        fmt.Sprintf("%s_kind", fieldName),
			Description: fmt.Sprintf("Validate %s kind is valid", fieldName),
			Passed:      false,
			Message:     fmt.Sprintf("Invalid symbol kind: %d", symbol.Kind),
		})
	} else {
		results = append(results, &cases.ValidationResult{
			Name:        fmt.Sprintf("%s_kind", fieldName),
			Description: fmt.Sprintf("Validate %s kind is valid", fieldName),
			Passed:      true,
			Message:     fmt.Sprintf("Symbol kind: %d", symbol.Kind),
		})
	}
	
	// Validate location if present
	if symbol.Location != nil {
		results = append(results, v.validateURI(symbol.Location.URI, fmt.Sprintf("%s_location", fieldName)))
		results = append(results, v.validateSymbolRange(symbol.Location.Range, fmt.Sprintf("%s_location_range", fieldName))...)
	}
	
	return results
}

// validateSymbolRange validates a symbol range
func (v *BaseValidator) validateSymbolRange(lspRange LSPRange, fieldName string) []*cases.ValidationResult {
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
	
	return results
}

// validateDocumentSymbolTypes validates expected symbol types are present
func (v *DocumentSymbolValidator) validateDocumentSymbolTypes(symbols []DocumentSymbol, expectedTypes []string) []*cases.ValidationResult {
	var results []*cases.ValidationResult
	
	foundTypes := make(map[string]bool)
	v.collectSymbolTypes(symbols, foundTypes)
	
	for _, expectedType := range expectedTypes {
		typeFound := foundTypes[strings.ToLower(expectedType)]
		results = append(results, &cases.ValidationResult{
			Name:        fmt.Sprintf("symbol_type_%s", strings.ToLower(expectedType)),
			Description: fmt.Sprintf("Validate symbol type '%s' is present", expectedType),
			Passed:      typeFound,
			Message:     fmt.Sprintf("Symbol type '%s' found: %t", expectedType, typeFound),
		})
	}
	
	return results
}

// collectSymbolTypes recursively collects all symbol types
func (v *DocumentSymbolValidator) collectSymbolTypes(symbols []DocumentSymbol, found map[string]bool) {
	for _, symbol := range symbols {
		typeName := strings.ToLower(v.getSymbolKindName(symbol.Kind))
		found[typeName] = true
		v.collectSymbolTypes(symbol.Children, found)
	}
}

// getSymbolKindName returns the name of a symbol kind
func (v *BaseValidator) getSymbolKindName(kind int) string {
	switch kind {
	case SymbolKindFile:
		return "file"
	case SymbolKindModule:
		return "module"
	case SymbolKindNamespace:
		return "namespace"
	case SymbolKindPackage:
		return "package"
	case SymbolKindClass:
		return "class"
	case SymbolKindMethod:
		return "method"
	case SymbolKindProperty:
		return "property"
	case SymbolKindField:
		return "field"
	case SymbolKindConstructor:
		return "constructor"
	case SymbolKindEnum:
		return "enum"
	case SymbolKindInterface:
		return "interface"
	case SymbolKindFunction:
		return "function"
	case SymbolKindVariable:
		return "variable"
	case SymbolKindConstant:
		return "constant"
	case SymbolKindString:
		return "string"
	case SymbolKindNumber:
		return "number"
	case SymbolKindBoolean:
		return "boolean"
	case SymbolKindArray:
		return "array"
	case SymbolKindObject:
		return "object"
	case SymbolKindKey:
		return "key"
	case SymbolKindNull:
		return "null"
	case SymbolKindEnumMember:
		return "enummember"
	case SymbolKindStruct:
		return "struct"
	case SymbolKindEvent:
		return "event"
	case SymbolKindOperator:
		return "operator"
	case SymbolKindTypeParameter:
		return "typeparameter"
	default:
		return "unknown"
	}
}