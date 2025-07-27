package project

import (
	"context"
	"errors"
	"fmt"
	"lsp-gateway/internal/config"
	"lsp-gateway/internal/project/types"
	"lsp-gateway/internal/setup"
	"path/filepath"
	"strings"
	"time"
)

// ProjectConfigGenerator defines the interface for generating project-specific LSP Gateway configurations
type ProjectConfigGenerator interface {
	// GenerateFromProject creates a complete LSP Gateway configuration tailored to the detected project
	GenerateFromProject(ctx context.Context, projectContext *ProjectContext) (*ProjectConfigGenerationResult, error)

	// ApplyLanguageOptimizations applies language-specific optimizations to server configurations
	ApplyLanguageOptimizations(ctx context.Context, servers []config.ServerConfig, projectContext *ProjectContext) ([]config.ServerConfig, error)

	// GenerateServerOverrides creates project-specific server overrides based on project characteristics
	GenerateServerOverrides(ctx context.Context, projectContext *ProjectContext) ([]config.ProjectServerOverride, error)

	// FilterGlobalServers filters the global server list to only include servers required by the project
	FilterGlobalServers(ctx context.Context, globalServers []config.ServerConfig, projectContext *ProjectContext) ([]config.ServerConfig, error)

	// ValidateProjectConfig validates the generated project configuration for completeness and compatibility
	ValidateProjectConfig(ctx context.Context, projectConfig *config.ProjectConfig, projectContext *ProjectContext) (*ProjectConfigValidationResult, error)

	// Configuration methods
	SetLogger(logger *setup.SetupLogger)
	SetServerRegistry(registry setup.ServerRegistry)
	SetServerVerifier(verifier setup.ServerVerifier)
}

// ProjectConfigGeneratorImpl implements sophisticated project-aware configuration generation
type ProjectConfigGeneratorImpl struct {
	logger         *setup.SetupLogger
	serverRegistry setup.ServerRegistry
	serverVerifier setup.ServerVerifier
	templates      map[string]*ProjectConfigTemplate
	optimizers     map[string]LanguageOptimizer
}

// ProjectConfigTemplate contains language-specific configuration templates
type ProjectConfigTemplate struct {
	Language          string                 `json:"language"`
	ServerName        string                 `json:"server_name"`
	DefaultArgs       []string               `json:"default_args"`
	DefaultSettings   map[string]interface{} `json:"default_settings"`
	RootMarkers       []string               `json:"root_markers"`
	OptimizationRules []OptimizationRule     `json:"optimization_rules"`
	PerformanceHints  *PerformanceHints      `json:"performance_hints,omitempty"`
}

// OptimizationRule defines conditional optimization rules
type OptimizationRule struct {
	Condition string                 `json:"condition"`
	Settings  map[string]interface{} `json:"settings"`
	Args      []string               `json:"args"`
	Priority  int                    `json:"priority"`
}

// PerformanceHints contains performance tuning hints for specific project characteristics
type PerformanceHints struct {
	LargeProjectThreshold int                    `json:"large_project_threshold"`
	LargeProjectSettings  map[string]interface{} `json:"large_project_settings"`
	MemorySettings        map[string]interface{} `json:"memory_settings"`
	IndexingSettings      map[string]interface{} `json:"indexing_settings"`
}

// LanguageOptimizer defines language-specific optimization logic
type LanguageOptimizer interface {
	OptimizeForProject(ctx context.Context, serverConfig *config.ServerConfig, projectContext *ProjectContext) error
	GetPerformanceSettings(projectSize ProjectSize) map[string]interface{}
	GetWorkspaceSettings(projectContext *ProjectContext) map[string]interface{}
}

// ProjectConfigGenerationResult contains the complete project configuration generation result
type ProjectConfigGenerationResult struct {
	GatewayConfig        *config.GatewayConfig  `json:"gateway_config"`
	ProjectConfig        *config.ProjectConfig  `json:"project_config"`
	ServersGenerated     int                    `json:"servers_generated"`
	ServersSkipped       int                    `json:"servers_skipped"`
	OptimizationsApplied int                    `json:"optimizations_applied"`
	GeneratedAt          time.Time              `json:"generated_at"`
	Duration             time.Duration          `json:"duration"`
	Messages             []string               `json:"messages"`
	Warnings             []string               `json:"warnings"`
	Issues               []string               `json:"issues"`
	Metadata             map[string]interface{} `json:"metadata"`
}

// ProjectConfigValidationResult contains project configuration validation results
type ProjectConfigValidationResult struct {
	IsValid                 bool                   `json:"is_valid"`
	ValidationErrors        []string               `json:"validation_errors,omitempty"`
	ValidationWarnings      []string               `json:"validation_warnings,omitempty"`
	CompatibilityIssues     []string               `json:"compatibility_issues,omitempty"`
	PerformanceWarnings     []string               `json:"performance_warnings,omitempty"`
	OptimizationSuggestions []string               `json:"optimization_suggestions,omitempty"`
	ServersValidated        int                    `json:"servers_validated"`
	ValidatedAt             time.Time              `json:"validated_at"`
	Duration                time.Duration          `json:"duration"`
	Metadata                map[string]interface{} `json:"metadata"`
}

// NewProjectConfigGenerator creates a new project configuration generator
func NewProjectConfigGenerator(logger *setup.SetupLogger, registry setup.ServerRegistry, verifier setup.ServerVerifier) *ProjectConfigGeneratorImpl {
	generator := &ProjectConfigGeneratorImpl{
		logger:         logger,
		serverRegistry: registry,
		serverVerifier: verifier,
		templates:      make(map[string]*ProjectConfigTemplate),
		optimizers:     make(map[string]LanguageOptimizer),
	}

	generator.initializeTemplates()
	generator.initializeOptimizers()

	return generator
}

// GenerateFromProject generates a complete project-specific configuration
func (g *ProjectConfigGeneratorImpl) GenerateFromProject(ctx context.Context, projectContext *ProjectContext) (*ProjectConfigGenerationResult, error) {
	startTime := time.Now()

	// Validate input parameters
	if projectContext == nil {
		return nil, errors.New("project context cannot be nil")
	}

	g.logger.WithOperation("generate-project-config").WithFields(map[string]interface{}{
		"project_type": projectContext.ProjectType,
		"project_path": projectContext.RootPath,
		"languages":    projectContext.Languages,
	}).Info("Starting project-specific configuration generation")

	result := &ProjectConfigGenerationResult{
		ServersGenerated:     0,
		ServersSkipped:       0,
		OptimizationsApplied: 0,
		GeneratedAt:          startTime,
		Messages:             []string{},
		Warnings:             []string{},
		Issues:               []string{},
		Metadata:             make(map[string]interface{}),
	}

	// Create base gateway configuration
	gatewayConfig := &config.GatewayConfig{
		Port:                  8080,
		Timeout:               "30s",
		MaxConcurrentRequests: 100,
		ProjectAware:          true,
		Servers:               []config.ServerConfig{},
	}

	// Set project context
	gatewayConfig.ProjectContext = g.convertToConfigProjectContext(projectContext)

	// Generate project-specific server configurations
	serverConfigs, err := g.generateProjectServers(ctx, projectContext, result)
	if err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("Failed to generate project servers: %v", err))
		result.Duration = time.Since(startTime)
		return result, setup.WrapWithContext(err, "generate-project-servers", "project-config-generator", map[string]interface{}{
			"project_type": projectContext.ProjectType,
		})
	}

	// Apply language-specific optimizations
	optimizedServers, err := g.ApplyLanguageOptimizations(ctx, serverConfigs, projectContext)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to apply some optimizations: %v", err))
		optimizedServers = serverConfigs // Use unoptimized servers as fallback
	} else {
		result.OptimizationsApplied = len(optimizedServers)
	}

	gatewayConfig.Servers = optimizedServers
	result.ServersGenerated = len(optimizedServers)

	// Generate project configuration
	projectConfig, err := g.generateProjectConfig(ctx, projectContext, result)
	if err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("Failed to generate project config: %v", err))
		result.Duration = time.Since(startTime)
		return result, setup.WrapWithContext(err, "generate-project-config", "project-config-generator", nil)
	}

	gatewayConfig.ProjectConfig = projectConfig

	// Apply project overrides
	if err := gatewayConfig.ApplyProjectOverrides(); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to apply project overrides: %v", err))
	}

	// Validate generated configuration
	if err := gatewayConfig.Validate(); err != nil {
		// If no servers were configured, this might be expected for empty projects or skipped servers
		if len(gatewayConfig.Servers) == 0 {
			if result.ServersSkipped > 0 {
				result.Warnings = append(result.Warnings, "No servers were configured due to installation issues")
			} else if len(projectContext.RequiredServers) == 0 {
				result.Warnings = append(result.Warnings, "No servers were configured for project with no detected languages")
			} else {
				result.Warnings = append(result.Warnings, "No servers were configured")
			}
			result.Issues = append(result.Issues, fmt.Sprintf("Generated configuration is invalid: %v", err))
		} else {
			// For other validation errors, fail as usual
			result.Issues = append(result.Issues, fmt.Sprintf("Generated configuration is invalid: %v", err))
			result.Duration = time.Since(startTime)
			return result, fmt.Errorf("configuration validation failed: %w", err)
		}
	}

	result.GatewayConfig = gatewayConfig
	result.ProjectConfig = projectConfig
	result.Duration = time.Since(startTime)

	// Populate metadata
	result.Metadata["generation_method"] = "project_aware"
	result.Metadata["project_type"] = projectContext.ProjectType
	result.Metadata["languages_count"] = len(projectContext.Languages)
	result.Metadata["project_size"] = projectContext.ProjectSize
	result.Metadata["is_monorepo"] = projectContext.IsMonorepo
	result.Metadata["optimization_level"] = g.getOptimizationLevel(projectContext)

	g.logger.UserSuccess(fmt.Sprintf("Project configuration generated successfully: %d servers configured for %s project",
		result.ServersGenerated, projectContext.ProjectType))

	return result, nil
}

// ApplyLanguageOptimizations applies language-specific optimizations
func (g *ProjectConfigGeneratorImpl) ApplyLanguageOptimizations(ctx context.Context, servers []config.ServerConfig, projectContext *ProjectContext) ([]config.ServerConfig, error) {
	optimizedServers := make([]config.ServerConfig, len(servers))
	copy(optimizedServers, servers)

	for i := range optimizedServers {
		server := &optimizedServers[i]

		// Apply language-specific optimizations
		for _, lang := range server.Languages {
			if optimizer, exists := g.optimizers[lang]; exists {
				if err := optimizer.OptimizeForProject(ctx, server, projectContext); err != nil {
					g.logger.WithError(err).WithFields(map[string]interface{}{
						"server":   server.Name,
						"language": lang,
					}).Warn("Failed to apply language optimization")
					continue
				}

				g.logger.WithFields(map[string]interface{}{
					"server":   server.Name,
					"language": lang,
				}).Debug("Applied language-specific optimizations")
			}
		}

		// Apply template-based optimizations
		if err := g.applyTemplateOptimizations(ctx, server, projectContext); err != nil {
			g.logger.WithError(err).WithField("server", server.Name).Warn("Failed to apply template optimizations")
		}

		// Apply project size optimizations
		g.applyProjectSizeOptimizations(server, projectContext)
	}

	return optimizedServers, nil
}

// GenerateServerOverrides creates project-specific server overrides
func (g *ProjectConfigGeneratorImpl) GenerateServerOverrides(ctx context.Context, projectContext *ProjectContext) ([]config.ProjectServerOverride, error) {
	var overrides []config.ProjectServerOverride

	// Process each required server
	for _, serverName := range projectContext.RequiredServers {
		serverDef, err := g.serverRegistry.GetServer(serverName)
		if err != nil {
			g.logger.WithError(err).WithField("server", serverName).Debug("Server not found in registry")
			continue
		}

		override := config.ProjectServerOverride{
			Name: serverDef.Name,
		}

		// Generate language-specific overrides
		for _, lang := range projectContext.Languages {
			if g.containsLanguage(serverDef.Languages, lang) {
				if template, exists := g.templates[lang]; exists {
					// Apply template settings
					if len(template.DefaultArgs) > 0 {
						override.Args = append(override.Args, template.DefaultArgs...)
					}

					if template.DefaultSettings != nil {
						if override.Settings == nil {
							override.Settings = make(map[string]interface{})
						}
						for key, value := range template.DefaultSettings {
							override.Settings[key] = value
						}
					}

					// Apply optimization rules
					g.applyOptimizationRules(template.OptimizationRules, &override, projectContext)
				}
			}
		}

		// Only add override if it has meaningful changes
		if len(override.Args) > 0 || len(override.Settings) > 0 || override.Transport != "" {
			overrides = append(overrides, override)
		}
	}

	return overrides, nil
}

// FilterGlobalServers filters global servers to project requirements
func (g *ProjectConfigGeneratorImpl) FilterGlobalServers(ctx context.Context, globalServers []config.ServerConfig, projectContext *ProjectContext) ([]config.ServerConfig, error) {
	var filteredServers []config.ServerConfig
	requiredServerMap := make(map[string]bool)

	// Build map of required servers
	for _, serverName := range projectContext.RequiredServers {
		requiredServerMap[serverName] = true
	}

	// Filter servers based on project languages and requirements
	for _, server := range globalServers {
		shouldInclude := false

		// Include if explicitly required
		if requiredServerMap[server.Name] {
			shouldInclude = true
		} else {
			// Include if server supports any project language
			for _, serverLang := range server.Languages {
				if projectContext.HasLanguage(serverLang) {
					shouldInclude = true
					break
				}
			}
		}

		if shouldInclude {
			filteredServers = append(filteredServers, server)
		}
	}

	return filteredServers, nil
}

// ValidateProjectConfig validates the generated project configuration
func (g *ProjectConfigGeneratorImpl) ValidateProjectConfig(ctx context.Context, projectConfig *config.ProjectConfig, projectContext *ProjectContext) (*ProjectConfigValidationResult, error) {
	startTime := time.Now()

	result := &ProjectConfigValidationResult{
		IsValid:                 true,
		ValidationErrors:        []string{},
		ValidationWarnings:      []string{},
		CompatibilityIssues:     []string{},
		PerformanceWarnings:     []string{},
		OptimizationSuggestions: []string{},
		ServersValidated:        0,
		ValidatedAt:             startTime,
		Metadata:                make(map[string]interface{}),
	}

	// Basic validation
	if err := projectConfig.Validate(); err != nil {
		result.IsValid = false
		result.ValidationErrors = append(result.ValidationErrors, fmt.Sprintf("Basic validation failed: %v", err))
	}

	// Validate server overrides
	for _, override := range projectConfig.ServerOverrides {
		result.ServersValidated++

		// Check if server exists in registry
		if _, err := g.serverRegistry.GetServer(override.Name); err != nil {
			result.CompatibilityIssues = append(result.CompatibilityIssues,
				fmt.Sprintf("Server %s not found in registry", override.Name))
		}

		// Verify server if verifier is available
		if g.serverVerifier != nil {
			if verResult, err := g.serverVerifier.VerifyServer(override.Name); err == nil {
				if !verResult.Installed {
					result.ValidationWarnings = append(result.ValidationWarnings,
						fmt.Sprintf("Server %s is not installed", override.Name))
				}
				if !verResult.Compatible {
					result.CompatibilityIssues = append(result.CompatibilityIssues,
						fmt.Sprintf("Server %s may not be compatible", override.Name))
				}
			}
		}
	}

	// Validate project-language server alignment
	g.validateProjectLanguageAlignment(projectContext, projectConfig, result)

	// Performance validation
	g.validatePerformanceSettings(projectContext, projectConfig, result)

	// Generate optimization suggestions
	g.generateOptimizationSuggestions(projectContext, projectConfig, result)

	result.Duration = time.Since(startTime)
	result.Metadata["validation_type"] = "project_aware"
	result.Metadata["project_type"] = projectContext.ProjectType
	result.Metadata["servers_validated"] = result.ServersValidated

	return result, nil
}

// Helper methods

// generateProjectServers generates server configurations for the project
func (g *ProjectConfigGeneratorImpl) generateProjectServers(ctx context.Context, projectContext *ProjectContext, result *ProjectConfigGenerationResult) ([]config.ServerConfig, error) {
	var servers []config.ServerConfig

	// Get servers for each language
	processedServers := make(map[string]bool)

	for _, lang := range projectContext.Languages {
		// Defensive nil check for serverRegistry to prevent panic
		if g.serverRegistry == nil {
			return nil, fmt.Errorf("server registry is not initialized - cannot retrieve servers for language %s (ensure SetServerRegistry was called during initialization)", lang)
		}

		serverDefs := g.serverRegistry.GetServersByRuntime(lang)

		for _, serverDef := range serverDefs {
			if processedServers[serverDef.Name] {
				continue
			}
			processedServers[serverDef.Name] = true

			// Verify server installation
			if g.serverVerifier != nil {
				// Additional defensive nil check for serverVerifier to prevent race conditions
				if g.serverVerifier == nil {
					result.Issues = append(result.Issues, fmt.Sprintf("Server verifier became nil during verification of %s (potential race condition or concurrent modification)", serverDef.Name))
					result.ServersSkipped++
					continue
				}

				verResult, err := g.serverVerifier.VerifyServer(serverDef.Name)
				if err != nil {
					result.Issues = append(result.Issues, fmt.Sprintf("Failed to verify server %s: %v", serverDef.Name, err))
					result.ServersSkipped++
					continue
				}

				if !verResult.Installed {
					result.Warnings = append(result.Warnings, fmt.Sprintf("Server %s is not installed", serverDef.Name))
					result.ServersSkipped++
					continue
				}
			}

			// Create server configuration
			serverConfig := config.ServerConfig{
				Name:      fmt.Sprintf("%s-project", serverDef.Name),
				Languages: serverDef.Languages,
				Command:   serverDef.Name,
				Args:      []string{},
				Transport: config.DefaultTransport,
				Settings:  make(map[string]interface{}),
			}

			// Apply default configuration
			if serverDef.DefaultConfig != nil {
				if cmd, ok := serverDef.DefaultConfig["command"].(string); ok && cmd != "" {
					serverConfig.Command = cmd
				}
				if transport, ok := serverDef.DefaultConfig["transport"].(string); ok && transport != "" {
					serverConfig.Transport = transport
				}
				if args, ok := serverDef.DefaultConfig["args"].([]interface{}); ok {
					for _, arg := range args {
						if argStr, ok := arg.(string); ok {
							serverConfig.Args = append(serverConfig.Args, argStr)
						}
					}
				}
			}

			// Add root markers based on project
			serverConfig.RootMarkers = g.getRootMarkersForProject(projectContext, lang)

			servers = append(servers, serverConfig)
			result.Messages = append(result.Messages,
				fmt.Sprintf("Generated configuration for %s (%s)", serverDef.DisplayName, lang))
		}
	}

	return servers, nil
}

// generateProjectConfig creates the project configuration
func (g *ProjectConfigGeneratorImpl) generateProjectConfig(ctx context.Context, projectContext *ProjectContext, result *ProjectConfigGenerationResult) (*config.ProjectConfig, error) {
	projectID := g.generateProjectID(projectContext)

	projectConfig := config.NewProjectConfig(projectID, projectContext.RootPath)
	projectConfig.Name = projectContext.DisplayName
	if projectConfig.Name == "" {
		projectConfig.Name = filepath.Base(projectContext.RootPath)
	}

	// Generate server overrides
	overrides, err := g.GenerateServerOverrides(ctx, projectContext)
	if err != nil {
		return nil, fmt.Errorf("failed to generate server overrides: %w", err)
	}
	projectConfig.ServerOverrides = overrides

	// Set enabled servers
	projectConfig.EnabledServers = projectContext.RequiredServers

	// Apply optimization settings
	projectConfig.Optimizations = g.generateOptimizationSettings(projectContext)

	return projectConfig, nil
}

// convertToConfigProjectContext converts ProjectContext to config.ProjectContext
func (g *ProjectConfigGeneratorImpl) convertToConfigProjectContext(projectContext *ProjectContext) *config.ProjectContext {
	var languages []config.LanguageInfo
	for _, lang := range projectContext.Languages {
		// Map detected language to LSP server language for validation compatibility
		mappedLanguage := g.mapDetectedLanguageToLSPLanguage(lang)
		
		langInfo := config.LanguageInfo{
			Language:     mappedLanguage,
			FilePatterns: []string{fmt.Sprintf("*.%s", lang)}, // Basic pattern based on original language
			FileCount:    1,                                   // Placeholder
		}
		languages = append(languages, langInfo)
	}

	// Map required server names to match generated server names (add -project suffix)
	mappedRequiredLSPs := make([]string, len(projectContext.RequiredServers))
	for i, serverName := range projectContext.RequiredServers {
		mappedRequiredLSPs[i] = fmt.Sprintf("%s-project", serverName)
	}
	
	return &config.ProjectContext{
		ProjectType:   g.mapProjectType(projectContext.ProjectType),
		RootDirectory: projectContext.RootPath,
		WorkspaceRoot: projectContext.WorkspaceRoot,
		Languages:     languages,
		RequiredLSPs:  mappedRequiredLSPs,
		DetectedAt:    projectContext.DetectedAt,
		Metadata:      projectContext.Metadata,
	}
}

// mapDetectedLanguageToLSPLanguage maps project detection language names to LSP server language names
func (g *ProjectConfigGeneratorImpl) mapDetectedLanguageToLSPLanguage(detectedLanguage string) string {
	languageMap := map[string]string{
		"nodejs":     "javascript", // Node.js projects map to javascript
		"javascript": "javascript", // JavaScript projects (no change)
		"typescript": "typescript", // TypeScript projects (no change)
		"js":         "javascript", // Common JS abbreviation
		"ts":         "typescript", // Common TS abbreviation
	}
	
	if mapped, exists := languageMap[detectedLanguage]; exists {
		return mapped
	}
	
	// Return original language if no mapping exists
	return detectedLanguage
}

// Language-specific optimization implementations

// GoOptimizer implements Go-specific optimizations
type GoOptimizer struct{}

func (g *GoOptimizer) OptimizeForProject(ctx context.Context, serverConfig *config.ServerConfig, projectContext *ProjectContext) error {
	if serverConfig.Settings == nil {
		serverConfig.Settings = make(map[string]interface{})
	}

	// Go-specific gopls settings
	goplsSettings := map[string]interface{}{
		"gofumpt": true,
		"analyses": map[string]interface{}{
			"unusedparams":   true,
			"shadow":         true,
			"fieldalignment": false, // Can be slow on large projects
		},
		"staticcheck":     true,
		"usePlaceholders": true,
	}

	// Apply project size optimizations
	if projectContext.ProjectSize.TotalFiles > 1000 {
		// Large project optimizations
		goplsSettings["memoryMode"] = "DegradeClosed"
		goplsSettings["analyses"] = map[string]interface{}{
			"unusedparams": true,
			"shadow":       false, // Disable expensive analysis
		}
	}

	// Module-specific settings
	if goCtx, ok := projectContext.Metadata["go_context"].(*GoProjectContext); ok {
		if goCtx.GoMod.ModuleName != "" {
			goplsSettings["buildFlags"] = []string{"-tags=integration"}
		}
	}

	serverConfig.Settings["gopls"] = goplsSettings
	return nil
}

func (g *GoOptimizer) GetPerformanceSettings(projectSize ProjectSize) map[string]interface{} {
	settings := make(map[string]interface{})

	if projectSize.TotalFiles > 500 {
		settings["memoryMode"] = "DegradeClosed"
	}

	return settings
}

func (g *GoOptimizer) GetWorkspaceSettings(projectContext *ProjectContext) map[string]interface{} {
	settings := make(map[string]interface{})

	// Add workspace-specific settings
	settings["directoryFilters"] = []string{"-vendor", "-node_modules"}

	return settings
}

// PythonOptimizer implements Python-specific optimizations
type PythonOptimizer struct{}

func (p *PythonOptimizer) OptimizeForProject(ctx context.Context, serverConfig *config.ServerConfig, projectContext *ProjectContext) error {
	if serverConfig.Settings == nil {
		serverConfig.Settings = make(map[string]interface{})
	}

	pylspSettings := map[string]interface{}{
		"plugins": map[string]interface{}{
			"pycodestyle": map[string]interface{}{
				"enabled":       true,
				"maxLineLength": 88,
			},
			"pylint": map[string]interface{}{
				"enabled": true,
			},
			"rope_completion": map[string]interface{}{
				"enabled": true,
			},
		},
	}

	// Virtual environment detection
	if pythonCtx, ok := projectContext.Metadata["python_context"].(*PythonProjectContext); ok {
		if pythonCtx.VirtualEnv != "" {
			pylspSettings["plugins"].(map[string]interface{})["jedi_completion"] = map[string]interface{}{
				"enabled":     true,
				"environment": pythonCtx.VirtualEnv,
			}
		}
	}

	serverConfig.Settings["pylsp"] = pylspSettings
	return nil
}

func (p *PythonOptimizer) GetPerformanceSettings(projectSize ProjectSize) map[string]interface{} {
	settings := make(map[string]interface{})

	if projectSize.TotalFiles > 200 {
		settings["rope_completion"] = map[string]interface{}{
			"enabled": false, // Disable slow completion for large projects
		}
	}

	return settings
}

func (p *PythonOptimizer) GetWorkspaceSettings(projectContext *ProjectContext) map[string]interface{} {
	return map[string]interface{}{
		"python.analysis.exclude": []string{"**/__pycache__", "**/node_modules"},
	}
}

// TypeScriptOptimizer implements TypeScript-specific optimizations
type TypeScriptOptimizer struct{}

func (t *TypeScriptOptimizer) OptimizeForProject(ctx context.Context, serverConfig *config.ServerConfig, projectContext *ProjectContext) error {
	if serverConfig.Settings == nil {
		serverConfig.Settings = make(map[string]interface{})
	}

	tsSettings := map[string]interface{}{
		"typescript": map[string]interface{}{
			"preferences": map[string]interface{}{
				"disableSuggestions": false,
				"quotePreference":    "auto",
			},
		},
		"javascript": map[string]interface{}{
			"preferences": map[string]interface{}{
				"disableSuggestions": false,
			},
		},
	}

	// Node.js project optimizations
	if nodeCtx, ok := projectContext.Metadata["nodejs_context"].(*NodeJSProjectContext); ok {
		if nodeCtx.PackageManager == types.PKG_MGR_YARN {
			serverConfig.Args = append(serverConfig.Args, "--yarn-pnp")
		}
	}

	serverConfig.Settings = tsSettings
	return nil
}

func (t *TypeScriptOptimizer) GetPerformanceSettings(projectSize ProjectSize) map[string]interface{} {
	settings := make(map[string]interface{})

	if projectSize.TotalFiles > 1000 {
		settings["maxTsServerMemory"] = 8192 // 8GB limit
	}

	return settings
}

func (t *TypeScriptOptimizer) GetWorkspaceSettings(projectContext *ProjectContext) map[string]interface{} {
	return map[string]interface{}{
		"typescript.preferences.exclude": []string{"**/node_modules", "**/dist"},
	}
}

// JavaOptimizer implements Java-specific optimizations
type JavaOptimizer struct{}

func (j *JavaOptimizer) OptimizeForProject(ctx context.Context, serverConfig *config.ServerConfig, projectContext *ProjectContext) error {
	if serverConfig.Settings == nil {
		serverConfig.Settings = make(map[string]interface{})
	}

	jdtSettings := map[string]interface{}{
		"java": map[string]interface{}{
			"configuration": map[string]interface{}{
				"updateBuildConfiguration": "interactive",
			},
			"compile": map[string]interface{}{
				"nullAnalysis": map[string]interface{}{
					"mode": "automatic",
				},
			},
		},
	}

	// Maven/Gradle specific settings
	if javaCtx, ok := projectContext.Metadata["java_context"].(*JavaProjectContext); ok {
		switch javaCtx.BuildSystem {
		case "maven":
			jdtSettings["java"].(map[string]interface{})["import"] = map[string]interface{}{
				"maven": map[string]interface{}{
					"enabled": true,
				},
			}
		case "gradle":
			jdtSettings["java"].(map[string]interface{})["import"] = map[string]interface{}{
				"gradle": map[string]interface{}{
					"enabled": true,
				},
			}
		}
	}

	serverConfig.Settings = jdtSettings
	return nil
}

func (j *JavaOptimizer) GetPerformanceSettings(projectSize ProjectSize) map[string]interface{} {
	settings := make(map[string]interface{})

	if projectSize.TotalFiles > 500 {
		settings["java.jdt.ls.vmargs"] = "-Xmx2G"
	}

	return settings
}

func (j *JavaOptimizer) GetWorkspaceSettings(projectContext *ProjectContext) map[string]interface{} {
	return map[string]interface{}{
		"java.import.exclusions": []string{"**/node_modules/**", "**/target/**", "**/build/**"},
	}
}

// Initialization methods

func (g *ProjectConfigGeneratorImpl) initializeTemplates() {
	// Go template
	g.templates["go"] = &ProjectConfigTemplate{
		Language:    "go",
		ServerName:  setup.SERVER_GOPLS,
		DefaultArgs: []string{},
		DefaultSettings: map[string]interface{}{
			"gofumpt": true,
		},
		RootMarkers: []string{"go.mod", "go.sum"},
		OptimizationRules: []OptimizationRule{
			{
				Condition: "project_size > 1000",
				Settings: map[string]interface{}{
					"memoryMode": "DegradeClosed",
				},
				Priority: 1,
			},
		},
		PerformanceHints: &PerformanceHints{
			LargeProjectThreshold: 1000,
			LargeProjectSettings: map[string]interface{}{
				"analyses": map[string]interface{}{
					"shadow": false,
				},
			},
		},
	}

	// Python template
	g.templates["python"] = &ProjectConfigTemplate{
		Language:    "python",
		ServerName:  setup.SERVER_PYLSP,
		DefaultArgs: []string{},
		DefaultSettings: map[string]interface{}{
			"plugins": map[string]interface{}{
				"pycodestyle": map[string]interface{}{
					"enabled": true,
				},
			},
		},
		RootMarkers: []string{"requirements.txt", "pyproject.toml", "setup.py"},
		OptimizationRules: []OptimizationRule{
			{
				Condition: "has_virtual_env",
				Settings: map[string]interface{}{
					"jedi_environment_path": "${virtual_env}",
				},
				Priority: 2,
			},
		},
	}

	// TypeScript template
	g.templates["typescript"] = &ProjectConfigTemplate{
		Language:    "typescript",
		ServerName:  "typescript-language-server",
		DefaultArgs: []string{"--stdio"},
		DefaultSettings: map[string]interface{}{
			"preferences": map[string]interface{}{
				"disableSuggestions": false,
			},
		},
		RootMarkers: []string{"tsconfig.json", "package.json"},
		OptimizationRules: []OptimizationRule{
			{
				Condition: "package_manager == yarn",
				Args:      []string{"--stdio", "--yarn-pnp"},
				Priority:  1,
			},
		},
	}

	// Java template
	g.templates["java"] = &ProjectConfigTemplate{
		Language:    "java",
		ServerName:  setup.SERVER_JDTLS,
		DefaultArgs: []string{},
		DefaultSettings: map[string]interface{}{
			"java": map[string]interface{}{
				"configuration": map[string]interface{}{
					"updateBuildConfiguration": "interactive",
				},
			},
		},
		RootMarkers: []string{"pom.xml", "build.gradle", "build.gradle.kts"},
	}
}

func (g *ProjectConfigGeneratorImpl) initializeOptimizers() {
	g.optimizers["go"] = &GoOptimizer{}
	g.optimizers["python"] = &PythonOptimizer{}
	g.optimizers["typescript"] = &TypeScriptOptimizer{}
	g.optimizers["javascript"] = &TypeScriptOptimizer{} // Use TypeScript optimizer for JavaScript
	g.optimizers["java"] = &JavaOptimizer{}
}

// Utility methods

func (g *ProjectConfigGeneratorImpl) applyTemplateOptimizations(ctx context.Context, server *config.ServerConfig, projectContext *ProjectContext) error {
	for _, lang := range server.Languages {
		if template, exists := g.templates[lang]; exists {
			g.applyOptimizationRules(template.OptimizationRules, server, projectContext)
		}
	}
	return nil
}

func (g *ProjectConfigGeneratorImpl) applyOptimizationRules(rules []OptimizationRule, target interface{}, projectContext *ProjectContext) {
	for _, rule := range rules {
		if g.evaluateCondition(rule.Condition, projectContext) {
			switch t := target.(type) {
			case *config.ServerConfig:
				g.applyRuleToServerConfig(rule, t)
			case *config.ProjectServerOverride:
				g.applyRuleToOverride(rule, t)
			}
		}
	}
}

func (g *ProjectConfigGeneratorImpl) applyRuleToServerConfig(rule OptimizationRule, server *config.ServerConfig) {
	if len(rule.Args) > 0 {
		server.Args = append(server.Args, rule.Args...)
	}

	if len(rule.Settings) > 0 {
		if server.Settings == nil {
			server.Settings = make(map[string]interface{})
		}
		for key, value := range rule.Settings {
			server.Settings[key] = value
		}
	}
}

func (g *ProjectConfigGeneratorImpl) applyRuleToOverride(rule OptimizationRule, override *config.ProjectServerOverride) {
	if len(rule.Args) > 0 {
		override.Args = append(override.Args, rule.Args...)
	}

	if len(rule.Settings) > 0 {
		if override.Settings == nil {
			override.Settings = make(map[string]interface{})
		}
		for key, value := range rule.Settings {
			override.Settings[key] = value
		}
	}
}

func (g *ProjectConfigGeneratorImpl) applyProjectSizeOptimizations(server *config.ServerConfig, projectContext *ProjectContext) {
	// Apply memory optimizations for large projects
	if projectContext.ProjectSize.TotalFiles > 1000 {
		if server.Settings == nil {
			server.Settings = make(map[string]interface{})
		}

		// Generic large project settings
		server.Settings["maxFileSize"] = 1024 * 1024 // 1MB limit
		server.Settings["watchFiles"] = false        // Disable file watching for performance
	}
}

func (g *ProjectConfigGeneratorImpl) evaluateCondition(condition string, projectContext *ProjectContext) bool {
	switch condition {
	case "project_size > 1000":
		return projectContext.ProjectSize.TotalFiles > 1000
	case "project_size > 500":
		return projectContext.ProjectSize.TotalFiles > 500
	case "has_virtual_env":
		if pythonCtx, ok := projectContext.Metadata["python_context"].(*PythonProjectContext); ok {
			return pythonCtx.VirtualEnv != ""
		}
		return false
	case "package_manager == yarn":
		if nodeCtx, ok := projectContext.Metadata["nodejs_context"].(*NodeJSProjectContext); ok {
			return nodeCtx.PackageManager == types.PKG_MGR_YARN
		}
		return false
	default:
		return false
	}
}

func (g *ProjectConfigGeneratorImpl) containsLanguage(languages []string, target string) bool {
	for _, lang := range languages {
		if lang == target {
			return true
		}
	}
	return false
}

func (g *ProjectConfigGeneratorImpl) getRootMarkersForProject(projectContext *ProjectContext, language string) []string {
	if template, exists := g.templates[language]; exists {
		return template.RootMarkers
	}

	// Fallback root markers
	defaultMarkers := map[string][]string{
		"go":         {"go.mod", "go.sum"},
		"python":     {"requirements.txt", "pyproject.toml", "setup.py"},
		"typescript": {"tsconfig.json", "package.json"},
		"javascript": {"package.json"},
		"java":       {"pom.xml", "build.gradle", "build.gradle.kts"},
	}

	if markers, exists := defaultMarkers[language]; exists {
		return markers
	}

	return []string{}
}

func (g *ProjectConfigGeneratorImpl) generateProjectID(projectContext *ProjectContext) string {
	// Generate a unique project ID based on path and type
	baseName := filepath.Base(projectContext.RootPath)
	return fmt.Sprintf("%s-%s", strings.ToLower(baseName), strings.ToLower(projectContext.ProjectType))
}

func (g *ProjectConfigGeneratorImpl) generateOptimizationSettings(projectContext *ProjectContext) map[string]interface{} {
	settings := make(map[string]interface{})

	// Project size optimizations
	if projectContext.ProjectSize.TotalFiles > 1000 {
		settings["large_project_mode"] = true
		settings["indexing_strategy"] = "lazy"
	}

	// Monorepo optimizations
	if projectContext.IsMonorepo {
		settings["monorepo_mode"] = true
		settings["workspace_symbol_cache"] = true
	}

	// Language-specific optimizations
	if len(projectContext.Languages) >= 3 {
		settings["multi_language_mode"] = true
		settings["cross_language_references"] = true
	}

	return settings
}

func (g *ProjectConfigGeneratorImpl) mapProjectType(projectType string) string {
	switch projectType {
	case "go", "python", "nodejs", "java", "typescript":
		return config.ProjectTypeSingle
	case "mixed":
		return config.ProjectTypeMulti
	case "monorepo":
		return config.ProjectTypeMonorepo
	default:
		return config.ProjectTypeWorkspace
	}
}

func (g *ProjectConfigGeneratorImpl) getOptimizationLevel(projectContext *ProjectContext) string {
	if projectContext.ProjectSize.TotalFiles > 2000 {
		return "aggressive"
	} else if projectContext.ProjectSize.TotalFiles > 500 {
		return "balanced"
	}
	return "conservative"
}

func (g *ProjectConfigGeneratorImpl) validateProjectLanguageAlignment(projectContext *ProjectContext, projectConfig *config.ProjectConfig, result *ProjectConfigValidationResult) {
	projectLanguages := make(map[string]bool)
	for _, lang := range projectContext.Languages {
		projectLanguages[lang] = true
	}

	// Check if enabled servers support project languages
	for _, serverName := range projectConfig.EnabledServers {
		if serverDef, err := g.serverRegistry.GetServer(serverName); err == nil && serverDef != nil {
			hasMatchingLanguage := false
			for _, serverLang := range serverDef.Languages {
				if projectLanguages[serverLang] {
					hasMatchingLanguage = true
					break
				}
			}

			if !hasMatchingLanguage {
				result.ValidationWarnings = append(result.ValidationWarnings,
					fmt.Sprintf("Server %s does not support any project languages", serverName))
			}
		}
	}
}

func (g *ProjectConfigGeneratorImpl) validatePerformanceSettings(projectContext *ProjectContext, projectConfig *config.ProjectConfig, result *ProjectConfigValidationResult) {
	// Check for performance issues with large projects
	if projectContext.ProjectSize.TotalFiles > 1000 {
		hasLargeProjectOptimizations := false
		if opts, exists := projectConfig.Optimizations["large_project_mode"]; exists {
			if enabled, ok := opts.(bool); ok && enabled {
				hasLargeProjectOptimizations = true
			}
		}

		if !hasLargeProjectOptimizations {
			result.PerformanceWarnings = append(result.PerformanceWarnings,
				"Large project detected but no large project optimizations enabled")
		}
	}
}

func (g *ProjectConfigGeneratorImpl) generateOptimizationSuggestions(projectContext *ProjectContext, projectConfig *config.ProjectConfig, result *ProjectConfigValidationResult) {
	// Suggest monorepo optimizations
	if projectContext.IsMonorepo {
		if _, exists := projectConfig.Optimizations["monorepo_mode"]; !exists {
			result.OptimizationSuggestions = append(result.OptimizationSuggestions,
				"Enable monorepo optimizations for better workspace handling")
		}
	}

	// Suggest multi-language optimizations
	if len(projectContext.Languages) >= 3 {
		if _, exists := projectConfig.Optimizations["multi_language_mode"]; !exists {
			result.OptimizationSuggestions = append(result.OptimizationSuggestions,
				"Enable multi-language optimizations for cross-language references")
		}
	}
}

// Configuration methods

func (g *ProjectConfigGeneratorImpl) SetLogger(logger *setup.SetupLogger) {
	g.logger = logger
}

func (g *ProjectConfigGeneratorImpl) SetServerRegistry(registry setup.ServerRegistry) {
	g.serverRegistry = registry
}

func (g *ProjectConfigGeneratorImpl) SetServerVerifier(verifier setup.ServerVerifier) {
	g.serverVerifier = verifier
}
