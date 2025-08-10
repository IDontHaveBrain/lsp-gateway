package installer

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"lsp-gateway/src/internal/common"
)

// RustInstaller handles Rust language server (rust-analyzer) installation
type RustInstaller struct {
	*BaseInstaller
}

// NewRustInstaller creates a new Rust installer
func NewRustInstaller(platform PlatformInfo) *RustInstaller {
	base := CreateSimpleInstaller("rust", "rust-analyzer", []string{}, platform)
	return &RustInstaller{BaseInstaller: base}
}

// Install installs rust-analyzer using rustup if available
func (r *RustInstaller) Install(ctx context.Context, options InstallOptions) error {
	// Check if rustup is available
	if _, err := exec.LookPath("rustup"); err != nil {
		// If rustup is not available, check if rust-analyzer works standalone
		testCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if output, err := r.RunCommandWithOutput(testCtx, "rust-analyzer", "--version"); err == nil {
			if !options.Force {
				common.CLILogger.Info("rust-analyzer already available and working (standalone): %s", strings.TrimSpace(output))
				return nil
			}
			common.CLILogger.Info("rust-analyzer found but force reinstall requested")
		}
		return fmt.Errorf("rustup not found. Install Rust from https://rustup.rs and then run this command again")
	}

	// If not forcing and rust-analyzer already works, skip installation
	if !options.Force {
		testCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if output, err := r.RunCommandWithOutput(testCtx, "rust-analyzer", "--version"); err == nil {
			common.CLILogger.Info("rust-analyzer already available and working: %s", strings.TrimSpace(output))
			return nil
		}
	}

	// Rustup is available, ensure rust-analyzer component is installed
	common.CLILogger.Info("Setting up rust-analyzer via rustup...")

	// First, try to add rust-analyzer component to stable toolchain
	err := r.RunCommand(ctx, "rustup", "component", "add", "rust-analyzer")
	if err == nil {
		// Verify it works
		testCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if output, err := r.RunCommandWithOutput(testCtx, "rust-analyzer", "--version"); err == nil {
			common.CLILogger.Info("rust-analyzer installed and working via rustup (stable): %s", strings.TrimSpace(output))
			return nil
		}
	}

	// If stable didn't work, try rust-analyzer-preview
	common.CLILogger.Info("Trying rust-analyzer-preview component...")
	err = r.RunCommand(ctx, "rustup", "component", "add", "rust-analyzer-preview")
	if err == nil {
		testCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if output, err := r.RunCommandWithOutput(testCtx, "rust-analyzer", "--version"); err == nil {
			common.CLILogger.Info("rust-analyzer-preview installed and working via rustup (stable): %s", strings.TrimSpace(output))
			return nil
		}
	}

	// If stable doesn't have rust-analyzer, try nightly toolchain
	common.CLILogger.Info("Stable toolchain doesn't have rust-analyzer, trying nightly...")

	// Install nightly toolchain if not present
	_ = r.RunCommand(ctx, "rustup", "toolchain", "install", "nightly")

	// Try to add rust-analyzer to nightly
	err = r.RunCommand(ctx, "rustup", "component", "add", "rust-analyzer", "--toolchain", "nightly")
	if err == nil {
		// Configure to use nightly rust-analyzer
		r.serverConfig.Command = "rustup"
		r.serverConfig.Args = []string{"run", "nightly", "rust-analyzer"}

		// Verify it works
		testCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if output, err := r.RunCommandWithOutput(testCtx, "rustup", "run", "nightly", "rust-analyzer", "--version"); err == nil {
			common.CLILogger.Info("rust-analyzer installed and working via rustup (nightly): %s", strings.TrimSpace(output))
			if err := r.ensureRustAnalyzerWrapper(); err != nil {
				common.CLILogger.Warn("failed to configure rust-analyzer wrapper: %v", err)
			}
			return nil
		}
	}

	// Try rust-analyzer-preview on nightly
	err = r.RunCommand(ctx, "rustup", "component", "add", "rust-analyzer-preview", "--toolchain", "nightly")
	if err == nil {
		r.serverConfig.Command = "rustup"
		r.serverConfig.Args = []string{"run", "nightly", "rust-analyzer"}

		testCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if output, err := r.RunCommandWithOutput(testCtx, "rustup", "run", "nightly", "rust-analyzer", "--version"); err == nil {
			common.CLILogger.Info("rust-analyzer-preview installed and working via rustup (nightly): %s", strings.TrimSpace(output))
			if err := r.ensureRustAnalyzerWrapper(); err != nil {
				common.CLILogger.Warn("failed to configure rust-analyzer wrapper: %v", err)
			}
			return nil
		}
	}

	return fmt.Errorf("failed to install rust-analyzer via rustup. Please install manually from https://rust-analyzer.github.io/manual.html#rust-analyzer-binaries")
}

// Uninstall removes rust-analyzer using rustup if available
func (r *RustInstaller) Uninstall() error {
	if _, err := exec.LookPath("rustup"); err == nil {
		return r.RunCommand(context.Background(), "rustup", "component", "remove", "rust-analyzer")
	}
	return nil
}

// IsInstalled checks if rust-analyzer is properly installed and working
func (r *RustInstaller) IsInstalled() bool {
	// Try to actually run rust-analyzer with --version
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check if rust-analyzer actually works (not just wrapper exists)
	if _, err := r.RunCommandWithOutput(ctx, "rust-analyzer", "--version"); err == nil {
		return true
	}

	// Check if configured via rustup run
	if r.serverConfig.Command == "rustup" && len(r.serverConfig.Args) > 0 {
		args := append(r.serverConfig.Args, "--version")
		if _, err := r.RunCommandWithOutput(ctx, "rustup", args...); err == nil {
			return true
		}
	}

	return false
}

// ValidateInstallation performs comprehensive validation
func (r *RustInstaller) ValidateInstallation() error {
	// Try to actually run rust-analyzer
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// First try direct execution
	if _, err := r.RunCommandWithOutput(ctx, "rust-analyzer", "--version"); err == nil {
		return nil
	}

	// Try wrapper path
	wrapperPath := filepath.Join(r.GetInstallPath(), "bin", "rust-analyzer")
	if _, err := os.Stat(wrapperPath); err == nil {
		if _, err := r.RunCommandWithOutput(ctx, wrapperPath, "--version"); err == nil {
			return nil
		}
	}

	// Try via rustup run if configured
	if r.serverConfig.Command == "rustup" && len(r.serverConfig.Args) > 0 {
		args := append(r.serverConfig.Args, "--version")
		if _, err := r.RunCommandWithOutput(ctx, "rustup", args...); err == nil {
			return nil
		}
	}

	return fmt.Errorf("rust-analyzer is not properly installed or not executable")
}

func (r *RustInstaller) ensureRustAnalyzerWrapper() error {
	binDir := filepath.Join(r.GetInstallPath(), "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return err
	}
	wrapperPath := filepath.Join(binDir, "rust-analyzer")
	script := "#!/usr/bin/env bash\nset -e\nif command -v rustup >/dev/null 2>&1; then\n  if rustup component list --installed --toolchain stable 2>/dev/null | grep -q '^rust-analyzer'; then\n    exec rustup run stable rust-analyzer \"$@\"\n  elif rustup which --toolchain nightly rust-analyzer >/dev/null 2>&1; then\n    exec rustup run nightly rust-analyzer \"$@\"\n  fi\nfi\necho 'rust-analyzer not available in stable or nightly via rustup' 1>&2\nexit 1\n"
	if err := os.WriteFile(wrapperPath, []byte(script), 0755); err != nil {
		return err
	}
	return r.addPathToShell(binDir)
}

func (r *RustInstaller) addPathToShell(binDir string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	files := []string{filepath.Join(home, ".bashrc"), filepath.Join(home, ".zshrc"), filepath.Join(home, ".profile")}
	exportLine := fmt.Sprintf("export PATH=\"%s:$PATH\"\n", binDir)
	for _, f := range files {
		data, _ := os.ReadFile(f)
		if strings.Contains(string(data), binDir) {
			continue
		}
		_ = os.MkdirAll(filepath.Dir(f), 0755)
		file, err := os.OpenFile(f, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			continue
		}
		_, _ = file.WriteString("\n" + exportLine)
		_ = file.Close()
	}
	return nil
}
