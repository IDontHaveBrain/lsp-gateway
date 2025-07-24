package project

import (
	"context"
	"lsp-gateway/internal/project/types"
	"lsp-gateway/internal/setup"
	"os"
	"path/filepath"
	"strings"
)

// PythonLanguageDetector implements comprehensive Python project detection
type PythonLanguageDetector struct {
	logger *setup.SetupLogger
}

// NewPythonLanguageDetector creates a new Python language detector
func NewPythonLanguageDetector() LanguageDetector {
	return &PythonLanguageDetector{
		logger: setup.NewSetupLogger(&setup.SetupLoggerConfig{Component: "python-detector"}),
	}
}

// DetectLanguage performs Python project detection
func (p *PythonLanguageDetector) DetectLanguage(ctx context.Context, path string) (*types.LanguageDetectionResult, error) {
	result := &types.LanguageDetectionResult{
		Language:        types.PROJECT_TYPE_PYTHON,
		RequiredServers: []string{types.SERVER_PYLSP},
		Metadata:        make(map[string]interface{}),
		MarkerFiles:     []string{},
		ConfigFiles:     []string{},
		SourceDirs:      []string{},
		TestDirs:        []string{},
		BuildFiles:      []string{},
		Dependencies:    make(map[string]string),
		DevDependencies: make(map[string]string),
	}

	confidence := 0.0

	// Check for Python marker files in priority order
	markers := []string{types.MARKER_PYPROJECT, types.MARKER_SETUP_PY, types.MARKER_REQUIREMENTS, types.MARKER_PIPFILE}
	for _, marker := range markers {
		if _, err := os.Stat(filepath.Join(path, marker)); err == nil {
			result.MarkerFiles = append(result.MarkerFiles, marker)
			result.ConfigFiles = append(result.ConfigFiles, marker)
			
			// Higher confidence for more modern markers
			switch marker {
			case types.MARKER_PYPROJECT:
				confidence = 0.95
			case types.MARKER_SETUP_PY:
				confidence = 0.9
			case types.MARKER_REQUIREMENTS:
				confidence = 0.85
			case types.MARKER_PIPFILE:
				confidence = 0.8
			}
			break // Use highest priority marker
		}
	}

	// Scan for Python files and directories
	pyFileCount := 0
	sourceDirs := make(map[string]bool)
	testDirs := make(map[string]bool)
	
	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue on errors
		}

		// Skip common ignore directories
		if info.IsDir() {
			name := info.Name()
			skipDirs := []string{"__pycache__", ".pytest_cache", ".tox", "venv", "env", ".venv", "build", "dist", ".git"}
			for _, skipDir := range skipDirs {
				if name == skipDir {
					return filepath.SkipDir
				}
			}
			return nil
		}

		if strings.HasSuffix(filePath, ".py") || strings.HasSuffix(filePath, ".pyw") || strings.HasSuffix(filePath, ".pyx") {
			pyFileCount++
			dir := filepath.Dir(filePath)
			relDir, _ := filepath.Rel(path, dir)
			
			fileName := strings.ToLower(info.Name())
			if strings.Contains(fileName, "test") || strings.Contains(filePath, "test") || strings.Contains(relDir, "test") {
				testDirs[relDir] = true
			} else {
				sourceDirs[relDir] = true
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Convert maps to slices
	for dir := range sourceDirs {
		if dir != "." {
			result.SourceDirs = append(result.SourceDirs, dir)
		}
	}
	for dir := range testDirs {
		if dir != "." {
			result.TestDirs = append(result.TestDirs, dir)
		}
	}

	// Adjust confidence based on Python files found
	if pyFileCount > 0 {
		if confidence == 0 {
			confidence = 0.8 // Python files without project files
		}
		result.Metadata["python_file_count"] = pyFileCount
	} else if confidence == 0 {
		confidence = 0.1 // No clear Python indicators
	}

	result.Confidence = confidence
	return result, nil
}

// GetMarkerFiles returns Python project marker files
func (p *PythonLanguageDetector) GetMarkerFiles() []string {
	return []string{types.MARKER_PYPROJECT, types.MARKER_SETUP_PY, types.MARKER_REQUIREMENTS, types.MARKER_PIPFILE}
}

// GetRequiredServers returns required LSP servers for Python
func (p *PythonLanguageDetector) GetRequiredServers() []string {
	return []string{types.SERVER_PYLSP}
}

// GetPriority returns the detection priority for Python
func (p *PythonLanguageDetector) GetPriority() int {
	return types.PRIORITY_PYTHON
}

// ValidateStructure validates Python project structure
func (p *PythonLanguageDetector) ValidateStructure(ctx context.Context, path string) error {
	// Check for Python files
	hasPythonFiles := false
	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		
		if !info.IsDir() && (strings.HasSuffix(filePath, ".py") || strings.HasSuffix(filePath, ".pyw") || strings.HasSuffix(filePath, ".pyx")) {
			hasPythonFiles = true
			return filepath.SkipAll // Found at least one, can stop
		}
		
		// Skip common ignore directories
		if info.IsDir() {
			name := info.Name()
			if name == "__pycache__" || name == ".pytest_cache" || name == ".tox" || 
			   name == "venv" || name == "env" || name == ".venv" {
				return filepath.SkipDir
			}
		}
		
		return nil
	})

	if err != nil {
		return types.NewValidationError(types.PROJECT_TYPE_PYTHON, "error scanning for Python files", err)
	}

	if !hasPythonFiles {
		return types.NewValidationError(types.PROJECT_TYPE_PYTHON, "no Python source files found", nil).
			WithMetadata("structure_issues", []string{"no .py, .pyw, or .pyx files found in project"})
	}

	return nil
}

// GetLanguageInfo returns information about the Python language
func (p *PythonLanguageDetector) GetLanguageInfo(language string) (*types.LanguageInfo, error) {
	return &types.LanguageInfo{
		Name:           types.PROJECT_TYPE_PYTHON,
		DisplayName:    "Python",
		FileExtensions: []string{".py", ".pyw", ".pyx"},
		LSPServers:     []string{types.SERVER_PYLSP},
	}, nil
}