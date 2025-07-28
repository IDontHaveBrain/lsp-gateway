package testutils

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// FastHealthChecker provides optimized health checking for rapid server startup detection
type FastHealthChecker struct {
	baseURL       string
	timeout       time.Duration
	client        *http.Client
	tcpTimeout    time.Duration
	fastFailDelay time.Duration
	maxRetries    int
	baseBackoff   time.Duration
}

// NewFastHealthChecker creates a new fast health checker optimized for quick server detection
func NewFastHealthChecker(baseURL string) *FastHealthChecker {
	// Reasonable HTTP client with balanced timeouts for reliability
	transport := &http.Transport{
		MaxIdleConns:        5,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     5 * time.Second,
		DisableKeepAlives:   false,
		DialContext: (&net.Dialer{
			Timeout:   2 * time.Second, // Reasonable connection timeout
			KeepAlive: 2 * time.Second,
		}).DialContext,
	}

	return &FastHealthChecker{
		baseURL:       baseURL,
		timeout:       3 * time.Second,        // Reasonable overall timeout
		tcpTimeout:    1 * time.Second,        // Reasonable TCP check
		fastFailDelay: 500 * time.Millisecond, // Allow server startup time
		maxRetries:    3,                      // Retry failed checks
		baseBackoff:   100 * time.Millisecond, // Base exponential backoff
		client: &http.Client{
			Timeout:   3 * time.Second,
			Transport: transport,
		},
	}
}

// QuickConnectivityCheck performs TCP connectivity test with retry logic
func (fhc *FastHealthChecker) QuickConnectivityCheck(ctx context.Context) error {
	// Extract host and port from baseURL
	host, port, err := fhc.extractHostPort()
	if err != nil {
		return fmt.Errorf("failed to parse URL: %w", err)
	}

	return fhc.retryWithBackoff(ctx, func() error {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), fhc.tcpTimeout)
		if err != nil {
			return fmt.Errorf("TCP connection failed: %w", err)
		}
		defer conn.Close()
		return nil
	})
}

// FastHeadCheck performs lightweight HTTP HEAD request for server readiness with retry logic
func (fhc *FastHealthChecker) FastHeadCheck(ctx context.Context) error {
	url := fmt.Sprintf("%s/health", fhc.baseURL)

	return fhc.retryWithBackoff(ctx, func() error {
		req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
		if err != nil {
			return fmt.Errorf("failed to create HEAD request: %w", err)
		}

		resp, err := fhc.client.Do(req)
		if err != nil {
			return fmt.Errorf("HEAD request failed: %w", err)
		}
		defer resp.Body.Close()

		// Accept any 2xx or 3xx status for HEAD request
		if resp.StatusCode >= 200 && resp.StatusCode < 400 {
			return nil
		}

		return fmt.Errorf("HEAD check returned status %d", resp.StatusCode)
	})
}

// FastHealthCheck combines TCP connectivity and HTTP HEAD check for rapid server validation
func (fhc *FastHealthChecker) FastHealthCheck(ctx context.Context) error {
	// First, quick TCP connectivity test
	if err := fhc.QuickConnectivityCheck(ctx); err != nil {
		return fmt.Errorf("connectivity check failed: %w", err)
	}

	// Then lightweight HTTP HEAD check
	if err := fhc.FastHeadCheck(ctx); err != nil {
		return fmt.Errorf("HTTP check failed: %w", err)
	}

	return nil
}

// HybridHealthCheck combines port binding check with actual HTTP connectivity
func (fhc *FastHealthChecker) HybridHealthCheck(ctx context.Context, port int) error {
	// First check if port is bound
	if isPortAvailable(port) {
		return fmt.Errorf("port %d is not bound by server", port)
	}

	// Small delay to allow server to fully initialize after port binding
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(fhc.fastFailDelay):
	}

	// Then verify actual HTTP connectivity
	return fhc.FastHealthCheck(ctx)
}

// IsServerDefinitelyFailed performs reasonable fast-fail checks to determine if server startup has definitively failed
func (fhc *FastHealthChecker) IsServerDefinitelyFailed(ctx context.Context, port int) (bool, error) {
	// Check 1: If port is available, give server reasonable time to start up
	if isPortAvailable(port) {
		// Allow more time for server startup - different languages have different startup times
		// Go: ~500ms-2s, Python: ~1-3s, Java: ~2-5s, TypeScript: ~1-3s
		initialWait := 2 * time.Second
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(initialWait):
		}
		
		// Double-check after reasonable startup time
		if isPortAvailable(port) {
			// Additional wait for slower servers (Java, Python with heavy imports)
			select {
			case <-ctx.Done():
				return false, ctx.Err()
			case <-time.After(3 * time.Second):
			}
			
			if isPortAvailable(port) {
				return true, fmt.Errorf("server failed to bind to port %d after 5s", port)
			}
		}
	}

	// Check 2: TCP connection actively refused (but only after port is bound)
	if !isPortAvailable(port) {
		// Port is bound, but connection might still be refused during startup
		// Give server time to accept connections after port binding
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(fhc.fastFailDelay):
		}
		
		if err := fhc.QuickConnectivityCheck(ctx); err != nil {
			if strings.Contains(err.Error(), "connection refused") ||
			   strings.Contains(err.Error(), "actively refused") {
				// Still might be temporary during startup, don't fail too quickly
				return false, nil
			}
		}
	}

	return false, nil
}

// retryWithBackoff performs an operation with exponential backoff retry logic
func (fhc *FastHealthChecker) retryWithBackoff(ctx context.Context, operation func() error) error {
	var lastErr error
	backoff := fhc.baseBackoff

	for attempt := 0; attempt <= fhc.maxRetries; attempt++ {
		// Try the operation
		if err := operation(); err != nil {
			lastErr = err
			
			// If this was the last attempt, return the error
			if attempt == fhc.maxRetries {
				return fmt.Errorf("operation failed after %d attempts: %w", fhc.maxRetries+1, lastErr)
			}
			
			// Wait with exponential backoff before retrying
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
				// Double the backoff for next attempt, with maximum of 2 seconds
				backoff *= 2
				if backoff > 2*time.Second {
					backoff = 2 * time.Second
				}
			}
		} else {
			// Operation succeeded
			return nil
		}
	}

	return lastErr
}

// extractHostPort extracts host and port from the baseURL
func (fhc *FastHealthChecker) extractHostPort() (string, string, error) {
	// Remove protocol prefix
	url := fhc.baseURL
	if strings.HasPrefix(url, "http://") {
		url = strings.TrimPrefix(url, "http://")
	} else if strings.HasPrefix(url, "https://") {
		url = strings.TrimPrefix(url, "https://")
	}

	// Split host:port
	parts := strings.Split(url, ":")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid URL format: %s", fhc.baseURL)
	}

	return parts[0], parts[1], nil
}

// Close cleans up resources used by the fast health checker
func (fhc *FastHealthChecker) Close() error {
	if transport, ok := fhc.client.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}
	return nil
}