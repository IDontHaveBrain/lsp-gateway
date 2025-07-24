package detectors

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"lsp-gateway/internal/project/types"
	"lsp-gateway/internal/setup"
)

// MavenProjectParser handles parsing of Maven pom.xml files
type MavenProjectParser struct {
	logger *setup.SetupLogger
}

// GradleProjectParser handles parsing of Gradle build files
type GradleProjectParser struct {
	logger *setup.SetupLogger
}

// Maven POM XML structures
type MavenPOM struct {
	XMLName      xml.Name           `xml:"project"`
	GroupId      string             `xml:"groupId"`
	ArtifactId   string             `xml:"artifactId"`
	Version      string             `xml:"version"`
	Packaging    string             `xml:"packaging"`
	Name         string             `xml:"name"`
	Description  string             `xml:"description"`
	Parent       *MavenParent       `xml:"parent"`
	Properties   *MavenProperties   `xml:"properties"`
	Dependencies *MavenDependencies `xml:"dependencies"`
	Plugins      *MavenPlugins      `xml:"build>plugins"`
	Profiles     []MavenProfile     `xml:"profiles>profile"`
}

type MavenParent struct {
	GroupId    string `xml:"groupId"`
	ArtifactId string `xml:"artifactId"`
	Version    string `xml:"version"`
}

type MavenProperties struct {
	XMLName xml.Name
	Props   []MavenProperty `xml:",any"`
}

type MavenProperty struct {
	XMLName xml.Name
	Value   string `xml:",chardata"`
}

type MavenDependencies struct {
	Dependencies []MavenDependency `xml:"dependency"`
}

type MavenDependency struct {
	GroupId    string `xml:"groupId"`
	ArtifactId string `xml:"artifactId"`
	Version    string `xml:"version"`
	Scope      string `xml:"scope"`
	Type       string `xml:"type"`
	Classifier string `xml:"classifier"`
	Optional   string `xml:"optional"`
}

type MavenPlugins struct {
	Plugins []MavenPlugin `xml:"plugin"`
}

type MavenPlugin struct {
	GroupId    string `xml:"groupId"`
	ArtifactId string `xml:"artifactId"`
	Version    string `xml:"version"`
}

type MavenProfile struct {
	Id           string             `xml:"id,attr"`
	Dependencies *MavenDependencies `xml:"dependencies"`
}

// Gradle build file structures (simplified parsing)
type GradleBuild struct {
	GroupId      string
	Version      string
	Dependencies map[string]string
	Plugins      []string
	Tasks        []string
	Repositories []string
}

// NewMavenProjectParser creates a new Maven project parser
func NewMavenProjectParser() *MavenProjectParser {
	return &MavenProjectParser{
		logger: setup.NewSetupLogger(&setup.SetupLoggerConfig{Component: "maven-parser"}),
	}
}

// NewGradleProjectParser creates a new Gradle project parser
func NewGradleProjectParser() *GradleProjectParser {
	return &GradleProjectParser{
		logger: setup.NewSetupLogger(&setup.SetupLoggerConfig{Component: "gradle-parser"}),
	}
}

// ParsePom parses a Maven pom.xml file and extracts project information
func (p *MavenProjectParser) ParsePom(pomPath string) (*types.MavenInfo, error) {
	p.logger.Debug("Parsing Maven pom.xml")

	file, err := os.Open(pomPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open pom.xml: %w", err)
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read pom.xml: %w", err)
	}

	var pom MavenPOM
	if err := xml.Unmarshal(data, &pom); err != nil {
		return nil, fmt.Errorf("failed to parse pom.xml: %w", err)
	}

	// Extract Maven information
	info := &types.MavenInfo{
		GroupId:      p.resolveProperty(pom.GroupId, pom.Properties, pom.Parent),
		ArtifactId:   p.resolveProperty(pom.ArtifactId, pom.Properties, pom.Parent),
		Version:      p.resolveProperty(pom.Version, pom.Properties, pom.Parent),
		Packaging:    pom.Packaging,
		Dependencies: make(map[string]string),
		Plugins:      make([]string, 0),
	}

	// Set default packaging
	if info.Packaging == "" {
		info.Packaging = "jar"
	}

	// Extract dependencies
	if pom.Dependencies != nil {
		for _, dep := range pom.Dependencies.Dependencies {
			// Skip test dependencies for main dependency list
			if dep.Scope != types.SCOPE_TEST {
				depKey := fmt.Sprintf("%s:%s", dep.GroupId, dep.ArtifactId)
				version := p.resolveProperty(dep.Version, pom.Properties, pom.Parent)
				info.Dependencies[depKey] = version
			}
		}
	}

	// Extract plugins
	if pom.Plugins != nil {
		for _, plugin := range pom.Plugins.Plugins {
			pluginId := plugin.ArtifactId
			if plugin.GroupId != "" {
				pluginId = fmt.Sprintf("%s:%s", plugin.GroupId, plugin.ArtifactId)
			}
			info.Plugins = append(info.Plugins, pluginId)
		}
	}

	// Handle parent information
	if pom.Parent != nil && info.GroupId == "" {
		info.GroupId = pom.Parent.GroupId
	}
	if pom.Parent != nil && info.Version == "" {
		info.Version = pom.Parent.Version
	}

	p.logger.Debug("Maven pom.xml parsed successfully")
	return info, nil
}

// ExtractDependencies extracts all dependencies from a Maven pom.xml including test dependencies
func (p *MavenProjectParser) ExtractDependencies(pomPath string) (map[string]string, map[string]string, error) {
	p.logger.Debug("Extracting dependencies from pom.xml")

	file, err := os.Open(pomPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open pom.xml: %w", err)
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read pom.xml: %w", err)
	}

	var pom MavenPOM
	if err := xml.Unmarshal(data, &pom); err != nil {
		return nil, nil, fmt.Errorf("failed to parse pom.xml: %w", err)
	}

	mainDeps := make(map[string]string)
	testDeps := make(map[string]string)

	if pom.Dependencies != nil {
		for _, dep := range pom.Dependencies.Dependencies {
			depKey := fmt.Sprintf("%s:%s", dep.GroupId, dep.ArtifactId)
			version := p.resolveProperty(dep.Version, pom.Properties, pom.Parent)

			if dep.Scope == types.SCOPE_TEST {
				testDeps[depKey] = version
			} else {
				mainDeps[depKey] = version
			}
		}
	}

	// Also check profile dependencies
	for _, profile := range pom.Profiles {
		if profile.Dependencies != nil {
			for _, dep := range profile.Dependencies.Dependencies {
				depKey := fmt.Sprintf("%s:%s", dep.GroupId, dep.ArtifactId)
				version := p.resolveProperty(dep.Version, pom.Properties, pom.Parent)

				if dep.Scope == types.SCOPE_TEST {
					testDeps[depKey] = version
				} else {
					mainDeps[depKey] = version
				}
			}
		}
	}

	return mainDeps, testDeps, nil
}

// resolveProperty resolves Maven properties like ${property.name}
func (p *MavenProjectParser) resolveProperty(value string, props *MavenProperties, parent *MavenParent) string {
	if value == "" {
		return ""
	}

	// Handle common Maven built-in properties
	if strings.Contains(value, "${") {
		// Replace parent properties
		if parent != nil {
			value = strings.ReplaceAll(value, "${project.parent.groupId}", parent.GroupId)
			value = strings.ReplaceAll(value, "${project.parent.version}", parent.Version)
		}

		// Replace custom properties
		if props != nil {
			for _, prop := range props.Props {
				propName := prop.XMLName.Local
				propValue := prop.Value
				placeholder := fmt.Sprintf("${%s}", propName)
				value = strings.ReplaceAll(value, placeholder, propValue)
			}
		}

		// Remove any remaining unresolved properties
		propertyRegex := regexp.MustCompile(`\$\{[^}]+\}`)
		value = propertyRegex.ReplaceAllString(value, "")
	}

	return value
}

// ParseGradle parses a Gradle build file and extracts project information
func (g *GradleProjectParser) ParseGradle(gradlePath string) (*types.GradleInfo, error) {
	g.logger.Debug("Parsing Gradle build file")

	content, err := os.ReadFile(gradlePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read Gradle build file: %w", err)
	}

	contentStr := string(content)

	info := &types.GradleInfo{
		Dependencies: make(map[string]string),
		Plugins:      make([]string, 0),
		Tasks:        make([]string, 0),
	}

	// Parse group and version
	info.GroupId = g.extractGradleProperty(contentStr, "group")
	info.Version = g.extractGradleProperty(contentStr, "version")

	// Extract dependencies
	dependencies, err := g.extractGradleDependencies(contentStr)
	if err != nil {
		g.logger.Warn(fmt.Sprintf("Failed to extract Gradle dependencies: %v", err))
	} else {
		info.Dependencies = dependencies
	}

	// Extract plugins
	plugins, err := g.extractGradlePlugins(contentStr)
	if err != nil {
		g.logger.Warn(fmt.Sprintf("Failed to extract Gradle plugins: %v", err))
	} else {
		info.Plugins = plugins
	}

	// Extract tasks
	tasks, err := g.extractGradleTasks(contentStr)
	if err != nil {
		g.logger.Warn(fmt.Sprintf("Failed to extract Gradle tasks: %v", err))
	} else {
		info.Tasks = tasks
	}

	g.logger.Debug("Gradle build file parsed successfully")
	return info, nil
}

// extractGradleProperty extracts a property value from Gradle build script
func (g *GradleProjectParser) extractGradleProperty(content, property string) string {
	// Try various formats: group = "value", group='value', group "value"
	patterns := []string{
		fmt.Sprintf(`%s\s*=\s*["']([^"']+)["']`, property),
		fmt.Sprintf(`%s\s+["']([^"']+)["']`, property),
		fmt.Sprintf(`%s\s*=\s*([^\s\n]+)`, property),
	}

	for _, pattern := range patterns {
		regex := regexp.MustCompile(pattern)
		matches := regex.FindStringSubmatch(content)
		if len(matches) > 1 {
			return strings.Trim(matches[1], `"'`)
		}
	}

	return ""
}

// extractGradleDependencies extracts dependencies from Gradle build script
func (g *GradleProjectParser) extractGradleDependencies(content string) (map[string]string, error) {
	dependencies := make(map[string]string)

	// Find dependencies block
	dependenciesRegex := regexp.MustCompile(`dependencies\s*\{([^}]*(?:\{[^}]*\}[^}]*)*)\}`)
	matches := dependenciesRegex.FindStringSubmatch(content)
	
	if len(matches) < 2 {
		return dependencies, nil
	}

	depsBlock := matches[1]

	// Extract dependency declarations
	// Matches: implementation 'group:artifact:version'
	//          compile "group:artifact:version"
	//          testImplementation group: 'group', name: 'artifact', version: 'version'
	depPatterns := []string{
		`(implementation|compile|api|testImplementation|testCompile|runtimeOnly)\s+["']([^"']+)["']`,
		`(implementation|compile|api|testImplementation|testCompile|runtimeOnly)\s+group:\s*["']([^"']+)["'],\s*name:\s*["']([^"']+)["'],\s*version:\s*["']([^"']+)["']`,
	}

	for _, pattern := range depPatterns {
		regex := regexp.MustCompile(pattern)
		allMatches := regex.FindAllStringSubmatch(depsBlock, -1)
		
		for _, match := range allMatches {
			if len(match) >= 3 {
				scope := match[1]
				
				// Skip test dependencies for main dependency list
				if strings.Contains(scope, types.SCOPE_TEST) {
					continue
				}

				if len(match) == 3 {
					// Format: implementation 'group:artifact:version'
					depCoord := match[2]
					parts := strings.Split(depCoord, ":")
					if len(parts) >= 2 {
						version := ""
						if len(parts) >= 3 {
							version = parts[2]
						}
						depKey := fmt.Sprintf("%s:%s", parts[0], parts[1])
						dependencies[depKey] = version
					}
				} else if len(match) == 5 {
					// Format: implementation group: 'group', name: 'artifact', version: 'version'
					group := match[2]
					name := match[3]
					version := match[4]
					depKey := fmt.Sprintf("%s:%s", group, name)
					dependencies[depKey] = version
				}
			}
		}
	}

	return dependencies, nil
}

// extractGradlePlugins extracts plugins from Gradle build script
func (g *GradleProjectParser) extractGradlePlugins(content string) ([]string, error) {
	var plugins []string

	// Find plugins block
	pluginsRegex := regexp.MustCompile(`plugins\s*\{([^}]*)\}`)
	matches := pluginsRegex.FindStringSubmatch(content)
	
	if len(matches) < 2 {
		// Also check for legacy apply plugin syntax
		applyRegex := regexp.MustCompile(`apply\s+plugin:\s*["']([^"']+)["']`)
		applyMatches := applyRegex.FindAllStringSubmatch(content, -1)
		for _, match := range applyMatches {
			if len(match) >= 2 {
				plugins = append(plugins, match[1])
			}
		}
		return plugins, nil
	}

	pluginsBlock := matches[1]

	// Extract plugin declarations
	// Matches: id 'plugin-name' version 'version'
	//          id "plugin-name"
	//          kotlin("jvm") version "version"
	pluginPatterns := []string{
		`id\s+["']([^"']+)["']`,
		`kotlin\(["']([^"']+)["']\)`,
		`(\w+)\s*\{`,
	}

	for _, pattern := range pluginPatterns {
		regex := regexp.MustCompile(pattern)
		allMatches := regex.FindAllStringSubmatch(pluginsBlock, -1)
		
		for _, match := range allMatches {
			if len(match) >= 2 {
				plugin := match[1]
				// Skip common keywords that aren't plugins
				if plugin != "version" && plugin != "apply" {
					plugins = append(plugins, plugin)
				}
			}
		}
	}

	return plugins, nil
}

// extractGradleTasks extracts custom tasks from Gradle build script
func (g *GradleProjectParser) extractGradleTasks(content string) ([]string, error) {
	var tasks []string

	// Find task declarations
	// Matches: task taskName { ... }
	//          task taskName(type: TaskType) { ... }
	//          tasks.register("taskName") { ... }
	taskPatterns := []string{
		`task\s+(\w+)\s*(?:\([^)]*\))?\s*\{`,
		`tasks\.register\(["'](\w+)["']\)`,
		`tasks\.create\(["'](\w+)["']\)`,
	}

	for _, pattern := range taskPatterns {
		regex := regexp.MustCompile(pattern)
		allMatches := regex.FindAllStringSubmatch(content, -1)
		
		for _, match := range allMatches {
			if len(match) >= 2 {
				tasks = append(tasks, match[1])
			}
		}
	}

	return tasks, nil
}

// ParseSettings parses Gradle settings.gradle file for multi-project builds
func (g *GradleProjectParser) ParseSettings(settingsPath string) ([]string, error) {
	g.logger.Debug("Parsing Gradle settings file")

	content, err := os.ReadFile(settingsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read settings.gradle: %w", err)
	}

	contentStr := string(content)
	var subprojects []string

	// Find include statements
	// Matches: include 'project-name'
	//          include ':project-name'
	//          include 'parent:child'
	includeRegex := regexp.MustCompile(`include\s+["']([^"']+)["']`)
	matches := includeRegex.FindAllStringSubmatch(contentStr, -1)

	for _, match := range matches {
		if len(match) >= 2 {
			project := strings.TrimPrefix(match[1], ":")
			subprojects = append(subprojects, project)
		}
	}

	return subprojects, nil
}


// ValidatePomXML validates Maven pom.xml for common issues
func (p *MavenProjectParser) ValidatePomXML(pomPath string) []string {
	var issues []string

	if _, err := os.Stat(pomPath); os.IsNotExist(err) {
		issues = append(issues, "pom.xml file not found")
		return issues
	}

	file, err := os.Open(pomPath)
	if err != nil {
		issues = append(issues, fmt.Sprintf("Cannot read pom.xml: %v", err))
		return issues
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(file)
	if err != nil {
		issues = append(issues, fmt.Sprintf("Cannot read pom.xml content: %v", err))
		return issues
	}

	var pom MavenPOM
	if err := xml.Unmarshal(data, &pom); err != nil {
		issues = append(issues, fmt.Sprintf("Invalid XML in pom.xml: %v", err))
		return issues
	}

	// Validate required fields
	if pom.GroupId == "" && (pom.Parent == nil || pom.Parent.GroupId == "") {
		issues = append(issues, "Missing groupId in pom.xml")
	}

	if pom.ArtifactId == "" {
		issues = append(issues, "Missing artifactId in pom.xml")
	}

	if pom.Version == "" && (pom.Parent == nil || pom.Parent.Version == "") {
		issues = append(issues, "Missing version in pom.xml")
	}

	return issues
}

// ValidateGradleBuild validates Gradle build file for common issues
func (g *GradleProjectParser) ValidateGradleBuild(gradlePath string) []string {
	var issues []string

	if _, err := os.Stat(gradlePath); os.IsNotExist(err) {
		issues = append(issues, "Gradle build file not found")
		return issues
	}

	content, err := os.ReadFile(gradlePath)
	if err != nil {
		issues = append(issues, fmt.Sprintf("Cannot read Gradle build file: %v", err))
		return issues
	}

	contentStr := string(content)

	// Check for basic structure
	if !strings.Contains(contentStr, "plugins") && !strings.Contains(contentStr, "apply plugin") {
		issues = append(issues, "No plugins declared in Gradle build file")
	}

	// Check for repositories (usually required for dependency resolution)
	if !strings.Contains(contentStr, "repositories") {
		issues = append(issues, "No repositories declared in Gradle build file")
	}

	// Validate syntax by checking for balanced braces
	openBraces := strings.Count(contentStr, "{")
	closeBraces := strings.Count(contentStr, "}")
	if openBraces != closeBraces {
		issues = append(issues, "Unbalanced braces in Gradle build file")
	}

	return issues
}