package config

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// WatcherConfiguration contains file system watcher configuration
type WatcherConfiguration struct {
	// Core watcher settings
	Enabled           bool          `yaml:"enabled" json:"enabled"`
	WatchPaths        []string      `yaml:"watch_paths,omitempty" json:"watch_paths,omitempty"`
	IgnorePatterns    []string      `yaml:"ignore_patterns,omitempty" json:"ignore_patterns,omitempty"`
	RecursiveWatch    bool          `yaml:"recursive_watch" json:"recursive_watch"`
	
	// Performance settings
	DebounceInterval  string        `yaml:"debounce_interval,omitempty" json:"debounce_interval,omitempty"`
	BatchSize         int           `yaml:"batch_size,omitempty" json:"batch_size,omitempty"`
	MaxMemoryUsage    int64         `yaml:"max_memory_usage_mb,omitempty" json:"max_memory_usage_mb,omitempty"`
	MaxWatchedFiles   int           `yaml:"max_watched_files,omitempty" json:"max_watched_files,omitempty"`
	
	// Event processing
	EventBufferSize   int           `yaml:"event_buffer_size,omitempty" json:"event_buffer_size,omitempty"`
	MaxRetries        int           `yaml:"max_retries,omitempty" json:"max_retries,omitempty"`
	RetryBackoff      string        `yaml:"retry_backoff,omitempty" json:"retry_backoff,omitempty"`
	
	// Integration settings
	InvalidateCache   bool          `yaml:"invalidate_cache" json:"invalidate_cache"`
	UpdateIndex       bool          `yaml:"update_index" json:"update_index"`
	TriggerReindexing bool          `yaml:"trigger_reindexing" json:"trigger_reindexing"`
	
	// Advanced features
	EnableSymlinkHandling     bool `yaml:"enable_symlink_handling" json:"enable_symlink_handling"`
	EnableResourceMonitoring  bool `yaml:"enable_resource_monitoring" json:"enable_resource_monitoring"`
	EnableBatchOptimization   bool `yaml:"enable_batch_optimization" json:"enable_batch_optimization"`
	
	// Language-specific settings
	LanguageSettings  map[string]*WatcherLanguageConfig `yaml:"language_settings,omitempty" json:"language_settings,omitempty"`
	
	// Integration configuration
	IntegrationConfig *WatcherIntegrationSettings `yaml:"integration,omitempty" json:"integration,omitempty"`
	
	// Event processor configuration
	ProcessorConfig   *WatcherProcessorSettings `yaml:"processor,omitempty" json:"processor,omitempty"`
}

// WatcherLanguageConfig contains language-specific watcher configuration
type WatcherLanguageConfig struct {
	Enabled           bool     `yaml:"enabled" json:"enabled"`
	FileExtensions    []string `yaml:"file_extensions,omitempty" json:"file_extensions,omitempty"`
	IgnorePatterns    []string `yaml:"ignore_patterns,omitempty" json:"ignore_patterns,omitempty"`
	Priority          int      `yaml:"priority,omitempty" json:"priority,omitempty"`
	DebounceInterval  string   `yaml:"debounce_interval,omitempty" json:"debounce_interval,omitempty"`
	EnableIndexing    bool     `yaml:"enable_indexing" json:"enable_indexing"`
	RequiresReindexing bool    `yaml:"requires_reindexing" json:"requires_reindexing"`
}

// WatcherIntegrationSettings contains SCIP integration configuration
type WatcherIntegrationSettings struct {
	// Cache invalidation settings
	EnableCacheInvalidation   bool          `yaml:"enable_cache_invalidation" json:"enable_cache_invalidation"`
	InvalidationDebounce      string        `yaml:"invalidation_debounce,omitempty" json:"invalidation_debounce,omitempty"`
	BatchInvalidation         bool          `yaml:"batch_invalidation" json:"batch_invalidation"`
	
	// Index update settings
	EnableIndexUpdates        bool          `yaml:"enable_index_updates" json:"enable_index_updates"`
	IndexUpdateDebounce       string        `yaml:"index_update_debounce,omitempty" json:"index_update_debounce,omitempty"`
	IncrementalUpdates        bool          `yaml:"incremental_updates" json:"incremental_updates"`
	
	// Re-indexing settings
	EnableReindexing          bool          `yaml:"enable_reindexing" json:"enable_reindexing"`
	ReindexingThreshold       int           `yaml:"reindexing_threshold,omitempty" json:"reindexing_threshold,omitempty"`
	ReindexingBatchSize       int           `yaml:"reindexing_batch_size,omitempty" json:"reindexing_batch_size,omitempty"`
	
	// Performance settings
	MaxConcurrentUpdates      int           `yaml:"max_concurrent_updates,omitempty" json:"max_concurrent_updates,omitempty"`
	UpdateTimeout             string        `yaml:"update_timeout,omitempty" json:"update_timeout,omitempty"`
	QueueSize                 int           `yaml:"queue_size,omitempty" json:"queue_size,omitempty"`
}

// WatcherProcessorSettings contains event processor configuration
type WatcherProcessorSettings struct {
	MaxConcurrentProcessing int    `yaml:"max_concurrent_processing,omitempty" json:"max_concurrent_processing,omitempty"`
	ProcessingTimeout       string `yaml:"processing_timeout,omitempty" json:"processing_timeout,omitempty"`
	EnableLanguageDetection bool   `yaml:"enable_language_detection" json:"enable_language_detection"`
	EnableFileTypeDetection bool   `yaml:"enable_file_type_detection" json:"enable_file_type_detection"`
	EnableBatchProcessing   bool   `yaml:"enable_batch_processing" json:"enable_batch_processing"`
}

// Extend SCIPConfiguration to include watcher settings
func ExtendSCIPConfigurationWithWatcher(scip *SCIPConfiguration) *SCIPConfiguration {
	if scip == nil {
		scip = DefaultSCIPConfiguration()
	}
	
	// Initialize watcher configuration if not present
	// This would be done by modifying the SCIPConfiguration struct, but for now we'll return as-is
	return scip
}

// DefaultSCIPConfiguration returns a default SCIP configuration with watcher settings
func DefaultSCIPConfiguration() *SCIPConfiguration {
	return &SCIPConfiguration{
		Enabled:         false,
		IndexPath:       ".scip",
		AutoRefresh:     false,
		RefreshInterval: 30 * time.Minute,
		FallbackToLSP:   true,
		CacheConfig: CacheConfig{
			Enabled: false,
			TTL:     30 * time.Minute,
			MaxSize: 1000,
		},
		LanguageSettings: map[string]*SCIPLanguageConfig{
			"go": {
				Enabled:      false,
				IndexCommand: []string{"scip-go"},
				IndexTimeout: 10 * time.Minute,
				IndexArguments: []string{
					"--module-path", ".",
					"--output", ".scip/index.scip",
				},
				WorkspaceSettings: make(map[string]interface{}),
			},
			"typescript": {
				Enabled:      false,
				IndexCommand: []string{"scip-typescript"},
				IndexTimeout: 15 * time.Minute,
				IndexArguments: []string{
					"--project-root", ".",
					"--output", ".scip/index.scip",
				},
				WorkspaceSettings: make(map[string]interface{}),
			},
			"python": {
				Enabled:      false,
				IndexCommand: []string{"scip-python"},
				IndexTimeout: 10 * time.Minute,
				IndexArguments: []string{
					"--project-root", ".",
					"--output", ".scip/index.scip",
				},
				WorkspaceSettings: make(map[string]interface{}),
			},
		},
	}
}

// DefaultWatcherConfiguration returns default watcher configuration
func DefaultWatcherConfiguration() *WatcherConfiguration {
	return &WatcherConfiguration{
		Enabled:           false, // Conservative default
		WatchPaths:        []string{"."},
		IgnorePatterns:    DefaultIgnorePatterns(),
		RecursiveWatch:    true,
		DebounceInterval:  "200ms",
		BatchSize:         100,
		MaxMemoryUsage:    100, // 100MB
		MaxWatchedFiles:   100000,
		EventBufferSize:   10000,
		MaxRetries:        3,
		RetryBackoff:      "1s",
		InvalidateCache:   true,
		UpdateIndex:       true,
		TriggerReindexing: false,
		EnableSymlinkHandling:     true,
		EnableResourceMonitoring:  true,
		EnableBatchOptimization:   true,
		LanguageSettings:          DefaultWatcherLanguageSettings(),
		IntegrationConfig:         DefaultWatcherIntegrationSettings(),
		ProcessorConfig:           DefaultWatcherProcessorSettings(),
	}
}

// DefaultIgnorePatterns returns default ignore patterns for file watching
func DefaultIgnorePatterns() []string {
	return []string{
		".git/**",
		".svn/**",
		".hg/**",
		".bzr/**",
		"node_modules/**",
		"vendor/**",
		"target/**",
		"build/**",
		"dist/**",
		"out/**",
		".idea/**",
		".vscode/**",
		".vs/**",
		"*.tmp",
		"*.swp",
		"*.bak",
		"*.log",
		"*.pid",
		"*.lock",
		"*.DS_Store",
		"Thumbs.db",
		"*.pyc",
		"__pycache__/**",
		"*.class",
		"*.jar",
		"*.war",
		"*.exe",
		"*.dll",
		"*.so",
		"*.dylib",
		"*.a",
		"*.lib",
		"*.bin",
		"*.o",
		"*.obj",
	}
}

// DefaultWatcherLanguageSettings returns default language-specific watcher settings
func DefaultWatcherLanguageSettings() map[string]*WatcherLanguageConfig {
	return map[string]*WatcherLanguageConfig{
		"go": {
			Enabled:           true,
			FileExtensions:    []string{".go", ".mod", ".sum"},
			IgnorePatterns:    []string{"vendor/**", "*_test.go"},
			Priority:          1,
			DebounceInterval:  "200ms",
			EnableIndexing:    true,
			RequiresReindexing: true,
		},
		"python": {
			Enabled:           true,
			FileExtensions:    []string{".py", ".pyi", ".pyx"},
			IgnorePatterns:    []string{"__pycache__/**", "*.pyc", ".venv/**", "venv/**"},
			Priority:          2,
			DebounceInterval:  "500ms",
			EnableIndexing:    true,
			RequiresReindexing: true,
		},
		"javascript": {
			Enabled:           true,
			FileExtensions:    []string{".js", ".jsx", ".mjs", ".cjs"},
			IgnorePatterns:    []string{"node_modules/**", "dist/**", "build/**"},
			Priority:          2,
			DebounceInterval:  "300ms",
			EnableIndexing:    true,
			RequiresReindexing: false,
		},
		"typescript": {
			Enabled:           true,
			FileExtensions:    []string{".ts", ".tsx", ".d.ts"},
			IgnorePatterns:    []string{"node_modules/**", "dist/**", "build/**", "*.js.map"},
			Priority:          2,
			DebounceInterval:  "300ms",
			EnableIndexing:    true,
			RequiresReindexing: true,
		},
		"java": {
			Enabled:           true,
			FileExtensions:    []string{".java", ".kt", ".kts"},
			IgnorePatterns:    []string{"target/**", "build/**", "*.class", "*.jar"},
			Priority:          2,
			DebounceInterval:  "400ms",
			EnableIndexing:    true,
			RequiresReindexing: true,
		},
		"rust": {
			Enabled:           true,
			FileExtensions:    []string{".rs", ".toml"},
			IgnorePatterns:    []string{"target/**", "Cargo.lock"},
			Priority:          1,
			DebounceInterval:  "250ms",
			EnableIndexing:    true,
			RequiresReindexing: true,
		},
		"cpp": {
			Enabled:           true,
			FileExtensions:    []string{".cpp", ".cc", ".cxx", ".c", ".h", ".hpp", ".hxx"},
			IgnorePatterns:    []string{"build/**", "*.o", "*.obj", "*.a", "*.lib"},
			Priority:          2,
			DebounceInterval:  "300ms",
			EnableIndexing:    true,
			RequiresReindexing: false,
		},
		"csharp": {
			Enabled:           true,
			FileExtensions:    []string{".cs", ".csx", ".vb"},
			IgnorePatterns:    []string{"bin/**", "obj/**", "*.dll", "*.exe"},
			Priority:          2,
			DebounceInterval:  "350ms",
			EnableIndexing:    true,
			RequiresReindexing: true,
		},
	}
}

// DefaultWatcherIntegrationSettings returns default integration settings
func DefaultWatcherIntegrationSettings() *WatcherIntegrationSettings {
	return &WatcherIntegrationSettings{
		EnableCacheInvalidation:  true,
		InvalidationDebounce:     "500ms",
		BatchInvalidation:        true,
		EnableIndexUpdates:       true,
		IndexUpdateDebounce:      "1s",
		IncrementalUpdates:       true,
		EnableReindexing:         false, // Conservative default
		ReindexingThreshold:      10,
		ReindexingBatchSize:      50,
		MaxConcurrentUpdates:     5,
		UpdateTimeout:            "30s",
		QueueSize:                1000,
	}
}

// DefaultWatcherProcessorSettings returns default processor settings
func DefaultWatcherProcessorSettings() *WatcherProcessorSettings {
	return &WatcherProcessorSettings{
		MaxConcurrentProcessing: 10,
		ProcessingTimeout:       "30s",
		EnableLanguageDetection: true,
		EnableFileTypeDetection: true,
		EnableBatchProcessing:   true,
	}
}

// Validation methods

// Validate validates the watcher configuration
func (wc *WatcherConfiguration) Validate() error {
	if wc == nil {
		return nil // Allow nil configuration
	}
	
	// Validate watch paths
	for i, path := range wc.WatchPaths {
		if strings.TrimSpace(path) == "" {
			return fmt.Errorf("watch path at index %d cannot be empty", i)
		}
		
		// Validate path format (basic checks)
		if strings.Contains(path, "\x00") {
			return fmt.Errorf("watch path at index %d contains null character", i)
		}
	}
	
	// Validate ignore patterns
	for i, pattern := range wc.IgnorePatterns {
		if strings.TrimSpace(pattern) == "" {
			return fmt.Errorf("ignore pattern at index %d cannot be empty", i)
		}
	}
	
	// Validate numeric limits
	if wc.BatchSize < 1 {
		return fmt.Errorf("batch size must be at least 1, got %d", wc.BatchSize)
	}
	if wc.BatchSize > 10000 {
		return fmt.Errorf("batch size too large: %d, maximum 10000", wc.BatchSize)
	}
	
	if wc.MaxMemoryUsage < 0 {
		return fmt.Errorf("max memory usage cannot be negative: %d", wc.MaxMemoryUsage)
	}
	if wc.MaxMemoryUsage > 0 && wc.MaxMemoryUsage < 10 {
		return fmt.Errorf("max memory usage too small: %d MB, minimum 10 MB", wc.MaxMemoryUsage)
	}
	
	if wc.MaxWatchedFiles < 1 {
		return fmt.Errorf("max watched files must be at least 1, got %d", wc.MaxWatchedFiles)
	}
	if wc.MaxWatchedFiles > 1000000 {
		return fmt.Errorf("max watched files too large: %d, maximum 1,000,000", wc.MaxWatchedFiles)
	}
	
	if wc.EventBufferSize < 10 {
		return fmt.Errorf("event buffer size must be at least 10, got %d", wc.EventBufferSize)
	}
	if wc.EventBufferSize > 100000 {
		return fmt.Errorf("event buffer size too large: %d, maximum 100,000", wc.EventBufferSize)
	}
	
	if wc.MaxRetries < 0 {
		return fmt.Errorf("max retries cannot be negative: %d", wc.MaxRetries)
	}
	if wc.MaxRetries > 10 {
		return fmt.Errorf("max retries too large: %d, maximum 10", wc.MaxRetries)
	}
	
	// Validate duration strings
	if wc.DebounceInterval != "" {
		if _, err := time.ParseDuration(wc.DebounceInterval); err != nil {
			return fmt.Errorf("invalid debounce interval '%s': %w", wc.DebounceInterval, err)
		}
	}
	
	if wc.RetryBackoff != "" {
		if _, err := time.ParseDuration(wc.RetryBackoff); err != nil {
			return fmt.Errorf("invalid retry backoff '%s': %w", wc.RetryBackoff, err)
		}
	}
	
	// Validate language settings
	for lang, langConfig := range wc.LanguageSettings {
		if err := langConfig.Validate(lang); err != nil {
			return fmt.Errorf("language config for '%s': %w", lang, err)
		}
	}
	
	// Validate integration settings
	if wc.IntegrationConfig != nil {
		if err := wc.IntegrationConfig.Validate(); err != nil {
			return fmt.Errorf("integration config: %w", err)
		}
	}
	
	// Validate processor settings
	if wc.ProcessorConfig != nil {
		if err := wc.ProcessorConfig.Validate(); err != nil {
			return fmt.Errorf("processor config: %w", err)
		}
	}
	
	return nil
}

// Validate validates language-specific watcher configuration
func (wlc *WatcherLanguageConfig) Validate(language string) error {
	if wlc == nil {
		return nil
	}
	
	// Validate file extensions
	for i, ext := range wlc.FileExtensions {
		if strings.TrimSpace(ext) == "" {
			return fmt.Errorf("file extension at index %d cannot be empty", i)
		}
		
		// Extensions should start with a dot
		if !strings.HasPrefix(ext, ".") && ext != "*" {
			return fmt.Errorf("file extension at index %d should start with '.' or be '*': %s", i, ext)
		}
	}
	
	// Validate priority
	if wlc.Priority < 0 || wlc.Priority > 10 {
		return fmt.Errorf("priority must be between 0 and 10, got %d", wlc.Priority)
	}
	
	// Validate debounce interval
	if wlc.DebounceInterval != "" {
		if _, err := time.ParseDuration(wlc.DebounceInterval); err != nil {
			return fmt.Errorf("invalid debounce interval '%s': %w", wlc.DebounceInterval, err)
		}
	}
	
	return nil
}

// Validate validates integration settings
func (wis *WatcherIntegrationSettings) Validate() error {
	if wis == nil {
		return nil
	}
	
	// Validate duration strings
	if wis.InvalidationDebounce != "" {
		if _, err := time.ParseDuration(wis.InvalidationDebounce); err != nil {
			return fmt.Errorf("invalid invalidation debounce '%s': %w", wis.InvalidationDebounce, err)
		}
	}
	
	if wis.IndexUpdateDebounce != "" {
		if _, err := time.ParseDuration(wis.IndexUpdateDebounce); err != nil {
			return fmt.Errorf("invalid index update debounce '%s': %w", wis.IndexUpdateDebounce, err)
		}
	}
	
	if wis.UpdateTimeout != "" {
		if _, err := time.ParseDuration(wis.UpdateTimeout); err != nil {
			return fmt.Errorf("invalid update timeout '%s': %w", wis.UpdateTimeout, err)
		}
	}
	
	// Validate numeric values
	if wis.ReindexingThreshold < 0 {
		return fmt.Errorf("reindexing threshold cannot be negative: %d", wis.ReindexingThreshold)
	}
	
	if wis.ReindexingBatchSize < 1 {
		return fmt.Errorf("reindexing batch size must be at least 1, got %d", wis.ReindexingBatchSize)
	}
	if wis.ReindexingBatchSize > 1000 {
		return fmt.Errorf("reindexing batch size too large: %d, maximum 1000", wis.ReindexingBatchSize)
	}
	
	if wis.MaxConcurrentUpdates < 1 {
		return fmt.Errorf("max concurrent updates must be at least 1, got %d", wis.MaxConcurrentUpdates)
	}
	if wis.MaxConcurrentUpdates > 100 {
		return fmt.Errorf("max concurrent updates too large: %d, maximum 100", wis.MaxConcurrentUpdates)
	}
	
	if wis.QueueSize < 10 {
		return fmt.Errorf("queue size must be at least 10, got %d", wis.QueueSize)
	}
	if wis.QueueSize > 100000 {
		return fmt.Errorf("queue size too large: %d, maximum 100,000", wis.QueueSize)
	}
	
	return nil
}

// Validate validates processor settings
func (wps *WatcherProcessorSettings) Validate() error {
	if wps == nil {
		return nil
	}
	
	// Validate processing timeout
	if wps.ProcessingTimeout != "" {
		if _, err := time.ParseDuration(wps.ProcessingTimeout); err != nil {
			return fmt.Errorf("invalid processing timeout '%s': %w", wps.ProcessingTimeout, err)
		}
	}
	
	// Validate max concurrent processing
	if wps.MaxConcurrentProcessing < 1 {
		return fmt.Errorf("max concurrent processing must be at least 1, got %d", wps.MaxConcurrentProcessing)
	}
	if wps.MaxConcurrentProcessing > 1000 {
		return fmt.Errorf("max concurrent processing too large: %d, maximum 1000", wps.MaxConcurrentProcessing)
	}
	
	return nil
}

// Helper methods

// GetDebounceInterval returns the debounce interval as a duration
func (wc *WatcherConfiguration) GetDebounceInterval() time.Duration {
	if wc.DebounceInterval == "" {
		return 200 * time.Millisecond // Default
	}
	
	duration, err := time.ParseDuration(wc.DebounceInterval)
	if err != nil {
		return 200 * time.Millisecond // Fallback to default
	}
	
	return duration
}

// GetRetryBackoff returns the retry backoff as a duration
func (wc *WatcherConfiguration) GetRetryBackoff() time.Duration {
	if wc.RetryBackoff == "" {
		return 1 * time.Second // Default
	}
	
	duration, err := time.ParseDuration(wc.RetryBackoff)
	if err != nil {
		return 1 * time.Second // Fallback to default
	}
	
	return duration
}

// GetLanguageConfig returns language-specific configuration
func (wc *WatcherConfiguration) GetLanguageConfig(language string) *WatcherLanguageConfig {
	if wc.LanguageSettings == nil {
		return nil
	}
	
	return wc.LanguageSettings[language]
}

// IsLanguageEnabled checks if watching is enabled for a specific language
func (wc *WatcherConfiguration) IsLanguageEnabled(language string) bool {
	if !wc.Enabled {
		return false
	}
	
	langConfig := wc.GetLanguageConfig(language)
	if langConfig == nil {
		return true // Default to enabled if no specific config
	}
	
	return langConfig.Enabled
}

// ShouldIgnoreFile checks if a file should be ignored based on patterns and language settings
func (wc *WatcherConfiguration) ShouldIgnoreFile(filePath, language string) bool {
	// Check global ignore patterns
	ext := strings.ToLower(filepath.Ext(filePath))
	filename := filepath.Base(filePath)
	
	for _, pattern := range wc.IgnorePatterns {
		if matchesPattern(filePath, filename, ext, pattern) {
			return true
		}
	}
	
	// Check language-specific ignore patterns
	if language != "" {
		langConfig := wc.GetLanguageConfig(language)
		if langConfig != nil {
			for _, pattern := range langConfig.IgnorePatterns {
				if matchesPattern(filePath, filename, ext, pattern) {
					return true
				}
			}
		}
	}
	
	return false
}

// matchesPattern checks if a file matches an ignore pattern (simplified implementation)
func matchesPattern(filePath, filename, ext, pattern string) bool {
	// Simple pattern matching - in production, this would use proper glob matching
	
	// Exact filename match
	if pattern == filename {
		return true
	}
	
	// Extension match
	if pattern == "*"+ext {
		return true
	}
	
	// Directory pattern
	if strings.HasSuffix(pattern, "/**") {
		dirPattern := strings.TrimSuffix(pattern, "/**")
		if strings.Contains(filePath, dirPattern+"/") {
			return true
		}
	}
	
	// Simple wildcard
	if pattern == "*" {
		return true
	}
	
	// Contains check
	if strings.Contains(filePath, strings.TrimSuffix(pattern, "*")) {
		return true
	}
	
	return false
}

// MergeWithDefaults merges configuration with defaults
func (wc *WatcherConfiguration) MergeWithDefaults() *WatcherConfiguration {
	if wc == nil {
		return DefaultWatcherConfiguration()
	}
	
	defaults := DefaultWatcherConfiguration()
	
	// Merge basic settings
	if len(wc.WatchPaths) == 0 {
		wc.WatchPaths = defaults.WatchPaths
	}
	if len(wc.IgnorePatterns) == 0 {
		wc.IgnorePatterns = defaults.IgnorePatterns
	}
	if wc.DebounceInterval == "" {
		wc.DebounceInterval = defaults.DebounceInterval
	}
	if wc.BatchSize == 0 {
		wc.BatchSize = defaults.BatchSize
	}
	if wc.MaxMemoryUsage == 0 {
		wc.MaxMemoryUsage = defaults.MaxMemoryUsage
	}
	if wc.MaxWatchedFiles == 0 {
		wc.MaxWatchedFiles = defaults.MaxWatchedFiles
	}
	if wc.EventBufferSize == 0 {
		wc.EventBufferSize = defaults.EventBufferSize
	}
	if wc.MaxRetries == 0 {
		wc.MaxRetries = defaults.MaxRetries
	}
	if wc.RetryBackoff == "" {
		wc.RetryBackoff = defaults.RetryBackoff
	}
	
	// Merge language settings
	if wc.LanguageSettings == nil {
		wc.LanguageSettings = defaults.LanguageSettings
	} else {
		// Merge missing language configs from defaults
		for lang, defaultConfig := range defaults.LanguageSettings {
			if wc.LanguageSettings[lang] == nil {
				wc.LanguageSettings[lang] = defaultConfig
			}
		}
	}
	
	// Merge integration config
	if wc.IntegrationConfig == nil {
		wc.IntegrationConfig = defaults.IntegrationConfig
	}
	
	// Merge processor config
	if wc.ProcessorConfig == nil {
		wc.ProcessorConfig = defaults.ProcessorConfig
	}
	
	return wc
}