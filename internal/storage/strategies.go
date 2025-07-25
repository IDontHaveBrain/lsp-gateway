package storage

import (
	"context"
	"time"
)

// Strategy types and interfaces for intelligent cache management

// PromotionStrategy defines interface for intelligent data promotion between tiers
// Implementations determine when and how data should be moved to faster tiers
type PromotionStrategy interface {
	// Core promotion decision logic
	ShouldPromote(entry *CacheEntry, accessPattern *AccessPattern, currentTier TierType) (bool, TierType)
	CalculatePromotionPriority(entry *CacheEntry, accessPattern *AccessPattern) int
	
	// Batch promotion optimization for efficiency
	SelectPromotionCandidates(tier TierType, maxCandidates int) ([]*PromotionCandidate, error)
	OptimizePromotionBatch(candidates []*PromotionCandidate) ([]*PromotionCandidate, error)
	
	// Strategy configuration and introspection
	UpdateParameters(params PromotionParameters) error
	GetStrategyType() PromotionStrategyType
	GetCurrentParameters() PromotionParameters
	GetStrategyName() string
	GetStrategyDescription() string
	
	// Performance analysis
	EstimatePromotionBenefit(entry *CacheEntry, fromTier, toTier TierType) float64
	ValidatePromotion(entry *CacheEntry, fromTier, toTier TierType) error
}

// EvictionPolicy defines interface for intelligent data eviction from tiers
// Implementations determine what data should be evicted when tier capacity is reached
type EvictionPolicy interface {
	// Core eviction decision logic
	ShouldEvict(entry *CacheEntry, accessPattern *AccessPattern, currentTier TierType) (bool, TierType)
	CalculateEvictionPriority(entry *CacheEntry, accessPattern *AccessPattern) int
	
	// Batch eviction optimization for capacity management
	SelectEvictionCandidates(tier TierType, requiredSpace int64) ([]*EvictionCandidate, error)
	OptimizeEvictionBatch(candidates []*EvictionCandidate) ([]*EvictionCandidate, error)
	
	// Policy configuration and introspection
	UpdateParameters(params EvictionParameters) error
	GetPolicyType() EvictionPolicyType
	GetCurrentParameters() EvictionParameters
	GetPolicyName() string
	GetPolicyDescription() string
	
	// Impact analysis
	EstimateEvictionImpact(entry *CacheEntry, tier TierType) float64
	ValidateEviction(entry *CacheEntry, tier TierType) error
}

// AccessPattern defines interface for tracking and analyzing data access patterns
// Enables intelligent promotion/eviction decisions based on usage patterns
type AccessPattern interface {
	// Access tracking and recording
	RecordAccess(key string, accessType AccessType, metadata *AccessMetadata)
	GetAccessFrequency(key string, timeWindow time.Duration) float64
	GetAccessRecency(key string) time.Duration
	GetAccessLocality(key string) *LocalityInfo
	
	// Pattern analysis and prediction
	AnalyzePattern(key string) *PatternAnalysis
	PredictFutureAccess(key string, horizon time.Duration) float64
	ClassifyAccessPattern(key string) AccessPatternType
	GetAccessTrend(key string, timeWindow time.Duration) AccessTrend
	
	// Bulk pattern operations
	GetTopAccessedKeys(tier TierType, limit int) ([]string, error)
	GetColdKeys(tier TierType, threshold time.Duration) ([]string, error)
	GetHotKeys(tier TierType, threshold float64) ([]string, error)
	GetRelatedKeys(key string, maxResults int) ([]string, error)
	
	// Pattern maintenance and configuration
	UpdateTrackingParameters(params AccessTrackingParameters) error
	PurgeOldPatterns(cutoff time.Time) error
	GetPatternStats() *AccessPatternStats
	
	// Context-aware analysis
	AnalyzeTemporalPattern(key string) *TemporalPattern
	AnalyzeSpatialPattern(key string) *SpatialPattern
	AnalyzeSemanticPattern(key string) *SemanticPattern
}

// CacheCoordinator manages intelligent coordination between tiers
// Orchestrates promotion, eviction, and rebalancing across the three-tier system
type CacheCoordinator interface {
	// Tier coordination
	CoordinateAccess(ctx context.Context, key string, preferredTier TierType) (*CacheEntry, TierType, error)
	CoordinateStorage(ctx context.Context, key string, entry *CacheEntry, hint CachingHint) error
	CoordinateEviction(ctx context.Context, tier TierType, requiredSpace int64) (*EvictionResult, error)
	
	// System-wide operations
	RebalanceSystem(ctx context.Context) (*RebalanceResult, error)
	OptimizeSystem(ctx context.Context) (*OptimizationResult, error)
	PerformMaintenance(ctx context.Context) (*MaintenanceResult, error)
	
	// Policy management
	SetPromotionStrategy(strategy PromotionStrategy) error
	SetEvictionPolicy(policy EvictionPolicy) error
	SetAccessTracker(tracker AccessPattern) error
	
	// Monitoring and introspection
	GetCoordinatorStats() *CoordinatorStats
	GetCoordinatorHealth() *CoordinatorHealth
	GetRecommendations() ([]*Recommendation, error)
}

// Strategy types and enums

// PromotionStrategyType represents different promotion strategy implementations
type PromotionStrategyType int

const (
	PromotionStrategyLFU PromotionStrategyType = iota // Least Frequently Used
	PromotionStrategyLRU                              // Least Recently Used
	PromotionStrategyAdaptive                         // Adaptive based on patterns
	PromotionStrategyPredictive                       // ML-based predictive
	PromotionStrategyHybrid                           // Combination of strategies
	PromotionStrategyCustom                           // Custom implementation
)

func (p PromotionStrategyType) String() string {
	switch p {
	case PromotionStrategyLFU:
		return "LFU"
	case PromotionStrategyLRU:
		return "LRU"
	case PromotionStrategyAdaptive:
		return "Adaptive"
	case PromotionStrategyPredictive:
		return "Predictive"
	case PromotionStrategyHybrid:
		return "Hybrid"
	case PromotionStrategyCustom:
		return "Custom"
	default:
		return "Unknown"
	}
}

// EvictionPolicyType represents different eviction policy implementations
type EvictionPolicyType int

const (
	EvictionPolicyLRU EvictionPolicyType = iota // Least Recently Used
	EvictionPolicyLFU                           // Least Frequently Used
	EvictionPolicyTTL                           // Time To Live based
	EvictionPolicySize                          // Size-based eviction
	EvictionPolicyAdaptive                      // Adaptive based on patterns
	EvictionPolicyPredictive                    // ML-based predictive
	EvictionPolicyCustom                        // Custom implementation
)

func (e EvictionPolicyType) String() string {
	switch e {
	case EvictionPolicyLRU:
		return "LRU"
	case EvictionPolicyLFU:
		return "LFU"
	case EvictionPolicyTTL:
		return "TTL"
	case EvictionPolicySize:
		return "Size"
	case EvictionPolicyAdaptive:
		return "Adaptive"
	case EvictionPolicyPredictive:
		return "Predictive"
	case EvictionPolicyCustom:
		return "Custom"
	default:
		return "Unknown"
	}
}

// AccessType represents different types of data access
type AccessType int

const (
	AccessTypeRead AccessType = iota
	AccessTypeWrite
	AccessTypeUpdate
	AccessTypeDelete
	AccessTypeStream
	AccessTypeBatch
)

func (a AccessType) String() string {
	switch a {
	case AccessTypeRead:
		return "Read"
	case AccessTypeWrite:
		return "Write"
	case AccessTypeUpdate:
		return "Update"
	case AccessTypeDelete:
		return "Delete"
	case AccessTypeStream:
		return "Stream"
	case AccessTypeBatch:
		return "Batch"
	default:
		return "Unknown"
	}
}

// AccessPatternType represents different access pattern classifications
type AccessPatternType int

const (
	AccessPatternSequential AccessPatternType = iota
	AccessPatternRandom
	AccessPatternHot
	AccessPatternCold
	AccessPatternBursty
	AccessPatternSteady
	AccessPatternSeasonal
	AccessPatternTrending
)

func (a AccessPatternType) String() string {
	switch a {
	case AccessPatternSequential:
		return "Sequential"
	case AccessPatternRandom:
		return "Random"
	case AccessPatternHot:
		return "Hot"
	case AccessPatternCold:
		return "Cold"
	case AccessPatternBursty:
		return "Bursty"
	case AccessPatternSteady:
		return "Steady"
	case AccessPatternSeasonal:
		return "Seasonal"
	case AccessPatternTrending:
		return "Trending"
	default:
		return "Unknown"
	}
}

// AccessTrend represents the trend direction of access patterns
type AccessTrend int

const (
	AccessTrendUnknown AccessTrend = iota
	AccessTrendIncreasing
	AccessTrendDecreasing
	AccessTrendStable
	AccessTrendVolatile
)

func (a AccessTrend) String() string {
	switch a {
	case AccessTrendIncreasing:
		return "Increasing"
	case AccessTrendDecreasing:
		return "Decreasing"
	case AccessTrendStable:
		return "Stable"
	case AccessTrendVolatile:
		return "Volatile"
	default:
		return "Unknown"
	}
}

// Data structures for promotion and eviction

// PromotionCandidate represents a candidate for promotion to a higher tier
type PromotionCandidate struct {
	Key             string        `json:"key"`
	Entry           *CacheEntry   `json:"entry"`
	FromTier        TierType      `json:"from_tier"`
	ToTier          TierType      `json:"to_tier"`
	Priority        int           `json:"priority"`
	ExpectedBenefit float64       `json:"expected_benefit"`
	Cost            int64         `json:"cost"`
	Reason          string        `json:"reason"`
	AccessPattern   *PatternAnalysis `json:"access_pattern,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// EvictionCandidate represents a candidate for eviction from a tier
type EvictionCandidate struct {
	Key           string        `json:"key"`
	Entry         *CacheEntry   `json:"entry"`
	FromTier      TierType      `json:"from_tier"`
	ToTier        *TierType     `json:"to_tier,omitempty"` // nil for complete eviction
	Priority      int           `json:"priority"`
	ExpectedImpact float64      `json:"expected_impact"`
	SpaceReclaimed int64        `json:"space_reclaimed"`
	Reason        EvictionReason `json:"reason"`
	AccessPattern *PatternAnalysis `json:"access_pattern,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// AccessMetadata contains metadata about a data access event
type AccessMetadata struct {
	Timestamp     time.Time              `json:"timestamp"`
	Duration      time.Duration          `json:"duration"`
	Size          int64                  `json:"size"`
	Source        string                 `json:"source,omitempty"`
	UserAgent     string                 `json:"user_agent,omitempty"`
	RequestID     string                 `json:"request_id,omitempty"`
	SessionID     string                 `json:"session_id,omitempty"`
	ProjectPath   string                 `json:"project_path,omitempty"`
	FilePath      string                 `json:"file_path,omitempty"`
	Method        string                 `json:"method,omitempty"`
	Success       bool                   `json:"success"`
	ErrorCode     string                 `json:"error_code,omitempty"`
	CustomData    map[string]interface{} `json:"custom_data,omitempty"`
}

// PatternAnalysis contains analysis results of access patterns
type PatternAnalysis struct {
	Key                string            `json:"key"`
	PatternType        AccessPatternType `json:"pattern_type"`
	Confidence         float64           `json:"confidence"`
	AccessFrequency    float64           `json:"access_frequency"`
	AccessRecency      time.Duration     `json:"access_recency"`
	AccessTrend        AccessTrend       `json:"access_trend"`
	Seasonality        *SeasonalityInfo  `json:"seasonality,omitempty"`
	Predictability     float64           `json:"predictability"`
	Volatility         float64           `json:"volatility"`
	TemporalPattern    *TemporalPattern  `json:"temporal_pattern,omitempty"`
	SpatialPattern     *SpatialPattern   `json:"spatial_pattern,omitempty"`
	SemanticPattern    *SemanticPattern  `json:"semantic_pattern,omitempty"`
	RelatedKeys        []string          `json:"related_keys,omitempty"`
	Score              float64           `json:"score"`
	LastAnalyzed       time.Time         `json:"last_analyzed"`
}

// TemporalPattern contains time-based access pattern information
type TemporalPattern struct {
	HourlyDistribution  map[int]float64    `json:"hourly_distribution"`
	DailyDistribution   map[time.Weekday]float64 `json:"daily_distribution"`
	WeeklyDistribution  map[int]float64    `json:"weekly_distribution"`
	MonthlyDistribution map[time.Month]float64 `json:"monthly_distribution"`
	PeakHours          []int              `json:"peak_hours"`
	QuietHours         []int              `json:"quiet_hours"`
	Seasonality        *SeasonalityInfo   `json:"seasonality,omitempty"`
	Trend              AccessTrend        `json:"trend"`
	Cyclical           bool               `json:"cyclical"`
}

// SpatialPattern contains location-based access pattern information
type SpatialPattern struct {
	ProjectDistribution map[string]float64 `json:"project_distribution"`
	FileDistribution    map[string]float64 `json:"file_distribution"`
	DirectoryDistribution map[string]float64 `json:"directory_distribution"`
	LanguageDistribution map[string]float64 `json:"language_distribution"`
	GeographicDistribution map[string]float64 `json:"geographic_distribution,omitempty"`
	Locality           *LocalityInfo       `json:"locality,omitempty"`
	Clustering         float64             `json:"clustering"`
	Dispersion         float64             `json:"dispersion"`
}

// SemanticPattern contains semantic/contextual access pattern information
type SemanticPattern struct {
	MethodDistribution  map[string]float64 `json:"method_distribution"`
	ParameterPatterns   map[string]float64 `json:"parameter_patterns"`
	ResponsePatterns    map[string]float64 `json:"response_patterns"`
	ErrorPatterns       map[string]float64 `json:"error_patterns"`
	ContextSimilarity   float64            `json:"context_similarity"`
	SemanticClusters    []string           `json:"semantic_clusters"`
	ConceptualRelatedness float64          `json:"conceptual_relatedness"`
}

// SeasonalityInfo contains information about seasonal access patterns
type SeasonalityInfo struct {
	HasSeasonality bool              `json:"has_seasonality"`
	SeasonalPeriod time.Duration     `json:"seasonal_period"`
	SeasonalStrength float64         `json:"seasonal_strength"`
	SeasonalPhase  time.Duration     `json:"seasonal_phase"`
	Cycles         []SeasonalCycle   `json:"cycles,omitempty"`
}

// SeasonalCycle represents a detected seasonal cycle
type SeasonalCycle struct {
	Period    time.Duration `json:"period"`
	Amplitude float64       `json:"amplitude"`
	Phase     time.Duration `json:"phase"`
	Strength  float64       `json:"strength"`
}

// Configuration parameters

// PromotionParameters contains configuration for promotion strategies
type PromotionParameters struct {
	// General parameters
	Enabled                bool                   `json:"enabled"`
	MinAccessCount         int64                  `json:"min_access_count"`
	MinAccessFrequency     float64                `json:"min_access_frequency"`
	AccessTimeWindow       time.Duration          `json:"access_time_window"`
	PromotionCooldown      time.Duration          `json:"promotion_cooldown"`
	
	// LRU/LFU parameters
	RecencyWeight          float64                `json:"recency_weight"`
	FrequencyWeight        float64                `json:"frequency_weight"`
	SizeWeight             float64                `json:"size_weight"`
	
	// Predictive parameters
	PredictionHorizon      time.Duration          `json:"prediction_horizon"`
	PredictionConfidence   float64                `json:"prediction_confidence"`
	ModelUpdateInterval    time.Duration          `json:"model_update_interval"`
	
	// Adaptive parameters
	LearningRate           float64                `json:"learning_rate"`
	AdaptationInterval     time.Duration          `json:"adaptation_interval"`
	PerformanceThreshold   float64                `json:"performance_threshold"`
	
	// Tier-specific parameters
	TierThresholds         map[TierType]float64   `json:"tier_thresholds"`
	TierWeights            map[TierType]float64   `json:"tier_weights"`
	TierCapacityLimits     map[TierType]float64   `json:"tier_capacity_limits"`
	
	// Batch optimization parameters
	BatchSize              int                    `json:"batch_size"`
	BatchTimeWindow        time.Duration          `json:"batch_time_window"`
	MaxBatchProcessingTime time.Duration          `json:"max_batch_processing_time"`
	
	// Custom parameters
	CustomParameters       map[string]interface{} `json:"custom_parameters,omitempty"`
}

// EvictionParameters contains configuration for eviction policies
type EvictionParameters struct {
	// General parameters
	Enabled                bool                   `json:"enabled"`
	EvictionThreshold      float64                `json:"eviction_threshold"`
	TargetUtilization      float64                `json:"target_utilization"`
	EvictionBatchSize      int                    `json:"eviction_batch_size"`
	EvictionCooldown       time.Duration          `json:"eviction_cooldown"`
	
	// LRU/LFU parameters
	AccessAgeThreshold     time.Duration          `json:"access_age_threshold"`
	AccessCountThreshold   int64                  `json:"access_count_threshold"`
	InactivityThreshold    time.Duration          `json:"inactivity_threshold"`
	
	// TTL parameters
	DefaultTTL             time.Duration          `json:"default_ttl"`
	MaxTTL                 time.Duration          `json:"max_ttl"`
	TTLExtensionFactor     float64                `json:"ttl_extension_factor"`
	
	// Size-based parameters
	MaxItemSize            int64                  `json:"max_item_size"`
	SizeWeight             float64                `json:"size_weight"`
	CompressionThreshold   int64                  `json:"compression_threshold"`
	
	// Adaptive parameters
	LearningRate           float64                `json:"learning_rate"`
	AdaptationInterval     time.Duration          `json:"adaptation_interval"`
	PerformanceThreshold   float64                `json:"performance_threshold"`
	
	// Tier-specific parameters
	TierEvictionRates      map[TierType]float64   `json:"tier_eviction_rates"`
	TierRetentionPolicies  map[TierType]string    `json:"tier_retention_policies"`
	
	// Protection parameters
	PinnedItemProtection   bool                   `json:"pinned_item_protection"`
	RecentItemProtection   time.Duration          `json:"recent_item_protection"`
	HighValueProtection    float64                `json:"high_value_protection"`
	
	// Custom parameters
	CustomParameters       map[string]interface{} `json:"custom_parameters,omitempty"`
}

// AccessTrackingParameters contains configuration for access pattern tracking
type AccessTrackingParameters struct {
	// General tracking parameters
	Enabled                bool          `json:"enabled"`
	TrackingGranularity    time.Duration `json:"tracking_granularity"`
	HistoryRetention       time.Duration `json:"history_retention"`
	MaxTrackedKeys         int           `json:"max_tracked_keys"`
	
	// Pattern analysis parameters
	AnalysisInterval       time.Duration `json:"analysis_interval"`
	MinSampleSize          int           `json:"min_sample_size"`
	ConfidenceThreshold    float64       `json:"confidence_threshold"`
	PredictionAccuracy     float64       `json:"prediction_accuracy"`
	
	// Temporal analysis parameters
	TemporalResolution     time.Duration `json:"temporal_resolution"`
	SeasonalityDetection   bool          `json:"seasonality_detection"`
	TrendDetection         bool          `json:"trend_detection"`
	CyclicityDetection     bool          `json:"cyclicity_detection"`
	
	// Spatial analysis parameters
	LocalityTracking       bool          `json:"locality_tracking"`
	GeographicTracking     bool          `json:"geographic_tracking"`
	ProjectAwareTracking   bool          `json:"project_aware_tracking"`
	
	// Semantic analysis parameters
	SemanticAnalysis       bool          `json:"semantic_analysis"`
	ContextAnalysis        bool          `json:"context_analysis"`
	RelationshipDetection  bool          `json:"relationship_detection"`
	
	// Performance parameters
	AsyncProcessing        bool          `json:"async_processing"`
	BatchProcessing        bool          `json:"batch_processing"`
	CompressionEnabled     bool          `json:"compression_enabled"`
	
	// Custom parameters
	CustomParameters       map[string]interface{} `json:"custom_parameters,omitempty"`
}

// Result types for strategy operations

// EvictionResult contains results of eviction operations
type EvictionResult struct {
	ItemsEvicted       int                    `json:"items_evicted"`
	SpaceReclaimed     int64                  `json:"space_reclaimed"`
	Duration           time.Duration          `json:"duration"`
	TierResults        map[TierType]int       `json:"tier_results"`
	EvictedItems       []*EvictionCandidate   `json:"evicted_items,omitempty"`
	Errors             []error                `json:"errors,omitempty"`
	PerformanceImpact  float64                `json:"performance_impact"`
}

// CoordinatorStats contains statistics about cache coordination
type CoordinatorStats struct {
	TotalRequests       int64                  `json:"total_requests"`
	TierHitRates        map[TierType]float64   `json:"tier_hit_rates"`
	PromotionStats      *PromotionStats        `json:"promotion_stats"`
	EvictionStats       *EvictionStats         `json:"eviction_stats"`
	RebalanceStats      *RebalanceStats        `json:"rebalance_stats"`
	OptimizationStats   *OptimizationStats     `json:"optimization_stats"`
	MaintenanceStats    *MaintenanceStats      `json:"maintenance_stats"`
	ErrorStats          *ErrorStats            `json:"error_stats"`
	PerformanceMetrics  *PerformanceMetrics    `json:"performance_metrics"`
}

// CoordinatorHealth contains health information about cache coordination
type CoordinatorHealth struct {
	Healthy             bool                   `json:"healthy"`
	OverallScore        float64                `json:"overall_score"`
	TierHealth          map[TierType]float64   `json:"tier_health"`
	StrategyHealth      map[string]float64     `json:"strategy_health"`
	Issues              []HealthIssue          `json:"issues,omitempty"`
	LastCheck           time.Time              `json:"last_check"`
	Recommendations     []*Recommendation      `json:"recommendations,omitempty"`
}

// Recommendation contains optimization recommendations
type Recommendation struct {
	Type           RecommendationType     `json:"type"`
	Priority       RecommendationPriority `json:"priority"`
	Title          string                 `json:"title"`
	Description    string                 `json:"description"`
	ExpectedImpact float64                `json:"expected_impact"`
	EstimatedCost  float64                `json:"estimated_cost"`
	Actions        []RecommendationAction `json:"actions"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
	ValidUntil     *time.Time             `json:"valid_until,omitempty"`
}

// RecommendationType represents different types of recommendations
type RecommendationType int

const (
	RecommendationTypePerformance RecommendationType = iota
	RecommendationTypeCapacity
	RecommendationTypeConfiguration
	RecommendationTypeMaintenance
	RecommendationTypeSecurity
	RecommendationTypeOptimization
)

func (r RecommendationType) String() string {
	switch r {
	case RecommendationTypePerformance:
		return "Performance"
	case RecommendationTypeCapacity:
		return "Capacity"
	case RecommendationTypeConfiguration:
		return "Configuration"
	case RecommendationTypeMaintenance:
		return "Maintenance"
	case RecommendationTypeSecurity:
		return "Security"
	case RecommendationTypeOptimization:
		return "Optimization"
	default:
		return "Unknown"
	}
}

// RecommendationPriority represents the priority of recommendations
type RecommendationPriority int

const (
	RecommendationPriorityLow RecommendationPriority = iota
	RecommendationPriorityMedium
	RecommendationPriorityHigh
	RecommendationPriorityCritical
)

func (r RecommendationPriority) String() string {
	switch r {
	case RecommendationPriorityLow:
		return "Low"
	case RecommendationPriorityMedium:
		return "Medium"
	case RecommendationPriorityHigh:
		return "High"
	case RecommendationPriorityCritical:
		return "Critical"
	default:
		return "Unknown"
	}
}

// RecommendationAction represents an actionable recommendation
type RecommendationAction struct {
	Type        string                 `json:"type"`
	Description string                 `json:"description"`
	Command     string                 `json:"command,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	Automated   bool                   `json:"automated"`
	EstimatedDuration time.Duration    `json:"estimated_duration"`
}

// AccessPatternStats contains statistics about access patterns
type AccessPatternStats struct {
	TotalTrackedKeys    int                           `json:"total_tracked_keys"`
	ActivePatterns      int                           `json:"active_patterns"`
	PatternDistribution map[AccessPatternType]int     `json:"pattern_distribution"`
	TrendDistribution   map[AccessTrend]int           `json:"trend_distribution"`
	AnalysisAccuracy    float64                       `json:"analysis_accuracy"`
	PredictionAccuracy  float64                       `json:"prediction_accuracy"`
	LastAnalysis        time.Time                     `json:"last_analysis"`
	AnalysisPerformance *AnalysisPerformanceMetrics   `json:"analysis_performance"`
}

// AnalysisPerformanceMetrics contains performance metrics for pattern analysis
type AnalysisPerformanceMetrics struct {
	AvgAnalysisTime     time.Duration `json:"avg_analysis_time"`
	MaxAnalysisTime     time.Duration `json:"max_analysis_time"`
	TotalAnalyses       int64         `json:"total_analyses"`
	SuccessfulAnalyses  int64         `json:"successful_analyses"`
	FailedAnalyses      int64         `json:"failed_analyses"`
	AnalysisRate        float64       `json:"analysis_rate"`
	MemoryUsage         int64         `json:"memory_usage"`
}

// Additional statistics types for comprehensive monitoring

// RebalanceStats contains statistics about system rebalancing
type RebalanceStats struct {
	TotalRebalances     int64         `json:"total_rebalances"`
	SuccessfulRebalances int64        `json:"successful_rebalances"`
	FailedRebalances    int64         `json:"failed_rebalances"`
	ItemsRebalanced     int64         `json:"items_rebalanced"`
	BytesRebalanced     int64         `json:"bytes_rebalanced"`
	AvgRebalanceTime    time.Duration `json:"avg_rebalance_time"`
	LastRebalance       time.Time     `json:"last_rebalance"`
	PerformanceGain     float64       `json:"performance_gain"`
}

// OptimizationStats contains statistics about system optimization
type OptimizationStats struct {
	TotalOptimizations  int64         `json:"total_optimizations"`
	SpaceSaved          int64         `json:"space_saved"`
	ItemsOptimized      int64         `json:"items_optimized"`
	CompressionGain     float64       `json:"compression_gain"`
	AvgOptimizationTime time.Duration `json:"avg_optimization_time"`
	LastOptimization    time.Time     `json:"last_optimization"`
	PerformanceGain     float64       `json:"performance_gain"`
}

// MaintenanceStats contains statistics about system maintenance
type MaintenanceStats struct {
	TotalMaintenanceRuns int64                    `json:"total_maintenance_runs"`
	TasksCompleted       map[string]int64         `json:"tasks_completed"`
	SpaceReclaimed       int64                    `json:"space_reclaimed"`
	AvgMaintenanceTime   time.Duration            `json:"avg_maintenance_time"`
	LastMaintenance      time.Time                `json:"last_maintenance"`
	MaintenanceSchedule  *MaintenanceSchedule     `json:"maintenance_schedule,omitempty"`
}

// MaintenanceSchedule defines when maintenance tasks should run
type MaintenanceSchedule struct {
	Enabled             bool                      `json:"enabled"`
	Schedule            string                    `json:"schedule"` // Cron expression
	Tasks               []MaintenanceTask         `json:"tasks"`
	NextRun             time.Time                 `json:"next_run"`
	LastRun             time.Time                 `json:"last_run"`
	MaintenanceWindow   *MaintenanceWindow        `json:"maintenance_window,omitempty"`
}

// MaintenanceTask represents a scheduled maintenance task
type MaintenanceTask struct {
	Name        string                 `json:"name"`
	Type        MaintenanceEventType   `json:"type"`
	Enabled     bool                   `json:"enabled"`
	Priority    int                    `json:"priority"`
	Frequency   time.Duration          `json:"frequency"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// MaintenanceWindow defines when maintenance can be performed
type MaintenanceWindow struct {
	StartTime   string        `json:"start_time"` // HH:MM format
	EndTime     string        `json:"end_time"`   // HH:MM format
	Timezone    string        `json:"timezone"`
	Days        []time.Weekday `json:"days"`
	Blackouts   []BlackoutPeriod `json:"blackouts,omitempty"`
}

// BlackoutPeriod defines when maintenance should not be performed
type BlackoutPeriod struct {
	Start       time.Time `json:"start"`
	End         time.Time `json:"end"`
	Description string    `json:"description"`
	Recurring   bool      `json:"recurring"`
}

// ErrorStats contains error-related statistics
type ErrorStats struct {
	TotalErrors         int64                    `json:"total_errors"`
	ErrorsByType        map[string]int64         `json:"errors_by_type"`
	ErrorsByTier        map[TierType]int64       `json:"errors_by_tier"`
	ErrorRate           float64                  `json:"error_rate"`
	RecoveredErrors     int64                    `json:"recovered_errors"`
	FatalErrors         int64                    `json:"fatal_errors"`
	LastError           time.Time                `json:"last_error"`
	ErrorTrend          AccessTrend              `json:"error_trend"`
}

// PerformanceMetrics contains performance-related metrics
type PerformanceMetrics struct {
	ResponseTime        *LatencyMetrics          `json:"response_time"`
	Throughput          *ThroughputMetrics       `json:"throughput"`
	ResourceUtilization *ResourceUtilization     `json:"resource_utilization"`
	BottleneckAnalysis  *BottleneckAnalysis      `json:"bottleneck_analysis"`
	TrendAnalysis       *TrendAnalysis           `json:"trend_analysis"`
}

// ResourceUtilization contains resource usage metrics
type ResourceUtilization struct {
	CPUUsage      float64 `json:"cpu_usage"`
	MemoryUsage   float64 `json:"memory_usage"`
	DiskUsage     float64 `json:"disk_usage"`
	NetworkUsage  float64 `json:"network_usage"`
	ThreadUsage   float64 `json:"thread_usage"`
	FileHandles   int     `json:"file_handles"`
}

// BottleneckAnalysis identifies system bottlenecks
type BottleneckAnalysis struct {
	PrimaryBottleneck    string                 `json:"primary_bottleneck"`
	BottleneckSeverity   float64                `json:"bottleneck_severity"`
	BottleneckComponents []BottleneckComponent  `json:"bottleneck_components"`
	RecommendedActions   []string               `json:"recommended_actions"`
	ImpactAssessment     *ImpactAssessment      `json:"impact_assessment"`
}

// BottleneckComponent represents a component contributing to bottlenecks
type BottleneckComponent struct {
	Component   string  `json:"component"`
	Contribution float64 `json:"contribution"`
	Severity    float64 `json:"severity"`
	Description string  `json:"description"`
}

// ImpactAssessment evaluates the impact of issues or changes
type ImpactAssessment struct {
	PerformanceImpact   float64 `json:"performance_impact"`
	CapacityImpact      float64 `json:"capacity_impact"`
	ReliabilityImpact   float64 `json:"reliability_impact"`
	CostImpact          float64 `json:"cost_impact"`
	UserExperienceImpact float64 `json:"user_experience_impact"`
	OverallImpact       float64 `json:"overall_impact"`
}

// TrendAnalysis analyzes performance trends over time
type TrendAnalysis struct {
	PerformanceTrend    AccessTrend      `json:"performance_trend"`
	CapacityTrend       AccessTrend      `json:"capacity_trend"`
	ErrorTrend          AccessTrend      `json:"error_trend"`
	TrendConfidence     float64          `json:"trend_confidence"`
	PredictedDegradation *time.Time      `json:"predicted_degradation,omitempty"`
	TrendDetails        *TrendDetails    `json:"trend_details,omitempty"`
}

// TrendDetails provides detailed trend information
type TrendDetails struct {
	Slope           float64       `json:"slope"`
	Correlation     float64       `json:"correlation"`
	Seasonality     float64       `json:"seasonality"`
	Volatility      float64       `json:"volatility"`
	ChangePoints    []time.Time   `json:"change_points,omitempty"`
	ForecastHorizon time.Duration `json:"forecast_horizon"`
}