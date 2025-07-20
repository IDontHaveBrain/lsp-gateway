package setup

import (
	"context"
	"testing"
	"time"
)

func TestGoDetectorIntegration(t *testing.T) {
	detector := NewRuntimeDetector()
	detector.SetTimeout(10 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	info, err := detector.DetectGo(ctx)
	if err != nil {
		t.Logf("Go detection failed (expected if Go not installed): %v", err)
		return
	}

	if info == nil {
		t.Fatal("DetectGo returned nil info")
	}

	if info.Name != "go" {
		t.Errorf("Expected runtime name 'go', got '%s'", info.Name)
	}

	t.Logf("Go Detection Results:")
	t.Logf("  Installed: %v", info.Installed)
	t.Logf("  Version: %s", info.Version)
	t.Logf("  Compatible: %v", info.Compatible)
	t.Logf("  Path: %s", info.Path)
	t.Logf("  Issues: %v", info.Issues)
}

func TestGoDetectorConfiguration(t *testing.T) {
	detector := NewGoDetector()

	originalTimeout := detector.timeout
	newTimeout := 20 * time.Second
	detector.SetTimeout(newTimeout)

	if detector.timeout != newTimeout {
		t.Errorf("Expected timeout %v, got %v", newTimeout, detector.timeout)
	}

	originalMinVersion := detector.GetMinimumVersion()
	newMinVersion := "1.21.0"
	detector.SetMinimumVersion(newMinVersion)

	currentMinVersion := detector.GetMinimumVersion()
	if currentMinVersion != newMinVersion {
		t.Errorf("Expected minimum version %s, got %s", newMinVersion, currentMinVersion)
	}

	recommendations := detector.GetGoRecommendations()
	if len(recommendations) == 0 {
		t.Error("Expected at least one recommendation")
	}

	t.Logf("Configuration test passed")
	t.Logf("  Original timeout: %v", originalTimeout)
	t.Logf("  New timeout: %v", detector.timeout)
	t.Logf("  Original min version: %s", originalMinVersion)
	t.Logf("  New min version: %s", currentMinVersion)
	t.Logf("  Recommendations: %d", len(recommendations))
}
