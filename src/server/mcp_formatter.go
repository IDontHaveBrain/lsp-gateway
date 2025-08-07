package server

import (
	"encoding/json"
	"fmt"
	"strings"

	"lsp-gateway/src/internal/models/lsp"
	"lsp-gateway/src/internal/types"
)

// formatStructuredResult formats any result as indented JSON for MCP responses
func formatStructuredResult(result interface{}, toolName string) string {
	if result == nil {
		return fmt.Sprintf("Tool '%s' returned no results", toolName)
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error formatting result for tool '%s': %v", toolName, err)
	}

	return string(data)
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

		// Add enhanced metadata for SCIP features (placeholder)
		result["occurrenceMetadata"] = map[string]interface{}{
			"filePath":      sym.FilePath,
			"lineNumber":    sym.LineNumber,
			"endLine":       sym.EndLine,
			"hasRoleFilter": hasRoleFilter,
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

		// Enhanced SCIP occurrence metadata
		occurrenceMetadata := map[string]interface{}{
			"symbolId": ref.SymbolID,
		}

		// Role-based metadata
		var roles []string
		if ref.IsDefinition {
			roles = append(roles, "definition")
		}
		if ref.IsReadAccess {
			roles = append(roles, "read")
		}
		if ref.IsWriteAccess {
			roles = append(roles, "write")
		}
		if ref.IsImport {
			roles = append(roles, "import")
		}
		if ref.IsGenerated {
			roles = append(roles, "generated")
		}
		if ref.IsTest {
			roles = append(roles, "test")
		}

		if len(roles) > 0 {
			occurrenceMetadata["roles"] = roles
		}

		// Role indicator flags for easier filtering
		occurrenceMetadata["isDefinition"] = ref.IsDefinition
		occurrenceMetadata["isReadAccess"] = ref.IsReadAccess
		occurrenceMetadata["isWriteAccess"] = ref.IsWriteAccess
		occurrenceMetadata["isImport"] = ref.IsImport
		occurrenceMetadata["isGenerated"] = ref.IsGenerated
		occurrenceMetadata["isTest"] = ref.IsTest

		// Add relationships if available
		if len(ref.Relationships) > 0 {
			occurrenceMetadata["relationships"] = ref.Relationships
		}

		// Add documentation if available
		if len(ref.Documentation) > 0 {
			occurrenceMetadata["documentation"] = ref.Documentation
		}

		// Add range information if available
		if ref.Range != nil {
			occurrenceMetadata["range"] = map[string]interface{}{
				"start": map[string]interface{}{
					"line":      ref.Range.Start.Line,
					"character": ref.Range.Start.Character,
				},
				"end": map[string]interface{}{
					"line":      ref.Range.End.Line,
					"character": ref.Range.End.Character,
				},
			}
		}

		result["occurrenceMetadata"] = occurrenceMetadata
		formatted[i] = result
	}

	return formatted
}
