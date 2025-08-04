package testutils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HttpClientConfig configures the simplified HttpClient
type HttpClientConfig struct {
	BaseURL         string
	Timeout         time.Duration
	MaxRetries      int
	RetryDelay      time.Duration
	WorkspaceID     string
	ProjectPath     string
	UserAgent       string
	WorkspaceRoot   string
	EnableLogging   bool
	EnableRecording bool
}

// HttpClient provides simple HTTP client for testing LSP Gateway
type HttpClient struct {
	config HttpClientConfig
	client *http.Client
}

// NewHttpClient creates a new simplified HttpClient
func NewHttpClient(config HttpClientConfig) *HttpClient {
	if config.BaseURL == "" {
		config.BaseURL = "http://localhost:8080"
	}
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = 500 * time.Millisecond
	}
	if config.UserAgent == "" {
		config.UserAgent = "LSP-Gateway-E2E-Test/1.0"
	}

	httpClient := &http.Client{
		Timeout: config.Timeout,
	}

	return &HttpClient{
		config: config,
		client: httpClient,
	}
}

// HealthCheck performs a basic health check
func (c *HttpClient) HealthCheck() error {
	url := fmt.Sprintf("%s/health", c.config.BaseURL)
	return QuickConnectivityCheck(url)
}

// FastHealthCheck alias for backward compatibility
func (c *HttpClient) FastHealthCheck() error {
	return c.HealthCheck()
}

// Note: Removed typed LSP methods to avoid internal package dependencies
// Use MakeRawJSONRPCRequest for E2E testing instead

// sendLSPRequest sends a JSON-RPC LSP request
func (c *HttpClient) sendLSPRequest(ctx context.Context, method string, params interface{}, result interface{}) error {
	requestBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/jsonrpc", c.config.BaseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.config.UserAgent)
	if c.config.WorkspaceID != "" {
		req.Header.Set("X-Workspace-ID", c.config.WorkspaceID)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if errField, ok := response["error"]; ok && errField != nil {
		return fmt.Errorf("LSP error: %v", errField)
	}

	if result != nil && response["result"] != nil {
		resultBytes, err := json.Marshal(response["result"])
		if err != nil {
			return fmt.Errorf("failed to marshal result: %w", err)
		}
		if err := json.Unmarshal(resultBytes, result); err != nil {
			return fmt.Errorf("failed to unmarshal result: %w", err)
		}
	}

	return nil
}

// Close cleans up the HttpClient resources
func (c *HttpClient) Close() error {
	// Nothing to cleanup for simplified client
	return nil
}

// SetWorkspaceRoot sets the workspace root path
func (c *HttpClient) SetWorkspaceRoot(root string) {
	c.config.WorkspaceRoot = root
}

// GetWorkspaceRoot returns the workspace root path
func (c *HttpClient) GetWorkspaceRoot() string {
	return c.config.WorkspaceRoot
}

func (c *HttpClient) ValidateConnection() error {
	return c.FastHealthCheck()
}

// MakeRawJSONRPCRequest makes a raw JSON-RPC request and returns the parsed response
func (c *HttpClient) MakeRawJSONRPCRequest(ctx context.Context, request map[string]interface{}) (map[string]interface{}, error) {
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/jsonrpc", c.config.BaseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.config.UserAgent)
	if c.config.WorkspaceID != "" {
		req.Header.Set("X-Workspace-ID", c.config.WorkspaceID)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return response, nil
}
