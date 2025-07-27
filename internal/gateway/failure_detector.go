package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// FailureCategory represents different types of LSP server failures
type FailureCategory int32

const (
	FailureCategoryStartup       FailureCategory = iota // Server fails to initialize or start properly
	FailureCategoryRuntime                              // Server becomes unresponsive during operation
	FailureCategoryConfiguration                        // Invalid server configuration or missing dependencies
	FailureCategoryTransport                            // Communication transport issues (STDIO/TCP)
	FailureCategoryResource                             // Memory, CPU, or connection limit exhaustion
	FailureCategoryUnknown                              // Unclassified failure
)

func (fc FailureCategory) String() string {
	switch fc {
	case FailureCategoryStartup:
		return "startup"
	case FailureCategoryRuntime:
		return "runtime"
	case FailureCategoryConfiguration:
		return "configuration"
	case FailureCategoryTransport:
		return "transport"
	case FailureCategoryResource:
		return "resource"
	default:
		return "unknown"
	}
}

// MarshalJSON implements JSON marshaling for FailureCategory
func (fc FailureCategory) MarshalJSON() ([]byte, error) {
	return json.Marshal(fc.String())
}

// FailureSeverity indicates the impact level of a failure
type FailureSeverity int32

const (
	FailureSeverityLow       FailureSeverity = iota // Minor issue, degraded performance
	FailureSeverityMedium                           // Significant issue, service partially affected
	FailureSeverityHigh                             // Critical issue, service severely affected
	FailureSeverityCritical                         // Complete failure, service unavailable
)

func (fs FailureSeverity) String() string {
	switch fs {
	case FailureSeverityLow:
		return "low"
	case FailureSeverityMedium:
		return "medium"
	case FailureSeverityHigh:
		return "high"
	case FailureSeverityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// MarshalJSON implements JSON marshaling for FailureSeverity
func (fs FailureSeverity) MarshalJSON() ([]byte, error) {
	return json.Marshal(fs.String())
}

// FailureContext provides additional context about a failure
type FailureContext struct {
	ServerName    string            `json:"server_name"`
	Language      string            `json:"language"`
	ProjectPath   string            `json:"project_path,omitempty"`
	AffectedFiles []string          `json:"affected_files,omitempty"`
	RequestMethod string            `json:"request_method,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// RecoveryRecommendation provides actionable suggestions for failure recovery
type RecoveryRecommendation struct {
	Action      string   `json:"action"`
	Description string   `json:"description"`
	Commands    []string `json:"commands,omitempty"`
	AutoFix     bool     `json:"auto_fix"`
	Priority    int      `json:"priority"` // Lower number = higher priority
}

// FailureInfo contains comprehensive information about a detected failure
type FailureInfo struct {
	ID                     string                   `json:"id"`
	Category               FailureCategory          `json:"category"`
	Severity               FailureSeverity          `json:"severity"`
	Timestamp              time.Time                `json:"timestamp"`
	ErrorMessage           string                   `json:"error_message"`
	RootCause              string                   `json:"root_cause"`
	Context                *FailureContext          `json:"context"`
	RecoveryRecommendations []RecoveryRecommendation `json:"recovery_recommendations"`
	CircuitBreakerState    string                   `json:"circuit_breaker_state,omitempty"`
	HealthMetrics          *HealthStatusInfo        `json:"health_metrics,omitempty"`
	DetectionSource        string                   `json:"detection_source"`
	IsRecoverable          bool                     `json:"is_recoverable"`
}

// FailureDetectionRule defines a rule for detecting specific failure patterns
type FailureDetectionRule struct {
	Name        string
	Category    FailureCategory
	Severity    FailureSeverity
	Pattern     string // Error message pattern to match
	Detector    func(error, *FailureContext) bool
	Generator   func(error, *FailureContext) *FailureInfo
}

// FailureDetector monitors and categorizes LSP server failures
type FailureDetector struct {
	config              *FailureDetectorConfig
	rules               []*FailureDetectionRule
	recentFailures      map[string][]*FailureInfo // server_name -> failures
	aggregatedFailures  map[string]*AggregatedFailureInfo
	subscribers         []FailureSubscriber
	healthMonitor       *HealthMonitor
	circuitBreakerMgr   *CircuitBreakerManager
	ctx                 context.Context
	cancel              context.CancelFunc
	mu                  sync.RWMutex
}

// FailureDetectorConfig configures the failure detector behavior
type FailureDetectorConfig struct {
	MaxRecentFailures    int           `json:"max_recent_failures"`
	FailureRetentionTime time.Duration `json:"failure_retention_time"`
	AggregationInterval  time.Duration `json:"aggregation_interval"`
	EnableAutoRecovery   bool          `json:"enable_auto_recovery"`
	SeverityThresholds   map[string]int `json:"severity_thresholds"`
}

// DefaultFailureDetectorConfig returns default configuration
func DefaultFailureDetectorConfig() *FailureDetectorConfig {
	return &FailureDetectorConfig{
		MaxRecentFailures:    100,
		FailureRetentionTime: 24 * time.Hour,
		AggregationInterval:  5 * time.Minute,
		EnableAutoRecovery:   true,
		SeverityThresholds: map[string]int{
			"consecutive_failures": 3,
			"error_rate_percent":   25,
			"response_time_ms":     5000,
		},
	}
}

// AggregatedFailureInfo provides summary statistics for failures
type AggregatedFailureInfo struct {
	ServerName           string                      `json:"server_name"`
	TotalFailures        int                         `json:"total_failures"`
	FailuresByCategory   map[FailureCategory]int     `json:"failures_by_category"`
	FailuresBySeverity   map[FailureSeverity]int     `json:"failures_by_severity"`
	FirstFailureTime     time.Time                   `json:"first_failure_time"`
	LastFailureTime      time.Time                   `json:"last_failure_time"`
	MostCommonCategory   FailureCategory             `json:"most_common_category"`
	AverageRecoveryTime  time.Duration               `json:"average_recovery_time"`
	RecoverySuccessRate  float64                     `json:"recovery_success_rate"`
	TopRecommendations   []RecoveryRecommendation    `json:"top_recommendations"`
}

// FailureSubscriber interface for components that want to be notified of failures
type FailureSubscriber interface {
	OnFailureDetected(failure *FailureInfo)
	OnFailureResolved(serverName string, failure *FailureInfo)
}

// NewFailureDetector creates a new failure detector with default configuration
func NewFailureDetector(healthMonitor *HealthMonitor, circuitBreakerMgr *CircuitBreakerManager) *FailureDetector {
	return NewFailureDetectorWithConfig(healthMonitor, circuitBreakerMgr, DefaultFailureDetectorConfig())
}

// NewFailureDetectorWithConfig creates a new failure detector with custom configuration
func NewFailureDetectorWithConfig(healthMonitor *HealthMonitor, circuitBreakerMgr *CircuitBreakerManager, config *FailureDetectorConfig) *FailureDetector {
	ctx, cancel := context.WithCancel(context.Background())
	
	fd := &FailureDetector{
		config:             config,
		recentFailures:     make(map[string][]*FailureInfo),
		aggregatedFailures: make(map[string]*AggregatedFailureInfo),
		subscribers:        make([]FailureSubscriber, 0),
		healthMonitor:      healthMonitor,
		circuitBreakerMgr:  circuitBreakerMgr,
		ctx:                ctx,
		cancel:             cancel,
	}
	
	fd.initializeDetectionRules()
	return fd
}

// Start starts the failure detector monitoring
func (fd *FailureDetector) Start() {
	go fd.aggregationLoop()
	go fd.cleanupLoop()
}

// Stop stops the failure detector
func (fd *FailureDetector) Stop() {
	fd.cancel()
}

// DetectFailure analyzes an error and creates a failure info if it matches detection rules
func (fd *FailureDetector) DetectFailure(err error, context *FailureContext) *FailureInfo {
	if err == nil {
		return nil
	}
	
	fd.mu.Lock()
	defer fd.mu.Unlock()
	
	// Try to match against detection rules
	for _, rule := range fd.rules {
		if rule.Detector(err, context) {
			failure := rule.Generator(err, context)
			if failure == nil {
				failure = fd.createGenericFailure(err, context, rule.Category, rule.Severity)
			}
			
			// Enhance with circuit breaker and health information
			fd.enhanceFailureInfo(failure, context)
			
			// Store the failure
			fd.recordFailure(failure)
			
			// Notify subscribers
			for _, subscriber := range fd.subscribers {
				go subscriber.OnFailureDetected(failure)
			}
			
			return failure
		}
	}
	
	// Create a generic failure if no rules matched
	failure := fd.createGenericFailure(err, context, FailureCategoryUnknown, FailureSeverityMedium)
	fd.enhanceFailureInfo(failure, context)
	fd.recordFailure(failure)
	
	for _, subscriber := range fd.subscribers {
		go subscriber.OnFailureDetected(failure)
	}
	
	return failure
}

// AnalyzeServerHealth analyzes current server health and detects potential failures
func (fd *FailureDetector) AnalyzeServerHealth(serverName string, healthInfo *HealthStatusInfo) []*FailureInfo {
	if healthInfo == nil || healthInfo.IsHealthy {
		return nil
	}
	
	var failures []*FailureInfo
	context := &FailureContext{
		ServerName: serverName,
		Metadata:   make(map[string]string),
	}
	
	// Check consecutive failures
	if healthInfo.ConsecutiveFailures >= fd.config.SeverityThresholds["consecutive_failures"] {
		failure := &FailureInfo{
			ID:              fmt.Sprintf("health_%s_%d", serverName, time.Now().UnixNano()),
			Category:        FailureCategoryRuntime,
			Severity:        fd.determineSeverityFromConsecutiveFailures(healthInfo.ConsecutiveFailures),
			Timestamp:       time.Now(),
			ErrorMessage:    fmt.Sprintf("Server has %d consecutive health check failures", healthInfo.ConsecutiveFailures),
			RootCause:       "Health check failures indicate server is unresponsive or overloaded",
			Context:         context,
			DetectionSource: "health_monitor",
			IsRecoverable:   true,
		}
		
		failure.RecoveryRecommendations = fd.generateHealthFailureRecommendations(healthInfo)
		fd.enhanceFailureInfo(failure, context)
		failures = append(failures, failure)
	}
	
	// Check response time
	if healthInfo.ResponseTime > time.Duration(fd.config.SeverityThresholds["response_time_ms"])*time.Millisecond {
		failure := &FailureInfo{
			ID:              fmt.Sprintf("response_time_%s_%d", serverName, time.Now().UnixNano()),
			Category:        FailureCategoryRuntime,
			Severity:        FailureSeverityMedium,
			Timestamp:       time.Now(),
			ErrorMessage:    fmt.Sprintf("Server response time %v exceeds threshold", healthInfo.ResponseTime),
			RootCause:       "Server is responding slowly, may indicate resource constraints or high load",
			Context:         context,
			DetectionSource: "health_monitor",
			IsRecoverable:   true,
		}
		
		failure.RecoveryRecommendations = fd.generatePerformanceRecommendations(healthInfo)
		fd.enhanceFailureInfo(failure, context)
		failures = append(failures, failure)
	}
	
	// Check error rate
	if healthInfo.ErrorRate*100 > float64(fd.config.SeverityThresholds["error_rate_percent"]) {
		failure := &FailureInfo{
			ID:              fmt.Sprintf("error_rate_%s_%d", serverName, time.Now().UnixNano()),
			Category:        FailureCategoryRuntime,
			Severity:        FailureSeverityHigh,
			Timestamp:       time.Now(),
			ErrorMessage:    fmt.Sprintf("Server error rate %.2f%% exceeds threshold", healthInfo.ErrorRate*100),
			RootCause:       "High error rate indicates systematic issues with server operation",
			Context:         context,
			DetectionSource: "health_monitor",
			IsRecoverable:   true,
		}
		
		failure.RecoveryRecommendations = fd.generateErrorRateRecommendations(healthInfo)
		fd.enhanceFailureInfo(failure, context)
		failures = append(failures, failure)
	}
	
	return failures
}

// AnalyzeCircuitBreakerState analyzes circuit breaker state for failure patterns
func (fd *FailureDetector) AnalyzeCircuitBreakerState(serverName string, circuitBreaker *CircuitBreaker) *FailureInfo {
	if circuitBreaker == nil || !circuitBreaker.IsOpen() {
		return nil
	}
	
	metrics := circuitBreaker.GetMetrics()
	context := &FailureContext{
		ServerName: serverName,
		Metadata: map[string]string{
			"circuit_breaker_state": circuitBreaker.GetState().String(),
			"failed_requests":       fmt.Sprintf("%d", metrics.FailedRequests),
			"total_requests":        fmt.Sprintf("%d", metrics.TotalRequests),
			"error_rate":           fmt.Sprintf("%.2f%%", metrics.GetErrorRate()*100),
		},
	}
	
	failure := &FailureInfo{
		ID:              fmt.Sprintf("circuit_breaker_%s_%d", serverName, time.Now().UnixNano()),
		Category:        FailureCategoryRuntime,
		Severity:        FailureSeverityHigh,
		Timestamp:       time.Now(),
		ErrorMessage:    "Circuit breaker is open, server is not accepting requests",
		RootCause:       fmt.Sprintf("Circuit breaker opened due to high failure rate (%.2f%%)", metrics.GetErrorRate()*100),
		Context:         context,
		DetectionSource: "circuit_breaker",
		IsRecoverable:   true,
	}
	
	failure.RecoveryRecommendations = fd.generateCircuitBreakerRecommendations(circuitBreaker)
	fd.enhanceFailureInfo(failure, context)
	
	return failure
}

// Subscribe adds a failure subscriber
func (fd *FailureDetector) Subscribe(subscriber FailureSubscriber) {
	fd.mu.Lock()
	defer fd.mu.Unlock()
	fd.subscribers = append(fd.subscribers, subscriber)
}

// GetRecentFailures returns recent failures for a server
func (fd *FailureDetector) GetRecentFailures(serverName string) []*FailureInfo {
	fd.mu.RLock()
	defer fd.mu.RUnlock()
	
	failures := fd.recentFailures[serverName]
	result := make([]*FailureInfo, len(failures))
	copy(result, failures)
	return result
}

// GetAggregatedFailures returns aggregated failure information for a server
func (fd *FailureDetector) GetAggregatedFailures(serverName string) *AggregatedFailureInfo {
	fd.mu.RLock()
	defer fd.mu.RUnlock()
	
	if info, exists := fd.aggregatedFailures[serverName]; exists {
		// Return a copy
		result := *info
		return &result
	}
	return nil
}

// GetAllAggregatedFailures returns aggregated failure information for all servers
func (fd *FailureDetector) GetAllAggregatedFailures() map[string]*AggregatedFailureInfo {
	fd.mu.RLock()
	defer fd.mu.RUnlock()
	
	result := make(map[string]*AggregatedFailureInfo)
	for k, v := range fd.aggregatedFailures {
		copied := *v
		result[k] = &copied
	}
	return result
}

// GetFailuresByCategory returns failures filtered by category
func (fd *FailureDetector) GetFailuresByCategory(serverName string, category FailureCategory) []*FailureInfo {
	failures := fd.GetRecentFailures(serverName)
	var filtered []*FailureInfo
	
	for _, failure := range failures {
		if failure.Category == category {
			filtered = append(filtered, failure)
		}
	}
	
	return filtered
}

// GetFailuresBySeverity returns failures filtered by severity
func (fd *FailureDetector) GetFailuresBySeverity(serverName string, severity FailureSeverity) []*FailureInfo {
	failures := fd.GetRecentFailures(serverName)
	var filtered []*FailureInfo
	
	for _, failure := range failures {
		if failure.Severity == severity {
			filtered = append(filtered, failure)
		}
	}
	
	return filtered
}

// initializeDetectionRules sets up the built-in failure detection rules
func (fd *FailureDetector) initializeDetectionRules() {
	fd.rules = []*FailureDetectionRule{
		{
			Name:     "startup_timeout",
			Category: FailureCategoryStartup,
			Severity: FailureSeverityHigh,
			Detector: func(err error, ctx *FailureContext) bool {
				errStr := err.Error()
				return strings.Contains(errStr, "timeout") && strings.Contains(errStr, "start")
			},
			Generator: fd.generateStartupTimeoutFailure,
		},
		{
			Name:     "connection_refused",
			Category: FailureCategoryTransport,
			Severity: FailureSeverityHigh,
			Detector: func(err error, ctx *FailureContext) bool {
				errStr := err.Error()
				return strings.Contains(errStr, "connection refused") || strings.Contains(errStr, "connect: connection refused")
			},
			Generator: fd.generateConnectionRefusedFailure,
		},
		{
			Name:     "broken_pipe",
			Category: FailureCategoryTransport,
			Severity: FailureSeverityMedium,
			Detector: func(err error, ctx *FailureContext) bool {
				errStr := err.Error()
				return strings.Contains(errStr, "broken pipe") || strings.Contains(errStr, "connection reset")
			},
			Generator: fd.generateBrokenPipeFailure,
		},
		{
			Name:     "config_invalid",
			Category: FailureCategoryConfiguration,
			Severity: FailureSeverityHigh,
			Detector: func(err error, ctx *FailureContext) bool {
				errStr := err.Error()
				return strings.Contains(errStr, "config") && (strings.Contains(errStr, "invalid") || strings.Contains(errStr, "missing"))
			},
			Generator: fd.generateConfigFailure,
		},
		{
			Name:     "out_of_memory",
			Category: FailureCategoryResource,
			Severity: FailureSeverityCritical,
			Detector: func(err error, ctx *FailureContext) bool {
				errStr := err.Error()
				return strings.Contains(errStr, "out of memory") || strings.Contains(errStr, "memory limit")
			},
			Generator: fd.generateMemoryFailure,
		},
		{
			Name:     "process_killed",
			Category: FailureCategoryRuntime,
			Severity: FailureSeverityHigh,
			Detector: func(err error, ctx *FailureContext) bool {
				errStr := err.Error()
				return strings.Contains(errStr, "killed") || strings.Contains(errStr, "terminated")
			},
			Generator: fd.generateProcessKilledFailure,
		},
	}
}

// recordFailure stores a failure in the recent failures list
func (fd *FailureDetector) recordFailure(failure *FailureInfo) {
	serverName := failure.Context.ServerName
	
	if fd.recentFailures[serverName] == nil {
		fd.recentFailures[serverName] = make([]*FailureInfo, 0)
	}
	
	fd.recentFailures[serverName] = append(fd.recentFailures[serverName], failure)
	
	// Trim to max size
	if len(fd.recentFailures[serverName]) > fd.config.MaxRecentFailures {
		fd.recentFailures[serverName] = fd.recentFailures[serverName][1:]
	}
}

// enhanceFailureInfo adds circuit breaker and health information to a failure
func (fd *FailureDetector) enhanceFailureInfo(failure *FailureInfo, context *FailureContext) {
	if fd.circuitBreakerMgr != nil {
		if cb := fd.circuitBreakerMgr.GetCircuitBreaker(context.ServerName); cb != nil {
			failure.CircuitBreakerState = cb.GetState().String()
		}
	}
	
	if fd.healthMonitor != nil {
		if healthInfo := fd.healthMonitor.GetHealthStatusInfo(); healthInfo != nil {
			if serverHealth, exists := healthInfo[context.ServerName]; exists {
				failure.HealthMetrics = serverHealth.Copy()
			}
		}
	}
}

// createGenericFailure creates a generic failure info for unmatched errors
func (fd *FailureDetector) createGenericFailure(err error, context *FailureContext, category FailureCategory, severity FailureSeverity) *FailureInfo {
	return &FailureInfo{
		ID:              fmt.Sprintf("generic_%s_%d", context.ServerName, time.Now().UnixNano()),
		Category:        category,
		Severity:        severity,
		Timestamp:       time.Now(),
		ErrorMessage:    err.Error(),
		RootCause:       "Unknown error occurred in LSP server",
		Context:         context,
		DetectionSource: "error_analysis",
		IsRecoverable:   true,
		RecoveryRecommendations: []RecoveryRecommendation{
			{
				Action:      "restart_server",
				Description: "Restart the LSP server to recover from the error",
				Commands:    []string{fmt.Sprintf("lsp-gateway server restart %s", context.ServerName)},
				AutoFix:     true,
				Priority:    1,
			},
			{
				Action:      "check_logs",
				Description: "Check server logs for more detailed error information",
				Commands:    []string{"lsp-gateway diagnose", "lsp-gateway status servers"},
				AutoFix:     false,
				Priority:    2,
			},
		},
	}
}

// aggregationLoop periodically aggregates failure statistics
func (fd *FailureDetector) aggregationLoop() {
	ticker := time.NewTicker(fd.config.AggregationInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-fd.ctx.Done():
			return
		case <-ticker.C:
			fd.aggregateFailures()
		}
	}
}

// cleanupLoop periodically cleans up old failures
func (fd *FailureDetector) cleanupLoop() {
	ticker := time.NewTicker(fd.config.FailureRetentionTime / 4) // Clean up 4 times per retention period
	defer ticker.Stop()
	
	for {
		select {
		case <-fd.ctx.Done():
			return
		case <-ticker.C:
			fd.cleanupOldFailures()
		}
	}
}

// aggregateFailures creates aggregated statistics from recent failures
func (fd *FailureDetector) aggregateFailures() {
	fd.mu.Lock()
	defer fd.mu.Unlock()
	
	for serverName, failures := range fd.recentFailures {
		if len(failures) == 0 {
			continue
		}
		
		aggInfo := &AggregatedFailureInfo{
			ServerName:         serverName,
			TotalFailures:      len(failures),
			FailuresByCategory: make(map[FailureCategory]int),
			FailuresBySeverity: make(map[FailureSeverity]int),
			FirstFailureTime:   failures[0].Timestamp,
			LastFailureTime:    failures[len(failures)-1].Timestamp,
		}
		
		// Count by category and severity
		for _, failure := range failures {
			aggInfo.FailuresByCategory[failure.Category]++
			aggInfo.FailuresBySeverity[failure.Severity]++
		}
		
		// Find most common category
		maxCount := 0
		for category, count := range aggInfo.FailuresByCategory {
			if count > maxCount {
				maxCount = count
				aggInfo.MostCommonCategory = category
			}
		}
		
		// Generate top recommendations based on most common failures
		aggInfo.TopRecommendations = fd.generateTopRecommendations(serverName, aggInfo)
		
		fd.aggregatedFailures[serverName] = aggInfo
	}
}

// cleanupOldFailures removes failures older than the retention time
func (fd *FailureDetector) cleanupOldFailures() {
	fd.mu.Lock()
	defer fd.mu.Unlock()
	
	cutoff := time.Now().Add(-fd.config.FailureRetentionTime)
	
	for serverName, failures := range fd.recentFailures {
		var filtered []*FailureInfo
		for _, failure := range failures {
			if failure.Timestamp.After(cutoff) {
				filtered = append(filtered, failure)
			}
		}
		fd.recentFailures[serverName] = filtered
	}
}

// Helper functions for determining severity and generating recommendations...

func (fd *FailureDetector) determineSeverityFromConsecutiveFailures(failures int) FailureSeverity {
	switch {
	case failures >= 10:
		return FailureSeverityCritical
	case failures >= 5:
		return FailureSeverityHigh
	case failures >= 3:
		return FailureSeverityMedium
	default:
		return FailureSeverityLow
	}
}

// ToJSON converts failure info to JSON
func (fi *FailureInfo) ToJSON() ([]byte, error) {
	return json.Marshal(fi)
}

// ToJSON converts aggregated failure info to JSON
func (afi *AggregatedFailureInfo) ToJSON() ([]byte, error) {
	return json.Marshal(afi)
}

// Failure generator functions for specific failure types

func (fd *FailureDetector) generateStartupTimeoutFailure(err error, context *FailureContext) *FailureInfo {
	return &FailureInfo{
		ID:              fmt.Sprintf("startup_timeout_%s_%d", context.ServerName, time.Now().UnixNano()),
		Category:        FailureCategoryStartup,
		Severity:        FailureSeverityHigh,
		Timestamp:       time.Now(),
		ErrorMessage:    err.Error(),
		RootCause:       "LSP server failed to start within the expected timeout period",
		Context:         context,
		DetectionSource: "startup_monitor",
		IsRecoverable:   true,
		RecoveryRecommendations: []RecoveryRecommendation{
			{
				Action:      "check_binary_path",
				Description: "Verify the LSP server binary exists and is executable",
				Commands:    []string{fmt.Sprintf("which %s", context.ServerName), "lsp-gateway verify server " + context.ServerName},
				AutoFix:     false,
				Priority:    1,
			},
			{
				Action:      "increase_timeout",
				Description: "Increase startup timeout in configuration",
				Commands:    []string{"lsp-gateway config edit"},
				AutoFix:     false,
				Priority:    2,
			},
			{
				Action:      "check_dependencies",
				Description: "Verify runtime dependencies are installed",
				Commands:    []string{"lsp-gateway status runtimes", "lsp-gateway install runtime " + context.Language},
				AutoFix:     true,
				Priority:    3,
			},
		},
	}
}

func (fd *FailureDetector) generateConnectionRefusedFailure(err error, context *FailureContext) *FailureInfo {
	return &FailureInfo{
		ID:              fmt.Sprintf("connection_refused_%s_%d", context.ServerName, time.Now().UnixNano()),
		Category:        FailureCategoryTransport,
		Severity:        FailureSeverityHigh,
		Timestamp:       time.Now(),
		ErrorMessage:    err.Error(),
		RootCause:       "LSP server is not accepting connections, likely not running or misconfigured",
		Context:         context,
		DetectionSource: "transport_monitor",
		IsRecoverable:   true,
		RecoveryRecommendations: []RecoveryRecommendation{
			{
				Action:      "restart_server",
				Description: "Restart the LSP server to reestablish connection",
				Commands:    []string{fmt.Sprintf("lsp-gateway server restart %s", context.ServerName)},
				AutoFix:     true,
				Priority:    1,
			},
			{
				Action:      "check_port_config",
				Description: "Verify server port configuration matches client settings",
				Commands:    []string{"lsp-gateway config validate", "netstat -tlnp"},
				AutoFix:     false,
				Priority:    2,
			},
			{
				Action:      "check_firewall",
				Description: "Check if firewall is blocking the connection",
				Commands:    []string{"sudo ufw status", "sudo iptables -L"},
				AutoFix:     false,
				Priority:    3,
			},
		},
	}
}

func (fd *FailureDetector) generateBrokenPipeFailure(err error, context *FailureContext) *FailureInfo {
	return &FailureInfo{
		ID:              fmt.Sprintf("broken_pipe_%s_%d", context.ServerName, time.Now().UnixNano()),
		Category:        FailureCategoryTransport,
		Severity:        FailureSeverityMedium,
		Timestamp:       time.Now(),
		ErrorMessage:    err.Error(),
		RootCause:       "Connection to LSP server was unexpectedly closed",
		Context:         context,
		DetectionSource: "transport_monitor",
		IsRecoverable:   true,
		RecoveryRecommendations: []RecoveryRecommendation{
			{
				Action:      "reconnect",
				Description: "Attempt to reconnect to the LSP server",
				Commands:    []string{fmt.Sprintf("lsp-gateway server reconnect %s", context.ServerName)},
				AutoFix:     true,
				Priority:    1,
			},
			{
				Action:      "check_server_health",
				Description: "Verify the server process is still running",
				Commands:    []string{"lsp-gateway status servers", "ps aux | grep " + context.ServerName},
				AutoFix:     false,
				Priority:    2,
			},
			{
				Action:      "review_logs",
				Description: "Check server logs for crash information",
				Commands:    []string{"lsp-gateway diagnose", "journalctl -u lsp-gateway"},
				AutoFix:     false,
				Priority:    3,
			},
		},
	}
}

func (fd *FailureDetector) generateConfigFailure(err error, context *FailureContext) *FailureInfo {
	return &FailureInfo{
		ID:              fmt.Sprintf("config_invalid_%s_%d", context.ServerName, time.Now().UnixNano()),
		Category:        FailureCategoryConfiguration,
		Severity:        FailureSeverityHigh,
		Timestamp:       time.Now(),
		ErrorMessage:    err.Error(),
		RootCause:       "LSP server configuration is invalid or incomplete",
		Context:         context,
		DetectionSource: "config_validator",
		IsRecoverable:   true,
		RecoveryRecommendations: []RecoveryRecommendation{
			{
				Action:      "validate_config",
				Description: "Run configuration validation to identify issues",
				Commands:    []string{"lsp-gateway config validate", "lsp-gateway config check " + context.ServerName},
				AutoFix:     false,
				Priority:    1,
			},
			{
				Action:      "regenerate_config",
				Description: "Generate a new configuration with auto-detection",
				Commands:    []string{"lsp-gateway config generate --auto-detect", "lsp-gateway setup " + context.Language},
				AutoFix:     true,
				Priority:    2,
			},
			{
				Action:      "check_paths",
				Description: "Verify all file paths in configuration exist",
				Commands:    []string{"lsp-gateway diagnose paths", "find /usr/local/bin -name '*lsp*'"},
				AutoFix:     false,
				Priority:    3,
			},
		},
	}
}

func (fd *FailureDetector) generateMemoryFailure(err error, context *FailureContext) *FailureInfo {
	return &FailureInfo{
		ID:              fmt.Sprintf("memory_limit_%s_%d", context.ServerName, time.Now().UnixNano()),
		Category:        FailureCategoryResource,
		Severity:        FailureSeverityCritical,
		Timestamp:       time.Now(),
		ErrorMessage:    err.Error(),
		RootCause:       "LSP server exceeded available memory limits",
		Context:         context,
		DetectionSource: "resource_monitor",
		IsRecoverable:   true,
		RecoveryRecommendations: []RecoveryRecommendation{
			{
				Action:      "restart_server",
				Description: "Restart server to clear memory usage",
				Commands:    []string{fmt.Sprintf("lsp-gateway server restart %s", context.ServerName)},
				AutoFix:     true,
				Priority:    1,
			},
			{
				Action:      "increase_memory_limit",
				Description: "Increase memory limits in configuration",
				Commands:    []string{"lsp-gateway config edit memory_limits"},
				AutoFix:     false,
				Priority:    2,
			},
			{
				Action:      "reduce_project_size",
				Description: "Consider excluding large files or directories from indexing",
				Commands:    []string{"lsp-gateway config edit exclude_patterns"},
				AutoFix:     false,
				Priority:    3,
			},
			{
				Action:      "monitor_usage",
				Description: "Monitor memory usage patterns",
				Commands:    []string{"lsp-gateway monitor memory", "htop"},
				AutoFix:     false,
				Priority:    4,
			},
		},
	}
}

func (fd *FailureDetector) generateProcessKilledFailure(err error, context *FailureContext) *FailureInfo {
	return &FailureInfo{
		ID:              fmt.Sprintf("process_killed_%s_%d", context.ServerName, time.Now().UnixNano()),
		Category:        FailureCategoryRuntime,
		Severity:        FailureSeverityHigh,
		Timestamp:       time.Now(),
		ErrorMessage:    err.Error(),
		RootCause:       "LSP server process was terminated unexpectedly",
		Context:         context,
		DetectionSource: "process_monitor",
		IsRecoverable:   true,
		RecoveryRecommendations: []RecoveryRecommendation{
			{
				Action:      "restart_server",
				Description: "Restart the terminated server process",
				Commands:    []string{fmt.Sprintf("lsp-gateway server restart %s", context.ServerName)},
				AutoFix:     true,
				Priority:    1,
			},
			{
				Action:      "check_system_logs",
				Description: "Check system logs for why process was killed",
				Commands:    []string{"dmesg | tail -50", "journalctl -xe", "lsp-gateway diagnose system"},
				AutoFix:     false,
				Priority:    2,
			},
			{
				Action:      "check_oom_killer",
				Description: "Check if OOM killer terminated the process",
				Commands:    []string{"dmesg | grep -i 'killed process'", "cat /var/log/syslog | grep oom"},
				AutoFix:     false,
				Priority:    3,
			},
		},
	}
}

// Recommendation generator functions

func (fd *FailureDetector) generateHealthFailureRecommendations(healthInfo *HealthStatusInfo) []RecoveryRecommendation {
	recommendations := []RecoveryRecommendation{
		{
			Action:      "restart_server",
			Description: "Restart the server to recover from health check failures",
			AutoFix:     true,
			Priority:    1,
		},
	}
	
	if healthInfo.ConsecutiveFailures > 5 {
		recommendations = append(recommendations, RecoveryRecommendation{
			Action:      "check_server_config",
			Description: "Review server configuration for potential issues",
			Commands:    []string{"lsp-gateway config validate"},
			AutoFix:     false,
			Priority:    2,
		})
	}
	
	if healthInfo.RecoveryAttempts > 3 {
		recommendations = append(recommendations, RecoveryRecommendation{
			Action:      "replace_server",
			Description: "Replace the failing server with a new instance",
			AutoFix:     false,
			Priority:    3,
		})
	}
	
	return recommendations
}

func (fd *FailureDetector) generatePerformanceRecommendations(healthInfo *HealthStatusInfo) []RecoveryRecommendation {
	return []RecoveryRecommendation{
		{
			Action:      "check_system_resources",
			Description: "Monitor CPU and memory usage to identify bottlenecks",
			Commands:    []string{"htop", "lsp-gateway monitor resources"},
			AutoFix:     false,
			Priority:    1,
		},
		{
			Action:      "reduce_workload",
			Description: "Reduce concurrent requests or project scope",
			AutoFix:     false,
			Priority:    2,
		},
		{
			Action:      "optimize_config",
			Description: "Tune server configuration for better performance",
			Commands:    []string{"lsp-gateway config tune"},
			AutoFix:     false,
			Priority:    3,
		},
	}
}

func (fd *FailureDetector) generateErrorRateRecommendations(healthInfo *HealthStatusInfo) []RecoveryRecommendation {
	return []RecoveryRecommendation{
		{
			Action:      "investigate_errors",
			Description: "Review error logs to identify common failure patterns",
			Commands:    []string{"lsp-gateway diagnose errors", "lsp-gateway logs"},
			AutoFix:     false,
			Priority:    1,
		},
		{
			Action:      "restart_server",
			Description: "Restart server to clear error state",
			AutoFix:     true,
			Priority:    2,
		},
		{
			Action:      "update_server",
			Description: "Check for LSP server updates or patches",
			Commands:    []string{"lsp-gateway update servers"},
			AutoFix:     false,
			Priority:    3,
		},
	}
}

func (fd *FailureDetector) generateCircuitBreakerRecommendations(circuitBreaker *CircuitBreaker) []RecoveryRecommendation {
	metrics := circuitBreaker.GetMetrics()
	
	recommendations := []RecoveryRecommendation{
		{
			Action:      "wait_for_recovery",
			Description: "Wait for circuit breaker to transition to half-open state",
			AutoFix:     false,
			Priority:    1,
		},
		{
			Action:      "reset_circuit_breaker",
			Description: "Manually reset the circuit breaker if server is healthy",
			Commands:    []string{"lsp-gateway circuit-breaker reset"},
			AutoFix:     false,
			Priority:    2,
		},
	}
	
	if metrics.GetErrorRate() > 0.5 {
		recommendations = append(recommendations, RecoveryRecommendation{
			Action:      "investigate_high_error_rate",
			Description: "High error rate indicates systematic issues requiring investigation",
			Commands:    []string{"lsp-gateway diagnose errors", "lsp-gateway logs --errors"},
			AutoFix:     false,
			Priority:    1,
		})
	}
	
	return recommendations
}

func (fd *FailureDetector) generateTopRecommendations(serverName string, aggInfo *AggregatedFailureInfo) []RecoveryRecommendation {
	recommendations := []RecoveryRecommendation{}
	
	// Generate recommendations based on most common failure category
	switch aggInfo.MostCommonCategory {
	case FailureCategoryStartup:
		recommendations = append(recommendations, RecoveryRecommendation{
			Action:      "fix_startup_issues",
			Description: "Address startup configuration or dependency issues",
			Commands:    []string{"lsp-gateway verify server " + serverName, "lsp-gateway setup " + serverName},
			AutoFix:     false,
			Priority:    1,
		})
	case FailureCategoryRuntime:
		recommendations = append(recommendations, RecoveryRecommendation{
			Action:      "monitor_runtime_health",
			Description: "Implement enhanced monitoring for runtime issues",
			Commands:    []string{"lsp-gateway monitor " + serverName, "lsp-gateway health-check " + serverName},
			AutoFix:     false,
			Priority:    1,
		})
	case FailureCategoryConfiguration:
		recommendations = append(recommendations, RecoveryRecommendation{
			Action:      "review_configuration",
			Description: "Review and fix configuration issues",
			Commands:    []string{"lsp-gateway config validate", "lsp-gateway config regenerate " + serverName},
			AutoFix:     true,
			Priority:    1,
		})
	case FailureCategoryTransport:
		recommendations = append(recommendations, RecoveryRecommendation{
			Action:      "fix_transport_issues",
			Description: "Address network or communication issues",
			Commands:    []string{"lsp-gateway diagnose network", "lsp-gateway transport test " + serverName},
			AutoFix:     false,
			Priority:    1,
		})
	case FailureCategoryResource:
		recommendations = append(recommendations, RecoveryRecommendation{
			Action:      "optimize_resources",
			Description: "Optimize resource usage and limits",
			Commands:    []string{"lsp-gateway config tune resources", "lsp-gateway monitor resources"},
			AutoFix:     false,
			Priority:    1,
		})
	}
	
	// Add general recommendations for high failure rates
	if aggInfo.TotalFailures > 10 {
		recommendations = append(recommendations, RecoveryRecommendation{
			Action:      "consider_replacement",
			Description: "Consider replacing the server with a different LSP implementation",
			AutoFix:     false,
			Priority:    2,
		})
	}
	
	return recommendations
}