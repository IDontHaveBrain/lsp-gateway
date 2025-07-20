package cli

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"lsp-gateway/internal/common"
	"lsp-gateway/internal/gateway"

	"github.com/spf13/cobra"
)

var serverRefactoredCmd = &cobra.Command{
	Use:   "server-refactored",
	Short: "Start the LSP Gateway HTTP server (refactored version)",
	Long: `This is a refactored version of the server command that demonstrates
how to use the consolidated shared utilities from internal/common.

This version eliminates duplicate patterns by using:
- common.ServerLifecycleManager for standardized server lifecycle management
- common.ConfigManager for standardized configuration loading and validation
- common.OutputManager for consistent output formatting (if verbose mode enabled)

The refactored version reduces code duplication and provides consistent
error handling and lifecycle management across all server commands.`,
	RunE: runServerRefactored,
}

func init() {
	serverRefactoredCmd.Flags().StringVarP(&configPath, "config", "c", DefaultConfigFile, "Configuration file path")
	serverRefactoredCmd.Flags().IntVarP(&port, "port", "p", DefaultServerPort, "Server port")

}

func runServerRefactored(cmd *cobra.Command, args []string) error {
	return runServerRefactoredWithContext(cmd.Context(), cmd, args)
}

func runServerRefactoredWithContext(ctx context.Context, cmd *cobra.Command, args []string) error {
	if ctx == nil {
		ctx = context.Background()
	}

	configManager := common.NewConfigManager()

	cfg, err := configManager.LoadAndValidateConfig(configPath)
	if err != nil {
		return err
	}

	configManager.OverridePortIfSpecified(cfg, port, DefaultServerPort)

	gw, err := gateway.NewGateway(cfg)
	if err != nil {
		return fmt.Errorf("failed to create gateway: %w", err)
	}

	lifecycleManager := common.NewServerLifecycleManager(30 * time.Second)

	httpConfig := common.HTTPServerConfig{
		Port:    cfg.Port,
		Handler: createHTTPHandler(gw),
	}

	gatewayConfig := common.ServiceConfig{
		StartFunc: func() error { return gw.Start(ctx) },
		StopFunc:  func() error { return gw.Stop() },
		Name:      "LSP Gateway",
	}

	gatewayCtx, gatewayCancel := context.WithCancel(ctx)
	defer gatewayCancel()

	go func() {
		if err := lifecycleManager.RunService(gatewayCtx, gatewayConfig); err != nil {
			log.Printf("Gateway service error: %v", err)
		}
	}()

	log.Printf("Starting LSP Gateway server on port %d", cfg.Port)

	return lifecycleManager.RunHTTPServer(ctx, httpConfig)
}

func createHTTPHandler(gw *gateway.Gateway) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/jsonrpc", gw.HandleJSONRPC)
	return mux
}
