package project

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lsp-gateway/internal/platform"
	"lsp-gateway/internal/project/types"
)

// mockJavaCommandExecutor for testing Java version detection
type mockJavaCommandExecutor struct {
	shouldFail    bool
	exitCode      int
	stdout        string
	stderr        string
	failError     error
	commandCalled string
	argsCalled    []string
}

func (m *mockJavaCommandExecutor) Execute(cmd string, args []string, timeout time.Duration) (*platform.Result, error) {
	m.commandCalled = cmd
	m.argsCalled = args

	if m.shouldFail {
		return &platform.Result{
			ExitCode: m.exitCode,
			Stdout:   m.stdout,
			Stderr:   m.stderr,
		}, m.failError
	}

	return &platform.Result{
		ExitCode: m.exitCode,
		Stdout:   m.stdout,
		Stderr:   m.stderr,
	}, nil
}

func (m *mockJavaCommandExecutor) ExecuteWithEnv(cmd string, args []string, env map[string]string, timeout time.Duration) (*platform.Result, error) {
	return m.Execute(cmd, args, timeout)
}

func (m *mockJavaCommandExecutor) GetShell() string {
	return "/bin/sh"
}

func (m *mockJavaCommandExecutor) GetShellArgs(command string) []string {
	return []string{"-c", command}
}

func (m *mockJavaCommandExecutor) IsCommandAvailable(command string) bool {
	return !m.shouldFail
}

// Test data creation helpers for Java projects
func createMavenProject(t *testing.T) string {
	tempDir := t.TempDir()

	// Create Maven pom.xml with Spring Boot
	pomContent := `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0"
         xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
         xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 
         http://maven.apache.org/xsd/maven-4.0.0.xsd">
    <modelVersion>4.0.0</modelVersion>
    
    <parent>
        <groupId>org.springframework.boot</groupId>
        <artifactId>spring-boot-starter-parent</artifactId>
        <version>3.1.0</version>
        <relativePath/>
    </parent>
    
    <groupId>com.example</groupId>
    <artifactId>demo-app</artifactId>
    <version>1.0.0</version>
    <name>Demo Application</name>
    <description>Demo Spring Boot application</description>
    <packaging>jar</packaging>
    
    <properties>
        <java.version>17</java.version>
        <maven.compiler.source>17</maven.compiler.source>
        <maven.compiler.target>17</maven.compiler.target>
    </properties>
    
    <dependencies>
        <dependency>
            <groupId>org.springframework.boot</groupId>
            <artifactId>spring-boot-starter-web</artifactId>
        </dependency>
        <dependency>
            <groupId>org.springframework.boot</groupId>
            <artifactId>spring-boot-starter-data-jpa</artifactId>
        </dependency>
        <dependency>
            <groupId>mysql</groupId>
            <artifactId>mysql-connector-java</artifactId>
            <scope>runtime</scope>
        </dependency>
        <dependency>
            <groupId>org.springframework.boot</groupId>
            <artifactId>spring-boot-starter-test</artifactId>
            <scope>test</scope>
        </dependency>
        <dependency>
            <groupId>org.junit.jupiter</groupId>
            <artifactId>junit-jupiter</artifactId>
            <scope>test</scope>
        </dependency>
    </dependencies>
</project>`

	err := os.WriteFile(filepath.Join(tempDir, types.MARKER_POM_XML), []byte(pomContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create pom.xml: %v", err)
	}

	// Create Maven directory structure
	dirs := []string{
		"src/main/java/com/example/demo",
		"src/main/resources",
		"src/test/java/com/example/demo",
		"src/test/resources",
		"target",
	}
	for _, dir := range dirs {
		err := os.MkdirAll(filepath.Join(tempDir, dir), 0755)
		if err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}

	// Create Java source files
	mainClass := `package com.example.demo;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;

@SpringBootApplication
public class DemoApplication {
    public static void main(String[] args) {
        SpringApplication.run(DemoApplication.class, args);
    }
}`

	err = os.WriteFile(filepath.Join(tempDir, "src/main/java/com/example/demo/DemoApplication.java"), []byte(mainClass), 0644)
	if err != nil {
		t.Fatalf("Failed to create main Java file: %v", err)
	}

	// Create test file
	testClass := `package com.example.demo;

import org.junit.jupiter.api.Test;
import org.springframework.boot.test.context.SpringBootTest;

@SpringBootTest
class DemoApplicationTests {
    @Test
    void contextLoads() {
    }
}`

	err = os.WriteFile(filepath.Join(tempDir, "src/test/java/com/example/demo/DemoApplicationTests.java"), []byte(testClass), 0644)
	if err != nil {
		t.Fatalf("Failed to create test Java file: %v", err)
	}

	// Create application properties
	appProps := `server.port=8080
spring.datasource.url=jdbc:mysql://localhost:3306/demo
spring.jpa.hibernate.ddl-auto=update`

	err = os.WriteFile(filepath.Join(tempDir, "src/main/resources/application.properties"), []byte(appProps), 0644)
	if err != nil {
		t.Fatalf("Failed to create application.properties: %v", err)
	}

	return tempDir
}

func createGradleProject(t *testing.T) string {
	tempDir := t.TempDir()

	// Create build.gradle with dependencies
	buildGradleContent := `plugins {
    id 'java'
    id 'org.springframework.boot' version '3.1.0'
    id 'io.spring.dependency-management' version '1.1.0'
}

group = 'com.example'
version = '1.0.0'
sourceCompatibility = '17'
targetCompatibility = '17'

repositories {
    mavenCentral()
}

dependencies {
    implementation 'org.springframework.boot:spring-boot-starter-web'
    implementation 'org.springframework.boot:spring-boot-starter-data-jpa'
    implementation 'com.google.guava:guava:32.1.0-jre'
    runtimeOnly 'mysql:mysql-connector-java'
    testImplementation 'org.springframework.boot:spring-boot-starter-test'
    testImplementation 'org.junit.jupiter:junit-jupiter'
    testImplementation 'org.mockito:mockito-core'
}

test {
    useJUnitPlatform()
}`

	err := os.WriteFile(filepath.Join(tempDir, types.MARKER_BUILD_GRADLE), []byte(buildGradleContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create build.gradle: %v", err)
	}

	// Create gradlew script
	err = os.WriteFile(filepath.Join(tempDir, "gradlew"), []byte("#!/bin/sh\necho 'Gradle wrapper'"), 0755)
	if err != nil {
		t.Fatalf("Failed to create gradlew: %v", err)
	}

	// Create settings.gradle
	settingsContent := `rootProject.name = 'demo-gradle'`
	err = os.WriteFile(filepath.Join(tempDir, "settings.gradle"), []byte(settingsContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create settings.gradle: %v", err)
	}

	// Create Gradle directory structure
	dirs := []string{
		"src/main/java/com/example",
		"src/main/resources",
		"src/test/java/com/example",
		"build",
	}
	for _, dir := range dirs {
		err := os.MkdirAll(filepath.Join(tempDir, dir), 0755)
		if err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}

	// Create Java files
	mainClass := `package com.example;

public class Application {
    public static void main(String[] args) {
        System.out.println("Hello World");
    }
}`

	err = os.WriteFile(filepath.Join(tempDir, "src/main/java/com/example/Application.java"), []byte(mainClass), 0644)
	if err != nil {
		t.Fatalf("Failed to create main Java file: %v", err)
	}

	testClass := `package com.example;

import org.junit.jupiter.api.Test;
import static org.junit.jupiter.api.Assertions.*;

class ApplicationTest {
    @Test
    void testMain() {
        assertDoesNotThrow(() -> Application.main(new String[]{}));
    }
}`

	err = os.WriteFile(filepath.Join(tempDir, "src/test/java/com/example/ApplicationTest.java"), []byte(testClass), 0644)
	if err != nil {
		t.Fatalf("Failed to create test Java file: %v", err)
	}

	return tempDir
}

func createJavaFilesOnlyProject(t *testing.T) string {
	tempDir := t.TempDir()

	// Create simple Java files without build system
	mainClass := `public class Main {
    public static void main(String[] args) {
        System.out.println("Hello World");
    }
}`

	err := os.WriteFile(filepath.Join(tempDir, "Main.java"), []byte(mainClass), 0644)
	if err != nil {
		t.Fatalf("Failed to create Main.java: %v", err)
	}

	helperClass := `public class Helper {
    public String help() {
        return "help";
    }
}`

	err = os.WriteFile(filepath.Join(tempDir, "Helper.java"), []byte(helperClass), 0644)
	if err != nil {
		t.Fatalf("Failed to create Helper.java: %v", err)
	}

	return tempDir
}

func createJavaEmptyProject(t *testing.T) string {
	tempDir := t.TempDir()
	// Create a directory with no Java-related files
	err := os.WriteFile(filepath.Join(tempDir, "README.md"), []byte("# Empty Project"), 0644)
	if err != nil {
		t.Fatalf("Failed to create README.md: %v", err)
	}
	return tempDir
}

func createMalformedPomProject(t *testing.T) string {
	tempDir := t.TempDir()

	// Create invalid XML pom.xml
	malformedPom := `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
    <modelVersion>4.0.0</modelVersion>
    <groupId>com.example</groupId>
    <artifactId>malformed
    <!-- Missing closing tag -->`

	err := os.WriteFile(filepath.Join(tempDir, types.MARKER_POM_XML), []byte(malformedPom), 0644)
	if err != nil {
		t.Fatalf("Failed to create malformed pom.xml: %v", err)
	}

	return tempDir
}

// Test constructor
func TestNewJavaLanguageDetector(t *testing.T) {
	detector := NewJavaLanguageDetector()

	if detector == nil {
		t.Fatal("Expected detector to be non-nil")
	}

	javaDetector, ok := detector.(*JavaLanguageDetector)
	if !ok {
		t.Fatal("Expected detector to be of type *JavaLanguageDetector")
	}

	if javaDetector.logger == nil {
		t.Error("Expected logger to be initialized")
	}

	if javaDetector.executor == nil {
		t.Error("Expected executor to be initialized")
	}
}

// Test Maven project detection
func TestDetectLanguage_MavenProject(t *testing.T) {
	testDir := createMavenProject(t)
	defer os.RemoveAll(testDir)

	detector := NewJavaLanguageDetector()
	ctx := context.Background()

	result, err := detector.DetectLanguage(ctx, testDir)
	if err != nil {
		t.Fatalf("DetectLanguage failed: %v", err)
	}

	// Verify basic properties
	if result.Language != types.PROJECT_TYPE_JAVA {
		t.Errorf("Expected language %s, got %s", types.PROJECT_TYPE_JAVA, result.Language)
	}

	if result.Confidence < 0.95 {
		t.Errorf("Expected confidence >= 0.95 for Maven project, got %f", result.Confidence)
	}

	// Verify marker files
	if !contains(result.MarkerFiles, types.MARKER_POM_XML) {
		t.Errorf("Expected marker files to contain %s", types.MARKER_POM_XML)
	}

	// Verify required servers
	expectedServers := []string{types.SERVER_JDTLS}
	if !javaSlicesEqual(result.RequiredServers, expectedServers) {
		t.Errorf("Expected servers %v, got %v", expectedServers, result.RequiredServers)
	}

	// Verify Maven-specific metadata
	if buildSystem, ok := result.Metadata["build_system"]; !ok || buildSystem != "maven" {
		t.Errorf("Expected build_system to be 'maven', got %v", buildSystem)
	}

	if groupId, ok := result.Metadata["group_id"]; !ok || groupId != "com.example" {
		t.Errorf("Expected group_id to be 'com.example', got %v", groupId)
	}

	if artifactId, ok := result.Metadata["artifact_id"]; !ok || artifactId != "demo-app" {
		t.Errorf("Expected artifact_id to be 'demo-app', got %v", artifactId)
	}

	// Verify Java version from pom.xml
	if result.Version != "17" {
		t.Errorf("Expected version to be '17', got %s", result.Version)
	}

	// Verify Spring Boot detection
	if parentArtifact, ok := result.Metadata["parent_artifact_id"]; !ok || parentArtifact != "spring-boot-starter-parent" {
		t.Errorf("Expected parent_artifact_id to be 'spring-boot-starter-parent', got %v", parentArtifact)
	}

	// Verify dependencies were parsed
	if len(result.Dependencies) == 0 {
		t.Error("Expected dependencies to be parsed from pom.xml")
	}

	if len(result.DevDependencies) == 0 {
		t.Error("Expected test dependencies to be parsed from pom.xml")
	}

	// Verify project structure analysis
	if len(result.SourceDirs) == 0 {
		t.Error("Expected source directories to be detected")
	}

	if len(result.TestDirs) == 0 {
		t.Error("Expected test directories to be detected")
	}

	// Verify Java file count
	if javaFileCount, ok := result.Metadata["java_file_count"]; !ok || javaFileCount.(int) < 1 {
		t.Error("Expected java_file_count metadata with positive value")
	}
}

// Test Gradle project detection
func TestDetectLanguage_GradleProject(t *testing.T) {
	testDir := createGradleProject(t)
	defer os.RemoveAll(testDir)

	detector := NewJavaLanguageDetector()
	ctx := context.Background()

	result, err := detector.DetectLanguage(ctx, testDir)
	if err != nil {
		t.Fatalf("DetectLanguage failed: %v", err)
	}

	// Verify basic properties
	if result.Language != types.PROJECT_TYPE_JAVA {
		t.Errorf("Expected language %s, got %s", types.PROJECT_TYPE_JAVA, result.Language)
	}

	if result.Confidence < 0.9 {
		t.Errorf("Expected confidence 0.9 for Gradle project, got %f", result.Confidence)
	}

	// Verify marker files include both build.gradle and gradlew
	if !contains(result.MarkerFiles, types.MARKER_BUILD_GRADLE) {
		t.Errorf("Expected marker files to contain %s", types.MARKER_BUILD_GRADLE)
	}

	if !contains(result.MarkerFiles, "gradlew") {
		t.Errorf("Expected marker files to contain gradlew")
	}

	// Verify Gradle-specific metadata
	if buildSystem, ok := result.Metadata["build_system"]; !ok || buildSystem != types.BUILD_SYSTEM_GRADLE {
		t.Errorf("Expected build_system to be '%s', got %v", types.BUILD_SYSTEM_GRADLE, buildSystem)
	}

	// Verify Java version from build.gradle
	if result.Version != "17" {
		t.Errorf("Expected version to be '17', got %s", result.Version)
	}

	// Verify dependencies were parsed
	if len(result.Dependencies) == 0 {
		t.Error("Expected dependencies to be parsed from build.gradle")
	}

	if len(result.DevDependencies) == 0 {
		t.Error("Expected test dependencies to be parsed from build.gradle")
	}

	// Check for specific dependencies
	foundSpringWeb := false
	foundGuava := false
	for dep := range result.Dependencies {
		if strings.Contains(dep, "spring-boot-starter-web") {
			foundSpringWeb = true
		}
		if strings.Contains(dep, "guava") {
			foundGuava = true
		}
	}

	if !foundSpringWeb {
		t.Error("Expected to find spring-boot-starter-web dependency")
	}

	if !foundGuava {
		t.Error("Expected to find guava dependency")
	}

	// Check for test dependencies
	foundJUnit := false
	foundMockito := false
	for dep := range result.DevDependencies {
		if strings.Contains(dep, "junit-jupiter") {
			foundJUnit = true
		}
		if strings.Contains(dep, "mockito") {
			foundMockito = true
		}
	}

	if !foundJUnit {
		t.Error("Expected to find junit-jupiter test dependency")
	}

	if !foundMockito {
		t.Error("Expected to find mockito test dependency")
	}
}

// Test Java files only project detection (no build system)
func TestDetectLanguage_JavaFilesOnly(t *testing.T) {
	testDir := createJavaFilesOnlyProject(t)
	defer os.RemoveAll(testDir)

	detector := NewJavaLanguageDetector()
	ctx := context.Background()

	result, err := detector.DetectLanguage(ctx, testDir)
	if err != nil {
		t.Fatalf("DetectLanguage failed: %v", err)
	}

	// Verify basic properties
	if result.Language != types.PROJECT_TYPE_JAVA {
		t.Errorf("Expected language %s, got %s", types.PROJECT_TYPE_JAVA, result.Language)
	}

	// Should have lower confidence without build files
	if result.Confidence < 0.8 {
		t.Errorf("Expected confidence 0.8 for Java files only, got %f", result.Confidence)
	}

	// Should not have marker files for build systems
	if len(result.MarkerFiles) > 0 {
		t.Errorf("Expected no marker files for Java files only project, got %v", result.MarkerFiles)
	}

	// Should still detect Java files
	if javaFileCount, ok := result.Metadata["java_file_count"]; !ok || javaFileCount.(int) != 2 {
		t.Errorf("Expected java_file_count to be 2, got %v", javaFileCount)
	}
}

// Test no Java files
func TestDetectLanguage_NoJavaFiles(t *testing.T) {
	testDir := createJavaEmptyProject(t)
	defer os.RemoveAll(testDir)

	detector := NewJavaLanguageDetector()
	ctx := context.Background()

	result, err := detector.DetectLanguage(ctx, testDir)
	if err != nil {
		t.Fatalf("DetectLanguage failed: %v", err)
	}

	// Should have very low confidence
	if result.Confidence != 0.1 {
		t.Errorf("Expected confidence 0.1 for no Java indicators, got %f", result.Confidence)
	}

	// Should not have any Java files detected
	if javaFileCount, ok := result.Metadata["java_file_count"]; ok && javaFileCount.(int) > 0 {
		t.Errorf("Expected no Java files for empty project, got %v", javaFileCount)
	}
}

// Test malformed pom.xml parsing
func TestParsePomXML_Malformed(t *testing.T) {
	testDir := createMalformedPomProject(t)
	defer os.RemoveAll(testDir)

	detector := NewJavaLanguageDetector()
	ctx := context.Background()

	result, err := detector.DetectLanguage(ctx, testDir)
	if err != nil {
		t.Fatalf("DetectLanguage failed: %v", err)
	}

	// Should still detect as Java project but with lower confidence
	if result.Language != types.PROJECT_TYPE_JAVA {
		t.Errorf("Expected language %s, got %s", types.PROJECT_TYPE_JAVA, result.Language)
	}

	// Confidence should be reduced due to parsing failure
	if result.Confidence < 0.8 {
		t.Errorf("Expected confidence 0.8 for malformed pom.xml, got %f", result.Confidence)
	}

	// Should still have pom.xml as marker file
	if !contains(result.MarkerFiles, types.MARKER_POM_XML) {
		t.Errorf("Expected marker files to contain %s", types.MARKER_POM_XML)
	}
}

// Test framework detection
func TestDetectJavaFrameworks(t *testing.T) {
	testDir := createMavenProject(t)
	defer os.RemoveAll(testDir)

	detector := NewJavaLanguageDetector()
	ctx := context.Background()

	result, err := detector.DetectLanguage(ctx, testDir)
	if err != nil {
		t.Fatalf("DetectLanguage failed: %v", err)
	}

	// Verify framework detection
	frameworks, ok := result.Metadata["frameworks"]
	if !ok {
		t.Fatal("Expected frameworks metadata to be present")
	}

	frameworkList, ok := frameworks.([]string)
	if !ok {
		t.Fatal("Expected frameworks to be string slice")
	}

	expectedFrameworks := []string{"Spring Boot", "JUnit"}
	for _, expected := range expectedFrameworks {
		if !contains(frameworkList, expected) {
			t.Errorf("Expected framework %s to be detected", expected)
		}
	}

	// Verify project type detection
	if projectType, ok := result.Metadata["project_type"]; !ok || projectType != "Spring Boot Application" {
		t.Errorf("Expected project_type to be 'Spring Boot Application', got %v", projectType)
	}
}

// Test ValidateStructure
func TestJavaValidateStructure(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func(t *testing.T) string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid Maven Project",
			setupFunc:   createMavenProject,
			expectError: false,
		},
		{
			name:        "Valid Gradle Project",
			setupFunc:   createGradleProject,
			expectError: false,
		},
		{
			name:        "No Build Files",
			setupFunc:   createJavaEmptyProject,
			expectError: true,
			errorMsg:    "no Java build file found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDir := tt.setupFunc(t)
			defer os.RemoveAll(testDir)

			detector := NewJavaLanguageDetector()
			ctx := context.Background()

			err := detector.ValidateStructure(ctx, testDir)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

// Test GetLanguageInfo
func TestJavaGetLanguageInfo(t *testing.T) {
	detector := NewJavaLanguageDetector()

	// Test valid language
	info, err := detector.GetLanguageInfo(types.PROJECT_TYPE_JAVA)
	if err != nil {
		t.Fatalf("GetLanguageInfo failed: %v", err)
	}

	if info.Name != types.PROJECT_TYPE_JAVA {
		t.Errorf("Expected name %s, got %s", types.PROJECT_TYPE_JAVA, info.Name)
	}

	if info.DisplayName != "Java" {
		t.Errorf("Expected display name 'Java', got %s", info.DisplayName)
	}

	expectedServers := []string{types.SERVER_JDTLS}
	if !javaSlicesEqual(info.LSPServers, expectedServers) {
		t.Errorf("Expected LSP servers %v, got %v", expectedServers, info.LSPServers)
	}

	// Test invalid language
	_, err = detector.GetLanguageInfo("invalid")
	if err == nil {
		t.Error("Expected error for invalid language")
	}
}

// Test interface methods
func TestJavaInterfaceMethods(t *testing.T) {
	detector := NewJavaLanguageDetector()

	// Test GetMarkerFiles
	markerFiles := detector.GetMarkerFiles()
	expectedMarkers := []string{types.MARKER_POM_XML, types.MARKER_BUILD_GRADLE}
	if !javaSlicesEqual(markerFiles, expectedMarkers) {
		t.Errorf("Expected marker files %v, got %v", expectedMarkers, markerFiles)
	}

	// Test GetRequiredServers
	requiredServers := detector.GetRequiredServers()
	expectedServers := []string{types.SERVER_JDTLS}
	if !javaSlicesEqual(requiredServers, expectedServers) {
		t.Errorf("Expected required servers %v, got %v", expectedServers, requiredServers)
	}

	// Test GetPriority
	priority := detector.GetPriority()
	if priority != types.PRIORITY_JAVA {
		t.Errorf("Expected priority %d, got %d", types.PRIORITY_JAVA, priority)
	}
}

// Test Java version detection with mock executor
func TestDetectJavaVersion(t *testing.T) {
	testDir := createMavenProject(t)
	defer os.RemoveAll(testDir)

	detector := NewJavaLanguageDetector().(*JavaLanguageDetector)

	// Test successful version detection
	mockExec := &mockJavaCommandExecutor{
		exitCode: 0,
		stderr:   `java version "17.0.2" 2022-01-18 LTS`,
	}
	detector.executor = mockExec

	ctx := context.Background()
	result, err := detector.DetectLanguage(ctx, testDir)
	if err != nil {
		t.Fatalf("DetectLanguage failed: %v", err)
	}

	// Verify Java version was detected from command
	if installedVersion, ok := result.Metadata["installed_java_version"]; !ok || installedVersion != "17.0.2" {
		t.Errorf("Expected installed_java_version to be '17.0.2', got %v", installedVersion)
	}

	// Version from pom.xml should take precedence
	if result.Version != "17" {
		t.Errorf("Expected version from pom.xml to be '17', got %s", result.Version)
	}

	// Test version detection failure
	mockExec.shouldFail = true
	mockExec.failError = &platform.PlatformError{}

	result, err = detector.DetectLanguage(ctx, testDir)
	if err != nil {
		t.Fatalf("DetectLanguage failed: %v", err)
	}

	// Should still work without version detection
	if result.Language != types.PROJECT_TYPE_JAVA {
		t.Errorf("Expected language %s even with version detection failure", types.PROJECT_TYPE_JAVA)
	}
}

// Helper functions
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func javaSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}