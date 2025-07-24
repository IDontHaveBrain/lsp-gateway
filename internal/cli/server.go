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
	DefaultServerPort    = 8080
	DefaultLSPGatewayURL = "http://localhost:8080"
)

var (
	configPath string
	port       int
)

var serverCmd = &cobra.Command{
	Use:   CmdServer,
	Short: "Start the LSP Gateway server",
	Long:  `Start the LSP Gateway server with the specified configuration.`,
	RunE:  runServer,
}

func init() {
	serverCmd.Flags().StringVarP(&configPath, "config", "c", DefaultConfigFile, "Configuration file path")
	serverCmd.Flags().IntVarP(&port, FLAG_PORT, "p", DefaultServerPort, FLAG_DESCRIPTION_SERVER_PORT)

	rootCmd.AddCommand(serverCmd)
}

// GetServerCmd returns the server command for testing purposes
func GetServerCmd() *cobra.Command {
	return serverCmd
}

func runServer(cmd *cobra.Command, args []string) error {
	return runServerWithContext(cmd.Context(), cmd, args)
}

func runServerWithContext(ctx context.Context, _ *cobra.Command, args []string) error {
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

	// Add health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		// Check if gateway is properly initialized and has active clients
		if gw == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = fmt.Fprintf(w, `{"status":"error","message":"gateway not initialized","timestamp":%d}`, time.Now().Unix())
			return
		}

		// Check if at least one LSP client is active
		hasActiveClient := false
		if len(gw.Clients) > 0 {
			for _, client := range gw.Clients {
				if client.IsActive() {
					hasActiveClient = true
					break
				}
			}
		}

		if hasActiveClient {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `{"status":"ok","active_clients":%d,"timestamp":%d}`, len(gw.Clients), time.Now().Unix())
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = fmt.Fprintf(w, `{"status":"starting","message":"no active LSP clients yet","timestamp":%d}`, time.Now().Unix())
		}
	})

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
