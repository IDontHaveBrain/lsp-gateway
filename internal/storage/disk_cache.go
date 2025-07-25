package storage

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/dgraph-io/badger/v4/options"
	"github.com/golang/snappy"
)

// DiskCache implements the L2 disk storage tier using BadgerDB with compression
// Provides <50ms access times on SSD storage with configurable capacity limits
type DiskCache struct {
	// Core storage
	db         *badger.DB
	dbPath     string
	config     TierConfig
	
	// Compression
	compression CompressionProvider
	
	// Statistics and monitoring
	stats      *diskStats
	health     *diskHealth
	capacity   *diskCapacity
	
	// Concurrency control
	mu         sync.RWMutex
	closed     int32
	
	// Background processes
	maintenanceCancel context.CancelFunc
	maintenanceDone   chan struct{}
	
	// Performance monitoring
	latencyHistogram  map[string]*latencyTracker
	throughputTracker *throughputTracker
}

// diskStats tracks comprehensive statistics for the disk cache
type diskStats struct {
	mu sync.RWMutex
	
	// Request statistics
	totalRequests  int64
	cacheHits     int64 
	cacheMisses   int64
	
	// Performance metrics
	totalLatency   time.Duration
	minLatency     time.Duration
	maxLatency     time.Duration
	latencyBuckets map[time.Duration]int64
	
	// Throughput metrics
	requestsPerSecond float64
	bytesPerSecond    float64
	
	// Operation counts
	getOperations    int64
	putOperations    int64
	deleteOperations int64
	batchOperations  int64
	
	// Error tracking
	errorCount       int64
	timeoutCount     int64
	
	// Maintenance operations
	compactionCount  int64
	cleanupCount     int64
	lastMaintenance  time.Time
	
	// Start time for uptime calculation
	startTime time.Time
}

// diskHealth tracks health status and issues
type diskHealth struct {
	mu sync.RWMutex
	
	healthy       bool
	status        TierStatus
	lastCheck     time.Time
	issues        []HealthIssue
	
	// Circuit breaker state
	circuitBreaker struct {
		state        CircuitBreakerState
		failureCount int
		lastFailure  time.Time
		nextRetry    time.Time
	}
	
	// Disk-specific health metrics
	diskIOLatency    time.Duration
	diskUtilization  float64
	availableSpace   int64
	inodeUsage       float64
}

// diskCapacity tracks capacity utilization
type diskCapacity struct {
	mu sync.RWMutex
	
	maxCapacity       int64
	usedCapacity      int64
	availableCapacity int64
	maxEntries        int64
	usedEntries       int64
	utilizationPct    float64
	growthRate        float64
	projectedFull     *time.Time
}

// latencyTracker tracks latency statistics for operations
type latencyTracker struct {
	mu sync.RWMutex
	
	samples    []time.Duration
	totalTime  time.Duration
	count      int64
	min        time.Duration
	max        time.Duration
}

// throughputTracker tracks throughput metrics
type throughputTracker struct {
	mu sync.RWMutex
	
	requestCount  int64
	bytesCount    int64
	windowStart   time.Time
	windowSize    time.Duration
}

// snappyCompression implements CompressionProvider using Snappy
type snappyCompression struct {
	level int
}

// NewDiskCache creates a new L2 disk cache instance
func NewDiskCache() *DiskCache {
	return &DiskCache{
		stats: &diskStats{
			latencyBuckets: make(map[time.Duration]int64),
			startTime:     time.Now(),
		},
		health: &diskHealth{
			healthy: true,
			status:  TierStatusHealthy,
		},
		capacity: &diskCapacity{},
		latencyHistogram: make(map[string]*latencyTracker),
		throughputTracker: &throughputTracker{
			windowStart: time.Now(),
			windowSize:  time.Minute,
		},
		compression: &snappyCompression{level: 1},
		maintenanceDone: make(chan struct{}),
	}
}

// Initialize sets up the disk cache with the provided configuration
func (d *DiskCache) Initialize(ctx context.Context, config TierConfig) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	
	if atomic.LoadInt32(&d.closed) == 1 {
		return fmt.Errorf("disk cache is closed")
	}
	
	d.config = config
	
	// Set up database path
	if d.config.BackendConfig.ConnectionString != "" {
		d.dbPath = d.config.BackendConfig.ConnectionString
	} else {
		d.dbPath = filepath.Join(os.TempDir(), "lsp-gateway-disk-cache")
	}
	
	// Ensure directory exists
	if err := os.MkdirAll(d.dbPath, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}
	
	// Configure BadgerDB options for SSD optimization using method chaining
	opts := badger.DefaultOptions(d.dbPath).
		WithLogger(nil). // Disable badger logging to avoid noise
		WithSyncWrites(false). // Async writes for better performance
		WithCompactL0OnClose(true).
		WithValueThreshold(1024). // Store values > 1KB separately
		WithNumMemtables(3).
		WithNumLevelZeroTables(5).
		WithNumLevelZeroTablesStall(10).
		WithBaseLevelSize(256 << 20). // 256MB base level size (replaces LevelOneSize)
		WithLevelSizeMultiplier(10).
		WithMaxLevels(7).
		WithCompression(options.ZSTD). // Configure compression at BadgerDB level
		WithZSTDCompressionLevel(1). // Fast compression
		WithMemTableSize(64 << 20). // 64MB
		WithBlockCacheSize(100 << 20) // 100MB block cache (replaces MaxCacheSize)
	
	// Open database
	db, err := badger.Open(opts)
	if err != nil {
		return fmt.Errorf("failed to open BadgerDB: %w", err)
	}
	d.db = db
	
	// Initialize capacity tracking
	d.capacity.maxCapacity = config.MaxCapacity
	d.capacity.maxEntries = config.MaxEntries
	d.updateCapacityMetrics()
	
	// Configure compression
	if config.CompressionConfig != nil && config.CompressionConfig.Enabled {
		if err := d.configureCompression(config.CompressionConfig); err != nil {
			return fmt.Errorf("failed to configure compression: %w", err)
		}
	}
	
	// Start background maintenance
	d.startBackgroundMaintenance(ctx)
	
	return nil
}

// Get retrieves a cache entry by key with <50ms performance target
func (d *DiskCache) Get(ctx context.Context, key string) (*CacheEntry, error) {
	start := time.Now()
	defer func() {
		latency := time.Since(start)
		d.recordGetLatency(latency)
		d.recordOperation("get", latency)
	}()
	
	if atomic.LoadInt32(&d.closed) == 1 {
		return nil, fmt.Errorf("disk cache is closed")
	}
	
	atomic.AddInt64(&d.stats.getOperations, 1)
	atomic.AddInt64(&d.stats.totalRequests, 1)
	
	var entry *CacheEntry
	err := d.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				atomic.AddInt64(&d.stats.cacheMisses, 1)
				return nil // Not an error, just a miss
			}
			return err
		}
		
		return item.Value(func(val []byte) error {
			// Decompress if needed
			data := val
			if d.isCompressed(data) {
				decompressed, err := d.compression.Decompress(data[1:]) // Skip compression flag
				if err != nil {
					return fmt.Errorf("decompression failed: %w", err)
				}
				data = decompressed
			}
			
			// Deserialize
			entry = &CacheEntry{}
			if err := json.Unmarshal(data, entry); err != nil {
				return fmt.Errorf("deserialization failed: %w", err)
			}
			
			// Update access metadata
			entry.Touch()
			entry.CurrentTier = TierL2Disk
			
			atomic.AddInt64(&d.stats.cacheHits, 1)
			return nil
		})
	})
	
	if err != nil {
		atomic.AddInt64(&d.stats.errorCount, 1)
		return nil, fmt.Errorf("get operation failed: %w", err)
	}
	
	if entry == nil {
		return nil, nil // Cache miss
	}
	
	// Check expiration
	if entry.IsExpired() {
		// Asynchronously delete expired entry
		go d.Delete(context.Background(), key)
		return nil, nil
	}
	
	return entry, nil
}

// Put stores a cache entry with atomic writes and compression
func (d *DiskCache) Put(ctx context.Context, key string, entry *CacheEntry) error {
	start := time.Now()
	defer func() {
		latency := time.Since(start)
		d.recordPutLatency(latency)
		d.recordOperation("put", latency)
	}()
	
	if atomic.LoadInt32(&d.closed) == 1 {
		return fmt.Errorf("disk cache is closed")
	}
	
	atomic.AddInt64(&d.stats.putOperations, 1)
	
	// Check capacity limits before write
	if !d.checkCapacityLimits(entry) {
		return fmt.Errorf("capacity limits exceeded")
	}
	
	// Prepare entry for storage
	entry.Key = key
	entry.CurrentTier = TierL2Disk
	entry.Version = atomic.AddInt64(&d.stats.putOperations, 0)
	
	// Calculate checksum
	entryJSON, err := json.Marshal(entry)
	if err != nil {
		atomic.AddInt64(&d.stats.errorCount, 1)
		return fmt.Errorf("serialization failed: %w", err)
	}
	
	checksum := fmt.Sprintf("%x", sha256.Sum256(entryJSON))
	entry.Checksum = checksum
	entry.Size = int64(len(entryJSON))
	
	// Re-serialize with checksum
	entryJSON, err = json.Marshal(entry)
	if err != nil {
		atomic.AddInt64(&d.stats.errorCount, 1)
		return fmt.Errorf("serialization with checksum failed: %w", err)
	}
	
	// Apply compression if beneficial
	data := entryJSON
	if d.shouldCompress(entryJSON, entry) {
		compressed, err := d.compression.Compress(entryJSON)
		if err != nil {
			return fmt.Errorf("compression failed: %w", err)
		}
		
		// Check if compression is beneficial
		if len(compressed) < len(entryJSON) {
			data = append([]byte{1}, compressed...) // Prefix with compression flag
			entry.CompressedSize = int64(len(compressed))
		} else {
			data = append([]byte{0}, entryJSON...) // No compression
		}
	} else {
		data = append([]byte{0}, entryJSON...) // No compression
	}
	
	// Atomic write with BadgerDB transaction
	err = d.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), data)
	})
	
	if err != nil {
		atomic.AddInt64(&d.stats.errorCount, 1)
		return fmt.Errorf("write operation failed: %w", err)
	}
	
	// Update capacity metrics
	d.updateCapacityAfterPut(entry)
	
	return nil
}

// Delete removes a cache entry atomically
func (d *DiskCache) Delete(ctx context.Context, key string) error {
	start := time.Now()
	defer func() {
		latency := time.Since(start)
		d.recordOperation("delete", latency)
	}()
	
	if atomic.LoadInt32(&d.closed) == 1 {
		return fmt.Errorf("disk cache is closed")
	}
	
	atomic.AddInt64(&d.stats.deleteOperations, 1)
	
	err := d.db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(key))
	})
	
	if err != nil && err != badger.ErrKeyNotFound {
		atomic.AddInt64(&d.stats.errorCount, 1)
		return fmt.Errorf("delete operation failed: %w", err)
	}
	
	return nil
}

// Exists checks if a key exists in the cache
func (d *DiskCache) Exists(ctx context.Context, key string) (bool, error) {
	if atomic.LoadInt32(&d.closed) == 1 {
		return false, fmt.Errorf("disk cache is closed")
	}
	
	var exists bool
	err := d.db.View(func(txn *badger.Txn) error {
		_, err := txn.Get([]byte(key))
		if err == badger.ErrKeyNotFound {
			exists = false
			return nil
		}
		if err != nil {
			return err
		}
		exists = true
		return nil
	})
	
	return exists, err
}

// GetBatch retrieves multiple cache entries efficiently
func (d *DiskCache) GetBatch(ctx context.Context, keys []string) (map[string]*CacheEntry, error) {
	start := time.Now()
	defer func() {
		latency := time.Since(start)
		d.recordOperation("batch_get", latency)
	}()
	
	if atomic.LoadInt32(&d.closed) == 1 {
		return nil, fmt.Errorf("disk cache is closed")
	}
	
	atomic.AddInt64(&d.stats.batchOperations, 1)
	
	results := make(map[string]*CacheEntry)
	
	err := d.db.View(func(txn *badger.Txn) error {
		for _, key := range keys {
			item, err := txn.Get([]byte(key))
			if err == badger.ErrKeyNotFound {
				continue // Skip missing keys
			}
			if err != nil {
				return err
			}
			
			err = item.Value(func(val []byte) error {
				// Decompress if needed
				data := val
				if d.isCompressed(data) {
					decompressed, err := d.compression.Decompress(data[1:])
					if err != nil {
						return fmt.Errorf("decompression failed for key %s: %w", key, err)
					}
					data = decompressed
				}
				
				// Deserialize
				entry := &CacheEntry{}
				if err := json.Unmarshal(data, entry); err != nil {
					return fmt.Errorf("deserialization failed for key %s: %w", key, err)
				}
				
				// Check expiration
				if !entry.IsExpired() {
					entry.Touch()
					entry.CurrentTier = TierL2Disk
					results[key] = entry
				}
				
				return nil
			})
			
			if err != nil {
				return err
			}
		}
		return nil
	})
	
	if err != nil {
		atomic.AddInt64(&d.stats.errorCount, 1)
		return nil, fmt.Errorf("batch get operation failed: %w", err)
	}
	
	return results, nil
}

// PutBatch stores multiple cache entries efficiently
func (d *DiskCache) PutBatch(ctx context.Context, entries map[string]*CacheEntry) error {
	start := time.Now()
	defer func() {
		latency := time.Since(start)
		d.recordOperation("batch_put", latency)
	}()
	
	if atomic.LoadInt32(&d.closed) == 1 {
		return fmt.Errorf("disk cache is closed")
	}
	
	atomic.AddInt64(&d.stats.batchOperations, 1)
	
	// Prepare all entries for batch write
	prepared := make(map[string][]byte)
	totalSize := int64(0)
	
	for key, entry := range entries {
		// Check individual capacity
		if !d.checkCapacityLimits(entry) {
			return fmt.Errorf("capacity limits exceeded for key: %s", key)
		}
		
		// Prepare entry
		entry.Key = key
		entry.CurrentTier = TierL2Disk
		entry.Version = time.Now().UnixNano()
		
		entryJSON, err := json.Marshal(entry)
		if err != nil {
			return fmt.Errorf("serialization failed for key %s: %w", key, err)
		}
		
		checksum := fmt.Sprintf("%x", sha256.Sum256(entryJSON))
		entry.Checksum = checksum
		entry.Size = int64(len(entryJSON))
		
		// Re-serialize with checksum
		entryJSON, err = json.Marshal(entry)
		if err != nil {
			return fmt.Errorf("serialization with checksum failed for key %s: %w", key, err)
		}
		
		// Apply compression
		data := entryJSON
		if d.shouldCompress(entryJSON, entry) {
			compressed, err := d.compression.Compress(entryJSON)
			if err != nil {
				return fmt.Errorf("compression failed for key %s: %w", key, err)
			}
			
			if len(compressed) < len(entryJSON) {
				data = append([]byte{1}, compressed...)
				entry.CompressedSize = int64(len(compressed))
			} else {
				data = append([]byte{0}, entryJSON...)
			}
		} else {
			data = append([]byte{0}, entryJSON...)
		}
		
		prepared[key] = data
		totalSize += int64(len(data))
	}
	
	// Batch write
	err := d.db.Update(func(txn *badger.Txn) error {
		for key, data := range prepared {
			if err := txn.Set([]byte(key), data); err != nil {
				return fmt.Errorf("failed to set key %s: %w", key, err)
			}
		}
		return nil
	})
	
	if err != nil {
		atomic.AddInt64(&d.stats.errorCount, 1)
		return fmt.Errorf("batch put operation failed: %w", err)
	}
	
	// Update capacity metrics
	d.updateCapacityAfterBatchPut(len(entries), totalSize)
	
	return nil
}

// DeleteBatch removes multiple cache entries efficiently
func (d *DiskCache) DeleteBatch(ctx context.Context, keys []string) error {
	start := time.Now()
	defer func() {
		latency := time.Since(start)
		d.recordOperation("batch_delete", latency)
	}()
	
	if atomic.LoadInt32(&d.closed) == 1 {
		return fmt.Errorf("disk cache is closed")
	}
	
	atomic.AddInt64(&d.stats.batchOperations, 1)
	
	err := d.db.Update(func(txn *badger.Txn) error {
		for _, key := range keys {
			if err := txn.Delete([]byte(key)); err != nil && err != badger.ErrKeyNotFound {
				return fmt.Errorf("failed to delete key %s: %w", key, err)
			}
		}
		return nil
	})
	
	if err != nil {
		atomic.AddInt64(&d.stats.errorCount, 1)
		return fmt.Errorf("batch delete operation failed: %w", err)
	}
	
	return nil
}

// Invalidate removes entries matching a pattern
func (d *DiskCache) Invalidate(ctx context.Context, pattern string) (int, error) {
	if atomic.LoadInt32(&d.closed) == 1 {
		return 0, fmt.Errorf("disk cache is closed")
	}
	
	count := 0
	keysToDelete := make([]string, 0)
	
	// Find matching keys
	err := d.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false // Only need keys
		
		it := txn.NewIterator(opts)
		defer it.Close()
		
		for it.Rewind(); it.Valid(); it.Next() {
			key := string(it.Item().Key())
			if d.matchesPattern(key, pattern) {
				keysToDelete = append(keysToDelete, key)
			}
		}
		return nil
	})
	
	if err != nil {
		return 0, fmt.Errorf("pattern scan failed: %w", err)
	}
	
	// Delete matching keys
	if len(keysToDelete) > 0 {
		if err := d.DeleteBatch(ctx, keysToDelete); err != nil {
			return 0, fmt.Errorf("batch delete failed: %w", err)
		}
		count = len(keysToDelete)
	}
	
	return count, nil
}

// InvalidateByFile removes entries associated with a file path
func (d *DiskCache) InvalidateByFile(ctx context.Context, filePath string) (int, error) {
	if atomic.LoadInt32(&d.closed) == 1 {
		return 0, fmt.Errorf("disk cache is closed")
	}
	
	count := 0
	keysToDelete := make([]string, 0)
	
	// Find entries with matching file paths
	err := d.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		
		it := txn.NewIterator(opts)
		defer it.Close()
		
		for it.Rewind(); it.Valid(); it.Next() {
			err := it.Item().Value(func(val []byte) error {
				// Decompress if needed
				data := val
				if d.isCompressed(data) {
					decompressed, err := d.compression.Decompress(data[1:])
					if err != nil {
						return err
					}
					data = decompressed
				}
				
				// Deserialize to check file paths
				entry := &CacheEntry{}
				if err := json.Unmarshal(data, entry); err != nil {
					return err
				}
				
				// Check if entry is associated with the file
				for _, entryFilePath := range entry.FilePaths {
					if entryFilePath == filePath {
						keysToDelete = append(keysToDelete, string(it.Item().Key()))
						break
					}
				}
				
				return nil
			})
			
			if err != nil {
				return err
			}
		}
		return nil
	})
	
	if err != nil {
		return 0, fmt.Errorf("file invalidation scan failed: %w", err)
	}
	
	// Delete matching keys
	if len(keysToDelete) > 0 {
		if err := d.DeleteBatch(ctx, keysToDelete); err != nil {
			return 0, fmt.Errorf("file invalidation delete failed: %w", err)
		}
		count = len(keysToDelete)
	}
	
	return count, nil
}

// InvalidateByProject removes entries associated with a project path
func (d *DiskCache) InvalidateByProject(ctx context.Context, projectPath string) (int, error) {
	if atomic.LoadInt32(&d.closed) == 1 {
		return 0, fmt.Errorf("disk cache is closed")
	}
	
	count := 0
	keysToDelete := make([]string, 0)
	
	// Find entries with matching project paths
	err := d.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		
		it := txn.NewIterator(opts)
		defer it.Close()
		
		for it.Rewind(); it.Valid(); it.Next() {
			err := it.Item().Value(func(val []byte) error {
				// Decompress if needed
				data := val
				if d.isCompressed(data) {
					decompressed, err := d.compression.Decompress(data[1:])
					if err != nil {
						return err
					}
					data = decompressed
				}
				
				// Deserialize to check project path
				entry := &CacheEntry{}
				if err := json.Unmarshal(data, entry); err != nil {
					return err
				}
				
				// Check if entry belongs to the project
				if entry.ProjectPath == projectPath || strings.HasPrefix(entry.ProjectPath, projectPath+"/") {
					keysToDelete = append(keysToDelete, string(it.Item().Key()))
				}
				
				return nil
			})
			
			if err != nil {
				return err
			}
		}
		return nil
	})
	
	if err != nil {
		return 0, fmt.Errorf("project invalidation scan failed: %w", err)
	}
	
	// Delete matching keys
	if len(keysToDelete) > 0 {
		if err := d.DeleteBatch(ctx, keysToDelete); err != nil {
			return 0, fmt.Errorf("project invalidation delete failed: %w", err)
		}
		count = len(keysToDelete)
	}
	
	return count, nil
}

// Clear removes all entries from the cache
func (d *DiskCache) Clear(ctx context.Context) error {
	if atomic.LoadInt32(&d.closed) == 1 {
		return fmt.Errorf("disk cache is closed")
	}
	
	err := d.db.DropAll()
	if err != nil {
		atomic.AddInt64(&d.stats.errorCount, 1)
		return fmt.Errorf("clear operation failed: %w", err)
	}
	
	// Reset capacity metrics
	d.capacity.mu.Lock()
	d.capacity.usedCapacity = 0
	d.capacity.usedEntries = 0
	d.capacity.utilizationPct = 0
	d.capacity.mu.Unlock()
	
	return nil
}

// GetStats returns current statistics for the disk cache
func (d *DiskCache) GetStats() TierStats {
	d.stats.mu.RLock()
	defer d.stats.mu.RUnlock()
	
	hitRate := float64(0)
	if d.stats.totalRequests > 0 {
		hitRate = float64(d.stats.cacheHits) / float64(d.stats.totalRequests)
	}
	
	avgLatency := time.Duration(0)
	if d.stats.totalRequests > 0 {
		avgLatency = d.stats.totalLatency / time.Duration(d.stats.totalRequests)
	}
	
	uptime := time.Since(d.stats.startTime)
	
	return TierStats{
		TierType:          TierL2Disk,
		TotalCapacity:     d.capacity.maxCapacity,
		UsedCapacity:      d.capacity.usedCapacity,
		FreeCapacity:      d.capacity.availableCapacity,
		EntryCount:        d.capacity.usedEntries,
		TotalRequests:     d.stats.totalRequests,
		CacheHits:         d.stats.cacheHits,
		CacheMisses:       d.stats.cacheMisses,
		HitRate:           hitRate,
		AvgLatency:        avgLatency,
		P50Latency:        d.calculatePercentile(0.5),
		P95Latency:        d.calculatePercentile(0.95),
		P99Latency:        d.calculatePercentile(0.99),
		MaxLatency:        d.stats.maxLatency,
		RequestsPerSecond: d.stats.requestsPerSecond,
		BytesPerSecond:    d.stats.bytesPerSecond,
		ErrorCount:        d.stats.errorCount,
		ErrorRate:         float64(d.stats.errorCount) / float64(d.stats.totalRequests),
		TimeoutCount:      d.stats.timeoutCount,
		CompactionCount:   d.stats.compactionCount,
		StartTime:         d.stats.startTime,
		LastUpdate:        time.Now(),
		Uptime:            uptime,
	}
}

// GetHealth returns current health status
func (d *DiskCache) GetHealth() TierHealth {
	d.health.mu.RLock()
	defer d.health.mu.RUnlock()
	
	return TierHealth{
		TierType:  TierL2Disk,
		Healthy:   d.health.healthy,
		Status:    d.health.status,
		LastCheck: d.health.lastCheck,
		Issues:    append([]HealthIssue{}, d.health.issues...),
		CircuitBreaker: struct {
			State        CircuitBreakerState `json:"state"`
			FailureCount int                 `json:"failure_count"`
			LastFailure  time.Time           `json:"last_failure,omitempty"`
			NextRetry    time.Time           `json:"next_retry,omitempty"`
		}{
			State:        d.health.circuitBreaker.state,
			FailureCount: d.health.circuitBreaker.failureCount,
			LastFailure:  d.health.circuitBreaker.lastFailure,
			NextRetry:    d.health.circuitBreaker.nextRetry,
		},
	}
}

// GetCapacity returns current capacity information
func (d *DiskCache) GetCapacity() TierCapacity {
	d.capacity.mu.RLock()
	defer d.capacity.mu.RUnlock()
	
	return TierCapacity{
		TierType:          TierL2Disk,
		MaxCapacity:       d.capacity.maxCapacity,
		UsedCapacity:      d.capacity.usedCapacity,
		AvailableCapacity: d.capacity.availableCapacity,
		MaxEntries:        d.capacity.maxEntries,
		UsedEntries:       d.capacity.usedEntries,
		UtilizationPct:    d.capacity.utilizationPct,
		GrowthRate:        d.capacity.growthRate,
		ProjectedFull:     d.capacity.projectedFull,
	}
}

// GetTierType returns the tier type
func (d *DiskCache) GetTierType() TierType {
	return TierL2Disk
}

// GetTierLevel returns the tier level
func (d *DiskCache) GetTierLevel() int {
	return 2
}

// Flush ensures all pending writes are committed to disk
func (d *DiskCache) Flush(ctx context.Context) error {
	if atomic.LoadInt32(&d.closed) == 1 {
		return fmt.Errorf("disk cache is closed")
	}
	
	return d.db.Sync()
}

// Close gracefully shuts down the disk cache
func (d *DiskCache) Close() error {
	if !atomic.CompareAndSwapInt32(&d.closed, 0, 1) {
		return nil // Already closed
	}
	
	// Cancel background maintenance
	if d.maintenanceCancel != nil {
		d.maintenanceCancel()
		<-d.maintenanceDone
	}
	
	// Close database
	if d.db != nil {
		return d.db.Close()
	}
	
	return nil
}

// Background maintenance and helper methods

// startBackgroundMaintenance starts background cleanup and compaction processes
func (d *DiskCache) startBackgroundMaintenance(ctx context.Context) {
	maintenanceCtx, cancel := context.WithCancel(ctx)
	d.maintenanceCancel = cancel
	
	go func() {
		defer close(d.maintenanceDone)
		
		ticker := time.NewTicker(d.config.MaintenanceInterval)
		defer ticker.Stop()
		
		for {
			select {
			case <-maintenanceCtx.Done():
				return
			case <-ticker.C:
				d.performMaintenance(maintenanceCtx)
			}
		}
	}()
}

// performMaintenance runs cleanup and optimization tasks
func (d *DiskCache) performMaintenance(ctx context.Context) {
	start := time.Now()
	
	// Run BadgerDB garbage collection
	go func() {
		if err := d.db.RunValueLogGC(0.5); err != nil && err != badger.ErrNoRewrite {
			d.recordHealthIssue("maintenance", IssueSeverityWarning, 
				fmt.Sprintf("GC failed: %v", err))
		}
	}()
	
	// Clean expired entries
	d.cleanExpiredEntries(ctx)
	
	// Update statistics and health
	d.updateHealthStatus()
	d.updateCapacityMetrics()
	
	// Record maintenance
	atomic.AddInt64(&d.stats.compactionCount, 1)
	d.stats.mu.Lock()
	d.stats.lastMaintenance = time.Now()
	d.stats.mu.Unlock()
	
	duration := time.Since(start)
	if duration > time.Minute {
		d.recordHealthIssue("maintenance", IssueSeverityWarning,
			fmt.Sprintf("Maintenance took %v (> 1 minute)", duration))
	}
}

// cleanExpiredEntries removes expired cache entries
func (d *DiskCache) cleanExpiredEntries(ctx context.Context) {
	expiredKeys := make([]string, 0)
	
	err := d.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()
		
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			
			// Check if item is expired based on BadgerDB TTL
			if item.ExpiresAt() > 0 && item.ExpiresAt() < uint64(time.Now().Unix()) {
				expiredKeys = append(expiredKeys, string(item.Key()))
				continue
			}
			
			// Check cache entry TTL
			err := item.Value(func(val []byte) error {
				data := val
				if d.isCompressed(data) {
					decompressed, err := d.compression.Decompress(data[1:])
					if err != nil {
						return err
					}
					data = decompressed
				}
				
				entry := &CacheEntry{}
				if err := json.Unmarshal(data, entry); err != nil {
					return err
				}
				
				if entry.IsExpired() {
					expiredKeys = append(expiredKeys, string(item.Key()))
				}
				
				return nil
			})
			
			if err != nil {
				// If we can't deserialize, consider it expired
				expiredKeys = append(expiredKeys, string(item.Key()))
			}
		}
		return nil
	})
	
	if err != nil {
		d.recordHealthIssue("cleanup", IssueSeverityError,
			fmt.Sprintf("Expired entry scan failed: %v", err))
		return
	}
	
	// Delete expired entries
	if len(expiredKeys) > 0 {
		if err := d.DeleteBatch(ctx, expiredKeys); err != nil {
			d.recordHealthIssue("cleanup", IssueSeverityError,
				fmt.Sprintf("Expired entry deletion failed: %v", err))
		} else {
			atomic.AddInt64(&d.stats.cleanupCount, 1)
		}
	}
}

// Compression implementation

// Compress implements CompressionProvider interface
func (s *snappyCompression) Compress(data []byte) ([]byte, error) {
	return snappy.Encode(nil, data), nil
}

// Decompress implements CompressionProvider interface
func (s *snappyCompression) Decompress(data []byte) ([]byte, error) {
	return snappy.Decode(nil, data)
}

// EstimateCompressionRatio implements CompressionProvider interface
func (s *snappyCompression) EstimateCompressionRatio(data []byte) float64 {
	// Snappy typically achieves 2-4x compression on JSON data
	return 0.3 // Conservative estimate
}

// GetCompressionType implements CompressionProvider interface
func (s *snappyCompression) GetCompressionType() CompressionType {
	return CompressionSnappy
}

// GetCompressionLevel implements CompressionProvider interface
func (s *snappyCompression) GetCompressionLevel() int {
	return s.level
}

// SetCompressionLevel implements CompressionProvider interface
func (s *snappyCompression) SetCompressionLevel(level int) error {
	s.level = level
	return nil
}

// SetCompressionType implements CompressionProvider interface
func (s *snappyCompression) SetCompressionType(compressionType CompressionType) error {
	if compressionType != CompressionSnappy {
		return fmt.Errorf("unsupported compression type: %s", compressionType)
	}
	return nil
}

// Helper methods

// isCompressed checks if data has compression flag
func (d *DiskCache) isCompressed(data []byte) bool {
	return len(data) > 0 && data[0] == 1
}

// shouldCompress determines if entry should be compressed
func (d *DiskCache) shouldCompress(data []byte, entry *CacheEntry) bool {
	if d.config.CompressionConfig == nil || !d.config.CompressionConfig.Enabled {
		return false
	}
	
	// Only compress if larger than minimum size
	if int64(len(data)) < d.config.CompressionConfig.MinSize {
		return false
	}
	
	// Check caching hints
	return entry.CompressionHint != CompressionNone
}

// configureCompression sets up compression based on configuration
func (d *DiskCache) configureCompression(config *CompressionConfig) error {
	switch config.Type {
	case CompressionSnappy:
		d.compression = &snappyCompression{level: config.Level}
	case CompressionNone:
		d.compression = nil
	default:
		return fmt.Errorf("unsupported compression type: %s", config.Type)
	}
	return nil
}

// checkCapacityLimits verifies if entry can be stored within limits
func (d *DiskCache) checkCapacityLimits(entry *CacheEntry) bool {
	d.capacity.mu.RLock()
	defer d.capacity.mu.RUnlock()
	
	// Check entry count limit
	if d.capacity.maxEntries > 0 && d.capacity.usedEntries >= d.capacity.maxEntries {
		return false
	}
	
	// Check capacity limit
	if d.capacity.maxCapacity > 0 {
		estimatedSize := entry.Size
		if estimatedSize == 0 {
			estimatedSize = 1024 // Default estimate
		}
		
		if d.capacity.usedCapacity+estimatedSize > d.capacity.maxCapacity {
			return false
		}
	}
	
	return true
}

// updateCapacityMetrics refreshes capacity tracking metrics
func (d *DiskCache) updateCapacityMetrics() {
	d.capacity.mu.Lock()
	defer d.capacity.mu.Unlock()
	
	// Get actual disk usage from BadgerDB
	lsm, vlog := d.db.Size()
	actualSize := lsm + vlog
	
	d.capacity.usedCapacity = actualSize
	d.capacity.availableCapacity = d.capacity.maxCapacity - d.capacity.usedCapacity
	
	if d.capacity.maxCapacity > 0 {
		d.capacity.utilizationPct = float64(d.capacity.usedCapacity) / float64(d.capacity.maxCapacity)
	}
	
	// Update entry count
	entryCount := int64(0)
	d.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		
		it := txn.NewIterator(opts)
		defer it.Close()
		
		for it.Rewind(); it.Valid(); it.Next() {
			entryCount++
		}
		return nil
	})
	
	d.capacity.usedEntries = entryCount
}

// updateCapacityAfterPut updates capacity after a put operation
func (d *DiskCache) updateCapacityAfterPut(entry *CacheEntry) {
	d.capacity.mu.Lock()
	defer d.capacity.mu.Unlock()
	
	d.capacity.usedEntries++
	d.capacity.usedCapacity += entry.Size
	
	if entry.CompressedSize > 0 {
		d.capacity.usedCapacity += entry.CompressedSize - entry.Size
	}
	
	d.capacity.availableCapacity = d.capacity.maxCapacity - d.capacity.usedCapacity
	
	if d.capacity.maxCapacity > 0 {
		d.capacity.utilizationPct = float64(d.capacity.usedCapacity) / float64(d.capacity.maxCapacity)
	}
}

// updateCapacityAfterBatchPut updates capacity after batch put
func (d *DiskCache) updateCapacityAfterBatchPut(entryCount int, totalSize int64) {
	d.capacity.mu.Lock()
	defer d.capacity.mu.Unlock()
	
	d.capacity.usedEntries += int64(entryCount)
	d.capacity.usedCapacity += totalSize
	d.capacity.availableCapacity = d.capacity.maxCapacity - d.capacity.usedCapacity
	
	if d.capacity.maxCapacity > 0 {
		d.capacity.utilizationPct = float64(d.capacity.usedCapacity) / float64(d.capacity.maxCapacity)
	}
}

// matchesPattern checks if a key matches a glob pattern
func (d *DiskCache) matchesPattern(key, pattern string) bool {
	// Simple glob pattern matching - extend as needed
	if pattern == "*" {
		return true
	}
	
	if strings.Contains(pattern, "*") {
		// Simple prefix/suffix matching
		if strings.HasPrefix(pattern, "*") {
			return strings.HasSuffix(key, pattern[1:])
		}
		if strings.HasSuffix(pattern, "*") {
			return strings.HasPrefix(key, pattern[:len(pattern)-1])
		}
	}
	
	return key == pattern
}

// recordGetLatency tracks get operation latency
func (d *DiskCache) recordGetLatency(latency time.Duration) {
	d.recordLatency("get", latency)
}

// recordPutLatency tracks put operation latency
func (d *DiskCache) recordPutLatency(latency time.Duration) {
	d.recordLatency("put", latency)
}

// recordLatency tracks operation latency in histogram
func (d *DiskCache) recordLatency(operation string, latency time.Duration) {
	d.stats.mu.Lock()
	defer d.stats.mu.Unlock()
	
	d.stats.totalLatency += latency
	
	if d.stats.minLatency == 0 || latency < d.stats.minLatency {
		d.stats.minLatency = latency
	}
	
	if latency > d.stats.maxLatency {
		d.stats.maxLatency = latency
	}
	
	// Update latency buckets
	bucket := d.getLatencyBucket(latency)
	d.stats.latencyBuckets[bucket]++
	
	// Update operation-specific tracking
	if tracker, exists := d.latencyHistogram[operation]; exists {
		tracker.mu.Lock()
		tracker.samples = append(tracker.samples, latency)
		if len(tracker.samples) > 1000 {
			// Keep only recent samples
			tracker.samples = tracker.samples[len(tracker.samples)-1000:]
		}
		tracker.totalTime += latency
		tracker.count++
		
		if tracker.min == 0 || latency < tracker.min {
			tracker.min = latency
		}
		if latency > tracker.max {
			tracker.max = latency
		}
		tracker.mu.Unlock()
	} else {
		d.latencyHistogram[operation] = &latencyTracker{
			samples:   []time.Duration{latency},
			totalTime: latency,
			count:     1,
			min:       latency,
			max:       latency,
		}
	}
}

// recordOperation tracks general operation metrics
func (d *DiskCache) recordOperation(operation string, latency time.Duration) {
	// Update throughput tracking
	d.throughputTracker.mu.Lock()
	d.throughputTracker.requestCount++
	
	// Update rates if window has elapsed
	now := time.Now()
	if now.Sub(d.throughputTracker.windowStart) >= d.throughputTracker.windowSize {
		elapsed := now.Sub(d.throughputTracker.windowStart).Seconds()
		
		d.stats.mu.Lock()
		d.stats.requestsPerSecond = float64(d.throughputTracker.requestCount) / elapsed
		d.stats.bytesPerSecond = float64(d.throughputTracker.bytesCount) / elapsed
		d.stats.mu.Unlock()
		
		// Reset window
		d.throughputTracker.requestCount = 0
		d.throughputTracker.bytesCount = 0
		d.throughputTracker.windowStart = now
	}
	d.throughputTracker.mu.Unlock()
}

// getLatencyBucket returns appropriate latency bucket for histogram
func (d *DiskCache) getLatencyBucket(latency time.Duration) time.Duration {
	buckets := []time.Duration{
		time.Millisecond,
		5 * time.Millisecond,
		10 * time.Millisecond,
		25 * time.Millisecond,
		50 * time.Millisecond,
		100 * time.Millisecond,
		250 * time.Millisecond,
		500 * time.Millisecond,
		time.Second,
		5 * time.Second,
	}
	
	for _, bucket := range buckets {
		if latency <= bucket {
			return bucket
		}
	}
	
	return 10 * time.Second
}

// calculatePercentile calculates latency percentiles from recorded samples
func (d *DiskCache) calculatePercentile(percentile float64) time.Duration {
	// This is a simplified percentile calculation
	// In production, use a proper histogram library like Prometheus
	
	totalRequests := atomic.LoadInt64(&d.stats.totalRequests)
	if totalRequests == 0 {
		return 0
	}
	
	avgLatency := time.Duration(0)
	d.stats.mu.RLock()
	if d.stats.totalRequests > 0 {
		avgLatency = d.stats.totalLatency / time.Duration(d.stats.totalRequests)
	}
	d.stats.mu.RUnlock()
	
	// Simple estimation based on average
	switch percentile {
	case 0.5:
		return avgLatency
	case 0.95:
		return time.Duration(float64(avgLatency) * 1.5)
	case 0.99:
		return time.Duration(float64(avgLatency) * 2.0)
	default:
		return avgLatency
	}
}

// updateHealthStatus performs health checks and updates status
func (d *DiskCache) updateHealthStatus() {
	d.health.mu.Lock()
	defer d.health.mu.Unlock()
	
	d.health.lastCheck = time.Now()
	previouslyHealthy := d.health.healthy
	
	// Clear old issues
	d.health.issues = d.health.issues[:0]
	
	// Check performance metrics
	stats := d.GetStats()
	
	// Check latency
	if stats.AvgLatency > 50*time.Millisecond {
		d.health.issues = append(d.health.issues, HealthIssue{
			Type:      "performance",
			Severity:  IssueSeverityWarning,
			Message:   fmt.Sprintf("Average latency %v exceeds 50ms target", stats.AvgLatency),
			Timestamp: time.Now(),
			Component: "disk_cache",
		})
	}
	
	// Check error rate
	if stats.ErrorRate > 0.05 { // 5% error threshold
		d.health.issues = append(d.health.issues, HealthIssue{
			Type:      "errors",
			Severity:  IssueSeverityError,
			Message:   fmt.Sprintf("Error rate %.2f%% exceeds 5%% threshold", stats.ErrorRate*100),
			Timestamp: time.Now(),
			Component: "disk_cache",
		})
	}
	
	// Check capacity
	capacity := d.GetCapacity()
	if capacity.UtilizationPct > 0.9 { // 90% capacity threshold
		severity := IssueSeverityWarning
		if capacity.UtilizationPct > 0.95 {
			severity = IssueSeverityCritical
		}
		
		d.health.issues = append(d.health.issues, HealthIssue{
			Type:      "capacity",
			Severity:  severity,
			Message:   fmt.Sprintf("Disk utilization %.1f%% is high", capacity.UtilizationPct*100),
			Timestamp: time.Now(),
			Component: "disk_cache",
		})
	}
	
	// Determine overall health status
	criticalIssues := 0
	for _, issue := range d.health.issues {
		if issue.Severity == IssueSeverityCritical {
			criticalIssues++
		}
	}
	
	if criticalIssues > 0 {
		d.health.healthy = false
		d.health.status = TierStatusUnavailable
	} else if len(d.health.issues) > 0 {
		d.health.healthy = true
		d.health.status = TierStatusDegraded
	} else {
		d.health.healthy = true
		d.health.status = TierStatusHealthy
	}
	
	// Update circuit breaker
	if !d.health.healthy && previouslyHealthy {
		d.health.circuitBreaker.failureCount++
		d.health.circuitBreaker.lastFailure = time.Now()
		
		if d.health.circuitBreaker.failureCount >= 5 {
			d.health.circuitBreaker.state = CircuitBreakerOpen
			d.health.circuitBreaker.nextRetry = time.Now().Add(30 * time.Second)
		}
	} else if d.health.healthy {
		d.health.circuitBreaker.failureCount = 0
		d.health.circuitBreaker.state = CircuitBreakerClosed
	}
}

// recordHealthIssue adds a health issue to tracking
func (d *DiskCache) recordHealthIssue(issueType string, severity IssueSeverity, message string) {
	d.health.mu.Lock()
	defer d.health.mu.Unlock()
	
	issue := HealthIssue{
		Type:      issueType,
		Severity:  severity,
		Message:   message,
		Timestamp: time.Now(),
		Component: "disk_cache",
	}
	
	d.health.issues = append(d.health.issues, issue)
	
	// Trigger health status update
	go d.updateHealthStatus()
}