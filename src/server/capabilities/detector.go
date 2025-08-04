package capabilities

import (
	"encoding/json"
	"fmt"
	"strings"

	"lsp-gateway/src/internal/common"
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

	common.LSPLogger.Debug("Server %s capabilities: workspaceSymbol=%v, definition=%v, references=%v, hover=%v, documentSymbol=%v, completion=%v",
		serverCommand,
		initResponse.Capabilities.WorkspaceSymbolProvider,
		initResponse.Capabilities.DefinitionProvider,
		initResponse.Capabilities.ReferencesProvider,
		initResponse.Capabilities.HoverProvider,
		initResponse.Capabilities.DocumentSymbolProvider,
		initResponse.Capabilities.CompletionProvider)

	// Special handling for jdtls - it supports all textDocument methods but may not report them correctly
	if strings.Contains(serverCommand, "jdtls") {
		common.LSPLogger.Info("Detected jdtls - enabling all textDocument capabilities")
		initResponse.Capabilities.DefinitionProvider = true
		initResponse.Capabilities.ReferencesProvider = true
		initResponse.Capabilities.HoverProvider = true
		initResponse.Capabilities.DocumentSymbolProvider = true
		if initResponse.Capabilities.CompletionProvider == nil {
			initResponse.Capabilities.CompletionProvider = true
		}
	}

	return initResponse.Capabilities, nil
}

func (d *LSPCapabilityDetector) SupportsMethod(caps ServerCapabilities, method string) bool {
	switch method {
	case "initialize", "shutdown", "exit":
		return true
	case "workspace/symbol":
		return d.isCapabilitySupported(caps.WorkspaceSymbolProvider)
	case "textDocument/definition":
		return d.isCapabilitySupported(caps.DefinitionProvider)
	case "textDocument/references":
		return d.isCapabilitySupported(caps.ReferencesProvider)
	case "textDocument/hover":
		return d.isCapabilitySupported(caps.HoverProvider)
	case "textDocument/documentSymbol":
		return d.isCapabilitySupported(caps.DocumentSymbolProvider)
	case "textDocument/completion":
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
