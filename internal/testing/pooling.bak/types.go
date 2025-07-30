package pooling

import (
	"context"
	"time"
)

// ServerState represents the current state of a pooled server
type ServerState int

const (
	ServerStateUnknown ServerState = iota
	ServerStateStarting
	ServerStateReady
	ServerStateBusy
	ServerStateIdle
	ServerStateStopping
	ServerStateFailed
)

func (s ServerState) String() string {
	switch s {
	case ServerStateStarting:
		return "starting"
	case ServerStateReady:
		return "ready"
	case ServerStateBusy:
		return "busy"
	case ServerStateIdle:
		return "idle"
	case ServerStateStopping:
		return "stopping"
	case ServerStateFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// HealthStatus represents the health status of a server
type HealthStatus int

const (
	HealthStatusUnknown HealthStatus = iota
	HealthStatusHealthy
	HealthStatusDegraded
	HealthStatusUnhealthy
)

func (h HealthStatus) String() string {
	switch h {
	case HealthStatusHealthy:
		return "healthy"
	case HealthStatusDegraded:
		return "degraded"
	case HealthStatusUnhealthy:
		return "unhealthy"
	default:
		return "unknown"
	}
}

// AllocationStrategy defines how servers are allocated from the pool
type AllocationStrategy int

const (
	AllocationStrategyRoundRobin AllocationStrategy = iota
	AllocationStrategyLeastUsed
	AllocationStrategyMostRecent
	AllocationStrategyRandom
)

func (a AllocationStrategy) String() string {
	switch a {
	case AllocationStrategyRoundRobin:
		return "round_robin"
	case AllocationStrategyLeastUsed:
		return "least_used"
	case AllocationStrategyMostRecent:
		return "most_recent"
	case AllocationStrategyRandom:
		return "random"
	default:
		return "round_robin"
	}
}

// PoolHealth provides aggregate health information for a pool
type PoolHealth struct {
	TotalServers     int                    `json:"total_servers"`
	HealthyServers   int                    `json:"healthy_servers"`
	DegradedServers  int                    `json:"degraded_servers"`
	UnhealthyServers int                    `json:"unhealthy_servers"`
	ActiveServers    int                    `json:"active_servers"`
	IdleServers      int                    `json:"idle_servers"`
	AverageUseCount  float64                `json:"average_use_count"`
	LastHealthCheck  time.Time              `json:"last_health_check"`
	Languages        map[string]*LanguageHealth `json:"languages"`
}

// LanguageHealth provides health information for a specific language pool
type LanguageHealth struct {
	Language         string    `json:"language"`
	TotalServers     int       `json:"total_servers"`
	HealthyServers   int       `json:"healthy_servers"`
	ActiveServers    int       `json:"active_servers"`
	IdleServers      int       `json:"idle_servers"`
	FailedServers    int       `json:"failed_servers"`
	AverageUseCount  float64   `json:"average_use_count"`
	LastUsed         time.Time `json:"last_used"`
	CreationErrors   int       `json:"creation_errors"`
	AllocationErrors int       `json:"allocation_errors"`
}

// PoolMetrics provides detailed metrics about pool performance
type PoolMetrics struct {
	// Allocation metrics
	TotalAllocations  int64         `json:"total_allocations"`
	SuccessfulAllocs  int64         `json:"successful_allocs"`
	FailedAllocs      int64         `json:"failed_allocs"`
	AverageAllocTime  time.Duration `json:"average_alloc_time"`
	MaxAllocTime      time.Duration `json:"max_alloc_time"`
	
	// Server lifecycle metrics
	ServersCreated    int64         `json:"servers_created"`
	ServersDestroyed  int64         `json:"servers_destroyed"`
	ServersFailed     int64         `json:"servers_failed"`
	AverageServerAge  time.Duration `json:"average_server_age"`
	
	// Usage metrics
	TotalUsageTime    time.Duration `json:"total_usage_time"`
	AverageUsageTime  time.Duration `json:"average_usage_time"`
	ReuseRate         float64       `json:"reuse_rate"`
	
	// Performance metrics
	CacheHitRate      float64       `json:"cache_hit_rate"`
	FallbackRate      float64       `json:"fallback_rate"`
	TimeoutRate       float64       `json:"timeout_rate"`
	
	// Resource metrics
	MemoryUsageMB     int64         `json:"memory_usage_mb"`
	PeakMemoryMB      int64         `json:"peak_memory_mb"`
}

// ServerAllocationRequest represents a request for a server from the pool
type ServerAllocationRequest struct {
	Language     string
	Workspace    string
	Timeout      time.Duration
	Context      context.Context
	Priority     int // Higher values = higher priority
	Metadata     map[string]string
}

// ServerAllocationResult represents the result of a server allocation request
type ServerAllocationResult struct {
	Server       *PooledServer
	WasPooled    bool // true if from pool, false if newly created
	AllocationID string
	AllocTime    time.Duration
	Error        error
}