package search

// SearchExecutor defines the interface for executing search operations
type SearchExecutor interface {
	// ExecuteSearch performs a unified search operation
	ExecuteSearch(request *SearchRequest) (*SearchResponse, error)

	// IsEnabled returns whether the search service is enabled
	IsEnabled() bool

	// SetEnabled updates the enabled state of the search service
	SetEnabled(enabled bool)
}

// GuardExecutor defines the interface for guard operations
type GuardExecutor interface {
	// WithEnabledGuard executes the provided function only if the service is enabled
	WithEnabledGuard(fn func() (*SearchResponse, error)) (*SearchResponse, error)
}

// LockExecutor defines the interface for lock management operations
type LockExecutor interface {
	// WithIndexReadLock executes a function while holding a read lock on the index mutex
	WithIndexReadLock(fn func() (*SearchResponse, error)) (*SearchResponse, error)
}

// SearchServiceInterface combines all search service capabilities
type SearchServiceInterface interface {
	SearchExecutor
	GuardExecutor
	LockExecutor
}

// SearchHandler defines the interface for type-specific search operations
type SearchHandler interface {
	// CanHandle returns true if this handler can process the given search type
	CanHandle(searchType SearchType) bool

	// Handle executes the search operation for the given request
	Handle(request *SearchRequest) (*SearchResponse, error)
}

// SearchMiddleware defines the interface for search middleware
type SearchMiddleware interface {
	// Process applies middleware logic to the search request/response
	Process(request *SearchRequest, next func(*SearchRequest) (*SearchResponse, error)) (*SearchResponse, error)
}

// ResultBuilder defines the interface for building search results
type ResultBuilder interface {
	// BuildSearchResponse creates a SearchResponse from raw data
	BuildSearchResponse(searchType SearchType, data interface{}, metadata *SearchMetadata) *SearchResponse

	// BuildErrorResponse creates an error SearchResponse
	BuildErrorResponse(searchType SearchType, err error) *SearchResponse

	// BuildDisabledResponse creates a response for when cache is disabled
	BuildDisabledResponse(searchType SearchType) *SearchResponse
}

// PatternMatcher defines the interface for file pattern matching
type PatternMatcher interface {
	// MatchFilePattern checks if a URI matches the given pattern
	MatchFilePattern(uri, pattern string) bool
}

// OccurrenceBuilder defines the interface for building occurrence information
type OccurrenceBuilder interface {
	// BuildOccurrenceInfo creates occurrence information from SCIP data
	BuildOccurrenceInfo(occ interface{}, docURI string) interface{}
}

// SymbolFormatter defines the interface for formatting symbol details
type SymbolFormatter interface {
	// FormatSymbolDetail formats symbol information for display
	FormatSymbolDetail(symbolInfo interface{}) string
}

// SearchFactory defines the interface for creating search services
type SearchFactory interface {
	// CreateSearchService creates a new search service with the given configuration
	CreateSearchService(config *SearchServiceConfig) SearchServiceInterface

	// CreateSearchHandler creates a search handler for the given type
	CreateSearchHandler(searchType SearchType) SearchHandler

	// CreateResultBuilder creates a result builder
	CreateResultBuilder() ResultBuilder
}

// SearchServiceManager defines the interface for managing search services
type SearchServiceManager interface {
	// GetSearchService returns the search service instance
	GetSearchService() SearchServiceInterface

	// RefreshSearchService recreates the search service with updated configuration
	RefreshSearchService(config *SearchServiceConfig) error

	// GetSupportedSearchTypes returns the list of supported search types
	GetSupportedSearchTypes() []SearchType
}

// SearchMetrics defines the interface for search metrics collection
type SearchMetrics interface {
	// RecordSearchRequest records a search request metric
	RecordSearchRequest(searchType SearchType, duration int64, resultCount int)

	// RecordSearchError records a search error metric
	RecordSearchError(searchType SearchType, errorType string)

	// GetSearchStats returns current search statistics
	GetSearchStats() map[string]interface{}
}

// ValidationRules defines the interface for search request validation
type ValidationRules interface {
	// ValidateSearchRequest validates a search request
	ValidateSearchRequest(request *SearchRequest) error

	// SanitizeSearchRequest sanitizes and normalizes a search request
	SanitizeSearchRequest(request *SearchRequest) *SearchRequest
}

// ConfigurationProvider defines the interface for search configuration
type ConfigurationProvider interface {
	// GetSearchConfig returns the current search configuration
	GetSearchConfig() *SearchServiceConfig

	// UpdateSearchConfig updates the search configuration
	UpdateSearchConfig(config *SearchServiceConfig) error

	// IsSearchEnabled returns whether search is enabled
	IsSearchEnabled() bool
}
