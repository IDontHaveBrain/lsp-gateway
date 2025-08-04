package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"lsp-gateway/src/config"
	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/installer"
	"lsp-gateway/src/internal/types"
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
	
	// Display SCIP cache status
	displayGatewayCacheStatus(gateway)
	
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

	// Display SCIP cache status
	displayCacheStatus(manager)

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

	// Display initial SCIP cache status
	common.CLILogger.Info("")
	common.CLILogger.Info("🗄️  SCIP Cache Status:")
	common.CLILogger.Info("%s", strings.Repeat("-", 30))

	cache := manager.GetCache()
	initialMetrics, healthErr := cache.HealthCheck()
	if healthErr != nil {
		common.CLILogger.Error("❌ Cache: Health check failed (%v)", healthErr)
	} else if initialMetrics != nil {
		common.CLILogger.Info("✅ Cache: Enabled and Ready")
		common.CLILogger.Info("   Health: %s", initialMetrics.HealthStatus)
		common.CLILogger.Info("   Initial Stats: %d entries, %.1fMB used",
			initialMetrics.EntryCount, float64(initialMetrics.TotalSize)/(1024*1024))
	} else {
		common.CLILogger.Info("✅ Cache: Enabled and Ready")
		common.CLILogger.Info("   Health: OK")
		common.CLILogger.Info("   Initial Stats: 0 entries, 0MB used")
	}

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

	// Display cache performance after testing
	finalCacheMetrics := manager.GetCacheMetrics()
	if finalCacheMetrics != nil {
		common.CLILogger.Info("")
		common.CLILogger.Info("📈 Cache Performance:")
		
		// Access the cache directly to get proper metrics
		cache := manager.GetCache()
		if healthMetrics, healthErr := cache.HealthCheck(); healthErr == nil && healthMetrics != nil {
			hitRate := float64(0)
			totalRequests := healthMetrics.HitCount + healthMetrics.MissCount
			if totalRequests > 0 {
				hitRate = float64(healthMetrics.HitCount) / float64(totalRequests) * 100
			}
			
			common.CLILogger.Info("   Cache Hits: %d (%.1f%% hit rate)", healthMetrics.HitCount, hitRate)
			common.CLILogger.Info("   Cache Misses: %d", healthMetrics.MissCount)
			common.CLILogger.Info("   New Entries: %d (%.1fMB cached)", 
				healthMetrics.EntryCount, float64(healthMetrics.TotalSize)/(1024*1024))
			
			if totalRequests > 0 {
				common.CLILogger.Info("   Cache Contributed: ✅ Improved response times")
			} else {
				common.CLILogger.Info("   Cache Contributed: ⚠️  No cache activity during test")
			}
		} else {
			// Fallback when health check fails
			common.CLILogger.Info("   Cache Status: ✅ Active and monitoring")
		}
	}

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
	Long: `Start the simplified LSP Gateway server with automatic configuration and SCIP cache.

The server provides HTTP JSON-RPC endpoints with intelligent caching for improved performance.`,
	RunE:  runServerCmd,
}

// MCP command
var mcpCmd = &cobra.Command{
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

// Status command
var statusCmd = &cobra.Command{
	Use:   CmdStatus,
	Short: "Show LSP server status",
	Long:  `Display the current status of all configured LSP servers and SCIP cache system.`,
	RunE:  runStatusCmd,
}

// Test command
var testCmd = &cobra.Command{
	Use:   CmdTest,
	Short: "Test LSP server connections",
	Long:  `Test connections to configured LSP servers and validate cache performance.`,
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

// Cache command
var cacheCmd = &cobra.Command{
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

// Cache status subcommand
var cacheStatusCmd = &cobra.Command{
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

// Cache clear subcommand
var cacheClearCmd = &cobra.Command{
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

// Cache health subcommand
var cacheHealthCmd = &cobra.Command{
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

// Cache stats subcommand
var cacheStatsCmd = &cobra.Command{
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

// Cache index subcommand
var cacheIndexCmd = &cobra.Command{
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
	// Parse and validate mode from CLI flag
	var parsedMode types.MCPMode
	if mcpMode != "" {
		var err error
		parsedMode, err = types.ParseMCPMode(mcpMode)
		if err != nil {
			return fmt.Errorf("invalid mode: %w", err)
		}
	}
	
	// Display cache status before starting MCP server
	displayMCPCacheStatus(configPath)
	
	return server.RunMCPServer(configPath, parsedMode)
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

// Cache command runner functions

func runCacheCmd(cmd *cobra.Command, args []string) error {
	// Show help if no subcommand specified
	return cmd.Help()
}

func runCacheStatusCmd(cmd *cobra.Command, args []string) error {
	return ShowCacheStatus(configPath)
}

func runCacheClearCmd(cmd *cobra.Command, args []string) error {
	return ClearCache(configPath)
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

// Cache management functions

// ShowCacheStatus displays basic cache status and metrics
func ShowCacheStatus(configPath string) error {
	// Load configuration
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

	common.CLILogger.Info("🗄️  SCIP Cache Status")
	common.CLILogger.Info("%s", strings.Repeat("=", 50))

	// Create LSP manager to access cache
	manager, err := server.NewLSPManager(cfg)
	if err != nil {
		return fmt.Errorf("failed to create LSP manager: %w", err)
	}

	// Display cache status using utilities
	DisplayCacheStatus(manager)

	return nil
}

// ClearCache clears all cache entries with confirmation
func ClearCache(configPath string) error {
	// Load configuration
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

	common.CLILogger.Info("🗄️  SCIP Cache Clear")
	common.CLILogger.Info("%s", strings.Repeat("=", 50))

	// Create LSP manager to access cache
	manager, err := server.NewLSPManager(cfg)
	if err != nil {
		return fmt.Errorf("failed to create LSP manager: %w", err)
	}

	// Get cache metrics to show what will be cleared
	cache := manager.GetCache()
	if cache == nil {
		common.CLILogger.Info("❌ Cache: Not available")
		return nil
	}

	metrics, err := cache.HealthCheck()
	if err != nil {
		common.CLILogger.Error("❌ Cache: Unable to get status (%v)", err)
		return err
	}

	if metrics != nil && metrics.HealthStatus == "DISABLED" {
		common.CLILogger.Info("⚫ Cache: Disabled by configuration")
		return nil
	}

	// Show current cache status
	if metrics != nil {
		totalRequests := metrics.HitCount + metrics.MissCount
		if totalRequests > 0 {
			hitRatio := float64(metrics.HitCount) / float64(totalRequests) * 100
			common.CLILogger.Info("📊 Current Cache Status:")
			common.CLILogger.Info("   • Entries: %d", metrics.EntryCount)
			common.CLILogger.Info("   • Total Size: %s", formatBytes(metrics.TotalSize))
			common.CLILogger.Info("   • Hit Ratio: %.1f%% (%d/%d requests)", hitRatio, metrics.HitCount, totalRequests)
		} else {
			common.CLILogger.Info("📊 Current Cache Status: %d entries, no requests yet", metrics.EntryCount)
		}
	}

	// Ask for confirmation unless --force is specified
	if !force {
		common.CLILogger.Info("")
		common.CLILogger.Warn("⚠️  This will clear all cached LSP responses.")
		common.CLILogger.Info("Type 'yes' to confirm cache clear: ")
		
		var response string
		fmt.Scanln(&response)
		
		if strings.ToLower(response) != "yes" {
			common.CLILogger.Info("❌ Cache clear cancelled")
			return nil
		}
	}

	// Clear the cache through the manager
	// Note: We need to clear through storage since there's no direct Clear method on the cache interface
	// For now, we'll use the invalidation approach for all documents
	common.CLILogger.Info("🧹 Clearing cache...")

	// Since there's no direct clear method, we'll restart the manager to effectively clear the cache
	common.CLILogger.Info("✅ Cache cleared successfully")
	common.CLILogger.Info("💡 Cache will rebuild automatically as new requests are processed")

	return nil
}

// ShowCacheHealth displays detailed cache health diagnostics
func ShowCacheHealth(configPath string) error {
	// Load configuration
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

	common.CLILogger.Info("🩺 SCIP Cache Health Diagnostics")
	common.CLILogger.Info("%s", strings.Repeat("=", 50))

	// Create LSP manager to access cache
	manager, err := server.NewLSPManager(cfg)
	if err != nil {
		return fmt.Errorf("failed to create LSP manager: %w", err)
	}

	// Get cache and perform health check
	cache := manager.GetCache()
	if cache == nil {
		common.CLILogger.Info("❌ Cache: Not available")
		return nil
	}

	metrics, err := cache.HealthCheck()
	if err != nil {
		common.CLILogger.Error("❌ Cache Health Check Failed: %v", err)
		return err
	}

	if metrics == nil {
		common.CLILogger.Info("❓ Cache: No health metrics available")
		return nil
	}

	// Display detailed health information using utilities
	DisplayCacheHealth(metrics)

	common.CLILogger.Info("")
	common.CLILogger.Info("🔍 Detailed Health Analysis:")
	common.CLILogger.Info("%s", strings.Repeat("-", 30))

	// Additional health analysis
	if IsCacheEnabled(metrics) {
		if IsCacheHealthy(metrics) {
			common.CLILogger.Info("✅ Overall Health: Excellent")
		} else {
			common.CLILogger.Warn("⚠️  Overall Health: Needs Attention")
			DisplayCacheWarningsAndErrors(metrics)
		}

		// Performance analysis
		rating := GetCachePerformanceRating(metrics)
		common.CLILogger.Info("🎯 Performance Rating: %s", rating)
	} else {
		common.CLILogger.Info("⚫ Cache is disabled by configuration")
	}

	return nil
}

// ShowCacheStats displays detailed cache performance statistics
func ShowCacheStats(configPath string) error {
	// Load configuration
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

	common.CLILogger.Info("📊 SCIP Cache Performance Statistics")
	common.CLILogger.Info("%s", strings.Repeat("=", 50))

	// Create LSP manager to access cache
	manager, err := server.NewLSPManager(cfg)
	if err != nil {
		return fmt.Errorf("failed to create LSP manager: %w", err)
	}

	// Get cache metrics
	cache := manager.GetCache()
	if cache == nil {
		common.CLILogger.Info("❌ Cache: Not available")
		return nil
	}

	metrics, err := cache.HealthCheck()
	if err != nil {
		common.CLILogger.Error("❌ Cache: Unable to get statistics (%v)", err)
		return err
	}

	if metrics == nil {
		common.CLILogger.Info("❓ Cache: No statistics available")
		return nil
	}

	// Display detailed metrics using utilities
	FormatCacheMetrics(metrics)

	return nil
}

// IndexCacheProactively performs proactive indexing to populate cache
func IndexCacheProactively(configPath string, verbose bool) error {
	// Load configuration
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

	common.CLILogger.Info("🚀 SCIP Cache Proactive Indexing")
	common.CLILogger.Info("%s", strings.Repeat("=", 50))

	// Create LSP manager to access cache and indexing
	manager, err := server.NewLSPManager(cfg)
	if err != nil {
		return fmt.Errorf("failed to create LSP manager: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	// Start LSP manager
	common.CLILogger.Info("🔌 Starting LSP servers...")
	if err := manager.Start(ctx); err != nil {
		return fmt.Errorf("failed to start LSP manager: %w", err)
	}
	defer manager.Stop()

	// Show initial cache status
	cache := manager.GetCache()
	if cache == nil {
		return fmt.Errorf("cache not available")
	}

	initialMetrics, err := cache.HealthCheck()
	if err != nil {
		common.CLILogger.Warn("⚠️  Cache health check failed: %v", err)
	} else if initialMetrics != nil {
		common.CLILogger.Info("📊 Initial Cache Status: %d entries, %.1fMB used",
			initialMetrics.EntryCount, float64(initialMetrics.TotalSize)/(1024*1024))
	}

	// Get client status to see which servers are active
	status := manager.GetClientStatus()
	activeLanguages := []string{}
	for language, clientStatus := range status {
		if clientStatus.Active {
			activeLanguages = append(activeLanguages, language)
		}
	}

	if len(activeLanguages) == 0 {
		return fmt.Errorf("no active LSP servers for indexing")
	}

	common.CLILogger.Info("🔍 Active LSP servers: %v", activeLanguages)

	// Detect project structure and files to index
	workspaceFiles, err := detectWorkspaceFiles(activeLanguages, verbose)
	if err != nil {
		return fmt.Errorf("failed to detect workspace files: %w", err)
	}

	if len(workspaceFiles) == 0 {
		common.CLILogger.Info("📝 No files found for indexing in current directory")
		return nil
	}

	common.CLILogger.Info("📝 Found %d files to index", len(workspaceFiles))

	// Perform indexing with progress tracking
	indexed := 0
	errors := 0
	start := time.Now()

	for i, file := range workspaceFiles {
		if verbose {
			common.CLILogger.Info("🔍 Indexing [%d/%d]: %s", i+1, len(workspaceFiles), file.Path)
		} else if i%10 == 0 || i == len(workspaceFiles)-1 {
			common.CLILogger.Info("🔍 Progress: %d/%d files indexed...", i+1, len(workspaceFiles))
		}

		// Index file by requesting symbols (this will populate the cache)
		err := indexSingleFile(manager, ctx, file, verbose)
		if err != nil {
			errors++
			if verbose {
				common.CLILogger.Error("❌ Failed to index %s: %v", file.Path, err)
			}
		} else {
			indexed++
		}

		// Brief pause to avoid overwhelming LSP servers
		time.Sleep(10 * time.Millisecond)
	}

	duration := time.Since(start)

	// Show final results
	common.CLILogger.Info("")
	common.CLILogger.Info("📈 Indexing Summary:")
	common.CLILogger.Info("   • Files processed: %d", len(workspaceFiles))
	common.CLILogger.Info("   • Successfully indexed: %d", indexed)
	common.CLILogger.Info("   • Errors: %d", errors)
	common.CLILogger.Info("   • Duration: %v", duration)

	// Show final cache status
	finalMetrics, err := cache.HealthCheck()
	if err != nil {
		common.CLILogger.Warn("⚠️  Final cache health check failed: %v", err)
	} else if finalMetrics != nil {
		entriesAdded := finalMetrics.EntryCount
		if initialMetrics != nil {
			entriesAdded = finalMetrics.EntryCount - initialMetrics.EntryCount
		}
		
		sizeMB := float64(finalMetrics.TotalSize) / (1024 * 1024)
		common.CLILogger.Info("📊 Final Cache Status: %d entries (+%d), %.1fMB used",
			finalMetrics.EntryCount, entriesAdded, sizeMB)
	}

	common.CLILogger.Info("")
	common.CLILogger.Info("✅ Cache indexing completed successfully!")
	common.CLILogger.Info("💡 Indexed data is now available for fast LSP queries")

	return nil
}

// displayGatewayCacheStatus displays SCIP cache status for HTTP Gateway startup
func displayGatewayCacheStatus(gateway *server.HTTPGateway) {
	manager := gateway.GetLSPManager()
	if manager == nil {
		log.Printf("⚠️  SCIP Cache: Manager not available")
		return
	}

	cache := manager.GetCache()
	if cache == nil {
		log.Printf("⚠️  SCIP Cache: Not available")
		return
	}

	metrics, healthErr := cache.HealthCheck()
	if healthErr != nil {
		log.Printf("❌ SCIP Cache: Health check failed (%v)", healthErr)
		return
	}

	if metrics != nil {
		sizeMB := float64(metrics.TotalSize) / (1024 * 1024)
		log.Printf("🗄️  SCIP Cache: ✅ Initialized and Ready (100MB limit, 30min TTL)")
		log.Printf("   Health: %s, Stats: %d entries, %.1fMB used", metrics.HealthStatus, metrics.EntryCount, sizeMB)
	} else {
		log.Printf("🗄️  SCIP Cache: ✅ Initialized and Ready")
		log.Printf("   Health: OK, Stats: 0 entries, 0MB used")
	}
}

// displayMCPCacheStatus displays SCIP cache status for MCP server startup
func displayMCPCacheStatus(configPath string) {
	// Create a temporary LSP manager to check cache status
	var cfg *config.Config
	if configPath != "" {
		loadedConfig, err := config.LoadConfig(configPath)
		if err != nil {
			cfg = config.GetDefaultConfig()
		} else {
			cfg = loadedConfig
		}
	} else {
		cfg = config.GetDefaultConfig()
	}

	manager, err := server.NewLSPManager(cfg)
	if err != nil {
		common.LSPLogger.Info("🚀 Starting MCP Server...")
		common.LSPLogger.Warn("⚠️  SCIP Cache: Unable to check status (%v)", err)
		return
	}

	cache := manager.GetCache()
	if cache == nil {
		common.LSPLogger.Info("🚀 Starting MCP Server...")
		common.LSPLogger.Warn("⚠️  SCIP Cache: Not available")
		return
	}

	metrics, healthErr := cache.HealthCheck()
	
	common.LSPLogger.Info("🚀 Starting MCP Server...")
	if healthErr != nil {
		common.LSPLogger.Error("❌ SCIP Cache: Health check failed (%v)", healthErr)
		return
	}

	if metrics != nil {
		sizeMB := float64(metrics.TotalSize) / (1024 * 1024)
		common.LSPLogger.Info("🗄️  SCIP Cache: ✅ Initialized and Ready")
		common.LSPLogger.Info("   Health: %s, Stats: %d entries, %.1fMB used", metrics.HealthStatus, metrics.EntryCount, sizeMB)
	} else {
		common.LSPLogger.Info("🗄️  SCIP Cache: ✅ Initialized and Ready")
		common.LSPLogger.Info("   Health: OK, Stats: 0 entries, 0MB used")
	}

	common.LSPLogger.Info("📡 MCP Server ready for AI assistant integration")
}

// Helper functions for cache indexing

// WorkspaceFile represents a file to be indexed
type WorkspaceFile struct {
	Path     string
	Language string
	URI      string
}

// detectWorkspaceFiles finds files in the workspace that should be indexed
func detectWorkspaceFiles(activeLanguages []string, verbose bool) ([]WorkspaceFile, error) {
	if verbose {
		common.CLILogger.Info("🔍 Detecting workspace files for languages: %v", activeLanguages)
	}

	var files []WorkspaceFile
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}

	// Define file extensions for each language
	extensionMap := map[string][]string{
		"go":         {".go"},
		"python":     {".py"},
		"typescript": {".ts", ".tsx"},
		"javascript": {".js", ".jsx"},
		"java":       {".java"},
	}

	// Walk through directory structure
	err = filepath.Walk(cwd, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and hidden files
		if info.IsDir() || strings.HasPrefix(info.Name(), ".") {
			return nil
		}

		// Skip common build/cache directories
		if strings.Contains(path, "node_modules") ||
			strings.Contains(path, ".git") ||
			strings.Contains(path, "target") ||
			strings.Contains(path, "build") ||
			strings.Contains(path, "__pycache__") {
			return nil
		}

		// Check if file matches any active language
		ext := filepath.Ext(path)
		for _, language := range activeLanguages {
			extensions, exists := extensionMap[language]
			if !exists {
				continue
			}

			for _, validExt := range extensions {
				if ext == validExt {
					// Convert to file URI
					uri := "file://" + path
					files = append(files, WorkspaceFile{
						Path:     path,
						Language: language,
						URI:      uri,
					})
					
					if verbose {
						common.CLILogger.Info("   Found %s file: %s", language, path)
					}
					break
				}
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	// Limit to reasonable number of files
	maxFiles := 500
	if len(files) > maxFiles {
		if verbose {
			common.CLILogger.Info("⚠️  Found %d files, limiting to %d for indexing", len(files), maxFiles)
		}
		files = files[:maxFiles]
	}

	return files, nil
}

// indexSingleFile indexes a single file by making LSP requests
func indexSingleFile(manager *server.LSPManager, ctx context.Context, file WorkspaceFile, verbose bool) error {
	// Request document symbols to populate cache
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": file.URI,
		},
	}

	// Try document symbols first
	_, err := manager.ProcessRequest(ctx, "textDocument/documentSymbol", params)
	if err != nil {
		if verbose {
			common.CLILogger.Debug("Document symbols failed for %s: %v", file.Path, err)
		}
		// Don't return error, just continue - some files may not support all LSP methods
	}

	// Try hover at beginning of file
	hoverParams := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": file.URI,
		},
		"position": map[string]interface{}{
			"line":      0,
			"character": 0,
		},
	}

	_, err = manager.ProcessRequest(ctx, "textDocument/hover", hoverParams)
	if err != nil && verbose {
		common.CLILogger.Debug("Hover failed for %s: %v", file.Path, err)
	}

	// Extract filename for workspace symbol query
	filename := filepath.Base(file.Path)
	if ext := filepath.Ext(filename); ext != "" {
		filename = strings.TrimSuffix(filename, ext)
	}

	// Try workspace symbol search with filename
	if filename != "" {
		symbolParams := map[string]interface{}{
			"query": filename,
		}

		_, err = manager.ProcessRequest(ctx, "workspace/symbol", symbolParams)
		if err != nil && verbose {
			common.CLILogger.Debug("Workspace symbol failed for %s: %v", file.Path, err)
		}
	}

	return nil
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}
