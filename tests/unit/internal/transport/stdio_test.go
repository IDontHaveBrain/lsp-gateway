package transport_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"lsp-gateway/internal/transport"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewStdioClient(t *testing.T) {
	config := transport.ClientConfig{
		Command:   "echo",
		Args:      []string{"test"},
		Transport: "stdio",
	}

	client, err := transport.NewStdioClient(config)
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
	config := transport.ClientConfig{
		Command:   "cat", // cat will echo input back
		Args:      []string{},
		Transport: "stdio",
	}

	client, err := transport.NewStdioClient(config)
	if err != nil {
		t.Fatalf("NewStdioClient failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !client.IsActive() {
		t.Error("Client should be active after Start()")
	}

	if err := client.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if client.IsActive() {
		t.Error("Client should not be active after Stop()")
	}
}

func TestLSPClientFactory(t *testing.T) {
	config := transport.ClientConfig{
		Command:   "echo",
		Args:      []string{"test"},
		Transport: "stdio",
	}

	client, err := transport.NewLSPClient(config)
	if err != nil {
		t.Fatalf("NewLSPClient failed for stdio: %v", err)
	}

	if client == nil {
		t.Fatal("NewLSPClient returned nil client")
	}

	config.Transport = "unsupported"
	client, err = transport.NewLSPClient(config)
	if err == nil {
		t.Error("NewLSPClient should fail for unsupported transport")
	}

	if client != nil {
		t.Error("NewLSPClient should return nil for unsupported transport")
	}

	config.Transport = "tcp"
	client, err = transport.NewLSPClient(config)
	if err != nil {
		t.Fatalf("NewLSPClient failed for TCP transport: %v", err)
	}

	if client == nil {
		t.Error("NewLSPClient should return a client for TCP transport")
	}
}

func TestStdioSendRequestSuccess(t *testing.T) {
	mockServer := createMockLSPServerProgram(t, false, 0, false, 0, 0)
	defer func() {
		if err := os.Remove(mockServer); err != nil {
			t.Logf("cleanup error removing mock server: %v", err)
		}
	}()

	config := transport.ClientConfig{
		Command:   mockServer,
		Args:      []string{},
		Transport: "stdio",
	}

	client, err := transport.NewStdioClient(config)
	if err != nil {
		t.Fatalf("NewStdioClient failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() {
		if err := client.Stop(); err != nil {
			t.Logf("cleanup error stopping client: %v", err)
		}
	}()

	initParams := map[string]interface{}{
		"processId":    nil,
		"rootUri":      "file:///test",
		"capabilities": map[string]interface{}{},
	}

	response, err := client.SendRequest(ctx, "initialize", initParams)
	if err != nil {
		t.Fatalf("SendRequest failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(response, &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if capabilities, ok := result["capabilities"]; !ok {
		t.Error("Response should contain capabilities")
	} else if capMap, ok := capabilities.(map[string]interface{}); !ok {
		t.Error("Capabilities should be an object")
	} else {
		if _, ok := capMap["definitionProvider"]; !ok {
			t.Error("Should have definitionProvider capability")
		}
	}

	hoverParams := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": "file:///test.go",
		},
		"position": map[string]interface{}{
			"line":      3,
			"character": 8,
		},
	}

	hoverResponse, err := client.SendRequest(ctx, "textDocument/hover", hoverParams)
	if err != nil {
		t.Fatalf("Hover SendRequest failed: %v", err)
	}

	var hoverResult map[string]interface{}
	if err := json.Unmarshal(hoverResponse, &hoverResult); err != nil {
		t.Fatalf("Failed to unmarshal hover response: %v", err)
	}

	if contents, ok := hoverResult["contents"]; !ok {
		t.Error("Hover response should contain contents")
	} else if contentsMap, ok := contents.(map[string]interface{}); !ok {
		t.Error("Hover contents should be an object")
	} else {
		if kind, ok := contentsMap["kind"]; !ok {
			t.Error("Hover contents should have kind field")
		} else if kind != "markdown" {
			t.Errorf("Expected hover kind to be 'markdown', got %v", kind)
		}

		if value, ok := contentsMap["value"]; !ok {
			t.Error("Hover contents should have value field")
		} else if valueStr, ok := value.(string); !ok {
			t.Error("Hover value should be a string")
		} else if valueStr == "" {
			t.Error("Hover value should not be empty")
		}
	}
}

func TestStdioSendHoverRequest(t *testing.T) {
	client := setupHoverTestClient(t)
	ctx := createHoverTestContext()

	testCases := createHoverTestCases()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testSingleHoverRequest(t, client, ctx, tc)
		})
	}
}

type HoverTestCase struct {
	name      string
	uri       string
	line      int
	character int
}

func setupHoverTestClient(t *testing.T) *transport.StdioClient {
	mockServer := createMockLSPServerProgram(t, false, 0, false, 0, 0)
	t.Cleanup(func() {
		if err := os.Remove(mockServer); err != nil {
			t.Logf("cleanup error removing mock server: %v", err)
		}
	})

	config := transport.ClientConfig{
		Command:   mockServer,
		Args:      []string{},
		Transport: "stdio",
	}

	client, err := transport.NewStdioClient(config)
	if err != nil {
		t.Fatalf("NewStdioClient failed: %v", err)
	}

	ctx := createHoverTestContext()
	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	t.Cleanup(func() {
		if err := client.Stop(); err != nil {
			t.Logf("cleanup error stopping client: %v", err)
		}
	})

	return client
}

func createHoverTestContext() context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	_ = cancel // Cancel is handled by the context timeout
	return ctx
}

func createHoverTestCases() []HoverTestCase {
	return []HoverTestCase{
		{
			name:      "Go file hover",
			uri:       "file:///main.go",
			line:      5,
			character: 10,
		},
		{
			name:      "Python file hover",
			uri:       "file:///script.py",
			line:      3,
			character: 7,
		},
		{
			name:      "TypeScript file hover",
			uri:       "file:///app.ts",
			line:      8,
			character: 15,
		},
		{
			name:      "Java file hover",
			uri:       "file:///Main.java",
			line:      2,
			character: 12,
		},
	}
}

func testSingleHoverRequest(t *testing.T, client *transport.StdioClient, ctx context.Context, tc HoverTestCase) {
	hoverParams := createHoverParams(tc)

	response, err := client.SendRequest(ctx, "textDocument/hover", hoverParams)
	if err != nil {
		t.Fatalf("Hover request failed for %s: %v", tc.name, err)
	}

	validateHoverResponse(t, response, tc.name)
}

func createHoverParams(tc HoverTestCase) map[string]interface{} {
	return map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": tc.uri,
		},
		"position": map[string]interface{}{
			"line":      tc.line,
			"character": tc.character,
		},
	}
}

func validateHoverResponse(t *testing.T, response []byte, testName string) {
	var result map[string]interface{}
	if err := json.Unmarshal(response, &result); err != nil {
		t.Fatalf("Failed to unmarshal hover response: %v", err)
	}

	validateHoverContents(t, result, testName)
	validateHoverRange(t, result, testName)
}

func validateHoverContents(t *testing.T, result map[string]interface{}, testName string) {
	contents, ok := result["contents"]
	if !ok {
		t.Errorf("Missing contents field in hover response for %s", testName)
		return
	}

	contentsMap, ok := contents.(map[string]interface{})
	if !ok {
		return
	}

	validateContentsKind(t, contentsMap, testName)
	validateContentsValue(t, contentsMap, testName)
}

func validateContentsKind(t *testing.T, contentsMap map[string]interface{}, testName string) {
	kind, ok := contentsMap["kind"]
	if !ok {
		t.Errorf("Missing kind field in hover contents for %s", testName)
		return
	}

	if kind != "markdown" {
		t.Errorf("Expected kind 'markdown' for %s, got %v", testName, kind)
	}
}

func validateContentsValue(t *testing.T, contentsMap map[string]interface{}, testName string) {
	value, ok := contentsMap["value"]
	if !ok {
		t.Errorf("Missing value field in hover contents for %s", testName)
		return
	}

	valueStr, ok := value.(string)
	if !ok {
		t.Errorf("Expected value to be string for %s", testName)
		return
	}

	if len(valueStr) == 0 {
		t.Errorf("Expected non-empty value for %s", testName)
	}
}

func validateHoverRange(t *testing.T, result map[string]interface{}, testName string) {
	rangeField, ok := result["range"]
	if !ok {
		return // Range is optional
	}

	rangeMap, ok := rangeField.(map[string]interface{})
	if !ok {
		return
	}

	if _, ok := rangeMap["start"]; !ok {
		t.Errorf("Missing start field in hover range for %s", testName)
	}

	if _, ok := rangeMap["end"]; !ok {
		t.Errorf("Missing end field in hover range for %s", testName)
	}
}

func TestStdioSendRequestError(t *testing.T) {
	mockServer := createMockLSPServerProgram(t, true, 1, false, 0, 0)
	defer func() {
		if err := os.Remove(mockServer); err != nil {
			t.Logf("cleanup error removing mock server: %v", err)
		}
	}()

	config := transport.ClientConfig{
		Command:   mockServer,
		Args:      []string{},
		Transport: "stdio",
	}

	client, err := transport.NewStdioClient(config)
	if err != nil {
		t.Fatalf("NewStdioClient failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() {
		if err := client.Stop(); err != nil {
			t.Logf("cleanup error stopping client: %v", err)
		}
	}()

	response, err := client.SendRequest(ctx, "textDocument/definition", map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": "file:///test.go"},
		"position":     map[string]interface{}{"line": 10, "character": 5},
	})

	if err != nil {
		t.Fatalf("SendRequest failed: %v", err)
	}

	var errorInfo RPCError
	if err := json.Unmarshal(response, &errorInfo); err != nil {
		t.Fatalf("Failed to unmarshal error response: %v", err)
	}

	if errorInfo.Code != -32601 {
		t.Errorf("Expected error code -32601, got %d", errorInfo.Code)
	}

	if !strings.Contains(errorInfo.Message, "Mock error") {
		t.Errorf("Expected error message to contain 'Mock error', got: %s", errorInfo.Message)
	}
}

func TestStdioSendRequestTimeout(t *testing.T) {
	mockServer := createMockLSPServerProgram(t, false, 0, false, 0, 35*time.Second)
	defer func() {
		if err := os.Remove(mockServer); err != nil {
			t.Logf("cleanup error removing mock server: %v", err)
		}
	}()

	config := transport.ClientConfig{
		Command:   mockServer,
		Args:      []string{},
		Transport: "stdio",
	}

	client, err := transport.NewStdioClient(config)
	if err != nil {
		t.Fatalf("NewStdioClient failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() {
		if err := client.Stop(); err != nil {
			t.Logf("cleanup error stopping client: %v", err)
		}
	}()

	requestCtx, requestCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer requestCancel()

	_, err = client.SendRequest(requestCtx, "textDocument/definition", map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": "file:///test.go"},
		"position":     map[string]interface{}{"line": 10, "character": 5},
	})

	if err == nil {
		t.Fatal("Expected request to timeout")
	}

	if !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("Expected timeout error, got: %v", err)
	}
}

func TestStdioSendRequestNotActive(t *testing.T) {
	config := transport.ClientConfig{
		Command:   "echo",
		Args:      []string{},
		Transport: "stdio",
	}

	client, err := transport.NewStdioClient(config)
	if err != nil {
		t.Fatalf("NewStdioClient failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = client.SendRequest(ctx, "test", nil)
	if err == nil {
		t.Error("Expected error when sending request to inactive client")
	}

	if !strings.Contains(err.Error(), "not active") {
		t.Errorf("Expected 'not active' error, got: %v", err)
	}
}

func TestStdioSendNotificationSuccess(t *testing.T) {
	mockServer := createMockLSPServerProgram(t, false, 0, false, 0, 0)
	defer func() {
		if err := os.Remove(mockServer); err != nil {
			t.Logf("cleanup error removing mock server: %v", err)
		}
	}()

	config := transport.ClientConfig{
		Command:   mockServer,
		Args:      []string{},
		Transport: "stdio",
	}

	client, err := transport.NewStdioClient(config)
	if err != nil {
		t.Fatalf("NewStdioClient failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() {
		if err := client.Stop(); err != nil {
			t.Logf("cleanup error stopping client: %v", err)
		}
	}()

	err = client.SendNotification(ctx, "textDocument/didOpen", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        "file:///test.go",
			"languageId": "go",
			"version":    1,
			"text":       "package main\n\nfunc main() {}\n",
		},
	})

	if err != nil {
		t.Fatalf("SendNotification failed: %v", err)
	}
}

func TestStdioSendNotificationNotActive(t *testing.T) {
	config := transport.ClientConfig{
		Command:   "echo",
		Args:      []string{},
		Transport: "stdio",
	}

	client, err := transport.NewStdioClient(config)
	if err != nil {
		t.Fatalf("NewStdioClient failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = client.SendNotification(ctx, "test", nil)
	if err == nil {
		t.Error("Expected error when sending notification to inactive client")
	}

	if !strings.Contains(err.Error(), "not active") {
		t.Errorf("Expected 'not active' error, got: %v", err)
	}
}

func TestStdioHandleNotifications(t *testing.T) {
	mockServer := createMockLSPServerProgram(t, false, 0, true, 0, 0)
	defer func() {
		if err := os.Remove(mockServer); err != nil {
			t.Logf("cleanup error removing mock server: %v", err)
		}
	}()

	config := transport.ClientConfig{
		Command:   mockServer,
		Args:      []string{},
		Transport: "stdio",
	}

	client, err := transport.NewStdioClient(config)
	if err != nil {
		t.Fatalf("NewStdioClient failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() {
		if err := client.Stop(); err != nil {
			t.Logf("cleanup error stopping client: %v", err)
		}
	}()

	_, err = client.SendRequest(ctx, "textDocument/definition", map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": "file:///test.go"},
		"position":     map[string]interface{}{"line": 10, "character": 5},
	})

	if err != nil {
		t.Fatalf("SendRequest failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

}

func TestStdioConcurrentRequests(t *testing.T) {
	mockServer := createMockLSPServerProgram(t, false, 0, false, 0, 0)
	defer func() {
		if err := os.Remove(mockServer); err != nil {
			t.Logf("cleanup error removing mock server: %v", err)
		}
	}()

	config := transport.ClientConfig{
		Command:   mockServer,
		Args:      []string{},
		Transport: "stdio",
	}

	client, err := transport.NewStdioClient(config)
	if err != nil {
		t.Fatalf("NewStdioClient failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() {
		if err := client.Stop(); err != nil {
			t.Logf("cleanup error stopping client: %v", err)
		}
	}()

	const numRequests = 10
	var wg sync.WaitGroup
	results := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(requestNum int) {
			defer wg.Done()

			requestCtx, requestCancel := context.WithTimeout(ctx, 5*time.Second)
			defer requestCancel()

			_, err := client.SendRequest(requestCtx, "textDocument/definition", map[string]interface{}{
				"textDocument": map[string]interface{}{"uri": fmt.Sprintf("file:///test%d.go", requestNum)},
				"position":     map[string]interface{}{"line": 10, "character": 5},
			})

			results <- err
		}(i)
	}

	wg.Wait()
	close(results)

	for err := range results {
		if err != nil {
			t.Errorf("Concurrent request failed: %v", err)
		}
	}
}

func TestStdioWriteMessageProtocol(t *testing.T) {
	var buf bytes.Buffer

	client := &transport.StdioClient{
		stdin: &writeCloserWrapper{&buf},
	}

	message := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      "test_123",
		Method:  "textDocument/definition",
		Params: map[string]interface{}{
			"textDocument": map[string]interface{}{"uri": "file:///test.go"},
			"position":     map[string]interface{}{"line": 10, "character": 5},
		},
	}

	err := client.writeMessage(message)
	if err != nil {
		t.Fatalf("writeMessage failed: %v", err)
	}

	output := buf.String()

	if !strings.HasPrefix(output, "Content-Length: ") {
		t.Error("Message should start with Content-Length header")
	}

	if !strings.Contains(output, "\r\n\r\n") {
		t.Error("Message should have proper header termination (\\r\\n\\r\\n)")
	}

	headerEnd := strings.Index(output, "\r\n\r\n")
	if headerEnd == -1 {
		t.Fatal("Could not find header termination")
	}

	jsonPart := output[headerEnd+4:]

	var parsedMsg JSONRPCMessage
	if err := json.Unmarshal([]byte(jsonPart), &parsedMsg); err != nil {
		t.Fatalf("Failed to parse JSON part: %v", err)
	}

	if parsedMsg.JSONRPC != "2.0" {
		t.Errorf("Expected JSON-RPC 2.0, got %s", parsedMsg.JSONRPC)
	}

	if parsedMsg.ID != "test_123" {
		t.Errorf("Expected ID 'test_123', got %v", parsedMsg.ID)
	}

	if parsedMsg.Method != "textDocument/definition" {
		t.Errorf("Expected method 'textDocument/definition', got %s", parsedMsg.Method)
	}
}

func TestStdioReadMessageProtocol(t *testing.T) {
	testJSON := `{"jsonrpc":"2.0","id":"test_456","result":{"uri":"file:///test.go","range":{"start":{"line":10,"character":5},"end":{"line":10,"character":15}}}}`
	testMessage := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(testJSON), testJSON)

	reader := bufio.NewReader(strings.NewReader(testMessage))
	client := &transport.StdioClient{}

	message, err := client.readMessage(reader)
	if err != nil {
		t.Fatalf("readMessage failed: %v", err)
	}

	var parsedMsg JSONRPCMessage
	if err := json.Unmarshal(message, &parsedMsg); err != nil {
		t.Fatalf("Failed to parse message: %v", err)
	}

	if parsedMsg.JSONRPC != "2.0" {
		t.Errorf("Expected JSON-RPC 2.0, got %s", parsedMsg.JSONRPC)
	}

	if parsedMsg.ID != "test_456" {
		t.Errorf("Expected ID 'test_456', got %v", parsedMsg.ID)
	}
}

func TestStdioReadMessageMalformed(t *testing.T) {
	testCases := []struct {
		name    string
		message string
	}{
		{
			name:    "missing content length",
			message: "\r\n\r\n{\"test\":\"data\"}",
		},
		{
			name:    "invalid content length",
			message: "Content-Length: invalid\r\n\r\n{\"test\":\"data\"}",
		},
		{
			name:    "content length too short",
			message: "Content-Length: 25\r\n\r\n{\"test\":\"data\"}",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reader := bufio.NewReader(strings.NewReader(tc.message))
			client := &transport.StdioClient{}

			_, err := client.readMessage(reader)
			if err == nil {
				t.Error("Expected error for malformed message")
			}
		})
	}
}

func TestStdioStartTwice(t *testing.T) {
	config := transport.ClientConfig{
		Command:   "cat",
		Args:      []string{},
		Transport: "stdio",
	}

	client, err := transport.NewStdioClient(config)
	if err != nil {
		t.Fatalf("NewStdioClient failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("First start failed: %v", err)
	}

	err = client.Start(ctx)
	if err == nil {
		t.Error("Second start should fail")
	}

	if !strings.Contains(err.Error(), "already active") {
		t.Errorf("Expected 'already active' error, got: %v", err)
	}

	if err := client.Stop(); err != nil {
		t.Logf("error stopping client: %v", err)
	}
}

func TestStdioStopInactive(t *testing.T) {
	config := transport.ClientConfig{
		Command:   "cat",
		Args:      []string{},
		Transport: "stdio",
	}

	client, err := transport.NewStdioClient(config)
	if err != nil {
		t.Fatalf("NewStdioClient failed: %v", err)
	}

	err = client.Stop()
	if err != nil {
		t.Errorf("Stop on inactive client should not error: %v", err)
	}
}

func TestStdioInvalidCommand(t *testing.T) {
	config := transport.ClientConfig{
		Command:   "non-existent-command-12345",
		Args:      []string{},
		Transport: "stdio",
	}

	client, err := transport.NewStdioClient(config)
	if err != nil {
		t.Fatalf("NewStdioClient failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = client.Start(ctx)
	if err == nil {
		t.Error("Start should fail with invalid command")
		if err := client.Stop(); err != nil {
			t.Logf("error stopping client: %v", err)
		}
	}
}

func TestStdioWriteMessageMarshalError(t *testing.T) {
	var buf bytes.Buffer
	client := &transport.StdioClient{
		stdin: &writeCloserWrapper{&buf},
	}

	message := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      "test",
		Method:  "test",
		Params:  make(chan int), // This cannot be marshaled to JSON
	}

	err := client.writeMessage(message)
	if err == nil {
		t.Error("writeMessage should fail with unmarshalable content")
	}

	if !strings.Contains(err.Error(), "marshal") {
		t.Errorf("Expected marshal error, got: %v", err)
	}
}

func TestStdioResponseCorrelation(t *testing.T) {
	mockServer := createMockLSPServerProgram(t, false, 0, false, 0, 0)
	defer func() {
		if err := os.Remove(mockServer); err != nil {
			t.Logf("cleanup error removing mock server: %v", err)
		}
	}()

	config := transport.ClientConfig{
		Command:   mockServer,
		Args:      []string{},
		Transport: "stdio",
	}

	client, err := transport.NewStdioClient(config)
	if err != nil {
		t.Fatalf("NewStdioClient failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() {
		if err := client.Stop(); err != nil {
			t.Logf("cleanup error stopping client: %v", err)
		}
	}()

	const numRequests = 5
	results := make([]json.RawMessage, numRequests)
	errors := make([]error, numRequests)

	var wg sync.WaitGroup
	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			requestCtx, requestCancel := context.WithTimeout(ctx, 5*time.Second)
			defer requestCancel()

			results[index], errors[index] = client.SendRequest(requestCtx, "textDocument/definition", map[string]interface{}{
				"textDocument": map[string]interface{}{"uri": fmt.Sprintf("file:///test%d.go", index)},
				"position":     map[string]interface{}{"line": index, "character": 5},
			})
		}(i)
	}

	wg.Wait()

	for i, err := range errors {
		if err != nil {
			t.Errorf("Request %d failed: %v", i, err)
		}
		if len(results[i]) == 0 {
			t.Errorf("Request %d got empty response", i)
		}
	}
}

func createMockLSPServerProgram(t *testing.T, shouldError bool, errorAfterN int, sendNotification bool, closeAfterN int, delay time.Duration) string {
	tmpDir := t.TempDir()
	serverPath := fmt.Sprintf("%s/mock_lsp_server", tmpDir)

	source := fmt.Sprintf(`package main

import (
	"lsp-gateway/internal/transport"
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type JSONRPCMessage struct {
	JSONRPC string      `+"`json:\"jsonrpc\"`"+`
	ID      interface{} `+"`json:\"id,omitempty\"`"+`
	Method  string      `+"`json:\"method,omitempty\"`"+`
	Params  interface{} `+"`json:\"params,omitempty\"`"+`
	Result  interface{} `+"`json:\"result,omitempty\"`"+`
	Error   *RPCError   `+"`json:\"error,omitempty\"`"+`
}

type RPCError struct {
	Code    int         `+"`json:\"code\"`"+`
	Message string      `+"`json:\"message\"`"+`
	Data    interface{} `+"`json:\"data,omitempty\"`"+`
}

func main() {
	shouldError := %t
	errorAfterN := %d
	sendNotification := %t
	closeAfterN := %d
	delay := %d * time.Nanosecond
	
	reader := bufio.NewReader(os.Stdin)
	requestCount := 0
	
	if sendNotification {
		notification := JSONRPCMessage{
			JSONRPC: "2.0",
			Method:  "window/showMessage",
			Params: map[string]interface{}{
				"type":    1,
				"message": "Mock LSP Server Started",
			},
		}
		writeMessage(notification)
	}
	
	for {
		if closeAfterN > 0 && requestCount >= closeAfterN {
			break
		}
		
		message, err := readMessage(reader)
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Printf("Error reading message: %%v", err)
			continue
		}
		
		var jsonrpcMsg JSONRPCMessage
		if err := json.Unmarshal(message, &jsonrpcMsg); err != nil {
			log.Printf("Error parsing JSON-RPC message: %%v", err)
			continue
		}
		
		if jsonrpcMsg.ID != nil {
			requestCount++
			
			if delay > 0 {
				time.Sleep(delay)
			}
			
			if jsonrpcMsg.Method == "initialize" {
				response := JSONRPCMessage{
					JSONRPC: "2.0",
					ID:      jsonrpcMsg.ID,
					Result: map[string]interface{}{
						"capabilities": map[string]interface{}{
							"textDocumentSync": 1,
							"definitionProvider": true,
							"referencesProvider": true,
						},
					},
				}
				writeMessage(response)
				continue
			}
			
			if shouldError && (errorAfterN == 0 || requestCount >= errorAfterN) {
				errorResponse := JSONRPCMessage{
					JSONRPC: "2.0",
					ID:      jsonrpcMsg.ID,
					Error: &RPCError{
						Code:    -32601,
						Message: "Mock error from test server",
					},
				}
				writeMessage(errorResponse)
				continue
			}
			
			var result interface{}
			switch jsonrpcMsg.Method {
			case "textDocument/definition":
				result = []map[string]interface{}{
					{
						"uri": "file:///test.go",
						"range": map[string]interface{}{
							"start": map[string]interface{}{"line": 10, "character": 5},
							"end":   map[string]interface{}{"line": 10, "character": 15},
						},
					},
				}
			case "textDocument/references":
				result = []map[string]interface{}{
					{
						"uri": "file:///test.go",
						"range": map[string]interface{}{
							"start": map[string]interface{}{"line": 5, "character": 0},
							"end":   map[string]interface{}{"line": 5, "character": 10},
						},
					},
				}
			case "textDocument/hover":
				result = map[string]interface{}{
					"contents": map[string]interface{}{
						"kind":  "markdown",
						"value": "**mockFunction()**\\n\\nA mock function for testing hover functionality",
					},
					"range": map[string]interface{}{
						"start": map[string]interface{}{"line": 3, "character": 5},
						"end":   map[string]interface{}{"line": 3, "character": 17},
					},
				}
			default:
				result = map[string]interface{}{
					"method": jsonrpcMsg.Method,
					"echo":   jsonrpcMsg.Params,
				}
			}
			
			response := JSONRPCMessage{
				JSONRPC: "2.0",
				ID:      jsonrpcMsg.ID,
				Result:  result,
			}
			writeMessage(response)
			
			if sendNotification && requestCount == 1 {
				notification := JSONRPCMessage{
					JSONRPC: "2.0",
					Method:  "textDocument/publishDiagnostics",
					Params: map[string]interface{}{
						"uri": "file:///test.go",
						"diagnostics": []interface{}{},
					},
				}
				writeMessage(notification)
			}
		}
	}
}

func writeMessage(msg JSONRPCMessage) error {
	jsonData, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %%w", err)
	}
	
	content := fmt.Sprintf("Content-Length: %%d\r\n\r\n%%s", len(jsonData), jsonData)
	_, err = os.Stdout.Write([]byte(content))
	return err
}

func readMessage(reader *bufio.Reader) ([]byte, error) {
	var contentLength int
	
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		
		if strings.HasPrefix(line, "Content-Length: ") {
			lengthStr := strings.TrimPrefix(line, "Content-Length: ")
			contentLength, err = strconv.Atoi(lengthStr)
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Length: %%s", lengthStr)
			}
		}
	}
	
	if contentLength == 0 {
		return nil, fmt.Errorf("no Content-Length header found")
	}
	
	body := make([]byte, contentLength)
	_, err := io.ReadFull(reader, body)
	if err != nil {
		return nil, fmt.Errorf("failed to read message body: %%w", err)
	}
	
	return body, nil
}`, shouldError, errorAfterN, sendNotification, closeAfterN, int64(delay))

	sourceFile := serverPath + ".go"
	if err := os.WriteFile(sourceFile, []byte(source), 0644); err != nil {
		t.Fatalf("Failed to write mock server source: %v", err)
	}

	modFile := fmt.Sprintf("%s/go.mod", tmpDir)
	modContent := "module mock-lsp-server\n\ngo 1.24\n"
	if err := os.WriteFile(modFile, []byte(modContent), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	cmd := exec.Command("go", "build", "-o", serverPath, sourceFile)
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build mock server: %v\nOutput: %s", err, output)
	}

	return serverPath
}

type writeCloserWrapper struct {
	io.Writer
}

func (w *writeCloserWrapper) Close() error {
	return nil
}
