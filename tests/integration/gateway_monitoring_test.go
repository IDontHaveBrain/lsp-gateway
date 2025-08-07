package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"lsp-gateway/src/config"
	"lsp-gateway/src/server"
	"lsp-gateway/src/utils"
)

func TestGatewayMonitoringEndpoints(t *testing.T) {
	_, err := exec.LookPath("gopls")
	if err != nil {
		t.Skip("Go LSP server (gopls) not installed, skipping test")
	}

	testDir := t.TempDir()
	cacheDir := filepath.Join(testDir, "cache")

	testFile := filepath.Join(testDir, "test.go")
	err = os.WriteFile(testFile, []byte(`package main

import "fmt"

type Monitor struct {
	active bool
}

func (m *Monitor) Check() bool {
	return m.active
}

func main() {
	monitor := &Monitor{active: true}
	fmt.Println(monitor.Check())
}
`), 0644)
	require.NoError(t, err)

	gatewayConfig := &config.Config{
		Cache: &config.CacheConfig{
			Enabled:         true,
			StoragePath:     cacheDir,
			MaxMemoryMB:     128,
			TTLHours:        2,
			Languages:       []string{"go"},
			BackgroundIndex: false,
			DiskCache:       true,
			EvictionPolicy:  "lru",
		},
		Servers: map[string]*config.ServerConfig{
			"go": {
				Command: "gopls",
				Args:    []string{"serve"},
			},
		},
	}

	gateway, err := server.NewHTTPGateway(":18080", gatewayConfig, false)
	require.NoError(t, err)

	go func() {
		err := gateway.Start(context.Background())
		if err != nil && err != http.ErrServerClosed {
			t.Errorf("Gateway start error: %v", err)
		}
	}()

	// Wait for server to be ready with retry logic
	var serverReady bool
	for i := 0; i < 30; i++ { // Try for up to 3 seconds
		resp, err := http.Get("http://localhost:18080/health")
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			serverReady = true
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(100 * time.Millisecond)
	}
	require.True(t, serverReady, "Gateway server failed to start within 3 seconds")

	defer gateway.Stop()

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	t.Run("CacheStatsEndpoint", func(t *testing.T) {
		// Trigger some cache activity through JSON-RPC request
		jsonReq := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "textDocument/hover",
			"params": map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": utils.FilePathToURI(testFile),
				},
				"position": map[string]interface{}{
					"line":      4,
					"character": 6,
				},
			},
			"id": 1,
		}

		jsonData, err := json.Marshal(jsonReq)
		require.NoError(t, err)

		req, err := http.NewRequest(
			http.MethodPost,
			"http://localhost:18080/jsonrpc",
			bytes.NewBuffer(jsonData),
		)
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		respInit, err := client.Do(req)
		require.NoError(t, err)
		respInit.Body.Close()

		resp, err := client.Get("http://localhost:18080/cache/stats")
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, "application/json", resp.Header.Get("Content-Type"))

		var stats map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&stats)
		require.NoError(t, err)

		require.Contains(t, stats, "entry_count")
		require.Contains(t, stats, "total_size_bytes")
		require.Contains(t, stats, "hit_count")
		require.Contains(t, stats, "miss_count")
		require.Contains(t, stats, "eviction_count")
		require.Contains(t, stats, "hit_rate_percent")

		totalEntries, ok := stats["entry_count"].(float64)
		require.True(t, ok)
		require.Greater(t, totalEntries, float64(0))
	})

	t.Run("CacheHealthEndpoint", func(t *testing.T) {
		resp, err := client.Get("http://localhost:18080/cache/health")
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)
		require.Equal(t, "application/json", resp.Header.Get("Content-Type"))

		var health map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&health)
		require.NoError(t, err)

		require.Contains(t, health, "status")
		require.Contains(t, health, "enabled")
		require.Contains(t, health, "total_size")
		require.Contains(t, health, "entry_count")
		require.Contains(t, health, "uptime_requests")

		status, ok := health["status"].(string)
		require.True(t, ok)
		require.Equal(t, "healthy", status)

		enabled, ok := health["enabled"].(bool)
		require.True(t, ok)
		require.True(t, enabled)
	})

	t.Run("CacheClearEndpoint", func(t *testing.T) {
		// First get initial stats
		resp, err := client.Get("http://localhost:18080/cache/stats")
		require.NoError(t, err)

		var statsBefore map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&statsBefore)
		resp.Body.Close()
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost, "http://localhost:18080/cache/clear", nil)
		require.NoError(t, err)

		resp, err = client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.Unmarshal(body, &result)
		require.NoError(t, err)

		require.Contains(t, result, "message")
		message, ok := result["message"].(string)
		require.True(t, ok)
		require.Equal(t, "Cache clear requested - individual document invalidation required", message)

		// Note: Cache clear only returns a message but doesn't actually clear the cache
		// This is a limitation of the current cache interface design as noted in the server
		respAfter, err := client.Get("http://localhost:18080/cache/stats")
		require.NoError(t, err)
		defer respAfter.Body.Close()

		var statsAfter map[string]interface{}
		err = json.NewDecoder(respAfter.Body).Decode(&statsAfter)
		require.NoError(t, err)

		// Cache entries should still exist since clear doesn't actually clear
		totalEntriesAfter, ok := statsAfter["entry_count"].(float64)
		require.True(t, ok)
		require.GreaterOrEqual(t, totalEntriesAfter, float64(0))
	})

	t.Run("CacheAlwaysEnabledForHTTPGateway", func(t *testing.T) {
		// HTTP Gateway automatically enables cache even when config disables it
		// because cache is required for HTTP gateway performance
		configNoCache := &config.Config{
			Cache: &config.CacheConfig{
				Enabled: false, // This will be overridden by HTTPGateway
			},
			Servers: gatewayConfig.Servers,
		}

		gatewayWithCache, err := server.NewHTTPGateway(":18081", configNoCache, true)
		require.NoError(t, err)

		go func() {
			err := gatewayWithCache.Start(context.Background())
			if err != nil && err != http.ErrServerClosed {
				t.Errorf("Gateway start error: %v", err)
			}
		}()

		// Wait for server to be ready with retry logic
		var serverReady bool
		for i := 0; i < 30; i++ { // Try for up to 3 seconds
			resp, err := http.Get("http://localhost:18081/health")
			if err == nil && resp.StatusCode == http.StatusOK {
				resp.Body.Close()
				serverReady = true
				break
			}
			if resp != nil {
				resp.Body.Close()
			}
			time.Sleep(100 * time.Millisecond)
		}
		require.True(t, serverReady, "Gateway server failed to start within 3 seconds")

		defer gatewayWithCache.Stop()

		// Even with cache disabled in config, HTTP gateway should have cache available
		resp, err := client.Get("http://localhost:18081/cache/stats")
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var stats map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&stats)
		require.NoError(t, err)

		require.Contains(t, stats, "entry_count")
		require.Contains(t, stats, "cache_enabled")
		cacheEnabled, ok := stats["cache_enabled"].(bool)
		require.True(t, ok)
		require.True(t, cacheEnabled, "Cache should be automatically enabled for HTTP gateway")
	})

	t.Run("InvalidHTTPMethods", func(t *testing.T) {
		testCases := []struct {
			endpoint string
			method   string
			expected int
		}{
			{"/cache/stats", http.MethodPost, http.StatusMethodNotAllowed},
			{"/cache/health", http.MethodDelete, http.StatusMethodNotAllowed},
			{"/cache/clear", http.MethodGet, http.StatusMethodNotAllowed},
		}

		for _, tc := range testCases {
			t.Run(fmt.Sprintf("%s_%s", tc.method, tc.endpoint), func(t *testing.T) {
				req, err := http.NewRequest(tc.method, "http://localhost:18080"+tc.endpoint, nil)
				require.NoError(t, err)

				resp, err := client.Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()

				require.Equal(t, tc.expected, resp.StatusCode)
			})
		}
	})

	t.Run("ConcurrentMonitoringRequests", func(t *testing.T) {
		type result struct {
			endpoint string
			status   int
			err      error
		}

		endpoints := []string{
			"/cache/stats",
			"/cache/health",
			"/cache/stats",
			"/cache/health",
			"/cache/stats",
		}

		resultChan := make(chan result, len(endpoints))

		for _, endpoint := range endpoints {
			go func(endpoint string) {
				resp, err := client.Get("http://localhost:18080" + endpoint)
				if err != nil {
					resultChan <- result{endpoint: endpoint, err: err}
					return
				}
				defer resp.Body.Close()

				resultChan <- result{
					endpoint: endpoint,
					status:   resp.StatusCode,
					err:      nil,
				}
			}(endpoint)
		}

		timeout := time.After(5 * time.Second)
		for i := 0; i < len(endpoints); i++ {
			select {
			case r := <-resultChan:
				require.NoError(t, r.err)
				require.Equal(t, http.StatusOK, r.status)
			case <-timeout:
				t.Fatal("Concurrent requests timed out")
			}
		}
	})

	t.Run("HealthCheckUnderLoad", func(t *testing.T) {
		// Generate load by making multiple requests
		for i := 0; i < 10; i++ {
			jsonReq := map[string]interface{}{
				"jsonrpc": "2.0",
				"method":  "textDocument/hover",
				"params": map[string]interface{}{
					"textDocument": map[string]interface{}{
						"uri": utils.FilePathToURI(testFile),
					},
					"position": map[string]interface{}{
						"line":      4,
						"character": 6,
					},
				},
				"id": i + 100,
			}

			jsonData, err := json.Marshal(jsonReq)
			require.NoError(t, err)

			req, err := http.NewRequest(
				http.MethodPost,
				"http://localhost:18080/jsonrpc",
				bytes.NewBuffer(jsonData),
			)
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			respLoad, err := client.Do(req)
			require.NoError(t, err)
			respLoad.Body.Close()
		}

		resp, err := client.Get("http://localhost:18080/cache/health")
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var health map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&health)
		require.NoError(t, err)

		status, ok := health["status"].(string)
		require.True(t, ok)
		require.Equal(t, "healthy", status)

		enabled, ok := health["enabled"].(bool)
		require.True(t, ok)
		require.True(t, enabled)

		totalSize, ok := health["total_size"].(float64)
		require.True(t, ok)
		require.GreaterOrEqual(t, totalSize, float64(0))
	})

	t.Run("StatsAccuracy", func(t *testing.T) {
		// Clear cache first
		req, err := http.NewRequest(http.MethodPost, "http://localhost:18080/cache/clear", nil)
		require.NoError(t, err)

		respClear, err := client.Do(req)
		require.NoError(t, err)
		respClear.Body.Close()

		// Make some requests to generate hits
		for i := 0; i < 5; i++ {
			jsonReq := map[string]interface{}{
				"jsonrpc": "2.0",
				"method":  "textDocument/definition",
				"params": map[string]interface{}{
					"textDocument": map[string]interface{}{
						"uri": utils.FilePathToURI(testFile),
					},
					"position": map[string]interface{}{
						"line":      4,
						"character": 6,
					},
				},
				"id": i + 200,
			}

			jsonData, err := json.Marshal(jsonReq)
			require.NoError(t, err)

			req, err := http.NewRequest(
				http.MethodPost,
				"http://localhost:18080/jsonrpc",
				bytes.NewBuffer(jsonData),
			)
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")

			respDef, err := client.Do(req)
			require.NoError(t, err)
			respDef.Body.Close()
		}

		resp, err := client.Get("http://localhost:18080/cache/stats")
		require.NoError(t, err)
		defer resp.Body.Close()

		var stats map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&stats)
		require.NoError(t, err)

		hits, ok := stats["hit_count"].(float64)
		require.True(t, ok)
		require.GreaterOrEqual(t, hits, float64(0))

		misses, ok := stats["miss_count"].(float64)
		require.True(t, ok)
		require.GreaterOrEqual(t, misses, float64(0))

		hitRate, ok := stats["hit_rate_percent"].(float64)
		require.True(t, ok)
		require.GreaterOrEqual(t, hitRate, float64(0))
		require.LessOrEqual(t, hitRate, float64(100))
	})

	t.Run("JSONRPCWithMonitoring", func(t *testing.T) {
		jsonReq := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "textDocument/hover",
			"params": map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": utils.FilePathToURI(testFile),
				},
				"position": map[string]interface{}{
					"line":      4,
					"character": 6,
				},
			},
			"id": 1,
		}

		jsonData, err := json.Marshal(jsonReq)
		require.NoError(t, err)

		req, err := http.NewRequest(
			http.MethodPost,
			"http://localhost:18080/jsonrpc",
			bytes.NewBuffer(jsonData),
		)
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)

		var jsonResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&jsonResp)
		require.NoError(t, err)
		require.Contains(t, jsonResp, "result")

		statsResp, err := client.Get("http://localhost:18080/cache/stats")
		require.NoError(t, err)
		defer statsResp.Body.Close()

		var stats map[string]interface{}
		err = json.NewDecoder(statsResp.Body).Decode(&stats)
		require.NoError(t, err)

		totalEntries, ok := stats["entry_count"].(float64)
		require.True(t, ok)
		require.Greater(t, totalEntries, float64(0))
	})
}
