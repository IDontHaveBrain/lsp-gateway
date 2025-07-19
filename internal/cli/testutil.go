package cli

import (
	"fmt"
	"net"
	"testing"
)

// Test port constants
const (
	DefaultMCPPort = 3000 // Default MCP server port for testing
)

// AllocateTestPort returns a dynamically allocated port for testing.
// This function ensures tests don't conflict when run in parallel.
func AllocateTestPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to allocate test port: %v", err)
	}
	defer func() {
		if err := listener.Close(); err != nil {
			t.Logf("Error closing test listener: %v", err)
		}
	}()
	return listener.Addr().(*net.TCPAddr).Port
}

// AllocateTestPortBench returns a dynamically allocated port for benchmarking.
// This function ensures benchmarks don't conflict when run in parallel.
func AllocateTestPortBench(b *testing.B) int {
	b.Helper()
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		b.Fatalf("Failed to allocate test port: %v", err)
	}
	defer func() {
		if err := listener.Close(); err != nil {
			b.Logf("Error closing test listener: %v", err)
		}
	}()
	return listener.Addr().(*net.TCPAddr).Port
}

// CreateConfigWithPort creates a test configuration with the specified port.
// This helper ensures consistent config generation across tests.
func CreateConfigWithPort(port int) string {
	return fmt.Sprintf(`port: %d
servers:
  - name: "go-lsp"
    languages: ["go"]
    command: "gopls" 
    args: []
    transport: "stdio"
`, port)
}

// CreateMinimalConfigWithPort creates a minimal test configuration with the specified port.
// Used for tests that need minimal configuration without full server definitions.
func CreateMinimalConfigWithPort(port int) string {
	return fmt.Sprintf(`port: %d
servers: []
`, port)
}
