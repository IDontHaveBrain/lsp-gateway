package setup

import (
	"strings"
	"testing"
)

func TestNodeJSDetector_NewNodeJSDetector(t *testing.T) {
	logger := NewSetupLogger(&SetupLoggerConfig{
		Level:     LogLevelInfo,
		Component: "nodejs-detector-test",
	})

	detector := NewNodeJSDetector(logger)
	if detector == nil {
		t.Fatal("Expected NewNodeJSDetector to return a detector instance")
	}

	if detector.logger == nil {
		t.Error("Expected detector to have a logger")
	}

	if detector.executor == nil {
		t.Error("Expected detector to have an executor")
	}

	if detector.versionChecker == nil {
		t.Error("Expected detector to have a version checker")
	}
}

func TestNodeJSDetector_DetectNodeJS_Basic(t *testing.T) {
	logger := NewSetupLogger(&SetupLoggerConfig{
		Level:     LogLevelDebug,
		Component: "nodejs-detector-test",
		QuietMode: true, // Suppress user output during tests
	})

	detector := NewNodeJSDetector(logger)

	info, err := detector.DetectNodeJS()
	if err != nil {
		t.Fatalf("DetectNodeJS should not return error, got: %v", err)
	}

	if info == nil {
		t.Fatal("Expected info to be returned")
	}

	if info.RuntimeInfo == nil {
		t.Fatal("Expected RuntimeInfo to be set")
	}

	if info.Name != "nodejs" {
		t.Errorf("Expected runtime name 'nodejs', got '%s'", info.Name)
	}

	if info.GlobalPackages == nil {
		t.Error("Expected GlobalPackages to be initialized")
	}

	if info.PermissionIssues == nil {
		t.Error("Expected PermissionIssues to be initialized")
	}
}

func TestNodeJSDetector_GetPackageManagerRecommendation(t *testing.T) {
	logger := NewSetupLogger(&SetupLoggerConfig{
		Level:     LogLevelDebug,
		Component: "nodejs-detector-test",
		QuietMode: true,
	})

	detector := NewNodeJSDetector(logger)

	info := &NodeJSInfo{
		RuntimeInfo: &RuntimeInfo{
			Name:      "nodejs",
			Installed: false,
		},
		GlobalPackages:   []string{},
		PermissionIssues: []string{},
	}

	recommendation := detector.GetPackageManagerRecommendation(info)
	if recommendation == nil {
		t.Fatal("Expected recommendation to be returned")
	}

	if recommendation.Name != "none" {
		t.Errorf("Expected recommendation name 'none', got '%s'", recommendation.Name)
	}

	if recommendation.Available {
		t.Error("Expected recommendation.Available to be false")
	}
}

func TestNodeJSDetector_ValidateForTypeScriptLS(t *testing.T) {
	logger := NewSetupLogger(&SetupLoggerConfig{
		Level:     LogLevelDebug,
		Component: "nodejs-detector-test",
		QuietMode: true,
	})

	detector := NewNodeJSDetector(logger)

	info := &NodeJSInfo{
		RuntimeInfo: &RuntimeInfo{
			Name:      "nodejs",
			Installed: false,
		},
		GlobalPackages:   []string{},
		PermissionIssues: []string{},
	}

	issues := detector.ValidateForTypeScriptLS(info)
	if len(issues) == 0 {
		t.Error("Expected validation issues when Node.js is not installed")
	}

	foundNodeNotInstalled := false
	for _, issue := range issues {
		if issue == "Node.js is not installed" {
			foundNodeNotInstalled = true
			break
		}
	}

	if !foundNodeNotInstalled {
		t.Error("Expected 'Node.js is not installed' issue")
	}
}

func TestNodeJSDetector_GetInstallationGuidance(t *testing.T) {
	logger := NewSetupLogger(&SetupLoggerConfig{
		Level:     LogLevelDebug,
		Component: "nodejs-detector-test",
		QuietMode: true,
	})

	detector := NewNodeJSDetector(logger)

	info := &NodeJSInfo{
		RuntimeInfo: &RuntimeInfo{
			Name:      "nodejs",
			Installed: false,
		},
		GlobalPackages:   []string{},
		PermissionIssues: []string{},
	}

	guidance := detector.GetInstallationGuidance(info)
	if len(guidance) == 0 {
		t.Error("Expected installation guidance when Node.js is not installed")
	}

	foundInstallInstructions := false
	for _, guide := range guidance {
		// Check for numbered instructions, accounting for possible leading whitespace
		trimmed := strings.TrimSpace(guide)
		if len(trimmed) > 0 && (strings.HasPrefix(trimmed, "1.") || strings.HasPrefix(trimmed, "2.") || strings.HasPrefix(trimmed, "3.")) {
			foundInstallInstructions = true
			break
		}
	}

	if !foundInstallInstructions {
		t.Errorf("Expected numbered installation instructions in guidance, got: %v", guidance)
	}
}
