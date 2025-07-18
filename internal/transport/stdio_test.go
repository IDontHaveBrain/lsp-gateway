package transport

import (
	"context"
	"testing"
	"time"
)

func TestNewStdioClient(t *testing.T) {
	config := ClientConfig{
		Command:   "echo",
		Args:      []string{"test"},
		Transport: "stdio",
	}
	
	client, err := NewStdioClient(config)
	if err != nil {
		t.Fatalf("NewStdioClient failed: %v", err)
	}
	
	if client == nil {
		t.Fatal("NewStdioClient returned nil client")
	}
	
	if client.IsActive() {
		t.Error("Client should not be active before Start() is called")
	}
}

func TestStdioClientLifecycle(t *testing.T) {
	config := ClientConfig{
		Command:   "cat", // cat will echo input back
		Args:      []string{},
		Transport: "stdio",
	}
	
	client, err := NewStdioClient(config)
	if err != nil {
		t.Fatalf("NewStdioClient failed: %v", err)
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
	
	if client.IsActive() {
		t.Error("Client should not be active after Stop()")
	}
}

func TestLSPClientFactory(t *testing.T) {
	// Test STDIO transport
	config := ClientConfig{
		Command:   "echo",
		Args:      []string{"test"},
		Transport: "stdio",
	}
	
	client, err := NewLSPClient(config)
	if err != nil {
		t.Fatalf("NewLSPClient failed for stdio: %v", err)
	}
	
	if client == nil {
		t.Fatal("NewLSPClient returned nil client")
	}
	
	// Test unsupported transport
	config.Transport = "unsupported"
	client, err = NewLSPClient(config)
	if err == nil {
		t.Error("NewLSPClient should fail for unsupported transport")
	}
	
	if client != nil {
		t.Error("NewLSPClient should return nil for unsupported transport")
	}
	
	// Test TCP transport (should work since it's implemented)
	config.Transport = "tcp"
	client, err = NewLSPClient(config)
	if err != nil {
		t.Fatalf("NewLSPClient failed for TCP transport: %v", err)
	}
	
	if client == nil {
		t.Error("NewLSPClient should return a client for TCP transport")
	}
}