package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"lsp-gateway/internal/indexing"
	"lsp-gateway/internal/storage"
)

// WorkspaceSCIPStore implements indexing.SCIPStore interface using workspace-isolated cache
// This adapter provides backward compatibility with existing SCIP infrastructure
// while adding workspace isolation capabilities
type WorkspaceSCIPStore struct {
	workspaceCache WorkspaceSCIPCache
	scipConfig     *indexing.SCIPConfig
	workspaceRoot  string
	initialized    bool
}

// NewWorkspaceSCIPStore creates a new workspace-isolated SCIP store
func NewWorkspaceSCIPStore(workspaceRoot string, configManager WorkspaceConfigManager) *WorkspaceSCIPStore {
	return &WorkspaceSCIPStore{
		workspaceCache: NewWorkspaceSCIPCache(configManager),
		workspaceRoot:  workspaceRoot,
	}
}

// LoadIndex implements indexing.SCIPStore interface
func (s *WorkspaceSCIPStore) LoadIndex(path string) error {
	if !s.initialized {
		return fmt.Errorf("workspace SCIP store not initialized")
	}
	
	// For workspace-isolated SCIP, we don't load external indexes
	// Instead, we rely on the workspace cache for isolation
	log.Printf("SCIP: Workspace store using isolated cache for path %s", path)
	return nil
}

// Query implements indexing.SCIPStore interface with workspace-isolated caching
func (s *WorkspaceSCIPStore) Query(method string, params interface{}) indexing.SCIPQueryResult {
	if !s.initialized {
		return indexing.SCIPQueryResult{
			Found:      false,
			Method:     method,
			Error:      "workspace SCIP store not initialized",
			CacheHit:   false,
			QueryTime:  0,
			Confidence: 0.0,
		}
	}
	
	startTime := time.Now()
	
	// Generate cache key
	cacheKey := s.generateCacheKey(method, params)
	
	// Try to get from workspace cache first
	if cachedEntry, found := s.workspaceCache.Get(cacheKey); found {
		return indexing.SCIPQueryResult{
			Found:      true,
			Method:     method,
			Response:   cachedEntry.Response,
			CacheHit:   true,
			QueryTime:  time.Since(startTime),
			Confidence: 0.95, // High confidence for cached results
			Metadata: map[string]interface{}{
				"workspace_root": s.workspaceRoot,
				"source":         "workspace_cache",
				"cache_hit":      true,
			},
		}
	}
	
	// If not in cache, perform actual SCIP query or fallback
	result := s.performWorkspaceQuery(method, params, startTime)
	
	// Cache successful results
	if result.Found && result.Response != nil {
		entry := &storage.CacheEntry{
			Method:      method,
			Response:    result.Response,
			ProjectPath: s.workspaceRoot,
			FilePaths:   s.extractFilePathsFromParams(params),
		}
		
		// Cache with appropriate TTL based on query type
		ttl := s.getTTLForMethod(method)
		if err := s.workspaceCache.Set(cacheKey, entry, ttl); err != nil {
			log.Printf("SCIP: Failed to cache workspace query result: %v", err)
		}
	}
	
	return result
}

// CacheResponse implements indexing.SCIPStore interface
func (s *WorkspaceSCIPStore) CacheResponse(method string, params interface{}, response json.RawMessage) error {
	if !s.initialized {
		return fmt.Errorf("workspace SCIP store not initialized")
	}
	
	cacheKey := s.generateCacheKey(method, params)
	
	entry := &storage.CacheEntry{
		Method:      method,
		Response:    response,
		ProjectPath: s.workspaceRoot,
		FilePaths:   s.extractFilePathsFromParams(params),
	}
	
	ttl := s.getTTLForMethod(method)
	return s.workspaceCache.Set(cacheKey, entry, ttl)
}

// InvalidateFile implements indexing.SCIPStore interface
func (s *WorkspaceSCIPStore) InvalidateFile(filePath string) {
	if !s.initialized {
		return
	}
	
	count, err := s.workspaceCache.InvalidateFile(filePath)
	if err != nil {
		log.Printf("SCIP: Failed to invalidate file %s: %v", filePath, err)
	} else {
		log.Printf("SCIP: Invalidated %d cache entries for file %s", count, filePath)
	}
}

// GetStats implements indexing.SCIPStore interface
func (s *WorkspaceSCIPStore) GetStats() indexing.SCIPStoreStats {
	if !s.initialized {
		return indexing.SCIPStoreStats{}
	}
	
	cacheStats := s.workspaceCache.GetStats()
	
	return indexing.SCIPStoreStats{
		IndexesLoaded:    1, // Workspace as single logical index
		TotalQueries:     cacheStats.TotalRequests,
		CacheHitRate:     cacheStats.HitRate,
		AverageQueryTime: cacheStats.AverageLatency,
		LastQueryTime:    time.Now(), // Approximate
		CacheSize:        int(cacheStats.TotalEntries),
		MemoryUsage:      cacheStats.TotalSize,
	}
}

// Close implements indexing.SCIPStore interface
func (s *WorkspaceSCIPStore) Close() error {
	if s.workspaceCache != nil {
		return s.workspaceCache.Close()
	}
	return nil
}

// Initialize initializes the workspace SCIP store with configuration
func (s *WorkspaceSCIPStore) Initialize(scipConfig *indexing.SCIPConfig) error {
	if s.initialized {
		return fmt.Errorf("workspace SCIP store already initialized")
	}
	
	s.scipConfig = scipConfig
	
	// Convert SCIP config to workspace cache config
	cacheConfig := s.convertSCIPConfigToCacheConfig(scipConfig)
	
	// Initialize workspace cache
	if err := s.workspaceCache.Initialize(s.workspaceRoot, cacheConfig); err != nil {
		return fmt.Errorf("failed to initialize workspace cache: %w", err)
	}
	
	s.initialized = true
	
	log.Printf("SCIP: Workspace store initialized for %s", s.workspaceRoot)
	return nil
}

// GetWorkspaceInfo returns workspace information
func (s *WorkspaceSCIPStore) GetWorkspaceInfo() *WorkspaceInfo {
	if s.workspaceCache != nil {
		return s.workspaceCache.GetWorkspaceInfo()
	}
	return nil
}

// OptimizeCache performs cache optimization
func (s *WorkspaceSCIPStore) OptimizeCache(ctx context.Context) (*OptimizationResult, error) {
	if !s.initialized {
		return nil, fmt.Errorf("workspace SCIP store not initialized")
	}
	
	return s.workspaceCache.Optimize(ctx)
}

// Private helper methods

func (s *WorkspaceSCIPStore) generateCacheKey(method string, params interface{}) string {
	paramsJSON, _ := json.Marshal(params)
	return fmt.Sprintf("%s:%s:%s", s.workspaceRoot, method, string(paramsJSON))
}

func (s *WorkspaceSCIPStore) performWorkspaceQuery(method string, params interface{}, startTime time.Time) indexing.SCIPQueryResult {
	// In a real implementation, this would perform the actual SCIP query
	// For now, we simulate a query that might not find results
	
	// Check if method is supported
	if !indexing.IsSupportedMethod(method) {
		return indexing.SCIPQueryResult{
			Found:      false,
			Method:     method,
			Error:      fmt.Sprintf("unsupported method: %s", method),
			CacheHit:   false,
			QueryTime:  time.Since(startTime),
			Confidence: 0.0,
		}
	}
	
	// Simulate query processing time
	time.Sleep(5 * time.Millisecond)
	
	// For demonstration, return a simulated result
	return indexing.SCIPQueryResult{
		Found:      false,
		Method:     method,
		Error:      "workspace SCIP: no indexed data available - using degraded mode",
		CacheHit:   false,
		QueryTime:  time.Since(startTime),
		Confidence: 0.0,
		Metadata: map[string]interface{}{
			"workspace_root": s.workspaceRoot,
			"source":         "workspace_query",
			"degraded_mode":  true,
		},
	}
}

func (s *WorkspaceSCIPStore) extractFilePathsFromParams(params interface{}) []string {
	var filePaths []string
	
	if paramMap, ok := params.(map[string]interface{}); ok {
		if textDoc, ok := paramMap["textDocument"].(map[string]interface{}); ok {
			if uri, ok := textDoc["uri"].(string); ok {
				// Convert file:// URI to file path
				if len(uri) > 7 && uri[:7] == "file://" {
					filePaths = append(filePaths, uri[7:])
				} else {
					filePaths = append(filePaths, uri)
				}
			}
		}
	}
	
	return filePaths
}

func (s *WorkspaceSCIPStore) getTTLForMethod(method string) time.Duration {
	// Different cache TTLs based on query type
	switch method {
	case "textDocument/definition", "textDocument/references":
		return 30 * time.Minute // Moderate TTL for navigation queries
	case "textDocument/hover":
		return 15 * time.Minute // Shorter TTL for hover info
	case "textDocument/documentSymbol":
		return 10 * time.Minute // Short TTL for document symbols
	case "workspace/symbol":
		return 45 * time.Minute // Longer TTL for workspace symbols
	default:
		return 20 * time.Minute // Default TTL
	}
}

func (s *WorkspaceSCIPStore) convertSCIPConfigToCacheConfig(scipConfig *indexing.SCIPConfig) *CacheConfig {
	if scipConfig == nil {
		return nil
	}
	
	// Convert indexing.SCIPConfig to workspace.CacheConfig
	return &CacheConfig{
		MaxMemorySize:        2 * 1024 * 1024 * 1024, // 2GB default
		MaxMemoryEntries:     int64(scipConfig.CacheConfig.MaxSize),
		MemoryTTL:            scipConfig.CacheConfig.TTL,
		MaxDiskSize:          10 * 1024 * 1024 * 1024, // 10GB default
		MaxDiskEntries:       int64(scipConfig.CacheConfig.MaxSize * 10),
		DiskTTL:              scipConfig.CacheConfig.TTL * 4, // Longer TTL for disk
		CompressionEnabled:   true,
		CompressionThreshold: 1024,
		CompressionType:      storage.CompressionLZ4,
		CleanupInterval:      5 * time.Minute,
		MetricsInterval:      30 * time.Second,
		SyncInterval:         60 * time.Second,
		IsolationLevel:       IsolationStrong,
		CrossWorkspaceAccess: false,
	}
}

// WorkspaceSCIPFactory creates workspace-isolated SCIP stores
type WorkspaceSCIPFactory struct {
	configManager WorkspaceConfigManager
}

// NewWorkspaceSCIPFactory creates a new factory for workspace SCIP stores
func NewWorkspaceSCIPFactory(configManager WorkspaceConfigManager) *WorkspaceSCIPFactory {
	return &WorkspaceSCIPFactory{
		configManager: configManager,
	}
}

// CreateStore creates a new workspace-isolated SCIP store
func (f *WorkspaceSCIPFactory) CreateStore(workspaceRoot string) indexing.SCIPStore {
	return NewWorkspaceSCIPStore(workspaceRoot, f.configManager)
}

// CreateStoreWithConfig creates a workspace SCIP store with specific configuration
func (f *WorkspaceSCIPFactory) CreateStoreWithConfig(workspaceRoot string, scipConfig *indexing.SCIPConfig) (indexing.SCIPStore, error) {
	store := NewWorkspaceSCIPStore(workspaceRoot, f.configManager)
	
	if err := store.Initialize(scipConfig); err != nil {
		return nil, fmt.Errorf("failed to initialize workspace SCIP store: %w", err)
	}
	
	return store, nil
}

// Ensure WorkspaceSCIPStore implements indexing.SCIPStore interface
var _ indexing.SCIPStore = (*WorkspaceSCIPStore)(nil)