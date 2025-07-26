package unit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"lsp-gateway/internal/config"
	"lsp-gateway/internal/gateway"
)

// ProjectDetectorTestSuite provides unit tests for project detector components
type ProjectDetectorTestSuite struct {
	suite.Suite
	tempDir string
	scanner *gateway.ProjectLanguageScanner
}

func (suite *ProjectDetectorTestSuite) SetupTest() {
	tempDir, err := os.MkdirTemp("", "project-detector-test-*")
	suite.Require().NoError(err)
	suite.tempDir = tempDir

	suite.scanner = gateway.NewProjectLanguageScanner()
}

func (suite *ProjectDetectorTestSuite) TearDownTest() {
	if suite.tempDir != "" {
		_ = os.RemoveAll(suite.tempDir)
	}
	if suite.scanner != nil {
		suite.scanner.Shutdown()
	}
}

func (suite *ProjectDetectorTestSuite) createFile(relPath, content string) string {
	fullPath := filepath.Join(suite.tempDir, relPath)
	err := os.MkdirAll(filepath.Dir(fullPath), 0755)
	suite.Require().NoError(err)
	err = os.WriteFile(fullPath, []byte(content), 0644)
	suite.Require().NoError(err)
	return fullPath
}

// TestProjectLanguageScannerInitialization tests scanner initialization
func (suite *ProjectDetectorTestSuite) TestProjectLanguageScannerInitialization() {
	scanner := gateway.NewProjectLanguageScanner()
	defer scanner.Shutdown()

	// Validate default configuration values are reasonable
	assert.Greater(suite.T(), scanner.MaxDepth, 0)
	assert.Greater(suite.T(), scanner.FileSizeLimit, int64(0))
	assert.Greater(suite.T(), scanner.MaxFiles, 0)
	assert.Greater(suite.T(), scanner.Timeout, time.Duration(0))
	assert.True(suite.T(), scanner.EnableEarlyExit)
	assert.True(suite.T(), scanner.EnableCache)

	// Validate supported languages
	supportedLanguages := scanner.GetSupportedLanguages()
	expectedLanguages := []string{"go", "python", "typescript", "javascript", "java", "kotlin", "rust", "cpp", "c", "csharp", "ruby", "php", "swift"}

	for _, lang := range expectedLanguages {
		assert.Contains(suite.T(), supportedLanguages, lang, fmt.Sprintf("Should support %s language", lang))
	}
}

// TestLanguagePatternMatching tests language detection patterns
func (suite *ProjectDetectorTestSuite) TestLanguagePatternMatching() {
	testCases := []struct {
		filename     string
		content      string
		expectedLang string
		description  string
	}{
		{"main.go", "package main\n\nfunc main() {}", "go", "Go source file"},
		{"server.py", "print('Hello Python')", "python", "Python source file"},
		{"app.ts", "console.log('TypeScript');", "typescript", "TypeScript source file"},
		{"component.tsx", "import React from 'react';", "typescript", "TypeScript React component"},
		{"script.js", "console.log('JavaScript');", "javascript", "JavaScript source file"},
		{"App.jsx", "import React from 'react';", "javascript", "JavaScript React component"},
		{"Main.java", "public class Main {}", "java", "Java source file"},
		{"Script.kt", "fun main() {}", "kotlin", "Kotlin source file"},
		{"main.rs", "fn main() {}", "rust", "Rust source file"},
		{"program.cpp", "#include <iostream>", "cpp", "C++ source file"},
		{"program.c", "#include <stdio.h>", "c", "C source file"},
		{"Program.cs", "using System;", "csharp", "C# source file"},
		{"script.rb", "puts 'Hello Ruby'", "ruby", "Ruby source file"},
		{"index.php", "<?php echo 'Hello PHP'; ?>", "php", "PHP source file"},
		{"main.swift", "import Foundation", "swift", "Swift source file"},
	}

	for _, tc := range testCases {
		suite.Run(tc.description, func() {
			// Create project with single file
			suite.createFile(tc.filename, tc.content)

			info, err := suite.scanner.ScanProject(suite.tempDir)
			suite.Require().NoError(err)
			suite.NotNil(info)

			suite.True(info.HasLanguage(tc.expectedLang),
				fmt.Sprintf("Should detect %s for file %s", tc.expectedLang, tc.filename))

			langCtx := info.GetLanguageContext(tc.expectedLang)
			suite.NotNil(langCtx, fmt.Sprintf("Should have context for %s", tc.expectedLang))
			suite.Greater(langCtx.FileCount, 0, fmt.Sprintf("%s should have file count > 0", tc.expectedLang))
		})
	}
}

// TestBuildFileDetection tests detection of build files
func (suite *ProjectDetectorTestSuite) TestBuildFileDetection() {
	testCases := []struct {
		filename     string
		content      string
		expectedLang string
		description  string
	}{
		{"go.mod", "module test\n\ngo 1.19", "go", "Go module file"},
		{"go.sum", "github.com/example/dep v1.0.0 h1:abc", "go", "Go sum file"},
		{"package.json", "{\"name\": \"test\", \"version\": \"1.0.0\"}", "javascript", "NPM package file"},
		{"yarn.lock", "# yarn lockfile v1", "javascript", "Yarn lock file"},
		{"setup.py", "from setuptools import setup\n\nsetup(name='test')", "python", "Python setup file"},
		{"pyproject.toml", "[build-system]\nrequires = [\"setuptools\"]", "python", "Python project file"},
		{"requirements.txt", "flask==2.0.1\nrequests==2.28.0", "python", "Python requirements file"},
		{"pom.xml", "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<project xmlns=\"http://maven.apache.org/POM/4.0.0\"\n         xmlns:xsi=\"http://www.w3.org/2001/XMLSchema-instance\"\n         xsi:schemaLocation=\"http://maven.apache.org/POM/4.0.0 http://maven.apache.org/xsd/maven-4.0.0.xsd\">\n    <modelVersion>4.0.0</modelVersion>\n    <groupId>com.example</groupId>\n    <artifactId>test-project</artifactId>\n    <version>1.0.0</version>\n</project>", "java", "Maven POM file"},
		{"build.gradle", "plugins {\n    id 'java'\n}", "java", "Gradle build file"},
		{"Cargo.toml", "[package]\nname = \"test\"\nversion = \"0.1.0\"", "rust", "Cargo manifest file"},
		{"Makefile", "all:\n\t@echo 'Building'", "c", "Make build file"},
		{"CMakeLists.txt", "cmake_minimum_required(VERSION 3.10)", "cpp", "CMake build file"},
	}

	for _, tc := range testCases {
		suite.Run(tc.description, func() {
			// Create fresh temp directory for each test
			tempDir, err := os.MkdirTemp("", "build-file-test-*")
			suite.Require().NoError(err)
			defer func() { _ = os.RemoveAll(tempDir) }()

			// Create build file
			fullPath := filepath.Join(tempDir, tc.filename)
			err = os.WriteFile(fullPath, []byte(tc.content), 0644)
			suite.Require().NoError(err)

			info, err := suite.scanner.ScanProject(tempDir)
			suite.Require().NoError(err)
			suite.NotNil(info)

			suite.True(info.HasLanguage(tc.expectedLang),
				fmt.Sprintf("Should detect %s for build file %s", tc.expectedLang, tc.filename))

			langCtx := info.GetLanguageContext(tc.expectedLang)
			suite.NotNil(langCtx, fmt.Sprintf("Should have context for %s", tc.expectedLang))
			suite.Greater(len(langCtx.BuildFiles), 0, fmt.Sprintf("%s should have build files", tc.expectedLang))

			// Verify the build file is included
			foundBuildFile := false
			for _, buildFile := range langCtx.BuildFiles {
				if strings.HasSuffix(buildFile, tc.filename) {
					foundBuildFile = true
					break
				}
			}
			suite.True(foundBuildFile, fmt.Sprintf("Build file %s should be included in %s context", tc.filename, tc.expectedLang))
		})
	}
}

// TestTestFileIdentification tests identification of test files
func (suite *ProjectDetectorTestSuite) TestTestFileIdentification() {
	testCases := []struct {
		filename    string
		content     string
		isTestFile  bool
		description string
	}{
		{"main_test.go", "package main\n\nimport \"testing\"\n\nfunc TestMain(t *testing.T) {}", true, "Go test file"},
		{"main.go", "package main\n\nfunc main() {}", false, "Go source file"},
		{"test_main.py", "import unittest\n\nclass TestMain(unittest.TestCase): pass", true, "Python test file"},
		{"main.py", "print('Hello')", false, "Python source file"},
		{"app.test.js", "test('should work', () => { expect(true).toBe(true); });", true, "JavaScript test file"},
		{"app.spec.ts", "describe('App', () => { it('should work', () => {}); });", true, "TypeScript spec file"},
		{"component.tsx", "import React from 'react';\n\nconst Component = () => <div></div>;", false, "TypeScript component"},
		{"tests/helper.py", "def test_helper(): pass", true, "Python test in tests directory"},
		{"__tests__/util.js", "test('util', () => {});", true, "JavaScript test in __tests__ directory"},
		{"spec/model_spec.rb", "RSpec.describe Model do\nend", true, "Ruby spec file"},
	}

	for _, tc := range testCases {
		suite.Run(tc.description, func() {
			// Create fresh temp directory for each test
			tempDir, err := os.MkdirTemp("", "test-file-test-*")
			suite.Require().NoError(err)
			defer func() { _ = os.RemoveAll(tempDir) }()

			// Create test file
			fullPath := filepath.Join(tempDir, tc.filename)
			err = os.MkdirAll(filepath.Dir(fullPath), 0755)
			suite.Require().NoError(err)
			err = os.WriteFile(fullPath, []byte(tc.content), 0644)
			suite.Require().NoError(err)

			info, err := suite.scanner.ScanProject(tempDir)
			suite.Require().NoError(err)
			suite.NotNil(info)

			// Find the language for this file
			var targetLang string
			ext := filepath.Ext(tc.filename)
			switch ext {
			case ".go":
				targetLang = "go"
			case ".py":
				targetLang = "python"
			case ".js":
				targetLang = "javascript"
			case ".ts", ".tsx":
				targetLang = "typescript"
			case ".rb":
				targetLang = "ruby"
			default:
				suite.Fail(fmt.Sprintf("Unknown extension: %s", ext))
				return
			}

			if info.HasLanguage(targetLang) {
				langCtx := info.GetLanguageContext(targetLang)
				suite.NotNil(langCtx, fmt.Sprintf("Should have context for %s", targetLang))

				if tc.isTestFile {
					suite.Greater(langCtx.TestFileCount, 0,
						fmt.Sprintf("File %s should be identified as test file", tc.filename))
				} else {
					suite.Greater(langCtx.FileCount, 0,
						fmt.Sprintf("File %s should be identified as source file", tc.filename))
				}
			}
		})
	}
}

// TestSourceDirectoryDetection tests detection of source directories
func (suite *ProjectDetectorTestSuite) TestSourceDirectoryDetection() {
	// Create project with various directory structures
	files := map[string]string{
		"src/main.go":                    "package main\n\nfunc main() {}",
		"pkg/util.go":                    "package pkg\n\nfunc Util() {}",
		"internal/service.go":            "package internal\n\ntype Service struct{}",
		"cmd/app.go":                     "package main\n\nfunc main() {}",
		"lib/helper.py":                  "def helper(): pass",
		"app/models.py":                  "class Model: pass",
		"components/Button.tsx":          "import React from 'react';\n\nexport const Button = () => <button></button>;",
		"pages/Home.tsx":                 "import React from 'react';\n\nexport const Home = () => <div>Home</div>;",
		"src/main/java/Application.java": "public class Application {}",
		"api/handler.go":                 "package api\n\nfunc Handler() {}",
	}

	for file, content := range files {
		suite.createFile(file, content)
	}

	info, err := suite.scanner.ScanProject(suite.tempDir)
	suite.Require().NoError(err)
	suite.NotNil(info)

	// Validate source directory detection for Go
	if info.HasLanguage("go") {
		goCtx := info.GetLanguageContext("go")
		suite.NotNil(goCtx)

		expectedGoDirs := []string{"src", "pkg", "internal", "cmd", "api"}
		sourcePaths := strings.Join(goCtx.SourcePaths, ",")

		for _, expectedDir := range expectedGoDirs {
			suite.Contains(sourcePaths, expectedDir,
				fmt.Sprintf("Go should detect %s as source directory", expectedDir))
		}
	}

	// Validate source directory detection for Python
	if info.HasLanguage("python") {
		pyCtx := info.GetLanguageContext("python")
		suite.NotNil(pyCtx)

		expectedPyDirs := []string{"lib", "app"}
		sourcePaths := strings.Join(pyCtx.SourcePaths, ",")

		for _, expectedDir := range expectedPyDirs {
			suite.Contains(sourcePaths, expectedDir,
				fmt.Sprintf("Python should detect %s as source directory", expectedDir))
		}
	}

	// Validate source directory detection for TypeScript
	if info.HasLanguage("typescript") {
		tsCtx := info.GetLanguageContext("typescript")
		suite.NotNil(tsCtx)

		expectedTsDirs := []string{"components", "pages"}
		sourcePaths := strings.Join(tsCtx.SourcePaths, ",")

		for _, expectedDir := range expectedTsDirs {
			suite.Contains(sourcePaths, expectedDir,
				fmt.Sprintf("TypeScript should detect %s as source directory", expectedDir))
		}
	}
}

// TestLanguagePriorityCalculation tests the language priority calculation algorithm
func (suite *ProjectDetectorTestSuite) TestLanguagePriorityCalculation() {
	// Create project with different language characteristics
	files := map[string]string{
		// Go - high priority (many files + build system)
		"go.mod":        "module test\n\ngo 1.19",
		"main.go":       "package main\n\nfunc main() {}",
		"server.go":     "package main\n\ntype Server struct{}",
		"client.go":     "package main\n\ntype Client struct{}",
		"util.go":       "package main\n\nfunc Util() {}",
		"cmd/app.go":    "package main\n\nfunc main() {}",
		"pkg/helper.go": "package pkg\n\nfunc Helper() {}",

		// Python - medium priority (fewer files + build system)
		"setup.py": "from setuptools import setup\n\nsetup(name='test')",
		"main.py":  "print('Hello Python')",
		"utils.py": "def utility(): pass",

		// JavaScript - lower priority (few files, no strong build indicators)
		"script.js": "console.log('Hello JS');",
		"helper.js": "function help() {}",
	}

	for file, content := range files {
		suite.createFile(file, content)
	}

	info, err := suite.scanner.ScanProject(suite.tempDir)
	suite.Require().NoError(err)
	suite.NotNil(info)

	// Validate Go has highest priority
	if info.HasLanguage("go") {
		goCtx := info.GetLanguageContext("go")
		suite.NotNil(goCtx)
		suite.Greater(goCtx.Priority, 80, "Go should have high priority")
		suite.Greater(goCtx.Confidence, 0.8, "Go should have high confidence")

		// Go should be the dominant language
		suite.Equal("go", info.DominantLanguage, "Go should be dominant language")
	}

	// Validate Python has medium priority
	if info.HasLanguage("python") {
		pyCtx := info.GetLanguageContext("python")
		suite.NotNil(pyCtx)
		suite.GreaterOrEqual(pyCtx.Priority, 40, "Python should have medium priority")
		suite.Greater(pyCtx.Confidence, 0.6, "Python should have good confidence")
	}

	// Validate JavaScript has lower priority
	if info.HasLanguage("javascript") {
		jsCtx := info.GetLanguageContext("javascript")
		suite.NotNil(jsCtx)
		suite.Less(jsCtx.Priority, 60, "JavaScript should have lower priority")
	}

	// Validate priority ordering
	languages := info.Languages
	if goCtx, exists := languages["go"]; exists {
		for lang, ctx := range languages {
			if lang != "go" {
				suite.GreaterOrEqual(goCtx.Priority, ctx.Priority,
					fmt.Sprintf("Go priority should be >= %s priority", lang))
			}
		}
	}
}

// TestProjectTypeIdentification tests project type identification logic
func (suite *ProjectDetectorTestSuite) TestProjectTypeIdentification() {
	testCases := []struct {
		name         string
		files        map[string]string
		expectedType string
		description  string
	}{
		{
			name: "single-go-project",
			files: map[string]string{
				"go.mod":  "module single\n\ngo 1.19",
				"main.go": "package main\n\nfunc main() {}",
			},
			expectedType: config.ProjectTypeSingle,
			description:  "Single language Go project",
		},
		{
			name: "multi-language-project",
			files: map[string]string{
				"go.mod":    "module multi\n\ngo 1.19",
				"main.go":   "package main\n\nfunc main() {}",
				"script.py": "print('Hello Python')",
				"setup.py":  "from setuptools import setup\n\nsetup(name='multi')",
			},
			expectedType: config.ProjectTypeMulti,
			description:  "Multi-language project with Go and Python",
		},
		{
			name: "frontend-backend-project",
			files: map[string]string{
				"frontend/package.json": "{\"name\": \"frontend\", \"dependencies\": {\"react\": \"^18.0.0\"}}",
				"frontend/src/App.tsx":  "import React from 'react';\n\nconst App = () => <div>App</div>;",
				"backend/go.mod":        "module backend\n\ngo 1.19",
				"backend/main.go":       "package main\n\nfunc main() {}",
			},
			expectedType: config.ProjectTypeFrontendBackend,
			description:  "Frontend-backend project",
		},
		{
			name: "microservices-project",
			files: map[string]string{
				"auth-service/go.mod":      "module auth\n\ngo 1.19",
				"auth-service/main.go":     "package main\n\nfunc main() {}",
				"user-service/pom.xml":     "<?xml version=\"1.0\"?>\n<project><modelVersion>4.0.0</modelVersion></project>",
				"user-service/Main.java":   "public class Main {}",
				"api-gateway/package.json": "{\"name\": \"gateway\", \"dependencies\": {\"express\": \"^4.0.0\"}}",
				"api-gateway/server.js":    "const express = require('express');",
				"docker-compose.yml":       "version: '3.8'\nservices:\n  auth:\n    build: ./auth-service",
			},
			expectedType: config.ProjectTypeMicroservices,
			description:  "Microservices project with multiple languages and build systems",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create fresh temp directory for each test
			tempDir, err := os.MkdirTemp("", fmt.Sprintf("project-type-test-%s-*", tc.name))
			suite.Require().NoError(err)
			defer func() { _ = os.RemoveAll(tempDir) }()

			// Create project files
			for file, content := range tc.files {
				fullPath := filepath.Join(tempDir, file)
				err = os.MkdirAll(filepath.Dir(fullPath), 0755)
				suite.Require().NoError(err)
				err = os.WriteFile(fullPath, []byte(content), 0644)
				suite.Require().NoError(err)
			}

			info, err := suite.scanner.ScanProject(tempDir)
			suite.Require().NoError(err)
			suite.NotNil(info)

			suite.Equal(tc.expectedType, info.ProjectType,
				fmt.Sprintf("%s: Expected project type %s, got %s", tc.description, tc.expectedType, info.ProjectType))
		})
	}
}

// TestIgnorePatterns tests that ignore patterns work correctly
func (suite *ProjectDetectorTestSuite) TestIgnorePatterns() {
	// Create files in directories that should be ignored
	ignoredFiles := map[string]string{
		"node_modules/package/index.js": "console.log('should be ignored');",
		"__pycache__/module.pyc":        "compiled python",
		".git/config":                   "git config",
		"build/output.class":            "compiled java",
		"target/classes/Main.class":     "maven compiled",
		".vscode/settings.json":         "{}",
		"logs/app.log":                  "log entry",
		".DS_Store":                     "mac file",
	}

	// Create files that should NOT be ignored
	validFiles := map[string]string{
		"src/main.go":    "package main\n\nfunc main() {}",
		"lib/utils.py":   "def util(): pass",
		"app/script.js":  "console.log('valid');",
		"test/helper.rb": "def helper; end",
	}

	// Create all files
	for file, content := range ignoredFiles {
		suite.createFile(file, content)
	}
	for file, content := range validFiles {
		suite.createFile(file, content)
	}

	info, err := suite.scanner.ScanProject(suite.tempDir)
	suite.Require().NoError(err)
	suite.NotNil(info)

	// Validate that only valid files are counted
	totalValidFiles := len(validFiles)
	totalDetectedFiles := 0

	for _, ctx := range info.Languages {
		totalDetectedFiles += ctx.FileCount
	}

	suite.Equal(totalValidFiles, totalDetectedFiles,
		"Should only count files that are not in ignored directories")

	// Validate specific languages are detected from valid files only
	suite.True(info.HasLanguage("go"), "Should detect Go from valid files")
	suite.True(info.HasLanguage("python"), "Should detect Python from valid files")
	suite.True(info.HasLanguage("javascript"), "Should detect JavaScript from valid files")
	suite.True(info.HasLanguage("ruby"), "Should detect Ruby from valid files")
}

// TestScannerConfiguration tests scanner configuration methods
func (suite *ProjectDetectorTestSuite) TestScannerConfiguration() {
	scanner := gateway.NewProjectLanguageScanner()
	defer scanner.Shutdown()

	// Test max depth configuration
	scanner.SetMaxDepth(5)
	assert.Equal(suite.T(), 5, scanner.MaxDepth)

	// Test invalid max depth (should not change)
	scanner.SetMaxDepth(0)
	assert.Equal(suite.T(), 5, scanner.MaxDepth) // Should remain unchanged

	scanner.SetMaxDepth(15)
	assert.Equal(suite.T(), 5, scanner.MaxDepth) // Should remain unchanged (max is 10)

	// Test file size limit configuration
	scanner.SetFileSizeLimit(1024 * 1024) // 1MB
	assert.Equal(suite.T(), int64(1024*1024), scanner.FileSizeLimit)

	// Test max files configuration
	scanner.SetMaxFiles(5000)
	assert.Equal(suite.T(), 5000, scanner.MaxFiles)

	// Test ignore patterns
	initialPatternCount := len(scanner.IgnorePatterns)
	scanner.AddIgnorePattern("custom-ignore")
	assert.Equal(suite.T(), initialPatternCount+1, len(scanner.IgnorePatterns))
	assert.Contains(suite.T(), scanner.IgnorePatterns, "custom-ignore")

	// Test optimization configurations
	scanner.OptimizeForLargeMonorepos()
	assert.True(suite.T(), scanner.EnableEarlyExit)
	assert.GreaterOrEqual(suite.T(), scanner.MaxConcurrentScans, 8)
	assert.GreaterOrEqual(suite.T(), scanner.MaxFiles, 50000)

	scanner.OptimizeForSmallProjects()
	assert.False(suite.T(), scanner.EnableEarlyExit)
	assert.LessOrEqual(suite.T(), scanner.MaxConcurrentScans, 4)
	assert.LessOrEqual(suite.T(), scanner.MaxFiles, 1000)
}

// TestFastPathDetection tests fast-path detection for obvious single-language projects
func (suite *ProjectDetectorTestSuite) TestFastPathDetection() {
	testCases := []struct {
		name         string
		filename     string
		content      string
		expectedLang string
		description  string
	}{
		{
			name:         "go-module",
			filename:     "go.mod",
			content:      "module test\n\ngo 1.19",
			expectedLang: "go",
			description:  "Should fast-path detect Go module",
		},
		{
			name:         "rust-cargo",
			filename:     "Cargo.toml",
			content:      "[package]\nname = \"test\"\nversion = \"0.1.0\"",
			expectedLang: "rust",
			description:  "Should fast-path detect Rust project",
		},
		{
			name:         "node-package",
			filename:     "package.json",
			content:      "{\"name\": \"test\", \"version\": \"1.0.0\"}",
			expectedLang: "javascript",
			description:  "Should fast-path detect Node.js project",
		},
		{
			name:         "python-setup",
			filename:     "setup.py",
			content:      "from setuptools import setup\n\nsetup(name='test')",
			expectedLang: "python",
			description:  "Should fast-path detect Python project",
		},
		{
			name:         "python-pyproject",
			filename:     "pyproject.toml",
			content:      "[build-system]\nrequires = [\"setuptools\"]",
			expectedLang: "python",
			description:  "Should fast-path detect Python pyproject",
		},
		{
			name:         "java-maven",
			filename:     "pom.xml",
			content:      "<?xml version=\"1.0\"?>\n<project><modelVersion>4.0.0</modelVersion></project>",
			expectedLang: "java",
			description:  "Should fast-path detect Java Maven project",
		},
		{
			name:         "java-gradle",
			filename:     "build.gradle",
			content:      "plugins {\n    id 'java'\n}",
			expectedLang: "java",
			description:  "Should fast-path detect Java Gradle project",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Create fresh temp directory for each test
			tempDir, err := os.MkdirTemp("", fmt.Sprintf("fast-path-test-%s-*", tc.name))
			suite.Require().NoError(err)
			defer func() { _ = os.RemoveAll(tempDir) }()

			// Create single build file
			fullPath := filepath.Join(tempDir, tc.filename)
			err = os.WriteFile(fullPath, []byte(tc.content), 0644)
			suite.Require().NoError(err)

			start := time.Now()
			info, err := suite.scanner.ScanProjectCached(tempDir)
			duration := time.Since(start)

			suite.Require().NoError(err)
			suite.NotNil(info)

			// Fast-path should be very quick
			suite.Less(duration, 100*time.Millisecond, "Fast-path detection should be very quick")

			// Should detect correct language
			suite.True(info.HasLanguage(tc.expectedLang),
				fmt.Sprintf("Should detect %s language", tc.expectedLang))

			// Should be single-language project
			suite.Equal(config.ProjectTypeSingle, info.ProjectType, "Should detect as single-language project")

			// Should have high priority and confidence
			langCtx := info.GetLanguageContext(tc.expectedLang)
			suite.NotNil(langCtx)
			suite.Equal(100, langCtx.Priority, "Fast-path detected language should have maximum priority")
			suite.Equal(0.95, langCtx.Confidence, "Fast-path detected language should have high confidence")

			// Should have detected the build file
			suite.NotEmpty(langCtx.BuildFiles, "Should have detected build files")
		})
	}
}

// TestCacheIntegration tests cache integration
func (suite *ProjectDetectorTestSuite) TestCacheIntegration() {
	// Create simple project
	suite.createFile("go.mod", "module test\n\ngo 1.19")
	suite.createFile("main.go", "package main\n\nfunc main() {}")

	// First scan (should cache)
	start := time.Now()
	info1, err := suite.scanner.ScanProjectCached(suite.tempDir)
	firstScanDuration := time.Since(start)

	suite.Require().NoError(err)
	suite.NotNil(info1)

	// Second scan (should use cache)
	start = time.Now()
	info2, err := suite.scanner.ScanProjectCached(suite.tempDir)
	secondScanDuration := time.Since(start)

	suite.Require().NoError(err)
	suite.NotNil(info2)

	// Cache should make second scan much faster
	suite.Less(secondScanDuration, firstScanDuration/2, "Cached scan should be much faster")

	// Results should be identical
	suite.Equal(info1.ProjectType, info2.ProjectType)
	suite.Equal(info1.DominantLanguage, info2.DominantLanguage)
	suite.Equal(len(info1.Languages), len(info2.Languages))

	// Test cache invalidation
	suite.scanner.InvalidateCache(suite.tempDir)

	start = time.Now()
	info3, err := suite.scanner.ScanProjectCached(suite.tempDir)
	thirdScanDuration := time.Since(start)

	suite.Require().NoError(err)
	suite.NotNil(info3)

	// After invalidation, should take similar time to first scan
	suite.Greater(thirdScanDuration, secondScanDuration*2,
		"Scan after cache invalidation should be slower than cached scan")

	// Test cache statistics
	stats := suite.scanner.GetCacheStats()
	if stats != nil {
		suite.GreaterOrEqual(stats.HitCount, int64(1), "Should have cache hits")
		suite.GreaterOrEqual(stats.MissCount, int64(1), "Should have cache misses")
		suite.Greater(stats.HitRatio, 0.0, "Should have positive hit ratio")
	}
}

// TestContextValidation tests validation of language contexts
func (suite *ProjectDetectorTestSuite) TestContextValidation() {
	// Create project for context validation
	suite.createFile("go.mod", "module validation-test\n\ngo 1.19")
	suite.createFile("main.go", "package main\n\nfunc main() {}")
	suite.createFile("util.go", "package main\n\nfunc Util() {}")
	suite.createFile("main_test.go", "package main\n\nimport \"testing\"\n\nfunc TestMain(t *testing.T) {}")

	info, err := suite.scanner.ScanProject(suite.tempDir)
	suite.Require().NoError(err)
	suite.NotNil(info)

	// Validate overall project info
	err = info.Validate()
	suite.NoError(err, "Project info should be valid")

	// Validate Go language context
	suite.True(info.HasLanguage("go"), "Should have Go language")
	goCtx := info.GetLanguageContext("go")
	suite.NotNil(goCtx)

	// Test individual context validation
	err = goCtx.Validate()
	suite.NoError(err, "Go context should be valid")

	// Validate context properties
	suite.Equal("go", goCtx.Language)
	suite.Greater(goCtx.FileCount, 0, "Should have source files")
	suite.Greater(goCtx.TestFileCount, 0, "Should have test files")
	suite.GreaterOrEqual(goCtx.Priority, 0, "Priority should be non-negative")
	suite.LessOrEqual(goCtx.Priority, 100, "Priority should not exceed 100")
	suite.GreaterOrEqual(goCtx.Confidence, 0.0, "Confidence should be non-negative")
	suite.LessOrEqual(goCtx.Confidence, 1.0, "Confidence should not exceed 1.0")
	suite.NotEmpty(goCtx.BuildFiles, "Should have build files")
	suite.NotEmpty(goCtx.FileExtensions, "Should have file extensions")

	// Test utility methods
	suite.NotEmpty(goCtx.GetMainSourcePath(), "Should have main source path")
	suite.Contains([]string{"small", "medium", "large"}, goCtx.EstimateProjectSize(), "Should estimate project size")
}

// TestMultiLanguageProjectInfoUtilityMethods tests utility methods on MultiLanguageProjectInfo
func (suite *ProjectDetectorTestSuite) TestMultiLanguageProjectInfoUtilityMethods() {
	// Create multi-language project
	suite.createFile("go.mod", "module test\n\ngo 1.19")
	suite.createFile("main.go", "package main\n\nfunc main() {}")
	suite.createFile("setup.py", "from setuptools import setup\n\nsetup(name='test')")
	suite.createFile("main.py", "print('Hello')")
	suite.createFile("package.json", "{\"name\": \"test\"}")
	suite.createFile("script.js", "console.log('Hello');")

	info, err := suite.scanner.ScanProject(suite.tempDir)
	suite.Require().NoError(err)
	suite.NotNil(info)

	// Test language queries
	suite.True(info.HasLanguage("go"), "Should have Go")
	suite.True(info.HasLanguage("python"), "Should have Python")
	suite.True(info.HasLanguage("javascript"), "Should have JavaScript")
	suite.False(info.HasLanguage("rust"), "Should not have Rust")

	// Test polyglot detection
	suite.True(info.IsPolyglot(), "Should be polyglot project")

	// Test primary/secondary language categorization
	primaryLanguages := info.GetPrimaryLanguages()
	_ = info.GetSecondaryLanguages() // Check method exists but don't need to use result

	suite.NotEmpty(primaryLanguages, "Should have primary languages")
	// May or may not have secondary languages depending on scoring

	// Test workspace root methods
	for _, lang := range []string{"go", "python", "javascript"} {
		if info.HasLanguage(lang) {
			root := info.GetWorkspaceRoot(lang)
			suite.NotEmpty(root, fmt.Sprintf("Should have workspace root for %s", lang))
		}
	}

	// Test LSP server recommendations
	servers := info.GetRecommendedLSPServers()
	suite.NotEmpty(servers, "Should recommend LSP servers")

	expectedServers := []string{"gopls", "pylsp", "typescript-language-server"}
	for _, expectedServer := range expectedServers {
		suite.Contains(servers, expectedServer, fmt.Sprintf("Should recommend %s", expectedServer))
	}

	// Test string representation
	str := info.String()
	suite.NotEmpty(str, "String representation should not be empty")
	suite.Contains(str, info.ProjectType, "String should contain project type")
	suite.Contains(str, info.DominantLanguage, "String should contain dominant language")

	// Test conversion to project context
	ctx := info.ToProjectContext()
	suite.NotNil(ctx, "Should convert to project context")
	suite.Equal(info.ProjectType, ctx.ProjectType, "Project types should match")
	suite.Equal(info.RootPath, ctx.RootDirectory, "Root directories should match")
	suite.GreaterOrEqual(len(ctx.Languages), 2, "Should have multiple languages in context")
}

// TestPerformanceOptimizations tests performance optimization features
func (suite *ProjectDetectorTestSuite) TestPerformanceOptimizations() {
	scanner := gateway.NewProjectLanguageScanner()
	defer scanner.Shutdown()

	// Test optimization configurations
	scanner.SetFastModeThreshold(100)
	scanner.SetMaxConcurrentScans(8)
	scanner.SetEarlyExitEnabled(true)

	// Test optimal configuration detection
	tempDir, err := os.MkdirTemp("", "perf-test-*")
	suite.Require().NoError(err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create small project
	if err := os.WriteFile(filepath.Join(tempDir, "main.go"), []byte("package main\n\nfunc main() {}"), 0644); err != nil {
		suite.T().Logf("Warning: failed to create test file: %v", err)
	}

	config := scanner.GetOptimalConfiguration(tempDir)
	suite.NotNil(config, "Should return optimal configuration")
	suite.Contains(config, "estimated_size", "Should include estimated size")
	suite.Contains(config, "recommendation", "Should include recommendation")

	// Test performance metrics
	metrics := scanner.GetPerformanceMetrics()
	suite.NotNil(metrics, "Should return performance metrics")
	suite.Contains(metrics, "configuration", "Should include configuration metrics")

	if cache, exists := metrics["cache"]; exists {
		cacheMap := cache.(map[string]interface{})
		suite.Contains(cacheMap, "hit_ratio", "Should include cache hit ratio")
		suite.Contains(cacheMap, "total_entries", "Should include total entries")
	}
}

// TestErrorHandling tests error handling in various scenarios
func (suite *ProjectDetectorTestSuite) TestErrorHandling() {
	scanner := gateway.NewProjectLanguageScanner()
	defer scanner.Shutdown()

	// Test scanning non-existent directory
	_, err := scanner.ScanProject("/non/existent/path")
	suite.Error(err, "Should error on non-existent path")

	// Test scanning with invalid configuration
	scanner.SetMaxDepth(-1)
	err = scanner.ValidateConfiguration()
	suite.Error(err, "Should error on invalid max depth")

	scanner.SetFileSizeLimit(-1)
	err = scanner.ValidateConfiguration()
	suite.Error(err, "Should error on invalid file size limit")

	scanner.SetMaxFiles(-1)
	err = scanner.ValidateConfiguration()
	suite.Error(err, "Should error on invalid max files")

	// Test timeout scenarios
	scanner.Timeout = 1 * time.Nanosecond // Very short timeout
	tempDir, err := os.MkdirTemp("", "timeout-test-*")
	suite.Require().NoError(err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create many files to trigger timeout
	for i := 0; i < 1000; i++ {
		_ = os.WriteFile(filepath.Join(tempDir, fmt.Sprintf("file%d.go", i)),
			[]byte(fmt.Sprintf("package main\n\nfunc main%d() {}", i)), 0644)
	}

	_, err = scanner.ScanProject(tempDir)
	// Should either succeed quickly or timeout
	if err != nil {
		suite.Contains(err.Error(), "context deadline exceeded", "Should timeout with context error")
	}
}

// TestConcurrentScanning tests concurrent scanning capabilities
func (suite *ProjectDetectorTestSuite) TestConcurrentScanning() {
	scanner := gateway.NewProjectLanguageScanner()
	defer scanner.Shutdown()

	// Create multiple test projects
	projects := make([]string, 5)
	for i := 0; i < 5; i++ {
		tempDir, err := os.MkdirTemp("", fmt.Sprintf("concurrent-test-%d-*", i))
		suite.Require().NoError(err)
		defer func() { _ = os.RemoveAll(tempDir) }()

		// Create simple Go project
		_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte(fmt.Sprintf("module project-%d\n\ngo 1.19", i)), 0644)
		_ = os.WriteFile(filepath.Join(tempDir, "main.go"), []byte(fmt.Sprintf("package main\n\nfunc main%d() {}", i)), 0644)

		projects[i] = tempDir
	}

	// Test batch scanning
	start := time.Now()
	results := scanner.BatchScanProjects(projects)
	duration := time.Since(start)

	suite.Equal(len(projects), len(results), "Should scan all projects")

	// All projects should be detected correctly
	for projectPath, info := range results {
		suite.NotNil(info, fmt.Sprintf("Project %s should have info", projectPath))
		suite.True(info.HasLanguage("go"), fmt.Sprintf("Project %s should detect Go", projectPath))
		suite.Equal(config.ProjectTypeSingle, info.ProjectType, fmt.Sprintf("Project %s should be single-language", projectPath))
	}

	// Batch scanning should be reasonably fast
	suite.Less(duration, 5*time.Second, "Batch scanning should complete within 5 seconds")
}

// Run the test suite
func TestProjectDetectorTestSuite(t *testing.T) {
	suite.Run(t, new(ProjectDetectorTestSuite))
}

// Individual unit tests for specific components

func TestLanguageContextValidation(t *testing.T) {
	// Test valid context
	validCtx := &gateway.LanguageContext{
		Language:       "go",
		RootPath:       "/test/path",
		FileCount:      5,
		TestFileCount:  2,
		Priority:       85,
		Confidence:     0.9,
		BuildFiles:     []string{"go.mod"},
		ConfigFiles:    []string{},
		SourcePaths:    []string{"src"},
		TestPaths:      []string{"test"},
		FileExtensions: []string{".go"},
		Dependencies:   []string{},
		Metadata:       make(map[string]interface{}),
	}

	err := validCtx.Validate()
	assert.NoError(t, err, "Valid context should pass validation")

	// Test invalid contexts
	invalidCases := []struct {
		name        string
		modify      func(*gateway.LanguageContext)
		errContains string
	}{
		{
			name:        "empty language",
			modify:      func(ctx *gateway.LanguageContext) { ctx.Language = "" },
			errContains: "language cannot be empty",
		},
		{
			name:        "negative file count",
			modify:      func(ctx *gateway.LanguageContext) { ctx.FileCount = -1 },
			errContains: "file count cannot be negative",
		},
		{
			name:        "negative test file count",
			modify:      func(ctx *gateway.LanguageContext) { ctx.TestFileCount = -1 },
			errContains: "test file count cannot be negative",
		},
		{
			name:        "invalid priority high",
			modify:      func(ctx *gateway.LanguageContext) { ctx.Priority = 150 },
			errContains: "priority must be between 0 and 100",
		},
		{
			name:        "invalid priority low",
			modify:      func(ctx *gateway.LanguageContext) { ctx.Priority = -10 },
			errContains: "priority must be between 0 and 100",
		},
		{
			name:        "invalid confidence high",
			modify:      func(ctx *gateway.LanguageContext) { ctx.Confidence = 1.5 },
			errContains: "confidence must be between 0.0 and 1.0",
		},
		{
			name:        "invalid confidence low",
			modify:      func(ctx *gateway.LanguageContext) { ctx.Confidence = -0.1 },
			errContains: "confidence must be between 0.0 and 1.0",
		},
		{
			name:        "relative root path",
			modify:      func(ctx *gateway.LanguageContext) { ctx.RootPath = "relative/path" },
			errContains: "root path must be absolute",
		},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create copy of valid context
			testCtx := *validCtx
			testCtx.BuildFiles = append([]string{}, validCtx.BuildFiles...)
			testCtx.ConfigFiles = append([]string{}, validCtx.ConfigFiles...)
			testCtx.SourcePaths = append([]string{}, validCtx.SourcePaths...)
			testCtx.TestPaths = append([]string{}, validCtx.TestPaths...)
			testCtx.FileExtensions = append([]string{}, validCtx.FileExtensions...)
			testCtx.Dependencies = append([]string{}, validCtx.Dependencies...)
			testCtx.Metadata = make(map[string]interface{})

			// Apply modification
			tc.modify(&testCtx)

			err := testCtx.Validate()
			assert.Error(t, err, "Invalid context should fail validation")
			assert.Contains(t, err.Error(), tc.errContains, "Error should contain expected message")
		})
	}
}

func TestProjectDetectionError(t *testing.T) {
	// Test error with path
	err1 := &gateway.ProjectDetectionError{
		Type:    "validation",
		Message: "invalid configuration",
		Path:    "/test/path",
		Context: "language detection",
	}

	expected1 := "project detection error (validation) at /test/path: invalid configuration"
	assert.Equal(t, expected1, err1.Error())

	// Test error without path
	err2 := &gateway.ProjectDetectionError{
		Type:    "timeout",
		Message: "scan timeout exceeded",
	}

	expected2 := "project detection error (timeout): scan timeout exceeded"
	assert.Equal(t, expected2, err2.Error())
}

func TestLanguageContextUtilityMethods(t *testing.T) {
	ctx := &gateway.LanguageContext{
		Language:       "go",
		RootPath:       "/test/project",
		FileCount:      25,
		TestFileCount:  8,
		Priority:       85,
		Confidence:     0.9,
		BuildFiles:     []string{"go.mod", "go.sum"},
		ConfigFiles:    []string{".golangci.yml"},
		SourcePaths:    []string{"cmd", "internal", "pkg"},
		TestPaths:      []string{"test", "tests"},
		FileExtensions: []string{".go"},
		Framework:      "Gin",
		LSPServerName:  "gopls",
		Dependencies:   []string{"python", "typescript"},
		Metadata:       map[string]interface{}{"key": "value"},
	}

	// Test GetMainSourcePath
	assert.Equal(t, "cmd", ctx.GetMainSourcePath())

	// Test GetMainTestPath
	assert.Equal(t, "test", ctx.GetMainTestPath())

	// Test HasFramework
	assert.True(t, ctx.HasFramework())

	// Test EstimateProjectSize
	size := ctx.EstimateProjectSize()
	assert.Equal(t, "small", size) // 25 + 8 = 33 files = small

	// Test GetDependencyLanguages
	deps := ctx.GetDependencyLanguages()
	assert.Equal(t, []string{"python", "typescript"}, deps)

	// Test with large project
	ctx.FileCount = 400
	ctx.TestFileCount = 100
	size = ctx.EstimateProjectSize()
	assert.Equal(t, "large", size) // 500 files = large

	// Test with no framework
	ctx.Framework = ""
	assert.False(t, ctx.HasFramework())

	// Test with no source paths
	ctx.SourcePaths = []string{}
	assert.Equal(t, "", ctx.GetMainSourcePath())

	// Test with no test paths
	ctx.TestPaths = []string{}
	assert.Equal(t, "", ctx.GetMainTestPath())
}
