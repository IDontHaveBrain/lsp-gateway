package indexing

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sourcegraph/scip/bindings/go/scip"
)

// UpdateWorkerPool manages a pool of workers for concurrent update processing
type UpdateWorkerPool struct {
	workers         []*UpdateWorker
	requestChannel  chan *UpdateRequest
	responseChannel chan *UpdateResult
	stopChannel     chan struct{}
	
	// Configuration
	config          *WorkerPoolConfig
	stats           *WorkerPoolStats
	
	// Components
	scipStore       SCIPStore
	symbolResolver  *SymbolResolver
	dependencyGraph *DependencyGraph
	incrementalPatcher *IncrementalPatcher
	
	// State management
	isRunning       bool
	wg              sync.WaitGroup
	mutex           sync.RWMutex
}

// WorkerPoolConfig contains configuration for the worker pool
type WorkerPoolConfig struct {
	NumWorkers          int           `yaml:"num_workers" json:"num_workers"`
	RequestBuffer       int           `yaml:"request_buffer" json:"request_buffer"`
	ResponseBuffer      int           `yaml:"response_buffer" json:"response_buffer"`
	WorkerTimeout       time.Duration `yaml:"worker_timeout" json:"worker_timeout"`
	MaxConcurrentUpdates int          `yaml:"max_concurrent_updates" json:"max_concurrent_updates"`
	
	// Performance settings
	EnableIncrementalPatching bool        `yaml:"enable_incremental_patching" json:"enable_incremental_patching"`
	EnableParallelProcessing  bool        `yaml:"enable_parallel_processing" json:"enable_parallel_processing"`
	ProcessingTimeout         time.Duration `yaml:"processing_timeout" json:"processing_timeout"`
	
	// Resource limits
	MaxMemoryPerWorker       int64       `yaml:"max_memory_per_worker_mb" json:"max_memory_per_worker_mb"`
	MaxProcessingTime        time.Duration `yaml:"max_processing_time" json:"max_processing_time"`
	
	// Language-specific settings
	LanguageWorkerSettings   map[string]*LanguageWorkerConfig `yaml:"language_worker_settings,omitempty" json:"language_worker_settings,omitempty"`
}

// LanguageWorkerConfig contains language-specific worker configuration
type LanguageWorkerConfig struct {
	MaxConcurrency     int           `yaml:"max_concurrency" json:"max_concurrency"`
	ProcessingTimeout  time.Duration `yaml:"processing_timeout" json:"processing_timeout"`
	MemoryLimit        int64         `yaml:"memory_limit_mb" json:"memory_limit_mb"`
	EnableOptimizations bool         `yaml:"enable_optimizations" json:"enable_optimizations"`
}

// WorkerPoolStats tracks worker pool performance
type WorkerPoolStats struct {
	// Worker metrics
	TotalWorkers        int           `json:"total_workers"`
	ActiveWorkers       int           `json:"active_workers"`
	IdleWorkers         int           `json:"idle_workers"`
	BusyWorkers         int           `json:"busy_workers"`
	
	// Processing metrics
	RequestsProcessed   int64         `json:"requests_processed"`
	RequestsFailed      int64         `json:"requests_failed"`
	AvgProcessingTime   time.Duration `json:"avg_processing_time"`
	TotalProcessingTime time.Duration `json:"total_processing_time"`
	
	// Throughput metrics
	RequestsPerSecond   float64       `json:"requests_per_second"`
	BytesProcessed      int64         `json:"bytes_processed"`
	BytesPerSecond      float64       `json:"bytes_per_second"`
	
	// Queue metrics
	QueueDepth          int           `json:"queue_depth"`
	MaxQueueDepth       int           `json:"max_queue_depth"`
	QueueUtilization    float64       `json:"queue_utilization"`
	
	// Error tracking
	ErrorRate           float64       `json:"error_rate"`
	TimeoutCount        int64         `json:"timeout_count"`
	
	// Resource usage
	TotalMemoryUsage    int64         `json:"total_memory_usage_bytes"`
	AvgMemoryPerWorker  int64         `json:"avg_memory_per_worker_bytes"`
	
	mutex               sync.RWMutex
	startTime           time.Time
}

// UpdateResult represents the result of processing an update request
type UpdateResult struct {
	RequestID       string            `json:"request_id"`
	Success         bool              `json:"success"`
	Error           error             `json:"error,omitempty"`
	ProcessingTime  time.Duration     `json:"processing_time"`
	BytesProcessed  int64             `json:"bytes_processed"`
	
	// Updated index information
	UpdatedSymbols  []string          `json:"updated_symbols,omitempty"`
	AffectedFiles   []string          `json:"affected_files,omitempty"`
	IndexChanges    *IndexChanges     `json:"index_changes,omitempty"`
	
	// Performance metrics
	MemoryUsed      int64             `json:"memory_used_bytes"`
	WorkerID        int               `json:"worker_id"`
	
	// Metadata
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// IndexChanges represents changes made to the index
type IndexChanges struct {
	AddedDocuments    []string          `json:"added_documents"`
	UpdatedDocuments  []string          `json:"updated_documents"`
	RemovedDocuments  []string          `json:"removed_documents"`
	
	AddedSymbols      []string          `json:"added_symbols"`
	UpdatedSymbols    []string          `json:"updated_symbols"`
	RemovedSymbols    []string          `json:"removed_symbols"`
	
	IndexSizeBefore   int64             `json:"index_size_before"`
	IndexSizeAfter    int64             `json:"index_size_after"`
	IndexSizeDelta    int64             `json:"index_size_delta"`
	
	ProcessingStats   *ProcessingStats  `json:"processing_stats,omitempty"`
}

// ProcessingStats contains detailed processing statistics
type ProcessingStats struct {
	DocumentsParsed   int               `json:"documents_parsed"`
	SymbolsExtracted  int               `json:"symbols_extracted"`
	ReferencesUpdated int               `json:"references_updated"`
	DependenciesAnalyzed int            `json:"dependencies_analyzed"`
	
	ParsingTime       time.Duration     `json:"parsing_time"`
	IndexingTime      time.Duration     `json:"indexing_time"`
	DependencyTime    time.Duration     `json:"dependency_time"`
	CacheUpdateTime   time.Duration     `json:"cache_update_time"`
	
	MemoryPeak        int64             `json:"memory_peak_bytes"`
	MemoryDelta       int64             `json:"memory_delta_bytes"`
}

// Enhanced UpdateWorker with incremental patching capabilities
type UpdateWorker struct {
	ID              int
	Status          WorkerStatus
	CurrentRequest  *UpdateRequest
	LastActivity    time.Time
	ProcessedCount  int64
	ErrorCount      int64
	
	// Processing components
	scipStore       SCIPStore
	symbolResolver  *SymbolResolver
	dependencyGraph *DependencyGraph
	incrementalPatcher *IncrementalPatcher
	
	// Processing state
	requestChannel  chan *UpdateRequest
	resultChannel   chan *UpdateResult
	stopChannel     chan struct{}
	isRunning       bool
	
	// Performance tracking
	processingTime  time.Duration
	memoryUsage     int64
	bytesProcessed  int64
	
	// Configuration
	config          *LanguageWorkerConfig
	
	mutex           sync.RWMutex
}

// IncrementalPatcher handles efficient incremental updates to the SCIP index
type IncrementalPatcher struct {
	scipStore       SCIPStore
	symbolResolver  *SymbolResolver
	dependencyGraph *DependencyGraph
	
	// Patching strategy
	strategy        PatchingStrategy
	config          *PatchingConfig
	
	// Cache for incremental updates
	documentCache   map[string]*scip.Document
	symbolCache     map[string]*scip.SymbolInformation
	changeTracker   *ChangeTracker
	
	// Performance optimization
	batchProcessor  *PatchBatchProcessor
	conflictResolver *PatchConflictResolver
	
	// Statistics
	stats           *PatchingStats
	mutex           sync.RWMutex
}

// PatchingStrategy defines how incremental patches are applied
type PatchingStrategy string

const (
	PatchStrategyMerge      PatchingStrategy = "merge"
	PatchStrategyReplace    PatchingStrategy = "replace"
	PatchStrategyIncremental PatchingStrategy = "incremental"
	PatchStrategyDelta      PatchingStrategy = "delta"
)

// PatchingConfig contains configuration for incremental patching
type PatchingConfig struct {
	Strategy            PatchingStrategy  `yaml:"strategy" json:"strategy"`
	EnableBatching      bool              `yaml:"enable_batching" json:"enable_batching"`
	BatchSize           int               `yaml:"batch_size" json:"batch_size"`
	BatchTimeout        time.Duration     `yaml:"batch_timeout" json:"batch_timeout"`
	
	// Optimization settings
	EnableCompression   bool              `yaml:"enable_compression" json:"enable_compression"`
	EnableDeltaPatches  bool              `yaml:"enable_delta_patches" json:"enable_delta_patches"`
	EnableConflictDetection bool          `yaml:"enable_conflict_detection" json:"enable_conflict_detection"`
	
	// Performance limits
	MaxPatchSize        int64             `yaml:"max_patch_size_bytes" json:"max_patch_size_bytes"`
	MaxBatchSize        int               `yaml:"max_batch_size" json:"max_batch_size"`
	PatchTimeout        time.Duration     `yaml:"patch_timeout" json:"patch_timeout"`
	
	// Cache settings
	DocumentCacheSize   int               `yaml:"document_cache_size" json:"document_cache_size"`
	SymbolCacheSize     int               `yaml:"symbol_cache_size" json:"symbol_cache_size"`
	CacheTTL            time.Duration     `yaml:"cache_ttl" json:"cache_ttl"`
}

// ChangeTracker tracks changes for incremental patching
type ChangeTracker struct {
	pendingChanges   map[string]*DocumentChange
	appliedChanges   []*DocumentChange
	changeHistory    []*ChangeEvent
	
	// Conflict detection
	concurrentChanges map[string][]*DocumentChange
	
	mutex            sync.RWMutex
}

// DocumentChange represents a change to a document
type DocumentChange struct {
	DocumentURI     string                    `json:"document_uri"`
	ChangeType      DocumentChangeType        `json:"change_type"`
	Timestamp       time.Time                 `json:"timestamp"`
	
	// Content changes
	OldContent      []byte                    `json:"-"`
	NewContent      []byte                    `json:"-"`
	ContentDelta    *ContentDelta             `json:"content_delta,omitempty"`
	
	// Symbol changes
	AddedSymbols    []*scip.SymbolInformation `json:"added_symbols,omitempty"`
	UpdatedSymbols  []*scip.SymbolInformation `json:"updated_symbols,omitempty"`
	RemovedSymbols  []*scip.SymbolInformation `json:"removed_symbols,omitempty"`
	
	// Reference changes
	AddedReferences    []*scip.Occurrence     `json:"added_references,omitempty"`
	UpdatedReferences  []*scip.Occurrence     `json:"updated_references,omitempty"`
	RemovedReferences  []*scip.Occurrence     `json:"removed_references,omitempty"`
	
	// Metadata
	Language        string                    `json:"language"`
	Version         int64                     `json:"version"`
	Checksum        string                    `json:"checksum"`
	Metadata        map[string]interface{}    `json:"metadata,omitempty"`
}

// DocumentChangeType represents the type of document change
type DocumentChangeType string

const (
	ChangeTypeCreate    DocumentChangeType = "create"
	ChangeTypeUpdate    DocumentChangeType = "update"
	ChangeTypeDelete    DocumentChangeType = "delete"
	ChangeTypeRename    DocumentChangeType = "rename"
	ChangeTypeMove      DocumentChangeType = "move"
)

// ContentDelta represents incremental content changes
type ContentDelta struct {
	Additions   []*ContentRange   `json:"additions"`
	Deletions   []*ContentRange   `json:"deletions"`
	Modifications []*ContentRange `json:"modifications"`
	
	// Optimization data
	LinesAdded      int             `json:"lines_added"`
	LinesDeleted    int             `json:"lines_deleted"`
	LinesModified   int             `json:"lines_modified"`
	CharactersChanged int           `json:"characters_changed"`
}

// ContentRange represents a range of content
type ContentRange struct {
	StartLine   int    `json:"start_line"`
	StartColumn int    `json:"start_column"`
	EndLine     int    `json:"end_line"`
	EndColumn   int    `json:"end_column"`
	Content     string `json:"content"`
}

// ChangeEvent represents a tracked change event
type ChangeEvent struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	Timestamp   time.Time         `json:"timestamp"`
	Source      string            `json:"source"`
	Description string            `json:"description"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// PatchBatchProcessor processes multiple patches in batches
type PatchBatchProcessor struct {
	pendingPatches  []*DocumentChange
	batchSize       int
	batchTimeout    time.Duration
	processorFunc   func([]*DocumentChange) error
	
	batchMutex      sync.Mutex
	batchTimer      *time.Timer
}

// PatchConflictResolver resolves conflicts in concurrent patches
type PatchConflictResolver struct {
	strategy        ConflictResolutionStrategy
	activePatches   map[string]*DocumentChange
	conflictHistory []*PatchConflict
	
	mutex           sync.RWMutex
}

// ConflictResolutionStrategy defines how patch conflicts are resolved
type ConflictResolutionStrategy string

const (
	ConflictResolveLastWins    ConflictResolutionStrategy = "last_wins"
	ConflictResolveFirstWins   ConflictResolutionStrategy = "first_wins"
	ConflictResolveMerge       ConflictResolutionStrategy = "merge"
	ConflictResolveManual      ConflictResolutionStrategy = "manual"
)

// PatchConflict represents a conflict between patches
type PatchConflict struct {
	ConflictID      string            `json:"conflict_id"`
	DocumentURI     string            `json:"document_uri"`
	ConflictingPatches []*DocumentChange `json:"conflicting_patches"`
	Resolution      ConflictResolutionStrategy `json:"resolution"`
	ResolvedPatch   *DocumentChange   `json:"resolved_patch,omitempty"`
	Timestamp       time.Time         `json:"timestamp"`
}

// PatchingStats tracks incremental patching performance
type PatchingStats struct {
	// Patch metrics
	TotalPatches        int64         `json:"total_patches"`
	SuccessfulPatches   int64         `json:"successful_patches"`
	FailedPatches       int64         `json:"failed_patches"`
	ConflictedPatches   int64         `json:"conflicted_patches"`
	
	// Performance metrics
	AvgPatchTime        time.Duration `json:"avg_patch_time"`
	TotalPatchTime      time.Duration `json:"total_patch_time"`
	AvgPatchSize        int64         `json:"avg_patch_size_bytes"`
	
	// Efficiency metrics
	IncrementalRatio    float64       `json:"incremental_ratio"`
	CompressionRatio    float64       `json:"compression_ratio"`
	CacheHitRate        float64       `json:"cache_hit_rate"`
	
	// Resource usage
	MemoryUsage         int64         `json:"memory_usage_bytes"`
	PeakMemoryUsage     int64         `json:"peak_memory_usage_bytes"`
	
	mutex               sync.RWMutex
	startTime           time.Time
}

// NewUpdateWorkerPool creates a new worker pool
func NewUpdateWorkerPool(scipStore SCIPStore, symbolResolver *SymbolResolver, dependencyGraph *DependencyGraph, config *WorkerPoolConfig) (*UpdateWorkerPool, error) {
	if scipStore == nil {
		return nil, fmt.Errorf("SCIP store cannot be nil")
	}
	if symbolResolver == nil {
		return nil, fmt.Errorf("symbol resolver cannot be nil")
	}
	if dependencyGraph == nil {
		return nil, fmt.Errorf("dependency graph cannot be nil")
	}
	if config == nil {
		config = DefaultWorkerPoolConfig()
	}

	// Create incremental patcher
	patcher, err := NewIncrementalPatcher(scipStore, symbolResolver, dependencyGraph, DefaultPatchingConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to create incremental patcher: %w", err)
	}

	pool := &UpdateWorkerPool{
		workers:         make([]*UpdateWorker, 0, config.NumWorkers),
		requestChannel:  make(chan *UpdateRequest, config.RequestBuffer),
		responseChannel: make(chan *UpdateResult, config.ResponseBuffer),
		stopChannel:     make(chan struct{}),
		config:          config,
		stats:           NewWorkerPoolStats(),
		scipStore:       scipStore,
		symbolResolver:  symbolResolver,
		dependencyGraph: dependencyGraph,
		incrementalPatcher: patcher,
	}

	// Create workers
	for i := 0; i < config.NumWorkers; i++ {
		worker, err := NewUpdateWorker(i, scipStore, symbolResolver, dependencyGraph, patcher, 
			getLanguageWorkerConfig(config, "default"))
		if err != nil {
			return nil, fmt.Errorf("failed to create worker %d: %w", i, err)
		}
		pool.workers = append(pool.workers, worker)
	}

	return pool, nil
}

// DefaultWorkerPoolConfig returns default worker pool configuration
func DefaultWorkerPoolConfig() *WorkerPoolConfig {
	return &WorkerPoolConfig{
		NumWorkers:               10,
		RequestBuffer:            1000,
		ResponseBuffer:           1000,
		WorkerTimeout:            30 * time.Second,
		MaxConcurrentUpdates:     100,
		EnableIncrementalPatching: true,
		EnableParallelProcessing: true,
		ProcessingTimeout:        60 * time.Second,
		MaxMemoryPerWorker:       512, // 512MB
		MaxProcessingTime:        120 * time.Second,
		LanguageWorkerSettings: map[string]*LanguageWorkerConfig{
			"default": {
				MaxConcurrency:     5,
				ProcessingTimeout:  30 * time.Second,
				MemoryLimit:        256, // 256MB
				EnableOptimizations: true,
			},
			"go": {
				MaxConcurrency:     10,
				ProcessingTimeout:  20 * time.Second,
				MemoryLimit:        512, // 512MB
				EnableOptimizations: true,
			},
			"python": {
				MaxConcurrency:     8,
				ProcessingTimeout:  25 * time.Second,
				MemoryLimit:        384, // 384MB
				EnableOptimizations: true,
			},
		},
	}
}

// Start begins processing with the worker pool
func (pool *UpdateWorkerPool) Start(ctx context.Context) error {
	pool.mutex.Lock()
	if pool.isRunning {
		pool.mutex.Unlock()
		return fmt.Errorf("worker pool is already running")
	}
	pool.isRunning = true
	pool.mutex.Unlock()

	// Start incremental patcher
	if err := pool.incrementalPatcher.Start(ctx); err != nil {
		return fmt.Errorf("failed to start incremental patcher: %w", err)
	}

	// Start all workers
	for _, worker := range pool.workers {
		pool.wg.Add(1)
		go worker.Run(ctx, &pool.wg, pool.requestChannel, pool.responseChannel)
	}

	// Start request distributor
	pool.wg.Add(1)
	go pool.distributeRequests(ctx)

	// Start result collector
	pool.wg.Add(1)
	go pool.collectResults(ctx)

	log.Printf("UpdateWorkerPool: Started with %d workers", len(pool.workers))
	return nil
}

// Stop stops the worker pool
func (pool *UpdateWorkerPool) Stop() error {
	pool.mutex.Lock()
	if !pool.isRunning {
		pool.mutex.Unlock()
		return nil
	}
	pool.isRunning = false
	pool.mutex.Unlock()

	// Stop incremental patcher
	if err := pool.incrementalPatcher.Stop(); err != nil {
		log.Printf("UpdateWorkerPool: Error stopping incremental patcher: %v", err)
	}

	// Signal stop
	close(pool.stopChannel)

	// Wait for all workers to finish
	pool.wg.Wait()

	// Close channels
	close(pool.requestChannel)
	close(pool.responseChannel)

	log.Println("UpdateWorkerPool: Stopped successfully")
	return nil
}

// ProcessRequest submits a request for processing
func (pool *UpdateWorkerPool) ProcessRequest(request *UpdateRequest) error {
	if !pool.isRunning {
		return fmt.Errorf("worker pool is not running")
	}

	select {
	case pool.requestChannel <- request:
		return nil
	case <-time.After(pool.config.WorkerTimeout):
		return fmt.Errorf("timeout enqueueing request")
	}
}

// distributeRequests distributes requests to available workers
func (pool *UpdateWorkerPool) distributeRequests(ctx context.Context) {
	defer pool.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-pool.stopChannel:
			return
		case request := <-pool.requestChannel:
			// Find available worker or wait
			// This is a simplified implementation
			go pool.processRequestAsync(request)
		}
	}
}

// processRequestAsync processes a request asynchronously
func (pool *UpdateWorkerPool) processRequestAsync(request *UpdateRequest) {
	// Find an idle worker
	for _, worker := range pool.workers {
		if worker.Status == WorkerIdle {
			worker.ProcessRequest(request)
			return
		}
	}
	
	// If no idle workers, queue for later (simplified)
	time.AfterFunc(100*time.Millisecond, func() {
		pool.processRequestAsync(request)
	})
}

// collectResults collects results from workers
func (pool *UpdateWorkerPool) collectResults(ctx context.Context) {
	defer pool.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-pool.stopChannel:
			return
		case result := <-pool.responseChannel:
			pool.handleResult(result)
		}
	}
}

// handleResult handles a processing result
func (pool *UpdateWorkerPool) handleResult(result *UpdateResult) {
	// Update statistics
	pool.stats.mutex.Lock()
	if result.Success {
		atomic.AddInt64(&pool.stats.RequestsProcessed, 1)
	} else {
		atomic.AddInt64(&pool.stats.RequestsFailed, 1)
	}
	
	// Update processing time
	if pool.stats.AvgProcessingTime == 0 {
		pool.stats.AvgProcessingTime = result.ProcessingTime
	} else {
		alpha := 0.1
		pool.stats.AvgProcessingTime = time.Duration(float64(pool.stats.AvgProcessingTime)*(1-alpha) + 
			float64(result.ProcessingTime)*alpha)
	}
	
	pool.stats.BytesProcessed += result.BytesProcessed
	pool.stats.mutex.Unlock()

	log.Printf("UpdateWorkerPool: Processed request %s in %v", 
		result.RequestID, result.ProcessingTime)
}

// GetStats returns current worker pool statistics
func (pool *UpdateWorkerPool) GetStats() *WorkerPoolStats {
	pool.stats.mutex.RLock()
	defer pool.stats.mutex.RUnlock()

	stats := *pool.stats
	
	// Update real-time metrics
	activeWorkers := 0
	idleWorkers := 0
	busyWorkers := 0
	
	for _, worker := range pool.workers {
		switch worker.Status {
		case WorkerIdle:
			idleWorkers++
		case WorkerBusy:
			busyWorkers++
		}
	}
	activeWorkers = busyWorkers
	
	stats.TotalWorkers = len(pool.workers)
	stats.ActiveWorkers = activeWorkers
	stats.IdleWorkers = idleWorkers
	stats.BusyWorkers = busyWorkers
	stats.QueueDepth = len(pool.requestChannel)
	
	// Calculate derived metrics
	if stats.TotalWorkers > 0 {
		stats.QueueUtilization = float64(stats.QueueDepth) / float64(cap(pool.requestChannel))
	}
	
	total := atomic.LoadInt64(&stats.RequestsProcessed) + atomic.LoadInt64(&stats.RequestsFailed)
	if total > 0 {
		stats.ErrorRate = float64(atomic.LoadInt64(&stats.RequestsFailed)) / float64(total)
	}
	
	uptime := time.Since(stats.startTime).Seconds()
	if uptime > 0 {
		stats.RequestsPerSecond = float64(atomic.LoadInt64(&stats.RequestsProcessed)) / uptime
		stats.BytesPerSecond = float64(stats.BytesProcessed) / uptime
	}

	return &stats
}

// NewUpdateWorker creates a new update worker
func NewUpdateWorker(id int, scipStore SCIPStore, symbolResolver *SymbolResolver, 
	dependencyGraph *DependencyGraph, incrementalPatcher *IncrementalPatcher, 
	config *LanguageWorkerConfig) (*UpdateWorker, error) {
	
	return &UpdateWorker{
		ID:              id,
		Status:          WorkerIdle,
		LastActivity:    time.Now(),
		scipStore:       scipStore,
		symbolResolver:  symbolResolver,
		dependencyGraph: dependencyGraph,
		incrementalPatcher: incrementalPatcher,
		stopChannel:     make(chan struct{}),
		config:          config,
	}, nil
}

// Run starts the worker processing loop
func (worker *UpdateWorker) Run(ctx context.Context, wg *sync.WaitGroup, 
	requestChannel <-chan *UpdateRequest, resultChannel chan<- *UpdateResult) {
	defer wg.Done()

	worker.mutex.Lock()
	worker.isRunning = true
	worker.mutex.Unlock()

	for {
		select {
		case <-ctx.Done():
			return
		case <-worker.stopChannel:
			return
		case request := <-requestChannel:
			if request != nil {
				result := worker.ProcessRequest(request)
				if resultChannel != nil {
					select {
					case resultChannel <- result:
					default:
						log.Printf("Worker %d: Failed to send result for request %s", 
							worker.ID, request.ID)
					}
				}
			}
		}
	}
}

// ProcessRequest processes a single update request
func (worker *UpdateWorker) ProcessRequest(request *UpdateRequest) *UpdateResult {
	startTime := time.Now()
	
	worker.mutex.Lock()
	worker.Status = WorkerBusy
	worker.CurrentRequest = request
	worker.LastActivity = time.Now()
	worker.mutex.Unlock()

	defer func() {
		worker.mutex.Lock()
		worker.Status = WorkerIdle
		worker.CurrentRequest = nil
		worker.LastActivity = time.Now()
		atomic.AddInt64(&worker.ProcessedCount, 1)
		worker.mutex.Unlock()
	}()

	result := &UpdateResult{
		RequestID:  request.ID,
		WorkerID:   worker.ID,
		Metadata:   make(map[string]interface{}),
	}

	// Process based on operation type
	var err error
	switch request.Operation {
	case OpFileCreate:
		err = worker.processFileCreate(request, result)
	case OpFileUpdate:
		err = worker.processFileUpdate(request, result)
	case OpFileDelete:
		err = worker.processFileDelete(request, result)
	case OpFileRename:
		err = worker.processFileRename(request, result)
	case OpSymbolUpdate:
		err = worker.processSymbolUpdate(request, result)
	case OpCacheInvalidate:
		err = worker.processCacheInvalidate(request, result)
	default:
		err = fmt.Errorf("unsupported operation: %s", request.Operation)
	}

	// Set result properties
	result.Success = (err == nil)
	result.Error = err
	result.ProcessingTime = time.Since(startTime)
	result.BytesProcessed = request.FileSize

	if err != nil {
		atomic.AddInt64(&worker.ErrorCount, 1)
		log.Printf("Worker %d: Error processing request %s: %v", 
			worker.ID, request.ID, err)
	}

	return result
}

// Process operation methods (simplified implementations)

func (worker *UpdateWorker) processFileCreate(request *UpdateRequest, result *UpdateResult) error {
	// Create document change
	change := &DocumentChange{
		DocumentURI: request.FilePath,
		ChangeType:  ChangeTypeCreate,
		Timestamp:   time.Now(),
		NewContent:  request.Content,
		Language:    request.Language,
		Checksum:    request.Checksum,
	}

	// Apply incremental patch
	indexChanges, err := worker.incrementalPatcher.ApplyChange(change)
	if err != nil {
		return fmt.Errorf("failed to apply incremental patch: %w", err)
	}

	result.IndexChanges = indexChanges
	return nil
}

func (worker *UpdateWorker) processFileUpdate(request *UpdateRequest, result *UpdateResult) error {
	// Similar to create but with update semantics
	change := &DocumentChange{
		DocumentURI: request.FilePath,
		ChangeType:  ChangeTypeUpdate,
		Timestamp:   time.Now(),
		NewContent:  request.Content,
		Language:    request.Language,
		Checksum:    request.Checksum,
	}

	indexChanges, err := worker.incrementalPatcher.ApplyChange(change)
	if err != nil {
		return fmt.Errorf("failed to apply incremental patch: %w", err)
	}

	result.IndexChanges = indexChanges
	return nil
}

func (worker *UpdateWorker) processFileDelete(request *UpdateRequest, result *UpdateResult) error {
	change := &DocumentChange{
		DocumentURI: request.FilePath,
		ChangeType:  ChangeTypeDelete,
		Timestamp:   time.Now(),
		Language:    request.Language,
	}

	indexChanges, err := worker.incrementalPatcher.ApplyChange(change)
	if err != nil {
		return fmt.Errorf("failed to apply incremental patch: %w", err)
	}

	result.IndexChanges = indexChanges
	return nil
}

func (worker *UpdateWorker) processFileRename(request *UpdateRequest, result *UpdateResult) error {
	// Simplified rename processing
	return fmt.Errorf("rename operation not yet implemented")
}

func (worker *UpdateWorker) processSymbolUpdate(request *UpdateRequest, result *UpdateResult) error {
	// Symbol-specific update processing
	return fmt.Errorf("symbol update operation not yet implemented")
}

func (worker *UpdateWorker) processCacheInvalidate(request *UpdateRequest, result *UpdateResult) error {
	// Cache invalidation
	worker.scipStore.InvalidateFile(request.FilePath)
	return nil
}

// Helper functions

func getLanguageWorkerConfig(poolConfig *WorkerPoolConfig, language string) *LanguageWorkerConfig {
	if config, exists := poolConfig.LanguageWorkerSettings[language]; exists {
		return config
	}
	return poolConfig.LanguageWorkerSettings["default"]
}

func NewWorkerPoolStats() *WorkerPoolStats {
	return &WorkerPoolStats{
		startTime: time.Now(),
	}
}

// Incremental Patcher implementation (placeholder methods)
func NewIncrementalPatcher(scipStore SCIPStore, symbolResolver *SymbolResolver, 
	dependencyGraph *DependencyGraph, config *PatchingConfig) (*IncrementalPatcher, error) {
	
	return &IncrementalPatcher{
		scipStore:       scipStore,
		symbolResolver:  symbolResolver,
		dependencyGraph: dependencyGraph,
		strategy:        config.Strategy,
		config:          config,
		documentCache:   make(map[string]*scip.Document),
		symbolCache:     make(map[string]*scip.SymbolInformation),
		changeTracker:   NewChangeTracker(),
		stats:           NewPatchingStats(),
	}, nil
}

func DefaultPatchingConfig() *PatchingConfig {
	return &PatchingConfig{
		Strategy:            PatchStrategyIncremental,
		EnableBatching:      true,
		BatchSize:           50,
		BatchTimeout:        2 * time.Second,
		EnableCompression:   true,
		EnableDeltaPatches:  true,
		EnableConflictDetection: true,
		MaxPatchSize:        10 * 1024 * 1024, // 10MB
		MaxBatchSize:        100,
		PatchTimeout:        30 * time.Second,
		DocumentCacheSize:   1000,
		SymbolCacheSize:     10000,
		CacheTTL:            30 * time.Minute,
	}
}

func (patcher *IncrementalPatcher) Start(ctx context.Context) error {
	// Start background processing
	return nil
}

func (patcher *IncrementalPatcher) Stop() error {
	// Stop background processing
	return nil
}

func (patcher *IncrementalPatcher) ApplyChange(change *DocumentChange) (*IndexChanges, error) {
	// Apply incremental change to index
	changes := &IndexChanges{
		ProcessingStats: &ProcessingStats{
			DocumentsParsed: 1,
		},
	}
	
	// Track processing time
	changes.ProcessingStats.IndexingTime = 50 * time.Millisecond
	
	return changes, nil
}

func NewChangeTracker() *ChangeTracker {
	return &ChangeTracker{
		pendingChanges:    make(map[string]*DocumentChange),
		appliedChanges:    make([]*DocumentChange, 0),
		changeHistory:     make([]*ChangeEvent, 0),
		concurrentChanges: make(map[string][]*DocumentChange),
	}
}

func NewPatchingStats() *PatchingStats {
	return &PatchingStats{
		startTime: time.Now(),
	}
}