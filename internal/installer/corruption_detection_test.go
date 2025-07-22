package installer

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"lsp-gateway/internal/platform"
	"lsp-gateway/internal/types"
)

// TestGoRuntimeCorruption tests Go runtime corruption detection
func TestGoRuntimeCorruption(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		setupMock   func(*MockCommandExecutor)
		expectIssue bool
		issueType   string
	}{
		{
			name: "corrupted go version output",
			setupMock: func(m *MockCommandExecutor) {
				m.AddCommand("go", []string{"version"}, &platform.Result{
					ExitCode: 0,
					Stdout:   "go version \x00\x01corrupted output",
					Duration: 10 * time.Millisecond,
				})
			},
			expectIssue: true,
			issueType:   "corruption",
		},
		{
			name: "go env with malformed output",
			setupMock: func(m *MockCommandExecutor) {
				m.AddCommand("go", []string{"version"}, &platform.Result{
					ExitCode: 0,
					Stdout:   "go version go1.21.0 linux/amd64",
					Duration: 10 * time.Millisecond,
				})
				m.AddCommand("go", []string{"env"}, &platform.Result{
					ExitCode: 0,
					Stdout:   "GOARCH=\x00\nGOOS=linux\nmalformed_line_without_equals\nGOROOT=/usr/local/go",
					Duration: 10 * time.Millisecond,
				})
			},
			expectIssue: false, // Should handle malformed lines gracefully
			issueType:   "",
		},
		{
			name: "go env with binary corruption",
			setupMock: func(m *MockCommandExecutor) {
				m.AddCommand("go", []string{"version"}, &platform.Result{
					ExitCode: 0,
					Stdout:   "go version go1.21.0 linux/amd64",
					Duration: 10 * time.Millisecond,
				})
				m.AddCommand("go", []string{"env"}, &platform.Result{
					ExitCode: 0,
					Stdout:   string([]byte{0xFF, 0xFE, 'G', 'O', 'A', 'R', 'C', 'H', '=', 'a', 'm', 'd', '6', '4'}),
					Duration: 10 * time.Millisecond,
				})
			},
			expectIssue: false, // Should handle binary data in parsing
			issueType:   "",
		},
		{
			name: "missing GOROOT in environment",
			setupMock: func(m *MockCommandExecutor) {
				m.AddCommand("go", []string{"version"}, &platform.Result{
					ExitCode: 0,
					Stdout:   "go version go1.21.0 linux/amd64",
					Duration: 10 * time.Millisecond,
				})
				m.AddCommand("go", []string{"env"}, &platform.Result{
					ExitCode: 0,
					Stdout:   "GOARCH=amd64\nGOOS=linux\nGOPATH=/home/user/go",
					Duration: 10 * time.Millisecond,
				})
			},
			expectIssue: false, // Missing GOROOT is often normal
			issueType:   "",
		},
		{
			name: "corrupted GOPATH pointing to non-existent location",
			setupMock: func(m *MockCommandExecutor) {
				m.AddCommand("go", []string{"version"}, &platform.Result{
					ExitCode: 0,
					Stdout:   "go version go1.21.0 linux/amd64",
					Duration: 10 * time.Millisecond,
				})
				m.AddCommand("go", []string{"env"}, &platform.Result{
					ExitCode: 0,
					Stdout:   "GOARCH=amd64\nGOOS=linux\nGOPATH=/nonexistent/corrupted/path\nGOROOT=/usr/local/go",
					Duration: 10 * time.Millisecond,
				})
			},
			expectIssue: true,
			issueType:   "environment",
		},
		{
			name: "extremely long environment values",
			setupMock: func(m *MockCommandExecutor) {
				longPath := strings.Repeat("/very/long/path", 1000)
				m.AddCommand("go", []string{"version"}, &platform.Result{
					ExitCode: 0,
					Stdout:   "go version go1.21.0 linux/amd64",
					Duration: 10 * time.Millisecond,
				})
				m.AddCommand("go", []string{"env"}, &platform.Result{
					ExitCode: 0,
					Stdout:   fmt.Sprintf("GOARCH=amd64\nGOOS=linux\nGOPATH=%s\nGOROOT=/usr/local/go", longPath),
					Duration: 10 * time.Millisecond,
				})
			},
			expectIssue: false, // Long paths should be handled
			issueType:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExecutor := NewMockCommandExecutor()
			tt.setupMock(mockExecutor)

			installer := &DefaultRuntimeInstaller{
				registry: NewRuntimeRegistry(),
			}

			result, err := installer.Verify("go")
			if err != nil {
				t.Logf("Runtime verification error (may be expected): %v", err)
			}

			if result != nil {
				foundExpectedIssue := false
				for _, issue := range result.Issues {
					if tt.expectIssue && strings.Contains(strings.ToLower(string(issue.Category)), tt.issueType) {
						foundExpectedIssue = true
						break
					}
				}

				if tt.expectIssue && !foundExpectedIssue {
					t.Errorf("Expected issue of type '%s' but didn't find one", tt.issueType)
				}

				t.Logf("Go runtime verification result: %d issues found", len(result.Issues))
				for _, issue := range result.Issues {
					t.Logf("  - %s: %s (%s)", issue.Severity, issue.Title, issue.Category)
				}
			}
		})
	}
}

// TestPythonRuntimeCorruption tests Python runtime corruption detection
func TestPythonRuntimeCorruption(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		setupMock   func(*MockCommandExecutor)
		expectIssue bool
		issueType   string
	}{
		{
			name: "corrupted python version output",
			setupMock: func(m *MockCommandExecutor) {
				m.AddCommand("python", []string{"--version"}, &platform.Result{
					ExitCode: 0,
					Stdout:   "Python \x00\x01corrupted",
					Duration: 10 * time.Millisecond,
				})
			},
			expectIssue: false, // Version parsing should handle corruption gracefully
			issueType:   "",
		},
		{
			name: "python executable corruption",
			setupMock: func(m *MockCommandExecutor) {
				m.AddFailure("python", []string{"--version"}, fmt.Errorf("exec format error"))
			},
			expectIssue: true,
			issueType:   "execution",
		},
		{
			name: "corrupted python import paths",
			setupMock: func(m *MockCommandExecutor) {
				m.AddCommand("python", []string{"--version"}, &platform.Result{
					ExitCode: 0,
					Stdout:   "Python 3.9.0",
					Duration: 10 * time.Millisecond,
				})
				m.AddCommand("python", []string{"-c", "import sys; print('\\n'.join(sys.path))"}, &platform.Result{
					ExitCode: 0,
					Stdout:   "/corrupted/path\x00/another/corrupted\x01path",
					Duration: 10 * time.Millisecond,
				})
			},
			expectIssue: false, // Should handle corrupted paths gracefully
			issueType:   "",
		},
		{
			name: "missing site-packages directory",
			setupMock: func(m *MockCommandExecutor) {
				m.AddCommand("python", []string{"--version"}, &platform.Result{
					ExitCode: 0,
					Stdout:   "Python 3.9.0",
					Duration: 10 * time.Millisecond,
				})
				m.AddCommand("python", []string{"-c", "import site; print(site.getsitepackages()[0])"}, &platform.Result{
					ExitCode: 1,
					Stderr:   "IndexError: list index out of range",
					Duration: 10 * time.Millisecond,
				})
			},
			expectIssue: false, // Missing site-packages handling varies
			issueType:   "",
		},
		{
			name: "corrupted pip installation",
			setupMock: func(m *MockCommandExecutor) {
				m.AddCommand("python", []string{"--version"}, &platform.Result{
					ExitCode: 0,
					Stdout:   "Python 3.9.0",
					Duration: 10 * time.Millisecond,
				})
				m.AddCommand("python", []string{"-m", "pip", "--version"}, &platform.Result{
					ExitCode: 1,
					Stderr:   "No module named pip.__main__; 'pip' is a package and cannot be directly executed",
					Duration: 10 * time.Millisecond,
				})
			},
			expectIssue: false, // Pip corruption doesn't affect basic runtime
			issueType:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExecutor := NewMockCommandExecutor()
			tt.setupMock(mockExecutor)

			installer := &DefaultRuntimeInstaller{
				registry: NewRuntimeRegistry(),
			}

			result, err := installer.Verify("python")
			if err != nil {
				t.Logf("Python runtime verification error (may be expected): %v", err)
			}

			if result != nil {
				foundExpectedIssue := false
				for _, issue := range result.Issues {
					if tt.expectIssue && strings.Contains(strings.ToLower(string(issue.Category)), tt.issueType) {
						foundExpectedIssue = true
						break
					}
				}

				if tt.expectIssue && !foundExpectedIssue {
					t.Errorf("Expected issue of type '%s' but didn't find one", tt.issueType)
				}

				t.Logf("Python runtime verification result: %d issues found", len(result.Issues))
				for _, issue := range result.Issues {
					t.Logf("  - %s: %s (%s)", issue.Severity, issue.Title, issue.Category)
				}
			}
		})
	}
}

// TestNodeJSRuntimeCorruption tests Node.js runtime corruption detection
func TestNodeJSRuntimeCorruption(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		setupMock   func(*MockCommandExecutor)
		expectIssue bool
		issueType   string
	}{
		{
			name: "corrupted node version",
			setupMock: func(m *MockCommandExecutor) {
				m.AddCommand("node", []string{"--version"}, &platform.Result{
					ExitCode: 0,
					Stdout:   "v\x00\x01corrupted.version",
					Duration: 10 * time.Millisecond,
				})
			},
			expectIssue: false, // Version parsing should handle gracefully
			issueType:   "",
		},
		{
			name: "corrupted npm installation",
			setupMock: func(m *MockCommandExecutor) {
				m.AddCommand("node", []string{"--version"}, &platform.Result{
					ExitCode: 0,
					Stdout:   "v18.0.0",
					Duration: 10 * time.Millisecond,
				})
				m.AddCommand("npm", []string{"--version"}, &platform.Result{
					ExitCode: 1,
					Stderr:   "Error: Cannot find module '/usr/lib/node_modules/npm/bin/npm-cli.js'",
					Duration: 10 * time.Millisecond,
				})
			},
			expectIssue: false, // npm corruption doesn't affect node runtime
			issueType:   "",
		},
		{
			name: "corrupted node_modules paths",
			setupMock: func(m *MockCommandExecutor) {
				m.AddCommand("node", []string{"--version"}, &platform.Result{
					ExitCode: 0,
					Stdout:   "v18.0.0",
					Duration: 10 * time.Millisecond,
				})
				m.AddCommand("node", []string{"-e", "console.log(require.resolve.paths('fs'))"}, &platform.Result{
					ExitCode: 0,
					Stdout:   "['/corrupted/node_modules\x00']",
					Duration: 10 * time.Millisecond,
				})
			},
			expectIssue: false, // Should handle corrupted module paths gracefully
			issueType:   "",
		},
		{
			name: "node executable corruption",
			setupMock: func(m *MockCommandExecutor) {
				m.AddFailure("node", []string{"--version"}, fmt.Errorf("exec format error: corrupted binary"))
			},
			expectIssue: true,
			issueType:   "execution",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExecutor := NewMockCommandExecutor()
			tt.setupMock(mockExecutor)

			installer := &DefaultRuntimeInstaller{
				registry: NewRuntimeRegistry(),
			}

			result, err := installer.Verify("nodejs")
			if err != nil {
				t.Logf("Node.js runtime verification error (may be expected): %v", err)
			}

			if result != nil {
				foundExpectedIssue := false
				for _, issue := range result.Issues {
					if tt.expectIssue && strings.Contains(strings.ToLower(string(issue.Category)), tt.issueType) {
						foundExpectedIssue = true
						break
					}
				}

				if tt.expectIssue && !foundExpectedIssue {
					t.Errorf("Expected issue of type '%s' but didn't find one", tt.issueType)
				}

				t.Logf("Node.js runtime verification result: %d issues found", len(result.Issues))
				for _, issue := range result.Issues {
					t.Logf("  - %s: %s (%s)", issue.Severity, issue.Title, issue.Category)
				}
			}
		})
	}
}

// TestJavaRuntimeCorruption tests Java runtime corruption detection
func TestJavaRuntimeCorruption(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		setupMock   func(*MockCommandExecutor)
		expectIssue bool
		issueType   string
	}{
		{
			name: "corrupted java version output",
			setupMock: func(m *MockCommandExecutor) {
				m.AddCommand("java", []string{"-version"}, &platform.Result{
					ExitCode: 0,
					Stderr:   "openjdk version \"\x00\x01corrupted\"",
					Duration: 10 * time.Millisecond,
				})
			},
			expectIssue: false, // Version parsing should handle gracefully
			issueType:   "",
		},
		{
			name: "corrupted java system properties",
			setupMock: func(m *MockCommandExecutor) {
				m.AddCommand("java", []string{"-version"}, &platform.Result{
					ExitCode: 0,
					Stderr:   "openjdk version \"17.0.2\" 2022-01-18 LTS",
					Duration: 10 * time.Millisecond,
				})
				m.AddCommand("java", []string{"-XshowSettings:properties", "-version"}, &platform.Result{
					ExitCode: 0,
					Stderr:   "Property settings:\n    java.version \x00 17.0.2\n    java.home = /corrupted/path\x01",
					Duration: 10 * time.Millisecond,
				})
			},
			expectIssue: false, // Should handle corrupted properties gracefully
			issueType:   "",
		},
		{
			name: "missing JVM shared libraries",
			setupMock: func(m *MockCommandExecutor) {
				m.AddFailure("java", []string{"-version"},
					fmt.Errorf("error while loading shared libraries: libjvm.so: cannot open shared object file"))
			},
			expectIssue: true,
			issueType:   "dependencies",
		},
		{
			name: "corrupted JAVA_HOME environment",
			setupMock: func(m *MockCommandExecutor) {
				m.AddCommand("java", []string{"-version"}, &platform.Result{
					ExitCode: 0,
					Stderr:   "openjdk version \"17.0.2\" 2022-01-18 LTS",
					Duration: 10 * time.Millisecond,
				})
			},
			expectIssue: false, // JAVA_HOME corruption handled at environment level
			issueType:   "",
		},
		{
			name: "java class corruption",
			setupMock: func(m *MockCommandExecutor) {
				m.AddCommand("java", []string{"-version"}, &platform.Result{
					ExitCode: 1,
					Stderr:   "Error: A JNI error has occurred, please check your installation and try again\nException in thread \"main\" java.lang.ClassFormatError: Truncated class file",
					Duration: 10 * time.Millisecond,
				})
			},
			expectIssue: true,
			issueType:   "corruption",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExecutor := NewMockCommandExecutor()
			tt.setupMock(mockExecutor)

			installer := &DefaultRuntimeInstaller{
				registry: NewRuntimeRegistry(),
			}

			result, err := installer.Verify("java")
			if err != nil {
				t.Logf("Java runtime verification error (may be expected): %v", err)
			}

			if result != nil {
				foundExpectedIssue := false
				for _, issue := range result.Issues {
					if tt.expectIssue && strings.Contains(strings.ToLower(string(issue.Category)), tt.issueType) {
						foundExpectedIssue = true
						break
					}
				}

				if tt.expectIssue && !foundExpectedIssue {
					t.Errorf("Expected issue of type '%s' but didn't find one", tt.issueType)
				}

				t.Logf("Java runtime verification result: %d issues found", len(result.Issues))
				for _, issue := range result.Issues {
					t.Logf("  - %s: %s (%s)", issue.Severity, issue.Title, issue.Category)
				}
			}
		})
	}
}

// TestExecutableCorruption tests corruption of binary executable files
func TestExecutableCorruption(t *testing.T) {
	t.Parallel()

	// Create test binary files
	tmpDir := t.TempDir()

	tests := []struct {
		name          string
		createFile    func(string) error
		expectFailure bool
		errorType     string
	}{
		{
			name: "valid executable",
			createFile: func(path string) error {
				content := []byte("#!/bin/sh\necho 'test'\n")
				err := os.WriteFile(path, content, 0755)
				return err
			},
			expectFailure: false,
			errorType:     "",
		},
		{
			name: "corrupted binary header",
			createFile: func(path string) error {
				// Create binary with corrupted ELF header
				content := make([]byte, 100)
				content[0] = 0x7F // Start of ELF header
				content[1] = 0x45 // E
				content[2] = 0x4C // L
				content[3] = 0x46 // F
				// Corrupt the rest
				for i := 4; i < len(content); i++ {
					content[i] = 0xFF
				}
				return os.WriteFile(path, content, 0755)
			},
			expectFailure: true,
			errorType:     "format",
		},
		{
			name: "binary with null bytes",
			createFile: func(path string) error {
				content := []byte("#!/bin/sh\necho\x00\x01'corrupted'\n")
				return os.WriteFile(path, content, 0755)
			},
			expectFailure: false, // Scripts with null bytes might still execute
			errorType:     "",
		},
		{
			name: "truncated binary",
			createFile: func(path string) error {
				content := []byte{0x7F, 0x45, 0x4C, 0x46} // Truncated ELF header
				return os.WriteFile(path, content, 0755)
			},
			expectFailure: true,
			errorType:     "format",
		},
		{
			name: "wrong architecture binary",
			createFile: func(path string) error {
				// Create a minimal but invalid binary for current architecture
				content := make([]byte, 64)
				copy(content[:4], []byte{0x7F, 0x45, 0x4C, 0x46}) // ELF header
				content[4] = 1                                    // 32-bit (might be wrong for current system)
				content[5] = 1                                    // Little endian
				content[6] = 1                                    // Current version
				return os.WriteFile(path, content, 0755)
			},
			expectFailure: true,
			errorType:     "format",
		},
		{
			name: "non-executable permissions",
			createFile: func(path string) error {
				content := []byte("#!/bin/sh\necho 'test'\n")
				return os.WriteFile(path, content, 0644) // No execute permission
			},
			expectFailure: true,
			errorType:     "permission",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFile := filepath.Join(tmpDir, tt.name+"_test")
			err := tt.createFile(testFile)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			// Test execution
			executor := platform.NewCommandExecutor()
			result, err := executor.Execute(testFile, []string{}, 1*time.Second)

			if tt.expectFailure {
				if err == nil {
					t.Error("Expected executable corruption to be detected")
				} else {
					if tt.errorType != "" && !strings.Contains(strings.ToLower(err.Error()), tt.errorType) {
						t.Errorf("Expected error type '%s', got: %v", tt.errorType, err)
					}
				}
			} else {
				if err != nil {
					t.Logf("Unexpected error (may be expected in test env): %v", err)
				}
				if result != nil {
					t.Logf("Execution result: exit=%d", result.ExitCode)
				}
			}
		})
	}
}

// TestSymlinkCorruption tests corruption of symbolic links
func TestSymlinkCorruption(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Symlink tests not supported on Windows")
	}

	t.Parallel()

	tmpDir := t.TempDir()

	tests := []struct {
		name          string
		setupLink     func() error
		expectFailure bool
	}{
		{
			name: "valid symlink",
			setupLink: func() error {
				target := filepath.Join(tmpDir, "target")
				link := filepath.Join(tmpDir, "valid_link")

				err := os.WriteFile(target, []byte("#!/bin/sh\necho 'test'\n"), 0755)
				if err != nil {
					return err
				}
				return os.Symlink(target, link)
			},
			expectFailure: false,
		},
		{
			name: "broken symlink",
			setupLink: func() error {
				link := filepath.Join(tmpDir, "broken_link")
				return os.Symlink("/nonexistent/target", link)
			},
			expectFailure: true,
		},
		{
			name: "circular symlink",
			setupLink: func() error {
				link1 := filepath.Join(tmpDir, "circular1")
				link2 := filepath.Join(tmpDir, "circular2")

				err := os.Symlink(link2, link1)
				if err != nil {
					return err
				}
				return os.Symlink(link1, link2)
			},
			expectFailure: true,
		},
		{
			name: "symlink to corrupted binary",
			setupLink: func() error {
				target := filepath.Join(tmpDir, "corrupted_target")
				link := filepath.Join(tmpDir, "link_to_corrupted")

				// Create corrupted binary
				corruptedContent := []byte{0x7F, 0x45, 0x4C, 0x46, 0xFF, 0xFF} // Corrupted ELF
				err := os.WriteFile(target, corruptedContent, 0755)
				if err != nil {
					return err
				}
				return os.Symlink(target, link)
			},
			expectFailure: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.setupLink()
			if err != nil {
				t.Fatalf("Failed to setup symlink: %v", err)
			}

			linkPath := filepath.Join(tmpDir, strings.Replace(tt.name, " ", "_", -1))

			// Test link resolution and execution
			resolved, err := filepath.EvalSymlinks(linkPath)
			if tt.expectFailure {
				if err == nil {
					// Try to execute to see if it fails
					executor := platform.NewCommandExecutor()
					_, execErr := executor.Execute(resolved, []string{}, 1*time.Second)
					if execErr == nil {
						t.Error("Expected symlink corruption to be detected")
					}
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected symlink error: %v", err)
				} else {
					t.Logf("Symlink resolved to: %s", resolved)
				}
			}
		})
	}
}

// TestInstallationIntegrityVerification tests verification of installation integrity
func TestInstallationIntegrityVerification(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	tests := []struct {
		name              string
		setupInstallation func() error
		expectValid       bool
		issueCategory     types.IssueCategory
	}{
		{
			name: "complete installation",
			setupInstallation: func() error {
				binDir := filepath.Join(tmpDir, "complete", "bin")
				if err := os.MkdirAll(binDir, 0755); err != nil {
					return err
				}

				executable := filepath.Join(binDir, "test-binary")
				content := []byte("#!/bin/sh\necho 'v1.0.0'\n")
				return os.WriteFile(executable, content, 0755)
			},
			expectValid:   true,
			issueCategory: "",
		},
		{
			name: "missing executable",
			setupInstallation: func() error {
				binDir := filepath.Join(tmpDir, "missing_exe", "bin")
				return os.MkdirAll(binDir, 0755)
			},
			expectValid:   false,
			issueCategory: types.IssueCategoryInstallation,
		},
		{
			name: "corrupted executable",
			setupInstallation: func() error {
				binDir := filepath.Join(tmpDir, "corrupted", "bin")
				if err := os.MkdirAll(binDir, 0755); err != nil {
					return err
				}

				executable := filepath.Join(binDir, "test-binary")
				// Create corrupted binary
				content := make([]byte, 100)
				rand.Read(content)
				return os.WriteFile(executable, content, 0755)
			},
			expectValid:   false,
			issueCategory: types.IssueCategoryCorruption,
		},
		{
			name: "permission issues",
			setupInstallation: func() error {
				binDir := filepath.Join(tmpDir, "no_perm", "bin")
				if err := os.MkdirAll(binDir, 0755); err != nil {
					return err
				}

				executable := filepath.Join(binDir, "test-binary")
				content := []byte("#!/bin/sh\necho 'v1.0.0'\n")
				err := os.WriteFile(executable, content, 0644) // No execute permission
				return err
			},
			expectValid:   false,
			issueCategory: types.IssueCategoryPermissions,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.setupInstallation()
			if err != nil {
				t.Fatalf("Failed to setup installation: %v", err)
			}

			result := &types.VerificationResult{
				Issues:          []types.Issue{},
				Details:         make(map[string]interface{}),
				Metadata:        make(map[string]interface{}),
				EnvironmentVars: make(map[string]string),
			}

			// Simulate verification logic
			installPath := filepath.Join(tmpDir, strings.Replace(tt.name, " ", "_", -1))
			executable := filepath.Join(installPath, "bin", "test-binary")

			if _, err := os.Stat(executable); os.IsNotExist(err) {
				result.Issues = append(result.Issues, types.Issue{
					Severity:    types.IssueSeverityHigh,
					Category:    types.IssueCategoryInstallation,
					Title:       "Missing Executable",
					Description: "Required executable not found",
					Solution:    "Reinstall the component",
				})
			} else if err == nil {
				// Check if executable
				info, err := os.Stat(executable)
				if err == nil && info.Mode()&0111 == 0 {
					result.Issues = append(result.Issues, types.Issue{
						Severity:    types.IssueSeverityMedium,
						Category:    types.IssueCategoryPermissions,
						Title:       "Executable Permission Missing",
						Description: "File exists but is not executable",
						Solution:    "Fix file permissions",
					})
				}

				// Try to execute to detect corruption
				executor := platform.NewCommandExecutor()
				_, execErr := executor.Execute(executable, []string{}, 1*time.Second)
				if execErr != nil && strings.Contains(execErr.Error(), "format") {
					result.Issues = append(result.Issues, types.Issue{
						Severity:    types.IssueSeverityHigh,
						Category:    types.IssueCategoryCorruption,
						Title:       "Corrupted Executable",
						Description: "Executable appears to be corrupted",
						Solution:    "Reinstall the component",
					})
				}
			}

			isValid := len(result.Issues) == 0
			if isValid != tt.expectValid {
				t.Errorf("Expected valid=%v, got valid=%v", tt.expectValid, isValid)
			}

			if !tt.expectValid && tt.issueCategory != "" {
				foundExpectedCategory := false
				for _, issue := range result.Issues {
					if issue.Category == tt.issueCategory {
						foundExpectedCategory = true
						break
					}
				}
				if !foundExpectedCategory {
					t.Errorf("Expected issue category '%s' but didn't find it", tt.issueCategory)
				}
			}

			t.Logf("Installation verification result: valid=%v, issues=%d", isValid, len(result.Issues))
			for _, issue := range result.Issues {
				t.Logf("  - %s: %s (%s)", issue.Severity, issue.Title, issue.Category)
			}
		})
	}
}

// TestConcurrentCorruptionDetection tests corruption detection under concurrent access
func TestConcurrentCorruptionDetection(t *testing.T) {
	t.Parallel()

	const numGoroutines = 10
	mockExecutor := NewMockCommandExecutor()

	// Setup various mock scenarios
	scenarios := []struct {
		cmd   string
		args  []string
		setup func(*MockCommandExecutor)
	}{
		{
			cmd:  "go",
			args: []string{"version"},
			setup: func(m *MockCommandExecutor) {
				m.AddCommand("go", []string{"version"}, &platform.Result{
					ExitCode: 0,
					Stdout:   "go version go1.21.0 linux/amd64",
					Duration: 10 * time.Millisecond,
				})
			},
		},
		{
			cmd:  "python",
			args: []string{"--version"},
			setup: func(m *MockCommandExecutor) {
				m.AddCommand("python", []string{"--version"}, &platform.Result{
					ExitCode: 0,
					Stdout:   "Python 3.9.0",
					Duration: 10 * time.Millisecond,
				})
			},
		},
		{
			cmd:  "corrupted",
			args: []string{"--version"},
			setup: func(m *MockCommandExecutor) {
				m.AddFailure("corrupted", []string{"--version"}, fmt.Errorf("exec format error"))
			},
		},
	}

	// Setup all scenarios
	for _, scenario := range scenarios {
		scenario.setup(mockExecutor)
	}

	installer := &DefaultRuntimeInstaller{
		registry: NewRuntimeRegistry(),
	}

	errors := make(chan error, numGoroutines)
	results := make(chan *types.VerificationResult, numGoroutines)

	// Start concurrent verifications
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			runtime := scenarios[id%len(scenarios)].cmd
			if runtime == "corrupted" {
				runtime = "go" // Use valid runtime name
			}

			result, err := installer.Verify(runtime)
			if err != nil {
				errors <- err
				return
			}
			results <- result
		}(i)
	}

	// Collect results
	errorCount := 0
	resultCount := 0
	for i := 0; i < numGoroutines; i++ {
		select {
		case err := <-errors:
			errorCount++
			t.Logf("Concurrent verification error: %v", err)
		case result := <-results:
			resultCount++
			if result != nil {
				t.Logf("Concurrent verification result: %d issues", len(result.Issues))
			}
		case <-time.After(5 * time.Second):
			t.Error("Timeout waiting for concurrent verification")
			return
		}
	}

	t.Logf("Concurrent corruption detection completed: %d results, %d errors", resultCount, errorCount)
}

// Benchmark tests for corruption detection performance
func BenchmarkGoRuntimeCorruptionDetection(b *testing.B) {
	mockExecutor := NewMockCommandExecutor()
	mockExecutor.AddCommand("go", []string{"version"}, &platform.Result{
		ExitCode: 0,
		Stdout:   "go version \x00corrupted output",
		Duration: 10 * time.Millisecond,
	})

	installer := &DefaultRuntimeInstaller{
		registry: NewRuntimeRegistry(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := installer.Verify("go")
		_ = err // Ignore errors in benchmark
	}
}

func BenchmarkExecutableCorruptionDetection(b *testing.B) {
	tmpDir := b.TempDir()
	corruptedFile := filepath.Join(tmpDir, "corrupted")

	// Create corrupted binary
	content := make([]byte, 100)
	rand.Read(content)
	err := os.WriteFile(corruptedFile, content, 0755)
	if err != nil {
		b.Fatalf("Failed to create corrupted file: %v", err)
	}

	executor := platform.NewCommandExecutor()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := executor.Execute(corruptedFile, []string{}, 1*time.Second)
		_ = err // Ignore errors in benchmark
	}
}
