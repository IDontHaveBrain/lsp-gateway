package indexing

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/sourcegraph/scip/bindings/go/scip"
)

// GoDependencyParser implements dependency parsing for Go language
type GoDependencyParser struct {
	language string
	fileSet  *token.FileSet
}

// NewGoDependencyParser creates a new Go dependency parser
func NewGoDependencyParser() *GoDependencyParser {
	return &GoDependencyParser{
		language: "go",
		fileSet:  token.NewFileSet(),
	}
}

// ParseDependencies extracts import dependencies from Go files
func (p *GoDependencyParser) ParseDependencies(filePath string, content []byte) ([]*DependencyEdge, error) {
	edges := make([]*DependencyEdge, 0)

	// Parse Go file
	astFile, err := parser.ParseFile(p.fileSet, filePath, content, parser.ParseComments)
	if err != nil {
		// Fallback to regex-based parsing for syntax errors
		return p.parseImportsWithRegex(filePath, content)
	}

	// Extract imports
	for _, imp := range astFile.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)

		// Skip standard library imports
		if p.isStandardLibrary(importPath) {
			continue
		}

		// Resolve import to file path
		targetPath := p.resolveImportPath(filePath, importPath)
		if targetPath == "" {
			continue
		}

		edge := &DependencyEdge{
			From:            filePath,
			To:              targetPath,
			DependencyType:  DependencyTypeImport,
			Strength:        1.0,
			Confidence:      0.95,
			ImportStatement: fmt.Sprintf("import %s", imp.Path.Value),
			CreatedAt:       time.Now(),
			LastUpdated:     time.Now(),
			Source:          "go_ast_parser",
		}

		// Handle aliased imports
		if imp.Name != nil {
			edge.ImportStatement = fmt.Sprintf("import %s %s", imp.Name.Name, imp.Path.Value)
		}

		edges = append(edges, edge)
	}

	return edges, nil
}

// ParseSymbols extracts symbol definitions and references from Go files
func (p *GoDependencyParser) ParseSymbols(filePath string, content []byte) ([]*SymbolDependency, error) {
	symbols := make([]*SymbolDependency, 0)

	// Parse Go file
	astFile, err := parser.ParseFile(p.fileSet, filePath, content, parser.ParseComments)
	if err != nil {
		return symbols, err
	}

	// Extract symbols using AST visitor
	visitor := &goSymbolVisitor{
		filePath: filePath,
		symbols:  &symbols,
		fileSet:  p.fileSet,
	}

	ast.Walk(visitor, astFile)

	return symbols, nil
}

// DetectCrossLanguageDependencies identifies cross-language dependencies in Go files
func (p *GoDependencyParser) DetectCrossLanguageDependencies(filePath string, content []byte) ([]*CrossLanguageDependency, error) {
	crossDeps := make([]*CrossLanguageDependency, 0)

	// Look for CGO dependencies
	cgoDeps := p.detectCGODependencies(filePath, content)
	crossDeps = append(crossDeps, cgoDeps...)

	// Look for embedded files (go:embed)
	embedDeps := p.detectEmbeddedFiles(filePath, content)
	crossDeps = append(crossDeps, embedDeps...)

	// Look for configuration file references
	configDeps := p.detectConfigReferences(filePath, content)
	crossDeps = append(crossDeps, configDeps...)

	return crossDeps, nil
}

// GetSupportedExtensions returns file extensions this parser supports
func (p *GoDependencyParser) GetSupportedExtensions() []string {
	return []string{".go"}
}

// GetLanguage returns the language this parser handles
func (p *GoDependencyParser) GetLanguage() string {
	return p.language
}

// Helper methods for Go parser

// parseImportsWithRegex provides fallback regex-based import parsing
func (p *GoDependencyParser) parseImportsWithRegex(filePath string, content []byte) ([]*DependencyEdge, error) {
	edges := make([]*DependencyEdge, 0)

	// Regex patterns for Go imports
	singleImportPattern := regexp.MustCompile(`import\s+"([^"]+)"`)
	multiImportPattern := regexp.MustCompile(`import\s*\(\s*((?:[^)]*\n?)*)\s*\)`)
	importLinePattern := regexp.MustCompile(`(?:(\w+)\s+)?"([^"]+)"`)

	contentStr := string(content)

	// Find single imports
	singleMatches := singleImportPattern.FindAllStringSubmatch(contentStr, -1)
	for _, match := range singleMatches {
		if len(match) > 1 {
			importPath := match[1]
			if !p.isStandardLibrary(importPath) {
				targetPath := p.resolveImportPath(filePath, importPath)
				if targetPath != "" {
					edge := &DependencyEdge{
						From:            filePath,
						To:              targetPath,
						DependencyType:  DependencyTypeImport,
						Strength:        1.0,
						Confidence:      0.8, // Lower confidence for regex parsing
						ImportStatement: fmt.Sprintf("import \"%s\"", importPath),
						CreatedAt:       time.Now(),
						LastUpdated:     time.Now(),
						Source:          "go_regex_parser",
					}
					edges = append(edges, edge)
				}
			}
		}
	}

	// Find multi-line imports
	multiMatches := multiImportPattern.FindAllStringSubmatch(contentStr, -1)
	for _, match := range multiMatches {
		if len(match) > 1 {
			importBlock := match[1]
			lineMatches := importLinePattern.FindAllStringSubmatch(importBlock, -1)

			for _, lineMatch := range lineMatches {
				if len(lineMatch) > 2 {
					importPath := lineMatch[2]
					if !p.isStandardLibrary(importPath) {
						targetPath := p.resolveImportPath(filePath, importPath)
						if targetPath != "" {
							importStmt := fmt.Sprintf("import \"%s\"", importPath)
							if lineMatch[1] != "" {
								importStmt = fmt.Sprintf("import %s \"%s\"", lineMatch[1], importPath)
							}

							edge := &DependencyEdge{
								From:            filePath,
								To:              targetPath,
								DependencyType:  DependencyTypeImport,
								Strength:        1.0,
								Confidence:      0.8,
								ImportStatement: importStmt,
								CreatedAt:       time.Now(),
								LastUpdated:     time.Now(),
								Source:          "go_regex_parser",
							}
							edges = append(edges, edge)
						}
					}
				}
			}
		}
	}

	return edges, nil
}

// isStandardLibrary checks if an import path is from Go's standard library
func (p *GoDependencyParser) isStandardLibrary(importPath string) bool {
	// Common standard library prefixes
	stdLibPrefixes := []string{
		"bufio", "bytes", "context", "crypto", "database", "encoding",
		"errors", "fmt", "go", "hash", "html", "image", "io", "log",
		"math", "mime", "net", "os", "path", "reflect", "regexp",
		"runtime", "sort", "strconv", "strings", "sync", "syscall",
		"testing", "text", "time", "unicode", "unsafe",
	}

	// Check if it starts with any standard library prefix
	for _, prefix := range stdLibPrefixes {
		if strings.HasPrefix(importPath, prefix) &&
			(len(importPath) == len(prefix) || importPath[len(prefix)] == '/') {
			return true
		}
	}

	// Additional check: if it doesn't contain a dot and doesn't start with known prefixes,
	// it's likely standard library
	return !strings.Contains(importPath, ".") && !strings.Contains(importPath, "/")
}

// resolveImportPath resolves a Go import path to a local file path
func (p *GoDependencyParser) resolveImportPath(currentFile, importPath string) string {
	// This is a simplified implementation
	// In practice, this would use go.mod, GOPATH, etc.

	currentDir := filepath.Dir(currentFile)

	// Try relative resolution first
	if strings.HasPrefix(importPath, ".") {
		resolved := filepath.Join(currentDir, importPath)
		return resolved
	}

	// Try to find in current project
	// Look for go.mod to find project root
	projectRoot := p.findProjectRoot(currentFile)
	if projectRoot != "" {
		// Simple heuristic: assume package corresponds to directory
		packageDir := filepath.Join(projectRoot, strings.ReplaceAll(importPath, "/", string(filepath.Separator)))
		return packageDir
	}

	return ""
}

// findProjectRoot finds the project root by looking for go.mod
func (p *GoDependencyParser) findProjectRoot(filePath string) string {
	dir := filepath.Dir(filePath)

	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := filepath.Abs(goModPath); err == nil {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return ""
}

// detectCGODependencies detects CGO dependencies
func (p *GoDependencyParser) detectCGODependencies(filePath string, content []byte) []*CrossLanguageDependency {
	deps := make([]*CrossLanguageDependency, 0)

	// Look for CGO comments
	cgoPattern := regexp.MustCompile(`(?m)^//\s*#(?:include|cgo).*$`)
	matches := cgoPattern.FindAll(content, -1)

	for _, match := range matches {
		dep := &CrossLanguageDependency{
			SourceFile:     filePath,
			SourceLanguage: "go",
			TargetFile:     "", // Would need to resolve the header file
			TargetLanguage: "c",
			RelationType:   "ffi",
			InterfaceType:  "cgo",
			Confidence:     0.9,
			Description:    fmt.Sprintf("CGO dependency: %s", string(match)),
		}
		deps = append(deps, dep)
	}

	return deps
}

// detectEmbeddedFiles detects go:embed dependencies
func (p *GoDependencyParser) detectEmbeddedFiles(filePath string, content []byte) []*CrossLanguageDependency {
	deps := make([]*CrossLanguageDependency, 0)

	// Look for go:embed directives
	embedPattern := regexp.MustCompile(`(?m)^//go:embed\s+(.+)$`)
	matches := embedPattern.FindAllStringSubmatch(string(content), -1)

	for _, match := range matches {
		if len(match) > 1 {
			embeddedPath := strings.TrimSpace(match[1])
			targetPath := filepath.Join(filepath.Dir(filePath), embeddedPath)

			dep := &CrossLanguageDependency{
				SourceFile:     filePath,
				SourceLanguage: "go",
				TargetFile:     targetPath,
				TargetLanguage: p.detectLanguageFromPath(targetPath),
				RelationType:   "data",
				InterfaceType:  "file",
				Confidence:     0.95,
				Description:    fmt.Sprintf("Embedded file: %s", embeddedPath),
			}
			deps = append(deps, dep)
		}
	}

	return deps
}

// detectConfigReferences detects configuration file references
func (p *GoDependencyParser) detectConfigReferences(filePath string, content []byte) []*CrossLanguageDependency {
	deps := make([]*CrossLanguageDependency, 0)

	// Look for common config file patterns in string literals
	configPatterns := []*regexp.Regexp{
		regexp.MustCompile(`"([^"]*\.(?:yaml|yml|json|toml|ini|env|conf))"`),
		regexp.MustCompile(`"(config/[^"]+)"`),
		regexp.MustCompile(`"(\.env[^"]*)"`),
	}

	contentStr := string(content)

	for _, pattern := range configPatterns {
		matches := pattern.FindAllStringSubmatch(contentStr, -1)
		for _, match := range matches {
			if len(match) > 1 {
				configPath := match[1]
				targetPath := configPath

				// Try to resolve relative to current file
				if !filepath.IsAbs(configPath) {
					targetPath = filepath.Join(filepath.Dir(filePath), configPath)
				}

				dep := &CrossLanguageDependency{
					SourceFile:     filePath,
					SourceLanguage: "go",
					TargetFile:     targetPath,
					TargetLanguage: p.detectLanguageFromPath(targetPath),
					RelationType:   "configuration",
					InterfaceType:  "file",
					Confidence:     0.7,
					Description:    fmt.Sprintf("Configuration file reference: %s", configPath),
				}
				deps = append(deps, dep)
			}
		}
	}

	return deps
}

// detectLanguageFromPath detects language based on file extension
func (p *GoDependencyParser) detectLanguageFromPath(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))

	languageMap := map[string]string{
		".yaml": "yaml",
		".yml":  "yaml",
		".json": "json",
		".toml": "toml",
		".ini":  "ini",
		".env":  "env",
		".conf": "config",
		".h":    "c",
		".hpp":  "cpp",
		".c":    "c",
		".cpp":  "cpp",
		".js":   "javascript",
		".ts":   "typescript",
		".py":   "python",
	}

	if language, exists := languageMap[ext]; exists {
		return language
	}

	return "unknown"
}

// goSymbolVisitor implements ast.Visitor for Go symbol extraction
type goSymbolVisitor struct {
	filePath string
	symbols  *[]*SymbolDependency
	fileSet  *token.FileSet
}

// Visit implements ast.Visitor
func (v *goSymbolVisitor) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return nil
	}

	switch n := node.(type) {
	case *ast.FuncDecl:
		v.addFunction(n)
	case *ast.TypeSpec:
		v.addType(n)
	case *ast.GenDecl:
		v.addGenDecl(n)
	case *ast.ValueSpec:
		v.addVariable(n)
	}

	return v
}

// addFunction adds a function symbol
func (v *goSymbolVisitor) addFunction(fn *ast.FuncDecl) {
	if fn.Name == nil {
		return
	}

	symbol := &SymbolDependency{
		Symbol:          fn.Name.Name,
		Kind:            scip.SymbolInformation_Function,
		DefinitionFile:  v.filePath,
		DefinitionRange: v.positionToRange(fn.Pos(), fn.End()),
		References:      make([]*SymbolReference, 0),
		RelationType:    DependencyTypeReference,
		Confidence:      0.95,
	}

	*v.symbols = append(*v.symbols, symbol)
}

// addType adds a type symbol
func (v *goSymbolVisitor) addType(ts *ast.TypeSpec) {
	if ts.Name == nil {
		return
	}

	symbol := &SymbolDependency{
		Symbol:          ts.Name.Name,
		Kind:            scip.SymbolInformation_Class, // Using Class for types
		DefinitionFile:  v.filePath,
		DefinitionRange: v.positionToRange(ts.Pos(), ts.End()),
		References:      make([]*SymbolReference, 0),
		RelationType:    DependencyTypeReference,
		Confidence:      0.95,
	}

	*v.symbols = append(*v.symbols, symbol)
}

// addGenDecl adds symbols from general declarations
func (v *goSymbolVisitor) addGenDecl(gd *ast.GenDecl) {
	for _, spec := range gd.Specs {
		switch s := spec.(type) {
		case *ast.ValueSpec:
			v.addVariable(s)
		case *ast.TypeSpec:
			v.addType(s)
		}
	}
}

// addVariable adds a variable symbol
func (v *goSymbolVisitor) addVariable(vs *ast.ValueSpec) {
	for _, name := range vs.Names {
		if name == nil {
			continue
		}

		symbol := &SymbolDependency{
			Symbol:          name.Name,
			Kind:            scip.SymbolInformation_Variable,
			DefinitionFile:  v.filePath,
			DefinitionRange: v.positionToRange(name.Pos(), name.End()),
			References:      make([]*SymbolReference, 0),
			RelationType:    DependencyTypeReference,
			Confidence:      0.95,
		}

		*v.symbols = append(*v.symbols, symbol)
	}
}

// positionToRange converts AST positions to SCIP range
func (v *goSymbolVisitor) positionToRange(start, end token.Pos) *scip.Range {
	startPos := v.fileSet.Position(start)
	endPos := v.fileSet.Position(end)

	return &scip.Range{
		Start: scip.Position{
			Line:      int32(startPos.Line - 1), // SCIP uses 0-based line numbers
			Character: int32(startPos.Column - 1),
		},
		End: scip.Position{
			Line:      int32(endPos.Line - 1),
			Character: int32(endPos.Column - 1),
		},
	}
}

// PythonDependencyParser implements dependency parsing for Python language
type PythonDependencyParser struct {
	language string
}

// NewPythonDependencyParser creates a new Python dependency parser
func NewPythonDependencyParser() *PythonDependencyParser {
	return &PythonDependencyParser{
		language: "python",
	}
}

// ParseDependencies extracts import dependencies from Python files
func (p *PythonDependencyParser) ParseDependencies(filePath string, content []byte) ([]*DependencyEdge, error) {
	edges := make([]*DependencyEdge, 0)

	// Use regex patterns for Python imports
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?m)^import\s+([^\s#]+)`),
		regexp.MustCompile(`(?m)^from\s+([^\s#]+)\s+import`),
	}

	contentStr := string(content)

	for _, pattern := range patterns {
		matches := pattern.FindAllStringSubmatch(contentStr, -1)
		for _, match := range matches {
			if len(match) > 1 {
				importPath := strings.TrimSpace(match[1])

				// Skip standard library imports
				if p.isStandardLibrary(importPath) {
					continue
				}

				// Resolve import to file path
				targetPath := p.resolveImportPath(filePath, importPath)
				if targetPath == "" {
					continue
				}

				edge := &DependencyEdge{
					From:            filePath,
					To:              targetPath,
					DependencyType:  DependencyTypeImport,
					Strength:        1.0,
					Confidence:      0.85,
					ImportStatement: match[0],
					CreatedAt:       time.Now(),
					LastUpdated:     time.Now(),
					Source:          "python_regex_parser",
				}

				edges = append(edges, edge)
			}
		}
	}

	return edges, nil
}

// ParseSymbols extracts symbol definitions from Python files (simplified)
func (p *PythonDependencyParser) ParseSymbols(filePath string, content []byte) ([]*SymbolDependency, error) {
	symbols := make([]*SymbolDependency, 0)

	// Regex patterns for Python symbols
	patterns := map[*regexp.Regexp]scip.SymbolInformation_Kind{
		regexp.MustCompile(`(?m)^def\s+(\w+)`):   scip.SymbolInformation_Function,
		regexp.MustCompile(`(?m)^class\s+(\w+)`): scip.SymbolInformation_Class,
	}

	contentStr := string(content)
	lines := strings.Split(contentStr, "\n")

	for pattern, kind := range patterns {
		matches := pattern.FindAllStringSubmatch(contentStr, -1)
		for _, match := range matches {
			if len(match) > 1 {
				symbolName := match[1]

				// Find line number (simplified)
				lineNum := p.findLineNumber(lines, match[0])

				symbol := &SymbolDependency{
					Symbol:         symbolName,
					Kind:           kind,
					DefinitionFile: filePath,
					DefinitionRange: &scip.Range{
						Start: scip.Position{Line: int32(lineNum), Character: 0},
						End:   scip.Position{Line: int32(lineNum), Character: int32(len(match[0]))},
					},
					References:   make([]*SymbolReference, 0),
					RelationType: DependencyTypeReference,
					Confidence:   0.9,
				}

				symbols = append(symbols, symbol)
			}
		}
	}

	return symbols, nil
}

// DetectCrossLanguageDependencies identifies cross-language dependencies in Python files
func (p *PythonDependencyParser) DetectCrossLanguageDependencies(filePath string, content []byte) ([]*CrossLanguageDependency, error) {
	crossDeps := make([]*CrossLanguageDependency, 0)

	// Look for subprocess calls to other languages
	subprocessDeps := p.detectSubprocessCalls(filePath, content)
	crossDeps = append(crossDeps, subprocessDeps...)

	// Look for configuration file references
	configDeps := p.detectConfigReferences(filePath, content)
	crossDeps = append(crossDeps, configDeps...)

	return crossDeps, nil
}

// GetSupportedExtensions returns file extensions this parser supports
func (p *PythonDependencyParser) GetSupportedExtensions() []string {
	return []string{".py", ".pyi"}
}

// GetLanguage returns the language this parser handles
func (p *PythonDependencyParser) GetLanguage() string {
	return p.language
}

// Helper methods for Python parser

// isStandardLibrary checks if an import is from Python's standard library
func (p *PythonDependencyParser) isStandardLibrary(importPath string) bool {
	// Common standard library modules
	stdLibModules := map[string]bool{
		"os": true, "sys": true, "json": true, "re": true, "time": true,
		"datetime": true, "collections": true, "itertools": true, "functools": true,
		"math": true, "random": true, "string": true, "io": true, "pathlib": true,
		"urllib": true, "http": true, "email": true, "html": true, "xml": true,
		"sqlite3": true, "logging": true, "unittest": true, "threading": true,
		"multiprocessing": true, "subprocess": true, "socket": true, "ssl": true,
	}

	// Get the top-level module name
	topLevel := strings.Split(importPath, ".")[0]
	return stdLibModules[topLevel]
}

// resolveImportPath resolves a Python import path to a local file path
func (p *PythonDependencyParser) resolveImportPath(currentFile, importPath string) string {
	// This is a simplified implementation
	currentDir := filepath.Dir(currentFile)

	// Convert module path to file path
	modulePath := strings.ReplaceAll(importPath, ".", string(filepath.Separator))

	// Try with .py extension
	pyPath := filepath.Join(currentDir, modulePath+".py")

	// Try as package with __init__.py
	// initPath := filepath.Join(currentDir, modulePath, "__init__.py")

	// Return the first one that would exist (simplified)
	// In practice, you'd check if the file exists
	return pyPath
}

// detectSubprocessCalls detects subprocess calls to other languages
func (p *PythonDependencyParser) detectSubprocessCalls(filePath string, content []byte) []*CrossLanguageDependency {
	deps := make([]*CrossLanguageDependency, 0)

	// Look for subprocess patterns
	subprocessPattern := regexp.MustCompile(`subprocess\.(?:call|run|Popen)\([^)]*["']([^"']+)["']`)
	matches := subprocessPattern.FindAllStringSubmatch(string(content), -1)

	for _, match := range matches {
		if len(match) > 1 {
			command := match[1]
			targetLang := p.detectLanguageFromCommand(command)

			dep := &CrossLanguageDependency{
				SourceFile:     filePath,
				SourceLanguage: "python",
				TargetFile:     command,
				TargetLanguage: targetLang,
				RelationType:   "script",
				InterfaceType:  "subprocess",
				Confidence:     0.8,
				Description:    fmt.Sprintf("Subprocess call: %s", command),
			}
			deps = append(deps, dep)
		}
	}

	return deps
}

// detectConfigReferences detects configuration file references
func (p *PythonDependencyParser) detectConfigReferences(filePath string, content []byte) []*CrossLanguageDependency {
	deps := make([]*CrossLanguageDependency, 0)

	// Look for config file patterns
	configPattern := regexp.MustCompile(`["']([^"']*\.(?:yaml|yml|json|toml|ini|cfg|conf))["']`)
	matches := configPattern.FindAllStringSubmatch(string(content), -1)

	for _, match := range matches {
		if len(match) > 1 {
			configPath := match[1]
			targetPath := configPath

			if !filepath.IsAbs(configPath) {
				targetPath = filepath.Join(filepath.Dir(filePath), configPath)
			}

			dep := &CrossLanguageDependency{
				SourceFile:     filePath,
				SourceLanguage: "python",
				TargetFile:     targetPath,
				TargetLanguage: p.detectLanguageFromPath(targetPath),
				RelationType:   "configuration",
				InterfaceType:  "file",
				Confidence:     0.7,
				Description:    fmt.Sprintf("Configuration file: %s", configPath),
			}
			deps = append(deps, dep)
		}
	}

	return deps
}

// detectLanguageFromCommand detects language from command name
func (p *PythonDependencyParser) detectLanguageFromCommand(command string) string {
	command = strings.ToLower(command)

	if strings.Contains(command, "node") || strings.Contains(command, "npm") {
		return "javascript"
	}
	if strings.Contains(command, "go") {
		return "go"
	}
	if strings.Contains(command, "java") {
		return "java"
	}
	if strings.Contains(command, "cargo") || strings.Contains(command, "rust") {
		return "rust"
	}

	return "shell"
}

// detectLanguageFromPath detects language from file path
func (p *PythonDependencyParser) detectLanguageFromPath(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))

	languageMap := map[string]string{
		".yaml": "yaml",
		".yml":  "yaml",
		".json": "json",
		".toml": "toml",
		".ini":  "ini",
		".cfg":  "config",
		".conf": "config",
	}

	if language, exists := languageMap[ext]; exists {
		return language
	}

	return "unknown"
}

// findLineNumber finds the line number of a match in the content
func (p *PythonDependencyParser) findLineNumber(lines []string, match string) int {
	for i, line := range lines {
		if strings.Contains(line, match) {
			return i
		}
	}
	return 0
}

// TypeScriptDependencyParser implements dependency parsing for TypeScript/JavaScript
type TypeScriptDependencyParser struct {
	language string
}

// NewTypeScriptDependencyParser creates a new TypeScript dependency parser
func NewTypeScriptDependencyParser() *TypeScriptDependencyParser {
	return &TypeScriptDependencyParser{
		language: "typescript",
	}
}

// ParseDependencies extracts import dependencies from TypeScript/JavaScript files
func (p *TypeScriptDependencyParser) ParseDependencies(filePath string, content []byte) ([]*DependencyEdge, error) {
	edges := make([]*DependencyEdge, 0)

	// Import patterns for TypeScript/JavaScript
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`import\s+.*?\s+from\s+['"]([^'"]+)['"]`),
		regexp.MustCompile(`import\s+['"]([^'"]+)['"]`),
		regexp.MustCompile(`require\(['"]([^'"]+)['"]\)`),
		regexp.MustCompile(`import\(['"]([^'"]+)['"]\)`),
	}

	contentStr := string(content)

	for _, pattern := range patterns {
		matches := pattern.FindAllStringSubmatch(contentStr, -1)
		for _, match := range matches {
			if len(match) > 1 {
				importPath := strings.TrimSpace(match[1])

				// Skip node_modules imports (external dependencies)
				if p.isExternalDependency(importPath) {
					continue
				}

				// Resolve import to file path
				targetPath := p.resolveImportPath(filePath, importPath)
				if targetPath == "" {
					continue
				}

				edge := &DependencyEdge{
					From:            filePath,
					To:              targetPath,
					DependencyType:  DependencyTypeImport,
					Strength:        1.0,
					Confidence:      0.9,
					ImportStatement: match[0],
					CreatedAt:       time.Now(),
					LastUpdated:     time.Now(),
					Source:          "typescript_regex_parser",
				}

				edges = append(edges, edge)
			}
		}
	}

	return edges, nil
}

// ParseSymbols extracts symbol definitions from TypeScript/JavaScript files
func (p *TypeScriptDependencyParser) ParseSymbols(filePath string, content []byte) ([]*SymbolDependency, error) {
	symbols := make([]*SymbolDependency, 0)

	// Symbol patterns
	patterns := map[*regexp.Regexp]scip.SymbolInformation_Kind{
		regexp.MustCompile(`(?m)^(?:export\s+)?function\s+(\w+)`):  scip.SymbolInformation_Function,
		regexp.MustCompile(`(?m)^(?:export\s+)?class\s+(\w+)`):     scip.SymbolInformation_Class,
		regexp.MustCompile(`(?m)^(?:export\s+)?interface\s+(\w+)`): scip.SymbolInformation_Interface,
		regexp.MustCompile(`(?m)^(?:export\s+)?type\s+(\w+)`):      scip.SymbolInformation_Class,
		regexp.MustCompile(`(?m)^(?:export\s+)?const\s+(\w+)`):     scip.SymbolInformation_Constant,
		regexp.MustCompile(`(?m)^(?:export\s+)?let\s+(\w+)`):       scip.SymbolInformation_Variable,
		regexp.MustCompile(`(?m)^(?:export\s+)?var\s+(\w+)`):       scip.SymbolInformation_Variable,
	}

	contentStr := string(content)
	lines := strings.Split(contentStr, "\n")

	for pattern, kind := range patterns {
		matches := pattern.FindAllStringSubmatch(contentStr, -1)
		for _, match := range matches {
			if len(match) > 1 {
				symbolName := match[1]
				lineNum := p.findLineNumber(lines, match[0])

				symbol := &SymbolDependency{
					Symbol:         symbolName,
					Kind:           kind,
					DefinitionFile: filePath,
					DefinitionRange: &scip.Range{
						Start: scip.Position{Line: int32(lineNum), Character: 0},
						End:   scip.Position{Line: int32(lineNum), Character: int32(len(match[0]))},
					},
					References:   make([]*SymbolReference, 0),
					RelationType: DependencyTypeReference,
					Confidence:   0.9,
				}

				symbols = append(symbols, symbol)
			}
		}
	}

	return symbols, nil
}

// DetectCrossLanguageDependencies identifies cross-language dependencies
func (p *TypeScriptDependencyParser) DetectCrossLanguageDependencies(filePath string, content []byte) ([]*CrossLanguageDependency, error) {
	crossDeps := make([]*CrossLanguageDependency, 0)

	// Look for configuration files
	configDeps := p.detectConfigReferences(filePath, content)
	crossDeps = append(crossDeps, configDeps...)

	// Look for shell command executions
	shellDeps := p.detectShellCommands(filePath, content)
	crossDeps = append(crossDeps, shellDeps...)

	return crossDeps, nil
}

// GetSupportedExtensions returns file extensions this parser supports
func (p *TypeScriptDependencyParser) GetSupportedExtensions() []string {
	return []string{".ts", ".tsx", ".js", ".jsx"}
}

// GetLanguage returns the language this parser handles
func (p *TypeScriptDependencyParser) GetLanguage() string {
	return p.language
}

// Helper methods for TypeScript parser

// isExternalDependency checks if import is external dependency
func (p *TypeScriptDependencyParser) isExternalDependency(importPath string) bool {
	// Relative imports start with . or ..
	if strings.HasPrefix(importPath, ".") {
		return false
	}

	// Absolute paths or URLs
	if strings.HasPrefix(importPath, "/") || strings.Contains(importPath, "://") {
		return true
	}

	// npm packages don't start with . and don't contain /
	return !strings.HasPrefix(importPath, "./") && !strings.HasPrefix(importPath, "../")
}

// resolveImportPath resolves import path to file
func (p *TypeScriptDependencyParser) resolveImportPath(currentFile, importPath string) string {
	if p.isExternalDependency(importPath) {
		return ""
	}

	currentDir := filepath.Dir(currentFile)

	// Handle relative imports
	if strings.HasPrefix(importPath, ".") {
		resolved := filepath.Join(currentDir, importPath)

		// Try different extensions
		extensions := []string{".ts", ".tsx", ".js", ".jsx", "/index.ts", "/index.js"}
		for _, ext := range extensions {
			candidate := resolved + ext
			// In practice, you'd check if file exists
			return candidate
		}
	}

	return ""
}

// detectConfigReferences detects configuration file references
func (p *TypeScriptDependencyParser) detectConfigReferences(filePath string, content []byte) []*CrossLanguageDependency {
	deps := make([]*CrossLanguageDependency, 0)

	configPattern := regexp.MustCompile(`['"]([^'"]*\.(?:json|yaml|yml|toml|env))['"]`)
	matches := configPattern.FindAllStringSubmatch(string(content), -1)

	for _, match := range matches {
		if len(match) > 1 {
			configPath := match[1]
			targetPath := configPath

			if !filepath.IsAbs(configPath) {
				targetPath = filepath.Join(filepath.Dir(filePath), configPath)
			}

			dep := &CrossLanguageDependency{
				SourceFile:     filePath,
				SourceLanguage: p.language,
				TargetFile:     targetPath,
				TargetLanguage: p.detectLanguageFromPath(targetPath),
				RelationType:   "configuration",
				InterfaceType:  "file",
				Confidence:     0.7,
				Description:    fmt.Sprintf("Configuration file: %s", configPath),
			}
			deps = append(deps, dep)
		}
	}

	return deps
}

// detectShellCommands detects shell command executions
func (p *TypeScriptDependencyParser) detectShellCommands(filePath string, content []byte) []*CrossLanguageDependency {
	deps := make([]*CrossLanguageDependency, 0)

	// Look for child_process usage
	shellPatterns := []*regexp.Regexp{
		regexp.MustCompile(`exec\(['"]([^'"]+)['"]`),
		regexp.MustCompile(`spawn\(['"]([^'"]+)['"]`),
	}

	for _, pattern := range shellPatterns {
		matches := pattern.FindAllStringSubmatch(string(content), -1)
		for _, match := range matches {
			if len(match) > 1 {
				command := match[1]

				dep := &CrossLanguageDependency{
					SourceFile:     filePath,
					SourceLanguage: p.language,
					TargetFile:     command,
					TargetLanguage: "shell",
					RelationType:   "script",
					InterfaceType:  "subprocess",
					Confidence:     0.8,
					Description:    fmt.Sprintf("Shell command: %s", command),
				}
				deps = append(deps, dep)
			}
		}
	}

	return deps
}

// detectLanguageFromPath detects language from file path
func (p *TypeScriptDependencyParser) detectLanguageFromPath(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))

	languageMap := map[string]string{
		".json": "json",
		".yaml": "yaml",
		".yml":  "yaml",
		".toml": "toml",
		".env":  "env",
	}

	if language, exists := languageMap[ext]; exists {
		return language
	}

	return "unknown"
}

// findLineNumber finds line number of match
func (p *TypeScriptDependencyParser) findLineNumber(lines []string, match string) int {
	for i, line := range lines {
		if strings.Contains(line, match) {
			return i
		}
	}
	return 0
}

// RegisterLanguageParsers registers all language parsers with the dependency graph
func RegisterLanguageParsers(graph *DependencyGraph) error {
	// Register Go parser
	goParser := NewGoDependencyParser()
	graph.languageParsers["go"] = goParser

	// Register Python parser
	pythonParser := NewPythonDependencyParser()
	graph.languageParsers["python"] = pythonParser

	// Register TypeScript parser
	tsParser := NewTypeScriptDependencyParser()
	graph.languageParsers["typescript"] = tsParser
	graph.languageParsers["javascript"] = tsParser // Use same parser for JS

	return nil
}
