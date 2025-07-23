package gateway_benchmark_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/gateway"
	"lsp-gateway/internal/transport"
	testutils "lsp-gateway/tests/utils/gateway"
)

func BenchmarkGatewayHTTPHandler(b *testing.B) {
	benchmarks := []struct {
		name     string
		method   string
		uri      string
		parallel int
	}{
		{"Definition", "textDocument/definition", "file:///test.go", 1},
		{"References", "textDocument/references", "file:///test.go", 1},
		{"Hover", "textDocument/hover", "file:///test.go", 1},
		{"DocumentSymbol", "textDocument/documentSymbol", "file:///test.go", 1},
		{"WorkspaceSymbol", "workspace/symbol", "", 1},
		{"Definition_Parallel_10", "textDocument/definition", "file:///test.go", 10},
		{"References_Parallel_10", "textDocument/references", "file:///test.go", 10},
		{"Hover_Parallel_10", "textDocument/hover", "file:///test.go", 10},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			if bm.parallel > 1 {
				benchmarkHTTPHandlerParallel(b, bm.method, bm.uri, bm.parallel)
			} else {
				benchmarkHTTPHandlerSingle(b, bm.method, bm.uri)
			}
		})
	}
}

func benchmarkHTTPHandlerSingle(b *testing.B, method, uri string) {
	gateway := setupBenchmarkGateway(b)
	defer teardownBenchmarkGateway(b, gateway)

	handler := http.HandlerFunc(gateway.HandleJSONRPC)

	reqBody := createJSONRPCRequest(method, uri)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			b.Fatalf("Expected status 200, got %d", w.Code)
		}
	}
}

func benchmarkHTTPHandlerParallel(b *testing.B, method, uri string, parallelism int) {
	gateway := setupBenchmarkGateway(b)
	defer teardownBenchmarkGateway(b, gateway)

	handler := http.HandlerFunc(gateway.HandleJSONRPC)
	reqBody := createJSONRPCRequest(method, uri)

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("POST", "/", bytes.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				b.Fatalf("Expected status 200, got %d", w.Code)
			}
		}
	})
}

func BenchmarkRouterPerformance(b *testing.B) {
	router := gateway.NewRouter()

	router.RegisterServer("go-lsp", []string{"go"})
	router.RegisterServer("python-lsp", []string{"python"})
	router.RegisterServer("typescript-lsp", []string{"typescript", "javascript"})
	router.RegisterServer("java-lsp", []string{"java"})

	testCases := []struct {
		name string
		uri  string
	}{
		{"GoFile", "file:///path/to/main.go"},
		{"PythonFile", "file:///path/to/script.py"},
		{"TypeScriptFile", "file:///path/to/app.ts"},
		{"JavaScriptFile", "file:///path/to/app.js"},
		{"JavaFile", "file:///path/to/Main.java"},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := router.RouteRequest(tc.uri)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkConcurrentClients(b *testing.B) {
	concurrencyLevels := []int{1, 5, 10, 20, 50}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Clients_%d", concurrency), func(b *testing.B) {
			benchmarkConcurrentClients(b, concurrency)
		})
	}
}

func benchmarkConcurrentClients(b *testing.B, concurrency int) {
	gateway := setupBenchmarkGateway(b)
	defer teardownBenchmarkGateway(b, gateway)

	handler := http.HandlerFunc(gateway.HandleJSONRPC)
	reqBody := createJSONRPCRequest("textDocument/definition", "file:///test.go")

	var wg sync.WaitGroup
	requests := make(chan struct{}, b.N)

	for i := 0; i < b.N; i++ {
		requests <- struct{}{}
	}
	close(requests)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range requests {
				req := httptest.NewRequest("POST", "/", bytes.NewReader(reqBody))
				req.Header.Set("Content-Type", "application/json")

				w := httptest.NewRecorder()
				handler.ServeHTTP(w, req)

				if w.Code != http.StatusOK {
					b.Errorf("Expected status 200, got %d", w.Code)
				}
			}
		}()
	}

	wg.Wait()
}

func BenchmarkLatencyProfile(b *testing.B) {
	gateway := setupBenchmarkGateway(b)
	defer teardownBenchmarkGateway(b, gateway)

	handler := http.HandlerFunc(gateway.HandleJSONRPC)
	reqBody := createJSONRPCRequest("textDocument/definition", "file:///test.go")

	latencies := make([]time.Duration, b.N)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		start := time.Now()

		req := httptest.NewRequest("POST", "/", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		latencies[i] = time.Since(start)

		if w.Code != http.StatusOK {
			b.Fatalf("Expected status 200, got %d", w.Code)
		}
	}

	if b.N > 0 {
		total := time.Duration(0)
		min := latencies[0]
		max := latencies[0]

		for _, lat := range latencies {
			total += lat
			if lat < min {
				min = lat
			}
			if lat > max {
				max = lat
			}
		}

		avg := total / time.Duration(b.N)
		b.Logf("Latency - Min: %v, Max: %v, Avg: %v", min, max, avg)

		if avg > 100*time.Millisecond {
			b.Errorf("Average latency %v exceeds target of 100ms", avg)
		}
	}
}

func BenchmarkMemoryLeakDetection(b *testing.B) {
	runtime.GC() // Clean start
	var startMem runtime.MemStats
	runtime.ReadMemStats(&startMem)

	gateway := setupBenchmarkGateway(b)
	defer teardownBenchmarkGateway(b, gateway)

	handler := http.HandlerFunc(gateway.HandleJSONRPC)
	reqBody := createJSONRPCRequest("textDocument/definition", "file:///test.go")

	const samplingInterval = 1000
	memSamples := make([]uint64, 0, b.N/samplingInterval+1)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			b.Fatalf("Expected status 200, got %d", w.Code)
		}

		if i%samplingInterval == 0 {
			runtime.GC()
			var mem runtime.MemStats
			runtime.ReadMemStats(&mem)
			memSamples = append(memSamples, mem.Alloc)
		}
	}

	if len(memSamples) > 1 {
		first := memSamples[0]
		last := memSamples[len(memSamples)-1]
		growth := int64(last) - int64(first)

		b.Logf("Memory - Start: %d bytes, End: %d bytes, Growth: %d bytes", first, last, growth)

		if growth > int64(first)/2 {
			b.Errorf("Potential memory leak detected: growth of %d bytes (%.1f%%)",
				growth, float64(growth)/float64(first)*100)
		}
	}
}

func BenchmarkThroughput(b *testing.B) {
	gateway := setupBenchmarkGateway(b)
	defer teardownBenchmarkGateway(b, gateway)

	handler := http.HandlerFunc(gateway.HandleJSONRPC)
	reqBody := createJSONRPCRequest("textDocument/definition", "file:///test.go")

	const duration = 10 * time.Second
	var requestCount int64
	done := make(chan struct{})

	const numWorkers = 20
	for i := 0; i < numWorkers; i++ {
		go func() {
			for {
				select {
				case <-done:
					return
				default:
					req := httptest.NewRequest("POST", "/", bytes.NewReader(reqBody))
					req.Header.Set("Content-Type", "application/json")

					w := httptest.NewRecorder()
					handler.ServeHTTP(w, req)

					if w.Code == http.StatusOK {
						atomic.AddInt64(&requestCount, 1)
					}
				}
			}
		}()
	}

	b.ResetTimer()

	time.Sleep(duration)
	close(done)

	finalCount := atomic.LoadInt64(&requestCount)
	throughput := float64(finalCount) / duration.Seconds()

	b.Logf("Throughput: %.1f requests/second", throughput)

	if throughput < 100 {
		b.Errorf("Throughput %.1f req/sec is below target of 100 req/sec", throughput)
	}

}

func BenchmarkErrorHandling(b *testing.B) {
	gateway := setupBenchmarkGateway(b)
	defer teardownBenchmarkGateway(b, gateway)

	handler := http.HandlerFunc(gateway.HandleJSONRPC)

	errorCases := []struct {
		name string
		body string
	}{
		{"InvalidJSON", `{"invalid": json}`},
		{"MissingMethod", `{"jsonrpc": "2.0", "id": 1}`},
		{"UnsupportedMethod", `{"jsonrpc": "2.0", "method": "unsupported", "id": 1}`},
		{"InvalidURI", `{"jsonrpc": "2.0", "method": "textDocument/definition", "params": {"textDocument": {"uri": "invalid"}}, "id": 1}`},
	}

	for _, ec := range errorCases {
		b.Run(ec.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				req := httptest.NewRequest("POST", "/", bytes.NewReader([]byte(ec.body)))
				req.Header.Set("Content-Type", "application/json")

				w := httptest.NewRecorder()
				handler.ServeHTTP(w, req)

				if w.Code != http.StatusOK {
					b.Fatalf("Expected status 200 even for errors, got %d", w.Code)
				}
			}
		})
	}
}

func setupBenchmarkGateway(b *testing.B) *gateway.Gateway {
	config := &config.GatewayConfig{
		Port: 8080,
		Servers: []config.ServerConfig{
			{
				Name:      "go-lsp",
				Languages: []string{"go"},
				Command:   "gopls",
				Args:      []string{},
				Transport: "stdio",
			},
			{
				Name:      "python-lsp",
				Languages: []string{"python"},
				Command:   "python",
				Args:      []string{"-m", "pylsp"},
				Transport: "stdio",
			},
		},
	}

	mockClientFactory := func(cfg transport.ClientConfig) (transport.LSPClient, error) {
		mock := NewBenchmarkMockClient()
		return mock, nil
	}

	testableGateway, err := testutils.NewTestableGateway(config, mockClientFactory)
	if err != nil {
		b.Fatalf("Failed to create gateway: %v", err)
	}

	ctx := context.Background()
	if err := testableGateway.Start(ctx); err != nil {
		b.Fatalf("Failed to start gateway: %v", err)
	}

	return testableGateway.Gateway
}

func teardownBenchmarkGateway(b *testing.B, gw *gateway.Gateway) {
	if err := gw.Stop(); err != nil {
		b.Logf("Error stopping gateway: %v", err)
	}
}

func createJSONRPCRequest(method, uri string) []byte {
	var params interface{}

	if uri != "" {
		params = map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": uri,
			},
			"position": map[string]interface{}{
				"line":      10,
				"character": 5,
			},
		}
	}

	req := gateway.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  method,
		Params:  params,
	}

	data, _ := json.Marshal(req)
	return data
}

type BenchmarkMockClient struct {
	active bool
	mu     sync.RWMutex
}

func NewBenchmarkMockClient() *BenchmarkMockClient {
	return &BenchmarkMockClient{active: false}
}

func (m *BenchmarkMockClient) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.active = true
	return nil
}

func (m *BenchmarkMockClient) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.active = false
	return nil
}

func (m *BenchmarkMockClient) SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	return json.RawMessage(`{"result":null}`), nil
}

func (m *BenchmarkMockClient) SendNotification(ctx context.Context, method string, params interface{}) error {
	return nil
}

func (m *BenchmarkMockClient) IsActive() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.active
}
