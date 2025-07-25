package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// LSPWorkflowScenario represents a complete LSP workflow test scenario
type LSPWorkflowScenario struct {
	Name        string
	Description string
	ProjectType string
	Language    string
	Steps       []WorkflowStep
	Timeout     time.Duration
	Parallel    bool
}

// WorkflowStep represents a single step in an LSP workflow
type WorkflowStep struct {
	Name           string
	Method         string
	URI            string
	Content        string
	Position       Position
	ExpectedResult interface{}
	ValidationFunc func(result interface{}) error
}

type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// LSPWorkflowManager manages LSP workflow test scenarios
type LSPWorkflowManager struct {
	gatewayURL string
	client     *http.Client
	mu         sync.RWMutex
	scenarios  map[string]*LSPWorkflowScenario
}

// NewLSPWorkflowManager creates a new workflow manager
func NewLSPWorkflowManager(gatewayURL string) *LSPWorkflowManager {
	return &LSPWorkflowManager{
		gatewayURL: gatewayURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		scenarios: make(map[string]*LSPWorkflowScenario),
	}
}

// RegisterScenario registers a new workflow scenario
func (m *LSPWorkflowManager) RegisterScenario(scenario *LSPWorkflowScenario) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.scenarios[scenario.Name] = scenario
}

// ExecuteScenario executes a specific workflow scenario
func (m *LSPWorkflowManager) ExecuteScenario(t *testing.T, scenarioName string, projectDir string) error {
	m.mu.RLock()
	scenario, exists := m.scenarios[scenarioName]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("scenario %s not found", scenarioName)
	}

	t.Logf("Executing LSP workflow scenario: %s", scenario.Name)
	t.Logf("Description: %s", scenario.Description)

	ctx, cancel := context.WithTimeout(context.Background(), scenario.Timeout)
	defer cancel()

	// Initialize LSP session
	if err := m.initializeLSPSession(ctx, projectDir, scenario.Language); err != nil {
		return fmt.Errorf("failed to initialize LSP session: %w", err)
	}

	// Execute workflow steps
	for i, step := range scenario.Steps {
		t.Logf("Executing step %d: %s", i+1, step.Name)
		
		result, err := m.executeStep(ctx, step, projectDir)
		if err != nil {
			return fmt.Errorf("step %d (%s) failed: %w", i+1, step.Name, err)
		}

		// Validate result if validation function provided
		if step.ValidationFunc != nil {
			if err := step.ValidationFunc(result); err != nil {
				return fmt.Errorf("step %d (%s) validation failed: %w", i+1, step.Name, err)
			}
		}

		t.Logf("Step %d completed successfully", i+1)
	}

	t.Logf("Scenario %s completed successfully", scenarioName)
	return nil
}

// initializeLSPSession initializes an LSP session for the given project
func (m *LSPWorkflowManager) initializeLSPSession(ctx context.Context, projectDir, language string) error {
	request := map[string]interface{}{
		"method": "initialize",
		"params": map[string]interface{}{
			"processId": nil,
			"rootUri":   fmt.Sprintf("file://%s", projectDir),
			"capabilities": map[string]interface{}{
				"textDocument": map[string]interface{}{
					"definition": map[string]interface{}{
						"dynamicRegistration": true,
						"linkSupport":         true,
					},
					"hover": map[string]interface{}{
						"dynamicRegistration": true,
						"contentFormat":       []string{"markdown", "plaintext"},
					},
					"references": map[string]interface{}{
						"dynamicRegistration": true,
					},
					"documentSymbol": map[string]interface{}{
						"dynamicRegistration": true,
						"hierarchicalDocumentSymbolSupport": true,
					},
				},
				"workspace": map[string]interface{}{
					"symbol": map[string]interface{}{
						"dynamicRegistration": true,
					},
				},
			},
		},
	}

	_, err := m.sendLSPRequest(ctx, request, language)
	return err
}

// executeStep executes a single workflow step
func (m *LSPWorkflowManager) executeStep(ctx context.Context, step WorkflowStep, projectDir string) (interface{}, error) {
	var request map[string]interface{}

	switch step.Method {
	case "textDocument/didOpen":
		content := step.Content
		if content == "" && step.URI != "" {
			// Read content from file
			filePath := filepath.Join(projectDir, step.URI)
			data, err := os.ReadFile(filePath)
			if err != nil {
				return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
			}
			content = string(data)
		}

		request = map[string]interface{}{
			"method": step.Method,
			"params": map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri":        fmt.Sprintf("file://%s", filepath.Join(projectDir, step.URI)),
					"languageId": "go", // TODO: detect from file extension
					"version":    1,
					"text":       content,
				},
			},
		}

	case "textDocument/definition":
		request = map[string]interface{}{
			"method": step.Method,
			"params": map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": fmt.Sprintf("file://%s", filepath.Join(projectDir, step.URI)),
				},
				"position": step.Position,
			},
		}

	case "textDocument/hover":
		request = map[string]interface{}{
			"method": step.Method,
			"params": map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": fmt.Sprintf("file://%s", filepath.Join(projectDir, step.URI)),
				},
				"position": step.Position,
			},
		}

	case "textDocument/references":
		request = map[string]interface{}{
			"method": step.Method,
			"params": map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": fmt.Sprintf("file://%s", filepath.Join(projectDir, step.URI)),
				},
				"position": step.Position,
				"context": map[string]interface{}{
					"includeDeclaration": true,
				},
			},
		}

	case "textDocument/documentSymbol":
		request = map[string]interface{}{
			"method": step.Method,
			"params": map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": fmt.Sprintf("file://%s", filepath.Join(projectDir, step.URI)),
				},
			},
		}

	case "workspace/symbol":
		request = map[string]interface{}{
			"method": step.Method,
			"params": map[string]interface{}{
				"query": step.Content,
			},
		}

	default:
		return nil, fmt.Errorf("unsupported LSP method: %s", step.Method)
	}

	return m.sendLSPRequest(ctx, request, "go") // TODO: pass language from step
}

// sendLSPRequest sends an LSP request to the gateway
func (m *LSPWorkflowManager) sendLSPRequest(ctx context.Context, request map[string]interface{}, language string) (interface{}, error) {
	// Add JSON-RPC envelope
	envelope := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      time.Now().UnixNano(),
		"method":  request["method"],
		"params":  request["params"],
	}

	jsonData, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/jsonrpc", m.gatewayURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Language", language)

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LSP request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if errData, exists := response["error"]; exists {
		return nil, fmt.Errorf("LSP error: %v", errData)
	}

	return response["result"], nil
}

// GetDefaultGoWorkflowScenario returns a default Go workflow scenario for testing
func GetDefaultGoWorkflowScenario() *LSPWorkflowScenario {
	return &LSPWorkflowScenario{
		Name:        "go-basic-workflow",
		Description: "Basic Go LSP workflow with definition, hover, and references",
		ProjectType: "go-module",
		Language:    "go",
		Timeout:     2 * time.Minute,
		Parallel:    false,
		Steps: []WorkflowStep{
			{
				Name:   "Open main.go file",
				Method: "textDocument/didOpen",
				URI:    "main.go",
			},
			{
				Name:     "Get definition of main function",
				Method:   "textDocument/definition",
				URI:      "main.go",
				Position: Position{Line: 5, Character: 5}, // Adjust based on actual file
			},
			{
				Name:     "Get hover information",
				Method:   "textDocument/hover",
				URI:      "main.go",
				Position: Position{Line: 5, Character: 5},
			},
			{
				Name:     "Find references",
				Method:   "textDocument/references",
				URI:      "main.go",
				Position: Position{Line: 5, Character: 5},
			},
			{
				Name:   "Get document symbols",
				Method: "textDocument/documentSymbol",
				URI:    "main.go",
			},
			{
				Name:    "Search workspace symbols",
				Method:  "workspace/symbol",
				Content: "main",
			},
		},
	}
}

// RunStandardLSPWorkflowTests runs a standard set of LSP workflow tests
func RunStandardLSPWorkflowTests(t *testing.T, gatewayURL string, projectDir string) {
	manager := NewLSPWorkflowManager(gatewayURL)
	
	// Register default scenarios
	manager.RegisterScenario(GetDefaultGoWorkflowScenario())
	
	// Execute scenarios
	scenarios := []string{"go-basic-workflow"}
	
	for _, scenarioName := range scenarios {
		t.Run(scenarioName, func(t *testing.T) {
			err := manager.ExecuteScenario(t, scenarioName, projectDir)
			require.NoError(t, err, "Scenario %s should complete successfully", scenarioName)
		})
	}
}

// ValidateDefinitionResponse validates a textDocument/definition response
func ValidateDefinitionResponse(result interface{}) error {
	if result == nil {
		return fmt.Errorf("definition result is nil")
	}

	// LSP definition response can be Location | Location[] | LocationLink[]
	switch v := result.(type) {
	case []interface{}:
		if len(v) == 0 {
			return fmt.Errorf("definition response is empty array")
		}
		
		// Validate first location
		if loc, ok := v[0].(map[string]interface{}); ok {
			if _, hasURI := loc["uri"]; !hasURI {
				return fmt.Errorf("definition location missing uri")
			}
			if _, hasRange := loc["range"]; !hasRange {
				return fmt.Errorf("definition location missing range")
			}
		} else {
			return fmt.Errorf("invalid definition location format")
		}
		
	case map[string]interface{}:
		if _, hasURI := v["uri"]; !hasURI {
			return fmt.Errorf("definition location missing uri")
		}
		if _, hasRange := v["range"]; !hasRange {
			return fmt.Errorf("definition location missing range")
		}
		
	default:
		return fmt.Errorf("unexpected definition response format: %T", result)
	}

	return nil
}

// ValidateHoverResponse validates a textDocument/hover response
func ValidateHoverResponse(result interface{}) error {
	if result == nil {
		return nil // Hover can legitimately return null
	}

	hover, ok := result.(map[string]interface{})
	if !ok {
		return fmt.Errorf("hover response is not an object")
	}

	if contents, hasContents := hover["contents"]; hasContents {
		switch contents.(type) {
		case string:
			// Valid: string content
		case []interface{}:
			// Valid: array of MarkupContent or strings
		case map[string]interface{}:
			// Valid: MarkupContent object
		default:
			return fmt.Errorf("invalid hover contents format: %T", contents)
		}
	} else {
		return fmt.Errorf("hover response missing contents")
	}

	return nil
}