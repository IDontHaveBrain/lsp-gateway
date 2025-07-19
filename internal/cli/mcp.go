package cli

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"lsp-gateway/internal/transport"
	"lsp-gateway/mcp"
)

var (
	mcpConfigPath string
	mcpGatewayURL string
	mcpPort       int
	mcpTransport  string
	mcpTimeout    time.Duration
	mcpMaxRetries int
)

// mcpCmd represents the mcp command
var mcpCmd = &cobra.Command{
	Use:   CmdMCP,
	Short: "Start the MCP server",
	Long: `Start the Model Context Protocol (MCP) server that provides LSP functionality.

The MCP server acts as a bridge between MCP clients and the LSP Gateway, exposing
LSP methods as MCP tools. This enables AI assistants and other MCP clients to
interact with language servers for code analysis, symbol search, definition lookup,
and reference finding.

Features:
- MCP protocol implementation
- LSP method mapping to MCP tools
- Multiple transport support (stdio, http)
- Graceful shutdown handling

Examples:
  # Start MCP server with default settings (stdio transport)
  lsp-gateway mcp

  # Start with custom LSP Gateway URL
  lsp-gateway mcp --gateway http://localhost:9090

  # Start with HTTP transport on specific port
  lsp-gateway mcp --transport http --port 3000

  # Start with custom configuration and timeout
  lsp-gateway mcp --config mcp-config.yaml --timeout 60s`,
	RunE: runMCPServer,
}

func init() {
	mcpCmd.Flags().StringVarP(&mcpConfigPath, "config", "c", "", "MCP configuration file path (optional)")
	mcpCmd.Flags().StringVarP(&mcpGatewayURL, "gateway", "g", DefaultLSPGatewayURL, "LSP Gateway URL")
	mcpCmd.Flags().IntVarP(&mcpPort, "port", "p", 3000, "MCP server port (for HTTP transport)")
	mcpCmd.Flags().StringVarP(&mcpTransport, "transport", "t", transport.TransportStdio, "Transport type (stdio, http)")
	mcpCmd.Flags().DurationVar(&mcpTimeout, "timeout", 30*time.Second, "Request timeout duration")
	mcpCmd.Flags().IntVar(&mcpMaxRetries, "max-retries", 3, "Maximum retries for failed requests")

	// Add mcp command to root
	rootCmd.AddCommand(mcpCmd)
}

func runMCPServer(cmd *cobra.Command, args []string) error {
	// Create MCP server configuration
	cfg := &mcp.ServerConfig{
		Name:          "lsp-gateway-mcp",
		Description:   "MCP server providing LSP functionality through LSP Gateway",
		Version:       "0.1.0",
		LSPGatewayURL: mcpGatewayURL,
		Transport:     mcpTransport,
		Timeout:       mcpTimeout,
		MaxRetries:    mcpMaxRetries,
	}

	// Load configuration from file if specified
	if mcpConfigPath != "" {
		// Note: This could be extended to load from YAML file if needed
		// For now, we use command-line flags as primary configuration
		log.Printf("Configuration file specified: %s (currently using command-line flags)", mcpConfigPath)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid MCP configuration: %w", err)
	}

	// Create MCP server
	server := mcp.NewServer(cfg)

	// Setup context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// For HTTP transport, setup HTTP server
	if mcpTransport == transport.TransportHTTP {
		return runMCPHTTPServer(ctx, server, mcpPort)
	}

	// For stdio transport (default), run directly
	return runMCPStdioServer(ctx, server)
}

func runMCPStdioServer(ctx context.Context, server *mcp.Server) error {
	log.Println("Starting MCP server with stdio transport")
	log.Printf("LSP Gateway URL: %s", mcpGatewayURL)

	// Start server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		if err := server.Start(); err != nil {
			serverErr <- fmt.Errorf("MCP server error: %w", err)
		}
	}()

	// Wait for interrupt signal or server error
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigCh:
		log.Println("Received shutdown signal")
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		log.Println("Context cancelled")
	}

	log.Println("Shutting down MCP server...")

	// Stop server
	if err := server.Stop(); err != nil {
		log.Printf("Error stopping MCP server: %v", err)
	}

	log.Println("MCP server stopped")
	return nil
}

func runMCPHTTPServer(ctx context.Context, server *mcp.Server, port int) error {
	log.Printf("Starting MCP server with HTTP transport on port %d", port)

	// Create HTTP handler for MCP over HTTP
	// Note: This is a placeholder for HTTP transport implementation
	// The current MCP server implementation primarily uses stdio
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if server.IsRunning() {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{"status":"ok","timestamp":%d}`, time.Now().Unix())
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = fmt.Fprintf(w, `{"status":"error","message":"server not running"}`)
		}
	})

	// MCP endpoint (placeholder for future HTTP MCP implementation)
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotImplemented)
		_, _ = fmt.Fprintf(w, `{"error":"HTTP MCP transport not yet implemented","message":"Use stdio transport for now"}`)
	})

	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// Start HTTP server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		log.Printf("MCP HTTP server listening on :%d", port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- fmt.Errorf("HTTP server error: %w", err)
		}
	}()

	// Wait for interrupt signal or server error
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigCh:
		log.Println("Received shutdown signal")
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		log.Println("Context cancelled")
	}

	log.Println("Shutting down MCP HTTP server...")

	// Shutdown HTTP server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	log.Println("MCP HTTP server stopped")
	return nil
}
