package transport

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

type StdioClient struct {
	config ClientConfig
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	mu       sync.RWMutex
	active   bool
	requests map[string]chan json.RawMessage
	nextID   int

	stopCh chan struct{}
	done   chan struct{}
}

func NewStdioClient(config ClientConfig) (*StdioClient, error) {
	return &StdioClient{
		config:   config,
		requests: make(map[string]chan json.RawMessage),
		stopCh:   make(chan struct{}),
		done:     make(chan struct{}),
	}, nil
}

func (c *StdioClient) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.active {
		return fmt.Errorf("client already active")
	}

	c.cmd = exec.CommandContext(ctx, c.config.Command, c.config.Args...)

	var err error
	c.stdin, err = c.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	c.stdout, err = c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	c.stderr, err = c.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start LSP server: %w", err)
	}

	c.active = true

	go c.handleResponses()

	go c.logStderr()

	return nil
}

func (c *StdioClient) SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	c.mu.RLock()
	if !c.active {
		c.mu.RUnlock()
		return nil, errors.New(ERROR_CLIENT_NOT_ACTIVE)
	}
	c.mu.RUnlock()

	c.mu.Lock()
	c.nextID++
	id := fmt.Sprintf("req_%d", c.nextID)
	c.mu.Unlock()

	respCh := make(chan json.RawMessage, 1)

	c.mu.Lock()
	c.requests[id] = respCh
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.requests, id)
		close(respCh)
		c.mu.Unlock()
	}()

	request := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	if err := c.writeMessage(request); err != nil {
		return nil, fmt.Errorf(ERROR_SEND_REQUEST, err)
	}

	select {
	case response := <-respCh:
		return response, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.stopCh:
		return nil, errors.New(ERROR_CLIENT_STOPPED)
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("request timeout")
	}
}

func (c *StdioClient) SendNotification(ctx context.Context, method string, params interface{}) error {
	c.mu.RLock()
	if !c.active {
		c.mu.RUnlock()
		return errors.New(ERROR_CLIENT_NOT_ACTIVE)
	}
	c.mu.RUnlock()

	notification := JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}

	if err := c.writeMessage(notification); err != nil {
		return fmt.Errorf(ERROR_SEND_NOTIFICATION, err)
	}

	return nil
}

func (c *StdioClient) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.active {
		return nil
	}

	c.active = false

	close(c.stopCh)

	if c.stdin != nil {
		_ = c.stdin.Close()
	}

	if c.cmd != nil && c.cmd.Process != nil {
		done := make(chan error, 1)
		go func() {
			done <- c.cmd.Wait()
		}()

		select {
		case err := <-done:
			if err != nil {
				log.Printf("LSP server exited with error: %v", err)
			}
		case <-time.After(5 * time.Second):
			log.Println("LSP server did not exit gracefully, killing process")
			if err := c.cmd.Process.Kill(); err != nil {
				log.Printf("Failed to kill LSP server process: %v", err)
			}
		}
	}

	if c.stdout != nil {
		_ = c.stdout.Close()
	}
	if c.stderr != nil {
		_ = c.stderr.Close()
	}

	<-c.done

	return nil
}

func (c *StdioClient) IsActive() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.active
}

func (c *StdioClient) handleResponses() {
	defer close(c.done)

	reader := bufio.NewReader(c.stdout)

	for {
		select {
		case <-c.stopCh:
			return
		default:
		}

		message, err := c.readMessage(reader)
		if err != nil {
			if err == io.EOF {
				log.Println("LSP server closed connection")
				return
			}
			log.Printf("Error reading message: %v", err)
			continue
		}

		var jsonrpcMsg JSONRPCMessage
		if err := json.Unmarshal(message, &jsonrpcMsg); err != nil {
			log.Printf("Error parsing JSON-RPC message: %v", err)
			continue
		}

		if jsonrpcMsg.ID != nil {
			c.handleResponse(jsonrpcMsg)
		} else {
			c.handleNotification(jsonrpcMsg)
		}
	}
}

func (c *StdioClient) handleResponse(msg JSONRPCMessage) {
	if msg.ID == nil {
		return
	}

	idStr := fmt.Sprintf("%v", msg.ID)

	c.mu.RLock()
	respCh, exists := c.requests[idStr]
	c.mu.RUnlock()

	if !exists {
		log.Printf("Received response for unknown request ID: %v", msg.ID)
		return
	}

	var result json.RawMessage
	if msg.Error != nil {
		errorData, _ := json.Marshal(msg.Error)
		result = errorData
	} else {
		resultData, _ := json.Marshal(msg.Result)
		result = resultData
	}

	select {
	case respCh <- result:
	default:
	}
}

func (c *StdioClient) handleNotification(msg JSONRPCMessage) {
	log.Printf("Received notification: %s", msg.Method)

}

func (c *StdioClient) writeMessage(msg JSONRPCMessage) error {
	jsonData, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf(ERROR_MARSHAL_MESSAGE, err)
	}

	content := fmt.Sprintf(PROTOCOL_HEADER_FORMAT, len(jsonData), jsonData)

	_, err = c.stdin.Write([]byte(content))
	if err != nil {
		return fmt.Errorf(ERROR_WRITE_MESSAGE, err)
	}

	return nil
}

func (c *StdioClient) readMessage(reader *bufio.Reader) ([]byte, error) {
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

		if strings.HasPrefix(line, PROTOCOL_CONTENT_LENGTH_PREFIX) {
			lengthStr := strings.TrimPrefix(line, PROTOCOL_CONTENT_LENGTH_PREFIX)
			contentLength, err = strconv.Atoi(lengthStr)
			if err != nil {
				return nil, fmt.Errorf(ERROR_INVALID_CONTENT_LENGTH, lengthStr)
			}
		}
	}

	if contentLength == 0 {
		return nil, fmt.Errorf("no Content-Length header found")
	}

	body := make([]byte, contentLength)
	_, err := io.ReadFull(reader, body)
	if err != nil {
		return nil, fmt.Errorf("failed to read message body: %w", err)
	}

	return body, nil
}

func (c *StdioClient) logStderr() {
	scanner := bufio.NewScanner(c.stderr)
	for scanner.Scan() {
		log.Printf("LSP stderr: %s", scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		log.Printf("Error reading stderr: %v", err)
	}
}
