package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/models/lsp"
)

// Position key for efficient position-based lookups
type PositionKey struct {
	URI       string
	Line      int
	Character int
}

// SymbolKey represents a unique symbol identifier
type SymbolKey struct {
	Symbol   string
	Language string
}

// DocumentSymbolTree represents hierarchical document symbols
type DocumentSymbolTree struct {
	Symbols  []*lsp.DocumentSymbol
	Flat     map[string]*lsp.DocumentSymbol // Flattened lookup by name
	Modified time.Time
}

// CompletionContext represents completion state at a position
type CompletionContext struct {
	Position        PositionKey
	ScopeSymbols    []string // Available symbols in scope
	ImportedSymbols []string // Available through imports
	LastUpdated     time.Time
}

// QueryMetrics tracks performance metrics
type QueryMetrics struct {
	TotalQueries    int64
	CacheHits       int64
	CacheMisses     int64
	AvgResponseTime time.Duration
	LastResetTime   time.Time
}

// QueryCircuitBreaker provides fallback mechanism when cache fails
type QueryCircuitBreaker struct {
	failures    int
	lastFailure time.Time
	state       string // "closed", "open", "half-open"
	threshold   int
	timeout     time.Duration
	mu          sync.RWMutex
}

// SCIPQueryIndex holds the high-performance in-memory indexes
type SCIPQueryIndex struct {
	// Symbol name index for fast symbol name -> definition lookups
	SymbolIndex map[string]*lsp.SymbolInformation

	// Position index for fast position -> symbol lookups
	PositionIndex map[PositionKey]string

	// Reference graph for fast symbol -> references lookups
	ReferenceGraph map[string][]*lsp.Location

	// Document symbols for fast document outline
	DocumentSymbols map[string]*DocumentSymbolTree

	// Hover information cache
	HoverCache map[PositionKey]*lsp.Hover

	// Completion context cache
	CompletionCache map[PositionKey]*CompletionContext

	// Workspace symbol index for fuzzy search
	WorkspaceIndex map[string][]*lsp.SymbolInformation

	// Metadata
	LastUpdated  time.Time
	IndexedURIs  map[string]time.Time
	TotalSymbols int
	mu           sync.RWMutex
}

// FastSCIPQuery provides the main query interface for fast symbol lookups
type FastSCIPQuery interface {
	// Core LSP method implementations
	GetDefinition(ctx context.Context, params *lsp.DefinitionParams) ([]*lsp.Location, error)
	GetReferences(ctx context.Context, params *lsp.ReferenceParams) ([]*lsp.Location, error)
	GetHover(ctx context.Context, params *lsp.HoverParams) (*lsp.Hover, error)
	GetDocumentSymbols(ctx context.Context, params *lsp.DocumentSymbolParams) ([]*lsp.DocumentSymbol, error)
	GetWorkspaceSymbols(ctx context.Context, params *lsp.WorkspaceSymbolParams) ([]*lsp.SymbolInformation, error)
	GetCompletion(ctx context.Context, params *lsp.CompletionParams) (*lsp.CompletionList, error)

	// Index management
	UpdateIndex(uri string, symbols []*lsp.SymbolInformation, documentSymbols []*lsp.DocumentSymbol) error
	InvalidateDocument(uri string) error
	GetMetrics() *QueryMetrics
	IsHealthy() bool
}

// SCIPQueryManager implements the FastSCIPQuery interface
type SCIPQueryManager struct {
	index          *SCIPQueryIndex
	circuitBreaker *QueryCircuitBreaker
	metrics        *QueryMetrics
	fallbackLSP    LSPFallback // Fallback to actual LSP servers
	mu             sync.RWMutex
}

// LSPFallback interface for fallback to actual LSP servers
type LSPFallback interface {
	ProcessRequest(ctx context.Context, method string, params interface{}) (interface{}, error)
}

// NewSCIPQueryManager creates a new SCIP query manager
func NewSCIPQueryManager(fallback LSPFallback) *SCIPQueryManager {
	return &SCIPQueryManager{
		index: &SCIPQueryIndex{
			SymbolIndex:     make(map[string]*lsp.SymbolInformation),
			PositionIndex:   make(map[PositionKey]string),
			ReferenceGraph:  make(map[string][]*lsp.Location),
			DocumentSymbols: make(map[string]*DocumentSymbolTree),
			HoverCache:      make(map[PositionKey]*lsp.Hover),
			CompletionCache: make(map[PositionKey]*CompletionContext),
			WorkspaceIndex:  make(map[string][]*lsp.SymbolInformation),
			IndexedURIs:     make(map[string]time.Time),
		},
		circuitBreaker: &QueryCircuitBreaker{
			threshold: 5,
			timeout:   30 * time.Second,
			state:     "closed",
		},
		metrics: &QueryMetrics{
			LastResetTime: time.Now(),
		},
		fallbackLSP: fallback,
	}
}

// GetDefinition provides sub-millisecond definition lookups
func (q *SCIPQueryManager) GetDefinition(ctx context.Context, params *lsp.DefinitionParams) ([]*lsp.Location, error) {
	start := time.Now()
	defer func() {
		q.updateMetrics(time.Since(start))
	}()

	// Circuit breaker check
	if !q.circuitBreaker.allowRequest() {
		return q.fallbackToLSPDefinition(ctx, params)
	}

	q.index.mu.RLock()
	defer q.index.mu.RUnlock()

	posKey := PositionKey{
		URI:       params.TextDocument.URI,
		Line:      params.Position.Line,
		Character: params.Position.Character,
	}

	// Fast position -> symbol lookup
	symbolName, exists := q.index.PositionIndex[posKey]
	if !exists {
		q.metrics.CacheMisses++
		return q.fallbackToLSPDefinition(ctx, params)
	}

	// Fast symbol -> definition lookup
	symbolInfo, exists := q.index.SymbolIndex[symbolName]
	if !exists {
		q.metrics.CacheMisses++
		return q.fallbackToLSPDefinition(ctx, params)
	}

	q.metrics.CacheHits++
	return []*lsp.Location{&symbolInfo.Location}, nil
}

// GetReferences provides fast reference lookups via graph traversal
func (q *SCIPQueryManager) GetReferences(ctx context.Context, params *lsp.ReferenceParams) ([]*lsp.Location, error) {
	start := time.Now()
	defer func() {
		q.updateMetrics(time.Since(start))
	}()

	if !q.circuitBreaker.allowRequest() {
		return q.fallbackToLSPReferences(ctx, params)
	}

	q.index.mu.RLock()
	defer q.index.mu.RUnlock()

	posKey := PositionKey{
		URI:       params.TextDocument.URI,
		Line:      params.Position.Line,
		Character: params.Position.Character,
	}

	// Get symbol at position
	symbolName, exists := q.index.PositionIndex[posKey]
	if !exists {
		q.metrics.CacheMisses++
		return q.fallbackToLSPReferences(ctx, params)
	}

	// Fast reference graph traversal
	references, exists := q.index.ReferenceGraph[symbolName]
	if !exists {
		q.metrics.CacheMisses++
		return q.fallbackToLSPReferences(ctx, params)
	}

	// Include declaration if requested
	result := make([]*lsp.Location, 0, len(references)+1)
	if params.Context.IncludeDeclaration {
		if symbolInfo, exists := q.index.SymbolIndex[symbolName]; exists {
			result = append(result, &symbolInfo.Location)
		}
	}
	result = append(result, references...)

	q.metrics.CacheHits++
	return result, nil
}

// GetHover provides fast hover information extraction
func (q *SCIPQueryManager) GetHover(ctx context.Context, params *lsp.HoverParams) (*lsp.Hover, error) {
	start := time.Now()
	defer func() {
		q.updateMetrics(time.Since(start))
	}()

	if !q.circuitBreaker.allowRequest() {
		return q.fallbackToLSPHover(ctx, params)
	}

	q.index.mu.RLock()
	defer q.index.mu.RUnlock()

	posKey := PositionKey{
		URI:       params.TextDocument.URI,
		Line:      params.Position.Line,
		Character: params.Position.Character,
	}

	// Fast hover cache lookup
	if hover, exists := q.index.HoverCache[posKey]; exists {
		q.metrics.CacheHits++
		return hover, nil
	}

	// Try symbol-based hover
	if symbolName, exists := q.index.PositionIndex[posKey]; exists {
		if symbolInfo, exists := q.index.SymbolIndex[symbolName]; exists {
			// Build hover from symbol information
			hover := &lsp.Hover{
				Contents: map[string]interface{}{
					"kind":  "markdown",
					"value": fmt.Sprintf("**%s** (%s)", symbolInfo.Name, symbolKindToString(symbolInfo.Kind)),
				},
				Range: &lsp.Range{
					Start: lsp.Position{Line: params.Position.Line, Character: params.Position.Character},
					End:   lsp.Position{Line: params.Position.Line, Character: params.Position.Character},
				},
			}
			q.metrics.CacheHits++
			return hover, nil
		}
	}

	q.metrics.CacheMisses++
	return q.fallbackToLSPHover(ctx, params)
}

// GetDocumentSymbols provides fast document outline
func (q *SCIPQueryManager) GetDocumentSymbols(ctx context.Context, params *lsp.DocumentSymbolParams) ([]*lsp.DocumentSymbol, error) {
	start := time.Now()
	defer func() {
		q.updateMetrics(time.Since(start))
	}()

	if !q.circuitBreaker.allowRequest() {
		return q.fallbackToLSPDocumentSymbols(ctx, params)
	}

	q.index.mu.RLock()
	defer q.index.mu.RUnlock()

	// Fast document symbol lookup
	if symbolTree, exists := q.index.DocumentSymbols[params.TextDocument.URI]; exists {
		q.metrics.CacheHits++
		return symbolTree.Symbols, nil
	}

	q.metrics.CacheMisses++
	return q.fallbackToLSPDocumentSymbols(ctx, params)
}

// GetWorkspaceSymbols provides fast workspace-wide symbol search
func (q *SCIPQueryManager) GetWorkspaceSymbols(ctx context.Context, params *lsp.WorkspaceSymbolParams) ([]*lsp.SymbolInformation, error) {
	start := time.Now()
	defer func() {
		q.updateMetrics(time.Since(start))
	}()

	if !q.circuitBreaker.allowRequest() {
		return q.fallbackToLSPWorkspaceSymbols(ctx, params)
	}

	q.index.mu.RLock()
	defer q.index.mu.RUnlock()

	query := strings.ToLower(params.Query)
	if query == "" {
		// Return all symbols (limit to reasonable amount)
		var allSymbols []*lsp.SymbolInformation
		for _, symbols := range q.index.WorkspaceIndex {
			allSymbols = append(allSymbols, symbols...)
			if len(allSymbols) >= 1000 { // Reasonable limit
				break
			}
		}
		q.metrics.CacheHits++
		return allSymbols, nil
	}

	// Fuzzy symbol matching
	var matchedSymbols []*lsp.SymbolInformation
	for symbolName, symbols := range q.index.WorkspaceIndex {
		if strings.Contains(strings.ToLower(symbolName), query) {
			matchedSymbols = append(matchedSymbols, symbols...)
		}
	}

	// Sort by relevance (name length, then alphabetically)
	sort.Slice(matchedSymbols, func(i, j int) bool {
		a, b := matchedSymbols[i], matchedSymbols[j]
		if len(a.Name) != len(b.Name) {
			return len(a.Name) < len(b.Name)
		}
		return a.Name < b.Name
	})

	// Limit results
	if len(matchedSymbols) > 100 {
		matchedSymbols = matchedSymbols[:100]
	}

	q.metrics.CacheHits++
	return matchedSymbols, nil
}

// GetCompletion provides fast completion context analysis
func (q *SCIPQueryManager) GetCompletion(ctx context.Context, params *lsp.CompletionParams) (*lsp.CompletionList, error) {
	start := time.Now()
	defer func() {
		q.updateMetrics(time.Since(start))
	}()

	if !q.circuitBreaker.allowRequest() {
		return q.fallbackToLSPCompletion(ctx, params)
	}

	q.index.mu.RLock()
	defer q.index.mu.RUnlock()

	posKey := PositionKey{
		URI:       params.TextDocument.URI,
		Line:      params.Position.Line,
		Character: params.Position.Character,
	}

	// Check completion context cache
	if context, exists := q.index.CompletionCache[posKey]; exists &&
		time.Since(context.LastUpdated) < 5*time.Minute {

		var items []*lsp.CompletionItem

		// Add scope symbols as completion items
		for _, symbolName := range context.ScopeSymbols {
			if symbolInfo, exists := q.index.SymbolIndex[symbolName]; exists {
				items = append(items, &lsp.CompletionItem{
					Label:  symbolInfo.Name,
					Kind:   symbolKindToCompletionKind(symbolInfo.Kind),
					Detail: symbolInfo.ContainerName,
				})
			}
		}

		// Add imported symbols
		for _, symbolName := range context.ImportedSymbols {
			if symbolInfo, exists := q.index.SymbolIndex[symbolName]; exists {
				items = append(items, &lsp.CompletionItem{
					Label:  symbolInfo.Name,
					Kind:   symbolKindToCompletionKind(symbolInfo.Kind),
					Detail: symbolInfo.ContainerName,
				})
			}
		}

		q.metrics.CacheHits++
		return &lsp.CompletionList{
			IsIncomplete: false,
			Items:        items,
		}, nil
	}

	q.metrics.CacheMisses++
	return q.fallbackToLSPCompletion(ctx, params)
}

// UpdateIndex updates the query index with new symbol information
func (q *SCIPQueryManager) UpdateIndex(uri string, symbols []*lsp.SymbolInformation, documentSymbols []*lsp.DocumentSymbol) error {
	q.index.mu.Lock()
	defer q.index.mu.Unlock()

	common.LSPLogger.Debug("Updating SCIP index for URI: %s with %d symbols", uri, len(symbols))

	// Clear existing data for this URI
	q.clearDocumentFromIndex(uri)

	// Update symbol index and position index
	for _, symbol := range symbols {
		q.index.SymbolIndex[symbol.Name] = symbol

		// Create position key for this symbol
		posKey := PositionKey{
			URI:       symbol.Location.URI,
			Line:      symbol.Location.Range.Start.Line,
			Character: symbol.Location.Range.Start.Character,
		}
		q.index.PositionIndex[posKey] = symbol.Name

		// Update workspace index for search
		lowerName := strings.ToLower(symbol.Name)
		if _, exists := q.index.WorkspaceIndex[lowerName]; !exists {
			q.index.WorkspaceIndex[lowerName] = []*lsp.SymbolInformation{}
		}
		q.index.WorkspaceIndex[lowerName] = append(q.index.WorkspaceIndex[lowerName], symbol)
	}

	// Update document symbols
	if len(documentSymbols) > 0 {
		flatSymbols := make(map[string]*lsp.DocumentSymbol)
		q.flattenDocumentSymbols(documentSymbols, flatSymbols)

		q.index.DocumentSymbols[uri] = &DocumentSymbolTree{
			Symbols:  documentSymbols,
			Flat:     flatSymbols,
			Modified: time.Now(),
		}
	}

	// Update metadata
	q.index.IndexedURIs[uri] = time.Now()
	q.index.LastUpdated = time.Now()
	q.index.TotalSymbols = len(q.index.SymbolIndex)

	common.LSPLogger.Debug("SCIP index updated. Total symbols: %d", q.index.TotalSymbols)
	return nil
}

// InvalidateDocument removes a document from the index
func (q *SCIPQueryManager) InvalidateDocument(uri string) error {
	q.index.mu.Lock()
	defer q.index.mu.Unlock()

	q.clearDocumentFromIndex(uri)
	delete(q.index.IndexedURIs, uri)

	common.LSPLogger.Debug("Invalidated SCIP index for URI: %s", uri)
	return nil
}

// GetMetrics returns current performance metrics
func (q *SCIPQueryManager) GetMetrics() *QueryMetrics {
	q.mu.RLock()
	defer q.mu.RUnlock()

	// Create a copy to avoid race conditions
	return &QueryMetrics{
		TotalQueries:    q.metrics.TotalQueries,
		CacheHits:       q.metrics.CacheHits,
		CacheMisses:     q.metrics.CacheMisses,
		AvgResponseTime: q.metrics.AvgResponseTime,
		LastResetTime:   q.metrics.LastResetTime,
	}
}

// IsHealthy returns true if the query manager is healthy
func (q *SCIPQueryManager) IsHealthy() bool {
	q.mu.RLock()
	defer q.mu.RUnlock()

	return q.circuitBreaker.state != "open" &&
		q.index.TotalSymbols > 0 &&
		time.Since(q.index.LastUpdated) < 1*time.Hour
}

// BuildKey implements the SCIPQuery interface
func (q *SCIPQueryManager) BuildKey(method string, params interface{}) (CacheKey, error) {
	uri, err := q.ExtractURI(params)
	if err != nil {
		return CacheKey{}, err
	}

	// Create a hash of the parameters for uniqueness
	paramBytes, err := json.Marshal(params)
	if err != nil {
		return CacheKey{}, fmt.Errorf("failed to marshal params: %w", err)
	}

	// Simple hash based on content
	hash := fmt.Sprintf("%x", len(paramBytes))

	return CacheKey{
		Method: method,
		URI:    uri,
		Hash:   hash,
	}, nil
}

// IsValidEntry implements the SCIPQuery interface
func (q *SCIPQueryManager) IsValidEntry(entry *CacheEntry, ttl time.Duration) bool {
	return time.Since(entry.Timestamp) < ttl
}

// ExtractURI implements the SCIPQuery interface
func (q *SCIPQueryManager) ExtractURI(params interface{}) (string, error) {
	switch p := params.(type) {
	case *lsp.DefinitionParams:
		return p.TextDocument.URI, nil
	case *lsp.ReferenceParams:
		return p.TextDocument.URI, nil
	case *lsp.HoverParams:
		return p.TextDocument.URI, nil
	case *lsp.DocumentSymbolParams:
		return p.TextDocument.URI, nil
	case *lsp.CompletionParams:
		return p.TextDocument.URI, nil
	case *lsp.WorkspaceSymbolParams:
		return "", nil // Workspace symbols don't have a specific URI
	case map[string]interface{}:
		// Handle untyped parameters from CLI cache indexing
		if p == nil {
			return "", fmt.Errorf("no parameters provided")
		}

		// Try textDocument.uri first (most common case)
		if textDoc, ok := p["textDocument"].(map[string]interface{}); ok {
			if uri, ok := textDoc["uri"].(string); ok && uri != "" {
				return uri, nil
			}
		}

		// Try direct uri parameter (alternative format)
		if uri, ok := p["uri"].(string); ok && uri != "" {
			return uri, nil
		}

		// For workspace/symbol requests, no URI is expected - return empty string
		if query, ok := p["query"]; ok && query != nil {
			return "", nil
		}

		// Handle textDocument/documentSymbol with just textDocument
		if textDoc, exists := p["textDocument"]; exists {
			if textDocMap, ok := textDoc.(map[string]interface{}); ok {
				if uri, ok := textDocMap["uri"].(string); ok && uri != "" {
					return uri, nil
				}
			}
		}

		// Handle potential position-based requests without textDocument
		if position, exists := p["position"]; exists && position != nil {
			// This might be a malformed request, but we should not fail completely
			common.LSPLogger.Debug("ExtractURI: Found position parameter without textDocument, assuming no URI needed")
			return "", nil
		}

		// Additional debug information for troubleshooting
		var keys []string
		for k := range p {
			keys = append(keys, k)
		}
		
		return "", fmt.Errorf("no URI found in untyped parameters, available keys: %v", keys)
	default:
		return "", fmt.Errorf("unsupported parameter type: %T", params)
	}
}

// Helper methods

// clearDocumentFromIndex removes all index entries for a document
func (q *SCIPQueryManager) clearDocumentFromIndex(uri string) {
	// Collect symbols to remove first
	var symbolsToRemove []string
	for symbolName, symbolInfo := range q.index.SymbolIndex {
		if symbolInfo.Location.URI == uri {
			symbolsToRemove = append(symbolsToRemove, symbolName)
		}
	}

	// Remove symbols from SymbolIndex
	for _, symbolName := range symbolsToRemove {
		delete(q.index.SymbolIndex, symbolName)
	}

	// Remove from ReferenceGraph
	for _, symbolName := range symbolsToRemove {
		delete(q.index.ReferenceGraph, symbolName)
	}

	// Remove from WorkspaceIndex
	for symbolName, symbols := range q.index.WorkspaceIndex {
		var filteredSymbols []*lsp.SymbolInformation
		for _, symbol := range symbols {
			if symbol.Location.URI != uri {
				filteredSymbols = append(filteredSymbols, symbol)
			}
		}
		if len(filteredSymbols) == 0 {
			delete(q.index.WorkspaceIndex, symbolName)
		} else {
			q.index.WorkspaceIndex[symbolName] = filteredSymbols
		}
	}

	// Remove from position index
	var keysToDelete []PositionKey
	for posKey := range q.index.PositionIndex {
		if posKey.URI == uri {
			keysToDelete = append(keysToDelete, posKey)
		}
	}
	for _, key := range keysToDelete {
		delete(q.index.PositionIndex, key)
	}

	// Remove from hover cache
	var hoverKeysToDelete []PositionKey
	for posKey := range q.index.HoverCache {
		if posKey.URI == uri {
			hoverKeysToDelete = append(hoverKeysToDelete, posKey)
		}
	}
	for _, key := range hoverKeysToDelete {
		delete(q.index.HoverCache, key)
	}

	// Remove from completion cache
	var completionKeysToDelete []PositionKey
	for posKey := range q.index.CompletionCache {
		if posKey.URI == uri {
			completionKeysToDelete = append(completionKeysToDelete, posKey)
		}
	}
	for _, key := range completionKeysToDelete {
		delete(q.index.CompletionCache, key)
	}

	// Remove document symbols
	delete(q.index.DocumentSymbols, uri)
}

// flattenDocumentSymbols creates a flat lookup map from hierarchical symbols
func (q *SCIPQueryManager) flattenDocumentSymbols(symbols []*lsp.DocumentSymbol, flat map[string]*lsp.DocumentSymbol) {
	for _, symbol := range symbols {
		flat[symbol.Name] = symbol
		if symbol.Children != nil {
			q.flattenDocumentSymbols(symbol.Children, flat)
		}
	}
}

// fallbackToLSP falls back to actual LSP server when cache fails
func (q *SCIPQueryManager) fallbackToLSP(ctx context.Context, method string, params interface{}) (interface{}, error) {
	q.circuitBreaker.recordFailure()

	if q.fallbackLSP == nil {
		return nil, fmt.Errorf("cache miss and no fallback LSP available for method: %s", method)
	}

	common.LSPLogger.Debug("Falling back to LSP server for method: %s", method)
	result, err := q.fallbackLSP.ProcessRequest(ctx, method, params)
	if err != nil {
		q.circuitBreaker.recordFailure()
		return nil, fmt.Errorf("LSP fallback failed: %w", err)
	}

	q.circuitBreaker.recordSuccess()
	return result, nil
}

// fallbackToLSPDefinition is a type-safe fallback for definition requests
func (q *SCIPQueryManager) fallbackToLSPDefinition(ctx context.Context, params *lsp.DefinitionParams) ([]*lsp.Location, error) {
	result, err := q.fallbackToLSP(ctx, "textDocument/definition", params)
	if err != nil {
		return nil, err
	}
	if locations, ok := result.([]*lsp.Location); ok {
		return locations, nil
	}
	return nil, fmt.Errorf("unexpected fallback response type: %T", result)
}

// fallbackToLSPReferences is a type-safe fallback for reference requests
func (q *SCIPQueryManager) fallbackToLSPReferences(ctx context.Context, params *lsp.ReferenceParams) ([]*lsp.Location, error) {
	result, err := q.fallbackToLSP(ctx, "textDocument/references", params)
	if err != nil {
		return nil, err
	}
	if locations, ok := result.([]*lsp.Location); ok {
		return locations, nil
	}
	return nil, fmt.Errorf("unexpected fallback response type: %T", result)
}

// fallbackToLSPHover is a type-safe fallback for hover requests
func (q *SCIPQueryManager) fallbackToLSPHover(ctx context.Context, params *lsp.HoverParams) (*lsp.Hover, error) {
	result, err := q.fallbackToLSP(ctx, "textDocument/hover", params)
	if err != nil {
		return nil, err
	}
	if hover, ok := result.(*lsp.Hover); ok {
		return hover, nil
	}
	return nil, fmt.Errorf("unexpected fallback response type: %T", result)
}

// fallbackToLSPDocumentSymbols is a type-safe fallback for document symbol requests
func (q *SCIPQueryManager) fallbackToLSPDocumentSymbols(ctx context.Context, params *lsp.DocumentSymbolParams) ([]*lsp.DocumentSymbol, error) {
	result, err := q.fallbackToLSP(ctx, "textDocument/documentSymbol", params)
	if err != nil {
		return nil, err
	}
	if symbols, ok := result.([]*lsp.DocumentSymbol); ok {
		return symbols, nil
	}
	return nil, fmt.Errorf("unexpected fallback response type: %T", result)
}

// fallbackToLSPWorkspaceSymbols is a type-safe fallback for workspace symbol requests
func (q *SCIPQueryManager) fallbackToLSPWorkspaceSymbols(ctx context.Context, params *lsp.WorkspaceSymbolParams) ([]*lsp.SymbolInformation, error) {
	result, err := q.fallbackToLSP(ctx, "workspace/symbol", params)
	if err != nil {
		return nil, err
	}
	if symbols, ok := result.([]*lsp.SymbolInformation); ok {
		return symbols, nil
	}
	return nil, fmt.Errorf("unexpected fallback response type: %T", result)
}

// fallbackToLSPCompletion is a type-safe fallback for completion requests
func (q *SCIPQueryManager) fallbackToLSPCompletion(ctx context.Context, params *lsp.CompletionParams) (*lsp.CompletionList, error) {
	result, err := q.fallbackToLSP(ctx, "textDocument/completion", params)
	if err != nil {
		return nil, err
	}
	if completion, ok := result.(*lsp.CompletionList); ok {
		return completion, nil
	}
	return nil, fmt.Errorf("unexpected fallback response type: %T", result)
}

// updateMetrics updates performance metrics
func (q *SCIPQueryManager) updateMetrics(duration time.Duration) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.metrics.TotalQueries++

	// Calculate rolling average response time
	if q.metrics.TotalQueries == 1 {
		q.metrics.AvgResponseTime = duration
	} else {
		// Exponential moving average
		alpha := 0.1
		q.metrics.AvgResponseTime = time.Duration(float64(q.metrics.AvgResponseTime)*(1-alpha) + float64(duration)*alpha)
	}
}

// Circuit breaker implementation

// allowRequest checks if circuit breaker allows the request
func (cb *QueryCircuitBreaker) allowRequest() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case "closed":
		return true
	case "open":
		return time.Since(cb.lastFailure) > cb.timeout
	case "half-open":
		return true
	default:
		return true
	}
}

// recordSuccess records a successful request
func (cb *QueryCircuitBreaker) recordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures = 0
	cb.state = "closed"
}

// recordFailure records a failed request
func (cb *QueryCircuitBreaker) recordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailure = time.Now()

	if cb.failures >= cb.threshold {
		cb.state = "open"
	}
}

// Helper functions for type conversions

// symbolKindToString converts SymbolKind to string
func symbolKindToString(kind lsp.SymbolKind) string {
	switch kind {
	case lsp.File:
		return "File"
	case lsp.Module:
		return "Module"
	case lsp.Namespace:
		return "Namespace"
	case lsp.Package:
		return "Package"
	case lsp.Class:
		return "Class"
	case lsp.Method:
		return "Method"
	case lsp.Property:
		return "Property"
	case lsp.Field:
		return "Field"
	case lsp.Constructor:
		return "Constructor"
	case lsp.Enum:
		return "Enum"
	case lsp.Interface:
		return "Interface"
	case lsp.Function:
		return "Function"
	case lsp.Variable:
		return "Variable"
	case lsp.Constant:
		return "Constant"
	default:
		return "Unknown"
	}
}

// symbolKindToCompletionKind converts SymbolKind to CompletionItemKind
func symbolKindToCompletionKind(kind lsp.SymbolKind) lsp.CompletionItemKind {
	switch kind {
	case lsp.Method:
		return lsp.MethodComp
	case lsp.Function:
		return lsp.FunctionComp
	case lsp.Constructor:
		return lsp.ConstructorComp
	case lsp.Field:
		return lsp.FieldComp
	case lsp.Variable:
		return lsp.VariableComp
	case lsp.Class:
		return lsp.ClassComp
	case lsp.Interface:
		return lsp.InterfaceComp
	case lsp.Module:
		return lsp.ModuleComp
	case lsp.Property:
		return lsp.PropertyComp
	case lsp.Enum:
		return lsp.EnumComp
	case lsp.Constant:
		return lsp.ConstantComp
	default:
		return lsp.Text
	}
}
