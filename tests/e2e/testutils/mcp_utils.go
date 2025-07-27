package testutils

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// SendMCPStdioMessage sends a JSON-RPC message via STDIO and reads the response
func SendMCPStdioMessage(stdin io.WriteCloser, reader *bufio.Reader, msg MCPMessage) (*MCPMessage, error) {
	// Serialize message
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	// Send with Content-Length header
	message := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(msgBytes), msgBytes)
	_, err = stdin.Write([]byte(message))
	if err != nil {
		return nil, err
	}

	// Read response
	return ReadMCPStdioMessage(reader)
}

// ReadMCPStdioMessage reads a JSON-RPC message from STDIO
func ReadMCPStdioMessage(reader *bufio.Reader) (*MCPMessage, error) {
	// Read Content-Length header
	var contentLength int
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			break // Empty line indicates end of headers
		}

		if strings.HasPrefix(line, "Content-Length:") {
			lengthStr := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
			contentLength, err = strconv.Atoi(lengthStr)
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Length: %s", lengthStr)
			}
		}
	}

	if contentLength == 0 {
		return nil, fmt.Errorf("missing Content-Length header")
	}

	// Read message body
	messageBytes := make([]byte, contentLength)
	_, err := io.ReadFull(reader, messageBytes)
	if err != nil {
		return nil, err
	}

	// Parse JSON-RPC message
	var response MCPMessage
	err = json.Unmarshal(messageBytes, &response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}