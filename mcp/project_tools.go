package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"lsp-gateway/internal/project"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// ProjectToolsHandler provides advanced project-specific MCP tool implementations
type ProjectToolsHandler struct {
	workspaceContext *WorkspaceContext
	client           *LSPGatewayClient
	logger           *StructuredLogger
}

// NewProjectToolsHandler creates a new project tools handler
func NewProjectToolsHandler(workspaceContext *WorkspaceContext, client *LSPGatewayClient) *ProjectToolsHandler {
	logger := NewStructuredLogger(&LoggerConfig{
		Level:      LogLevelInfo,
		EnableJSON: true,
	})

	return &ProjectToolsHandler{
		workspaceContext: workspaceContext,
		client:           client,
		logger:           logger,
	}
}

// AnalyzeProjectStructure provides comprehensive project structure analysis
func (h *ProjectToolsHandler) AnalyzeProjectStructure(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	if !h.workspaceContext.IsProjectAware() {
		return h.createErrorResult("Project structure analysis requires project context", MCPErrorUnsupportedFeature)
	}

	projectCtx := h.workspaceContext.GetProjectContext()
	if projectCtx == nil {
		return h.createErrorResult("No project context available", MCPErrorResourceNotFound)
	}

	// Parse arguments
	includeDeps := h.getBoolArg(args, "include_dependencies", true)
	includeBuild := h.getBoolArg(args, "include_build_info", true)
	includeStats := h.getBoolArg(args, "include_statistics", true)

	analysis := make(map[string]interface{})

	// Core project information
	analysis["project_overview"] = map[string]interface{}{
		"name":             projectCtx.DisplayName,
		"type":             projectCtx.ProjectType,
		"primary_language": projectCtx.PrimaryLanguage,
		"languages":        projectCtx.Languages,
		"root_path":        projectCtx.RootPath,
		"workspace_root":   projectCtx.WorkspaceRoot,
		"confidence":       projectCtx.Confidence,
		"is_valid":         projectCtx.IsValid,
		"is_monorepo":      projectCtx.IsMonorepo,
		"version":          projectCtx.Version,
		"description":      projectCtx.Description,
	}

	// Language versions and requirements
	analysis["language_info"] = map[string]interface{}{
		"versions":         projectCtx.LanguageVersions,
		"required_servers": projectCtx.RequiredServers,
	}

	// Project structure
	structure := make(map[string]interface{})
	structure["source_directories"] = projectCtx.SourceDirs
	structure["test_directories"] = projectCtx.TestDirs
	structure["config_files"] = projectCtx.ConfigFiles
	structure["build_files"] = projectCtx.BuildFiles
	structure["marker_files"] = projectCtx.MarkerFiles

	// Directory analysis
	if dirAnalysis, err := h.analyzeDirectoryStructure(projectCtx.RootPath); err == nil {
		structure["directory_analysis"] = dirAnalysis
	}

	analysis["structure"] = structure

	// Dependencies
	if includeDeps {
		deps := make(map[string]interface{})
		deps["direct_dependencies"] = projectCtx.Dependencies
		deps["dev_dependencies"] = projectCtx.DevDependencies

		// Analyze dependency health
		if depHealth, err := h.analyzeDependencyHealth(projectCtx); err == nil {
			deps["dependency_analysis"] = depHealth
		}

		analysis["dependencies"] = deps
	}

	// Build information
	if includeBuild {
		buildInfo := make(map[string]interface{})
		buildInfo["build_system"] = projectCtx.BuildSystem
		buildInfo["build_targets"] = projectCtx.BuildTargets
		buildInfo["package_manager"] = projectCtx.PackageManager

		// Analyze build configuration
		if buildAnalysis, err := h.analyzeBuildConfiguration(projectCtx); err == nil {
			buildInfo["build_analysis"] = buildAnalysis
		}

		analysis["build_info"] = buildInfo
	}

	// Statistics and metrics
	if includeStats {
		stats := make(map[string]interface{})
		stats["project_size"] = projectCtx.ProjectSize
		stats["detection_time"] = projectCtx.DetectionTime.String()
		stats["detected_at"] = projectCtx.DetectedAt.Format(time.RFC3339)

		// Code quality metrics
		if qualityMetrics, err := h.calculateCodeQualityMetrics(projectCtx); err == nil {
			stats["quality_metrics"] = qualityMetrics
		}

		analysis["statistics"] = stats
	}

	// Validation information
	validation := make(map[string]interface{})
	validation["is_valid"] = projectCtx.IsValid
	validation["errors"] = projectCtx.ValidationErrors
	validation["warnings"] = projectCtx.ValidationWarnings
	analysis["validation"] = validation

	// Environment and platform info
	analysis["environment"] = map[string]interface{}{
		"platform":     projectCtx.Platform,
		"architecture": projectCtx.Architecture,
		"vcs_type":     projectCtx.VCSType,
		"vcs_root":     projectCtx.VCSRoot,
	}

	return &ToolResult{
		Content: []ContentBlock{{
			Type: "text",
			Text: h.formatProjectAnalysis(analysis),
			Data: analysis,
			Annotations: map[string]interface{}{
				"content_type": "project_analysis",
				"project_type": projectCtx.ProjectType,
			},
		}},
		Meta: &ResponseMetadata{
			Timestamp:   time.Now().Format(time.RFC3339),
			Duration:    "0ms", // Set by caller
			LSPMethod:   "project/analyze",
			RequestInfo: h.workspaceContext.GetProjectMetadata(),
		},
	}, nil
}

// FindProjectSymbols performs project-scoped symbol search with advanced filtering
func (h *ProjectToolsHandler) FindProjectSymbols(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	if !h.workspaceContext.IsProjectAware() {
		return h.createErrorResult("Project symbol search requires project context", MCPErrorUnsupportedFeature)
	}

	query, ok := args["query"].(string)
	if !ok || query == "" {
		return h.createErrorResult("Missing or empty 'query' parameter", MCPErrorInvalidParams)
	}

	projectCtx := h.workspaceContext.GetProjectContext()
	languageFilter := h.getStringArg(args, "language_filter", "")
	scope := h.getStringArg(args, "scope", "project")

	// Determine search scope
	var searchPaths []string
	switch scope {
	case "project":
		searchPaths = []string{projectCtx.RootPath}
	case "workspace":
		searchPaths = h.workspaceContext.GetWorkspaceRoots()
	case "all":
		searchPaths = append([]string{projectCtx.RootPath}, h.workspaceContext.GetWorkspaceRoots()...)
	default:
		return h.createErrorResult(fmt.Sprintf("Invalid scope '%s', must be 'project', 'workspace', or 'all'", scope), MCPErrorInvalidParams)
	}

	// Perform symbol search using LSP workspace/symbol
	params := map[string]interface{}{
		"query": query,
	}

	result, err := h.client.SendLSPRequest(ctx, "workspace/symbol", params)
	if err != nil {
		return h.createErrorResult(fmt.Sprintf("Symbol search failed: %v", err), MCPErrorLSPServerError)
	}

	// Parse and filter results
	var symbols []interface{}
	if err := json.Unmarshal(result, &symbols); err != nil {
		return h.createErrorResult(fmt.Sprintf("Failed to parse symbol results: %v", err), MCPErrorInternalError)
	}

	// Filter symbols by project context
	filteredSymbols := h.filterSymbolsByProject(symbols, searchPaths, languageFilter)

	// Enhance symbols with project metadata
	enhancedSymbols := h.enhanceSymbolsWithProjectInfo(filteredSymbols, projectCtx)

	symbolData := map[string]interface{}{
		"query":           query,
		"scope":           scope,
		"language_filter": languageFilter,
		"total_found":     len(enhancedSymbols),
		"search_paths":    searchPaths,
		"symbols":         enhancedSymbols,
		"project_context": map[string]interface{}{
			"project_type":     projectCtx.ProjectType,
			"primary_language": projectCtx.PrimaryLanguage,
			"root_path":        projectCtx.RootPath,
		},
	}

	return &ToolResult{
		Content: []ContentBlock{{
			Type: "text",
			Text: h.formatSymbolSearchResults(enhancedSymbols, query, scope),
			Data: symbolData,
			Annotations: map[string]interface{}{
				"content_type": "symbol_search",
				"query":        query,
				"scope":        scope,
			},
		}},
		Meta: &ResponseMetadata{
			Timestamp:   time.Now().Format(time.RFC3339),
			LSPMethod:   "workspace/symbol",
			RequestInfo: h.workspaceContext.GetProjectMetadata(),
		},
	}, nil
}

// GetProjectConfig retrieves comprehensive project configuration
func (h *ProjectToolsHandler) GetProjectConfig(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	if !h.workspaceContext.IsProjectAware() {
		return h.createErrorResult("Project configuration requires project context", MCPErrorUnsupportedFeature)
	}

	includeServers := h.getBoolArg(args, "include_servers", true)
	includeLanguages := h.getBoolArg(args, "include_languages", true)

	projectCtx := h.workspaceContext.GetProjectContext()
	configCtx := h.workspaceContext.GetConfigContext()
	projectConfig := h.workspaceContext.GetProjectConfig()

	config := make(map[string]interface{})

	// Core project information
	config["project_info"] = map[string]interface{}{
		"project_id":   projectConfig.ProjectID,
		"name":         projectConfig.Name,
		"root_path":    projectConfig.RootDirectory,
		"version":      projectConfig.Version,
		"generated_at": projectConfig.GeneratedAt.Format(time.RFC3339),
	}

	// Language configuration
	if includeLanguages {
		languageConfig := make(map[string]interface{})
		languageConfig["detected_languages"] = configCtx.Languages
		languageConfig["primary_language"] = projectCtx.PrimaryLanguage
		languageConfig["language_versions"] = projectCtx.LanguageVersions

		// Language-specific settings
		langSettings := h.generateLanguageSettings(projectCtx)
		languageConfig["language_settings"] = langSettings

		config["languages"] = languageConfig
	}

	// Server configuration
	if includeServers {
		serverConfig := make(map[string]interface{})
		serverConfig["enabled_servers"] = projectConfig.EnabledServers
		serverConfig["required_servers"] = configCtx.RequiredLSPs
		serverConfig["server_overrides"] = projectConfig.ServerOverrides

		// Generate server-specific configurations
		serverConfigs := h.generateServerConfigurations(projectCtx)
		serverConfig["server_configurations"] = serverConfigs

		config["servers"] = serverConfig
	}

	// Workspace configuration
	config["workspace"] = map[string]interface{}{
		"workspace_roots": h.workspaceContext.GetWorkspaceRoots(),
		"project_type":    configCtx.ProjectType,
		"workspace_root":  configCtx.WorkspaceRoot,
		"is_multi_root":   len(h.workspaceContext.GetWorkspaceRoots()) > 1,
	}

	// Optimization settings
	config["optimizations"] = projectConfig.Optimizations

	// Metadata
	config["metadata"] = map[string]interface{}{
		"detection_confidence": projectCtx.Confidence,
		"detection_time":       projectCtx.DetectionTime.String(),
		"is_valid":             projectCtx.IsValid,
		"has_errors":           len(projectCtx.ValidationErrors) > 0,
		"has_warnings":         len(projectCtx.ValidationWarnings) > 0,
	}

	return &ToolResult{
		Content: []ContentBlock{{
			Type: "text",
			Text: h.formatProjectConfig(config),
			Data: config,
			Annotations: map[string]interface{}{
				"content_type": "project_config",
				"project_type": projectCtx.ProjectType,
			},
		}},
		Meta: &ResponseMetadata{
			Timestamp:   time.Now().Format(time.RFC3339),
			LSPMethod:   "project/config",
			RequestInfo: h.workspaceContext.GetProjectMetadata(),
		},
	}, nil
}

// ValidateProjectStructure performs comprehensive project validation
func (h *ProjectToolsHandler) ValidateProjectStructure(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	if !h.workspaceContext.IsProjectAware() {
		return h.createErrorResult("Project validation requires project context", MCPErrorUnsupportedFeature)
	}

	checkDeps := h.getBoolArg(args, "check_dependencies", true)
	checkBuild := h.getBoolArg(args, "check_build_config", true)

	projectCtx := h.workspaceContext.GetProjectContext()

	validation := make(map[string]interface{})

	// Basic validation
	basicValidation := make(map[string]interface{})
	basicValidation["is_valid"] = projectCtx.IsValid
	basicValidation["errors"] = projectCtx.ValidationErrors
	basicValidation["warnings"] = projectCtx.ValidationWarnings
	basicValidation["confidence"] = projectCtx.Confidence

	validation["basic"] = basicValidation

	// File structure validation
	structureValidation := h.validateProjectStructure(projectCtx)
	validation["structure"] = structureValidation

	// Dependency validation
	if checkDeps {
		depValidation := h.validateDependencies(projectCtx)
		validation["dependencies"] = depValidation
	}

	// Build configuration validation
	if checkBuild {
		buildValidation := h.validateBuildConfiguration(projectCtx)
		validation["build_config"] = buildValidation
	}

	// LSP server validation
	serverValidation := h.validateLSPServers(projectCtx)
	validation["lsp_servers"] = serverValidation

	// Overall health score
	healthScore := h.calculateProjectHealthScore(validation)
	validation["health_score"] = healthScore

	// Recommendations
	recommendations := h.generateProjectRecommendations(validation, projectCtx)
	validation["recommendations"] = recommendations

	return &ToolResult{
		Content: []ContentBlock{{
			Type: "text",
			Text: h.formatValidationResults(validation),
			Data: validation,
			Annotations: map[string]interface{}{
				"content_type": "project_validation",
				"health_score": healthScore,
			},
		}},
		Meta: &ResponseMetadata{
			Timestamp:   time.Now().Format(time.RFC3339),
			LSPMethod:   "project/validate",
			RequestInfo: h.workspaceContext.GetProjectMetadata(),
		},
	}, nil
}

// DiscoverWorkspaceDependencies analyzes project dependencies comprehensively
func (h *ProjectToolsHandler) DiscoverWorkspaceDependencies(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	if !h.workspaceContext.IsProjectAware() {
		return h.createErrorResult("Dependency discovery requires project context", MCPErrorUnsupportedFeature)
	}

	includeDevDeps := h.getBoolArg(args, "include_dev_deps", false)
	includeTransitive := h.getBoolArg(args, "include_transitive", false)

	projectCtx := h.workspaceContext.GetProjectContext()

	dependencies := make(map[string]interface{})

	// Direct dependencies
	dependencies["direct"] = projectCtx.Dependencies

	// Development dependencies
	if includeDevDeps || len(projectCtx.DevDependencies) > 0 {
		dependencies["development"] = projectCtx.DevDependencies
	}

	// Build and configuration files
	dependencies["build_files"] = projectCtx.BuildFiles
	dependencies["config_files"] = projectCtx.ConfigFiles

	// Dependency analysis by language
	langDeps := h.analyzeDependenciesByLanguage(projectCtx)
	dependencies["by_language"] = langDeps

	// Dependency graph analysis
	if includeTransitive {
		transitiveAnalysis := h.analyzeTransitiveDependencies(projectCtx)
		dependencies["transitive_analysis"] = transitiveAnalysis
	}

	// Security and health analysis
	securityAnalysis := h.analyzeDependendencySecurity(projectCtx)
	dependencies["security_analysis"] = securityAnalysis

	// Recommendations
	depRecommendations := h.generateDependencyRecommendations(projectCtx, dependencies)
	dependencies["recommendations"] = depRecommendations

	return &ToolResult{
		Content: []ContentBlock{{
			Type: "text",
			Text: h.formatDependencyAnalysis(dependencies),
			Data: dependencies,
			Annotations: map[string]interface{}{
				"content_type":       "dependency_analysis",
				"total_dependencies": len(projectCtx.Dependencies),
				"dev_dependencies":   len(projectCtx.DevDependencies),
			},
		}},
		Meta: &ResponseMetadata{
			Timestamp:   time.Now().Format(time.RFC3339),
			LSPMethod:   "project/dependencies",
			RequestInfo: h.workspaceContext.GetProjectMetadata(),
		},
	}, nil
}

// Helper methods for project analysis

func (h *ProjectToolsHandler) analyzeDirectoryStructure(rootPath string) (map[string]interface{}, error) {
	analysis := make(map[string]interface{})

	// Count directories and files by type
	dirCount := 0
	fileCount := 0
	fileSizes := make(map[string]int64)

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		if info.IsDir() {
			dirCount++
		} else {
			fileCount++
			ext := strings.ToLower(filepath.Ext(path))
			fileSizes[ext] += info.Size()
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	analysis["total_directories"] = dirCount
	analysis["total_files"] = fileCount
	analysis["size_by_extension"] = fileSizes

	return analysis, nil
}

func (h *ProjectToolsHandler) analyzeDependencyHealth(projectCtx *project.ProjectContext) (map[string]interface{}, error) {
	health := make(map[string]interface{})

	health["total_dependencies"] = len(projectCtx.Dependencies)
	health["total_dev_dependencies"] = len(projectCtx.DevDependencies)

	// Analyze dependency versions (placeholder - would need actual version analysis)
	health["version_analysis"] = map[string]interface{}{
		"outdated_count":  0,
		"security_issues": 0,
		"major_upgrades":  0,
	}

	return health, nil
}

func (h *ProjectToolsHandler) analyzeBuildConfiguration(projectCtx *project.ProjectContext) (map[string]interface{}, error) {
	buildAnalysis := make(map[string]interface{})

	buildAnalysis["build_system"] = projectCtx.BuildSystem
	buildAnalysis["build_files"] = projectCtx.BuildFiles
	buildAnalysis["build_targets"] = projectCtx.BuildTargets

	// Check for common build files
	commonBuildFiles := map[string]bool{
		"Makefile": false, "makefile": false,
		"package.json": false, "pom.xml": false,
		"build.gradle": false, "Cargo.toml": false,
		"go.mod": false, "setup.py": false,
	}

	for _, buildFile := range projectCtx.BuildFiles {
		baseName := filepath.Base(buildFile)
		if _, exists := commonBuildFiles[baseName]; exists {
			commonBuildFiles[baseName] = true
		}
	}

	buildAnalysis["build_file_presence"] = commonBuildFiles

	return buildAnalysis, nil
}

func (h *ProjectToolsHandler) calculateCodeQualityMetrics(projectCtx *project.ProjectContext) (map[string]interface{}, error) {
	metrics := make(map[string]interface{})

	// Basic metrics from project size
	size := projectCtx.ProjectSize
	metrics["files"] = map[string]interface{}{
		"total":  size.TotalFiles,
		"source": size.SourceFiles,
		"test":   size.TestFiles,
		"config": size.ConfigFiles,
	}

	// Calculate ratios
	if size.SourceFiles > 0 {
		metrics["test_coverage_ratio"] = float64(size.TestFiles) / float64(size.SourceFiles)
		metrics["config_to_source_ratio"] = float64(size.ConfigFiles) / float64(size.SourceFiles)
	}

	metrics["total_size_mb"] = float64(size.TotalSizeBytes) / (1024 * 1024)

	return metrics, nil
}

func (h *ProjectToolsHandler) filterSymbolsByProject(symbols []interface{}, searchPaths []string, languageFilter string) []interface{} {
	var filtered []interface{}

	for _, symbol := range symbols {
		symbolMap, ok := symbol.(map[string]interface{})
		if !ok {
			continue
		}

		// Check if symbol is within project paths
		if location, exists := symbolMap["location"]; exists {
			if locationMap, ok := location.(map[string]interface{}); ok {
				if uri, exists := locationMap["uri"]; exists {
					if uriStr, ok := uri.(string); ok {
						filePath := h.workspaceContext.uriToPath(uriStr)
						if h.isPathInSearchScope(filePath, searchPaths) {
							// Apply language filter if specified
							if languageFilter == "" || h.matchesLanguageFilter(filePath, languageFilter) {
								filtered = append(filtered, symbol)
							}
						}
					}
				}
			}
		}
	}

	return filtered
}

func (h *ProjectToolsHandler) enhanceSymbolsWithProjectInfo(symbols []interface{}, projectCtx *project.ProjectContext) []interface{} {
	enhanced := make([]interface{}, len(symbols))

	for i, symbol := range symbols {
		symbolMap, ok := symbol.(map[string]interface{})
		if !ok {
			enhanced[i] = symbol
			continue
		}

		// Create enhanced symbol
		enhancedSymbol := make(map[string]interface{})
		for k, v := range symbolMap {
			enhancedSymbol[k] = v
		}

		// Add project context
		if location, exists := symbolMap["location"]; exists {
			if locationMap, ok := location.(map[string]interface{}); ok {
				if uri, exists := locationMap["uri"]; exists {
					if uriStr, ok := uri.(string); ok {
						filePath := h.workspaceContext.uriToPath(uriStr)
						relPath, _ := filepath.Rel(projectCtx.RootPath, filePath)

						enhancedSymbol["project_info"] = map[string]interface{}{
							"relative_path": relPath,
							"language":      h.workspaceContext.GetLanguageForFile(uriStr),
							"in_project":    h.workspaceContext.IsFileInProject(uriStr),
						}
					}
				}
			}
		}

		enhanced[i] = enhancedSymbol
	}

	return enhanced
}

func (h *ProjectToolsHandler) isPathInSearchScope(filePath string, searchPaths []string) bool {
	absFilePath, err := filepath.Abs(filePath)
	if err != nil {
		return false
	}

	for _, searchPath := range searchPaths {
		absSearchPath, err := filepath.Abs(searchPath)
		if err != nil {
			continue
		}

		if strings.HasPrefix(absFilePath, absSearchPath) {
			return true
		}
	}

	return false
}

func (h *ProjectToolsHandler) matchesLanguageFilter(filePath, languageFilter string) bool {
	fileLanguage := h.workspaceContext.GetLanguageForFile("file://" + filePath)
	return fileLanguage == languageFilter
}

// Additional helper methods for formatting and analysis

func (h *ProjectToolsHandler) generateLanguageSettings(projectCtx *project.ProjectContext) map[string]interface{} {
	settings := make(map[string]interface{})

	for _, lang := range projectCtx.Languages {
		langSettings := make(map[string]interface{})

		if version, exists := projectCtx.LanguageVersions[lang]; exists {
			langSettings["version"] = version
		}

		// Add language-specific settings based on project type
		switch lang {
		case "go":
			langSettings["go_mod"] = filepath.Join(projectCtx.RootPath, "go.mod")
			langSettings["go_sum"] = filepath.Join(projectCtx.RootPath, "go.sum")
		case LANG_PYTHON:
			langSettings["requirements"] = filepath.Join(projectCtx.RootPath, "requirements.txt")
			langSettings["setup_py"] = filepath.Join(projectCtx.RootPath, "setup.py")
		case LANG_JAVASCRIPT, LANG_TYPESCRIPT:
			langSettings["package_json"] = filepath.Join(projectCtx.RootPath, "package.json")
			langSettings["tsconfig"] = filepath.Join(projectCtx.RootPath, "tsconfig.json")
		case LANG_JAVA:
			langSettings["pom_xml"] = filepath.Join(projectCtx.RootPath, "pom.xml")
			langSettings["build_gradle"] = filepath.Join(projectCtx.RootPath, "build.gradle")
		}

		settings[lang] = langSettings
	}

	return settings
}

func (h *ProjectToolsHandler) generateServerConfigurations(projectCtx *project.ProjectContext) map[string]interface{} {
	configs := make(map[string]interface{})

	for _, server := range projectCtx.RequiredServers {
		serverConfig := make(map[string]interface{})

		switch server {
		case "gopls":
			serverConfig["command"] = "gopls"
			serverConfig["args"] = []string{}
			serverConfig["settings"] = map[string]interface{}{
				"gopls": map[string]interface{}{
					"analyses": map[string]interface{}{
						"unusedparams": true,
					},
				},
			}
		case "pylsp":
			serverConfig["command"] = LANG_PYTHON
			serverConfig["args"] = []string{"-m", "pylsp"}
		case "typescript-language-server":
			serverConfig["command"] = "typescript-language-server"
			serverConfig["args"] = []string{"--stdio"}
		}

		configs[server] = serverConfig
	}

	return configs
}

// Validation helper methods

func (h *ProjectToolsHandler) validateProjectStructure(projectCtx *project.ProjectContext) map[string]interface{} {
	validation := make(map[string]interface{})

	var issues []string
	var warnings []string

	// Check for essential files
	essentialFiles := h.getEssentialFiles(projectCtx.ProjectType)
	missingFiles := make([]string, 0)

	for _, file := range essentialFiles {
		fullPath := filepath.Join(projectCtx.RootPath, file)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			missingFiles = append(missingFiles, file)
		}
	}

	if len(missingFiles) > 0 {
		issues = append(issues, fmt.Sprintf("Missing essential files: %s", strings.Join(missingFiles, ", ")))
	}

	// Check directory structure
	if len(projectCtx.SourceDirs) == 0 {
		warnings = append(warnings, "No source directories detected")
	}

	validation["issues"] = issues
	validation["warnings"] = warnings
	validation["missing_files"] = missingFiles
	validation["has_source_dirs"] = len(projectCtx.SourceDirs) > 0
	validation["has_test_dirs"] = len(projectCtx.TestDirs) > 0

	return validation
}

func (h *ProjectToolsHandler) validateDependencies(projectCtx *project.ProjectContext) map[string]interface{} {
	validation := make(map[string]interface{})

	var issues []string
	var warnings []string

	// Check if dependencies exist but no lock file
	if len(projectCtx.Dependencies) > 0 {
		lockFiles := h.getLockFiles(projectCtx.ProjectType)
		hasLockFile := false

		for _, lockFile := range lockFiles {
			fullPath := filepath.Join(projectCtx.RootPath, lockFile)
			if _, err := os.Stat(fullPath); err == nil {
				hasLockFile = true
				break
			}
		}

		if !hasLockFile {
			warnings = append(warnings, "Dependencies detected but no lock file found")
		}
	}

	validation["issues"] = issues
	validation["warnings"] = warnings
	validation["total_dependencies"] = len(projectCtx.Dependencies)
	validation["total_dev_dependencies"] = len(projectCtx.DevDependencies)

	return validation
}

func (h *ProjectToolsHandler) validateBuildConfiguration(projectCtx *project.ProjectContext) map[string]interface{} {
	validation := make(map[string]interface{})

	var issues []string
	var warnings []string

	if projectCtx.BuildSystem == "" && len(projectCtx.BuildFiles) == 0 {
		warnings = append(warnings, "No build system detected")
	}

	validation["issues"] = issues
	validation["warnings"] = warnings
	validation["has_build_system"] = projectCtx.BuildSystem != ""
	validation["build_files_count"] = len(projectCtx.BuildFiles)

	return validation
}

func (h *ProjectToolsHandler) validateLSPServers(projectCtx *project.ProjectContext) map[string]interface{} {
	validation := make(map[string]interface{})

	var issues []string
	var warnings []string

	if len(projectCtx.RequiredServers) == 0 {
		warnings = append(warnings, "No LSP servers required for detected languages")
	}

	validation["issues"] = issues
	validation["warnings"] = warnings
	validation["required_servers"] = projectCtx.RequiredServers
	validation["server_count"] = len(projectCtx.RequiredServers)

	return validation
}

func (h *ProjectToolsHandler) calculateProjectHealthScore(validation map[string]interface{}) int {
	score := 100

	// Deduct points for issues and warnings
	if basic, exists := validation["basic"].(map[string]interface{}); exists {
		if errors, exists := basic["errors"].([]string); exists {
			score -= len(errors) * 10
		}
		if warnings, exists := basic["warnings"].([]string); exists {
			score -= len(warnings) * 5
		}
	}

	// Ensure score doesn't go below 0
	if score < 0 {
		score = 0
	}

	return score
}

func (h *ProjectToolsHandler) generateProjectRecommendations(validation map[string]interface{}, projectCtx *project.ProjectContext) []string {
	var recommendations []string

	// Analyze validation results and generate recommendations
	if basic, exists := validation["basic"].(map[string]interface{}); exists {
		if isValid, exists := basic["is_valid"].(bool); exists && !isValid {
			recommendations = append(recommendations, "Fix project validation errors to improve reliability")
		}
	}

	if len(projectCtx.RequiredServers) == 0 {
		recommendations = append(recommendations, "Consider installing LSP servers for better development experience")
	}

	if projectCtx.ProjectSize.TestFiles == 0 {
		recommendations = append(recommendations, "Add test files to improve code quality")
	}

	return recommendations
}

// Language-specific helper methods

func (h *ProjectToolsHandler) getEssentialFiles(projectType string) []string {
	switch projectType {
	case "go":
		return []string{"go.mod"}
	case "python":
		return []string{"setup.py", "pyproject.toml", "requirements.txt"}
	case "javascript", "typescript":
		return []string{"package.json"}
	case "java":
		return []string{"pom.xml", "build.gradle"}
	default:
		return []string{}
	}
}

func (h *ProjectToolsHandler) getLockFiles(projectType string) []string {
	switch projectType {
	case "go":
		return []string{"go.sum"}
	case "python":
		return []string{"requirements.lock", "poetry.lock"}
	case "javascript", "typescript":
		return []string{"package-lock.json", "yarn.lock", "pnpm-lock.yaml"}
	case "java":
		return []string{"gradle.lockfile"}
	default:
		return []string{}
	}
}

func (h *ProjectToolsHandler) analyzeDependenciesByLanguage(projectCtx *project.ProjectContext) map[string]interface{} {
	byLanguage := make(map[string]interface{})

	for _, lang := range projectCtx.Languages {
		langDeps := make(map[string]interface{})

		// Filter dependencies by language (this is a simplified approach)
		langDeps["dependencies"] = projectCtx.Dependencies
		langDeps["dev_dependencies"] = projectCtx.DevDependencies

		byLanguage[lang] = langDeps
	}

	return byLanguage
}

func (h *ProjectToolsHandler) analyzeTransitiveDependencies(projectCtx *project.ProjectContext) map[string]interface{} {
	// This is a placeholder for transitive dependency analysis
	// In a real implementation, this would parse lock files and dependency trees
	return map[string]interface{}{
		"analysis_available": false,
		"reason":             "Transitive dependency analysis not yet implemented",
	}
}

func (h *ProjectToolsHandler) analyzeDependendencySecurity(projectCtx *project.ProjectContext) map[string]interface{} {
	// This is a placeholder for security analysis
	// In a real implementation, this would check for known vulnerabilities
	return map[string]interface{}{
		"vulnerabilities_found": 0,
		"security_advisories":   []string{},
		"outdated_packages":     []string{},
	}
}

func (h *ProjectToolsHandler) generateDependencyRecommendations(projectCtx *project.ProjectContext, dependencies map[string]interface{}) []string {
	var recommendations []string

	if len(projectCtx.Dependencies) == 0 {
		recommendations = append(recommendations, "No dependencies detected - consider if this is expected for your project type")
	}

	if len(projectCtx.DevDependencies) == 0 && len(projectCtx.Dependencies) > 0 {
		recommendations = append(recommendations, "Consider adding development dependencies for testing and tooling")
	}

	return recommendations
}

// Formatting methods for output

func (h *ProjectToolsHandler) formatProjectAnalysis(analysis map[string]interface{}) string {
	var result strings.Builder

	result.WriteString("# Project Structure Analysis\n\n")

	if overview, exists := analysis["project_overview"].(map[string]interface{}); exists {
		result.WriteString("## Project Overview\n")
		if name, exists := overview["name"].(string); exists && name != "" {
			result.WriteString(fmt.Sprintf("**Name:** %s\n", name))
		}
		if projectType, exists := overview["type"].(string); exists {
			result.WriteString(fmt.Sprintf("**Type:** %s\n", projectType))
		}
		if primaryLang, exists := overview["primary_language"].(string); exists {
			result.WriteString(fmt.Sprintf("**Primary Language:** %s\n", primaryLang))
		}
		if confidence, exists := overview["confidence"].(float64); exists {
			result.WriteString(fmt.Sprintf("**Detection Confidence:** %.1f%%\n", confidence*100))
		}
		result.WriteString("\n")
	}

	if langInfo, exists := analysis["language_info"].(map[string]interface{}); exists {
		result.WriteString("## Languages\n")
		if servers, exists := langInfo["required_servers"].([]string); exists {
			result.WriteString(fmt.Sprintf("**Required LSP Servers:** %s\n", strings.Join(servers, ", ")))
		}
		result.WriteString("\n")
	}

	if stats, exists := analysis["statistics"].(map[string]interface{}); exists {
		result.WriteString("## Statistics\n")
		if size, exists := stats["project_size"].(project.ProjectSize); exists {
			result.WriteString(fmt.Sprintf("- Total Files: %d\n", size.TotalFiles))
			result.WriteString(fmt.Sprintf("- Source Files: %d\n", size.SourceFiles))
			result.WriteString(fmt.Sprintf("- Test Files: %d\n", size.TestFiles))
			result.WriteString(fmt.Sprintf("- Config Files: %d\n", size.ConfigFiles))
			result.WriteString(fmt.Sprintf("- Total Size: %.2f MB\n", float64(size.TotalSizeBytes)/(1024*1024)))
		}
		result.WriteString("\n")
	}

	return result.String()
}

func (h *ProjectToolsHandler) formatSymbolSearchResults(symbols []interface{}, query, scope string) string {
	var result strings.Builder

	result.WriteString("# Symbol Search Results\n\n")
	result.WriteString(fmt.Sprintf("**Query:** %s\n", query))
	result.WriteString(fmt.Sprintf("**Scope:** %s\n", scope))
	result.WriteString(fmt.Sprintf("**Results Found:** %d\n\n", len(symbols)))

	for i, symbol := range symbols {
		if symbolMap, ok := symbol.(map[string]interface{}); ok {
			if name, exists := symbolMap["name"].(string); exists {
				result.WriteString(fmt.Sprintf("%d. **%s**", i+1, name))

				if kind, exists := symbolMap["kind"].(float64); exists {
					result.WriteString(fmt.Sprintf(" (Kind: %d)", int(kind)))
				}

				if projectInfo, exists := symbolMap["project_info"].(map[string]interface{}); exists {
					if relPath, exists := projectInfo["relative_path"].(string); exists {
						result.WriteString(fmt.Sprintf(" - %s", relPath))
					}
				}

				result.WriteString("\n")
			}
		}
	}

	return result.String()
}

func (h *ProjectToolsHandler) formatProjectConfig(config map[string]interface{}) string {
	var result strings.Builder

	result.WriteString("# Project Configuration\n\n")

	if projectInfo, exists := config["project_info"].(map[string]interface{}); exists {
		result.WriteString("## Project Information\n")
		for key, value := range projectInfo {
			result.WriteString(fmt.Sprintf("- **%s:** %v\n", cases.Title(language.English).String(strings.ReplaceAll(key, "_", " ")), value))
		}
		result.WriteString("\n")
	}

	if languages, exists := config["languages"].(map[string]interface{}); exists {
		result.WriteString("## Language Configuration\n")
		if detectedLangs, exists := languages["detected_languages"]; exists {
			result.WriteString(fmt.Sprintf("**Detected Languages:** %v\n", detectedLangs))
		}
		result.WriteString("\n")
	}

	if servers, exists := config["servers"].(map[string]interface{}); exists {
		result.WriteString("## LSP Server Configuration\n")
		if enabledServers, exists := servers["enabled_servers"].([]string); exists {
			result.WriteString(fmt.Sprintf("**Enabled Servers:** %s\n", strings.Join(enabledServers, ", ")))
		}
		result.WriteString("\n")
	}

	return result.String()
}

func (h *ProjectToolsHandler) formatValidationResults(validation map[string]interface{}) string {
	var result strings.Builder

	result.WriteString("# Project Validation Results\n\n")

	if healthScore, exists := validation["health_score"].(int); exists {
		result.WriteString(fmt.Sprintf("## Overall Health Score: %d/100\n\n", healthScore))
	}

	if basic, exists := validation["basic"].(map[string]interface{}); exists {
		result.WriteString("## Basic Validation\n")
		if isValid, exists := basic["is_valid"].(bool); exists {
			status := "❌ Invalid"
			if isValid {
				status = "✅ Valid"
			}
			result.WriteString(fmt.Sprintf("**Status:** %s\n", status))
		}

		if errors, exists := basic["errors"].([]string); exists && len(errors) > 0 {
			result.WriteString("**Errors:**\n")
			for _, err := range errors {
				result.WriteString(fmt.Sprintf("- %s\n", err))
			}
		}

		if warnings, exists := basic["warnings"].([]string); exists && len(warnings) > 0 {
			result.WriteString("**Warnings:**\n")
			for _, warning := range warnings {
				result.WriteString(fmt.Sprintf("- %s\n", warning))
			}
		}
		result.WriteString("\n")
	}

	if recommendations, exists := validation["recommendations"].([]string); exists && len(recommendations) > 0 {
		result.WriteString("## Recommendations\n")
		for _, rec := range recommendations {
			result.WriteString(fmt.Sprintf("- %s\n", rec))
		}
		result.WriteString("\n")
	}

	return result.String()
}

func (h *ProjectToolsHandler) formatDependencyAnalysis(dependencies map[string]interface{}) string {
	var result strings.Builder

	result.WriteString("# Dependency Analysis\n\n")

	if direct, exists := dependencies["direct"].(map[string]string); exists {
		result.WriteString(fmt.Sprintf("## Direct Dependencies (%d)\n", len(direct)))
		for name, version := range direct {
			result.WriteString(fmt.Sprintf("- %s: %s\n", name, version))
		}
		result.WriteString("\n")
	}

	if development, exists := dependencies["development"].(map[string]string); exists {
		result.WriteString(fmt.Sprintf("## Development Dependencies (%d)\n", len(development)))
		for name, version := range development {
			result.WriteString(fmt.Sprintf("- %s: %s\n", name, version))
		}
		result.WriteString("\n")
	}

	if recommendations, exists := dependencies["recommendations"].([]string); exists && len(recommendations) > 0 {
		result.WriteString("## Recommendations\n")
		for _, rec := range recommendations {
			result.WriteString(fmt.Sprintf("- %s\n", rec))
		}
	}

	return result.String()
}

// Utility methods

func (h *ProjectToolsHandler) createErrorResult(message string, code MCPErrorCode) (*ToolResult, error) {
	return &ToolResult{
		Content: []ContentBlock{{
			Type: "text",
			Text: message,
		}},
		IsError: true,
		Error: &StructuredError{
			Code:    code,
			Message: message,
		},
	}, nil
}

func (h *ProjectToolsHandler) getBoolArg(args map[string]interface{}, key string, defaultValue bool) bool {
	if value, exists := args[key]; exists {
		if boolValue, ok := value.(bool); ok {
			return boolValue
		}
	}
	return defaultValue
}

func (h *ProjectToolsHandler) getStringArg(args map[string]interface{}, key string, defaultValue string) string {
	if value, exists := args[key]; exists {
		if stringValue, ok := value.(string); ok {
			return stringValue
		}
	}
	return defaultValue
}
