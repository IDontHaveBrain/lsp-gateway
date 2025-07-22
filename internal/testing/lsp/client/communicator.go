package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"

	"lsp-gateway/internal/testing/lsp/cases"
)

// LSPCommunicator handles LSP communication for test cases
type LSPCommunicator struct {
	serverManager *LSPServerManager
}

// NewLSPCommunicator creates a new LSP communicator
func NewLSPCommunicator(serverManager *LSPServerManager) *LSPCommunicator {
	return &LSPCommunicator{
		serverManager: serverManager,
	}
}

// ExecuteTestCase executes a single LSP test case
func (c *LSPCommunicator) ExecuteTestCase(ctx context.Context, testCase *cases.TestCase) error {
	// Get the appropriate server for this test case
	serverName := c.getServerNameForLanguage(testCase.Language)
	if serverName == "" {
		return fmt.Errorf("no server configured for language: %s", testCase.Language)
	}

	// Create workspace URI
	workspaceURI, err := c.createWorkspaceURI(testCase.Workspace)
	if err != nil {
		return fmt.Errorf("failed to create workspace URI: %w", err)
	}

	// Get or create server
	server, err := c.serverManager.GetOrCreateServer(ctx, serverName, workspaceURI)
	if err != nil {
		return fmt.Errorf("failed to get server: %w", err)
	}

	// Check server capabilities
	if !c.isMethodSupported(server, testCase.Method) {
		testCase.Status = cases.TestStatusSkipped
		testCase.Error = fmt.Errorf("method %s not supported by server %s", testCase.Method, serverName)
		return nil
	}

	// Execute the specific LSP method
	response, err := c.executeMethod(ctx, server, testCase)
	if err != nil {
		testCase.Status = cases.TestStatusError
		testCase.Error = err
		return err
	}

	testCase.Response = response
	testCase.Status = cases.TestStatusPassed
	return nil
}

// getServerNameForLanguage returns the server name for a given language
func (c *LSPCommunicator) getServerNameForLanguage(language string) string {
	for name, serverConfig := range c.serverManager.config.Servers {
		if serverConfig.Language == language {
			return name
		}
	}
	return ""
}

// createWorkspaceURI creates a file URI from a workspace path
func (c *LSPCommunicator) createWorkspaceURI(workspacePath string) (string, error) {
	absPath, err := filepath.Abs(workspacePath)
	if err != nil {
		return "", err
	}

	// Convert to file URI
	uri := url.URL{
		Scheme: "file",
		Path:   absPath,
	}

	return uri.String(), nil
}

// isMethodSupported checks if the server supports the given method
func (c *LSPCommunicator) isMethodSupported(server *ManagedLSPServer, method string) bool {
	capabilities := server.GetCapabilities()
	if capabilities == nil {
		return false
	}

	switch method {
	case cases.LSPMethodDefinition:
		return capabilities.DefinitionProvider
	case cases.LSPMethodReferences:
		return capabilities.ReferencesProvider
	case cases.LSPMethodHover:
		return capabilities.HoverProvider
	case cases.LSPMethodDocumentSymbol:
		return capabilities.DocumentSymbolProvider
	case cases.LSPMethodWorkspaceSymbol:
		return capabilities.WorkspaceSymbolProvider
	default:
		return false
	}
}

// executeMethod executes the specific LSP method based on the test case
func (c *LSPCommunicator) executeMethod(ctx context.Context, server *ManagedLSPServer, testCase *cases.TestCase) (json.RawMessage, error) {
	switch testCase.Method {
	case cases.LSPMethodDefinition:
		return c.executeDefinition(ctx, server, testCase)
	case cases.LSPMethodReferences:
		return c.executeReferences(ctx, server, testCase)
	case cases.LSPMethodHover:
		return c.executeHover(ctx, server, testCase)
	case cases.LSPMethodDocumentSymbol:
		return c.executeDocumentSymbol(ctx, server, testCase)
	case cases.LSPMethodWorkspaceSymbol:
		return c.executeWorkspaceSymbol(ctx, server, testCase)
	default:
		return nil, fmt.Errorf("unsupported method: %s", testCase.Method)
	}
}

// executeDefinition executes textDocument/definition request
func (c *LSPCommunicator) executeDefinition(ctx context.Context, server *ManagedLSPServer, testCase *cases.TestCase) (json.RawMessage, error) {
	fileURI, err := c.createFileURI(testCase.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file URI: %w", err)
	}

	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": fileURI,
		},
		"position": map[string]interface{}{
			"line":      testCase.Position.Line,
			"character": testCase.Position.Character,
		},
	}

	// Add any additional parameters from test case
	for key, value := range testCase.Params {
		params[key] = value
	}

	return server.SendRequest(ctx, cases.LSPMethodDefinition, params)
}

// executeReferences executes textDocument/references request
func (c *LSPCommunicator) executeReferences(ctx context.Context, server *ManagedLSPServer, testCase *cases.TestCase) (json.RawMessage, error) {
	fileURI, err := c.createFileURI(testCase.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file URI: %w", err)
	}

	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": fileURI,
		},
		"position": map[string]interface{}{
			"line":      testCase.Position.Line,
			"character": testCase.Position.Character,
		},
		"context": map[string]interface{}{
			"includeDeclaration": true, // Default to including declaration
		},
	}

	// Override with test case parameters
	for key, value := range testCase.Params {
		if key == "includeDeclaration" {
			params["context"].(map[string]interface{})["includeDeclaration"] = value
		} else {
			params[key] = value
		}
	}

	return server.SendRequest(ctx, cases.LSPMethodReferences, params)
}

// executeHover executes textDocument/hover request
func (c *LSPCommunicator) executeHover(ctx context.Context, server *ManagedLSPServer, testCase *cases.TestCase) (json.RawMessage, error) {
	fileURI, err := c.createFileURI(testCase.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file URI: %w", err)
	}

	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": fileURI,
		},
		"position": map[string]interface{}{
			"line":      testCase.Position.Line,
			"character": testCase.Position.Character,
		},
	}

	// Add any additional parameters from test case
	for key, value := range testCase.Params {
		params[key] = value
	}

	return server.SendRequest(ctx, cases.LSPMethodHover, params)
}

// executeDocumentSymbol executes textDocument/documentSymbol request
func (c *LSPCommunicator) executeDocumentSymbol(ctx context.Context, server *ManagedLSPServer, testCase *cases.TestCase) (json.RawMessage, error) {
	fileURI, err := c.createFileURI(testCase.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file URI: %w", err)
	}

	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": fileURI,
		},
	}

	// Add any additional parameters from test case
	for key, value := range testCase.Params {
		params[key] = value
	}

	return server.SendRequest(ctx, cases.LSPMethodDocumentSymbol, params)
}

// executeWorkspaceSymbol executes workspace/symbol request
func (c *LSPCommunicator) executeWorkspaceSymbol(ctx context.Context, server *ManagedLSPServer, testCase *cases.TestCase) (json.RawMessage, error) {
	params := map[string]interface{}{
		"query": "", // Default empty query
	}

	// Override with test case parameters
	for key, value := range testCase.Params {
		params[key] = value
	}

	// Ensure query parameter exists
	if _, exists := params["query"]; !exists {
		return nil, fmt.Errorf("workspace/symbol requires a 'query' parameter")
	}

	return server.SendRequest(ctx, cases.LSPMethodWorkspaceSymbol, params)
}

// createFileURI creates a file URI from a file path
func (c *LSPCommunicator) createFileURI(filePath string) (string, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return "", err
	}

	// Convert to file URI
	uri := url.URL{
		Scheme: "file",
		Path:   absPath,
	}

	return uri.String(), nil
}

// OpenDocument sends textDocument/didOpen notification
func (c *LSPCommunicator) OpenDocument(ctx context.Context, server *ManagedLSPServer, filePath string, content string, languageId string) error {
	fileURI, err := c.createFileURI(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file URI: %w", err)
	}

	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        fileURI,
			"languageId": languageId,
			"version":    1,
			"text":       content,
		},
	}

	return server.SendNotification(ctx, "textDocument/didOpen", params)
}

// CloseDocument sends textDocument/didClose notification
func (c *LSPCommunicator) CloseDocument(ctx context.Context, server *ManagedLSPServer, filePath string) error {
	fileURI, err := c.createFileURI(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file URI: %w", err)
	}

	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": fileURI,
		},
	}

	return server.SendNotification(ctx, "textDocument/didClose", params)
}

// ChangeDocument sends textDocument/didChange notification
func (c *LSPCommunicator) ChangeDocument(ctx context.Context, server *ManagedLSPServer, filePath string, content string, version int) error {
	fileURI, err := c.createFileURI(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file URI: %w", err)
	}

	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":     fileURI,
			"version": version,
		},
		"contentChanges": []map[string]interface{}{
			{
				"text": content,
			},
		},
	}

	return server.SendNotification(ctx, "textDocument/didChange", params)
}

// ExecuteTestCaseWithDocumentLifecycle executes a test case with proper document lifecycle
func (c *LSPCommunicator) ExecuteTestCaseWithDocumentLifecycle(ctx context.Context, testCase *cases.TestCase, fileContent string) error {
	// Get the appropriate server for this test case
	serverName := c.getServerNameForLanguage(testCase.Language)
	if serverName == "" {
		return fmt.Errorf("no server configured for language: %s", testCase.Language)
	}

	// Create workspace URI
	workspaceURI, err := c.createWorkspaceURI(testCase.Workspace)
	if err != nil {
		return fmt.Errorf("failed to create workspace URI: %w", err)
	}

	// Get or create server
	server, err := c.serverManager.GetOrCreateServer(ctx, serverName, workspaceURI)
	if err != nil {
		return fmt.Errorf("failed to get server: %w", err)
	}

	// Open document
	if err := c.OpenDocument(ctx, server, testCase.FilePath, fileContent, testCase.Language); err != nil {
		return fmt.Errorf("failed to open document: %w", err)
	}

	defer func() {
		// Close document on exit
		_ = c.CloseDocument(ctx, server, testCase.FilePath)
	}()

	// Execute the test case
	return c.ExecuteTestCase(ctx, testCase)
}
