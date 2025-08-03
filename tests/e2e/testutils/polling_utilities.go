package testutils

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// PollingConfig configures basic polling behavior
type PollingConfig struct {
	Interval time.Duration
	Timeout  time.Duration
}

// DefaultPollingConfig returns sensible defaults for polling
func DefaultPollingConfig() PollingConfig {
	return PollingConfig{
		Interval: 100 * time.Millisecond,
		Timeout:  30 * time.Second,
	}
}

// QuickPollingConfig returns config for quick operations
func QuickPollingConfig() PollingConfig {
	return PollingConfig{
		Interval: 50 * time.Millisecond,
		Timeout:  10 * time.Second,
	}
}

// ConditionFunc represents a condition that can be checked
type ConditionFunc func() (bool, error)

// WaitForCondition waits for a condition to become true
func WaitForCondition(condition ConditionFunc, config PollingConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()
	return WaitForConditionWithContext(ctx, condition, config)
}

// WaitForConditionWithContext waits for a condition with context
func WaitForConditionWithContext(ctx context.Context, condition ConditionFunc, config PollingConfig) error {
	ticker := time.NewTicker(config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for condition: %w", ctx.Err())
		case <-ticker.C:
			ok, err := condition()
			if err != nil {
				continue // Not ready yet, but don't fail
			}
			if ok {
				return nil
			}
		}
	}
}

// WaitForHttpEndpoint waits for an HTTP endpoint to be available
func WaitForHttpEndpoint(url string, config PollingConfig) error {
	condition := func() (bool, error) {
		resp, err := http.Get(url)
		if err != nil {
			return false, nil // Not ready yet, but don't fail
		}
		defer resp.Body.Close()
		return resp.StatusCode == http.StatusOK, nil
	}
	return WaitForCondition(condition, config)
}

// WaitForServerReady waits for server to be ready on given port
func WaitForServerReady(port int) error {
	url := fmt.Sprintf("http://localhost:%d/health", port)
	return WaitForHttpEndpoint(url, DefaultPollingConfig())
}

// WaitForServerReadyFast waits for server with quick config
func WaitForServerReadyFast(port int) error {
	url := fmt.Sprintf("http://localhost:%d/health", port)
	return WaitForHttpEndpoint(url, QuickPollingConfig())
}

// RetryOperation retries an operation with simple backoff
func RetryOperation(operation func() error, maxRetries int, delay time.Duration) error {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		if err := operation(); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if i < maxRetries-1 {
			time.Sleep(delay)
		}
	}
	return fmt.Errorf("operation failed after %d retries: %w", maxRetries, lastErr)
}

// SlowPollingConfig returns config for slower operations
func SlowPollingConfig() PollingConfig {
	return PollingConfig{Interval: 200 * time.Millisecond, Timeout: 60 * time.Second}
}

func BatchServerReadinessCheck(ports []int) error {
	for _, port := range ports {
		if err := WaitForServerReadyFast(port); err != nil {
			return fmt.Errorf("port %d not ready: %w", port, err)
		}
	}
	return nil
}

func WaitForServerStartupWithConfig(port int, config PollingConfig) error {
	url := fmt.Sprintf("http://localhost:%d/health", port)
	return WaitForHttpEndpoint(url, config)
}
