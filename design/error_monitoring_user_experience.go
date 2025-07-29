// Error Monitoring, Alerting, and User Experience Design
// Comprehensive monitoring, alerting, and user experience guidelines for the 
// enhanced sub-project routing system error handling and fallback strategies.

package design

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// =====================================================================================
// MONITORING AND ALERTING FRAMEWORK
// =====================================================================================

// AlertSeverity defines the severity levels for alerts
type AlertSeverity string

const (
	AlertSeverityInfo     AlertSeverity = "info"
	AlertSeverityWarning  AlertSeverity = "warning"
	AlertSeverityError    AlertSeverity = "error"
	AlertSeverityCritical AlertSeverity = "critical"
)

// AlertType categorizes different types of alerts
type AlertType string

const (
	AlertTypeErrorRate       AlertType = "error_rate"
	AlertTypeFallbackFailure AlertType = "fallback_failure"
	AlertTypeCircuitOpen     AlertType = "circuit_open"
	AlertTypeResourceExhaustion AlertType = "resource_exhaustion"
	AlertTypePerformanceDegradation AlertType = "performance_degradation"
	AlertTypeSystemHealth    AlertType = "system_health"
)

// Alert represents a monitoring alert
type Alert struct {
	ID          string                 `json:"id"`
	Type        AlertType              `json:"type"`
	Severity    AlertSeverity          `json:"severity"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Context     map[string]interface{} `json:"context"`
	Timestamp   time.Time              `json:"timestamp"`
	Source      string                 `json:"source"`
	Resolved    bool                   `json:"resolved"`
	ResolvedAt  *time.Time             `json:"resolved_at,omitempty"`
}

// AlertRule defines conditions that trigger alerts
type AlertRule struct {
	ID               string              `json:"id"`
	Name             string              `json:"name"`
	Type             AlertType           `json:"type"`
	Severity         AlertSeverity       `json:"severity"`
	Condition        AlertCondition      `json:"condition"`
	EvaluationWindow time.Duration       `json:"evaluation_window"`
	Cooldown         time.Duration       `json:"cooldown"`
	Enabled          bool                `json:"enabled"`
	Recipients       []string            `json:"recipients"`
	Actions          []AlertAction       `json:"actions"`
}

// AlertCondition defines the condition that triggers an alert
type AlertCondition struct {
	Metric    string      `json:"metric"`
	Operator  string      `json:"operator"` // "gt", "lt", "eq", "gte", "lte"
	Threshold interface{} `json:"threshold"`
	Duration  time.Duration `json:"duration"`
}

// AlertAction defines actions to take when an alert is triggered
type AlertAction struct {
	Type   string                 `json:"type"` // "email", "slack", "webhook", "restart"
	Config map[string]interface{} `json:"config"`
}

// MonitoringSystem handles comprehensive monitoring and alerting
type MonitoringSystem struct {
	metrics     *MetricsCollector
	alertRules  []AlertRule
	activeAlerts map[string]*Alert
	notifier    AlertNotifier
	storage     MetricsStorage
	logger      Logger
	config      *MonitoringConfig
	mu          sync.RWMutex
}

// MonitoringConfig configures the monitoring system
type MonitoringConfig struct {
	EnableMonitoring     bool          `yaml:"enable_monitoring" json:"enable_monitoring"`
	EnableAlerting       bool          `yaml:"enable_alerting" json:"enable_alerting"`
	MetricsRetention     time.Duration `yaml:"metrics_retention" json:"metrics_retention"`
	AlertEvaluationInterval time.Duration `yaml:"alert_evaluation_interval" json:"alert_evaluation_interval"`
	EnableDetailedMetrics bool         `yaml:"enable_detailed_metrics" json:"enable_detailed_metrics"`
	EnableUserMetrics    bool          `yaml:"enable_user_metrics" json:"enable_user_metrics"`
}

// =====================================================================================
// METRICS COLLECTION AND STORAGE
// =====================================================================================

// MetricsCollector collects detailed metrics for monitoring and analysis
type MetricsCollector struct {
	// Error metrics
	errorRates           map[string]*RateCounter     // errors per minute by type
	errorCounts          map[string]*Counter         // total error counts
	errorDurations       map[string]*DurationTracker // error handling durations
	
	// Fallback metrics
	fallbackUsage        map[string]*Counter         // fallback strategy usage
	fallbackSuccess      map[string]*RateCounter     // fallback success rates
	fallbackDurations    map[string]*DurationTracker // fallback response times
	
	// Circuit breaker metrics
	circuitStates        map[string]*StateTracker    // circuit breaker states
	circuitTransitions   map[string]*Counter         // state transition counts
	
	// Performance metrics
	responseTimeP95      map[string]*PercentileTracker // 95th percentile response times
	responseTimeP99      map[string]*PercentileTracker // 99th percentile response times
	requestThroughput    map[string]*RateCounter       // requests per second
	
	// Resource metrics
	memoryUsage          *GaugeMetric                  // memory usage
	cpuUsage             *GaugeMetric                  // CPU usage
	connectionCounts     map[string]*GaugeMetric       // active connections
	
	// User experience metrics
	userSatisfaction     *UserSatisfactionTracker      // user satisfaction scores
	errorImpact          map[string]*ImpactTracker     // user impact of errors
	
	mu sync.RWMutex
}

// Counter tracks simple counts
type Counter struct {
	value int64
	mu    sync.RWMutex
}

// RateCounter tracks rates over time windows
type RateCounter struct {
	samples []TimestampedValue
	window  time.Duration
	mu      sync.RWMutex
}

// DurationTracker tracks duration statistics
type DurationTracker struct {
	samples []time.Duration
	maxSamples int
	mu      sync.RWMutex
}

// StateTracker tracks state changes over time
type StateTracker struct {
	currentState string
	history      []StateChange
	mu           sync.RWMutex
}

// PercentileTracker tracks percentile statistics
type PercentileTracker struct {
	samples    []float64
	maxSamples int
	mu         sync.RWMutex
}

// GaugeMetric tracks current values
type GaugeMetric struct {
	value float64
	mu    sync.RWMutex
}

// UserSatisfactionTracker tracks user satisfaction metrics
type UserSatisfactionTracker struct {
	scores    []UserSatisfactionScore
	mu        sync.RWMutex
}

// ImpactTracker tracks the impact of errors on users
type ImpactTracker struct {
	incidents []UserImpactIncident
	mu        sync.RWMutex
}

// Supporting types
type TimestampedValue struct {
	Value     float64
	Timestamp time.Time
}

type StateChange struct {
	From      string
	To        string
	Timestamp time.Time
}

type UserSatisfactionScore struct {
	Score     int       // 1-10 scale
	Context   string    // what operation was being performed
	Timestamp time.Time
}

type UserImpactIncident struct {
	ErrorType     SubProjectErrorType
	UsersAffected int
	Duration      time.Duration
	Severity      ErrorImpactSeverity
	Timestamp     time.Time
}

type ErrorImpactSeverity string

const (
	ImpactLow      ErrorImpactSeverity = "low"
	ImpactMedium   ErrorImpactSeverity = "medium"
	ImpactHigh     ErrorImpactSeverity = "high"
	ImpactCritical ErrorImpactSeverity = "critical"
)

// =====================================================================================
// ALERTING RULES AND THRESHOLDS
// =====================================================================================

// DefaultAlertRules returns a set of default alert rules for sub-project routing
func DefaultAlertRules() []AlertRule {
	return []AlertRule{
		{
			ID:       "high_error_rate",
			Name:     "High Error Rate",
			Type:     AlertTypeErrorRate,
			Severity: AlertSeverityWarning,
			Condition: AlertCondition{
				Metric:    "error_rate_per_minute",
				Operator:  "gt",
				Threshold: 10.0, // More than 10 errors per minute
				Duration:  5 * time.Minute,
			},
			EvaluationWindow: 1 * time.Minute,
			Cooldown:         15 * time.Minute,
			Enabled:          true,
		},
		{
			ID:       "critical_error_rate",
			Name:     "Critical Error Rate",
			Type:     AlertTypeErrorRate,
			Severity: AlertSeverityCritical,
			Condition: AlertCondition{
				Metric:    "error_rate_per_minute",
				Operator:  "gt",
				Threshold: 50.0, // More than 50 errors per minute
				Duration:  2 * time.Minute,
			},
			EvaluationWindow: 30 * time.Second,
			Cooldown:         5 * time.Minute,
			Enabled:          true,
		},
		{
			ID:       "fallback_failure_rate",
			Name:     "High Fallback Failure Rate",
			Type:     AlertTypeFallbackFailure,
			Severity: AlertSeverityError,
			Condition: AlertCondition{
				Metric:    "fallback_failure_rate",
				Operator:  "gt",
				Threshold: 0.5, // More than 50% failure rate
				Duration:  10 * time.Minute,
			},
			EvaluationWindow: 2 * time.Minute,
			Cooldown:         20 * time.Minute,
			Enabled:          true,
		},
		{
			ID:       "circuit_breaker_open",
			Name:     "Circuit Breaker Open",
			Type:     AlertTypeCircuitOpen,
			Severity: AlertSeverityWarning,
			Condition: AlertCondition{
				Metric:    "circuit_open_duration",
				Operator:  "gt",
				Threshold: 60.0, // Open for more than 1 minute
				Duration:  1 * time.Minute,
			},
			EvaluationWindow: 30 * time.Second,
			Cooldown:         10 * time.Minute,
			Enabled:          true,
		},
		{
			ID:       "performance_degradation",
			Name:     "Performance Degradation",
			Type:     AlertTypePerformanceDegradation,
			Severity: AlertSeverityWarning,
			Condition: AlertCondition{
				Metric:    "response_time_p95",
				Operator:  "gt",
				Threshold: 5000.0, // P95 response time > 5 seconds
				Duration:  5 * time.Minute,
			},
			EvaluationWindow: 1 * time.Minute,
			Cooldown:         15 * time.Minute,
			Enabled:          true,
		},
		{
			ID:       "resource_exhaustion",
			Name:     "Resource Exhaustion",
			Type:     AlertTypeResourceExhaustion,
			Severity: AlertSeverityError,
			Condition: AlertCondition{
				Metric:    "memory_usage_percent",
				Operator:  "gt",
				Threshold: 90.0, // Memory usage > 90%
				Duration:  3 * time.Minute,
			},
			EvaluationWindow: 1 * time.Minute,
			Cooldown:         10 * time.Minute,
			Enabled:          true,
		},
	}
}

// =====================================================================================
// USER EXPERIENCE AND ERROR REPORTING
// =====================================================================================

// UserExperienceManager handles user-facing error reporting and experience optimization
type UserExperienceManager struct {
	errorFormatter  ErrorFormatter
	progressTracker ProgressTracker
	diagnostics     DiagnosticsCollector
	feedback        FeedbackCollector
	recovery        RecoveryGuidance
	config          *UserExperienceConfig
	logger          Logger
}

// UserExperienceConfig configures user experience settings
type UserExperienceConfig struct {
	EnableProgressReporting   bool          `yaml:"enable_progress_reporting" json:"enable_progress_reporting"`
	EnableDiagnostics        bool          `yaml:"enable_diagnostics" json:"enable_diagnostics"`
	EnableRecoveryGuidance   bool          `yaml:"enable_recovery_guidance" json:"enable_recovery_guidance"`
	ShowDetailedErrors       bool          `yaml:"show_detailed_errors" json:"show_detailed_errors"`
	EnableFeedbackCollection bool          `yaml:"enable_feedback_collection" json:"enable_feedback_collection"`
	ProgressUpdateInterval   time.Duration `yaml:"progress_update_interval" json:"progress_update_interval"`
}

// UserFacingError represents an error formatted for user consumption
type UserFacingError struct {
	Title              string                 `json:"title"`
	Description        string                 `json:"description"`
	UserActions        []UserAction           `json:"user_actions"`
	TechnicalDetails   *TechnicalDetails      `json:"technical_details,omitempty"`
	EstimatedRecoveryTime time.Duration       `json:"estimated_recovery_time"`
	ProgressUpdates    []ProgressUpdate       `json:"progress_updates,omitempty"`
	SupportContext     map[string]interface{} `json:"support_context"`
	Severity           string                 `json:"severity"`
	ErrorCode          string                 `json:"error_code"`
	Timestamp          time.Time              `json:"timestamp"`
}

// UserAction represents an action the user can take
type UserAction struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Action      string `json:"action"` // "retry", "report", "ignore", "configure"
	Automated   bool   `json:"automated"`
	Priority    int    `json:"priority"`
}

// TechnicalDetails provides technical information for advanced users
type TechnicalDetails struct {
	ErrorType    string                 `json:"error_type"`
	StackTrace   string                 `json:"stack_trace,omitempty"`
	SystemInfo   map[string]interface{} `json:"system_info"`
	RequestInfo  map[string]interface{} `json:"request_info"`
	Diagnostics  []DiagnosticResult     `json:"diagnostics"`
}

// ProgressUpdate represents progress in error recovery
type ProgressUpdate struct {
	Stage       string    `json:"stage"`
	Description string    `json:"description"`
	Progress    int       `json:"progress"` // 0-100
	Timestamp   time.Time `json:"timestamp"`
	Status      string    `json:"status"` // "in_progress", "completed", "failed"
}

// DiagnosticResult represents the result of a diagnostic check
type DiagnosticResult struct {
	Check       string                 `json:"check"`
	Status      string                 `json:"status"` // "pass", "fail", "warning"
	Message     string                 `json:"message"`
	Details     map[string]interface{} `json:"details,omitempty"`
	Suggestion  string                 `json:"suggestion,omitempty"`
}

// =====================================================================================
// ERROR FORMATTING AND USER GUIDANCE
// =====================================================================================

// ErrorFormatter formats errors for user consumption
type ErrorFormatter interface {
	FormatError(err *SubProjectRoutingError) *UserFacingError
	FormatRecoveryProgress(stage string, progress int) *ProgressUpdate
	FormatDiagnostics(results []DiagnosticResult) *TechnicalDetails
}

// DefaultErrorFormatter provides default error formatting
type DefaultErrorFormatter struct {
	config *UserExperienceConfig
	logger Logger
}

// FormatError formats a SubProjectRoutingError for user display
func (f *DefaultErrorFormatter) FormatError(err *SubProjectRoutingError) *UserFacingError {
	userError := &UserFacingError{
		ErrorCode:             string(err.Type),
		Severity:              f.formatSeverity(err.Severity),
		Timestamp:             err.Timestamp,
		EstimatedRecoveryTime: f.estimateRecoveryTime(err),
		SupportContext:        f.generateSupportContext(err),
	}
	
	// Format based on error type
	switch err.Type {
	case ErrorTypeProjectResolution:
		userError.Title = "Project Detection Issue"
		userError.Description = "Unable to determine which project contains this file. The file may be outside of known project boundaries or the project configuration may be incomplete."
		userError.UserActions = []UserAction{
			{
				Title:       "Check Project Structure",
				Description: "Ensure the file is within a recognized project directory with proper configuration files",
				Action:      "configure",
				Priority:    1,
			},
			{
				Title:       "Retry Operation",
				Description: "Try the operation again as this might be a temporary issue",
				Action:      "retry",
				Automated:   true,
				Priority:    2,
			},
		}
		
	case ErrorTypeClientConnection:
		userError.Title = "Language Server Connection Issue"
		userError.Description = "Unable to connect to the language server for this project. The server may be starting up or experiencing issues."
		userError.UserActions = []UserAction{
			{
				Title:       "Wait and Retry",
				Description: "Language servers sometimes take time to initialize",
				Action:      "retry",
				Automated:   true,
				Priority:    1,
			},
			{
				Title:       "Check Server Status",
				Description: "Verify that the language server is properly installed and configured",
				Action:      "configure",
				Priority:    2,
			},
		}
		
	case ErrorTypeClientTimeout:
		userError.Title = "Operation Timeout"
		userError.Description = "The language server took too long to respond. This might be due to large project size or server performance issues."
		userError.UserActions = []UserAction{
			{
				Title:       "Retry with Longer Timeout",
				Description: "Try the operation again with extended timeout",
				Action:      "retry",
				Automated:   true,
				Priority:    1,
			},
			{
				Title:       "Use Cached Results",
				Description: "Fall back to previously cached results if available",
				Action:      "fallback",
				Automated:   true,
				Priority:    2,
			},
		}
		
	default:
		userError.Title = "Unexpected Error"
		userError.Description = "An unexpected error occurred. Please try again or report this issue if it persists."
		userError.UserActions = []UserAction{
			{
				Title:       "Retry Operation",
				Description: "Try the operation again",
				Action:      "retry",
				Priority:    1,
			},
			{
				Title:       "Report Issue",
				Description: "Report this issue to help improve the system",
				Action:      "report",
				Priority:    2,
			},
		}
	}
	
	// Add technical details if enabled
	if f.config.ShowDetailedErrors {
		userError.TechnicalDetails = &TechnicalDetails{
			ErrorType:   string(err.Type),
			SystemInfo:  err.Metadata,
			RequestInfo: f.extractRequestInfo(err.Context),
		}
	}
	
	return userError
}

// formatSeverity formats severity for user display
func (f *DefaultErrorFormatter) formatSeverity(severity ErrorSeverity) string {
	switch severity {
	case SeverityInfo:
		return "info"
	case SeverityWarning:
		return "warning"
	case SeverityError:
		return "error"
	case SeverityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// estimateRecoveryTime estimates how long recovery might take
func (f *DefaultErrorFormatter) estimateRecoveryTime(err *SubProjectRoutingError) time.Duration {
	switch err.Type {
	case ErrorTypeClientConnection:
		return 30 * time.Second
	case ErrorTypeClientTimeout:
		return 10 * time.Second
	case ErrorTypeProjectResolution:
		return 5 * time.Second
	default:
		return 15 * time.Second
	}
}

// generateSupportContext generates context information for support
func (f *DefaultErrorFormatter) generateSupportContext(err *SubProjectRoutingError) map[string]interface{} {
	context := make(map[string]interface{})
	
	if err.Context != nil {
		context["operation"] = err.Context.Operation
		context["file_uri"] = err.Context.FileURI
		context["lsp_method"] = err.Context.LSPMethod
		context["project_root"] = err.Context.ProjectRoot
		context["workspace_root"] = err.Context.WorkspaceRoot
		context["attempt_number"] = err.Context.AttemptNumber
	}
	
	context["error_type"] = string(err.Type)
	context["timestamp"] = err.Timestamp.Unix()
	context["request_id"] = err.RequestID
	context["session_id"] = err.SessionID
	
	return context
}

// extractRequestInfo extracts relevant request information
func (f *DefaultErrorFormatter) extractRequestInfo(ctx *ErrorContext) map[string]interface{} {
	if ctx == nil {
		return nil
	}
	
	return map[string]interface{}{
		"method":         ctx.LSPMethod,
		"file_uri":       ctx.FileURI,
		"client_type":    ctx.ClientType,
		"language":       ctx.Language,
		"attempt":        ctx.AttemptNumber,
		"elapsed_time":   ctx.ElapsedTime.String(),
	}
}

// =====================================================================================
// PROGRESS TRACKING AND RECOVERY GUIDANCE
// =====================================================================================

// ProgressTracker tracks recovery progress for user feedback
type ProgressTracker interface {
	StartProgress(operationID string, stages []string) error
	UpdateProgress(operationID string, stage string, progress int, message string) error
	CompleteProgress(operationID string, success bool) error
	GetProgress(operationID string) (*RecoveryProgress, error)
}

// RecoveryProgress represents the current state of error recovery
type RecoveryProgress struct {
	OperationID     string           `json:"operation_id"`
	CurrentStage    string           `json:"current_stage"`
	TotalStages     int              `json:"total_stages"`
	CompletedStages int              `json:"completed_stages"`
	Progress        int              `json:"progress"` // 0-100
	Message         string           `json:"message"`
	Updates         []ProgressUpdate `json:"updates"`
	StartTime       time.Time        `json:"start_time"`
	EstimatedEnd    *time.Time       `json:"estimated_end,omitempty"`
	Status          string           `json:"status"` // "running", "completed", "failed"
}

// RecoveryGuidance provides step-by-step guidance for error recovery
type RecoveryGuidance interface {
	GetRecoverySteps(errorType SubProjectErrorType, context *ErrorContext) []RecoveryStep
	ExecuteRecoveryStep(stepID string, context *ErrorContext) (*RecoveryStepResult, error)
	ValidateRecovery(errorType SubProjectErrorType, context *ErrorContext) (*RecoveryValidation, error)
}

// RecoveryStep represents a step in the recovery process
type RecoveryStep struct {
	ID          string                 `json:"id"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Command     string                 `json:"command,omitempty"`
	Automated   bool                   `json:"automated"`
	Required    bool                   `json:"required"`
	EstimatedTime time.Duration        `json:"estimated_time"`
	Prerequisites []string             `json:"prerequisites,omitempty"`
	ValidationCheck string             `json:"validation_check,omitempty"`
}

// RecoveryStepResult represents the result of executing a recovery step
type RecoveryStepResult struct {
	StepID    string                 `json:"step_id"`
	Success   bool                   `json:"success"`
	Message   string                 `json:"message"`
	Output    string                 `json:"output,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Duration  time.Duration          `json:"duration"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// RecoveryValidation represents the result of validating error recovery
type RecoveryValidation struct {
	IsRecovered    bool                   `json:"is_recovered"`
	Confidence     float64                `json:"confidence"` // 0.0-1.0
	RemainingIssues []string              `json:"remaining_issues,omitempty"`
	Recommendations []string              `json:"recommendations,omitempty"`
	TestResults    []ValidationTestResult `json:"test_results"`
}

// ValidationTestResult represents the result of a validation test
type ValidationTestResult struct {
	Test    string `json:"test"`
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
}

// =====================================================================================
// DIAGNOSTICS AND TROUBLESHOOTING
// =====================================================================================

// DiagnosticsCollector collects diagnostic information for troubleshooting
type DiagnosticsCollector interface {
	RunDiagnostics(context *ErrorContext) []DiagnosticResult
	RunTargetedDiagnostics(errorType SubProjectErrorType, context *ErrorContext) []DiagnosticResult
	CollectSystemInfo() map[string]interface{}
	GenerateTroubleshootingReport(err *SubProjectRoutingError) *TroubleshootingReport
}

// TroubleshootingReport provides comprehensive troubleshooting information
type TroubleshootingReport struct {
	Summary         string                 `json:"summary"`
	ErrorAnalysis   *ErrorAnalysis         `json:"error_analysis"`
	SystemState     map[string]interface{} `json:"system_state"`
	Diagnostics     []DiagnosticResult     `json:"diagnostics"`
	Recommendations []TroubleshootingAction `json:"recommendations"`
	SimilarIssues   []SimilarIssue         `json:"similar_issues,omitempty"`
	GeneratedAt     time.Time              `json:"generated_at"`
}

// ErrorAnalysis provides detailed analysis of the error
type ErrorAnalysis struct {
	RootCause       string                 `json:"root_cause"`
	ContributingFactors []string           `json:"contributing_factors"`
	ImpactAssessment *ImpactAssessment     `json:"impact_assessment"`
	Patterns        []ErrorPattern         `json:"patterns,omitempty"`
}

// ImpactAssessment assesses the impact of the error
type ImpactAssessment struct {
	Severity        string   `json:"severity"`
	AffectedUsers   int      `json:"affected_users"`
	AffectedProjects []string `json:"affected_projects"`
	BusinessImpact  string   `json:"business_impact"`
	TechnicalImpact string   `json:"technical_impact"`
}

// ErrorPattern represents a pattern in error occurrence
type ErrorPattern struct {
	Pattern     string    `json:"pattern"`
	Frequency   int       `json:"frequency"`
	LastSeen    time.Time `json:"last_seen"`
	Description string    `json:"description"`
}

// TroubleshootingAction represents an action to resolve the issue
type TroubleshootingAction struct {
	Priority    int       `json:"priority"`
	Action      string    `json:"action"`
	Description string    `json:"description"`
	Command     string    `json:"command,omitempty"`
	Automated   bool      `json:"automated"`
	Impact      string    `json:"impact"`
	Risk        string    `json:"risk"`
}

// SimilarIssue represents a similar issue that has been seen before
type SimilarIssue struct {
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Resolution  string    `json:"resolution"`
	Confidence  float64   `json:"confidence"`
	LastSeen    time.Time `json:"last_seen"`
	Frequency   int       `json:"frequency"`
}

// =====================================================================================
// FEEDBACK COLLECTION AND LEARNING SYSTEM
// =====================================================================================

// FeedbackCollector collects user feedback to improve error handling
type FeedbackCollector interface {
	CollectErrorFeedback(errorID string, feedback *ErrorFeedback) error
	CollectRecoveryFeedback(recoveryID string, feedback *RecoveryFeedback) error
	GetFeedbackSummary(timeRange TimeRange) *FeedbackSummary
	AnalyzeFeedbackTrends() *FeedbackTrends
}

// ErrorFeedback represents user feedback on an error
type ErrorFeedback struct {
	ErrorID         string                 `json:"error_id"`
	UserID          string                 `json:"user_id,omitempty"`
	Helpful         bool                   `json:"helpful"`
	Satisfaction    int                    `json:"satisfaction"` // 1-10 scale
	Comments        string                 `json:"comments,omitempty"`
	SuggestedActions []string              `json:"suggested_actions,omitempty"`
	Context         map[string]interface{} `json:"context,omitempty"`
	Timestamp       time.Time              `json:"timestamp"`
}

// RecoveryFeedback represents user feedback on error recovery
type RecoveryFeedback struct {
	RecoveryID      string                 `json:"recovery_id"`
	UserID          string                 `json:"user_id,omitempty"`
	Successful      bool                   `json:"successful"`
	EasyToFollow    bool                   `json:"easy_to_follow"`
	TimeToRecover   time.Duration          `json:"time_to_recover"`
	MostHelpfulStep string                 `json:"most_helpful_step,omitempty"`
	Suggestions     string                 `json:"suggestions,omitempty"`
	Rating          int                    `json:"rating"` // 1-5 scale
	Context         map[string]interface{} `json:"context,omitempty"`
	Timestamp       time.Time              `json:"timestamp"`
}

// FeedbackSummary summarizes feedback over a time period
type FeedbackSummary struct {
	TimeRange       TimeRange                    `json:"time_range"`
	TotalFeedback   int                          `json:"total_feedback"`
	AverageRating   float64                      `json:"average_rating"`
	SatisfactionRate float64                     `json:"satisfaction_rate"`
	CommonComplaints []string                    `json:"common_complaints"`
	CommonPraises    []string                    `json:"common_praises"`
	ErrorsByType     map[string]FeedbackMetrics  `json:"errors_by_type"`
	RecoverysByType  map[string]FeedbackMetrics  `json:"recoveries_by_type"`
}

// FeedbackMetrics represents metrics for a specific category
type FeedbackMetrics struct {
	Count           int     `json:"count"`
	AverageRating   float64 `json:"average_rating"`
	SuccessRate     float64 `json:"success_rate"`
	SatisfactionRate float64 `json:"satisfaction_rate"`
}

// FeedbackTrends analyzes trends in user feedback
type FeedbackTrends struct {
	SatisfactionTrend string                      `json:"satisfaction_trend"` // "improving", "declining", "stable"
	ProblemAreas      []ProblemArea              `json:"problem_areas"`
	Improvements      []ImprovementArea          `json:"improvements"`
	Recommendations   []FeedbackRecommendation   `json:"recommendations"`
}

// ProblemArea represents an area with declining feedback
type ProblemArea struct {
	Area            string  `json:"area"`
	TrendDirection  string  `json:"trend_direction"`
	SeverityScore   float64 `json:"severity_score"`
	AffectedUsers   int     `json:"affected_users"`
	CommonIssues    []string `json:"common_issues"`
}

// ImprovementArea represents an area with improving feedback
type ImprovementArea struct {
	Area            string  `json:"area"`
	ImprovementRate float64 `json:"improvement_rate"`
	KeyFactors      []string `json:"key_factors"`
}

// FeedbackRecommendation represents a recommendation based on feedback analysis
type FeedbackRecommendation struct {
	Priority        int     `json:"priority"`
	Area            string  `json:"area"`
	Recommendation  string  `json:"recommendation"`
	ExpectedImpact  string  `json:"expected_impact"`
	EffortEstimate  string  `json:"effort_estimate"`
	Confidence      float64 `json:"confidence"`
}

// TimeRange represents a time range for analysis
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// =====================================================================================
// INTEGRATION INTERFACES
// =====================================================================================

// AlertNotifier sends alerts to various channels
type AlertNotifier interface {
	SendAlert(alert *Alert) error
	SendBulkAlerts(alerts []*Alert) error
	GetDeliveryStatus(alertID string) (*DeliveryStatus, error)
}

// MetricsStorage stores metrics data for analysis
type MetricsStorage interface {
	StoreMetrics(metrics map[string]interface{}) error
	QueryMetrics(query *MetricsQuery) (*MetricsResult, error)
	GetMetricsHistory(metric string, timeRange TimeRange) ([]TimestampedValue, error)
	DeleteOldMetrics(cutoff time.Time) error
}

// DeliveryStatus represents the status of alert delivery
type DeliveryStatus struct {
	AlertID     string                 `json:"alert_id"`
	Status      string                 `json:"status"` // "pending", "delivered", "failed"
	Attempts    int                    `json:"attempts"`
	LastAttempt time.Time              `json:"last_attempt"`
	Error       string                 `json:"error,omitempty"`
	Recipients  []RecipientStatus      `json:"recipients"`
}

// RecipientStatus represents the delivery status for a specific recipient
type RecipientStatus struct {
	Recipient string    `json:"recipient"`
	Status    string    `json:"status"`
	DeliveredAt *time.Time `json:"delivered_at,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// MetricsQuery represents a query for metrics data
type MetricsQuery struct {
	Metrics   []string               `json:"metrics"`
	TimeRange TimeRange              `json:"time_range"`
	Filters   map[string]interface{} `json:"filters,omitempty"`
	GroupBy   []string               `json:"group_by,omitempty"`
	Aggregate string                 `json:"aggregate,omitempty"`
}

// MetricsResult represents the result of a metrics query
type MetricsResult struct {
	Data      []map[string]interface{} `json:"data"`
	Metadata  map[string]interface{}   `json:"metadata"`
	QueryTime time.Duration            `json:"query_time"`
}

// =====================================================================================
// USAGE EXAMPLES AND IMPLEMENTATION GUIDELINES
// =====================================================================================

/*
COMPREHENSIVE MONITORING SETUP EXAMPLE:

```go
// Initialize monitoring system
monitoringConfig := &MonitoringConfig{
    EnableMonitoring:         true,
    EnableAlerting:          true,
    MetricsRetention:        7 * 24 * time.Hour, // 7 days
    AlertEvaluationInterval: 1 * time.Minute,
    EnableDetailedMetrics:   true,
    EnableUserMetrics:       true,
}

monitoring := NewMonitoringSystem(monitoringConfig, logger)

// Set up alert rules
alertRules := DefaultAlertRules()
monitoring.SetAlertRules(alertRules)

// Initialize user experience manager
uxConfig := &UserExperienceConfig{
    EnableProgressReporting:   true,
    EnableDiagnostics:        true,
    EnableRecoveryGuidance:   true,
    ShowDetailedErrors:       false, // Hide technical details from regular users
    EnableFeedbackCollection: true,
    ProgressUpdateInterval:   5 * time.Second,
}

uxManager := NewUserExperienceManager(uxConfig, logger)

// Example of handling an error with comprehensive UX
err := &SubProjectRoutingError{
    Type:         ErrorTypeClientConnection,
    Severity:     SeverityError,
    RecoveryType: RecoveryTypeFallback,
    Message:      "Language server connection failed",
    Context: &ErrorContext{
        Operation:     "textDocument/definition",
        FileURI:       "file:///project/src/main.go",
        LSPMethod:     "textDocument/definition",
        ProjectRoot:   "/project",
        WorkspaceRoot: "/workspace",
    },
    RequestID: "req-123",
    Timestamp: time.Now(),
}

// Format error for user
userError := uxManager.FormatError(err)

// Start progress tracking for recovery
progressID := "recovery-" + err.RequestID
stages := []string{"Diagnosing", "Attempting Fallback", "Validating"}
uxManager.StartProgress(progressID, stages)

// Update progress as recovery proceeds
uxManager.UpdateProgress(progressID, "Diagnosing", 30, "Running diagnostics...")
uxManager.UpdateProgress(progressID, "Attempting Fallback", 60, "Using workspace fallback...")
uxManager.UpdateProgress(progressID, "Validating", 90, "Validating recovery...")
uxManager.CompleteProgress(progressID, true)

// Collect user feedback
feedback := &ErrorFeedback{
    ErrorID:      err.RequestID,
    Helpful:      true,
    Satisfaction: 8,
    Comments:     "The fallback worked well, but it took a while",
    Timestamp:    time.Now(),
}
uxManager.CollectErrorFeedback(err.RequestID, feedback)
```

USER EXPERIENCE BEST PRACTICES:

1. **Clear Communication**:
   - Use plain language, avoid technical jargon
   - Provide specific, actionable guidance
   - Show progress for long-running operations
   - Give realistic time estimates

2. **Progressive Disclosure**:
   - Show essential information first
   - Provide technical details on demand
   - Offer different levels of guidance for different user types
   - Allow users to dive deeper when needed

3. **Proactive Support**:
   - Detect common patterns automatically
   - Offer solutions before users ask
   - Learn from user feedback
   - Continuously improve error messages

4. **Recovery-Focused Design**:
   - Always provide recovery options
   - Make recovery actions obvious
   - Automate recovery when safe
   - Validate recovery success

MONITORING DASHBOARD DESIGN:

1. **System Health Overview**:
   - Current error rates (last 1h, 24h, 7d)
   - Active alerts and their severity
   - Circuit breaker states
   - Performance metrics (P95, P99 response times)

2. **Error Analysis**:
   - Error breakdown by type and frequency
   - Top error sources (projects, files, operations)
   - Error trend over time
   - Recovery success rates

3. **User Experience Metrics**:
   - User satisfaction scores
   - Feedback sentiment analysis
   - Most common user complaints
   - Recovery effectiveness

4. **System Performance**:
   - Resource utilization (CPU, memory, disk)
   - Fallback strategy effectiveness
   - Cache hit rates
   - Request throughput

ALERT ESCALATION STRATEGY:

1. **Level 1 - Information** (Log only):
   - Minor performance degradation
   - Single error occurrences
   - Successful recovery events

2. **Level 2 - Warning** (Notify team):
   - Elevated error rates
   - Circuit breaker activations
   - Performance degradation

3. **Level 3 - Error** (Page on-call):
   - High error rates
   - Multiple fallback failures
   - Resource exhaustion

4. **Level 4 - Critical** (Immediate response):
   - System-wide failures
   - Data corruption
   - Security incidents

CONTINUOUS IMPROVEMENT PROCESS:

1. **Weekly Reviews**:
   - Analyze error patterns and trends
   - Review user feedback
   - Identify improvement opportunities
   - Update alert thresholds

2. **Monthly Analysis**:
   - Deep dive into problematic areas
   - Review recovery strategy effectiveness
   - Update error messages and guidance
   - Plan system improvements

3. **Quarterly Planning**:
   - Strategic improvements based on data
   - User experience enhancements
   - New monitoring capabilities
   - System architecture improvements
*/