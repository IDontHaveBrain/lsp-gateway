package cache

import (
	"context"
	"lsp-gateway/src/config"
	"lsp-gateway/src/internal/common"
)

// CacheIntegrator provides unified cache management with graceful degradation
type CacheIntegrator struct {
	cache   SCIPCache
	config  *config.CacheConfig
	logger  *common.SafeLogger
	enabled bool
}

// NewCacheIntegrator creates a new cache integrator with graceful degradation
func NewCacheIntegrator(cfg *config.Config, logger *common.SafeLogger) *CacheIntegrator {
	integrator := &CacheIntegrator{
		cache:   nil,
		config:  nil,
		logger:  logger,
		enabled: false,
	}

	// Try to create cache with unified config - graceful degradation if it fails
	if cfg != nil && cfg.Cache != nil && cfg.Cache.Enabled {
		scipCache, err := NewSCIPCacheManager(cfg.Cache)
		if err != nil {
			logger.Warn("Failed to create cache (continuing without cache): %v", err)
		} else {
			integrator.cache = scipCache
			integrator.config = cfg.Cache
			integrator.enabled = true
		}
	}

	return integrator
}

// StartCache starts the cache if enabled - graceful degradation on failure
func (ci *CacheIntegrator) StartCache(ctx context.Context) error {
	if !ci.enabled || ci.cache == nil {
		return nil
	}

	ci.logger.Info("Starting SCIP cache")
	if err := ci.cache.Start(ctx); err != nil {
		ci.logger.Warn("Failed to start SCIP cache (continuing without cache): %v", err)
		ci.cache = nil // Disable cache on start failure
		ci.enabled = false
		return nil
	}

	ci.logger.Info("SCIP cache started successfully")
	return nil
}

// StopCache stops the cache if enabled
func (ci *CacheIntegrator) StopCache() error {
	if !ci.enabled || ci.cache == nil {
		return nil
	}

	return ci.cache.Stop()
}

// GetCache returns the cache instance or nil if not enabled
func (ci *CacheIntegrator) GetCache() SCIPCache {
	if !ci.enabled {
		return nil
	}
	return ci.cache
}

// IsEnabled returns true if cache is enabled and available
func (ci *CacheIntegrator) IsEnabled() bool {
	return ci.enabled && ci.cache != nil
}
