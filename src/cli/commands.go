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
		cfg = config.GetDefaultConfig()
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
	}

	fmt.Println("🔍 LSP Gateway Status")
	fmt.Println(strings.Repeat("=", 50))

	manager, err := server.NewLSPManager(cfg)
	if err != nil {
		return fmt.Errorf("failed to create LSP manager: %w", err)
	}

	// Check server availability without starting them (fast, non-destructive)
	status := manager.CheckServerAvailability()

	fmt.Printf("📊 Configured Languages: %d\n\n", len(cfg.Servers))

	for language, serverConfig := range cfg.Servers {
		clientStatus := status[language]
		statusIcon := "❌"
		statusText := "Unavailable"

		if clientStatus.Available {
			statusIcon = "✅"
			statusText = "Available"
		}

		fmt.Printf("%s %s: %s\n", statusIcon, language, statusText)
		fmt.Printf("   Command: %s %v\n", serverConfig.Command, serverConfig.Args)

		if !clientStatus.Available && clientStatus.Error != nil {
			fmt.Printf("   Error: %v\n", clientStatus.Error)
		}

		fmt.Println()
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
	}

	fmt.Println("🧪 Testing LSP Server Connections")
	fmt.Println(strings.Repeat("=", 50))

	manager, err := server.NewLSPManager(cfg)
	if err != nil {
		return fmt.Errorf("failed to create LSP manager: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := manager.Start(ctx); err != nil {
		return fmt.Errorf("failed to start LSP manager: %w", err)
	}
	defer manager.Stop()

	// Test basic workspace/symbol request
	params := map[string]interface{}{
		"query": "test",
	}

	fmt.Println("Testing workspace/symbol request...")
	result, err := manager.ProcessRequest(ctx, "workspace/symbol", params)
	if err != nil {
		fmt.Printf("❌ Test failed: %v\n", err)
		return err
	}

	fmt.Printf("✅ Test successful! Result type: %T\n", result)
	return nil
}

// CLI Constants
const (
	DefaultServerPort = 8080
	CmdServer         = "server"
	CmdMCP            = "mcp"
	CmdStatus         = "status"
	CmdTest           = "test"
	FlagConfig        = "config"
	FlagPort          = "port"
)

// CLI Variables
var (
	configPath string
	port       int
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
  • Python (pylsp) - Code analysis, completion, diagnostics  
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

	// Add commands to root
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(mcpCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(testCmd)
}

// Command runner functions
func runServerCmd(cmd *cobra.Command, args []string) error {
	// Determine server address
	addr := fmt.Sprintf(":%d", port)

	// Check if config file was provided and exists
	var configFile string
	if configPath != "" {
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			fmt.Printf("Warning: Config file %s not found, using defaults\n", configPath)
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

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}
