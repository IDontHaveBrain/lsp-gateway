// Package capabilities detects and represents server capabilities.
package capabilities

import (
	"encoding/json"
	"fmt"
	"strings"

	"lsp-gateway/src/internal/types"
)

type ServerCapabilities struct {
	WorkspaceSymbolProvider interface{} `json:"workspaceSymbolProvider,omitempty"`
	CompletionProvider      interface{} `json:"completionProvider,omitempty"`
	DefinitionProvider      interface{} `json:"definitionProvider,omitempty"`
	ReferencesProvider      interface{} `json:"referencesProvider,omitempty"`
	HoverProvider           interface{} `json:"hoverProvider,omitempty"`
	DocumentSymbolProvider  interface{} `json:"documentSymbolProvider,omitempty"`
	SemanticTokensProvider  interface{} `json:"semanticTokensProvider,omitempty"`
}

type CapabilityDetector interface {
	ParseCapabilities(response json.RawMessage, serverCommand string) (ServerCapabilities, error)
	SupportsMethod(caps ServerCapabilities, method string) bool
}

type LSPCapabilityDetector struct{}

func NewLSPCapabilityDetector() *LSPCapabilityDetector {
	return &LSPCapabilityDetector{}
}

func (d *LSPCapabilityDetector) ParseCapabilities(response json.RawMessage, serverCommand string) (ServerCapabilities, error) {
	var initResponse struct {
		Capabilities ServerCapabilities `json:"capabilities"`
	}

	if err := json.Unmarshal(response, &initResponse); err != nil {
		return ServerCapabilities{}, fmt.Errorf("failed to unmarshal initialize response: %w", err)
	}

	// Special handling for jdtls - it supports all textDocument methods but may not report them correctly
	if strings.Contains(serverCommand, "jdtls") {
		initResponse.Capabilities.DefinitionProvider = true
		initResponse.Capabilities.ReferencesProvider = true
		initResponse.Capabilities.HoverProvider = true
		initResponse.Capabilities.DocumentSymbolProvider = true
		if initResponse.Capabilities.CompletionProvider == nil {
			initResponse.Capabilities.CompletionProvider = true
		}
	}

	// Special handling for OmniSharp (C#) - known to sometimes under-report capabilities
	// Ensure core textDocument features are treated as supported
	if strings.Contains(strings.ToLower(serverCommand), "omnisharp") {
		if initResponse.Capabilities.DefinitionProvider == nil {
			initResponse.Capabilities.DefinitionProvider = true
		}
		if initResponse.Capabilities.ReferencesProvider == nil {
			initResponse.Capabilities.ReferencesProvider = true
		}
		if initResponse.Capabilities.HoverProvider == nil {
			initResponse.Capabilities.HoverProvider = true
		}
		if initResponse.Capabilities.DocumentSymbolProvider == nil {
			initResponse.Capabilities.DocumentSymbolProvider = true
		}
		if initResponse.Capabilities.CompletionProvider == nil {
			initResponse.Capabilities.CompletionProvider = true
		}
	}

	return initResponse.Capabilities, nil
}

func (d *LSPCapabilityDetector) SupportsMethod(caps ServerCapabilities, method string) bool {
	switch method {
	case types.MethodInitialize, types.MethodShutdown, types.MethodExit:
		return true
	case types.MethodWorkspaceSymbol:
		return d.isCapabilitySupported(caps.WorkspaceSymbolProvider)
	case types.MethodTextDocumentDefinition:
		return d.isCapabilitySupported(caps.DefinitionProvider)
	case types.MethodTextDocumentReferences:
		return d.isCapabilitySupported(caps.ReferencesProvider)
	case types.MethodTextDocumentHover:
		return d.isCapabilitySupported(caps.HoverProvider)
	case types.MethodTextDocumentDocumentSymbol:
		return d.isCapabilitySupported(caps.DocumentSymbolProvider)
	case types.MethodTextDocumentCompletion:
		return d.isCapabilitySupported(caps.CompletionProvider)
	default:
		return true
	}
}

func (d *LSPCapabilityDetector) isCapabilitySupported(capability interface{}) bool {
	if capability == nil {
		return false
	}

	if boolVal, ok := capability.(bool); ok {
		return boolVal
	}

	// Check if it's a map/object (some LSP servers report capabilities as objects)
	if mapVal, ok := capability.(map[string]interface{}); ok {
		// If it's an object, assume it means the capability is supported
		return len(mapVal) > 0
	}

	return true
}
