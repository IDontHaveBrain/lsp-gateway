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
	"lsp-gateway/internal/config"
	"lsp-gateway/internal/gateway"
)

// CLI command constants
const (
	CmdServer  = "server"
	CmdMCP     = "mcp"
	CmdVersion = "version"
)

// Default configuration constants
const (
	DefaultConfigFile    = "config.yaml"
	DefaultServerPort    = 8080
	DefaultLSPGatewayURL = "http://localhost:8080"
)

var (
	configPath string
	port       int
)

// serverCmd represents the server command
var serverCmd = &cobra.Command{
	Use:   CmdServer,
	Short: "Start the LSP Gateway server",
	Long:  `Start the LSP Gateway server with the specified configuration.`,
	RunE:  runServer,
}

func init() {
	serverCmd.Flags().StringVarP(&configPath, "config", "c", DefaultConfigFile, "Configuration file path")
	serverCmd.Flags().IntVarP(&port, "port", "p", DefaultServerPort, "Server port")

	// Add server command to root
	rootCmd.AddCommand(serverCmd)
}

func runServer(cmd *cobra.Command, args []string) error {
	return runServerWithContext(cmd.Context(), cmd, args)
}

func runServerWithContext(ctx context.Context, cmd *cobra.Command, args []string) error {
	// Handle nil context by creating a background context
	if ctx == nil {
		ctx = context.Background()
	}
	// Load configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Override port if specified
	if port != DefaultServerPort {
		cfg.Port = port
	}

	// Validate configuration
	if err := config.ValidateConfig(cfg); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Create gateway
	gw, err := gateway.NewGateway(cfg)
	if err != nil {
		return fmt.Errorf("failed to create gateway: %w", err)
	}

	// Start gateway
	gatewayCtx, gatewayCancel := context.WithCancel(ctx)
	defer gatewayCancel()

	if err := gw.Start(gatewayCtx); err != nil {
		return fmt.Errorf("failed to start gateway: %w", err)
	}
	defer func() {
		if err := gw.Stop(); err != nil {
			fmt.Printf("Error stopping gateway: %v\n", err)
		}
	}()

	// Setup HTTP server
	http.HandleFunc("/jsonrpc", gw.HandleJSONRPC)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      nil,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// Start server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		log.Printf("Starting LSP Gateway server on port %d", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Server error: %v", err)
			serverErr <- err
		}
	}()

	// Setup signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	// Wait for context cancellation, signal, or server error
	select {
	case <-ctx.Done():
		log.Println("Context cancelled, shutting down server...")
	case <-sigCh:
		log.Println("Received shutdown signal, shutting down server...")
	case err := <-serverErr:
		log.Printf("Server error: %v", err)
		return err
	}

	// Shutdown server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("Server stopped")
	return nil
}
