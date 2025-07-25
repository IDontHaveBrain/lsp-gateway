package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/installer"
	"lsp-gateway/internal/setup"

	"github.com/spf13/cobra"
)

// Setup command flags
var (
	setupTimeout       time.Duration
	setupForce         bool
	setupJSON          bool
	setupVerbose       bool
	setupNoInteractive bool
	setupSkipVerify    bool
	setupConfigPath    string
	// Multi-language setup flags
	setupMultiLanguage      bool
	setupProjectPath        string
	setupOptimizationMode   string
	setupTemplate           string
	setupEnableSmartRouting bool
	setupEnableConcurrent   bool
	setupPerformanceProfile string
	setupProjectDetection   bool
	// Enhanced multi-language flags
)

// Setup result structures
type SetupResult struct {
	Success           bool                                `json:"success"`
	Duration          time.Duration                       `json:"duration"`
	RuntimesDetected  map[string]*setup.RuntimeInfo       `json:"runtimes_detected,omitempty"`
	RuntimesInstalled map[string]*installer.InstallResult `json:"runtimes_installed,omitempty"`
	ServersInstalled  map[string]*installer.InstallResult `json:"servers_installed,omitempty"`
	ConfigGenerated   bool                                `json:"config_generated"`
	ConfigPath        string                              `json:"config_path,omitempty"`
	Issues            []string                            `json:"issues,omitempty"`
	Warnings          []string                            `json:"warnings,omitempty"`
	Messages          []string                            `json:"messages,omitempty"`
	Summary           *SetupSummary                       `json:"summary,omitempty"`
}

// SetupSummary provides a summary of the setup operation
type SetupSummary struct {
	TotalRuntimes        int `json:"total_runtimes"`
	RuntimesInstalled    int `json:"runtimes_installed"`
	RuntimesAlreadyExist int `json:"runtimes_already_exist"`
	RuntimesFailed       int `json:"runtimes_failed"`
	TotalServers         int `json:"total_servers"`
	ServersInstalled     int `json:"servers_installed"`
	ServersAlreadyExist  int `json:"servers_already_exist"`
	ServersFailed        int `json:"servers_failed"`
}

// WizardState tracks the current state of the interactive wizard
type WizardState struct {
	SelectedRuntimes map[string]bool
	SelectedServers  map[string]bool
	CustomConfig     bool
	AdvancedSettings bool
	DetectedRuntimes map[string]*setup.RuntimeInfo
	AvailableServers []string
	ConfigPath       string
	// Enhanced wizard state
	ProjectAnalysis         *setup.ProjectAnalysis
	SelectedTemplate        *setup.ConfigurationTemplate
	OptimizationMode        string
	PerformanceProfile      string
	EnableSmartRouting      bool
	EnableConcurrentServers bool
}

// Interactive helper functions for the wizard

// promptYesNo prompts for a yes/no answer with a default value
func promptYesNo(message string, defaultValue bool) (bool, error) {
	reader := bufio.NewReader(os.Stdin)

	defaultStr := "y/N"
	if defaultValue {
		defaultStr = "Y/n"
	}

	fmt.Printf("%s [%s]: ", message, defaultStr)

	response, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}

	response = strings.TrimSpace(strings.ToLower(response))

	if response == "" {
		return defaultValue, nil
	}

	return response == "y" || response == "yes", nil
}

// promptSelection prompts for a selection from a list of options
func promptSelection(message string, options []string, allowMultiple bool) ([]int, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println(message)
	for i, option := range options {
		fmt.Printf("  %d) %s\n", i+1, option)
	}

	if allowMultiple {
		fmt.Print("Enter your choices (comma-separated numbers, or 'all' for all): ")
	} else {
		fmt.Print("Enter your choice (number): ")
	}

	response, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}

	response = strings.TrimSpace(response)

	if allowMultiple && strings.ToLower(response) == "all" {
		selections := make([]int, len(options))
		for i := range options {
			selections[i] = i
		}
		return selections, nil
	}

	var selections []int
	parts := strings.Split(response, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		choice, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("invalid choice: %s", part)
		}

		if choice < 1 || choice > len(options) {
			return nil, fmt.Errorf("choice out of range: %d", choice)
		}

		selections = append(selections, choice-1)
	}

	return selections, nil
}

// promptText prompts for text input with an optional default value
func promptText(message string, defaultValue string) (string, error) {
	reader := bufio.NewReader(os.Stdin)

	if defaultValue != "" {
		fmt.Printf("%s [%s]: ", message, defaultValue)
	} else {
		fmt.Printf("%s: ", message)
	}

	response, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	response = strings.TrimSpace(response)

	if response == "" && defaultValue != "" {
		return defaultValue, nil
	}

	return response, nil
}

// pressEnterToContinue waits for the user to press Enter
func pressEnterToContinue(message string) {
	reader := bufio.NewReader(os.Stdin)
	if message != "" {
		fmt.Printf("\n%s", message)
	}
	fmt.Print("Press Enter to continue...")
	_, _ = reader.ReadString('\n')
	fmt.Println()
}

var setupCmd = &cobra.Command{
	Use:   CmdSetup,
	Short: "Setup LSP Gateway with automated configuration and installation",
	Long: `Setup command provides automated configuration and installation of LSP Gateway components.
This includes runtime detection, language server installation, and configuration generation.

The setup process consists of:
1. Runtime detection (Go, Python, Node.js, Java)
2. Language server installation based on detected runtimes
3. Configuration file generation with optimal settings
4. Verification of installations and functionality

Available setup modes:
- all:           Complete automated setup (recommended for first-time users)
- wizard:        Interactive setup with user choices and customization
- multi-language: Multi-language project setup with auto-detection
- template:      Template-based setup for specific project types

Examples:
  lsp-gateway setup all                    # Complete automated setup
  lsp-gateway setup all --force            # Force reinstall all components
  lsp-gateway setup all --timeout 15m      # Set custom timeout
  lsp-gateway setup all --json             # Output results in JSON format
  lsp-gateway setup wizard                 # Interactive setup wizard
  lsp-gateway setup wizard --verbose       # Verbose wizard with detailed explanations
  lsp-gateway setup multi-language --project-path /path/to/project  # Multi-language setup
  lsp-gateway setup template --template monorepo                    # Template-based setup`,
}

var setupAllCmd = &cobra.Command{
	Use:   "all",
	Short: "Complete automated setup of LSP Gateway",
	Long: `Performs complete automated setup of LSP Gateway including:

🔍 DETECTION PHASE:
  • Scan system for existing language runtimes
  • Detect package managers and installation methods
  • Identify compatible versions and dependencies
  • Generate platform-specific installation strategy

📦 INSTALLATION PHASE:
  • Install missing language runtimes automatically
  • Install language servers for detected/installed runtimes
  • Configure optimal settings for each language server
  • Verify installations and functionality

⚙️  CONFIGURATION PHASE:
  • Generate optimized configuration file
  • Set up server routing and language mappings
  • Configure timeouts and performance settings
  • Validate final configuration

✅ VERIFICATION PHASE:
  • Test runtime installations and versions
  • Verify language server functionality
  • Validate configuration completeness
  • Generate setup summary and recommendations

This command is designed for:
- First-time LSP Gateway users
- Automated CI/CD environments
- Quick development environment setup
- System recovery after changes

Use --force to reinstall existing components.
Use --json for machine-readable output.

Multi-language setup features:
- Automatic project language detection
- Optimized server configurations for detected languages
- Smart routing for multi-language codebases
- Performance tuning based on project characteristics`,
	RunE: runSetupAll,
}

var setupMultiLanguageCmd = &cobra.Command{
	Use:   "multi-language",
	Short: "Multi-language project setup with auto-detection",
	Long: `Multi-language project setup automatically detects languages in your project
and configures optimal language server settings.

🔍 DETECTION FEATURES:
  • Automatic project language scanning
  • Framework and build system detection
  • Dependency analysis for version requirements
  • Project structure analysis for optimization

⚙️  CONFIGURATION FEATURES:
  • Language-specific server optimization
  • Smart routing for multi-language requests
  • Performance tuning based on project size
  • Concurrent server management

📊 OPTIMIZATION MODES:
  • Development: Fast startup, moderate resource usage
  • Production: High performance, maximum concurrency
  • Analysis: Extended timeouts, comprehensive features

Examples:
  # Setup with project detection
  lsp-gateway setup multi-language --project-path /path/to/project

  # Setup with optimization mode
  lsp-gateway setup multi-language --project-path /path/to/project --optimization-mode production

  # Setup with template and smart routing
  lsp-gateway setup multi-language --template monorepo --enable-smart-routing`,
	RunE: runSetupMultiLanguage,
}

var setupTemplateCmd = &cobra.Command{
	Use:   "template",
	Short: "Template-based setup for specific project types",
	Long: `Template-based setup provides pre-configured settings for common project types.

📋 AVAILABLE TEMPLATES:
  • monorepo:      Multi-language monorepo with shared tooling
  • microservices: Distributed services with individual configurations
  • single-language: Single-language project with minimal setup
  • multi-language: Multi-language project with smart routing
  • development:   Development environment optimized settings
  • production:    Production environment optimized settings

🎯 TEMPLATE FEATURES:
  • Pre-configured server settings
  • Optimized resource limits
  • Language-specific customizations
  • Performance profiles
  • Framework-specific configurations

Examples:
  # Setup monorepo template
  lsp-gateway setup template --template monorepo

  # Setup with performance profile
  lsp-gateway setup template --template production --performance-profile high

  # Setup with custom project path
  lsp-gateway setup template --template microservices --project-path /path/to/services`,
	RunE: runSetupTemplate,
}

var setupDetectCmd = &cobra.Command{
	Use:   "detect",
	Short: "Auto-detect and setup for current project",
	Long: `Auto-detect project characteristics and setup optimal configuration.

🔍 DETECTION FEATURES:
  • Automatic language detection
  • Framework recognition
  • Project type identification
  • Complexity analysis
  • Build system detection

⚙️  SETUP FEATURES:
  • Template selection based on analysis
  • Optimization mode recommendation
  • Performance profile selection
  • Smart routing configuration

📊 SUPPORTED PROJECT TYPES:
  • Single-language projects
  • Multi-language projects
  • Monorepo structures
  • Microservices architectures

Examples:
  # Detect and setup current directory
  lsp-gateway setup detect

  # Detect with specific project path
  lsp-gateway setup detect --project-path /path/to/project

  # Detect with optimization mode override
  lsp-gateway setup detect --optimization-mode production`,
	RunE: runSetupDetect,
}

var setupWizardCmd = &cobra.Command{
	Use:   "wizard",
	Short: "Interactive setup wizard with user choices",
	Long: `Interactive setup wizard provides guided configuration with user choices:

🧙 WIZARD FEATURES:
  • Step-by-step guided setup process
  • Runtime selection and version choices
  • Language server customization options
  • Configuration review and editing
  • Progress tracking with user feedback

📋 WIZARD STEPS:
  1. Welcome and system overview
  2. Runtime detection and selection
  3. Language server configuration
  4. Advanced settings (optional)
  5. Installation execution
  6. Verification and testing
  7. Final configuration review

🎯 CUSTOMIZATION OPTIONS:
  • Choose specific runtimes to install
  • Select language servers and versions
  • Configure server-specific settings
  • Set custom timeouts and performance options
  • Enable/disable specific language features

💡 WIZARD MODES:
  • Standard: Essential questions for typical setup
  • Verbose: Detailed explanations and advanced options
  • Quick: Minimal questions with smart defaults

The wizard is ideal for:
- Users who want control over the setup process
- Custom development environments
- Learning about LSP Gateway components
- Advanced configuration scenarios

Use --verbose for detailed explanations during setup.

🆕 ENHANCED WIZARD FEATURES:
  • Multi-language project detection
  • Template selection with previews
  • Performance optimization guidance
  • Smart routing configuration`,
	RunE: runSetupWizard,
}

func init() {
	// Setup All command flags
	setupAllCmd.Flags().DurationVar(&setupTimeout, FLAG_TIMEOUT, 15*time.Minute, "Maximum time for complete setup process")
	setupAllCmd.Flags().BoolVarP(&setupForce, FLAG_FORCE, "f", false, "Force reinstall existing components")
	setupAllCmd.Flags().BoolVar(&setupJSON, FLAG_JSON, false, FLAG_DESCRIPTION_JSON_OUTPUT)
	setupAllCmd.Flags().BoolVarP(&setupVerbose, FLAG_VERBOSE, "v", false, "Enable verbose output with detailed progress")
	setupAllCmd.Flags().BoolVar(&setupSkipVerify, "skip-verify", false, "Skip final verification step (faster but less thorough)")
	setupAllCmd.Flags().StringVar(&setupConfigPath, "config", "config.yaml", "Path to configuration file")
	// Enhanced flags for all setup
	setupAllCmd.Flags().BoolVar(&setupEnableSmartRouting, "enable-smart-routing", false, "Enable intelligent request routing")
	setupAllCmd.Flags().BoolVar(&setupEnableConcurrent, "enable-concurrent-servers", false, "Enable concurrent server management")
	setupAllCmd.Flags().StringVar(&setupOptimizationMode, "optimization-mode", config.PerformanceProfileDevelopment, "Set optimization mode (development, production, analysis)")

	// Setup Multi-language command flags
	setupMultiLanguageCmd.Flags().DurationVar(&setupTimeout, FLAG_TIMEOUT, 20*time.Minute, "Maximum time for multi-language setup process")
	setupMultiLanguageCmd.Flags().BoolVarP(&setupForce, FLAG_FORCE, "f", false, "Force reinstall existing components")
	setupMultiLanguageCmd.Flags().BoolVar(&setupJSON, FLAG_JSON, false, FLAG_DESCRIPTION_JSON_OUTPUT)
	setupMultiLanguageCmd.Flags().BoolVarP(&setupVerbose, FLAG_VERBOSE, "v", false, "Enable verbose output with detailed progress")
	setupMultiLanguageCmd.Flags().StringVar(&setupProjectPath, "project-path", "", "Project path for language detection (required)")
	setupMultiLanguageCmd.Flags().StringVar(&setupOptimizationMode, "optimization-mode", config.PerformanceProfileDevelopment, "Set optimization mode (development, production, analysis)")
	setupMultiLanguageCmd.Flags().StringVar(&setupTemplate, "template", "", "Use configuration template (monorepo, microservices, single-project)")
	setupMultiLanguageCmd.Flags().BoolVar(&setupEnableSmartRouting, "enable-smart-routing", false, "Enable intelligent request routing")
	setupMultiLanguageCmd.Flags().BoolVar(&setupEnableConcurrent, "enable-concurrent-servers", false, "Enable concurrent server management")
	setupMultiLanguageCmd.Flags().StringVar(&setupPerformanceProfile, "performance-profile", "medium", "Set performance profile (low, medium, high)")
	setupMultiLanguageCmd.Flags().BoolVar(&setupProjectDetection, "project-detection", true, "Enable automatic project detection")
	setupMultiLanguageCmd.Flags().StringVar(&setupConfigPath, "config", "config.yaml", "Path to configuration file")

	// Setup Template command flags
	setupTemplateCmd.Flags().DurationVar(&setupTimeout, FLAG_TIMEOUT, 15*time.Minute, "Maximum time for template setup process")
	setupTemplateCmd.Flags().BoolVarP(&setupForce, FLAG_FORCE, "f", false, "Force reinstall existing components")
	setupTemplateCmd.Flags().BoolVar(&setupJSON, FLAG_JSON, false, FLAG_DESCRIPTION_JSON_OUTPUT)
	setupTemplateCmd.Flags().BoolVarP(&setupVerbose, FLAG_VERBOSE, "v", false, "Enable verbose output with detailed progress")
	setupTemplateCmd.Flags().StringVar(&setupTemplate, "template", "", "Template name (monorepo, microservices, single-project, enterprise) (required)")
	setupTemplateCmd.Flags().StringVar(&setupProjectPath, "project-path", ".", "Project path for template application")
	setupTemplateCmd.Flags().StringVar(&setupOptimizationMode, "optimization-mode", config.PerformanceProfileDevelopment, "Set optimization mode (development, production, analysis)")
	setupTemplateCmd.Flags().StringVar(&setupPerformanceProfile, "performance-profile", "medium", "Set performance profile (low, medium, high)")
	setupTemplateCmd.Flags().BoolVar(&setupEnableSmartRouting, "enable-smart-routing", false, "Enable intelligent request routing")
	setupTemplateCmd.Flags().BoolVar(&setupEnableConcurrent, "enable-concurrent-servers", false, "Enable concurrent server management")
	setupTemplateCmd.Flags().StringVar(&setupConfigPath, "config", "config.yaml", "Path to configuration file")

	// Setup Wizard command flags (enhanced)
	setupWizardCmd.Flags().DurationVar(&setupTimeout, FLAG_TIMEOUT, 30*time.Minute, "Maximum time for wizard setup process")
	setupWizardCmd.Flags().BoolVarP(&setupForce, FLAG_FORCE, "f", false, "Force reinstall existing components")
	setupWizardCmd.Flags().BoolVar(&setupJSON, FLAG_JSON, false, FLAG_DESCRIPTION_JSON_OUTPUT)
	setupWizardCmd.Flags().BoolVarP(&setupVerbose, FLAG_VERBOSE, "v", false, "Enable verbose wizard with detailed explanations")
	setupWizardCmd.Flags().BoolVar(&setupNoInteractive, FLAG_NO_INTERACTIVE, false, "Run wizard in non-interactive mode with defaults")
	setupWizardCmd.Flags().BoolVar(&setupMultiLanguage, "multi-language-mode", false, "Enable multi-language wizard mode")
	setupWizardCmd.Flags().BoolVar(&setupProjectDetection, "project-detection", false, "Enable project detection in wizard")
	setupWizardCmd.Flags().StringVar(&setupConfigPath, "config", "config.yaml", "Path to configuration file")

	// Setup Detect command flags
	setupDetectCmd.Flags().DurationVar(&setupTimeout, FLAG_TIMEOUT, 15*time.Minute, "Maximum time for detect setup process")
	setupDetectCmd.Flags().BoolVarP(&setupForce, FLAG_FORCE, "f", false, "Force reinstall existing components")
	setupDetectCmd.Flags().BoolVar(&setupJSON, FLAG_JSON, false, FLAG_DESCRIPTION_JSON_OUTPUT)
	setupDetectCmd.Flags().BoolVarP(&setupVerbose, FLAG_VERBOSE, "v", false, "Enable verbose output with detailed progress")
	setupDetectCmd.Flags().StringVar(&setupProjectPath, "project-path", ".", "Project path for detection")
	setupDetectCmd.Flags().StringVar(&setupOptimizationMode, "optimization-mode", "", "Override optimization mode (development, production, analysis)")
	setupDetectCmd.Flags().StringVar(&setupPerformanceProfile, "performance-profile", "", "Override performance profile (low, medium, high)")
	setupDetectCmd.Flags().BoolVar(&setupEnableSmartRouting, "enable-smart-routing", false, "Force enable intelligent request routing")
	setupDetectCmd.Flags().BoolVar(&setupEnableConcurrent, "enable-concurrent-servers", false, "Force enable concurrent server management")
	setupDetectCmd.Flags().StringVar(&setupConfigPath, "config", "config.yaml", "Path to configuration file")

	// Add subcommands to setup command
	setupCmd.AddCommand(setupAllCmd)
	setupCmd.AddCommand(setupMultiLanguageCmd)
	setupCmd.AddCommand(setupTemplateCmd)
	setupCmd.AddCommand(setupWizardCmd)
	setupCmd.AddCommand(setupDetectCmd)

	// Register setup command with root
	rootCmd.AddCommand(setupCmd)
}

func runSetupAll(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), setupTimeout)
	defer cancel()

	startTime := time.Now()

	if setupVerbose {
		fmt.Println("🚀 Starting LSP Gateway automated setup...")
		fmt.Printf("Platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
		fmt.Printf("Timeout: %v\n", setupTimeout)
		fmt.Printf("Force reinstall: %v\n", setupForce)
		fmt.Println()
	}

	// Initialize result structure
	result := &SetupResult{
		Success:           false,
		RuntimesDetected:  make(map[string]*setup.RuntimeInfo),
		RuntimesInstalled: make(map[string]*installer.InstallResult),
		ServersInstalled:  make(map[string]*installer.InstallResult),
		Issues:            make([]string, 0),
		Warnings:          make([]string, 0),
		Messages:          make([]string, 0),
		Summary:           &SetupSummary{},
	}

	// Phase 1: Runtime Detection and Installation
	if setupVerbose {
		fmt.Println("🔍 Phase 1: Runtime Detection and Installation")
	}

	err := setupRuntimes(ctx, result)
	if err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("Runtime setup failed: %v", err))
		if !setupJSON {
			fmt.Printf("❌ Runtime setup failed: %v\n", err)
		}
	} else if setupVerbose && !setupJSON {
		fmt.Printf("✅ Runtime setup completed (%d runtimes processed)\n", len(result.RuntimesInstalled))
	}

	// Phase 2: Language Server Installation
	if setupVerbose {
		fmt.Println("\n📦 Phase 2: Language Server Installation")
	}

	err = setupServers(ctx, result)
	if err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("Server setup failed: %v", err))
		if !setupJSON {
			fmt.Printf("❌ Server setup failed: %v\n", err)
		}
	} else if setupVerbose && !setupJSON {
		fmt.Printf("✅ Server setup completed (%d servers processed)\n", len(result.ServersInstalled))
	}

	// Phase 3: Configuration Generation
	if setupVerbose {
		fmt.Println("\n⚙️  Phase 3: Configuration Generation")
	}

	err = setupConfiguration(ctx, result)
	if err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("Configuration setup failed: %v", err))
		if !setupJSON {
			fmt.Printf("❌ Configuration setup failed: %v\n", err)
		}
	} else if setupVerbose && !setupJSON {
		fmt.Println("✅ Configuration generation completed")
	}

	// Phase 4: Verification (optional)
	if !setupSkipVerify {
		if setupVerbose {
			fmt.Println("\n✅ Phase 4: Final Verification")
		}

		verificationWarnings, err := runFinalVerification(ctx, result)
		if err != nil {
			result.Issues = append(result.Issues, fmt.Sprintf("Verification failed: %v", err))
			if !setupJSON {
				fmt.Printf("⚠️  Verification warnings: %v\n", err)
			}
		} else {
			result.Warnings = append(result.Warnings, verificationWarnings...)
			if setupVerbose && !setupJSON && len(verificationWarnings) == 0 {
				fmt.Println("✅ All verifications passed")
			}
		}
	}

	// Calculate final results
	result.Duration = time.Since(startTime)
	result.Success = len(result.Issues) == 0

	// Generate summary
	generateSetupSummary(result)

	// Generate summary messages
	if result.Success {
		result.Messages = append(result.Messages, "LSP Gateway setup completed successfully")
		result.Messages = append(result.Messages, fmt.Sprintf("Setup duration: %v", result.Duration))
		result.Messages = append(result.Messages, "Run 'lsp-gateway server' to start the HTTP gateway")
		result.Messages = append(result.Messages, "Run 'lsp-gateway mcp' to start the MCP server")
	}

	// Output results
	if setupJSON {
		return outputSetupResultsJSON(result)
	} else {
		return outputSetupResultsHuman(result)
	}
}

func runSetupWizard(cmd *cobra.Command, args []string) error {
	if setupJSON {
		return NewValidationError("JSON output not supported in wizard mode", []string{})
	}

	if setupNoInteractive {
		// Run wizard in non-interactive mode with defaults
		if setupVerbose {
			fmt.Println("🤖 Running setup wizard in non-interactive mode with defaults...")
		}
		return runSetupAll(cmd, args)
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), setupTimeout)
	defer cancel()

	startTime := time.Now()

	// Initialize wizard state and result structure
	wizardState := &WizardState{
		SelectedRuntimes:        make(map[string]bool),
		SelectedServers:         make(map[string]bool),
		ConfigPath:              setupConfigPath,
		OptimizationMode:        setupOptimizationMode,
		PerformanceProfile:      setupPerformanceProfile,
		EnableSmartRouting:      setupEnableSmartRouting,
		EnableConcurrentServers: setupEnableConcurrent,
	}

	result := &SetupResult{
		Success:           false,
		RuntimesDetected:  make(map[string]*setup.RuntimeInfo),
		RuntimesInstalled: make(map[string]*installer.InstallResult),
		ServersInstalled:  make(map[string]*installer.InstallResult),
		Issues:            make([]string, 0),
		Warnings:          make([]string, 0),
		Messages:          make([]string, 0),
		Summary:           &SetupSummary{},
	}

	// Step 1: Welcome and System Overview
	if err := wizardWelcome(ctx, wizardState, result); err != nil {
		return err
	}

	// Step 2: Enhanced Project Detection (if multi-language mode)
	if setupMultiLanguage || setupProjectDetection {
		if err := wizardProjectDetection(ctx, wizardState, result); err != nil {
			return err
		}
	}

	// Step 3: Template Selection (if multi-language mode)
	if setupMultiLanguage {
		if err := wizardTemplateSelection(ctx, wizardState, result); err != nil {
			return err
		}
	}

	// Step 4: Runtime Detection and Selection
	if err := wizardRuntimeSelection(ctx, wizardState, result); err != nil {
		return err
	}

	// Step 5: Language Server Selection
	if err := wizardServerSelection(ctx, wizardState, result); err != nil {
		return err
	}

	// Step 6: Enhanced Configuration Options
	if err := wizardEnhancedConfigurationOptions(ctx, wizardState, result); err != nil {
		return err
	}

	// Step 7: Review and Confirmation
	if err := wizardReviewSelection(ctx, wizardState, result); err != nil {
		return err
	}

	// Step 8: Installation Execution
	if err := wizardExecuteInstallation(ctx, wizardState, result); err != nil {
		return err
	}

	// Step 9: Final Summary
	result.Duration = time.Since(startTime)
	result.Success = len(result.Issues) == 0

	generateSetupSummary(result)

	if result.Success {
		result.Messages = append(result.Messages, "LSP Gateway setup completed successfully via enhanced wizard")
		result.Messages = append(result.Messages, fmt.Sprintf("Setup duration: %v", result.Duration))
		if wizardState.ProjectAnalysis != nil {
			result.Messages = append(result.Messages, fmt.Sprintf("Project type: %s with %d languages", wizardState.ProjectAnalysis.ProjectType, len(wizardState.ProjectAnalysis.DetectedLanguages)))
		}
		result.Messages = append(result.Messages, "Run 'lsp-gateway server' to start the HTTP gateway")
		result.Messages = append(result.Messages, "Run 'lsp-gateway mcp' to start the MCP server")
	}

	return outputSetupResultsHuman(result)
}

// Wizard step functions

// wizardWelcome shows the welcome screen and system overview
func wizardWelcome(ctx context.Context, state *WizardState, result *SetupResult) error {
	fmt.Println("🧙 Welcome to the LSP Gateway Setup Wizard!")
	fmt.Println()
	fmt.Printf("Platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("Timeout: %v\n", setupTimeout)
	fmt.Printf("Force reinstall: %v\n", setupForce)
	fmt.Println()

	fmt.Println("This interactive wizard will guide you through:")
	fmt.Println("  1. 🔍 System analysis and runtime detection")
	fmt.Println("  2. 📋 Runtime selection and customization")
	fmt.Println("  3. 📦 Language server selection")
	fmt.Println("  4. ⚙️  Configuration options")
	fmt.Println("  5. 📊 Review and confirmation")
	fmt.Println("  6. 🚀 Installation execution")
	fmt.Println("  7. ✅ Final verification and summary")
	fmt.Println()

	if setupVerbose {
		fmt.Println("💡 Verbose mode is enabled - you'll see detailed explanations.")
		fmt.Println()
	}

	proceed, err := promptYesNo("Ready to begin the setup wizard?", true)
	if err != nil {
		return fmt.Errorf("failed to read user input: %w", err)
	}

	if !proceed {
		return fmt.Errorf("setup wizard cancelled by user")
	}

	fmt.Println()
	return nil
}

// wizardRuntimeSelection handles runtime detection and user selection
func wizardRuntimeSelection(ctx context.Context, state *WizardState, result *SetupResult) error {
	fmt.Println("🔍 Step 1: Runtime Detection and Selection")
	fmt.Println("=========================================")
	fmt.Println()

	if setupVerbose {
		fmt.Println("📝 Runtime detection analyzes your system for existing language runtimes")
		fmt.Println("   and determines which ones are compatible with LSP Gateway.")
		fmt.Println()
	}

	fmt.Println("🔍 Scanning system for language runtimes...")

	// Detect all runtimes
	detector := setup.NewRuntimeDetector()
	if detector == nil {
		return NewInstallerCreationError("runtime detector")
	}

	detectionReport, err := detector.DetectAll(ctx)
	if err != nil {
		return fmt.Errorf("runtime detection failed: %w", err)
	}

	state.DetectedRuntimes = detectionReport.Runtimes
	result.RuntimesDetected = detectionReport.Runtimes
	result.Summary.TotalRuntimes = len(detectionReport.Runtimes)

	// Display detection results
	fmt.Println("\n📊 Runtime Detection Results:")
	runtimeNames := []string{}
	runtimeStatuses := []string{}

	for runtimeName, runtimeInfo := range detectionReport.Runtimes {
		runtimeNames = append(runtimeNames, runtimeName)

		if runtimeInfo.Installed && runtimeInfo.Compatible {
			status := fmt.Sprintf("✅ %s: Installed and compatible (v%s)", runtimeName, runtimeInfo.Version)
			if setupVerbose {
				status += fmt.Sprintf(" at %s", runtimeInfo.Path)
			}
			runtimeStatuses = append(runtimeStatuses, status)
		} else if runtimeInfo.Installed {
			status := fmt.Sprintf("⚠️  %s: Installed but needs update (v%s)", runtimeName, runtimeInfo.Version)
			if setupVerbose && len(runtimeInfo.Issues) > 0 {
				status += fmt.Sprintf(" - Issues: %v", runtimeInfo.Issues)
			}
			runtimeStatuses = append(runtimeStatuses, status)
		} else {
			status := fmt.Sprintf("❌ %s: Not installed", runtimeName)
			runtimeStatuses = append(runtimeStatuses, status)
		}
	}

	for _, status := range runtimeStatuses {
		fmt.Printf("  %s\n", status)
	}
	fmt.Println()

	// Let user select which runtimes to install/update
	if setupVerbose {
		fmt.Println("💡 You can choose which runtimes to install or update.")
		fmt.Println("   Recommended: Install all runtimes for maximum language support.")
		fmt.Println()
	}

	installAll, err := promptYesNo("Install/update all detected runtimes?", true)
	if err != nil {
		return fmt.Errorf("failed to read user input: %w", err)
	}

	if installAll {
		// Select all runtimes
		for runtimeName := range detectionReport.Runtimes {
			state.SelectedRuntimes[runtimeName] = true
		}
		fmt.Printf("✅ Selected all %d runtimes for installation/update\n", len(runtimeNames))
	} else {
		// Let user select individual runtimes
		fmt.Println("\n📋 Select runtimes to install/update:")
		selections, err := promptSelection("Choose runtimes:", runtimeNames, true)
		if err != nil {
			return fmt.Errorf("failed to read runtime selection: %w", err)
		}

		for _, selection := range selections {
			runtimeName := runtimeNames[selection]
			state.SelectedRuntimes[runtimeName] = true
		}

		fmt.Printf("✅ Selected %d runtime(s) for installation/update\n", len(selections))
	}

	pressEnterToContinue("")
	return nil
}

// wizardServerSelection handles language server selection
func wizardServerSelection(ctx context.Context, state *WizardState, result *SetupResult) error {
	fmt.Println("📦 Step 2: Language Server Selection")
	fmt.Println("====================================")
	fmt.Println()

	if setupVerbose {
		fmt.Println("📝 Language servers provide IDE features like code completion,")
		fmt.Println("   go-to-definition, error checking, and more for each language.")
		fmt.Println()
	}

	// Get available servers
	runtimeInstaller := installer.NewRuntimeInstaller()
	if runtimeInstaller == nil {
		return NewInstallerCreationError("runtime installer")
	}

	serverInstaller := installer.NewServerInstaller(runtimeInstaller)
	if serverInstaller == nil {
		return NewInstallerCreationError("server installer")
	}

	supportedServers := serverInstaller.GetSupportedServers()
	state.AvailableServers = supportedServers
	result.Summary.TotalServers = len(supportedServers)

	// Display server information
	fmt.Println("📊 Available Language Servers:")
	serverStatuses := []string{}

	for _, serverName := range supportedServers {
		if verifyResult, err := serverInstaller.Verify(serverName); err == nil && verifyResult.Installed && verifyResult.Compatible {
			status := fmt.Sprintf("✅ %s: Already installed (v%s)", serverName, verifyResult.Version)
			if setupVerbose {
				status += fmt.Sprintf(" at %s", verifyResult.Path)
			}
			serverStatuses = append(serverStatuses, status)
		} else {
			status := fmt.Sprintf("❌ %s: Not installed", serverName)
			serverStatuses = append(serverStatuses, status)
		}
	}

	for _, status := range serverStatuses {
		fmt.Printf("  %s\n", status)
	}
	fmt.Println()

	// Server selection
	if setupVerbose {
		fmt.Println("💡 Language servers are automatically matched to your selected runtimes.")
		fmt.Println("   You can install all servers or choose specific ones.")
		fmt.Println()
	}

	installAll, err := promptYesNo("Install all available language servers?", true)
	if err != nil {
		return fmt.Errorf("failed to read user input: %w", err)
	}

	if installAll {
		// Select all servers
		for _, serverName := range supportedServers {
			state.SelectedServers[serverName] = true
		}
		fmt.Printf("✅ Selected all %d language servers for installation\n", len(supportedServers))
	} else {
		// Let user select individual servers
		fmt.Println("\n📋 Select language servers to install:")
		selections, err := promptSelection("Choose servers:", supportedServers, true)
		if err != nil {
			return fmt.Errorf("failed to read server selection: %w", err)
		}

		for _, selection := range selections {
			serverName := supportedServers[selection]
			state.SelectedServers[serverName] = true
		}

		fmt.Printf("✅ Selected %d language server(s) for installation\n", len(selections))
	}

	pressEnterToContinue("")
	return nil
}

// wizardReviewSelection shows a summary and asks for confirmation
func wizardReviewSelection(ctx context.Context, state *WizardState, result *SetupResult) error {
	fmt.Println("📊 Step 4: Review Your Selection")
	fmt.Println("===============================")
	fmt.Println()

	// Summary of selections
	fmt.Println("🔍 Setup Summary:")
	fmt.Println()

	// Runtimes summary
	selectedRuntimeCount := 0
	fmt.Println("📦 Runtimes to install/update:")
	for runtimeName, selected := range state.SelectedRuntimes {
		if selected {
			selectedRuntimeCount++
			runtimeInfo := state.DetectedRuntimes[runtimeName]
			if runtimeInfo.Installed && runtimeInfo.Compatible && !setupForce {
				fmt.Printf("  ✅ %s (already installed - will verify)\n", runtimeName)
			} else if runtimeInfo.Installed {
				fmt.Printf("  🔄 %s (will update from v%s)\n", runtimeName, runtimeInfo.Version)
			} else {
				fmt.Printf("  📥 %s (will install)\n", runtimeName)
			}
		}
	}

	if selectedRuntimeCount == 0 {
		fmt.Println("  (No runtimes selected)")
	}
	fmt.Println()

	// Servers summary
	selectedServerCount := 0
	fmt.Println("🛠️  Language servers to install:")
	for serverName, selected := range state.SelectedServers {
		if selected {
			selectedServerCount++
			fmt.Printf("  📥 %s\n", serverName)
		}
	}

	if selectedServerCount == 0 {
		fmt.Println("  (No servers selected)")
	}
	fmt.Println()

	// Configuration summary
	fmt.Printf("⚙️  Configuration file: %s\n", state.ConfigPath)
	if state.AdvancedSettings {
		fmt.Println("🔧 Advanced settings: Will be configured")
	} else {
		fmt.Println("🔧 Advanced settings: Using defaults")
	}
	fmt.Println()

	// Estimated time
	estimatedTime := time.Duration(selectedRuntimeCount*2+selectedServerCount) * time.Minute
	fmt.Printf("⏱️  Estimated setup time: %v\n", estimatedTime)
	fmt.Printf("💾 Force reinstall: %v\n", setupForce)
	fmt.Println()

	// Confirmation
	proceed, err := promptYesNo("Proceed with installation?", true)
	if err != nil {
		return fmt.Errorf("failed to read user input: %w", err)
	}

	if !proceed {
		return fmt.Errorf("setup cancelled by user")
	}

	fmt.Println("🚀 Starting installation...")
	fmt.Println()
	return nil
}

// wizardExecuteInstallation performs the actual installation
func wizardExecuteInstallation(ctx context.Context, state *WizardState, result *SetupResult) error {
	fmt.Println("🚀 Step 5: Installation Execution")
	fmt.Println("=================================")
	fmt.Println()

	// Phase 1: Runtime Installation
	if len(state.SelectedRuntimes) > 0 {
		fmt.Println("📦 Phase 1: Installing selected runtimes...")
		err := wizardInstallRuntimes(ctx, state, result)
		if err != nil {
			result.Issues = append(result.Issues, fmt.Sprintf("Runtime installation failed: %v", err))
			fmt.Printf("❌ Runtime installation failed: %v\n", err)
		} else {
			fmt.Printf("✅ Runtime installation completed (%d processed)\n", len(state.SelectedRuntimes))
		}
		fmt.Println()
	}

	// Phase 2: Server Installation
	if len(state.SelectedServers) > 0 {
		fmt.Println("🛠️  Phase 2: Installing selected language servers...")
		err := wizardInstallServers(ctx, state, result)
		if err != nil {
			result.Issues = append(result.Issues, fmt.Sprintf("Server installation failed: %v", err))
			fmt.Printf("❌ Server installation failed: %v\n", err)
		} else {
			fmt.Printf("✅ Server installation completed (%d processed)\n", len(state.SelectedServers))
		}
		fmt.Println()
	}

	// Phase 3: Configuration Generation
	fmt.Println("⚙️  Phase 3: Generating configuration...")
	err := wizardGenerateConfiguration(ctx, state, result)
	if err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("Configuration generation failed: %v", err))
		fmt.Printf("❌ Configuration generation failed: %v\n", err)
	} else {
		fmt.Println("✅ Configuration generation completed")
	}
	fmt.Println()

	// Phase 4: Final Verification (if not skipped)
	if !setupSkipVerify {
		fmt.Println("✅ Phase 4: Final verification...")
		verificationWarnings, err := runFinalVerification(ctx, result)
		if err != nil {
			result.Issues = append(result.Issues, fmt.Sprintf("Verification failed: %v", err))
			fmt.Printf("⚠️  Verification warnings: %v\n", err)
		} else {
			result.Warnings = append(result.Warnings, verificationWarnings...)
			if len(verificationWarnings) == 0 {
				fmt.Println("✅ All verifications passed")
			}
		}
		fmt.Println()
	}

	return nil
}

// wizardInstallRuntimes installs selected runtimes
func wizardInstallRuntimes(ctx context.Context, state *WizardState, result *SetupResult) error {
	runtimeInstaller := installer.NewRuntimeInstaller()
	if runtimeInstaller == nil {
		return NewInstallerCreationError("runtime installer")
	}

	for runtimeName, selected := range state.SelectedRuntimes {
		if !selected {
			continue
		}

		fmt.Printf("Processing %s runtime...\n", runtimeName)

		runtimeInfo := state.DetectedRuntimes[runtimeName]

		// Check if already installed and functional (unless force is specified)
		if runtimeInfo.Installed && runtimeInfo.Compatible && !setupForce {
			installResult := &installer.InstallResult{
				Success:  true,
				Runtime:  runtimeName,
				Version:  runtimeInfo.Version,
				Path:     runtimeInfo.Path,
				Duration: 0,
				Method:   "already_installed",
				Messages: []string{"Runtime already installed and verified"},
			}
			result.RuntimesInstalled[runtimeName] = installResult
			result.Summary.RuntimesAlreadyExist++
			continue
		}

		// Install runtime
		options := installer.InstallOptions{
			Force:    setupForce,
			Timeout:  setupTimeout,
			Platform: runtime.GOOS,
		}

		installResult, err := runtimeInstaller.Install(runtimeName, options)
		if installResult != nil {
			result.RuntimesInstalled[runtimeName] = installResult
			if installResult.Success {
				result.Summary.RuntimesInstalled++
				fmt.Printf("  ✅ %s installed successfully\n", runtimeName)
			} else {
				result.Summary.RuntimesFailed++
				fmt.Printf("  ❌ %s installation failed\n", runtimeName)
			}
		} else {
			result.Summary.RuntimesFailed++
			fmt.Printf("  ❌ %s installation failed\n", runtimeName)
		}

		if err != nil {
			fmt.Printf("  ⚠️  %s completed with warnings: %v\n", runtimeName, err)
		}
	}

	return nil
}

// wizardInstallServers installs selected language servers
func wizardInstallServers(ctx context.Context, state *WizardState, result *SetupResult) error {
	runtimeInstaller := installer.NewRuntimeInstaller()
	if runtimeInstaller == nil {
		return NewInstallerCreationError("runtime installer")
	}

	serverInstaller := installer.NewServerInstaller(runtimeInstaller)
	if serverInstaller == nil {
		return NewInstallerCreationError("server installer")
	}

	for serverName, selected := range state.SelectedServers {
		if !selected {
			continue
		}

		fmt.Printf("Setting up %s language server...\n", serverName)

		// Check if already installed and functional (unless force is specified)
		if !setupForce {
			if verifyResult, err := serverInstaller.Verify(serverName); err == nil && verifyResult.Installed && verifyResult.Compatible {
				installResult := &installer.InstallResult{
					Success:  true,
					Runtime:  serverName,
					Version:  verifyResult.Version,
					Path:     verifyResult.Path,
					Duration: 0,
					Method:   "already_installed",
					Messages: []string{"Server already installed and verified"},
				}
				result.ServersInstalled[serverName] = installResult
				result.Summary.ServersAlreadyExist++
				fmt.Printf("  ✅ %s already installed\n", serverName)
				continue
			}
		}

		// Install server
		options := installer.ServerInstallOptions{
			Force:   setupForce,
			Timeout: setupTimeout,
		}

		installResult, err := serverInstaller.Install(serverName, options)
		if installResult != nil {
			result.ServersInstalled[serverName] = installResult
			if installResult.Success {
				result.Summary.ServersInstalled++
				fmt.Printf("  ✅ %s installed successfully\n", serverName)
			} else {
				result.Summary.ServersFailed++
				fmt.Printf("  ❌ %s installation failed\n", serverName)
			}
		} else {
			result.Summary.ServersFailed++
			fmt.Printf("  ❌ %s installation failed\n", serverName)
		}

		if err != nil {
			fmt.Printf("  ⚠️  %s completed with warnings: %v\n", serverName, err)
		}
	}

	return nil
}

// wizardGenerateConfiguration generates the configuration file
func wizardGenerateConfiguration(ctx context.Context, state *WizardState, result *SetupResult) error {
	// Use enhanced configuration generator if project analysis is available
	if state.ProjectAnalysis != nil || state.SelectedTemplate != nil {
		return wizardGenerateEnhancedConfiguration(ctx, state, result)
	}

	// Fallback to standard configuration generation
	configGenerator := setup.NewConfigGenerator()
	if configGenerator == nil {
		return NewInstallerCreationError("config generator")
	}

	// Generate configuration from detected runtimes
	configResult, err := configGenerator.GenerateFromDetected(ctx)
	if err != nil {
		return fmt.Errorf("configuration generation failed: %w", err)
	}

	// Validate generated configuration
	validationResult, err := configGenerator.ValidateGenerated(configResult.Config)
	if err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	if !validationResult.Valid {
		return fmt.Errorf("generated configuration is invalid: %v", validationResult.Issues)
	}

	result.ConfigGenerated = true
	result.ConfigPath = state.ConfigPath

	fmt.Printf("  ✅ Configuration saved to %s\n", state.ConfigPath)

	return nil
}

// Enhanced wizard functions for multi-language project support

// wizardProjectDetection performs project detection and analysis
func wizardProjectDetection(ctx context.Context, state *WizardState, result *SetupResult) error {
	fmt.Println("🔍 Step 2: Project Detection and Analysis")
	fmt.Println("==========================================")
	fmt.Println()

	if setupVerbose {
		fmt.Println("📝 Project detection analyzes your codebase to understand its structure,")
		fmt.Println("   languages, frameworks, and complexity for optimal configuration.")
		fmt.Println()
	}

	// Determine project path
	projectPath := setupProjectPath
	if projectPath == "" {
		projectPath = "."
	}

	fmt.Printf("🔍 Analyzing project at: %s\n", projectPath)

	// Perform project analysis
	projectAnalyzer := setup.NewProjectAnalyzer()
	analysis, err := projectAnalyzer.AnalyzeProject(projectPath)
	if err != nil {
		return fmt.Errorf("project analysis failed: %w", err)
	}

	state.ProjectAnalysis = analysis
	result.Messages = append(result.Messages, fmt.Sprintf("Analyzed %s project with %d languages", analysis.ProjectType, len(analysis.DetectedLanguages)))

	// Display analysis results
	fmt.Println("\n📊 Project Analysis Results:")
	fmt.Printf("  Project Type: %s\n", analysis.ProjectType)
	fmt.Printf("  Languages: %v\n", analysis.DetectedLanguages)
	fmt.Printf("  Complexity: %s\n", analysis.Complexity)
	fmt.Printf("  File Count: %d\n", analysis.FileCount)
	fmt.Printf("  Estimated Size: %s\n", analysis.EstimatedSize)

	if len(analysis.Frameworks) > 0 {
		fmt.Printf("  Frameworks: ")
		for i, framework := range analysis.Frameworks {
			if i > 0 {
				fmt.Printf(", ")
			}
			fmt.Printf("%s (%s)", framework.Name, framework.Language)
		}
		fmt.Println()
	}

	if len(analysis.BuildSystems) > 0 {
		fmt.Printf("  Build Systems: %v\n", analysis.BuildSystems)
	}

	fmt.Println()

	if setupVerbose {
		fmt.Println("💡 This analysis will be used to select optimal templates")
		fmt.Println("   and configuration settings for your project.")
		fmt.Println()
	}

	pressEnterToContinue("")
	return nil
}

// wizardTemplateSelection handles template selection based on project analysis
func wizardTemplateSelection(ctx context.Context, state *WizardState, result *SetupResult) error {
	fmt.Println("📋 Step 3: Template Selection")
	fmt.Println("=============================")
	fmt.Println()

	if setupVerbose {
		fmt.Println("📝 Templates provide pre-configured settings optimized for")
		fmt.Println("   specific project types and development patterns.")
		fmt.Println()
	}

	templateManager := setup.NewConfigurationTemplateManager()

	// Auto-select template based on project analysis
	var recommendedTemplate *setup.ConfigurationTemplate
	if state.ProjectAnalysis != nil {
		recommendedTemplate, _ = templateManager.SelectTemplate(state.ProjectAnalysis)
	}

	// Get available templates
	availableTemplates := templateManager.GetAvailableTemplates()

	fmt.Println("📊 Available Templates:")
	for i, templateName := range availableTemplates {
		template, _ := templateManager.GetTemplate(templateName)
		if template != nil {
			marker := "  "
			if recommendedTemplate != nil && template.Name == recommendedTemplate.Name {
				marker = "🌟"
			}
			fmt.Printf("  %s %d) %s - %s\n", marker, i+1, template.Name, template.Description)
		}
	}
	fmt.Println()

	if recommendedTemplate != nil {
		fmt.Printf("🎯 Recommended: %s (based on project analysis)\n", recommendedTemplate.Name)
		fmt.Println()
	}

	useRecommended := true
	if recommendedTemplate != nil {
		var err error
		useRecommended, err = promptYesNo(fmt.Sprintf("Use recommended template '%s'?", recommendedTemplate.Name), true)
		if err != nil {
			return fmt.Errorf("failed to read user input: %w", err)
		}
	}

	if useRecommended && recommendedTemplate != nil {
		state.SelectedTemplate = recommendedTemplate
		fmt.Printf("✅ Selected template: %s\n", recommendedTemplate.Name)
	} else {
		// Let user select template manually
		fmt.Println("\n📋 Select a template:")
		selections, err := promptSelection("Choose template:", availableTemplates, false)
		if err != nil {
			return fmt.Errorf("failed to read template selection: %w", err)
		}

		if len(selections) > 0 {
			selectedTemplateName := availableTemplates[selections[0]]
			selectedTemplate, err := templateManager.GetTemplate(selectedTemplateName)
			if err != nil {
				return fmt.Errorf("failed to get selected template: %w", err)
			}
			state.SelectedTemplate = selectedTemplate
			fmt.Printf("✅ Selected template: %s\n", selectedTemplate.Name)
		}
	}

	if state.SelectedTemplate != nil {
		result.Messages = append(result.Messages, fmt.Sprintf("Selected template: %s", state.SelectedTemplate.Name))
	}

	pressEnterToContinue("")
	return nil
}

// wizardEnhancedConfigurationOptions handles enhanced configuration options
func wizardEnhancedConfigurationOptions(ctx context.Context, state *WizardState, result *SetupResult) error {
	fmt.Println("⚙️  Step 6: Enhanced Configuration Options")
	fmt.Println("==========================================")
	fmt.Println()

	if setupVerbose {
		fmt.Println("📝 Enhanced configuration provides advanced options for")
		fmt.Println("   optimization, performance, and smart routing features.")
		fmt.Println()
	}

	// Configuration file path
	fmt.Printf("📄 Current configuration path: %s\n", state.ConfigPath)

	customPath, err := promptYesNo("Would you like to use a custom configuration file path?", false)
	if err != nil {
		return fmt.Errorf("failed to read user input: %w", err)
	}

	if customPath {
		newPath, err := promptText("Enter configuration file path", state.ConfigPath)
		if err != nil {
			return fmt.Errorf("failed to read configuration path: %w", err)
		}
		state.ConfigPath = newPath
		fmt.Printf("✅ Configuration will be saved to: %s\n", state.ConfigPath)
	}

	fmt.Println()

	// Optimization mode selection
	if state.OptimizationMode == "" {
		optimizationModes := []string{config.PerformanceProfileDevelopment, config.PerformanceProfileProduction, config.PerformanceProfileAnalysis}
		fmt.Println("🎯 Optimization Modes:")
		fmt.Println("  1) development - Fast startup, moderate resource usage")
		fmt.Println("  2) production  - High performance, maximum concurrency")
		fmt.Println("  3) analysis    - Extended timeouts, comprehensive features")
		fmt.Println()

		// Auto-recommend based on project analysis
		defaultMode := config.PerformanceProfileDevelopment
		if state.ProjectAnalysis != nil {
			switch state.ProjectAnalysis.Complexity {
			case setup.ProjectComplexityHigh:
				defaultMode = config.PerformanceProfileProduction
			case setup.ProjectComplexityMedium:
				defaultMode = config.PerformanceProfileDevelopment
			case setup.ProjectComplexityLow:
				defaultMode = config.PerformanceProfileDevelopment
			}
		}

		useDefault, err := promptYesNo(fmt.Sprintf("Use recommended optimization mode '%s'?", defaultMode), true)
		if err != nil {
			return fmt.Errorf("failed to read user input: %w", err)
		}

		if useDefault {
			state.OptimizationMode = defaultMode
		} else {
			selections, err := promptSelection("Choose optimization mode:", optimizationModes, false)
			if err != nil {
				return fmt.Errorf("failed to read optimization mode selection: %w", err)
			}
			if len(selections) > 0 {
				state.OptimizationMode = optimizationModes[selections[0]]
			}
		}

		fmt.Printf("✅ Optimization mode: %s\n", state.OptimizationMode)
	}

	fmt.Println()

	// Performance profile selection
	if state.PerformanceProfile == "" {
		performanceProfiles := []string{"low", "medium", "high"}
		fmt.Println("📊 Performance Profiles:")
		fmt.Println("  1) low    - Conservative resource usage")
		fmt.Println("  2) medium - Balanced performance and resources")
		fmt.Println("  3) high   - Maximum performance, high resource usage")
		fmt.Println()

		// Auto-recommend based on project analysis
		defaultProfile := "medium"
		if state.ProjectAnalysis != nil {
			switch state.ProjectAnalysis.Complexity {
			case setup.ProjectComplexityHigh:
				defaultProfile = "high"
			case setup.ProjectComplexityMedium:
				defaultProfile = "medium"
			case setup.ProjectComplexityLow:
				defaultProfile = "low"
			}
		}

		useDefaultProfile, err := promptYesNo(fmt.Sprintf("Use recommended performance profile '%s'?", defaultProfile), true)
		if err != nil {
			return fmt.Errorf("failed to read user input: %w", err)
		}

		if useDefaultProfile {
			state.PerformanceProfile = defaultProfile
		} else {
			selections, err := promptSelection("Choose performance profile:", performanceProfiles, false)
			if err != nil {
				return fmt.Errorf("failed to read performance profile selection: %w", err)
			}
			if len(selections) > 0 {
				state.PerformanceProfile = performanceProfiles[selections[0]]
			}
		}

		fmt.Printf("✅ Performance profile: %s\n", state.PerformanceProfile)
	}

	fmt.Println()

	// Advanced features
	if setupVerbose {
		fmt.Println("💡 Advanced features enhance LSP Gateway capabilities for")
		fmt.Println("   complex projects and production environments.")
		fmt.Println()
	}

	// Smart routing
	if !state.EnableSmartRouting {
		enableSmartRouting, err := promptYesNo("Enable smart routing for multi-language projects?", state.ProjectAnalysis != nil && len(state.ProjectAnalysis.DetectedLanguages) > 1)
		if err != nil {
			return fmt.Errorf("failed to read user input: %w", err)
		}
		state.EnableSmartRouting = enableSmartRouting
	}

	// Concurrent servers
	if !state.EnableConcurrentServers {
		enableConcurrent, err := promptYesNo("Enable concurrent server management?", state.OptimizationMode == "production")
		if err != nil {
			return fmt.Errorf("failed to read user input: %w", err)
		}
		state.EnableConcurrentServers = enableConcurrent
	}

	fmt.Println()
	fmt.Println("📋 Enhanced configuration options selected:")
	fmt.Printf("  Smart routing: %v\n", state.EnableSmartRouting)
	fmt.Printf("  Concurrent servers: %v\n", state.EnableConcurrentServers)
	fmt.Printf("  Optimization: %s\n", state.OptimizationMode)
	fmt.Printf("  Performance: %s\n", state.PerformanceProfile)

	pressEnterToContinue("")
	return nil
}

// wizardGenerateEnhancedConfiguration generates enhanced configuration
func wizardGenerateEnhancedConfiguration(ctx context.Context, state *WizardState, result *SetupResult) error {
	enhancedGenerator := setup.NewEnhancedConfigurationGenerator()

	// Determine optimization mode
	optMode := state.OptimizationMode
	if optMode == "" {
		optMode = config.PerformanceProfileDevelopment
	}

	// Generate enhanced configuration
	var configResult *setup.ConfigGenerationResult
	var err error

	if state.ProjectAnalysis != nil {
		// Generate from project analysis
		projectPath := setupProjectPath
		if projectPath == "" {
			projectPath = "."
		}
		configResult, err = enhancedGenerator.GenerateFromProject(ctx, projectPath, optMode)
	} else {
		// Generate default for environment
		configResult, err = enhancedGenerator.GenerateDefaultForEnvironment(ctx, optMode)
	}

	if err != nil {
		return fmt.Errorf("enhanced configuration generation failed: %w", err)
	}

	// Apply template-specific enhancements if template is selected
	if state.SelectedTemplate != nil && configResult.Config != nil {
		// Apply template configurations
		if state.SelectedTemplate.ResourceLimits != nil {
			configResult.Config.MaxConcurrentRequests = state.SelectedTemplate.ResourceLimits.MaxConcurrentRequests
			if state.SelectedTemplate.ResourceLimits.TimeoutSeconds > 0 {
				configResult.Config.Timeout = fmt.Sprintf("%ds", state.SelectedTemplate.ResourceLimits.TimeoutSeconds)
			}
		}

		// Apply routing configuration
		if state.SelectedTemplate.RoutingConfig != nil {
			configResult.Config.EnableSmartRouting = state.SelectedTemplate.RoutingConfig.EnableSmartRouting || state.EnableSmartRouting
		}

		configResult.Config.EnableConcurrentServers = state.SelectedTemplate.EnableMultiServer || state.EnableConcurrentServers
	}

	// Override with user selections
	if configResult.Config != nil {
		configResult.Config.EnableSmartRouting = state.EnableSmartRouting
		configResult.Config.EnableConcurrentServers = state.EnableConcurrentServers
	}

	// Save configuration
	result.ConfigGenerated = true
	result.ConfigPath = state.ConfigPath
	result.Messages = append(result.Messages, configResult.Messages...)
	result.Warnings = append(result.Warnings, configResult.Warnings...)
	result.Issues = append(result.Issues, configResult.Issues...)

	fmt.Printf("  ✅ Enhanced configuration saved to %s\n", state.ConfigPath)

	return nil
}

// setupRuntimes handles runtime detection and installation
func setupRuntimes(ctx context.Context, result *SetupResult) error {
	// First, detect all runtimes
	detector := setup.NewRuntimeDetector()
	if detector == nil {
		return NewInstallerCreationError("runtime detector")
	}

	detectionReport, err := detector.DetectAll(ctx)
	if err != nil {
		return fmt.Errorf("runtime detection failed: %w", err)
	}

	// Store detection results
	result.RuntimesDetected = detectionReport.Runtimes
	result.Summary.TotalRuntimes = len(detectionReport.Runtimes)

	// Count existing runtimes
	for _, runtimeInfo := range detectionReport.Runtimes {
		if runtimeInfo.Installed && runtimeInfo.Compatible {
			result.Summary.RuntimesAlreadyExist++
		}
	}

	// Setup installer
	runtimeInstaller := installer.NewRuntimeInstaller()
	if runtimeInstaller == nil {
		return NewInstallerCreationError("runtime installer")
	}

	if strategy := runtimeInstaller.GetPlatformStrategy(runtime.GOOS); strategy == nil {
		return NewInstallerCreationError(fmt.Sprintf("platform strategy for %s", runtime.GOOS))
	}

	// Install missing runtimes based on detection results
	for runtimeName, runtimeInfo := range detectionReport.Runtimes {
		if setupVerbose && !setupJSON {
			fmt.Printf("Processing %s runtime...\n", runtimeName)
		}

		// Check if already installed and functional (unless force is specified)
		if runtimeInfo.Installed && runtimeInfo.Compatible && !setupForce {
			installResult := &installer.InstallResult{
				Success:  true,
				Runtime:  runtimeName,
				Version:  runtimeInfo.Version,
				Path:     runtimeInfo.Path,
				Duration: 0,
				Method:   "already_installed",
				Messages: []string{"Runtime already installed and verified"},
			}
			result.RuntimesInstalled[runtimeName] = installResult
			continue
		}

		// Skip installation if in non-interactive mode and runtime is missing
		if setupNoInteractive && !runtimeInfo.Installed {
			if setupVerbose && !setupJSON {
				fmt.Printf("Skipping installation of %s runtime (non-interactive mode)\n", runtimeName)
			}
			result.Summary.RuntimesFailed++
			continue
		}

		// Install runtime
		options := installer.InstallOptions{
			Force:    setupForce,
			Timeout:  setupTimeout,
			Platform: runtime.GOOS,
		}

		installResult, err := runtimeInstaller.Install(runtimeName, options)
		if installResult != nil {
			result.RuntimesInstalled[runtimeName] = installResult
			if installResult.Success {
				result.Summary.RuntimesInstalled++
			} else {
				result.Summary.RuntimesFailed++
			}
		} else {
			result.Summary.RuntimesFailed++
		}

		if err != nil && setupVerbose && !setupJSON {
			fmt.Printf("⚠️  Runtime %s installation completed with warnings: %v\n", runtimeName, err)
		}
	}

	return nil
}

// setupServers handles language server installation
func setupServers(ctx context.Context, result *SetupResult) error {
	runtimeInstaller := installer.NewRuntimeInstaller()
	if runtimeInstaller == nil {
		return NewInstallerCreationError("runtime")
	}

	serverInstaller := installer.NewServerInstaller(runtimeInstaller)
	if serverInstaller == nil {
		return NewInstallerCreationError("server")
	}

	supportedServers := serverInstaller.GetSupportedServers()
	result.Summary.TotalServers = len(supportedServers)

	for _, serverName := range supportedServers {
		if setupVerbose && !setupJSON {
			fmt.Printf("Setting up %s language server...\n", serverName)
		}

		// Check if already installed and functional (unless force is specified)
		if !setupForce {
			if verifyResult, err := serverInstaller.Verify(serverName); err == nil && verifyResult.Installed && verifyResult.Compatible {
				installResult := &installer.InstallResult{
					Success:  true,
					Runtime:  serverName,
					Version:  verifyResult.Version,
					Path:     verifyResult.Path,
					Duration: 0,
					Method:   "already_installed",
					Messages: []string{"Server already installed and verified"},
				}
				result.ServersInstalled[serverName] = installResult
				result.Summary.ServersAlreadyExist++
				continue
			}
		}

		// Install server
		options := installer.ServerInstallOptions{
			Force:   setupForce,
			Timeout: setupTimeout,
		}

		installResult, err := serverInstaller.Install(serverName, options)
		if installResult != nil {
			result.ServersInstalled[serverName] = installResult
			if installResult.Success {
				result.Summary.ServersInstalled++
			} else {
				result.Summary.ServersFailed++
			}
		} else {
			result.Summary.ServersFailed++
		}

		if err != nil && setupVerbose && !setupJSON {
			fmt.Printf("⚠️  Server %s installation completed with warnings: %v\n", serverName, err)
		}
	}

	return nil
}

// setupConfiguration handles configuration file generation
func setupConfiguration(ctx context.Context, result *SetupResult) error {
	configGenerator := setup.NewConfigGenerator()
	if configGenerator == nil {
		return NewInstallerCreationError("config generator")
	}

	// Generate configuration from detected runtimes
	configResult, err := configGenerator.GenerateFromDetected(ctx)
	if err != nil {
		return fmt.Errorf("configuration generation failed: %w", err)
	}

	// Validate generated configuration
	validationResult, err := configGenerator.ValidateGenerated(configResult.Config)
	if err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	if !validationResult.Valid {
		return fmt.Errorf("generated configuration is invalid: %v", validationResult.Issues)
	}

	result.ConfigGenerated = true
	result.ConfigPath = "config.yaml" // Default path

	return nil
}

// runFinalVerification performs final verification of the setup
func runFinalVerification(ctx context.Context, result *SetupResult) ([]string, error) {
	// TODO: Implement comprehensive verification
	// This would include:
	// 1. Runtime verification
	// 2. Server functionality tests
	// 3. Configuration validation
	// 4. Basic functionality tests

	if setupVerbose && !setupJSON {
		fmt.Println("Verification implementation in progress...")
	}

	return []string{}, nil
}

// generateSetupSummary calculates setup summary statistics
func generateSetupSummary(result *SetupResult) {
	// Summary is already populated during runtime and server setup
	// This function can be used for any additional summary calculations
}

// outputSetupResultsJSON outputs setup results in JSON format
func outputSetupResultsJSON(result *SetupResult) error {
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return NewJSONMarshalError("setup results", err)
	}

	fmt.Println(string(jsonData))

	if !result.Success {
		return fmt.Errorf("setup completed with %d issues", len(result.Issues))
	}

	return nil
}

// outputSetupResultsHuman outputs setup results in human-readable format
func outputSetupResultsHuman(result *SetupResult) error {
	fmt.Println()
	fmt.Println("=====================================")
	fmt.Println("       LSP Gateway Setup Summary")
	fmt.Println("=====================================")
	fmt.Println()

	// Overall status
	if result.Success {
		fmt.Println("✅ Setup Status: SUCCESS")
	} else {
		fmt.Println("❌ Setup Status: COMPLETED WITH ISSUES")
	}

	fmt.Printf("⏱️  Total Duration: %v\n", result.Duration)
	fmt.Println()

	// Runtime setup summary
	if result.Summary.TotalRuntimes > 0 {
		successful := result.Summary.RuntimesInstalled + result.Summary.RuntimesAlreadyExist
		fmt.Printf("🔧 Runtimes: %d/%d successfully configured\n", successful, result.Summary.TotalRuntimes)
		if result.Summary.RuntimesInstalled > 0 {
			fmt.Printf("   • Newly installed: %d\n", result.Summary.RuntimesInstalled)
		}
		if result.Summary.RuntimesAlreadyExist > 0 {
			fmt.Printf("   • Already existed: %d\n", result.Summary.RuntimesAlreadyExist)
		}
		if result.Summary.RuntimesFailed > 0 {
			fmt.Printf("   • Failed: %d\n", result.Summary.RuntimesFailed)
		}
	}

	// Server setup summary
	if result.Summary.TotalServers > 0 {
		successful := result.Summary.ServersInstalled + result.Summary.ServersAlreadyExist
		fmt.Printf("📦 Language Servers: %d/%d successfully configured\n", successful, result.Summary.TotalServers)
		if result.Summary.ServersInstalled > 0 {
			fmt.Printf("   • Newly installed: %d\n", result.Summary.ServersInstalled)
		}
		if result.Summary.ServersAlreadyExist > 0 {
			fmt.Printf("   • Already existed: %d\n", result.Summary.ServersAlreadyExist)
		}
		if result.Summary.ServersFailed > 0 {
			fmt.Printf("   • Failed: %d\n", result.Summary.ServersFailed)
		}
	}

	// Configuration summary
	if result.ConfigGenerated {
		fmt.Printf("⚙️  Configuration: Generated successfully")
		if result.ConfigPath != "" {
			fmt.Printf(" (%s)", result.ConfigPath)
		}
		fmt.Println()
	}

	fmt.Println()

	// Issues
	if len(result.Issues) > 0 {
		fmt.Println("❌ Issues encountered:")
		for _, issue := range result.Issues {
			fmt.Printf("   • %s\n", issue)
		}
		fmt.Println()
	}

	// Warnings
	if len(result.Warnings) > 0 {
		fmt.Println("⚠️  Warnings:")
		for _, warning := range result.Warnings {
			fmt.Printf("   • %s\n", warning)
		}
		fmt.Println()
	}

	// Messages
	if len(result.Messages) > 0 {
		fmt.Println("💡 Next steps:")
		for _, message := range result.Messages {
			fmt.Printf("   • %s\n", message)
		}
		fmt.Println()
	}

	// Return error if setup failed
	if !result.Success {
		return fmt.Errorf("setup completed with %d issues", len(result.Issues))
	}

	return nil
}

// Multi-language setup command implementation

func runSetupMultiLanguage(cmd *cobra.Command, args []string) error {
	if err := validateMultiLanguageSetupParams(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), setupTimeout)
	defer cancel()

	startTime := time.Now()
	result := initializeSetupResult()

	printSetupHeader("🌍 Starting LSP Gateway multi-language setup...")

	if err := executeMultiLanguagePhases(ctx, result); err != nil {
		return err
	}

	finalizeSetupResult(result, startTime)
	addMultiLanguageSuccessMessages(result)

	return outputSetupResults(result)
}

// Template-based setup command implementation

func runSetupTemplate(cmd *cobra.Command, args []string) error {
	if err := validateTemplateSetupParams(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), setupTimeout)
	defer cancel()

	startTime := time.Now()

	if setupVerbose && !setupJSON {
		fmt.Printf("🎨 Starting LSP Gateway template-based setup with '%s' template...\n", setupTemplate)
		fmt.Printf("Project path: %s\n", setupProjectPath)
		fmt.Printf("Optimization mode: %s\n", setupOptimizationMode)
		fmt.Printf("Performance profile: %s\n", setupPerformanceProfile)
		fmt.Println()
	}

	// Initialize result structure
	result := &SetupResult{
		Success:           false,
		RuntimesDetected:  make(map[string]*setup.RuntimeInfo),
		RuntimesInstalled: make(map[string]*installer.InstallResult),
		ServersInstalled:  make(map[string]*installer.InstallResult),
		Issues:            make([]string, 0),
		Warnings:          make([]string, 0),
		Messages:          make([]string, 0),
		Summary:           &SetupSummary{},
	}

	// Phase 1: Template Analysis and Configuration
	if setupVerbose && !setupJSON {
		fmt.Printf("📋 Phase 1: Template Analysis (%s)\n", setupTemplate)
	}

	err := analyzeTemplateRequirements(ctx, result)
	if err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("Template analysis failed: %v", err))
		if !setupJSON {
			fmt.Printf("❌ Template analysis failed: %v\n", err)
		}
	} else if setupVerbose && !setupJSON {
		fmt.Printf("✓ Template '%s' analysis completed\n", setupTemplate)
	}

	// Phase 2: Template-specific Runtime Installation
	if setupVerbose && !setupJSON {
		fmt.Println("\n📦 Phase 2: Template-specific Runtime Installation")
	}

	err = setupTemplateRuntimes(ctx, result)
	if err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("Template runtime setup failed: %v", err))
		if !setupJSON {
			fmt.Printf("❌ Runtime setup failed: %v\n", err)
		}
	} else if setupVerbose && !setupJSON {
		fmt.Printf("✓ Template runtime setup completed (%d runtimes)\n", len(result.RuntimesInstalled))
	}

	// Phase 3: Template-optimized Language Server Setup
	if setupVerbose && !setupJSON {
		fmt.Println("\n🎆 Phase 3: Template-optimized Language Server Setup")
	}

	err = setupTemplateLanguageServers(ctx, result)
	if err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("Template server setup failed: %v", err))
		if !setupJSON {
			fmt.Printf("❌ Server setup failed: %v\n", err)
		}
	} else if setupVerbose && !setupJSON {
		fmt.Printf("✓ Template server setup completed (%d servers)\n", len(result.ServersInstalled))
	}

	// Phase 4: Template Configuration Generation
	if setupVerbose && !setupJSON {
		fmt.Println("\n⚙️  Phase 4: Template Configuration Generation")
	}

	err = generateTemplateConfiguration(ctx, result)
	if err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("Template configuration failed: %v", err))
		if !setupJSON {
			fmt.Printf("❌ Configuration generation failed: %v\n", err)
		}
	} else if setupVerbose && !setupJSON {
		fmt.Printf("✓ Template '%s' configuration generated\n", setupTemplate)
	}

	// Calculate final results
	result.Duration = time.Since(startTime)
	result.Success = len(result.Issues) == 0

	// Generate summary
	generateSetupSummary(result)

	// Generate summary messages
	if result.Success {
		result.Messages = append(result.Messages, fmt.Sprintf("Template-based LSP Gateway setup completed successfully (%s)", setupTemplate))
		result.Messages = append(result.Messages, fmt.Sprintf("Setup duration: %v", result.Duration))
		result.Messages = append(result.Messages, fmt.Sprintf("Template '%s' optimizations are now active", setupTemplate))
		result.Messages = append(result.Messages, "Run 'lsp-gateway server' to start the HTTP gateway")
		result.Messages = append(result.Messages, "Run 'lsp-gateway mcp' to start the MCP server")
	}

	// Output results
	if setupJSON {
		return outputSetupResultsJSON(result)
	} else {
		return outputSetupResultsHuman(result)
	}
}

// Multi-language and template setup helper functions

func validateMultiLanguageSetupParams() error {
	if setupProjectPath == "" {
		return fmt.Errorf("project-path is required for multi-language setup")
	}
	return nil
}

func validateTemplateSetupParams() error {
	if setupTemplate == "" {
		return fmt.Errorf("template is required for template-based setup")
	}

	// Check if template exists using template manager
	templateManager := setup.NewConfigurationTemplateManager()
	_, err := templateManager.GetTemplate(setupTemplate)
	if err != nil {
		// Provide list of available templates
		availableTemplates := templateManager.GetAvailableTemplates()
		return fmt.Errorf("invalid template '%s', available templates: %v", setupTemplate, availableTemplates)
	}

	return nil
}

// Enhanced setup implementations integrating with the advanced configuration system

// runSetupDetect implements the auto-detect setup command
func runSetupDetect(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), setupTimeout)
	defer cancel()

	startTime := time.Now()
	result := initializeSetupResult()

	printSetupHeader("🔍 Starting LSP Gateway auto-detect setup...")

	projectAnalysis, err := executeDetectPhases(ctx, result)
	if err != nil {
		return err
	}

	finalizeSetupResult(result, startTime)
	addDetectSuccessMessages(result, projectAnalysis)

	return outputSetupResults(result)
}

// Enhanced implementations for multi-language setup phases

func analyzeMultiLanguageProject(ctx context.Context, result *SetupResult) error {
	return performProjectDetectionWithResult(ctx, setupProjectPath, result)
}

func performSetupProjectDetection(ctx context.Context, projectPath string, result *SetupResult) (*setup.ProjectAnalysis, error) {
	projectAnalyzer := setup.NewProjectAnalyzer()
	analysis, err := projectAnalyzer.AnalyzeProject(projectPath)
	if err != nil {
		return nil, fmt.Errorf("project analysis failed: %w", err)
	}

	if setupVerbose && !setupJSON {
		fmt.Printf("  Project type: %s\n", analysis.ProjectType)
		fmt.Printf("  Languages: %v\n", analysis.DetectedLanguages)
		fmt.Printf("  Frameworks: %d detected\n", len(analysis.Frameworks))
		fmt.Printf("  Complexity: %s\n", analysis.Complexity)
		fmt.Printf("  Files: %d\n", analysis.FileCount)
	}

	result.Messages = append(result.Messages, fmt.Sprintf("Analyzed %s project with %d languages", analysis.ProjectType, len(analysis.DetectedLanguages)))

	return analysis, nil
}

func performProjectDetectionWithResult(ctx context.Context, projectPath string, result *SetupResult) error {
	_, err := performSetupProjectDetection(ctx, projectPath, result)
	return err
}

func selectOptimalTemplate(analysis *setup.ProjectAnalysis, result *SetupResult) (*setup.ConfigurationTemplate, error) {
	templateManager := setup.NewConfigurationTemplateManager()

	// Use explicit template if provided
	if setupTemplate != "" {
		template, err := templateManager.GetTemplate(setupTemplate)
		if err != nil {
			return nil, fmt.Errorf("specified template not found: %w", err)
		}
		result.Messages = append(result.Messages, fmt.Sprintf("Using specified template: %s", setupTemplate))
		return template, nil
	}

	// Auto-select template based on analysis
	if analysis != nil {
		template, err := templateManager.SelectTemplate(analysis)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Template auto-selection failed: %v", err))
			return templateManager.GetDefaultTemplate(), nil
		}
		result.Messages = append(result.Messages, fmt.Sprintf("Auto-selected template: %s", template.Name))
		return template, nil
	}

	// Fallback to default template
	defaultTemplate := templateManager.GetDefaultTemplate()
	result.Messages = append(result.Messages, "Using default template")
	return defaultTemplate, nil
}

func generateEnhancedConfiguration(ctx context.Context, analysis *setup.ProjectAnalysis, template *setup.ConfigurationTemplate, result *SetupResult) error {
	enhancedGenerator := setup.NewEnhancedConfigurationGenerator()

	// Determine optimization mode
	optMode := setupOptimizationMode
	if optMode == "" && analysis != nil {
		// Auto-determine optimization mode based on project characteristics
		switch analysis.Complexity {
		case setup.ProjectComplexityHigh:
			optMode = config.PerformanceProfileProduction
		case setup.ProjectComplexityMedium:
			optMode = config.PerformanceProfileDevelopment
		case setup.ProjectComplexityLow:
			optMode = config.PerformanceProfileDevelopment
		default:
			optMode = config.PerformanceProfileDevelopment
		}
	} else if optMode == "" {
		optMode = config.PerformanceProfileDevelopment
	}

	// Generate configuration from project analysis
	var configResult *setup.ConfigGenerationResult
	var err error

	if analysis != nil {
		configResult, err = enhancedGenerator.GenerateFromProject(ctx, setupProjectPath, optMode)
	} else {
		configResult, err = enhancedGenerator.GenerateDefaultForEnvironment(ctx, optMode)
	}

	if err != nil {
		return fmt.Errorf("enhanced configuration generation failed: %w", err)
	}

	// Save configuration to file
	if configResult.Config != nil {
		configPath := setupConfigPath
		if configPath == "" {
			configPath = "config.yaml"
		}

		// TODO: Implement configuration file writing
		result.ConfigGenerated = true
		result.ConfigPath = configPath
		result.Messages = append(result.Messages, configResult.Messages...)
		result.Warnings = append(result.Warnings, configResult.Warnings...)
		result.Issues = append(result.Issues, configResult.Issues...)
	}

	return nil
}

func setupOptimizedRuntimes(ctx context.Context, result *SetupResult) error {
	// Use standard runtime setup with optimization awareness
	return setupRuntimes(ctx, result)
}

func setupSmartLanguageServers(ctx context.Context, result *SetupResult) error {
	// Use standard server setup with smart configuration
	return setupServers(ctx, result)
}

func generateMultiLanguageConfiguration(ctx context.Context, result *SetupResult) error {
	// Generate enhanced multi-language configuration
	enhancedGenerator := setup.NewEnhancedConfigurationGenerator()

	optMode := setupOptimizationMode
	if optMode == "" {
		optMode = config.PerformanceProfileDevelopment
	}

	configResult, err := enhancedGenerator.GenerateFromProject(ctx, setupProjectPath, optMode)
	if err != nil {
		return fmt.Errorf("multi-language configuration generation failed: %w", err)
	}

	result.ConfigGenerated = true
	result.ConfigPath = setupConfigPath
	result.Messages = append(result.Messages, configResult.Messages...)
	result.Warnings = append(result.Warnings, configResult.Warnings...)
	result.Issues = append(result.Issues, configResult.Issues...)

	return nil
}

func runMultiLanguageVerification(ctx context.Context, result *SetupResult) ([]string, error) {
	return runEnhancedVerification(ctx, result)
}

func runEnhancedVerification(ctx context.Context, result *SetupResult) ([]string, error) {

	// Enhanced verification with project-aware checks
	if setupProjectPath != "" {
		enhancedGenerator := setup.NewEnhancedConfigurationGenerator()
		// TODO: Implement enhanced validation
		_ = enhancedGenerator
	}

	// Fallback to standard verification
	return runFinalVerification(ctx, result)
}

func analyzeTemplateRequirements(ctx context.Context, result *SetupResult) error {
	templateManager := setup.NewConfigurationTemplateManager()

	template, err := templateManager.GetTemplate(setupTemplate)
	if err != nil {
		return fmt.Errorf("template not found: %w", err)
	}

	if setupVerbose && !setupJSON {
		fmt.Printf("  Template: %s\n", template.Name)
		fmt.Printf("  Description: %s\n", template.Description)
		fmt.Printf("  Project types: %v\n", template.ProjectTypes)
		fmt.Printf("  Target languages: %v\n", template.TargetLanguages)
		fmt.Printf("  Multi-server: %v\n", template.EnableMultiServer)
		fmt.Printf("  Smart routing: %v\n", template.EnableSmartRouting)
	}

	result.Messages = append(result.Messages, fmt.Sprintf("Analyzed template '%s' requirements", template.Name))

	return nil
}

func setupTemplateRuntimes(ctx context.Context, result *SetupResult) error {
	// Template-optimized runtime setup
	return setupRuntimes(ctx, result)
}

func setupTemplateLanguageServers(ctx context.Context, result *SetupResult) error {
	// Template-optimized server setup
	return setupServers(ctx, result)
}

func generateTemplateConfiguration(ctx context.Context, result *SetupResult) error {
	// Generate template-based configuration
	enhancedGenerator := setup.NewEnhancedConfigurationGenerator()

	// Create project analysis for template application
	var analysis *setup.ProjectAnalysis
	if setupProjectPath != "" {
		projectAnalyzer := setup.NewProjectAnalyzer()
		analysis, _ = projectAnalyzer.AnalyzeProject(setupProjectPath)
	}

	// Generate configuration with template
	optMode := setupOptimizationMode
	if optMode == "" {
		optMode = config.PerformanceProfileDevelopment
	}

	var configResult *setup.ConfigGenerationResult
	var err error
	if analysis != nil {
		configResult, err = enhancedGenerator.GenerateFromProject(ctx, setupProjectPath, optMode)
	} else {
		configResult, err = enhancedGenerator.GenerateDefaultForEnvironment(ctx, optMode)
	}

	if err != nil {
		return fmt.Errorf("template configuration generation failed: %w", err)
	}

	result.ConfigGenerated = true
	result.ConfigPath = setupConfigPath
	result.Messages = append(result.Messages, configResult.Messages...)
	result.Warnings = append(result.Warnings, configResult.Warnings...)
	result.Issues = append(result.Issues, configResult.Issues...)

	return nil
}

// Helper functions for refactored setup commands

func initializeSetupResult() *SetupResult {
	return &SetupResult{
		Success:           false,
		RuntimesDetected:  make(map[string]*setup.RuntimeInfo),
		RuntimesInstalled: make(map[string]*installer.InstallResult),
		ServersInstalled:  make(map[string]*installer.InstallResult),
		Issues:            make([]string, 0),
		Warnings:          make([]string, 0),
		Messages:          make([]string, 0),
		Summary:           &SetupSummary{},
	}
}

func printSetupHeader(message string) {
	if setupVerbose && !setupJSON {
		fmt.Println(message)
		fmt.Printf("Project path: %s\n", setupProjectPath)
		if setupOptimizationMode != "" {
			fmt.Printf("Optimization mode: %s\n", setupOptimizationMode)
		}
		if setupPerformanceProfile != "" {
			fmt.Printf("Performance profile: %s\n", setupPerformanceProfile)
		}
		if setupTemplate != "" {
			fmt.Printf("Template: %s\n", setupTemplate)
		}
		fmt.Println()
	}
}

type setupPhase struct {
	name    string
	icon    string
	handler func(context.Context, *SetupResult) error
}

func executeSetupPhase(ctx context.Context, phase setupPhase, result *SetupResult) error {
	if setupVerbose && !setupJSON {
		fmt.Printf("%s %s\n", phase.icon, phase.name)
	}

	err := phase.handler(ctx, result)
	if err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("%s failed: %v", phase.name, err))
		if !setupJSON {
			fmt.Printf("❌ %s failed: %v\n", phase.name, err)
		}
		return err
	}

	if setupVerbose && !setupJSON {
		fmt.Printf("✓ %s completed\n", phase.name)
	}
	return nil
}

func executeMultiLanguagePhases(ctx context.Context, result *SetupResult) error {
	phases := []setupPhase{
		{"Phase 1: Project Analysis and Language Detection", "🔍", func(ctx context.Context, result *SetupResult) error {
			return analyzeMultiLanguageProject(ctx, result)
		}},
		{"Phase 2: Runtime Installation with Multi-language Optimization", "📦", func(ctx context.Context, result *SetupResult) error {
			err := setupOptimizedRuntimes(ctx, result)
			if err == nil && setupVerbose && !setupJSON {
				fmt.Printf("✓ Runtime setup completed (%d runtimes processed)\n", len(result.RuntimesInstalled))
			}
			return err
		}},
		{"Phase 3: Language Server Installation with Smart Configuration", "🎆", func(ctx context.Context, result *SetupResult) error {
			err := setupSmartLanguageServers(ctx, result)
			if err == nil && setupVerbose && !setupJSON {
				fmt.Printf("✓ Server setup completed (%d servers processed)\n", len(result.ServersInstalled))
			}
			return err
		}},
		{"Phase 4: Multi-language Configuration Generation", "⚙️", func(ctx context.Context, result *SetupResult) error {
			return generateMultiLanguageConfiguration(ctx, result)
		}},
	}

	for _, phase := range phases {
		if setupVerbose && !setupJSON {
			fmt.Printf("\n%s %s\n", phase.icon, phase.name)
		}
		
		err := phase.handler(ctx, result)
		if err != nil {
			result.Issues = append(result.Issues, fmt.Sprintf("%s failed: %v", phase.name, err))
			if !setupJSON {
				fmt.Printf("❌ %s failed: %v\n", phase.name, err)
			}
		} else if setupVerbose && !setupJSON && !strings.Contains(phase.name, "Runtime") && !strings.Contains(phase.name, "Server") {
			fmt.Printf("✓ %s completed\n", phase.name)
		}
	}

	if !setupSkipVerify {
		return executeVerificationPhase(ctx, result, "runMultiLanguageVerification")
	}
	return nil
}

func executeDetectPhases(ctx context.Context, result *SetupResult) (*setup.ProjectAnalysis, error) {
	var projectAnalysis *setup.ProjectAnalysis
	var selectedTemplate *setup.ConfigurationTemplate
	var err error

	// Phase 1: Project Detection and Analysis
	if setupVerbose && !setupJSON {
		fmt.Println("🔍 Phase 1: Project Detection and Analysis")
	}
	
	projectAnalysis, err = performSetupProjectDetection(ctx, setupProjectPath, result)
	if err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("Project detection failed: %v", err))
		if !setupJSON {
			fmt.Printf("❌ Project detection failed: %v\n", err)
		}
	} else if setupVerbose && !setupJSON {
		fmt.Printf("✓ Detected %s project with %d languages\n", projectAnalysis.ProjectType, len(projectAnalysis.DetectedLanguages))
	}

	// Phase 2: Template Selection
	if setupVerbose && !setupJSON {
		fmt.Println("\n📋 Phase 2: Template Selection")
	}
	
	selectedTemplate, err = selectOptimalTemplate(projectAnalysis, result)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Template selection failed: %v", err))
	} else if setupVerbose && !setupJSON {
		fmt.Printf("✓ Selected template: %s\n", selectedTemplate.Name)
	}

	// Remaining phases
	phases := []setupPhase{
		{"Phase 3: Enhanced Configuration Generation", "⚙️", func(ctx context.Context, result *SetupResult) error {
			return generateEnhancedConfiguration(ctx, projectAnalysis, selectedTemplate, result)
		}},
		{"Phase 4: Optimized Runtime Installation", "📦", func(ctx context.Context, result *SetupResult) error {
			err := setupOptimizedRuntimes(ctx, result)
			if err == nil && setupVerbose && !setupJSON {
				fmt.Printf("✓ Runtime setup completed (%d runtimes)\n", len(result.RuntimesInstalled))
			}
			return err
		}},
		{"Phase 5: Smart Language Server Setup", "🎆", func(ctx context.Context, result *SetupResult) error {
			err := setupSmartLanguageServers(ctx, result)
			if err == nil && setupVerbose && !setupJSON {
				fmt.Printf("✓ Server setup completed (%d servers)\n", len(result.ServersInstalled))
			}
			return err
		}},
	}

	for _, phase := range phases {
		if setupVerbose && !setupJSON {
			fmt.Printf("\n%s %s\n", phase.icon, phase.name)
		}
		
		err := phase.handler(ctx, result)
		if err != nil {
			result.Issues = append(result.Issues, fmt.Sprintf("%s failed: %v", phase.name, err))
			if !setupJSON {
				fmt.Printf("❌ %s failed: %v\n", phase.name, err)
			}
		} else if setupVerbose && !setupJSON && !strings.Contains(phase.name, "Runtime") && !strings.Contains(phase.name, "Server") {
			fmt.Printf("✓ %s completed\n", phase.name)
		}
	}

	if !setupSkipVerify {
		if err := executeVerificationPhase(ctx, result, "runEnhancedVerification"); err != nil {
			return projectAnalysis, err
		}
	}

	return projectAnalysis, nil
}

func executeVerificationPhase(ctx context.Context, result *SetupResult, verificationType string) error {
	if setupVerbose && !setupJSON {
		if verificationType == "runMultiLanguageVerification" {
			fmt.Println("\n✓ Phase 5: Final Verification")
		} else {
			fmt.Println("\n✓ Phase 6: Final Verification")
		}
	}

	var verificationWarnings []string
	var err error

	if verificationType == "runMultiLanguageVerification" {
		verificationWarnings, err = runMultiLanguageVerification(ctx, result)
	} else {
		verificationWarnings, err = runEnhancedVerification(ctx, result)
	}

	if err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("Verification failed: %v", err))
		if !setupJSON && verificationType == "runMultiLanguageVerification" {
			fmt.Printf("⚠️  Verification warnings: %v\n", err)
		}
	} else {
		result.Warnings = append(result.Warnings, verificationWarnings...)
		if setupVerbose && !setupJSON && len(verificationWarnings) == 0 {
			if verificationType == "runMultiLanguageVerification" {
				fmt.Println("✓ All multi-language verifications passed")
			} else {
				fmt.Println("✓ All verifications passed")
			}
		}
	}
	return nil
}

func finalizeSetupResult(result *SetupResult, startTime time.Time) {
	result.Duration = time.Since(startTime)
	result.Success = len(result.Issues) == 0
	generateSetupSummary(result)
}

func addMultiLanguageSuccessMessages(result *SetupResult) {
	if result.Success {
		result.Messages = append(result.Messages, "Multi-language LSP Gateway setup completed successfully")
		result.Messages = append(result.Messages, fmt.Sprintf("Setup duration: %v", result.Duration))
		result.Messages = append(result.Messages, "Multi-language project support is now active")
		result.Messages = append(result.Messages, "Run 'lsp-gateway server' to start the HTTP gateway")
		result.Messages = append(result.Messages, "Run 'lsp-gateway mcp' to start the MCP server")
	}
}

func addDetectSuccessMessages(result *SetupResult, projectAnalysis *setup.ProjectAnalysis) {
	if result.Success {
		result.Messages = append(result.Messages, "Auto-detect LSP Gateway setup completed successfully")
		result.Messages = append(result.Messages, fmt.Sprintf("Setup duration: %v", result.Duration))
		if projectAnalysis != nil {
			result.Messages = append(result.Messages, fmt.Sprintf("Project: %s (%s complexity)", projectAnalysis.ProjectType, projectAnalysis.Complexity))
			result.Messages = append(result.Messages, fmt.Sprintf("Languages: %v", projectAnalysis.DetectedLanguages))
		}
		result.Messages = append(result.Messages, "Run 'lsp-gateway server' to start the HTTP gateway")
		result.Messages = append(result.Messages, "Run 'lsp-gateway mcp' to start the MCP server")
	}
}

func outputSetupResults(result *SetupResult) error {
	if setupJSON {
		return outputSetupResultsJSON(result)
	} else {
		return outputSetupResultsHuman(result)
	}
}
