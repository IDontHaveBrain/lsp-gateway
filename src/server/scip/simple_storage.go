package scip

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/types"
)

const (
	DefaultMemoryLimit = 256 * 1024 * 1024 // 256MB
)

// SimpleSCIPStorage implements SCIPDocumentStorage with occurrence-centric architecture
type SimpleSCIPStorage struct {
	config SCIPStorageConfig
	mutex  sync.RWMutex

	// Single LRU cache for documents
	documents   map[string]*SCIPDocument
	accessOrder []string // LRU tracking: first = oldest, last = newest
	currentSize int64

	// Occurrence-centric indexes for fast lookup
	occurrencesByURI    map[string][]SCIPOccurrence         // document URI -> occurrences
	occurrencesBySymbol map[string][]SCIPOccurrence         // symbol ID -> occurrences across all documents
	definitionIndex     map[string]*SCIPOccurrence          // symbol ID -> definition occurrence
	referenceIndex      map[string][]SCIPOccurrence         // symbol ID -> reference occurrences
	symbolInfoIndex     map[string]*SCIPSymbolInformation   // symbol ID -> symbol information
	symbolNameIndex     map[string][]*SCIPSymbolInformation // symbol name -> symbol information list
	relationshipIndex   map[string][]SCIPRelationship       // symbol ID -> relationships
	documentIndex       map[string][]string                 // document URI -> symbol IDs in document

	// Basic metrics
	hitCount  int64
	missCount int64
	started   bool

	// Optional persistence
	diskFile string

	// SCIP index for workspace operations
	scipIndex *SCIPIndex
}

// NewSimpleSCIPStorage creates a new simple SCIP storage with occurrence-centric indexes
func NewSimpleSCIPStorage(config SCIPStorageConfig) (*SimpleSCIPStorage, error) {
	if config.MemoryLimit == 0 {
		config.MemoryLimit = DefaultMemoryLimit
	}
	if config.DiskCacheDir == "" {
		config.DiskCacheDir = filepath.Join(os.TempDir(), "lsp-gateway-scip-simple")
	}

	storage := &SimpleSCIPStorage{
		config:              config,
		documents:           make(map[string]*SCIPDocument),
		accessOrder:         make([]string, 0),
		occurrencesByURI:    make(map[string][]SCIPOccurrence),
		occurrencesBySymbol: make(map[string][]SCIPOccurrence),
		definitionIndex:     make(map[string]*SCIPOccurrence),
		referenceIndex:      make(map[string][]SCIPOccurrence),
		symbolInfoIndex:     make(map[string]*SCIPSymbolInformation),
		symbolNameIndex:     make(map[string][]*SCIPSymbolInformation),
		relationshipIndex:   make(map[string][]SCIPRelationship),
		documentIndex:       make(map[string][]string),
		diskFile:            filepath.Join(config.DiskCacheDir, "simple_cache.json"),
		scipIndex:           &SCIPIndex{},
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

// StoreDocument stores a document with occurrence-centric indexing
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
		s.removeFromOccurrenceIndexes(doc.URI)
	}

	// Store document
	s.documents[doc.URI] = s.cloneDocument(doc)
	s.currentSize += doc.Size
	s.addToAccessOrder(doc.URI)
	s.updateOccurrenceIndexes(doc)

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
		s.removeFromOccurrenceIndexes(uri)
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

// Occurrence operations - core occurrence-centric queries

// GetOccurrences retrieves all occurrences in a document
func (s *SimpleSCIPStorage) GetOccurrences(ctx context.Context, uri string) ([]SCIPOccurrence, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	occurrences, found := s.occurrencesByURI[uri]
	if !found {
		return []SCIPOccurrence{}, nil
	}

	result := make([]SCIPOccurrence, len(occurrences))
	copy(result, occurrences)

	common.LSPLogger.Debug("Found %d occurrences in document: %s", len(result), uri)
	return result, nil
}

// GetOccurrencesInRange retrieves occurrences within a specific range
func (s *SimpleSCIPStorage) GetOccurrencesInRange(ctx context.Context, uri string, start, end types.Position) ([]SCIPOccurrence, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	occurrences, found := s.occurrencesByURI[uri]
	if !found {
		return []SCIPOccurrence{}, nil
	}

	var result []SCIPOccurrence
	for _, occ := range occurrences {
		if s.isPositionInRange(occ.Range, start, end) {
			result = append(result, occ)
		}
	}

	common.LSPLogger.Debug("Found %d occurrences in range for %s", len(result), uri)
	return result, nil
}

// GetOccurrencesBySymbol retrieves all occurrences of a specific symbol across documents
func (s *SimpleSCIPStorage) GetOccurrencesBySymbol(ctx context.Context, symbolID string) ([]SCIPOccurrence, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	occurrences, found := s.occurrencesBySymbol[symbolID]
	if !found {
		return []SCIPOccurrence{}, nil
	}

	result := make([]SCIPOccurrence, len(occurrences))
	copy(result, occurrences)

	common.LSPLogger.Debug("Found %d occurrences for symbol: %s", len(result), symbolID)
	return result, nil
}

// GetDefinitionOccurrence retrieves the definition occurrence of a symbol
func (s *SimpleSCIPStorage) GetDefinitionOccurrence(ctx context.Context, symbolID string) (*SCIPOccurrence, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	definition, found := s.definitionIndex[symbolID]
	if !found {
		return nil, fmt.Errorf("definition not found for symbol: %s", symbolID)
	}

	common.LSPLogger.Debug("Found definition for symbol: %s", symbolID)
	result := *definition
	return &result, nil
}

// GetReferenceOccurrences retrieves all reference occurrences of a symbol
func (s *SimpleSCIPStorage) GetReferenceOccurrences(ctx context.Context, symbolID string) ([]SCIPOccurrence, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	references, found := s.referenceIndex[symbolID]
	if !found {
		return []SCIPOccurrence{}, nil
	}

	result := make([]SCIPOccurrence, len(references))
	copy(result, references)

	common.LSPLogger.Debug("Found %d reference occurrences for symbol: %s", len(result), symbolID)
	return result, nil
}

// Symbol information operations - metadata about symbols

// GetSymbolInformation retrieves symbol information by symbol ID
func (s *SimpleSCIPStorage) GetSymbolInformation(ctx context.Context, symbolID string) (*SCIPSymbolInformation, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	info, found := s.symbolInfoIndex[symbolID]
	if !found {
		return nil, fmt.Errorf("symbol information not found: %s", symbolID)
	}

	common.LSPLogger.Debug("Found symbol information for: %s", symbolID)
	result := *info
	return &result, nil
}

// GetSymbolInformationByName retrieves symbol information by name
func (s *SimpleSCIPStorage) GetSymbolInformationByName(ctx context.Context, name string) ([]SCIPSymbolInformation, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	infos, found := s.symbolNameIndex[name]
	if !found {
		return []SCIPSymbolInformation{}, nil
	}

	result := make([]SCIPSymbolInformation, len(infos))
	for i, info := range infos {
		result[i] = *info
	}

	common.LSPLogger.Debug("Found %d symbol information entries for name: %s", len(result), name)
	return result, nil
}

// StoreSymbolInformation stores symbol information
func (s *SimpleSCIPStorage) StoreSymbolInformation(ctx context.Context, info *SCIPSymbolInformation) error {
	if info == nil {
		return fmt.Errorf("symbol information cannot be nil")
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Store in symbol info index
	s.symbolInfoIndex[info.Symbol] = info

	// Store in symbol name index
	s.symbolNameIndex[info.DisplayName] = append(s.symbolNameIndex[info.DisplayName], info)

	common.LSPLogger.Debug("Stored symbol information for: %s", info.Symbol)
	return nil
}

// Relationship operations - symbol relationships

// GetSymbolRelationships retrieves relationships for a symbol
func (s *SimpleSCIPStorage) GetSymbolRelationships(ctx context.Context, symbolID string) ([]SCIPRelationship, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	relationships, found := s.relationshipIndex[symbolID]
	if !found {
		return []SCIPRelationship{}, nil
	}

	result := make([]SCIPRelationship, len(relationships))
	copy(result, relationships)

	common.LSPLogger.Debug("Found %d relationships for symbol: %s", len(result), symbolID)
	return result, nil
}

// GetImplementations finds implementation occurrences for a symbol
func (s *SimpleSCIPStorage) GetImplementations(ctx context.Context, symbolID string) ([]SCIPOccurrence, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var implementations []SCIPOccurrence
	if relationships, found := s.relationshipIndex[symbolID]; found {
		for _, rel := range relationships {
			if rel.IsImplementation {
				if occurrences, occFound := s.occurrencesBySymbol[rel.Symbol]; occFound {
					for _, occ := range occurrences {
						if occ.SymbolRoles.HasRole(types.SymbolRoleDefinition) {
							implementations = append(implementations, occ)
						}
					}
				}
			}
		}
	}

	common.LSPLogger.Debug("Found %d implementations for symbol: %s", len(implementations), symbolID)
	return implementations, nil
}

// GetTypeDefinition finds type definition occurrence for a symbol
func (s *SimpleSCIPStorage) GetTypeDefinition(ctx context.Context, symbolID string) (*SCIPOccurrence, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if relationships, found := s.relationshipIndex[symbolID]; found {
		for _, rel := range relationships {
			if rel.IsTypeDefinition {
				if definition, defFound := s.definitionIndex[rel.Symbol]; defFound {
					common.LSPLogger.Debug("Found type definition for symbol: %s", symbolID)
					result := *definition
					return &result, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("type definition not found for symbol: %s", symbolID)
}

// Search operations - finding symbols across documents

// SearchSymbols searches symbols by name pattern
func (s *SimpleSCIPStorage) SearchSymbols(ctx context.Context, query string, limit int) ([]SCIPSymbolInformation, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var results []SCIPSymbolInformation
	pattern, err := s.compileSearchPattern(query)
	if err != nil {
		return nil, fmt.Errorf("invalid search pattern: %w", err)
	}

	for _, info := range s.symbolInfoIndex {
		if pattern.MatchString(info.DisplayName) {
			results = append(results, *info)
			if len(results) >= limit {
				break
			}
		}
	}

	common.LSPLogger.Debug("Found %d symbols matching query: %s", len(results), query)
	return results, nil
}

// SearchOccurrences searches occurrences by symbol pattern
func (s *SimpleSCIPStorage) SearchOccurrences(ctx context.Context, symbolPattern string, limit int) ([]SCIPOccurrence, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var results []SCIPOccurrence
	pattern, err := s.compileSearchPattern(symbolPattern)
	if err != nil {
		return nil, fmt.Errorf("invalid symbol pattern: %w", err)
	}

	for symbolID, occurrences := range s.occurrencesBySymbol {
		if pattern.MatchString(symbolID) {
			for _, occ := range occurrences {
				results = append(results, occ)
				if len(results) >= limit {
					return results, nil
				}
			}
		}
	}

	common.LSPLogger.Debug("Found %d occurrences matching pattern: %s", len(results), symbolPattern)
	return results, nil
}

// Workspace operations - project-level queries

// GetWorkspaceSymbols retrieves workspace symbols matching query
func (s *SimpleSCIPStorage) GetWorkspaceSymbols(ctx context.Context, query string) ([]SCIPSymbolInformation, error) {
	return s.SearchSymbols(ctx, query, 100) // Default limit of 100 for workspace symbols
}

// GetDocumentSymbols retrieves all symbols defined in a document
func (s *SimpleSCIPStorage) GetDocumentSymbols(ctx context.Context, uri string) ([]SCIPSymbolInformation, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	symbolIDs, found := s.documentIndex[uri]
	if !found {
		return []SCIPSymbolInformation{}, nil
	}

	var results []SCIPSymbolInformation
	for _, symbolID := range symbolIDs {
		if info, infoFound := s.symbolInfoIndex[symbolID]; infoFound {
			results = append(results, *info)
		}
	}

	common.LSPLogger.Debug("Found %d document symbols in: %s", len(results), uri)
	return results, nil
}

// Index operations - SCIP index management

// StoreIndex stores the complete SCIP index
func (s *SimpleSCIPStorage) StoreIndex(ctx context.Context, index *SCIPIndex) error {
	if index == nil {
		return fmt.Errorf("index cannot be nil")
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.scipIndex = index

	// Store all documents from the index
	for _, doc := range index.Documents {
		docCopy := doc
		s.documents[doc.URI] = &docCopy
		s.updateOccurrenceIndexes(&docCopy)
	}

	// Store external symbols
	for _, symbolInfo := range index.ExternalSymbols {
		symbolCopy := symbolInfo
		s.symbolInfoIndex[symbolInfo.Symbol] = &symbolCopy
		s.symbolNameIndex[symbolInfo.DisplayName] = append(s.symbolNameIndex[symbolInfo.DisplayName], &symbolCopy)
	}

	common.LSPLogger.Info("Stored SCIP index with %d documents and %d external symbols", len(index.Documents), len(index.ExternalSymbols))
	return nil
}

// GetIndex retrieves the complete SCIP index
func (s *SimpleSCIPStorage) GetIndex(ctx context.Context) (*SCIPIndex, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.scipIndex == nil {
		return nil, fmt.Errorf("no SCIP index available")
	}

	// Create a copy with current documents
	index := &SCIPIndex{
		Metadata: s.scipIndex.Metadata,
	}

	// Add all current documents
	for _, doc := range s.documents {
		index.Documents = append(index.Documents, *doc)
	}

	// Add external symbols
	for _, info := range s.symbolInfoIndex {
		// Check if this is an external symbol (basic heuristic)
		if strings.Contains(info.Symbol, " ") && !strings.HasPrefix(info.Symbol, "local ") {
			index.ExternalSymbols = append(index.ExternalSymbols, *info)
		}
	}

	common.LSPLogger.Debug("Retrieved SCIP index with %d documents and %d external symbols", len(index.Documents), len(index.ExternalSymbols))
	return index, nil
}

// Cache management - storage optimization

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

// GetStats returns occurrence-centric storage statistics
func (s *SimpleSCIPStorage) GetStats(ctx context.Context) (*SCIPStorageStats, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	totalRequests := s.hitCount + s.missCount
	hitRate := float64(0)
	if totalRequests > 0 {
		hitRate = float64(s.hitCount) / float64(totalRequests)
	}

	// Calculate total occurrences and symbols
	totalOccurrences := int64(0)
	for _, occurrences := range s.occurrencesByURI {
		totalOccurrences += int64(len(occurrences))
	}

	stats := &SCIPStorageStats{
		MemoryUsage:      s.currentSize,
		DiskUsage:        0, // No separate disk storage in simple implementation
		MemoryLimit:      s.config.MemoryLimit,
		HitRate:          hitRate,
		CachedDocuments:  len(s.documents),
		TotalOccurrences: totalOccurrences,
		TotalSymbols:     int64(len(s.symbolInfoIndex)),
		UniqueSymbols:    len(s.occurrencesBySymbol),
		HotCacheSize:     len(s.documents), // All documents are in single cache
		CacheHits:        s.hitCount,
		CacheMisses:      s.missCount,
		EvictionCount:    0, // Track separately if needed
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

// Private helper methods for occurrence-centric operations

// evictLRU removes the least recently used document with occurrence cleanup
func (s *SimpleSCIPStorage) evictLRU() {
	if len(s.accessOrder) == 0 {
		return
	}

	// Remove oldest document (first in order)
	oldestURI := s.accessOrder[0]
	if doc, found := s.documents[oldestURI]; found {
		s.currentSize -= doc.Size
		delete(s.documents, oldestURI)
		s.removeFromOccurrenceIndexes(oldestURI)
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

// updateOccurrenceIndexes updates occurrence-centric indexes
func (s *SimpleSCIPStorage) updateOccurrenceIndexes(doc *SCIPDocument) {
	symbolIDs := make([]string, 0, len(doc.Occurrences)+len(doc.SymbolInformation))

	// Index occurrences by document URI
	s.occurrencesByURI[doc.URI] = doc.Occurrences

	// Index occurrences by symbol ID and role
	for _, occ := range doc.Occurrences {
		// Add to symbol-based occurrence index
		s.occurrencesBySymbol[occ.Symbol] = append(s.occurrencesBySymbol[occ.Symbol], occ)

		// Index definitions and references separately
		if occ.SymbolRoles.HasRole(types.SymbolRoleDefinition) {
			s.definitionIndex[occ.Symbol] = &occ
		} else {
			s.referenceIndex[occ.Symbol] = append(s.referenceIndex[occ.Symbol], occ)
		}

		symbolIDs = append(symbolIDs, occ.Symbol)
	}

	// Index symbol information
	for _, symbolInfo := range doc.SymbolInformation {
		s.symbolInfoIndex[symbolInfo.Symbol] = &symbolInfo
		s.symbolNameIndex[symbolInfo.DisplayName] = append(s.symbolNameIndex[symbolInfo.DisplayName], &symbolInfo)

		// Index relationships
		if len(symbolInfo.Relationships) > 0 {
			s.relationshipIndex[symbolInfo.Symbol] = symbolInfo.Relationships
		}

		symbolIDs = append(symbolIDs, symbolInfo.Symbol)
	}

	// Track which symbols belong to this document
	s.documentIndex[doc.URI] = symbolIDs
}

// removeFromOccurrenceIndexes removes document occurrences/symbols from indexes
func (s *SimpleSCIPStorage) removeFromOccurrenceIndexes(uri string) {
	// Remove occurrences by URI
	delete(s.occurrencesByURI, uri)

	// Get symbol IDs for this document
	symbolIDs, exists := s.documentIndex[uri]
	if !exists {
		return
	}

	// Remove occurrences from symbol-based indexes
	for _, symbolID := range symbolIDs {
		// Remove from occurrences by symbol
		if occurrences, found := s.occurrencesBySymbol[symbolID]; found {
			filteredOccs := make([]SCIPOccurrence, 0, len(occurrences))
			for _, occ := range occurrences {
				// Keep occurrences from other documents
				occDocURI := s.extractDocumentURIFromOccurrence(occ)
				if occDocURI != uri {
					filteredOccs = append(filteredOccs, occ)
				}
			}
			if len(filteredOccs) == 0 {
				delete(s.occurrencesBySymbol, symbolID)
			} else {
				s.occurrencesBySymbol[symbolID] = filteredOccs
			}
		}

		// Clean up definition index
		if def, defFound := s.definitionIndex[symbolID]; defFound {
			defDocURI := s.extractDocumentURIFromOccurrence(*def)
			if defDocURI == uri {
				delete(s.definitionIndex, symbolID)
			}
		}

		// Clean up reference index
		if refs, refsFound := s.referenceIndex[symbolID]; refsFound {
			filteredRefs := make([]SCIPOccurrence, 0, len(refs))
			for _, ref := range refs {
				refDocURI := s.extractDocumentURIFromOccurrence(ref)
				if refDocURI != uri {
					filteredRefs = append(filteredRefs, ref)
				}
			}
			if len(filteredRefs) == 0 {
				delete(s.referenceIndex, symbolID)
			} else {
				s.referenceIndex[symbolID] = filteredRefs
			}
		}

		// Clean up symbol information that belongs to this document
		if info, infoFound := s.symbolInfoIndex[symbolID]; infoFound {
			// Only remove if this was the only document defining this symbol
			// For now, keep symbol information as it might be used across documents
			_ = info // Keep for future reference
		}
	}

	// Remove document from documentIndex
	delete(s.documentIndex, uri)
}

// Helper method to extract document URI from occurrence (heuristic)
func (s *SimpleSCIPStorage) extractDocumentURIFromOccurrence(occ SCIPOccurrence) string {
	// In our current implementation, we track occurrences by document,
	// so we need to find which document this occurrence belongs to
	for uri, occurrences := range s.occurrencesByURI {
		for _, docOcc := range occurrences {
			if s.occurrencesEqual(occ, docOcc) {
				return uri
			}
		}
	}
	return "" // Not found
}

// Helper to compare occurrences for equality
func (s *SimpleSCIPStorage) occurrencesEqual(occ1, occ2 SCIPOccurrence) bool {
	return occ1.Symbol == occ2.Symbol &&
		occ1.Range.Start.Line == occ2.Range.Start.Line &&
		occ1.Range.Start.Character == occ2.Range.Start.Character &&
		occ1.Range.End.Line == occ2.Range.End.Line &&
		occ1.Range.End.Character == occ2.Range.End.Character
}

// isPositionInRange checks if occurrence range is within specified bounds
func (s *SimpleSCIPStorage) isPositionInRange(occRange types.Range, start, end types.Position) bool {
	return (occRange.Start.Line >= start.Line && occRange.Start.Character >= start.Character) &&
		(occRange.End.Line <= end.Line && occRange.End.Character <= end.Character)
}

// compileSearchPattern compiles a search pattern with case-insensitive support
func (s *SimpleSCIPStorage) compileSearchPattern(pattern string) (*regexp.Regexp, error) {
	// Check for case-insensitive flag (?i)
	finalPattern := pattern
	if strings.HasPrefix(pattern, "(?i)") {
		// Pattern already has case-insensitive flag
	} else {
		// Add case-insensitive flag by default for better search experience
		finalPattern = "(?i)" + regexp.QuoteMeta(pattern)
	}

	return regexp.Compile(finalPattern)
}

// cloneDocument creates a deep copy of the occurrence-centric document
func (s *SimpleSCIPStorage) cloneDocument(doc *SCIPDocument) *SCIPDocument {
	cloned := *doc

	// Clone occurrences
	cloned.Occurrences = make([]SCIPOccurrence, len(doc.Occurrences))
	copy(cloned.Occurrences, doc.Occurrences)

	// Clone symbol information
	cloned.SymbolInformation = make([]SCIPSymbolInformation, len(doc.SymbolInformation))
	copy(cloned.SymbolInformation, doc.SymbolInformation)

	// Clone content if present
	if doc.Content != nil {
		cloned.Content = make([]byte, len(doc.Content))
		copy(cloned.Content, doc.Content)
	}

	return &cloned
}

// Optional persistence methods

// saveToDisk saves the occurrence-centric cache to disk as JSON (optional persistence)
func (s *SimpleSCIPStorage) saveToDisk() error {
	if s.diskFile == "" {
		return nil
	}

	data := struct {
		Documents         map[string]*SCIPDocument          `json:"documents"`
		AccessOrder       []string                          `json:"access_order"`
		CurrentSize       int64                             `json:"current_size"`
		HitCount          int64                             `json:"hit_count"`
		MissCount         int64                             `json:"miss_count"`
		DocumentIndex     map[string][]string               `json:"document_index"`
		OccurrencesByURI  map[string][]SCIPOccurrence       `json:"occurrences_by_uri"`
		SymbolInfoIndex   map[string]*SCIPSymbolInformation `json:"symbol_info_index"`
		RelationshipIndex map[string][]SCIPRelationship     `json:"relationship_index"`
		SavedAt           time.Time                         `json:"saved_at"`
	}{
		Documents:         s.documents,
		AccessOrder:       s.accessOrder,
		CurrentSize:       s.currentSize,
		HitCount:          s.hitCount,
		MissCount:         s.missCount,
		DocumentIndex:     s.documentIndex,
		OccurrencesByURI:  s.occurrencesByURI,
		SymbolInfoIndex:   s.symbolInfoIndex,
		RelationshipIndex: s.relationshipIndex,
		SavedAt:           time.Now(),
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

	common.LSPLogger.Debug("Saved occurrence-centric cache with %d documents to %s", len(s.documents), s.diskFile)
	return nil
}

// loadFromDisk loads the occurrence-centric cache from disk (optional persistence)
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
		Documents         map[string]*SCIPDocument          `json:"documents"`
		AccessOrder       []string                          `json:"access_order"`
		CurrentSize       int64                             `json:"current_size"`
		HitCount          int64                             `json:"hit_count"`
		MissCount         int64                             `json:"miss_count"`
		DocumentIndex     map[string][]string               `json:"document_index"`
		OccurrencesByURI  map[string][]SCIPOccurrence       `json:"occurrences_by_uri"`
		SymbolInfoIndex   map[string]*SCIPSymbolInformation `json:"symbol_info_index"`
		RelationshipIndex map[string][]SCIPRelationship     `json:"relationship_index"`
		SavedAt           time.Time                         `json:"saved_at"`
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

	// Restore document index or initialize if not present
	if data.DocumentIndex != nil {
		s.documentIndex = data.DocumentIndex
	} else {
		s.documentIndex = make(map[string][]string)
	}

	// Restore occurrence-centric indexes or rebuild
	if data.OccurrencesByURI != nil {
		s.occurrencesByURI = data.OccurrencesByURI
	} else {
		s.occurrencesByURI = make(map[string][]SCIPOccurrence)
	}

	if data.SymbolInfoIndex != nil {
		s.symbolInfoIndex = data.SymbolInfoIndex
	} else {
		s.symbolInfoIndex = make(map[string]*SCIPSymbolInformation)
	}

	if data.RelationshipIndex != nil {
		s.relationshipIndex = data.RelationshipIndex
	} else {
		s.relationshipIndex = make(map[string][]SCIPRelationship)
	}

	// Rebuild occurrence-based indexes from documents if not present in cache
	s.occurrencesBySymbol = make(map[string][]SCIPOccurrence)
	s.definitionIndex = make(map[string]*SCIPOccurrence)
	s.referenceIndex = make(map[string][]SCIPOccurrence)
	s.symbolNameIndex = make(map[string][]*SCIPSymbolInformation)

	for _, doc := range s.documents {
		s.updateOccurrenceIndexes(doc)
	}

	common.LSPLogger.Info("Loaded occurrence-centric cache with %d documents from %s (saved at %v)",
		len(s.documents), s.diskFile, data.SavedAt)
	return nil
}

// Interface compliance verification
var _ SCIPDocumentStorage = (*SimpleSCIPStorage)(nil)
