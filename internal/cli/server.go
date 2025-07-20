package cli

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/gateway"

	"github.com/spf13/cobra"
)

const (
	CmdServer   = "server"
	CmdMCP      = "mcp"
	CmdVersion  = "version"
	CmdStatus   = "status"
	CmdDiagnose = "diagnose"
	CmdInstall  = "install"
	CmdVerify   = "verify"
	CmdSetup    = "setup"
	CmdConfig   = "config"
)

const (
	DefaultConfigFile    = "config.yaml"
	DefaultServerPort    = 8080
	DefaultLSPGatewayURL = "http://localhost:8080"
)

var (
	configPath string
	port       int
)

var serverCmd = &cobra.Command{
	Use:   CmdServer,
	Short: "Start the LSP Gateway HTTP server",
	Long: `Start the LSP Gateway HTTP server that provides a unified JSON-RPC interface 
for multiple Language Server Protocol (LSP) servers.

🌐 HTTP GATEWAY OVERVIEW:
The server command starts an HTTP server that accepts JSON-RPC requests and routes them
to appropriate language servers based on file extensions and language configuration.

📡 ENDPOINT: http://localhost:8080/jsonrpc (default)

🔧 SERVER FEATURES:
  • Multi-language LSP server management
  • Automatic routing based on file types (.go → gopls, .py → pylsp, etc.)
  • Concurrent request handling with connection pooling
  • Graceful shutdown with SIGINT/SIGTERM handling
  • Health monitoring and error reporting
  • Configurable timeouts and retry logic

💻 USAGE EXAMPLES:

  Basic usage:
    lsp-gateway server                          # Start with default config (config.yaml)
    lsp-gateway server --config config.yaml    # Start with specific config file
    lsp-gateway server --port 8080             # Start on specific port
    
  Configuration override:
    lsp-gateway server --port 9090             # Override config port setting
    lsp-gateway server --config custom.yaml    # Use custom configuration
    
  Development workflow:
    lsp-gateway setup all                      # First: setup system
    lsp-gateway config validate                # Second: verify config
    lsp-gateway server                         # Third: start server

🚨 TROUBLESHOOTING:

  Common issues and solutions:
  
  "Configuration file not found"
    → Run: lsp-gateway config generate
    → Or: lsp-gateway setup all
    
  "Port already in use"
    → Use: lsp-gateway server --port 8081
    → Or: Check running processes: lsof -i :8080
    
  "Language server not found"
    → Run: lsp-gateway install servers
    → Or: lsp-gateway diagnose
    
  "Server fails to start"
    → Check: lsp-gateway config validate
    → Check: lsp-gateway status runtimes

🔗 RELATED COMMANDS:
  lsp-gateway config generate    # Generate server configuration
  lsp-gateway status runtimes    # Check runtime installations  
  lsp-gateway install servers    # Install missing language servers
  lsp-gateway diagnose          # Troubleshoot server issues
  lsp-gateway mcp               # Start MCP server instead

🌍 INTEGRATION:
Once started, the server accepts JSON-RPC requests at the /jsonrpc endpoint.
Supported LSP methods: textDocument/definition, textDocument/references,
textDocument/documentSymbol, workspace/symbol, textDocument/hover

For MCP server functionality, use 'lsp-gateway mcp' instead.`,
	RunE: runServer,
}

func init() {
	serverCmd.Flags().StringVarP(&configPath, "config", "c", DefaultConfigFile, "Configuration file path")
	serverCmd.Flags().IntVarP(&port, FLAG_PORT, "p", DefaultServerPort, FLAG_DESCRIPTION_SERVER_PORT)

	rootCmd.AddCommand(serverCmd)
}

func runServer(cmd *cobra.Command, args []string) error {
	return runServerWithContext(cmd.Context(), cmd, args)
}

func runServerWithContext(ctx context.Context, cmd *cobra.Command, args []string) error {
	log.Printf("[INFO] Starting LSP Gateway server with config_path=%s\n", configPath)

	if ctx == nil {
		ctx = context.Background()
	}

	cfg, err := setupServerConfiguration()
	if err != nil {
		return err
	}

	gw, err := initializeGateway(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanupGateway(gw)

	server := createHTTPServer(cfg, gw)
	return runServerLifecycle(ctx, server, cfg.Port)
}

func setupServerConfiguration() (*config.GatewayConfig, error) {
	if err := validateServerParams(); err != nil {
		log.Printf("[ERROR] Server parameter validation failed: %v\n", err)
		return nil, err
	}

	log.Printf("[DEBUG] Loading configuration\n")
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Printf("[ERROR] Failed to load configuration from %s: %v\n", configPath, err)
		return nil, HandleConfigError(err, configPath)
	}

	log.Printf("[INFO] Configuration loaded from %s\n", configPath)

	if port != DefaultServerPort {
		log.Printf("[INFO] Port override specified: default_port=%d, override_port=%d\n", DefaultServerPort, port)
		cfg.Port = port
	}

	log.Printf("[DEBUG] Validating configuration\n")
	if err := config.ValidateConfig(cfg); err != nil {
		log.Printf("[ERROR] Configuration validation failed: %v\n", err)
		return nil, NewValidationError("configuration", []string{err.Error()})
	}
	log.Printf("[INFO] Configuration validated successfully\n")

	log.Printf("[DEBUG] Checking port availability on %d\n", cfg.Port)
	if err := ValidatePortAvailability(cfg.Port, "port"); err != nil {
		log.Printf("[ERROR] Port %d not available: %v\n", cfg.Port, err)
		return nil, ToValidationError(err)
	}

	return cfg, nil
}

func initializeGateway(ctx context.Context, cfg *config.GatewayConfig) (*gateway.Gateway, error) {
	log.Printf("[DEBUG] Creating LSP gateway\n")
	gw, err := gateway.NewGateway(cfg)
	if err != nil {
		log.Printf("[ERROR] Failed to create gateway: %v\n", err)
		return nil, NewGatewayStartupError(err)
	}
	log.Printf("[INFO] Gateway created successfully\n")

	log.Printf("[INFO] Starting gateway\n")
	if err := gw.Start(ctx); err != nil {
		log.Printf("[ERROR] Failed to start gateway: %v\n", err)
		return nil, NewGatewayStartupError(err)
	}

	return gw, nil
}

func cleanupGateway(gw *gateway.Gateway) {
	if err := gw.Stop(); err != nil {
		log.Printf("[WARN] Error stopping gateway during shutdown: %v\n", err)
	}
}

func createHTTPServer(cfg *config.GatewayConfig, gw *gateway.Gateway) *http.Server {
	log.Printf("[DEBUG] Setting up HTTP server\n")
	
	// Create a new ServeMux to avoid global state conflicts in tests
	mux := http.NewServeMux()
	mux.HandleFunc("/jsonrpc", gw.HandleJSONRPC)

	return &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
}

func runServerLifecycle(ctx context.Context, server *http.Server, port int) error {
	serverErr := createServerErrorChannel()
	go func() {
		logServerStart("LSP Gateway HTTP server", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logServerError("HTTP server", err)
			serverErr <- HandleServerStartError(err, port)
		}
	}()

	config := &ServerLifecycleConfig{
		ServerName:      "LSP Gateway server",
		Port:            port,
		ShutdownFunc:    func() error { return shutdownHTTPServer(server) },
		ShutdownTimeout: 30 * time.Second,
	}

	return waitForShutdownSignal(ctx, config, serverErr)
}

func shutdownHTTPServer(server *http.Server) error {
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return err
	}
	return nil
}

func validateServerParams() error {
	err := ValidateMultiple(
		func() *ValidationError {
			return ValidateFilePath(configPath, "config", "read")
		},
		func() *ValidationError {
			return ValidatePort(port, "port")
		},
	)
	if err == nil {
		return nil
	}
	return err
}
