package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "lsp-gateway",
	Short: "LSP Gateway - A JSON-RPC gateway for Language Server Protocol servers",
	Long: `LSP Gateway provides a unified JSON-RPC interface for multiple Language Server Protocol (LSP) servers.
It enables LSP clients to interact with language servers for code analysis, symbol search, definition lookup,
and reference finding across multiple programming languages.

Features:
- Multi-language LSP server management
- JSON-RPC interface over HTTP
- Automatic server routing based on file types
- Simple YAML configuration`,
	// Don't show usage when there's an error
	SilenceUsage: true,
	// Don't show errors (we'll handle them ourselves)
	SilenceErrors: true,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
