package cache

import (
	"time"

	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/server/scip"
)

// Enhanced occurrence-based query types
type OccurrenceQueryType string

const (
	QueryTypeSymbols             OccurrenceQueryType = "symbols"
	QueryTypeWriteAccesses       OccurrenceQueryType = "write_accesses"
	QueryTypeImportStatements    OccurrenceQueryType = "import_statements"
	QueryTypeForwardDeclarations OccurrenceQueryType = "forward_declarations"
	QueryTypeGeneratedCode       OccurrenceQueryType = "generated_code"
	QueryTypeTestCode            OccurrenceQueryType = "test_code"
	QueryTypeImplementations     OccurrenceQueryType = "implementations"
	QueryTypeRelatedSymbols      OccurrenceQueryType = "related_symbols"
)

// EnhancedSymbolQuery represents an enhanced symbol query with occurrence-based filtering
type EnhancedSymbolQuery struct {
	// Basic query parameters
	Pattern     string `json:"pattern"`
	FilePattern string `json:"file_pattern,omitempty"`
	Language    string `json:"language,omitempty"`
	MaxResults  int    `json:"max_results,omitempty"`

	// Occurrence-based filtering
	QueryType      OccurrenceQueryType   `json:"query_type,omitempty"`
	SymbolRoles    []types.SymbolRole    `json:"symbol_roles,omitempty"`
	SymbolKinds    []scip.SCIPSymbolKind `json:"symbol_kinds,omitempty"`
	ExcludeRoles   []types.SymbolRole    `json:"exclude_roles,omitempty"`
	MinOccurrences int                   `json:"min_occurrences,omitempty"`
	MaxOccurrences int                   `json:"max_occurrences,omitempty"`

	// Advanced filtering
	IncludeDocumentation bool   `json:"include_documentation"`
	IncludeRelationships bool   `json:"include_relationships"`
	OnlyWithDefinition   bool   `json:"only_with_definition"`
	SortBy               string `json:"sort_by,omitempty"` // "relevance", "name", "occurrences", "kind"
	IncludeScore         bool   `json:"include_score"`
}

// EnhancedSymbolResult represents an enhanced symbol search result with occurrence metadata
type EnhancedSymbolResult struct {
	// Symbol information
	SymbolInfo  *scip.SCIPSymbolInformation `json:"symbol_info"`
	SymbolID    string                      `json:"symbol_id"`
	DisplayName string                      `json:"display_name"`
	Kind        scip.SCIPSymbolKind         `json:"kind"`

	// Occurrence metadata
	Occurrences      []scip.SCIPOccurrence `json:"occurrences,omitempty"`
	OccurrenceCount  int                   `json:"occurrence_count"`
	DefinitionCount  int                   `json:"definition_count"`
	ReferenceCount   int                   `json:"reference_count"`
	WriteAccessCount int                   `json:"write_access_count"`
	ReadAccessCount  int                   `json:"read_access_count"`

	// Role aggregation
	AllRoles        types.SymbolRole `json:"all_roles"`
	HasDefinition   bool             `json:"has_definition"`
	HasReferences   bool             `json:"has_references"`
	InGeneratedCode bool             `json:"in_generated_code"`
	InTestCode      bool             `json:"in_test_code"`

	// Documentation and relationships
	Documentation  []string                `json:"documentation,omitempty"`
	Signature      string                  `json:"signature,omitempty"`
	Relationships  []scip.SCIPRelationship `json:"relationships,omitempty"`
	RelatedSymbols []string                `json:"related_symbols,omitempty"`

	// Scoring and relevance
	RelevanceScore  float64 `json:"relevance_score"`
	PopularityScore float64 `json:"popularity_score"`
	FinalScore      float64 `json:"final_score"`

	// File distribution
	DocumentURIs []string `json:"document_uris,omitempty"`
	FileCount    int      `json:"file_count"`
}

// EnhancedSymbolSearchResult wraps multiple enhanced symbol results with metadata
type EnhancedSymbolSearchResult struct {
	Symbols   []EnhancedSymbolResult `json:"symbols"`
	Total     int                    `json:"total"`
	Truncated bool                   `json:"truncated"`
	Query     *EnhancedSymbolQuery   `json:"query,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// ReferenceSearchOptions represents options for reference searching
type ReferenceSearchOptions struct {
	IncludeDefinition bool                  `json:"include_definition"`
	MaxResults        int                   `json:"max_results,omitempty"`
	SymbolRoles       []types.SymbolRole    `json:"symbol_roles,omitempty"`
	SymbolKinds       []scip.SCIPSymbolKind `json:"symbol_kinds,omitempty"`
	IncludeCode       bool                  `json:"include_code"`
	SortBy            string                `json:"sort_by,omitempty"` // "location", "relevance", "file"
}

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

// SCIPOccurrenceInfo represents occurrence information with context
type SCIPOccurrenceInfo struct {
	Occurrence  scip.SCIPOccurrence `json:"occurrence"`
	DocumentURI string              `json:"document_uri"`
	SymbolRoles types.SymbolRole    `json:"symbol_roles"`
	SyntaxKind  types.SyntaxKind    `json:"syntax_kind"`
	Context     string              `json:"context,omitempty"` // Surrounding code context
	LineNumber  int32               `json:"line_number"`
	Score       float64             `json:"score,omitempty"` // Relevance score
}
