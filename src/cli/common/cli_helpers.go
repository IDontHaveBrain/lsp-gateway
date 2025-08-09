package common

import (
	"fmt"
	"os"

	"lsp-gateway/src/config"
	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/server"
	"lsp-gateway/src/server/cache"
	"lsp-gateway/src/utils/configloader"
)

// LoadConfigAndManager loads configuration and creates an LSP manager with proper cache path setup
func LoadConfigAndManager(configPath string) (*config.Config, *server.LSPManager, error) {
	cfg := LoadConfigForCLI(configPath)

	manager, err := CreateLSPManager(cfg)
	if err != nil {
		return cfg, nil, fmt.Errorf("failed to create LSP manager: %w", err)
	}

	return cfg, manager, nil
}

// LoadConfigForCLI loads configuration for CLI commands with project-specific cache path setup
func LoadConfigForCLI(configPath string) *config.Config {
	cfg := configloader.LoadForCLI(configPath)

	// Ensure cache path is project-specific so it matches CLI indexing
	setupProjectSpecificCachePath(cfg)

	return cfg
}

// LoadConfigForServer loads configuration for server operations with project-specific cache path setup
func LoadConfigForServer(configPath string) *config.Config {
	cfg := configloader.LoadForServer(configPath)

	// Ensure cache path is project-specific so it matches CLI indexing
	setupProjectSpecificCachePath(cfg)

	return cfg
}

// CreateLSPManager creates an LSP manager from configuration
func CreateLSPManager(cfg *config.Config) (*server.LSPManager, error) {
	manager, err := server.NewLSPManager(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create LSP manager: %w", err)
	}

	return manager, nil
}

// CheckCacheHealth performs standardized cache health check with error logging
func CheckCacheHealth(cacheInstance cache.SCIPCache) (*cache.CacheMetrics, error) {
	if cacheInstance == nil {
		common.CLILogger.Info("Cache: Not available")
		return nil, fmt.Errorf("cache is not available")
	}

	metrics, err := cacheInstance.HealthCheck()
	if err != nil {
		common.CLILogger.Error("Cache: Unable to get status (%v)", err)
		return nil, fmt.Errorf("cache health check failed: %w", err)
	}

	if metrics == nil {
		common.CLILogger.Info("Cache: Disabled by configuration")
		return nil, fmt.Errorf("cache is disabled by configuration")
	}

	return metrics, nil
}

// setupProjectSpecificCachePath ensures cache path is project-specific
func setupProjectSpecificCachePath(cfg *config.Config) {
	if cfg != nil && cfg.Cache != nil {
		if wd, err := os.Getwd(); err == nil {
			projectPath := config.GetProjectSpecificCachePath(wd)
			cfg.SetCacheStoragePath(projectPath)
		}
	}
}
