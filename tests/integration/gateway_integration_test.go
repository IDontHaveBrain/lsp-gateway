package integration

import (
	"testing"
	"time"

	"lsp-gateway/tests/shared"
)

func TestGatewayIntegration(t *testing.T) {
	shared.SkipIfShortMode(t)
	shared.CheckLSPAvailability(t, "go")

	// Create test workspace with server code
	workspace := shared.CreateGoProjectWithServer(t, "testproject")
	defer workspace.Cleanup()

	// Create HTTP gateway with cache
	cacheConfig := shared.CreateLargeCacheConfig(workspace.TempDir)
	cfg := shared.CreateConfigWithCache(cacheConfig)
	gatewaySetup := shared.CreateHTTPGateway(t, ":18888", cfg)
	gatewaySetup.Start(t)
	defer gatewaySetup.Stop()

	shared.WaitForServer(3 * time.Second)

	client := shared.CreateHTTPClient(10 * time.Second)
	testFile := workspace.GetFilePath("server.go")

	t.Run("Health check", func(t *testing.T) {
		shared.CheckHealthEndpoint(t, client, gatewaySetup.ServerURL)
	})

	t.Run("TextDocument Definition via JSON-RPC", func(t *testing.T) {
		request := shared.CreateDefinitionRequestFromFile(testFile, 24, 10, 1)
		response := shared.SendJSONRPCRequest(t, client, gatewaySetup.GetJSONRPCURL(), request)
		shared.AssertJSONRPCSuccess(t, response)
	})

	t.Run("TextDocument Hover via JSON-RPC", func(t *testing.T) {
		request := shared.CreateHoverRequestFromFile(testFile, 13, 15, 2)
		response := shared.SendJSONRPCRequest(t, client, gatewaySetup.GetJSONRPCURL(), request)
		shared.AssertHoverResponse(t, response)
	})

	t.Run("TextDocument DocumentSymbol via JSON-RPC", func(t *testing.T) {
		request := shared.CreateDocumentSymbolRequestFromFile(testFile, 3)
		response := shared.SendJSONRPCRequest(t, client, gatewaySetup.GetJSONRPCURL(), request)
		shared.AssertDocumentSymbolResponse(t, response, 1)
	})

	t.Run("Workspace Symbol via JSON-RPC", func(t *testing.T) {
		request := shared.CreateWorkspaceSymbolRequest("Server", 4)
		response := shared.SendJSONRPCRequest(t, client, gatewaySetup.GetJSONRPCURL(), request)
		shared.AssertWorkspaceSymbolResponse(t, response)
	})

	t.Run("TextDocument References via JSON-RPC", func(t *testing.T) {
		request := shared.CreateReferencesRequestFromFile(testFile, 8, 5, true, 5)
		response := shared.SendJSONRPCRequest(t, client, gatewaySetup.GetJSONRPCURL(), request)
		shared.AssertReferencesResponse(t, response, 1)
	})

	t.Run("TextDocument Completion via JSON-RPC", func(t *testing.T) {
		request := shared.CreateCompletionRequestFromFile(testFile, 24, 8, 6)
		response := shared.SendJSONRPCRequest(t, client, gatewaySetup.GetJSONRPCURL(), request)
		shared.AssertCompletionResponse(t, response)
	})

	t.Run("Batch JSON-RPC requests not supported", func(t *testing.T) {
		// Server only handles single requests, not batch arrays
		hoverReq := shared.CreateHoverRequestFromFile(testFile, 13, 15, 100)
		batchRequest := shared.CreateBatchRequest(hoverReq)

		response := shared.SendJSONRPCRequestRaw(t, client, gatewaySetup.GetJSONRPCURL(), 
			map[string]interface{}{"batch": batchRequest})
		shared.AssertJSONRPCError(t, response)
	})

	t.Run("Invalid method handling", func(t *testing.T) {
		request := shared.CreateInvalidMethodRequest(999)
		response := shared.SendJSONRPCRequest(t, client, gatewaySetup.GetJSONRPCURL(), request)
		shared.AssertJSONRPCError(t, response)
	})

	t.Run("Concurrent requests", func(t *testing.T) {
		done := make(chan bool, 5)

		for i := 0; i < 5; i++ {
			go func(id int) {
				defer func() { done <- true }()

				request := shared.CreateHoverRequestFromFile(testFile, 13, 15, 1000+id)
				response := shared.SendJSONRPCRequest(t, client, gatewaySetup.GetJSONRPCURL(), request)
				shared.AssertJSONRPCSuccess(t, response)
			}(i)
		}

		for i := 0; i < 5; i++ {
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				t.Fatal("Concurrent request timeout")
			}
		}
	})
}
