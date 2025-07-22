package gateway

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/transport"
)

// MemoryStats tracks detailed memory usage metrics
type MemoryStats struct {
	HeapAlloc     uint64
	HeapSys       uint64
	HeapIdle      uint64
	HeapInuse     uint64
	HeapReleased  uint64
	HeapObjects   uint64
	StackInuse    uint64
	StackSys      uint64
	MSpanInuse    uint64
	MSpanSys      uint64
	MCacheInuse   uint64
	MCacheSys     uint64
	BuckHashSys   uint64
	GCSys         uint64
	OtherSys      uint64
	NextGC        uint64
	LastGC        uint64
	PauseTotalNs  uint64
	PauseNs       [256]uint64
	PauseEnd      [256]uint64
	NumGC         uint32
	NumForcedGC   uint32
	GCCPUFraction float64
	TotalAlloc    uint64
	Mallocs       uint64
	Frees         uint64
	Lookups       uint64
	Timestamp     time.Time
}

// MemoryMonitor provides comprehensive memory monitoring capabilities
type MemoryMonitor struct {
	mu           sync.RWMutex
	samples      []MemoryStats
	maxSamples   int
	sampleRate   time.Duration
	stopCh       chan struct{}
	monitoringWG sync.WaitGroup
}

// NewMemoryMonitor creates a new memory monitor
func NewMemoryMonitor(maxSamples int, sampleRate time.Duration) *MemoryMonitor {
	return &MemoryMonitor{
		samples:    make([]MemoryStats, 0, maxSamples),
		maxSamples: maxSamples,
		sampleRate: sampleRate,
		stopCh:     make(chan struct{}),
	}
}

// Start begins memory monitoring
func (m *MemoryMonitor) Start() {
	m.monitoringWG.Add(1)
	go func() {
		defer m.monitoringWG.Done()
		ticker := time.NewTicker(m.sampleRate)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				m.takeSample()
			case <-m.stopCh:
				return
			}
		}
	}()
}

// Stop stops memory monitoring
func (m *MemoryMonitor) Stop() {
	close(m.stopCh)
	m.monitoringWG.Wait()
}

// takeSample captures current memory statistics
func (m *MemoryMonitor) takeSample() {
	runtime.GC() // Force GC for accurate measurement

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	sample := MemoryStats{
		HeapAlloc:     memStats.HeapAlloc,
		HeapSys:       memStats.HeapSys,
		HeapIdle:      memStats.HeapIdle,
		HeapInuse:     memStats.HeapInuse,
		HeapReleased:  memStats.HeapReleased,
		HeapObjects:   memStats.HeapObjects,
		StackInuse:    memStats.StackInuse,
		StackSys:      memStats.StackSys,
		MSpanInuse:    memStats.MSpanInuse,
		MSpanSys:      memStats.MSpanSys,
		MCacheInuse:   memStats.MCacheInuse,
		MCacheSys:     memStats.MCacheSys,
		BuckHashSys:   memStats.BuckHashSys,
		GCSys:         memStats.GCSys,
		OtherSys:      memStats.OtherSys,
		NextGC:        memStats.NextGC,
		LastGC:        memStats.LastGC,
		PauseTotalNs:  memStats.PauseTotalNs,
		PauseNs:       memStats.PauseNs,
		PauseEnd:      memStats.PauseEnd,
		NumGC:         memStats.NumGC,
		NumForcedGC:   memStats.NumForcedGC,
		GCCPUFraction: memStats.GCCPUFraction,
		TotalAlloc:    memStats.TotalAlloc,
		Mallocs:       memStats.Mallocs,
		Frees:         memStats.Frees,
		Lookups:       memStats.Lookups,
		Timestamp:     time.Now(),
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.samples) >= m.maxSamples {
		// Remove oldest sample
		copy(m.samples, m.samples[1:])
		m.samples = m.samples[:len(m.samples)-1]
	}
	m.samples = append(m.samples, sample)
}

// GetCurrentMemory returns current memory statistics
func (m *MemoryMonitor) GetCurrentMemory() MemoryStats {
	m.takeSample()
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.samples) == 0 {
		return MemoryStats{}
	}
	return m.samples[len(m.samples)-1]
}

// GetMemoryGrowth calculates memory growth between first and last samples
func (m *MemoryMonitor) GetMemoryGrowth() (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.samples) < 2 {
		return 0, fmt.Errorf("insufficient samples for growth calculation")
	}

	first := m.samples[0]
	last := m.samples[len(m.samples)-1]
	return int64(last.HeapAlloc) - int64(first.HeapAlloc), nil
}

// DetectMemoryLeak analyzes samples for potential memory leaks
func (m *MemoryMonitor) DetectMemoryLeak(threshold float64) (bool, string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.samples) < 10 {
		return false, "insufficient samples for leak detection"
	}

	// Calculate trend in heap allocation
	start := m.samples[0].HeapAlloc
	end := m.samples[len(m.samples)-1].HeapAlloc
	growth := float64(end) - float64(start)
	growthPercent := (growth / float64(start)) * 100

	if growthPercent > threshold {
		return true, fmt.Sprintf("potential memory leak detected: %.2f%% growth", growthPercent)
	}

	return false, fmt.Sprintf("no memory leak detected: %.2f%% growth", growthPercent)
}

// GetStats returns summary statistics
func (m *MemoryMonitor) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.samples) == 0 {
		return map[string]interface{}{"error": "no samples available"}
	}

	first := m.samples[0]
	last := m.samples[len(m.samples)-1]

	return map[string]interface{}{
		"sample_count":      len(m.samples),
		"initial_heap":      first.HeapAlloc,
		"final_heap":        last.HeapAlloc,
		"heap_growth":       int64(last.HeapAlloc) - int64(first.HeapAlloc),
		"total_allocations": last.TotalAlloc - first.TotalAlloc,
		"gc_runs":           last.NumGC - first.NumGC,
		"forced_gc_runs":    last.NumForcedGC - first.NumForcedGC,
		"gc_cpu_fraction":   last.GCCPUFraction,
		"objects_count":     last.HeapObjects,
		"duration":          last.Timestamp.Sub(first.Timestamp),
	}
}

// LargePayloadMockClient simulates memory-intensive processing
type LargePayloadMockClient struct {
	active     bool
	mu         sync.RWMutex
	buffers    [][]byte
	processing bool
	delay      time.Duration
}

// NewLargePayloadMockClient creates a mock client for large payload testing
func NewLargePayloadMockClient(processingDelay time.Duration) *LargePayloadMockClient {
	return &LargePayloadMockClient{
		active:  false,
		buffers: make([][]byte, 0),
		delay:   processingDelay,
	}
}

func (m *LargePayloadMockClient) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.active = true
	return nil
}

func (m *LargePayloadMockClient) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.active = false
	// Clear buffers to simulate cleanup
	m.buffers = nil
	return nil
}

func (m *LargePayloadMockClient) SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	m.mu.Lock()
	m.processing = true
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.processing = false
		m.mu.Unlock()
	}()

	// Simulate memory-intensive processing
	if strings.Contains(method, "large") {
		// Allocate large buffer to simulate processing
		buffer := make([]byte, 10*1024*1024) // 10MB buffer
		for i := range buffer {
			buffer[i] = byte(i % 256)
		}

		m.mu.Lock()
		m.buffers = append(m.buffers, buffer)
		m.mu.Unlock()

		// Simulate processing delay
		if m.delay > 0 {
			time.Sleep(m.delay)
		}
	}

	return json.RawMessage(`{"result": "mock_result"}`), nil
}

func (m *LargePayloadMockClient) SendNotification(ctx context.Context, method string, params interface{}) error {
	return nil
}

func (m *LargePayloadMockClient) IsActive() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.active
}

func (m *LargePayloadMockClient) IsProcessing() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.processing
}

func (m *LargePayloadMockClient) GetBufferCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.buffers)
}

// generateLargePayload creates a large JSON payload for testing
func generateLargePayload(sizeBytes int) []byte {
	// Create large payload with repeated data
	data := make([]byte, sizeBytes/2)
	rand.Read(data)

	// Create JSON structure
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "textDocument/large_definition",
		"params": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": "file:///large_test.go",
			},
			"position": map[string]interface{}{
				"line":      10,
				"character": 5,
			},
			"largeData": fmt.Sprintf("%x", data),
		},
	}

	result, _ := json.Marshal(payload)
	return result
}

// Test extremely large request processing
func TestLargeRequestProcessing(t *testing.T) {
	testCases := []struct {
		name        string
		payloadSize int
		concurrent  int
		timeout     time.Duration
	}{
		{"10MB_Single", 10 * 1024 * 1024, 1, 30 * time.Second},
		{"50MB_Single", 50 * 1024 * 1024, 1, 60 * time.Second},
		{"100MB_Single", 100 * 1024 * 1024, 1, 120 * time.Second},
		{"10MB_Concurrent_5", 10 * 1024 * 1024, 5, 45 * time.Second},
		{"50MB_Concurrent_3", 50 * 1024 * 1024, 3, 90 * time.Second},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			monitor := NewMemoryMonitor(1000, 100*time.Millisecond)
			monitor.Start()
			defer monitor.Stop()

			initialMem := monitor.GetCurrentMemory()
			t.Logf("Initial memory: %d bytes", initialMem.HeapAlloc)

			gateway := setupMemoryTestGateway(t, 100*time.Millisecond)
			defer teardownMemoryTestGateway(t, gateway)

			handler := http.HandlerFunc(gateway.HandleJSONRPC)
			payload := generateLargePayload(tc.payloadSize)

			var wg sync.WaitGroup
			var successCount int64
			var errorCount int64

			startTime := time.Now()

			for i := 0; i < tc.concurrent; i++ {
				wg.Add(1)
				go func(workerID int) {
					defer wg.Done()

					ctx, cancel := context.WithTimeout(context.Background(), tc.timeout)
					defer cancel()

					req := httptest.NewRequest("POST", "/", bytes.NewReader(payload))
					req.Header.Set("Content-Type", "application/json")
					req = req.WithContext(ctx)

					w := httptest.NewRecorder()
					handler.ServeHTTP(w, req)

					if w.Code == http.StatusOK {
						atomic.AddInt64(&successCount, 1)
					} else {
						atomic.AddInt64(&errorCount, 1)
						t.Logf("Worker %d failed with status %d", workerID, w.Code)
					}
				}(i)
			}

			wg.Wait()
			duration := time.Since(startTime)

			finalMem := monitor.GetCurrentMemory()
			growth, _ := monitor.GetMemoryGrowth()

			t.Logf("Test completed in %v", duration)
			t.Logf("Successful requests: %d, Failed requests: %d", successCount, errorCount)
			t.Logf("Memory growth: %d bytes (%.2f MB)", growth, float64(growth)/(1024*1024))
			t.Logf("Final memory: %d bytes", finalMem.HeapAlloc)

			// Verify we handled large payloads without excessive memory growth
			maxExpectedGrowth := int64(tc.payloadSize * tc.concurrent * 2) // Allow 2x overhead
			if growth > maxExpectedGrowth {
				t.Errorf("Excessive memory growth: %d bytes > %d bytes", growth, maxExpectedGrowth)
			}

			// Verify successful processing
			if successCount == 0 {
				t.Error("No requests were processed successfully")
			}

			// Allow for some errors under extreme load
			if errorCount > int64(tc.concurrent)/2 {
				t.Errorf("Too many errors: %d out of %d", errorCount, tc.concurrent)
			}
		})
	}
}

// Test memory cleanup after large request completion
func TestMemoryCleanupAfterLargeRequests(t *testing.T) {
	monitor := NewMemoryMonitor(500, 50*time.Millisecond)
	monitor.Start()
	defer monitor.Stop()

	gateway := setupMemoryTestGateway(t, 50*time.Millisecond)
	defer teardownMemoryTestGateway(t, gateway)

	handler := http.HandlerFunc(gateway.HandleJSONRPC)

	// Baseline memory
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	baselineMem := monitor.GetCurrentMemory()

	// Process large requests
	const requestCount = 10
	const payloadSize = 20 * 1024 * 1024 // 20MB

	for i := 0; i < requestCount; i++ {
		payload := generateLargePayload(payloadSize)
		req := httptest.NewRequest("POST", "/", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d failed with status %d", i, w.Code)
		}
	}

	// Force cleanup and measure
	runtime.GC()
	runtime.GC() // Double GC to ensure cleanup
	time.Sleep(500 * time.Millisecond)

	finalMem := monitor.GetCurrentMemory()
	memoryRetained := int64(finalMem.HeapAlloc) - int64(baselineMem.HeapAlloc)

	t.Logf("Baseline memory: %d bytes", baselineMem.HeapAlloc)
	t.Logf("Final memory: %d bytes", finalMem.HeapAlloc)
	t.Logf("Memory retained: %d bytes (%.2f MB)", memoryRetained, float64(memoryRetained)/(1024*1024))

	// Verify memory cleanup - should not retain more than 50% of processed data
	maxRetained := int64(payloadSize * requestCount / 2)
	if memoryRetained > maxRetained {
		t.Errorf("Excessive memory retention: %d bytes > %d bytes", memoryRetained, maxRetained)
	}

	// Check for memory leaks
	isLeak, leakMsg := monitor.DetectMemoryLeak(25.0) // 25% threshold
	if isLeak {
		t.Errorf("Memory leak detected: %s", leakMsg)
	} else {
		t.Logf("Memory leak check: %s", leakMsg)
	}
}

// Test concurrent large request handling without memory leaks
func TestConcurrentLargeRequestsMemoryLeaks(t *testing.T) {
	monitor := NewMemoryMonitor(1000, 100*time.Millisecond)
	monitor.Start()
	defer monitor.Stop()

	gateway := setupMemoryTestGateway(t, 25*time.Millisecond)
	defer teardownMemoryTestGateway(t, gateway)

	handler := http.HandlerFunc(gateway.HandleJSONRPC)

	const (
		workerCount       = 20
		requestsPerWorker = 5
		payloadSize       = 10 * 1024 * 1024 // 10MB
		testDuration      = 30 * time.Second
	)

	var wg sync.WaitGroup
	var totalRequests int64
	var totalErrors int64

	ctx, cancel := context.WithTimeout(context.Background(), testDuration)
	defer cancel()

	startTime := time.Now()

	for worker := 0; worker < workerCount; worker++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for req := 0; req < requestsPerWorker; req++ {
				select {
				case <-ctx.Done():
					return
				default:
				}

				payload := generateLargePayload(payloadSize)
				httpReq := httptest.NewRequest("POST", "/", bytes.NewReader(payload))
				httpReq.Header.Set("Content-Type", "application/json")
				httpReq = httpReq.WithContext(ctx)

				w := httptest.NewRecorder()
				handler.ServeHTTP(w, httpReq)

				atomic.AddInt64(&totalRequests, 1)
				if w.Code != http.StatusOK {
					atomic.AddInt64(&totalErrors, 1)
				}

				// Small delay between requests
				time.Sleep(100 * time.Millisecond)
			}
		}(worker)
	}

	wg.Wait()
	duration := time.Since(startTime)

	// Force cleanup
	runtime.GC()
	runtime.GC()
	time.Sleep(500 * time.Millisecond)

	stats := monitor.GetStats()

	t.Logf("Test completed in %v", duration)
	t.Logf("Total requests: %d, Errors: %d", totalRequests, totalErrors)
	t.Logf("Memory stats: %+v", stats)

	// Verify reasonable error rate
	errorRate := float64(totalErrors) / float64(totalRequests) * 100
	if errorRate > 10.0 {
		t.Errorf("High error rate: %.2f%%", errorRate)
	}

	// Check for memory leaks
	if growth, ok := stats["heap_growth"].(int64); ok {
		maxGrowth := int64(payloadSize * workerCount * 2) // Allow 2x overhead
		if growth > maxGrowth {
			t.Errorf("Excessive memory growth: %d bytes > %d bytes", growth, maxGrowth)
		}
	}
}

// Test memory pressure scenarios with bounded request queues
func TestMemoryPressureWithBoundedQueues(t *testing.T) {
	monitor := NewMemoryMonitor(1000, 100*time.Millisecond)
	monitor.Start()
	defer monitor.Stop()

	gateway := setupMemoryTestGateway(t, 10*time.Millisecond)
	defer teardownMemoryTestGateway(t, gateway)

	handler := http.HandlerFunc(gateway.HandleJSONRPC)

	// Set memory limit for test
	debug.SetMemoryLimit(500 * 1024 * 1024) // 500MB limit
	defer debug.SetMemoryLimit(math.MaxInt64)

	const (
		maxConcurrent = 50
		payloadSize   = 5 * 1024 * 1024 // 5MB
		totalRequests = 200
	)

	semaphore := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	var processedCount int64
	var rejectedCount int64

	for i := 0; i < totalRequests; i++ {
		wg.Add(1)
		go func(reqID int) {
			defer wg.Done()

			// Try to acquire semaphore
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			default:
				atomic.AddInt64(&rejectedCount, 1)
				return
			}

			payload := generateLargePayload(payloadSize)
			req := httptest.NewRequest("POST", "/", bytes.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			atomic.AddInt64(&processedCount, 1)
		}(i)
	}

	wg.Wait()

	stats := monitor.GetStats()

	t.Logf("Processed: %d, Rejected: %d, Total: %d", processedCount, rejectedCount, totalRequests)
	t.Logf("Memory stats: %+v", stats)

	// Verify bounded queue behavior
	if processedCount+rejectedCount != totalRequests {
		t.Errorf("Request count mismatch: %d + %d != %d", processedCount, rejectedCount, totalRequests)
	}

	// Verify memory stayed within reasonable bounds
	if growth, ok := stats["heap_growth"].(int64); ok {
		maxGrowth := int64(payloadSize * maxConcurrent * 3) // Allow 3x overhead for bounded execution
		if growth > maxGrowth {
			t.Errorf("Memory growth exceeded bounds: %d bytes > %d bytes", growth, maxGrowth)
		}
	}
}

// Test JSON parsing memory usage for extremely large requests
func TestJSONParsingMemoryUsage(t *testing.T) {
	monitor := NewMemoryMonitor(500, 100*time.Millisecond)
	monitor.Start()
	defer monitor.Stop()

	testCases := []struct {
		name        string
		payloadSize int
		expectError bool
	}{
		{"ValidJSON_1MB", 1 * 1024 * 1024, false},
		{"ValidJSON_10MB", 10 * 1024 * 1024, false},
		{"ValidJSON_50MB", 50 * 1024 * 1024, false},
		{"ValidJSON_100MB", 100 * 1024 * 1024, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Clear memory before test
			runtime.GC()
			time.Sleep(100 * time.Millisecond)

			preTestMem := monitor.GetCurrentMemory()

			gateway := setupMemoryTestGateway(t, 10*time.Millisecond)
			defer teardownMemoryTestGateway(t, gateway)

			payload := generateLargePayload(tc.payloadSize)
			req := httptest.NewRequest("POST", "/", bytes.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			handler := http.HandlerFunc(gateway.HandleJSONRPC)

			startTime := time.Now()
			handler.ServeHTTP(w, req)
			duration := time.Since(startTime)

			postTestMem := monitor.GetCurrentMemory()
			memoryUsed := int64(postTestMem.HeapAlloc) - int64(preTestMem.HeapAlloc)

			t.Logf("Payload size: %d bytes, Processing time: %v", len(payload), duration)
			t.Logf("Memory used: %d bytes (%.2f MB)", memoryUsed, float64(memoryUsed)/(1024*1024))
			t.Logf("Memory efficiency: %.2f%% of payload size", float64(memoryUsed)/float64(len(payload))*100)

			if tc.expectError {
				if w.Code == http.StatusOK {
					t.Error("Expected error but request succeeded")
				}
			} else {
				if w.Code != http.StatusOK {
					t.Errorf("Request failed with status %d", w.Code)
				}

				// Verify memory usage is reasonable (should be less than 5x payload size)
				maxMemory := int64(len(payload) * 5)
				if memoryUsed > maxMemory {
					t.Errorf("Excessive memory usage: %d bytes > %d bytes", memoryUsed, maxMemory)
				}
			}

			// Force cleanup
			runtime.GC()
			time.Sleep(200 * time.Millisecond)
		})
	}
}

// setupMemoryTestGateway creates a gateway with memory-optimized mock clients
func setupMemoryTestGateway(t *testing.T, processingDelay time.Duration) *Gateway {
	config := &config.GatewayConfig{
		Port: 8080,
		Servers: []config.ServerConfig{
			{
				Name:      "memory-test-lsp",
				Languages: []string{"go"},
				Command:   "mock-lsp-memory",
				Args:      []string{},
				Transport: "stdio",
			},
		},
	}

	mockClientFactory := func(cfg transport.ClientConfig) (transport.LSPClient, error) {
		return NewLargePayloadMockClient(processingDelay), nil
	}

	testableGateway, err := NewTestableGateway(config, mockClientFactory)
	if err != nil {
		t.Fatalf("Failed to create memory test gateway: %v", err)
	}

	ctx := context.Background()
	if err := testableGateway.Start(ctx); err != nil {
		t.Fatalf("Failed to start memory test gateway: %v", err)
	}

	return testableGateway.Gateway
}

// teardownMemoryTestGateway cleans up test gateway
func teardownMemoryTestGateway(t *testing.T, gateway *Gateway) {
	if err := gateway.Stop(); err != nil {
		t.Logf("Error stopping memory test gateway: %v", err)
	}

	// Force cleanup
	runtime.GC()
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
}

// Benchmark large request processing performance
func BenchmarkLargeRequestProcessing(b *testing.B) {
	payloadSizes := []int{
		1 * 1024 * 1024,   // 1MB
		10 * 1024 * 1024,  // 10MB
		50 * 1024 * 1024,  // 50MB
		100 * 1024 * 1024, // 100MB
	}

	for _, size := range payloadSizes {
		b.Run(fmt.Sprintf("PayloadSize_%dMB", size/(1024*1024)), func(b *testing.B) {
			monitor := NewMemoryMonitor(b.N, 100*time.Millisecond)
			monitor.Start()
			defer monitor.Stop()

			gateway := setupMemoryTestGateway(&testing.T{}, 1*time.Millisecond)
			defer teardownMemoryTestGateway(&testing.T{}, gateway)

			handler := http.HandlerFunc(gateway.HandleJSONRPC)
			payload := generateLargePayload(size)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				req := httptest.NewRequest("POST", "/", bytes.NewReader(payload))
				req.Header.Set("Content-Type", "application/json")

				w := httptest.NewRecorder()
				handler.ServeHTTP(w, req)

				if w.Code != http.StatusOK {
					b.Errorf("Request failed with status %d", w.Code)
				}
			}

			stats := monitor.GetStats()
			if growth, ok := stats["heap_growth"].(int64); ok {
				b.Logf("Memory growth: %d bytes per operation", growth/int64(b.N))
			}
		})
	}
}

// Benchmark memory efficiency under concurrent load
func BenchmarkConcurrentMemoryEfficiency(b *testing.B) {
	concurrencyLevels := []int{1, 5, 10, 20, 50}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrency_%d", concurrency), func(b *testing.B) {
			monitor := NewMemoryMonitor(1000, 50*time.Millisecond)
			monitor.Start()
			defer monitor.Stop()

			gateway := setupMemoryTestGateway(&testing.T{}, 1*time.Millisecond)
			defer teardownMemoryTestGateway(&testing.T{}, gateway)

			handler := http.HandlerFunc(gateway.HandleJSONRPC)
			payload := generateLargePayload(5 * 1024 * 1024) // 5MB

			b.ResetTimer()
			b.ReportAllocs()

			requests := make(chan struct{}, b.N)
			for i := 0; i < b.N; i++ {
				requests <- struct{}{}
			}
			close(requests)

			var wg sync.WaitGroup
			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for range requests {
						req := httptest.NewRequest("POST", "/", bytes.NewReader(payload))
						req.Header.Set("Content-Type", "application/json")

						w := httptest.NewRecorder()
						handler.ServeHTTP(w, req)

						if w.Code != http.StatusOK {
							b.Errorf("Request failed with status %d", w.Code)
						}
					}
				}()
			}

			wg.Wait()

			stats := monitor.GetStats()
			if growth, ok := stats["heap_growth"].(int64); ok {
				b.Logf("Average memory per request: %d bytes", growth/int64(b.N))
			}
		})
	}
}
