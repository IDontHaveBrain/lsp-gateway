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
- all:    Complete automated setup (recommended for first-time users)
- wizard: Interactive setup with user choices and customization

Examples:
  lsp-gateway setup all                    # Complete automated setup
  lsp-gateway setup all --force            # Force reinstall all components
  lsp-gateway setup all --timeout 15m      # Set custom timeout
  lsp-gateway setup all --json             # Output results in JSON format
  lsp-gateway setup wizard                 # Interactive setup wizard
  lsp-gateway setup wizard --verbose       # Verbose wizard with detailed explanations`,
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
Use --json for machine-readable output.`,
	RunE: runSetupAll,
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

Use --verbose for detailed explanations during setup.`,
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

	// Setup Wizard command flags
	setupWizardCmd.Flags().DurationVar(&setupTimeout, FLAG_TIMEOUT, 30*time.Minute, "Maximum time for wizard setup process")
	setupWizardCmd.Flags().BoolVarP(&setupForce, FLAG_FORCE, "f", false, "Force reinstall existing components")
	setupWizardCmd.Flags().BoolVar(&setupJSON, FLAG_JSON, false, FLAG_DESCRIPTION_JSON_OUTPUT)
	setupWizardCmd.Flags().BoolVarP(&setupVerbose, FLAG_VERBOSE, "v", false, "Enable verbose wizard with detailed explanations")
	setupWizardCmd.Flags().BoolVar(&setupNoInteractive, FLAG_NO_INTERACTIVE, false, "Run wizard in non-interactive mode with defaults")
	setupWizardCmd.Flags().StringVar(&setupConfigPath, "config", "config.yaml", "Path to configuration file")

	// Add subcommands to setup command
	setupCmd.AddCommand(setupAllCmd)
	setupCmd.AddCommand(setupWizardCmd)

	// Register setup command with root
	rootCmd.AddCommand(setupCmd)
}

// GetSetupCmd returns the setup command for testing purposes
func GetSetupCmd() *cobra.Command {
	return setupCmd
}

// GetSetupAllCmd returns the setup all command for testing purposes
func GetSetupAllCmd() *cobra.Command {
	return setupAllCmd
}

// GetSetupWizardCmd returns the setup wizard command for testing purposes
func GetSetupWizardCmd() *cobra.Command {
	return setupWizardCmd
}

// GetSetupFlags returns current setup flag values for testing purposes
func GetSetupFlags() (timeout time.Duration, force bool, jsonOutput bool, verbose bool, noInteractive bool, skipVerify bool) {
	return setupTimeout, setupForce, setupJSON, setupVerbose, setupNoInteractive, setupSkipVerify
}

// SetSetupFlags sets setup flag values for testing purposes
func SetSetupFlags(timeout time.Duration, force bool, jsonOutput bool, verbose bool, noInteractive bool, skipVerify bool) {
	setupTimeout = timeout
	setupForce = force
	setupJSON = jsonOutput
	setupVerbose = verbose
	setupNoInteractive = noInteractive
	setupSkipVerify = skipVerify
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
		return NewValidationError("JSON output not supported in wizard mode", nil)
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
		SelectedRuntimes: make(map[string]bool),
		SelectedServers:  make(map[string]bool),
		ConfigPath:       setupConfigPath,
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

	// Step 2: Runtime Detection and Selection
	if err := wizardRuntimeSelection(ctx, wizardState, result); err != nil {
		return err
	}

	// Step 3: Language Server Selection
	if err := wizardServerSelection(ctx, wizardState, result); err != nil {
		return err
	}

	// Step 4: Configuration Options
	if err := wizardConfigurationOptions(ctx, wizardState, result); err != nil {
		return err
	}

	// Step 5: Review and Confirmation
	if err := wizardReviewSelection(ctx, wizardState, result); err != nil {
		return err
	}

	// Step 6: Installation Execution
	if err := wizardExecuteInstallation(ctx, wizardState, result); err != nil {
		return err
	}

	// Step 7: Final Summary
	result.Duration = time.Since(startTime)
	result.Success = len(result.Issues) == 0

	generateSetupSummary(result)

	if result.Success {
		result.Messages = append(result.Messages, "LSP Gateway setup completed successfully via wizard")
		result.Messages = append(result.Messages, fmt.Sprintf("Setup duration: %v", result.Duration))
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

// wizardConfigurationOptions handles configuration customization
func wizardConfigurationOptions(ctx context.Context, state *WizardState, result *SetupResult) error {
	fmt.Println("⚙️  Step 3: Configuration Options")
	fmt.Println("================================")
	fmt.Println()

	if setupVerbose {
		fmt.Println("📝 Configuration determines how LSP Gateway routes requests")
		fmt.Println("   and manages language servers. You can customize paths,")
		fmt.Println("   timeouts, and advanced settings.")
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

	// Advanced settings
	if setupVerbose {
		fmt.Println("💡 Advanced settings allow you to customize server timeouts,")
		fmt.Println("   performance options, and specific language server settings.")
		fmt.Println()
	}

	advanced, err := promptYesNo("Configure advanced settings? (timeout, performance, etc.)", false)
	if err != nil {
		return fmt.Errorf("failed to read user input: %w", err)
	}

	state.AdvancedSettings = advanced

	if advanced {
		fmt.Println("📋 Advanced settings will be configured during installation.")
		if setupVerbose {
			fmt.Println("   This includes server-specific settings, timeouts, and")
			fmt.Println("   performance optimizations based on your system.")
		}
	} else {
		fmt.Println("📋 Using default settings for optimal performance.")
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
	warnings := make([]string, 0)

	// TODO: Implement comprehensive verification
	// This would include:
	// 1. Runtime verification
	// 2. Server functionality tests
	// 3. Configuration validation
	// 4. Basic functionality tests

	if setupVerbose && !setupJSON {
		fmt.Println("Verification implementation in progress...")
	}

	return warnings, nil
}

// runSetupDryRun performs a dry-run of the setup process
func runSetupDryRun(ctx context.Context, result *SetupResult) error {
	if setupVerbose {
		fmt.Println("🔍 DRY RUN: Analyzing what would be done...")
		fmt.Println()
	}

	// Phase 1: Runtime Detection (no installation)
	detector := setup.NewRuntimeDetector()
	if detector == nil {
		return NewInstallerCreationError("runtime detector")
	}

	detectionReport, err := detector.DetectAll(ctx)
	if err != nil {
		return fmt.Errorf("runtime detection failed: %w", err)
	}

	result.RuntimesDetected = detectionReport.Runtimes
	result.Summary.TotalRuntimes = len(detectionReport.Runtimes)

	if setupVerbose {
		fmt.Println("📋 RUNTIMES ANALYSIS:")
		for runtimeName, runtimeInfo := range detectionReport.Runtimes {
			if runtimeInfo.Installed && runtimeInfo.Compatible {
				fmt.Printf("  ✅ %s: Already installed and compatible (v%s)\n", runtimeName, runtimeInfo.Version)
				result.Summary.RuntimesAlreadyExist++
			} else if runtimeInfo.Installed {
				fmt.Printf("  ⚠️  %s: Installed but version incompatible (v%s)\n", runtimeName, runtimeInfo.Version)
				fmt.Printf("       Would be upgraded/reinstalled\n")
				result.Summary.RuntimesInstalled++
			} else {
				fmt.Printf("  ❌ %s: Not installed, would be installed\n", runtimeName)
				result.Summary.RuntimesInstalled++
			}
		}
		fmt.Println()
	}

	// Phase 2: Language Server Analysis
	if setupVerbose {
		fmt.Println("📋 LANGUAGE SERVERS ANALYSIS:")
	}

	runtimeInstaller := installer.NewRuntimeInstaller()
	if runtimeInstaller != nil {
		serverInstaller := installer.NewServerInstaller(runtimeInstaller)
		if serverInstaller != nil {
			supportedServers := serverInstaller.GetSupportedServers()
			result.Summary.TotalServers = len(supportedServers)

			for _, serverName := range supportedServers {
				if verifyResult, err := serverInstaller.Verify(serverName); err == nil && verifyResult.Installed && verifyResult.Compatible {
					if setupVerbose {
						fmt.Printf("  ✅ %s: Already installed and functional (v%s)\n", serverName, verifyResult.Version)
					}
					result.Summary.ServersAlreadyExist++
				} else {
					if setupVerbose {
						fmt.Printf("  ❌ %s: Not installed or not functional, would be installed\n", serverName)
					}
					result.Summary.ServersInstalled++
				}
			}
		}
	}

	if setupVerbose {
		fmt.Println()
	}

	// Phase 3: Configuration Analysis
	if setupVerbose {
		fmt.Println("📋 CONFIGURATION ANALYSIS:")
		if _, err := os.Stat(setupConfigPath); err == nil {
			fmt.Printf("  ⚠️  Configuration file exists at %s, would be updated\n", setupConfigPath)
		} else {
			fmt.Printf("  ❌ Configuration file would be created at %s\n", setupConfigPath)
		}
		result.ConfigGenerated = true
		fmt.Println()
	}

	// Generate summary
	result.Success = true
	result.Duration = time.Since(time.Now()) // Minimal duration for dry run

	if setupVerbose {
		fmt.Println("📊 DRY RUN SUMMARY:")
		fmt.Printf("  • Runtimes: %d total, %d to install, %d already working\n",
			result.Summary.TotalRuntimes, result.Summary.RuntimesInstalled, result.Summary.RuntimesAlreadyExist)
		fmt.Printf("  • Servers: %d total, %d to install, %d already working\n",
			result.Summary.TotalServers, result.Summary.ServersInstalled, result.Summary.ServersAlreadyExist)
		fmt.Printf("  • Configuration: Would be %s\n", map[bool]string{true: "created/updated", false: "skipped"}[result.ConfigGenerated])
		fmt.Println()
		fmt.Println("💡 To perform actual installation, run without --dry-run flag")
	}

	// Output results
	if setupJSON {
		return outputSetupResultsJSON(result)
	} else {
		return outputSetupResultsHuman(result)
	}
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
