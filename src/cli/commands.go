package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"lsp-gateway/src/internal/common"
	versionpkg "lsp-gateway/src/internal/version"
)

// CLI Constants
const (
	DefaultServerPort = 8080
	CmdServer         = "server"
	CmdMCP            = "mcp"
	CmdContext        = "context"
	CmdStatus         = "status"
	CmdTest           = "test"
	CmdInstall        = "install"
	CmdVersion        = "version"
	CmdCache          = "cache"
	CmdCacheClear     = "clear"
	CmdCacheIndex     = "index"
	FlagConfig        = "config"
	FlagLSP           = "lsp"
	FlagPort          = "port"
	FlagForce         = "force"
	FlagOffline       = "offline"
	FlagVersion       = "version"
	FlagInstallPath   = "install-path"
	FlagVerbose       = "verbose"
	FlagOut           = "out"
)

// CLI Variables
var (
    configPath  string
    lspOnly     bool
    port        int
    force       bool
    offline     bool
    version     string
    installPath string
    verbose     bool
    outPath     string
    formatJSON  bool
    targetFiles []string
    includeRefs bool
    
)

// Root command
var rootCmd = &cobra.Command{
	Use:   "lsp-gateway",
	Short: "LSP Gateway - A simplified JSON-RPC gateway for Language Server Protocol servers",
	Long: `LSP Gateway provides a unified JSON-RPC interface for multiple Language Server Protocol (LSP) servers
with Model Context Protocol (MCP) server functionality for AI assistant integration.

QUICK START:
  lsp-gateway server                       # Start HTTP Gateway (port 8080)
  lsp-gateway mcp                          # Start MCP Server for AI assistants

CORE FEATURES:
  - Multi-language LSP server management (Go, Python, TypeScript, Java, Rust)
  - SCIP Cache system for improved response times and performance
  - HTTP JSON-RPC Gateway for IDE/editor integration
  - MCP Server for AI assistant integration (Claude, ChatGPT, etc.)
  - Automatic server detection and configuration
  - Simplified architecture focused on core functionality

AVAILABLE COMMANDS:

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

INTEGRATION MODES:
  - HTTP Gateway: JSON-RPC endpoint at http://localhost:8080/jsonrpc
  - MCP Server: AI assistant integration via Model Context Protocol

SUPPORTED LANGUAGES:
  - Go (gopls) - Definition lookup, references, symbols, hover
  - Python (jedi-language-server) - Code analysis, completion, diagnostics  
  - TypeScript/JavaScript (typescript-language-server) - Full language support
  - Java (jdtls) - Enterprise-grade Java development
  - Rust (rust-analyzer) - Modern Rust language support
  - C# (OmniSharp) - C# language support

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

The server provides HTTP JSON-RPC endpoints with intelligent caching for improved performance.

By default, includes all 6 basic LSP functions plus additional enhanced features.
Use --lsp flag to restrict to only the 6 basic LSP functions.`,
		RunE: runServerCmd,
	}

	mcpCmd = &cobra.Command{
		Use:   CmdMCP,
		Short: "Start MCP Server for AI assistants",
		Long: `Start the Model Context Protocol (MCP) server for AI assistant integration.

The MCP server provides a standardized interface for AI assistants like Claude to access
enhanced development tools through a structured tool-based protocol.

This mode provides only enhanced tools and does NOT include the basic 6 LSP functions.
Use 'lsp-gateway server' for basic LSP functionality.

Cache Integration:
The MCP server includes SCIP cache support that improves response times for
enhanced development operations.

Usage Examples:
  lsp-gateway mcp                           # Start MCP server with enhanced tools
  lsp-gateway mcp --config custom.yaml     # Start with custom configuration

Configuration:
The MCP server uses the same configuration file as the HTTP gateway, supporting
the same LSP servers and language detection.

AI Assistant Integration:
Configure your AI assistant to connect to this MCP server:
- Protocol: STDIO
- Command: lsp-gateway mcp
- Working Directory: Your project root`,
		RunE: runMCPCmd,
	}

	contextCmd = &cobra.Command{
		Use:   CmdContext,
		Short: "Context-related utilities",
		Long:  `Tools for generating and exporting project context artifacts from indexed data.`,
		RunE:  runContextCmd,
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
  lsp-gateway install python        # Install Python language server (jedi-language-server)
  lsp-gateway install typescript    # Install TypeScript language server
  lsp-gateway install javascript    # Install JavaScript language server (same as TypeScript)
  lsp-gateway install java          # Install Java JDK + Eclipse JDT Language Server
  lsp-gateway install rust          # Install Rust language server (rust-analyzer)
  lsp-gateway install csharp        # Install C# language server (OmniSharp)
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
  lsp-gateway cache info        # Show cache information and statistics
  lsp-gateway cache clear       # Clear all cached entries
  lsp-gateway cache index       # Proactively index files for cache

Examples:
  lsp-gateway cache info
  lsp-gateway cache clear
  lsp-gateway cache index`,
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
		Long:  `Install Python language server (jedi-language-server) using pip.`,
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

	installRustCmd = &cobra.Command{
		Use:   "rust",
		Short: "Install Rust language server",
		Long:  `Install Rust language server (rust-analyzer) using rustup.`,
		RunE:  runInstallRustCmd,
	}

	installCSharpCmd = &cobra.Command{
		Use:   "csharp",
		Short: "Install C# language server",
		Long:  `Install C# language server (OmniSharp).`,
		RunE:  runInstallCSharpCmd,
	}
)

// Cache subcommands
var (
	cacheInfoCmd = &cobra.Command{
		Use:   "info",
		Short: "Show cache information and statistics",
		Long: `Display cache information including indexed documents and symbols.

Shows cache statistics, indexed file counts, and language breakdown.

Examples:
  lsp-gateway cache info
  lsp-gateway cache info --config custom.yaml`,
		RunE: runCacheInfoCmd,
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

// Context subcommands
var (
	contextMapCmd = &cobra.Command{
		Use:   "map <file>",
		Short: "Print code of referenced files",
		Long:  `Given a source file path, find files it references and print their code.`,
		Args:  cobra.ExactArgs(1),
		RunE:  runContextMapCmd,
	}

	contextSymbolsCmd = &cobra.Command{
		Use:   "symbols [files...]",
		Short: "Extract symbol defs/refs via LSP index",
		Long: `Extract and display symbol definitions and references from specified files.

Uses the SCIP cache and LSP to enumerate symbols in each file and, when requested,
lists reference locations across the workspace.`,
		RunE:  runContextSymbolsCmd,
	}

)

func init() {
	// Server command flags
	serverCmd.Flags().StringVarP(&configPath, FlagConfig, "c", "", "Configuration file path (optional, will use defaults if not provided)")
	serverCmd.Flags().IntVarP(&port, FlagPort, "p", DefaultServerPort, "Server port")
	serverCmd.Flags().BoolVar(&lspOnly, FlagLSP, false, "Provide only the 6 basic LSP functions")

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
	installCmd.AddCommand(installRustCmd)
	installCmd.AddCommand(installCSharpCmd)
	installCmd.AddCommand(installUpdateConfigCmd)

	// Cache command flags
	cacheInfoCmd.Flags().StringVarP(&configPath, FlagConfig, "c", "", "Configuration file path (optional)")
	cacheClearCmd.Flags().StringVarP(&configPath, FlagConfig, "c", "", "Configuration file path (optional)")
	cacheIndexCmd.Flags().StringVarP(&configPath, FlagConfig, "c", "", "Configuration file path (optional)")
	cacheIndexCmd.Flags().BoolVarP(&verbose, FlagVerbose, "v", false, "Show detailed progress")

	// Cache subcommands
	cacheCmd.AddCommand(cacheInfoCmd)
	cacheCmd.AddCommand(cacheClearCmd)
	cacheCmd.AddCommand(cacheIndexCmd)

    // Context command flags
    contextMapCmd.Flags().StringVarP(&configPath, FlagConfig, "c", "", "Configuration file path (optional)")

    contextSymbolsCmd.Flags().StringVarP(&configPath, FlagConfig, "c", "", "Configuration file path (optional)")
    contextSymbolsCmd.Flags().BoolVar(&formatJSON, "json", false, "Output in JSON format")
    contextSymbolsCmd.Flags().BoolVar(&includeRefs, "include-refs", true, "Include reference locations for each symbol")
    contextSymbolsCmd.Flags().StringSliceVar(&targetFiles, "files", []string{}, "Specific files to analyze")

    // Context subcommands
    contextCmd.AddCommand(contextMapCmd)
    contextCmd.AddCommand(contextSymbolsCmd)

	// Add commands to root
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(mcpCmd)
	rootCmd.AddCommand(contextCmd)
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
	return RunServer(addr, configFile, lspOnly)
}

func runMCPCmd(cmd *cobra.Command, args []string) error {
	return RunMCPServer(configPath)
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
	return InstallLanguage("go", installPath, version, force, offline, "")
}

func runInstallPythonCmd(cmd *cobra.Command, args []string) error {
	return InstallLanguage("python", installPath, version, force, offline, "")
}

func runInstallTypeScriptCmd(cmd *cobra.Command, args []string) error {
	return InstallLanguage("typescript", installPath, version, force, offline, "")
}

func runInstallJavaScriptCmd(cmd *cobra.Command, args []string) error {
	return InstallLanguage("javascript", installPath, version, force, offline, "")
}

func runInstallJavaCmd(cmd *cobra.Command, args []string) error {
	return InstallLanguage("java", installPath, version, force, offline, "")
}

func runInstallRustCmd(cmd *cobra.Command, args []string) error {
	return InstallLanguage("rust", installPath, version, force, offline, "")
}

func runInstallCSharpCmd(cmd *cobra.Command, args []string) error {
	return InstallLanguage("csharp", installPath, version, force, offline, "")
}

func runInstallUpdateConfigCmd(cmd *cobra.Command, args []string) error {
	return UpdateConfigWithInstalled(configPath)
}

// Cache command runners
func runCacheCmd(cmd *cobra.Command, args []string) error {
	return cmd.Help()
}

func runCacheInfoCmd(cmd *cobra.Command, args []string) error {
	return ShowCacheInfo(configPath)
}

func runCacheClearCmd(cmd *cobra.Command, args []string) error {
	return ClearCache(configPath)
}

func runCacheIndexCmd(cmd *cobra.Command, args []string) error {
	return IndexCache(configPath)
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}
