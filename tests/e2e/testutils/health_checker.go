package testutils

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// QuickConnectivityCheck performs a basic connectivity check (standalone function)
func QuickConnectivityCheck(url string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("connectivity check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	return nil
}

// IsServerFailed checks if server is definitely failed
func IsServerFailed(port int) bool {
	url := fmt.Sprintf("http://localhost:%d/health", port)
	err := QuickConnectivityCheck(url)
	return err != nil
}
