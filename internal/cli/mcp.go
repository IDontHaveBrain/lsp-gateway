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

	"lsp-gateway/internal/transport"
	"lsp-gateway/mcp"

	"github.com/spf13/cobra"
)

var (
	// MCP command flag variables - exported for testing purposes
	McpConfigPath string
	McpGatewayURL string
	McpPort       int
	McpTransport  string
	McpTimeout    time.Duration
	McpMaxRetries int
)

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
- Multiple transport support (stdio, tcp, http)
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
	mcpCmd.Flags().StringVarP(&McpConfigPath, "config", "c", "", "MCP configuration file path (optional)")
	mcpCmd.Flags().StringVarP(&McpGatewayURL, FLAG_GATEWAY, "g", DefaultLSPGatewayURL, "LSP Gateway URL")
	mcpCmd.Flags().IntVarP(&McpPort, FLAG_PORT, "p", 3000, "MCP server port (for HTTP transport)")
	mcpCmd.Flags().StringVarP(&McpTransport, FLAG_DESCRIPTION_TRANSPORT, "t", transport.TransportStdio, "Transport type (stdio, tcp, http)")
	mcpCmd.Flags().DurationVar(&McpTimeout, FLAG_TIMEOUT, 30*time.Second, "Request timeout duration")
	mcpCmd.Flags().IntVar(&McpMaxRetries, "max-retries", 3, "Maximum retries for failed requests")

	rootCmd.AddCommand(mcpCmd)
}

// GetMcpCmd returns the mcp command for testing purposes
func GetMcpCmd() *cobra.Command {
	return mcpCmd
}

func runMCPServer(_ *cobra.Command, args []string) error {
	log.Printf("[INFO] Starting MCP server with transport=%s, gateway_url=%s\n", McpTransport, McpGatewayURL)

	logger := createMCPLogger()
	server, err := setupMCPServer(logger)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	return runMCPTransport(ctx, server, logger)
}

func createMCPHTTPServer(server *mcp.Server, port int) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		if server.IsRunning() {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{"status":"ok","timestamp":%d}`, time.Now().Unix())
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = fmt.Fprintf(w, `{"status":"error","message":"server not running"}`)
		}
	})

	mux.HandleFunc("/mcp", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", transport.HTTP_CONTENT_TYPE_JSON)
		w.WriteHeader(http.StatusNotImplemented)
		_, _ = fmt.Fprintf(w, `{"error":"HTTP MCP transport not yet implemented","message":"Use stdio transport for now"}`)
	})

	return &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
}

func runMCPHTTPLifecycle(ctx context.Context, httpServer *http.Server, port int, logger *mcp.StructuredLogger) error {
	serverErr := createServerErrorChannel()
	go func() {
		logger.WithField("port", port).Info("MCP HTTP server listening")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.WithError(err).Error("MCP HTTP server error")
			serverErr <- HandleServerStartError(err, port)
		}
	}()

	config := &ServerLifecycleConfig{
		ServerName:      "MCP HTTP server",
		Port:            port,
		ShutdownFunc:    func() error { return shutdownMCPHTTPServerInternal(httpServer) },
		ShutdownTimeout: 30 * time.Second,
	}

	logger.WithField("port", port).Info("MCP HTTP server started successfully, waiting for requests")
	return waitForShutdownSignal(ctx, config, serverErr)
}

func shutdownMCPHTTPServerInternal(httpServer *http.Server) error {
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		return err
	}
	return nil
}

// RunMCPStdioServer starts the MCP server with stdio transport - exported for testing
func RunMCPStdioServer(ctx context.Context, server *mcp.Server) error {
	log.Printf("[INFO] Starting MCP server with stdio transport, gateway_url=%s\n", McpGatewayURL)

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("[DEBUG] Starting MCP server\n")
		if err := server.Start(); err != nil {
			log.Printf("[ERROR] MCP server startup failed: %v\n", err)
			serverErr <- NewMCPServerError("failed to start", err)
		} else {
			log.Printf("[INFO] MCP server started successfully\n")
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	log.Printf("[INFO] MCP server is running, waiting for requests\n")

	select {
	case <-sigCh:
		log.Printf("[INFO] Received shutdown signal\n")
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		log.Printf("[INFO] Context cancelled\n")
	}

	log.Printf("[INFO] Shutting down MCP server\n")

	if err := server.Stop(); err != nil {
		log.Printf("[WARN] Error stopping MCP server during shutdown: %v\n", err)
	} else {
		log.Printf("[INFO] MCP server stopped successfully\n")
	}

	return nil
}

func runMCPHTTPServer(ctx context.Context, server *mcp.Server, port int, logger *mcp.StructuredLogger) error {
	log.Printf("[INFO] Starting MCP server with HTTP transport on port %d\n", port)

	httpServer := createMCPHTTPServer(server, port)
	return runMCPHTTPLifecycle(ctx, httpServer, port, logger)
}

func createMCPLogger() *mcp.StructuredLogger {
	logConfig := &mcp.LoggerConfig{
		Level:              mcp.LogLevelInfo,
		Component:          "mcp-cli",
		EnableJSON:         false,
		EnableStackTrace:   false,
		EnableCaller:       true,
		EnableMetrics:      false,
		Output:             os.Stderr,
		IncludeTimestamp:   true,
		TimestampFormat:    time.RFC3339,
		MaxStackTraceDepth: 10,
		EnableAsyncLogging: false,
		AsyncBufferSize:    1000,
	}
	return mcp.NewStructuredLogger(logConfig)
}

func setupMCPServer(logger *mcp.StructuredLogger) (*mcp.Server, error) {
	if err := validateMCPParams(); err != nil {
		log.Printf("[ERROR] MCP parameter validation failed: %v\n", err)
		return nil, err
	}

	cfg := &mcp.ServerConfig{
		Name:          "lsp-gateway-mcp",
		Description:   "MCP server providing LSP functionality through LSP Gateway",
		Version:       "0.1.0",
		LSPGatewayURL: McpGatewayURL,
		Transport:     McpTransport,
		Timeout:       McpTimeout,
		MaxRetries:    McpMaxRetries,
	}

	log.Printf("[DEBUG] MCP server configuration created\n")

	if McpConfigPath != "" {
		log.Printf("[INFO] Configuration file specified (currently using command-line flags): %s\n", McpConfigPath)
	}

	logger.Debug("Validating MCP configuration")
	if err := cfg.Validate(); err != nil {
		logger.WithError(err).Error("MCP configuration validation failed")
		return nil, NewValidationError("MCP configuration", []string{err.Error()})
	}
	logger.Info("MCP configuration validated successfully")

	logger.Debug("Creating MCP server")
	server := mcp.NewServer(cfg)
	logger.Info("MCP server created successfully")

	return server, nil
}

func runMCPTransport(ctx context.Context, server *mcp.Server, logger *mcp.StructuredLogger) error {
	if McpTransport == transport.TransportHTTP {
		logger.WithField("port", McpPort).Info("Using HTTP transport for MCP server")
		return runMCPHTTPServer(ctx, server, McpPort, logger)
	}

	logger.Info("Using stdio transport for MCP server")
	return RunMCPStdioServer(ctx, server)
}

func validateMCPParams() error {
	err := ValidateMultiple(
		func() *ValidationError {
			return ValidateTransport(McpTransport, FLAG_DESCRIPTION_TRANSPORT)
		},
		func() *ValidationError {
			return ValidateURL(McpGatewayURL, FLAG_GATEWAY)
		},
		func() *ValidationError {
			if McpTransport == transport.TransportHTTP {
				return ValidatePortAvailability(McpPort, FLAG_PORT)
			}
			return ValidatePort(McpPort, "port")
		},
		func() *ValidationError {
			return ValidateTimeout(McpTimeout, FLAG_TIMEOUT)
		},
		func() *ValidationError {
			return ValidateIntRange(McpMaxRetries, 0, 10, "max-retries")
		},
		func() *ValidationError {
			if McpConfigPath != "" {
				return ValidateFilePath(McpConfigPath, "config", "read")
			}
			return nil
		},
	)
	if err == nil {
		return nil
	}
	return err
}
