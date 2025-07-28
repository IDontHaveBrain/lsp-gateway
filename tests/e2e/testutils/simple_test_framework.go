package testutils

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

type SimpleTestFramework struct {
	logger             *SimpleLogger
	performanceValidator *SimplePerformanceValidator
	errorHandler       *SimpleErrorHandler
	timeoutManager     *SimpleTimeoutManager
	config             *SimpleTestConfig
	isRunning          bool
	mu                 sync.RWMutex
}

type SimpleTestConfig struct {
	TestName           string
	TestSuite          string
	EnablePerformance  bool
	EnableErrorRecovery bool
	EnableTimeouts     bool
	EnableLogging      bool
}

func DefaultSimpleTestConfig() *SimpleTestConfig {
	return &SimpleTestConfig{
		TestName:           "python-patterns-e2e",
		TestSuite:          "comprehensive-test", 
		EnablePerformance:  true,
		EnableErrorRecovery: true,
		EnableTimeouts:     true,
		EnableLogging:      true,
	}
}

func NewSimpleTestFramework(config *SimpleTestConfig) *SimpleTestFramework {
	if config == nil {
		config = DefaultSimpleTestConfig()
	}

	logger := NewSimpleLogger()
	
	framework := &SimpleTestFramework{
		logger:             logger,
		performanceValidator: NewSimplePerformanceValidator(),
		errorHandler:       NewSimpleErrorHandler(),
		timeoutManager:     NewSimpleTimeoutManager(),
		config:             config,
	}

	logger.Info("Simple Test Framework initialized", map[string]interface{}{
		"test_name": config.TestName,
	})

	return framework
}

func (f *SimpleTestFramework) StartTestSession(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.isRunning {
		return fmt.Errorf("test session already running")
	}

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

func (f *SimpleTestFramework) StopTestSession() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if !f.isRunning {
		return fmt.Errorf("no test session running")
	}

	f.logger.Info("Stopping test session", map[string]interface{}{})

	if f.config.EnablePerformance {
		f.performanceValidator.StopMonitoring()
	}

	if f.config.EnableTimeouts {
		f.timeoutManager.CancelAllActiveContexts()
	}

	f.isRunning = false
	return nil
}

func (f *SimpleTestFramework) ExecuteWithTimeout(ctx context.Context, operation string, fn func(context.Context) error) error {
	var timeoutCtx context.Context
	var cancel context.CancelFunc

	if f.config.EnableTimeouts {
		timeoutCtx, cancel = f.timeoutManager.CreateTimeoutContext(ctx, operation)
		defer cancel()
	} else {
		timeoutCtx = ctx
	}

	startTime := time.Now()
	f.logger.Debug("Executing operation with timeout", map[string]interface{}{
		"operation": operation,
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
		
		f.performanceValidator.RecordLSPRequest(operation, duration, success, errorType)
	}

	f.logger.Debug("Operation completed", map[string]interface{}{
		"operation": operation,
		"duration":  duration,
		"success":   err == nil,
	})

	return err
}

func (f *SimpleTestFramework) classifyError(err error) string {
	if err == nil {
		return ""
	}

	errorStr := err.Error()
	
	if strings.Contains(errorStr, "timeout") || strings.Contains(errorStr, "deadline exceeded") {
		return "timeout"
	}
	if strings.Contains(errorStr, "connection") {
		return "connection"
	}
	if strings.Contains(errorStr, "network") {
		return "network"
	}

	return "unknown"
}

func (f *SimpleTestFramework) RecordServerStartup(duration time.Duration) {
	if f.config.EnablePerformance {
		f.performanceValidator.RecordServerStartup(duration)
	}
	
	f.logger.Info("Server startup recorded", map[string]interface{}{
		"duration": duration,
	})
}

func (f *SimpleTestFramework) RecordWorkspaceLoad(duration time.Duration) {
	if f.config.EnablePerformance {
		f.performanceValidator.RecordWorkspaceLoad(duration)
	}
	
	f.logger.Info("Workspace load recorded", map[string]interface{}{
		"duration": duration,
	})
}

func (f *SimpleTestFramework) ExecuteConcurrentLSPRequests(ctx context.Context, requests []ConcurrentRequest) (*ConcurrentTestResult, error) {
	if !f.isRunning {
		return nil, fmt.Errorf("test session not running")
	}

	f.logger.Info("Starting concurrent LSP requests", map[string]interface{}{
		"request_count": len(requests),
	})

	timeoutCtx := ctx
	if f.config.EnableTimeouts {
		var cancel context.CancelFunc
		timeoutCtx, cancel = f.timeoutManager.CreateTimeoutContext(ctx, "concurrent_test")
		defer cancel()
	}

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
			err := f.ExecuteWithTimeout(timeoutCtx, req.Method, req.Execute)
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
	}

	f.logger.Info("Concurrent LSP requests completed", map[string]interface{}{
		"total_requests":     result.TotalRequests,
		"successful_requests": result.SuccessfulRequests,
		"failed_requests":     result.FailedRequests,
		"total_duration":      result.TotalDuration,
	})

	return result, nil
}

func (f *SimpleTestFramework) GetComprehensiveReport() *SimpleTestReport {
	report := &SimpleTestReport{
		TestName:  f.config.TestName,
		TestSuite: f.config.TestSuite,
		IsRunning: f.isRunning,
	}

	if f.config.EnablePerformance {
		report.Performance = f.performanceValidator.GetMetrics()
	}

	return report
}

type SimpleTestReport struct {
	TestName    string         `json:"test_name"`
	TestSuite   string         `json:"test_suite"`
	IsRunning   bool           `json:"is_running"`
	Performance *SimpleMetrics `json:"performance,omitempty"`
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
}

func (r *ConcurrentTestResult) CalculateMetrics() {
	if len(r.RequestResults) == 0 {
		return
	}

	var totalDuration time.Duration
	successCount := 0

	for _, result := range r.RequestResults {
		if result.Success {
			totalDuration += result.Duration
			successCount++
		}
	}

	if successCount > 0 {
		r.AverageDuration = totalDuration / time.Duration(successCount)
		r.ThroughputRPS = float64(successCount) / r.TotalDuration.Seconds()
	}
}

func (f *SimpleTestFramework) Cleanup() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.logger.Info("Starting test framework cleanup", map[string]interface{}{})

	if f.isRunning {
		f.StopTestSession()
	}

	if f.config.EnableTimeouts {
		f.timeoutManager.CancelAllActiveContexts()
	}

	if f.config.EnableLogging {
		f.logger.Close()
	}

	f.logger.Info("Test framework cleanup completed", map[string]interface{}{})
}

func (f *SimpleTestFramework) IsCircuitBreakerOpen(operation string) bool {
	if !f.config.EnableErrorRecovery {
		return false
	}

	return f.errorHandler.IsCircuitBreakerOpen(operation)
}

func (f *SimpleTestFramework) GetLogger() *SimpleLogger {
	return f.logger
}

func (f *SimpleTestFramework) GetPerformanceValidator() *SimplePerformanceValidator {
	return f.performanceValidator
}

func (f *SimpleTestFramework) GetErrorHandler() *SimpleErrorHandler {
	return f.errorHandler
}

func (f *SimpleTestFramework) GetTimeoutManager() *SimpleTimeoutManager {
	return f.timeoutManager
}