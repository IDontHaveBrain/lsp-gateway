package documents

import (
    "context"
    "path/filepath"
    "strings"
    "time"

    "go.lsp.dev/protocol"
    "lsp-gateway/src/internal/common"
    "lsp-gateway/src/internal/constants"
    "lsp-gateway/src/internal/registry"
    "lsp-gateway/src/internal/types"
    "lsp-gateway/src/utils"
)

// DocumentManager interface for document-related operations
type DocumentManager interface {
	DetectLanguage(uri string) string
	ExtractURI(params interface{}) (string, error)
	EnsureOpen(client types.LSPClient, uri string, params interface{}) error
}

// LSPDocumentManager implements document management functionality
type LSPDocumentManager struct {
	// Minimal state - could be extended in the future if needed
}

// NewLSPDocumentManager creates a new document manager
func NewLSPDocumentManager() *LSPDocumentManager {
	return &LSPDocumentManager{}
}

// DetectLanguage detects the programming language from a file URI
func (dm *LSPDocumentManager) DetectLanguage(uri string) string {
    path := utils.URIToFilePath(uri)
    ext := strings.ToLower(filepath.Ext(path))
    if lang, ok := registry.GetLanguageByExtension(ext); ok {
        return lang.Name
    }
    return ""
}

// ExtractURI extracts the file URI from request parameters
func (dm *LSPDocumentManager) ExtractURI(params interface{}) (string, error) {
	if params == nil {
		return "", common.NoParametersError()
	}

	// Handle typed protocol structs first (most efficient for tests)
	switch p := params.(type) {
	case *protocol.DefinitionParams:
		return string(p.TextDocument.URI), nil
	case protocol.DefinitionParams:
		return string(p.TextDocument.URI), nil
	case *protocol.ReferenceParams:
		return string(p.TextDocument.URI), nil
	case protocol.ReferenceParams:
		return string(p.TextDocument.URI), nil
	case *protocol.HoverParams:
		return string(p.TextDocument.URI), nil
	case protocol.HoverParams:
		return string(p.TextDocument.URI), nil
	case *protocol.DocumentSymbolParams:
		return string(p.TextDocument.URI), nil
	case protocol.DocumentSymbolParams:
		return string(p.TextDocument.URI), nil
	case *protocol.CompletionParams:
		return string(p.TextDocument.URI), nil
	case protocol.CompletionParams:
		return string(p.TextDocument.URI), nil
	case *protocol.WorkspaceSymbolParams:
		return "", nil // Workspace symbols don't have a specific URI
	case protocol.WorkspaceSymbolParams:
		return "", nil // Workspace symbols don't have a specific URI
	}

	// Handle untyped map parameters (from HTTP gateway)
	paramsMap, err := common.ValidateParamMap(params)
	if err != nil {
		return "", common.WrapProcessingError("failed to validate params", err)
	}

	// Try textDocument.uri first
	if textDoc, ok := paramsMap["textDocument"].(map[string]interface{}); ok {
		if uri, ok := textDoc["uri"].(string); ok {
			return uri, nil
		}
	}

	// Try direct uri parameter
	if uri, ok := paramsMap["uri"].(string); ok {
		return uri, nil
	}

	return "", common.ParameterValidationError("no URI found in parameters")
}

// EnsureOpen sends a textDocument/didOpen notification if needed
func (dm *LSPDocumentManager) EnsureOpen(client types.LSPClient, uri string, params interface{}) error {

	// Read actual file content for proper LSP functionality
	var fileContent string
	language := dm.DetectLanguage(uri)

    // Extract file path from URI
    if strings.HasPrefix(uri, "file://") {
        filePath := utils.URIToFilePath(uri)
        if data, err := common.SafeReadFile(filePath); err == nil {
            fileContent = string(data)
        } else {
            common.LSPLogger.Error("Failed to read file content for %s: %v", uri, err)
            fileContent = ""
        }
	} else {
		common.LSPLogger.Warn("URI does not start with file://: %s", uri)
	}

	didOpenParams := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        uri,
			"languageId": language,
			"version":    1,
			"text":       fileContent, // Use actual file content
		},
	}

	// Send notification (ignore errors as this is optional)
	err := client.SendNotification(context.Background(), types.MethodTextDocumentDidOpen, didOpenParams)
	if err != nil {
		common.LSPLogger.Error("Failed to send didOpen notification for %s: %v", uri, err)
		return common.WrapProcessingError("failed to send didOpen notification", err)
	}

    // Apply language-aware document analysis delay
    time.Sleep(constants.GetDocumentAnalysisDelay(language))

	return nil
}
