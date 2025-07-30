package testutils

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// RepoCacheEntry represents an enhanced cache entry with metadata
type RepoCacheEntry struct {
	RepoURL      string                 `json:"repo_url"`
	CommitHash   string                 `json:"commit_hash"`
	CachePath    string                 `json:"cache_path"`
	CreatedAt    time.Time              `json:"created_at"`
	LastAccessed time.Time              `json:"last_accessed"`
	AccessCount  int64                  `json:"access_count"`
	SizeBytes    int64                  `json:"size_bytes"`
	TTL          time.Duration          `json:"ttl"`
	Metadata     map[string]interface{} `json:"metadata"`
	IsHealthy    bool                   `json:"is_healthy"`
}

// CacheConfig defines cache configuration
type CacheConfig struct {
	MaxSizeGB      int           `json:"max_size_gb"`      // Maximum cache size in GB (default: 10)
	MaxEntries     int           `json:"max_entries"`      // Maximum number of entries (default: 1000)
	DefaultTTL     time.Duration `json:"default_ttl"`      // Default entry TTL (default: 30 days)
	CleanupInterval time.Duration `json:"cleanup_interval"` // Background cleanup interval (default: 1 hour)
	CacheDir       string        `json:"cache_dir"`        // Cache directory path
	EnableLogging  bool          `json:"enable_logging"`   // Enable debug logging
}

// CacheStats provides cache statistics
type CacheStats struct {
	TotalEntries     int           `json:"total_entries"`
	CurrentSizeGB    float64       `json:"current_size_gb"`
	HitCount         int64         `json:"hit_count"`
	MissCount        int64         `json:"miss_count"`
	EvictionCount    int64         `json:"eviction_count"`
	HitRate          float64       `json:"hit_rate"`
	AverageAccessTime time.Duration `json:"average_access_time"`
	OldestEntry      time.Time     `json:"oldest_entry"`
	NewestEntry      time.Time     `json:"newest_entry"`
	LastCleanup      time.Time     `json:"last_cleanup"`
}

// lruNode represents a node in the LRU linked list
type lruNode struct {
	key   string
	entry *RepoCacheEntry
	prev  *lruNode
	next  *lruNode
}

// RepoCacheCore is the core cache engine with LRU policy
type RepoCacheCore struct {
	config          CacheConfig
	entries         map[string]*lruNode // Key -> LRU Node
	head            *lruNode            // Most recently used
	tail            *lruNode            // Least recently used
	currentSizeGB   float64
	
	// Statistics
	hitCount      int64
	missCount     int64
	evictionCount int64
	accessTimes   []time.Duration
	
	// Thread safety
	mu sync.RWMutex
	
	// Background cleanup
	cleanupTicker *time.Ticker
	stopCleanup   chan struct{}
	cleanupDone   chan struct{}
}

// NewRepoCacheCore creates a new cache core instance
func NewRepoCacheCore(config CacheConfig) *RepoCacheCore {
	// Set defaults
	if config.MaxSizeGB <= 0 {
		config.MaxSizeGB = 10
	}
	if config.MaxEntries <= 0 {
		config.MaxEntries = 1000
	}
	if config.DefaultTTL <= 0 {
		config.DefaultTTL = 30 * 24 * time.Hour // 30 days
	}
	if config.CleanupInterval <= 0 {
		config.CleanupInterval = time.Hour
	}
	if config.CacheDir == "" {
		config.CacheDir = filepath.Join(os.TempDir(), "lsp-gateway-cache")
	}

	// Create cache directory
	os.MkdirAll(config.CacheDir, 0755)

	// Initialize LRU list with dummy head/tail
	head := &lruNode{}
	tail := &lruNode{}
	head.next = tail
	tail.prev = head

	cache := &RepoCacheCore{
		config:        config,
		entries:       make(map[string]*lruNode),
		head:          head,
		tail:          tail,
		stopCleanup:   make(chan struct{}),
		cleanupDone:   make(chan struct{}),
		accessTimes:   make([]time.Duration, 0, 1000),
	}

	// Start background cleanup
	cache.startBackgroundCleanup()

	return cache
}

// Get retrieves a cache entry by key
func (c *RepoCacheCore) Get(key string) (*RepoCacheEntry, bool) {
	startTime := time.Now()
	defer func() {
		c.recordAccessTime(time.Since(startTime))
	}()

	c.mu.Lock()
	defer c.mu.Unlock()

	node, exists := c.entries[key]
	if !exists {
		c.missCount++
		return nil, false
	}

	entry := node.entry

	// Check TTL
	if c.isExpired(entry) {
		c.removeNodeUnsafe(node)
		delete(c.entries, key)
		c.missCount++
		return nil, false
	}

	// Move to front (most recently used)
	c.moveToFrontUnsafe(node)
	
	// Update access information
	entry.LastAccessed = time.Now()
	entry.AccessCount++
	
	c.hitCount++
	return entry, true
}

// Set stores a cache entry
func (c *RepoCacheCore) Set(key string, entry *RepoCacheEntry) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if entry already exists
	if node, exists := c.entries[key]; exists {
		// Update existing entry
		oldSize := c.calculateSizeGB(node.entry)
		node.entry = entry
		newSize := c.calculateSizeGB(entry)
		c.currentSizeGB = c.currentSizeGB - oldSize + newSize
		c.moveToFrontUnsafe(node)
		return nil
	}

	// Calculate entry size
	entrySize := c.calculateSizeGB(entry)

	// Enforce size limits
	for (c.currentSizeGB+entrySize > float64(c.config.MaxSizeGB)) || (len(c.entries) >= c.config.MaxEntries) {
		if !c.evictLRUUnsafe() {
			return fmt.Errorf("cache full and unable to evict entries")
		}
	}

	// Create new node and add to front
	node := &lruNode{
		key:   key,
		entry: entry,
	}
	
	c.entries[key] = node
	c.addToFrontUnsafe(node)
	c.currentSizeGB += entrySize

	// Set creation time if not set
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	entry.LastAccessed = time.Now()

	return nil
}

// Delete removes a cache entry
func (c *RepoCacheCore) Delete(key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	node, exists := c.entries[key]
	if !exists {
		return fmt.Errorf("key not found: %s", key)
	}

	c.removeNodeUnsafe(node)
	delete(c.entries, key)
	c.currentSizeGB -= c.calculateSizeGB(node.entry)

	return nil
}

// GetStats returns current cache statistics
func (c *RepoCacheCore) GetStats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var oldest, newest time.Time
	if len(c.entries) > 0 {
		// Find oldest and newest entries
		first := true
		for _, node := range c.entries {
			if first {
				oldest = node.entry.CreatedAt
				newest = node.entry.CreatedAt
				first = false
			} else {
				if node.entry.CreatedAt.Before(oldest) {
					oldest = node.entry.CreatedAt
				}
				if node.entry.CreatedAt.After(newest) {
					newest = node.entry.CreatedAt
				}
			}
		}
	}

	total := c.hitCount + c.missCount
	hitRate := 0.0
	if total > 0 {
		hitRate = float64(c.hitCount) / float64(total)
	}

	avgAccessTime := time.Duration(0)
	if len(c.accessTimes) > 0 {
		var sum time.Duration
		for _, t := range c.accessTimes {
			sum += t
		}
		avgAccessTime = sum / time.Duration(len(c.accessTimes))
	}

	return CacheStats{
		TotalEntries:      len(c.entries),
		CurrentSizeGB:     c.currentSizeGB,
		HitCount:          c.hitCount,
		MissCount:         c.missCount,
		EvictionCount:     c.evictionCount,
		HitRate:           hitRate,
		AverageAccessTime: avgAccessTime,
		OldestEntry:       oldest,
		NewestEntry:       newest,
		LastCleanup:       time.Now(), // Will be updated by actual cleanup
	}
}

// Clear removes all cache entries
func (c *RepoCacheCore) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Reset LRU list
	c.head.next = c.tail
	c.tail.prev = c.head
	
	// Clear maps and counters
	c.entries = make(map[string]*lruNode)
	c.currentSizeGB = 0
	c.hitCount = 0
	c.missCount = 0
	c.evictionCount = 0
	c.accessTimes = c.accessTimes[:0]

	return nil
}

// Close stops background cleanup and releases resources
func (c *RepoCacheCore) Close() error {
	if c.cleanupTicker != nil {
		close(c.stopCleanup)
		<-c.cleanupDone
	}
	return nil
}

// Helper methods

// generateCacheKey creates a SHA256-based cache key
func GenerateCacheKey(repoURL, commitHash string) string {
	data := fmt.Sprintf("%s@%s", repoURL, commitHash)
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash)
}

// moveToFrontUnsafe moves a node to the front of the LRU list (most recently used)
func (c *RepoCacheCore) moveToFrontUnsafe(node *lruNode) {
	c.removeNodeUnsafe(node)
	c.addToFrontUnsafe(node)
}

// addToFrontUnsafe adds a node to the front of the LRU list
func (c *RepoCacheCore) addToFrontUnsafe(node *lruNode) {
	node.prev = c.head
	node.next = c.head.next
	c.head.next.prev = node
	c.head.next = node
}

// removeNodeUnsafe removes a node from the LRU list
func (c *RepoCacheCore) removeNodeUnsafe(node *lruNode) {
	if node.prev != nil {
		node.prev.next = node.next
	}
	if node.next != nil {
		node.next.prev = node.prev
	}
}

// evictLRUUnsafe removes the least recently used entry
func (c *RepoCacheCore) evictLRUUnsafe() bool {
	if c.tail.prev == c.head {
		return false // Empty cache
	}

	lru := c.tail.prev
	c.removeNodeUnsafe(lru)
	delete(c.entries, lru.key)
	c.currentSizeGB -= c.calculateSizeGB(lru.entry)
	c.evictionCount++

	return true
}

// isExpired checks if an entry has expired based on TTL
func (c *RepoCacheCore) isExpired(entry *RepoCacheEntry) bool {
	if entry.TTL <= 0 {
		return false // No expiration
	}
	return time.Since(entry.CreatedAt) > entry.TTL
}

// calculateSizeGB estimates the size of a cache entry in GB
func (c *RepoCacheCore) calculateSizeGB(entry *RepoCacheEntry) float64 {
	if entry.SizeBytes > 0 {
		return float64(entry.SizeBytes) / (1024 * 1024 * 1024)
	}
	
	// Estimate size if not provided
	if entry.CachePath != "" {
		if stat, err := os.Stat(entry.CachePath); err == nil && stat.IsDir() {
			if size, err := c.calculateDirectorySize(entry.CachePath); err == nil {
				return float64(size) / (1024 * 1024 * 1024)
			}
		}
	}
	
	// Default estimate: 50MB per repository
	return 0.05
}

// calculateDirectorySize calculates the total size of a directory
func (c *RepoCacheCore) calculateDirectorySize(dirPath string) (int64, error) {
	var size int64
	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// recordAccessTime records access time for statistics
func (c *RepoCacheCore) recordAccessTime(duration time.Duration) {
	// Keep only last 1000 access times for memory efficiency
	if len(c.accessTimes) >= 1000 {
		c.accessTimes = c.accessTimes[1:]
	}
	c.accessTimes = append(c.accessTimes, duration)
}

// startBackgroundCleanup starts the background cleanup routine
func (c *RepoCacheCore) startBackgroundCleanup() {
	c.cleanupTicker = time.NewTicker(c.config.CleanupInterval)
	
	go func() {
		defer close(c.cleanupDone)
		
		for {
			select {
			case <-c.cleanupTicker.C:
				c.performCleanup()
			case <-c.stopCleanup:
				c.cleanupTicker.Stop()
				return
			}
		}
	}()
}

// performCleanup removes expired entries and enforces size limits
func (c *RepoCacheCore) performCleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Remove expired entries
	keysToRemove := make([]string, 0)
	for key, node := range c.entries {
		if c.isExpired(node.entry) {
			keysToRemove = append(keysToRemove, key)
		}
	}

	for _, key := range keysToRemove {
		if node, exists := c.entries[key]; exists {
			c.removeNodeUnsafe(node)
			delete(c.entries, key)
			c.currentSizeGB -= c.calculateSizeGB(node.entry)
		}
	}

	// Enforce size limits by evicting LRU entries
	for c.currentSizeGB > float64(c.config.MaxSizeGB) || len(c.entries) > c.config.MaxEntries {
		if !c.evictLRUUnsafe() {
			break
		}
	}

	if c.config.EnableLogging && len(keysToRemove) > 0 {
		fmt.Printf("Cache cleanup: removed %d expired entries\n", len(keysToRemove))
	}
}