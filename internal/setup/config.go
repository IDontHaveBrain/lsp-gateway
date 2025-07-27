package setup

import (
	"context"
	"fmt"
	"lsp-gateway/internal/config"
	"lsp-gateway/internal/installer"
	"lsp-gateway/internal/types"
	"time"
)

type ServerVerifier interface {
	VerifyServer(serverName string) (*ServerVerificationResult, error)
}

type ProjectScanner interface {
	ScanProject(rootPath string) (*config.MultiLanguageProjectInfo, error)
}

// SimpleProjectScanner is a basic implementation that avoids import cycles
type SimpleProjectScanner struct{}

func NewSimpleProjectScanner() *SimpleProjectScanner {
	return &SimpleProjectScanner{}
}

func (s *SimpleProjectScanner) ScanProject(rootPath string) (*config.MultiLanguageProjectInfo, error) {
	// Basic implementation to avoid import cycle
	// TODO: Replace with proper implementation once import cycle is resolved
	return &config.MultiLanguageProjectInfo{
		ProjectType:      "mixed",
		RootDirectory:    rootPath,
		LanguageContexts: []*config.LanguageContext{},
		DetectedAt:       time.Now(),
		Metadata:         make(map[string]interface{}),
	}, nil
}

type ServerRegistry interface {
	GetServersByRuntime(runtime string) []*ServerDefinition
	GetServer(serverName string) (*ServerDefinition, error)
}

type ServerDefinition struct {
	Name          string                 `json:"name"`
	DisplayName   string                 `json:"display_name"`
	Languages     []string               `json:"languages"`
	DefaultConfig map[string]interface{} `json:"default_config,omitempty"`
}

type ServerVerificationResult struct {
	Installed  bool   `json:"installed"`
	Compatible bool   `json:"compatible"`
	Functional bool   `json:"functional"`
	Path       string `json:"path,omitempty"`
}

type ConfigGenerator interface {
	GenerateFromDetected(ctx context.Context) (*ConfigGenerationResult, error)

	GenerateFromDetectedWithInstallResults(ctx context.Context, detectedRuntimes map[string]*RuntimeInfo, serverInstallResults map[string]*types.InstallResult) (*ConfigGenerationResult, error)

	GenerateForRuntime(ctx context.Context, runtime string) (*ConfigGenerationResult, error)

	GenerateMultiLanguageConfig(ctx context.Context, projectPath string, options *GenerationOptions) (*ConfigGenerationResult, error)

	GenerateMultiLanguageConfigWithInstallResults(ctx context.Context, projectPath string, options *GenerationOptions, detectedRuntimes map[string]*RuntimeInfo, serverInstallResults map[string]*types.InstallResult) (*ConfigGenerationResult, error)

	GenerateDefault() (*ConfigGenerationResult, error)

	UpdateConfig(existing *config.GatewayConfig, updates ConfigUpdates) (*ConfigUpdateResult, error)

	ValidateConfig(config *config.GatewayConfig) (*ConfigValidationResult, error)

	ValidateGenerated(config *config.GatewayConfig) (*ConfigValidationResult, error)

	SetLogger(logger *SetupLogger)
}

type ConfigGenerationResult struct {
	Config           *config.GatewayConfig  `json:"config"`
	DetectionReport  *DetectionReport       `json:"detection_report,omitempty"`
	ServersGenerated int                    `json:"servers_generated"`
	ServersSkipped   int                    `json:"servers_skipped"`
	AutoDetected     bool                   `json:"auto_detected"`
	GeneratedAt      time.Time              `json:"generated_at"`
	Duration         time.Duration          `json:"duration"`
	Messages         []string               `json:"messages"`
	Warnings         []string               `json:"warnings"`
	Issues           []string               `json:"issues"`
	Metadata         map[string]interface{} `json:"metadata"`
}

type ConfigValidationResult struct {
	Valid            bool                   `json:"valid"`
	Issues           []string               `json:"issues"`
	Warnings         []string               `json:"warnings"`
	ServersValidated int                    `json:"servers_validated"`
	ServerIssues     map[string][]string    `json:"server_issues"`
	ValidatedAt      time.Time              `json:"validated_at"`
	Duration         time.Duration          `json:"duration"`
	Metadata         map[string]interface{} `json:"metadata"`
}

type ConfigUpdates struct {
	Port int `json:"port,omitempty"`

	AddServers []config.ServerConfig `json:"add_servers,omitempty"`

	RemoveServers []string `json:"remove_servers,omitempty"`

	UpdateServers []config.ServerConfig `json:"update_servers,omitempty"`

	ReplaceAllServers bool `json:"replace_all_servers,omitempty"`

	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type ConfigUpdateResult struct {
	Config         *config.GatewayConfig  `json:"config"`
	UpdatesApplied ConfigUpdatesSummary   `json:"updates_applied"`
	UpdatedAt      time.Time              `json:"updated_at"`
	Duration       time.Duration          `json:"duration"`
	Messages       []string               `json:"messages"`
	Warnings       []string               `json:"warnings"`
	Issues         []string               `json:"issues"`
	Metadata       map[string]interface{} `json:"metadata"`
}

type ConfigUpdatesSummary struct {
	PortChanged        bool     `json:"port_changed"`
	ServersAdded       int      `json:"servers_added"`
	ServersRemoved     int      `json:"servers_removed"`
	ServersUpdated     int      `json:"servers_updated"`
	ServersReplaced    bool     `json:"servers_replaced"`
	AddedServerNames   []string `json:"added_server_names"`
	RemovedServerNames []string `json:"removed_server_names"`
	UpdatedServerNames []string `json:"updated_server_names"`
}

type ServerConfigTemplate struct {
	RuntimeName       string   `json:"runtime_name"`
	ServerName        string   `json:"server_name"`
	ConfigName        string   `json:"config_name"`
	Command           string   `json:"command"`
	Args              []string `json:"args"`
	Languages         []string `json:"languages"`
	Transport         string   `json:"transport"`
	RequiredRuntime   string   `json:"required_runtime"`
	MinRuntimeVersion string   `json:"min_runtime_version"`
}

type DefaultConfigGenerator struct {
	runtimeDetector RuntimeDetector
	serverVerifier  ServerVerifier
	serverRegistry  ServerRegistry
	logger          *SetupLogger
	templates       map[string]*ServerConfigTemplate
	// Multi-language enhancements
	multiLangGenerator *config.ConfigGenerator
	integrator         *config.ConfigurationIntegrator
	projectScanner     ProjectScanner
}

func NewConfigGenerator() *DefaultConfigGenerator {
	generator := &DefaultConfigGenerator{
		runtimeDetector: NewRuntimeDetector(),
		serverVerifier:  NewDefaultServerVerifier(),
		serverRegistry:  NewDefaultServerRegistry(),
		logger:          NewSetupLogger(nil),
		templates:       make(map[string]*ServerConfigTemplate),
		// Initialize multi-language components
		multiLangGenerator: config.NewConfigGenerator(),
		integrator:         config.NewConfigurationIntegrator(),
		projectScanner:     NewSimpleProjectScanner(),
	}

	generator.initializeTemplates()

	return generator
}

// GenerateMultiLanguageConfig generates configuration with multi-language awareness and optimization
func (g *DefaultConfigGenerator) GenerateMultiLanguageConfig(ctx context.Context, projectPath string, options *GenerationOptions) (*ConfigGenerationResult, error) {
	startTime := time.Now()

	g.logger.WithOperation("generate-multi-language-config").WithField("project_path", projectPath).Info("Generating multi-language configuration")

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

	// Apply default options if none provided
	if options == nil {
		options = &GenerationOptions{
			OptimizationMode:   config.PerformanceProfileDevelopment,
			EnableMultiServer:  true,
			EnableSmartRouting: true,
			ProjectPath:        projectPath,
			PerformanceProfile: "medium",
		}
	}

	// Scan project for language detection
	projectInfo, err := g.projectScanner.ScanProject(projectPath)
	if err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("Project scanning failed: %v", err))
		result.Duration = time.Since(startTime)
		return result, fmt.Errorf("project scanning failed: %w", err)
	}

	// Generate multi-language configuration
	mlConfig, err := g.multiLangGenerator.GenerateMultiLanguageConfig(projectInfo)
	if err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("Multi-language config generation failed: %v", err))
		result.Duration = time.Since(startTime)
		return result, fmt.Errorf("multi-language config generation failed: %w", err)
	}

	// Apply optimization based on mode
	if options.OptimizationMode != "" {
		optimizationManager := config.NewOptimizationManager()
		if err := optimizationManager.ApplyOptimization(mlConfig, options.OptimizationMode); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Optimization failed: %v", err))
		} else {
			result.Messages = append(result.Messages, fmt.Sprintf("Applied %s optimization", options.OptimizationMode))
		}
	}

	// Convert to gateway config
	gatewayConfig, err := g.integrator.ConvertToGatewayConfig(mlConfig)
	if err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("Gateway config conversion failed: %v", err))
		result.Duration = time.Since(startTime)
		return result, fmt.Errorf("gateway config conversion failed: %w", err)
	}

	// Apply smart configuration based on project characteristics
	g.applySmartConfiguration(gatewayConfig, mlConfig, options)

	// Validate generated configuration
	if err := gatewayConfig.Validate(); err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("Generated configuration is invalid: %v", err))
		result.Duration = time.Since(startTime)
		return result, fmt.Errorf("configuration validation failed: %w", err)
	}

	result.Config = gatewayConfig
	result.ServersGenerated = len(gatewayConfig.Servers)
	result.Duration = time.Since(startTime)

	// Populate metadata
	result.Metadata["generation_method"] = "multi_language_aware"
	result.Metadata["project_type"] = mlConfig.ProjectInfo.ProjectType
	result.Metadata["languages_detected"] = len(mlConfig.ProjectInfo.LanguageContexts)
	result.Metadata["optimization_mode"] = options.OptimizationMode
	result.Metadata["smart_routing_enabled"] = options.EnableSmartRouting
	result.Metadata["multi_server_enabled"] = options.EnableMultiServer

	g.logger.UserSuccess(fmt.Sprintf("Multi-language configuration generated: %d servers for %d languages",
		result.ServersGenerated, len(mlConfig.ProjectInfo.LanguageContexts)))

	return result, nil
}

// GenerateFromProjectPath is a convenience method that generates configuration from a project path with smart defaults
func (g *DefaultConfigGenerator) GenerateFromProjectPath(ctx context.Context, projectPath string, optimizationMode string) (*ConfigGenerationResult, error) {
	options := &GenerationOptions{
		OptimizationMode:   optimizationMode,
		EnableMultiServer:  true,
		EnableSmartRouting: true,
		ProjectPath:        projectPath,
		PerformanceProfile: "medium",
	}

	if optimizationMode == "" {
		options.OptimizationMode = config.PerformanceProfileDevelopment
	}

	return g.GenerateMultiLanguageConfig(ctx, projectPath, options)
}

// GenerationOptions defines options for enhanced configuration generation
type GenerationOptions struct {
	OptimizationMode   string   `json:"optimization_mode"`    // "production", "development", "analysis"
	EnableMultiServer  bool     `json:"enable_multi_server"`  // Enable concurrent server management
	EnableSmartRouting bool     `json:"enable_smart_routing"` // Enable intelligent request routing
	ProjectPath        string   `json:"project_path"`         // Project root path for detection
	TargetLanguages    []string `json:"target_languages"`     // Specific languages to configure (optional)
	PerformanceProfile string   `json:"performance_profile"`  // "low", "medium", "high"
}

// applySmartConfiguration applies intelligent configuration based on project characteristics
func (g *DefaultConfigGenerator) applySmartConfiguration(gatewayConfig *config.GatewayConfig, mlConfig *config.MultiLanguageConfig, options *GenerationOptions) {
	// Configure based on project type
	switch mlConfig.ProjectInfo.ProjectType {
	case config.ProjectTypeMonorepo:
		gatewayConfig.ProjectAware = true
		gatewayConfig.EnableConcurrentServers = true
		gatewayConfig.MaxConcurrentServersPerLanguage = 3
		if gatewayConfig.SmartRouterConfig != nil {
			gatewayConfig.SmartRouterConfig.EnablePerformanceMonitoring = true
			gatewayConfig.SmartRouterConfig.EnableCircuitBreaker = true
		}

	case config.ProjectTypeMulti:
		gatewayConfig.EnableConcurrentServers = true
		gatewayConfig.MaxConcurrentServersPerLanguage = 2
		if gatewayConfig.SmartRouterConfig != nil {
			gatewayConfig.SmartRouterConfig.DefaultStrategy = "single_target_with_fallback"
		}

	case config.ProjectTypeMicroservices:
		gatewayConfig.EnableSmartRouting = true
		if gatewayConfig.SmartRouterConfig != nil {
			gatewayConfig.SmartRouterConfig.DefaultStrategy = "broadcast_aggregate"
		}
	}

	// Apply performance profile
	switch options.PerformanceProfile {
	case "high":
		gatewayConfig.MaxConcurrentRequests = 300
		gatewayConfig.Timeout = "60s"
	case "medium":
		gatewayConfig.MaxConcurrentRequests = 150
		gatewayConfig.Timeout = "30s"
	case "low":
		gatewayConfig.MaxConcurrentRequests = 50
		gatewayConfig.Timeout = "15s"
	}

	// Configure language-specific pools
	if len(mlConfig.ProjectInfo.LanguageContexts) > 1 {
		// Create language pools for multi-language projects
		if err := gatewayConfig.MigrateServersToPool(); err != nil {
			g.logger.WithOperation("migrate-servers").WithError(err).Warn("Failed to migrate servers to language pools, continuing with current configuration")
		}
	}
}

func (g *DefaultConfigGenerator) GenerateForRuntime(ctx context.Context, runtime string) (*ConfigGenerationResult, error) {
	startTime := time.Now()

	g.logger.WithOperation("generate-config-runtime").WithField("runtime", runtime).Info("Generating configuration for specific runtime")

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

	var runtimeInfo *RuntimeInfo
	var err error

	switch runtime {
	case "go":
		runtimeInfo, err = g.runtimeDetector.DetectGo(ctx)
	case COMMAND_PYTHON:
		runtimeInfo, err = g.runtimeDetector.DetectPython(ctx)
	case "nodejs":
		runtimeInfo, err = g.runtimeDetector.DetectNodejs(ctx)
	case "java":
		runtimeInfo, err = g.runtimeDetector.DetectJava(ctx)
	default:
		result.Issues = append(result.Issues, fmt.Sprintf("Unsupported runtime: %s", runtime))
		result.Duration = time.Since(startTime)
		return result, fmt.Errorf("unsupported runtime: %s", runtime)
	}

	if err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("Failed to detect %s runtime: %v", runtime, err))
		result.Duration = time.Since(startTime)
		return result, fmt.Errorf("runtime detection failed: %w", err)
	}

	gatewayConfig := &config.GatewayConfig{
		Port:    8080,
		Servers: []config.ServerConfig{},
	}

	if runtimeInfo.Installed && runtimeInfo.Compatible {
		servers := g.serverRegistry.GetServersByRuntime(runtime)
		for _, serverDef := range servers {
			serverConfig, err := g.generateServerConfig(ctx, runtimeInfo, serverDef)
			if err != nil {
				result.Issues = append(result.Issues,
					fmt.Sprintf("Failed to generate config for %s: %v", serverDef.Name, err))
				result.ServersSkipped++
				continue
			}

			if serverConfig != nil {
				gatewayConfig.Servers = append(gatewayConfig.Servers, *serverConfig)
				result.ServersGenerated++
				result.Messages = append(result.Messages,
					fmt.Sprintf("Generated configuration for %s", serverDef.DisplayName))
			} else {
				result.ServersSkipped++
			}
		}
	} else {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("%s runtime is not installed or compatible", runtime))
	}

	if len(gatewayConfig.Servers) > 0 {
		if err := gatewayConfig.Validate(); err != nil {
			result.Issues = append(result.Issues, fmt.Sprintf("Generated configuration is invalid: %v", err))
			result.Duration = time.Since(startTime)
			return result, fmt.Errorf("configuration validation failed: %w", err)
		}
	} else {
		result.Warnings = append(result.Warnings, fmt.Sprintf("No servers configured for %s runtime", runtime))
	}

	result.Config = gatewayConfig
	result.Duration = time.Since(startTime)
	result.Metadata["target_runtime"] = runtime
	result.Metadata["runtime_installed"] = runtimeInfo.Installed
	result.Metadata["runtime_compatible"] = runtimeInfo.Compatible
	result.Metadata["runtime_version"] = runtimeInfo.Version

	return result, nil
}

func (g *DefaultConfigGenerator) GenerateFromDetected(ctx context.Context) (*ConfigGenerationResult, error) {
	startTime := time.Now()

	g.logger.WithOperation("generate-from-detected").Info("Starting auto-detection and configuration generation")

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

	// Detect all available runtimes
	detectionReport, err := g.runtimeDetector.DetectAll(ctx)
	if err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("Runtime detection failed: %v", err))
		result.Duration = time.Since(startTime)
		return result, fmt.Errorf("runtime detection failed: %w", err)
	}

	result.DetectionReport = detectionReport

	// Create base configuration
	gatewayConfig := &config.GatewayConfig{
		Port:                  8080,
		MaxConcurrentRequests: 100, // Set default value to pass validation
		Servers:               []config.ServerConfig{},
	}

	// Generate configuration for each detected and compatible runtime
	for runtimeName, runtimeInfo := range detectionReport.Runtimes {
		if !runtimeInfo.Installed || !runtimeInfo.Compatible {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Skipping %s: installed=%v, compatible=%v",
					runtimeName, runtimeInfo.Installed, runtimeInfo.Compatible))
			result.ServersSkipped++
			continue
		}

		servers := g.serverRegistry.GetServersByRuntime(runtimeName)
		for _, serverDef := range servers {
			serverConfig, err := g.generateServerConfig(ctx, runtimeInfo, serverDef)
			if err != nil {
				result.Issues = append(result.Issues,
					fmt.Sprintf("Failed to generate config for %s: %v", serverDef.Name, err))
				result.ServersSkipped++
				continue
			}

			if serverConfig != nil {
				gatewayConfig.Servers = append(gatewayConfig.Servers, *serverConfig)
				result.ServersGenerated++
				result.Messages = append(result.Messages,
					fmt.Sprintf("Generated configuration for %s (%s)", serverDef.DisplayName, runtimeName))
			} else {
				result.ServersSkipped++
			}
		}
	}

	// If no servers were generated, fall back to default configuration
	if len(gatewayConfig.Servers) == 0 {
		result.Warnings = append(result.Warnings, "No compatible runtimes detected, using default configuration")
		defaultResult, err := g.GenerateDefault()
		if err != nil {
			result.Issues = append(result.Issues, fmt.Sprintf("Failed to generate default configuration: %v", err))
			result.Duration = time.Since(startTime)
			return result, fmt.Errorf("default configuration generation failed: %w", err)
		}
		gatewayConfig = defaultResult.Config
		result.ServersGenerated = defaultResult.ServersGenerated
		result.Messages = append(result.Messages, "Used default configuration as fallback")
	}

	// Validate generated configuration
	if err := gatewayConfig.Validate(); err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("Generated configuration is invalid: %v", err))
		result.Duration = time.Since(startTime)
		return result, fmt.Errorf("configuration validation failed: %w", err)
	}

	result.Config = gatewayConfig
	result.Duration = time.Since(startTime)

	// Populate metadata
	result.Metadata["generation_method"] = "auto_detected"
	result.Metadata["runtimes_detected"] = len(detectionReport.Runtimes)
	result.Metadata["runtimes_installed"] = detectionReport.Summary.InstalledRuntimes
	result.Metadata["runtimes_compatible"] = detectionReport.Summary.CompatibleRuntimes
	result.Metadata["detection_success_rate"] = detectionReport.Summary.SuccessRate
	result.Metadata["detection_duration"] = detectionReport.Duration

	g.logger.UserSuccess(fmt.Sprintf("Auto-detection completed: %d servers generated from %d detected runtimes",
		result.ServersGenerated, detectionReport.Summary.InstalledRuntimes))

	return result, nil
}

// GenerateFromDetectedWithInstallResults generates configuration using installation results instead of verification
func (g *DefaultConfigGenerator) GenerateFromDetectedWithInstallResults(ctx context.Context, detectedRuntimes map[string]*RuntimeInfo, serverInstallResults map[string]*types.InstallResult) (*ConfigGenerationResult, error) {
	startTime := time.Now()

	g.logger.WithOperation("generate-from-install-results").Info("Starting configuration generation from installation results")

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

	// Create base configuration
	gatewayConfig := &config.GatewayConfig{
		Port:                  8080,
		MaxConcurrentRequests: 100,
		Servers:               []config.ServerConfig{},
	}

	// Use installation results instead of detection
	successfulInstalls := 0
	for serverName, installResult := range serverInstallResults {
		if !installResult.Success {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Skipping %s: installation failed", serverName))
			result.ServersSkipped++
			continue
		}

		// Get runtime info for this server
		runtimeInfo := g.getRuntimeInfoForServer(serverName, detectedRuntimes)
		if runtimeInfo == nil {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Skipping %s: runtime not detected", serverName))
			result.ServersSkipped++
			continue
		}

		// Get server definition
		serverDef, err := g.serverRegistry.GetServer(serverName)
		if err != nil {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Skipping %s: server definition not found", serverName))
			result.ServersSkipped++
			continue
		}

		// Generate server config from installation result
		serverConfig := g.generateServerConfigFromInstallResult(installResult, serverDef)
		if serverConfig != nil {
			gatewayConfig.Servers = append(gatewayConfig.Servers, *serverConfig)
			result.ServersGenerated++
			successfulInstalls++
			result.Messages = append(result.Messages,
				fmt.Sprintf("Generated configuration for %s (installed: %s)", serverDef.DisplayName, installResult.Version))
		} else {
			result.ServersSkipped++
		}
	}

	// If no servers were generated, fall back to default configuration
	if len(gatewayConfig.Servers) == 0 {
		result.Warnings = append(result.Warnings, "No successful server installations found, using default configuration")
		defaultResult, err := g.GenerateDefault()
		if err != nil {
			result.Issues = append(result.Issues, fmt.Sprintf("Failed to generate default configuration: %v", err))
			result.Duration = time.Since(startTime)
			return result, fmt.Errorf("default configuration generation failed: %w", err)
		}
		gatewayConfig = defaultResult.Config
		result.ServersGenerated = defaultResult.ServersGenerated
		result.Messages = append(result.Messages, "Used default configuration as fallback")
	}

	// Validate generated configuration
	if err := gatewayConfig.Validate(); err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("Generated configuration is invalid: %v", err))
		result.Duration = time.Since(startTime)
		return result, fmt.Errorf("configuration validation failed: %w", err)
	}

	result.Config = gatewayConfig
	result.Duration = time.Since(startTime)

	// Populate metadata
	result.Metadata["generation_method"] = "install_results_based"
	result.Metadata["successful_installs"] = successfulInstalls
	result.Metadata["total_install_results"] = len(serverInstallResults)
	result.Metadata["runtimes_detected"] = len(detectedRuntimes)

	g.logger.UserSuccess(fmt.Sprintf("Configuration generated from installation results: %d servers from %d successful installations",
		result.ServersGenerated, successfulInstalls))

	return result, nil
}

// GenerateMultiLanguageConfigWithInstallResults generates multi-language configuration using installation results
func (g *DefaultConfigGenerator) GenerateMultiLanguageConfigWithInstallResults(ctx context.Context, projectPath string, options *GenerationOptions, detectedRuntimes map[string]*RuntimeInfo, serverInstallResults map[string]*types.InstallResult) (*ConfigGenerationResult, error) {
	startTime := time.Now()

	g.logger.WithOperation("generate-multi-language-with-install-results").WithField("project_path", projectPath).Info("Generating multi-language configuration with installation results")

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

	// Apply default options if none provided
	if options == nil {
		options = &GenerationOptions{
			OptimizationMode:   config.PerformanceProfileDevelopment,
			EnableMultiServer:  true,
			EnableSmartRouting: true,
			ProjectPath:        projectPath,
			PerformanceProfile: "medium",
		}
	}

	// Scan project for language detection
	projectInfo, err := g.projectScanner.ScanProject(projectPath)
	if err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("Project scanning failed: %v", err))
		result.Duration = time.Since(startTime)
		return result, fmt.Errorf("project scanning failed: %w", err)
	}

	// Create base configuration
	gatewayConfig := &config.GatewayConfig{
		Port:                  8080,
		MaxConcurrentRequests: 100,
		Servers:               []config.ServerConfig{},
	}

	// Filter servers based on installation results and target languages
	successfulInstalls := 0
	for serverName, installResult := range serverInstallResults {
		if !installResult.Success {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Skipping %s: installation failed", serverName))
			result.ServersSkipped++
			continue
		}

		// Get server definition and check if it matches target languages
		serverDef, err := g.serverRegistry.GetServer(serverName)
		if err != nil {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Skipping %s: server definition not found", serverName))
			result.ServersSkipped++
			continue
		}

		// Check if server supports any of the target languages
		if options.TargetLanguages != nil && len(options.TargetLanguages) > 0 {
			if !g.serverSupportsTargetLanguages(serverDef, options.TargetLanguages) {
				result.Messages = append(result.Messages,
					fmt.Sprintf("Skipping %s: doesn't support target languages %v", serverName, options.TargetLanguages))
				result.ServersSkipped++
				continue
			}
		}

		// Generate server config from installation result
		serverConfig := g.generateServerConfigFromInstallResult(installResult, serverDef)
		if serverConfig != nil {
			gatewayConfig.Servers = append(gatewayConfig.Servers, *serverConfig)
			result.ServersGenerated++
			successfulInstalls++
			result.Messages = append(result.Messages,
				fmt.Sprintf("Generated configuration for %s (installed: %s)", serverDef.DisplayName, installResult.Version))
		} else {
			result.ServersSkipped++
		}
	}

	// Apply optimization based on mode
	if options.OptimizationMode != "" {
		optimizationManager := config.NewOptimizationManager()
		// Create a minimal MultiLanguageConfig for optimization
		mlConfig := &config.MultiLanguageConfig{
			ProjectInfo: projectInfo,
		}
		if err := optimizationManager.ApplyOptimization(mlConfig, options.OptimizationMode); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Optimization failed: %v", err))
		} else {
			result.Messages = append(result.Messages, fmt.Sprintf("Applied %s optimization", options.OptimizationMode))
		}
	}

	// Apply smart configuration based on project characteristics
	if projectInfo != nil {
		mlConfig := &config.MultiLanguageConfig{ProjectInfo: projectInfo}
		g.applySmartConfiguration(gatewayConfig, mlConfig, options)
	}

	// Validate generated configuration
	if err := gatewayConfig.Validate(); err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("Generated configuration is invalid: %v", err))
		result.Duration = time.Since(startTime)
		return result, fmt.Errorf("configuration validation failed: %w", err)
	}

	result.Config = gatewayConfig
	result.Duration = time.Since(startTime)

	// Populate metadata
	result.Metadata["generation_method"] = "multi_language_install_results"
	result.Metadata["project_type"] = projectInfo.ProjectType
	result.Metadata["successful_installs"] = successfulInstalls
	result.Metadata["optimization_mode"] = options.OptimizationMode
	result.Metadata["smart_routing_enabled"] = options.EnableSmartRouting
	result.Metadata["multi_server_enabled"] = options.EnableMultiServer

	g.logger.UserSuccess(fmt.Sprintf("Multi-language configuration generated from installation results: %d servers for project type %s",
		result.ServersGenerated, projectInfo.ProjectType))

	return result, nil
}

// Helper methods

// getRuntimeInfoForServer finds the runtime info for a given server
func (g *DefaultConfigGenerator) getRuntimeInfoForServer(serverName string, detectedRuntimes map[string]*RuntimeInfo) *RuntimeInfo {
	// Map server names to their runtimes
	serverToRuntime := map[string]string{
		"gopls":                      "go",
		"pylsp":                      "python",
		"typescript-language-server": "nodejs",
		"jdtls":                      "java",
	}

	runtimeName, exists := serverToRuntime[serverName]
	if !exists {
		return nil
	}

	return detectedRuntimes[runtimeName]
}

// generateServerConfigFromInstallResult creates a server config from installation result
func (g *DefaultConfigGenerator) generateServerConfigFromInstallResult(installResult *types.InstallResult, serverDef *ServerDefinition) *config.ServerConfig {
	serverConfig := &config.ServerConfig{
		Name:      serverDef.Name + "-lsp", // Generate config name from server name
		Languages: serverDef.Languages,
		Command:   serverDef.Name,
		Args:      []string{},
		Transport: "stdio", // Default transport
	}

	// Use path from install result if available
	if installResult.Path != "" {
		serverConfig.Command = installResult.Path
	}

	// Apply server-specific configurations
	switch serverDef.Name {
	case "gopls":
		serverConfig.Args = []string{}

	case "pylsp":
		serverConfig.Args = []string{}

	case "typescript-language-server":
		serverConfig.Args = []string{"--stdio"}

	case "jdtls":
		serverConfig.Args = []string{}
	}

	// Apply default config from server definition if available
	if serverDef.DefaultConfig != nil {
		if transport, ok := serverDef.DefaultConfig["transport"].(string); ok && transport != "" {
			serverConfig.Transport = transport
		}
		if args, ok := serverDef.DefaultConfig["args"].([]string); ok && len(args) > 0 {
			serverConfig.Args = args
		}
	}

	return serverConfig
}

// serverSupportsTargetLanguages checks if a server supports any of the target languages
func (g *DefaultConfigGenerator) serverSupportsTargetLanguages(serverDef *ServerDefinition, targetLanguages []string) bool {
	for _, targetLang := range targetLanguages {
		for _, serverLang := range serverDef.Languages {
			if targetLang == serverLang {
				return true
			}
		}
	}
	return false
}

func (g *DefaultConfigGenerator) GenerateDefault() (*ConfigGenerationResult, error) {
	startTime := time.Now()

	result := &ConfigGenerationResult{
		Config:           config.DefaultConfig(),
		ServersGenerated: 1, // Default config has one Go server
		ServersSkipped:   0,
		AutoDetected:     false,
		GeneratedAt:      startTime,
		Duration:         time.Since(startTime),
		Messages:         []string{"Generated default configuration with Go language server"},
		Warnings:         []string{},
		Issues:           []string{},
		Metadata: map[string]interface{}{
			"generation_method": "default",
			"default_server":    "go-lsp",
		},
	}

	return result, nil
}

func (g *DefaultConfigGenerator) UpdateConfig(existing *config.GatewayConfig, updates ConfigUpdates) (*ConfigUpdateResult, error) {
	startTime := time.Now()
	g.logger.WithOperation("update-config").Info("Starting configuration update")

	result := g.initializeUpdateResult(startTime)

	if existing == nil {
		return g.handleNilUpdateConfig(result, startTime)
	}

	updatedConfig := g.cloneExistingConfig(existing)
	g.applyPortUpdates(existing, updates, updatedConfig, result)
	g.applyServerUpdates(updates, updatedConfig, result)

	if err := g.validateUpdatedConfig(updatedConfig, result, startTime); err != nil {
		return result, err
	}

	g.finalizeUpdateResult(existing, updatedConfig, updates, result, startTime)
	g.logUpdateCompletion(result)

	return result, nil
}

func (g *DefaultConfigGenerator) initializeUpdateResult(startTime time.Time) *ConfigUpdateResult {
	return &ConfigUpdateResult{
		Config:         nil,
		UpdatesApplied: ConfigUpdatesSummary{},
		UpdatedAt:      startTime,
		Messages:       []string{},
		Warnings:       []string{},
		Issues:         []string{},
		Metadata:       make(map[string]interface{}),
	}
}

func (g *DefaultConfigGenerator) handleNilUpdateConfig(result *ConfigUpdateResult, startTime time.Time) (*ConfigUpdateResult, error) {
	result.Issues = append(result.Issues, "Existing configuration cannot be nil")
	result.Duration = time.Since(startTime)
	return result, fmt.Errorf("existing configuration cannot be nil")
}

func (g *DefaultConfigGenerator) cloneExistingConfig(existing *config.GatewayConfig) *config.GatewayConfig {
	updatedConfig := &config.GatewayConfig{
		Port:    existing.Port,
		Servers: make([]config.ServerConfig, len(existing.Servers)),
	}
	copy(updatedConfig.Servers, existing.Servers)
	return updatedConfig
}

func (g *DefaultConfigGenerator) applyPortUpdates(existing *config.GatewayConfig, updates ConfigUpdates, updatedConfig *config.GatewayConfig, result *ConfigUpdateResult) {
	if updates.Port > 0 && updates.Port != existing.Port {
		updatedConfig.Port = updates.Port
		result.UpdatesApplied.PortChanged = true
		result.Messages = append(result.Messages, fmt.Sprintf("Updated port from %d to %d", existing.Port, updates.Port))
	}
}

func (g *DefaultConfigGenerator) applyServerUpdates(updates ConfigUpdates, updatedConfig *config.GatewayConfig, result *ConfigUpdateResult) {
	if updates.ReplaceAllServers {
		g.replaceAllServers(updates, updatedConfig, result)
	} else {
		g.applyIncrementalServerUpdates(updates, updatedConfig, result)
	}
}

func (g *DefaultConfigGenerator) replaceAllServers(updates ConfigUpdates, updatedConfig *config.GatewayConfig, result *ConfigUpdateResult) {
	updatedConfig.Servers = make([]config.ServerConfig, len(updates.AddServers))
	copy(updatedConfig.Servers, updates.AddServers)

	result.UpdatesApplied.ServersReplaced = true
	result.UpdatesApplied.ServersAdded = len(updates.AddServers)
	result.Messages = append(result.Messages, fmt.Sprintf("Replaced all servers with %d new servers", len(updates.AddServers)))

	for _, server := range updates.AddServers {
		result.UpdatesApplied.AddedServerNames = append(result.UpdatesApplied.AddedServerNames, server.Name)
	}
}

func (g *DefaultConfigGenerator) applyIncrementalServerUpdates(updates ConfigUpdates, updatedConfig *config.GatewayConfig, result *ConfigUpdateResult) {
	g.removeServers(updates.RemoveServers, updatedConfig, result)
	g.updateServers(updates.UpdateServers, updatedConfig, result)
	g.addServers(updates.AddServers, updatedConfig, result)
}

func (g *DefaultConfigGenerator) removeServers(removeServers []string, updatedConfig *config.GatewayConfig, result *ConfigUpdateResult) {
	for _, serverName := range removeServers {
		g.removeServer(serverName, updatedConfig, result)
	}
}

func (g *DefaultConfigGenerator) removeServer(serverName string, updatedConfig *config.GatewayConfig, result *ConfigUpdateResult) {
	newServers := []config.ServerConfig{}
	removed := false

	for _, server := range updatedConfig.Servers {
		if server.Name != serverName {
			newServers = append(newServers, server)
		} else {
			removed = true
		}
	}

	if removed {
		updatedConfig.Servers = newServers
		result.UpdatesApplied.ServersRemoved++
		result.UpdatesApplied.RemovedServerNames = append(result.UpdatesApplied.RemovedServerNames, serverName)
		result.Messages = append(result.Messages, fmt.Sprintf("Removed server: %s", serverName))
	} else {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Server %s not found for removal", serverName))
	}
}

func (g *DefaultConfigGenerator) updateServers(updateServers []config.ServerConfig, updatedConfig *config.GatewayConfig, result *ConfigUpdateResult) {
	for _, updateServer := range updateServers {
		g.updateServer(updateServer, updatedConfig, result)
	}
}

func (g *DefaultConfigGenerator) updateServer(updateServer config.ServerConfig, updatedConfig *config.GatewayConfig, result *ConfigUpdateResult) {
	updated := false
	for i, server := range updatedConfig.Servers {
		if server.Name == updateServer.Name {
			updatedConfig.Servers[i] = updateServer
			updated = true
			result.UpdatesApplied.ServersUpdated++
			result.UpdatesApplied.UpdatedServerNames = append(result.UpdatesApplied.UpdatedServerNames, updateServer.Name)
			result.Messages = append(result.Messages, fmt.Sprintf("Updated server: %s", updateServer.Name))
			break
		}
	}

	if !updated {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Server %s not found for update", updateServer.Name))
	}
}

func (g *DefaultConfigGenerator) addServers(addServers []config.ServerConfig, updatedConfig *config.GatewayConfig, result *ConfigUpdateResult) {
	for _, addServer := range addServers {
		g.addServer(addServer, updatedConfig, result)
	}
}

func (g *DefaultConfigGenerator) addServer(addServer config.ServerConfig, updatedConfig *config.GatewayConfig, result *ConfigUpdateResult) {
	for _, server := range updatedConfig.Servers {
		if server.Name == addServer.Name {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Server %s already exists, skipping add", addServer.Name))
			return
		}
	}

	updatedConfig.Servers = append(updatedConfig.Servers, addServer)
	result.UpdatesApplied.ServersAdded++
	result.UpdatesApplied.AddedServerNames = append(result.UpdatesApplied.AddedServerNames, addServer.Name)
	result.Messages = append(result.Messages, fmt.Sprintf("Added server: %s", addServer.Name))
}

func (g *DefaultConfigGenerator) validateUpdatedConfig(updatedConfig *config.GatewayConfig, result *ConfigUpdateResult, startTime time.Time) error {
	if err := updatedConfig.Validate(); err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("Updated configuration is invalid: %v", err))
		result.Duration = time.Since(startTime)
		return fmt.Errorf("updated configuration validation failed: %w", err)
	}
	return nil
}

func (g *DefaultConfigGenerator) finalizeUpdateResult(existing, updatedConfig *config.GatewayConfig, updates ConfigUpdates, result *ConfigUpdateResult, startTime time.Time) {
	result.Config = updatedConfig
	result.Duration = time.Since(startTime)

	result.Metadata["original_servers"] = len(existing.Servers)
	result.Metadata["final_servers"] = len(updatedConfig.Servers)
	result.Metadata["original_port"] = existing.Port
	result.Metadata["final_port"] = updatedConfig.Port
	result.Metadata["update_method"] = "merge"
	if updates.ReplaceAllServers {
		result.Metadata["update_method"] = "replace_all"
	}
}

func (g *DefaultConfigGenerator) logUpdateCompletion(result *ConfigUpdateResult) {
	g.logger.UserSuccess(fmt.Sprintf("Configuration updated successfully: %d messages, %d warnings", len(result.Messages), len(result.Warnings)))
	g.logger.WithFields(map[string]interface{}{
		"servers_added":   result.UpdatesApplied.ServersAdded,
		"servers_removed": result.UpdatesApplied.ServersRemoved,
		"servers_updated": result.UpdatesApplied.ServersUpdated,
		"port_changed":    result.UpdatesApplied.PortChanged,
		"duration":        result.Duration,
	}).Info("Configuration update completed")
}

func (g *DefaultConfigGenerator) ValidateConfig(config *config.GatewayConfig) (*ConfigValidationResult, error) {
	startTime := time.Now()
	g.logger.WithOperation("validate-config").Info("Starting configuration validation")

	result := g.initializeValidationResult(startTime)

	if config == nil {
		return g.handleNilConfig(result, startTime)
	}

	g.validateBasicConfiguration(config, result)
	g.validatePortConfiguration(config, result)
	g.validateServerConfigurations(config, result)
	languageToServers := g.buildLanguageServerMapping(config)
	g.validateLanguageMapping(languageToServers, result)
	g.validateServerPresence(config, result)

	g.finalizeValidationResult(config, result, languageToServers, startTime)
	g.logValidationCompletion(result)

	return result, nil
}

func (g *DefaultConfigGenerator) initializeValidationResult(startTime time.Time) *ConfigValidationResult {
	return &ConfigValidationResult{
		Valid:            true,
		Issues:           []string{},
		Warnings:         []string{},
		ServersValidated: 0,
		ServerIssues:     make(map[string][]string),
		ValidatedAt:      startTime,
		Metadata:         make(map[string]interface{}),
	}
}

func (g *DefaultConfigGenerator) handleNilConfig(result *ConfigValidationResult, startTime time.Time) (*ConfigValidationResult, error) {
	result.Valid = false
	result.Issues = append(result.Issues, "Configuration cannot be nil")
	result.Duration = time.Since(startTime)
	return result, fmt.Errorf("configuration cannot be nil")
}

func (g *DefaultConfigGenerator) validateBasicConfiguration(config *config.GatewayConfig, result *ConfigValidationResult) {
	if err := config.Validate(); err != nil {
		result.Valid = false
		result.Issues = append(result.Issues, fmt.Sprintf("Basic validation failed: %v", err))
	}
}

func (g *DefaultConfigGenerator) validatePortConfiguration(config *config.GatewayConfig, result *ConfigValidationResult) {
	if config.Port < 1 || config.Port > 65535 {
		result.Valid = false
		result.Issues = append(result.Issues, fmt.Sprintf("Invalid port %d: must be between 1 and 65535", config.Port))
		return
	}

	g.checkCommonPortWarnings(config.Port, result)
}

func (g *DefaultConfigGenerator) checkCommonPortWarnings(port int, result *ConfigValidationResult) {
	commonPorts := map[int]string{
		22:    "SSH",
		80:    "HTTP",
		443:   "HTTPS",
		3000:  "Common development server",
		5432:  "PostgreSQL",
		3306:  "MySQL",
		6379:  "Redis",
		27017: "MongoDB",
	}

	if service, exists := commonPorts[port]; exists {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Port %d is commonly used by %s", port, service))
	}
}

func (g *DefaultConfigGenerator) validateServerConfigurations(config *config.GatewayConfig, result *ConfigValidationResult) {
	for _, serverConfig := range config.Servers {
		result.ServersValidated++
		serverIssues := g.validateIndividualServer(serverConfig, result)

		if len(serverIssues) > 0 {
			result.ServerIssues[serverConfig.Name] = serverIssues
		}
	}
}

func (g *DefaultConfigGenerator) validateIndividualServer(serverConfig config.ServerConfig, result *ConfigValidationResult) []string {
	var serverIssues []string

	serverIssues = append(serverIssues, g.validateServerBasics(serverConfig, result)...)
	serverIssues = append(serverIssues, g.validateServerLanguages(serverConfig)...)
	g.validateServerInstallation(serverConfig, result)

	return serverIssues
}

func (g *DefaultConfigGenerator) validateServerBasics(serverConfig config.ServerConfig, result *ConfigValidationResult) []string {
	var issues []string

	if err := serverConfig.Validate(); err != nil {
		issues = append(issues, fmt.Sprintf("Server validation failed: %v", err))
		result.Valid = false
	}

	if len(serverConfig.Languages) == 0 {
		issues = append(issues, "No languages specified")
	}

	return issues
}

func (g *DefaultConfigGenerator) validateServerLanguages(serverConfig config.ServerConfig) []string {
	var issues []string
	langMap := make(map[string]bool)

	for _, lang := range serverConfig.Languages {
		if langMap[lang] {
			issues = append(issues, fmt.Sprintf("Duplicate language: %s", lang))
		}
		langMap[lang] = true
	}

	return issues
}

func (g *DefaultConfigGenerator) validateServerInstallation(serverConfig config.ServerConfig, result *ConfigValidationResult) {
	if g.serverVerifier == nil {
		return
	}

	verificationResult, err := g.serverVerifier.VerifyServer(serverConfig.Command)
	if err != nil {
		return
	}

	g.processServerVerificationResult(verificationResult, serverConfig, result)
}

func (g *DefaultConfigGenerator) processServerVerificationResult(verificationResult *ServerVerificationResult, serverConfig config.ServerConfig, result *ConfigValidationResult) {
	if !verificationResult.Installed {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Server %s (%s) is not installed", serverConfig.Name, serverConfig.Command))
	}
	if !verificationResult.Compatible {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Server %s may not be compatible", serverConfig.Name))
	}
	if !verificationResult.Functional {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Server %s may not be functional", serverConfig.Name))
	}
}

func (g *DefaultConfigGenerator) buildLanguageServerMapping(config *config.GatewayConfig) map[string][]string {
	languageToServers := make(map[string][]string)
	for _, serverConfig := range config.Servers {
		for _, lang := range serverConfig.Languages {
			languageToServers[lang] = append(languageToServers[lang], serverConfig.Name)
		}
	}
	return languageToServers
}

func (g *DefaultConfigGenerator) validateLanguageMapping(languageToServers map[string][]string, result *ConfigValidationResult) {
	for lang, servers := range languageToServers {
		if len(servers) > 1 {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Language %s is supported by multiple servers: %v (first one will be used)", lang, servers))
		}
	}
}

func (g *DefaultConfigGenerator) validateServerPresence(config *config.GatewayConfig, result *ConfigValidationResult) {
	if len(config.Servers) == 0 {
		result.Valid = false
		result.Issues = append(result.Issues, "No servers configured")
	}
}

func (g *DefaultConfigGenerator) finalizeValidationResult(config *config.GatewayConfig, result *ConfigValidationResult, languageToServers map[string][]string, startTime time.Time) {
	result.Duration = time.Since(startTime)

	result.Metadata["total_servers"] = len(config.Servers)
	result.Metadata["total_languages"] = len(languageToServers)
	result.Metadata["port"] = config.Port
	result.Metadata["validation_type"] = "comprehensive"
}

func (g *DefaultConfigGenerator) logValidationCompletion(result *ConfigValidationResult) {
	if result.Valid {
		g.logger.UserSuccess("Configuration validation passed")
	} else {
		g.logger.UserError(fmt.Sprintf("Configuration validation failed: %d issues found", len(result.Issues)))
	}

	g.logger.WithFields(map[string]interface{}{
		"valid":             result.Valid,
		"servers_validated": result.ServersValidated,
		"issues":            len(result.Issues),
		"warnings":          len(result.Warnings),
		"duration":          result.Duration,
	}).Info("Configuration validation completed")
}

func (g *DefaultConfigGenerator) ValidateGenerated(config *config.GatewayConfig) (*ConfigValidationResult, error) {
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

	if err := config.Validate(); err != nil {
		result.Valid = false
		result.Issues = append(result.Issues, fmt.Sprintf("Configuration validation failed: %v", err))
	}

	for _, serverConfig := range config.Servers {
		result.ServersValidated++

		serverIssues := []string{}

		if serverDef, err := g.serverRegistry.GetServer(serverConfig.Command); err == nil {
			verificationResult, err := g.serverVerifier.VerifyServer(serverDef.Name)
			if err != nil {
				serverIssues = append(serverIssues, fmt.Sprintf("Verification failed: %v", err))
			} else {
				if !verificationResult.Installed {
					serverIssues = append(serverIssues, "Server not installed")
					result.Valid = false
				}
				if !verificationResult.Compatible {
					serverIssues = append(serverIssues, "Server version incompatible")
				}
				if !verificationResult.Functional {
					serverIssues = append(serverIssues, "Server not functional")
					result.Warnings = append(result.Warnings,
						fmt.Sprintf("Server %s may not be functional", serverConfig.Name))
				}
			}
		} else {
			serverIssues = append(serverIssues, "Server definition not found")
		}

		if len(serverIssues) > 0 {
			result.ServerIssues[serverConfig.Name] = serverIssues
		}
	}

	result.Duration = time.Since(startTime)
	result.Metadata["validation_type"] = "generated_config"
	return result, nil
}

func (g *DefaultConfigGenerator) generateServerConfig(ctx context.Context, runtimeInfo *RuntimeInfo, serverDef *ServerDefinition) (*config.ServerConfig, error) {
	verificationResult, err := g.serverVerifier.VerifyServer(serverDef.Name)
	if err != nil {
		g.logger.WithError(err).WithField("server", serverDef.Name).Debug("Server verification failed")
		return nil, nil // Return nil to skip this server, not an error
	}

	if !verificationResult.Installed {
		g.logger.WithField("server", serverDef.Name).Debug("Server not installed")
		return nil, nil // Server not installed, skip it
	}

	serverConfig := &config.ServerConfig{
		Name:      serverDef.Name + "-lsp", // Generate config name from server name
		Languages: serverDef.Languages,
		Command:   serverDef.Name,
		Args:      []string{},
		Transport: "stdio", // Default transport
	}

	switch serverDef.Name {
	case SERVER_GOPLS:
		serverConfig.Args = []string{}

	case SERVER_PYLSP:
		// pylsp uses stdio by default, no additional args needed
		serverConfig.Args = []string{}

	case "typescript-language-server":
		serverConfig.Args = []string{"--stdio"}

	case SERVER_JDTLS:
		// Always use the installer path to ensure we get the correct JDTLS executable
		serverConfig.Command = installer.GetJDTLSExecutablePath()
		// Use actual JDTLS args: includes --stdio, -configuration, -data
		serverConfig.Args = installer.GetJDTLSArgs()
	}

	// Skip DefaultConfig override for JDTLS as we use custom path and args
	if serverDef.DefaultConfig != nil && serverDef.Name != SERVER_JDTLS {
		if command, ok := serverDef.DefaultConfig["command"].(string); ok && command != "" {
			serverConfig.Command = command
		}
		if transport, ok := serverDef.DefaultConfig["transport"].(string); ok && transport != "" {
			serverConfig.Transport = transport
		}
		if args, ok := serverDef.DefaultConfig["args"].([]string); ok && len(args) > 0 {
			serverConfig.Args = args
		}
	}

	return serverConfig, nil
}

func (g *DefaultConfigGenerator) initializeTemplates() {
	g.templates[SERVER_GOPLS] = &ServerConfigTemplate{
		RuntimeName:       "go",
		ServerName:        SERVER_GOPLS,
		ConfigName:        "go-lsp",
		Command:           SERVER_GOPLS,
		Args:              []string{},
		Languages:         []string{"go"},
		Transport:         "stdio",
		RequiredRuntime:   "go",
		MinRuntimeVersion: "1.19.0",
	}

	g.templates[SERVER_PYLSP] = &ServerConfigTemplate{
		RuntimeName:       "python",
		ServerName:        SERVER_PYLSP,
		ConfigName:        "python-lsp",
		Command:           "python",
		Args:              []string{"-m", "pylsp"},
		Languages:         []string{"python"},
		Transport:         "stdio",
		RequiredRuntime:   "python",
		MinRuntimeVersion: "3.9.0",
	}

	g.templates["typescript-language-server"] = &ServerConfigTemplate{
		RuntimeName:       "nodejs",
		ServerName:        "typescript-language-server",
		ConfigName:        "typescript-lsp",
		Command:           "typescript-language-server",
		Args:              []string{"--stdio"},
		Languages:         []string{"typescript", "javascript"},
		Transport:         "stdio",
		RequiredRuntime:   "nodejs",
		MinRuntimeVersion: "22.0.0",
	}

	g.templates[SERVER_JDTLS] = &ServerConfigTemplate{
		RuntimeName:       "java",
		ServerName:        SERVER_JDTLS,
		ConfigName:        "java-lsp",
		Command:           installer.GetJDTLSExecutablePath(),
		Args:              installer.GetJDTLSArgs(),
		Languages:         []string{"java"},
		Transport:         "stdio",
		RequiredRuntime:   "java",
		MinRuntimeVersion: "17.0.0",
	}
}

func (g *DefaultConfigGenerator) GetSupportedRuntimes() []string {
	return []string{"go", "python", "nodejs", "java"}
}

func (g *DefaultConfigGenerator) GetSupportedServers() []string {
	servers := make([]string, 0, len(g.templates))
	for serverName := range g.templates {
		servers = append(servers, serverName)
	}
	return servers
}

func (g *DefaultConfigGenerator) GetTemplate(serverName string) (*ServerConfigTemplate, error) {
	if template, exists := g.templates[serverName]; exists {
		return template, nil
	}
	return nil, fmt.Errorf("template not found for server: %s", serverName)
}

func (g *DefaultConfigGenerator) SetLogger(logger *SetupLogger) {
	if logger != nil {
		g.logger = logger
		if g.runtimeDetector != nil {
			g.runtimeDetector.SetLogger(logger)
		}
	}
}

// SetRuntimeDetector sets the runtime detector for testing purposes
func (g *DefaultConfigGenerator) SetRuntimeDetector(detector RuntimeDetector) {
	g.runtimeDetector = detector
	if g.logger != nil {
		detector.SetLogger(g.logger)
	}
}

type DefaultServerVerifier struct{}

func NewDefaultServerVerifier() *DefaultServerVerifier {
	return &DefaultServerVerifier{}
}

func (v *DefaultServerVerifier) VerifyServer(serverName string) (*ServerVerificationResult, error) {
	result := &ServerVerificationResult{
		Installed:  true, // Assume installed for basic implementation
		Compatible: true, // Assume compatible
		Functional: true, // Assume functional
		Path:       "",   // Path not determined in basic implementation
	}
	return result, nil
}

type DefaultServerRegistry struct {
	servers map[string]*ServerDefinition
}

func NewDefaultServerRegistry() *DefaultServerRegistry {
	registry := &DefaultServerRegistry{
		servers: make(map[string]*ServerDefinition),
	}

	registry.initializeServers()

	return registry
}

func (r *DefaultServerRegistry) initializeServers() {
	r.servers[SERVER_GOPLS] = &ServerDefinition{
		Name:        SERVER_GOPLS,
		DisplayName: "Go Language Server",
		Languages:   []string{"go"},
		DefaultConfig: map[string]interface{}{
			"command":   SERVER_GOPLS,
			"transport": "stdio",
			"args":      []string{},
		},
	}

	r.servers[SERVER_PYLSP] = &ServerDefinition{
		Name:        SERVER_PYLSP,
		DisplayName: "Python Language Server",
		Languages:   []string{"python"},
		DefaultConfig: map[string]interface{}{
			"command":   SERVER_PYLSP,
			"transport": "stdio",
			"args":      []string{},
		},
	}

	r.servers["typescript-language-server"] = &ServerDefinition{
		Name:        "typescript-language-server",
		DisplayName: "TypeScript Language Server",
		Languages:   []string{"typescript", "javascript"},
		DefaultConfig: map[string]interface{}{
			"command":   "typescript-language-server",
			"transport": "stdio",
			"args":      []string{"--stdio"},
		},
	}

	r.servers[SERVER_JDTLS] = &ServerDefinition{
		Name:        SERVER_JDTLS,
		DisplayName: "Java Language Server",
		Languages:   []string{"java"},
		DefaultConfig: map[string]interface{}{
			"command":   SERVER_JDTLS,
			"transport": "stdio",
			"args":      []string{},
		},
	}
}

func (r *DefaultServerRegistry) GetServersByRuntime(runtime string) []*ServerDefinition {
	var servers []*ServerDefinition

	runtimeToServers := map[string][]string{
		"go":         {SERVER_GOPLS},
		"python":     {SERVER_PYLSP},
		"nodejs":     {"typescript-language-server"},
		"typescript": {"typescript-language-server"},
		"javascript": {"typescript-language-server"},
		"java":       {SERVER_JDTLS},
	}

	serverNames, exists := runtimeToServers[runtime]
	if !exists {
		return servers
	}

	for _, serverName := range serverNames {
		if server, exists := r.servers[serverName]; exists {
			servers = append(servers, server)
		}
	}

	return servers
}

func (r *DefaultServerRegistry) GetServer(serverName string) (*ServerDefinition, error) {
	if server, exists := r.servers[serverName]; exists {
		return server, nil
	}
	return nil, fmt.Errorf("server not found: %s", serverName)
}
