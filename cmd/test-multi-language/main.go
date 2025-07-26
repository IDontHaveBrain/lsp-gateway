package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/gateway"
	"lsp-gateway/mcp"
)

func main() {
	// Create logger
	logger := log.New(os.Stdout, "[MULTI-LANG-TEST] ", log.LstdFlags)

	// Test multi-language functionality
	if err := testMultiLanguageSupport(logger); err != nil {
		logger.Printf("Test failed: %v", err)
		os.Exit(1)
	}

	logger.Printf("All multi-language tests passed successfully!")
}

func testMultiLanguageSupport(logger *log.Logger) error {
	logger.Printf("Starting multi-language LSP Gateway tests...")

	// 1. Test project language detection
	if err := testProjectLanguageDetection(logger); err != nil {
		return fmt.Errorf("project language detection test failed: %w", err)
	}

	// 2. Test multi-language configuration generation
	if err := testMultiLanguageConfigGeneration(logger); err != nil {
		return fmt.Errorf("config generation test failed: %w", err)
	}

	// 3. Test project language scanning
	if err := testProjectLanguageScanning(logger); err != nil {
		return fmt.Errorf("project scanning test failed: %w", err)
	}

	// 4. Test smart routing system
	if err := testSmartRouting(logger); err != nil {
		return fmt.Errorf("smart routing test failed: %w", err)
	}

	return nil
}

func testProjectLanguageDetection(logger *log.Logger) error {
	logger.Printf("Testing project language detection...")

	// Create a temporary multi-language test project
	tempDir, err := createTestProject()
	if err != nil {
		return fmt.Errorf("failed to create test project: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			logger.Printf("Warning: failed to clean up temp directory %s: %v", tempDir, err)
		}
	}()

	// Initialize project language scanner
	scanner := gateway.NewProjectLanguageScanner()

	// Scan the test project
	projectInfo, err := scanner.ScanProjectComprehensive(tempDir)
	if err != nil {
		return fmt.Errorf("failed to scan project: %w", err)
	}

	// Verify results
	expectedLanguages := []string{"go", mcp.LANG_PYTHON, mcp.LANG_TYPESCRIPT, mcp.LANG_JAVA}

	logger.Printf("Detected project type: %s", projectInfo.ProjectType)
	logger.Printf("Detected %d languages:", len(projectInfo.Languages))

	for lang, ctx := range projectInfo.Languages {
		logger.Printf("  - %s: %d files, priority %d, confidence %.2f",
			lang, ctx.FileCount, ctx.Priority, ctx.Confidence)

		// Check if expected language
		found := false
		for _, expected := range expectedLanguages {
			if lang == expected {
				found = true
				break
			}
		}
		if !found {
			logger.Printf("    Warning: unexpected language detected: %s", lang)
		}
	}

	// Verify minimum expected languages are detected
	for _, expected := range expectedLanguages {
		if _, exists := projectInfo.Languages[expected]; !exists {
			return fmt.Errorf("expected language %s not detected", expected)
		}
	}

	logger.Printf("✓ Project language detection test passed")
	return nil
}

func testMultiLanguageConfigGeneration(logger *log.Logger) error {
	logger.Printf("Testing multi-language configuration generation...")

	// Create test project info using config types
	var languageContexts []*config.LanguageContext
	languages := []string{"go", mcp.LANG_PYTHON, mcp.LANG_TYPESCRIPT}

	for i, lang := range languages {
		ctx := &config.LanguageContext{
			Language:       lang,
			RootPath:       fmt.Sprintf("/test/project/%s", lang),
			FileCount:      10 + i*5,
			FilePatterns:   []string{fmt.Sprintf("*.%s", lang)},
			RootMarkers:    []string{fmt.Sprintf("%s-marker", lang)},
			BuildSystem:    fmt.Sprintf("%s-build", lang),
			PackageManager: fmt.Sprintf("%s-package-manager", lang),
			Frameworks:     []string{fmt.Sprintf("%s-framework", lang)},
		}
		languageContexts = append(languageContexts, ctx)
	}

	projectInfo := &config.MultiLanguageProjectInfo{
		ProjectType:      "monorepo",
		RootDirectory:    "/test/project",
		WorkspaceRoot:    "/test/project",
		LanguageContexts: languageContexts,
		DetectedAt:       time.Now(),
		Metadata:         make(map[string]interface{}),
	}

	// Generate configuration
	generator := config.NewConfigGenerator()
	mlConfig, err := generator.GenerateMultiLanguageConfig(projectInfo)
	if err != nil {
		return fmt.Errorf("failed to generate config: %w", err)
	}

	// Verify configuration
	if len(mlConfig.ServerConfigs) != len(languages) {
		return fmt.Errorf("expected %d server configs, got %d", len(languages), len(mlConfig.ServerConfigs))
	}

	supportedLangs := mlConfig.GetSupportedLanguages()
	logger.Printf("Generated configuration for languages: %v", supportedLangs)

	// Convert to gateway config
	gatewayConfig, err := mlConfig.ToGatewayConfig()
	if err != nil {
		return fmt.Errorf("failed to convert to gateway config: %w", err)
	}

	if !gatewayConfig.EnableConcurrentServers {
		return fmt.Errorf("concurrent servers should be enabled for multi-language config")
	}

	logger.Printf("✓ Multi-language configuration generation test passed")
	return nil
}

func testProjectLanguageScanning(logger *log.Logger) error {
	logger.Printf("Testing project language scanning performance...")

	// Create scanner with different optimization modes
	scanner := gateway.NewProjectLanguageScanner()

	// Test small project optimization
	scanner.OptimizeForSmallProjects()
	perfMetrics := scanner.GetPerformanceMetrics()

	logger.Printf("Small project optimization metrics:")
	if cache, ok := perfMetrics["configuration"].(map[string]interface{}); ok {
		logger.Printf("  - Max depth: %v", cache["max_depth"])
		logger.Printf("  - Max files: %v", cache["max_files"])
		logger.Printf("  - Cache enabled: %v", cache["cache_enabled"])
	}

	// Test large monorepo optimization
	scanner.OptimizeForLargeMonorepos()
	perfMetrics = scanner.GetPerformanceMetrics()

	logger.Printf("Large monorepo optimization metrics:")
	if cache, ok := perfMetrics["configuration"].(map[string]interface{}); ok {
		logger.Printf("  - Max depth: %v", cache["max_depth"])
		logger.Printf("  - Max files: %v", cache["max_files"])
		logger.Printf("  - Timeout: %v", cache["timeout"])
	}

	// Test cache functionality
	scanner.SetCacheEnabled(true)
	cacheStats := scanner.GetCacheStats()
	if cacheStats != nil {
		logger.Printf("Cache stats: hit_ratio=%.2f, entries=%d",
			cacheStats.HitRatio, cacheStats.TotalEntries)
	}

	logger.Printf("✓ Project language scanning test passed")
	return nil
}

func testSmartRouting(logger *log.Logger) error {
	logger.Printf("Testing smart routing system...")

	// Create minimal gateway config for testing
	gatewayConfig := &config.GatewayConfig{
		Port:                    8080,
		Timeout:                 "30s",
		MaxConcurrentRequests:   100,
		EnableConcurrentServers: true,
		Servers: []config.ServerConfig{
			{
				Name:      "go-server",
				Languages: []string{"go"},
				Command:   "gopls",
				Transport: "stdio",
			},
			{
				Name:      "python-server",
				Languages: []string{mcp.LANG_PYTHON},
				Command:   "python",
				Args:      []string{"-m", "pylsp"},
				Transport: "stdio",
			},
		},
	}

	// Create multi-language integrator
	integrator := gateway.NewMultiLanguageIntegrator(gatewayConfig, logger)

	// Test project detection
	tempDir, err := createTestProject()
	if err != nil {
		return fmt.Errorf("failed to create test project: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			logger.Printf("Warning: failed to clean up temp directory %s: %v", tempDir, err)
		}
	}()

	projectInfo, err := integrator.DetectAndConfigureProject(tempDir)
	if err != nil {
		return fmt.Errorf("failed to detect and configure project: %w", err)
	}

	logger.Printf("Configured project with %d languages", len(projectInfo.Languages))

	// Test routing strategies
	strategies := []gateway.RoutingStrategyType{
		gateway.RoutingStrategySingle,
		gateway.RoutingStrategyMulti,
		gateway.RoutingStrategyAggregate,
		gateway.RoutingStrategyLoadBalanced,
	}

	for _, strategy := range strategies {
		logger.Printf("Testing routing strategy: %s", strategy)
		// Note: In a real test, we would create actual LSP requests
		// and verify routing behavior, but for this demo we just log
	}

	// Test metrics collection
	metrics, err := integrator.GetMultiLanguageMetrics()
	if err != nil {
		return fmt.Errorf("failed to get metrics: %w", err)
	}

	logger.Printf("Multi-language metrics:")
	logger.Printf("  - Total projects: %d", metrics.TotalProjects)
	logger.Printf("  - Active languages: %v", metrics.ActiveLanguages)

	logger.Printf("✓ Smart routing test passed")
	return nil
}

func createTestProject() (string, error) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "lsp-gateway-test-*")
	if err != nil {
		return "", err
	}

	// Create test files for different languages
	testFiles := map[string]string{
		"main.go": `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}`,
		"go.mod": `module test-project

go 1.21`,
		"src/main.py": `#!/usr/bin/env python3

def main():
	print("Hello, World!")

if __name__ == "__main__":
	main()`,
		"setup.py": `from setuptools import setup, find_packages

setup(
	name="test-project",
	version="1.0.0",
	packages=find_packages(),
)`,
		"frontend/app.ts": `interface User {
	name: string;
	email: string;
}

const user: User = {
	name: "Test User",
	email: "test@example.com"
};

console.log(user);`,
		"frontend/package.json": `{
	"name": "test-frontend",
	"version": "1.0.0",
	"dependencies": {
		"typescript": "^5.0.0"
	}
}`,
		"frontend/tsconfig.json": `{
	"compilerOptions": {
		"target": "es2020",
		"module": "commonjs",
		"strict": true
	}
}`,
		"backend/Main.java": `public class Main {
	public static void main(String[] args) {
		System.out.println("Hello, World!");
	}
}`,
		"backend/pom.xml": `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
	<modelVersion>4.0.0</modelVersion>
	<groupId>com.example</groupId>
	<artifactId>test-backend</artifactId>
	<version>1.0.0</version>
</project>`,
		"README.md": `# Test Multi-Language Project

This is a test project demonstrating multi-language LSP support.

## Languages Included
- Go (main.go, go.mod)
- Python (src/main.py, setup.py)  
- TypeScript (frontend/app.ts, tsconfig.json)
- Java (backend/Main.java, pom.xml)`,
	}

	// Create files and directories
	for filePath, content := range testFiles {
		fullPath := filepath.Join(tempDir, filePath)

		// Create directory if needed
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", err
		}

		// Write file
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return "", err
		}
	}

	return tempDir, nil
}

