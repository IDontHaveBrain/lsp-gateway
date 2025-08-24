package server

import (
	"context"
	"encoding/json"
	"fmt"
	"mime"
	"net"
	"net/http"
	"strconv"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/constants"
	"lsp-gateway/src/internal/registry"
	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/server/cache"
	"lsp-gateway/src/server/protocol"
)

const (
	cacheStatusDisabled = "disabled"
	cacheStatusHit      = "hit"
	cacheStatusMiss     = "miss"
	cacheStatusUnknown  = "unknown"
)

// HTTPGateway provides a simple HTTP JSON-RPC gateway to LSP servers with unified cache config
type HTTPGateway struct {
	lspManager      *LSPManager
	server          *http.Server
	listener        net.Listener
	cacheConfig     *config.CacheConfig
	lspOnly         bool
	responseFactory *protocol.ResponseFactory
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
		lspManager:      lspManager,
		cacheConfig:     cfg.Cache,
		lspOnly:         lspOnly,
		responseFactory: protocol.NewResponseFactory(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/jsonrpc", gateway.handleJSONRPC)
	mux.HandleFunc("/health", gateway.handleHealth)
	mux.HandleFunc("/languages", gateway.handleLanguages)
	// Cache monitoring endpoints
	mux.HandleFunc("/cache/stats", gateway.handleCacheStats)
	mux.HandleFunc("/cache/health", gateway.handleCacheHealth)
	mux.HandleFunc("/cache/clear", gateway.handleCacheClear)

	// Use a WriteTimeout that safely exceeds the slowest LSP request
	// Add a generous buffer to cover CI variance and server-side processing
	writeTimeout := constants.GetRequestTimeout("java") + 60*time.Second
	gateway.server = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  constants.DefaultRequestTimeout,
		WriteTimeout: writeTimeout,
		// Prevent slowloris: bound time to read headers and keep-alives
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	return gateway, nil
}

// Start starts the HTTP gateway
func (g *HTTPGateway) Start(ctx context.Context) error {
	// Start LSP manager (cache is initialized and started internally)
	if err := g.lspManager.Start(ctx); err != nil {
		return fmt.Errorf("failed to start LSP manager: %w", err)
	}
	ln, err := net.Listen("tcp", g.server.Addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", g.server.Addr, err)
	}
	g.listener = ln

	go func() {
		if err := g.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			common.GatewayLogger.Error("HTTP server error: %v", err)
		}
	}()

	return nil
}

// Stop stops the HTTP gateway and cache
func (g *HTTPGateway) Stop() error {
	ctx, cancel := common.CreateContext(30 * time.Second)
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

// Address returns the bound address of the HTTP server (host:port). If not yet started, returns configured Addr.
func (g *HTTPGateway) Address() string {
	if g.listener != nil {
		return g.listener.Addr().String()
	}
	return g.server.Addr
}

// Port returns the actual TCP port the server is listening on, or 0 if unavailable.
func (g *HTTPGateway) Port() int {
	if g.listener == nil {
		return 0
	}
	if addr, ok := g.listener.Addr().(*net.TCPAddr); ok {
		return addr.Port
	}
	return 0
}

// GetLSPManager returns the LSP manager for cache status access
func (g *HTTPGateway) GetLSPManager() *LSPManager {
	return g.lspManager
}

// handleJSONRPC handles JSON-RPC requests with cache performance tracking
func (g *HTTPGateway) handleJSONRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		g.writeResponse(w, g.responseFactory.CreateInvalidRequest(nil, "Only POST method allowed"))
		return
	}

	// Accept application/json with optional parameters (e.g., charset)
	ct := r.Header.Get("Content-Type")
	if mt, _, err := mime.ParseMediaType(ct); err != nil || mt != "application/json" {
		g.writeResponse(w, g.responseFactory.CreateInvalidRequest(nil, "Content-Type must be application/json"))
		return
	}

	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.writeResponse(w, g.responseFactory.CreateParseError(nil))
		return
	}

	if req.JSONRPC != "2.0" {
		g.writeResponse(w, g.responseFactory.CreateInvalidRequest(req.ID, "jsonrpc must be 2.0"))
		return
	}

	// If lspOnly mode, restrict to the 6 basic LSP methods
	if g.lspOnly {
		allowedMethods := map[string]bool{
			types.MethodTextDocumentDefinition:     true,
			types.MethodTextDocumentReferences:     true,
			types.MethodTextDocumentHover:          true,
			types.MethodTextDocumentDocumentSymbol: true,
			types.MethodWorkspaceSymbol:            true,
			types.MethodTextDocumentCompletion:     true,
		}
		if !allowedMethods[req.Method] {
			g.writeResponse(w, g.responseFactory.CreateMethodNotFound(req.ID, fmt.Sprintf("Method '%s' is not available in LSP-only mode", req.Method)))
			return
		}
	}

	// Track request timing for performance metrics
	startTime := time.Now()
	cacheMetricsBefore := g.getCacheMetricsSnapshot()

	result, err := g.lspManager.ProcessRequest(r.Context(), req.Method, req.Params)
	if err != nil {
		g.writeResponse(w, g.responseFactory.CreateInternalError(req.ID, err))
		return
	}

	// Calculate performance metrics
	responseTime := time.Since(startTime)
	cacheMetricsAfter := g.getCacheMetricsSnapshot()
	cacheStatus := g.determineCacheStatus(cacheMetricsBefore, cacheMetricsAfter)

	response := g.responseFactory.CreateSuccess(req.ID, result)

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
			hitRate := cache.HitRate(cacheMetrics)

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
	if err := json.NewEncoder(w).Encode(health); err != nil {
		common.GatewayLogger.Error("Failed to encode health response: %v", err)
	}
}

// handleLanguages returns supported language names and their file extensions
func (g *HTTPGateway) handleLanguages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	languages := config.GetAllSupportedLanguages()
	extensions := registry.GetSupportedExtensions()

	result := map[string]interface{}{
		"languages":  languages,
		"extensions": extensions,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		common.GatewayLogger.Error("Failed to encode languages response: %v", err)
	}
}

// writeResponse writes a JSON-RPC response (success or error) to the HTTP response writer
func (g *HTTPGateway) writeResponse(w http.ResponseWriter, response JSONRPCResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK) // JSON-RPC errors still return 200
	if err := json.NewEncoder(w).Encode(response); err != nil {
		common.GatewayLogger.Error("Failed to encode JSON-RPC response: %v", err)
	}
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
		return cacheStatusDisabled
	}

	if after.HitCount > before.HitCount {
		return cacheStatusHit
	} else if after.MissCount > before.MissCount {
		return cacheStatusMiss
	}
	return cacheStatusUnknown
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
	hitRate := cache.HitRate(metrics)

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
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		common.GatewayLogger.Error("Failed to encode cache stats response: %v", err)
	}
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
		if encErr := json.NewEncoder(w).Encode(health); encErr != nil {
			common.GatewayLogger.Error("Failed to encode cache health error response: %v", encErr)
		}
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
	if err := json.NewEncoder(w).Encode(health); err != nil {
		common.GatewayLogger.Error("Failed to encode cache health response: %v", err)
	}
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

	// Clear all cache entries using the cache interface
	if err := cache.Clear(); err != nil {
		http.Error(w, fmt.Sprintf("Failed to clear cache: %v", err), http.StatusInternalServerError)
		return
	}
	metrics := cache.GetMetrics()
	result := map[string]interface{}{
		"status": "cleared",
		"entry_count": func() int64 {
			if metrics != nil {
				return metrics.EntryCount
			}
			return 0
		}(),
		"total_size": func() int64 {
			if metrics != nil {
				return metrics.TotalSize
			}
			return 0
		}(),
		"hit_count": func() int64 {
			if metrics != nil {
				return metrics.HitCount
			}
			return 0
		}(),
		"miss_count": func() int64 {
			if metrics != nil {
				return metrics.MissCount
			}
			return 0
		}(),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		common.GatewayLogger.Error("Failed to encode cache clear response: %v", err)
	}
}
