package indexing

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// WatcherIntegration provides integration between FileSystemWatcher and SCIP infrastructure
type WatcherIntegration struct {
	scipStore      SCIPStore
	symbolResolver *SymbolResolver
	config         *WatcherIntegrationConfig
	stats          *IntegrationStats

	// Processing queues
	invalidationQueue chan string
	indexUpdateQueue  chan *IndexUpdateRequest

	// Background processing
	stopChan chan struct{}
	wg       sync.WaitGroup

	// Debouncing for related file changes
	pendingUpdates map[string]*PendingUpdate
	updateMutex    sync.RWMutex
	updateTimer    *time.Timer

	// Performance optimization
	lastInvalidation         time.Time
	batchInvalidationEnabled bool

	mutex sync.RWMutex
}

// WatcherIntegrationConfig contains configuration for SCIP integration
type WatcherIntegrationConfig struct {
	// Cache invalidation settings
	EnableCacheInvalidation bool          `yaml:"enable_cache_invalidation" json:"enable_cache_invalidation"`
	InvalidationDebounce    time.Duration `yaml:"invalidation_debounce" json:"invalidation_debounce"`
	BatchInvalidation       bool          `yaml:"batch_invalidation" json:"batch_invalidation"`

	// Index update settings
	EnableIndexUpdates  bool          `yaml:"enable_index_updates" json:"enable_index_updates"`
	IndexUpdateDebounce time.Duration `yaml:"index_update_debounce" json:"index_update_debounce"`
	IncrementalUpdates  bool          `yaml:"incremental_updates" json:"incremental_updates"`

	// Re-indexing settings
	EnableReindexing    bool `yaml:"enable_reindexing" json:"enable_reindexing"`
	ReindexingThreshold int  `yaml:"reindexing_threshold" json:"reindexing_threshold"`
	ReindexingBatchSize int  `yaml:"reindexing_batch_size" json:"reindexing_batch_size"`

	// Language-specific settings
	LanguageSpecificSettings map[string]*LanguageIntegrationConfig `yaml:"language_settings,omitempty" json:"language_settings,omitempty"`

	// Performance settings
	MaxConcurrentUpdates int           `yaml:"max_concurrent_updates" json:"max_concurrent_updates"`
	UpdateTimeout        time.Duration `yaml:"update_timeout" json:"update_timeout"`
	QueueSize            int           `yaml:"queue_size" json:"queue_size"`
}

// LanguageIntegrationConfig contains language-specific integration settings
type LanguageIntegrationConfig struct {
	EnableCacheInvalidation bool          `yaml:"enable_cache_invalidation" json:"enable_cache_invalidation"`
	EnableIndexUpdates      bool          `yaml:"enable_index_updates" json:"enable_index_updates"`
	Priority                int           `yaml:"priority" json:"priority"`
	DebounceInterval        time.Duration `yaml:"debounce_interval" json:"debounce_interval"`
	RequiresReindexing      bool          `yaml:"requires_reindexing" json:"requires_reindexing"`
}

// IndexUpdateRequest represents a request to update the symbol index
type IndexUpdateRequest struct {
	FilePath  string                 `json:"file_path"`
	Language  string                 `json:"language"`
	Operation string                 `json:"operation"`
	Timestamp time.Time              `json:"timestamp"`
	Priority  int                    `json:"priority"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`

	// Dependencies and related files
	Dependencies []string `json:"dependencies,omitempty"`
	RelatedFiles []string `json:"related_files,omitempty"`

	// Processing context
	RetryCount  int       `json:"retry_count,omitempty"`
	LastAttempt time.Time `json:"last_attempt,omitempty"`
}

// PendingUpdate represents a pending update that's being debounced
type PendingUpdate struct {
	Request     *IndexUpdateRequest
	LastUpdate  time.Time
	UpdateCount int
}

// IntegrationStats tracks integration performance and metrics
type IntegrationStats struct {
	// Cache invalidation metrics
	TotalInvalidations  int64         `json:"total_invalidations"`
	BatchInvalidations  int64         `json:"batch_invalidations"`
	AvgInvalidationTime time.Duration `json:"avg_invalidation_time"`

	// Index update metrics
	TotalIndexUpdates int64         `json:"total_index_updates"`
	SuccessfulUpdates int64         `json:"successful_updates"`
	FailedUpdates     int64         `json:"failed_updates"`
	AvgUpdateTime     time.Duration `json:"avg_update_time"`

	// Re-indexing metrics
	ReindexingOperations int64         `json:"reindexing_operations"`
	FilesReindexed       int64         `json:"files_reindexed"`
	AvgReindexingTime    time.Duration `json:"avg_reindexing_time"`

	// Performance metrics
	QueueUtilization    float64       `json:"queue_utilization"`
	ProcessingLatency   time.Duration `json:"processing_latency"`
	ThroughputPerSecond float64       `json:"throughput_per_second"`

	// Error tracking
	ErrorCount    int64     `json:"error_count"`
	LastError     string    `json:"last_error,omitempty"`
	LastErrorTime time.Time `json:"last_error_time,omitempty"`

	mutex     sync.RWMutex
	startTime time.Time
}

// NewWatcherIntegration creates a new watcher integration
func NewWatcherIntegration(scipStore SCIPStore, symbolResolver *SymbolResolver, config *WatcherIntegrationConfig) (*WatcherIntegration, error) {
	if scipStore == nil {
		return nil, fmt.Errorf("SCIPStore cannot be nil")
	}
	if symbolResolver == nil {
		return nil, fmt.Errorf("SymbolResolver cannot be nil")
	}
	if config == nil {
		config = DefaultWatcherIntegrationConfig()
	}

	integration := &WatcherIntegration{
		scipStore:                scipStore,
		symbolResolver:           symbolResolver,
		config:                   config,
		stats:                    NewIntegrationStats(),
		invalidationQueue:        make(chan string, config.QueueSize),
		indexUpdateQueue:         make(chan *IndexUpdateRequest, config.QueueSize),
		stopChan:                 make(chan struct{}),
		pendingUpdates:           make(map[string]*PendingUpdate),
		batchInvalidationEnabled: config.BatchInvalidation,
	}

	return integration, nil
}

// DefaultWatcherIntegrationConfig returns default integration configuration
func DefaultWatcherIntegrationConfig() *WatcherIntegrationConfig {
	return &WatcherIntegrationConfig{
		EnableCacheInvalidation: true,
		InvalidationDebounce:    500 * time.Millisecond,
		BatchInvalidation:       true,
		EnableIndexUpdates:      true,
		IndexUpdateDebounce:     1 * time.Second,
		IncrementalUpdates:      true,
		EnableReindexing:        false, // Conservative default
		ReindexingThreshold:     10,
		ReindexingBatchSize:     50,
		MaxConcurrentUpdates:    5,
		UpdateTimeout:           30 * time.Second,
		QueueSize:               1000,
		LanguageSpecificSettings: map[string]*LanguageIntegrationConfig{
			"go": {
				EnableCacheInvalidation: true,
				EnableIndexUpdates:      true,
				Priority:                1,
				DebounceInterval:        200 * time.Millisecond,
				RequiresReindexing:      true,
			},
			"python": {
				EnableCacheInvalidation: true,
				EnableIndexUpdates:      true,
				Priority:                2,
				DebounceInterval:        500 * time.Millisecond,
				RequiresReindexing:      true,
			},
			"javascript": {
				EnableCacheInvalidation: true,
				EnableIndexUpdates:      true,
				Priority:                2,
				DebounceInterval:        300 * time.Millisecond,
				RequiresReindexing:      false,
			},
			"typescript": {
				EnableCacheInvalidation: true,
				EnableIndexUpdates:      true,
				Priority:                2,
				DebounceInterval:        300 * time.Millisecond,
				RequiresReindexing:      true,
			},
		},
	}
}

// Start begins the integration background processing
func (wi *WatcherIntegration) Start(ctx context.Context) error {
	wi.mutex.Lock()
	defer wi.mutex.Unlock()

	// Start background processors
	wi.wg.Add(2)
	go wi.processInvalidationQueue(ctx)
	go wi.processIndexUpdateQueue(ctx)

	log.Println("WatcherIntegration: Started SCIP integration processing")
	return nil
}

// Stop stops the integration background processing
func (wi *WatcherIntegration) Stop() error {
	wi.mutex.Lock()
	defer wi.mutex.Unlock()

	// Signal stop
	close(wi.stopChan)

	// Wait for background processors to finish
	wi.wg.Wait()

	// Close channels
	close(wi.invalidationQueue)
	close(wi.indexUpdateQueue)

	log.Println("WatcherIntegration: Stopped SCIP integration processing")
	return nil
}

// HandleFileChange handles a file change event and integrates with SCIP infrastructure
func (wi *WatcherIntegration) HandleFileChange(event *FileChangeEvent) error {
	if event == nil {
		return fmt.Errorf("file change event cannot be nil")
	}

	// Handle cache invalidation
	if wi.config.EnableCacheInvalidation && wi.shouldInvalidateCache(event) {
		if err := wi.invalidateCache(event.FilePath); err != nil {
			wi.recordError(fmt.Sprintf("Cache invalidation failed for %s: %v", event.FilePath, err))
		}
	}

	// Handle index updates
	if wi.config.EnableIndexUpdates && wi.shouldUpdateIndex(event) {
		updateRequest := &IndexUpdateRequest{
			FilePath:  event.FilePath,
			Language:  event.Language,
			Operation: event.Operation,
			Timestamp: event.Timestamp,
			Priority:  wi.getUpdatePriority(event),
			Metadata:  event.Metadata,
		}

		if err := wi.scheduleIndexUpdate(updateRequest); err != nil {
			wi.recordError(fmt.Sprintf("Index update scheduling failed for %s: %v", event.FilePath, err))
		}
	}

	// Handle re-indexing if enabled
	if wi.config.EnableReindexing && wi.shouldTriggerReindexing(event) {
		if err := wi.triggerReindexing(event); err != nil {
			wi.recordError(fmt.Sprintf("Re-indexing failed for %s: %v", event.FilePath, err))
		}
	}

	return nil
}

// invalidateCache invalidates cache entries for a file
func (wi *WatcherIntegration) invalidateCache(filePath string) error {
	startTime := time.Now()

	// Queue for batch processing if enabled
	if wi.batchInvalidationEnabled {
		select {
		case wi.invalidationQueue <- filePath:
			// Queued successfully
		default:
			// Queue is full, perform immediate invalidation
			wi.scipStore.InvalidateFile(filePath)
		}
	} else {
		// Immediate invalidation
		wi.scipStore.InvalidateFile(filePath)
	}

	// Update statistics
	wi.stats.mutex.Lock()
	wi.stats.TotalInvalidations++
	if wi.stats.AvgInvalidationTime == 0 {
		wi.stats.AvgInvalidationTime = time.Since(startTime)
	} else {
		alpha := 0.1
		wi.stats.AvgInvalidationTime = time.Duration(float64(wi.stats.AvgInvalidationTime)*(1-alpha) + float64(time.Since(startTime))*alpha)
	}
	wi.stats.mutex.Unlock()

	wi.lastInvalidation = time.Now()
	return nil
}

// scheduleIndexUpdate schedules an index update with debouncing
func (wi *WatcherIntegration) scheduleIndexUpdate(request *IndexUpdateRequest) error {
	// Apply debouncing logic
	if wi.config.IndexUpdateDebounce > 0 {
		wi.updateMutex.Lock()
		defer wi.updateMutex.Unlock()

		// Check if there's already a pending update for this file
		if pending, exists := wi.pendingUpdates[request.FilePath]; exists {
			// Update the existing pending request
			pending.Request = request
			pending.LastUpdate = time.Now()
			pending.UpdateCount++

			// Reset the debounce timer
			if wi.updateTimer != nil {
				wi.updateTimer.Stop()
			}
			wi.updateTimer = time.AfterFunc(wi.config.IndexUpdateDebounce, func() {
				wi.flushPendingUpdates()
			})

			return nil
		}

		// Add new pending update
		wi.pendingUpdates[request.FilePath] = &PendingUpdate{
			Request:     request,
			LastUpdate:  time.Now(),
			UpdateCount: 1,
		}

		// Start or reset debounce timer
		if wi.updateTimer != nil {
			wi.updateTimer.Stop()
		}
		wi.updateTimer = time.AfterFunc(wi.config.IndexUpdateDebounce, func() {
			wi.flushPendingUpdates()
		})

		return nil
	}

	// No debouncing, queue immediately
	select {
	case wi.indexUpdateQueue <- request:
		return nil
	default:
		return fmt.Errorf("index update queue is full")
	}
}

// flushPendingUpdates sends all pending updates to the processing queue
func (wi *WatcherIntegration) flushPendingUpdates() {
	wi.updateMutex.Lock()
	pending := make([]*IndexUpdateRequest, 0, len(wi.pendingUpdates))
	for _, update := range wi.pendingUpdates {
		pending = append(pending, update.Request)
	}
	wi.pendingUpdates = make(map[string]*PendingUpdate)
	wi.updateMutex.Unlock()

	// Queue all pending updates
	for _, request := range pending {
		select {
		case wi.indexUpdateQueue <- request:
			// Queued successfully
		default:
			// Queue is full, drop the update and log error
			wi.recordError(fmt.Sprintf("Dropped index update for %s: queue full", request.FilePath))
		}
	}
}

// processInvalidationQueue processes cache invalidation requests
func (wi *WatcherIntegration) processInvalidationQueue(ctx context.Context) {
	defer wi.wg.Done()

	batch := make([]string, 0, 50)
	batchTimer := time.NewTicker(1 * time.Second)
	defer batchTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			// Process remaining batch
			if len(batch) > 0 {
				wi.processBatchInvalidation(batch)
			}
			return
		case <-wi.stopChan:
			return
		case filePath, ok := <-wi.invalidationQueue:
			if !ok {
				return
			}

			batch = append(batch, filePath)

			// Process batch if it's full
			if len(batch) >= 50 {
				wi.processBatchInvalidation(batch)
				batch = batch[:0]
			}

		case <-batchTimer.C:
			// Process batch on timer
			if len(batch) > 0 {
				wi.processBatchInvalidation(batch)
				batch = batch[:0]
			}
		}
	}
}

// processBatchInvalidation processes a batch of cache invalidations
func (wi *WatcherIntegration) processBatchInvalidation(filePaths []string) {
	startTime := time.Now()

	// Remove duplicates
	uniquePaths := make(map[string]bool)
	for _, path := range filePaths {
		uniquePaths[path] = true
	}

	// Invalidate each unique path
	for path := range uniquePaths {
		wi.scipStore.InvalidateFile(path)
	}

	// Update statistics
	wi.stats.mutex.Lock()
	wi.stats.BatchInvalidations++
	wi.stats.TotalInvalidations += int64(len(uniquePaths))
	wi.stats.mutex.Unlock()

	log.Printf("WatcherIntegration: Processed batch invalidation of %d files in %v",
		len(uniquePaths), time.Since(startTime))
}

// processIndexUpdateQueue processes index update requests
func (wi *WatcherIntegration) processIndexUpdateQueue(ctx context.Context) {
	defer wi.wg.Done()

	// Semaphore for concurrent processing
	semaphore := make(chan struct{}, wi.config.MaxConcurrentUpdates)

	for {
		select {
		case <-ctx.Done():
			return
		case <-wi.stopChan:
			return
		case request, ok := <-wi.indexUpdateQueue:
			if !ok {
				return
			}

			// Acquire semaphore slot
			select {
			case semaphore <- struct{}{}:
				go func(req *IndexUpdateRequest) {
					defer func() { <-semaphore }()
					wi.processIndexUpdate(req)
				}(request)
			case <-ctx.Done():
				return
			}
		}
	}
}

// processIndexUpdate processes a single index update request
func (wi *WatcherIntegration) processIndexUpdate(request *IndexUpdateRequest) {
	startTime := time.Now()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), wi.config.UpdateTimeout)
	defer cancel()

	// Process the update based on operation type
	var err error
	switch request.Operation {
	case FileOpCreate, FileOpWrite:
		err = wi.handleFileCreateOrUpdate(ctx, request)
	case FileOpRemove:
		err = wi.handleFileRemoval(ctx, request)
	case FileOpRename:
		err = wi.handleFileRename(ctx, request)
	default:
		err = fmt.Errorf("unsupported operation: %s", request.Operation)
	}

	// Update statistics
	wi.stats.mutex.Lock()
	wi.stats.TotalIndexUpdates++
	if err != nil {
		wi.stats.FailedUpdates++
		wi.recordError(fmt.Sprintf("Index update failed for %s: %v", request.FilePath, err))
	} else {
		wi.stats.SuccessfulUpdates++
	}

	processingTime := time.Since(startTime)
	if wi.stats.AvgUpdateTime == 0 {
		wi.stats.AvgUpdateTime = processingTime
	} else {
		alpha := 0.1
		wi.stats.AvgUpdateTime = time.Duration(float64(wi.stats.AvgUpdateTime)*(1-alpha) + float64(processingTime)*alpha)
	}
	wi.stats.mutex.Unlock()
}

// handleFileCreateOrUpdate handles file creation or update operations
func (wi *WatcherIntegration) handleFileCreateOrUpdate(ctx context.Context, request *IndexUpdateRequest) error {
	// For now, we'll trigger cache invalidation and log the update
	// Full implementation would require SCIP document parsing and indexing

	// Invalidate related cache entries
	wi.scipStore.InvalidateFile(request.FilePath)

	// Log the update request
	log.Printf("WatcherIntegration: Processing %s for file %s (language: %s)",
		request.Operation, request.FilePath, request.Language)

	// In a full implementation, this would:
	// 1. Parse the file to create a SCIP document
	// 2. Update the symbol resolver's position index
	// 3. Update the symbol graph with new relationships
	// 4. Update any cross-references

	// Placeholder for actual implementation
	return nil
}

// handleFileRemoval handles file removal operations
func (wi *WatcherIntegration) handleFileRemoval(ctx context.Context, request *IndexUpdateRequest) error {
	// Invalidate cache entries for the removed file
	wi.scipStore.InvalidateFile(request.FilePath)

	// Log the removal
	log.Printf("WatcherIntegration: Processing file removal for %s", request.FilePath)

	// In a full implementation, this would:
	// 1. Remove the file from the position index
	// 2. Remove symbols defined in this file from the symbol graph
	// 3. Update cross-references
	// 4. Clean up any stale cache entries

	return nil
}

// handleFileRename handles file rename operations
func (wi *WatcherIntegration) handleFileRename(ctx context.Context, request *IndexUpdateRequest) error {
	// For renames, we need both old and new paths
	// This is simplified since fsnotify doesn't always provide both paths

	wi.scipStore.InvalidateFile(request.FilePath)

	log.Printf("WatcherIntegration: Processing file rename for %s", request.FilePath)

	// In a full implementation, this would:
	// 1. Update file paths in the position index
	// 2. Update symbol URIs in the symbol graph
	// 3. Update cache keys
	// 4. Maintain symbol relationships

	return nil
}

// Helper methods for decision making

// shouldInvalidateCache determines if cache should be invalidated for an event
func (wi *WatcherIntegration) shouldInvalidateCache(event *FileChangeEvent) bool {
	// Check global setting
	if !wi.config.EnableCacheInvalidation {
		return false
	}

	// Check language-specific settings
	if langConfig, exists := wi.config.LanguageSpecificSettings[event.Language]; exists {
		return langConfig.EnableCacheInvalidation
	}

	// Default to true for known source file types
	return wi.isSourceFile(event.FilePath)
}

// shouldUpdateIndex determines if the index should be updated for an event
func (wi *WatcherIntegration) shouldUpdateIndex(event *FileChangeEvent) bool {
	// Check global setting
	if !wi.config.EnableIndexUpdates {
		return false
	}

	// Check language-specific settings
	if langConfig, exists := wi.config.LanguageSpecificSettings[event.Language]; exists {
		return langConfig.EnableIndexUpdates
	}

	// Default to true for known source file types
	return wi.isSourceFile(event.FilePath)
}

// shouldTriggerReindexing determines if re-indexing should be triggered
func (wi *WatcherIntegration) shouldTriggerReindexing(event *FileChangeEvent) bool {
	// Check global setting
	if !wi.config.EnableReindexing {
		return false
	}

	// Check language-specific settings
	if langConfig, exists := wi.config.LanguageSpecificSettings[event.Language]; exists {
		return langConfig.RequiresReindexing
	}

	return false
}

// getUpdatePriority determines the priority for an index update
func (wi *WatcherIntegration) getUpdatePriority(event *FileChangeEvent) int {
	// Check language-specific settings
	if langConfig, exists := wi.config.LanguageSpecificSettings[event.Language]; exists {
		return langConfig.Priority
	}

	// Default priority based on file type
	if wi.isSourceFile(event.FilePath) {
		return 1 // High priority for source files
	}

	return 3 // Lower priority for other files
}

// isSourceFile determines if a file is a source code file
func (wi *WatcherIntegration) isSourceFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	sourceExts := map[string]bool{
		".go": true, ".py": true, ".js": true, ".ts": true, ".jsx": true, ".tsx": true,
		".java": true, ".kt": true, ".rs": true, ".cpp": true, ".cc": true, ".c": true,
		".cs": true, ".php": true, ".rb": true, ".swift": true, ".scala": true,
	}
	return sourceExts[ext]
}

// triggerReindexing triggers re-indexing for related files
func (wi *WatcherIntegration) triggerReindexing(event *FileChangeEvent) error {
	// This is a placeholder for re-indexing logic
	log.Printf("WatcherIntegration: Triggering re-indexing for %s", event.FilePath)

	wi.stats.mutex.Lock()
	wi.stats.ReindexingOperations++
	wi.stats.mutex.Unlock()

	return nil
}

// recordError records an error in the statistics
func (wi *WatcherIntegration) recordError(errorMsg string) {
	wi.stats.mutex.Lock()
	defer wi.stats.mutex.Unlock()

	wi.stats.ErrorCount++
	wi.stats.LastError = errorMsg
	wi.stats.LastErrorTime = time.Now()
}

// GetStats returns integration statistics
func (wi *WatcherIntegration) GetStats() *IntegrationStats {
	wi.stats.mutex.RLock()
	defer wi.stats.mutex.RUnlock()

	// Create a copy
	stats := *wi.stats

	// Calculate derived metrics
	if stats.TotalIndexUpdates > 0 {
		stats.ThroughputPerSecond = float64(stats.TotalIndexUpdates) / time.Since(stats.startTime).Seconds()
	}

	// Calculate queue utilization
	queueUsage := float64(len(wi.indexUpdateQueue)) / float64(cap(wi.indexUpdateQueue))
	stats.QueueUtilization = queueUsage

	return &stats
}

// NewIntegrationStats creates new integration statistics
func NewIntegrationStats() *IntegrationStats {
	return &IntegrationStats{
		startTime: time.Now(),
	}
}
