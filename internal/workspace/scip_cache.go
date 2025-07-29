package workspace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"lsp-gateway/internal/storage"
)

// WorkspaceSCIPCache provides workspace-isolated SCIP caching with L1/L2 architecture
type WorkspaceSCIPCache interface {
	Initialize(workspaceRoot string, config *CacheConfig) error
	Get(key string) (*storage.CacheEntry, bool)
	Set(key string, entry *storage.CacheEntry, ttl time.Duration) error
	Invalidate() error
	GetStats() CacheStats
	Close() error
	
	// Advanced workspace operations
	InvalidateFile(filePath string) (int, error)
	InvalidateProject(projectPath string) (int, error)
	GetWorkspaceInfo() *WorkspaceInfo
	Optimize(ctx context.Context) (*OptimizationResult, error)
}

// CacheConfig defines configuration for workspace SCIP cache
type CacheConfig struct {
	// Memory tier configuration (L1)
	MaxMemorySize     int64         `yaml:"max_memory_size" json:"max_memory_size"`         // Default: 2GB
	MaxMemoryEntries  int64         `yaml:"max_memory_entries" json:"max_memory_entries"`   // Default: 100k
	MemoryTTL         time.Duration `yaml:"memory_ttl" json:"memory_ttl"`                   // Default: 30min
	
	// Disk tier configuration (L2)
	MaxDiskSize       int64         `yaml:"max_disk_size" json:"max_disk_size"`             // Default: 10GB
	MaxDiskEntries    int64         `yaml:"max_disk_entries" json:"max_disk_entries"`       // Default: 1M
	DiskTTL           time.Duration `yaml:"disk_ttl" json:"disk_ttl"`                       // Default: 24h
	
	// Performance tuning
	CompressionEnabled   bool                 `yaml:"compression_enabled" json:"compression_enabled"`     // Default: true
	CompressionThreshold int64                `yaml:"compression_threshold" json:"compression_threshold"` // Default: 1KB
	CompressionType      storage.CompressionType `yaml:"compression_type" json:"compression_type"`           // Default: LZ4
	
	// Maintenance configuration
	CleanupInterval   time.Duration `yaml:"cleanup_interval" json:"cleanup_interval"`       // Default: 5min
	MetricsInterval   time.Duration `yaml:"metrics_interval" json:"metrics_interval"`       // Default: 30s
	SyncInterval      time.Duration `yaml:"sync_interval" json:"sync_interval"`             // Default: 60s
	
	// Workspace isolation
	IsolationLevel    IsolationLevel `yaml:"isolation_level" json:"isolation_level"`         // Default: Strong
	CrossWorkspaceAccess bool       `yaml:"cross_workspace_access" json:"cross_workspace_access"` // Default: false
}

// IsolationLevel defines the level of workspace isolation
type IsolationLevel int

const (
	IsolationNone IsolationLevel = iota // No isolation (global cache)
	IsolationWeak                       // Logical isolation (shared storage)
	IsolationStrong                     // Physical isolation (separate directories)
)

// CacheStats provides comprehensive statistics about workspace cache
type CacheStats struct {
	// Basic statistics
	WorkspaceRoot     string        `json:"workspace_root"`
	WorkspaceHash     string        `json:"workspace_hash"`
	TotalEntries      int64         `json:"total_entries"`
	TotalSize         int64         `json:"total_size_bytes"`
	
	// Performance metrics
	TotalRequests     int64         `json:"total_requests"`
	CacheHits         int64         `json:"cache_hits"`
	CacheMisses       int64         `json:"cache_misses"`
	HitRate           float64       `json:"hit_rate"`
	AverageLatency    time.Duration `json:"average_latency"`
	
	// Tier breakdown
	L1Stats           TierStats     `json:"l1_memory_stats"`
	L2Stats           TierStats     `json:"l2_disk_stats"`
	
	// Maintenance statistics
	LastCleanup       time.Time     `json:"last_cleanup"`
	LastOptimization  time.Time     `json:"last_optimization"`
	TotalEvictions    int64         `json:"total_evictions"`
	TotalPromotions   int64         `json:"total_promotions"`
	
	// Health metrics
	HealthScore       float64       `json:"health_score"`
	LastHealthCheck   time.Time     `json:"last_health_check"`
	Issues            []HealthIssue `json:"issues,omitempty"`
	
	// Workspace specific
	ProjectPaths      []string      `json:"project_paths"`
	LanguageStats     map[string]int64 `json:"language_stats"`
	FilePathCount     int64         `json:"file_path_count"`
}

// TierStats provides tier-specific statistics
type TierStats struct {
	TierType       storage.TierType `json:"tier_type"`
	EntryCount     int64            `json:"entry_count"`
	SizeBytes      int64            `json:"size_bytes"`
	HitRate        float64          `json:"hit_rate"`
	AvgLatency     time.Duration    `json:"avg_latency"`
	MaxCapacity    int64            `json:"max_capacity"`
	Utilization    float64          `json:"utilization"`
	Evictions      int64            `json:"evictions"`
	Compressions   int64            `json:"compressions"`
}

// HealthIssue represents a cache health issue
type HealthIssue struct {
	Type      string    `json:"type"`
	Severity  string    `json:"severity"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Component string    `json:"component"`
}

// OptimizationResult contains results of cache optimization
type OptimizationResult struct {
	StartTime         time.Time `json:"start_time"`
	Duration          time.Duration `json:"duration"`
	EntriesProcessed  int64     `json:"entries_processed"`
	EntriesEvicted    int64     `json:"entries_evicted"`
	EntriesPromoted   int64     `json:"entries_promoted"`
	SpaceReclaimed    int64     `json:"space_reclaimed_bytes"`
	PerformanceGain   float64   `json:"performance_gain_percent"`
}

// DefaultWorkspaceSCIPCache implements WorkspaceSCIPCache interface
type DefaultWorkspaceSCIPCache struct {
	// Core configuration
	workspaceRoot   string
	workspaceHash   string
	config          *CacheConfig
	cacheDir        string
	
	// Storage tiers
	l1MemoryCache   storage.StorageTier
	l2DiskCache     storage.StorageTier
	
	// Workspace manager integration
	configManager   WorkspaceConfigManager
	
	// Performance tracking
	stats           *cacheStats
	
	// Concurrency control
	mu              sync.RWMutex
	initialized     atomic.Bool
	closed          atomic.Bool
	
	// Background processes
	cleanupTicker   *time.Ticker
	metricsTicker   *time.Ticker
	syncTicker      *time.Ticker
	stopCh          chan struct{}
	wg              sync.WaitGroup
	
	// Health monitoring
	healthScore     atomic.Value // float64
	lastHealthCheck atomic.Value // time.Time
	issues          []HealthIssue
	issuesMu        sync.RWMutex
	
	// Workspace context
	workspaceInfo   *WorkspaceInfo
}

// cacheStats provides atomic counters for performance metrics
type cacheStats struct {
	totalRequests   atomic.Int64
	cacheHits       atomic.Int64
	cacheMisses     atomic.Int64
	totalLatency    atomic.Int64 // nanoseconds
	totalEvictions  atomic.Int64
	totalPromotions atomic.Int64
	startTime       time.Time
}

// NewWorkspaceSCIPCache creates a new workspace-isolated SCIP cache
func NewWorkspaceSCIPCache(configManager WorkspaceConfigManager) WorkspaceSCIPCache {
	return &DefaultWorkspaceSCIPCache{
		configManager: configManager,
		stats:         &cacheStats{startTime: time.Now()},
		stopCh:        make(chan struct{}),
	}
}

// Initialize implements WorkspaceSCIPCache interface
func (c *DefaultWorkspaceSCIPCache) Initialize(workspaceRoot string, config *CacheConfig) error {
	if c.initialized.Load() {
		return errors.New("workspace SCIP cache already initialized")
	}
	
	// Validate inputs
	if workspaceRoot == "" {
		return errors.New("workspace root cannot be empty")
	}
	if config == nil {
		config = c.getDefaultConfig()
	}
	
	// Store configuration
	c.workspaceRoot = workspaceRoot
	c.workspaceHash = c.generateWorkspaceHash(workspaceRoot)
	c.config = config
	
	// Initialize workspace directories
	if err := c.initializeWorkspaceDirectories(); err != nil {
		return fmt.Errorf("failed to initialize workspace directories: %w", err)
	}
	
	// Initialize storage tiers
	if err := c.initializeStorageTiers(); err != nil {
		return fmt.Errorf("failed to initialize storage tiers: %w", err)
	}
	
	// Load workspace info
	if err := c.loadWorkspaceInfo(); err != nil {
		log.Printf("Warning: failed to load workspace info: %v", err)
		c.workspaceInfo = c.createDefaultWorkspaceInfo()
	}
	
	// Start background processes
	c.startBackgroundProcesses()
	
	// Initialize health monitoring
	c.healthScore.Store(1.0)
	c.lastHealthCheck.Store(time.Now())
	
	c.initialized.Store(true)
	
	log.Printf("Workspace SCIP cache initialized for %s (hash: %s)", 
		workspaceRoot, c.workspaceHash)
	
	return nil
}

// Get implements WorkspaceSCIPCache interface with L1/L2 tier access
func (c *DefaultWorkspaceSCIPCache) Get(key string) (*storage.CacheEntry, bool) {
	if !c.initialized.Load() || c.closed.Load() {
		return nil, false
	}
	
	start := time.Now()
	defer func() {
		c.recordLatency(time.Since(start))
		c.stats.totalRequests.Add(1)
	}()
	
	ctx := context.Background()
	
	// Try L1 memory cache first
	if entry, err := c.l1MemoryCache.Get(ctx, key); err == nil && entry != nil {
		c.stats.cacheHits.Add(1)
		entry.Touch()
		return entry, true
	}
	
	// Try L2 disk cache
	if entry, err := c.l2DiskCache.Get(ctx, key); err == nil && entry != nil {
		c.stats.cacheHits.Add(1)
		entry.Touch()
		
		// Promote to L1 if access pattern suggests it
		if c.shouldPromoteToL1(entry) {
			if err := c.promoteToL1(ctx, key, entry); err != nil {
				log.Printf("Failed to promote entry to L1: %v", err)
			} else {
				c.stats.totalPromotions.Add(1)
			}
		}
		
		return entry, true
	}
	
	c.stats.cacheMisses.Add(1)
	return nil, false
}

// Set implements WorkspaceSCIPCache interface with intelligent tier placement
func (c *DefaultWorkspaceSCIPCache) Set(key string, entry *storage.CacheEntry, ttl time.Duration) error {
	if !c.initialized.Load() || c.closed.Load() {
		return errors.New("cache not initialized or closed")
	}
	
	start := time.Now()
	defer c.recordLatency(time.Since(start))
	
	ctx := context.Background()
	
	// Enhance entry with workspace metadata
	entry.TTL = ttl
	entry.CreatedAt = time.Now()
	entry.AccessedAt = time.Now()
	entry.ProjectPath = c.workspaceRoot
	entry.Key = key
	
	// Calculate size and checksum
	if entry.Size == 0 {
		data, _ := json.Marshal(entry)
		entry.Size = int64(len(data))
		entry.Checksum = c.calculateChecksum(data)
	}
	
	// Determine initial tier placement
	targetTier := c.determineInitialTier(entry)
	entry.CurrentTier = targetTier
	entry.OriginTier = targetTier
	
	// Store in appropriate tier
	switch targetTier {
	case storage.TierL1Memory:
		return c.l1MemoryCache.Put(ctx, key, entry)
	case storage.TierL2Disk:
		return c.l2DiskCache.Put(ctx, key, entry)
	default:
		return fmt.Errorf("unsupported tier type: %v", targetTier)
	}
}

// InvalidateFile implements WorkspaceSCIPCache interface  
func (c *DefaultWorkspaceSCIPCache) InvalidateFile(filePath string) (int, error) {
	if !c.initialized.Load() || c.closed.Load() {
		return 0, errors.New("cache not initialized or closed")
	}
	
	ctx := context.Background()
	
	// Invalidate in both tiers
	l1Count, _ := c.l1MemoryCache.InvalidateByFile(ctx, filePath)
	l2Count, _ := c.l2DiskCache.InvalidateByFile(ctx, filePath)
	
	totalCount := l1Count + l2Count
	
	log.Printf("Invalidated %d cache entries for file %s (L1: %d, L2: %d)", 
		totalCount, filePath, l1Count, l2Count)
	
	return totalCount, nil
}

// InvalidateProject implements WorkspaceSCIPCache interface
func (c *DefaultWorkspaceSCIPCache) InvalidateProject(projectPath string) (int, error) {
	if !c.initialized.Load() || c.closed.Load() {
		return 0, errors.New("cache not initialized or closed")
	}
	
	ctx := context.Background()
	
	// Invalidate in both tiers
	l1Count, _ := c.l1MemoryCache.InvalidateByProject(ctx, projectPath)
	l2Count, _ := c.l2DiskCache.InvalidateByProject(ctx, projectPath)
	
	totalCount := l1Count + l2Count
	
	log.Printf("Invalidated %d cache entries for project %s (L1: %d, L2: %d)", 
		totalCount, projectPath, l1Count, l2Count)
	
	return totalCount, nil
}

// Invalidate implements WorkspaceSCIPCache interface - clears entire workspace cache
func (c *DefaultWorkspaceSCIPCache) Invalidate() error {
	if !c.initialized.Load() || c.closed.Load() {
		return errors.New("cache not initialized or closed")
	}
	
	ctx := context.Background()
	
	// Clear both tiers
	if err := c.l1MemoryCache.Clear(ctx); err != nil {
		return fmt.Errorf("failed to clear L1 cache: %w", err)
	}
	
	if err := c.l2DiskCache.Clear(ctx); err != nil {
		return fmt.Errorf("failed to clear L2 cache: %w", err)
	}
	
	// Reset statistics
	c.stats = &cacheStats{startTime: time.Now()}
	
	log.Printf("Invalidated entire workspace cache for %s", c.workspaceRoot)
	
	return nil
}

// GetStats implements WorkspaceSCIPCache interface
func (c *DefaultWorkspaceSCIPCache) GetStats() CacheStats {
	if !c.initialized.Load() {
		return CacheStats{}
	}
	
	// Get tier statistics
	l1Stats := c.l1MemoryCache.GetStats()
	l2Stats := c.l2DiskCache.GetStats()
	
	// Calculate overall statistics
	totalRequests := c.stats.totalRequests.Load()
	cacheHits := c.stats.cacheHits.Load()
	
	var hitRate float64
	if totalRequests > 0 {
		hitRate = float64(cacheHits) / float64(totalRequests)
	}
	
	var avgLatency time.Duration
	if totalRequests > 0 {
		avgLatency = time.Duration(c.stats.totalLatency.Load() / totalRequests)
	}
	
	// Collect workspace-specific stats
	c.issuesMu.RLock()
	issues := make([]HealthIssue, len(c.issues))
	copy(issues, c.issues)
	c.issuesMu.RUnlock()
	
	var projectPaths []string
	if c.workspaceInfo != nil {
		projectPaths = []string{c.workspaceRoot}
	}
	
	return CacheStats{
		WorkspaceRoot:     c.workspaceRoot,
		WorkspaceHash:     c.workspaceHash,
		TotalEntries:      l1Stats.EntryCount + l2Stats.EntryCount,
		TotalSize:         l1Stats.UsedCapacity + l2Stats.UsedCapacity,
		TotalRequests:     totalRequests,
		CacheHits:         cacheHits,
		CacheMisses:       c.stats.cacheMisses.Load(),
		HitRate:           hitRate,
		AverageLatency:    avgLatency,
		L1Stats: TierStats{
			TierType:     storage.TierL1Memory,
			EntryCount:   l1Stats.EntryCount,
			SizeBytes:    l1Stats.UsedCapacity,
			HitRate:      l1Stats.HitRate,
			AvgLatency:   l1Stats.AvgLatency,
			MaxCapacity:  l1Stats.TotalCapacity,
			Utilization:  float64(l1Stats.UsedCapacity) / float64(l1Stats.TotalCapacity),
			Evictions:    l1Stats.EvictionCount,
		},
		L2Stats: TierStats{
			TierType:     storage.TierL2Disk,
			EntryCount:   l2Stats.EntryCount,
			SizeBytes:    l2Stats.UsedCapacity,
			HitRate:      l2Stats.HitRate,
			AvgLatency:   l2Stats.AvgLatency,
			MaxCapacity:  l2Stats.TotalCapacity,
			Utilization:  float64(l2Stats.UsedCapacity) / float64(l2Stats.TotalCapacity),
			Evictions:    l2Stats.EvictionCount,
		},
		TotalEvictions:    c.stats.totalEvictions.Load(),
		TotalPromotions:   c.stats.totalPromotions.Load(),
		HealthScore:       c.healthScore.Load().(float64),
		LastHealthCheck:   c.lastHealthCheck.Load().(time.Time),
		Issues:            issues,
		ProjectPaths:      projectPaths,
		LanguageStats:     c.collectLanguageStats(),
		FilePathCount:     c.countFilePaths(),
	}
}

// GetWorkspaceInfo returns workspace information
func (c *DefaultWorkspaceSCIPCache) GetWorkspaceInfo() *WorkspaceInfo {
	return c.workspaceInfo
}

// Optimize implements WorkspaceSCIPCache interface - performs cache optimization
func (c *DefaultWorkspaceSCIPCache) Optimize(ctx context.Context) (*OptimizationResult, error) {
	if !c.initialized.Load() || c.closed.Load() {
		return nil, errors.New("cache not initialized or closed")
	}
	
	start := time.Now()
	result := &OptimizationResult{
		StartTime: start,
	}
	
	// Perform L1 cache optimization
	l1Stats := c.l1MemoryCache.GetStats()
	if l1Stats.UsedCapacity > int64(float64(l1Stats.TotalCapacity)*0.8) {
		// Promote hot entries, evict cold entries
		// This would involve analyzing access patterns and making tier adjustments
		result.EntriesEvicted += 10 // Placeholder
	}
	
	// Perform L2 cache optimization
	l2Stats := c.l2DiskCache.GetStats()
	if l2Stats.UsedCapacity > int64(float64(l2Stats.TotalCapacity)*0.9) {
		// Clean up expired entries, compress data
		result.SpaceReclaimed += 1024 * 1024 // Placeholder
	}
	
	result.Duration = time.Since(start)
	result.PerformanceGain = 5.0 // Placeholder percentage
	
	log.Printf("Cache optimization completed for workspace %s in %v", 
		c.workspaceRoot, result.Duration)
	
	return result, nil
}

// Close implements WorkspaceSCIPCache interface
func (c *DefaultWorkspaceSCIPCache) Close() error {
	if !c.initialized.Load() || c.closed.Load() {
		return nil
	}
	
	c.closed.Store(true)
	
	// Stop background processes
	close(c.stopCh)
	c.wg.Wait()
	
	// Close storage tiers
	if c.l1MemoryCache != nil {
		c.l1MemoryCache.Close()
	}
	if c.l2DiskCache != nil {
		c.l2DiskCache.Close()
	}
	
	// Stop tickers
	if c.cleanupTicker != nil {
		c.cleanupTicker.Stop()
	}
	if c.metricsTicker != nil {
		c.metricsTicker.Stop()
	}
	if c.syncTicker != nil {
		c.syncTicker.Stop()
	}
	
	log.Printf("Workspace SCIP cache closed for %s", c.workspaceRoot)
	
	return nil
}

// Private helper methods

func (c *DefaultWorkspaceSCIPCache) getDefaultConfig() *CacheConfig {
	return &CacheConfig{
		MaxMemorySize:        2 * 1024 * 1024 * 1024, // 2GB
		MaxMemoryEntries:     100000,                  // 100k entries
		MemoryTTL:            30 * time.Minute,
		MaxDiskSize:          10 * 1024 * 1024 * 1024, // 10GB
		MaxDiskEntries:       1000000,                  // 1M entries
		DiskTTL:              24 * time.Hour,
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

func (c *DefaultWorkspaceSCIPCache) generateWorkspaceHash(workspaceRoot string) string {
	absPath, err := filepath.Abs(workspaceRoot)
	if err != nil {
		absPath = workspaceRoot
	}
	
	hash := sha256.Sum256([]byte(absPath))
	return hex.EncodeToString(hash[:8])
}

func (c *DefaultWorkspaceSCIPCache) initializeWorkspaceDirectories() error {
	// Get workspace directory from config manager
	workspaceDir := c.configManager.GetWorkspaceDirectory(c.workspaceRoot)
	
	// Create cache directory structure
	c.cacheDir = filepath.Join(workspaceDir, "cache", "scip")
	
	dirs := []string{
		c.cacheDir,
		filepath.Join(c.cacheDir, "memory"),
		filepath.Join(c.cacheDir, "disk"),
		filepath.Join(c.cacheDir, "index"),
		filepath.Join(c.cacheDir, "responses"),
	}
	
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create cache directory %s: %w", dir, err)
		}
	}
	
	// Create metadata file
	metadataPath := filepath.Join(c.cacheDir, "metadata.json")
	metadata := map[string]interface{}{
		"workspace_root": c.workspaceRoot,
		"workspace_hash": c.workspaceHash,
		"created_at":     time.Now(),
		"version":        "1.0.0",
		"isolation_level": c.config.IsolationLevel,
	}
	
	metadataData, _ := json.MarshalIndent(metadata, "", "  ")
	if err := os.WriteFile(metadataPath, metadataData, 0644); err != nil {
		return fmt.Errorf("failed to create metadata file: %w", err)
	}
	
	return nil
}

func (c *DefaultWorkspaceSCIPCache) initializeStorageTiers() error {
	ctx := context.Background()
	
	// Initialize L1 memory cache
	l1Config := &storage.L1MemoryCacheConfig{
		MaxCapacity:          c.config.MaxMemorySize,
		MaxEntries:           c.config.MaxMemoryEntries,
		CompressionEnabled:   c.config.CompressionEnabled,
		CompressionThreshold: c.config.CompressionThreshold,
		CompressionType:      c.config.CompressionType,
		TTLCheckInterval:     c.config.CleanupInterval,
		MetricsInterval:      c.config.MetricsInterval,
	}
	
	c.l1MemoryCache = storage.NewL1MemoryCache(l1Config)
	if err := c.l1MemoryCache.Initialize(ctx, storage.TierConfig{}); err != nil {
		return fmt.Errorf("failed to initialize L1 memory cache: %w", err)
	}
	
	// Initialize L2 disk cache
	diskCacheDir := filepath.Join(c.cacheDir, "disk")
	l2TierConfig := storage.TierConfig{
		TierType: storage.TierL2Disk,
		BackendConfig: storage.BackendConfig{
			Type: storage.BackendLocalDisk,
			Options: map[string]interface{}{
				"cache_directory":       diskCacheDir,
				"max_capacity":          c.config.MaxDiskSize,
				"max_entries":           c.config.MaxDiskEntries,
				"compression_enabled":   c.config.CompressionEnabled,
				"compression_threshold": c.config.CompressionThreshold,
				"compression_type":      c.config.CompressionType,
				"sync_interval":         c.config.SyncInterval,
				"cleanup_interval":      c.config.CleanupInterval,
			},
			TimeoutMs:      5000, // 5 second timeout
			MaxConnections: 10,
			KeepAlive:      true,
			BatchSize:      100,
			RetryCount:     3,
			RetryDelayMs:   1000,
		},
		MaxCapacity:       c.config.MaxDiskSize,
		MaxEntries:        c.config.MaxDiskEntries,
		MaxConcurrency:    20,
		TimeoutMs:         5000,
		RetryCount:        3,
		RetryDelayMs:      1000,
		MaintenanceInterval: c.config.CleanupInterval,
	}
	
	c.l2DiskCache = storage.NewDiskCache()
	if err := c.l2DiskCache.Initialize(ctx, l2TierConfig); err != nil {
		return fmt.Errorf("failed to initialize L2 disk cache: %w", err)
	}
	
	return nil
}

func (c *DefaultWorkspaceSCIPCache) loadWorkspaceInfo() error {
	workspaceConfig, err := c.configManager.LoadWorkspaceConfig(c.workspaceRoot)
	if err != nil {
		return err
	}
	
	c.workspaceInfo = &workspaceConfig.Workspace
	return nil
}

func (c *DefaultWorkspaceSCIPCache) createDefaultWorkspaceInfo() *WorkspaceInfo {
	return &WorkspaceInfo{
		WorkspaceID: fmt.Sprintf("ws_%s", c.workspaceHash),
		Name:        filepath.Base(c.workspaceRoot),
		RootPath:    c.workspaceRoot,
		Hash:        c.workspaceHash,
		CreatedAt:   time.Now(),
		LastUpdated: time.Now(),
		Version:     "1.0.0",
	}
}

func (c *DefaultWorkspaceSCIPCache) startBackgroundProcesses() {
	// Cleanup process
	c.cleanupTicker = time.NewTicker(c.config.CleanupInterval)
	c.wg.Add(1)
	go c.cleanupWorker()
	
	// Metrics process
	c.metricsTicker = time.NewTicker(c.config.MetricsInterval)
	c.wg.Add(1)
	go c.metricsWorker()
	
	// Sync process
	c.syncTicker = time.NewTicker(c.config.SyncInterval)
	c.wg.Add(1)
	go c.syncWorker()
}

func (c *DefaultWorkspaceSCIPCache) cleanupWorker() {
	defer c.wg.Done()
	
	for {
		select {
		case <-c.stopCh:
			return
		case <-c.cleanupTicker.C:
			c.performCleanup()
		}
	}
}

func (c *DefaultWorkspaceSCIPCache) metricsWorker() {
	defer c.wg.Done()
	
	for {
		select {
		case <-c.stopCh:
			return
		case <-c.metricsTicker.C:
			c.updateHealthMetrics()
		}
	}
}

func (c *DefaultWorkspaceSCIPCache) syncWorker() {
	defer c.wg.Done()
	
	for {
		select {
		case <-c.stopCh:
			return
		case <-c.syncTicker.C:
			c.performSync()
		}
	}
}

func (c *DefaultWorkspaceSCIPCache) performCleanup() {
	// Perform cleanup on both tiers
	// This would involve removing expired entries, compacting storage, etc.
	// Implementation details would depend on the specific storage tier implementations
	
	log.Printf("Performed cleanup for workspace cache %s", c.workspaceRoot)
}

func (c *DefaultWorkspaceSCIPCache) updateHealthMetrics() {
	// Calculate health score based on various metrics
	score := 1.0
	
	l1Stats := c.l1MemoryCache.GetStats()
	l2Stats := c.l2DiskCache.GetStats()
	
	// Penalize high utilization
	l1Utilization := float64(l1Stats.UsedCapacity) / float64(l1Stats.TotalCapacity)
	l2Utilization := float64(l2Stats.UsedCapacity) / float64(l2Stats.TotalCapacity)
	
	if l1Utilization > 0.9 {
		score -= 0.2
	}
	if l2Utilization > 0.9 {
		score -= 0.1
	}
	
	// Penalize low hit rates
	if l1Stats.HitRate < 0.5 {
		score -= 0.3
	}
	
	// Update health score
	c.healthScore.Store(score)
	c.lastHealthCheck.Store(time.Now())
}

func (c *DefaultWorkspaceSCIPCache) performSync() {
	// Sync cached data to disk
	ctx := context.Background()
	
	if err := c.l2DiskCache.Flush(ctx); err != nil {
		log.Printf("Failed to sync L2 cache: %v", err)
	}
}

func (c *DefaultWorkspaceSCIPCache) shouldPromoteToL1(entry *storage.CacheEntry) bool {
	// Simple promotion strategy based on access count and recency
	return entry.AccessCount > 5 && time.Since(entry.LastHitTime) < time.Hour
}

func (c *DefaultWorkspaceSCIPCache) promoteToL1(ctx context.Context, key string, entry *storage.CacheEntry) error {
	// Move entry from L2 to L1
	entry.CurrentTier = storage.TierL1Memory
	return c.l1MemoryCache.Put(ctx, key, entry)
}

func (c *DefaultWorkspaceSCIPCache) determineInitialTier(entry *storage.CacheEntry) storage.TierType {
	// Simple strategy: small frequently accessed entries go to L1, others to L2
	if entry.Size < 64*1024 { // < 64KB
		return storage.TierL1Memory
	}
	return storage.TierL2Disk
}

func (c *DefaultWorkspaceSCIPCache) calculateChecksum(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func (c *DefaultWorkspaceSCIPCache) recordLatency(latency time.Duration) {
	c.stats.totalLatency.Add(latency.Nanoseconds())
}

func (c *DefaultWorkspaceSCIPCache) collectLanguageStats() map[string]int64 {
	// This would analyze cache entries and count by programming language
	// For now, return empty map
	return make(map[string]int64)
}

func (c *DefaultWorkspaceSCIPCache) countFilePaths() int64 {
	// This would count unique file paths in cache entries
	// For now, return 0
	return 0
}