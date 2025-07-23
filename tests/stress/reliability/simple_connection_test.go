package gateway_reliability_test

import (
	"testing"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/transport"
	testutils "lsp-gateway/tests/utils/gateway"
)

// TestSimpleConnectionFailure provides a basic test that compiles correctly
func TestSimpleConnectionFailure(t *testing.T) {
	t.Run("basic connection test", func(t *testing.T) {
		// Create a simple config
		cfg := &config.GatewayConfig{
			Port: 8080,
			Servers: []config.ServerConfig{
				{Name: "test-server", Languages: []string{"go"}, Command: "mock", Transport: "stdio"},
			},
		}

		// Create a mock client factory
		mockClientFactory := func(transport.ClientConfig) (transport.LSPClient, error) {
			return testutils.NewMockLSPClient(), nil
		}

		// Create testable gateway
		testableGateway, err := testutils.NewTestableGateway(cfg, mockClientFactory)
		if err != nil {
			t.Fatalf("Failed to create testable gateway: %v", err)
		}

		// Verify the gateway was created successfully
		if testableGateway == nil {
			t.Error("Expected non-nil testable gateway")
		}

		if testableGateway.Gateway == nil {
			t.Error("Expected non-nil gateway")
		}
	})
}