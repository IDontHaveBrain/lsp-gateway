package search

import (
	"fmt"
	"sync"

	"lsp-gateway/src/server/scip"
)

// DefaultSearchFactory provides a default implementation of SearchFactory
type DefaultSearchFactory struct{}

// NewDefaultSearchFactory creates a new default search factory
func NewDefaultSearchFactory() *DefaultSearchFactory {
	return &DefaultSearchFactory{}
}

// CreateSearchService creates a new search service with the given configuration
func (f *DefaultSearchFactory) CreateSearchService(config *SearchServiceConfig) SearchServiceInterface {
	if config == nil {
		panic("SearchServiceConfig cannot be nil")
	}

	// Validate required fields
	if config.Storage == nil {
		panic("Storage is required in SearchServiceConfig")
	}
	if config.IndexMutex == nil {
		panic("IndexMutex is required in SearchServiceConfig")
	}

	return NewSearchService(config)
}

// CreateSearchHandler creates a search handler for the given type
func (f *DefaultSearchFactory) CreateSearchHandler(searchType SearchType) SearchHandler {
	switch searchType {
	case SearchTypeDefinition:
		return &DefinitionSearchHandler{}
	case SearchTypeReference:
		return &ReferenceSearchHandler{}
	case SearchTypeSymbol:
		return &SymbolSearchHandler{}
	case SearchTypeWorkspace:
		return &WorkspaceSearchHandler{}
	default:
		return &UnsupportedSearchHandler{searchType: searchType}
	}
}

// CreateResultBuilder creates a result builder
func (f *DefaultSearchFactory) CreateResultBuilder() ResultBuilder {
	return &DefaultResultBuilder{}
}

// SearchServiceFactory provides a singleton factory for search services
type SearchServiceFactory struct {
	factory SearchFactory
	mu      sync.RWMutex
}

var (
	factoryInstance *SearchServiceFactory
	factoryOnce     sync.Once
)

// GetSearchServiceFactory returns the singleton search service factory
func GetSearchServiceFactory() *SearchServiceFactory {
	factoryOnce.Do(func() {
		factoryInstance = &SearchServiceFactory{
			factory: NewDefaultSearchFactory(),
		}
	})
	return factoryInstance
}

// SetFactory sets a custom factory implementation
func (sf *SearchServiceFactory) SetFactory(factory SearchFactory) {
	sf.mu.Lock()
	defer sf.mu.Unlock()
	sf.factory = factory
}

// CreateSearchService creates a new search service
func (sf *SearchServiceFactory) CreateSearchService(config *SearchServiceConfig) SearchServiceInterface {
	sf.mu.RLock()
	defer sf.mu.RUnlock()
	return sf.factory.CreateSearchService(config)
}

// CreateSearchHandler creates a search handler
func (sf *SearchServiceFactory) CreateSearchHandler(searchType SearchType) SearchHandler {
	sf.mu.RLock()
	defer sf.mu.RUnlock()
	return sf.factory.CreateSearchHandler(searchType)
}

// CreateResultBuilder creates a result builder
func (sf *SearchServiceFactory) CreateResultBuilder() ResultBuilder {
	sf.mu.RLock()
	defer sf.mu.RUnlock()
	return sf.factory.CreateResultBuilder()
}

// SearchServiceBuilder provides a fluent interface for building search services
type SearchServiceBuilder struct {
	config *SearchServiceConfig
}

// NewSearchServiceBuilder creates a new search service builder
func NewSearchServiceBuilder() *SearchServiceBuilder {
	return &SearchServiceBuilder{
		config: &SearchServiceConfig{},
	}
}

// WithStorage sets the storage implementation
func (b *SearchServiceBuilder) WithStorage(storage StorageAccess) *SearchServiceBuilder {
	b.config.Storage = storage
	return b
}

// WithEnabled sets the enabled state
func (b *SearchServiceBuilder) WithEnabled(enabled bool) *SearchServiceBuilder {
	b.config.Enabled = enabled
	return b
}

// WithIndexMutex sets the index mutex
func (b *SearchServiceBuilder) WithIndexMutex(mu *sync.RWMutex) *SearchServiceBuilder {
	b.config.IndexMutex = mu
	return b
}

// WithMatchFilePatternFn sets the file pattern matching function
func (b *SearchServiceBuilder) WithMatchFilePatternFn(fn func(uri, pattern string) bool) *SearchServiceBuilder {
	b.config.MatchFilePatternFn = fn
	return b
}

// WithBuildOccurrenceInfoFn sets the occurrence info building function
func (b *SearchServiceBuilder) WithBuildOccurrenceInfoFn(fn func(occ *scip.SCIPOccurrence, docURI string) interface{}) *SearchServiceBuilder {
	b.config.BuildOccurrenceInfoFn = fn
	return b
}

// WithFormatSymbolDetailFn sets the symbol detail formatting function
func (b *SearchServiceBuilder) WithFormatSymbolDetailFn(fn func(symbolInfo *scip.SCIPSymbolInformation) string) *SearchServiceBuilder {
	b.config.FormatSymbolDetailFn = fn
	return b
}

// Build creates the search service with the configured options
func (b *SearchServiceBuilder) Build() (SearchServiceInterface, error) {
	if err := b.validate(); err != nil {
		return nil, err
	}

	factory := GetSearchServiceFactory()
	return factory.CreateSearchService(b.config), nil
}

// validate checks that all required configuration is provided
func (b *SearchServiceBuilder) validate() error {
	if b.config.Storage == nil {
		return fmt.Errorf("storage is required")
	}
	if b.config.IndexMutex == nil {
		return fmt.Errorf("index mutex is required")
	}
	if b.config.MatchFilePatternFn == nil {
		return fmt.Errorf("match file pattern function is required")
	}
	if b.config.BuildOccurrenceInfoFn == nil {
		return fmt.Errorf("build occurrence info function is required")
	}
	if b.config.FormatSymbolDetailFn == nil {
		return fmt.Errorf("format symbol detail function is required")
	}
	return nil
}

// GetConfig returns a copy of the current configuration
func (b *SearchServiceBuilder) GetConfig() *SearchServiceConfig {
	// Return a copy to prevent external modification
	return &SearchServiceConfig{
		Storage:               b.config.Storage,
		Enabled:               b.config.Enabled,
		IndexMutex:            b.config.IndexMutex,
		MatchFilePatternFn:    b.config.MatchFilePatternFn,
		BuildOccurrenceInfoFn: b.config.BuildOccurrenceInfoFn,
		FormatSymbolDetailFn:  b.config.FormatSymbolDetailFn,
	}
}

// Reset clears the builder configuration
func (b *SearchServiceBuilder) Reset() *SearchServiceBuilder {
	b.config = &SearchServiceConfig{}
	return b
}

// Clone creates a copy of the builder
func (b *SearchServiceBuilder) Clone() *SearchServiceBuilder {
	return &SearchServiceBuilder{
		config: b.GetConfig(),
	}
}
