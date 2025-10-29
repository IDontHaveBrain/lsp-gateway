package installer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type testPlatformInfo struct {
	os string
}

func (t *testPlatformInfo) GetPlatform() string                             { return t.os }
func (t *testPlatformInfo) GetArch() string                                 { return "amd64" }
func (t *testPlatformInfo) GetPlatformString() string                       { return t.os + "-amd64" }
func (t *testPlatformInfo) IsSupported() bool                               { return true }
func (t *testPlatformInfo) GetJavaDownloadURL(string) (string, string, error) { return "", "", nil }
func (t *testPlatformInfo) GetNodeInstallCommand() []string                 { return nil }
func (t *testPlatformInfo) IsWindows() bool                                 { return t.os == "windows" }
func (t *testPlatformInfo) IsLinux() bool                                   { return t.os == "linux" }
func (t *testPlatformInfo) IsDarwin() bool                                  { return t.os == "darwin" }
func (t *testPlatformInfo) GetBinaryExtension() string {
	if t.IsWindows() {
		return ".exe"
	}
	return ""
}
func (t *testPlatformInfo) GetScriptExtension() string {
	if t.IsWindows() {
		return ".bat"
	}
	return ".sh"
}
func (t *testPlatformInfo) FormatBinaryName(name string) string {
	if t.IsWindows() && !strings.HasSuffix(name, ".exe") {
		return name + ".exe"
	}
	return name
}

func TestNewWrapperScriptBuilder(t *testing.T) {
	platform := &testPlatformInfo{os: "linux"}
	builder := NewWrapperScriptBuilder(platform, "go")

	if builder == nil {
		t.Fatal("Expected non-nil builder")
	}

	if builder.language != "go" {
		t.Errorf("Expected language 'go', got '%s'", builder.language)
	}
}

func TestWrapperScriptBuilder_Build_NoExecutable(t *testing.T) {
	platform := &testPlatformInfo{os: "linux"}
	builder := NewWrapperScriptBuilder(platform, "go")

	_, err := builder.Build()
	if err == nil {
		t.Error("Expected error when executable not set")
	}
}

func TestWrapperScriptBuilder_UnixScript(t *testing.T) {
	platform := &testPlatformInfo{os: "linux"}
	builder := NewWrapperScriptBuilder(platform, "go").
		SetExecutable("/path/to/gopls").
		SetArgs("serve").
		SetEnvVar("GOPATH", "/home/user/go")

	content, err := builder.Build()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify shebang
	if !strings.HasPrefix(content, "#!/bin/sh\n") {
		t.Error("Expected Unix script to start with shebang")
	}

	// Verify environment variable
	if !strings.Contains(content, "export GOPATH=\"/home/user/go\"") {
		t.Error("Expected GOPATH environment variable in script")
	}

	// Verify executable call
	if !strings.Contains(content, "exec \"$SCRIPT_DIR/gopls\"") {
		t.Error("Expected executable call in script")
	}

	// Verify arguments
	if !strings.Contains(content, " serve") {
		t.Error("Expected 'serve' argument in script")
	}

	// Verify pass-through args
	if !strings.Contains(content, "\"$@\"") {
		t.Error("Expected pass-through args in script")
	}
}

func TestWrapperScriptBuilder_WindowsScript(t *testing.T) {
	platform := &testPlatformInfo{os: "windows"}
	builder := NewWrapperScriptBuilder(platform, "python").
		SetExecutable("C:\\tools\\pyright").
		SetArgs("--stdio").
		SetEnvVar("PYTHONPATH", "C:\\Python")

	content, err := builder.Build()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify batch file header
	if !strings.HasPrefix(content, "@echo off\n") {
		t.Error("Expected Windows script to start with @echo off")
	}

	// Verify environment variable
	if !strings.Contains(content, "set PYTHONPATH=C:\\Python") {
		t.Error("Expected PYTHONPATH environment variable in script")
	}

	// Verify executable call (should add .exe if not present)
	if !strings.Contains(content, "pyright.exe") {
		t.Error("Expected executable with .exe extension in script")
	}

	// Verify arguments
	if !strings.Contains(content, " --stdio") {
		t.Error("Expected '--stdio' argument in script")
	}

	// Verify pass-through args
	if !strings.Contains(content, " %*") {
		t.Error("Expected pass-through args in script")
	}
}

func TestWrapperScriptBuilder_WriteToFile(t *testing.T) {
	tempDir := t.TempDir()
	scriptPath := filepath.Join(tempDir, "bin", "wrapper.sh")

	platform := &testPlatformInfo{os: "linux"}
	builder := NewWrapperScriptBuilder(platform, "test").
		SetExecutable("/path/to/test-server")

	err := builder.WriteToFile(scriptPath)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		t.Error("Expected script file to be created")
	}

	// Verify file is executable on Unix
	info, err := os.Stat(scriptPath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	if info.Mode().Perm()&0111 == 0 {
		t.Error("Expected script file to be executable")
	}

	// Verify content
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if !strings.Contains(string(content), "#!/bin/sh") {
		t.Error("Expected shebang in written file")
	}
}

func TestWrapperScriptBuilder_MultipleArgs(t *testing.T) {
	platform := &testPlatformInfo{os: "linux"}
	builder := NewWrapperScriptBuilder(platform, "java").
		SetExecutable("/path/to/jdtls").
		SetArgs("-Xmx1G", "-jar", "server.jar")

	content, err := builder.Build()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !strings.Contains(content, " -Xmx1G") {
		t.Error("Expected first arg in script")
	}

	if !strings.Contains(content, " -jar") {
		t.Error("Expected second arg in script")
	}

	if !strings.Contains(content, " server.jar") {
		t.Error("Expected third arg in script")
	}
}
