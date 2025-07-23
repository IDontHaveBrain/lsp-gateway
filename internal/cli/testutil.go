package cli

import (
	"testing"

	testutil "lsp-gateway/tests/utils/helpers"
)

const (
	DefaultMCPPort = 3000 // Default MCP server port for testing
)

// AllocateTestPort allocates a test port using the centralized testutil implementation
func AllocateTestPort(t *testing.T) int {
	return testutil.AllocateTestPort(t)
}

// AllocateTestPortBench allocates a test port for benchmarks using the centralized testutil implementation
func AllocateTestPortBench(b *testing.B) int {
	return testutil.AllocateTestPortBench(b)
}

// CreateConfigWithPort creates a config with the specified port using the centralized testutil implementation
func CreateConfigWithPort(port int) string {
	return testutil.CreateConfigWithPort(port)
}

// CreateMinimalConfigWithPort creates a minimal config with the specified port using the centralized testutil implementation
func CreateMinimalConfigWithPort(port int) string {
	return testutil.CreateMinimalConfigWithPort(port)
}
