package types

import (
	"time"
)

// BenchmarkConfig defines performance benchmarking configuration
type BenchmarkConfig struct {
	// Performance targets
	LatencyThresholds    LatencyThresholds    `yaml:"latency_thresholds"`
	ThroughputThresholds ThroughputThresholds `yaml:"throughput_thresholds"`
	MemoryThresholds     MemoryThresholds     `yaml:"memory_thresholds"`

	// Test configuration
	WarmupIterations  int           `yaml:"warmup_iterations"`
	BenchmarkDuration time.Duration `yaml:"benchmark_duration"`
	ConcurrencyLevels []int         `yaml:"concurrency_levels"`
	SamplingRate      time.Duration `yaml:"sampling_rate"`

	// Reporting
	EnableCSVOutput   bool   `yaml:"enable_csv_output"`
	EnableReporting   bool   `yaml:"enable_reporting"`
	OutputDir         string `yaml:"output_dir"`
	DetectRegressions bool   `yaml:"detect_regressions"`
}

// LatencyThresholds defines acceptable latency limits for each LSP method
type LatencyThresholds struct {
	Definition      MethodThresholds `yaml:"definition"`
	References      MethodThresholds `yaml:"references"`
	Hover           MethodThresholds `yaml:"hover"`
	DocumentSymbol  MethodThresholds `yaml:"document_symbol"`
	WorkspaceSymbol MethodThresholds `yaml:"workspace_symbol"`
}

// MethodThresholds defines latency thresholds for a specific method
type MethodThresholds struct {
	P50 time.Duration `yaml:"p50"`
	P95 time.Duration `yaml:"p95"`
	P99 time.Duration `yaml:"p99"`
	Max time.Duration `yaml:"max"`
}

// ThroughputThresholds defines minimum acceptable throughput
type ThroughputThresholds struct {
	MinRequestsPerSecond float64 `yaml:"min_requests_per_second"`
	MaxErrorRate         float64 `yaml:"max_error_rate"`
}

// MemoryThresholds defines acceptable memory usage limits
type MemoryThresholds struct {
	MaxAllocPerRequest int64         `yaml:"max_alloc_per_request"` // bytes
	MaxMemoryGrowth    float64       `yaml:"max_memory_growth"`     // percentage
	MaxGCPause         time.Duration `yaml:"max_gc_pause"`
}

// BenchmarkResult contains performance benchmark results
type BenchmarkResult struct {
	Method             string              `json:"method"`
	TotalRequests      int64               `json:"total_requests"`
	Duration           time.Duration       `json:"duration"`
	ThroughputRPS      float64             `json:"throughput_rps"`
	ErrorCount         int64               `json:"error_count"`
	ErrorRate          float64             `json:"error_rate"`
	LatencyMetrics     LatencyMetrics      `json:"latency_metrics"`
	MemoryMetrics      MemoryMetrics       `json:"memory_metrics"`
	ConcurrencyResults []ConcurrencyResult `json:"concurrency_results"`
	ThresholdResults   ThresholdResults    `json:"threshold_results"`
	Timestamp          time.Time           `json:"timestamp"`
}

// LatencyMetrics contains latency statistics
type LatencyMetrics struct {
	Average time.Duration `json:"average"`
	Min     time.Duration `json:"min"`
	Max     time.Duration `json:"max"`
	P50     time.Duration `json:"p50"`
	P95     time.Duration `json:"p95"`
	P99     time.Duration `json:"p99"`
}

// MemoryMetrics contains memory usage statistics
type MemoryMetrics struct {
	AllocPerRequest     int64         `json:"alloc_per_request"` // bytes
	TotalAlloc          int64         `json:"total_alloc"`       // bytes
	PeakMemoryUsage     int64         `json:"peak_memory_usage"` // bytes
	MemoryGrowthPercent float64       `json:"memory_growth_percent"`
	GCCount             uint32        `json:"gc_count"`
	TotalGCPause        time.Duration `json:"total_gc_pause"`
	AvgGCPause          time.Duration `json:"avg_gc_pause"`
}

// ConcurrencyResult contains results for a specific concurrency level
type ConcurrencyResult struct {
	ConcurrentUsers int            `json:"concurrent_users"`
	ThroughputRPS   float64        `json:"throughput_rps"`
	LatencyMetrics  LatencyMetrics `json:"latency_metrics"`
	ErrorRate       float64        `json:"error_rate"`
}

// ThresholdResults indicates which thresholds passed/failed
type ThresholdResults struct {
	LatencyPassed    bool     `json:"latency_passed"`
	ThroughputPassed bool     `json:"throughput_passed"`
	MemoryPassed     bool     `json:"memory_passed"`
	OverallPassed    bool     `json:"overall_passed"`
	FailureReasons   []string `json:"failure_reasons"`
}

// MemorySample represents a memory usage sample
type MemorySample struct {
	Timestamp  time.Time
	Alloc      uint64
	TotalAlloc uint64
	Sys        uint64
	GCCount    uint32
}
