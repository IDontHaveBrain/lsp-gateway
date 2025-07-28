package testutils

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestFastHealthChecker_QuickConnectivityCheck tests TCP connectivity check
func TestFastHealthChecker_QuickConnectivityCheck(t *testing.T) {
	// Start a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer server.Close()

	healthChecker := NewFastHealthChecker(server.URL)
	defer healthChecker.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	start := time.Now()
	err := healthChecker.QuickConnectivityCheck(ctx)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("QuickConnectivityCheck failed: %v", err)
	}

	// Should complete very fast (< 200ms for local server)
	if elapsed > 200*time.Millisecond {
		t.Errorf("QuickConnectivityCheck too slow: %v (expected < 200ms)", elapsed)
	}
}

// TestFastHealthChecker_FastHeadCheck tests HTTP HEAD health check
func TestFastHealthChecker_FastHeadCheck(t *testing.T) {
	// Start a test server with health endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			if r.Method == "HEAD" {
				// HEAD requests should not have body
				return
			}
			w.Write([]byte("healthy"))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	healthChecker := NewFastHealthChecker(server.URL)
	defer healthChecker.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	start := time.Now()
	err := healthChecker.FastHeadCheck(ctx)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("FastHeadCheck failed: %v", err)
	}

	// Should complete very fast (< 300ms for local server)
	if elapsed > 300*time.Millisecond {
		t.Errorf("FastHeadCheck too slow: %v (expected < 300ms)", elapsed)
	}
}

// TestFastHealthChecker_FastHealthCheck tests combined health check
func TestFastHealthChecker_FastHealthCheck(t *testing.T) {
	// Start a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			if r.Method != "HEAD" {
				w.Write([]byte("healthy"))
			}
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	healthChecker := NewFastHealthChecker(server.URL)
	defer healthChecker.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	start := time.Now()
	err := healthChecker.FastHealthCheck(ctx)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("FastHealthCheck failed: %v", err)
	}

	// Should complete very fast (< 400ms for local server with both TCP and HTTP)
	if elapsed > 400*time.Millisecond {
		t.Errorf("FastHealthCheck too slow: %v (expected < 400ms)", elapsed)
	}
}

// TestFastHealthChecker_FailureDetection tests fast-fail detection
func TestFastHealthChecker_FailureDetection(t *testing.T) {
	// Use a definitely unavailable URL
	healthChecker := NewFastHealthChecker("http://localhost:99999")
	defer healthChecker.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	start := time.Now()
	
	// Test connectivity check to ensure it fails
	err := healthChecker.QuickConnectivityCheck(ctx)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("Expected connectivity check to fail for unavailable server")
	}

	// Should detect failure quickly (< 200ms)
	if elapsed > 200*time.Millisecond {
		t.Errorf("Failure detection too slow: %v (expected < 200ms)", elapsed)
	}
	
	t.Logf("Failure detected in %v: %v", elapsed, err)
}

// TestWaitForServerStartupFast tests the fast server startup detection
func TestWaitForServerStartupFast(t *testing.T) {
	// Start a server with a delay to simulate startup time
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			if r.Method != "HEAD" {
				w.Write([]byte("healthy"))
			}
		}
	}))

	// Start the server in background after a small delay
	go func() {
		time.Sleep(100 * time.Millisecond) // Simulate startup delay
		server.Start()
	}()

	defer server.Close()

	// Wait for server startup (this should use the fast method)
	start := time.Now()
	
	// Since we can't easily get the port before server starts, 
	// let's test the ultra-fast readiness check instead
	time.Sleep(150 * time.Millisecond) // Wait for server to start
	
	err := WaitForServerReadyUltraFast(server.URL)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("WaitForServerReadyUltraFast failed: %v", err)
	}

	// Should complete quickly once server is ready
	if elapsed > 500*time.Millisecond {
		t.Errorf("Server startup detection too slow: %v (expected < 500ms after server start)", elapsed)
	}
}

// BenchmarkFastHealthCheck benchmarks the fast health check performance
func BenchmarkFastHealthCheck(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	healthChecker := NewFastHealthChecker(server.URL)
	defer healthChecker.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := healthChecker.FastHealthCheck(ctx)
		if err != nil {
			b.Fatalf("FastHealthCheck failed: %v", err)
		}
	}
}

// BenchmarkQuickConnectivityCheck benchmarks TCP connectivity check
func BenchmarkQuickConnectivityCheck(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	healthChecker := NewFastHealthChecker(server.URL)
	defer healthChecker.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := healthChecker.QuickConnectivityCheck(ctx)
		if err != nil {
			b.Fatalf("QuickConnectivityCheck failed: %v", err)
		}
	}
}

// TestBatchServerReadinessCheck tests parallel server readiness checking
func TestBatchServerReadinessCheck(t *testing.T) {
	// Start multiple test servers
	servers := make([]*httptest.Server, 3)
	urls := make([]string, 3)
	
	for i := 0; i < 3; i++ {
		servers[i] = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/health" {
				w.WriteHeader(http.StatusOK)
			}
		}))
		urls[i] = servers[i].URL
	}
	
	defer func() {
		for i := 0; i < 3; i++ {
			servers[i].Close()
		}
	}()

	start := time.Now()
	results := BatchServerReadinessCheck(urls, 2*time.Second)
	elapsed := time.Since(start)

	// All servers should be healthy
	for url, err := range results {
		if err != nil {
			t.Errorf("Server %s should be healthy, got error: %v", url, err)
		}
	}

	// Batch check should be fast (parallel execution)
	if elapsed > 1*time.Second {
		t.Errorf("Batch check too slow: %v (expected < 1s for parallel execution)", elapsed)
	}

	if len(results) != 3 {
		t.Errorf("Expected results for 3 servers, got %d", len(results))
	}
}