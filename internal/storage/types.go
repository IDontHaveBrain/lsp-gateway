package storage

import (
	"encoding/json"
	"time"

	"lsp-gateway/internal/indexing"
)

// Core storage types and enums

// TierType represents the three storage tiers in the architecture
type TierType int

const (
	TierL1Memory TierType = iota + 1 // L1: In-memory cache (<10ms access)
	TierL2Disk                       // L2: SSD/HDD storage (<50ms access)
	TierL3Remote                     // L3: Remote/network storage (<200ms access)
)

func (t TierType) String() string {
	switch t {
	case TierL1Memory:
		return "L1_Memory"
	case TierL2Disk:
		return "L2_Disk"
	case TierL3Remote:
		return "L3_Remote"
	default:
		return "Unknown"
	}
}

// BackendType represents different storage backend implementations
type BackendType int

const (
	BackendMemory BackendType = iota + 1
	BackendLocalDisk
	BackendNetworkDisk
	BackendS3
	BackendRedis
	BackendDatabase
	BackendCustom
)

func (b BackendType) String() string {
	switch b {
	case BackendMemory:
		return "Memory"
	case BackendLocalDisk:
		return "LocalDisk"
	case BackendNetworkDisk:
		return "NetworkDisk"
	case BackendS3:
		return "S3"
	case BackendRedis:
		return "Redis"
	case BackendDatabase:
		return "Database"
	case BackendCustom:
		return "Custom"
	default:
		return "Unknown"
	}
}

// CompressionType represents different compression algorithms
type CompressionType int

const (
	CompressionNone CompressionType = iota
	CompressionGzip
	CompressionLZ4
	CompressionZstd
	CompressionSnappy
)

func (c CompressionType) String() string {
	switch c {
	case CompressionNone:
		return "None"
	case CompressionGzip:
		return "Gzip"
	case CompressionLZ4:
		return "LZ4"
	case CompressionZstd:
		return "Zstd"
	case CompressionSnappy:
		return "Snappy"
	default:
		return "Unknown"
	}
}

// EncryptionAlgorithm represents different encryption algorithms
type EncryptionAlgorithm int

const (
	EncryptionNone EncryptionAlgorithm = iota
	EncryptionAES256
	EncryptionChaCha20
	EncryptionAESGCM
)

func (e EncryptionAlgorithm) String() string {
	switch e {
	case EncryptionNone:
		return "None"
	case EncryptionAES256:
		return "AES256"
	case EncryptionChaCha20:
		return "ChaCha20"
	case EncryptionAESGCM:
		return "AES-GCM"
	default:
		return "Unknown"
	}
}

// Core data structures

// CacheEntry represents a comprehensive cache entry with multi-tier support
// Extends the existing SCIPCacheEntry with additional metadata for tier management
type CacheEntry struct {
	// Core data (compatible with existing indexing.SCIPCacheEntry)
	Method     string          `json:"method"`
	Params     string          `json:"params"`
	Response   json.RawMessage `json:"response"`
	CreatedAt  time.Time       `json:"created_at"`
	AccessedAt time.Time       `json:"accessed_at"`
	TTL        time.Duration   `json:"ttl"`

	// Enhanced metadata for three-tier storage
	Key           string                 `json:"key"`
	Size          int64                  `json:"size"`
	CompressedSize int64                 `json:"compressed_size,omitempty"`
	Version       int64                  `json:"version"`
	Checksum      string                 `json:"checksum"`
	ContentType   string                 `json:"content_type"`
	Encoding      string                 `json:"encoding,omitempty"`
	
	// Tier management
	CurrentTier   TierType               `json:"current_tier"`
	OriginTier    TierType               `json:"origin_tier"`
	TierHistory   []TierTransition       `json:"tier_history,omitempty"`
	Priority      int                    `json:"priority"`
	Pinned        bool                   `json:"pinned"`
	
	// Access patterns and statistics
	AccessCount   int64                  `json:"access_count"`
	HitCount      int64                  `json:"hit_count"`
	LastHitTime   time.Time              `json:"last_hit_time"`
	AvgAccessTime time.Duration          `json:"avg_access_time"`
	Locality      *LocalityInfo          `json:"locality,omitempty"`
	
	// File associations for invalidation
	FilePaths     []string               `json:"file_paths,omitempty"`
	ProjectPath   string                 `json:"project_path,omitempty"`
	Dependencies  []string               `json:"dependencies,omitempty"`
	Tags          map[string]string      `json:"tags,omitempty"`
	
	// Performance optimization hints
	CompressionHint CompressionType      `json:"compression_hint,omitempty"`
	CachingHint     CachingHint          `json:"caching_hint,omitempty"`
	PreloadHint     bool                 `json:"preload_hint,omitempty"`
	
	// Metadata for advanced features
	CustomMetadata map[string]interface{} `json:"custom_metadata,omitempty"`
}

// IsExpired checks if the cache entry has expired based on TTL
func (e *CacheEntry) IsExpired() bool {
	if e.TTL <= 0 {
		return false // No expiration
	}
	return time.Since(e.CreatedAt) > e.TTL
}

// Touch updates access metadata
func (e *CacheEntry) Touch() {
	now := time.Now()
	e.AccessedAt = now
	e.LastHitTime = now
	e.AccessCount++
	e.HitCount++
}

// GetCompressionRatio returns the compression ratio if compressed
func (e *CacheEntry) GetCompressionRatio() float64 {
	if e.CompressedSize > 0 && e.Size > 0 {
		return float64(e.CompressedSize) / float64(e.Size)
	}
	return 1.0
}

// ToSCIPCacheEntry converts to legacy indexing.SCIPCacheEntry for compatibility
func (e *CacheEntry) ToSCIPCacheEntry() *indexing.SCIPCacheEntry {
	return &indexing.SCIPCacheEntry{
		Method:     e.Method,
		Params:     e.Params,
		Response:   e.Response,
		CreatedAt:  e.CreatedAt,
		AccessedAt: e.AccessedAt,
		TTL:        e.TTL,
	}
}

// StorageMetadata contains metadata about stored data
type StorageMetadata struct {
	Key           string                 `json:"key"`
	Size          int64                  `json:"size"`
	Checksum      string                 `json:"checksum"`
	ContentType   string                 `json:"content_type"`
	Encoding      string                 `json:"encoding,omitempty"`
	Compressed    bool                   `json:"compressed"`
	Encrypted     bool                   `json:"encrypted"`
	Version       int64                  `json:"version"`
	CreatedAt     time.Time              `json:"created_at"`
	ModifiedAt    time.Time              `json:"modified_at"`
	AccessedAt    time.Time              `json:"accessed_at"`
	ExpiresAt     *time.Time             `json:"expires_at,omitempty"`
	Tags          map[string]string      `json:"tags,omitempty"`
	CustomData    map[string]interface{} `json:"custom_data,omitempty"`
}

// TierTransition records when data moves between tiers
type TierTransition struct {
	FromTier    TierType      `json:"from_tier"`
	ToTier      TierType      `json:"to_tier"`
	Timestamp   time.Time     `json:"timestamp"`
	Reason      string        `json:"reason"`
	Latency     time.Duration `json:"latency"`
	Success     bool          `json:"success"`
	Error       string        `json:"error,omitempty"`
}

// Statistics and monitoring types

// TierStats provides comprehensive statistics for a storage tier
type TierStats struct {
	TierType      TierType      `json:"tier_type"`
	
	// Capacity metrics
	TotalCapacity int64         `json:"total_capacity"`
	UsedCapacity  int64         `json:"used_capacity"`
	FreeCapacity  int64         `json:"free_capacity"`
	EntryCount    int64         `json:"entry_count"`
	
	// Performance metrics
	TotalRequests int64         `json:"total_requests"`
	CacheHits     int64         `json:"cache_hits"`
	CacheMisses   int64         `json:"cache_misses"`
	HitRate       float64       `json:"hit_rate"`
	
	// Latency metrics (microseconds for precision)
	AvgLatency    time.Duration `json:"avg_latency"`
	P50Latency    time.Duration `json:"p50_latency"`
	P95Latency    time.Duration `json:"p95_latency"`
	P99Latency    time.Duration `json:"p99_latency"`
	MaxLatency    time.Duration `json:"max_latency"`
	
	// Throughput metrics
	RequestsPerSecond float64   `json:"requests_per_second"`
	BytesPerSecond    float64   `json:"bytes_per_second"`
	
	// Error metrics
	ErrorCount        int64     `json:"error_count"`
	ErrorRate         float64   `json:"error_rate"`
	TimeoutCount      int64     `json:"timeout_count"`
	
	// Tier-specific operations
	PromotionCount    int64     `json:"promotion_count"`
	EvictionCount     int64     `json:"eviction_count"`
	CompactionCount   int64     `json:"compaction_count"`
	
	// Time tracking
	StartTime         time.Time `json:"start_time"`
	LastUpdate        time.Time `json:"last_update"`
	Uptime            time.Duration `json:"uptime"`
}

// TierHealth represents the health status of a storage tier
type TierHealth struct {
	TierType      TierType               `json:"tier_type"`
	Healthy       bool                   `json:"healthy"`
	Status        TierStatus             `json:"status"`
	LastCheck     time.Time              `json:"last_check"`
	Issues        []HealthIssue          `json:"issues,omitempty"`
	Metrics       map[string]float64     `json:"metrics,omitempty"`
	
	// Circuit breaker state
	CircuitBreaker struct {
		State        CircuitBreakerState `json:"state"`
		FailureCount int                 `json:"failure_count"`
		LastFailure  time.Time           `json:"last_failure,omitempty"`
		NextRetry    time.Time           `json:"next_retry,omitempty"`
	} `json:"circuit_breaker"`
}

// TierCapacity represents capacity information for a storage tier
type TierCapacity struct {
	TierType        TierType  `json:"tier_type"`
	MaxCapacity     int64     `json:"max_capacity"`
	UsedCapacity    int64     `json:"used_capacity"`
	AvailableCapacity int64   `json:"available_capacity"`
	MaxEntries      int64     `json:"max_entries"`
	UsedEntries     int64     `json:"used_entries"`
	UtilizationPct  float64   `json:"utilization_pct"`
	GrowthRate      float64   `json:"growth_rate"`
	ProjectedFull   *time.Time `json:"projected_full,omitempty"`
}

// SystemStats provides overall system statistics across all tiers
type SystemStats struct {
	TierStats       map[TierType]*TierStats `json:"tier_stats"`
	TotalRequests   int64                   `json:"total_requests"`
	OverallHitRate  float64                 `json:"overall_hit_rate"`
	AvgLatency      time.Duration           `json:"avg_latency"`
	TotalCapacity   int64                   `json:"total_capacity"`
	TotalUsed       int64                   `json:"total_used"`
	SystemUptime    time.Duration           `json:"system_uptime"`
	LastUpdate      time.Time               `json:"last_update"`
	
	// Cross-tier operations
	PromotionStats  *PromotionStats         `json:"promotion_stats"`
	EvictionStats   *EvictionStats          `json:"eviction_stats"`
	ReplicationStats *ReplicationStats      `json:"replication_stats,omitempty"`
}

// SystemHealth provides overall system health across all tiers
type SystemHealth struct {
	Healthy         bool                    `json:"healthy"`
	OverallStatus   SystemStatus            `json:"overall_status"`
	TierHealth      map[TierType]*TierHealth `json:"tier_health"`
	SystemIssues    []HealthIssue           `json:"system_issues,omitempty"`
	LastHealthCheck time.Time               `json:"last_health_check"`
	
	// Aggregate health metrics
	HealthScore     float64                 `json:"health_score"` // 0.0 to 1.0
	Availability    float64                 `json:"availability"` // Percentage
	Reliability     float64                 `json:"reliability"`  // Percentage
}

// Configuration types

// TierConfig contains configuration for a specific storage tier
type TierConfig struct {
	TierType        TierType              `json:"tier_type"`
	BackendType     BackendType           `json:"backend_type"`
	BackendConfig   BackendConfig         `json:"backend_config"`
	
	// Capacity configuration
	MaxCapacity     int64                 `json:"max_capacity"`
	MaxEntries      int64                 `json:"max_entries"`
	WarningThreshold float64              `json:"warning_threshold"`
	CriticalThreshold float64             `json:"critical_threshold"`
	
	// Performance configuration
	MaxConcurrency  int                   `json:"max_concurrency"`
	TimeoutMs       int                   `json:"timeout_ms"`
	RetryCount      int                   `json:"retry_count"`
	RetryDelayMs    int                   `json:"retry_delay_ms"`
	
	// Feature configuration
	CompressionConfig *CompressionConfig  `json:"compression_config,omitempty"`
	EncryptionConfig  *EncryptionConfig   `json:"encryption_config,omitempty"`
	MonitoringConfig  *MonitoringConfig   `json:"monitoring_config,omitempty"`
	
	// Maintenance configuration
	MaintenanceInterval time.Duration     `json:"maintenance_interval"`
	CompactionThreshold float64           `json:"compaction_threshold"`
	VacuumInterval      time.Duration     `json:"vacuum_interval"`
}

// BackendConfig contains configuration for storage backends
type BackendConfig struct {
	Type            BackendType           `json:"type"`
	ConnectionString string                `json:"connection_string,omitempty"`
	Options         map[string]interface{} `json:"options,omitempty"`
	
	// Authentication
	Username        string                `json:"username,omitempty"`
	Password        string                `json:"password,omitempty"`
	APIKey          string                `json:"api_key,omitempty"`
	CertPath        string                `json:"cert_path,omitempty"`
	
	// Network configuration
	TimeoutMs       int                   `json:"timeout_ms"`
	MaxConnections  int                   `json:"max_connections"`
	KeepAlive       bool                  `json:"keep_alive"`
	
	// Performance tuning
	ReadBufferSize  int                   `json:"read_buffer_size"`
	WriteBufferSize int                   `json:"write_buffer_size"`
	BatchSize       int                   `json:"batch_size"`
	
	// Reliability
	RetryCount      int                   `json:"retry_count"`
	RetryDelayMs    int                   `json:"retry_delay_ms"`
	CircuitBreaker  *CircuitBreakerConfig `json:"circuit_breaker,omitempty"`
}

// CompressionConfig contains compression settings
type CompressionConfig struct {
	Enabled     bool            `json:"enabled"`
	Type        CompressionType `json:"type"`
	Level       int             `json:"level"`
	MinSize     int64           `json:"min_size"`
	Threshold   float64         `json:"threshold"`
	Dictionary  []byte          `json:"dictionary,omitempty"`
}

// EncryptionConfig contains encryption settings
type EncryptionConfig struct {
	Enabled     bool                `json:"enabled"`
	Algorithm   EncryptionAlgorithm `json:"algorithm"`
	KeySize     int                 `json:"key_size"`
	KeyRotation time.Duration       `json:"key_rotation"`
	KeyPath     string              `json:"key_path,omitempty"`
}

// MonitoringConfig contains monitoring and observability settings
type MonitoringConfig struct {
	Enabled         bool          `json:"enabled"`
	MetricsInterval time.Duration `json:"metrics_interval"`
	HealthInterval  time.Duration `json:"health_interval"`
	TraceRequests   bool          `json:"trace_requests"`
	LogLevel        string        `json:"log_level"`
	AlertThresholds map[string]float64 `json:"alert_thresholds,omitempty"`
}

// CircuitBreakerConfig contains circuit breaker settings
type CircuitBreakerConfig struct {
	Enabled          bool          `json:"enabled"`
	FailureThreshold int           `json:"failure_threshold"`
	RecoveryTimeout  time.Duration `json:"recovery_timeout"`
	HalfOpenRequests int           `json:"half_open_requests"`
	MinRequestCount  int           `json:"min_request_count"`
}

// Enums and constants

// TierStatus represents the operational status of a storage tier
type TierStatus int

const (
	TierStatusUnknown TierStatus = iota
	TierStatusHealthy
	TierStatusDegraded
	TierStatusUnavailable
	TierStatusMaintenance
)

func (s TierStatus) String() string {
	switch s {
	case TierStatusHealthy:
		return "Healthy"
	case TierStatusDegraded:
		return "Degraded"
	case TierStatusUnavailable:
		return "Unavailable"
	case TierStatusMaintenance:
		return "Maintenance"
	default:
		return "Unknown"
	}
}

// SystemStatus represents the overall system status
type SystemStatus int

const (
	SystemStatusUnknown SystemStatus = iota
	SystemStatusHealthy
	SystemStatusDegraded
	SystemStatusCritical
	SystemStatusDown
)

func (s SystemStatus) String() string {
	switch s {
	case SystemStatusHealthy:
		return "Healthy"
	case SystemStatusDegraded:
		return "Degraded"
	case SystemStatusCritical:
		return "Critical"
	case SystemStatusDown:
		return "Down"
	default:
		return "Unknown"
	}
}

// CircuitBreakerState represents circuit breaker states
type CircuitBreakerState int

const (
	CircuitBreakerClosed CircuitBreakerState = iota
	CircuitBreakerOpen
	CircuitBreakerHalfOpen
)

func (s CircuitBreakerState) String() string {
	switch s {
	case CircuitBreakerClosed:
		return "Closed"
	case CircuitBreakerOpen:
		return "Open"
	case CircuitBreakerHalfOpen:
		return "HalfOpen"
	default:
		return "Unknown"
	}
}

// CachingHint provides optimization hints for caching behavior
type CachingHint int

const (
	CachingHintNone CachingHint = iota
	CachingHintPreferMemory
	CachingHintPreferDisk
	CachingHintPreferRemote
	CachingHintNoCache
	CachingHintPinned
)

func (h CachingHint) String() string {
	switch h {
	case CachingHintPreferMemory:
		return "PreferMemory"
	case CachingHintPreferDisk:
		return "PreferDisk"
	case CachingHintPreferRemote:
		return "PreferRemote"
	case CachingHintNoCache:
		return "NoCache"
	case CachingHintPinned:
		return "Pinned"
	default:
		return "None"
	}
}

// HealthIssue represents a health problem
type HealthIssue struct {
	Type        string                 `json:"type"`
	Severity    IssueSeverity          `json:"severity"`
	Message     string                 `json:"message"`
	Timestamp   time.Time              `json:"timestamp"`
	Component   string                 `json:"component,omitempty"`
	Details     map[string]interface{} `json:"details,omitempty"`
	Resolved    bool                   `json:"resolved"`
	ResolvedAt  *time.Time             `json:"resolved_at,omitempty"`
}

// IssueSeverity represents the severity of a health issue
type IssueSeverity int

const (
	IssueSeverityInfo IssueSeverity = iota
	IssueSeverityWarning
	IssueSeverityError
	IssueSeverityCritical
)

func (s IssueSeverity) String() string {
	switch s {
	case IssueSeverityInfo:
		return "Info"
	case IssueSeverityWarning:
		return "Warning"
	case IssueSeverityError:
		return "Error"
	case IssueSeverityCritical:
		return "Critical"
	default:
		return "Unknown"
	}
}

// LocalityInfo contains information about data locality
type LocalityInfo struct {
	ProjectPath    string            `json:"project_path,omitempty"`
	FilePaths      []string          `json:"file_paths,omitempty"`
	Language       string            `json:"language,omitempty"`
	Framework      string            `json:"framework,omitempty"`
	Dependencies   []string          `json:"dependencies,omitempty"`
	RelatedKeys    []string          `json:"related_keys,omitempty"`
	Locality       float64           `json:"locality"` // 0.0 to 1.0
}

// KeyInfo contains encryption key information
type KeyInfo struct {
	KeyID      string    `json:"key_id"`
	Algorithm  string    `json:"algorithm"`
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	RotatedAt  *time.Time `json:"rotated_at,omitempty"`
	Version    int       `json:"version"`
}

// Result types for operations

// InvalidationResult contains results of invalidation operations
type InvalidationResult struct {
	TotalInvalidated int                     `json:"total_invalidated"`
	TierResults      map[TierType]int        `json:"tier_results"`
	Duration         time.Duration           `json:"duration"`
	Errors           []error                 `json:"errors,omitempty"`
	Details          map[string]interface{}  `json:"details,omitempty"`
}

// RebalanceResult contains results of rebalancing operations
type RebalanceResult struct {
	ItemsMoved       int                     `json:"items_moved"`
	BytesMoved       int64                   `json:"bytes_moved"`
	Duration         time.Duration           `json:"duration"`
	TierChanges      map[TierType]int        `json:"tier_changes"`
	Errors           []error                 `json:"errors,omitempty"`
	PerformanceGain  float64                 `json:"performance_gain"`
}

// OptimizationResult contains results of storage optimization
type OptimizationResult struct {
	SpaceSaved       int64                   `json:"space_saved"`
	ItemsOptimized   int                     `json:"items_optimized"`
	CompressionGain  float64                 `json:"compression_gain"`
	Duration         time.Duration           `json:"duration"`
	TierResults      map[TierType]*TierOptimizationResult `json:"tier_results"`
	Recommendations  []string                `json:"recommendations,omitempty"`
}

// TierOptimizationResult contains tier-specific optimization results
type TierOptimizationResult struct {
	SpaceSaved      int64         `json:"space_saved"`
	ItemsProcessed  int           `json:"items_processed"`
	CompressionRatio float64      `json:"compression_ratio"`
	Duration        time.Duration `json:"duration"`
	Errors          []error       `json:"errors,omitempty"`
}

// MaintenanceResult contains results of maintenance operations
type MaintenanceResult struct {
	TasksCompleted   []string                `json:"tasks_completed"`
	Duration         time.Duration           `json:"duration"`
	ItemsProcessed   int                     `json:"items_processed"`
	SpaceReclaimed   int64                   `json:"space_reclaimed"`
	ErrorsEncountered []error                `json:"errors_encountered,omitempty"`
	TierResults      map[TierType]*TierMaintenanceResult `json:"tier_results"`
}

// TierMaintenanceResult contains tier-specific maintenance results
type TierMaintenanceResult struct {
	TasksCompleted  []string      `json:"tasks_completed"`
	ItemsProcessed  int           `json:"items_processed"`
	SpaceReclaimed  int64         `json:"space_reclaimed"`
	Duration        time.Duration `json:"duration"`
	Errors          []error       `json:"errors,omitempty"`
}

// BackendInfo contains information about a storage backend
type BackendInfo struct {
	Type            BackendType            `json:"type"`
	Version         string                 `json:"version"`
	Capabilities    []string               `json:"capabilities"`
	MaxKeySize      int                    `json:"max_key_size"`
	MaxValueSize    int64                  `json:"max_value_size"`
	SupportsStreaming bool                 `json:"supports_streaming"`
	SupportsCompression bool               `json:"supports_compression"`
	SupportsEncryption bool                `json:"supports_encryption"`
	SupportsTransactions bool              `json:"supports_transactions"`
	Performance     *BackendPerformance    `json:"performance,omitempty"`
}

// BackendPerformance contains performance characteristics of a backend
type BackendPerformance struct {
	TypicalReadLatency  time.Duration `json:"typical_read_latency"`
	TypicalWriteLatency time.Duration `json:"typical_write_latency"`
	MaxThroughput       float64       `json:"max_throughput"`
	MaxIOPS             float64       `json:"max_iops"`
	ReliabilityScore    float64       `json:"reliability_score"` // 0.0 to 1.0
}

// Promotion and eviction types

// PromotionStats contains statistics about data promotion
type PromotionStats struct {
	TotalPromotions   int64                          `json:"total_promotions"`
	SuccessfulPromotions int64                       `json:"successful_promotions"`
	FailedPromotions  int64                          `json:"failed_promotions"`
	BytesPromoted     int64                          `json:"bytes_promoted"`
	AvgPromotionTime  time.Duration                  `json:"avg_promotion_time"`
	TierPromotions    map[TierType]map[TierType]int64 `json:"tier_promotions"` // from -> to -> count
}

// EvictionStats contains statistics about data eviction
type EvictionStats struct {
	TotalEvictions    int64                    `json:"total_evictions"`
	SuccessfulEvictions int64                  `json:"successful_evictions"`
	FailedEvictions   int64                    `json:"failed_evictions"`
	BytesEvicted      int64                    `json:"bytes_evicted"`
	AvgEvictionTime   time.Duration            `json:"avg_eviction_time"`
	TierEvictions     map[TierType]int64       `json:"tier_evictions"`
	EvictionReasons   map[EvictionReason]int64 `json:"eviction_reasons"`
}

// ReplicationStats contains statistics about data replication
type ReplicationStats struct {
	TotalReplications int64         `json:"total_replications"`
	BytesReplicated   int64         `json:"bytes_replicated"`
	AvgReplicationTime time.Duration `json:"avg_replication_time"`
	ReplicationLag    time.Duration `json:"replication_lag"`
	SyncErrors        int64         `json:"sync_errors"`
}

// EvictionReason represents why data was evicted
type EvictionReason int

const (
	EvictionReasonCapacity EvictionReason = iota
	EvictionReasonTTL
	EvictionReasonLRU
	EvictionReasonPolicy
	EvictionReasonMaintenance
	EvictionReasonError
)

func (r EvictionReason) String() string {
	switch r {
	case EvictionReasonCapacity:
		return "Capacity"
	case EvictionReasonTTL:
		return "TTL"
	case EvictionReasonLRU:
		return "LRU"
	case EvictionReasonPolicy:
		return "Policy"
	case EvictionReasonMaintenance:
		return "Maintenance"
	case EvictionReasonError:
		return "Error"
	default:
		return "Unknown"
	}
}

// Observability types

// MetricsSnapshot contains a snapshot of metrics at a point in time
type MetricsSnapshot struct {
	Timestamp    time.Time              `json:"timestamp"`
	TimeRange    TimeRange              `json:"time_range"`
	TierMetrics  map[TierType]*TierMetrics `json:"tier_metrics"`
	SystemMetrics *SystemMetrics        `json:"system_metrics"`
}

// TierMetrics contains detailed metrics for a storage tier
type TierMetrics struct {
	Requests      *RequestMetrics     `json:"requests"`
	Latency       *LatencyMetrics     `json:"latency"`
	Throughput    *ThroughputMetrics  `json:"throughput"`
	Capacity      *CapacityMetrics    `json:"capacity"`
	Errors        *ErrorMetrics       `json:"errors"`
	Operations    *OperationMetrics   `json:"operations"`
}

// RequestMetrics contains request-related metrics
type RequestMetrics struct {
	Total       int64   `json:"total"`
	Hits        int64   `json:"hits"`
	Misses      int64   `json:"misses"`
	HitRate     float64 `json:"hit_rate"`
	RequestRate float64 `json:"request_rate"`
}

// LatencyMetrics contains latency distribution metrics
type LatencyMetrics struct {
	Mean   time.Duration `json:"mean"`
	Median time.Duration `json:"median"`
	P95    time.Duration `json:"p95"`
	P99    time.Duration `json:"p99"`
	P999   time.Duration `json:"p999"`
	Min    time.Duration `json:"min"`
	Max    time.Duration `json:"max"`
}

// ThroughputMetrics contains throughput metrics
type ThroughputMetrics struct {
	RequestsPerSecond float64 `json:"requests_per_second"`
	BytesPerSecond    float64 `json:"bytes_per_second"`
	ReadThroughput    float64 `json:"read_throughput"`
	WriteThroughput   float64 `json:"write_throughput"`
}

// CapacityMetrics contains capacity utilization metrics
type CapacityMetrics struct {
	TotalCapacity    int64   `json:"total_capacity"`
	UsedCapacity     int64   `json:"used_capacity"`
	AvailableCapacity int64  `json:"available_capacity"`
	UtilizationPct   float64 `json:"utilization_pct"`
	GrowthRate       float64 `json:"growth_rate"`
}

// ErrorMetrics contains error-related metrics
type ErrorMetrics struct {
	TotalErrors  int64   `json:"total_errors"`
	ErrorRate    float64 `json:"error_rate"`
	Timeouts     int64   `json:"timeouts"`
	Retries      int64   `json:"retries"`
	CircuitBreaker int64 `json:"circuit_breaker"`
}

// OperationMetrics contains operation-specific metrics
type OperationMetrics struct {
	Promotions  int64 `json:"promotions"`
	Evictions   int64 `json:"evictions"`
	Compactions int64 `json:"compactions"`
	Repairs     int64 `json:"repairs"`
	Cleanups    int64 `json:"cleanups"`
}

// SystemMetrics contains system-wide metrics
type SystemMetrics struct {
	TotalRequests    int64         `json:"total_requests"`
	OverallHitRate   float64       `json:"overall_hit_rate"`
	SystemLatency    time.Duration `json:"system_latency"`
	SystemThroughput float64       `json:"system_throughput"`
	SystemUptime     time.Duration `json:"system_uptime"`
	MemoryUsage      int64         `json:"memory_usage"`
	DiskUsage        int64         `json:"disk_usage"`
	NetworkUsage     int64         `json:"network_usage"`
}

// HealthSummary contains a summary of system health
type HealthSummary struct {
	OverallHealth   float64                   `json:"overall_health"` // 0.0 to 1.0
	TierHealth      map[TierType]float64      `json:"tier_health"`
	ActiveIssues    int                       `json:"active_issues"`
	CriticalIssues  int                       `json:"critical_issues"`
	LastCheck       time.Time                 `json:"last_check"`
	Issues          []HealthIssue             `json:"issues,omitempty"`
	Recommendations []string                  `json:"recommendations,omitempty"`
}

// TimeRange represents a time range for metrics
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// MetricsFormat represents different formats for exporting metrics
type MetricsFormat int

const (
	MetricsFormatJSON MetricsFormat = iota
	MetricsFormatPrometheus
	MetricsFormatCSV
	MetricsFormatInfluxDB
)

func (f MetricsFormat) String() string {
	switch f {
	case MetricsFormatJSON:
		return "JSON"
	case MetricsFormatPrometheus:
		return "Prometheus"
	case MetricsFormatCSV:
		return "CSV"
	case MetricsFormatInfluxDB:
		return "InfluxDB"
	default:
		return "Unknown"
	}
}

// Event types for monitoring
type MaintenanceEventType int

const (
	MaintenanceEventCompaction MaintenanceEventType = iota
	MaintenanceEventVacuum
	MaintenanceEventRepair
	MaintenanceEventCleanup
	MaintenanceEventRebalance
	MaintenanceEventOptimization
)

func (e MaintenanceEventType) String() string {
	switch e {
	case MaintenanceEventCompaction:
		return "Compaction"
	case MaintenanceEventVacuum:
		return "Vacuum"
	case MaintenanceEventRepair:
		return "Repair"
	case MaintenanceEventCleanup:
		return "Cleanup"
	case MaintenanceEventRebalance:
		return "Rebalance"
	case MaintenanceEventOptimization:
		return "Optimization"
	default:
		return "Unknown"
	}
}

type ConfigChangeType int

const (
	ConfigChangeCapacity ConfigChangeType = iota
	ConfigChangePerformance
	ConfigChangeSecurity
	ConfigChangeMonitoring
	ConfigChangeMaintenance
)

func (c ConfigChangeType) String() string {
	switch c {
	case ConfigChangeCapacity:
		return "Capacity"
	case ConfigChangePerformance:
		return "Performance"
	case ConfigChangeSecurity:
		return "Security"
	case ConfigChangeMonitoring:
		return "Monitoring"
	case ConfigChangeMaintenance:
		return "Maintenance"
	default:
		return "Unknown"
	}
}