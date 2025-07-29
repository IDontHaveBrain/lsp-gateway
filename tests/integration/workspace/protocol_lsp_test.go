package workspace_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"lsp-gateway/internal/config"
	"lsp-gateway/internal/gateway"
	"lsp-gateway/internal/transport"
	"lsp-gateway/internal/workspace"
)

// LSPProtocolComplianceTestSuite tests LSP protocol compliance with multi-project routing
type LSPProtocolComplianceTestSuite struct {
	suite.Suite
	tempDir        string
	workspaceRoot  string
	gateway        workspace.WorkspaceGateway
	workspaceConfig *workspace.WorkspaceConfig
	gatewayConfig  *workspace.WorkspaceGatewayConfig
	ctx            context.Context
	cancel         context.CancelFunc
	clients        map[string]transport.LSPClient
}

func TestLSPProtocolComplianceTestSuite(t *testing.T) {
	suite.Run(t, new(LSPProtocolComplianceTestSuite))
}

func (suite *LSPProtocolComplianceTestSuite) SetupSuite() {
	suite.ctx, suite.cancel = context.WithTimeout(context.Background(), 5*time.Minute)
	
	var err error
	suite.tempDir, err = os.MkdirTemp("", "lsp-protocol-test-*")
	suite.Require().NoError(err, "Failed to create temp directory")
	
	suite.workspaceRoot = filepath.Join(suite.tempDir, "workspace")
	err = os.MkdirAll(suite.workspaceRoot, 0755)
	suite.Require().NoError(err)
	
	suite.clients = make(map[string]transport.LSPClient)
	
	suite.setupMultiProjectWorkspace()
	suite.setupLSPProtocolConfig()
}

func (suite *LSPProtocolComplianceTestSuite) TearDownSuite() {
	if suite.gateway != nil {
		suite.gateway.Stop()
	}
	
	// Stop all LSP clients
	for _, client := range suite.clients {
		if client != nil && client.IsActive() {
			client.Stop()
		}
	}
	
	suite.cancel()
	os.RemoveAll(suite.tempDir)
}

func (suite *LSPProtocolComplianceTestSuite) SetupTest() {
	suite.gateway = workspace.NewWorkspaceGateway()
	err := suite.gateway.Initialize(suite.ctx, suite.workspaceConfig, suite.gatewayConfig)
	suite.Require().NoError(err, "Failed to initialize gateway")
}

func (suite *LSPProtocolComplianceTestSuite) TearDownTest() {
	if suite.gateway != nil {
		suite.gateway.Stop()
	}
}

func (suite *LSPProtocolComplianceTestSuite) setupMultiProjectWorkspace() {
	// Create Go sub-project
	goProject := filepath.Join(suite.workspaceRoot, "go-service")
	suite.Require().NoError(os.MkdirAll(goProject, 0755))
	
	goMod := `module go-service

go 1.21

require (
	github.com/stretchr/testify v1.8.4
)
`
	suite.Require().NoError(os.WriteFile(filepath.Join(goProject, "go.mod"), []byte(goMod), 0644))
	
	mainGo := `package main

import (
	"fmt"
	"log"
	"net/http"
)

// Server represents an HTTP server
type Server struct {
	Port int
	Router *http.ServeMux
}

// NewServer creates a new server instance
func NewServer(port int) *Server {
	return &Server{
		Port: port,
		Router: http.NewServeMux(),
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	s.Router.HandleFunc("/health", s.healthHandler)
	s.Router.HandleFunc("/api/status", s.statusHandler)
	
	addr := fmt.Sprintf(":%d", s.Port)
	log.Printf("Server starting on %s", addr)
	return http.ListenAndServe(addr, s.Router)
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "OK")
}

func (s *Server) statusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"status": "running", "version": "1.0.0"}`)
}

func main() {
	server := NewServer(8080)
	if err := server.Start(); err != nil {
		log.Fatal(err)
	}
}
`
	suite.Require().NoError(os.WriteFile(filepath.Join(goProject, "main.go"), []byte(mainGo), 0644))
	
	// Create Python sub-project
	pythonProject := filepath.Join(suite.workspaceRoot, "python-api")
	suite.Require().NoError(os.MkdirAll(pythonProject, 0755))
	
	requirementsTxt := `fastapi==0.104.1
uvicorn==0.24.0
pydantic==2.5.0
`
	suite.Require().NoError(os.WriteFile(filepath.Join(pythonProject, "requirements.txt"), []byte(requirementsTxt), 0644))
	
	mainPy := `"""FastAPI application for testing LSP protocol compliance."""

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from typing import List, Optional
import uvicorn

app = FastAPI(title="Python API", version="1.0.0")

class Item(BaseModel):
    id: int
    name: str
    description: Optional[str] = None
    price: float
    tax: Optional[float] = None

class ItemResponse(BaseModel):
    id: int
    name: str
    total_price: float

items_db: List[Item] = []

@app.get("/")
async def root():
    """Root endpoint returning API info."""
    return {"message": "Python API", "version": "1.0.0"}

@app.get("/health")
async def health_check():
    """Health check endpoint."""
    return {"status": "healthy"}

@app.post("/items/", response_model=ItemResponse)
async def create_item(item: Item):
    """Create a new item."""
    items_db.append(item)
    total_price = item.price + (item.tax or 0)
    return ItemResponse(id=item.id, name=item.name, total_price=total_price)

@app.get("/items/{item_id}", response_model=Item)
async def read_item(item_id: int):
    """Get item by ID."""
    for item in items_db:
        if item.id == item_id:
            return item
    raise HTTPException(status_code=404, detail="Item not found")

@app.get("/items/", response_model=List[Item])
async def list_items(skip: int = 0, limit: int = 100):
    """List all items with pagination."""
    return items_db[skip : skip + limit]

def calculate_total_price(item: Item) -> float:
    """Calculate total price including tax."""
    return item.price + (item.tax or 0)

if __name__ == "__main__":
    uvicorn.run(app, host="0.0.0.0", port=8000)
`
	suite.Require().NoError(os.WriteFile(filepath.Join(pythonProject, "main.py"), []byte(mainPy), 0644))
	
	// Create TypeScript sub-project
	tsProject := filepath.Join(suite.workspaceRoot, "ts-frontend")
	suite.Require().NoError(os.MkdirAll(filepath.Join(tsProject, "src"), 0755))
	
	packageJson := `{
  "name": "ts-frontend",
  "version": "1.0.0",
  "description": "TypeScript frontend for testing",
  "main": "dist/index.js",
  "scripts": {
    "build": "tsc",
    "start": "node dist/index.js",
    "dev": "ts-node src/index.ts"
  },
  "dependencies": {
    "express": "^4.18.2",
    "cors": "^2.8.5"
  },
  "devDependencies": {
    "@types/express": "^4.17.21",
    "@types/cors": "^2.8.17",
    "@types/node": "^20.10.0",
    "typescript": "^5.3.0",
    "ts-node": "^10.9.0"
  }
}
`
	suite.Require().NoError(os.WriteFile(filepath.Join(tsProject, "package.json"), []byte(packageJson), 0644))
	
	tsConfig := `{
  "compilerOptions": {
    "target": "ES2020",
    "module": "commonjs",
    "lib": ["ES2020"],
    "outDir": "./dist",
    "rootDir": "./src",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "forceConsistentCasingInFileNames": true,
    "resolveJsonModule": true,
    "declaration": true,
    "declarationMap": true,
    "sourceMap": true
  },
  "include": [
    "src/**/*"
  ],
  "exclude": [
    "node_modules",
    "dist"
  ]
}
`
	suite.Require().NoError(os.WriteFile(filepath.Join(tsProject, "tsconfig.json"), []byte(tsConfig), 0644))
	
	indexTs := `import express, { Application, Request, Response } from 'express';
import cors from 'cors';

interface User {
  id: number;
  name: string;
  email: string;
  isActive: boolean;
}

interface ApiResponse<T> {
  success: boolean;
  data?: T;
  message?: string;
}

class UserService {
  private users: User[] = [
    { id: 1, name: 'John Doe', email: 'john@example.com', isActive: true },
    { id: 2, name: 'Jane Smith', email: 'jane@example.com', isActive: false },
  ];

  getAllUsers(): User[] {
    return this.users;
  }

  getUserById(id: number): User | undefined {
    return this.users.find(user => user.id === id);
  }

  createUser(userData: Omit<User, 'id'>): User {
    const newUser: User = {
      id: Math.max(...this.users.map(u => u.id)) + 1,
      ...userData
    };
    this.users.push(newUser);
    return newUser;
  }

  updateUser(id: number, updates: Partial<User>): User | null {
    const userIndex = this.users.findIndex(user => user.id === id);
    if (userIndex === -1) return null;
    
    this.users[userIndex] = { ...this.users[userIndex], ...updates };
    return this.users[userIndex];
  }

  deleteUser(id: number): boolean {
    const userIndex = this.users.findIndex(user => user.id === id);
    if (userIndex === -1) return false;
    
    this.users.splice(userIndex, 1);
    return true;
  }
}

class App {
  private app: Application;
  private userService: UserService;
  private port: number;

  constructor(port: number = 3000) {
    this.app = express();
    this.userService = new UserService();
    this.port = port;
    this.setupMiddleware();
    this.setupRoutes();
  }

  private setupMiddleware(): void {
    this.app.use(cors());
    this.app.use(express.json());
    this.app.use(express.urlencoded({ extended: true }));
  }

  private setupRoutes(): void {
    this.app.get('/health', this.healthCheck.bind(this));
    this.app.get('/api/users', this.getAllUsers.bind(this));
    this.app.get('/api/users/:id', this.getUserById.bind(this));
    this.app.post('/api/users', this.createUser.bind(this));
    this.app.put('/api/users/:id', this.updateUser.bind(this));
    this.app.delete('/api/users/:id', this.deleteUser.bind(this));
  }

  private healthCheck(req: Request, res: Response): void {
    const response: ApiResponse<{ status: string; timestamp: string }> = {
      success: true,
      data: {
        status: 'healthy',
        timestamp: new Date().toISOString()
      }
    };
    res.json(response);
  }

  private getAllUsers(req: Request, res: Response): void {
    const users = this.userService.getAllUsers();
    const response: ApiResponse<User[]> = {
      success: true,
      data: users
    };
    res.json(response);
  }

  private getUserById(req: Request, res: Response): void {
    const id = parseInt(req.params.id);
    const user = this.userService.getUserById(id);
    
    if (!user) {
      const response: ApiResponse<null> = {
        success: false,
        message: 'User not found'
      };
      res.status(404).json(response);
      return;
    }

    const response: ApiResponse<User> = {
      success: true,
      data: user
    };
    res.json(response);
  }

  private createUser(req: Request, res: Response): void {
    try {
      const userData = req.body as Omit<User, 'id'>;
      const newUser = this.userService.createUser(userData);
      
      const response: ApiResponse<User> = {
        success: true,
        data: newUser
      };
      res.status(201).json(response);
    } catch (error) {
      const response: ApiResponse<null> = {
        success: false,
        message: 'Failed to create user'
      };
      res.status(400).json(response);
    }
  }

  private updateUser(req: Request, res: Response): void {
    const id = parseInt(req.params.id);
    const updates = req.body as Partial<User>;
    
    const updatedUser = this.userService.updateUser(id, updates);
    
    if (!updatedUser) {
      const response: ApiResponse<null> = {
        success: false,
        message: 'User not found'
      };
      res.status(404).json(response);
      return;
    }

    const response: ApiResponse<User> = {
      success: true,
      data: updatedUser
    };
    res.json(response);
  }

  private deleteUser(req: Request, res: Response): void {
    const id = parseInt(req.params.id);
    const deleted = this.userService.deleteUser(id);
    
    if (!deleted) {
      const response: ApiResponse<null> = {
        success: false,
        message: 'User not found'
      };
      res.status(404).json(response);
      return;
    }

    const response: ApiResponse<null> = {
      success: true,
      message: 'User deleted successfully'
    };
    res.json(response);
  }

  public start(): void {
    this.app.listen(this.port, () => {
      console.log(`Server is running on port ${this.port}`);
    });
  }
}

const app = new App(3000);
app.start();
`
	suite.Require().NoError(os.WriteFile(filepath.Join(tsProject, "src", "index.ts"), []byte(indexTs), 0644))
}

func (suite *LSPProtocolComplianceTestSuite) setupLSPProtocolConfig() {
	// Create configuration for multiple language servers
	suite.workspaceConfig = &workspace.WorkspaceConfig{
		Root: suite.workspaceRoot,
		Projects: []workspace.ProjectConfig{
			{
				Name: "go-service",
				Type: "go",
				Path: filepath.Join(suite.workspaceRoot, "go-service"),
				Languages: []string{"go"},
				RootMarkers: []string{"go.mod"},
			},
			{
				Name: "python-api", 
				Type: "python",
				Path: filepath.Join(suite.workspaceRoot, "python-api"),
				Languages: []string{"python"},
				RootMarkers: []string{"requirements.txt", "pyproject.toml"},
			},
			{
				Name: "ts-frontend",
				Type: "typescript",
				Path: filepath.Join(suite.workspaceRoot, "ts-frontend"),
				Languages: []string{"typescript", "javascript"},
				RootMarkers: []string{"package.json", "tsconfig.json"},
			},
		},
	}

	suite.gatewayConfig = &workspace.WorkspaceGatewayConfig{
		Servers: []config.ServerConfig{
			{
				Name: "gopls",
				Languages: []string{"go"},
				Command: "gopls",
				Args: []string{},
				Transport: "stdio",
				RootMarkers: []string{"go.mod", "go.sum"},
				Priority: 1,
				Weight: 1.0,
			},
			{
				Name: "pylsp",
				Languages: []string{"python"},
				Command: "pylsp",
				Args: []string{},
				Transport: "stdio",
				RootMarkers: []string{"requirements.txt", "pyproject.toml", "setup.py"},
				Priority: 1,
				Weight: 1.0,
			},
			{
				Name: "typescript-language-server",
				Languages: []string{"typescript", "javascript"},
				Command: "typescript-language-server",
				Args: []string{"--stdio"},
				Transport: "stdio",
				RootMarkers: []string{"package.json", "tsconfig.json"},
				Priority: 1,
				Weight: 1.0,
			},
		},
		Port: 8080,
		Timeout: 45 * time.Second,
		MaxConcurrentRequests: 150,
	}
}

// Test textDocument/definition routing
func (suite *LSPProtocolComplianceTestSuite) TestTextDocumentDefinition() {
	ctx, cancel := context.WithTimeout(suite.ctx, 5*time.Second)
	defer cancel()

	testCases := []struct {
		name     string
		fileUri  string
		line     int
		char     int
		project  string
		language string
	}{
		{
			name:     "Go function definition",
			fileUri:  fmt.Sprintf("file://%s", filepath.Join(suite.workspaceRoot, "go-service", "main.go")),
			line:     18, // NewServer call
			char:     15,
			project:  "go-service",
			language: "go",
		},
		{
			name:     "Python class definition",
			fileUri:  fmt.Sprintf("file://%s", filepath.Join(suite.workspaceRoot, "python-api", "main.py")),
			line:     15, // Item class
			char:     10,
			project:  "python-api",
			language: "python",
		},
		{
			name:     "TypeScript interface definition",
			fileUri:  fmt.Sprintf("file://%s", filepath.Join(suite.workspaceRoot, "ts-frontend", "src", "index.ts")),
			line:     3, // User interface
			char:     15,
			project:  "ts-frontend",
			language: "typescript",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			request := gateway.JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      1,
				Method:  "textDocument/definition",
				Params: map[string]interface{}{
					"textDocument": map[string]interface{}{
						"uri": tc.fileUri,
					},
					"position": map[string]interface{}{
						"line":      tc.line,
						"character": tc.char,
					},
				},
			}

			result, err := suite.gateway.RouteRequest(ctx, &request, &gateway.RequestContext{
				ProjectType: tc.project,
				Language:    tc.language,
				FileURI:     tc.fileUri,
				RequestType: "textDocument/definition",
			})

			suite.NoError(err, "textDocument/definition request should succeed")
			suite.NotNil(result, "Result should not be nil")
			
			// Verify response structure
			response, ok := result.(*gateway.JSONRPCResponse)
			suite.True(ok, "Result should be JSONRPCResponse")
			suite.Equal("2.0", response.JSONRPC, "Should have correct JSON-RPC version")
			suite.Nil(response.Error, "Should not have error")
			suite.NotNil(response.Result, "Should have result data")
		})
	}
}

// Test textDocument/references routing
func (suite *LSPProtocolComplianceTestSuite) TestTextDocumentReferences() {
	ctx, cancel := context.WithTimeout(suite.ctx, 5*time.Second)
	defer cancel()

	testCases := []struct {
		name     string
		fileUri  string
		line     int
		char     int
		project  string
		language string
	}{
		{
			name:     "Go struct references",
			fileUri:  fmt.Sprintf("file://%s", filepath.Join(suite.workspaceRoot, "go-service", "main.go")),
			line:     8, // Server struct
			char:     5,
			project:  "go-service",
			language: "go",
		},
		{
			name:     "Python class references",
			fileUri:  fmt.Sprintf("file://%s", filepath.Join(suite.workspaceRoot, "python-api", "main.py")),
			line:     10, // Item model
			char:     6,
			project:  "python-api",
			language: "python",
		},
		{
			name:     "TypeScript interface references",
			fileUri:  fmt.Sprintf("file://%s", filepath.Join(suite.workspaceRoot, "ts-frontend", "src", "index.ts")),
			line:     3, // User interface
			char:     10,
			project:  "ts-frontend",
			language: "typescript",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			request := gateway.JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      2,
				Method:  "textDocument/references",
				Params: map[string]interface{}{
					"textDocument": map[string]interface{}{
						"uri": tc.fileUri,
					},
					"position": map[string]interface{}{
						"line":      tc.line,
						"character": tc.char,
					},
					"context": map[string]interface{}{
						"includeDeclaration": true,
					},
				},
			}

			result, err := suite.gateway.RouteRequest(ctx, &request, &gateway.RequestContext{
				ProjectType: tc.project,
				Language:    tc.language,
				FileURI:     tc.fileUri,
				RequestType: "textDocument/references",
			})

			suite.NoError(err, "textDocument/references request should succeed")
			suite.NotNil(result, "Result should not be nil")
			
			// Verify response structure
			response, ok := result.(*gateway.JSONRPCResponse)
			suite.True(ok, "Result should be JSONRPCResponse")
			suite.Equal("2.0", response.JSONRPC, "Should have correct JSON-RPC version")
			suite.Nil(response.Error, "Should not have error")
		})
	}
}

// Test textDocument/hover routing
func (suite *LSPProtocolComplianceTestSuite) TestTextDocumentHover() {
	ctx, cancel := context.WithTimeout(suite.ctx, 5*time.Second)
	defer cancel()

	testCases := []struct {
		name     string
		fileUri  string
		line     int
		char     int
		project  string
		language string
	}{
		{
			name:     "Go function hover",
			fileUri:  fmt.Sprintf("file://%s", filepath.Join(suite.workspaceRoot, "go-service", "main.go")),
			line:     36, // fmt.Fprint
			char:     5,
			project:  "go-service",
			language: "go",
		},
		{
			name:     "Python function hover",
			fileUri:  fmt.Sprintf("file://%s", filepath.Join(suite.workspaceRoot, "python-api", "main.py")),
			line:     28, // FastAPI app
			char:     10,
			project:  "python-api",
			language: "python",
		},
		{
			name:     "TypeScript method hover",
			fileUri:  fmt.Sprintf("file://%s", filepath.Join(suite.workspaceRoot, "ts-frontend", "src", "index.ts")),
			line:     85, // express() call
			char:     20,
			project:  "ts-frontend",
			language: "typescript",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			request := gateway.JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      3,
				Method:  "textDocument/hover",
				Params: map[string]interface{}{
					"textDocument": map[string]interface{}{
						"uri": tc.fileUri,
					},
					"position": map[string]interface{}{
						"line":      tc.line,
						"character": tc.char,
					},
				},
			}

			result, err := suite.gateway.RouteRequest(ctx, &request, &gateway.RequestContext{
				ProjectType: tc.project,
				Language:    tc.language,
				FileURI:     tc.fileUri,
				RequestType: "textDocument/hover",
			})

			suite.NoError(err, "textDocument/hover request should succeed")
			suite.NotNil(result, "Result should not be nil")
			
			// Verify response structure
			response, ok := result.(*gateway.JSONRPCResponse)
			suite.True(ok, "Result should be JSONRPCResponse")
			suite.Equal("2.0", response.JSONRPC, "Should have correct JSON-RPC version")
			suite.Nil(response.Error, "Should not have error")
		})
	}
}

// Test textDocument/documentSymbol routing
func (suite *LSPProtocolComplianceTestSuite) TestTextDocumentDocumentSymbol() {
	ctx, cancel := context.WithTimeout(suite.ctx, 5*time.Second)
	defer cancel()

	testCases := []struct {
		name     string
		fileUri  string
		project  string
		language string
	}{
		{
			name:     "Go document symbols",
			fileUri:  fmt.Sprintf("file://%s", filepath.Join(suite.workspaceRoot, "go-service", "main.go")),
			project:  "go-service",
			language: "go",
		},
		{
			name:     "Python document symbols",
			fileUri:  fmt.Sprintf("file://%s", filepath.Join(suite.workspaceRoot, "python-api", "main.py")),
			project:  "python-api",
			language: "python",
		},
		{
			name:     "TypeScript document symbols",
			fileUri:  fmt.Sprintf("file://%s", filepath.Join(suite.workspaceRoot, "ts-frontend", "src", "index.ts")),
			project:  "ts-frontend",
			language: "typescript",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			request := gateway.JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      4,
				Method:  "textDocument/documentSymbol",
				Params: map[string]interface{}{
					"textDocument": map[string]interface{}{
						"uri": tc.fileUri,
					},
				},
			}

			result, err := suite.gateway.RouteRequest(ctx, &request, &gateway.RequestContext{
				ProjectType: tc.project,
				Language:    tc.language,
				FileURI:     tc.fileUri,
				RequestType: "textDocument/documentSymbol",
			})

			suite.NoError(err, "textDocument/documentSymbol request should succeed")
			suite.NotNil(result, "Result should not be nil")
			
			// Verify response structure
			response, ok := result.(*gateway.JSONRPCResponse)
			suite.True(ok, "Result should be JSONRPCResponse")
			suite.Equal("2.0", response.JSONRPC, "Should have correct JSON-RPC version")
			suite.Nil(response.Error, "Should not have error")
			
			// Verify result is array of symbols
			if response.Result != nil {
				resultBytes, err := json.Marshal(response.Result)
				suite.NoError(err, "Should marshal result")
				
				var symbols []interface{}
				err = json.Unmarshal(resultBytes, &symbols)
				suite.NoError(err, "Result should be array of symbols")
			}
		})
	}
}

// Test workspace/symbol routing
func (suite *LSPProtocolComplianceTestSuite) TestWorkspaceSymbol() {
	ctx, cancel := context.WithTimeout(suite.ctx, 10*time.Second)
	defer cancel()

	testCases := []struct {
		name     string
		query    string
		projects []string
	}{
		{
			name:     "Search for 'Server' across projects",
			query:    "Server",
			projects: []string{"go-service", "ts-frontend"},
		},
		{
			name:     "Search for 'main' across projects",
			query:    "main",
			projects: []string{"go-service", "python-api"},
		},
		{
			name:     "Search for 'User' in TypeScript project",
			query:    "User",
			projects: []string{"ts-frontend"},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			request := gateway.JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      5,
				Method:  "workspace/symbol",
				Params: map[string]interface{}{
					"query": tc.query,
				},
			}

			result, err := suite.gateway.RouteRequest(ctx, &request, &gateway.RequestContext{
				RequestType:         "workspace/symbol",
				WorkspaceRoot:       suite.workspaceRoot,
				RequiresAggregation: true,
			})

			suite.NoError(err, "workspace/symbol request should succeed")
			suite.NotNil(result, "Result should not be nil")
			
			// Verify response structure
			response, ok := result.(*gateway.JSONRPCResponse)
			suite.True(ok, "Result should be JSONRPCResponse")
			suite.Equal("2.0", response.JSONRPC, "Should have correct JSON-RPC version")
			suite.Nil(response.Error, "Should not have error")
		})
	}
}

// Test textDocument/completion routing
func (suite *LSPProtocolComplianceTestSuite) TestTextDocumentCompletion() {
	ctx, cancel := context.WithTimeout(suite.ctx, 5*time.Second)
	defer cancel()

	testCases := []struct {
		name     string
		fileUri  string
		line     int
		char     int
		project  string
		language string
	}{
		{
			name:     "Go completion",
			fileUri:  fmt.Sprintf("file://%s", filepath.Join(suite.workspaceRoot, "go-service", "main.go")),
			line:     25, // Inside NewServer function
			char:     10,
			project:  "go-service",
			language: "go",
		},
		{
			name:     "Python completion",
			fileUri:  fmt.Sprintf("file://%s", filepath.Join(suite.workspaceRoot, "python-api", "main.py")),
			line:     50, // Inside function
			char:     5,
			project:  "python-api",
			language: "python",
		},
		{
			name:     "TypeScript completion",
			fileUri:  fmt.Sprintf("file://%s", filepath.Join(suite.workspaceRoot, "ts-frontend", "src", "index.ts")),
			line:     90, // Inside method
			char:     15,
			project:  "ts-frontend",
			language: "typescript",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			request := gateway.JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      6,
				Method:  "textDocument/completion",
				Params: map[string]interface{}{
					"textDocument": map[string]interface{}{
						"uri": tc.fileUri,
					},
					"position": map[string]interface{}{
						"line":      tc.line,
						"character": tc.char,
					},
				},
			}

			result, err := suite.gateway.RouteRequest(ctx, &request, &gateway.RequestContext{
				ProjectType: tc.project,
				Language:    tc.language,
				FileURI:     tc.fileUri,
				RequestType: "textDocument/completion",
			})

			suite.NoError(err, "textDocument/completion request should succeed")
			suite.NotNil(result, "Result should not be nil")
			
			// Verify response structure
			response, ok := result.(*gateway.JSONRPCResponse)
			suite.True(ok, "Result should be JSONRPCResponse")
			suite.Equal("2.0", response.JSONRPC, "Should have correct JSON-RPC version")
			suite.Nil(response.Error, "Should not have error")
		})
	}
}

// Test protocol error handling
func (suite *LSPProtocolComplianceTestSuite) TestProtocolErrorHandling() {
	ctx, cancel := context.WithTimeout(suite.ctx, 5*time.Second)
	defer cancel()

	testCases := []struct {
		name           string
		request        gateway.JSONRPCRequest
		expectedError  bool
		expectedCode   int
	}{
		{
			name: "Invalid method",
			request: gateway.JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      1,
				Method:  "textDocument/invalidMethod",
				Params:  map[string]interface{}{},
			},
			expectedError: true,
			expectedCode:  gateway.MethodNotFound,
		},
		{
			name: "Missing required params",
			request: gateway.JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      2,
				Method:  "textDocument/definition",
				Params:  map[string]interface{}{},
			},
			expectedError: true,
			expectedCode:  gateway.InvalidParams,
		},
		{
			name: "Invalid URI format",
			request: gateway.JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      3,
				Method:  "textDocument/hover",
				Params: map[string]interface{}{
					"textDocument": map[string]interface{}{
						"uri": "invalid-uri",
					},
					"position": map[string]interface{}{
						"line":      0,
						"character": 0,
					},
				},
			},
			expectedError: true,
			expectedCode:  gateway.InvalidParams,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			result, err := suite.gateway.RouteRequest(ctx, &tc.request, &gateway.RequestContext{
				RequestType: tc.request.Method,
			})

			if tc.expectedError {
				suite.Error(err, "Should return error for invalid request")
			} else {
				suite.NoError(err, "Should not return error")
				suite.NotNil(result, "Result should not be nil")
				
				// Verify error response structure
				response, ok := result.(*gateway.JSONRPCResponse)
				suite.True(ok, "Result should be JSONRPCResponse")
				suite.NotNil(response.Error, "Should have error")
				suite.Equal(tc.expectedCode, response.Error.Code, "Should have correct error code")
			}
		})
	}
}

// Test cross-project URI resolution
func (suite *LSPProtocolComplianceTestSuite) TestCrossProjectURIResolution() {
	ctx, cancel := context.WithTimeout(suite.ctx, 5*time.Second)
	defer cancel()

	// Create cross-project file URIs
	goFileUri := fmt.Sprintf("file://%s", filepath.Join(suite.workspaceRoot, "go-service", "main.go"))
	pythonFileUri := fmt.Sprintf("file://%s", filepath.Join(suite.workspaceRoot, "python-api", "main.py"))
	tsFileUri := fmt.Sprintf("file://%s", filepath.Join(suite.workspaceRoot, "ts-frontend", "src", "index.ts"))

	testCases := []struct {
		name            string
		fileUri         string
		expectedProject string
		expectedLang    string
	}{
		{
			name:            "Go project resolution",
			fileUri:         goFileUri,
			expectedProject: "go-service",
			expectedLang:    "go",
		},
		{
			name:            "Python project resolution",
			fileUri:         pythonFileUri,
			expectedProject: "python-api",
			expectedLang:    "python",
		},
		{
			name:            "TypeScript project resolution",
			fileUri:         tsFileUri,
			expectedProject: "ts-frontend",
			expectedLang:    "typescript",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			request := gateway.JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      1,
				Method:  "textDocument/documentSymbol",
				Params: map[string]interface{}{
					"textDocument": map[string]interface{}{
						"uri": tc.fileUri,
					},
				},
			}

			// Test URI resolution through gateway
			context := suite.gateway.ResolveRequestContext(&request)
			suite.NotNil(context, "Should resolve request context")
			
			// Verify project and language detection
			if context.ProjectType != "" {
				suite.Contains(strings.ToLower(context.ProjectType), strings.ToLower(tc.expectedProject), 
					"Should detect correct project type")
			}
			
			if context.Language != "" {
				suite.Equal(tc.expectedLang, context.Language, "Should detect correct language")
			}
			
			suite.Equal(tc.fileUri, context.FileURI, "Should preserve file URI")
		})
	}
}

// Test performance requirements
func (suite *LSPProtocolComplianceTestSuite) TestPerformanceRequirements() {
	ctx, cancel := context.WithTimeout(suite.ctx, 10*time.Second)
	defer cancel()

	// Test LSP method routing performance (should complete within 5 seconds)
	testCases := []struct {
		name    string
		method  string
		params  interface{}
		timeout time.Duration
	}{
		{
			name:   "textDocument/definition performance",
			method: "textDocument/definition",
			params: map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": fmt.Sprintf("file://%s", filepath.Join(suite.workspaceRoot, "go-service", "main.go")),
				},
				"position": map[string]interface{}{
					"line":      15,
					"character": 10,
				},
			},
			timeout: 5 * time.Second,
		},
		{
			name:   "textDocument/hover performance",
			method: "textDocument/hover",
			params: map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri": fmt.Sprintf("file://%s", filepath.Join(suite.workspaceRoot, "python-api", "main.py")),
				},
				"position": map[string]interface{}{
					"line":      25,
					"character": 5,
				},
			},
			timeout: 5 * time.Second,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			start := time.Now()
			
			request := gateway.JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      1,
				Method:  tc.method,
				Params:  tc.params,
			}

			perfCtx, perfCancel := context.WithTimeout(ctx, tc.timeout)
			defer perfCancel()

			result, err := suite.gateway.RouteRequest(perfCtx, &request, &gateway.RequestContext{
				RequestType: tc.method,
			})

			duration := time.Since(start)
			
			suite.NoError(err, "Request should succeed within timeout")
			suite.NotNil(result, "Result should not be nil")
			suite.True(duration < tc.timeout, 
				fmt.Sprintf("Request should complete within %v, took %v", tc.timeout, duration))
		})
	}
}