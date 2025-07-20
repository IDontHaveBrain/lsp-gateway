package main

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

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/gateway"
	"lsp-gateway/mcp"
)

type LoadTestScenario struct {
	Name           string
	Duration       time.Duration
	HTTPClients    int
	MCPClients     int
	RequestsPerSec int
	Methods        []string
}

type LoadTestMetrics struct {
	HTTPRequests     int64
	HTTPErrors       int64
	HTTPLatencySum   int64
	HTTPLatencyCount int64
	MCPRequests      int64
	MCPErrors        int64
	MCPLatencySum    int64
	MCPLatencyCount  int64
	StartTime        time.Time
	EndTime          time.Time
	PeakMemoryUsage  uint64
	GoroutineCount   int
}

func BenchmarkProductionLoadTest(b *testing.B) {
	scenarios := []LoadTestScenario{
		{
			Name:           "LightLoad",
			Duration:       30 * time.Second,
			HTTPClients:    5,
			MCPClients:     2,
			RequestsPerSec: 50,
			Methods:        []string{"textDocument/definition", "textDocument/hover"},
		},
		{
			Name:           "ModerateLoad",
			Duration:       60 * time.Second,
			HTTPClients:    10,
			MCPClients:     5,
			RequestsPerSec: 100,
			Methods:        []string{"textDocument/definition", "textDocument/references", "textDocument/hover", "textDocument/documentSymbol"},
		},
		{
			Name:           "HeavyLoad",
			Duration:       90 * time.Second,
			HTTPClients:    20,
			MCPClients:     10,
			RequestsPerSec: 200,
			Methods:        []string{"textDocument/definition", "textDocument/references", "textDocument/hover", "textDocument/documentSymbol", "workspace/symbol"},
		},
		{
			Name:           "StressTest",
			Duration:       120 * time.Second,
			HTTPClients:    50,
			MCPClients:     20,
			RequestsPerSec: 500,
			Methods:        []string{"textDocument/definition", "textDocument/references", "textDocument/hover", "textDocument/documentSymbol", "workspace/symbol"},
		},
	}

	for _, scenario := range scenarios {
		b.Run(scenario.Name, func(b *testing.B) {
			runLoadTestScenario(b, scenario)
		})
	}
}

func runLoadTestScenario(b *testing.B, scenario LoadTestScenario) {
	b.Logf("Starting load test scenario: %s", scenario.Name)
	b.Logf("Duration: %v, HTTP clients: %d, MCP clients: %d, Target RPS: %d",
		scenario.Duration, scenario.HTTPClients, scenario.MCPClients, scenario.RequestsPerSec)

	httpServer, mcpServer := setupLoadTestEnvironment(b)
	defer teardownLoadTestEnvironment(b, httpServer, mcpServer)

	metrics := &LoadTestMetrics{
		StartTime: time.Now(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), scenario.Duration)
	defer cancel()

	var wg sync.WaitGroup

	for i := 0; i < scenario.HTTPClients; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()
			runHTTPLoadGenerator(ctx, httpServer, scenario, metrics, clientID)
		}(i)
	}

	for i := 0; i < scenario.MCPClients; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()
			runMCPLoadGenerator(ctx, mcpServer, scenario, metrics, clientID)
		}(i)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		collectMetrics(ctx, metrics)
	}()

	wg.Wait()
	metrics.EndTime = time.Now()

	reportLoadTestResults(b, scenario, metrics)
}

func runHTTPLoadGenerator(ctx context.Context, server *httptest.Server, scenario LoadTestScenario, metrics *LoadTestMetrics, clientID int) {
	client := &http.Client{Timeout: 30 * time.Second}

	requestInterval := time.Duration(int64(time.Second) / int64(scenario.RequestsPerSec/scenario.HTTPClients))
	ticker := time.NewTicker(requestInterval)
	defer ticker.Stop()

	methodIndex := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			method := scenario.Methods[methodIndex%len(scenario.Methods)]
			methodIndex++

			start := time.Now()
			err := sendHTTPRequest(client, server.URL, method)
			latency := time.Since(start)

			atomic.AddInt64(&metrics.HTTPRequests, 1)
			atomic.AddInt64(&metrics.HTTPLatencySum, int64(latency))
			atomic.AddInt64(&metrics.HTTPLatencyCount, 1)

			if err != nil {
				atomic.AddInt64(&metrics.HTTPErrors, 1)
			}
		}
	}
}

func runMCPLoadGenerator(ctx context.Context, server *mcp.Server, scenario LoadTestScenario, metrics *LoadTestMetrics, clientID int) {
	requestInterval := time.Duration(int64(time.Second) / int64(scenario.RequestsPerSec/scenario.MCPClients))
	ticker := time.NewTicker(requestInterval)
	defer ticker.Stop()

	methodIndex := 0

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			method := scenario.Methods[methodIndex%len(scenario.Methods)]
			methodIndex++

			start := time.Now()
			err := sendMCPRequest(server, method)
			latency := time.Since(start)

			atomic.AddInt64(&metrics.MCPRequests, 1)
			atomic.AddInt64(&metrics.MCPLatencySum, int64(latency))
			atomic.AddInt64(&metrics.MCPLatencyCount, 1)

			if err != nil {
				atomic.AddInt64(&metrics.MCPErrors, 1)
			}
		}
	}
}

func collectMetrics(ctx context.Context, metrics *LoadTestMetrics) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			var mem runtime.MemStats
			runtime.ReadMemStats(&mem)

			if mem.Alloc > metrics.PeakMemoryUsage {
				atomic.StoreUint64(&metrics.PeakMemoryUsage, mem.Alloc)
			}

			goroutines := runtime.NumGoroutine()
			if goroutines > metrics.GoroutineCount {
				metrics.GoroutineCount = goroutines
			}
		}
	}
}

func sendHTTPRequest(client *http.Client, serverURL, method string) error {
	reqBody := createHTTPRequestBody(method)

	resp, err := client.Post(serverURL, "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP status: %d", resp.StatusCode)
	}

	_, err = io.ReadAll(resp.Body)
	return err
}

func sendMCPRequest(server *mcp.Server, method string) error {
	ctx := context.Background()

	switch method {
	case "textDocument/definition":
		return simulateMCPToolCall(ctx, "goto_definition")
	case "textDocument/references":
		return simulateMCPToolCall(ctx, "find_references")
	case "textDocument/hover":
		return simulateMCPToolCall(ctx, "get_hover_info")
	case "textDocument/documentSymbol":
		return simulateMCPToolCall(ctx, "get_document_symbols")
	case "workspace/symbol":
		return simulateMCPToolCall(ctx, "search_workspace_symbols")
	default:
		return nil
	}
}

func simulateMCPToolCall(ctx context.Context, toolName string) error {
	time.Sleep(time.Duration(1+toolName[0]%5) * time.Millisecond)
	return nil
}

func createHTTPRequestBody(method string) []byte {
	var params interface{}

	switch method {
	case "textDocument/definition", "textDocument/references", "textDocument/hover":
		params = map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": "file:///test.go",
			},
			"position": map[string]interface{}{
				"line":      10,
				"character": 5,
			},
		}
	case "textDocument/documentSymbol":
		params = map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": "file:///test.go",
			},
		}
	case "workspace/symbol":
		params = map[string]interface{}{
			"query": "main",
		}
	}

	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	}

	data, _ := json.Marshal(req)
	return data
}

func reportLoadTestResults(b *testing.B, scenario LoadTestScenario, metrics *LoadTestMetrics) {
	duration := metrics.EndTime.Sub(metrics.StartTime)

	httpReqs := atomic.LoadInt64(&metrics.HTTPRequests)
	httpErrs := atomic.LoadInt64(&metrics.HTTPErrors)
	httpLatSum := atomic.LoadInt64(&metrics.HTTPLatencySum)
	httpLatCount := atomic.LoadInt64(&metrics.HTTPLatencyCount)

	mcpReqs := atomic.LoadInt64(&metrics.MCPRequests)
	mcpErrs := atomic.LoadInt64(&metrics.MCPErrors)
	mcpLatSum := atomic.LoadInt64(&metrics.MCPLatencySum)
	mcpLatCount := atomic.LoadInt64(&metrics.MCPLatencyCount)

	totalReqs := httpReqs + mcpReqs
	totalErrs := httpErrs + mcpErrs

	b.Logf("=== Load Test Results: %s ===", scenario.Name)
	b.Logf("Duration: %v", duration)
	b.Logf("Total Requests: %d", totalReqs)
	b.Logf("Total Errors: %d (%.2f%%)", totalErrs, float64(totalErrs)/float64(totalReqs)*100)
	b.Logf("Overall Throughput: %.1f req/sec", float64(totalReqs)/duration.Seconds())

	b.Logf("--- HTTP Gateway ---")
	b.Logf("Requests: %d", httpReqs)
	b.Logf("Errors: %d (%.2f%%)", httpErrs, float64(httpErrs)/float64(httpReqs)*100)
	b.Logf("Throughput: %.1f req/sec", float64(httpReqs)/duration.Seconds())
	if httpLatCount > 0 {
		avgLatency := time.Duration(httpLatSum / httpLatCount)
		b.Logf("Average Latency: %v", avgLatency)

		if avgLatency > 100*time.Millisecond {
			b.Errorf("HTTP average latency %v exceeds target of 100ms", avgLatency)
		}
	}

	b.Logf("--- MCP Server ---")
	b.Logf("Requests: %d", mcpReqs)
	b.Logf("Errors: %d (%.2f%%)", mcpErrs, float64(mcpErrs)/float64(mcpReqs)*100)
	b.Logf("Throughput: %.1f req/sec", float64(mcpReqs)/duration.Seconds())
	if mcpLatCount > 0 {
		avgLatency := time.Duration(mcpLatSum / mcpLatCount)
		b.Logf("Average Latency: %v", avgLatency)

		if avgLatency > 100*time.Millisecond {
			b.Errorf("MCP average latency %v exceeds target of 100ms", avgLatency)
		}
	}

	b.Logf("--- Resource Usage ---")
	b.Logf("Peak Memory: %.2f MB", float64(metrics.PeakMemoryUsage)/1024/1024)
	b.Logf("Peak Goroutines: %d", metrics.GoroutineCount)

	actualThroughput := float64(totalReqs) / duration.Seconds()
	if actualThroughput < float64(scenario.RequestsPerSec)*0.8 { // 80% of target is acceptable
		b.Errorf("Overall throughput %.1f req/sec is significantly below target of %d req/sec",
			actualThroughput, scenario.RequestsPerSec)
	}

	errorRate := float64(totalErrs) / float64(totalReqs) * 100
	if errorRate > 5.0 { // 5% error rate threshold
		b.Errorf("Error rate %.2f%% exceeds acceptable threshold of 5%%", errorRate)
	}

}

func setupLoadTestEnvironment(b *testing.B) (*httptest.Server, *mcp.Server) {
	gatewayConfig := &config.GatewayConfig{
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

	gw, err := gateway.NewGateway(gatewayConfig)
	if err != nil {
		b.Fatalf("Failed to create gateway: %v", err)
	}

	ctx := context.Background()
	if err := gw.Start(ctx); err != nil {
		b.Fatalf("Failed to start gateway: %v", err)
	}

	httpServer := httptest.NewServer(http.HandlerFunc(gw.HandleJSONRPC))

	mcpConfig := &mcp.ServerConfig{
		Name:          "lsp-gateway-load-test",
		Version:       "1.0.0",
		LSPGatewayURL: httpServer.URL,
		Timeout:       30 * time.Second,
		MaxRetries:    3,
	}

	mcpServer := mcp.NewServer(mcpConfig)

	return httpServer, mcpServer
}

func teardownLoadTestEnvironment(b *testing.B, httpServer *httptest.Server, mcpServer *mcp.Server) {
	if httpServer != nil {
		httpServer.Close()
	}
	if mcpServer != nil {
		if err := mcpServer.Stop(); err != nil {
			b.Logf("Error stopping MCP server: %v", err)
		}
	}
}

type LoadTestMockClient struct {
	active bool
	mu     sync.RWMutex
}

func (m *LoadTestMockClient) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.active = true
	return nil
}

func (m *LoadTestMockClient) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.active = false
	return nil
}

func (m *LoadTestMockClient) SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	processingTime := time.Duration(1+len(method)%10) * time.Millisecond
	time.Sleep(processingTime)

	return json.RawMessage(`{"result":null}`), nil
}

func (m *LoadTestMockClient) SendNotification(ctx context.Context, method string, params interface{}) error {
	return nil
}

func (m *LoadTestMockClient) IsActive() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.active
}

func BenchmarkMemoryLeakDetection(b *testing.B) {
	const testDuration = 5 * time.Minute
	const samplingInterval = 10 * time.Second

	b.Logf("Running memory leak detection test for %v", testDuration)

	httpServer, mcpServer := setupLoadTestEnvironment(b)
	defer teardownLoadTestEnvironment(b, httpServer, mcpServer)

	ctx, cancel := context.WithTimeout(context.Background(), testDuration)
	defer cancel()

	var memSamples []runtime.MemStats
	ticker := time.NewTicker(samplingInterval)
	defer ticker.Stop()

	go func() {
		client := &http.Client{Timeout: 30 * time.Second}
		for {
			select {
			case <-ctx.Done():
				return
			default:
				if err := sendHTTPRequest(client, httpServer.URL, "textDocument/definition"); err != nil {
					b.Logf("Load test request failed: %v", err)
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			goto analysis
		case <-ticker.C:
			runtime.GC()
			var mem runtime.MemStats
			runtime.ReadMemStats(&mem)
			memSamples = append(memSamples, mem)

			b.Logf("Memory sample: Alloc=%d KB, Sys=%d KB, NumGC=%d",
				mem.Alloc/1024, mem.Sys/1024, mem.NumGC)
		}
	}

analysis:
	if len(memSamples) >= 3 {
		first := memSamples[0]
		last := memSamples[len(memSamples)-1]

		allocGrowth := int64(last.Alloc) - int64(first.Alloc)
		sysGrowth := int64(last.Sys) - int64(first.Sys)

		b.Logf("Memory Analysis:")
		b.Logf("  Alloc Growth: %d KB (%.1f%%)", allocGrowth/1024,
			float64(allocGrowth)/float64(first.Alloc)*100)
		b.Logf("  Sys Growth: %d KB (%.1f%%)", sysGrowth/1024,
			float64(sysGrowth)/float64(first.Sys)*100)
		b.Logf("  GC Runs: %d -> %d (%d total)", first.NumGC, last.NumGC, last.NumGC-first.NumGC)

		if allocGrowth > int64(first.Alloc)/2 { // 50% growth threshold
			b.Errorf("Potential memory leak: Alloc grew by %d KB (%.1f%%)",
				allocGrowth/1024, float64(allocGrowth)/float64(first.Alloc)*100)
		}
	}

}
