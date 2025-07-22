package setup

import (
	"context"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"lsp-gateway/internal/platform"
)

type mockJavaCommandExecutor struct {
	commands  map[string]*platform.Result
	available map[string]bool
}

func newMockJavaCommandExecutor() *mockJavaCommandExecutor {
	return &mockJavaCommandExecutor{
		commands:  make(map[string]*platform.Result),
		available: make(map[string]bool),
	}
}

func (m *mockJavaCommandExecutor) Execute(cmd string, args []string, timeout time.Duration) (*platform.Result, error) {
	key := cmd
	if len(args) > 0 {
		key += " " + strings.Join(args, " ")
	}

	if result, exists := m.commands[key]; exists {
		return result, nil
	}

	return &platform.Result{
		ExitCode: 127,
		Stderr:   "command not found",
	}, nil
}

func (m *mockJavaCommandExecutor) ExecuteWithEnv(cmd string, args []string, env map[string]string, timeout time.Duration) (*platform.Result, error) {
	return m.Execute(cmd, args, timeout)
}

func (m *mockJavaCommandExecutor) GetShell() string {
	if runtime.GOOS == "windows" {
		return "cmd"
	}
	return "bash"
}

func (m *mockJavaCommandExecutor) GetShellArgs(command string) []string {
	if runtime.GOOS == "windows" {
		return []string{"/C", command}
	}
	return []string{"-c", command}
}

func (m *mockJavaCommandExecutor) IsCommandAvailable(command string) bool {
	available, exists := m.available[command]
	return exists && available
}

func (m *mockJavaCommandExecutor) setCommandAvailable(command string, available bool) {
	m.available[command] = available
}

func (m *mockJavaCommandExecutor) setCommandResult(command string, result *platform.Result) {
	m.commands[command] = result
}

func TestJavaDetector_DetectJava_NotInstalled(t *testing.T) {
	mockExecutor := newMockJavaCommandExecutor()
	mockExecutor.setCommandAvailable("java", false)

	detector := &JavaDetector{
		executor:       mockExecutor,
		versionChecker: NewVersionChecker(),
	}

	javaInfo, err := detector.DetectJava()
	if err != nil {
		t.Fatalf("DetectJava() error = %v, want nil", err)
	}

	if javaInfo.Installed {
		t.Error("Expected Java to not be installed")
	}

	if len(javaInfo.Issues) == 0 {
		t.Error("Expected issues to be reported when Java is not installed")
	}

	expectedIssue := "Java runtime not found in PATH"
	found := false
	for _, issue := range javaInfo.Issues {
		if strings.Contains(issue, expectedIssue) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected issue containing '%s', got issues: %v", expectedIssue, javaInfo.Issues)
	}
}

func TestJavaDetector_DetectJava_OpenJDK17(t *testing.T) {
	mockExecutor := newMockJavaCommandExecutor()
	mockExecutor.setCommandAvailable("java", true)
	mockExecutor.setCommandAvailable("javac", true)
	mockExecutor.setCommandAvailable("jar", true)

	javaVersionOutput := `openjdk version "17.0.2" 2022-01-18
OpenJDK Runtime Environment (build 17.0.2+8-Ubuntu-120.04)
OpenJDK 64-Bit Server VM (build 17.0.2+8-Ubuntu-120.04, mixed mode, sharing)`

	mockExecutor.setCommandResult("java -version", &platform.Result{
		ExitCode: 0,
		Stderr:   javaVersionOutput,
	})

	mockExecutor.setCommandResult("which java", &platform.Result{
		ExitCode: 0,
		Stdout:   "/usr/bin/java",
	})

	mockExecutor.setCommandResult("javac -version", &platform.Result{
		ExitCode: 0,
		Stderr:   "javac 17.0.2",
	})

	mockExecutor.setCommandResult("jar --version", &platform.Result{
		ExitCode: 0,
		Stdout:   "jar 17.0.2",
	})

	mockExecutor.setCommandResult("java -cp . -version", &platform.Result{
		ExitCode: 0,
		Stderr:   javaVersionOutput,
	})

	detector := &JavaDetector{
		executor:       mockExecutor,
		versionChecker: NewVersionChecker(),
	}

	javaInfo, err := detector.DetectJava()
	if err != nil {
		t.Fatalf("DetectJava() error = %v, want nil", err)
	}

	if !javaInfo.Installed {
		t.Error("Expected Java to be installed")
	}

	if javaInfo.Version != "17.0.2" {
		t.Errorf("Expected version '17.0.2', got '%s'", javaInfo.Version)
	}

	if javaInfo.Distribution != "OpenJDK" {
		t.Errorf("Expected distribution 'OpenJDK', got '%s'", javaInfo.Distribution)
	}

	if !javaInfo.Compatible {
		t.Error("Expected Java 17.0.2 to be compatible (min version 17.0.0)")
	}

	// Check that path is valid but allow flexibility for different system configurations
	if javaInfo.Path == "" {
		t.Error("Expected non-empty Java path")
	} else if !strings.Contains(javaInfo.Path, "java") {
		t.Errorf("Expected path to contain 'java', got '%s'", javaInfo.Path)
	}

	if !javaInfo.IsJDK {
		t.Error("Expected JDK to be detected (javac available)")
	}

	expectedTools := []string{"javac", "jar"}
	for _, tool := range expectedTools {
		found := false
		for _, foundTool := range javaInfo.DevToolsFound {
			if foundTool == tool {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected development tool '%s' to be found", tool)
		}
	}
}

func TestJavaDetector_DetectJava_OracleJDK8(t *testing.T) {
	mockExecutor := newMockJavaCommandExecutor()
	mockExecutor.setCommandAvailable("java", true)
	mockExecutor.setCommandAvailable("javac", true)

	javaVersionOutput := `java version "1.8.0_333"
Java(TM) SE Runtime Environment (build 1.8.0_333-b02)
Java HotSpot(TM) 64-Bit Server VM (build 25.333-b02, mixed mode)`

	mockExecutor.setCommandResult("java -version", &platform.Result{
		ExitCode: 0,
		Stderr:   javaVersionOutput,
	})

	mockExecutor.setCommandResult("which java", &platform.Result{
		ExitCode: 0,
		Stdout:   "/usr/bin/java",
	})

	mockExecutor.setCommandResult("javac -version", &platform.Result{
		ExitCode: 0,
		Stderr:   "javac 1.8.0_333",
	})

	detector := &JavaDetector{
		executor:       mockExecutor,
		versionChecker: NewVersionChecker(),
	}

	javaInfo, err := detector.DetectJava()
	if err != nil {
		t.Fatalf("DetectJava() error = %v, want nil", err)
	}

	if javaInfo.Version != "8.0.333" {
		t.Errorf("Expected normalized version '8.0.333', got '%s'", javaInfo.Version)
	}

	if javaInfo.Distribution != "Oracle JDK" {
		t.Errorf("Expected distribution 'Oracle JDK', got '%s'", javaInfo.Distribution)
	}

	if javaInfo.Compatible {
		t.Error("Expected Java 8.0.333 to not be compatible (min version 17.0.0)")
	}

	found := false
	for _, issue := range javaInfo.Issues {
		if strings.Contains(issue, "does not meet minimum requirement") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected compatibility issue to be reported")
	}
}

func TestJavaDetector_DetectJava_JREOnly(t *testing.T) {
	mockExecutor := newMockJavaCommandExecutor()
	mockExecutor.setCommandAvailable("java", true)
	mockExecutor.setCommandAvailable("javac", false) // No javac = JRE only

	javaVersionOutput := `openjdk version "17.0.2" 2022-01-18
OpenJDK Runtime Environment (build 17.0.2+8-Ubuntu-120.04)
OpenJDK 64-Bit Server VM (build 17.0.2+8-Ubuntu-120.04, mixed mode, sharing)`

	mockExecutor.setCommandResult("java -version", &platform.Result{
		ExitCode: 0,
		Stderr:   javaVersionOutput,
	})

	mockExecutor.setCommandResult("which java", &platform.Result{
		ExitCode: 0,
		Stdout:   "/usr/bin/java",
	})

	detector := &JavaDetector{
		executor:       mockExecutor,
		versionChecker: NewVersionChecker(),
	}

	javaInfo, err := detector.DetectJava()
	if err != nil {
		t.Fatalf("DetectJava() error = %v, want nil", err)
	}

	if !javaInfo.Installed {
		t.Error("Expected Java to be installed")
	}

	if !javaInfo.Compatible {
		t.Error("Expected Java 17.0.2 to be compatible")
	}

	if javaInfo.IsJDK {
		t.Error("Expected JRE to be detected (not JDK)")
	}

	found := false
	for _, issue := range javaInfo.Issues {
		if strings.Contains(issue, "JRE detected but JDK required") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected JDK requirement issue to be reported")
	}
}

func TestJavaDetector_ParseJavaVersionOutput(t *testing.T) {
	detector := &JavaDetector{}

	testCases := []struct {
		name         string
		output       string
		expectedVer  string
		expectedDist string
	}{
		{
			name: "OpenJDK 17",
			output: `openjdk version "17.0.2" 2022-01-18
OpenJDK Runtime Environment (build 17.0.2+8-Ubuntu-120.04)`,
			expectedVer:  "17.0.2",
			expectedDist: "OpenJDK",
		},
		{
			name: "Oracle JDK 8",
			output: `java version "1.8.0_333"
Java(TM) SE Runtime Environment (build 1.8.0_333-b02)`,
			expectedVer:  "8.0.333",
			expectedDist: "Oracle JDK",
		},
		{
			name: "Eclipse Temurin",
			output: `openjdk version "11.0.15" 2022-04-19
OpenJDK Runtime Environment Temurin-11.0.15+10 (build 11.0.15+10)`,
			expectedVer:  "11.0.15",
			expectedDist: "Eclipse Temurin",
		},
		{
			name: "Amazon Corretto",
			output: `openjdk version "17.0.3" 2022-04-19 LTS
OpenJDK Runtime Environment Corretto-17.0.3.6.1 (build 17.0.3+6-LTS)`,
			expectedVer:  "17.0.3",
			expectedDist: "Amazon Corretto",
		},
		{
			name: "GraalVM",
			output: `openjdk version "17.0.3" 2022-04-19
OpenJDK Runtime Environment GraalVM CE 22.1.0 (build 17.0.3+7-jvmci-22.1-b06)`,
			expectedVer:  "17.0.3",
			expectedDist: "GraalVM",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			version, distribution := detector.parseJavaVersionOutput(tc.output)

			if version != tc.expectedVer {
				t.Errorf("Expected version '%s', got '%s'", tc.expectedVer, version)
			}

			if distribution != tc.expectedDist {
				t.Errorf("Expected distribution '%s', got '%s'", tc.expectedDist, distribution)
			}
		})
	}
}

func TestJavaDetector_ValidateJavaHome(t *testing.T) {
	originalJavaHome := os.Getenv("JAVA_HOME")
	defer func() {
		if err := os.Setenv("JAVA_HOME", originalJavaHome); err != nil {
			t.Logf("Failed to restore JAVA_HOME: %v", err)
		}
	}()

	if err := os.Unsetenv("JAVA_HOME"); err != nil {
		t.Logf("Failed to unset JAVA_HOME: %v", err)
	}

	mockExecutor := newMockJavaCommandExecutor()
	detector := &JavaDetector{
		executor:       mockExecutor,
		versionChecker: NewVersionChecker(),
	}

	javaInfo := &JavaRuntimeInfo{
		RuntimeInfo: &RuntimeInfo{Issues: []string{}},
	}

	detector.validateJavaHome(javaInfo)

	if javaInfo.JavaHomeValid {
		t.Error("Expected JAVA_HOME to be invalid when not set")
	}

	found := false
	for _, issue := range javaInfo.Issues {
		if strings.Contains(issue, "JAVA_HOME environment variable is not set") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected JAVA_HOME not set issue to be reported")
	}
}

func TestJavaDetector_ExtractJavacVersion(t *testing.T) {
	detector := &JavaDetector{}

	testCases := []struct {
		name     string
		output   string
		expected string
	}{
		{
			name:     "Modern javac",
			output:   "javac 17.0.2",
			expected: "17.0.2",
		},
		{
			name:     "Old javac format",
			output:   "javac 1.8.0_333",
			expected: "8.0.333",
		},
		{
			name:     "No version",
			output:   "invalid output",
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := detector.extractJavacVersion(tc.output)
			if result != tc.expected {
				t.Errorf("Expected '%s', got '%s'", tc.expected, result)
			}
		})
	}
}

func TestJavaDetector_Integration_DefaultDetector(t *testing.T) {
	detector := NewRuntimeDetector()

	runtimeInfo, err := detector.DetectJava(context.Background())
	if err != nil {
		t.Fatalf("DetectJava() error = %v, want nil", err)
	}

	if runtimeInfo.Name != "java" {
		t.Errorf("Expected name 'java', got '%s'", runtimeInfo.Name)
	}

	if runtimeInfo.Installed && runtimeInfo.Version == "" {
		t.Logf("Java is installed but version could not be determined. This may be expected on some systems.")
		t.Logf("Runtime info: Path=%s, Compatible=%v", runtimeInfo.Path, runtimeInfo.Compatible)
	}

	if runtimeInfo.Installed && runtimeInfo.Path == "" {
		t.Error("If Java is installed, path should not be empty")
	}
}
