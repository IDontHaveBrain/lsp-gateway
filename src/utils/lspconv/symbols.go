package lspconv

import (
	"encoding/json"
	"strings"

	"lsp-gateway/src/internal/models/lsp"
	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/server/scip"
	"lsp-gateway/src/utils/jsonutil"
)

// ParseWorkspaceSymbols normalizes workspace/symbol results into []types.SymbolInformation
func ParseWorkspaceSymbols(result interface{}) []types.SymbolInformation {
	if result == nil {
		return nil
	}
	switch v := result.(type) {
	case []types.SymbolInformation:
		return v
	case []interface{}:
		out := make([]types.SymbolInformation, 0, len(v))
		for _, it := range v {
			if si, err := jsonutil.Convert[types.SymbolInformation](it); err == nil && si.Name != "" {
				out = append(out, si)
			}
		}
		return out
	case json.RawMessage:
		var arr []types.SymbolInformation
		if err := json.Unmarshal(v, &arr); err == nil {
			return arr
		}
		var raw []interface{}
		if err := json.Unmarshal(v, &raw); err == nil {
			out := make([]types.SymbolInformation, 0, len(raw))
			for _, it := range raw {
				if si, err := jsonutil.Convert[types.SymbolInformation](it); err == nil && si.Name != "" {
					out = append(out, si)
				}
			}
			return out
		}
		return nil
	default:
		return nil
	}
}

// ParseDocumentSymbols converts documentSymbol responses to []lsp.DocumentSymbol (pointers)
func ParseDocumentSymbols(result interface{}) []*lsp.DocumentSymbol {
	switch v := result.(type) {
	case []lsp.DocumentSymbol:
		out := make([]*lsp.DocumentSymbol, 0, len(v))
		for i := range v {
			out = append(out, &v[i])
		}
		return out
	case []*lsp.DocumentSymbol:
		return v
	case []interface{}:
		out := make([]*lsp.DocumentSymbol, 0, len(v))
		for _, it := range v {
			if ds, err := jsonutil.Convert[lsp.DocumentSymbol](it); err == nil && ds.Name != "" {
				// capture new var to take address safely
				dsc := ds
				out = append(out, &dsc)
			}
		}
		return out
	case json.RawMessage:
		var arr []lsp.DocumentSymbol
		if err := json.Unmarshal(v, &arr); err == nil {
			out := make([]*lsp.DocumentSymbol, 0, len(arr))
			for i := range arr {
				out = append(out, &arr[i])
			}
			return out
		}
		var raw []interface{}
		if err := json.Unmarshal(v, &raw); err == nil {
			out := make([]*lsp.DocumentSymbol, 0, len(raw))
			for _, it := range raw {
				if ds, err := jsonutil.Convert[lsp.DocumentSymbol](it); err == nil && ds.Name != "" {
					dsc := ds
					out = append(out, &dsc)
				}
			}
			return out
		}
		return nil
	default:
		return nil
	}
}

// ParseDocumentSymbolsToSymbolInformation converts documentSymbol responses to []types.SymbolInformation
// using the provided defaultURI to populate locations.
func ParseDocumentSymbolsToSymbolInformation(result interface{}, defaultURI string) []types.SymbolInformation {
	// First try to get DocumentSymbols, then map to SymbolInformation
	docSyms := ParseDocumentSymbols(result)
	if len(docSyms) > 0 {
		out := make([]types.SymbolInformation, 0, len(docSyms))
		for _, ds := range docSyms {
			if ds == nil {
				continue
			}
			si := types.SymbolInformation{
				Name: ds.Name,
				Kind: ds.Kind,
				Location: types.Location{
					URI:   defaultURI,
					Range: ds.Range,
				},
			}
			// preserve selection range via pointer
			sel := ds.SelectionRange
			si.SelectionRange = &sel
			out = append(out, si)
		}
		return out
	}

	// Otherwise, attempt to parse directly as SymbolInformation array
	switch v := result.(type) {
	case []types.SymbolInformation:
		return v
	case []interface{}:
		out := make([]types.SymbolInformation, 0, len(v))
		for _, it := range v {
			if si, err := jsonutil.Convert[types.SymbolInformation](it); err == nil && si.Name != "" {
				out = append(out, si)
			}
		}
		return out
	case json.RawMessage:
		var arr []types.SymbolInformation
		if err := json.Unmarshal(v, &arr); err == nil {
			return arr
		}
		var raw []interface{}
		if err := json.Unmarshal(v, &raw); err == nil {
			out := make([]types.SymbolInformation, 0, len(raw))
			for _, it := range raw {
				if si, err := jsonutil.Convert[types.SymbolInformation](it); err == nil && si.Name != "" {
					out = append(out, si)
				}
			}
			return out
		}
	}
	return nil
}

// SymbolKindStyle defines the output format for symbol kind strings
type SymbolKindStyle string

const (
	// StyleLowercase converts to lowercase strings (e.g., "class", "method")
	StyleLowercase SymbolKindStyle = "lower"
	// StyleTitleCase converts to title case strings (e.g., "Class", "Method")
	StyleTitleCase SymbolKindStyle = "title"
	// StyleUppercase converts to uppercase strings (e.g., "CLASS", "METHOD")
	StyleUppercase SymbolKindStyle = "upper"
)

// LSPSymbolKindToString converts types.SymbolKind to string with specified style
func LSPSymbolKindToString(kind types.SymbolKind, style SymbolKindStyle) string {
	var base string
	switch kind {
	case types.File:
		base = "File"
	case types.Module:
		base = "Module"
	case types.Namespace:
		base = "Namespace"
	case types.Package:
		base = "Package"
	case types.Class:
		base = "Class"
	case types.Method:
		base = "Method"
	case types.Property:
		base = "Property"
	case types.Field:
		base = "Field"
	case types.Constructor:
		base = "Constructor"
	case types.Enum:
		base = "Enum"
	case types.Interface:
		base = "Interface"
	case types.Function:
		base = "Function"
	case types.Variable:
		base = "Variable"
	case types.Constant:
		base = "Constant"
	case types.String:
		base = "String"
	case types.Number:
		base = "Number"
	case types.Boolean:
		base = "Boolean"
	case types.Array:
		base = "Array"
	case types.Object:
		base = "Object"
	case types.Key:
		base = "Key"
	case types.Null:
		base = "Null"
	case types.EnumMember:
		base = "EnumMember"
	case types.Struct:
		base = "Struct"
	case types.Event:
		base = "Event"
	case types.Operator:
		base = "Operator"
	case types.TypeParameter:
		base = "TypeParameter"
	default:
		base = "Unknown"
	}

	return applyStyle(base, style)
}

// SCIPSymbolKindToString converts scip.SCIPSymbolKind to string with specified style
func SCIPSymbolKindToString(kind scip.SCIPSymbolKind, style SymbolKindStyle) string {
	var base string
	switch kind {
	case scip.SCIPSymbolKindClass:
		base = "Class"
	case scip.SCIPSymbolKindInterface:
		base = "Interface"
	case scip.SCIPSymbolKindFunction:
		base = "Function"
	case scip.SCIPSymbolKindMethod:
		base = "Method"
	case scip.SCIPSymbolKindVariable:
		base = "Variable"
	case scip.SCIPSymbolKindField:
		base = "Field"
	case scip.SCIPSymbolKindProperty:
		base = "Property"
	case scip.SCIPSymbolKindEnum:
		base = "Enum"
	case scip.SCIPSymbolKindStruct:
		base = "Struct"
	case scip.SCIPSymbolKindModule:
		base = "Module"
	case scip.SCIPSymbolKindNamespace:
		base = "Namespace"
	case scip.SCIPSymbolKindPackage:
		base = "Package"
	case scip.SCIPSymbolKindFile:
		base = "File"
	case scip.SCIPSymbolKindConstructor:
		base = "Constructor"
	case scip.SCIPSymbolKindConstant:
		base = "Constant"
	case scip.SCIPSymbolKindString:
		base = "String"
	case scip.SCIPSymbolKindNumber:
		base = "Number"
	case scip.SCIPSymbolKindBoolean:
		base = "Boolean"
	case scip.SCIPSymbolKindArray:
		base = "Array"
	case scip.SCIPSymbolKindObject:
		base = "Object"
	case scip.SCIPSymbolKindKey:
		base = "Key"
	case scip.SCIPSymbolKindNull:
		base = "Null"
	case scip.SCIPSymbolKindEnumMember:
		base = "EnumMember"
	case scip.SCIPSymbolKindEvent:
		base = "Event"
	case scip.SCIPSymbolKindOperator:
		base = "Operator"
	case scip.SCIPSymbolKindTypeParameter:
		base = "TypeParameter"
	default:
		base = "Symbol"
	}

	return applyStyle(base, style)
}

// applyStyle applies the specified style to the base string
func applyStyle(base string, style SymbolKindStyle) string {
	switch style {
	case StyleLowercase:
		return strings.ToLower(base)
	case StyleTitleCase:
		return base
	case StyleUppercase:
		return strings.ToUpper(base)
	default:
		return strings.ToLower(base) // Default to lowercase
	}
}
