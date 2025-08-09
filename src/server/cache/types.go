package cache

import (
	"context"
	"time"

	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/server/scip"
)

// IndexQuery represents a query to the SCIP index
type IndexQuery struct {
	Type     string                 `json:"type"` // "symbol", "definition", "references", etc.
	Symbol   string                 `json:"symbol,omitempty"`
	URI      string                 `json:"uri,omitempty"`
	Position *Position              `json:"position,omitempty"`
	Language string                 `json:"language,omitempty"`
	Filters  map[string]interface{} `json:"filters,omitempty"`
	MaxDepth int                    `json:"max_depth,omitempty"`
}

// Position represents a position in a document
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// IndexResult represents the result of an index query
type IndexResult struct {
	Type      string                 `json:"type"`
	Results   []interface{}          `json:"results"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// IndexStats represents statistics about the SCIP index
type IndexStats struct {
	DocumentCount    int64            `json:"document_count"`
	SymbolCount      int64            `json:"symbol_count"`
	ReferenceCount   int64            `json:"reference_count"`
	IndexSize        int64            `json:"index_size_bytes"`
	LastUpdate       time.Time        `json:"last_update"`
	LanguageStats    map[string]int64 `json:"language_stats"`
	IndexedLanguages []string         `json:"indexed_languages"`
	Status           string           `json:"status"`
}

// SCIPSymbol wraps LSP SymbolInformation with enhanced SCIP metadata
type SCIPSymbol struct {
	SymbolInfo          types.SymbolInformation `json:"symbol_info"`
	Language            string                  `json:"language"`
	Score               float64                 `json:"score,omitempty"`
	FullRange           *Range                  `json:"full_range,omitempty"`           // Full range from document symbols
	Documentation       string                  `json:"documentation,omitempty"`        // Documentation from hover
	Signature           string                  `json:"signature,omitempty"`            // Signature from hover
	RelatedSymbols      []string                `json:"related_symbols,omitempty"`      // Related symbol names
	DefinitionLocations []types.Location        `json:"definition_locations,omitempty"` // Definition locations for this symbol
	ReferenceLocations  []types.Location        `json:"reference_locations,omitempty"`  // Reference locations for this symbol
	UsageCount          int                     `json:"usage_count,omitempty"`          // Number of references to this symbol
	Metadata            map[string]interface{}  `json:"metadata,omitempty"`             // Additional SCIP metadata
}

// Range represents a range in a document
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// CacheKey represents a unique identifier for cached LSP responses
type CacheKey struct {
	Method string `json:"method"`
	URI    string `json:"uri"`
	Hash   string `json:"hash"` // Hash of parameters for uniqueness
}

// CacheEntry represents a cached LSP response with metadata
type CacheEntry struct {
	Key        CacheKey    `json:"key"`
	Response   interface{} `json:"response"`
	Timestamp  time.Time   `json:"timestamp"`
	AccessedAt time.Time   `json:"accessed_at"`
	Size       int64       `json:"size"`
}

// SimpleCacheStats represents basic cache statistics
type SimpleCacheStats struct {
	HitCount   int64 `json:"hit_count"`
	MissCount  int64 `json:"miss_count"`
	ErrorCount int64 `json:"error_count"`
	TotalSize  int64 `json:"total_size"`
	EntryCount int64 `json:"entry_count"`
}

// CacheMetrics represents cache performance metrics
type CacheMetrics struct {
	HitCount        int64         `json:"hit_count"`
	MissCount       int64         `json:"miss_count"`
	ErrorCount      int64         `json:"error_count"`
	EvictionCount   int64         `json:"eviction_count"`
	TotalSize       int64         `json:"total_size"`
	EntryCount      int64         `json:"entry_count"`
	AverageHitTime  time.Duration `json:"average_hit_time"`
	AverageMissTime time.Duration `json:"average_miss_time"`
}

// SCIPCache interface for the main cache operations with integrated SCIP indexing
type SCIPCache interface {
	Start(ctx context.Context) error
	Stop() error
	Lookup(method string, params interface{}) (interface{}, bool, error)
	Store(method string, params interface{}, response interface{}) error
	InvalidateDocument(uri string) error
	HealthCheck() (*CacheMetrics, error)
	GetMetrics() *CacheMetrics
	Clear() error // Clear all cache entries

	// SCIP indexing capabilities - integrated as core functionality
	IndexDocument(ctx context.Context, uri string, language string, symbols []types.SymbolInformation) error
	QueryIndex(ctx context.Context, query *IndexQuery) (*IndexResult, error)
	GetIndexStats() *IndexStats
	UpdateIndex(ctx context.Context, files []string) error

	// Direct SCIP search methods for MCP tools
	SearchSymbols(ctx context.Context, pattern string, filePattern string, maxResults int) ([]interface{}, error)
	SearchReferences(ctx context.Context, symbolName string, filePattern string, maxResults int) ([]interface{}, error)
	SearchDefinitions(ctx context.Context, symbolName string, filePattern string, maxResults int) ([]interface{}, error)
	GetSymbolInfo(ctx context.Context, symbolName string, filePattern string) (interface{}, error)

	// Access to underlying SCIP storage
	GetSCIPStorage() scip.SCIPDocumentStorage
}