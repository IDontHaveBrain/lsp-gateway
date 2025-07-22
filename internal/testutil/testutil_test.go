package testutil

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// Test TempDir function
func TestTempDir(t *testing.T) {
	// Test basic functionality
	t.Run("BasicCreation", func(t *testing.T) {
		dir := TempDir(t)

		// Verify directory exists
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("TempDir did not create directory: %s", dir)
		}

		// Verify it's in temp directory
		tempDir := os.TempDir()
		if !strings.HasPrefix(dir, tempDir) {
			t.Errorf("TempDir not created in system temp dir. Expected prefix %s, got %s", tempDir, dir)
		}

		// Verify directory name pattern
		baseName := filepath.Base(dir)
		if !strings.HasPrefix(baseName, "lsp-gateway-test-") {
			t.Errorf("TempDir name doesn't match expected pattern. Got: %s", baseName)
		}
	})

	// Test uniqueness across multiple calls
	t.Run("Uniqueness", func(t *testing.T) {
		dirs := make(map[string]bool)
		for i := 0; i < 10; i++ {
			dir := TempDir(t)
			if dirs[dir] {
				t.Errorf("TempDir returned duplicate directory: %s", dir)
			}
			dirs[dir] = true
		}
	})

	// Test permissions
	t.Run("Permissions", func(t *testing.T) {
		dir := TempDir(t)

		// Check if we can write to the directory
		testFile := filepath.Join(dir, "test.txt")
		err := os.WriteFile(testFile, []byte("test"), 0644)
		if err != nil {
			t.Errorf("Cannot write to temp directory: %v", err)
		}

		// Check if we can read from the directory
		_, err = os.ReadFile(testFile)
		if err != nil {
			t.Errorf("Cannot read from temp directory: %v", err)
		}
	})

	// Test cleanup behavior - we can't easily test automatic cleanup
	// but we can test manual cleanup works
	t.Run("ManualCleanup", func(t *testing.T) {
		// Create temp dir manually to test cleanup
		tmpdir, err := os.MkdirTemp("", "lsp-gateway-test-manual-*")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}

		// Verify it exists
		if _, err := os.Stat(tmpdir); os.IsNotExist(err) {
			t.Errorf("Manually created temp dir doesn't exist: %s", tmpdir)
		}

		// Clean it up
		err = os.RemoveAll(tmpdir)
		if err != nil {
			t.Errorf("Failed to clean up temp dir: %v", err)
		}

		// Verify it's gone
		if _, err := os.Stat(tmpdir); !os.IsNotExist(err) {
			t.Errorf("Temp dir still exists after cleanup: %s", tmpdir)
		}
	})
}

// Test AllocateTestPort function
func TestAllocateTestPort(t *testing.T) {
	// Test basic functionality
	t.Run("BasicAllocation", func(t *testing.T) {
		port := AllocateTestPort(t)

		// Verify port is in valid range
		if port <= 0 || port > 65535 {
			t.Errorf("AllocateTestPort returned invalid port: %d", port)
		}

		// Verify we can bind to this port (briefly)
		addr := fmt.Sprintf("localhost:%d", port)
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			t.Errorf("Cannot bind to allocated port %d: %v", port, err)
		} else {
			_ = listener.Close()
		}
	})

	// Test uniqueness across multiple calls
	t.Run("Uniqueness", func(t *testing.T) {
		ports := make(map[int]bool)
		for i := 0; i < 10; i++ {
			port := AllocateTestPort(t)
			if ports[port] {
				t.Errorf("AllocateTestPort returned duplicate port: %d", port)
			}
			ports[port] = true
		}
	})

	// Test concurrent allocation
	t.Run("ConcurrentAllocation", func(t *testing.T) {
		const numGoroutines = 5
		portsChan := make(chan int, numGoroutines)
		var wg sync.WaitGroup

		// Start multiple goroutines allocating ports
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				port := AllocateTestPort(t)
				portsChan <- port
			}()
		}

		// Wait for all to complete
		wg.Wait()
		close(portsChan)

		// Collect and verify all ports are unique
		ports := make(map[int]bool)
		for port := range portsChan {
			if ports[port] {
				t.Errorf("Concurrent AllocateTestPort returned duplicate port: %d", port)
			}
			ports[port] = true
		}

		if len(ports) != numGoroutines {
			t.Errorf("Expected %d unique ports, got %d", numGoroutines, len(ports))
		}
	})
}

// Test AllocateTestPortBench function
func TestAllocateTestPortBench(t *testing.T) {
	// Test basic functionality (similar to AllocateTestPort but for benchmarks)
	t.Run("BasicAllocation", func(t *testing.T) {
		// Create a mock benchmark
		b := &testing.B{}
		port := AllocateTestPortBench(b)

		// Verify port is in valid range
		if port <= 0 || port > 65535 {
			t.Errorf("AllocateTestPortBench returned invalid port: %d", port)
		}

		// Verify we can bind to this port
		addr := fmt.Sprintf("localhost:%d", port)
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			t.Errorf("Cannot bind to allocated benchmark port %d: %v", port, err)
		} else {
			_ = listener.Close()
		}
	})

	// Test uniqueness
	t.Run("Uniqueness", func(t *testing.T) {
		b := &testing.B{}
		ports := make(map[int]bool)
		for i := 0; i < 5; i++ {
			port := AllocateTestPortBench(b)
			if ports[port] {
				t.Errorf("AllocateTestPortBench returned duplicate port: %d", port)
			}
			ports[port] = true
		}
	})
}

// Test CreateConfigWithPort function
func TestCreateConfigWithPort(t *testing.T) {
	// Test basic functionality
	t.Run("BasicConfig", func(t *testing.T) {
		port := 8080
		config := CreateConfigWithPort(port)

		// Verify config is not empty
		if config == "" {
			t.Error("CreateConfigWithPort returned empty config")
		}

		// Verify port is included in config
		if !strings.Contains(config, fmt.Sprintf("port: %d", port)) {
			t.Errorf("Config doesn't contain expected port. Config: %s", config)
		}

		// Verify config contains servers section
		if !strings.Contains(config, "servers:") {
			t.Errorf("Config doesn't contain servers section. Config: %s", config)
		}

		// Verify config contains go-lsp server
		if !strings.Contains(config, "go-lsp") {
			t.Errorf("Config doesn't contain go-lsp server. Config: %s", config)
		}
	})

	// Test YAML validity
	t.Run("ValidYAML", func(t *testing.T) {
		port := 9090
		config := CreateConfigWithPort(port)

		// Parse as YAML to verify syntax
		var parsed map[string]interface{}
		err := yaml.Unmarshal([]byte(config), &parsed)
		if err != nil {
			t.Errorf("Config is not valid YAML: %v. Config: %s", err, config)
		}

		// Verify port is correctly parsed
		if parsedPort, ok := parsed["port"].(int); !ok || parsedPort != port {
			t.Errorf("Port not correctly parsed from YAML. Expected: %d, Got: %v", port, parsed["port"])
		}

		// Verify servers section exists and is array
		if servers, ok := parsed["servers"].([]interface{}); !ok {
			t.Errorf("Servers section is not an array: %v", parsed["servers"])
		} else if len(servers) == 0 {
			t.Error("Servers array is empty")
		}
	})

	// Test different port values
	t.Run("DifferentPorts", func(t *testing.T) {
		testPorts := []int{1, 80, 443, 8080, 9000, 65535}

		for _, port := range testPorts {
			config := CreateConfigWithPort(port)
			if !strings.Contains(config, fmt.Sprintf("port: %d", port)) {
				t.Errorf("Config for port %d doesn't contain correct port declaration", port)
			}

			// Verify YAML is still valid
			var parsed map[string]interface{}
			err := yaml.Unmarshal([]byte(config), &parsed)
			if err != nil {
				t.Errorf("Config for port %d is not valid YAML: %v", port, err)
			}
		}
	})
}

// Test CreateMinimalConfigWithPort function
func TestCreateMinimalConfigWithPort(t *testing.T) {
	// Test basic functionality
	t.Run("BasicMinimalConfig", func(t *testing.T) {
		port := 3000
		config := CreateMinimalConfigWithPort(port)

		// Verify config is not empty
		if config == "" {
			t.Error("CreateMinimalConfigWithPort returned empty config")
		}

		// Verify port is included
		if !strings.Contains(config, fmt.Sprintf("port: %d", port)) {
			t.Errorf("Minimal config doesn't contain expected port. Config: %s", config)
		}

		// Verify it contains empty servers array
		if !strings.Contains(config, "servers: []") {
			t.Errorf("Minimal config doesn't contain empty servers array. Config: %s", config)
		}
	})

	// Test YAML validity
	t.Run("ValidYAML", func(t *testing.T) {
		port := 4000
		config := CreateMinimalConfigWithPort(port)

		// Parse as YAML to verify syntax
		var parsed map[string]interface{}
		err := yaml.Unmarshal([]byte(config), &parsed)
		if err != nil {
			t.Errorf("Minimal config is not valid YAML: %v. Config: %s", err, config)
		}

		// Verify port is correctly parsed
		if parsedPort, ok := parsed["port"].(int); !ok || parsedPort != port {
			t.Errorf("Port not correctly parsed from minimal YAML. Expected: %d, Got: %v", port, parsed["port"])
		}

		// Verify servers is empty array
		if servers, ok := parsed["servers"].([]interface{}); !ok {
			t.Errorf("Servers section is not an array in minimal config: %v", parsed["servers"])
		} else if len(servers) != 0 {
			t.Errorf("Servers array should be empty in minimal config, got: %v", servers)
		}
	})

	// Test minimality - should be shorter than full config
	t.Run("Minimality", func(t *testing.T) {
		port := 5000
		fullConfig := CreateConfigWithPort(port)
		minimalConfig := CreateMinimalConfigWithPort(port)

		if len(minimalConfig) >= len(fullConfig) {
			t.Errorf("Minimal config should be shorter than full config. Minimal: %d chars, Full: %d chars",
				len(minimalConfig), len(fullConfig))
		}
	})

	// Test different port values
	t.Run("DifferentPorts", func(t *testing.T) {
		testPorts := []int{1, 8080, 9090, 65535}

		for _, port := range testPorts {
			config := CreateMinimalConfigWithPort(port)
			if !strings.Contains(config, fmt.Sprintf("port: %d", port)) {
				t.Errorf("Minimal config for port %d doesn't contain correct port declaration", port)
			}

			// Verify YAML is still valid
			var parsed map[string]interface{}
			err := yaml.Unmarshal([]byte(config), &parsed)
			if err != nil {
				t.Errorf("Minimal config for port %d is not valid YAML: %v", port, err)
			}
		}
	})
}

// Benchmark tests for performance validation
func BenchmarkTempDir(b *testing.B) {
	for i := 0; i < b.N; i++ {
		// Create temp dir manually since we can't use TempDir in benchmark
		tmpdir, err := os.MkdirTemp("", "lsp-gateway-bench-*")
		if err != nil {
			b.Fatalf("Failed to create temp dir: %v", err)
		}
		// Clean up immediately for benchmark
		_ = os.RemoveAll(tmpdir)
	}
}

func BenchmarkAllocateTestPortBench(b *testing.B) {
	for i := 0; i < b.N; i++ {
		AllocateTestPortBench(b)
	}
}

func BenchmarkCreateConfigWithPort(b *testing.B) {
	port := 8080
	for i := 0; i < b.N; i++ {
		_ = CreateConfigWithPort(port)
	}
}

func BenchmarkCreateMinimalConfigWithPort(b *testing.B) {
	port := 8080
	for i := 0; i < b.N; i++ {
		_ = CreateMinimalConfigWithPort(port)
	}
}

// Edge case and error handling tests
func TestEdgeCases(t *testing.T) {
	// Test extreme port values for config generation
	t.Run("ExtremePortValues", func(t *testing.T) {
		extremePorts := []int{0, 1, 65535, 99999, -1}

		for _, port := range extremePorts {
			// Test full config
			fullConfig := CreateConfigWithPort(port)
			if fullConfig == "" {
				t.Errorf("CreateConfigWithPort returned empty config for port %d", port)
			}

			// Test minimal config
			minimalConfig := CreateMinimalConfigWithPort(port)
			if minimalConfig == "" {
				t.Errorf("CreateMinimalConfigWithPort returned empty config for port %d", port)
			}

			// Both should contain the port value (even if invalid)
			portStr := fmt.Sprintf("port: %d", port)
			if !strings.Contains(fullConfig, portStr) {
				t.Errorf("Full config doesn't contain port %d", port)
			}
			if !strings.Contains(minimalConfig, portStr) {
				t.Errorf("Minimal config doesn't contain port %d", port)
			}
		}
	})

	// Test rapid port allocation
	t.Run("RapidPortAllocation", func(t *testing.T) {
		ports := make([]int, 0, 20)

		// Rapidly allocate many ports
		start := time.Now()
		for i := 0; i < 20; i++ {
			port := AllocateTestPort(t)
			ports = append(ports, port)
		}
		duration := time.Since(start)

		// Should complete quickly (less than 1 second for 20 ports)
		if duration > time.Second {
			t.Errorf("Rapid port allocation took too long: %v", duration)
		}

		// All ports should be unique
		portSet := make(map[int]bool)
		for _, port := range ports {
			if portSet[port] {
				t.Errorf("Duplicate port found in rapid allocation: %d", port)
			}
			portSet[port] = true
		}
	})
}
