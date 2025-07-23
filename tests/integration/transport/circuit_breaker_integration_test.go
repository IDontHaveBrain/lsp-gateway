package transport_test

import (
	"sync"
	"testing"
	"time"

	"lsp-gateway/internal/transport"
)

// TestStdioClientCircuitBreakerIntegration tests the embedded circuit breaker logic in StdioClient
func TestStdioClientCircuitBreakerIntegration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		maxRetries          int
		errorSequence       []bool // true = success, false = error
		expectedCircuitOpen bool
	}{
		{
			name:                "circuit stays closed with few errors",
			maxRetries:          3,
			errorSequence:       []bool{false, true, false, true}, // 2 errors, 2 successes
			expectedCircuitOpen: false,
		},
		{
			name:                "circuit opens with too many errors",
			maxRetries:          2,
			errorSequence:       []bool{false, false, false}, // 3 consecutive errors, max=2
			expectedCircuitOpen: true,
		},
		{
			name:                "circuit resets on success",
			maxRetries:          3,
			errorSequence:       []bool{false, false, true, true}, // errors then successes
			expectedCircuitOpen: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := transport.ClientConfig{
				Command:   "echo",
				Args:      []string{"test"},
				Transport: transport.TransportStdio,
			}

			client, err := transport.NewStdioClient(config)
			if err != nil {
				t.Fatalf("Failed to create stdio client: %v", err)
			}

			// Set max retries for test
			client.maxRetries = tt.maxRetries

			// Simulate the error sequence by directly manipulating the circuit breaker state
			for i, shouldSucceed := range tt.errorSequence {
				if shouldSucceed {
					client.resetErrorCount()
					t.Logf("Step %d: Reset error count (success)", i+1)
				} else {
					client.recordError()
					t.Logf("Step %d: Recorded error (errorCount now: %d)", i+1, client.errorCount)
				}
			}

			// Check final circuit state
			circuitOpen := client.isCircuitOpen()
			t.Logf("Final state: circuitOpen=%v, errorCount=%d, expectedOpen=%v",
				circuitOpen, client.errorCount, tt.expectedCircuitOpen)

			if circuitOpen != tt.expectedCircuitOpen {
				t.Errorf("Expected circuit open=%v, got %v", tt.expectedCircuitOpen, circuitOpen)
			}
		})
	}
}

// TestStdioClientCircuitBreakerRecovery tests automatic circuit recovery after timeout
func TestStdioClientCircuitBreakerRecovery(t *testing.T) {
	t.Parallel()

	config := transport.ClientConfig{
		Command:   "echo",
		Args:      []string{"test"},
		Transport: transport.TransportStdio,
	}

	client, err := transport.NewStdioClient(config)
	if err != nil {
		t.Fatalf("Failed to create stdio client: %v", err)
	}

	client.maxRetries = 2

	// Force circuit open
	client.recordError()
	client.recordError()
	client.recordError() // Exceed maxRetries
	client.openCircuit() // Explicitly open circuit

	if !client.isCircuitOpen() {
		t.Fatal("Expected circuit to be open initially")
	}

	t.Logf("Circuit opened, waiting for recovery...")

	// The actual implementation uses a 30-second timeout, but for testing we'll manipulate time
	// by directly modifying lastErrorTime to simulate time passage
	client.lastErrorTime = time.Now().Add(-35 * time.Second) // Simulate 35 seconds ago

	// Check if circuit recovered
	if client.isCircuitOpen() {
		t.Error("Expected circuit to be closed after timeout")
	}

	t.Logf("Circuit successfully recovered after timeout")
}

// TestStdioClientCircuitBreakerThreadSafety tests circuit breaker under concurrent access
func TestStdioClientCircuitBreakerThreadSafety(t *testing.T) {
	t.Parallel()

	config := transport.ClientConfig{
		Command:   "echo",
		Args:      []string{"test"},
		Transport: transport.TransportStdio,
	}

	client, err := transport.NewStdioClient(config)
	if err != nil {
		t.Fatalf("Failed to create stdio client: %v", err)
	}

	client.maxRetries = 5

	const numWorkers = 10
	const operationsPerWorker = 50
	var wg sync.WaitGroup

	// Launch concurrent workers that record errors and check circuit state
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < operationsPerWorker; j++ {
				// Mix of error recording and circuit checks
				switch j % 4 {
				case 0:
					client.recordError()
				case 1:
					client.isCircuitOpen()
				case 2:
					client.resetErrorCount()
				case 3:
					if client.errorCount > 3 {
						client.openCircuit()
					}
				}

				// Small delay to increase chance of race conditions
				time.Sleep(time.Microsecond)
			}
		}(i)
	}

	wg.Wait()

	// Verify that the circuit breaker is still in a valid state
	finalErrorCount := client.errorCount
	finalCircuitOpen := client.isCircuitOpen()

	t.Logf("Final state after concurrent operations: errorCount=%d, circuitOpen=%v",
		finalErrorCount, finalCircuitOpen)

	// The specific values don't matter as much as ensuring no panics occurred
	// and the state is consistent
	if finalErrorCount < 0 {
		t.Errorf("Error count should not be negative: %d", finalErrorCount)
	}
}

// TestTCPClientCircuitBreakerBehavior tests circuit breaker logic in TCPClient
func TestTCPClientCircuitBreakerBehavior(t *testing.T) {
	t.Parallel()

	// TCPClient has similar circuit breaker implementation to StdioClient
	config := transport.ClientConfig{
		Command:   "localhost:0", // Will fail to connect, which is what we want for testing
		Transport: TransportTCP,
	}

	clientInterface, err := NewTCPClient(config)
	if err != nil {
		t.Fatalf("Failed to create TCP client: %v", err)
	}

	// Cast to concrete type to access circuit breaker methods
	client := clientInterface.(*TCPClient)
	client.maxRetries = 3

	// Test error recording
	initiallyOpen := client.isCircuitOpen()
	if initiallyOpen {
		t.Error("Circuit should start closed")
	}

	// Record errors to trigger circuit opening
	for i := 0; i < 5; i++ { // More than maxRetries
		client.recordError()
	}

	if !client.isCircuitOpen() {
		t.Error("Circuit should be open after exceeding max retries")
	}

	// Test reset
	client.resetErrorCount()
	if client.isCircuitOpen() {
		t.Error("Circuit should be closed after reset")
	}

	t.Logf("TCP client circuit breaker behavior validated successfully")
}

// TestCircuitBreakerBackoffCalculation tests the backoff calculation logic
func TestCircuitBreakerBackoffCalculation(t *testing.T) {
	t.Parallel()

	config := transport.ClientConfig{
		Command:   "echo",
		Args:      []string{"test"},
		Transport: transport.TransportStdio,
	}

	client, err := transport.NewStdioClient(config)
	if err != nil {
		t.Fatalf("Failed to create stdio client: %v", err)
	}

	// Test that backoff increases with attempt number
	attempt1 := client.calculateBackoff(1)
	attempt2 := client.calculateBackoff(2)
	attempt3 := client.calculateBackoff(3)

	t.Logf("Backoff delays: attempt1=%v, attempt2=%v, attempt3=%v",
		attempt1, attempt2, attempt3)

	// Each attempt should generally be longer (exponential backoff)
	// We can't test exact values due to jitter, but should be increasing trend
	if attempt1 > attempt2 {
		t.Errorf("Backoff should increase with attempts: attempt1=%v > attempt2=%v",
			attempt1, attempt2)
	}

	if attempt2 > attempt3 {
		t.Errorf("Backoff should increase with attempts: attempt2=%v > attempt3=%v",
			attempt2, attempt3)
	}

	// Ensure backoff is reasonable (not zero, not too large)
	if attempt1 == 0 {
		t.Error("Backoff should not be zero")
	}

	maxReasonableBackoff := 10 * time.Second
	if attempt3 > maxReasonableBackoff {
		t.Errorf("Backoff too large for attempt 3: %v > %v", attempt3, maxReasonableBackoff)
	}
}

// TestCircuitBreakerErrorThreshold tests the error threshold behavior
func TestCircuitBreakerErrorThreshold(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		maxRetries          int
		errorCount          int
		expectedCircuitOpen bool
	}{
		{
			name:                "below threshold",
			maxRetries:          5,
			errorCount:          3,
			expectedCircuitOpen: false,
		},
		{
			name:                "at threshold",
			maxRetries:          3,
			errorCount:          3,
			expectedCircuitOpen: false, // Equal to threshold is still closed
		},
		{
			name:                "above threshold",
			maxRetries:          3,
			errorCount:          5,
			expectedCircuitOpen: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := transport.ClientConfig{
				Command:   "echo",
				Args:      []string{"test"},
				Transport: transport.TransportStdio,
			}

			client, err := transport.NewStdioClient(config)
			if err != nil {
				t.Fatalf("Failed to create stdio client: %v", err)
			}

			client.maxRetries = tt.maxRetries

			// Simulate the specified error count
			for i := 0; i < tt.errorCount; i++ {
				client.recordError()
			}

			circuitOpen := client.isCircuitOpen()
			t.Logf("maxRetries=%d, errorCount=%d, circuitOpen=%v",
				tt.maxRetries, tt.errorCount, circuitOpen)

			if circuitOpen != tt.expectedCircuitOpen {
				t.Errorf("Expected circuit open=%v, got %v", tt.expectedCircuitOpen, circuitOpen)
			}
		})
	}
}

// TestCircuitBreakerExplicitOpen tests explicit circuit opening
func TestCircuitBreakerExplicitOpen(t *testing.T) {
	t.Parallel()

	config := transport.ClientConfig{
		Command:   "echo",
		Args:      []string{"test"},
		Transport: transport.TransportStdio,
	}

	client, err := transport.NewStdioClient(config)
	if err != nil {
		t.Fatalf("Failed to create stdio client: %v", err)
	}

	// Initially circuit should be closed
	if client.isCircuitOpen() {
		t.Error("Circuit should start closed")
	}

	// Explicitly open circuit
	client.openCircuit()

	if !client.isCircuitOpen() {
		t.Error("Circuit should be open after explicit open call")
	}

	// Reset should close it
	client.resetErrorCount()

	if client.isCircuitOpen() {
		t.Error("Circuit should be closed after reset")
	}

	t.Logf("Explicit circuit control validated successfully")
}
