package project

import (
	"fmt"
	"lsp-gateway/internal/setup"
	"strings"
	"time"
)

// IntegrationConfig defines configuration options for project integration and analysis
type IntegrationConfig struct {
	// Analysis timeout settings
	AnalysisTimeout time.Duration `yaml:"analysis_timeout" json:"analysis_timeout"`
	FileTimeout     time.Duration `yaml:"file_timeout" json:"file_timeout"`

	// Detection depth limits
	DetectionDepth    int `yaml:"detection_depth" json:"detection_depth"`
	MaxDirectoryDepth int `yaml:"max_directory_depth" json:"max_directory_depth"`

	// Language detection thresholds
	MinFileCount            int     `yaml:"min_file_count" json:"min_file_count"`
	LanguageConfidenceLimit float64 `yaml:"language_confidence_limit" json:"language_confidence_limit"`
	MaxFilesPerLanguage     int     `yaml:"max_files_per_language" json:"max_files_per_language"`

	// Performance optimization settings
	EnableParallelAnalysis bool `yaml:"enable_parallel_analysis" json:"enable_parallel_analysis"`
	MaxConcurrentWorkers   int  `yaml:"max_concurrent_workers" json:"max_concurrent_workers"`
	EnableCaching          bool `yaml:"enable_caching" json:"enable_caching"`
	CacheSize              int  `yaml:"cache_size" json:"cache_size"`

	// Logging and output preferences
	LogLevel       string `yaml:"log_level" json:"log_level"`
	EnableVerbose  bool   `yaml:"enable_verbose" json:"enable_verbose"`
	EnableProgress bool   `yaml:"enable_progress" json:"enable_progress"`
	EnableMetrics  bool   `yaml:"enable_metrics" json:"enable_metrics"`
	QuietMode      bool   `yaml:"quiet_mode" json:"quiet_mode"`
	OutputFormat   string `yaml:"output_format" json:"output_format"`

	// Project-specific settings
	SkipHiddenFiles     bool     `yaml:"skip_hidden_files" json:"skip_hidden_files"`
	IgnorePatterns      []string `yaml:"ignore_patterns" json:"ignore_patterns"`
	IncludePatterns     []string `yaml:"include_patterns" json:"include_patterns"`
	FollowSymlinks      bool     `yaml:"follow_symlinks" json:"follow_symlinks"`
	EnableErrorRecovery bool     `yaml:"enable_error_recovery" json:"enable_error_recovery"`
}

// DefaultIntegrationConfig returns sensible defaults for all settings
func DefaultIntegrationConfig() *IntegrationConfig {
	return &IntegrationConfig{
		// Analysis timeout settings (matching CLI usage)
		AnalysisTimeout: 60 * time.Second,
		FileTimeout:     5 * time.Second,

		// Detection depth limits (matching language detector)
		DetectionDepth:    3,
		MaxDirectoryDepth: 10,

		// Language detection thresholds for large projects
		MinFileCount:            1,
		LanguageConfidenceLimit: 0.1,
		MaxFilesPerLanguage:     1000,

		// Performance optimization settings
		EnableParallelAnalysis: true,
		MaxConcurrentWorkers:   4,
		EnableCaching:          true,
		CacheSize:              1000,

		// Logging preferences (matching setup patterns)
		LogLevel:       "info",
		EnableVerbose:  false,
		EnableProgress: true,
		EnableMetrics:  true,
		QuietMode:      false,
		OutputFormat:   "text",

		// Project-specific settings
		SkipHiddenFiles: true,
		IgnorePatterns: []string{
			"node_modules", ".git", ".vscode", ".idea",
			"*.tmp", "*.log", "*.cache", "build", "dist",
		},
		IncludePatterns:     []string{},
		FollowSymlinks:      false,
		EnableErrorRecovery: true,
	}
}

// Validate performs comprehensive validation with helpful error messages
func (c *IntegrationConfig) Validate() error {
	// Validate timeout ranges
	if c.AnalysisTimeout < 10*time.Second {
		return fmt.Errorf("analysis timeout too short: %v, minimum 10 seconds", c.AnalysisTimeout)
	}
	if c.AnalysisTimeout > 30*time.Minute {
		return fmt.Errorf("analysis timeout too long: %v, maximum 30 minutes", c.AnalysisTimeout)
	}

	if c.FileTimeout < 1*time.Second {
		return fmt.Errorf("file timeout too short: %v, minimum 1 second", c.FileTimeout)
	}
	if c.FileTimeout > c.AnalysisTimeout {
		return fmt.Errorf("file timeout cannot exceed analysis timeout: %v > %v", c.FileTimeout, c.AnalysisTimeout)
	}

	// Check depth limits
	if c.DetectionDepth < 1 {
		return fmt.Errorf("detection depth must be at least 1, got: %d", c.DetectionDepth)
	}
	if c.DetectionDepth > 10 {
		return fmt.Errorf("detection depth too deep: %d, maximum 10 levels", c.DetectionDepth)
	}

	if c.MaxDirectoryDepth < c.DetectionDepth {
		return fmt.Errorf("max directory depth cannot be less than detection depth: %d < %d", c.MaxDirectoryDepth, c.DetectionDepth)
	}
	if c.MaxDirectoryDepth > 50 {
		return fmt.Errorf("max directory depth too deep: %d, maximum 50 levels", c.MaxDirectoryDepth)
	}

	// Ensure reasonable threshold values
	if c.MinFileCount < 0 {
		return fmt.Errorf("min file count cannot be negative: %d", c.MinFileCount)
	}

	if c.LanguageConfidenceLimit < 0.0 || c.LanguageConfidenceLimit > 1.0 {
		return fmt.Errorf("language confidence limit must be between 0.0 and 1.0, got: %f", c.LanguageConfidenceLimit)
	}

	if c.MaxFilesPerLanguage < 1 {
		return fmt.Errorf("max files per language must be at least 1, got: %d", c.MaxFilesPerLanguage)
	}

	// Validate performance settings
	if c.MaxConcurrentWorkers < 1 {
		return fmt.Errorf("max concurrent workers must be at least 1, got: %d", c.MaxConcurrentWorkers)
	}
	if c.MaxConcurrentWorkers > 100 {
		return fmt.Errorf("max concurrent workers too high: %d, maximum 100", c.MaxConcurrentWorkers)
	}

	if c.CacheSize < 0 {
		return fmt.Errorf("cache size cannot be negative: %d", c.CacheSize)
	}

	// Validate logging configuration
	validLogLevels := map[string]bool{
		"trace": true, "debug": true, "info": true,
		"warn": true, "error": true, "fatal": true,
	}
	if !validLogLevels[strings.ToLower(c.LogLevel)] {
		return fmt.Errorf("invalid log level: %s, must be one of: trace, debug, info, warn, error, fatal", c.LogLevel)
	}

	validOutputFormats := map[string]bool{
		"text": true, "json": true, "yaml": true,
	}
	if !validOutputFormats[strings.ToLower(c.OutputFormat)] {
		return fmt.Errorf("invalid output format: %s, must be one of: text, json, yaml", c.OutputFormat)
	}

	// Validate pattern arrays
	for i, pattern := range c.IgnorePatterns {
		if strings.TrimSpace(pattern) == "" {
			return fmt.Errorf("ignore pattern at index %d cannot be empty or whitespace-only", i)
		}
	}

	return nil
}

// ToLoggerConfig converts integration config to setup logger configuration
func (c *IntegrationConfig) ToLoggerConfig(component string) *setup.SetupLoggerConfig {
	var logLevel setup.LogLevel
	switch strings.ToLower(c.LogLevel) {
	case "trace":
		logLevel = setup.LogLevelTrace
	case "debug":
		logLevel = setup.LogLevelDebug
	case "warn":
		logLevel = setup.LogLevelWarn
	case "error":
		logLevel = setup.LogLevelError
	case "fatal":
		logLevel = setup.LogLevelFatal
	default:
		logLevel = setup.LogLevelInfo
	}

	return &setup.SetupLoggerConfig{
		Level:                  logLevel,
		Component:              component,
		EnableJSON:             c.OutputFormat == "json",
		EnableUserMessages:     !c.QuietMode,
		EnableProgressTracking: c.EnableProgress,
		EnableMetrics:          c.EnableMetrics,
		VerboseMode:            c.EnableVerbose,
		QuietMode:              c.QuietMode,
	}
}

// Builder pattern support for fluent configuration construction
type IntegrationConfigBuilder struct {
	config *IntegrationConfig
}

func NewIntegrationConfigBuilder() *IntegrationConfigBuilder {
	return &IntegrationConfigBuilder{
		config: DefaultIntegrationConfig(),
	}
}

func (b *IntegrationConfigBuilder) WithTimeout(timeout time.Duration) *IntegrationConfigBuilder {
	b.config.AnalysisTimeout = timeout
	return b
}

func (b *IntegrationConfigBuilder) WithDepth(depth int) *IntegrationConfigBuilder {
	b.config.DetectionDepth = depth
	return b
}

func (b *IntegrationConfigBuilder) WithConcurrency(workers int) *IntegrationConfigBuilder {
	b.config.MaxConcurrentWorkers = workers
	return b
}

func (b *IntegrationConfigBuilder) WithLogLevel(level string) *IntegrationConfigBuilder {
	b.config.LogLevel = level
	return b
}

func (b *IntegrationConfigBuilder) WithQuietMode(quiet bool) *IntegrationConfigBuilder {
	b.config.QuietMode = quiet
	return b
}

func (b *IntegrationConfigBuilder) Build() (*IntegrationConfig, error) {
	if err := b.config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	return b.config, nil
}
