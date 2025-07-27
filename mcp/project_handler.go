package mcp

import (
	"context"
	"fmt"
	"lsp-gateway/internal/config"
	"lsp-gateway/internal/project"
	"path/filepath"
	"time"
)

// ProjectAwareToolHandler extends ToolHandler with project context awareness
type ProjectAwareToolHandler struct {
	*ToolHandler     // Embed existing ToolHandler
	workspaceContext *WorkspaceContext
	projectConfig    *config.GatewayConfig
	isProjectAware   bool
	logger           *StructuredLogger
}

// ProjectAwareConfig holds configuration for project-aware tool handling
type ProjectAwareConfig struct {
	EnableProjectAware     bool                    `json:"enable_project_aware"`
	AutoDetectProject      bool                    `json:"auto_detect_project"`
	ProjectPath            string                  `json:"project_path,omitempty"`
	WorkspaceContextConfig *WorkspaceContextConfig `json:"workspace_context_config,omitempty"`
	GatewayConfig          *config.GatewayConfig   `json:"gateway_config,omitempty"`
}

// NewProjectAwareToolHandler creates a new project-aware tool handler
func NewProjectAwareToolHandler(client *LSPGatewayClient, config *ProjectAwareConfig) (*ProjectAwareToolHandler, error) {
	if config == nil {
		config = &ProjectAwareConfig{
			EnableProjectAware: true,
			AutoDetectProject:  true,
		}
	}

	// Create base tool handler
	baseHandler := NewToolHandler(client)

	// Create logger
	logger := NewStructuredLogger(&LoggerConfig{
		Level:      LogLevelInfo,
		EnableJSON: true,
	})

	handler := &ProjectAwareToolHandler{
		ToolHandler:    baseHandler,
		isProjectAware: config.EnableProjectAware,
		logger:         logger,
	}

	// Initialize workspace context if project awareness is enabled
	if config.EnableProjectAware {
		wsConfig := config.WorkspaceContextConfig
		if wsConfig == nil {
			wsConfig = DefaultWorkspaceContextConfig()
		}

		if config.ProjectPath != "" {
			wsConfig.ProjectPath = config.ProjectPath
		}

		handler.workspaceContext = NewWorkspaceContext(wsConfig)

		// Initialize workspace context
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := handler.workspaceContext.Initialize(ctx); err != nil {
			handler.logger.WithError(err).Warn("Failed to initialize workspace context")
			// Continue without project awareness rather than failing
			handler.isProjectAware = false
		}
	}

	// Set project configuration if provided
	if config.GatewayConfig != nil {
		handler.projectConfig = config.GatewayConfig
	}

	// Register project-specific tools
	handler.registerProjectTools()

	return handler, nil
}

// IsProjectAware returns whether the handler has project awareness enabled
func (h *ProjectAwareToolHandler) IsProjectAware() bool {
	return h.isProjectAware && h.workspaceContext != nil && h.workspaceContext.IsProjectAware()
}

// GetWorkspaceContext returns the workspace context (may be nil)
func (h *ProjectAwareToolHandler) GetWorkspaceContext() *WorkspaceContext {
	return h.workspaceContext
}

// GetProjectContext returns the project context (may be nil)
func (h *ProjectAwareToolHandler) GetProjectContext() *project.ProjectContext {
	if h.workspaceContext == nil {
		return nil
	}
	return h.workspaceContext.GetProjectContext()
}

// ListTools returns all available tools, including project-specific ones
func (h *ProjectAwareToolHandler) ListTools() []Tool {
	tools := h.ToolHandler.ListTools()

	// Add project-aware metadata to tool descriptions
	if h.IsProjectAware() {
		for i := range tools {
			tools[i].Description = h.enhanceToolDescription(tools[i].Name, tools[i].Description)
		}
	}

	return tools
}

// CallTool executes a tool with project context enhancement
func (h *ProjectAwareToolHandler) CallTool(ctx context.Context, call ToolCall) (*ToolResult, error) {
	startTime := time.Now()

	h.logger.WithFields(map[string]interface{}{
		"tool_name":     call.Name,
		"project_aware": h.IsProjectAware(),
		"has_workspace": h.workspaceContext != nil,
	}).Debug("Calling tool with project awareness")

	// Enhance arguments with project context
	enhancedCall := call
	if h.IsProjectAware() {
		enhancedCall.Arguments = h.workspaceContext.EnhanceToolArgs(call.Name, call.Arguments)

		// Refresh project context if needed
		if err := h.workspaceContext.RefreshProject(ctx); err != nil {
			h.logger.WithError(err).Warn("Failed to refresh project context")
		}
	}

	// Check if this is a project-specific tool
	if h.isProjectSpecificTool(call.Name) {
		return h.handleProjectSpecificTool(ctx, enhancedCall)
	}

	// Call base tool handler
	result, err := h.ToolHandler.CallTool(ctx, enhancedCall)
	if err != nil {
		return result, err
	}

	// Enhance response with project metadata
	if h.IsProjectAware() && result != nil {
		result = h.enhanceToolResult(call.Name, result)
	}

	duration := time.Since(startTime)
	h.logger.WithFields(map[string]interface{}{
		"tool_name": call.Name,
		"duration":  duration,
		"success":   !result.IsError,
	}).Debug("Tool execution completed")

	return result, nil
}

// RefreshProjectContext refreshes the project detection
func (h *ProjectAwareToolHandler) RefreshProjectContext(ctx context.Context) error {
	if !h.isProjectAware || h.workspaceContext == nil {
		return fmt.Errorf("project awareness not enabled")
	}

	return h.workspaceContext.RefreshProject(ctx)
}

// SetProjectPath updates the project path and re-initializes context
func (h *ProjectAwareToolHandler) SetProjectPath(ctx context.Context, path string) error {
	if !h.isProjectAware {
		return fmt.Errorf("project awareness not enabled")
	}

	// Create new workspace context with updated path
	config := DefaultWorkspaceContextConfig()
	config.ProjectPath = path

	h.workspaceContext = NewWorkspaceContext(config)
	return h.workspaceContext.Initialize(ctx)
}

// GetProjectMetadata returns comprehensive project metadata for responses
func (h *ProjectAwareToolHandler) GetProjectMetadata() map[string]interface{} {
	metadata := make(map[string]interface{})

	metadata["project_aware_enabled"] = h.isProjectAware
	metadata["has_workspace_context"] = h.workspaceContext != nil

	if h.IsProjectAware() {
		projectMeta := h.workspaceContext.GetProjectMetadata()
		for k, v := range projectMeta {
			metadata[k] = v
		}
	}

	return metadata
}

// Private methods for project-specific functionality

func (h *ProjectAwareToolHandler) registerProjectTools() {
	// Register new project-specific tools
	analyzeProjectTitle := "Analyze Project Structure"
	analyzeProjectOutputSchema := &map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"content": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"type": map[string]interface{}{"type": "string"},
						"text": map[string]interface{}{"type": "string"},
						"data": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"project_type": map[string]interface{}{"type": "string"},
								"primary_language": map[string]interface{}{"type": "string"},
								"languages": map[string]interface{}{
									"type": "array",
									"items": map[string]interface{}{"type": "string"},
								},
								"root_path": map[string]interface{}{"type": "string"},
								"confidence": map[string]interface{}{"type": "number"},
								"is_valid": map[string]interface{}{"type": "boolean"},
							},
						},
					},
				},
			},
			"meta": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"timestamp": map[string]interface{}{"type": "string"},
					"lspMethod": map[string]interface{}{"type": "string"},
				},
			},
		},
	}
	h.Tools["analyze_project_structure"] = Tool{
		Name:         "analyze_project_structure",
		Title:        &analyzeProjectTitle,
		Description:  "Analyze the structure and characteristics of the current project",
		OutputSchema: analyzeProjectOutputSchema,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"include_dependencies": map[string]interface{}{
					"type":        "boolean",
					"description": "Include dependency analysis",
					"default":     true,
				},
				"include_build_info": map[string]interface{}{
					"type":        "boolean",
					"description": "Include build system information",
					"default":     true,
				},
				"include_statistics": map[string]interface{}{
					"type":        "boolean",
					"description": "Include project statistics",
					"default":     true,
				},
			},
		},
	}

	findProjectSymbolsTitle := "Find Project Symbols"
	findProjectSymbolsOutputSchema := &map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"content": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"type": map[string]interface{}{"type": "string"},
						"text": map[string]interface{}{"type": "string"},
						"data": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"name": map[string]interface{}{"type": "string"},
									"kind": map[string]interface{}{"type": "integer"},
									"location": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"uri": map[string]interface{}{"type": "string"},
											"range": map[string]interface{}{
												"type": "object",
												"properties": map[string]interface{}{
													"start": map[string]interface{}{
														"type": "object",
														"properties": map[string]interface{}{
															"line": map[string]interface{}{"type": "integer"},
															"character": map[string]interface{}{"type": "integer"},
														},
													},
													"end": map[string]interface{}{
														"type": "object",
														"properties": map[string]interface{}{
															"line": map[string]interface{}{"type": "integer"},
															"character": map[string]interface{}{"type": "integer"},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			"meta": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"timestamp": map[string]interface{}{"type": "string"},
					"lspMethod": map[string]interface{}{"type": "string"},
				},
			},
		},
	}
	h.Tools["find_project_symbols"] = Tool{
		Name:         "find_project_symbols",
		Title:        &findProjectSymbolsTitle,
		Description:  "Search for symbols across the entire project with project-aware scoping",
		OutputSchema: findProjectSymbolsOutputSchema,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Symbol search query",
				},
				"language_filter": map[string]interface{}{
					"type":        "string",
					"description": "Filter by specific language (optional)",
				},
				"scope": map[string]interface{}{
					"type":        "string",
					"description": "Search scope: 'project', 'workspace', or 'all'",
					"default":     "project",
				},
			},
			"required": []string{"query"},
		},
	}

	getProjectConfigTitle := "Get Project Configuration"
	getProjectConfigOutputSchema := &map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"content": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"type": map[string]interface{}{"type": "string"},
						"text": map[string]interface{}{"type": "string"},
						"data": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"project_context": map[string]interface{}{"type": "object"},
								"config_context": map[string]interface{}{"type": "object"},
								"project_config": map[string]interface{}{"type": "object"},
								"workspace_roots": map[string]interface{}{
									"type": "array",
									"items": map[string]interface{}{"type": "string"},
								},
								"active_languages": map[string]interface{}{
									"type": "array",
									"items": map[string]interface{}{"type": "string"},
								},
								"enabled_servers": map[string]interface{}{
									"type": "array",
									"items": map[string]interface{}{"type": "string"},
								},
							},
						},
					},
				},
			},
			"meta": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"timestamp": map[string]interface{}{"type": "string"},
					"lspMethod": map[string]interface{}{"type": "string"},
				},
			},
		},
	}
	h.Tools["get_project_config"] = Tool{
		Name:         "get_project_config",
		Title:        &getProjectConfigTitle,
		Description:  "Retrieve project configuration and metadata",
		OutputSchema: getProjectConfigOutputSchema,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"include_servers": map[string]interface{}{
					"type":        "boolean",
					"description": "Include LSP server configuration",
					"default":     true,
				},
				"include_languages": map[string]interface{}{
					"type":        "boolean",
					"description": "Include language information",
					"default":     true,
				},
			},
		},
	}

	validateProjectTitle := "Validate Project Structure"
	validateProjectOutputSchema := &map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"content": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"type": map[string]interface{}{"type": "string"},
						"text": map[string]interface{}{"type": "string"},
						"data": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"is_valid": map[string]interface{}{"type": "boolean"},
								"validation_errors": map[string]interface{}{
									"type": "array",
									"items": map[string]interface{}{"type": "string"},
								},
								"validation_warnings": map[string]interface{}{
									"type": "array",
									"items": map[string]interface{}{"type": "string"},
								},
								"marker_files": map[string]interface{}{
									"type": "array",
									"items": map[string]interface{}{"type": "string"},
								},
								"required_servers": map[string]interface{}{
									"type": "array",
									"items": map[string]interface{}{"type": "string"},
								},
							},
						},
					},
				},
			},
			"meta": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"timestamp": map[string]interface{}{"type": "string"},
					"lspMethod": map[string]interface{}{"type": "string"},
				},
			},
		},
	}
	h.Tools["validate_project_structure"] = Tool{
		Name:         "validate_project_structure",
		Title:        &validateProjectTitle,
		Description:  "Validate project structure and configuration",
		OutputSchema: validateProjectOutputSchema,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"check_dependencies": map[string]interface{}{
					"type":        "boolean",
					"description": "Check dependency consistency",
					"default":     true,
				},
				"check_build_config": map[string]interface{}{
					"type":        "boolean",
					"description": "Validate build configuration",
					"default":     true,
				},
			},
		},
	}

	discoverDepsTitle := "Discover Workspace Dependencies"
	discoverDepsOutputSchema := &map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"content": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"type": map[string]interface{}{"type": "string"},
						"text": map[string]interface{}{"type": "string"},
						"data": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"dependencies": map[string]interface{}{"type": "object"},
								"dev_dependencies": map[string]interface{}{"type": "object"},
								"build_files": map[string]interface{}{
									"type": "array",
									"items": map[string]interface{}{"type": "string"},
								},
								"config_files": map[string]interface{}{
									"type": "array",
									"items": map[string]interface{}{"type": "string"},
								},
							},
						},
					},
				},
			},
			"meta": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"timestamp": map[string]interface{}{"type": "string"},
					"lspMethod": map[string]interface{}{"type": "string"},
				},
			},
		},
	}
	h.Tools["discover_workspace_dependencies"] = Tool{
		Name:         "discover_workspace_dependencies",
		Title:        &discoverDepsTitle,
		Description:  "Analyze project dependencies and their relationships",
		OutputSchema: discoverDepsOutputSchema,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"include_dev_deps": map[string]interface{}{
					"type":        "boolean",
					"description": "Include development dependencies",
					"default":     false,
				},
				"include_transitive": map[string]interface{}{
					"type":        "boolean",
					"description": "Include transitive dependencies",
					"default":     false,
				},
			},
		},
	}
}

func (h *ProjectAwareToolHandler) isProjectSpecificTool(toolName string) bool {
	projectTools := map[string]bool{
		"analyze_project_structure":       true,
		"find_project_symbols":            true,
		"get_project_config":              true,
		"validate_project_structure":      true,
		"discover_workspace_dependencies": true,
	}
	return projectTools[toolName]
}

func (h *ProjectAwareToolHandler) handleProjectSpecificTool(ctx context.Context, call ToolCall) (*ToolResult, error) {
	if !h.IsProjectAware() {
		return &ToolResult{
			Content: []ContentBlock{{
				Type: "text",
				Text: fmt.Sprintf("Tool %s requires project awareness, but no project context is available", call.Name),
			}},
			IsError: true,
			Error: &StructuredError{
				Code:    MCPErrorUnsupportedFeature,
				Message: "Project-specific tool requires project context",
				Details: "Project detection failed or is disabled",
			},
		}, nil
	}

	switch call.Name {
	case "analyze_project_structure":
		return h.handleAnalyzeProjectStructure(ctx, call.Arguments)
	case "find_project_symbols":
		return h.handleFindProjectSymbols(ctx, call.Arguments)
	case "get_project_config":
		return h.handleGetProjectConfig(ctx, call.Arguments)
	case "validate_project_structure":
		return h.handleValidateProjectStructure(ctx, call.Arguments)
	case "discover_workspace_dependencies":
		return h.handleDiscoverWorkspaceDependencies(ctx, call.Arguments)
	default:
		return &ToolResult{
			Content: []ContentBlock{{
				Type: "text",
				Text: fmt.Sprintf("Unknown project-specific tool: %s", call.Name),
			}},
			IsError: true,
		}, nil
	}
}

func (h *ProjectAwareToolHandler) enhanceToolDescription(toolName, originalDesc string) string {
	if !h.IsProjectAware() {
		return originalDesc
	}

	projectCtx := h.workspaceContext.GetProjectContext()
	if projectCtx == nil {
		return originalDesc
	}

	projectInfo := fmt.Sprintf(" (Project: %s, Type: %s)",
		filepath.Base(projectCtx.RootPath), projectCtx.ProjectType)

	return originalDesc + projectInfo
}

func (h *ProjectAwareToolHandler) enhanceToolResult(toolName string, result *ToolResult) *ToolResult {
	if result == nil || !h.IsProjectAware() {
		return result
	}

	// Add project metadata to response
	if result.Meta == nil {
		result.Meta = &ResponseMetadata{}
	}

	if result.Meta.RequestInfo == nil {
		result.Meta.RequestInfo = make(map[string]interface{})
	}

	// Add project context to metadata
	projectMeta := h.workspaceContext.GetProjectMetadata()
	result.Meta.RequestInfo["project_context"] = projectMeta

	return result
}

// Project-specific tool implementations (basic implementations)

func (h *ProjectAwareToolHandler) handleAnalyzeProjectStructure(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	projectCtx := h.workspaceContext.GetProjectContext()

	analysis := map[string]interface{}{
		"project_type":     projectCtx.ProjectType,
		"primary_language": projectCtx.PrimaryLanguage,
		"languages":        projectCtx.Languages,
		"root_path":        projectCtx.RootPath,
		"workspace_root":   projectCtx.WorkspaceRoot,
		"confidence":       projectCtx.Confidence,
		"is_valid":         projectCtx.IsValid,
		"size_metrics":     projectCtx.ProjectSize,
	}

	// Include dependencies if requested
	if includeDeps, ok := args["include_dependencies"].(bool); ok && includeDeps {
		analysis["dependencies"] = projectCtx.Dependencies
		analysis["dev_dependencies"] = projectCtx.DevDependencies
	}

	// Include build info if requested
	if includeBuild, ok := args["include_build_info"].(bool); ok && includeBuild {
		analysis["build_system"] = projectCtx.BuildSystem
		analysis["build_targets"] = projectCtx.BuildTargets
		analysis["package_manager"] = projectCtx.PackageManager
	}

	return &ToolResult{
		Content: []ContentBlock{{
			Type: "text",
			Text: fmt.Sprintf("Project Analysis Complete:\n%+v", analysis),
			Data: analysis,
		}},
		Meta: &ResponseMetadata{
			Timestamp: time.Now().Format(time.RFC3339),
			LSPMethod: "project/analyze",
		},
	}, nil
}

func (h *ProjectAwareToolHandler) handleFindProjectSymbols(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	query, ok := args["query"].(string)
	if !ok {
		return &ToolResult{
			Content: []ContentBlock{{
				Type: "text",
				Text: "Missing required 'query' parameter",
			}},
			IsError: true,
		}, nil
	}

	// Enhance the workspace symbol search with project context
	enhancedArgs := map[string]interface{}{
		"query": query,
	}

	// Delegate to enhanced workspace symbol search
	return h.handleSearchWorkspaceSymbols(ctx, enhancedArgs)
}

func (h *ProjectAwareToolHandler) handleGetProjectConfig(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	projectCtx := h.workspaceContext.GetProjectContext()
	configCtx := h.workspaceContext.GetConfigContext()
	projectConfig := h.workspaceContext.GetProjectConfig()

	config := map[string]interface{}{
		"project_context":  projectCtx,
		"config_context":   configCtx,
		"project_config":   projectConfig,
		"workspace_roots":  h.workspaceContext.GetWorkspaceRoots(),
		"active_languages": h.workspaceContext.GetActiveLanguages(),
		"enabled_servers":  h.workspaceContext.GetEnabledServers(),
	}

	return &ToolResult{
		Content: []ContentBlock{{
			Type: "text",
			Text: fmt.Sprintf("Project Configuration:\n%+v", config),
			Data: config,
		}},
		Meta: &ResponseMetadata{
			Timestamp: time.Now().Format(time.RFC3339),
			LSPMethod: "project/config",
		},
	}, nil
}

func (h *ProjectAwareToolHandler) handleValidateProjectStructure(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	projectCtx := h.workspaceContext.GetProjectContext()

	validation := map[string]interface{}{
		"is_valid":            projectCtx.IsValid,
		"validation_errors":   projectCtx.ValidationErrors,
		"validation_warnings": projectCtx.ValidationWarnings,
		"marker_files":        projectCtx.MarkerFiles,
		"required_servers":    projectCtx.RequiredServers,
	}

	return &ToolResult{
		Content: []ContentBlock{{
			Type: "text",
			Text: fmt.Sprintf("Project Validation Results:\n%+v", validation),
			Data: validation,
		}},
		Meta: &ResponseMetadata{
			Timestamp: time.Now().Format(time.RFC3339),
			LSPMethod: "project/validate",
		},
	}, nil
}

func (h *ProjectAwareToolHandler) handleDiscoverWorkspaceDependencies(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	projectCtx := h.workspaceContext.GetProjectContext()

	dependencies := map[string]interface{}{
		"dependencies":     projectCtx.Dependencies,
		"dev_dependencies": projectCtx.DevDependencies,
		"build_files":      projectCtx.BuildFiles,
		"config_files":     projectCtx.ConfigFiles,
	}

	return &ToolResult{
		Content: []ContentBlock{{
			Type: "text",
			Text: fmt.Sprintf("Workspace Dependencies:\n%+v", dependencies),
			Data: dependencies,
		}},
		Meta: &ResponseMetadata{
			Timestamp: time.Now().Format(time.RFC3339),
			LSPMethod: "project/dependencies",
		},
	}, nil
}
