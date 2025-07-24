package gateway

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"lsp-gateway/mcp"
)

// ProjectAwareRouter wraps the existing Router and adds workspace-aware routing capabilities
type ProjectAwareRouter struct {
	*Router                // Embedded traditional router for fallback
	workspaceManager *WorkspaceManager
	logger           *mcp.StructuredLogger
	mu               sync.RWMutex
	
	// Cache for workspace-specific server overrides
	workspaceServerOverrides map[string]map[string]string // workspaceID -> language -> serverName
}

// NewProjectAwareRouter creates a new ProjectAwareRouter that wraps the existing Router
func NewProjectAwareRouter(router *Router, workspaceManager *WorkspaceManager, logger *mcp.StructuredLogger) *ProjectAwareRouter {
	return &ProjectAwareRouter{
		Router:                   router,
		workspaceManager:         workspaceManager,
		logger:                   logger,
		workspaceServerOverrides: make(map[string]map[string]string),
	}
}

// RouteRequestWithWorkspace provides workspace-aware routing with fallback to traditional routing
func (par *ProjectAwareRouter) RouteRequestWithWorkspace(uri string) (string, error) {
	if par.logger != nil {
		par.logger.Debugf("Attempting workspace-aware routing for URI: %s", uri)
	}

	// Try workspace-aware routing first
	serverName, err := par.routeWithWorkspaceContext(uri)
	if err == nil {
		if par.logger != nil {
			par.logger.Debugf("Successfully routed using workspace context: %s -> %s", uri, serverName)
		}
		return serverName, nil
	}

	// Log workspace routing failure but continue with traditional routing
	if par.logger != nil {
		par.logger.Debugf("Workspace-aware routing failed for %s, falling back to traditional routing: %s", uri, err.Error())
	}

	// Fallback to traditional Router behavior
	return par.Router.RouteRequest(uri)
}

// routeWithWorkspaceContext performs workspace-aware server selection
func (par *ProjectAwareRouter) routeWithWorkspaceContext(uri string) (string, error) {
	// Get or create workspace context for the file
	workspace, err := par.workspaceManager.GetOrCreateWorkspace(uri)
	if err != nil {
		return "", fmt.Errorf("failed to get workspace context: %w", err)
	}

	// Check for workspace-specific server overrides first
	if serverName := par.getWorkspaceServerOverride(workspace.GetID(), uri); serverName != "" {
		return serverName, nil
	}

	// Determine server based on workspace project type and file context
	serverName, err := par.selectServerForWorkspace(workspace, uri)
	if err != nil {
		return "", fmt.Errorf("failed to select server for workspace: %w", err)
	}

	return serverName, nil
}

// selectServerForWorkspace selects the most appropriate server based on workspace context
func (par *ProjectAwareRouter) selectServerForWorkspace(workspace WorkspaceContext, uri string) (string, error) {
	// Extract file extension for language detection
	filePath := uri
	if strings.HasPrefix(uri, URIPrefixFile) {
		filePath = strings.TrimPrefix(uri, URIPrefixFile)
	}
	
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext != "" {
		ext = strings.TrimPrefix(ext, ".")
	}

	// Priority 1: Try to match based on file extension and workspace languages
	if ext != "" {
		if lang, exists := par.GetLanguageByExtension(ext); exists {
			// Check if this language is supported in the workspace context
			if par.isLanguageSupportedInWorkspace(workspace, lang) {
				if serverName, exists := par.GetServerByLanguage(lang); exists {
					return serverName, nil
				}
			}
		}
	}

	// Priority 2: Select server based on workspace project type
	serverName := par.selectServerByProjectType(workspace)
	if serverName != "" {
		return serverName, nil
	}

	// Priority 3: Select first available server for workspace languages
	for _, lang := range workspace.GetLanguages() {
		if serverName, exists := par.GetServerByLanguage(lang); exists {
			return serverName, nil
		}
	}

	return "", fmt.Errorf("no suitable server found for workspace %s with project type %s and languages %v", 
		workspace.GetID(), workspace.GetProjectType(), workspace.GetLanguages())
}

// selectServerByProjectType selects server based on workspace project type
func (par *ProjectAwareRouter) selectServerByProjectType(workspace WorkspaceContext) string {
	// Map project types to preferred server languages
	projectTypeToLanguage := map[string]string{
		"go":         "go",
		"node":       "typescript", // Prefer TypeScript server for Node.js projects
		"python":     "python",
		"java":       "java",
		"typescript": "typescript",
		"javascript": "javascript",
	}

	if preferredLang, exists := projectTypeToLanguage[workspace.GetProjectType()]; exists {
		if serverName, exists := par.GetServerByLanguage(preferredLang); exists {
			return serverName
		}
	}

	return ""
}

// isLanguageSupportedInWorkspace checks if a language is supported in the workspace context
func (par *ProjectAwareRouter) isLanguageSupportedInWorkspace(workspace WorkspaceContext, language string) bool {
	for _, lang := range workspace.GetLanguages() {
		if lang == language {
			return true
		}
	}

	// Special cases for language compatibility
	switch workspace.GetProjectType() {
	case "node":
		// Node projects support both JavaScript and TypeScript
		return language == "javascript" || language == "typescript"
	case "python":
		// Python projects may have various Python-related files
		return language == "python"
	case "java":
		// Java projects support Java
		return language == "java"
	case "go":
		// Go projects support Go
		return language == "go"
	}

	return false
}

// SetWorkspaceServerOverride sets a specific server override for a workspace and language
func (par *ProjectAwareRouter) SetWorkspaceServerOverride(workspaceID, language, serverName string) {
	par.mu.Lock()
	defer par.mu.Unlock()

	if par.workspaceServerOverrides[workspaceID] == nil {
		par.workspaceServerOverrides[workspaceID] = make(map[string]string)
	}
	par.workspaceServerOverrides[workspaceID][language] = serverName

	if par.logger != nil {
		par.logger.Infof("Set workspace server override: workspace=%s, language=%s, server=%s", workspaceID, language, serverName)
	}
}

// RemoveWorkspaceServerOverride removes a server override for a workspace and language
func (par *ProjectAwareRouter) RemoveWorkspaceServerOverride(workspaceID, language string) {
	par.mu.Lock()
	defer par.mu.Unlock()

	if overrides, exists := par.workspaceServerOverrides[workspaceID]; exists {
		delete(overrides, language)
		if len(overrides) == 0 {
			delete(par.workspaceServerOverrides, workspaceID)
		}

		if par.logger != nil {
			par.logger.Infof("Removed workspace server override: workspace=%s, language=%s", workspaceID, language)
		}
	}
}

// getWorkspaceServerOverride gets server override for workspace and file
func (par *ProjectAwareRouter) getWorkspaceServerOverride(workspaceID, uri string) string {
	par.mu.RLock()
	defer par.mu.RUnlock()

	overrides, exists := par.workspaceServerOverrides[workspaceID]
	if !exists {
		return ""
	}

	// Extract language from URI
	filePath := uri
	if strings.HasPrefix(uri, URIPrefixFile) {
		filePath = strings.TrimPrefix(uri, URIPrefixFile)
	}
	
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext != "" {
		ext = strings.TrimPrefix(ext, ".")
		if lang, exists := par.GetLanguageByExtension(ext); exists {
			if serverName, exists := overrides[lang]; exists {
				return serverName
			}
		}
	}

	return ""
}

// GetWorkspaceContext returns the workspace context for a given URI
func (par *ProjectAwareRouter) GetWorkspaceContext(uri string) (WorkspaceContext, error) {
	return par.workspaceManager.GetOrCreateWorkspace(uri)
}

// GetWorkspaceServerOverrides returns all server overrides for a workspace
func (par *ProjectAwareRouter) GetWorkspaceServerOverrides(workspaceID string) map[string]string {
	par.mu.RLock()
	defer par.mu.RUnlock()

	overrides, exists := par.workspaceServerOverrides[workspaceID]
	if !exists {
		return make(map[string]string)
	}

	// Return a copy to prevent external modification
	result := make(map[string]string)
	for lang, server := range overrides {
		result[lang] = server
	}
	return result
}

// ClearWorkspaceServerOverrides removes all server overrides for a workspace
func (par *ProjectAwareRouter) ClearWorkspaceServerOverrides(workspaceID string) {
	par.mu.Lock()
	defer par.mu.Unlock()

	delete(par.workspaceServerOverrides, workspaceID)

	if par.logger != nil {
		par.logger.Infof("Cleared all workspace server overrides for workspace: %s", workspaceID)
	}
}

// GetSupportedWorkspaceTypes returns all supported workspace/project types
func (par *ProjectAwareRouter) GetSupportedWorkspaceTypes() []string {
	return []string{
		"go",
		"node",
		"python", 
		"java",
		"typescript",
		"javascript",
		"generic",
	}
}

// RouteRequest maintains backward compatibility by delegating to traditional Router
func (par *ProjectAwareRouter) RouteRequest(uri string) (string, error) {
	return par.Router.RouteRequest(uri)
}

// IsWorkspaceAware returns true if workspace-aware routing is available
func (par *ProjectAwareRouter) IsWorkspaceAware() bool {
	return par.workspaceManager != nil
}