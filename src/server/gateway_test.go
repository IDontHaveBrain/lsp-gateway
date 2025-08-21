package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lsp-gateway/src/config"
	"lsp-gateway/src/server/cache"
	"lsp-gateway/src/server/protocol"
)

// Helper functions for creating test configurations
func createTestGatewayConfig() *config.Config {
	return &config.Config{
		Servers: map[string]*config.ServerConfig{
			"go": {
				Command: "gopls",
				Args:    []string{"serve"},
			},
		},
		Cache: &config.CacheConfig{
			Enabled:         true,
			MaxMemoryMB:     256,
			TTLHours:        1,
			StoragePath:     "/tmp/test-cache",
			Languages:       []string{"go"},
			BackgroundIndex: false,
			EvictionPolicy:  "lru",
			DiskCache:       false,
		},
	}
}

// Helper function to create gateway with mock cache
func createGatewayWithMockCache(mockCache *MockSCIPCache, lspOnly bool) *HTTPGateway {
	return &HTTPGateway{
		lspManager:      &LSPManager{scipCache: mockCache},
		cacheConfig:     &config.CacheConfig{Enabled: true},
		lspOnly:         lspOnly,
		responseFactory: protocol.NewResponseFactory(),
	}
}

// Test NewHTTPGateway constructor
func TestNewHTTPGateway(t *testing.T) {
	tests := []struct {
		name        string
		addr        string
		config      *config.Config
		lspOnly     bool
		expectError bool
	}{
		{
			name:        "with valid config",
			addr:        ":8080",
			config:      createTestGatewayConfig(),
			lspOnly:     false,
			expectError: false,
		},
		{
			name:        "with nil config creates default with cache",
			addr:        ":8082",
			config:      nil,
			lspOnly:     false,
			expectError: false,
		},
		{
			name:        "lspOnly mode enabled",
			addr:        ":8083",
			config:      createTestGatewayConfig(),
			lspOnly:     true,
			expectError: false,
		},
		{
			name: "cache disabled config gets enabled",
			addr: ":8084",
			config: &config.Config{
				Servers: map[string]*config.ServerConfig{"go": {Command: "gopls"}},
				Cache:   &config.CacheConfig{Enabled: false},
			},
			lspOnly:     false,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gateway, err := NewHTTPGateway(tt.addr, tt.config, tt.lspOnly)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, gateway)
			} else {
				assert.NoError(t, err)
				if gateway != nil {
					assert.NotNil(t, gateway.lspManager)
					assert.NotNil(t, gateway.cacheConfig)
					assert.True(t, gateway.cacheConfig.Enabled, "HTTP gateway should enable cache")
					assert.Equal(t, tt.lspOnly, gateway.lspOnly)
					assert.NotNil(t, gateway.server)
					assert.NotNil(t, gateway.responseFactory)
					gateway.Stop()
				}
			}
		})
	}
}

// Test basic HTTPGateway properties
func TestHTTPGateway_BasicProperties(t *testing.T) {
	mockCache := NewMockSCIPCache()
	gateway := createGatewayWithMockCache(mockCache, false)

	// Test basic properties
	assert.NotNil(t, gateway.lspManager)
	assert.NotNil(t, gateway.cacheConfig)
	assert.NotNil(t, gateway.responseFactory)
	assert.False(t, gateway.lspOnly)
	assert.True(t, gateway.cacheConfig.Enabled)
}

func TestHTTPGateway_LSPOnlyMode(t *testing.T) {
	mockCache := NewMockSCIPCache()
	gateway := createGatewayWithMockCache(mockCache, true)

	assert.True(t, gateway.lspOnly)
}

// Test handleJSONRPC protocol validation
func TestHTTPGateway_handleJSONRPC_ProtocolValidation(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		contentType    string
		body           interface{}
		lspOnly        bool
		expectedStatus int
		checkResponse  func(t *testing.T, response protocol.JSONRPCResponse)
	}{
		{
			name:           "wrong HTTP method returns error",
			method:         "GET",
			contentType:    "application/json",
			body:           nil,
			expectedStatus: 200,
			checkResponse: func(t *testing.T, response protocol.JSONRPCResponse) {
				assert.NotNil(t, response.Error)
				assert.Equal(t, "Invalid Request", response.Error.Message)
				// The custom message is in the Data field
				assert.Contains(t, response.Error.Data, "Only POST method allowed")
			},
		},
		{
			name:           "wrong content type returns error",
			method:         "POST",
			contentType:    "text/plain",
			body:           `{"jsonrpc":"2.0"}`,
			expectedStatus: 200,
			checkResponse: func(t *testing.T, response protocol.JSONRPCResponse) {
				assert.NotNil(t, response.Error)
				assert.Equal(t, "Invalid Request", response.Error.Message)
				assert.Contains(t, response.Error.Data, "Content-Type must be application/json")
			},
		},
		{
			name:           "invalid JSON returns parse error",
			method:         "POST",
			contentType:    "application/json",
			body:           `{invalid json}`,
			expectedStatus: 200,
			checkResponse: func(t *testing.T, response protocol.JSONRPCResponse) {
				assert.NotNil(t, response.Error)
				assert.Equal(t, -32700, response.Error.Code)
			},
		},
		{
			name:        "invalid jsonrpc version returns error",
			method:      "POST",
			contentType: "application/json",
			body: protocol.JSONRPCRequest{
				JSONRPC: "1.0",
				ID:      json.RawMessage(`5`),
				Method:  "test",
			},
			expectedStatus: 200,
			checkResponse: func(t *testing.T, response protocol.JSONRPCResponse) {
				assert.NotNil(t, response.Error)
				assert.Equal(t, "Invalid Request", response.Error.Message)
				assert.Contains(t, response.Error.Data, "jsonrpc must be 2.0")
			},
		},
		{
			name:        "lspOnly mode blocks non-allowed method",
			method:      "POST",
			contentType: "application/json",
			body: protocol.JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      json.RawMessage(`3`),
				Method:  "textDocument/formatting",
				Params:  map[string]interface{}{},
			},
			lspOnly:        true,
			expectedStatus: 200,
			checkResponse: func(t *testing.T, response protocol.JSONRPCResponse) {
				assert.Equal(t, "2.0", response.JSONRPC)
				assert.Nil(t, response.Result)
				assert.NotNil(t, response.Error)
				assert.Equal(t, "Method not found", response.Error.Message)
				assert.Contains(t, response.Error.Data, "not available in LSP-only mode")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gateway := &HTTPGateway{
				lspManager:      &LSPManager{}, // Empty manager for protocol tests
				cacheConfig:     &config.CacheConfig{Enabled: true},
				lspOnly:         tt.lspOnly,
				responseFactory: protocol.NewResponseFactory(),
			}

			// Create HTTP request
			var body []byte
			if tt.body != nil {
				if str, ok := tt.body.(string); ok {
					body = []byte(str)
				} else {
					body, _ = json.Marshal(tt.body)
				}
			}

			req := httptest.NewRequest(tt.method, "/jsonrpc", bytes.NewReader(body))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}

			w := httptest.NewRecorder()

			// Execute
			gateway.handleJSONRPC(w, req)

			// Check status
			assert.Equal(t, tt.expectedStatus, w.Code)

			// Parse and check response
			if tt.checkResponse != nil {
				var response protocol.JSONRPCResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				tt.checkResponse(t, response)
			}
		})
	}
}

// Test handleHealth endpoint - simplified to test cache logic only
func TestHTTPGateway_handleHealth_CacheLogic(t *testing.T) {
	t.Run("cache status with metrics", func(t *testing.T) {
		mockCache := NewMockSCIPCache()
		metrics := &cache.CacheMetrics{HitCount: 100, MissCount: 20, EntryCount: 50}
		mockCache.On("GetMetrics").Return(metrics)

		gateway := createGatewayWithMockCache(mockCache, false)

		// Test getCacheMetricsSnapshot directly
		result := gateway.getCacheMetricsSnapshot()
		assert.Equal(t, metrics, result)
		mockCache.AssertExpectations(t)
	})

	t.Run("cache status with nil cache", func(t *testing.T) {
		gateway := &HTTPGateway{
			lspManager:      &LSPManager{scipCache: nil},
			cacheConfig:     &config.CacheConfig{Enabled: false},
			lspOnly:         false,
			responseFactory: protocol.NewResponseFactory(),
		}

		// Test getCacheMetricsSnapshot with nil cache
		result := gateway.getCacheMetricsSnapshot()
		assert.Nil(t, result)
	})
}

// Test handleCacheStats endpoint
func TestHTTPGateway_handleCacheStats(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		cacheMetrics   *cache.CacheMetrics
		expectedStatus int
		checkResponse  func(t *testing.T, w *httptest.ResponseRecorder)
	}{
		{
			name:   "GET with valid cache metrics",
			method: "GET",
			cacheMetrics: &cache.CacheMetrics{
				HitCount:        100,
				MissCount:       25,
				ErrorCount:      2,
				EvictionCount:   5,
				TotalSize:       1024 * 1024,
				EntryCount:      75,
				AverageHitTime:  time.Millisecond * 5,
				AverageMissTime: time.Millisecond * 50,
			},
			expectedStatus: 200,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.True(t, response["cache_enabled"].(bool))
				assert.Equal(t, float64(100), response["hit_count"])
				assert.Equal(t, float64(25), response["miss_count"])
				assert.Equal(t, float64(80), response["hit_rate_percent"]) // 100/(100+25)*100
				assert.Equal(t, float64(75), response["entry_count"])
			},
		},
		{
			name:           "GET with nil cache metrics returns service unavailable",
			method:         "GET",
			cacheMetrics:   nil,
			expectedStatus: 503,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Contains(t, w.Body.String(), "Cache not available")
			},
		},
		{
			name:           "POST method not allowed",
			method:         "POST",
			expectedStatus: 405,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Contains(t, w.Body.String(), "Method not allowed")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCache := NewMockSCIPCache()
			if tt.cacheMetrics != nil {
				mockCache.On("GetMetrics").Return(tt.cacheMetrics)
			} else {
				mockCache.On("GetMetrics").Return(nil)
			}

			gateway := &HTTPGateway{
				lspManager:      &LSPManager{scipCache: mockCache},
				cacheConfig:     &config.CacheConfig{Enabled: true},
				lspOnly:         false,
				responseFactory: protocol.NewResponseFactory(),
			}

			req := httptest.NewRequest(tt.method, "/cache/stats", nil)
			w := httptest.NewRecorder()

			gateway.handleCacheStats(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			tt.checkResponse(t, w)

			if tt.method == "GET" {
				mockCache.AssertExpectations(t)
			}
		})
	}
}

// Test handleCacheHealth endpoint
func TestHTTPGateway_handleCacheHealth(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		cacheAvailable bool
		healthMetrics  *cache.CacheMetrics
		healthError    error
		expectedStatus int
		checkResponse  func(t *testing.T, w *httptest.ResponseRecorder)
	}{
		{
			name:           "GET with healthy cache",
			method:         "GET",
			cacheAvailable: true,
			healthMetrics: &cache.CacheMetrics{
				TotalSize:  1024 * 1024,
				EntryCount: 100,
				HitCount:   200,
				MissCount:  50,
			},
			expectedStatus: 200,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.Equal(t, "healthy", response["status"])
				assert.True(t, response["enabled"].(bool))
				assert.Equal(t, float64(1024*1024), response["total_size"])
				assert.Equal(t, float64(100), response["entry_count"])
				assert.Equal(t, float64(250), response["uptime_requests"]) // 200+50
			},
		},
		{
			name:           "GET with cache health check error",
			method:         "GET",
			cacheAvailable: true,
			healthError:    errors.New("cache health check failed"),
			expectedStatus: 503,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.Equal(t, "unhealthy", response["status"])
				assert.False(t, response["enabled"].(bool))
				assert.Contains(t, response["error"], "cache health check failed")
			},
		},
		{
			name:           "GET with unavailable cache",
			method:         "GET",
			cacheAvailable: false,
			expectedStatus: 503,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Contains(t, w.Body.String(), "Cache not available")
			},
		},
		{
			name:           "POST method not allowed",
			method:         "POST",
			expectedStatus: 405,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Contains(t, w.Body.String(), "Method not allowed")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mockCache *MockSCIPCache
			var lspManager *LSPManager

			if tt.cacheAvailable {
				mockCache = NewMockSCIPCache()
				if tt.method == "GET" {
					mockCache.On("HealthCheck").Return(tt.healthMetrics, tt.healthError)
				}
				lspManager = &LSPManager{scipCache: mockCache}
			} else {
				lspManager = &LSPManager{scipCache: nil}
			}

			gateway := &HTTPGateway{
				lspManager:      lspManager,
				cacheConfig:     &config.CacheConfig{Enabled: tt.cacheAvailable},
				lspOnly:         false,
				responseFactory: protocol.NewResponseFactory(),
			}

			req := httptest.NewRequest(tt.method, "/cache/health", nil)
			w := httptest.NewRecorder()

			gateway.handleCacheHealth(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			tt.checkResponse(t, w)

			if tt.cacheAvailable && tt.method == "GET" {
				mockCache.AssertExpectations(t)
			}
		})
	}
}

// Test handleCacheClear endpoint
func TestHTTPGateway_handleCacheClear(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		cacheAvailable bool
		expectedStatus int
		checkResponse  func(t *testing.T, w *httptest.ResponseRecorder)
	}{
		{
			name:           "POST with available cache",
			method:         "POST",
			cacheAvailable: true,
			expectedStatus: 200,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)

				assert.Equal(t, "requested", response["status"])
				assert.Contains(t, response["message"], "Cache clear requested")
			},
		},
		{
			name:           "POST with unavailable cache",
			method:         "POST",
			cacheAvailable: false,
			expectedStatus: 503,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Contains(t, w.Body.String(), "Cache not available")
			},
		},
		{
			name:           "GET method not allowed",
			method:         "GET",
			expectedStatus: 405,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Contains(t, w.Body.String(), "Method not allowed")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var lspManager *LSPManager
			if tt.cacheAvailable {
				mockCache := NewMockSCIPCache()
				lspManager = &LSPManager{scipCache: mockCache}
			} else {
				lspManager = &LSPManager{scipCache: nil}
			}

			gateway := &HTTPGateway{
				lspManager:      lspManager,
				cacheConfig:     &config.CacheConfig{Enabled: tt.cacheAvailable},
				lspOnly:         false,
				responseFactory: protocol.NewResponseFactory(),
			}

			req := httptest.NewRequest(tt.method, "/cache/clear", nil)
			w := httptest.NewRecorder()

			gateway.handleCacheClear(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			tt.checkResponse(t, w)
		})
	}
}

// Test cache status determination
func TestHTTPGateway_determineCacheStatus(t *testing.T) {
	gateway := &HTTPGateway{}

	tests := []struct {
		name     string
		before   *cache.CacheMetrics
		after    *cache.CacheMetrics
		expected string
	}{
		{
			name:     "nil metrics returns disabled",
			before:   nil,
			after:    nil,
			expected: "disabled",
		},
		{
			name:     "hit count increased returns hit",
			before:   &cache.CacheMetrics{HitCount: 5, MissCount: 3},
			after:    &cache.CacheMetrics{HitCount: 6, MissCount: 3},
			expected: "hit",
		},
		{
			name:     "miss count increased returns miss",
			before:   &cache.CacheMetrics{HitCount: 5, MissCount: 3},
			after:    &cache.CacheMetrics{HitCount: 5, MissCount: 4},
			expected: "miss",
		},
		{
			name:     "no change returns unknown",
			before:   &cache.CacheMetrics{HitCount: 5, MissCount: 3},
			after:    &cache.CacheMetrics{HitCount: 5, MissCount: 3},
			expected: "unknown",
		},
		{
			name:     "both counts increased returns hit (hit takes precedence)",
			before:   &cache.CacheMetrics{HitCount: 5, MissCount: 3},
			after:    &cache.CacheMetrics{HitCount: 6, MissCount: 4},
			expected: "hit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := gateway.determineCacheStatus(tt.before, tt.after)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test addCacheHeaders function
func TestHTTPGateway_addCacheHeaders(t *testing.T) {
	mockCache := NewMockSCIPCache()
	metrics := &cache.CacheMetrics{
		EntryCount: 100,
		TotalSize:  2048,
	}
	mockCache.On("GetMetrics").Return(metrics)

	gateway := &HTTPGateway{
		lspManager:      &LSPManager{scipCache: mockCache},
		cacheConfig:     &config.CacheConfig{Enabled: true},
		lspOnly:         false,
		responseFactory: protocol.NewResponseFactory(),
	}

	w := httptest.NewRecorder()
	responseTime := 15 * time.Millisecond

	gateway.addCacheHeaders(w, "hit", responseTime)

	assert.Equal(t, "hit", w.Header().Get("X-LSP-Cache-Status"))
	assert.Equal(t, "15000", w.Header().Get("X-LSP-Response-Time")) // microseconds
	assert.Equal(t, "100", w.Header().Get("X-LSP-Cache-Size"))
	assert.Equal(t, "2048", w.Header().Get("X-LSP-Cache-Memory"))

	mockCache.AssertExpectations(t)
}

// Test addCacheHeaders with nil metrics
func TestHTTPGateway_addCacheHeaders_NilMetrics(t *testing.T) {
	mockCache := NewMockSCIPCache()
	mockCache.On("GetMetrics").Return(nil)

	gateway := &HTTPGateway{
		lspManager:      &LSPManager{scipCache: mockCache},
		cacheConfig:     &config.CacheConfig{Enabled: true},
		lspOnly:         false,
		responseFactory: protocol.NewResponseFactory(),
	}

	w := httptest.NewRecorder()
	responseTime := 10 * time.Millisecond

	gateway.addCacheHeaders(w, "disabled", responseTime)

	assert.Equal(t, "disabled", w.Header().Get("X-LSP-Cache-Status"))
	assert.Equal(t, "10000", w.Header().Get("X-LSP-Response-Time"))
	assert.Empty(t, w.Header().Get("X-LSP-Cache-Size"))
	assert.Empty(t, w.Header().Get("X-LSP-Cache-Memory"))

	mockCache.AssertExpectations(t)
}

// Test handleLanguages endpoint
func TestHTTPGateway_handleLanguages(t *testing.T) {
	gateway := &HTTPGateway{
		lspManager:      &LSPManager{},
		cacheConfig:     &config.CacheConfig{Enabled: true},
		lspOnly:         false,
		responseFactory: protocol.NewResponseFactory(),
	}

	t.Run("GET returns languages and extensions", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/languages", nil)
		w := httptest.NewRecorder()

		gateway.handleLanguages(w, req)

		assert.Equal(t, 200, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var resp map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		langs, ok := resp["languages"].([]interface{})
		require.True(t, ok)
		require.Greater(t, len(langs), 0)

		foundGo := false
		for _, l := range langs {
			if s, ok := l.(string); ok && s == "go" {
				foundGo = true
				break
			}
		}
		assert.True(t, foundGo)

		exts, ok := resp["extensions"].(map[string]interface{})
		require.True(t, ok)
		goExts, ok := exts["go"].([]interface{})
		require.True(t, ok)

		containsGo := false
		for _, e := range goExts {
			if s, ok := e.(string); ok && s == ".go" {
				containsGo = true
				break
			}
		}
		assert.True(t, containsGo)
	})

	t.Run("method not allowed", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/languages", nil)
		w := httptest.NewRecorder()

		gateway.handleLanguages(w, req)
		assert.Equal(t, 405, w.Code)
		assert.Contains(t, w.Body.String(), "Method not allowed")
	})
}

// Test error response formatting
func TestHTTPGateway_ErrorResponseFormatting(t *testing.T) {
	gateway := &HTTPGateway{responseFactory: protocol.NewResponseFactory()}

	tests := []struct {
		name     string
		response protocol.JSONRPCResponse
	}{
		{
			name:     "parse error",
			response: gateway.responseFactory.CreateParseError(nil),
		},
		{
			name:     "invalid request",
			response: gateway.responseFactory.CreateInvalidRequest(json.RawMessage(`1`), "test error"),
		},
		{
			name:     "method not found",
			response: gateway.responseFactory.CreateMethodNotFound(json.RawMessage(`2`), "method not found"),
		},
		{
			name:     "internal error",
			response: gateway.responseFactory.CreateInternalError(json.RawMessage(`3`), errors.New("internal error")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			gateway.writeResponse(w, tt.response)

			assert.Equal(t, 200, w.Code) // JSON-RPC errors return 200
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			var response protocol.JSONRPCResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, "2.0", response.JSONRPC)
			assert.NotNil(t, response.Error)
			assert.Nil(t, response.Result)
		})
	}
}
