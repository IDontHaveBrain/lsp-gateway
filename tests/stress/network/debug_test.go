package network_test

import (
	"lsp-gateway/internal/gateway"
	"testing"
)

// Test the exact pattern used in network_timeout_test.go
func TestDebugGatewayTypes(t *testing.T) {
	// This is the exact pattern from line 216 of network_timeout_test.go
	requestBody := gateway.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "test-multi",
		Method:  "textDocument/definition",
	}

	// This is the exact pattern from line 249 of network_timeout_test.go
	var response gateway.JSONRPCResponse

	t.Logf("Request: %+v", requestBody)
	t.Logf("Response: %+v", response)
}
