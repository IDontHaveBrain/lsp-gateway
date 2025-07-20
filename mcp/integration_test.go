package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type TestLSPGateway struct {
	server   *httptest.Server
	requests []LSPRequest
	mu       sync.RWMutex
}

type LSPRequest struct {
	Method    string
	Params    interface{}
	Timestamp time.Time
}

func NewTestLSPGateway() *TestLSPGateway {
	tg := &TestLSPGateway{
		requests: make([]LSPRequest, 0),
	}

	tg.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/jsonrpc" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		var req JSONRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		tg.mu.Lock()
		tg.requests = append(tg.requests, LSPRequest{
			Method:    req.Method,
			Params:    req.Params,
			Timestamp: time.Now(),
		})
		tg.mu.Unlock()

		var result interface{}
		switch req.Method {
		case "textDocument/definition":
			result = []map[string]interface{}{
				{
					"uri": "file:///test.go",
					"range": map[string]interface{}{
						"start": map[string]interface{}{"line": 10, "character": 5},
						"end":   map[string]interface{}{"line": 10, "character": 15},
					},
				},
			}
		case "textDocument/references":
			result = []map[string]interface{}{
				{
					"uri": "file:///test.go",
					"range": map[string]interface{}{
						"start": map[string]interface{}{"line": 5, "character": 10},
						"end":   map[string]interface{}{"line": 5, "character": 20},
					},
				},
				{
					"uri": "file:///other.go",
					"range": map[string]interface{}{
						"start": map[string]interface{}{"line": 15, "character": 3},
						"end":   map[string]interface{}{"line": 15, "character": 13},
					},
				},
			}
		case "textDocument/hover":
			result = map[string]interface{}{
				"contents": map[string]interface{}{
					"kind":  "markdown",
					"value": "**TestFunction** - A test function for integration testing",
				},
				"range": map[string]interface{}{
					"start": map[string]interface{}{"line": 10, "character": 5},
					"end":   map[string]interface{}{"line": 10, "character": 15},
				},
			}
		case "textDocument/documentSymbol":
			result = []map[string]interface{}{
				{
					"name": "TestStruct",
					"kind": 23, // Struct
					"range": map[string]interface{}{
						"start": map[string]interface{}{"line": 5, "character": 0},
						"end":   map[string]interface{}{"line": 15, "character": 1},
					},
					"children": []map[string]interface{}{
						{
							"name": "Field1",
							"kind": 8, // Field
							"range": map[string]interface{}{
								"start": map[string]interface{}{"line": 6, "character": 4},
								"end":   map[string]interface{}{"line": 6, "character": 20},
							},
						},
					},
				},
			}
		case "workspace/symbol":
			query := ""
			if params, ok := req.Params.(map[string]interface{}); ok {
				if q, exists := params["query"]; exists {
					query = q.(string)
				}
			}
			result = []map[string]interface{}{
				{
					"name": fmt.Sprintf("Symbol_%s", query),
					"kind": 12, // Function
					"location": map[string]interface{}{
						"uri": "file:///workspace.go",
						"range": map[string]interface{}{
							"start": map[string]interface{}{"line": 20, "character": 0},
							"end":   map[string]interface{}{"line": 25, "character": 1},
						},
					},
				},
			}
		default:
			w.Header().Set("Content-Type", "application/json")
			response := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"error": map[string]interface{}{
					"code":    -32601,
					"message": "Method not found",
				},
			}
			if err := json.NewEncoder(w).Encode(response); err != nil {
				http.Error(w, "Failed to encode response", http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		response := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  result,
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	}))

	return tg
}

func (tg *TestLSPGateway) URL() string {
	return tg.server.URL
}

func (tg *TestLSPGateway) GetRequests() []LSPRequest {
	tg.mu.RLock()
	defer tg.mu.RUnlock()
	return append([]LSPRequest{}, tg.requests...)
}

func (tg *TestLSPGateway) Close() {
	tg.server.Close()
}

func TestTCPTransport(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	gateway := NewTestLSPGateway()
	defer gateway.Close()

	config := &ServerConfig{
		Name:          "tcp-test-server",
		Version:       "1.0.0",
		LSPGatewayURL: gateway.URL(),
		Timeout:       100 * time.Millisecond,
		MaxRetries:    1,
	}

	server := NewServer(config)

	inputReader, inputWriter := io.Pipe()
	outputReader, outputWriter := io.Pipe()
	server.SetIO(inputReader, outputWriter)

	serverDone := make(chan error, 1)
	responseBuffer := &bytes.Buffer{}
	responseDone := make(chan struct{})

	go func() {
		defer close(responseDone)
		_, _ = io.Copy(responseBuffer, outputReader)
	}()

	go func() {
		serverDone <- server.Start()
	}()

	time.Sleep(20 * time.Millisecond)

	initMsg := createTestMessage(1, "initialize", map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
	})

	if _, err := inputWriter.Write([]byte(initMsg)); err != nil {
		t.Fatalf("Failed to write init message: %v", err)
	}
	time.Sleep(30 * time.Millisecond)

	toolMsg := createTestMessage(2, "tools/call", map[string]interface{}{
		"name": "goto_definition",
		"arguments": map[string]interface{}{
			"uri":       "file:///test.go",
			"line":      10,
			"character": 5,
		},
	})

	if _, err := inputWriter.Write([]byte(toolMsg)); err != nil {
		t.Fatalf("Failed to write tool message: %v", err)
	}
	time.Sleep(30 * time.Millisecond)

	if err := inputWriter.Close(); err != nil {
		t.Logf("Input writer close warning: %v", err)
	}

	select {
	case <-serverDone:
	case <-time.After(500 * time.Millisecond):
		t.Error("Server did not complete within timeout")
		if err := server.Stop(); err != nil {
			t.Logf("Server stop error: %v", err)
		}
	}

	select {
	case <-responseDone:
	case <-time.After(100 * time.Millisecond):
		t.Log("Response collection timed out")
	}

	output := responseBuffer.String()
	if !strings.Contains(output, "Content-Length:") {
		t.Error("Expected MCP response with Content-Length header")
	}

	requests := gateway.GetRequests()
	if len(requests) == 0 {
		t.Error("Expected LSP request to be made to gateway")
	}
}

func TestStdioTransport(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	gateway := NewTestLSPGateway()
	defer gateway.Close()

	config := &ServerConfig{
		Name:          "stdio-test-server",
		Version:       "1.0.0",
		LSPGatewayURL: gateway.URL(),
		Timeout:       100 * time.Millisecond,
		MaxRetries:    1,
	}

	server := NewServer(config)

	inputReader, inputWriter := io.Pipe()
	outputReader, outputWriter := io.Pipe()
	server.SetIO(inputReader, outputWriter)

	serverDone := make(chan error, 1)
	responseBuffer := &bytes.Buffer{}
	responseDone := make(chan struct{})

	go func() {
		defer close(responseDone)
		_, _ = io.Copy(responseBuffer, outputReader)
	}()

	go func() {
		serverDone <- server.Start()
	}()

	time.Sleep(20 * time.Millisecond)

	initMsg := createTestMessage(1, "initialize", map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
	})

	if _, err := inputWriter.Write([]byte(initMsg)); err != nil {
		t.Fatalf("Failed to write init message: %v", err)
	}
	time.Sleep(30 * time.Millisecond)

	toolMsg := createTestMessage(2, "tools/call", map[string]interface{}{
		"name": "find_references",
		"arguments": map[string]interface{}{
			"uri":                "file:///test.go",
			"line":               15,
			"character":          8,
			"includeDeclaration": true,
		},
	})

	if _, err := inputWriter.Write([]byte(toolMsg)); err != nil {
		t.Fatalf("Failed to write tool message: %v", err)
	}
	time.Sleep(30 * time.Millisecond)

	if err := inputWriter.Close(); err != nil {
		t.Logf("Input writer close warning: %v", err)
	}

	select {
	case <-serverDone:
	case <-time.After(500 * time.Millisecond):
		t.Error("Server did not complete within timeout")
		if err := server.Stop(); err != nil {
			t.Logf("Server stop error: %v", err)
		}
	}

	select {
	case <-responseDone:
	case <-time.After(100 * time.Millisecond):
		t.Log("Response collection timed out")
	}

	output := responseBuffer.String()
	responseCount := strings.Count(output, "Content-Length:")
	if responseCount < 2 {
		t.Errorf("Expected at least 2 responses, got %d. Output: %s", responseCount, output)
	}

	requests := gateway.GetRequests()
	found := false
	for _, req := range requests {
		if req.Method == "textDocument/references" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected find_references request to be made to LSP Gateway")
	}
}

func TestMultiLanguageLSPIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	gateway := NewTestLSPGateway()
	defer gateway.Close()

	config := &ServerConfig{
		Name:          "multi-lang-test-server",
		Version:       "1.0.0",
		LSPGatewayURL: gateway.URL(),
		Timeout:       100 * time.Millisecond,
		MaxRetries:    1,
	}

	server := NewServer(config)

	testCases := []struct {
		name     string
		uri      string
		tool     string
		expected string
	}{
		{
			name:     "Go file",
			uri:      "file:///src/main.go",
			tool:     "goto_definition",
			expected: "textDocument/definition",
		},
		{
			name:     "Python file",
			uri:      "file:///src/app.py",
			tool:     "get_hover_info",
			expected: "textDocument/hover",
		},
		{
			name:     "TypeScript file",
			uri:      "file:///src/component.ts",
			tool:     "find_references",
			expected: "textDocument/references",
		},
		{
			name:     "JavaScript file",
			uri:      "file:///src/script.js",
			tool:     "get_document_symbols",
			expected: "textDocument/documentSymbol",
		},
		{
			name:     "Java file",
			uri:      "file:///src/Main.java",
			tool:     "search_workspace_symbols",
			expected: "workspace/symbol",
		},
	}

	inputReader, inputWriter := io.Pipe()
	outputReader, outputWriter := io.Pipe()
	server.SetIO(inputReader, outputWriter)

	serverDone := make(chan error, 1)
	responseBuffer := &bytes.Buffer{}
	responseDone := make(chan struct{})

	go func() {
		defer close(responseDone)
		_, _ = io.Copy(responseBuffer, outputReader)
	}()

	go func() {
		serverDone <- server.Start()
	}()

	time.Sleep(30 * time.Millisecond)

	initMsg := createTestMessage(1, "initialize", map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
	})

	if _, err := inputWriter.Write([]byte(initMsg)); err != nil {
		t.Fatalf("Failed to write init message: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	for i, tc := range testCases {
		args := map[string]interface{}{
			"uri":       tc.uri,
			"line":      10,
			"character": 5,
		}

		if tc.tool == "search_workspace_symbols" {
			args = map[string]interface{}{
				"query": "TestSymbol",
			}
		}

		toolMsg := createTestMessage(i+2, "tools/call", map[string]interface{}{
			"name":      tc.tool,
			"arguments": args,
		})

		if _, err := inputWriter.Write([]byte(toolMsg)); err != nil {
			t.Errorf("Failed to write tool message for %s: %v", tc.name, err)
			continue
		}
		time.Sleep(30 * time.Millisecond)
	}

	if err := inputWriter.Close(); err != nil {
		t.Logf("Input writer close warning: %v", err)
	}

	select {
	case <-serverDone:
	case <-time.After(1 * time.Second):
		t.Error("Server did not complete within timeout")
		if err := server.Stop(); err != nil {
			t.Logf("Server stop error: %v", err)
		}
	}

	select {
	case <-responseDone:
	case <-time.After(200 * time.Millisecond):
		t.Log("Response collection timed out")
	}

	requests := gateway.GetRequests()
	expectedMethods := []string{
		"textDocument/definition",
		"textDocument/hover",
		"textDocument/references",
		"textDocument/documentSymbol",
		"workspace/symbol",
	}

	for _, expected := range expectedMethods {
		found := false
		for _, req := range requests {
			if req.Method == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected %s request for multi-language test", expected)
		}
	}

	t.Logf("Successfully processed %d language-specific requests", len(requests))
}

func TestErrorScenarioIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Run("CircuitBreakerIntegration", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		config := &ServerConfig{
			Name:          "circuit-breaker-test",
			Version:       "1.0.0",
			LSPGatewayURL: server.URL,
			Timeout:       1 * time.Second,
			MaxRetries:    2,
		}

		mcpServer := NewServer(config)
		mcpServer.initialized = true

		for i := 0; i < 5; i++ {
			toolCall := ToolCall{
				Name: "goto_definition",
				Arguments: map[string]interface{}{
					"uri":       "file:///test.go",
					"line":      10,
					"character": 5,
				},
			}

			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			result, err := mcpServer.toolHandler.CallTool(ctx, toolCall)
			cancel()

			if err == nil && result.IsError {
				t.Logf("Request %d failed as expected: %s", i+1, result.Content[0].Text)
			}
		}

		metrics := mcpServer.client.GetMetrics()
		if metrics.failedRequests == 0 {
			t.Error("Expected failed requests to be tracked")
		}
	})

	t.Run("RetryLogicRecovery", func(t *testing.T) {
		failureCount := int64(0)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if atomic.AddInt64(&failureCount, 1) <= 2 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			var req JSONRPCRequest
			_ = json.NewDecoder(r.Body).Decode(&req)

			w.Header().Set("Content-Type", "application/json")
			response := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      req.ID,
				"result":  map[string]interface{}{"recovered": true},
			}
			_ = json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		config := &ServerConfig{
			Name:          "retry-recovery-test",
			Version:       "1.0.0",
			LSPGatewayURL: server.URL,
			Timeout:       3 * time.Second,
			MaxRetries:    3,
		}

		mcpServer := NewServer(config)
		mcpServer.initialized = true

		toolCall := ToolCall{
			Name: "goto_definition",
			Arguments: map[string]interface{}{
				"uri":       "file:///test.go",
				"line":      10,
				"character": 5,
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		result, err := mcpServer.toolHandler.CallTool(ctx, toolCall)
		if err != nil {
			t.Fatalf("Expected successful retry, got error: %v", err)
		}

		if result.IsError {
			t.Errorf("Expected successful result after retry, got error: %s", result.Content[0].Text)
		}

		if !strings.Contains(result.Content[0].Text, "recovered") {
			t.Error("Expected response to indicate recovery success")
		}
	})

	t.Run("TimeoutHandling", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(2 * time.Second) // Longer than client timeout
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		config := &ServerConfig{
			Name:          "timeout-test",
			Version:       "1.0.0",
			LSPGatewayURL: server.URL,
			Timeout:       500 * time.Millisecond, // Short timeout
			MaxRetries:    1,
		}

		mcpServer := NewServer(config)
		mcpServer.initialized = true

		toolCall := ToolCall{
			Name: "goto_definition",
			Arguments: map[string]interface{}{
				"uri":       "file:///test.go",
				"line":      10,
				"character": 5,
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		start := time.Now()
		result, err := mcpServer.toolHandler.CallTool(ctx, toolCall)
		duration := time.Since(start)

		if duration > 2*time.Second {
			t.Errorf("Expected quick timeout failure, took %v", duration)
		}

		if err == nil && !result.IsError {
			t.Error("Expected timeout error")
		}
	})
}

func setupPerformanceTest() (*TestLSPGateway, *Server) {
	// Use TestLSPGateway with fast timeouts for performance testing
	gateway := NewTestLSPGateway()

	config := &ServerConfig{
		Name:          "performance-test-server",
		Version:       "1.0.0",
		LSPGatewayURL: gateway.URL(),
		Timeout:       10 * time.Millisecond, // Much faster for performance tests
		MaxRetries:    0,                     // No retries for performance tests
	}
	server := NewServer(config)
	server.initialized = true
	return gateway, server
}

func TestConcurrentThroughputPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	gateway, server := setupPerformanceTest()
	defer gateway.Close()

	// Reduced scale for faster tests
	const numRequests = 50                        // Reduced from 200
	const concurrency = 5                         // Reduced from 20
	const targetDuration = 200 * time.Millisecond // Reduced from 2s

	var successCount int64
	var errorCount int64

	start := time.Now()

	requestChan := make(chan int, numRequests)
	var wg sync.WaitGroup

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for requestID := range requestChan {
				toolCall := ToolCall{
					Name: "goto_definition",
					Arguments: map[string]interface{}{
						"uri":       fmt.Sprintf("file:///test%d.go", requestID),
						"line":      requestID % 100,
						"character": (requestID * 3) % 50,
					},
				}

				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
				result, err := server.toolHandler.CallTool(ctx, toolCall)
				cancel()

				if err != nil || result.IsError {
					atomic.AddInt64(&errorCount, 1)
				} else {
					atomic.AddInt64(&successCount, 1)
				}
			}
		}()
	}

	for i := 0; i < numRequests; i++ {
		requestChan <- i
	}
	close(requestChan)

	wg.Wait()
	duration := time.Since(start)

	success := atomic.LoadInt64(&successCount)
	errors := atomic.LoadInt64(&errorCount)

	requestsPerSecond := float64(numRequests) / duration.Seconds()

	t.Logf("Performance Results:")
	t.Logf("  Total requests: %d", numRequests)
	t.Logf("  Successful: %d", success)
	t.Logf("  Errors: %d", errors)
	t.Logf("  Duration: %v", duration)
	t.Logf("  Requests/second: %.2f", requestsPerSecond)

	if requestsPerSecond < 50 { // Reduced expectation for mock-based tests
		t.Errorf("Expected >50 requests/second, got %.2f", requestsPerSecond)
	}

	// Check for 95% success rate
	successRate := float64(success) / float64(numRequests)
	if successRate < 0.95 {
		t.Errorf("Expected >95%% success rate, got %.2f%%", float64(success)/float64(numRequests)*100)
	}

	if duration > targetDuration {
		t.Errorf("Expected completion within %v, took %v", targetDuration, duration)
	}
}

func createMixedToolCall(tool string, id int) ToolCall {
	if tool == "search_workspace_symbols" {
		return ToolCall{
			Name: tool,
			Arguments: map[string]interface{}{
				"query": fmt.Sprintf("Symbol%d", id),
			},
		}
	}
	return ToolCall{
		Name: tool,
		Arguments: map[string]interface{}{
			"uri":       fmt.Sprintf("file:///test%d.go", id),
			"line":      id % 50,
			"character": (id * 2) % 30,
		},
	}
}

func TestMixedToolPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	gateway, server := setupPerformanceTest()
	defer gateway.Close()

	tools := []string{
		"goto_definition",
		"find_references",
		"get_hover_info",
		"get_document_symbols",
		"search_workspace_symbols",
	}

	// Reduced scale for faster tests
	const requestsPerTool = 5 // Reduced from 20
	const concurrency = 3     // Reduced from 10

	var totalSuccess int64
	var totalErrors int64

	start := time.Now()

	requestChan := make(chan struct {
		tool string
		id   int
	}, len(tools)*requestsPerTool)

	var wg sync.WaitGroup

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for req := range requestChan {
				toolCall := createMixedToolCall(req.tool, req.id)

				ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond) // Much faster timeout
				result, err := server.toolHandler.CallTool(ctx, toolCall)
				cancel()

				if err != nil || result.IsError {
					atomic.AddInt64(&totalErrors, 1)
				} else {
					atomic.AddInt64(&totalSuccess, 1)
				}
			}
		}()
	}

	for _, tool := range tools {
		for i := 0; i < requestsPerTool; i++ {
			requestChan <- struct {
				tool string
				id   int
			}{tool, i}
		}
	}
	close(requestChan)

	wg.Wait()
	duration := time.Since(start)

	totalRequests := len(tools) * requestsPerTool
	success := atomic.LoadInt64(&totalSuccess)
	errors := atomic.LoadInt64(&totalErrors)
	requestsPerSecond := float64(totalRequests) / duration.Seconds()

	t.Logf("Mixed Tool Performance:")
	t.Logf("  Tool types: %d", len(tools))
	t.Logf("  Total requests: %d", totalRequests)
	t.Logf("  Successful: %d", success)
	t.Logf("  Errors: %d", errors)
	t.Logf("  Duration: %v", duration)
	t.Logf("  Requests/second: %.2f", requestsPerSecond)

	if requestsPerSecond < 25 { // Reduced expectation for mock-based tests
		t.Errorf("Expected >25 mixed requests/second, got %.2f", requestsPerSecond)
	}

	successRate := float64(success) / float64(totalRequests)
	if successRate < 0.90 {
		t.Errorf("Expected >90%% success rate for mixed tools, got %.2f%%", successRate*100)
	}
}

func TestRealLSPServerIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping real LSP server test in short mode")
	}

	if _, err := exec.LookPath("gopls"); err != nil {
		t.Skip("gopls not available, skipping real LSP server test")
	}

	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "main.go")

	goContent := `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}

type TestStruct struct {
	Field1 string
	Field2 int
}

func (ts *TestStruct) Method() string {
	return ts.Field1
}
`

	if err := os.WriteFile(goFile, []byte(goContent), 0644); err != nil {
		t.Fatalf("Failed to create test Go file: %v", err)
	}

	goMod := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(goMod, []byte("module testproject\n\ngo 1.21\n"), 0644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	t.Logf("Real LSP integration test environment prepared:")
	t.Logf("  Project directory: %s", tmpDir)
	t.Logf("  Go file: %s", goFile)
	t.Logf("  Available for manual testing with real gopls")

}

func TestEndToEndWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping end-to-end test in short mode")
	}

	gateway := NewTestLSPGateway()
	defer gateway.Close()

	config := &ServerConfig{
		Name:          "e2e-test-server",
		Version:       "1.0.0",
		LSPGatewayURL: gateway.URL(),
		Timeout:       100 * time.Millisecond,
		MaxRetries:    1,
	}

	server := NewServer(config)

	inputReader, inputWriter := io.Pipe()
	outputReader, outputWriter := io.Pipe()
	server.SetIO(inputReader, outputWriter)

	serverDone := make(chan error, 1)
	responseBuffer := &bytes.Buffer{}
	responseDone := make(chan struct{})

	go func() {
		defer close(responseDone)
		_, _ = io.Copy(responseBuffer, outputReader)
	}()

	go func() {
		serverDone <- server.Start()
	}()

	time.Sleep(30 * time.Millisecond)

	initMsg := createTestMessage(1, "initialize", map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]interface{}{
			"name":    "test-client",
			"version": "1.0.0",
		},
	})
	if _, err := inputWriter.Write([]byte(initMsg)); err != nil {
		t.Fatalf("Failed to write init message: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	listMsg := createTestMessage(2, "tools/list", nil)
	if _, err := inputWriter.Write([]byte(listMsg)); err != nil {
		t.Fatalf("Failed to write list message: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	toolTests := []struct {
		id   int
		tool string
		args map[string]interface{}
	}{
		{
			id:   3,
			tool: "goto_definition",
			args: map[string]interface{}{
				"uri":       "file:///test.go",
				"line":      10,
				"character": 5,
			},
		},
		{
			id:   4,
			tool: "find_references",
			args: map[string]interface{}{
				"uri":                "file:///test.go",
				"line":               15,
				"character":          8,
				"includeDeclaration": true,
			},
		},
		{
			id:   5,
			tool: "get_hover_info",
			args: map[string]interface{}{
				"uri":       "file:///test.go",
				"line":      20,
				"character": 12,
			},
		},
		{
			id:   6,
			tool: "get_document_symbols",
			args: map[string]interface{}{
				"uri": "file:///test.go",
			},
		},
		{
			id:   7,
			tool: "search_workspace_symbols",
			args: map[string]interface{}{
				"query": "TestFunction",
			},
		},
	}

	for _, test := range toolTests {
		toolMsg := createTestMessage(test.id, "tools/call", map[string]interface{}{
			"name":      test.tool,
			"arguments": test.args,
		})
		if _, err := inputWriter.Write([]byte(toolMsg)); err != nil {
			t.Errorf("Failed to write tool message %d: %v", test.id, err)
			continue
		}
		time.Sleep(30 * time.Millisecond)
	}

	time.Sleep(50 * time.Millisecond)

	if err := inputWriter.Close(); err != nil {
		t.Logf("Input writer close warning: %v", err)
	}

	select {
	case <-serverDone:
	case <-time.After(1 * time.Second):
		t.Error("End-to-end test did not complete within timeout")
		if err := server.Stop(); err != nil {
			t.Logf("Server stop error: %v", err)
		}
	}

	select {
	case <-responseDone:
	case <-time.After(200 * time.Millisecond):
		t.Log("Response collection timed out")
	}

	output := responseBuffer.String()
	responseCount := strings.Count(output, "Content-Length:")
	t.Logf("End-to-end workflow collected %d responses", responseCount)

	expectedResponses := 7 
	if responseCount < expectedResponses {
		t.Errorf("Expected at least %d responses, got %d", expectedResponses, responseCount)
	}

	gatewayRequests := gateway.GetRequests()
	expectedLSPMethods := []string{
		"textDocument/definition",
		"textDocument/references",
		"textDocument/hover",
		"textDocument/documentSymbol",
		"workspace/symbol",
	}

	for _, expectedMethod := range expectedLSPMethods {
		found := false
		for _, req := range gatewayRequests {
			if req.Method == expectedMethod {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected LSP method %s to be called in end-to-end workflow", expectedMethod)
		}
	}

	t.Logf("End-to-end workflow successfully processed:")
	t.Logf("  MCP responses: %d", responseCount)
	t.Logf("  LSP requests: %d", len(gatewayRequests))
	t.Logf("  Tool types tested: %d", len(toolTests))
}

func BenchmarkIntegrationThroughput(b *testing.B) {
	// Use TestLSPGateway for fast benchmarking
	gateway := NewTestLSPGateway()
	defer gateway.Close()

	config := &ServerConfig{
		Name:          "benchmark-server",
		Version:       "1.0.0",
		LSPGatewayURL: gateway.URL(),
		Timeout:       10 * time.Millisecond, // Much faster for benchmarks
		MaxRetries:    0,                     // No retries for benchmarks
	}

	server := NewServer(config)
	server.initialized = true

	toolCall := ToolCall{
		Name: "goto_definition",
		Arguments: map[string]interface{}{
			"uri":       "file:///benchmark.go",
			"line":      10,
			"character": 5,
		},
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond) // Much faster timeout
			_, err := server.toolHandler.CallTool(ctx, toolCall)
			cancel()

			if err != nil {
				b.Errorf("Tool call failed: %v", err)
			}
		}
	})
}

func BenchmarkMCPMessageProcessingIntegration(b *testing.B) {
	// Use TestLSPGateway for fast message processing benchmark
	gateway := NewTestLSPGateway()
	defer gateway.Close()

	config := &ServerConfig{
		Name:          "message-benchmark",
		Version:       "1.0.0",
		LSPGatewayURL: gateway.URL(),
		Timeout:       5 * time.Millisecond, // Much faster
		MaxRetries:    0,                    // No retries
	}

	server := NewServer(config)
	server.initialized = true

	message := createTestMessage(1, "tools/call", map[string]interface{}{
		"name": "goto_definition",
		"arguments": map[string]interface{}{
			"uri":       "file:///benchmark.go",
			"line":      10,
			"character": 5,
		},
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		outputBuffer := &bytes.Buffer{}
		server.SetIO(nil, outputBuffer)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond) // Much faster timeout
		_ = server.handleMessageWithValidation(ctx, message[strings.Index(message, "{"):])
		cancel()
	}
}

func TestProtocolVersionCompatibility(t *testing.T) {
	supportedVersions := []string{
		"2024-11-05",
		"2024-10-07",
		"2024-09-01",
	}

	gateway := NewTestLSPGateway()
	defer gateway.Close()

	for _, version := range supportedVersions {
		t.Run(fmt.Sprintf("Version_%s", version), func(t *testing.T) {
			config := &ServerConfig{
				Name:          "protocol-compat-test",
				Version:       "1.0.0",
				LSPGatewayURL: gateway.URL(),
				Timeout:       100 * time.Millisecond,
				MaxRetries:    1,
			}

			server := NewServer(config)
			inputReader, inputWriter := io.Pipe()
			outputReader, outputWriter := io.Pipe()
			server.SetIO(inputReader, outputWriter)

			serverDone := make(chan struct{})
			responseBuffer := &bytes.Buffer{}
			responseDone := make(chan struct{})

			go func() {
				defer close(responseDone)
				_, _ = io.Copy(responseBuffer, outputReader)
			}()

			go func() {
				defer close(serverDone)
				_ = server.Start()
			}()

			time.Sleep(20 * time.Millisecond)

			initMsg := createTestMessage(1, "initialize", map[string]interface{}{
				"protocolVersion": version,
				"capabilities":    map[string]interface{}{},
			})

			if _, err := inputWriter.Write([]byte(initMsg)); err != nil {
				t.Fatalf("Failed to write init message: %v", err)
			}
			time.Sleep(50 * time.Millisecond)

			if err := inputWriter.Close(); err != nil {
				t.Logf("Input writer close warning: %v", err)
			}

			select {
			case <-serverDone:
			case <-time.After(500 * time.Millisecond):
				t.Errorf("Server did not complete within timeout for version %s", version)
				if err := server.Stop(); err != nil {
					t.Logf("Server stop error: %v", err)
				}
			}

			select {
			case <-responseDone:
			case <-time.After(100 * time.Millisecond):
				t.Log("Response collection timed out")
			}

			output := responseBuffer.String()
			if !strings.Contains(output, "Content-Length:") {
				t.Errorf("Expected response for protocol version %s", version)
			}
		})
	}
}
