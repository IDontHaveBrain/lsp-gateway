package indexing

import (
	"encoding/json"
	"time"
)

// SCIPQueryResult represents the result of a SCIP index query
type SCIPQueryResult struct {
	Found      bool            `json:"found"`
	Method     string          `json:"method"`
	Response   json.RawMessage `json:"response,omitempty"`
	Error      string          `json:"error,omitempty"`
	CacheHit   bool            `json:"cache_hit"`
	QueryTime  time.Duration   `json:"query_time"`
	IndexPath  string          `json:"index_path,omitempty"`
	Confidence float64         `json:"confidence"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// SCIPStore defines the interface for SCIP index operations
type SCIPStore interface {
	// LoadIndex loads a SCIP index from the specified path
	LoadIndex(path string) error

	// Query performs a query against the loaded SCIP index
	Query(method string, params interface{}) SCIPQueryResult

	// CacheResponse caches a response for future queries
	CacheResponse(method string, params interface{}, response json.RawMessage) error

	// InvalidateFile invalidates cached entries for a specific file
	InvalidateFile(filePath string)

	// GetStats returns statistics about the store
	GetStats() SCIPStoreStats

	// Close cleans up resources and closes the store
	Close() error
}

// SCIPStoreStats represents statistics about the SCIP store
type SCIPStoreStats struct {
	IndexesLoaded   int           `json:"indexes_loaded"`
	TotalQueries    int64         `json:"total_queries"`
	CacheHitRate    float64       `json:"cache_hit_rate"`
	AverageQueryTime time.Duration `json:"average_query_time"`
	LastQueryTime   time.Time     `json:"last_query_time"`
	CacheSize       int           `json:"cache_size"`
	MemoryUsage     int64         `json:"memory_usage_bytes"`
}

// SCIPConfig represents SCIP configuration
type SCIPConfig struct {
	CacheConfig    CacheConfig       `json:"cache_config"`
	Logging        LoggingConfig     `json:"logging"`
	Performance    PerformanceConfig `json:"performance"`
}

// CacheConfig represents cache configuration
type CacheConfig struct {
	Enabled bool          `json:"enabled"`
	MaxSize int           `json:"max_size"`
	TTL     time.Duration `json:"ttl"`
}

// LoggingConfig represents logging configuration
type LoggingConfig struct {
	LogQueries         bool `json:"log_queries"`
	LogCacheOperations bool `json:"log_cache_operations"`
	LogIndexOperations bool `json:"log_index_operations"`
}

// PerformanceConfig represents performance configuration
type PerformanceConfig struct {
	QueryTimeout           time.Duration `json:"query_timeout"`
	MaxConcurrentQueries   int           `json:"max_concurrent_queries"`
	IndexLoadTimeout       time.Duration `json:"index_load_timeout"`
}