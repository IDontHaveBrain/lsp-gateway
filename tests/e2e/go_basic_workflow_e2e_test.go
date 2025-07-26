package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"lsp-gateway/tests/mocks"
)

// GoBasicWorkflowE2ETestSuite provides comprehensive E2E tests for Go LSP workflows
// covering all essential Go development scenarios that developers use daily
type GoBasicWorkflowE2ETestSuite struct {
	suite.Suite
	mockClient    *mocks.MockMcpClient
	testTimeout   time.Duration
	workspaceRoot string
	goFiles       map[string]GoTestFile
}

// GoTestFile represents a realistic Go file with its metadata
type GoTestFile struct {
	FileName    string
	Content     string
	Language    string
	Description string
	Symbols     []GoSymbol
}

// GoSymbol represents a Go symbol with position information
type GoSymbol struct {
	Name        string
	Kind        int // LSP SymbolKind
	Position    LSPPosition
	Type        string // Go type information
	Description string
}

// GoWorkflowResult captures results from Go workflow execution
type GoWorkflowResult struct {
	DefinitionFound       bool
	ReferencesCount       int
	HoverInfoRetrieved    bool
	DocumentSymbolsCount  int
	CompletionItemsCount  int
	WorkspaceSymbolsCount int
	ResponseTimeMs        int64
	ErrorEncountered      error
}

// Note: LSPPosition is defined in typescript_basic_workflow_e2e_test.go

// SetupSuite initializes the test suite with Go-specific configurations
func (suite *GoBasicWorkflowE2ETestSuite) SetupSuite() {
	suite.testTimeout = 30 * time.Second
	suite.workspaceRoot = "/tmp/go-e2e-test-workspace"

	// Setup realistic Go test files that represent common development scenarios
	suite.setupGoTestFiles()
}

// SetupTest initializes a fresh mock client for each test
func (suite *GoBasicWorkflowE2ETestSuite) SetupTest() {
	suite.mockClient = mocks.NewMockMcpClient()
	suite.mockClient.SetHealthy(true)
}

// TearDownTest cleans up mock client state
func (suite *GoBasicWorkflowE2ETestSuite) TearDownTest() {
	if suite.mockClient != nil {
		suite.mockClient.Reset()
	}
}

// setupGoTestFiles creates realistic Go files for testing
func (suite *GoBasicWorkflowE2ETestSuite) setupGoTestFiles() {
	suite.goFiles = map[string]GoTestFile{
		"main.go": {
			FileName:    "main.go",
			Language:    "go",
			Description: "Main application entry point with HTTP server setup",
			Content: `package main

import (
	"fmt"
	"log"
	"net/http"
	"github.com/gin-gonic/gin"
)

// Server represents the HTTP server configuration
type Server struct {
	port   int
	router *gin.Engine
}

// NewServer creates a new server instance
func NewServer(port int) *Server {
	return &Server{
		port:   port,
		router: gin.Default(),
	}
}

// HandleRequest processes incoming HTTP requests
func (s *Server) HandleRequest(c *gin.Context) {
	response := map[string]interface{}{
		"message": "Hello from Go LSP Gateway",
		"status":  "success",
	}
	c.JSON(http.StatusOK, response)
}

// Start starts the HTTP server
func (s *Server) Start() error {
	s.router.GET("/health", s.HandleRequest)
	return s.router.Run(fmt.Sprintf(":%d", s.port))
}

func main() {
	server := NewServer(8080)
	log.Printf("Starting server on port %d", 8080)
	if err := server.Start(); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}`,
			Symbols: []GoSymbol{
				{Name: "Server", Kind: 5, Position: LSPPosition{Line: 10, Character: 5}, Type: "struct", Description: "HTTP server configuration"},
				{Name: "NewServer", Kind: 12, Position: LSPPosition{Line: 16, Character: 5}, Type: "func", Description: "Server constructor"},
				{Name: "HandleRequest", Kind: 6, Position: LSPPosition{Line: 23, Character: 19}, Type: "method", Description: "HTTP request handler"},
				{Name: "Start", Kind: 6, Position: LSPPosition{Line: 31, Character: 19}, Type: "method", Description: "Server startup method"},
				{Name: "main", Kind: 12, Position: LSPPosition{Line: 36, Character: 5}, Type: "func", Description: "Application entry point"},
			},
		},
		"handler.go": {
			FileName:    "handler.go",
			Language:    "go",
			Description: "HTTP handlers and middleware for the application",
			Content: `package main

import (
	"net/http"
	"time"
	"github.com/gin-gonic/gin"
)

// RequestMetrics holds request processing metrics
type RequestMetrics struct {
	StartTime    time.Time
	Duration     time.Duration
	StatusCode   int
	Method       string
	Path         string
}

// LoggingMiddleware provides request logging functionality
func LoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		
		metrics := &RequestMetrics{
			StartTime:  start,
			Duration:   time.Since(start),
			StatusCode: c.Writer.Status(),
			Method:     c.Request.Method,
			Path:       c.Request.URL.Path,
		}
		
		logRequestMetrics(metrics)
	}
}

// logRequestMetrics logs the request metrics
func logRequestMetrics(metrics *RequestMetrics) {
	// Log implementation would go here
}

// HealthHandler provides health check endpoint
func HealthHandler(c *gin.Context) {
	status := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"version":   "1.0.0",
	}
	c.JSON(http.StatusOK, status)
}`,
			Symbols: []GoSymbol{
				{Name: "RequestMetrics", Kind: 5, Position: LSPPosition{Line: 9, Character: 5}, Type: "struct", Description: "Request processing metrics"},
				{Name: "LoggingMiddleware", Kind: 12, Position: LSPPosition{Line: 18, Character: 5}, Type: "func", Description: "Request logging middleware"},
				{Name: "logRequestMetrics", Kind: 12, Position: LSPPosition{Line: 36, Character: 5}, Type: "func", Description: "Metrics logging function"},
				{Name: "HealthHandler", Kind: 12, Position: LSPPosition{Line: 41, Character: 5}, Type: "func", Description: "Health check handler"},
			},
		},
		"config.go": {
			FileName:    "config.go",
			Language:    "go",
			Description: "Configuration management for the application",
			Content: `package main

import (
	"os"
	"strconv"
	"time"
)

// Config holds application configuration
type Config struct {
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	MaxHeaderBytes  int
	DatabaseURL     string
	LogLevel        string
	EnableMetrics   bool
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	port, _ := strconv.Atoi(getEnvOrDefault("PORT", "8080"))
	readTimeout, _ := time.ParseDuration(getEnvOrDefault("READ_TIMEOUT", "30s"))
	writeTimeout, _ := time.ParseDuration(getEnvOrDefault("WRITE_TIMEOUT", "30s"))
	maxHeaderBytes, _ := strconv.Atoi(getEnvOrDefault("MAX_HEADER_BYTES", "1048576"))
	
	return &Config{
		Port:            port,
		ReadTimeout:     readTimeout,
		WriteTimeout:    writeTimeout,
		MaxHeaderBytes:  maxHeaderBytes,
		DatabaseURL:     getEnvOrDefault("DATABASE_URL", ""),
		LogLevel:        getEnvOrDefault("LOG_LEVEL", "info"),
		EnableMetrics:   getEnvOrDefault("ENABLE_METRICS", "true") == "true",
	}
}

// getEnvOrDefault gets environment variable or returns default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Configuration validation logic would go here
	return nil
}`,
			Symbols: []GoSymbol{
				{Name: "Config", Kind: 5, Position: LSPPosition{Line: 9, Character: 5}, Type: "struct", Description: "Application configuration"},
				{Name: "LoadConfig", Kind: 12, Position: LSPPosition{Line: 20, Character: 5}, Type: "func", Description: "Configuration loader"},
				{Name: "getEnvOrDefault", Kind: 12, Position: LSPPosition{Line: 37, Character: 5}, Type: "func", Description: "Environment variable helper"},
				{Name: "Validate", Kind: 6, Position: LSPPosition{Line: 44, Character: 17}, Type: "method", Description: "Configuration validator"},
			},
		},
	}
}

// TestGoDefinitionWorkflow tests Go definition resolution workflow
func (suite *GoBasicWorkflowE2ETestSuite) TestGoDefinitionWorkflow() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	// Test definition lookup for various Go symbols
	testCases := []struct {
		name     string
		file     string
		position LSPPosition
		expected string
	}{
		{
			name:     "Function definition lookup",
			file:     "main.go",
			position: LSPPosition{Line: 36, Character: 15}, // NewServer call
			expected: "NewServer",
		},
		{
			name:     "Method definition lookup",
			file:     "main.go",
			position: LSPPosition{Line: 23, Character: 25}, // HandleRequest method
			expected: "HandleRequest",
		},
		{
			name:     "Type definition lookup",
			file:     "handler.go",
			position: LSPPosition{Line: 24, Character: 15}, // RequestMetrics type
			expected: "RequestMetrics",
		},
		{
			name:     "Package import definition",
			file:     "main.go",
			position: LSPPosition{Line: 6, Character: 25}, // gin import
			expected: "gin",
		},
	}

	var wg sync.WaitGroup
	results := make([]GoWorkflowResult, len(testCases))

	for i, tc := range testCases {
		wg.Add(1)
		go func(index int, testCase struct {
			name     string
			file     string
			position LSPPosition
			expected string
		}) {
			defer wg.Done()

			start := time.Now()

			// Execute definition request
			definitionResult := suite.executeGoDefinitionRequest(ctx, testCase.file, testCase.position)

			results[index] = GoWorkflowResult{
				DefinitionFound:  definitionResult,
				ResponseTimeMs:   time.Since(start).Milliseconds(),
				ErrorEncountered: nil,
			}
		}(i, tc)
	}

	wg.Wait()

	// Validate results
	for i, result := range results {
		suite.True(result.DefinitionFound,
			fmt.Sprintf("Definition lookup failed for test case: %s", testCases[i].name))
		suite.Less(result.ResponseTimeMs, int64(5000),
			fmt.Sprintf("Definition lookup took too long for: %s", testCases[i].name))
	}
}

// TestGoReferencesWorkflow tests Go references finding workflow
func (suite *GoBasicWorkflowE2ETestSuite) TestGoReferencesWorkflow() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	// Test references finding for Go symbols
	testCases := []struct {
		name            string
		file            string
		position        LSPPosition
		expectedMinRefs int
	}{
		{
			name:            "Function references",
			file:            "main.go",
			position:        LSPPosition{Line: 16, Character: 5}, // NewServer function
			expectedMinRefs: 1,                                   // Called in main
		},
		{
			name:            "Method references",
			file:            "main.go",
			position:        LSPPosition{Line: 23, Character: 19}, // HandleRequest method
			expectedMinRefs: 1,                                    // Used in Start method
		},
		{
			name:            "Type references",
			file:            "handler.go",
			position:        LSPPosition{Line: 9, Character: 5}, // RequestMetrics type
			expectedMinRefs: 2,                                  // Used in middleware and logging
		},
	}

	for _, tc := range testCases {
		start := time.Now()

		referencesCount := suite.executeGoReferencesRequest(ctx, tc.file, tc.position)
		responseTime := time.Since(start).Milliseconds()

		suite.GreaterOrEqual(referencesCount, tc.expectedMinRefs,
			fmt.Sprintf("Expected at least %d references for %s, got %d",
				tc.expectedMinRefs, tc.name, referencesCount))
		suite.Less(responseTime, int64(5000),
			fmt.Sprintf("References lookup took too long for: %s", tc.name))
	}
}

// TestGoHoverWorkflow tests Go hover information workflow
func (suite *GoBasicWorkflowE2ETestSuite) TestGoHoverWorkflow() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	// Test hover information for Go symbols
	testCases := []struct {
		name     string
		file     string
		position LSPPosition
		expected string
	}{
		{
			name:     "Function hover",
			file:     "main.go",
			position: LSPPosition{Line: 16, Character: 5}, // NewServer function
			expected: "NewServer",
		},
		{
			name:     "Method hover",
			file:     "handler.go",
			position: LSPPosition{Line: 18, Character: 5}, // LoggingMiddleware function
			expected: "LoggingMiddleware",
		},
		{
			name:     "Type hover",
			file:     "config.go",
			position: LSPPosition{Line: 9, Character: 5}, // Config type
			expected: "Config",
		},
	}

	for _, tc := range testCases {
		start := time.Now()

		hoverResult := suite.executeGoHoverRequest(ctx, tc.file, tc.position)
		responseTime := time.Since(start).Milliseconds()

		suite.True(hoverResult,
			fmt.Sprintf("Hover information not found for: %s", tc.name))
		suite.Less(responseTime, int64(5000),
			fmt.Sprintf("Hover lookup took too long for: %s", tc.name))
	}
}

// TestGoDocumentSymbolsWorkflow tests Go document symbols workflow
func (suite *GoBasicWorkflowE2ETestSuite) TestGoDocumentSymbolsWorkflow() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	// Test document symbols for each Go file
	for fileName, expectedFile := range suite.goFiles {
		start := time.Now()

		symbolsCount := suite.executeGoDocumentSymbolsRequest(ctx, fileName)
		responseTime := time.Since(start).Milliseconds()

		expectedSymbolsCount := len(expectedFile.Symbols)
		suite.GreaterOrEqual(symbolsCount, expectedSymbolsCount,
			fmt.Sprintf("Expected at least %d symbols in %s, got %d",
				expectedSymbolsCount, fileName, symbolsCount))
		suite.Less(responseTime, int64(5000),
			fmt.Sprintf("Document symbols lookup took too long for: %s", fileName))
	}
}

// TestGoWorkspaceSymbolsWorkflow tests Go workspace-wide symbol search
func (suite *GoBasicWorkflowE2ETestSuite) TestGoWorkspaceSymbolsWorkflow() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	// Test workspace symbol search queries
	testCases := []struct {
		name        string
		query       string
		expectedMin int
	}{
		{
			name:        "Function search",
			query:       "NewServer",
			expectedMin: 1,
		},
		{
			name:        "Type search",
			query:       "Config",
			expectedMin: 1,
		},
		{
			name:        "Method search",
			query:       "HandleRequest",
			expectedMin: 1,
		},
		{
			name:        "Partial name search",
			query:       "Handler",
			expectedMin: 2, // HealthHandler, HandleRequest
		},
	}

	for _, tc := range testCases {
		start := time.Now()

		symbolsCount := suite.executeGoWorkspaceSymbolsRequest(ctx, tc.query)
		responseTime := time.Since(start).Milliseconds()

		suite.GreaterOrEqual(symbolsCount, tc.expectedMin,
			fmt.Sprintf("Expected at least %d symbols for query '%s', got %d",
				tc.expectedMin, tc.query, symbolsCount))
		suite.Less(responseTime, int64(5000),
			fmt.Sprintf("Workspace symbols search took too long for: %s", tc.name))
	}
}

// TestGoCompletionWorkflow tests Go code completion workflow
func (suite *GoBasicWorkflowE2ETestSuite) TestGoCompletionWorkflow() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	// Test code completion at various positions
	testCases := []struct {
		name             string
		file             string
		position         LSPPosition
		expectedMinItems int
	}{
		{
			name:             "Method completion",
			file:             "main.go",
			position:         LSPPosition{Line: 32, Character: 15}, // After s.router.
			expectedMinItems: 5,                                    // GET, POST, PUT, DELETE, etc.
		},
		{
			name:             "Package completion",
			file:             "handler.go",
			position:         LSPPosition{Line: 20, Character: 10}, // After time.
			expectedMinItems: 10,                                   // Now, Since, Parse, etc.
		},
		{
			name:             "Variable completion",
			file:             "config.go",
			position:         LSPPosition{Line: 26, Character: 15}, // After config variable
			expectedMinItems: 5,                                    // Config struct fields
		},
	}

	for _, tc := range testCases {
		start := time.Now()

		completionCount := suite.executeGoCompletionRequest(ctx, tc.file, tc.position)
		responseTime := time.Since(start).Milliseconds()

		suite.GreaterOrEqual(completionCount, tc.expectedMinItems,
			fmt.Sprintf("Expected at least %d completion items for %s, got %d",
				tc.expectedMinItems, tc.name, completionCount))
		suite.Less(responseTime, int64(5000),
			fmt.Sprintf("Completion took too long for: %s", tc.name))
	}
}

// TestGoE2EFullWorkflow tests complete Go development workflow
func (suite *GoBasicWorkflowE2ETestSuite) TestGoE2EFullWorkflow() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Execute complete Go development workflow
	workflowSteps := []struct {
		name string
		exec func() GoWorkflowResult
	}{
		{
			name: "Definition lookup",
			exec: func() GoWorkflowResult {
				start := time.Now()
				result := suite.executeGoDefinitionRequest(ctx, "main.go", LSPPosition{Line: 36, Character: 15})
				return GoWorkflowResult{
					DefinitionFound: result,
					ResponseTimeMs:  time.Since(start).Milliseconds(),
				}
			},
		},
		{
			name: "References search",
			exec: func() GoWorkflowResult {
				start := time.Now()
				count := suite.executeGoReferencesRequest(ctx, "main.go", LSPPosition{Line: 16, Character: 5})
				return GoWorkflowResult{
					ReferencesCount: count,
					ResponseTimeMs:  time.Since(start).Milliseconds(),
				}
			},
		},
		{
			name: "Hover information",
			exec: func() GoWorkflowResult {
				start := time.Now()
				result := suite.executeGoHoverRequest(ctx, "handler.go", LSPPosition{Line: 18, Character: 5})
				return GoWorkflowResult{
					HoverInfoRetrieved: result,
					ResponseTimeMs:     time.Since(start).Milliseconds(),
				}
			},
		},
		{
			name: "Document symbols",
			exec: func() GoWorkflowResult {
				start := time.Now()
				count := suite.executeGoDocumentSymbolsRequest(ctx, "main.go")
				return GoWorkflowResult{
					DocumentSymbolsCount: count,
					ResponseTimeMs:       time.Since(start).Milliseconds(),
				}
			},
		},
		{
			name: "Workspace symbols",
			exec: func() GoWorkflowResult {
				start := time.Now()
				count := suite.executeGoWorkspaceSymbolsRequest(ctx, "NewServer")
				return GoWorkflowResult{
					WorkspaceSymbolsCount: count,
					ResponseTimeMs:        time.Since(start).Milliseconds(),
				}
			},
		},
		{
			name: "Code completion",
			exec: func() GoWorkflowResult {
				start := time.Now()
				count := suite.executeGoCompletionRequest(ctx, "main.go", LSPPosition{Line: 32, Character: 15})
				return GoWorkflowResult{
					CompletionItemsCount: count,
					ResponseTimeMs:       time.Since(start).Milliseconds(),
				}
			},
		},
	}

	totalStartTime := time.Now()
	var wg sync.WaitGroup
	results := make([]GoWorkflowResult, len(workflowSteps))

	// Execute all workflow steps concurrently
	for i, step := range workflowSteps {
		wg.Add(1)
		go func(index int, workflowStep struct {
			name string
			exec func() GoWorkflowResult
		}) {
			defer wg.Done()
			results[index] = workflowStep.exec()
		}(i, step)
	}

	wg.Wait()
	totalDuration := time.Since(totalStartTime)

	// Validate workflow results
	suite.True(results[0].DefinitionFound, "Definition lookup should succeed")
	suite.Greater(results[1].ReferencesCount, 0, "References should be found")
	suite.True(results[2].HoverInfoRetrieved, "Hover information should be retrieved")
	suite.Greater(results[3].DocumentSymbolsCount, 0, "Document symbols should be found")
	suite.Greater(results[4].WorkspaceSymbolsCount, 0, "Workspace symbols should be found")
	suite.Greater(results[5].CompletionItemsCount, 0, "Completion items should be provided")

	// Validate performance requirements
	suite.Less(totalDuration.Seconds(), 30.0, "Full workflow should complete within 30 seconds")

	for i, result := range results {
		suite.Less(result.ResponseTimeMs, int64(5000),
			fmt.Sprintf("Step '%s' took too long: %dms", workflowSteps[i].name, result.ResponseTimeMs))
	}
}

// Helper methods for executing LSP requests

func (suite *GoBasicWorkflowE2ETestSuite) executeGoDefinitionRequest(ctx context.Context, file string, position LSPPosition) bool {
	// Mock implementation - in real test this would call actual LSP gateway
	params := map[string]interface{}{
		"textDocument": map[string]string{
			"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, file),
		},
		"position": position,
	}

	// Simulate LSP definition request
	response, err := suite.mockClient.SendLSPRequest(ctx, "textDocument/definition", params)
	if err != nil {
		return false
	}

	// Parse response to verify definition was found
	var result interface{}
	if err := json.Unmarshal(response, &result); err != nil {
		return false
	}

	return result != nil
}

func (suite *GoBasicWorkflowE2ETestSuite) executeGoReferencesRequest(ctx context.Context, file string, position LSPPosition) int {
	// Mock implementation - in real test this would call actual LSP gateway
	params := map[string]interface{}{
		"textDocument": map[string]string{
			"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, file),
		},
		"position": position,
		"context": map[string]bool{
			"includeDeclaration": true,
		},
	}

	response, err := suite.mockClient.SendLSPRequest(ctx, "textDocument/references", params)
	if err != nil {
		return 0
	}

	var references []interface{}
	if err := json.Unmarshal(response, &references); err != nil {
		return 0
	}

	return len(references)
}

func (suite *GoBasicWorkflowE2ETestSuite) executeGoHoverRequest(ctx context.Context, file string, position LSPPosition) bool {
	// Mock implementation - in real test this would call actual LSP gateway
	params := map[string]interface{}{
		"textDocument": map[string]string{
			"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, file),
		},
		"position": position,
	}

	response, err := suite.mockClient.SendLSPRequest(ctx, "textDocument/hover", params)
	if err != nil {
		return false
	}

	var hover map[string]interface{}
	if err := json.Unmarshal(response, &hover); err != nil {
		return false
	}

	contents, exists := hover["contents"]
	return exists && contents != nil
}

func (suite *GoBasicWorkflowE2ETestSuite) executeGoDocumentSymbolsRequest(ctx context.Context, file string) int {
	// Mock implementation - in real test this would call actual LSP gateway
	params := map[string]interface{}{
		"textDocument": map[string]string{
			"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, file),
		},
	}

	response, err := suite.mockClient.SendLSPRequest(ctx, "textDocument/documentSymbol", params)
	if err != nil {
		return 0
	}

	var symbols []interface{}
	if err := json.Unmarshal(response, &symbols); err != nil {
		return 0
	}

	return len(symbols)
}

func (suite *GoBasicWorkflowE2ETestSuite) executeGoWorkspaceSymbolsRequest(ctx context.Context, query string) int {
	// Mock implementation - in real test this would call actual LSP gateway
	params := map[string]interface{}{
		"query": query,
	}

	response, err := suite.mockClient.SendLSPRequest(ctx, "workspace/symbol", params)
	if err != nil {
		return 0
	}

	var symbols []interface{}
	if err := json.Unmarshal(response, &symbols); err != nil {
		return 0
	}

	return len(symbols)
}

func (suite *GoBasicWorkflowE2ETestSuite) executeGoCompletionRequest(ctx context.Context, file string, position LSPPosition) int {
	// Mock implementation - in real test this would call actual LSP gateway
	params := map[string]interface{}{
		"textDocument": map[string]string{
			"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, file),
		},
		"position": position,
	}

	response, err := suite.mockClient.SendLSPRequest(ctx, "textDocument/completion", params)
	if err != nil {
		return 0
	}

	var completion map[string]interface{}
	if err := json.Unmarshal(response, &completion); err != nil {
		return 0
	}

	if items, exists := completion["items"].([]interface{}); exists {
		return len(items)
	}

	return 0
}

// TearDownSuite cleans up after all tests
func (suite *GoBasicWorkflowE2ETestSuite) TearDownSuite() {
	// Cleanup mock client and test workspace
	if suite.mockClient != nil {
		// Cleanup mock client resources
	}
}

// TestGoBasicWorkflowE2ETestSuite runs the Go E2E test suite
func TestGoBasicWorkflowE2ETestSuite(t *testing.T) {
	suite.Run(t, new(GoBasicWorkflowE2ETestSuite))
}
