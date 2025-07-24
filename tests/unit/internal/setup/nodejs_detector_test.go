package setup_test

import (
	"lsp-gateway/internal/setup"
	"strings"
	"testing"
)

func TestNodeJSDetector_NewNodeJSDetector(t *testing.T) {
	logger := setup.NewSetupLogger(&setup.SetupLoggerConfig{
		Level:     setup.LogLevelInfo,
		Component: "nodejs-detector-test",
	})

	detector := setup.NewNodeJSDetector(logger)
	if detector == nil {
		t.Fatal("Expected NewNodeJSDetector to return a detector instance")
	}

	// Test that the detector is functional by calling one of its methods
	// This indirectly verifies that the internal components are properly initialized
	info, err := detector.DetectNodeJS()
	if info == nil && err == nil {
		t.Error("Expected detector to return either info or error")
	}
}

func TestNodeJSDetector_DetectNodeJS_Basic(t *testing.T) {
	logger := setup.NewSetupLogger(&setup.SetupLoggerConfig{
		Level:     setup.LogLevelDebug,
		Component: "nodejs-detector-test",
		QuietMode: true, // Suppress user output during tests
	})

	detector := setup.NewNodeJSDetector(logger)

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
	logger := setup.NewSetupLogger(&setup.SetupLoggerConfig{
		Level:     setup.LogLevelDebug,
		Component: "nodejs-detector-test",
		QuietMode: true,
	})

	detector := setup.NewNodeJSDetector(logger)

	info := &setup.NodeJSInfo{
		RuntimeInfo: &setup.RuntimeInfo{
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
	logger := setup.NewSetupLogger(&setup.SetupLoggerConfig{
		Level:     setup.LogLevelDebug,
		Component: "nodejs-detector-test",
		QuietMode: true,
	})

	detector := setup.NewNodeJSDetector(logger)

	info := &setup.NodeJSInfo{
		RuntimeInfo: &setup.RuntimeInfo{
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
	logger := setup.NewSetupLogger(&setup.SetupLoggerConfig{
		Level:     setup.LogLevelDebug,
		Component: "nodejs-detector-test",
		QuietMode: true,
	})

	detector := setup.NewNodeJSDetector(logger)

	info := &setup.NodeJSInfo{
		RuntimeInfo: &setup.RuntimeInfo{
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
