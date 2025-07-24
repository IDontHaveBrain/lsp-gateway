package network_test

import (
	"lsp-gateway/internal/gateway"
	"testing"
)

func TestMinimalRepro(t *testing.T) {
	requestBody := gateway.JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "test",
	}

	var response gateway.JSONRPCResponse

	t.Logf("Request: %+v", requestBody)
	t.Logf("Response: %+v", response)
}
