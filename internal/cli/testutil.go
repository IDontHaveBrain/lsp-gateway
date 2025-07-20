package cli

import (
	"fmt"
	"net"
	"testing"
)

const (
	DefaultMCPPort = 3000 // Default MCP server port for testing
)

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

func CreateMinimalConfigWithPort(port int) string {
	return fmt.Sprintf(`port: %d
servers: []
`, port)
}
