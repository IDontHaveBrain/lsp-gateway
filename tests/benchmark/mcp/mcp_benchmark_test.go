package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"lsp-gateway/mcp"
)

func BenchmarkMCPServerThroughput(b *testing.B) {
	throughputTests := []struct {
		name      string
		workers   int
		targetTPS float64
		method    string
	}{
		{"ToolsList_Single", 1, 100.0, "tools/list"},
		{"ToolsList_Parallel_5", 5, 300.0, "tools/list"},
		{"ToolsList_Parallel_10", 10, 500.0, "tools/list"},
		{"ToolCall_Single", 1, 100.0, "tools/call"},
		{"ToolCall_Parallel_5", 5, 300.0, "tools/call"},
		{"ToolCall_Parallel_10", 10, 500.0, "tools/call"},
		{"Ping_HighThroughput", 20, 1000.0, "ping"},
	}

	for _, tt := range throughputTests {
		b.Run(tt.name, func(b *testing.B) {
			benchmarkMCPThroughput(b, tt.workers, tt.targetTPS, tt.method)
		})
	}
}

func benchmarkMCPThroughput(b *testing.B, workers int, targetTPS float64, method string) {
	server := setupBenchmarkMCPServer(b)
	defer teardownBenchmarkMCPServer(b, server)

	const testDuration = 10 * time.Second
	var requestCount int64
	var errorCount int64
	done := make(chan struct{})

	message := createMCPMessage(method)

	b.ResetTimer()
	startTime := time.Now()

	for i := 0; i < workers; i++ {
		go func() {
			for {
				select {
				case <-done:
					return
				default:
					err := processMCPMessage(server, message)
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

func BenchmarkMCPMessageProcessing(b *testing.B) {
	messageTypes := []struct {
		name   string
		method string
	}{
		{"Initialize", "initialize"},
		{"ToolsList", "tools/list"},
		{"ToolCall_Definition", "tools/call"},
		{"ToolCall_References", "tools/call"},
		{"ToolCall_Hover", "tools/call"},
		{"Ping", "ping"},
	}

	for _, mt := range messageTypes {
		b.Run(mt.name, func(b *testing.B) {
			benchmarkMessageProcessing(b, mt.method)
		})
	}
}

func benchmarkMessageProcessing(b *testing.B, method string) {
	server := setupBenchmarkMCPServer(b)
	defer teardownBenchmarkMCPServer(b, server)

	message := createMCPMessage(method)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		err := processMCPMessage(server, message)
		if err != nil {
			b.Fatalf("Message processing failed: %v", err)
		}
	}
}

func BenchmarkMCPConcurrentClients(b *testing.B) {
	concurrencyLevels := []int{1, 5, 10, 20, 50}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Clients_%d", concurrency), func(b *testing.B) {
			benchmarkConcurrentMCPClients(b, concurrency)
		})
	}
}

func benchmarkConcurrentMCPClients(b *testing.B, concurrency int) {
	server := setupBenchmarkMCPServer(b)
	defer teardownBenchmarkMCPServer(b, server)

	message := createMCPMessage("tools/list")

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
				err := processMCPMessage(server, message)
				if err != nil {
					b.Errorf("Message processing failed: %v", err)
				}
			}
		}()
	}

	wg.Wait()
}

func BenchmarkMCPProtocolParsing(b *testing.B) {
	parseTests := []struct {
		name    string
		message string
	}{
		{"SimpleMessage", "Content-Length: 45\r\n\r\n{\"jsonrpc\":\"2.0\",\"method\":\"ping\",\"id\":1}"},
		{"ToolCallMessage", "Content-Length: 150\r\n\r\n{\"jsonrpc\":\"2.0\",\"method\":\"tools/call\",\"params\":{\"name\":\"goto_definition\",\"arguments\":{\"uri\":\"file:///test.go\",\"position\":{\"line\":10,\"character\":5}}},\"id\":1}"},
		{"LargeMessage", createLargeMCPMessage()},
	}

	for _, pt := range parseTests {
		b.Run(pt.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := parseMCPMessage(pt.message)
				if err != nil {
					b.Fatalf("Message parsing failed: %v", err)
				}
			}
		})
	}
}

func BenchmarkMCPMemoryUsage(b *testing.B) {
	runtime.GC()
	var startMem runtime.MemStats
	runtime.ReadMemStats(&startMem)

	server := setupBenchmarkMCPServer(b)
	defer teardownBenchmarkMCPServer(b, server)

	message := createMCPMessage("tools/call")

	const samplingInterval = 1000
	memSamples := make([]uint64, 0, b.N/samplingInterval+1)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := processMCPMessage(server, message)
		if err != nil {
			b.Fatalf("Message processing failed: %v", err)
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

func BenchmarkMCPLatencyProfile(b *testing.B) {
	server := setupBenchmarkMCPServer(b)
	defer teardownBenchmarkMCPServer(b, server)

	message := createMCPMessage("tools/call")
	latencies := make([]time.Duration, b.N)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		start := time.Now()
		err := processMCPMessage(server, message)
		latencies[i] = time.Since(start)

		if err != nil {
			b.Fatalf("Message processing failed: %v", err)
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

func BenchmarkMCPErrorHandling(b *testing.B) {
	server := setupBenchmarkMCPServer(b)
	defer teardownBenchmarkMCPServer(b, server)

	errorCases := []struct {
		name    string
		message string
	}{
		{"InvalidJSON", `Content-Length: 20\r\n\r\n{"invalid": json}`},
		{"MissingMethod", `Content-Length: 30\r\n\r\n{"jsonrpc": "2.0", "id": 1}`},
		{"UnsupportedMethod", `Content-Length: 55\r\n\r\n{"jsonrpc": "2.0", "method": "unsupported", "id": 1}`},
		{"MalformedHeader", `Invalid-Header: 30\r\n\r\n{"jsonrpc": "2.0", "id": 1}`},
	}

	for _, ec := range errorCases {
		b.Run(ec.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, err := parseMCPMessage(ec.message)
				_ = err
			}
		})
	}
}

func BenchmarkMCPToolExecution(b *testing.B) {
	tools := []struct {
		name string
		tool string
		args map[string]interface{}
	}{
		{
			"GotoDefinition",
			"goto_definition",
			map[string]interface{}{
				"uri":      "file:///test.go",
				"position": map[string]interface{}{"line": 10, "character": 5},
			},
		},
		{
			"FindReferences",
			"find_references",
			map[string]interface{}{
				"uri":      "file:///test.go",
				"position": map[string]interface{}{"line": 10, "character": 5},
			},
		},
		{
			"GetHoverInfo",
			"get_hover_info",
			map[string]interface{}{
				"uri":      "file:///test.go",
				"position": map[string]interface{}{"line": 10, "character": 5},
			},
		},
		{
			"GetDocumentSymbols",
			"get_document_symbols",
			map[string]interface{}{
				"uri": "file:///test.go",
			},
		},
		{
			"SearchWorkspaceSymbols",
			"search_workspace_symbols",
			map[string]interface{}{
				"query": "main",
			},
		},
	}

	for _, tool := range tools {
		b.Run(tool.name, func(b *testing.B) {
			benchmarkToolExecution(b, tool.tool, tool.args)
		})
	}
}

func benchmarkToolExecution(b *testing.B, toolName string, args map[string]interface{}) {
	server := setupBenchmarkMCPServer(b)
	defer teardownBenchmarkMCPServer(b, server)

	call := mcp.ToolCall{
		Name:      toolName,
		Arguments: args,
	}

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		result, err := server.ToolHandler.CallTool(ctx, call)
		if err != nil {
			b.Fatalf("Tool execution failed: %v", err)
		}
		if result == nil {
			b.Fatal("Tool result is nil")
		}
	}
}

func setupBenchmarkMCPServer(b *testing.B) *mcp.Server {
	config := &mcp.ServerConfig{
		Name:          "lsp-gateway-benchmark",
		Version:       "1.0.0",
		LSPGatewayURL: "http://localhost:8080",
		Timeout:       30 * time.Second,
		MaxRetries:    3,
	}

	server := mcp.NewServer(config)

	inputBuf := &bytes.Buffer{}
	outputBuf := &bytes.Buffer{}
	server.SetIO(inputBuf, outputBuf)

	return server
}

func teardownBenchmarkMCPServer(b *testing.B, server *mcp.Server) {
	if err := server.Stop(); err != nil {
		b.Logf("Error stopping MCP server: %v", err)
	}
}

func createMCPMessage(method string) string {
	var msg mcp.MCPMessage

	switch method {
	case "initialize":
		msg = mcp.MCPMessage{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "initialize",
			Params: mcp.InitializeParams{
				ProtocolVersion: "2024-11-05",
				Capabilities:    map[string]interface{}{},
				ClientInfo:      map[string]interface{}{"name": "benchmark-client"},
			},
		}
	case "tools/list":
		msg = mcp.MCPMessage{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "tools/list",
		}
	case "tools/call":
		msg = mcp.MCPMessage{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "tools/call",
			Params: mcp.ToolCall{
				Name: "goto_definition",
				Arguments: map[string]interface{}{
					"uri":      "file:///test.go",
					"position": map[string]interface{}{"line": 10, "character": 5},
				},
			},
		}
	case "ping":
		msg = mcp.MCPMessage{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "ping",
		}
	}

	data, _ := json.Marshal(msg)
	return fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(data), string(data))
}

func createLargeMCPMessage() string {
	largeArgs := make(map[string]interface{})
	for i := 0; i < 100; i++ {
		largeArgs[fmt.Sprintf("arg_%d", i)] = fmt.Sprintf("value_%d", i)
	}

	msg := mcp.MCPMessage{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params: mcp.ToolCall{
			Name:      "large_tool",
			Arguments: largeArgs,
		},
	}

	data, _ := json.Marshal(msg)
	return fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(data), string(data))
}

func parseMCPMessage(rawMessage string) (*mcp.MCPMessage, error) {
	lines := bytes.Split([]byte(rawMessage), []byte("\r\n"))

	var contentLength int
	var err error

	for _, line := range lines {
		lineStr := string(line)
		if bytes.HasPrefix(line, []byte("Content-Length:")) {
			parts := bytes.SplitN(line, []byte(":"), 2)
			if len(parts) == 2 {
				lengthStr := bytes.TrimSpace(parts[1])
				contentLength, err = fmt.Sscanf(string(lengthStr), "%d", &contentLength)
				if err != nil {
					return nil, err
				}
			}
		}
		if lineStr == "" {
			break
		}
	}

	parts := bytes.Split([]byte(rawMessage), []byte("\r\n\r\n"))
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid message format")
	}

	var msg mcp.MCPMessage
	err = json.Unmarshal(parts[1], &msg)
	return &msg, err
}

func processMCPMessage(server *mcp.Server, rawMessage string) error {
	_, err := parseMCPMessage(rawMessage)
	if err != nil {
		return err
	}

	ctx := context.Background()

	parts := bytes.Split([]byte(rawMessage), []byte("\r\n\r\n"))
	if len(parts) < 2 {
		return fmt.Errorf("invalid message format")
	}

	return server.HandleMessageWithValidation(ctx, string(parts[1]))
}
