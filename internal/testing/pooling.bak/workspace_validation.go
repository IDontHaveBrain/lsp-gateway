package pooling

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WorkspaceValidator provides validation utilities for workspace paths
type WorkspaceValidator struct {
	logger Logger
}

// NewWorkspaceValidator creates a new workspace validator
func NewWorkspaceValidator(logger Logger) *WorkspaceValidator {
	return &WorkspaceValidator{
		logger: logger,
	}
}

// ValidateWorkspace performs comprehensive validation of a workspace path
func (wv *WorkspaceValidator) ValidateWorkspace(workspacePath string) error {
	if workspacePath == "" {
		return fmt.Errorf("workspace path cannot be empty")
	}
	
	// Check if path exists
	info, err := os.Stat(workspacePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("workspace path does not exist: %s", workspacePath)
		}
		return fmt.Errorf("cannot access workspace path %s: %w", workspacePath, err)
	}
	
	// Must be a directory
	if !info.IsDir() {
		return fmt.Errorf("workspace path must be a directory: %s", workspacePath)
	}
	
	// Check if directory is readable
	if err := wv.checkDirectoryReadable(workspacePath); err != nil {
		return fmt.Errorf("workspace directory is not readable: %w", err)
	}
	
	// Validate it's a reasonable workspace (not system directories)
	if err := wv.validateWorkspaceLocation(workspacePath); err != nil {
		return fmt.Errorf("invalid workspace location: %w", err)
	}
	
	// Check for project indicators
	projectType := wv.detectProjectType(workspacePath)
	if projectType == ProjectTypeUnknown {
		wv.logger.Warn("No recognized project markers found in workspace: %s", workspacePath)
		// Don't fail validation for unknown project types - it might still be valid
	} else {
		wv.logger.Debug("Detected project type %s in workspace: %s", projectType, workspacePath)
	}
	
	return nil
}

// ProjectType represents the detected type of project in a workspace
type ProjectType string

const (
	ProjectTypeUnknown    ProjectType = "unknown"
	ProjectTypeJava       ProjectType = "java"
	ProjectTypeTypeScript ProjectType = "typescript"
	ProjectTypeJavaScript ProjectType = "javascript"
	ProjectTypeGo         ProjectType = "go"
	ProjectTypePython     ProjectType = "python"
	ProjectTypeMulti      ProjectType = "multi"
)

// detectProjectType attempts to determine the project type based on file markers
func (wv *WorkspaceValidator) detectProjectType(workspacePath string) ProjectType {
	detectedTypes := make(map[ProjectType]int)
	
	// Java markers
	javaMarkers := []string{"pom.xml", "build.gradle", "build.gradle.kts", ".project", "src/main/java"}
	for _, marker := range javaMarkers {
		if wv.fileExists(filepath.Join(workspacePath, marker)) {
			detectedTypes[ProjectTypeJava]++
		}
	}
	
	// TypeScript markers
	tsMarkers := []string{"tsconfig.json", "tsconfig.*.json"}
	for _, marker := range tsMarkers {
		if wv.fileExistsGlob(workspacePath, marker) {
			detectedTypes[ProjectTypeTypeScript]++
		}
	}
	
	// JavaScript markers
	jsMarkers := []string{"package.json", "jsconfig.json", "node_modules"}
	for _, marker := range jsMarkers {
		if wv.fileExists(filepath.Join(workspacePath, marker)) {
			detectedTypes[ProjectTypeJavaScript]++
		}
	}
	
	// Go markers
	goMarkers := []string{"go.mod", "go.sum", "main.go"}
	for _, marker := range goMarkers {
		if wv.fileExists(filepath.Join(workspacePath, marker)) {
			detectedTypes[ProjectTypeGo]++
		}
	}
	
	// Python markers
	pythonMarkers := []string{"setup.py", "pyproject.toml", "requirements.txt", "Pipfile", "__init__.py"}
	for _, marker := range pythonMarkers {
		if wv.fileExists(filepath.Join(workspacePath, marker)) {
			detectedTypes[ProjectTypePython]++
		}
	}
	
	// Check for source files if no specific markers found
	if len(detectedTypes) == 0 {
		if wv.hasSourceFiles(workspacePath, ".java") {
			detectedTypes[ProjectTypeJava]++
		}
		if wv.hasSourceFiles(workspacePath, ".ts") {
			detectedTypes[ProjectTypeTypeScript]++
		}
		if wv.hasSourceFiles(workspacePath, ".js") {
			detectedTypes[ProjectTypeJavaScript]++
		}
		if wv.hasSourceFiles(workspacePath, ".go") {
			detectedTypes[ProjectTypeGo]++
		}
		if wv.hasSourceFiles(workspacePath, ".py") {
			detectedTypes[ProjectTypePython]++
		}
	}
	
	// Determine the most likely project type
	if len(detectedTypes) == 0 {
		return ProjectTypeUnknown
	}
	
	if len(detectedTypes) > 1 {
		return ProjectTypeMulti
	}
	
	// Find the type with the highest score
	maxScore := 0
	var primaryType ProjectType = ProjectTypeUnknown
	
	for pType, score := range detectedTypes {
		if score > maxScore {
			maxScore = score
			primaryType = pType
		}
	}
	
	return primaryType
}

// checkDirectoryReadable checks if a directory can be read
func (wv *WorkspaceValidator) checkDirectoryReadable(dirPath string) error {
	// Try to read the directory
	_, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("directory is not readable: %w", err)
	}
	return nil
}

// validateWorkspaceLocation ensures the workspace is in a reasonable location
func (wv *WorkspaceValidator) validateWorkspaceLocation(workspacePath string) error {
	absPath, err := filepath.Abs(workspacePath)
	if err != nil {
		return fmt.Errorf("cannot resolve absolute path: %w", err)
	}
	
	// Normalize path separators for cross-platform compatibility
	normalizedPath := filepath.ToSlash(strings.ToLower(absPath))
	
	// List of potentially problematic system directories
	forbiddenPaths := []string{
		"/",
		"/bin",
		"/boot",
		"/dev",
		"/etc",
		"/lib",
		"/lib64",
		"/proc",
		"/root",
		"/sbin",
		"/sys",
		"/usr/bin",
		"/usr/sbin",
		"/var",
		"c:/windows",
		"c:/program files",
		"c:/program files (x86)",
	}
	
	for _, forbidden := range forbiddenPaths {
		if normalizedPath == strings.ToLower(forbidden) || 
		   strings.HasPrefix(normalizedPath, strings.ToLower(forbidden)+"/") {
			return fmt.Errorf("workspace cannot be in system directory: %s", absPath)
		}
	}
	
	// Check for common temp directories
	tempPaths := []string{"/tmp", "/var/tmp", "c:/temp", "c:/windows/temp"}
	for _, tempPath := range tempPaths {
		if strings.HasPrefix(normalizedPath, strings.ToLower(tempPath)+"/") {
			wv.logger.Warn("Workspace is in temporary directory, this may cause issues: %s", absPath)
		}
	}
	
	return nil
}

// fileExists checks if a file exists
func (wv *WorkspaceValidator) fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// fileExistsGlob checks if files matching a glob pattern exist
func (wv *WorkspaceValidator) fileExistsGlob(basePath, pattern string) bool {
	matches, err := filepath.Glob(filepath.Join(basePath, pattern))
	if err != nil {
		return false
	}
	return len(matches) > 0
}

// hasSourceFiles checks if directory contains source files with given extension
func (wv *WorkspaceValidator) hasSourceFiles(dirPath, extension string) bool {
	found := false
	
	// Walk up to 2 levels deep to find source files
	err := filepath.WalkDir(dirPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Continue walking despite errors
		}
		
		// Limit depth to avoid excessive scanning
		relPath, err := filepath.Rel(dirPath, path)
		if err == nil {
			depth := len(strings.Split(relPath, string(filepath.Separator)))
			if depth > 3 { // Skip deeply nested directories
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), extension) {
			found = true
			return fmt.Errorf("found") // Use error to break out of walk
		}
		
		return nil
	})
	
	// We use error to break out of walk, so ignore the error if we found files
	return found || (err != nil && err.Error() == "found")
}

// ValidateWorkspaceSwitch validates that a switch between two workspaces is reasonable
func (wv *WorkspaceValidator) ValidateWorkspaceSwitch(from, to string) error {
	// Validate the target workspace
	if err := wv.ValidateWorkspace(to); err != nil {
		return fmt.Errorf("target workspace validation failed: %w", err)
	}
	
	// If we have a source workspace, validate it's different from target
	if from != "" {
		fromAbs, err := filepath.Abs(from)
		if err != nil {
			wv.logger.Warn("Could not resolve absolute path for source workspace: %s", from)
		} else {
			toAbs, err := filepath.Abs(to)
			if err != nil {
				return fmt.Errorf("could not resolve absolute path for target workspace: %w", err)
			}
			
			if fromAbs == toAbs {
				return fmt.Errorf("source and target workspaces are the same: %s", toAbs)
			}
		}
	}
	
	return nil
}

// GetWorkspaceInfo returns detailed information about a workspace
func (wv *WorkspaceValidator) GetWorkspaceInfo(workspacePath string) (*WorkspaceInfo, error) {
	if err := wv.ValidateWorkspace(workspacePath); err != nil {
		return nil, fmt.Errorf("workspace validation failed: %w", err)
	}
	
	absPath, err := filepath.Abs(workspacePath)
	if err != nil {
		return nil, fmt.Errorf("could not resolve absolute path: %w", err)
	}
	
	info := &WorkspaceInfo{
		Path:         absPath,
		Name:         filepath.Base(absPath),
		ProjectType:  wv.detectProjectType(workspacePath),
		ProjectFiles: wv.getProjectFiles(workspacePath),
	}
	
	return info, nil
}

// WorkspaceInfo contains detailed information about a workspace
type WorkspaceInfo struct {
	Path         string            `json:"path"`
	Name         string            `json:"name"`
	ProjectType  ProjectType       `json:"project_type"`
	ProjectFiles map[string]bool   `json:"project_files"`
}

// getProjectFiles returns a map of important project files found in the workspace
func (wv *WorkspaceValidator) getProjectFiles(workspacePath string) map[string]bool {
	projectFiles := make(map[string]bool)
	
	// List of important project files to check for
	importantFiles := []string{
		// Java
		"pom.xml", "build.gradle", "build.gradle.kts", ".project",
		// JavaScript/TypeScript
		"package.json", "tsconfig.json", "jsconfig.json",
		// Go
		"go.mod", "go.sum",
		// Python
		"setup.py", "pyproject.toml", "requirements.txt", "Pipfile",
		// Generic
		"README.md", "LICENSE", ".gitignore", "Makefile",
	}
	
	for _, fileName := range importantFiles {
		filePath := filepath.Join(workspacePath, fileName)
		projectFiles[fileName] = wv.fileExists(filePath)
	}
	
	return projectFiles
}