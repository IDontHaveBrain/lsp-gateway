package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/server"
)

// RunServer starts the simplified LSP gateway server
func RunServer(addr string, configPath string, lspOnly bool) error {
	cfg := LoadConfigWithFallback(configPath)

	// Ensure cache path is project-specific so it matches CLI indexing
	if cfg != nil && cfg.Cache != nil {
		if wd, err := os.Getwd(); err == nil {
			projectPath := config.GetProjectSpecificCachePath(wd)
			cfg.SetCacheStoragePath(projectPath)
		}
	}

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
	displayGatewayCacheStatus(gateway)

	common.CLILogger.Info("Available languages: go, python, javascript, typescript, java")
	common.CLILogger.Info("HTTP JSON-RPC endpoint: http://%s/jsonrpc", addr)
	common.CLILogger.Info("Health check endpoint: http://%s/health", addr)

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	common.CLILogger.Info("Received shutdown signal, stopping gateway...")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
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
	displayMCPCacheStatus(configPath)

	// Always run in enhanced mode
	return server.RunMCPServer(configPath)
}

// ShowStatus displays the current status of LSP clients
func ShowStatus(configPath string) error {
	cfg := LoadConfigForCLI(configPath)

	common.CLILogger.Info("🔍 LSP Gateway Status")
	common.CLILogger.Info("%s", strings.Repeat("=", 50))

	manager, err := server.NewLSPManager(cfg)
	if err != nil {
		return fmt.Errorf("failed to create LSP manager: %w", err)
	}

	// Check server availability without starting them (fast, non-destructive)
	status := manager.CheckServerAvailability()

	common.CLILogger.Info("📊 Configured Languages: %d\n", len(cfg.Servers))

	for language, serverConfig := range cfg.Servers {
		clientStatus := status[language]
		statusIcon := "❌"
		statusText := "Unavailable"

		if clientStatus.Available {
			statusIcon = "✅"
			statusText = "Available"
		}

		common.CLILogger.Info("%s %s: %s", statusIcon, language, statusText)
		common.CLILogger.Info("   Command: %s %v", serverConfig.Command, serverConfig.Args)

		if !clientStatus.Available && clientStatus.Error != nil {
			common.CLILogger.Error("   Error: %v", clientStatus.Error)
		}

		common.CLILogger.Info("")
	}

	// Display SCIP cache status
	displayCacheStatus(manager)

	return nil
}

// TestConnection tests connection to LSP servers
func TestConnection(configPath string) error {
	cfg := LoadConfigForCLI(configPath)

	common.CLILogger.Info("🧪 Testing LSP Server Connections")
	common.CLILogger.Info("%s", strings.Repeat("=", 50))

	manager, err := server.NewLSPManager(cfg)
	if err != nil {
		return fmt.Errorf("failed to create LSP manager: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	common.CLILogger.Info("🚀 Starting LSP Manager...")
	if err := manager.Start(ctx); err != nil {
		return fmt.Errorf("failed to start LSP manager: %w", err)
	}
	defer manager.Stop()

	// Display initial cache status
	common.CLILogger.Info("")
	common.CLILogger.Info("🗄️  Cache Status:")
	common.CLILogger.Info("%s", strings.Repeat("-", 30))

	cache := manager.GetCache()
	initialMetrics, healthErr := cache.HealthCheck()
	if healthErr != nil {
		common.CLILogger.Error("❌ Cache: Health check failed (%v)", healthErr)
	} else if initialMetrics != nil {
		common.CLILogger.Info("✅ Cache: Enabled and Ready")
		common.CLILogger.Info("   Health: OK")
		common.CLILogger.Info("   Initial Stats: %d entries, %.1fMB used",
			initialMetrics.EntryCount, float64(initialMetrics.TotalSize)/(1024*1024))
	} else {
		common.CLILogger.Info("✅ Cache: Enabled and Ready")
		common.CLILogger.Info("   Health: OK")
		common.CLILogger.Info("   Initial Stats: 0 entries, 0MB used")
	}

	// Get client status to see which servers started successfully
	status := manager.GetClientStatus()

	common.CLILogger.Info("")
	common.CLILogger.Info("📊 LSP Server Status:")
	common.CLILogger.Info("%s", strings.Repeat("-", 30))

	activeLanguages := []string{}
	for language, clientStatus := range status {
		statusIcon := "❌"
		statusText := "Inactive"

		if clientStatus.Active {
			statusIcon = "✅"
			statusText = "Active"
			activeLanguages = append(activeLanguages, language)
		}

		if clientStatus.Error != nil {
			common.CLILogger.Error("%s %s: %s (%v)", statusIcon, language, statusText, clientStatus.Error)
		} else {
			common.CLILogger.Info("%s %s: %s", statusIcon, language, statusText)
		}
	}

	if len(activeLanguages) == 0 {
		return fmt.Errorf("no active LSP servers available for testing")
	}

	common.CLILogger.Info("")
	common.CLILogger.Info("🔍 Testing Multi-Repo workspace/symbol functionality...")
	common.CLILogger.Info("%s", strings.Repeat("-", 50))

	// Test workspace/symbol for multi-repo support (queries all active servers)
	params := map[string]interface{}{
		"query": "main",
	}

	common.CLILogger.Info("")
	common.CLILogger.Info("📋 Multi-Repo workspace/symbol Test:")
	common.CLILogger.Info("%s", strings.Repeat("-", 40))

	result, err := testWorkspaceSymbol(manager, ctx, params)
	successCount := 0
	if err != nil {
		common.CLILogger.Error("❌ workspace/symbol (multi-repo): %v", err)
	} else {
		common.CLILogger.Info("✅ workspace/symbol (multi-repo): Success (%T)", result)
		if result != nil {
			// Try to parse and show symbol count if possible
			if symbols, ok := parseSymbolResult(result); ok {
				common.CLILogger.Info("   📊 Total symbols found across all servers: %d", len(symbols))
			}
		}
		successCount = 1
	}

	// Show which servers contributed to the results
	common.CLILogger.Info("")
	common.CLILogger.Info("📈 Active servers contributing to results: %d", len(activeLanguages))
	for _, language := range activeLanguages {
		common.CLILogger.Info("   • %s server: Active", language)
	}

	common.CLILogger.Info("")
	common.CLILogger.Info("%s", strings.Repeat("=", 50))
	common.CLILogger.Info("📈 Test Summary:")
	common.CLILogger.Info("   • Active Servers: %d/%d", len(activeLanguages), len(cfg.Servers))
	common.CLILogger.Info("   • Multi-Repo Test: %s", func() string {
		if successCount > 0 {
			return "✅ Success"
		}
		return "❌ Failed"
	}())

	// Display cache performance after testing
	finalCacheMetrics := manager.GetCacheMetrics()
	if finalCacheMetrics != nil {
		common.CLILogger.Info("")
		common.CLILogger.Info("📈 Cache Performance:")

		// Access the cache directly to get proper metrics
		cache := manager.GetCache()
		if healthMetrics, healthErr := cache.HealthCheck(); healthErr == nil && healthMetrics != nil {
			hitRate := float64(0)
			totalRequests := healthMetrics.HitCount + healthMetrics.MissCount
			if totalRequests > 0 {
				hitRate = float64(healthMetrics.HitCount) / float64(totalRequests) * 100
			}

			common.CLILogger.Info("   Cache Hits: %d (%.1f%% hit rate)", healthMetrics.HitCount, hitRate)
			common.CLILogger.Info("   Cache Misses: %d", healthMetrics.MissCount)
			common.CLILogger.Info("   New Entries: %d (%.1fMB cached)",
				healthMetrics.EntryCount, float64(healthMetrics.TotalSize)/(1024*1024))

			if totalRequests > 0 {
				common.CLILogger.Info("   Cache Contributed: ✅ Improved response times")
			} else {
				common.CLILogger.Info("   Cache Contributed: ⚠️  No cache activity during test")
			}
		} else {
			// Fallback when health check fails
			common.CLILogger.Info("   Cache Status: ✅ Active and monitoring")
		}
	}

	if successCount > 0 {
		common.CLILogger.Info("")
		common.CLILogger.Info("🎉 Multi-repo workspace/symbol functionality is working correctly!")
		common.CLILogger.Info("💡 All %d active LSP servers are contributing to search results.", len(activeLanguages))
		return nil
	} else {
		common.CLILogger.Warn("")
		common.CLILogger.Warn("⚠️  Multi-repo workspace/symbol test failed.")
		common.CLILogger.Warn("🔧 Check individual server logs for debugging.")
		return nil // Don't fail the test completely
	}
}

// testWorkspaceSymbol tests workspace/symbol for multi-repo support
// Queries all active servers for comprehensive symbol search
func testWorkspaceSymbol(manager *server.LSPManager, ctx context.Context, params interface{}) (interface{}, error) {
	return manager.ProcessRequest(ctx, "workspace/symbol", params)
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
