package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
)

const (
	DefaultCacheMaxEntries    = 1000
	DefaultCacheTTL           = 30 * time.Minute
	DefaultCleanupInterval    = 5 * time.Minute
	DefaultBackgroundWorkers  = 4
	DefaultUpdateThreshold    = 5 * time.Second
	DefaultFastModeThreshold  = 100 // Switch to fast mode for projects with < 100 files
	DefaultMaxConcurrentScans = 10
)

// CacheStats represents cache performance statistics
type CacheStats struct {
	HitCount           int64         `json:"hit_count"`
	MissCount          int64         `json:"miss_count"`
	TotalEntries       int           `json:"total_entries"`
	HitRatio           float64       `json:"hit_ratio"`
	EvictionCount      int64         `json:"eviction_count"`
	BackgroundScans    int64         `json:"background_scans"`
	IncrementalUpdates int64         `json:"incremental_updates"`
	AverageAccessTime  time.Duration `json:"average_access_time"`
	CacheSize          int64         `json:"cache_size_bytes"`
	LastCleanup        time.Time     `json:"last_cleanup"`
}

// CacheEntry represents a cached project detection result
type CacheEntry struct {
	ProjectInfo    *MultiLanguageProjectInfo `json:"project_info"`
	LastScanned    time.Time                 `json:"last_scanned"`
	LastModified   time.Time                 `json:"last_modified"`
	FileCount      int                       `json:"file_count"`
	IsValid        bool                      `json:"is_valid"`
	AccessCount    int64                     `json:"access_count"`
	ScanDuration   time.Duration             `json:"scan_duration"`
	CacheHitTime   time.Time                 `json:"cache_hit_time"`
	ValidationHash string                    `json:"validation_hash"`
	Size           int64                     `json:"size_bytes"`
}

// ProjectCache provides thread-safe caching for project detection results
type ProjectCache struct {
	cache           map[string]*CacheEntry
	mutex           sync.RWMutex
	maxEntries      int
	ttl             time.Duration
	cleanupInterval time.Duration

	// Statistics
	hitCount           int64
	missCount          int64
	evictionCount      int64
	backgroundScans    int64
	incrementalUpdates int64

	// Background processing
	backgroundScanner  *BackgroundScanner
	incrementalUpdater *IncrementalUpdater

	// Cleanup
	cleanupTicker *time.Ticker
	stopCleanup   chan struct{}

	// Context for graceful shutdown
	ctx    context.Context
	cancel context.CancelFunc
}

// NewProjectCache creates a new project cache with default settings
func NewProjectCache() *ProjectCache {
	return NewProjectCacheWithConfig(CacheConfig{
		MaxEntries:        DefaultCacheMaxEntries,
		TTL:               DefaultCacheTTL,
		CleanupInterval:   DefaultCleanupInterval,
		BackgroundWorkers: DefaultBackgroundWorkers,
	})
}

// CacheConfig represents configuration for the project cache
type CacheConfig struct {
	MaxEntries         int
	TTL                time.Duration
	CleanupInterval    time.Duration
	BackgroundWorkers  int
	EnableFileWatching bool
}

// NewProjectCacheWithConfig creates a new project cache with custom configuration
func NewProjectCacheWithConfig(config CacheConfig) *ProjectCache {
	ctx, cancel := context.WithCancel(context.Background())

	pc := &ProjectCache{
		cache:           make(map[string]*CacheEntry),
		maxEntries:      config.MaxEntries,
		ttl:             config.TTL,
		cleanupInterval: config.CleanupInterval,
		stopCleanup:     make(chan struct{}),
		ctx:             ctx,
		cancel:          cancel,
	}

	// Initialize background scanner
	pc.backgroundScanner = NewBackgroundScanner(pc, config.BackgroundWorkers)

	// Initialize incremental updater
	if config.EnableFileWatching {
		pc.incrementalUpdater = NewIncrementalUpdater(pc)
	}

	// Start cleanup routine
	pc.startCleanupRoutine()

	return pc
}

// Get retrieves a cached project info if valid, returns nil if not found or expired
func (pc *ProjectCache) Get(rootPath string) (*MultiLanguageProjectInfo, bool) {
	defer func() {
		// Update access time statistics
		// This could be enhanced with a sliding window for better average calculation
		// TODO: Add access time tracking when needed
	}()

	pc.mutex.RLock()
	entry, exists := pc.cache[rootPath]
	pc.mutex.RUnlock()

	if !exists {
		atomic.AddInt64(&pc.missCount, 1)
		return nil, false
	}

	// Check if entry is still valid
	if !pc.isEntryValid(entry) {
		pc.InvalidateProject(rootPath)
		atomic.AddInt64(&pc.missCount, 1)
		return nil, false
	}

	// Update access statistics
	atomic.AddInt64(&entry.AccessCount, 1)
	entry.CacheHitTime = time.Now()

	atomic.AddInt64(&pc.hitCount, 1)
	return entry.ProjectInfo, true
}

// Set stores project info in the cache
func (pc *ProjectCache) Set(rootPath string, info *MultiLanguageProjectInfo) {
	if info == nil {
		return
	}

	// Calculate entry size for statistics
	entrySize := pc.calculateEntrySize(info)

	// Create validation hash for cache coherency
	validationHash := pc.generateValidationHash(rootPath, info)

	entry := &CacheEntry{
		ProjectInfo:    info,
		LastScanned:    time.Now(),
		LastModified:   pc.getDirectoryModTime(rootPath),
		FileCount:      info.TotalFileCount,
		IsValid:        true,
		AccessCount:    0,
		ScanDuration:   info.ScanDuration,
		CacheHitTime:   time.Now(),
		ValidationHash: validationHash,
		Size:           entrySize,
	}

	pc.mutex.Lock()
	defer pc.mutex.Unlock()

	// Check if we need to evict entries
	if len(pc.cache) >= pc.maxEntries {
		pc.evictLRU()
	}

	pc.cache[rootPath] = entry

	// Queue for file system watching if enabled
	if pc.incrementalUpdater != nil {
		go func() {
			if err := pc.incrementalUpdater.WatchProject(rootPath); err != nil {
				// Log error or handle gracefully
				// For now, we'll continue without file watching for this project
				_ = err
			}
		}()
	}
}

// InvalidateProject removes a project from the cache
func (pc *ProjectCache) InvalidateProject(rootPath string) {
	pc.mutex.Lock()
	defer pc.mutex.Unlock()

	if _, exists := pc.cache[rootPath]; exists {
		delete(pc.cache, rootPath)
		atomic.AddInt64(&pc.evictionCount, 1)
	}

	// Stop watching this project
	if pc.incrementalUpdater != nil {
		pc.incrementalUpdater.StopWatching(rootPath)
	}
}

// InvalidateAll clears all cached entries
func (pc *ProjectCache) InvalidateAll() {
	pc.mutex.Lock()
	defer pc.mutex.Unlock()

	evicted := int64(len(pc.cache))
	pc.cache = make(map[string]*CacheEntry)
	atomic.AddInt64(&pc.evictionCount, evicted)

	// Stop all file watching
	if pc.incrementalUpdater != nil {
		pc.incrementalUpdater.StopAllWatching()
	}
}

// GetStats returns current cache statistics
func (pc *ProjectCache) GetStats() CacheStats {
	pc.mutex.RLock()
	totalEntries := len(pc.cache)

	var totalSize int64
	var totalAccessTime time.Duration
	var accessCount int64

	for _, entry := range pc.cache {
		totalSize += entry.Size
		if entry.AccessCount > 0 {
			totalAccessTime += time.Since(entry.CacheHitTime)
			accessCount += entry.AccessCount
		}
	}
	pc.mutex.RUnlock()

	hits := atomic.LoadInt64(&pc.hitCount)
	misses := atomic.LoadInt64(&pc.missCount)
	total := hits + misses

	var hitRatio float64
	if total > 0 {
		hitRatio = float64(hits) / float64(total)
	}

	var avgAccessTime time.Duration
	if accessCount > 0 {
		avgAccessTime = totalAccessTime / time.Duration(accessCount)
	}

	return CacheStats{
		HitCount:           hits,
		MissCount:          misses,
		TotalEntries:       totalEntries,
		HitRatio:           hitRatio,
		EvictionCount:      atomic.LoadInt64(&pc.evictionCount),
		BackgroundScans:    atomic.LoadInt64(&pc.backgroundScans),
		IncrementalUpdates: atomic.LoadInt64(&pc.incrementalUpdates),
		AverageAccessTime:  avgAccessTime,
		CacheSize:          totalSize,
		LastCleanup:        time.Now(), // This should be tracked properly
	}
}

// Shutdown gracefully shuts down the cache and all background processes
func (pc *ProjectCache) Shutdown() {
	pc.cancel()

	if pc.cleanupTicker != nil {
		pc.cleanupTicker.Stop()
	}

	if pc.backgroundScanner != nil {
		pc.backgroundScanner.Stop()
	}

	if pc.incrementalUpdater != nil {
		pc.incrementalUpdater.Stop()
	}

	close(pc.stopCleanup)
}

// isEntryValid checks if a cache entry is still valid
func (pc *ProjectCache) isEntryValid(entry *CacheEntry) bool {
	if !entry.IsValid {
		return false
	}

	// Check TTL
	if time.Since(entry.LastScanned) > pc.ttl {
		return false
	}

	// Check if directory was modified since last scan
	currentModTime := pc.getDirectoryModTime(entry.ProjectInfo.RootPath)
	return !currentModTime.After(entry.LastModified)
}

// evictLRU evicts the least recently used entry
func (pc *ProjectCache) evictLRU() {
	var oldestKey string
	var oldestTime = time.Now()

	for key, entry := range pc.cache {
		if entry.CacheHitTime.Before(oldestTime) {
			oldestTime = entry.CacheHitTime
			oldestKey = key
		}
	}

	if oldestKey != "" {
		delete(pc.cache, oldestKey)
		atomic.AddInt64(&pc.evictionCount, 1)
	}
}

// startCleanupRoutine starts the background cleanup routine
func (pc *ProjectCache) startCleanupRoutine() {
	pc.cleanupTicker = time.NewTicker(pc.cleanupInterval)

	go func() {
		for {
			select {
			case <-pc.cleanupTicker.C:
				pc.cleanup()
			case <-pc.stopCleanup:
				return
			case <-pc.ctx.Done():
				return
			}
		}
	}()
}

// cleanup removes expired entries from the cache
func (pc *ProjectCache) cleanup() {
	pc.mutex.Lock()
	defer pc.mutex.Unlock()

	var toDelete []string

	for key, entry := range pc.cache {
		if !pc.isEntryValid(entry) {
			toDelete = append(toDelete, key)
		}
	}

	for _, key := range toDelete {
		delete(pc.cache, key)
		atomic.AddInt64(&pc.evictionCount, 1)
	}
}

// getDirectoryModTime returns the last modification time of a directory
// This includes checking files within the directory for a more accurate cache invalidation
func (pc *ProjectCache) getDirectoryModTime(dirPath string) time.Time {
	info, err := os.Stat(dirPath)
	if err != nil {
		return time.Time{}
	}
	
	dirModTime := info.ModTime()
	
	// Also check immediate files in the directory for changes
	// This provides better cache invalidation when files are added/modified
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return dirModTime
	}
	
	maxModTime := dirModTime
	for _, entry := range entries {
		if !entry.IsDir() {
			entryPath := filepath.Join(dirPath, entry.Name())
			entryInfo, err := os.Stat(entryPath)
			if err != nil {
				continue
			}
			if entryInfo.ModTime().After(maxModTime) {
				maxModTime = entryInfo.ModTime()
			}
		}
	}
	
	return maxModTime
}

// calculateEntrySize estimates the memory size of a cache entry
func (pc *ProjectCache) calculateEntrySize(info *MultiLanguageProjectInfo) int64 {
	// This is a rough estimation - in production you might want more accurate measurement
	data, err := json.Marshal(info)
	if err != nil {
		return 1024 // Default estimate
	}
	return int64(len(data))
}

// generateValidationHash creates a hash for cache validation
func (pc *ProjectCache) generateValidationHash(rootPath string, info *MultiLanguageProjectInfo) string {
	// Simple hash based on key properties
	// In production, you might use a proper hash function
	return fmt.Sprintf("%s_%d_%v_%s", rootPath, info.TotalFileCount,
		info.ScanDuration, info.DominantLanguage)
}

// =============================================================================
// Background Scanner
// =============================================================================

// BackgroundScanner handles background project scanning
type BackgroundScanner struct {
	cache   *ProjectCache
	scanner *ProjectLanguageScanner
	queue   chan string
	workers int
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// NewBackgroundScanner creates a new background scanner
func NewBackgroundScanner(cache *ProjectCache, workers int) *BackgroundScanner {
	ctx, cancel := context.WithCancel(context.Background())

	bs := &BackgroundScanner{
		cache:   cache,
		scanner: nil, // Will be set lazily to avoid circular dependency
		queue:   make(chan string, workers*2), // Buffer for better performance
		workers: workers,
		ctx:     ctx,
		cancel:  cancel,
	}

	bs.Start()
	return bs
}

// getScanner returns the scanner, creating it lazily to avoid circular dependency
func (bs *BackgroundScanner) getScanner() *ProjectLanguageScanner {
	if bs.scanner == nil {
		bs.scanner = NewProjectLanguageScannerWithoutCache()
	}
	return bs.scanner
}

// Start begins the background scanning workers
func (bs *BackgroundScanner) Start() {
	for i := 0; i < bs.workers; i++ {
		bs.wg.Add(1)
		go bs.worker(i)
	}
}

// QueueProject queues a project for background scanning
func (bs *BackgroundScanner) QueueProject(rootPath string) {
	select {
	case bs.queue <- rootPath:
		// Successfully queued
	case <-bs.ctx.Done():
		return
	default:
		// Queue is full, skip this scan
		// In production, you might want to implement a priority queue
	}
}

// Stop gracefully stops all background workers
func (bs *BackgroundScanner) Stop() {
	bs.cancel()
	close(bs.queue)
	bs.wg.Wait()
}

// worker processes projects from the queue
func (bs *BackgroundScanner) worker(id int) {
	defer bs.wg.Done()

	for {
		select {
		case rootPath, ok := <-bs.queue:
			if !ok {
				return // Channel closed
			}
			bs.scanProject(rootPath)
		case <-bs.ctx.Done():
			return
		}
	}
}

// scanProject performs the actual project scan
func (bs *BackgroundScanner) scanProject(rootPath string) {
	// Check if we already have a recent cache entry
	if _, exists := bs.cache.Get(rootPath); exists {
		// Entry is still valid, no need to rescan
		return
	}

	// Perform the scan
	info, err := bs.getScanner().ScanProjectComprehensive(rootPath)
	if err != nil {
		// Log error and continue
		return
	}

	// Store in cache
	bs.cache.Set(rootPath, info)
	atomic.AddInt64(&bs.cache.backgroundScans, 1)
}

// =============================================================================
// Incremental Updater
// =============================================================================

// IncrementalUpdater handles file system watching for incremental updates
type IncrementalUpdater struct {
	cache           *ProjectCache
	watchedPaths    map[string]*fsnotify.Watcher
	mutex           sync.RWMutex
	updateThreshold time.Duration
	ctx             context.Context
	cancel          context.CancelFunc
}

// NewIncrementalUpdater creates a new incremental updater
func NewIncrementalUpdater(cache *ProjectCache) *IncrementalUpdater {
	ctx, cancel := context.WithCancel(context.Background())

	return &IncrementalUpdater{
		cache:           cache,
		watchedPaths:    make(map[string]*fsnotify.Watcher),
		updateThreshold: DefaultUpdateThreshold,
		ctx:             ctx,
		cancel:          cancel,
	}
}

// WatchProject starts watching a project directory for changes
func (iu *IncrementalUpdater) WatchProject(rootPath string) error {
	iu.mutex.Lock()
	defer iu.mutex.Unlock()

	// Check if already watching
	if _, exists := iu.watchedPaths[rootPath]; exists {
		return nil
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}

	err = watcher.Add(rootPath)
	if err != nil {
		_ = watcher.Close()
		return fmt.Errorf("failed to watch directory: %w", err)
	}

	iu.watchedPaths[rootPath] = watcher

	// Start watching in a goroutine
	go iu.watchLoop(rootPath, watcher)

	return nil
}

// StopWatching stops watching a specific project
func (iu *IncrementalUpdater) StopWatching(rootPath string) {
	iu.mutex.Lock()
	defer iu.mutex.Unlock()

	if watcher, exists := iu.watchedPaths[rootPath]; exists {
		_ = watcher.Close()
		delete(iu.watchedPaths, rootPath)
	}
}

// StopAllWatching stops watching all projects
func (iu *IncrementalUpdater) StopAllWatching() {
	iu.mutex.Lock()
	defer iu.mutex.Unlock()

	for _, watcher := range iu.watchedPaths {
		_ = watcher.Close()
	}
	iu.watchedPaths = make(map[string]*fsnotify.Watcher)
}

// Stop gracefully stops the incremental updater
func (iu *IncrementalUpdater) Stop() {
	iu.cancel()
	iu.StopAllWatching()
}

// watchLoop handles file system events for a specific project
func (iu *IncrementalUpdater) watchLoop(rootPath string, watcher *fsnotify.Watcher) {
	defer func() { _ = watcher.Close() }()

	lastUpdate := time.Now()

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			// Debounce updates
			if time.Since(lastUpdate) < iu.updateThreshold {
				continue
			}

			iu.handleFileChange(rootPath, event)
			lastUpdate = time.Now()

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			// Log error and continue
			_ = err

		case <-iu.ctx.Done():
			return
		}
	}
}

// handleFileChange processes file system changes
func (iu *IncrementalUpdater) handleFileChange(rootPath string, event fsnotify.Event) {
	// Check if this change affects our cached project info
	if iu.shouldInvalidateCache(event) {
		iu.cache.InvalidateProject(rootPath)

		// Optionally queue for background re-scan
		if iu.cache.backgroundScanner != nil {
			iu.cache.backgroundScanner.QueueProject(rootPath)
		}

		atomic.AddInt64(&iu.cache.incrementalUpdates, 1)
	}
}

// shouldInvalidateCache determines if a file change should invalidate the cache
func (iu *IncrementalUpdater) shouldInvalidateCache(event fsnotify.Event) bool {
	// Check for important file changes that would affect project detection
	fileName := filepath.Base(event.Name)

	// Build files
	buildFiles := []string{
		"package.json", "go.mod", "Cargo.toml", "pom.xml", "build.gradle",
		"setup.py", "pyproject.toml", "requirements.txt", "Makefile",
	}

	for _, buildFile := range buildFiles {
		if fileName == buildFile {
			return true
		}
	}

	// Config files
	configFiles := []string{
		"tsconfig.json", ".eslintrc", ".golangci.yml", "tox.ini",
		"angular.json", "next.config.js",
	}

	for _, configFile := range configFiles {
		if fileName == configFile {
			return true
		}
	}

	// Directory structure changes
	if event.Op&fsnotify.Create == fsnotify.Create ||
		event.Op&fsnotify.Remove == fsnotify.Remove {

		// New directories could contain new language files
		if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
			return true
		}
	}

	return false
}
