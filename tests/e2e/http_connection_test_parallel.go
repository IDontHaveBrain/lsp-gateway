package e2e_test

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"lsp-gateway/tests/e2e/testutils"
)

// Parallel test functions using resource isolation

// TestHttpBasicConnectionLifecycleParallel tests basic server startup and health check with parallel execution
func TestHttpBasicConnectionLifecycleParallel(t *testing.T) {
	t.Parallel()
	
	setup, err := testutils.SetupIsolatedTest("http_basic_connection")
	require.NoError(t, err, "Failed to setup isolated test")
	defer func() {
		if cleanupErr := setup.Cleanup(); cleanupErr != nil {
			t.Logf("Warning: Cleanup failed: %v", cleanupErr)
		}
	}()
	
	// Create simple test config
	configContent := fmt.Sprintf(`
servers:
- name: test-lsp
  languages:
  - go
  command: echo
  args: ["test"]
  transport: stdio
  root_markers:
  - go.mod
  priority: 1
  weight: 1.0
port: %d
timeout: 30s
max_concurrent_requests: 50
`, setup.Resources.Port)
	
	configPath, err := setup.CreateTempConfig(configContent)
	require.NoError(t, err, "Failed to create config")
	
	// Start gateway server
	projectRoot, err := testutils.GetProjectRoot()
	require.NoError(t, err, "Failed to get project root")
	
	binaryPath := filepath.Join(projectRoot, "bin", "lsp-gateway")
	serverCmd := []string{binaryPath, "server", "--config", configPath}
	
	err = setup.StartServer(serverCmd, 30*time.Second)
	require.NoError(t, err, "Failed to start server")
	
	err = setup.WaitForServerReady(15 * time.Second)
	require.NoError(t, err, "Server failed to become ready")
	
	// Test health check
	httpClient := setup.GetHTTPClient()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	
	err = httpClient.HealthCheck(ctx)
	require.NoError(t, err, "Health check should succeed after server startup")
	
	// Test connection validation
	err = httpClient.ValidateConnection(ctx)
	require.NoError(t, err, "Connection validation should succeed")
	
	metrics := httpClient.GetMetrics()
	assert.Greater(t, metrics.TotalRequests, 0, "Should have recorded requests")
	assert.Greater(t, metrics.SuccessfulReqs, 0, "Should have successful requests")
	
	t.Logf("HTTP basic connection test completed successfully on port %d", setup.Resources.Port)
}

// TestHttpConnectionTimeoutsParallel tests connection timeout handling with parallel execution
func TestHttpConnectionTimeoutsParallel(t *testing.T) {
	t.Parallel()
	
	setup, err := testutils.SetupIsolatedTest("http_connection_timeouts")
	require.NoError(t, err, "Failed to setup isolated test")
	defer func() {
		if cleanupErr := setup.Cleanup(); cleanupErr != nil {
			t.Logf("Warning: Cleanup failed: %v", cleanupErr)
		}
	}()
	
	// Test timeout when server is not running
	shortTimeoutConfig := testutils.HttpClientConfig{
		BaseURL:    fmt.Sprintf("http://localhost:%d", setup.Resources.Port),
		Timeout:    100 * time.Millisecond,
		MaxRetries: 1,
		RetryDelay: 50 * time.Millisecond,
	}
	shortTimeoutClient := testutils.NewHttpClient(shortTimeoutConfig)
	defer shortTimeoutClient.Close()
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	err = shortTimeoutClient.HealthCheck(ctx)
	assert.Error(t, err, "Should timeout when server is not running")
	
	metrics := shortTimeoutClient.GetMetrics()
	assert.Greater(t, metrics.ConnectionErrors, 0, "Should record connection errors")
	
	t.Logf("HTTP connection timeouts test completed successfully on port %d", setup.Resources.Port)
}

// TestHttpConcurrentConnectionsParallel tests concurrent connection handling with parallel execution
func TestHttpConcurrentConnectionsParallel(t *testing.T) {
	t.Parallel()
	
	setup, err := testutils.SetupIsolatedTest("http_concurrent_connections")
	require.NoError(t, err, "Failed to setup isolated test")
	defer func() {
		if cleanupErr := setup.Cleanup(); cleanupErr != nil {
			t.Logf("Warning: Cleanup failed: %v", cleanupErr)
		}
	}()
	
	// Create test config
	configContent := fmt.Sprintf(`
servers:
- name: test-lsp
  languages:
  - go
  command: echo
  args: ["test"]
  transport: stdio
  root_markers:
  - go.mod
  priority: 1
  weight: 1.0
port: %d
timeout: 30s
max_concurrent_requests: 50
`, setup.Resources.Port)
	
	configPath, err := setup.CreateTempConfig(configContent)
	require.NoError(t, err, "Failed to create config")
	
	// Start gateway server
	projectRoot, err := testutils.GetProjectRoot()
	require.NoError(t, err, "Failed to get project root")
	
	binaryPath := filepath.Join(projectRoot, "bin", "lsp-gateway")
	serverCmd := []string{binaryPath, "server", "--config", configPath}
	
	err = setup.StartServer(serverCmd, 30*time.Second)
	require.NoError(t, err, "Failed to start server")
	
	err = setup.WaitForServerReady(15 * time.Second)
	require.NoError(t, err, "Server failed to become ready")
	
	// Test concurrent connections
	concurrentConfig := testutils.HttpClientConfig{
		BaseURL:            fmt.Sprintf("http://localhost:%d", setup.Resources.Port),
		Timeout:            15 * time.Second,
		MaxRetries:         2,
		RetryDelay:         100 * time.Millisecond,
		ConnectionPoolSize: 10,
		KeepAlive:          30 * time.Second,
	}
	concurrentClient := testutils.NewHttpClient(concurrentConfig)
	defer concurrentClient.Close()
	
	numWorkers := 5
	requestsPerWorker := 10
	var wg sync.WaitGroup
	errorChan := make(chan error, numWorkers*requestsPerWorker)
	
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < requestsPerWorker; j++ {
				err := concurrentClient.HealthCheck(ctx)
				if err != nil {
					errorChan <- err
				}
			}
		}()
	}
	
	wg.Wait()
	close(errorChan)
	
	errorCount := 0
	for err := range errorChan {
		t.Logf("Concurrent request error: %v", err)
		errorCount++
	}
	
	assert.LessOrEqual(t, errorCount, 5, "Should have minimal errors in concurrent requests")
	
	metrics := concurrentClient.GetMetrics()
	assert.Equal(t, numWorkers*requestsPerWorker, metrics.TotalRequests, "Should match expected request count")
	assert.GreaterOrEqual(t, metrics.SuccessfulReqs, numWorkers*requestsPerWorker-errorCount, "Successful requests should match")
	
	t.Logf("HTTP concurrent connections test completed successfully on port %d", setup.Resources.Port)
}

// TestHttpInvalidConnectionsParallel tests invalid connection scenarios with parallel execution
func TestHttpInvalidConnectionsParallel(t *testing.T) {
	t.Parallel()
	
	setup, err := testutils.SetupIsolatedTest("http_invalid_connections")
	require.NoError(t, err, "Failed to setup isolated test")
	defer func() {
		if cleanupErr := setup.Cleanup(); cleanupErr != nil {
			t.Logf("Warning: Cleanup failed: %v", cleanupErr)
		}
	}()
	
	// Test connection to invalid port (not started)
	invalidConfig := testutils.HttpClientConfig{
		BaseURL:    fmt.Sprintf("http://localhost:%d", setup.Resources.Port),
		Timeout:    2 * time.Second,
		MaxRetries: 1,
		RetryDelay: 100 * time.Millisecond,
	}
	invalidClient := testutils.NewHttpClient(invalidConfig)
	defer invalidClient.Close()
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	err = invalidClient.HealthCheck(ctx)
	assert.Error(t, err, "Should fail to connect to unused port")
	
	metrics := invalidClient.GetMetrics()
	assert.Greater(t, metrics.ConnectionErrors, 0, "Should record connection errors")
	assert.Equal(t, 0, metrics.SuccessfulReqs, "Should have no successful requests")
	
	// Test connection to invalid hostname
	hostnameConfig := testutils.HttpClientConfig{
		BaseURL:    "http://invalid-hostname-that-does-not-exist:8080",
		Timeout:    2 * time.Second,
		MaxRetries: 1,
		RetryDelay: 100 * time.Millisecond,
	}
	hostnameClient := testutils.NewHttpClient(hostnameConfig)
	defer hostnameClient.Close()
	
	err = hostnameClient.HealthCheck(ctx)
	assert.Error(t, err, "Should fail to connect to invalid hostname")
	
	hostnameMetrics := hostnameClient.GetMetrics()
	assert.Greater(t, hostnameMetrics.ConnectionErrors, 0, "Should record connection errors")
	
	t.Logf("HTTP invalid connections test completed successfully")
}