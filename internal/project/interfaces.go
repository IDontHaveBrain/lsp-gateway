package project

import (
	"context"
	"fmt"
	"lsp-gateway/internal/platform"
	"lsp-gateway/internal/project/types"
	"lsp-gateway/internal/setup"
	"time"
)

// ProjectDetector defines the interface for comprehensive project detection and analysis.
// This interface provides methods to detect project types, analyze project structure,
// and determine workspace configuration for LSP Gateway integration.
type ProjectDetector interface {
	// DetectProject performs comprehensive project detection at the specified root path.
	// Returns a ProjectContext containing all detected project information including
	// languages, dependencies, build configuration, and LSP server requirements.
	DetectProject(ctx context.Context, rootPath string) (*ProjectContext, error)

	// GetSupportedLanguages returns the list of programming languages that this
	// detector can identify and analyze. Used for capability discovery and
	// configuration validation.
	GetSupportedLanguages() []string

	// DetectProjectType determines the primary project type (go, python, nodejs, java, etc.)
	// for the specified path. Returns PROJECT_TYPE_MIXED for multi-language projects.
	DetectProjectType(ctx context.Context, rootPath string) (string, error)

	// GetWorkspaceRoot determines the workspace root directory by looking for
	// version control markers (.git, .svn) or project configuration files.
	GetWorkspaceRoot(ctx context.Context, path string) (string, error)

	// ValidateProject validates the project structure and configuration integrity.
	// Checks for required files, dependency consistency, and LSP server compatibility.
	ValidateProject(ctx context.Context, projectCtx *ProjectContext) error

	// ScanWorkspace recursively scans a workspace directory to discover all
	// projects within the specified depth limits and ignore patterns.
	ScanWorkspace(ctx context.Context, workspaceRoot string) ([]*ProjectContext, error)

	// Configuration methods for customizing detection behavior
	SetLogger(logger *setup.SetupLogger)
	SetTimeout(timeout time.Duration)
	SetMaxDepth(depth int)
	SetCustomDetectors(detectors map[string]LanguageDetector)
}

// LanguageDetector defines the interface for language-specific project detection.
// Each language implementation (Go, Python, Node.js, Java, TypeScript) provides
// specialized detection logic for identifying project structure and requirements.
type LanguageDetector interface {
	// DetectLanguage analyzes the specified path for language-specific markers,
	// configuration files, and project structure. Returns detailed detection
	// results including confidence score and required LSP servers.
	DetectLanguage(ctx context.Context, rootPath string) (*types.LanguageDetectionResult, error)

	// GetLanguageInfo returns comprehensive information about the detected language
	// including version requirements, build tools, and dependency management.
	GetLanguageInfo(language string) (*types.LanguageInfo, error)

	// GetMarkerFiles returns the list of files that indicate the presence of
	// this language in a project (e.g., go.mod, package.json, requirements.txt).
	GetMarkerFiles() []string

	// GetRequiredServers returns the list of LSP servers required for this language.
	// Used for automatic server installation and configuration generation.
	GetRequiredServers() []string

	// GetPriority returns the detection priority for this language detector.
	// Higher values take precedence when multiple languages are detected.
	GetPriority() int

	// ValidateStructure performs language-specific validation of project structure,
	// checking for required directories, build files, and dependency configurations.
	ValidateStructure(ctx context.Context, rootPath string) error
}

// ProjectAnalyzer defines the interface for comprehensive project analysis.
// Provides deeper analysis capabilities beyond basic detection including
// dependency analysis, performance profiling, and optimization recommendations.
type ProjectAnalyzer interface {
	// AnalyzeProject performs comprehensive analysis of the project including
	// dependency graphs, build complexity, performance characteristics, and
	// LSP server optimization opportunities.
	AnalyzeProject(ctx context.Context, rootPath string) (*ProjectAnalysisResult, error)

	// GetProjectContext returns the current project context with all analysis
	// results including detected languages, dependencies, and configuration.
	GetProjectContext() (*ProjectContext, error)

	// AnalyzeDependencies performs detailed dependency analysis including
	// version conflicts, security vulnerabilities, and optimization opportunities.
	AnalyzeDependencies(ctx context.Context, projectCtx *ProjectContext) (*DependencyAnalysis, error)

	// AnalyzePerformance evaluates project characteristics that affect LSP
	// server performance including file count, project size, and complexity metrics.
	AnalyzePerformance(ctx context.Context, projectCtx *ProjectContext) (*PerformanceAnalysis, error)

	// GenerateOptimizationRecommendations analyzes project structure and
	// provides recommendations for LSP server configuration and performance tuning.
	GenerateOptimizationRecommendations(ctx context.Context, projectCtx *ProjectContext) (*OptimizationRecommendations, error)

	// Configuration and lifecycle methods
	SetLogger(logger *setup.SetupLogger)
	SetTimeout(timeout time.Duration)
	SetAnalysisDepth(depth AnalysisDepth)
	Reset() error
}

// ConfigurationGenerator defines the interface for generating project-specific
// LSP Gateway configurations. Creates optimized configurations based on
// detected project characteristics and performance requirements.
type ConfigurationGenerator interface {
	// GenerateProjectConfig creates a complete LSP Gateway configuration
	// tailored to the detected project structure and requirements.
	GenerateProjectConfig(ctx context.Context, projectCtx *ProjectContext) (*GeneratedConfig, error)

	// ApplyProjectOptimizations applies performance optimizations to the
	// configuration based on project size, complexity, and detected patterns.
	ApplyProjectOptimizations(ctx context.Context, config *GeneratedConfig, projectCtx *ProjectContext) error

	// GenerateServerConfigs creates language server specific configurations
	// with optimized settings for the detected project characteristics.
	GenerateServerConfigs(ctx context.Context, projectCtx *ProjectContext) (map[string]*ServerConfig, error)

	// ValidateGeneratedConfig validates the generated configuration for
	// completeness, compatibility, and best practices compliance.
	ValidateGeneratedConfig(ctx context.Context, config *GeneratedConfig) (*ConfigValidationResult, error)

	// ExportConfiguration exports the generated configuration in the specified
	// format (YAML, JSON, TOML) with appropriate formatting and documentation.
	ExportConfiguration(ctx context.Context, config *GeneratedConfig, format ConfigFormat) ([]byte, error)

	// Configuration methods
	SetLogger(logger *setup.SetupLogger)
	SetTimeout(timeout time.Duration)
	SetConfigurationTemplate(template *ConfigurationTemplate)
	SetOptimizationLevel(level OptimizationLevel)
}

// Supporting data structures and types

// MemoryEstimates contains memory usage estimates for different operations
type MemoryEstimates struct {
	BaselineMemory     int64   `json:"baseline_memory"`     // bytes
	IndexingMemory     int64   `json:"indexing_memory"`     // bytes
	PeakMemory         int64   `json:"peak_memory"`         // bytes
	RecommendedHeap    int64   `json:"recommended_heap"`    // bytes
	GCOverhead         float64 `json:"gc_overhead"`         // percentage
	MemoryGrowthRate   float64 `json:"memory_growth_rate"`  // bytes per operation
	EstimationAccuracy float64 `json:"estimation_accuracy"` // confidence 0.0-1.0
}

// ProcessingTimeEstimates contains processing time estimates for various operations
type ProcessingTimeEstimates struct {
	IndexingTime       time.Duration `json:"indexing_time"`
	FirstResponseTime  time.Duration `json:"first_response_time"`
	AvgResponseTime    time.Duration `json:"avg_response_time"`
	ComplexQueryTime   time.Duration `json:"complex_query_time"`
	ColdStartTime      time.Duration `json:"cold_start_time"`
	WarmupTime         time.Duration `json:"warmup_time"`
	EstimationAccuracy float64       `json:"estimation_accuracy"` // confidence 0.0-1.0
}

// ResourceRecommendations contains recommendations for resource allocation
type ResourceRecommendations struct {
	CPUCores           int                    `json:"cpu_cores"`
	MemoryMB           int                    `json:"memory_mb"`
	DiskSpaceGB        int                    `json:"disk_space_gb"`
	ConcurrentClients  int                    `json:"concurrent_clients"`
	ThreadPoolSize     int                    `json:"thread_pool_size"`
	CacheSize          int64                  `json:"cache_size"`
	IOBufferSize       int                    `json:"io_buffer_size"`
	NetworkTimeout     time.Duration          `json:"network_timeout"`
	AdditionalSettings map[string]interface{} `json:"additional_settings,omitempty"`
	Justification      string                 `json:"justification"`
}

// ConfigOptimization represents a configuration optimization recommendation
type ConfigOptimization struct {
	ID               string               `json:"id"`
	Title            string               `json:"title"`
	Description      string               `json:"description"`
	Category         string               `json:"category"`
	Priority         OptimizationPriority `json:"priority"`
	EstimatedImpact  string               `json:"estimated_impact"`
	ConfigPath       string               `json:"config_path"`
	CurrentValue     interface{}          `json:"current_value,omitempty"`
	RecommendedValue interface{}          `json:"recommended_value"`
	Rationale        string               `json:"rationale"`
	Implementation   string               `json:"implementation"`
	RiskLevel        string               `json:"risk_level"`
	Prerequisites    []string             `json:"prerequisites,omitempty"`
}

// PerformanceOptimization represents a performance optimization recommendation
type PerformanceOptimization struct {
	ID                  string               `json:"id"`
	Title               string               `json:"title"`
	Description         string               `json:"description"`
	Category            string               `json:"category"`
	Priority            OptimizationPriority `json:"priority"`
	ExpectedImprovement string               `json:"expected_improvement"`
	PerformanceMetric   string               `json:"performance_metric"`
	CurrentValue        interface{}          `json:"current_value,omitempty"`
	TargetValue         interface{}          `json:"target_value"`
	Implementation      string               `json:"implementation"`
	RiskLevel           string               `json:"risk_level"`
	EstimatedEffort     string               `json:"estimated_effort"`
	Dependencies        []string             `json:"dependencies,omitempty"`
}

// ResourceOptimization represents a resource optimization recommendation
type ResourceOptimization struct {
	ID               string               `json:"id"`
	Title            string               `json:"title"`
	Description      string               `json:"description"`
	ResourceType     string               `json:"resource_type"` // cpu, memory, disk, network
	Priority         OptimizationPriority `json:"priority"`
	CurrentUsage     interface{}          `json:"current_usage"`
	OptimizedUsage   interface{}          `json:"optimized_usage"`
	SavingsEstimate  string               `json:"savings_estimate"`
	Implementation   string               `json:"implementation"`
	MonitoringPoints []string             `json:"monitoring_points,omitempty"`
	RiskLevel        string               `json:"risk_level"`
	Prerequisites    []string             `json:"prerequisites,omitempty"`
}

// SecurityOptimization represents a security optimization recommendation
type SecurityOptimization struct {
	ID                  string               `json:"id"`
	Title               string               `json:"title"`
	Description         string               `json:"description"`
	SecurityDomain      string               `json:"security_domain"` // authentication, authorization, encryption, etc.
	Priority            OptimizationPriority `json:"priority"`
	ThreatMitigation    []string             `json:"threat_mitigation"`
	ComplianceStandards []string             `json:"compliance_standards,omitempty"`
	Implementation      string               `json:"implementation"`
	RiskReduction       string               `json:"risk_reduction"`
	AuditRequirements   []string             `json:"audit_requirements,omitempty"`
	Prerequisites       []string             `json:"prerequisites,omitempty"`
}

// WorkflowOptimization represents a workflow optimization recommendation
type WorkflowOptimization struct {
	ID                  string               `json:"id"`
	Title               string               `json:"title"`
	Description         string               `json:"description"`
	WorkflowArea        string               `json:"workflow_area"` // development, testing, deployment, etc.
	Priority            OptimizationPriority `json:"priority"`
	CurrentWorkflow     string               `json:"current_workflow"`
	OptimizedWorkflow   string               `json:"optimized_workflow"`
	EfficiencyGain      string               `json:"efficiency_gain"`
	Implementation      string               `json:"implementation"`
	ToolingRequirements []string             `json:"tooling_requirements,omitempty"`
	TrainingNeeded      bool                 `json:"training_needed"`
	RiskLevel           string               `json:"risk_level"`
}

// OptimizationImpact represents the estimated impact of optimization recommendations
type OptimizationImpact struct {
	OverallScore          float64                `json:"overall_score"`          // 0.0-1.0
	PerformanceGain       float64                `json:"performance_gain"`       // percentage improvement
	ResourceSavings       float64                `json:"resource_savings"`       // percentage savings
	SecurityImprovement   float64                `json:"security_improvement"`   // 0.0-1.0 risk reduction
	DeveloperProductivity float64                `json:"developer_productivity"` // percentage improvement
	ImplementationCost    string                 `json:"implementation_cost"`    // low, medium, high
	MaintenanceBurden     string                 `json:"maintenance_burden"`     // low, medium, high
	RiskAssessment        string                 `json:"risk_assessment"`        // low, medium, high
	TimeToImplement       time.Duration          `json:"time_to_implement"`
	TimeToRealizeValue    time.Duration          `json:"time_to_realize_value"`
	QuantifiedBenefits    map[string]interface{} `json:"quantified_benefits,omitempty"`
	Assumptions           []string               `json:"assumptions,omitempty"`
}

// DependencyAnalysis contains detailed dependency analysis results
type DependencyAnalysis struct {
	TotalDependencies       int                      `json:"total_dependencies"`
	DirectDependencies      int                      `json:"direct_dependencies"`
	TransitiveDependencies  int                      `json:"transitive_dependencies"`
	DependencyGraph         map[string][]string      `json:"dependency_graph"`
	VersionConflicts        []*VersionConflict       `json:"version_conflicts"`
	SecurityVulnerabilities []*SecurityVulnerability `json:"security_vulnerabilities"`
	OutdatedDependencies    []*OutdatedDependency    `json:"outdated_dependencies"`
	LicenseConflicts        []*LicenseConflict       `json:"license_conflicts"`
	CircularDependencies    [][]string               `json:"circular_dependencies"`
	RecommendedUpdates      map[string]string        `json:"recommended_updates"`
	RiskScore               float64                  `json:"risk_score"` // 0.0-1.0
}

// PerformanceAnalysis contains project performance characteristics
type PerformanceAnalysis struct {
	ProjectSize           ProjectSize              `json:"project_size"`
	FileSystemMetrics     *FileSystemMetrics       `json:"filesystem_metrics"`
	BuildComplexity       *BuildComplexity         `json:"build_complexity"`
	LSPPerformanceProfile *LSPPerformanceProfile   `json:"lsp_performance_profile"`
	MemoryEstimates       *MemoryEstimates         `json:"memory_estimates"`
	ProcessingTime        *ProcessingTimeEstimates `json:"processing_time"`
	RecommendedResources  *ResourceRecommendations `json:"recommended_resources"`
	PerformanceRating     PerformanceRating        `json:"performance_rating"`
}

// SecurityAnalysis contains security-related analysis results
type SecurityAnalysis struct {
	SecurityRating          SecurityRating           `json:"security_rating"`
	VulnerabilityCount      int                      `json:"vulnerability_count"`
	CriticalVulnerabilities []*SecurityVulnerability `json:"critical_vulnerabilities"`
	SecretScanResults       []*SecretScanResult      `json:"secret_scan_results"`
	InsecurePatterns        []*InsecurePattern       `json:"insecure_patterns"`
	RecommendedActions      []string                 `json:"recommended_actions"`
	ComplianceStatus        map[string]bool          `json:"compliance_status"`
}

// GeneratedConfig contains the complete generated LSP Gateway configuration
type GeneratedConfig struct {
	Version              string                   `json:"version"`
	GeneratedAt          time.Time                `json:"generated_at"`
	ProjectInfo          *ProjectContext          `json:"project_info"`
	GlobalConfig         *GlobalConfiguration     `json:"global_config"`
	ServerConfigs        map[string]*ServerConfig `json:"server_configs"`
	OptimizationSettings *OptimizationSettings    `json:"optimization_settings"`
	PerformanceTuning    *PerformanceTuning       `json:"performance_tuning"`
	ValidationResults    *ConfigValidationResult  `json:"validation_results,omitempty"`
	Metadata             map[string]interface{}   `json:"metadata,omitempty"`
}

// ConfigValidationResult contains configuration validation results
type ConfigValidationResult struct {
	IsValid                bool                  `json:"is_valid"`
	ValidationErrors       []string              `json:"validation_errors,omitempty"`
	ValidationWarnings     []string              `json:"validation_warnings,omitempty"`
	CompatibilityIssues    []*CompatibilityIssue `json:"compatibility_issues,omitempty"`
	PerformanceWarnings    []string              `json:"performance_warnings,omitempty"`
	SecurityIssues         []string              `json:"security_issues,omitempty"`
	BestPracticeViolations []string              `json:"best_practice_violations,omitempty"`
	RecommendedFixes       map[string]string     `json:"recommended_fixes,omitempty"`
}

// OptimizationRecommendations contains performance and configuration optimization recommendations
type OptimizationRecommendations struct {
	ConfigOptimizations      []*ConfigOptimization      `json:"config_optimizations"`
	PerformanceOptimizations []*PerformanceOptimization `json:"performance_optimizations"`
	ResourceOptimizations    []*ResourceOptimization    `json:"resource_optimizations"`
	SecurityOptimizations    []*SecurityOptimization    `json:"security_optimizations"`
	WorkflowOptimizations    []*WorkflowOptimization    `json:"workflow_optimizations"`
	EstimatedImpact          *OptimizationImpact        `json:"estimated_impact"`
	ImplementationGuide      *ImplementationGuide       `json:"implementation_guide"`
	Priority                 OptimizationPriority       `json:"priority"`
}

// Enumeration types for configuration and analysis

// AnalysisDepth defines the depth level for project analysis
type AnalysisDepth int

const (
	AnalysisDepthBasic         AnalysisDepth = iota // Basic detection and validation
	AnalysisDepthStandard                           // Standard analysis with dependencies
	AnalysisDepthComprehensive                      // Full analysis including performance
	AnalysisDepthDeep                               // Deep analysis with security scanning
)

// ConfigFormat defines supported configuration export formats
type ConfigFormat int

const (
	ConfigFormatYAML ConfigFormat = iota
	ConfigFormatJSON
	ConfigFormatTOML
)

// OptimizationLevel defines the level of configuration optimization to apply
type OptimizationLevel int

const (
	OptimizationLevelConservative OptimizationLevel = iota // Minimal optimizations
	OptimizationLevelBalanced                              // Balanced performance/compatibility
	OptimizationLevelAggressive                            // Maximum performance optimizations
	OptimizationLevelCustom                                // User-defined optimization rules
)

// PerformanceRating categorizes project performance characteristics
type PerformanceRating int

const (
	PerformanceRatingExcellent PerformanceRating = iota // < 1s startup, minimal memory
	PerformanceRatingGood                               // 1-3s startup, moderate memory
	PerformanceRatingFair                               // 3-10s startup, high memory
	PerformanceRatingPoor                               // > 10s startup, very high memory
)

// SecurityRating categorizes project security posture
type SecurityRating int

const (
	SecurityRatingExcellent SecurityRating = iota // No vulnerabilities, secure practices
	SecurityRatingGood                            // Minor issues, mostly secure
	SecurityRatingFair                            // Some vulnerabilities, needs attention
	SecurityRatingPoor                            // Critical vulnerabilities, high risk
)

// OptimizationPriority defines the priority level for optimization recommendations
type OptimizationPriority int

const (
	OptimizationPriorityCritical OptimizationPriority = iota // Must implement immediately
	OptimizationPriorityHigh                                 // Should implement soon
	OptimizationPriorityMedium                               // Should implement eventually
	OptimizationPriorityLow                                  // Nice to have
)

// Supporting structures for detailed analysis results

// VersionConflict represents a dependency version conflict
type VersionConflict struct {
	Package             string   `json:"package"`
	ConflictingVersions []string `json:"conflicting_versions"`
	RequiredBy          []string `json:"required_by"`
	Severity            string   `json:"severity"`
	Resolution          string   `json:"resolution,omitempty"`
}

// SecurityVulnerability represents a security vulnerability in a dependency
type SecurityVulnerability struct {
	CVEID            string   `json:"cve_id,omitempty"`
	Package          string   `json:"package"`
	AffectedVersions []string `json:"affected_versions"`
	Severity         string   `json:"severity"`
	Description      string   `json:"description"`
	FixedInVersion   string   `json:"fixed_in_version,omitempty"`
	References       []string `json:"references,omitempty"`
}

// OutdatedDependency represents an outdated dependency
type OutdatedDependency struct {
	Package         string `json:"package"`
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version"`
	SecurityUpdates bool   `json:"security_updates"`
	BreakingChanges bool   `json:"breaking_changes"`
	UpdatePriority  string `json:"update_priority"`
}

// LicenseConflict represents a license compatibility conflict
type LicenseConflict struct {
	Package        string   `json:"package"`
	License        string   `json:"license"`
	ConflictsWith  []string `json:"conflicts_with"`
	ConflictType   string   `json:"conflict_type"`
	Recommendation string   `json:"recommendation"`
}

// MemoryUsageProfile contains memory usage metrics and profiling data
type MemoryUsageProfile struct {
	CurrentUsage      int64    `json:"current_usage"`   // bytes
	PeakUsage         int64    `json:"peak_usage"`      // bytes
	AverageUsage      int64    `json:"average_usage"`   // bytes
	GCPressure        float64  `json:"gc_pressure"`     // 0.0-1.0
	AllocationRate    int64    `json:"allocation_rate"` // bytes/second
	LeakSuspicion     bool     `json:"leak_suspicion"`
	OptimizationHints []string `json:"optimization_hints,omitempty"`
}

// FileSystemMetrics contains file system related performance metrics
type FileSystemMetrics struct {
	TotalFiles            int      `json:"total_files"`
	TotalDirectories      int      `json:"total_directories"`
	AverageFileSize       int64    `json:"average_file_size"`
	LargestFiles          []string `json:"largest_files"`
	DeepestNesting        int      `json:"deepest_nesting"`
	SymlinkCount          int      `json:"symlink_count"`
	BinaryFileCount       int      `json:"binary_file_count"`
	AccessPatterns        []string `json:"access_patterns"`
	IOIntensiveOperations []string `json:"io_intensive_operations"`
}

// BuildComplexity contains build system complexity metrics
type BuildComplexity struct {
	BuildSystemType     string        `json:"build_system_type"`
	ConfigurationFiles  []string      `json:"configuration_files"`
	BuildSteps          int           `json:"build_steps"`
	Dependencies        int           `json:"dependencies"`
	ParallelizableSteps int           `json:"parallelizable_steps"`
	EstimatedBuildTime  time.Duration `json:"estimated_build_time"`
	ComplexityScore     float64       `json:"complexity_score"` // 0.0-1.0
}

// LSPPerformanceProfile contains LSP server performance characteristics
type LSPPerformanceProfile struct {
	IndexingTime          time.Duration            `json:"indexing_time"`
	MemoryUsage           *MemoryUsageProfile      `json:"memory_usage"`
	ResponseTimes         map[string]time.Duration `json:"response_times"`
	ConcurrentConnections int                      `json:"concurrent_connections"`
	CacheEffectiveness    float64                  `json:"cache_effectiveness"`
	RecommendedSettings   map[string]interface{}   `json:"recommended_settings"`
}

// Error types and utility functions

// NewProjectDetectionError creates a new error for project detection failures
func NewProjectDetectionError(projectType, path, message string, cause error) error {
	return platform.NewPlatformError(
		"PROJECT_DETECTION_FAILED",
		message,
		fmt.Sprintf("Failed to detect project at %s", path),
		"Check if the path contains a valid project structure",
	).WithSystemInfo(map[string]interface{}{
		"project_type": projectType,
		"path":         path,
	}).WithCause(cause)
}

// NewLanguageDetectionError creates a new error for language detection failures
func NewLanguageDetectionError(language, path, message string, cause error) error {
	return platform.NewPlatformError(
		"LANGUAGE_DETECTION_FAILED",
		message,
		fmt.Sprintf("Failed to detect %s language at %s", language, path),
		"Ensure the project contains valid language-specific files",
	).WithSystemInfo(map[string]interface{}{
		"language": language,
		"path":     path,
	}).WithCause(cause)
}

// NewProjectAnalysisError creates a new error for project analysis failures
func NewProjectAnalysisError(operation, message string, projectCtx *ProjectContext, cause error) error {
	info := map[string]interface{}{
		"operation": operation,
	}
	if projectCtx != nil {
		info["project_type"] = projectCtx.ProjectType
		info["project_path"] = projectCtx.RootPath
	}

	return platform.NewPlatformError(
		"PROJECT_ANALYSIS_FAILED",
		message,
		fmt.Sprintf("Project analysis operation '%s' failed", operation),
		"Review project structure and try again",
	).WithSystemInfo(info).WithCause(cause)
}

// NewConfigurationGenerationError creates a new error for configuration generation failures
func NewConfigurationGenerationError(operation, message string, cause error) error {
	return platform.NewPlatformError(
		"CONFIG_GENERATION_FAILED",
		message,
		fmt.Sprintf("Configuration generation operation '%s' failed", operation),
		"Check project analysis results and try regenerating configuration",
	).WithSystemInfo(map[string]interface{}{
		"operation": operation,
	}).WithCause(cause)
}

// Additional types for complete interface definitions

// ServerConfig contains language server specific configuration
type ServerConfig struct {
	Name           string                 `json:"name"`
	Languages      []string               `json:"languages"`
	Command        string                 `json:"command"`
	Args           []string               `json:"args,omitempty"`
	Transport      string                 `json:"transport"` // stdio, tcp
	RootMarkers    []string               `json:"root_markers,omitempty"`
	Settings       map[string]interface{} `json:"settings,omitempty"`
	InitOptions    map[string]interface{} `json:"init_options,omitempty"`
	Timeout        time.Duration          `json:"timeout,omitempty"`
	MaxConcurrency int                    `json:"max_concurrency,omitempty"`
	Enabled        bool                   `json:"enabled"`
}

// ConfigurationTemplate contains template for configuration generation
type ConfigurationTemplate struct {
	Name            string                 `json:"name"`
	Description     string                 `json:"description"`
	Version         string                 `json:"version"`
	TargetLanguages []string               `json:"target_languages"`
	Template        map[string]interface{} `json:"template"`
	Variables       map[string]interface{} `json:"variables,omitempty"`
	Conditions      []string               `json:"conditions,omitempty"`
	Documentation   string                 `json:"documentation,omitempty"`
}

// SecretScanResult contains results from secret scanning
type SecretScanResult struct {
	ID              string   `json:"id"`
	SecretType      string   `json:"secret_type"` // api_key, token, password, etc.
	FilePath        string   `json:"file_path"`
	LineNumber      int      `json:"line_number"`
	ColumnStart     int      `json:"column_start"`
	ColumnEnd       int      `json:"column_end"`
	Value           string   `json:"value,omitempty"` // Masked/redacted value
	Confidence      float64  `json:"confidence"`      // 0.0-1.0
	Severity        string   `json:"severity"`        // critical, high, medium, low
	Description     string   `json:"description"`
	Recommendations []string `json:"recommendations,omitempty"`
	Verified        bool     `json:"verified"` // Whether secret was validated
}

// InsecurePattern contains information about insecure coding patterns
type InsecurePattern struct {
	ID          string   `json:"id"`
	Pattern     string   `json:"pattern"`      // Regex or description of pattern
	PatternType string   `json:"pattern_type"` // sql_injection, xss, hardcoded_secret, etc.
	FilePath    string   `json:"file_path"`
	LineNumber  int      `json:"line_number"`
	CodeSnippet string   `json:"code_snippet"`
	Severity    string   `json:"severity"` // critical, high, medium, low
	Description string   `json:"description"`
	Remediation string   `json:"remediation"`
	References  []string `json:"references,omitempty"`
	CWENumber   string   `json:"cwe_number,omitempty"`
}

// GlobalConfiguration contains global LSP Gateway configuration settings
type GlobalConfiguration struct {
	Port                  int                    `json:"port"`
	Host                  string                 `json:"host"`
	Timeout               time.Duration          `json:"timeout"`
	MaxConcurrentRequests int                    `json:"max_concurrent_requests"`
	LogLevel              string                 `json:"log_level"`
	EnableMetrics         bool                   `json:"enable_metrics"`
	EnableTracing         bool                   `json:"enable_tracing"`
	CacheSettings         map[string]interface{} `json:"cache_settings,omitempty"`
	SecuritySettings      map[string]interface{} `json:"security_settings,omitempty"`
	HealthCheck           map[string]interface{} `json:"health_check,omitempty"`
	RateLimiting          map[string]interface{} `json:"rate_limiting,omitempty"`
	CORS                  map[string]interface{} `json:"cors,omitempty"`
}

// OptimizationSettings contains optimization configuration settings
type OptimizationSettings struct {
	Level               OptimizationLevel      `json:"level"`
	EnableCaching       bool                   `json:"enable_caching"`
	CacheSize           int64                  `json:"cache_size"`
	EnablePreloading    bool                   `json:"enable_preloading"`
	PreloadPatterns     []string               `json:"preload_patterns,omitempty"`
	EnableCompression   bool                   `json:"enable_compression"`
	CompressionLevel    int                    `json:"compression_level"`
	EnableBatching      bool                   `json:"enable_batching"`
	BatchSize           int                    `json:"batch_size"`
	CustomOptimizations map[string]interface{} `json:"custom_optimizations,omitempty"`
	PerformanceTargets  map[string]interface{} `json:"performance_targets,omitempty"`
}

// PerformanceTuning contains performance tuning parameters
type PerformanceTuning struct {
	MemoryLimit        int64                  `json:"memory_limit"` // bytes
	CPULimit           float64                `json:"cpu_limit"`    // CPU cores
	ThreadPoolSize     int                    `json:"thread_pool_size"`
	IOBufferSize       int                    `json:"io_buffer_size"`
	NetworkTimeout     time.Duration          `json:"network_timeout"`
	GCSettings         map[string]interface{} `json:"gc_settings,omitempty"`
	JVMSettings        map[string]interface{} `json:"jvm_settings,omitempty"`
	ProcessPriority    int                    `json:"process_priority"`
	EnableProfiling    bool                   `json:"enable_profiling"`
	ProfilingSettings  map[string]interface{} `json:"profiling_settings,omitempty"`
	ResourceMonitoring map[string]interface{} `json:"resource_monitoring,omitempty"`
}

// CompatibilityIssue represents a compatibility issue found during validation
type CompatibilityIssue struct {
	ID                string   `json:"id"`
	Title             string   `json:"title"`
	Description       string   `json:"description"`
	IssueType         string   `json:"issue_type"` // version, platform, dependency, etc.
	Severity          string   `json:"severity"`   // critical, high, medium, low
	AffectedComponent string   `json:"affected_component"`
	MinimumVersion    string   `json:"minimum_version,omitempty"`
	MaximumVersion    string   `json:"maximum_version,omitempty"`
	Platforms         []string `json:"platforms,omitempty"`
	Workarounds       []string `json:"workarounds,omitempty"`
	Resolution        string   `json:"resolution,omitempty"`
	References        []string `json:"references,omitempty"`
}

// ImplementationGuide contains guidance for implementing optimizations
type ImplementationGuide struct {
	Overview                string                `json:"overview"`
	Prerequisites           []string              `json:"prerequisites,omitempty"`
	ImplementationSteps     []*ImplementationStep `json:"implementation_steps"`
	TestingGuidelines       []string              `json:"testing_guidelines,omitempty"`
	RollbackProcedure       []string              `json:"rollback_procedure,omitempty"`
	MonitoringPoints        []string              `json:"monitoring_points,omitempty"`
	TroubleshootingTips     []string              `json:"troubleshooting_tips,omitempty"`
	BestPractices           []string              `json:"best_practices,omitempty"`
	EstimatedTimeToComplete time.Duration         `json:"estimated_time_to_complete"`
	RequiredSkills          []string              `json:"required_skills,omitempty"`
	RiskMitigation          map[string]string     `json:"risk_mitigation,omitempty"`
}

// ImplementationStep represents a single step in the implementation guide
type ImplementationStep struct {
	ID              string        `json:"id"`
	Title           string        `json:"title"`
	Description     string        `json:"description"`
	Commands        []string      `json:"commands,omitempty"`
	ExpectedOutput  string        `json:"expected_output,omitempty"`
	ValidationSteps []string      `json:"validation_steps,omitempty"`
	EstimatedTime   time.Duration `json:"estimated_time"`
	Dependencies    []string      `json:"dependencies,omitempty"`
	RiskLevel       string        `json:"risk_level"`
	Notes           string        `json:"notes,omitempty"`
}
