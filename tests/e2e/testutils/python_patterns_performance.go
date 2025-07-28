package testutils

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type PythonPatternsMetrics struct {
	ServerStartupTime    time.Duration              `json:"server_startup_time"`
	InitializationTime   time.Duration              `json:"initialization_time"`
	WorkspaceLoadTime    time.Duration              `json:"workspace_load_time"`
	LSPMethodTimes       map[string]time.Duration   `json:"lsp_method_times"`
	MemoryUsage          int64                      `json:"memory_usage"`
	TotalRequests        int64                      `json:"total_requests"`
	SuccessfulRequests   int64                      `json:"successful_requests"`
	FailedRequests       int64                      `json:"failed_requests"`
	TimeoutErrors        int64                      `json:"timeout_errors"`
	ConnectionErrors     int64                      `json:"connection_errors"`
	ErrorsByType         map[string]int64           `json:"errors_by_type"`
	ResponseTimes        []time.Duration            `json:"response_times"`
	Percentiles          map[int]time.Duration      `json:"percentiles"`
	ThroughputRPS        float64                    `json:"throughput_rps"`
	ConcurrentUsers      int                        `json:"concurrent_users"`
	TestStartTime        time.Time                  `json:"test_start_time"`
	TestDuration         time.Duration              `json:"test_duration"`
	CircuitBreakerState  string                     `json:"circuit_breaker_state"`
	ResourceConsumption  *ResourceMetrics           `json:"resource_consumption"`
	mu                   sync.RWMutex
}

type ResourceMetrics struct {
	CPUUsagePercent    float64 `json:"cpu_usage_percent"`
	MemoryAllocated    int64   `json:"memory_allocated"`
	MemoryInUse        int64   `json:"memory_in_use"`
	GoroutineCount     int     `json:"goroutine_count"`
	GCPauseTotal       int64   `json:"gc_pause_total"`
	GCCycles           uint32  `json:"gc_cycles"`
	HeapObjects        uint64  `json:"heap_objects"`
	StackInUse         uint64  `json:"stack_in_use"`
}

type PythonPatternsPerformanceValidator struct {
	metrics               *PythonPatternsMetrics
	thresholds           *PerformanceThresholds
	circuitBreaker       *CircuitBreaker
	logger               *PythonPatternsLogger
	resourceMonitor      *ResourceMonitor
	running              int32
	stopChan             chan struct{}
	mu                   sync.RWMutex
}

type PerformanceThresholds struct {
	MaxServerStartupTime    time.Duration
	MaxInitializationTime   time.Duration
	MaxLSPMethodTime        time.Duration
	MaxMemoryUsageMB        int64
	MinSuccessRate          float64
	MaxErrorRate            float64
	MinThroughputRPS        float64
	Max95thPercentile       time.Duration
	MaxConcurrentUsers      int
	MaxResponseTimeVariance float64
}

func DefaultPerformanceThresholds() *PerformanceThresholds {
	return &PerformanceThresholds{
		MaxServerStartupTime:    30 * time.Second,
		MaxInitializationTime:   10 * time.Second,
		MaxLSPMethodTime:        5 * time.Second,
		MaxMemoryUsageMB:        200,
		MinSuccessRate:          0.95,
		MaxErrorRate:            0.05,
		MinThroughputRPS:        1.0,
		Max95thPercentile:       5 * time.Second,
		MaxConcurrentUsers:      20,
		MaxResponseTimeVariance: 2.0,
	}
}

func NewPythonPatternsPerformanceValidator(logger *PythonPatternsLogger) *PythonPatternsPerformanceValidator {
	return &PythonPatternsPerformanceValidator{
		metrics: &PythonPatternsMetrics{
			LSPMethodTimes:      make(map[string]time.Duration),
			ErrorsByType:        make(map[string]int64),
			ResponseTimes:       make([]time.Duration, 0, 10000),
			Percentiles:         make(map[int]time.Duration),
			TestStartTime:       time.Now(),
			ResourceConsumption: &ResourceMetrics{},
		},
		thresholds:      DefaultPerformanceThresholds(),
		circuitBreaker:  NewCircuitBreaker("python-patterns", 5, 10*time.Second),
		logger:          logger,
		resourceMonitor: NewResourceMonitor(),
		stopChan:        make(chan struct{}),
	}
}

func (pv *PythonPatternsPerformanceValidator) StartMonitoring(ctx context.Context) {
	if !atomic.CompareAndSwapInt32(&pv.running, 0, 1) {
		return
	}

	pv.logger.Info("Starting performance monitoring", map[string]interface{}{
		"monitoring_interval": "1s",
		"thresholds":         pv.thresholds,
	})

	go pv.monitoringLoop(ctx)
}

func (pv *PythonPatternsPerformanceValidator) StopMonitoring() {
	if !atomic.CompareAndSwapInt32(&pv.running, 1, 0) {
		return
	}

	close(pv.stopChan)
	pv.logger.Info("Performance monitoring stopped", nil)
}

func (pv *PythonPatternsPerformanceValidator) monitoringLoop(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-pv.stopChan:
			return
		case <-ticker.C:
			pv.collectResourceMetrics()
			pv.validatePerformanceThresholds()
		}
	}
}

func (pv *PythonPatternsPerformanceValidator) collectResourceMetrics() {
	pv.mu.Lock()
	defer pv.mu.Unlock()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	pv.metrics.ResourceConsumption = &ResourceMetrics{
		MemoryAllocated: int64(m.Alloc),
		MemoryInUse:     int64(m.Sys),
		GoroutineCount:  runtime.NumGoroutine(),
		GCPauseTotal:    int64(m.PauseTotalNs),
		GCCycles:        m.NumGC,
		HeapObjects:     m.HeapObjects,
		StackInUse:      m.StackInuse,
	}

	pv.metrics.MemoryUsage = int64(m.Alloc)
}

func (pv *PythonPatternsPerformanceValidator) RecordServerStartup(duration time.Duration) {
	pv.mu.Lock()
	defer pv.mu.Unlock()

	pv.metrics.ServerStartupTime = duration
	pv.logger.Debug("Server startup recorded", map[string]interface{}{
		"duration": duration,
		"threshold": pv.thresholds.MaxServerStartupTime,
	})
}

func (pv *PythonPatternsPerformanceValidator) RecordInitialization(duration time.Duration) {
	pv.mu.Lock()
	defer pv.mu.Unlock()

	pv.metrics.InitializationTime = duration
	pv.logger.Debug("Initialization recorded", map[string]interface{}{
		"duration": duration,
		"threshold": pv.thresholds.MaxInitializationTime,
	})
}

func (pv *PythonPatternsPerformanceValidator) RecordWorkspaceLoad(duration time.Duration) {
	pv.mu.Lock()
	defer pv.mu.Unlock()

	pv.metrics.WorkspaceLoadTime = duration
	pv.logger.Debug("Workspace load recorded", map[string]interface{}{
		"duration": duration,
	})
}

func (pv *PythonPatternsPerformanceValidator) RecordLSPRequest(method string, duration time.Duration, success bool, errorType string) {
	pv.mu.Lock()
	defer pv.mu.Unlock()

	atomic.AddInt64(&pv.metrics.TotalRequests, 1)

	if success {
		atomic.AddInt64(&pv.metrics.SuccessfulRequests, 1)
		pv.circuitBreaker.RecordSuccess()
	} else {
		atomic.AddInt64(&pv.metrics.FailedRequests, 1)
		pv.circuitBreaker.RecordFailure()
		
		if errorType != "" {
			pv.metrics.ErrorsByType[errorType]++
		}
	}

	pv.metrics.LSPMethodTimes[method] = duration
	pv.metrics.ResponseTimes = append(pv.metrics.ResponseTimes, duration)

	pv.logger.Debug("LSP request recorded", map[string]interface{}{
		"method":    method,
		"duration":  duration,
		"success":   success,
		"error_type": errorType,
	})
}

func (pv *PythonPatternsPerformanceValidator) RecordTimeoutError() {
	atomic.AddInt64(&pv.metrics.TimeoutErrors, 1)
	pv.metrics.mu.Lock()
	pv.metrics.ErrorsByType["timeout"]++
	pv.metrics.mu.Unlock()
}

func (pv *PythonPatternsPerformanceValidator) RecordConnectionError() {
	atomic.AddInt64(&pv.metrics.ConnectionErrors, 1)
	pv.metrics.mu.Lock()
	pv.metrics.ErrorsByType["connection"]++
	pv.metrics.mu.Unlock()
}

func (pv *PythonPatternsPerformanceValidator) CalculateMetrics() {
	pv.mu.Lock()
	defer pv.mu.Unlock()

	totalRequests := atomic.LoadInt64(&pv.metrics.TotalRequests)
	if totalRequests == 0 {
		return
	}

	pv.metrics.TestDuration = time.Since(pv.metrics.TestStartTime)
	pv.metrics.ThroughputRPS = float64(totalRequests) / pv.metrics.TestDuration.Seconds()

	if len(pv.metrics.ResponseTimes) > 0 {
		pv.calculatePercentiles()
	}

	pv.metrics.CircuitBreakerState = pv.circuitBreaker.State()
}

func (pv *PythonPatternsPerformanceValidator) calculatePercentiles() {
	if len(pv.metrics.ResponseTimes) == 0 {
		return
	}

	times := make([]time.Duration, len(pv.metrics.ResponseTimes))
	copy(times, pv.metrics.ResponseTimes)
	sort.Slice(times, func(i, j int) bool {
		return times[i] < times[j]
	})

	percentiles := []int{50, 90, 95, 99}
	for _, p := range percentiles {
		index := (len(times) * p) / 100
		if index >= len(times) {
			index = len(times) - 1
		}
		pv.metrics.Percentiles[p] = times[index]
	}
}

func (pv *PythonPatternsPerformanceValidator) validatePerformanceThresholds() error {
	pv.mu.RLock()
	defer pv.mu.RUnlock()

	var violations []string

	if pv.metrics.ServerStartupTime > pv.thresholds.MaxServerStartupTime {
		violations = append(violations, fmt.Sprintf("Server startup time %v exceeds threshold %v", 
			pv.metrics.ServerStartupTime, pv.thresholds.MaxServerStartupTime))
	}

	if pv.metrics.InitializationTime > pv.thresholds.MaxInitializationTime {
		violations = append(violations, fmt.Sprintf("Initialization time %v exceeds threshold %v", 
			pv.metrics.InitializationTime, pv.thresholds.MaxInitializationTime))
	}

	memoryMB := pv.metrics.MemoryUsage / (1024 * 1024)
	if memoryMB > pv.thresholds.MaxMemoryUsageMB {
		violations = append(violations, fmt.Sprintf("Memory usage %dMB exceeds threshold %dMB", 
			memoryMB, pv.thresholds.MaxMemoryUsageMB))
	}

	totalRequests := atomic.LoadInt64(&pv.metrics.TotalRequests)
	if totalRequests > 0 {
		successfulRequests := atomic.LoadInt64(&pv.metrics.SuccessfulRequests)
		successRate := float64(successfulRequests) / float64(totalRequests)
		
		if successRate < pv.thresholds.MinSuccessRate {
			violations = append(violations, fmt.Sprintf("Success rate %.2f%% below threshold %.2f%%", 
				successRate*100, pv.thresholds.MinSuccessRate*100))
		}

		errorRate := float64(atomic.LoadInt64(&pv.metrics.FailedRequests)) / float64(totalRequests)
		if errorRate > pv.thresholds.MaxErrorRate {
			violations = append(violations, fmt.Sprintf("Error rate %.2f%% exceeds threshold %.2f%%", 
				errorRate*100, pv.thresholds.MaxErrorRate*100))
		}
	}

	if pv.metrics.ThroughputRPS < pv.thresholds.MinThroughputRPS {
		violations = append(violations, fmt.Sprintf("Throughput %.2f RPS below threshold %.2f RPS", 
			pv.metrics.ThroughputRPS, pv.thresholds.MinThroughputRPS))
	}

	if p95, exists := pv.metrics.Percentiles[95]; exists && p95 > pv.thresholds.Max95thPercentile {
		violations = append(violations, fmt.Sprintf("95th percentile %v exceeds threshold %v", 
			p95, pv.thresholds.Max95thPercentile))
	}

	if len(violations) > 0 {
		pv.logger.Warn("Performance threshold violations detected", map[string]interface{}{
			"violations": violations,
			"metrics":    pv.GetMetrics(),
		})
		return fmt.Errorf("performance threshold violations: %v", violations)
	}

	return nil
}

func (pv *PythonPatternsPerformanceValidator) IsCircuitBreakerOpen() bool {
	return pv.circuitBreaker.State() == "OPEN"
}

func (pv *PythonPatternsPerformanceValidator) GetMetrics() *PythonPatternsMetrics {
	pv.mu.RLock()
	defer pv.mu.RUnlock()

	pv.CalculateMetrics()

	return &PythonPatternsMetrics{
		ServerStartupTime:    pv.metrics.ServerStartupTime,
		InitializationTime:   pv.metrics.InitializationTime,
		WorkspaceLoadTime:    pv.metrics.WorkspaceLoadTime,
		LSPMethodTimes:       copyStringDurationMap(pv.metrics.LSPMethodTimes),
		MemoryUsage:          pv.metrics.MemoryUsage,
		TotalRequests:        atomic.LoadInt64(&pv.metrics.TotalRequests),
		SuccessfulRequests:   atomic.LoadInt64(&pv.metrics.SuccessfulRequests),
		FailedRequests:       atomic.LoadInt64(&pv.metrics.FailedRequests),
		TimeoutErrors:        atomic.LoadInt64(&pv.metrics.TimeoutErrors),
		ConnectionErrors:     atomic.LoadInt64(&pv.metrics.ConnectionErrors),
		ErrorsByType:         copyStringInt64Map(pv.metrics.ErrorsByType),
		Percentiles:          copyIntDurationMap(pv.metrics.Percentiles),
		ThroughputRPS:        pv.metrics.ThroughputRPS,
		ConcurrentUsers:      pv.metrics.ConcurrentUsers,
		TestStartTime:        pv.metrics.TestStartTime,
		TestDuration:         pv.metrics.TestDuration,
		CircuitBreakerState:  pv.circuitBreaker.State(),
		ResourceConsumption:  pv.metrics.ResourceConsumption,
	}
}

func copyStringDurationMap(original map[string]time.Duration) map[string]time.Duration {
	copy := make(map[string]time.Duration)
	for k, v := range original {
		copy[k] = v
	}
	return copy
}

func copyStringInt64Map(original map[string]int64) map[string]int64 {
	copy := make(map[string]int64)
	for k, v := range original {
		copy[k] = v
	}
	return copy
}

func copyIntDurationMap(original map[int]time.Duration) map[int]time.Duration {
	copy := make(map[int]time.Duration)
	for k, v := range original {
		copy[k] = v
	}
	return copy
}

type ResourceMonitor struct {
	mu sync.RWMutex
}

func NewResourceMonitor() *ResourceMonitor {
	return &ResourceMonitor{}
}

func (rm *ResourceMonitor) GetCurrentResourceUsage() *ResourceMetrics {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return &ResourceMetrics{
		MemoryAllocated: int64(m.Alloc),
		MemoryInUse:     int64(m.Sys),
		GoroutineCount:  runtime.NumGoroutine(),
		GCPauseTotal:    int64(m.PauseTotalNs),
		GCCycles:        m.NumGC,
		HeapObjects:     m.HeapObjects,
		StackInUse:      m.StackInuse,
	}
}