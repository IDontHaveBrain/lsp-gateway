package testutils

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type PythonPatternsTestFramework struct {
	logger             *PythonPatternsLogger
	performanceValidator *PythonPatternsPerformanceValidator
	errorHandler       *PythonPatternsErrorHandler
	timeoutManager     *PythonPatternsTimeoutManager
	config             *TestFrameworkConfig
	isRunning          bool
	mu                 sync.RWMutex
}

type TestFrameworkConfig struct {
	TestName           string
	TestSuite          string
	EnablePerformance  bool
	EnableErrorRecovery bool
	EnableTimeouts     bool
	EnableLogging      bool
	LoggingConfig      LoggingConfig
	PerformanceThresholds *PerformanceThresholds
	TimeoutConfig      TimeoutConfig
	WorkspaceDir       string
	RepoURL            string
}

func DefaultTestFrameworkConfig() *TestFrameworkConfig {
	return &TestFrameworkConfig{
		TestName:           "python-patterns-e2e",
		TestSuite:          "comprehensive-test",
		EnablePerformance:  true,
		EnableErrorRecovery: true,
		EnableTimeouts:     true,
		EnableLogging:      true,
		LoggingConfig:      DefaultLoggingConfig(),
		PerformanceThresholds: DefaultPerformanceThresholds(),
		TimeoutConfig:      DefaultTimeoutConfig(),
	}
}

func NewPythonPatternsTestFramework(config *TestFrameworkConfig) *PythonPatternsTestFramework {
	if config == nil {
		config = DefaultTestFrameworkConfig()
	}

	config.LoggingConfig.TestInfo = TestInfo{
		TestName:  config.TestName,
		TestPhase: "initialization",
		TestID:    fmt.Sprintf("%s-%d", config.TestName, time.Now().Unix()),
	}

	logger := NewPythonPatternsLogger(config.LoggingConfig)
	
	framework := &PythonPatternsTestFramework{
		logger:             logger,
		performanceValidator: NewPythonPatternsPerformanceValidator(logger),
		errorHandler:       NewPythonPatternsErrorHandler(logger),
		timeoutManager:     NewPythonPatternsTimeoutManager(logger),
		config:             config,
	}

	logger.Info("Python Patterns Test Framework initialized", map[string]interface{}{
		"config": config,
	})

	return framework
}

func (f *PythonPatternsTestFramework) StartTestSession(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.isRunning {
		return fmt.Errorf("test session already running")
	}

	f.logger.SetTestInfo(f.config.TestName, "test_session_start", f.config.LoggingConfig.TestInfo.TestID)
	f.logger.Info("Starting test session", map[string]interface{}{
		"test_name":  f.config.TestName,
		"test_suite": f.config.TestSuite,
	})

	if f.config.EnablePerformance {
		f.performanceValidator.StartMonitoring(ctx)
	}

	f.isRunning = true
	return nil
}

func (f *PythonPatternsTestFramework) StopTestSession() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if !f.isRunning {
		return fmt.Errorf("no test session running")
	}

	f.logger.SetTestInfo(f.config.TestName, "test_session_stop", f.config.LoggingConfig.TestInfo.TestID)
	f.logger.Info("Stopping test session", nil)

	if f.config.EnablePerformance {
		f.performanceValidator.StopMonitoring()
	}

	if f.config.EnableTimeouts {
		f.timeoutManager.CancelAllActiveContexts()
	}

	f.isRunning = false
	return nil
}

func (f *PythonPatternsTestFramework) ExecuteWithTimeout(ctx context.Context, timeoutType TimeoutType, operation string, fn func(context.Context) error) error {
	if !f.config.EnableTimeouts {
		return fn(ctx)
	}

	timeoutCtx, cancel := f.timeoutManager.CreateTimeoutContext(ctx, timeoutType, operation)
	defer cancel()

	startTime := time.Now()
	f.logger.Debug("Executing operation with timeout", map[string]interface{}{
		"operation":    operation,
		"timeout_type": timeoutType,
		"timeout":      f.timeoutManager.getTimeout(timeoutType),
	})

	err := fn(timeoutCtx)
	duration := time.Since(startTime)

	if err != nil && f.config.EnableErrorRecovery {
		recoveryErr := f.errorHandler.HandleError(timeoutCtx, err, operation)
		if recoveryErr != nil {
			f.logger.Error("Operation failed after error recovery", map[string]interface{}{
				"operation":     operation,
				"duration":      duration,
				"original_error": err.Error(),
				"recovery_error": recoveryErr.Error(),
			})
			return recoveryErr
		}
	}

	if f.config.EnablePerformance {
		success := err == nil
		errorType := ""
		if err != nil {
			errorType = f.classifyError(err)
		}
		
		if timeoutType == TimeoutLSPRequest {
			f.performanceValidator.RecordLSPRequest(operation, duration, success, errorType)
		}
	}

	f.logger.Debug("Operation completed", map[string]interface{}{
		"operation": operation,
		"duration":  duration,
		"success":   err == nil,
		"error":     err,
	})

	return err
}

func (f *PythonPatternsTestFramework) classifyError(err error) string {
	if err == nil {
		return ""
	}

	errorStr := err.Error()
	
	if contains(errorStr, "timeout") || contains(errorStr, "deadline exceeded") {
		return "timeout"
	}
	if contains(errorStr, "connection") {
		return "connection"
	}
	if contains(errorStr, "network") {
		return "network"
	}
	if contains(errorStr, "lsp") {
		return "lsp"
	}
	if contains(errorStr, "git") {
		return "git"
	}

	return "unknown"
}

func (f *PythonPatternsTestFramework) RecordServerStartup(duration time.Duration) {
	if f.config.EnablePerformance {
		f.performanceValidator.RecordServerStartup(duration)
	}
	
	f.logger.Info("Server startup recorded", map[string]interface{}{
		"duration": duration,
	})
}

func (f *PythonPatternsTestFramework) RecordWorkspaceLoad(duration time.Duration) {
	if f.config.EnablePerformance {
		f.performanceValidator.RecordWorkspaceLoad(duration)
	}
	
	f.logger.Info("Workspace load recorded", map[string]interface{}{
		"duration": duration,
	})
}

func (f *PythonPatternsTestFramework) ExecuteConcurrentLSPRequests(ctx context.Context, requests []ConcurrentRequest) (*ConcurrentTestResult, error) {
	if !f.isRunning {
		return nil, fmt.Errorf("test session not running")
	}

	f.logger.SetTestInfo(f.config.TestName, "concurrent_lsp_requests", f.config.LoggingConfig.TestInfo.TestID)
	f.logger.Info("Starting concurrent LSP requests", map[string]interface{}{
		"request_count": len(requests),
	})

	timeoutCtx, cancel := f.timeoutManager.CreateTimeoutContext(ctx, TimeoutConcurrentTest, "concurrent_lsp_requests")
	defer cancel()

	result := &ConcurrentTestResult{
		StartTime:      time.Now(),
		TotalRequests:  len(requests),
		RequestResults: make([]RequestResult, 0, len(requests)),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	
	for i, request := range requests {
		wg.Add(1)
		go func(requestIndex int, req ConcurrentRequest) {
			defer wg.Done()

			requestStart := time.Now()
			err := f.ExecuteWithTimeout(timeoutCtx, TimeoutLSPRequest, req.Method, req.Execute)
			duration := time.Since(requestStart)

			requestResult := RequestResult{
				RequestIndex: requestIndex,
				Method:       req.Method,
				Duration:     duration,
				Success:      err == nil,
				Error:        err,
			}

			mu.Lock()
			result.RequestResults = append(result.RequestResults, requestResult)
			if err == nil {
				result.SuccessfulRequests++
			} else {
				result.FailedRequests++
			}
			mu.Unlock()

		}(i, request)
	}

	wg.Wait()
	result.EndTime = time.Now()
	result.TotalDuration = result.EndTime.Sub(result.StartTime)

	if f.config.EnablePerformance {
		result.CalculateMetrics()
		f.logger.LogMetrics(f.performanceValidator.GetMetrics())
	}

	f.logger.Info("Concurrent LSP requests completed", map[string]interface{}{
		"total_requests":     result.TotalRequests,
		"successful_requests": result.SuccessfulRequests,
		"failed_requests":     result.FailedRequests,
		"total_duration":      result.TotalDuration,
		"average_duration":    result.AverageDuration,
		"throughput_rps":      result.ThroughputRPS,
	})

	return result, nil
}

type ConcurrentRequest struct {
	Method  string
	Execute func(context.Context) error
}

type RequestResult struct {
	RequestIndex int
	Method       string
	Duration     time.Duration
	Success      bool
	Error        error
}

type ConcurrentTestResult struct {
	StartTime          time.Time
	EndTime            time.Time
	TotalDuration      time.Duration
	TotalRequests      int
	SuccessfulRequests int
	FailedRequests     int
	RequestResults     []RequestResult
	AverageDuration    time.Duration
	ThroughputRPS      float64
	Percentiles        map[int]time.Duration
}

func (r *ConcurrentTestResult) CalculateMetrics() {
	if len(r.RequestResults) == 0 {
		return
	}

	var totalDuration time.Duration
	durations := make([]time.Duration, 0, len(r.RequestResults))

	for _, result := range r.RequestResults {
		if result.Success {
			totalDuration += result.Duration
			durations = append(durations, result.Duration)
		}
	}

	if len(durations) > 0 {
		r.AverageDuration = totalDuration / time.Duration(len(durations))
		r.ThroughputRPS = float64(len(durations)) / r.TotalDuration.Seconds()
		
		r.Percentiles = calculatePercentiles(durations)
	}
}

func calculatePercentiles(durations []time.Duration) map[int]time.Duration {
	if len(durations) == 0 {
		return make(map[int]time.Duration)
	}

	// Simple sort
	for i := 0; i < len(durations)-1; i++ {
		for j := 0; j < len(durations)-i-1; j++ {
			if durations[j] > durations[j+1] {
				durations[j], durations[j+1] = durations[j+1], durations[j]
			}
		}
	}

	percentiles := make(map[int]time.Duration)
	percentileValues := []int{50, 90, 95, 99}

	for _, p := range percentileValues {
		index := (len(durations) * p) / 100
		if index >= len(durations) {
			index = len(durations) - 1
		}
		percentiles[p] = durations[index]
	}

	return percentiles
}

func (f *PythonPatternsTestFramework) GetComprehensiveReport() *TestFrameworkReport {
	report := &TestFrameworkReport{
		TestInfo: TestReportInfo{
			TestName:     f.config.TestName,
			TestSuite:    f.config.TestSuite,
			TestID:       f.config.LoggingConfig.TestInfo.TestID,
			GeneratedAt:  time.Now(),
		},
		IsRunning: f.isRunning,
	}

	if f.config.EnablePerformance {
		report.Performance = f.performanceValidator.GetMetrics()
	}

	if f.config.EnableErrorRecovery {
		report.ErrorStats = f.errorHandler.GetErrorStats()
	}

	if f.config.EnableTimeouts {
		report.TimeoutStats = f.timeoutManager.GetTimeoutStats()
	}

	if f.config.EnableLogging {
		report.LogStats = f.logger.GetLogStats()
		report.TestLogger = f.logger.GetTestLogger()
	}

	return report
}

type TestFrameworkReport struct {
	TestInfo     TestReportInfo          `json:"test_info"`
	IsRunning    bool                    `json:"is_running"`
	Performance  *PythonPatternsMetrics  `json:"performance,omitempty"`
	ErrorStats   map[string]interface{}  `json:"error_stats,omitempty"`
	TimeoutStats map[string]interface{}  `json:"timeout_stats,omitempty"`
	LogStats     map[string]interface{}  `json:"log_stats,omitempty"`
	TestLogger   *TestLogger             `json:"test_logger,omitempty"`
}

type TestReportInfo struct {
	TestName    string    `json:"test_name"`
	TestSuite   string    `json:"test_suite"`
	TestID      string    `json:"test_id"`
	GeneratedAt time.Time `json:"generated_at"`
}

func (f *PythonPatternsTestFramework) Cleanup() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.logger.SetTestInfo(f.config.TestName, "cleanup", f.config.LoggingConfig.TestInfo.TestID)
	f.logger.Info("Starting test framework cleanup", nil)

	if f.isRunning {
		f.StopTestSession()
	}

	if f.config.EnableTimeouts {
		f.timeoutManager.CancelAllActiveContexts()
	}

	if f.config.EnableLogging {
		f.logger.Close()
	}

	f.logger.Info("Test framework cleanup completed", nil)
}

func (f *PythonPatternsTestFramework) ValidatePerformanceThresholds() error {
	if !f.config.EnablePerformance {
		return nil
	}

	return f.performanceValidator.validatePerformanceThresholds()
}

func (f *PythonPatternsTestFramework) IsCircuitBreakerOpen(operation string) bool {
	if !f.config.EnableErrorRecovery {
		return false
	}

	return f.errorHandler.IsCircuitBreakerOpen(operation)
}

func (f *PythonPatternsTestFramework) GetLogger() *PythonPatternsLogger {
	return f.logger
}

func (f *PythonPatternsTestFramework) GetPerformanceValidator() *PythonPatternsPerformanceValidator {
	return f.performanceValidator
}

func (f *PythonPatternsTestFramework) GetErrorHandler() *PythonPatternsErrorHandler {
	return f.errorHandler
}

func (f *PythonPatternsTestFramework) GetTimeoutManager() *PythonPatternsTimeoutManager {
	return f.timeoutManager
}