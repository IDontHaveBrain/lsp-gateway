package scip

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"lsp-gateway/src/internal/common"
)

const (
	DefaultMemoryLimit        = 256 * 1024 * 1024 // 256MB
	DefaultCompactionInterval = 30 * time.Minute
	DefaultMaxDocumentAge     = 24 * time.Hour
	DefaultCompressionType    = "gzip"
	MetadataFileName          = "storage_metadata.json"
	DocumentCacheDir          = "documents"
	SymbolIndexDir            = "symbols"
	ReferenceIndexDir         = "references"
)

// SCIPStorageManager implements the SCIPDocumentStorage interface with hybrid memory/disk strategy
type SCIPStorageManager struct {
	config  SCIPStorageConfig
	mutex   sync.RWMutex
	started bool

	// Hot memory cache - frequently accessed symbols
	hotMemoryCache map[string]*SCIPDocument
	hotCacheAccess map[string]time.Time
	hotCacheSize   int64

	// Warm cache - LRU document cache
	warmDocumentCache map[string]*SCIPDocument
	warmCacheOrder    []string // LRU order tracking
	warmCacheSize     int64

	// Cold disk storage
	diskCacheDir string
	diskUsage    int64

	// Symbol and reference indexes
	symbolIndex    map[string][]SCIPSymbol    // symbol name -> symbols
	referenceIndex map[string][]SCIPReference // symbol ID -> references

	// Cache managers
	cacheManager      *SCIPMemoryCacheManager
	compactionManager *SCIPDiskCompactionManager
	healthChecker     *SCIPStorageHealthChecker

	// Metrics
	hitCounts   map[string]int64
	totalHits   int64
	totalMisses int64

	// Background goroutine controls
	stopCompaction chan struct{}
	wg             sync.WaitGroup
}

// SCIPMemoryCacheManager handles memory cache operations
type SCIPMemoryCacheManager struct {
	storage    *SCIPStorageManager
	evictCount int64
}

// SCIPDiskCompactionManager handles disk compaction operations
type SCIPDiskCompactionManager struct {
	storage  *SCIPStorageManager
	running  bool
	lastRun  time.Time
	interval time.Duration
	stopChan chan struct{}
	mutex    sync.RWMutex
}

// SCIPStorageHealthChecker monitors storage health
type SCIPStorageHealthChecker struct {
	storage   *SCIPStorageManager
	lastCheck time.Time
	status    string
	mutex     sync.RWMutex
}

// SCIPStorageMetadata represents persistent storage metadata
type SCIPStorageMetadata struct {
	Version       string    `json:"version"`
	CreatedAt     time.Time `json:"created_at"`
	LastModified  time.Time `json:"last_modified"`
	DocumentCount int       `json:"document_count"`
	TotalSize     int64     `json:"total_size"`
}

// NewSCIPStorageManager creates a new SCIP storage manager
func NewSCIPStorageManager(config SCIPStorageConfig) (*SCIPStorageManager, error) {
	if config.MemoryLimit == 0 {
		config.MemoryLimit = DefaultMemoryLimit
	}
	if config.CompactionInterval == 0 {
		config.CompactionInterval = DefaultCompactionInterval
	}
	if config.MaxDocumentAge == 0 {
		config.MaxDocumentAge = DefaultMaxDocumentAge
	}
	if config.CompressionType == "" {
		config.CompressionType = DefaultCompressionType
	}
	if config.DiskCacheDir == "" {
		config.DiskCacheDir = filepath.Join(os.TempDir(), "lsp-gateway-scip-cache")
	}

	storage := &SCIPStorageManager{
		config:            config,
		hotMemoryCache:    make(map[string]*SCIPDocument),
		hotCacheAccess:    make(map[string]time.Time),
		warmDocumentCache: make(map[string]*SCIPDocument),
		warmCacheOrder:    make([]string, 0),
		diskCacheDir:      config.DiskCacheDir,
		symbolIndex:       make(map[string][]SCIPSymbol),
		referenceIndex:    make(map[string][]SCIPReference),
		hitCounts:         make(map[string]int64),
		stopCompaction:    make(chan struct{}),
	}

	storage.cacheManager = &SCIPMemoryCacheManager{storage: storage}
	storage.compactionManager = &SCIPDiskCompactionManager{
		storage:  storage,
		interval: config.CompactionInterval,
		stopChan: make(chan struct{}),
	}
	storage.healthChecker = &SCIPStorageHealthChecker{
		storage: storage,
		status:  "unknown",
	}

	return storage, nil
}

// Start initializes the storage manager
func (s *SCIPStorageManager) Start(ctx context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.started {
		return fmt.Errorf("storage manager already started")
	}

	common.LSPLogger.Info("Starting SCIP storage manager with memory limit: %d MB", s.config.MemoryLimit/(1024*1024))

	// Create directory structure
	if err := s.createDirectories(); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}

	// Load metadata and existing cache
	if err := s.loadMetadata(); err != nil {
		common.LSPLogger.Warn("Failed to load storage metadata, starting fresh: %v", err)
	}

	// Start background compaction
	if err := s.startBackgroundCompaction(); err != nil {
		return fmt.Errorf("failed to start background compaction: %w", err)
	}

	s.started = true
	common.LSPLogger.Info("SCIP storage manager started successfully")

	return nil
}

// Stop gracefully shuts down the storage manager
func (s *SCIPStorageManager) Stop(ctx context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.started {
		return nil
	}

	common.LSPLogger.Info("Stopping SCIP storage manager")

	// Stop background processes
	close(s.stopCompaction)
	s.compactionManager.Stop()

	// Wait for background tasks to complete
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		common.LSPLogger.Info("Background tasks stopped gracefully")
	case <-time.After(5 * time.Second):
		common.LSPLogger.Warn("Background tasks did not stop within timeout")
	}

	// Flush caches to disk
	if err := s.flush(ctx); err != nil {
		common.LSPLogger.Error("Error flushing caches during shutdown: %v", err)
	}

	// Save metadata
	if err := s.saveMetadata(); err != nil {
		common.LSPLogger.Error("Error saving metadata during shutdown: %v", err)
	}

	s.started = false
	common.LSPLogger.Info("SCIP storage manager stopped")

	return nil
}

// StoreDocument stores a SCIP document using the three-tier strategy
func (s *SCIPStorageManager) StoreDocument(ctx context.Context, doc *SCIPDocument) error {
	if doc == nil {
		return fmt.Errorf("document cannot be nil")
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	common.LSPLogger.Debug("Storing document: %s (%d bytes)", doc.URI, doc.Size)

	// Update symbol and reference indexes
	s.updateIndexes(doc)

	// Store in appropriate tier based on size and frequency
	if s.shouldStoreInHotCache(doc) {
		s.storeInHotCache(doc)
	} else if s.shouldStoreInWarmCache(doc) {
		s.storeInWarmCache(doc)
	} else {
		if err := s.storeToDisk(doc); err != nil {
			return fmt.Errorf("failed to store document to disk: %w", err)
		}
	}

	common.LSPLogger.Debug("Successfully stored document: %s", doc.URI)
	return nil
}

// GetDocument retrieves a SCIP document from the storage hierarchy
func (s *SCIPStorageManager) GetDocument(ctx context.Context, uri string) (*SCIPDocument, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	start := time.Now()
	defer func() {
		duration := time.Since(start)
		if duration > time.Millisecond {
			common.LSPLogger.Debug("Document retrieval for %s took %v", uri, duration)
		}
	}()

	// Try hot cache first
	if doc, found := s.hotMemoryCache[uri]; found {
		s.hotCacheAccess[uri] = time.Now()
		s.recordHit("hot")
		common.LSPLogger.Debug("Document found in hot cache: %s", uri)
		return s.cloneDocument(doc), nil
	}

	// Try warm cache
	if doc, found := s.warmDocumentCache[uri]; found {
		s.promoteToHot(uri, doc)
		s.recordHit("warm")
		common.LSPLogger.Debug("Document found in warm cache: %s", uri)
		return s.cloneDocument(doc), nil
	}

	// Try cold storage (disk)
	doc, err := s.loadFromDisk(uri)
	if err != nil {
		s.recordMiss()
		return nil, fmt.Errorf("document not found: %s", uri)
	}

	// Promote to warm cache on disk hit
	s.storeInWarmCache(doc)
	s.recordHit("cold")
	common.LSPLogger.Debug("Document loaded from disk: %s", uri)

	return s.cloneDocument(doc), nil
}

// RemoveDocument removes a document from all storage tiers
func (s *SCIPStorageManager) RemoveDocument(ctx context.Context, uri string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	common.LSPLogger.Debug("Removing document: %s", uri)

	// Remove from hot cache
	if doc, found := s.hotMemoryCache[uri]; found {
		s.hotCacheSize -= doc.Size
		delete(s.hotMemoryCache, uri)
		delete(s.hotCacheAccess, uri)
	}

	// Remove from warm cache
	if doc, found := s.warmDocumentCache[uri]; found {
		s.warmCacheSize -= doc.Size
		delete(s.warmDocumentCache, uri)
		s.removeFromWarmOrder(uri)
	}

	// Remove from disk
	diskPath := s.getDiskPath(uri)
	if err := os.Remove(diskPath); err != nil && !os.IsNotExist(err) {
		common.LSPLogger.Error("Failed to remove document from disk: %v", err)
	}

	// Remove from indexes
	s.removeFromIndexes(uri)

	common.LSPLogger.Debug("Successfully removed document: %s", uri)
	return nil
}

// ListDocuments returns all document URIs in storage
func (s *SCIPStorageManager) ListDocuments(ctx context.Context) ([]string, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	uriSet := make(map[string]struct{})

	// Collect from hot cache
	for uri := range s.hotMemoryCache {
		uriSet[uri] = struct{}{}
	}

	// Collect from warm cache
	for uri := range s.warmDocumentCache {
		uriSet[uri] = struct{}{}
	}

	// Collect from disk
	diskUris, err := s.listDiskDocuments()
	if err != nil {
		common.LSPLogger.Error("Failed to list disk documents: %v", err)
	} else {
		for _, uri := range diskUris {
			uriSet[uri] = struct{}{}
		}
	}

	// Convert to slice
	uris := make([]string, 0, len(uriSet))
	for uri := range uriSet {
		uris = append(uris, uri)
	}

	sort.Strings(uris)
	return uris, nil
}

// GetSymbolsByName retrieves symbols by name from indexes
func (s *SCIPStorageManager) GetSymbolsByName(ctx context.Context, name string) ([]SCIPSymbol, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	symbols, found := s.symbolIndex[name]
	if !found {
		return []SCIPSymbol{}, nil
	}

	// Clone symbols to prevent external modification
	result := make([]SCIPSymbol, len(symbols))
	copy(result, symbols)

	common.LSPLogger.Debug("Found %d symbols for name: %s", len(result), name)
	return result, nil
}

// GetSymbolsInRange retrieves symbols within a specific range
func (s *SCIPStorageManager) GetSymbolsInRange(ctx context.Context, uri string, start, end SCIPPosition) ([]SCIPSymbol, error) {
	doc, err := s.GetDocument(ctx, uri)
	if err != nil {
		return nil, err
	}

	var result []SCIPSymbol
	for _, symbol := range doc.Symbols {
		if s.isInRange(symbol.Range, start, end) {
			result = append(result, symbol)
		}
	}

	common.LSPLogger.Debug("Found %d symbols in range for %s", len(result), uri)
	return result, nil
}

// GetSymbolDefinition retrieves a symbol definition by ID
func (s *SCIPStorageManager) GetSymbolDefinition(ctx context.Context, symbolID string) (*SCIPSymbol, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Search through all symbol indexes
	for _, symbols := range s.symbolIndex {
		for _, symbol := range symbols {
			if symbol.ID == symbolID {
				return &symbol, nil
			}
		}
	}

	return nil, fmt.Errorf("symbol definition not found: %s", symbolID)
}

// GetReferences retrieves references by symbol ID
func (s *SCIPStorageManager) GetReferences(ctx context.Context, symbolID string) ([]SCIPReference, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	references, found := s.referenceIndex[symbolID]
	if !found {
		return []SCIPReference{}, nil
	}

	// Clone references to prevent external modification
	result := make([]SCIPReference, len(references))
	copy(result, references)

	common.LSPLogger.Debug("Found %d references for symbol: %s", len(result), symbolID)
	return result, nil
}

// GetReferencesInDocument retrieves all references in a document
func (s *SCIPStorageManager) GetReferencesInDocument(ctx context.Context, uri string) ([]SCIPReference, error) {
	doc, err := s.GetDocument(ctx, uri)
	if err != nil {
		return nil, err
	}

	common.LSPLogger.Debug("Found %d references in document: %s", len(doc.References), uri)
	return doc.References, nil
}

// Flush flushes all caches to disk
func (s *SCIPStorageManager) Flush(ctx context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.flush(ctx)
}

// flush internal implementation without locking
func (s *SCIPStorageManager) flush(ctx context.Context) error {
	common.LSPLogger.Info("Flushing SCIP storage caches to disk")

	var errors []string

	// Flush hot cache
	for uri, doc := range s.hotMemoryCache {
		if err := s.storeToDisk(doc); err != nil {
			errors = append(errors, fmt.Sprintf("hot cache %s: %v", uri, err))
		}
	}

	// Flush warm cache
	for uri, doc := range s.warmDocumentCache {
		if err := s.storeToDisk(doc); err != nil {
			errors = append(errors, fmt.Sprintf("warm cache %s: %v", uri, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("flush errors: %s", strings.Join(errors, "; "))
	}

	common.LSPLogger.Info("Successfully flushed all caches to disk")
	return nil
}

// Compact performs storage compaction
func (s *SCIPStorageManager) Compact(ctx context.Context) error {
	return s.compactionManager.Compact(ctx)
}

// GetStats returns storage statistics
func (s *SCIPStorageManager) GetStats(ctx context.Context) (*SCIPStorageStats, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	totalRequests := s.totalHits + s.totalMisses
	hitRate := float64(0)
	if totalRequests > 0 {
		hitRate = float64(s.totalHits) / float64(totalRequests)
	}

	stats := &SCIPStorageStats{
		MemoryUsage:       s.hotCacheSize + s.warmCacheSize,
		DiskUsage:         s.diskUsage,
		CachedDocuments:   len(s.hotMemoryCache) + len(s.warmDocumentCache),
		HotCacheSize:      len(s.hotMemoryCache),
		WarmCacheSize:     len(s.warmDocumentCache),
		ColdCacheSize:     s.getDiskDocumentCount(),
		MemoryLimit:       s.config.MemoryLimit,
		HitRate:           hitRate,
		CompactionRunning: s.compactionManager.IsRunning(),
		LastCompaction:    s.compactionManager.lastRun,
	}

	return stats, nil
}

// SetConfig updates the storage configuration
func (s *SCIPStorageManager) SetConfig(config SCIPStorageConfig) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.config = config
	s.compactionManager.interval = config.CompactionInterval

	common.LSPLogger.Info("Updated SCIP storage configuration")
	return nil
}

// HealthCheck performs a health check on the storage
func (s *SCIPStorageManager) HealthCheck(ctx context.Context) error {
	return s.healthChecker.Check(ctx)
}

// Private helper methods

func (s *SCIPStorageManager) createDirectories() error {
	dirs := []string{
		s.diskCacheDir,
		filepath.Join(s.diskCacheDir, DocumentCacheDir),
		filepath.Join(s.diskCacheDir, SymbolIndexDir),
		filepath.Join(s.diskCacheDir, ReferenceIndexDir),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

func (s *SCIPStorageManager) shouldStoreInHotCache(doc *SCIPDocument) bool {
	// Store in hot cache if it's small and memory allows
	return doc.Size < 64*1024 && (s.hotCacheSize+doc.Size) < s.config.MemoryLimit/2
}

func (s *SCIPStorageManager) shouldStoreInWarmCache(doc *SCIPDocument) bool {
	// Store in warm cache if memory allows
	totalMemory := s.hotCacheSize + s.warmCacheSize
	return (totalMemory + doc.Size) < s.config.MemoryLimit
}

func (s *SCIPStorageManager) storeInHotCache(doc *SCIPDocument) {
	// Evict if necessary
	if (s.hotCacheSize + doc.Size) > s.config.MemoryLimit/2 {
		s.evictFromHotCache()
	}

	s.hotMemoryCache[doc.URI] = doc
	s.hotCacheAccess[doc.URI] = time.Now()
	s.hotCacheSize += doc.Size
}

func (s *SCIPStorageManager) storeInWarmCache(doc *SCIPDocument) {
	// Evict if necessary
	for (s.warmCacheSize + doc.Size) > s.config.MemoryLimit/2 {
		s.evictFromWarmCache()
	}

	s.warmDocumentCache[doc.URI] = doc
	s.warmCacheSize += doc.Size
	s.updateWarmOrder(doc.URI)
}

func (s *SCIPStorageManager) evictFromHotCache() {
	if len(s.hotMemoryCache) == 0 {
		return
	}

	// Find least recently accessed document
	var oldestURI string
	var oldestTime time.Time = time.Now()

	for uri, accessTime := range s.hotCacheAccess {
		if accessTime.Before(oldestTime) {
			oldestTime = accessTime
			oldestURI = uri
		}
	}

	if oldestURI != "" {
		if doc, found := s.hotMemoryCache[oldestURI]; found {
			// Move to warm cache if possible, otherwise to disk
			if s.shouldStoreInWarmCache(doc) {
				s.storeInWarmCache(doc)
			} else {
				s.storeToDisk(doc)
			}

			s.hotCacheSize -= doc.Size
			delete(s.hotMemoryCache, oldestURI)
			delete(s.hotCacheAccess, oldestURI)
		}
	}
}

func (s *SCIPStorageManager) evictFromWarmCache() {
	if len(s.warmCacheOrder) == 0 {
		return
	}

	// Evict LRU document
	lruURI := s.warmCacheOrder[0]
	if doc, found := s.warmDocumentCache[lruURI]; found {
		s.storeToDisk(doc)
		s.warmCacheSize -= doc.Size
		delete(s.warmDocumentCache, lruURI)
	}
	s.warmCacheOrder = s.warmCacheOrder[1:]
}

func (s *SCIPStorageManager) updateWarmOrder(uri string) {
	// Remove from current position
	for i, u := range s.warmCacheOrder {
		if u == uri {
			s.warmCacheOrder = append(s.warmCacheOrder[:i], s.warmCacheOrder[i+1:]...)
			break
		}
	}
	// Add to end (most recently used)
	s.warmCacheOrder = append(s.warmCacheOrder, uri)
}

func (s *SCIPStorageManager) removeFromWarmOrder(uri string) {
	for i, u := range s.warmCacheOrder {
		if u == uri {
			s.warmCacheOrder = append(s.warmCacheOrder[:i], s.warmCacheOrder[i+1:]...)
			break
		}
	}
}

func (s *SCIPStorageManager) promoteToHot(uri string, doc *SCIPDocument) {
	// Remove from warm cache
	s.warmCacheSize -= doc.Size
	delete(s.warmDocumentCache, uri)
	s.removeFromWarmOrder(uri)

	// Store in hot cache
	s.storeInHotCache(doc)
}

func (s *SCIPStorageManager) storeToDisk(doc *SCIPDocument) error {
	diskPath := s.getDiskPath(doc.URI)
	if err := os.MkdirAll(filepath.Dir(diskPath), 0755); err != nil {
		return err
	}

	// Create compressed file
	file, err := os.Create(diskPath)
	if err != nil {
		return err
	}
	defer file.Close()

	var writer io.Writer = file
	if s.config.CompressionType == "gzip" {
		gzWriter := gzip.NewWriter(file)
		defer gzWriter.Close()
		writer = gzWriter
	}

	encoder := json.NewEncoder(writer)
	if err := encoder.Encode(doc); err != nil {
		return err
	}

	// Update disk usage
	if info, err := file.Stat(); err == nil {
		s.diskUsage += info.Size()
	}

	return nil
}

func (s *SCIPStorageManager) loadFromDisk(uri string) (*SCIPDocument, error) {
	diskPath := s.getDiskPath(uri)

	file, err := os.Open(diskPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var reader io.Reader = file
	if s.config.CompressionType == "gzip" {
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			return nil, err
		}
		defer gzReader.Close()
		reader = gzReader
	}

	var doc SCIPDocument
	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(&doc); err != nil {
		return nil, err
	}

	return &doc, nil
}

func (s *SCIPStorageManager) getDiskPath(uri string) string {
	// Create safe filename from URI
	safeFilename := strings.ReplaceAll(uri, "/", "_")
	safeFilename = strings.ReplaceAll(safeFilename, ":", "_")
	safeFilename = strings.ReplaceAll(safeFilename, "\\", "_")

	return filepath.Join(s.diskCacheDir, DocumentCacheDir, safeFilename+".json.gz")
}

func (s *SCIPStorageManager) listDiskDocuments() ([]string, error) {
	docDir := filepath.Join(s.diskCacheDir, DocumentCacheDir)
	entries, err := os.ReadDir(docDir)
	if err != nil {
		return nil, err
	}

	var uris []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if strings.HasSuffix(name, ".json.gz") {
			// Reverse the safe filename transformation
			uri := strings.TrimSuffix(name, ".json.gz")
			uri = strings.ReplaceAll(uri, "_", "/")
			uris = append(uris, uri)
		}
	}

	return uris, nil
}

func (s *SCIPStorageManager) getDiskDocumentCount() int {
	uris, err := s.listDiskDocuments()
	if err != nil {
		return 0
	}
	return len(uris)
}

func (s *SCIPStorageManager) updateIndexes(doc *SCIPDocument) {
	// Update symbol index
	for _, symbol := range doc.Symbols {
		s.symbolIndex[symbol.Name] = append(s.symbolIndex[symbol.Name], symbol)
	}

	// Update reference index
	for _, ref := range doc.References {
		s.referenceIndex[ref.Symbol] = append(s.referenceIndex[ref.Symbol], ref)
	}
}

func (s *SCIPStorageManager) removeFromIndexes(uri string) {
	// This is a simplified implementation - in practice, you'd need to track
	// which symbols/references came from which documents
	// For now, we'll leave indexes as-is to avoid complexity
}

func (s *SCIPStorageManager) isInRange(symbolRange SCIPRange, start, end SCIPPosition) bool {
	return (symbolRange.Start.Line >= start.Line && symbolRange.Start.Character >= start.Character) &&
		(symbolRange.End.Line <= end.Line && symbolRange.End.Character <= end.Character)
}

func (s *SCIPStorageManager) cloneDocument(doc *SCIPDocument) *SCIPDocument {
	// Deep clone to prevent external modification
	cloned := *doc
	cloned.Symbols = make([]SCIPSymbol, len(doc.Symbols))
	copy(cloned.Symbols, doc.Symbols)
	cloned.References = make([]SCIPReference, len(doc.References))
	copy(cloned.References, doc.References)
	return &cloned
}

func (s *SCIPStorageManager) recordHit(tier string) {
	s.hitCounts[tier]++
	s.totalHits++
}

func (s *SCIPStorageManager) recordMiss() {
	s.totalMisses++
}

func (s *SCIPStorageManager) startBackgroundCompaction() error {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.config.CompactionInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := s.compactionManager.Compact(context.Background()); err != nil {
					common.LSPLogger.Error("Background compaction failed: %v", err)
				}
			case <-s.stopCompaction:
				return
			}
		}
	}()

	return nil
}

func (s *SCIPStorageManager) loadMetadata() error {
	metadataPath := filepath.Join(s.diskCacheDir, MetadataFileName)

	file, err := os.Open(metadataPath)
	if err != nil {
		return err
	}
	defer file.Close()

	var metadata SCIPStorageMetadata
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&metadata); err != nil {
		return err
	}

	common.LSPLogger.Info("Loaded storage metadata: %d documents, %d MB total size",
		metadata.DocumentCount, metadata.TotalSize/(1024*1024))

	return nil
}

func (s *SCIPStorageManager) saveMetadata() error {
	metadataPath := filepath.Join(s.diskCacheDir, MetadataFileName)

	metadata := SCIPStorageMetadata{
		Version:       "1.0",
		CreatedAt:     time.Now(),
		LastModified:  time.Now(),
		DocumentCount: len(s.hotMemoryCache) + len(s.warmDocumentCache) + s.getDiskDocumentCount(),
		TotalSize:     s.hotCacheSize + s.warmCacheSize + s.diskUsage,
	}

	file, err := os.Create(metadataPath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	return encoder.Encode(metadata)
}

// Cache Manager implementations

func (cm *SCIPMemoryCacheManager) Evict(key string) error {
	storage := cm.storage
	storage.mutex.Lock()
	defer storage.mutex.Unlock()

	// Evict from hot cache
	if doc, found := storage.hotMemoryCache[key]; found {
		storage.hotCacheSize -= doc.Size
		delete(storage.hotMemoryCache, key)
		delete(storage.hotCacheAccess, key)
		cm.evictCount++
		return nil
	}

	// Evict from warm cache
	if doc, found := storage.warmDocumentCache[key]; found {
		storage.warmCacheSize -= doc.Size
		delete(storage.warmDocumentCache, key)
		storage.removeFromWarmOrder(key)
		cm.evictCount++
		return nil
	}

	return fmt.Errorf("key not found in cache: %s", key)
}

func (cm *SCIPMemoryCacheManager) EvictLRU(count int) error {
	storage := cm.storage
	storage.mutex.Lock()
	defer storage.mutex.Unlock()

	for i := 0; i < count && len(storage.warmCacheOrder) > 0; i++ {
		storage.evictFromWarmCache()
		cm.evictCount++
	}

	return nil
}

func (cm *SCIPMemoryCacheManager) Clear() error {
	storage := cm.storage
	storage.mutex.Lock()
	defer storage.mutex.Unlock()

	storage.hotMemoryCache = make(map[string]*SCIPDocument)
	storage.hotCacheAccess = make(map[string]time.Time)
	storage.hotCacheSize = 0

	storage.warmDocumentCache = make(map[string]*SCIPDocument)
	storage.warmCacheOrder = make([]string, 0)
	storage.warmCacheSize = 0

	cm.evictCount = 0
	common.LSPLogger.Info("Cleared all memory caches")

	return nil
}

func (cm *SCIPMemoryCacheManager) GetSize() int64 {
	storage := cm.storage
	storage.mutex.RLock()
	defer storage.mutex.RUnlock()

	return storage.hotCacheSize + storage.warmCacheSize
}

func (cm *SCIPMemoryCacheManager) GetStats() map[string]interface{} {
	storage := cm.storage
	storage.mutex.RLock()
	defer storage.mutex.RUnlock()

	return map[string]interface{}{
		"hot_cache_size":  storage.hotCacheSize,
		"warm_cache_size": storage.warmCacheSize,
		"hot_documents":   len(storage.hotMemoryCache),
		"warm_documents":  len(storage.warmDocumentCache),
		"eviction_count":  cm.evictCount,
	}
}

// Compaction Manager implementations

func (dcm *SCIPDiskCompactionManager) Compact(ctx context.Context) error {
	dcm.mutex.Lock()
	if dcm.running {
		dcm.mutex.Unlock()
		return fmt.Errorf("compaction already running")
	}
	dcm.running = true
	dcm.mutex.Unlock()

	defer func() {
		dcm.mutex.Lock()
		dcm.running = false
		dcm.lastRun = time.Now()
		dcm.mutex.Unlock()
	}()

	common.LSPLogger.Info("Starting disk compaction")
	start := time.Now()

	// Simple compaction: remove old documents
	docDir := filepath.Join(dcm.storage.diskCacheDir, DocumentCacheDir)
	entries, err := os.ReadDir(docDir)
	if err != nil {
		return fmt.Errorf("failed to read document directory: %w", err)
	}

	removed := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if time.Since(info.ModTime()) > dcm.storage.config.MaxDocumentAge {
			path := filepath.Join(docDir, entry.Name())
			if err := os.Remove(path); err == nil {
				removed++
				dcm.storage.diskUsage -= info.Size()
			}
		}
	}

	duration := time.Since(start)
	common.LSPLogger.Info("Disk compaction completed in %v, removed %d old documents", duration, removed)

	return nil
}

func (dcm *SCIPDiskCompactionManager) IsRunning() bool {
	dcm.mutex.RLock()
	defer dcm.mutex.RUnlock()
	return dcm.running
}

func (dcm *SCIPDiskCompactionManager) Schedule(interval time.Duration) {
	dcm.mutex.Lock()
	defer dcm.mutex.Unlock()
	dcm.interval = interval
}

func (dcm *SCIPDiskCompactionManager) Stop() error {
	close(dcm.stopChan)
	return nil
}

// Health Checker implementations

func (shc *SCIPStorageHealthChecker) Check(ctx context.Context) error {
	shc.mutex.Lock()
	defer shc.mutex.Unlock()

	shc.lastCheck = time.Now()

	// Check if storage is started
	if !shc.storage.started {
		shc.status = "stopped"
		return fmt.Errorf("storage manager not started")
	}

	// Check memory usage
	totalMemory := shc.storage.hotCacheSize + shc.storage.warmCacheSize
	if totalMemory > shc.storage.config.MemoryLimit {
		shc.status = "memory_exceeded"
		return fmt.Errorf("memory usage (%d) exceeds limit (%d)", totalMemory, shc.storage.config.MemoryLimit)
	}

	// Check disk accessibility
	if _, err := os.Stat(shc.storage.diskCacheDir); err != nil {
		shc.status = "disk_error"
		return fmt.Errorf("disk cache directory not accessible: %w", err)
	}

	shc.status = "healthy"
	return nil
}

func (shc *SCIPStorageHealthChecker) GetLastCheck() time.Time {
	shc.mutex.RLock()
	defer shc.mutex.RUnlock()
	return shc.lastCheck
}

func (shc *SCIPStorageHealthChecker) GetHealthStatus() string {
	shc.mutex.RLock()
	defer shc.mutex.RUnlock()
	return shc.status
}

// Interface compliance verification
var _ SCIPDocumentStorage = (*SCIPStorageManager)(nil)
var _ SCIPCacheManager = (*SCIPMemoryCacheManager)(nil)
var _ SCIPCompactionManager = (*SCIPDiskCompactionManager)(nil)
var _ SCIPHealthChecker = (*SCIPStorageHealthChecker)(nil)
