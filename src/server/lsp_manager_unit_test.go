package server

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"lsp-gateway/src/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Simple LSP Client mock for isolated testing
type SimpleMockLSPClient struct {
	mock.Mock
	active   bool
	supports map[string]bool
}

func NewSimpleMockLSPClient() *SimpleMockLSPClient {
	return &SimpleMockLSPClient{
		supports: make(map[string]bool),
		active:   true,
	}
}

func (m *SimpleMockLSPClient) Start(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *SimpleMockLSPClient) Stop() error {
	args := m.Called()
	return args.Error(0)
}

func (m *SimpleMockLSPClient) SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	args := m.Called(ctx, method, params)
	return args.Get(0).(json.RawMessage), args.Error(1)
}

func (m *SimpleMockLSPClient) SendNotification(ctx context.Context, method string, params interface{}) error {
	args := m.Called(ctx, method, params)
	return args.Error(0)
}

func (m *SimpleMockLSPClient) IsActive() bool {
	return m.active
}

func (m *SimpleMockLSPClient) Supports(method string) bool {
	if supported, exists := m.supports[method]; exists {
		return supported
	}
	return true
}

func (m *SimpleMockLSPClient) SetActive(active bool) {
	m.active = active
}

func (m *SimpleMockLSPClient) SetSupports(method string, supports bool) {
	m.supports[method] = supports
}

// Helper functions for test configurations
func createSimpleTestConfig() *config.Config {
	return &config.Config{
		Servers: map[string]*config.ServerConfig{
			"go": {
				Command: "gopls",
				Args:    []string{"serve"},
			},
		},
	}
}

func createCacheTestConfig() *config.Config {
	cfg := createSimpleTestConfig()
	cfg.Cache = &config.CacheConfig{
		Enabled:         true,
		MaxMemoryMB:     64,
		TTLHours:        1,
		StoragePath:     "/tmp/test-cache",
		Languages:       []string{"go"},
		BackgroundIndex: false,
		EvictionPolicy:  "lru",
		DiskCache:       false,
	}
	return cfg
}

// Test NewLSPManager constructor - Core functionality
func TestLSPManager_Constructor(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.Config
		expectError bool
		validate    func(*testing.T, *LSPManager)
	}{
		{
			name:        "nil config creates default",
			config:      nil,
			expectError: false,
			validate: func(t *testing.T, manager *LSPManager) {
				require.NotNil(t, manager.config)
				require.NotNil(t, manager.clients)
				require.NotNil(t, manager.clientErrors)
				require.NotNil(t, manager.cacheIntegrator)
				assert.Equal(t, 2, cap(manager.indexLimiter))
			},
		},
		{
			name:        "basic config initialization",
			config:      createSimpleTestConfig(),
			expectError: false,
			validate: func(t *testing.T, manager *LSPManager) {
				require.NotNil(t, manager.config)
				assert.Equal(t, 1, len(manager.config.Servers))
				assert.Contains(t, manager.config.Servers, "go")
				assert.NotNil(t, manager.documentManager)
				assert.NotNil(t, manager.workspaceAggregator)
			},
		},
		{
			name:        "cache config initialization",
			config:      createCacheTestConfig(),
			expectError: false,
			validate: func(t *testing.T, manager *LSPManager) {
				require.NotNil(t, manager.config.Cache)
				assert.True(t, manager.config.Cache.Enabled)
				assert.Equal(t, 64, manager.config.Cache.MaxMemoryMB)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, err := NewLSPManager(tt.config)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, manager)
			} else {
				assert.NoError(t, err)
				require.NotNil(t, manager)

				if tt.validate != nil {
					tt.validate(t, manager)
				}

				// Cleanup
				manager.Stop()
			}
		})
	}
}

// Test ProcessRequest with nil cache handling - Critical for panic prevention
func TestLSPManager_ProcessRequest_NilCacheHandling(t *testing.T) {
	cfg := createSimpleTestConfig()
	manager, err := NewLSPManager(cfg)
	require.NoError(t, err)

	// Explicitly set cache to nil to test nil handling
	manager.scipCache = nil

	// Setup mock client
	mockClient := NewSimpleMockLSPClient()
	mockClient.On("SendRequest", mock.Anything, "textDocument/definition", mock.Anything).Return(
		json.RawMessage(`[]`),
		nil,
	)
	// Mock textDocument/didOpen notification (may or may not be called)
	mockClient.On("SendNotification", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	// Mock Stop method for cleanup (may or may not be called)
	mockClient.On("Stop").Return(nil).Maybe()

	manager.mu.Lock()
	manager.clients["go"] = mockClient
	manager.mu.Unlock()

	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": "file:///test.go",
		},
		"position": map[string]interface{}{
			"line":      10,
			"character": 5,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// This should not panic even with nil cache
	result, err := manager.ProcessRequest(ctx, "textDocument/definition", params)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	mockClient.AssertExpectations(t)

	// Cleanup
	manager.Stop()
}

// Test Start method basic functionality
func TestLSPManager_Start_Basic(t *testing.T) {
	cfg := createSimpleTestConfig()
	manager, err := NewLSPManager(cfg)
	require.NoError(t, err)
	defer manager.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Start should not return error even if LSP servers aren't actually available
	err = manager.Start(ctx)
	// We expect this might fail due to missing LSP servers, but shouldn't panic
	t.Logf("Start result: %v", err)
}

// Test Stop method
func TestLSPManager_Stop_Basic(t *testing.T) {
	cfg := createSimpleTestConfig()
	manager, err := NewLSPManager(cfg)
	require.NoError(t, err)

	// Add mock client
	mockClient := NewSimpleMockLSPClient()
	mockClient.On("Stop").Return(nil)

	manager.mu.Lock()
	manager.clients["go"] = mockClient
	manager.mu.Unlock()

	err = manager.Stop()
	assert.NoError(t, err)

	// Clients map should be cleared
	manager.mu.RLock()
	assert.Empty(t, manager.clients)
	manager.mu.RUnlock()

	mockClient.AssertExpectations(t)
}

// Test Stop with errors
func TestLSPManager_Stop_WithErrors(t *testing.T) {
	cfg := createSimpleTestConfig()
	manager, err := NewLSPManager(cfg)
	require.NoError(t, err)

	// Add mock client that returns error
	mockClient := NewSimpleMockLSPClient()
	mockClient.On("Stop").Return(errors.New("stop failed"))

	manager.mu.Lock()
	manager.clients["go"] = mockClient
	manager.mu.Unlock()

	err = manager.Stop()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "stop failed")

	mockClient.AssertExpectations(t)
}

// Test CheckServerAvailability method
func TestLSPManager_CheckServerAvailability(t *testing.T) {
	cfg := &config.Config{
		Servers: map[string]*config.ServerConfig{
			"nonexistent": {
				Command: "nonexistent-command",
				Args:    []string{},
			},
		},
	}

	manager, err := NewLSPManager(cfg)
	require.NoError(t, err)
	defer manager.Stop()

	status := manager.CheckServerAvailability()

	require.Contains(t, status, "nonexistent")
	assert.False(t, status["nonexistent"].Available)
	assert.False(t, status["nonexistent"].Active)
	assert.NotNil(t, status["nonexistent"].Error)
	// Could be either "command not found" or security validation error
	errMsg := status["nonexistent"].Error.Error()
	assert.True(t,
		strings.Contains(errMsg, "command not found") || strings.Contains(errMsg, "invalid command"),
		"Expected error about invalid/missing command, got: %s", errMsg)
}

// Test GetClientStatus method
func TestLSPManager_GetClientStatus(t *testing.T) {
	cfg := createSimpleTestConfig()
	manager, err := NewLSPManager(cfg)
	require.NoError(t, err)

	// Add active mock client
	mockClient := NewSimpleMockLSPClient()
	mockClient.SetActive(true)
	mockClient.On("Stop").Return(nil).Maybe() // For cleanup

	manager.mu.Lock()
	manager.clients["go"] = mockClient
	manager.mu.Unlock()

	status := manager.GetClientStatus()

	require.Contains(t, status, "go")
	assert.True(t, status["go"].Active)
	assert.True(t, status["go"].Available)
	assert.NoError(t, status["go"].Error)

	// Cleanup
	manager.Stop()
}

// Test GetClient method
func TestLSPManager_GetClient(t *testing.T) {
	cfg := createSimpleTestConfig()
	manager, err := NewLSPManager(cfg)
	require.NoError(t, err)

	// Test getting non-existent client
	client, err := manager.GetClient("nonexistent")
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "no client for language")

	// Add mock client and test successful retrieval
	mockClient := NewSimpleMockLSPClient()
	mockClient.On("Stop").Return(nil).Maybe() // For cleanup

	manager.mu.Lock()
	manager.clients["go"] = mockClient
	manager.mu.Unlock()

	client, err = manager.GetClient("go")
	assert.NoError(t, err)
	assert.Equal(t, mockClient, client)

	// Cleanup
	manager.Stop()
}

// Test GetConfiguredServers method
func TestLSPManager_GetConfiguredServers(t *testing.T) {
	cfg := createSimpleTestConfig()
	manager, err := NewLSPManager(cfg)
	require.NoError(t, err)
	defer manager.Stop()

	servers := manager.GetConfiguredServers()
	assert.Equal(t, cfg.Servers, servers)
	assert.Contains(t, servers, "go")
}

// Test detectPrimaryLanguage method
func TestLSPManager_DetectPrimaryLanguage(t *testing.T) {
	cfg := createSimpleTestConfig()
	manager, err := NewLSPManager(cfg)
	require.NoError(t, err)
	defer manager.Stop()

	// Create temporary directory with Go files
	tempDir := t.TempDir()

	// Create go.mod file
	goMod := `module test
go 1.21`
	err = os.WriteFile(tempDir+"/go.mod", []byte(goMod), 0644)
	require.NoError(t, err)

	language := manager.detectPrimaryLanguage(tempDir)
	assert.Equal(t, "go", language)

	// Test with unknown directory
	tempDir2 := t.TempDir()
	language = manager.detectPrimaryLanguage(tempDir2)
	assert.Equal(t, "unknown", language)
}

// Test concurrent access to LSPManager
func TestLSPManager_ConcurrentAccess(t *testing.T) {
	cfg := createSimpleTestConfig()
	manager, err := NewLSPManager(cfg)
	require.NoError(t, err)

	// Add mock client
	mockClient := NewSimpleMockLSPClient()
	mockClient.On("Stop").Return(nil).Maybe() // For cleanup

	manager.mu.Lock()
	manager.clients["go"] = mockClient
	manager.mu.Unlock()

	// Test concurrent access to GetClientStatus
	const numGoroutines = 5
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			status := manager.GetClientStatus()
			assert.Contains(t, status, "go")
		}()
	}

	wg.Wait()

	// Cleanup
	manager.Stop()
}

// Test isNoViewsRPCError function
func TestIsNoViewsRPCError(t *testing.T) {
	tests := []struct {
		name     string
		raw      json.RawMessage
		expected bool
	}{
		{
			name:     "empty message",
			raw:      json.RawMessage(``),
			expected: false,
		},
		{
			name:     "no views error",
			raw:      json.RawMessage(`{"code": -32603, "message": "no views"}`),
			expected: true,
		},
		{
			name:     "no views error case insensitive",
			raw:      json.RawMessage(`{"code": -32603, "message": "No Views available"}`),
			expected: true,
		},
		{
			name:     "different error",
			raw:      json.RawMessage(`{"code": -32601, "message": "method not found"}`),
			expected: false,
		},
		{
			name:     "invalid JSON",
			raw:      json.RawMessage(`{invalid json`),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNoViewsRPCError(tt.raw)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test cache-related methods
func TestLSPManager_CacheIntegration(t *testing.T) {
	t.Run("isCacheableMethod works", func(t *testing.T) {
		cfg := createCacheTestConfig()
		manager, err := NewLSPManager(cfg)
		require.NoError(t, err)
		defer manager.Stop()

		// Test that the method exists and works
		result := manager.isCacheableMethod("textDocument/definition")
		assert.True(t, result)

		result = manager.isCacheableMethod("nonexistent/method")
		assert.False(t, result)
	})

	t.Run("InvalidateCache with nil cache", func(t *testing.T) {
		cfg := createSimpleTestConfig()
		manager, err := NewLSPManager(cfg)
		require.NoError(t, err)
		defer manager.Stop()

		// Set cache to nil
		manager.scipCache = nil

		// Should not panic
		err = manager.InvalidateCache("file:///test.go")
		assert.NoError(t, err)
	})
}

// Test ProcessRequest with unsupported file types
func TestLSPManager_ProcessRequest_UnsupportedFileType(t *testing.T) {
	cfg := createSimpleTestConfig()
	manager, err := NewLSPManager(cfg)
	require.NoError(t, err)
	defer manager.Stop()

	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": "file:///test.unknown",
		},
		"position": map[string]interface{}{
			"line":      10,
			"character": 5,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := manager.ProcessRequest(ctx, "textDocument/definition", params)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "unsupported file type")
}

// Test ProcessRequest for workspace symbol (no URI needed)
func TestLSPManager_ProcessRequest_WorkspaceSymbol(t *testing.T) {
	cfg := createSimpleTestConfig()
	manager, err := NewLSPManager(cfg)
	require.NoError(t, err)
	defer manager.Stop()

	params := map[string]interface{}{
		"query": "TestFunction",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Should not error due to missing URI since this is workspace/symbol
	result, err := manager.ProcessRequest(ctx, "workspace/symbol", params)

	// May return empty results but shouldn't error on missing URI
	t.Logf("Workspace symbol result: %v, error: %v", result, err)
	// In this test environment, it may fail due to no actual clients, but shouldn't panic
}
