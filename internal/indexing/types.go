package indexing

import (
	"encoding/json"
	"time"

	"github.com/sourcegraph/scip/bindings/go/scip"
)

// SCIPQueryResult represents the result of a SCIP index query
type SCIPQueryResult struct {
	Found      bool                   `json:"found"`
	Method     string                 `json:"method"`
	Response   json.RawMessage        `json:"response,omitempty"`
	Error      string                 `json:"error,omitempty"`
	CacheHit   bool                   `json:"cache_hit"`
	QueryTime  time.Duration          `json:"query_time"`
	IndexPath  string                 `json:"index_path,omitempty"`
	Confidence float64                `json:"confidence"`
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
	IndexesLoaded    int           `json:"indexes_loaded"`
	TotalQueries     int64         `json:"total_queries"`
	CacheHitRate     float64       `json:"cache_hit_rate"`
	AverageQueryTime time.Duration `json:"average_query_time"`
	LastQueryTime    time.Time     `json:"last_query_time"`
	CacheSize        int           `json:"cache_size"`
	MemoryUsage      int64         `json:"memory_usage_bytes"`
}

// SCIPConfig represents SCIP configuration
type SCIPConfig struct {
	CacheConfig CacheConfig       `json:"cache_config"`
	Logging     LoggingConfig     `json:"logging"`
	Performance PerformanceConfig `json:"performance"`
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
	QueryTimeout         time.Duration `json:"query_timeout"`
	MaxConcurrentQueries int           `json:"max_concurrent_queries"`
	IndexLoadTimeout     time.Duration `json:"index_load_timeout"`
}

// SymbolRelationship represents a relationship between symbols
type SymbolRelationship struct {
	Symbol           string  `json:"symbol"`
	IsReference      bool    `json:"is_reference"`
	IsDefinition     bool    `json:"is_definition"`
	IsTypeDefinition bool    `json:"is_type_definition"`
	IsImplementation bool    `json:"is_implementation"`
	RelationshipKind string  `json:"relationship_kind"`
	Confidence       float64 `json:"confidence"`
}

// ResolvedReference represents a resolved symbol reference with context
type ResolvedReference struct {
	Symbol     string      `json:"symbol"`
	URI        string      `json:"uri"`
	Range      *scip.Range `json:"range"`
	Role       int32       `json:"role"`
	Confidence float64     `json:"confidence"`
	Context    string      `json:"context,omitempty"`
	ResolvedAt time.Time   `json:"resolved_at"`
}

// SCIP type aliases and definitions for missing types in current version

// SCIPSignature represents symbol signature information
type SCIPSignature struct {
	Text          string `json:"text"`
	Language      string `json:"language"`
	Kind          string `json:"kind"`
	Documentation string `json:"documentation,omitempty"`
}

// SCIPDocumentation represents symbol documentation
type SCIPDocumentation struct {
	Format string   `json:"format"`
	Value  string   `json:"value"`
	Tags   []string `json:"tags,omitempty"`
}

// Helper functions for SCIP range conversions

// ConvertSCIPRangeToRange converts a SCIP []int32 range to *scip.Range
func ConvertSCIPRangeToRange(scipRange []int32) *scip.Range {
	if len(scipRange) < 4 {
		return nil
	}
	return &scip.Range{
		Start: scip.Position{Line: scipRange[0], Character: scipRange[1]},
		End:   scip.Position{Line: scipRange[2], Character: scipRange[3]},
	}
}

// ConvertRangeToSCIPRange converts a *scip.Range to SCIP []int32 range
func ConvertRangeToSCIPRange(range_ *scip.Range) []int32 {
	if range_ == nil {
		return nil
	}
	return []int32{
		range_.Start.Line,
		range_.Start.Character,
		range_.End.Line,
		range_.End.Character,
	}
}
