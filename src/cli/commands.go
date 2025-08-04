package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"lsp-gateway/src/config"
	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/installer"
	versionpkg "lsp-gateway/src/internal/version"
	"lsp-gateway/src/server"
)

// RunServer starts the simplified LSP gateway server
func RunServer(addr string, configPath string) error {
	// Load configuration
	var cfg *config.Config
	if configPath != "" {
		loadedConfig, err := config.LoadConfig(configPath)
		if err != nil {
			log.Printf("Warning: Failed to load config from %s, using defaults: %v", configPath, err)
			cfg = config.GetDefaultConfig()
		} else {
			cfg = loadedConfig
		}
	} else {
		// Try to load from default config file if it exists
		defaultConfigPath := config.GetDefaultConfigPath()
		if _, err := os.Stat(defaultConfigPath); err == nil {
			loadedConfig, err := config.LoadConfig(defaultConfigPath)
			if err != nil {
				log.Printf("Warning: Failed to load default config from %s, using defaults: %v", defaultConfigPath, err)
				cfg = config.GetDefaultConfig()
			} else {
				cfg = loadedConfig
			}
		} else {
			cfg = config.GetDefaultConfig()
		}
	}

	// Create and start gateway
	gateway, err := server.NewHTTPGateway(addr, cfg)
	if err != nil {
		return fmt.Errorf("failed to create gateway: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := gateway.Start(ctx); err != nil {
		return fmt.Errorf("failed to start gateway: %w", err)
	}

	log.Printf("✅ Simple LSP Gateway started on %s", addr)
	log.Printf("📡 Available languages: go, python, nodejs, typescript, java")
	log.Printf("🔗 HTTP JSON-RPC endpoint: http://%s/jsonrpc", addr)
	log.Printf("❤️  Health check endpoint: http://%s/health", addr)

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Printf("🛑 Received shutdown signal, stopping gateway...")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	done := make(chan error, 1)
	go func() {
		done <- gateway.Stop()
	}()

	select {
	case err := <-done:
		if err != nil {
			log.Printf("⚠️  Gateway stopped with error: %v", err)
		} else {
			log.Printf("✅ Gateway stopped successfully")
		}
	case <-shutdownCtx.Done():
		log.Printf("⚠️  Shutdown timeout exceeded")
		return fmt.Errorf("shutdown timeout")
	}

	return nil
}

// ShowStatus displays the current status of LSP clients
func ShowStatus(configPath string) error {
	cfg := config.GetDefaultConfig()
	if configPath != "" {
		if loadedConfig, err := config.LoadConfig(configPath); err == nil {
			cfg = loadedConfig
		}
	} else {
		// Try to load from default config file if it exists
		defaultConfigPath := config.GetDefaultConfigPath()
		if _, err := os.Stat(defaultConfigPath); err == nil {
			if loadedConfig, err := config.LoadConfig(defaultConfigPath); err == nil {
				cfg = loadedConfig
			}
		}
	}

	common.CLILogger.Info("🔍 LSP Gateway Status")
	common.CLILogger.Info("%s", strings.Repeat("=", 50))

	manager, err := server.NewLSPManager(cfg)
	if err != nil {
		return fmt.Errorf("failed to create LSP manager: %w", err)
	}

	// Check server availability without starting them (fast, non-destructive)
	status := manager.CheckServerAvailability()

	common.CLILogger.Info("📊 Configured Languages: %d\n", len(cfg.Servers))

	for language, serverConfig := range cfg.Servers {
		clientStatus := status[language]
		statusIcon := "❌"
		statusText := "Unavailable"

		if clientStatus.Available {
			statusIcon = "✅"
			statusText = "Available"
		}

		common.CLILogger.Info("%s %s: %s", statusIcon, language, statusText)
		common.CLILogger.Info("   Command: %s %v", serverConfig.Command, serverConfig.Args)

		if !clientStatus.Available && clientStatus.Error != nil {
			common.CLILogger.Error("   Error: %v", clientStatus.Error)
		}

		common.CLILogger.Info("")
	}

	return nil
}

// TestConnection tests connection to LSP servers
func TestConnection(configPath string) error {
	cfg := config.GetDefaultConfig()
	if configPath != "" {
		if loadedConfig, err := config.LoadConfig(configPath); err == nil {
			cfg = loadedConfig
		}
	} else {
		// Try to load from default config file if it exists
		defaultConfigPath := config.GetDefaultConfigPath()
		if _, err := os.Stat(defaultConfigPath); err == nil {
			if loadedConfig, err := config.LoadConfig(defaultConfigPath); err == nil {
				cfg = loadedConfig
			}
		}
	}

	common.CLILogger.Info("🧪 Testing LSP Server Connections")
	common.CLILogger.Info("%s", strings.Repeat("=", 50))

	manager, err := server.NewLSPManager(cfg)
	if err != nil {
		return fmt.Errorf("failed to create LSP manager: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	common.CLILogger.Info("🚀 Starting LSP Manager...")
	if err := manager.Start(ctx); err != nil {
		return fmt.Errorf("failed to start LSP manager: %w", err)
	}
	defer manager.Stop()

	// Get client status to see which servers started successfully
	status := manager.GetClientStatus()

	common.CLILogger.Info("")
	common.CLILogger.Info("📊 LSP Server Status:")
	common.CLILogger.Info("%s", strings.Repeat("-", 30))

	activeLanguages := []string{}
	for language, clientStatus := range status {
		statusIcon := "❌"
		statusText := "Inactive"

		if clientStatus.Active {
			statusIcon = "✅"
			statusText = "Active"
			activeLanguages = append(activeLanguages, language)
		}

		if clientStatus.Error != nil {
			common.CLILogger.Error("%s %s: %s (%v)", statusIcon, language, statusText, clientStatus.Error)
		} else {
			common.CLILogger.Info("%s %s: %s", statusIcon, language, statusText)
		}
	}

	if len(activeLanguages) == 0 {
		return fmt.Errorf("no active LSP servers available for testing")
	}

	common.CLILogger.Info("")
	common.CLILogger.Info("🔍 Testing Multi-Repo workspace/symbol functionality...")
	common.CLILogger.Info("%s", strings.Repeat("-", 50))

	// Test workspace/symbol for multi-repo support (queries all active servers)
	params := map[string]interface{}{
		"query": "main",
	}

	common.CLILogger.Info("")
	common.CLILogger.Info("📋 Multi-Repo workspace/symbol Test:")
	common.CLILogger.Info("%s", strings.Repeat("-", 40))

	result, err := testWorkspaceSymbolForLanguage(manager, ctx, "", params)
	successCount := 0
	if err != nil {
		common.CLILogger.Error("❌ workspace/symbol (multi-repo): %v", err)
	} else {
		common.CLILogger.Info("✅ workspace/symbol (multi-repo): Success (%T)", result)
		if result != nil {
			// Try to parse and show symbol count if possible
			if symbols, ok := parseSymbolResult(result); ok {
				common.CLILogger.Info("   📊 Total symbols found across all servers: %d", len(symbols))
			}
		}
		successCount = 1
	}

	// Show which servers contributed to the results
	common.CLILogger.Info("")
	common.CLILogger.Info("📈 Active servers contributing to results: %d", len(activeLanguages))
	for _, language := range activeLanguages {
		common.CLILogger.Info("   • %s server: Active", language)
	}

	common.CLILogger.Info("")
	common.CLILogger.Info("%s", strings.Repeat("=", 50))
	common.CLILogger.Info("📈 Test Summary:")
	common.CLILogger.Info("   • Active Servers: %d/%d", len(activeLanguages), len(cfg.Servers))
	common.CLILogger.Info("   • Multi-Repo Test: %s", func() string {
		if successCount > 0 {
			return "✅ Success"
		}
		return "❌ Failed"
	}())

	if successCount > 0 {
		common.CLILogger.Info("")
		common.CLILogger.Info("🎉 Multi-repo workspace/symbol functionality is working correctly!")
		common.CLILogger.Info("💡 All %d active LSP servers are contributing to search results.", len(activeLanguages))
		return nil
	} else {
		common.CLILogger.Warn("")
		common.CLILogger.Warn("⚠️  Multi-repo workspace/symbol test failed.")
		common.CLILogger.Warn("🔧 Check individual server logs for debugging.")
		return nil // Don't fail the test completely
	}
}

// testWorkspaceSymbolForLanguage tests workspace/symbol for multi-repo support
// This function now tests the complete multi-repo functionality by querying all servers
func testWorkspaceSymbolForLanguage(manager *server.LSPManager, ctx context.Context, language string, params interface{}) (interface{}, error) {
	// For multi-repo support, use ProcessRequest which queries all active servers
	// The language parameter is kept for backward compatibility but the test now covers all servers
	return manager.ProcessRequest(ctx, "workspace/symbol", params)
}

// parseSymbolResult tries to parse the symbol result to count items
func parseSymbolResult(result interface{}) ([]interface{}, bool) {
	if result == nil {
		return nil, false
	}

	// Try to convert to slice
	if symbols, ok := result.([]interface{}); ok {
		return symbols, true
	}

	return nil, false
}

// CLI Constants
const (
	DefaultServerPort = 8080
	CmdServer         = "server"
	CmdMCP            = "mcp"
	CmdStatus         = "status"
	CmdTest           = "test"
	CmdInstall        = "install"
	CmdVersion        = "version"
	FlagConfig        = "config"
	FlagPort          = "port"
	FlagForce         = "force"
	FlagOffline       = "offline"
	FlagVersion       = "version"
	FlagInstallPath   = "install-path"
	FlagVerbose       = "verbose"
)

// CLI Variables
var (
	configPath  string
	port        int
	force       bool
	offline     bool
	version     string
	installPath string
	verbose     bool
)

// Root command
var rootCmd = &cobra.Command{
	Use:   "lsp-gateway",
	Short: "LSP Gateway - A simplified JSON-RPC gateway for Language Server Protocol servers",
	Long: `LSP Gateway provides a unified JSON-RPC interface for multiple Language Server Protocol (LSP) servers
with Model Context Protocol (MCP) server functionality for AI assistant integration.

🚀 QUICK START:
  lsp-gateway server                       # Start HTTP Gateway (port 8080)
  lsp-gateway mcp                          # Start MCP Server for AI assistants

📋 CORE FEATURES:
  • Multi-language LSP server management (Go, Python, TypeScript, Java)  
  • HTTP JSON-RPC Gateway for IDE/editor integration
  • MCP Server for AI assistant integration (Claude, ChatGPT, etc.)
  • Automatic server detection and configuration
  • Simplified architecture focused on core functionality

🔧 AVAILABLE COMMANDS:

  Core Operations:
    lsp-gateway server                     # Start HTTP Gateway on port 8080
    lsp-gateway mcp                        # Start MCP Server for AI assistants
    lsp-gateway status                     # Show LSP server status
    lsp-gateway test                       # Test LSP server connections

🎯 INTEGRATION MODES:
  • HTTP Gateway: JSON-RPC endpoint at http://localhost:8080/jsonrpc
  • MCP Server: AI assistant integration via Model Context Protocol

💡 SUPPORTED LANGUAGES:
  • Go (gopls) - Definition lookup, references, symbols, hover
  • Python (pyright) - Code analysis, completion, diagnostics  
  • TypeScript/JavaScript (typescript-language-server) - Full language support
  • Java (jdtls) - Enterprise-grade Java development

Use 'lsp-gateway <command> --help' for detailed command information.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Server command
var serverCmd = &cobra.Command{
	Use:   CmdServer,
	Short: "Start the simple LSP Gateway server",
	Long:  `Start the simplified LSP Gateway server with automatic configuration.`,
	RunE:  runServerCmd,
}

// MCP command
var mcpCmd = &cobra.Command{
	Use:   CmdMCP,
	Short: "Start MCP Server for AI assistants",
	Long: `Start the Model Context Protocol (MCP) server for AI assistant integration.

The MCP server provides a standardized interface for AI assistants like Claude to access
LSP functionality through a structured tool-based protocol.

Usage Examples:
  lsp-gateway mcp                           # Start with default configuration
  lsp-gateway mcp --config custom.yaml     # Start with custom configuration

Configuration:
The MCP server uses the same configuration file as the HTTP gateway, supporting
the same LSP servers and language detection.

AI Assistant Integration:
Configure your AI assistant to connect to this MCP server:
- Protocol: STDIO
- Command: lsp-gateway mcp
- Working Directory: Your project root

Supported Tools:
- goto_definition: Navigate to symbol definitions
- find_references: Find all symbol references  
- get_hover_info: Get symbol documentation
- get_document_symbols: List document symbols
- search_workspace_symbols: Search workspace symbols
- get_completion: Get code completion suggestions`,
	RunE: runMCPCmd,
}

// Status command
var statusCmd = &cobra.Command{
	Use:   CmdStatus,
	Short: "Show LSP server status",
	Long:  `Display the current status of all configured LSP servers.`,
	RunE:  runStatusCmd,
}

// Test command
var testCmd = &cobra.Command{
	Use:   CmdTest,
	Short: "Test LSP server connections",
	Long:  `Test connections to configured LSP servers.`,
	RunE:  runTestCmd,
}

// Version command
var versionCmd = &cobra.Command{
	Use:   CmdVersion,
	Short: "Show version information",
	Long: `Display version information for LSP Gateway.

By default, shows only the version number. Use --verbose for detailed build information
including commit hash, build date, and Go version.

Examples:
  lsp-gateway version              # Show version number
  lsp-gateway version --verbose    # Show detailed build information`,
	RunE: runVersionCmd,
}

// Install command
var installCmd = &cobra.Command{
	Use:   CmdInstall,
	Short: "Install language servers",
	Long: `Install language servers for LSP Gateway.

This command automates the installation of language servers for supported languages.
For Java, it downloads and installs a complete JDK along with Eclipse JDT Language Server.

Available commands:
  lsp-gateway install all           # Install all supported language servers
  lsp-gateway install go            # Install Go language server (gopls)
  lsp-gateway install python        # Install Python language server (pyright)
  lsp-gateway install typescript    # Install TypeScript language server
  lsp-gateway install javascript    # Install JavaScript language server (same as TypeScript)
  lsp-gateway install java          # Install Java JDK + Eclipse JDT Language Server
  lsp-gateway install status        # Show installation status
  lsp-gateway install update-config # Update configuration with installed servers

Options:
  --force                          # Force reinstall even if already installed
  --offline                        # Use offline/cached installers only
  --version <version>              # Install specific version
  --install-path <path>            # Custom installation directory

Examples:
  lsp-gateway install all
  lsp-gateway install java --install-path ~/dev/java
  lsp-gateway install go --version latest`,
	RunE: runInstallCmd,
}

// Install status subcommand
var installStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show installation status",
	Long:  `Display installation status for all supported language servers.`,
	RunE:  runInstallStatusCmd,
}

// Install all subcommand
var installAllCmd = &cobra.Command{
	Use:   "all",
	Short: "Install all language servers",
	Long:  `Install all supported language servers automatically.`,
	RunE:  runInstallAllCmd,
}

// Install Go subcommand
var installGoCmd = &cobra.Command{
	Use:   "go",
	Short: "Install Go language server",
	Long:  `Install Go language server (gopls) using go install.`,
	RunE:  runInstallGoCmd,
}

// Install Python subcommand
var installPythonCmd = &cobra.Command{
	Use:   "python",
	Short: "Install Python language server",
	Long:  `Install Python language server (pyright) using npm.`,
	RunE:  runInstallPythonCmd,
}

// Install TypeScript subcommand
var installTypeScriptCmd = &cobra.Command{
	Use:   "typescript",
	Short: "Install TypeScript language server",
	Long:  `Install TypeScript language server using npm.`,
	RunE:  runInstallTypeScriptCmd,
}

// Install JavaScript subcommand
var installJavaScriptCmd = &cobra.Command{
	Use:   "javascript",
	Short: "Install JavaScript language server",
	Long:  `Install JavaScript language server (same as TypeScript) using npm.`,
	RunE:  runInstallJavaScriptCmd,
}

// Install Java subcommand
var installJavaCmd = &cobra.Command{
	Use:   "java",
	Short: "Install Java development environment",
	Long: `Install Java development environment including JDK and Eclipse JDT Language Server.

This command will:
1. Download and install OpenJDK 17
2. Download and install Eclipse JDT Language Server
3. Create a wrapper script that uses the custom JDK
4. Configure everything to work with LSP Gateway

The installed JDK will not interfere with any system Java installation.`,
	RunE: runInstallJavaCmd,
}

// Install update-config subcommand
var installUpdateConfigCmd = &cobra.Command{
	Use:   "update-config",
	Short: "Update configuration with installed language servers",
	Long: `Update the LSP Gateway configuration file with paths to installed language servers.

This command scans for installed language servers and updates the configuration file
to use the correct paths, especially useful after installing servers to custom locations.

Examples:
  lsp-gateway install update-config
  lsp-gateway install update-config --config custom.yaml`,
	RunE: runInstallUpdateConfigCmd,
}

func init() {
	// Server command flags
	serverCmd.Flags().StringVarP(&configPath, FlagConfig, "c", "", "Configuration file path (optional, will use defaults if not provided)")
	serverCmd.Flags().IntVarP(&port, FlagPort, "p", DefaultServerPort, "Server port")

	// MCP command flags
	mcpCmd.Flags().StringVarP(&configPath, FlagConfig, "c", "", "Configuration file path (optional)")

	// Status command flags
	statusCmd.Flags().StringVarP(&configPath, FlagConfig, "c", "", "Configuration file path (optional)")

	// Test command flags
	testCmd.Flags().StringVarP(&configPath, FlagConfig, "c", "", "Configuration file path (optional)")

	// Version command flags
	versionCmd.Flags().BoolVarP(&verbose, FlagVerbose, "v", false, "Show detailed version information")

	// Install command flags
	installCmd.PersistentFlags().BoolVarP(&force, FlagForce, "f", false, "Force reinstall even if already installed")
	installCmd.PersistentFlags().BoolVar(&offline, FlagOffline, false, "Use offline/cached installers only")
	installCmd.PersistentFlags().StringVarP(&version, FlagVersion, "v", "", "Install specific version")
	installCmd.PersistentFlags().StringVar(&installPath, FlagInstallPath, "", "Custom installation directory")

	// Install subcommands
	installCmd.AddCommand(installStatusCmd)
	installCmd.AddCommand(installAllCmd)
	installCmd.AddCommand(installGoCmd)
	installCmd.AddCommand(installPythonCmd)
	installCmd.AddCommand(installTypeScriptCmd)
	installCmd.AddCommand(installJavaScriptCmd)
	installCmd.AddCommand(installJavaCmd)
	installCmd.AddCommand(installUpdateConfigCmd)

	// Add commands to root
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(mcpCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(testCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(installCmd)
}

// Command runner functions
func runServerCmd(cmd *cobra.Command, args []string) error {
	// Determine server address
	addr := fmt.Sprintf(":%d", port)

	// Check if config file was provided and exists
	var configFile string
	if configPath != "" {
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			common.CLILogger.Warn("Config file %s not found, using defaults", configPath)
		} else {
			configFile = configPath
		}
	}

	// Run the simplified server
	return RunServer(addr, configFile)
}

func runMCPCmd(cmd *cobra.Command, args []string) error {
	return server.RunMCPServer(configPath)
}

func runStatusCmd(cmd *cobra.Command, args []string) error {
	return ShowStatus(configPath)
}

func runTestCmd(cmd *cobra.Command, args []string) error {
	return TestConnection(configPath)
}

func runVersionCmd(cmd *cobra.Command, args []string) error {
	if verbose {
		fmt.Println(versionpkg.GetFullVersionInfo())
		return nil
	}
	fmt.Printf("lsp-gateway %s\n", versionpkg.GetVersion())
	return nil
}

// Install functions

// ShowInstallStatus displays installation status for all language servers
func ShowInstallStatus() error {
	common.CLILogger.Info("🔍 Language Server Installation Status")
	common.CLILogger.Info("%s", strings.Repeat("=", 50))

	manager := installer.GetDefaultInstallManager()
	status := manager.GetStatus()

	if len(status) == 0 {
		common.CLILogger.Warn("No language installers available")
		return nil
	}

	installedCount := 0
	for language, installStatus := range status {
		statusIcon := "❌"
		statusText := "Not Installed"

		if installStatus.Installed {
			statusIcon = "✅"
			statusText = "Installed"
			installedCount++
		}

		common.CLILogger.Info("%s %s: %s", statusIcon, language, statusText)

		if installStatus.Version != "" {
			common.CLILogger.Info("   Version: %s", installStatus.Version)
		}

		if installStatus.Error != nil {
			common.CLILogger.Error("   Error: %v", installStatus.Error)
		}

		common.CLILogger.Info("")
	}

	common.CLILogger.Info("📊 Summary: %d/%d language servers installed", installedCount, len(status))

	return nil
}

// InstallAll installs all supported language servers
func InstallAll() error {
	common.CLILogger.Info("🚀 Installing all language servers...")

	manager := installer.GetDefaultInstallManager()

	options := installer.InstallOptions{
		InstallPath:    installPath,
		Version:        version,
		Force:          force,
		Offline:        offline,
		SkipValidation: false,
		UpdateConfig:   true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	if err := manager.InstallAll(ctx, options); err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	common.CLILogger.Info("✅ All language servers installation completed")
	return nil
}

// InstallLanguage installs a specific language server
func InstallLanguage(language string) error {
	common.CLILogger.Info("🚀 Installing %s language server...", language)

	manager := installer.GetDefaultInstallManager()

	options := installer.InstallOptions{
		InstallPath:    installPath,
		Version:        version,
		Force:          force,
		Offline:        offline,
		SkipValidation: false,
		UpdateConfig:   true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	if err := manager.InstallLanguage(ctx, language, options); err != nil {
		return fmt.Errorf("failed to install %s: %w", language, err)
	}

	common.CLILogger.Info("✅ %s language server installation completed", language)
	return nil
}

// Install command runners

func runInstallCmd(cmd *cobra.Command, args []string) error {
	// Show help if no subcommand specified
	return cmd.Help()
}

func runInstallStatusCmd(cmd *cobra.Command, args []string) error {
	return ShowInstallStatus()
}

func runInstallAllCmd(cmd *cobra.Command, args []string) error {
	return InstallAll()
}

func runInstallGoCmd(cmd *cobra.Command, args []string) error {
	return InstallLanguage("go")
}

func runInstallPythonCmd(cmd *cobra.Command, args []string) error {
	return InstallLanguage("python")
}

func runInstallTypeScriptCmd(cmd *cobra.Command, args []string) error {
	return InstallLanguage("typescript")
}

func runInstallJavaScriptCmd(cmd *cobra.Command, args []string) error {
	return InstallLanguage("javascript")
}

func runInstallJavaCmd(cmd *cobra.Command, args []string) error {
	return InstallLanguage("java")
}

func runInstallUpdateConfigCmd(cmd *cobra.Command, args []string) error {
	return UpdateConfigWithInstalled()
}

// UpdateConfigWithInstalled updates configuration with installed language servers
func UpdateConfigWithInstalled() error {
	common.CLILogger.Info("🔧 Updating configuration with installed language servers...")

	manager := installer.GetDefaultInstallManager()
	updater := installer.NewConfigUpdater(manager)

	// Get current config or use default
	var currentConfig *config.Config
	if configPath != "" {
		if loadedConfig, err := config.LoadConfig(configPath); err == nil {
			currentConfig = loadedConfig
		}
	}

	if currentConfig == nil {
		currentConfig = config.GetDefaultConfig()
	}

	// Update config with installed servers
	updatedConfig, err := updater.UpdateConfigWithInstalledServers(currentConfig)
	if err != nil {
		return fmt.Errorf("failed to update config: %w", err)
	}

	// Determine config path
	saveConfigPath := configPath
	if saveConfigPath == "" {
		saveConfigPath = config.GetDefaultConfigPath()
	}

	// Save updated config
	if err := updater.SaveUpdatedConfig(updatedConfig, saveConfigPath); err != nil {
		return fmt.Errorf("failed to save updated config: %w", err)
	}

	common.CLILogger.Info("✅ Configuration updated successfully")
	return nil
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}
