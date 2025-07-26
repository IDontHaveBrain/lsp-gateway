package gateway

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"lsp-gateway/mcp"
)

// RoutingDecisionTracker tracks comprehensive routing decisions and outcomes
type RoutingDecisionTracker struct {
	decisions       []TrackedDecision
	decisionHistory map[string]*DecisionHistory
	mutex           sync.RWMutex
	maxHistory      int
	logger          *mcp.StructuredLogger
}

// TrackedDecision represents a complete routing decision with outcome tracking
type TrackedDecision struct {
	ID                string                  `json:"id"`
	Timestamp         time.Time               `json:"timestamp"`
	Method            string                  `json:"method"`
	Strategy          SCIPRoutingStrategyType `json:"strategy"`
	Source            RoutingSource           `json:"source"`
	Confidence        ConfidenceLevel         `json:"confidence"`
	ExpectedLatency   time.Duration           `json:"expected_latency"`
	
	// Decision context
	QueryAnalysis     *QueryAnalysis          `json:"query_analysis"`
	DecisionReason    string                  `json:"decision_reason"`
	SCIPQueried       bool                    `json:"scip_queried"`
	UsedFallback      bool                    `json:"used_fallback"`
	
	// Outcome tracking
	ActualLatency     time.Duration           `json:"actual_latency"`
	Success           bool                    `json:"success"`
	Error             string                  `json:"error,omitempty"`
	AccuracyScore     float64                 `json:"accuracy_score"`
	CacheHit          bool                    `json:"cache_hit"`
	DataSize          int64                   `json:"data_size"`
	
	// Performance metrics
	LatencyDeviation  float64                 `json:"latency_deviation"`
	ConfidenceAccuracy float64                `json:"confidence_accuracy"`
	RoutingEffectiveness float64             `json:"routing_effectiveness"`
	
	// Metadata
	FileLanguage      string                  `json:"file_language,omitempty"`
	ProjectContext    string                  `json:"project_context,omitempty"`
	ComplexityLevel   QueryComplexity         `json:"complexity_level"`
}

// DecisionHistory tracks historical decision patterns for specific method-context combinations
type DecisionHistory struct {
	Method           string                    `json:"method"`
	Context          string                    `json:"context"`
	Decisions        []TrackedDecision         `json:"decisions"`
	TotalDecisions   int64                     `json:"total_decisions"`
	SuccessRate      float64                   `json:"success_rate"`
	AverageLatency   time.Duration             `json:"average_latency"`
	OptimalStrategy  SCIPRoutingStrategyType   `json:"optimal_strategy"`
	LastAnalyzed     time.Time                 `json:"last_analyzed"`
	PerformanceTrend TrendDirection            `json:"performance_trend"`
	
	// Advanced metrics
	LatencyP50       time.Duration             `json:"latency_p50"`
	LatencyP95       time.Duration             `json:"latency_p95"`
	LatencyP99       time.Duration             `json:"latency_p99"`
	ErrorRate        float64                   `json:"error_rate"`
	FallbackRate     float64                   `json:"fallback_rate"`
}

// TrendDirection indicates performance trend direction
type TrendDirection int

const (
	TrendUnknown TrendDirection = iota
	TrendImproving
	TrendDegrading
	TrendStable
)

func (td TrendDirection) String() string {
	switch td {
	case TrendImproving:
		return "Improving"
	case TrendDegrading:
		return "Degrading"
	case TrendStable:
		return "Stable"
	default:
		return "Unknown"
	}
}

// AdaptiveRoutingOptimizer provides sophisticated routing optimization algorithms
type AdaptiveRoutingOptimizer struct {
	decisionTracker     *RoutingDecisionTracker
	metricsCollector    *PerformanceMetricsCollector
	thresholdAdjuster   *ThresholdAdjuster
	strategyEngine      *StrategyRecommendationEngine
	cacheWarmer         *CacheWarmer
	
	config              *AdaptiveOptimizationConfig
	optimizationHistory []OptimizationEvent
	learningModel       *RoutingLearningModel
	
	isOptimizing        atomic.Bool
	lastOptimization    time.Time
	mutex               sync.RWMutex
	ctx                 context.Context
	cancelFunc          context.CancelFunc
	logger              *mcp.StructuredLogger
}

// AdaptiveOptimizationConfig configures adaptive optimization behavior
type AdaptiveOptimizationConfig struct {
	OptimizationInterval    time.Duration `json:"optimization_interval"`
	MinDataPoints          int           `json:"min_data_points"`
	ConfidenceThreshold    float64       `json:"confidence_threshold"`
	PerformanceThreshold   float64       `json:"performance_threshold"`
	
	// Learning configuration
	LearningRate           float64       `json:"learning_rate"`
	AdaptationRate         float64       `json:"adaptation_rate"`
	ExplorationRate        float64       `json:"exploration_rate"`
	
	// Threshold adjustment
	MaxThresholdAdjustment float64       `json:"max_threshold_adjustment"`
	ThresholdDecayRate     float64       `json:"threshold_decay_rate"`
	
	// Strategy evaluation
	StrategyEvaluationWindow int         `json:"strategy_evaluation_window"`
	MinStrategyConfidence    float64     `json:"min_strategy_confidence"`
	
	// Cache warming
	CacheWarmingEnabled    bool          `json:"cache_warming_enabled"`
	WarmingTriggerThreshold float64      `json:"warming_trigger_threshold"`
	MaxWarmingOperations   int           `json:"max_warming_operations"`
}

// OptimizationEvent represents a single optimization event
type OptimizationEvent struct {
	Timestamp         time.Time                 `json:"timestamp"`
	TriggerReason     string                    `json:"trigger_reason"`
	Changes           []OptimizationChange      `json:"changes"`
	PerformanceImpact float64                   `json:"performance_impact"`
	Success           bool                      `json:"success"`
	Duration          time.Duration             `json:"duration"`
}

// OptimizationChange represents a specific optimization change
type OptimizationChange struct {
	Type          ChangeType                `json:"type"`
	Target        string                    `json:"target"`
	OldValue      interface{}               `json:"old_value"`
	NewValue      interface{}               `json:"new_value"`
	Confidence    float64                   `json:"confidence"`
	ExpectedGain  float64                   `json:"expected_gain"`
}

// ChangeType defines types of optimization changes
type ChangeType int

const (
	ChangeStrategyUpdate ChangeType = iota + 1
	ChangeThresholdAdjustment
	ChangeConfidenceScoring
	ChangeCacheWarming
	ChangeMethodMapping
)

func (ct ChangeType) String() string {
	switch ct {
	case ChangeStrategyUpdate:
		return "StrategyUpdate"
	case ChangeThresholdAdjustment:
		return "ThresholdAdjustment"
	case ChangeConfidenceScoring:
		return "ConfidenceScoring"
	case ChangeCacheWarming:
		return "CacheWarming"
	case ChangeMethodMapping:
		return "MethodMapping"
	default:
		return "Unknown"
	}
}

// PerformanceMetricsCollector provides real-time performance metrics aggregation
type PerformanceMetricsCollector struct {
	realTimeMetrics     *RealTimeMetrics
	aggregatedMetrics   map[string]*AggregatedMetrics
	metricHistory       []MetricSnapshot
	
	config              *MetricsConfig
	collectors          map[MetricType]*MetricCollector
	alerts              []PerformanceAlert
	
	updateInterval      time.Duration
	isCollecting        atomic.Bool
	mutex               sync.RWMutex
	ctx                 context.Context
	cancelFunc          context.CancelFunc
	logger              *mcp.StructuredLogger
}

// RealTimeMetrics tracks real-time performance metrics
type RealTimeMetrics struct {
	Timestamp           time.Time                         `json:"timestamp"`
	RequestsPerSecond   float64                           `json:"requests_per_second"`
	AverageLatency      time.Duration                     `json:"average_latency"`
	ErrorRate           float64                           `json:"error_rate"`
	CacheHitRate        float64                           `json:"cache_hit_rate"`
	SCIPSuccessRate     float64                           `json:"scip_success_rate"`
	
	// Source-specific metrics
	SCIPLatency         time.Duration                     `json:"scip_latency"`
	LSPLatency          time.Duration                     `json:"lsp_latency"`
	HybridLatency       time.Duration                     `json:"hybrid_latency"`
	
	// Method-specific metrics
	MethodMetrics       map[string]*MethodMetrics         `json:"method_metrics"`
	
	// Strategy effectiveness
	StrategyEffectiveness map[SCIPRoutingStrategyType]float64 `json:"strategy_effectiveness"`
	
	// Quality metrics
	AccuracyScore       float64                           `json:"accuracy_score"`
	DecisionQuality     float64                           `json:"decision_quality"`
	OptimizationGain    float64                           `json:"optimization_gain"`
}

// MethodMetrics tracks metrics for specific LSP methods
type MethodMetrics struct {
	RequestCount        int64         `json:"request_count"`
	SuccessRate         float64       `json:"success_rate"`
	AverageLatency      time.Duration `json:"average_latency"`
	PreferredSource     RoutingSource `json:"preferred_source"`
	ConfidenceScore     float64       `json:"confidence_score"`
	ErrorRate           float64       `json:"error_rate"`
	CacheHitRate        float64       `json:"cache_hit_rate"`
	LastUpdated         time.Time     `json:"last_updated"`
}

// AggregatedMetrics provides aggregated performance metrics over time windows
type AggregatedMetrics struct {
	WindowStart         time.Time                         `json:"window_start"`
	WindowEnd           time.Time                         `json:"window_end"`
	WindowDuration      time.Duration                     `json:"window_duration"`
	
	TotalRequests       int64                             `json:"total_requests"`
	SuccessfulRequests  int64                             `json:"successful_requests"`
	FailedRequests      int64                             `json:"failed_requests"`
	
	// Latency statistics
	LatencyStats        *LatencyStatistics                `json:"latency_stats"`
	
	// Source distribution
	SourceDistribution  map[RoutingSource]int64           `json:"source_distribution"`
	
	// Strategy performance
	StrategyPerformance map[SCIPRoutingStrategyType]*StrategyPerformanceMetrics `json:"strategy_performance"`
	
	// Quality metrics
	OverallAccuracy     float64                           `json:"overall_accuracy"`
	DecisionQuality     float64                           `json:"decision_quality"`
	OptimizationImpact  float64                           `json:"optimization_impact"`
}

// LatencyStatistics provides comprehensive latency statistics
type LatencyStatistics struct {
	Mean        time.Duration `json:"mean"`
	Median      time.Duration `json:"median"`
	P90         time.Duration `json:"p90"`
	P95         time.Duration `json:"p95"`
	P99         time.Duration `json:"p99"`
	Min         time.Duration `json:"min"`
	Max         time.Duration `json:"max"`
	StdDev      time.Duration `json:"std_dev"`
}

// StrategyPerformanceMetrics tracks performance for specific strategies
type StrategyPerformanceMetrics struct {
	Usage               int64         `json:"usage"`
	SuccessRate         float64       `json:"success_rate"`
	AverageLatency      time.Duration `json:"average_latency"`
	LatencyImprovement  float64       `json:"latency_improvement"`
	AccuracyScore       float64       `json:"accuracy_score"`
	CacheEffectiveness  float64       `json:"cache_effectiveness"`
	Cost                float64       `json:"cost"`
	Benefit             float64       `json:"benefit"`
}

// MetricSnapshot captures metrics at a specific point in time
type MetricSnapshot struct {
	Timestamp       time.Time                `json:"timestamp"`
	RealTimeMetrics *RealTimeMetrics         `json:"real_time_metrics"`
	SystemHealth    *SystemHealthMetrics     `json:"system_health"`
	TrendIndicators *TrendIndicators         `json:"trend_indicators"`
}

// SystemHealthMetrics tracks overall system health
type SystemHealthMetrics struct {
	MemoryUsage     float64   `json:"memory_usage"`
	CPUUsage        float64   `json:"cpu_usage"`
	GoroutineCount  int       `json:"goroutine_count"`
	HeapSize        int64     `json:"heap_size"`
	GCFrequency     float64   `json:"gc_frequency"`
	Uptime          time.Duration `json:"uptime"`
}

// TrendIndicators provide trend analysis
type TrendIndicators struct {
	LatencyTrend        TrendDirection `json:"latency_trend"`
	ErrorRateTrend      TrendDirection `json:"error_rate_trend"`
	CacheHitTrend       TrendDirection `json:"cache_hit_trend"`
	ThroughputTrend     TrendDirection `json:"throughput_trend"`
	AccuracyTrend       TrendDirection `json:"accuracy_trend"`
}

// MetricsConfig configures metrics collection
type MetricsConfig struct {
	CollectionInterval    time.Duration `json:"collection_interval"`
	AggregationWindow     time.Duration `json:"aggregation_window"`
	HistoryRetention      time.Duration `json:"history_retention"`
	
	EnableRealTimeMetrics bool          `json:"enable_real_time_metrics"`
	EnableAggregation     bool          `json:"enable_aggregation"`
	EnableAlerting        bool          `json:"enable_alerting"`
	
	// Alert thresholds
	LatencyThreshold      time.Duration `json:"latency_threshold"`
	ErrorRateThreshold    float64       `json:"error_rate_threshold"`
	CacheHitThreshold     float64       `json:"cache_hit_threshold"`
}

// MetricType defines types of metrics to collect
type MetricType int

const (
	MetricLatency MetricType = iota + 1
	MetricErrorRate
	MetricCacheHit
	MetricThroughput
	MetricAccuracy
	MetricDecisionQuality
)

// MetricCollector collects specific types of metrics
type MetricCollector struct {
	metricType   MetricType
	samples      []float64
	timestamps   []time.Time
	windowSize   int
	mutex        sync.RWMutex
}

// PerformanceAlert represents a performance-related alert
type PerformanceAlert struct {
	ID          string                 `json:"id"`
	Timestamp   time.Time              `json:"timestamp"`
	Severity    AlertSeverity          `json:"severity"`
	Type        AlertType              `json:"type"`
	Message     string                 `json:"message"`
	Metric      string                 `json:"metric"`
	Threshold   float64                `json:"threshold"`
	ActualValue float64                `json:"actual_value"`
	Context     map[string]interface{} `json:"context"`
	Resolved    bool                   `json:"resolved"`
	ResolvedAt  *time.Time             `json:"resolved_at,omitempty"`
}

// AlertSeverity defines alert severity levels
type AlertSeverity int

const (
	SeverityInfo AlertSeverity = iota + 1
	SeverityWarning
	SeverityError
	SeverityCritical
)

func (as AlertSeverity) String() string {
	switch as {
	case SeverityInfo:
		return "Info"
	case SeverityWarning:
		return "Warning"
	case SeverityError:
		return "Error"
	case SeverityCritical:
		return "Critical"
	default:
		return "Unknown"
	}
}

// AlertType defines types of alerts
type AlertType int

const (
	AlertLatencyHigh AlertType = iota + 1
	AlertErrorRateHigh
	AlertCacheHitLow
	AlertThroughputLow
	AlertAccuracyLow
	AlertDecisionQualityLow
)

func (at AlertType) String() string {
	switch at {
	case AlertLatencyHigh:
		return "LatencyHigh"
	case AlertErrorRateHigh:
		return "ErrorRateHigh"
	case AlertCacheHitLow:
		return "CacheHitLow"
	case AlertThroughputLow:
		return "ThroughputLow"
	case AlertAccuracyLow:
		return "AccuracyLow"
	case AlertDecisionQualityLow:
		return "DecisionQualityLow"
	default:
		return "Unknown"
	}
}

// ThresholdAdjuster dynamically adjusts routing thresholds based on performance
type ThresholdAdjuster struct {
	currentThresholds   map[string]ConfidenceLevel
	adjustmentHistory   []ThresholdAdjustment
	performanceImpact   map[string]float64
	
	config              *ThresholdConfig
	learningAlgorithm   *ThresholdLearningAlgorithm
	
	lastAdjustment      time.Time
	mutex               sync.RWMutex
	logger              *mcp.StructuredLogger
}

// ThresholdAdjustment represents a threshold adjustment event
type ThresholdAdjustment struct {
	Timestamp       time.Time          `json:"timestamp"`
	Method          string             `json:"method"`
	OldThreshold    ConfidenceLevel    `json:"old_threshold"`
	NewThreshold    ConfidenceLevel    `json:"new_threshold"`
	Reason          string             `json:"reason"`
	ExpectedImpact  float64            `json:"expected_impact"`
	ActualImpact    float64            `json:"actual_impact"`
	Success         bool               `json:"success"`
}

// ThresholdConfig configures threshold adjustment behavior
type ThresholdConfig struct {
	MinThreshold           ConfidenceLevel `json:"min_threshold"`
	MaxThreshold           ConfidenceLevel `json:"max_threshold"`
	AdjustmentStep         float64         `json:"adjustment_step"`
	AdjustmentInterval     time.Duration   `json:"adjustment_interval"`
	
	PerformanceWindow      int             `json:"performance_window"`
	MinDataPoints          int             `json:"min_data_points"`
	ConfidenceInterval     float64         `json:"confidence_interval"`
	
	// Learning parameters
	LearningRate           float64         `json:"learning_rate"`
	ExplorationRate        float64         `json:"exploration_rate"`
	ConvergenceThreshold   float64         `json:"convergence_threshold"`
}

// ThresholdLearningAlgorithm implements threshold learning algorithms
type ThresholdLearningAlgorithm struct {
	algorithm      LearningAlgorithmType
	parameters     map[string]float64
	history        []LearningDataPoint
	model          interface{}
	trained        bool
	lastTrained    time.Time
}

// LearningAlgorithmType defines types of learning algorithms
type LearningAlgorithmType int

const (
	AlgorithmGradientDescent LearningAlgorithmType = iota + 1
	AlgorithmBayesianOptimization
	AlgorithmReinforcementLearning
	AlgorithmEvolutionary
)

// LearningDataPoint represents a data point for learning
type LearningDataPoint struct {
	Input      []float64 `json:"input"`
	Output     float64   `json:"output"`
	Timestamp  time.Time `json:"timestamp"`
	Context    string    `json:"context"`
}

// StrategyRecommendationEngine recommends optimal routing strategies
type StrategyRecommendationEngine struct {
	recommendations     map[string]*StrategyRecommendation
	evaluationHistory   []StrategyEvaluation
	performanceMatrix   *StrategyPerformanceMatrix
	
	config              *RecommendationConfig
	evaluationAlgorithm *StrategyEvaluationAlgorithm
	
	lastEvaluation      time.Time
	mutex               sync.RWMutex
	logger              *mcp.StructuredLogger
}

// StrategyRecommendation represents a strategy recommendation
type StrategyRecommendation struct {
	Method              string                    `json:"method"`
	RecommendedStrategy SCIPRoutingStrategyType   `json:"recommended_strategy"`
	Confidence          float64                   `json:"confidence"`
	Reasoning           string                    `json:"reasoning"`
	ExpectedGain        float64                   `json:"expected_gain"`
	RiskAssessment      float64                   `json:"risk_assessment"`
	
	Alternatives        []AlternativeStrategy     `json:"alternatives"`
	Context             map[string]interface{}    `json:"context"`
	
	CreatedAt           time.Time                 `json:"created_at"`
	ValidUntil          time.Time                 `json:"valid_until"`
	Applied             bool                      `json:"applied"`
	AppliedAt           *time.Time                `json:"applied_at,omitempty"`
}

// AlternativeStrategy represents an alternative strategy option
type AlternativeStrategy struct {
	Strategy       SCIPRoutingStrategyType `json:"strategy"`
	Confidence     float64                 `json:"confidence"`
	ExpectedGain   float64                 `json:"expected_gain"`
	Risk           float64                 `json:"risk"`
	TradeOffs      []string                `json:"trade_offs"`
}

// StrategyEvaluation represents a strategy evaluation event
type StrategyEvaluation struct {
	Timestamp      time.Time                         `json:"timestamp"`
	Method         string                            `json:"method"`
	Strategies     []EvaluatedStrategy               `json:"strategies"`
	Winner         SCIPRoutingStrategyType           `json:"winner"`
	Confidence     float64                           `json:"confidence"`
	Criteria       map[string]float64                `json:"criteria"`
	Duration       time.Duration                     `json:"duration"`
}

// EvaluatedStrategy represents a strategy being evaluated
type EvaluatedStrategy struct {
	Strategy       SCIPRoutingStrategyType `json:"strategy"`
	Score          float64                 `json:"score"`
	Latency        time.Duration           `json:"latency"`
	Accuracy       float64                 `json:"accuracy"`
	CacheHitRate   float64                 `json:"cache_hit_rate"`
	ErrorRate      float64                 `json:"error_rate"`
	Cost           float64                 `json:"cost"`
}

// StrategyPerformanceMatrix tracks performance across strategies and methods
type StrategyPerformanceMatrix struct {
	Matrix         map[string]map[SCIPRoutingStrategyType]*PerformanceCell `json:"matrix"`
	LastUpdated    time.Time                                               `json:"last_updated"`
	UpdateCount    int64                                                   `json:"update_count"`
}

// PerformanceCell represents performance data for a method-strategy combination
type PerformanceCell struct {
	SampleCount    int64         `json:"sample_count"`
	SuccessRate    float64       `json:"success_rate"`
	AverageLatency time.Duration `json:"average_latency"`
	AccuracyScore  float64       `json:"accuracy_score"`
	CacheHitRate   float64       `json:"cache_hit_rate"`
	ErrorRate      float64       `json:"error_rate"`
	LastUpdated    time.Time     `json:"last_updated"`
	Confidence     float64       `json:"confidence"`
}

// RecommendationConfig configures strategy recommendation behavior
type RecommendationConfig struct {
	EvaluationInterval     time.Duration `json:"evaluation_interval"`
	MinDataPoints          int           `json:"min_data_points"`
	ConfidenceThreshold    float64       `json:"confidence_threshold"`
	
	// Evaluation weights
	LatencyWeight          float64       `json:"latency_weight"`
	AccuracyWeight         float64       `json:"accuracy_weight"`
	CacheHitWeight         float64       `json:"cache_hit_weight"`
	ErrorRateWeight        float64       `json:"error_rate_weight"`
	
	// Risk assessment
	RiskTolerance          float64       `json:"risk_tolerance"`
	MaxRiskThreshold       float64       `json:"max_risk_threshold"`
	
	// Recommendation validity
	RecommendationTTL      time.Duration `json:"recommendation_ttl"`
	MaxAlternatives        int           `json:"max_alternatives"`
}

// StrategyEvaluationAlgorithm implements strategy evaluation algorithms
type StrategyEvaluationAlgorithm struct {
	algorithm      StrategyAlgorithmType
	criteria       map[string]float64
	weights        map[string]float64
	model          interface{}
}

// StrategyAlgorithmType defines types of strategy evaluation algorithms
type StrategyAlgorithmType int

const (
	AlgorithmMultiCriteria StrategyAlgorithmType = iota + 1
	AlgorithmMachineLearning
	AlgorithmHeuristic
	AlgorithmGenetic
)

// CacheWarmer intelligently pre-warms SCIP cache based on usage patterns
type CacheWarmer struct {
	warmingQueue        []WarmingTask
	warmingHistory      []WarmingEvent
	usagePatterns       *UsagePatternAnalyzer
	
	config              *CacheWarmingConfig
	predictiveModel     *CachePredictiveModel
	
	isWarming           atomic.Bool
	lastWarming         time.Time
	mutex               sync.RWMutex
	ctx                 context.Context
	cancelFunc          context.CancelFunc
	logger              *mcp.StructuredLogger
}

// WarmingTask represents a cache warming task
type WarmingTask struct {
	ID                  string                    `json:"id"`
	Method              string                    `json:"method"`
	FileURI             string                    `json:"file_uri"`
	Language            string                    `json:"language"`
	Priority            WarmingPriority           `json:"priority"`
	ExpectedBenefit     float64                   `json:"expected_benefit"`
	EstimatedCost       time.Duration             `json:"estimated_cost"`
	
	Reasoning           string                    `json:"reasoning"`
	Context             map[string]interface{}    `json:"context"`
	
	CreatedAt           time.Time                 `json:"created_at"`
	ScheduledAt         time.Time                 `json:"scheduled_at"`
	StartedAt           *time.Time                `json:"started_at,omitempty"`
	CompletedAt         *time.Time                `json:"completed_at,omitempty"`
	Status              TaskStatus                `json:"status"`
}

// WarmingPriority defines cache warming task priorities
type WarmingPriority int

const (
	WarmingPriorityLow WarmingPriority = iota + 1
	WarmingPriorityMedium
	WarmingPriorityHigh
	WarmingPriorityCritical
)

func (wp WarmingPriority) String() string {
	switch wp {
	case WarmingPriorityLow:
		return "Low"
	case WarmingPriorityMedium:
		return "Medium"
	case WarmingPriorityHigh:
		return "High"
	case WarmingPriorityCritical:
		return "Critical"
	default:
		return "Unknown"
	}
}

// TaskStatus defines cache warming task status
type TaskStatus int

const (
	StatusPending TaskStatus = iota + 1
	StatusScheduled
	StatusRunning
	StatusCompleted
	StatusFailed
	StatusCancelled
)

func (ts TaskStatus) String() string {
	switch ts {
	case StatusPending:
		return "Pending"
	case StatusScheduled:
		return "Scheduled"
	case StatusRunning:
		return "Running"
	case StatusCompleted:
		return "Completed"
	case StatusFailed:
		return "Failed"
	case StatusCancelled:
		return "Cancelled"
	default:
		return "Unknown"
	}
}

// WarmingEvent represents a cache warming event
type WarmingEvent struct {
	ID              string                    `json:"id"`
	Timestamp       time.Time                 `json:"timestamp"`
	TriggerReason   string                    `json:"trigger_reason"`
	TasksCreated    int                       `json:"tasks_created"`
	TasksCompleted  int                       `json:"tasks_completed"`
	TasksFailed     int                       `json:"tasks_failed"`
	Duration        time.Duration             `json:"duration"`
	CacheHitGain    float64                   `json:"cache_hit_gain"`
	Success         bool                      `json:"success"`
}

// UsagePatternAnalyzer analyzes usage patterns for cache warming
type UsagePatternAnalyzer struct {
	patterns        map[string]*UsagePattern
	fileAccess      map[string]*FileAccessPattern
	methodUsage     map[string]*MethodUsagePattern
	timePatterns    *TimeBasedPattern
	
	config          *PatternAnalysisConfig
	lastAnalysis    time.Time
	mutex           sync.RWMutex
}

// UsagePattern represents a detected usage pattern
type UsagePattern struct {
	ID              string                    `json:"id"`
	Type            PatternType               `json:"type"`
	Confidence      float64                   `json:"confidence"`
	Frequency       float64                   `json:"frequency"`
	Recency         time.Duration             `json:"recency"`
	Context         map[string]interface{}    `json:"context"`
	PredictedNext   []PredictedAccess         `json:"predicted_next"`
	LastDetected    time.Time                 `json:"last_detected"`
}

// PatternType defines types of usage patterns
type PatternType int

const (
	PatternSequential PatternType = iota + 1
	PatternCyclical
	PatternBurst
	PatternRandom
	PatternTrending
)

// PredictedAccess represents a predicted file access
type PredictedAccess struct {
	FileURI         string        `json:"file_uri"`
	Method          string        `json:"method"`
	Probability     float64       `json:"probability"`
	ExpectedTime    time.Time     `json:"expected_time"`
	Confidence      float64       `json:"confidence"`
}

// FileAccessPattern tracks file access patterns
type FileAccessPattern struct {
	FileURI         string                    `json:"file_uri"`
	AccessCount     int64                     `json:"access_count"`
	LastAccess      time.Time                 `json:"last_access"`
	AccessTimes     []time.Time               `json:"access_times"`
	Methods         map[string]int64          `json:"methods"`
	Languages       []string                  `json:"languages"`
	Frequency       float64                   `json:"frequency"`
	Recency         time.Duration             `json:"recency"`
}

// MethodUsagePattern tracks method usage patterns
type MethodUsagePattern struct {
	Method          string                    `json:"method"`
	UsageCount      int64                     `json:"usage_count"`
	LastUsed        time.Time                 `json:"last_used"`
	UsageTimes      []time.Time               `json:"usage_times"`
	SuccessRate     float64                   `json:"success_rate"`
	AverageLatency  time.Duration             `json:"average_latency"`
	Files           map[string]int64          `json:"files"`
	Popularity      float64                   `json:"popularity"`
}

// TimeBasedPattern tracks time-based usage patterns
type TimeBasedPattern struct {
	HourlyUsage     [24]float64               `json:"hourly_usage"`
	DailyUsage      [7]float64                `json:"daily_usage"`
	MonthlyUsage    [12]float64               `json:"monthly_usage"`
	PeakHours       []int                     `json:"peak_hours"`
	PeakDays        []int                     `json:"peak_days"`
	LastUpdated     time.Time                 `json:"last_updated"`
}

// CachePredictiveModel implements predictive models for cache warming
type CachePredictiveModel struct {
	modelType       PredictiveModelType
	features        []string
	model           interface{}
	accuracy        float64
	lastTrained     time.Time
	trainingData    []TrainingDataPoint
	
	config          *PredictiveModelConfig
	mutex           sync.RWMutex
}

// PredictiveModelType defines types of predictive models
type PredictiveModelType int

const (
	ModelLinearRegression PredictiveModelType = iota + 1
	ModelNeuralNetwork
	ModelRandomForest
	ModelMarkov
	ModelTimeSeries
)

// TrainingDataPoint represents a training data point
type TrainingDataPoint struct {
	Features        []float64                 `json:"features"`
	Target          float64                   `json:"target"`
	Timestamp       time.Time                 `json:"timestamp"`
	Context         map[string]interface{}    `json:"context"`
}

// CacheWarmingConfig configures cache warming behavior
type CacheWarmingConfig struct {
	Enabled                 bool          `json:"enabled"`
	WarmingInterval         time.Duration `json:"warming_interval"`
	MaxConcurrentTasks      int           `json:"max_concurrent_tasks"`
	TaskTimeout             time.Duration `json:"task_timeout"`
	
	// Triggering conditions
	CacheHitThreshold       float64       `json:"cache_hit_threshold"`
	LatencyThreshold        time.Duration `json:"latency_threshold"`
	UsageThreshold          int64         `json:"usage_threshold"`
	
	// Pattern analysis
	PatternAnalysisEnabled  bool          `json:"pattern_analysis_enabled"`
	PatternMinConfidence    float64       `json:"pattern_min_confidence"`
	
	// Predictive warming
	PredictiveEnabled       bool          `json:"predictive_enabled"`
	PredictionHorizon       time.Duration `json:"prediction_horizon"`
	MinPredictionConfidence float64       `json:"min_prediction_confidence"`
	
	// Resource limits
	MaxMemoryUsage          int64         `json:"max_memory_usage"`
	MaxDiskUsage            int64         `json:"max_disk_usage"`
	MaxNetworkBandwidth     int64         `json:"max_network_bandwidth"`
}

// PatternAnalysisConfig configures pattern analysis
type PatternAnalysisConfig struct {
	AnalysisInterval        time.Duration `json:"analysis_interval"`
	MinDataPoints           int           `json:"min_data_points"`
	ConfidenceThreshold     float64       `json:"confidence_threshold"`
	
	// Pattern detection
	SequentialDetection     bool          `json:"sequential_detection"`
	CyclicalDetection       bool          `json:"cyclical_detection"`
	BurstDetection          bool          `json:"burst_detection"`
	TrendDetection          bool          `json:"trend_detection"`
	
	// Analysis windows
	ShortTermWindow         time.Duration `json:"short_term_window"`
	MediumTermWindow        time.Duration `json:"medium_term_window"`
	LongTermWindow          time.Duration `json:"long_term_window"`
}

// PredictiveModelConfig configures predictive models
type PredictiveModelConfig struct {
	ModelType               PredictiveModelType `json:"model_type"`
	TrainingInterval        time.Duration       `json:"training_interval"`
	MinTrainingData         int                 `json:"min_training_data"`
	MaxTrainingData         int                 `json:"max_training_data"`
	
	// Model parameters
	LearningRate            float64             `json:"learning_rate"`
	Epochs                  int                 `json:"epochs"`
	BatchSize               int                 `json:"batch_size"`
	ValidationSplit         float64             `json:"validation_split"`
	
	// Feature engineering
	FeatureWindow           time.Duration       `json:"feature_window"`
	FeatureEngineering      bool                `json:"feature_engineering"`
	AutoFeatureSelection    bool                `json:"auto_feature_selection"`
	
	// Model evaluation
	CrossValidation         bool                `json:"cross_validation"`
	MinAccuracy             float64             `json:"min_accuracy"`
	RetrainingThreshold     float64             `json:"retraining_threshold"`
}

// RoutingLearningModel implements learning models for routing optimization
type RoutingLearningModel struct {
	modelType       LearningModelType
	features        []string
	model           interface{}
	accuracy        float64
	
	trainingData    []RoutingTrainingDataPoint
	testData        []RoutingTrainingDataPoint
	
	config          *LearningModelConfig
	lastTrained     time.Time
	trained         bool
	mutex           sync.RWMutex
}

// LearningModelType defines types of learning models
type LearningModelType int

const (
	ModelQTable LearningModelType = iota + 1
	ModelDeepQ
	ModelPolicyGradient
	ModelActorCritic
	ModelEnsemble
)

// RoutingTrainingDataPoint represents routing training data
type RoutingTrainingDataPoint struct {
	State           []float64                 `json:"state"`
	Action          int                       `json:"action"`
	Reward          float64                   `json:"reward"`
	NextState       []float64                 `json:"next_state"`
	Done            bool                      `json:"done"`
	
	Method          string                    `json:"method"`
	Strategy        SCIPRoutingStrategyType   `json:"strategy"`
	Context         map[string]interface{}    `json:"context"`
	Timestamp       time.Time                 `json:"timestamp"`
}

// LearningModelConfig configures learning models
type LearningModelConfig struct {
	ModelType               LearningModelType   `json:"model_type"`
	StateSize               int                 `json:"state_size"`
	ActionSize              int                 `json:"action_size"`
	
	// Training parameters
	LearningRate            float64             `json:"learning_rate"`
	DiscountFactor          float64             `json:"discount_factor"`
	ExplorationRate         float64             `json:"exploration_rate"`
	ExplorationDecay        float64             `json:"exploration_decay"`
	MinExplorationRate      float64             `json:"min_exploration_rate"`
	
	// Network architecture
	HiddenLayers            []int               `json:"hidden_layers"`
	ActivationFunction      string              `json:"activation_function"`
	Optimizer               string              `json:"optimizer"`
	
	// Training configuration
	BatchSize               int                 `json:"batch_size"`
	MemorySize              int                 `json:"memory_size"`
	TargetUpdateFreq        int                 `json:"target_update_freq"`
	
	// Evaluation
	EvaluationInterval      time.Duration       `json:"evaluation_interval"`
	EvaluationEpisodes      int                 `json:"evaluation_episodes"`
}

// NewRoutingDecisionTracker creates a new routing decision tracker
func NewRoutingDecisionTracker(maxHistory int, logger *mcp.StructuredLogger) *RoutingDecisionTracker {
	return &RoutingDecisionTracker{
		decisions:       make([]TrackedDecision, 0, maxHistory),
		decisionHistory: make(map[string]*DecisionHistory),
		maxHistory:      maxHistory,
		logger:          logger,
	}
}

// TrackDecision tracks a routing decision and its outcome
func (rdt *RoutingDecisionTracker) TrackDecision(decision *TrackedDecision) {
	rdt.mutex.Lock()
	defer rdt.mutex.Unlock()
	
	// Add to decisions list
	if len(rdt.decisions) >= rdt.maxHistory {
		// Remove oldest decisions to maintain size limit
		copy(rdt.decisions, rdt.decisions[1:])
		rdt.decisions = rdt.decisions[:len(rdt.decisions)-1]
	}
	rdt.decisions = append(rdt.decisions, *decision)
	
	// Update decision history
	historyKey := fmt.Sprintf("%s:%s", decision.Method, decision.ProjectContext)
	if history, exists := rdt.decisionHistory[historyKey]; exists {
		history.Decisions = append(history.Decisions, *decision)
		history.TotalDecisions++
		rdt.updateDecisionHistory(history)
	} else {
		history := &DecisionHistory{
			Method:         decision.Method,
			Context:        decision.ProjectContext,
			Decisions:      []TrackedDecision{*decision},
			TotalDecisions: 1,
			LastAnalyzed:   time.Now(),
		}
		rdt.updateDecisionHistory(history)
		rdt.decisionHistory[historyKey] = history
	}
	
	if rdt.logger != nil {
		rdt.logger.Debugf("Tracked routing decision: %s for %s (confidence: %.2f, success: %v)", 
			decision.Strategy, decision.Method, decision.Confidence, decision.Success)
	}
}

// updateDecisionHistory updates decision history metrics
func (rdt *RoutingDecisionTracker) updateDecisionHistory(history *DecisionHistory) {
	if len(history.Decisions) == 0 {
		return
	}
	
	// Calculate success rate
	successCount := 0
	totalLatency := time.Duration(0)
	latencies := make([]time.Duration, 0, len(history.Decisions))
	errorCount := 0
	fallbackCount := 0
	
	for _, decision := range history.Decisions {
		if decision.Success {
			successCount++
		} else {
			errorCount++
		}
		
		if decision.UsedFallback {
			fallbackCount++
		}
		
		totalLatency += decision.ActualLatency
		latencies = append(latencies, decision.ActualLatency)
	}
	
	history.SuccessRate = float64(successCount) / float64(len(history.Decisions))
	history.AverageLatency = totalLatency / time.Duration(len(history.Decisions))
	history.ErrorRate = float64(errorCount) / float64(len(history.Decisions))
	history.FallbackRate = float64(fallbackCount) / float64(len(history.Decisions))
	
	// Calculate percentiles
	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})
	
	if len(latencies) > 0 {
		history.LatencyP50 = latencies[len(latencies)*50/100]
		history.LatencyP95 = latencies[len(latencies)*95/100]
		history.LatencyP99 = latencies[len(latencies)*99/100]
	}
	
	// Determine performance trend
	history.PerformanceTrend = rdt.calculatePerformanceTrend(history.Decisions)
	
	// Find optimal strategy
	history.OptimalStrategy = rdt.findOptimalStrategy(history.Decisions)
	
	history.LastAnalyzed = time.Now()
}

// calculatePerformanceTrend calculates performance trend for decisions
func (rdt *RoutingDecisionTracker) calculatePerformanceTrend(decisions []TrackedDecision) TrendDirection {
	if len(decisions) < 10 {
		return TrendUnknown
	}
	
	// Split decisions into two halves and compare performance
	mid := len(decisions) / 2
	firstHalf := decisions[:mid]
	secondHalf := decisions[mid:]
	
	firstHalfSuccess := 0
	secondHalfSuccess := 0
	
	for _, decision := range firstHalf {
		if decision.Success {
			firstHalfSuccess++
		}
	}
	
	for _, decision := range secondHalf {
		if decision.Success {
			secondHalfSuccess++
		}
	}
	
	firstHalfRate := float64(firstHalfSuccess) / float64(len(firstHalf))
	secondHalfRate := float64(secondHalfSuccess) / float64(len(secondHalf))
	
	diff := secondHalfRate - firstHalfRate
	
	if diff > 0.05 {
		return TrendImproving
	} else if diff < -0.05 {
		return TrendDegrading
	} else {
		return TrendStable
	}
}

// findOptimalStrategy finds the optimal strategy for given decisions
func (rdt *RoutingDecisionTracker) findOptimalStrategy(decisions []TrackedDecision) SCIPRoutingStrategyType {
	strategyPerformance := make(map[SCIPRoutingStrategyType]float64)
	strategyCounts := make(map[SCIPRoutingStrategyType]int)
	
	for _, decision := range decisions {
		score := 0.0
		if decision.Success {
			score += 1.0
		}
		
		// Normalize latency score (lower is better)
		if decision.ExpectedLatency > 0 {
			latencyScore := 1.0 - (float64(decision.ActualLatency)/float64(decision.ExpectedLatency))
			score += math.Max(0, latencyScore)
		}
		
		score += decision.AccuracyScore
		
		strategyPerformance[decision.Strategy] += score
		strategyCounts[decision.Strategy]++
	}
	
	var bestStrategy SCIPRoutingStrategyType
	bestScore := -1.0
	
	for strategy, totalScore := range strategyPerformance {
		avgScore := totalScore / float64(strategyCounts[strategy])
		if avgScore > bestScore {
			bestScore = avgScore
			bestStrategy = strategy
		}
	}
	
	return bestStrategy
}

// GetDecisionHistory returns decision history for a method
func (rdt *RoutingDecisionTracker) GetDecisionHistory(method, context string) *DecisionHistory {
	rdt.mutex.RLock()
	defer rdt.mutex.RUnlock()
	
	historyKey := fmt.Sprintf("%s:%s", method, context)
	if history, exists := rdt.decisionHistory[historyKey]; exists {
		return history
	}
	
	return nil
}

// GetRecentDecisions returns recent decisions within the specified time window
func (rdt *RoutingDecisionTracker) GetRecentDecisions(window time.Duration) []TrackedDecision {
	rdt.mutex.RLock()
	defer rdt.mutex.RUnlock()
	
	cutoff := time.Now().Add(-window)
	var recent []TrackedDecision
	
	for i := len(rdt.decisions) - 1; i >= 0; i-- {
		if rdt.decisions[i].Timestamp.Before(cutoff) {
			break
		}
		recent = append([]TrackedDecision{rdt.decisions[i]}, recent...)
	}
	
	return recent
}

// GetPerformanceMetrics returns performance metrics for all tracked decisions
func (rdt *RoutingDecisionTracker) GetPerformanceMetrics() map[string]interface{} {
	rdt.mutex.RLock()
	defer rdt.mutex.RUnlock()
	
	if len(rdt.decisions) == 0 {
		return make(map[string]interface{})
	}
	
	successCount := 0
	totalLatency := time.Duration(0)
	errorCount := 0
	fallbackCount := 0
	scipUsage := 0
	
	methodMetrics := make(map[string]*MethodMetrics)
	
	for _, decision := range rdt.decisions {
		if decision.Success {
			successCount++
		} else {
			errorCount++
		}
		
		if decision.UsedFallback {
			fallbackCount++
		}
		
		if decision.Source == SourceSCIPCache {
			scipUsage++
		}
		
		totalLatency += decision.ActualLatency
		
		// Update method metrics
		if methodMetrics[decision.Method] == nil {
			methodMetrics[decision.Method] = &MethodMetrics{
				LastUpdated: time.Now(),
			}
		}
		
		metrics := methodMetrics[decision.Method]
		metrics.RequestCount++
		if decision.Success {
			metrics.SuccessRate = (metrics.SuccessRate*float64(metrics.RequestCount-1) + 1.0) / float64(metrics.RequestCount)
		} else {
			metrics.SuccessRate = (metrics.SuccessRate*float64(metrics.RequestCount-1) + 0.0) / float64(metrics.RequestCount)
		}
		
		metrics.AverageLatency = time.Duration(
			(float64(metrics.AverageLatency)*float64(metrics.RequestCount-1) + 
			 float64(decision.ActualLatency)) / float64(metrics.RequestCount),
		)
		
		if decision.Source == SourceSCIPCache {
			metrics.PreferredSource = SourceSCIPCache
		} else if decision.Source == SourceLSPServer {
			metrics.PreferredSource = SourceLSPServer
		}
		
		metrics.ConfidenceScore = (metrics.ConfidenceScore*float64(metrics.RequestCount-1) + 
			float64(decision.Confidence)) / float64(metrics.RequestCount)
	}
	
	totalDecisions := len(rdt.decisions)
	
	return map[string]interface{}{
		"total_decisions":  totalDecisions,
		"success_rate":     float64(successCount) / float64(totalDecisions),
		"average_latency":  totalLatency / time.Duration(totalDecisions),
		"error_rate":       float64(errorCount) / float64(totalDecisions),
		"fallback_rate":    float64(fallbackCount) / float64(totalDecisions),
		"scip_usage_rate":  float64(scipUsage) / float64(totalDecisions),
		"method_metrics":   methodMetrics,
	}
}

// NewAdaptiveRoutingOptimizer creates a new adaptive routing optimizer
func NewAdaptiveRoutingOptimizer(
	config *AdaptiveOptimizationConfig,
	logger *mcp.StructuredLogger,
) *AdaptiveRoutingOptimizer {
	ctx, cancel := context.WithCancel(context.Background())
	
	optimizer := &AdaptiveRoutingOptimizer{
		decisionTracker:     NewRoutingDecisionTracker(1000, logger),
		metricsCollector:    NewPerformanceMetricsCollector(&MetricsConfig{
			CollectionInterval:    30 * time.Second,
			AggregationWindow:     5 * time.Minute,
			HistoryRetention:      24 * time.Hour,
			EnableRealTimeMetrics: true,
			EnableAggregation:     true,
			EnableAlerting:        true,
		}, logger),
		thresholdAdjuster:   NewThresholdAdjuster(&ThresholdConfig{
			MinThreshold:         ConfidenceMinimum,
			MaxThreshold:         ConfidenceHigh,
			AdjustmentStep:       0.05,
			AdjustmentInterval:   10 * time.Minute,
		}, logger),
		strategyEngine:      NewStrategyRecommendationEngine(&RecommendationConfig{
			EvaluationInterval:   5 * time.Minute,
			MinDataPoints:        10,
			ConfidenceThreshold:  0.8,
		}, logger),
		cacheWarmer:         NewCacheWarmer(&CacheWarmingConfig{
			Enabled:                 true,
			WarmingInterval:         15 * time.Minute,
			MaxConcurrentTasks:      5,
			CacheHitThreshold:       0.7,
			PatternAnalysisEnabled:  true,
			PredictiveEnabled:       true,
		}, logger),
		config:              config,
		optimizationHistory: make([]OptimizationEvent, 0),
		learningModel:       NewRoutingLearningModel(&LearningModelConfig{
			ModelType:            ModelQTable,
			StateSize:            10,
			ActionSize:           5,
			LearningRate:         0.1,
			DiscountFactor:       0.95,
			ExplorationRate:      0.1,
		}),
		lastOptimization:    time.Now(),
		ctx:                 ctx,
		cancelFunc:          cancel,
		logger:              logger,
	}
	
	if config == nil {
		optimizer.config = getDefaultAdaptiveOptimizationConfig()
	}
	
	// Start background optimization
	go optimizer.backgroundOptimization()
	
	return optimizer
}

// TrackRoutingDecision tracks a routing decision for optimization
func (aro *AdaptiveRoutingOptimizer) TrackRoutingDecision(decision *TrackedDecision) {
	aro.decisionTracker.TrackDecision(decision)
	
	// Update metrics collector
	if aro.metricsCollector != nil {
		aro.metricsCollector.RecordDecision(decision)
	}
	
	// Feed to learning model
	if aro.learningModel != nil {
		aro.learningModel.Learn(decision)
	}
}

// OptimizeRouting performs comprehensive routing optimization
func (aro *AdaptiveRoutingOptimizer) OptimizeRouting() (*OptimizationEvent, error) {
	if aro.isOptimizing.Load() {
		return nil, fmt.Errorf("optimization already in progress")
	}
	
	aro.isOptimizing.Store(true)
	defer aro.isOptimizing.Store(false)
	
	startTime := time.Now()
	event := &OptimizationEvent{
		Timestamp:     startTime,
		TriggerReason: "Scheduled optimization",
		Changes:       make([]OptimizationChange, 0),
	}
	
	// Collect current performance metrics
	metrics := aro.metricsCollector.GetRealTimeMetrics()
	
	// Threshold optimization
	thresholdChanges, err := aro.thresholdAdjuster.OptimizeThresholds(metrics)
	if err != nil && aro.logger != nil {
		aro.logger.Warnf("Threshold optimization failed: %v", err)
	} else {
		event.Changes = append(event.Changes, thresholdChanges...)
	}
	
	// Strategy optimization
	strategyChanges, err := aro.strategyEngine.OptimizeStrategies(metrics)
	if err != nil && aro.logger != nil {
		aro.logger.Warnf("Strategy optimization failed: %v", err)
	} else {
		event.Changes = append(event.Changes, strategyChanges...)
	}
	
	// Cache warming optimization
	if aro.cacheWarmer != nil {
		warmingChanges, err := aro.cacheWarmer.OptimizeWarming(metrics)
		if err != nil && aro.logger != nil {
			aro.logger.Warnf("Cache warming optimization failed: %v", err)
		} else {
			event.Changes = append(event.Changes, warmingChanges...)
		}
	}
	
	event.Duration = time.Since(startTime)
	event.Success = len(event.Changes) > 0
	
	// Calculate performance impact
	if len(event.Changes) > 0 {
		event.PerformanceImpact = aro.calculatePerformanceImpact(event.Changes)
	}
	
	// Store optimization event
	aro.mutex.Lock()
	aro.optimizationHistory = append(aro.optimizationHistory, *event)
	aro.lastOptimization = time.Now()
	aro.mutex.Unlock()
	
	if aro.logger != nil {
		aro.logger.Infof("Routing optimization completed: %d changes, %.2f%% impact, duration: %v", 
			len(event.Changes), event.PerformanceImpact*100, event.Duration)
	}
	
	return event, nil
}

// calculatePerformanceImpact calculates expected performance impact of changes
func (aro *AdaptiveRoutingOptimizer) calculatePerformanceImpact(changes []OptimizationChange) float64 {
	totalGain := 0.0
	totalConfidence := 0.0
	
	for _, change := range changes {
		weightedGain := change.ExpectedGain * change.Confidence
		totalGain += weightedGain
		totalConfidence += change.Confidence
	}
	
	if totalConfidence > 0 {
		return totalGain / totalConfidence
	}
	
	return 0.0
}

// backgroundOptimization runs continuous optimization in the background
func (aro *AdaptiveRoutingOptimizer) backgroundOptimization() {
	ticker := time.NewTicker(aro.config.OptimizationInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			if time.Since(aro.lastOptimization) >= aro.config.OptimizationInterval {
				_, err := aro.OptimizeRouting()
				if err != nil && aro.logger != nil {
					aro.logger.Errorf("Background optimization failed: %v", err)
				}
			}
		case <-aro.ctx.Done():
			return
		}
	}
}

// GetOptimizationHistory returns the optimization history
func (aro *AdaptiveRoutingOptimizer) GetOptimizationHistory() []OptimizationEvent {
	aro.mutex.RLock()
	defer aro.mutex.RUnlock()
	
	history := make([]OptimizationEvent, len(aro.optimizationHistory))
	copy(history, aro.optimizationHistory)
	return history
}

// GetPerformanceMetrics returns current performance metrics
func (aro *AdaptiveRoutingOptimizer) GetPerformanceMetrics() *RealTimeMetrics {
	if aro.metricsCollector != nil {
		return aro.metricsCollector.GetRealTimeMetrics()
	}
	return nil
}

// Close cleans up resources
func (aro *AdaptiveRoutingOptimizer) Close() error {
	if aro.cancelFunc != nil {
		aro.cancelFunc()
	}
	
	if aro.metricsCollector != nil {
		aro.metricsCollector.Close()
	}
	
	if aro.cacheWarmer != nil {
		aro.cacheWarmer.Close()
	}
	
	return nil
}

// Helper function to get default adaptive optimization config
func getDefaultAdaptiveOptimizationConfig() *AdaptiveOptimizationConfig {
	return &AdaptiveOptimizationConfig{
		OptimizationInterval:    5 * time.Minute,
		MinDataPoints:          10,
		ConfidenceThreshold:    0.8,
		PerformanceThreshold:   0.1,
		LearningRate:           0.1,
		AdaptationRate:         0.05,
		ExplorationRate:        0.1,
		MaxThresholdAdjustment: 0.2,
		ThresholdDecayRate:     0.95,
		StrategyEvaluationWindow: 100,
		MinStrategyConfidence:   0.7,
		CacheWarmingEnabled:     true,
		WarmingTriggerThreshold: 0.7,
		MaxWarmingOperations:    10,
	}
}

// Placeholder implementations for other components

// NewPerformanceMetricsCollector creates a new performance metrics collector
func NewPerformanceMetricsCollector(config *MetricsConfig, logger *mcp.StructuredLogger) *PerformanceMetricsCollector {
	ctx, cancel := context.WithCancel(context.Background())
	
	collector := &PerformanceMetricsCollector{
		realTimeMetrics:   &RealTimeMetrics{
			Timestamp:             time.Now(),
			MethodMetrics:         make(map[string]*MethodMetrics),
			StrategyEffectiveness: make(map[SCIPRoutingStrategyType]float64),
		},
		aggregatedMetrics: make(map[string]*AggregatedMetrics),
		metricHistory:     make([]MetricSnapshot, 0),
		config:            config,
		collectors:        make(map[MetricType]*MetricCollector),
		alerts:            make([]PerformanceAlert, 0),
		updateInterval:    config.CollectionInterval,
		ctx:               ctx,
		cancelFunc:        cancel,
		logger:            logger,
	}
	
	// Start background collection
	go collector.backgroundCollection()
	
	return collector
}

// RecordDecision records a routing decision for metrics
func (pmc *PerformanceMetricsCollector) RecordDecision(decision *TrackedDecision) {
	// Implementation would update real-time metrics based on the decision
}

// GetRealTimeMetrics returns current real-time metrics
func (pmc *PerformanceMetricsCollector) GetRealTimeMetrics() *RealTimeMetrics {
	pmc.mutex.RLock()
	defer pmc.mutex.RUnlock()
	return pmc.realTimeMetrics
}

// backgroundCollection runs background metrics collection
func (pmc *PerformanceMetricsCollector) backgroundCollection() {
	ticker := time.NewTicker(pmc.updateInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			pmc.collectMetrics()
		case <-pmc.ctx.Done():
			return
		}
	}
}

// collectMetrics collects current metrics
func (pmc *PerformanceMetricsCollector) collectMetrics() {
	// Implementation would collect and update metrics
}

// Close cleans up resources
func (pmc *PerformanceMetricsCollector) Close() error {
	if pmc.cancelFunc != nil {
		pmc.cancelFunc()
	}
	return nil
}

// NewThresholdAdjuster creates a new threshold adjuster
func NewThresholdAdjuster(config *ThresholdConfig, logger *mcp.StructuredLogger) *ThresholdAdjuster {
	return &ThresholdAdjuster{
		currentThresholds:  make(map[string]ConfidenceLevel),
		adjustmentHistory:  make([]ThresholdAdjustment, 0),
		performanceImpact:  make(map[string]float64),
		config:             config,
		learningAlgorithm:  &ThresholdLearningAlgorithm{
			algorithm:   AlgorithmGradientDescent,
			parameters:  make(map[string]float64),
			history:     make([]LearningDataPoint, 0),
		},
		lastAdjustment:     time.Now(),
		logger:             logger,
	}
}

// OptimizeThresholds optimizes routing thresholds based on metrics
func (ta *ThresholdAdjuster) OptimizeThresholds(metrics *RealTimeMetrics) ([]OptimizationChange, error) {
	changes := make([]OptimizationChange, 0)
	
	// Implementation would analyze metrics and adjust thresholds
	
	return changes, nil
}

// NewStrategyRecommendationEngine creates a new strategy recommendation engine
func NewStrategyRecommendationEngine(config *RecommendationConfig, logger *mcp.StructuredLogger) *StrategyRecommendationEngine {
	return &StrategyRecommendationEngine{
		recommendations:     make(map[string]*StrategyRecommendation),
		evaluationHistory:   make([]StrategyEvaluation, 0),
		performanceMatrix:   &StrategyPerformanceMatrix{
			Matrix:      make(map[string]map[SCIPRoutingStrategyType]*PerformanceCell),
			LastUpdated: time.Now(),
		},
		config:              config,
		evaluationAlgorithm: &StrategyEvaluationAlgorithm{
			algorithm: AlgorithmMultiCriteria,
			criteria:  make(map[string]float64),
			weights:   make(map[string]float64),
		},
		lastEvaluation:      time.Now(),
		logger:              logger,
	}
}

// OptimizeStrategies optimizes routing strategies based on metrics
func (sre *StrategyRecommendationEngine) OptimizeStrategies(metrics *RealTimeMetrics) ([]OptimizationChange, error) {
	changes := make([]OptimizationChange, 0)
	
	// Implementation would analyze metrics and recommend strategy changes
	
	return changes, nil
}

// NewCacheWarmer creates a new cache warmer
func NewCacheWarmer(config *CacheWarmingConfig, logger *mcp.StructuredLogger) *CacheWarmer {
	ctx, cancel := context.WithCancel(context.Background())
	
	warmer := &CacheWarmer{
		warmingQueue:   make([]WarmingTask, 0),
		warmingHistory: make([]WarmingEvent, 0),
		usagePatterns:  &UsagePatternAnalyzer{
			patterns:     make(map[string]*UsagePattern),
			fileAccess:   make(map[string]*FileAccessPattern),
			methodUsage:  make(map[string]*MethodUsagePattern),
			timePatterns: &TimeBasedPattern{},
		},
		config:         config,
		predictiveModel: &CachePredictiveModel{
			modelType:    ModelLinearRegression,
			features:     []string{"frequency", "recency", "popularity"},
			trainingData: make([]TrainingDataPoint, 0),
		},
		lastWarming:    time.Now(),
		ctx:            ctx,
		cancelFunc:     cancel,
		logger:         logger,
	}
	
	// Start background warming
	if config.Enabled {
		go warmer.backgroundWarming()
	}
	
	return warmer
}

// OptimizeWarming optimizes cache warming based on metrics
func (cw *CacheWarmer) OptimizeWarming(metrics *RealTimeMetrics) ([]OptimizationChange, error) {
	changes := make([]OptimizationChange, 0)
	
	// Implementation would analyze patterns and optimize warming
	
	return changes, nil
}

// backgroundWarming runs background cache warming
func (cw *CacheWarmer) backgroundWarming() {
	ticker := time.NewTicker(cw.config.WarmingInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			cw.performWarming()
		case <-cw.ctx.Done():
			return
		}
	}
}

// performWarming performs cache warming operations
func (cw *CacheWarmer) performWarming() {
	// Implementation would perform intelligent cache warming
}

// Close cleans up resources
func (cw *CacheWarmer) Close() error {
	if cw.cancelFunc != nil {
		cw.cancelFunc()
	}
	return nil
}

// NewRoutingLearningModel creates a new routing learning model
func NewRoutingLearningModel(config *LearningModelConfig) *RoutingLearningModel {
	return &RoutingLearningModel{
		modelType:    config.ModelType,
		features:     []string{"latency", "accuracy", "cache_hit", "complexity", "confidence"},
		trainingData: make([]RoutingTrainingDataPoint, 0),
		testData:     make([]RoutingTrainingDataPoint, 0),
		config:       config,
		lastTrained:  time.Now(),
		trained:      false,
	}
}

// Learn learns from a routing decision
func (rlm *RoutingLearningModel) Learn(decision *TrackedDecision) {
	// Implementation would update the learning model based on the decision
}