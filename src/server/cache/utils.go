package cache

import (
	"fmt"

	"go.lsp.dev/protocol"
	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/utils/jsonutil"
)

// ExtractURIFromParams extracts the URI from LSP method parameters
// This provides centralized URI extraction logic for cache operations
func ExtractURIFromParams(method string, params interface{}) (string, error) {
	if params == nil {
		return "", common.NoParametersError()
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
		return "", common.NoParametersError()
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

	return "", common.ParameterValidationError(fmt.Sprintf("no URI found in untyped parameters, available keys: %v", keys))
}

// extractURIFromGeneric handles other parameter types using JSON marshaling/unmarshaling
func extractURIFromGeneric(params interface{}) (string, error) {
	type td struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
	}
	v, err := jsonutil.Convert[td](params)
	if err == nil && v.TextDocument.URI != "" {
		return v.TextDocument.URI, nil
	}
	return "", common.ParameterValidationError(fmt.Sprintf("unable to extract URI from parameters type: %T", params))
}
