package project

import (
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

	t.Logf("Detected languages in repo root: %v", langs)

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
			t.Logf("Detected languages in test-multi-lang: %v", langs2)
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

	t.Log("LSP server availability:")
	for _, lang := range languages {
		available := IsLSPServerAvailable(lang)
		t.Logf("  %s: %t", lang, available)
	}
}

// TestGetAvailableLanguages tests getting only available languages
func TestGetAvailableLanguages(t *testing.T) {
	repoRoot := "../../../"

	available, err := GetAvailableLanguages(repoRoot)
	if err != nil {
		t.Fatalf("Error getting available languages: %v", err)
	}

	t.Logf("Available languages (with LSP servers): %v", available)
}

// TestDetectPrimaryLanguage tests primary language detection
func TestDetectPrimaryLanguage(t *testing.T) {
	repoRoot := "../../../"

	primary, err := DetectPrimaryLanguage(repoRoot)
	if err != nil {
		t.Logf("Primary language detection: %v", err)
	} else {
		t.Logf("Primary language: %s", primary)
	}
}
