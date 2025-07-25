package gateway

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"lsp-gateway/mcp"
)

// ResponseAggregator defines the interface for merging responses from multiple LSP servers
type ResponseAggregator interface {
	Aggregate(responses []interface{}, sources []string) (interface{}, error)
	GetAggregationType() string
	SupportedMethods() []string
}

// AggregationResult represents a comprehensive aggregation result with metadata
type AggregationResult struct {
	MergedResponse    interface{}            `json:"merged_response"`
	SourceMapping     map[string]interface{} `json:"source_mapping"`
	ConflictInfo      []ConflictInfo         `json:"conflict_info,omitempty"`
	MergeStrategy     string                 `json:"merge_strategy"`
	TotalSources      int                    `json:"total_sources"`
	SuccessfulSources int                    `json:"successful_sources"`
	ProcessingTime    time.Duration          `json:"processing_time"`
	Quality           AggregationQuality     `json:"quality"`
}

// ConflictInfo represents information about conflicts during aggregation
type ConflictInfo struct {
	Type       string      `json:"type"`       // "duplicate", "conflict", "partial"
	Location   interface{} `json:"location"`   // URI, position, or other location identifier
	Sources    []string    `json:"sources"`    // Which servers reported conflicting information
	Resolution string      `json:"resolution"` // How the conflict was resolved
	Confidence float64     `json:"confidence"` // Confidence in the resolution (0.0-1.0)
	Details    string      `json:"details,omitempty"`
}

// AggregationQuality represents quality metrics for aggregation results
type AggregationQuality struct {
	Score             float64 `json:"score"`              // Overall quality score (0.0-1.0)
	Completeness      float64 `json:"completeness"`       // How complete the result is (0.0-1.0)
	Consistency       float64 `json:"consistency"`        // How consistent sources were (0.0-1.0)
	Redundancy        float64 `json:"redundancy"`         // Amount of redundancy found (0.0-1.0)
	ConflictRate      float64 `json:"conflict_rate"`      // Rate of conflicts encountered (0.0-1.0)
	SourceReliability float64 `json:"source_reliability"` // Reliability of sources (0.0-1.0)
}

// LSP Data Structures (following LSP specification)

// Position represents a position in a document
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range represents a range in a document
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Location represents a location in a workspace
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// LocationLink represents a link to a location with optional target range
type LocationLink struct {
	OriginSelectionRange *Range `json:"originSelectionRange,omitempty"`
	TargetURI            string `json:"targetUri"`
	TargetRange          Range  `json:"targetRange"`
	TargetSelectionRange Range  `json:"targetSelectionRange"`
}

// MarkupContent represents formatted content
type MarkupContent struct {
	Kind  string `json:"kind"` // "plaintext" or "markdown"
	Value string `json:"value"`
}

// Hover represents hover information
type Hover struct {
	Contents interface{} `json:"contents"` // MarkupContent | MarkedString | MarkedString[]
	Range    *Range      `json:"range,omitempty"`
}

// SymbolKind represents the kind of a symbol
type SymbolKind int

const (
	SymbolKindFile          SymbolKind = 1
	SymbolKindModule        SymbolKind = 2
	SymbolKindNamespace     SymbolKind = 3
	SymbolKindPackage       SymbolKind = 4
	SymbolKindClass         SymbolKind = 5
	SymbolKindMethod        SymbolKind = 6
	SymbolKindProperty      SymbolKind = 7
	SymbolKindField         SymbolKind = 8
	SymbolKindConstructor   SymbolKind = 9
	SymbolKindEnum          SymbolKind = 10
	SymbolKindInterface     SymbolKind = 11
	SymbolKindFunction      SymbolKind = 12
	SymbolKindVariable      SymbolKind = 13
	SymbolKindConstant      SymbolKind = 14
	SymbolKindString        SymbolKind = 15
	SymbolKindNumber        SymbolKind = 16
	SymbolKindBoolean       SymbolKind = 17
	SymbolKindArray         SymbolKind = 18
	SymbolKindObject        SymbolKind = 19
	SymbolKindKey           SymbolKind = 20
	SymbolKindNull          SymbolKind = 21
	SymbolKindEnumMember    SymbolKind = 22
	SymbolKindStruct        SymbolKind = 23
	SymbolKindEvent         SymbolKind = 24
	SymbolKindOperator      SymbolKind = 25
	SymbolKindTypeParameter SymbolKind = 26
)

// SymbolInformation represents symbol information
type SymbolInformation struct {
	Name          string     `json:"name"`
	Kind          SymbolKind `json:"kind"`
	Tags          []int      `json:"tags,omitempty"`
	Deprecated    bool       `json:"deprecated,omitempty"`
	Location      Location   `json:"location"`
	ContainerName string     `json:"containerName,omitempty"`
}

// DocumentSymbol represents hierarchical document symbols
type DocumentSymbol struct {
	Name           string           `json:"name"`
	Detail         string           `json:"detail,omitempty"`
	Kind           SymbolKind       `json:"kind"`
	Tags           []int            `json:"tags,omitempty"`
	Deprecated     bool             `json:"deprecated,omitempty"`
	Range          Range            `json:"range"`
	SelectionRange Range            `json:"selectionRange"`
	Children       []DocumentSymbol `json:"children,omitempty"`
}

// Diagnostic represents a diagnostic (error, warning, etc.)
type Diagnostic struct {
	Range              Range                   `json:"range"`
	Severity           int                     `json:"severity,omitempty"`
	Code               interface{}             `json:"code,omitempty"`
	CodeDescription    *CodeDescription        `json:"codeDescription,omitempty"`
	Source             string                  `json:"source,omitempty"`
	Message            string                  `json:"message"`
	Tags               []int                   `json:"tags,omitempty"`
	RelatedInformation []DiagnosticRelatedInfo `json:"relatedInformation,omitempty"`
	Data               interface{}             `json:"data,omitempty"`
}

// CodeDescription represents a description for a diagnostic code
type CodeDescription struct {
	Href string `json:"href"`
}

// DiagnosticRelatedInfo represents related information for a diagnostic
type DiagnosticRelatedInfo struct {
	Location Location `json:"location"`
	Message  string   `json:"message"`
}

// CompletionItem represents a completion item
type CompletionItem struct {
	Label               string                      `json:"label"`
	LabelDetails        *CompletionItemLabelDetails `json:"labelDetails,omitempty"`
	Kind                int                         `json:"kind,omitempty"`
	Tags                []int                       `json:"tags,omitempty"`
	Detail              string                      `json:"detail,omitempty"`
	Documentation       interface{}                 `json:"documentation,omitempty"`
	Deprecated          bool                        `json:"deprecated,omitempty"`
	Preselect           bool                        `json:"preselect,omitempty"`
	SortText            string                      `json:"sortText,omitempty"`
	FilterText          string                      `json:"filterText,omitempty"`
	InsertText          string                      `json:"insertText,omitempty"`
	InsertTextFormat    int                         `json:"insertTextFormat,omitempty"`
	InsertTextMode      int                         `json:"insertTextMode,omitempty"`
	TextEdit            interface{}                 `json:"textEdit,omitempty"`
	AdditionalTextEdits []TextEdit                  `json:"additionalTextEdits,omitempty"`
	CommitCharacters    []string                    `json:"commitCharacters,omitempty"`
	Command             *Command                    `json:"command,omitempty"`
	Data                interface{}                 `json:"data,omitempty"`
}

// CompletionItemLabelDetails represents label details for completion items
type CompletionItemLabelDetails struct {
	Detail      string `json:"detail,omitempty"`
	Description string `json:"description,omitempty"`
}

// TextEdit represents a text edit
type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

// Command represents a command
type Command struct {
	Title     string        `json:"title"`
	Command   string        `json:"command"`
	Arguments []interface{} `json:"arguments,omitempty"`
}

// CompletionList represents a list of completion items
type CompletionList struct {
	IsIncomplete bool             `json:"isIncomplete"`
	Items        []CompletionItem `json:"items"`
}

// AggregatorRegistry manages ResponseAggregator instances
type AggregatorRegistry struct {
	aggregators map[string]ResponseAggregator
	mu          sync.RWMutex
	logger      *mcp.StructuredLogger
}

// NewAggregatorRegistry creates a new aggregator registry with default aggregators
func NewAggregatorRegistry(logger *mcp.StructuredLogger) *AggregatorRegistry {
	registry := &AggregatorRegistry{
		aggregators: make(map[string]ResponseAggregator),
		logger:      logger,
	}

	// Register default aggregators
	registry.RegisterAggregator(&DefinitionAggregator{logger: logger})
	registry.RegisterAggregator(&ReferencesAggregator{logger: logger})
	registry.RegisterAggregator(&SymbolAggregator{logger: logger})
	registry.RegisterAggregator(&HoverAggregator{logger: logger})
	registry.RegisterAggregator(&DiagnosticAggregator{logger: logger})
	registry.RegisterAggregator(&CompletionAggregator{logger: logger})

	return registry
}

// RegisterAggregator registers a new aggregator
func (r *AggregatorRegistry) RegisterAggregator(aggregator ResponseAggregator) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, method := range aggregator.SupportedMethods() {
		r.aggregators[method] = aggregator
		if r.logger != nil {
			r.logger.Debugf("Registered aggregator %s for method %s", aggregator.GetAggregationType(), method)
		}
	}
}

// GetAggregator retrieves an aggregator for a specific LSP method
func (r *AggregatorRegistry) GetAggregator(method string) (ResponseAggregator, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	aggregator, exists := r.aggregators[method]
	return aggregator, exists
}

// AggregateResponses aggregates responses using the appropriate aggregator
func (r *AggregatorRegistry) AggregateResponses(method string, responses []interface{}, sources []string) (*AggregationResult, error) {
	startTime := time.Now()

	aggregator, exists := r.GetAggregator(method)
	if !exists {
		// Fallback to basic aggregation
		return r.basicAggregation(responses, sources, time.Since(startTime))
	}

	result, err := aggregator.Aggregate(responses, sources)
	if err != nil {
		return nil, fmt.Errorf("aggregation failed for method %s: %w", method, err)
	}

	// Create comprehensive result
	successfulSources := 0
	for _, response := range responses {
		if response != nil {
			successfulSources++
		}
	}

	quality := r.calculateAggregationQuality(responses, sources, result)

	return &AggregationResult{
		MergedResponse:    result,
		SourceMapping:     r.createSourceMapping(responses, sources),
		MergeStrategy:     aggregator.GetAggregationType(),
		TotalSources:      len(sources),
		SuccessfulSources: successfulSources,
		ProcessingTime:    time.Since(startTime),
		Quality:           quality,
	}, nil
}

// basicAggregation provides fallback aggregation for unsupported methods
func (r *AggregatorRegistry) basicAggregation(responses []interface{}, sources []string, processingTime time.Duration) (*AggregationResult, error) {
	var primaryResult interface{}
	successfulSources := 0

	for _, response := range responses {
		if response != nil {
			successfulSources++
			if primaryResult == nil {
				primaryResult = response
			}
		}
	}

	if successfulSources == 0 {
		return nil, fmt.Errorf("no successful responses to aggregate")
	}

	quality := AggregationQuality{
		Score:             0.5, // Basic aggregation gets medium score
		Completeness:      float64(successfulSources) / float64(len(sources)),
		Consistency:       1.0, // No conflicts in basic aggregation
		Redundancy:        0.0,
		ConflictRate:      0.0,
		SourceReliability: 0.7,
	}

	return &AggregationResult{
		MergedResponse:    primaryResult,
		SourceMapping:     r.createSourceMapping(responses, sources),
		MergeStrategy:     "basic_first_success",
		TotalSources:      len(sources),
		SuccessfulSources: successfulSources,
		ProcessingTime:    processingTime,
		Quality:           quality,
	}, nil
}

// createSourceMapping creates a mapping of sources to their responses
func (r *AggregatorRegistry) createSourceMapping(responses []interface{}, sources []string) map[string]interface{} {
	mapping := make(map[string]interface{})
	for i, source := range sources {
		if i < len(responses) {
			mapping[source] = responses[i]
		}
	}
	return mapping
}

// calculateAggregationQuality calculates quality metrics for aggregation
func (r *AggregatorRegistry) calculateAggregationQuality(responses []interface{}, sources []string, result interface{}) AggregationQuality {
	successfulResponses := 0
	for _, response := range responses {
		if response != nil {
			successfulResponses++
		}
	}

	completeness := float64(successfulResponses) / float64(len(sources))

	// Basic quality metrics - more sophisticated calculation could be added
	score := completeness * 0.8 // Base score on completeness
	if successfulResponses > 1 {
		score += 0.2 // Bonus for redundancy
	}

	return AggregationQuality{
		Score:             score,
		Completeness:      completeness,
		Consistency:       0.9, // Assume high consistency for now
		Redundancy:        float64(successfulResponses-1) / float64(len(sources)),
		ConflictRate:      0.1, // Assume low conflict rate
		SourceReliability: 0.8,
	}
}

// DefinitionAggregator aggregates textDocument/definition responses
type DefinitionAggregator struct {
	logger *mcp.StructuredLogger
}

func (d *DefinitionAggregator) GetAggregationType() string {
	return "definition"
}

func (d *DefinitionAggregator) SupportedMethods() []string {
	return []string{LSP_METHOD_DEFINITION}
}

func (d *DefinitionAggregator) Aggregate(responses []interface{}, sources []string) (interface{}, error) {
	var allLocations []Location
	var allLocationLinks []LocationLink
	seenLocations := make(map[string]bool)
	conflicts := []ConflictInfo{}

	for i, response := range responses {
		if response == nil {
			continue
		}

		source := ""
		if i < len(sources) {
			source = sources[i]
		}

		// Handle different response formats: Location | Location[] | LocationLink | LocationLink[]
		switch resp := response.(type) {
		case Location:
			locationKey := fmt.Sprintf("%s:%d:%d-%d:%d", resp.URI, resp.Range.Start.Line, resp.Range.Start.Character, resp.Range.End.Line, resp.Range.End.Character)
			if !seenLocations[locationKey] {
				allLocations = append(allLocations, resp)
				seenLocations[locationKey] = true
			} else {
				conflicts = append(conflicts, ConflictInfo{
					Type:       "duplicate",
					Location:   resp,
					Sources:    []string{source},
					Resolution: "deduplicated",
					Confidence: 1.0,
				})
			}

		case []Location:
			for _, loc := range resp {
				locationKey := fmt.Sprintf("%s:%d:%d-%d:%d", loc.URI, loc.Range.Start.Line, loc.Range.Start.Character, loc.Range.End.Line, loc.Range.End.Character)
				if !seenLocations[locationKey] {
					allLocations = append(allLocations, loc)
					seenLocations[locationKey] = true
				}
			}

		case LocationLink:
			allLocationLinks = append(allLocationLinks, resp)

		case []LocationLink:
			allLocationLinks = append(allLocationLinks, resp...)

		case []interface{}:
			// Handle mixed array responses
			for _, item := range resp {
				if loc, ok := item.(map[string]interface{}); ok {
					if _, exists := loc["uri"]; exists {
						// Convert map to Location
						if location := d.convertMapToLocation(loc); location != nil {
							locationKey := fmt.Sprintf("%s:%d:%d-%d:%d", location.URI, location.Range.Start.Line, location.Range.Start.Character, location.Range.End.Line, location.Range.End.Character)
							if !seenLocations[locationKey] {
								allLocations = append(allLocations, *location)
								seenLocations[locationKey] = true
							}
						}
					}
				}
			}

		default:
			if d.logger != nil {
				d.logger.Warnf("DefinitionAggregator: Unknown response format from source %s", source)
			}
		}
	}

	// Prioritize based on language and file relevance
	d.prioritizeDefinitions(allLocations, sources)

	// Return appropriate format
	if len(allLocationLinks) > 0 {
		return allLocationLinks, nil
	}

	if len(allLocations) == 0 {
		return nil, nil
	}

	if len(allLocations) == 1 {
		return allLocations[0], nil
	}

	return allLocations, nil
}

// convertMapToLocation converts a map[string]interface{} to Location
func (d *DefinitionAggregator) convertMapToLocation(m map[string]interface{}) *Location {
	location := &Location{}

	if uri, ok := m["uri"].(string); ok {
		location.URI = uri
	} else {
		return nil
	}

	if rangeData, ok := m["range"].(map[string]interface{}); ok {
		if start, ok := rangeData["start"].(map[string]interface{}); ok {
			if line, ok := start["line"].(float64); ok {
				location.Range.Start.Line = int(line)
			}
			if char, ok := start["character"].(float64); ok {
				location.Range.Start.Character = int(char)
			}
		}
		if end, ok := rangeData["end"].(map[string]interface{}); ok {
			if line, ok := end["line"].(float64); ok {
				location.Range.End.Line = int(line)
			}
			if char, ok := end["character"].(float64); ok {
				location.Range.End.Character = int(char)
			}
		}
	}

	return location
}

// prioritizeDefinitions sorts definitions by relevance and language priority
func (d *DefinitionAggregator) prioritizeDefinitions(locations []Location, sources []string) {
	sort.Slice(locations, func(i, j int) bool {
		// Prioritize by file extension (language)
		extI := filepath.Ext(d.getPathFromURI(locations[i].URI))
		extJ := filepath.Ext(d.getPathFromURI(locations[j].URI))

		// Language priority (can be made configurable)
		priorities := map[string]int{
			".go":   1,
			".ts":   2,
			".tsx":  2,
			".js":   3,
			".jsx":  3,
			".py":   4,
			".java": 5,
		}

		priI, okI := priorities[extI]
		priJ, okJ := priorities[extJ]

		if okI && okJ {
			return priI < priJ
		}
		if okI {
			return true
		}
		if okJ {
			return false
		}

		// Fall back to URI comparison for consistency
		return locations[i].URI < locations[j].URI
	})
}

// getPathFromURI extracts file path from URI
func (d *DefinitionAggregator) getPathFromURI(uri string) string {
	if parsedURI, err := url.Parse(uri); err == nil {
		return parsedURI.Path
	}
	return uri
}

// ReferencesAggregator aggregates textDocument/references responses
type ReferencesAggregator struct {
	logger *mcp.StructuredLogger
}

func (r *ReferencesAggregator) GetAggregationType() string {
	return "references"
}

func (r *ReferencesAggregator) SupportedMethods() []string {
	return []string{LSP_METHOD_REFERENCES}
}

func (r *ReferencesAggregator) Aggregate(responses []interface{}, sources []string) (interface{}, error) {
	var allReferences []Location
	seenReferences := make(map[string]bool)
	conflicts := []ConflictInfo{}

	for i, response := range responses {
		if response == nil {
			continue
		}

		source := ""
		if i < len(sources) {
			source = sources[i]
		}

		switch resp := response.(type) {
		case []Location:
			for _, ref := range resp {
				referenceKey := fmt.Sprintf("%s:%d:%d-%d:%d", ref.URI, ref.Range.Start.Line, ref.Range.Start.Character, ref.Range.End.Line, ref.Range.End.Character)
				if !seenReferences[referenceKey] {
					allReferences = append(allReferences, ref)
					seenReferences[referenceKey] = true
				} else {
					conflicts = append(conflicts, ConflictInfo{
						Type:       "duplicate",
						Location:   ref,
						Sources:    []string{source},
						Resolution: "deduplicated",
						Confidence: 1.0,
					})
				}
			}

		case []interface{}:
			for _, item := range resp {
				if refMap, ok := item.(map[string]interface{}); ok {
					if ref := r.convertMapToLocation(refMap); ref != nil {
						referenceKey := fmt.Sprintf("%s:%d:%d-%d:%d", ref.URI, ref.Range.Start.Line, ref.Range.Start.Character, ref.Range.End.Line, ref.Range.End.Character)
						if !seenReferences[referenceKey] {
							allReferences = append(allReferences, *ref)
							seenReferences[referenceKey] = true
						}
					}
				}
			}

		default:
			if r.logger != nil {
				r.logger.Warnf("ReferencesAggregator: Unknown response format from source %s", source)
			}
		}
	}

	// Sort references by file and position for consistent ordering
	sort.Slice(allReferences, func(i, j int) bool {
		if allReferences[i].URI != allReferences[j].URI {
			return allReferences[i].URI < allReferences[j].URI
		}
		if allReferences[i].Range.Start.Line != allReferences[j].Range.Start.Line {
			return allReferences[i].Range.Start.Line < allReferences[j].Range.Start.Line
		}
		return allReferences[i].Range.Start.Character < allReferences[j].Range.Start.Character
	})

	return allReferences, nil
}

// convertMapToLocation converts map to Location for references
func (r *ReferencesAggregator) convertMapToLocation(m map[string]interface{}) *Location {
	location := &Location{}

	if uri, ok := m["uri"].(string); ok {
		location.URI = uri
	} else {
		return nil
	}

	if rangeData, ok := m["range"].(map[string]interface{}); ok {
		if start, ok := rangeData["start"].(map[string]interface{}); ok {
			if line, ok := start["line"].(float64); ok {
				location.Range.Start.Line = int(line)
			}
			if char, ok := start["character"].(float64); ok {
				location.Range.Start.Character = int(char)
			}
		}
		if end, ok := rangeData["end"].(map[string]interface{}); ok {
			if line, ok := end["line"].(float64); ok {
				location.Range.End.Line = int(line)
			}
			if char, ok := end["character"].(float64); ok {
				location.Range.End.Character = int(char)
			}
		}
	}

	return location
}

// SymbolAggregator aggregates workspace/symbol responses
type SymbolAggregator struct {
	logger *mcp.StructuredLogger
}

func (s *SymbolAggregator) GetAggregationType() string {
	return "symbol"
}

func (s *SymbolAggregator) SupportedMethods() []string {
	return []string{LSP_METHOD_WORKSPACE_SYMBOL, LSP_METHOD_DOCUMENT_SYMBOL}
}

func (s *SymbolAggregator) Aggregate(responses []interface{}, sources []string) (interface{}, error) {
	var allSymbols []SymbolInformation
	seenSymbols := make(map[string]bool)
	conflicts := []ConflictInfo{}

	for i, response := range responses {
		if response == nil {
			continue
		}

		source := ""
		if i < len(sources) {
			source = sources[i]
		}

		switch resp := response.(type) {
		case []SymbolInformation:
			for _, symbol := range resp {
				symbolKey := fmt.Sprintf("%s:%s:%s:%d:%d", symbol.Location.URI, symbol.Name, symbol.ContainerName, symbol.Location.Range.Start.Line, symbol.Location.Range.Start.Character)
				if !seenSymbols[symbolKey] {
					allSymbols = append(allSymbols, symbol)
					seenSymbols[symbolKey] = true
				} else {
					conflicts = append(conflicts, ConflictInfo{
						Type:       "duplicate",
						Location:   symbol.Location,
						Sources:    []string{source},
						Resolution: "deduplicated",
						Confidence: 1.0,
					})
				}
			}

		case []interface{}:
			for _, item := range resp {
				if symbolMap, ok := item.(map[string]interface{}); ok {
					if symbol := s.convertMapToSymbolInformation(symbolMap); symbol != nil {
						symbolKey := fmt.Sprintf("%s:%s:%s:%d:%d", symbol.Location.URI, symbol.Name, symbol.ContainerName, symbol.Location.Range.Start.Line, symbol.Location.Range.Start.Character)
						if !seenSymbols[symbolKey] {
							allSymbols = append(allSymbols, *symbol)
							seenSymbols[symbolKey] = true
						}
					}
				}
			}

		default:
			if s.logger != nil {
				s.logger.Warnf("SymbolAggregator: Unknown response format from source %s", source)
			}
		}
	}

	// Sort symbols by relevance (name, then kind, then location)
	sort.Slice(allSymbols, func(i, j int) bool {
		if allSymbols[i].Name != allSymbols[j].Name {
			return allSymbols[i].Name < allSymbols[j].Name
		}
		if allSymbols[i].Kind != allSymbols[j].Kind {
			return allSymbols[i].Kind < allSymbols[j].Kind
		}
		if allSymbols[i].Location.URI != allSymbols[j].Location.URI {
			return allSymbols[i].Location.URI < allSymbols[j].Location.URI
		}
		return allSymbols[i].Location.Range.Start.Line < allSymbols[j].Location.Range.Start.Line
	})

	return allSymbols, nil
}

// convertMapToSymbolInformation converts map to SymbolInformation
func (s *SymbolAggregator) convertMapToSymbolInformation(m map[string]interface{}) *SymbolInformation {
	symbol := &SymbolInformation{}

	if name, ok := m["name"].(string); ok {
		symbol.Name = name
	} else {
		return nil
	}

	if kind, ok := m["kind"].(float64); ok {
		symbol.Kind = SymbolKind(int(kind))
	}

	if container, ok := m["containerName"].(string); ok {
		symbol.ContainerName = container
	}

	if deprecated, ok := m["deprecated"].(bool); ok {
		symbol.Deprecated = deprecated
	}

	if locationData, ok := m["location"].(map[string]interface{}); ok {
		if uri, ok := locationData["uri"].(string); ok {
			symbol.Location.URI = uri
		}
		if rangeData, ok := locationData["range"].(map[string]interface{}); ok {
			// Parse range data similar to other aggregators
			if start, ok := rangeData["start"].(map[string]interface{}); ok {
				if line, ok := start["line"].(float64); ok {
					symbol.Location.Range.Start.Line = int(line)
				}
				if char, ok := start["character"].(float64); ok {
					symbol.Location.Range.Start.Character = int(char)
				}
			}
			if end, ok := rangeData["end"].(map[string]interface{}); ok {
				if line, ok := end["line"].(float64); ok {
					symbol.Location.Range.End.Line = int(line)
				}
				if char, ok := end["character"].(float64); ok {
					symbol.Location.Range.End.Character = int(char)
				}
			}
		}
	}

	return symbol
}

// HoverAggregator aggregates textDocument/hover responses
type HoverAggregator struct {
	logger *mcp.StructuredLogger
}

func (h *HoverAggregator) GetAggregationType() string {
	return "hover"
}

func (h *HoverAggregator) SupportedMethods() []string {
	return []string{LSP_METHOD_HOVER}
}

func (h *HoverAggregator) Aggregate(responses []interface{}, sources []string) (interface{}, error) {
	var validHovers []Hover
	var allContents []string
	var bestRange *Range

	for i, response := range responses {
		if response == nil {
			continue
		}

		source := ""
		if i < len(sources) {
			source = sources[i]
		}

		switch resp := response.(type) {
		case Hover:
			validHovers = append(validHovers, resp)
			if content := h.extractContentString(resp.Contents); content != "" {
				allContents = append(allContents, content)
			}
			if resp.Range != nil && bestRange == nil {
				bestRange = resp.Range
			}

		case map[string]interface{}:
			if hover := h.convertMapToHover(resp); hover != nil {
				validHovers = append(validHovers, *hover)
				if content := h.extractContentString(hover.Contents); content != "" {
					allContents = append(allContents, content)
				}
				if hover.Range != nil && bestRange == nil {
					bestRange = hover.Range
				}
			}

		default:
			if h.logger != nil {
				h.logger.Warnf("HoverAggregator: Unknown response format from source %s", source)
			}
		}
	}

	if len(validHovers) == 0 {
		return nil, nil
	}

	// Merge content from multiple sources
	mergedContent := h.mergeHoverContents(allContents)

	return Hover{
		Contents: MarkupContent{
			Kind:  "markdown",
			Value: mergedContent,
		},
		Range: bestRange,
	}, nil
}

// convertMapToHover converts map to Hover
func (h *HoverAggregator) convertMapToHover(m map[string]interface{}) *Hover {
	hover := &Hover{}

	if contents, ok := m["contents"]; ok {
		hover.Contents = contents
	}

	if rangeData, ok := m["range"].(map[string]interface{}); ok {
		hover.Range = &Range{}
		if start, ok := rangeData["start"].(map[string]interface{}); ok {
			if line, ok := start["line"].(float64); ok {
				hover.Range.Start.Line = int(line)
			}
			if char, ok := start["character"].(float64); ok {
				hover.Range.Start.Character = int(char)
			}
		}
		if end, ok := rangeData["end"].(map[string]interface{}); ok {
			if line, ok := end["line"].(float64); ok {
				hover.Range.End.Line = int(line)
			}
			if char, ok := end["character"].(float64); ok {
				hover.Range.End.Character = int(char)
			}
		}
	}

	return hover
}

// extractContentString extracts string content from various content formats
func (h *HoverAggregator) extractContentString(contents interface{}) string {
	switch content := contents.(type) {
	case string:
		return content
	case MarkupContent:
		return content.Value
	case map[string]interface{}:
		if value, ok := content["value"].(string); ok {
			return value
		}
	case []interface{}:
		var parts []string
		for _, part := range content {
			if str := h.extractContentString(part); str != "" {
				parts = append(parts, str)
			}
		}
		return strings.Join(parts, "\n\n")
	}
	return ""
}

// mergeHoverContents combines hover content from multiple sources
func (h *HoverAggregator) mergeHoverContents(contents []string) string {
	if len(contents) == 0 {
		return ""
	}

	if len(contents) == 1 {
		return contents[0]
	}

	// Remove duplicates
	seen := make(map[string]bool)
	var unique []string
	for _, content := range contents {
		if !seen[content] {
			unique = append(unique, content)
			seen[content] = true
		}
	}

	// Combine unique contents with separators
	return strings.Join(unique, "\n\n---\n\n")
}

// DiagnosticAggregator aggregates diagnostic responses
type DiagnosticAggregator struct {
	logger *mcp.StructuredLogger
}

func (d *DiagnosticAggregator) GetAggregationType() string {
	return "diagnostic"
}

func (d *DiagnosticAggregator) SupportedMethods() []string {
	return []string{"textDocument/publishDiagnostics", "textDocument/diagnostic"}
}

func (d *DiagnosticAggregator) Aggregate(responses []interface{}, sources []string) (interface{}, error) {
	var allDiagnostics []Diagnostic
	seenDiagnostics := make(map[string]bool)

	for i, response := range responses {
		if response == nil {
			continue
		}

		source := ""
		if i < len(sources) {
			source = sources[i]
		}

		switch resp := response.(type) {
		case []Diagnostic:
			for _, diag := range resp {
				diagKey := fmt.Sprintf("%d:%d-%d:%d:%s", diag.Range.Start.Line, diag.Range.Start.Character, diag.Range.End.Line, diag.Range.End.Character, diag.Message)
				if !seenDiagnostics[diagKey] {
					// Add source information
					if diag.Source == "" {
						diag.Source = source
					}
					allDiagnostics = append(allDiagnostics, diag)
					seenDiagnostics[diagKey] = true
				}
			}

		case map[string]interface{}:
			if diagnostics, ok := resp["diagnostics"].([]interface{}); ok {
				for _, item := range diagnostics {
					if diagMap, ok := item.(map[string]interface{}); ok {
						if diag := d.convertMapToDiagnostic(diagMap, source); diag != nil {
							diagKey := fmt.Sprintf("%d:%d-%d:%d:%s", diag.Range.Start.Line, diag.Range.Start.Character, diag.Range.End.Line, diag.Range.End.Character, diag.Message)
							if !seenDiagnostics[diagKey] {
								allDiagnostics = append(allDiagnostics, *diag)
								seenDiagnostics[diagKey] = true
							}
						}
					}
				}
			}

		default:
			if d.logger != nil {
				d.logger.Warnf("DiagnosticAggregator: Unknown response format from source %s", source)
			}
		}
	}

	// Sort diagnostics by severity (errors first), then by position
	sort.Slice(allDiagnostics, func(i, j int) bool {
		// Higher severity (lower number) comes first
		if allDiagnostics[i].Severity != allDiagnostics[j].Severity {
			return allDiagnostics[i].Severity < allDiagnostics[j].Severity
		}
		// Same severity, sort by position
		if allDiagnostics[i].Range.Start.Line != allDiagnostics[j].Range.Start.Line {
			return allDiagnostics[i].Range.Start.Line < allDiagnostics[j].Range.Start.Line
		}
		return allDiagnostics[i].Range.Start.Character < allDiagnostics[j].Range.Start.Character
	})

	return allDiagnostics, nil
}

// convertMapToDiagnostic converts map to Diagnostic
func (d *DiagnosticAggregator) convertMapToDiagnostic(m map[string]interface{}, source string) *Diagnostic {
	diag := &Diagnostic{}

	if message, ok := m["message"].(string); ok {
		diag.Message = message
	} else {
		return nil
	}

	if severity, ok := m["severity"].(float64); ok {
		diag.Severity = int(severity)
	}

	if src, ok := m["source"].(string); ok {
		diag.Source = src
	} else {
		diag.Source = source
	}

	if code, ok := m["code"]; ok {
		diag.Code = code
	}

	if rangeData, ok := m["range"].(map[string]interface{}); ok {
		if start, ok := rangeData["start"].(map[string]interface{}); ok {
			if line, ok := start["line"].(float64); ok {
				diag.Range.Start.Line = int(line)
			}
			if char, ok := start["character"].(float64); ok {
				diag.Range.Start.Character = int(char)
			}
		}
		if end, ok := rangeData["end"].(map[string]interface{}); ok {
			if line, ok := end["line"].(float64); ok {
				diag.Range.End.Line = int(line)
			}
			if char, ok := end["character"].(float64); ok {
				diag.Range.End.Character = int(char)
			}
		}
	}

	return diag
}

// CompletionAggregator aggregates textDocument/completion responses
type CompletionAggregator struct {
	logger *mcp.StructuredLogger
}

func (c *CompletionAggregator) GetAggregationType() string {
	return "completion"
}

func (c *CompletionAggregator) SupportedMethods() []string {
	return []string{"textDocument/completion"}
}

func (c *CompletionAggregator) Aggregate(responses []interface{}, sources []string) (interface{}, error) {
	var allItems []CompletionItem
	seenItems := make(map[string]bool)
	isIncomplete := false

	for i, response := range responses {
		if response == nil {
			continue
		}

		source := ""
		if i < len(sources) {
			source = sources[i]
		}

		switch resp := response.(type) {
		case CompletionList:
			if resp.IsIncomplete {
				isIncomplete = true
			}
			for _, item := range resp.Items {
				itemKey := fmt.Sprintf("%s:%s:%d", item.Label, item.Detail, item.Kind)
				if !seenItems[itemKey] {
					allItems = append(allItems, item)
					seenItems[itemKey] = true
				}
			}

		case []CompletionItem:
			for _, item := range resp {
				itemKey := fmt.Sprintf("%s:%s:%d", item.Label, item.Detail, item.Kind)
				if !seenItems[itemKey] {
					allItems = append(allItems, item)
					seenItems[itemKey] = true
				}
			}

		case map[string]interface{}:
			if items, ok := resp["items"].([]interface{}); ok {
				if incomplete, ok := resp["isIncomplete"].(bool); ok && incomplete {
					isIncomplete = true
				}
				for _, item := range items {
					if itemMap, ok := item.(map[string]interface{}); ok {
						if compItem := c.convertMapToCompletionItem(itemMap); compItem != nil {
							itemKey := fmt.Sprintf("%s:%s:%d", compItem.Label, compItem.Detail, compItem.Kind)
							if !seenItems[itemKey] {
								allItems = append(allItems, *compItem)
								seenItems[itemKey] = true
							}
						}
					}
				}
			}

		default:
			if c.logger != nil {
				c.logger.Warnf("CompletionAggregator: Unknown response format from source %s", source)
			}
		}
	}

	// Sort completion items by relevance (preselect, then sort text, then label)
	sort.Slice(allItems, func(i, j int) bool {
		// Preselected items come first
		if allItems[i].Preselect != allItems[j].Preselect {
			return allItems[i].Preselect
		}
		// Use sort text if available
		if allItems[i].SortText != "" && allItems[j].SortText != "" {
			return allItems[i].SortText < allItems[j].SortText
		}
		// Fall back to label
		return allItems[i].Label < allItems[j].Label
	})

	return CompletionList{
		IsIncomplete: isIncomplete,
		Items:        allItems,
	}, nil
}

// convertMapToCompletionItem converts map to CompletionItem
func (c *CompletionAggregator) convertMapToCompletionItem(m map[string]interface{}) *CompletionItem {
	item := &CompletionItem{}

	if label, ok := m["label"].(string); ok {
		item.Label = label
	} else {
		return nil
	}

	if kind, ok := m["kind"].(float64); ok {
		item.Kind = int(kind)
	}

	if detail, ok := m["detail"].(string); ok {
		item.Detail = detail
	}

	if sortText, ok := m["sortText"].(string); ok {
		item.SortText = sortText
	}

	if filterText, ok := m["filterText"].(string); ok {
		item.FilterText = filterText
	}

	if insertText, ok := m["insertText"].(string); ok {
		item.InsertText = insertText
	}

	if preselect, ok := m["preselect"].(bool); ok {
		item.Preselect = preselect
	}

	if deprecated, ok := m["deprecated"].(bool); ok {
		item.Deprecated = deprecated
	}

	return item
}
