package mcp_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"lsp-gateway/mcp"
	"net"
	"strings"
	"testing"
	"time"
)

func TestMCPProtocolCompliance(t *testing.T) {
	config := &mcp.ServerConfig{
		Name:          "test-server",
		Version:       "1.0.0",
		LSPGatewayURL: fmt.Sprintf("http://localhost:%d", testPort),
		Timeout:       30 * time.Second,
	}

	server := mcp.NewServer(config)

	t.Run("ReadMessageWithHeaders", func(t *testing.T) {
		testMsg := mcp.MCPMessage{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "initialize",
			Params: map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]interface{}{},
				"clientInfo": map[string]interface{}{
					"name":    "test-client",
					"version": "1.0.0",
				},
			},
		}

		jsonData, err := json.Marshal(testMsg)
		if err != nil {
			t.Fatalf("Failed to marshal test message: %v", err)
		}

		content := string(jsonData)
		input := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(content), content)

		inputReader := strings.NewReader(input)
		outputBuffer := &bytes.Buffer{}
		server.SetIO(inputReader, outputBuffer)

		go func() {
			if err := server.Start(); err != nil {
				t.Logf("server start error: %v", err)
			}
		}()

		time.Sleep(100 * time.Millisecond)
		if err := server.Stop(); err != nil {
			t.Logf("server stop error: %v", err)
		}

		output := outputBuffer.String()
		if !strings.Contains(output, "Content-Length:") {
			t.Errorf("Expected Content-Length header in output, got: %s", output)
		}

		if !strings.Contains(output, "\r\n\r\n") {
			t.Errorf("Expected double CRLF separator in output, got: %s", output)
		}
	})

	t.Run("WriteMessageWithHeaders", func(t *testing.T) {
		outputBuffer := &bytes.Buffer{}
		server.SetIO(nil, outputBuffer)

		response := mcp.MCPMessage{
			JSONRPC: "2.0",
			ID:      1,
			Result: map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]interface{}{},
				"serverInfo": map[string]interface{}{
					"name":    "test-server",
					"version": "1.0.0",
				},
			},
		}

		err := server.sendMessage(response)
		if err != nil {
			t.Fatalf("Failed to send message: %v", err)
		}

		output := outputBuffer.String()

		if !strings.HasPrefix(output, "Content-Length: ") {
			t.Errorf("Expected output to start with Content-Length header, got: %s", output)
		}

		lines := strings.Split(output, "\r\n")
		if len(lines) < 3 {
			t.Fatalf("Expected at least 3 lines (header, empty, content), got: %v", lines)
		}

		headerLine := lines[0]
		if !strings.HasPrefix(headerLine, "Content-Length: ") {
			t.Errorf("Invalid header format: %s", headerLine)
		}

		if lines[1] != "" {
			t.Errorf("Expected empty line after headers, got: %s", lines[1])
		}

		jsonStart := strings.Index(output, "\r\n\r\n") + 4
		if jsonStart == 3 {
			t.Errorf("Could not find double CRLF separator")
		}

		jsonContent := output[jsonStart:]
		var parsedMsg mcp.MCPMessage
		err = json.Unmarshal([]byte(jsonContent), &parsedMsg)
		if err != nil {
			t.Errorf("Failed to parse JSON content: %v", err)
		}

		if parsedMsg.JSONRPC != "2.0" {
			t.Errorf("Expected JSONRPC 2.0, got: %s", parsedMsg.JSONRPC)
		}
	})
}

func TestMessageFraming(t *testing.T) {
	config := mcp.DefaultConfig()
	server := mcp.NewServer(config)

	t.Run("MultipleMessages", func(t *testing.T) {
		msg1 := `{"jsonrpc":"2.0","id":1,"method":"ping"}`
		msg2 := `{"jsonrpc":"2.0","id":2,"method":"ping"}`

		input1 := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(msg1), msg1)
		input2 := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(msg2), msg2)

		combinedInput := input1 + input2

		inputReader := strings.NewReader(combinedInput)
		outputBuffer := &bytes.Buffer{}
		server.SetIO(inputReader, outputBuffer)

		go func() {
			if err := server.Start(); err != nil {
				t.Logf("server start error: %v", err)
			}
		}()

		time.Sleep(200 * time.Millisecond)
		if err := server.Stop(); err != nil {
			t.Logf("server stop error: %v", err)
		}

		output := outputBuffer.String()

		responseCount := strings.Count(output, "Content-Length:")
		if responseCount != 2 {
			t.Errorf("Expected 2 responses, found %d in output: %s", responseCount, output)
		}
	})

	t.Run("InvalidContentLength", func(t *testing.T) {
		invalidInput := "Content-Length: invalid\r\n\r\n{}"

		inputReader := strings.NewReader(invalidInput)
		outputBuffer := &bytes.Buffer{}
		server.SetIO(inputReader, outputBuffer)

		go func() {
			if err := server.Start(); err != nil {
				t.Logf("server start error: %v", err)
			}
		}()

		time.Sleep(100 * time.Millisecond)
		if err := server.Stop(); err != nil {
			t.Logf("server stop error: %v", err)
		}

	})

	t.Run("MissingContentLength", func(t *testing.T) {
		invalidInput := "\r\n{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"ping\"}"

		inputReader := strings.NewReader(invalidInput)
		outputBuffer := &bytes.Buffer{}
		server.SetIO(inputReader, outputBuffer)

		go func() {
			if err := server.Start(); err != nil {
				t.Logf("server start error: %v", err)
			}
		}()

		time.Sleep(100 * time.Millisecond)
		if err := server.Stop(); err != nil {
			t.Logf("server stop error: %v", err)
		}

	})
}

// findAvailablePort finds an available port for testing
func findAvailablePort() int {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}
