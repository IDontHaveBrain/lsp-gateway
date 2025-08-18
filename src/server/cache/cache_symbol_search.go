package cache

import (
    "time"

    "lsp-gateway/src/server/cache/search"
    "lsp-gateway/src/server/scip"
)

type EnhancedSymbolQuery = search.EnhancedSymbolQuery

// EnhancedSymbolResult represents an enhanced symbol search result with occurrence metadata
type EnhancedSymbolResult = search.EnhancedSymbolResult

// EnhancedSymbolSearchResult wraps multiple enhanced symbol results with metadata
type EnhancedSymbolSearchResult struct {
	Symbols   []EnhancedSymbolResult `json:"symbols"`
	Total     int                    `json:"total"`
	Truncated bool                   `json:"truncated"`
	Query     *EnhancedSymbolQuery   `json:"query,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// ReferenceSearchOptions: reuse unified search options
type ReferenceSearchOptions = search.ReferenceSearchOptions

// ReferenceSearchResult represents reference search results with occurrence details
type ReferenceSearchResult struct {
	SymbolName string                  `json:"symbol_name"`
	SymbolID   string                  `json:"symbol_id,omitempty"`
	References []SCIPOccurrenceInfo    `json:"references"`
	Definition *SCIPOccurrenceInfo     `json:"definition,omitempty"`
	TotalCount int                     `json:"total_count"`
	FileCount  int                     `json:"file_count"`
	Options    *ReferenceSearchOptions `json:"options,omitempty"`
	Metadata   map[string]interface{}  `json:"metadata,omitempty"`
	Timestamp  time.Time               `json:"timestamp"`
}

// DefinitionSearchResult represents definition search results
type DefinitionSearchResult struct {
	SymbolName  string                 `json:"symbol_name"`
	SymbolID    string                 `json:"symbol_id,omitempty"`
	Definitions []SCIPOccurrenceInfo   `json:"definitions"`
	TotalCount  int                    `json:"total_count"`
	FileCount   int                    `json:"file_count"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
}

// SymbolInfoResult represents detailed symbol information result
type SymbolInfoResult struct {
	SymbolName      string                      `json:"symbol_name"`
	SymbolID        string                      `json:"symbol_id,omitempty"`
	SymbolInfo      *scip.SCIPSymbolInformation `json:"symbol_info,omitempty"`
	Kind            scip.SCIPSymbolKind         `json:"kind"`
	Documentation   []string                    `json:"documentation,omitempty"`
	Signature       string                      `json:"signature,omitempty"`
	Relationships   []scip.SCIPRelationship     `json:"relationships,omitempty"`
	Occurrences     []SCIPOccurrenceInfo        `json:"occurrences,omitempty"`
	OccurrenceCount int                         `json:"occurrence_count"`
	DefinitionCount int                         `json:"definition_count"`
	ReferenceCount  int                         `json:"reference_count"`
	FileCount       int                         `json:"file_count"`
	Metadata        map[string]interface{}      `json:"metadata,omitempty"`
	Timestamp       time.Time                   `json:"timestamp"`
}

// SCIPOccurrenceInfo: reuse unified occurrence info type
type SCIPOccurrenceInfo = search.SCIPOccurrenceInfo
