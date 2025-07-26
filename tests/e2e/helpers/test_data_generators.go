package e2e_test

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"lsp-gateway/mcp"
	"lsp-gateway/tests/mocks"
)

// TestDataGenerator provides utilities for generating test data for E2E tests
type TestDataGenerator struct {
	rand *rand.Rand
}

// NewTestDataGenerator creates a new test data generator
func NewTestDataGenerator() *TestDataGenerator {
	return &TestDataGenerator{
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// GenerateRandomLSPRequest creates a random LSP request for testing
func (g *TestDataGenerator) GenerateRandomLSPRequest() (string, interface{}) {
	methods := []string{
		mcp.LSP_METHOD_WORKSPACE_SYMBOL,
		mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION,
		mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES,
		mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER,
		mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS,
	}

	method := methods[g.rand.Intn(len(methods))]

	switch method {
	case mcp.LSP_METHOD_WORKSPACE_SYMBOL:
		return method, map[string]interface{}{
			"query": g.generateRandomSymbolQuery(),
		}
	case mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION:
		return method, g.generateTextDocumentPositionParams()
	case mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES:
		params := g.generateTextDocumentPositionParams()
		params["context"] = map[string]interface{}{
			"includeDeclaration": g.rand.Intn(2) == 1,
		}
		return method, params
	case mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER:
		return method, g.generateTextDocumentPositionParams()
	case mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS:
		return method, map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": g.generateRandomFileURI(),
			},
		}
	default:
		return method, map[string]interface{}{}
	}
}

// ConfigureMockClientForScenario configures a mock client for specific test scenarios
func (g *TestDataGenerator) ConfigureMockClientForScenario(mockClient *mocks.MockMcpClient, scenario string) error {
	switch scenario {
	case "basic-workflow":
		return g.configureBasicWorkflow(mockClient)
	case "error-handling":
		return g.configureErrorHandling(mockClient)
	case "performance-testing":
		return g.configurePerformanceTesting(mockClient)
	case "circuit-breaker":
		return g.configureCircuitBreakerTesting(mockClient)
	default:
		return fmt.Errorf("unknown scenario: %s", scenario)
	}
}

// Private helper methods

func (g *TestDataGenerator) generateRandomSymbolQuery() string {
	queries := []string{"User", "Handler", "Service", "Model", "Controller", "Component", "Function", "Class"}
	return queries[g.rand.Intn(len(queries))]
}

func (g *TestDataGenerator) generateTextDocumentPositionParams() map[string]interface{} {
	return map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": g.generateRandomFileURI(),
		},
		"position": map[string]interface{}{
			"line":      g.rand.Intn(100),
			"character": g.rand.Intn(80),
		},
	}
}

func (g *TestDataGenerator) generateRandomFileURI() string {
	languages := []string{"go", "py", "ts", "js", "java"}
	extensions := map[string]string{
		"go":   ".go",
		"py":   ".py",
		"ts":   ".ts",
		"js":   ".js",
		"java": ".java",
	}

	lang := languages[g.rand.Intn(len(languages))]
	filename := fmt.Sprintf("file%d%s", g.rand.Intn(10), extensions[lang])
	return fmt.Sprintf("file:///workspace/%s", filename)
}

func (g *TestDataGenerator) configureBasicWorkflow(mockClient *mocks.MockMcpClient) error {
	mockClient.SetHealthy(true)
	mockClient.UpdateMetrics(0, 0, 0, 0, 0, 100*time.Millisecond)
	return nil
}

func (g *TestDataGenerator) configureErrorHandling(mockClient *mocks.MockMcpClient) error {
	mockClient.QueueError(fmt.Errorf("network error"))
	mockClient.QueueError(fmt.Errorf("timeout"))
	mockClient.QueueError(fmt.Errorf("server error"))
	return nil
}

func (g *TestDataGenerator) configurePerformanceTesting(mockClient *mocks.MockMcpClient) error {
	mockClient.SetHealthy(true)
	mockClient.UpdateMetrics(0, 0, 0, 0, 0, 50*time.Millisecond)
	return nil
}

func (g *TestDataGenerator) configureCircuitBreakerTesting(mockClient *mocks.MockMcpClient) error {
	mockClient.SetCircuitBreakerConfig(3, 10*time.Second)
	for i := 0; i < 5; i++ {
		mockClient.QueueError(fmt.Errorf("circuit breaker test error %d", i))
	}
	return nil
}

// CreateContextWithTimeout creates a context with timeout for testing
func CreateContextWithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}
