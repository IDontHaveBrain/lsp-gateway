package cache

import (
	"context"
	"fmt"
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

// SCIPQueryManager - DEPRECATED: Complex query wrapper removed, use direct cache operations
type SCIPQueryManager struct {
	fallbackLSP LSPFallback // Direct fallback to LSP servers
	metrics     *QueryMetrics
	mu          sync.RWMutex
}

// LSPFallback interface for fallback to actual LSP servers
type LSPFallback interface {
	ProcessRequest(ctx context.Context, method string, params interface{}) (interface{}, error)
}

// NewSCIPQueryManager - DEPRECATED: Creates simplified query manager (complex indexing removed)
func NewSCIPQueryManager(fallback LSPFallback) *SCIPQueryManager {
	common.LSPLogger.Warn("SCIPQueryManager is deprecated - use direct LSPManager cache integration")
	return &SCIPQueryManager{
		fallbackLSP: fallback,
		metrics: &QueryMetrics{
			LastResetTime: time.Now(),
		},
	}
}

// GetDefinition - DEPRECATED: Direct LSP fallback (complex indexing removed)
func (q *SCIPQueryManager) GetDefinition(ctx context.Context, params *lsp.DefinitionParams) ([]*lsp.Location, error) {
	start := time.Now()
	defer func() {
		q.updateMetrics(time.Since(start))
	}()

	// Simplified implementation - direct LSP fallback
	q.metrics.CacheMisses++
	return q.fallbackToLSPDefinition(ctx, params)
}

// GetReferences - DEPRECATED: Direct LSP fallback (complex graph traversal removed)
func (q *SCIPQueryManager) GetReferences(ctx context.Context, params *lsp.ReferenceParams) ([]*lsp.Location, error) {
	start := time.Now()
	defer func() {
		q.updateMetrics(time.Since(start))
	}()

	// Simplified implementation - direct LSP fallback
	q.metrics.CacheMisses++
	return q.fallbackToLSPReferences(ctx, params)
}

// GetHover - DEPRECATED: Direct LSP fallback (complex hover caching removed)
func (q *SCIPQueryManager) GetHover(ctx context.Context, params *lsp.HoverParams) (*lsp.Hover, error) {
	start := time.Now()
	defer func() {
		q.updateMetrics(time.Since(start))
	}()

	// Simplified implementation - direct LSP fallback
	q.metrics.CacheMisses++
	return q.fallbackToLSPHover(ctx, params)
}

// GetDocumentSymbols - DEPRECATED: Direct LSP fallback (complex symbol tree removed)
func (q *SCIPQueryManager) GetDocumentSymbols(ctx context.Context, params *lsp.DocumentSymbolParams) ([]*lsp.DocumentSymbol, error) {
	start := time.Now()
	defer func() {
		q.updateMetrics(time.Since(start))
	}()

	// Simplified implementation - direct LSP fallback
	q.metrics.CacheMisses++
	return q.fallbackToLSPDocumentSymbols(ctx, params)
}

// GetWorkspaceSymbols - DEPRECATED: Direct LSP fallback (complex fuzzy matching removed)
func (q *SCIPQueryManager) GetWorkspaceSymbols(ctx context.Context, params *lsp.WorkspaceSymbolParams) ([]*lsp.SymbolInformation, error) {
	start := time.Now()
	defer func() {
		q.updateMetrics(time.Since(start))
	}()

	// Simplified implementation - direct LSP fallback
	q.metrics.CacheMisses++
	return q.fallbackToLSPWorkspaceSymbols(ctx, params)
}

// GetCompletion - DEPRECATED: Direct LSP fallback (complex context analysis removed)
func (q *SCIPQueryManager) GetCompletion(ctx context.Context, params *lsp.CompletionParams) (*lsp.CompletionList, error) {
	start := time.Now()
	defer func() {
		q.updateMetrics(time.Since(start))
	}()

	// Simplified implementation - direct LSP fallback
	q.metrics.CacheMisses++
	return q.fallbackToLSPCompletion(ctx, params)
}

// UpdateIndex - DEPRECATED: No-op method (complex indexing removed)
func (q *SCIPQueryManager) UpdateIndex(uri string, symbols []*lsp.SymbolInformation, documentSymbols []*lsp.DocumentSymbol) error {
	common.LSPLogger.Debug("UpdateIndex called for URI: %s (no-op in simplified implementation)", uri)
	return nil
}

// InvalidateDocument - DEPRECATED: No-op method (complex indexing removed)
func (q *SCIPQueryManager) InvalidateDocument(uri string) error {
	common.LSPLogger.Debug("InvalidateDocument called for URI: %s (no-op in simplified implementation)", uri)
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

// IsHealthy - DEPRECATED: Always returns true (complex health checks removed)
func (q *SCIPQueryManager) IsHealthy() bool {
	return true // Simplified implementation - always healthy
}

// BuildKey - DEPRECATED: Simplified implementation (complex key building removed)
func (q *SCIPQueryManager) BuildKey(method string, params interface{}) (CacheKey, error) {
	uri, err := q.ExtractURI(params)
	if err != nil {
		return CacheKey{}, err
	}

	// Simplified hash based on method and URI only
	hash := fmt.Sprintf("%s-%s", method, uri)

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

// clearDocumentFromIndex - DEPRECATED: No-op method (complex indexing removed)
func (q *SCIPQueryManager) clearDocumentFromIndex(uri string) {
	// Simplified implementation - no indexing to clear
	common.LSPLogger.Debug("clearDocumentFromIndex called for URI: %s (no-op in simplified implementation)", uri)
}

// flattenDocumentSymbols - DEPRECATED: No-op method (complex symbol flattening removed)
func (q *SCIPQueryManager) flattenDocumentSymbols(symbols []*lsp.DocumentSymbol, flat map[string]*lsp.DocumentSymbol) {
	// Simplified implementation - no flattening needed
}

// fallbackToLSP falls back to actual LSP server when cache fails
func (q *SCIPQueryManager) fallbackToLSP(ctx context.Context, method string, params interface{}) (interface{}, error) {

	if q.fallbackLSP == nil {
		return nil, fmt.Errorf("cache miss and no fallback LSP available for method: %s", method)
	}

	common.LSPLogger.Debug("Falling back to LSP server for method: %s", method)
	result, err := q.fallbackLSP.ProcessRequest(ctx, method, params)
	if err != nil {
		return nil, fmt.Errorf("LSP fallback failed: %w", err)
	}

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
