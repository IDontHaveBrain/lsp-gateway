package indexing

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"
)

// LSPSCIPMapper translates LSP requests into SCIP index queries
type LSPSCIPMapper struct {
	scipStore    SCIPStore
	config       *SCIPConfig
	statsTracker *MapperStats
	mutex        sync.RWMutex
	
	// Performance tracking
	requestCount     int64
	successCount     int64
	errorCount       int64
	avgResponseTime  time.Duration
	lastRequestTime  time.Time
}

// LSPParams represents parameters for LSP method calls
type LSPParams struct {
	Method     string                 `json:"method"`
	URI        string                 `json:"uri,omitempty"`
	Position   *LSPPosition           `json:"position,omitempty"`
	Query      string                 `json:"query,omitempty"`
	Context    map[string]interface{} `json:"context,omitempty"`
	TextDocument *TextDocumentIdentifier `json:"textDocument,omitempty"`
}

// LSPPosition represents a position in a text document
type LSPPosition struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// LSPRange represents a range in a text document
type LSPRange struct {
	Start LSPPosition `json:"start"`
	End   LSPPosition `json:"end"`
}

// LSPLocation represents a location reference
type LSPLocation struct {
	URI   string   `json:"uri"`
	Range LSPRange `json:"range"`
}

// TextDocumentIdentifier identifies a text document
type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

// TextDocumentPositionParams represents position-based parameters
type TextDocumentPositionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     LSPPosition            `json:"position"`
}

// DocumentSymbolParams represents parameters for document symbol requests
type DocumentSymbolParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

// WorkspaceSymbolParams represents parameters for workspace symbol requests  
type WorkspaceSymbolParams struct {
	Query string `json:"query"`
}

// LSPSymbolKind represents LSP symbol kinds
type LSPSymbolKind int

const (
	LSPSymbolKindFile          LSPSymbolKind = 1
	LSPSymbolKindModule        LSPSymbolKind = 2
	LSPSymbolKindNamespace     LSPSymbolKind = 3
	LSPSymbolKindPackage       LSPSymbolKind = 4
	LSPSymbolKindClass         LSPSymbolKind = 5
	LSPSymbolKindMethod        LSPSymbolKind = 6
	LSPSymbolKindProperty      LSPSymbolKind = 7
	LSPSymbolKindField         LSPSymbolKind = 8
	LSPSymbolKindConstructor   LSPSymbolKind = 9
	LSPSymbolKindEnum          LSPSymbolKind = 10
	LSPSymbolKindInterface     LSPSymbolKind = 11
	LSPSymbolKindFunction      LSPSymbolKind = 12
	LSPSymbolKindVariable      LSPSymbolKind = 13
	LSPSymbolKindConstant      LSPSymbolKind = 14
	LSPSymbolKindString        LSPSymbolKind = 15
	LSPSymbolKindNumber        LSPSymbolKind = 16
	LSPSymbolKindBoolean       LSPSymbolKind = 17
	LSPSymbolKindArray         LSPSymbolKind = 18
	LSPSymbolKindObject        LSPSymbolKind = 19
	LSPSymbolKindKey           LSPSymbolKind = 20
	LSPSymbolKindNull          LSPSymbolKind = 21
	LSPSymbolKindEnumMember    LSPSymbolKind = 22
	LSPSymbolKindStruct        LSPSymbolKind = 23
	LSPSymbolKindEvent         LSPSymbolKind = 24
	LSPSymbolKindOperator      LSPSymbolKind = 25
	LSPSymbolKindTypeParameter LSPSymbolKind = 26
)

// DocumentSymbol represents a symbol in a document
type DocumentSymbol struct {
	Name           string           `json:"name"`
	Detail         string           `json:"detail,omitempty"`
	Kind           LSPSymbolKind    `json:"kind"`
	Deprecated     bool             `json:"deprecated,omitempty"`
	Range          LSPRange         `json:"range"`
	SelectionRange LSPRange         `json:"selectionRange"`
	Children       []DocumentSymbol `json:"children,omitempty"`
}

// SymbolInformation represents workspace symbol information
type SymbolInformation struct {
	Name          string        `json:"name"`
	Kind          LSPSymbolKind `json:"kind"`
	Deprecated    bool          `json:"deprecated,omitempty"`
	Location      LSPLocation   `json:"location"`
	ContainerName string        `json:"containerName,omitempty"`
}

// HoverContents represents hover information contents
type HoverContents struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

// Hover represents hover information
type Hover struct {
	Contents HoverContents `json:"contents"`
	Range    *LSPRange     `json:"range,omitempty"`
}

// MapperStats tracks performance statistics for the mapper
type MapperStats struct {
	TotalRequests      int64         `json:"total_requests"`
	SuccessfulRequests int64         `json:"successful_requests"`
	FailedRequests     int64         `json:"failed_requests"`
	AverageResponseTime time.Duration `json:"average_response_time"`
	LastRequestTime    time.Time     `json:"last_request_time"`
	
	// Method-specific statistics
	DefinitionRequests      int64 `json:"definition_requests"`
	ReferencesRequests      int64 `json:"references_requests"`
	HoverRequests          int64 `json:"hover_requests"`
	DocumentSymbolRequests int64 `json:"document_symbol_requests"`
	WorkspaceSymbolRequests int64 `json:"workspace_symbol_requests"`
	
	// Performance metrics
	FastQueries  int64 `json:"fast_queries"`  // < 5ms
	SlowQueries  int64 `json:"slow_queries"`  // > 50ms
	CacheHits    int64 `json:"cache_hits"`
	CacheMisses  int64 `json:"cache_misses"`
}

// NewLSPSCIPMapper creates a new LSP to SCIP query mapper
func NewLSPSCIPMapper(store SCIPStore, config *SCIPConfig) *LSPSCIPMapper {
	if config == nil {
		config = &SCIPConfig{
			CacheConfig: CacheConfig{
				Enabled: true,
				MaxSize: 10000,
				TTL:     30 * time.Minute,
			},
			Logging: LoggingConfig{
				LogQueries:         false,
				LogCacheOperations: false,
				LogIndexOperations: true,
			},
			Performance: PerformanceConfig{
				QueryTimeout:         10 * time.Second, // Enterprise target: sub-10ms
				MaxConcurrentQueries: 100,
				IndexLoadTimeout:     5 * time.Minute,
			},
		}
	}

	return &LSPSCIPMapper{
		scipStore:    store,
		config:       config,
		statsTracker: &MapperStats{},
	}
}

// MapDefinition handles textDocument/definition requests
func (m *LSPSCIPMapper) MapDefinition(params *LSPParams) (json.RawMessage, error) {
	startTime := time.Now()
	defer m.updateRequestStats("definition", startTime)

	if params.TextDocument == nil || params.Position == nil {
		return nil, fmt.Errorf("invalid parameters: textDocument and position are required")
	}

	// Build SCIP query parameters
	scipParams := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": params.TextDocument.URI,
		},
		"position": map[string]interface{}{
			"line":      params.Position.Line,
			"character": params.Position.Character,
		},
		"context": map[string]interface{}{
			"includeDeclaration": true,
		},
	}

	// Query SCIP store
	result := m.scipStore.Query("textDocument/definition", scipParams)
	
	if !result.Found {
		if result.Error != "" {
			return nil, fmt.Errorf("SCIP query failed: %s", result.Error)
		}
		// Return empty array for no results
		return json.RawMessage("[]"), nil
	}

	// Validate and return the SCIP response
	if err := m.validateLSPLocationResponse(result.Response); err != nil {
		return nil, fmt.Errorf("invalid SCIP response format: %w", err)
	}

	return result.Response, nil
}

// MapReferences handles textDocument/references requests
func (m *LSPSCIPMapper) MapReferences(params *LSPParams) (json.RawMessage, error) {
	startTime := time.Now()
	defer m.updateRequestStats("references", startTime)

	if params.TextDocument == nil || params.Position == nil {
		return nil, fmt.Errorf("invalid parameters: textDocument and position are required")
	}

	// Build SCIP query parameters
	scipParams := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": params.TextDocument.URI,
		},
		"position": map[string]interface{}{
			"line":      params.Position.Line,
			"character": params.Position.Character,
		},
		"context": map[string]interface{}{
			"includeDeclaration": false,
		},
	}

	// Query SCIP store
	result := m.scipStore.Query("textDocument/references", scipParams)
	
	if !result.Found {
		if result.Error != "" {
			return nil, fmt.Errorf("SCIP query failed: %s", result.Error)
		}
		// Return empty array for no results
		return json.RawMessage("[]"), nil
	}

	// Validate and return the SCIP response
	if err := m.validateLSPLocationArrayResponse(result.Response); err != nil {
		return nil, fmt.Errorf("invalid SCIP response format: %w", err)
	}

	return result.Response, nil
}

// MapHover handles textDocument/hover requests
func (m *LSPSCIPMapper) MapHover(params *LSPParams) (json.RawMessage, error) {
	startTime := time.Now()
	defer m.updateRequestStats("hover", startTime)

	if params.TextDocument == nil || params.Position == nil {
		return nil, fmt.Errorf("invalid parameters: textDocument and position are required")
	}

	// Build SCIP query parameters
	scipParams := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": params.TextDocument.URI,
		},
		"position": map[string]interface{}{
			"line":      params.Position.Line,
			"character": params.Position.Character,
		},
	}

	// Query SCIP store
	result := m.scipStore.Query("textDocument/hover", scipParams)
	
	if !result.Found {
		if result.Error != "" {
			return nil, fmt.Errorf("SCIP query failed: %s", result.Error)
		}
		// Return null for no hover information
		return json.RawMessage("null"), nil
	}

	// Validate and return the SCIP response
	if err := m.validateLSPHoverResponse(result.Response); err != nil {
		return nil, fmt.Errorf("invalid SCIP response format: %w", err)
	}

	return result.Response, nil
}

// MapDocumentSymbol handles textDocument/documentSymbol requests
func (m *LSPSCIPMapper) MapDocumentSymbol(params *LSPParams) (json.RawMessage, error) {
	startTime := time.Now()
	defer m.updateRequestStats("documentSymbol", startTime)

	if params.TextDocument == nil {
		return nil, fmt.Errorf("invalid parameters: textDocument is required")
	}

	// Build SCIP query parameters
	scipParams := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": params.TextDocument.URI,
		},
	}

	// Query SCIP store
	result := m.scipStore.Query("textDocument/documentSymbol", scipParams)
	
	if !result.Found {
		if result.Error != "" {
			return nil, fmt.Errorf("SCIP query failed: %s", result.Error)
		}
		// Return empty array for no symbols
		return json.RawMessage("[]"), nil
	}

	// Validate and return the SCIP response
	if err := m.validateLSPDocumentSymbolResponse(result.Response); err != nil {
		return nil, fmt.Errorf("invalid SCIP response format: %w", err)
	}

	return result.Response, nil
}

// MapWorkspaceSymbol handles workspace/symbol requests
func (m *LSPSCIPMapper) MapWorkspaceSymbol(params *LSPParams) (json.RawMessage, error) {
	startTime := time.Now()
	defer m.updateRequestStats("workspaceSymbol", startTime)

	if params.Query == "" {
		return nil, fmt.Errorf("invalid parameters: query is required")
	}

	// Build SCIP query parameters
	scipParams := map[string]interface{}{
		"query": params.Query,
	}

	// Query SCIP store
	result := m.scipStore.Query("workspace/symbol", scipParams)
	
	if !result.Found {
		if result.Error != "" {
			return nil, fmt.Errorf("SCIP query failed: %s", result.Error)
		}
		// Return empty array for no symbols
		return json.RawMessage("[]"), nil
	}

	// Validate and return the SCIP response
	if err := m.validateLSPWorkspaceSymbolResponse(result.Response); err != nil {
		return nil, fmt.Errorf("invalid SCIP response format: %w", err)
	}

	return result.Response, nil
}

// QuerySCIP provides the main query interface for LSP methods
func (m *LSPSCIPMapper) QuerySCIP(method string, params interface{}) SCIPQueryResult {
	startTime := time.Now()
	
	// Increment request count
	atomic.AddInt64(&m.requestCount, 1)

	// Check if method is supported
	if !IsSupportedMethod(method) {
		atomic.AddInt64(&m.errorCount, 1)
		return SCIPQueryResult{
			Found:      false,
			Method:     method,
			Error:      fmt.Sprintf("unsupported LSP method: %s", method),
			CacheHit:   false,
			QueryTime:  time.Since(startTime),
			Confidence: 0.0,
		}
	}

	// Query the SCIP store
	result := m.scipStore.Query(method, params)
	
	// Update statistics
	if result.Found {
		atomic.AddInt64(&m.successCount, 1)
	} else {
		atomic.AddInt64(&m.errorCount, 1)
	}

	// Update cache statistics
	if result.CacheHit {
		atomic.AddInt64(&m.statsTracker.CacheHits, 1)
	} else {
		atomic.AddInt64(&m.statsTracker.CacheMisses, 1)
	}

	// Track query performance
	queryTime := time.Since(startTime)
	if queryTime < 5*time.Millisecond {
		atomic.AddInt64(&m.statsTracker.FastQueries, 1)
	} else if queryTime > 50*time.Millisecond {
		atomic.AddInt64(&m.statsTracker.SlowQueries, 1)
	}

	m.updateLastRequestTime()

	if m.config.Logging.LogQueries {
		log.Printf("LSP-SCIP: Query for method %s completed in %v (found: %t, cache_hit: %t, confidence: %.2f)", 
			method, queryTime, result.Found, result.CacheHit, result.Confidence)
	}

	return result
}

// ConvertLSPPositionToByteOffset converts LSP line/character position to byte offset
func (m *LSPSCIPMapper) ConvertLSPPositionToByteOffset(content string, position LSPPosition) (int, error) {
	lines := strings.Split(content, "\n")
	
	if position.Line >= len(lines) {
		return 0, fmt.Errorf("line %d exceeds document length %d", position.Line, len(lines))
	}

	byteOffset := 0
	
	// Add bytes for all previous lines
	for i := 0; i < position.Line; i++ {
		byteOffset += len(lines[i]) + 1 // +1 for newline character
	}
	
	// Add bytes for character position in current line
	currentLine := lines[position.Line]
	if position.Character > utf8.RuneCountInString(currentLine) {
		return 0, fmt.Errorf("character %d exceeds line length %d", position.Character, utf8.RuneCountInString(currentLine))
	}
	
	// Convert character position to byte position
	runeIndex := 0
	for byteIndex, _ := range currentLine {
		if runeIndex == position.Character {
			byteOffset += byteIndex
			break
		}
		runeIndex++
		if runeIndex == len(currentLine) {
			byteOffset += len(currentLine)
		}
	}
	
	return byteOffset, nil
}

// ConvertByteOffsetToLSPPosition converts byte offset to LSP line/character position
func (m *LSPSCIPMapper) ConvertByteOffsetToLSPPosition(content string, byteOffset int) (LSPPosition, error) {
	if byteOffset > len(content) {
		return LSPPosition{}, fmt.Errorf("byte offset %d exceeds content length %d", byteOffset, len(content))
	}

	lines := strings.Split(content[:byteOffset], "\n")
	line := len(lines) - 1
	
	var character int
	if line > 0 {
		lastLine := lines[line]
		character = utf8.RuneCountInString(lastLine)
	} else {
		character = utf8.RuneCountInString(content[:byteOffset])
	}

	return LSPPosition{
		Line:      line,
		Character: character,
	}, nil
}

// NormalizeURI normalizes a file URI for consistent processing
func (m *LSPSCIPMapper) NormalizeURI(uri string) (string, error) {
	if uri == "" {
		return "", fmt.Errorf("empty URI")
	}

	// Parse and validate URI
	parsedURI, err := url.Parse(uri)
	if err != nil {
		return "", fmt.Errorf("invalid URI format: %w", err)
	}

	// Ensure file:// scheme
	if parsedURI.Scheme == "" {
		parsedURI.Scheme = "file"
	} else if parsedURI.Scheme != "file" {
		return "", fmt.Errorf("unsupported URI scheme: %s", parsedURI.Scheme)
	}

	return parsedURI.String(), nil
}

// GetStats returns comprehensive mapper statistics
func (m *LSPSCIPMapper) GetStats() MapperStats {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	requestCount := atomic.LoadInt64(&m.requestCount)
	successCount := atomic.LoadInt64(&m.successCount)
	
	stats := *m.statsTracker
	stats.TotalRequests = requestCount
	stats.SuccessfulRequests = successCount
	stats.FailedRequests = atomic.LoadInt64(&m.errorCount)
	stats.AverageResponseTime = m.avgResponseTime
	stats.LastRequestTime = m.lastRequestTime

	return stats
}

// Validation methods for LSP response formats

func (m *LSPSCIPMapper) validateLSPLocationResponse(response json.RawMessage) error {
	var location LSPLocation
	if err := json.Unmarshal(response, &location); err != nil {
		// Try array format
		var locations []LSPLocation
		if err := json.Unmarshal(response, &locations); err != nil {
			return fmt.Errorf("response is not a valid LSP location or location array: %w", err)
		}
	}
	return nil
}

func (m *LSPSCIPMapper) validateLSPLocationArrayResponse(response json.RawMessage) error {
	var locations []LSPLocation
	if err := json.Unmarshal(response, &locations); err != nil {
		return fmt.Errorf("response is not a valid LSP location array: %w", err)
	}
	return nil
}

func (m *LSPSCIPMapper) validateLSPHoverResponse(response json.RawMessage) error {
	var hover Hover
	if err := json.Unmarshal(response, &hover); err != nil {
		return fmt.Errorf("response is not a valid LSP hover: %w", err)
	}
	return nil
}

func (m *LSPSCIPMapper) validateLSPDocumentSymbolResponse(response json.RawMessage) error {
	var symbols []DocumentSymbol
	if err := json.Unmarshal(response, &symbols); err != nil {
		return fmt.Errorf("response is not a valid LSP document symbol array: %w", err)
	}
	return nil
}

func (m *LSPSCIPMapper) validateLSPWorkspaceSymbolResponse(response json.RawMessage) error {
	var symbols []SymbolInformation
	if err := json.Unmarshal(response, &symbols); err != nil {
		return fmt.Errorf("response is not a valid LSP workspace symbol array: %w", err)
	}
	return nil
}

// Helper methods for statistics and performance tracking

func (m *LSPSCIPMapper) updateRequestStats(method string, startTime time.Time) {
	queryTime := time.Since(startTime)
	
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	// Update average response time (simple moving average)
	if m.avgResponseTime == 0 {
		m.avgResponseTime = queryTime
	} else {
		m.avgResponseTime = (m.avgResponseTime + queryTime) / 2
	}
	
	// Update method-specific counters
	switch method {
	case "definition":
		atomic.AddInt64(&m.statsTracker.DefinitionRequests, 1)
	case "references":
		atomic.AddInt64(&m.statsTracker.ReferencesRequests, 1)
	case "hover":
		atomic.AddInt64(&m.statsTracker.HoverRequests, 1)
	case "documentSymbol":
		atomic.AddInt64(&m.statsTracker.DocumentSymbolRequests, 1)
	case "workspaceSymbol":
		atomic.AddInt64(&m.statsTracker.WorkspaceSymbolRequests, 1)
	}
}

func (m *LSPSCIPMapper) updateLastRequestTime() {
	m.mutex.Lock()
	m.lastRequestTime = time.Now()
	m.statsTracker.LastRequestTime = m.lastRequestTime
	m.mutex.Unlock()
}

// ParseLSPParams parses generic parameters into strongly-typed LSP parameter structures
func (m *LSPSCIPMapper) ParseLSPParams(method string, params interface{}) (*LSPParams, error) {
	paramBytes, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal parameters: %w", err)
	}

	lspParams := &LSPParams{
		Method: method,
	}

	switch method {
	case "textDocument/definition", "textDocument/references", "textDocument/hover":
		var posParams TextDocumentPositionParams
		if err := json.Unmarshal(paramBytes, &posParams); err != nil {
			return nil, fmt.Errorf("invalid position parameters for %s: %w", method, err)
		}
		lspParams.TextDocument = &posParams.TextDocument
		lspParams.Position = &posParams.Position
		lspParams.URI = posParams.TextDocument.URI

	case "textDocument/documentSymbol":
		var docParams DocumentSymbolParams
		if err := json.Unmarshal(paramBytes, &docParams); err != nil {
			return nil, fmt.Errorf("invalid document parameters for %s: %w", method, err)
		}
		lspParams.TextDocument = &docParams.TextDocument
		lspParams.URI = docParams.TextDocument.URI

	case "workspace/symbol":
		var wsParams WorkspaceSymbolParams
		if err := json.Unmarshal(paramBytes, &wsParams); err != nil {
			return nil, fmt.Errorf("invalid workspace parameters for %s: %w", method, err)
		}
		lspParams.Query = wsParams.Query

	default:
		return nil, fmt.Errorf("unsupported method: %s", method)
	}

	// Normalize URI if present
	if lspParams.URI != "" {
		normalizedURI, err := m.NormalizeURI(lspParams.URI)
		if err != nil {
			return nil, fmt.Errorf("invalid URI: %w", err)
		}
		lspParams.URI = normalizedURI
		if lspParams.TextDocument != nil {
			lspParams.TextDocument.URI = normalizedURI
		}
	}

	return lspParams, nil
}

// IsWithinPerformanceTarget checks if query time meets enterprise performance targets
func (m *LSPSCIPMapper) IsWithinPerformanceTarget(queryTime time.Duration) bool {
	// Enterprise target: sub-10ms P99 response time
	return queryTime < 10*time.Millisecond
}

// GenerateErrorResponse creates a standardized error response for LSP methods
func (m *LSPSCIPMapper) GenerateErrorResponse(method string, err error) (json.RawMessage, error) {
	errorResponse := map[string]interface{}{
		"error": map[string]interface{}{
			"code":    -32603, // LSP Internal Error code
			"message": fmt.Sprintf("SCIP query failed for method %s: %s", method, err.Error()),
			"data": map[string]interface{}{
				"method":    method,
				"timestamp": time.Now().UTC().Format(time.RFC3339),
				"source":    "lsp_scip_mapper",
			},
		},
	}

	response, jsonErr := json.Marshal(errorResponse)
	if jsonErr != nil {
		return nil, fmt.Errorf("failed to marshal error response: %w", jsonErr)
	}

	return json.RawMessage(response), err
}

// LogPerformanceWarning logs warnings for queries that exceed performance targets
func (m *LSPSCIPMapper) LogPerformanceWarning(method string, queryTime time.Duration, uri string) {
	if m.config.Logging.LogQueries && queryTime > 25*time.Millisecond {
		log.Printf("LSP-SCIP Performance Warning: Method %s took %v (target: <10ms, max: 25ms) for URI %s", 
			method, queryTime, uri)
	}
}

// Close cleanly shuts down the mapper and releases resources
func (m *LSPSCIPMapper) Close() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.config.Logging.LogIndexOperations {
		stats := m.GetStats()
		log.Printf("LSP-SCIP Mapper closing: %d total requests, %d successful, %.2f%% success rate, avg response: %v", 
			stats.TotalRequests, stats.SuccessfulRequests, 
			float64(stats.SuccessfulRequests)/float64(stats.TotalRequests)*100.0,
			stats.AverageResponseTime)
	}

	return nil
}

// Validate interface compliance
var _ interface{} = (*LSPSCIPMapper)(nil)