package mcp

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"lsp-gateway/internal/indexing"
)

// SCIPMCPIntegration provides seamless integration between SCIP and MCP tools
type SCIPMCPIntegration struct {
	// Core components
	enhancedHandler       *SCIPEnhancedToolHandler
	standardHandler       *ToolHandler
	scipStore            indexing.SCIPStore
	symbolResolver       *indexing.SymbolResolver
	workspaceContext     *WorkspaceContext
	
	// Configuration and monitoring
	config               *SCIPMCPConfig
	performanceMonitor   *IntegrationPerformanceMonitor
	healthMonitor        *SCIPHealthMonitor
	configWatcher        *ConfigWatcher
	
	// State management
	initialized          int32 // atomic bool
	scipEnabled          int32 // atomic bool
	fallbackMode         int32 // atomic bool
	shutdownChan         chan struct{}
	mutex                sync.RWMutex
}

// SCIPMCPConfig represents comprehensive configuration for SCIP-MCP integration
type SCIPMCPConfig struct {
	// Feature toggles
	EnableSCIPEnhancements    bool                      `json:"enable_scip_enhancements"`
	EnableFallbackMode        bool                      `json:"enable_fallback_mode"`
	EnablePerformanceMonitoring bool                   `json:"enable_performance_monitoring"`
	EnableHealthChecking      bool                      `json:"enable_health_checking"`
	
	// Performance settings
	PerformanceConfig         *SCIPPerformanceConfig    `json:"performance_config"`
	
	// Tool-specific configurations
	ToolConfigs              map[string]*ToolSpecificConfig `json:"tool_configs"`
	
	// Monitoring and alerting
	MonitoringConfig         *MonitoringConfig         `json:"monitoring_config"`
	
	// Fallback behavior
	FallbackConfig           *FallbackConfig           `json:"fallback_config"`
	
	// Auto-configuration
	AutoConfig               *AutoConfigSettings       `json:"auto_config"`
}

// SCIPPerformanceConfig defines performance thresholds and limits
type SCIPPerformanceConfig struct {
	// Target response times (from requirements)
	TargetSymbolSearchTime    time.Duration `json:"target_symbol_search_time"`    // <50ms
	TargetCrossLanguageTime   time.Duration `json:"target_cross_language_time"`   // <100ms
	TargetContextAnalysisTime time.Duration `json:"target_context_analysis_time"` // <200ms
	TargetSemanticAnalysisTime time.Duration `json:"target_semantic_analysis_time"` // <500ms
	
	// Resource limits
	MaxConcurrentQueries      int           `json:"max_concurrent_queries"`
	MaxMemoryUsage           int64         `json:"max_memory_usage_bytes"`
	MaxCacheSize             int           `json:"max_cache_size"`
	
	// Performance thresholds
	PerformanceDegradationThreshold float64 `json:"performance_degradation_threshold"`
	FailureRateThreshold            float64 `json:"failure_rate_threshold"`
	LatencyP99Threshold            time.Duration `json:"latency_p99_threshold"`
	
	// Auto-tuning
	EnableAutoTuning         bool          `json:"enable_auto_tuning"`
	TuningInterval           time.Duration `json:"tuning_interval"`
}

// ToolSpecificConfig provides per-tool configuration
type ToolSpecificConfig struct {
	Enabled                  bool              `json:"enabled"`
	Timeout                  time.Duration     `json:"timeout"`
	CacheEnabled             bool              `json:"cache_enabled"`
	CacheTTL                 time.Duration     `json:"cache_ttl"`
	MaxResults               int               `json:"max_results"`
	ConfidenceThreshold      float64           `json:"confidence_threshold"`
	FallbackEnabled          bool              `json:"fallback_enabled"`
	CustomSettings           map[string]interface{} `json:"custom_settings"`
}

// MonitoringConfig defines monitoring and alerting behavior
type MonitoringConfig struct {
	EnableMetrics            bool              `json:"enable_metrics"`
	EnableTracing            bool              `json:"enable_tracing"`
	EnableAlerting           bool              `json:"enable_alerting"`
	MetricsInterval          time.Duration     `json:"metrics_interval"`
	HealthCheckInterval      time.Duration     `json:"health_check_interval"`
	AlertThresholds          *AlertThresholds  `json:"alert_thresholds"`
	MetricsRetention         time.Duration     `json:"metrics_retention"`
}

// FallbackConfig defines fallback behavior when SCIP is unavailable
type FallbackConfig struct {
	EnableGracefulDegradation bool             `json:"enable_graceful_degradation"`
	FallbackToStandardTools   bool             `json:"fallback_to_standard_tools"`
	MaxConsecutiveFailures    int              `json:"max_consecutive_failures"`
	RecoveryCheckInterval     time.Duration    `json:"recovery_check_interval"`
	FallbackTimeoutMultiplier float64          `json:"fallback_timeout_multiplier"`
	DisableFeaturesOnFailure  []string         `json:"disable_features_on_failure"`
}

// AutoConfigSettings enables automatic configuration optimization
type AutoConfigSettings struct {
	EnableAutoConfiguration  bool              `json:"enable_auto_configuration"`
	OptimizationGoal         string            `json:"optimization_goal"` // "performance", "accuracy", "balanced"
	LearningPeriod           time.Duration     `json:"learning_period"`
	AdaptationInterval       time.Duration     `json:"adaptation_interval"`
	EnableFeedbackLoop       bool              `json:"enable_feedback_loop"`
}

// AlertThresholds defines when to trigger alerts
type AlertThresholds struct {
	LatencyP99               time.Duration     `json:"latency_p99"`
	ErrorRate                float64           `json:"error_rate"`
	MemoryUsage              float64           `json:"memory_usage"`
	CacheHitRate             float64           `json:"cache_hit_rate"`
	SCIPAvailability         float64           `json:"scip_availability"`
}

// IntegrationPerformanceMonitor tracks overall integration performance
type IntegrationPerformanceMonitor struct {
	// Request metrics
	totalRequests            int64
	scipRequests             int64
	fallbackRequests         int64
	errorRequests            int64
	
	// Timing metrics
	latencyMetrics           *LatencyMetrics
	scipLatencyMetrics       *LatencyMetrics
	fallbackLatencyMetrics   *LatencyMetrics
	
	// Quality metrics
	scipSuccessRate          float64
	fallbackSuccessRate      float64
	overallSuccessRate       float64
	avgConfidence            float64
	
	// Resource metrics
	memoryUsage              int64
	cpuUsage                 float64
	cacheHitRate             float64
	
	// Time-series data
	metricsHistory           []MetricsSnapshot
	maxHistorySize           int
	
	mutex                    sync.RWMutex
	startTime               time.Time
}

// LatencyMetrics tracks detailed latency statistics
type LatencyMetrics struct {
	Min                     time.Duration
	Max                     time.Duration
	Mean                    time.Duration
	P50                     time.Duration
	P90                     time.Duration
	P95                     time.Duration
	P99                     time.Duration
	Count                   int64
	samples                 []time.Duration
	maxSamples              int
}

// MetricsSnapshot represents a point-in-time metrics snapshot
type MetricsSnapshot struct {
	Timestamp               time.Time         `json:"timestamp"`
	RequestRate             float64           `json:"request_rate"`
	ErrorRate               float64           `json:"error_rate"`
	LatencyP99              time.Duration     `json:"latency_p99"`
	MemoryUsage             int64             `json:"memory_usage"`
	CacheHitRate            float64           `json:"cache_hit_rate"`
	SCIPAvailability        bool              `json:"scip_availability"`
	ActiveConnections       int               `json:"active_connections"`
}

// SCIPHealthMonitor monitors SCIP component health
type SCIPHealthMonitor struct {
	scipStore               indexing.SCIPStore
	symbolResolver          *indexing.SymbolResolver
	healthStatus            *HealthStatus
	lastHealthCheck         time.Time
	consecutiveFailures     int64
	recoveryAttempts        int64
	mutex                   sync.RWMutex
}

// HealthStatus represents the health state of SCIP components
type HealthStatus struct {
	Overall                 HealthState       `json:"overall"`
	SCIPStore               HealthState       `json:"scip_store"`
	SymbolResolver          HealthState       `json:"symbol_resolver"`
	Cache                   HealthState       `json:"cache"`
	LastCheck               time.Time         `json:"last_check"`
	Issues                  []HealthIssue     `json:"issues,omitempty"`
	Recommendations         []string          `json:"recommendations,omitempty"`
}

// HealthState represents the health state of a component
type HealthState string

const (
	HealthStateHealthy   HealthState = "healthy"
	HealthStateDegraded  HealthState = "degraded"
	HealthStateUnhealthy HealthState = "unhealthy"
	HealthStateUnknown   HealthState = "unknown"
)

// HealthIssue represents a health issue
type HealthIssue struct {
	Component               string            `json:"component"`
	Severity                string            `json:"severity"`
	Description             string            `json:"description"`
	FirstDetected           time.Time         `json:"first_detected"`
	LastSeen                time.Time         `json:"last_seen"`
	Count                   int64             `json:"count"`
}

// ConfigWatcher monitors configuration changes and applies updates
type ConfigWatcher struct {
	configPath              string
	currentConfig           *SCIPMCPConfig
	lastModified            time.Time
	watchInterval           time.Duration
	changeCallbacks         []ConfigChangeCallback
	stopChan                chan struct{}
	mutex                   sync.RWMutex
}

// ConfigChangeCallback is called when configuration changes
type ConfigChangeCallback func(oldConfig, newConfig *SCIPMCPConfig) error

// NewSCIPMCPIntegration creates a new SCIP-MCP integration instance
func NewSCIPMCPIntegration(
	standardHandler *ToolHandler,
	scipStore indexing.SCIPStore,
	symbolResolver *indexing.SymbolResolver,
	workspaceContext *WorkspaceContext,
	config *SCIPMCPConfig,
) (*SCIPMCPIntegration, error) {
	
	if config == nil {
		config = DefaultSCIPMCPConfig()
	}
	
	integration := &SCIPMCPIntegration{
		standardHandler:     standardHandler,
		scipStore:          scipStore,
		symbolResolver:     symbolResolver,
		workspaceContext:   workspaceContext,
		config:             config,
		shutdownChan:       make(chan struct{}),
	}
	
	// Initialize performance monitoring
	if config.EnablePerformanceMonitoring {
		integration.performanceMonitor = NewIntegrationPerformanceMonitor()
	}
	
	// Initialize health monitoring
	if config.EnableHealthChecking {
		integration.healthMonitor = NewSCIPHealthMonitor(scipStore, symbolResolver)
	}
	
	// Initialize configuration watcher
	integration.configWatcher = NewConfigWatcher(config)
	
	// Initialize enhanced handler if SCIP is available
	if err := integration.initializeSCIPEnhancements(); err != nil {
		log.Printf("SCIP enhancements initialization failed: %v", err)
		atomic.StoreInt32(&integration.fallbackMode, 1)
	}
	
	atomic.StoreInt32(&integration.initialized, 1)
	
	// Start background processes
	integration.startBackgroundProcesses()
	
	return integration, nil
}

// DefaultSCIPMCPConfig returns optimized default configuration
func DefaultSCIPMCPConfig() *SCIPMCPConfig {
	return &SCIPMCPConfig{
		EnableSCIPEnhancements:      true,
		EnableFallbackMode:          true,
		EnablePerformanceMonitoring: true,
		EnableHealthChecking:        true,
		
		PerformanceConfig: &SCIPPerformanceConfig{
			// Performance targets from requirements
			TargetSymbolSearchTime:    50 * time.Millisecond,
			TargetCrossLanguageTime:   100 * time.Millisecond,
			TargetContextAnalysisTime: 200 * time.Millisecond,
			TargetSemanticAnalysisTime: 500 * time.Millisecond,
			
			MaxConcurrentQueries: 100,
			MaxMemoryUsage:       1 << 30, // 1GB
			MaxCacheSize:         10000,
			
			PerformanceDegradationThreshold: 0.2, // 20% degradation
			FailureRateThreshold:           0.05, // 5% failure rate
			LatencyP99Threshold:            1 * time.Second,
			
			EnableAutoTuning:    true,
			TuningInterval:      5 * time.Minute,
		},
		
		ToolConfigs: map[string]*ToolSpecificConfig{
			"scip_intelligent_symbol_search": {
				Enabled:             true,
				Timeout:             50 * time.Millisecond,
				CacheEnabled:        true,
				CacheTTL:           15 * time.Minute,
				MaxResults:         50,
				ConfidenceThreshold: 0.7,
				FallbackEnabled:    true,
			},
			"scip_cross_language_references": {
				Enabled:             true,
				Timeout:             100 * time.Millisecond,
				CacheEnabled:        true,
				CacheTTL:           10 * time.Minute,
				MaxResults:         100,
				ConfidenceThreshold: 0.8,
				FallbackEnabled:    true,
			},
			"scip_semantic_code_analysis": {
				Enabled:             true,
				Timeout:             500 * time.Millisecond,
				CacheEnabled:        true,
				CacheTTL:           30 * time.Minute,
				MaxResults:         1,
				ConfidenceThreshold: 0.75,
				FallbackEnabled:    true,
			},
		},
		
		MonitoringConfig: &MonitoringConfig{
			EnableMetrics:           true,
			EnableTracing:          false, // Disabled by default for performance
			EnableAlerting:         true,
			MetricsInterval:        30 * time.Second,
			HealthCheckInterval:    1 * time.Minute,
			MetricsRetention:       24 * time.Hour,
			AlertThresholds: &AlertThresholds{
				LatencyP99:       1 * time.Second,
				ErrorRate:        0.05,
				MemoryUsage:      0.8,
				CacheHitRate:     0.7,
				SCIPAvailability: 0.95,
			},
		},
		
		FallbackConfig: &FallbackConfig{
			EnableGracefulDegradation: true,
			FallbackToStandardTools:   true,
			MaxConsecutiveFailures:    5,
			RecoveryCheckInterval:     30 * time.Second,
			FallbackTimeoutMultiplier: 2.0,
			DisableFeaturesOnFailure:  []string{"semantic_analysis", "refactoring_suggestions"},
		},
		
		AutoConfig: &AutoConfigSettings{
			EnableAutoConfiguration: true,
			OptimizationGoal:        "balanced",
			LearningPeriod:          1 * time.Hour,
			AdaptationInterval:      15 * time.Minute,
			EnableFeedbackLoop:      true,
		},
	}
}

// GetToolHandler returns the appropriate tool handler (enhanced or standard)
func (integration *SCIPMCPIntegration) GetToolHandler() ToolHandlerInterface {
	if atomic.LoadInt32(&integration.scipEnabled) == 1 && integration.enhancedHandler != nil {
		return integration.enhancedHandler
	}
	return integration.standardHandler
}

// ToolHandlerInterface defines the common interface for tool handlers
type ToolHandlerInterface interface {
	ListTools() []Tool
	CallTool(ctx context.Context, call ToolCall) (*ToolResult, error)
}

// Ensure both handlers implement the interface
var _ ToolHandlerInterface = (*ToolHandler)(nil)
var _ ToolHandlerInterface = (*SCIPEnhancedToolHandler)(nil)

// CallTool routes tool calls through the integration layer with monitoring
func (integration *SCIPMCPIntegration) CallTool(ctx context.Context, call ToolCall) (*ToolResult, error) {
	startTime := time.Now()
	
	// Record request
	atomic.AddInt64(&integration.performanceMonitor.totalRequests, 1)
	
	// Get appropriate handler
	handler := integration.GetToolHandler()
	
	// Track request type
	if integration.enhancedHandler != nil && handler == integration.enhancedHandler {
		atomic.AddInt64(&integration.performanceMonitor.scipRequests, 1)
	} else {
		atomic.AddInt64(&integration.performanceMonitor.fallbackRequests, 1)
	}
	
	// Execute tool call
	result, err := handler.CallTool(ctx, call)
	
	// Record metrics
	duration := time.Since(startTime)
	integration.recordRequestMetrics(call.Name, duration, err != nil)
	
	// Add integration metadata
	if result != nil && result.Meta != nil {
		if result.Meta.RequestInfo == nil {
			result.Meta.RequestInfo = make(map[string]interface{})
		}
		result.Meta.RequestInfo["integration_layer"] = true
		result.Meta.RequestInfo["handler_type"] = integration.getHandlerType(handler)
		result.Meta.RequestInfo["scip_available"] = atomic.LoadInt32(&integration.scipEnabled) == 1
	}
	
	return result, err
}

// ListTools returns all available tools from the current handler
func (integration *SCIPMCPIntegration) ListTools() []Tool {
	handler := integration.GetToolHandler()
	tools := handler.ListTools()
	
	// Add integration metadata to tool descriptions
	for i := range tools {
		if strings.HasPrefix(tools[i].Name, "scip_") {
			tools[i].Description += " (SCIP-enhanced with intelligent fallback)"
		}
	}
	
	return tools
}

// GetPerformanceMetrics returns comprehensive performance metrics
func (integration *SCIPMCPIntegration) GetPerformanceMetrics() map[string]interface{} {
	integration.performanceMonitor.mutex.RLock()
	defer integration.performanceMonitor.mutex.RUnlock()
	
	totalReq := atomic.LoadInt64(&integration.performanceMonitor.totalRequests)
	scipReq := atomic.LoadInt64(&integration.performanceMonitor.scipRequests)
	fallbackReq := atomic.LoadInt64(&integration.performanceMonitor.fallbackRequests)
	errorReq := atomic.LoadInt64(&integration.performanceMonitor.errorRequests)
	
	var errorRate float64
	if totalReq > 0 {
		errorRate = float64(errorReq) / float64(totalReq)
	}
	
	metrics := map[string]interface{}{
		"total_requests":      totalReq,
		"scip_requests":       scipReq,
		"fallback_requests":   fallbackReq,
		"error_requests":      errorReq,
		"error_rate":          errorRate,
		"scip_success_rate":   integration.performanceMonitor.scipSuccessRate,
		"fallback_success_rate": integration.performanceMonitor.fallbackSuccessRate,
		"overall_success_rate": integration.performanceMonitor.overallSuccessRate,
		"avg_confidence":      integration.performanceMonitor.avgConfidence,
		"memory_usage_bytes":  atomic.LoadInt64(&integration.performanceMonitor.memoryUsage),
		"cache_hit_rate":      integration.performanceMonitor.cacheHitRate,
		"uptime_seconds":      time.Since(integration.performanceMonitor.startTime).Seconds(),
		"scip_enabled":        atomic.LoadInt32(&integration.scipEnabled) == 1,
		"fallback_mode":       atomic.LoadInt32(&integration.fallbackMode) == 1,
	}
	
	// Add latency metrics
	if integration.performanceMonitor.latencyMetrics != nil {
		metrics["latency_metrics"] = map[string]interface{}{
			"min_ms":  integration.performanceMonitor.latencyMetrics.Min.Milliseconds(),
			"max_ms":  integration.performanceMonitor.latencyMetrics.Max.Milliseconds(),
			"mean_ms": integration.performanceMonitor.latencyMetrics.Mean.Milliseconds(),
			"p50_ms":  integration.performanceMonitor.latencyMetrics.P50.Milliseconds(),
			"p90_ms":  integration.performanceMonitor.latencyMetrics.P90.Milliseconds(),
			"p95_ms":  integration.performanceMonitor.latencyMetrics.P95.Milliseconds(),
			"p99_ms":  integration.performanceMonitor.latencyMetrics.P99.Milliseconds(),
			"count":   integration.performanceMonitor.latencyMetrics.Count,
		}
	}
	
	return metrics
}

// GetHealthStatus returns current health status
func (integration *SCIPMCPIntegration) GetHealthStatus() *HealthStatus {
	if integration.healthMonitor == nil {
		return &HealthStatus{
			Overall:    HealthStateUnknown,
			LastCheck:  time.Now(),
		}
	}
	
	return integration.healthMonitor.GetHealthStatus()
}

// UpdateConfiguration updates the integration configuration
func (integration *SCIPMCPIntegration) UpdateConfiguration(newConfig *SCIPMCPConfig) error {
	integration.mutex.Lock()
	defer integration.mutex.Unlock()
	
	oldConfig := integration.config
	integration.config = newConfig
	
	// Update enhanced handler configuration if available
	if integration.enhancedHandler != nil {
		enhancedConfig := integration.convertToEnhancedToolConfig(newConfig)
		integration.enhancedHandler.UpdateConfiguration(enhancedConfig)
	}
	
	// Apply configuration changes
	if newConfig.EnableSCIPEnhancements != oldConfig.EnableSCIPEnhancements {
		if newConfig.EnableSCIPEnhancements {
			integration.enableSCIPEnhancements()
		} else {
			integration.disableSCIPEnhancements()
		}
	}
	
	return nil
}

// Private methods

func (integration *SCIPMCPIntegration) initializeSCIPEnhancements() error {
	if !integration.config.EnableSCIPEnhancements {
		return fmt.Errorf("SCIP enhancements disabled in configuration")
	}
	
	if integration.scipStore == nil || integration.symbolResolver == nil {
		return fmt.Errorf("SCIP components not available")
	}
	
	// Create enhanced tool configuration
	enhancedConfig := integration.convertToEnhancedToolConfig(integration.config)
	
	// Create enhanced handler
	enhancedHandler := NewSCIPEnhancedToolHandler(
		integration.standardHandler,
		integration.scipStore,
		integration.symbolResolver,
		integration.workspaceContext,
		enhancedConfig,
	)
	
	integration.enhancedHandler = enhancedHandler
	atomic.StoreInt32(&integration.scipEnabled, 1)
	
	return nil
}

func (integration *SCIPMCPIntegration) convertToEnhancedToolConfig(config *SCIPMCPConfig) *SCIPEnhancedToolConfig {
	enhancedConfig := DefaultSCIPEnhancedToolConfig()
	
	if config.PerformanceConfig != nil {
		enhancedConfig.SymbolSearchTimeout = config.PerformanceConfig.TargetSymbolSearchTime
		enhancedConfig.CrossLanguageTimeout = config.PerformanceConfig.TargetCrossLanguageTime
		enhancedConfig.ContextAnalysisTimeout = config.PerformanceConfig.TargetContextAnalysisTime
		enhancedConfig.MaxQueryTime = config.PerformanceConfig.LatencyP99Threshold
	}
	
	if config.FallbackConfig != nil {
		enhancedConfig.EnableGracefulDegradation = config.FallbackConfig.EnableGracefulDegradation
		enhancedConfig.FallbackToStandardTools = config.FallbackConfig.FallbackToStandardTools
	}
	
	return enhancedConfig
}

func (integration *SCIPMCPIntegration) startBackgroundProcesses() {
	// Start performance monitoring
	if integration.performanceMonitor != nil {
		go integration.performanceMonitorLoop()
	}
	
	// Start health monitoring
	if integration.healthMonitor != nil {
		go integration.healthMonitorLoop()
	}
	
	// Start configuration watching
	if integration.configWatcher != nil {
		go integration.configWatcher.Watch()
	}
	
	// Start auto-tuning if enabled
	if integration.config.AutoConfig != nil && integration.config.AutoConfig.EnableAutoConfiguration {
		go integration.autoTuningLoop()
	}
}

func (integration *SCIPMCPIntegration) performanceMonitorLoop() {
	ticker := time.NewTicker(integration.config.MonitoringConfig.MetricsInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			integration.collectPerformanceMetrics()
		case <-integration.shutdownChan:
			return
		}
	}
}

func (integration *SCIPMCPIntegration) healthMonitorLoop() {
	ticker := time.NewTicker(integration.config.MonitoringConfig.HealthCheckInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			integration.performHealthCheck()
		case <-integration.shutdownChan:
			return
		}
	}
}

func (integration *SCIPMCPIntegration) autoTuningLoop() {
	ticker := time.NewTicker(integration.config.AutoConfig.AdaptationInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			integration.performAutoTuning()
		case <-integration.shutdownChan:
			return
		}
	}
}

func (integration *SCIPMCPIntegration) recordRequestMetrics(toolName string, duration time.Duration, hasError bool) {
	if integration.performanceMonitor == nil {
		return
	}
	
	if hasError {
		atomic.AddInt64(&integration.performanceMonitor.errorRequests, 1)
	}
	
	// Update latency metrics
	integration.performanceMonitor.mutex.Lock()
	if integration.performanceMonitor.latencyMetrics == nil {
		integration.performanceMonitor.latencyMetrics = NewLatencyMetrics()
	}
	integration.performanceMonitor.latencyMetrics.Record(duration)
	integration.performanceMonitor.mutex.Unlock()
}

func (integration *SCIPMCPIntegration) getHandlerType(handler ToolHandlerInterface) string {
	if integration.enhancedHandler != nil && handler == integration.enhancedHandler {
		return "scip_enhanced"
	}
	return "standard"
}

func (integration *SCIPMCPIntegration) enableSCIPEnhancements() {
	if integration.enhancedHandler != nil {
		integration.enhancedHandler.EnableSCIPEnhancements()
		atomic.StoreInt32(&integration.scipEnabled, 1)
		atomic.StoreInt32(&integration.fallbackMode, 0)
	}
}

func (integration *SCIPMCPIntegration) disableSCIPEnhancements() {
	if integration.enhancedHandler != nil {
		integration.enhancedHandler.DisableSCIPEnhancements()
	}
	atomic.StoreInt32(&integration.scipEnabled, 0)
	atomic.StoreInt32(&integration.fallbackMode, 1)
}

func (integration *SCIPMCPIntegration) collectPerformanceMetrics() {
	// Create metrics snapshot
	snapshot := MetricsSnapshot{
		Timestamp:         time.Now(),
		SCIPAvailability:  atomic.LoadInt32(&integration.scipEnabled) == 1,
		MemoryUsage:       atomic.LoadInt64(&integration.performanceMonitor.memoryUsage),
		CacheHitRate:      integration.performanceMonitor.cacheHitRate,
		ActiveConnections: 0, // Would be populated with actual connection count
	}
	
	// Calculate rates
	if len(integration.performanceMonitor.metricsHistory) > 0 {
		prev := integration.performanceMonitor.metricsHistory[len(integration.performanceMonitor.metricsHistory)-1]
		timeDelta := snapshot.Timestamp.Sub(prev.Timestamp).Seconds()
		if timeDelta > 0 {
			snapshot.RequestRate = float64(atomic.LoadInt64(&integration.performanceMonitor.totalRequests)) / timeDelta
			snapshot.ErrorRate = float64(atomic.LoadInt64(&integration.performanceMonitor.errorRequests)) / timeDelta
		}
	}
	
	// Add latency metrics
	if integration.performanceMonitor.latencyMetrics != nil {
		snapshot.LatencyP99 = integration.performanceMonitor.latencyMetrics.P99
	}
	
	// Store snapshot
	integration.performanceMonitor.mutex.Lock()
	integration.performanceMonitor.metricsHistory = append(integration.performanceMonitor.metricsHistory, snapshot)
	
	// Limit history size
	if len(integration.performanceMonitor.metricsHistory) > integration.performanceMonitor.maxHistorySize {
		integration.performanceMonitor.metricsHistory = integration.performanceMonitor.metricsHistory[1:]
	}
	integration.performanceMonitor.mutex.Unlock()
}

func (integration *SCIPMCPIntegration) performHealthCheck() {
	if integration.healthMonitor != nil {
		integration.healthMonitor.PerformHealthCheck()
		
		// Check if SCIP components are healthy
		status := integration.healthMonitor.GetHealthStatus()
		if status.Overall == HealthStateUnhealthy {
			// Switch to fallback mode if SCIP is unhealthy
			atomic.StoreInt32(&integration.scipEnabled, 0)
			atomic.StoreInt32(&integration.fallbackMode, 1)
		} else if status.Overall == HealthStateHealthy && atomic.LoadInt32(&integration.fallbackMode) == 1 {
			// Attempt to re-enable SCIP if it becomes healthy again
			if err := integration.initializeSCIPEnhancements(); err == nil {
				atomic.StoreInt32(&integration.scipEnabled, 1)
				atomic.StoreInt32(&integration.fallbackMode, 0)
			}
		}
	}
}

func (integration *SCIPMCPIntegration) performAutoTuning() {
	// Placeholder for auto-tuning logic
	// Would analyze performance metrics and adjust configuration
	metrics := integration.GetPerformanceMetrics()
	
	// Example: Adjust timeouts based on actual performance
	if p99, ok := metrics["latency_metrics"].(map[string]interface{})["p99_ms"].(int64); ok {
		if time.Duration(p99)*time.Millisecond > integration.config.PerformanceConfig.LatencyP99Threshold {
			// Increase timeouts if P99 is consistently high
			log.Printf("Auto-tuning: P99 latency high (%dms), considering timeout adjustment", p99)
		}
	}
}

// Shutdown gracefully shuts down the integration
func (integration *SCIPMCPIntegration) Shutdown() error {
	close(integration.shutdownChan)
	
	if integration.configWatcher != nil {
		integration.configWatcher.Stop()
	}
	
	atomic.StoreInt32(&integration.initialized, 0)
	
	return nil
}

// Performance monitor implementation

func NewIntegrationPerformanceMonitor() *IntegrationPerformanceMonitor {
	return &IntegrationPerformanceMonitor{
		latencyMetrics: NewLatencyMetrics(),
		maxHistorySize: 1000, // Keep last 1000 snapshots
		startTime:      time.Now(),
	}
}

func NewLatencyMetrics() *LatencyMetrics {
	return &LatencyMetrics{
		maxSamples: 1000,
		samples:    make([]time.Duration, 0, 1000),
	}
}

func (lm *LatencyMetrics) Record(duration time.Duration) {
	lm.samples = append(lm.samples, duration)
	if len(lm.samples) > lm.maxSamples {
		lm.samples = lm.samples[1:]
	}
	
	lm.Count++
	
	// Update min/max
	if lm.Min == 0 || duration < lm.Min {
		lm.Min = duration
	}
	if duration > lm.Max {
		lm.Max = duration
	}
	
	// Calculate percentiles
	lm.calculatePercentiles()
}

func (lm *LatencyMetrics) calculatePercentiles() {
	if len(lm.samples) == 0 {
		return
	}
	
	// Create a sorted copy
	sorted := make([]time.Duration, len(lm.samples))
	copy(sorted, lm.samples)
	
	// Simple sort for small sample sizes
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	
	// Calculate percentiles
	n := len(sorted)
	lm.P50 = sorted[n*50/100]
	lm.P90 = sorted[n*90/100]
	lm.P95 = sorted[n*95/100]
	lm.P99 = sorted[n*99/100]
	
	// Calculate mean
	var total time.Duration
	for _, sample := range sorted {
		total += sample
	}
	lm.Mean = total / time.Duration(n)
}

// Health monitor implementation

func NewSCIPHealthMonitor(scipStore indexing.SCIPStore, symbolResolver *indexing.SymbolResolver) *SCIPHealthMonitor {
	return &SCIPHealthMonitor{
		scipStore:      scipStore,
		symbolResolver: symbolResolver,
		healthStatus:   &HealthStatus{Overall: HealthStateUnknown},
	}
}

func (hm *SCIPHealthMonitor) GetHealthStatus() *HealthStatus {
	hm.mutex.RLock()
	defer hm.mutex.RUnlock()
	
	// Return a copy to avoid concurrent modification
	status := *hm.healthStatus
	return &status
}

func (hm *SCIPHealthMonitor) PerformHealthCheck() {
	hm.mutex.Lock()
	defer hm.mutex.Unlock()
	
	hm.lastHealthCheck = time.Now()
	issues := []HealthIssue{}
	
	// Check SCIP store health
	scipStoreHealth := hm.checkSCIPStoreHealth()
	
	// Check symbol resolver health
	symbolResolverHealth := hm.checkSymbolResolverHealth()
	
	// Check cache health
	cacheHealth := hm.checkCacheHealth()
	
	// Determine overall health
	overallHealth := HealthStateHealthy
	if scipStoreHealth == HealthStateUnhealthy || symbolResolverHealth == HealthStateUnhealthy {
		overallHealth = HealthStateUnhealthy
		hm.consecutiveFailures++
	} else if scipStoreHealth == HealthStateDegraded || symbolResolverHealth == HealthStateDegraded {
		overallHealth = HealthStateDegraded
		hm.consecutiveFailures = 0
	} else {
		hm.consecutiveFailures = 0
	}
	
	// Update health status
	hm.healthStatus = &HealthStatus{
		Overall:        overallHealth,
		SCIPStore:      scipStoreHealth,
		SymbolResolver: symbolResolverHealth,
		Cache:          cacheHealth,
		LastCheck:      hm.lastHealthCheck,
		Issues:         issues,
	}
}

func (hm *SCIPHealthMonitor) checkSCIPStoreHealth() HealthState {
	if hm.scipStore == nil {
		return HealthStateUnhealthy
	}
	
	// Test SCIP store with a simple query
	_, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	result := hm.scipStore.Query("health_check", map[string]interface{}{})
	if result.Error != "" && !strings.Contains(result.Error, "degraded mode") {
		return HealthStateUnhealthy
	}
	
	// Check performance
	if result.QueryTime > 100*time.Millisecond {
		return HealthStateDegraded
	}
	
	return HealthStateHealthy
}

func (hm *SCIPHealthMonitor) checkSymbolResolverHealth() HealthState {
	if hm.symbolResolver == nil {
		return HealthStateUnhealthy
	}
	
	// Get resolver stats
	stats := hm.symbolResolver.GetStats()
	if stats == nil {
		return HealthStateUnhealthy
	}
	
	// Check performance metrics
	if stats.AvgResolutionTime > 50*time.Millisecond {
		return HealthStateDegraded
	}
	
	if stats.CacheHitRate < 0.7 {
		return HealthStateDegraded
	}
	
	return HealthStateHealthy
}

func (hm *SCIPHealthMonitor) checkCacheHealth() HealthState {
	// Placeholder: Would check cache performance and hit rates
	return HealthStateHealthy
}

// Configuration watcher implementation

func NewConfigWatcher(config *SCIPMCPConfig) *ConfigWatcher {
	return &ConfigWatcher{
		currentConfig:  config,
		watchInterval:  30 * time.Second,
		stopChan:       make(chan struct{}),
		changeCallbacks: make([]ConfigChangeCallback, 0),
	}
}

func (cw *ConfigWatcher) Watch() {
	ticker := time.NewTicker(cw.watchInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			cw.checkForConfigChanges()
		case <-cw.stopChan:
			return
		}
	}
}

func (cw *ConfigWatcher) Stop() {
	close(cw.stopChan)
}

func (cw *ConfigWatcher) checkForConfigChanges() {
	// Placeholder: Would check for configuration file changes
	// and reload configuration if necessary
}

func (cw *ConfigWatcher) AddChangeCallback(callback ConfigChangeCallback) {
	cw.mutex.Lock()
	defer cw.mutex.Unlock()
	cw.changeCallbacks = append(cw.changeCallbacks, callback)
}