package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func BenchmarkLSPClientLatency(b *testing.B) {
	latencyTests := []struct {
		name     string
		method   string
		parallel bool
	}{
		{"Definition_Single", "textDocument/definition", false},
		{"References_Single", "textDocument/references", false},
		{"Hover_Single", "textDocument/hover", false},
		{"DocumentSymbol_Single", "textDocument/documentSymbol", false},
		{"WorkspaceSymbol_Single", "workspace/symbol", false},
		{"Definition_Parallel", "textDocument/definition", true},
		{"References_Parallel", "textDocument/references", true},
		{"Hover_Parallel", "textDocument/hover", true},
	}

	for _, lt := range latencyTests {
		b.Run(lt.name, func(b *testing.B) {
			if lt.parallel {
				benchmarkLSPClientLatencyParallel(b, lt.method)
			} else {
				benchmarkLSPClientLatencySingle(b, lt.method)
			}
		})
	}
}

func benchmarkLSPClientLatencySingle(b *testing.B, method string) {
	client := setupBenchmarkLSPClient(b)
	defer teardownBenchmarkLSPClient(b, client)

	params := createLSPParams(method)
	ctx := context.Background()
	latencies := make([]time.Duration, b.N)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		start := time.Now()
		_, err := client.SendRequest(ctx, method, params)
		latencies[i] = time.Since(start)

		if err != nil {
			b.Fatalf("LSP request failed: %v", err)
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

func benchmarkLSPClientLatencyParallel(b *testing.B, method string) {
	client := setupBenchmarkLSPClient(b)
	defer teardownBenchmarkLSPClient(b, client)

	params := createLSPParams(method)
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := client.SendRequest(ctx, method, params)
			if err != nil {
				b.Errorf("LSP request failed: %v", err)
			}
		}
	})
}

func BenchmarkLSPClientThroughput(b *testing.B) {
	throughputTests := []struct {
		name      string
		method    string
		workers   int
		targetTPS float64
	}{
		{"Definition_Single", "textDocument/definition", 1, 50.0},
		{"Definition_Multi_5", "textDocument/definition", 5, 200.0},
		{"Definition_Multi_10", "textDocument/definition", 10, 300.0},
		{"References_Single", "textDocument/references", 1, 50.0},
		{"References_Multi_5", "textDocument/references", 5, 200.0},
		{"Hover_HighThroughput", "textDocument/hover", 20, 500.0},
	}

	for _, tt := range throughputTests {
		b.Run(tt.name, func(b *testing.B) {
			benchmarkLSPThroughput(b, tt.method, tt.workers, tt.targetTPS)
		})
	}
}

func benchmarkLSPThroughput(b *testing.B, method string, workers int, targetTPS float64) {
	client := setupBenchmarkLSPClient(b)
	defer teardownBenchmarkLSPClient(b, client)

	params := createLSPParams(method)
	ctx := context.Background()

	const testDuration = 10 * time.Second
	var requestCount int64
	var errorCount int64
	done := make(chan struct{})

	b.ResetTimer()
	startTime := time.Now()

	for i := 0; i < workers; i++ {
		go func() {
			for {
				select {
				case <-done:
					return
				default:
					_, err := client.SendRequest(ctx, method, params)
					if err != nil {
						atomic.AddInt64(&errorCount, 1)
					} else {
						atomic.AddInt64(&requestCount, 1)
					}
				}
			}
		}()
	}

	time.Sleep(testDuration)
	close(done)

	elapsed := time.Since(startTime)
	finalRequestCount := atomic.LoadInt64(&requestCount)
	finalErrorCount := atomic.LoadInt64(&errorCount)

	actualTPS := float64(finalRequestCount) / elapsed.Seconds()

	b.Logf("Throughput: %.1f requests/second (target: %.1f)", actualTPS, targetTPS)
	b.Logf("Errors: %d/%d (%.2f%%)", finalErrorCount, finalRequestCount+finalErrorCount,
		float64(finalErrorCount)/float64(finalRequestCount+finalErrorCount)*100)

	if actualTPS < targetTPS {
		b.Errorf("Throughput %.1f req/sec is below target of %.1f req/sec", actualTPS, targetTPS)
	}

	// Note: b.N is controlled by the testing framework
	_ = finalRequestCount
}

func BenchmarkLSPClientConcurrency(b *testing.B) {
	concurrencyLevels := []int{1, 5, 10, 20, 50}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrency_%d", concurrency), func(b *testing.B) {
			benchmarkLSPConcurrency(b, concurrency)
		})
	}
}

func benchmarkLSPConcurrency(b *testing.B, concurrency int) {
	client := setupBenchmarkLSPClient(b)
	defer teardownBenchmarkLSPClient(b, client)

	params := createLSPParams("textDocument/definition")
	ctx := context.Background()

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
				_, err := client.SendRequest(ctx, "textDocument/definition", params)
				if err != nil {
					b.Errorf("LSP request failed: %v", err)
				}
			}
		}()
	}

	wg.Wait()
}

func BenchmarkLSPClientNotifications(b *testing.B) {
	client := setupBenchmarkLSPClient(b)
	defer teardownBenchmarkLSPClient(b, client)

	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        "file:///test.go",
			"languageId": "go",
			"version":    1,
			"text":       "package main\n\nfunc main() {\n\tprintln(\"Hello, World!\")\n}",
		},
	}

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		err := client.SendNotification(ctx, "textDocument/didOpen", params)
		if err != nil {
			b.Fatalf("LSP notification failed: %v", err)
		}
	}
}

func BenchmarkLSPClientMemoryUsage(b *testing.B) {
	runtime.GC()
	var startMem runtime.MemStats
	runtime.ReadMemStats(&startMem)

	client := setupBenchmarkLSPClient(b)
	defer teardownBenchmarkLSPClient(b, client)

	params := createLSPParams("textDocument/definition")
	ctx := context.Background()

	const samplingInterval = 1000
	memSamples := make([]uint64, 0, b.N/samplingInterval+1)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := client.SendRequest(ctx, "textDocument/definition", params)
		if err != nil {
			b.Fatalf("LSP request failed: %v", err)
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

func BenchmarkLSPMessageSerialization(b *testing.B) {
	serializationTests := []struct {
		name    string
		message JSONRPCMessage
	}{
		{
			"SimpleRequest",
			JSONRPCMessage{
				JSONRPC: "2.0",
				ID:      1,
				Method:  "textDocument/definition",
				Params: map[string]interface{}{
					"textDocument": map[string]interface{}{
						"uri": "file:///test.go",
					},
					"position": map[string]interface{}{
						"line":      10,
						"character": 5,
					},
				},
			},
		},
		{
			"LargeResponse",
			JSONRPCMessage{
				JSONRPC: "2.0",
				ID:      1,
				Result:  createLargeResponse(),
			},
		},
		{
			"ErrorResponse",
			JSONRPCMessage{
				JSONRPC: "2.0",
				ID:      1,
				Error: &RPCError{
					Code:    -32603,
					Message: "Internal error",
					Data:    "Detailed error information",
				},
			},
		},
	}

	for _, st := range serializationTests {
		b.Run("Serialize_"+st.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := json.Marshal(st.message)
				if err != nil {
					b.Fatalf("Serialization failed: %v", err)
				}
			}
		})

		b.Run("Deserialize_"+st.name, func(b *testing.B) {
			data, _ := json.Marshal(st.message)
			b.ResetTimer()
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				var msg JSONRPCMessage
				err := json.Unmarshal(data, &msg)
				if err != nil {
					b.Fatalf("Deserialization failed: %v", err)
				}
			}
		})
	}
}

func BenchmarkLSPClientErrorHandling(b *testing.B) {
	client := setupBenchmarkLSPClient(b)
	defer teardownBenchmarkLSPClient(b, client)

	mockClient := &ErrorMockLSPClient{}

	ctx := context.Background()
	params := createLSPParams("textDocument/definition")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := mockClient.SendRequest(ctx, "textDocument/definition", params)
		_ = err
	}
}

func BenchmarkLSPClientStartupShutdown(b *testing.B) {
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		client := NewBenchmarkMockLSPClient()

		err := client.Start(ctx)
		if err != nil {
			b.Fatalf("Client start failed: %v", err)
		}

		err = client.Stop()
		if err != nil {
			b.Fatalf("Client stop failed: %v", err)
		}
	}
}

func setupBenchmarkLSPClient(b *testing.B) LSPClient {
	client := NewBenchmarkMockLSPClient()
	ctx := context.Background()

	err := client.Start(ctx)
	if err != nil {
		b.Fatalf("Failed to start LSP client: %v", err)
	}

	return client
}

func teardownBenchmarkLSPClient(b *testing.B, client LSPClient) {
	if err := client.Stop(); err != nil {
		b.Logf("Error stopping LSP client: %v", err)
	}
}

func createLSPParams(method string) interface{} {
	switch method {
	case "textDocument/definition", "textDocument/references", "textDocument/hover":
		return map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": "file:///test.go",
			},
			"position": map[string]interface{}{
				"line":      10,
				"character": 5,
			},
		}
	case "textDocument/documentSymbol":
		return map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": "file:///test.go",
			},
		}
	case "workspace/symbol":
		return map[string]interface{}{
			"query": "main",
		}
	default:
		return map[string]interface{}{}
	}
}

func createLargeResponse() interface{} {
	result := make([]map[string]interface{}, 1000)
	for i := 0; i < 1000; i++ {
		result[i] = map[string]interface{}{
			"name": fmt.Sprintf("symbol_%d", i),
			"kind": 12, // Function
			"location": map[string]interface{}{
				"uri": "file:///test.go",
				"range": map[string]interface{}{
					"start": map[string]interface{}{"line": i, "character": 0},
					"end":   map[string]interface{}{"line": i, "character": 10},
				},
			},
		}
	}
	return result
}

type BenchmarkMockLSPClient struct {
	active bool
	mu     sync.RWMutex
}

func NewBenchmarkMockLSPClient() *BenchmarkMockLSPClient {
	return &BenchmarkMockLSPClient{active: false}
}

func (m *BenchmarkMockLSPClient) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.active = true
	return nil
}

func (m *BenchmarkMockLSPClient) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.active = false
	return nil
}

func (m *BenchmarkMockLSPClient) SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	time.Sleep(1 * time.Microsecond)

	switch method {
	case "textDocument/definition":
		return json.RawMessage(`{"uri":"file:///test.go","range":{"start":{"line":10,"character":5},"end":{"line":10,"character":15}}}`), nil
	case "textDocument/references":
		return json.RawMessage(`[{"uri":"file:///test.go","range":{"start":{"line":10,"character":5},"end":{"line":10,"character":15}}}]`), nil
	case "textDocument/hover":
		return json.RawMessage(`{"contents":"function definition"}`), nil
	case "textDocument/documentSymbol":
		return json.RawMessage(`[{"name":"main","kind":12,"range":{"start":{"line":0,"character":0},"end":{"line":10,"character":0}}}]`), nil
	case "workspace/symbol":
		return json.RawMessage(`[{"name":"main","kind":12,"location":{"uri":"file:///test.go","range":{"start":{"line":0,"character":0},"end":{"line":10,"character":0}}}}]`), nil
	default:
		return json.RawMessage(`{"result":null}`), nil
	}
}

func (m *BenchmarkMockLSPClient) SendNotification(ctx context.Context, method string, params interface{}) error {
	return nil
}

func (m *BenchmarkMockLSPClient) IsActive() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.active
}

type ErrorMockLSPClient struct{}

func (m *ErrorMockLSPClient) Start(ctx context.Context) error {
	return fmt.Errorf("mock start error")
}

func (m *ErrorMockLSPClient) Stop() error {
	return fmt.Errorf("mock stop error")
}

func (m *ErrorMockLSPClient) SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	return nil, fmt.Errorf("mock request error")
}

func (m *ErrorMockLSPClient) SendNotification(ctx context.Context, method string, params interface{}) error {
	return fmt.Errorf("mock notification error")
}

func (m *ErrorMockLSPClient) IsActive() bool {
	return false
}
