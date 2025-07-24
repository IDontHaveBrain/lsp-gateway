package detectors

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"lsp-gateway/internal/platform"
	"lsp-gateway/internal/project/types"
	"lsp-gateway/internal/setup"
)

// PythonProjectDetector provides comprehensive Python project detection and analysis
type PythonProjectDetector struct {
	logger         *setup.SetupLogger
	executor       platform.CommandExecutor
	versionChecker *setup.VersionChecker
	pythonDetector *setup.PythonDetector
	parser         *PythonConfigParser
	timeout        time.Duration
}

// PythonDetectionMetadata contains detailed Python project analysis results
type PythonDetectionMetadata struct {
	PyProjectInfo    *PyProjectInfo             `json:"pyproject_info,omitempty"`
	SetupPyInfo      *SetupPyInfo               `json:"setup_py_info,omitempty"`
	VirtualEnvInfo   *VirtualEnvironmentInfo    `json:"venv_info,omitempty"`
	FrameworkInfo    *FrameworkAnalysis         `json:"framework_info,omitempty"`
	PythonVersion    string                     `json:"python_version,omitempty"`
	Dependencies     map[string]*types.PythonDependencyInfo `json:"dependencies,omitempty"`
	DevDependencies  map[string]*types.PythonDependencyInfo `json:"dev_dependencies,omitempty"`
	TestFrameworks   []string                   `json:"test_frameworks,omitempty"`
	PackageManager   string                     `json:"package_manager,omitempty"`
	BuildBackend     string                     `json:"build_backend,omitempty"`
	ProjectType      string                     `json:"project_type,omitempty"`
	HasC2Extensions  bool                       `json:"has_c_extensions,omitempty"`
	EntryPoints      map[string]string          `json:"entry_points,omitempty"`
}

// VirtualEnvironmentInfo contains virtual environment details
type VirtualEnvironmentInfo struct {
	Path         string `json:"path,omitempty"`
	Type         string `json:"type,omitempty"` // venv, virtualenv, conda, pipenv
	IsActive     bool   `json:"is_active"`
	PythonPath   string `json:"python_path,omitempty"`
	SitePackages string `json:"site_packages,omitempty"`
}

// FrameworkAnalysis contains detected framework information
type FrameworkAnalysis struct {
	WebFrameworks  []string `json:"web_frameworks,omitempty"`
	TestFrameworks []string `json:"test_frameworks,omitempty"`
	DataScience    []string `json:"data_science,omitempty"`
	MLFrameworks   []string `json:"ml_frameworks,omitempty"`
	AsyncFrameworks []string `json:"async_frameworks,omitempty"`
}

// types.PythonDependencyInfo contains detailed dependency information

// NewPythonProjectDetector creates a new comprehensive Python project detector
func NewPythonProjectDetector() *PythonProjectDetector {
	return &PythonProjectDetector{
		logger:         setup.NewSetupLogger(&setup.SetupLoggerConfig{Component: "python-detector"}),
		executor:       platform.NewCommandExecutor(),
		versionChecker: setup.NewVersionChecker(),
		pythonDetector: setup.NewPythonDetector(),
		parser:         NewPythonConfigParser(),
		timeout:        30 * time.Second,
	}
}

// DetectLanguage performs comprehensive Python project detection
func (d *PythonProjectDetector) DetectLanguage(ctx context.Context, path string) (*types.LanguageDetectionResult, error) {
	d.logger.Debug(fmt.Sprintf("Starting Python project detection at path: %s", path))

	timeoutCtx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()

	// Phase 1: Detect Python runtime and basic project markers
	runtimeInfo, err := d.detectPythonRuntime(timeoutCtx, path)
	if err != nil {
		return nil, types.NewDetectionError(types.PROJECT_TYPE_PYTHON, "runtime", path,
			fmt.Sprintf("Failed to detect Python runtime: %v", err), err)
	}

	// Phase 2: Analyze configuration files
	configAnalysis, err := d.analyzePythonConfigs(timeoutCtx, path)
	if err != nil {
		return nil, types.NewDetectionError(types.PROJECT_TYPE_PYTHON, "config", path,
			fmt.Sprintf("Failed to analyze Python configurations: %v", err), err)
	}

	// Phase 3: Analyze source structure and frameworks
	sourceAnalysis, err := d.analyzePythonSources(timeoutCtx, path)
	if err != nil {
		d.logger.WithError(err).Debug("Non-critical error in source analysis")
		// Continue with limited source analysis
	}

	// Phase 4: Analyze virtual environment
	venvInfo, err := d.analyzeVirtualEnvironment(timeoutCtx, path)
	if err != nil {
		d.logger.WithError(err).Debug("Non-critical error in virtual environment analysis")
	}

	// Phase 5: Calculate confidence and compile results
	result := d.compileDetectionResult(path, runtimeInfo, configAnalysis, sourceAnalysis, venvInfo)

	d.logger.Debug(fmt.Sprintf("Python detection completed with confidence: %f, files: %d", result.Confidence, len(result.MarkerFiles)))

	return result, nil
}

// GetLanguageInfo returns comprehensive information about Python language support
func (d *PythonProjectDetector) GetLanguageInfo(language string) (*types.LanguageInfo, error) {
	if language != types.PROJECT_TYPE_PYTHON {
		return nil, fmt.Errorf("unsupported language: %s", language)
	}

	return &types.LanguageInfo{
		Name:           types.PROJECT_TYPE_PYTHON,
		DisplayName:    "Python",
		MinVersion:     "3.8",
		MaxVersion:     "3.12",
		BuildTools:     []string{"setuptools", "poetry", "pdm", "hatchling", "flit"},
		PackageManager: "pip",
		TestFrameworks: []string{"pytest", "unittest", "nose2", "tox"},
		LintTools:      []string{"pylint", "flake8", "ruff", "bandit", "mypy"},
		FormatTools:    []string{"black", "autopep8", "yapf", "isort"},
		LSPServers:     []string{types.SERVER_PYLSP},
		FileExtensions: []string{".py", ".pyw", ".pyi", ".pyx"},
		Capabilities:   []string{"completion", "hover", "definition", "references", "formatting", "code_action", "rename"},
		Metadata: map[string]interface{}{
			"package_managers": []string{"pip", "conda", "pipenv", "poetry", "pdm"},
			"virtual_envs":     []string{"venv", "virtualenv", "conda", "pipenv", "poetry"},
			"documentation":    "docs.python.org",
			"pep_standards":    true,
		},
	}, nil
}

// GetMarkerFiles returns Python project marker files
func (d *PythonProjectDetector) GetMarkerFiles() []string {
	return []string{
		types.MARKER_PYPROJECT,
		types.MARKER_SETUP_PY,
		types.MARKER_REQUIREMENTS,
		types.MARKER_PIPFILE,
		"setup.cfg",
		"pytypes.toml",
		"Pipfile.lock",
		"poetry.lock",
		"pdm.lock",
		"environment.yml",
		"conda.yaml",
		"requirements-dev.txt",
		"requirements-test.txt",
		"tox.ini",
		"pytest.ini",
	}
}

// GetRequiredServers returns required LSP servers for Python
func (d *PythonProjectDetector) GetRequiredServers() []string {
	return []string{types.SERVER_PYLSP}
}

// GetPriority returns the detection priority for Python projects
func (d *PythonProjectDetector) GetPriority() int {
	return types.PRIORITY_PYTHON
}

// ValidateStructure validates Python project structure
func (d *PythonProjectDetector) ValidateStructure(ctx context.Context, path string) error {
	d.logger.Debug(fmt.Sprintf("Validating Python project structure at path: %s", path))

	timeoutCtx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()

	// Check for Python files
	hasPythonFiles, err := d.hasPythonSourceFiles(timeoutCtx, path)
	if err != nil {
		return types.NewValidationError(types.PROJECT_TYPE_PYTHON, fmt.Sprintf("Failed to check for Python source files at %s: %v", path, err), err)
	}

	if !hasPythonFiles {
		return types.NewValidationError(types.PROJECT_TYPE_PYTHON, fmt.Sprintf("No Python source files found in project at %s", path), nil)
	}

	// Validate configuration files
	if err := d.validateConfigurationFiles(timeoutCtx, path); err != nil {
		return types.NewValidationError(types.PROJECT_TYPE_PYTHON, fmt.Sprintf("Configuration validation failed at %s: %v", path, err), err)
	}

	// Validate Python runtime compatibility
	if err := d.validatePythonRuntime(timeoutCtx, path); err != nil {
		return types.NewValidationError(types.PROJECT_TYPE_PYTHON, fmt.Sprintf("Python runtime validation failed at %s: %v", path, err), err)
	}

	d.logger.Debug("Python project structure validation completed")
	return nil
}

// detectPythonRuntime detects Python runtime and basic project information
func (d *PythonProjectDetector) detectPythonRuntime(ctx context.Context, path string) (*setup.RuntimeInfo, error) {
	// Use the existing Python detector for runtime detection
	runtime, err := d.pythonDetector.DetectPython()
	if err != nil {
		return nil, fmt.Errorf("failed to detect Python runtime: %w", err)
	}

	// Enhance with project-specific Python version detection
	projectPythonVersion, err := d.detectProjectPythonVersion(ctx, path)
	if err != nil {
		d.logger.WithError(err).Debug("Could not detect project-specific Python version")
	} else if projectPythonVersion != "" {
		runtime.Version = projectPythonVersion
	}

	return runtime, nil
}

// analyzePythonConfigs analyzes Python configuration files
func (d *PythonProjectDetector) analyzePythonConfigs(ctx context.Context, path string) (*PythonDetectionMetadata, error) {
	metadata := &PythonDetectionMetadata{
		Dependencies:    make(map[string]*types.PythonDependencyInfo),
		DevDependencies: make(map[string]*types.PythonDependencyInfo),
		EntryPoints:     make(map[string]string),
	}

	// Parse pytypes.toml
	pyprojectPath := filepath.Join(path, "pytypes.toml")
	if _, err := os.Stat(pyprojectPath); err == nil {
		pyprojectInfo, err := d.parser.ParsePyproject(ctx, pyprojectPath)
		if err != nil {
			d.logger.WithError(err).Debug("Failed to parse pytypes.toml")
		} else {
			metadata.PyProjectInfo = pyprojectInfo
			metadata.BuildBackend = pyprojectInfo.BuildSystem.BuildBackend
			d.mergeDependencies(metadata.Dependencies, pyprojectInfo.Dependencies, "pytypes.toml")
			d.mergeDependencies(metadata.DevDependencies, pyprojectInfo.DevDependencies, "pytypes.toml")
		}
	}

	// Parse setup.py
	setupPyPath := filepath.Join(path, "setup.py")
	if _, err := os.Stat(setupPyPath); err == nil {
		setupInfo, err := d.parser.ParseSetupPy(ctx, setupPyPath)
		if err != nil {
			d.logger.WithError(err).Debug("Failed to parse setup.py")
		} else {
			metadata.SetupPyInfo = setupInfo
			d.mergeDependencies(metadata.Dependencies, setupInfo.Dependencies, "setup.py")
		}
	}

	// Parse requirements.txt
	requirementsPath := filepath.Join(path, "requirements.txt")
	if _, err := os.Stat(requirementsPath); err == nil {
		reqDeps, err := d.parser.ParseRequirements(ctx, requirementsPath)
		if err != nil {
			d.logger.WithError(err).Debug("Failed to parse requirements.txt")
		} else {
			d.mergeDependencies(metadata.Dependencies, reqDeps, "requirements.txt")
		}
	}

	// Parse Pipfile
	pipfilePath := filepath.Join(path, "Pipfile")
	if _, err := os.Stat(pipfilePath); err == nil {
		pipfileDeps, pipfileDevDeps, err := d.parser.ParsePipfile(ctx, pipfilePath)
		if err != nil {
			d.logger.WithError(err).Debug("Failed to parse Pipfile")
		} else {
			metadata.PackageManager = "pipenv"
			d.mergeDependencies(metadata.Dependencies, pipfileDeps, "Pipfile")
			d.mergeDependencies(metadata.DevDependencies, pipfileDevDeps, "Pipfile")
		}
	}

	// Determine project type and package manager
	d.determineProjectType(metadata)

	return metadata, nil
}

// analyzePythonSources analyzes Python source files and structure
func (d *PythonProjectDetector) analyzePythonSources(ctx context.Context, path string) (*FrameworkAnalysis, error) {
	analysis := &FrameworkAnalysis{}

	// Scan for framework imports and patterns
	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue walking
		}

		// Skip hidden directories and common ignore patterns
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "__pycache__" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		// Only analyze Python files
		if !strings.HasSuffix(filePath, ".py") {
			return nil
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil // Continue with other files
		}

		d.analyzeFileImports(string(content), analysis)
		return nil
	})

	if err != nil && err != context.Canceled {
		return nil, fmt.Errorf("failed to analyze source files: %w", err)
	}

	return analysis, nil
}

// analyzeVirtualEnvironment detects and analyzes virtual environment
func (d *PythonProjectDetector) analyzeVirtualEnvironment(ctx context.Context, path string) (*VirtualEnvironmentInfo, error) {
	venvInfo := &VirtualEnvironmentInfo{}

	// Check for common virtual environment directories
	venvPaths := []string{".venv", "venv", "env", ".env"}
	for _, venvPath := range venvPaths {
		fullPath := filepath.Join(path, venvPath)
		if info, err := os.Stat(fullPath); err == nil && info.IsDir() {
			venvInfo.Path = fullPath
			venvInfo.Type = "venv"
			
			// Check for Python executable
			pythonPath := filepath.Join(fullPath, "bin", "python")
			if _, err := os.Stat(pythonPath); err == nil {
				venvInfo.PythonPath = pythonPath
				venvInfo.SitePackages = filepath.Join(fullPath, "lib", "python*/site-packages")
			}
			break
		}
	}

	// Check for conda environment
	if condaEnv := os.Getenv("CONDA_DEFAULT_ENV"); condaEnv != "" {
		venvInfo.Type = "conda"
		venvInfo.IsActive = true
	}

	// Check for pipenv
	if _, err := os.Stat(filepath.Join(path, "Pipfile")); err == nil {
		venvInfo.Type = "pipenv"
	}

	return venvInfo, nil
}

// compileDetectionResult compiles all analysis results into final detection result
func (d *PythonProjectDetector) compileDetectionResult(path string, runtime *setup.RuntimeInfo, 
	config *PythonDetectionMetadata, source *FrameworkAnalysis, venv *VirtualEnvironmentInfo) *types.LanguageDetectionResult {
	
	result := &types.LanguageDetectionResult{
		Language:        types.PROJECT_TYPE_PYTHON,
		RequiredServers: d.GetRequiredServers(),
		Dependencies:    make(map[string]string),
		DevDependencies: make(map[string]string),
		Metadata:        make(map[string]interface{}),
	}

	// Set version information
	if runtime != nil {
		result.Version = runtime.Version
	}

	// Collect marker files
	markerFiles := d.collectMarkerFiles(path)
	result.MarkerFiles = markerFiles
	result.ConfigFiles = markerFiles // Same for Python projects

	// Calculate confidence based on detected files and runtime
	result.Confidence = d.calculateConfidence(markerFiles, runtime != nil, config, source)

	// Convert dependencies to simple map for compatibility
	if config != nil {
		for name, dep := range config.Dependencies {
			result.Dependencies[name] = dep.Version
		}
		for name, dep := range config.DevDependencies {
			result.DevDependencies[name] = dep.Version
		}
	}

	// Detect source and test directories
	result.SourceDirs = d.detectSourceDirectories(path)
	result.TestDirs = d.detectTestDirectories(path)

	// Add comprehensive metadata
	if config != nil {
		result.Metadata["python_metadata"] = config
	}
	if source != nil {
		result.Metadata["framework_analysis"] = source
	}
	if venv != nil {
		result.Metadata["virtual_environment"] = venv
	}

	return result
}

// Helper methods

func (d *PythonProjectDetector) detectProjectPythonVersion(ctx context.Context, path string) (string, error) {
	// Check .python-version file
	pythonVersionFile := filepath.Join(path, ".python-version")
	if content, err := os.ReadFile(pythonVersionFile); err == nil {
		return strings.TrimSpace(string(content)), nil
	}

	// Check pytypes.toml python version requirement
	pyprojectPath := filepath.Join(path, "pytypes.toml")
	if _, err := os.Stat(pyprojectPath); err == nil {
		pyproject, err := d.parser.ParsePyproject(ctx, pyprojectPath)
		if err == nil && pyproject.Project.RequiresPython != "" {
			return pyproject.Project.RequiresPython, nil
		}
	}

	return "", nil
}

func (d *PythonProjectDetector) hasPythonSourceFiles(ctx context.Context, path string) (bool, error) {
	found := false
	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || name == "__pycache__" {
				return filepath.SkipDir
			}
			return nil
		}

		if strings.HasSuffix(filePath, ".py") {
			found = true
			return fmt.Errorf("found") // Use error to stop walking
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		return nil
	})

	if err != nil && err.Error() == "found" {
		return true, nil
	}
	return found, err
}

func (d *PythonProjectDetector) validateConfigurationFiles(ctx context.Context, path string) error {
	// Validate pytypes.toml if present
	pyprojectPath := filepath.Join(path, "pytypes.toml")
	if _, err := os.Stat(pyprojectPath); err == nil {
		if _, err := d.parser.ParsePyproject(ctx, pyprojectPath); err != nil {
			return fmt.Errorf("invalid pytypes.toml: %w", err)
		}
	}

	// Validate setup.py if present
	setupPyPath := filepath.Join(path, "setup.py")
	if _, err := os.Stat(setupPyPath); err == nil {
		if _, err := d.parser.ParseSetupPy(ctx, setupPyPath); err != nil {
			return fmt.Errorf("invalid setup.py: %w", err)
		}
	}

	return nil
}

func (d *PythonProjectDetector) validatePythonRuntime(ctx context.Context, path string) error {
	// Check if Python is available
	if _, err := d.pythonDetector.DetectPython(); err != nil {
		return fmt.Errorf("Python runtime not available: %w", err)
	}

	return nil
}

func (d *PythonProjectDetector) collectMarkerFiles(path string) []string {
	var markerFiles []string
	for _, marker := range d.GetMarkerFiles() {
		if _, err := os.Stat(filepath.Join(path, marker)); err == nil {
			markerFiles = append(markerFiles, marker)
		}
	}
	return markerFiles
}

func (d *PythonProjectDetector) calculateConfidence(markerFiles []string, hasRuntime bool, 
	config *PythonDetectionMetadata, source *FrameworkAnalysis) float64 {
	
	confidence := 0.0

	// Base confidence from marker files
	if len(markerFiles) > 0 {
		confidence += 0.3
	}

	// Boost for modern configuration files
	for _, marker := range markerFiles {
		switch marker {
		case "pytypes.toml":
			confidence += 0.4
		case "setup.py":
			confidence += 0.3
		case "requirements.txt":
			confidence += 0.2
		case "Pipfile":
			confidence += 0.3
		}
	}

	// Runtime detection boost
	if hasRuntime {
		confidence += 0.2
	}

	// Configuration analysis boost
	if config != nil {
		if config.PyProjectInfo != nil {
			confidence += 0.1
		}
		if len(config.Dependencies) > 0 {
			confidence += 0.1
		}
	}

	// Limit to 1.0
	if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence
}

func (d *PythonProjectDetector) detectSourceDirectories(path string) []string {
	var sourceDirs []string
	commonSrcDirs := []string{"src", "lib", filepath.Base(path)}

	for _, dir := range commonSrcDirs {
		dirPath := filepath.Join(path, dir)
		if info, err := os.Stat(dirPath); err == nil && info.IsDir() {
			sourceDirs = append(sourceDirs, dir)
		}
	}

	// If no common source directories found, check for Python packages in root
	if len(sourceDirs) == 0 {
		entries, err := os.ReadDir(path)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
					initPath := filepath.Join(path, entry.Name(), "__init__.py")
					if _, err := os.Stat(initPath); err == nil {
						sourceDirs = append(sourceDirs, entry.Name())
					}
				}
			}
		}
	}

	return sourceDirs
}

func (d *PythonProjectDetector) detectTestDirectories(path string) []string {
	var testDirs []string
	commonTestDirs := []string{"tests", "test", "testing"}

	for _, dir := range commonTestDirs {
		dirPath := filepath.Join(path, dir)
		if info, err := os.Stat(dirPath); err == nil && info.IsDir() {
			testDirs = append(testDirs, dir)
		}
	}

	return testDirs
}

func (d *PythonProjectDetector) mergeDependencies(target map[string]*types.PythonDependencyInfo, 
	source map[string]*types.PythonDependencyInfo, sourceFile string) {
	for name, dep := range source {
		if existing, exists := target[name]; exists {
			// Merge dependency information, preferring more specific sources
			if existing.Version == "" && dep.Version != "" {
				existing.Version = dep.Version
			}
			if existing.Specifier == "" && dep.Specifier != "" {
				existing.Specifier = dep.Specifier
			}
		} else {
			// Create new dependency info
			target[name] = &types.PythonDependencyInfo{
				Name:      dep.Name,
				Version:   dep.Version,
				Specifier: dep.Specifier,
				Extras:    dep.Extras,
				Source:    sourceFile,
			}
		}
	}
}

func (d *PythonProjectDetector) determineProjectType(metadata *PythonDetectionMetadata) {
	// Determine package manager
	if metadata.PyProjectInfo != nil {
		if metadata.PyProjectInfo.Tool.Poetry != nil {
			metadata.PackageManager = "poetry"
		} else if metadata.PyProjectInfo.Tool.PDM != nil {
			metadata.PackageManager = "pdm"
		} else {
			metadata.PackageManager = "pip"
		}
	} else if metadata.SetupPyInfo != nil {
		metadata.PackageManager = "setuptools"
	}

	// Determine project type based on dependencies and structure
	if d.hasWebFramework(metadata.Dependencies) {
		metadata.ProjectType = "web-application"
	} else if d.hasDataSciencePackages(metadata.Dependencies) {
		metadata.ProjectType = "data-science"
	} else if d.hasMLPackages(metadata.Dependencies) {
		metadata.ProjectType = "machine-learning"
	} else {
		metadata.ProjectType = "library"
	}
}

func (d *PythonProjectDetector) hasWebFramework(deps map[string]*types.PythonDependencyInfo) bool {
	webFrameworks := []string{"django", "flask", "fastapi", "tornado", "pyramid", "bottle"}
	for _, framework := range webFrameworks {
		if _, exists := deps[framework]; exists {
			return true
		}
	}
	return false
}

func (d *PythonProjectDetector) hasDataSciencePackages(deps map[string]*types.PythonDependencyInfo) bool {
	dsPackages := []string{"pandas", "numpy", "scipy", "matplotlib", "seaborn", "plotly"}
	for _, pkg := range dsPackages {
		if _, exists := deps[pkg]; exists {
			return true
		}
	}
	return false
}

func (d *PythonProjectDetector) hasMLPackages(deps map[string]*types.PythonDependencyInfo) bool {
	mlPackages := []string{"tensorflow", "pytorch", "scikit-learn", "keras", "xgboost"}
	for _, pkg := range mlPackages {
		if _, exists := deps[pkg]; exists {
			return true
		}
	}
	return false
}

func (d *PythonProjectDetector) analyzeFileImports(content string, analysis *FrameworkAnalysis) {
	lines := strings.Split(content, "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Skip comments and empty lines
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		// Check for import statements
		if strings.HasPrefix(line, "import ") || strings.HasPrefix(line, "from ") {
			d.analyzeImportLine(line, analysis)
		}
	}
}

func (d *PythonProjectDetector) analyzeImportLine(line string, analysis *FrameworkAnalysis) {
	// Web frameworks
	webFrameworks := map[string]string{
		"django": "Django",
		"flask": "Flask", 
		"fastapi": "FastAPI",
		"tornado": "Tornado",
		"pyramid": "Pyramid",
		"bottle": "Bottle",
	}

	// Test frameworks
	testFrameworks := map[string]string{
		"pytest": "pytest",
		"unittest": "unittest",
		"nose": "nose",
		"testify": "testify",
	}

	// Data science packages
	dataScience := map[string]string{
		"pandas": "Pandas",
		"numpy": "NumPy",
		"scipy": "SciPy",
		"matplotlib": "Matplotlib",
		"seaborn": "Seaborn",
	}

	// ML frameworks
	mlFrameworks := map[string]string{
		"tensorflow": "TensorFlow",
		"torch": "PyTorch",
		"sklearn": "scikit-learn",
		"keras": "Keras",
		"xgboost": "XGBoost",
	}

	// Async frameworks
	asyncFrameworks := map[string]string{
		"asyncio": "asyncio",
		"aiohttp": "aiohttp",
		"trio": "Trio",
		"anyio": "AnyIO",
	}

	// Check each category
	d.checkFrameworkImports(line, webFrameworks, &analysis.WebFrameworks)
	d.checkFrameworkImports(line, testFrameworks, &analysis.TestFrameworks)
	d.checkFrameworkImports(line, dataScience, &analysis.DataScience)
	d.checkFrameworkImports(line, mlFrameworks, &analysis.MLFrameworks)
	d.checkFrameworkImports(line, asyncFrameworks, &analysis.AsyncFrameworks)
}

func (d *PythonProjectDetector) checkFrameworkImports(line string, frameworks map[string]string, target *[]string) {
	for pkg, name := range frameworks {
		if strings.Contains(line, pkg) {
			// Check if not already added
			found := false
			for _, existing := range *target {
				if existing == name {
					found = true
					break
				}
			}
			if !found {
				*target = append(*target, name)
			}
		}
	}
}