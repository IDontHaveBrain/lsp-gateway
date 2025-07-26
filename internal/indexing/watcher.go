package indexing

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
)

const (
	// Performance targets for file system watcher
	TargetFileChangeDetectionLatency = 1 * time.Second
	DefaultMaxMemoryUsage            = 100 * 1024 * 1024 // 100MB
	DefaultMaxWatchedFiles           = 100000
	DefaultDebounceInterval          = 200 * time.Millisecond
	DefaultBatchProcessingSize       = 100
	DefaultEventBufferSize           = 10000

	// Default ignore patterns
	DefaultIgnorePatterns = ".git/**,.svn/**,node_modules/**,*.tmp,*.swp,*.log,build/**,dist/**,target/**,.idea/**,.vscode/**"

	// File system operation types
	FileOpCreate = "create"
	FileOpWrite  = "write"
	FileOpRemove = "remove"
	FileOpRename = "rename"
	FileOpChmod  = "chmod"
)

// FileSystemWatcher provides real-time file change detection with <1s latency
type FileSystemWatcher struct {
	watcher        *fsnotify.Watcher
	scipStore      SCIPStore
	symbolResolver *SymbolResolver
	config         *FileWatcherConfig
	eventProcessor *ChangeEventProcessor
	pathFilter     *PathFilter
	stats          *WatcherStats

	// Event processing
	eventChan chan *FileChangeEvent
	stopChan  chan struct{}
	doneChan  chan struct{}

	// Debouncing and batching
	debouncedEvents map[string]*FileChangeEvent
	debounceMutex   sync.RWMutex
	debounceTimer   *time.Timer

	// Resource monitoring
	memoryUsage      int64
	watchedFileCount int64
	eventCount       int64
	processingQueue  int64

	// Concurrency control
	wg      sync.WaitGroup
	mutex   sync.RWMutex
	running bool

	// Performance optimization
	prefixTree      *PathPrefixTree
	resourceMonitor *ResourceMonitor
}

// FileChangeEvent represents a file system change event
type FileChangeEvent struct {
	FilePath    string                 `json:"file_path"`
	Operation   string                 `json:"operation"`
	Timestamp   time.Time              `json:"timestamp"`
	FileSize    int64                  `json:"file_size,omitempty"`
	Language    string                 `json:"language,omitempty"`
	IsDirectory bool                   `json:"is_directory"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`

	// Processing state
	ProcessingStarted time.Time     `json:"processing_started,omitempty"`
	ProcessingTime    time.Duration `json:"processing_time,omitempty"`
	RetryCount        int           `json:"retry_count,omitempty"`

	// Context for batch processing
	BatchID       string   `json:"batch_id,omitempty"`
	RelatedEvents []string `json:"related_events,omitempty"`
}

// ChangeEventProcessor handles initial filtering and validation of file change events
type ChangeEventProcessor struct {
	languageDetector *LanguageDetector
	fileTypeDetector *FileTypeDetector
	validator        *EventValidator
	batchProcessor   *BatchProcessor
	config           *ProcessorConfig
	stats            *ProcessorStats

	// Processing state
	processingMutex sync.RWMutex
	activeEvents    map[string]*FileChangeEvent
	batchQueue      chan []*FileChangeEvent

	// Performance metrics
	totalProcessed        int64
	successfulEvents      int64
	filteredEvents        int64
	averageProcessingTime time.Duration
}

// FileWatcherConfig contains configuration for file system watching
type FileWatcherConfig struct {
	// Core settings
	Enabled        bool     `yaml:"enabled" json:"enabled"`
	WatchPaths     []string `yaml:"watch_paths" json:"watch_paths"`
	IgnorePatterns []string `yaml:"ignore_patterns" json:"ignore_patterns"`
	RecursiveWatch bool     `yaml:"recursive_watch" json:"recursive_watch"`

	// Performance settings
	DebounceInterval time.Duration `yaml:"debounce_interval" json:"debounce_interval"`
	BatchSize        int           `yaml:"batch_size" json:"batch_size"`
	MaxMemoryUsage   int64         `yaml:"max_memory_usage" json:"max_memory_usage"`
	MaxWatchedFiles  int           `yaml:"max_watched_files" json:"max_watched_files"`

	// Event processing
	EventBufferSize int           `yaml:"event_buffer_size" json:"event_buffer_size"`
	MaxRetries      int           `yaml:"max_retries" json:"max_retries"`
	RetryBackoff    time.Duration `yaml:"retry_backoff" json:"retry_backoff"`

	// Language-specific settings
	LanguageSettings map[string]*LanguageWatchConfig `yaml:"language_settings,omitempty" json:"language_settings,omitempty"`

	// Integration settings
	InvalidateCache   bool `yaml:"invalidate_cache" json:"invalidate_cache"`
	UpdateIndex       bool `yaml:"update_index" json:"update_index"`
	TriggerReindexing bool `yaml:"trigger_reindexing" json:"trigger_reindexing"`

	// Advanced features
	EnableSymlinkHandling    bool `yaml:"enable_symlink_handling" json:"enable_symlink_handling"`
	EnableResourceMonitoring bool `yaml:"enable_resource_monitoring" json:"enable_resource_monitoring"`
	EnableBatchOptimization  bool `yaml:"enable_batch_optimization" json:"enable_batch_optimization"`
}

// LanguageWatchConfig contains language-specific watch configuration
type LanguageWatchConfig struct {
	FileExtensions   []string      `yaml:"file_extensions" json:"file_extensions"`
	IgnorePatterns   []string      `yaml:"ignore_patterns" json:"ignore_patterns"`
	Priority         int           `yaml:"priority" json:"priority"`
	DebounceInterval time.Duration `yaml:"debounce_interval" json:"debounce_interval"`
	EnableIndexing   bool          `yaml:"enable_indexing" json:"enable_indexing"`
}

// ProcessorConfig contains configuration for event processing
type ProcessorConfig struct {
	MaxConcurrentProcessing int           `yaml:"max_concurrent_processing" json:"max_concurrent_processing"`
	ProcessingTimeout       time.Duration `yaml:"processing_timeout" json:"processing_timeout"`
	EnableLanguageDetection bool          `yaml:"enable_language_detection" json:"enable_language_detection"`
	EnableFileTypeDetection bool          `yaml:"enable_file_type_detection" json:"enable_file_type_detection"`
	EnableBatchProcessing   bool          `yaml:"enable_batch_processing" json:"enable_batch_processing"`
}

// PathFilter provides efficient path matching using prefix trees
type PathFilter struct {
	includeTree    *PathPrefixTree
	excludeTree    *PathPrefixTree
	ignorePatterns []*IgnorePattern
	mutex          sync.RWMutex

	// Performance metrics
	matchCount   int64
	avgMatchTime time.Duration
}

// PathPrefixTree implements an efficient prefix tree for path matching
type PathPrefixTree struct {
	root  *PrefixNode
	size  int
	mutex sync.RWMutex
}

// PrefixNode represents a node in the prefix tree
type PrefixNode struct {
	segment   string
	children  map[string]*PrefixNode
	isPattern bool
	pattern   *IgnorePattern
	depth     int
}

// IgnorePattern represents a compiled ignore pattern
type IgnorePattern struct {
	Pattern       string      `json:"pattern"`
	Regex         string      `json:"regex,omitempty"`
	IsGlob        bool        `json:"is_glob"`
	IsDirectory   bool        `json:"is_directory"`
	Negated       bool        `json:"negated"`
	CompiledRegex interface{} `json:"-"` // *regexp.Regexp
}

// LanguageDetector detects programming language from file paths and content
type LanguageDetector struct {
	extensionMap    map[string]string
	patternMap      map[string]string
	contentDetector *ContentDetector
	cache           map[string]string
	cacheMutex      sync.RWMutex

	// Performance metrics
	detectionCount   int64
	cacheHitRate     float64
	avgDetectionTime time.Duration
}

// FileTypeDetector detects file types and categories
type FileTypeDetector struct {
	typeMap        map[string]FileType
	binaryDetector *BinaryDetector
	mimeDetector   *MimeDetector
	cache          map[string]FileType
	cacheMutex     sync.RWMutex
}

// FileType represents detected file type information
type FileType struct {
	Type       string   `json:"type"`     // "source", "binary", "config", "documentation"
	Category   string   `json:"category"` // "code", "resource", "test", "build"
	MimeType   string   `json:"mime_type,omitempty"`
	Language   string   `json:"language,omitempty"`
	Indexable  bool     `json:"indexable"`
	Extensions []string `json:"extensions,omitempty"`
}

// EventValidator validates and filters file change events
type EventValidator struct {
	rules      []ValidationRule
	pathFilter *PathFilter
	sizeLimit  int64
	allowedOps map[string]bool
	stats      *ValidationStats
}

// ValidationRule represents a validation rule for events
type ValidationRule struct {
	Name        string                      `json:"name"`
	Description string                      `json:"description"`
	Condition   func(*FileChangeEvent) bool `json:"-"`
	Action      string                      `json:"action"` // "allow", "deny", "modify"
	Priority    int                         `json:"priority"`
}

// BatchProcessor handles batch processing of file change events
type BatchProcessor struct {
	batchSize     int
	batchTimeout  time.Duration
	currentBatch  []*FileChangeEvent
	batchMutex    sync.Mutex
	batchTimer    *time.Timer
	processorFunc func([]*FileChangeEvent) error
	stats         *BatchStats
}

// WatcherStats tracks file system watcher performance and usage statistics
type WatcherStats struct {
	// Core metrics
	TotalEvents     int64 `json:"total_events"`
	ProcessedEvents int64 `json:"processed_events"`
	FilteredEvents  int64 `json:"filtered_events"`
	FailedEvents    int64 `json:"failed_events"`

	// Performance metrics
	AvgProcessingTime time.Duration `json:"avg_processing_time"`
	P95ProcessingTime time.Duration `json:"p95_processing_time"`
	P99ProcessingTime time.Duration `json:"p99_processing_time"`
	MaxProcessingTime time.Duration `json:"max_processing_time"`

	// Resource usage
	WatchedFiles     int64 `json:"watched_files"`
	MemoryUsage      int64 `json:"memory_usage_bytes"`
	EventBufferUsage int64 `json:"event_buffer_usage"`

	// Operational metrics
	UptimeSeconds   int64     `json:"uptime_seconds"`
	LastEventTime   time.Time `json:"last_event_time"`
	EventsPerSecond float64   `json:"events_per_second"`

	// Error tracking
	ErrorCount    int64     `json:"error_count"`
	LastError     string    `json:"last_error,omitempty"`
	LastErrorTime time.Time `json:"last_error_time,omitempty"`

	mutex     sync.RWMutex
	startTime time.Time
}

// ProcessorStats tracks event processor performance
type ProcessorStats struct {
	ProcessedEvents    int64         `json:"processed_events"`
	FilteredEvents     int64         `json:"filtered_events"`
	LanguageDetections int64         `json:"language_detections"`
	FileTypeDetections int64         `json:"file_type_detections"`
	BatchesProcessed   int64         `json:"batches_processed"`
	AvgBatchSize       float64       `json:"avg_batch_size"`
	AvgProcessingTime  time.Duration `json:"avg_processing_time"`

	mutex sync.RWMutex
}

// ValidationStats tracks validation performance
type ValidationStats struct {
	ValidatedEvents   int64         `json:"validated_events"`
	AcceptedEvents    int64         `json:"accepted_events"`
	RejectedEvents    int64         `json:"rejected_events"`
	ModifiedEvents    int64         `json:"modified_events"`
	AvgValidationTime time.Duration `json:"avg_validation_time"`

	mutex sync.RWMutex
}

// BatchStats tracks batch processing performance
type BatchStats struct {
	BatchesProcessed    int64         `json:"batches_processed"`
	TotalEventsInBatch  int64         `json:"total_events_in_batch"`
	AvgBatchSize        float64       `json:"avg_batch_size"`
	AvgBatchProcessTime time.Duration `json:"avg_batch_process_time"`

	mutex sync.RWMutex
}

// ResourceMonitor monitors system resource usage
type ResourceMonitor struct {
	maxMemoryUsage     int64
	currentMemoryUsage int64
	memoryThreshold    int64
	cpuThreshold       float64

	// Monitoring state
	monitoring      bool
	monitorInterval time.Duration
	alertCallback   func(ResourceAlert)

	// Metrics
	peakMemoryUsage int64
	avgCPUUsage     float64
	lastMonitorTime time.Time

	mutex sync.RWMutex
}

// ResourceAlert represents a resource usage alert
type ResourceAlert struct {
	Type      string    `json:"type"`     // "memory", "cpu", "disk"
	Severity  string    `json:"severity"` // "warning", "critical"
	Current   float64   `json:"current"`
	Threshold float64   `json:"threshold"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// ContentDetector detects programming language from file content
type ContentDetector struct {
	patterns      map[string][]*ContentPattern
	fallbackRules []FallbackRule
	cache         map[string]string
	cacheMutex    sync.RWMutex
}

// ContentPattern represents a pattern for content-based language detection
type ContentPattern struct {
	Language      string      `json:"language"`
	Pattern       string      `json:"pattern"`
	Weight        int         `json:"weight"`
	CompiledRegex interface{} `json:"-"` // *regexp.Regexp
}

// FallbackRule represents a fallback rule for language detection
type FallbackRule struct {
	Extension string `json:"extension"`
	Language  string `json:"language"`
	Priority  int    `json:"priority"`
}

// BinaryDetector detects binary files
type BinaryDetector struct {
	binaryExtensions map[string]bool
	maxScanBytes     int
}

// MimeDetector detects MIME types
type MimeDetector struct {
	mimeMap  map[string]string
	detector interface{} // http.DetectContentType equivalent
}

// NewFileSystemWatcher creates a new file system watcher with the given configuration
func NewFileSystemWatcher(scipStore SCIPStore, symbolResolver *SymbolResolver, config *FileWatcherConfig) (*FileSystemWatcher, error) {
	// Validate parameters
	if scipStore == nil {
		return nil, fmt.Errorf("SCIPStore cannot be nil")
	}
	if symbolResolver == nil {
		return nil, fmt.Errorf("SymbolResolver cannot be nil")
	}
	if config == nil {
		config = DefaultFileWatcherConfig()
	}

	// Create fsnotify watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}

	// Create event processor
	eventProcessor, err := NewChangeEventProcessor(&ProcessorConfig{
		MaxConcurrentProcessing: 10,
		ProcessingTimeout:       30 * time.Second,
		EnableLanguageDetection: true,
		EnableFileTypeDetection: true,
		EnableBatchProcessing:   config.EnableBatchOptimization,
	})
	if err != nil {
		watcher.Close()
		return nil, fmt.Errorf("failed to create event processor: %w", err)
	}

	// Create path filter
	pathFilter, err := NewPathFilter(config.IgnorePatterns)
	if err != nil {
		watcher.Close()
		return nil, fmt.Errorf("failed to create path filter: %w", err)
	}

	// Create resource monitor
	resourceMonitor := NewResourceMonitor(config.MaxMemoryUsage, 80.0) // 80% CPU threshold

	fsWatcher := &FileSystemWatcher{
		watcher:         watcher,
		scipStore:       scipStore,
		symbolResolver:  symbolResolver,
		config:          config,
		eventProcessor:  eventProcessor,
		pathFilter:      pathFilter,
		stats:           NewWatcherStats(),
		eventChan:       make(chan *FileChangeEvent, config.EventBufferSize),
		stopChan:        make(chan struct{}),
		doneChan:        make(chan struct{}),
		debouncedEvents: make(map[string]*FileChangeEvent),
		prefixTree:      NewPathPrefixTree(),
		resourceMonitor: resourceMonitor,
	}

	return fsWatcher, nil
}

// DefaultFileWatcherConfig returns default configuration for file system watcher
func DefaultFileWatcherConfig() *FileWatcherConfig {
	ignorePatterns := strings.Split(DefaultIgnorePatterns, ",")

	return &FileWatcherConfig{
		Enabled:                  true,
		WatchPaths:               []string{"."},
		IgnorePatterns:           ignorePatterns,
		RecursiveWatch:           true,
		DebounceInterval:         DefaultDebounceInterval,
		BatchSize:                DefaultBatchProcessingSize,
		MaxMemoryUsage:           DefaultMaxMemoryUsage,
		MaxWatchedFiles:          DefaultMaxWatchedFiles,
		EventBufferSize:          DefaultEventBufferSize,
		MaxRetries:               3,
		RetryBackoff:             1 * time.Second,
		InvalidateCache:          true,
		UpdateIndex:              true,
		TriggerReindexing:        false,
		EnableSymlinkHandling:    true,
		EnableResourceMonitoring: true,
		EnableBatchOptimization:  true,
	}
}

// Start begins file system monitoring
func (fsw *FileSystemWatcher) Start(ctx context.Context) error {
	fsw.mutex.Lock()
	if fsw.running {
		fsw.mutex.Unlock()
		return fmt.Errorf("file system watcher is already running")
	}
	fsw.running = true
	fsw.mutex.Unlock()

	// Start resource monitoring if enabled
	if fsw.config.EnableResourceMonitoring {
		fsw.resourceMonitor.Start()
	}

	// Add watch paths
	for _, path := range fsw.config.WatchPaths {
		if err := fsw.addWatchPath(path); err != nil {
			log.Printf("FileWatcher: Failed to add watch path %s: %v", path, err)
			continue
		}
	}

	// Start event processing goroutines
	fsw.wg.Add(3)
	go fsw.eventProcessor.Run(ctx, &fsw.wg)
	go fsw.processEvents(ctx, &fsw.wg)
	go fsw.watchFileSystemEvents(ctx, &fsw.wg)

	log.Printf("FileWatcher: Started watching %d paths with %d ignore patterns",
		len(fsw.config.WatchPaths), len(fsw.config.IgnorePatterns))

	return nil
}

// Stop stops file system monitoring
func (fsw *FileSystemWatcher) Stop() error {
	fsw.mutex.Lock()
	if !fsw.running {
		fsw.mutex.Unlock()
		return nil
	}
	fsw.running = false
	fsw.mutex.Unlock()

	// Signal stop
	close(fsw.stopChan)

	// Stop resource monitoring
	if fsw.resourceMonitor != nil {
		fsw.resourceMonitor.Stop()
	}

	// Close fsnotify watcher
	if err := fsw.watcher.Close(); err != nil {
		log.Printf("FileWatcher: Error closing fsnotify watcher: %v", err)
	}

	// Wait for goroutines to finish
	fsw.wg.Wait()

	// Close channels
	close(fsw.eventChan)
	close(fsw.doneChan)

	log.Println("FileWatcher: Stopped successfully")
	return nil
}

// addWatchPath adds a path to be watched recursively
func (fsw *FileSystemWatcher) addWatchPath(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for %s: %w", path, err)
	}

	if !fsw.config.RecursiveWatch {
		return fsw.watcher.Add(absPath)
	}

	// Walk directory tree and add all directories
	err = filepath.Walk(absPath, func(walkPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip if it's not a directory
		if info == nil {
			return nil
		}

		// Check if path should be ignored
		if fsw.pathFilter.ShouldIgnore(walkPath) {
			if info != nil {
				return filepath.SkipDir
			}
			return nil
		}

		// Add directory to watcher
		if err := fsw.watcher.Add(walkPath); err != nil {
			log.Printf("FileWatcher: Failed to add path %s: %v", walkPath, err)
			return nil // Continue with other paths
		}

		atomic.AddInt64(&fsw.watchedFileCount, 1)
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk directory tree %s: %w", absPath, err)
	}

	return nil
}

// watchFileSystemEvents monitors fsnotify events and creates FileChangeEvents
func (fsw *FileSystemWatcher) watchFileSystemEvents(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-fsw.stopChan:
			return
		case event, ok := <-fsw.watcher.Events:
			if !ok {
				return
			}

			// Process fsnotify event
			if err := fsw.handleFSNotifyEvent(event); err != nil {
				fsw.stats.recordError(fmt.Sprintf("Failed to handle event %v: %v", event, err))
			}

		case err, ok := <-fsw.watcher.Errors:
			if !ok {
				return
			}

			fsw.stats.recordError(fmt.Sprintf("Watcher error: %v", err))
			log.Printf("FileWatcher: Watcher error: %v", err)
		}
	}
}

// handleFSNotifyEvent converts fsnotify.Event to FileChangeEvent and applies debouncing
func (fsw *FileSystemWatcher) handleFSNotifyEvent(event fsnotify.Event) error {
	// Check if file should be ignored
	if fsw.pathFilter.ShouldIgnore(event.Name) {
		atomic.AddInt64(&fsw.stats.FilteredEvents, 1)
		return nil
	}

	// Determine operation type
	operation := fsw.mapFSNotifyOpToFileOp(event.Op)

	// Create file change event
	changeEvent := &FileChangeEvent{
		FilePath:    event.Name,
		Operation:   operation,
		Timestamp:   time.Now(),
		IsDirectory: fsw.isDirectory(event.Name),
		Metadata:    make(map[string]interface{}),
	}

	// Add file size if available
	if !changeEvent.IsDirectory && operation != FileOpRemove {
		if size, err := fsw.getFileSize(event.Name); err == nil {
			changeEvent.FileSize = size
		}
	}

	// Apply debouncing
	fsw.applyDebouncing(changeEvent)

	atomic.AddInt64(&fsw.stats.TotalEvents, 1)
	return nil
}

// mapFSNotifyOpToFileOp maps fsnotify operations to our file operations
func (fsw *FileSystemWatcher) mapFSNotifyOpToFileOp(op fsnotify.Op) string {
	switch {
	case op&fsnotify.Create == fsnotify.Create:
		return FileOpCreate
	case op&fsnotify.Write == fsnotify.Write:
		return FileOpWrite
	case op&fsnotify.Remove == fsnotify.Remove:
		return FileOpRemove
	case op&fsnotify.Rename == fsnotify.Rename:
		return FileOpRename
	case op&fsnotify.Chmod == fsnotify.Chmod:
		return FileOpChmod
	default:
		return FileOpWrite // Default fallback
	}
}

// applyDebouncing applies debouncing logic to prevent rapid-fire events
func (fsw *FileSystemWatcher) applyDebouncing(event *FileChangeEvent) {
	fsw.debounceMutex.Lock()
	defer fsw.debounceMutex.Unlock()

	// Store or update debounced event
	fsw.debouncedEvents[event.FilePath] = event

	// Reset or start debounce timer
	if fsw.debounceTimer != nil {
		fsw.debounceTimer.Stop()
	}

	fsw.debounceTimer = time.AfterFunc(fsw.config.DebounceInterval, func() {
		fsw.flushDebouncedEvents()
	})
}

// flushDebouncedEvents sends debounced events for processing
func (fsw *FileSystemWatcher) flushDebouncedEvents() {
	fsw.debounceMutex.Lock()
	events := make([]*FileChangeEvent, 0, len(fsw.debouncedEvents))
	for _, event := range fsw.debouncedEvents {
		events = append(events, event)
	}
	fsw.debouncedEvents = make(map[string]*FileChangeEvent)
	fsw.debounceMutex.Unlock()

	// Send events for processing
	for _, event := range events {
		select {
		case fsw.eventChan <- event:
			// Event queued successfully
		default:
			// Channel is full, drop event and record error
			fsw.stats.recordError("Event channel full, dropping event")
			atomic.AddInt64(&fsw.stats.FailedEvents, 1)
		}
	}
}

// processEvents processes file change events and integrates with SCIP infrastructure
func (fsw *FileSystemWatcher) processEvents(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-fsw.stopChan:
			return
		case event, ok := <-fsw.eventChan:
			if !ok {
				return
			}

			if err := fsw.processFileChangeEvent(event); err != nil {
				fsw.stats.recordError(fmt.Sprintf("Failed to process event %v: %v", event, err))
				atomic.AddInt64(&fsw.stats.FailedEvents, 1)
			} else {
				atomic.AddInt64(&fsw.stats.ProcessedEvents, 1)
			}
		}
	}
}

// processFileChangeEvent processes a single file change event
func (fsw *FileSystemWatcher) processFileChangeEvent(event *FileChangeEvent) error {
	startTime := time.Now()
	event.ProcessingStarted = startTime

	// Process through event processor for language detection and validation
	processedEvent, err := fsw.eventProcessor.ProcessEvent(event)
	if err != nil {
		return fmt.Errorf("event processor failed: %w", err)
	}

	if processedEvent == nil {
		// Event was filtered out
		atomic.AddInt64(&fsw.stats.FilteredEvents, 1)
		return nil
	}

	// Update processing time
	processedEvent.ProcessingTime = time.Since(startTime)

	// Integrate with SCIP infrastructure
	if err := fsw.integrateWithSCIP(processedEvent); err != nil {
		return fmt.Errorf("SCIP integration failed: %w", err)
	}

	// Update performance metrics
	fsw.stats.updatePerformanceMetrics(processedEvent.ProcessingTime)

	return nil
}

// integrateWithSCIP integrates file change events with SCIP infrastructure
func (fsw *FileSystemWatcher) integrateWithSCIP(event *FileChangeEvent) error {
	// Cache invalidation
	if fsw.config.InvalidateCache && fsw.scipStore != nil {
		fsw.scipStore.InvalidateFile(event.FilePath)
	}

	// Position index updates
	if fsw.config.UpdateIndex && fsw.symbolResolver != nil {
		// For now, we signal that the file has changed
		// Full implementation would require SCIP document parsing
		// This is a placeholder for the integration point
		log.Printf("FileWatcher: File changed, should update index for: %s", event.FilePath)
	}

	// Trigger re-indexing if configured
	if fsw.config.TriggerReindexing && event.Language != "" {
		// This would trigger language-specific re-indexing
		log.Printf("FileWatcher: Triggering re-indexing for %s file: %s", event.Language, event.FilePath)
	}

	return nil
}

// GetStats returns comprehensive watcher statistics
func (fsw *FileSystemWatcher) GetStats() *WatcherStats {
	fsw.stats.mutex.RLock()
	defer fsw.stats.mutex.RUnlock()

	// Create a copy with current values
	stats := *fsw.stats
	stats.WatchedFiles = atomic.LoadInt64(&fsw.watchedFileCount)
	stats.MemoryUsage = atomic.LoadInt64(&fsw.memoryUsage)
	stats.EventBufferUsage = int64(len(fsw.eventChan))
	stats.UptimeSeconds = int64(time.Since(stats.startTime).Seconds())

	// Calculate events per second
	if stats.UptimeSeconds > 0 {
		stats.EventsPerSecond = float64(stats.TotalEvents) / float64(stats.UptimeSeconds)
	}

	return &stats
}

// Helper methods for FileSystemWatcher

func (fsw *FileSystemWatcher) isDirectory(path string) bool {
	// This would use os.Stat to check if path is directory
	// Simplified implementation for now
	return false
}

func (fsw *FileSystemWatcher) getFileSize(path string) (int64, error) {
	// This would use os.Stat to get file size
	// Simplified implementation for now
	return 0, fmt.Errorf("not implemented")
}

// WatcherStats methods

// NewWatcherStats creates new watcher statistics
func NewWatcherStats() *WatcherStats {
	return &WatcherStats{
		startTime: time.Now(),
	}
}

// recordError records an error in the statistics
func (ws *WatcherStats) recordError(errorMsg string) {
	ws.mutex.Lock()
	defer ws.mutex.Unlock()

	ws.ErrorCount++
	ws.LastError = errorMsg
	ws.LastErrorTime = time.Now()
}

// updatePerformanceMetrics updates performance timing metrics
func (ws *WatcherStats) updatePerformanceMetrics(processingTime time.Duration) {
	ws.mutex.Lock()
	defer ws.mutex.Unlock()

	// Update average (exponential moving average)
	if ws.AvgProcessingTime == 0 {
		ws.AvgProcessingTime = processingTime
	} else {
		alpha := 0.1
		ws.AvgProcessingTime = time.Duration(float64(ws.AvgProcessingTime)*(1-alpha) + float64(processingTime)*alpha)
	}

	// Update max
	if processingTime > ws.MaxProcessingTime {
		ws.MaxProcessingTime = processingTime
	}

	// Update P95/P99 (simplified - would use proper percentile calculation in production)
	if processingTime > ws.P95ProcessingTime {
		ws.P95ProcessingTime = processingTime
	}
	if processingTime > ws.P99ProcessingTime {
		ws.P99ProcessingTime = processingTime
	}

	ws.LastEventTime = time.Now()
}
