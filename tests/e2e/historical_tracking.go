package e2e_test

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"sort"
	"sync"
	"time"
)

// HistoricalTracker provides comprehensive historical tracking and analysis
type HistoricalTracker struct {
	mu                 sync.RWMutex
	config             *HistoricalConfig
	logger             *log.Logger
	persistenceManager *ResultPersistenceManager

	// Analysis components
	trendAnalyzer       *AdvancedTrendAnalyzer
	regressionDetector  *RegressionDetector
	baselineManager     *BaselineManager
	performanceProfiler *PerformanceProfiler

	// Cache for frequently accessed data
	dataCache *HistoricalDataCache

	// Background analysis
	analysisScheduler *AnalysisScheduler

	// Context and cleanup
	ctx              context.Context
	cancel           context.CancelFunc
	cleanupFunctions []func() error
}

// HistoricalConfig defines configuration for historical tracking
type HistoricalConfig struct {
	AnalysisWindowDays       int
	TrendAnalysisWindow      int
	RegressionThreshold      float64
	BaselineUpdateInterval   time.Duration
	CacheSize                int
	CacheTTL                 time.Duration
	EnableBackgroundAnalysis bool
	AnalysisInterval         time.Duration
	AlertingEnabled          bool
	AlertThresholds          *HistoricalAlertThresholds
}

// HistoricalAlertThresholds defines thresholds for historical alerting
type HistoricalAlertThresholds struct {
	PerformanceDegradation float64
	ErrorRateIncrease      float64
	DurationIncrease       float64
	ThroughputDecrease     float64
	ConsecutiveFailures    int
}

// AdvancedTrendAnalyzer provides sophisticated trend analysis
type AdvancedTrendAnalyzer struct {
	mu                sync.RWMutex
	config            *TrendAnalysisConfig
	statisticalModels map[string]*StatisticalModel
	seasonalDetector  *SeasonalityDetector
	anomalyDetector   *AnomalyDetector
}

// TrendAnalysisConfig configuration for trend analysis
type TrendAnalysisConfig struct {
	WindowSize        int
	SmoothingFactor   float64
	SeasonalityWindow int
	ConfidenceLevel   float64
	OutlierThreshold  float64
	ForecastHorizon   int
}

// StatisticalModel represents a statistical model for trend analysis
type StatisticalModel struct {
	ModelType        ModelType
	Parameters       map[string]float64
	Accuracy         float64
	LastUpdated      time.Time
	SampleCount      int
	PredictionWindow int
	Confidence       float64
}

// ModelType defines different statistical model types
type ModelType string

const (
	ModelTypeLinearRegression     ModelType = "linear_regression"
	ModelTypeExponentialSmoothing ModelType = "exponential_smoothing"
	ModelTypeARIMA                ModelType = "arima"
	ModelTypeSeasonalTrend        ModelType = "seasonal_trend"
	ModelTypePolynomial           ModelType = "polynomial"
)

// RegressionDetector detects performance regressions
type RegressionDetector struct {
	mu                sync.RWMutex
	config            *RegressionConfig
	baselineManager   *BaselineManager
	alertingSystem    *AlertingSystem
	regressionHistory []*RegressionEvent
	suppressionRules  map[string]*SuppressionRule
}

// RegressionConfig configuration for regression detection
type RegressionConfig struct {
	DetectionWindow        int
	MinSampleSize          int
	StatisticalMethod      StatisticalMethod
	ConfidenceLevel        float64
	AlertingEnabled        bool
	AutoSuppressionEnabled bool
	SuppressionDuration    time.Duration
}

// StatisticalMethod defines statistical methods for regression detection
type StatisticalMethod string

const (
	StatisticalMethodTTest       StatisticalMethod = "t_test"
	StatisticalMethodMannWhitney StatisticalMethod = "mann_whitney"
	StatisticalMethodWilcoxon    StatisticalMethod = "wilcoxon"
	StatisticalMethodBootstrap   StatisticalMethod = "bootstrap"
)

// RegressionEvent represents a detected regression
type RegressionEvent struct {
	ID                string
	TestName          string
	Scenario          string
	MetricName        string
	DetectedAt        time.Time
	BaselineValue     float64
	CurrentValue      float64
	RegressionPercent float64
	Confidence        float64
	Severity          RegressionSeverity
	StatisticalTest   string
	PValue            float64
	EffectSize        float64
	Context           map[string]interface{}
	Status            RegressionStatus
	ResolutionNotes   string
	FirstOccurrence   time.Time
	LastOccurrence    time.Time
	OccurrenceCount   int
}

// RegressionStatus represents the status of a regression
type RegressionStatus string

const (
	RegressionStatusNew           RegressionStatus = "new"
	RegressionStatusConfirmed     RegressionStatus = "confirmed"
	RegressionStatusResolved      RegressionStatus = "resolved"
	RegressionStatusSuppressed    RegressionStatus = "suppressed"
	RegressionStatusFalsePositive RegressionStatus = "false_positive"
)

// SuppressionRule defines rules for suppressing false positive regressions
type SuppressionRule struct {
	ID         string
	Pattern    string
	TestName   string
	MetricName string
	Threshold  float64
	Duration   time.Duration
	CreatedAt  time.Time
	ExpiresAt  *time.Time
	Reason     string
	CreatedBy  string
	Active     bool
}

// BaselineManager manages performance baselines
type BaselineManager struct {
	mu                 sync.RWMutex
	config             *BaselineConfig
	persistenceManager *ResultPersistenceManager
	baselines          map[string]*PerformanceBaseline
	baselineHistory    map[string][]*BaselineSnapshot
	updateScheduler    *BaselineUpdateScheduler
}

// BaselineConfig configuration for baseline management
type BaselineConfig struct {
	UpdateStrategy         BaselineUpdateStrategy
	UpdateInterval         time.Duration
	MinSampleSize          int
	MaxAge                 time.Duration
	OutlierFiltering       bool
	OutlierThreshold       float64
	ConfidenceLevel        float64
	AutoUpdateEnabled      bool
	ManualApprovalRequired bool
}

// BaselineUpdateStrategy defines strategies for updating baselines
type BaselineUpdateStrategy string

const (
	BaselineUpdateStrategyFixed      BaselineUpdateStrategy = "fixed"
	BaselineUpdateStrategyRolling    BaselineUpdateStrategy = "rolling"
	BaselineUpdateStrategyAdaptive   BaselineUpdateStrategy = "adaptive"
	BaselineUpdateStrategySeasonally BaselineUpdateStrategy = "seasonally"
)

// BaselineSnapshot represents a point-in-time baseline snapshot
type BaselineSnapshot struct {
	Timestamp    time.Time
	Baseline     *PerformanceBaseline
	ChangeReason string
	ApprovedBy   string
	SampleCount  int
	QualityScore float64
}

// BaselineUpdateScheduler schedules baseline updates
type BaselineUpdateScheduler struct {
	mu              sync.RWMutex
	updateQueue     []*BaselineUpdateTask
	processingTasks bool
	config          *BaselineConfig
}

// BaselineUpdateTask represents a baseline update task
type BaselineUpdateTask struct {
	ID           string
	TestName     string
	Scenario     string
	Environment  string
	Priority     TaskPriority
	ScheduledAt  time.Time
	Status       TaskStatus
	Progress     float64
	ErrorMessage string
}

// TaskPriority defines task priority levels
type TaskPriority string

const (
	TaskPriorityLow      TaskPriority = "low"
	TaskPriorityMedium   TaskPriority = "medium"
	TaskPriorityHigh     TaskPriority = "high"
	TaskPriorityCritical TaskPriority = "critical"
)

// TaskStatus defines task status
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// PerformanceProfiler profiles performance characteristics over time
type PerformanceProfiler struct {
	mu              sync.RWMutex
	config          *ProfilerConfig
	profiles        map[string]*PerformanceProfile
	profileHistory  map[string][]*ProfileSnapshot
	anomalyDetector *AnomalyDetector
}

// ProfilerConfig configuration for performance profiling
type ProfilerConfig struct {
	ProfilingInterval time.Duration
	ProfileRetention  time.Duration
	AnomalyDetection  bool
	MetricThresholds  map[string]float64
	ProfileComparison bool
	TrendAnalysis     bool
}

// PerformanceProfile represents a performance profile
type PerformanceProfile struct {
	TestName          string
	Scenario          string
	Environment       string
	LastUpdated       time.Time
	DurationProfile   *MetricProfile
	ThroughputProfile *MetricProfile
	MemoryProfile     *MetricProfile
	ErrorRateProfile  *MetricProfile
	ResourceProfile   *ResourceUtilizationProfile
	TrendAnalysis     *ProfileTrendAnalysis
	AnomalyScore      float64
	QualityScore      float64
	Stability         StabilityLevel
}

// MetricProfile represents statistical profile of a metric
type MetricProfile struct {
	Mean              float64
	Median            float64
	StandardDeviation float64
	Variance          float64
	Skewness          float64
	Kurtosis          float64
	Min               float64
	Max               float64
	Percentiles       map[int]float64 // P50, P90, P95, P99
	Distribution      DistributionType
	SampleCount       int
	LastUpdated       time.Time
}

// DistributionType defines statistical distribution types
type DistributionType string

const (
	DistributionTypeNormal      DistributionType = "normal"
	DistributionTypeLogNormal   DistributionType = "log_normal"
	DistributionTypeExponential DistributionType = "exponential"
	DistributionTypeUniform     DistributionType = "uniform"
	DistributionTypeGamma       DistributionType = "gamma"
	DistributionTypeWeibull     DistributionType = "weibull"
)

// ResourceUtilizationProfile represents resource utilization patterns
type ResourceUtilizationProfile struct {
	MemoryUtilization     *UtilizationPattern
	CPUUtilization        *UtilizationPattern
	NetworkUtilization    *UtilizationPattern
	DiskUtilization       *UtilizationPattern
	ConnectionUtilization *UtilizationPattern
}

// UtilizationPattern represents utilization patterns over time
type UtilizationPattern struct {
	BaselineUtilization float64
	PeakUtilization     float64
	AverageUtilization  float64
	UtilizationTrend    TrendDirection
	SeasonalPatterns    []SeasonalPattern
	GrowthRate          float64
}

// ProfileTrendAnalysis represents trend analysis for performance profiles
type ProfileTrendAnalysis struct {
	OverallTrend       TrendDirection
	DurationTrend      TrendDirection
	ThroughputTrend    TrendDirection
	MemoryTrend        TrendDirection
	ErrorRateTrend     TrendDirection
	TrendConfidence    float64
	ForecastAccuracy   float64
	NextPeriodForecast *PerformanceForecast
}

// StabilityLevel represents performance stability level
type StabilityLevel string

const (
	StabilityLevelVeryStable   StabilityLevel = "very_stable"
	StabilityLevelStable       StabilityLevel = "stable"
	StabilityLevelModerate     StabilityLevel = "moderate"
	StabilityLevelUnstable     StabilityLevel = "unstable"
	StabilityLevelVeryUnstable StabilityLevel = "very_unstable"
)

// ProfileSnapshot represents a point-in-time profile snapshot
type ProfileSnapshot struct {
	Timestamp time.Time
	Profile   *PerformanceProfile
	Changes   []ProfileChange
	Triggers  []string
}

// ProfileChange represents a change in performance profile
type ProfileChange struct {
	MetricName    string
	ChangeType    ChangeType
	PreviousValue float64
	CurrentValue  float64
	ChangePercent float64
	Significance  float64
	ChangeReason  string
}

// ChangeType defines types of profile changes
type ChangeType string

const (
	ChangeTypeImprovement ChangeType = "improvement"
	ChangeTypeDegradation ChangeType = "degradation"
	ChangeTypeFluctuation ChangeType = "fluctuation"
	ChangeTypeShift       ChangeType = "shift"
)

// SeasonalityDetector detects seasonal patterns in performance data
type SeasonalityDetector struct {
	mu           sync.RWMutex
	config       *SeasonalityConfig
	patterns     map[string][]*SeasonalPattern
	patternCache map[string]*CachedSeasonalAnalysis
}

// SeasonalityConfig configuration for seasonality detection
type SeasonalityConfig struct {
	MinDataPoints       int
	SeasonalityWindow   time.Duration
	ConfidenceThreshold float64
	DetectionMethods    []SeasonalityMethod
}

// SeasonalityMethod defines methods for seasonality detection
type SeasonalityMethod string

const (
	SeasonalityMethodAutocorrelation SeasonalityMethod = "autocorrelation"
	SeasonalityMethodFFT             SeasonalityMethod = "fft"
	SeasonalityMethodSTL             SeasonalityMethod = "stl"
	SeasonalityMethodXGBOOST         SeasonalityMethod = "xgboost"
)

// CachedSeasonalAnalysis represents cached seasonal analysis results
type CachedSeasonalAnalysis struct {
	Analysis   *SeasonalAnalysis
	CachedAt   time.Time
	ValidUntil time.Time
}

// SeasonalAnalysis represents seasonal analysis results
type SeasonalAnalysis struct {
	HasSeasonality   bool
	SeasonalStrength float64
	Periods          []SeasonalPeriod
	Forecast         []float64
	Confidence       float64
	Method           SeasonalityMethod
	AnalyzedPeriod   time.Duration
}

// SeasonalPeriod represents a detected seasonal period
type SeasonalPeriod struct {
	Period     time.Duration
	Strength   float64
	Phase      float64
	Amplitude  float64
	Confidence float64
}

// AnomalyDetector detects anomalies in performance data
type AnomalyDetector struct {
	mu             sync.RWMutex
	config         *AnomalyConfig
	models         map[string]*AnomalyModel
	anomalyHistory map[string][]*AnomalyEvent
}

// AnomalyConfig configuration for anomaly detection
type AnomalyConfig struct {
	DetectionMethods    []AnomalyMethod
	SensitivityLevel    SensitivityLevel
	WindowSize          int
	ConfidenceThreshold float64
	MinAnomalyDuration  time.Duration
	AlertingEnabled     bool
}

// AnomalyMethod defines methods for anomaly detection
type AnomalyMethod string

const (
	AnomalyMethodStatistical     AnomalyMethod = "statistical"
	AnomalyMethodIsolationForest AnomalyMethod = "isolation_forest"
	AnomalyMethodLocalOutlier    AnomalyMethod = "local_outlier"
	AnomalyMethodOneClassSVM     AnomalyMethod = "one_class_svm"
	AnomalyMethodAutoEncoder     AnomalyMethod = "autoencoder"
)

// SensitivityLevel defines anomaly detection sensitivity
type SensitivityLevel string

const (
	SensitivityLevelLow    SensitivityLevel = "low"
	SensitivityLevelMedium SensitivityLevel = "medium"
	SensitivityLevelHigh   SensitivityLevel = "high"
	SensitivityLevelUltra  SensitivityLevel = "ultra"
)

// AnomalyModel represents an anomaly detection model
type AnomalyModel struct {
	ModelType         AnomalyMethod
	Parameters        map[string]float64
	TrainingData      []float64
	Threshold         float64
	Accuracy          float64
	FalsePositiveRate float64
	LastTrained       time.Time
	ModelVersion      string
}

// AnomalyEvent represents a detected anomaly
type AnomalyEvent struct {
	ID            string
	TestName      string
	MetricName    string
	Timestamp     time.Time
	Value         float64
	ExpectedValue float64
	AnomalyScore  float64
	Severity      AnomalySeverity
	Method        AnomalyMethod
	Context       map[string]interface{}
	Duration      time.Duration
	Status        AnomalyStatus
	Resolution    string
}

// AnomalyStatus represents anomaly status
type AnomalyStatus string

const (
	AnomalyStatusActive   AnomalyStatus = "active"
	AnomalyStatusResolved AnomalyStatus = "resolved"
	AnomalyStatusIgnored  AnomalyStatus = "ignored"
)

// HistoricalDataCache provides caching for frequently accessed historical data
type HistoricalDataCache struct {
	mu        sync.RWMutex
	cache     map[string]*CacheEntry
	maxSize   int
	ttl       time.Duration
	hitCount  int64
	missCount int64
}

// CacheEntry represents a cache entry
type CacheEntry struct {
	Key          string
	Data         interface{}
	CreatedAt    time.Time
	ExpiresAt    time.Time
	AccessCount  int64
	LastAccessed time.Time
}

// AnalysisScheduler schedules and manages background analysis tasks
type AnalysisScheduler struct {
	mu             sync.RWMutex
	tasks          []*AnalysisTask
	running        bool
	workerCount    int
	taskQueue      chan *AnalysisTask
	completedTasks []*AnalysisTask
	failedTasks    []*AnalysisTask
}

// AnalysisTask represents a background analysis task
type AnalysisTask struct {
	ID           string
	TaskType     AnalysisTaskType
	Parameters   map[string]interface{}
	Priority     TaskPriority
	ScheduledAt  time.Time
	StartedAt    *time.Time
	CompletedAt  *time.Time
	Status       TaskStatus
	Progress     float64
	Result       interface{}
	ErrorMessage string
	RetryCount   int
	MaxRetries   int
}

// AnalysisTaskType defines types of analysis tasks
type AnalysisTaskType string

const (
	AnalysisTaskTypeTrendAnalysis       AnalysisTaskType = "trend_analysis"
	AnalysisTaskTypeRegressionDetection AnalysisTaskType = "regression_detection"
	AnalysisTaskTypeBaselineUpdate      AnalysisTaskType = "baseline_update"
	AnalysisTaskTypeProfileUpdate       AnalysisTaskType = "profile_update"
	AnalysisTaskTypeAnomalyDetection    AnalysisTaskType = "anomaly_detection"
	AnalysisTaskTypeSeasonalityAnalysis AnalysisTaskType = "seasonality_analysis"
)

// AlertingSystem manages alerts for historical analysis
type AlertingSystem struct {
	mu           sync.RWMutex
	config       *AlertingConfig
	activeAlerts map[string]*Alert
	alertHistory []*Alert
	notifiers    []AlertNotifier
}

// AlertingConfig configuration for alerting system
type AlertingConfig struct {
	EnabledChannels   []AlertChannel
	ThrottleInterval  time.Duration
	EscalationEnabled bool
	EscalationDelay   time.Duration
	MaxAlerts         int
	AlertRetention    time.Duration
}

// AlertChannel defines alert channels
type AlertChannel string

const (
	AlertChannelEmail   AlertChannel = "email"
	AlertChannelSlack   AlertChannel = "slack"
	AlertChannelWebhook AlertChannel = "webhook"
	AlertChannelConsole AlertChannel = "console"
)

// Alert represents an alert
type Alert struct {
	ID                string
	Type              AlertType
	Severity          AlertSeverity
	Title             string
	Message           string
	Context           map[string]interface{}
	CreatedAt         time.Time
	UpdatedAt         time.Time
	ResolvedAt        *time.Time
	Status            AlertStatus
	EscalationLevel   int
	NotificationsSent int
}

// AlertType defines alert types
type AlertType string

const (
	AlertTypeRegression AlertType = "regression"
	AlertTypeAnomaly    AlertType = "anomaly"
	AlertTypeThreshold  AlertType = "threshold"
	AlertTypeBaseline   AlertType = "baseline"
	AlertTypeSystem     AlertType = "system"
)

// AlertStatus represents alert status
type AlertStatus string

const (
	AlertStatusActive     AlertStatus = "active"
	AlertStatusResolved   AlertStatus = "resolved"
	AlertStatusSuppressed AlertStatus = "suppressed"
)

// AlertNotifier interface for alert notifications
type AlertNotifier interface {
	SendAlert(alert *Alert) error
	GetChannel() AlertChannel
	IsHealthy() bool
}

// NewHistoricalTracker creates a new historical tracker
func NewHistoricalTracker(config *HistoricalConfig, persistenceManager *ResultPersistenceManager) (*HistoricalTracker, error) {
	if config == nil {
		config = getDefaultHistoricalConfig()
	}

	if err := validateHistoricalConfig(config); err != nil {
		return nil, fmt.Errorf("invalid historical configuration: %w", err)
	}

	logger := log.New(os.Stdout, "[E2E-Historical] ", log.LstdFlags|log.Lshortfile)
	ctx, cancel := context.WithCancel(context.Background())

	tracker := &HistoricalTracker{
		config:             config,
		logger:             logger,
		persistenceManager: persistenceManager,
		ctx:                ctx,
		cancel:             cancel,
		cleanupFunctions:   make([]func() error, 0),
	}

	// Initialize components
	if err := tracker.initializeComponents(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize components: %w", err)
	}

	// Start background analysis if enabled
	if config.EnableBackgroundAnalysis {
		go tracker.startBackgroundAnalysis()
	}

	logger.Printf("Historical tracker initialized")
	return tracker, nil
}

// initializeComponents initializes all tracker components
func (ht *HistoricalTracker) initializeComponents() error {
	// Initialize trend analyzer
	trendConfig := &TrendAnalysisConfig{
		WindowSize:        ht.config.TrendAnalysisWindow,
		SmoothingFactor:   0.3,
		SeasonalityWindow: 7 * 24, // 7 days in hours
		ConfidenceLevel:   0.95,
		OutlierThreshold:  2.0,
		ForecastHorizon:   24, // 24 periods
	}
	ht.trendAnalyzer = &AdvancedTrendAnalyzer{
		config:            trendConfig,
		statisticalModels: make(map[string]*StatisticalModel),
		seasonalDetector: &SeasonalityDetector{
			patterns:     make(map[string][]*SeasonalPattern),
			patternCache: make(map[string]*CachedSeasonalAnalysis),
			config: &SeasonalityConfig{
				MinDataPoints:       50,
				SeasonalityWindow:   7 * 24 * time.Hour,
				ConfidenceThreshold: 0.8,
				DetectionMethods:    []SeasonalityMethod{SeasonalityMethodAutocorrelation, SeasonalityMethodFFT},
			},
		},
		anomalyDetector: &AnomalyDetector{
			models:         make(map[string]*AnomalyModel),
			anomalyHistory: make(map[string][]*AnomalyEvent),
			config: &AnomalyConfig{
				DetectionMethods:    []AnomalyMethod{AnomalyMethodStatistical, AnomalyMethodIsolationForest},
				SensitivityLevel:    SensitivityLevelMedium,
				WindowSize:          100,
				ConfidenceThreshold: 0.95,
				MinAnomalyDuration:  5 * time.Minute,
				AlertingEnabled:     true,
			},
		},
	}

	// Initialize regression detector
	regressionConfig := &RegressionConfig{
		DetectionWindow:        30,
		MinSampleSize:          10,
		StatisticalMethod:      StatisticalMethodTTest,
		ConfidenceLevel:        0.95,
		AlertingEnabled:        ht.config.AlertingEnabled,
		AutoSuppressionEnabled: true,
		SuppressionDuration:    24 * time.Hour,
	}
	ht.regressionDetector = &RegressionDetector{
		config:            regressionConfig,
		regressionHistory: make([]*RegressionEvent, 0),
		suppressionRules:  make(map[string]*SuppressionRule),
	}

	// Initialize baseline manager
	baselineConfig := &BaselineConfig{
		UpdateStrategy:         BaselineUpdateStrategyRolling,
		UpdateInterval:         ht.config.BaselineUpdateInterval,
		MinSampleSize:          20,
		MaxAge:                 30 * 24 * time.Hour,
		OutlierFiltering:       true,
		OutlierThreshold:       2.0,
		ConfidenceLevel:        0.95,
		AutoUpdateEnabled:      true,
		ManualApprovalRequired: false,
	}
	ht.baselineManager = &BaselineManager{
		config:             baselineConfig,
		persistenceManager: ht.persistenceManager,
		baselines:          make(map[string]*PerformanceBaseline),
		baselineHistory:    make(map[string][]*BaselineSnapshot),
		updateScheduler: &BaselineUpdateScheduler{
			updateQueue: make([]*BaselineUpdateTask, 0),
			config:      baselineConfig,
		},
	}

	// Initialize performance profiler
	profilerConfig := &ProfilerConfig{
		ProfilingInterval: 1 * time.Hour,
		ProfileRetention:  30 * 24 * time.Hour,
		AnomalyDetection:  true,
		MetricThresholds: map[string]float64{
			"duration":   1000, // ms
			"throughput": 100,  // req/s
			"memory":     1024, // MB
			"error_rate": 5.0,  // %
		},
		ProfileComparison: true,
		TrendAnalysis:     true,
	}
	ht.performanceProfiler = &PerformanceProfiler{
		config:          profilerConfig,
		profiles:        make(map[string]*PerformanceProfile),
		profileHistory:  make(map[string][]*ProfileSnapshot),
		anomalyDetector: ht.trendAnalyzer.anomalyDetector,
	}

	// Initialize data cache
	ht.dataCache = &HistoricalDataCache{
		cache:   make(map[string]*CacheEntry),
		maxSize: ht.config.CacheSize,
		ttl:     ht.config.CacheTTL,
	}

	// Initialize analysis scheduler
	ht.analysisScheduler = &AnalysisScheduler{
		tasks:          make([]*AnalysisTask, 0),
		workerCount:    4,
		taskQueue:      make(chan *AnalysisTask, 100),
		completedTasks: make([]*AnalysisTask, 0),
		failedTasks:    make([]*AnalysisTask, 0),
	}

	return nil
}

// AnalyzeHistoricalTrends analyzes historical trends for given test data
func (ht *HistoricalTracker) AnalyzeHistoricalTrends(testName, scenario string, days int) (*HistoricalTrendAnalysis, error) {
	ht.mu.RLock()
	defer ht.mu.RUnlock()

	// Check cache first
	cacheKey := fmt.Sprintf("trends_%s_%s_%d", testName, scenario, days)
	if cached := ht.dataCache.Get(cacheKey); cached != nil {
		if analysis, ok := cached.(*HistoricalTrendAnalysis); ok {
			return analysis, nil
		}
	}

	// Get historical data
	historicalData, err := ht.persistenceManager.GetHistoricalData(days, "")
	if err != nil {
		return nil, fmt.Errorf("failed to get historical data: %w", err)
	}

	// Filter data for specific test and scenario
	var filteredData []*TestRunRecord
	for _, record := range historicalData {
		if ht.matchesTestAndScenario(record, testName, scenario) {
			filteredData = append(filteredData, record)
		}
	}

	if len(filteredData) < 5 {
		return nil, fmt.Errorf("insufficient data for trend analysis: need at least 5 data points, got %d", len(filteredData))
	}

	// Perform trend analysis
	analysis := ht.performTrendAnalysis(filteredData, testName, scenario)

	// Cache the result
	ht.dataCache.Set(cacheKey, analysis, ht.config.CacheTTL)

	return analysis, nil
}

// DetectRegressions detects performance regressions
func (ht *HistoricalTracker) DetectRegressions(testName, scenario string) ([]*RegressionEvent, error) {
	ht.mu.RLock()
	defer ht.mu.RUnlock()

	// Get recent data for regression detection
	recentData, err := ht.persistenceManager.GetHistoricalData(ht.regressionDetector.config.DetectionWindow, "")
	if err != nil {
		return nil, fmt.Errorf("failed to get recent data: %w", err)
	}

	// Filter data
	var filteredData []*TestRunRecord
	for _, record := range recentData {
		if ht.matchesTestAndScenario(record, testName, scenario) {
			filteredData = append(filteredData, record)
		}
	}

	if len(filteredData) < ht.regressionDetector.config.MinSampleSize {
		return []*RegressionEvent{}, nil
	}

	// Get baseline for comparison
	baseline, err := ht.baselineManager.GetBaseline(testName, scenario, "")
	if err != nil {
		return nil, fmt.Errorf("failed to get baseline: %w", err)
	}

	// Perform regression detection
	regressions := ht.performRegressionDetection(filteredData, baseline, testName, scenario)

	return regressions, nil
}

// UpdateBaselines updates performance baselines
func (ht *HistoricalTracker) UpdateBaselines(testName, scenario, environment string) error {
	ht.mu.Lock()
	defer ht.mu.Unlock()

	// Get recent data for baseline calculation
	recentData, err := ht.persistenceManager.GetHistoricalData(30, environment)
	if err != nil {
		return fmt.Errorf("failed to get recent data: %w", err)
	}

	// Filter data
	var filteredData []*TestRunRecord
	for _, record := range recentData {
		if ht.matchesTestAndScenario(record, testName, scenario) {
			filteredData = append(filteredData, record)
		}
	}

	if len(filteredData) < ht.baselineManager.config.MinSampleSize {
		return fmt.Errorf("insufficient data for baseline calculation: need at least %d data points, got %d",
			ht.baselineManager.config.MinSampleSize, len(filteredData))
	}

	// Calculate new baseline
	newBaseline := ht.calculateBaseline(filteredData, testName, scenario, environment)

	// Store baseline
	baselineKey := fmt.Sprintf("%s_%s_%s", testName, scenario, environment)

	// Store previous baseline in history
	if existing, exists := ht.baselineManager.baselines[baselineKey]; exists {
		snapshot := &BaselineSnapshot{
			Timestamp:    time.Now(),
			Baseline:     existing,
			ChangeReason: "automatic_update",
			SampleCount:  len(filteredData),
			QualityScore: ht.calculateBaselineQuality(existing, filteredData),
		}
		ht.baselineManager.baselineHistory[baselineKey] = append(ht.baselineManager.baselineHistory[baselineKey], snapshot)
	}

	// Update baseline
	ht.baselineManager.baselines[baselineKey] = newBaseline

	ht.logger.Printf("Updated baseline for %s/%s in %s environment", testName, scenario, environment)
	return nil
}

// GetPerformanceProfile gets performance profile for a test
func (ht *HistoricalTracker) GetPerformanceProfile(testName, scenario, environment string) (*PerformanceProfile, error) {
	ht.mu.RLock()
	defer ht.mu.RUnlock()

	profileKey := fmt.Sprintf("%s_%s_%s", testName, scenario, environment)

	// Check if profile exists and is recent
	if profile, exists := ht.performanceProfiler.profiles[profileKey]; exists {
		if time.Since(profile.LastUpdated) < ht.performanceProfiler.config.ProfilingInterval {
			return profile, nil
		}
	}

	// Update profile
	return ht.updatePerformanceProfile(testName, scenario, environment)
}

// updatePerformanceProfile updates performance profile for a test
func (ht *HistoricalTracker) updatePerformanceProfile(testName, scenario, environment string) (*PerformanceProfile, error) {
	// Get historical data for profiling
	profileData, err := ht.persistenceManager.GetHistoricalData(30, environment)
	if err != nil {
		return nil, fmt.Errorf("failed to get profile data: %w", err)
	}

	// Filter data
	var filteredData []*TestRunRecord
	for _, record := range profileData {
		if ht.matchesTestAndScenario(record, testName, scenario) {
			filteredData = append(filteredData, record)
		}
	}

	if len(filteredData) < 10 {
		return nil, fmt.Errorf("insufficient data for profiling: need at least 10 data points, got %d", len(filteredData))
	}

	// Create performance profile
	profile := ht.createPerformanceProfile(filteredData, testName, scenario, environment)

	// Store profile
	profileKey := fmt.Sprintf("%s_%s_%s", testName, scenario, environment)
	ht.performanceProfiler.profiles[profileKey] = profile

	return profile, nil
}

// Helper methods

func (ht *HistoricalTracker) matchesTestAndScenario(record *TestRunRecord, testName, scenario string) bool {
	if testName != "" && scenario != "" {
		// Check if any test result matches both test name and scenario
		for _, result := range record.TestResults {
			if result.TestName == testName && result.Scenario == scenario {
				return true
			}
		}
		return false
	} else if testName != "" {
		// Check if any test result matches test name
		for _, result := range record.TestResults {
			if result.TestName == testName {
				return true
			}
		}
		return false
	}
	// If no specific filters, include all records
	return true
}

func (ht *HistoricalTracker) performTrendAnalysis(data []*TestRunRecord, testName, scenario string) *HistoricalTrendAnalysis {
	// Extract time series data
	var timestamps []time.Time
	var durations []float64
	var throughputs []float64
	var errorRates []float64

	for _, record := range data {
		for _, result := range record.TestResults {
			if (testName == "" || result.TestName == testName) && (scenario == "" || result.Scenario == scenario) {
				timestamps = append(timestamps, result.StartTime)
				durations = append(durations, float64(result.Duration.Milliseconds()))

				if result.Metrics != nil {
					throughputs = append(throughputs, result.Metrics.ThroughputPerSecond)
					errorRates = append(errorRates, result.Metrics.ErrorRate)
				}
			}
		}
	}

	// Analyze trends
	durationTrend := ht.analyzeTrendDirection(durations)
	throughputTrend := ht.analyzeTrendDirection(throughputs)
	errorRateTrend := ht.analyzeTrendDirection(errorRates)

	// Calculate volatility
	durationVolatility := ht.calculateVolatility(durations)
	throughputVolatility := ht.calculateVolatility(throughputs)
	errorRateVolatility := ht.calculateVolatility(errorRates)

	// Detect seasonality
	seasonalPatterns := ht.detectSeasonality(timestamps, durations)

	// Generate forecast
	durationForecast := ht.generateForecast(durations, 10)

	return &HistoricalTrendAnalysis{
		LongTermTrends: map[string]TrendDirection{
			"duration":   durationTrend,
			"throughput": throughputTrend,
			"error_rate": errorRateTrend,
		},
		ShortTermTrends: map[string]TrendDirection{
			"duration":   ht.analyzeTrendDirection(durations[len(durations)-10:]),
			"throughput": ht.analyzeTrendDirection(throughputs[len(throughputs)-10:]),
			"error_rate": ht.analyzeTrendDirection(errorRates[len(errorRates)-10:]),
		},
		VolatilityAnalysis: map[string]float64{
			"duration":   durationVolatility,
			"throughput": throughputVolatility,
			"error_rate": errorRateVolatility,
		},
		PredictiveTrends: map[string]float64{
			"duration_forecast": durationForecast[0],
		},
		TrendReliability: ht.calculateTrendReliability(durations),
	}
}

func (ht *HistoricalTracker) performRegressionDetection(data []*TestRunRecord, baseline *PerformanceBaseline, testName, scenario string) []*RegressionEvent {
	var regressions []*RegressionEvent

	// Extract recent values
	var recentDurations []float64
	var recentThroughputs []float64
	var recentErrorRates []float64

	for _, record := range data {
		for _, result := range record.TestResults {
			if result.TestName == testName && result.Scenario == scenario {
				recentDurations = append(recentDurations, float64(result.Duration.Milliseconds()))
				if result.Metrics != nil {
					recentThroughputs = append(recentThroughputs, result.Metrics.ThroughputPerSecond)
					recentErrorRates = append(recentErrorRates, result.Metrics.ErrorRate)
				}
			}
		}
	}

	// Check duration regression
	if len(recentDurations) > 0 && baseline != nil {
		avgDuration := ht.calculateMean(recentDurations)
		baselineDuration := float64(baseline.BaselineDuration.Milliseconds())

		if ht.isRegression(avgDuration, baselineDuration, "duration") {
			regression := &RegressionEvent{
				ID:                fmt.Sprintf("reg_%s_%s_%d", testName, scenario, time.Now().Unix()),
				TestName:          testName,
				Scenario:          scenario,
				MetricName:        "duration",
				DetectedAt:        time.Now(),
				BaselineValue:     baselineDuration,
				CurrentValue:      avgDuration,
				RegressionPercent: ((avgDuration - baselineDuration) / baselineDuration) * 100,
				Confidence:        ht.calculateRegressionConfidence(recentDurations, baselineDuration),
				Severity:          ht.calculateRegressionSeverity(avgDuration, baselineDuration),
				StatisticalTest:   string(ht.regressionDetector.config.StatisticalMethod),
				Status:            RegressionStatusNew,
				FirstOccurrence:   time.Now(),
				LastOccurrence:    time.Now(),
				OccurrenceCount:   1,
			}
			regressions = append(regressions, regression)
		}
	}

	// Check throughput regression
	if len(recentThroughputs) > 0 && baseline != nil {
		avgThroughput := ht.calculateMean(recentThroughputs)
		baselineThroughput := baseline.BaselineThroughput

		if ht.isRegression(baselineThroughput, avgThroughput, "throughput") { // Note: lower throughput is regression
			regression := &RegressionEvent{
				ID:                fmt.Sprintf("reg_%s_%s_throughput_%d", testName, scenario, time.Now().Unix()),
				TestName:          testName,
				Scenario:          scenario,
				MetricName:        "throughput",
				DetectedAt:        time.Now(),
				BaselineValue:     baselineThroughput,
				CurrentValue:      avgThroughput,
				RegressionPercent: ((baselineThroughput - avgThroughput) / baselineThroughput) * 100,
				Confidence:        ht.calculateRegressionConfidence(recentThroughputs, baselineThroughput),
				Severity:          ht.calculateRegressionSeverity(baselineThroughput, avgThroughput),
				StatisticalTest:   string(ht.regressionDetector.config.StatisticalMethod),
				Status:            RegressionStatusNew,
				FirstOccurrence:   time.Now(),
				LastOccurrence:    time.Now(),
				OccurrenceCount:   1,
			}
			regressions = append(regressions, regression)
		}
	}

	return regressions
}

func (ht *HistoricalTracker) calculateBaseline(data []*TestRunRecord, testName, scenario, environment string) *PerformanceBaseline {
	var durations []float64
	var throughputs []float64
	var memoryUsages []float64

	for _, record := range data {
		for _, result := range record.TestResults {
			if result.TestName == testName && result.Scenario == scenario {
				durations = append(durations, float64(result.Duration.Milliseconds()))
				if result.Metrics != nil {
					throughputs = append(throughputs, result.Metrics.ThroughputPerSecond)
				}
				if result.ResourceUsage != nil {
					memoryUsages = append(memoryUsages, result.ResourceUsage.MemoryUsedMB)
				}
			}
		}
	}

	// Filter outliers if enabled
	if ht.baselineManager.config.OutlierFiltering {
		durations = ht.filterOutliers(durations, ht.baselineManager.config.OutlierThreshold)
		throughputs = ht.filterOutliers(throughputs, ht.baselineManager.config.OutlierThreshold)
		memoryUsages = ht.filterOutliers(memoryUsages, ht.baselineManager.config.OutlierThreshold)
	}

	// Calculate baseline values
	avgDuration := time.Duration(ht.calculateMean(durations)) * time.Millisecond
	avgThroughput := ht.calculateMean(throughputs)
	avgMemory := ht.calculateMean(memoryUsages)

	// Calculate confidence based on sample size and variance
	confidence := ht.calculateBaselineConfidence(durations, len(data))

	return &PerformanceBaseline{
		TestName:           testName,
		BaselineDuration:   avgDuration,
		BaselineThroughput: avgThroughput,
		BaselineMemory:     avgMemory,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
		SampleCount:        len(data),
		Confidence:         confidence,
		Metadata: map[string]interface{}{
			"scenario":           scenario,
			"environment":        environment,
			"calculation_method": string(ht.baselineManager.config.UpdateStrategy),
		},
	}
}

func (ht *HistoricalTracker) createPerformanceProfile(data []*TestRunRecord, testName, scenario, environment string) *PerformanceProfile {
	var durations []float64
	var throughputs []float64
	var memoryUsages []float64
	var errorRates []float64

	for _, record := range data {
		for _, result := range record.TestResults {
			if result.TestName == testName && result.Scenario == scenario {
				durations = append(durations, float64(result.Duration.Milliseconds()))
				if result.Metrics != nil {
					throughputs = append(throughputs, result.Metrics.ThroughputPerSecond)
					errorRates = append(errorRates, result.Metrics.ErrorRate)
				}
				if result.ResourceUsage != nil {
					memoryUsages = append(memoryUsages, result.ResourceUsage.MemoryUsedMB)
				}
			}
		}
	}

	// Create metric profiles
	durationProfile := ht.createMetricProfile(durations)
	throughputProfile := ht.createMetricProfile(throughputs)
	memoryProfile := ht.createMetricProfile(memoryUsages)
	errorRateProfile := ht.createMetricProfile(errorRates)

	// Analyze trends
	trendAnalysis := &ProfileTrendAnalysis{
		OverallTrend:     ht.analyzeTrendDirection(durations),
		DurationTrend:    ht.analyzeTrendDirection(durations),
		ThroughputTrend:  ht.analyzeTrendDirection(throughputs),
		MemoryTrend:      ht.analyzeTrendDirection(memoryUsages),
		ErrorRateTrend:   ht.analyzeTrendDirection(errorRates),
		TrendConfidence:  ht.calculateTrendReliability(durations),
		ForecastAccuracy: 0.85, // TODO: Calculate based on historical forecast accuracy
	}

	// Calculate stability
	stability := ht.calculateStability(durations, throughputs, errorRates)

	// Calculate quality score
	qualityScore := ht.calculateProfileQuality(durationProfile, throughputProfile, errorRateProfile)

	return &PerformanceProfile{
		TestName:          testName,
		Scenario:          scenario,
		Environment:       environment,
		LastUpdated:       time.Now(),
		DurationProfile:   durationProfile,
		ThroughputProfile: throughputProfile,
		MemoryProfile:     memoryProfile,
		ErrorRateProfile:  errorRateProfile,
		TrendAnalysis:     trendAnalysis,
		QualityScore:      qualityScore,
		Stability:         stability,
	}
}

// Statistical analysis helper methods

func (ht *HistoricalTracker) analyzeTrendDirection(values []float64) TrendDirection {
	if len(values) < 3 {
		return TrendDirectionUnknown
	}

	// Simple linear regression to determine trend
	n := float64(len(values))
	var sumX, sumY, sumXY, sumX2 float64

	for i, y := range values {
		x := float64(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	slope := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)

	// Determine trend based on slope and significance
	if math.Abs(slope) < 0.01 {
		return TrendDirectionStable
	} else if slope > 0 {
		return TrendDirectionUp
	} else {
		return TrendDirectionDown
	}
}

func (ht *HistoricalTracker) calculateVolatility(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}

	mean := ht.calculateMean(values)
	var sumSquaredDiff float64

	for _, value := range values {
		diff := value - mean
		sumSquaredDiff += diff * diff
	}

	variance := sumSquaredDiff / float64(len(values)-1)
	return math.Sqrt(variance) / mean // Coefficient of variation
}

func (ht *HistoricalTracker) calculateMean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	var sum float64
	for _, value := range values {
		sum += value
	}
	return sum / float64(len(values))
}

func (ht *HistoricalTracker) calculateTrendReliability(values []float64) float64 {
	if len(values) < 5 {
		return 0.5 // Low reliability for small samples
	}

	// Calculate R-squared for linear trend
	n := float64(len(values))
	var sumX, sumY, sumXY, sumX2, sumY2 float64

	for i, y := range values {
		x := float64(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
		sumY2 += y * y
	}

	numerator := n*sumXY - sumX*sumY
	denominatorX := n*sumX2 - sumX*sumX
	denominatorY := n*sumY2 - sumY*sumY

	if denominatorX == 0 || denominatorY == 0 {
		return 0
	}

	correlation := numerator / math.Sqrt(denominatorX*denominatorY)
	return correlation * correlation // R-squared
}

func (ht *HistoricalTracker) isRegression(currentValue, baselineValue float64, metricType string) bool {
	threshold := ht.config.RegressionThreshold

	switch metricType {
	case "duration", "error_rate", "memory":
		// Higher values are regressions
		return (currentValue-baselineValue)/baselineValue*100 > threshold
	case "throughput":
		// Lower values are regressions
		return (baselineValue-currentValue)/baselineValue*100 > threshold
	default:
		return false
	}
}

func (ht *HistoricalTracker) calculateRegressionConfidence(values []float64, baseline float64) float64 {
	if len(values) < 3 {
		return 0.5
	}

	// Simplified statistical confidence calculation
	mean := ht.calculateMean(values)
	stdDev := ht.calculateStandardDeviation(values)

	if stdDev == 0 {
		return 1.0
	}

	// Z-score calculation
	zScore := math.Abs(mean-baseline) / stdDev

	// Convert to confidence (simplified)
	confidence := math.Min(zScore/3.0, 1.0)
	return confidence
}

func (ht *HistoricalTracker) calculateRegressionSeverity(currentValue, baselineValue float64) RegressionSeverity {
	changePercent := math.Abs((currentValue - baselineValue) / baselineValue * 100)

	switch {
	case changePercent >= 50:
		return RegressionSeverityCritical
	case changePercent >= 25:
		return RegressionSeverityMajor
	case changePercent >= 10:
		return RegressionSeverityModerate
	default:
		return RegressionSeverityMinor
	}
}

func (ht *HistoricalTracker) calculateStandardDeviation(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}

	mean := ht.calculateMean(values)
	var sumSquaredDiff float64

	for _, value := range values {
		diff := value - mean
		sumSquaredDiff += diff * diff
	}

	variance := sumSquaredDiff / float64(len(values)-1)
	return math.Sqrt(variance)
}

func (ht *HistoricalTracker) filterOutliers(values []float64, threshold float64) []float64 {
	if len(values) < 3 {
		return values
	}

	mean := ht.calculateMean(values)
	stdDev := ht.calculateStandardDeviation(values)

	var filtered []float64
	for _, value := range values {
		zScore := math.Abs(value-mean) / stdDev
		if zScore <= threshold {
			filtered = append(filtered, value)
		}
	}

	// Ensure we don't filter out too many values
	if len(filtered) < len(values)/2 {
		return values
	}

	return filtered
}

func (ht *HistoricalTracker) calculateBaselineConfidence(values []float64, sampleCount int) float64 {
	if len(values) == 0 {
		return 0
	}

	// Base confidence on sample size and coefficient of variation
	sampleConfidence := math.Min(float64(sampleCount)/50.0, 1.0) // Max confidence at 50+ samples

	cv := ht.calculateVolatility(values)
	variabilityConfidence := math.Max(0, 1.0-cv) // Lower variability = higher confidence

	return (sampleConfidence + variabilityConfidence) / 2.0
}

func (ht *HistoricalTracker) calculateBaselineQuality(baseline *PerformanceBaseline, data []*TestRunRecord) float64 {
	// Simplified quality calculation based on age, sample count, and variance
	age := time.Since(baseline.UpdatedAt)
	ageFactor := math.Max(0, 1.0-age.Hours()/(30*24)) // Decrease over 30 days

	sampleFactor := math.Min(float64(baseline.SampleCount)/50.0, 1.0)

	return (ageFactor + sampleFactor + baseline.Confidence) / 3.0
}

func (ht *HistoricalTracker) createMetricProfile(values []float64) *MetricProfile {
	if len(values) == 0 {
		return &MetricProfile{}
	}

	// Sort for percentile calculations
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	mean := ht.calculateMean(values)
	stdDev := ht.calculateStandardDeviation(values)

	// Calculate percentiles
	percentiles := map[int]float64{
		50: ht.calculatePercentile(sorted, 50),
		90: ht.calculatePercentile(sorted, 90),
		95: ht.calculatePercentile(sorted, 95),
		99: ht.calculatePercentile(sorted, 99),
	}

	return &MetricProfile{
		Mean:              mean,
		Median:            percentiles[50],
		StandardDeviation: stdDev,
		Variance:          stdDev * stdDev,
		Min:               sorted[0],
		Max:               sorted[len(sorted)-1],
		Percentiles:       percentiles,
		Distribution:      ht.detectDistributionType(values),
		SampleCount:       len(values),
		LastUpdated:       time.Now(),
	}
}

func (ht *HistoricalTracker) calculatePercentile(sortedValues []float64, percentile int) float64 {
	if len(sortedValues) == 0 {
		return 0
	}

	index := float64(percentile) / 100.0 * float64(len(sortedValues)-1)
	lower := int(math.Floor(index))
	upper := int(math.Ceil(index))

	if lower == upper {
		return sortedValues[lower]
	}

	weight := index - float64(lower)
	return sortedValues[lower]*(1-weight) + sortedValues[upper]*weight
}

func (ht *HistoricalTracker) detectDistributionType(values []float64) DistributionType {
	// Simplified distribution detection based on skewness and kurtosis
	mean := ht.calculateMean(values)
	stdDev := ht.calculateStandardDeviation(values)

	if stdDev == 0 {
		return DistributionTypeUniform
	}

	// Calculate skewness
	var skewnessSum float64
	for _, value := range values {
		skewnessSum += math.Pow((value-mean)/stdDev, 3)
	}
	skewness := skewnessSum / float64(len(values))

	// Simple classification based on skewness
	if math.Abs(skewness) < 0.5 {
		return DistributionTypeNormal
	} else if skewness > 0.5 {
		return DistributionTypeLogNormal
	} else {
		return DistributionTypeExponential
	}
}

func (ht *HistoricalTracker) calculateStability(durations, throughputs, errorRates []float64) StabilityLevel {
	// Calculate coefficient of variation for each metric
	durationCV := ht.calculateVolatility(durations)
	throughputCV := ht.calculateVolatility(throughputs)
	errorRateCV := ht.calculateVolatility(errorRates)

	// Average CV
	avgCV := (durationCV + throughputCV + errorRateCV) / 3.0

	// Classify stability
	switch {
	case avgCV < 0.1:
		return StabilityLevelVeryStable
	case avgCV < 0.2:
		return StabilityLevelStable
	case avgCV < 0.4:
		return StabilityLevelModerate
	case avgCV < 0.8:
		return StabilityLevelUnstable
	default:
		return StabilityLevelVeryUnstable
	}
}

func (ht *HistoricalTracker) calculateProfileQuality(duration, throughput, errorRate *MetricProfile) float64 {
	// Quality based on sample size, stability, and consistency
	sampleQuality := math.Min(float64(duration.SampleCount)/100.0, 1.0)

	// Stability based on coefficient of variation
	durationStability := 1.0 - math.Min(duration.StandardDeviation/duration.Mean, 1.0)
	throughputStability := 1.0 - math.Min(throughput.StandardDeviation/throughput.Mean, 1.0)
	errorRateStability := 1.0 - math.Min(errorRate.StandardDeviation/errorRate.Mean, 1.0)

	avgStability := (durationStability + throughputStability + errorRateStability) / 3.0

	return (sampleQuality + avgStability) / 2.0
}

func (ht *HistoricalTracker) detectSeasonality(timestamps []time.Time, values []float64) []SeasonalPattern {
	// Simplified seasonality detection
	// In a real implementation, this would use more sophisticated algorithms like FFT or STL decomposition
	return []SeasonalPattern{}
}

func (ht *HistoricalTracker) generateForecast(values []float64, periods int) []float64 {
	// Simple linear extrapolation for demonstration
	if len(values) < 2 {
		return make([]float64, periods)
	}

	// Calculate trend
	n := float64(len(values))
	var sumX, sumY, sumXY, sumX2 float64

	for i, y := range values {
		x := float64(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	slope := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
	intercept := (sumY - slope*sumX) / n

	// Generate forecast
	forecast := make([]float64, periods)
	for i := 0; i < periods; i++ {
		x := float64(len(values) + i)
		forecast[i] = slope*x + intercept
	}

	return forecast
}

// Cache implementation

func (hdc *HistoricalDataCache) Get(key string) interface{} {
	hdc.mu.RLock()
	defer hdc.mu.RUnlock()

	entry, exists := hdc.cache[key]
	if !exists {
		hdc.missCount++
		return nil
	}

	if time.Now().After(entry.ExpiresAt) {
		// Entry expired
		delete(hdc.cache, key)
		hdc.missCount++
		return nil
	}

	entry.AccessCount++
	entry.LastAccessed = time.Now()
	hdc.hitCount++
	return entry.Data
}

func (hdc *HistoricalDataCache) Set(key string, data interface{}, ttl time.Duration) {
	hdc.mu.Lock()
	defer hdc.mu.Unlock()

	// Evict if cache is full
	if len(hdc.cache) >= hdc.maxSize {
		hdc.evictLRU()
	}

	entry := &CacheEntry{
		Key:          key,
		Data:         data,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(ttl),
		AccessCount:  1,
		LastAccessed: time.Now(),
	}

	hdc.cache[key] = entry
}

func (hdc *HistoricalDataCache) evictLRU() {
	var oldestKey string
	var oldestTime time.Time = time.Now()

	for key, entry := range hdc.cache {
		if entry.LastAccessed.Before(oldestTime) {
			oldestTime = entry.LastAccessed
			oldestKey = key
		}
	}

	if oldestKey != "" {
		delete(hdc.cache, oldestKey)
	}
}

// Background analysis

func (ht *HistoricalTracker) startBackgroundAnalysis() {
	ticker := time.NewTicker(ht.config.AnalysisInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ht.ctx.Done():
			return
		case <-ticker.C:
			ht.performBackgroundAnalysis()
		}
	}
}

func (ht *HistoricalTracker) performBackgroundAnalysis() {
	// Schedule various analysis tasks
	tasks := []*AnalysisTask{
		{
			ID:          fmt.Sprintf("trend_analysis_%d", time.Now().Unix()),
			TaskType:    AnalysisTaskTypeTrendAnalysis,
			Priority:    TaskPriorityMedium,
			ScheduledAt: time.Now(),
			Status:      TaskStatusPending,
			MaxRetries:  3,
		},
		{
			ID:          fmt.Sprintf("regression_detection_%d", time.Now().Unix()),
			TaskType:    AnalysisTaskTypeRegressionDetection,
			Priority:    TaskPriorityHigh,
			ScheduledAt: time.Now(),
			Status:      TaskStatusPending,
			MaxRetries:  3,
		},
		{
			ID:          fmt.Sprintf("baseline_update_%d", time.Now().Unix()),
			TaskType:    AnalysisTaskTypeBaselineUpdate,
			Priority:    TaskPriorityLow,
			ScheduledAt: time.Now(),
			Status:      TaskStatusPending,
			MaxRetries:  2,
		},
	}

	for _, task := range tasks {
		ht.analysisScheduler.ScheduleTask(task)
	}
}

func (as *AnalysisScheduler) ScheduleTask(task *AnalysisTask) {
	as.mu.Lock()
	defer as.mu.Unlock()

	as.tasks = append(as.tasks, task)

	// Sort by priority and scheduled time
	sort.Slice(as.tasks, func(i, j int) bool {
		if as.tasks[i].Priority != as.tasks[j].Priority {
			return as.priorityValue(as.tasks[i].Priority) > as.priorityValue(as.tasks[j].Priority)
		}
		return as.tasks[i].ScheduledAt.Before(as.tasks[j].ScheduledAt)
	})

	// Add to queue if not running
	if !as.running {
		select {
		case as.taskQueue <- task:
		default:
			// Queue full
		}
	}
}

func (as *AnalysisScheduler) priorityValue(priority TaskPriority) int {
	switch priority {
	case TaskPriorityCritical:
		return 4
	case TaskPriorityHigh:
		return 3
	case TaskPriorityMedium:
		return 2
	case TaskPriorityLow:
		return 1
	default:
		return 0
	}
}

// Helper functions for configuration

func getDefaultHistoricalConfig() *HistoricalConfig {
	return &HistoricalConfig{
		AnalysisWindowDays:       30,
		TrendAnalysisWindow:      100,
		RegressionThreshold:      20.0,
		BaselineUpdateInterval:   24 * time.Hour,
		CacheSize:                1000,
		CacheTTL:                 1 * time.Hour,
		EnableBackgroundAnalysis: true,
		AnalysisInterval:         1 * time.Hour,
		AlertingEnabled:          true,
		AlertThresholds: &HistoricalAlertThresholds{
			PerformanceDegradation: 25.0,
			ErrorRateIncrease:      10.0,
			DurationIncrease:       30.0,
			ThroughputDecrease:     20.0,
			ConsecutiveFailures:    3,
		},
	}
}

func validateHistoricalConfig(config *HistoricalConfig) error {
	if config.AnalysisWindowDays <= 0 {
		return fmt.Errorf("analysis window days must be positive")
	}
	if config.TrendAnalysisWindow <= 0 {
		return fmt.Errorf("trend analysis window must be positive")
	}
	if config.RegressionThreshold <= 0 {
		return fmt.Errorf("regression threshold must be positive")
	}
	if config.CacheSize <= 0 {
		return fmt.Errorf("cache size must be positive")
	}
	return nil
}

// Baseline manager implementation

func (bm *BaselineManager) GetBaseline(testName, scenario, environment string) (*PerformanceBaseline, error) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	key := fmt.Sprintf("%s_%s_%s", testName, scenario, environment)
	baseline, exists := bm.baselines[key]
	if !exists {
		return nil, fmt.Errorf("baseline not found for %s", key)
	}

	// Check if baseline is too old
	if time.Since(baseline.UpdatedAt) > bm.config.MaxAge {
		return nil, fmt.Errorf("baseline is too old: %v", time.Since(baseline.UpdatedAt))
	}

	return baseline, nil
}

// Cleanup performs cleanup of the historical tracker
func (ht *HistoricalTracker) Cleanup() error {
	ht.mu.Lock()
	defer ht.mu.Unlock()

	ht.logger.Printf("Starting historical tracker cleanup")

	// Cancel context
	if ht.cancel != nil {
		ht.cancel()
	}

	// Execute cleanup functions
	var errors []error
	for _, cleanup := range ht.cleanupFunctions {
		if err := cleanup(); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("cleanup errors: %v", errors)
	}

	ht.logger.Printf("Historical tracker cleanup completed")
	return nil
}
