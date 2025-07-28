package testutils

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"
)

// PollingConfig configures polling behavior
type PollingConfig struct {
	Interval       time.Duration
	Timeout        time.Duration
	BackoffFactor  float64
	MaxInterval    time.Duration
	InitialBackoff bool
	LogProgress    bool
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
		Interval:       50 * time.Millisecond,
		Timeout:        5 * time.Second,
		BackoffFactor:  1.2,
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

// WaitForServerReadyFast waits for the LSP Gateway server using optimized fast health checks
func WaitForServerReadyFast(baseURL string) error {
	return WaitForServerReadyUltraFast(baseURL)
}

// WaitForServerStartup waits for server startup with port verification and fast health checks
func WaitForServerStartup(baseURL string, port int) error {
	return WaitForServerStartupFast(baseURL, port)
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
			elapsed := time.Since(start)
			return fmt.Errorf("operation failed after %v (%d attempts): %s - last error: %w", elapsed, attempts, description, lastErr)
		case <-time.After(interval):
		}
	}
}