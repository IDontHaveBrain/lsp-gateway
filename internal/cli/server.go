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

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/gateway"
	"github.com/spf13/cobra"
)

var (
	configPath string
	port       int
)

// serverCmd represents the server command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the LSP Gateway server",
	Long:  `Start the LSP Gateway server with the specified configuration.`,
	RunE:  runServer,
}

func init() {
	serverCmd.Flags().StringVarP(&configPath, "config", "c", "config.yaml", "Configuration file path")
	serverCmd.Flags().IntVarP(&port, "port", "p", 8080, "Server port")
	
	// Add server command to root
	rootCmd.AddCommand(serverCmd)
}

func runServer(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	
	// Override port if specified
	if port != 8080 {
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	if err := gw.Start(ctx); err != nil {
		return fmt.Errorf("failed to start gateway: %w", err)
	}
	defer gw.Stop()
	
	// Setup HTTP server
	http.HandleFunc("/jsonrpc", gw.HandleJSONRPC)
	
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      nil,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	
	// Start server in goroutine
	go func() {
		log.Printf("Starting LSP Gateway server on port %d", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Server error: %v", err)
		}
	}()
	
	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	
	log.Println("Shutting down server...")
	
	// Shutdown server
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}
	
	log.Println("Server stopped")
	return nil
}