package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"lsp-gateway/src/internal/common"
	"lsp-gateway/src/internal/models/lsp"
)

// IndexBuildStrategy defines how the index should be built
type IndexBuildStrategy int

const (
	// IncrementalIndex builds index incrementally as files are accessed
	IncrementalIndex IndexBuildStrategy = iota
	// FullWorkspaceIndex builds index for entire workspace upfront
	FullWorkspaceIndex
	// HotFilesIndex builds index only for frequently accessed files
	HotFilesIndex
)

// IndexingJob represents a single indexing task
type IndexingJob struct {
	URI         string
	Language    string
	Priority    int // Higher number = higher priority
	RequestedAt time.Time
	CompletedAt *time.Time
	Error       error
}

// IndexingStats tracks indexing performance
type IndexingStats struct {
	TotalJobs          int
	CompletedJobs      int
	FailedJobs         int
	AverageIndexTime   time.Duration
	DocumentsIndexed   int
	SymbolsExtracted   int
	LastIndexingTime   time.Time
	IndexBuildStrategy IndexBuildStrategy
}

// SCIPIndexer manages the process of building and maintaining SCIP indexes
type SCIPIndexer struct {
	queryManager *SCIPQueryManager
	lspFallback  LSPFallback
	strategy     IndexBuildStrategy

	// Job queue and workers
	jobQueue chan *IndexingJob
	workers  int
	workerWG sync.WaitGroup
	running  bool

	// Statistics and monitoring
	stats           *IndexingStats
	fileAccessCount map[string]int // Track file access frequency

	// Concurrency control
	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
}

// NewSCIPIndexer creates a new SCIP indexer
func NewSCIPIndexer(queryManager *SCIPQueryManager, lspFallback LSPFallback, strategy IndexBuildStrategy) *SCIPIndexer {
	ctx, cancel := context.WithCancel(context.Background())

	return &SCIPIndexer{
		queryManager:    queryManager,
		lspFallback:     lspFallback,
		strategy:        strategy,
		jobQueue:        make(chan *IndexingJob, 1000), // Buffered queue
		workers:         4,                             // Default worker count
		fileAccessCount: make(map[string]int),
		stats: &IndexingStats{
			IndexBuildStrategy: strategy,
		},
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start begins the indexing process
func (idx *SCIPIndexer) Start() error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if idx.running {
		return fmt.Errorf("indexer is already running")
	}

	idx.running = true

	// Start worker goroutines
	for i := 0; i < idx.workers; i++ {
		idx.workerWG.Add(1)
		go idx.worker(i)
	}

	common.LSPLogger.Info("SCIP indexer started with %d workers using %s strategy",
		idx.workers, idx.strategyName())

	return nil
}

// Stop stops the indexing process
func (idx *SCIPIndexer) Stop() error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if !idx.running {
		return nil
	}

	idx.running = false
	idx.cancel()
	close(idx.jobQueue)

	// Wait for workers to finish
	idx.workerWG.Wait()

	common.LSPLogger.Info("SCIP indexer stopped")
	return nil
}

// RequestIndexing requests indexing for a specific file
func (idx *SCIPIndexer) RequestIndexing(uri, language string, priority int) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if !idx.running {
		return
	}

	// Track file access frequency
	idx.fileAccessCount[uri]++

	job := &IndexingJob{
		URI:         uri,
		Language:    language,
		Priority:    priority,
		RequestedAt: time.Now(),
	}

	select {
	case idx.jobQueue <- job:
		common.LSPLogger.Debug("Queued indexing job for %s (priority: %d)", uri, priority)
	default:
		common.LSPLogger.Warn("Indexing queue full, dropping job for %s", uri)
	}
}

// IndexDocument indexes a single document immediately
func (idx *SCIPIndexer) IndexDocument(ctx context.Context, uri, language string) error {
	start := time.Now()

	// Request document symbols from LSP server
	documentSymbols, err := idx.requestDocumentSymbols(ctx, uri)
	if err != nil {
		return fmt.Errorf("failed to get document symbols for %s: %w", uri, err)
	}

	// Request workspace symbols related to this document
	workspaceSymbols, err := idx.requestWorkspaceSymbols(ctx, filepath.Base(uri))
	if err != nil {
		common.LSPLogger.Warn("Failed to get workspace symbols for %s: %v", uri, err)
		workspaceSymbols = []*lsp.SymbolInformation{} // Continue with empty list
	}

	// Extract additional symbol information if available
	symbolInfos := idx.extractSymbolInformation(documentSymbols, workspaceSymbols, uri)

	// Update the query index
	err = idx.queryManager.UpdateIndex(uri, symbolInfos, documentSymbols)
	if err != nil {
		return fmt.Errorf("failed to update index for %s: %w", uri, err)
	}

	// Update statistics
	idx.mu.Lock()
	idx.stats.DocumentsIndexed++
	idx.stats.SymbolsExtracted += len(symbolInfos)
	idx.stats.LastIndexingTime = time.Now()
	if idx.stats.CompletedJobs > 0 {
		// Update rolling average
		alpha := 0.1
		duration := time.Since(start)
		idx.stats.AverageIndexTime = time.Duration(
			float64(idx.stats.AverageIndexTime)*(1-alpha) + float64(duration)*alpha)
	} else {
		idx.stats.AverageIndexTime = time.Since(start)
	}
	idx.mu.Unlock()

	common.LSPLogger.Debug("Indexed document %s: %d symbols in %v",
		uri, len(symbolInfos), time.Since(start))

	return nil
}

// GetStats returns current indexing statistics
func (idx *SCIPIndexer) GetStats() *IndexingStats {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	// Return a copy to avoid race conditions
	return &IndexingStats{
		TotalJobs:          idx.stats.TotalJobs,
		CompletedJobs:      idx.stats.CompletedJobs,
		FailedJobs:         idx.stats.FailedJobs,
		AverageIndexTime:   idx.stats.AverageIndexTime,
		DocumentsIndexed:   idx.stats.DocumentsIndexed,
		SymbolsExtracted:   idx.stats.SymbolsExtracted,
		LastIndexingTime:   idx.stats.LastIndexingTime,
		IndexBuildStrategy: idx.stats.IndexBuildStrategy,
	}
}

// GetHotFiles returns files accessed most frequently
func (idx *SCIPIndexer) GetHotFiles(limit int) []string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	type fileCount struct {
		uri   string
		count int
	}

	var files []fileCount
	for uri, count := range idx.fileAccessCount {
		files = append(files, fileCount{uri, count})
	}

	// Sort by access count (descending)
	for i := 0; i < len(files)-1; i++ {
		for j := i + 1; j < len(files); j++ {
			if files[i].count < files[j].count {
				files[i], files[j] = files[j], files[i]
			}
		}
	}

	var result []string
	for i, file := range files {
		if i >= limit {
			break
		}
		result = append(result, file.uri)
	}

	return result
}

// Private methods

// worker processes indexing jobs from the queue
func (idx *SCIPIndexer) worker(workerID int) {
	defer idx.workerWG.Done()

	common.LSPLogger.Debug("SCIP indexer worker %d started", workerID)

	for job := range idx.jobQueue {
		idx.processJob(job, workerID)
	}

	common.LSPLogger.Debug("SCIP indexer worker %d stopped", workerID)
}

// processJob processes a single indexing job
func (idx *SCIPIndexer) processJob(job *IndexingJob, workerID int) {
	start := time.Now()

	idx.mu.Lock()
	idx.stats.TotalJobs++
	idx.mu.Unlock()

	// Create context with timeout for this job
	ctx, cancel := context.WithTimeout(idx.ctx, 30*time.Second)
	defer cancel()

	err := idx.IndexDocument(ctx, job.URI, job.Language)

	completedAt := time.Now()
	job.CompletedAt = &completedAt
	job.Error = err

	idx.mu.Lock()
	if err != nil {
		idx.stats.FailedJobs++
		common.LSPLogger.Error("Worker %d failed to index %s: %v", workerID, job.URI, err)
	} else {
		idx.stats.CompletedJobs++
		common.LSPLogger.Debug("Worker %d indexed %s in %v", workerID, job.URI, time.Since(start))
	}
	idx.mu.Unlock()
}

// requestDocumentSymbols requests document symbols from LSP server
func (idx *SCIPIndexer) requestDocumentSymbols(ctx context.Context, uri string) ([]*lsp.DocumentSymbol, error) {
	params := &lsp.DocumentSymbolParams{
		TextDocument: lsp.TextDocumentIdentifier{URI: uri},
	}

	result, err := idx.lspFallback.ProcessRequest(ctx, "textDocument/documentSymbol", params)
	if err != nil {
		return nil, err
	}

	// Handle different response formats
	switch v := result.(type) {
	case []*lsp.DocumentSymbol:
		return v, nil
	case []interface{}:
		// Parse JSON response
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal response: %w", err)
		}

		var symbols []*lsp.DocumentSymbol
		err = json.Unmarshal(jsonBytes, &symbols)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal document symbols: %w", err)
		}
		return symbols, nil
	default:
		return nil, fmt.Errorf("unexpected response type: %T", result)
	}
}

// requestWorkspaceSymbols requests workspace symbols from LSP server
func (idx *SCIPIndexer) requestWorkspaceSymbols(ctx context.Context, query string) ([]*lsp.SymbolInformation, error) {
	params := &lsp.WorkspaceSymbolParams{Query: query}

	result, err := idx.lspFallback.ProcessRequest(ctx, "workspace/symbol", params)
	if err != nil {
		return nil, err
	}

	// Handle different response formats
	switch v := result.(type) {
	case []*lsp.SymbolInformation:
		return v, nil
	case []interface{}:
		// Parse JSON response
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal response: %w", err)
		}

		var symbols []*lsp.SymbolInformation
		err = json.Unmarshal(jsonBytes, &symbols)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal workspace symbols: %w", err)
		}
		return symbols, nil
	default:
		return nil, fmt.Errorf("unexpected response type: %T", result)
	}
}

// extractSymbolInformation creates SymbolInformation from DocumentSymbol
func (idx *SCIPIndexer) extractSymbolInformation(
	documentSymbols []*lsp.DocumentSymbol,
	workspaceSymbols []*lsp.SymbolInformation,
	uri string,
) []*lsp.SymbolInformation {
	var result []*lsp.SymbolInformation

	// Add workspace symbols first
	result = append(result, workspaceSymbols...)

	// Convert document symbols to symbol information
	for _, docSymbol := range documentSymbols {
		symbolInfo := idx.documentSymbolToSymbolInfo(docSymbol, uri, "")
		result = append(result, symbolInfo...)
	}

	return result
}

// documentSymbolToSymbolInfo recursively converts DocumentSymbol to SymbolInformation
func (idx *SCIPIndexer) documentSymbolToSymbolInfo(
	docSymbol *lsp.DocumentSymbol,
	uri string,
	containerName string,
) []*lsp.SymbolInformation {
	var result []*lsp.SymbolInformation

	// Create symbol information for this symbol
	symbolInfo := &lsp.SymbolInformation{
		Name: docSymbol.Name,
		Kind: docSymbol.Kind,
		Location: lsp.Location{
			URI:   uri,
			Range: docSymbol.Range,
		},
		ContainerName: containerName,
	}
	result = append(result, symbolInfo)

	// Process children recursively
	childContainer := docSymbol.Name
	if containerName != "" {
		childContainer = containerName + "." + docSymbol.Name
	}

	for _, child := range docSymbol.Children {
		childSymbols := idx.documentSymbolToSymbolInfo(child, uri, childContainer)
		result = append(result, childSymbols...)
	}

	return result
}

// strategyName returns human-readable strategy name
func (idx *SCIPIndexer) strategyName() string {
	switch idx.strategy {
	case IncrementalIndex:
		return "incremental indexing"
	case FullWorkspaceIndex:
		return "full workspace indexing"
	case HotFilesIndex:
		return "hot files indexing"
	default:
		return "unknown indexing strategy"
	}
}

// IndexingManager provides high-level indexing management
type IndexingManager struct {
	indexer *SCIPIndexer
	query   *SCIPQueryManager
	mu      sync.RWMutex
}

// NewIndexingManager creates a new indexing manager
func NewIndexingManager(lspFallback LSPFallback, strategy IndexBuildStrategy) *IndexingManager {
	queryManager := NewSCIPQueryManager(lspFallback)
	indexer := NewSCIPIndexer(queryManager, lspFallback, strategy)

	return &IndexingManager{
		indexer: indexer,
		query:   queryManager,
	}
}

// Start starts the indexing system
func (im *IndexingManager) Start() error {
	return im.indexer.Start()
}

// Stop stops the indexing system
func (im *IndexingManager) Stop() error {
	return im.indexer.Stop()
}

// GetQueryManager returns the query manager
func (im *IndexingManager) GetQueryManager() FastSCIPQuery {
	return im.query
}

// GetIndexer returns the indexer
func (im *IndexingManager) GetIndexer() *SCIPIndexer {
	return im.indexer
}

// IndexFile requests indexing for a specific file
func (im *IndexingManager) IndexFile(uri, language string, priority int) {
	im.indexer.RequestIndexing(uri, language, priority)
}

// IsReady returns true if the indexing system is ready to serve queries
func (im *IndexingManager) IsReady() bool {
	return im.query.IsHealthy() && im.indexer.running
}

// GetHealthStatus returns detailed health information
func (im *IndexingManager) GetHealthStatus() map[string]interface{} {
	queryMetrics := im.query.GetMetrics()
	indexStats := im.indexer.GetStats()

	return map[string]interface{}{
		"query_manager": map[string]interface{}{
			"healthy":           im.query.IsHealthy(),
			"total_queries":     queryMetrics.TotalQueries,
			"cache_hits":        queryMetrics.CacheHits,
			"cache_misses":      queryMetrics.CacheMisses,
			"avg_response_time": queryMetrics.AvgResponseTime.String(),
			"hit_ratio":         float64(queryMetrics.CacheHits) / float64(queryMetrics.TotalQueries),
		},
		"indexer": map[string]interface{}{
			"running":            im.indexer.running,
			"total_jobs":         indexStats.TotalJobs,
			"completed_jobs":     indexStats.CompletedJobs,
			"failed_jobs":        indexStats.FailedJobs,
			"documents_indexed":  indexStats.DocumentsIndexed,
			"symbols_extracted":  indexStats.SymbolsExtracted,
			"avg_index_time":     indexStats.AverageIndexTime.String(),
			"last_indexing_time": indexStats.LastIndexingTime.Format(time.RFC3339),
			"strategy":           indexStats.IndexBuildStrategy,
		},
	}
}
