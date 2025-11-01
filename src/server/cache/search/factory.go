package search

import (
	"sync"
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
