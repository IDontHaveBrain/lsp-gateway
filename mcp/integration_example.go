package mcp

import (
	"context"
	"fmt"
	"lsp-gateway/internal/config"
	"log"
	"time"
)

// IntegrationExample demonstrates how to use the enhanced MCP tools with project context awareness
type IntegrationExample struct {
	projectHandler     *ProjectAwareToolHandler
	workspaceContext   *WorkspaceContext
	projectToolsHandler *ProjectToolsHandler
	logger             *StructuredLogger
}

// NewIntegrationExample creates a new integration example
func NewIntegrationExample(gatewayURL string, projectPath string) (*IntegrationExample, error) {
	// Create LSP Gateway client
	client := NewLSPGatewayClient(&ServerConfig{
		LSPGatewayURL: gatewayURL,
	})
	
	// Create logger
	logger := NewStructuredLogger(&LoggerConfig{
		Level:      LogLevelInfo,
		EnableJSON: true,
	})

	// Configure project-aware settings
	config := &ProjectAwareConfig{
		EnableProjectAware: true,
		AutoDetectProject:  true,
		ProjectPath:        projectPath,
		WorkspaceContextConfig: &WorkspaceContextConfig{
			AutoDetectProject:   true,
			ProjectPath:         projectPath,
			CacheTimeout:        10 * time.Minute,
			MaxDetectionTime:    30 * time.Second,
			EnableProjectConfig: true,
		},
	}

	// Create project-aware tool handler
	projectHandler, err := NewProjectAwareToolHandler(client, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create project-aware tool handler: %w", err)
	}

	// Get workspace context
	workspaceContext := projectHandler.GetWorkspaceContext()
	
	// Create project tools handler
	projectToolsHandler := NewProjectToolsHandler(workspaceContext, client)

	return &IntegrationExample{
		projectHandler:     projectHandler,
		workspaceContext:   workspaceContext,
		projectToolsHandler: projectToolsHandler,
		logger:             logger,
	}, nil
}

// DemonstrateProjectAwareness shows how project-aware tools work
func (ie *IntegrationExample) DemonstrateProjectAwareness(ctx context.Context) error {
	ie.logger.Info("=== Project-Aware MCP Tools Integration Demo ===")

	// 1. Demonstrate project detection and context
	if err := ie.demonstrateProjectDetection(ctx); err != nil {
		return fmt.Errorf("project detection demo failed: %w", err)
	}

	// 2. Demonstrate enhanced existing tools
	if err := ie.demonstrateEnhancedTools(ctx); err != nil {
		return fmt.Errorf("enhanced tools demo failed: %w", err)
	}

	// 3. Demonstrate new project-specific tools
	if err := ie.demonstrateProjectSpecificTools(ctx); err != nil {
		return fmt.Errorf("project-specific tools demo failed: %w", err)
	}

	// 4. Demonstrate workspace context features
	if err := ie.demonstrateWorkspaceFeatures(ctx); err != nil {
		return fmt.Errorf("workspace features demo failed: %w", err)
	}

	ie.logger.Info("=== Integration Demo Completed Successfully ===")
	return nil
}

// demonstrateProjectDetection shows project detection capabilities
func (ie *IntegrationExample) demonstrateProjectDetection(ctx context.Context) error {
	ie.logger.Info("--- Demonstrating Project Detection ---")

	if !ie.projectHandler.IsProjectAware() {
		ie.logger.Warn("Project awareness is not enabled or no project detected")
		return nil
	}

	projectCtx := ie.projectHandler.GetProjectContext()
	if projectCtx == nil {
		ie.logger.Warn("No project context available")
		return nil
	}

	ie.logger.WithFields(map[string]interface{}{
		"project_type":      projectCtx.ProjectType,
		"primary_language":  projectCtx.PrimaryLanguage,
		"languages":         projectCtx.Languages,
		"workspace_root":    projectCtx.WorkspaceRoot,
		"project_root":      projectCtx.RootPath,
		"confidence":        projectCtx.Confidence,
		"required_servers":  projectCtx.RequiredServers,
		"is_monorepo":       projectCtx.IsMonorepo,
		"detection_time":    projectCtx.DetectionTime,
	}).Info("Project detection results")

	// Show project statistics
	ie.logger.WithFields(map[string]interface{}{
		"total_files":   projectCtx.ProjectSize.TotalFiles,
		"source_files":  projectCtx.ProjectSize.SourceFiles,
		"test_files":    projectCtx.ProjectSize.TestFiles,
		"config_files":  projectCtx.ProjectSize.ConfigFiles,
		"total_size_mb": float64(projectCtx.ProjectSize.TotalSizeBytes) / (1024 * 1024),
	}).Info("Project size metrics")

	return nil
}

// demonstrateEnhancedTools shows how existing tools are enhanced with project context
func (ie *IntegrationExample) demonstrateEnhancedTools(ctx context.Context) error {
	ie.logger.Info("--- Demonstrating Enhanced Existing Tools ---")

	// Example file in the project (you would replace with actual project files)
	exampleURI := "file://" + ie.workspaceContext.GetProjectRoot() + "/main.go"

	// 1. Enhanced goto definition
	if err := ie.demonstrateEnhancedGotoDefinition(ctx, exampleURI); err != nil {
		ie.logger.WithError(err).Warn("Enhanced goto definition demo failed")
	}

	// 2. Enhanced find references
	if err := ie.demonstrateEnhancedFindReferences(ctx, exampleURI); err != nil {
		ie.logger.WithError(err).Warn("Enhanced find references demo failed")
	}

	// 3. Enhanced workspace symbol search
	if err := ie.demonstrateEnhancedWorkspaceSymbols(ctx); err != nil {
		ie.logger.WithError(err).Warn("Enhanced workspace symbols demo failed")
	}

	return nil
}

// demonstrateProjectSpecificTools shows new project-specific tools
func (ie *IntegrationExample) demonstrateProjectSpecificTools(ctx context.Context) error {
	ie.logger.Info("--- Demonstrating Project-Specific Tools ---")

	// 1. Analyze project structure
	if err := ie.demonstrateAnalyzeProjectStructure(ctx); err != nil {
		ie.logger.WithError(err).Warn("Analyze project structure demo failed")
	}

	// 2. Get project configuration
	if err := ie.demonstrateGetProjectConfig(ctx); err != nil {
		ie.logger.WithError(err).Warn("Get project config demo failed")
	}

	// 3. Validate project structure
	if err := ie.demonstrateValidateProjectStructure(ctx); err != nil {
		ie.logger.WithError(err).Warn("Validate project structure demo failed")
	}

	// 4. Discover workspace dependencies
	if err := ie.demonstrateDiscoverDependencies(ctx); err != nil {
		ie.logger.WithError(err).Warn("Discover dependencies demo failed")
	}

	return nil
}

// demonstrateWorkspaceFeatures shows workspace context features
func (ie *IntegrationExample) demonstrateWorkspaceFeatures(ctx context.Context) error {
	ie.logger.Info("--- Demonstrating Workspace Context Features ---")

	// Show workspace metadata
	metadata := ie.workspaceContext.GetProjectMetadata()
	ie.logger.WithField("metadata", metadata).Info("Workspace metadata")

	// Show file categorization
	exampleFiles := []string{
		"file://" + ie.workspaceContext.GetProjectRoot() + "/main.go",
		"file://" + ie.workspaceContext.GetProjectRoot() + "/config.yaml",
		"file://" + ie.workspaceContext.GetProjectRoot() + "/test/main_test.go",
	}

	for _, file := range exampleFiles {
		ie.logger.WithFields(map[string]interface{}{
			"file":        file,
			"in_project":  ie.workspaceContext.IsFileInProject(file),
			"language":    ie.workspaceContext.GetLanguageForFile(file),
		}).Info("File categorization")
	}

	// Show workspace roots
	ie.logger.WithField("workspace_roots", ie.workspaceContext.GetWorkspaceRoots()).Info("Workspace roots")
	ie.logger.WithField("active_languages", ie.workspaceContext.GetActiveLanguages()).Info("Active languages")
	ie.logger.WithField("enabled_servers", ie.workspaceContext.GetEnabledServers()).Info("Enabled servers")

	return nil
}

// Individual tool demonstrations

func (ie *IntegrationExample) demonstrateEnhancedGotoDefinition(ctx context.Context, uri string) error {
	ie.logger.Info("Testing enhanced goto definition")

	call := ToolCall{
		Name: "goto_definition",
		Arguments: map[string]interface{}{
			"uri":       uri,
			"line":      10,
			"character": 5,
		},
	}

	result, err := ie.projectHandler.CallTool(ctx, call)
	if err != nil {
		return fmt.Errorf("goto definition failed: %w", err)
	}

	ie.logger.WithFields(map[string]interface{}{
		"has_project_metadata": result.Meta != nil && result.Meta.RequestInfo != nil,
		"content_enhanced":     len(result.Content) > 0 && result.Content[0].Annotations != nil,
		"is_error":            result.IsError,
	}).Info("Enhanced goto definition result")

	return nil
}

func (ie *IntegrationExample) demonstrateEnhancedFindReferences(ctx context.Context, uri string) error {
	ie.logger.Info("Testing enhanced find references")

	call := ToolCall{
		Name: "find_references",
		Arguments: map[string]interface{}{
			"uri":                uri,
			"line":               10,
			"character":          5,
			"includeDeclaration": true,
		},
	}

	result, err := ie.projectHandler.CallTool(ctx, call)
	if err != nil {
		return fmt.Errorf("find references failed: %w", err)
	}

	ie.logger.WithFields(map[string]interface{}{
		"has_project_context": result.Meta != nil && result.Meta.RequestInfo != nil,
		"content_categorized": len(result.Content) > 0 && result.Content[0].Data != nil,
		"is_error":           result.IsError,
	}).Info("Enhanced find references result")

	return nil
}

func (ie *IntegrationExample) demonstrateEnhancedWorkspaceSymbols(ctx context.Context) error {
	ie.logger.Info("Testing enhanced workspace symbol search")

	call := ToolCall{
		Name: "search_workspace_symbols",
		Arguments: map[string]interface{}{
			"query": "main",
		},
	}

	result, err := ie.projectHandler.CallTool(ctx, call)
	if err != nil {
		return fmt.Errorf("workspace symbol search failed: %w", err)
	}

	ie.logger.WithFields(map[string]interface{}{
		"has_search_context": result.Meta != nil && result.Meta.RequestInfo != nil,
		"symbols_filtered":   len(result.Content) > 0 && result.Content[0].Annotations != nil,
		"is_error":          result.IsError,
	}).Info("Enhanced workspace symbols result")

	return nil
}

func (ie *IntegrationExample) demonstrateAnalyzeProjectStructure(ctx context.Context) error {
	ie.logger.Info("Testing analyze project structure")

	call := ToolCall{
		Name: "analyze_project_structure",
		Arguments: map[string]interface{}{
			"include_dependencies": true,
			"include_build_info":   true,
			"include_statistics":   true,
		},
	}

	result, err := ie.projectHandler.CallTool(ctx, call)
	if err != nil {
		return fmt.Errorf("analyze project structure failed: %w", err)
	}

	ie.logger.WithFields(map[string]interface{}{
		"has_analysis_data": len(result.Content) > 0 && result.Content[0].Data != nil,
		"content_type":      result.Content[0].Annotations["content_type"],
		"is_error":         result.IsError,
	}).Info("Analyze project structure result")

	return nil
}

func (ie *IntegrationExample) demonstrateGetProjectConfig(ctx context.Context) error {
	ie.logger.Info("Testing get project config")

	call := ToolCall{
		Name: "get_project_config",
		Arguments: map[string]interface{}{
			"include_servers":   true,
			"include_languages": true,
		},
	}

	result, err := ie.projectHandler.CallTool(ctx, call)
	if err != nil {
		return fmt.Errorf("get project config failed: %w", err)
	}

	ie.logger.WithFields(map[string]interface{}{
		"has_config_data": len(result.Content) > 0 && result.Content[0].Data != nil,
		"content_type":    result.Content[0].Annotations["content_type"],
		"is_error":       result.IsError,
	}).Info("Get project config result")

	return nil
}

func (ie *IntegrationExample) demonstrateValidateProjectStructure(ctx context.Context) error {
	ie.logger.Info("Testing validate project structure")

	call := ToolCall{
		Name: "validate_project_structure",
		Arguments: map[string]interface{}{
			"check_dependencies": true,
			"check_build_config": true,
		},
	}

	result, err := ie.projectHandler.CallTool(ctx, call)
	if err != nil {
		return fmt.Errorf("validate project structure failed: %w", err)
	}

	ie.logger.WithFields(map[string]interface{}{
		"has_validation_data": len(result.Content) > 0 && result.Content[0].Data != nil,
		"health_score":        result.Content[0].Annotations["health_score"],
		"is_error":           result.IsError,
	}).Info("Validate project structure result")

	return nil
}

func (ie *IntegrationExample) demonstrateDiscoverDependencies(ctx context.Context) error {
	ie.logger.Info("Testing discover workspace dependencies")

	call := ToolCall{
		Name: "discover_workspace_dependencies",
		Arguments: map[string]interface{}{
			"include_dev_deps":   false,
			"include_transitive": false,
		},
	}

	result, err := ie.projectHandler.CallTool(ctx, call)
	if err != nil {
		return fmt.Errorf("discover dependencies failed: %w", err)
	}

	ie.logger.WithFields(map[string]interface{}{
		"has_dependency_data": len(result.Content) > 0 && result.Content[0].Data != nil,
		"content_type":        result.Content[0].Annotations["content_type"],
		"is_error":           result.IsError,
	}).Info("Discover dependencies result")

	return nil
}

// ConfigurationExample shows how to configure the enhanced MCP system
func ConfigurationExample() *config.GatewayConfig {
	return &config.GatewayConfig{
		Port:                  8080,
		Timeout:               "30s",
		MaxConcurrentRequests: 100,
		ProjectAware:          true, // Enable project awareness
		Servers: []config.ServerConfig{
			{
				Name:        "go-lsp",
				Languages:   []string{"go"},
				Command:     "gopls",
				Args:        []string{},
				Transport:   "stdio",
				RootMarkers: []string{"go.mod", "go.sum"},
			},
			{
				Name:        "python-lsp",
				Languages:   []string{"python"},
				Command:     "python",
				Args:        []string{"-m", "pylsp"},
				Transport:   "stdio",
				RootMarkers: []string{"pyproject.toml", "setup.py"},
			},
			{
				Name:        "typescript-lsp",
				Languages:   []string{"typescript", "javascript"},
				Command:     "typescript-language-server",
				Args:        []string{"--stdio"},
				Transport:   "stdio",
				RootMarkers: []string{"tsconfig.json", "package.json"},
			},
		},
		ProjectContext: &config.ProjectContext{
			ProjectType:   config.ProjectTypeSingle,
			RootDirectory: "/path/to/project",
			Languages: []config.LanguageInfo{
				{
					Language:     "go",
					FilePatterns: []string{"*.go"},
					FileCount:    50,
				},
			},
			RequiredLSPs: []string{"gopls"},
			DetectedAt:   time.Now(),
		},
	}
}

// UsageExample shows typical usage patterns
func UsageExample() {
	// This would be called from main() or a server setup function
	fmt.Println(`
# Enhanced MCP Tools with Project Context Awareness

## Basic Usage

1. **Create Enhanced Tool Handler**
   ` + "```go" + `
   config := &ProjectAwareConfig{
       EnableProjectAware: true,
       AutoDetectProject:  true,
       ProjectPath:        "/path/to/project",
   }
   
   client := NewLSPGatewayClient("http://localhost:8080")
   handler, err := NewProjectAwareToolHandler(client, config)
   ` + "```" + `

2. **Use Enhanced Existing Tools**
   - All existing tools (goto_definition, find_references, etc.) now include project context
   - Results are filtered and enhanced with project-specific information
   - Metadata includes file categorization and project relationships

3. **Use New Project-Specific Tools**
   - analyze_project_structure: Comprehensive project analysis
   - find_project_symbols: Project-scoped symbol search
   - get_project_config: Project configuration retrieval
   - validate_project_structure: Project validation
   - discover_workspace_dependencies: Dependency analysis

4. **Access Project Context**
   ` + "```go" + `
   workspaceCtx := handler.GetWorkspaceContext()
   projectCtx := workspaceCtx.GetProjectContext()
   
   // Check if file belongs to project
   inProject := workspaceCtx.IsFileInProject("file:///path/to/file.go")
   
   // Get file language
   language := workspaceCtx.GetLanguageForFile("file:///path/to/file.go")
   ` + "```" + `

## Features

- **Automatic Project Detection**: Detects Go, Python, TypeScript, Java projects
- **Multi-language Support**: Handles polyglot projects
- **Workspace Context**: Project-aware file categorization
- **Enhanced Results**: All LSP results include project metadata
- **Backward Compatibility**: Works with existing MCP clients
- **Performance Optimized**: Caching and efficient detection

## Integration Points

- Compatible with existing LSP Gateway architecture
- Integrates with internal/project detection system
- Uses existing config system with project-aware extensions
- Maintains MCP protocol compliance
`)
}

// RunIntegrationDemo runs a complete integration demonstration
func RunIntegrationDemo(projectPath string) error {
	log.Println("Starting Enhanced MCP Tools Integration Demo")

	// Create integration example
	ie, err := NewIntegrationExample("http://localhost:8080", projectPath)
	if err != nil {
		return fmt.Errorf("failed to create integration example: %w", err)
	}

	// Run demonstration
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if err := ie.DemonstrateProjectAwareness(ctx); err != nil {
		return fmt.Errorf("integration demo failed: %w", err)
	}

	log.Println("Integration demo completed successfully")
	return nil
}