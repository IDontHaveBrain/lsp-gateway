package platform

import (
	"runtime"
	"testing"
)

func TestHomebrewManager(t *testing.T) {
	mgr := NewHomebrewManager()

	if mgr.GetName() != "homebrew" {
		t.Errorf("expected name 'homebrew', got '%s'", mgr.GetName())
	}
	if mgr.RequiresAdmin() {
		t.Errorf("expected RequiresAdmin to be false for homebrew")
	}

	if runtime.GOOS == "darwin" {
		_ = mgr.IsAvailable()
	} else {
		if mgr.IsAvailable() {
			t.Errorf("expected homebrew to not be available on non-darwin platform")
		}
	}
}

func TestAptManager(t *testing.T) {
	mgr := NewAptManager()

	if mgr.GetName() != "apt" {
		t.Errorf("expected name 'apt', got '%s'", mgr.GetName())
	}
	if !mgr.RequiresAdmin() {
		t.Errorf("expected RequiresAdmin to be true for apt")
	}

	if runtime.GOOS == "linux" {
		_ = mgr.IsAvailable()
	} else {
		if mgr.IsAvailable() {
			t.Errorf("expected apt to not be available on non-linux platform")
		}
	}
}

func TestWingetManager(t *testing.T) {
	mgr := NewWingetManager()

	if mgr.GetName() != "winget" {
		t.Errorf("expected name 'winget', got '%s'", mgr.GetName())
	}
	if mgr.RequiresAdmin() {
		t.Errorf("expected RequiresAdmin to be false for winget")
	}

	if runtime.GOOS == "windows" {
		_ = mgr.IsAvailable()
	} else {
		if mgr.IsAvailable() {
			t.Errorf("expected winget to not be available on non-windows platform")
		}
	}
}

func TestChocolateyManager(t *testing.T) {
	mgr := NewChocolateyManager()

	if mgr.GetName() != "chocolatey" {
		t.Errorf("expected name 'chocolatey', got '%s'", mgr.GetName())
	}
	if !mgr.RequiresAdmin() {
		t.Errorf("expected RequiresAdmin to be true for chocolatey")
	}

	if runtime.GOOS == "windows" {
		_ = mgr.IsAvailable()
	} else {
		if mgr.IsAvailable() {
			t.Errorf("expected chocolatey to not be available on non-windows platform")
		}
	}
}

func TestGetAvailablePackageManagers(t *testing.T) {
	managers := GetAvailablePackageManagers()

	if managers == nil {
		t.Error("expected non-nil slice of managers")
	}

	for _, mgr := range managers {
		platforms := mgr.GetPlatforms()
		found := false
		for _, platform := range platforms {
			if platform == runtime.GOOS {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("manager %s should support current platform %s", mgr.GetName(), runtime.GOOS)
		}
	}
}

func TestGetBestPackageManager(t *testing.T) {
	best := GetBestPackageManager()

	switch runtime.GOOS {
	case "darwin", "linux", "windows":
		if best != nil {
			t.Logf("got package manager %s on platform %s", best.GetName(), runtime.GOOS)
		}
	default:
		if best != nil {
			t.Logf("unexpectedly got package manager %s on unsupported platform %s", best.GetName(), runtime.GOOS)
		}
	}
}
