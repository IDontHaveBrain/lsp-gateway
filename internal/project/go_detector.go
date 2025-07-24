package project

import (
	"bufio"
	"context"
	"fmt"
	"lsp-gateway/internal/platform"
	"lsp-gateway/internal/project/types"
	"lsp-gateway/internal/setup"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// GoLanguageDetector implements comprehensive Go project detection
type GoLanguageDetector struct {
	logger   *setup.SetupLogger
	executor platform.CommandExecutor
}

// NewGoLanguageDetector creates a new Go language detector
func NewGoLanguageDetector() LanguageDetector {
	return &GoLanguageDetector{
		logger:   setup.NewSetupLogger(&setup.SetupLoggerConfig{Component: "go-detector"}),
		executor: platform.NewCommandExecutor(),
	}
}

func (g *GoLanguageDetector) DetectLanguage(ctx context.Context, path string) (*types.LanguageDetectionResult, error) {
	startTime := time.Now()
	
	g.logger.WithField("path", path).Debug("Starting Go project detection")
	
	result := &types.LanguageDetectionResult{
		Language:        types.PROJECT_TYPE_GO,
		RequiredServers: []string{types.SERVER_GOPLS},
		Metadata:        make(map[string]interface{}),
		MarkerFiles:     []string{},
		ConfigFiles:     []string{},
		SourceDirs:      []string{},
		TestDirs:        []string{},
		BuildFiles:      []string{},
		Dependencies:    make(map[string]string),
		DevDependencies: make(map[string]string),
	}

	// Check for go.mod (primary indicator)
	goModPath := filepath.Join(path, types.MARKER_GO_MOD)
	goModExists := false
	if _, err := os.Stat(goModPath); err == nil {
		goModExists = true
		result.MarkerFiles = append(result.MarkerFiles, types.MARKER_GO_MOD)
		result.ConfigFiles = append(result.ConfigFiles, types.MARKER_GO_MOD)
		result.Confidence = 0.95
		
		// Parse go.mod file
		if err := g.parseGoMod(goModPath, result); err != nil {
			g.logger.WithError(err).Warn("Failed to parse go.mod")
			result.Confidence = 0.7
		}
	}

	// Check for go.sum
	goSumPath := filepath.Join(path, types.MARKER_GO_SUM)
	if _, err := os.Stat(goSumPath); err == nil {
		result.MarkerFiles = append(result.MarkerFiles, types.MARKER_GO_SUM)
		if goModExists {
			result.Confidence = 0.98
		} else {
			result.Confidence = 0.6 // go.sum without go.mod is unusual
		}
	}

	// Check for .go files
	goFileCount := 0
	if err := g.scanForGoFiles(path, result, &goFileCount); err != nil {
		g.logger.WithError(err).Debug("Error scanning for Go files")
	}

	// Adjust confidence based on Go files found
	if goFileCount > 0 {
		if !goModExists {
			result.Confidence = 0.8 // Go files without go.mod (older Go project)
		}
		result.Metadata["go_file_count"] = goFileCount
	} else if !goModExists {
		result.Confidence = 0.1 // No clear Go indicators
	}

	// Detect Go version and toolchain
	if err := g.detectGoVersion(ctx, result); err != nil {
		g.logger.WithError(err).Debug("Failed to detect Go version")
	}

	// Detect frameworks and libraries
	g.detectGoFrameworks(path, result)

	// Detect project structure
	g.analyzeProjectStructure(path, result)

	// Set detection metadata
	result.Metadata["detection_time"] = time.Since(startTime)
	result.Metadata["detector_version"] = "1.0.0"

	g.logger.WithFields(map[string]interface{}{
		"confidence":      result.Confidence,
		"marker_files":    len(result.MarkerFiles),
		"source_dirs":     len(result.SourceDirs),
		"dependencies":    len(result.Dependencies),
		"detection_time":  time.Since(startTime),
	}).Info("Go project detection completed")

	return result, nil
}

func (g *GoLanguageDetector) parseGoMod(goModPath string, result *types.LanguageDetectionResult) error {
	file, err := os.Open(goModPath)
	if err != nil {
		return fmt.Errorf("failed to open go.mod: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var currentSection string
	moduleNameRegex := regexp.MustCompile(`^module\s+(.+)$`)
	goVersionRegex := regexp.MustCompile(`^go\s+([\d\.]+)$`)
	requireRegex := regexp.MustCompile(`^\s*([^\s]+)\s+([^\s]+)(?:\s+//\s*(.*))?$`)
	replaceRegex := regexp.MustCompile(`^\s*([^\s]+)\s+=>\s+([^\s]+)(?:\s+([^\s]+))?`)

	goModInfo := make(map[string]interface{})
	
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		
		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		// Detect sections
		if strings.HasPrefix(line, "require (") {
			currentSection = "require"
			continue
		} else if strings.HasPrefix(line, "replace (") {
			currentSection = "replace"
			continue
		} else if strings.HasPrefix(line, "exclude (") {
			currentSection = "exclude"
			continue
		} else if line == ")" {
			currentSection = ""
			continue
		}

		// Parse module name
		if matches := moduleNameRegex.FindStringSubmatch(line); len(matches) == 2 {
			moduleName := matches[1]
			result.Metadata["module_name"] = moduleName
			goModInfo["module"] = moduleName
			
			// Extract project name from module path
			parts := strings.Split(moduleName, "/")
			if len(parts) > 0 {
				result.Metadata["project_name"] = parts[len(parts)-1]
			}
			continue
		}

		// Parse Go version
		if matches := goVersionRegex.FindStringSubmatch(line); len(matches) == 2 {
			version := matches[1]
			result.Version = version
			result.Metadata["go_version"] = version
			goModInfo["go_version"] = version
			continue
		}

		// Parse requirements
		if currentSection == "require" || strings.HasPrefix(line, "require ") {
			line = strings.TrimPrefix(line, "require ")
			if matches := requireRegex.FindStringSubmatch(line); len(matches) >= 3 {
				depName := matches[1]
				depVersion := matches[2]
				result.Dependencies[depName] = depVersion
				
				// Check for indirect dependencies
				if len(matches) >= 4 && strings.Contains(matches[3], "indirect") {
					result.DevDependencies[depName] = depVersion
					delete(result.Dependencies, depName)
				}
			}
			continue
		}

		// Parse replacements
		if currentSection == "replace" || strings.HasPrefix(line, "replace ") {
			line = strings.TrimPrefix(line, "replace ")
			if matches := replaceRegex.FindStringSubmatch(line); len(matches) >= 3 {
				original := matches[1]
				replacement := matches[2]
				if result.Metadata["replacements"] == nil {
					result.Metadata["replacements"] = make(map[string]string)
				}
				result.Metadata["replacements"].(map[string]string)[original] = replacement
			}
			continue
		}
	}

	result.Metadata["go_mod_info"] = goModInfo
	return nil
}

func (g *GoLanguageDetector) scanForGoFiles(path string, result *types.LanguageDetectionResult, goFileCount *int) error {
	sourceDirs := make(map[string]bool)
	testDirs := make(map[string]bool)
	
	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue on errors
		}

		// Skip vendor and hidden directories
		if info.IsDir() {
			name := info.Name()
			if name == "vendor" || name == ".git" || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}

		if strings.HasSuffix(filePath, ".go") {
			*goFileCount++
			dir := filepath.Dir(filePath)
			relDir, _ := filepath.Rel(path, dir)
			
			fileName := info.Name()
			if strings.HasSuffix(fileName, "_test.go") {
				testDirs[relDir] = true
			} else {
				sourceDirs[relDir] = true
			}
		}

		// Look for build files
		if info.Name() == "Makefile" || info.Name() == "Dockerfile" {
			result.BuildFiles = append(result.BuildFiles, info.Name())
		}

		return nil
	})

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

	return err
}

func (g *GoLanguageDetector) detectGoVersion(ctx context.Context, result *types.LanguageDetectionResult) error {
	// Try to get Go version from installed Go
	cmd := []string{"go", "version"}
	execResult, err := g.executor.Execute(cmd[0], cmd[1:], 10*time.Second)
	if err != nil {
		return err
	}

	if execResult.ExitCode == 0 {
		output := strings.TrimSpace(execResult.Stdout)
		// Parse "go version go1.21.0 linux/amd64"
		parts := strings.Fields(output)
		if len(parts) >= 3 {
			version := strings.TrimPrefix(parts[2], "go")
			if result.Version == "" { // Only set if not already set from go.mod
				result.Version = version
			}
			result.Metadata["installed_go_version"] = version
		}
	}

	return nil
}

func (g *GoLanguageDetector) detectGoFrameworks(path string, result *types.LanguageDetectionResult) {
	frameworks := []string{}
	
	// Check dependencies for known frameworks
	for dep := range result.Dependencies {
		switch {
		case strings.Contains(dep, "gin-gonic/gin"):
			frameworks = append(frameworks, "Gin")
		case strings.Contains(dep, "gorilla/mux"):
			frameworks = append(frameworks, "Gorilla Mux")
		case strings.Contains(dep, "echo"):
			frameworks = append(frameworks, "Echo")
		case strings.Contains(dep, "fiber"):
			frameworks = append(frameworks, "Fiber")
		case strings.Contains(dep, "chi"):
			frameworks = append(frameworks, "Chi")
		case strings.Contains(dep, "beego"):
			frameworks = append(frameworks, "Beego")
		case strings.Contains(dep, "revel"):
			frameworks = append(frameworks, "Revel")
		case strings.Contains(dep, "kubernetes"):
			frameworks = append(frameworks, "Kubernetes")
		case strings.Contains(dep, "docker"):
			frameworks = append(frameworks, "Docker")
		case strings.Contains(dep, "grpc"):
			frameworks = append(frameworks, "gRPC")
		case strings.Contains(dep, "protobuf"):
			frameworks = append(frameworks, "Protocol Buffers")
		}
	}

	if len(frameworks) > 0 {
		result.Metadata["frameworks"] = frameworks
	}

	// Check for specific configuration files
	configFiles := map[string]string{
		"Dockerfile":     "Docker",
		"docker-compose.yml": "Docker Compose",
		".goreleaser.yml": "GoReleaser",
		"air.toml":       "Air (Hot Reload)",
	}

	for file, framework := range configFiles {
		if _, err := os.Stat(filepath.Join(path, file)); err == nil {
			result.ConfigFiles = append(result.ConfigFiles, file)
			frameworks = append(frameworks, framework)
		}
	}

	// Update frameworks list
	if len(frameworks) > 0 {
		result.Metadata["frameworks"] = frameworks
	}
}

func (g *GoLanguageDetector) analyzeProjectStructure(path string, result *types.LanguageDetectionResult) {
	commonDirs := []string{"cmd", "internal", "pkg", "api", "web", "docs", "scripts", "build"}
	foundDirs := []string{}

	for _, dir := range commonDirs {
		dirPath := filepath.Join(path, dir)
		if info, err := os.Stat(dirPath); err == nil && info.IsDir() {
			foundDirs = append(foundDirs, dir)
		}
	}

	if len(foundDirs) > 0 {
		result.Metadata["project_structure"] = foundDirs
		
		// Adjust confidence based on Go project structure
		if len(foundDirs) >= 3 {
			result.Confidence = min(result.Confidence+0.05, 1.0)
		}
	}

	// Check for main.go in standard locations
	mainLocations := []string{"main.go", "cmd/main.go", "cmd/*/main.go"}
	for _, location := range mainLocations {
		matches, _ := filepath.Glob(filepath.Join(path, location))
		if len(matches) > 0 {
			result.Metadata["has_main"] = true
			break
		}
	}
}

// GetLanguageInfo returns comprehensive information about Go language support
func (g *GoLanguageDetector) GetLanguageInfo(language string) (*types.LanguageInfo, error) {
	if language != types.PROJECT_TYPE_GO {
		return nil, fmt.Errorf("unsupported language: %s", language)
	}

	return &types.LanguageInfo{
		Name:           types.PROJECT_TYPE_GO,
		DisplayName:    "Go",
		MinVersion:     "1.19",
		MaxVersion:     "1.22",
		BuildTools:     []string{"go build", "go install", "make", "goreleaser", "task"},
		PackageManager: "go mod",
		TestFrameworks: []string{"testing", "testify", "ginkgo", "gomega"},
		LintTools:      []string{"golangci-lint", "gofmt", "goimports", "go vet", "staticcheck"},
		FormatTools:    []string{"gofmt", "goimports", "gofumpt"},
		LSPServers:     []string{types.SERVER_GOPLS},
		FileExtensions: []string{".go", ".mod", ".sum"},
		Capabilities:   []string{"completion", "hover", "definition", "references", "formatting", "code_action", "rename"},
		Metadata: map[string]interface{}{
			"module_system":   "go modules",
			"package_hosting": "pkg.go.dev",
			"documentation":   "godoc",
		},
	}, nil
}

// Interface implementation methods
func (g *GoLanguageDetector) GetMarkerFiles() []string {
	return []string{types.MARKER_GO_MOD, types.MARKER_GO_SUM}
}

func (g *GoLanguageDetector) GetRequiredServers() []string {
	return []string{types.SERVER_GOPLS}
}

func (g *GoLanguageDetector) GetPriority() int {
	return types.PRIORITY_GO
}

func (g *GoLanguageDetector) ValidateStructure(ctx context.Context, path string) error {
	// Check for go.mod
	goModPath := filepath.Join(path, types.MARKER_GO_MOD)
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		return types.NewValidationError(types.PROJECT_TYPE_GO, "go.mod file not found", nil).
			WithMetadata("missing_files", []string{types.MARKER_GO_MOD}).
			WithSuggestion("Initialize Go module: go mod init <module-name>").
			WithSuggestion("Ensure you're in the correct Go project directory")
	}

	// Check if go.mod is readable
	if file, err := os.Open(goModPath); err != nil {
		return types.NewValidationError(types.PROJECT_TYPE_GO, "cannot read go.mod file", err).
			WithMetadata("invalid_files", []string{types.MARKER_GO_MOD})
	} else {
		file.Close()
	}

	// Check for .go files
	hasGoFiles := false
	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		
		if !info.IsDir() && strings.HasSuffix(filePath, ".go") {
			hasGoFiles = true
			return filepath.SkipAll // Found at least one, can stop
		}
		
		// Skip vendor directory
		if info.IsDir() && info.Name() == "vendor" {
			return filepath.SkipDir
		}
		
		return nil
	})

	if err != nil {
		return types.NewValidationError(types.PROJECT_TYPE_GO, "error scanning for Go files", err)
	}

	if !hasGoFiles {
		return types.NewValidationError(types.PROJECT_TYPE_GO, "no Go source files found", nil).
			WithMetadata("structure_issues", []string{"no .go files found in project"})
	}

	return nil
}

// Helper function
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}