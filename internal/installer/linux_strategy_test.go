package installer

import (
	"runtime"
	"testing"

	"lsp-gateway/internal/platform"
)

func TestNewLinuxStrategy(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping Linux-specific test on non-Linux platform")
	}

	strategy, err := NewLinuxStrategy()
	if err != nil {
		t.Fatalf("Failed to create Linux strategy: %v", err)
	}

	if strategy == nil {
		t.Fatal("Linux strategy is nil")
	}

	info := strategy.GetPlatformInfo()
	if info.OS != "linux" {
		t.Errorf("Expected OS to be 'linux', got %s", info.OS)
	}

	managers := strategy.GetPackageManagers()
	if len(managers) == 0 {
		t.Error("Expected at least one package manager to be available")
	}

	for _, mgr := range managers {
		if !strategy.IsPackageManagerAvailable(mgr) {
			t.Errorf("Package manager %s should be available", mgr)
		}
	}
}

func TestLinuxStrategyMethods(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping Linux-specific test on non-Linux platform")
	}

	strategy, err := NewLinuxStrategy()
	if err != nil {
		t.Fatalf("Failed to create Linux strategy: %v", err)
	}

	methods := []func(string) error{
		strategy.InstallGo,
		strategy.InstallPython,
		strategy.InstallNodejs,
		strategy.InstallJava,
	}

	for i, method := range methods {
		err := method("latest")
		if err == nil {
			t.Logf("Method %d succeeded (component may already be installed)", i)
		} else {
			t.Logf("Method %d failed as expected: %v", i, err)
		}
	}
}

func TestLinuxDistributionDetection(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping Linux-specific test on non-Linux platform")
	}

	info, err := platform.DetectLinuxDistribution()
	if err != nil {
		t.Fatalf("Failed to detect Linux distribution: %v", err)
	}

	if info.Distribution == platform.DistributionUnknown {
		t.Error("Distribution detection returned unknown")
	}

	t.Logf("Detected distribution: %s, version: %s", info.Distribution, info.Version)

	preferred := platform.GetPreferredPackageManagers(info.Distribution)
	if len(preferred) == 0 {
		t.Error("No preferred package managers returned")
	}

	t.Logf("Preferred package managers: %v", preferred)
}

func TestPackageManagerAvailability(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping Linux-specific test on non-Linux platform")
	}

	managers := []platform.PackageManager{
		platform.NewAptManager(),
		platform.NewYumManager(),
		platform.NewDnfManager(),
	}

	var available []string
	for _, mgr := range managers {
		if mgr.IsAvailable() {
			available = append(available, mgr.GetName())
		}
	}

	if len(available) == 0 {
		t.Error("No package managers are available on this Linux system")
	}

	t.Logf("Available package managers: %v", available)
}
