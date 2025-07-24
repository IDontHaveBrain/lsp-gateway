package mcp

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// Helper methods for parsing LSP results

func (h *ToolHandler) parseDefinitionResult(result []byte) interface{} {
	var parsed interface{}
	if err := json.Unmarshal(result, &parsed); err != nil {
		return string(result) // Fallback to raw string
	}
	return parsed
}

func (h *ToolHandler) parseReferencesResult(result []byte) interface{} {
	var parsed interface{}
	if err := json.Unmarshal(result, &parsed); err != nil {
		return string(result) // Fallback to raw string
	}
	return parsed
}

func (h *ToolHandler) parseHoverResult(result []byte) interface{} {
	var parsed interface{}
	if err := json.Unmarshal(result, &parsed); err != nil {
		return string(result) // Fallback to raw string
	}
	return parsed
}

func (h *ToolHandler) parseDocumentSymbolsResult(result []byte) interface{} {
	var parsed interface{}
	if err := json.Unmarshal(result, &parsed); err != nil {
		return string(result) // Fallback to raw string
	}
	return parsed
}

func (h *ToolHandler) parseWorkspaceSymbolsResult(result []byte) interface{} {
	var parsed interface{}
	if err := json.Unmarshal(result, &parsed); err != nil {
		return string(result) // Fallback to raw string
	}
	return parsed
}

// Helper methods for enhancing results with project context

func (h *ToolHandler) enhanceDefinitionWithProjectContext(result *ToolResult, args map[string]interface{}, workspaceCtx *WorkspaceContext) *ToolResult {
	projectCtx := workspaceCtx.GetProjectContext()
	if projectCtx == nil {
		return result
	}

	// Add project context to metadata
	if result.Meta == nil {
		result.Meta = &ResponseMetadata{}
	}
	if result.Meta.RequestInfo == nil {
		result.Meta.RequestInfo = make(map[string]interface{})
	}

	// Add file context
	if uri, ok := args["uri"].(string); ok {
		result.Meta.RequestInfo["file_context"] = map[string]interface{}{
			"in_project":    workspaceCtx.IsFileInProject(uri),
			"language":      workspaceCtx.GetLanguageForFile(uri),
			"relative_path": h.getRelativePathFromContext(uri, workspaceCtx),
		}
	}

	// Add project metadata
	result.Meta.RequestInfo["project_context"] = map[string]interface{}{
		"project_type":      projectCtx.ProjectType,
		"primary_language":  projectCtx.PrimaryLanguage,
		"confidence":        projectCtx.Confidence,
		"workspace_root":    projectCtx.WorkspaceRoot,
	}

	// Enhance content with project context
	if len(result.Content) > 0 && result.Content[0].Data != nil {
		result.Content[0] = h.enhanceDefinitionContent(result.Content[0], workspaceCtx)
	}

	return result
}

func (h *ToolHandler) enhanceReferencesWithProjectContext(result *ToolResult, args map[string]interface{}, workspaceCtx *WorkspaceContext) *ToolResult {
	projectCtx := workspaceCtx.GetProjectContext()
	if projectCtx == nil {
		return result
	}

	// Add project context to metadata
	if result.Meta == nil {
		result.Meta = &ResponseMetadata{}
	}
	if result.Meta.RequestInfo == nil {
		result.Meta.RequestInfo = make(map[string]interface{})
	}

	// Add file and project context
	if uri, ok := args["uri"].(string); ok {
		result.Meta.RequestInfo["file_context"] = map[string]interface{}{
			"in_project":    workspaceCtx.IsFileInProject(uri),
			"language":      workspaceCtx.GetLanguageForFile(uri),
			"relative_path": h.getRelativePathFromContext(uri, workspaceCtx),
		}
	}

	result.Meta.RequestInfo["project_context"] = map[string]interface{}{
		"project_type":     projectCtx.ProjectType,
		"primary_language": projectCtx.PrimaryLanguage,
		"workspace_root":   projectCtx.WorkspaceRoot,
	}

	// Enhance content by filtering and categorizing references
	if len(result.Content) > 0 && result.Content[0].Data != nil {
		result.Content[0] = h.enhanceReferencesContent(result.Content[0], workspaceCtx)
	}

	return result
}

func (h *ToolHandler) enhanceHoverWithProjectContext(result *ToolResult, args map[string]interface{}, workspaceCtx *WorkspaceContext) *ToolResult {
	projectCtx := workspaceCtx.GetProjectContext()
	if projectCtx == nil {
		return result
	}

	// Add project context to metadata
	if result.Meta == nil {
		result.Meta = &ResponseMetadata{}
	}
	if result.Meta.RequestInfo == nil {
		result.Meta.RequestInfo = make(map[string]interface{})
	}

	// Add file context
	if uri, ok := args["uri"].(string); ok {
		result.Meta.RequestInfo["file_context"] = map[string]interface{}{
			"in_project":    workspaceCtx.IsFileInProject(uri),
			"language":      workspaceCtx.GetLanguageForFile(uri),
			"relative_path": h.getRelativePathFromContext(uri, workspaceCtx),
		}
	}

	result.Meta.RequestInfo["project_context"] = map[string]interface{}{
		"project_type":     projectCtx.ProjectType,
		"primary_language": projectCtx.PrimaryLanguage,
	}

	// Enhance hover content with project-specific documentation
	if len(result.Content) > 0 {
		result.Content[0] = h.enhanceHoverContent(result.Content[0], workspaceCtx)
	}

	return result
}

func (h *ToolHandler) enhanceDocumentSymbolsWithProjectContext(result *ToolResult, args map[string]interface{}, workspaceCtx *WorkspaceContext) *ToolResult {
	projectCtx := workspaceCtx.GetProjectContext()
	if projectCtx == nil {
		return result
	}

	// Add project context to metadata
	if result.Meta == nil {
		result.Meta = &ResponseMetadata{}
	}
	if result.Meta.RequestInfo == nil {
		result.Meta.RequestInfo = make(map[string]interface{})
	}

	// Add file context
	if uri, ok := args["uri"].(string); ok {
		result.Meta.RequestInfo["file_context"] = map[string]interface{}{
			"in_project":    workspaceCtx.IsFileInProject(uri),
			"language":      workspaceCtx.GetLanguageForFile(uri),
			"relative_path": h.getRelativePathFromContext(uri, workspaceCtx),
		}
	}

	result.Meta.RequestInfo["project_context"] = map[string]interface{}{
		"project_type": projectCtx.ProjectType,
		"file_type":    h.categorizeFileType(args, workspaceCtx),
	}

	// Enhance symbols with project categorization
	if len(result.Content) > 0 && result.Content[0].Data != nil {
		result.Content[0] = h.enhanceDocumentSymbolsContent(result.Content[0], workspaceCtx)
	}

	return result
}

func (h *ToolHandler) enhanceWorkspaceSymbolsWithProjectContext(result *ToolResult, args map[string]interface{}, workspaceCtx *WorkspaceContext) *ToolResult {
	projectCtx := workspaceCtx.GetProjectContext()
	if projectCtx == nil {
		return result
	}

	// Add project context to metadata
	if result.Meta == nil {
		result.Meta = &ResponseMetadata{}
	}
	if result.Meta.RequestInfo == nil {
		result.Meta.RequestInfo = make(map[string]interface{})
	}

	result.Meta.RequestInfo["project_context"] = map[string]interface{}{
		"project_type":     projectCtx.ProjectType,
		"primary_language": projectCtx.PrimaryLanguage,
		"workspace_roots":  workspaceCtx.GetWorkspaceRoots(),
		"active_languages": workspaceCtx.GetActiveLanguages(),
	}

	if query, ok := args["query"].(string); ok {
		result.Meta.RequestInfo["search_context"] = map[string]interface{}{
			"query":                query,
			"project_scoped":       true,
			"multi_language_search": len(projectCtx.Languages) > 1,
		}
	}

	// Filter and enhance symbols with project context
	if len(result.Content) > 0 && result.Content[0].Data != nil {
		result.Content[0] = h.enhanceWorkspaceSymbolsContent(result.Content[0], workspaceCtx)
	}

	return result
}

// Content enhancement methods

func (h *ToolHandler) enhanceDefinitionContent(content ContentBlock, workspaceCtx *WorkspaceContext) ContentBlock {
	enhanced := content
	
	// Add project-aware annotations
	if enhanced.Annotations == nil {
		enhanced.Annotations = make(map[string]interface{})
	}
	
	enhanced.Annotations["project_enhanced"] = true
	enhanced.Annotations["workspace_aware"] = workspaceCtx.IsProjectAware()
	
	// Try to parse and enhance definition locations
	if definitions, ok := content.Data.([]interface{}); ok {
		enhancedDefs := h.filterAndEnhanceDefinitions(definitions, workspaceCtx)
		enhanced.Data = enhancedDefs
		enhanced.Text = h.formatEnhancedDefinitions(enhancedDefs)
	}
	
	return enhanced
}

func (h *ToolHandler) enhanceReferencesContent(content ContentBlock, workspaceCtx *WorkspaceContext) ContentBlock {
	enhanced := content
	
	if enhanced.Annotations == nil {
		enhanced.Annotations = make(map[string]interface{})
	}
	
	enhanced.Annotations["project_enhanced"] = true
	
	// Parse and categorize references by project structure
	if references, ok := content.Data.([]interface{}); ok {
		categorized := h.categorizeReferencesByProject(references, workspaceCtx)
		enhanced.Data = categorized
		enhanced.Text = h.formatCategorizedReferences(categorized)
	}
	
	return enhanced
}

func (h *ToolHandler) enhanceHoverContent(content ContentBlock, workspaceCtx *WorkspaceContext) ContentBlock {
	enhanced := content
	
	if enhanced.Annotations == nil {
		enhanced.Annotations = make(map[string]interface{})
	}
	
	enhanced.Annotations["project_enhanced"] = true
	enhanced.Annotations["language_context"] = workspaceCtx.GetProjectContext().PrimaryLanguage
	
	return enhanced
}

func (h *ToolHandler) enhanceDocumentSymbolsContent(content ContentBlock, workspaceCtx *WorkspaceContext) ContentBlock {
	enhanced := content
	
	if enhanced.Annotations == nil {
		enhanced.Annotations = make(map[string]interface{})
	}
	
	enhanced.Annotations["project_enhanced"] = true
	
	// Categorize symbols by type and importance within project context
	if symbols, ok := content.Data.([]interface{}); ok {
		categorized := h.categorizeSymbolsByProject(symbols, workspaceCtx)
		enhanced.Data = categorized
		enhanced.Text = h.formatCategorizedSymbols(categorized)
	}
	
	return enhanced
}

func (h *ToolHandler) enhanceWorkspaceSymbolsContent(content ContentBlock, workspaceCtx *WorkspaceContext) ContentBlock {
	enhanced := content
	
	if enhanced.Annotations == nil {
		enhanced.Annotations = make(map[string]interface{})
	}
	
	enhanced.Annotations["project_enhanced"] = true
	enhanced.Annotations["project_filtered"] = true
	
	// Filter symbols to project scope and enhance with context
	if symbols, ok := content.Data.([]interface{}); ok {
		filtered := h.filterSymbolsToProject(symbols, workspaceCtx)
		enhanced.Data = filtered
		enhanced.Text = h.formatProjectScopedSymbols(filtered, workspaceCtx)
	}
	
	return enhanced
}

// Utility methods for project-aware processing

func (h *ToolHandler) getRelativePathFromContext(uri string, workspaceCtx *WorkspaceContext) string {
	if workspaceCtx == nil {
		return ""
	}
	
	// Convert URI to file path
	filePath := strings.TrimPrefix(uri, "file://")
	projectRoot := workspaceCtx.GetProjectRoot()
	
	if relPath, err := filepath.Rel(projectRoot, filePath); err == nil {
		return relPath
	}
	
	return filepath.Base(filePath)
}

func (h *ToolHandler) categorizeFileType(args map[string]interface{}, workspaceCtx *WorkspaceContext) string {
	if uri, ok := args["uri"].(string); ok {
		language := workspaceCtx.GetLanguageForFile(uri)
		relPath := h.getRelativePathFromContext(uri, workspaceCtx)
		
		// Categorize based on path patterns
		if strings.Contains(relPath, "test") || strings.Contains(relPath, "spec") {
			return "test_file"
		} else if strings.Contains(relPath, "config") || strings.HasSuffix(relPath, ".config") {
			return "config_file"
		} else if language != "" {
			return "source_file"
		}
	}
	
	return "unknown_file"
}

func (h *ToolHandler) filterAndEnhanceDefinitions(definitions []interface{}, workspaceCtx *WorkspaceContext) []interface{} {
	enhanced := make([]interface{}, 0, len(definitions))
	
	for _, def := range definitions {
		if defMap, ok := def.(map[string]interface{}); ok {
			// Enhance with project context
			enhancedDef := make(map[string]interface{})
			for k, v := range defMap {
				enhancedDef[k] = v
			}
			
			// Add project-specific information
			if uri, exists := h.extractURIFromLocation(defMap); exists {
				enhancedDef["project_info"] = map[string]interface{}{
					"in_project":    workspaceCtx.IsFileInProject(uri),
					"relative_path": h.getRelativePathFromContext(uri, workspaceCtx),
					"language":      workspaceCtx.GetLanguageForFile(uri),
				}
			}
			
			enhanced = append(enhanced, enhancedDef)
		} else {
			enhanced = append(enhanced, def)
		}
	}
	
	return enhanced
}

func (h *ToolHandler) categorizeReferencesByProject(references []interface{}, workspaceCtx *WorkspaceContext) map[string]interface{} {
	projectReferences := make([]interface{}, 0)
	externalReferences := make([]interface{}, 0)
	
	for _, ref := range references {
		if refMap, ok := ref.(map[string]interface{}); ok {
			if uri, exists := h.extractURIFromLocation(refMap); exists {
				if workspaceCtx.IsFileInProject(uri) {
					// Enhance project references
					enhanced := make(map[string]interface{})
					for k, v := range refMap {
						enhanced[k] = v
					}
					enhanced["project_info"] = map[string]interface{}{
						"relative_path": h.getRelativePathFromContext(uri, workspaceCtx),
						"language":      workspaceCtx.GetLanguageForFile(uri),
					}
					projectReferences = append(projectReferences, enhanced)
				} else {
					externalReferences = append(externalReferences, ref)
				}
			}
		}
	}
	
	return map[string]interface{}{
		"project_references":  projectReferences,
		"external_references": externalReferences,
		"total_project":       len(projectReferences),
		"total_external":      len(externalReferences),
	}
}

func (h *ToolHandler) categorizeSymbolsByProject(symbols []interface{}, workspaceCtx *WorkspaceContext) map[string]interface{} {
	publicSymbols := make([]interface{}, 0)
	privateSymbols := make([]interface{}, 0)
	testSymbols := make([]interface{}, 0)
	
	for _, sym := range symbols {
		if symMap, ok := sym.(map[string]interface{}); ok {
			// Categorize based on symbol name and context
			if name, exists := symMap["name"].(string); exists {
				if strings.HasPrefix(name, "test") || strings.HasPrefix(name, "Test") {
					testSymbols = append(testSymbols, sym)
				} else if strings.HasPrefix(name, "_") || strings.ToLower(name) == name {
					privateSymbols = append(privateSymbols, sym)
				} else {
					publicSymbols = append(publicSymbols, sym)
				}
			}
		}
	}
	
	return map[string]interface{}{
		"public_symbols":  publicSymbols,
		"private_symbols": privateSymbols,
		"test_symbols":    testSymbols,
		"total_public":    len(publicSymbols),
		"total_private":   len(privateSymbols),
		"total_test":      len(testSymbols),
	}
}

func (h *ToolHandler) filterSymbolsToProject(symbols []interface{}, workspaceCtx *WorkspaceContext) []interface{} {
	filtered := make([]interface{}, 0)
	
	for _, sym := range symbols {
		if symMap, ok := sym.(map[string]interface{}); ok {
			if uri, exists := h.extractURIFromLocation(symMap); exists {
				if workspaceCtx.IsFileInProject(uri) {
					// Enhance with project info
					enhanced := make(map[string]interface{})
					for k, v := range symMap {
						enhanced[k] = v
					}
					enhanced["project_info"] = map[string]interface{}{
						"relative_path": h.getRelativePathFromContext(uri, workspaceCtx),
						"language":      workspaceCtx.GetLanguageForFile(uri),
					}
					filtered = append(filtered, enhanced)
				}
			}
		}
	}
	
	return filtered
}

func (h *ToolHandler) extractURIFromLocation(item map[string]interface{}) (string, bool) {
	if location, exists := item["location"]; exists {
		if locationMap, ok := location.(map[string]interface{}); ok {
			if uri, exists := locationMap["uri"]; exists {
				if uriStr, ok := uri.(string); ok {
					return uriStr, true
				}
			}
		}
	}
	// Also check for direct URI field
	if uri, exists := item["uri"]; exists {
		if uriStr, ok := uri.(string); ok {
			return uriStr, true
		}
	}
	return "", false
}

// Formatting methods for enhanced results

func (h *ToolHandler) formatEnhancedDefinitions(definitions []interface{}) string {
	if len(definitions) == 0 {
		return "No definitions found"
	}
	
	var result strings.Builder
	result.WriteString(fmt.Sprintf("Found %d definition(s):\n", len(definitions)))
	
	for i, def := range definitions {
		if defMap, ok := def.(map[string]interface{}); ok {
			result.WriteString(fmt.Sprintf("%d. ", i+1))
			
			if projectInfo, exists := defMap["project_info"].(map[string]interface{}); exists {
				if relPath, exists := projectInfo["relative_path"].(string); exists {
					result.WriteString(fmt.Sprintf("%s", relPath))
				}
				if lang, exists := projectInfo["language"].(string); exists && lang != "" {
					result.WriteString(fmt.Sprintf(" (%s)", lang))
				}
			}
			
			result.WriteString("\n")
		}
	}
	
	return result.String()
}

func (h *ToolHandler) formatCategorizedReferences(categorized map[string]interface{}) string {
	var result strings.Builder
	
	if totalProject, ok := categorized["total_project"].(int); ok {
		result.WriteString(fmt.Sprintf("Project references: %d\n", totalProject))
	}
	
	if totalExternal, ok := categorized["total_external"].(int); ok {
		result.WriteString(fmt.Sprintf("External references: %d\n", totalExternal))
	}
	
	return result.String()
}

func (h *ToolHandler) formatCategorizedSymbols(categorized map[string]interface{}) string {
	var result strings.Builder
	
	if totalPublic, ok := categorized["total_public"].(int); ok {
		result.WriteString(fmt.Sprintf("Public symbols: %d\n", totalPublic))
	}
	
	if totalPrivate, ok := categorized["total_private"].(int); ok {
		result.WriteString(fmt.Sprintf("Private symbols: %d\n", totalPrivate))
	}
	
	if totalTest, ok := categorized["total_test"].(int); ok {
		result.WriteString(fmt.Sprintf("Test symbols: %d\n", totalTest))
	}
	
	return result.String()
}

func (h *ToolHandler) formatProjectScopedSymbols(symbols []interface{}, workspaceCtx *WorkspaceContext) string {
	var result strings.Builder
	
	result.WriteString(fmt.Sprintf("Found %d symbol(s) in project:\n", len(symbols)))
	
	for i, sym := range symbols {
		if symMap, ok := sym.(map[string]interface{}); ok {
			result.WriteString(fmt.Sprintf("%d. ", i+1))
			
			if name, exists := symMap["name"].(string); exists {
				result.WriteString(name)
			}
			
			if projectInfo, exists := symMap["project_info"].(map[string]interface{}); exists {
				if relPath, exists := projectInfo["relative_path"].(string); exists {
					result.WriteString(fmt.Sprintf(" - %s", relPath))
				}
			}
			
			result.WriteString("\n")
		}
	}
	
	return result.String()
}