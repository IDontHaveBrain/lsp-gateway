package cli

import (
	"log"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/server"
)

// displayGatewayCacheStatus displays SCIP cache status for HTTP Gateway startup
func displayGatewayCacheStatus(gateway *server.HTTPGateway) {
	manager := gateway.GetLSPManager()
	if manager == nil {
		log.Printf("⚠️  SCIP Cache: Manager not available")
		return
	}

	cache := manager.GetCache()
	if cache == nil {
		log.Printf("⚠️  SCIP Cache: Not available")
		return
	}

	metrics, healthErr := cache.HealthCheck()
	if healthErr != nil {
		log.Printf("❌ SCIP Cache: Health check failed (%v)", healthErr)
		return
	}

	if metrics != nil {
		sizeMB := float64(metrics.TotalSize) / (1024 * 1024)
		log.Printf("🗄️  SCIP Cache: ✅ Initialized and Ready (100MB limit, 30min TTL)")
		log.Printf("   Health: %s, Stats: %d entries, %.1fMB used", metrics.HealthStatus, metrics.EntryCount, sizeMB)
	} else {
		log.Printf("🗄️  SCIP Cache: ✅ Initialized and Ready")
		log.Printf("   Health: OK, Stats: 0 entries, 0MB used")
	}
}

// displayMCPCacheStatus displays SCIP cache status for MCP server startup
func displayMCPCacheStatus(configPath string) {
	cfg := LoadConfigWithFallback(configPath)

	manager, err := server.NewLSPManager(cfg)
	if err != nil {
		common.LSPLogger.Info("🚀 Starting MCP Server...")
		common.LSPLogger.Warn("⚠️  SCIP Cache: Unable to check status (%v)", err)
		return
	}

	cache := manager.GetCache()
	if cache == nil {
		common.LSPLogger.Info("🚀 Starting MCP Server...")
		common.LSPLogger.Warn("⚠️  SCIP Cache: Not available")
		return
	}

	metrics, healthErr := cache.HealthCheck()

	common.LSPLogger.Info("🚀 Starting MCP Server...")
	if healthErr != nil {
		common.LSPLogger.Error("❌ SCIP Cache: Health check failed (%v)", healthErr)
		return
	}

	if metrics != nil {
		sizeMB := float64(metrics.TotalSize) / (1024 * 1024)
		common.LSPLogger.Info("🗄️  SCIP Cache: ✅ Initialized and Ready")
		common.LSPLogger.Info("   Health: %s, Stats: %d entries, %.1fMB used", metrics.HealthStatus, metrics.EntryCount, sizeMB)
	} else {
		common.LSPLogger.Info("🗄️  SCIP Cache: ✅ Initialized and Ready")
		common.LSPLogger.Info("   Health: OK, Stats: 0 entries, 0MB used")
	}

	common.LSPLogger.Info("📡 MCP Server ready for AI assistant integration")
}

// Note: Cache display functions are implemented in cache_utils.go to avoid duplication
