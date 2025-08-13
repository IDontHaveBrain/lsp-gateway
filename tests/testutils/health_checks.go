package testutils

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net"
	"net/http"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"lsp-gateway/src/server"
)

// HealthCheckConfig configures health check behavior
type HealthCheckConfig struct {
	Timeout        time.Duration
	Interval       time.Duration
	MaxRetries     int
	RetryDelay     time.Duration
	ExpectHealthy  bool
	CustomClient   *http.Client
	RequestHeaders map[string]string
}

// DefaultHealthCheckConfig returns sensible defaults for health checks
func DefaultHealthCheckConfig() HealthCheckConfig {
	return HealthCheckConfig{
		Timeout:       30 * time.Second,
		Interval:      500 * time.Millisecond,
		MaxRetries:    3,
		RetryDelay:    1 * time.Second,
		ExpectHealthy: true,
		CustomClient:  nil,
		RequestHeaders: map[string]string{
			"User-Agent": "LSP-Gateway-Test/1.0",
		},
	}
}

// PlatformAdjustedTimeout adjusts timeouts based on platform
func PlatformAdjustedTimeout(baseTimeout time.Duration) time.Duration {
	if runtime.GOOS == "windows" {
		return baseTimeout * 2
	}
	return baseTimeout
}

// WaitForServerReady performs comprehensive server readiness check
func WaitForServerReady(t *testing.T, baseURL string, timeout time.Duration) {
	t.Helper()

	if timeout == 0 {
		timeout = PlatformAdjustedTimeout(30 * time.Second)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client := &http.Client{Timeout: 5 * time.Second}

	healthURL := fmt.Sprintf("%s/health", baseURL)
	jsonrpcURL := fmt.Sprintf("%s/jsonrpc", baseURL)

	err := PollingCheck(ctx, 500*time.Millisecond, func() bool {
		healthOK := checkEndpointAvailability(healthURL, client)
		jsonrpcOK := checkEndpointAvailability(jsonrpcURL, client)
		return healthOK && jsonrpcOK
	})

	require.NoError(t, err, "Server failed to become ready within timeout %v", timeout)
}

// WaitForHTTPGatewayReady waits for HTTP gateway specific readiness
func WaitForHTTPGatewayReady(t *testing.T, port string, timeout time.Duration) {
	t.Helper()

	if timeout == 0 {
		timeout = PlatformAdjustedTimeout(20 * time.Second)
	}

	baseURL := fmt.Sprintf("http://localhost%s", port)
	if port[0] != ':' {
		baseURL = fmt.Sprintf("http://localhost:%s", port)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client := &http.Client{Timeout: 3 * time.Second}
	healthURL := fmt.Sprintf("%s/health", baseURL)

	err := PollingCheck(ctx, 300*time.Millisecond, func() bool {
		return checkHealthEndpointResponse(healthURL, client)
	})

	require.NoError(t, err, "HTTP Gateway failed to become ready on port %s within timeout %v", port, timeout)
}

// WaitForLSPManagerReady waits for LSP manager to be ready and responsive
func WaitForManagerReadyGeneric(t *testing.T, manager interface{}, timeout time.Duration) {
	t.Helper()

	if timeout == 0 {
		timeout = PlatformAdjustedTimeout(15 * time.Second)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	err := PollingCheck(ctx, 200*time.Millisecond, func() bool {
		return checkLSPManagerHealth(manager)
	})

	require.NoError(t, err, "LSP Manager failed to become ready within timeout %v", timeout)
}

// PollingCheck executes a condition with configurable polling until success or timeout
func PollingCheck(ctx context.Context, interval time.Duration, condition func() bool) error {
	if interval <= 0 {
		interval = 100 * time.Millisecond
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	if condition() {
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("polling condition not met within timeout: %w", ctx.Err())
		case <-ticker.C:
			if condition() {
				return nil
			}
		}
	}
}

// PollingCheckWithBackoff executes condition with exponential backoff
func PollingCheckWithBackoff(ctx context.Context, minInterval, maxInterval time.Duration, condition func() (bool, error)) error {
	if minInterval <= 0 {
		minInterval = 100 * time.Millisecond
	}
	if maxInterval <= 0 {
		maxInterval = 5 * time.Second
	}

	currentInterval := minInterval

	if ready, err := condition(); err != nil {
		return fmt.Errorf("condition check failed: %w", err)
	} else if ready {
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("condition not met within timeout: %w", ctx.Err())
		case <-time.After(currentInterval):
			if ready, err := condition(); err != nil {
				return fmt.Errorf("condition check failed: %w", err)
			} else if ready {
				return nil
			}

			currentInterval = currentInterval * 2
			if currentInterval > maxInterval {
				currentInterval = maxInterval
			}
		}
	}
}

// WaitForHealthEndpoint waits for health endpoint to become available and healthy
func WaitForHealthEndpoint(t *testing.T, url string, timeout time.Duration) {
	t.Helper()

	if timeout == 0 {
		timeout = PlatformAdjustedTimeout(15 * time.Second)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client := &http.Client{Timeout: 3 * time.Second}

	err := PollingCheck(ctx, 200*time.Millisecond, func() bool {
		return checkHealthEndpointResponse(url, client)
	})

	require.NoError(t, err, "Health endpoint %s failed to become available within timeout %v", url, timeout)
}

// WaitForJSONRPCEndpoint waits for JSON-RPC endpoint to become responsive
func WaitForJSONRPCEndpoint(t *testing.T, url string, timeout time.Duration) {
	t.Helper()

	if timeout == 0 {
		timeout = PlatformAdjustedTimeout(20 * time.Second)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client := &http.Client{Timeout: 5 * time.Second}

	err := PollingCheck(ctx, 300*time.Millisecond, func() bool {
		return checkJSONRPCEndpointResponse(url, client)
	})

	require.NoError(t, err, "JSON-RPC endpoint %s failed to become responsive within timeout %v", url, timeout)
}

// CheckServerConnectivity performs basic connectivity check to host:port
func CheckServerConnectivity(t *testing.T, host string, port string) {
	t.Helper()

	address := net.JoinHostPort(host, port)
	conn, err := net.DialTimeout("tcp", address, 3*time.Second)
	require.NoError(t, err, "Failed to connect to %s", address)

	if conn != nil {
		conn.Close()
	}
}

// WaitForServerConnectivity waits for basic TCP connectivity
func WaitForServerConnectivity(t *testing.T, host, port string, timeout time.Duration) {
	t.Helper()

	if timeout == 0 {
		timeout = 10 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	address := net.JoinHostPort(host, port)

	err := PollingCheck(ctx, 100*time.Millisecond, func() bool {
		conn, err := net.DialTimeout("tcp", address, 1*time.Second)
		if err != nil {
			return false
		}
		if conn != nil {
			conn.Close()
		}
		return true
	})

	require.NoError(t, err, "Server connectivity to %s failed within timeout %v", address, timeout)
}

// WaitForEndpointWithConfig waits for endpoint with custom configuration
func WaitForEndpointWithConfig(t *testing.T, url string, config HealthCheckConfig) {
	t.Helper()

	if config.Timeout == 0 {
		config.Timeout = 15 * time.Second
	}
	if config.Interval == 0 {
		config.Interval = 500 * time.Millisecond
	}

	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()

	client := config.CustomClient
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}

	err := PollingCheck(ctx, config.Interval, func() bool {
		return checkEndpointWithHeaders(url, client, config.RequestHeaders)
	})

	require.NoError(t, err, "Endpoint %s failed health check within timeout %v", url, config.Timeout)
}

// Advanced polling utilities

// WaitForConditionWithRetries waits for condition with configurable retries
func WaitForConditionWithRetries(t *testing.T, condition func() bool, maxRetries int, retryDelay time.Duration, description string) {
	t.Helper()

	if maxRetries <= 0 {
		maxRetries = 5
	}
	if retryDelay <= 0 {
		retryDelay = 1 * time.Second
	}

	for i := 0; i < maxRetries; i++ {
		if condition() {
			return
		}
		if i < maxRetries-1 {
			time.Sleep(retryDelay)
		}
	}

	require.Fail(t, fmt.Sprintf("Condition '%s' failed after %d retries", description, maxRetries))
}

// WaitForMultipleEndpoints waits for multiple endpoints to become ready
func WaitForMultipleEndpoints(t *testing.T, endpoints []string, timeout time.Duration) {
	t.Helper()

	if timeout == 0 {
		timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client := &http.Client{Timeout: 3 * time.Second}

	err := PollingCheck(ctx, 500*time.Millisecond, func() bool {
		for _, endpoint := range endpoints {
			if !checkEndpointAvailability(endpoint, client) {
				return false
			}
		}
		return true
	})

	require.NoError(t, err, "Not all endpoints became ready within timeout %v", timeout)
}

// Helper functions

// checkEndpointAvailability performs basic HTTP GET check
func checkEndpointAvailability(url string, client *http.Client) bool {
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// checkHealthEndpointResponse checks health endpoint and validates response
func checkHealthEndpointResponse(url string, client *http.Client) bool {
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false
	}

	var health map[string]interface{}
	if err := json.Unmarshal(body, &health); err != nil {
		return false
	}

	status, ok := health["status"].(string)
	return ok && status == "healthy"
}

// checkJSONRPCEndpointResponse tests JSON-RPC endpoint with basic request
func checkJSONRPCEndpointResponse(url string, client *http.Client) bool {
	testRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "textDocument/hover",
		"params": map[string]interface{}{
			"textDocument": map[string]interface{}{"uri": "file://test"},
			"position":     map[string]interface{}{"line": 0, "character": 0},
		},
		"id": 1,
	}

	requestBody, err := json.Marshal(testRequest)
	if err != nil {
		return false
	}

    resp, err := client.Post(url, "application/json", bytes.NewReader(requestBody))
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// checkEndpointWithHeaders checks endpoint with custom headers
func checkEndpointWithHeaders(url string, client *http.Client, headers map[string]string) bool {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// checkLSPManagerHealth checks if LSP manager is responsive
func checkLSPManagerHealth(manager interface{}) bool {
	switch m := manager.(type) {
	case *server.LSPManager:
		return m != nil
	case interface{ Started() bool }:
		return m.Started()
	case interface{ IsHealthy() bool }:
		return m.IsHealthy()
	default:
		return manager != nil
	}
}

// Specialized health checks for common scenarios

// WaitForLSPServerStartup waits for language-specific LSP server startup
func WaitForLSPServerStartup(t *testing.T, language string, baseURL string, timeout time.Duration) {
	t.Helper()

	if timeout == 0 {
		switch language {
		case "java":
			timeout = PlatformAdjustedTimeout(90 * time.Second)
		case "python":
			timeout = PlatformAdjustedTimeout(30 * time.Second)
		default:
			timeout = PlatformAdjustedTimeout(15 * time.Second)
		}
	}

	WaitForServerReady(t, baseURL, timeout)
}

// WaitForCacheReady waits for cache system to be initialized and responsive
func WaitForHTTPCacheReady(t *testing.T, baseURL string, timeout time.Duration) {
	t.Helper()

	if timeout == 0 {
		timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client := &http.Client{Timeout: 5 * time.Second}
	jsonrpcURL := fmt.Sprintf("%s/jsonrpc", baseURL)

	testRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "workspace/symbol",
		"params":  map[string]interface{}{"query": ""},
		"id":      1,
	}

	err := PollingCheck(ctx, 1*time.Second, func() bool {
		requestBody, err := json.Marshal(testRequest)
		if err != nil {
			return false
		}

        resp, err := client.Post(jsonrpcURL, "application/json", bytes.NewReader(requestBody))
		if err != nil {
			return false
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return false
		}

		body, _ := io.ReadAll(resp.Body)
		var response map[string]interface{}
		if err := json.Unmarshal(body, &response); err != nil {
			return false
		}

		_, hasError := response["error"]
		return !hasError
	})

	require.NoError(t, err, "Cache failed to become ready within timeout %v", timeout)
}
