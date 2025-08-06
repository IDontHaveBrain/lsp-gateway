package cache

import (
	"encoding/json"
	"fmt"

	"go.lsp.dev/protocol"
	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/types"
)

// ExtractURIFromParams extracts the URI from LSP method parameters
// This provides centralized URI extraction logic for cache operations
func ExtractURIFromParams(method string, params interface{}) (string, error) {
	if params == nil {
		return "", fmt.Errorf("no parameters provided")
	}

	// Handle typed LSP parameters first (most efficient path)
	switch p := params.(type) {
	case *protocol.DefinitionParams:
		return string(p.TextDocument.URI), nil
	case *protocol.ReferenceParams:
		return string(p.TextDocument.URI), nil
	case *protocol.HoverParams:
		return string(p.TextDocument.URI), nil
	case *protocol.DocumentSymbolParams:
		return string(p.TextDocument.URI), nil
	case *protocol.CompletionParams:
		return string(p.TextDocument.URI), nil
	case *protocol.WorkspaceSymbolParams:
		return "", nil // Workspace symbols don't have a specific URI
	case map[string]interface{}:
		return extractURIFromUntyped(p)
	default:
		return extractURIFromGeneric(params)
	}
}

// extractURIFromUntyped handles untyped parameter maps (from CLI cache indexing)
func extractURIFromUntyped(params map[string]interface{}) (string, error) {
	if params == nil {
		return "", fmt.Errorf("no parameters provided")
	}

	// Try textDocument.uri first (most common case)
	if textDoc, ok := params["textDocument"].(map[string]interface{}); ok {
		if uri, ok := textDoc["uri"].(string); ok && uri != "" {
			return uri, nil
		}
	}

	// Try direct uri parameter (alternative format)
	if uri, ok := params["uri"].(string); ok && uri != "" {
		return uri, nil
	}

	// For workspace/symbol requests, no URI is expected - return empty string
	if query, ok := params["query"]; ok && query != nil {
		return "", nil
	}

	// Handle textDocument/documentSymbol with just textDocument
	if textDoc, exists := params["textDocument"]; exists {
		if textDocMap, ok := textDoc.(map[string]interface{}); ok {
			if uri, ok := textDocMap["uri"].(string); ok && uri != "" {
				return uri, nil
			}
		}
	}

	// Handle potential position-based requests without textDocument
	if position, exists := params["position"]; exists && position != nil {
		// This might be a malformed request, but we should not fail completely
		common.LSPLogger.Debug("ExtractURI: Found position parameter without textDocument, assuming no URI needed")
		return "", nil
	}

	// Additional debug information for troubleshooting
	var keys []string
	for k := range params {
		keys = append(keys, k)
	}

	return "", fmt.Errorf("no URI found in untyped parameters, available keys: %v", keys)
}

// extractURIFromGeneric handles other parameter types using JSON marshaling/unmarshaling
func extractURIFromGeneric(params interface{}) (string, error) {
	// Try JSON marshaling/unmarshaling for other types
	if data, err := json.Marshal(params); err == nil {
		var parsed struct {
			TextDocument struct {
				URI string `json:"uri"`
			} `json:"textDocument"`
		}
		if err := json.Unmarshal(data, &parsed); err == nil {
			if parsed.TextDocument.URI != "" {
				return parsed.TextDocument.URI, nil
			}
		}
	}

	return "", fmt.Errorf("unable to extract URI from parameters type: %T", params)
}

// IsSupported6LSPMethod checks if a method is one of the 6 core supported LSP methods
func IsSupported6LSPMethod(method string) bool {
	switch method {
	case types.MethodTextDocumentDefinition,
		types.MethodTextDocumentReferences,
		types.MethodTextDocumentHover,
		types.MethodTextDocumentDocumentSymbol,
		types.MethodWorkspaceSymbol,
		types.MethodTextDocumentCompletion:
		return true
	default:
		return false
	}
}

// IsCacheableMethod determines if a given LSP method should be cached
func IsCacheableMethod(method string) bool {
	return IsSupported6LSPMethod(method)
}

// GetMethodPriority returns priority level for different LSP methods (for caching/indexing)
func GetMethodPriority(method string) int {
	priorities := map[string]int{
		types.MethodTextDocumentDefinition:     10, // High priority - user navigation
		types.MethodTextDocumentReferences:     9,  // High priority - user navigation
		types.MethodTextDocumentHover:          8,  // High priority - immediate feedback
		types.MethodTextDocumentCompletion:     7,  // Medium-high priority - typing assistance
		types.MethodTextDocumentDocumentSymbol: 5,  // Medium priority - outline view
		types.MethodWorkspaceSymbol:            3,  // Lower priority - search functionality
	}

	if priority, exists := priorities[method]; exists {
		return priority
	}
	return 1 // Default low priority
}

// LSPMethodInfo provides metadata about LSP methods
type LSPMethodInfo struct {
	Method        string
	RequiresURI   bool
	Priority      int
	Cacheable     bool
	ParameterType string
}

// GetLSPMethodInfo returns comprehensive information about a given LSP method
func GetLSPMethodInfo(method string) *LSPMethodInfo {
	info := &LSPMethodInfo{
		Method:      method,
		Cacheable:   IsSupported6LSPMethod(method),
		Priority:    GetMethodPriority(method),
		RequiresURI: method != types.MethodWorkspaceSymbol,
	}

	switch method {
	case types.MethodTextDocumentDefinition:
		info.ParameterType = "*protocol.DefinitionParams"
	case types.MethodTextDocumentReferences:
		info.ParameterType = "*protocol.ReferenceParams"
	case types.MethodTextDocumentHover:
		info.ParameterType = "*protocol.HoverParams"
	case types.MethodTextDocumentDocumentSymbol:
		info.ParameterType = "*protocol.DocumentSymbolParams"
	case types.MethodWorkspaceSymbol:
		info.ParameterType = "*protocol.WorkspaceSymbolParams"
	case types.MethodTextDocumentCompletion:
		info.ParameterType = "*protocol.CompletionParams"
	default:
		info.ParameterType = "interface{}"
	}

	return info
}

// GetAllSupportedMethods returns a list of all 6 supported LSP methods
func GetAllSupportedMethods() []string {
	return []string{
		types.MethodTextDocumentDefinition,
		types.MethodTextDocumentReferences,
		types.MethodTextDocumentHover,
		types.MethodTextDocumentDocumentSymbol,
		types.MethodWorkspaceSymbol,
		types.MethodTextDocumentCompletion,
	}
}
