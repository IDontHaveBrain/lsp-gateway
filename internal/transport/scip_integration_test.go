package transport

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"
)

// TestableSCIPIndexer extends SCIPIndexer with testing-specific methods
type TestableSCIPIndexer interface {
	SCIPIndexer
	GetIndexedRequests() []IndexedRequest
}

// IntegrationSCIPIndexer simulates a real SCIP indexer for integration testing
type IntegrationSCIPIndexer struct {
	mu              sync.RWMutex
	indexedRequests []IndexedRequest
	enabled         bool
	indexDelay      time.Duration
}

func NewTestSCIPIndexer() *IntegrationSCIPIndexer {
	return &IntegrationSCIPIndexer{
		indexedRequests: make([]IndexedRequest, 0),
		enabled:         true,
		indexDelay:      50 * time.Millisecond, // Simulate realistic indexing delay
	}
}

func (i *IntegrationSCIPIndexer) IndexResponse(method string, params interface{}, response json.RawMessage, requestID string) {
	// Simulate processing time
	time.Sleep(i.indexDelay)
	
	i.mu.Lock()
	defer i.mu.Unlock()
	
	i.indexedRequests = append(i.indexedRequests, IndexedRequest{
		Method:    method,
		Params:    params,
		Response:  response,
		RequestID: requestID,
	})
}

func (i *IntegrationSCIPIndexer) IsEnabled() bool {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.enabled
}

func (i *IntegrationSCIPIndexer) Close() error {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.enabled = false
	return nil
}

func (i *IntegrationSCIPIndexer) GetIndexedRequests() []IndexedRequest {
	i.mu.RLock()
	defer i.mu.RUnlock()
	
	// Return a copy to prevent data races
	result := make([]IndexedRequest, len(i.indexedRequests))
	copy(result, i.indexedRequests)
	return result
}

func (i *IntegrationSCIPIndexer) SetEnabled(enabled bool) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.enabled = enabled
}

// TestStdioClientSCIPIntegration tests the full SCIP indexing flow with stdio client
func TestStdioClientSCIPIntegration(t *testing.T) {
	scipIndexer := NewTestSCIPIndexer()
	
	config := ClientConfig{
		Command:   "echo", // Simple command for testing
		Args:      []string{},
		Transport: TransportStdio,
	}
	
	client, err := NewStdioClientWithSCIP(config, scipIndexer)
	if err != nil {
		t.Fatalf("Failed to create STDIO client with SCIP: %v", err)
	}
	
	stdioClient, ok := client.(*StdioClient)
	if !ok {
		t.Fatal("Client is not a StdioClient")
	}
	
	if stdioClient.scipIndexer == nil {
		t.Fatal("SCIP indexer was not properly integrated into STDIO client")
	}
	
	if !stdioClient.scipIndexer.IsEnabled() {
		t.Error("SCIP indexer should be enabled")
	}
	
	// Test cleanup
	err = client.Stop()
	if err != nil {
		t.Errorf("Failed to stop client: %v", err)
	}
}

// TestTCPClientSCIPIntegration tests the full SCIP indexing flow with TCP client
func TestTCPClientSCIPIntegration(t *testing.T) {
	scipIndexer := NewTestSCIPIndexer()
	
	config := ClientConfig{
		Command:   "localhost:9999", // Non-existent server for testing
		Args:      []string{},
		Transport: TransportTCP,
	}
	
	client, err := NewTCPClientWithSCIP(config, scipIndexer)
	if err != nil {
		t.Fatalf("Failed to create TCP client with SCIP: %v", err)
	}
	
	// Verify SCIP indexer is integrated
	tcpClient, ok := client.(*TCPClient)
	if !ok {
		t.Fatal("Client is not a TCPClient")
	}
	
	if tcpClient.scipIndexer == nil {
		t.Fatal("SCIP indexer was not properly integrated into TCP client")
	}
	
	if !tcpClient.scipIndexer.IsEnabled() {
		t.Error("SCIP indexer should be enabled")
	}
	
	// Test cleanup
	err = client.Stop()
	if err != nil {
		t.Errorf("Failed to stop client: %v", err)
	}
}

// TestClientFactoryWithSCIP tests the client factory functions with SCIP support
func TestClientFactoryWithSCIP(t *testing.T) {
	scipIndexer := NewTestSCIPIndexer()
	
	// Test STDIO client creation
	stdioConfig := ClientConfig{
		Command:   "echo",
		Args:      []string{},
		Transport: TransportStdio,
	}
	
	stdioClient, err := NewLSPClientWithSCIP(stdioConfig, scipIndexer)
	if err != nil {
		t.Fatalf("Failed to create STDIO client via factory: %v", err)
	}
	
	stdioClientTyped, ok := stdioClient.(*StdioClient)
	if !ok {
		t.Fatal("STDIO client is not of correct type")
	}
	if !stdioClientTyped.scipIndexer.IsEnabled() {
		t.Error("SCIP indexer should be enabled for STDIO client")
	}
	
	// Test TCP client creation
	tcpConfig := ClientConfig{
		Command:   "localhost:9999",
		Args:      []string{},
		Transport: TransportTCP,
	}
	
	tcpClient, err := NewLSPClientWithSCIP(tcpConfig, scipIndexer)
	if err != nil {
		t.Fatalf("Failed to create TCP client via factory: %v", err)
	}
	
	tcpClientTyped, ok := tcpClient.(*TCPClient)
	if !ok {
		t.Fatal("TCP client is not of correct type")
	}
	if !tcpClientTyped.scipIndexer.IsEnabled() {
		t.Error("SCIP indexer should be enabled for TCP client")
	}
	
	// Test without SCIP indexer (nil)
	stdioClientNoSCIP, err := NewLSPClientWithSCIP(stdioConfig, nil)
	if err != nil {
		t.Fatalf("Failed to create STDIO client without SCIP: %v", err)
	}
	
	stdioClientNoSCIPTyped, ok := stdioClientNoSCIP.(*StdioClient)
	if !ok {
		t.Fatal("STDIO client without SCIP is not of correct type")
	}
	if stdioClientNoSCIPTyped.scipIndexer != nil {
		t.Error("SCIP indexer should be nil when not provided")
	}
	
	// Cleanup
	stdioClient.Stop()
	tcpClient.Stop()
	stdioClientNoSCIP.Stop()
}

// TestSCIPIndexingPerformance tests that SCIP indexing doesn't impact transport performance
func TestSCIPIndexingPerformance(t *testing.T) {
	scipIndexer := NewTestSCIPIndexer()
	
	// Create wrapper with limited goroutines to test resource management
	wrapper := NewSafeIndexerWrapper(scipIndexer, 3)
	
	// Simulate concurrent indexing requests
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	var wg sync.WaitGroup
	numRequests := 20
	
	start := time.Now()
	
	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(requestNum int) {
			defer wg.Done()
			
			select {
			case <-ctx.Done():
				return
			default:
				// Simulate different types of cacheable requests
				methods := []string{
					"textDocument/definition",
					"textDocument/references",
					"textDocument/documentSymbol",
					"workspace/symbol",
				}
				
				method := methods[requestNum%len(methods)]
				response := json.RawMessage(`{"result": {"uri": "file:///test.go"}}`)
				wrapper.SafeIndexResponse(method, nil, response, string(rune(requestNum)))
			}
		}(i)
	}
	
	wg.Wait()
	elapsed := time.Since(start)
	
	if elapsed > 2*time.Second {
		t.Errorf("SCIP indexing took too long: %v (should be under 2s for %d requests)", elapsed, numRequests)
	}
	
	// Verify some requests were indexed (not all might be due to capacity limits)
	time.Sleep(200 * time.Millisecond) // Allow background processing to complete
	
	indexed := scipIndexer.GetIndexedRequests()
	if len(indexed) == 0 {
		t.Error("Expected at least some requests to be indexed")
	}
	
	t.Logf("Successfully indexed %d out of %d requests in %v", len(indexed), numRequests, elapsed)
}

// BenchmarkSCIPIndexingOverhead benchmarks the overhead of SCIP indexing
func BenchmarkSCIPIndexingOverhead(b *testing.B) {
	scipIndexer := NewTestSCIPIndexer()
	wrapper := NewSafeIndexerWrapper(scipIndexer, 10)
	
	response := json.RawMessage(`{"result": {"uri": "file:///test.go", "range": {"start": {"line": 10, "character": 5}, "end": {"line": 10, "character": 15}}}}`)
	
	b.ResetTimer()
	
	b.Run("WithSCIPIndexing", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			wrapper.SafeIndexResponse("textDocument/definition", nil, response, "req_1")
		}
	})
	
	b.Run("WithoutSCIPIndexing", func(b *testing.B) {
		// Simulate the same response handling without SCIP indexing
		for i := 0; i < b.N; i++ {
			// Just check if method is cacheable (minimal overhead)
			IsCacheableMethod("textDocument/definition")
		}
	})
}

// TestSCIPIndexerResourceManagement tests proper resource cleanup and management
func TestSCIPIndexerResourceManagement(t *testing.T) {
	scipIndexer := NewTestSCIPIndexer()
	
	// Create multiple clients with SCIP indexing
	configs := []ClientConfig{
		{Command: "echo", Args: []string{}, Transport: TransportStdio},
	}
	
	clients := make([]LSPClient, 0, len(configs))
	
	// Create clients
	for _, config := range configs {
		client, err := NewLSPClientWithSCIP(config, scipIndexer)
		if err != nil {
			t.Fatalf("Failed to create client: %v", err)
		}
		clients = append(clients, client)
	}
	
	// Stop all clients and verify cleanup
	for i, client := range clients {
		err := client.Stop()
		if err != nil {
			t.Errorf("Failed to stop client %d: %v", i, err)
		}
	}
	
	// Verify SCIP indexer can be closed properly
	err := scipIndexer.Close()
	if err != nil {
		t.Errorf("Failed to close SCIP indexer: %v", err)
	}
	
	if scipIndexer.IsEnabled() {
		t.Error("SCIP indexer should be disabled after closing")
	}
}