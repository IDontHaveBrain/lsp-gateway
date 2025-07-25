package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Simple standalone test for multi-language detection
// This doesn't depend on the complex config system

type SimpleLanguageInfo struct {
	Language    string
	FileCount   int
	Extensions  []string
	BuildFiles  []string
	ConfigFiles []string
}

type SimpleProjectInfo struct {
	RootPath    string
	Languages   map[string]*SimpleLanguageInfo
	ProjectType string
	ScanTime    time.Duration
}

func main() {
	logger := log.New(os.Stdout, "[STANDALONE-TEST] ", log.LstdFlags)
	
	if err := runStandaloneTest(logger); err != nil {
		logger.Printf("Test failed: %v", err)
		os.Exit(1)
	}
	
	logger.Printf("Standalone multi-language test completed successfully!")
}

func runStandaloneTest(logger *log.Logger) error {
	logger.Printf("Starting standalone multi-language detection test...")
	
	// Create test project
	tempDir, err := createStandaloneTestProject()
	if err != nil {
		return fmt.Errorf("failed to create test project: %w", err)
	}
	defer os.RemoveAll(tempDir)
	
	logger.Printf("Created test project at: %s", tempDir)
	
	// Scan project
	startTime := time.Now()
	projectInfo, err := scanProject(tempDir)
	if err != nil {
		return fmt.Errorf("failed to scan project: %w", err)
	}
	projectInfo.ScanTime = time.Since(startTime)
	
	// Print results
	logger.Printf("\n=== PROJECT SCAN RESULTS ===")
	logger.Printf("Root Path: %s", projectInfo.RootPath)
	logger.Printf("Project Type: %s", projectInfo.ProjectType)
	logger.Printf("Scan Duration: %v", projectInfo.ScanTime)
	logger.Printf("Languages Found: %d", len(projectInfo.Languages))
	
	for lang, info := range projectInfo.Languages {
		logger.Printf("\nLanguage: %s", lang)
		logger.Printf("  File Count: %d", info.FileCount)
		logger.Printf("  Extensions: %v", info.Extensions)
		logger.Printf("  Build Files: %v", info.BuildFiles)
		logger.Printf("  Config Files: %v", info.ConfigFiles)
	}
	
	// Validate expected languages
	expectedLanguages := []string{"go", "python", "typescript", "java", "rust"}
	for _, expected := range expectedLanguages {
		if _, found := projectInfo.Languages[expected]; found {
			logger.Printf("✓ Expected language %s detected", expected)
		} else {
			logger.Printf("✗ Expected language %s NOT detected", expected)
		}
	}
	
	// Test multi-language scenarios
	logger.Printf("\n=== MULTI-LANGUAGE SCENARIOS ===")
	
	// Scenario 1: Cross-language symbol search simulation
	logger.Printf("Simulating cross-language symbol search for 'main'...")
	for lang := range projectInfo.Languages {
		logger.Printf("  Would search for 'main' in %s files", lang)
	}
	
	// Scenario 2: Multi-server routing simulation
	logger.Printf("Simulating multi-server routing...")
	for lang := range projectInfo.Languages {
		serverName := getRecommendedLSPServer(lang)
		logger.Printf("  %s files → %s server", lang, serverName)
	}
	
	// Scenario 3: Project type classification
	projectType := classifyProject(projectInfo.Languages)
	logger.Printf("Classified project type: %s", projectType)
	
	return nil
}

func scanProject(rootPath string) (*SimpleProjectInfo, error) {
	languages := make(map[string]*SimpleLanguageInfo)
	
	// Language detection patterns
	langPatterns := map[string][]string{
		"go":         {".go"},
		"python":     {".py", ".pyx", ".pyi"},
		"typescript": {".ts", ".tsx"},
		"javascript": {".js", ".jsx"},
		"java":       {".java"},
		"rust":       {".rs"},
		"c":          {".c", ".h"},
		"cpp":        {".cpp", ".hpp", ".cc", ".cxx"},
		"csharp":     {".cs"},
		"ruby":       {".rb"},
		"php":        {".php"},
		"swift":      {".swift"},
	}
	
	buildFilePatterns := map[string][]string{
		"go":         {"go.mod", "go.sum", "go.work"},
		"python":     {"setup.py", "pyproject.toml", "requirements.txt", "Pipfile"},
		"typescript": {"tsconfig.json", "package.json"},
		"javascript": {"package.json", "yarn.lock"},
		"java":       {"pom.xml", "build.gradle", "build.gradle.kts"},
		"rust":       {"Cargo.toml", "Cargo.lock"},
		"c":          {"Makefile", "CMakeLists.txt"},
		"cpp":        {"Makefile", "CMakeLists.txt", "meson.build"},
		"csharp":     {"*.csproj", "*.sln"},
		"ruby":       {"Gemfile", "*.gemspec"},
		"php":        {"composer.json"},
		"swift":      {"Package.swift", "*.xcodeproj"},
	}
	
	// Walk through project directory
	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue on errors
		}
		
		if info.IsDir() {
			// Skip common ignore directories
			name := info.Name()
			if shouldIgnoreDir(name) {
				return filepath.SkipDir
			}
			return nil
		}
		
		filename := info.Name()
		ext := filepath.Ext(filename)
		
		// Check for source files
		for lang, extensions := range langPatterns {
			for _, langExt := range extensions {
				if ext == langExt {
					if languages[lang] == nil {
						languages[lang] = &SimpleLanguageInfo{
							Language:    lang,
							Extensions:  []string{},
							BuildFiles:  []string{},
							ConfigFiles: []string{},
						}
					}
					languages[lang].FileCount++
					
					// Add extension if not already present
					found := false
					for _, existing := range languages[lang].Extensions {
						if existing == langExt {
							found = true
							break
						}
					}
					if !found {
						languages[lang].Extensions = append(languages[lang].Extensions, langExt)
					}
				}
			}
		}
		
		// Check for build files
		for lang, buildFiles := range buildFilePatterns {
			for _, buildFile := range buildFiles {
				if strings.HasPrefix(buildFile, "*") {
					// Handle wildcard patterns like *.csproj
					pattern := strings.TrimPrefix(buildFile, "*")
					if strings.HasSuffix(filename, pattern) {
						if languages[lang] == nil {
							languages[lang] = &SimpleLanguageInfo{
								Language:    lang,
								Extensions:  []string{},
								BuildFiles:  []string{},
								ConfigFiles: []string{},
							}
						}
						languages[lang].BuildFiles = append(languages[lang].BuildFiles, filename)
					}
				} else {
					if filename == buildFile {
						if languages[lang] == nil {
							languages[lang] = &SimpleLanguageInfo{
								Language:    lang,
								Extensions:  []string{},
								BuildFiles:  []string{},
								ConfigFiles: []string{},
							}
						}
						languages[lang].BuildFiles = append(languages[lang].BuildFiles, filename)
					}
				}
			}
		}
		
		return nil
	})
	
	if err != nil {
		return nil, err
	}
	
	projectType := "single-language"
	if len(languages) > 1 {
		if len(languages) >= 3 {
			projectType = "monorepo"
		} else {
			projectType = "multi-language"
		}
	}
	
	return &SimpleProjectInfo{
		RootPath:    rootPath,
		Languages:   languages,
		ProjectType: projectType,
	}, nil
}

func shouldIgnoreDir(name string) bool {
	ignoreDirs := []string{
		"node_modules", ".git", ".svn", ".hg",
		"__pycache__", ".pytest_cache", ".mypy_cache",
		"target", "build", "dist", "out",
		".idea", ".vscode", "vendor",
		".gradle", ".m2",
	}
	
	for _, ignore := range ignoreDirs {
		if name == ignore {
			return true
		}
	}
	
	return strings.HasPrefix(name, ".")
}

func getRecommendedLSPServer(language string) string {
	servers := map[string]string{
		"go":         "gopls",
		"python":     "python-lsp-server",
		"typescript": "typescript-language-server",
		"javascript": "typescript-language-server",
		"java":       "eclipse-jdtls",
		"rust":       "rust-analyzer",
		"c":          "clangd",
		"cpp":        "clangd",
		"csharp":     "omnisharp",
		"ruby":       "solargraph",
		"php":        "intelephense",
		"swift":      "sourcekit-lsp",
	}
	
	if server, found := servers[language]; found {
		return server
	}
	
	return "generic-lsp"
}

func classifyProject(languages map[string]*SimpleLanguageInfo) string {
	if len(languages) == 0 {
		return "empty"
	}
	
	if len(languages) == 1 {
		return "single-language"
	}
	
	// Check for frontend-backend pattern
	frontendLangs := map[string]bool{
		"typescript": true,
		"javascript": true,
	}
	backendLangs := map[string]bool{
		"go":     true,
		"python": true,
		"java":   true,
		"rust":   true,
	}
	
	hasFrontend := false
	hasBackend := false
	
	for lang := range languages {
		if frontendLangs[lang] {
			hasFrontend = true
		}
		if backendLangs[lang] {
			hasBackend = true
		}
	}
	
	if hasFrontend && hasBackend {
		return "frontend-backend"
	}
	
	if len(languages) >= 3 {
		return "monorepo"
	}
	
	return "multi-language"
}

func createStandaloneTestProject() (string, error) {
	tempDir, err := os.MkdirTemp("", "standalone-test-*")
	if err != nil {
		return "", err
	}
	
	files := map[string]string{
		// Go
		"main.go": `package main
func main() { println("Hello Go") }`,
		"go.mod": `module test
go 1.21`,
		
		// Python
		"app.py": `def main():
    print("Hello Python")
if __name__ == "__main__":
    main()`,
		"setup.py": `from setuptools import setup
setup(name="test", version="1.0.0")`,
		
		// TypeScript
		"app.ts": `const msg: string = "Hello TypeScript";
console.log(msg);`,
		"tsconfig.json": `{
  "compilerOptions": {
    "target": "es2020"
  }
}`,
		"package.json": `{
  "name": "test",
  "version": "1.0.0"
}`,
		
		// Java
		"Main.java": `public class Main {
    public static void main(String[] args) {
        System.out.println("Hello Java");
    }
}`,
		"pom.xml": `<?xml version="1.0"?>
<project>
    <modelVersion>4.0.0</modelVersion>
    <groupId>test</groupId>
    <artifactId>test</artifactId>
    <version>1.0.0</version>
</project>`,
		
		// Rust
		"src/main.rs": `fn main() {
    println!("Hello Rust");
}`,
		"Cargo.toml": `[package]
name = "test"
version = "1.0.0"
edition = "2021"`,
	}
	
	for path, content := range files {
		fullPath := filepath.Join(tempDir, path)
		dir := filepath.Dir(fullPath)
		
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", err
		}
		
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return "", err
		}
	}
	
	return tempDir, nil
}