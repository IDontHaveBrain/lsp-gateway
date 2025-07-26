package project

import (
	"bufio"
	"context"
	"encoding/xml"
	"fmt"
	"lsp-gateway/internal/platform"
	"lsp-gateway/internal/project/types"
	"lsp-gateway/internal/setup"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// JavaLanguageDetector implements comprehensive Java project detection
type JavaLanguageDetector struct {
	logger   *setup.SetupLogger
	executor platform.CommandExecutor
}

// NewJavaLanguageDetector creates a new Java language detector
func NewJavaLanguageDetector() LanguageDetector {
	return &JavaLanguageDetector{
		logger:   setup.NewSetupLogger(&setup.SetupLoggerConfig{Component: "java-detector"}),
		executor: platform.NewCommandExecutor(),
	}
}

func (j *JavaLanguageDetector) DetectLanguage(ctx context.Context, path string) (*types.LanguageDetectionResult, error) {
	startTime := time.Now()

	j.logger.WithField("path", path).Debug("Starting Java project detection")

	result := &types.LanguageDetectionResult{
		Language:        types.PROJECT_TYPE_JAVA,
		RequiredServers: []string{types.SERVER_JDTLS},
		Metadata:        make(map[string]interface{}),
		MarkerFiles:     []string{},
		ConfigFiles:     []string{},
		SourceDirs:      []string{},
		TestDirs:        []string{},
		BuildFiles:      []string{},
		Dependencies:    make(map[string]string),
		DevDependencies: make(map[string]string),
	}

	confidence := 0.0

	// Check for Maven project (pom.xml)
	pomPath := filepath.Join(path, types.MARKER_POM_XML)
	if _, err := os.Stat(pomPath); err == nil {
		result.MarkerFiles = append(result.MarkerFiles, types.MARKER_POM_XML)
		result.ConfigFiles = append(result.ConfigFiles, types.MARKER_POM_XML)
		confidence = 0.95
		result.Metadata["build_system"] = "maven"

		if err := j.parsePomXml(pomPath, result); err != nil {
			j.logger.WithError(err).Warn("Failed to parse pom.xml")
			confidence = 0.8
		}
	}

	// Check for Gradle project (build.gradle or build.gradle.kts)
	gradleFiles := []string{types.MARKER_BUILD_GRADLE, "build.gradle.kts"}
	for _, gradleFile := range gradleFiles {
		gradlePath := filepath.Join(path, gradleFile)
		if _, err := os.Stat(gradlePath); err == nil {
			result.MarkerFiles = append(result.MarkerFiles, gradleFile)
			result.ConfigFiles = append(result.ConfigFiles, gradleFile)
			if confidence == 0 {
				confidence = 0.9
				result.Metadata["build_system"] = types.BUILD_SYSTEM_GRADLE
			} else if confidence > 0 {
				// Both Maven and Gradle - unusual but possible
				result.Metadata["build_system"] = "hybrid"
			}

			if err := j.parseBuildGradle(gradlePath, result); err != nil {
				j.logger.WithError(err).Warn("Failed to parse build.gradle")
			}
		}
	}

	// Check for additional Gradle files
	gradleWrapper := filepath.Join(path, "gradlew")
	if _, err := os.Stat(gradleWrapper); err == nil {
		result.MarkerFiles = append(result.MarkerFiles, "gradlew")
		if confidence == 0 {
			confidence = 0.7
			result.Metadata["build_system"] = types.BUILD_SYSTEM_GRADLE
		}
	}

	// Check for Java files
	javaFileCount := 0
	if err := j.scanForJavaFiles(path, result, &javaFileCount); err != nil {
		j.logger.WithError(err).Debug("Error scanning for Java files")
	}

	// Adjust confidence based on Java files found
	if javaFileCount > 0 {
		if confidence == 0 {
			confidence = 0.8 // Java files without build files
		}
		result.Metadata["java_file_count"] = javaFileCount
	} else if confidence == 0 {
		confidence = 0.1 // No clear Java indicators
	}

	result.Confidence = confidence

	// Detect Java version
	if err := j.detectJavaVersion(ctx, result); err != nil {
		j.logger.WithError(err).Debug("Failed to detect Java version")
	}

	// Detect frameworks and libraries
	j.detectJavaFrameworks(result)

	// Detect project structure
	j.analyzeProjectStructure(path, result)

	// Look for additional configuration files
	j.detectAdditionalConfigs(path, result)

	// Set detection metadata
	result.Metadata["detection_time"] = time.Since(startTime)
	result.Metadata["detector_version"] = types.DETECTOR_VERSION_DEFAULT

	j.logger.WithFields(map[string]interface{}{
		"confidence":     result.Confidence,
		"marker_files":   len(result.MarkerFiles),
		"source_dirs":    len(result.SourceDirs),
		"dependencies":   len(result.Dependencies),
		"detection_time": time.Since(startTime),
	}).Info("Java project detection completed")

	return result, nil
}

// MavenPOM represents a simplified Maven POM structure
type MavenPOM struct {
	XMLName      xml.Name     `xml:"project"`
	GroupId      string       `xml:"groupId"`
	ArtifactId   string       `xml:"artifactId"`
	Version      string       `xml:"version"`
	Packaging    string       `xml:"packaging"`
	Name         string       `xml:"name"`
	Description  string       `xml:"description"`
	Properties   Properties   `xml:"properties"`
	Dependencies Dependencies `xml:"dependencies"`
	Parent       Parent       `xml:"parent"`
}

type Properties struct {
	MavenCompilerSource string `xml:"maven.compiler.source"`
	MavenCompilerTarget string `xml:"maven.compiler.target"`
	JavaVersion         string `xml:"java.version"`
}

type Dependencies struct {
	Dependency []Dependency `xml:"dependency"`
}

type Dependency struct {
	GroupId    string `xml:"groupId"`
	ArtifactId string `xml:"artifactId"`
	Version    string `xml:"version"`
	Scope      string `xml:"scope"`
}

type Parent struct {
	GroupId    string `xml:"groupId"`
	ArtifactId string `xml:"artifactId"`
	Version    string `xml:"version"`
}

func (j *JavaLanguageDetector) parsePomXml(pomPath string, result *types.LanguageDetectionResult) error {
	content, err := os.ReadFile(pomPath)
	if err != nil {
		return fmt.Errorf("failed to read pom.xml: %w", err)
	}

	var pom MavenPOM
	if err := xml.Unmarshal(content, &pom); err != nil {
		return fmt.Errorf("failed to parse pom.xml: %w", err)
	}

	// Extract basic project information
	if pom.GroupId != "" {
		result.Metadata["group_id"] = pom.GroupId
	}
	if pom.ArtifactId != "" {
		result.Metadata["artifact_id"] = pom.ArtifactId
		result.Metadata["project_name"] = pom.ArtifactId
	}
	if pom.Version != "" {
		result.Metadata["project_version"] = pom.Version
	}
	if pom.Packaging != "" {
		result.Metadata["packaging"] = pom.Packaging
	}
	if pom.Name != "" {
		result.Metadata["display_name"] = pom.Name
	}
	if pom.Description != "" {
		result.Metadata["description"] = pom.Description
	}

	// Extract Java version from properties
	if pom.Properties.JavaVersion != "" {
		result.Version = pom.Properties.JavaVersion
	} else if pom.Properties.MavenCompilerSource != "" {
		result.Version = pom.Properties.MavenCompilerSource
	} else if pom.Properties.MavenCompilerTarget != "" {
		result.Version = pom.Properties.MavenCompilerTarget
	}

	// Extract parent information (for Spring Boot projects)
	if pom.Parent.GroupId != "" && pom.Parent.ArtifactId != "" {
		result.Metadata["parent_group_id"] = pom.Parent.GroupId
		result.Metadata["parent_artifact_id"] = pom.Parent.ArtifactId
		result.Metadata["parent_version"] = pom.Parent.Version
	}

	// Extract dependencies
	for _, dep := range pom.Dependencies.Dependency {
		depKey := fmt.Sprintf("%s:%s", dep.GroupId, dep.ArtifactId)
		if dep.Scope == "test" {
			result.DevDependencies[depKey] = dep.Version
		} else {
			result.Dependencies[depKey] = dep.Version
		}
	}

	return nil
}

func (j *JavaLanguageDetector) parseBuildGradle(gradlePath string, result *types.LanguageDetectionResult) error {
	file, err := os.Open(gradlePath)
	if err != nil {
		return fmt.Errorf("failed to open build.gradle: %w", err)
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	var currentSection string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		// Detect sections
		if strings.Contains(line, "dependencies") && strings.Contains(line, "{") {
			currentSection = "dependencies"
			continue
		} else if line == "}" && currentSection != "" {
			currentSection = ""
			continue
		}

		// Parse dependencies
		if currentSection == "dependencies" {
			if err := j.parseGradleDependency(line, result); err != nil {
				j.logger.WithError(err).Debug("Failed to parse Gradle dependency")
			}
		}

		// Parse other configurations
		if strings.Contains(line, "sourceCompatibility") {
			if matches := regexp.MustCompile(`sourceCompatibility\s*=\s*['"]*([^'"]+)['"]*`).FindStringSubmatch(line); len(matches) > 1 {
				result.Version = matches[1]
			}
		}

		if strings.Contains(line, "targetCompatibility") {
			if matches := regexp.MustCompile(`targetCompatibility\s*=\s*['"]*([^'"]+)['"]*`).FindStringSubmatch(line); len(matches) > 1 {
				result.Metadata["target_compatibility"] = matches[1]
			}
		}
	}

	return nil
}

func (j *JavaLanguageDetector) parseGradleDependency(line string, result *types.LanguageDetectionResult) error {
	// Parse different dependency formats:
	// implementation 'group:artifact:version'
	// testImplementation 'group:artifact:version'
	// compile 'group:artifact:version'

	depRegex := regexp.MustCompile(`(implementation|testImplementation|compile|testCompile|api|testApi)\s+['"]+([^'"]+)['"]+`)
	matches := depRegex.FindStringSubmatch(line)

	if len(matches) >= 3 {
		depType := matches[1]
		depString := matches[2]

		// Parse group:artifact:version
		parts := strings.Split(depString, ":")
		if len(parts) >= 2 {
			depKey := fmt.Sprintf("%s:%s", parts[0], parts[1])
			version := ""
			if len(parts) >= 3 {
				version = parts[2]
			}

			if strings.Contains(depType, "test") {
				result.DevDependencies[depKey] = version
			} else {
				result.Dependencies[depKey] = version
			}
		}
	}

	return nil
}

func (j *JavaLanguageDetector) scanForJavaFiles(path string, result *types.LanguageDetectionResult, javaFileCount *int) error {
	sourceDirs := make(map[string]bool)
	testDirs := make(map[string]bool)

	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue on errors
		}

		// Skip build and cache directories
		if info.IsDir() {
			name := info.Name()
			skipDirs := []string{"build", "target", ".gradle", ".mvn", "out", ".idea", ".vscode"}
			for _, skipDir := range skipDirs {
				if name == skipDir {
					return filepath.SkipDir
				}
			}
			return nil
		}

		if strings.HasSuffix(filePath, ".java") {
			*javaFileCount++
			dir := filepath.Dir(filePath)
			relDir, _ := filepath.Rel(path, dir)

			// Determine if it's a test directory
			if strings.Contains(relDir, "test") || strings.Contains(relDir, "Test") {
				testDirs[relDir] = true
			} else {
				sourceDirs[relDir] = true
			}
		}

		// Look for build files
		buildFiles := []string{"Makefile", "Dockerfile", "settings.gradle", "gradle.properties"}
		for _, buildFile := range buildFiles {
			if info.Name() == buildFile {
				result.BuildFiles = append(result.BuildFiles, buildFile)
			}
		}

		return nil
	})

	// Convert maps to slices
	for dir := range sourceDirs {
		if dir != "." {
			result.SourceDirs = append(result.SourceDirs, dir)
		}
	}
	for dir := range testDirs {
		if dir != "." {
			result.TestDirs = append(result.TestDirs, dir)
		}
	}

	return err
}

func (j *JavaLanguageDetector) detectJavaVersion(ctx context.Context, result *types.LanguageDetectionResult) error {
	// Try to get Java version from installed Java
	cmd := []string{"java", "-version"}
	execResult, err := j.executor.Execute(cmd[0], cmd[1:], 10*time.Second)
	if err != nil {
		return err
	}

	if execResult.ExitCode == 0 {
		// Java version output goes to stderr
		output := strings.TrimSpace(execResult.Stderr)
		if output == "" {
			output = strings.TrimSpace(execResult.Stdout)
		}

		// Parse output like: java version "11.0.2" 2019-01-15 LTS
		lines := strings.Split(output, "\n")
		if len(lines) > 0 {
			versionLine := lines[0]
			versionRegex := regexp.MustCompile(`"([^"]+)"`)
			if matches := versionRegex.FindStringSubmatch(versionLine); len(matches) > 1 {
				version := matches[1]
				if result.Version == "" { // Only set if not already set from build file
					result.Version = version
				}
				result.Metadata["installed_java_version"] = version
			}
		}
	}

	return nil
}

func (j *JavaLanguageDetector) detectJavaFrameworks(result *types.LanguageDetectionResult) {
	frameworks := []string{}

	// Check dependencies for known frameworks
	allDeps := make(map[string]string)
	for k, v := range result.Dependencies {
		allDeps[k] = v
	}
	for k, v := range result.DevDependencies {
		allDeps[k] = v
	}

	for dep := range allDeps {
		switch {
		case strings.Contains(dep, "spring-boot"):
			frameworks = append(frameworks, "Spring Boot")
		case strings.Contains(dep, "springframework"):
			frameworks = append(frameworks, "Spring Framework")
		case strings.Contains(dep, "junit"):
			frameworks = append(frameworks, "JUnit")
		case strings.Contains(dep, "testng"):
			frameworks = append(frameworks, "TestNG")
		case strings.Contains(dep, "mockito"):
			frameworks = append(frameworks, "Mockito")
		case strings.Contains(dep, "hibernate"):
			frameworks = append(frameworks, "Hibernate")
		case strings.Contains(dep, "jackson"):
			frameworks = append(frameworks, "Jackson")
		case strings.Contains(dep, "slf4j"):
			frameworks = append(frameworks, "SLF4J")
		case strings.Contains(dep, "logback"):
			frameworks = append(frameworks, "Logback")
		case strings.Contains(dep, "apache.commons"):
			frameworks = append(frameworks, "Apache Commons")
		case strings.Contains(dep, "google.guava"):
			frameworks = append(frameworks, "Google Guava")
		case strings.Contains(dep, "servlet-api"):
			frameworks = append(frameworks, "Java Servlet")
		case strings.Contains(dep, "jersey"):
			frameworks = append(frameworks, "Jersey")
		case strings.Contains(dep, "resteasy"):
			frameworks = append(frameworks, "RESTEasy")
		case strings.Contains(dep, "quarkus"):
			frameworks = append(frameworks, "Quarkus")
		case strings.Contains(dep, "micronaut"):
			frameworks = append(frameworks, "Micronaut")
		case strings.Contains(dep, "vertx"):
			frameworks = append(frameworks, "Vert.x")
		case strings.Contains(dep, "netty"):
			frameworks = append(frameworks, "Netty")
		}
	}

	if len(frameworks) > 0 {
		result.Metadata["frameworks"] = frameworks
	}

	// Detect project type based on parent POM or dependencies
	if parentArtifact, exists := result.Metadata["parent_artifact_id"]; exists {
		if parentStr, ok := parentArtifact.(string); ok {
			switch parentStr {
			case "spring-boot-starter-parent":
				result.Metadata["project_type"] = "Spring Boot Application"
			}
		}
	}
}

func (j *JavaLanguageDetector) analyzeProjectStructure(path string, result *types.LanguageDetectionResult) {
	// Standard Maven/Gradle directory structure
	standardDirs := []string{"src/main/java", "src/test/java", "src/main/resources", "src/test/resources"}
	foundStandardDirs := []string{}

	for _, dir := range standardDirs {
		dirPath := filepath.Join(path, dir)
		if info, err := os.Stat(dirPath); err == nil && info.IsDir() {
			foundStandardDirs = append(foundStandardDirs, dir)
		}
	}

	if len(foundStandardDirs) > 0 {
		result.Metadata["standard_structure"] = foundStandardDirs
		if len(foundStandardDirs) >= 2 {
			result.Confidence = min(result.Confidence+0.05, 1.0)
		}
	}

	// Check for other common Java directories
	commonDirs := []string{"lib", "libs", "target", "build", "out", "classes"}
	foundDirs := []string{}

	for _, dir := range commonDirs {
		dirPath := filepath.Join(path, dir)
		if info, err := os.Stat(dirPath); err == nil && info.IsDir() {
			foundDirs = append(foundDirs, dir)
		}
	}

	if len(foundDirs) > 0 {
		result.Metadata["additional_structure"] = foundDirs
	}
}

func (j *JavaLanguageDetector) detectAdditionalConfigs(path string, result *types.LanguageDetectionResult) {
	configFiles := map[string]string{
		"application.properties":                   "Spring Boot Configuration",
		"application.yml":                          "Spring Boot Configuration",
		"application.yaml":                         "Spring Boot Configuration",
		"logback.xml":                              "Logback Configuration",
		"log4j.properties":                         "Log4j Configuration",
		"log4j2.xml":                               "Log4j2 Configuration",
		"hibernate.cfg.xml":                        "Hibernate Configuration",
		"persistence.xml":                          "JPA Configuration",
		"web.xml":                                  "Java Web Application",
		"beans.xml":                                "CDI Configuration",
		"META-INF/MANIFEST.MF":                     "JAR Manifest",
		".mvn/wrapper/maven-wrapper.properties":    "Maven Wrapper",
		"gradle/wrapper/gradle-wrapper.properties": "Gradle Wrapper",
		"settings.gradle":                          "Gradle Settings",
		"gradle.properties":                        "Gradle Properties",
	}

	for file, description := range configFiles {
		if _, err := os.Stat(filepath.Join(path, file)); err == nil {
			result.ConfigFiles = append(result.ConfigFiles, file)
			if result.Metadata["config_tools"] == nil {
				result.Metadata["config_tools"] = []string{}
			}
			result.Metadata["config_tools"] = append(result.Metadata["config_tools"].([]string), description)
		}
	}
}

// Interface implementation methods
// GetLanguageInfo returns comprehensive information about Java language support
func (j *JavaLanguageDetector) GetLanguageInfo(language string) (*types.LanguageInfo, error) {
	if language != types.PROJECT_TYPE_JAVA {
		return nil, fmt.Errorf("unsupported language: %s", language)
	}

	return &types.LanguageInfo{
		Name:           types.PROJECT_TYPE_JAVA,
		DisplayName:    "Java",
		MinVersion:     "8",
		MaxVersion:     "22",
		BuildTools:     []string{"maven", types.BUILD_SYSTEM_GRADLE, "ant", "sbt"},
		PackageManager: "maven",
		TestFrameworks: []string{"junit", "testng", "spock"},
		LintTools:      []string{"checkstyle", "spotbugs", "pmd"},
		FormatTools:    []string{"google-java-format", "spotless"},
		LSPServers:     []string{types.SERVER_JDTLS},
		FileExtensions: []string{".java", ".class", ".jar"},
		Capabilities:   []string{"completion", "hover", "definition", "references", "formatting", "code_action", "rename"},
		Metadata: map[string]interface{}{
			"jvm_languages": []string{"java", "kotlin", "scala", "groovy"},
			"build_systems": []string{"maven", types.BUILD_SYSTEM_GRADLE},
			"documentation": "docs.oracle.com/javase",
		},
	}, nil
}

func (j *JavaLanguageDetector) GetMarkerFiles() []string {
	return []string{types.MARKER_POM_XML, types.MARKER_BUILD_GRADLE}
}

func (j *JavaLanguageDetector) GetRequiredServers() []string {
	return []string{types.SERVER_JDTLS}
}

func (j *JavaLanguageDetector) GetPriority() int {
	return types.PRIORITY_JAVA
}

func (j *JavaLanguageDetector) ValidateStructure(ctx context.Context, path string) error {
	// Check for either Maven or Gradle build file
	hasBuildFile := false

	pomPath := filepath.Join(path, types.MARKER_POM_XML)
	if _, err := os.Stat(pomPath); err == nil {
		hasBuildFile = true

		// Validate pom.xml is readable
		if _, err := os.ReadFile(pomPath); err != nil {
			return types.NewValidationError(types.PROJECT_TYPE_JAVA, "pom.xml is not readable", err).
				WithMetadata("invalid_files", []string{types.MARKER_POM_XML})
		}
	}

	gradlePath := filepath.Join(path, types.MARKER_BUILD_GRADLE)
	if _, err := os.Stat(gradlePath); err == nil {
		hasBuildFile = true

		// Validate build.gradle is readable
		if _, err := os.ReadFile(gradlePath); err != nil {
			return types.NewValidationError(types.PROJECT_TYPE_JAVA, "build.gradle is not readable", err).
				WithMetadata("invalid_files", []string{types.MARKER_BUILD_GRADLE})
		}
	}

	if !hasBuildFile {
		return types.NewValidationError(types.PROJECT_TYPE_JAVA, "no Java build file found", nil).
			WithMetadata("missing_files", []string{types.MARKER_POM_XML, types.MARKER_BUILD_GRADLE}).
			WithSuggestion("Create a Maven project with pom.xml").
			WithSuggestion("Create a Gradle project with build.gradle").
			WithSuggestion("Ensure you're in the correct Java project directory")
	}

	// Check for Java files
	hasJavaFiles := false
	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if !info.IsDir() && strings.HasSuffix(filePath, ".java") {
			hasJavaFiles = true
			return filepath.SkipAll // Found at least one, can stop
		}

		// Skip build directories
		if info.IsDir() {
			name := info.Name()
			if name == "target" || name == "build" || name == ".gradle" {
				return filepath.SkipDir
			}
		}

		return nil
	})

	if err != nil {
		return types.NewValidationError(types.PROJECT_TYPE_JAVA, "error scanning for Java files", err)
	}

	if !hasJavaFiles {
		return types.NewValidationError(types.PROJECT_TYPE_JAVA, "no Java source files found", nil).
			WithMetadata("structure_issues", []string{"no .java files found in project"})
	}

	return nil
}
