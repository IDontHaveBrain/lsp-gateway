package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/types"
	versionpkg "lsp-gateway/src/internal/version"
)

// CLI Constants
const (
	DefaultServerPort = 8080
	CmdServer         = "server"
	CmdMCP            = "mcp"
	CmdStatus         = "status"
	CmdTest           = "test"
	CmdInstall        = "install"
	CmdVersion        = "version"
	CmdCache          = "cache"
	CmdCacheStatus    = "status"
	CmdCacheClear     = "clear"
	CmdCacheHealth    = "health"
	CmdCacheStats     = "stats"
	CmdCacheIndex     = "index"
	FlagConfig        = "config"
	FlagMode          = "mode"
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
	mcpMode     string
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
  • SCIP Cache system for improved response times and performance
  • HTTP JSON-RPC Gateway for IDE/editor integration
  • MCP Server for AI assistant integration (Claude, ChatGPT, etc.)
  • Automatic server detection and configuration
  • Simplified architecture focused on core functionality

🔧 AVAILABLE COMMANDS:

  Core Operations:
    lsp-gateway server                     # Start HTTP Gateway on port 8080
    lsp-gateway mcp                        # Start MCP Server for AI assistants
    lsp-gateway status                     # Show LSP server and cache status
    lsp-gateway test                       # Test LSP servers and cache performance
    lsp-gateway cache                      # Manage SCIP cache system

  Cache Management:
    lsp-gateway cache status               # Show cache configuration and status
    lsp-gateway cache clear                # Clear cache contents
    lsp-gateway cache health               # Cache health diagnostics
    lsp-gateway cache stats                # Performance statistics

🎯 INTEGRATION MODES:
  • HTTP Gateway: JSON-RPC endpoint at http://localhost:8080/jsonrpc
  • MCP Server: AI assistant integration via Model Context Protocol

💡 SUPPORTED LANGUAGES:
  • Go (gopls) - Definition lookup, references, symbols, hover
  • Python (pylsp) - Code analysis, completion, diagnostics  
  • TypeScript/JavaScript (typescript-language-server) - Full language support
  • Java (jdtls) - Enterprise-grade Java development

Use 'lsp-gateway <command> --help' for detailed command information.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Command definitions
var (
	serverCmd = &cobra.Command{
		Use:   CmdServer,
		Short: "Start the simple LSP Gateway server",
		Long: `Start the simplified LSP Gateway server with automatic configuration and SCIP cache.

The server provides HTTP JSON-RPC endpoints with intelligent caching for improved performance.`,
		RunE: runServerCmd,
	}

	mcpCmd = &cobra.Command{
		Use:   CmdMCP,
		Short: "Start MCP Server for AI assistants",
		Long: `Start the Model Context Protocol (MCP) server for AI assistant integration.

The MCP server provides a standardized interface for AI assistants like Claude to access
LSP functionality through a structured tool-based protocol.

Cache Integration:
The MCP server includes SCIP cache support that improves response times for:
- Definition lookups and symbol navigation
- Reference searches across codebases  
- Hover information and documentation
- Code completion suggestions

Modes:
  lsp       Direct LSP mapping with minimal processing (default)
  enhanced  AI-optimized tools with processed, structured responses

Usage Examples:
  lsp-gateway mcp                           # Start in default mode (lsp)
  lsp-gateway mcp --mode=enhanced           # Start in enhanced mode
  lsp-gateway mcp --config custom.yaml     # Start with custom configuration
  lsp-gateway mcp --mode=enhanced --config custom.yaml  # Combined usage

Configuration Priority (highest to lowest):
1. CLI flag (--mode)
2. Configuration file (mcp.mode)
3. Default (lsp)

Configuration:
The MCP server uses the same configuration file as the HTTP gateway, supporting
the same LSP servers and language detection.

AI Assistant Integration:
Configure your AI assistant to connect to this MCP server:
- Protocol: STDIO
- Command: lsp-gateway mcp
- Working Directory: Your project root

Supported Tools (behavior varies by mode):
- goto_definition: Navigate to symbol definitions
- find_references: Find all symbol references  
- get_hover_info: Get symbol documentation
- get_document_symbols: List document symbols
- search_workspace_symbols: Search workspace symbols
- get_completion: Get code completion suggestions`,
		RunE: runMCPCmd,
	}

	statusCmd = &cobra.Command{
		Use:   CmdStatus,
		Short: "Show LSP server status",
		Long:  `Display the current status of all configured LSP servers and SCIP cache system.`,
		RunE:  runStatusCmd,
	}

	testCmd = &cobra.Command{
		Use:   CmdTest,
		Short: "Test LSP server connections",
		Long:  `Test connections to configured LSP servers and validate cache performance.`,
		RunE:  runTestCmd,
	}

	versionCmd = &cobra.Command{
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

	installCmd = &cobra.Command{
		Use:   CmdInstall,
		Short: "Install language servers",
		Long: `Install language servers for LSP Gateway.

This command automates the installation of language servers for supported languages.
For Java, it downloads and installs a complete JDK along with Eclipse JDT Language Server.

Available commands:
  lsp-gateway install all           # Install all supported language servers
  lsp-gateway install go            # Install Go language server (gopls)
  lsp-gateway install python        # Install Python language server (pylsp)
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

	cacheCmd = &cobra.Command{
		Use:   CmdCache,
		Short: "Manage SCIP cache system",
		Long: `Manage the SCIP cache system used to improve LSP response times.

The SCIP cache stores LSP responses to reduce latency and improve performance
for repeated requests. The cache is automatically managed but can be monitored
and controlled through these commands.

Available commands:
  lsp-gateway cache status      # Show cache status and basic metrics
  lsp-gateway cache health      # Show detailed cache health diagnostics  
  lsp-gateway cache stats       # Show performance statistics
  lsp-gateway cache clear       # Clear all cached entries
  lsp-gateway cache index       # Proactively index files for cache

Examples:
  lsp-gateway cache status
  lsp-gateway cache health --config config.yaml
  lsp-gateway cache stats
  lsp-gateway cache clear --force
  lsp-gateway cache index --verbose`,
		RunE: runCacheCmd,
	}
)

// Install subcommands
var (
	installStatusCmd = &cobra.Command{
		Use:   "status",
		Short: "Show installation status",
		Long:  `Display installation status for all supported language servers.`,
		RunE:  runInstallStatusCmd,
	}

	installAllCmd = &cobra.Command{
		Use:   "all",
		Short: "Install all language servers",
		Long:  `Install all supported language servers automatically.`,
		RunE:  runInstallAllCmd,
	}

	installGoCmd = &cobra.Command{
		Use:   "go",
		Short: "Install Go language server",
		Long:  `Install Go language server (gopls) using go install.`,
		RunE:  runInstallGoCmd,
	}

	installPythonCmd = &cobra.Command{
		Use:   "python",
		Short: "Install Python language server",
		Long:  `Install Python language server (pylsp) using pip.`,
		RunE:  runInstallPythonCmd,
	}

	installTypeScriptCmd = &cobra.Command{
		Use:   "typescript",
		Short: "Install TypeScript language server",
		Long:  `Install TypeScript language server using npm.`,
		RunE:  runInstallTypeScriptCmd,
	}

	installJavaScriptCmd = &cobra.Command{
		Use:   "javascript",
		Short: "Install JavaScript language server",
		Long:  `Install JavaScript language server (same as TypeScript) using npm.`,
		RunE:  runInstallJavaScriptCmd,
	}

	installJavaCmd = &cobra.Command{
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

	installUpdateConfigCmd = &cobra.Command{
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
)

// Cache subcommands
var (
	cacheStatusCmd = &cobra.Command{
		Use:   CmdCacheStatus,
		Short: "Show cache status",
		Long: `Display current SCIP cache status and basic metrics.

Shows cache health status, entry count, hit/miss ratio, and storage usage.
This is a quick overview of cache performance without detailed diagnostics.

Examples:
  lsp-gateway cache status
  lsp-gateway cache status --config custom.yaml`,
		RunE: runCacheStatusCmd,
	}

	cacheClearCmd = &cobra.Command{
		Use:   CmdCacheClear,
		Short: "Clear cache entries",
		Long: `Clear all SCIP cache entries.

This removes all cached LSP responses, forcing fresh requests to LSP servers.
Use this when you suspect cache corruption or want to start fresh.

The operation requires confirmation unless --force is specified.

Examples:
  lsp-gateway cache clear           # Interactive confirmation
  lsp-gateway cache clear --force   # Skip confirmation`,
		RunE: runCacheClearCmd,
	}

	cacheHealthCmd = &cobra.Command{
		Use:   CmdCacheHealth,
		Short: "Show cache health diagnostics",
		Long: `Display detailed SCIP cache health diagnostics and metrics.

Provides comprehensive health information including error rates, performance
metrics, circuit breaker status, and detailed statistics for troubleshooting.

Examples:
  lsp-gateway cache health
  lsp-gateway cache health --config custom.yaml`,
		RunE: runCacheHealthCmd,
	}

	cacheStatsCmd = &cobra.Command{
		Use:   CmdCacheStats,
		Short: "Show cache performance statistics",
		Long: `Display detailed SCIP cache performance statistics.

Shows hit/miss ratios, response times, storage utilization, eviction counts,
and performance ratings to help evaluate cache effectiveness.

Examples:
  lsp-gateway cache stats
  lsp-gateway cache stats --config custom.yaml`,
		RunE: runCacheStatsCmd,
	}

	cacheIndexCmd = &cobra.Command{
		Use:   CmdCacheIndex,
		Short: "Index files proactively for cache",
		Long: `Proactively index files to populate the SCIP cache.

This command pre-processes files in the workspace to build cache entries
for symbols, definitions, references, and other LSP data. Pre-indexing
improves response times for subsequent LSP queries.

Behavior:
- Automatically detects languages in the current directory
- Processes files based on detected project structure
- Populates cache with symbol information from LSP servers
- Shows progress and statistics during indexing

The indexed data is stored persistently and loaded on server startup,
providing immediate sub-millisecond responses for cached queries.

Examples:
  lsp-gateway cache index                    # Index current directory
  lsp-gateway cache index --config config.yaml  # Index with custom config
  lsp-gateway cache index --verbose             # Show detailed progress`,
		RunE: runCacheIndexCmd,
	}
)

// getModeStrings returns a slice of valid mode strings for help text
func getModeStrings() []string {
	modes := types.ValidModes()
	strs := make([]string, len(modes))
	for i, mode := range modes {
		strs[i] = string(mode)
	}
	return strs
}

func init() {
	// Server command flags
	serverCmd.Flags().StringVarP(&configPath, FlagConfig, "c", "", "Configuration file path (optional, will use defaults if not provided)")
	serverCmd.Flags().IntVarP(&port, FlagPort, "p", DefaultServerPort, "Server port")

	// MCP command flags
	mcpCmd.Flags().StringVarP(&configPath, FlagConfig, "c", "", "Configuration file path (optional)")
	mcpCmd.Flags().StringVar(&mcpMode, FlagMode, "", fmt.Sprintf("MCP server mode (%s)", strings.Join(getModeStrings(), "|")))

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

	// Cache command flags
	cacheStatusCmd.Flags().StringVarP(&configPath, FlagConfig, "c", "", "Configuration file path (optional)")
	cacheClearCmd.Flags().StringVarP(&configPath, FlagConfig, "c", "", "Configuration file path (optional)")
	cacheClearCmd.Flags().BoolVarP(&force, FlagForce, "f", false, "Skip confirmation prompt")
	cacheHealthCmd.Flags().StringVarP(&configPath, FlagConfig, "c", "", "Configuration file path (optional)")
	cacheStatsCmd.Flags().StringVarP(&configPath, FlagConfig, "c", "", "Configuration file path (optional)")
	cacheIndexCmd.Flags().StringVarP(&configPath, FlagConfig, "c", "", "Configuration file path (optional)")
	cacheIndexCmd.Flags().BoolVarP(&verbose, FlagVerbose, "v", false, "Show detailed indexing progress")

	// Cache subcommands
	cacheCmd.AddCommand(cacheStatusCmd)
	cacheCmd.AddCommand(cacheClearCmd)
	cacheCmd.AddCommand(cacheHealthCmd)
	cacheCmd.AddCommand(cacheStatsCmd)
	cacheCmd.AddCommand(cacheIndexCmd)

	// Add commands to root
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(mcpCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(testCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(cacheCmd)
}

// Command runner functions - these delegate to the extracted modules

func runServerCmd(cmd *cobra.Command, args []string) error {
	addr := fmt.Sprintf(":%d", port)
	var configFile string
	if configPath != "" {
		configFile = configPath
	}
	return RunServer(addr, configFile)
}

func runMCPCmd(cmd *cobra.Command, args []string) error {
	var parsedMode types.MCPMode
	if mcpMode != "" {
		var err error
		parsedMode, err = types.ParseMCPMode(mcpMode)
		if err != nil {
			return fmt.Errorf("invalid mode: %w", err)
		}
	}
	return RunMCPServer(configPath, parsedMode)
}

func runStatusCmd(cmd *cobra.Command, args []string) error {
	return ShowStatus(configPath)
}

func runTestCmd(cmd *cobra.Command, args []string) error {
	return TestConnection(configPath)
}

func runVersionCmd(cmd *cobra.Command, args []string) error {
	if verbose {
		common.CLILogger.Info("%s", versionpkg.GetFullVersionInfo())
		return nil
	}
	common.CLILogger.Info("lsp-gateway %s", versionpkg.GetVersion())
	return nil
}

// Install command runners
func runInstallCmd(cmd *cobra.Command, args []string) error {
	return cmd.Help()
}

func runInstallStatusCmd(cmd *cobra.Command, args []string) error {
	return ShowInstallStatus()
}

func runInstallAllCmd(cmd *cobra.Command, args []string) error {
	return InstallAll(installPath, version, force, offline)
}

func runInstallGoCmd(cmd *cobra.Command, args []string) error {
	return InstallLanguage("go", installPath, version, force, offline)
}

func runInstallPythonCmd(cmd *cobra.Command, args []string) error {
	return InstallLanguage("python", installPath, version, force, offline)
}

func runInstallTypeScriptCmd(cmd *cobra.Command, args []string) error {
	return InstallLanguage("typescript", installPath, version, force, offline)
}

func runInstallJavaScriptCmd(cmd *cobra.Command, args []string) error {
	return InstallLanguage("javascript", installPath, version, force, offline)
}

func runInstallJavaCmd(cmd *cobra.Command, args []string) error {
	return InstallLanguage("java", installPath, version, force, offline)
}

func runInstallUpdateConfigCmd(cmd *cobra.Command, args []string) error {
	return UpdateConfigWithInstalled(configPath)
}

// Cache command runners
func runCacheCmd(cmd *cobra.Command, args []string) error {
	return cmd.Help()
}

func runCacheStatusCmd(cmd *cobra.Command, args []string) error {
	return ShowCacheStatus(configPath)
}

func runCacheClearCmd(cmd *cobra.Command, args []string) error {
	return ClearCache(configPath, force)
}

func runCacheHealthCmd(cmd *cobra.Command, args []string) error {
	return ShowCacheHealth(configPath)
}

func runCacheStatsCmd(cmd *cobra.Command, args []string) error {
	return ShowCacheStats(configPath)
}

func runCacheIndexCmd(cmd *cobra.Command, args []string) error {
	return IndexCacheProactively(configPath, verbose)
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}
