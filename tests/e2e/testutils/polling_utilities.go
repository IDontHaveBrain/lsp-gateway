package testutils

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"
)

// PollingConfig configures optimized polling behavior
type PollingConfig struct {
	Interval       time.Duration
	Timeout        time.Duration
	BackoffFactor  float64
	MaxInterval    time.Duration
	InitialBackoff bool
	LogProgress    bool
}

// OptimizedPollingConfig returns optimized defaults for fast polling
func OptimizedPollingConfig() PollingConfig {
	return PollingConfig{
		Interval:       25 * time.Millisecond, // 50% faster initial interval
		Timeout:        15 * time.Second,
		BackoffFactor:  1.3,                   // More aggressive backoff for faster convergence
		MaxInterval:    1 * time.Second,
		InitialBackoff: false,
		LogProgress:    false,
	}
}

// DefaultPollingConfig returns sensible defaults for polling
func DefaultPollingConfig() PollingConfig {
	return PollingConfig{
		Interval:       100 * time.Millisecond,
		Timeout:        30 * time.Second,
		BackoffFactor:  1.5,
		MaxInterval:    2 * time.Second,
		InitialBackoff: false,
		LogProgress:    true,
	}
}

// QuickPollingConfig returns config for quick operations (shorter timeout)
func QuickPollingConfig() PollingConfig {
	return PollingConfig{
		Interval:       25 * time.Millisecond, // Optimized for speed
		Timeout:        5 * time.Second,
		BackoffFactor:  1.3,                   // Faster convergence
		MaxInterval:    500 * time.Millisecond,
		InitialBackoff: false,
		LogProgress:    false,
	}
}

// SlowPollingConfig returns config for slow operations (longer timeout)
func SlowPollingConfig() PollingConfig {
	return PollingConfig{
		Interval:       200 * time.Millisecond,
		Timeout:        60 * time.Second,
		BackoffFactor:  1.8,
		MaxInterval:    5 * time.Second,
		InitialBackoff: true,
		LogProgress:    true,
	}
}

// ConditionFunc represents a condition to wait for
type ConditionFunc func() (bool, error)

// WaitForCondition polls until condition returns true or timeout occurs
func WaitForCondition(condition ConditionFunc, config PollingConfig, description string) error {
	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()
	
	return WaitForConditionWithContext(ctx, condition, config, description)
}

// WaitForConditionWithContext polls until condition returns true, timeout occurs, or context is cancelled
func WaitForConditionWithContext(ctx context.Context, condition ConditionFunc, config PollingConfig, description string) error {
	if config.LogProgress {
		log.Printf("[Polling] Starting wait for: %s (timeout: %v)", description, config.Timeout)
	}
	
	interval := config.Interval
	start := time.Now()
	attempts := 0
	
	// Initial backoff if enabled
	if config.InitialBackoff {
		select {
		case <-ctx.Done():
			return fmt.Errorf("cancelled before first attempt while waiting for: %s", description)
		case <-time.After(interval):
		}
	}
	
	for {
		attempts++
		
		// Check condition
		satisfied, err := condition()
		if err != nil {
			if config.LogProgress {
				log.Printf("[Polling] Error in condition check (attempt %d): %v", attempts, err)
			}
		} else if satisfied {
			elapsed := time.Since(start)
			if config.LogProgress {
				log.Printf("[Polling] Condition satisfied after %v (%d attempts): %s", elapsed, attempts, description)
			}
			return nil
		}
		
		// Check context cancellation
		select {
		case <-ctx.Done():
			elapsed := time.Since(start)
			return fmt.Errorf("timeout after %v (%d attempts) waiting for: %s", elapsed, attempts, description)
		default:
		}
		
		// Apply optimized exponential backoff
		if config.BackoffFactor > 1.0 {
			newInterval := time.Duration(float64(interval) * config.BackoffFactor)
			if newInterval > config.MaxInterval {
				interval = config.MaxInterval
			} else {
				interval = newInterval
			}
		}
		
		// Wait for next attempt
		select {
		case <-ctx.Done():
			elapsed := time.Since(start)
			return fmt.Errorf("timeout after %v (%d attempts) waiting for: %s", elapsed, attempts, description)
		case <-time.After(interval):
		}
	}
}


// WaitForHttpEndpoint waits for an HTTP endpoint to become available
func WaitForHttpEndpoint(baseURL string, endpoint string, config PollingConfig) error {
	condition := func() (bool, error) {
		client := &http.Client{Timeout: 2 * time.Second}
		resp, err := client.Get(fmt.Sprintf("%s%s", baseURL, endpoint))
		if err != nil {
			return false, nil // Don't treat as error, just not ready
		}
		defer resp.Body.Close()
		
		return resp.StatusCode >= 200 && resp.StatusCode < 400, nil
	}
	
	description := fmt.Sprintf("HTTP endpoint %s%s to be available", baseURL, endpoint)
	return WaitForCondition(condition, config, description)
}

// WaitForServerReady waits for the LSP Gateway server to be ready
func WaitForServerReady(baseURL string) error {
	config := DefaultPollingConfig()
	config.Timeout = 15 * time.Second
	
	return WaitForHttpEndpoint(baseURL, "/health", config)
}

// WaitForServerReadyQuick waits for the LSP Gateway server with shorter timeout
func WaitForServerReadyQuick(baseURL string) error {
	config := QuickPollingConfig()
	return WaitForHttpEndpoint(baseURL, "/health", config)
}

// WaitForServerReadyFast waits for the LSP Gateway server using optimized health checks
func WaitForServerReadyFast(baseURL string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	healthChecker := NewFastHealthChecker(baseURL)
	defer healthChecker.Close()

	config := OptimizedPollingConfig()
	interval := config.Interval
	startTime := time.Now()
	attempts := 0

	for {
		attempts++

		if err := healthChecker.FastHealthCheck(ctx); err == nil {
			return nil
		}

		select {
		case <-ctx.Done():
			elapsed := time.Since(startTime)
			return fmt.Errorf("fast readiness check timeout after %v (%d attempts)", elapsed, attempts)
		default:
		}

		// Apply optimized exponential backoff
		if config.BackoffFactor > 1.0 {
			newInterval := time.Duration(float64(interval) * config.BackoffFactor)
			if newInterval > config.MaxInterval {
				interval = config.MaxInterval
			} else {
				interval = newInterval
			}
		}

		select {
		case <-ctx.Done():
			elapsed := time.Since(startTime)
			return fmt.Errorf("fast readiness check timeout after %v (%d attempts)", elapsed, attempts)
		case <-time.After(interval):
		}
	}
}

// WaitForServerReadyUltraFast provides ultra-fast server readiness detection (legacy compatibility)
func WaitForServerReadyUltraFast(baseURL string) error {
	return WaitForServerReadyFast(baseURL)
}

// WaitForServerStartup waits for server startup with port verification and fast health checks
func WaitForServerStartup(baseURL string, port int) error {
	return WaitForServerStartupFast(baseURL, port)
}

// WaitForServerStartupFast waits for server startup using optimized health checks
func WaitForServerStartupFast(baseURL string, port int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	healthChecker := NewFastHealthChecker(baseURL)
	defer healthChecker.Close()

	config := OptimizedPollingConfig()
	interval := config.Interval
	startTime := time.Now()
	attempts := 0

	for {
		attempts++

		// Primary health check using hybrid approach
		if err := healthChecker.HybridHealthCheck(ctx, port); err == nil {
			elapsed := time.Since(startTime)
			if config.LogProgress {
				log.Printf("[Optimized] Server ready after %v (%d attempts, port: %d)", elapsed, attempts, port)
			}
			return nil
		}

		// Check for context cancellation/timeout
		select {
		case <-ctx.Done():
			elapsed := time.Since(startTime)
			return fmt.Errorf("server startup timeout after %v (%d attempts, port: %d)", elapsed, attempts, port)
		default:
		}

		// Apply optimized exponential backoff
		if config.BackoffFactor > 1.0 {
			newInterval := time.Duration(float64(interval) * config.BackoffFactor)
			if newInterval > config.MaxInterval {
				interval = config.MaxInterval
			} else {
				interval = newInterval
			}
		}

		// Wait for next attempt
		select {
		case <-ctx.Done():
			elapsed := time.Since(startTime)
			return fmt.Errorf("server startup timeout after %v (%d attempts, port: %d)", elapsed, attempts, port)
		case <-time.After(interval):
		}
	}
}

// WaitForFileExists waits for a file to exist
func WaitForFileExists(filepath string, config PollingConfig) error {
	condition := func() (bool, error) {
		_, err := http.DefaultClient.Head(filepath)
		return err == nil, nil
	}
	
	description := fmt.Sprintf("file %s to exist", filepath)
	return WaitForCondition(condition, config, description)
}

// WaitForProcessReady waits for a process to be ready by checking a condition
func WaitForProcessReady(checkFunc func() bool, processName string, config PollingConfig) error {
	condition := func() (bool, error) {
		return checkFunc(), nil
	}
	
	description := fmt.Sprintf("process %s to be ready", processName)
	return WaitForCondition(condition, config, description)
}

// WaitForLSPResponse waits for a successful LSP response
func WaitForLSPResponse(httpClient *HttpClient, method string, params interface{}, config PollingConfig) error {
	condition := func() (bool, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		
		_, err := httpClient.sendLSPRequest(ctx, method, params)
		return err == nil, nil
	}
	
	description := fmt.Sprintf("LSP method %s to respond successfully", method)
	return WaitForCondition(condition, config, description)
}

// WaitForWorkspaceReady waits for workspace to be loaded and ready
func WaitForWorkspaceReady(httpClient *HttpClient, config PollingConfig) error {
	condition := func() (bool, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		
		// Check if workspace symbols are available (indicates loaded workspace)
		symbols, err := httpClient.WorkspaceSymbol(ctx, "test")
		if err != nil {
			return false, nil
		}
		// If we get a response (even empty), workspace is likely ready
		return symbols != nil, nil
	}
	
	description := "workspace to be loaded and ready"
	return WaitForCondition(condition, config, description)
}

// RetryOperation executes an operation with retries and exponential backoff
func RetryOperation(operation func() error, config PollingConfig, description string) error {
	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()
	
	return RetryOperationWithContext(ctx, operation, config, description)
}

// RetryOperationWithContext executes an operation with retries and context support
func RetryOperationWithContext(ctx context.Context, operation func() error, config PollingConfig, description string) error {
	if config.LogProgress {
		log.Printf("[Retry] Starting operation: %s (timeout: %v)", description, config.Timeout)
	}
	
	interval := config.Interval
	start := time.Now()
	attempts := 0
	var lastErr error
	
	for {
		attempts++
		
		// Execute operation
		err := operation()
		if err == nil {
			elapsed := time.Since(start)
			if config.LogProgress {
				log.Printf("[Retry] Operation succeeded after %v (%d attempts): %s", elapsed, attempts, description)
			}
			return nil
		}
		
		lastErr = err
		if config.LogProgress {
			log.Printf("[Retry] Operation failed (attempt %d): %v", attempts, err)
		}
		
		// Check context cancellation
		select {
		case <-ctx.Done():
			elapsed := time.Since(start)
			return fmt.Errorf("operation failed after %v (%d attempts): %s - last error: %w", elapsed, attempts, description, lastErr)
		default:
		}
		
		// Apply optimized exponential backoff
		if config.BackoffFactor > 1.0 {
			newInterval := time.Duration(float64(interval) * config.BackoffFactor)
			if newInterval > config.MaxInterval {
				interval = config.MaxInterval
			} else {
				interval = newInterval
			}
		}
		
		// Wait for next attempt
		select {
		case <-ctx.Done():
			elapsed := time.Since(start)
			return fmt.Errorf("operation failed after %v (%d attempts): %s - last error: %w", elapsed, attempts, description, lastErr)
		case <-time.After(interval):
		}
	}
}

// QuickConnectivityTest performs instant TCP connectivity test
func QuickConnectivityTest(baseURL string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	healthChecker := NewFastHealthChecker(baseURL)
	defer healthChecker.Close()

	return healthChecker.QuickConnectivityCheck(ctx)
}

// BatchServerReadinessCheck tests multiple servers for readiness in parallel
func BatchServerReadinessCheck(baseURLs []string, timeout time.Duration) map[string]error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	results := make(map[string]error)
	resultChan := make(chan struct {
		url string
		err error
	}, len(baseURLs))

	// Start parallel checks
	for _, url := range baseURLs {
		go func(url string) {
			healthChecker := NewFastHealthChecker(url)
			defer healthChecker.Close()
			
			err := healthChecker.FastHealthCheck(ctx)
			resultChan <- struct {
				url string
				err error
			}{url, err}
		}(url)
	}

	// Collect results
	for i := 0; i < len(baseURLs); i++ {
		select {
		case result := <-resultChan:
			results[result.url] = result.err
		case <-ctx.Done():
			// Mark remaining URLs as timed out
			for _, url := range baseURLs {
				if _, exists := results[url]; !exists {
					results[url] = fmt.Errorf("timeout during batch check")
				}
			}
			return results
		}
	}

	return results
}

// FastPollingConfig provides configurations for fast server detection (legacy compatibility)
type FastPollingConfig struct {
	InitialInterval   time.Duration
	MaxInterval       time.Duration
	BackoffFactor     float64
	FastFailTimeout   time.Duration
	StartupTimeout    time.Duration
	LogProgress       bool
	EnableFastFail    bool
	AdaptivePolling   bool
	MinSystemInterval time.Duration
}

// FastStartupPollingConfig returns config optimized for server startup detection (legacy compatibility)
func FastStartupPollingConfig() FastPollingConfig {
	return FastPollingConfig{
		InitialInterval:   25 * time.Millisecond,
		MaxInterval:       1 * time.Second,
		BackoffFactor:     1.3,
		FastFailTimeout:   3 * time.Second,
		StartupTimeout:    15 * time.Second,
		LogProgress:       false,
		EnableFastFail:    true,
		AdaptivePolling:   false, // Simplified, no adaptive logic
		MinSystemInterval: 25 * time.Millisecond,
	}
}

// WaitForServerStartupWithConfig waits for server startup with custom FastPollingConfig (legacy compatibility)
func WaitForServerStartupWithConfig(baseURL string, port int, config FastPollingConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), config.StartupTimeout)
	defer cancel()

	healthChecker := NewFastHealthChecker(baseURL)
	defer healthChecker.Close()

	if config.LogProgress {
		log.Printf("[FastPolling] Starting server startup detection (port: %d, timeout: %v)", port, config.StartupTimeout)
	}

	interval := config.InitialInterval
	startTime := time.Now()
	attempts := 0

	for {
		attempts++

		// Primary health check using hybrid approach
		if err := healthChecker.HybridHealthCheck(ctx, port); err == nil {
			elapsed := time.Since(startTime)
			if config.LogProgress {
				log.Printf("[FastPolling] Server ready after %v (%d attempts, port: %d)", elapsed, attempts, port)
			}
			return nil
		}

		// Check for context cancellation/timeout
		select {
		case <-ctx.Done():
			elapsed := time.Since(startTime)
			return fmt.Errorf("server startup timeout after %v (%d attempts, port: %d)", elapsed, attempts, port)
		default:
		}

		// Apply exponential backoff
		if config.BackoffFactor > 1.0 {
			newInterval := time.Duration(float64(interval) * config.BackoffFactor)
			if newInterval > config.MaxInterval {
				interval = config.MaxInterval
			} else {
				interval = newInterval
			}
		}

		// Wait for next attempt
		select {
		case <-ctx.Done():
			elapsed := time.Since(startTime)
			return fmt.Errorf("server startup timeout after %v (%d attempts, port: %d)", elapsed, attempts, port)
		case <-time.After(interval):
		}
	}
}

// WaitForServerStartupAdaptive provides legacy compatibility for adaptive server startup
func WaitForServerStartupAdaptive(baseURL string, port int) error {
	return WaitForServerStartupFast(baseURL, port)
}

// WaitForServerReadyAdaptive provides legacy compatibility for adaptive server readiness
func WaitForServerReadyAdaptive(baseURL string) error {
	return WaitForServerReadyFast(baseURL)
}