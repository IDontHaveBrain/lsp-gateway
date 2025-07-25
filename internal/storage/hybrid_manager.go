package storage

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// HybridStorageManager implements intelligent three-tier storage coordination
// Orchestrates L1 Memory, L2 Disk, and L3 Remote storage tiers with intelligent
// promotion/eviction strategies and comprehensive monitoring
type HybridStorageManager struct {
	// Storage tiers
	l1Memory StorageTier
	l2Disk   StorageTier
	l3Remote StorageTier
	
	// Intelligent strategies
	promotionStrategy PromotionStrategy
	evictionPolicy    EvictionPolicy
	accessTracker     AccessPattern
	
	// Configuration
	config *HybridStorageConfig
	
	// Concurrency control - reader-writer locks for minimal contention
	tierMu    sync.RWMutex // Protects tier operations
	strategyMu sync.RWMutex // Protects strategy updates
	
	// Atomic counters for metrics
	stats *hybridStats
	
	// Background processing
	stopCh          chan struct{}
	backgroundWg    sync.WaitGroup
	maintenanceCh   chan maintenanceRequest
	promotionCh     chan promotionRequest
	evictionCh      chan evictionRequest
	
	// Circuit breakers for tier protection
	l1CircuitBreaker *tierCircuitBreaker
	l2CircuitBreaker *tierCircuitBreaker
	l3CircuitBreaker *tierCircuitBreaker
	
	// Health monitoring
	health           *hybridHealth
	healthUpdateCh   chan healthUpdate
	
	// Object pools for performance
	candidatePool    sync.Pool
	resultPool       sync.Pool
	
	// Observability
	observer         ObservabilityProvider
	
	// Lifecycle management
	started          atomic.Bool
	shutdownOnce     sync.Once
}

// HybridStorageConfig contains configuration for the hybrid storage manager
type HybridStorageConfig struct {
	// Tier configuration
	L1Config *TierConfig `json:"l1_config"`
	L2Config *TierConfig `json:"l2_config"`
	L3Config *TierConfig `json:"l3_config"`
	
	// Strategy configuration
	PromotionConfig *PromotionParameters `json:"promotion_config"`
	EvictionConfig  *EvictionParameters  `json:"eviction_config"`
	AccessConfig    *AccessTrackingParameters `json:"access_config"`
	
	// Performance configuration
	MaxConcurrentOps     int           `json:"max_concurrent_ops"`
	BatchProcessingSize  int           `json:"batch_processing_size"`
	BackgroundWorkers    int           `json:"background_workers"`
	PromotionInterval    time.Duration `json:"promotion_interval"`
	EvictionInterval     time.Duration `json:"eviction_interval"`
	MaintenanceInterval  time.Duration `json:"maintenance_interval"`
	
	// Circuit breaker configuration
	CircuitBreakerConfig *CircuitBreakerConfig `json:"circuit_breaker_config"`
	
	// Monitoring configuration
	MetricsInterval      time.Duration `json:"metrics_interval"`
	HealthCheckInterval  time.Duration `json:"health_check_interval"`
	EnableDetailedMetrics bool         `json:"enable_detailed_metrics"`
	
	// Advanced options
	EnablePredictivePreloading bool    `json:"enable_predictive_preloading"`
	EnableAdaptiveStrategies   bool    `json:"enable_adaptive_strategies"`
	PreloadingThreshold        float64 `json:"preloading_threshold"`
	AdaptationInterval         time.Duration `json:"adaptation_interval"`
}

// hybridStats contains atomic counters for comprehensive metrics
type hybridStats struct {
	// Request metrics
	totalRequests    atomic.Int64
	l1Hits          atomic.Int64
	l2Hits          atomic.Int64
	l3Hits          atomic.Int64
	totalMisses     atomic.Int64
	
	// Operation metrics
	promotions      atomic.Int64
	evictions       atomic.Int64
	rebalances      atomic.Int64
	invalidations   atomic.Int64
	
	// Performance metrics
	totalLatency    atomic.Int64 // nanoseconds
	l1Latency       atomic.Int64
	l2Latency       atomic.Int64
	l3Latency       atomic.Int64
	
	// Error metrics
	totalErrors     atomic.Int64
	l1Errors        atomic.Int64
	l2Errors        atomic.Int64
	l3Errors        atomic.Int64
	
	// Background operation metrics
	backgroundPromotions atomic.Int64
	backgroundEvictions  atomic.Int64
	backgroundFailures   atomic.Int64
	
	// Circuit breaker metrics
	l1CircuitBreakerTrips atomic.Int64
	l2CircuitBreakerTrips atomic.Int64
	l3CircuitBreakerTrips atomic.Int64
	
	// Start time for uptime calculation
	startTime time.Time
}

// hybridHealth contains health status information
type hybridHealth struct {
	mu                sync.RWMutex
	healthy           bool
	lastCheck         time.Time
	issues            []HealthIssue
	tierHealth        map[TierType]*TierHealth
	overallScore      float64
	systemStatus      SystemStatus
	recommendations   []*Recommendation
}

// Request types for background processing
type maintenanceRequest struct {
	requestType  string
	responseC    chan *MaintenanceResult
	ctx          context.Context
}

type promotionRequest struct {
	tier      TierType
	maxItems  int
	responseC chan *PromotionResult
	ctx       context.Context
}

type evictionRequest struct {
	tier         TierType
	requiredSpace int64
	responseC    chan *EvictionResult
	ctx          context.Context
}

type healthUpdate struct {
	tierType TierType
	health   *TierHealth
}

// PromotionResult contains results of promotion operations
type PromotionResult struct {
	ItemsPromoted    int                    `json:"items_promoted"`
	BytesPromoted    int64                  `json:"bytes_promoted"`
	Duration         time.Duration          `json:"duration"`
	PromotedItems    []*PromotionCandidate  `json:"promoted_items,omitempty"`
	Errors           []error                `json:"errors,omitempty"`
	PerformanceGain  float64                `json:"performance_gain"`
}

// NewHybridStorageManager creates a new hybrid storage manager
func NewHybridStorageManager(config *HybridStorageConfig, observer ObservabilityProvider) (*HybridStorageManager, error) {
	if config == nil {
		return nil, errors.New("configuration is required")
	}
	
	hsm := &HybridStorageManager{
		config:          config,
		observer:        observer,
		stats:           &hybridStats{startTime: time.Now()},
		health:          &hybridHealth{tierHealth: make(map[TierType]*TierHealth)},
		stopCh:          make(chan struct{}),
		maintenanceCh:   make(chan maintenanceRequest, 100),
		promotionCh:     make(chan promotionRequest, 100),
		evictionCh:      make(chan evictionRequest, 100),
		healthUpdateCh:  make(chan healthUpdate, 100),
	}
	
	// Initialize object pools
	hsm.candidatePool = sync.Pool{
		New: func() interface{} {
			return make([]*PromotionCandidate, 0, config.BatchProcessingSize)
		},
	}
	hsm.resultPool = sync.Pool{
		New: func() interface{} {
			return &PromotionResult{}
		},
	}
	
	// Initialize circuit breakers
	hsm.l1CircuitBreaker = newTierCircuitBreaker(TierL1Memory, config.CircuitBreakerConfig)
	hsm.l2CircuitBreaker = newTierCircuitBreaker(TierL2Disk, config.CircuitBreakerConfig)
	hsm.l3CircuitBreaker = newTierCircuitBreaker(TierL3Remote, config.CircuitBreakerConfig)
	
	return hsm, nil
}

// Initialize initializes the hybrid storage manager with storage tiers
func (hsm *HybridStorageManager) Initialize(ctx context.Context, l1 StorageTier, l2 StorageTier, l3 StorageTier) error {
	if hsm.started.Load() {
		return errors.New("hybrid storage manager already initialized")
	}
	
	// Set storage tiers
	hsm.tierMu.Lock()
	hsm.l1Memory = l1
	hsm.l2Disk = l2
	hsm.l3Remote = l3
	hsm.tierMu.Unlock()
	
	// Initialize tiers
	if err := hsm.initializeTiers(ctx); err != nil {
		return fmt.Errorf("failed to initialize tiers: %w", err)
	}
	
	// Initialize strategies
	if err := hsm.initializeStrategies(); err != nil {
		return fmt.Errorf("failed to initialize strategies: %w", err)
	}
	
	// Start background processing
	hsm.startBackgroundProcessing()
	
	hsm.started.Store(true)
	return nil
}

// Get retrieves an entry from the optimal storage tier
func (hsm *HybridStorageManager) Get(ctx context.Context, key string) (*CacheEntry, error) {
	start := time.Now()
	defer func() {
		hsm.stats.totalRequests.Add(1)
		latency := time.Since(start)
		hsm.stats.totalLatency.Add(int64(latency))
		if hsm.observer != nil {
			hsm.observer.RecordGet(TierL1Memory, key, false, latency)
		}
	}()
	
	// Record access for pattern analysis
	if hsm.accessTracker != nil {
		hsm.accessTracker.RecordAccess(key, AccessTypeRead, &AccessMetadata{
			Timestamp: start,
			Source:    "hybrid_manager",
		})
	}
	
	// Try L1 (Memory) first
	if entry, err := hsm.tryGetFromTier(ctx, TierL1Memory, key); err == nil {
		hsm.stats.l1Hits.Add(1)
		entry.Touch()
		hsm.promoteInBackground(ctx, key, entry, TierL2Disk, TierL1Memory)
		return entry, nil
	}
	
	// Try L2 (Disk) second
	if entry, err := hsm.tryGetFromTier(ctx, TierL2Disk, key); err == nil {
		hsm.stats.l2Hits.Add(1)
		entry.Touch()
		// Promote to L1 asynchronously
		hsm.promoteInBackground(ctx, key, entry, TierL2Disk, TierL1Memory)
		return entry, nil
	}
	
	// Try L3 (Remote) last
	if entry, err := hsm.tryGetFromTier(ctx, TierL3Remote, key); err == nil {
		hsm.stats.l3Hits.Add(1)
		entry.Touch()
		// Promote to L2 asynchronously
		hsm.promoteInBackground(ctx, key, entry, TierL3Remote, TierL2Disk)
		return entry, nil
	}
	
	hsm.stats.totalMisses.Add(1)
	return nil, errors.New("entry not found in any tier")
}

// Put stores an entry in the appropriate storage tier
func (hsm *HybridStorageManager) Put(ctx context.Context, key string, entry *CacheEntry) error {
	start := time.Now()
	defer func() {
		latency := time.Since(start)
		if hsm.observer != nil {
			hsm.observer.RecordPut(TierL1Memory, key, entry.Size, latency)
		}
	}()
	
	// Record access for pattern analysis
	if hsm.accessTracker != nil {
		hsm.accessTracker.RecordAccess(key, AccessTypeWrite, &AccessMetadata{
			Timestamp: start,
			Size:      entry.Size,
			Source:    "hybrid_manager",
		})
	}
	
	// Determine initial tier based on caching hint and size
	targetTier := hsm.determineInitialTier(entry)
	
	// Store in target tier
	if err := hsm.putInTier(ctx, targetTier, key, entry); err != nil {
		// If target tier fails, try fallback tiers
		if err := hsm.putWithFallback(ctx, key, entry, targetTier); err != nil {
			hsm.stats.totalErrors.Add(1)
			return fmt.Errorf("failed to store entry in any tier: %w", err)
		}
	}
	
	// Trigger background maintenance if needed
	hsm.triggerMaintenanceIfNeeded(ctx)
	
	return nil
}

// Delete removes an entry from all storage tiers
func (hsm *HybridStorageManager) Delete(ctx context.Context, key string) error {
	start := time.Now()
	defer func() {
		latency := time.Since(start)
		if hsm.observer != nil {
			hsm.observer.RecordDelete(TierL1Memory, key, latency)
		}
	}()
	
	// Record access for pattern analysis
	if hsm.accessTracker != nil {
		hsm.accessTracker.RecordAccess(key, AccessTypeDelete, &AccessMetadata{
			Timestamp: start,
			Source:    "hybrid_manager",
		})
	}
	
	var errors []error
	
	// Delete from all tiers concurrently
	results := make(chan error, 3)
	
	go func() {
		results <- hsm.deleteFromTier(ctx, TierL1Memory, key)
	}()
	go func() {
		results <- hsm.deleteFromTier(ctx, TierL2Disk, key)
	}()
	go func() {
		results <- hsm.deleteFromTier(ctx, TierL3Remote, key)
	}()
	
	// Collect results
	for i := 0; i < 3; i++ {
		if err := <-results; err != nil {
			errors = append(errors, err)
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("delete errors: %v", errors)
	}
	
	return nil
}

// Promote moves data from a lower tier to a higher tier
func (hsm *HybridStorageManager) Promote(ctx context.Context, key string, fromTier, toTier TierType) error {
	start := time.Now()
	defer func() {
		latency := time.Since(start)
		if hsm.observer != nil {
			hsm.observer.RecordPromotion(fromTier, toTier, key, latency)
		}
	}()
	
	// Get entry from source tier
	entry, err := hsm.getFromTier(ctx, fromTier, key)
	if err != nil {
		return fmt.Errorf("failed to get entry from tier %s: %w", fromTier, err)
	}
	
	// Check if promotion is beneficial
	if hsm.promotionStrategy != nil {
		accessPattern := hsm.getAccessPattern(key)
		if shouldPromote, targetTier := hsm.promotionStrategy.ShouldPromote(entry, accessPattern, fromTier); !shouldPromote || targetTier != toTier {
			return errors.New("promotion not beneficial")
		}
	}
	
	// Store in target tier
	if err := hsm.putInTier(ctx, toTier, key, entry); err != nil {
		return fmt.Errorf("failed to store in tier %s: %w", toTier, err)
	}
	
	// Update tier information
	entry.CurrentTier = toTier
	entry.TierHistory = append(entry.TierHistory, TierTransition{
		FromTier:  fromTier,
		ToTier:    toTier,
		Timestamp: time.Now(),
		Reason:    "manual_promotion",
		Latency:   time.Since(start),
		Success:   true,
	})
	
	hsm.stats.promotions.Add(1)
	return nil
}

// Demote moves data from a higher tier to a lower tier
func (hsm *HybridStorageManager) Demote(ctx context.Context, key string, fromTier, toTier TierType) error {
	start := time.Now()
	
	// Get entry from source tier
	entry, err := hsm.getFromTier(ctx, fromTier, key)
	if err != nil {
		return fmt.Errorf("failed to get entry from tier %s: %w", fromTier, err)
	}
	
	// Store in target tier
	if err := hsm.putInTier(ctx, toTier, key, entry); err != nil {
		return fmt.Errorf("failed to store in tier %s: %w", toTier, err)
	}
	
	// Remove from source tier
	if err := hsm.deleteFromTier(ctx, fromTier, key); err != nil {
		// Best effort cleanup, don't fail the operation
		hsm.logError("failed to cleanup after demotion", err)
	}
	
	// Update tier information
	entry.CurrentTier = toTier
	entry.TierHistory = append(entry.TierHistory, TierTransition{
		FromTier:  fromTier,
		ToTier:    toTier,
		Timestamp: time.Now(),
		Reason:    "manual_demotion",
		Latency:   time.Since(start),
		Success:   true,
	})
	
	return nil
}

// Rebalance optimizes data distribution across tiers
func (hsm *HybridStorageManager) Rebalance(ctx context.Context) (*RebalanceResult, error) {
	start := time.Now()
	
	result := &RebalanceResult{
		TierChanges: make(map[TierType]int),
	}
	
	// Get rebalancing recommendations
	promotionCandidates, err := hsm.getPromotionCandidates(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get promotion candidates: %w", err)
	}
	
	evictionCandidates, err := hsm.getEvictionCandidates(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get eviction candidates: %w", err)
	}
	
	// Execute promotions
	for _, candidate := range promotionCandidates {
		if err := hsm.Promote(ctx, candidate.Key, candidate.FromTier, candidate.ToTier); err != nil {
			result.Errors = append(result.Errors, err)
		} else {
			result.ItemsMoved++
			result.BytesMoved += candidate.Entry.Size
			result.TierChanges[candidate.ToTier]++
		}
	}
	
	// Execute evictions
	for _, candidate := range evictionCandidates {
		if candidate.ToTier != nil {
			if err := hsm.Demote(ctx, candidate.Key, candidate.FromTier, *candidate.ToTier); err != nil {
				result.Errors = append(result.Errors, err)
			} else {
				result.ItemsMoved++
				result.BytesMoved += candidate.Entry.Size
				result.TierChanges[*candidate.ToTier]++
			}
		}
	}
	
	result.Duration = time.Since(start)
	hsm.stats.rebalances.Add(1)
	
	return result, nil
}

// InvalidateFile invalidates entries associated with a file across all tiers
func (hsm *HybridStorageManager) InvalidateFile(ctx context.Context, filePath string) (*InvalidationResult, error) {
	start := time.Now()
	
	result := &InvalidationResult{
		TierResults: make(map[TierType]int),
		Details:     make(map[string]interface{}),
	}
	
	// Invalidate across all tiers concurrently
	results := make(chan struct {
		tier  TierType
		count int
		err   error
	}, 3)
	
	go func() {
		count, err := hsm.invalidateInTier(ctx, TierL1Memory, filePath)
		results <- struct {
			tier  TierType
			count int
			err   error
		}{TierL1Memory, count, err}
	}()
	
	go func() {
		count, err := hsm.invalidateInTier(ctx, TierL2Disk, filePath)
		results <- struct {
			tier  TierType
			count int
			err   error
		}{TierL2Disk, count, err}
	}()
	
	go func() {
		count, err := hsm.invalidateInTier(ctx, TierL3Remote, filePath)
		results <- struct {
			tier  TierType
			count int
			err   error
		}{TierL3Remote, count, err}
	}()
	
	// Collect results
	for i := 0; i < 3; i++ {
		res := <-results
		if res.err != nil {
			result.Errors = append(result.Errors, res.err)
		} else {
			result.TierResults[res.tier] = res.count
			result.TotalInvalidated += res.count
		}
	}
	
	result.Duration = time.Since(start)
	result.Details["file_path"] = filePath
	
	hsm.stats.invalidations.Add(1)
	return result, nil
}

// InvalidateProject invalidates entries associated with a project across all tiers
func (hsm *HybridStorageManager) InvalidateProject(ctx context.Context, projectPath string) (*InvalidationResult, error) {
	start := time.Now()
	
	result := &InvalidationResult{
		TierResults: make(map[TierType]int),
		Details:     make(map[string]interface{}),
	}
	
	// Invalidate across all tiers concurrently
	results := make(chan struct {
		tier  TierType
		count int
		err   error
	}, 3)
	
	go func() {
		count, err := hsm.invalidateProjectInTier(ctx, TierL1Memory, projectPath)
		results <- struct {
			tier  TierType
			count int
			err   error
		}{TierL1Memory, count, err}
	}()
	
	go func() {
		count, err := hsm.invalidateProjectInTier(ctx, TierL2Disk, projectPath)
		results <- struct {
			tier  TierType
			count int
			err   error
		}{TierL2Disk, count, err}
	}()
	
	go func() {
		count, err := hsm.invalidateProjectInTier(ctx, TierL3Remote, projectPath)
		results <- struct {
			tier  TierType
			count int
			err   error
		}{TierL3Remote, count, err}
	}()
	
	// Collect results
	for i := 0; i < 3; i++ {
		res := <-results
		if res.err != nil {
			result.Errors = append(result.Errors, res.err)
		} else {
			result.TierResults[res.tier] = res.count
			result.TotalInvalidated += res.count
		}
	}
	
	result.Duration = time.Since(start)
	result.Details["project_path"] = projectPath
	
	hsm.stats.invalidations.Add(1)
	return result, nil
}

// InvalidatePattern invalidates entries matching a pattern across all tiers
func (hsm *HybridStorageManager) InvalidatePattern(ctx context.Context, pattern string) (*InvalidationResult, error) {
	start := time.Now()
	
	result := &InvalidationResult{
		TierResults: make(map[TierType]int),
		Details:     make(map[string]interface{}),
	}
	
	// Invalidate across all tiers concurrently
	results := make(chan struct {
		tier  TierType
		count int
		err   error
	}, 3)
	
	go func() {
		count, err := hsm.invalidatePatternInTier(ctx, TierL1Memory, pattern)
		results <- struct {
			tier  TierType
			count int
			err   error
		}{TierL1Memory, count, err}
	}()
	
	go func() {
		count, err := hsm.invalidatePatternInTier(ctx, TierL2Disk, pattern)
		results <- struct {
			tier  TierType
			count int
			err   error
		}{TierL2Disk, count, err}
	}()
	
	go func() {
		count, err := hsm.invalidatePatternInTier(ctx, TierL3Remote, pattern)
		results <- struct {
			tier  TierType
			count int
			err   error
		}{TierL3Remote, count, err}
	}()
	
	// Collect results
	for i := 0; i < 3; i++ {
		res := <-results
		if res.err != nil {
			result.Errors = append(result.Errors, res.err)
		} else {
			result.TierResults[res.tier] = res.count
			result.TotalInvalidated += res.count
		}
	}
	
	result.Duration = time.Since(start)
	result.Details["pattern"] = pattern
	
	hsm.stats.invalidations.Add(1)
	return result, nil
}

// GetSystemStats returns comprehensive system statistics
func (hsm *HybridStorageManager) GetSystemStats() *SystemStats {
	stats := &SystemStats{
		TierStats:      make(map[TierType]*TierStats),
		TotalRequests:  hsm.stats.totalRequests.Load(),
		LastUpdate:     time.Now(),
		SystemUptime:   time.Since(hsm.stats.startTime),
	}
	
	// Get tier statistics
	hsm.tierMu.RLock()
	if hsm.l1Memory != nil {
		tierStats := hsm.l1Memory.GetStats()
		stats.TierStats[TierL1Memory] = &tierStats
	}
	if hsm.l2Disk != nil {
		tierStats := hsm.l2Disk.GetStats()
		stats.TierStats[TierL2Disk] = &tierStats
	}
	if hsm.l3Remote != nil {
		tierStats := hsm.l3Remote.GetStats()
		stats.TierStats[TierL3Remote] = &tierStats
	}
	hsm.tierMu.RUnlock()
	
	// Calculate overall hit rate
	totalHits := hsm.stats.l1Hits.Load() + hsm.stats.l2Hits.Load() + hsm.stats.l3Hits.Load()
	totalRequests := hsm.stats.totalRequests.Load()
	if totalRequests > 0 {
		stats.OverallHitRate = float64(totalHits) / float64(totalRequests)
	}
	
	// Calculate average latency
	if totalRequests > 0 {
		stats.AvgLatency = time.Duration(hsm.stats.totalLatency.Load() / totalRequests)
	}
	
	// Calculate total capacity
	for _, tierStats := range stats.TierStats {
		stats.TotalCapacity += tierStats.TotalCapacity
		stats.TotalUsed += tierStats.UsedCapacity
	}
	
	// Add cross-tier operation statistics
	stats.PromotionStats = &PromotionStats{
		TotalPromotions: hsm.stats.promotions.Load(),
	}
	stats.EvictionStats = &EvictionStats{
		TotalEvictions: hsm.stats.evictions.Load(),
	}
	
	return stats
}

// GetSystemHealth returns comprehensive system health information
func (hsm *HybridStorageManager) GetSystemHealth() *SystemHealth {
	hsm.health.mu.RLock()
	defer hsm.health.mu.RUnlock()
	
	health := &SystemHealth{
		Healthy:           hsm.health.healthy,
		OverallStatus:     hsm.health.systemStatus,
		TierHealth:        make(map[TierType]*TierHealth),
		SystemIssues:      hsm.health.issues,
		LastHealthCheck:   hsm.health.lastCheck,
		HealthScore:       hsm.health.overallScore,
		Recommendations:   hsm.health.recommendations,
	}
	
	// Copy tier health information
	for tierType, tierHealth := range hsm.health.tierHealth {
		health.TierHealth[tierType] = tierHealth
	}
	
	return health
}

// GetTierStats returns statistics for a specific tier
func (hsm *HybridStorageManager) GetTierStats(tierType TierType) *TierStats {
	hsm.tierMu.RLock()
	defer hsm.tierMu.RUnlock()
	
	var tier StorageTier
	switch tierType {
	case TierL1Memory:
		tier = hsm.l1Memory
	case TierL2Disk:
		tier = hsm.l2Disk
	case TierL3Remote:
		tier = hsm.l3Remote
	default:
		return nil
	}
	
	if tier == nil {
		return nil
	}
	
	stats := tier.GetStats()
	return &stats
}

// UpdateStrategy updates the promotion strategy
func (hsm *HybridStorageManager) UpdateStrategy(strategy PromotionStrategy) error {
	hsm.strategyMu.Lock()
	defer hsm.strategyMu.Unlock()
	
	hsm.promotionStrategy = strategy
	return nil
}

// UpdateEvictionPolicy updates the eviction policy
func (hsm *HybridStorageManager) UpdateEvictionPolicy(policy EvictionPolicy) error {
	hsm.strategyMu.Lock()
	defer hsm.strategyMu.Unlock()
	
	hsm.evictionPolicy = policy
	return nil
}

// Shutdown gracefully shuts down the hybrid storage manager
func (hsm *HybridStorageManager) Shutdown(ctx context.Context) error {
	var shutdownErr error
	
	hsm.shutdownOnce.Do(func() {
		// Signal shutdown
		close(hsm.stopCh)
		
		// Wait for background goroutines to finish
		hsm.backgroundWg.Wait()
		
		// Close storage tiers
		hsm.tierMu.Lock()
		defer hsm.tierMu.Unlock()
		
		if hsm.l1Memory != nil {
			if err := hsm.l1Memory.Close(); err != nil {
				shutdownErr = fmt.Errorf("failed to close L1 tier: %w", err)
			}
		}
		if hsm.l2Disk != nil {
			if err := hsm.l2Disk.Close(); err != nil {
				shutdownErr = fmt.Errorf("failed to close L2 tier: %w", err)
			}
		}
		if hsm.l3Remote != nil {
			if err := hsm.l3Remote.Close(); err != nil {
				shutdownErr = fmt.Errorf("failed to close L3 tier: %w", err)
			}
		}
		
		hsm.started.Store(false)
	})
	
	return shutdownErr
}

// Helper methods

// initializeTiers initializes all storage tiers
func (hsm *HybridStorageManager) initializeTiers(ctx context.Context) error {
	hsm.tierMu.RLock()
	defer hsm.tierMu.RUnlock()
	
	if hsm.l1Memory != nil {
		if err := hsm.l1Memory.Initialize(ctx, *hsm.config.L1Config); err != nil {
			return fmt.Errorf("failed to initialize L1 tier: %w", err)
		}
	}
	
	if hsm.l2Disk != nil {
		if err := hsm.l2Disk.Initialize(ctx, *hsm.config.L2Config); err != nil {
			return fmt.Errorf("failed to initialize L2 tier: %w", err)
		}
	}
	
	if hsm.l3Remote != nil {
		if err := hsm.l3Remote.Initialize(ctx, *hsm.config.L3Config); err != nil {
			return fmt.Errorf("failed to initialize L3 tier: %w", err)
		}
	}
	
	return nil
}

// initializeStrategies initializes promotion and eviction strategies
func (hsm *HybridStorageManager) initializeStrategies() error {
	// Initialize default hybrid promotion strategy if not set
	if hsm.promotionStrategy == nil {
		hsm.promotionStrategy = NewHybridPromotionStrategy(hsm.config.PromotionConfig)
	}
	
	// Initialize default LRU eviction policy if not set
	if hsm.evictionPolicy == nil {
		hsm.evictionPolicy = NewLRUEvictionPolicy(hsm.config.EvictionConfig)
	}
	
	// Initialize access pattern tracker if not set
	if hsm.accessTracker == nil {
		hsm.accessTracker = NewAccessPatternTracker(hsm.config.AccessConfig)
	}
	
	return nil
}

// startBackgroundProcessing starts background goroutines for various tasks
func (hsm *HybridStorageManager) startBackgroundProcessing() {
	// Start background workers
	for i := 0; i < hsm.config.BackgroundWorkers; i++ {
		hsm.backgroundWg.Add(1)
		go hsm.backgroundWorker()
	}
	
	// Start periodic maintenance
	hsm.backgroundWg.Add(1)
	go hsm.periodicMaintenance()
	
	// Start health monitoring
	hsm.backgroundWg.Add(1)
	go hsm.healthMonitor()
	
	// Start metrics collection
	hsm.backgroundWg.Add(1)
	go hsm.metricsCollector()
	
	// Start promotion processor
	hsm.backgroundWg.Add(1)
	go hsm.promotionProcessor()
	
	// Start eviction processor
	hsm.backgroundWg.Add(1)
	go hsm.evictionProcessor()
}

// backgroundWorker processes background requests
func (hsm *HybridStorageManager) backgroundWorker() {
	defer hsm.backgroundWg.Done()
	
	for {
		select {
		case <-hsm.stopCh:
			return
		case req := <-hsm.maintenanceCh:
			result := hsm.performMaintenance(req.ctx)
			select {
			case req.responseC <- result:
			case <-req.ctx.Done():
			}
		}
	}
}

// periodicMaintenance performs periodic maintenance tasks
func (hsm *HybridStorageManager) periodicMaintenance() {
	defer hsm.backgroundWg.Done()
	
	ticker := time.NewTicker(hsm.config.MaintenanceInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-hsm.stopCh:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			hsm.performBackgroundMaintenance(ctx)
			cancel()
		}
	}
}

// healthMonitor monitors system health
func (hsm *HybridStorageManager) healthMonitor() {
	defer hsm.backgroundWg.Done()
	
	ticker := time.NewTicker(hsm.config.HealthCheckInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-hsm.stopCh:
			return
		case <-ticker.C:
			hsm.updateSystemHealth()
		case update := <-hsm.healthUpdateCh:
			hsm.updateTierHealth(update.tierType, update.health)
		}
	}
}

// metricsCollector collects and processes metrics
func (hsm *HybridStorageManager) metricsCollector() {
	defer hsm.backgroundWg.Done()
	
	ticker := time.NewTicker(hsm.config.MetricsInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-hsm.stopCh:
			return
		case <-ticker.C:
			hsm.collectMetrics()
		}
	}
}

// promotionProcessor handles background promotions
func (hsm *HybridStorageManager) promotionProcessor() {
	defer hsm.backgroundWg.Done()
	
	ticker := time.NewTicker(hsm.config.PromotionInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-hsm.stopCh:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			hsm.performBackgroundPromotions(ctx)
			cancel()
		case req := <-hsm.promotionCh:
			result := hsm.processPromotionRequest(req.ctx, req.tier, req.maxItems)
			select {
			case req.responseC <- result:
			case <-req.ctx.Done():
			}
		}
	}
}

// evictionProcessor handles background evictions
func (hsm *HybridStorageManager) evictionProcessor() {
	defer hsm.backgroundWg.Done()
	
	ticker := time.NewTicker(hsm.config.EvictionInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-hsm.stopCh:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			hsm.performBackgroundEvictions(ctx)
			cancel()
		case req := <-hsm.evictionCh:
			result := hsm.processEvictionRequest(req.ctx, req.tier, req.requiredSpace)
			select {
			case req.responseC <- result:
			case <-req.ctx.Done():
			}
		}
	}
}

// Tier operation methods

// tryGetFromTier attempts to get an entry from a specific tier with circuit breaker protection
func (hsm *HybridStorageManager) tryGetFromTier(ctx context.Context, tierType TierType, key string) (*CacheEntry, error) {
	circuitBreaker := hsm.getCircuitBreaker(tierType)
	if circuitBreaker != nil && !circuitBreaker.canExecute() {
		return nil, errors.New("circuit breaker open")
	}
	
	start := time.Now()
	entry, err := hsm.getFromTier(ctx, tierType, key)
	latency := time.Since(start)
	
	if circuitBreaker != nil {
		if err != nil {
			circuitBreaker.onFailure()
		} else {
			circuitBreaker.onSuccess()
		}
	}
	
	// Record tier-specific metrics
	switch tierType {
	case TierL1Memory:
		hsm.stats.l1Latency.Add(int64(latency))
		if err != nil {
			hsm.stats.l1Errors.Add(1)
		}
	case TierL2Disk:
		hsm.stats.l2Latency.Add(int64(latency))
		if err != nil {
			hsm.stats.l2Errors.Add(1)
		}
	case TierL3Remote:
		hsm.stats.l3Latency.Add(int64(latency))
		if err != nil {
			hsm.stats.l3Errors.Add(1)
		}
	}
	
	return entry, err
}

// getFromTier gets an entry from a specific tier
func (hsm *HybridStorageManager) getFromTier(ctx context.Context, tierType TierType, key string) (*CacheEntry, error) {
	hsm.tierMu.RLock()
	defer hsm.tierMu.RUnlock()
	
	var tier StorageTier
	switch tierType {
	case TierL1Memory:
		tier = hsm.l1Memory
	case TierL2Disk:
		tier = hsm.l2Disk
	case TierL3Remote:
		tier = hsm.l3Remote
	default:
		return nil, fmt.Errorf("unknown tier type: %s", tierType)
	}
	
	if tier == nil {
		return nil, fmt.Errorf("tier %s not available", tierType)
	}
	
	return tier.Get(ctx, key)
}

// putInTier stores an entry in a specific tier
func (hsm *HybridStorageManager) putInTier(ctx context.Context, tierType TierType, key string, entry *CacheEntry) error {
	hsm.tierMu.RLock()
	defer hsm.tierMu.RUnlock()
	
	var tier StorageTier
	switch tierType {
	case TierL1Memory:
		tier = hsm.l1Memory
	case TierL2Disk:
		tier = hsm.l2Disk
	case TierL3Remote:
		tier = hsm.l3Remote
	default:
		return fmt.Errorf("unknown tier type: %s", tierType)
	}
	
	if tier == nil {
		return fmt.Errorf("tier %s not available", tierType)
	}
	
	// Update entry metadata
	entry.CurrentTier = tierType
	if entry.OriginTier == 0 {
		entry.OriginTier = tierType
	}
	
	return tier.Put(ctx, key, entry)
}

// deleteFromTier deletes an entry from a specific tier
func (hsm *HybridStorageManager) deleteFromTier(ctx context.Context, tierType TierType, key string) error {
	hsm.tierMu.RLock()
	defer hsm.tierMu.RUnlock()
	
	var tier StorageTier
	switch tierType {
	case TierL1Memory:
		tier = hsm.l1Memory
	case TierL2Disk:
		tier = hsm.l2Disk
	case TierL3Remote:
		tier = hsm.l3Remote
	default:
		return fmt.Errorf("unknown tier type: %s", tierType)
	}
	
	if tier == nil {
		return fmt.Errorf("tier %s not available", tierType)
	}
	
	return tier.Delete(ctx, key)
}

// Intelligent routing and decision methods

// determineInitialTier determines the best initial tier for a new entry
func (hsm *HybridStorageManager) determineInitialTier(entry *CacheEntry) TierType {
	// Check caching hint
	switch entry.CachingHint {
	case CachingHintPreferMemory:
		return TierL1Memory
	case CachingHintPreferDisk:
		return TierL2Disk
	case CachingHintPreferRemote:
		return TierL3Remote
	case CachingHintNoCache:
		return TierL3Remote
	case CachingHintPinned:
		return TierL1Memory
	}
	
	// Size-based decision
	if entry.Size <= 1024*1024 { // 1MB
		return TierL1Memory
	} else if entry.Size <= 100*1024*1024 { // 100MB
		return TierL2Disk
	} else {
		return TierL3Remote
	}
}

// putWithFallback attempts to store an entry with fallback to other tiers
func (hsm *HybridStorageManager) putWithFallback(ctx context.Context, key string, entry *CacheEntry, preferredTier TierType) error {
	tiers := []TierType{TierL1Memory, TierL2Disk, TierL3Remote}
	
	// Try preferred tier first
	if err := hsm.putInTier(ctx, preferredTier, key, entry); err == nil {
		return nil
	}
	
	// Try other tiers
	for _, tier := range tiers {
		if tier == preferredTier {
			continue
		}
		if err := hsm.putInTier(ctx, tier, key, entry); err == nil {
			return nil
		}
	}
	
	return errors.New("failed to store in any tier")
}

// Background operation methods

// promoteInBackground queues a promotion operation for background processing
func (hsm *HybridStorageManager) promoteInBackground(ctx context.Context, key string, entry *CacheEntry, fromTier, toTier TierType) {
	if hsm.promotionStrategy == nil || hsm.accessTracker == nil {
		return
	}
	
	// Check if promotion is beneficial
	accessPattern := hsm.getAccessPattern(key)
	if shouldPromote, targetTier := hsm.promotionStrategy.ShouldPromote(entry, accessPattern, fromTier); shouldPromote && targetTier == toTier {
		go func() {
			if err := hsm.Promote(ctx, key, fromTier, toTier); err != nil {
				hsm.stats.backgroundFailures.Add(1)
			} else {
				hsm.stats.backgroundPromotions.Add(1)
			}
		}()
	}
}

// performBackgroundPromotions performs background promotion operations
func (hsm *HybridStorageManager) performBackgroundPromotions(ctx context.Context) {
	// Get promotion candidates from L3 to L2
	if candidates, err := hsm.getPromotionCandidatesForTier(ctx, TierL3Remote, TierL2Disk); err == nil {
		hsm.processPromotionCandidates(ctx, candidates)
	}
	
	// Get promotion candidates from L2 to L1
	if candidates, err := hsm.getPromotionCandidatesForTier(ctx, TierL2Disk, TierL1Memory); err == nil {
		hsm.processPromotionCandidates(ctx, candidates)
	}
}

// performBackgroundEvictions performs background eviction operations
func (hsm *HybridStorageManager) performBackgroundEvictions(ctx context.Context) {
	// Check if any tier needs eviction
	stats := hsm.GetSystemStats()
	
	for tierType, tierStats := range stats.TierStats {
		utilizationPct := float64(tierStats.UsedCapacity) / float64(tierStats.TotalCapacity)
		
		// Trigger eviction if utilization is above threshold
		if utilizationPct > 0.8 { // 80% threshold
			requiredSpace := int64(float64(tierStats.TotalCapacity) * 0.1) // Free up 10%
			hsm.performEvictionForTier(ctx, tierType, requiredSpace)
		}
	}
}

// performBackgroundMaintenance performs background maintenance tasks
func (hsm *HybridStorageManager) performBackgroundMaintenance(ctx context.Context) {
	// Flush tiers
	hsm.flushTiers(ctx)
	
	// Clean up expired entries
	hsm.cleanupExpiredEntries(ctx)
	
	// Update access patterns
	if hsm.accessTracker != nil {
		hsm.accessTracker.PurgeOldPatterns(time.Now().Add(-24 * time.Hour))
	}
	
	// Optimize storage
	hsm.optimizeStorage(ctx)
}

// Utility methods

// getCircuitBreaker returns the circuit breaker for a tier
func (hsm *HybridStorageManager) getCircuitBreaker(tierType TierType) *tierCircuitBreaker {
	switch tierType {
	case TierL1Memory:
		return hsm.l1CircuitBreaker
	case TierL2Disk:
		return hsm.l2CircuitBreaker
	case TierL3Remote:
		return hsm.l3CircuitBreaker
	default:
		return nil
	}
}

// getAccessPattern gets access pattern for a key
func (hsm *HybridStorageManager) getAccessPattern(key string) *AccessPattern {
	if hsm.accessTracker == nil {
		return nil
	}
	// This would return the actual access pattern, simplified for brevity
	return &hsm.accessTracker
}

// triggerMaintenanceIfNeeded triggers maintenance if capacity thresholds are exceeded
func (hsm *HybridStorageManager) triggerMaintenanceIfNeeded(ctx context.Context) {
	// Check if maintenance is needed based on capacity or other metrics
	stats := hsm.GetSystemStats()
	
	for _, tierStats := range stats.TierStats {
		utilizationPct := float64(tierStats.UsedCapacity) / float64(tierStats.TotalCapacity)
		if utilizationPct > 0.9 { // 90% threshold
			select {
			case hsm.maintenanceCh <- maintenanceRequest{
				requestType: "capacity_maintenance",
				responseC:   make(chan *MaintenanceResult, 1),
				ctx:         ctx,
			}:
			default:
				// Channel full, skip this maintenance request
			}
			break
		}
	}
}

// logError logs an error (simplified implementation)
func (hsm *HybridStorageManager) logError(message string, err error) {
	// In a real implementation, this would use a proper logger
	fmt.Printf("HybridStorageManager error: %s: %v\n", message, err)
}

// Placeholder methods for operations that need implementation in subsequent parts

// invalidateInTier invalidates entries in a specific tier by file path
func (hsm *HybridStorageManager) invalidateInTier(ctx context.Context, tierType TierType, filePath string) (int, error) {
	tier := hsm.getTier(tierType)
	if tier == nil {
		return 0, fmt.Errorf("tier %s not available", tierType)
	}
	return tier.InvalidateByFile(ctx, filePath)
}

// invalidateProjectInTier invalidates entries in a specific tier by project path
func (hsm *HybridStorageManager) invalidateProjectInTier(ctx context.Context, tierType TierType, projectPath string) (int, error) {
	tier := hsm.getTier(tierType)
	if tier == nil {
		return 0, fmt.Errorf("tier %s not available", tierType)
	}
	return tier.InvalidateByProject(ctx, projectPath)
}

// invalidatePatternInTier invalidates entries in a specific tier by pattern
func (hsm *HybridStorageManager) invalidatePatternInTier(ctx context.Context, tierType TierType, pattern string) (int, error) {
	tier := hsm.getTier(tierType)
	if tier == nil {
		return 0, fmt.Errorf("tier %s not available", tierType)
	}
	return tier.Invalidate(ctx, pattern)
}

// getTier returns the storage tier for a given tier type
func (hsm *HybridStorageManager) getTier(tierType TierType) StorageTier {
	hsm.tierMu.RLock()
	defer hsm.tierMu.RUnlock()
	
	switch tierType {
	case TierL1Memory:
		return hsm.l1Memory
	case TierL2Disk:
		return hsm.l2Disk
	case TierL3Remote:
		return hsm.l3Remote
	default:
		return nil
	}
}

// Additional placeholder methods that would be implemented in full version
func (hsm *HybridStorageManager) getPromotionCandidates(ctx context.Context) ([]*PromotionCandidate, error) {
	// Implementation would analyze all tiers for promotion candidates
	return nil, nil
}

func (hsm *HybridStorageManager) getEvictionCandidates(ctx context.Context) ([]*EvictionCandidate, error) {
	// Implementation would analyze all tiers for eviction candidates
	return nil, nil
}

func (hsm *HybridStorageManager) getPromotionCandidatesForTier(ctx context.Context, fromTier, toTier TierType) ([]*PromotionCandidate, error) {
	// Implementation would get promotion candidates between specific tiers
	return nil, nil
}

func (hsm *HybridStorageManager) processPromotionCandidates(ctx context.Context, candidates []*PromotionCandidate) {
	// Implementation would process promotion candidates
}

func (hsm *HybridStorageManager) performEvictionForTier(ctx context.Context, tierType TierType, requiredSpace int64) {
	// Implementation would perform eviction for a specific tier
}

func (hsm *HybridStorageManager) performMaintenance(ctx context.Context) *MaintenanceResult {
	// Implementation would perform comprehensive maintenance
	return &MaintenanceResult{}
}

func (hsm *HybridStorageManager) processPromotionRequest(ctx context.Context, tier TierType, maxItems int) *PromotionResult {
	// Implementation would process promotion requests
	return &PromotionResult{}
}

func (hsm *HybridStorageManager) processEvictionRequest(ctx context.Context, tier TierType, requiredSpace int64) *EvictionResult {
	// Implementation would process eviction requests
	return &EvictionResult{}
}

func (hsm *HybridStorageManager) updateSystemHealth() {
	// Implementation would update system health status
}

func (hsm *HybridStorageManager) updateTierHealth(tierType TierType, health *TierHealth) {
	// Implementation would update tier health
}

func (hsm *HybridStorageManager) collectMetrics() {
	// Implementation would collect and process metrics
}

func (hsm *HybridStorageManager) flushTiers(ctx context.Context) {
	// Implementation would flush all tiers
}

func (hsm *HybridStorageManager) cleanupExpiredEntries(ctx context.Context) {
	// Implementation would cleanup expired entries
}

func (hsm *HybridStorageManager) optimizeStorage(ctx context.Context) {
	// Implementation would optimize storage across tiers
}

// Placeholder strategy implementations that would be in separate files

// NewHybridPromotionStrategy creates a new hybrid promotion strategy
func NewHybridPromotionStrategy(config *PromotionParameters) PromotionStrategy {
	// Implementation would create a hybrid strategy combining LRU and LFU
	return nil
}

// NewLRUEvictionPolicy creates a new LRU eviction policy
func NewLRUEvictionPolicy(config *EvictionParameters) EvictionPolicy {
	// Implementation would create an LRU eviction policy
	return nil
}

// NewAccessPatternTracker creates a new access pattern tracker
func NewAccessPatternTracker(config *AccessTrackingParameters) AccessPattern {
	// Implementation would create an access pattern tracker
	return nil
}

// tierCircuitBreaker implements circuit breaker pattern for tier protection
type tierCircuitBreaker struct {
	tierType     TierType
	state        CircuitBreakerState
	failures     atomic.Int64
	lastFailure  time.Time
	config       *CircuitBreakerConfig
	mu           sync.RWMutex
}

// newTierCircuitBreaker creates a new tier circuit breaker
func newTierCircuitBreaker(tierType TierType, config *CircuitBreakerConfig) *tierCircuitBreaker {
	return &tierCircuitBreaker{
		tierType: tierType,
		state:    CircuitBreakerClosed,
		config:   config,
	}
}

// canExecute checks if the circuit breaker allows execution
func (cb *tierCircuitBreaker) canExecute() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	
	switch cb.state {
	case CircuitBreakerClosed:
		return true
	case CircuitBreakerOpen:
		// Check if recovery timeout has passed
		if time.Since(cb.lastFailure) > cb.config.RecoveryTimeout {
			cb.state = CircuitBreakerHalfOpen
			return true
		}
		return false
	case CircuitBreakerHalfOpen:
		return true
	default:
		return false
	}
}

// onSuccess records a successful operation
func (cb *tierCircuitBreaker) onSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	if cb.state == CircuitBreakerHalfOpen {
		cb.state = CircuitBreakerClosed
		cb.failures.Store(0)
	}
}

// onFailure records a failed operation
func (cb *tierCircuitBreaker) onFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	cb.failures.Add(1)
	cb.lastFailure = time.Now()
	
	if cb.failures.Load() >= int64(cb.config.FailureThreshold) {
		cb.state = CircuitBreakerOpen
	}
}