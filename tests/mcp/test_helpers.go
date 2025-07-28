package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sync"
	"testing"
	"time"
)

// TestHelper provides helper methods for MCP client testing
type TestHelper struct {
	t      *testing.T
	client *TestMCPClient
}

// NewTestHelper creates a new test helper
func NewTestHelper(t *testing.T, client *TestMCPClient) *TestHelper {
	return &TestHelper{
		t:      t,
		client: client,
	}
}

// AssertState verifies the client is in the expected state
func (th *TestHelper) AssertState(expected ClientState) {
	th.t.Helper()
	actual := th.client.GetState()
	if actual != expected {
		th.t.Errorf("Expected state %s, got %s", expected, actual)
	}
}

// AssertStateWithin verifies the client reaches the expected state within timeout
func (th *TestHelper) AssertStateWithin(expected ClientState, timeout time.Duration) {
	th.t.Helper()
	err := th.client.WaitForState(expected, timeout)
	if err != nil {
		th.t.Errorf("Failed to reach state %s within %v: %v", expected, timeout, err)
	}
}

// AssertNoError checks that error is nil
func (th *TestHelper) AssertNoError(err error, msg string) {
	th.t.Helper()
	if err != nil {
		th.t.Errorf("%s: %v", msg, err)
	}
}

// AssertError checks that error is not nil
func (th *TestHelper) AssertError(err error, msg string) {
	th.t.Helper()
	if err == nil {
		th.t.Errorf("%s: expected error but got nil", msg)
	}
}

// AssertValidMessage validates an MCP message
func (th *TestHelper) AssertValidMessage(msg *MCPMessage) {
	th.t.Helper()
	errors := ValidateMessage(msg)
	if len(errors) > 0 {
		th.t.Errorf("Message validation failed:\n")
		for _, err := range errors {
			th.t.Errorf("  - %s", err)
		}
		th.t.Logf("Message: %s", FormatMessage(msg))
	}
}

// AssertMessageType checks the type of message
func (th *TestHelper) AssertMessageType(msg *MCPMessage, expectRequest, expectResponse, expectNotification bool) {
	th.t.Helper()
	
	if expectRequest && !msg.IsRequest() {
		th.t.Errorf("Expected request message, got: %s", FormatMessage(msg))
	}
	if expectResponse && !msg.IsResponse() {
		th.t.Errorf("Expected response message, got: %s", FormatMessage(msg))
	}
	if expectNotification && !msg.IsNotification() {
		th.t.Errorf("Expected notification message, got: %s", FormatMessage(msg))
	}
}

// AssertEqual checks if two values are equal
func (th *TestHelper) AssertEqual(expected, actual interface{}, msg string) {
	th.t.Helper()
	if !reflect.DeepEqual(expected, actual) {
		expectedJSON, _ := json.MarshalIndent(expected, "", "  ")
		actualJSON, _ := json.MarshalIndent(actual, "", "  ")
		th.t.Errorf("%s:\nExpected:\n%s\nActual:\n%s", msg, expectedJSON, actualJSON)
	}
}

// AssertNotNil checks that value is not nil
func (th *TestHelper) AssertNotNil(value interface{}, msg string) {
	th.t.Helper()
	if value == nil || (reflect.ValueOf(value).Kind() == reflect.Ptr && reflect.ValueOf(value).IsNil()) {
		th.t.Errorf("%s: expected non-nil value", msg)
	}
}

// AssertNil checks that value is nil
func (th *TestHelper) AssertNil(value interface{}, msg string) {
	th.t.Helper()
	if value != nil && !(reflect.ValueOf(value).Kind() == reflect.Ptr && reflect.ValueOf(value).IsNil()) {
		th.t.Errorf("%s: expected nil value, got %v", msg, value)
	}
}

// MockTransport provides a mock transport for testing
type MockTransport struct {
	// Channels for communication
	SendCh    chan []byte
	ReceiveCh chan []byte
	ErrorCh   chan error
	
	// State
	connected bool
	closed    bool
}

// NewMockTransport creates a new mock transport
func NewMockTransport() *MockTransport {
	return &MockTransport{
		SendCh:    make(chan []byte, 100),
		ReceiveCh: make(chan []byte, 100),
		ErrorCh:   make(chan error, 10),
	}
}

// Connect simulates connection
func (mt *MockTransport) Connect(ctx context.Context) error {
	if mt.closed {
		return fmt.Errorf("transport is closed")
	}
	if mt.connected {
		return fmt.Errorf("already connected")
	}
	mt.connected = true
	return nil
}

// Disconnect simulates disconnection
func (mt *MockTransport) Disconnect() error {
	if !mt.connected {
		return fmt.Errorf("not connected")
	}
	mt.connected = false
	return nil
}

// Send simulates sending data
func (mt *MockTransport) Send(ctx context.Context, data []byte) error {
	if !mt.connected {
		return fmt.Errorf("not connected")
	}
	select {
	case mt.SendCh <- data:
		return nil
	default:
		return fmt.Errorf("send buffer full")
	}
}

// Receive simulates receiving data
func (mt *MockTransport) Receive(ctx context.Context) ([]byte, error) {
	if !mt.connected {
		return nil, fmt.Errorf("not connected")
	}
	
	select {
	case data := <-mt.ReceiveCh:
		return data, nil
	case err := <-mt.ErrorCh:
		return nil, err
	default:
		return nil, fmt.Errorf("no data available")
	}
}

// Close closes the mock transport
func (mt *MockTransport) Close() error {
	if mt.closed {
		return fmt.Errorf("already closed")
	}
	mt.closed = true
	mt.connected = false
	close(mt.SendCh)
	close(mt.ReceiveCh)
	close(mt.ErrorCh)
	return nil
}

// IsConnected returns connection status
func (mt *MockTransport) IsConnected() bool {
	return mt.connected
}

// SimulateResponse adds a response to the receive channel
func (mt *MockTransport) SimulateResponse(msg *MCPMessage) error {
	data, err := msg.ToJSON()
	if err != nil {
		return err
	}
	
	select {
	case mt.ReceiveCh <- data:
		return nil
	default:
		return fmt.Errorf("receive buffer full")
	}
}

// SimulateError adds an error to the error channel
func (mt *MockTransport) SimulateError(err error) error {
	select {
	case mt.ErrorCh <- err:
		return nil
	default:
		return fmt.Errorf("error buffer full")
	}
}

// GetLastSent retrieves the last sent message
func (mt *MockTransport) GetLastSent() (*MCPMessage, error) {
	select {
	case data := <-mt.SendCh:
		return ParseMCPMessage(data)
	default:
		return nil, fmt.Errorf("no sent messages")
	}
}

// CreateTestClient creates a test client with mock transport
func CreateTestClient(t *testing.T, config ...ClientConfig) (*TestMCPClient, *MockTransport) {
	var cfg ClientConfig
	if len(config) > 0 {
		cfg = config[0]
	} else {
		cfg = DefaultClientConfig()
	}
	
	transport := NewMockTransport()
	testClient := NewTestMCPClient(cfg, transport)
	
	return testClient, transport
}

// STDIOTransport implements Transport interface for STDIO communication
type STDIOTransport struct {
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	stderr    io.ReadCloser
	connected bool
	closed    bool
	mu        sync.RWMutex
}

// NewSTDIOTransport creates a new STDIO transport
func NewSTDIOTransport(stdin io.WriteCloser, stdout, stderr io.ReadCloser) *STDIOTransport {
	return &STDIOTransport{
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
	}
}

// Connect establishes the STDIO connection
func (st *STDIOTransport) Connect() error {
	st.mu.Lock()
	defer st.mu.Unlock()
	
	if st.closed {
		return fmt.Errorf("transport is closed")
	}
	if st.connected {
		return fmt.Errorf("already connected")
	}
	
	st.connected = true
	return nil
}

// Disconnect closes the STDIO connection
func (st *STDIOTransport) Disconnect() error {
	st.mu.Lock()
	defer st.mu.Unlock()
	
	if !st.connected {
		return fmt.Errorf("not connected")
	}
	
	st.connected = false
	return nil
}

// Send sends data via STDIO
func (st *STDIOTransport) Send(data []byte) error {
	st.mu.RLock()
	defer st.mu.RUnlock()
	
	if !st.connected {
		return fmt.Errorf("not connected")
	}
	
	// Add newline for Direct JSON format
	data = append(data, '\n')
	
	_, err := st.stdin.Write(data)
	return err
}

// Receive receives data from STDIO
func (st *STDIOTransport) Receive() ([]byte, error) {
	st.mu.RLock()
	defer st.mu.RUnlock()
	
	if !st.connected {
		return nil, fmt.Errorf("not connected")
	}
	
	// Read a line from stdout
	reader := bufio.NewReader(st.stdout)
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	
	// Remove trailing newline
	if len(line) > 0 && line[len(line)-1] == '\n' {
		line = line[:len(line)-1]
	}
	
	return line, nil
}

// IsConnected returns connection status
func (st *STDIOTransport) IsConnected() bool {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return st.connected
}

// Close closes the transport
func (st *STDIOTransport) Close() error {
	st.mu.Lock()
	defer st.mu.Unlock()
	
	if st.closed {
		return fmt.Errorf("already closed")
	}
	
	st.closed = true
	st.connected = false
	
	var errors []error
	
	if st.stdin != nil {
		if err := st.stdin.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close stdin: %w", err))
		}
	}
	
	if st.stdout != nil {
		if err := st.stdout.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close stdout: %w", err))
		}
	}
	
	if st.stderr != nil {
		if err := st.stderr.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close stderr: %w", err))
		}
	}
	
	if len(errors) > 0 {
		return fmt.Errorf("close errors: %v", errors)
	}
	
	return nil
}