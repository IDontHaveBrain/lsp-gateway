package project

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDetectLanguages tests the language detection functionality
func TestDetectLanguages(t *testing.T) {
	// Get the repository root (go up 3 levels from src/internal/project)
	repoRoot := "../../../"

	// Test current directory
	langs, err := DetectLanguages(repoRoot)
	if err != nil {
		t.Fatalf("Error detecting languages: %v", err)
	}

	// Should detect Go (because of go.mod and .go files)
	found := false
	for _, lang := range langs {
		if lang == "go" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected to detect Go language")
	}

	// Test multi-language directory if it exists
	multiLangDir := repoRoot + "test-multi-lang"
	if _, err := os.Stat(multiLangDir); err == nil {
		langs2, err := DetectLanguages(multiLangDir)
		if err != nil {
			t.Errorf("Error detecting languages in test-multi-lang: %v", err)
		} else {
			// Should detect multiple languages
			if len(langs2) == 0 {
				t.Error("Expected to detect at least one language in test-multi-lang")
			}
		}
	}
}

// TestIsLSPServerAvailable tests LSP server availability
func TestIsLSPServerAvailable(t *testing.T) {
	// Test supported languages
	languages := []string{"go", "python", "javascript", "typescript", "java", "kotlin"}

	for _, lang := range languages {
		_ = IsLSPServerAvailable(lang)
	}
}

// TestGetAvailableLanguages tests getting only available languages
func TestGetAvailableLanguages(t *testing.T) {
	repoRoot := "../../../"

	available, err := GetAvailableLanguages(repoRoot)
	if err != nil {
		t.Fatalf("Error getting available languages: %v", err)
	}

	// Verify we got some languages
	if len(available) == 0 {
		t.Error("Expected at least one available language")
	}
}

// TestKotlinDetection tests Kotlin-specific detection scenarios
func TestKotlinDetection(t *testing.T) {
	// Create temporary test directory
	tempDir := t.TempDir()

	// Test 1: Pure Kotlin project with .kt files
	t.Run("KotlinFiles", func(t *testing.T) {
		testDir := tempDir + "/kotlin-pure"
		if err := os.MkdirAll(testDir+"/src/main/kotlin", 0755); err != nil {
			t.Fatal(err)
		}

		// Create Kotlin source files
		files := []string{
			"src/main/kotlin/Main.kt",
			"src/main/kotlin/model/User.kt",
			"src/main/kotlin/controller/UserController.kt",
		}
		for _, file := range files {
			fullPath := testDir + "/" + file
			if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(fullPath, []byte("package com.example\n\nfun main() {\n    println(\"Hello Kotlin\")\n}"), 0644); err != nil {
				t.Fatal(err)
			}
		}

		langs, err := DetectLanguages(testDir)
		if err != nil {
			t.Fatalf("Error detecting languages: %v", err)
		}

		found := false
		for _, lang := range langs {
			if lang == "kotlin" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected to detect Kotlin from .kt files")
		}
	})

	// Test 2: Kotlin script files (.kts)
	t.Run("KotlinScriptFiles", func(t *testing.T) {
		testDir := tempDir + "/kotlin-scripts"
		if err := os.MkdirAll(testDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Create Kotlin script files
		files := []string{
			"build.gradle.kts",
			"script.main.kts",
			"settings.gradle.kts",
		}
		for _, file := range files {
			fullPath := testDir + "/" + file
			if err := os.WriteFile(fullPath, []byte("// Kotlin script content\nprintln(\"Hello from Kotlin script\")"), 0644); err != nil {
				t.Fatal(err)
			}
		}

		langs, err := DetectLanguages(testDir)
		if err != nil {
			t.Fatalf("Error detecting languages: %v", err)
		}

		found := false
		for _, lang := range langs {
			if lang == "kotlin" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected to detect Kotlin from .kts files")
		}
	})

	// Test 3: Mixed Kotlin/Java project
	t.Run("MixedKotlinJava", func(t *testing.T) {
		testDir := tempDir + "/kotlin-java-mixed"
		if err := os.MkdirAll(testDir+"/src/main/kotlin", 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(testDir+"/src/main/java", 0755); err != nil {
			t.Fatal(err)
		}

		// Create mixed Kotlin and Java files
		kotlinFiles := []string{
			"src/main/kotlin/Main.kt",
			"src/main/kotlin/service/KotlinService.kt",
		}
		javaFiles := []string{
			"src/main/java/com/example/JavaService.java",
			"src/main/java/com/example/model/JavaModel.java",
		}

		for _, file := range kotlinFiles {
			fullPath := testDir + "/" + file
			if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(fullPath, []byte("package com.example\n\nclass KotlinClass"), 0644); err != nil {
				t.Fatal(err)
			}
		}

		for _, file := range javaFiles {
			fullPath := testDir + "/" + file
			if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(fullPath, []byte("package com.example;\n\npublic class JavaClass {}"), 0644); err != nil {
				t.Fatal(err)
			}
		}

		// Add Kotlin Gradle DSL
		if err := os.WriteFile(testDir+"/build.gradle.kts", []byte(`
plugins {
    kotlin("jvm") version "1.8.0"
    java
}
`), 0644); err != nil {
			t.Fatal(err)
		}

		langs, err := DetectLanguages(testDir)
		if err != nil {
			t.Fatalf("Error detecting languages: %v", err)
		}

		foundKotlin := false
		foundJava := false
		for _, lang := range langs {
			if lang == "kotlin" {
				foundKotlin = true
			}
			if lang == "java" {
				foundJava = true
			}
		}

		if !foundKotlin {
			t.Error("Expected to detect Kotlin in mixed project")
		}
		if !foundJava {
			t.Error("Expected to detect Java in mixed project")
		}
	})

	// Test 4: Kotlin Gradle DSL project marker
	t.Run("KotlinGradleDSL", func(t *testing.T) {
		testDir := tempDir + "/kotlin-gradle-dsl"
		if err := os.MkdirAll(testDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Create build.gradle.kts with Kotlin-specific content
		buildContent := `
plugins {
    kotlin("jvm") version "1.8.0"
    application
}

repositories {
    mavenCentral()
}

dependencies {
    implementation("org.jetbrains.kotlin:kotlin-stdlib")
    testImplementation("org.jetbrains.kotlin:kotlin-test")
}

application {
    mainClass.set("MainKt")
}
`
		if err := os.WriteFile(testDir+"/build.gradle.kts", []byte(buildContent), 0644); err != nil {
			t.Fatal(err)
		}

		langs, err := DetectLanguages(testDir)
		if err != nil {
			t.Fatalf("Error detecting languages: %v", err)
		}

		foundKotlin := false
		foundJava := false
		for _, lang := range langs {
			if lang == "kotlin" {
				foundKotlin = true
			}
			if lang == "java" {
				foundJava = true
			}
		}

		if !foundKotlin {
			t.Error("Expected to detect Kotlin from build.gradle.kts")
		}
		if !foundJava {
			t.Error("Expected to detect Java from Gradle build file (lower priority)")
		}

		// Verify Kotlin comes before Java in priority
		kotlinIndex := -1
		javaIndex := -1
		for i, lang := range langs {
			if lang == "kotlin" {
				kotlinIndex = i
			}
			if lang == "java" {
				javaIndex = i
			}
		}

		if kotlinIndex != -1 && javaIndex != -1 && kotlinIndex > javaIndex {
			t.Error("Expected Kotlin to have higher priority than Java when build.gradle.kts is present")
		}
	})
}

// TestKotlinFileExtensions tests that Kotlin file extensions are properly recognized
func TestKotlinFileExtensions(t *testing.T) {
	tempDir := t.TempDir()

	// Test various Kotlin file extensions
	testCases := []struct {
		fileName string
		expected bool
	}{
		{"Main.kt", true},
		{"Script.kts", true},
		{"build.gradle.kts", true},
		{"settings.gradle.kts", true},
		{"script.main.kts", true},
		{"Main.java", false}, // Should not detect as Kotlin
		{"main.py", false},   // Should not detect as Kotlin
		{"app.js", false},    // Should not detect as Kotlin
	}

	for _, tc := range testCases {
		t.Run(tc.fileName, func(t *testing.T) {
			// Create a separate subdirectory for each test case to avoid interference
			testSubDir := tempDir + "/test_" + tc.fileName
			if err := os.MkdirAll(testSubDir, 0755); err != nil {
				t.Fatal(err)
			}

			fullPath := testSubDir + "/" + tc.fileName
			if err := os.WriteFile(fullPath, []byte("// Test content"), 0644); err != nil {
				t.Fatal(err)
			}

			langs, err := DetectLanguages(testSubDir)
			if err != nil {
				t.Fatalf("Error detecting languages: %v", err)
			}

			foundKotlin := false
			for _, lang := range langs {
				if lang == "kotlin" {
					foundKotlin = true
					break
				}
			}

			if tc.expected && !foundKotlin {
				t.Errorf("Expected to detect Kotlin for file %s", tc.fileName)
			}
			if !tc.expected && foundKotlin {
				t.Errorf("Did not expect to detect Kotlin for file %s", tc.fileName)
			}
		})
	}
}
