package server

import (
	"encoding/json"
	"fmt"
	"strings"

	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/utils"
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
func getSymbolKindName(kind types.SymbolKind) string {
	switch kind {
	case types.File:
		return "File"
	case types.Module:
		return "Module"
	case types.Namespace:
		return "Namespace"
	case types.Package:
		return "Package"
	case types.Class:
		return "Class"
	case types.Method:
		return "Method"
	case types.Property:
		return "Property"
	case types.Field:
		return "Field"
	case types.Constructor:
		return "Constructor"
	case types.Enum:
		return "Enum"
	case types.Interface:
		return "Interface"
	case types.Function:
		return "Function"
	case types.Variable:
		return "Variable"
	case types.Constant:
		return "Constant"
	case types.String:
		return "String"
	case types.Number:
		return "Number"
	case types.Boolean:
		return "Boolean"
	case types.Array:
		return "Array"
	case types.Object:
		return "Object"
	case types.Key:
		return "Key"
	case types.Null:
		return "Null"
	case types.EnumMember:
		return "EnumMember"
	case types.Struct:
		return "Struct"
	case types.Event:
		return "Event"
	case types.Operator:
		return "Operator"
	case types.TypeParameter:
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

// formatEnhancedSymbolsForMCP formats symbols with enhanced metadata including occurrence roles
func formatEnhancedSymbolsForMCP(symbols []types.EnhancedSymbolInfo) []map[string]interface{} {
	formatted := make([]map[string]interface{}, len(symbols))

	for i, sym := range symbols {
		// Build enhanced response with SCIP-style occurrence metadata
		start := sym.LineNumber
		end := sym.EndLine
		if end < start {
			end = start
		}
		filePath := sym.FilePath
		if filePath == "" && sym.Location.URI != "" {
			filePath = utils.URIToFilePath(sym.Location.URI)
		}
		if filePath == "" {
			filePath = "unknown"
		}
		loc := fmt.Sprintf("%s:%d", filePath, start)
		if end > start {
			loc = fmt.Sprintf("%s:%d-%d", filePath, start, end)
		}
		result := map[string]interface{}{
			"name":     sym.Name,
			"location": loc,
		}

		// Add optional fields
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
			"location": fmt.Sprintf("%s:%d", ref.FilePath, ref.LineNumber),
		}

		lineText, _ := extractCodeLines(ref.FilePath, ref.LineNumber, ref.LineNumber)
		if lineText != "" {
			result["text"] = lineText
		} else if ref.Text != "" {
			result["text"] = ref.Text
		}
		if ref.Context != "" {
			result["context"] = ref.Context
		}

		formatted[i] = result
	}

	return formatted
}
