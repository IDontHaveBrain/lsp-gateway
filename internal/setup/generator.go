package setup

import (
	"context"
	"fmt"
	"lsp-gateway/internal/config"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// EnhancedConfigurationGenerator provides advanced configuration generation with intelligent project analysis
type EnhancedConfigurationGenerator struct {
	configGenerator  *DefaultConfigGenerator
	templateManager  *ConfigurationTemplateManager
	projectAnalyzer  *ProjectAnalyzer
	optimizationMgr  *OptimizationConfigManager
	logger           *SetupLogger
}

// NewEnhancedConfigurationGenerator creates a new enhanced configuration generator
func NewEnhancedConfigurationGenerator() *EnhancedConfigurationGenerator {
	return &EnhancedConfigurationGenerator{
		configGenerator: NewConfigGenerator(),
		templateManager: NewConfigurationTemplateManager(),
		projectAnalyzer: NewProjectAnalyzer(),
		optimizationMgr: NewOptimizationConfigManager(),
		logger:          NewSetupLogger(nil),
	}
}

// GenerateFromProject generates an optimized configuration from project analysis
func (g *EnhancedConfigurationGenerator) GenerateFromProject(ctx context.Context, projectPath string, optimizationMode string) (*ConfigGenerationResult, error) {
	startTime := time.Now()
	
	g.logger.WithOperation("enhanced-generate-from-project").WithField("project_path", projectPath).Info("Starting enhanced project configuration generation")
	
	result := &ConfigGenerationResult{
		Config:           nil,
		ServersGenerated: 0,
		ServersSkipped:   0,
		AutoDetected:     true,
		GeneratedAt:      startTime,
		Messages:         []string{},
		Warnings:         []string{},
		Issues:           []string{},
		Metadata:         make(map[string]interface{}),
	}
	
	// Perform comprehensive project analysis
	projectAnalysis, err := g.projectAnalyzer.AnalyzeProject(projectPath)
	if err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("Project analysis failed: %v", err))
		result.Duration = time.Since(startTime)
		return result, fmt.Errorf("project analysis failed: %w", err)
	}
	
	result.Messages = append(result.Messages, fmt.Sprintf("Analyzed project: %s (%s)", projectAnalysis.ProjectType, projectAnalysis.Complexity))
	
	// Select appropriate configuration template
	template, err := g.templateManager.SelectTemplate(projectAnalysis)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Template selection failed, using default: %v", err))
		template = g.templateManager.GetDefaultTemplate()
	} else {
		result.Messages = append(result.Messages, fmt.Sprintf("Selected template: %s", template.Name))
	}
	
	// Generate base configuration using template
	options := &GenerationOptions{
		OptimizationMode:    optimizationMode,
		EnableMultiServer:   template.EnableMultiServer,
		EnableSmartRouting:  template.EnableSmartRouting,
		ProjectPath:         projectPath,
		TargetLanguages:     projectAnalysis.DetectedLanguages,
		PerformanceProfile:  g.determinePerformanceProfile(projectAnalysis),
	}
	
	baseResult, err := g.configGenerator.GenerateMultiLanguageConfig(ctx, projectPath, options)
	if err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("Base configuration generation failed: %v", err))
		result.Duration = time.Since(startTime)
		return result, fmt.Errorf("base configuration generation failed: %w", err)
	}
	
	// Apply template-specific enhancements
	if err := g.applyTemplateEnhancements(baseResult.Config, template, projectAnalysis); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Template enhancement failed: %v", err))
	} else {
		result.Messages = append(result.Messages, "Applied template-specific enhancements")
	}
	
	// Apply optimization-specific configurations
	if optimizationMode != "" {
		if err := g.optimizationMgr.ApplyOptimizationConfig(baseResult.Config, optimizationMode, projectAnalysis); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Optimization configuration failed: %v", err))
		} else {
			result.Messages = append(result.Messages, fmt.Sprintf("Applied %s optimization configuration", optimizationMode))
		}
	}
	
	// Merge results
	result.Config = baseResult.Config
	result.ServersGenerated = baseResult.ServersGenerated
	result.ServersSkipped = baseResult.ServersSkipped
	result.Messages = append(result.Messages, baseResult.Messages...)
	result.Warnings = append(result.Warnings, baseResult.Warnings...)
	result.Issues = append(result.Issues, baseResult.Issues...)
	
	// Add enhanced metadata
	result.Metadata = baseResult.Metadata
	result.Metadata["enhanced_generation"] = true
	result.Metadata["project_analysis"] = projectAnalysis.ToMetadata()
	result.Metadata["template_used"] = template.Name
	result.Metadata["template_version"] = template.Version
	result.Metadata["optimization_applied"] = optimizationMode
	
	result.Duration = time.Since(startTime)
	
	g.logger.UserSuccess(fmt.Sprintf("Enhanced configuration generated: %d servers for %s project with %d languages",
		result.ServersGenerated, projectAnalysis.ProjectType, len(projectAnalysis.DetectedLanguages)))
	
	return result, nil
}

// determinePerformanceProfile determines the appropriate performance profile based on project analysis
func (g *EnhancedConfigurationGenerator) determinePerformanceProfile(analysis *ProjectAnalysis) string {
	switch analysis.Complexity {
	case ProjectComplexityHigh:
		return "high"
	case ProjectComplexityMedium:
		return "medium"
	case ProjectComplexityLow:
		return "low"
	default:
		return "medium"
	}
}

// applyTemplateEnhancements applies template-specific enhancements to the configuration
func (g *EnhancedConfigurationGenerator) applyTemplateEnhancements(cfg *config.GatewayConfig, template *ConfigurationTemplate, analysis *ProjectAnalysis) error {
	// Apply port configuration
	if template.DefaultPort > 0 {
		cfg.Port = template.DefaultPort
	}
	
	// Apply framework-specific settings
	for _, framework := range analysis.Frameworks {
		if frameworkConfig, exists := template.FrameworkConfigs[framework.Name]; exists {
			if err := g.applyFrameworkConfiguration(cfg, framework, frameworkConfig); err != nil {
				return fmt.Errorf("failed to apply framework configuration for %s: %w", framework.Name, err)
			}
		}
	}
	
	// Apply resource limits based on template
	if template.ResourceLimits != nil {
		cfg.MaxConcurrentRequests = template.ResourceLimits.MaxConcurrentRequests
		if template.ResourceLimits.TimeoutSeconds > 0 {
			cfg.Timeout = fmt.Sprintf("%ds", template.ResourceLimits.TimeoutSeconds)
		}
	}
	
	// Apply advanced routing configuration
	if template.RoutingConfig != nil {
		cfg.EnableSmartRouting = template.RoutingConfig.EnableSmartRouting
		if cfg.SmartRouterConfig != nil {
			cfg.SmartRouterConfig.DefaultStrategy = template.RoutingConfig.DefaultStrategy
			cfg.SmartRouterConfig.EnablePerformanceMonitoring = template.RoutingConfig.EnablePerformanceMonitoring
			cfg.SmartRouterConfig.EnableCircuitBreaker = template.RoutingConfig.EnableCircuitBreaker
		}
	}
	
	return nil
}

// applyFrameworkConfiguration applies framework-specific configuration
func (g *EnhancedConfigurationGenerator) applyFrameworkConfiguration(cfg *config.GatewayConfig, framework *DetectedFramework, frameworkConfig *FrameworkConfig) error {
	// Find servers for this framework's language
	for i, server := range cfg.Servers {
		for _, lang := range server.Languages {
			if lang == framework.Language {
				// Apply framework-specific settings
				if frameworkConfig.ServerSettings != nil {
					if server.Settings == nil {
						server.Settings = make(map[string]interface{})
					}
					for key, value := range frameworkConfig.ServerSettings {
						server.Settings[key] = value
					}
				}
				
				// Apply framework-specific root markers
				if len(frameworkConfig.RootMarkers) > 0 {
					server.RootMarkers = append(server.RootMarkers, frameworkConfig.RootMarkers...)
				}
				
				// Update server configuration
				cfg.Servers[i] = server
				break
			}
		}
	}
	
	return nil
}

// GenerateDefaultForEnvironment generates a default configuration optimized for a specific environment
func (g *EnhancedConfigurationGenerator) GenerateDefaultForEnvironment(ctx context.Context, environment string) (*ConfigGenerationResult, error) {
	startTime := time.Now()
	
	g.logger.WithOperation("generate-default-environment").WithField("environment", environment).Info("Generating default configuration for environment")
	
	result := &ConfigGenerationResult{
		Config:           nil,
		ServersGenerated: 0,
		ServersSkipped:   0,
		AutoDetected:     false,
		GeneratedAt:      startTime,
		Messages:         []string{},
		Warnings:         []string{},
		Issues:           []string{},
		Metadata:         make(map[string]interface{}),
	}
	
	// Get environment-specific template
	template := g.templateManager.GetEnvironmentTemplate(environment)
	if template == nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("No specific template for environment %s, using default", environment))
		template = g.templateManager.GetDefaultTemplate()
	}
	
	// Generate base configuration
	baseResult, err := g.configGenerator.GenerateDefault()
	if err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("Default configuration generation failed: %v", err))
		result.Duration = time.Since(startTime)
		return result, fmt.Errorf("default configuration generation failed: %w", err)
	}
	
	// Apply environment-specific optimizations
	optimizationMode := g.getOptimizationModeForEnvironment(environment)
	if err := g.optimizationMgr.ApplyEnvironmentOptimization(baseResult.Config, environment); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Environment optimization failed: %v", err))
	} else {
		result.Messages = append(result.Messages, fmt.Sprintf("Applied %s environment optimizations", environment))
	}
	
	// Apply template configurations
	if err := g.applyEnvironmentTemplate(baseResult.Config, template, environment); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Template application failed: %v", err))
	}
	
	result.Config = baseResult.Config
	result.ServersGenerated = baseResult.ServersGenerated
	result.Messages = append(result.Messages, baseResult.Messages...)
	result.Warnings = append(result.Warnings, baseResult.Warnings...)
	result.Issues = append(result.Issues, baseResult.Issues...)
	
	result.Metadata = baseResult.Metadata
	result.Metadata["environment"] = environment
	result.Metadata["template_used"] = template.Name
	result.Metadata["optimization_mode"] = optimizationMode
	
	result.Duration = time.Since(startTime)
	
	g.logger.UserSuccess(fmt.Sprintf("Default configuration generated for %s environment", environment))
	
	return result, nil
}

// getOptimizationModeForEnvironment returns the appropriate optimization mode for an environment
func (g *EnhancedConfigurationGenerator) getOptimizationModeForEnvironment(environment string) string {
	switch strings.ToLower(environment) {
	case "production", "prod":
		return "production"
	case "development", "dev":
		return "development"
	case "analysis", "qa", "test":
		return "analysis"
	default:
		return "development"
	}
}

// applyEnvironmentTemplate applies environment-specific template configurations
func (g *EnhancedConfigurationGenerator) applyEnvironmentTemplate(cfg *config.GatewayConfig, template *ConfigurationTemplate, environment string) error {
	// Apply environment-specific port
	if template.DefaultPort > 0 {
		cfg.Port = template.DefaultPort
	}
	
	// Apply environment-specific resource limits
	if template.ResourceLimits != nil {
		cfg.MaxConcurrentRequests = template.ResourceLimits.MaxConcurrentRequests
		if template.ResourceLimits.TimeoutSeconds > 0 {
			cfg.Timeout = fmt.Sprintf("%ds", template.ResourceLimits.TimeoutSeconds)
		}
	}
	
	// Apply environment-specific features
	switch strings.ToLower(environment) {
	case "production", "prod":
		cfg.EnableSmartRouting = true
		cfg.EnableConcurrentServers = true
		if cfg.SmartRouterConfig != nil {
			cfg.SmartRouterConfig.EnablePerformanceMonitoring = true
			cfg.SmartRouterConfig.EnableCircuitBreaker = true
		}
		
	case "development", "dev":
		cfg.EnableSmartRouting = false
		cfg.EnableConcurrentServers = true
		
	case "analysis", "qa", "test":
		cfg.EnableEnhancements = true
		cfg.MaxConcurrentRequests = 50 // Lower for analysis
	}
	
	return nil
}

// ValidateEnhancedConfiguration performs comprehensive validation of enhanced configurations
func (g *EnhancedConfigurationGenerator) ValidateEnhancedConfiguration(cfg *config.GatewayConfig, projectPath string) (*ConfigValidationResult, error) {
	startTime := time.Now()
	
	result := &ConfigValidationResult{
		Valid:            true,
		Issues:           []string{},
		Warnings:         []string{},
		ServersValidated: 0,
		ServerIssues:     make(map[string][]string),
		ValidatedAt:      startTime,
		Metadata:         make(map[string]interface{}),
	}
	
	// Perform base validation
	baseResult, err := g.configGenerator.ValidateConfig(cfg)
	if err != nil {
		return baseResult, err
	}
	
	// Merge base results
	result.Valid = baseResult.Valid
	result.Issues = append(result.Issues, baseResult.Issues...)
	result.Warnings = append(result.Warnings, baseResult.Warnings...)
	result.ServersValidated = baseResult.ServersValidated
	for server, issues := range baseResult.ServerIssues {
		result.ServerIssues[server] = issues
	}
	
	// Enhanced validations
	if projectPath != "" {
		if err := g.validateProjectStructure(cfg, projectPath, result); err != nil {
			result.Issues = append(result.Issues, fmt.Sprintf("Project structure validation failed: %v", err))
			result.Valid = false
		}
	}
	
	if err := g.validateEnhancedFeatures(cfg, result); err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("Enhanced features validation failed: %v", err))
		result.Valid = false
	}
	
	result.Duration = time.Since(startTime)
	result.Metadata["validation_type"] = "enhanced"
	result.Metadata["project_path"] = projectPath
	
	return result, nil
}

// validateProjectStructure validates configuration against actual project structure
func (g *EnhancedConfigurationGenerator) validateProjectStructure(cfg *config.GatewayConfig, projectPath string, result *ConfigValidationResult) error {
	// Check if project path exists
	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		result.Issues = append(result.Issues, fmt.Sprintf("Project path does not exist: %s", projectPath))
		return nil
	}
	
	// Validate language-specific root markers
	for _, server := range cfg.Servers {
		for _, marker := range server.RootMarkers {
			markerPath := filepath.Join(projectPath, marker)
			if _, err := os.Stat(markerPath); os.IsNotExist(err) {
				result.Warnings = append(result.Warnings, 
					fmt.Sprintf("Root marker %s not found for server %s", marker, server.Name))
			}
		}
	}
	
	return nil
}

// validateEnhancedFeatures validates enhanced configuration features
func (g *EnhancedConfigurationGenerator) validateEnhancedFeatures(cfg *config.GatewayConfig, result *ConfigValidationResult) error {
	// Validate smart routing configuration
	if cfg.EnableSmartRouting && cfg.SmartRouterConfig == nil {
		result.Warnings = append(result.Warnings, "Smart routing enabled but no router configuration provided")
	}
	
	// Validate concurrent servers configuration
	if cfg.EnableConcurrentServers {
		if cfg.MaxConcurrentServersPerLanguage <= 0 {
			result.Warnings = append(result.Warnings, "Concurrent servers enabled but no max limit specified")
		}
		
		if len(cfg.LanguagePools) == 0 && len(cfg.Servers) > 1 {
			result.Warnings = append(result.Warnings, "Multiple servers configured but no language pools defined")
		}
	}
	
	// Validate performance configuration
	if cfg.MaxConcurrentRequests > 1000 {
		result.Warnings = append(result.Warnings, "Very high concurrent request limit may affect performance")
	}
	
	return nil
}

// SetLogger sets the logger for the enhanced generator
func (g *EnhancedConfigurationGenerator) SetLogger(logger *SetupLogger) {
	if logger != nil {
		g.logger = logger
		g.configGenerator.SetLogger(logger)
		g.templateManager.SetLogger(logger)
		g.projectAnalyzer.SetLogger(logger)
		g.optimizationMgr.SetLogger(logger)
	}
}

// ProjectAnalyzer analyzes project structure and characteristics
type ProjectAnalyzer struct {
	logger *SetupLogger
}

// NewProjectAnalyzer creates a new project analyzer
func NewProjectAnalyzer() *ProjectAnalyzer {
	return &ProjectAnalyzer{
		logger: NewSetupLogger(nil),
	}
}

// ProjectAnalysis contains the results of project analysis
type ProjectAnalysis struct {
	ProjectType       string                `json:"project_type"`
	ProjectPath       string                `json:"project_path"`
	DetectedLanguages []string              `json:"detected_languages"`
	Frameworks        []*DetectedFramework  `json:"frameworks"`
	Complexity        string                `json:"complexity"`
	FileCount         int                   `json:"file_count"`
	DirectoryCount    int                   `json:"directory_count"`
	EstimatedSize     string                `json:"estimated_size"`
	BuildSystems      []string              `json:"build_systems"`
	PackageManagers   []string              `json:"package_managers"`
	TestFrameworks    []string              `json:"test_frameworks"`
	Metadata          map[string]interface{} `json:"metadata"`
	AnalyzedAt        time.Time             `json:"analyzed_at"`
}

// DetectedFramework represents a detected framework
type DetectedFramework struct {
	Name        string   `json:"name"`
	Language    string   `json:"language"`
	Version     string   `json:"version"`
	ConfigFiles []string `json:"config_files"`
	Confidence  float64  `json:"confidence"`
}

// Project complexity constants
const (
	ProjectComplexityLow    = "low"
	ProjectComplexityMedium = "medium"
	ProjectComplexityHigh   = "high"
)

// AnalyzeProject performs comprehensive project analysis
func (pa *ProjectAnalyzer) AnalyzeProject(projectPath string) (*ProjectAnalysis, error) {
	startTime := time.Now()
	
	pa.logger.WithField("project_path", projectPath).Info("Starting project analysis")
	
	analysis := &ProjectAnalysis{
		ProjectPath:       projectPath,
		DetectedLanguages: []string{},
		Frameworks:        []*DetectedFramework{},
		BuildSystems:      []string{},
		PackageManagers:   []string{},
		TestFrameworks:    []string{},
		Metadata:          make(map[string]interface{}),
		AnalyzedAt:        startTime,
	}
	
	// Check if project path exists
	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("project path does not exist: %s", projectPath)
	}
	
	// Detect languages
	if err := pa.detectLanguages(projectPath, analysis); err != nil {
		return nil, fmt.Errorf("language detection failed: %w", err)
	}
	
	// Detect frameworks
	if err := pa.detectFrameworks(projectPath, analysis); err != nil {
		pa.logger.WithError(err).Warn("Framework detection failed")
	}
	
	// Determine project type
	analysis.ProjectType = pa.determineProjectType(analysis)
	
	// Calculate complexity
	analysis.Complexity = pa.calculateComplexity(analysis)
	
	// Count files and directories
	pa.countProjectFiles(projectPath, analysis)
	
	// Detect build systems and package managers
	pa.detectBuildSystems(projectPath, analysis)
	pa.detectPackageManagers(projectPath, analysis)
	
	// Estimate project size
	analysis.EstimatedSize = pa.estimateProjectSize(analysis)
	
	pa.logger.WithFields(map[string]interface{}{
		"project_type": analysis.ProjectType,
		"languages":    len(analysis.DetectedLanguages),
		"frameworks":   len(analysis.Frameworks),
		"complexity":   analysis.Complexity,
		"duration":     time.Since(startTime),
	}).Info("Project analysis completed")
	
	return analysis, nil
}

// detectLanguages detects programming languages in the project
func (pa *ProjectAnalyzer) detectLanguages(projectPath string, analysis *ProjectAnalysis) error {
	languagePatterns := map[string][]string{
		"go":         {"*.go"},
		"python":     {"*.py"},
		"typescript": {"*.ts", "*.tsx"},
		"javascript": {"*.js", "*.jsx"},
		"java":       {"*.java"},
		"rust":       {"*.rs"},
		"cpp":        {"*.cpp", "*.cxx", "*.cc"},
		"c":          {"*.c"},
		"csharp":     {"*.cs"},
	}
	
	languageCount := make(map[string]int)
	
	for language, patterns := range languagePatterns {
		count := 0
		for _, pattern := range patterns {
			matches, err := filepath.Glob(filepath.Join(projectPath, "**", pattern))
			if err == nil {
				count += len(matches)
			}
			// Also check root level
			matches, err = filepath.Glob(filepath.Join(projectPath, pattern))
			if err == nil {
				count += len(matches)
			}
		}
		
		if count > 0 {
			languageCount[language] = count
			analysis.DetectedLanguages = append(analysis.DetectedLanguages, language)
		}
	}
	
	analysis.Metadata["language_file_counts"] = languageCount
	
	return nil
}

// detectFrameworks detects frameworks used in the project
func (pa *ProjectAnalyzer) detectFrameworks(projectPath string, analysis *ProjectAnalysis) error {
	frameworkDetectors := map[string]FrameworkDetector{
		"react": {
			Language:    "typescript",
			ConfigFiles: []string{"package.json"},
			Patterns:    []string{"react", "@types/react"},
		},
		"django": {
			Language:    "python",
			ConfigFiles: []string{"manage.py", "requirements.txt", "pyproject.toml"},
			Patterns:    []string{"django", "Django"},
		},
		"spring": {
			Language:    "java",
			ConfigFiles: []string{"pom.xml", "build.gradle"},
			Patterns:    []string{"spring-boot", "springframework"},
		},
		"gin": {
			Language:    "go",
			ConfigFiles: []string{"go.mod"},
			Patterns:    []string{"gin-gonic/gin"},
		},
		"axum": {
			Language:    "rust",
			ConfigFiles: []string{"Cargo.toml"},
			Patterns:    []string{"axum"},
		},
	}
	
	for frameworkName, detector := range frameworkDetectors {
		if framework := pa.detectFramework(projectPath, frameworkName, detector); framework != nil {
			analysis.Frameworks = append(analysis.Frameworks, framework)
		}
	}
	
	return nil
}

// FrameworkDetector defines how to detect a specific framework
type FrameworkDetector struct {
	Language    string
	ConfigFiles []string
	Patterns    []string
}

// detectFramework detects a specific framework
func (pa *ProjectAnalyzer) detectFramework(projectPath, frameworkName string, detector FrameworkDetector) *DetectedFramework {
	var configFiles []string
	var confidence float64
	
	// Check for configuration files
	for _, configFile := range detector.ConfigFiles {
		configPath := filepath.Join(projectPath, configFile)
		if _, err := os.Stat(configPath); err == nil {
			configFiles = append(configFiles, configFile)
			confidence += 0.3
			
			// Check patterns in config files
			if pa.checkPatternsInFile(configPath, detector.Patterns) {
				confidence += 0.4
			}
		}
	}
	
	if confidence > 0.5 {
		return &DetectedFramework{
			Name:        frameworkName,
			Language:    detector.Language,
			ConfigFiles: configFiles,
			Confidence:  confidence,
		}
	}
	
	return nil
}

// checkPatternsInFile checks if patterns exist in a file
func (pa *ProjectAnalyzer) checkPatternsInFile(filePath string, patterns []string) bool {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return false
	}
	
	fileContent := string(content)
	for _, pattern := range patterns {
		if strings.Contains(fileContent, pattern) {
			return true
		}
	}
	
	return false
}

// determineProjectType determines the project type based on detected languages and structure
func (pa *ProjectAnalyzer) determineProjectType(analysis *ProjectAnalysis) string {
	numLanguages := len(analysis.DetectedLanguages)
	
	if numLanguages == 1 {
		return config.ProjectTypeSingle
	} else if numLanguages > 1 {
		// Check if it's a monorepo (multiple package files in subdirectories)
		if pa.isMonorepo(analysis.ProjectPath) {
			return config.ProjectTypeMonorepo
		}
		
		// Check if it's microservices (multiple service directories)
		if pa.isMicroservices(analysis.ProjectPath) {
			return config.ProjectTypeMicroservices
		}
		
		return config.ProjectTypeMulti
	}
	
	return config.ProjectTypeUnknown
}

// isMonorepo checks if the project is a monorepo
func (pa *ProjectAnalyzer) isMonorepo(projectPath string) bool {
	// Look for monorepo indicators
	monorepoIndicators := []string{
		"lerna.json",
		"nx.json",
		"rush.json",
		"pnpm-workspace.yaml",
		"workspace.json",
	}
	
	for _, indicator := range monorepoIndicators {
		if _, err := os.Stat(filepath.Join(projectPath, indicator)); err == nil {
			return true
		}
	}
	
	// Check for multiple package.json files in subdirectories
	packageJsonCount := 0
	if err := filepath.Walk(projectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Name() == "package.json" {
			packageJsonCount++
			if packageJsonCount > 2 { // More than root + one subdirectory
				return filepath.SkipDir
			}
		}
		return nil
	}); err == nil && packageJsonCount > 2 {
		return true
	}
	
	return false
}

// isMicroservices checks if the project follows microservices architecture
func (pa *ProjectAnalyzer) isMicroservices(projectPath string) bool {
	// Look for microservices indicators
	microservicesIndicators := []string{
		"docker-compose.yml",
		"kubernetes",
		"k8s",
		"services",
		"microservices",
	}
	
	for _, indicator := range microservicesIndicators {
		if _, err := os.Stat(filepath.Join(projectPath, indicator)); err == nil {
			return true
		}
	}
	
	return false
}

// calculateComplexity calculates project complexity
func (pa *ProjectAnalyzer) calculateComplexity(analysis *ProjectAnalysis) string {
	score := 0
	
	// Language count factor
	if len(analysis.DetectedLanguages) > 3 {
		score += 3
	} else if len(analysis.DetectedLanguages) > 1 {
		score += 2
	} else {
		score += 1
	}
	
	// Framework count factor
	if len(analysis.Frameworks) > 2 {
		score += 2
	} else if len(analysis.Frameworks) > 0 {
		score += 1
	}
	
	// Project type factor
	switch analysis.ProjectType {
	case config.ProjectTypeMonorepo, config.ProjectTypeMicroservices:
		score += 3
	case config.ProjectTypeMulti:
		score += 2
	case config.ProjectTypeSingle:
		score += 1
	}
	
	// File count factor (estimated)
	if analysis.FileCount > 1000 {
		score += 2
	} else if analysis.FileCount > 100 {
		score += 1
	}
	
	// Determine complexity level
	if score >= 8 {
		return ProjectComplexityHigh
	} else if score >= 5 {
		return ProjectComplexityMedium
	} else {
		return ProjectComplexityLow
	}
}

// countProjectFiles counts files and directories in the project
func (pa *ProjectAnalyzer) countProjectFiles(projectPath string, analysis *ProjectAnalysis) {
	fileCount := 0
	dirCount := 0
	
	filepath.Walk(projectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		// Skip hidden directories and common non-source directories
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || 
			   name == "node_modules" || 
			   name == "target" || 
			   name == "__pycache__" ||
			   name == "vendor" {
				return filepath.SkipDir
			}
			dirCount++
		} else {
			fileCount++
		}
		
		return nil
	})
	
	analysis.FileCount = fileCount
	analysis.DirectoryCount = dirCount
}

// detectBuildSystems detects build systems used in the project
func (pa *ProjectAnalyzer) detectBuildSystems(projectPath string, analysis *ProjectAnalysis) {
	buildSystemFiles := map[string]string{
		"Makefile":      "make",
		"CMakeLists.txt": "cmake",
		"build.gradle":  "gradle",
		"pom.xml":       "maven",
		"Cargo.toml":    "cargo",
		"go.mod":        "go-modules",
		"package.json":  "npm",
		"pyproject.toml": "python-build",
	}
	
	for file, system := range buildSystemFiles {
		if _, err := os.Stat(filepath.Join(projectPath, file)); err == nil {
			analysis.BuildSystems = append(analysis.BuildSystems, system)
		}
	}
}

// detectPackageManagers detects package managers used in the project
func (pa *ProjectAnalyzer) detectPackageManagers(projectPath string, analysis *ProjectAnalysis) {
	packageManagerFiles := map[string]string{
		"package-lock.json": "npm",
		"yarn.lock":         "yarn",
		"pnpm-lock.yaml":    "pnpm",
		"Pipfile":           "pipenv",
		"poetry.lock":       "poetry",
		"requirements.txt":  "pip",
		"Cargo.lock":        "cargo",
		"go.sum":            "go-modules",
	}
	
	for file, manager := range packageManagerFiles {
		if _, err := os.Stat(filepath.Join(projectPath, file)); err == nil {
			analysis.PackageManagers = append(analysis.PackageManagers, manager)
		}
	}
}

// estimateProjectSize estimates the project size category
func (pa *ProjectAnalyzer) estimateProjectSize(analysis *ProjectAnalysis) string {
	if analysis.FileCount > 5000 {
		return "large"
	} else if analysis.FileCount > 500 {
		return "medium"
	} else {
		return "small"
	}
}

// ToMetadata converts project analysis to metadata map
func (pa *ProjectAnalysis) ToMetadata() map[string]interface{} {
	return map[string]interface{}{
		"project_type":       pa.ProjectType,
		"detected_languages": pa.DetectedLanguages,
		"frameworks":         len(pa.Frameworks),
		"complexity":         pa.Complexity,
		"file_count":         pa.FileCount,
		"directory_count":    pa.DirectoryCount,
		"estimated_size":     pa.EstimatedSize,
		"build_systems":      pa.BuildSystems,
		"package_managers":   pa.PackageManagers,
		"analyzed_at":        pa.AnalyzedAt,
	}
}

// SetLogger sets the logger for project analyzer
func (pa *ProjectAnalyzer) SetLogger(logger *SetupLogger) {
	if logger != nil {
		pa.logger = logger
	}
}

// OptimizationConfigManager manages optimization-specific configurations
type OptimizationConfigManager struct {
	logger *SetupLogger
}

// NewOptimizationConfigManager creates a new optimization config manager
func NewOptimizationConfigManager() *OptimizationConfigManager {
	return &OptimizationConfigManager{
		logger: NewSetupLogger(nil),
	}
}

// ApplyOptimizationConfig applies optimization-specific configuration
func (ocm *OptimizationConfigManager) ApplyOptimizationConfig(cfg *config.GatewayConfig, optimizationMode string, analysis *ProjectAnalysis) error {
	ocm.logger.WithFields(map[string]interface{}{
		"optimization_mode": optimizationMode,
		"project_type":      analysis.ProjectType,
		"complexity":        analysis.Complexity,
	}).Info("Applying optimization configuration")
	
	switch strings.ToLower(optimizationMode) {
	case "production":
		return ocm.applyProductionOptimization(cfg, analysis)
	case "development":
		return ocm.applyDevelopmentOptimization(cfg, analysis)
	case "analysis":
		return ocm.applyAnalysisOptimization(cfg, analysis)
	default:
		return fmt.Errorf("unknown optimization mode: %s", optimizationMode)
	}
}

// applyProductionOptimization applies production-specific optimizations
func (ocm *OptimizationConfigManager) applyProductionOptimization(cfg *config.GatewayConfig, analysis *ProjectAnalysis) error {
	cfg.EnableSmartRouting = true
	cfg.EnableConcurrentServers = true
	
	// Adjust based on project complexity
	switch analysis.Complexity {
	case ProjectComplexityHigh:
		cfg.MaxConcurrentRequests = 500
		cfg.Timeout = "15s"
		cfg.MaxConcurrentServersPerLanguage = 3
	case ProjectComplexityMedium:
		cfg.MaxConcurrentRequests = 300
		cfg.Timeout = "20s"
		cfg.MaxConcurrentServersPerLanguage = 2
	case ProjectComplexityLow:
		cfg.MaxConcurrentRequests = 200
		cfg.Timeout = "30s"
		cfg.MaxConcurrentServersPerLanguage = 1
	}
	
	// Configure smart router for production
	if cfg.SmartRouterConfig != nil {
		cfg.SmartRouterConfig.EnablePerformanceMonitoring = true
		cfg.SmartRouterConfig.EnableCircuitBreaker = true
		cfg.SmartRouterConfig.DefaultStrategy = "single_target_with_fallback"
	}
	
	return nil
}

// applyDevelopmentOptimization applies development-specific optimizations
func (ocm *OptimizationConfigManager) applyDevelopmentOptimization(cfg *config.GatewayConfig, analysis *ProjectAnalysis) error {
	cfg.EnableSmartRouting = false
	cfg.EnableConcurrentServers = true
	
	// Development settings are more permissive
	cfg.MaxConcurrentRequests = 100
	cfg.Timeout = "60s"
	cfg.MaxConcurrentServersPerLanguage = 2
	
	// Disable performance monitoring for development
	if cfg.SmartRouterConfig != nil {
		cfg.SmartRouterConfig.EnablePerformanceMonitoring = false
		cfg.SmartRouterConfig.EnableCircuitBreaker = false
	}
	
	return nil
}

// applyAnalysisOptimization applies analysis-specific optimizations
func (ocm *OptimizationConfigManager) applyAnalysisOptimization(cfg *config.GatewayConfig, analysis *ProjectAnalysis) error {
	cfg.EnableEnhancements = true
	cfg.EnableSmartRouting = false
	cfg.EnableConcurrentServers = false // Single server for consistent analysis
	
	// Analysis settings prioritize accuracy over performance
	cfg.MaxConcurrentRequests = 50
	cfg.Timeout = "120s" // Longer timeout for analysis operations
	
	return nil
}

// ApplyEnvironmentOptimization applies environment-specific optimizations
func (ocm *OptimizationConfigManager) ApplyEnvironmentOptimization(cfg *config.GatewayConfig, environment string) error {
	optimizationMode := ocm.getOptimizationModeForEnvironment(environment)
	
	// Create a basic analysis for environment optimization
	analysis := &ProjectAnalysis{
		ProjectType: config.ProjectTypeSingle,
		Complexity:  ProjectComplexityMedium,
	}
	
	return ocm.ApplyOptimizationConfig(cfg, optimizationMode, analysis)
}

// getOptimizationModeForEnvironment maps environment to optimization mode
func (ocm *OptimizationConfigManager) getOptimizationModeForEnvironment(environment string) string {
	switch strings.ToLower(environment) {
	case "production", "prod":
		return "production"
	case "development", "dev":
		return "development"
	case "analysis", "qa", "test":
		return "analysis"
	default:
		return "development"
	}
}

// SetLogger sets the logger for optimization config manager
func (ocm *OptimizationConfigManager) SetLogger(logger *SetupLogger) {
	if logger != nil {
		ocm.logger = logger
	}
}