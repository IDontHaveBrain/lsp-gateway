package cli

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "lsp-gateway",
	Short: "LSP Gateway - A JSON-RPC gateway for Language Server Protocol servers",
	Long: `LSP Gateway provides a unified JSON-RPC interface for multiple Language Server Protocol (LSP) servers
with Model Context Protocol (MCP) server functionality.

🚀 QUICK START (30 seconds):
  lsp-gateway install runtime all         # Install language runtimes
  lsp-gateway install servers             # Install language servers
  lsp-gateway server --config config.yaml # Start HTTP Gateway (port 8080)
  lsp-gateway mcp --config config.yaml    # Start MCP Server (stdio)

📋 CORE FEATURES:
  • Multi-language LSP server management (Go, Python, TypeScript, Java)
  • HTTP JSON-RPC Gateway for IDE/editor integration
  • MCP Server for AI assistant integration (Claude, ChatGPT, etc.)
  • Cross-platform installation and configuration
  • Automatic server routing based on file types

🔧 COMMON WORKFLOWS:

  Installation:
    lsp-gateway install runtime all      # Install all language runtimes
    lsp-gateway install servers          # Install all language servers
    lsp-gateway config validate          # Verify configuration
    lsp-gateway status runtimes          # Check installation status

  Development workflow:
    lsp-gateway server --port 8080        # Start HTTP Gateway
    lsp-gateway mcp --transport stdio     # Start MCP Server for AI tools

  Maintenance:
    lsp-gateway diagnose                  # System health check
    lsp-gateway install servers          # Install missing language servers
    lsp-gateway verify runtime go        # Verify specific runtime

  Configuration management:
    lsp-gateway config generate          # Generate config
    lsp-gateway config show --json       # View current config
    lsp-gateway config validate          # Validate config file

🎯 INTEGRATION MODES:
  • HTTP Gateway: JSON-RPC endpoint at http://localhost:8080/jsonrpc
  • MCP Server: AI assistant integration via Model Context Protocol
  • Direct LSP: Connect to individual language servers

💡 SUPPORTED LANGUAGES:
  • Go (gopls) - Definition lookup, references, symbols, hover
  • Python (pylsp) - Code analysis, completion, diagnostics
  • TypeScript/JavaScript (typescript-language-server) - Full language support
  • Java (jdtls) - Enterprise-grade Java development

Use 'lsp-gateway <command> --help' for detailed command information.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() error {
	return rootCmd.Execute()
}

// GetRootCmd returns the root command for testing purposes
func GetRootCmd() *cobra.Command {
	return rootCmd
}
