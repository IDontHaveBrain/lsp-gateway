package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"lsp-gateway/mcp"
)

type SimpleRequestRouter struct {
	resolver      SubProjectResolver
	clientManager SubProjectClientManager
	uriExtractor  URIExtractor
	logger        *mcp.StructuredLogger
	mu            sync.RWMutex
}

func NewSimpleRequestRouter(resolver SubProjectResolver, clientManager SubProjectClientManager, logger *mcp.StructuredLogger) *SimpleRequestRouter {
	return &SimpleRequestRouter{
		resolver:      resolver,
		clientManager: clientManager,
		uriExtractor:  NewURIExtractor(logger),
		logger:        logger,
	}
}

func (r *SimpleRequestRouter) RouteRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 1. Extract file URI from request
	fileURI, err := r.uriExtractor.ExtractFileURI(method, params)
	if err != nil {
		if r.logger != nil {
			r.logger.WithError(err).WithField("method", method).Debug("URI extraction failed")
		}
		return nil, fmt.Errorf("URI extraction failed: %w", err)
	}

	// 2. Handle workspace-wide methods (no specific file)
	if fileURI == "" && method == "workspace/symbol" {
		return r.routeWorkspaceMethod(ctx, method, params)
	}

	if fileURI == "" {
		return nil, fmt.Errorf("no file URI found for method %s", method)
	}

	// 3. Resolve file URI to sub-project
	subProject, err := r.resolver.ResolveSubProject(fileURI)
	if err != nil {
		if r.logger != nil {
			r.logger.WithError(err).WithField("file_uri", fileURI).Debug("Project resolution failed")
		}
		return nil, fmt.Errorf("project resolution failed for %s: %w", fileURI, err)
	}

	// 4. Determine language from file extension
	language, err := r.determineLanguage(fileURI)
	if err != nil {
		return nil, fmt.Errorf("language determination failed: %w", err)
	}

	// 5. Get LSP client for sub-project + language
	client, err := r.clientManager.GetClient(subProject.ID, language)
	if err != nil {
		if r.logger != nil {
			r.logger.WithError(err).WithFields(map[string]interface{}{
				"sub_project_id": subProject.ID,
				"language":       language,
			}).Debug("Client not found")
		}
		return nil, fmt.Errorf("no client found for project %s language %s: %w", subProject.ID, language, err)
	}

	// 6. Execute LSP request
	result, err := client.SendRequest(ctx, method, params)
	if err != nil {
		return nil, fmt.Errorf("LSP request failed: %w", err)
	}

	// 7. Convert result to json.RawMessage for gateway compatibility
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("result marshaling failed: %w", err)
	}

	if r.logger != nil {
		r.logger.WithFields(map[string]interface{}{
			"method":         method,
			"file_uri":       fileURI,
			"sub_project_id": subProject.ID,
			"language":       language,
		}).Debug("Request routed successfully")
	}

	return json.RawMessage(resultBytes), nil
}

func (r *SimpleRequestRouter) routeWorkspaceMethod(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	// For workspace/symbol, use any available client from the first available project
	projects := r.resolver.GetAllSubProjects()
	if len(projects) == 0 {
		return nil, fmt.Errorf("no sub-projects available for workspace method")
	}

	// Try first project's primary language client
	project := projects[0]
	client, err := r.clientManager.GetClient(project.ID, project.PrimaryLang)
	if err != nil {
		return nil, fmt.Errorf("no client available for workspace method: %w", err)
	}

	result, err := client.SendRequest(ctx, method, params)
	if err != nil {
		return nil, fmt.Errorf("workspace LSP request failed: %w", err)
	}

	resultBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("workspace result marshaling failed: %w", err)
	}

	return json.RawMessage(resultBytes), nil
}

func (r *SimpleRequestRouter) determineLanguage(fileURI string) (string, error) {
	ext := r.getFileExtension(fileURI)
	switch ext {
	case "go":
		return "go", nil
	case "py":
		return "python", nil
	case "js", "jsx":
		return "javascript", nil
	case "ts", "tsx":
		return "typescript", nil
	case "java":
		return "java", nil
	case "c", "h":
		return "c", nil
	case "cpp", "cxx", "cc", "hpp":
		return "cpp", nil
	case "rs":
		return "rust", nil
	case "php":
		return "php", nil
	case "rb":
		return "ruby", nil
	case "cs":
		return "csharp", nil
	default:
		return "", fmt.Errorf("unsupported file extension: %s", ext)
	}
}

func (r *SimpleRequestRouter) getFileExtension(fileURI string) string {
	// Remove file:// prefix if present
	path := strings.TrimPrefix(fileURI, "file://")
	ext := filepath.Ext(path)
	if ext != "" && len(ext) > 1 {
		return strings.ToLower(ext[1:]) // Remove leading dot
	}
	return ""
}

func (r *SimpleRequestRouter) GetSupportedMethods() []string {
	return []string{
		"textDocument/definition",
		"textDocument/references",
		"textDocument/hover",
		"textDocument/documentSymbol",
		"textDocument/completion",
		"workspace/symbol",
	}
}