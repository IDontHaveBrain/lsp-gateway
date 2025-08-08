package cli

import (
	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/server"
	"lsp-gateway/src/utils/configloader"
)

// displayGatewayCacheStatus displays SCIP cache status for HTTP Gateway startup
func displayGatewayCacheStatus(gateway *server.HTTPGateway) {
	manager := gateway.GetLSPManager()
	if manager == nil {
		common.CLILogger.Warn("SCIP Cache: Manager not available")
		return
	}

	cache := manager.GetCache()
	if cache == nil {
		common.CLILogger.Warn("SCIP Cache: Not available")
		return
	}

	metrics, healthErr := cache.HealthCheck()
	if healthErr != nil {
		common.CLILogger.Error("SCIP Cache: Health check failed (%v)", healthErr)
		return
	}

	if metrics != nil {
		sizeMB := float64(metrics.TotalSize) / (1024 * 1024)
		common.CLILogger.Info("SCIP Cache: Initialized and Ready (100MB limit, 30min TTL)")
		common.CLILogger.Info("Health: OK, Stats: %d entries, %.1fMB used", metrics.EntryCount, sizeMB)
	} else {
		common.CLILogger.Info("SCIP Cache: Initialized and Ready")
		common.CLILogger.Info("Health: OK, Stats: 0 entries, 0MB used")
	}
}

// displayMCPCacheStatus displays SCIP cache status for MCP server startup
func displayMCPCacheStatus(configPath string) {
	cfg := configloader.LoadForCLI(configPath)

	manager, err := server.NewLSPManager(cfg)
	if err != nil {
		common.CLILogger.Info("🚀 Starting MCP Server...")
		common.CLILogger.Warn("⚠️  SCIP Cache: Unable to check status (%v)", err)
		return
	}

	cache := manager.GetCache()
	if cache == nil {
		common.CLILogger.Info("🚀 Starting MCP Server...")
		common.CLILogger.Warn("⚠️  SCIP Cache: Not available")
		return
	}

	metrics, healthErr := cache.HealthCheck()

	common.CLILogger.Info("🚀 Starting MCP Server...")
	if healthErr != nil {
		common.CLILogger.Error("❌ SCIP Cache: Health check failed (%v)", healthErr)
		return
	}

	if metrics != nil {
		sizeMB := float64(metrics.TotalSize) / (1024 * 1024)
		common.CLILogger.Info("🗄️  SCIP Cache: ✅ Initialized and Ready")
		common.CLILogger.Info("   Health: OK, Stats: %d entries, %.1fMB used", metrics.EntryCount, sizeMB)
	} else {
		common.CLILogger.Info("🗄️  SCIP Cache: ✅ Initialized and Ready")
		common.CLILogger.Info("   Health: OK, Stats: 0 entries, 0MB used")
	}

	common.CLILogger.Info("📡 MCP Server ready for AI assistant integration")
}

// Note: Cache display functions are implemented in cache_utils.go to avoid duplication
