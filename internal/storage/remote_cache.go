package storage

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/klauspost/compress/zstd"
	"github.com/vmihailenco/msgpack/v5"
	"lsp-gateway/internal/gateway"
)

// RemoteCache implements the L3 remote storage tier for distributed cold data
// Provides <200ms access time with multiple backend support and network resilience
type RemoteCache struct {
	// Configuration
	config      TierConfig
	backendType BackendType

	// Backend implementations
	backends      map[string]RemoteBackend
	activeBackend string
	backendMu     sync.RWMutex

	// Network resilience
	httpClient     *http.Client
	circuitBreaker *gateway.CircuitBreaker
	retryConfig    *RetryConfig

	// Compression and serialization
	compression CompressionProvider
	serializer  SerializationProvider

	// Connection management
	connectionPool *ConnectionPool

	// Background operations
	syncManager *BackgroundSyncManager

	// Statistics and monitoring
	stats    *remoteStats
	health   *remoteHealth
	capacity *remoteCapacity

	// Concurrency control
	mu     sync.RWMutex
	closed int32

	// Background processes
	maintenanceCancel context.CancelFunc
	maintenanceDone   chan struct{}

	// Authentication
	authProvider AuthProvider

	// Performance tracking
	latencyHistogram  map[string]*LatencyTracker
	throughputTracker *ThroughputTracker
	networkMonitor    *NetworkMonitor
}

// RemoteBackend defines the interface for different remote storage backends
type RemoteBackend interface {
	// Backend identification
	GetBackendType() BackendType
	GetEndpoint() string

	// Core operations with context support
	Get(ctx context.Context, key string) ([]byte, *StorageMetadata, error)
	Put(ctx context.Context, key string, data []byte, metadata *StorageMetadata) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)

	// Batch operations for efficiency
	GetBatch(ctx context.Context, keys []string) (map[string][]byte, error)
	PutBatch(ctx context.Context, entries map[string][]byte) error
	DeleteBatch(ctx context.Context, keys []string) error

	// Health and diagnostics
	Ping(ctx context.Context) error
	GetStats(ctx context.Context) (*BackendStats, error)

	// Lifecycle management
	Initialize(ctx context.Context, config BackendConfig) error
	Close() error
}

// HTTPRestBackend implements HTTP REST API backend
type HTTPRestBackend struct {
	endpoint    string
	client      *http.Client
	auth        AuthProvider
	compression bool
	headers     map[string]string
	timeout     time.Duration
	mu          sync.RWMutex
}

// S3Backend implements S3-compatible storage backend
type S3Backend struct {
	endpoint  string
	bucket    string
	region    string
	accessKey string
	secretKey string
	client    *http.Client
	useSSL    bool
	pathStyle bool
	mu        sync.RWMutex
}

// RedisBackend implements Redis cluster backend
type RedisBackend struct {
	client      *redis.ClusterClient
	keyPrefix   string
	db          int
	timeout     time.Duration
	compression bool
	mu          sync.RWMutex
}

// CustomBackend allows for extensible custom protocols
type CustomBackend struct {
	protocol string
	endpoint string
	handler  CustomProtocolHandler
	config   map[string]interface{}
	mu       sync.RWMutex
}

// RetryConfig defines retry behavior for network operations
type RetryConfig struct {
	MaxRetries      int           `json:"max_retries"`
	InitialDelay    time.Duration `json:"initial_delay"`
	MaxDelay        time.Duration `json:"max_delay"`
	BackoffFactor   float64       `json:"backoff_factor"`
	JitterFactor    float64       `json:"jitter_factor"`
	RetryableErrors []string      `json:"retryable_errors"`
}

// ConnectionPool manages HTTP/TCP connections efficiently
type ConnectionPool struct {
	maxConnections    int
	activeConnections int32
	idleTimeout       time.Duration
	keepAlive         time.Duration
	connectionMap     map[string]*PooledConnection
	mu                sync.RWMutex
}

// PooledConnection represents a reusable network connection
type PooledConnection struct {
	conn     net.Conn
	lastUsed time.Time
	inUse    bool
	backend  string
}

// BackgroundSyncManager handles asynchronous operations
type BackgroundSyncManager struct {
	uploadQueue   chan *SyncOperation
	downloadQueue chan *SyncOperation
	workers       int
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
}

// SyncOperation represents a background sync operation
type SyncOperation struct {
	Type     SyncOperationType
	Key      string
	Data     []byte
	Metadata *StorageMetadata
	Callback func(error)
	Priority int
	Retries  int
}

// SyncOperationType defines types of sync operations
type SyncOperationType int

const (
	SyncTypeUpload SyncOperationType = iota
	SyncTypeDownload
	SyncTypeDelete
	SyncTypeVerify
)

// SerializationProvider handles data serialization for network transport
type SerializationProvider interface {
	Serialize(data interface{}) ([]byte, error)
	Deserialize(data []byte, target interface{}) error
	GetFormat() SerializationFormat
}

// SerializationFormat represents different serialization formats
type SerializationFormat int

const (
	SerializationJSON SerializationFormat = iota
	SerializationMsgPack
	SerializationProtobuf
	SerializationAvro
)

// AuthProvider handles authentication for remote backends
type AuthProvider interface {
	GetAuthHeaders() map[string]string
	GetAuthToken(ctx context.Context) (string, error)
	RefreshToken(ctx context.Context) error
	IsTokenExpired() bool
}

// CustomProtocolHandler allows custom protocol implementations
type CustomProtocolHandler interface {
	Execute(ctx context.Context, operation string, data []byte) ([]byte, error)
	HealthCheck(ctx context.Context) error
	Configure(config map[string]interface{}) error
}

// Statistics and monitoring types
type remoteStats struct {
	mu sync.RWMutex

	// Request statistics
	totalRequests   int64
	cacheHits       int64
	cacheMisses     int64
	networkRequests int64

	// Performance metrics
	avgLatency time.Duration
	p50Latency time.Duration
	p95Latency time.Duration
	p99Latency time.Duration
	maxLatency time.Duration

	// Network metrics
	bytesTransferred int64
	networkErrors    int64
	timeoutErrors    int64
	retryCount       int64

	// Backend metrics
	backendSwitches  int64
	compressionRatio float64

	// Operation counts
	getOperations    int64
	putOperations    int64
	deleteOperations int64
	batchOperations  int64

	// Background sync metrics
	syncOperations int64
	syncErrors     int64
	queueSize      int64

	startTime time.Time
}

type remoteHealth struct {
	mu sync.RWMutex

	healthy   bool
	status    TierStatus
	lastCheck time.Time
	issues    []HealthIssue

	// Backend health
	backendHealth  map[string]bool
	activeBackends []string

	// Network health
	networkLatency time.Duration
	packetLoss     float64

	// Circuit breaker state
	circuitBreaker struct {
		state        gateway.CircuitBreakerState
		failureCount int
		lastFailure  time.Time
		nextRetry    time.Time
	}
}

type remoteCapacity struct {
	mu sync.RWMutex

	maxCapacity       int64
	usedCapacity      int64
	availableCapacity int64
	maxEntries        int64
	usedEntries       int64
	utilizationPct    float64

	// Network capacity
	bandwidth         int64
	maxThroughput     float64
	currentThroughput float64
}

// LatencyTracker tracks latency percentiles with efficient histogram
type LatencyTracker struct {
	buckets      []int64
	bucketRanges []time.Duration
	totalSamples int64
	mu           sync.RWMutex
}

// ThroughputTracker tracks throughput over time windows
type ThroughputTracker struct {
	requestCounts []int64
	byteCounts    []int64
	timeWindows   []time.Time
	windowSize    time.Duration
	mu            sync.RWMutex
}

// NetworkMonitor tracks network-specific metrics
type NetworkMonitor struct {
	latencyHistory []time.Duration
	errorRates     map[string]float64
	retryStats     map[string]int64
	mu             sync.RWMutex
}

// BackendStats contains backend-specific statistics
type BackendStats struct {
	Endpoint         string        `json:"endpoint"`
	Healthy          bool          `json:"healthy"`
	ResponseTime     time.Duration `json:"response_time"`
	ErrorRate        float64       `json:"error_rate"`
	RequestCount     int64         `json:"request_count"`
	BytesTransferred int64         `json:"bytes_transferred"`
	LastCheck        time.Time     `json:"last_check"`
}

// NewRemoteCache creates a new L3 remote storage cache instance
func NewRemoteCache(config TierConfig) (*RemoteCache, error) {
	if config.TierType != TierL3Remote {
		return nil, fmt.Errorf("invalid tier type for remote cache: %v", config.TierType)
	}

	// Create HTTP client with optimized settings
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
		},
		DisableKeepAlives:      false,
		DisableCompression:     false,
		MaxResponseHeaderBytes: 1 << 20, // 1MB
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   time.Duration(config.TimeoutMs) * time.Millisecond,
	}

	// Initialize circuit breaker with configuration
	var circuitBreaker *gateway.CircuitBreaker
	if config.BackendConfig.CircuitBreaker != nil {
		circuitBreaker = gateway.NewCircuitBreaker(
			config.BackendConfig.CircuitBreaker.FailureThreshold,
			config.BackendConfig.CircuitBreaker.RecoveryTimeout,
			int32(config.BackendConfig.CircuitBreaker.HalfOpenRequests),
		)
	} else {
		// Use default circuit breaker configuration
		circuitBreaker = gateway.NewCircuitBreaker(
			10,           // Default failure threshold
			30*time.Second, // Default recovery timeout
			5,            // Default half-open requests
		)
	}

	// Initialize retry configuration
	retryConfig := &RetryConfig{
		MaxRetries:    config.RetryCount,
		InitialDelay:  time.Duration(config.RetryDelayMs) * time.Millisecond,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
		JitterFactor:  0.1,
		RetryableErrors: []string{
			"connection timeout",
			"connection refused",
			"temporary failure",
			"503",
			"502",
			"504",
		},
	}

	rc := &RemoteCache{
		config:         config,
		backendType:    config.BackendType,
		backends:       make(map[string]RemoteBackend),
		httpClient:     httpClient,
		circuitBreaker: circuitBreaker,
		retryConfig:    retryConfig,
		stats: &remoteStats{
			startTime: time.Now(),
		},
		health: &remoteHealth{
			healthy:       true,
			status:        TierStatusHealthy,
			lastCheck:     time.Now(),
			backendHealth: make(map[string]bool),
		},
		capacity: &remoteCapacity{
			maxCapacity: config.MaxCapacity,
			maxEntries:  config.MaxEntries,
		},
		latencyHistogram: make(map[string]*LatencyTracker),
		maintenanceDone:  make(chan struct{}),
	}

	// Initialize compression provider
	if config.CompressionConfig != nil && config.CompressionConfig.Enabled {
		rc.compression = NewCompressionProvider(config.CompressionConfig)
	}

	// Initialize serialization provider
	rc.serializer = NewMsgPackSerializer()

	// Initialize connection pool
	rc.connectionPool = &ConnectionPool{
		maxConnections: config.BackendConfig.MaxConnections,
		idleTimeout:    60 * time.Second,
		keepAlive:      30 * time.Second,
		connectionMap:  make(map[string]*PooledConnection),
	}

	// Initialize background sync manager
	ctx, cancel := context.WithCancel(context.Background())
	rc.syncManager = &BackgroundSyncManager{
		uploadQueue:   make(chan *SyncOperation, 1000),
		downloadQueue: make(chan *SyncOperation, 1000),
		workers:       4,
		ctx:           ctx,
		cancel:        cancel,
	}

	// Initialize performance trackers
	rc.throughputTracker = &ThroughputTracker{
		windowSize: 5 * time.Minute,
	}
	rc.networkMonitor = &NetworkMonitor{
		errorRates: make(map[string]float64),
		retryStats: make(map[string]int64),
	}

	return rc, nil
}

// Initialize implements StorageTier interface
func (rc *RemoteCache) Initialize(ctx context.Context, config TierConfig) error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	rc.config = config

	// Initialize backends based on configuration
	if err := rc.initializeBackends(ctx); err != nil {
		return fmt.Errorf("failed to initialize backends: %w", err)
	}

	// Start background sync workers
	rc.startBackgroundWorkers()

	// Start maintenance routine
	maintenanceCtx, cancel := context.WithCancel(ctx)
	rc.maintenanceCancel = cancel
	go rc.maintenanceRoutine(maintenanceCtx)

	return nil
}

// Get implements StorageTier interface with network resilience
func (rc *RemoteCache) Get(ctx context.Context, key string) (*CacheEntry, error) {
	startTime := time.Now()
	defer func() {
		latency := time.Since(startTime)
		rc.recordLatency("get", latency)
		atomic.AddInt64(&rc.stats.getOperations, 1)
		atomic.AddInt64(&rc.stats.totalRequests, 1)
	}()

	// Check circuit breaker
	if !rc.circuitBreaker.CanExecute() {
		atomic.AddInt64(&rc.stats.cacheMisses, 1)
		return nil, errors.New("circuit breaker is open")
	}

	// Execute with retry logic
	var entry *CacheEntry
	var err error
	err = rc.executeWithRetry(ctx, "get", func() error {
		entry, err = rc.performGet(ctx, key)
		return err
	})

	if err != nil {
		rc.circuitBreaker.RecordFailure()
		atomic.AddInt64(&rc.stats.cacheMisses, 1)
		return nil, err
	}

	rc.circuitBreaker.RecordSuccess()
	atomic.AddInt64(&rc.stats.cacheHits, 1)

	// Update access patterns
	if entry != nil {
		entry.Touch()
	}

	return entry, nil
}

// Put implements StorageTier interface with background sync
func (rc *RemoteCache) Put(ctx context.Context, key string, entry *CacheEntry) error {
	startTime := time.Now()
	defer func() {
		latency := time.Since(startTime)
		rc.recordLatency("put", latency)
		atomic.AddInt64(&rc.stats.putOperations, 1)
		atomic.AddInt64(&rc.stats.totalRequests, 1)
	}()

	// Check circuit breaker
	if !rc.circuitBreaker.CanExecute() {
		return errors.New("circuit breaker is open")
	}

	// For cold data, use background sync for better performance
	if entry.CachingHint == CachingHintPreferRemote || entry.Priority < 50 {
		return rc.putAsync(ctx, key, entry)
	}

	// Execute synchronously with retry logic
	err := rc.executeWithRetry(ctx, "put", func() error {
		return rc.performPut(ctx, key, entry)
	})

	if err != nil {
		rc.circuitBreaker.RecordFailure()
		return err
	}

	rc.circuitBreaker.RecordSuccess()
	return nil
}

// Delete implements StorageTier interface
func (rc *RemoteCache) Delete(ctx context.Context, key string) error {
	startTime := time.Now()
	defer func() {
		latency := time.Since(startTime)
		rc.recordLatency("delete", latency)
		atomic.AddInt64(&rc.stats.deleteOperations, 1)
		atomic.AddInt64(&rc.stats.totalRequests, 1)
	}()

	// Check circuit breaker
	if !rc.circuitBreaker.CanExecute() {
		return errors.New("circuit breaker is open")
	}

	// Execute with retry logic
	err := rc.executeWithRetry(ctx, "delete", func() error {
		return rc.performDelete(ctx, key)
	})

	if err != nil {
		rc.circuitBreaker.RecordFailure()
		return err
	}

	rc.circuitBreaker.RecordSuccess()
	return nil
}

// Exists implements StorageTier interface
func (rc *RemoteCache) Exists(ctx context.Context, key string) (bool, error) {
	backend := rc.getActiveBackend()
	if backend == nil {
		return false, errors.New("no active backend available")
	}

	return backend.Exists(ctx, key)
}

// GetBatch implements StorageTier interface with optimized network usage
func (rc *RemoteCache) GetBatch(ctx context.Context, keys []string) (map[string]*CacheEntry, error) {
	startTime := time.Now()
	defer func() {
		latency := time.Since(startTime)
		rc.recordLatency("get_batch", latency)
		atomic.AddInt64(&rc.stats.batchOperations, 1)
		atomic.AddInt64(&rc.stats.totalRequests, 1)
	}()

	if len(keys) == 0 {
		return make(map[string]*CacheEntry), nil
	}

	// Check circuit breaker
	if !rc.circuitBreaker.CanExecute() {
		atomic.AddInt64(&rc.stats.cacheMisses, int64(len(keys)))
		return nil, errors.New("circuit breaker is open")
	}

	var result map[string]*CacheEntry
	var err error

	err = rc.executeWithRetry(ctx, "get_batch", func() error {
		result, err = rc.performGetBatch(ctx, keys)
		return err
	})

	if err != nil {
		rc.circuitBreaker.RecordFailure()
		atomic.AddInt64(&rc.stats.cacheMisses, int64(len(keys)))
		return nil, err
	}

	rc.circuitBreaker.RecordSuccess()

	// Update hit/miss statistics
	hits := int64(len(result))
	misses := int64(len(keys)) - hits
	atomic.AddInt64(&rc.stats.cacheHits, hits)
	atomic.AddInt64(&rc.stats.cacheMisses, misses)

	return result, nil
}

// PutBatch implements StorageTier interface with optimized network usage
func (rc *RemoteCache) PutBatch(ctx context.Context, entries map[string]*CacheEntry) error {
	startTime := time.Now()
	defer func() {
		latency := time.Since(startTime)
		rc.recordLatency("put_batch", latency)
		atomic.AddInt64(&rc.stats.batchOperations, 1)
		atomic.AddInt64(&rc.stats.totalRequests, 1)
	}()

	if len(entries) == 0 {
		return nil
	}

	// Check circuit breaker
	if !rc.circuitBreaker.CanExecute() {
		return errors.New("circuit breaker is open")
	}

	// Execute with retry logic
	err := rc.executeWithRetry(ctx, "put_batch", func() error {
		return rc.performPutBatch(ctx, entries)
	})

	if err != nil {
		rc.circuitBreaker.RecordFailure()
		return err
	}

	rc.circuitBreaker.RecordSuccess()
	return nil
}

// DeleteBatch implements StorageTier interface
func (rc *RemoteCache) DeleteBatch(ctx context.Context, keys []string) error {
	startTime := time.Now()
	defer func() {
		latency := time.Since(startTime)
		rc.recordLatency("delete_batch", latency)
		atomic.AddInt64(&rc.stats.batchOperations, 1)
		atomic.AddInt64(&rc.stats.totalRequests, 1)
	}()

	if len(keys) == 0 {
		return nil
	}

	// Check circuit breaker
	if !rc.circuitBreaker.CanExecute() {
		return errors.New("circuit breaker is open")
	}

	// Execute with retry logic
	err := rc.executeWithRetry(ctx, "delete_batch", func() error {
		return rc.performDeleteBatch(ctx, keys)
	})

	if err != nil {
		rc.circuitBreaker.RecordFailure()
		return err
	}

	rc.circuitBreaker.RecordSuccess()
	return nil
}

// Invalidate implements StorageTier interface with pattern matching
func (rc *RemoteCache) Invalidate(ctx context.Context, pattern string) (int, error) {
	// For remote cache, pattern invalidation requires listing keys first
	// This is less efficient but necessary for distributed systems
	backend := rc.getActiveBackend()
	if backend == nil {
		return 0, errors.New("no active backend available")
	}

	// Implementation depends on backend capabilities
	// Some backends support pattern deletion, others require enumeration
	return rc.performPatternInvalidation(ctx, pattern)
}

// InvalidateByFile implements StorageTier interface
func (rc *RemoteCache) InvalidateByFile(ctx context.Context, filePath string) (int, error) {
	// Use file path as invalidation pattern
	pattern := fmt.Sprintf("*%s*", filePath)
	return rc.Invalidate(ctx, pattern)
}

// InvalidateByProject implements StorageTier interface
func (rc *RemoteCache) InvalidateByProject(ctx context.Context, projectPath string) (int, error) {
	// Use project path as invalidation pattern
	pattern := fmt.Sprintf("%s:*", projectPath)
	return rc.Invalidate(ctx, pattern)
}

// Clear implements StorageTier interface
func (rc *RemoteCache) Clear(ctx context.Context) error {
	backend := rc.getActiveBackend()
	if backend == nil {
		return errors.New("no active backend available")
	}

	// Clear implementation depends on backend
	return rc.performClear(ctx)
}

// GetStats implements StorageTier interface
func (rc *RemoteCache) GetStats() TierStats {
	rc.stats.mu.RLock()
	defer rc.stats.mu.RUnlock()

	hitRate := float64(0)
	if rc.stats.totalRequests > 0 {
		hitRate = float64(rc.stats.cacheHits) / float64(rc.stats.totalRequests)
	}

	return TierStats{
		TierType:      TierL3Remote,
		TotalCapacity: rc.capacity.maxCapacity,
		UsedCapacity:  rc.capacity.usedCapacity,
		FreeCapacity:  rc.capacity.availableCapacity,
		EntryCount:    rc.capacity.usedEntries,
		TotalRequests: rc.stats.totalRequests,
		CacheHits:     rc.stats.cacheHits,
		CacheMisses:   rc.stats.cacheMisses,
		HitRate:       hitRate,
		AvgLatency:    rc.stats.avgLatency,
		P50Latency:    rc.stats.p50Latency,
		P95Latency:    rc.stats.p95Latency,
		P99Latency:    rc.stats.p99Latency,
		MaxLatency:    rc.stats.maxLatency,
		ErrorCount:    rc.stats.networkErrors,
		ErrorRate:     rc.calculateErrorRate(),
		StartTime:     rc.stats.startTime,
		LastUpdate:    time.Now(),
		Uptime:        time.Since(rc.stats.startTime),
	}
}

// GetHealth implements StorageTier interface
func (rc *RemoteCache) GetHealth() TierHealth {
	rc.health.mu.RLock()
	defer rc.health.mu.RUnlock()

	return TierHealth{
		TierType:  TierL3Remote,
		Healthy:   rc.health.healthy,
		Status:    rc.health.status,
		LastCheck: rc.health.lastCheck,
		Issues:    append([]HealthIssue{}, rc.health.issues...),
		CircuitBreaker: struct {
			State        CircuitBreakerState `json:"state"`
			FailureCount int                 `json:"failure_count"`
			LastFailure  time.Time           `json:"last_failure,omitempty"`
			NextRetry    time.Time           `json:"next_retry,omitempty"`
		}{
			State:        CircuitBreakerState(rc.circuitBreaker.GetState()),
			FailureCount: rc.health.circuitBreaker.failureCount,
			LastFailure:  rc.health.circuitBreaker.lastFailure,
			NextRetry:    rc.health.circuitBreaker.nextRetry,
		},
	}
}

// GetCapacity implements StorageTier interface
func (rc *RemoteCache) GetCapacity() TierCapacity {
	rc.capacity.mu.RLock()
	defer rc.capacity.mu.RUnlock()

	return TierCapacity{
		TierType:          TierL3Remote,
		MaxCapacity:       rc.capacity.maxCapacity,
		UsedCapacity:      rc.capacity.usedCapacity,
		AvailableCapacity: rc.capacity.availableCapacity,
		MaxEntries:        rc.capacity.maxEntries,
		UsedEntries:       rc.capacity.usedEntries,
		UtilizationPct:    rc.capacity.utilizationPct,
	}
}

// Flush implements StorageTier interface
func (rc *RemoteCache) Flush(ctx context.Context) error {
	// Wait for background sync operations to complete
	rc.syncManager.wg.Wait()

	// Force synchronization of any pending operations
	return rc.forceSyncPendingOperations(ctx)
}

// Close implements StorageTier interface
func (rc *RemoteCache) Close() error {
	if !atomic.CompareAndSwapInt32(&rc.closed, 0, 1) {
		return nil // Already closed
	}

	// Stop maintenance routine
	if rc.maintenanceCancel != nil {
		rc.maintenanceCancel()
		<-rc.maintenanceDone
	}

	// Stop background sync
	rc.syncManager.cancel()
	rc.syncManager.wg.Wait()

	// Close backends
	rc.backendMu.Lock()
	for _, backend := range rc.backends {
		backend.Close()
	}
	rc.backendMu.Unlock()

	// Close HTTP client transport
	if transport, ok := rc.httpClient.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}

	return nil
}

// GetTierType implements StorageTier interface
func (rc *RemoteCache) GetTierType() TierType {
	return TierL3Remote
}

// GetTierLevel implements StorageTier interface
func (rc *RemoteCache) GetTierLevel() int {
	return 3
}

// Backend management methods

func (rc *RemoteCache) initializeBackends(ctx context.Context) error {
	switch rc.backendType {
	case BackendS3:
		backend, err := rc.createS3Backend()
		if err != nil {
			return err
		}
		if err := backend.Initialize(ctx, rc.config.BackendConfig); err != nil {
			return err
		}
		rc.backends["s3"] = backend
		rc.activeBackend = "s3"

	case BackendRedis:
		backend, err := rc.createRedisBackend()
		if err != nil {
			return err
		}
		if err := backend.Initialize(ctx, rc.config.BackendConfig); err != nil {
			return err
		}
		rc.backends["redis"] = backend
		rc.activeBackend = "redis"

	case BackendCustom:
		backend, err := rc.createHTTPRestBackend()
		if err != nil {
			return err
		}
		if err := backend.Initialize(ctx, rc.config.BackendConfig); err != nil {
			return err
		}
		rc.backends["http"] = backend
		rc.activeBackend = "http"

	default:
		return fmt.Errorf("unsupported backend type: %v", rc.backendType)
	}

	return nil
}

func (rc *RemoteCache) getActiveBackend() RemoteBackend {
	rc.backendMu.RLock()
	defer rc.backendMu.RUnlock()

	if rc.activeBackend == "" {
		return nil
	}

	return rc.backends[rc.activeBackend]
}

// Core operation implementations

func (rc *RemoteCache) performGet(ctx context.Context, key string) (*CacheEntry, error) {
	backend := rc.getActiveBackend()
	if backend == nil {
		return nil, errors.New("no active backend available")
	}

	data, metadata, err := backend.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	// Decompress if needed
	if rc.compression != nil && metadata.Compressed {
		data, err = rc.compression.Decompress(data)
		if err != nil {
			return nil, fmt.Errorf("decompression failed: %w", err)
		}
	}

	// Deserialize cache entry
	var entry CacheEntry
	if err := rc.serializer.Deserialize(data, &entry); err != nil {
		return nil, fmt.Errorf("deserialization failed: %w", err)
	}

	return &entry, nil
}

func (rc *RemoteCache) performPut(ctx context.Context, key string, entry *CacheEntry) error {
	backend := rc.getActiveBackend()
	if backend == nil {
		return errors.New("no active backend available")
	}

	// Serialize cache entry
	data, err := rc.serializer.Serialize(entry)
	if err != nil {
		return fmt.Errorf("serialization failed: %w", err)
	}

	metadata := &StorageMetadata{
		Key:         key,
		Size:        int64(len(data)),
		ContentType: "application/msgpack",
		Version:     entry.Version,
		CreatedAt:   time.Now(),
		ModifiedAt:  time.Now(),
	}

	// Compress if beneficial
	if rc.compression != nil {
		compressedData, err := rc.compression.Compress(data)
		if err == nil && len(compressedData) < len(data) {
			data = compressedData
			metadata.Compressed = true
			metadata.Size = int64(len(data))
		}
	}

	// Calculate checksum
	hash := sha256.Sum256(data)
	metadata.Checksum = hex.EncodeToString(hash[:])

	return backend.Put(ctx, key, data, metadata)
}

func (rc *RemoteCache) performDelete(ctx context.Context, key string) error {
	backend := rc.getActiveBackend()
	if backend == nil {
		return errors.New("no active backend available")
	}

	return backend.Delete(ctx, key)
}

func (rc *RemoteCache) performGetBatch(ctx context.Context, keys []string) (map[string]*CacheEntry, error) {
	backend := rc.getActiveBackend()
	if backend == nil {
		return nil, errors.New("no active backend available")
	}

	rawData, err := backend.GetBatch(ctx, keys)
	if err != nil {
		return nil, err
	}

	result := make(map[string]*CacheEntry)

	for key, data := range rawData {
		// Deserialize cache entry
		var entry CacheEntry
		if err := rc.serializer.Deserialize(data, &entry); err != nil {
			// Log error but continue with other entries
			continue
		}
		result[key] = &entry
	}

	return result, nil
}

func (rc *RemoteCache) performPutBatch(ctx context.Context, entries map[string]*CacheEntry) error {
	backend := rc.getActiveBackend()
	if backend == nil {
		return errors.New("no active backend available")
	}

	serializedData := make(map[string][]byte)

	for key, entry := range entries {
		data, err := rc.serializer.Serialize(entry)
		if err != nil {
			return fmt.Errorf("serialization failed for key %s: %w", key, err)
		}

		// Compress if beneficial
		if rc.compression != nil {
			compressedData, err := rc.compression.Compress(data)
			if err == nil && len(compressedData) < len(data) {
				data = compressedData
			}
		}

		serializedData[key] = data
	}

	return backend.PutBatch(ctx, serializedData)
}

func (rc *RemoteCache) performDeleteBatch(ctx context.Context, keys []string) error {
	backend := rc.getActiveBackend()
	if backend == nil {
		return errors.New("no active backend available")
	}

	return backend.DeleteBatch(ctx, keys)
}

// Async operations for improved performance

func (rc *RemoteCache) putAsync(ctx context.Context, key string, entry *CacheEntry) error {
	// Serialize immediately to avoid race conditions
	data, err := rc.serializer.Serialize(entry)
	if err != nil {
		return fmt.Errorf("serialization failed: %w", err)
	}

	metadata := &StorageMetadata{
		Key:         key,
		Size:        int64(len(data)),
		ContentType: "application/msgpack",
		Version:     entry.Version,
		CreatedAt:   time.Now(),
	}

	op := &SyncOperation{
		Type:     SyncTypeUpload,
		Key:      key,
		Data:     data,
		Metadata: metadata,
		Priority: entry.Priority,
	}

	select {
	case rc.syncManager.uploadQueue <- op:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Queue is full, fallback to synchronous operation
		return rc.performPut(ctx, key, entry)
	}
}

// Background workers and maintenance

func (rc *RemoteCache) startBackgroundWorkers() {
	// Start upload workers
	for i := 0; i < rc.syncManager.workers; i++ {
		rc.syncManager.wg.Add(1)
		go rc.uploadWorker()
	}

	// Start download workers
	for i := 0; i < rc.syncManager.workers/2; i++ {
		rc.syncManager.wg.Add(1)
		go rc.downloadWorker()
	}
}

func (rc *RemoteCache) uploadWorker() {
	defer rc.syncManager.wg.Done()

	for {
		select {
		case op := <-rc.syncManager.uploadQueue:
			err := rc.processUploadOperation(op)
			if op.Callback != nil {
				op.Callback(err)
			}
			atomic.AddInt64(&rc.stats.syncOperations, 1)
			if err != nil {
				atomic.AddInt64(&rc.stats.syncErrors, 1)
			}

		case <-rc.syncManager.ctx.Done():
			return
		}
	}
}

func (rc *RemoteCache) downloadWorker() {
	defer rc.syncManager.wg.Done()

	for {
		select {
		case op := <-rc.syncManager.downloadQueue:
			err := rc.processDownloadOperation(op)
			if op.Callback != nil {
				op.Callback(err)
			}
			atomic.AddInt64(&rc.stats.syncOperations, 1)
			if err != nil {
				atomic.AddInt64(&rc.stats.syncErrors, 1)
			}

		case <-rc.syncManager.ctx.Done():
			return
		}
	}
}

func (rc *RemoteCache) processUploadOperation(op *SyncOperation) error {
	ctx, cancel := context.WithTimeout(rc.syncManager.ctx, 30*time.Second)
	defer cancel()

	backend := rc.getActiveBackend()
	if backend == nil {
		return errors.New("no active backend available")
	}

	return backend.Put(ctx, op.Key, op.Data, op.Metadata)
}

func (rc *RemoteCache) processDownloadOperation(op *SyncOperation) error {
	ctx, cancel := context.WithTimeout(rc.syncManager.ctx, 30*time.Second)
	defer cancel()

	backend := rc.getActiveBackend()
	if backend == nil {
		return errors.New("no active backend available")
	}

	_, _, err := backend.Get(ctx, op.Key)
	return err
}

func (rc *RemoteCache) maintenanceRoutine(ctx context.Context) {
	defer close(rc.maintenanceDone)

	ticker := time.NewTicker(rc.config.MaintenanceInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rc.performMaintenance(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (rc *RemoteCache) performMaintenance(ctx context.Context) {
	// Update health status
	rc.updateHealthStatus(ctx)

	// Update capacity metrics
	rc.updateCapacityMetrics(ctx)

	// Cleanup expired connections
	rc.cleanupConnections()

	// Update performance metrics
	rc.updatePerformanceMetrics()
}

// Network resilience and retry logic

func (rc *RemoteCache) executeWithRetry(ctx context.Context, operation string, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt <= rc.retryConfig.MaxRetries; attempt++ {
		if attempt > 0 {
			// Calculate backoff delay with jitter
			delay := rc.calculateBackoffDelay(attempt)

			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		err := fn()
		if err == nil {
			if attempt > 0 {
				atomic.AddInt64(&rc.stats.retryCount, int64(attempt))
			}
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !rc.isRetryableError(err) {
			break
		}

		atomic.AddInt64(&rc.stats.networkErrors, 1)
	}

	return fmt.Errorf("operation %s failed after %d attempts: %w",
		operation, rc.retryConfig.MaxRetries+1, lastErr)
}

func (rc *RemoteCache) calculateBackoffDelay(attempt int) time.Duration {
	// Exponential backoff with jitter
	delay := float64(rc.retryConfig.InitialDelay) *
		func(base float64, exp int) float64 {
			result := 1.0
			for i := 0; i < exp; i++ {
				result *= base
			}
			return result
		}(rc.retryConfig.BackoffFactor, attempt-1)

	// Add jitter
	jitter := delay * rc.retryConfig.JitterFactor * (rand.Float64()*2 - 1)
	delay += jitter

	// Cap at max delay
	if delay > float64(rc.retryConfig.MaxDelay) {
		delay = float64(rc.retryConfig.MaxDelay)
	}

	return time.Duration(delay)
}

func (rc *RemoteCache) isRetryableError(err error) bool {
	errStr := strings.ToLower(err.Error())

	for _, retryable := range rc.retryConfig.RetryableErrors {
		if strings.Contains(errStr, strings.ToLower(retryable)) {
			return true
		}
	}

	return false
}

// Performance monitoring

func (rc *RemoteCache) recordLatency(operation string, latency time.Duration) {
	tracker, exists := rc.latencyHistogram[operation]
	if !exists {
		tracker = &LatencyTracker{
			buckets: make([]int64, 20),
			bucketRanges: []time.Duration{
				1 * time.Millisecond,
				2 * time.Millisecond,
				5 * time.Millisecond,
				10 * time.Millisecond,
				20 * time.Millisecond,
				50 * time.Millisecond,
				100 * time.Millisecond,
				200 * time.Millisecond,
				500 * time.Millisecond,
				1 * time.Second,
				2 * time.Second,
				5 * time.Second,
				10 * time.Second,
			},
		}
		rc.latencyHistogram[operation] = tracker
	}

	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	// Find appropriate bucket
	for i, threshold := range tracker.bucketRanges {
		if latency <= threshold {
			tracker.buckets[i]++
			break
		}
	}

	tracker.totalSamples++

	// Update global latency stats
	rc.updateLatencyStats(latency)
}

func (rc *RemoteCache) updateLatencyStats(latency time.Duration) {
	rc.stats.mu.Lock()
	defer rc.stats.mu.Unlock()

	if rc.stats.maxLatency < latency {
		rc.stats.maxLatency = latency
	}

	// Simple running average (could be improved with more sophisticated algorithm)
	if rc.stats.avgLatency == 0 {
		rc.stats.avgLatency = latency
	} else {
		rc.stats.avgLatency = (rc.stats.avgLatency + latency) / 2
	}
}

func (rc *RemoteCache) calculateErrorRate() float64 {
	if rc.stats.totalRequests == 0 {
		return 0.0
	}
	return float64(rc.stats.networkErrors) / float64(rc.stats.totalRequests)
}

func (rc *RemoteCache) updateHealthStatus(ctx context.Context) {
	rc.health.mu.Lock()
	defer rc.health.mu.Unlock()

	rc.health.lastCheck = time.Now()

	// Check backend health
	healthy := true
	issues := []HealthIssue{}

	for name, backend := range rc.backends {
		if err := backend.Ping(ctx); err != nil {
			healthy = false
			rc.health.backendHealth[name] = false
			issues = append(issues, HealthIssue{
				Type:      "backend_unhealthy",
				Severity:  IssueSeverityError,
				Message:   fmt.Sprintf("Backend %s is unhealthy: %v", name, err),
				Timestamp: time.Now(),
				Component: name,
			})
		} else {
			rc.health.backendHealth[name] = true
		}
	}

	// Check circuit breaker state
	if rc.circuitBreaker.IsOpen() {
		healthy = false
		issues = append(issues, HealthIssue{
			Type:      "circuit_breaker_open",
			Severity:  IssueSeverityWarning,
			Message:   "Circuit breaker is open",
			Timestamp: time.Now(),
			Component: "circuit_breaker",
		})
	}

	rc.health.healthy = healthy
	rc.health.issues = issues

	if healthy {
		rc.health.status = TierStatusHealthy
	} else {
		rc.health.status = TierStatusDegraded
	}
}

func (rc *RemoteCache) updateCapacityMetrics(ctx context.Context) {
	// Implementation would query backend for capacity information
	// This is a simplified version
	rc.capacity.mu.Lock()
	defer rc.capacity.mu.Unlock()

	if rc.capacity.maxCapacity > 0 {
		rc.capacity.utilizationPct = float64(rc.capacity.usedCapacity) / float64(rc.capacity.maxCapacity) * 100
	}

	rc.capacity.availableCapacity = rc.capacity.maxCapacity - rc.capacity.usedCapacity
}

func (rc *RemoteCache) cleanupConnections() {
	rc.connectionPool.mu.Lock()
	defer rc.connectionPool.mu.Unlock()

	now := time.Now()
	for key, conn := range rc.connectionPool.connectionMap {
		if !conn.inUse && now.Sub(conn.lastUsed) > rc.connectionPool.idleTimeout {
			if conn.conn != nil {
				conn.conn.Close()
			}
			delete(rc.connectionPool.connectionMap, key)
		}
	}
}

func (rc *RemoteCache) updatePerformanceMetrics() {
	rc.throughputTracker.mu.Lock()
	defer rc.throughputTracker.mu.Unlock()

	// Update throughput calculations
	now := time.Now()
	// Add current window
	rc.throughputTracker.timeWindows = append(rc.throughputTracker.timeWindows, now)
	rc.throughputTracker.requestCounts = append(rc.throughputTracker.requestCounts,
		atomic.LoadInt64(&rc.stats.totalRequests))
	rc.throughputTracker.byteCounts = append(rc.throughputTracker.byteCounts,
		atomic.LoadInt64(&rc.stats.bytesTransferred))

	// Remove old windows
	cutoff := now.Add(-rc.throughputTracker.windowSize)
	validIndex := 0
	for i, t := range rc.throughputTracker.timeWindows {
		if t.After(cutoff) {
			rc.throughputTracker.timeWindows[validIndex] = rc.throughputTracker.timeWindows[i]
			rc.throughputTracker.requestCounts[validIndex] = rc.throughputTracker.requestCounts[i]
			rc.throughputTracker.byteCounts[validIndex] = rc.throughputTracker.byteCounts[i]
			validIndex++
		}
	}

	rc.throughputTracker.timeWindows = rc.throughputTracker.timeWindows[:validIndex]
	rc.throughputTracker.requestCounts = rc.throughputTracker.requestCounts[:validIndex]
	rc.throughputTracker.byteCounts = rc.throughputTracker.byteCounts[:validIndex]
}

// Helper functions

func (rc *RemoteCache) performPatternInvalidation(ctx context.Context, pattern string) (int, error) {
	// Implementation depends on backend capabilities
	// This is a simplified version that would need backend-specific logic
	return 0, errors.New("pattern invalidation not implemented for current backend")
}

func (rc *RemoteCache) performClear(ctx context.Context) error {
	// Implementation depends on backend capabilities
	return errors.New("clear operation not implemented for current backend")
}

func (rc *RemoteCache) forceSyncPendingOperations(ctx context.Context) error {
	// Wait for queues to drain with timeout
	timeout := time.NewTimer(30 * time.Second)
	defer timeout.Stop()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout.C:
			return errors.New("timeout waiting for sync operations to complete")
		case <-ticker.C:
			if len(rc.syncManager.uploadQueue) == 0 && len(rc.syncManager.downloadQueue) == 0 {
				return nil
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Backend factory methods (simplified - full implementation would be more comprehensive)

func (rc *RemoteCache) createS3Backend() (RemoteBackend, error) {
	return &S3Backend{
		endpoint:  rc.config.BackendConfig.ConnectionString,
		client:    rc.httpClient,
		useSSL:    true,
		pathStyle: false,
	}, nil
}

func (rc *RemoteCache) createRedisBackend() (RemoteBackend, error) {
	return &RedisBackend{
		client: redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:    []string{rc.config.BackendConfig.ConnectionString},
			Password: rc.config.BackendConfig.Password,
		}),
		timeout: time.Duration(rc.config.TimeoutMs) * time.Millisecond,
	}, nil
}

func (rc *RemoteCache) createHTTPRestBackend() (RemoteBackend, error) {
	return &HTTPRestBackend{
		endpoint:    rc.config.BackendConfig.ConnectionString,
		client:      rc.httpClient,
		compression: true,
		timeout:     time.Duration(rc.config.TimeoutMs) * time.Millisecond,
		headers: map[string]string{
			"Content-Type": "application/msgpack",
			"User-Agent":   "LSP-Gateway-RemoteCache/1.0",
		},
	}, nil
}

// Serialization provider implementations

type MsgPackSerializer struct{}

func NewMsgPackSerializer() SerializationProvider {
	return &MsgPackSerializer{}
}

func (s *MsgPackSerializer) Serialize(data interface{}) ([]byte, error) {
	return msgpack.Marshal(data)
}

func (s *MsgPackSerializer) Deserialize(data []byte, target interface{}) error {
	return msgpack.Unmarshal(data, target)
}

func (s *MsgPackSerializer) GetFormat() SerializationFormat {
	return SerializationMsgPack
}

// Compression provider implementations

type ZstdCompressionProvider struct {
	encoder *zstd.Encoder
	decoder *zstd.Decoder
	level   zstd.EncoderLevel
}

func NewCompressionProvider(config *CompressionConfig) CompressionProvider {
	if config.Type == CompressionZstd {
		encoder, _ := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.EncoderLevel(config.Level)))
		decoder, _ := zstd.NewReader(nil)

		return &ZstdCompressionProvider{
			encoder: encoder,
			decoder: decoder,
			level:   zstd.EncoderLevel(config.Level),
		}
	}

	// Default to no compression
	return &NoCompressionProvider{}
}

func (z *ZstdCompressionProvider) Compress(data []byte) ([]byte, error) {
	return z.encoder.EncodeAll(data, nil), nil
}

func (z *ZstdCompressionProvider) Decompress(data []byte) ([]byte, error) {
	return z.decoder.DecodeAll(data, nil)
}

func (z *ZstdCompressionProvider) EstimateCompressionRatio(data []byte) float64 {
	// Estimate based on entropy (simplified)
	return 0.3 // Assume 30% compression ratio
}

func (z *ZstdCompressionProvider) GetCompressionType() CompressionType {
	return CompressionZstd
}

func (z *ZstdCompressionProvider) GetCompressionLevel() int {
	return int(z.level)
}

func (z *ZstdCompressionProvider) SetCompressionLevel(level int) error {
	z.level = zstd.EncoderLevel(level)
	encoder, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(z.level))
	if err != nil {
		return err
	}
	z.encoder = encoder
	return nil
}

func (z *ZstdCompressionProvider) SetCompressionType(compressionType CompressionType) error {
	if compressionType != CompressionZstd {
		return errors.New("unsupported compression type")
	}
	return nil
}

type NoCompressionProvider struct{}

func (n *NoCompressionProvider) Compress(data []byte) ([]byte, error) {
	return data, nil
}

func (n *NoCompressionProvider) Decompress(data []byte) ([]byte, error) {
	return data, nil
}

func (n *NoCompressionProvider) EstimateCompressionRatio(data []byte) float64 {
	return 1.0
}

func (n *NoCompressionProvider) GetCompressionType() CompressionType {
	return CompressionNone
}

func (n *NoCompressionProvider) GetCompressionLevel() int {
	return 0
}

func (n *NoCompressionProvider) SetCompressionLevel(level int) error {
	return nil
}

func (n *NoCompressionProvider) SetCompressionType(compressionType CompressionType) error {
	return nil
}
