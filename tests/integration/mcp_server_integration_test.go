package integration

import (
	"testing"
)

func TestMCPServerIntegration(t *testing.T) {
	t.Skip("MCP server integration tests disabled - MCP server uses continuous STDIO protocol that is difficult to test in isolation. MCP functionality is covered by unit tests and dual-protocol integration tests.")

	// This test is intentionally skipped. The MCP server is designed as a continuous
	// STDIO service that reads from stdin and writes to stdout in a blocking manner.
	// Testing this requires complex setup with pipes and goroutines that is not
	// suitable for integration tests. The MCP functionality is adequately covered by:
	// 1. Unit tests in tests/unit/server/mcp_server_test.go
	// 2. Dual-protocol integration tests that verify MCP server creation
	// 3. End-to-end tests that test the actual MCP protocol via CLI
}
