package transport

import (
	"context"
	"testing"
	"time"
)

// TestClientCreation tests client creation through the factory for all transport types
func TestClientCreation(t *testing.T) {
	tests := []struct {
		name      string
		config    ClientConfig
		shouldErr bool
	}{
		{
			name: "STDIO transport creation",
			config: ClientConfig{
				Command:   "cat",
				Args:      []string{},
				Transport: "stdio",
			},
			shouldErr: false,
		},
		{
			name: "TCP transport creation",
			config: ClientConfig{
				Command:   "localhost:8080",
				Args:      []string{},
				Transport: "tcp",
			},
			shouldErr: false,
		},
		{
			name: "Unsupported transport",
			config: ClientConfig{
				Command:   "test",
				Args:      []string{},
				Transport: "websocket",
			},
			shouldErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewLSPClient(tt.config)
			
			if tt.shouldErr {
				if err == nil {
					t.Error("Expected error for unsupported transport")
				}
				if client != nil {
					t.Error("Client should be nil for unsupported transport")
				}
				return
			}
			
			if err != nil {
				t.Fatalf("Unexpected error creating client: %v", err)
			}
			
			if client == nil {
				t.Fatal("Client should not be nil for supported transport")
			}
			
			// Test initial state
			if client.IsActive() {
				t.Error("Client should not be active initially")
			}
		})
	}
}

// TestStdioTransportIntegration tests the complete lifecycle of STDIO transport
func TestStdioTransportIntegration(t *testing.T) {
	config := ClientConfig{
		Command:   "cat",
		Args:      []string{},
		Transport: "stdio",
	}
	
	client, err := NewLSPClient(config)
	if err != nil {
		t.Fatalf("Failed to create STDIO client: %v", err)
	}
	
	// Test initial state
	if client.IsActive() {
		t.Error("Client should not be active initially")
	}
	
	// Test start
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	err = client.Start(ctx)
	if err != nil {
		t.Fatalf("STDIO client should start successfully: %v", err)
	}
	
	if !client.IsActive() {
		t.Error("STDIO client should be active after start")
	}
	
	// Test stop
	if err := client.Stop(); err != nil {
		t.Fatalf("STDIO client stop failed: %v", err)
	}
	
	if client.IsActive() {
		t.Error("STDIO client should not be active after stop")
	}
}

// TestTcpTransportIntegration tests the complete lifecycle of TCP transport
func TestTcpTransportIntegration(t *testing.T) {
	config := ClientConfig{
		Command:   "localhost:8080",
		Args:      []string{},
		Transport: "tcp",
	}
	
	client, err := NewLSPClient(config)
	if err != nil {
		t.Fatalf("Failed to create TCP client: %v", err)
	}
	
	// Test initial state
	if client.IsActive() {
		t.Error("Client should not be active initially")
	}
	
	// Test start behavior (may succeed or fail depending on whether server is running)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	err = client.Start(ctx)
	
	// TCP behavior depends on whether a server is running on localhost:8080
	if err != nil {
		// Expected case: no server running, start should fail
		if client.IsActive() {
			t.Error("TCP client should not be active after failed start")
		}
	} else {
		// Unexpected case: server might be running, but we should clean up
		t.Logf("TCP client started successfully (server might be running on localhost:8080)")
		if !client.IsActive() {
			t.Error("TCP client should be active after successful start")
		}
		
		// Clean up
		if err := client.Stop(); err != nil {
			t.Fatalf("TCP client stop failed: %v", err)
		}
		
		if client.IsActive() {
			t.Error("TCP client should not be active after stop")
		}
	}
}

// TestUnsupportedTransport tests error handling for unsupported transport types
func TestUnsupportedTransport(t *testing.T) {
	config := ClientConfig{
		Command:   "test",
		Args:      []string{},
		Transport: "websocket",
	}
	
	client, err := NewLSPClient(config)
	
	if err == nil {
		t.Error("Expected error for unsupported transport")
	}
	if client != nil {
		t.Error("Client should be nil for unsupported transport")
	}
}

// TestTransportTypeVerification verifies that the factory returns the correct transport type
func TestTransportTypeVerification(t *testing.T) {
	// Test STDIO transport returns StdioClient
	stdioConfig := ClientConfig{
		Command:   "cat",
		Transport: "stdio",
	}
	
	stdioClient, err := NewLSPClient(stdioConfig)
	if err != nil {
		t.Fatalf("Failed to create STDIO client: %v", err)
	}
	
	if _, ok := stdioClient.(*StdioClient); !ok {
		t.Error("STDIO transport should return StdioClient")
	}
	
	// Test TCP transport returns TCPClient
	tcpConfig := ClientConfig{
		Command:   "localhost:8080",
		Transport: "tcp",
	}
	
	tcpClient, err := NewLSPClient(tcpConfig)
	if err != nil {
		t.Fatalf("Failed to create TCP client: %v", err)
	}
	
	if _, ok := tcpClient.(*TCPClient); !ok {
		t.Error("TCP transport should return TCPClient")
	}
}

// TestClientInterfaceCompliance verifies that all transport implementations satisfy the LSPClient interface
func TestClientInterfaceCompliance(t *testing.T) {
	// Test that both client types implement the LSPClient interface
	var _ LSPClient = &StdioClient{}
	var _ LSPClient = &TCPClient{}
	
	// Test interface methods are callable
	configs := []ClientConfig{
		{
			Command:   "cat",
			Transport: "stdio",
		},
		{
			Command:   "localhost:8080",
			Transport: "tcp",
		},
	}
	
	for _, config := range configs {
		client, err := NewLSPClient(config)
		if err != nil {
			t.Fatalf("Failed to create client for %s: %v", config.Transport, err)
		}
		
		// Test all interface methods are callable
		if client.IsActive() {
			t.Errorf("Client should not be active initially for %s", config.Transport)
		}
		
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		
		// These should not panic (though they may return errors)
		_, _ = client.SendRequest(ctx, "test", nil)
		_ = client.SendNotification(ctx, "test", nil)
		_ = client.Stop()
	}
}

// TestConfigurationVariants tests different configuration options
func TestConfigurationVariants(t *testing.T) {
	tests := []struct {
		name   string
		config ClientConfig
		valid  bool
	}{
		{
			name: "STDIO with command and args",
			config: ClientConfig{
				Command:   "echo",
				Args:      []string{"hello", "world"},
				Transport: "stdio",
			},
			valid: true,
		},
		{
			name: "TCP with full address",
			config: ClientConfig{
				Command:   "example.com:9999",
				Transport: "tcp",
			},
			valid: true,
		},
		{
			name: "TCP with port only",
			config: ClientConfig{
				Command:   "8080",
				Transport: "tcp",
			},
			valid: true,
		},
		{
			name: "TCP with empty command",
			config: ClientConfig{
				Command:   "",
				Transport: "tcp",
			},
			valid: true, // Client creation succeeds, but Start() will fail
		},
		{
			name: "Empty transport",
			config: ClientConfig{
				Command:   "test",
				Transport: "",
			},
			valid: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewLSPClient(tt.config)
			
			if tt.valid {
				if err != nil {
					t.Errorf("Expected valid config to succeed: %v", err)
				}
				if client == nil {
					t.Error("Expected valid config to return client")
				}
			} else {
				if err == nil {
					t.Error("Expected invalid config to fail")
				}
				if client != nil {
					t.Error("Expected invalid config to return nil client")
				}
			}
		})
	}
}