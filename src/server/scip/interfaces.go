package scip

import (
	"context"
	"time"
)

// SCIPDocument represents a SCIP document with symbols and references
type SCIPDocument struct {
	URI          string
	Language     string
	Content      []byte
	Symbols      []SCIPSymbol
	References   []SCIPReference
	LastModified time.Time
	Size         int64
}

// SCIPSymbol represents a symbol in SCIP format
type SCIPSymbol struct {
	ID            string
	Name          string
	Kind          int
	Range         SCIPRange
	Definition    SCIPRange
	Documentation string
}

// SCIPReference represents a reference in SCIP format
type SCIPReference struct {
	Symbol string
	Range  SCIPRange
}

// SCIPRange represents a position range
type SCIPRange struct {
	Start SCIPPosition
	End   SCIPPosition
}

// SCIPPosition represents a position in a document
type SCIPPosition struct {
	Line      int
	Character int
}

// SCIPStorageStats provides storage statistics and health information
type SCIPStorageStats struct {
	MemoryUsage       int64
	DiskUsage         int64
	CachedDocuments   int
	HotCacheSize      int
	WarmCacheSize     int
	ColdCacheSize     int
	MemoryLimit       int64
	HitRate           float64
	CompactionRunning bool
	LastCompaction    time.Time
}

// SCIPStorageConfig defines storage configuration
type SCIPStorageConfig struct {
	MemoryLimit        int64         // Memory limit in bytes (default: 256MB)
	DiskCacheDir       string        // Directory for disk storage
	CompressionType    string        // Compression type for disk storage
	CompactionInterval time.Duration // Compaction interval
	MaxDocumentAge     time.Duration // Max age for cached documents
	EnableMetrics      bool          // Enable metrics collection
}

// SCIPDocumentStorage interface defines the storage operations for SCIP indexes
type SCIPDocumentStorage interface {
	// Document operations
	StoreDocument(ctx context.Context, doc *SCIPDocument) error
	GetDocument(ctx context.Context, uri string) (*SCIPDocument, error)
	RemoveDocument(ctx context.Context, uri string) error
	ListDocuments(ctx context.Context) ([]string, error)

	// Symbol operations
	GetSymbolsByName(ctx context.Context, name string) ([]SCIPSymbol, error)
	GetSymbolsInRange(ctx context.Context, uri string, start, end SCIPPosition) ([]SCIPSymbol, error)
	GetSymbolDefinition(ctx context.Context, symbolID string) (*SCIPSymbol, error)

	// Reference operations
	GetReferences(ctx context.Context, symbolID string) ([]SCIPReference, error)
	GetReferencesInDocument(ctx context.Context, uri string) ([]SCIPReference, error)

	// Cache management
	Flush(ctx context.Context) error
	Compact(ctx context.Context) error
	GetStats(ctx context.Context) (*SCIPStorageStats, error)
	SetConfig(config SCIPStorageConfig) error

	// Lifecycle
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	HealthCheck(ctx context.Context) error
}

// SCIPCacheManager interface for cache-specific operations
type SCIPCacheManager interface {
	Evict(key string) error
	EvictLRU(count int) error
	Clear() error
	GetSize() int64
	GetStats() map[string]interface{}
}

// SCIPCompactionManager interface for disk compaction operations
type SCIPCompactionManager interface {
	Compact(ctx context.Context) error
	IsRunning() bool
	Schedule(interval time.Duration)
	Stop() error
}

// SCIPHealthChecker interface for storage health monitoring
type SCIPHealthChecker interface {
	Check(ctx context.Context) error
	GetLastCheck() time.Time
	GetHealthStatus() string
}
