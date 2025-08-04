package project

import (
	"fmt"
	"os"
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

	fmt.Printf("Detected languages in repo root: %v\n", langs)

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
			fmt.Printf("Detected languages in test-multi-lang: %v\n", langs2)
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
	languages := []string{"go", "python", "javascript", "typescript", "java"}

	fmt.Println("LSP server availability:")
	for _, lang := range languages {
		available := IsLSPServerAvailable(lang)
		fmt.Printf("  %s: %t\n", lang, available)
	}
}

// TestGetAvailableLanguages tests getting only available languages
func TestGetAvailableLanguages(t *testing.T) {
	repoRoot := "../../../"

	available, err := GetAvailableLanguages(repoRoot)
	if err != nil {
		t.Fatalf("Error getting available languages: %v", err)
	}

	fmt.Printf("Available languages (with LSP servers): %v\n", available)
}

// TestDetectPrimaryLanguage tests primary language detection
func TestDetectPrimaryLanguage(t *testing.T) {
	repoRoot := "../../../"

	primary, err := DetectPrimaryLanguage(repoRoot)
	if err != nil {
		fmt.Printf("Primary language detection: %v\n", err)
	} else {
		fmt.Printf("Primary language: %s\n", primary)
	}
}
