package cache

import (
    "lsp-gateway/src/server/cache/search"
)

type EnhancedSymbolQuery = search.EnhancedSymbolQuery

// EnhancedSymbolResult represents an enhanced symbol search result with occurrence metadata
type EnhancedSymbolResult = search.EnhancedSymbolResult

// ReferenceSearchOptions reuses unified search options.
type ReferenceSearchOptions = search.ReferenceSearchOptions

// Note: unified types are available in search package. No cache-level wrappers.

// SymbolInfoResult wrapper removed; use search.SymbolInfoResponse instead.

// SCIPOccurrenceInfo reuses unified occurrence info type.
type SCIPOccurrenceInfo = search.SCIPOccurrenceInfo
