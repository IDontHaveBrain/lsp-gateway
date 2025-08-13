package search

import (
	"fmt"
	"time"

	"lsp-gateway/src/internal/constants"
	"lsp-gateway/src/server/scip"
)

// BaseSearchHandler provides common functionality for search handlers
type BaseSearchHandler struct {
	storage               StorageAccess
	matchFilePatternFn    func(uri, pattern string) bool
	buildOccurrenceInfoFn func(occ *scip.SCIPOccurrence, docURI string) interface{}
}

// DefinitionSearchHandler handles definition search operations
type DefinitionSearchHandler struct {
	BaseSearchHandler
}

// CanHandle returns true if this handler can process definition searches
func (h *DefinitionSearchHandler) CanHandle(searchType SearchType) bool {
	return searchType == SearchTypeDefinition
}

// Handle executes the definition search operation
func (h *DefinitionSearchHandler) Handle(request *SearchRequest) (*SearchResponse, error) {
	maxResults := request.MaxResults
	if maxResults <= 0 {
		maxResults = constants.DefaultMaxResults
	}

	symbolInfos, err := h.storage.SearchSymbols(request.Context, request.SymbolName, 10)
	if err != nil || len(symbolInfos) == 0 {
		return &SearchResponse{
			Type:      SearchTypeDefinition,
			Results:   []interface{}{},
			Metadata:  &SearchMetadata{SymbolsFound: 0, CacheEnabled: true, SCIPEnabled: true},
			Timestamp: time.Now(),
			Success:   true,
		}, nil
	}

	var allDefinitions []interface{}
	fileSet := make(map[string]bool)

	for _, symbolInfo := range symbolInfos {
		defOccs, err := h.storage.GetDefinitionsWithDocuments(request.Context, symbolInfo.Symbol)
		if err != nil || len(defOccs) == 0 {
			continue
		}

		defOcc := &defOccs[0]
		docURI := defOcc.DocumentURI

		if request.FilePattern != "" && !h.matchFilePatternFn(docURI, request.FilePattern) {
			continue
		}

		occInfo := h.buildOccurrenceInfoFn(&defOcc.SCIPOccurrence, docURI)
		allDefinitions = append(allDefinitions, occInfo)
		fileSet[docURI] = true

		if len(allDefinitions) >= maxResults {
			break
		}
	}

	return &SearchResponse{
		Type:      SearchTypeDefinition,
		Results:   allDefinitions,
		Total:     len(allDefinitions),
		Truncated: false,
		Metadata: &SearchMetadata{
			SCIPEnabled:  true,
			SymbolsFound: len(symbolInfos),
			FilePattern:  request.FilePattern,
			FilesMatched: len(fileSet),
			CacheEnabled: true,
		},
		Timestamp: time.Now(),
		Success:   true,
	}, nil
}

// ReferenceSearchHandler handles reference search operations
type ReferenceSearchHandler struct {
	BaseSearchHandler
}

// CanHandle returns true if this handler can process reference searches
func (h *ReferenceSearchHandler) CanHandle(searchType SearchType) bool {
	return searchType == SearchTypeReference
}

// Handle executes the reference search operation
func (h *ReferenceSearchHandler) Handle(request *SearchRequest) (*SearchResponse, error) {
	maxResults := request.MaxResults
	if maxResults <= 0 {
		maxResults = constants.DefaultMaxResults
	}

	symbolInfos, err := h.storage.SearchSymbols(request.Context, request.SymbolName, 10)
	if err != nil || len(symbolInfos) == 0 {
		return &SearchResponse{
			Type:      SearchTypeReference,
			Results:   []interface{}{},
			Metadata:  &SearchMetadata{SymbolsFound: 0, CacheEnabled: true, SCIPEnabled: true},
			Timestamp: time.Now(),
			Success:   true,
		}, nil
	}

	var allReferences []interface{}
	fileSet := make(map[string]bool)

	for _, symbolInfo := range symbolInfos {
		refOccs, err := h.storage.GetReferencesWithDocuments(request.Context, symbolInfo.Symbol)
		if err != nil {
			continue
		}

		for _, occWithDoc := range refOccs {
			docURI := occWithDoc.DocumentURI
			if request.FilePattern != "" && !h.matchFilePatternFn(docURI, request.FilePattern) {
				continue
			}

			occInfo := h.buildOccurrenceInfoFn(&occWithDoc.SCIPOccurrence, docURI)
			allReferences = append(allReferences, occInfo)
			fileSet[docURI] = true

			if len(allReferences) >= maxResults {
				goto done
			}
		}
	}

done:
	return &SearchResponse{
		Type:      SearchTypeReference,
		Results:   allReferences,
		Total:     len(allReferences),
		Truncated: len(allReferences) >= maxResults,
		Metadata: &SearchMetadata{
			SCIPEnabled:  true,
			SymbolsFound: len(symbolInfos),
			FilePattern:  request.FilePattern,
			FilesMatched: len(fileSet),
			CacheEnabled: true,
		},
		Timestamp: time.Now(),
		Success:   true,
	}, nil
}

// SymbolSearchHandler handles symbol search operations
type SymbolSearchHandler struct {
	BaseSearchHandler
}

// CanHandle returns true if this handler can process symbol searches
func (h *SymbolSearchHandler) CanHandle(searchType SearchType) bool {
	return searchType == SearchTypeSymbol
}

// Handle executes the symbol search operation
func (h *SymbolSearchHandler) Handle(request *SearchRequest) (*SearchResponse, error) {
	maxResults := request.MaxResults
	if maxResults <= 0 {
		maxResults = constants.DefaultMaxResults
	}

	searchLimit := maxResults
	if request.FilePattern != "" {
		searchLimit = maxResults * 10
		if searchLimit > 1000 {
			searchLimit = 1000
		}
	}

	symbolInfos, err := h.storage.SearchSymbols(request.Context, request.SymbolName, searchLimit)
	if err != nil {
		return &SearchResponse{
			Type:      SearchTypeSymbol,
			Results:   []interface{}{},
			Metadata:  &SearchMetadata{CacheEnabled: true, SCIPEnabled: true, Errors: []string{err.Error()}},
			Timestamp: time.Now(),
			Success:   false,
			Error:     err.Error(),
		}, nil
	}

	var results []interface{}
	for _, symbolInfo := range symbolInfos {
		var occWithDoc *scip.OccurrenceWithDocument

		// Prefer definition; else any occurrence
		defs, defErr := h.storage.GetDefinitionsWithDocuments(request.Context, symbolInfo.Symbol)
		if len(defs) > 0 && defErr == nil {
			occWithDoc = &defs[0]
		} else {
			occs, occErr := h.storage.GetOccurrencesWithDocuments(request.Context, symbolInfo.Symbol)
			if len(occs) > 0 && occErr == nil {
				occWithDoc = &occs[0]
			}
		}

		if occWithDoc == nil {
			continue
		}

		// Apply file filter
		if request.FilePattern != "" && !h.matchFilePatternFn(occWithDoc.DocumentURI, request.FilePattern) {
			continue
		}

		enhancedResult := map[string]interface{}{
			"symbolInfo":  symbolInfo,
			"occurrence":  &occWithDoc.SCIPOccurrence,
			"documentURI": occWithDoc.DocumentURI,
			"range":       occWithDoc.Range,
		}
		results = append(results, enhancedResult)

		if len(results) >= maxResults {
			break
		}
	}

	return &SearchResponse{
		Type:      SearchTypeSymbol,
		Results:   results,
		Total:     len(results),
		Truncated: len(symbolInfos) > maxResults,
		Metadata: &SearchMetadata{
			SCIPEnabled:     true,
			TotalCandidates: len(symbolInfos),
			FilePattern:     request.FilePattern,
			CacheEnabled:    true,
		},
		Timestamp: time.Now(),
		Success:   true,
	}, nil
}

// WorkspaceSearchHandler handles workspace symbol search operations
type WorkspaceSearchHandler struct {
	BaseSearchHandler
}

// CanHandle returns true if this handler can process workspace searches
func (h *WorkspaceSearchHandler) CanHandle(searchType SearchType) bool {
	return searchType == SearchTypeWorkspace
}

// Handle executes the workspace search operation
func (h *WorkspaceSearchHandler) Handle(request *SearchRequest) (*SearchResponse, error) {
	maxResults := request.MaxResults
	if maxResults <= 0 {
		maxResults = constants.DefaultMaxResults
	}

	symbolInfos, err := h.storage.SearchSymbols(request.Context, request.SymbolName, maxResults)
	if err != nil {
		return &SearchResponse{
			Type:      SearchTypeWorkspace,
			Results:   []interface{}{},
			Metadata:  &SearchMetadata{CacheEnabled: true, SCIPEnabled: true, Errors: []string{err.Error()}},
			Timestamp: time.Now(),
			Success:   false,
			Error:     err.Error(),
		}, nil
	}

	results := make([]interface{}, len(symbolInfos))
	for i, sym := range symbolInfos {
		results[i] = sym
	}

    stats := h.storage.GetIndexStats()

	return &SearchResponse{
		Type:      SearchTypeWorkspace,
		Results:   results,
		Total:     len(results),
		Truncated: false,
        Metadata: &SearchMetadata{
            SCIPEnabled:  true,
            CacheEnabled: true,
            IndexStats:  stats,
        },
		Timestamp: time.Now(),
		Success:   true,
	}, nil
}

// UnsupportedSearchHandler handles unsupported search operations
type UnsupportedSearchHandler struct {
	BaseSearchHandler
	searchType SearchType
}

// CanHandle returns false for unsupported search types
func (h *UnsupportedSearchHandler) CanHandle(searchType SearchType) bool {
	return false
}

// Handle returns an error result for unsupported search operations
func (h *UnsupportedSearchHandler) Handle(request *SearchRequest) (*SearchResponse, error) {
	return &SearchResponse{
		Type:      h.searchType,
		Results:   []interface{}{},
		Metadata:  &SearchMetadata{CacheEnabled: true, SCIPEnabled: true, Errors: []string{fmt.Sprintf("unsupported search type: %s", h.searchType)}},
		Timestamp: time.Now(),
		Success:   false,
		Error:     fmt.Sprintf("unsupported search type: %s", h.searchType),
	}, fmt.Errorf("unsupported search type: %s", h.searchType)
}

// HandlerRegistry manages search handlers
type HandlerRegistry struct {
	handlers map[SearchType]SearchHandler
}

// NewHandlerRegistry creates a new handler registry
func NewHandlerRegistry() *HandlerRegistry {
	return &HandlerRegistry{
		handlers: make(map[SearchType]SearchHandler),
	}
}

// RegisterHandler registers a search handler for a specific type
func (r *HandlerRegistry) RegisterHandler(searchType SearchType, handler SearchHandler) {
	r.handlers[searchType] = handler
}

// GetHandler returns the handler for a specific search type
func (r *HandlerRegistry) GetHandler(searchType SearchType) SearchHandler {
	if handler, exists := r.handlers[searchType]; exists {
		return handler
	}
	return &UnsupportedSearchHandler{searchType: searchType}
}

// GetSupportedTypes returns a list of supported search types
func (r *HandlerRegistry) GetSupportedTypes() []SearchType {
	types := make([]SearchType, 0, len(r.handlers))
	for searchType := range r.handlers {
		types = append(types, searchType)
	}
	return types
}

// HasHandler checks if a handler exists for the given search type
func (r *HandlerRegistry) HasHandler(searchType SearchType) bool {
	_, exists := r.handlers[searchType]
	return exists
}
