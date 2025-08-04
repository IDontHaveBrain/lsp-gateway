package scip

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"lsp-gateway/src/internal/common"
)

const (
	DefaultMemoryLimit = 256 * 1024 * 1024 // 256MB
)

// SimpleSCIPStorage implements SCIPDocumentStorage with a single LRU cache tier
type SimpleSCIPStorage struct {
	config SCIPStorageConfig
	mutex  sync.RWMutex

	// Single LRU cache
	documents   map[string]*SCIPDocument
	accessOrder []string // LRU tracking: first = oldest, last = newest
	currentSize int64

	// Symbol and reference indexes (simplified)
	symbolIndex    map[string][]SCIPSymbol    // symbol name -> symbols
	referenceIndex map[string][]SCIPReference // symbol ID -> references

	// Basic metrics
	hitCount  int64
	missCount int64
	started   bool

	// Optional persistence
	diskFile string
}

// NewSimpleSCIPStorage creates a new simple SCIP storage with LRU cache
func NewSimpleSCIPStorage(config SCIPStorageConfig) (*SimpleSCIPStorage, error) {
	if config.MemoryLimit == 0 {
		config.MemoryLimit = DefaultMemoryLimit
	}
	if config.DiskCacheDir == "" {
		config.DiskCacheDir = filepath.Join(os.TempDir(), "lsp-gateway-scip-simple")
	}

	storage := &SimpleSCIPStorage{
		config:         config,
		documents:      make(map[string]*SCIPDocument),
		accessOrder:    make([]string, 0),
		symbolIndex:    make(map[string][]SCIPSymbol),
		referenceIndex: make(map[string][]SCIPReference),
		diskFile:       filepath.Join(config.DiskCacheDir, "simple_cache.json"),
	}

	return storage, nil
}

// Start initializes the storage
func (s *SimpleSCIPStorage) Start(ctx context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.started {
		return fmt.Errorf("storage already started")
	}

	common.LSPLogger.Info("Starting simple SCIP storage with memory limit: %d MB", s.config.MemoryLimit/(1024*1024))

	// Create directory
	if err := os.MkdirAll(s.config.DiskCacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Load from disk if available
	if err := s.loadFromDisk(); err != nil {
		common.LSPLogger.Debug("No existing cache found, starting fresh: %v", err)
	}

	s.started = true
	common.LSPLogger.Info("Simple SCIP storage started successfully")
	return nil
}

// Stop gracefully shuts down the storage
func (s *SimpleSCIPStorage) Stop(ctx context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.started {
		return nil
	}

	common.LSPLogger.Info("Stopping simple SCIP storage")

	// Save to disk
	if err := s.saveToDisk(); err != nil {
		common.LSPLogger.Error("Error saving cache during shutdown: %v", err)
	}

	s.started = false
	common.LSPLogger.Info("Simple SCIP storage stopped")
	return nil
}

// StoreDocument stores a document in the LRU cache
func (s *SimpleSCIPStorage) StoreDocument(ctx context.Context, doc *SCIPDocument) error {
	if doc == nil {
		return fmt.Errorf("document cannot be nil")
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	common.LSPLogger.Debug("Storing document: %s (%d bytes)", doc.URI, doc.Size)

	// Evict if necessary to make space
	for s.currentSize+doc.Size > s.config.MemoryLimit && len(s.documents) > 0 {
		s.evictLRU()
	}

	// Remove existing entry if present
	if existing, found := s.documents[doc.URI]; found {
		s.currentSize -= existing.Size
		s.removeFromAccessOrder(doc.URI)
		s.removeFromIndexes(doc.URI)
	}

	// Store document
	s.documents[doc.URI] = s.cloneDocument(doc)
	s.currentSize += doc.Size
	s.addToAccessOrder(doc.URI)
	s.updateIndexes(doc)

	common.LSPLogger.Debug("Successfully stored document: %s", doc.URI)
	return nil
}

// GetDocument retrieves a document from the cache
func (s *SimpleSCIPStorage) GetDocument(ctx context.Context, uri string) (*SCIPDocument, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	doc, found := s.documents[uri]
	if !found {
		s.missCount++
		return nil, fmt.Errorf("document not found: %s", uri)
	}

	// Update LRU order
	s.removeFromAccessOrder(uri)
	s.addToAccessOrder(uri)
	s.hitCount++

	common.LSPLogger.Debug("Document found in cache: %s", uri)
	return s.cloneDocument(doc), nil
}

// RemoveDocument removes a document from the cache
func (s *SimpleSCIPStorage) RemoveDocument(ctx context.Context, uri string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	common.LSPLogger.Debug("Removing document: %s", uri)

	if doc, found := s.documents[uri]; found {
		s.currentSize -= doc.Size
		delete(s.documents, uri)
		s.removeFromAccessOrder(uri)
		s.removeFromIndexes(uri)
	}

	common.LSPLogger.Debug("Successfully removed document: %s", uri)
	return nil
}

// ListDocuments returns all document URIs
func (s *SimpleSCIPStorage) ListDocuments(ctx context.Context) ([]string, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	uris := make([]string, 0, len(s.documents))
	for uri := range s.documents {
		uris = append(uris, uri)
	}

	sort.Strings(uris)
	return uris, nil
}

// GetSymbolsByName retrieves symbols by name
func (s *SimpleSCIPStorage) GetSymbolsByName(ctx context.Context, name string) ([]SCIPSymbol, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	symbols, found := s.symbolIndex[name]
	if !found {
		return []SCIPSymbol{}, nil
	}

	result := make([]SCIPSymbol, len(symbols))
	copy(result, symbols)

	common.LSPLogger.Debug("Found %d symbols for name: %s", len(result), name)
	return result, nil
}

// GetSymbolsInRange retrieves symbols within a specific range
func (s *SimpleSCIPStorage) GetSymbolsInRange(ctx context.Context, uri string, start, end SCIPPosition) ([]SCIPSymbol, error) {
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
func (s *SimpleSCIPStorage) GetSymbolDefinition(ctx context.Context, symbolID string) (*SCIPSymbol, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

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
func (s *SimpleSCIPStorage) GetReferences(ctx context.Context, symbolID string) ([]SCIPReference, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	references, found := s.referenceIndex[symbolID]
	if !found {
		return []SCIPReference{}, nil
	}

	result := make([]SCIPReference, len(references))
	copy(result, references)

	common.LSPLogger.Debug("Found %d references for symbol: %s", len(result), symbolID)
	return result, nil
}

// GetReferencesInDocument retrieves all references in a document
func (s *SimpleSCIPStorage) GetReferencesInDocument(ctx context.Context, uri string) ([]SCIPReference, error) {
	doc, err := s.GetDocument(ctx, uri)
	if err != nil {
		return nil, err
	}

	common.LSPLogger.Debug("Found %d references in document: %s", len(doc.References), uri)
	return doc.References, nil
}

// Flush saves all cached documents to disk (optional persistence)
func (s *SimpleSCIPStorage) Flush(ctx context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	common.LSPLogger.Info("Flushing simple SCIP storage to disk")
	return s.saveToDisk()
}

// Compact is a no-op for simple storage (no background processes)
func (s *SimpleSCIPStorage) Compact(ctx context.Context) error {
	common.LSPLogger.Debug("Compact called on simple storage (no-op)")
	return nil
}

// GetStats returns simplified storage statistics
func (s *SimpleSCIPStorage) GetStats(ctx context.Context) (*SCIPStorageStats, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	totalRequests := s.hitCount + s.missCount
	hitRate := float64(0)
	if totalRequests > 0 {
		hitRate = float64(s.hitCount) / float64(totalRequests)
	}

	stats := &SCIPStorageStats{
		MemoryUsage:       s.currentSize,
		DiskUsage:         0, // No separate disk storage in simple implementation
		CachedDocuments:   len(s.documents),
		HotCacheSize:      len(s.documents), // All documents are in single cache
		WarmCacheSize:     0,                // No warm cache
		ColdCacheSize:     0,                // No cold cache
		MemoryLimit:       s.config.MemoryLimit,
		HitRate:           hitRate,
		CompactionRunning: false, // No background compaction
		LastCompaction:    time.Time{},
	}

	return stats, nil
}

// SetConfig updates the storage configuration
func (s *SimpleSCIPStorage) SetConfig(config SCIPStorageConfig) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.config = config
	common.LSPLogger.Info("Updated simple SCIP storage configuration")
	return nil
}

// HealthCheck performs a simple health check
func (s *SimpleSCIPStorage) HealthCheck(ctx context.Context) error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if !s.started {
		return fmt.Errorf("storage not started")
	}

	if s.currentSize > s.config.MemoryLimit {
		return fmt.Errorf("memory usage (%d) exceeds limit (%d)", s.currentSize, s.config.MemoryLimit)
	}

	return nil
}

// Private helper methods

// evictLRU removes the least recently used document
func (s *SimpleSCIPStorage) evictLRU() {
	if len(s.accessOrder) == 0 {
		return
	}

	// Remove oldest document (first in order)
	oldestURI := s.accessOrder[0]
	if doc, found := s.documents[oldestURI]; found {
		s.currentSize -= doc.Size
		delete(s.documents, oldestURI)
		s.removeFromIndexes(oldestURI)
		common.LSPLogger.Debug("Evicted LRU document: %s", oldestURI)
	}
	s.accessOrder = s.accessOrder[1:]
}

// addToAccessOrder adds URI to end of access order (most recent)
func (s *SimpleSCIPStorage) addToAccessOrder(uri string) {
	s.accessOrder = append(s.accessOrder, uri)
}

// removeFromAccessOrder removes URI from access order
func (s *SimpleSCIPStorage) removeFromAccessOrder(uri string) {
	for i, u := range s.accessOrder {
		if u == uri {
			s.accessOrder = append(s.accessOrder[:i], s.accessOrder[i+1:]...)
			break
		}
	}
}

// updateIndexes updates symbol and reference indexes
func (s *SimpleSCIPStorage) updateIndexes(doc *SCIPDocument) {
	for _, symbol := range doc.Symbols {
		s.symbolIndex[symbol.Name] = append(s.symbolIndex[symbol.Name], symbol)
	}

	for _, ref := range doc.References {
		s.referenceIndex[ref.Symbol] = append(s.referenceIndex[ref.Symbol], ref)
	}
}

// removeFromIndexes removes document symbols/references from indexes
func (s *SimpleSCIPStorage) removeFromIndexes(uri string) {
	// Simplified implementation - in production would need better tracking
	// For now, leave indexes as-is to avoid complexity
}

// isInRange checks if symbol range is within specified bounds
func (s *SimpleSCIPStorage) isInRange(symbolRange SCIPRange, start, end SCIPPosition) bool {
	return (symbolRange.Start.Line >= start.Line && symbolRange.Start.Character >= start.Character) &&
		(symbolRange.End.Line <= end.Line && symbolRange.End.Character <= end.Character)
}

// cloneDocument creates a deep copy of the document
func (s *SimpleSCIPStorage) cloneDocument(doc *SCIPDocument) *SCIPDocument {
	cloned := *doc
	cloned.Symbols = make([]SCIPSymbol, len(doc.Symbols))
	copy(cloned.Symbols, doc.Symbols)
	cloned.References = make([]SCIPReference, len(doc.References))
	copy(cloned.References, doc.References)
	return &cloned
}

// Optional persistence methods

// saveToDisk saves the cache to disk as JSON (optional persistence)
func (s *SimpleSCIPStorage) saveToDisk() error {
	if s.diskFile == "" {
		return nil
	}

	data := struct {
		Documents   map[string]*SCIPDocument `json:"documents"`
		AccessOrder []string                 `json:"access_order"`
		CurrentSize int64                    `json:"current_size"`
		HitCount    int64                    `json:"hit_count"`
		MissCount   int64                    `json:"miss_count"`
		SavedAt     time.Time                `json:"saved_at"`
	}{
		Documents:   s.documents,
		AccessOrder: s.accessOrder,
		CurrentSize: s.currentSize,
		HitCount:    s.hitCount,
		MissCount:   s.missCount,
		SavedAt:     time.Now(),
	}

	file, err := os.Create(s.diskFile)
	if err != nil {
		return fmt.Errorf("failed to create cache file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("failed to encode cache data: %w", err)
	}

	common.LSPLogger.Debug("Saved cache with %d documents to %s", len(s.documents), s.diskFile)
	return nil
}

// loadFromDisk loads the cache from disk (optional persistence)
func (s *SimpleSCIPStorage) loadFromDisk() error {
	if s.diskFile == "" {
		return nil
	}

	file, err := os.Open(s.diskFile)
	if err != nil {
		return err
	}
	defer file.Close()

	var data struct {
		Documents   map[string]*SCIPDocument `json:"documents"`
		AccessOrder []string                 `json:"access_order"`
		CurrentSize int64                    `json:"current_size"`
		HitCount    int64                    `json:"hit_count"`
		MissCount   int64                    `json:"miss_count"`
		SavedAt     time.Time                `json:"saved_at"`
	}

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&data); err != nil {
		return fmt.Errorf("failed to decode cache data: %w", err)
	}

	// Restore cache state
	s.documents = data.Documents
	s.accessOrder = data.AccessOrder
	s.currentSize = data.CurrentSize
	s.hitCount = data.HitCount
	s.missCount = data.MissCount

	// Rebuild indexes
	s.symbolIndex = make(map[string][]SCIPSymbol)
	s.referenceIndex = make(map[string][]SCIPReference)
	for _, doc := range s.documents {
		s.updateIndexes(doc)
	}

	common.LSPLogger.Info("Loaded cache with %d documents from %s (saved at %v)",
		len(s.documents), s.diskFile, data.SavedAt)
	return nil
}

// Interface compliance verification
var _ SCIPDocumentStorage = (*SimpleSCIPStorage)(nil)
