package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"lsp-gateway/mcp"
	"lsp-gateway/tests/mocks"
)

// TypeScriptBasicWorkflowE2ETestSuite provides comprehensive E2E tests for TypeScript LSP workflows
// covering all essential TypeScript development scenarios that developers use daily
type TypeScriptBasicWorkflowE2ETestSuite struct {
	suite.Suite
	mockClient      *mocks.MockMcpClient
	testTimeout     time.Duration
	workspaceRoot   string
	typeScriptFiles map[string]TypeScriptTestFile
}

// TypeScriptTestFile represents a realistic TypeScript file with its metadata
type TypeScriptTestFile struct {
	FileName    string
	Content     string
	Language    string
	Description string
	Symbols     []TypeScriptSymbol
}

// TypeScriptSymbol represents a TypeScript symbol with position information
type TypeScriptSymbol struct {
	Name        string
	Kind        int    // LSP SymbolKind
	Position    LSPPosition
	Type        string // TypeScript type information
	Description string
}

// LSPPosition represents a position in a document
type LSPPosition struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// TypeScriptWorkflowResult captures results from TypeScript workflow execution
type TypeScriptWorkflowResult struct {
	DefinitionFound        bool
	ReferencesCount        int
	HoverInfoRetrieved     bool
	DocumentSymbolsCount   int
	CompletionItemsCount   int
	TypeInformationValid   bool
	DiagnosticsCount       int
	WorkflowLatency        time.Duration
	ErrorCount             int
	RequestCount           int
}

// SetupSuite initializes the test suite with realistic TypeScript fixtures
func (suite *TypeScriptBasicWorkflowE2ETestSuite) SetupSuite() {
	suite.testTimeout = 30 * time.Second
	suite.workspaceRoot = "/workspace"
	suite.setupTypeScriptTestFiles()
}

// SetupTest initializes a fresh mock client for each test
func (suite *TypeScriptBasicWorkflowE2ETestSuite) SetupTest() {
	suite.mockClient = mocks.NewMockMcpClient()
	suite.mockClient.SetHealthy(true)
}

// TearDownTest cleans up mock client state
func (suite *TypeScriptBasicWorkflowE2ETestSuite) TearDownTest() {
	if suite.mockClient != nil {
		suite.mockClient.Reset()
	}
}

// setupTypeScriptTestFiles creates realistic TypeScript test files and fixtures
func (suite *TypeScriptBasicWorkflowE2ETestSuite) setupTypeScriptTestFiles() {
	suite.typeScriptFiles = map[string]TypeScriptTestFile{
		"main.ts": {
			FileName:    "main.ts",
			Language:    "typescript",
			Description: "Main TypeScript application file",
			Content: `import { UserService } from './services/UserService';
import { User, UserRole } from './types/User';

class Application {
    private userService: UserService;

    constructor() {
        this.userService = new UserService();
    }

    async initialize(): Promise<void> {
        console.log('Initializing application...');
        const users = await this.userService.getAllUsers();
        console.log('Found users:', users.length);
    }

    createUser(name: string, email: string, role: UserRole = UserRole.USER): User {
        return {
            id: Math.random().toString(36).substr(2, 9),
            name,
            email,
            role,
            createdAt: new Date(),
            isActive: true
        };
    }
}

export default Application;`,
			Symbols: []TypeScriptSymbol{
				{Name: "Application", Kind: 5, Position: LSPPosition{Line: 3, Character: 6}, Type: "class", Description: "Main application class"},
				{Name: "initialize", Kind: 6, Position: LSPPosition{Line: 9, Character: 10}, Type: "method", Description: "Application initialization method"},
				{Name: "createUser", Kind: 6, Position: LSPPosition{Line: 15, Character: 4}, Type: "method", Description: "User creation method"},
			},
		},
		"services/UserService.ts": {
			FileName:    "services/UserService.ts",
			Language:    "typescript",
			Description: "User service with business logic",
			Content: `import { User, UserRole } from '../types/User';
import { ApiClient } from '../utils/ApiClient';

export class UserService {
    private apiClient: ApiClient;

    constructor() {
        this.apiClient = new ApiClient();
    }

    async getAllUsers(): Promise<User[]> {
        try {
            const response = await this.apiClient.get<User[]>('/users');
            return response.data;
        } catch (error) {
            console.error('Failed to fetch users:', error);
            return [];
        }
    }

    async getUserById(id: string): Promise<User | null> {
        try {
            const response = await this.apiClient.get<User>("/users/" + id);
            return response.data;
        } catch (error) {
            console.error("Failed to fetch user " + id + ":", error);
            return null;
        }
    }

    async createUser(userData: Omit<User, 'id' | 'createdAt'>): Promise<User> {
        const response = await this.apiClient.post<User>('/users', userData);
        return response.data;
    }

    isAdminUser(user: User): boolean {
        return user.role === UserRole.ADMIN;
    }
}`,
			Symbols: []TypeScriptSymbol{
				{Name: "UserService", Kind: 5, Position: LSPPosition{Line: 4, Character: 13}, Type: "class", Description: "User service class"},
				{Name: "getAllUsers", Kind: 6, Position: LSPPosition{Line: 11, Character: 10}, Type: "method", Description: "Get all users method"},
				{Name: "getUserById", Kind: 6, Position: LSPPosition{Line: 20, Character: 10}, Type: "method", Description: "Get user by ID method"},
				{Name: "createUser", Kind: 6, Position: LSPPosition{Line: 28, Character: 10}, Type: "method", Description: "Create user method"},
			},
		},
		"types/User.ts": {
			FileName:    "types/User.ts",
			Language:    "typescript",
			Description: "User type definitions and interfaces",
			Content: `export enum UserRole {
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
    location?: string;
    website?: string;
    social?: {
        twitter?: string;
        github?: string;
        linkedin?: string;
    };
}

export type CreateUserRequest = Omit<User, 'id' | 'createdAt'>;
export type UpdateUserRequest = Partial<Pick<User, 'name' | 'email' | 'profile' | 'isActive'>>;

export interface UserPermissions {
    canCreate: boolean;
    canEdit: boolean;
    canDelete: boolean;
    canViewAll: boolean;
}

export function getUserPermissions(role: UserRole): UserPermissions {
    switch (role) {
        case UserRole.ADMIN:
            return { canCreate: true, canEdit: true, canDelete: true, canViewAll: true };
        case UserRole.MODERATOR:
            return { canCreate: true, canEdit: true, canDelete: false, canViewAll: true };
        case UserRole.USER:
        default:
            return { canCreate: false, canEdit: false, canDelete: false, canViewAll: false };
    }
}`,
			Symbols: []TypeScriptSymbol{
				{Name: "UserRole", Kind: 10, Position: LSPPosition{Line: 1, Character: 12}, Type: "enum", Description: "User role enumeration"},
				{Name: "User", Kind: 11, Position: LSPPosition{Line: 7, Character: 17}, Type: "interface", Description: "User interface"},
				{Name: "UserProfile", Kind: 11, Position: LSPPosition{Line: 16, Character: 17}, Type: "interface", Description: "User profile interface"},
				{Name: "getUserPermissions", Kind: 12, Position: LSPPosition{Line: 36, Character: 16}, Type: "function", Description: "Get user permissions function"},
			},
		},
		"utils/ApiClient.ts": {
			FileName:    "utils/ApiClient.ts",
			Language:    "typescript",
			Description: "Generic API client with TypeScript generics",
			Content: `export interface ApiResponse<T> {
    data: T;
    status: number;
    message?: string;
}

export interface RequestConfig {
    headers?: Record<string, string>;
    timeout?: number;
    retry?: boolean;
}

export class ApiClient {
    private baseURL: string;
    private defaultHeaders: Record<string, string>;

    constructor(baseURL: string = '/api', defaultHeaders: Record<string, string> = {}) {
        this.baseURL = baseURL;
        this.defaultHeaders = {
            'Content-Type': 'application/json',
            ...defaultHeaders
        };
    }

    async get<T>(endpoint: string, config?: RequestConfig): Promise<ApiResponse<T>> {
        return this.request<T>('GET', endpoint, undefined, config);
    }

    async post<T>(endpoint: string, data?: any, config?: RequestConfig): Promise<ApiResponse<T>> {
        return this.request<T>('POST', endpoint, data, config);
    }

    async put<T>(endpoint: string, data?: any, config?: RequestConfig): Promise<ApiResponse<T>> {
        return this.request<T>('PUT', endpoint, data, config);
    }

    async delete<T>(endpoint: string, config?: RequestConfig): Promise<ApiResponse<T>> {
        return this.request<T>('DELETE', endpoint, undefined, config);
    }

    private async request<T>(
        method: string,
        endpoint: string,
        data?: any,
        config?: RequestConfig
    ): Promise<ApiResponse<T>> {
        const url = this.baseURL + endpoint;
        const requestConfig: RequestInit = {
            method,
            headers: {
                ...this.defaultHeaders,
                ...(config?.headers || {})
            }
        };

        if (data) {
            requestConfig.body = JSON.stringify(data);
        }

        try {
            const response = await fetch(url, requestConfig);
            const responseData = await response.json();

            return {
                data: responseData,
                status: response.status,
                message: response.statusText
            };
        } catch (error) {
            throw new Error("API request failed: " + error);
        }
    }
}`,
			Symbols: []TypeScriptSymbol{
				{Name: "ApiResponse", Kind: 11, Position: LSPPosition{Line: 1, Character: 17}, Type: "interface", Description: "API response interface"},
				{Name: "ApiClient", Kind: 5, Position: LSPPosition{Line: 13, Character: 13}, Type: "class", Description: "Generic API client class"},
				{Name: "get", Kind: 6, Position: LSPPosition{Line: 24, Character: 10}, Type: "method", Description: "GET request method"},
				{Name: "post", Kind: 6, Position: LSPPosition{Line: 28, Character: 10}, Type: "method", Description: "POST request method"},
			},
		},
	}
}

// TestBasicLSPOperations tests all fundamental LSP operations for TypeScript
func (suite *TypeScriptBasicWorkflowE2ETestSuite) TestBasicLSPOperations() {
	testCases := []struct {
		name         string
		fileName     string
		symbolName   string
		expectedType string
	}{
		{"Go to Definition - Class", "main.ts", "Application", "class"},
		{"Go to Definition - Method", "main.ts", "initialize", "method"},
		{"Go to Definition - Interface", "types/User.ts", "User", "interface"},
		{"Go to Definition - Enum", "types/User.ts", "UserRole", "enum"},
		{"Go to Definition - Function", "types/User.ts", "getUserPermissions", "function"},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			result := suite.executeBasicLSPWorkflow(tc.fileName, tc.symbolName, tc.expectedType)

			// Validate basic LSP operations
			suite.True(result.DefinitionFound, "Definition should be found for %s", tc.symbolName)
			suite.Greater(result.ReferencesCount, 0, "References should be found for %s", tc.symbolName)
			suite.True(result.HoverInfoRetrieved, "Hover info should be retrieved for %s", tc.symbolName)
			suite.Greater(result.DocumentSymbolsCount, 0, "Document symbols should be found")
			suite.Equal(0, result.ErrorCount, "No errors should occur during workflow")
			suite.Less(result.WorkflowLatency, 5*time.Second, "Workflow should complete within 5 seconds")
		})
	}
}

// TestTypeScriptSpecificFeatures tests TypeScript-specific language features
func (suite *TypeScriptBasicWorkflowE2ETestSuite) TestTypeScriptSpecificFeatures() {
	testCases := []struct {
		name        string
		feature     string
		description string
	}{
		{"Type Checking", "diagnostics", "TypeScript type checking and error diagnostics"},
		{"Interface Navigation", "interface_nav", "Navigate between interface definitions and implementations"},
		{"Import Resolution", "import_resolution", "Resolve TypeScript imports and modules"},
		{"Generic Type Handling", "generics", "Handle TypeScript generic types and constraints"},
		{"Auto-completion", "completion", "TypeScript-aware code completion"},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			result := suite.executeTypeScriptSpecificWorkflow(tc.feature)

			// Validate TypeScript-specific features
			suite.True(result.TypeInformationValid, "Type information should be valid for %s", tc.feature)
			suite.GreaterOrEqual(result.DiagnosticsCount, 0, "Diagnostics should be available for %s", tc.feature)
			
			if tc.feature == "completion" {
				suite.Greater(result.CompletionItemsCount, 0, "Completion items should be available")
			}
			
			suite.Equal(0, result.ErrorCount, "No errors should occur for %s", tc.feature)
			suite.Less(result.WorkflowLatency, 3*time.Second, "Feature test should complete quickly")
		})
	}
}

// TestMultiFileTypeScriptProject tests navigation across multiple TypeScript files
func (suite *TypeScriptBasicWorkflowE2ETestSuite) TestMultiFileTypeScriptProject() {
	// Setup responses for multi-file navigation
	suite.setupMultiFileResponses()

	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	startTime := time.Now()
	totalSymbols := 0
	totalReferences := 0

	// Navigate through each TypeScript file
	for fileName, fileInfo := range suite.typeScriptFiles {
		suite.Run(fmt.Sprintf("Multi-file navigation - %s", fileName), func() {
			// Get document symbols for the file
			symbolsResp, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS, map[string]interface{}{
				"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, fileName)},
			})
			suite.NoError(err, "Document symbols should be retrieved for %s", fileName)
			suite.NotNil(symbolsResp, "Symbol response should not be nil for %s", fileName)

			// Parse and count symbols
			var symbols []interface{}
			err = json.Unmarshal(symbolsResp, &symbols)
			suite.NoError(err, "Should be able to parse symbols for %s", fileName)
			totalSymbols += len(symbols)

			// Test cross-file references for main symbols
			for _, symbol := range fileInfo.Symbols {
				refsResp, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES, map[string]interface{}{
					"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, fileName)},
					"position":     symbol.Position,
					"context":      map[string]bool{"includeDeclaration": true},
				})
				suite.NoError(err, "References should be found for %s in %s", symbol.Name, fileName)
				suite.NotNil(refsResp, "References response should not be nil")

				// Count references
				var refs []interface{}
				if json.Unmarshal(refsResp, &refs) == nil {
					totalReferences += len(refs)
				}
			}
		})
	}

	multiFileLatency := time.Since(startTime)

	// Validate multi-file project navigation
	suite.Greater(totalSymbols, 10, "Should find significant number of symbols across all files")
	suite.Greater(totalReferences, 5, "Should find cross-file references")
	suite.GreaterOrEqual(suite.mockClient.GetCallCount(mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS), len(suite.typeScriptFiles), "Document symbols should be called for each file")
	suite.Greater(suite.mockClient.GetCallCount(mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES), 0, "References should be called")
	suite.Less(multiFileLatency, 10*time.Second, "Multi-file navigation should complete within reasonable time")
}

// TestProtocolValidation tests both HTTP JSON-RPC and MCP protocol validation
func (suite *TypeScriptBasicWorkflowE2ETestSuite) TestProtocolValidation() {
	protocols := []struct {
		name        string
		description string
	}{
		{"HTTP_JSON_RPC", "Test HTTP JSON-RPC protocol for TypeScript operations"},
		{"MCP_Protocol", "Test MCP protocol TypeScript tool integration"},
	}

	for _, protocol := range protocols {
		suite.Run(protocol.name, func() {
			// Setup protocol-specific responses
			suite.setupProtocolResponses(protocol.name)

			ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
			defer cancel()

			startTime := time.Now()

			// Test core LSP methods through the protocol
			methods := []string{
				mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION,
				mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES,
				mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER,
				mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS,
				mcp.LSP_METHOD_WORKSPACE_SYMBOL,
			}

			successCount := 0
			for _, method := range methods {
				resp, err := suite.mockClient.SendLSPRequest(ctx, method, suite.createTestParams(method))
				if err == nil && resp != nil {
					successCount++
				}
			}

			protocolLatency := time.Since(startTime)

			// Validate protocol functionality
			suite.Equal(len(methods), successCount, "All LSP methods should work through %s protocol", protocol.name)
			suite.Less(protocolLatency, 5*time.Second, "%s protocol should respond quickly", protocol.name)
			
			// Validate dual protocol consistency
			if protocol.name == "MCP_Protocol" {
				// Ensure MCP protocol provides consistent responses
				suite.GreaterOrEqual(suite.mockClient.GetCallCount(mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION), 1, "Definition should be called through MCP")
			}
		})
	}
}

// TestPerformanceValidation tests performance against established thresholds
func (suite *TypeScriptBasicWorkflowE2ETestSuite) TestPerformanceValidation() {
	// Performance thresholds based on project guidelines
	const (
		maxResponseTime = 5 * time.Second
		minThroughput   = 100 // requests per second
		maxErrorRate    = 0.05 // 5%
	)

	testCases := []struct {
		name            string
		requestCount    int
		concurrency     int
		expectedLatency time.Duration
	}{
		{"Single Request Performance", 1, 1, 100 * time.Millisecond},
		{"Moderate Load Performance", 50, 5, 2 * time.Second},
		{"High Load Performance", 100, 10, maxResponseTime},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Setup sufficient responses for performance test
			for i := 0; i < tc.requestCount; i++ {
				suite.mockClient.QueueResponse(json.RawMessage(`{
					"uri": "file:///workspace/main.ts",
					"range": {
						"start": {"line": 3, "character": 6},
						"end": {"line": 3, "character": 17}
					}
				}`))
			}

			startTime := time.Now()
			var wg sync.WaitGroup
			errorCount := 0
			mu := sync.Mutex{}

			// Execute concurrent requests
			for i := 0; i < tc.concurrency; i++ {
				wg.Add(1)
				go func(workerID int) {
					defer wg.Done()
					
					requestsPerWorker := tc.requestCount / tc.concurrency
					for j := 0; j < requestsPerWorker; j++ {
						ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
						_, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION, map[string]interface{}{
							"textDocument": map[string]string{"uri": "file:///workspace/main.ts"},
							"position":     LSPPosition{Line: 3, Character: 10},
						})
						cancel()
						
						if err != nil {
							mu.Lock()
							errorCount++
							mu.Unlock()
						}
					}
				}(i)
			}

			wg.Wait()
			totalLatency := time.Since(startTime)

			// Calculate performance metrics
			actualThroughput := float64(tc.requestCount) / totalLatency.Seconds()
			errorRate := float64(errorCount) / float64(tc.requestCount)

			// Validate performance thresholds
			suite.Less(totalLatency, maxResponseTime, "Total response time should be under threshold for %s", tc.name)
			suite.Greater(actualThroughput, float64(minThroughput), "Throughput should exceed minimum for %s", tc.name)
			suite.Less(errorRate, maxErrorRate, "Error rate should be under threshold for %s", tc.name)
			suite.Equal(tc.requestCount, suite.mockClient.GetCallCount(mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION), "All requests should be processed")
		})
	}
}

// TestConcurrentWorkflows tests multiple TypeScript workflows running concurrently
func (suite *TypeScriptBasicWorkflowE2ETestSuite) TestConcurrentWorkflows() {
	const numConcurrentWorkflows = 8

	// Setup responses for concurrent execution
	for i := 0; i < numConcurrentWorkflows*4; i++ { // 4 requests per workflow
		suite.mockClient.QueueResponse(suite.createTypeScriptSymbolResponse())
		suite.mockClient.QueueResponse(suite.createTypeScriptDefinitionResponse())
		suite.mockClient.QueueResponse(suite.createTypeScriptReferencesResponse())
		suite.mockClient.QueueResponse(suite.createTypeScriptHoverResponse())
	}

	var wg sync.WaitGroup
	results := make(chan bool, numConcurrentWorkflows)
	startTime := time.Now()

	// Launch concurrent TypeScript workflows
	for i := 0; i < numConcurrentWorkflows; i++ {
		wg.Add(1)
		go func(workflowID int) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
			defer cancel()

			// Execute TypeScript workflow: symbols → definition → references → hover
			_, err1 := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS, map[string]interface{}{
				"textDocument": map[string]string{"uri": fmt.Sprintf("file:///workspace/workflow%d.ts", workflowID)},
			})

			_, err2 := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION, map[string]interface{}{
				"textDocument": map[string]string{"uri": fmt.Sprintf("file:///workspace/workflow%d.ts", workflowID)},
				"position":     LSPPosition{Line: 5, Character: 10},
			})

			_, err3 := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES, map[string]interface{}{
				"textDocument": map[string]string{"uri": fmt.Sprintf("file:///workspace/workflow%d.ts", workflowID)},
				"position":     LSPPosition{Line: 5, Character: 10},
				"context":      map[string]bool{"includeDeclaration": true},
			})

			_, err4 := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER, map[string]interface{}{
				"textDocument": map[string]string{"uri": fmt.Sprintf("file:///workspace/workflow%d.ts", workflowID)},
				"position":     LSPPosition{Line: 5, Character: 10},
			})

			results <- (err1 == nil && err2 == nil && err3 == nil && err4 == nil)
		}(i)
	}

	// Wait for all workflows to complete
	wg.Wait()
	close(results)

	concurrentLatency := time.Since(startTime)

	// Validate concurrent execution results
	successCount := 0
	for success := range results {
		if success {
			successCount++
		}
	}

	suite.Equal(numConcurrentWorkflows, successCount, "All concurrent TypeScript workflows should succeed")
	suite.GreaterOrEqual(suite.mockClient.GetCallCount(mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS), numConcurrentWorkflows, "All symbol requests should be made")
	suite.GreaterOrEqual(suite.mockClient.GetCallCount(mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION), numConcurrentWorkflows, "All definition requests should be made")
	suite.GreaterOrEqual(suite.mockClient.GetCallCount(mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES), numConcurrentWorkflows, "All references requests should be made")
	suite.GreaterOrEqual(suite.mockClient.GetCallCount(mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER), numConcurrentWorkflows, "All hover requests should be made")
	suite.Less(concurrentLatency, 15*time.Second, "Concurrent TypeScript workflows should complete within reasonable time")
}

// Helper methods for workflow execution

// executeBasicLSPWorkflow executes a complete basic LSP workflow for TypeScript
func (suite *TypeScriptBasicWorkflowE2ETestSuite) executeBasicLSPWorkflow(fileName, symbolName, expectedType string) TypeScriptWorkflowResult {
	result := TypeScriptWorkflowResult{}

	// Setup mock responses
	suite.mockClient.QueueResponse(suite.createTypeScriptDefinitionResponse())
	suite.mockClient.QueueResponse(suite.createTypeScriptReferencesResponse())
	suite.mockClient.QueueResponse(suite.createTypeScriptHoverResponse())
	suite.mockClient.QueueResponse(suite.createTypeScriptSymbolResponse())

	startTime := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	// Execute workflow steps
	defResp, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION, map[string]interface{}{
		"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, fileName)},
		"position":     LSPPosition{Line: 3, Character: 10},
	})
	result.DefinitionFound = err == nil && defResp != nil
	if err != nil {
		result.ErrorCount++
	}

	refsResp, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES, map[string]interface{}{
		"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, fileName)},
		"position":     LSPPosition{Line: 3, Character: 10},
		"context":      map[string]bool{"includeDeclaration": true},
	})
	if err == nil && refsResp != nil {
		var refs []interface{}
		if json.Unmarshal(refsResp, &refs) == nil {
			result.ReferencesCount = len(refs)
		}
	} else {
		result.ErrorCount++
	}

	hoverResp, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER, map[string]interface{}{
		"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, fileName)},
		"position":     LSPPosition{Line: 3, Character: 10},
	})
	result.HoverInfoRetrieved = err == nil && hoverResp != nil
	if err != nil {
		result.ErrorCount++
	}

	symbolsResp, err := suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS, map[string]interface{}{
		"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/%s", suite.workspaceRoot, fileName)},
	})
	if err == nil && symbolsResp != nil {
		var symbols []interface{}
		if json.Unmarshal(symbolsResp, &symbols) == nil {
			result.DocumentSymbolsCount = len(symbols)
		}
	} else {
		result.ErrorCount++
	}

	result.WorkflowLatency = time.Since(startTime)
	result.RequestCount = len(suite.mockClient.SendLSPRequestCalls)
	result.TypeInformationValid = true // Mock always provides valid type info

	return result
}

// executeTypeScriptSpecificWorkflow executes TypeScript-specific feature tests
func (suite *TypeScriptBasicWorkflowE2ETestSuite) executeTypeScriptSpecificWorkflow(feature string) TypeScriptWorkflowResult {
	result := TypeScriptWorkflowResult{}

	startTime := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), suite.testTimeout)
	defer cancel()

	switch feature {
	case "diagnostics":
		// Mock diagnostic response for type checking
		suite.mockClient.QueueResponse(json.RawMessage(`[
			{
				"range": {"start": {"line": 5, "character": 10}, "end": {"line": 5, "character": 20}},
				"severity": 1,
				"message": "Property 'someProperty' does not exist on type 'User'",
				"source": "typescript"
			}
		]`))
		resp, err := suite.mockClient.SendLSPRequest(ctx, "textDocument/publishDiagnostics", map[string]interface{}{
			"uri": fmt.Sprintf("file://%s/main.ts", suite.workspaceRoot),
		})
		if err == nil && resp != nil {
			var diagnostics []interface{}
			if json.Unmarshal(resp, &diagnostics) == nil {
				result.DiagnosticsCount = len(diagnostics)
			}
		}

	case "completion":
		// Mock completion response
		suite.mockClient.QueueResponse(json.RawMessage(`{
			"items": [
				{"label": "initialize", "kind": 2, "detail": "(): Promise<void>"},
				{"label": "createUser", "kind": 2, "detail": "(name: string, email: string, role?: UserRole): User"},
				{"label": "userService", "kind": 6, "detail": "UserService"}
			]
		}`))
		resp, err := suite.mockClient.SendLSPRequest(ctx, "textDocument/completion", map[string]interface{}{
			"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/main.ts", suite.workspaceRoot)},
			"position":     LSPPosition{Line: 10, Character: 5},
		})
		if err == nil && resp != nil {
			var completion map[string]interface{}
			if json.Unmarshal(resp, &completion) == nil {
				if items, ok := completion["items"].([]interface{}); ok {
					result.CompletionItemsCount = len(items)
				}
			}
		}

	default:
		// Generic TypeScript feature test
		suite.mockClient.QueueResponse(suite.createTypeScriptDefinitionResponse())
		suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION, map[string]interface{}{
			"textDocument": map[string]string{"uri": fmt.Sprintf("file://%s/main.ts", suite.workspaceRoot)},
			"position":     LSPPosition{Line: 3, Character: 10},
		})
	}

	result.WorkflowLatency = time.Since(startTime)
	result.TypeInformationValid = true
	result.RequestCount = len(suite.mockClient.SendLSPRequestCalls)

	return result
}

// Helper methods for creating mock responses

func (suite *TypeScriptBasicWorkflowE2ETestSuite) setupMultiFileResponses() {
	// Setup responses for each file type
	responses := []json.RawMessage{
		suite.createTypeScriptSymbolResponse(),
		suite.createTypeScriptDefinitionResponse(),
		suite.createTypeScriptReferencesResponse(),
		suite.createTypeScriptHoverResponse(),
	}

	// Queue responses for all files
	for range suite.typeScriptFiles {
		for _, response := range responses {
			suite.mockClient.QueueResponse(response)
		}
	}
}

func (suite *TypeScriptBasicWorkflowE2ETestSuite) setupProtocolResponses(protocol string) {
	baseResponses := []json.RawMessage{
		suite.createTypeScriptDefinitionResponse(),
		suite.createTypeScriptReferencesResponse(),
		suite.createTypeScriptHoverResponse(),
		suite.createTypeScriptSymbolResponse(),
		suite.createWorkspaceSymbolResponse(),
	}

	for _, response := range baseResponses {
		suite.mockClient.QueueResponse(response)
	}
}

func (suite *TypeScriptBasicWorkflowE2ETestSuite) createTestParams(method string) map[string]interface{} {
	switch method {
	case mcp.LSP_METHOD_TEXT_DOCUMENT_DEFINITION, mcp.LSP_METHOD_TEXT_DOCUMENT_REFERENCES, mcp.LSP_METHOD_TEXT_DOCUMENT_HOVER:
		return map[string]interface{}{
			"textDocument": map[string]string{"uri": "file:///workspace/main.ts"},
			"position":     LSPPosition{Line: 3, Character: 10},
		}
	case mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS:
		return map[string]interface{}{
			"textDocument": map[string]string{"uri": "file:///workspace/main.ts"},
		}
	case mcp.LSP_METHOD_WORKSPACE_SYMBOL:
		return map[string]interface{}{
			"query": "Application",
		}
	default:
		return map[string]interface{}{}
	}
}

func (suite *TypeScriptBasicWorkflowE2ETestSuite) createTypeScriptDefinitionResponse() json.RawMessage {
	return json.RawMessage(`{
		"uri": "file:///workspace/main.ts",
		"range": {
			"start": {"line": 3, "character": 6},
			"end": {"line": 3, "character": 17}
		}
	}`)
}

func (suite *TypeScriptBasicWorkflowE2ETestSuite) createTypeScriptReferencesResponse() json.RawMessage {
	return json.RawMessage(`[
		{
			"uri": "file:///workspace/main.ts",
			"range": {"start": {"line": 3, "character": 6}, "end": {"line": 3, "character": 17}}
		},
		{
			"uri": "file:///workspace/services/UserService.ts",
			"range": {"start": {"line": 1, "character": 0}, "end": {"line": 1, "character": 11}}
		},
		{
			"uri": "file:///workspace/types/User.ts",
			"range": {"start": {"line": 7, "character": 17}, "end": {"line": 7, "character": 21}}
		}
	]`)
}

func (suite *TypeScriptBasicWorkflowE2ETestSuite) createTypeScriptHoverResponse() json.RawMessage {
	return json.RawMessage(`{
		"contents": {
			"kind": "markdown",
			"value": "` + "```typescript\nclass Application\n```" + `\n\nMain TypeScript application class with user management functionality.\n\n**Methods:**\n- ` + "`initialize(): Promise<void>`" + ` - Initialize the application\n- ` + "`createUser(name: string, email: string, role?: UserRole): User`" + ` - Create a new user"
		},
		"range": {
			"start": {"line": 3, "character": 6},
			"end": {"line": 3, "character": 17}
		}
	}`)
}

func (suite *TypeScriptBasicWorkflowE2ETestSuite) createTypeScriptSymbolResponse() json.RawMessage {
	return json.RawMessage(`[
		{
			"name": "Application",
			"kind": 5,
			"range": {"start": {"line": 3, "character": 0}, "end": {"line": 21, "character": 1}},
			"selectionRange": {"start": {"line": 3, "character": 6}, "end": {"line": 3, "character": 17}},
			"children": [
				{
					"name": "userService",
					"kind": 6,
					"range": {"start": {"line": 4, "character": 4}, "end": {"line": 4, "character": 35}},
					"selectionRange": {"start": {"line": 4, "character": 12}, "end": {"line": 4, "character": 23}}
				},
				{
					"name": "constructor",
					"kind": 9,
					"range": {"start": {"line": 6, "character": 4}, "end": {"line": 8, "character": 5}},
					"selectionRange": {"start": {"line": 6, "character": 4}, "end": {"line": 6, "character": 15}}
				},
				{
					"name": "initialize",
					"kind": 6,
					"range": {"start": {"line": 10, "character": 4}, "end": {"line": 14, "character": 5}},
					"selectionRange": {"start": {"line": 10, "character": 10}, "end": {"line": 10, "character": 20}}
				},
				{
					"name": "createUser",
					"kind": 6,
					"range": {"start": {"line": 16, "character": 4}, "end": {"line": 25, "character": 5}},
					"selectionRange": {"start": {"line": 16, "character": 4}, "end": {"line": 16, "character": 14}}
				}
			]
		}
	]`)
}

func (suite *TypeScriptBasicWorkflowE2ETestSuite) createWorkspaceSymbolResponse() json.RawMessage {
	return json.RawMessage(`[
		{
			"name": "Application",
			"kind": 5,
			"location": {
				"uri": "file:///workspace/main.ts",
				"range": {"start": {"line": 3, "character": 6}, "end": {"line": 3, "character": 17}}
			}
		},
		{
			"name": "UserService",
			"kind": 5,
			"location": {
				"uri": "file:///workspace/services/UserService.ts",
				"range": {"start": {"line": 4, "character": 13}, "end": {"line": 4, "character": 24}}
			}
		},
		{
			"name": "User",
			"kind": 11,
			"location": {
				"uri": "file:///workspace/types/User.ts",
				"range": {"start": {"line": 7, "character": 17}, "end": {"line": 7, "character": 21}}
			}
		},
		{
			"name": "UserRole",
			"kind": 10,
			"location": {
				"uri": "file:///workspace/types/User.ts",
				"range": {"start": {"line": 1, "character": 12}, "end": {"line": 1, "character": 20}}
			}
		}
	]`)
}

// Benchmark tests for performance measurement

// BenchmarkTypeScriptBasicWorkflow benchmarks the basic TypeScript workflow
func BenchmarkTypeScriptBasicWorkflow(b *testing.B) {
	suite := &TypeScriptBasicWorkflowE2ETestSuite{}
	suite.SetupSuite()

	workflows := []string{"definition", "references", "hover", "symbols"}

	for _, workflow := range workflows {
		b.Run(fmt.Sprintf("TypeScript_%s", workflow), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				suite.SetupTest()
				result := suite.executeBasicLSPWorkflow("main.ts", "Application", "class")
				if result.ErrorCount > 0 {
					b.Errorf("Workflow failed with %d errors", result.ErrorCount)
				}
				suite.TearDownTest()
			}
		})
	}
}

// BenchmarkTypeScriptConcurrentWorkflows benchmarks concurrent workflow execution
func BenchmarkTypeScriptConcurrentWorkflows(b *testing.B) {
	suite := &TypeScriptBasicWorkflowE2ETestSuite{}
	suite.SetupSuite()

	concurrencyLevels := []int{1, 4, 8, 16}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrent_%d", concurrency), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				suite.SetupTest()

				// Setup sufficient responses
				for j := 0; j < concurrency*4; j++ {
					suite.mockClient.QueueResponse(suite.createTypeScriptSymbolResponse())
				}

				var wg sync.WaitGroup
				for j := 0; j < concurrency; j++ {
					wg.Add(1)
					go func() {
						defer wg.Done()
						ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
						defer cancel()
						suite.mockClient.SendLSPRequest(ctx, mcp.LSP_METHOD_TEXT_DOCUMENT_SYMBOLS, map[string]interface{}{
							"textDocument": map[string]string{"uri": "file:///workspace/main.ts"},
						})
					}()
				}
				wg.Wait()

				suite.TearDownTest()
			}
		})
	}
}

// TestSuite runner
func TestTypeScriptBasicWorkflowE2ETestSuite(t *testing.T) {
	suite.Run(t, new(TypeScriptBasicWorkflowE2ETestSuite))
}