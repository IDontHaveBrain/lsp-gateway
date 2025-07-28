package helpers

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lsp-gateway/tests/e2e/mcp/types"
)


// NewTestWorkspace creates a new test workspace instance
func NewTestWorkspace(tempDir string) *types.TestWorkspace {
	return &types.TestWorkspace{
		RootPath: tempDir,
		TempDir:  tempDir,
		Files:    make(map[string]*types.WorkspaceFile),
		Symbols:  make(map[string]*types.SymbolInfo),
	}
}

// SetupFatihColorRepository clones and sets up the fatih/color repository
func SetupFatihColorRepository(w *types.TestWorkspace) error {
	repoURL := "https://github.com/fatih/color.git"
	commitHash := "4c05561a8fbfd21922e4908479e63b48b677a61f"
	
	w.RepositoryPath = filepath.Join(w.TempDir, "color")
	
	if err := cloneRepository(w, repoURL, w.RepositoryPath, commitHash); err != nil {
		return fmt.Errorf("failed to clone fatih/color: %w", err)
	}
	
	if err := loadSourceFiles(w); err != nil {
		return fmt.Errorf("failed to load source files: %w", err)
	}
	
	if err := parseSymbols(w); err != nil {
		return fmt.Errorf("failed to parse symbols: %w", err)
	}
	
	return nil
}

// cloneRepository clones a git repository to the specified path at a specific commit
func cloneRepository(w *types.TestWorkspace, repoURL, localPath, commitHash string) error {
	cloner := NewRepositoryCloner(repoURL, commitHash)
	
	result, err := cloner.CloneToWorkspace(filepath.Dir(localPath))
	if err != nil {
		return fmt.Errorf("repository clone failed: %w", err)
	}
	
	if len(result.ValidationErrors) > 0 {
		return fmt.Errorf("repository validation failed: %v", result.ValidationErrors)
	}
	
	return nil
}

// loadSourceFiles loads Go source files from the repository
func loadSourceFiles(w *types.TestWorkspace) error {
	sourceFiles := []string{
		"color.go",
		"color_test.go",
		"color_windows.go",
		"doc.go",
	}
	
	for _, filename := range sourceFiles {
		filePath := filepath.Join(w.RepositoryPath, "color", filename)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			continue
		}
		
		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", filename, err)
		}
		
		w.Files[filename] = &types.WorkspaceFile{
			Path:         filePath,
			Content:      string(content),
			Language:     "go",
			LastModified: time.Now(),
		}
	}
	
	return nil
}

// parseSymbols parses Go source files to extract symbol information
func parseSymbols(w *types.TestWorkspace) error {
	parser := NewGoSymbolParser()
	
	for filename, file := range w.Files {
		if filepath.Ext(filename) != ".go" {
			continue
		}
		
		symbols, err := parser.ParseFile(file.Path, file.Content)
		if err != nil {
			return fmt.Errorf("failed to parse symbols in %s: %w", filename, err)
		}
		
		for _, symbol := range symbols {
			symbolKey := fmt.Sprintf("%s:%s", filename, symbol.Name)
			w.Symbols[symbolKey] = symbol
		}
	}
	
	return nil
}

// BuilderConfig contains configuration for test case generation
type BuilderConfig struct {
	Repository     RepositoryConfig     `json:"repository"`
	TestGeneration TestGenerationConfig `json:"testGeneration"`
	Validation     types.ValidationConfig     `json:"validation"`
}

// RepositoryConfig contains repository-specific configuration
type RepositoryConfig struct {
	URL          string        `json:"url"`
	CommitHash   string        `json:"commitHash"`
	LocalPath    string        `json:"localPath"`
	CloneTimeout time.Duration `json:"cloneTimeout"`
}

// TestGenerationConfig contains test generation configuration
type TestGenerationConfig struct {
	MaxTestCases     int            `json:"maxTestCases"`
	IncludeNegative  bool           `json:"includeNegative"`
	IncludeEdgeCases bool           `json:"includeEdgeCases"`
	PriorityFilter   []types.TestPriority `json:"priorityFilter"`
	TagFilter        []string       `json:"tagFilter"`
}


// DefaultBuilderConfig returns default configuration for test case building
func DefaultBuilderConfig() *BuilderConfig {
	return &BuilderConfig{
		Repository: RepositoryConfig{
			URL:          "https://github.com/fatih/color.git",
			CommitHash:   "4c05561a8fbfd21922e4908479e63b48b677a61f",
			CloneTimeout: 30 * time.Second,
		},
		TestGeneration: TestGenerationConfig{
			MaxTestCases:     100,
			IncludeNegative:  true,
			IncludeEdgeCases: true,
			PriorityFilter:   []types.TestPriority{types.HighPriority, types.MediumPriority},
		},
		Validation: types.ValidationConfig{
			ValidateSymbols:    true,
			ValidateReferences: true,
			ValidateHover:      true,
		},
	}
}

// TestCaseBuilder builds test cases for MCP E2E testing
type TestCaseBuilder struct {
	workspace     *types.TestWorkspace
	symbolMap     map[string]*types.SymbolInfo
	config        *BuilderConfig
	repository    *RepositoryCloner
}

// NewTestCaseBuilder creates a new test case builder
func NewTestCaseBuilder(workspace *types.TestWorkspace) *TestCaseBuilder {
	return &TestCaseBuilder{
		workspace: workspace,
		symbolMap: make(map[string]*types.SymbolInfo),
		config:    DefaultBuilderConfig(),
		repository: NewRepositoryCloner(
			"https://github.com/fatih/color.git",
			"4c05561a8fbfd21922e4908479e63b48b677a61f",
		),
	}
}

// GenerateDefinitionTestCases generates test cases for goto definition functionality
func (b *TestCaseBuilder) GenerateDefinitionTestCases() []*types.TestCase {
	cases := []*types.TestCase{}
	
	colorSymbols := []struct {
		name            string
		line            int
		character       int
		expectedDefLine int
		expectedDefChar int
		symbolType      string
	}{
		{"Red", 123, 8, 110, 5, "function"},
		{"Color", 45, 10, 35, 5, "struct"},
		{"Print", 78, 15, 65, 5, "method"},
		{"Bold", 89, 12, 55, 1, "constant"},
		{"Reset", 67, 18, 42, 1, "constant"},
		{"New", 34, 12, 28, 5, "function"},
		{"Set", 91, 8, 85, 5, "method"},
		{"Unset", 102, 14, 95, 5, "method"},
		{"FgColor", 156, 20, 145, 1, "constant"},
		{"BgColor", 178, 22, 167, 1, "constant"},
	}
	
	for _, symbol := range colorSymbols {
		testCase := &types.TestCase{
			ID:          fmt.Sprintf("def_%s", strings.ToLower(symbol.name)),
			Name:        fmt.Sprintf("Go %s definition - %s", symbol.symbolType, symbol.name),
			Description: fmt.Sprintf("Test goto definition for %s %s in fatih/color", symbol.symbolType, symbol.name),
			Tool:        "goto_definition",
			Language:    "go",
			File:        "color.go",
			Position:    types.LSPPosition{Line: symbol.line, Character: symbol.character},
			Expected: &types.ExpectedResult{
				Type: "definition",
				Location: &types.LSPLocation{
					URI: "file:///workspace/color.go",
					Range: types.LSPRange{
						Start: types.LSPPosition{Line: symbol.expectedDefLine, Character: symbol.expectedDefChar},
						End:   types.LSPPosition{Line: symbol.expectedDefLine, Character: symbol.expectedDefChar + len(symbol.name)},
					},
				},
			},
			Tags:     []string{"definition", "go", "fatih-color", symbol.symbolType},
			Priority: types.HighPriority,
		}
		cases = append(cases, testCase)
	}
	
	return cases
}

// GenerateReferencesTestCases generates test cases for find references functionality
func (b *TestCaseBuilder) GenerateReferencesTestCases() []*types.TestCase {
	cases := []*types.TestCase{}
	
	referenceSymbols := []struct {
		name           string
		definitionLine int
		definitionChar int
		expectedRefs   []types.LSPLocation
		symbolType     string
	}{
		{
			name:           "Color",
			definitionLine: 35,
			definitionChar: 5,
			symbolType:     "struct",
			expectedRefs: []types.LSPLocation{
				{URI: "file:///workspace/color.go", Range: types.LSPRange{Start: types.LSPPosition{Line: 45, Character: 10}, End: types.LSPPosition{Line: 45, Character: 15}}},
				{URI: "file:///workspace/color.go", Range: types.LSPRange{Start: types.LSPPosition{Line: 78, Character: 8}, End: types.LSPPosition{Line: 78, Character: 13}}},
				{URI: "file:///workspace/color_test.go", Range: types.LSPRange{Start: types.LSPPosition{Line: 23, Character: 15}, End: types.LSPPosition{Line: 23, Character: 20}}},
			},
		},
		{
			name:           "Red",
			definitionLine: 110,
			definitionChar: 5,
			symbolType:     "function",
			expectedRefs: []types.LSPLocation{
				{URI: "file:///workspace/color.go", Range: types.LSPRange{Start: types.LSPPosition{Line: 123, Character: 8}, End: types.LSPPosition{Line: 123, Character: 11}}},
				{URI: "file:///workspace/color_test.go", Range: types.LSPRange{Start: types.LSPPosition{Line: 34, Character: 12}, End: types.LSPPosition{Line: 34, Character: 15}}},
			},
		},
		{
			name:           "Print",
			definitionLine: 65,
			definitionChar: 5,
			symbolType:     "method",
			expectedRefs: []types.LSPLocation{
				{URI: "file:///workspace/color.go", Range: types.LSPRange{Start: types.LSPPosition{Line: 78, Character: 15}, End: types.LSPPosition{Line: 78, Character: 20}}},
				{URI: "file:///workspace/color_test.go", Range: types.LSPRange{Start: types.LSPPosition{Line: 45, Character: 18}, End: types.LSPPosition{Line: 45, Character: 23}}},
			},
		},
	}
	
	for _, ref := range referenceSymbols {
		testCase := &types.TestCase{
			ID:          fmt.Sprintf("ref_%s", strings.ToLower(ref.name)),
			Name:        fmt.Sprintf("Find references - %s %s", ref.symbolType, ref.name),
			Description: fmt.Sprintf("Test find references for %s %s in fatih/color", ref.symbolType, ref.name),
			Tool:        "find_references",
			Language:    "go",
			File:        "color.go",
			Position:    types.LSPPosition{Line: ref.definitionLine, Character: ref.definitionChar},
			Expected: &types.ExpectedResult{
				Type:      "references",
				Locations: ref.expectedRefs,
			},
			Tags:     []string{"references", "go", "fatih-color", ref.symbolType},
			Priority: types.HighPriority,
		}
		cases = append(cases, testCase)
	}
	
	return cases
}

// GenerateHoverTestCases generates test cases for hover functionality
func (b *TestCaseBuilder) GenerateHoverTestCases() []*types.TestCase {
	cases := []*types.TestCase{}
	
	hoverSymbols := []struct {
		name         string
		line         int
		character    int
		expectedText string
		symbolType   string
	}{
		{
			name:         "Color",
			line:         45,
			character:    10,
			expectedText: "type Color struct",
			symbolType:   "struct",
		},
		{
			name:         "Red",
			line:         123,
			character:    8,
			expectedText: "func Red(format string, a ...interface{}) string",
			symbolType:   "function",
		},
		{
			name:         "Print",
			line:         78,
			character:    15,
			expectedText: "func (c *Color) Print(a ...interface{}) (n int, err error)",
			symbolType:   "method",
		},
		{
			name:         "Bold",
			line:         89,
			character:    12,
			expectedText: "const Bold",
			symbolType:   "constant",
		},
	}
	
	for _, hover := range hoverSymbols {
		testCase := &types.TestCase{
			ID:          fmt.Sprintf("hover_%s", strings.ToLower(hover.name)),
			Name:        fmt.Sprintf("Hover info - %s %s", hover.symbolType, hover.name),
			Description: fmt.Sprintf("Test hover information for %s %s in fatih/color", hover.symbolType, hover.name),
			Tool:        "get_hover_info",
			Language:    "go",
			File:        "color.go",
			Position:    types.LSPPosition{Line: hover.line, Character: hover.character},
			Expected: &types.ExpectedResult{
				Type:    "hover",
				Content: hover.expectedText,
			},
			Tags:     []string{"hover", "go", "fatih-color", hover.symbolType},
			Priority: types.MediumPriority,
		}
		cases = append(cases, testCase)
	}
	
	return cases
}

// GenerateDocumentSymbolTestCases generates test cases for document symbols functionality
func (b *TestCaseBuilder) GenerateDocumentSymbolTestCases() []*types.TestCase {
	cases := []*types.TestCase{}
	
	documentFiles := []struct {
		filename      string
		expectedCount int
		symbolTypes   []string
	}{
		{
			filename:      "color.go",
			expectedCount: 25,
			symbolTypes:   []string{"struct", "function", "method", "constant"},
		},
		{
			filename:      "color_test.go",
			expectedCount: 15,
			symbolTypes:   []string{"function"},
		},
	}
	
	for _, doc := range documentFiles {
		testCase := &types.TestCase{
			ID:          fmt.Sprintf("docsym_%s", strings.ReplaceAll(doc.filename, ".", "_")),
			Name:        fmt.Sprintf("Document symbols - %s", doc.filename),
			Description: fmt.Sprintf("Test document symbol extraction for %s in fatih/color", doc.filename),
			Tool:        "get_document_symbols",
			Language:    "go",
			File:        doc.filename,
			Position:    types.LSPPosition{Line: 0, Character: 0},
			Expected: &types.ExpectedResult{
				Type:    "documentSymbols",
				Content: fmt.Sprintf("Expected %d symbols of types: %v", doc.expectedCount, doc.symbolTypes),
			},
			Tags:     []string{"documentSymbols", "go", "fatih-color"},
			Priority: types.MediumPriority,
		}
		cases = append(cases, testCase)
	}
	
	return cases
}

// GenerateWorkspaceSymbolTestCases generates test cases for workspace symbols functionality
func (b *TestCaseBuilder) GenerateWorkspaceSymbolTestCases() []*types.TestCase {
	cases := []*types.TestCase{}
	
	workspaceQueries := []struct {
		query           string
		expectedSymbols []string
		symbolType      string
	}{
		{
			query:           "Color",
			expectedSymbols: []string{"Color", "FgColor", "BgColor"},
			symbolType:      "mixed",
		},
		{
			query:           "Print",
			expectedSymbols: []string{"Print", "Printf", "Println", "Sprint", "Sprintf", "Sprintln"},
			symbolType:      "function/method",
		},
		{
			query:           "Red",
			expectedSymbols: []string{"Red", "FgRed", "BgRed"},
			symbolType:      "color",
		},
		{
			query:           "Test",
			expectedSymbols: []string{"TestColor", "TestPrint", "TestSprint"},
			symbolType:      "test",
		},
	}
	
	for _, query := range workspaceQueries {
		testCase := &types.TestCase{
			ID:          fmt.Sprintf("worksym_%s", strings.ToLower(query.query)),
			Name:        fmt.Sprintf("Workspace symbols - %s", query.query),
			Description: fmt.Sprintf("Test workspace symbol search for '%s' in fatih/color", query.query),
			Tool:        "search_workspace_symbols",
			Language:    "go",
			File:        "",
			Position:    types.LSPPosition{Line: 0, Character: 0},
			Expected: &types.ExpectedResult{
				Type:            "workspaceSymbols",
				Query:           query.query,
				CompletionItems: query.expectedSymbols,
			},
			Tags:     []string{"workspaceSymbols", "go", "fatih-color", query.symbolType},
			Priority: types.MediumPriority,
		}
		cases = append(cases, testCase)
	}
	
	return cases
}

// GenerateCompletionTestCases generates test cases for completion functionality
func (b *TestCaseBuilder) GenerateCompletionTestCases() []*types.TestCase {
	cases := []*types.TestCase{}
	
	completionContexts := []struct {
		name              string
		line              int
		character         int
		expectedItems     []string
		completionContext string
	}{
		{
			name:              "color_methods",
			line:              85,
			character:         12,
			expectedItems:     []string{"Print", "Printf", "Println", "Sprint", "Sprintf", "Set", "Unset"},
			completionContext: "method completion on Color instance",
		},
		{
			name:              "color_package",
			line:              5,
			character:         8,
			expectedItems:     []string{"Red", "Green", "Blue", "Yellow", "Magenta", "Cyan", "White", "Black"},
			completionContext: "package-level function completion",
		},
		{
			name:              "attribute_constants",
			line:              45,
			character:         15,
			expectedItems:     []string{"Bold", "Faint", "Italic", "Underline", "BlinkSlow", "BlinkRapid", "ReverseVideo", "Concealed", "CrossedOut"},
			completionContext: "attribute constant completion",
		},
	}
	
	for _, completion := range completionContexts {
		testCase := &types.TestCase{
			ID:          fmt.Sprintf("comp_%s", completion.name),
			Name:        fmt.Sprintf("Completion - %s", completion.completionContext),
			Description: fmt.Sprintf("Test code completion for %s in fatih/color", completion.completionContext),
			Tool:        "get_completions",
			Language:    "go",
			File:        "color.go",
			Position:    types.LSPPosition{Line: completion.line, Character: completion.character},
			Expected: &types.ExpectedResult{
				Type:            "completion",
				CompletionItems: completion.expectedItems,
			},
			Tags:     []string{"completion", "go", "fatih-color"},
			Priority: types.LowPriority,
		}
		cases = append(cases, testCase)
	}
	
	return cases
}

// GenerateAllTestCases generates all test cases for all LSP features
func (b *TestCaseBuilder) GenerateAllTestCases() []*types.TestCase {
	allCases := []*types.TestCase{}
	
	allCases = append(allCases, b.GenerateDefinitionTestCases()...)
	allCases = append(allCases, b.GenerateReferencesTestCases()...)
	allCases = append(allCases, b.GenerateHoverTestCases()...)
	allCases = append(allCases, b.GenerateDocumentSymbolTestCases()...)
	allCases = append(allCases, b.GenerateWorkspaceSymbolTestCases()...)
	allCases = append(allCases, b.GenerateCompletionTestCases()...)
	
	return b.filterTestCases(allCases)
}

// filterTestCases filters test cases based on configuration
func (b *TestCaseBuilder) filterTestCases(cases []*types.TestCase) []*types.TestCase {
	filtered := []*types.TestCase{}
	
	for _, testCase := range cases {
		if len(filtered) >= b.config.TestGeneration.MaxTestCases {
			break
		}
		
		if b.shouldIncludeTestCase(testCase) {
			filtered = append(filtered, testCase)
		}
	}
	
	return filtered
}

// shouldIncludeTestCase determines if a test case should be included based on configuration
func (b *TestCaseBuilder) shouldIncludeTestCase(testCase *types.TestCase) bool {
	for _, priority := range b.config.TestGeneration.PriorityFilter {
		if testCase.Priority == priority {
			if len(b.config.TestGeneration.TagFilter) == 0 {
				return true
			}
			
			for _, filterTag := range b.config.TestGeneration.TagFilter {
				for _, testTag := range testCase.Tags {
					if testTag == filterTag {
						return true
					}
				}
			}
		}
	}
	
	return false
}

// MCPToolMapper maps LSP methods to MCP tool names and builds parameters
type MCPToolMapper struct {
	toolMap       map[string]string
	paramBuilders map[string]func(tc *types.TestCase) map[string]interface{}
}

// NewMCPToolMapper creates a new MCP tool mapper
func NewMCPToolMapper() *MCPToolMapper {
	mapper := &MCPToolMapper{
		toolMap: map[string]string{
			"definition":      "goto_definition",
			"references":      "find_references", 
			"hover":           "get_hover_info",
			"documentSymbol":  "get_document_symbols",
			"workspaceSymbol": "search_workspace_symbols",
			"completion":      "get_completions",
		},
		paramBuilders: make(map[string]func(tc *types.TestCase) map[string]interface{}),
	}
	
	mapper.setupParameterBuilders()
	return mapper
}

// BuildParameters builds MCP tool parameters from a test case
func (m *MCPToolMapper) BuildParameters(testCase *types.TestCase) map[string]interface{} {
	builder, exists := m.paramBuilders[testCase.Tool]
	if !exists {
		return map[string]interface{}{
			"uri":       fmt.Sprintf("file:///workspace/%s", testCase.File),
			"line":      testCase.Position.Line,
			"character": testCase.Position.Character,
		}
	}
	return builder(testCase)
}

// setupParameterBuilders configures parameter builders for each tool type
func (m *MCPToolMapper) setupParameterBuilders() {
	m.paramBuilders["goto_definition"] = func(tc *types.TestCase) map[string]interface{} {
		return map[string]interface{}{
			"uri":       fmt.Sprintf("file:///workspace/%s", tc.File),
			"line":      tc.Position.Line,
			"character": tc.Position.Character,
		}
	}
	
	m.paramBuilders["find_references"] = func(tc *types.TestCase) map[string]interface{} {
		return map[string]interface{}{
			"uri":                 fmt.Sprintf("file:///workspace/%s", tc.File),
			"line":                tc.Position.Line,
			"character":           tc.Position.Character,
			"include_declaration": true,
		}
	}
	
	m.paramBuilders["get_hover_info"] = func(tc *types.TestCase) map[string]interface{} {
		return map[string]interface{}{
			"uri":       fmt.Sprintf("file:///workspace/%s", tc.File),
			"line":      tc.Position.Line,
			"character": tc.Position.Character,
		}
	}
	
	m.paramBuilders["get_document_symbols"] = func(tc *types.TestCase) map[string]interface{} {
		return map[string]interface{}{
			"uri": fmt.Sprintf("file:///workspace/%s", tc.File),
		}
	}
	
	m.paramBuilders["search_workspace_symbols"] = func(tc *types.TestCase) map[string]interface{} {
		return map[string]interface{}{
			"query": tc.Expected.Query,
		}
	}
	
	m.paramBuilders["get_completions"] = func(tc *types.TestCase) map[string]interface{} {
		return map[string]interface{}{
			"uri":       fmt.Sprintf("file:///workspace/%s", tc.File),
			"line":      tc.Position.Line,
			"character": tc.Position.Character,
		}
	}
}

// GoSymbolParser parses Go source files to extract symbol information
type GoSymbolParser struct {
	fileSet *token.FileSet
}

// NewGoSymbolParser creates a new Go symbol parser
func NewGoSymbolParser() *GoSymbolParser {
	return &GoSymbolParser{
		fileSet: token.NewFileSet(),
	}
}

// ParseFile parses a Go file and extracts symbol information
func (p *GoSymbolParser) ParseFile(filePath, content string) ([]*types.SymbolInfo, error) {
	file, err := parser.ParseFile(p.fileSet, filePath, content, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Go file: %w", err)
	}
	
	symbols := []*types.SymbolInfo{}
	packageName := file.Name.Name
	
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncDecl:
			symbol := p.extractFunctionSymbol(node, packageName, filePath)
			if symbol != nil {
				symbols = append(symbols, symbol)
			}
		case *ast.GenDecl:
			funcSymbols := p.extractGenDeclSymbols(node, packageName, filePath)
			symbols = append(symbols, funcSymbols...)
		}
		return true
	})
	
	return symbols, nil
}

// extractFunctionSymbol extracts symbol information from function declarations
func (p *GoSymbolParser) extractFunctionSymbol(funcDecl *ast.FuncDecl, packageName, filePath string) *types.SymbolInfo {
	if funcDecl.Name == nil {
		return nil
	}
	
	pos := p.fileSet.Position(funcDecl.Name.Pos())
	
	symbolType := "function"
	if funcDecl.Recv != nil {
		symbolType = "method"
	}
	
	return &types.SymbolInfo{
		Name:       funcDecl.Name.Name,
		Kind:       12, // LSP Function kind
		Package:    packageName,
		Type:       symbolType,
		IsExported: ast.IsExported(funcDecl.Name.Name),
		Location: types.LSPLocation{
			URI: fmt.Sprintf("file://%s", filePath),
			Range: types.LSPRange{
				Start: types.LSPPosition{Line: pos.Line - 1, Character: pos.Column - 1},
				End:   types.LSPPosition{Line: pos.Line - 1, Character: pos.Column - 1 + len(funcDecl.Name.Name)},
			},
		},
		Definition: types.LSPLocation{
			URI: fmt.Sprintf("file://%s", filePath),
			Range: types.LSPRange{
				Start: types.LSPPosition{Line: pos.Line - 1, Character: pos.Column - 1},
				End:   types.LSPPosition{Line: pos.Line - 1, Character: pos.Column - 1 + len(funcDecl.Name.Name)},
			},
		},
	}
}

// extractGenDeclSymbols extracts symbols from general declarations (types, constants, variables)
func (p *GoSymbolParser) extractGenDeclSymbols(genDecl *ast.GenDecl, packageName, filePath string) []*types.SymbolInfo {
	symbols := []*types.SymbolInfo{}
	
	for _, spec := range genDecl.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			if s.Name != nil {
				pos := p.fileSet.Position(s.Name.Pos())
				symbols = append(symbols, &types.SymbolInfo{
					Name:       s.Name.Name,
					Kind:       5, // LSP Class kind for structs/types
					Package:    packageName,
					Type:       "type",
					IsExported: ast.IsExported(s.Name.Name),
					Location: types.LSPLocation{
						URI: fmt.Sprintf("file://%s", filePath),
						Range: types.LSPRange{
							Start: types.LSPPosition{Line: pos.Line - 1, Character: pos.Column - 1},
							End:   types.LSPPosition{Line: pos.Line - 1, Character: pos.Column - 1 + len(s.Name.Name)},
						},
					},
					Definition: types.LSPLocation{
						URI: fmt.Sprintf("file://%s", filePath),
						Range: types.LSPRange{
							Start: types.LSPPosition{Line: pos.Line - 1, Character: pos.Column - 1},
							End:   types.LSPPosition{Line: pos.Line - 1, Character: pos.Column - 1 + len(s.Name.Name)},
						},
					},
				})
			}
		case *ast.ValueSpec:
			for _, name := range s.Names {
				if name != nil {
					pos := p.fileSet.Position(name.Pos())
					symbolKind := 13 // LSP Variable kind
					symbolType := "variable"
					
					if genDecl.Tok == token.CONST {
						symbolKind = 14 // LSP Constant kind
						symbolType = "constant"
					}
					
					symbols = append(symbols, &types.SymbolInfo{
						Name:       name.Name,
						Kind:       symbolKind,
						Package:    packageName,
						Type:       symbolType,
						IsExported: ast.IsExported(name.Name),
						Location: types.LSPLocation{
							URI: fmt.Sprintf("file://%s", filePath),
							Range: types.LSPRange{
								Start: types.LSPPosition{Line: pos.Line - 1, Character: pos.Column - 1},
								End:   types.LSPPosition{Line: pos.Line - 1, Character: pos.Column - 1 + len(name.Name)},
							},
						},
						Definition: types.LSPLocation{
							URI: fmt.Sprintf("file://%s", filePath),
							Range: types.LSPRange{
								Start: types.LSPPosition{Line: pos.Line - 1, Character: pos.Column - 1},
								End:   types.LSPPosition{Line: pos.Line - 1, Character: pos.Column - 1 + len(name.Name)},
							},
						},
					})
				}
			}
		}
	}
	
	return symbols
}