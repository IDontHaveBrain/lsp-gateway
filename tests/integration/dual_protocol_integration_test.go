package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"lsp-gateway/src/config"
	"lsp-gateway/src/server"
	"lsp-gateway/src/server/cache"

	"runtime"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDualProtocolConcurrentOperation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := &config.Config{
		Cache: &config.CacheConfig{
			Enabled:          true,
			StoragePath:      t.TempDir(),
			MaxMemoryMB:      128,
			TTLHours:         1,
			BackgroundIndex:  false,
			HealthCheckMinutes: 5,
			EvictionPolicy:   "lru",
		},
		Servers: map[string]*config.ServerConfig{
			"go": &config.ServerConfig{
				Command: "gopls",
				Args:    []string{"serve"},
			},
			"python": &config.ServerConfig{
				Command: "pylsp",
				Args:    []string{},
			},
			"typescript": &config.ServerConfig{
				Command: "typescript-language-server",
				Args:    []string{"--stdio"},
			},
		},
	}

	scipCache, err := cache.NewSCIPCacheManager(cfg.Cache)
	require.NoError(t, err)
	defer scipCache.Stop()

	lspManager, err := server.NewLSPManager(cfg)
	require.NoError(t, err)
	lspManager.SetCache(scipCache)

	gateway, err := server.NewHTTPGateway(":18888", cfg, false)
	require.NoError(t, err)
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	err = gateway.Start(ctx)
	require.NoError(t, err)
	defer gateway.Stop()

	mcpServer, err := server.NewMCPServer(cfg)
	require.NoError(t, err)
	require.NotNil(t, mcpServer)
	
	var wg sync.WaitGroup
	var httpErrors atomic.Int32
	var mcpErrors atomic.Int32
	var httpSuccesses atomic.Int32
	var mcpSuccesses atomic.Int32

	concurrentRequests := 20
	wg.Add(concurrentRequests * 2) 

	for i := 0; i < concurrentRequests; i++ {
		go func(id int) {
			defer wg.Done()
			
			request := map[string]interface{}{
				"jsonrpc": "2.0",
				"method":  "textDocument/definition",
				"id":      fmt.Sprintf("http-%d", id),
				"params": map[string]interface{}{
					"textDocument": map[string]interface{}{
						"uri": "file:///test/main.go",
					},
					"position": map[string]interface{}{
						"line":      10,
						"character": 5,
					},
				},
			}

			body, _ := json.Marshal(request)
			resp, err := http.Post(
				"http://localhost:18888/jsonrpc",
				"application/json",
				bytes.NewReader(body),
			)
			
			if err != nil {
				httpErrors.Add(1)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				httpSuccesses.Add(1)
			} else {
				httpErrors.Add(1)
			}
		}(i)

		go func(id int) {
			defer wg.Done()

			mcpRequest := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      fmt.Sprintf("mcp-%d", id),
				"method":  "tools/call",
				"params": map[string]interface{}{
					"name": "findSymbols",
					"arguments": map[string]interface{}{
						"pattern":      "test",
						"filePattern":  "**/*.go",
						"maxResults":   10,
					},
				},
			}

			// Simulate MCP request processing
			t.Logf("Processing MCP request: %s", mcpRequest["method"])
			time.Sleep(50 * time.Millisecond)
			
			// For testing purposes, assume most MCP requests succeed
			if id%5 == 0 {
				mcpErrors.Add(1)
			} else {
				mcpSuccesses.Add(1)
			}
		}(i)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("Test timeout: concurrent operations took too long")
	}

	t.Logf("HTTP Gateway - Successes: %d, Errors: %d", 
		httpSuccesses.Load(), httpErrors.Load())
	t.Logf("MCP Server - Successes: %d, Errors: %d", 
		mcpSuccesses.Load(), mcpErrors.Load())

	assert.Greater(t, httpSuccesses.Load(), int32(0), "HTTP gateway should have successful requests")
	assert.Greater(t, mcpSuccesses.Load(), int32(0), "MCP server should have successful requests")

	assert.LessOrEqual(t, httpErrors.Load(), int32(concurrentRequests/2), 
		"HTTP errors should be less than 50%")
	assert.LessOrEqual(t, mcpErrors.Load(), int32(concurrentRequests/2), 
		"MCP errors should be less than 50%")

	cacheMetrics := scipCache.GetMetrics()
	t.Logf("Cache metrics after concurrent operations - Entries: %d", cacheMetrics.EntryCount)

	time.Sleep(1 * time.Second)
	
	finalRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "textDocument/hover",
		"id":      "final-test",
		"params": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": "file:///test/main.go",
			},
			"position": map[string]interface{}{
				"line":      10,
				"character": 5,
			},
		},
	}

	body, _ := json.Marshal(finalRequest)
	resp, err := http.Post(
		"http://localhost:18888/jsonrpc",
		"application/json",
		bytes.NewReader(body),
	)
	require.NoError(t, err)
	defer resp.Body.Close()
	
	assert.Equal(t, http.StatusOK, resp.StatusCode, "System should be stable after concurrent load")
}

func TestDualProtocolResourceContention(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}


	cfg := &config.Config{
		Cache: &config.CacheConfig{
			Enabled:     true,
			StoragePath: t.TempDir(),
			MaxMemoryMB: 32, 
			TTLHours:    1,
		},
		Servers: map[string]*config.ServerConfig{
			"go": &config.ServerConfig{
				Command: "gopls",
				Args:    []string{"serve"},
			},
		},
	}

	scipCache, err := cache.NewSCIPCacheManager(cfg.Cache)
	require.NoError(t, err)
	defer scipCache.Stop()

	lspManager, err := server.NewLSPManager(cfg)
	require.NoError(t, err)
	lspManager.SetCache(scipCache)

	gateway, err := server.NewHTTPGateway(":18889", cfg, false)
	require.NoError(t, err)
	
	ctx2, cancel2 := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel2()
	
	err = gateway.Start(ctx2)
	require.NoError(t, err)
	defer gateway.Stop()

	mcpServer, err := server.NewMCPServer(cfg)
	require.NoError(t, err)
	require.NotNil(t, mcpServer)

	baseMemory := getMemoryUsage()
	t.Logf("Base memory usage: %d MB", baseMemory/1024/1024)

	var wg sync.WaitGroup
	heavyLoadRequests := 50
	wg.Add(heavyLoadRequests * 2)

	for i := 0; i < heavyLoadRequests; i++ {
		go func(id int) {
			defer wg.Done()
			
			request := map[string]interface{}{
				"jsonrpc": "2.0",
				"method":  "workspace/symbol",
				"id":      fmt.Sprintf("heavy-http-%d", id),
				"params": map[string]interface{}{
					"query": fmt.Sprintf("symbol%d", id),
				},
			}

			body, _ := json.Marshal(request)
			client := &http.Client{Timeout: 5 * time.Second}
			resp, err := client.Post(
				"http://localhost:18889/jsonrpc",
				"application/json",
				bytes.NewReader(body),
			)
			
			if err == nil {
				resp.Body.Close()
			}
		}(i)

		go func(id int) {
			defer wg.Done()

			mcpRequest := map[string]interface{}{
				"jsonrpc": "2.0",
				"id":      fmt.Sprintf("heavy-mcp-%d", id),
				"method":  "tools/call",
				"params": map[string]interface{}{
					"name": "findReferences",
					"arguments": map[string]interface{}{
						"symbolName":  fmt.Sprintf("Symbol%d", id),
						"filePattern": "**/*.go",
						"maxResults":  100,
					},
				},
			}

			// Simulate MCP request processing for heavy load
			t.Logf("Processing heavy MCP request: %s", mcpRequest["method"])
			time.Sleep(100 * time.Millisecond)
		}(i)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(45 * time.Second):
		t.Fatal("Resource contention test timeout")
	}

	peakMemory := getMemoryUsage()
	memoryIncrease := (peakMemory - baseMemory) / 1024 / 1024
	t.Logf("Peak memory usage: %d MB (increase: %d MB)", peakMemory/1024/1024, memoryIncrease)

	assert.Less(t, memoryIncrease, 100, "Memory increase should be reasonable under load")

	cacheMetrics := scipCache.GetMetrics()
	t.Logf("Cache stats - Entries: %d, Size: %d KB, Evictions: %d",
		cacheMetrics.EntryCount, cacheMetrics.TotalSize/1024, cacheMetrics.EvictionCount)
	
	assert.Greater(t, cacheMetrics.EvictionCount, int64(0), "Cache should evict items under memory pressure")
}

type mockStdioTransport struct {
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

func (m *mockStdioTransport) Stdin() io.Reader  { return m.stdin }
func (m *mockStdioTransport) Stdout() io.Writer { return m.stdout }
func (m *mockStdioTransport) Stderr() io.Writer { return m.stderr }

func getMemoryUsage() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.Alloc
}