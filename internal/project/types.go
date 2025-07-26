package project

import (
	"encoding/json"
	"time"
)

// AnalysisStatus represents the status of project analysis
type AnalysisStatus string

const (
	AnalysisStatusSuccess    AnalysisStatus = "success"
	AnalysisStatusFailed     AnalysisStatus = "failed"
	AnalysisStatusPartial    AnalysisStatus = "partial"
	AnalysisStatusTimeout    AnalysisStatus = "timeout"
	AnalysisStatusInProgress AnalysisStatus = "in_progress"
)

// ConfidenceLevel represents detection confidence levels
type ConfidenceLevel string

const (
	ConfidenceLevelHigh   ConfidenceLevel = "high"   // 0.8-1.0
	ConfidenceLevelMedium ConfidenceLevel = "medium" // 0.5-0.8
	ConfidenceLevelLow    ConfidenceLevel = "low"    // 0.2-0.5
	ConfidenceLevelNone   ConfidenceLevel = "none"   // 0.0-0.2
)

// Analysis thresholds and limits
const (
	DefaultAnalysisTimeout    = 30 * time.Second
	DefaultMaxFiles           = 10000
	DefaultMaxAnalysisDepth   = 5
	LargeProjectFileThreshold = 1000
	HugeProjectFileThreshold  = 5000
	HighConfidenceThreshold   = 0.8
	MediumConfidenceThreshold = 0.5
	LowConfidenceThreshold    = 0.2
)

// ProjectAnalysisResult contains the complete result of project detection and analysis
// This structure unifies both comprehensive analysis results and simple CLI integration needs
type ProjectAnalysisResult struct {
	// Core identification and metadata
	Path           string         `json:"path"`            // Original input path
	NormalizedPath string         `json:"normalized_path"` // Normalized absolute path
	Timestamp      time.Time      `json:"timestamp"`       // When analysis was performed
	Duration       time.Duration  `json:"duration"`        // Total analysis time
	Status         AnalysisStatus `json:"status"`          // Overall analysis status

	// Core project information
	ProjectContext *ProjectContext `json:"project_context,omitempty"` // Main project detection results
	ProjectConfig  interface{}     `json:"project_config,omitempty"`  // Generated configuration

	// Analysis results and metrics
	ProjectSize     ProjectSize     `json:"project_size"`     // Project size metrics for CLI timeout adjustments
	AnalysisDepth   int             `json:"analysis_depth"`   // Depth of analysis performed
	DetectionScore  float64         `json:"detection_score"`  // Overall detection confidence (0-1)
	ConfidenceLevel ConfidenceLevel `json:"confidence_level"` // Human-readable confidence level

	// Error and warning information
	Errors   []AnalysisError   `json:"errors,omitempty"`   // Analysis errors
	Warnings []AnalysisWarning `json:"warnings,omitempty"` // Analysis warnings
	Issues   []string          `json:"issues,omitempty"`   // General issues found

	// Performance and resource metrics
	PerformanceMetrics PerformanceMetrics `json:"performance_metrics"` // Analysis performance data

	// Validation and recommendations
	ValidationErrors   []string         `json:"validation_errors,omitempty"`   // Project validation errors
	ValidationWarnings []string         `json:"validation_warnings,omitempty"` // Project validation warnings
	Recommendations    []Recommendation `json:"recommendations,omitempty"`     // Improvement recommendations

	// Additional metadata and context
	Metadata           map[string]interface{} `json:"metadata,omitempty"`            // Additional analysis metadata
	RequiredServers    []string               `json:"required_servers"`              // LSP servers needed
	DetectedFrameworks []DetectedFramework    `json:"detected_frameworks,omitempty"` // Framework information
}

// ProjectSize contains metrics about project size for timeout and performance adjustments
type ProjectSize struct {
	TotalFiles      int   `json:"total_files"`       // Total number of files (used by CLI for timeouts)
	SourceFiles     int   `json:"source_files"`      // Number of source code files
	TestFiles       int   `json:"test_files"`        // Number of test files
	ConfigFiles     int   `json:"config_files"`      // Number of configuration files
	TotalLines      int   `json:"total_lines"`       // Total lines of code (estimated)
	TotalSizeBytes  int64 `json:"total_size_bytes"`  // Total size in bytes
	LinesOfCode     int   `json:"lines_of_code"`     // Actual lines of code (excluding comments/whitespace)
	AverageFileSize int64 `json:"average_file_size"` // Average file size in bytes
}

// AnalysisError represents a structured error during analysis
type AnalysisError struct {
	Type        string                 `json:"type"`                  // Error type/category
	Phase       string                 `json:"phase"`                 // Analysis phase where error occurred
	Message     string                 `json:"message"`               // Error message
	Path        string                 `json:"path,omitempty"`        // File/directory path related to error
	Details     string                 `json:"details,omitempty"`     // Additional error details
	Severity    ErrorSeverity          `json:"severity"`              // Error severity level
	Recoverable bool                   `json:"recoverable"`           // Whether error is recoverable
	Suggestions []string               `json:"suggestions,omitempty"` // Recovery suggestions
	Metadata    map[string]interface{} `json:"metadata,omitempty"`    // Additional error metadata
	Timestamp   time.Time              `json:"timestamp"`             // When error occurred
}

// AnalysisWarning represents a non-critical issue during analysis
type AnalysisWarning struct {
	Type        string                 `json:"type"`                  // Warning type/category
	Phase       string                 `json:"phase"`                 // Analysis phase where warning occurred
	Message     string                 `json:"message"`               // Warning message
	Path        string                 `json:"path,omitempty"`        // File/directory path related to warning
	Details     string                 `json:"details,omitempty"`     // Additional warning details
	Severity    WarningSeverity        `json:"severity"`              // Warning severity level
	Suggestions []string               `json:"suggestions,omitempty"` // Improvement suggestions
	Metadata    map[string]interface{} `json:"metadata,omitempty"`    // Additional warning metadata
	Timestamp   time.Time              `json:"timestamp"`             // When warning occurred
}

// ErrorSeverity represents the severity level of analysis errors
type ErrorSeverity string

const (
	ErrorSeverityCritical ErrorSeverity = "critical" // Analysis cannot continue
	ErrorSeverityHigh     ErrorSeverity = "high"     // Major functionality affected
	ErrorSeverityMedium   ErrorSeverity = "medium"   // Minor functionality affected
	ErrorSeverityLow      ErrorSeverity = "low"      // Cosmetic or minor issues
)

// WarningSeverity represents the severity level of analysis warnings
type WarningSeverity string

const (
	WarningSeverityHigh   WarningSeverity = "high"   // Should be addressed soon
	WarningSeverityMedium WarningSeverity = "medium" // Should be considered
	WarningSeverityLow    WarningSeverity = "low"    // Optional improvement
	WarningSeverityInfo   WarningSeverity = "info"   // Informational only
)

// PerformanceMetrics contains timing and resource usage data
type PerformanceMetrics struct {
	AnalysisTime         time.Duration `json:"analysis_time"`          // Total analysis time
	DetectionTime        time.Duration `json:"detection_time"`         // Time spent on detection
	ValidationTime       time.Duration `json:"validation_time"`        // Time spent on validation
	ConfigGenerationTime time.Duration `json:"config_generation_time"` // Time spent generating config
	FilesScanned         int           `json:"files_scanned"`          // Number of files scanned
	DirsTraversed        int           `json:"dirs_traversed"`         // Number of directories traversed
	MemoryUsageBytes     int64         `json:"memory_usage_bytes"`     // Peak memory usage
	CacheHits            int           `json:"cache_hits"`             // Cache hit count
	CacheMisses          int           `json:"cache_misses"`           // Cache miss count
}

// Recommendation represents an improvement suggestion
type Recommendation struct {
	Type        string                 `json:"type"`               // Recommendation type
	Priority    RecommendationPriority `json:"priority"`           // Priority level
	Title       string                 `json:"title"`              // Short title
	Description string                 `json:"description"`        // Detailed description
	Actions     []string               `json:"actions,omitempty"`  // Suggested actions
	Benefits    []string               `json:"benefits,omitempty"` // Expected benefits
	Metadata    map[string]interface{} `json:"metadata,omitempty"` // Additional data
}

// RecommendationPriority represents recommendation priority levels
type RecommendationPriority string

const (
	RecommendationPriorityHigh   RecommendationPriority = "high"   // Should implement immediately
	RecommendationPriorityMedium RecommendationPriority = "medium" // Should implement soon
	RecommendationPriorityLow    RecommendationPriority = "low"    // Nice to have
)

// DetectedFramework represents a detected framework or library
type DetectedFramework struct {
	Name        string                 `json:"name"`                   // Framework name
	Version     string                 `json:"version,omitempty"`      // Framework version
	Type        FrameworkType          `json:"type"`                   // Framework type
	Confidence  float64                `json:"confidence"`             // Detection confidence (0-1)
	Path        string                 `json:"path,omitempty"`         // Path where detected
	ConfigFiles []string               `json:"config_files,omitempty"` // Related config files
	Metadata    map[string]interface{} `json:"metadata,omitempty"`     // Framework-specific data
}

// FrameworkType represents different types of frameworks
type FrameworkType string

const (
	FrameworkTypeWeb      FrameworkType = "web"      // Web frameworks
	FrameworkTypeAPI      FrameworkType = "api"      // API frameworks
	FrameworkTypeDatabase FrameworkType = "database" // Database frameworks
	FrameworkTypeTesting  FrameworkType = "testing"  // Testing frameworks
	FrameworkTypeBuild    FrameworkType = "build"    // Build tools
	FrameworkTypeUtility  FrameworkType = "utility"  // Utility libraries
	FrameworkTypeUnknown  FrameworkType = "unknown"  // Unknown type
)

// Helper methods for ProjectAnalysisResult

// IsSuccessful returns true if analysis completed successfully
func (par *ProjectAnalysisResult) IsSuccessful() bool {
	return par.Status == AnalysisStatusSuccess
}

// HasErrors returns true if analysis encountered errors
func (par *ProjectAnalysisResult) HasErrors() bool {
	return len(par.Errors) > 0 || len(par.ValidationErrors) > 0
}

// HasWarnings returns true if analysis encountered warnings
func (par *ProjectAnalysisResult) HasWarnings() bool {
	return len(par.Warnings) > 0 || len(par.ValidationWarnings) > 0
}

// GetConfidenceLevel returns the human-readable confidence level
func (par *ProjectAnalysisResult) GetConfidenceLevel() ConfidenceLevel {
	if par.DetectionScore >= HighConfidenceThreshold {
		return ConfidenceLevelHigh
	} else if par.DetectionScore >= MediumConfidenceThreshold {
		return ConfidenceLevelMedium
	} else if par.DetectionScore >= LowConfidenceThreshold {
		return ConfidenceLevelLow
	}
	return ConfidenceLevelNone
}

// IsLargeProject returns true if this is considered a large project
func (par *ProjectAnalysisResult) IsLargeProject() bool {
	return par.ProjectSize.TotalFiles > LargeProjectFileThreshold
}

// IsHugeProject returns true if this is considered a huge project
func (par *ProjectAnalysisResult) IsHugeProject() bool {
	return par.ProjectSize.TotalFiles > HugeProjectFileThreshold
}

// GetSuggestedTimeout returns a suggested timeout based on project size
func (par *ProjectAnalysisResult) GetSuggestedTimeout() time.Duration {
	baseTimeout := DefaultAnalysisTimeout
	if par.IsHugeProject() {
		return baseTimeout * 4
	} else if par.IsLargeProject() {
		return baseTimeout * 2
	}
	return baseTimeout
}

// ToJSON returns a JSON representation of the analysis result
func (par *ProjectAnalysisResult) ToJSON() ([]byte, error) {
	return json.MarshalIndent(par, "", "  ")
}

// FromJSON creates a ProjectAnalysisResult from JSON data
func FromJSON(data []byte) (*ProjectAnalysisResult, error) {
	var result ProjectAnalysisResult
	err := json.Unmarshal(data, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}
