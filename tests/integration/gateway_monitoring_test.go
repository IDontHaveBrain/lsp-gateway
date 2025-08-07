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

	time.Sleep(100 * time.Millisecond)
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
					"uri": fmt.Sprintf("file://%s", testFile),
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

		require.Contains(t, stats, "totalEntries")
		require.Contains(t, stats, "totalSize")
		require.Contains(t, stats, "hits")
		require.Contains(t, stats, "misses")
		require.Contains(t, stats, "evictions")
		require.Contains(t, stats, "hitRate")

		totalEntries, ok := stats["totalEntries"].(float64)
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
		require.Contains(t, health, "memoryUsage")
		require.Contains(t, health, "uptime")
		require.Contains(t, health, "lastHealthCheck")

		status, ok := health["status"].(string)
		require.True(t, ok)
		require.Equal(t, "healthy", status)

		memoryUsage, ok := health["memoryUsage"].(map[string]interface{})
		require.True(t, ok)
		require.Contains(t, memoryUsage, "used")
		require.Contains(t, memoryUsage, "limit")
		require.Contains(t, memoryUsage, "percentage")
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
		require.Equal(t, "Cache cleared successfully", message)

		// Get stats after clearing
		respAfter, err := client.Get("http://localhost:18080/cache/stats")
		require.NoError(t, err)
		defer respAfter.Body.Close()

		var statsAfter map[string]interface{}
		err = json.NewDecoder(respAfter.Body).Decode(&statsAfter)
		require.NoError(t, err)

		totalEntriesAfter, ok := statsAfter["totalEntries"].(float64)
		require.True(t, ok)
		require.Equal(t, float64(0), totalEntriesAfter)
	})

	t.Run("CacheEndpointWithoutCache", func(t *testing.T) {
		// Create a config without cache
		configNoCache := &config.Config{
			Cache: &config.CacheConfig{
				Enabled: false,
			},
			Servers: gatewayConfig.Servers,
		}

		gatewayWithoutCache, err := server.NewHTTPGateway(":18081", configNoCache, true)
		require.NoError(t, err)

		go func() {
			err := gatewayWithoutCache.Start(context.Background())
			if err != nil && err != http.ErrServerClosed {
				t.Errorf("Gateway start error: %v", err)
			}
		}()

		time.Sleep(100 * time.Millisecond)
		defer gatewayWithoutCache.Stop()

		resp, err := client.Get("http://localhost:18081/cache/stats")
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)

		var errorResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&errorResp)
		require.NoError(t, err)

		require.Contains(t, errorResp, "error")
		errorMsg, ok := errorResp["error"].(string)
		require.True(t, ok)
		require.Equal(t, "Cache not available", errorMsg)
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
						"uri": fmt.Sprintf("file://%s", testFile),
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

		memoryUsage, ok := health["memoryUsage"].(map[string]interface{})
		require.True(t, ok)

		percentage, ok := memoryUsage["percentage"].(float64)
		require.True(t, ok)
		require.Less(t, percentage, float64(100))
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
						"uri": fmt.Sprintf("file://%s", testFile),
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

		hits, ok := stats["hits"].(float64)
		require.True(t, ok)
		require.GreaterOrEqual(t, hits, float64(5))

		misses, ok := stats["misses"].(float64)
		require.True(t, ok)
		require.GreaterOrEqual(t, misses, float64(3))

		hitRate, ok := stats["hitRate"].(float64)
		require.True(t, ok)
		require.Greater(t, hitRate, float64(0))
		require.LessOrEqual(t, hitRate, float64(100))
	})

	t.Run("JSONRPCWithMonitoring", func(t *testing.T) {
		jsonReq := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  "textDocument/hover",
			"params": map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": fmt.Sprintf("file://%s", testFile),
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

		totalEntries, ok := stats["totalEntries"].(float64)
		require.True(t, ok)
		require.Greater(t, totalEntries, float64(0))
	})
}
