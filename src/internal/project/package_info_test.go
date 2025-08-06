package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetPackageInfo_Go(t *testing.T) {
	// Create temporary directory with go.mod
	tmpDir := t.TempDir()

	goModContent := `module github.com/user/testproject

go 1.21

require (
	github.com/stretchr/testify v1.8.4
)
`

	goModPath := filepath.Join(tmpDir, "go.mod")
	err := os.WriteFile(goModPath, []byte(goModContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	info, err := GetPackageInfo(tmpDir, "go")
	if err != nil {
		t.Fatalf("GetPackageInfo failed: %v", err)
	}

	if info.Name != "github.com/user/testproject" {
		t.Errorf("Expected name 'github.com/user/testproject', got '%s'", info.Name)
	}

	if info.Version != "go1.21" {
		t.Errorf("Expected version 'go1.21', got '%s'", info.Version)
	}

	if info.Repository != "https://github.com/user/testproject" {
		t.Errorf("Expected repository 'https://github.com/user/testproject', got '%s'", info.Repository)
	}

	if info.Language != "go" {
		t.Errorf("Expected language 'go', got '%s'", info.Language)
	}
}

func TestGetPackageInfo_GoWithVersionSuffix(t *testing.T) {
	// Create temporary directory with go.mod containing version suffix
	tmpDir := t.TempDir()

	goModContent := `module github.com/user/testproject/v2

go 1.20
`

	goModPath := filepath.Join(tmpDir, "go.mod")
	err := os.WriteFile(goModPath, []byte(goModContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}

	info, err := GetPackageInfo(tmpDir, "go")
	if err != nil {
		t.Fatalf("GetPackageInfo failed: %v", err)
	}

	if info.Name != "github.com/user/testproject" {
		t.Errorf("Expected name 'github.com/user/testproject', got '%s'", info.Name)
	}

	if info.Version != "2.x.x" {
		t.Errorf("Expected version '2.x.x', got '%s'", info.Version)
	}
}

func TestGetPackageInfo_NodeJS(t *testing.T) {
	// Create temporary directory with package.json
	tmpDir := t.TempDir()

	packageJsonContent := `{
  "name": "test-package",
  "version": "1.2.3",
  "description": "Test package",
  "main": "index.js",
  "repository": {
    "type": "git",
    "url": "https://github.com/user/test-package.git"
  },
  "homepage": "https://github.com/user/test-package"
}
`

	packageJsonPath := filepath.Join(tmpDir, "package.json")
	err := os.WriteFile(packageJsonPath, []byte(packageJsonContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write package.json: %v", err)
	}

	info, err := GetPackageInfo(tmpDir, "javascript")
	if err != nil {
		t.Fatalf("GetPackageInfo failed: %v", err)
	}

	if info.Name != "test-package" {
		t.Errorf("Expected name 'test-package', got '%s'", info.Name)
	}

	if info.Version != "1.2.3" {
		t.Errorf("Expected version '1.2.3', got '%s'", info.Version)
	}

	if info.Repository != "https://github.com/user/test-package.git" {
		t.Errorf("Expected repository 'https://github.com/user/test-package.git', got '%s'", info.Repository)
	}

	if info.Language != "javascript" {
		t.Errorf("Expected language 'javascript', got '%s'", info.Language)
	}
}

func TestGetPackageInfo_Python_PyprojectToml(t *testing.T) {
	// Create temporary directory with pyproject.toml
	tmpDir := t.TempDir()

	pyprojectContent := `[tool.poetry]
name = "test-python-package"
version = "0.1.0"
description = "Test Python package"
repository = "https://github.com/user/test-python-package"

[project]
name = "alt-test-package"
version = "0.2.0"
`

	pyprojectPath := filepath.Join(tmpDir, "pyproject.toml")
	err := os.WriteFile(pyprojectPath, []byte(pyprojectContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write pyproject.toml: %v", err)
	}

	info, err := GetPackageInfo(tmpDir, "python")
	if err != nil {
		t.Fatalf("GetPackageInfo failed: %v", err)
	}

	// Should pick up [tool.poetry] section first
	if info.Name != "test-python-package" {
		t.Errorf("Expected name 'test-python-package', got '%s'", info.Name)
	}

	if info.Version != "0.1.0" {
		t.Errorf("Expected version '0.1.0', got '%s'", info.Version)
	}

	if info.Language != "python" {
		t.Errorf("Expected language 'python', got '%s'", info.Language)
	}
}

func TestGetPackageInfo_Python_SetupPy(t *testing.T) {
	// Create temporary directory with setup.py
	tmpDir := t.TempDir()

	setupPyContent := `from setuptools import setup

setup(
    name='test-setup-package',
    version='1.0.0',
    description='Test setup.py package',
    url='https://github.com/user/test-setup-package',
    packages=['testpackage'],
    install_requires=[
        'requests>=2.25.0',
    ],
)
`

	setupPyPath := filepath.Join(tmpDir, "setup.py")
	err := os.WriteFile(setupPyPath, []byte(setupPyContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write setup.py: %v", err)
	}

	info, err := GetPackageInfo(tmpDir, "python")
	if err != nil {
		t.Fatalf("GetPackageInfo failed: %v", err)
	}

	if info.Name != "test-setup-package" {
		t.Errorf("Expected name 'test-setup-package', got '%s'", info.Name)
	}

	if info.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got '%s'", info.Version)
	}

	if info.Repository != "https://github.com/user/test-setup-package" {
		t.Errorf("Expected repository 'https://github.com/user/test-setup-package', got '%s'", info.Repository)
	}

	if info.Language != "python" {
		t.Errorf("Expected language 'python', got '%s'", info.Language)
	}
}

func TestGetPackageInfo_Java_Maven(t *testing.T) {
	// Create temporary directory with pom.xml
	tmpDir := t.TempDir()

	pomContent := `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0"
         xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
         xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 
         http://maven.apache.org/xsd/maven-4.0.0.xsd">
    <modelVersion>4.0.0</modelVersion>
    
    <groupId>com.example</groupId>
    <artifactId>test-maven-project</artifactId>
    <version>1.0.0-SNAPSHOT</version>
    <packaging>jar</packaging>
    
    <name>Test Maven Project</name>
    <url>https://github.com/user/test-maven-project</url>
    
    <scm>
        <url>https://github.com/user/test-maven-project.git</url>
    </scm>
</project>
`

	pomPath := filepath.Join(tmpDir, "pom.xml")
	err := os.WriteFile(pomPath, []byte(pomContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write pom.xml: %v", err)
	}

	info, err := GetPackageInfo(tmpDir, "java")
	if err != nil {
		t.Fatalf("GetPackageInfo failed: %v", err)
	}

	if info.Name != "com.example:test-maven-project" {
		t.Errorf("Expected name 'com.example:test-maven-project', got '%s'", info.Name)
	}

	if info.Version != "1.0.0-SNAPSHOT" {
		t.Errorf("Expected version '1.0.0-SNAPSHOT', got '%s'", info.Version)
	}

	if info.Repository != "https://github.com/user/test-maven-project.git" {
		t.Errorf("Expected repository 'https://github.com/user/test-maven-project.git', got '%s'", info.Repository)
	}

	if info.Language != "java" {
		t.Errorf("Expected language 'java', got '%s'", info.Language)
	}
}

func TestGetPackageInfo_Java_Gradle(t *testing.T) {
	// Create temporary directory with build.gradle
	tmpDir := t.TempDir()

	buildGradleContent := `plugins {
    id 'java'
}

group = 'com.example'
version = '1.0.0'
archivesBaseName = 'test-gradle-project'

repositories {
    mavenCentral()
}

dependencies {
    testImplementation 'org.junit.jupiter:junit-jupiter:5.8.2'
}
`

	buildGradlePath := filepath.Join(tmpDir, "build.gradle")
	err := os.WriteFile(buildGradlePath, []byte(buildGradleContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write build.gradle: %v", err)
	}

	info, err := GetPackageInfo(tmpDir, "java")
	if err != nil {
		t.Fatalf("GetPackageInfo failed: %v", err)
	}

	if info.Name != "com.example:test-gradle-project" {
		t.Errorf("Expected name 'com.example:test-gradle-project', got '%s'", info.Name)
	}

	if info.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got '%s'", info.Version)
	}

	if info.Language != "java" {
		t.Errorf("Expected language 'java', got '%s'", info.Language)
	}
}

func TestGetPackageInfo_MissingFiles(t *testing.T) {
	// Create empty temporary directory
	tmpDir := t.TempDir()

	// Test Go without go.mod
	info, err := GetPackageInfo(tmpDir, "go")
	if err != nil {
		t.Fatalf("GetPackageInfo failed: %v", err)
	}

	if info.Name != "unknown-go-project" {
		t.Errorf("Expected fallback name 'unknown-go-project', got '%s'", info.Name)
	}

	if info.Version != "" {
		t.Errorf("Expected empty version, got '%s'", info.Version)
	}

	// Test JavaScript without package.json
	info, err = GetPackageInfo(tmpDir, "javascript")
	if err != nil {
		t.Fatalf("GetPackageInfo failed: %v", err)
	}

	if info.Name != "unknown-node-project" {
		t.Errorf("Expected fallback name 'unknown-node-project', got '%s'", info.Name)
	}

	// Test Python without any package files
	info, err = GetPackageInfo(tmpDir, "python")
	if err != nil {
		t.Fatalf("GetPackageInfo failed: %v", err)
	}

	expected := filepath.Base(tmpDir)
	if info.Name != expected {
		t.Errorf("Expected directory name '%s', got '%s'", expected, info.Name)
	}
}

func TestGetPackageInfo_UnsupportedLanguage(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := GetPackageInfo(tmpDir, "unsupported")
	if err == nil {
		t.Error("Expected error for unsupported language, got nil")
	}

	if !strings.Contains(err.Error(), "unsupported language") {
		t.Errorf("Expected 'unsupported language' error, got: %v", err)
	}
}

func TestExtractRepositoryFromModulePath(t *testing.T) {
	tests := []struct {
		modulePath string
		expected   string
	}{
		{"github.com/user/repo", "https://github.com/user/repo"},
		{"gitlab.com/group/project", "https://gitlab.com/group/project"},
		{"bitbucket.org/team/repo", "https://bitbucket.org/team/repo"},
		{"example.com/some/path", "https://example.com/some/path"},
		{"simple", ""},
	}

	for _, test := range tests {
		result := extractRepositoryFromModulePath(test.modulePath)
		if result != test.expected {
			t.Errorf("extractRepositoryFromModulePath(%s) = %s, expected %s",
				test.modulePath, result, test.expected)
		}
	}
}
