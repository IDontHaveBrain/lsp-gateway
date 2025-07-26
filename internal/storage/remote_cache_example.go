package storage

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

// ExampleRemoteCacheUsage demonstrates how to configure and use the L3 Remote Storage
func ExampleRemoteCacheUsage() {
	// Example 1: HTTP REST Backend Configuration
	httpConfig := TierConfig{
		TierType:    TierL3Remote,
		BackendType: BackendCustom,
		BackendConfig: BackendConfig{
			Type:             BackendCustom,
			ConnectionString: "https://cache-api.example.com/v1",
			APIKey:           "your-api-key-here",
			TimeoutMs:        5000,
			MaxConnections:   20,
			RetryCount:       3,
			RetryDelayMs:     500,
			CircuitBreaker: &CircuitBreakerConfig{
				Enabled:          true,
				FailureThreshold: 10,
				RecoveryTimeout:  30 * time.Second,
				HalfOpenRequests: 5,
			},
			Options: map[string]interface{}{
				"compression": true,
				"auth_type":   "bearer",
			},
		},
		MaxCapacity:         10 * 1024 * 1024 * 1024, // 10GB
		MaxEntries:          100000,
		TimeoutMs:           5000,
		RetryCount:          3,
		RetryDelayMs:        500,
		MaintenanceInterval: 10 * time.Minute,
		CompressionConfig: &CompressionConfig{
			Enabled:   true,
			Type:      CompressionZstd,
			Level:     3,
			MinSize:   1024,
			Threshold: 0.8,
		},
	}

	// Example 2: S3-Compatible Backend Configuration
	s3Config := TierConfig{
		TierType:    TierL3Remote,
		BackendType: BackendS3,
		BackendConfig: BackendConfig{
			Type:             BackendS3,
			ConnectionString: "s3://my-cache-bucket@us-west-2/cache",
			Username:         "AKIAIOSFODNN7EXAMPLE",
			Password:         "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			TimeoutMs:        10000,
			MaxConnections:   10,
			RetryCount:       5,
			RetryDelayMs:     1000,
			CircuitBreaker: &CircuitBreakerConfig{
				Enabled:          true,
				FailureThreshold: 5,
				RecoveryTimeout:  60 * time.Second,
				HalfOpenRequests: 3,
			},
			Options: map[string]interface{}{
				"endpoint":   "https://s3.us-west-2.amazonaws.com",
				"ssl":        true,
				"path_style": false,
			},
		},
		MaxCapacity:         100 * 1024 * 1024 * 1024, // 100GB
		MaxEntries:          1000000,
		TimeoutMs:           10000,
		RetryCount:          5,
		RetryDelayMs:        1000,
		MaintenanceInterval: 15 * time.Minute,
		CompressionConfig: &CompressionConfig{
			Enabled:   true,
			Type:      CompressionZstd,
			Level:     5,
			MinSize:   2048,
			Threshold: 0.7,
		},
	}

	// Example 3: Redis Cluster Backend Configuration
	redisConfig := TierConfig{
		TierType:    TierL3Remote,
		BackendType: BackendRedis,
		BackendConfig: BackendConfig{
			Type:             BackendRedis,
			ConnectionString: "redis-cluster-1.example.com:6379,redis-cluster-2.example.com:6379,redis-cluster-3.example.com:6379",
			Password:         "your-redis-password",
			TimeoutMs:        3000,
			MaxConnections:   50,
			RetryCount:       3,
			RetryDelayMs:     200,
			CircuitBreaker: &CircuitBreakerConfig{
				Enabled:          true,
				FailureThreshold: 15,
				RecoveryTimeout:  20 * time.Second,
				HalfOpenRequests: 10,
			},
			Options: map[string]interface{}{
				"key_prefix":  "lsp_cache",
				"db":          0,
				"compression": true,
			},
		},
		MaxCapacity:         50 * 1024 * 1024 * 1024, // 50GB
		MaxEntries:          500000,
		TimeoutMs:           3000,
		RetryCount:          3,
		RetryDelayMs:        200,
		MaintenanceInterval: 5 * time.Minute,
		CompressionConfig: &CompressionConfig{
			Enabled:   true,
			Type:      CompressionLZ4,
			Level:     1,
			MinSize:   512,
			Threshold: 0.9,
		},
	}

	// Create and use remote cache with HTTP backend
	remoteCache, err := NewRemoteCache(httpConfig)
	if err != nil {
		log.Fatalf("Failed to create remote cache: %v", err)
	}
	defer remoteCache.Close()

	// Initialize the cache
	ctx := context.Background()
	if err := remoteCache.Initialize(ctx, httpConfig); err != nil {
		log.Fatalf("Failed to initialize remote cache: %v", err)
	}

	// Example usage
	demonstrateRemoteCacheOperations(ctx, remoteCache)

	// Print other configurations for reference
	log.Printf("S3 Configuration: %+v", s3Config)
	log.Printf("Redis Configuration: %+v", redisConfig)
}

func demonstrateRemoteCacheOperations(ctx context.Context, cache *RemoteCache) {
	// Create a sample cache entry
	entry := &CacheEntry{
		Method:      "textDocument/documentSymbol",
		Params:      `{"textDocument":{"uri":"file:///example/project/src/main.go"}}`,
		Response:    []byte(`{"result":[{"name":"main","kind":12,"range":{"start":{"line":0,"character":0},"end":{"line":10,"character":1}}}]}`),
		CreatedAt:   time.Now(),
		AccessedAt:  time.Now(),
		TTL:         24 * time.Hour,
		Key:         "main.go:symbols",
		Size:        150,
		Version:     1,
		CurrentTier: TierL3Remote,
		OriginTier:  TierL1Memory,
		Priority:    75,
		AccessCount: 1,
		FilePaths:   []string{"/example/project/src/main.go"},
		ProjectPath: "/example/project",
	}

	// Put operation
	log.Println("Storing cache entry...")
	if err := cache.Put(ctx, entry.Key, entry); err != nil {
		log.Printf("Put operation failed: %v", err)
		return
	}
	log.Println("Cache entry stored successfully")

	// Get operation
	log.Println("Retrieving cache entry...")
	retrievedEntry, err := cache.Get(ctx, entry.Key)
	if err != nil {
		log.Printf("Get operation failed: %v", err)
		return
	}
	log.Printf("Retrieved entry: method=%s, size=%d bytes", retrievedEntry.Method, retrievedEntry.Size)

	// Batch operations
	log.Println("Demonstrating batch operations...")
	entries := map[string]*CacheEntry{
		"file1.go:symbols": createExampleEntry("file1.go", "symbols"),
		"file2.go:symbols": createExampleEntry("file2.go", "symbols"),
		"file3.go:hover":   createExampleEntry("file3.go", "hover"),
	}

	// Batch put
	if err := cache.PutBatch(ctx, entries); err != nil {
		log.Printf("Batch put failed: %v", err)
	} else {
		log.Printf("Batch put successful: %d entries", len(entries))
	}

	// Batch get
	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}

	batchEntries, err := cache.GetBatch(ctx, keys)
	if err != nil {
		log.Printf("Batch get failed: %v", err)
	} else {
		log.Printf("Batch get successful: %d entries retrieved", len(batchEntries))
	}

	// Check cache statistics
	stats := cache.GetStats()
	log.Printf("Cache Statistics:")
	log.Printf("  Total Requests: %d", stats.TotalRequests)
	log.Printf("  Cache Hits: %d", stats.CacheHits)
	log.Printf("  Cache Misses: %d", stats.CacheMisses)
	log.Printf("  Hit Rate: %.2f%%", stats.HitRate*100)
	log.Printf("  Average Latency: %v", stats.AvgLatency)
	log.Printf("  P95 Latency: %v", stats.P95Latency)

	// Check cache health
	health := cache.GetHealth()
	log.Printf("Cache Health:")
	log.Printf("  Healthy: %t", health.Healthy)
	log.Printf("  Status: %s", health.Status)
	log.Printf("  Issues: %d", len(health.Issues))

	// Check capacity
	capacity := cache.GetCapacity()
	log.Printf("Cache Capacity:")
	log.Printf("  Max Capacity: %d bytes", capacity.MaxCapacity)
	log.Printf("  Used Capacity: %d bytes", capacity.UsedCapacity)
	log.Printf("  Utilization: %.2f%%", capacity.UtilizationPct)

	// Invalidation examples
	log.Println("Demonstrating invalidation...")

	// Invalidate by file
	invalidated, err := cache.InvalidateByFile(ctx, "/example/project/src/main.go")
	if err != nil {
		log.Printf("File invalidation failed: %v", err)
	} else {
		log.Printf("File invalidation successful: %d entries invalidated", invalidated)
	}

	// Invalidate by project
	invalidated, err = cache.InvalidateByProject(ctx, "/example/project")
	if err != nil {
		log.Printf("Project invalidation failed: %v", err)
	} else {
		log.Printf("Project invalidation successful: %d entries invalidated", invalidated)
	}

	// Flush to ensure all async operations complete
	log.Println("Flushing cache...")
	if err := cache.Flush(ctx); err != nil {
		log.Printf("Cache flush failed: %v", err)
	} else {
		log.Println("Cache flush successful")
	}
}

func createExampleEntry(filename, operation string) *CacheEntry {
	key := filename + ":" + operation

	var response []byte
	switch operation {
	case "symbols":
		response = []byte(`{"result":[{"name":"example","kind":5,"range":{"start":{"line":0,"character":0},"end":{"line":5,"character":1}}}]}`)
	case "hover":
		response = []byte(`{"result":{"contents":"Example hover information"}}`)
	default:
		response = []byte(`{"result":null}`)
	}

	return &CacheEntry{
		Method:      "textDocument/" + operation,
		Params:      fmt.Sprintf(`{"textDocument":{"uri":"file:///%s"}}`, filename),
		Response:    response,
		CreatedAt:   time.Now(),
		AccessedAt:  time.Now(),
		TTL:         6 * time.Hour,
		Key:         key,
		Size:        int64(len(response)),
		Version:     1,
		CurrentTier: TierL3Remote,
		OriginTier:  TierL1Memory,
		Priority:    50,
		AccessCount: 1,
		FilePaths:   []string{filename},
		ProjectPath: "/example/project",
	}
}

// Configuration helper functions

// NewHTTPRemoteCacheConfig creates a configuration for HTTP REST backend
func NewHTTPRemoteCacheConfig(endpoint, apiKey string) TierConfig {
	return TierConfig{
		TierType:    TierL3Remote,
		BackendType: BackendCustom,
		BackendConfig: BackendConfig{
			Type:             BackendCustom,
			ConnectionString: endpoint,
			APIKey:           apiKey,
			TimeoutMs:        5000,
			MaxConnections:   20,
			RetryCount:       3,
			RetryDelayMs:     500,
			CircuitBreaker: &CircuitBreakerConfig{
				Enabled:          true,
				FailureThreshold: 10,
				RecoveryTimeout:  30 * time.Second,
				HalfOpenRequests: 5,
			},
		},
		MaxCapacity:         10 * 1024 * 1024 * 1024, // 10GB
		MaxEntries:          100000,
		TimeoutMs:           5000,
		RetryCount:          3,
		RetryDelayMs:        500,
		MaintenanceInterval: 10 * time.Minute,
		CompressionConfig: &CompressionConfig{
			Enabled:   true,
			Type:      CompressionZstd,
			Level:     3,
			MinSize:   1024,
			Threshold: 0.8,
		},
	}
}

// NewS3RemoteCacheConfig creates a configuration for S3-compatible backend
func NewS3RemoteCacheConfig(bucket, region, accessKey, secretKey string) TierConfig {
	connectionString := fmt.Sprintf("s3://%s@%s/cache", bucket, region)

	return TierConfig{
		TierType:    TierL3Remote,
		BackendType: BackendS3,
		BackendConfig: BackendConfig{
			Type:             BackendS3,
			ConnectionString: connectionString,
			Username:         accessKey,
			Password:         secretKey,
			TimeoutMs:        10000,
			MaxConnections:   10,
			RetryCount:       5,
			RetryDelayMs:     1000,
			CircuitBreaker: &CircuitBreakerConfig{
				Enabled:          true,
				FailureThreshold: 5,
				RecoveryTimeout:  60 * time.Second,
				HalfOpenRequests: 3,
			},
			Options: map[string]interface{}{
				"ssl":        true,
				"path_style": false,
			},
		},
		MaxCapacity:         100 * 1024 * 1024 * 1024, // 100GB
		MaxEntries:          1000000,
		TimeoutMs:           10000,
		RetryCount:          5,
		RetryDelayMs:        1000,
		MaintenanceInterval: 15 * time.Minute,
		CompressionConfig: &CompressionConfig{
			Enabled:   true,
			Type:      CompressionZstd,
			Level:     5,
			MinSize:   2048,
			Threshold: 0.7,
		},
	}
}

// NewRedisRemoteCacheConfig creates a configuration for Redis cluster backend
func NewRedisRemoteCacheConfig(addresses []string, password string) TierConfig {
	connectionString := strings.Join(addresses, ",")

	return TierConfig{
		TierType:    TierL3Remote,
		BackendType: BackendRedis,
		BackendConfig: BackendConfig{
			Type:             BackendRedis,
			ConnectionString: connectionString,
			Password:         password,
			TimeoutMs:        3000,
			MaxConnections:   50,
			RetryCount:       3,
			RetryDelayMs:     200,
			CircuitBreaker: &CircuitBreakerConfig{
				Enabled:          true,
				FailureThreshold: 15,
				RecoveryTimeout:  20 * time.Second,
				HalfOpenRequests: 10,
			},
			Options: map[string]interface{}{
				"key_prefix":  "lsp_cache",
				"db":          0,
				"compression": true,
			},
		},
		MaxCapacity:         50 * 1024 * 1024 * 1024, // 50GB
		MaxEntries:          500000,
		TimeoutMs:           3000,
		RetryCount:          3,
		RetryDelayMs:        200,
		MaintenanceInterval: 5 * time.Minute,
		CompressionConfig: &CompressionConfig{
			Enabled:   true,
			Type:      CompressionLZ4,
			Level:     1,
			MinSize:   512,
			Threshold: 0.9,
		},
	}
}

// Production deployment helper
func CreateProductionRemoteCache(config TierConfig) (*RemoteCache, error) {
	// Create remote cache
	cache, err := NewRemoteCache(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create remote cache: %w", err)
	}

	// Initialize with context timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := cache.Initialize(ctx, config); err != nil {
		cache.Close()
		return nil, fmt.Errorf("failed to initialize remote cache: %w", err)
	}

	// Verify health
	health := cache.GetHealth()
	if !health.Healthy {
		cache.Close()
		return nil, fmt.Errorf("remote cache is not healthy: %+v", health.Issues)
	}

	return cache, nil
}
