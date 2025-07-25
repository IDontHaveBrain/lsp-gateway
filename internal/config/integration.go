package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ConfigurationIntegrator provides seamless integration between all configuration components
type ConfigurationIntegrator struct {
	generator           *ConfigGenerator
	optimizationManager *OptimizationManager
	migrationHandlers   map[string]MigrationHandler
	validators          []ConfigValidator
}

// MigrationHandler handles configuration format migrations
type MigrationHandler interface {
	CanHandle(configData map[string]interface{}) bool
	Migrate(configData map[string]interface{}) (*MultiLanguageConfig, error)
	GetSourceVersion() string
	GetTargetVersion() string
}

// ConfigValidator validates configuration integrity
type ConfigValidator interface {
	Validate(config *MultiLanguageConfig) error
	GetValidatorName() string
}

// NewConfigurationIntegrator creates a new configuration integrator
func NewConfigurationIntegrator() *ConfigurationIntegrator {
	integrator := &ConfigurationIntegrator{
		generator:           NewConfigGenerator(),
		optimizationManager: NewOptimizationManager(),
		migrationHandlers:   make(map[string]MigrationHandler),
		validators:          []ConfigValidator{},
	}

	// Register default migration handlers
	integrator.registerDefaultMigrationHandlers()

	// Register default validators
	integrator.registerDefaultValidators()

	return integrator
}

// IntegrateConfigurations merges multiple configuration sources into a unified configuration
func (ci *ConfigurationIntegrator) IntegrateConfigurations(configs ...*MultiLanguageConfig) (*MultiLanguageConfig, error) {
	if len(configs) == 0 {
		return nil, fmt.Errorf("no configurations provided for integration")
	}

	// Start with the first configuration
	integrated := &MultiLanguageConfig{
		ProjectInfo:     configs[0].ProjectInfo,
		ServerConfigs:   []*ServerConfig{},
		WorkspaceConfig: configs[0].WorkspaceConfig,
		OptimizedFor:    configs[0].OptimizedFor,
		GeneratedAt:     time.Now(),
		Version:         "1.0",
		Metadata:        make(map[string]interface{}),
	}

	// Merge server configurations from all sources
	serverMap := make(map[string]*ServerConfig)

	for _, config := range configs {
		for _, serverConfig := range config.ServerConfigs {
			if serverConfig != nil {
				key := fmt.Sprintf("%s-%s", serverConfig.Name, strings.Join(serverConfig.Languages, ","))
				if existing, exists := serverMap[key]; exists {
					// Merge server configurations
					merged, err := ci.mergeServerConfigs(existing, serverConfig)
					if err != nil {
						return nil, fmt.Errorf("failed to merge server config %s: %w", key, err)
					}
					serverMap[key] = merged
				} else {
					serverMap[key] = serverConfig
				}
			}
		}
	}

	// Convert map back to slice
	for _, serverConfig := range serverMap {
		integrated.ServerConfigs = append(integrated.ServerConfigs, serverConfig)
	}

	// Merge workspace configurations
	if err := ci.mergeWorkspaceConfigs(integrated, configs...); err != nil {
		return nil, fmt.Errorf("failed to merge workspace configurations: %w", err)
	}

	// Merge project information
	if err := ci.mergeProjectInfo(integrated, configs...); err != nil {
		return nil, fmt.Errorf("failed to merge project information: %w", err)
	}

	// Add integration metadata
	integrated.Metadata["integration_timestamp"] = time.Now()
	integrated.Metadata["source_configs"] = len(configs)
	integrated.Metadata["integrated_servers"] = len(integrated.ServerConfigs)

	return integrated, nil
}

// mergeServerConfigs merges two server configurations
func (ci *ConfigurationIntegrator) mergeServerConfigs(existing, new *ServerConfig) (*ServerConfig, error) {
	merged := &ServerConfig{
		Name:        existing.Name,
		Languages:   ci.mergeStringSlices(existing.Languages, new.Languages),
		Command:     existing.Command, // Keep original command
		Args:        ci.mergeStringSlices(existing.Args, new.Args),
		Transport:   existing.Transport,
		RootMarkers: ci.mergeStringSlices(existing.RootMarkers, new.RootMarkers),
		Settings:    ci.deepMergeSettings(existing.Settings, new.Settings),
		Priority:    maxInt(existing.Priority, new.Priority),
		Weight:      maxFloat64(existing.Weight, new.Weight),
		ServerType:  existing.ServerType,
		Frameworks:  ci.mergeStringSlices(existing.Frameworks, new.Frameworks),
		Version:     existing.Version,
	}

	// Merge language settings
	merged.LanguageSettings = make(map[string]map[string]interface{})
	for lang, settings := range existing.LanguageSettings {
		merged.LanguageSettings[lang] = ci.deepMergeSettings(nil, settings)
	}
	for lang, settings := range new.LanguageSettings {
		if existing, exists := merged.LanguageSettings[lang]; exists {
			merged.LanguageSettings[lang] = ci.deepMergeSettings(existing, settings)
		} else {
			merged.LanguageSettings[lang] = ci.deepMergeSettings(nil, settings)
		}
	}

	// Merge workspace roots
	merged.WorkspaceRoots = make(map[string]string)
	for lang, root := range existing.WorkspaceRoots {
		merged.WorkspaceRoots[lang] = root
	}
	for lang, root := range new.WorkspaceRoots {
		if root != "" {
			merged.WorkspaceRoots[lang] = root
		}
	}

	return merged, nil
}

// mergeWorkspaceConfigs merges workspace configurations
func (ci *ConfigurationIntegrator) mergeWorkspaceConfigs(target *MultiLanguageConfig, configs ...*MultiLanguageConfig) error {
	if target.WorkspaceConfig == nil {
		target.WorkspaceConfig = &WorkspaceConfig{
			LanguageRoots:  make(map[string]string),
			SharedSettings: make(map[string]interface{}),
		}
	}

	for _, config := range configs {
		if config.WorkspaceConfig != nil {
			// Merge language roots
			for lang, root := range config.WorkspaceConfig.LanguageRoots {
				if root != "" {
					target.WorkspaceConfig.LanguageRoots[lang] = root
				}
			}

			// Merge shared settings
			target.WorkspaceConfig.SharedSettings = ci.deepMergeSettings(
				target.WorkspaceConfig.SharedSettings,
				config.WorkspaceConfig.SharedSettings,
			)

			// Use most permissive settings
			if config.WorkspaceConfig.MultiRoot {
				target.WorkspaceConfig.MultiRoot = true
			}
			if config.WorkspaceConfig.CrossLanguageReferences {
				target.WorkspaceConfig.CrossLanguageReferences = true
			}

			// Use the most comprehensive indexing strategy
			if target.WorkspaceConfig.IndexingStrategy == "" ||
				config.WorkspaceConfig.IndexingStrategy == IndexingStrategyFull {
				target.WorkspaceConfig.IndexingStrategy = config.WorkspaceConfig.IndexingStrategy
			}
		}
	}

	return nil
}

// mergeProjectInfo merges project information
func (ci *ConfigurationIntegrator) mergeProjectInfo(target *MultiLanguageConfig, configs ...*MultiLanguageConfig) error {
	if target.ProjectInfo == nil {
		target.ProjectInfo = &MultiLanguageProjectInfo{
			LanguageContexts: []*LanguageContext{},
			Frameworks:       []*Framework{},
			Metadata:         make(map[string]interface{}),
		}
	}

	languageMap := make(map[string]*LanguageContext)
	frameworkMap := make(map[string]*Framework)

	// Start with target's existing contexts
	for _, langCtx := range target.ProjectInfo.LanguageContexts {
		if langCtx != nil {
			languageMap[langCtx.Language] = langCtx
		}
	}

	for _, framework := range target.ProjectInfo.Frameworks {
		if framework != nil {
			key := fmt.Sprintf("%s-%s", framework.Language, framework.Name)
			frameworkMap[key] = framework
		}
	}

	// Merge from all configurations
	for _, config := range configs {
		if config.ProjectInfo != nil {
			// Merge language contexts
			for _, langCtx := range config.ProjectInfo.LanguageContexts {
				if langCtx != nil {
					if existing, exists := languageMap[langCtx.Language]; exists {
						merged := ci.mergeLanguageContexts(existing, langCtx)
						languageMap[langCtx.Language] = merged
					} else {
						languageMap[langCtx.Language] = langCtx
					}
				}
			}

			// Merge frameworks
			for _, framework := range config.ProjectInfo.Frameworks {
				if framework != nil {
					key := fmt.Sprintf("%s-%s", framework.Language, framework.Name)
					frameworkMap[key] = framework
				}
			}

			// Merge metadata
			target.ProjectInfo.Metadata = ci.deepMergeSettings(
				target.ProjectInfo.Metadata,
				config.ProjectInfo.Metadata,
			)

			// Use most comprehensive project type
			if target.ProjectInfo.ProjectType == ProjectTypeSingle &&
				config.ProjectInfo.ProjectType != ProjectTypeSingle {
				target.ProjectInfo.ProjectType = config.ProjectInfo.ProjectType
			}
		}
	}

	// Convert maps back to slices
	target.ProjectInfo.LanguageContexts = make([]*LanguageContext, 0, len(languageMap))
	for _, langCtx := range languageMap {
		target.ProjectInfo.LanguageContexts = append(target.ProjectInfo.LanguageContexts, langCtx)
	}

	target.ProjectInfo.Frameworks = make([]*Framework, 0, len(frameworkMap))
	for _, framework := range frameworkMap {
		target.ProjectInfo.Frameworks = append(target.ProjectInfo.Frameworks, framework)
	}

	return nil
}

// mergeLanguageContexts merges two language contexts
func (ci *ConfigurationIntegrator) mergeLanguageContexts(existing, new *LanguageContext) *LanguageContext {
	merged := &LanguageContext{
		Language:       existing.Language,
		Version:        existing.Version,
		FilePatterns:   ci.mergeStringSlices(existing.FilePatterns, new.FilePatterns),
		FileCount:      maxInt(existing.FileCount, new.FileCount),
		RootMarkers:    ci.mergeStringSlices(existing.RootMarkers, new.RootMarkers),
		RootPath:       existing.RootPath,
		Submodules:     ci.mergeStringSlices(existing.Submodules, new.Submodules),
		BuildSystem:    existing.BuildSystem,
		PackageManager: existing.PackageManager,
		Frameworks:     ci.mergeStringSlices(existing.Frameworks, new.Frameworks),
		TestFrameworks: ci.mergeStringSlices(existing.TestFrameworks, new.TestFrameworks),
		LintingTools:   ci.mergeStringSlices(existing.LintingTools, new.LintingTools),
	}

	// Use newer version if available
	if new.Version != "" {
		merged.Version = new.Version
	}

	// Use more specific build system and package manager
	if new.BuildSystem != "" {
		merged.BuildSystem = new.BuildSystem
	}
	if new.PackageManager != "" {
		merged.PackageManager = new.PackageManager
	}

	// Merge complexity metrics
	if existing.Complexity != nil || new.Complexity != nil {
		merged.Complexity = &LanguageComplexity{}
		if existing.Complexity != nil {
			*merged.Complexity = *existing.Complexity
		}
		if new.Complexity != nil {
			merged.Complexity.LinesOfCode = maxInt(merged.Complexity.LinesOfCode, new.Complexity.LinesOfCode)
			merged.Complexity.CyclomaticComplexity = maxInt(merged.Complexity.CyclomaticComplexity, new.Complexity.CyclomaticComplexity)
			merged.Complexity.DependencyCount = maxInt(merged.Complexity.DependencyCount, new.Complexity.DependencyCount)
			merged.Complexity.NestingDepth = maxInt(merged.Complexity.NestingDepth, new.Complexity.NestingDepth)
		}
	}

	return merged
}

// MigrateConfiguration migrates configuration from older formats
func (ci *ConfigurationIntegrator) MigrateConfiguration(configPath string) (*MultiLanguageConfig, error) {
	// Read configuration file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse as generic map to detect format
	var configData map[string]interface{}
	ext := strings.ToLower(filepath.Ext(configPath))

	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &configData); err != nil {
			return nil, fmt.Errorf("failed to parse YAML config: %w", err)
		}
	case ".json":
		if err := json.Unmarshal(data, &configData); err != nil {
			return nil, fmt.Errorf("failed to parse JSON config: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported config file format: %s", ext)
	}

	// Find appropriate migration handler
	for _, handler := range ci.migrationHandlers {
		if handler.CanHandle(configData) {
			config, err := handler.Migrate(configData)
			if err != nil {
				return nil, fmt.Errorf("migration failed: %w", err)
			}

			// Validate migrated configuration
			if err := ci.ValidateConfiguration(config); err != nil {
				return nil, fmt.Errorf("migrated configuration validation failed: %w", err)
			}

			return config, nil
		}
	}

	return nil, fmt.Errorf("no migration handler found for configuration format")
}

// ValidateConfiguration validates configuration using all registered validators
func (ci *ConfigurationIntegrator) ValidateConfiguration(config *MultiLanguageConfig) error {
	if config == nil {
		return fmt.Errorf("configuration cannot be nil")
	}

	// Run built-in validation
	if err := config.Validate(); err != nil {
		return fmt.Errorf("built-in validation failed: %w", err)
	}

	// Run custom validators
	for _, validator := range ci.validators {
		if err := validator.Validate(config); err != nil {
			return fmt.Errorf("validator %s failed: %w", validator.GetValidatorName(), err)
		}
	}

	return nil
}

// GenerateEnhancedConfiguration generates a comprehensive configuration
func (ci *ConfigurationIntegrator) GenerateEnhancedConfiguration(projectPath string, optimizationMode string) (*MultiLanguageConfig, error) {
	// Auto-generate base configuration
	config, err := AutoGenerateConfigFromPath(projectPath)
	if err != nil {
		return nil, fmt.Errorf("failed to auto-generate configuration: %w", err)
	}

	// Apply optimization
	if optimizationMode != "" {
		if err := ci.optimizationManager.ApplyOptimization(config, optimizationMode); err != nil {
			return nil, fmt.Errorf("failed to apply optimization: %w", err)
		}
	}

	// Apply framework enhancements
	if err := ci.generator.applyFrameworkEnhancements(config, config.ProjectInfo.Frameworks); err != nil {
		return nil, fmt.Errorf("failed to apply framework enhancements: %w", err)
	}

	// Validate final configuration
	if err := ci.ValidateConfiguration(config); err != nil {
		return nil, fmt.Errorf("enhanced configuration validation failed: %w", err)
	}

	return config, nil
}

// ConvertToGatewayConfig converts MultiLanguageConfig to GatewayConfig with full integration
func (ci *ConfigurationIntegrator) ConvertToGatewayConfig(mlConfig *MultiLanguageConfig) (*GatewayConfig, error) {
	gatewayConfig, err := mlConfig.ToGatewayConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to convert to gateway config: %w", err)
	}

	// Apply additional integration enhancements
	if err := ci.enhanceGatewayConfig(gatewayConfig, mlConfig); err != nil {
		return nil, fmt.Errorf("failed to enhance gateway config: %w", err)
	}

	return gatewayConfig, nil
}

// enhanceGatewayConfig applies additional enhancements to gateway configuration
func (ci *ConfigurationIntegrator) enhanceGatewayConfig(gatewayConfig *GatewayConfig, mlConfig *MultiLanguageConfig) error {
	// Create language pools for multi-server support
	if err := gatewayConfig.MigrateServersToPool(); err != nil {
		return fmt.Errorf("failed to create language pools: %w", err)
	}

	// Apply optimization-specific gateway settings
	switch mlConfig.OptimizedFor {
	case PerformanceProfileProduction:
		gatewayConfig.MaxConcurrentRequests = 200
		gatewayConfig.Timeout = DEFAULT_TIMEOUT_15S
		gatewayConfig.EnableSmartRouting = true
	case PerformanceProfileAnalysis:
		gatewayConfig.MaxConcurrentRequests = 50
		gatewayConfig.Timeout = DEFAULT_TIMEOUT_60S
		gatewayConfig.EnableEnhancements = true
	case PerformanceProfileDevelopment:
		gatewayConfig.MaxConcurrentRequests = 100
		gatewayConfig.Timeout = DEFAULT_TIMEOUT_30S
		gatewayConfig.EnableSmartRouting = false
	}

	// Configure smart router based on project type
	if gatewayConfig.SmartRouterConfig != nil && mlConfig.ProjectInfo != nil {
		switch mlConfig.ProjectInfo.ProjectType {
		case ProjectTypeMonorepo:
			gatewayConfig.SmartRouterConfig.EnablePerformanceMonitoring = true
			gatewayConfig.SmartRouterConfig.EnableCircuitBreaker = true
		case ProjectTypeMicroservices:
			gatewayConfig.SmartRouterConfig.DefaultStrategy = "single_target_with_fallback"
		case ProjectTypePolyglot:
			gatewayConfig.SmartRouterConfig.DefaultStrategy = "broadcast_aggregate"
		}
	}

	return nil
}

// registerDefaultMigrationHandlers registers default migration handlers
func (ci *ConfigurationIntegrator) registerDefaultMigrationHandlers() {
	// Register legacy gateway config migration
	ci.migrationHandlers["legacy_gateway"] = &LegacyGatewayMigrationHandler{}

	// Register simple config migration
	ci.migrationHandlers["simple_config"] = &SimpleConfigMigrationHandler{}
}

// registerDefaultValidators registers default validators
func (ci *ConfigurationIntegrator) registerDefaultValidators() {
	ci.validators = append(ci.validators, &FrameworkCompatibilityValidator{})
	ci.validators = append(ci.validators, &ResourceConstraintValidator{})
	ci.validators = append(ci.validators, &LanguageConsistencyValidator{})
}

// Utility functions

func (ci *ConfigurationIntegrator) mergeStringSlices(slice1, slice2 []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, s := range slice1 {
		if !seen[s] {
			result = append(result, s)
			seen[s] = true
		}
	}

	for _, s := range slice2 {
		if !seen[s] {
			result = append(result, s)
			seen[s] = true
		}
	}

	return result
}

func (ci *ConfigurationIntegrator) deepMergeSettings(dst, src map[string]interface{}) map[string]interface{} {
	if dst == nil {
		dst = make(map[string]interface{})
	}
	if src == nil {
		return dst
	}

	result := make(map[string]interface{})

	// Copy destination first
	for key, value := range dst {
		result[key] = value
	}

	// Merge source
	for key, srcValue := range src {
		if dstValue, exists := result[key]; exists {
			if dstMap, dstOk := dstValue.(map[string]interface{}); dstOk {
				if srcMap, srcOk := srcValue.(map[string]interface{}); srcOk {
					result[key] = ci.deepMergeSettings(dstMap, srcMap)
					continue
				}
			}
		}
		result[key] = srcValue
	}

	return result
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func maxFloat64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// Migration handlers

// LegacyGatewayMigrationHandler migrates from legacy GatewayConfig format
type LegacyGatewayMigrationHandler struct{}

func (h *LegacyGatewayMigrationHandler) CanHandle(configData map[string]interface{}) bool {
	// Check if this is a legacy gateway config (has 'servers' and 'port' but no 'project_info')
	_, hasServers := configData["servers"]
	_, hasPort := configData["port"]
	_, hasProjectInfo := configData["project_info"]

	return hasServers && hasPort && !hasProjectInfo
}

func (h *LegacyGatewayMigrationHandler) Migrate(configData map[string]interface{}) (*MultiLanguageConfig, error) {
	// Parse as legacy GatewayConfig first
	data, err := json.Marshal(configData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config data: %w", err)
	}

	var legacyConfig GatewayConfig
	if err := json.Unmarshal(data, &legacyConfig); err != nil {
		return nil, fmt.Errorf("failed to parse as legacy gateway config: %w", err)
	}

	// Create new multi-language config
	mlConfig := &MultiLanguageConfig{
		ServerConfigs: make([]*ServerConfig, len(legacyConfig.Servers)),
		WorkspaceConfig: &WorkspaceConfig{
			LanguageRoots:  make(map[string]string),
			SharedSettings: make(map[string]interface{}),
		},
		OptimizedFor: PerformanceProfileDevelopment,
		GeneratedAt:  time.Now(),
		Version:      "1.0",
		Metadata:     make(map[string]interface{}),
	}

	// Convert servers
	for i, server := range legacyConfig.Servers {
		mlConfig.ServerConfigs[i] = &server
	}

	// Create basic project info
	languages := make([]*LanguageContext, 0)
	languageMap := make(map[string]*LanguageContext)

	for _, server := range legacyConfig.Servers {
		for _, lang := range server.Languages {
			if _, exists := languageMap[lang]; !exists {
				languageMap[lang] = &LanguageContext{
					Language:     lang,
					FilePatterns: []string{fmt.Sprintf("*.%s", lang)},
					FileCount:    10, // Default estimate
					RootMarkers:  server.RootMarkers,
					RootPath:     "/",
					Frameworks:   []string{},
				}
			}
		}
	}

	for _, langCtx := range languageMap {
		languages = append(languages, langCtx)
	}

	projectType := ProjectTypeSingle
	if len(languages) > 1 {
		projectType = ProjectTypeMulti
	}

	mlConfig.ProjectInfo = &MultiLanguageProjectInfo{
		ProjectType:      projectType,
		RootDirectory:    "/",
		LanguageContexts: languages,
		DetectedAt:       time.Now(),
		Metadata:         make(map[string]interface{}),
	}

	mlConfig.Metadata["migrated_from"] = "legacy_gateway"
	mlConfig.Metadata["migration_timestamp"] = time.Now()

	return mlConfig, nil
}

func (h *LegacyGatewayMigrationHandler) GetSourceVersion() string {
	return "legacy_gateway"
}

func (h *LegacyGatewayMigrationHandler) GetTargetVersion() string {
	return "1.0"
}

// SimpleConfigMigrationHandler migrates from simple configuration formats
type SimpleConfigMigrationHandler struct{}

func (h *SimpleConfigMigrationHandler) CanHandle(configData map[string]interface{}) bool {
	// Check if this is a simple config with just language mappings
	_, hasLanguages := configData["languages"]
	_, hasServers := configData["servers"]
	_, hasProjectInfo := configData["project_info"]

	return hasLanguages && !hasServers && !hasProjectInfo
}

func (h *SimpleConfigMigrationHandler) Migrate(configData map[string]interface{}) (*MultiLanguageConfig, error) {
	languages, ok := configData["languages"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid languages format in simple config")
	}

	// Generate configuration from language list
	generator := NewConfigGenerator()
	langContexts := make([]*LanguageContext, 0, len(languages))

	for _, lang := range languages {
		langStr, ok := lang.(string)
		if !ok {
			continue
		}

		langContexts = append(langContexts, &LanguageContext{
			Language:     langStr,
			FilePatterns: []string{fmt.Sprintf("*.%s", langStr)},
			FileCount:    5,
			RootMarkers:  []string{},
			RootPath:     "/",
			Frameworks:   []string{},
		})
	}

	projectInfo := &MultiLanguageProjectInfo{
		ProjectType:      ProjectTypeMulti,
		RootDirectory:    "/",
		LanguageContexts: langContexts,
		DetectedAt:       time.Now(),
		Metadata:         make(map[string]interface{}),
	}

	config, err := generator.GenerateMultiLanguageConfig(projectInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to generate config from simple format: %w", err)
	}

	config.Metadata["migrated_from"] = "simple_config"
	config.Metadata["migration_timestamp"] = time.Now()

	return config, nil
}

func (h *SimpleConfigMigrationHandler) GetSourceVersion() string {
	return "simple"
}

func (h *SimpleConfigMigrationHandler) GetTargetVersion() string {
	return "1.0"
}

// Validators

// FrameworkCompatibilityValidator ensures framework compatibility
type FrameworkCompatibilityValidator struct{}

func (v *FrameworkCompatibilityValidator) Validate(config *MultiLanguageConfig) error {
	if config.ProjectInfo == nil {
		return nil
	}

	for _, framework := range config.ProjectInfo.Frameworks {
		if framework != nil {
			// Find corresponding server config
			serverFound := false
			for _, serverConfig := range config.ServerConfigs {
				for _, lang := range serverConfig.Languages {
					if lang == framework.Language {
						serverFound = true
						break
					}
				}
				if serverFound {
					break
				}
			}

			if !serverFound {
				return fmt.Errorf("framework %s requires language %s but no server configured for that language",
					framework.Name, framework.Language)
			}
		}
	}

	return nil
}

func (v *FrameworkCompatibilityValidator) GetValidatorName() string {
	return "framework_compatibility"
}

// ResourceConstraintValidator validates resource constraints
type ResourceConstraintValidator struct{}

func (v *ResourceConstraintValidator) Validate(config *MultiLanguageConfig) error {
	totalMemory := int64(0)
	totalConcurrentRequests := 0

	for _, serverConfig := range config.ServerConfigs {
		if serverConfig.MaxConcurrentRequests > 0 {
			totalConcurrentRequests += serverConfig.MaxConcurrentRequests
		}
	}

	// Check reasonable limits
	if totalConcurrentRequests > 1000 {
		return fmt.Errorf("total concurrent requests (%d) exceeds reasonable limit of 1000", totalConcurrentRequests)
	}

	if totalMemory > 16384 { // 16GB
		return fmt.Errorf("total memory allocation (%d MB) exceeds reasonable limit of 16GB", totalMemory)
	}

	return nil
}

func (v *ResourceConstraintValidator) GetValidatorName() string {
	return "resource_constraints"
}

// LanguageConsistencyValidator validates language consistency
type LanguageConsistencyValidator struct{}

func (v *LanguageConsistencyValidator) Validate(config *MultiLanguageConfig) error {
	if config.ProjectInfo == nil {
		return nil
	}

	// Check that all languages in project info have corresponding server configs
	projectLanguages := make(map[string]bool)
	for _, langCtx := range config.ProjectInfo.LanguageContexts {
		if langCtx != nil {
			projectLanguages[langCtx.Language] = true
		}
	}

	serverLanguages := make(map[string]bool)
	for _, serverConfig := range config.ServerConfigs {
		for _, lang := range serverConfig.Languages {
			serverLanguages[lang] = true
		}
	}

	for lang := range projectLanguages {
		if !serverLanguages[lang] {
			return fmt.Errorf("project language %s has no corresponding server configuration", lang)
		}
	}

	return nil
}

func (v *LanguageConsistencyValidator) GetValidatorName() string {
	return "language_consistency"
}
