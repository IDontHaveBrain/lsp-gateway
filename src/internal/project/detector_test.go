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
	languages := []string{"go", "python", "javascript", "typescript", "java"}

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
