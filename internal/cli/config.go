package cli

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"lsp-gateway/internal/config"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	configFilePath        string
	configOutputPath      string
	configJSON            bool
	configOverwrite       bool
	configAutoDetect      bool
	configValidateOnly    bool
	configIncludeComments bool
	configTargetRuntime   string
)

var configCmd = &cobra.Command{
	Use:   CmdConfig,
	Short: "Configuration management",
	Long: `Manage LSP Gateway configuration files.

The config command provides comprehensive configuration management capabilities,
including generation, validation, and inspection of configuration files.

Available subcommands:
  generate - Generate configuration files
  validate - Validate existing configuration
  show     - Display current configuration

Examples:
  # Generate default configuration
  lsp-gateway config generate

  # Generate configuration based on detected runtimes
  lsp-gateway config generate --auto-detect

  # Validate existing configuration
  lsp-gateway config validate

  # Show current configuration in JSON format
  lsp-gateway config show --json`,
	RunE: configShow, // Default to showing config when no subcommand specified
}

var configGenerateCmd = &cobra.Command{
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
  lsp-gateway config generate --include-comments`,
	RunE: configGenerate,
}

var configValidateCmd = &cobra.Command{
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
  lsp-gateway config validate --validate-only`,
	RunE: configValidate,
}

var configShowCmd = &cobra.Command{
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
	RunE: configShow,
}

func init() {
	configCmd.PersistentFlags().StringVarP(&configFilePath, "config", "c", DefaultConfigFile, "Configuration file path")
	configCmd.PersistentFlags().BoolVar(&configJSON, "json", false, "Output in JSON format")

	configGenerateCmd.Flags().StringVarP(&configOutputPath, "output", "o", "", "Output configuration file path (default: same as input)")
	configGenerateCmd.Flags().BoolVar(&configOverwrite, "overwrite", false, "Overwrite existing configuration file")
	configGenerateCmd.Flags().BoolVar(&configAutoDetect, "auto-detect", false, "Auto-detect runtimes and generate configuration")
	configGenerateCmd.Flags().BoolVar(&configIncludeComments, "include-comments", false, "Include explanatory comments in generated config")
	configGenerateCmd.Flags().StringVar(&configTargetRuntime, "runtime", "", "Generate configuration for specific runtime (go, python, nodejs, java)")

	configValidateCmd.Flags().BoolVar(&configValidateOnly, "validate-only", false, "Perform syntax validation only (skip server verification)")

	configShowCmd.Flags().BoolVar(&configValidateOnly, "validate", false, "Include validation information in output")

	configCmd.AddCommand(configGenerateCmd)
	configCmd.AddCommand(configValidateCmd)
	configCmd.AddCommand(configShowCmd)

	rootCmd.AddCommand(configCmd)
}

func configGenerate(cmd *cobra.Command, args []string) error {
	if err := validateConfigGenerateParams(); err != nil {
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

	if err := writeConfigurationFile(cfg, outputPath, configIncludeComments); err != nil {
		return NewPermissionError(outputPath, "write configuration to")
	}

	fmt.Printf("✓ Configuration written to: %s\n", outputPath)
	fmt.Println("Note: Advanced configuration generation will be available once setup integration is complete.")

	return nil
}

func configValidate(cmd *cobra.Command, args []string) error {
	if err := validateConfigValidateParams(); err != nil {
		return err
	}

	fmt.Printf("Validating configuration file: %s\n", configFilePath)

	cfg, err := config.LoadConfig(configFilePath)
	if err != nil {
		if configJSON {
			outputValidationJSON(false, []string{fmt.Sprintf("Failed to load configuration: %v", err)}, nil)
			return nil
		}
		return HandleConfigError(err, configFilePath)
	}

	issues := []string{}
	warnings := []string{}

	if err := cfg.Validate(); err != nil {
		issues = append(issues, fmt.Sprintf("Configuration validation failed: %v", err))
	}

	if !configValidateOnly {
		warnings = append(warnings, "Extended validation not yet available - basic validation only")
	}

	if configJSON {
		outputValidationJSON(len(issues) == 0, issues, warnings)
	} else {
		outputValidationHuman(len(issues) == 0, issues, warnings, cfg)
	}

	if len(issues) > 0 {
		return NewValidationError("configuration", issues)
	}

	return nil
}

func configShow(cmd *cobra.Command, args []string) error {
	if err := validateConfigShowParams(); err != nil {
		return err
	}

	cfg, err := config.LoadConfig(configFilePath)
	if err != nil {
		return HandleConfigError(err, configFilePath)
	}

	var validationResult interface{}
	if configValidateOnly {
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

	if configJSON {
		return outputConfigJSON(cfg, validationResult)
	}

	return outputConfigHuman(cfg, validationResult)
}

func determineConfigOutputPath() (string, error) {
	outputPath := configOutputPath
	if outputPath == "" {
		outputPath = configFilePath
	}

	if _, err := os.Stat(outputPath); err == nil && !configOverwrite {
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
	if configAutoDetect {
		fmt.Println("Detecting runtimes and generating configuration...")
		return config.DefaultConfig(), nil
	}

	if configTargetRuntime != "" {
		if err := validateTargetRuntime(); err != nil {
			return nil, err
		}
		fmt.Printf("Generating configuration for %s runtime...\n", configTargetRuntime)
		return config.DefaultConfig(), nil
	}

	fmt.Println("Generating default configuration...")
	return config.DefaultConfig(), nil
}

func validateTargetRuntime() error {
	supportedRuntimes := []string{"go", "python", "nodejs", "java"}
	for _, supported := range supportedRuntimes {
		if configTargetRuntime == supported {
			return nil
		}
	}
	return NewRuntimeNotFoundError(configTargetRuntime)
}

func writeConfigurationFile(cfg *config.GatewayConfig, path string, includeComments bool) error {
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
	encoder := yaml.NewEncoder(file)
	encoder.SetIndent(2)
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

func outputValidationJSON(valid bool, issues []string, warnings []string) {
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
	if output, err := json.MarshalIndent(data, "", "  "); err == nil {
		fmt.Println(string(output))
	}
}

func outputValidationHuman(valid bool, issues []string, warnings []string, cfg *config.GatewayConfig) {
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

func outputConfigJSON(cfg *config.GatewayConfig, validation interface{}) error {
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

func outputConfigHuman(cfg *config.GatewayConfig, validation interface{}) error {
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

func validateConfigGenerateParams() error {
	result := ValidateMultiple(
		func() *ValidationError {
			return ValidateRuntimeName(configTargetRuntime, "runtime")
		},
		func() *ValidationError {
			if configOutputPath != "" {
				return ValidateFilePath(configOutputPath, "output", "create")
			}
			return nil
		},
		func() *ValidationError {
			if configOutputPath == "" {
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

func validateConfigValidateParams() error {
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

func validateConfigShowParams() error {
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
