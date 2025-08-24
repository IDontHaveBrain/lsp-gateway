package cache

import (
	"encoding/json"
	"lsp-gateway/src/utils"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultPackageName = "main"
	defaultVersion     = "v1.0.0"
)

// detectPackageInfo detects package information from the document URI and language
func (m *SCIPCacheManager) detectPackageInfo(uri string, language string) (packageName string, version string) {
	// Default values for transition period
	defaultPackage := defaultPackageName

	if uri == "" || language == "" {
		return defaultPackage, defaultVersion
	}

	// Extract file path from URI
	filePath := utils.URIToFilePathCached(uri)
	dir := filepath.Dir(filePath)

	// Language-specific package detection
	switch language {
	case "go":
		// Look for go.mod file
		if packageName, version := m.detectGoPackageInfo(dir); packageName != "" {
			return packageName, version
		}
	case "python":
		// Look for setup.py, pyproject.toml, or __init__.py
		if packageName := m.detectPythonPackageInfo(dir); packageName != "" {
			return packageName, defaultVersion
		}
	case "javascript", "typescript":
		// Look for package.json
		if packageName, version := m.detectNodePackageInfo(dir); packageName != "" {
			return packageName, version
		}
	case "java":
		// Look for pom.xml or build.gradle
		if packageName, version := m.detectJavaPackageInfo(dir); packageName != "" {
			return packageName, version
		}
	}

	// Fallback to directory-based detection
	if dirName := filepath.Base(dir); dirName != "" && dirName != "." && dirName != "/" {
		return dirName, defaultVersion
	}

	return defaultPackage, defaultVersion
}

// detectGoPackageInfo detects Go package info from go.mod
func (m *SCIPCacheManager) detectGoPackageInfo(dir string) (string, string) {
	for currentDir := dir; currentDir != "/" && currentDir != "."; currentDir = filepath.Dir(currentDir) {
		goModPath := filepath.Join(currentDir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			// Simple go.mod parsing - look for module line
			if content, err := os.ReadFile(goModPath); err == nil {
				lines := strings.Split(string(content), "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if strings.HasPrefix(line, "module ") {
						moduleName := strings.TrimSpace(strings.TrimPrefix(line, "module"))
						return moduleName, defaultVersion
					}
				}
			}
		}
		if currentDir == filepath.Dir(currentDir) { // Reached root
			break
		}
	}
	return "", ""
}

// detectPythonPackageInfo detects Python package info
func (m *SCIPCacheManager) detectPythonPackageInfo(dir string) string {
	// Look for setup.py or pyproject.toml in parent directories
	for currentDir := dir; currentDir != "/" && currentDir != "."; currentDir = filepath.Dir(currentDir) {
		// Check for setup.py
		if _, err := os.Stat(filepath.Join(currentDir, "setup.py")); err == nil {
			return filepath.Base(currentDir)
		}
		// Check for pyproject.toml
		if _, err := os.Stat(filepath.Join(currentDir, "pyproject.toml")); err == nil {
			return filepath.Base(currentDir)
		}
		if currentDir == filepath.Dir(currentDir) { // Reached root
			break
		}
	}
	return ""
}

// detectNodePackageInfo detects Node.js package info from package.json
func (m *SCIPCacheManager) detectNodePackageInfo(dir string) (string, string) {
	for currentDir := dir; currentDir != "/" && currentDir != "."; currentDir = filepath.Dir(currentDir) {
		packageJSONPath := filepath.Join(currentDir, "package.json")
		if _, err := os.Stat(packageJSONPath); err == nil {
			// Simple package.json parsing - look for name and version
			if content, err := os.ReadFile(packageJSONPath); err == nil {
				var packageData map[string]interface{}
				if json.Unmarshal(content, &packageData) == nil {
					name := ""
					version := defaultVersion
					if n, ok := packageData["name"].(string); ok {
						name = n
					}
					if v, ok := packageData["version"].(string); ok && v != "" {
						version = v
						if !strings.HasPrefix(version, "v") {
							version = "v" + version
						}
					}
					if name != "" {
						return name, version
					}
				}
			}
		}
		if currentDir == filepath.Dir(currentDir) { // Reached root
			break
		}
	}
	return "", ""
}

// detectJavaPackageInfo detects Java package info from pom.xml or build.gradle
func (m *SCIPCacheManager) detectJavaPackageInfo(dir string) (string, string) {
	for currentDir := dir; currentDir != "/" && currentDir != "."; currentDir = filepath.Dir(currentDir) {
		// Check for pom.xml (Maven)
		pomPath := filepath.Join(currentDir, "pom.xml")
		if _, err := os.Stat(pomPath); err == nil {
			// Simple XML parsing - look for artifactId
			if content, err := os.ReadFile(pomPath); err == nil {
				contentStr := string(content)
				if start := strings.Index(contentStr, "<artifactId>"); start != -1 {
					start += len("<artifactId>")
					if end := strings.Index(contentStr[start:], "</artifactId>"); end != -1 {
						artifactID := strings.TrimSpace(contentStr[start : start+end])
						if artifactID != "" {
							return artifactID, defaultVersion
						}
					}
				}
			}
		}

		// Check for build.gradle (Gradle)
		if _, err := os.Stat(filepath.Join(currentDir, "build.gradle")); err == nil {
			return filepath.Base(currentDir), defaultVersion
		}

		if currentDir == filepath.Dir(currentDir) { // Reached root
			break
		}
	}
	return "", ""
}
