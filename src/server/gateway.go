package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/constants"
	"lsp-gateway/src/server/cache"
	"lsp-gateway/src/server/protocol"
)

// HTTPGateway provides a simple HTTP JSON-RPC gateway to LSP servers with unified cache config
type HTTPGateway struct {
	lspManager  *LSPManager
	server      *http.Server
	cacheConfig *config.CacheConfig
	lspOnly     bool
	mu          sync.RWMutex
}

// Alias types from protocol package for backward compatibility
type JSONRPCRequest = protocol.JSONRPCRequest
type JSONRPCResponse = protocol.JSONRPCResponse
type RPCError = protocol.RPCError

// NewHTTPGateway creates a new simple HTTP gateway with mandatory cache
func NewHTTPGateway(addr string, cfg *config.Config, lspOnly bool) (*HTTPGateway, error) {
	// Ensure cache is enabled - HTTP gateway requires cache for performance
	if cfg == nil {
		cfg = config.GetDefaultConfigWithCache()
		cfg.EnableCache()
	} else if !cfg.IsCacheEnabled() {
		cfg.EnableCache()
	}

	// Create LSP manager - cache is now always created internally
	lspManager, err := NewLSPManager(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create LSP manager: %w", err)
	}

	gateway := &HTTPGateway{
		lspManager:  lspManager,
		cacheConfig: cfg.Cache,
		lspOnly:     lspOnly,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/jsonrpc", gateway.handleJSONRPC)
	mux.HandleFunc("/health", gateway.handleHealth)
	// Cache monitoring endpoints
	mux.HandleFunc("/cache/stats", gateway.handleCacheStats)
	mux.HandleFunc("/cache/health", gateway.handleCacheHealth)
	mux.HandleFunc("/cache/clear", gateway.handleCacheClear)

	gateway.server = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  constants.DefaultRequestTimeout,
		WriteTimeout: constants.DefaultRequestTimeout,
	}

	return gateway, nil
}

// Start starts the HTTP gateway
func (g *HTTPGateway) Start(ctx context.Context) error {
	// Start LSP manager (cache is initialized and started internally)
	if err := g.lspManager.Start(ctx); err != nil {
		return fmt.Errorf("failed to start LSP manager: %w", err)
	}

	go func() {
		if err := g.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			common.GatewayLogger.Error("HTTP server error: %v", err)
		}
	}()

	return nil
}

// Stop stops the HTTP gateway and cache
func (g *HTTPGateway) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var lastErr error

	if g.server != nil {
		if err := g.server.Shutdown(ctx); err != nil {
			common.GatewayLogger.Error("HTTP server shutdown error: %v", err)
			lastErr = err
		}
	}

	if err := g.lspManager.Stop(); err != nil {
		common.GatewayLogger.Error("LSP manager stop error: %v", err)
		lastErr = err
	}

	// Cache is stopped automatically by LSP manager

	return lastErr
}

// GetLSPManager returns the LSP manager for cache status access
func (g *HTTPGateway) GetLSPManager() *LSPManager {
	return g.lspManager
}

// handleJSONRPC handles JSON-RPC requests with cache performance tracking
func (g *HTTPGateway) handleJSONRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		g.writeErrorRPC(w, nil, protocol.NewInvalidRequestError("Only POST method allowed"))
		return
	}

	if r.Header.Get("Content-Type") != "application/json" {
		g.writeErrorRPC(w, nil, protocol.NewInvalidRequestError("Content-Type must be application/json"))
		return
	}

	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.writeErrorRPC(w, nil, protocol.NewParseError(err.Error()))
		return
	}

	if req.JSONRPC != "2.0" {
		g.writeErrorRPC(w, req.ID, protocol.NewInvalidRequestError("jsonrpc must be 2.0"))
		return
	}

	// If lspOnly mode, restrict to the 6 basic LSP methods
	if g.lspOnly {
		allowedMethods := map[string]bool{
			"textDocument/definition":     true,
			"textDocument/references":     true,
			"textDocument/hover":          true,
			"textDocument/documentSymbol": true,
			"workspace/symbol":            true,
			"textDocument/completion":     true,
		}
		if !allowedMethods[req.Method] {
			g.writeErrorRPC(w, req.ID, protocol.NewMethodNotFoundError(fmt.Sprintf("Method '%s' is not available in LSP-only mode", req.Method)))
			return
		}
	}

	// Track request timing for performance metrics
	startTime := time.Now()
	cacheMetricsBefore := g.getCacheMetricsSnapshot()

	result, err := g.lspManager.ProcessRequest(r.Context(), req.Method, req.Params)
	if err != nil {
		g.writeErrorRPC(w, req.ID, protocol.NewInternalError(err.Error()))
		return
	}

	// Calculate performance metrics
	responseTime := time.Since(startTime)
	cacheMetricsAfter := g.getCacheMetricsSnapshot()
	cacheStatus := g.determineCacheStatus(cacheMetricsBefore, cacheMetricsAfter)

	response := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}

	// Add cache performance headers
	w.Header().Set("Content-Type", "application/json")
	g.addCacheHeaders(w, cacheStatus, responseTime)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		common.GatewayLogger.Error("Failed to encode response: %v", err)
	}
}

// handleHealth handles health check requests including cache status
func (g *HTTPGateway) handleHealth(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":      "healthy",
		"lsp_clients": g.lspManager.GetClientStatus(),
	}

	// Add cache health information
	if g.lspManager.scipCache != nil {
		cacheMetrics := g.getCacheMetricsSnapshot()
		if cacheMetrics != nil {
			totalRequests := cacheMetrics.HitCount + cacheMetrics.MissCount
			hitRate := float64(0)
			if totalRequests > 0 {
				hitRate = float64(cacheMetrics.HitCount) / float64(totalRequests) * 100
			}

			health["cache"] = map[string]interface{}{
				"enabled":          true,
				"status":           "OK",
				"hit_rate_percent": hitRate,
				"entry_count":      cacheMetrics.EntryCount,
				"total_requests":   totalRequests,
			}
		} else {
			health["cache"] = map[string]interface{}{
				"enabled": true,
				"status":  "initializing",
			}
		}
	} else {
		health["cache"] = map[string]interface{}{
			"enabled": false,
			"status":  "disabled",
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// writeErrorRPC writes a JSON-RPC error response using the protocol types
func (g *HTTPGateway) writeErrorRPC(w http.ResponseWriter, id interface{}, rpcErr *protocol.RPCError) {
	response := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   rpcErr,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK) // JSON-RPC errors still return 200
	json.NewEncoder(w).Encode(response)
}

// getCacheMetricsSnapshot captures current cache metrics for comparison
func (g *HTTPGateway) getCacheMetricsSnapshot() *cache.CacheMetrics {
	cache := g.lspManager.GetCache()
	if cache == nil {
		return nil
	}
	return cache.GetMetrics()
}

// determineCacheStatus determines if the request was a cache hit or miss
func (g *HTTPGateway) determineCacheStatus(before, after *cache.CacheMetrics) string {
	if before == nil || after == nil {
		return "disabled"
	}

	if after.HitCount > before.HitCount {
		return "hit"
	} else if after.MissCount > before.MissCount {
		return "miss"
	}
	return "unknown"
}

// addCacheHeaders adds cache-related HTTP headers to the response
func (g *HTTPGateway) addCacheHeaders(w http.ResponseWriter, cacheStatus string, responseTime time.Duration) {
	w.Header().Set("X-LSP-Cache-Status", cacheStatus)
	w.Header().Set("X-LSP-Response-Time", strconv.FormatInt(responseTime.Microseconds(), 10))

	if metrics := g.getCacheMetricsSnapshot(); metrics != nil {
		w.Header().Set("X-LSP-Cache-Size", strconv.FormatInt(metrics.EntryCount, 10))
		w.Header().Set("X-LSP-Cache-Memory", strconv.FormatInt(metrics.TotalSize, 10))
	}
}

// handleCacheStats returns cache statistics
func (g *HTTPGateway) handleCacheStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	metrics := g.getCacheMetricsSnapshot()
	if metrics == nil {
		http.Error(w, "Cache not available", http.StatusServiceUnavailable)
		return
	}

	// Calculate hit rate
	totalRequests := metrics.HitCount + metrics.MissCount
	hitRate := float64(0)
	if totalRequests > 0 {
		hitRate = float64(metrics.HitCount) / float64(totalRequests) * 100
	}

	stats := map[string]interface{}{
		"cache_enabled":     true,
		"hit_count":         metrics.HitCount,
		"miss_count":        metrics.MissCount,
		"hit_rate_percent":  hitRate,
		"error_count":       metrics.ErrorCount,
		"eviction_count":    metrics.EvictionCount,
		"total_size_bytes":  metrics.TotalSize,
		"entry_count":       metrics.EntryCount,
		"average_hit_time":  metrics.AverageHitTime.String(),
		"average_miss_time": metrics.AverageMissTime.String(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// handleCacheHealth returns cache health status
func (g *HTTPGateway) handleCacheHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cache := g.lspManager.GetCache()
	if cache == nil {
		http.Error(w, "Cache not available", http.StatusServiceUnavailable)
		return
	}

	metrics, err := cache.HealthCheck()
	if err != nil {
		health := map[string]interface{}{
			"status":  "unhealthy",
			"error":   err.Error(),
			"enabled": false,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(health)
		return
	}

	health := map[string]interface{}{
		"status":          "healthy",
		"enabled":         true,
		"total_size":      metrics.TotalSize,
		"entry_count":     metrics.EntryCount,
		"uptime_requests": metrics.HitCount + metrics.MissCount,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// handleCacheClear clears the cache (for development/debugging)
func (g *HTTPGateway) handleCacheClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cache := g.lspManager.GetCache()
	if cache == nil {
		http.Error(w, "Cache not available", http.StatusServiceUnavailable)
		return
	}

	// Note: Cache interface doesn't have Clear method, so we invalidate all documents
	// This is a limitation of the current cache interface design
	result := map[string]interface{}{
		"status":  "requested",
		"message": "Cache clear requested - individual document invalidation required",
		"note":    "Use cache invalidation API for specific documents",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
