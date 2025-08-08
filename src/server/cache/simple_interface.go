package cache

import (
	"context"
	"lsp-gateway/src/internal/models/lsp"
	"lsp-gateway/src/internal/types"
)

// LSPFallback interface for fallback to actual LSP servers
type LSPFallback interface {
	ProcessRequest(ctx context.Context, method string, params interface{}) (interface{}, error)
}

// SimpleCache provides a direct, streamlined cache interface with integrated SCIP indexing
type SimpleCache interface {
	// Core cache operations - direct and simple
	Lookup(method string, params interface{}) (interface{}, bool, error)
	Store(method string, params interface{}, response interface{}) error
	InvalidateDocument(uri string) error
	Clear() error // Clear all cache entries

	// Lifecycle management
	Start(ctx context.Context) error
	Stop() error

	// Status and metrics
	GetMetrics() *CacheMetrics
	HealthCheck() (*CacheMetrics, error)
	IsEnabled() bool

	// SCIP indexing capabilities - integrated as core functionality
	IndexDocument(ctx context.Context, uri string, language string, symbols []lsp.SymbolInformation) error
	QueryIndex(ctx context.Context, query *IndexQuery) (*IndexResult, error)
	GetIndexStats() *IndexStats
	UpdateIndex(ctx context.Context, files []string) error

	// Cached retrieval methods
	GetCachedDefinition(symbolID string) ([]types.Location, bool)
	GetCachedReferences(symbolID string) ([]types.Location, bool)
	GetCachedHover(symbolID string) (*lsp.Hover, bool)
	GetCachedDocumentSymbols(uri string) ([]types.SymbolInformation, bool)
	GetCachedWorkspaceSymbols(query string) ([]types.SymbolInformation, bool)
	StoreMethodResult(method string, params interface{}, response interface{}) error
}
