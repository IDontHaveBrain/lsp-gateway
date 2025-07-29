package workspace_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"lsp-gateway/internal/config"
	"lsp-gateway/internal/workspace"
)

// GatewayRoutingTestSuite tests WorkspaceGateway sub-project routing functionality
type GatewayRoutingTestSuite struct {
	suite.Suite
	tempDir        string
	workspaceRoot  string
	gateway        workspace.WorkspaceGateway
	workspaceConfig *workspace.WorkspaceConfig
	gatewayConfig  *workspace.WorkspaceGatewayConfig
	ctx            context.Context
	cancel         context.CancelFunc
}

func TestGatewayRoutingTestSuite(t *testing.T) {
	suite.Run(t, new(GatewayRoutingTestSuite))
}

func (suite *GatewayRoutingTestSuite) SetupSuite() {
	suite.ctx, suite.cancel = context.WithTimeout(context.Background(), 30*time.Second)
	
	var err error
	suite.tempDir, err = os.MkdirTemp("", "gateway-routing-test-*")
	suite.Require().NoError(err, "Failed to create temp directory")
	
	suite.workspaceRoot = filepath.Join(suite.tempDir, "workspace")
	err = os.MkdirAll(suite.workspaceRoot, 0755)
	suite.Require().NoError(err)
	
	suite.setupTestWorkspace()
	suite.setupGatewayConfig()
}

func (suite *GatewayRoutingTestSuite) TearDownSuite() {
	if suite.gateway != nil {
		suite.gateway.Stop()
	}
	suite.cancel()
	os.RemoveAll(suite.tempDir)
}

func (suite *GatewayRoutingTestSuite) SetupTest() {
	suite.gateway = workspace.NewWorkspaceGateway()
	err := suite.gateway.Initialize(suite.ctx, suite.workspaceConfig, suite.gatewayConfig)
	suite.Require().NoError(err, "Failed to initialize gateway")
}

func (suite *GatewayRoutingTestSuite) TearDownTest() {
	if suite.gateway != nil {
		suite.gateway.Stop()
	}
}

func (suite *GatewayRoutingTestSuite) setupTestWorkspace() {
	// Create Go sub-project
	goProject := filepath.Join(suite.workspaceRoot, "go-service")
	suite.Require().NoError(os.MkdirAll(goProject, 0755))
	
	goMod := `module go-service

go 1.21

require (
	github.com/gin-gonic/gin v1.9.1
)
`
	suite.Require().NoError(os.WriteFile(filepath.Join(goProject, "go.mod"), []byte(goMod), 0644))
	
	mainGo := `package main

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	})
	r.Run(":8080")
}

func HelloWorld() string {
	return "Hello, World!"
}
`
	suite.Require().NoError(os.WriteFile(filepath.Join(goProject, "main.go"), []byte(mainGo), 0644))
	
	// Create Python sub-project
	pythonProject := filepath.Join(suite.workspaceRoot, "python-api")
	suite.Require().NoError(os.MkdirAll(pythonProject, 0755))
	
	requirements := `fastapi==0.104.1
uvicorn==0.24.0
pydantic==2.4.2
`
	suite.Require().NoError(os.WriteFile(filepath.Join(pythonProject, "requirements.txt"), []byte(requirements), 0644))
	
	mainPy := `from fastapi import FastAPI
from pydantic import BaseModel

app = FastAPI()

class Item(BaseModel):
    name: str
    price: float
    is_offer: bool = False

@app.get("/")
def read_root():
    return {"Hello": "World"}

@app.get("/items/{item_id}")
def read_item(item_id: int, q: str = None):
    return {"item_id": item_id, "q": q}

@app.post("/items/")
def create_item(item: Item):
    return item

def hello_python():
    return "Hello from Python!"
`
	suite.Require().NoError(os.WriteFile(filepath.Join(pythonProject, "main.py"), []byte(mainPy), 0644))
	
	// Create TypeScript sub-project
	tsProject := filepath.Join(suite.workspaceRoot, "ts-frontend")
	suite.Require().NoError(os.MkdirAll(tsProject, 0755))
	
	packageJson := `{
  "name": "ts-frontend",
  "version": "1.0.0",
  "description": "TypeScript frontend application",
  "main": "src/index.ts",
  "scripts": {
    "build": "tsc",
    "start": "node dist/index.js"
  },
  "dependencies": {
    "express": "^4.18.2",
    "typescript": "^5.2.2"
  },
  "devDependencies": {
    "@types/express": "^4.17.20",
    "@types/node": "^20.8.7"
  }
}
`
	suite.Require().NoError(os.WriteFile(filepath.Join(tsProject, "package.json"), []byte(packageJson), 0644))
	
	srcDir := filepath.Join(tsProject, "src")
	suite.Require().NoError(os.MkdirAll(srcDir, 0755))
	
	indexTs := `import express from 'express';

const app = express();
const port = 3000;

app.use(express.json());

interface User {
    id: number;
    name: string;
    email: string;
}

const users: User[] = [];

app.get('/', (req, res) => {
    res.json({ message: 'Hello from TypeScript!' });
});

app.get('/users', (req, res) => {
    res.json(users);
});

app.post('/users', (req, res) => {
    const newUser: User = {
        id: users.length + 1,
        name: req.body.name,
        email: req.body.email
    };
    users.push(newUser);
    res.status(201).json(newUser);
});

app.listen(port, () => {
    console.log(\`Server running at http://localhost:\${port}\`);
});

export function helloTypeScript(): string {
    return "Hello from TypeScript!";
}
`
	suite.Require().NoError(os.WriteFile(filepath.Join(srcDir, "index.ts"), []byte(indexTs), 0644))
}

func (suite *GatewayRoutingTestSuite) setupGatewayConfig() {
	suite.workspaceConfig = &workspace.WorkspaceConfig{
		Workspace: config.WorkspaceConfig{
			RootPath: suite.workspaceRoot,
		},
		Servers: map[string]*config.ServerConfig{
			"gopls": {
				Command:   "gopls",
				Args:      []string{"serve"},
				Languages: []string{"go"},
				Transport: "stdio",
				Enabled:   true,
			},
			"pylsp": {
				Command:   "pylsp",
				Args:      []string{},
				Languages: []string{"python"},
				Transport: "stdio",
				Enabled:   true,
			},
			"typescript-language-server": {
				Command:   "typescript-language-server",
				Args:      []string{"--stdio"},
				Languages: []string{"typescript", "javascript"},
				Transport: "stdio",
				Enabled:   true,
			},
		},
		Logging: config.LoggingConfig{
			Level: "debug",
		},
	}
	
	suite.gatewayConfig = &workspace.WorkspaceGatewayConfig{
		WorkspaceRoot: suite.workspaceRoot,
		ExtensionMapping: map[string]string{
			"go":   "go",
			"py":   "python",
			"ts":   "typescript",
			"js":   "javascript",
		},
		Timeout:       30 * time.Second,
		EnableLogging: true,
	}
}

func (suite *GatewayRoutingTestSuite) TestGatewayInitialization() {
	suite.T().Log("Testing gateway initialization")
	
	// Test gateway is properly initialized
	health := suite.gateway.Health()
	suite.NotNil(health)
	suite.Equal(suite.workspaceRoot, health.WorkspaceRoot)
	suite.WithinDuration(time.Now(), health.LastCheck, 5*time.Second)
}

func (suite *GatewayRoutingTestSuite) TestSubProjectDetection() {
	suite.T().Log("Testing sub-project detection")
	
	err := suite.gateway.Start(suite.ctx)
	suite.Require().NoError(err, "Failed to start gateway")
	
	// Test sub-project detection
	subProjects := suite.gateway.GetSubProjects()
	suite.NotNil(subProjects)
	
	// Verify detected projects
	projectTypes := make(map[string]bool)
	for _, project := range subProjects {
		projectTypes[project.ProjectType] = true
		suite.NotEmpty(project.ID)
		suite.NotEmpty(project.Root)
		suite.True(strings.HasPrefix(project.Root, suite.workspaceRoot))
	}
	
	expectedTypes := []string{"go", "python", "typescript"}
	for _, expectedType := range expectedTypes {
		suite.True(projectTypes[expectedType], "Expected project type %s not found", expectedType)
	}
}

func (suite *GatewayRoutingTestSuite) TestSubProjectClientRetrieval() {
	suite.T().Log("Testing sub-project client retrieval")
	
	err := suite.gateway.Start(suite.ctx)
	suite.Require().NoError(err, "Failed to start gateway")
	
	subProjects := suite.gateway.GetSubProjects()
	suite.Require().NotEmpty(subProjects, "No sub-projects detected")
	
	// Test client retrieval for each project
	for _, project := range subProjects {
		for _, language := range project.Languages {
			client, err := suite.gateway.GetSubProjectClient(project.ID, language)
			if err == nil {
				suite.NotNil(client, "Client should not be nil for project %s, language %s", project.ID, language)
			} else {
				suite.T().Logf("Client not available for project %s, language %s: %v", project.ID, language, err)
			}
		}
	}
}

func (suite *GatewayRoutingTestSuite) TestRoutingMetrics() {
	suite.T().Log("Testing routing metrics")
	
	err := suite.gateway.Start(suite.ctx)
	suite.Require().NoError(err, "Failed to start gateway")
	
	metrics := suite.gateway.GetRoutingMetrics()
	if metrics != nil {
		suite.NotNil(metrics.StrategyUsage)
		suite.WithinDuration(time.Now(), metrics.LastUpdated, 5*time.Minute)
	}
}

func (suite *GatewayRoutingTestSuite) TestSubProjectRefresh() {
	suite.T().Log("Testing sub-project refresh")
	
	err := suite.gateway.Start(suite.ctx)
	suite.Require().NoError(err, "Failed to start gateway")
	
	initialProjects := suite.gateway.GetSubProjects()
	initialCount := len(initialProjects)
	
	// Refresh sub-projects
	err = suite.gateway.RefreshSubProjects(suite.ctx)
	suite.NoError(err, "Failed to refresh sub-projects")
	
	refreshedProjects := suite.gateway.GetSubProjects()
	suite.GreaterOrEqual(len(refreshedProjects), initialCount, "Project count should not decrease after refresh")
}

func (suite *GatewayRoutingTestSuite) TestJSONRPCRouting() {
	suite.T().Log("Testing JSON-RPC routing")
	
	err := suite.gateway.Start(suite.ctx)
	suite.Require().NoError(err, "Failed to start gateway")
	
	// Test routing for different file types
	testCases := []struct {
		name     string
		method   string
		fileURI  string
		language string
	}{
		{
			name:     "Go file hover",
			method:   "textDocument/hover",
			fileURI:  "file://" + filepath.Join(suite.workspaceRoot, "go-service", "main.go"),
			language: "go",
		},
		{
			name:     "Python file definition",
			method:   "textDocument/definition",
			fileURI:  "file://" + filepath.Join(suite.workspaceRoot, "python-api", "main.py"),
			language: "python",
		},
		{
			name:     "TypeScript file references",
			method:   "textDocument/references",
			fileURI:  "file://" + filepath.Join(suite.workspaceRoot, "ts-frontend", "src", "index.ts"),
			language: "typescript",
		},
	}
	
	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			// Create JSON-RPC request
			request := workspace.JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      1,
				Method:  tc.method,
				Params: map[string]interface{}{
					"textDocument": map[string]interface{}{
						"uri": tc.fileURI,
					},
					"position": map[string]interface{}{
						"line":      0,
						"character": 0,
					},
				},
			}
			
			requestJSON, err := json.Marshal(request)
			suite.Require().NoError(err)
			
			// Create HTTP request
			req := httptest.NewRequest(http.MethodPost, "/jsonrpc", bytes.NewReader(requestJSON))
			req.Header.Set("Content-Type", "application/json")
			
			// Create response recorder
			recorder := httptest.NewRecorder()
			
			// Test routing (note: may fail if LSP servers not available, but should not panic)
			suite.NotPanics(func() {
				suite.gateway.HandleJSONRPC(recorder, req)
			}, "JSON-RPC handling should not panic")
			
			// Basic response validation
			suite.Equal("application/json", recorder.Header().Get("Content-Type"))
		})
	}
}

func (suite *GatewayRoutingTestSuite) TestGatewayHealth() {
	suite.T().Log("Testing gateway health monitoring")
	
	// Test health before starting
	health := suite.gateway.Health()
	suite.Equal(suite.workspaceRoot, health.WorkspaceRoot)
	suite.WithinDuration(time.Now(), health.LastCheck, 5*time.Second)
	
	// Start gateway and test health
	err := suite.gateway.Start(suite.ctx)
	suite.Require().NoError(err, "Failed to start gateway")
	
	health = suite.gateway.Health()
	suite.Equal(suite.workspaceRoot, health.WorkspaceRoot)
	suite.GreaterOrEqual(health.ActiveClients, 0)
	suite.NotNil(health.ClientStatuses)
	suite.NotNil(health.SubProjectClients)
	
	// Test health contains sub-project information
	suite.GreaterOrEqual(health.SubProjects, 0)
	
	// Test health check completes within reasonable time
	start := time.Now()
	health = suite.gateway.Health()
	duration := time.Since(start)
	suite.Less(duration, 100*time.Millisecond, "Health check should complete within 100ms")
}

func (suite *GatewayRoutingTestSuite) TestSubProjectRoutingToggle() {
	suite.T().Log("Testing sub-project routing enable/disable")
	
	err := suite.gateway.Start(suite.ctx)
	suite.Require().NoError(err, "Failed to start gateway")
	
	// Test enabling sub-project routing
	suite.gateway.EnableSubProjectRouting(true)
	
	// Test disabling sub-project routing
	suite.gateway.EnableSubProjectRouting(false)
	
	// Verify gateway continues to function
	health := suite.gateway.Health()
	suite.NotNil(health)
}

func (suite *GatewayRoutingTestSuite) TestErrorHandling() {
	suite.T().Log("Testing error handling scenarios")
	
	err := suite.gateway.Start(suite.ctx)
	suite.Require().NoError(err, "Failed to start gateway")
	
	// Test invalid sub-project ID
	_, err = suite.gateway.GetSubProjectClient("invalid-project-id", "go")
	suite.Error(err, "Should return error for invalid project ID")
	
	// Test invalid JSON-RPC request
	req := httptest.NewRequest(http.MethodPost, "/jsonrpc", strings.NewReader("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	
	suite.NotPanics(func() {
		suite.gateway.HandleJSONRPC(recorder, req)
	}, "Should handle invalid JSON gracefully")
	
	// Test unsupported HTTP method
	req = httptest.NewRequest(http.MethodGet, "/jsonrpc", nil)
	recorder = httptest.NewRecorder()
	
	suite.NotPanics(func() {
		suite.gateway.HandleJSONRPC(recorder, req)
	}, "Should handle unsupported HTTP method gracefully")
}

func (suite *GatewayRoutingTestSuite) TestConcurrentRequests() {
	suite.T().Log("Testing concurrent request handling")
	
	err := suite.gateway.Start(suite.ctx)
	suite.Require().NoError(err, "Failed to start gateway")
	
	// Test concurrent health checks
	const numGoroutines = 10
	results := make(chan bool, numGoroutines)
	
	for i := 0; i < numGoroutines; i++ {
		go func() {
			health := suite.gateway.Health()
			results <- health.WorkspaceRoot == suite.workspaceRoot
		}()
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		select {
		case result := <-results:
			suite.True(result, "Concurrent health check failed")
		case <-time.After(5 * time.Second):
			suite.Fail("Concurrent health check timed out")
		}
	}
}

func (suite *GatewayRoutingTestSuite) TestShutdown() {
	suite.T().Log("Testing gateway shutdown")
	
	err := suite.gateway.Start(suite.ctx)
	suite.Require().NoError(err, "Failed to start gateway")
	
	// Verify gateway is running
	health := suite.gateway.Health()
	suite.NotNil(health)
	
	// Test graceful shutdown
	err = suite.gateway.Stop()
	suite.NoError(err, "Gateway shutdown should not return error")
	
	// Test double shutdown (should be safe)
	err = suite.gateway.Stop()
	suite.NoError(err, "Double shutdown should be safe")
}