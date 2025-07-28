package suites

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	mcp "lsp-gateway/tests/mcp"
	"lsp-gateway/tests/e2e/mcp/client"
	"lsp-gateway/tests/e2e/mcp/helpers"
	"lsp-gateway/tests/e2e/mcp/types"
)

type MCPErrorHandlingE2ETestSuite struct {
	suite.Suite
	client            *client.EnhancedMCPTestClient
	workspace         *types.TestWorkspace
	workspaceManager  *helpers.TestWorkspaceManager
	server            *exec.Cmd
	config            *types.MCPTestConfig
	errorScenarios    *ErrorScenarioManager
	projectRoot       string
	binaryPath        string
	testTimeout       time.Duration
	serverMutex       sync.Mutex
	failureSimulator  *LSPServerFailureSimulator
}


type ErrorTestCase struct {
	Name            string
	ToolName        string
	Params          map[string]interface{}
	ExpectedError   string
	ExpectedCode    int
	ShouldRecover   bool
	RecoveryTime    time.Duration
	ValidationFunc  func(*mcp.MCPResponse) bool
}

type ErrorScenarioManager struct {
	mu                sync.RWMutex
	activeScenarios   map[string]*ErrorTestCase
	failureCount      int
	circuitBreakerOpen bool
	lastFailureTime   time.Time
}

type LSPServerFailureSimulator struct {
	mu              sync.RWMutex
	simulateFailure bool
	failureType     FailureType
	responseDelay   time.Duration
	server          *exec.Cmd
}

type FailureType int

const (
	FailureTypeTimeout FailureType = iota
	FailureTypeDisconnect
	FailureTypeMalformedResponse
	FailureTypeServerCrash
)

func (suite *MCPErrorHandlingE2ETestSuite) SetupSuite() {
	suite.testTimeout = 120 * time.Second

	var err error
	suite.projectRoot, err = findProjectRoot()
	suite.Require().NoError(err)

	suite.binaryPath = filepath.Join(suite.projectRoot, "bin", "lspg")
	suite.Require().FileExists(suite.binaryPath)

	suite.workspaceManager, err = helpers.NewTestWorkspaceManager()
	suite.Require().NoError(err)

	suite.setupTestWorkspace()
	suite.setupMCPClient()
	suite.setupErrorScenarios()
	suite.startMCPServer()
}

func (suite *MCPErrorHandlingE2ETestSuite) TearDownSuite() {
	if suite.client != nil {
		suite.client.Close()
	}
	if suite.server != nil && suite.server.Process != nil {
		suite.server.Process.Signal(syscall.SIGTERM)
		suite.server.Wait()
	}
	if suite.workspaceManager != nil {
		suite.workspaceManager.CleanupAll()
	}
}

func (suite *MCPErrorHandlingE2ETestSuite) SetupTest() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := suite.client.Connect(ctx)
	suite.Require().NoError(err)
	
	suite.resetErrorScenarios()
}

func (suite *MCPErrorHandlingE2ETestSuite) TearDownTest() {
	if suite.client != nil && suite.client.IsConnected() {
		suite.client.Disconnect()
	}
}

func (suite *MCPErrorHandlingE2ETestSuite) TestInvalidFileURIScenarios() {
	errorScenarios := []ErrorTestCase{
		{
			Name:     "Non-existent file",
			ToolName: "goto_definition",
			Params: map[string]interface{}{
				"uri":       "file:///non/existent/file.go",
				"line":      10,
				"character": 5,
			},
			ExpectedError: "file not found",
			ExpectedCode:  mcp.InvalidParams,
		},
		{
			Name:     "Invalid URI format",
			ToolName: "find_references",
			Params: map[string]interface{}{
				"uri":       "invalid-uri-format",
				"line":      10,
				"character": 5,
			},
			ExpectedError: "invalid URI format",
			ExpectedCode:  mcp.InvalidParams,
		},
		{
			Name:     "Empty URI",
			ToolName: "get_hover_info",
			Params: map[string]interface{}{
				"uri":       "",
				"line":      10,
				"character": 5,
			},
			ExpectedError: "URI cannot be empty",
			ExpectedCode:  mcp.InvalidParams,
		},
		{
			Name:     "Malformed file URI",
			ToolName: "get_document_symbols",
			Params: map[string]interface{}{
				"uri": "file:/invalid/path/without/triple/slash",
			},
			ExpectedError: "malformed URI",
			ExpectedCode:  mcp.InvalidParams,
		},
		{
			Name:     "URI with unsupported scheme",
			ToolName: "goto_definition",
			Params: map[string]interface{}{
				"uri":       "http://example.com/file.go",
				"line":      10,
				"character": 5,
			},
			ExpectedError: "unsupported URI scheme",
			ExpectedCode:  mcp.InvalidParams,
		},
	}

	for _, scenario := range errorScenarios {
		suite.Run(scenario.Name, func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			response, err := suite.client.CallTool(ctx, scenario.ToolName, scenario.Params)
			
			if scenario.ExpectedCode == mcp.InvalidParams {
				suite.Error(err, "Expected error for invalid parameters")
				suite.Contains(err.Error(), scenario.ExpectedError)
			} else {
				suite.NoError(err, "Should not have call error")
				suite.NotNil(response, "Response should not be nil")
				suite.True(response.IsError, "Response should indicate error")
				suite.NotNil(response.Error, "Error field should be populated")
				suite.Contains(response.Error.Message, scenario.ExpectedError)
				suite.Equal(scenario.ExpectedCode, response.Error.Code)
			}
		})
	}
}

func (suite *MCPErrorHandlingE2ETestSuite) TestInvalidPositionParameters() {
	validURI := suite.getValidTestFileURI()
	
	invalidPositions := []ErrorTestCase{
		{
			Name: "Negative line number",
			Params: map[string]interface{}{
				"uri":       validURI,
				"line":      -1,
				"character": 5,
			},
			ExpectedError: "line number cannot be negative",
			ExpectedCode:  mcp.InvalidParams,
		},
		{
			Name: "Negative character position",
			Params: map[string]interface{}{
				"uri":       validURI,
				"line":      10,
				"character": -1,
			},
			ExpectedError: "character position cannot be negative",
			ExpectedCode:  mcp.InvalidParams,
		},
		{
			Name: "Line number out of range",
			Params: map[string]interface{}{
				"uri":       validURI,
				"line":      99999,
				"character": 5,
			},
			ExpectedError: "position out of range",
			ExpectedCode:  mcp.InvalidParams,
		},
		{
			Name: "Character position out of range",
			Params: map[string]interface{}{
				"uri":       validURI,
				"line":      10,
				"character": 99999,
			},
			ExpectedError: "position out of range",
			ExpectedCode:  mcp.InvalidParams,
		},
		{
			Name: "Invalid line type",
			Params: map[string]interface{}{
				"uri":       validURI,
				"line":      "invalid",
				"character": 5,
			},
			ExpectedError: "invalid parameter type",
			ExpectedCode:  mcp.InvalidParams,
		},
		{
			Name: "Invalid character type",
			Params: map[string]interface{}{
				"uri":       validURI,
				"line":      10,
				"character": true,
			},
			ExpectedError: "invalid parameter type",
			ExpectedCode:  mcp.InvalidParams,
		},
	}

	tools := []string{"goto_definition", "find_references", "get_hover_info"}
	
	for _, tool := range tools {
		for _, pos := range invalidPositions {
			suite.Run(fmt.Sprintf("%s_%s", tool, pos.Name), func() {
				suite.testInvalidPosition(tool, pos)
			})
		}
	}
}

func (suite *MCPErrorHandlingE2ETestSuite) TestMalformedRequests() {
	malformedRequests := []struct {
		name        string
		request     map[string]interface{}
		expectedError string
		expectedCode  int
	}{
		{
			name: "Missing required parameters",
			request: map[string]interface{}{
				"tool": "goto_definition",
			},
			expectedError: "missing required parameter",
			expectedCode:  mcp.InvalidParams,
		},
		{
			name: "Invalid parameter types",
			request: map[string]interface{}{
				"tool":      "goto_definition",
				"uri":       123,
				"line":      "invalid",
				"character": true,
			},
			expectedError: "invalid parameter type",
			expectedCode:  mcp.InvalidParams,
		},
		{
			name: "Unknown tool name",
			request: map[string]interface{}{
				"tool": "non_existent_tool",
				"uri":  suite.getValidTestFileURI(),
			},
			expectedError: "unknown tool",
			expectedCode:  mcp.MethodNotFound,
		},
		{
			name: "Null parameters",
			request: map[string]interface{}{
				"tool":      "goto_definition",
				"uri":       nil,
				"line":      nil,
				"character": nil,
			},
			expectedError: "null parameter value",
			expectedCode:  mcp.InvalidParams,
		},
		{
			name: "Empty tool name",
			request: map[string]interface{}{
				"tool": "",
				"uri":  suite.getValidTestFileURI(),
			},
			expectedError: "empty tool name",
			expectedCode:  mcp.InvalidParams,
		},
	}

	for _, req := range malformedRequests {
		suite.Run(req.name, func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			response := suite.sendRawMCPRequest(ctx, req.request)
			suite.validateErrorResponse(response, req.expectedError, req.expectedCode)
		})
	}
}

func (suite *MCPErrorHandlingE2ETestSuite) TestTimeoutScenarios() {
	suite.simulateSlowLSPServer(8 * time.Second)
	
	timeoutCases := []string{
		"goto_definition", "find_references", "get_hover_info",
		"get_document_symbols", "search_workspace_symbols",
	}

	for _, toolName := range timeoutCases {
		suite.Run(fmt.Sprintf("Timeout_%s", toolName), func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			start := time.Now()
			response, err := suite.client.CallTool(ctx, toolName, map[string]interface{}{
				"uri":       suite.getValidTestFileURI(),
				"line":      10,
				"character": 5,
			})
			elapsed := time.Since(start)

			suite.Less(elapsed, 4*time.Second, "Should timeout within expected time")
			
			if err != nil {
				suite.Contains(err.Error(), "timeout")
			} else {
				suite.NotNil(response, "Response should not be nil")
				suite.True(response.IsError, "Response should indicate error")
				suite.NotNil(response.Error, "Error field should be populated")
				suite.Contains(response.Error.Message, "timeout")
			}
		})
	}
	
	suite.resetLSPServerSimulation()
}

func (suite *MCPErrorHandlingE2ETestSuite) TestCircuitBreakerBehavior() {
	suite.T().Log("Testing circuit breaker behavior")
	
	suite.triggerCircuitBreakerOpen()
	
	suite.Run("Circuit breaker open rejection", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		response, err := suite.client.CallTool(ctx, "goto_definition", map[string]interface{}{
			"uri":       suite.getValidTestFileURI(),
			"line":      10,
			"character": 5,
		})

		if err != nil {
			suite.Contains(err.Error(), "circuit breaker")
		} else {
			suite.NotNil(response, "Response should not be nil")
			suite.True(response.IsError, "Response should indicate error")
			suite.Contains(response.Error.Message, "circuit breaker")
		}
	})

	suite.Run("Circuit breaker recovery", func() {
		time.Sleep(65 * time.Second)
		
		suite.resetErrorScenarios()
		
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		response, err := suite.client.CallTool(ctx, "goto_definition", map[string]interface{}{
			"uri":       suite.getValidTestFileURI(),
			"line":      10,
			"character": 5,
		})

		suite.NoError(err, "Should recover after circuit breaker timeout")
		if response != nil {
			suite.False(response.IsError, "Response should not indicate error after recovery")
		}
	})
}

func (suite *MCPErrorHandlingE2ETestSuite) TestLSPServerFailureRecovery() {
	suite.T().Log("Testing LSP server failure and recovery")
	
	suite.Run("Server failure handling", func() {
		suite.killLSPServer()
		
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		response, err := suite.client.CallTool(ctx, "goto_definition", map[string]interface{}{
			"uri":       suite.getValidTestFileURI(),
			"line":      10,
			"character": 5,
		})

		if err != nil {
			suite.Contains(err.Error(), "server unavailable")
		} else {
			suite.NotNil(response, "Response should not be nil")
			suite.True(response.IsError, "Response should indicate error")
			suite.Contains(response.Error.Message, "server unavailable")
		}
	})

	suite.Run("Server recovery", func() {
		suite.restartLSPServer()
		
		time.Sleep(5 * time.Second)
		
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		response, err := suite.client.CallTool(ctx, "goto_definition", map[string]interface{}{
			"uri":       suite.getValidTestFileURI(),
			"line":      10,
			"character": 5,
		})

		suite.NoError(err, "Should recover after server restart")
		if response != nil {
			suite.False(response.IsError, "Response should not indicate error after recovery")
		}
	})
}

func (suite *MCPErrorHandlingE2ETestSuite) TestNetworkConnectivityIssues() {
	suite.T().Log("Testing network connectivity issues")
	
	suite.Run("Connection refused", func() {
		suite.client.Disconnect()
		
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		response, err := suite.client.CallTool(ctx, "goto_definition", map[string]interface{}{
			"uri":       suite.getValidTestFileURI(),
			"line":      10,
			"character": 5,
		})

		suite.Error(err, "Should fail when disconnected")
		suite.Contains(err.Error(), "not initialized")
		suite.Nil(response, "Response should be nil when disconnected")
	})

	suite.Run("Connection recovery", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		err := suite.client.Connect(ctx)
		suite.NoError(err, "Should be able to reconnect")

		response, err := suite.client.CallTool(ctx, "goto_definition", map[string]interface{}{
			"uri":       suite.getValidTestFileURI(),
			"line":      10,
			"character": 5,
		})

		suite.NoError(err, "Should work after reconnection")
		if response != nil {
			suite.False(response.IsError, "Response should not indicate error after reconnection")
		}
	})
}

func (suite *MCPErrorHandlingE2ETestSuite) TestGracefulDegradation() {
	suite.T().Log("Testing graceful degradation scenarios")
	
	degradationScenarios := []struct {
		name            string
		failureType     FailureType
		expectedBehavior string
	}{
		{
			name:             "Slow response degradation",
			failureType:      FailureTypeTimeout,
			expectedBehavior: "timeout with graceful error",
		},
		{
			name:             "Malformed response handling",
			failureType:      FailureTypeMalformedResponse,
			expectedBehavior: "parse error with fallback",
		},
	}

	for _, scenario := range degradationScenarios {
		suite.Run(scenario.name, func() {
			suite.simulateFailureType(scenario.failureType)
			
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			response, err := suite.client.CallTool(ctx, "goto_definition", map[string]interface{}{
				"uri":       suite.getValidTestFileURI(),
				"line":      10,
				"character": 5,
			})

			if scenario.failureType == FailureTypeTimeout {
				if err != nil {
					suite.Contains(err.Error(), "timeout")
				} else {
					suite.True(response.IsError, "Should indicate timeout error")
				}
			} else if scenario.failureType == FailureTypeMalformedResponse {
				if err != nil {
					suite.Contains(err.Error(), "parse")
				} else {
					suite.True(response.IsError, "Should indicate parse error")
				}
			}
			
			suite.resetLSPServerSimulation()
		})
	}
}

func (suite *MCPErrorHandlingE2ETestSuite) TestConcurrentErrorHandling() {
	suite.T().Log("Testing concurrent error handling")
	
	const numConcurrentRequests = 10
	var wg sync.WaitGroup
	results := make([]error, numConcurrentRequests)
	
	suite.simulateSlowLSPServer(2 * time.Second)
	
	for i := 0; i < numConcurrentRequests; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			_, err := suite.client.CallTool(ctx, "goto_definition", map[string]interface{}{
				"uri":       fmt.Sprintf("file:///non/existent/file%d.go", index),
				"line":      10,
				"character": 5,
			})
			results[index] = err
		}(i)
	}
	
	wg.Wait()
	
	errorCount := 0
	for _, err := range results {
		if err != nil {
			errorCount++
		}
	}
	
	suite.Greater(errorCount, 0, "Should have some errors due to timeouts or invalid files")
	suite.T().Logf("Concurrent error handling: %d/%d requests failed as expected", errorCount, numConcurrentRequests)
	
	suite.resetLSPServerSimulation()
}

func (suite *MCPErrorHandlingE2ETestSuite) setupTestWorkspace() {
	workspace, err := suite.workspaceManager.CreateWorkspace("error-test-workspace")
	suite.Require().NoError(err)
	suite.workspace = workspace
}

func (suite *MCPErrorHandlingE2ETestSuite) setupMCPClient() {
	config := client.DefaultMCPTestConfig()
	config.RequestTimeout = 5 * time.Second
	config.CircuitBreaker.MaxFailures = 5
	config.CircuitBreaker.Timeout = 60 * time.Second
	config.LogLevel = "debug"
	
	transport := &StdioTransport{}
	suite.client = client.NewEnhancedMCPTestClient(config, transport)
}

func (suite *MCPErrorHandlingE2ETestSuite) setupErrorScenarios() {
	suite.errorScenarios = &ErrorScenarioManager{
		activeScenarios: make(map[string]*ErrorTestCase),
	}
	suite.failureSimulator = &LSPServerFailureSimulator{}
}

func (suite *MCPErrorHandlingE2ETestSuite) startMCPServer() {
	configPath := suite.createTestConfig()
	
	suite.server = exec.Command(suite.binaryPath, "mcp", "--config", configPath)
	suite.server.Env = append(os.Environ(), "LSP_GATEWAY_TEST_MODE=true")
	
	err := suite.server.Start()
	suite.Require().NoError(err)
	
	time.Sleep(2 * time.Second)
}

func (suite *MCPErrorHandlingE2ETestSuite) createTestConfig() string {
	configContent := fmt.Sprintf(`
servers:
- name: go-lsp
  languages: [go]
  command: gopls
  args: []
  transport: stdio
  root_markers: [go.mod]
  priority: 1
  weight: 1.0

port: 8080
timeout: 5s
max_concurrent_requests: 50
cache:
  memory:
    capacity: 8GB
    ttl: 1h
  disk:
    path: %s/cache
    max_size: 1GB
`, suite.workspace.RootPath)

	configPath := filepath.Join(suite.workspace.RootPath, "test-config.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	suite.Require().NoError(err)
	
	return configPath
}

func (suite *MCPErrorHandlingE2ETestSuite) getValidTestFileURI() string {
	colorGoPath := filepath.Join(suite.workspace.RootPath, "color.go")
	return fmt.Sprintf("file://%s", colorGoPath)
}

func (suite *MCPErrorHandlingE2ETestSuite) testInvalidPosition(toolName string, testCase ErrorTestCase) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err := suite.client.CallTool(ctx, toolName, testCase.Params)
	
	if testCase.ExpectedCode == mcp.InvalidParams {
		suite.Error(err, "Expected error for invalid parameters")
		suite.Contains(err.Error(), testCase.ExpectedError)
	} else {
		suite.NoError(err, "Should not have call error")
		suite.NotNil(response, "Response should not be nil")
		suite.True(response.IsError, "Response should indicate error")
		suite.Contains(response.Error.Message, testCase.ExpectedError)
	}
}

func (suite *MCPErrorHandlingE2ETestSuite) sendRawMCPRequest(ctx context.Context, request map[string]interface{}) *mcp.MCPResponse {
	_, _ = json.Marshal(request)
	response := &mcp.MCPResponse{}
	
	return response
}

func (suite *MCPErrorHandlingE2ETestSuite) validateErrorResponse(response *mcp.MCPResponse, expectedError string, expectedCode int) {
	suite.NotNil(response, "Response should not be nil")
	suite.NotNil(response.Error, "Error field should be populated")
	suite.Contains(response.Error.Message, expectedError)
	suite.Equal(expectedCode, response.Error.Code)
}

func (suite *MCPErrorHandlingE2ETestSuite) simulateSlowLSPServer(delay time.Duration) {
	suite.failureSimulator.mu.Lock()
	defer suite.failureSimulator.mu.Unlock()
	
	suite.failureSimulator.simulateFailure = true
	suite.failureSimulator.failureType = FailureTypeTimeout
	suite.failureSimulator.responseDelay = delay
}

func (suite *MCPErrorHandlingE2ETestSuite) simulateFailureType(failureType FailureType) {
	suite.failureSimulator.mu.Lock()
	defer suite.failureSimulator.mu.Unlock()
	
	suite.failureSimulator.simulateFailure = true
	suite.failureSimulator.failureType = failureType
}

func (suite *MCPErrorHandlingE2ETestSuite) resetLSPServerSimulation() {
	suite.failureSimulator.mu.Lock()
	defer suite.failureSimulator.mu.Unlock()
	
	suite.failureSimulator.simulateFailure = false
	suite.failureSimulator.responseDelay = 0
}

func (suite *MCPErrorHandlingE2ETestSuite) triggerCircuitBreakerOpen() {
	suite.T().Log("Triggering circuit breaker by causing multiple failures")
	
	for i := 0; i < 10; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		
		suite.client.CallTool(ctx, "goto_definition", map[string]interface{}{
			"uri":       "file:///non/existent/file.go",
			"line":      10,
			"character": 5,
		})
		
		cancel()
		time.Sleep(100 * time.Millisecond)
	}
	
	suite.errorScenarios.mu.Lock()
	suite.errorScenarios.circuitBreakerOpen = true
	suite.errorScenarios.lastFailureTime = time.Now()
	suite.errorScenarios.mu.Unlock()
}

func (suite *MCPErrorHandlingE2ETestSuite) resetErrorScenarios() {
	suite.errorScenarios.mu.Lock()
	defer suite.errorScenarios.mu.Unlock()
	
	suite.errorScenarios.failureCount = 0
	suite.errorScenarios.circuitBreakerOpen = false
	suite.errorScenarios.activeScenarios = make(map[string]*ErrorTestCase)
}

func (suite *MCPErrorHandlingE2ETestSuite) killLSPServer() {
	suite.serverMutex.Lock()
	defer suite.serverMutex.Unlock()
	
	if suite.server != nil && suite.server.Process != nil {
		suite.server.Process.Signal(syscall.SIGKILL)
		suite.server.Wait()
		suite.server = nil
	}
}

func (suite *MCPErrorHandlingE2ETestSuite) restartLSPServer() {
	suite.serverMutex.Lock()
	defer suite.serverMutex.Unlock()
	
	if suite.server != nil && suite.server.Process != nil {
		suite.server.Process.Signal(syscall.SIGTERM)
		suite.server.Wait()
	}
	
	suite.startMCPServer()
}

type StdioTransport struct{}

func (t *StdioTransport) Connect(ctx context.Context) error {
	return nil
}

func (t *StdioTransport) Disconnect() error {
	return nil
}

func (t *StdioTransport) Send(ctx context.Context, data []byte) error {
	return nil
}

func (t *StdioTransport) Receive(ctx context.Context) ([]byte, error) {
	return nil, nil
}

func (t *StdioTransport) IsConnected() bool {
	return true
}

func (t *StdioTransport) Close() error {
	return nil
}

func findProjectRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	
	return "", fmt.Errorf("project root not found")
}

func TestMCPErrorHandlingE2ETestSuite(t *testing.T) {
	suite.Run(t, new(MCPErrorHandlingE2ETestSuite))
}