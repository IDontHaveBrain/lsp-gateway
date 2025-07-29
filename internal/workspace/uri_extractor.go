package workspace

import (
	"fmt"
	"path/filepath"
	"strings"

	"lsp-gateway/mcp"
)

// URIExtractor interface defines methods for extracting file URIs from LSP requests
type URIExtractor interface {
	ExtractFileURI(method string, params interface{}) (string, error)
	ExtractAllFileURIs(method string, params interface{}) ([]string, error)
	ValidateURI(uri string) error
	NormalizeURI(uri string) (string, error)
	IsWorkspaceMethod(method string) bool
	ExtractFileExtension(uri string) string
	ExtensionToLanguage(extension string) string
}

type uriExtractorImpl struct{}

func NewURIExtractor(logger *mcp.StructuredLogger) URIExtractor {
	return &uriExtractorImpl{}
}

// ExtractFileURI extracts file URI from LSP request parameters
func (e *uriExtractorImpl) ExtractFileURI(method string, params interface{}) (string, error) {
	paramsMap, ok := params.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid request parameters")
	}

	// Handle the 6 supported LSP methods
	switch method {
	case "textDocument/definition", "textDocument/references", "textDocument/hover", "textDocument/completion":
		return e.extractFromTextDocument(paramsMap)
	case "textDocument/documentSymbol":
		return e.extractFromTextDocument(paramsMap)
	case "workspace/symbol":
		return "", nil // No specific file URI for workspace symbol
	default:
		return "", fmt.Errorf("unsupported method: %s", method)
	}
}

func (e *uriExtractorImpl) extractFromTextDocument(params map[string]interface{}) (string, error) {
	textDoc, exists := params["textDocument"]
	if !exists {
		return "", fmt.Errorf("textDocument parameter missing")
	}
	
	textDocMap, ok := textDoc.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("textDocument is not an object")
	}
	
	uri, exists := textDocMap["uri"]
	if !exists {
		return "", fmt.Errorf("uri missing in textDocument")
	}
	
	uriStr, ok := uri.(string)
	if !ok {
		return "", fmt.Errorf("uri is not a string")
	}
	
	return e.normalizeURI(uriStr), nil
}

func (e *uriExtractorImpl) normalizeURI(uri string) string {
	// Remove file:// prefix if present
	if strings.HasPrefix(uri, "file://") {
		return strings.TrimPrefix(uri, "file://")
	}
	return uri
}

// Additional methods needed for compatibility with existing codebase

func (e *uriExtractorImpl) ExtractAllFileURIs(method string, params interface{}) ([]string, error) {
	uri, err := e.ExtractFileURI(method, params)
	if err != nil {
		return nil, err
	}
	if uri == "" {
		return []string{}, nil
	}
	return []string{uri}, nil
}

func (e *uriExtractorImpl) ValidateURI(uri string) error {
	if uri == "" {
		return fmt.Errorf("empty URI")
	}
	return nil
}

func (e *uriExtractorImpl) NormalizeURI(uri string) (string, error) {
	return e.normalizeURI(uri), nil
}

func (e *uriExtractorImpl) IsWorkspaceMethod(method string) bool {
	return method == "workspace/symbol"
}

func (e *uriExtractorImpl) ExtractFileExtension(uri string) string {
	normalizedURI := e.normalizeURI(uri)
	ext := filepath.Ext(normalizedURI)
	if ext != "" {
		return strings.ToLower(strings.TrimPrefix(ext, "."))
	}
	return ""
}

func (e *uriExtractorImpl) ExtensionToLanguage(extension string) string {
	// Simple extension to language mapping
	langMap := map[string]string{
		"go":   "go",
		"py":   "python", 
		"js":   "javascript",
		"ts":   "typescript",
		"java": "java",
	}
	ext := strings.ToLower(strings.TrimPrefix(extension, "."))
	if lang, exists := langMap[ext]; exists {
		return lang
	}
	return ""
}