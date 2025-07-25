package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"lsp-gateway/internal/gateway"
)

func main() {
	logger := log.New(os.Stdout, "[SIMPLE-MULTI-TEST] ", log.LstdFlags)

	if err := runSimpleTests(logger); err != nil {
		logger.Printf("Tests failed: %v", err)
		os.Exit(1)
	}

	logger.Printf("All tests passed successfully!")
}

func runSimpleTests(logger *log.Logger) error {
	logger.Printf("Starting simple multi-language tests...")

	// 1. Test project language scanner
	if err := testProjectScanner(logger); err != nil {
		return fmt.Errorf("project scanner test failed: %w", err)
	}

	// 2. Test temporary project creation and detection
	if err := testTempProjectDetection(logger); err != nil {
		return fmt.Errorf("temp project detection test failed: %w", err)
	}

	return nil
}

func testProjectScanner(logger *log.Logger) error {
	logger.Printf("Testing ProjectLanguageScanner...")

	// Create scanner
	scanner := gateway.NewProjectLanguageScanner()

	// Test configuration
	scanner.SetMaxDepth(3)
	scanner.SetMaxFiles(1000)
	scanner.SetCacheEnabled(true)

	// Test validation
	if err := scanner.ValidateConfiguration(); err != nil {
		return fmt.Errorf("scanner validation failed: %w", err)
	}

	// Test supported languages
	languages := scanner.GetSupportedLanguages()
	logger.Printf("Supported languages: %v", languages)

	if len(languages) == 0 {
		return fmt.Errorf("no supported languages found")
	}

	// Test performance metrics
	metrics := scanner.GetPerformanceMetrics()
	if metrics == nil {
		return fmt.Errorf("failed to get performance metrics")
	}

	logger.Printf("Scanner configuration is valid")
	return nil
}

func testTempProjectDetection(logger *log.Logger) error {
	logger.Printf("Testing temporary project detection...")

	// Create temporary test project
	tempDir, err := createMinimalTestProject()
	if err != nil {
		return fmt.Errorf("failed to create test project: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			logger.Printf("Warning: failed to clean up temp directory %s: %v", tempDir, err)
		}
	}()

	logger.Printf("Created test project at: %s", tempDir)

	// Initialize scanner
	scanner := gateway.NewProjectLanguageScanner()

	// Scan project
	projectInfo, err := scanner.ScanProject(tempDir)
	if err != nil {
		return fmt.Errorf("failed to scan project: %w", err)
	}

	// Print results
	logger.Printf("Project detection results:")
	logger.Printf("  Root path: %s", projectInfo.RootPath)
	logger.Printf("  Project type: %s", projectInfo.ProjectType)
	logger.Printf("  Dominant language: %s", projectInfo.DominantLanguage)
	logger.Printf("  Total files: %d", projectInfo.TotalFileCount)
	logger.Printf("  Scan duration: %v", projectInfo.ScanDuration)
	logger.Printf("  Languages found: %d", len(projectInfo.Languages))

	for lang, ctx := range projectInfo.Languages {
		logger.Printf("    - %s: %d files, priority %d, confidence %.2f",
			lang, ctx.FileCount, ctx.Priority, ctx.Confidence)

		if ctx.Framework != "" {
			logger.Printf("      Framework: %s", ctx.Framework)
		}
		if ctx.LSPServerName != "" {
			logger.Printf("      LSP Server: %s", ctx.LSPServerName)
		}
	}

	// Validate results
	if len(projectInfo.Languages) == 0 {
		return fmt.Errorf("no languages detected in test project")
	}

	// Test project info methods
	primaryLanguages := projectInfo.GetPrimaryLanguages()
	secondaryLanguages := projectInfo.GetSecondaryLanguages()

	logger.Printf("Primary languages: %v", primaryLanguages)
	logger.Printf("Secondary languages: %v", secondaryLanguages)

	// Test language checking
	for lang := range projectInfo.Languages {
		if !projectInfo.HasLanguage(lang) {
			return fmt.Errorf("HasLanguage failed for detected language %s", lang)
		}

		ctx := projectInfo.GetLanguageContext(lang)
		if ctx == nil {
			return fmt.Errorf("GetLanguageContext failed for language %s", lang)
		}
	}

	// Test workspace roots
	for lang := range projectInfo.Languages {
		root := projectInfo.GetWorkspaceRoot(lang)
		if root == "" {
			return fmt.Errorf("GetWorkspaceRoot failed for language %s", lang)
		}
	}

	// Test polyglot detection
	isPolyglot := projectInfo.IsPolyglot()
	logger.Printf("Is polyglot project: %t", isPolyglot)

	// Test validation
	if err := projectInfo.Validate(); err != nil {
		return fmt.Errorf("project info validation failed: %w", err)
	}

	// Test JSON serialization
	jsonData, err := json.MarshalIndent(projectInfo, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal project info to JSON: %w", err)
	}

	logger.Printf("Project info JSON size: %d bytes", len(jsonData))

	return nil
}

func createMinimalTestProject() (string, error) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "lsp-test-*")
	if err != nil {
		return "", err
	}

	// Create test files for multiple languages
	files := map[string]string{
		"main.go": `package main

import "fmt"

func main() {
	fmt.Println("Hello from Go!")
}`,
		"go.mod": `module test-project

go 1.21`,
		"app.py": `#!/usr/bin/env python3

def main():
    print("Hello from Python!")

if __name__ == "__main__":
    main()`,
		"setup.py": `from setuptools import setup

setup(
    name="test-app",
    version="1.0.0",
)`,
		"app.ts": `interface Greeting {
    message: string;
}

const greeting: Greeting = {
    message: "Hello from TypeScript!"
};

console.log(greeting.message);`,
		"package.json": `{
    "name": "test-app",
    "version": "1.0.0",
    "dependencies": {
        "typescript": "^5.0.0"
    }
}`,
		"tsconfig.json": `{
    "compilerOptions": {
        "target": "es2020",
        "module": "commonjs",
        "strict": true
    }
}`,
		"Main.java": `public class Main {
    public static void main(String[] args) {
        System.out.println("Hello from Java!");
    }
}`,
		"pom.xml": `<?xml version="1.0" encoding="UTF-8"?>
<project>
    <modelVersion>4.0.0</modelVersion>
    <groupId>com.example</groupId>
    <artifactId>test-app</artifactId>
    <version>1.0.0</version>
</project>`,
		"README.md": `# Multi-Language Test Project

This project contains files in multiple programming languages:
- Go (main.go, go.mod)
- Python (app.py, setup.py)
- TypeScript (app.ts, tsconfig.json, package.json)
- Java (Main.java, pom.xml)

This is used for testing the multi-language LSP Gateway functionality.`,
	}

	// Write all files
	for filename, content := range files {
		filePath := filepath.Join(tempDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return "", fmt.Errorf("failed to write file %s: %w", filename, err)
		}
	}

	return tempDir, nil
}
