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
	// Pattern cache limits for performance
	MaxPatternCacheSize    = 1000
	MaxSymbolSearchResults = 500
)

// OccurrenceWithDocument wraps an occurrence with its document URI
type OccurrenceWithDocument struct {
	SCIPOccurrence
	DocumentURI string
}

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
	definitionIndex     map[string][]*SCIPOccurrence        // symbol ID -> definition occurrences
	referenceIndex      map[string][]SCIPOccurrence         // symbol ID -> reference occurrences
	referenceWithDocs   map[string][]OccurrenceWithDocument // symbol ID -> references with doc URIs
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

	// Performance optimizations for MCP tools
	patternCache    map[string]*regexp.Regexp // Compiled regex cache
	patternCacheLRU []string                  // LRU order for pattern cache
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
		definitionIndex:     make(map[string][]*SCIPOccurrence),
		referenceIndex:      make(map[string][]SCIPOccurrence),
		referenceWithDocs:   make(map[string][]OccurrenceWithDocument),
		symbolInfoIndex:     make(map[string]*SCIPSymbolInformation),
		symbolNameIndex:     make(map[string][]*SCIPSymbolInformation),
		relationshipIndex:   make(map[string][]SCIPRelationship),
		documentIndex:       make(map[string][]string),
		diskFile:            filepath.Join(config.DiskCacheDir, "simple_cache.json"),
		// Pattern cache for SearchSymbols
		patternCache:    make(map[string]*regexp.Regexp),
		patternCacheLRU: make([]string, 0),
	}

	return storage, nil
}

// Start initializes the storage
func (s *SimpleSCIPStorage) Start(ctx context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.started {
		common.LSPLogger.Info("[SimpleSCIPStorage.Start] Storage already started, returning")
		return fmt.Errorf("storage already started")
	}

	common.LSPLogger.Info("[SimpleSCIPStorage.Start] Starting SCIP storage, disk file: %s", s.diskFile)

	// Create directory
	if err := os.MkdirAll(s.config.DiskCacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Load from disk if available
	s.loadFromDisk()

	s.started = true
	common.LSPLogger.Info("[SimpleSCIPStorage.Start] Storage started successfully with %d symbols in symbolNameIndex", len(s.symbolNameIndex))
	return nil
}

// Stop gracefully shuts down the storage
func (s *SimpleSCIPStorage) Stop(ctx context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.started {
		return nil
	}

	// Save to disk
	if err := s.saveToDisk(); err != nil {
		common.LSPLogger.Error("Error saving cache during shutdown: %v", err)
	}

	s.started = false
	return nil
}

// StoreDocument stores a document with occurrence-centric indexing and merging support
func (s *SimpleSCIPStorage) StoreDocument(ctx context.Context, doc *SCIPDocument) error {
	if doc == nil {
		return fmt.Errorf("document cannot be nil")
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.storeDocumentUnsafe(doc)
}

// storeDocumentUnsafe stores a document without mutex locking (internal use)
func (s *SimpleSCIPStorage) storeDocumentUnsafe(doc *SCIPDocument) error {
	// Check if document exists and merge occurrences
	if existing, found := s.documents[doc.URI]; found {
		// Merge occurrences instead of replacing
		merged := s.mergeDocuments(existing, doc)
		s.currentSize -= existing.Size
		s.removeFromAccessOrder(doc.URI)
		s.removeFromOccurrenceIndexes(doc.URI)
		doc = merged
	}

	// Evict if necessary to make space
	for s.currentSize+doc.Size > s.config.MemoryLimit && len(s.documents) > 0 {
		s.evictLRU()
	}

	// Store document
	s.documents[doc.URI] = s.cloneDocument(doc)
	s.currentSize += doc.Size
	s.addToAccessOrder(doc.URI)
	s.updateOccurrenceIndexes(doc)

	// Debug: Check symbol info index size after update
	common.LSPLogger.Debug("StoreDocument: Stored doc %s with %d occurrences and %d symbol infos. Total symbols in index: %d",
		doc.URI, len(doc.Occurrences), len(doc.SymbolInformation), len(s.symbolInfoIndex))

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
	return s.cloneDocument(doc), nil
}

// RemoveDocument removes a document from the cache
func (s *SimpleSCIPStorage) RemoveDocument(ctx context.Context, uri string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if doc, found := s.documents[uri]; found {
		s.currentSize -= doc.Size
		delete(s.documents, uri)
		s.removeFromAccessOrder(uri)
		s.removeFromOccurrenceIndexes(uri)
	}
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

// GetOccurrences retrieves all occurrences of a specific symbol across documents
func (s *SimpleSCIPStorage) GetOccurrences(ctx context.Context, symbolID string) ([]SCIPOccurrence, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Direct symbol lookup
	if occurrences, found := s.occurrencesBySymbol[symbolID]; found {
		result := make([]SCIPOccurrence, len(occurrences))
		copy(result, occurrences)
		return result, nil
	}

	return []SCIPOccurrence{}, nil
}

// GetDefinitions retrieves all definition occurrences of a symbol
func (s *SimpleSCIPStorage) GetDefinitions(ctx context.Context, symbolID string) ([]SCIPOccurrence, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	definitions, found := s.definitionIndex[symbolID]
	if !found {
		return []SCIPOccurrence{}, nil
	}

	result := make([]SCIPOccurrence, len(definitions))
	for i, def := range definitions {
		result[i] = *def
	}
	return result, nil
}

// GetReferences retrieves all reference occurrences of a symbol
func (s *SimpleSCIPStorage) GetReferences(ctx context.Context, symbolID string) ([]SCIPOccurrence, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	references, found := s.referenceIndex[symbolID]
	if !found {
		return []SCIPOccurrence{}, nil
	}

	result := make([]SCIPOccurrence, len(references))
	copy(result, references)
	return result, nil
}

// GetReferencesWithDocuments retrieves all reference occurrences with their document URIs
func (s *SimpleSCIPStorage) GetReferencesWithDocuments(ctx context.Context, symbolID string) ([]OccurrenceWithDocument, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	references, found := s.referenceWithDocs[symbolID]
	if !found {
		return []OccurrenceWithDocument{}, nil
	}

	result := make([]OccurrenceWithDocument, len(references))
	copy(result, references)
	return result, nil
}

// Symbol information operations - metadata about symbols

// GetSymbolInfo retrieves symbol information by symbol ID
func (s *SimpleSCIPStorage) GetSymbolInfo(ctx context.Context, symbolID string) (*SCIPSymbolInformation, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	info, found := s.symbolInfoIndex[symbolID]
	if !found {
		return nil, fmt.Errorf("symbol information not found: %s", symbolID)
	}
	result := *info
	return &result, nil
}

// Relationship operations - symbol relationships

// Search operations - finding symbols across documents

// SearchSymbols searches symbols by name pattern with simplified implementation
func (s *SimpleSCIPStorage) SearchSymbols(ctx context.Context, query string, limit int) ([]SCIPSymbolInformation, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if limit <= 0 {
		limit = MaxSymbolSearchResults
	}

	common.LSPLogger.Info("[SCIP] SearchSymbols called with query: %q, limit: %d", query, limit)
	common.LSPLogger.Info("[SCIP] Total symbols in index: %d, in name index: %d", len(s.symbolInfoIndex), len(s.symbolNameIndex))

	// Debug: Log first 5 names in symbolNameIndex to verify it's populated
	if len(s.symbolNameIndex) > 0 {
		var sampleNames []string
		count := 0
		for name := range s.symbolNameIndex {
			sampleNames = append(sampleNames, name)
			count++
			if count >= 5 {
				break
			}
		}
		common.LSPLogger.Info("[SCIP] Sample names in symbolNameIndex: %v", sampleNames)
	}

	// Check if query is a regex pattern
	var pattern *regexp.Regexp
	var err error
	if s.containsRegexPattern(query) {
		pattern, err = s.getCachedPattern(query)
		if err != nil {
			return nil, fmt.Errorf("invalid search pattern: %w", err)
		}
	}

	// Search through all symbol information
	var results []SCIPSymbolInformation
	seenSymbols := make(map[string]bool) // Track symbols we've already added

	// First, check symbolNameIndex for direct name lookups
	if pattern == nil {
		// Log what we're looking for
		common.LSPLogger.Info("[SCIP] Looking for exact match: %q in symbolNameIndex with %d entries", query, len(s.symbolNameIndex))

		// Check if the query exists in the index
		if _, exists := s.symbolNameIndex[query]; exists {
			common.LSPLogger.Info("[SCIP] Query %q EXISTS in symbolNameIndex!", query)
		} else {
			common.LSPLogger.Info("[SCIP] Query %q NOT FOUND in symbolNameIndex", query)
		}

		// Try exact match first from symbolNameIndex
		if symbols, found := s.symbolNameIndex[query]; found {
			common.LSPLogger.Info("[SCIP] Found exact match in symbolNameIndex: %d symbols", len(symbols))
			for _, sym := range symbols {
				if sym != nil && !seenSymbols[sym.Symbol] {
					results = append(results, *sym)
					seenSymbols[sym.Symbol] = true
					if len(results) >= limit {
						return results, nil
					}
				}
			}
		}

		// Also try case-insensitive match
		for name, symbols := range s.symbolNameIndex {
			if strings.EqualFold(name, query) && name != query {
				common.LSPLogger.Debug("[SCIP] Found case-insensitive match: %s", name)
				for _, sym := range symbols {
					if sym != nil && !seenSymbols[sym.Symbol] {
						results = append(results, *sym)
						seenSymbols[sym.Symbol] = true
						if len(results) >= limit {
							return results, nil
						}
					}
				}
			}
		}
	} else {
		// Pattern-based search through symbolNameIndex
		for name, symbols := range s.symbolNameIndex {
			if pattern.MatchString(name) {
				for _, sym := range symbols {
					if sym != nil && !seenSymbols[sym.Symbol] {
						results = append(results, *sym)
						seenSymbols[sym.Symbol] = true
						if len(results) >= limit {
							return results, nil
						}
					}
				}
			}
		}
	}

	// If still need more results, search through symbolInfoIndex for prefix matches
	if len(results) < limit && pattern == nil {
		lowerQuery := strings.ToLower(query)
		for _, info := range s.symbolInfoIndex {
			if info != nil && !seenSymbols[info.Symbol] {
				if strings.HasPrefix(strings.ToLower(info.DisplayName), lowerQuery) {
					results = append(results, *info)
					seenSymbols[info.Symbol] = true
					if len(results) >= limit {
						break
					}
				}
			}
		}
	}

	common.LSPLogger.Debug("[SCIP] SearchSymbols returning %d results", len(results))
	return results, nil
}

// Workspace operations - project-level queries

// MCP Tool Direct Lookup Methods - Sub-millisecond performance

// Index operations - SCIP index management

// Cache management - storage optimization

// GetIndexStats returns simple storage statistics
func (s *SimpleSCIPStorage) GetIndexStats() IndexStats {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	totalRequests := s.hitCount + s.missCount
	hitRate := float64(0)
	if totalRequests > 0 {
		hitRate = float64(s.hitCount) / float64(totalRequests)
	}

	// Calculate total occurrences from multiple sources
	totalOccurrences := int64(0)

	// Count from occurrencesByURI if available
	if len(s.occurrencesByURI) > 0 {
		for _, occurrences := range s.occurrencesByURI {
			totalOccurrences += int64(len(occurrences))
		}
	}

	// If occurrencesByURI is empty but we have documents, count from documents
	if totalOccurrences == 0 && len(s.documents) > 0 {
		for _, doc := range s.documents {
			if doc != nil {
				totalOccurrences += int64(len(doc.Occurrences))
			}
		}
	}

	// Count references from referenceWithDocs as a fallback
	totalReferences := int64(0)
	for _, refs := range s.referenceWithDocs {
		totalReferences += int64(len(refs))
	}

	// Use the maximum of occurrences or references
	if totalReferences > totalOccurrences {
		totalOccurrences = totalReferences
	}

	common.LSPLogger.Debug("[GetIndexStats] docs=%d, symbols=%d, occurrences=%d, refs=%d, symbolNames=%d",
		len(s.documents), len(s.symbolInfoIndex), totalOccurrences, totalReferences, len(s.symbolNameIndex))

	return IndexStats{
		TotalDocuments:   len(s.documents),
		TotalOccurrences: totalOccurrences,
		TotalSymbols:     int64(len(s.symbolInfoIndex)),
		MemoryUsage:      s.currentSize,
		HitRate:          hitRate,
	}
}

// ClearIndex clears all cached data
func (s *SimpleSCIPStorage) ClearIndex(ctx context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Clear all maps and slices
	s.documents = make(map[string]*SCIPDocument)
	s.accessOrder = make([]string, 0)
	s.occurrencesByURI = make(map[string][]SCIPOccurrence)
	s.occurrencesBySymbol = make(map[string][]SCIPOccurrence)
	s.definitionIndex = make(map[string][]*SCIPOccurrence)
	s.referenceIndex = make(map[string][]SCIPOccurrence)
	s.referenceWithDocs = make(map[string][]OccurrenceWithDocument)
	s.symbolInfoIndex = make(map[string]*SCIPSymbolInformation)
	s.symbolNameIndex = make(map[string][]*SCIPSymbolInformation)
	s.documentIndex = make(map[string][]string)
	s.patternCache = make(map[string]*regexp.Regexp)
	s.patternCacheLRU = make([]string, 0)

	// Reset counters
	s.currentSize = 0
	s.hitCount = 0
	s.missCount = 0

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

// updateOccurrenceIndexes updates simplified occurrence-centric indexes
func (s *SimpleSCIPStorage) updateOccurrenceIndexes(doc *SCIPDocument) {
	symbolIDs := make([]string, 0, len(doc.Occurrences)+len(doc.SymbolInformation))

	// Clear existing indexes for this URI to avoid duplicates
	s.occurrencesByURI[doc.URI] = make([]SCIPOccurrence, len(doc.Occurrences))
	copy(s.occurrencesByURI[doc.URI], doc.Occurrences)

	// Index occurrences by symbol ID and role
	for i := range doc.Occurrences {
		occ := &doc.Occurrences[i]

		// Add to symbol-based occurrence index
		s.occurrencesBySymbol[occ.Symbol] = append(s.occurrencesBySymbol[occ.Symbol], *occ)

		// Index definitions and references separately
		if occ.SymbolRoles.HasRole(types.SymbolRoleDefinition) {
			s.definitionIndex[occ.Symbol] = append(s.definitionIndex[occ.Symbol], occ)
		} else {
			s.referenceIndex[occ.Symbol] = append(s.referenceIndex[occ.Symbol], *occ)
			// Also store with document URI for easy lookup
			s.referenceWithDocs[occ.Symbol] = append(s.referenceWithDocs[occ.Symbol], OccurrenceWithDocument{
				SCIPOccurrence: *occ,
				DocumentURI:    doc.URI,
			})
		}

		symbolIDs = append(symbolIDs, occ.Symbol)
	}

	// Index symbol information with additional indexes
	for i := range doc.SymbolInformation {
		symbolInfo := &doc.SymbolInformation[i]
		s.symbolInfoIndex[symbolInfo.Symbol] = symbolInfo

		// Update symbol name index
		s.symbolNameIndex[symbolInfo.DisplayName] = append(s.symbolNameIndex[symbolInfo.DisplayName], symbolInfo)

		// Update relationship index if relationships exist
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
		if definitions, defFound := s.definitionIndex[symbolID]; defFound {
			filteredDefs := make([]*SCIPOccurrence, 0, len(definitions))
			for _, def := range definitions {
				defDocURI := s.extractDocumentURIFromOccurrence(*def)
				if defDocURI != uri {
					filteredDefs = append(filteredDefs, def)
				}
			}
			if len(filteredDefs) == 0 {
				delete(s.definitionIndex, symbolID)
			} else {
				s.definitionIndex[symbolID] = filteredDefs
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

		// Note: Symbol information is kept as it might be used across documents
	}

	// Remove document from documentIndex
	delete(s.documentIndex, uri)
}

// GetAffectedDocuments returns all documents that reference symbols defined in the given document
// This is used for cascading cache invalidation when a file changes
func (s *SimpleSCIPStorage) GetAffectedDocuments(uri string) []string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	affectedDocs := make(map[string]bool)

	// 1. Get all symbols defined in this document
	symbolIDs, exists := s.documentIndex[uri]
	if !exists {
		return []string{}
	}

	// 2. For each symbol defined in this document, find all documents that reference it
	for _, symbolID := range symbolIDs {
		// Check if this symbol has its definition in the current document
		if definitions, found := s.definitionIndex[symbolID]; found {
			for _, def := range definitions {
				defURI := s.extractDocumentURIFromOccurrence(*def)
				if defURI == uri {
					// This symbol is defined in our document
					// Find all references to this symbol in other documents
					if refs, found := s.referenceIndex[symbolID]; found {
						for _, ref := range refs {
							refURI := s.extractDocumentURIFromOccurrence(ref)
							if refURI != "" && refURI != uri {
								affectedDocs[refURI] = true
							}
						}
					}

					// Also check occurrences by symbol for completeness
					if occs, found := s.occurrencesBySymbol[symbolID]; found {
						for _, occ := range occs {
							occURI := s.extractDocumentURIFromOccurrence(occ)
							if occURI != "" && occURI != uri {
								affectedDocs[occURI] = true
							}
						}
					}
				}
			}
		}
	}

	// 3. Convert map to slice
	result := make([]string, 0, len(affectedDocs))
	for doc := range affectedDocs {
		result = append(result, doc)
	}

	common.LSPLogger.Debug("Document %s affects %d other documents", uri, len(result))
	return result
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

// MCP Tool Optimization Methods - Performance-critical paths

// getCachedPattern gets or compiles and caches a regex pattern for performance
func (s *SimpleSCIPStorage) getCachedPattern(pattern string) (*regexp.Regexp, error) {
	// Check cache first
	if cached, found := s.patternCache[pattern]; found {
		// Move to end of LRU
		s.movePatternToEnd(pattern)
		return cached, nil
	}

	// Compile new pattern
	compiled, err := s.compileSearchPattern(pattern)
	if err != nil {
		return nil, err
	}

	// Add to cache with LRU eviction
	s.addPatternToCache(pattern, compiled)
	return compiled, nil
}

// containsRegexPattern checks if a string contains regex special characters
func (s *SimpleSCIPStorage) containsRegexPattern(pattern string) bool {
	// Check for common regex metacharacters
	return strings.ContainsAny(pattern, ".*+?^${}()|[]\\")
}

// Pattern cache management for performance

// addPatternToCache adds a compiled pattern to cache with LRU eviction
func (s *SimpleSCIPStorage) addPatternToCache(pattern string, compiled *regexp.Regexp) {
	// Evict if cache is full
	if len(s.patternCache) >= MaxPatternCacheSize {
		s.evictOldestPattern()
	}

	s.patternCache[pattern] = compiled
	s.patternCacheLRU = append(s.patternCacheLRU, pattern)
}

// movePatternToEnd moves a pattern to the end of LRU list
func (s *SimpleSCIPStorage) movePatternToEnd(pattern string) {
	for i, p := range s.patternCacheLRU {
		if p == pattern {
			// Remove from current position
			s.patternCacheLRU = append(s.patternCacheLRU[:i], s.patternCacheLRU[i+1:]...)
			// Add to end
			s.patternCacheLRU = append(s.patternCacheLRU, pattern)
			break
		}
	}
}

// evictOldestPattern removes the oldest pattern from cache
func (s *SimpleSCIPStorage) evictOldestPattern() {
	if len(s.patternCacheLRU) > 0 {
		oldest := s.patternCacheLRU[0]
		delete(s.patternCache, oldest)
		s.patternCacheLRU = s.patternCacheLRU[1:]
	}
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

	common.LSPLogger.Debug("[saveToDisk] Saving cache with %d documents, %d symbols", len(s.documents), len(s.symbolInfoIndex))

	// Convert symbolNameIndex pointers to values for JSON serialization
	symbolNameData := make(map[string][]SCIPSymbolInformation)
	for name, symbols := range s.symbolNameIndex {
		for _, sym := range symbols {
			if sym != nil {
				symbolNameData[name] = append(symbolNameData[name], *sym)
			}
		}
	}

	data := struct {
		Documents         map[string]*SCIPDocument           `json:"documents"`
		AccessOrder       []string                           `json:"access_order"`
		CurrentSize       int64                              `json:"current_size"`
		HitCount          int64                              `json:"hit_count"`
		MissCount         int64                              `json:"miss_count"`
		DocumentIndex     map[string][]string                `json:"document_index"`
		OccurrencesByURI  map[string][]SCIPOccurrence        `json:"occurrences_by_uri"`
		SymbolInfoIndex   map[string]*SCIPSymbolInformation  `json:"symbol_info_index"`
		SymbolNameIndex   map[string][]SCIPSymbolInformation `json:"symbol_name_index"`
		RelationshipIndex map[string][]SCIPRelationship      `json:"relationship_index"`
		SavedAt           time.Time                          `json:"saved_at"`
	}{
		Documents:         s.documents,
		AccessOrder:       s.accessOrder,
		CurrentSize:       s.currentSize,
		HitCount:          s.hitCount,
		MissCount:         s.missCount,
		DocumentIndex:     s.documentIndex,
		OccurrencesByURI:  s.occurrencesByURI,
		SymbolInfoIndex:   s.symbolInfoIndex,
		SymbolNameIndex:   symbolNameData,
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
	return nil
}

// loadFromDisk loads the occurrence-centric cache from disk (optional persistence)
func (s *SimpleSCIPStorage) loadFromDisk() error {
	if s.diskFile == "" {
		return nil
	}

	common.LSPLogger.Debug("Attempting to load SCIP cache from disk: %s", s.diskFile)
	file, err := os.Open(s.diskFile)
	if err != nil {
		if os.IsNotExist(err) {
			common.LSPLogger.Debug("SCIP cache file does not exist (starting fresh): %s", s.diskFile)
		} else {
			common.LSPLogger.Warn("Failed to open SCIP cache file: %v", err)
		}
		return err
	}
	defer file.Close()

	var data struct {
		Documents         map[string]*SCIPDocument           `json:"documents"`
		AccessOrder       []string                           `json:"access_order"`
		CurrentSize       int64                              `json:"current_size"`
		HitCount          int64                              `json:"hit_count"`
		MissCount         int64                              `json:"miss_count"`
		DocumentIndex     map[string][]string                `json:"document_index"`
		OccurrencesByURI  map[string][]SCIPOccurrence        `json:"occurrences_by_uri"`
		SymbolInfoIndex   map[string]*SCIPSymbolInformation  `json:"symbol_info_index"`
		SymbolNameIndex   map[string][]SCIPSymbolInformation `json:"symbol_name_index"`
		RelationshipIndex map[string][]SCIPRelationship      `json:"relationship_index"`
		SavedAt           time.Time                          `json:"saved_at"`
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

	common.LSPLogger.Debug("Loaded SCIP cache from disk: %d documents, %d symbols",
		len(s.documents), len(data.SymbolInfoIndex))

	// Restore document index or initialize if not present
	if data.DocumentIndex != nil {
		s.documentIndex = data.DocumentIndex
	} else {
		s.documentIndex = make(map[string][]string)
	}

	// Restore occurrences by URI
	if data.OccurrencesByURI != nil {
		s.occurrencesByURI = data.OccurrencesByURI
	} else {
		s.occurrencesByURI = make(map[string][]SCIPOccurrence)
	}

	// Restore symbol info index
	if data.SymbolInfoIndex != nil {
		s.symbolInfoIndex = data.SymbolInfoIndex
		common.LSPLogger.Debug("[loadFromDisk] Loaded %d symbols from disk", len(s.symbolInfoIndex))
		// Log a sample of symbols for debugging
		count := 0
		for id, info := range s.symbolInfoIndex {
			if count < 5 && info != nil {
				common.LSPLogger.Debug("[loadFromDisk] Sample symbol: ID=%s, DisplayName=%s", id, info.DisplayName)
				count++
			}
		}
	} else {
		s.symbolInfoIndex = make(map[string]*SCIPSymbolInformation)
		common.LSPLogger.Debug("[loadFromDisk] No symbol info index in saved data")
	}

	// Restore relationship index
	if data.RelationshipIndex != nil {
		s.relationshipIndex = data.RelationshipIndex
	} else {
		s.relationshipIndex = make(map[string][]SCIPRelationship)
	}

	// Restore symbolNameIndex if present
	if data.SymbolNameIndex != nil && len(data.SymbolNameIndex) > 0 {
		common.LSPLogger.Info("[loadFromDisk] Loading symbolNameIndex from disk with %d entries", len(data.SymbolNameIndex))
		s.symbolNameIndex = make(map[string][]*SCIPSymbolInformation)
		for name, symbols := range data.SymbolNameIndex {
			for i := range symbols {
				// Create pointer to the symbol from the loaded data
				sym := symbols[i]
				s.symbolNameIndex[name] = append(s.symbolNameIndex[name], &sym)
			}
		}
		common.LSPLogger.Info("[loadFromDisk] Loaded symbolNameIndex with %d unique names", len(s.symbolNameIndex))
	} else {
		common.LSPLogger.Info("[loadFromDisk] No symbolNameIndex in saved data, will rebuild from symbolInfoIndex")
		s.symbolNameIndex = make(map[string][]*SCIPSymbolInformation)
	}

	// Rebuild the other indexes from documents
	common.LSPLogger.Info("[loadFromDisk] About to rebuild other indexes from documents, symbolInfoIndex has %d entries", len(s.symbolInfoIndex))
	s.rebuildIndexesFromDocuments()

	// Log statistics after loading
	common.LSPLogger.Info("[loadFromDisk] SCIP storage loaded: %d documents, %d symbols, %d symbol names in index, %d occurrences",
		len(s.documents), len(s.symbolInfoIndex), len(s.symbolNameIndex), len(s.occurrencesByURI))

	// Check if NewLSPManager is in the symbolNameIndex
	if symbols, found := s.symbolNameIndex["NewLSPManager"]; found {
		common.LSPLogger.Info("[loadFromDisk] NewLSPManager found in symbolNameIndex with %d entries", len(symbols))
	} else {
		common.LSPLogger.Warn("[loadFromDisk] NewLSPManager NOT found in symbolNameIndex!")
	}

	return nil
}

// rebuildIndexesFromDocuments rebuilds the in-memory indexes from loaded documents
func (s *SimpleSCIPStorage) rebuildIndexesFromDocuments() {
	// Initialize indexes that aren't persisted
	s.occurrencesBySymbol = make(map[string][]SCIPOccurrence)
	s.definitionIndex = make(map[string][]*SCIPOccurrence)
	s.referenceIndex = make(map[string][]SCIPOccurrence)
	s.referenceWithDocs = make(map[string][]OccurrenceWithDocument)

	// Only rebuild symbolNameIndex if it's empty (wasn't loaded from disk)
	if len(s.symbolNameIndex) == 0 {
		s.symbolNameIndex = make(map[string][]*SCIPSymbolInformation)

		// Rebuild from symbol info index
		common.LSPLogger.Info("[rebuildIndexesFromDocuments] Rebuilding symbolNameIndex from %d symbols in symbolInfoIndex", len(s.symbolInfoIndex))
		addedCount := 0
		for symbolID, symbolInfo := range s.symbolInfoIndex {
			if symbolInfo != nil && symbolInfo.DisplayName != "" {
				s.symbolNameIndex[symbolInfo.DisplayName] = append(s.symbolNameIndex[symbolInfo.DisplayName], symbolInfo)
				addedCount++
				if symbolInfo.DisplayName == "NewLSPManager" {
					common.LSPLogger.Info("[rebuildIndexesFromDocuments] Found NewLSPManager! Symbol ID: %s", symbolID)
				}
			} else if symbolInfo != nil {
				common.LSPLogger.Info("[rebuildIndexesFromDocuments] Symbol %s has empty DisplayName", symbolID)
			}
		}
		common.LSPLogger.Info("[rebuildIndexesFromDocuments] Rebuilt symbolNameIndex with %d unique names from %d symbols", len(s.symbolNameIndex), addedCount)
	} else {
		common.LSPLogger.Info("[rebuildIndexesFromDocuments] symbolNameIndex already loaded from disk with %d entries, skipping rebuild", len(s.symbolNameIndex))
	}

	// Rebuild occurrence indexes from documents
	for uri, doc := range s.documents {
		if doc != nil {
			// Index occurrences by symbol
			for i := range doc.Occurrences {
				occ := &doc.Occurrences[i]
				s.occurrencesBySymbol[occ.Symbol] = append(s.occurrencesBySymbol[occ.Symbol], *occ)

				// Index definitions and references separately
				if occ.SymbolRoles.HasRole(types.SymbolRoleDefinition) {
					s.definitionIndex[occ.Symbol] = append(s.definitionIndex[occ.Symbol], occ)
				} else {
					s.referenceIndex[occ.Symbol] = append(s.referenceIndex[occ.Symbol], *occ)
					// Also store with document URI for easy lookup
					s.referenceWithDocs[occ.Symbol] = append(s.referenceWithDocs[occ.Symbol], OccurrenceWithDocument{
						SCIPOccurrence: *occ,
						DocumentURI:    uri,
					})
				}
			}
		}
	}

	common.LSPLogger.Debug("Rebuilt indexes: %d symbol names, %d occurrences by symbol, %d references with docs",
		len(s.symbolNameIndex), len(s.occurrencesBySymbol), len(s.referenceWithDocs))
}

// occurrenceKey provides a struct-based key for occurrence deduplication
type occurrenceKey struct {
	symbol string
	line   int32
	char   int32
}

// mergeDocuments efficiently merges occurrences and symbols from two documents
func (s *SimpleSCIPStorage) mergeDocuments(existing, new *SCIPDocument) *SCIPDocument {
	// Create merged document
	merged := &SCIPDocument{
		URI:          existing.URI,
		Language:     existing.Language,
		LastModified: new.LastModified,
	}

	// Use struct key for better performance than string concatenation
	occMap := make(map[occurrenceKey]SCIPOccurrence)

	// Add existing occurrences
	for _, occ := range existing.Occurrences {
		key := occurrenceKey{
			symbol: occ.Symbol,
			line:   occ.Range.Start.Line,
			char:   occ.Range.Start.Character,
		}
		occMap[key] = occ
	}

	// Add/update with new occurrences
	for _, occ := range new.Occurrences {
		key := occurrenceKey{
			symbol: occ.Symbol,
			line:   occ.Range.Start.Line,
			char:   occ.Range.Start.Character,
		}
		occMap[key] = occ // Overwrites if exists
	}

	// Convert back to slice with pre-allocated capacity
	merged.Occurrences = make([]SCIPOccurrence, 0, len(occMap))
	for _, occ := range occMap {
		merged.Occurrences = append(merged.Occurrences, occ)
	}

	// Merge symbol information using symbol ID as key
	symbolMap := make(map[string]SCIPSymbolInformation)
	for _, si := range existing.SymbolInformation {
		symbolMap[si.Symbol] = si
	}
	for _, si := range new.SymbolInformation {
		symbolMap[si.Symbol] = si
	}

	// Convert back to slice with pre-allocated capacity
	merged.SymbolInformation = make([]SCIPSymbolInformation, 0, len(symbolMap))
	for _, si := range symbolMap {
		merged.SymbolInformation = append(merged.SymbolInformation, si)
	}

	// Calculate size
	merged.Size = s.calculateDocumentSize(merged)

	return merged
}

// AddOccurrences efficiently adds occurrences to a document in batch
func (s *SimpleSCIPStorage) AddOccurrences(ctx context.Context, uri string, occurrences []SCIPOccurrence) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	doc, found := s.documents[uri]
	if !found {
		// Create new document with just these occurrences
		doc = &SCIPDocument{
			URI:         uri,
			Occurrences: occurrences,
		}
		doc.Size = s.calculateDocumentSize(doc)
		return s.storeDocumentUnsafe(doc)
	}

	// Build occurrence map for efficient duplicate detection using struct keys
	occMap := make(map[occurrenceKey]bool)
	for _, occ := range doc.Occurrences {
		key := occurrenceKey{
			symbol: occ.Symbol,
			line:   occ.Range.Start.Line,
			char:   occ.Range.Start.Character,
		}
		occMap[key] = true
	}

	// Add only new occurrences
	newOccurrences := make([]SCIPOccurrence, 0, len(occurrences))
	for _, occ := range occurrences {
		key := occurrenceKey{
			symbol: occ.Symbol,
			line:   occ.Range.Start.Line,
			char:   occ.Range.Start.Character,
		}
		if !occMap[key] {
			newOccurrences = append(newOccurrences, occ)
		}
	}

	if len(newOccurrences) == 0 {
		return nil // No new occurrences to add
	}

	// Update document size calculation
	s.currentSize -= doc.Size
	doc.Occurrences = append(doc.Occurrences, newOccurrences...)
	doc.Size = s.calculateDocumentSize(doc)
	s.currentSize += doc.Size

	// Update indexes
	s.updateOccurrenceIndexes(doc)

	return nil
}

// calculateDocumentSize calculates the memory footprint of a document
func (s *SimpleSCIPStorage) calculateDocumentSize(doc *SCIPDocument) int64 {
	if doc == nil {
		return 0
	}

	// Base size for document structure
	size := int64(len(doc.URI)) + 64 // URI + overhead

	// Add occurrences size
	size += int64(len(doc.Occurrences)) * 128 // Estimate per occurrence

	// Add symbol information size
	for _, si := range doc.SymbolInformation {
		size += int64(len(si.Symbol)) + int64(len(si.DisplayName)) + 64
	}

	// Add content size if present
	if doc.Content != nil {
		size += int64(len(doc.Content))
	}

	return size
}

// GetAllDocuments returns all documents in storage
func (s *SimpleSCIPStorage) GetAllDocuments() map[string]*SCIPDocument {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Create a copy to avoid external modifications
	docs := make(map[string]*SCIPDocument)
	for uri, doc := range s.documents {
		docs[uri] = doc
	}
	return docs
}

// Interface compliance verification
var _ SCIPDocumentStorage = (*SimpleSCIPStorage)(nil)
