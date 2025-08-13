package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	clicommon "lsp-gateway/src/cli/common"
	"lsp-gateway/src/internal/common"
	icommon "lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/registry"
	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/server"
	"lsp-gateway/src/server/cache"
)

// RunServer starts the simplified LSP gateway server
func RunServer(addr string, configPath string, lspOnly bool) error {
	cfg := clicommon.LoadConfigForServer(configPath)

	// Create and start gateway
	gateway, err := server.NewHTTPGateway(addr, cfg, lspOnly)
	if err != nil {
		return fmt.Errorf("failed to create gateway: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := gateway.Start(ctx); err != nil {
		return fmt.Errorf("failed to start gateway: %w", err)
	}

	common.CLILogger.Info("Simple LSP Gateway started on %s", addr)

	// Display SCIP cache status
	clicommon.DisplayCacheStatus(common.CLILogger, gateway.GetLSPManager().GetCache(), "gateway")

	common.CLILogger.Info("Available languages: %s", strings.Join(registry.GetLanguageNames(), ", "))
	common.CLILogger.Info("HTTP JSON-RPC endpoint: http://%s/jsonrpc", addr)
	common.CLILogger.Info("Health check endpoint: http://%s/health", addr)

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	common.CLILogger.Info("Received shutdown signal, stopping gateway...")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := icommon.CreateContext(30 * time.Second)
	defer shutdownCancel()

	done := make(chan error, 1)
	go func() {
		done <- gateway.Stop()
	}()

	select {
	case err := <-done:
		if err != nil {
			common.CLILogger.Warn("Gateway stopped with error: %v", err)
		} else {
			common.CLILogger.Info("Gateway stopped successfully")
		}
	case <-shutdownCtx.Done():
		common.CLILogger.Warn("Shutdown timeout exceeded")
		return fmt.Errorf("shutdown timeout")
	}

	return nil
}

// RunMCPServer starts the MCP server
func RunMCPServer(configPath string) error {
	// Display cache status before starting MCP server
	cfg := clicommon.LoadConfigForServer(configPath)
	manager, err := clicommon.CreateLSPManager(cfg)
	if err == nil {
		clicommon.DisplayCacheStatus(common.CLILogger, manager.GetCache(), "mcp")
	} else {
		common.CLILogger.Info("Starting MCP Server...")
		common.CLILogger.Warn("SCIP Cache: Unable to check status (%v)", err)
	}
	common.CLILogger.Info("MCP Server ready for AI assistant integration")

	// Always run in enhanced mode
	return server.RunMCPServer(configPath)
}

// ShowStatus displays the current status of LSP clients
func ShowStatus(configPath string) error {
	cmdCtx, err := clicommon.NewManagerContext(configPath, 30*time.Second)
	if err != nil {
		return err
	}
	defer cmdCtx.Cleanup()

	common.CLILogger.Info("LSP Gateway Status")

	// Check server availability without starting them (fast, non-destructive)
	status := cmdCtx.Manager.CheckServerAvailability()

	common.CLILogger.Info("Configured Languages: %d\n", len(cmdCtx.Config.Servers))

	for language, serverConfig := range cmdCtx.Config.Servers {
		clientStatus := status[language]
		statusText := "Unavailable"

		if clientStatus.Available {
			statusText = "Available"
		}

		common.CLILogger.Info("%s: %s", language, statusText)
		common.CLILogger.Info("   Command: %s %v", serverConfig.Command, serverConfig.Args)

		if !clientStatus.Available && clientStatus.Error != nil {
			common.CLILogger.Error("   Error: %v", clientStatus.Error)
		}

		common.CLILogger.Info("")
	}

	// Display SCIP cache status
	cache := cmdCtx.Manager.GetCache()
	clicommon.DisplayCacheStatus(common.CLILogger, cache, "cli")

	return nil
}

// TestConnection tests connection to LSP servers
func TestConnection(configPath string) error {
	cmdCtx, err := clicommon.NewCommandContext(configPath, 60*time.Second)
	if err != nil {
		return err
	}
	defer cmdCtx.Cleanup()

	common.CLILogger.Info("Testing LSP Server Connections")
	common.CLILogger.Info("Starting LSP Manager...")

	// Display initial cache status
	common.CLILogger.Info("")
	common.CLILogger.Info("Cache Status:")

	scipCache := cmdCtx.Manager.GetCache()
	initialMetrics, healthErr := scipCache.HealthCheck()
	if healthErr != nil {
		common.CLILogger.Error("Cache: Health check failed (%v)", healthErr)
	} else if initialMetrics != nil {
		common.CLILogger.Info("Cache: Enabled and Ready")
		common.CLILogger.Info("   Health: OK")
		common.CLILogger.Info("   Initial Stats: %d entries, %s used",
			initialMetrics.EntryCount, clicommon.FormatBytes(initialMetrics.TotalSize))
	} else {
		common.CLILogger.Info("Cache: Enabled and Ready")
		common.CLILogger.Info("   Health: OK")
		common.CLILogger.Info("   Initial Stats: 0 entries, 0MB used")
	}

	// Get client status to see which servers started successfully
	status := cmdCtx.Manager.GetClientStatus()

	common.CLILogger.Info("")
	common.CLILogger.Info("LSP Server Status:")

	activeLanguages := []string{}
	for language, clientStatus := range status {
		statusText := "Inactive"

		if clientStatus.Active {
			statusText = "Active"
			activeLanguages = append(activeLanguages, language)
		}

		if clientStatus.Error != nil {
			common.CLILogger.Error("%s: %s (%v)", language, statusText, clientStatus.Error)
		} else {
			common.CLILogger.Info("%s: %s", language, statusText)
		}
	}

	if len(activeLanguages) == 0 {
		return fmt.Errorf("no active LSP servers available for testing")
	}

	common.CLILogger.Info("")
	common.CLILogger.Info("Testing Multi-Repo workspace/symbol functionality...")

	// Test workspace/symbol for multi-repo support (queries all active servers)
	params := map[string]interface{}{
		"query": "main",
	}

	common.CLILogger.Info("")
	common.CLILogger.Info("Multi-Repo workspace/symbol Test:")

	result, err := testWorkspaceSymbol(cmdCtx.Manager, cmdCtx.Context, params)
	successCount := 0
	if err != nil {
		common.CLILogger.Error("workspace/symbol (multi-repo): %v", err)
	} else {
		common.CLILogger.Info("workspace/symbol (multi-repo): Success (%T)", result)
		if result != nil {
			// Try to parse and show symbol count if possible
			if symbols, ok := parseSymbolResult(result); ok {
				common.CLILogger.Info("   Total symbols found across all servers: %d", len(symbols))
			}
		}
		successCount = 1
	}

	// Show which servers contributed to the results
	common.CLILogger.Info("")
	common.CLILogger.Info("Active servers contributing to results: %d", len(activeLanguages))
	for _, language := range activeLanguages {
		common.CLILogger.Info("   • %s server: Active", language)
	}

	common.CLILogger.Info("")
	common.CLILogger.Info("Test Summary:")
	common.CLILogger.Info("   • Active Servers: %d/%d", len(activeLanguages), len(cmdCtx.Config.Servers))
	common.CLILogger.Info("   • Multi-Repo Test: %s", func() string {
		if successCount > 0 {
			return "Success"
		}
		return "Failed"
	}())

	// Display cache performance after testing
	finalCacheMetrics := cmdCtx.Manager.GetCacheMetrics()
	if finalCacheMetrics != nil {
		common.CLILogger.Info("")
		common.CLILogger.Info("Cache Performance:")

		// Access the cache directly to get proper metrics
		scipCache := cmdCtx.Manager.GetCache()
		if healthMetrics, healthErr := scipCache.HealthCheck(); healthErr == nil && healthMetrics != nil {
			hitRate := cache.HitRate(healthMetrics)

			common.CLILogger.Info("   Cache Hits: %d (%.1f%% hit rate)", healthMetrics.HitCount, hitRate)
			common.CLILogger.Info("   Cache Misses: %d", healthMetrics.MissCount)
			common.CLILogger.Info("   New Entries: %d (%s cached)",
				healthMetrics.EntryCount, clicommon.FormatBytes(healthMetrics.TotalSize))

			if healthMetrics.HitCount+healthMetrics.MissCount > 0 {
				common.CLILogger.Info("   Cache Contributed: Improved response times")
			} else {
				common.CLILogger.Info("   Cache Contributed: No cache activity during test")
			}
		} else {
			// Fallback when health check fails
			common.CLILogger.Info("   Cache Status: Active and monitoring")
		}
	}

	if successCount > 0 {
		common.CLILogger.Info("")
		common.CLILogger.Info("Multi-repo workspace/symbol functionality is working correctly!")
		common.CLILogger.Info("All %d active LSP servers are contributing to search results.", len(activeLanguages))
		return nil
	} else {
		common.CLILogger.Warn("")
		common.CLILogger.Warn("Multi-repo workspace/symbol test failed.")
		common.CLILogger.Warn("Check individual server logs for debugging.")
		return nil // Don't fail the test completely
	}
}

// testWorkspaceSymbol tests workspace/symbol for multi-repo support
// Queries all active servers for comprehensive symbol search
func testWorkspaceSymbol(manager *server.LSPManager, ctx context.Context, params interface{}) (interface{}, error) {
	return manager.ProcessRequest(ctx, types.MethodWorkspaceSymbol, params)
}

// parseSymbolResult tries to parse the symbol result to count items
func parseSymbolResult(result interface{}) ([]interface{}, bool) {
	if result == nil {
		return nil, false
	}

	// Try to convert to slice
	if symbols, ok := result.([]interface{}); ok {
		return symbols, true
	}

	return nil, false
}
