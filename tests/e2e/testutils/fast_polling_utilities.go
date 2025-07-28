package testutils

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"time"
)

// FastPollingConfig provides configurations optimized for adaptive server detection
type FastPollingConfig struct {
	InitialInterval   time.Duration // Reasonable initial polling interval
	MaxInterval       time.Duration // Cap for backoff
	BackoffFactor     float64       // Exponential backoff to balance speed vs resource usage
	FastFailTimeout   time.Duration // Timeout for fast-fail detection
	StartupTimeout    time.Duration // Overall timeout for server startup
	LogProgress       bool          // Whether to log polling progress
	EnableFastFail    bool          // Whether to enable fast-fail detection
	AdaptivePolling   bool          // Whether to enable adaptive polling based on system performance
	MinSystemInterval time.Duration // Minimum interval to respect system limitations
}

// AdaptivePollingConfig returns config for adaptive server startup detection (system-aware)
func AdaptivePollingConfig() FastPollingConfig {
	return FastPollingConfig{
		InitialInterval:   50 * time.Millisecond,  // Reasonable start for all systems
		MaxInterval:       500 * time.Millisecond, // Higher cap for slow systems
		BackoffFactor:     1.5,                    // Better exponential backoff
		FastFailTimeout:   3 * time.Second,        // Reasonable failure detection
		StartupTimeout:    15 * time.Second,       // Extended timeout for slow systems
		LogProgress:       false,                  // Minimal logging for speed
		EnableFastFail:    true,                   // Enable fast-fail
		AdaptivePolling:   true,                   // Enable adaptive behavior
		MinSystemInterval: 30 * time.Millisecond,  // Absolute minimum to respect systems
	}
}

// BalancedPollingConfig returns config for balanced server startup detection
func BalancedPollingConfig() FastPollingConfig {
	return FastPollingConfig{
		InitialInterval:   75 * time.Millisecond,  // Balanced initial polling
		MaxInterval:       1 * time.Second,        // Higher cap for reliability
		BackoffFactor:     1.4,                    // Moderate exponential backoff
		FastFailTimeout:   4 * time.Second,        // Reasonable failure detection
		StartupTimeout:    20 * time.Second,       // Extended timeout
		LogProgress:       true,                   // Normal logging
		EnableFastFail:    true,                   // Enable fast-fail
		AdaptivePolling:   true,                   // Enable adaptive behavior
		MinSystemInterval: 50 * time.Millisecond,  // Reasonable minimum for logging mode
	}
}

// WaitForServerStartupAdaptive waits for server startup using adaptive health checks
func WaitForServerStartupAdaptive(baseURL string, port int) error {
	config := AdaptivePollingConfig()
	return WaitForServerStartupWithConfig(baseURL, port, config)
}

// WaitForServerStartupFast waits for server startup using optimized health checks (legacy compatibility)
func WaitForServerStartupFast(baseURL string, port int) error {
	config := AdaptivePollingConfig()
	return WaitForServerStartupWithConfig(baseURL, port, config)
}

// WaitForServerStartupWithConfig waits for server startup with custom configuration
func WaitForServerStartupWithConfig(baseURL string, port int, config FastPollingConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), config.StartupTimeout)
	defer cancel()

	healthChecker := NewFastHealthChecker(baseURL)
	defer healthChecker.Close()

	if config.LogProgress {
		log.Printf("[FastPolling] Starting fast server startup detection (port: %d, timeout: %v)", 
			port, config.StartupTimeout)
	}

	interval := config.InitialInterval
	startTime := time.Now()
	attempts := 0
	fastFailCtx, fastFailCancel := context.WithTimeout(ctx, config.FastFailTimeout)
	defer fastFailCancel()

	for {
		attempts++
		
		// Fast-fail check: determine if server has definitively failed
		if config.EnableFastFail && attempts > 3 { // Skip fast-fail for first few attempts
			if failed, err := healthChecker.IsServerDefinitelyFailed(fastFailCtx, port); failed {
				elapsed := time.Since(startTime)
				return fmt.Errorf("server startup definitely failed after %v (%d attempts): %w", 
					elapsed, attempts, err)
			}
		}

		// Primary health check using hybrid approach
		if err := healthChecker.HybridHealthCheck(ctx, port); err == nil {
			elapsed := time.Since(startTime)
			if config.LogProgress {
				log.Printf("[FastPolling] Server ready after %v (%d attempts, port: %d)", 
					elapsed, attempts, port)
			}
			return nil
		}

		// Check for context cancellation/timeout
		select {
		case <-ctx.Done():
			elapsed := time.Since(startTime)
			return fmt.Errorf("server startup timeout after %v (%d attempts, port: %d)", 
				elapsed, attempts, port)
		default:
		}

		// Apply adaptive exponential backoff
		if config.BackoffFactor > 1.0 {
			newInterval := time.Duration(float64(interval) * config.BackoffFactor)
			
			// Apply system-aware adaptive logic
			if config.AdaptivePolling {
				newInterval = adaptIntervalForSystem(newInterval, attempts, config)
			}
			
			// Ensure we respect minimum and maximum bounds
			if newInterval < config.MinSystemInterval {
				interval = config.MinSystemInterval
			} else if newInterval > config.MaxInterval {
				interval = config.MaxInterval
			} else {
				interval = newInterval
			}
		}

		// Wait for next attempt
		select {
		case <-ctx.Done():
			elapsed := time.Since(startTime)
			return fmt.Errorf("server startup timeout after %v (%d attempts, port: %d)", 
				elapsed, attempts, port)
		case <-time.After(interval):
		}
	}
}

// WaitForServerReadyAdaptive provides adaptive server readiness detection
func WaitForServerReadyAdaptive(baseURL string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	healthChecker := NewFastHealthChecker(baseURL) 
	defer healthChecker.Close()

	config := AdaptivePollingConfig()
	interval := config.InitialInterval
	startTime := time.Now()
	attempts := 0

	for {
		attempts++

		// Use adaptive health check strategy
		if err := healthChecker.FastHealthCheck(ctx); err == nil {
			return nil
		}

		// Check for timeout
		select {
		case <-ctx.Done():
			elapsed := time.Since(startTime)
			return fmt.Errorf("adaptive readiness check timeout after %v (%d attempts)", 
				elapsed, attempts)
		default:
		}

		// Apply adaptive exponential backoff
		if attempts > 3 && config.BackoffFactor > 1.0 {
			newInterval := time.Duration(float64(interval) * config.BackoffFactor)
			
			// Apply system-aware adaptive logic
			if config.AdaptivePolling {
				newInterval = adaptIntervalForSystem(newInterval, attempts, config)
			}
			
			// Ensure we respect minimum and maximum bounds
			if newInterval < config.MinSystemInterval {
				interval = config.MinSystemInterval
			} else if newInterval > config.MaxInterval {
				interval = config.MaxInterval
			} else {
				interval = newInterval
			}
		}

		// Wait for next attempt
		select {
		case <-ctx.Done():
			elapsed := time.Since(startTime)
			return fmt.Errorf("adaptive readiness check timeout after %v (%d attempts)", 
				elapsed, attempts)
		case <-time.After(interval):
		}
	}
}

// WaitForServerReadyUltraFast provides ultra-fast server readiness detection (legacy compatibility)
func WaitForServerReadyUltraFast(baseURL string) error {
	return WaitForServerReadyAdaptive(baseURL)
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

// adaptIntervalForSystem applies system-aware adaptive logic to polling intervals
func adaptIntervalForSystem(baseInterval time.Duration, attempts int, config FastPollingConfig) time.Duration {
	// Consider system characteristics for adaptive behavior
	multiplier := 1.0
	
	// Adapt based on number of attempts (slow systems need more time)
	if attempts > 10 {
		multiplier *= 1.3 // Slow system detected, give more time
	} else if attempts > 20 {
		multiplier *= 1.6 // Very slow system, significantly more time
	}
	
	// Consider runtime characteristics (GOMAXPROCS as system indicator)
	if runtime.GOMAXPROCS(0) <= 2 {
		multiplier *= 1.2 // Limited CPU, be more conservative
	}
	
	// Apply multiplier but respect bounds
	adaptedInterval := time.Duration(float64(baseInterval) * multiplier)
	
	// Ensure we don't go below system minimum or above maximum
	if adaptedInterval < config.MinSystemInterval {
		adaptedInterval = config.MinSystemInterval
	} else if adaptedInterval > config.MaxInterval {
		adaptedInterval = config.MaxInterval
	}
	
	return adaptedInterval
}

// SlowSystemPollingConfig returns config optimized for slow systems
func SlowSystemPollingConfig() FastPollingConfig {
	return FastPollingConfig{
		InitialInterval:   100 * time.Millisecond, // Conservative start
		MaxInterval:       2 * time.Second,        // High cap for very slow systems
		BackoffFactor:     1.6,                    // Aggressive backoff
		FastFailTimeout:   6 * time.Second,        // Extended failure detection
		StartupTimeout:    30 * time.Second,       // Very extended timeout
		LogProgress:       true,                   // Enable logging for debugging
		EnableFastFail:    false,                  // Disable fast-fail for slow systems
		AdaptivePolling:   true,                   // Enable adaptive behavior
		MinSystemInterval: 75 * time.Millisecond,  // Higher minimum for slow systems
	}
}

// FastStartupPollingConfig returns config optimized for server startup detection
func FastStartupPollingConfig() FastPollingConfig {
	return AdaptivePollingConfig()
}