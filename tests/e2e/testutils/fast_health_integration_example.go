package testutils

import (
	"context"
	"fmt"
	"time"
)

// Example: Integration patterns for using faster health checks in E2E tests

// FastServerReadyExample demonstrates how to use ultra-fast server readiness detection
func FastServerReadyExample() {
	// Before: Using slow polling with 100ms intervals
	// err := WaitForServerReady("http://localhost:8080") // Takes 15s timeout with 100ms polling
	
	// After: Using ultra-fast health checks with sub-second detection
	err := WaitForServerReadyFast("http://localhost:8080") // Sub-second detection
	if err != nil {
		fmt.Printf("Server ready check failed: %v\n", err)
		return
	}
	fmt.Println("Server ready in under 1 second!")
}

// FastServerStartupExample shows how to combine port binding check with health verification
func FastServerStartupExample(port int) {
	baseURL := fmt.Sprintf("http://localhost:%d", port)
	
	// Before: Only checking port binding (race conditions possible)
	// WaitForCondition(func() (bool, error) { return !isPortAvailable(port), nil }, ...)
	
	// After: Hybrid approach - port binding + actual HTTP health check
	err := WaitForServerStartup(baseURL, port) // Combines port check + fast health validation
	if err != nil {
		fmt.Printf("Server startup failed: %v\n", err)
		return
	}
	fmt.Println("Server fully started and responding to requests!")
}

// QuickConnectivityTestExample demonstrates instant TCP connectivity validation
func QuickConnectivityTestExample() {
	// Before: Full HTTP requests for basic connectivity
	// httpClient.HealthCheck(ctx) // Full GET request with body parsing
	
	// After: Instant TCP connectivity test
	err := QuickConnectivityTest("http://localhost:8080", 100*time.Millisecond)
	if err != nil {
		fmt.Printf("Quick connectivity test failed: %v\n", err)
		return
	}
	fmt.Println("Server is accepting TCP connections!")
}

// BatchHealthCheckExample shows parallel server validation
func BatchHealthCheckExample() {
	servers := []string{
		"http://localhost:8080",
		"http://localhost:8081", 
		"http://localhost:8082",
	}
	
	// Before: Sequential health checks (slow)
	// for _, server := range servers { WaitForServerReady(server) }
	
	// After: Parallel batch checking (fast)
	results := BatchServerReadinessCheck(servers, 2*time.Second)
	
	healthy := 0
	for server, err := range results {
		if err == nil {
			healthy++
			fmt.Printf("✓ %s is healthy\n", server)
		} else {
			fmt.Printf("✗ %s failed: %v\n", server, err)
		}
	}
	fmt.Printf("Parallel check completed: %d/%d servers healthy\n", healthy, len(servers))
}

// OptimizedIsolatedTestExample shows how to use fast health checks in isolated tests
func OptimizedIsolatedTestExample() error {
	// Setup isolated test environment
	setup, err := SetupIsolatedTest("fast_health_test")
	if err != nil {
		return err
	}
	defer setup.Cleanup()
	
	// Start server (same as before)
	serverCmd := []string{"./bin/lspg", "server", "--config", setup.ConfigPath}
	if err := setup.StartServer(serverCmd, 30*time.Second); err != nil {
		return err
	}
	
	// Before: Slow server readiness check (5+ seconds typical)
	// setup.WaitForServerReady(15 * time.Second)
	
	// After: Fast server readiness check (sub-second typical)
	start := time.Now()
	if err := setup.WaitForServerReady(3 * time.Second); err != nil {
		return fmt.Errorf("server readiness check failed: %w", err)
	}
	elapsed := time.Since(start)
	
	fmt.Printf("Server ready in %v (using optimized health checks)\n", elapsed)
	
	// Continue with test logic...
	httpClient := setup.GetHTTPClient()
	
	// Use fast health check for ongoing validation
	ctx := context.Background()
	err = httpClient.FastHealthCheck(ctx)
	if err != nil {
		return fmt.Errorf("fast health check failed: %w", err)
	}
	
	return nil
}

// PerformanceComparisonExample demonstrates the speed difference
func PerformanceComparisonExample() {
	baseURL := "http://localhost:8080"
	
	// Measure old approach
	start := time.Now()
	WaitForServerReadyQuick(baseURL) // 5s timeout with 50ms polling
	oldApproachTime := time.Since(start)
	
	// Measure new approach
	start = time.Now()
	WaitForServerReadyFast(baseURL) // Sub-second detection
	newApproachTime := time.Since(start)
	
	improvement := float64(oldApproachTime) / float64(newApproachTime)
	fmt.Printf("Performance improvement: %.1fx faster\n", improvement)
	fmt.Printf("Old approach: %v\n", oldApproachTime)
	fmt.Printf("New approach: %v\n", newApproachTime)
}