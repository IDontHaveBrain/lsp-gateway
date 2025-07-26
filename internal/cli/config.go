package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/setup"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

var (
	configFilePath        string
	ConfigOutputPath      string
	ConfigJSON            bool
	ConfigOverwrite       bool
	ConfigAutoDetect      bool
	ConfigValidateOnly    bool
	ConfigIncludeComments bool
	ConfigTargetRuntime   string
	// Multi-language configuration flags
	ConfigMultiLanguage           bool
	ConfigOptimizationMode        string
	ConfigTemplate                string
	ConfigProjectPath             string
	ConfigEnableSmartRouting      bool
	ConfigEnableConcurrentServers bool
	ConfigPerformanceProfile      string
	ConfigProjectDetection        bool
	ConfigComprehensive           bool
	ConfigCheckMultiLang          bool
	ConfigCheckPerformance        bool
	ConfigValidateRouting         bool
	ConfigCheckResourceLimits     bool
	ConfigFromPath                string
	ConfigToPath                  string
	ConfigApplyTuning             bool
)

var ConfigCmd = &cobra.Command{
	Use:   CmdConfig,
	Short: "Configuration management",
	Long: `Manage LSP Gateway configuration files.

The config command provides comprehensive configuration management capabilities,
including generation, validation, and inspection of configuration files.

Available subcommands:
  generate - Generate configuration files
  validate - Validate existing configuration
  show     - Display current configuration
  migrate  - Migrate legacy configuration files
  optimize - Optimize configuration for performance

Examples:
  # Generate default configuration
  lsp-gateway config generate

  # Generate configuration based on detected runtimes
  lsp-gateway config generate --auto-detect

  # Validate existing configuration
  lsp-gateway config validate

  # Show current configuration in JSON format
  lsp-gateway config show --json

  # Generate multi-language configuration
  lsp-gateway config generate --multi-language --project-path /path/to/project

  # Generate optimized configuration
  lsp-gateway config generate --optimization-mode production --enable-smart-routing

  # Validate comprehensive multi-language setup
  lsp-gateway config validate --comprehensive --check-multi-language

  # Migrate legacy configuration
  lsp-gateway config migrate --from legacy-config.yaml --to multi-config.yaml

  # Optimize existing configuration
  lsp-gateway config optimize --mode production --apply-performance-tuning`,
	RunE: ConfigShow, // Default to showing config when no subcommand specified
}

var ConfigGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate configuration files",
	Long: `Generate LSP Gateway configuration files based on system detection or defaults.

This command can generate configuration files in several ways:
1. Auto-detection: Scan system for runtimes and generate config for available servers
2. Target runtime: Generate config for a specific runtime (go, python, nodejs, java)
3. Default: Generate a basic default configuration

The generated configuration will include appropriate server definitions,
language mappings, and transport settings based on detected capabilities.

Examples:
  # Generate configuration with auto-detection
  lsp-gateway config generate --auto-detect

  # Generate configuration for specific runtime
  lsp-gateway config generate --runtime go

  # Generate default configuration
  lsp-gateway config generate

  # Generate to specific file with overwrite
  lsp-gateway config generate --output custom-config.yaml --overwrite

  # Generate with comments for documentation
  lsp-gateway config generate --include-comments

  # Generate multi-language configuration with project detection
  lsp-gateway config generate --multi-language --project-path /path/to/project

  # Generate with optimization mode and template
  lsp-gateway config generate --optimization-mode production --template monorepo

  # Generate with smart routing and concurrent servers
  lsp-gateway config generate --enable-smart-routing --enable-concurrent-servers

  # Generate with performance profile
  lsp-gateway config generate --performance-profile high --project-detection`,
	RunE: ConfigGenerate,
}

var ConfigValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration file",
	Long: `Validate an existing LSP Gateway configuration file.

This command performs comprehensive validation of configuration files including:
- YAML syntax validation
- Schema validation against expected structure
- Server configuration validation
- Runtime dependency checks
- Language server availability checks

The validation includes both basic structural checks and functional verification
where possible (checking if configured servers are actually available).

Examples:
  # Validate default configuration file
  lsp-gateway config validate

  # Validate specific configuration file
  lsp-gateway config validate --config /path/to/config.yaml

  # Validate with JSON output for automation
  lsp-gateway config validate --json

  # Quick syntax check only
  lsp-gateway config validate --validate-only

  # Comprehensive validation with multi-language checks
  lsp-gateway config validate --comprehensive --check-multi-language

  # Validate routing and performance settings
  lsp-gateway config validate --validate-routing --check-performance

  # Check resource limits and multi-language consistency
  lsp-gateway config validate --check-resource-limits --check-multi-language`,
	RunE: ConfigValidate,
}

var ConfigShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Display current configuration",
	Long: `Display the current LSP Gateway configuration.

This command loads and displays the current configuration file, showing:
- Server configurations and their settings
- Port and transport configurations
- Language mappings and file associations
- Any configuration issues or warnings

The output can be formatted as human-readable text or JSON for automation.

Examples:
  # Show current configuration
  lsp-gateway config show

  # Show configuration from specific file
  lsp-gateway config show --config /path/to/config.yaml

  # Show configuration in JSON format
  lsp-gateway config show --json

  # Show configuration with validation information
  lsp-gateway config show --validate`,
	RunE: ConfigShow,
}

// New commands for multi-language configuration
var ConfigMigrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate legacy configuration to multi-language format",
	Long: `Migrate existing legacy configuration files to the new multi-language format.

This command converts older configuration formats to support:
- Multi-language project detection
- Enhanced server configurations
- Smart routing and performance optimizations
- Modern configuration schema

Examples:
  # Migrate legacy configuration
  lsp-gateway config migrate --from legacy-config.yaml --to multi-config.yaml

  # Migrate with automatic optimization
  lsp-gateway config migrate --from old.yaml --to new.yaml --optimization-mode production`,
	RunE: ConfigMigrate,
}

var ConfigOptimizeCmd = &cobra.Command{
	Use:   "optimize",
	Short: "Optimize configuration for performance and multi-language support",
	Long: `Optimize existing configuration files for better performance and multi-language support.

This command applies various optimization strategies including:
- Performance tuning for high-throughput scenarios
- Resource limit adjustments
- Smart routing configuration
- Concurrent server management settings
- Memory and CPU optimizations

Examples:
  # Optimize for production use
  lsp-gateway config optimize --mode production --apply-performance-tuning

  # Optimize with smart routing enabled
  lsp-gateway config optimize --mode production --enable-smart-routing`,
	RunE: ConfigOptimize,
}

func init() {
	ConfigCmd.PersistentFlags().StringVarP(&configFilePath, "config", "c", DefaultConfigFile, "Configuration file path")
	ConfigCmd.PersistentFlags().BoolVar(&ConfigJSON, "json", false, "Output in JSON format")

	// Enhanced ConfigGenerateCmd flags
	ConfigGenerateCmd.Flags().StringVarP(&ConfigOutputPath, "output", "o", "", "Output configuration file path (default: same as input)")
	ConfigGenerateCmd.Flags().BoolVar(&ConfigOverwrite, "overwrite", false, "Overwrite existing configuration file")
	ConfigGenerateCmd.Flags().BoolVar(&ConfigAutoDetect, "auto-detect", false, "Auto-detect runtimes and generate configuration")
	ConfigGenerateCmd.Flags().BoolVar(&ConfigIncludeComments, "include-comments", false, "Include explanatory comments in generated config")
	ConfigGenerateCmd.Flags().StringVar(&ConfigTargetRuntime, "runtime", "", "Generate configuration for specific runtime (go, python, nodejs, java)")
	// Multi-language flags
	ConfigGenerateCmd.Flags().BoolVar(&ConfigMultiLanguage, "multi-language", false, "Enable multi-language configuration features")
	ConfigGenerateCmd.Flags().StringVar(&ConfigOptimizationMode, "optimization-mode", config.PerformanceProfileDevelopment, "Set optimization mode (development, production, analysis)")
	ConfigGenerateCmd.Flags().StringVar(&ConfigTemplate, "template", "", "Use configuration template (monorepo, microservices, single-project)")
	ConfigGenerateCmd.Flags().StringVar(&ConfigProjectPath, "project-path", "", "Project path for multi-language detection")
	ConfigGenerateCmd.Flags().BoolVar(&ConfigEnableSmartRouting, "enable-smart-routing", false, "Enable intelligent request routing")
	ConfigGenerateCmd.Flags().BoolVar(&ConfigEnableConcurrentServers, "enable-concurrent-servers", false, "Enable concurrent server management")
	ConfigGenerateCmd.Flags().StringVar(&ConfigPerformanceProfile, "performance-profile", setup.ProjectComplexityMedium, "Set performance profile (low, medium, high)")
	ConfigGenerateCmd.Flags().BoolVar(&ConfigProjectDetection, "project-detection", false, "Enable automatic project detection")

	// Enhanced ConfigValidateCmd flags
	ConfigValidateCmd.Flags().BoolVar(&ConfigValidateOnly, "validate-only", false, "Perform syntax validation only (skip server verification)")
	ConfigValidateCmd.Flags().BoolVar(&ConfigComprehensive, "comprehensive", false, "Enable comprehensive validation mode")
	ConfigValidateCmd.Flags().BoolVar(&ConfigCheckMultiLang, "check-multi-language", false, "Check multi-language consistency")
	ConfigValidateCmd.Flags().BoolVar(&ConfigCheckPerformance, "check-performance", false, "Validate performance settings")
	ConfigValidateCmd.Flags().BoolVar(&ConfigValidateRouting, "validate-routing", false, "Check routing strategy configuration")
	ConfigValidateCmd.Flags().BoolVar(&ConfigCheckResourceLimits, "check-resource-limits", false, "Validate resource limit settings")

	ConfigShowCmd.Flags().BoolVar(&ConfigValidateOnly, "validate", false, "Include validation information in output")

	// ConfigMigrateCmd flags
	ConfigMigrateCmd.Flags().StringVar(&ConfigFromPath, "from", "", "Source configuration file path")
	ConfigMigrateCmd.Flags().StringVar(&ConfigToPath, "to", "", "Target configuration file path")
	ConfigMigrateCmd.Flags().StringVar(&ConfigOptimizationMode, "optimization-mode", config.PerformanceProfileDevelopment, "Apply optimization mode during migration")
	ConfigMigrateCmd.Flags().BoolVar(&ConfigOverwrite, "overwrite", false, "Overwrite target file if it exists")

	// ConfigOptimizeCmd flags
	ConfigOptimizeCmd.Flags().StringVar(&ConfigOptimizationMode, "mode", config.PerformanceProfileProduction, "Optimization mode (development, production, analysis)")
	ConfigOptimizeCmd.Flags().BoolVar(&ConfigApplyTuning, "apply-performance-tuning", false, "Apply performance tuning optimizations")
	ConfigOptimizeCmd.Flags().BoolVar(&ConfigEnableSmartRouting, "enable-smart-routing", false, "Enable smart routing optimizations")
	ConfigOptimizeCmd.Flags().BoolVar(&ConfigEnableConcurrentServers, "enable-concurrent-servers", false, "Enable concurrent server optimizations")
	ConfigOptimizeCmd.Flags().BoolVar(&ConfigOverwrite, "overwrite", false, "Overwrite existing configuration file")

	ConfigCmd.AddCommand(ConfigGenerateCmd)
	ConfigCmd.AddCommand(ConfigValidateCmd)
	ConfigCmd.AddCommand(ConfigShowCmd)
	ConfigCmd.AddCommand(ConfigMigrateCmd)
	ConfigCmd.AddCommand(ConfigOptimizeCmd)

	rootCmd.AddCommand(ConfigCmd)
}

func ConfigGenerate(cmd *cobra.Command, args []string) error {
	if err := ValidateConfigGenerateParams(); err != nil {
		return err
	}

	outputPath, err := determineConfigOutputPath()
	if err != nil {
		return err
	}

	cfg, err := generateConfigurationByMode()
	if err != nil {
		return err
	}

	if err := WriteConfigurationFile(cfg, outputPath, ConfigIncludeComments); err != nil {
		return NewPermissionError(outputPath, "write configuration to")
	}

	fmt.Printf("✓ Configuration written to: %s\n", outputPath)
	fmt.Println("Note: Advanced configuration generation will be available once setup integration is complete.")

	return nil
}

func ConfigValidate(cmd *cobra.Command, args []string) error {
	if err := ValidateConfigValidateParams(); err != nil {
		return err
	}

	fmt.Printf("Validating configuration file: %s\n", configFilePath)

	cfg, err := config.LoadConfig(configFilePath)
	if err != nil {
		if ConfigJSON {
			OutputValidationJSON(false, []string{fmt.Sprintf("Failed to load configuration: %v", err)}, nil)
			return nil
		}
		return HandleConfigError(err, configFilePath)
	}

	issues := []string{}
	warnings := []string{}

	// Basic configuration validation
	if err := cfg.Validate(); err != nil {
		issues = append(issues, fmt.Sprintf("Configuration validation failed: %v", err))
	}

	// Comprehensive validation mode
	if ConfigComprehensive {
		if comprehensiveIssues, comprehensiveWarnings := runComprehensiveValidation(cfg); len(comprehensiveIssues) > 0 || len(comprehensiveWarnings) > 0 {
			issues = append(issues, comprehensiveIssues...)
			warnings = append(warnings, comprehensiveWarnings...)
		}
	}

	// Multi-language consistency checks
	if ConfigCheckMultiLang {
		if multiLangIssues, multiLangWarnings := validateMultiLanguageConsistency(cfg); len(multiLangIssues) > 0 || len(multiLangWarnings) > 0 {
			issues = append(issues, multiLangIssues...)
			warnings = append(warnings, multiLangWarnings...)
		}
	}

	// Performance settings validation
	if ConfigCheckPerformance {
		if perfIssues, perfWarnings := validatePerformanceSettings(cfg); len(perfIssues) > 0 || len(perfWarnings) > 0 {
			issues = append(issues, perfIssues...)
			warnings = append(warnings, perfWarnings...)
		}
	}

	// Routing strategy validation
	if ConfigValidateRouting {
		if routingIssues, routingWarnings := validateRoutingStrategy(cfg); len(routingIssues) > 0 || len(routingWarnings) > 0 {
			issues = append(issues, routingIssues...)
			warnings = append(warnings, routingWarnings...)
		}
	}

	// Resource limits validation
	if ConfigCheckResourceLimits {
		if resourceIssues, resourceWarnings := validateResourceLimits(cfg); len(resourceIssues) > 0 || len(resourceWarnings) > 0 {
			issues = append(issues, resourceIssues...)
			warnings = append(warnings, resourceWarnings...)
		}
	}

	if !ConfigValidateOnly && !ConfigComprehensive && !ConfigCheckMultiLang && !ConfigCheckPerformance && !ConfigValidateRouting && !ConfigCheckResourceLimits {
		warnings = append(warnings, "Extended validation not yet available - basic validation only")
	}

	if ConfigJSON {
		OutputValidationJSON(len(issues) == 0, issues, warnings)
	} else {
		OutputValidationHuman(len(issues) == 0, issues, warnings, cfg)
	}

	if len(issues) > 0 {
		return NewValidationError("configuration", issues)
	}

	return nil
}

func ConfigShow(cmd *cobra.Command, args []string) error {
	if err := ValidateConfigShowParams(); err != nil {
		return err
	}

	cfg, err := config.LoadConfig(configFilePath)
	if err != nil {
		return HandleConfigError(err, configFilePath)
	}

	var validationResult interface{}
	if ConfigValidateOnly {
		if err := cfg.Validate(); err != nil {
			validationResult = map[string]interface{}{
				"valid":  false,
				"errors": []string{err.Error()},
			}
		} else {
			validationResult = map[string]interface{}{
				"valid":  true,
				"errors": []string{},
			}
		}
	}

	if ConfigJSON {
		return OutputConfigJSON(cfg, validationResult)
	}

	return OutputConfigHuman(cfg, validationResult)
}

func determineConfigOutputPath() (string, error) {
	outputPath := ConfigOutputPath
	if outputPath == "" {
		outputPath = configFilePath
	}

	if _, err := os.Stat(outputPath); err == nil && !ConfigOverwrite {
		return "", &CLIError{
			Type:    ErrorTypeConfig,
			Message: fmt.Sprintf("Configuration file %s already exists", outputPath),
			Suggestions: []string{
				"Use --overwrite flag: lsp-gateway config generate --overwrite",
				"Choose different output file: lsp-gateway config generate --output config-new.yaml",
				fmt.Sprintf("Remove existing file: rm %s", outputPath),
				"Backup existing file before overwriting",
			},
			RelatedCmds: []string{
				"config generate --overwrite",
				"config show",
			},
		}
	}

	return outputPath, nil
}

func generateConfigurationByMode() (*config.GatewayConfig, error) {
	// Multi-language configuration generation
	if ConfigMultiLanguage {
		fmt.Println("Generating multi-language configuration...")
		return generateMultiLanguageConfig()
	}

	// Template-based configuration generation
	if ConfigTemplate != "" {
		fmt.Printf("Generating configuration from template: %s...\n", ConfigTemplate)
		return generateTemplateBasedConfig()
	}

	// Project detection-based generation
	if ConfigProjectDetection && ConfigProjectPath != "" {
		fmt.Printf("Generating configuration with project detection for: %s...\n", ConfigProjectPath)
		return generateProjectDetectedConfig()
	}

	// Auto-detect mode (enhanced)
	if ConfigAutoDetect {
		fmt.Println("Detecting runtimes and generating enhanced configuration...")
		return generateAutoDetectedConfig()
	}

	// Runtime-specific generation
	if ConfigTargetRuntime != "" {
		if err := validateTargetRuntime(); err != nil {
			return nil, err
		}
		fmt.Printf("Generating configuration for %s runtime...\n", ConfigTargetRuntime)
		return generateRuntimeSpecificConfig()
	}

	fmt.Println("Generating default configuration...")
	return config.DefaultConfig(), nil
}

func validateTargetRuntime() error {
	supportedRuntimes := []string{"go", config.LANG_PYTHON, "nodejs", "java"}
	for _, supported := range supportedRuntimes {
		if ConfigTargetRuntime == supported {
			return nil
		}
	}
	return NewRuntimeNotFoundError(ConfigTargetRuntime)
}

func WriteConfigurationFile(cfg *config.GatewayConfig, path string, includeComments bool) error {
	file, err := os.Create(path)
	if err != nil {
		return NewFileOperationError("create", path, err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("Warning: failed to close file %s: %v", path, err)
		}
	}()

	if includeComments {
		if err := writeConfigHeader(file, path); err != nil {
			return err
		}
	}

	return writeConfigYAML(file, cfg, path)
}

func writeConfigHeader(file *os.File, path string) error {
	header := `# LSP Gateway Configuration
# Generated by lsp-gateway config generate
#
# This configuration file defines the LSP Gateway server settings,
# including port configuration and language server definitions.
#
# For more information, see: https://github.com/your-repo/lsp-gateway

`
	if _, err := file.WriteString(header); err != nil {
		return NewFileOperationError("write header to", path, err)
	}
	return nil
}

func writeConfigYAML(file *os.File, cfg *config.GatewayConfig, path string) error {
	encoder := yaml.NewEncoder(file, yaml.Indent(2))
	defer func() {
		if err := encoder.Close(); err != nil {
			log.Printf("Warning: failed to close YAML encoder for %s: %v", path, err)
		}
	}()

	if err := encoder.Encode(cfg); err != nil {
		return NewFileOperationError("encode YAML to", path, err)
	}

	return nil
}

func OutputValidationJSON(valid bool, issues []string, warnings []string) {
	result := createValidationResult(valid, issues, warnings)
	printJSONOutput(result)
}

func createValidationResult(valid bool, issues []string, warnings []string) map[string]interface{} {
	return map[string]interface{}{
		"valid":    valid,
		"issues":   issues,
		"warnings": warnings,
	}
}

func printJSONOutput(data interface{}) {
	output, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		fmt.Printf("Error: Failed to format JSON output: %v\n", err)
		// Fallback to basic JSON marshaling
		if basicOutput, basicErr := json.Marshal(data); basicErr == nil {
			fmt.Println(string(basicOutput))
		} else {
			fmt.Printf("Error: Failed to marshal data: %v\n", basicErr)
		}
		return
	}
	fmt.Println(string(output))
}

func OutputValidationHuman(valid bool, issues []string, warnings []string, cfg *config.GatewayConfig) {
	printSectionHeader("Configuration Validation")
	printValidationStatus(valid)
	printConfigSummary(cfg)
	printValidationDetails(issues, warnings, valid)
}

func printSectionHeader(title string) {
	fmt.Println()
	fmt.Println("=======================================================")
	fmt.Printf("  %s\n", title)
	fmt.Println("=======================================================")
	fmt.Println()
}

func printValidationStatus(valid bool) {
	if valid {
		fmt.Println("✓ Configuration is valid")
	} else {
		fmt.Println("✗ Configuration has issues")
	}
}

func printConfigSummary(cfg *config.GatewayConfig) {
	fmt.Printf("Servers configured: %d\n", len(cfg.Servers))
	fmt.Printf("Port: %d\n", cfg.Port)
	fmt.Println()
}

func printValidationDetails(issues []string, warnings []string, valid bool) {
	printIssuesList(issues)
	printWarningsList(warnings)

	if valid {
		fmt.Println("The configuration is ready to use.")
	}
}

func printIssuesList(issues []string) {
	if len(issues) > 0 {
		fmt.Println("Issues found:")
		for _, issue := range issues {
			fmt.Printf("  ✗ %s\n", issue)
		}
		fmt.Println()
	}
}

func printWarningsList(warnings []string) {
	if len(warnings) > 0 {
		fmt.Println("Warnings:")
		for _, warning := range warnings {
			fmt.Printf("  ⚠ %s\n", warning)
		}
		fmt.Println()
	}
}

func OutputConfigJSON(cfg *config.GatewayConfig, validation interface{}) error {
	output := map[string]interface{}{
		"config": cfg,
	}

	if validation != nil {
		output["validation"] = validation
	}

	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return NewJSONMarshalError("configuration", err)
	}

	fmt.Println(string(jsonData))
	return nil
}

func OutputConfigHuman(cfg *config.GatewayConfig, validation interface{}) error {
	title := fmt.Sprintf("LSP Gateway Configuration (%s)", configFilePath)
	printSectionHeader(title)
	printConfigDetails(cfg)
	printServersList(cfg.Servers)
	printValidationInfo(validation)
	return nil
}

func printConfigDetails(cfg *config.GatewayConfig) {
	fmt.Printf("Server Port: %d\n", cfg.Port)
	fmt.Printf("Configured Servers: %d\n", len(cfg.Servers))
	fmt.Println()
}

func printServersList(servers []config.ServerConfig) {
	if len(servers) > 0 {
		fmt.Println("Language Servers:")
		fmt.Println("─────────────────")
		for i, server := range servers {
			printServerInfo(i+1, server)
		}
	}
}

func printServerInfo(index int, server config.ServerConfig) {
	fmt.Printf("%d. %s\n", index, server.Name)
	fmt.Printf("   Command: %s\n", server.Command)
	if len(server.Args) > 0 {
		fmt.Printf("   Arguments: %s\n", strings.Join(server.Args, " "))
	}
	fmt.Printf("   Transport: %s\n", server.Transport)
	fmt.Printf("   Languages: %s\n", strings.Join(server.Languages, ", "))
	fmt.Println()
}

func printValidationInfo(validation interface{}) {
	if validation != nil {
		fmt.Println("Validation Status:")
		fmt.Println("─────────────────")
		fmt.Println("⚠ Extended validation not yet available")
	}
}

func ValidateConfigGenerateParams() error {
	result := ValidateMultiple(
		func() *ValidationError {
			return ValidateRuntimeName(ConfigTargetRuntime, "runtime")
		},
		func() *ValidationError {
			if ConfigOutputPath != "" {
				return ValidateFilePath(ConfigOutputPath, "output", "create")
			}
			return nil
		},
		func() *ValidationError {
			if ConfigOutputPath == "" {
				return ValidateFilePath(configFilePath, "config", "create")
			}
			return nil
		},
	)
	if result == nil {
		return nil
	}
	return result
}

func ValidateConfigValidateParams() error {
	result := ValidateMultiple(
		func() *ValidationError {
			return ValidateFilePath(configFilePath, "config", "read")
		},
	)
	if result == nil {
		return nil
	}
	return result
}

func ValidateConfigShowParams() error {
	result := ValidateMultiple(
		func() *ValidationError {
			return ValidateFilePath(configFilePath, "config", "read")
		},
	)
	if result == nil {
		return nil
	}
	return result
}

// Multi-language configuration generation functions

func generateMultiLanguageConfig() (*config.GatewayConfig, error) {
	// Initialize enhanced configuration generator
	enhancedGen := setup.NewEnhancedConfigurationGenerator()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Generate configuration based on project path if provided
	if ConfigProjectPath != "" {
		result, err := enhancedGen.GenerateFromProject(ctx, ConfigProjectPath, ConfigOptimizationMode)
		if err != nil {
			return nil, fmt.Errorf("failed to generate multi-language configuration from project: %w", err)
		}
		
		// result.Config is already a *config.GatewayConfig, no conversion needed
		gatewayConfig := result.Config
		
		// Apply additional CLI flags
		applyCliFlags(gatewayConfig)
		
		languageCount := 0
		if gatewayConfig.ProjectContext != nil {
			languageCount = len(gatewayConfig.ProjectContext.Languages)
		}
		
		fmt.Printf("✓ Multi-language configuration generated: %d servers, %d languages detected\n", 
			result.ServersGenerated, languageCount)
		
		return gatewayConfig, nil
	}

	// Generate environment-based configuration
	environment := getEnvironmentFromOptimizationMode(ConfigOptimizationMode)
	result, err := enhancedGen.GenerateDefaultForEnvironment(ctx, environment)
	if err != nil {
		return nil, fmt.Errorf("failed to generate environment-based configuration: %w", err)
	}

	// result.Config is already a *config.GatewayConfig, no conversion needed
	gatewayConfig := result.Config

	// Apply additional CLI flags
	applyCliFlags(gatewayConfig)

	fmt.Printf("✓ Multi-language configuration generated for %s environment\n", environment)

	return gatewayConfig, nil
}

func generateTemplateBasedConfig() (*config.GatewayConfig, error) {
	// Initialize template manager
	templateManager := setup.NewConfigurationTemplateManager()

	// Get the specified template
	template, err := templateManager.GetTemplate(ConfigTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to get template '%s': %w", ConfigTemplate, err)
	}

	// Create base configuration from template
	cfg := config.DefaultConfig()

	// Apply template-specific configurations
	cfg.Port = template.DefaultPort
	if template.ResourceLimits != nil {
		cfg.MaxConcurrentRequests = template.ResourceLimits.MaxConcurrentRequests
		if template.ResourceLimits.TimeoutSeconds > 0 {
			cfg.Timeout = fmt.Sprintf("%ds", template.ResourceLimits.TimeoutSeconds)
		}
	}

	// Apply smart routing configuration
	if template.EnableSmartRouting {
		cfg.EnableSmartRouting = true
		if template.RoutingConfig != nil {
			// Initialize smart router config if needed
			if cfg.SmartRouterConfig == nil {
				cfg.SmartRouterConfig = &config.SmartRouterConfig{}
			}
			cfg.SmartRouterConfig.DefaultStrategy = template.RoutingConfig.DefaultStrategy
			cfg.SmartRouterConfig.EnablePerformanceMonitoring = template.RoutingConfig.EnablePerformanceMonitoring
			cfg.SmartRouterConfig.EnableCircuitBreaker = template.RoutingConfig.EnableCircuitBreaker
		}
	}

	// Apply multi-server configuration
	cfg.EnableConcurrentServers = template.EnableMultiServer

	// Apply additional CLI flags
	applyCliFlags(cfg)

	fmt.Printf("✓ Configuration generated from template: %s (v%s)\n", template.Name, template.Version)
	fmt.Printf("  Target languages: %s\n", strings.Join(template.TargetLanguages, ", "))
	fmt.Printf("  Project types: %s\n", strings.Join(template.ProjectTypes, ", "))

	return cfg, nil
}

func generateProjectDetectedConfig() (*config.GatewayConfig, error) {
	// Initialize enhanced configuration generator
	enhancedGen := setup.NewEnhancedConfigurationGenerator()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if ConfigProjectPath == "" {
		return nil, fmt.Errorf("project path is required for project detection")
	}

	fmt.Printf("Analyzing project structure at: %s\n", ConfigProjectPath)

	// Generate configuration from project analysis
	result, err := enhancedGen.GenerateFromProject(ctx, ConfigProjectPath, ConfigOptimizationMode)
	if err != nil {
		return nil, fmt.Errorf("failed to generate configuration from project: %w", err)
	}

	// result.Config is already a *config.GatewayConfig, no conversion needed
	gatewayConfig := result.Config

	// Apply CLI flags
	applyCliFlags(gatewayConfig)

	languageCount := 0
	projectType := "unknown"
	if gatewayConfig.ProjectContext != nil {
		languageCount = len(gatewayConfig.ProjectContext.Languages)
		projectType = gatewayConfig.ProjectContext.ProjectType
	}

	fmt.Printf("✓ Project analysis complete:\n")
	fmt.Printf("  Languages detected: %d\n", languageCount)
	fmt.Printf("  Servers generated: %d\n", result.ServersGenerated)
	fmt.Printf("  Project type: %s\n", projectType)

	return gatewayConfig, nil
}

func generateAutoDetectedConfig() (*config.GatewayConfig, error) {
	// Initialize enhanced configuration generator
	enhancedGen := setup.NewEnhancedConfigurationGenerator()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Determine environment from optimization mode
	environment := getEnvironmentFromOptimizationMode(ConfigOptimizationMode)

	fmt.Printf("Generating auto-detected configuration for %s environment...\n", environment)

	// Generate environment-based configuration with auto-detection
	result, err := enhancedGen.GenerateDefaultForEnvironment(ctx, environment)
	if err != nil {
		return nil, fmt.Errorf("failed to generate auto-detected configuration: %w", err)
	}

	// result.Config is already a *config.GatewayConfig, no conversion needed
	gatewayConfig := result.Config
	// Apply CLI flags and performance profile
	applyCliFlags(gatewayConfig)

	fmt.Printf("✓ Auto-detected configuration generated:\n")
	fmt.Printf("  Environment: %s\n", environment)
	fmt.Printf("  Performance profile: %s\n", ConfigPerformanceProfile)
	fmt.Printf("  Servers configured: %d\n", len(gatewayConfig.Servers))

	return gatewayConfig, nil
}

func generateRuntimeSpecificConfig() (*config.GatewayConfig, error) {
	// Initialize multi-language configuration generator
	multiLangGen := config.NewConfigGenerator()

	fmt.Printf("Generating configuration for %s runtime...\n", ConfigTargetRuntime)

	// Get template for the target runtime
	template, exists := multiLangGen.GetTemplate(ConfigTargetRuntime)
	if !exists {
		return nil, fmt.Errorf("no configuration template found for runtime: %s", ConfigTargetRuntime)
	}

	// Create basic gateway configuration
	cfg := config.DefaultConfig()

	// Create a server configuration from the template
	serverConfig := config.ServerConfig{
		Name:        template.Name,
		Languages:   []string{ConfigTargetRuntime},
		Command:     template.Command,
		Args:        template.Args,
		Transport:   template.Transport,
		RootMarkers: template.RootMarkers,
		Settings:    template.Settings,
		Priority:    template.Priority,
		Weight:      template.Weight,
	}

	cfg.Servers = []config.ServerConfig{serverConfig}

	// Apply CLI flags and optimization
	applyCliFlags(cfg)

	// Apply optimization if requested
	if ConfigOptimizationMode != config.PerformanceProfileDevelopment {
		optimizationMgr := config.NewOptimizationManager()

		multiLangConfig := &config.MultiLanguageConfig{
			ServerConfigs: convertGatewayToServerConfigs(cfg),
			WorkspaceConfig: &config.WorkspaceConfig{
				SharedSettings: make(map[string]interface{}),
			},
		}

		if err := optimizationMgr.ApplyOptimization(multiLangConfig, ConfigOptimizationMode); err != nil {
			fmt.Printf("Warning: Failed to apply optimization: %v\n", err)
		} else {
			cfg = convertMultiLanguageToGateway(multiLangConfig)
		}
	}

	fmt.Printf("✓ Runtime-specific configuration generated:\n")
	fmt.Printf("  Runtime: %s\n", ConfigTargetRuntime)
	fmt.Printf("  Server: %s\n", template.Name)
	fmt.Printf("  Optimization: %s\n", ConfigOptimizationMode)

	return cfg, nil
}

// New command implementations

func ConfigMigrate(cmd *cobra.Command, args []string) error {
	if err := ValidateConfigMigrateParams(); err != nil {
		return err
	}

	fmt.Printf("Migrating configuration from %s to %s...\n", ConfigFromPath, ConfigToPath)

	// Load source configuration
	sourceCfg, err := config.LoadConfig(ConfigFromPath)
	if err != nil {
		return fmt.Errorf("failed to load source configuration: %w", err)
	}

	// Enhanced configuration generator would be initialized here if needed for migration
	
	// Create migrated configuration with enhanced features
	migratedCfg := migrateToEnhancedFormat(sourceCfg)

	// Apply optimization during migration if specified
	if ConfigOptimizationMode != config.PerformanceProfileDevelopment {
		optimizationMgr := config.NewOptimizationManager()

		// Create a minimal multi-language config for optimization
		multiLangConfig := &config.MultiLanguageConfig{
			ServerConfigs: convertGatewayToServerConfigs(migratedCfg),
			WorkspaceConfig: &config.WorkspaceConfig{
				SharedSettings: make(map[string]interface{}),
			},
		}

		if err := optimizationMgr.ApplyOptimization(multiLangConfig, ConfigOptimizationMode); err != nil {
			fmt.Printf("Warning: Failed to apply optimization during migration: %v\n", err)
		} else {
			// Convert back to gateway config
			migratedCfg = convertMultiLanguageToGateway(multiLangConfig)
			fmt.Printf("✓ Applied %s optimization during migration\n", ConfigOptimizationMode)
		}
	}

	// Apply CLI flags to migrated configuration
	applyCliFlags(migratedCfg)

	// Write migrated configuration
	if err := WriteConfigurationFile(migratedCfg, ConfigToPath, ConfigIncludeComments); err != nil {
		return NewPermissionError(ConfigToPath, "write migrated configuration to")
	}

	fmt.Printf("✓ Configuration migrated successfully to: %s\n", ConfigToPath)
	fmt.Printf("  Servers: %d\n", len(migratedCfg.Servers))
	fmt.Printf("  Enhanced features: %s\n", getEnhancedFeaturesSummary(migratedCfg))

	return nil
}

func ConfigOptimize(cmd *cobra.Command, args []string) error {
	if err := ValidateConfigOptimizeParams(); err != nil {
		return err
	}

	fmt.Printf("Optimizing configuration for %s mode...\n", ConfigOptimizationMode)

	// Load existing configuration
	cfg, err := config.LoadConfig(configFilePath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Initialize optimization manager
	optimizationMgr := config.NewOptimizationManager()

	// Convert gateway config to multi-language config for optimization
	multiLangConfig := &config.MultiLanguageConfig{
		ServerConfigs: convertGatewayToServerConfigs(cfg),
		WorkspaceConfig: &config.WorkspaceConfig{
			SharedSettings: make(map[string]interface{}),
		},
		OptimizedFor: ConfigOptimizationMode,
		GeneratedAt:  time.Now(),
		Version:      "1.0",
		Metadata:     make(map[string]interface{}),
	}

	// Apply optimizations using the optimization manager
	if err := optimizationMgr.ApplyOptimization(multiLangConfig, ConfigOptimizationMode); err != nil {
		return fmt.Errorf("failed to apply %s optimization: %w", ConfigOptimizationMode, err)
	}

	// Convert back to gateway config
	optimizedCfg := convertMultiLanguageToGateway(multiLangConfig)

	// Apply additional CLI flags
	applyCliFlags(optimizedCfg)

	// Determine output path
	outputPath := configFilePath
	if ConfigOutputPath != "" {
		outputPath = ConfigOutputPath
	}

	// Write optimized configuration
	if err := WriteConfigurationFile(optimizedCfg, outputPath, ConfigIncludeComments); err != nil {
		return NewPermissionError(outputPath, "write optimized configuration to")
	}

	fmt.Printf("✓ Configuration optimized and saved to: %s\n", outputPath)
	fmt.Printf("  Optimization mode: %s\n", ConfigOptimizationMode)
	fmt.Printf("  Servers optimized: %d\n", len(optimizedCfg.Servers))
	fmt.Printf("  Performance settings applied: %v\n", multiLangConfig.Metadata["performance_settings"] != nil)

	return nil
}

// Enhanced validation functions

func ValidateConfigMigrateParams() error {
	result := ValidateMultiple(
		func() *ValidationError {
			if ConfigFromPath == "" {
				return &ValidationError{
					Field:   "from",
					Message: "source configuration path is required",
				}
			}
			return ValidateFilePath(ConfigFromPath, "from", "read")
		},
		func() *ValidationError {
			if ConfigToPath == "" {
				return &ValidationError{
					Field:   "to",
					Message: "target configuration path is required",
				}
			}
			return ValidateFilePath(ConfigToPath, "to", "create")
		},
		func() *ValidationError {
			return ValidateOptimizationMode(ConfigOptimizationMode)
		},
	)
	if result == nil {
		return nil
	}
	return result
}

func ValidateConfigOptimizeParams() error {
	result := ValidateMultiple(
		func() *ValidationError {
			return ValidateFilePath(configFilePath, "config", "read")
		},
		func() *ValidationError {
			return ValidateOptimizationMode(ConfigOptimizationMode)
		},
	)
	if result == nil {
		return nil
	}
	return result
}

func ValidateOptimizationMode(mode string) *ValidationError {
	validModes := []string{config.PerformanceProfileDevelopment, config.PerformanceProfileProduction, config.PerformanceProfileAnalysis}
	for _, validMode := range validModes {
		if mode == validMode {
			return nil
		}
	}
	return &ValidationError{
		Field:   "optimization-mode",
		Message: fmt.Sprintf("invalid optimization mode '%s', must be one of: %v", mode, validModes),
	}
}

// Enhanced validation helper functions

func runComprehensiveValidation(cfg *config.GatewayConfig) ([]string, []string) {
	issues := []string{}
	warnings := []string{}

	// Check server configuration completeness
	if len(cfg.Servers) == 0 {
		issues = append(issues, "No language servers configured")
	}

	// Validate server definitions
	for i, server := range cfg.Servers {
		if server.Name == "" {
			issues = append(issues, fmt.Sprintf("Server %d has no name", i))
		}
		if server.Command == "" {
			issues = append(issues, fmt.Sprintf("Server '%s' has no command specified", server.Name))
		}
		if len(server.Languages) == 0 {
			warnings = append(warnings, fmt.Sprintf("Server '%s' has no languages specified", server.Name))
		}
	}

	// Validate port configuration
	if cfg.Port <= 0 || cfg.Port > 65535 {
		issues = append(issues, fmt.Sprintf("Invalid port number: %d", cfg.Port))
	}

	// Check for common port conflicts
	commonPorts := []int{80, 443, 22, 25, 53, 110, 143, 993, 995}
	for _, commonPort := range commonPorts {
		if cfg.Port == commonPort {
			warnings = append(warnings, fmt.Sprintf("Port %d is commonly used by other services", cfg.Port))
			break
		}
	}

	return issues, warnings
}

func validateMultiLanguageConsistency(cfg *config.GatewayConfig) ([]string, []string) {
	issues := []string{}
	warnings := []string{}

	// Check for language overlap between servers
	languageMap := make(map[string][]string)
	for _, server := range cfg.Servers {
		for _, lang := range server.Languages {
			languageMap[lang] = append(languageMap[lang], server.Name)
		}
	}

	// Report overlapping language support
	for lang, servers := range languageMap {
		if len(servers) > 1 {
			warnings = append(warnings, fmt.Sprintf("Language '%s' is supported by multiple servers: %v", lang, servers))
		}
	}

	// Check for common multi-language project patterns
	supportedLanguages := make(map[string]bool)
	for _, server := range cfg.Servers {
		for _, lang := range server.Languages {
			supportedLanguages[lang] = true
		}
	}

	// Check for common language combinations
	if supportedLanguages[config.LANG_TYPESCRIPT] && !supportedLanguages["javascript"] {
		warnings = append(warnings, "TypeScript support detected but JavaScript support may be missing")
	}

	if supportedLanguages[config.LANG_PYTHON] && len(cfg.Servers) > 1 {
		// Check if there's a Python LSP server configured
		hasPythonLSP := false
		for _, server := range cfg.Servers {
			if strings.Contains(strings.ToLower(server.Command), "pylsp") || strings.Contains(strings.ToLower(server.Command), "pyright") {
				hasPythonLSP = true
				break
			}
		}
		if !hasPythonLSP {
			warnings = append(warnings, "Python support detected but no recognized Python LSP server found")
		}
	}

	return issues, warnings
}

func validatePerformanceSettings(cfg *config.GatewayConfig) ([]string, []string) {
	issues := []string{}
	warnings := []string{}

	// Check concurrent request limits
	if cfg.MaxConcurrentRequests <= 0 {
		issues = append(issues, "MaxConcurrentRequests must be greater than 0")
	} else if cfg.MaxConcurrentRequests > 1000 {
		warnings = append(warnings, fmt.Sprintf("Very high MaxConcurrentRequests (%d) may cause resource exhaustion", cfg.MaxConcurrentRequests))
	} else if cfg.MaxConcurrentRequests < 10 {
		warnings = append(warnings, fmt.Sprintf("Low MaxConcurrentRequests (%d) may limit performance", cfg.MaxConcurrentRequests))
	}

	// Check timeout settings
	if cfg.Timeout != "" {
		if strings.HasSuffix(cfg.Timeout, "s") {
			timeoutStr := strings.TrimSuffix(cfg.Timeout, "s")
			if timeoutVal, err := strconv.Atoi(timeoutStr); err == nil {
				if timeoutVal <= 0 {
					issues = append(issues, "Timeout must be greater than 0")
				} else if timeoutVal > 300 {
					warnings = append(warnings, fmt.Sprintf("Very high timeout (%ds) may cause client timeouts", timeoutVal))
				} else if timeoutVal < 5 {
					warnings = append(warnings, fmt.Sprintf("Very low timeout (%ds) may cause premature request failures", timeoutVal))
				}
			}
		}
	}

	return issues, warnings
}

func validateRoutingStrategy(cfg *config.GatewayConfig) ([]string, []string) {
	issues := []string{}
	warnings := []string{}

	// Check for routing conflicts or gaps
	languageCoverage := make(map[string]int)
	for _, server := range cfg.Servers {
		for _, lang := range server.Languages {
			languageCoverage[lang]++
		}
	}

	// Report languages with no coverage
	commonLanguages := []string{"go", config.LANG_PYTHON, "javascript", config.LANG_TYPESCRIPT, "java", "rust", "cpp", "c"}
	uncoveredLanguages := []string{}
	for _, lang := range commonLanguages {
		if languageCoverage[lang] == 0 {
			uncoveredLanguages = append(uncoveredLanguages, lang)
		}
	}

	if len(uncoveredLanguages) > 0 {
		warnings = append(warnings, fmt.Sprintf("Common languages without server coverage: %v", uncoveredLanguages))
	}

	// Check server transport consistency
	transportTypes := make(map[string]int)
	for _, server := range cfg.Servers {
		transportTypes[server.Transport]++
	}

	if len(transportTypes) > 2 {
		warnings = append(warnings, "Multiple transport types may complicate routing logic")
	}

	return issues, warnings
}

func validateResourceLimits(cfg *config.GatewayConfig) ([]string, []string) {
	issues := []string{}
	warnings := []string{}

	// Check for resource-intensive configurations
	serverCount := len(cfg.Servers)
	if serverCount > 10 {
		warnings = append(warnings, fmt.Sprintf("High number of servers (%d) may require significant system resources", serverCount))
	}

	// Estimate memory requirements
	estimatedMemoryMB := serverCount * 50 // Rough estimate: 50MB per server
	if cfg.MaxConcurrentRequests > 0 {
		estimatedMemoryMB += cfg.MaxConcurrentRequests * 2 // 2MB per concurrent request
	}

	if estimatedMemoryMB > 1000 {
		warnings = append(warnings, fmt.Sprintf("Estimated memory usage: ~%dMB - ensure sufficient system resources", estimatedMemoryMB))
	}

	// Check for servers that may conflict on resources
	javaServerCount := 0
	for _, server := range cfg.Servers {
		for _, lang := range server.Languages {
			if lang == "java" {
				javaServerCount++
				break
			}
		}
	}

	if javaServerCount > 1 {
		warnings = append(warnings, "Multiple Java language servers may consume significant memory")
	}

	return issues, warnings
}

// Helper functions for multi-language configuration integration

// convertMultiLanguageToGateway converts a MultiLanguageConfig to GatewayConfig
func convertMultiLanguageToGateway(multiConfig *config.MultiLanguageConfig) *config.GatewayConfig {
	if multiConfig == nil {
		return config.DefaultConfig()
	}

	gatewayConfig := config.DefaultConfig()

	// Convert server configurations
	for _, serverConfig := range multiConfig.ServerConfigs {
		gwServer := config.ServerConfig{
			Name:                  serverConfig.Name,
			Languages:             serverConfig.Languages,
			Command:               serverConfig.Command,
			Args:                  serverConfig.Args,
			Transport:             serverConfig.Transport,
			RootMarkers:           serverConfig.RootMarkers,
			Settings:              serverConfig.Settings,
			MaxConcurrentRequests: serverConfig.MaxConcurrentRequests,
			Priority:              serverConfig.Priority,
			Weight:                serverConfig.Weight,
		}
		gatewayConfig.Servers = append(gatewayConfig.Servers, gwServer)
	}

	// Copy workspace configuration
	if multiConfig.WorkspaceConfig != nil {
		gatewayConfig.EnableConcurrentServers = multiConfig.WorkspaceConfig.MultiRoot
		gatewayConfig.EnableSmartRouting = multiConfig.WorkspaceConfig.CrossLanguageReferences

		// Apply shared settings
		for key, value := range multiConfig.WorkspaceConfig.SharedSettings {
			switch key {
			case "production_mode":
				if enabled, ok := value.(bool); ok && enabled {
					gatewayConfig.MaxConcurrentRequests = 200
				}
			case "development_mode":
				if enabled, ok := value.(bool); ok && enabled {
					gatewayConfig.MaxConcurrentRequests = 100
				}
			}
		}
	}

	return gatewayConfig
}

// convertGatewayToServerConfigs converts GatewayConfig servers to MultiLanguage ServerConfigs
func convertGatewayToServerConfigs(gatewayConfig *config.GatewayConfig) []*config.ServerConfig {
	var serverConfigs []*config.ServerConfig

	for _, gwServer := range gatewayConfig.Servers {
		serverConfig := &config.ServerConfig{
			Name:                  gwServer.Name,
			Languages:             gwServer.Languages,
			Command:               gwServer.Command,
			Args:                  gwServer.Args,
			Transport:             gwServer.Transport,
			RootMarkers:           gwServer.RootMarkers,
			Settings:              gwServer.Settings,
			MaxConcurrentRequests: gwServer.MaxConcurrentRequests,
			Priority:              gwServer.Priority,
			Weight:                gwServer.Weight,
		}
		serverConfigs = append(serverConfigs, serverConfig)
	}

	return serverConfigs
}

// applyCliFlags applies CLI flags to the gateway configuration
func applyCliFlags(cfg *config.GatewayConfig) {
	if ConfigEnableSmartRouting {
		cfg.EnableSmartRouting = true
		if cfg.SmartRouterConfig == nil {
			cfg.SmartRouterConfig = &config.SmartRouterConfig{}
		}
	}

	if ConfigEnableConcurrentServers {
		cfg.EnableConcurrentServers = true
	}

	// Apply performance profile settings
	switch ConfigPerformanceProfile {
	case "high":
		cfg.MaxConcurrentRequests = 300
		cfg.Timeout = config.DEFAULT_TIMEOUT_30S
	case "low":
		cfg.MaxConcurrentRequests = 50
		cfg.Timeout = "60s"
	case setup.ProjectComplexityMedium:
		cfg.MaxConcurrentRequests = 150
		cfg.Timeout = "45s"
	}
}

// getEnvironmentFromOptimizationMode maps optimization mode to environment
func getEnvironmentFromOptimizationMode(optimizationMode string) string {
	switch strings.ToLower(optimizationMode) {
	case config.PerformanceProfileProduction:
		return config.PerformanceProfileProduction
	case config.PerformanceProfileAnalysis:
		return config.PerformanceProfileAnalysis
	case config.PerformanceProfileDevelopment:
		return config.PerformanceProfileDevelopment
	default:
		return config.PerformanceProfileDevelopment
	}
}

// migrateToEnhancedFormat migrates legacy configuration to enhanced format
func migrateToEnhancedFormat(cfg *config.GatewayConfig) *config.GatewayConfig {
	enhanced := *cfg

	// Add enhanced features if not present
	if enhanced.SmartRouterConfig == nil {
		enhanced.SmartRouterConfig = &config.SmartRouterConfig{
			DefaultStrategy:             "single_target_with_fallback",
			EnablePerformanceMonitoring: false,
			EnableCircuitBreaker:        true,
		}
	}

	// Ensure concurrent servers configuration
	if enhanced.MaxConcurrentServersPerLanguage == 0 {
		enhanced.MaxConcurrentServersPerLanguage = 2
	}

	// Initialize language pools if missing
	if enhanced.LanguagePools == nil {
		enhanced.LanguagePools = make([]config.LanguageServerPool, 0)

		// Create a map to track languages we've already added
		langMap := make(map[string]bool)

		for _, server := range enhanced.Servers {
			for _, lang := range server.Languages {
				if !langMap[lang] {
					langMap[lang] = true
					pool := config.LanguageServerPool{
						Language:      lang,
						Servers:       make(map[string]*config.ServerConfig),
						DefaultServer: "",
						LoadBalancingConfig: &config.LoadBalancingConfig{
							Strategy:        "round_robin",
							HealthThreshold: 0.8,
						},
						ResourceLimits: nil,
					}
					enhanced.LanguagePools = append(enhanced.LanguagePools, pool)
				}
			}
		}
	}

	return &enhanced
}

// getEnhancedFeaturesSummary returns a summary of enhanced features
func getEnhancedFeaturesSummary(cfg *config.GatewayConfig) string {
	var features []string

	if cfg.EnableSmartRouting {
		features = append(features, "smart-routing")
	}
	if cfg.EnableConcurrentServers {
		features = append(features, "concurrent-servers")
	}
	if cfg.EnableEnhancements {
		features = append(features, "enhancements")
	}
	if len(cfg.LanguagePools) > 0 {
		features = append(features, fmt.Sprintf("language-pools(%d)", len(cfg.LanguagePools)))
	}

	if len(features) == 0 {
		return "basic"
	}
	return strings.Join(features, ", ")
}
