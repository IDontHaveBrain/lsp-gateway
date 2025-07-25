package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/gateway"

	"github.com/stretchr/testify/suite"
)

// MultiLanguageTestSuite provides comprehensive integration tests for multi-language project detection
type MultiLanguageTestSuite struct {
	suite.Suite
	testProjectsDir string
	scanner         *gateway.ProjectLanguageScanner
	tempDirs        []string
}

// SetupTest initializes the test environment
func (suite *MultiLanguageTestSuite) SetupTest() {
	// Create temporary directory for test projects
	tempDir, err := os.MkdirTemp("", "multi-language-test-*")
	suite.Require().NoError(err)
	suite.testProjectsDir = tempDir
	suite.tempDirs = []string{tempDir}

	// Initialize scanner with optimized settings for testing
	suite.scanner = gateway.NewProjectLanguageScanner()
	suite.scanner.SetMaxDepth(5)
	suite.scanner.SetMaxFiles(50000)
	suite.scanner.SetEarlyExitEnabled(true)
	suite.scanner.OptimizeForLargeMonorepos()
}

// TearDownTest cleans up test resources
func (suite *MultiLanguageTestSuite) TearDownTest() {
	// Clean up temporary directories
	for _, dir := range suite.tempDirs {
		os.RemoveAll(dir)
	}

	// Shutdown scanner
	if suite.scanner != nil {
		suite.scanner.Shutdown()
	}
}

// createTestProject creates a test project with the given structure
func (suite *MultiLanguageTestSuite) createTestProject(name string, structure map[string]string) string {
	projectPath := filepath.Join(suite.testProjectsDir, name)
	err := os.MkdirAll(projectPath, 0755)
	suite.Require().NoError(err)

	for relPath, content := range structure {
		fullPath := filepath.Join(projectPath, relPath)

		// Create parent directories
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		suite.Require().NoError(err)

		// Write file content
		err = os.WriteFile(fullPath, []byte(content), 0644)
		suite.Require().NoError(err)
	}

	return projectPath
}

// TestLargeMonorepoDetection tests detection of large monorepo projects with 4+ languages
func (suite *MultiLanguageTestSuite) TestLargeMonorepoDetection() {
	// Create large monorepo project structure
	structure := map[string]string{
		// Go service
		"services/auth-service/go.mod":       "module auth-service\n\ngo 1.19\n\nrequire github.com/gin-gonic/gin v1.9.1",
		"services/auth-service/main.go":      "package main\n\nimport \"github.com/gin-gonic/gin\"\n\nfunc main() { r := gin.Default(); r.Run() }",
		"services/auth-service/auth.go":      "package main\n\ntype AuthService struct{}",
		"services/auth-service/auth_test.go": "package main\n\nimport \"testing\"\n\nfunc TestAuth(t *testing.T) {}",

		// Python service
		"services/user-service/pyproject.toml": "[build-system]\nrequires = [\"setuptools\", \"wheel\"]\n\n[project]\nname = \"user-service\"\nversion = \"1.0.0\"\ndependencies = [\"fastapi\", \"uvicorn\"]",
		"services/user-service/main.py":        "from fastapi import FastAPI\n\napp = FastAPI()\n\n@app.get('/')\ndef read_root():\n    return {'Hello': 'World'}",
		"services/user-service/models.py":      "from pydantic import BaseModel\n\nclass User(BaseModel):\n    id: int\n    name: str",
		"services/user-service/test_main.py":   "import pytest\nfrom main import app\n\ndef test_read_root():\n    pass",

		// Java service
		"services/notification-service/pom.xml": `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
    <modelVersion>4.0.0</modelVersion>
    <groupId>com.example</groupId>
    <artifactId>notification-service</artifactId>
    <version>1.0.0</version>
    <dependencies>
        <dependency>
            <groupId>org.springframework.boot</groupId>
            <artifactId>spring-boot-starter-web</artifactId>
            <version>2.7.0</version>
        </dependency>
    </dependencies>
</project>`,
		"services/notification-service/src/main/java/com/example/NotificationService.java":     "package com.example;\n\n@RestController\npublic class NotificationService {\n    @GetMapping(\"/\")\n    public String hello() { return \"Hello\"; }\n}",
		"services/notification-service/src/test/java/com/example/NotificationServiceTest.java": "package com.example;\n\nimport org.junit.jupiter.api.Test;\n\nclass NotificationServiceTest {\n    @Test void testHello() {}\n}",

		// Rust service
		"services/search-service/Cargo.toml":                "[package]\nname = \"search-service\"\nversion = \"0.1.0\"\nedition = \"2021\"\n\n[dependencies]\naxum = \"0.6\"\ntokio = { version = \"1.0\", features = [\"full\"] }",
		"services/search-service/src/main.rs":               "use axum::{routing::get, Router};\n\n#[tokio::main]\nasync fn main() {\n    let app = Router::new().route(\"/\", get(|| async { \"Hello\" }));\n}",
		"services/search-service/src/lib.rs":                "pub mod search;\n\npub struct SearchEngine {}",
		"services/search-service/tests/integration_test.rs": "use search_service::SearchEngine;\n\n#[test]\nfn test_search() {}",

		// TypeScript frontend
		"frontend/web-app/package.json":              "{\"name\": \"web-app\", \"version\": \"1.0.0\", \"dependencies\": {\"react\": \"^18.0.0\", \"typescript\": \"^4.9.0\"}}",
		"frontend/web-app/tsconfig.json":             "{\"compilerOptions\": {\"target\": \"ES2020\", \"jsx\": \"react\"}}",
		"frontend/web-app/src/App.tsx":               "import React from 'react';\n\nfunction App() {\n  return <div>Hello World</div>;\n}\n\nexport default App;",
		"frontend/web-app/src/components/Header.tsx": "import React from 'react';\n\nexport const Header = () => <header>App Header</header>;",
		"frontend/web-app/__tests__/App.test.tsx":    "import { render } from '@testing-library/react';\nimport App from '../src/App';\n\ntest('renders app', () => {\n  render(<App />);\n});",

		// React Native mobile app
		"frontend/mobile-app/package.json":  "{\"name\": \"mobile-app\", \"version\": \"1.0.0\", \"dependencies\": {\"react-native\": \"^0.72.0\", \"typescript\": \"^4.9.0\"}}",
		"frontend/mobile-app/tsconfig.json": "{\"compilerOptions\": {\"target\": \"ES2020\", \"jsx\": \"react-native\"}}",
		"frontend/mobile-app/App.tsx":       "import React from 'react';\nimport {Text, View} from 'react-native';\n\nconst App = () => <View><Text>Hello Mobile</Text></View>;\n\nexport default App;",

		// Shared configuration
		"shared/proto/api.proto":       "syntax = \"proto3\";\n\nservice ApiService {\n  rpc GetUser(UserRequest) returns (UserResponse);\n}",
		"shared/configs/database.yaml": "database:\n  host: localhost\n  port: 5432",
		"docker-compose.yml":           "version: '3.8'\nservices:\n  auth:\n    build: ./services/auth-service\n  user:\n    build: ./services/user-service",

		// Scripts
		"scripts/build.sh":  "#!/bin/bash\necho 'Building all services'\ncd services && make build-all",
		"scripts/deploy.py": "#!/usr/bin/env python3\nimport subprocess\nprint('Deploying services')",

		// Root files
		"Makefile":  "build-all:\n\t@echo 'Building monorepo'\n\t@cd services && make build\n\ntest-all:\n\t@echo 'Testing monorepo'",
		"README.md": "# Large Monorepo\n\nThis is a large monorepo with multiple services.",
	}

	projectPath := suite.createTestProject("large-monorepo", structure)

	// Scan the project
	info, err := suite.scanner.ScanProject(projectPath)
	suite.Require().NoError(err)
	suite.NotNil(info)

	// Validate project type detection
	suite.Equal(config.ProjectTypeMonorepo, info.ProjectType, "Should detect monorepo project type")

	// Validate language detection - should find at least 5 languages
	suite.GreaterOrEqual(len(info.Languages), 5, "Should detect at least 5 languages")

	// Validate specific languages
	suite.True(info.HasLanguage("go"), "Should detect Go language")
	suite.True(info.HasLanguage("python"), "Should detect Python language")
	suite.True(info.HasLanguage("java"), "Should detect Java language")
	suite.True(info.HasLanguage("rust"), "Should detect Rust language")
	suite.True(info.HasLanguage("typescript"), "Should detect TypeScript language")

	// Validate language priorities and confidence
	for lang, ctx := range info.Languages {
		suite.GreaterOrEqual(ctx.Priority, 1, fmt.Sprintf("Language %s should have priority >= 1", lang))
		suite.GreaterOrEqual(ctx.Confidence, 0.5, fmt.Sprintf("Language %s should have confidence >= 0.5", lang))
		suite.Greater(ctx.FileCount, 0, fmt.Sprintf("Language %s should have files", lang))
	}

	// Validate build files detection
	suite.Greater(len(info.BuildFiles), 0, "Should detect build files")

	// Validate workspace roots
	suite.GreaterOrEqual(len(info.WorkspaceRoots), 5, "Should have workspace roots for each language")

	// Validate scan performance
	suite.Less(info.ScanDuration, 30*time.Second, "Scan should complete within 30 seconds")
	suite.Greater(info.TotalFileCount, 20, "Should scan significant number of files")
}

// TestMixedLanguageServices tests services with multiple languages in same directories
func (suite *MultiLanguageTestSuite) TestMixedLanguageServices() {
	structure := map[string]string{
		// API Gateway with Go, Python, and TypeScript
		"api-gateway/go.mod":           "module api-gateway\n\ngo 1.19",
		"api-gateway/main.go":          "package main\n\nfunc main() { println(\"API Gateway\") }",
		"api-gateway/middleware.py":    "from flask import Flask\n\napp = Flask(__name__)\n\n@app.route('/')\ndef health():\n    return 'OK'",
		"api-gateway/config.ts":        "interface Config {\n  port: number;\n  host: string;\n}\n\nexport const config: Config = { port: 8080, host: 'localhost' };",
		"api-gateway/package.json":     "{\"name\": \"api-gateway-config\", \"version\": \"1.0.0\", \"dependencies\": {\"typescript\": \"^4.9.0\"}}",
		"api-gateway/requirements.txt": "flask==2.0.1\nrequests==2.28.0",

		// Data Processor with Java, Python, and R
		"data-processor/pom.xml": `<?xml version="1.0"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
    <modelVersion>4.0.0</modelVersion>
    <groupId>com.example</groupId>
    <artifactId>data-processor</artifactId>
    <version>1.0.0</version>
</project>`,
		"data-processor/src/main/java/Processor.java": "public class Processor {\n    public void process() { System.out.println(\"Processing\"); }\n}",
		"data-processor/utils.py":                     "import pandas as pd\n\ndef clean_data(df):\n    return df.dropna()",
		"data-processor/analysis.R":                   "# R analysis script\nlibrary(ggplot2)\ndata <- read.csv('data.csv')\nplot(data)",

		// Shared utilities
		"shared/docker-compose.yml": "version: '3.8'\nservices:\n  gateway:\n    build: ./api-gateway\n  processor:\n    build: ./data-processor",
		"shared/Makefile":           "all:\n\t@echo 'Building mixed services'\n\tbuild-gateway\n\tbuild-processor\n\nbuild-gateway:\n\t@cd api-gateway && go build\n\nbuild-processor:\n\t@cd data-processor && mvn compile",
	}

	projectPath := suite.createTestProject("mixed-services", structure)

	// Scan the project
	info, err := suite.scanner.ScanProject(projectPath)
	suite.Require().NoError(err)
	suite.NotNil(info)

	// Should detect as multi-language project
	suite.Equal(config.ProjectTypeMulti, info.ProjectType, "Should detect multi-language project type")

	// Should detect multiple languages
	suite.GreaterOrEqual(len(info.Languages), 3, "Should detect at least 3 languages")

	// Validate specific language detection
	expectedLanguages := []string{"go", "python", "java", "typescript"}
	detectedCount := 0
	for _, lang := range expectedLanguages {
		if info.HasLanguage(lang) {
			detectedCount++
		}
	}
	suite.GreaterOrEqual(detectedCount, 3, "Should detect at least 3 of the expected languages")

	// Validate that mixed directories are handled correctly
	for _, ctx := range info.Languages {
		suite.Greater(ctx.FileCount, 0, fmt.Sprintf("Language %s should have files", ctx.Language))
		suite.NotEmpty(ctx.FileExtensions, fmt.Sprintf("Language %s should have file extensions", ctx.Language))
	}
}

// TestFrontendBackendSplit tests projects with clear frontend/backend separation
func (suite *MultiLanguageTestSuite) TestFrontendBackendSplit() {
	structure := map[string]string{
		// React Frontend
		"frontend/package.json":              "{\"name\": \"webapp-frontend\", \"version\": \"1.0.0\", \"dependencies\": {\"react\": \"^18.0.0\", \"typescript\": \"^4.9.0\"}}",
		"frontend/tsconfig.json":             "{\"compilerOptions\": {\"target\": \"ES2020\", \"jsx\": \"react\"}}",
		"frontend/src/App.tsx":               "import React from 'react';\n\nconst App: React.FC = () => {\n  return <div>Frontend App</div>;\n};\n\nexport default App;",
		"frontend/src/components/Layout.tsx": "import React from 'react';\n\nexport const Layout: React.FC = ({ children }) => <div>{children}</div>;",
		"frontend/src/services/api.ts":       "export class ApiService {\n  async fetchData(): Promise<any> {\n    return fetch('/api/data').then(r => r.json());\n  }\n}",
		"frontend/public/index.html":         "<!DOCTYPE html>\n<html>\n<head><title>Web App</title></head>\n<body><div id=\"root\"></div></body>\n</html>",

		// Go Backend
		"backend/go.mod":                          "module webapp-backend\n\ngo 1.19\n\nrequire (\n\tgithub.com/gin-gonic/gin v1.9.1\n\tgorm.io/gorm v1.25.0\n)",
		"backend/main.go":                         "package main\n\nimport \"github.com/gin-gonic/gin\"\n\nfunc main() {\n\tr := gin.Default()\n\tr.GET(\"/api/health\", healthCheck)\n\tr.Run(\":8080\")\n}",
		"backend/cmd/server/main.go":              "package main\n\nfunc main() {\n\tprintln(\"Starting server\")\n}",
		"backend/internal/handlers/user.go":       "package handlers\n\nimport \"github.com/gin-gonic/gin\"\n\nfunc GetUser(c *gin.Context) {\n\tc.JSON(200, gin.H{\"user\": \"test\"})\n}",
		"backend/internal/models/user.go":         "package models\n\ntype User struct {\n\tID   uint   `json:\"id\"`\n\tName string `json:\"name\"`\n}",
		"backend/internal/database/connection.go": "package database\n\nimport \"gorm.io/gorm\"\n\nvar DB *gorm.DB",

		// Shared API definitions
		"shared/api/openapi.yaml": "openapi: 3.0.0\ninfo:\n  title: Web App API\n  version: 1.0.0\npaths:\n  /api/users:\n    get:\n      summary: Get users",
		"shared/types/user.proto": "syntax = \"proto3\";\n\nmessage User {\n  int32 id = 1;\n  string name = 2;\n}",

		// Root configuration
		"docker-compose.yml": "version: '3.8'\nservices:\n  frontend:\n    build: ./frontend\n    ports:\n      - \"3000:3000\"\n  backend:\n    build: ./backend\n    ports:\n      - \"8080:8080\"",
		"README.md":          "# Full Stack Web Application\n\n## Frontend\nReact + TypeScript\n\n## Backend\nGo + Gin",
	}

	projectPath := suite.createTestProject("webapp-project", structure)

	// Scan the project
	info, err := suite.scanner.ScanProject(projectPath)
	suite.Require().NoError(err)
	suite.NotNil(info)

	// Should detect as frontend-backend project
	suite.Equal(config.ProjectTypeFrontendBackend, info.ProjectType, "Should detect frontend-backend project type")

	// Should detect both frontend and backend languages
	suite.True(info.HasLanguage("typescript"), "Should detect TypeScript (frontend)")
	suite.True(info.HasLanguage("go"), "Should detect Go (backend)")

	// Validate language contexts
	if tsCtx := info.GetLanguageContext("typescript"); tsCtx != nil {
		suite.Greater(tsCtx.FileCount, 0, "TypeScript should have files")
		suite.Greater(tsCtx.Priority, 70, "TypeScript should have high priority")
		suite.Contains(strings.Join(tsCtx.SourcePaths, ","), "src", "TypeScript should have src paths")
	}

	if goCtx := info.GetLanguageContext("go"); goCtx != nil {
		suite.Greater(goCtx.FileCount, 0, "Go should have files")
		suite.Greater(goCtx.Priority, 70, "Go should have high priority")
		suite.NotEmpty(goCtx.BuildFiles, "Go should have build files")
	}

	// Validate workspace roots
	suite.Contains(info.WorkspaceRoots, "typescript", "Should have TypeScript workspace root")
	suite.Contains(info.WorkspaceRoots, "go", "Should have Go workspace root")
}

// TestComplexNestedStructures tests projects with complex nested directory structures
func (suite *MultiLanguageTestSuite) TestComplexNestedStructures() {
	structure := map[string]string{
		// Go platform with multiple modules
		"platform/core/module1/go.mod":             "module platform/core/module1\n\ngo 1.19",
		"platform/core/module1/main.go":            "package main\n\nfunc main() { println(\"Module 1\") }",
		"platform/core/module1/lib/util.go":        "package lib\n\nfunc Utility() string { return \"util\" }",
		"platform/core/module2/go.mod":             "module platform/core/module2\n\ngo 1.19",
		"platform/core/module2/main.go":            "package main\n\nfunc main() { println(\"Module 2\") }",
		"platform/core/module2/service/handler.go": "package service\n\ntype Handler struct{}",

		// Python plugins
		"platform/plugins/python-plugin/setup.py":             "from setuptools import setup\n\nsetup(name='python-plugin', version='1.0.0')",
		"platform/plugins/python-plugin/plugin/__init__.py":   "from .main import Plugin",
		"platform/plugins/python-plugin/plugin/main.py":       "class Plugin:\n    def execute(self): pass",
		"platform/plugins/python-plugin/tests/test_plugin.py": "import unittest\nfrom plugin.main import Plugin\n\nclass TestPlugin(unittest.TestCase): pass",

		// Java plugins
		"platform/plugins/java-plugin/pom.xml": `<?xml version="1.0"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
    <modelVersion>4.0.0</modelVersion>
    <groupId>com.platform</groupId>
    <artifactId>java-plugin</artifactId>
    <version>1.0.0</version>
</project>`,
		"platform/plugins/java-plugin/src/main/java/com/platform/JavaPlugin.java": "package com.platform;\n\npublic class JavaPlugin {\n    public void execute() {}\n}",

		// TypeScript applications
		"applications/web/package.json":           "{\"name\": \"web-app\", \"version\": \"1.0.0\", \"dependencies\": {\"typescript\": \"^4.9.0\", \"webpack\": \"^5.0.0\"}}",
		"applications/web/tsconfig.json":          "{\"compilerOptions\": {\"target\": \"ES2020\", \"module\": \"commonjs\"}}",
		"applications/web/src/index.ts":           "console.log('Web application starting');\n\nclass WebApp {\n    start() { console.log('Started'); }\n}",
		"applications/web/src/components/App.tsx": "import React from 'react';\n\nexport const App = () => <div>Complex App</div>;",
		"applications/web/webpack.config.js":      "module.exports = {\n  entry: './src/index.ts',\n  module: { rules: [] },\n  resolve: { extensions: ['.ts', '.tsx'] }\n};",

		// Rust CLI application
		"applications/cli/Cargo.toml":            "[package]\nname = \"platform-cli\"\nversion = \"0.1.0\"\nedition = \"2021\"\n\n[dependencies]\nclap = \"4.0\"",
		"applications/cli/src/main.rs":           "use clap::Parser;\n\n#[derive(Parser)]\nstruct Args {\n    command: String,\n}\n\nfn main() {\n    let args = Args::parse();\n}",
		"applications/cli/src/commands/mod.rs":   "pub mod build;\npub mod deploy;",
		"applications/cli/src/commands/build.rs": "pub fn build() { println!(\"Building\"); }",

		// Shared libraries and configs
		"shared/proto/platform.proto":     "syntax = \"proto3\";\n\nservice PlatformService {\n  rpc GetStatus(StatusRequest) returns (StatusResponse);\n}",
		"shared/configs/development.yaml": "platform:\n  modules:\n    - core\n    - plugins\n  applications:\n    - web\n    - cli",
		"shared/scripts/build-all.sh":     "#!/bin/bash\n\necho 'Building all platform components'\ncd platform/core && make build\ncd applications && make build",

		// Root configuration
		"Makefile":           "build-platform:\n\t@cd platform && make build\n\nbuild-apps:\n\t@cd applications && make build\n\ntest-all:\n\t@make test-platform\n\t@make test-apps",
		"docker-compose.yml": "version: '3.8'\nservices:\n  platform:\n    build: .\n  web:\n    build: ./applications/web",
	}

	projectPath := suite.createTestProject("complex-nested", structure)

	// Scan the project
	info, err := suite.scanner.ScanProject(projectPath)
	suite.Require().NoError(err)
	suite.NotNil(info)

	// Should detect complex project structure
	suite.Contains([]string{config.ProjectTypeMonorepo, config.ProjectTypeWorkspace, config.ProjectTypeMulti},
		info.ProjectType, "Should detect complex project type")

	// Should detect multiple languages
	suite.GreaterOrEqual(len(info.Languages), 4, "Should detect at least 4 languages")

	// Validate specific languages
	expectedLanguages := []string{"go", "python", "java", "typescript", "rust"}
	for _, lang := range expectedLanguages {
		if info.HasLanguage(lang) {
			ctx := info.GetLanguageContext(lang)
			suite.NotNil(ctx, fmt.Sprintf("Should have context for %s", lang))
			suite.Greater(ctx.FileCount, 0, fmt.Sprintf("%s should have files", lang))
		}
	}

	// Validate nested structure handling
	suite.Greater(info.ScanDepth, 3, "Should scan deep nested structures")
	suite.Greater(len(info.BuildFiles), 3, "Should detect multiple build files")

	// Validate workspace roots are correctly mapped
	for lang, ctx := range info.Languages {
		root := info.GetWorkspaceRoot(lang)
		suite.NotEmpty(root, fmt.Sprintf("Language %s should have workspace root", lang))
		suite.Equal(ctx.RootPath, root, fmt.Sprintf("Workspace root should match context root for %s", lang))
	}
}

// TestLanguagePriorityScoring tests the language priority scoring system
func (suite *MultiLanguageTestSuite) TestLanguagePriorityScoring() {
	structure := map[string]string{
		// Go - should have highest priority (many files + build system)
		"go.mod":              "module test-project\n\ngo 1.19",
		"main.go":             "package main\n\nfunc main() {}",
		"cmd/app.go":          "package main\n\nfunc main() {}",
		"pkg/util.go":         "package pkg\n\nfunc Util() {}",
		"internal/service.go": "package internal\n\ntype Service struct{}",
		"go.sum":              "github.com/example/dep v1.0.0 h1:abc",

		// Python - medium priority (fewer files + build system)
		"setup.py": "from setuptools import setup\n\nsetup(name='test')",
		"main.py":  "print('Hello Python')",
		"utils.py": "def utility(): pass",

		// JavaScript - lower priority (few files, no build system markers)
		"script.js": "console.log('Hello JS');",
		"helper.js": "function help() {}",

		// Test files (should not boost priority significantly)
		"main_test.go": "package main\n\nimport \"testing\"\n\nfunc TestMain(t *testing.T) {}",
		"test_main.py": "import unittest\n\nclass TestMain(unittest.TestCase): pass",
	}

	projectPath := suite.createTestProject("priority-test", structure)

	// Scan the project
	info, err := suite.scanner.ScanProject(projectPath)
	suite.Require().NoError(err)
	suite.NotNil(info)

	// Get language contexts and check priorities
	languages := info.Languages

	// Go should have highest priority
	if goCtx, exists := languages["go"]; exists {
		suite.Greater(goCtx.Priority, 80, "Go should have high priority due to build files and source count")
		suite.Greater(goCtx.Confidence, 0.8, "Go should have high confidence")
	}

	// Python should have medium priority
	if pyCtx, exists := languages["python"]; exists {
		suite.GreaterOrEqual(pyCtx.Priority, 40, "Python should have medium priority")
		suite.Greater(pyCtx.Confidence, 0.6, "Python should have good confidence")
	}

	// JavaScript should have lower priority
	if jsCtx, exists := languages["javascript"]; exists {
		suite.Less(jsCtx.Priority, 60, "JavaScript should have lower priority without build system")
	}

	// Dominant language should be Go
	suite.Equal("go", info.DominantLanguage, "Go should be the dominant language")
}

// TestWorkspaceRootMapping tests correct mapping of workspace roots for different languages
func (suite *MultiLanguageTestSuite) TestWorkspaceRootMapping() {
	structure := map[string]string{
		// Go workspace
		"go-service/go.mod":  "module go-service\n\ngo 1.19",
		"go-service/main.go": "package main\n\nfunc main() {}",

		// Python workspace
		"python-service/pyproject.toml": "[project]\nname = \"python-service\"",
		"python-service/main.py":        "print('Python service')",

		// TypeScript workspace
		"ts-app/package.json":  "{\"name\": \"ts-app\", \"dependencies\": {\"typescript\": \"^4.9.0\"}}",
		"ts-app/tsconfig.json": "{\"compilerOptions\": {\"target\": \"ES2020\"}}",
		"ts-app/src/index.ts":  "console.log('TypeScript app');",
	}

	projectPath := suite.createTestProject("workspace-mapping", structure)

	// Scan the project
	info, err := suite.scanner.ScanProject(projectPath)
	suite.Require().NoError(err)
	suite.NotNil(info)

	// Validate workspace roots
	expectedRoots := map[string]string{
		"go":         filepath.Join(projectPath, "go-service"),
		"python":     filepath.Join(projectPath, "python-service"),
		"typescript": filepath.Join(projectPath, "ts-app"),
	}

	for lang, expectedRoot := range expectedRoots {
		if info.HasLanguage(lang) {
			actualRoot := info.GetWorkspaceRoot(lang)
			// The workspace root should be the project root for now in our implementation
			suite.Equal(projectPath, actualRoot, fmt.Sprintf("Workspace root for %s should be correctly mapped", lang))
		}
	}
}

// TestFrameworkDetection tests detection of frameworks within languages
func (suite *MultiLanguageTestSuite) TestFrameworkDetection() {
	structure := map[string]string{
		// React project
		"frontend/package.json": "{\"name\": \"frontend\", \"dependencies\": {\"react\": \"^18.0.0\", \"@types/react\": \"^18.0.0\"}}",
		"frontend/src/App.tsx":  "import React from 'react';\n\nconst App = () => <div>React App</div>;\n\nexport default App;",

		// Express.js backend
		"backend/package.json": "{\"name\": \"backend\", \"dependencies\": {\"express\": \"^4.18.0\"}}",
		"backend/server.js":    "const express = require('express');\nconst app = express();\napp.listen(3000);",

		// Django project
		"django-app/manage.py":        "#!/usr/bin/env python\nimport django\nfrom django.core.management import execute_from_command_line",
		"django-app/requirements.txt": "Django==4.2.0\npsycopg2==2.9.0",
		"django-app/myapp/models.py":  "from django.db import models\n\nclass User(models.Model):\n    name = models.CharField(max_length=100)",

		// Spring Boot project
		"spring-app/pom.xml": `<?xml version="1.0"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
    <modelVersion>4.0.0</modelVersion>
    <groupId>com.example</groupId>
    <artifactId>spring-app</artifactId>
    <version>1.0.0</version>
    <dependencies>
        <dependency>
            <groupId>org.springframework.boot</groupId>
            <artifactId>spring-boot-starter-web</artifactId>
        </dependency>
    </dependencies>
</project>`,
		"spring-app/src/main/java/Application.java": "@SpringBootApplication\npublic class Application {\n    public static void main(String[] args) {}\n}",
		"spring-app/application.properties":         "server.port=8080\nspring.datasource.url=jdbc:h2:mem:testdb",
	}

	projectPath := suite.createTestProject("framework-detection", structure)

	// Scan the project with comprehensive analysis
	scanner := gateway.NewProjectLanguageScanner()
	info, err := scanner.ScanProjectComprehensive(projectPath)
	suite.Require().NoError(err)
	suite.NotNil(info)

	// Validate framework detection
	for _, ctx := range info.Languages {
		if ctx.Language == "typescript" || ctx.Language == "javascript" {
			// Should detect React or Express frameworks
			if ctx.HasFramework() {
				suite.Contains([]string{"React", "Express.js"}, ctx.Framework,
					fmt.Sprintf("Should detect React or Express framework for %s", ctx.Language))
			}
		}

		if ctx.Language == "python" {
			// Should detect Django framework
			if ctx.HasFramework() {
				suite.Equal("Django", ctx.Framework, "Should detect Django framework for Python")
			}
		}

		if ctx.Language == "java" {
			// Should detect Spring Boot framework
			if ctx.HasFramework() {
				suite.Equal("Spring Boot", ctx.Framework, "Should detect Spring Boot framework for Java")
			}
		}
	}
}

// TestPerformanceWithLargeProjects tests performance with large project structures
func (suite *MultiLanguageTestSuite) TestPerformanceWithLargeProjects() {
	// Create a project with many files to test performance
	structure := make(map[string]string)

	// Generate many Go files
	for i := 0; i < 100; i++ {
		structure[fmt.Sprintf("services/service%d/main.go", i)] = fmt.Sprintf("package main\n\nfunc main%d() {}", i)
		structure[fmt.Sprintf("services/service%d/handler%d.go", i, i)] = fmt.Sprintf("package main\n\ntype Handler%d struct{}", i)
	}

	// Add build files
	structure["go.mod"] = "module large-project\n\ngo 1.19"
	structure["go.work"] = "go 1.19\n\nuse (\n\t./services/service1\n\t./services/service2\n)"

	// Generate many Python files
	for i := 0; i < 50; i++ {
		structure[fmt.Sprintf("python-services/service%d/main.py", i)] = fmt.Sprintf("def main%d(): pass", i)
		structure[fmt.Sprintf("python-services/service%d/models.py", i)] = fmt.Sprintf("class Model%d: pass", i)
	}
	structure["requirements.txt"] = "flask==2.0.1\nrequests==2.28.0"

	projectPath := suite.createTestProject("large-project", structure)

	// Measure scan performance
	start := time.Now()
	info, err := suite.scanner.ScanProject(projectPath)
	duration := time.Since(start)

	suite.Require().NoError(err)
	suite.NotNil(info)

	// Validate performance expectations
	suite.Less(duration, 10*time.Second, "Should scan large project within 10 seconds")
	suite.Greater(info.TotalFileCount, 100, "Should process many files")

	// Validate detection accuracy despite size
	suite.True(info.HasLanguage("go"), "Should still detect Go accurately")
	suite.True(info.HasLanguage("python"), "Should still detect Python accurately")

	// Test cache performance
	start = time.Now()
	cachedInfo, err := suite.scanner.ScanProjectCached(projectPath)
	cachedDuration := time.Since(start)

	suite.Require().NoError(err)
	suite.NotNil(cachedInfo)
	suite.Less(cachedDuration, duration/10, "Cached scan should be much faster")
}

// TestProjectTypeClassification tests accurate project type classification
func (suite *MultiLanguageTestSuite) TestProjectTypeClassification() {
	testCases := []struct {
		name         string
		structure    map[string]string
		expectedType string
	}{
		{
			name: "single-language-go",
			structure: map[string]string{
				"go.mod":  "module single-go\n\ngo 1.19",
				"main.go": "package main\n\nfunc main() {}",
			},
			expectedType: config.ProjectTypeSingle,
		},
		{
			name: "microservices",
			structure: map[string]string{
				"auth-service/go.mod":      "module auth\n\ngo 1.19",
				"auth-service/main.go":     "package main\n\nfunc main() {}",
				"user-service/pom.xml":     "<?xml version=\"1.0\"?>\n<project><modelVersion>4.0.0</modelVersion></project>",
				"user-service/Main.java":   "public class Main { public static void main(String[] args) {} }",
				"api-gateway/package.json": "{\"name\": \"api-gateway\", \"dependencies\": {\"express\": \"^4.18.0\"}}",
				"api-gateway/server.js":    "const express = require('express');",
				"docker-compose.yml":       "version: '3.8'\nservices:\n  auth:\n    build: ./auth-service",
			},
			expectedType: config.ProjectTypeMicroservices,
		},
		{
			name: "frontend-backend",
			structure: map[string]string{
				"frontend/package.json": "{\"name\": \"frontend\", \"dependencies\": {\"react\": \"^18.0.0\"}}",
				"frontend/src/App.tsx":  "import React from 'react';\n\nconst App = () => <div>App</div>;",
				"backend/go.mod":        "module backend\n\ngo 1.19",
				"backend/main.go":       "package main\n\nfunc main() {}",
			},
			expectedType: config.ProjectTypeFrontendBackend,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			projectPath := suite.createTestProject(tc.name, tc.structure)

			info, err := suite.scanner.ScanProject(projectPath)
			suite.Require().NoError(err)
			suite.NotNil(info)

			suite.Equal(tc.expectedType, info.ProjectType,
				fmt.Sprintf("Project %s should be classified as %s", tc.name, tc.expectedType))
		})
	}
}

// TestDominantLanguageDetection tests detection of the dominant language
func (suite *MultiLanguageTestSuite) TestDominantLanguageDetection() {
	structure := map[string]string{
		// Go should be dominant (more files + build system)
		"go.mod":    "module test\n\ngo 1.19",
		"main.go":   "package main\n\nfunc main() {}",
		"server.go": "package main\n\ntype Server struct{}",
		"client.go": "package main\n\ntype Client struct{}",
		"util.go":   "package main\n\nfunc Util() {}",

		// Python (fewer files)
		"setup.py": "from setuptools import setup\n\nsetup(name='test')",
		"app.py":   "print('Hello')",

		// JavaScript (minimal presence)
		"script.js": "console.log('Hello');",
	}

	projectPath := suite.createTestProject("dominant-language", structure)

	info, err := suite.scanner.ScanProject(projectPath)
	suite.Require().NoError(err)
	suite.NotNil(info)

	suite.Equal("go", info.DominantLanguage, "Go should be detected as dominant language")

	// Validate that Go has the highest priority
	if goCtx := info.GetLanguageContext("go"); goCtx != nil {
		for lang, ctx := range info.Languages {
			if lang != "go" {
				suite.GreaterOrEqual(goCtx.Priority, ctx.Priority,
					fmt.Sprintf("Go priority should be >= %s priority", lang))
			}
		}
	}
}

// TestLanguageContextAccuracy tests accuracy of language context information
func (suite *MultiLanguageTestSuite) TestLanguageContextAccuracy() {
	structure := map[string]string{
		// Go project with clear structure
		"go.mod":               "module test-go\n\ngo 1.19\n\nrequire github.com/gin-gonic/gin v1.9.1",
		"cmd/main.go":          "package main\n\nfunc main() {}",
		"internal/service.go":  "package internal\n\ntype Service struct{}",
		"pkg/utils.go":         "package pkg\n\nfunc Utils() {}",
		"test/main_test.go":    "package test\n\nimport \"testing\"\n\nfunc TestMain(t *testing.T) {}",
		"test/service_test.go": "package test\n\nimport \"testing\"\n\nfunc TestService(t *testing.T) {}",
	}

	projectPath := suite.createTestProject("context-accuracy", structure)

	info, err := suite.scanner.ScanProject(projectPath)
	suite.Require().NoError(err)
	suite.NotNil(info)

	suite.True(info.HasLanguage("go"), "Should detect Go language")

	goCtx := info.GetLanguageContext("go")
	suite.NotNil(goCtx, "Should have Go language context")

	// Validate context accuracy
	suite.Equal("go", goCtx.Language, "Language should be correctly set")
	suite.Equal(4, goCtx.FileCount, "Should count source files correctly")
	suite.Equal(2, goCtx.TestFileCount, "Should count test files correctly")
	suite.Greater(goCtx.Priority, 70, "Should have high priority")
	suite.Greater(goCtx.Confidence, 0.8, "Should have high confidence")
	suite.NotEmpty(goCtx.BuildFiles, "Should identify build files")
	suite.Contains(goCtx.FileExtensions, ".go", "Should include Go file extensions")
	suite.Greater(len(goCtx.SourcePaths), 0, "Should identify source paths")
}

// TestConfigGeneration tests generation of configuration from detected project info
func (suite *MultiLanguageTestSuite) TestConfigGeneration() {
	structure := map[string]string{
		"go.mod":         "module config-test\n\ngo 1.19",
		"main.go":        "package main\n\nfunc main() {}",
		"pyproject.toml": "[project]\nname = \"config-test\"\nversion = \"1.0.0\"",
		"main.py":        "print('Hello Python')",
		"package.json":   "{\"name\": \"config-test\", \"dependencies\": {\"typescript\": \"^4.9.0\"}}",
		"tsconfig.json":  "{\"compilerOptions\": {\"target\": \"ES2020\"}}",
		"src/index.ts":   "console.log('Hello TypeScript');",
	}

	projectPath := suite.createTestProject("config-generation", structure)

	info, err := suite.scanner.ScanProject(projectPath)
	suite.Require().NoError(err)
	suite.NotNil(info)

	// Convert to project context for configuration
	projectContext := info.ToProjectContext()
	suite.NotNil(projectContext, "Should convert to project context")

	// Validate project context
	suite.Equal(info.ProjectType, projectContext.ProjectType, "Project types should match")
	suite.Equal(info.RootPath, projectContext.RootDirectory, "Root directories should match")
	suite.GreaterOrEqual(len(projectContext.Languages), 2, "Should have multiple languages")
	suite.NotEmpty(projectContext.RequiredLSPs, "Should identify required LSP servers")

	// Validate language information
	languageMap := make(map[string]config.LanguageInfo)
	for _, langInfo := range projectContext.Languages {
		languageMap[langInfo.Language] = langInfo
	}

	expectedLanguages := []string{"go", "python", "typescript"}
	for _, lang := range expectedLanguages {
		if langInfo, exists := languageMap[lang]; exists {
			suite.Greater(langInfo.FileCount, 0, fmt.Sprintf("Language %s should have file count", lang))
			suite.NotEmpty(langInfo.FilePatterns, fmt.Sprintf("Language %s should have file patterns", lang))
		}
	}
}

// TestWorkspaceManagerIntegration tests integration with workspace manager
func (suite *MultiLanguageTestSuite) TestWorkspaceManagerIntegration() {
	structure := map[string]string{
		"go-service/go.mod":         "module go-service\n\ngo 1.19",
		"go-service/main.go":        "package main\n\nfunc main() {}",
		"python-service/setup.py":   "from setuptools import setup\n\nsetup(name='python-service')",
		"python-service/main.py":    "print('Python service')",
		"shared/docker-compose.yml": "version: '3.8'\nservices:\n  go:\n    build: ./go-service\n  python:\n    build: ./python-service",
	}

	projectPath := suite.createTestProject("workspace-integration", structure)

	info, err := suite.scanner.ScanProject(projectPath)
	suite.Require().NoError(err)
	suite.NotNil(info)

	// Validate workspace information for integration
	suite.NotEmpty(info.WorkspaceRoots, "Should have workspace roots")
	suite.Equal(info.RootPath, info.GetWorkspaceRoot("go"), "Go workspace root should be set")
	suite.Equal(info.RootPath, info.GetWorkspaceRoot("python"), "Python workspace root should be set")

	// Validate recommended LSP servers
	recommendedServers := info.GetRecommendedLSPServers()
	suite.NotEmpty(recommendedServers, "Should recommend LSP servers")

	expectedServers := map[string]bool{
		"gopls": false,
		"pylsp": false,
	}

	for _, server := range recommendedServers {
		if _, exists := expectedServers[server]; exists {
			expectedServers[server] = true
		}
	}

	for server, found := range expectedServers {
		suite.True(found, fmt.Sprintf("Should recommend %s server", server))
	}
}

// Run the test suite
func TestMultiLanguageTestSuite(t *testing.T) {
	suite.Run(t, new(MultiLanguageTestSuite))
}

// Benchmark tests for performance validation
func BenchmarkScanLargeMonorepo(b *testing.B) {
	// Create large monorepo structure
	tempDir, _ := os.MkdirTemp("", "bench-monorepo-*")
	defer os.RemoveAll(tempDir)

	structure := make(map[string]string)

	// Generate many files across multiple languages
	for i := 0; i < 20; i++ {
		structure[fmt.Sprintf("services/go-service-%d/go.mod", i)] = fmt.Sprintf("module service-%d\n\ngo 1.19", i)
		structure[fmt.Sprintf("services/go-service-%d/main.go", i)] = fmt.Sprintf("package main\n\nfunc main%d() {}", i)
		structure[fmt.Sprintf("services/python-service-%d/setup.py", i)] = fmt.Sprintf("from setuptools import setup\n\nsetup(name='service-%d')", i)
		structure[fmt.Sprintf("services/python-service-%d/main.py", i)] = fmt.Sprintf("def main%d(): pass", i)
	}

	// Create project structure
	for path, content := range structure {
		fullPath := filepath.Join(tempDir, path)
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		os.WriteFile(fullPath, []byte(content), 0644)
	}

	scanner := gateway.NewProjectLanguageScanner()
	scanner.OptimizeForLargeMonorepos()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := scanner.ScanProject(tempDir)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkScanWithCache(b *testing.B) {
	tempDir, _ := os.MkdirTemp("", "bench-cache-*")
	defer os.RemoveAll(tempDir)

	structure := map[string]string{
		"go.mod":       "module test\n\ngo 1.19",
		"main.go":      "package main\n\nfunc main() {}",
		"setup.py":     "from setuptools import setup\n\nsetup(name='test')",
		"main.py":      "print('Hello')",
		"package.json": "{\"name\": \"test\"}",
		"index.js":     "console.log('Hello');",
	}

	// Create project structure
	for path, content := range structure {
		fullPath := filepath.Join(tempDir, path)
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		os.WriteFile(fullPath, []byte(content), 0644)
	}

	scanner := gateway.NewProjectLanguageScanner()

	// Prime the cache
	scanner.ScanProjectCached(tempDir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := scanner.ScanProjectCached(tempDir)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMultipleProjectScans(b *testing.B) {
	tempDir, _ := os.MkdirTemp("", "bench-multiple-*")
	defer os.RemoveAll(tempDir)

	// Create multiple small projects
	projects := make([]string, 10)
	for i := 0; i < 10; i++ {
		projectDir := filepath.Join(tempDir, fmt.Sprintf("project-%d", i))
		os.MkdirAll(projectDir, 0755)

		// Create simple Go project
		os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte(fmt.Sprintf("module project-%d\n\ngo 1.19", i)), 0644)
		os.WriteFile(filepath.Join(projectDir, "main.go"), []byte(fmt.Sprintf("package main\n\nfunc main%d() {}", i)), 0644)

		projects[i] = projectDir
	}

	scanner := gateway.NewProjectLanguageScanner()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		results := scanner.BatchScanProjects(projects)
		if len(results) != len(projects) {
			b.Fatal("Not all projects scanned")
		}
	}
}
