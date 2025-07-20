package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

type SimpleTCPEchoServer struct {
	listener net.Listener
	port     int
	messages chan []byte
	closed   chan bool
}

func NewSimpleTCPEchoServer() (*SimpleTCPEchoServer, error) {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, err
	}

	port := listener.Addr().(*net.TCPAddr).Port

	return &SimpleTCPEchoServer{
		listener: listener,
		port:     port,
		messages: make(chan []byte, 10),
		closed:   make(chan bool, 1),
	}, nil
}

func (s *SimpleTCPEchoServer) Start() {
	go s.acceptConnections()
}

func (s *SimpleTCPEchoServer) Stop() error {
	close(s.closed)
	return s.listener.Close()
}

func (s *SimpleTCPEchoServer) Port() int {
	return s.port
}

func (s *SimpleTCPEchoServer) GetMessages() [][]byte {
	var messages [][]byte
	for {
		select {
		case msg := <-s.messages:
			messages = append(messages, msg)
		default:
			return messages
		}
	}
}

func (s *SimpleTCPEchoServer) acceptConnections() {
	for {
		select {
		case <-s.closed:
			return
		default:
		}

		conn, err := s.listener.Accept()
		if err != nil {
			return
		}

		go s.handleConnection(conn)
	}
}

func (s *SimpleTCPEchoServer) handleConnection(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	reader := bufio.NewReader(conn)

	contentLength := 0
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		if strings.HasPrefix(line, "Content-Length:") {
			lengthStr := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
			contentLength, _ = strconv.Atoi(lengthStr)
		}
	}

	if contentLength > 0 {
		body := make([]byte, contentLength)
		if _, err := io.ReadFull(reader, body); err == nil {
			select {
			case s.messages <- body:
			default:
			}
		}
	}
}

func TestTCPClientSendMessage(t *testing.T) {
	server, err := NewSimpleTCPEchoServer()
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer func() { _ = server.Stop() }()

	server.Start()

	config := ClientConfig{
		Command:   fmt.Sprintf("localhost:%d", server.Port()),
		Transport: "tcp",
	}

	client, err := NewTCPClient(config)
	if err != nil {
		t.Fatalf("NewTCPClient failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = client.Stop() }()

	tcpClient := client.(*TCPClient)
	testMessage := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      "1",
		Method:  "test",
		Params:  map[string]string{"key": "value"},
	}

	err = tcpClient.sendMessage(testMessage)
	if err != nil {
		t.Fatalf("sendMessage failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	messages := server.GetMessages()
	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}

	var receivedMessage JSONRPCMessage
	if err := json.Unmarshal(messages[0], &receivedMessage); err != nil {
		t.Fatalf("Failed to unmarshal received message: %v", err)
	}

	if receivedMessage.Method != "test" {
		t.Errorf("Expected method 'test', got %s", receivedMessage.Method)
	}

	if receivedMessage.ID != "1" {
		t.Errorf("Expected ID '1', got %v", receivedMessage.ID)
	}
}

func TestTCPClientHandleMessage(t *testing.T) {
	testPort := allocateTestPort(t)
	config := ClientConfig{
		Command:   fmt.Sprintf("localhost:%d", testPort), // Won't actually connect
		Transport: "tcp",
	}

	client, err := NewTCPClient(config)
	if err != nil {
		t.Fatalf("NewTCPClient failed: %v", err)
	}

	tcpClient := client.(*TCPClient)

	tcpClient.requests = make(map[string]chan json.RawMessage)

	respCh := make(chan json.RawMessage, 1)
	tcpClient.requests["1"] = respCh

	responseMsg := &JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      "1",
		Result:  map[string]string{"test": "value"},
	}

	tcpClient.handleMessage(responseMsg)

	select {
	case response := <-respCh:
		var result map[string]string
		if err := json.Unmarshal(response, &result); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}
		if result["test"] != "value" {
			t.Errorf("Expected test=value, got test=%s", result["test"])
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Response not received within timeout")
	}

	errorRespCh := make(chan json.RawMessage, 1)
	tcpClient.requests["2"] = errorRespCh

	errorMsg := &JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      "2",
		Error: &RPCError{
			Code:    -32601,
			Message: "Method not found",
		},
	}

	tcpClient.handleMessage(errorMsg)

	select {
	case errorResponse := <-errorRespCh:
		var errorResult RPCError
		if err := json.Unmarshal(errorResponse, &errorResult); err != nil {
			t.Fatalf("Failed to unmarshal error response: %v", err)
		}
		if errorResult.Code != -32601 {
			t.Errorf("Expected error code -32601, got %d", errorResult.Code)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Error response not received within timeout")
	}

	notificationMsg := &JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  "textDocument/publishDiagnostics",
		Params:  map[string]interface{}{"uri": "file:///test.go"},
	}

	tcpClient.handleMessage(notificationMsg)

	close(respCh)
	close(errorRespCh)
}

func TestTCPClientSendNotificationBasic(t *testing.T) {
	server, err := NewSimpleTCPEchoServer()
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer func() { _ = server.Stop() }()

	server.Start()

	config := ClientConfig{
		Command:   fmt.Sprintf("localhost:%d", server.Port()),
		Transport: "tcp",
	}

	client, err := NewTCPClient(config)
	if err != nil {
		t.Fatalf("NewTCPClient failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() { _ = client.Stop() }()

	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":     "file:///test.go",
			"version": 1,
		},
	}

	err = client.SendNotification(ctx, "textDocument/didOpen", params)
	if err != nil {
		t.Fatalf("SendNotification failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	messages := server.GetMessages()
	if len(messages) != 1 {
		t.Fatalf("Expected 1 notification, got %d", len(messages))
	}

	var notification JSONRPCMessage
	if err := json.Unmarshal(messages[0], &notification); err != nil {
		t.Fatalf("Failed to unmarshal notification: %v", err)
	}

	if notification.Method != "textDocument/didOpen" {
		t.Errorf("Expected method 'textDocument/didOpen', got %s", notification.Method)
	}

	if notification.ID != nil {
		t.Error("Notification should not have an ID")
	}
}

func TestTCPClientMessageHandling(t *testing.T) {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer func() { _ = listener.Close() }()

	port := listener.Addr().(*net.TCPAddr).Port

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()

		time.Sleep(200 * time.Millisecond)
	}()

	config := ClientConfig{
		Command:   fmt.Sprintf("localhost:%d", port),
		Transport: "tcp",
	}

	client, err := NewTCPClient(config)
	if err != nil {
		t.Fatalf("NewTCPClient failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if !client.IsActive() {
		t.Error("Client should be active with good connection")
	}

	_ = client.Stop()
}
