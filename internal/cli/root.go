package cli

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "lspg",
	Short: "LSP Gateway - A JSON-RPC gateway for Language Server Protocol servers",
	Long: `LSP Gateway provides a unified JSON-RPC interface for multiple Language Server Protocol (LSP) servers
with embedded auto-setup capabilities and Model Context Protocol (MCP) server functionality.

🚀 QUICK START (30 seconds):
  lspg setup all                    # Complete automated setup
  lspg server --config config.yaml # Start HTTP Gateway (port 8080)
  lspg mcp --config config.yaml    # Start MCP Server (stdio)

📋 CORE FEATURES:
  • Multi-language LSP server management (Go, Python, TypeScript, Java)
  • HTTP JSON-RPC Gateway for IDE/editor integration
  • MCP Server for AI assistant integration (Claude, ChatGPT, etc.)
  • Embedded auto-setup system with runtime detection
  • Cross-platform installation and configuration
  • Automatic server routing based on file types

🔧 COMMON WORKFLOWS:

  First-time setup:
    lspg setup wizard              # Interactive guided setup
    lspg config validate           # Verify configuration
    lspg status runtimes           # Check installation status

  Development workflow:
    lspg server --port 8080        # Start HTTP Gateway
    lspg mcp --transport stdio     # Start MCP Server for AI tools

  Maintenance:
    lspg diagnose                  # System health check
    lspg install servers          # Install missing language servers
    lspg verify runtime go        # Verify specific runtime

  Configuration management:
    lspg config generate --auto-detect    # Auto-generate config
    lspg config show --json              # View current config
    lspg config validate                 # Validate config file

🎯 INTEGRATION MODES:
  • HTTP Gateway: JSON-RPC endpoint at http://localhost:8080/jsonrpc
  • MCP Server: AI assistant integration via Model Context Protocol
  • Direct LSP: Connect to individual language servers

💡 SUPPORTED LANGUAGES:
  • Go (gopls) - Definition lookup, references, symbols, hover
  • Python (pylsp) - Code analysis, completion, diagnostics
  • TypeScript/JavaScript (typescript-language-server) - Full language support
  • Java (jdtls) - Enterprise-grade Java development

Use 'lspg <command> --help' for detailed command information.
Get started: lspg setup all`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() error {
	return rootCmd.Execute()
}
