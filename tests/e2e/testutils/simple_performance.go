package testutils

import (
	"context"
	"sync/atomic"
	"time"
)

type SimplePerformanceValidator struct {
	totalRequests      int64
	successfulRequests int64
	failedRequests     int64
	startTime          time.Time
}

type SimpleMetrics struct {
	TotalRequests      int64         `json:"total_requests"`
	SuccessfulRequests int64         `json:"successful_requests"`
	FailedRequests     int64         `json:"failed_requests"`
	TestDuration       time.Duration `json:"test_duration"`
	ThroughputRPS      float64       `json:"throughput_rps"`
}

func NewSimplePerformanceValidator() *SimplePerformanceValidator {
	return &SimplePerformanceValidator{
		startTime: time.Now(),
	}
}

func (pv *SimplePerformanceValidator) StartMonitoring(ctx context.Context) {
	// Simple implementation - no background monitoring needed
}

func (pv *SimplePerformanceValidator) StopMonitoring() {
	// Simple implementation - no cleanup needed
}

func (pv *SimplePerformanceValidator) RecordServerStartup(duration time.Duration) {
	// Record server startup time if needed
}

func (pv *SimplePerformanceValidator) RecordWorkspaceLoad(duration time.Duration) {
	// Record workspace load time if needed
}

func (pv *SimplePerformanceValidator) RecordLSPRequest(method string, duration time.Duration, success bool, errorType string) {
	atomic.AddInt64(&pv.totalRequests, 1)
	if success {
		atomic.AddInt64(&pv.successfulRequests, 1)
	} else {
		atomic.AddInt64(&pv.failedRequests, 1)
	}
}

func (pv *SimplePerformanceValidator) GetMetrics() *SimpleMetrics {
	duration := time.Since(pv.startTime)
	total := atomic.LoadInt64(&pv.totalRequests)
	
	var throughput float64
	if duration.Seconds() > 0 {
		throughput = float64(total) / duration.Seconds()
	}

	return &SimpleMetrics{
		TotalRequests:      total,
		SuccessfulRequests: atomic.LoadInt64(&pv.successfulRequests),
		FailedRequests:     atomic.LoadInt64(&pv.failedRequests),
		TestDuration:       duration,
		ThroughputRPS:      throughput,
	}
}