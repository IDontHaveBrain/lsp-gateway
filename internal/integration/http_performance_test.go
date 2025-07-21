package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Test Configuration
const (
	TestTimeout       = 15 * time.Second // Reduce from 30s to 15s
	LowConcurrency    = 10
	MediumConcurrency = 25
	HighConcurrency   = 50
	MaxConcurrency    = 100
	MaxErrorRate      = 0.05
)

// SimplePerformanceMetrics for HTTP-level testing
type SimplePerformanceMetrics struct {
	TotalRequests  int64
	SuccessfulReqs int64
	FailedReqs     int64
	StartTime      time.Time
	EndTime        time.Time
	ErrorTypes     map[string]int64
	ResponseTimes  []time.Duration
	mu             sync.RWMutex
}

func NewSimplePerformanceMetrics() *SimplePerformanceMetrics {
	return &SimplePerformanceMetrics{
		ErrorTypes:    make(map[string]int64),
		ResponseTimes: make([]time.Duration, 0, 1000),
		StartTime:     time.Now(),
	}
}

func (spm *SimplePerformanceMetrics) RecordRequest(duration time.Duration, success bool, errorType string) {
	spm.mu.Lock()
	defer spm.mu.Unlock()
	
	atomic.AddInt64(&spm.TotalRequests, 1)
	
	if success {
		atomic.AddInt64(&spm.SuccessfulReqs, 1)
	} else {
		atomic.AddInt64(&spm.FailedReqs, 1)
		if errorType != "" {
			spm.ErrorTypes[errorType]++
		}
	}
	
	// Store response times for analysis
	if len(spm.ResponseTimes) < cap(spm.ResponseTimes) {
		spm.ResponseTimes = append(spm.ResponseTimes, duration)
	}
}

func (spm *SimplePerformanceMetrics) Finalize() {
	spm.mu.Lock()
	defer spm.mu.Unlock()
	spm.EndTime = time.Now()
}

func (spm *SimplePerformanceMetrics) ErrorRate() float64 {
	total := atomic.LoadInt64(&spm.TotalRequests)
	if total == 0 {
		return 0
	}
	failed := atomic.LoadInt64(&spm.FailedReqs)
	return float64(failed) / float64(total)
}

func (spm *SimplePerformanceMetrics) Duration() time.Duration {
	spm.mu.RLock()
	defer spm.mu.RUnlock()
	return spm.EndTime.Sub(spm.StartTime)
}

func (spm *SimplePerformanceMetrics) ThroughputRPS() float64 {
	duration := spm.Duration()
	if duration > 0 {
		return float64(spm.TotalRequests) / duration.Seconds()
	}
	return 0
}

func (spm *SimplePerformanceMetrics) Report() string {
	spm.mu.RLock()
	defer spm.mu.RUnlock()
	
	report := fmt.Sprintf(`HTTP Performance Report:
Duration: %v
Total Requests: %d
Successful: %d
Failed: %d
Error Rate: %.2f%%
Throughput: %.2f RPS`,
		spm.Duration(),
		spm.TotalRequests,
		spm.SuccessfulReqs,
		spm.FailedReqs,
		spm.ErrorRate()*100,
		spm.ThroughputRPS())
	
	if len(spm.ErrorTypes) > 0 {
		report += "\nError Breakdown:\n"
		for errorType, count := range spm.ErrorTypes {
			report += fmt.Sprintf("  %s: %d\n", errorType, count)
		}
	}
	
	return report
}

// MockLSPHTTPServer creates a mock LSP server at HTTP level
type MockLSPHTTPServer struct {
	responseDelay time.Duration
	errorRate     float64
	requestCount  int64
}

func NewMockLSPHTTPServer(responseDelay time.Duration, errorRate float64) *MockLSPHTTPServer {
	return &MockLSPHTTPServer{
		responseDelay: responseDelay,
		errorRate:     errorRate,
	}
}

func (m *MockLSPHTTPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestID := atomic.AddInt64(&m.requestCount, 1)
	
	// Simulate processing delay
	if m.responseDelay > 0 {
		time.Sleep(m.responseDelay)
	}
	
	// Simulate errors
	if m.errorRate > 0 && float64(requestID%100) < m.errorRate*100 {
		http.Error(w, "Mock LSP server error", http.StatusInternalServerError)
		return
	}
	
	// Parse JSON-RPC request
	var jsonRPCReq map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&jsonRPCReq); err != nil {
		http.Error(w, "Invalid JSON-RPC request", http.StatusBadRequest)
		return
	}
	
	// Create mock response
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      jsonRPCReq["id"],
		"result": map[string]interface{}{
			"uri":    "file:///test.go",
			"method": jsonRPCReq["method"],
			"range": map[string]interface{}{
				"start": map[string]interface{}{"line": 10, "character": 5},
				"end":   map[string]interface{}{"line": 10, "character": 15},
			},
		},
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HTTPConcurrentGenerator handles concurrent HTTP requests
type HTTPConcurrentGenerator struct {
	httpClient *http.Client
	baseURL    string
	metrics    *SimplePerformanceMetrics
}

func NewHTTPConcurrentGenerator(baseURL string, metrics *SimplePerformanceMetrics) *HTTPConcurrentGenerator {
	return &HTTPConcurrentGenerator{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    baseURL,
		metrics:    metrics,
	}
}

func (hcg *HTTPConcurrentGenerator) SendConcurrentRequests(ctx context.Context, concurrency, totalRequests int) error {
	semaphore := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	
	for i := 0; i < totalRequests; i++ {
		wg.Add(1)
		
		go func(requestID int) {
			defer wg.Done()
			
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
				hcg.sendSingleRequest(ctx, requestID)
			case <-ctx.Done():
				return
			}
		}(i)
	}
	
	wg.Wait()
	return nil
}

func (hcg *HTTPConcurrentGenerator) sendSingleRequest(ctx context.Context, requestID int) {
	start := time.Now()
	success := false
	errorType := ""
	
	defer func() {
		duration := time.Since(start)
		hcg.metrics.RecordRequest(duration, success, errorType)
	}()
	
	// Create JSON-RPC request
	jsonRPCReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      requestID,
		"method":  "textDocument/definition",
		"params": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": "file:///test.go",
			},
			"position": map[string]interface{}{
				"line":      10,
				"character": 5,
			},
		},
	}
	
	reqBody, err := json.Marshal(jsonRPCReq)
	if err != nil {
		errorType = "marshal_error"
		return
	}
	
	req, err := http.NewRequestWithContext(ctx, "POST", hcg.baseURL, bytes.NewReader(reqBody))
	if err != nil {
		errorType = "request_creation_error"
		return
	}
	
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := hcg.httpClient.Do(req)
	if err != nil {
		errorType = "network_error"
		return
	}
	defer resp.Body.Close()
	
	_, err = io.ReadAll(resp.Body)
	if err != nil {
		errorType = "response_read_error"
		return
	}
	
	if resp.StatusCode != http.StatusOK {
		errorType = fmt.Sprintf("http_%d", resp.StatusCode)
		return
	}
	
	success = true
}

// TEST SUITE

func TestHTTPConcurrentRequests_BurstLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent tests in short mode")
	}
	
	testCases := []struct {
		name        string
		concurrency int
		totalReqs   int
	}{
		{"Low_Burst_10_Concurrent", LowConcurrency, 50},
		{"Medium_Burst_25_Concurrent", MediumConcurrency, 100},
		{"High_Burst_50_Concurrent", HighConcurrency, 200},
		{"Max_Burst_100_Concurrent", MaxConcurrency, 300},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create mock LSP HTTP server
			mockServer := NewMockLSPHTTPServer(1*time.Millisecond, 0.0) // Reduce delay
			httpServer := httptest.NewServer(mockServer)
			defer httpServer.Close()
			
			// Initialize metrics and load generator
			metrics := NewSimplePerformanceMetrics()
			generator := NewHTTPConcurrentGenerator(httpServer.URL, metrics)
			
			// Run test with timeout
			ctx, cancel := context.WithTimeout(context.Background(), TestTimeout)
			defer cancel()
			
			// Execute concurrent requests
			err := generator.SendConcurrentRequests(ctx, tc.concurrency, tc.totalReqs)
			if err != nil {
				t.Fatalf("Failed to send concurrent requests: %v", err)
			}
			
			// Finalize metrics
			metrics.Finalize()
			
			// Log detailed report
			t.Logf("Performance Report for %s:\n%s", tc.name, metrics.Report())
			
			// Validate expectations
			if metrics.TotalRequests != int64(tc.totalReqs) {
				t.Errorf("Expected %d total requests, got %d", tc.totalReqs, metrics.TotalRequests)
			}
			
			if metrics.ErrorRate() > MaxErrorRate {
				t.Errorf("Error rate %.2f%% exceeds maximum %.2f%%", 
					metrics.ErrorRate()*100, MaxErrorRate*100)
			}
			
			if metrics.ThroughputRPS() == 0 {
				t.Error("Expected non-zero throughput")
			}
		})
	}
}

func TestHTTPConcurrentRequests_SustainedLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping sustained load tests in short mode")
	}
	
	const (
		concurrency    = 25
		totalRequests  = 250
		minThroughput  = 20.0 // At least 20 RPS
	)
	
	mockServer := NewMockLSPHTTPServer(1*time.Millisecond, 0.01) // 1% error rate, reduce delay
	httpServer := httptest.NewServer(mockServer)
	defer httpServer.Close()
	
	metrics := NewSimplePerformanceMetrics()
	generator := NewHTTPConcurrentGenerator(httpServer.URL, metrics)
	
	ctx, cancel := context.WithTimeout(context.Background(), TestTimeout)
	defer cancel()
	
	start := time.Now()
	err := generator.SendConcurrentRequests(ctx, concurrency, totalRequests)
	duration := time.Since(start)
	
	if err != nil {
		t.Fatalf("Sustained load test failed: %v", err)
	}
	
	metrics.Finalize()
	
	t.Logf("Sustained Load Test Report:\n%s", metrics.Report())
	
	// Validate performance
	actualRPS := float64(metrics.TotalRequests) / duration.Seconds()
	if actualRPS < minThroughput {
		t.Errorf("Throughput %.2f RPS below minimum %.2f RPS", actualRPS, minThroughput)
	}
	
	if metrics.ErrorRate() > MaxErrorRate {
		t.Errorf("Error rate %.2f%% exceeds maximum %.2f%%", 
			metrics.ErrorRate()*100, MaxErrorRate*100)
	}
}

func TestHTTPConcurrentRequests_ErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping error handling tests in short mode")
	}
	
	// Test with higher error rate
	mockServer := NewMockLSPHTTPServer(5*time.Millisecond, 0.2) // 20% error rate, reduce delay
	httpServer := httptest.NewServer(mockServer)
	defer httpServer.Close()
	
	metrics := NewSimplePerformanceMetrics()
	generator := NewHTTPConcurrentGenerator(httpServer.URL, metrics)
	
	ctx, cancel := context.WithTimeout(context.Background(), TestTimeout)
	defer cancel()
	
	const concurrency = 30
	const totalRequests = 100
	
	err := generator.SendConcurrentRequests(ctx, concurrency, totalRequests)
	if err != nil {
		t.Fatalf("Error handling test failed: %v", err)
	}
	
	metrics.Finalize()
	
	t.Logf("Error Handling Test Report:\n%s", metrics.Report())
	
	// Should have some errors due to configured error rate
	if metrics.ErrorRate() < 0.1 {
		t.Errorf("Expected error rate >= 10%% due to mock server configuration, got %.2f%%", 
			metrics.ErrorRate()*100)
	}
	
	// But should still complete all requests
	if metrics.TotalRequests != totalRequests {
		t.Errorf("Expected %d total requests, got %d", totalRequests, metrics.TotalRequests)
	}
}

func TestHTTPConcurrentRequests_MemoryUsage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory usage tests in short mode")
	}
	
	// Force garbage collection before test
	runtime.GC()
	var initialMem runtime.MemStats
	runtime.ReadMemStats(&initialMem)
	
	mockServer := NewMockLSPHTTPServer(5*time.Millisecond, 0.05) // Reduce delay
	httpServer := httptest.NewServer(mockServer)
	defer httpServer.Close()
	
	metrics := NewSimplePerformanceMetrics()
	generator := NewHTTPConcurrentGenerator(httpServer.URL, metrics)
	
	ctx, cancel := context.WithTimeout(context.Background(), TestTimeout)
	defer cancel()
	
	// High concurrency to test memory usage
	const concurrency = 100
	const totalRequests = 500
	
	err := generator.SendConcurrentRequests(ctx, concurrency, totalRequests)
	if err != nil {
		t.Fatalf("Memory usage test failed: %v", err)
	}
	
	metrics.Finalize()
	
	// Check final memory
	var finalMem runtime.MemStats
	runtime.ReadMemStats(&finalMem)
	
	memoryIncreaseMB := (finalMem.Alloc - initialMem.Alloc) / (1024 * 1024)
	
	t.Logf("Memory Usage Test Report:\n%s", metrics.Report())
	t.Logf("Memory increase: %d MB", memoryIncreaseMB)
	
	// Check for excessive memory usage (allow up to 25MB increase for HTTP-level testing)
	if memoryIncreaseMB > 25 {
		t.Errorf("Excessive memory increase: %d MB", memoryIncreaseMB)
	}
	
	if metrics.ErrorRate() > MaxErrorRate {
		t.Errorf("Error rate %.2f%% exceeds maximum %.2f%%", 
			metrics.ErrorRate()*100, MaxErrorRate*100)
	}
}

func TestHTTPConcurrentRequests_LatencyDistribution(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping latency distribution tests in short mode")
	}
	
	// Test with varying latencies
	mockServer := NewMockLSPHTTPServer(5*time.Millisecond, 0.0) // Reduce delay
	httpServer := httptest.NewServer(mockServer)
	defer httpServer.Close()
	
	metrics := NewSimplePerformanceMetrics()
	generator := NewHTTPConcurrentGenerator(httpServer.URL, metrics)
	
	ctx, cancel := context.WithTimeout(context.Background(), TestTimeout)
	defer cancel()
	
	const concurrency = 40
	const totalRequests = 200
	
	err := generator.SendConcurrentRequests(ctx, concurrency, totalRequests)
	if err != nil {
		t.Fatalf("Latency distribution test failed: %v", err)
	}
	
	metrics.Finalize()
	
	t.Logf("Latency Distribution Test Report:\n%s", metrics.Report())
	
	// Check response time distribution
	metrics.mu.RLock()
	responseTimes := metrics.ResponseTimes
	metrics.mu.RUnlock()
	
	if len(responseTimes) == 0 {
		t.Error("No response times recorded")
		return
	}
	
	// Calculate basic statistics
	var totalTime time.Duration
	var minTime, maxTime time.Duration = time.Hour, 0
	
	for _, rt := range responseTimes {
		totalTime += rt
		if rt < minTime {
			minTime = rt
		}
		if rt > maxTime {
			maxTime = rt
		}
	}
	
	avgTime := totalTime / time.Duration(len(responseTimes))
	
	t.Logf("Response time stats: min=%v, max=%v, avg=%v", minTime, maxTime, avgTime)
	
	// Sanity checks
	if maxTime > 5*time.Second {
		t.Errorf("Maximum response time %v too high", maxTime)
	}
	
	if avgTime > 500*time.Millisecond {
		t.Errorf("Average response time %v too high", avgTime)
	}
}

// BENCHMARK TESTS

func BenchmarkHTTPConcurrentRequests(b *testing.B) {
	concurrencyLevels := []int{10, 25, 50, 100}
	
	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrency_%d", concurrency), func(b *testing.B) {
			mockServer := NewMockLSPHTTPServer(5*time.Millisecond, 0.0)
			httpServer := httptest.NewServer(mockServer)
			defer httpServer.Close()
			
			b.ResetTimer()
			b.SetParallelism(concurrency)
			b.RunParallel(func(pb *testing.PB) {
				client := &http.Client{Timeout: 10 * time.Second}
				
				for pb.Next() {
					sendHTTPBenchmarkRequest(b, client, httpServer.URL)
				}
			})
		})
	}
}

func sendHTTPBenchmarkRequest(b *testing.B, client *http.Client, baseURL string) {
	jsonRPCReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "textDocument/definition",
		"params": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": "file:///test.go",
			},
			"position": map[string]interface{}{
				"line":      10,
				"character": 5,
			},
		},
	}
	
	reqBody, err := json.Marshal(jsonRPCReq)
	if err != nil {
		return // Don't fail benchmark
	}
	
	req, err := http.NewRequest("POST", baseURL, bytes.NewReader(reqBody))
	if err != nil {
		return
	}
	
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	
	_, err = io.ReadAll(resp.Body)
	if err != nil {
		return
	}
}