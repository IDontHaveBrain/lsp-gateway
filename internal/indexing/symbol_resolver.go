package indexing

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sourcegraph/scip/bindings/go/scip"
)

// SymbolResolver provides advanced symbol resolution with position-based lookups,
// cross-language navigation, and intelligent caching for sub-10ms P99 performance
type SymbolResolver struct {
	scipClient      *SCIPClient
	positionIndex   *PositionIndex
	symbolGraph     *SymbolGraph
	rangeCalculator *RangeCalculator
	config          *ResolverConfig
	cache           *ResolutionCache
	stats           *ResolverStats
	mutex           sync.RWMutex

	// Performance optimization
	requestCount        int64
	totalResolutionTime int64
	lastOptimization    time.Time
}

// PositionIndex provides O(log n) spatial indexing for fast position-based lookups
type PositionIndex struct {
	documents   map[string]*DocumentIndex // file -> position index
	spatialTree *IntervalTree             // Fast range overlap queries
	symbols     map[string]*IndexedSymbol // symbol -> indexed data
	mutex       sync.RWMutex

	// Performance metrics
	queryCount   int64
	avgQueryTime time.Duration
	cacheHitRate float64
	lastReindex  time.Time
}

// DocumentIndex represents spatial index for a single document
type DocumentIndex struct {
	uri          string
	intervalTree *IntervalTree             // Ranges -> symbols
	symbolMap    map[string]*IndexedSymbol // Quick symbol lookup
	lineIndex    []int32                   // Line start byte offsets
	lastModified time.Time
	symbolCount  int
	queryCount   int64
	avgQueryTime time.Duration
}

// IntervalTree implements an efficient interval tree for range overlap queries
type IntervalTree struct {
	root     *IntervalNode
	nodePool sync.Pool // Reuse nodes for performance
	size     int
}

// IntervalNode represents a node in the interval tree
type IntervalNode struct {
	interval    *Interval
	maxEnd      int32
	left, right *IntervalNode
	symbols     []*IndexedSymbol
	height      int
}

// Interval represents a range with efficient comparison operations
type Interval struct {
	start, end int32
	data       interface{}
}

// IndexedSymbol represents a symbol with optimized access patterns
type IndexedSymbol struct {
	symbol          string
	displayName     string
	kind            scip.SymbolInformation_Kind
	uri             string
	definitionRange *scip.Range
	occurrences     []*IndexedOccurrence
	relationships   []*SymbolRelationship
	signature       *SCIPSignature
	documentation   string

	// Performance data
	accessCount  int64
	lastAccessed time.Time
	resolvedRefs []*ResolvedReference // Cached references
	cacheValid   bool
}

// IndexedOccurrence represents an optimized occurrence for fast access
type IndexedOccurrence struct {
	range_         *scip.Range
	role           int32
	syntaxKind     scip.SyntaxKind
	enclosingRange *scip.Range
	contextSymbols []string // Surrounding symbols for disambiguation
}

// SymbolGraph manages symbol relationships and cross-references
type SymbolGraph struct {
	symbols     map[string]*SymbolNode         // symbol -> node
	references  map[string][]*Reference        // symbol -> references
	inheritance map[string][]*SymbolNode       // inheritance relationships
	crossLang   map[string][]*CrossLanguageRef // cross-language refs
	typeGraph   map[string]*TypeRelation       // type relationships
	mutex       sync.RWMutex

	// Graph metrics
	nodeCount  int
	edgeCount  int
	avgDepth   float64
	lastUpdate time.Time
}

// SymbolNode represents a node in the symbol relationship graph
type SymbolNode struct {
	symbol     string
	kind       scip.SymbolInformation_Kind
	language   string
	uri        string
	definition *scip.Range

	// Relationships
	parents         []*SymbolNode // Inheritance/composition
	children        []*SymbolNode // Derived symbols
	references      []*Reference  // All references
	implementations []*SymbolNode // Interface implementations

	// Context information
	scope    *ScopeInfo
	typeInfo *TypeInfo

	// Performance tracking
	popularity   int64 // Access frequency
	lastAccessed time.Time
}

// Reference represents a symbol reference with context
type Reference struct {
	symbol        string
	uri           string
	range_        *scip.Range
	role          int32
	context       *ReferenceContext
	confidence    float64
	crossLanguage bool
}

// ReferenceContext provides disambiguation context
type ReferenceContext struct {
	enclosingScope  string
	nearbySymbols   []string
	syntaxContext   string
	semanticContext map[string]interface{}
}

// CrossLanguageRef represents cross-language symbol references
type CrossLanguageRef struct {
	sourceSymbol string
	sourceLang   string
	targetSymbol string
	targetLang   string
	relationship string // "calls", "imports", "extends", etc.
	confidence   float64
}

// TypeRelation represents type relationships for advanced resolution
type TypeRelation struct {
	fromType   string
	toType     string
	relation   string // "extends", "implements", "contains", etc.
	confidence float64
}

// ScopeInfo provides scope context for symbol disambiguation
type ScopeInfo struct {
	scopeType      string // "function", "class", "namespace", etc.
	parentScope    string
	childScopes    []string
	visibleSymbols []string
}

// TypeInfo provides type information for symbol resolution
type TypeInfo struct {
	typeName    string
	typeParams  []string
	baseTypes   []string
	interfaces  []string
	isGeneric   bool
	constraints map[string]string
}

// RangeCalculator provides advanced geometric operations for symbol ranges
type RangeCalculator struct {
	cache map[string]*RangeResult // Cached calculations
	mutex sync.RWMutex

	// Performance metrics
	calcCount   int64
	avgCalcTime time.Duration
}

// RangeResult represents cached range calculation results
type RangeResult struct {
	operation   string
	result      interface{}
	cachedAt    time.Time
	accessCount int64
}

// ResolverConfig provides configuration for symbol resolution
type ResolverConfig struct {
	// Performance settings
	MaxResolveTime time.Duration `json:"max_resolve_time"`
	CacheSize      int           `json:"cache_size"`
	CacheTTL       time.Duration `json:"cache_ttl"`
	MaxConcurrent  int           `json:"max_concurrent"`

	// Resolution settings
	EnableCrossLanguage bool    `json:"enable_cross_language"`
	EnableFuzzyMatching bool    `json:"enable_fuzzy_matching"`
	FuzzyThreshold      float64 `json:"fuzzy_threshold"`
	MaxContextDistance  int     `json:"max_context_distance"`
	EnableTypeInference bool    `json:"enable_type_inference"`

	// Index settings
	IndexUpdateInterval time.Duration `json:"index_update_interval"`
	EnableSpatialIndex  bool          `json:"enable_spatial_index"`
	SpatialIndexDepth   int           `json:"spatial_index_depth"`

	// Advanced features
	EnableInheritanceGraph bool `json:"enable_inheritance_graph"`
	EnableSemanticSearch   bool `json:"enable_semantic_search"`
	TrackSymbolPopularity  bool `json:"track_symbol_popularity"`
}

// ResolutionCache provides high-performance caching for symbol resolution
type ResolutionCache struct {
	entries map[string]*SymbolResolverCacheEntry
	lru     *SymbolResolverLRUList
	maxSize int
	ttl     time.Duration
	mutex   sync.RWMutex

	// Performance metrics
	hitCount    int64
	missCount   int64
	evictions   int64
	lastCleanup time.Time
}

// SymbolResolverCacheEntry represents a cached resolution result
type SymbolResolverCacheEntry struct {
	key         string
	result      *ResolvedSymbol
	cachedAt    time.Time
	accessCount int64
	lastAccess  time.Time
	next, prev  *SymbolResolverCacheEntry // LRU links
}

// SymbolResolverLRUList implements a doubly-linked list for LRU cache management
type SymbolResolverLRUList struct {
	head, tail *SymbolResolverCacheEntry
	size       int
}

// ResolverStats tracks performance and usage statistics
type ResolverStats struct {
	// Resolution metrics
	TotalResolutions      int64         `json:"total_resolutions"`
	SuccessfulResolutions int64         `json:"successful_resolutions"`
	FailedResolutions     int64         `json:"failed_resolutions"`
	AvgResolutionTime     time.Duration `json:"avg_resolution_time"`
	P95ResolutionTime     time.Duration `json:"p95_resolution_time"`
	P99ResolutionTime     time.Duration `json:"p99_resolution_time"`

	// Cache metrics
	CacheHitRate   float64 `json:"cache_hit_rate"`
	CacheSize      int     `json:"cache_size"`
	CacheEvictions int64   `json:"cache_evictions"`

	// Index metrics
	IndexSize       int       `json:"index_size"`
	SymbolCount     int       `json:"symbol_count"`
	DocumentCount   int       `json:"document_count"`
	LastIndexUpdate time.Time `json:"last_index_update"`

	// Performance metrics
	MemoryUsage           int64 `json:"memory_usage_bytes"`
	ConcurrentRequests    int64 `json:"concurrent_requests"`
	MaxConcurrentRequests int64 `json:"max_concurrent_requests"`

	// Advanced metrics
	CrossLanguageRefs int64 `json:"cross_language_refs"`
	InheritanceChains int64 `json:"inheritance_chains"`
	TypeInferences    int64 `json:"type_inferences"`

	mutex     sync.RWMutex
	lastReset time.Time
}

// ResolvedSymbol represents a fully resolved symbol with all context
type ResolvedSymbol struct {
	// Core symbol information
	Symbol      string                      `json:"symbol"`
	DisplayName string                      `json:"display_name"`
	Kind        scip.SymbolInformation_Kind `json:"kind"`
	URI         string                      `json:"uri"`
	Range       *scip.Range                 `json:"range"`

	// Resolution metadata
	Confidence     float64       `json:"confidence"`
	ResolutionTime time.Duration `json:"resolution_time"`
	ResolutionPath []string      `json:"resolution_path"`

	// Symbol details
	Definition     *SymbolDefinition  `json:"definition,omitempty"`
	References     []*SymbolReference `json:"references,omitempty"`
	RelatedSymbols []*RelatedSymbol   `json:"related_symbols,omitempty"`
	Documentation  string             `json:"documentation,omitempty"`
	Signature      *SCIPSignature     `json:"signature,omitempty"`

	// Context information
	Scope    *ScopeInfo `json:"scope,omitempty"`
	TypeInfo *TypeInfo  `json:"type_info,omitempty"`

	// Cross-language information
	CrossLanguageRefs []*CrossLanguageRef `json:"cross_language_refs,omitempty"`

	// Disambiguation data
	Context      *ReferenceContext    `json:"context,omitempty"`
	Alternatives []*AlternativeSymbol `json:"alternatives,omitempty"`
}

// SymbolDefinition represents a symbol definition location
type SymbolDefinition struct {
	URI        string      `json:"uri"`
	Range      *scip.Range `json:"range"`
	Symbol     string      `json:"symbol"`
	Confidence float64     `json:"confidence"`
}

// SymbolReference represents a symbol reference location
type SymbolReference struct {
	URI        string            `json:"uri"`
	Range      *scip.Range       `json:"range"`
	Role       int32             `json:"role"`
	Context    *ReferenceContext `json:"context,omitempty"`
	Confidence float64           `json:"confidence"`
}

// RelatedSymbol represents a symbol related through inheritance, composition, etc.
type RelatedSymbol struct {
	Symbol       string      `json:"symbol"`
	Relationship string      `json:"relationship"`
	URI          string      `json:"uri"`
	Range        *scip.Range `json:"range"`
	Confidence   float64     `json:"confidence"`
}

// AlternativeSymbol represents alternative symbol interpretations
type AlternativeSymbol struct {
	Symbol     string      `json:"symbol"`
	Reason     string      `json:"reason"`
	Confidence float64     `json:"confidence"`
	URI        string      `json:"uri"`
	Range      *scip.Range `json:"range"`
}

// Position represents a position for symbol resolution
type Position struct {
	Line      int32 `json:"line"`
	Character int32 `json:"character"`
}

const (
	// Performance targets from enterprise config
	TargetResolutionTimeP99 = 10 * time.Millisecond
	TargetCacheHitRate      = 0.85
	TargetCrossLanguageTime = 25 * time.Millisecond

	// Default configuration values
	DefaultCacheSize          = 10000
	DefaultCacheTTL           = 30 * time.Minute
	DefaultMaxConcurrent      = 100
	DefaultFuzzyThreshold     = 0.8
	DefaultMaxContextDistance = 50
	DefaultSpatialIndexDepth  = 8

	// Interval tree constants
	IntervalTreeRebalanceThreshold = 1000
	NodePoolSize                   = 1000
)

// NewSymbolResolver creates a new high-performance symbol resolver
func NewSymbolResolver(client *SCIPClient, config *ResolverConfig) (*SymbolResolver, error) {
	if client == nil {
		return nil, fmt.Errorf("SCIP client cannot be nil")
	}

	if config == nil {
		config = DefaultResolverConfig()
	}

	resolver := &SymbolResolver{
		scipClient:       client,
		positionIndex:    NewPositionIndex(),
		symbolGraph:      NewSymbolGraph(),
		rangeCalculator:  NewRangeCalculator(),
		config:           config,
		cache:            NewResolutionCache(config.CacheSize, config.CacheTTL),
		stats:            NewResolverStats(),
		lastOptimization: time.Now(),
	}

	// Initialize position index from existing SCIP data
	if err := resolver.initializeFromSCIPClient(); err != nil {
		return nil, fmt.Errorf("failed to initialize resolver: %w", err)
	}

	return resolver, nil
}

// DefaultResolverConfig returns default configuration optimized for performance
func DefaultResolverConfig() *ResolverConfig {
	return &ResolverConfig{
		MaxResolveTime:         TargetResolutionTimeP99,
		CacheSize:              DefaultCacheSize,
		CacheTTL:               DefaultCacheTTL,
		MaxConcurrent:          DefaultMaxConcurrent,
		EnableCrossLanguage:    true,
		EnableFuzzyMatching:    true,
		FuzzyThreshold:         DefaultFuzzyThreshold,
		MaxContextDistance:     DefaultMaxContextDistance,
		EnableTypeInference:    true,
		IndexUpdateInterval:    5 * time.Minute,
		EnableSpatialIndex:     true,
		SpatialIndexDepth:      DefaultSpatialIndexDepth,
		EnableInheritanceGraph: true,
		EnableSemanticSearch:   true,
		TrackSymbolPopularity:  true,
	}
}

// ResolveSymbolAtPosition performs precise position-based symbol resolution
func (r *SymbolResolver) ResolveSymbolAtPosition(uri string, position Position) (*ResolvedSymbol, error) {
	startTime := time.Now()
	atomic.AddInt64(&r.requestCount, 1)

	// Check cache first
	cacheKey := fmt.Sprintf("%s:%d:%d", uri, position.Line, position.Character)
	if cached := r.cache.Get(cacheKey); cached != nil {
		r.stats.recordCacheHit()
		return cached, nil
	}

	r.stats.recordCacheMiss()

	// Context for timeout handling
	ctx, cancel := context.WithTimeout(context.Background(), r.config.MaxResolveTime)
	defer cancel()

	// Perform resolution
	result, err := r.resolveSymbolAtPositionInternal(ctx, uri, position)
	if err != nil {
		r.stats.recordFailure(time.Since(startTime))
		return nil, err
	}

	// Cache successful result
	r.cache.Set(cacheKey, result)

	// Update performance metrics
	resolutionTime := time.Since(startTime)
	atomic.AddInt64(&r.totalResolutionTime, int64(resolutionTime))
	r.stats.recordSuccess(resolutionTime)

	result.ResolutionTime = resolutionTime

	return result, nil
}

// resolveSymbolAtPositionInternal performs the actual resolution logic
func (r *SymbolResolver) resolveSymbolAtPositionInternal(ctx context.Context, uri string, position Position) (*ResolvedSymbol, error) {
	// Get document index
	docIndex, err := r.positionIndex.GetDocumentIndex(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to get document index: %w", err)
	}

	// Convert LSP position to byte offset
	byteOffset := r.positionToByteOffset(docIndex, position)

	// Find symbol at position using interval tree
	candidates := docIndex.FindSymbolsAtPosition(byteOffset)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no symbol found at position %d:%d", position.Line, position.Character)
	}

	// Select best candidate using disambiguation
	bestCandidate, err := r.disambiguateSymbols(ctx, candidates, position)
	if err != nil {
		return nil, fmt.Errorf("failed to disambiguate symbols: %w", err)
	}

	// Build resolved symbol with full context
	resolved, err := r.buildResolvedSymbol(ctx, bestCandidate, uri, position)
	if err != nil {
		return nil, fmt.Errorf("failed to build resolved symbol: %w", err)
	}

	return resolved, nil
}

// FindDefinition finds the definition of a resolved symbol
func (r *SymbolResolver) FindDefinition(symbol *ResolvedSymbol) (*SymbolDefinition, error) {
	if symbol == nil {
		return nil, fmt.Errorf("symbol cannot be nil")
	}

	// Check cache
	cacheKey := fmt.Sprintf("def:%s", symbol.Symbol)
	if cached := r.cache.GetDefinition(cacheKey); cached != nil {
		return cached, nil
	}

	// Look up symbol in graph
	node := r.symbolGraph.GetSymbolNode(symbol.Symbol)
	if node == nil {
		return nil, fmt.Errorf("symbol not found in graph: %s", symbol.Symbol)
	}

	definition := &SymbolDefinition{
		URI:        node.uri,
		Range:      node.definition,
		Symbol:     symbol.Symbol,
		Confidence: 1.0,
	}

	// Cache result
	r.cache.SetDefinition(cacheKey, definition)

	return definition, nil
}

// FindReferences finds all references to a symbol
func (r *SymbolResolver) FindReferences(symbol *ResolvedSymbol) ([]*SymbolReference, error) {
	if symbol == nil {
		return nil, fmt.Errorf("symbol cannot be nil")
	}

	// Check cache
	cacheKey := fmt.Sprintf("refs:%s", symbol.Symbol)
	if cached := r.cache.GetReferences(cacheKey); cached != nil {
		return cached, nil
	}

	// Get all references from symbol graph
	refs := r.symbolGraph.GetReferences(symbol.Symbol)

	// Convert to SymbolReference format
	symbolRefs := make([]*SymbolReference, 0, len(refs))
	for _, ref := range refs {
		symbolRef := &SymbolReference{
			URI:        ref.uri,
			Range:      ref.range_,
			Role:       ref.role,
			Context:    ref.context,
			Confidence: ref.confidence,
		}
		symbolRefs = append(symbolRefs, symbolRef)
	}

	// Cache result
	r.cache.SetReferences(cacheKey, symbolRefs)

	return symbolRefs, nil
}

// FindRelatedSymbols finds symbols related through inheritance, composition, etc.
func (r *SymbolResolver) FindRelatedSymbols(symbol *ResolvedSymbol) ([]*RelatedSymbol, error) {
	if symbol == nil {
		return nil, fmt.Errorf("symbol cannot be nil")
	}

	// Check cache
	cacheKey := fmt.Sprintf("related:%s", symbol.Symbol)
	if cached := r.cache.GetRelatedSymbols(cacheKey); cached != nil {
		return cached, nil
	}

	var related []*RelatedSymbol

	// Find inheritance relationships
	if r.config.EnableInheritanceGraph {
		inheritanceRefs := r.symbolGraph.GetInheritanceChain(symbol.Symbol)
		for _, ref := range inheritanceRefs {
			related = append(related, &RelatedSymbol{
				Symbol:       ref.symbol,
				Relationship: "inherits",
				URI:          ref.uri,
				Range:        ref.definition,
				Confidence:   0.95,
			})
		}
	}

	// Find cross-language references
	if r.config.EnableCrossLanguage {
		crossRefs := r.symbolGraph.GetCrossLanguageRefs(symbol.Symbol)
		for _, ref := range crossRefs {
			related = append(related, &RelatedSymbol{
				Symbol:       ref.targetSymbol,
				Relationship: ref.relationship,
				URI:          "",  // Will be resolved separately
				Range:        nil, // Will be resolved separately
				Confidence:   ref.confidence,
			})
		}
	}

	// Cache result
	r.cache.SetRelatedSymbols(cacheKey, related)

	return related, nil
}

// BuildPositionIndex builds the spatial index from SCIP documents
func (r *SymbolResolver) BuildPositionIndex(documents []*scip.Document) error {
	r.positionIndex.mutex.Lock()
	defer r.positionIndex.mutex.Unlock()

	startTime := time.Now()

	// Clear existing index
	r.positionIndex.documents = make(map[string]*DocumentIndex)
	r.positionIndex.symbols = make(map[string]*IndexedSymbol)

	// Process each document
	for _, doc := range documents {
		if err := r.indexDocument(doc); err != nil {
			return fmt.Errorf("failed to index document %s: %w", doc.RelativePath, err)
		}
	}

	// Build spatial tree
	r.positionIndex.spatialTree = r.buildSpatialTree()

	// Update statistics
	r.positionIndex.lastReindex = time.Now()
	r.stats.recordIndexUpdate(time.Since(startTime), len(documents))

	return nil
}

// UpdatePositionIndex incrementally updates the index for a single document
func (r *SymbolResolver) UpdatePositionIndex(document *scip.Document) error {
	if document == nil {
		return fmt.Errorf("document cannot be nil")
	}

	r.positionIndex.mutex.Lock()
	defer r.positionIndex.mutex.Unlock()

	// Remove existing document index
	if existing, exists := r.positionIndex.documents[document.RelativePath]; exists {
		r.removeDocumentFromIndex(existing)
	}

	// Index the updated document
	if err := r.indexDocument(document); err != nil {
		return fmt.Errorf("failed to update document index: %w", err)
	}

	// Rebuild spatial tree if necessary
	if r.shouldRebuildSpatialTree() {
		r.positionIndex.spatialTree = r.buildSpatialTree()
	}

	return nil
}

// GetStats returns comprehensive resolver statistics
func (r *SymbolResolver) GetStats() *ResolverStats {
	r.stats.mutex.RLock()
	defer r.stats.mutex.RUnlock()

	// Create a copy to avoid concurrent modification
	stats := *r.stats

	// Calculate derived metrics
	if stats.TotalResolutions > 0 {
		totalTime := atomic.LoadInt64(&r.totalResolutionTime)
		stats.AvgResolutionTime = time.Duration(totalTime / stats.TotalResolutions)
	}

	// Update cache statistics
	stats.CacheHitRate = r.cache.GetHitRate()
	stats.CacheSize = r.cache.Size()
	stats.CacheEvictions = r.cache.GetEvictions()

	// Update index statistics
	stats.IndexSize = r.positionIndex.Size()
	stats.SymbolCount = r.symbolGraph.SymbolCount()
	stats.DocumentCount = r.positionIndex.DocumentCount()

	return &stats
}

// Close cleans up resources and shuts down the resolver
func (r *SymbolResolver) Close() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Clear caches
	r.cache.Clear()

	// Clear indices
	r.positionIndex.Clear()
	r.symbolGraph.Clear()
	r.rangeCalculator.Clear()

	return nil
}

// Helper methods for PositionIndex

// NewPositionIndex creates a new position index
func NewPositionIndex() *PositionIndex {
	return &PositionIndex{
		documents:   make(map[string]*DocumentIndex),
		symbols:     make(map[string]*IndexedSymbol),
		spatialTree: NewIntervalTree(),
	}
}

// GetDocumentIndex retrieves or creates a document index
func (pi *PositionIndex) GetDocumentIndex(uri string) (*DocumentIndex, error) {
	pi.mutex.RLock()
	if doc, exists := pi.documents[uri]; exists {
		pi.mutex.RUnlock()
		return doc, nil
	}
	pi.mutex.RUnlock()

	return nil, fmt.Errorf("document index not found: %s", uri)
}

// Size returns the total size of the position index
func (pi *PositionIndex) Size() int {
	pi.mutex.RLock()
	defer pi.mutex.RUnlock()
	return len(pi.documents)
}

// DocumentCount returns the number of indexed documents
func (pi *PositionIndex) DocumentCount() int {
	pi.mutex.RLock()
	defer pi.mutex.RUnlock()
	return len(pi.documents)
}

// Clear clears all position index data
func (pi *PositionIndex) Clear() {
	pi.mutex.Lock()
	defer pi.mutex.Unlock()

	pi.documents = make(map[string]*DocumentIndex)
	pi.symbols = make(map[string]*IndexedSymbol)
	pi.spatialTree = NewIntervalTree()
}

// Helper methods for DocumentIndex

// FindSymbolsAtPosition finds symbols at a specific byte position
func (di *DocumentIndex) FindSymbolsAtPosition(byteOffset int32) []*IndexedSymbol {
	if di.intervalTree == nil {
		return nil
	}

	// Query interval tree for overlapping ranges
	overlapping := di.intervalTree.QueryPoint(byteOffset)

	// Extract symbols from overlapping intervals
	var symbols []*IndexedSymbol
	for _, interval := range overlapping {
		if symbolList, ok := interval.data.([]*IndexedSymbol); ok {
			symbols = append(symbols, symbolList...)
		}
	}

	// Sort by specificity (smaller ranges first)
	sort.Slice(symbols, func(i, j int) bool {
		rangeI := symbols[i].definitionRange
		rangeJ := symbols[j].definitionRange

		if rangeI == nil || rangeJ == nil {
			return rangeI != nil
		}

		sizeI := rangeI.End.Character - rangeI.Start.Character
		sizeJ := rangeJ.End.Character - rangeJ.Start.Character

		return sizeI < sizeJ
	})

	return symbols
}

// Helper methods for IntervalTree

// NewIntervalTree creates a new interval tree with node pooling
func NewIntervalTree() *IntervalTree {
	tree := &IntervalTree{}
	tree.nodePool.New = func() interface{} {
		return &IntervalNode{}
	}
	return tree
}

// Insert adds an interval to the tree
func (it *IntervalTree) Insert(start, end int32, symbols []*IndexedSymbol) {
	interval := &Interval{
		start: start,
		end:   end,
		data:  symbols,
	}

	node := it.nodePool.Get().(*IntervalNode)
	node.interval = interval
	node.symbols = symbols
	node.maxEnd = end
	node.left = nil
	node.right = nil
	node.height = 1

	it.root = it.insertNode(it.root, node)
	it.size++
}

// QueryPoint finds all intervals that contain a specific point
func (it *IntervalTree) QueryPoint(point int32) []*Interval {
	var result []*Interval
	it.queryPointRecursive(it.root, point, &result)
	return result
}

// insertNode recursively inserts a node maintaining AVL tree properties
func (it *IntervalTree) insertNode(root, newNode *IntervalNode) *IntervalNode {
	if root == nil {
		return newNode
	}

	// Standard BST insertion based on start position
	if newNode.interval.start < root.interval.start {
		root.left = it.insertNode(root.left, newNode)
	} else {
		root.right = it.insertNode(root.right, newNode)
	}

	// Update height and max end
	root.height = 1 + maxInt(it.getHeight(root.left), it.getHeight(root.right))
	root.maxEnd = maxInt32(root.interval.end, maxInt32(it.getMaxEnd(root.left), it.getMaxEnd(root.right)))

	// AVL rebalancing
	balance := it.getBalance(root)

	// Left heavy
	if balance > 1 {
		if newNode.interval.start < root.left.interval.start {
			return it.rotateRight(root)
		} else {
			root.left = it.rotateLeft(root.left)
			return it.rotateRight(root)
		}
	}

	// Right heavy
	if balance < -1 {
		if newNode.interval.start > root.right.interval.start {
			return it.rotateLeft(root)
		} else {
			root.right = it.rotateRight(root.right)
			return it.rotateLeft(root)
		}
	}

	return root
}

// queryPointRecursive recursively finds all intervals containing a point
func (it *IntervalTree) queryPointRecursive(node *IntervalNode, point int32, result *[]*Interval) {
	if node == nil {
		return
	}

	// Check if current interval contains the point
	if node.interval.start <= point && point <= node.interval.end {
		*result = append(*result, node.interval)
	}

	// Recursively search left subtree if it might contain overlapping intervals
	if node.left != nil && it.getMaxEnd(node.left) >= point {
		it.queryPointRecursive(node.left, point, result)
	}

	// Recursively search right subtree if necessary
	if node.right != nil && node.interval.start <= point {
		it.queryPointRecursive(node.right, point, result)
	}
}

// AVL tree helper methods
func (it *IntervalTree) getHeight(node *IntervalNode) int {
	if node == nil {
		return 0
	}
	return node.height
}

func (it *IntervalTree) getMaxEnd(node *IntervalNode) int32 {
	if node == nil {
		return 0
	}
	return node.maxEnd
}

func (it *IntervalTree) getBalance(node *IntervalNode) int {
	if node == nil {
		return 0
	}
	return it.getHeight(node.left) - it.getHeight(node.right)
}

func (it *IntervalTree) rotateLeft(node *IntervalNode) *IntervalNode {
	right := node.right
	node.right = right.left
	right.left = node

	// Update heights and maxEnd
	node.height = 1 + maxInt(it.getHeight(node.left), it.getHeight(node.right))
	right.height = 1 + maxInt(it.getHeight(right.left), it.getHeight(right.right))

	node.maxEnd = maxInt32(node.interval.end, maxInt32(it.getMaxEnd(node.left), it.getMaxEnd(node.right)))
	right.maxEnd = maxInt32(right.interval.end, maxInt32(it.getMaxEnd(right.left), it.getMaxEnd(right.right)))

	return right
}

func (it *IntervalTree) rotateRight(node *IntervalNode) *IntervalNode {
	left := node.left
	node.left = left.right
	left.right = node

	// Update heights and maxEnd
	node.height = 1 + maxInt(it.getHeight(node.left), it.getHeight(node.right))
	left.height = 1 + maxInt(it.getHeight(left.left), it.getHeight(left.right))

	node.maxEnd = maxInt32(node.interval.end, maxInt32(it.getMaxEnd(node.left), it.getMaxEnd(node.right)))
	left.maxEnd = maxInt32(left.interval.end, maxInt32(it.getMaxEnd(left.left), it.getMaxEnd(left.right)))

	return left
}

// Helper methods for SymbolGraph

// NewSymbolGraph creates a new symbol relationship graph
func NewSymbolGraph() *SymbolGraph {
	return &SymbolGraph{
		symbols:     make(map[string]*SymbolNode),
		references:  make(map[string][]*Reference),
		inheritance: make(map[string][]*SymbolNode),
		crossLang:   make(map[string][]*CrossLanguageRef),
		typeGraph:   make(map[string]*TypeRelation),
	}
}

// GetSymbolNode retrieves a symbol node from the graph
func (sg *SymbolGraph) GetSymbolNode(symbol string) *SymbolNode {
	sg.mutex.RLock()
	defer sg.mutex.RUnlock()
	return sg.symbols[symbol]
}

// GetReferences retrieves all references for a symbol
func (sg *SymbolGraph) GetReferences(symbol string) []*Reference {
	sg.mutex.RLock()
	defer sg.mutex.RUnlock()
	return sg.references[symbol]
}

// GetInheritanceChain retrieves inheritance relationships for a symbol
func (sg *SymbolGraph) GetInheritanceChain(symbol string) []*SymbolNode {
	sg.mutex.RLock()
	defer sg.mutex.RUnlock()
	return sg.inheritance[symbol]
}

// GetCrossLanguageRefs retrieves cross-language references for a symbol
func (sg *SymbolGraph) GetCrossLanguageRefs(symbol string) []*CrossLanguageRef {
	sg.mutex.RLock()
	defer sg.mutex.RUnlock()
	return sg.crossLang[symbol]
}

// SymbolCount returns the total number of symbols in the graph
func (sg *SymbolGraph) SymbolCount() int {
	sg.mutex.RLock()
	defer sg.mutex.RUnlock()
	return len(sg.symbols)
}

// Clear clears all symbol graph data
func (sg *SymbolGraph) Clear() {
	sg.mutex.Lock()
	defer sg.mutex.Unlock()

	sg.symbols = make(map[string]*SymbolNode)
	sg.references = make(map[string][]*Reference)
	sg.inheritance = make(map[string][]*SymbolNode)
	sg.crossLang = make(map[string][]*CrossLanguageRef)
	sg.typeGraph = make(map[string]*TypeRelation)
}

// Helper methods for RangeCalculator

// NewRangeCalculator creates a new range calculator
func NewRangeCalculator() *RangeCalculator {
	return &RangeCalculator{
		cache: make(map[string]*RangeResult),
	}
}

// Intersects checks if two ranges intersect
func (rc *RangeCalculator) Intersects(r1, r2 *scip.Range) bool {
	if r1 == nil || r2 == nil {
		return false
	}

	// Check line intersection first
	if r1.End.Line < r2.Start.Line || r2.End.Line < r1.Start.Line {
		return false
	}

	// If on same line, check character intersection
	if r1.Start.Line == r2.Start.Line && r1.End.Line == r2.End.Line {
		return !(r1.End.Character < r2.Start.Character || r2.End.Character < r1.Start.Character)
	}

	return true
}

// Contains checks if one range contains another
func (rc *RangeCalculator) Contains(outer, inner *scip.Range) bool {
	if outer == nil || inner == nil {
		return false
	}

	// Check if inner range is completely within outer range
	if inner.Start.Line < outer.Start.Line || inner.End.Line > outer.End.Line {
		return false
	}

	// Check start position
	if inner.Start.Line == outer.Start.Line && inner.Start.Character < outer.Start.Character {
		return false
	}

	// Check end position
	if inner.End.Line == outer.End.Line && inner.End.Character > outer.End.Character {
		return false
	}

	return true
}

// Distance calculates the distance between two ranges
func (rc *RangeCalculator) Distance(r1, r2 *scip.Range) int32 {
	if r1 == nil || r2 == nil {
		return -1
	}

	// If ranges intersect, distance is 0
	if rc.Intersects(r1, r2) {
		return 0
	}

	// Calculate minimum distance between ranges
	var distance int32

	if r1.End.Line < r2.Start.Line {
		// r1 is completely before r2
		distance = (r2.Start.Line-r1.End.Line)*1000 + (r2.Start.Character - r1.End.Character)
	} else if r2.End.Line < r1.Start.Line {
		// r2 is completely before r1
		distance = (r1.Start.Line-r2.End.Line)*1000 + (r1.Start.Character - r2.End.Character)
	} else {
		// Ranges are on overlapping lines but don't intersect
		if r1.End.Character < r2.Start.Character {
			distance = r2.Start.Character - r1.End.Character
		} else {
			distance = r1.Start.Character - r2.End.Character
		}
	}

	return distance
}

// FindNearestSymbol finds the nearest symbol to a position
func (rc *RangeCalculator) FindNearestSymbol(position Position, symbols []*IndexedSymbol) *IndexedSymbol {
	if len(symbols) == 0 {
		return nil
	}

	var nearest *IndexedSymbol
	var minDistance int32 = -1

	posRange := &scip.Range{
		Start: scip.Position{Line: position.Line, Character: position.Character},
		End:   scip.Position{Line: position.Line, Character: position.Character},
	}

	for _, symbol := range symbols {
		if symbol.definitionRange == nil {
			continue
		}

		distance := rc.Distance(posRange, symbol.definitionRange)
		if distance >= 0 && (minDistance < 0 || distance < minDistance) {
			nearest = symbol
			minDistance = distance
		}
	}

	return nearest
}

// ExpandRange intelligently expands a range based on context
func (rc *RangeCalculator) ExpandRange(range_ *scip.Range, context map[string]interface{}) *scip.Range {
	if range_ == nil {
		return nil
	}

	// Create expanded range (simple implementation)
	expanded := &scip.Range{
		Start: scip.Position{
			Line:      range_.Start.Line,
			Character: maxInt32(0, range_.Start.Character-1),
		},
		End: scip.Position{
			Line:      range_.End.Line,
			Character: range_.End.Character + 1,
		},
	}

	return expanded
}

// Clear clears the range calculator cache
func (rc *RangeCalculator) Clear() {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()
	rc.cache = make(map[string]*RangeResult)
}

// Helper methods for ResolutionCache

// NewResolutionCache creates a new resolution cache
func NewResolutionCache(maxSize int, ttl time.Duration) *ResolutionCache {
	return &ResolutionCache{
		entries: make(map[string]*SymbolResolverCacheEntry),
		lru:     &SymbolResolverLRUList{},
		maxSize: maxSize,
		ttl:     ttl,
	}
}

// Get retrieves a cached resolution result
func (rc *ResolutionCache) Get(key string) *ResolvedSymbol {
	rc.mutex.RLock()
	entry, exists := rc.entries[key]
	rc.mutex.RUnlock()

	if !exists {
		atomic.AddInt64(&rc.missCount, 1)
		return nil
	}

	// Check TTL
	if time.Since(entry.cachedAt) > rc.ttl {
		rc.mutex.Lock()
		rc.evict(key)
		rc.mutex.Unlock()
		atomic.AddInt64(&rc.missCount, 1)
		return nil
	}

	// Move to front of LRU
	rc.mutex.Lock()
	rc.lru.moveToFront(entry)
	entry.lastAccess = time.Now()
	entry.accessCount++
	rc.mutex.Unlock()

	atomic.AddInt64(&rc.hitCount, 1)
	return entry.result
}

// Set stores a resolution result in the cache
func (rc *ResolutionCache) Set(key string, result *ResolvedSymbol) {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()

	// Remove existing entry if present
	if existing, exists := rc.entries[key]; exists {
		rc.lru.remove(existing)
	}

	// Create new entry
	entry := &SymbolResolverCacheEntry{
		key:         key,
		result:      result,
		cachedAt:    time.Now(),
		lastAccess:  time.Now(),
		accessCount: 1,
	}

	// Add to cache
	rc.entries[key] = entry
	rc.lru.addToFront(entry)

	// Evict if necessary
	for len(rc.entries) > rc.maxSize {
		oldest := rc.lru.removeLast()
		if oldest != nil {
			delete(rc.entries, oldest.key)
			atomic.AddInt64(&rc.evictions, 1)
		}
	}
}

// GetDefinition retrieves cached definition
func (rc *ResolutionCache) GetDefinition(key string) *SymbolDefinition {
	// Simplified implementation - in practice would use separate cache
	return nil
}

// SetDefinition stores cached definition
func (rc *ResolutionCache) SetDefinition(key string, def *SymbolDefinition) {
	// Simplified implementation
}

// GetReferences retrieves cached references
func (rc *ResolutionCache) GetReferences(key string) []*SymbolReference {
	// Simplified implementation
	return nil
}

// SetReferences stores cached references
func (rc *ResolutionCache) SetReferences(key string, refs []*SymbolReference) {
	// Simplified implementation
}

// GetRelatedSymbols retrieves cached related symbols
func (rc *ResolutionCache) GetRelatedSymbols(key string) []*RelatedSymbol {
	// Simplified implementation
	return nil
}

// SetRelatedSymbols stores cached related symbols
func (rc *ResolutionCache) SetRelatedSymbols(key string, related []*RelatedSymbol) {
	// Simplified implementation
}

// GetHitRate returns the cache hit rate
func (rc *ResolutionCache) GetHitRate() float64 {
	hits := float64(atomic.LoadInt64(&rc.hitCount))
	misses := float64(atomic.LoadInt64(&rc.missCount))
	total := hits + misses

	if total == 0 {
		return 0
	}

	return hits / total
}

// Size returns the current cache size
func (rc *ResolutionCache) Size() int {
	rc.mutex.RLock()
	defer rc.mutex.RUnlock()
	return len(rc.entries)
}

// GetEvictions returns the number of cache evictions
func (rc *ResolutionCache) GetEvictions() int64 {
	return atomic.LoadInt64(&rc.evictions)
}

// Clear clears all cache entries
func (rc *ResolutionCache) Clear() {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()

	rc.entries = make(map[string]*SymbolResolverCacheEntry)
	rc.lru = &SymbolResolverLRUList{}
}

// evict removes an entry from the cache
func (rc *ResolutionCache) evict(key string) {
	if entry, exists := rc.entries[key]; exists {
		rc.lru.remove(entry)
		delete(rc.entries, key)
		atomic.AddInt64(&rc.evictions, 1)
	}
}

// Helper methods for SymbolResolverLRUList

// addToFront adds an entry to the front of the LRU list
func (lru *SymbolResolverLRUList) addToFront(entry *SymbolResolverCacheEntry) {
	if lru.head == nil {
		lru.head = entry
		lru.tail = entry
	} else {
		entry.next = lru.head
		lru.head.prev = entry
		lru.head = entry
	}
	lru.size++
}

// remove removes an entry from the LRU list
func (lru *SymbolResolverLRUList) remove(entry *SymbolResolverCacheEntry) {
	if entry.prev != nil {
		entry.prev.next = entry.next
	} else {
		lru.head = entry.next
	}

	if entry.next != nil {
		entry.next.prev = entry.prev
	} else {
		lru.tail = entry.prev
	}

	entry.prev = nil
	entry.next = nil
	lru.size--
}

// moveToFront moves an entry to the front of the LRU list
func (lru *SymbolResolverLRUList) moveToFront(entry *SymbolResolverCacheEntry) {
	lru.remove(entry)
	lru.addToFront(entry)
}

// removeLast removes and returns the last entry in the LRU list
func (lru *SymbolResolverLRUList) removeLast() *SymbolResolverCacheEntry {
	if lru.tail == nil {
		return nil
	}

	last := lru.tail
	lru.remove(last)
	return last
}

// Helper methods for ResolverStats

// NewResolverStats creates new resolver statistics
func NewResolverStats() *ResolverStats {
	return &ResolverStats{
		lastReset: time.Now(),
	}
}

// recordSuccess records a successful resolution
func (rs *ResolverStats) recordSuccess(duration time.Duration) {
	rs.mutex.Lock()
	defer rs.mutex.Unlock()

	rs.TotalResolutions++
	rs.SuccessfulResolutions++

	// Update timing statistics
	if rs.TotalResolutions == 1 {
		rs.AvgResolutionTime = duration
	} else {
		// Exponential moving average
		alpha := 0.1
		rs.AvgResolutionTime = time.Duration(float64(rs.AvgResolutionTime)*(1-alpha) + float64(duration)*alpha)
	}

	// Update P95/P99 (simplified - would use proper percentile calculation in production)
	if duration > rs.P95ResolutionTime {
		rs.P95ResolutionTime = duration
	}
	if duration > rs.P99ResolutionTime {
		rs.P99ResolutionTime = duration
	}
}

// recordFailure records a failed resolution
func (rs *ResolverStats) recordFailure(duration time.Duration) {
	rs.mutex.Lock()
	defer rs.mutex.Unlock()

	rs.TotalResolutions++
	rs.FailedResolutions++
}

// recordCacheHit records a cache hit
func (rs *ResolverStats) recordCacheHit() {
	// Cache hit rate is calculated by the cache itself
}

// recordCacheMiss records a cache miss
func (rs *ResolverStats) recordCacheMiss() {
	// Cache miss rate is calculated by the cache itself
}

// recordIndexUpdate records an index update
func (rs *ResolverStats) recordIndexUpdate(duration time.Duration, documentCount int) {
	rs.mutex.Lock()
	defer rs.mutex.Unlock()

	rs.LastIndexUpdate = time.Now()
	rs.DocumentCount = documentCount
}

// Private helper methods

// initializeFromSCIPClient initializes the resolver from existing SCIP client data
func (r *SymbolResolver) initializeFromSCIPClient() error {
	// Get all loaded indices from SCIP client
	indices := r.scipClient.GetLoadedIndices()

	// Extract documents from all indices
	var allDocuments []*scip.Document
	for _, indexData := range indices {
		allDocuments = append(allDocuments, indexData.Documents...)
	}

	// Build position index
	if err := r.BuildPositionIndex(allDocuments); err != nil {
		return fmt.Errorf("failed to build position index: %w", err)
	}

	// Build symbol graph
	if err := r.buildSymbolGraph(allDocuments); err != nil {
		return fmt.Errorf("failed to build symbol graph: %w", err)
	}

	return nil
}

// indexDocument indexes a single SCIP document
func (r *SymbolResolver) indexDocument(doc *scip.Document) error {
	// Create document index
	docIndex := &DocumentIndex{
		uri:          doc.RelativePath,
		symbolMap:    make(map[string]*IndexedSymbol),
		intervalTree: NewIntervalTree(),
		lastModified: time.Now(),
	}

	// Process all symbol occurrences
	for _, occurrence := range doc.Occurrences {
		if occurrence.Range == nil {
			continue
		}

		// Create indexed symbol if not exists
		symbolKey := occurrence.Symbol
		indexedSymbol, exists := r.positionIndex.symbols[symbolKey]
		if !exists {
			indexedSymbol = &IndexedSymbol{
				symbol:      occurrence.Symbol,
				displayName: occurrence.Symbol, // Would extract display name
				uri:         doc.RelativePath,
				occurrences: make([]*IndexedOccurrence, 0),
			}
			r.positionIndex.symbols[symbolKey] = indexedSymbol
		}

		// Create indexed occurrence
		indexedOcc := &IndexedOccurrence{
			range_:         ConvertSCIPRangeToRange(occurrence.Range),
			role:           occurrence.SymbolRoles,
			syntaxKind:     occurrence.SyntaxKind,
			enclosingRange: ConvertSCIPRangeToRange(occurrence.EnclosingRange),
		}

		indexedSymbol.occurrences = append(indexedSymbol.occurrences, indexedOcc)

		// Add to document symbol map
		docIndex.symbolMap[symbolKey] = indexedSymbol

		// Add to interval tree
		// SCIP range format: [start_line, start_character, end_line, end_character]
		if len(occurrence.Range) >= 4 {
			docIndex.intervalTree.Insert(
				occurrence.Range[1], // start_character
				occurrence.Range[3], // end_character
				[]*IndexedSymbol{indexedSymbol},
			)
		}

		docIndex.symbolCount++
	}

	// Store document index
	r.positionIndex.documents[doc.RelativePath] = docIndex

	return nil
}

// buildSymbolGraph builds the symbol relationship graph
func (r *SymbolResolver) buildSymbolGraph(documents []*scip.Document) error {
	// Process all documents to build relationships
	for _, doc := range documents {
		if err := r.processDocumentForGraph(doc); err != nil {
			return fmt.Errorf("failed to process document for graph: %w", err)
		}
	}

	return nil
}

// processDocumentForGraph processes a document to extract symbol relationships
func (r *SymbolResolver) processDocumentForGraph(doc *scip.Document) error {
	// Extract symbols and their relationships
	for _, symInfo := range doc.Symbols {
		if symInfo == nil {
			continue
		}

		// Create symbol node
		node := &SymbolNode{
			symbol:       symInfo.Symbol,
			kind:         symInfo.Kind,
			uri:          doc.RelativePath,
			lastAccessed: time.Now(),
		}

		// Process relationships
		for _, rel := range symInfo.Relationships {
			if rel == nil {
				continue
			}

			// Add to symbol graph based on relationship type
			r.symbolGraph.mutex.Lock()
			r.symbolGraph.symbols[symInfo.Symbol] = node
			r.symbolGraph.mutex.Unlock()
		}
	}

	return nil
}

// positionToByteOffset converts LSP position to byte offset
func (r *SymbolResolver) positionToByteOffset(docIndex *DocumentIndex, position Position) int32 {
	// Simplified implementation - would use proper line indexing
	return position.Line*1000 + position.Character
}

// disambiguateSymbols selects the best symbol candidate from multiple options
func (r *SymbolResolver) disambiguateSymbols(ctx context.Context, candidates []*IndexedSymbol, position Position) (*IndexedSymbol, error) {
	if len(candidates) == 1 {
		return candidates[0], nil
	}

	// Use various disambiguation strategies

	// 1. Prefer more specific symbols (smaller ranges)
	sort.Slice(candidates, func(i, j int) bool {
		rangeI := candidates[i].definitionRange
		rangeJ := candidates[j].definitionRange

		if rangeI == nil || rangeJ == nil {
			return rangeI != nil
		}

		sizeI := rangeI.End.Character - rangeI.Start.Character
		sizeJ := rangeJ.End.Character - rangeJ.Start.Character

		return sizeI < sizeJ
	})

	// 2. Consider symbol popularity
	bestCandidate := candidates[0]
	for _, candidate := range candidates[1:] {
		if candidate.accessCount > bestCandidate.accessCount {
			bestCandidate = candidate
		}
	}

	return bestCandidate, nil
}

// buildResolvedSymbol constructs a complete ResolvedSymbol with all context
func (r *SymbolResolver) buildResolvedSymbol(ctx context.Context, symbol *IndexedSymbol, uri string, position Position) (*ResolvedSymbol, error) {
	resolved := &ResolvedSymbol{
		Symbol:         symbol.symbol,
		DisplayName:    symbol.displayName,
		Kind:           symbol.kind,
		URI:            uri,
		Range:          symbol.definitionRange,
		Confidence:     1.0,
		ResolutionPath: []string{"position_based", "disambiguation", "context_resolution"},
	}

	// Add documentation if available
	resolved.Documentation = symbol.documentation

	// Add signature if available
	resolved.Signature = symbol.signature

	// Update access tracking
	atomic.AddInt64(&symbol.accessCount, 1)
	symbol.lastAccessed = time.Now()

	return resolved, nil
}

// buildSpatialTree builds a spatial tree for efficient range queries
func (r *SymbolResolver) buildSpatialTree() *IntervalTree {
	tree := NewIntervalTree()

	// Add all symbol ranges to the spatial tree
	for _, symbol := range r.positionIndex.symbols {
		if symbol.definitionRange != nil {
			tree.Insert(
				symbol.definitionRange.Start.Character,
				symbol.definitionRange.End.Character,
				[]*IndexedSymbol{symbol},
			)
		}
	}

	return tree
}

// removeDocumentFromIndex removes a document from the position index
func (r *SymbolResolver) removeDocumentFromIndex(docIndex *DocumentIndex) {
	// Remove symbols associated with this document
	for symbolKey, _ := range docIndex.symbolMap {
		// Remove from global symbol index if this is the only reference
		if globalSymbol, exists := r.positionIndex.symbols[symbolKey]; exists {
			if globalSymbol.uri == docIndex.uri {
				delete(r.positionIndex.symbols, symbolKey)
			}
		}
	}

	// Remove document index
	delete(r.positionIndex.documents, docIndex.uri)
}

// shouldRebuildSpatialTree determines if the spatial tree should be rebuilt
func (r *SymbolResolver) shouldRebuildSpatialTree() bool {
	// Rebuild if tree is getting unbalanced or too large
	return r.positionIndex.spatialTree.size > IntervalTreeRebalanceThreshold
}

// Utility functions

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func maxInt32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}
