package transport

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"
)

func TestNewTCPClient(t *testing.T) {
	config := ClientConfig{
		Command:   "localhost:8080",
		Args:      []string{},
		Transport: "tcp",
	}
	
	client, err := NewTCPClient(config)
	if err != nil {
		t.Fatalf("NewTCPClient failed: %v", err)
	}
	
	if client == nil {
		t.Fatal("NewTCPClient returned nil client")
	}
	
	if client.IsActive() {
		t.Error("Client should not be active before Start() is called")
	}
}

func TestTCPClientInvalidTransport(t *testing.T) {
	config := ClientConfig{
		Command:   "localhost:8080",
		Args:      []string{},
		Transport: "stdio", // Wrong transport
	}
	
	client, err := NewTCPClient(config)
	if err == nil {
		t.Error("NewTCPClient should fail for invalid transport")
	}
	
	if client != nil {
		t.Error("NewTCPClient should return nil for invalid transport")
	}
}

func TestTCPClientAddressParsing(t *testing.T) {
	tests := []struct {
		name      string
		command   string
		expected  string
		shouldErr bool
	}{
		{
			name:      "Full address",
			command:   "localhost:8080",
			expected:  "localhost:8080",
			shouldErr: false,
		},
		{
			name:      "Port only",
			command:   "8080",
			expected:  "localhost:8080",
			shouldErr: false,
		},
		{
			name:      "Empty address",
			command:   "",
			expected:  "",
			shouldErr: true,
		},
		{
			name:      "Invalid format",
			command:   "invalidaddress",
			expected:  "",
			shouldErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := ClientConfig{
				Command:   tt.command,
				Transport: "tcp",
			}
			
			client, err := NewTCPClient(config)
			if err != nil {
				t.Fatalf("NewTCPClient failed: %v", err)
			}
			
			tcpClient := client.(*TCPClient)
			address, err := tcpClient.parseAddress()
			
			if tt.shouldErr {
				if err == nil {
					t.Errorf("Expected error for address %s, but got none", tt.command)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for address %s: %v", tt.command, err)
				}
				if address != tt.expected {
					t.Errorf("Expected address %s, got %s", tt.expected, address)
				}
			}
		})
	}
}

func TestTCPClientConnectionFailure(t *testing.T) {
	config := ClientConfig{
		Command:   "localhost:99999", // Invalid port
		Transport: "tcp",
	}
	
	client, err := NewTCPClient(config)
	if err != nil {
		t.Fatalf("NewTCPClient failed: %v", err)
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	err = client.Start(ctx)
	if err == nil {
		t.Error("Start should fail for invalid address")
		client.Stop()
	}
	
	if client.IsActive() {
		t.Error("Client should not be active after failed start")
	}
}

func TestTCPClientWithMockServer(t *testing.T) {
	// Start a mock TCP server
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to start mock server: %v", err)
	}
	defer listener.Close()
	
	// Get the actual port
	port := listener.Addr().(*net.TCPAddr).Port
	
	// Handle connections in a goroutine
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			
			// Echo server - just close the connection for this test
			conn.Close()
		}
	}()
	
	// Create TCP client
	config := ClientConfig{
		Command:   fmt.Sprintf("localhost:%d", port),
		Transport: "tcp",
	}
	
	client, err := NewTCPClient(config)
	if err != nil {
		t.Fatalf("NewTCPClient failed: %v", err)
	}
	
	// Test Start
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	
	if !client.IsActive() {
		t.Error("Client should be active after Start()")
	}
	
	// Test Stop
	if err := client.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	
	// Give some time for cleanup
	time.Sleep(100 * time.Millisecond)
	
	if client.IsActive() {
		t.Error("Client should not be active after Stop()")
	}
}

func TestTCPClientDoubleStart(t *testing.T) {
	// Start a mock TCP server
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to start mock server: %v", err)
	}
	defer listener.Close()
	
	// Get the actual port
	port := listener.Addr().(*net.TCPAddr).Port
	
	// Handle connections in a goroutine
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()
	
	// Create TCP client
	config := ClientConfig{
		Command:   fmt.Sprintf("localhost:%d", port),
		Transport: "tcp",
	}
	
	client, err := NewTCPClient(config)
	if err != nil {
		t.Fatalf("NewTCPClient failed: %v", err)
	}
	
	// Test Start
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := client.Start(ctx); err != nil {
		t.Fatalf("First Start failed: %v", err)
	}
	
	// Test second Start should fail
	if err := client.Start(ctx); err == nil {
		t.Error("Second Start should fail")
	}
	
	// Cleanup
	client.Stop()
}

func TestTCPClientRequestWhenInactive(t *testing.T) {
	config := ClientConfig{
		Command:   "localhost:8080",
		Transport: "tcp",
	}
	
	client, err := NewTCPClient(config)
	if err != nil {
		t.Fatalf("NewTCPClient failed: %v", err)
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	
	// Try to send request without starting client
	_, err = client.SendRequest(ctx, "test", nil)
	if err == nil {
		t.Error("SendRequest should fail when client is inactive")
	}
	
	// Try to send notification without starting client
	err = client.SendNotification(ctx, "test", nil)
	if err == nil {
		t.Error("SendNotification should fail when client is inactive")
	}
}

func TestTCPClientRequestIDGeneration(t *testing.T) {
	config := ClientConfig{
		Command:   "localhost:8080",
		Transport: "tcp",
	}
	
	client, err := NewTCPClient(config)
	if err != nil {
		t.Fatalf("NewTCPClient failed: %v", err)
	}
	
	tcpClient := client.(*TCPClient)
	
	// Generate multiple IDs and ensure they're unique
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := tcpClient.generateRequestID()
		if ids[id] {
			t.Errorf("Duplicate ID generated: %s", id)
		}
		ids[id] = true
	}
}

func TestLSPClientFactoryTCP(t *testing.T) {
	// Test TCP transport creation through factory
	config := ClientConfig{
		Command:   "localhost:8080",
		Args:      []string{},
		Transport: "tcp",
	}
	
	client, err := NewLSPClient(config)
	if err != nil {
		t.Fatalf("NewLSPClient failed for TCP transport: %v", err)
	}
	
	if client == nil {
		t.Fatal("NewLSPClient returned nil client for TCP transport")
	}
	
	// Verify it's actually a TCP client
	_, ok := client.(*TCPClient)
	if !ok {
		t.Error("NewLSPClient should return a TCPClient for TCP transport")
	}
}