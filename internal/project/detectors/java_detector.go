package detectors

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lsp-gateway/internal/platform"
	"lsp-gateway/internal/project/types"
	"lsp-gateway/internal/setup"
)

// JavaProjectDetector provides comprehensive Java project detection and analysis
type JavaProjectDetector struct {
	logger         *setup.SetupLogger
	executor       platform.CommandExecutor
	versionChecker *setup.VersionChecker
	javaDetector   *setup.JavaDetector
	mavenParser    *MavenProjectParser
	gradleParser   *GradleProjectParser
	timeout        time.Duration
}

// JavaDetectionMetadata contains detailed Java project analysis results
type JavaDetectionMetadata struct {
	JavaVersion      string                       `json:"java_version,omitempty"`
	BuildSystem      string                       `json:"build_system,omitempty"`
	JavaHome         string                       `json:"java_home,omitempty"`
	MavenInfo        *types.MavenInfo           `json:"maven_info,omitempty"`
	GradleInfo       *types.GradleInfo          `json:"gradle_info,omitempty"`
	Dependencies     map[string]*JavaDependency   `json:"dependencies,omitempty"`
	DevDependencies  map[string]*JavaDependency   `json:"dev_dependencies,omitempty"`
	TestFrameworks   []string                     `json:"test_frameworks,omitempty"`
	ProjectType      string                       `json:"project_type,omitempty"`
	SourceDirs       []string                     `json:"source_dirs,omitempty"`
	TestDirs         []string                     `json:"test_dirs,omitempty"`
	EntryPoints      map[string]string            `json:"entry_points,omitempty"`
}

// JavaDependency contains detailed dependency information
type JavaDependency struct {
	GroupId      string `json:"group_id"`
	ArtifactId   string `json:"artifact_id"`
	Version      string `json:"version,omitempty"`
	Scope        string `json:"scope,omitempty"`
	Classifier   string `json:"classifier,omitempty"`
	Type         string `json:"type,omitempty"`
	Source       string `json:"source,omitempty"` // pom.xml, build.gradle, etc.
}

// NewJavaProjectDetector creates a new comprehensive Java project detector
func NewJavaProjectDetector() *JavaProjectDetector {
	return &JavaProjectDetector{
		logger:         setup.NewSetupLogger(&setup.SetupLoggerConfig{Component: "java-detector"}),
		executor:       platform.NewCommandExecutor(),
		versionChecker: setup.NewVersionChecker(),
		javaDetector:   setup.NewJavaDetector(),
		mavenParser:    NewMavenProjectParser(),
		gradleParser:   NewGradleProjectParser(),
		timeout:        30 * time.Second,
	}
}

// DetectLanguage performs comprehensive Java project detection
func (d *JavaProjectDetector) DetectLanguage(ctx context.Context, path string) (*types.LanguageDetectionResult, error) {
	d.logger.Debug("Starting Java project detection")

	timeoutCtx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()

	// Phase 1: Detect Java runtime and basic project markers
	runtimeInfo, err := d.detectJavaRuntime(timeoutCtx, path)
	if err != nil {
		return nil, types.NewDetectionError(types.PROJECT_TYPE_JAVA, "runtime", path,
			fmt.Sprintf("Failed to detect Java runtime: %v", err), err)
	}

	// Phase 2: Analyze build system configuration files
	buildSystemAnalysis, err := d.analyzeBuildSystemConfigs(timeoutCtx, path)
	if err != nil {
		return nil, types.NewDetectionError(types.PROJECT_TYPE_JAVA, "build_system", path,
			fmt.Sprintf("Failed to analyze build system: %v", err), err)
	}

	// Phase 3: Validate project structure
	structureValidation, err := d.validateJavaProjectStructure(timeoutCtx, path, buildSystemAnalysis.BuildSystem)
	if err != nil {
		return nil, types.NewDetectionError(types.PROJECT_TYPE_JAVA, "structure", path,
			fmt.Sprintf("Failed to validate project structure: %v", err), err)
	}

	// Phase 4: Create comprehensive detection result
	result := &types.LanguageDetectionResult{
		Language:        types.PROJECT_TYPE_JAVA,
		Confidence:      d.calculateConfidence(runtimeInfo, buildSystemAnalysis, structureValidation),
		Version:         runtimeInfo.JavaVersion,
		MarkerFiles:     d.findMarkerFiles(path),
		RequiredServers: d.GetRequiredServers(),
	}

	d.logger.Debug("Java project detection completed")
	return result, nil
}

// GetLanguageInfo returns comprehensive information about Java language support
func (d *JavaProjectDetector) GetLanguageInfo(language string) (*types.LanguageInfo, error) {
	if language != types.PROJECT_TYPE_JAVA {
		return nil, fmt.Errorf("unsupported language: %s", language)
	}

	return &types.LanguageInfo{
		Name:           types.PROJECT_TYPE_JAVA,
		DisplayName:    "Java",
		MinVersion:     "8",
		MaxVersion:     "22",
		BuildTools:     []string{"maven", "gradle", "ant"},
		PackageManager: "maven",
		TestFrameworks: []string{"junit", "testng", "spock"},
		LintTools:      []string{"checkstyle", "spotbugs", "pmd"},
		FormatTools:    []string{"google-java-format", "spotless"},
		LSPServers:     []string{"jdtls"},
		FileExtensions: []string{".java", ".class", ".jar"},
		Capabilities:   []string{"completion", "hover", "definition", "references", "formatting"},
	}, nil
}

// GetMarkerFiles returns the list of files that indicate Java project presence
func (d *JavaProjectDetector) GetMarkerFiles() []string {
	return []string{"pom.xml", "build.gradle", "build.gradle.kts", "settings.gradle", "gradle.properties", "gradlew"}
}

// GetRequiredServers returns the list of LSP servers required for Java
func (d *JavaProjectDetector) GetRequiredServers() []string {
	return []string{"jdtls"}
}

// GetPriority returns the detection priority for Java projects
func (d *JavaProjectDetector) GetPriority() int {
	return 80 // High priority for Java projects
}

// ValidateStructure performs Java-specific validation of project structure
func (d *JavaProjectDetector) ValidateStructure(ctx context.Context, rootPath string) error {
	d.logger.Debug("Validating Java project structure")

	// Check for required Java marker files
	hasMarkerFile := false
	for _, markerFile := range d.GetMarkerFiles() {
		if _, err := os.Stat(filepath.Join(rootPath, markerFile)); err == nil {
			hasMarkerFile = true
			break
		}
	}

	if !hasMarkerFile {
		return types.NewValidationError(types.PROJECT_TYPE_JAVA,
			"No Java build configuration files found",
			nil)
	}

	// Validate source directory structure
	sourceDirs := []string{"src/main/java", "src", "main"}
	hasSourceDir := false
	for _, sourceDir := range sourceDirs {
		if stat, err := os.Stat(filepath.Join(rootPath, sourceDir)); err == nil && stat.IsDir() {
			hasSourceDir = true
			break
		}
	}

	if !hasSourceDir {
		return types.NewValidationError(types.PROJECT_TYPE_JAVA,
			"No Java source directories found",
			nil)
	}

	return nil
}

// detectJavaRuntime detects Java runtime information
func (d *JavaProjectDetector) detectJavaRuntime(ctx context.Context, path string) (*JavaRuntimeAnalysis, error) {
	d.logger.Debug("Detecting Java runtime")

	// Use existing Java detector from setup package
	javaInfo, err := d.javaDetector.DetectJava()
	if err != nil {
		return nil, fmt.Errorf("failed to detect Java runtime: %w", err)
	}

	analysis := &JavaRuntimeAnalysis{
		JavaVersion:  javaInfo.Version,
		JavaHome:     javaInfo.JavaHome,
		Distribution: javaInfo.Distribution,
		IsJDK:        javaInfo.IsJDK,
		Issues:       javaInfo.Issues,
	}

	// Check if Java is compatible for LSP
	if !javaInfo.Compatible {
		analysis.Issues = append(analysis.Issues, "Java version may not be compatible with Java Language Server")
	}

	if !javaInfo.IsJDK {
		analysis.Issues = append(analysis.Issues, "JDK required for Java Language Server functionality")
	}

	return analysis, nil
}

// analyzeBuildSystemConfigs analyzes Maven and Gradle configuration files
func (d *JavaProjectDetector) analyzeBuildSystemConfigs(ctx context.Context, path string) (*BuildSystemAnalysis, error) {
	d.logger.Debug("Analyzing build system configurations")

	analysis := &BuildSystemAnalysis{
		BuildSystem: "unknown",
	}

	// Check for Maven
	pomPath := filepath.Join(path, "pom.xml")
	if _, err := os.Stat(pomPath); err == nil {
		d.logger.Debug("Maven project detected")
		analysis.BuildSystem = "maven"
		
		mavenInfo, err := d.mavenParser.ParsePom(pomPath)
		if err != nil {
			d.logger.Warn("Failed to parse pom.xml")
			analysis.Issues = append(analysis.Issues, fmt.Sprintf("Failed to parse pom.xml: %v", err))
		} else {
			analysis.MavenInfo = mavenInfo
		}
	}

	// Check for Gradle
	gradlePaths := []string{
		filepath.Join(path, "build.gradle"),
		filepath.Join(path, "build.gradle.kts"),
	}

	for _, gradlePath := range gradlePaths {
		if _, err := os.Stat(gradlePath); err == nil {
			d.logger.Debug("Gradle project detected")
			
			// If we already found Maven, this is a mixed build system
			if analysis.BuildSystem == "maven" {
				analysis.BuildSystem = "mixed"
				analysis.Issues = append(analysis.Issues, "Both Maven and Gradle configurations detected")
			} else {
				analysis.BuildSystem = "gradle"
			}

			gradleInfo, err := d.gradleParser.ParseGradle(gradlePath)
			if err != nil {
				d.logger.Warn("Failed to parse build.gradle")
				analysis.Issues = append(analysis.Issues, fmt.Sprintf("Failed to parse %s: %v", gradlePath, err))
			} else {
				analysis.GradleInfo = gradleInfo
			}
			break
		}
	}

	if analysis.BuildSystem == "unknown" {
		analysis.Issues = append(analysis.Issues, "No recognized build system configuration found")
	}

	return analysis, nil
}

// validateJavaProjectStructure validates the Java project directory structure
func (d *JavaProjectDetector) validateJavaProjectStructure(ctx context.Context, path, buildSystem string) (*StructureValidation, error) {
	d.logger.Debug("Validating Java project structure")

	validation := &StructureValidation{
		IsValid:        true,
		SourceDirs:     []string{},
		TestDirs:       []string{},
		ConfigFiles:    []string{},
		Issues:         []string{},
	}

	// Standard Java directory structures to check
	standardDirs := map[string]string{
		"src/main/java":      "source",
		"src/test/java":      "test",
		"src/main/resources": "resources",
		"src/test/resources": "test_resources",
	}

	// Alternative directory structures
	altDirs := map[string]string{
		"src":  "source",
		"test": "test",
		"main": "source",
	}

	// Check standard directories first
	foundSourceDir := false
	for dir, dirType := range standardDirs {
		fullPath := filepath.Join(path, dir)
		if stat, err := os.Stat(fullPath); err == nil && stat.IsDir() {
			switch dirType {
			case "source":
				validation.SourceDirs = append(validation.SourceDirs, dir)
				foundSourceDir = true
			case "test":
				validation.TestDirs = append(validation.TestDirs, dir)
			}
		}
	}

	// If no standard source dir found, check alternatives
	if !foundSourceDir {
		for dir, dirType := range altDirs {
			fullPath := filepath.Join(path, dir)
			if stat, err := os.Stat(fullPath); err == nil && stat.IsDir() {
				if dirType == "source" {
					validation.SourceDirs = append(validation.SourceDirs, dir)
					foundSourceDir = true
					break
				}
			}
		}
	}

	if !foundSourceDir {
		validation.IsValid = false
		validation.Issues = append(validation.Issues, "No Java source directories found")
	}

	// Check for configuration files
	configFiles := []string{"pom.xml", "build.gradle", "build.gradle.kts", "settings.gradle", "gradle.properties"}
	for _, configFile := range configFiles {
		if _, err := os.Stat(filepath.Join(path, configFile)); err == nil {
			validation.ConfigFiles = append(validation.ConfigFiles, configFile)
		}
	}

	return validation, nil
}

// Helper methods

func (d *JavaProjectDetector) calculateConfidence(runtime *JavaRuntimeAnalysis, buildSystem *BuildSystemAnalysis, structure *StructureValidation) float64 {
	confidence := 0.0

	// Base confidence for having Java runtime
	if runtime.JavaVersion != "" {
		confidence += 0.3
	}

	// Build system configuration
	switch buildSystem.BuildSystem {
	case "maven":
		confidence += 0.4
	case "gradle":
		confidence += 0.4
	case "mixed":
		confidence += 0.2 // Lower confidence for mixed systems
	}

	// Project structure
	if structure.IsValid {
		confidence += 0.3
	}

	// Bonus for having proper source directories
	if len(structure.SourceDirs) > 0 {
		confidence += 0.1
	}

	// Cap at 1.0
	if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence
}

func (d *JavaProjectDetector) findMarkerFiles(path string) []string {
	var found []string
	for _, markerFile := range d.GetMarkerFiles() {
		if _, err := os.Stat(filepath.Join(path, markerFile)); err == nil {
			found = append(found, markerFile)
		}
	}
	return found
}

func (d *JavaProjectDetector) createProjectMetadata(runtime *JavaRuntimeAnalysis, buildSystem *BuildSystemAnalysis, structure *StructureValidation) interface{} {
	metadata := &JavaDetectionMetadata{
		JavaVersion:   runtime.JavaVersion,
		BuildSystem:   buildSystem.BuildSystem,
		JavaHome:      runtime.JavaHome,
		ProjectType:   "application", // Default, could be enhanced
		SourceDirs:    structure.SourceDirs,
		TestDirs:      structure.TestDirs,
		Dependencies:  make(map[string]*JavaDependency),
		EntryPoints:   make(map[string]string),
	}

	if buildSystem.MavenInfo != nil {
		metadata.MavenInfo = buildSystem.MavenInfo
		// Convert Maven dependencies to Java dependencies
		for name, version := range buildSystem.MavenInfo.Dependencies {
			parts := strings.Split(name, ":")
			if len(parts) >= 2 {
				metadata.Dependencies[name] = &JavaDependency{
					GroupId:    parts[0],
					ArtifactId: parts[1],
					Version:    version,
					Source:     "pom.xml",
				}
			}
		}
	}

	if buildSystem.GradleInfo != nil {
		metadata.GradleInfo = buildSystem.GradleInfo
		// Convert Gradle dependencies to Java dependencies
		for name, version := range buildSystem.GradleInfo.Dependencies {
			parts := strings.Split(name, ":")
			if len(parts) >= 2 {
				metadata.Dependencies[name] = &JavaDependency{
					GroupId:    parts[0],
					ArtifactId: parts[1],
					Version:    version,
					Source:     "build.gradle",
				}
			}
		}
	}

	return metadata
}

func (d *JavaProjectDetector) collectIssues(runtime *JavaRuntimeAnalysis, buildSystem *BuildSystemAnalysis, structure *StructureValidation) []string {
	var issues []string
	
	issues = append(issues, runtime.Issues...)
	issues = append(issues, buildSystem.Issues...)
	issues = append(issues, structure.Issues...)
	
	return issues
}

// Supporting types for internal analysis

type JavaRuntimeAnalysis struct {
	JavaVersion  string
	JavaHome     string
	Distribution string
	IsJDK        bool
	Issues       []string
}

type BuildSystemAnalysis struct {
	BuildSystem string
	MavenInfo   *types.MavenInfo
	GradleInfo  *types.GradleInfo
	Issues      []string
}

type StructureValidation struct {
	IsValid     bool
	SourceDirs  []string
	TestDirs    []string
	ConfigFiles []string
	Issues      []string
}