package documents

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lsp-gateway/src/internal/common"
)

// LSPClient interface for LSP communication (minimal interface needed for document management)
type LSPClient interface {
	SendNotification(ctx context.Context, method string, params interface{}) error
}

// DocumentManager interface for document-related operations
type DocumentManager interface {
	DetectLanguage(uri string) string
	ExtractURI(params interface{}) (string, error)
	EnsureOpen(client LSPClient, uri string, params interface{}) error
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

	paramsMap, ok := params.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("params is not a map")
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
func (dm *LSPDocumentManager) EnsureOpen(client LSPClient, uri string, params interface{}) error {
	common.LSPLogger.Info("Ensuring document is open: %s", uri)

	// Read actual file content for proper LSP functionality
	var fileContent string

	// Extract file path from URI
	if strings.HasPrefix(uri, "file://") {
		filePath := strings.TrimPrefix(uri, "file://")
		if content, err := os.ReadFile(filePath); err == nil {
			fileContent = string(content)
			common.LSPLogger.Info("Successfully read file content for %s: %d bytes", uri, len(fileContent))
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
	common.LSPLogger.Info("Sending textDocument/didOpen notification for %s", uri)
	err := client.SendNotification(context.Background(), "textDocument/didOpen", didOpenParams)
	if err != nil {
		common.LSPLogger.Error("Failed to send didOpen notification for %s: %v", uri, err)
		return fmt.Errorf("failed to send didOpen notification: %w", err)
	}

	common.LSPLogger.Info("Successfully sent didOpen notification for %s", uri)
	// Give LSP server more time to process the didOpen notification and analyze file
	time.Sleep(500 * time.Millisecond)

	return nil
}
