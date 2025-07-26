package indexing

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// NewChangeEventProcessor creates a new change event processor
func NewChangeEventProcessor(config *ProcessorConfig) (*ChangeEventProcessor, error) {
	if config == nil {
		config = &ProcessorConfig{
			MaxConcurrentProcessing: 10,
			ProcessingTimeout:       30 * time.Second,
			EnableLanguageDetection: true,
			EnableFileTypeDetection: true,
			EnableBatchProcessing:   true,
		}
	}

	// Create language detector
	languageDetector, err := NewLanguageDetector()
	if err != nil {
		return nil, fmt.Errorf("failed to create language detector: %w", err)
	}

	// Create file type detector
	fileTypeDetector, err := NewFileTypeDetector()
	if err != nil {
		return nil, fmt.Errorf("failed to create file type detector: %w", err)
	}

	// Create event validator
	validator, err := NewEventValidator()
	if err != nil {
		return nil, fmt.Errorf("failed to create event validator: %w", err)
	}

	// Create batch processor
	batchProcessor, err := NewBatchProcessor(50, 5*time.Second, nil) // Will set processor func later
	if err != nil {
		return nil, fmt.Errorf("failed to create batch processor: %w", err)
	}

	processor := &ChangeEventProcessor{
		languageDetector: languageDetector,
		fileTypeDetector: fileTypeDetector,
		validator:        validator,
		batchProcessor:   batchProcessor,
		config:           config,
		stats:            NewProcessorStats(),
		activeEvents:     make(map[string]*FileChangeEvent),
		batchQueue:       make(chan []*FileChangeEvent, 100),
	}

	// Set the batch processor function
	batchProcessor.processorFunc = processor.processBatch

	return processor, nil
}

// ProcessEvent processes a single file change event
func (cep *ChangeEventProcessor) ProcessEvent(event *FileChangeEvent) (*FileChangeEvent, error) {
	startTime := time.Now()

	// Validate event
	if !cep.validator.ValidateEvent(event) {
		atomic.AddInt64(&cep.stats.FilteredEvents, 1)
		return nil, nil // Event filtered out
	}

	// Detect language if enabled
	if cep.config.EnableLanguageDetection {
		language, err := cep.languageDetector.DetectLanguage(event.FilePath)
		if err != nil {
			log.Printf("EventProcessor: Failed to detect language for %s: %v", event.FilePath, err)
		} else {
			event.Language = language
			atomic.AddInt64(&cep.stats.LanguageDetections, 1)
		}
	}

	// Detect file type if enabled
	if cep.config.EnableFileTypeDetection {
		fileType, err := cep.fileTypeDetector.DetectFileType(event.FilePath)
		if err != nil {
			log.Printf("EventProcessor: Failed to detect file type for %s: %v", event.FilePath, err)
		} else {
			event.Metadata["file_type"] = fileType
			atomic.AddInt64(&cep.stats.FileTypeDetections, 1)
		}
	}

	// Update processing time
	processingTime := time.Since(startTime)
	cep.updateProcessingTime(processingTime)

	atomic.AddInt64(&cep.stats.ProcessedEvents, 1)
	return event, nil
}

// Run starts the event processor
func (cep *ChangeEventProcessor) Run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	// Start batch processor if enabled
	if cep.config.EnableBatchProcessing {
		go cep.batchProcessor.Run(ctx)
	}

	// Process batch queue
	for {
		select {
		case <-ctx.Done():
			return
		case batch := <-cep.batchQueue:
			if err := cep.processBatch(batch); err != nil {
				log.Printf("EventProcessor: Failed to process batch: %v", err)
			}
		}
	}
}

// processBatch processes a batch of file change events
func (cep *ChangeEventProcessor) processBatch(events []*FileChangeEvent) error {
	startTime := time.Now()

	// Process events in parallel
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, cep.config.MaxConcurrentProcessing)

	for _, event := range events {
		wg.Add(1)
		go func(e *FileChangeEvent) {
			defer wg.Done()

			// Acquire semaphore slot
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Process individual event
			if _, err := cep.ProcessEvent(e); err != nil {
				log.Printf("EventProcessor: Failed to process event %v: %v", e, err)
			}
		}(event)
	}

	wg.Wait()

	// Update batch statistics
	cep.stats.mutex.Lock()
	cep.stats.BatchesProcessed++
	cep.stats.ProcessedEvents += int64(len(events))
	batchSize := float64(len(events))
	if cep.stats.AvgBatchSize == 0 {
		cep.stats.AvgBatchSize = batchSize
	} else {
		alpha := 0.1
		cep.stats.AvgBatchSize = cep.stats.AvgBatchSize*(1-alpha) + batchSize*alpha
	}
	cep.stats.mutex.Unlock()

	atomic.AddInt64(&cep.stats.BatchesProcessed, 1)

	log.Printf("EventProcessor: Processed batch of %d events in %v", len(events), time.Since(startTime))
	return nil
}

// updateProcessingTime updates average processing time
func (cep *ChangeEventProcessor) updateProcessingTime(duration time.Duration) {
	cep.stats.mutex.Lock()
	defer cep.stats.mutex.Unlock()

	if cep.stats.AvgProcessingTime == 0 {
		cep.stats.AvgProcessingTime = duration
	} else {
		alpha := 0.1
		cep.stats.AvgProcessingTime = time.Duration(float64(cep.stats.AvgProcessingTime)*(1-alpha) + float64(duration)*alpha)
	}
}

// GetStats returns processor statistics
func (cep *ChangeEventProcessor) GetStats() *ProcessorStats {
	cep.stats.mutex.RLock()
	defer cep.stats.mutex.RUnlock()

	stats := *cep.stats
	return &stats
}

// NewLanguageDetector creates a new language detector
func NewLanguageDetector() (*LanguageDetector, error) {
	detector := &LanguageDetector{
		extensionMap: make(map[string]string),
		patternMap:   make(map[string]string),
		cache:        make(map[string]string),
	}

	// Initialize extension mappings
	detector.initializeExtensionMappings()

	// Create content detector
	contentDetector, err := NewContentDetector()
	if err != nil {
		return nil, fmt.Errorf("failed to create content detector: %w", err)
	}
	detector.contentDetector = contentDetector

	return detector, nil
}

// DetectLanguage detects the programming language of a file
func (ld *LanguageDetector) DetectLanguage(filePath string) (string, error) {
	// Check cache first
	ld.cacheMutex.RLock()
	if cached, exists := ld.cache[filePath]; exists {
		ld.cacheMutex.RUnlock()
		atomic.AddInt64(&ld.detectionCount, 1)
		return cached, nil
	}
	ld.cacheMutex.RUnlock()

	startTime := time.Now()

	// Extract file extension
	ext := strings.ToLower(filepath.Ext(filePath))

	// Check extension mapping
	if language, exists := ld.extensionMap[ext]; exists {
		ld.cacheResult(filePath, language)
		ld.updateDetectionTime(time.Since(startTime))
		return language, nil
	}

	// Check filename patterns
	filename := filepath.Base(filePath)
	if language := ld.checkFilenamePatterns(filename); language != "" {
		ld.cacheResult(filePath, language)
		ld.updateDetectionTime(time.Since(startTime))
		return language, nil
	}

	// Fall back to content detection if available
	if ld.contentDetector != nil {
		if language, err := ld.contentDetector.DetectFromContent(filePath); err == nil && language != "" {
			ld.cacheResult(filePath, language)
			ld.updateDetectionTime(time.Since(startTime))
			return language, nil
		}
	}

	// Return empty if no language detected
	ld.updateDetectionTime(time.Since(startTime))
	return "", nil
}

// initializeExtensionMappings initializes the extension to language mappings
func (ld *LanguageDetector) initializeExtensionMappings() {
	// Common programming language extensions
	extensions := map[string]string{
		".go":         "go",
		".py":         "python",
		".js":         "javascript",
		".ts":         "typescript",
		".jsx":        "javascript",
		".tsx":        "typescript",
		".java":       "java",
		".kt":         "kotlin",
		".rs":         "rust",
		".cpp":        "cpp",
		".cc":         "cpp",
		".cxx":        "cpp",
		".c":          "c",
		".h":          "c",
		".hpp":        "cpp",
		".cs":         "csharp",
		".php":        "php",
		".rb":         "ruby",
		".swift":      "swift",
		".scala":      "scala",
		".sh":         "shell",
		".bash":       "shell",
		".zsh":        "shell",
		".fish":       "shell",
		".ps1":        "powershell",
		".yaml":       "yaml",
		".yml":        "yaml",
		".json":       "json",
		".xml":        "xml",
		".html":       "html",
		".htm":        "html",
		".css":        "css",
		".scss":       "scss",
		".sass":       "sass",
		".less":       "less",
		".sql":        "sql",
		".r":          "r",
		".R":          "r",
		".m":          "matlab",
		".pl":         "perl",
		".lua":        "lua",
		".vim":        "vim",
		".md":         "markdown",
		".tex":        "latex",
		".dockerfile": "dockerfile",
		".proto":      "protobuf",
		".toml":       "toml",
		".ini":        "ini",
		".cfg":        "ini",
		".conf":       "conf",
	}

	for ext, lang := range extensions {
		ld.extensionMap[ext] = lang
	}
}

// checkFilenamePatterns checks filename patterns for language detection
func (ld *LanguageDetector) checkFilenamePatterns(filename string) string {
	patterns := map[string]string{
		"Dockerfile":       "dockerfile",
		"Makefile":         "makefile",
		"makefile":         "makefile",
		"Rakefile":         "ruby",
		"Gemfile":          "ruby",
		"Podfile":          "ruby",
		"package.json":     "json",
		"composer.json":    "json",
		"cargo.toml":       "toml",
		"pyproject.toml":   "toml",
		"go.mod":           "go",
		"go.sum":           "go",
		".gitignore":       "gitignore",
		".dockerignore":    "dockerignore",
		"requirements.txt": "text",
		"readme.md":        "markdown",
		"README.md":        "markdown",
		"LICENSE":          "text",
		"CHANGELOG":        "text",
	}

	lowerFilename := strings.ToLower(filename)

	for pattern, language := range patterns {
		if strings.ToLower(pattern) == lowerFilename {
			return language
		}
	}

	return ""
}

// cacheResult caches a language detection result
func (ld *LanguageDetector) cacheResult(filePath, language string) {
	ld.cacheMutex.Lock()
	defer ld.cacheMutex.Unlock()

	ld.cache[filePath] = language

	// Simple cache size management - remove oldest entries if cache gets too large
	if len(ld.cache) > 10000 {
		// Remove random entries (in production, would use LRU)
		count := 0
		for key := range ld.cache {
			delete(ld.cache, key)
			count++
			if count >= 1000 {
				break
			}
		}
	}
}

// updateDetectionTime updates average detection time
func (ld *LanguageDetector) updateDetectionTime(duration time.Duration) {
	if ld.avgDetectionTime == 0 {
		ld.avgDetectionTime = duration
	} else {
		alpha := 0.1
		ld.avgDetectionTime = time.Duration(float64(ld.avgDetectionTime)*(1-alpha) + float64(duration)*alpha)
	}

	atomic.AddInt64(&ld.detectionCount, 1)
}

// NewFileTypeDetector creates a new file type detector
func NewFileTypeDetector() (*FileTypeDetector, error) {
	detector := &FileTypeDetector{
		typeMap: make(map[string]FileType),
		cache:   make(map[string]FileType),
	}

	// Initialize type mappings
	detector.initializeTypeMappings()

	// Create binary detector
	detector.binaryDetector = NewBinaryDetector()

	// Create mime detector
	detector.mimeDetector = NewMimeDetector()

	return detector, nil
}

// DetectFileType detects the file type and category
func (ftd *FileTypeDetector) DetectFileType(filePath string) (FileType, error) {
	// Check cache first
	ftd.cacheMutex.RLock()
	if cached, exists := ftd.cache[filePath]; exists {
		ftd.cacheMutex.RUnlock()
		return cached, nil
	}
	ftd.cacheMutex.RUnlock()

	// Get file extension
	ext := strings.ToLower(filepath.Ext(filePath))

	// Check type mapping
	if fileType, exists := ftd.typeMap[ext]; exists {
		ftd.cacheFileType(filePath, fileType)
		return fileType, nil
	}

	// Check if binary
	if ftd.binaryDetector.IsBinary(filePath) {
		fileType := FileType{
			Type:      "binary",
			Category:  "resource",
			Indexable: false,
		}
		ftd.cacheFileType(filePath, fileType)
		return fileType, nil
	}

	// Default to text file
	fileType := FileType{
		Type:      "text",
		Category:  "unknown",
		Indexable: true,
	}
	ftd.cacheFileType(filePath, fileType)
	return fileType, nil
}

// initializeTypeMappings initializes file type mappings
func (ftd *FileTypeDetector) initializeTypeMappings() {
	// Source code files
	sourceTypes := []string{".go", ".py", ".js", ".ts", ".java", ".cpp", ".c", ".rs", ".rb", ".php", ".cs", ".swift", ".kt", ".scala"}
	for _, ext := range sourceTypes {
		ftd.typeMap[ext] = FileType{
			Type:      "source",
			Category:  "code",
			Indexable: true,
		}
	}

	// Configuration files
	configTypes := []string{".yaml", ".yml", ".json", ".toml", ".ini", ".cfg", ".conf", ".xml"}
	for _, ext := range configTypes {
		ftd.typeMap[ext] = FileType{
			Type:      "config",
			Category:  "config",
			Indexable: true,
		}
	}

	// Documentation files
	docTypes := []string{".md", ".txt", ".rst", ".adoc", ".tex"}
	for _, ext := range docTypes {
		ftd.typeMap[ext] = FileType{
			Type:      "documentation",
			Category:  "documentation",
			Indexable: true,
		}
	}

	// Test files (pattern-based, simplified)
	testTypes := []string{"_test.go", "_test.py", ".test.js", ".spec.js", "test.py", "spec.py"}
	for _, pattern := range testTypes {
		ftd.typeMap[pattern] = FileType{
			Type:      "test",
			Category:  "test",
			Indexable: true,
		}
	}

	// Binary files
	binaryTypes := []string{".exe", ".dll", ".so", ".dylib", ".a", ".lib", ".bin", ".pdf", ".jpg", ".png", ".gif", ".mp4", ".zip", ".tar", ".gz"}
	for _, ext := range binaryTypes {
		ftd.typeMap[ext] = FileType{
			Type:      "binary",
			Category:  "resource",
			Indexable: false,
		}
	}
}

// cacheFileType caches a file type detection result
func (ftd *FileTypeDetector) cacheFileType(filePath string, fileType FileType) {
	ftd.cacheMutex.Lock()
	defer ftd.cacheMutex.Unlock()

	ftd.cache[filePath] = fileType

	// Simple cache size management
	if len(ftd.cache) > 5000 {
		count := 0
		for key := range ftd.cache {
			delete(ftd.cache, key)
			count++
			if count >= 500 {
				break
			}
		}
	}
}

// NewEventValidator creates a new event validator
func NewEventValidator() (*EventValidator, error) {
	validator := &EventValidator{
		rules:      make([]ValidationRule, 0),
		sizeLimit:  100 * 1024 * 1024, // 100MB file size limit
		allowedOps: make(map[string]bool),
		stats:      NewValidationStats(),
	}

	// Initialize allowed operations
	validator.allowedOps[FileOpCreate] = true
	validator.allowedOps[FileOpWrite] = true
	validator.allowedOps[FileOpRemove] = true
	validator.allowedOps[FileOpRename] = true
	// FileOpChmod is often not relevant for indexing

	// Add default validation rules
	validator.addDefaultRules()

	return validator, nil
}

// ValidateEvent validates a file change event
func (ev *EventValidator) ValidateEvent(event *FileChangeEvent) bool {
	startTime := time.Now()

	// Basic validation
	if event == nil || event.FilePath == "" {
		ev.updateStats(false, time.Since(startTime))
		return false
	}

	// Check if operation is allowed
	if !ev.allowedOps[event.Operation] {
		ev.updateStats(false, time.Since(startTime))
		return false
	}

	// Check file size limit
	if event.FileSize > ev.sizeLimit {
		ev.updateStats(false, time.Since(startTime))
		return false
	}

	// Apply validation rules
	for _, rule := range ev.rules {
		if rule.Condition != nil && !rule.Condition(event) {
			ev.updateStats(false, time.Since(startTime))
			return false
		}
	}

	ev.updateStats(true, time.Since(startTime))
	return true
}

// addDefaultRules adds default validation rules
func (ev *EventValidator) addDefaultRules() {
	// Rule: Ignore temporary files
	ev.rules = append(ev.rules, ValidationRule{
		Name:        "ignore_temp_files",
		Description: "Ignore temporary files and swap files",
		Condition: func(event *FileChangeEvent) bool {
			path := strings.ToLower(event.FilePath)
			return !strings.HasSuffix(path, ".tmp") &&
				!strings.HasSuffix(path, ".swp") &&
				!strings.HasSuffix(path, ".bak") &&
				!strings.Contains(path, "/.git/") &&
				!strings.Contains(path, "/node_modules/")
		},
		Action:   "deny",
		Priority: 1,
	})

	// Rule: Ignore hidden files (optional)
	ev.rules = append(ev.rules, ValidationRule{
		Name:        "ignore_hidden_files",
		Description: "Ignore hidden files starting with dot",
		Condition: func(event *FileChangeEvent) bool {
			filename := filepath.Base(event.FilePath)
			// Allow some important dotfiles
			if filename == ".gitignore" || filename == ".dockerignore" {
				return true
			}
			return !strings.HasPrefix(filename, ".")
		},
		Action:   "deny",
		Priority: 2,
	})
}

// updateStats updates validation statistics
func (ev *EventValidator) updateStats(accepted bool, duration time.Duration) {
	ev.stats.mutex.Lock()
	defer ev.stats.mutex.Unlock()

	ev.stats.ValidatedEvents++
	if accepted {
		ev.stats.AcceptedEvents++
	} else {
		ev.stats.RejectedEvents++
	}

	// Update average validation time
	if ev.stats.AvgValidationTime == 0 {
		ev.stats.AvgValidationTime = duration
	} else {
		alpha := 0.1
		ev.stats.AvgValidationTime = time.Duration(float64(ev.stats.AvgValidationTime)*(1-alpha) + float64(duration)*alpha)
	}
}

// NewBatchProcessor creates a new batch processor
func NewBatchProcessor(batchSize int, batchTimeout time.Duration, processorFunc func([]*FileChangeEvent) error) (*BatchProcessor, error) {
	if batchSize <= 0 {
		batchSize = 50
	}
	if batchTimeout <= 0 {
		batchTimeout = 5 * time.Second
	}

	return &BatchProcessor{
		batchSize:     batchSize,
		batchTimeout:  batchTimeout,
		currentBatch:  make([]*FileChangeEvent, 0, batchSize),
		processorFunc: processorFunc,
		stats:         NewBatchStats(),
	}, nil
}

// Run starts the batch processor
func (bp *BatchProcessor) Run(ctx context.Context) {
	bp.batchTimer = time.NewTimer(bp.batchTimeout)
	defer bp.batchTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			// Process remaining batch before exit
			if len(bp.currentBatch) > 0 {
				bp.processBatch()
			}
			return
		case <-bp.batchTimer.C:
			// Timeout reached, process current batch
			if len(bp.currentBatch) > 0 {
				bp.processBatch()
			}
		}
	}
}

// AddEvent adds an event to the current batch
func (bp *BatchProcessor) AddEvent(event *FileChangeEvent) {
	bp.batchMutex.Lock()
	defer bp.batchMutex.Unlock()

	bp.currentBatch = append(bp.currentBatch, event)

	// Process batch if it's full
	if len(bp.currentBatch) >= bp.batchSize {
		bp.processBatch()
	}
}

// processBatch processes the current batch
func (bp *BatchProcessor) processBatch() {
	if len(bp.currentBatch) == 0 {
		return
	}

	startTime := time.Now()
	batchSize := len(bp.currentBatch)

	// Process the batch
	if bp.processorFunc != nil {
		if err := bp.processorFunc(bp.currentBatch); err != nil {
			log.Printf("BatchProcessor: Failed to process batch: %v", err)
		}
	}

	// Update statistics
	bp.stats.mutex.Lock()
	bp.stats.BatchesProcessed++
	bp.stats.TotalEventsInBatch += int64(batchSize)
	if bp.stats.AvgBatchSize == 0 {
		bp.stats.AvgBatchSize = float64(batchSize)
	} else {
		alpha := 0.1
		bp.stats.AvgBatchSize = bp.stats.AvgBatchSize*(1-alpha) + float64(batchSize)*alpha
	}

	processingTime := time.Since(startTime)
	if bp.stats.AvgBatchProcessTime == 0 {
		bp.stats.AvgBatchProcessTime = processingTime
	} else {
		alpha := 0.1
		bp.stats.AvgBatchProcessTime = time.Duration(float64(bp.stats.AvgBatchProcessTime)*(1-alpha) + float64(processingTime)*alpha)
	}
	bp.stats.mutex.Unlock()

	// Clear the batch
	bp.currentBatch = bp.currentBatch[:0]

	// Reset timer
	if bp.batchTimer != nil {
		bp.batchTimer.Reset(bp.batchTimeout)
	}
}

// Helper constructors for stats

// NewProcessorStats creates new processor statistics
func NewProcessorStats() *ProcessorStats {
	return &ProcessorStats{}
}

// NewValidationStats creates new validation statistics
func NewValidationStats() *ValidationStats {
	return &ValidationStats{}
}

// NewBatchStats creates new batch statistics
func NewBatchStats() *BatchStats {
	return &BatchStats{}
}

// Stub implementations for content detection, binary detection, and mime detection

// NewContentDetector creates a new content detector
func NewContentDetector() (*ContentDetector, error) {
	return &ContentDetector{
		patterns:      make(map[string][]*ContentPattern),
		fallbackRules: make([]FallbackRule, 0),
		cache:         make(map[string]string),
	}, nil
}

// DetectFromContent detects language from file content (stub implementation)
func (cd *ContentDetector) DetectFromContent(filePath string) (string, error) {
	// This is a stub implementation
	// In a real implementation, this would read file content and apply pattern matching
	return "", nil
}

// NewBinaryDetector creates a new binary detector
func NewBinaryDetector() *BinaryDetector {
	binaryExts := map[string]bool{
		".exe": true, ".dll": true, ".so": true, ".dylib": true,
		".jpg": true, ".png": true, ".gif": true, ".pdf": true,
		".zip": true, ".tar": true, ".gz": true, ".bin": true,
	}

	return &BinaryDetector{
		binaryExtensions: binaryExts,
		maxScanBytes:     512,
	}
}

// IsBinary checks if a file is binary (stub implementation)
func (bd *BinaryDetector) IsBinary(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	return bd.binaryExtensions[ext]
}

// NewMimeDetector creates a new MIME detector
func NewMimeDetector() *MimeDetector {
	return &MimeDetector{
		mimeMap: make(map[string]string),
	}
}
