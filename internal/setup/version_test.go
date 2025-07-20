package setup

import (
	"testing"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedMajor int
		expectedMinor int
		expectedPatch int
		expectedPre   string
		expectedBuild string
		expectError   bool
	}{
		{"Go version with prefix", "go1.21.0", 1, 21, 0, "", "", false},
		{"Go version with prefix and patch", "go1.21.1", 1, 21, 1, "", "", false},
		{"Go development version", "devel +abc123", 0, 0, 0, "", "", true},

		{"Python standard", "3.11.2", 3, 11, 2, "", "", false},
		{"Python older", "3.8.10", 3, 8, 10, "", "", false},
		{"Python prerelease", "3.12.0a1", 3, 12, 0, "a1", "", false},
		{"Python beta", "3.11.0b4", 3, 11, 0, "b4", "", false},
		{"Python rc", "3.10.0rc2", 3, 10, 0, "rc2", "", false},

		{"Node.js with v prefix", "v18.16.0", 18, 16, 0, "", "", false},
		{"Node.js newer", "v20.2.0", 20, 2, 0, "", "", false},
		{"Node.js without v", "18.16.0", 18, 16, 0, "", "", false},
		{"Node.js LTS", "v16.20.1", 16, 20, 1, "", "", false},

		{"Java modern", "17.0.1", 17, 0, 1, "", "", false},
		{"Java newer", "21.0.0", 21, 0, 0, "", "", false},
		{"Java legacy", "1.8.0", 1, 8, 0, "", "", false},
		{"Java with underscore", "1.8.0_291", 1, 8, 0, "", "", false},
		{"OpenJDK version", "openjdk-17", 17, 0, 0, "", "", false},
		{"OpenJDK with dots", "openjdk-17.0.1", 17, 0, 1, "", "", false},

		{"Version with prerelease", "1.0.0-alpha", 1, 0, 0, "alpha", "", false},
		{"Version with build", "1.0.0+build.1", 1, 0, 0, "", "build.1", false},
		{"Version with both", "1.0.0-beta.1+build.123", 1, 0, 0, "beta.1", "build.123", false},
		{"Complex prerelease", "2.1.0-rc.1.2", 2, 1, 0, "rc.1.2", "", false},

		{"Major.Minor only", "3.8", 3, 8, 0, "", "", false},
		{"Version prefix", "version1.2.3", 1, 2, 3, "", "", false},
		{"Release prefix", "release2.0.0", 2, 0, 0, "", "", false},

		{"Empty string", "", 0, 0, 0, "", "", true},
		{"Invalid format", "abc", 0, 0, 0, "", "", true},
		{"Only numbers", "123", 0, 0, 0, "", "", true},
		{"Invalid major", "a.1.0", 0, 0, 0, "", "", true},
		{"Invalid minor", "1.a.0", 0, 0, 0, "", "", true},
		{"Invalid patch", "1.1.a", 0, 0, 0, "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version, err := ParseVersion(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for input %q, but got none", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error for input %q: %v", tt.input, err)
				return
			}

			if version.Major != tt.expectedMajor {
				t.Errorf("Expected major %d, got %d", tt.expectedMajor, version.Major)
			}
			if version.Minor != tt.expectedMinor {
				t.Errorf("Expected minor %d, got %d", tt.expectedMinor, version.Minor)
			}
			if version.Patch != tt.expectedPatch {
				t.Errorf("Expected patch %d, got %d", tt.expectedPatch, version.Patch)
			}
			if version.Prerelease != tt.expectedPre {
				t.Errorf("Expected prerelease %q, got %q", tt.expectedPre, version.Prerelease)
			}
			if version.Build != tt.expectedBuild {
				t.Errorf("Expected build %q, got %q", tt.expectedBuild, version.Build)
			}
			if version.Original != tt.input {
				t.Errorf("Expected original %q, got %q", tt.input, version.Original)
			}
		})
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name     string
		v1       string
		v2       string
		expected int
	}{
		{"Equal versions", "1.0.0", "1.0.0", 0},
		{"Greater major", "2.0.0", "1.0.0", 1},
		{"Lesser major", "1.0.0", "2.0.0", -1},
		{"Greater minor", "1.1.0", "1.0.0", 1},
		{"Lesser minor", "1.0.0", "1.1.0", -1},
		{"Greater patch", "1.0.1", "1.0.0", 1},
		{"Lesser patch", "1.0.0", "1.0.1", -1},

		{"Stable vs prerelease", "1.0.0", "1.0.0-alpha", 1},
		{"Prerelease vs stable", "1.0.0-alpha", "1.0.0", -1},
		{"Alpha vs beta", "1.0.0-alpha", "1.0.0-beta", -1},
		{"Beta vs alpha", "1.0.0-beta", "1.0.0-alpha", 1},
		{"Alpha versions", "1.0.0-alpha.1", "1.0.0-alpha.2", -1},

		{"Build metadata ignored", "1.0.0+build1", "1.0.0+build2", 0},
		{"Prerelease with build", "1.0.0-alpha+build1", "1.0.0-alpha+build2", 0},

		{"Go versions", "go1.21.0", "go1.20.5", 1},
		{"Python versions", "3.11.2", "3.10.8", 1},
		{"Node versions", "v18.16.0", "v16.20.1", 1},
		{"Java versions", "17.0.1", "1.8.0", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v1, err := ParseVersion(tt.v1)
			if err != nil {
				t.Fatalf("Failed to parse v1 %q: %v", tt.v1, err)
			}

			v2, err := ParseVersion(tt.v2)
			if err != nil {
				t.Fatalf("Failed to parse v2 %q: %v", tt.v2, err)
			}

			result := CompareVersions(v1, v2)
			if result != tt.expected {
				t.Errorf("Expected %d, got %d for %q vs %q", tt.expected, result, tt.v1, tt.v2)
			}
		})
	}
}

func TestVersionChecker(t *testing.T) {
	checker := NewVersionChecker()

	minVersions := map[string]string{
		"go":     "1.19.0",
		"python": "3.8.0",
		"nodejs": "18.0.0",
		"java":   "17.0.0",
	}

	for runtime, expectedMin := range minVersions {
		minVer, exists := checker.GetMinVersion(runtime)
		if !exists {
			t.Errorf("Expected minimum version for %s to exist", runtime)
		}
		if minVer != expectedMin {
			t.Errorf("Expected minimum version %s for %s, got %s", expectedMin, runtime, minVer)
		}
	}

	compatibilityTests := []struct {
		runtime            string
		version            string
		shouldBeCompatible bool
		expectedScore      int
	}{
		{"go", "go1.21.0", true, 100}, // Meets recommended
		{"go", "go1.20.0", true, 75},  // Above minimum, below recommended
		{"go", "go1.18.0", false, 0},  // Below minimum

		{"python", "3.11.0", true, 100},   // Above recommended
		{"python", "3.9.0", true, 75},     // Between minimum and recommended
		{"python", "3.7.0", false, 0},     // Below minimum
		{"python", "3.12.0a1", true, 100}, // Prerelease above recommended

		{"nodejs", "v20.1.0", true, 100}, // Meets recommended
		{"nodejs", "v19.0.0", true, 75},  // Between minimum and recommended
		{"nodejs", "v17.0.0", false, 0},  // Below minimum
		{"nodejs", "18.16.0", true, 75},  // Without 'v' prefix

		{"java", "21.0.0", true, 100},    // Meets recommended
		{"java", "17.0.1", true, 75},     // Above minimum, below recommended
		{"java", "1.8.0", false, 0},      // Below minimum
		{"java", "openjdk-17", true, 75}, // With prefix

		{"unknown", "1.0.0", true, 100}, // No requirements defined
	}

	for _, tt := range compatibilityTests {
		t.Run(tt.runtime+"_"+tt.version, func(t *testing.T) {
			result := checker.CheckCompatibility(tt.runtime, tt.version)

			if result.IsCompatible != tt.shouldBeCompatible {
				t.Errorf("Expected compatible=%v, got %v for %s %s",
					tt.shouldBeCompatible, result.IsCompatible, tt.runtime, tt.version)
			}

			if result.IsCompatible && result.Score != tt.expectedScore {
				t.Errorf("Expected score=%d, got %d for %s %s",
					tt.expectedScore, result.Score, tt.runtime, tt.version)
			}

			simpleResult := checker.IsCompatible(tt.runtime, tt.version)
			if simpleResult != result.IsCompatible {
				t.Errorf("IsCompatible mismatch: simple=%v, detailed=%v",
					simpleResult, result.IsCompatible)
			}
		})
	}
}

func TestVersionMethods(t *testing.T) {
	v1, _ := ParseVersion("1.2.3")
	v2, _ := ParseVersion("1.2.4")
	v3, _ := ParseVersion("1.2.3")
	v4, _ := ParseVersion("1.2.3-alpha")
	v5, _ := ParseVersion("1.2.3+build")

	if !v2.IsGreaterThan(v1) {
		t.Error("Expected v2 > v1")
	}
	if !v1.IsLessThan(v2) {
		t.Error("Expected v1 < v2")
	}
	if !v1.IsEqualTo(v3) {
		t.Error("Expected v1 == v3")
	}
	if !v2.IsGreaterOrEqualTo(v1) {
		t.Error("Expected v2 >= v1")
	}
	if !v1.IsLessOrEqualTo(v2) {
		t.Error("Expected v1 <= v2")
	}

	if !v4.IsPrerelease() {
		t.Error("Expected v4 to be prerelease")
	}
	if v1.IsPrerelease() {
		t.Error("Expected v1 not to be prerelease")
	}
	if !v1.IsStable() {
		t.Error("Expected v1 to be stable")
	}
	if v4.IsStable() {
		t.Error("Expected v4 not to be stable")
	}

	if !v5.HasBuildMetadata() {
		t.Error("Expected v5 to have build metadata")
	}
	if v1.HasBuildMetadata() {
		t.Error("Expected v1 not to have build metadata")
	}

	if v1.String() != "1.2.3" {
		t.Errorf("Expected String() = '1.2.3', got '%s'", v1.String())
	}
	if v4.String() != "1.2.3-alpha" {
		t.Errorf("Expected String() = '1.2.3-alpha', got '%s'", v4.String())
	}
	if v5.String() != "1.2.3+build" {
		t.Errorf("Expected String() = '1.2.3+build', got '%s'", v5.String())
	}
	if v1.ShortString() != "1.2.3" {
		t.Errorf("Expected ShortString() = '1.2.3', got '%s'", v1.ShortString())
	}
}

func TestGetUpgradePath(t *testing.T) {
	tests := []struct {
		name        string
		from        string
		to          string
		contains    []string
		expectError bool
	}{
		{
			name:     "Same version",
			from:     "1.0.0",
			to:       "1.0.0",
			contains: []string{"Already at target"},
		},
		{
			name:     "Major upgrade",
			from:     "1.0.0",
			to:       "2.0.0",
			contains: []string{"Major upgrade", "breaking changes"},
		},
		{
			name:     "Minor upgrade",
			from:     "1.0.0",
			to:       "1.1.0",
			contains: []string{"Minor upgrade", "backward compatible"},
		},
		{
			name:     "Patch upgrade",
			from:     "1.0.0",
			to:       "1.0.1",
			contains: []string{"Patch upgrade", "bug fixes"},
		},
		{
			name:     "Downgrade",
			from:     "2.0.0",
			to:       "1.0.0",
			contains: []string{"Downgrade"},
		},
		{
			name:        "Invalid from version",
			from:        "invalid",
			to:          "1.0.0",
			expectError: true,
		},
		{
			name:        "Invalid to version",
			from:        "1.0.0",
			to:          "invalid",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := GetUpgradePath(tt.from, tt.to)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error, but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			for _, expected := range tt.contains {
				if !containsSubstring(path, expected) {
					t.Errorf("Expected path to contain %q, but got %q", expected, path)
				}
			}
		})
	}
}

func TestCompatibilityResult(t *testing.T) {
	checker := NewVersionChecker()

	result := checker.CheckCompatibility("go", "go1.18.0")
	if result.IsCompatible {
		t.Error("Expected Go 1.18.0 to be incompatible")
	}
	if result.Score != 0 {
		t.Errorf("Expected score 0, got %d", result.Score)
	}
	if result.MinimumVersion != "1.19.0" {
		t.Errorf("Expected minimum version 1.19.0, got %s", result.MinimumVersion)
	}
	if result.UpgradePath == "" {
		t.Error("Expected upgrade path to be provided")
	}
	if len(result.Notes) == 0 {
		t.Error("Expected notes to be provided")
	}

	result = checker.CheckCompatibility("python", "3.11.0")
	if !result.IsCompatible {
		t.Error("Expected Python 3.11.0 to be compatible")
	}
	if result.Score != 100 {
		t.Errorf("Expected score 100, got %d", result.Score)
	}
	if result.RecommendedVersion != "3.10.0" {
		t.Errorf("Expected recommended version 3.10.0, got %s", result.RecommendedVersion)
	}
}

func containsSubstring(str, substr string) bool {
	return len(str) >= len(substr) &&
		(str == substr ||
			len(substr) == 0 ||
			(len(str) > 0 && (str[:len(substr)] == substr ||
				str[len(str)-len(substr):] == substr ||
				findSubstring(str, substr))))
}

func findSubstring(str, substr string) bool {
	for i := 0; i <= len(str)-len(substr); i++ {
		if str[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
