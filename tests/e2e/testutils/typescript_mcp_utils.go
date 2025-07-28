package testutils

import (
	"bufio"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// TypeScriptMCPTestHelper provides TypeScript-specific MCP testing utilities
type TypeScriptMCPTestHelper struct {
	stdin        io.WriteCloser
	reader       *bufio.Reader
	workspaceDir string
	tsFiles      []string
	repoManager  RepositoryManager
	lastID       int
}

// TypeScriptMCPRequest represents TypeScript-specific MCP request parameters
type TypeScriptMCPRequest struct {
	Method       string
	URI          string
	Position     LSPPosition
	Query        string
	Context      map[string]interface{}
	CustomParams map[string]interface{}
}

// TypeScriptMCPResponse represents parsed TypeScript MCP response data
type TypeScriptMCPResponse struct {
	MCPMessage *MCPMessage
	Locations  []TypeScriptLocation
	Symbols    []TypeScriptSymbol
	HoverInfo  *TypeScriptHover
	Error      *MCPError
}

// TypeScriptLocation represents a TypeScript file location
type TypeScriptLocation struct {
	URI   string      `json:"uri"`
	Range LSPRange    `json:"range"`
}

// TypeScriptSymbol represents a TypeScript symbol
type TypeScriptSymbol struct {
	Name         string      `json:"name"`
	Kind         int         `json:"kind"`
	Location     TypeScriptLocation `json:"location"`
	ContainerName string     `json:"containerName,omitempty"`
}

// TypeScriptHover represents TypeScript hover information
type TypeScriptHover struct {
	Contents []string `json:"contents"`
	Range    *LSPRange `json:"range,omitempty"`
}

// LSPRange represents an LSP range
type LSPRange struct {
	Start LSPPosition `json:"start"`
	End   LSPPosition `json:"end"`
}

// MCPError represents an MCP error response
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// NewTypeScriptMCPTestHelper creates a new TypeScript MCP test helper
func NewTypeScriptMCPTestHelper(stdin io.WriteCloser, reader *bufio.Reader, repoManager RepositoryManager) *TypeScriptMCPTestHelper {
	return &TypeScriptMCPTestHelper{
		stdin:       stdin,
		reader:      reader,
		repoManager: repoManager,
		workspaceDir: repoManager.GetWorkspaceDir(),
		lastID:      0,
	}
}

// DiscoverTypeScriptFiles discovers and filters TypeScript files from the repository
func (helper *TypeScriptMCPTestHelper) DiscoverTypeScriptFiles() error {
	testFiles, err := helper.repoManager.GetTestFiles()
	if err != nil {
		return fmt.Errorf("failed to get test files: %w", err)
	}

	helper.tsFiles = make([]string, 0)
	for _, file := range testFiles {
		ext := filepath.Ext(file)
		if ext == ".ts" || ext == ".tsx" || ext == ".js" || ext == ".jsx" {
			helper.tsFiles = append(helper.tsFiles, file)
		}
	}

	if len(helper.tsFiles) == 0 {
		return fmt.Errorf("no TypeScript files found in repository")
	}

	return nil
}

// GetTypeScriptFiles returns the discovered TypeScript files
func (helper *TypeScriptMCPTestHelper) GetTypeScriptFiles() []string {
	return helper.tsFiles
}

// GetFileURI converts a relative file path to a file URI
func (helper *TypeScriptMCPTestHelper) GetFileURI(filePath string) string {
	return "file://" + filepath.Join(helper.workspaceDir, filePath)
}

// nextID generates the next request ID
func (helper *TypeScriptMCPTestHelper) nextID() int {
	helper.lastID++
	return helper.lastID
}

// SendInitializeRequest sends an MCP initialize request
func (helper *TypeScriptMCPTestHelper) SendInitializeRequest() (*TypeScriptMCPResponse, error) {
	request := MCPMessage{
		Jsonrpc: "2.0",
		ID:      helper.nextID(),
		Method:  "initialize",
		Params: map[string]interface{}{
			"capabilities": map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "lspg-typescript-mcp-test",
				"version": "1.0.0",
			},
		},
	}

	response, err := SendMCPStdioMessage(helper.stdin, helper.reader, request)
	if err != nil {
		return nil, fmt.Errorf("initialize request failed: %w", err)
	}

	return helper.parseResponse(response)
}

// SendDefinitionRequest sends a textDocument/definition request for TypeScript
func (helper *TypeScriptMCPTestHelper) SendDefinitionRequest(req TypeScriptMCPRequest) (*TypeScriptMCPResponse, error) {
	request := MCPMessage{
		Jsonrpc: "2.0",
		ID:      helper.nextID(),
		Method:  "textDocument/definition",
		Params: map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": req.URI,
			},
			"position": map[string]interface{}{
				"line":      req.Position.Line,
				"character": req.Position.Character,
			},
		},
	}

	// Add custom parameters if provided
	if req.CustomParams != nil {
		params := request.Params.(map[string]interface{})
		for key, value := range req.CustomParams {
			params[key] = value
		}
	}

	response, err := SendMCPStdioMessage(helper.stdin, helper.reader, request)
	if err != nil {
		return nil, fmt.Errorf("definition request failed: %w", err)
	}

	return helper.parseResponse(response)
}

// SendReferencesRequest sends a textDocument/references request for TypeScript
func (helper *TypeScriptMCPTestHelper) SendReferencesRequest(req TypeScriptMCPRequest) (*TypeScriptMCPResponse, error) {
	request := MCPMessage{
		Jsonrpc: "2.0",
		ID:      helper.nextID(),
		Method:  "textDocument/references",
		Params: map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": req.URI,
			},
			"position": map[string]interface{}{
				"line":      req.Position.Line,
				"character": req.Position.Character,
			},
			"context": map[string]interface{}{
				"includeDeclaration": true,
			},
		},
	}

	// Override context if provided
	if req.Context != nil {
		params := request.Params.(map[string]interface{})
		params["context"] = req.Context
	}

	// Add custom parameters if provided
	if req.CustomParams != nil {
		params := request.Params.(map[string]interface{})
		for key, value := range req.CustomParams {
			params[key] = value
		}
	}

	response, err := SendMCPStdioMessage(helper.stdin, helper.reader, request)
	if err != nil {
		return nil, fmt.Errorf("references request failed: %w", err)
	}

	return helper.parseResponse(response)
}

// SendHoverRequest sends a textDocument/hover request for TypeScript
func (helper *TypeScriptMCPTestHelper) SendHoverRequest(req TypeScriptMCPRequest) (*TypeScriptMCPResponse, error) {
	request := MCPMessage{
		Jsonrpc: "2.0",
		ID:      helper.nextID(),
		Method:  "textDocument/hover",
		Params: map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": req.URI,
			},
			"position": map[string]interface{}{
				"line":      req.Position.Line,
				"character": req.Position.Character,
			},
		},
	}

	// Add custom parameters if provided
	if req.CustomParams != nil {
		params := request.Params.(map[string]interface{})
		for key, value := range req.CustomParams {
			params[key] = value
		}
	}

	response, err := SendMCPStdioMessage(helper.stdin, helper.reader, request)
	if err != nil {
		return nil, fmt.Errorf("hover request failed: %w", err)
	}

	return helper.parseResponse(response)
}

// SendDocumentSymbolRequest sends a textDocument/documentSymbol request for TypeScript
func (helper *TypeScriptMCPTestHelper) SendDocumentSymbolRequest(req TypeScriptMCPRequest) (*TypeScriptMCPResponse, error) {
	request := MCPMessage{
		Jsonrpc: "2.0",
		ID:      helper.nextID(),
		Method:  "textDocument/documentSymbol",
		Params: map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": req.URI,
			},
		},
	}

	// Add custom parameters if provided
	if req.CustomParams != nil {
		params := request.Params.(map[string]interface{})
		for key, value := range req.CustomParams {
			params[key] = value
		}
	}

	response, err := SendMCPStdioMessage(helper.stdin, helper.reader, request)
	if err != nil {
		return nil, fmt.Errorf("document symbol request failed: %w", err)
	}

	return helper.parseResponse(response)
}

// SendWorkspaceSymbolRequest sends a workspace/symbol request for TypeScript
func (helper *TypeScriptMCPTestHelper) SendWorkspaceSymbolRequest(req TypeScriptMCPRequest) (*TypeScriptMCPResponse, error) {
	request := MCPMessage{
		Jsonrpc: "2.0",
		ID:      helper.nextID(),
		Method:  "workspace/symbol",
		Params: map[string]interface{}{
			"query": req.Query,
		},
	}

	// Add custom parameters if provided
	if req.CustomParams != nil {
		params := request.Params.(map[string]interface{})
		for key, value := range req.CustomParams {
			params[key] = value
		}
	}

	response, err := SendMCPStdioMessage(helper.stdin, helper.reader, request)
	if err != nil {
		return nil, fmt.Errorf("workspace symbol request failed: %w", err)
	}

	return helper.parseResponse(response)
}

// SendCompletionRequest sends a textDocument/completion request for TypeScript
func (helper *TypeScriptMCPTestHelper) SendCompletionRequest(req TypeScriptMCPRequest) (*TypeScriptMCPResponse, error) {
	request := MCPMessage{
		Jsonrpc: "2.0",
		ID:      helper.nextID(),
		Method:  "textDocument/completion",
		Params: map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri": req.URI,
			},
			"position": map[string]interface{}{
				"line":      req.Position.Line,
				"character": req.Position.Character,
			},
		},
	}

	// Add TypeScript-specific completion context
	params := request.Params.(map[string]interface{})
	params["context"] = map[string]interface{}{
		"triggerKind": 1, // Invoked completion
	}

	// Add custom parameters if provided
	if req.CustomParams != nil {
		for key, value := range req.CustomParams {
			params[key] = value
		}
	}

	response, err := SendMCPStdioMessage(helper.stdin, helper.reader, request)
	if err != nil {
		return nil, fmt.Errorf("completion request failed: %w", err)
	}

	return helper.parseResponse(response)
}

// SendCustomRequest sends a custom MCP request with full control over parameters
func (helper *TypeScriptMCPTestHelper) SendCustomRequest(method string, params interface{}) (*TypeScriptMCPResponse, error) {
	request := MCPMessage{
		Jsonrpc: "2.0",
		ID:      helper.nextID(),
		Method:  method,
		Params:  params,
	}

	response, err := SendMCPStdioMessage(helper.stdin, helper.reader, request)
	if err != nil {
		return nil, fmt.Errorf("custom request failed: %w", err)
	}

	return helper.parseResponse(response)
}

// parseResponse parses an MCP response into TypeScript-specific structure
func (helper *TypeScriptMCPTestHelper) parseResponse(response *MCPMessage) (*TypeScriptMCPResponse, error) {
	tsResponse := &TypeScriptMCPResponse{
		MCPMessage: response,
	}

	// Handle error responses
	if response.Error != nil {
		tsResponse.Error = &MCPError{}
		if errorMap, ok := response.Error.(map[string]interface{}); ok {
			if code, exists := errorMap["code"]; exists {
				if codeFloat, ok := code.(float64); ok {
					tsResponse.Error.Code = int(codeFloat)
				}
			}
			if msg, exists := errorMap["message"]; exists {
				if msgStr, ok := msg.(string); ok {
					tsResponse.Error.Message = msgStr
				}
			}
			if data, exists := errorMap["data"]; exists {
				tsResponse.Error.Data = data
			}
		}
		return tsResponse, nil
	}

	// Parse successful responses based on result content
	if response.Result != nil {
		helper.parseResultData(tsResponse, response.Result)
	}

	return tsResponse, nil
}

// parseResultData parses the result data into TypeScript-specific structures
func (helper *TypeScriptMCPTestHelper) parseResultData(tsResponse *TypeScriptMCPResponse, result interface{}) {
	if resultArray, ok := result.([]interface{}); ok {
		// Handle array results (locations, symbols, etc.)
		for _, item := range resultArray {
			if itemMap, ok := item.(map[string]interface{}); ok {
				helper.parseResultItem(tsResponse, itemMap)
			}
		}
	} else if resultMap, ok := result.(map[string]interface{}); ok {
		// Handle single result objects (hover, etc.)
		helper.parseResultItem(tsResponse, resultMap)
	}
}

// parseResultItem parses individual result items
func (helper *TypeScriptMCPTestHelper) parseResultItem(tsResponse *TypeScriptMCPResponse, item map[string]interface{}) {
	// Check if this is a location
	if uri, hasURI := item["uri"]; hasURI {
		if rangeData, hasRange := item["range"]; hasRange {
			location := TypeScriptLocation{
				URI: uri.(string),
			}
			if rangeMap, ok := rangeData.(map[string]interface{}); ok {
				location.Range = helper.parseRange(rangeMap)
			}
			tsResponse.Locations = append(tsResponse.Locations, location)
		}
	}

	// Check if this is a symbol
	if name, hasName := item["name"]; hasName {
		if kind, hasKind := item["kind"]; hasKind {
			symbol := TypeScriptSymbol{
				Name: name.(string),
			}
			if kindFloat, ok := kind.(float64); ok {
				symbol.Kind = int(kindFloat)
			}
			if location, hasLocation := item["location"]; hasLocation {
				if locMap, ok := location.(map[string]interface{}); ok {
					symbol.Location = helper.parseLocation(locMap)
				}
			}
			if container, hasContainer := item["containerName"]; hasContainer {
				symbol.ContainerName = container.(string)
			}
			tsResponse.Symbols = append(tsResponse.Symbols, symbol)
		}
	}

	// Check if this is hover information
	if contents, hasContents := item["contents"]; hasContents {
		hover := &TypeScriptHover{}
		if contentsArray, ok := contents.([]interface{}); ok {
			for _, content := range contentsArray {
				if contentStr, ok := content.(string); ok {
					hover.Contents = append(hover.Contents, contentStr)
				} else if contentMap, ok := content.(map[string]interface{}); ok {
					if value, hasValue := contentMap["value"]; hasValue {
						if valueStr, ok := value.(string); ok {
							hover.Contents = append(hover.Contents, valueStr)
						}
					}
				}
			}
		}
		if rangeData, hasRange := item["range"]; hasRange {
			if rangeMap, ok := rangeData.(map[string]interface{}); ok {
				r := helper.parseRange(rangeMap)
				hover.Range = &r
			}
		}
		tsResponse.HoverInfo = hover
	}
}

// parseLocation parses a location object
func (helper *TypeScriptMCPTestHelper) parseLocation(locMap map[string]interface{}) TypeScriptLocation {
	location := TypeScriptLocation{}
	if uri, hasURI := locMap["uri"]; hasURI {
		location.URI = uri.(string)
	}
	if rangeData, hasRange := locMap["range"]; hasRange {
		if rangeMap, ok := rangeData.(map[string]interface{}); ok {
			location.Range = helper.parseRange(rangeMap)
		}
	}
	return location
}

// parseRange parses a range object
func (helper *TypeScriptMCPTestHelper) parseRange(rangeMap map[string]interface{}) LSPRange {
	r := LSPRange{}
	if start, hasStart := rangeMap["start"]; hasStart {
		if startMap, ok := start.(map[string]interface{}); ok {
			r.Start = helper.parsePosition(startMap)
		}
	}
	if end, hasEnd := rangeMap["end"]; hasEnd {
		if endMap, ok := end.(map[string]interface{}); ok {
			r.End = helper.parsePosition(endMap)
		}
	}
	return r
}

// parsePosition parses a position object
func (helper *TypeScriptMCPTestHelper) parsePosition(posMap map[string]interface{}) LSPPosition {
	pos := LSPPosition{}
	if line, hasLine := posMap["line"]; hasLine {
		if lineFloat, ok := line.(float64); ok {
			pos.Line = int(lineFloat)
		}
	}
	if char, hasChar := posMap["character"]; hasChar {
		if charFloat, ok := char.(float64); ok {
			pos.Character = int(charFloat)
		}
	}
	return pos
}

// Assertion helpers for TypeScript MCP responses

// AssertValidResponse asserts that the response is valid and not an error
func (helper *TypeScriptMCPTestHelper) AssertValidResponse(response *TypeScriptMCPResponse) error {
	if response == nil {
		return fmt.Errorf("response is nil")
	}
	if response.MCPMessage == nil {
		return fmt.Errorf("MCP message is nil")
	}
	if response.Error != nil {
		return fmt.Errorf("MCP error: %d - %s", response.Error.Code, response.Error.Message)
	}
	if response.MCPMessage.Jsonrpc != "2.0" {
		return fmt.Errorf("invalid JSON-RPC version: %s", response.MCPMessage.Jsonrpc)
	}
	return nil
}

// AssertHasLocations asserts that the response contains location data
func (helper *TypeScriptMCPTestHelper) AssertHasLocations(response *TypeScriptMCPResponse, minCount int) error {
	if err := helper.AssertValidResponse(response); err != nil {
		return err
	}
	if len(response.Locations) < minCount {
		return fmt.Errorf("expected at least %d locations, got %d", minCount, len(response.Locations))
	}
	return nil
}

// AssertHasSymbols asserts that the response contains symbol data
func (helper *TypeScriptMCPTestHelper) AssertHasSymbols(response *TypeScriptMCPResponse, minCount int) error {
	if err := helper.AssertValidResponse(response); err != nil {
		return err
	}
	if len(response.Symbols) < minCount {
		return fmt.Errorf("expected at least %d symbols, got %d", minCount, len(response.Symbols))
	}
	return nil
}

// AssertHasHoverInfo asserts that the response contains hover information
func (helper *TypeScriptMCPTestHelper) AssertHasHoverInfo(response *TypeScriptMCPResponse) error {
	if err := helper.AssertValidResponse(response); err != nil {
		return err
	}
	if response.HoverInfo == nil {
		return fmt.Errorf("expected hover information, got nil")
	}
	if len(response.HoverInfo.Contents) == 0 {
		return fmt.Errorf("expected hover contents, got empty array")
	}
	return nil
}

// AssertLocationContainsFile asserts that at least one location contains the specified file
func (helper *TypeScriptMCPTestHelper) AssertLocationContainsFile(response *TypeScriptMCPResponse, filename string) error {
	if err := helper.AssertHasLocations(response, 1); err != nil {
		return err
	}
	for _, location := range response.Locations {
		if strings.Contains(location.URI, filename) {
			return nil
		}
	}
	return fmt.Errorf("no location found containing file: %s", filename)
}

// AssertSymbolContainsName asserts that at least one symbol contains the specified name
func (helper *TypeScriptMCPTestHelper) AssertSymbolContainsName(response *TypeScriptMCPResponse, name string) error {
	if err := helper.AssertHasSymbols(response, 1); err != nil {
		return err
	}
	for _, symbol := range response.Symbols {
		if strings.Contains(symbol.Name, name) {
			return nil
		}
	}
	return fmt.Errorf("no symbol found containing name: %s", name)
}

// Test data preparation utilities

// CreateTypeScriptTestRequest creates a TypeScript MCP request with common defaults
func (helper *TypeScriptMCPTestHelper) CreateTypeScriptTestRequest(method string, filePath string, line, character int) TypeScriptMCPRequest {
	return TypeScriptMCPRequest{
		Method: method,
		URI:    helper.GetFileURI(filePath),
		Position: LSPPosition{
			Line:      line,
			Character: character,
		},
	}
}

// GetRandomTypeScriptFile returns a random TypeScript file for testing
func (helper *TypeScriptMCPTestHelper) GetRandomTypeScriptFile() (string, error) {
	if len(helper.tsFiles) == 0 {
		return "", fmt.Errorf("no TypeScript files available")
	}
	// Return the first file for consistency in tests
	return helper.tsFiles[0], nil
}

// GetTypeScriptFileByExtension returns the first file with the specified extension
func (helper *TypeScriptMCPTestHelper) GetTypeScriptFileByExtension(ext string) (string, error) {
	for _, file := range helper.tsFiles {
		if filepath.Ext(file) == ext {
			return file, nil
		}
	}
	return "", fmt.Errorf("no file found with extension: %s", ext)
}

// CreateTestPositions creates common test positions for TypeScript files
func (helper *TypeScriptMCPTestHelper) CreateTestPositions() []LSPPosition {
	return []LSPPosition{
		{Line: 0, Character: 0},   // Beginning of file
		{Line: 1, Character: 0},   // Second line start
		{Line: 2, Character: 8},   // Common indent position
		{Line: 5, Character: 10},  // Mid-function position
		{Line: 10, Character: 0},  // Further down in file
	}
}

// GetCommonTypeScriptQueries returns common search queries for TypeScript
func (helper *TypeScriptMCPTestHelper) GetCommonTypeScriptQueries() []string {
	return []string{
		"interface",
		"class",
		"function",
		"type",
		"const",
		"import",
		"export",
		"namespace",
		"enum",
		"module",
	}
}

// WaitForServerReadiness waits for the MCP server to be ready with retries
func (helper *TypeScriptMCPTestHelper) WaitForServerReadiness(maxRetries int) error {
	for i := 0; i < maxRetries; i++ {
		response, err := helper.SendInitializeRequest()
		if err == nil && helper.AssertValidResponse(response) == nil {
			return nil
		}
		time.Sleep(time.Duration(i+1) * time.Second)
	}
	return fmt.Errorf("server not ready after %d retries", maxRetries)
}

// LogResponse logs response information for debugging
func (helper *TypeScriptMCPTestHelper) LogResponse(response *TypeScriptMCPResponse, method string) string {
	if response == nil {
		return fmt.Sprintf("Response for %s: nil", method)
	}
	
	var details []string
	if response.Error != nil {
		details = append(details, fmt.Sprintf("Error: %d - %s", response.Error.Code, response.Error.Message))
	}
	if len(response.Locations) > 0 {
		details = append(details, fmt.Sprintf("Locations: %d", len(response.Locations)))
	}
	if len(response.Symbols) > 0 {
		details = append(details, fmt.Sprintf("Symbols: %d", len(response.Symbols)))
	}
	if response.HoverInfo != nil {
		details = append(details, fmt.Sprintf("Hover: %d contents", len(response.HoverInfo.Contents)))
	}
	
	return fmt.Sprintf("Response for %s: %s", method, strings.Join(details, ", "))
}

// GetResponseSummary returns a summary string for the response
func (helper *TypeScriptMCPTestHelper) GetResponseSummary(response *TypeScriptMCPResponse) string {
	if response == nil {
		return "nil response"
	}
	if response.Error != nil {
		return fmt.Sprintf("error: %s", response.Error.Message)
	}
	
	parts := []string{}
	if len(response.Locations) > 0 {
		parts = append(parts, strconv.Itoa(len(response.Locations))+" locations")
	}
	if len(response.Symbols) > 0 {
		parts = append(parts, strconv.Itoa(len(response.Symbols))+" symbols")
	}
	if response.HoverInfo != nil {
		parts = append(parts, "hover info")
	}
	
	if len(parts) == 0 {
		return "success (no data)"
	}
	return strings.Join(parts, ", ")
}