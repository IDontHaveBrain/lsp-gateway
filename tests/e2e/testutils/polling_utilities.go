package testutils

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"time"
)

// WaitUntil polls until predicate returns true or timeout
func WaitUntil(ctx context.Context, interval, timeout time.Duration, predicate func() bool) error {
	if interval <= 0 {
		interval = 100 * time.Millisecond
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Check immediately first
	if predicate() {
		return nil
	}

	for {
		select {
		case <-timeoutCtx.Done():
			return fmt.Errorf("condition not met within timeout %v", timeout)
		case <-ticker.C:
			if predicate() {
				return nil
			}
		}
	}
}

// WaitForLSPReady waits for LSP server to be ready
func WaitForLSPReady(ctx context.Context, httpClient *HttpClient, language string) error {
	predicate := func() bool {
		if httpClient == nil {
			return false
		}

		err := httpClient.HealthCheck()
		if err != nil {
			return false
		}

		// Additional check for language server readiness
		request := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "initialize",
			"params": map[string]interface{}{
				"capabilities": map[string]interface{}{},
			},
		}

		_, err = httpClient.MakeRawJSONRPCRequest(ctx, request)
		return err == nil
	}

	// Adjust timeout based on language - Java needs longer initialization
	timeout := 15 * time.Second
	if language == "java" {
		timeout = 90 * time.Second
	} else if language == "python" {
		timeout = 30 * time.Second
	}

	return WaitUntil(ctx, 500*time.Millisecond, timeout, predicate)
}

// WaitForCacheIndexing waits for cache indexing to complete
func WaitForCacheIndexing(ctx context.Context, httpClient *HttpClient, expectedFiles int) error {
	predicate := func() bool {
		if httpClient == nil {
			return false
		}

		// Check cache status via workspace/symbol request which uses cache
		request := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "workspace/symbol",
			"params": map[string]interface{}{
				"query": "",
			},
		}

		response, err := httpClient.MakeRawJSONRPCRequest(ctx, request)
		if err != nil {
			return false
		}

		// Check if we have reasonable results (indicating cache is populated)
		if result, ok := response["result"].([]interface{}); ok {
			// If we expect specific number of files, check that
			if expectedFiles > 0 {
				return len(result) >= expectedFiles
			}
			// Otherwise, just ensure we have some symbols
			return len(result) > 0
		}

		return false
	}

	// Cache indexing can take time, especially for large projects
	timeout := 60 * time.Second
	interval := 1 * time.Second

	return WaitUntil(ctx, interval, timeout, predicate)
}

// WaitForFileUpdate waits for file modification to be detected
func WaitForFileUpdate(ctx context.Context, filePath string, lastModTime time.Time) error {
	predicate := func() bool {
		stat, err := os.Stat(filePath)
		if err != nil {
			return false
		}
		return stat.ModTime().After(lastModTime)
	}

	return WaitUntil(ctx, 100*time.Millisecond, 10*time.Second, predicate)
}

// WaitForConditionWithBackoff polls with exponential backoff
func WaitForConditionWithBackoff(ctx context.Context, minInterval, maxInterval, timeout time.Duration, condition func() (bool, error)) error {
	if minInterval <= 0 {
		minInterval = 100 * time.Millisecond
	}
	if maxInterval <= 0 {
		maxInterval = 5 * time.Second
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	currentInterval := minInterval

	// Check immediately first
	if ready, err := condition(); err != nil {
		return fmt.Errorf("condition check failed: %w", err)
	} else if ready {
		return nil
	}

	for {
		select {
		case <-timeoutCtx.Done():
			return fmt.Errorf("condition not met within timeout %v", timeout)
		case <-time.After(currentInterval):
			if ready, err := condition(); err != nil {
				return fmt.Errorf("condition check failed: %w", err)
			} else if ready {
				return nil
			}

			// Exponential backoff with cap
			currentInterval = currentInterval * 2
			if currentInterval > maxInterval {
				currentInterval = maxInterval
			}
		}
	}
}

// WaitForServerStartup waits for server to be ready with platform-specific timing
func WaitForServerStartup(ctx context.Context, httpClient *HttpClient) error {
	predicate := func() bool {
		return httpClient != nil && httpClient.HealthCheck() == nil
	}

	// Platform-specific timeout adjustment
	timeout := 15 * time.Second
	if runtime.GOOS == "windows" {
		timeout = 25 * time.Second // Windows typically needs more time
	}

	return WaitUntil(ctx, 500*time.Millisecond, timeout, predicate)
}

// WaitForLSPResponse waits for a successful LSP method response
func WaitForLSPResponse(ctx context.Context, httpClient *HttpClient, method string, params interface{}) error {
	predicate := func() bool {
		if httpClient == nil {
			return false
		}

		request := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  method,
			"params":  params,
		}

		response, err := httpClient.MakeRawJSONRPCRequest(ctx, request)
		if err != nil {
			return false
		}

		// Check for successful response (no error field)
		_, hasError := response["error"]
		return !hasError
	}

	return WaitUntil(ctx, 200*time.Millisecond, 10*time.Second, predicate)
}

// WaitForCacheReady waits for cache to be available and populated
func WaitForCacheReady(ctx context.Context, httpClient *HttpClient) error {
	return WaitForCacheIndexing(ctx, httpClient, 0) // 0 means just check cache is working
}

// PlatformAdjustedWait provides platform-specific wait times
func PlatformAdjustedWait(baseInterval time.Duration) time.Duration {
	if runtime.GOOS == "windows" {
		return baseInterval * 2 // Windows usually needs longer waits
	}
	return baseInterval
}

// WaitForDocumentSymbols waits for document symbols to be available
func WaitForDocumentSymbols(ctx context.Context, httpClient *HttpClient, fileURI string) error {
	predicate := func() bool {
		if httpClient == nil {
			return false
		}

		request := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"method":  "textDocument/documentSymbol",
			"params": map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": fileURI,
				},
			},
		}

		response, err := httpClient.MakeRawJSONRPCRequest(ctx, request)
		if err != nil {
			return false
		}

		// Check if we got symbols back
		if result, ok := response["result"].([]interface{}); ok {
			return len(result) > 0
		}

		return false
	}

	return WaitUntil(ctx, 500*time.Millisecond, 15*time.Second, predicate)
}
