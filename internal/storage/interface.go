package storage

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"lsp-gateway/internal/indexing"
)

// StorageTier defines the core interface for all storage tiers in the three-tier architecture
// All tiers (L1 Memory, L2 Disk, L3 Remote) implement this interface for consistent operations
type StorageTier interface {
	// Core CRUD operations
	Get(ctx context.Context, key string) (*CacheEntry, error)
	Put(ctx context.Context, key string, entry *CacheEntry) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)

	// Batch operations for efficiency
	GetBatch(ctx context.Context, keys []string) (map[string]*CacheEntry, error)
	PutBatch(ctx context.Context, entries map[string]*CacheEntry) error
	DeleteBatch(ctx context.Context, keys []string) error

	// Cache management
	Invalidate(ctx context.Context, pattern string) (int, error)
	InvalidateByFile(ctx context.Context, filePath string) (int, error)
	InvalidateByProject(ctx context.Context, projectPath string) (int, error)
	Clear(ctx context.Context) error

	// Performance and monitoring
	GetStats() TierStats
	GetHealth() TierHealth
	GetCapacity() TierCapacity

	// Lifecycle management
	Initialize(ctx context.Context, config TierConfig) error
	Close() error
	Flush(ctx context.Context) error

	// Tier-specific identification
	GetTierType() TierType
	GetTierLevel() int
}

// ThreeTierStorage coordinates operations across L1, L2, and L3 storage tiers
// Implements intelligent cache management with promotion/eviction strategies
type ThreeTierStorage interface {
	// Primary operations delegate to appropriate tier based on strategy
	Get(ctx context.Context, key string) (*CacheEntry, error)
	Put(ctx context.Context, key string, entry *CacheEntry) error
	Delete(ctx context.Context, key string) error

	// Cross-tier operations
	Promote(ctx context.Context, key string, fromTier, toTier TierType) error
	Demote(ctx context.Context, key string, fromTier, toTier TierType) error
	Rebalance(ctx context.Context) (*RebalanceResult, error)

	// Intelligent invalidation across all tiers
	InvalidateFile(ctx context.Context, filePath string) (*InvalidationResult, error)
	InvalidateProject(ctx context.Context, projectPath string) (*InvalidationResult, error)
	InvalidatePattern(ctx context.Context, pattern string) (*InvalidationResult, error)

	// Monitoring and observability
	GetSystemStats() *SystemStats
	GetSystemHealth() *SystemHealth
	GetTierStats(tierType TierType) *TierStats

	// Configuration and lifecycle
	UpdateStrategy(strategy PromotionStrategy) error
	UpdateEvictionPolicy(policy EvictionPolicy) error
	Shutdown(ctx context.Context) error
}

// SCIPStorageAdapter adapts the three-tier storage system to work with existing SCIP interfaces
// Provides seamless integration with indexing.SCIPStore while adding multi-tier capabilities
type SCIPStorageAdapter interface {
	indexing.SCIPStore

	// Enhanced operations leveraging three-tier architecture
	QueryWithTierHint(method string, params interface{}, preferredTier TierType) indexing.SCIPQueryResult
	CacheResponseWithTier(method string, params interface{}, response json.RawMessage, tier TierType) error
	GetTierStatistics() map[TierType]*TierStats

	// Advanced cache management
	PrewarmCache(ctx context.Context, entries map[string]*CacheEntry) error
	OptimizeStorage(ctx context.Context) (*OptimizationResult, error)
	PerformMaintenance(ctx context.Context) (*MaintenanceResult, error)
}

// StorageBackend defines the interface for different storage backend implementations
// Each tier can use different backends (Memory, SSD, HDD, Remote, etc.)
type StorageBackend interface {
	// Backend identification
	GetBackendType() BackendType
	GetBackendInfo() *BackendInfo

	// Core operations
	Read(ctx context.Context, key string) ([]byte, *StorageMetadata, error)
	Write(ctx context.Context, key string, data []byte, metadata *StorageMetadata) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)

	// Streaming operations for large data
	ReadStream(ctx context.Context, key string) (io.ReadCloser, *StorageMetadata, error)
	WriteStream(ctx context.Context, key string, data io.Reader, metadata *StorageMetadata) error

	// Maintenance operations
	Compact(ctx context.Context) error
	Vacuum(ctx context.Context) error
	Repair(ctx context.Context) error

	// Configuration and lifecycle
	Configure(config BackendConfig) error
	Initialize(ctx context.Context) error
	Close() error
}

// CompressionProvider defines interface for data compression in storage tiers
// Enables space optimization especially for L2 disk and L3 remote storage
type CompressionProvider interface {
	// Compression operations
	Compress(data []byte) ([]byte, error)
	Decompress(data []byte) ([]byte, error)

	// Compression analysis
	EstimateCompressionRatio(data []byte) float64
	GetCompressionType() CompressionType
	GetCompressionLevel() int

	// Configuration
	SetCompressionLevel(level int) error
	SetCompressionType(compressionType CompressionType) error
}

// EncryptionProvider defines interface for data encryption in storage tiers
// Provides security for sensitive code data especially in remote storage
type EncryptionProvider interface {
	// Encryption operations
	Encrypt(data []byte) ([]byte, error)
	Decrypt(data []byte) ([]byte, error)

	// Key management
	RotateKeys() error
	GetKeyInfo() *KeyInfo

	// Configuration
	SetEncryptionAlgorithm(algorithm EncryptionAlgorithm) error
	SetKeySize(size int) error
}

// ObservabilityProvider defines interface for comprehensive monitoring and metrics
// Enables detailed performance tracking and optimization insights
type ObservabilityProvider interface {
	// Metrics collection
	RecordGet(tier TierType, key string, hit bool, latency time.Duration)
	RecordPut(tier TierType, key string, size int64, latency time.Duration)
	RecordDelete(tier TierType, key string, latency time.Duration)
	RecordPromotion(fromTier, toTier TierType, key string, latency time.Duration)
	RecordEviction(tier TierType, key string, reason EvictionReason)

	// Performance tracking
	RecordCacheHitRate(tier TierType, rate float64)
	RecordThroughput(tier TierType, ops float64)
	RecordLatencyPercentiles(tier TierType, p50, p95, p99 time.Duration)
	RecordErrorRate(tier TierType, rate float64)

	// Capacity monitoring
	RecordStorageUtilization(tier TierType, used, total int64)
	RecordMemoryUsage(tier TierType, used, total int64)
	RecordDiskIOPS(tier TierType, readIOPS, writeIOPS float64)
	RecordNetworkLatency(tier TierType, latency time.Duration)

	// Health monitoring
	RecordHealthCheck(tier TierType, healthy bool, details string)
	RecordCircuitBreakerState(tier TierType, state CircuitBreakerState)

	// Event tracking
	RecordMaintenanceEvent(tier TierType, eventType MaintenanceEventType, duration time.Duration)
	RecordConfigurationChange(tier TierType, changeType ConfigChangeType, details string)

	// Query and reporting
	GetMetrics(tier TierType, timeRange TimeRange) (*MetricsSnapshot, error)
	GetHealthSummary() (*HealthSummary, error)
	ExportMetrics(format MetricsFormat) ([]byte, error)
}

// PromotionStrategy defines interface for intelligent data promotion between tiers
// Determines when and how data should be moved to faster tiers
type PromotionStrategy interface {
	// Promotion decision logic
	ShouldPromote(entry *CacheEntry, accessPattern *AccessPattern, currentTier TierType) (bool, TierType)
	CalculatePromotionPriority(entry *CacheEntry, accessPattern *AccessPattern) int

	// Batch promotion optimization
	SelectPromotionCandidates(tier TierType, maxCandidates int) ([]*PromotionCandidate, error)
	OptimizePromotionBatch(candidates []*PromotionCandidate) ([]*PromotionCandidate, error)

	// Strategy configuration
	UpdateParameters(params PromotionParameters) error
	GetStrategyType() PromotionStrategyType
	GetCurrentParameters() PromotionParameters
}

// EvictionPolicy defines interface for intelligent data eviction from tiers
// Determines what data should be evicted when tier capacity is reached
type EvictionPolicy interface {
	// Eviction decision logic
	ShouldEvict(entry *CacheEntry, accessPattern *AccessPattern, currentTier TierType) (bool, TierType)
	CalculateEvictionPriority(entry *CacheEntry, accessPattern *AccessPattern) int

	// Batch eviction optimization
	SelectEvictionCandidates(tier TierType, requiredSpace int64) ([]*EvictionCandidate, error)
	OptimizeEvictionBatch(candidates []*EvictionCandidate) ([]*EvictionCandidate, error)

	// Policy configuration
	UpdateParameters(params EvictionParameters) error
	GetPolicyType() EvictionPolicyType
	GetCurrentParameters() EvictionParameters
}

// AccessPattern defines interface for tracking and analyzing data access patterns
// Enables intelligent promotion/eviction decisions based on usage
type AccessPattern interface {
	// Access tracking
	RecordAccess(key string, accessType AccessType, metadata *AccessMetadata)
	GetAccessFrequency(key string, timeWindow time.Duration) float64
	GetAccessRecency(key string) time.Duration
	GetAccessLocality(key string) *LocalityInfo

	// Pattern analysis
	AnalyzePattern(key string) *PatternAnalysis
	PredictFutureAccess(key string, horizon time.Duration) float64
	ClassifyAccessPattern(key string) AccessPatternType

	// Bulk operations
	GetTopAccessedKeys(tier TierType, limit int) ([]string, error)
	GetColdKeys(tier TierType, threshold time.Duration) ([]string, error)
	GetHotKeys(tier TierType, threshold float64) ([]string, error)

	// Configuration
	UpdateTrackingParameters(params AccessTrackingParameters) error
	PurgeOldPatterns(cutoff time.Time) error
}
