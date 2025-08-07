package server

import (
	"encoding/json"
	"fmt"
	"strings"

	"lsp-gateway/src/internal/models/lsp"
	"lsp-gateway/src/internal/types"
)

// formatStructuredResult formats any result as indented JSON for MCP responses
// For findSymbols tool, it uses raw code blocks to reduce token usage
func formatStructuredResult(result interface{}, toolName string) string {
	if result == nil {
		return fmt.Sprintf("Tool '%s' returned no results", toolName)
	}

	// Special handling for findSymbols to use raw code blocks
	if toolName == "findSymbols" {
		return formatSymbolsWithRawCode(result)
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error formatting result for tool '%s': %v", toolName, err)
	}

	return string(data)
}

// formatSymbolsWithRawCode formats symbol results with raw code blocks to minimize tokens
func formatSymbolsWithRawCode(result interface{}) string {
	// First marshal to JSON to get the structure
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Sprintf("Error formatting symbols: %v", err)
	}

	// Parse back to extract code fields
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return string(data) // Fallback to regular JSON
	}

	// Build custom output with raw code blocks
	var output strings.Builder
	output.WriteString("{\n")

	// Process each field
	for key, value := range parsed {
		if key == "symbols" {
			output.WriteString(`  "symbols": [`)
			if symbols, ok := value.([]interface{}); ok {
				for i, sym := range symbols {
					if i > 0 {
						output.WriteString(",")
					}
					output.WriteString("\n    {")
					if symMap, ok := sym.(map[string]interface{}); ok {
						first := true
						for k, v := range symMap {
							if !first {
								output.WriteString(",")
							}
							first = false
							output.WriteString("\n      \"")
							output.WriteString(k)
							output.WriteString("\": ")

							if k == "code" && v != nil {
								// Use raw code block with custom delimiters
								if codeStr, ok := v.(string); ok && codeStr != "" {
									output.WriteString("<<<CODE_BLOCK>>>\n")
									output.WriteString(codeStr)
									if !strings.HasSuffix(codeStr, "\n") {
										output.WriteString("\n")
									}
									output.WriteString("<<<CODE_BLOCK>>>")
								} else {
									output.WriteString("null")
								}
							} else {
								// Regular JSON encoding for other fields
								valBytes, _ := json.Marshal(v)
								output.Write(valBytes)
							}
						}
					}
					output.WriteString("\n    }")
				}
			}
			output.WriteString("\n  ],\n")
		} else {
			output.WriteString("  \"")
			output.WriteString(key)
			output.WriteString("\": ")
			valBytes, _ := json.Marshal(value)
			output.Write(valBytes)
			output.WriteString(",\n")
		}
	}

	// Remove trailing comma and close
	resultStr := output.String()
	if strings.HasSuffix(resultStr, ",\n") {
		resultStr = resultStr[:len(resultStr)-2] + "\n"
	}
	return resultStr + "}"
}

// getSymbolKindName returns the string representation of a symbol kind
func getSymbolKindName(kind lsp.SymbolKind) string {
	switch kind {
	case lsp.File:
		return "File"
	case lsp.Module:
		return "Module"
	case lsp.Namespace:
		return "Namespace"
	case lsp.Package:
		return "Package"
	case lsp.Class:
		return "Class"
	case lsp.Method:
		return "Method"
	case lsp.Property:
		return "Property"
	case lsp.Field:
		return "Field"
	case lsp.Constructor:
		return "Constructor"
	case lsp.Enum:
		return "Enum"
	case lsp.Interface:
		return "Interface"
	case lsp.Function:
		return "Function"
	case lsp.Variable:
		return "Variable"
	case lsp.Constant:
		return "Constant"
	case lsp.String:
		return "String"
	case lsp.Number:
		return "Number"
	case lsp.Boolean:
		return "Boolean"
	case lsp.Array:
		return "Array"
	case lsp.Object:
		return "Object"
	case lsp.Key:
		return "Key"
	case lsp.Null:
		return "Null"
	case lsp.EnumMember:
		return "EnumMember"
	case lsp.Struct:
		return "Struct"
	case lsp.Event:
		return "Event"
	case lsp.Operator:
		return "Operator"
	case lsp.TypeParameter:
		return "TypeParameter"
	default:
		return "Unknown"
	}
}

// parseSymbolRole converts a string role name to SymbolRole bitflag
func parseSymbolRole(roleStr string) types.SymbolRole {
	switch strings.ToLower(roleStr) {
	case "definition":
		return types.SymbolRoleDefinition
	case "reference":
		return types.SymbolRoleReadAccess // Default references are read access
	case "import":
		return types.SymbolRoleImport
	case "write":
		return types.SymbolRoleWriteAccess
	case "read":
		return types.SymbolRoleReadAccess
	case "generated":
		return types.SymbolRoleGenerated
	case "test":
		return types.SymbolRoleTest
	case "forward":
		return types.SymbolRoleForwardDefinition
	default:
		return 0 // Unknown role
	}
}

// formatRoleFilter formats a role filter for metadata display
func formatRoleFilter(roleFilter *types.SymbolRole) []string {
	if roleFilter == nil {
		return []string{}
	}

	var roles []string
	if roleFilter.HasRole(types.SymbolRoleDefinition) {
		roles = append(roles, "definition")
	}
	if roleFilter.HasRole(types.SymbolRoleImport) {
		roles = append(roles, "import")
	}
	if roleFilter.HasRole(types.SymbolRoleWriteAccess) {
		roles = append(roles, "write")
	}
	if roleFilter.HasRole(types.SymbolRoleReadAccess) {
		roles = append(roles, "read")
	}
	if roleFilter.HasRole(types.SymbolRoleGenerated) {
		roles = append(roles, "generated")
	}
	if roleFilter.HasRole(types.SymbolRoleTest) {
		roles = append(roles, "test")
	}
	if roleFilter.HasRole(types.SymbolRoleForwardDefinition) {
		roles = append(roles, "forward")
	}
	return roles
}

// formatEnhancedSymbolsForMCP formats symbols with enhanced metadata including occurrence roles
func formatEnhancedSymbolsForMCP(symbols []types.EnhancedSymbolInfo, hasRoleFilter bool) []map[string]interface{} {
	formatted := make([]map[string]interface{}, len(symbols))

	for i, sym := range symbols {
		// Build enhanced response with SCIP-style occurrence metadata
		result := map[string]interface{}{
			"name":     sym.Name,
			"kind":     getSymbolKindName(sym.Kind),
			"location": fmt.Sprintf("%s:%d-%d", sym.FilePath, sym.LineNumber, sym.EndLine),
		}

		// Add optional fields
		if sym.Container != "" {
			result["container"] = sym.Container
		}
		if sym.Signature != "" {
			result["signature"] = sym.Signature
		}
		if sym.Documentation != "" {
			result["documentation"] = sym.Documentation
		}
		if sym.Code != "" {
			result["code"] = sym.Code
		}

		formatted[i] = result
	}

	return formatted
}

// formatEnhancedReferencesForMCP formats references with enhanced SCIP occurrence metadata
func formatEnhancedReferencesForMCP(references []ReferenceInfo) []map[string]interface{} {
	formatted := make([]map[string]interface{}, len(references))

	for i, ref := range references {
		result := map[string]interface{}{
			"location": fmt.Sprintf("%s:%d:%d", ref.FilePath, ref.LineNumber, ref.Column),
		}

		// Include basic text fields if available
		if ref.Text != "" {
			result["text"] = ref.Text
		}
		if ref.Code != "" {
			result["code"] = ref.Code
		}
		if ref.Context != "" {
			result["context"] = ref.Context
		}

		formatted[i] = result
	}

	return formatted
}

// formatSimpleReferencesForMCP formats references with only location information
func formatSimpleReferencesForMCP(references []ReferenceInfo) []map[string]interface{} {
	formatted := make([]map[string]interface{}, len(references))

	for i, ref := range references {
		formatted[i] = map[string]interface{}{
			"location": fmt.Sprintf("%s:%d", ref.FilePath, ref.LineNumber),
		}
	}

	return formatted
}
