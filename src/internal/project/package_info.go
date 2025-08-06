package project

import (
	"bufio"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// PackageInfo represents package metadata extracted from project files
type PackageInfo struct {
	Name       string // package name or module path
	Version    string // version if available (empty if not found)
	Repository string // repository URL if determinable (empty if not found)
	Language   string // associated language
}

// GetPackageInfo extracts package information for a given directory and language
func GetPackageInfo(workingDir, language string) (*PackageInfo, error) {
	if workingDir == "" {
		var err error
		workingDir, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
	}

	// Convert to absolute path for consistency
	absPath, err := filepath.Abs(workingDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}
	workingDir = absPath

	switch language {
	case "go":
		return getGoPackageInfo(workingDir)
	case "python":
		return getPythonPackageInfo(workingDir)
	case "javascript", "typescript":
		return getNodePackageInfo(workingDir, language)
	case "java":
		return getJavaPackageInfo(workingDir)
	default:
		return nil, fmt.Errorf("unsupported language: %s", language)
	}
}

// getGoPackageInfo extracts Go module information from go.mod
func getGoPackageInfo(workingDir string) (*PackageInfo, error) {
	goModPath := filepath.Join(workingDir, "go.mod")
	content, err := os.ReadFile(goModPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &PackageInfo{
				Name:     "unknown-go-project",
				Version:  "",
				Language: "go",
			}, nil
		}
		return nil, fmt.Errorf("failed to read go.mod: %w", err)
	}

	var modulePath, version string
	scanner := bufio.NewScanner(strings.NewReader(string(content)))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Extract module declaration
		if strings.HasPrefix(line, "module ") {
			modulePath = strings.TrimSpace(strings.TrimPrefix(line, "module"))
			// Remove any version suffix if present
			if idx := strings.LastIndex(modulePath, "/v"); idx > 0 {
				if matched, _ := regexp.MatchString(`/v\d+$`, modulePath[idx:]); matched {
					version = modulePath[idx+2:] + ".x.x"
					modulePath = modulePath[:idx]
				}
			}
		}

		// Look for go version directive
		if strings.HasPrefix(line, "go ") && version == "" {
			goVersion := strings.TrimSpace(strings.TrimPrefix(line, "go"))
			version = "go" + goVersion
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to parse go.mod: %w", err)
	}

	if modulePath == "" {
		modulePath = "unknown-go-project"
	}

	// Extract repository URL from module path
	repository := extractRepositoryFromModulePath(modulePath)

	return &PackageInfo{
		Name:       modulePath,
		Version:    version,
		Repository: repository,
		Language:   "go",
	}, nil
}

// getPythonPackageInfo extracts Python package information from various sources
func getPythonPackageInfo(workingDir string) (*PackageInfo, error) {
	// Try pyproject.toml first (modern Python packaging)
	if info := tryPyprojectToml(workingDir); info != nil {
		return info, nil
	}

	// Try setup.py (traditional Python packaging)
	if info := trySetupPy(workingDir); info != nil {
		return info, nil
	}

	// Try __init__.py for version information
	if info := tryPythonInit(workingDir); info != nil {
		return info, nil
	}

	// Fallback to directory name
	dirName := filepath.Base(workingDir)
	return &PackageInfo{
		Name:     dirName,
		Version:  "",
		Language: "python",
	}, nil
}

// tryPyprojectToml attempts to extract package info from pyproject.toml
func tryPyprojectToml(workingDir string) *PackageInfo {
	pyprojectPath := filepath.Join(workingDir, "pyproject.toml")
	content, err := os.ReadFile(pyprojectPath)
	if err != nil {
		return nil
	}

	// Try [tool.poetry] section first (more specific)
	if info := tryPyprojectSection(string(content), "[tool.poetry]"); info != nil {
		return info
	}

	// Fallback to [project] section
	if info := tryPyprojectSection(string(content), "[project]"); info != nil {
		return info
	}

	return nil
}

// tryPyprojectSection attempts to extract info from a specific section in pyproject.toml
func tryPyprojectSection(content, sectionName string) *PackageInfo {
	var name, version, repository string
	scanner := bufio.NewScanner(strings.NewReader(content))
	inSection := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == sectionName {
			inSection = true
			continue
		}

		if inSection && strings.HasPrefix(line, "[") {
			inSection = false
		}

		if inSection {
			if strings.HasPrefix(line, "name") {
				name = extractQuotedValue(line)
			} else if strings.HasPrefix(line, "version") {
				version = extractQuotedValue(line)
			} else if strings.Contains(line, "repository") || strings.Contains(line, "homepage") {
				repository = extractQuotedValue(line)
			}
		}
	}

	if name != "" {
		return &PackageInfo{
			Name:       name,
			Version:    version,
			Repository: repository,
			Language:   "python",
		}
	}

	return nil
}

// trySetupPy attempts to extract package info from setup.py
func trySetupPy(workingDir string) *PackageInfo {
	setupPath := filepath.Join(workingDir, "setup.py")
	content, err := os.ReadFile(setupPath)
	if err != nil {
		return nil
	}

	contentStr := string(content)

	// Extract name from setup() call
	nameRegex := regexp.MustCompile(`name\s*=\s*['"](.*?)['"]`)
	versionRegex := regexp.MustCompile(`version\s*=\s*['"](.*?)['"]`)
	urlRegex := regexp.MustCompile(`url\s*=\s*['"](.*?)['"]`)

	var name, version, repository string

	if matches := nameRegex.FindStringSubmatch(contentStr); len(matches) > 1 {
		name = matches[1]
	}

	if matches := versionRegex.FindStringSubmatch(contentStr); len(matches) > 1 {
		version = matches[1]
	}

	if matches := urlRegex.FindStringSubmatch(contentStr); len(matches) > 1 {
		repository = matches[1]
	}

	if name != "" {
		return &PackageInfo{
			Name:       name,
			Version:    version,
			Repository: repository,
			Language:   "python",
		}
	}

	return nil
}

// tryPythonInit attempts to extract version from __init__.py
func tryPythonInit(workingDir string) *PackageInfo {
	initPath := filepath.Join(workingDir, "__init__.py")
	content, err := os.ReadFile(initPath)
	if err != nil {
		return nil
	}

	contentStr := string(content)
	versionRegex := regexp.MustCompile(`__version__\s*=\s*['"](.*?)['"]`)

	if matches := versionRegex.FindStringSubmatch(contentStr); len(matches) > 1 {
		dirName := filepath.Base(workingDir)
		return &PackageInfo{
			Name:     dirName,
			Version:  matches[1],
			Language: "python",
		}
	}

	return nil
}

// getNodePackageInfo extracts Node.js package information from package.json
func getNodePackageInfo(workingDir, language string) (*PackageInfo, error) {
	packageJsonPath := filepath.Join(workingDir, "package.json")
	content, err := os.ReadFile(packageJsonPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &PackageInfo{
				Name:     "unknown-node-project",
				Version:  "",
				Language: language,
			}, nil
		}
		return nil, fmt.Errorf("failed to read package.json: %w", err)
	}

	var packageData map[string]interface{}
	if err := json.Unmarshal(content, &packageData); err != nil {
		return nil, fmt.Errorf("failed to parse package.json: %w", err)
	}

	name, _ := packageData["name"].(string)
	version, _ := packageData["version"].(string)

	// Extract repository information
	var repository string
	if repo, ok := packageData["repository"]; ok {
		if repoMap, ok := repo.(map[string]interface{}); ok {
			if url, ok := repoMap["url"].(string); ok {
				repository = url
			}
		} else if repoStr, ok := repo.(string); ok {
			repository = repoStr
		}
	}

	// Fallback to homepage if no repository
	if repository == "" {
		if homepage, ok := packageData["homepage"].(string); ok {
			repository = homepage
		}
	}

	if name == "" {
		name = "unknown-node-project"
	}

	return &PackageInfo{
		Name:       name,
		Version:    version,
		Repository: repository,
		Language:   language,
	}, nil
}

// getJavaPackageInfo extracts Java package information from pom.xml or build.gradle
func getJavaPackageInfo(workingDir string) (*PackageInfo, error) {
	// Try Maven first (pom.xml)
	if info := tryMavenPom(workingDir); info != nil {
		return info, nil
	}

	// Try Gradle (build.gradle or build.gradle.kts)
	if info := tryGradleBuild(workingDir); info != nil {
		return info, nil
	}

	// Fallback to directory name
	dirName := filepath.Base(workingDir)
	return &PackageInfo{
		Name:     dirName,
		Version:  "",
		Language: "java",
	}, nil
}

// tryMavenPom attempts to extract package info from pom.xml
func tryMavenPom(workingDir string) *PackageInfo {
	pomPath := filepath.Join(workingDir, "pom.xml")
	content, err := os.ReadFile(pomPath)
	if err != nil {
		return nil
	}

	type PomProject struct {
		GroupID    string `xml:"groupId"`
		ArtifactID string `xml:"artifactId"`
		Version    string `xml:"version"`
		URL        string `xml:"url"`
		SCM        struct {
			URL string `xml:"url"`
		} `xml:"scm"`
	}

	var pom PomProject
	if err := xml.Unmarshal(content, &pom); err != nil {
		return nil
	}

	// Construct name from groupId and artifactId
	var name string
	if pom.GroupID != "" && pom.ArtifactID != "" {
		name = pom.GroupID + ":" + pom.ArtifactID
	} else if pom.ArtifactID != "" {
		name = pom.ArtifactID
	} else {
		return nil
	}

	// Use SCM URL if available, otherwise project URL
	repository := pom.SCM.URL
	if repository == "" {
		repository = pom.URL
	}

	return &PackageInfo{
		Name:       name,
		Version:    pom.Version,
		Repository: repository,
		Language:   "java",
	}
}

// tryGradleBuild attempts to extract package info from build.gradle files
func tryGradleBuild(workingDir string) *PackageInfo {
	gradlePaths := []string{
		filepath.Join(workingDir, "build.gradle"),
		filepath.Join(workingDir, "build.gradle.kts"),
	}

	for _, gradlePath := range gradlePaths {
		content, err := os.ReadFile(gradlePath)
		if err != nil {
			continue
		}

		contentStr := string(content)
		var name, version string

		// Extract group and name from Gradle build script
		groupRegex := regexp.MustCompile(`group\s*[=:]\s*['"]([^'"]*?)['"]`)
		versionRegex := regexp.MustCompile(`version\s*[=:]\s*['"]([^'"]*?)['"]`)
		nameRegex := regexp.MustCompile(`archivesBaseName\s*[=:]\s*['"]([^'"]*?)['"]`)

		if matches := groupRegex.FindStringSubmatch(contentStr); len(matches) > 1 {
			group := matches[1]

			// Look for archivesBaseName or project name
			if nameMatches := nameRegex.FindStringSubmatch(contentStr); len(nameMatches) > 1 {
				name = group + ":" + nameMatches[1]
			} else {
				// Use directory name as artifact ID
				name = group + ":" + filepath.Base(workingDir)
			}
		}

		if matches := versionRegex.FindStringSubmatch(contentStr); len(matches) > 1 {
			version = matches[1]
		}

		if name != "" {
			return &PackageInfo{
				Name:     name,
				Version:  version,
				Language: "java",
			}
		}
	}

	return nil
}

// extractQuotedValue extracts a quoted value from a TOML-style line
func extractQuotedValue(line string) string {
	// Handle both single and double quotes
	regex := regexp.MustCompile(`=\s*['"]([^'"]*?)['"]`)
	if matches := regex.FindStringSubmatch(line); len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// extractRepositoryFromModulePath attempts to construct a repository URL from Go module path
func extractRepositoryFromModulePath(modulePath string) string {
	// Handle common hosting platforms
	if strings.HasPrefix(modulePath, "github.com/") {
		return "https://" + modulePath
	}
	if strings.HasPrefix(modulePath, "gitlab.com/") {
		return "https://" + modulePath
	}
	if strings.HasPrefix(modulePath, "bitbucket.org/") {
		return "https://" + modulePath
	}

	// For other domains, try HTTPS
	if strings.Contains(modulePath, "/") {
		return "https://" + modulePath
	}

	return ""
}
