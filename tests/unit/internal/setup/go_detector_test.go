package setup_test

import (
	"context"
	"lsp-gateway/internal/setup"
	"testing"
	"time"
)

func TestGoDetector_DetectGo(t *testing.T) {
	detector := setup.NewGoDetector()
	detector.SetTimeout(10 * time.Second)

	info, err := detector.DetectGo()
	if err != nil {
		t.Fatalf("DetectGo failed: %v", err)
	}

	if info == nil {
		t.Fatal("DetectGo returned nil info")
	}

	if info.Name != "go" {
		t.Errorf("Expected runtime name 'go', got '%s'", info.Name)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	info2, err := detector.DetectGoWithContext(ctx)
	if err != nil {
		t.Fatalf("DetectGoWithContext failed: %v", err)
	}

	if info2 == nil {
		t.Fatal("DetectGoWithContext returned nil info")
	}

	if info2.Name != "go" {
		t.Errorf("Expected runtime name 'go', got '%s'", info2.Name)
	}
}

func TestGoDetector_DetectGoExtended(t *testing.T) {
	detector := setup.NewGoDetector()

	extendedInfo, err := detector.DetectGoExtended()
	if err != nil {
		t.Fatalf("DetectGoExtended failed: %v", err)
	}

	if extendedInfo == nil {
		t.Fatal("DetectGoExtended returned nil info")
	}

	if extendedInfo.RuntimeInfo == nil {
		t.Fatal("DetectGoExtended returned nil RuntimeInfo")
	}

	if extendedInfo.Name != "go" {
		t.Errorf("Expected runtime name 'go', got '%s'", extendedInfo.Name)
	}
}

func TestGoDetector_ValidateGoInstallation(t *testing.T) {
	detector := setup.NewGoDetector()

	err := detector.ValidateGoInstallation()
	if err != nil {
		t.Logf("Go validation failed (expected if Go not installed): %v", err)
	}
}

func TestGoDetector_GetGoRecommendations(t *testing.T) {
	detector := setup.NewGoDetector()

	recommendations := detector.GetGoRecommendations()
	if len(recommendations) == 0 {
		t.Error("Expected at least one recommendation")
	}

	for i, rec := range recommendations {
		if rec == "" {
			t.Errorf("Recommendation %d is empty", i)
		}
	}
}

func TestGoDetector_VersionMethods(t *testing.T) {
	detector := setup.NewGoDetector()

	minVersion := detector.GetMinimumVersion()
	if minVersion == "" {
		t.Error("GetMinimumVersion returned empty string")
	}

	detector.SetMinimumVersion("1.20.0")
	newMinVersion := detector.GetMinimumVersion()
	if newMinVersion != "1.20.0" {
		t.Errorf("Expected minimum version '1.20.0', got '%s'", newMinVersion)
	}
}

func TestGoDetector_Timeout(t *testing.T) {
	detector := setup.NewGoDetector()

	newTimeout := 15 * time.Second
	detector.SetTimeout(newTimeout)

	if detector.timeout != newTimeout {
		t.Errorf("Expected timeout %v, got %v", newTimeout, detector.timeout)
	}
}

func TestDefaultRuntimeDetector_DetectGo(t *testing.T) {
	detector := setup.NewRuntimeDetector()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	info, err := detector.DetectGo(ctx)
	if err != nil {
		t.Fatalf("DefaultRuntimeDetector.DetectGo failed: %v", err)
	}

	if info == nil {
		t.Fatal("DefaultRuntimeDetector.DetectGo returned nil info")
	}

	if info.Name != "go" {
		t.Errorf("Expected runtime name 'go', got '%s'", info.Name)
	}
}
