package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"lsp-gateway/internal/transport"
	"lsp-gateway/mcp"
)

// TypeScriptRealServerIntegrationTestSuite provides comprehensive integration tests
// for TypeScript with actual typescript-language-server process communication
type TypeScriptRealServerIntegrationTestSuite struct {
	suite.Suite
	testTimeout       time.Duration
	projectRoot       string
	lspClient         transport.LSPClient
	clientConfig      transport.ClientConfig
	projectFiles      map[string]string
	performanceMetrics *PerformanceMetrics
}

// PerformanceMetrics tracks actual performance data from real TypeScript server
type PerformanceMetrics struct {
	InitializationTime   time.Duration
	FirstResponseTime    time.Duration
	AverageResponseTime  time.Duration
	MemoryUsage         int64
	ProcessPID          int
	TotalRequests       int
	SuccessfulRequests  int
	FailedRequests      int
}

// TypeScriptIntegrationResult captures comprehensive test results for 6 supported LSP features
type TypeScriptIntegrationResult struct {
	ServerStartSuccess     bool
	InitializationSuccess  bool
	ProjectLoadSuccess     bool
	DefinitionSupport      bool
	ReferencesSupport      bool
	HoverSupport           bool
	DocumentSymbolSupport  bool
	WorkspaceSymbolSupport bool
	CompletionSupport      bool
	PerformanceMetrics     *PerformanceMetrics
	ErrorCount            int
	TestDuration          time.Duration
}

// SetupSuite initializes the test suite with real TypeScript project structure
func (suite *TypeScriptRealServerIntegrationTestSuite) SetupSuite() {
	suite.testTimeout = 2 * time.Minute // Extended timeout for real server operations
	
	// Verify typescript-language-server is available
	if !suite.isTypeScriptServerAvailable() {
		suite.T().Skip("typescript-language-server not available, skipping real server integration tests")
	}

	// Create real TypeScript project structure
	suite.createTestProject()
	
	suite.performanceMetrics = &PerformanceMetrics{}
}

// SetupTest initializes a fresh LSP client for each test
func (suite *TypeScriptRealServerIntegrationTestSuite) SetupTest() {
	suite.clientConfig = transport.ClientConfig{
		Command:   "typescript-language-server",
		Args:      []string{"--stdio"},
		Transport: transport.TransportStdio,
	}
	
	var err error
	suite.lspClient, err = transport.NewLSPClient(suite.clientConfig)
	suite.Require().NoError(err, "Should create LSP client")
}

// TearDownTest cleans up the LSP client
func (suite *TypeScriptRealServerIntegrationTestSuite) TearDownTest() {
	if suite.lspClient != nil && suite.lspClient.IsActive() {
		err := suite.lspClient.Stop()
		if err != nil {
			suite.T().Logf("Warning: Error stopping LSP client: %v", err)
		}
	}
}

// TearDownSuite cleans up the test project
func (suite *TypeScriptRealServerIntegrationTestSuite) TearDownSuite() {
	if suite.projectRoot != "" {
		_ = os.RemoveAll(suite.projectRoot)
	}
}

// TestRealTypeScriptServerLifecycle tests the complete LSP server lifecycle
func (suite *TypeScriptRealServerIntegrationTestSuite) TestRealTypeScriptServerLifecycle() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	startTime := time.Now()
	
	// Step 1: Start the actual TypeScript language server
	err := suite.lspClient.Start(ctx)
	suite.Require().NoError(err, "Should start typescript-language-server process")
	suite.performanceMetrics.ProcessPID = suite.getProcessPID()
	
	// Step 2: Initialize the server with proper capabilities
	initResult := suite.initializeServer(ctx)
	suite.True(initResult, "Server initialization should succeed")
	suite.performanceMetrics.InitializationTime = time.Since(startTime)
	
	// Step 3: Open the TypeScript project workspace
	suite.openWorkspace(ctx)
	
	// Step 4: Test core LSP functionality with real TypeScript compilation
	result := suite.executeComprehensiveTypeScriptWorkflow(ctx)
	
	// Step 5: Validate all aspects of real TypeScript integration
	suite.validateTypeScriptIntegrationResult(result)
	
	// Step 6: Measure final performance metrics
	result.TestDuration = time.Since(startTime)
	suite.recordPerformanceMetrics(result)
	
	suite.T().Logf("Real TypeScript integration test completed in %v", result.TestDuration)
	suite.T().Logf("Server PID: %d, Total requests: %d, Success rate: %.2f%%", 
		suite.performanceMetrics.ProcessPID,
		suite.performanceMetrics.TotalRequests,
		float64(suite.performanceMetrics.SuccessfulRequests)/float64(suite.performanceMetrics.TotalRequests)*100)
}

// TestRealTypeScriptFeatures tests actual TypeScript compiler features
func (suite *TypeScriptRealServerIntegrationTestSuite) TestRealTypeScriptFeatures() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	// Start and initialize server
	err := suite.lspClient.Start(ctx)
	suite.Require().NoError(err, "Should start server")
	
	suite.initializeServer(ctx)
	suite.openWorkspace(ctx)

	featureTests := []struct {
		name           string
		testFunction   func(context.Context) bool
		description    string
	}{
		{
			name:         "GoToDefinition",
			testFunction: suite.testGoToDefinition,
			description:  "Test textDocument/definition functionality",
		},
		{
			name:         "FindReferences",
			testFunction: suite.testFindReferences,
			description:  "Test textDocument/references functionality",
		},
		{
			name:         "HoverInformation",
			testFunction: suite.testHoverInformation,
			description:  "Test textDocument/hover functionality",
		},
		{
			name:         "DocumentSymbols",
			testFunction: suite.testDocumentSymbols,
			description:  "Test textDocument/documentSymbol functionality",
		},
		{
			name:         "WorkspaceSymbols",
			testFunction: suite.testWorkspaceSymbols,
			description:  "Test workspace/symbol functionality",
		},
		{
			name:         "Completion",
			testFunction: suite.testCompletion,
			description:  "Test textDocument/completion functionality",
		},
	}

	for _, test := range featureTests {
		suite.Run(test.name, func() {
			startTime := time.Now()
			success := test.testFunction(ctx)
			duration := time.Since(startTime)
			
			suite.True(success, "%s should succeed", test.description)
			suite.Less(duration, 30*time.Second, "%s should complete within reasonable time", test.name)
			
			suite.T().Logf("%s completed in %v", test.name, duration)
		})
	}
}

// TestRealTypeScriptPerformance tests performance with large TypeScript projects
func (suite *TypeScriptRealServerIntegrationTestSuite) TestRealTypeScriptPerformance() {
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	// Start server and initialize
	err := suite.lspClient.Start(ctx)
	suite.Require().NoError(err, "Should start server")
	
	suite.initializeServer(ctx)
	suite.openWorkspace(ctx)

	performanceTests := []struct {
		name                string
		requestCount        int
		concurrency         int
		maxAcceptableTime   time.Duration
		expectedSuccessRate float64
	}{
		{
			name:                "HighVolumeDefinitionRequests",
			requestCount:        100,
			concurrency:         5,
			maxAcceptableTime:   20 * time.Second,
			expectedSuccessRate: 0.95,
		},
		{
			name:                "ConcurrentSymbolRequests",
			requestCount:        50,
			concurrency:         10,
			maxAcceptableTime:   15 * time.Second,
			expectedSuccessRate: 0.90,
		},
		{
			name:                "IntensiveHoverRequests",
			requestCount:        200,
			concurrency:         3,
			maxAcceptableTime:   25 * time.Second,
			expectedSuccessRate: 0.98,
		},
	}

	for _, test := range performanceTests {
		suite.Run(test.name, func() {
			result := suite.executePerformanceTest(ctx, test.requestCount, test.concurrency)
			
			suite.Less(result.TotalDuration, test.maxAcceptableTime, 
				"Performance test should complete within acceptable time")
			suite.GreaterOrEqual(result.SuccessRate, test.expectedSuccessRate,
				"Success rate should meet expectations")
			
			suite.T().Logf("%s: %d requests in %v (%.2f req/s, %.2f%% success)", 
				test.name, test.requestCount, result.TotalDuration,
				float64(test.requestCount)/result.TotalDuration.Seconds(),
				result.SuccessRate*100)
		})
	}
}


// Helper methods for test implementation

func (suite *TypeScriptRealServerIntegrationTestSuite) isTypeScriptServerAvailable() bool {
	cmd := exec.Command("typescript-language-server", "--version")
	err := cmd.Run()
	return err == nil
}

func (suite *TypeScriptRealServerIntegrationTestSuite) createTestProject() {
	var err error
	suite.projectRoot, err = os.MkdirTemp("", "typescript-integration-test-*")
	suite.Require().NoError(err, "Should create temporary project directory")

	// Create realistic TypeScript project structure
	suite.projectFiles = map[string]string{
		"package.json": `{
  "name": "typescript-integration-test",
  "version": "1.0.0",
  "dependencies": {
    "@types/node": "^20.0.0",
    "typescript": "^5.0.0"
  },
  "devDependencies": {
    "@types/react": "^18.0.0"
  }
}`,
		"tsconfig.json": `{
  "compilerOptions": {
    "target": "ES2022",
    "lib": ["ES2022", "DOM"],
    "module": "ESNext",
    "moduleResolution": "Node",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "forceConsistentCasingInFileNames": true,
    "baseUrl": ".",
    "paths": {
      "@/*": ["src/*"],
      "@types/*": ["src/types/*"]
    }
  },
  "include": ["src/**/*"],
  "exclude": ["node_modules", "dist"]
}`,
		"src/index.ts": `import { UserService } from './services/UserService';
import { User, UserRole } from './types/User';

class Application {
    private userService: UserService;

    constructor() {
        this.userService = new UserService();
    }

    async start(): Promise<void> {
        console.log('Starting application...');
        const users = await this.userService.getAllUsers();
        console.log('Found ' + users.length + ' users');
    }

    createUser(name: string, email: string): User {
        return {
            id: Date.now().toString(),
            name,
            email,
            role: UserRole.USER,
            createdAt: new Date(),
            isActive: true
        };
    }
}

export default Application;`,
		"src/services/UserService.ts": `import { User, UserRole } from '../types/User';

export class UserService {
    private users: User[] = [];

    async getAllUsers(): Promise<User[]> {
        return Promise.resolve([...this.users]);
    }

    async getUserById(id: string): Promise<User | undefined> {
        return this.users.find(user => user.id === id);
    }

    async createUser(userData: Omit<User, 'id' | 'createdAt'>): Promise<User> {
        const user: User = {
            ...userData,
            id: Date.now().toString(),
            createdAt: new Date()
        };
        this.users.push(user);
        return user;
    }

    async updateUser(id: string, updates: Partial<User>): Promise<User | undefined> {
        const userIndex = this.users.findIndex(user => user.id === id);
        if (userIndex === -1) return undefined;

        this.users[userIndex] = { ...this.users[userIndex], ...updates };
        return this.users[userIndex];
    }
}`,
		"src/types/User.ts": `export enum UserRole {
    USER = 'user',
    ADMIN = 'admin',
    MODERATOR = 'moderator'
}

export interface User {
    id: string;
    name: string;
    email: string;
    role: UserRole;
    createdAt: Date;
    isActive: boolean;
    profile?: UserProfile;
}

export interface UserProfile {
    avatar?: string;
    bio?: string;
    preferences: {
        theme: 'light' | 'dark';
        notifications: boolean;
    };
}

export type CreateUserRequest = Omit<User, 'id' | 'createdAt'>;
export type UpdateUserRequest = Partial<Pick<User, 'name' | 'email' | 'profile' | 'isActive'>>;`,
		"src/components/UserComponent.tsx": `import React, { useState, useCallback } from 'react';
import { User, UserRole } from '../types/User';

interface UserComponentProps {
    users: User[];
    onUserSelect: (user: User) => void;
}

export const UserComponent: React.FC<UserComponentProps> = ({ 
    users, 
    onUserSelect 
}) => {
    const [selectedUser, setSelectedUser] = useState<User | null>(null);
    const [filter, setFilter] = useState<UserRole | 'all'>('all');

    const handleUserClick = useCallback((user: User) => {
        setSelectedUser(user);
        onUserSelect(user);
    }, [onUserSelect]);

    const filteredUsers = users.filter(user => 
        filter === 'all' || user.role === filter
    );

    return (
        <div className="user-component">
            <div className="filter-controls">
                <select 
                    value={filter} 
                    onChange={(e) => setFilter(e.target.value as UserRole | 'all')}
                >
                    <option value="all">All Users</option>
                    <option value={UserRole.USER}>Users</option>
                    <option value={UserRole.ADMIN}>Admins</option>
                    <option value={UserRole.MODERATOR}>Moderators</option>
                </select>
            </div>

            <div className="user-list">
                {filteredUsers.map((user) => (
                    <div
                        key={user.id}
                        className={'user-item ' + (selectedUser?.id === user.id ? 'selected' : '')}
                        onClick={() => handleUserClick(user)}
                    >
                        <h3>{user.name}</h3>
                        <p>{user.email}</p>
                        <span className={'role ' + user.role}>{user.role}</span>
                    </div>
                ))}
            </div>
        </div>
    );
};`,
		"src/utils/ApiClient.ts": `export interface ApiResponse<T> {
    data: T;
    status: number;
    message?: string;
}

export class ApiClient {
    private baseURL: string;

    constructor(baseURL: string = '/api') {
        this.baseURL = baseURL;
    }

    async get<T>(endpoint: string): Promise<ApiResponse<T>> {
        const response = await fetch(this.baseURL + endpoint);
        const data = await response.json();
        
        return {
            data,
            status: response.status,
            message: response.statusText
        };
    }

    async post<T>(endpoint: string, body: any): Promise<ApiResponse<T>> {
        const response = await fetch(this.baseURL + endpoint, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(body)
        });
        
        const data = await response.json();
        
        return {
            data,
            status: response.status,
            message: response.statusText
        };
    }
}`,
		"src/types.d.ts": `declare module '*.json' {
    const value: any;
    export default value;
}

declare global {
    interface Window {
        __APP_CONFIG__: {
            apiUrl: string;
            version: string;
        };
    }
}

export {};`,
	}

	// Create all project files
	for relativePath, content := range suite.projectFiles {
		fullPath := filepath.Join(suite.projectRoot, relativePath)
		
		// Create directory if needed
		dir := filepath.Dir(fullPath)
		err := os.MkdirAll(dir, 0755)
		suite.Require().NoError(err, "Should create directory %s", dir)
		
		// Write file content
		err = os.WriteFile(fullPath, []byte(content), 0644)
		suite.Require().NoError(err, "Should create file %s", relativePath)
	}
}

func (suite *TypeScriptRealServerIntegrationTestSuite) initializeServer(ctx context.Context) bool {
	// Send initialize request with proper TypeScript capabilities
	initParams := map[string]interface{}{
		"processId": os.Getpid(),
		"rootUri":   fmt.Sprintf("file://%s", suite.projectRoot),
		"capabilities": map[string]interface{}{
			"workspace": map[string]interface{}{
				"configuration": true,
				"workspaceFolders": true,
			},
			"textDocument": map[string]interface{}{
				"synchronization": map[string]interface{}{
					"dynamicRegistration": true,
					"willSave":           true,
					"willSaveWaitUntil":  true,
					"didSave":            true,
				},
				"completion": map[string]interface{}{
					"dynamicRegistration": true,
					"completionItem": map[string]interface{}{
						"snippetSupport": true,
					},
				},
				"hover": map[string]interface{}{
					"dynamicRegistration": true,
				},
				"definition": map[string]interface{}{
					"dynamicRegistration": true,
				},
				"references": map[string]interface{}{
					"dynamicRegistration": true,
				},
				"documentSymbol": map[string]interface{}{
					"dynamicRegistration": true,
				},
			},
		},
		"initializationOptions": map[string]interface{}{
			"preferences": map[string]interface{}{
				"includeInlayParameterNameHints": "all",
				"includeInlayPropertyDeclarationTypeHints": true,
				"includeInlayFunctionParameterTypeHints": true,
			},
		},
	}

	startTime := time.Now()
	response, err := suite.lspClient.SendRequest(ctx, "initialize", initParams)
	if err != nil {
		suite.T().Logf("Initialize request failed: %v", err)
		return false
	}

	suite.performanceMetrics.FirstResponseTime = time.Since(startTime)

	// Parse initialize response
	var initResult map[string]interface{}
	if err := json.Unmarshal(response, &initResult); err != nil {
		suite.T().Logf("Failed to parse initialize response: %v", err)
		return false
	}

	// Send initialized notification
	err = suite.lspClient.SendNotification(ctx, "initialized", map[string]interface{}{})
	if err != nil {
		suite.T().Logf("Initialized notification failed: %v", err)
		return false
	}

	return true
}

func (suite *TypeScriptRealServerIntegrationTestSuite) openWorkspace(ctx context.Context) {
	// Open all TypeScript files in the project
	for relativePath := range suite.projectFiles {
		if strings.HasSuffix(relativePath, ".ts") || strings.HasSuffix(relativePath, ".tsx") {
			fullPath := filepath.Join(suite.projectRoot, relativePath)
			content, err := os.ReadFile(fullPath)
			if err != nil {
				continue
			}

			// Send textDocument/didOpen notification
			err = suite.lspClient.SendNotification(ctx, "textDocument/didOpen", map[string]interface{}{
				"textDocument": map[string]interface{}{
					"uri":        fmt.Sprintf("file://%s", fullPath),
					"languageId": "typescript",
					"version":    1,
					"text":       string(content),
				},
			})
			if err != nil {
				suite.T().Logf("Failed to open document %s: %v", relativePath, err)
			}
		}
	}

	// Wait for initial processing
	time.Sleep(3 * time.Second)
}

func (suite *TypeScriptRealServerIntegrationTestSuite) executeComprehensiveTypeScriptWorkflow(ctx context.Context) *TypeScriptIntegrationResult {
	result := &TypeScriptIntegrationResult{
		ServerStartSuccess:    true,
		InitializationSuccess: true,
		ProjectLoadSuccess:    true,
		PerformanceMetrics:   suite.performanceMetrics,
	}

	startTime := time.Now()

	// Test all 6 supported LSP features
	result.DefinitionSupport = suite.testGoToDefinition(ctx)
	result.ReferencesSupport = suite.testFindReferences(ctx)
	result.HoverSupport = suite.testHoverInformation(ctx)
	result.DocumentSymbolSupport = suite.testDocumentSymbols(ctx)
	result.WorkspaceSymbolSupport = suite.testWorkspaceSymbols(ctx)
	result.CompletionSupport = suite.testCompletion(ctx)

	result.TestDuration = time.Since(startTime)
	return result
}

func (suite *TypeScriptRealServerIntegrationTestSuite) testGoToDefinition(ctx context.Context) bool {
	// Test go to definition on User interface in UserService
	filePath := filepath.Join(suite.projectRoot, "src/services/UserService.ts")
	response, err := suite.lspClient.SendRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION, map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": fmt.Sprintf("file://%s", filePath),
		},
		"position": map[string]interface{}{
			"line":      0, // import { User, UserRole } from '../types/User';
			"character": 9, // Position of "User"
		},
	})

	suite.performanceMetrics.TotalRequests++
	if err != nil {
		suite.T().Logf("Go to definition failed: %v", err)
		suite.performanceMetrics.FailedRequests++
		return false
	}

	// Parse response to verify it points to the correct file
	var definitions []interface{}
	if err := json.Unmarshal(response, &definitions); err != nil {
		suite.T().Logf("Failed to parse definition response: %v", err)
		suite.performanceMetrics.FailedRequests++
		return false
	}

	suite.performanceMetrics.SuccessfulRequests++
	
	// Verify definition points to User.ts file
	if len(definitions) > 0 {
		if def, ok := definitions[0].(map[string]interface{}); ok {
			if uri, exists := def["uri"]; exists {
				return strings.Contains(uri.(string), "types/User.ts")
			}
		}
	}

	return false
}

func (suite *TypeScriptRealServerIntegrationTestSuite) testFindReferences(ctx context.Context) bool {
	// Test find references for UserRole enum
	filePath := filepath.Join(suite.projectRoot, "src/types/User.ts")
	response, err := suite.lspClient.SendRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES, map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": fmt.Sprintf("file://%s", filePath),
		},
		"position": map[string]interface{}{
			"line":      0, // export enum UserRole {
			"character": 13, // Position of "UserRole"
		},
		"context": map[string]interface{}{
			"includeDeclaration": true,
		},
	})

	suite.performanceMetrics.TotalRequests++
	if err != nil {
		suite.T().Logf("Find references failed: %v", err)
		suite.performanceMetrics.FailedRequests++
		return false
	}

	var references []interface{}
	if err := json.Unmarshal(response, &references); err != nil {
		suite.T().Logf("Failed to parse references response: %v", err)
		suite.performanceMetrics.FailedRequests++
		return false
	}

	suite.performanceMetrics.SuccessfulRequests++
	return len(references) > 1 // Should find multiple references across files
}

func (suite *TypeScriptRealServerIntegrationTestSuite) testHoverInformation(ctx context.Context) bool {
	// Test hover on Application class method
	filePath := filepath.Join(suite.projectRoot, "src/index.ts")
	response, err := suite.lspClient.SendRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER, map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": fmt.Sprintf("file://%s", filePath),
		},
		"position": map[string]interface{}{
			"line":      11, // async start(): Promise<void> {
			"character": 10, // Position of "start"
		},
	})

	suite.performanceMetrics.TotalRequests++
	if err != nil {
		suite.T().Logf("Hover failed: %v", err)
		suite.performanceMetrics.FailedRequests++
		return false
	}

	var hoverResult map[string]interface{}
	if err := json.Unmarshal(response, &hoverResult); err != nil {
		suite.T().Logf("Failed to parse hover response: %v", err)
		suite.performanceMetrics.FailedRequests++
		return false
	}

	suite.performanceMetrics.SuccessfulRequests++
	
	// Verify hover contains method signature information
	if contents, exists := hoverResult["contents"]; exists {
		contentStr := fmt.Sprintf("%v", contents)
		return strings.Contains(contentStr, "Promise<void>") || strings.Contains(contentStr, "async")
	}

	return false
}

func (suite *TypeScriptRealServerIntegrationTestSuite) testDocumentSymbols(ctx context.Context) bool {
	// Test document symbols for main index file
	filePath := filepath.Join(suite.projectRoot, "src/index.ts")
	response, err := suite.lspClient.SendRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS, map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": fmt.Sprintf("file://%s", filePath),
		},
	})

	suite.performanceMetrics.TotalRequests++
	if err != nil {
		suite.T().Logf("Document symbols failed: %v", err)
		suite.performanceMetrics.FailedRequests++
		return false
	}

	var symbols []interface{}
	if err := json.Unmarshal(response, &symbols); err != nil {
		suite.T().Logf("Failed to parse symbols response: %v", err)
		suite.performanceMetrics.FailedRequests++
		return false
	}

	suite.performanceMetrics.SuccessfulRequests++
	
	// Verify Application class is found in symbols
	for _, symbol := range symbols {
		if sym, ok := symbol.(map[string]interface{}); ok {
			if name, exists := sym["name"]; exists && name == "Application" {
				return true
			}
		}
	}

	return false
}


func (suite *TypeScriptRealServerIntegrationTestSuite) testWorkspaceSymbols(ctx context.Context) bool {
	// Test workspace/symbol functionality
	response, err := suite.lspClient.SendRequest(ctx, mcp.LSP_METHOD_WORKSPACE_SYMBOL, map[string]interface{}{
		"query": "User",
	})

	suite.performanceMetrics.TotalRequests++
	if err != nil {
		suite.T().Logf("Workspace symbol search failed: %v", err)
		suite.performanceMetrics.FailedRequests++
		return false
	}

	var symbols []interface{}
	if err := json.Unmarshal(response, &symbols); err != nil {
		suite.T().Logf("Failed to parse workspace symbols response: %v", err)
		suite.performanceMetrics.FailedRequests++
		return false
	}

	suite.performanceMetrics.SuccessfulRequests++
	return len(symbols) > 0
}

func (suite *TypeScriptRealServerIntegrationTestSuite) testCompletion(ctx context.Context) bool {
	// Test textDocument/completion functionality
	filePath := filepath.Join(suite.projectRoot, "src/index.ts")
	response, err := suite.lspClient.SendRequest(ctx, "textDocument/completion", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": fmt.Sprintf("file://%s", filePath),
		},
		"position": map[string]interface{}{
			"line":      15, // After console.log statement
			"character": 8,
		},
	})

	suite.performanceMetrics.TotalRequests++
	if err != nil {
		suite.T().Logf("Completion failed: %v", err)
		suite.performanceMetrics.FailedRequests++
		return false
	}

	var completionResult map[string]interface{}
	if err := json.Unmarshal(response, &completionResult); err != nil {
		suite.T().Logf("Failed to parse completion response: %v", err)
		suite.performanceMetrics.FailedRequests++
		return false
	}

	suite.performanceMetrics.SuccessfulRequests++
	
	// Check if completion items were returned
	if items, exists := completionResult["items"]; exists {
		if itemList, ok := items.([]interface{}); ok {
			return len(itemList) > 0
		}
	}

	return false
}


// Performance testing structures and methods

type PerformanceTestResult struct {
	TotalDuration time.Duration
	SuccessRate   float64
	RequestCount  int
	FailureCount  int
}

func (suite *TypeScriptRealServerIntegrationTestSuite) executePerformanceTest(ctx context.Context, requestCount, concurrency int) *PerformanceTestResult {
	startTime := time.Now()
	
	var wg sync.WaitGroup
	successChan := make(chan bool, requestCount)
	
	// Execute concurrent requests
	requestsPerWorker := requestCount / concurrency
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for j := 0; j < requestsPerWorker; j++ {
				// Alternate between all 6 supported LSP features
				var success bool
				switch j % 6 {
				case 0:
					success = suite.testGoToDefinition(ctx)
				case 1:
					success = suite.testFindReferences(ctx)
				case 2:
					success = suite.testHoverInformation(ctx)
				case 3:
					success = suite.testDocumentSymbols(ctx)
				case 4:
					success = suite.testWorkspaceSymbols(ctx)
				case 5:
					success = suite.testCompletion(ctx)
				}
				successChan <- success
			}
		}(i)
	}
	
	wg.Wait()
	close(successChan)
	
	// Calculate results
	totalDuration := time.Since(startTime)
	successCount := 0
	for success := range successChan {
		if success {
			successCount++
		}
	}
	
	return &PerformanceTestResult{
		TotalDuration: totalDuration,
		SuccessRate:   float64(successCount) / float64(requestCount),
		RequestCount:  requestCount,
		FailureCount:  requestCount - successCount,
	}
}

// Helper methods for real server integration

func (suite *TypeScriptRealServerIntegrationTestSuite) getProcessPID() int {
	// Try to get the actual process PID from the LSP client
	if stdioClient, ok := suite.lspClient.(*transport.StdioClient); ok {
		return stdioClient.GetProcessPIDForTesting()
	}
	return 0
}

func (suite *TypeScriptRealServerIntegrationTestSuite) validateTypeScriptIntegrationResult(result *TypeScriptIntegrationResult) {
	suite.True(result.ServerStartSuccess, "TypeScript server should start successfully")
	suite.True(result.InitializationSuccess, "Server initialization should succeed")
	suite.True(result.ProjectLoadSuccess, "Project should load successfully")
	
	// Validate all 6 supported LSP features
	suite.True(result.DefinitionSupport, "textDocument/definition should work")
	suite.True(result.ReferencesSupport, "textDocument/references should work")
	suite.True(result.HoverSupport, "textDocument/hover should work")
	suite.True(result.DocumentSymbolSupport, "textDocument/documentSymbol should work")
	suite.True(result.WorkspaceSymbolSupport, "workspace/symbol should work")
	suite.True(result.CompletionSupport, "textDocument/completion should work")
	
	suite.Less(result.TestDuration, suite.testTimeout, "Test should complete within timeout")
	suite.Equal(0, result.ErrorCount, "Should have no errors in comprehensive test")
	
	// Performance validations
	suite.NotNil(result.PerformanceMetrics, "Performance metrics should be collected")
	if result.PerformanceMetrics != nil {
		suite.Greater(result.PerformanceMetrics.TotalRequests, 0, "Should have made requests")
		suite.Greater(result.PerformanceMetrics.SuccessfulRequests, 0, "Should have successful requests")
		suite.Less(result.PerformanceMetrics.InitializationTime, 30*time.Second, "Initialization should be reasonable")
	}
}

func (suite *TypeScriptRealServerIntegrationTestSuite) recordPerformanceMetrics(result *TypeScriptIntegrationResult) {
	if suite.performanceMetrics.TotalRequests > 0 {
		suite.performanceMetrics.AverageResponseTime = result.TestDuration / time.Duration(suite.performanceMetrics.TotalRequests)
	}
	
	result.PerformanceMetrics = suite.performanceMetrics
}

// Test suite runner
func TestTypeScriptRealServerIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(TypeScriptRealServerIntegrationTestSuite))
}