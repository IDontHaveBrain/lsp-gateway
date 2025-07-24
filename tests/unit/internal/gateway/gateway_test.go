package gateway_test

import (
	"context"
	"errors"
	"lsp-gateway/internal/gateway"
	"testing"
	"time"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/transport"
)

func TestNewGateway(t *testing.T) {
	t.Parallel()
	mockClientFactory, _ := createMockClientFactory(t)

	t.Run("ValidConfig", func(t *testing.T) {
		testValidGatewayConfig(t, mockClientFactory)
	})

	t.Run("EmptyConfig", func(t *testing.T) {
		testEmptyGatewayConfig(t, mockClientFactory)
	})

	t.Run("ClientCreationError", func(t *testing.T) {
		testClientCreationError(t)
	})
}

func testValidGatewayConfig(t *testing.T, mockClientFactory func(transport.ClientConfig) (transport.LSPClient, error)) {
	config := createTestConfig()
	testableGateway, err := NewTestableGateway(config, mockClientFactory)

	if err != nil {
		t.Fatalf("NewTestableGateway() failed: %v", err)
	}

	gateway := testableGateway.Gateway
	validateGatewayBasics(t, gateway, config)
	validateGatewayClients(t, gateway)
	validateRouterConfiguration(t, gateway)
}

func testEmptyGatewayConfig(t *testing.T, mockClientFactory func(transport.ClientConfig) (transport.LSPClient, error)) {
	config := &config.GatewayConfig{
		Port:    8080,
		Servers: []config.ServerConfig{},
	}

	testableGateway, err := NewTestableGateway(config, mockClientFactory)

	if err != nil {
		t.Fatalf("NewTestableGateway() failed: %v", err)
	}

	gateway := testableGateway.Gateway

	if gateway == nil {
		t.Fatal("NewTestableGateway() returned nil gateway")
	}

	if len(gateway.Clients) != 0 {
		t.Fatalf("Expected 0 clients, got %d", len(gateway.Clients))
	}
}

func testClientCreationError(t *testing.T) {
	errorClientFactory := func(config transport.ClientConfig) (transport.LSPClient, error) {
		return nil, errors.New("mock client creation error")
	}

	config := createSingleServerConfig()
	testableGateway, err := NewTestableGateway(config, errorClientFactory)

	if err == nil {
		t.Fatal("NewTestableGateway() should have failed")
	}

	if testableGateway != nil {
		t.Fatal("NewTestableGateway() should return nil on error")
	}

	expectedError := "failed to create client for go-lsp"
	if err.Error()[:len(expectedError)] != expectedError {
		t.Fatalf("Expected error to start with %s, got %s", expectedError, err.Error())
	}
}

func validateGatewayBasics(t *testing.T, gateway *gateway.Gateway, config *config.GatewayConfig) {
	if gateway == nil {
		t.Fatal("NewGateway() returned nil gateway")
	}

	if gateway.Config != config {
		t.Fatal("Gateway config not set correctly")
	}

	if gateway.Clients == nil {
		t.Fatal("Gateway clients map is nil")
	}

	if gateway.Router == nil {
		t.Fatal("Gateway router is nil")
	}
}

func validateGatewayClients(t *testing.T, gateway *gateway.Gateway) {
	expectedServers := []string{"go-lsp", "python-lsp", "typescript-lsp"}
	for _, serverName := range expectedServers {
		client, exists := gateway.GetClient(serverName)
		if !exists {
			t.Fatalf("Client %s not found", serverName)
		}
		if client == nil {
			t.Fatalf("Client %s is nil", serverName)
		}
	}
}

func validateRouterConfiguration(t *testing.T, gateway *gateway.Gateway) {
	server, err := gateway.Router.RouteRequest("file:///test.go")
	if err != nil {
		t.Fatalf("Router not configured correctly: %v", err)
	}
	if server != "go-lsp" {
		t.Fatalf("Expected go-lsp, got %s", server)
	}
}

func TestGatewayStart(t *testing.T) {
	t.Parallel()
	mockClientFactory, mockClients := createMockClientFactory(t)

	t.Run("StartAllClients", func(t *testing.T) {
		config := createTestConfig()
		testableGateway, err := NewTestableGateway(config, mockClientFactory)
		if err != nil {
			t.Fatalf("NewTestableGateway() failed: %v", err)
		}

		gateway := testableGateway.Gateway

		ctx := context.Background()
		err = gateway.Start(ctx)

		if err != nil {
			t.Fatalf("Gateway.Start() failed: %v", err)
		}

		for command, mock := range mockClients {
			if mock.GetStartCount() != 1 {
				t.Fatalf("Client %s Start() called %d times, expected 1", command, mock.GetStartCount())
			}
			if !mock.IsActive() {
				t.Fatalf("Client %s should be active", command)
			}
		}
	})

	t.Run("StartWithClientError", func(t *testing.T) {
		mockClientsLocal := make(map[string]*MockLSPClient)
		errorClientFactory := func(config transport.ClientConfig) (transport.LSPClient, error) {
			mock := NewMockLSPClient()
			mockClientsLocal[config.Command] = mock
			return mock, nil
		}

		config := createTestConfig()
		testableGateway, err := NewTestableGateway(config, errorClientFactory)
		if err != nil {
			t.Fatalf("NewTestableGateway() failed: %v", err)
		}

		gateway := testableGateway.Gateway

		var firstMock *MockLSPClient
		for _, mock := range mockClientsLocal {
			firstMock = mock
			break
		}
		firstMock.SetStartError(errors.New("mock start error"))

		ctx := context.Background()
		err = gateway.Start(ctx)

		if err == nil {
			t.Fatal("Gateway.Start() should have failed")
		}

		expectedError := "failed to start client"
		if err.Error()[:len(expectedError)] != expectedError {
			t.Fatalf("Expected error to start with %s, got %s", expectedError, err.Error())
		}
	})

	t.Run("StartEmptyGateway", func(t *testing.T) {
		config := &config.GatewayConfig{
			Port:    8080,
			Servers: []config.ServerConfig{},
		}
		testableGateway, err := NewTestableGateway(config, mockClientFactory)
		if err != nil {
			t.Fatalf("NewTestableGateway() failed: %v", err)
		}

		gateway := testableGateway.Gateway

		ctx := context.Background()
		err = gateway.Start(ctx)

		if err != nil {
			t.Fatalf("Gateway.Start() should not fail for empty gateway: %v", err)
		}
	})
}

func TestGatewayStop(t *testing.T) {
	t.Parallel()
	mockClientFactory, mockClients := createMockClientFactory(t)

	t.Run("StopAllClients", func(t *testing.T) {
		config := createTestConfig()
		testableGateway, err := NewTestableGateway(config, mockClientFactory)
		if err != nil {
			t.Fatalf("NewTestableGateway() failed: %v", err)
		}

		gateway := testableGateway.Gateway

		ctx := context.Background()
		if err := gateway.Start(ctx); err != nil {
			t.Fatalf("Gateway.Start() failed: %v", err)
		}

		err = gateway.Stop()

		if err != nil {
			t.Fatalf("Gateway.Stop() failed: %v", err)
		}

		for command, mock := range mockClients {
			if mock.GetStopCount() != 1 {
				t.Fatalf("Client %s Stop() called %d times, expected 1", command, mock.GetStopCount())
			}
			if mock.IsActive() {
				t.Fatalf("Client %s should not be active", command)
			}
		}
	})

	t.Run("StopWithClientError", func(t *testing.T) {
		mockClientsLocal := make(map[string]*MockLSPClient)
		errorClientFactory := func(config transport.ClientConfig) (transport.LSPClient, error) {
			mock := NewMockLSPClient()
			mockClientsLocal[config.Command] = mock
			return mock, nil
		}

		config := createTestConfig()
		testableGateway, err := NewTestableGateway(config, errorClientFactory)
		if err != nil {
			t.Fatalf("NewTestableGateway() failed: %v", err)
		}

		gateway := testableGateway.Gateway

		var firstMock *MockLSPClient
		for _, mock := range mockClientsLocal {
			firstMock = mock
			break
		}
		firstMock.SetStopError(errors.New("mock stop error"))

		err = gateway.Stop()

		if err == nil {
			t.Fatal("Gateway.Stop() should have failed")
		}

		expectedError := "failed to stop client"
		if err.Error()[:len(expectedError)] != expectedError {
			t.Fatalf("Expected error to start with %s, got %s", expectedError, err.Error())
		}
	})

	t.Run("StopEmptyGateway", func(t *testing.T) {
		config := &config.GatewayConfig{
			Port:    8080,
			Servers: []config.ServerConfig{},
		}
		testableGateway, err := NewTestableGateway(config, mockClientFactory)
		if err != nil {
			t.Fatalf("NewTestableGateway() failed: %v", err)
		}

		gateway := testableGateway.Gateway

		err = gateway.Stop()

		if err != nil {
			t.Fatalf("Gateway.Stop() should not fail for empty gateway: %v", err)
		}
	})
}

func TestGatewayGetClient(t *testing.T) {
	t.Parallel()
	mockClientFactory, _ := createMockClientFactory(t)

	t.Run("GetExistingClient", func(t *testing.T) {
		config := createTestConfig()
		testableGateway, err := NewTestableGateway(config, mockClientFactory)
		if err != nil {
			t.Fatalf("NewTestableGateway() failed: %v", err)
		}

		gateway := testableGateway.Gateway

		client, exists := gateway.GetClient("go-lsp")

		if !exists {
			t.Fatal("GetClient() should find go-lsp client")
		}

		if client == nil {
			t.Fatal("GetClient() returned nil client")
		}

		if _, ok := client.(*MockLSPClient); !ok {
			t.Fatal("GetClient() returned wrong client type")
		}
	})

	t.Run("GetNonexistentClient", func(t *testing.T) {
		config := createTestConfig()
		testableGateway, err := NewTestableGateway(config, mockClientFactory)
		if err != nil {
			t.Fatalf("NewTestableGateway() failed: %v", err)
		}

		gateway := testableGateway.Gateway

		client, exists := gateway.GetClient("nonexistent")

		if exists {
			t.Fatal("GetClient() should not find nonexistent client")
		}

		if client != nil {
			t.Fatal("GetClient() should return nil for nonexistent client")
		}
	})

	t.Run("GetAllConfiguredClients", func(t *testing.T) {
		config := createTestConfig()
		testableGateway, err := NewTestableGateway(config, mockClientFactory)
		if err != nil {
			t.Fatalf("NewTestableGateway() failed: %v", err)
		}

		gateway := testableGateway.Gateway

		expectedClients := []string{"go-lsp", "python-lsp", "typescript-lsp"}

		for _, serverName := range expectedClients {
			client, exists := gateway.GetClient(serverName)

			if !exists {
				t.Fatalf("GetClient() should find %s client", serverName)
			}

			if client == nil {
				t.Fatalf("GetClient() returned nil for %s client", serverName)
			}
		}
	})
}

func TestGatewayLifecycleIntegration(t *testing.T) {
	t.Parallel()
	mockClientFactory, mockClients := createMockClientFactory(t)

	t.Run("FullLifecycle", func(t *testing.T) {
		config := createTestConfig()

		testableGateway, err := NewTestableGateway(config, mockClientFactory)
		if err != nil {
			t.Fatalf("NewTestableGateway() failed: %v", err)
		}

		gateway := testableGateway.Gateway

		client, exists := gateway.GetClient("go-lsp")
		if !exists {
			t.Fatal("go-lsp client not found after creation")
		}
		if client.IsActive() {
			t.Fatal("Client should not be active before Start()")
		}

		ctx := context.Background()
		if err := gateway.Start(ctx); err != nil {
			t.Fatalf("Gateway.Start() failed: %v", err)
		}

		if !client.IsActive() {
			t.Fatal("Client should be active after Start()")
		}

		if err := gateway.Stop(); err != nil {
			t.Fatalf("Gateway.Stop() failed: %v", err)
		}

		if client.IsActive() {
			t.Fatal("Client should not be active after Stop()")
		}

		mock := mockClients["gopls"]
		if mock.GetStartCount() != 1 {
			t.Fatalf("Expected 1 Start() call, got %d", mock.GetStartCount())
		}
		if mock.GetStopCount() != 1 {
			t.Fatalf("Expected 1 Stop() call, got %d", mock.GetStopCount())
		}
	})
}

func TestGatewayConcurrentAccess(t *testing.T) {
	mockClientFactory, _ := createMockClientFactory(t)

	config := createTestConfig()
	testableGateway, err := NewTestableGateway(config, mockClientFactory)
	if err != nil {
		t.Fatalf("NewTestableGateway() failed: %v", err)
	}

	gateway := testableGateway.Gateway

	done := make(chan bool)
	const numGoroutines = 10
	const numIterations = 100

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer func() { done <- true }()

			for j := 0; j < numIterations; j++ {
				client, exists := gateway.GetClient("go-lsp")
				if !exists || client == nil {
					t.Errorf("GetClient failed in concurrent test")
					return
				}

				_, exists = gateway.GetClient("nonexistent")
				if exists {
					t.Errorf("GetClient should not find nonexistent client")
					return
				}
			}
		}()
	}

	for i := 0; i < numGoroutines; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("Concurrent test timed out")
		}
	}

	t.Log("Concurrent access test passed")
}

func TestGatewayStartStopConcurrency(t *testing.T) {
	mockClientFactory, _ := createMockClientFactory(t)

	config := createSingleServerConfig()
	testableGateway, err := NewTestableGateway(config, mockClientFactory)
	if err != nil {
		t.Fatalf("NewTestableGateway() failed: %v", err)
	}

	gateway := testableGateway.Gateway

	const numOperations = 5
	done := make(chan error, numOperations*2)

	for i := 0; i < numOperations; i++ {
		go func() {
			ctx := context.Background()
			done <- gateway.Start(ctx)
		}()
	}

	for i := 0; i < numOperations; i++ {
		go func() {
			done <- gateway.Stop()
		}()
	}

	for i := 0; i < numOperations*2; i++ {
		select {
		case err := <-done:
			_ = err
		case <-time.After(5 * time.Second):
			t.Fatal("Concurrent start/stop test timed out")
		}
	}

	t.Log("Concurrent start/stop test passed")
}
