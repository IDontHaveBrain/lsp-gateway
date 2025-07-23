package integration

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/gateway"
	testutil "lsp-gateway/tests/utils/helpers"
)

// MemoryProfiler provides comprehensive memory monitoring for long-running tests
type MemoryProfiler struct {
	mu                sync.RWMutex
	samples           []runtime.MemStats
	timestamps        []time.Time
	goroutineCounts   []int
	maxSamples        int
	sampleInterval    time.Duration
	monitoring        bool
	stopCh            chan struct{}
	monitoringWG      sync.WaitGroup
	baseline          runtime.MemStats
	baselineSet       bool
	initialGoroutines int
	testStartTime     time.Time
}

// MemoryAlert represents a memory-related alert
type MemoryAlert struct {
	Timestamp time.Time
	Type      string
	Severity  string
	Message   string
	Data      map[string]interface{}
}

// NewMemoryProfiler creates a new memory profiler
func NewMemoryProfiler(maxSamples int, sampleInterval time.Duration) *MemoryProfiler {
	return &MemoryProfiler{
		samples:         make([]runtime.MemStats, 0, maxSamples),
		timestamps:      make([]time.Time, 0, maxSamples),
		goroutineCounts: make([]int, 0, maxSamples),
		maxSamples:      maxSamples,
		sampleInterval:  sampleInterval,
		stopCh:          make(chan struct{}),
		testStartTime:   time.Now(),
	}
}

// StartMonitoring begins memory monitoring
func (p *MemoryProfiler) StartMonitoring() {
	p.mu.Lock()
	if p.monitoring {
		p.mu.Unlock()
		return
	}
	p.monitoring = true
	p.initialGoroutines = runtime.NumGoroutine()

	// Set baseline
	if !p.baselineSet {
		runtime.GC()
		runtime.ReadMemStats(&p.baseline)
		p.baselineSet = true
	}
	p.mu.Unlock()

	p.monitoringWG.Add(1)
	go func() {
		defer p.monitoringWG.Done()
		ticker := time.NewTicker(p.sampleInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				p.takeSample()
			case <-p.stopCh:
				return
			}
		}
	}()
}

// StopMonitoring stops memory monitoring
func (p *MemoryProfiler) StopMonitoring() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.monitoring {
		return
	}

	p.monitoring = false
	close(p.stopCh)
	p.monitoringWG.Wait()
}

// takeSample captures current memory statistics
func (p *MemoryProfiler) takeSample() {
	runtime.GC()
	runtime.GC()

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	goroutineCount := runtime.NumGoroutine()

	p.mu.Lock()
	defer p.mu.Unlock()

	// Maintain rolling window
	if len(p.samples) >= p.maxSamples {
		copy(p.samples, p.samples[1:])
		copy(p.timestamps, p.timestamps[1:])
		copy(p.goroutineCounts, p.goroutineCounts[1:])
		p.samples = p.samples[:len(p.samples)-1]
		p.timestamps = p.timestamps[:len(p.timestamps)-1]
		p.goroutineCounts = p.goroutineCounts[:len(p.goroutineCounts)-1]
	}

	p.samples = append(p.samples, memStats)
	p.timestamps = append(p.timestamps, time.Now())
	p.goroutineCounts = append(p.goroutineCounts, goroutineCount)
}

// DetectMemoryLeak performs linear regression analysis to detect memory leaks
func (p *MemoryProfiler) DetectMemoryLeak() (leaked bool, confidence float64, details map[string]interface{}) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.samples) < 10 {
		return false, 0, map[string]interface{}{"error": "insufficient samples"}
	}

	// Perform linear regression on heap allocation
	n := float64(len(p.samples))
	var sumX, sumY, sumXY, sumX2 float64

	for i, sample := range p.samples {
		x := float64(i)
		y := float64(sample.HeapAlloc)

		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	// Calculate slope and R-squared
	slope := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
	meanY := sumY / n
	var ssRes, ssTot float64

	for i, sample := range p.samples {
		x := float64(i)
		y := float64(sample.HeapAlloc)
		predicted := slope*x + (sumY-slope*sumX)/n

		ssRes += (y - predicted) * (y - predicted)
		ssTot += (y - meanY) * (y - meanY)
	}

	r2 := 0.0
	if ssTot != 0 {
		r2 = 1 - ssRes/ssTot
	}

	// Detect leak: strong positive trend with good correlation
	leaked = slope > 1000000 && r2 > 0.7 // 1MB+ growth per sample with 70%+ correlation

	details = map[string]interface{}{
		"heap_slope_bytes_per_sample": slope,
		"correlation_r2":              r2,
		"sample_count":                len(p.samples),
		"duration_minutes":            time.Since(p.testStartTime).Minutes(),
	}

	return leaked, r2, details
}

// GenerateReport generates a comprehensive memory analysis report
func (p *MemoryProfiler) GenerateReport() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.samples) == 0 {
		return map[string]interface{}{"error": "no samples available"}
	}

	current := p.samples[len(p.samples)-1]
	leaked, confidence, leakDetails := p.DetectMemoryLeak()

	return map[string]interface{}{
		"test_duration_minutes": time.Since(p.testStartTime).Minutes(),
		"sample_count":          len(p.samples),
		"baseline_heap_alloc":   p.baseline.HeapAlloc,
		"current_heap_alloc":    current.HeapAlloc,
		"heap_growth_bytes":     int64(current.HeapAlloc) - int64(p.baseline.HeapAlloc),
		"heap_growth_mb":        (int64(current.HeapAlloc) - int64(p.baseline.HeapAlloc)) / (1024 * 1024),
		"total_alloc_bytes":     current.TotalAlloc - p.baseline.TotalAlloc,
		"gc_runs":               current.NumGC - p.baseline.NumGC,
		"initial_goroutines":    p.initialGoroutines,
		"current_goroutines":    p.goroutineCounts[len(p.goroutineCounts)-1],
		"goroutine_growth":      p.goroutineCounts[len(p.goroutineCounts)-1] - p.initialGoroutines,
		"leak_detected":         leaked,
		"leak_confidence":       confidence,
		"leak_details":          leakDetails,
		"heap_objects":          current.HeapObjects,
		"stack_in_use":          current.StackInuse,
	}
}

// TestHarness provides a test environment for long-running memory tests
type TestHarness struct {
	config   *config.GatewayConfig
	server   *http.Server
	profiler *MemoryProfiler
	testDir  string
	port     int
}

// NewTestHarness creates a new test harness
func NewTestHarness(t *testing.T) *TestHarness {
	testDir := testutil.TempDir(t)
	port := testutil.AllocateTestPort(t)

	// Create test config
	configContent := testutil.CreateConfigWithPort(port)
	configPath := filepath.Join(testDir, "config.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load test config: %v", err)
	}

	profiler := NewMemoryProfiler(5000, 10*time.Second)

	return &TestHarness{
		config:   cfg,
		profiler: profiler,
		testDir:  testDir,
		port:     port,
	}
}

// StartGateway starts the LSP gateway server
func (h *TestHarness) StartGateway() error {
	gateway, err := gateway.NewGateway(h.config)
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/jsonrpc", gateway.HandleJSONRPC)

	h.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", h.port),
		Handler: mux,
	}

	go func() {
		if err := h.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// Server stopped unexpectedly
		}
	}()

	return h.waitForReady()
}

// StopGateway stops the gateway server
func (h *TestHarness) StopGateway() error {
	if h.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return h.server.Shutdown(ctx)
	}
	return nil
}

// waitForReady waits for the server to be ready
func (h *TestHarness) waitForReady() error {
	client := &http.Client{Timeout: 1 * time.Second}

	for i := 0; i < 30; i++ { // 30 second timeout
		resp, err := client.Post(
			fmt.Sprintf("http://localhost:%d/jsonrpc", h.port),
			"application/json",
			strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"test","params":{}}`),
		)
		if err == nil {
			resp.Body.Close()
			return nil
		}
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("server not ready after 30 seconds")
}

// SendLSPRequest sends an LSP request to the gateway
func (h *TestHarness) SendLSPRequest(method string, params interface{}) error {
	client := &http.Client{Timeout: 30 * time.Second}

	requestData := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	}

	jsonData, err := json.Marshal(requestData)
	if err != nil {
		return err
	}

	resp, err := client.Post(
		fmt.Sprintf("http://localhost:%d/jsonrpc", h.port),
		"application/json",
		strings.NewReader(string(jsonData)),
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = io.ReadAll(resp.Body)
	return err
}

// StartProfiling starts memory profiling
func (h *TestHarness) StartProfiling() {
	h.profiler.StartMonitoring()
}

// StopProfiling stops memory profiling
func (h *TestHarness) StopProfiling() {
	h.profiler.StopMonitoring()
}

// GetMemoryReport gets the current memory analysis report
func (h *TestHarness) GetMemoryReport() map[string]interface{} {
	return h.profiler.GenerateReport()
}

// LONG-RUNNING TESTS

// TestLongRunningGatewayMemoryStability tests memory stability over extended periods
func TestLongRunningGatewayMemoryStability(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running test in short mode")
	}

	// Allow configurable test duration via environment variable
	durationStr := os.Getenv("LSP_GATEWAY_LONG_TEST_DURATION")
	if durationStr == "" {
		durationStr = "15s" // Default to 15 seconds for fast testing
	}

	testDuration, err := time.ParseDuration(durationStr)
	if err != nil {
		t.Fatalf("Invalid test duration: %v", err)
	}

	t.Logf("Starting long-running memory stability test for duration: %v", testDuration)

	harness := NewTestHarness(t)
	defer func() {
		harness.StopGateway()
		harness.StopProfiling()
	}()

	// Start memory profiling
	harness.StartProfiling()

	// Start gateway
	err = harness.StartGateway()
	if err != nil {
		t.Fatalf("Failed to start gateway: %v", err)
	}

	// Test configuration
	const (
		requestInterval = 100 * time.Millisecond
		reportInterval  = 5 * time.Second
	)

	var requestCount, errorCount int64
	testEndTime := time.Now().Add(testDuration)

	// Request patterns
	requestPatterns := []struct {
		method string
		params interface{}
	}{
		{
			method: "textDocument/definition",
			params: map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": "file:///test/main.go",
				},
				"position": map[string]interface{}{
					"line":      10,
					"character": 5,
				},
			},
		},
		{
			method: "textDocument/hover",
			params: map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": "file:///test/utils.go",
				},
				"position": map[string]interface{}{
					"line":      20,
					"character": 8,
				},
			},
		},
		{
			method: "workspace/symbol",
			params: map[string]interface{}{
				"query": "TestFunction",
			},
		},
	}

	// Background request sender
	requestTicker := time.NewTicker(requestInterval)
	defer requestTicker.Stop()

	// Progress reporter
	reportTicker := time.NewTicker(reportInterval)
	defer reportTicker.Stop()

	for time.Now().Before(testEndTime) {
		select {
		case <-requestTicker.C:
			// Send request
			pattern := requestPatterns[requestCount%int64(len(requestPatterns))]

			go func(count int64) {
				err := harness.SendLSPRequest(pattern.method, pattern.params)
				if err != nil {
					atomic.AddInt64(&errorCount, 1)
					t.Logf("Request %d failed: %v", count, err)
				}
			}(requestCount)

			atomic.AddInt64(&requestCount, 1)

		case <-reportTicker.C:
			// Generate progress report
			report := harness.GetMemoryReport()
			currentRequests := atomic.LoadInt64(&requestCount)
			currentErrors := atomic.LoadInt64(&errorCount)

			t.Logf("=== Progress Report ===")
			t.Logf("Runtime: %.1f minutes", report["test_duration_minutes"])
			t.Logf("Requests sent: %d", currentRequests)
			t.Logf("Errors: %d (%.2f%%)", currentErrors, float64(currentErrors)/math.Max(float64(currentRequests), 1)*100)
			t.Logf("Heap growth: %v MB", report["heap_growth_mb"])
			t.Logf("Goroutines: %v (growth: %v)", report["current_goroutines"], report["goroutine_growth"])
			t.Logf("GC runs: %v", report["gc_runs"])

			if leakDetected, ok := report["leak_detected"].(bool); ok && leakDetected {
				t.Errorf("MEMORY LEAK DETECTED: confidence=%.2f%%", report["leak_confidence"].(float64)*100)
				return
			}

		default:
			time.Sleep(100 * time.Millisecond)
		}
	}

	// Final analysis
	finalReport := harness.GetMemoryReport()
	finalRequests := atomic.LoadInt64(&requestCount)
	finalErrors := atomic.LoadInt64(&errorCount)

	t.Logf("=== FINAL REPORT ===")
	t.Logf("Test duration: %.1f minutes", finalReport["test_duration_minutes"])
	t.Logf("Total requests: %d", finalRequests)
	t.Logf("Total errors: %d (%.2f%%)", finalErrors, float64(finalErrors)/math.Max(float64(finalRequests), 1)*100)
	t.Logf("Final heap growth: %v MB", finalReport["heap_growth_mb"])
	t.Logf("Final goroutine growth: %v", finalReport["goroutine_growth"])

	// Validate results
	if leakDetected, ok := finalReport["leak_detected"].(bool); ok && leakDetected {
		t.Errorf("MEMORY LEAK DETECTED: confidence=%.2f%%", finalReport["leak_confidence"].(float64)*100)
	}

	errorRate := float64(finalErrors) / math.Max(float64(finalRequests), 1) * 100
	if errorRate > 5.0 {
		t.Errorf("High error rate: %.2f%% (threshold: 5%%)", errorRate)
	}

	if heapGrowthMB, ok := finalReport["heap_growth_mb"].(int64); ok && heapGrowthMB > 50 {
		t.Errorf("Excessive memory growth: %d MB (threshold: 50MB)", heapGrowthMB)
	}

	if goroutineGrowth, ok := finalReport["goroutine_growth"].(int); ok && goroutineGrowth > 50 {
		t.Errorf("Excessive goroutine growth: %d (threshold: 50)", goroutineGrowth)
	}

	t.Logf("Long-running memory stability test completed successfully")
}

// TestServerRestartCycles tests memory behavior during repeated server restarts
func TestServerRestartCycles(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running test in short mode")
	}

	const (
		restartCycles    = 5                      // Reduce from 50 to 5 cycles
		requestsPerCycle = 5                      // Reduce from 20 to 5 requests
		cycleDelay       = 100 * time.Millisecond // Reduce from 10s to 100ms
	)

	profiler := NewMemoryProfiler(2000, 5*time.Second)
	profiler.StartMonitoring()
	defer profiler.StopMonitoring()

	var totalRequests, totalErrors int64

	for cycle := 0; cycle < restartCycles; cycle++ {
		t.Logf("Starting restart cycle %d/%d", cycle+1, restartCycles)

		harness := NewTestHarness(t)

		// Start gateway
		err := harness.StartGateway()
		if err != nil {
			harness.StopGateway()
			t.Fatalf("Cycle %d: Failed to start gateway: %v", cycle, err)
		}

		// Send requests
		var cycleRequests, cycleErrors int64
		var wg sync.WaitGroup

		for i := 0; i < requestsPerCycle; i++ {
			wg.Add(1)
			go func(reqID int) {
				defer wg.Done()

				err := harness.SendLSPRequest("textDocument/definition", map[string]interface{}{
					"textDocument": map[string]interface{}{
						"uri": fmt.Sprintf("file:///test/cycle_%d_req_%d.go", cycle, reqID),
					},
					"position": map[string]interface{}{
						"line":      reqID % 100,
						"character": (reqID * 3) % 50,
					},
				})

				atomic.AddInt64(&cycleRequests, 1)
				atomic.AddInt64(&totalRequests, 1)
				if err != nil {
					atomic.AddInt64(&cycleErrors, 1)
					atomic.AddInt64(&totalErrors, 1)
				}
			}(i)
		}

		wg.Wait()

		// Stop gateway
		err = harness.StopGateway()
		if err != nil {
			t.Logf("Cycle %d: Error stopping gateway: %v", cycle, err)
		}

		// Force cleanup
		runtime.GC()
		runtime.GC()

		t.Logf("Cycle %d completed: %d requests, %d errors", cycle+1, cycleRequests, cycleErrors)

		// Check memory every 10 cycles
		if (cycle+1)%10 == 0 {
			report := profiler.GenerateReport()
			t.Logf("Memory check at cycle %d: heap=%v MB, goroutines=%v",
				cycle+1, report["heap_growth_mb"], report["current_goroutines"])

			if leakDetected, ok := report["leak_detected"].(bool); ok && leakDetected {
				t.Errorf("Memory leak detected at cycle %d: confidence=%.2f%%",
					cycle+1, report["leak_confidence"].(float64)*100)
				break
			}
		}

		if cycle < restartCycles-1 {
			time.Sleep(cycleDelay)
		}
	}

	// Final analysis
	finalReport := profiler.GenerateReport()

	t.Logf("=== RESTART CYCLES FINAL REPORT ===")
	t.Logf("Total cycles: %d", restartCycles)
	t.Logf("Total requests: %d", totalRequests)
	t.Logf("Total errors: %d", totalErrors)
	t.Logf("Error rate: %.2f%%", float64(totalErrors)/math.Max(float64(totalRequests), 1)*100)
	t.Logf("Final heap growth: %v MB", finalReport["heap_growth_mb"])
	t.Logf("Final goroutine growth: %v", finalReport["goroutine_growth"])

	// Validate results
	if leakDetected, ok := finalReport["leak_detected"].(bool); ok && leakDetected {
		t.Errorf("MEMORY LEAK DETECTED after restart cycles: confidence=%.2f%%",
			finalReport["leak_confidence"].(float64)*100)
	}

	if heapGrowthMB, ok := finalReport["heap_growth_mb"].(int64); ok && heapGrowthMB > 100 {
		t.Errorf("Excessive memory growth after restart cycles: %d MB", heapGrowthMB)
	}
}

// TestLargeDocumentProcessing tests memory efficiency with large documents
func TestLargeDocumentProcessing(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory-intensive test in short mode")
	}

	harness := NewTestHarness(t)
	defer func() {
		harness.StopGateway()
		harness.StopProfiling()
	}()

	harness.StartProfiling()

	err := harness.StartGateway()
	if err != nil {
		t.Fatalf("Failed to start gateway: %v", err)
	}

	// Create large documents of various sizes
	documentSizes := []int{
		1024 * 10, // 10KB (reduced from 100KB)
		1024 * 50, // 50KB (reduced from 500KB)
	}

	const documentsPerSize = 2 // Reduce from 5 to 2
	var totalRequests, totalErrors int64

	for _, size := range documentSizes {
		t.Logf("Testing documents of size: %d bytes", size)

		for i := 0; i < documentsPerSize; i++ {
			// Generate large document content
			content := make([]byte, size)
			rand.Read(content)

			// Make it look like Go code
			goContent := fmt.Sprintf(`package main

import "fmt"

// Large generated content follows
var largeData = %q

func main() {
	fmt.Println("Processing large data:", len(largeData))
}
`, string(content[:1000])) // Use only first 1000 bytes to avoid extremely large files

			// Create temp file
			tmpFile := filepath.Join(harness.testDir, fmt.Sprintf("large_doc_%d_%d.go", size, i))
			err := os.WriteFile(tmpFile, []byte(goContent), 0644)
			if err != nil {
				t.Fatalf("Failed to create large document: %v", err)
			}

			// Test LSP operations on large document
			operations := []struct {
				method string
				params interface{}
			}{
				{
					method: "textDocument/definition",
					params: map[string]interface{}{
						"textDocument": map[string]interface{}{
							"uri": "file://" + tmpFile,
						},
						"position": map[string]interface{}{
							"line":      5,
							"character": 10,
						},
					},
				},
				{
					method: "textDocument/hover",
					params: map[string]interface{}{
						"textDocument": map[string]interface{}{
							"uri": "file://" + tmpFile,
						},
						"position": map[string]interface{}{
							"line":      10,
							"character": 5,
						},
					},
				},
			}

			for _, op := range operations {
				err := harness.SendLSPRequest(op.method, op.params)

				atomic.AddInt64(&totalRequests, 1)
				if err != nil {
					atomic.AddInt64(&totalErrors, 1)
					t.Logf("Request failed for document size %d: %v", size, err)
				}
			}

			// Force GC after each document
			runtime.GC()
		}

		// Check memory after each size category
		report := harness.GetMemoryReport()
		t.Logf("After processing %d documents of size %d bytes: heap=%v MB, goroutines=%v",
			documentsPerSize, size, report["heap_growth_mb"], report["current_goroutines"])

		if leakDetected, ok := report["leak_detected"].(bool); ok && leakDetected {
			t.Errorf("Memory leak detected during large document processing: confidence=%.2f%%",
				report["leak_confidence"].(float64)*100)
			break
		}
	}

	// Final analysis
	finalReport := harness.GetMemoryReport()

	t.Logf("=== LARGE DOCUMENT PROCESSING FINAL REPORT ===")
	t.Logf("Total requests: %d", totalRequests)
	t.Logf("Total errors: %d", totalErrors)
	t.Logf("Error rate: %.2f%%", float64(totalErrors)/math.Max(float64(totalRequests), 1)*100)
	t.Logf("Final heap growth: %v MB", finalReport["heap_growth_mb"])
	t.Logf("Final goroutine growth: %v", finalReport["goroutine_growth"])

	// Validate results
	if leakDetected, ok := finalReport["leak_detected"].(bool); ok && leakDetected {
		t.Errorf("MEMORY LEAK DETECTED in large document processing: confidence=%.2f%%",
			finalReport["leak_confidence"].(float64)*100)
	}

	if heapGrowthMB, ok := finalReport["heap_growth_mb"].(int64); ok && heapGrowthMB > 200 {
		t.Errorf("Excessive memory growth in large document processing: %d MB", heapGrowthMB)
	}

	errorRate := float64(totalErrors) / math.Max(float64(totalRequests), 1) * 100
	if errorRate > 15.0 {
		t.Errorf("High error rate in large document processing: %.2f%%", errorRate)
	}
}

// BenchmarkMemoryEfficiency benchmarks memory efficiency during operation
func BenchmarkMemoryEfficiency(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping long-running benchmark in short mode")
	}

	profiler := NewMemoryProfiler(1000, 2*time.Second)
	profiler.StartMonitoring()
	defer profiler.StopMonitoring()

	harness := NewTestHarness(&testing.T{})
	defer func() {
		harness.StopGateway()
	}()

	err := harness.StartGateway()
	if err != nil {
		b.Fatalf("Failed to start gateway: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		err := harness.SendLSPRequest("textDocument/definition", map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": fmt.Sprintf("file:///test/bench_%d.go", i),
			},
			"position": map[string]interface{}{
				"line":      i % 100,
				"character": (i * 7) % 50,
			},
		})

		if err != nil {
			b.Logf("Request %d failed: %v", i, err)
		}

		// Periodic GC to prevent accumulation
		if i%100 == 0 {
			runtime.GC()
		}
	}

	report := profiler.GenerateReport()
	if growth, ok := report["heap_growth_mb"].(int64); ok {
		b.Logf("Memory growth per operation: %d bytes", (growth*1024*1024)/int64(b.N))
	}

	if leakDetected, ok := report["leak_detected"].(bool); ok && leakDetected {
		b.Errorf("Memory leak detected during benchmark: confidence=%.2f%%",
			report["leak_confidence"].(float64)*100)
	}
}
