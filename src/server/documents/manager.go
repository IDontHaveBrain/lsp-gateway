package documents

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.lsp.dev/protocol"
	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/types"
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
	// Remove file:// prefix and get extension
	path := strings.TrimPrefix(uri, "file://")
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".js", ".jsx":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".java":
		return "java"
	default:
		return ""
	}
}

// ExtractURI extracts the file URI from request parameters
func (dm *LSPDocumentManager) ExtractURI(params interface{}) (string, error) {
	if params == nil {
		return "", fmt.Errorf("no parameters provided")
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

	return "", fmt.Errorf("no URI found in parameters")
}

// EnsureOpen sends a textDocument/didOpen notification if needed
func (dm *LSPDocumentManager) EnsureOpen(client types.LSPClient, uri string, params interface{}) error {

	// Read actual file content for proper LSP functionality
	var fileContent string

	// Extract file path from URI
	if strings.HasPrefix(uri, "file://") {
		filePath := strings.TrimPrefix(uri, "file://")
		if content, err := os.ReadFile(filePath); err == nil {
			fileContent = string(content)
		} else {
			// If we can't read the file, log but continue with empty content
			common.LSPLogger.Error("Failed to read file content for %s: %v", uri, err)
			fileContent = ""
		}
	} else {
		common.LSPLogger.Warn("URI does not start with file://: %s", uri)
	}

	didOpenParams := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        uri,
			"languageId": dm.DetectLanguage(uri),
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

	// Give LSP server more time to process the didOpen notification and analyze file
	time.Sleep(500 * time.Millisecond)

	return nil
}
