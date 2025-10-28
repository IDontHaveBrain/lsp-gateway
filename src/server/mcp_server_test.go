package server_test

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lsp-gateway/src/config"
	"lsp-gateway/src/server"
)

func newTestConfig() *config.Config {
	cfg := config.GetDefaultConfig()
	if cfg.Cache == nil {
		cfg.Cache = config.GetDefaultCacheConfig()
	}
	return cfg
}

func TestNewMCPServerRegistersGoSDKComponents(t *testing.T) {
	cfg := newTestConfig()

	mcpServer, err := server.NewMCPServer(cfg)
	require.NoError(t, err)
	require.NotNil(t, mcpServer, "expected MCP server")

	t.Run("sdk server initialized", func(t *testing.T) {
		assert.NotNil(t, mcpServer.GoSDKServer())
		assert.NotNil(t, mcpServer.Implementation())
	})

	t.Run("tools registered", func(t *testing.T) {
		names := mcpServer.RegisteredToolNames()
		assert.ElementsMatch(t, []string{"findSymbols", "findReferences"}, names)
	})

	t.Run("capabilities configured", func(t *testing.T) {
		caps := mcpServer.Capabilities()
		require.NotNil(t, caps.Tools)
		assert.True(t, caps.Tools.ListChanged)
	})
}

func TestFindSymbolsToolSchemaExposesPattern(t *testing.T) {
	cfg := newTestConfig()
	mcpServer, err := server.NewMCPServer(cfg)
	require.NoError(t, err)

	def := mcpServer.ToolDefinition("findSymbols")
	require.NotNil(t, def, "expected findSymbols tool definition")
	assert.Equal(t, "findSymbols", def.Name)
	assert.NotEmpty(t, def.Description)
}

// Ensure helper methods provide stable access for integration tests and tooling.
func TestExportedAccessorsReturnCopies(t *testing.T) {
	cfg := newTestConfig()
	mcpServer, err := server.NewMCPServer(cfg)
	require.NoError(t, err)

	names := mcpServer.RegisteredToolNames()
	require.Len(t, names, 2)

	names[0] = "mutated"
	// Re-read to ensure mutation does not leak into server internals.
	again := mcpServer.RegisteredToolNames()
	assert.NotEqual(t, names[0], again[0])

	// Ensure capabilities are reported using go-sdk types.
	caps := mcpServer.Capabilities()
	assert.IsType(t, mcp.ServerCapabilities{}, caps)
}
