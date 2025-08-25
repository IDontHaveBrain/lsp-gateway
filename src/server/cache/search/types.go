package search

import (
	"context"
	"time"

	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/server/scip"
)

// SearchType defines the type of search operation
type SearchType string

const (
	SearchTypeDefinition SearchType = "definition"
	SearchTypeReference  SearchType = "reference"
	SearchTypeSymbol     SearchType = "symbol"
	SearchTypeWorkspace  SearchType = "workspace"
	SearchTypeHover      SearchType = "hover"
	SearchTypeCompletion SearchType = "completion"
)

// SearchRequest represents a unified search request with type-specific options
type SearchRequest struct {
	// Core search parameters
	Type        SearchType      `json:"type" validate:"required,oneof=definition reference symbol workspace hover completion"`
	SymbolName  string          `json:"symbol_name" validate:"required"`
	FilePattern string          `json:"file_pattern,omitempty"`
	MaxResults  int             `json:"max_results,omitempty" validate:"min=0,max=10000"`
	Context     context.Context `json:"-"`

	// Position-based parameters (for hover, completion, etc.)
	URI      string          `json:"uri,omitempty"`
	Position *types.Position `json:"position,omitempty"`

	// Language filtering
	Language string `json:"language,omitempty"`

	// Type-specific options (unmarshaled based on Type)
	Options interface{} `json:"options,omitempty"`

	// Common search options
	SearchOptions *SearchOptions `json:"search_options,omitempty"`

	// Request metadata
	RequestID string                 `json:"request_id,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// SearchResponse represents a unified search response wrapper
type SearchResponse struct {
	// Request identification
	Type      string `json:"type"`
	RequestID string `json:"request_id,omitempty"`

	// Results
	Results   []interface{} `json:"results"`
	Total     int           `json:"total"`
	Truncated bool          `json:"truncated"`

	// Response metadata
	Metadata  *SearchMetadata `json:"metadata"`
	Timestamp time.Time       `json:"timestamp"`
	Duration  time.Duration   `json:"duration,omitempty"`
	Success   bool            `json:"success"`
	Error     string          `json:"error,omitempty"`
}

// SearchOptions contains common parameters for all search types
type SearchOptions struct {
	// Result filtering
	MaxResults     int      `json:"max_results,omitempty" validate:"min=0,max=10000"`
	MinOccurrences int      `json:"min_occurrences,omitempty" validate:"min=0"`
	MaxOccurrences int      `json:"max_occurrences,omitempty" validate:"min=0"`
	FilePatterns   []string `json:"file_patterns,omitempty"`
	ExcludeFiles   []string `json:"exclude_files,omitempty"`

	// Symbol filtering
	SymbolKinds  []scip.SCIPSymbolKind `json:"symbol_kinds,omitempty"`
	SymbolRoles  []types.SymbolRole    `json:"symbol_roles,omitempty"`
	ExcludeRoles []types.SymbolRole    `json:"exclude_roles,omitempty"`

	// Content filtering
	IncludeCode          bool `json:"include_code"`
	IncludeDocumentation bool `json:"include_documentation"`
	IncludeRelationships bool `json:"include_relationships"`
	OnlyWithDefinition   bool `json:"only_with_definition"`

	// Result ordering and scoring
	SortBy       string  `json:"sort_by,omitempty"`    // "relevance", "name", "location", "occurrences", "kind", "file"
	SortOrder    string  `json:"sort_order,omitempty"` // "asc", "desc"
	IncludeScore bool    `json:"include_score"`
	MinScore     float64 `json:"min_score,omitempty"`

	// Performance options
	TimeoutMs     int  `json:"timeout_ms,omitempty" validate:"min=0,max=300000"`
	UseCache      bool `json:"use_cache"`
	RefreshCache  bool `json:"refresh_cache"`
	ParallelQuery bool `json:"parallel_query"`

	// Debug options
	IncludeDebugInfo bool `json:"include_debug_info"`
	VerboseMetadata  bool `json:"verbose_metadata"`
}

// SearchMetadata contains metadata about search execution and results
type SearchMetadata struct {
	// Performance metrics
	ExecutionTime   time.Duration `json:"execution_time"`
	IndexingTime    time.Duration `json:"indexing_time,omitempty"`
	CacheHitRate    float64       `json:"cache_hit_rate,omitempty"`
	ParallelQueries int           `json:"parallel_queries,omitempty"`

	// Result statistics
	TotalCandidates  int `json:"total_candidates"`
	FilteredResults  int `json:"filtered_results"`
	SymbolsFound     int `json:"symbols_found"`
	DocumentsScanned int `json:"documents_scanned"`
	FilesMatched     int `json:"files_matched"`

	// Index statistics
	IndexStats       *scip.IndexStats `json:"index_stats,omitempty"`
	IndexedLanguages []string         `json:"indexed_languages,omitempty"`
	IndexStatus      string           `json:"index_status,omitempty"`

	// Search context
	SearchPattern  string                 `json:"search_pattern,omitempty"`
	FilePattern    string                 `json:"file_pattern,omitempty"`
	Language       string                 `json:"language,omitempty"`
	FilterCriteria map[string]interface{} `json:"filter_criteria,omitempty"`

	// Error information
	Warnings []string               `json:"warnings,omitempty"`
	Errors   []string               `json:"errors,omitempty"`
	Debug    map[string]interface{} `json:"debug,omitempty"`

	// System state
	CacheEnabled  bool   `json:"cache_enabled"`
	SCIPEnabled   bool   `json:"scip_enabled"`
	ServerVersion string `json:"server_version,omitempty"`
}

// Position type unified to internal/types.Position

// IndexStats unified to scip.IndexStats

// DefinitionSearchRequest contains definition-specific search parameters
type DefinitionSearchRequest struct {
	SymbolName  string          `json:"symbol_name" validate:"required"`
	FilePattern string          `json:"file_pattern,omitempty"`
	MaxResults  int             `json:"max_results,omitempty" validate:"min=0,max=10000"`
	Options     *SearchOptions  `json:"options,omitempty"`
	Context     context.Context `json:"-"`
}

// DefinitionSearchResponse contains definition search results
type DefinitionSearchResponse struct {
	SymbolName  string               `json:"symbol_name"`
	Definitions []SCIPOccurrenceInfo `json:"definitions"`
	TotalCount  int                  `json:"total_count"`
	FileCount   int                  `json:"file_count"`
	Metadata    *SearchMetadata      `json:"metadata"`
	Timestamp   time.Time            `json:"timestamp"`
}

// ReferenceSearchRequest contains reference-specific search parameters
type ReferenceSearchRequest struct {
	SymbolName        string                  `json:"symbol_name" validate:"required"`
	FilePattern       string                  `json:"file_pattern,omitempty"`
	MaxResults        int                     `json:"max_results,omitempty" validate:"min=0,max=10000"`
	IncludeDefinition bool                    `json:"include_definition"`
	Options           *ReferenceSearchOptions `json:"options,omitempty"`
	Context           context.Context         `json:"-"`
}

// ReferenceSearchOptions represents options for reference searching
type ReferenceSearchOptions struct {
	MaxResults  int                   `json:"max_results,omitempty" validate:"min=0,max=10000"`
	SymbolRoles []types.SymbolRole    `json:"symbol_roles,omitempty"`
	SymbolKinds []scip.SCIPSymbolKind `json:"symbol_kinds,omitempty"`
	IncludeCode bool                  `json:"include_code"`
	SortBy      string                `json:"sort_by,omitempty"` // "location", "relevance", "file"
}

// ReferenceSearchResponse contains reference search results
type ReferenceSearchResponse struct {
	SymbolName string                  `json:"symbol_name"`
	SymbolID   string                  `json:"symbol_id,omitempty"`
	References []SCIPOccurrenceInfo    `json:"references"`
	Definition *SCIPOccurrenceInfo     `json:"definition,omitempty"`
	TotalCount int                     `json:"total_count"`
	FileCount  int                     `json:"file_count"`
	Options    *ReferenceSearchOptions `json:"options,omitempty"`
	Metadata   *SearchMetadata         `json:"metadata"`
	Timestamp  time.Time               `json:"timestamp"`
}

// SymbolSearchRequest contains symbol-specific search parameters
type SymbolSearchRequest struct {
	Pattern     string               `json:"pattern" validate:"required"`
	FilePattern string               `json:"file_pattern,omitempty"`
	Language    string               `json:"language,omitempty"`
	MaxResults  int                  `json:"max_results,omitempty" validate:"min=0,max=10000"`
	Enhanced    bool                 `json:"enhanced"`
	Query       *EnhancedSymbolQuery `json:"query,omitempty"`
	Options     *SearchOptions       `json:"options,omitempty"`
	Context     context.Context      `json:"-"`
}

// EnhancedSymbolQuery contains enhanced symbol search parameters
type EnhancedSymbolQuery struct {
	// Basic query parameters
	Pattern     string `json:"pattern" validate:"required"`
	FilePattern string `json:"file_pattern,omitempty"`
	Language    string `json:"language,omitempty"`
	MaxResults  int    `json:"max_results,omitempty" validate:"min=0,max=10000"`

	// Occurrence-based filtering
	QueryType      OccurrenceQueryType   `json:"query_type,omitempty"`
	SymbolRoles    []types.SymbolRole    `json:"symbol_roles,omitempty"`
	SymbolKinds    []scip.SCIPSymbolKind `json:"symbol_kinds,omitempty"`
	ExcludeRoles   []types.SymbolRole    `json:"exclude_roles,omitempty"`
	MinOccurrences int                   `json:"min_occurrences,omitempty" validate:"min=0"`
	MaxOccurrences int                   `json:"max_occurrences,omitempty" validate:"min=0"`

	// Advanced filtering
	IncludeDocumentation bool   `json:"include_documentation"`
	IncludeRelationships bool   `json:"include_relationships"`
	OnlyWithDefinition   bool   `json:"only_with_definition"`
	SortBy               string `json:"sort_by,omitempty"` // "relevance", "name", "occurrences", "kind"
	IncludeScore         bool   `json:"include_score"`
}

// OccurrenceQueryType defines the type of occurrence query
type OccurrenceQueryType string

const (
	OccurrenceQueryTypeAll        OccurrenceQueryType = "all"
	OccurrenceQueryTypeDefinition OccurrenceQueryType = "definition"
	OccurrenceQueryTypeReference  OccurrenceQueryType = "reference"
	OccurrenceQueryTypeWrite      OccurrenceQueryType = "write"
	OccurrenceQueryTypeRead       OccurrenceQueryType = "read"
)

// SymbolSearchResponse contains symbol search results
type SymbolSearchResponse struct {
	Pattern    string          `json:"pattern"`
	Symbols    []interface{}   `json:"symbols"`
	TotalCount int             `json:"total_count"`
	Truncated  bool            `json:"truncated"`
	Enhanced   bool            `json:"enhanced"`
	Metadata   *SearchMetadata `json:"metadata"`
	Timestamp  time.Time       `json:"timestamp"`
}

// EnhancedSymbolSearchResponse contains enhanced symbol search results
type EnhancedSymbolSearchResponse struct {
	Symbols   []EnhancedSymbolResult `json:"symbols"`
	Total     int                    `json:"total"`
	Truncated bool                   `json:"truncated"`
	Query     *EnhancedSymbolQuery   `json:"query,omitempty"`
	Metadata  *SearchMetadata        `json:"metadata"`
	Timestamp time.Time              `json:"timestamp"`
}

// EnhancedSymbolResult represents enhanced symbol search result information
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

	// Relevance and scoring
	Score          float64 `json:"score,omitempty"`
	Relevance      float64 `json:"relevance,omitempty"`
	UsageFrequency int     `json:"usage_frequency,omitempty"`

	// Context information
	Documentation   []string                `json:"documentation,omitempty"`
	Signature       string                  `json:"signature,omitempty"`
	Relationships   []scip.SCIPRelationship `json:"relationships,omitempty"`
	PrimaryLocation *types.Location         `json:"primary_location,omitempty"`

	// File and context metadata
	FileCount int                    `json:"file_count"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
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

// SymbolInfoRequest contains symbol information request parameters
type SymbolInfoRequest struct {
	SymbolName  string          `json:"symbol_name" validate:"required"`
	FilePattern string          `json:"file_pattern,omitempty"`
	Detailed    bool            `json:"detailed"`
	Options     *SearchOptions  `json:"options,omitempty"`
	Context     context.Context `json:"-"`
}

// SymbolInfoResponse contains comprehensive symbol information
type SymbolInfoResponse struct {
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
	Metadata        *SearchMetadata             `json:"metadata"`
	Timestamp       time.Time                   `json:"timestamp"`
}

// WorkspaceSearchRequest contains workspace-specific search parameters
type WorkspaceSearchRequest struct {
	Pattern    string          `json:"pattern" validate:"required"`
	Language   string          `json:"language,omitempty"`
	MaxResults int             `json:"max_results,omitempty" validate:"min=0,max=10000"`
	Options    *SearchOptions  `json:"options,omitempty"`
	Context    context.Context `json:"-"`
}

// WorkspaceSearchResponse contains workspace symbol search results
type WorkspaceSearchResponse struct {
	Pattern   string          `json:"pattern"`
	Symbols   []interface{}   `json:"symbols"`
	Total     int             `json:"total"`
	Truncated bool            `json:"truncated"`
	Metadata  *SearchMetadata `json:"metadata"`
	Timestamp time.Time       `json:"timestamp"`
}

// SearchServiceConfig holds configuration for creating a SearchService
type SearchServiceConfig struct {
	// Core dependencies
	Storage    StorageAccess `json:"-"`
	Enabled    bool          `json:"enabled"`
	IndexMutex interface{}   `json:"-"` // *sync.RWMutex

	// Helper functions
	MatchFilePatternFn    func(uri, pattern string) bool                            `json:"-"`
	BuildOccurrenceInfoFn func(occ *scip.SCIPOccurrence, docURI string) interface{} `json:"-"`
	FormatSymbolDetailFn  func(symbolInfo *scip.SCIPSymbolInformation) string       `json:"-"`

	// Performance settings
	DefaultMaxResults    int           `json:"default_max_results"`
	DefaultTimeout       time.Duration `json:"default_timeout"`
	CacheEnabled         bool          `json:"cache_enabled"`
	ParallelQueryEnabled bool          `json:"parallel_query_enabled"`

	// Feature flags
	EnhancedSearchEnabled bool `json:"enhanced_search_enabled"`
	DebugEnabled          bool `json:"debug_enabled"`
	MetricsEnabled        bool `json:"metrics_enabled"`

	// Search behavior
	FuzzyMatchEnabled    bool    `json:"fuzzy_match_enabled"`
	ScoreThreshold       float64 `json:"score_threshold"`
	MaxConcurrentQueries int     `json:"max_concurrent_queries"`
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string      `json:"field"`
	Value   interface{} `json:"value"`
	Message string      `json:"message"`
}

// Error implements the error interface
func (v ValidationError) Error() string {
	return v.Message
}

// SearchError represents a search-specific error
type SearchError struct {
	Type      SearchType             `json:"type"`
	Code      string                 `json:"code"`
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details,omitempty"`
	Retryable bool                   `json:"retryable"`
	Timestamp time.Time              `json:"timestamp"`
}

// Error implements the error interface
func (s SearchError) Error() string {
	return s.Message
}

// SearchStats represents search performance statistics
type SearchStats struct {
	TotalRequests       int64                `json:"total_requests"`
	SuccessfulRequests  int64                `json:"successful_requests"`
	FailedRequests      int64                `json:"failed_requests"`
	AverageResponseTime time.Duration        `json:"average_response_time"`
	CacheHitRate        float64              `json:"cache_hit_rate"`
	IndexUtilization    float64              `json:"index_utilization"`
	LastRequestTime     time.Time            `json:"last_request_time"`
	RequestsByType      map[SearchType]int64 `json:"requests_by_type"`
	ErrorsByType        map[string]int64     `json:"errors_by_type"`
}
