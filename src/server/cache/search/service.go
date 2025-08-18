package search

import (
    "context"
    "fmt"
    "sync"
    "time"

	"lsp-gateway/src/internal/constants"
	"lsp-gateway/src/internal/types"
	"lsp-gateway/src/server/scip"
)

// StorageAccess defines the interface for SCIP storage operations
type StorageAccess interface {
	SearchSymbols(ctx context.Context, pattern string, maxResults int) ([]scip.SCIPSymbolInformation, error)
	GetDefinitions(ctx context.Context, symbolID string) ([]scip.SCIPOccurrence, error)
	GetDefinitionsWithDocuments(ctx context.Context, symbolID string) ([]scip.OccurrenceWithDocument, error)
	GetReferences(ctx context.Context, symbolID string) ([]scip.SCIPOccurrence, error)
	GetReferencesWithDocuments(ctx context.Context, symbolID string) ([]scip.OccurrenceWithDocument, error)
	GetOccurrences(ctx context.Context, symbolID string) ([]scip.SCIPOccurrence, error)
	GetOccurrencesWithDocuments(ctx context.Context, symbolID string) ([]scip.OccurrenceWithDocument, error)
	GetIndexStats() *scip.IndexStats
	ListDocuments(ctx context.Context) ([]string, error)
	GetDocument(ctx context.Context, uri string) (*scip.SCIPDocument, error)
}

// SearchService provides unified search capabilities with consolidated patterns
type SearchService struct {
	storage StorageAccess
	guard   *SearchGuard
	indexMu *sync.RWMutex

	// Helper functions from cache manager
	matchFilePatternFn    func(uri, pattern string) bool
	buildOccurrenceInfoFn func(occ *scip.SCIPOccurrence, docURI string) interface{}
	formatSymbolDetailFn  func(symbolInfo *scip.SCIPSymbolInformation) string
}

// NewSearchService creates a new unified search service
func NewSearchService(config *SearchServiceConfig) *SearchService {
	return &SearchService{
		storage:               config.Storage,
		guard:                 NewSearchGuard(config.Enabled),
		indexMu:               config.IndexMutex.(*sync.RWMutex),
		matchFilePatternFn:    config.MatchFilePatternFn,
		buildOccurrenceInfoFn: config.BuildOccurrenceInfoFn,
		formatSymbolDetailFn:  config.FormatSymbolDetailFn,
	}
}

// ExecuteSearch performs unified search operation dispatching to type-specific handlers
func (s *SearchService) ExecuteSearch(request *SearchRequest) (*SearchResponse, error) {
	switch request.Type {
	case SearchTypeDefinition:
		return s.ExecuteDefinitionSearch(request)
	case SearchTypeReference:
		return s.ExecuteReferenceSearch(request)
	case SearchTypeSymbol:
		return s.ExecuteSymbolSearch(request)
	case SearchTypeWorkspace:
		return s.ExecuteWorkspaceSearch(request)
	default:
		return &SearchResponse{
			Type:      request.Type,
			Results:   []interface{}{},
			Total:     0,
			Truncated: false,
			Metadata:  &SearchMetadata{CacheEnabled: s.guard.enabled, SCIPEnabled: s.guard.enabled},
			Timestamp: time.Now(),
			Success:   false,
			Error:     fmt.Sprintf("unsupported search type: %s", request.Type),
		}, fmt.Errorf("unsupported search type: %s", request.Type)
	}
}

// ExecuteDefinitionSearch consolidates definition search logic
func (s *SearchService) ExecuteDefinitionSearch(request *SearchRequest) (*SearchResponse, error) {
    return s.guard.WithSearchResponse(SearchTypeDefinition, func() (*SearchResponse, error) {
        return s.withIndexReadLock(func() (*SearchResponse, error) {
            h := &DefinitionSearchHandler{BaseSearchHandler: BaseSearchHandler{storage: s.storage, matchFilePatternFn: s.matchFilePatternFn, buildOccurrenceInfoFn: s.buildOccurrenceInfoFn}}
            return h.Handle(request)
        })
    })
}


// ExecuteReferenceSearch consolidates reference search logic
func (s *SearchService) ExecuteReferenceSearch(request *SearchRequest) (*SearchResponse, error) {
    return s.guard.WithSearchResponse(SearchTypeReference, func() (*SearchResponse, error) {
        return s.withIndexReadLock(func() (*SearchResponse, error) {
            h := &ReferenceSearchHandler{BaseSearchHandler: BaseSearchHandler{storage: s.storage, matchFilePatternFn: s.matchFilePatternFn, buildOccurrenceInfoFn: s.buildOccurrenceInfoFn}}
            return h.Handle(request)
        })
    })
}


// ExecuteSymbolSearch consolidates symbol search logic
func (s *SearchService) ExecuteSymbolSearch(request *SearchRequest) (*SearchResponse, error) {
    return s.guard.WithSearchResponse(SearchTypeSymbol, func() (*SearchResponse, error) {
        return s.withIndexReadLock(func() (*SearchResponse, error) {
            h := &SymbolSearchHandler{BaseSearchHandler: BaseSearchHandler{storage: s.storage, matchFilePatternFn: s.matchFilePatternFn, buildOccurrenceInfoFn: s.buildOccurrenceInfoFn}}
            return h.Handle(request)
        })
    })
}


// ExecuteWorkspaceSearch consolidates workspace search logic
func (s *SearchService) ExecuteWorkspaceSearch(request *SearchRequest) (*SearchResponse, error) {
    return s.guard.WithSearchResponse(SearchTypeWorkspace, func() (*SearchResponse, error) {
        return s.withIndexReadLock(func() (*SearchResponse, error) {
            h := &WorkspaceSearchHandler{BaseSearchHandler: BaseSearchHandler{storage: s.storage, matchFilePatternFn: s.matchFilePatternFn, buildOccurrenceInfoFn: s.buildOccurrenceInfoFn}}
            return h.Handle(request)
        })
    })
}


// Enhanced search operations for specialized use cases

// ExecuteEnhancedSymbolSearch provides enhanced symbol search with detailed metadata
func (s *SearchService) ExecuteEnhancedSymbolSearch(query *EnhancedSymbolQuery) (*EnhancedSymbolSearchResponse, error) {
	return s.guard.WithEnhancedSymbolResult(query, func() (*EnhancedSymbolSearchResponse, error) {
		result, err := s.withIndexReadLockTyped(func() (interface{}, error) {
			return s.executeEnhancedSymbolSearchInternal(query)
		})
		if err != nil {
			return nil, err
		}
		return result.(*EnhancedSymbolSearchResponse), nil
	})
}

// executeEnhancedSymbolSearchInternal contains the actual enhanced symbol search logic
func (s *SearchService) executeEnhancedSymbolSearchInternal(query *EnhancedSymbolQuery) (*EnhancedSymbolSearchResponse, error) {
	maxResults := constants.DefaultMaxResults
	if query.MaxResults > 0 {
		maxResults = query.MaxResults
	}

	searchSymbols, err := s.storage.SearchSymbols(context.Background(), query.Pattern, maxResults*2)
	if err != nil {
		return &EnhancedSymbolSearchResponse{
			Symbols:   []EnhancedSymbolResult{},
			Total:     0,
			Truncated: false,
			Query:     query,
			Metadata:  &SearchMetadata{CacheEnabled: true, SCIPEnabled: true, Errors: []string{err.Error()}},
			Timestamp: time.Now(),
		}, nil
	}

	var enhancedResults []EnhancedSymbolResult
	for i, symbolInfo := range searchSymbols {
		if i >= maxResults {
			break
		}

		// Apply filters
		if len(query.SymbolKinds) > 0 {
			kindMatched := false
			for _, kind := range query.SymbolKinds {
				if symbolInfo.Kind == kind {
					kindMatched = true
					break
				}
			}
			if !kindMatched {
				continue
			}
		}

		occurrences, _ := s.storage.GetOccurrences(context.Background(), symbolInfo.Symbol)

		if query.MinOccurrences > 0 && len(occurrences) < query.MinOccurrences {
			continue
		}
		if query.MaxOccurrences > 0 && len(occurrences) > query.MaxOccurrences {
			continue
		}

		sig := ""
		if s.formatSymbolDetailFn != nil {
			sig = s.formatSymbolDetailFn(&symbolInfo)
		}
		enhancedResults = append(enhancedResults, BuildEnhancedSymbolResult(&symbolInfo, occurrences, sig, query != nil && query.IncludeDocumentation, false))
	}

	// Apply sorting if specified
	if query.SortBy != "" {
        SortEnhancedResults(enhancedResults, query.SortBy)
	}

	return &EnhancedSymbolSearchResponse{
		Symbols:   enhancedResults,
		Total:     len(enhancedResults),
		Truncated: len(searchSymbols) > maxResults,
		Query:     query,
		Metadata: &SearchMetadata{
			CacheEnabled:    true,
			SCIPEnabled:     true,
			TotalCandidates: len(searchSymbols),
		},
		Timestamp: time.Now(),
	}, nil
}

// ExecuteReferenceSearchEnhanced provides enhanced reference search
func (s *SearchService) ExecuteReferenceSearchEnhanced(symbolName, filePattern string, options *ReferenceSearchOptions) (*ReferenceSearchResponse, error) {
	return s.guard.WithReferenceResult(symbolName, options, func() (*ReferenceSearchResponse, error) {
		result, err := s.withIndexReadLockTyped(func() (interface{}, error) {
			return s.executeReferenceSearchEnhancedInternal(symbolName, filePattern, options)
		})
		if err != nil {
			return nil, err
		}
		return result.(*ReferenceSearchResponse), nil
	})
}

// executeReferenceSearchEnhancedInternal contains the actual enhanced reference search logic
func (s *SearchService) executeReferenceSearchEnhancedInternal(symbolName, filePattern string, options *ReferenceSearchOptions) (*ReferenceSearchResponse, error) {
	if options == nil {
		options = &ReferenceSearchOptions{MaxResults: constants.DefaultMaxResults}
	}

	symbolInfos, err := s.storage.SearchSymbols(context.Background(), symbolName, 10)
	if err != nil || len(symbolInfos) == 0 {
		return &ReferenceSearchResponse{
			SymbolName: symbolName,
			References: []SCIPOccurrenceInfo{},
			TotalCount: 0,
			FileCount:  0,
			Options:    options,
			Metadata:   &SearchMetadata{CacheEnabled: true, SCIPEnabled: true, SymbolsFound: 0},
			Timestamp:  time.Now(),
		}, nil
	}

	var allReferences []SCIPOccurrenceInfo
	var definition *SCIPOccurrenceInfo
	fileSet := make(map[string]bool)
	symbolID := ""

	for _, symbolInfo := range symbolInfos {
		symbolID = symbolInfo.Symbol

		// Get references
		refOccurrences, err := s.storage.GetReferencesWithDocuments(context.Background(), symbolInfo.Symbol)
		if err == nil {
			for _, occWithDoc := range refOccurrences {
				docURI := occWithDoc.DocumentURI
				if filePattern != "" && !s.matchFilePatternFn(docURI, filePattern) {
					continue
				}

				occInfo := s.buildOccurrenceInfoFn(&occWithDoc.SCIPOccurrence, docURI)
				if scipOccInfo, ok := occInfo.(SCIPOccurrenceInfo); ok {
					allReferences = append(allReferences, scipOccInfo)
					fileSet[docURI] = true

					if options.MaxResults > 0 && len(allReferences) >= options.MaxResults {
						break
					}
				}
			}
		}

		// Get definition if not already found
		if definition == nil {
			defOccs, err := s.storage.GetDefinitionsWithDocuments(context.Background(), symbolInfo.Symbol)
			if err == nil && len(defOccs) > 0 {
				defOcc := &defOccs[0]
				docURI := defOcc.DocumentURI
				if filePattern == "" || s.matchFilePatternFn(docURI, filePattern) {
					defInfo := s.buildOccurrenceInfoFn(&defOcc.SCIPOccurrence, docURI)
					if scipDefInfo, ok := defInfo.(SCIPOccurrenceInfo); ok {
						definition = &scipDefInfo
						fileSet[docURI] = true
					}
				}
			}
		}

		if options.MaxResults > 0 && len(allReferences) >= options.MaxResults {
			break
		}
	}

	// Apply sorting if specified
	if options.SortBy != "" {
        SortOccurrenceResults(allReferences, options.SortBy)
	}

	return &ReferenceSearchResponse{
		SymbolName: symbolName,
		SymbolID:   symbolID,
		References: allReferences,
		Definition: definition,
		TotalCount: len(allReferences),
		FileCount:  len(fileSet),
		Options:    options,
		Metadata: &SearchMetadata{
			CacheEnabled: true,
			SCIPEnabled:  true,
			SymbolsFound: len(symbolInfos),
			FilePattern:  filePattern,
		},
		Timestamp: time.Now(),
	}, nil
}

// Helper methods

// withIndexReadLock executes a function while holding a read lock on the index mutex
func (s *SearchService) withIndexReadLock(fn func() (*SearchResponse, error)) (*SearchResponse, error) {
	s.indexMu.RLock()
	defer s.indexMu.RUnlock()
	return fn()
}

// withIndexReadLockTyped executes a function while holding a read lock on the index mutex (typed interface)
func (s *SearchService) withIndexReadLockTyped(fn func() (interface{}, error)) (interface{}, error) {
	s.indexMu.RLock()
	defer s.indexMu.RUnlock()
	return fn()
}

// normalizeMaxResults applies default max results if not specified
func (s *SearchService) normalizeMaxResults(maxResults int) int {
	if maxResults <= 0 {
		return constants.DefaultMaxResults
	}
	return maxResults
}

// buildSearchResponse constructs a standardized SearchResponse
func (s *SearchService) buildSearchResponse(searchType SearchType, results []interface{}, metadata *SearchMetadata, request *SearchRequest) *SearchResponse {
    return &SearchResponse{
        Type:      searchType,
        RequestID: request.RequestID,
        Results:   results,
        Total:     len(results),
        Truncated: false,
        Metadata:  metadata,
        Timestamp: time.Now(),
        Success:   true,
    }
}

// IsEnabled returns whether the search service is enabled
func (s *SearchService) IsEnabled() bool {
	return s.guard.enabled
}

// SetEnabled updates the enabled state of the search service
func (s *SearchService) SetEnabled(enabled bool) {
	s.guard.enabled = enabled
}

// GetGuard returns the search guard for direct access if needed
func (s *SearchService) GetGuard() *SearchGuard {
	return s.guard
}

// GetStorage returns the storage interface for direct access if needed
func (s *SearchService) GetStorage() StorageAccess {
	return s.storage
}

// GetIndexMutex returns the index mutex for external synchronization if needed
func (s *SearchService) GetIndexMutex() *sync.RWMutex {
	return s.indexMu
}

// WithEnabledGuard executes the provided function only if the service is enabled
// This method implements the GuardExecutor interface
func (s *SearchService) WithEnabledGuard(fn func() (*SearchResponse, error)) (*SearchResponse, error) {
	return s.guard.WithSearchResponse(SearchTypeSymbol, fn)
}

// WithIndexReadLock executes a function while holding a read lock on the index mutex
// This method implements the LockExecutor interface
func (s *SearchService) WithIndexReadLock(fn func() (*SearchResponse, error)) (*SearchResponse, error) {
	return s.withIndexReadLock(fn)
}

// ExecuteSymbolInfoSearch provides detailed symbol information retrieval
func (s *SearchService) ExecuteSymbolInfoSearch(symbolName, filePattern string) (*SymbolInfoResponse, error) {
	return s.guard.WithSymbolInfoResult(symbolName, func() (*SymbolInfoResponse, error) {
		result, err := s.withIndexReadLockTyped(func() (interface{}, error) {
			return s.executeSymbolInfoSearchInternal(symbolName, filePattern)
		})
		if err != nil {
			return nil, err
		}
		return result.(*SymbolInfoResponse), nil
	})
}

// executeSymbolInfoSearchInternal contains the actual symbol info search logic
func (s *SearchService) executeSymbolInfoSearchInternal(symbolName, filePattern string) (*SymbolInfoResponse, error) {
	symbolInfos, err := s.storage.SearchSymbols(context.Background(), symbolName, 10)
	if err != nil || len(symbolInfos) == 0 {
		return &SymbolInfoResponse{
			SymbolName:      symbolName,
			Kind:            scip.SCIPSymbolKindUnknown,
			Documentation:   []string{},
			Occurrences:     []SCIPOccurrenceInfo{},
			OccurrenceCount: 0,
			DefinitionCount: 0,
			ReferenceCount:  0,
			FileCount:       0,
			Metadata:        &SearchMetadata{CacheEnabled: true, SCIPEnabled: true, SymbolsFound: 0},
			Timestamp:       time.Now(),
		}, nil
	}

	symbolInfo := symbolInfos[0]
	symbolID := symbolInfo.Symbol

	// Get all occurrences
	allOccurrences, _ := s.storage.GetOccurrencesWithDocuments(context.Background(), symbolID)

	var filteredOccurrences []SCIPOccurrenceInfo
	fileSet := make(map[string]bool)
	definitionCount := 0
	referenceCount := 0

	for _, occWithDoc := range allOccurrences {
		docURI := occWithDoc.DocumentURI
		occ := &occWithDoc.SCIPOccurrence

		if filePattern != "" && !s.matchFilePatternFn(docURI, filePattern) {
			continue
		}

		occInfo := s.buildOccurrenceInfoFn(occ, docURI)
		if scipOccInfo, ok := occInfo.(SCIPOccurrenceInfo); ok {
			filteredOccurrences = append(filteredOccurrences, scipOccInfo)
			fileSet[docURI] = true

			if occ.SymbolRoles.HasRole(types.SymbolRoleDefinition) {
				definitionCount++
			}
			if occ.SymbolRoles.HasRole(types.SymbolRoleReadAccess) || occ.SymbolRoles.HasRole(types.SymbolRoleWriteAccess) {
				referenceCount++
			}
		}
	}

	signature := ""
	if s.formatSymbolDetailFn != nil {
		signature = s.formatSymbolDetailFn(&symbolInfo)
	}

	return &SymbolInfoResponse{
		SymbolName:      symbolName,
		SymbolID:        symbolID,
		SymbolInfo:      &symbolInfo,
		Kind:            symbolInfo.Kind,
		Documentation:   symbolInfo.Documentation,
		Signature:       signature,
		Relationships:   []scip.SCIPRelationship{}, // Not available in simplified interface
		Occurrences:     filteredOccurrences,
		OccurrenceCount: len(filteredOccurrences),
		DefinitionCount: definitionCount,
		ReferenceCount:  referenceCount,
		FileCount:       len(fileSet),
		Metadata: &SearchMetadata{
			CacheEnabled:    true,
			SCIPEnabled:     true,
			SymbolsFound:    len(symbolInfos),
			FilteredResults: len(filteredOccurrences),
			FilePattern:     filePattern,
		},
		Timestamp: time.Now(),
	}, nil
}
