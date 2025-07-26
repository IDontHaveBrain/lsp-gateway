package indexing

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/sourcegraph/scip/bindings/go/scip"
	"google.golang.org/protobuf/proto"
)

// SCIPClient provides core SCIP functionality for loading, parsing, and querying SCIP index files
type SCIPClient struct {
	// Index storage and management
	indices      map[string]*SCIPIndexData // path -> index data
	symbolIndex  map[string]*SymbolEntry   // symbol -> entry
	documentIndex map[string]*DocumentData  // file path -> document
	
	// Configuration and state  
	config       *SCIPConfig
	mutex        sync.RWMutex
	
	// Performance tracking
	stats        *ClientStats
	lastActivity time.Time
}

// SCIPIndexData represents loaded SCIP index data
type SCIPIndexData struct {
	Metadata     *scip.Metadata
	Documents    []*scip.Document
	LoadedAt     time.Time
	FilePath     string
	FileSize     int64
	CheckSum     string
}

// DocumentData represents processed document information
type DocumentData struct {
	URI          string
	Language     string
	Symbols      []*SymbolEntry
	Occurrences  []*OccurrenceEntry
	ExternalRefs []*scip.SymbolInformation
	RelativePath string
	LineCount    int
	LastModified time.Time
}

// SymbolEntry represents a symbol with its metadata and locations
type SymbolEntry struct {
	Symbol         string
	DisplayName    string
	Kind           scip.SymbolInformation_Kind
	Documentation  []*SCIPDocumentation
	Relationships  []*scip.Relationship
	Signature      *SCIPSignature
	Occurrences    []*OccurrenceEntry
	DefinitionURI  string
	DefinitionRange *scip.Range
}

// OccurrenceEntry represents a symbol occurrence
type OccurrenceEntry struct {
	Range         *scip.Range
	Symbol        string
	Role          int32
	SyntaxKind    scip.SyntaxKind
	Diagnostics   []*scip.Diagnostic
	EnclosingRange *scip.Range
}

// ClientStats tracks performance and usage statistics
type ClientStats struct {
	IndexLoads        int64
	TotalDocuments    int64
	TotalSymbols      int64
	TotalOccurrences  int64
	MemoryUsage       int64
	AverageLoadTime   time.Duration
	LastLoadTime      time.Time
	QueryCount        int64
	AverageQueryTime  time.Duration
	ErrorCount        int64
	LastError         string
	LastErrorTime     time.Time
}

// SCIPVisitor implements the IndexVisitor interface for processing SCIP data
type SCIPVisitor struct {
	client    *SCIPClient
	indexData *SCIPIndexData
	errors    []error
	ctx       context.Context
}

// NewSCIPClient creates a new SCIP client with the given configuration
func NewSCIPClient(config *SCIPConfig) (*SCIPClient, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// Validate configuration
	if err := validateSCIPConfig(config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	client := &SCIPClient{
		indices:      make(map[string]*SCIPIndexData),
		symbolIndex:  make(map[string]*SymbolEntry),
		documentIndex: make(map[string]*DocumentData),
		config:       config,
		stats:        &ClientStats{},
		lastActivity: time.Now(),
	}

	return client, nil
}

// LoadIndex loads a .scip file using ParseStreaming
func (c *SCIPClient) LoadIndex(indexPath string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	startTime := time.Now()
	defer func() {
		c.stats.AverageLoadTime = time.Since(startTime)
		c.stats.LastLoadTime = time.Now()
		c.lastActivity = time.Now()
	}()

	// Validate file path
	if indexPath == "" {
		err := fmt.Errorf("index path cannot be empty")
		c.recordError(err)
		return err
	}

	// Check if file exists and get info
	fileInfo, err := os.Stat(indexPath)
	if err != nil {
		err = fmt.Errorf("failed to stat index file %s: %w", indexPath, err)
		c.recordError(err)
		return err
	}

	if fileInfo.IsDir() {
		err = fmt.Errorf("path %s is a directory, not a file", indexPath)
		c.recordError(err)
		return err
	}

	// Check if already loaded and up-to-date
	absPath, err := filepath.Abs(indexPath)
	if err != nil {
		err = fmt.Errorf("failed to get absolute path for %s: %w", indexPath, err)
		c.recordError(err)
		return err
	}

	if existing, exists := c.indices[absPath]; exists {
		if existing.LoadedAt.After(fileInfo.ModTime()) {
			// Index is already loaded and up-to-date
			return nil
		}
		// Remove old index data
		c.removeIndex(absPath)
	}

	// Open and read the file
	file, err := os.Open(indexPath)
	if err != nil {
		err = fmt.Errorf("failed to open index file %s: %w", indexPath, err)
		c.recordError(err)
		return err
	}
	defer file.Close()

	// Parse the SCIP file
	if err := c.ParseSCIPFile(file, absPath, fileInfo.Size()); err != nil {
		c.recordError(err)
		return fmt.Errorf("failed to parse SCIP file %s: %w", indexPath, err)
	}

	c.stats.IndexLoads++
	if c.config.Logging.LogIndexOperations {
		fmt.Printf("SCIP: Successfully loaded index from %s (%d documents, %d symbols)\n", 
			indexPath, c.stats.TotalDocuments, c.stats.TotalSymbols)
	}

	return nil
}

// ParseSCIPFile parses SCIP data using IndexVisitor
func (c *SCIPClient) ParseSCIPFile(reader io.Reader, filePath string, fileSize int64) error {
	// Create index data structure
	indexData := &SCIPIndexData{
		FilePath:  filePath,
		FileSize:  fileSize,
		LoadedAt:  time.Now(),
		Documents: make([]*scip.Document, 0),
	}

	// Create visitor for processing the SCIP data
	visitor := &SCIPVisitor{
		client:    c,
		indexData: indexData,
		errors:    make([]error, 0),
		ctx:       context.Background(),
	}

	// Parse the SCIP index - read data and unmarshal protobuf
	data, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("failed to read SCIP data: %w", err)
	}
	
	// Create a SCIP Index and unmarshal using protobuf
	var index scip.Index
	if err := proto.Unmarshal(data, &index); err != nil {
		return fmt.Errorf("SCIP unmarshaling failed: %w", err)
	}
	
	// Process the metadata
	if index.Metadata != nil {
		if err := visitor.VisitMetadata(index.Metadata); err != nil {
			return fmt.Errorf("failed to visit metadata: %w", err)
		}
	}
	
	// Process all documents
	for _, doc := range index.Documents {
		if err := visitor.VisitDocument(doc); err != nil {
			return fmt.Errorf("failed to visit document: %w", err)
		}
	}

	// Check for visitor errors
	if len(visitor.errors) > 0 {
		var errorMsgs []string
		for _, err := range visitor.errors {
			errorMsgs = append(errorMsgs, err.Error())
		}
		return fmt.Errorf("visitor errors: %s", strings.Join(errorMsgs, "; "))
	}

	// Store the parsed index
	c.indices[filePath] = indexData

	return nil
}

// VisitMetadata processes index metadata (implements IndexVisitor)
func (v *SCIPVisitor) VisitMetadata(metadata *scip.Metadata) error {
	if metadata == nil {
		err := fmt.Errorf("metadata is nil")
		v.errors = append(v.errors, err)
		return err
	}

	v.indexData.Metadata = metadata
	
	// Log metadata information if configured
	if v.client.config.Logging.LogIndexOperations {
		fmt.Printf("SCIP: Processing metadata - Version: %s, Tool: %s, Project: %s\n",
			metadata.Version, metadata.ToolInfo.GetName(), metadata.ProjectRoot)
	}

	return nil
}

// VisitDocument processes individual documents (implements IndexVisitor)
func (v *SCIPVisitor) VisitDocument(document *scip.Document) error {
	if document == nil {
		err := fmt.Errorf("document is nil")
		v.errors = append(v.errors, err)
		return err
	}

	// Check context cancellation
	select {
	case <-v.ctx.Done():
		return v.ctx.Err()
	default:
	}

	// Process document and build internal structures
	docData, err := v.processDocument(document)
	if err != nil {
		v.errors = append(v.errors, fmt.Errorf("failed to process document %s: %w", document.RelativePath, err))
		return err
	}

	// Store document data
	v.client.documentIndex[document.RelativePath] = docData
	v.indexData.Documents = append(v.indexData.Documents, document)

	// Update statistics
	v.client.stats.TotalDocuments++
	v.client.stats.TotalSymbols += int64(len(docData.Symbols))
	v.client.stats.TotalOccurrences += int64(len(docData.Occurrences))

	return nil
}

// processDocument processes a single document and builds queryable data structures
func (v *SCIPVisitor) processDocument(document *scip.Document) (*DocumentData, error) {
	docData := &DocumentData{
		URI:          document.RelativePath,
		Language:     document.Language,
		RelativePath: document.RelativePath,
		Symbols:      make([]*SymbolEntry, 0),
		Occurrences:  make([]*OccurrenceEntry, 0),
		ExternalRefs: document.Symbols,
		LastModified: time.Now(),
	}

	// Process symbols
	for _, symbol := range document.Symbols {
		// Convert documentation from []string to []*SCIPDocumentation
		var docs []*SCIPDocumentation
		for _, docStr := range symbol.Documentation {
			docs = append(docs, &SCIPDocumentation{
				Format: "text",
				Value:  docStr,
			})
		}
		
		symEntry := &SymbolEntry{
			Symbol:        symbol.Symbol,
			DisplayName:   symbol.DisplayName,
			Kind:          symbol.Kind,
			Documentation: docs,
			Relationships: symbol.Relationships,
			Signature:     nil, // Signature field not available in current SCIP version
			Occurrences:   make([]*OccurrenceEntry, 0),
		}

		// Store in symbol index for fast lookup
		v.client.symbolIndex[symbol.Symbol] = symEntry
		docData.Symbols = append(docData.Symbols, symEntry)
	}

	// Process occurrences
	for _, occurrence := range document.Occurrences {
		occEntry := &OccurrenceEntry{
			Range:         ConvertSCIPRangeToRange(occurrence.Range),
			Symbol:        occurrence.Symbol,
			Role:          occurrence.SymbolRoles,
			SyntaxKind:    occurrence.SyntaxKind,
			Diagnostics:   occurrence.Diagnostics,
			EnclosingRange: ConvertSCIPRangeToRange(occurrence.EnclosingRange),
		}

		docData.Occurrences = append(docData.Occurrences, occEntry)

		// Link to symbol entry
		if symEntry, exists := v.client.symbolIndex[occurrence.Symbol]; exists {
			symEntry.Occurrences = append(symEntry.Occurrences, occEntry)
			
			// Set definition if this is a definition occurrence
			if occurrence.SymbolRoles&int32(scip.SymbolRole_Definition) != 0 {
				symEntry.DefinitionURI = document.RelativePath
				symEntry.DefinitionRange = ConvertSCIPRangeToRange(occurrence.Range)
			}
		}
	}

	// Count lines (rough estimate)
	docData.LineCount = len(strings.Split(document.Text, "\n"))

	return docData, nil
}

// GetDocumentData retrieves document information by file path
func (c *SCIPClient) GetDocumentData(filePath string) (*DocumentData, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	startTime := time.Now()
	defer func() {
		c.stats.QueryCount++
		queryTime := time.Since(startTime)
		if c.stats.AverageQueryTime == 0 {
			c.stats.AverageQueryTime = queryTime
		} else {
			c.stats.AverageQueryTime = (c.stats.AverageQueryTime + queryTime) / 2
		}
		c.lastActivity = time.Now()
	}()

	if filePath == "" {
		return nil, fmt.Errorf("file path cannot be empty")
	}

	// Normalize path
	normPath := filepath.Clean(filePath)
	
	// Try exact match first
	if docData, exists := c.documentIndex[normPath]; exists {
		return docData, nil
	}

	// Try relative path variations
	for path, docData := range c.documentIndex {
		if strings.HasSuffix(path, normPath) || strings.HasSuffix(normPath, path) {
			return docData, nil
		}
	}

	return nil, fmt.Errorf("document not found: %s", filePath)
}

// FindSymbolsByPrefix finds symbols matching the given prefix
func (c *SCIPClient) FindSymbolsByPrefix(prefix string) ([]*SymbolEntry, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	startTime := time.Now()
	defer func() {
		c.stats.QueryCount++
		queryTime := time.Since(startTime)
		if c.stats.AverageQueryTime == 0 {
			c.stats.AverageQueryTime = queryTime
		} else {
			c.stats.AverageQueryTime = (c.stats.AverageQueryTime + queryTime) / 2
		}
		c.lastActivity = time.Now()
	}()

	if prefix == "" {
		return nil, fmt.Errorf("prefix cannot be empty")
	}

	var results []*SymbolEntry
	lowerPrefix := strings.ToLower(prefix)

	for symbol, entry := range c.symbolIndex {
		// Check symbol name
		if strings.HasPrefix(strings.ToLower(symbol), lowerPrefix) {
			results = append(results, entry)
			continue
		}

		// Check display name
		if entry.DisplayName != "" && strings.HasPrefix(strings.ToLower(entry.DisplayName), lowerPrefix) {
			results = append(results, entry)
		}
	}

	// Sort results by symbol name for consistent ordering
	sort.Slice(results, func(i, j int) bool {
		return results[i].Symbol < results[j].Symbol
	})

	return results, nil
}

// GetSymbolByName finds a symbol by exact name
func (c *SCIPClient) GetSymbolByName(symbolName string) (*SymbolEntry, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if symbolName == "" {
		return nil, fmt.Errorf("symbol name cannot be empty")
	}

	if entry, exists := c.symbolIndex[symbolName]; exists {
		return entry, nil
	}

	return nil, fmt.Errorf("symbol not found: %s", symbolName)
}

// GetStats returns current client statistics
func (c *SCIPClient) GetStats() *ClientStats {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	// Create a copy to avoid concurrent modification
	statsCopy := *c.stats
	statsCopy.MemoryUsage = c.estimateMemoryUsage()
	
	return &statsCopy
}

// IsHealthy checks if the client is in a healthy state
func (c *SCIPClient) IsHealthy() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	// Check if we have loaded indices
	if len(c.indices) == 0 {
		return false
	}

	// Check if recent activity (within last hour)
	if time.Since(c.lastActivity) > time.Hour {
		return false
	}

	// Check error rate (should be less than 10%)
	if c.stats.QueryCount > 0 {
		errorRate := float64(c.stats.ErrorCount) / float64(c.stats.QueryCount)
		if errorRate > 0.10 {
			return false
		}
	}

	return true
}

// Close cleans up resources and closes the client
func (c *SCIPClient) Close() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Clear all indices
	c.indices = make(map[string]*SCIPIndexData)
	c.symbolIndex = make(map[string]*SymbolEntry)
	c.documentIndex = make(map[string]*DocumentData)

	if c.config.Logging.LogIndexOperations {
		fmt.Println("SCIP: Client closed successfully")
	}

	return nil
}

// Helper methods

// removeIndex removes an index from all internal data structures
func (c *SCIPClient) removeIndex(indexPath string) {
	if indexData, exists := c.indices[indexPath]; exists {
		// Remove associated symbols and documents
		for _, doc := range indexData.Documents {
			delete(c.documentIndex, doc.RelativePath)
			
			// Remove symbols from this document
			for _, symbol := range doc.Symbols {
				delete(c.symbolIndex, symbol.Symbol)
			}
		}
		
		delete(c.indices, indexPath)
	}
}

// recordError records an error in statistics
func (c *SCIPClient) recordError(err error) {
	c.stats.ErrorCount++
	c.stats.LastError = err.Error()
	c.stats.LastErrorTime = time.Now()
}

// estimateMemoryUsage provides a rough estimate of memory usage
func (c *SCIPClient) estimateMemoryUsage() int64 {
	var total int64
	
	// Base structure overhead
	total += 1024 * 8 // Base struct sizes
	
	// Index data
	total += int64(len(c.indices) * 512)
	
	// Symbol index  
	for symbol, entry := range c.symbolIndex {
		total += int64(len(symbol) * 2) // UTF-16 estimate
		total += int64(len(entry.DisplayName) * 2)
		total += int64(len(entry.Occurrences) * 128) // Rough per-occurrence size
		total += 256 // Base entry overhead
	}
	
	// Document index
	for path, doc := range c.documentIndex {
		total += int64(len(path) * 2)
		total += int64(len(doc.Symbols) * 64)
		total += int64(len(doc.Occurrences) * 128)
		total += 512 // Base document overhead
	}
	
	return total
}

// validateSCIPConfig validates the SCIP configuration
func validateSCIPConfig(config *SCIPConfig) error {
	if config.Performance.QueryTimeout <= 0 {
		return fmt.Errorf("query timeout must be positive")
	}
	
	if config.Performance.MaxConcurrentQueries <= 0 {
		return fmt.Errorf("max concurrent queries must be positive")
	}
	
	if config.Performance.IndexLoadTimeout <= 0 {
		return fmt.Errorf("index load timeout must be positive")
	}
	
	if config.CacheConfig.MaxSize < 0 {
		return fmt.Errorf("cache max size cannot be negative")
	}
	
	if config.CacheConfig.TTL < 0 {
		return fmt.Errorf("cache TTL cannot be negative")
	}
	
	return nil
}

// GetLoadedIndices returns information about all loaded indices
func (c *SCIPClient) GetLoadedIndices() map[string]*SCIPIndexData {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	// Return a copy to prevent external modification
	result := make(map[string]*SCIPIndexData, len(c.indices))
	for k, v := range c.indices {
		indexCopy := *v
		result[k] = &indexCopy
	}
	
	return result
}

// ReloadIndex reloads a specific index file
func (c *SCIPClient) ReloadIndex(indexPath string) error {
	// Remove existing index first
	c.mutex.Lock()
	absPath, err := filepath.Abs(indexPath)
	if err != nil {
		c.mutex.Unlock()
		return fmt.Errorf("failed to get absolute path: %w", err)
	}
	c.removeIndex(absPath)
	c.mutex.Unlock()

	// Load the index again
	return c.LoadIndex(indexPath)
}

// Ensure SCIPClient can work with the existing SCIPStore interface (partial implementation)
var _ SCIPStore = (*SCIPClientAdapter)(nil)

// SCIPClientAdapter adapts SCIPClient to work with the existing SCIPStore interface
type SCIPClientAdapter struct {
	client *SCIPClient
	cache  map[string]json.RawMessage
	cacheMutex sync.RWMutex
}

// NewSCIPClientAdapter creates an adapter that wraps SCIPClient to implement SCIPStore
func NewSCIPClientAdapter(client *SCIPClient) *SCIPClientAdapter {
	return &SCIPClientAdapter{
		client: client,
		cache:  make(map[string]json.RawMessage),
	}
}

// LoadIndex implements SCIPStore interface
func (a *SCIPClientAdapter) LoadIndex(path string) error {
	return a.client.LoadIndex(path)
}

// Query implements SCIPStore interface (basic implementation)
func (a *SCIPClientAdapter) Query(method string, params interface{}) SCIPQueryResult {
	startTime := time.Now()
	
	// This is a simplified implementation - a full implementation would
	// map LSP queries to SCIP data structures
	result := SCIPQueryResult{
		Found:      false,
		Method:     method,
		CacheHit:   false,
		QueryTime:  time.Since(startTime),
		Confidence: 0.0,
		Error:      "query not implemented in base SCIP client",
	}
	
	return result
}

// CacheResponse implements SCIPStore interface
func (a *SCIPClientAdapter) CacheResponse(method string, params interface{}, response json.RawMessage) error {
	a.cacheMutex.Lock()
	defer a.cacheMutex.Unlock()
	
	key := fmt.Sprintf("%s:%v", method, params)
	a.cache[key] = response
	return nil
}

// InvalidateFile implements SCIPStore interface
func (a *SCIPClientAdapter) InvalidateFile(filePath string) {
	a.cacheMutex.Lock()
	defer a.cacheMutex.Unlock()
	
	// Simple implementation - remove all cache entries
	// A more sophisticated implementation would track file associations
	for key := range a.cache {
		if strings.Contains(key, filePath) {
			delete(a.cache, key)
		}
	}
}

// GetStats implements SCIPStore interface
func (a *SCIPClientAdapter) GetStats() SCIPStoreStats {
	clientStats := a.client.GetStats()
	
	return SCIPStoreStats{
		IndexesLoaded:    int(clientStats.IndexLoads),
		TotalQueries:     clientStats.QueryCount,
		CacheHitRate:     0.0, // Would need to track cache hits
		AverageQueryTime: clientStats.AverageQueryTime,
		LastQueryTime:    time.Now(),
		CacheSize:        len(a.cache),
		MemoryUsage:      clientStats.MemoryUsage,
	}
}

// Close implements SCIPStore interface
func (a *SCIPClientAdapter) Close() error {
	a.cacheMutex.Lock()
	a.cache = make(map[string]json.RawMessage)
	a.cacheMutex.Unlock()
	
	return a.client.Close()
}