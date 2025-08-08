package shared

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"lsp-gateway/src/utils"
)

// WorkspaceSetup holds the workspace configuration for tests
type WorkspaceSetup struct {
	TempDir      string
	ProjectDir   string
	OriginalWD   string
	TestFiles    map[string]string
	CreatedFiles []string
}

// CreateTestWorkspace creates a temporary workspace for tests
func CreateTestWorkspace(t *testing.T) *WorkspaceSetup {
	tmpDir, err := os.MkdirTemp("", "lsp-gateway-test-*")
	require.NoError(t, err, "Failed to create temp dir")

	originalWD, err := os.Getwd()
	require.NoError(t, err, "Failed to get current directory")

	return &WorkspaceSetup{
		TempDir:    tmpDir,
		ProjectDir: tmpDir,
		OriginalWD: originalWD,
		TestFiles:  make(map[string]string),
	}
}

// CreateProjectSubdir creates a subdirectory within the workspace
func (w *WorkspaceSetup) CreateProjectSubdir(t *testing.T, name string) string {
	projectDir := filepath.Join(w.TempDir, name)
	err := os.MkdirAll(projectDir, 0755)
	require.NoError(t, err, "Failed to create project directory")
	w.ProjectDir = projectDir
	return projectDir
}

// WriteGoFile writes a Go source file to the workspace
func (w *WorkspaceSetup) WriteGoFile(t *testing.T, filename, content string) string {
	filePath := filepath.Join(w.ProjectDir, filename)
	err := os.WriteFile(filePath, []byte(content), 0644)
	require.NoError(t, err, "Failed to write Go file: %s", filename)

	w.TestFiles[filename] = filePath
	w.CreatedFiles = append(w.CreatedFiles, filePath)
	return filePath
}

// WriteGoMod writes a go.mod file to enable Go language detection
func (w *WorkspaceSetup) WriteGoMod(t *testing.T, moduleName string) string {
	content := "module " + moduleName + "\n\ngo 1.21\n"
	return w.WriteGoFile(t, "go.mod", content)
}

// WriteMainGoFile writes a standard main.go file with common patterns
func (w *WorkspaceSetup) WriteMainGoFile(t *testing.T) string {
	content := `package main

import "fmt"

type Router struct {
	routes map[string]string
}

func NewRouter() *Router {
	return &Router{
		routes: make(map[string]string),
	}
}

func (r *Router) Handle(path string) {
	_ = path
}

func main() {
	r := NewRouter()
	r.Handle("/home")
}
`
	return w.WriteGoFile(t, "main.go", content)
}

// WriteServerGoFile writes a server.go file with HTTP server patterns
func (w *WorkspaceSetup) WriteServerGoFile(t *testing.T) string {
	content := `package main

import (
	"fmt"
	"net/http"
)

type Server struct {
	port int
	handler http.Handler
}

func NewServer(port int) *Server {
	return &Server{
		port: port,
	}
}

func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.port)
	return http.ListenAndServe(addr, s.handler)
}

func main() {
	server := NewServer(8080)
	server.Start()
}
`
	return w.WriteGoFile(t, "server.go", content)
}

// WriteTestStructGoFile writes a test Go file with struct definitions
func (w *WorkspaceSetup) WriteTestStructGoFile(t *testing.T) string {
	content := `package main

type TestStruct struct {
	Name string
	ID   int
}

func NewTestStruct(name string) *TestStruct {
	return &TestStruct{
		Name: name,
		ID:   1,
	}
}

func (ts *TestStruct) GetName() string {
	return ts.Name
}

func TestFunction() {
	ts := NewTestStruct("test")
	_ = ts.GetName()
}
`
	return w.WriteGoFile(t, "test_struct.go", content)
}

// WritePythonFile writes a Python source file to the workspace
func (w *WorkspaceSetup) WritePythonFile(t *testing.T, filename, content string) string {
	if !strings.HasSuffix(filename, ".py") {
		filename = filename + ".py"
	}
	filePath := filepath.Join(w.ProjectDir, filename)
	err := os.WriteFile(filePath, []byte(content), 0644)
	require.NoError(t, err, "Failed to write Python file: %s", filename)

	w.TestFiles[filename] = filePath
	w.CreatedFiles = append(w.CreatedFiles, filePath)
	return filePath
}

// WriteJavaScriptFile writes a JavaScript source file to the workspace
func (w *WorkspaceSetup) WriteJavaScriptFile(t *testing.T, filename, content string) string {
	if !strings.HasSuffix(filename, ".js") {
		filename = filename + ".js"
	}
	filePath := filepath.Join(w.ProjectDir, filename)
	err := os.WriteFile(filePath, []byte(content), 0644)
	require.NoError(t, err, "Failed to write JavaScript file: %s", filename)

	w.TestFiles[filename] = filePath
	w.CreatedFiles = append(w.CreatedFiles, filePath)
	return filePath
}

// ChangeToWorkspace changes the current directory to the workspace
func (w *WorkspaceSetup) ChangeToWorkspace(t *testing.T) {
	err := os.Chdir(w.ProjectDir)
	require.NoError(t, err, "Failed to change to workspace directory")
}

// RestoreWorkingDirectory restores the original working directory
func (w *WorkspaceSetup) RestoreWorkingDirectory() error {
	return os.Chdir(w.OriginalWD)
}

// GetFileURI returns the file URI for a given filename
func (w *WorkspaceSetup) GetFileURI(filename string) string {
	if filepath, exists := w.TestFiles[filename]; exists {
		return utils.FilePathToURI(filepath)
	}
	// Fallback to constructing URI from project dir
	fullPath := filepath.Join(w.ProjectDir, filename)
	return utils.FilePathToURI(fullPath)
}

// GetFilePath returns the full file path for a given filename
func (w *WorkspaceSetup) GetFilePath(filename string) string {
	if filepath, exists := w.TestFiles[filename]; exists {
		return filepath
	}
	return filepath.Join(w.ProjectDir, filename)
}

// Cleanup removes the temporary workspace
func (w *WorkspaceSetup) Cleanup() {
	if w.OriginalWD != "" {
		os.Chdir(w.OriginalWD)
	}
	if w.TempDir != "" {
		os.RemoveAll(w.TempDir)
	}
}

// CreateBasicGoProject creates a basic Go project with main.go and go.mod
func CreateBasicGoProject(t *testing.T, moduleName string) *WorkspaceSetup {
	workspace := CreateTestWorkspace(t)
	workspace.WriteGoMod(t, moduleName)
	workspace.WriteMainGoFile(t)
	workspace.ChangeToWorkspace(t)
	return workspace
}

// CreateGoProjectWithServer creates a Go project with HTTP server code
func CreateGoProjectWithServer(t *testing.T, moduleName string) *WorkspaceSetup {
	workspace := CreateTestWorkspace(t)
	workspace.WriteGoMod(t, moduleName)
	workspace.WriteServerGoFile(t)
	workspace.ChangeToWorkspace(t)
	return workspace
}

// CreateMultiFileGoProject creates a Go project with multiple source files
func CreateMultiFileGoProject(t *testing.T, moduleName string) *WorkspaceSetup {
	workspace := CreateTestWorkspace(t)
	workspace.WriteGoMod(t, moduleName)
	workspace.WriteMainGoFile(t)
	workspace.WriteServerGoFile(t)
	workspace.WriteTestStructGoFile(t)
	workspace.ChangeToWorkspace(t)
	return workspace
}
