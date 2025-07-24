package detectors

import (
	"bufio"
	"context"
	"fmt"
	"lsp-gateway/internal/project/types"
	"lsp-gateway/internal/setup"
	"os"
	"regexp"
	"strings"
	"time"
)

// GoModInfo represents parsed go.mod file information
type GoModInfo struct {
	ModuleName string            `json:"module_name"`
	GoVersion  string            `json:"go_version"`
	Requires   map[string]string `json:"requires"`
	Replaces   map[string]string `json:"replaces"`
	Excludes   []string          `json:"excludes"`
	Retracts   []string          `json:"retracts"`
}

// GoWorkspaceInfo represents parsed go.work file information  
type GoWorkspaceInfo struct {
	GoVersion string            `json:"go_version"`
	Modules   []string          `json:"modules"`
	Replaces  map[string]string `json:"replaces"`
}


// ImportAnalysis contains import path analysis results
type ImportAnalysis struct {
	StandardLibrary []string          `json:"standard_library"`
	ThirdParty      []string          `json:"third_party"`
	Internal        []string          `json:"internal"`
	Relative        []string          `json:"relative"`
	Dependencies    map[string]int    `json:"dependencies"` // import path -> count
}

// GoModuleParser handles parsing of go.mod files
type GoModuleParser struct {
	logger  *setup.SetupLogger
	timeout time.Duration
}

// GoWorkspaceParser handles parsing of go.work files
type GoWorkspaceParser struct {
	logger  *setup.SetupLogger
	timeout time.Duration
}

// NewGoModuleParser creates a new Go module parser
func NewGoModuleParser(logger *setup.SetupLogger) *GoModuleParser {
	if logger == nil {
		logger = setup.NewSetupLogger(&setup.SetupLoggerConfig{
			Component: "go-module-parser",
			Level:     setup.LogLevelInfo,
		})
	}

	return &GoModuleParser{
		logger:  logger.WithField("component", "go_module_parser"),
		timeout: 30 * time.Second,
	}
}

// NewGoWorkspaceParser creates a new Go workspace parser
func NewGoWorkspaceParser(logger *setup.SetupLogger) *GoWorkspaceParser {
	if logger == nil {
		logger = setup.NewSetupLogger(&setup.SetupLoggerConfig{
			Component: "go-workspace-parser",
			Level:     setup.LogLevelInfo,
		})
	}

	return &GoWorkspaceParser{
		logger:  logger.WithField("component", "go_workspace_parser"),
		timeout: 30 * time.Second,
	}
}

// ParseGoMod parses a go.mod file and extracts module information
func (p *GoModuleParser) ParseGoMod(ctx context.Context, goModPath string) (*GoModInfo, error) {
	startTime := time.Now()
	p.logger.WithField("path", goModPath).Debug("Starting go.mod parsing")

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	// Initialize result
	modInfo := &GoModInfo{
		Requires: make(map[string]string),
		Replaces: make(map[string]string),
		Excludes: []string{},
		Retracts: []string{},
	}

	// Read file with context cancellation support
	content, err := p.readFileWithContext(timeoutCtx, goModPath)
	if err != nil {
		return nil, types.NewDetectionError(types.PROJECT_TYPE_GO, "parsing", goModPath,
			fmt.Sprintf("Failed to read go.mod file: %v", err), err)
	}

	// Parse content
	if err := p.parseGoModContent(content, modInfo); err != nil {
		return nil, types.NewDetectionError(types.PROJECT_TYPE_GO, "parsing", goModPath,
			fmt.Sprintf("Failed to parse go.mod content: %v", err), err)
	}

	// Validate parsed information
	if err := p.validateGoModInfo(modInfo); err != nil {
		p.logger.WithError(err).WithField("path", goModPath).Warn("go.mod validation issues found")
		// Don't return error - just log warning for validation issues
	}

	parseTime := time.Since(startTime)
	p.logger.WithFields(map[string]interface{}{
		"path":         goModPath,
		"module":       modInfo.ModuleName,
		"go_version":   modInfo.GoVersion,
		"requires":     len(modInfo.Requires),
		"replaces":     len(modInfo.Replaces),
		"excludes":     len(modInfo.Excludes),
		"parse_time":   parseTime,
	}).Debug("go.mod parsing completed")

	return modInfo, nil
}

// parseGoModContent parses the content of a go.mod file
func (p *GoModuleParser) parseGoModContent(content string, modInfo *GoModInfo) error {
	scanner := bufio.NewScanner(strings.NewReader(content))
	lineNum := 0
	inRequireBlock := false
	inReplaceBlock := false
	inExcludeBlock := false
	inRetractBlock := false

	// Regular expressions for parsing
	moduleRe := regexp.MustCompile(`^module\s+(.+)$`)
	goVersionRe := regexp.MustCompile(`^go\s+(.+)$`)
	requireRe := regexp.MustCompile(`^require\s*\(?(.*)$`)
	replaceRe := regexp.MustCompile(`^replace\s*\(?(.*)$`)
	excludeRe := regexp.MustCompile(`^exclude\s*\(?(.*)$`)
	retractRe := regexp.MustCompile(`^retract\s*\(?(.*)$`)
	
	// Regex for dependency lines (handles both single line and block format)
	_ = regexp.MustCompile(`^\s*([^\s]+)\s+([^\s]+)(?:\s+//\s*(.*))?$`)
	_ = regexp.MustCompile(`^\s*([^\s]+)\s*=>\s*([^\s]+)(?:\s+([^\s]+))?(?:\s+//\s*(.*))?$`)

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		// Handle end of blocks
		if line == ")" {
			inRequireBlock = false
			inReplaceBlock = false
			inExcludeBlock = false
			inRetractBlock = false
			continue
		}

		// Parse module directive
		if matches := moduleRe.FindStringSubmatch(line); matches != nil {
			modInfo.ModuleName = strings.TrimSpace(matches[1])
			continue
		}

		// Parse go version directive
		if matches := goVersionRe.FindStringSubmatch(line); matches != nil {
			modInfo.GoVersion = strings.TrimSpace(matches[1])
			continue
		}

		// Handle require block
		if matches := requireRe.FindStringSubmatch(line); matches != nil {
			requirement := strings.TrimSpace(matches[1])
			if requirement == "" || strings.HasSuffix(requirement, "(") {
				inRequireBlock = true
			} else {
				// Single line require
				if err := p.parseRequireLine(requirement, modInfo); err != nil {
					p.logger.WithError(err).WithField("line", lineNum).Debug("Failed to parse require line")
				}
			}
			continue
		}

		// Handle replace block
		if matches := replaceRe.FindStringSubmatch(line); matches != nil {
			replacement := strings.TrimSpace(matches[1])
			if replacement == "" || strings.HasSuffix(replacement, "(") {
				inReplaceBlock = true
			} else {
				// Single line replace
				if err := p.parseReplaceLine(replacement, modInfo); err != nil {
					p.logger.WithError(err).WithField("line", lineNum).Debug("Failed to parse replace line")
				}
			}
			continue
		}

		// Handle exclude block
		if matches := excludeRe.FindStringSubmatch(line); matches != nil {
			exclusion := strings.TrimSpace(matches[1])
			if exclusion == "" || strings.HasSuffix(exclusion, "(") {
				inExcludeBlock = true
			} else {
				// Single line exclude
				modInfo.Excludes = append(modInfo.Excludes, exclusion)
			}
			continue
		}

		// Handle retract block
		if matches := retractRe.FindStringSubmatch(line); matches != nil {
			retraction := strings.TrimSpace(matches[1])
			if retraction == "" || strings.HasSuffix(retraction, "(") {
				inRetractBlock = true
			} else {
				// Single line retract
				modInfo.Retracts = append(modInfo.Retracts, retraction)
			}
			continue
		}

		// Parse lines within blocks
		if inRequireBlock {
			if err := p.parseRequireLine(line, modInfo); err != nil {
				p.logger.WithError(err).WithField("line", lineNum).Debug("Failed to parse require line in block")
			}
		} else if inReplaceBlock {
			if err := p.parseReplaceLine(line, modInfo); err != nil {
				p.logger.WithError(err).WithField("line", lineNum).Debug("Failed to parse replace line in block")
			}
		} else if inExcludeBlock {
			modInfo.Excludes = append(modInfo.Excludes, line)
		} else if inRetractBlock {
			modInfo.Retracts = append(modInfo.Retracts, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading go.mod content: %w", err)
	}

	return nil
}

// parseRequireLine parses a single require line
func (p *GoModuleParser) parseRequireLine(line string, modInfo *GoModInfo) error {
	// Remove inline comments
	if commentIdx := strings.Index(line, "//"); commentIdx != -1 {
		line = strings.TrimSpace(line[:commentIdx])
	}

	parts := strings.Fields(line)
	if len(parts) < 2 {
		return fmt.Errorf("invalid require line format: %s", line)
	}

	module := parts[0]
	version := parts[1]

	// Handle indirect dependencies (marked with // indirect comment in original)
	// This is a simplified approach - in practice you'd want to preserve the comment info
	modInfo.Requires[module] = version

	return nil
}

// parseReplaceLine parses a single replace line
func (p *GoModuleParser) parseReplaceLine(line string, modInfo *GoModInfo) error {
	// Remove inline comments
	if commentIdx := strings.Index(line, "//"); commentIdx != -1 {
		line = strings.TrimSpace(line[:commentIdx])
	}

	// Handle replace directive: old_module => new_module [version]
	arrowIdx := strings.Index(line, "=>")
	if arrowIdx == -1 {
		return fmt.Errorf("invalid replace line format (missing '=>'): %s", line)
	}

	oldModule := strings.TrimSpace(line[:arrowIdx])
	replacement := strings.TrimSpace(line[arrowIdx+2:])

	if oldModule == "" || replacement == "" {
		return fmt.Errorf("invalid replace line format: %s", line)
	}

	modInfo.Replaces[oldModule] = replacement

	return nil
}

// validateGoModInfo validates the parsed go.mod information
func (p *GoModuleParser) validateGoModInfo(modInfo *GoModInfo) error {
	var validationErrors []string

	// Validate module name
	if modInfo.ModuleName == "" {
		validationErrors = append(validationErrors, "module name is required")
	} else if !p.isValidModuleName(modInfo.ModuleName) {
		validationErrors = append(validationErrors, fmt.Sprintf("invalid module name format: %s", modInfo.ModuleName))
	}

	// Validate Go version if present
	if modInfo.GoVersion != "" && !p.isValidGoVersion(modInfo.GoVersion) {
		validationErrors = append(validationErrors, fmt.Sprintf("invalid Go version format: %s", modInfo.GoVersion))
	}

	// Validate dependencies
	for module, version := range modInfo.Requires {
		if !p.isValidModuleName(module) {
			validationErrors = append(validationErrors, fmt.Sprintf("invalid dependency module name: %s", module))
		}
		if !p.isValidVersion(version) {
			validationErrors = append(validationErrors, fmt.Sprintf("invalid dependency version for %s: %s", module, version))
		}
	}

	// Validate replacements
	for oldModule, newPath := range modInfo.Replaces {
		if !p.isValidModuleName(oldModule) {
			validationErrors = append(validationErrors, fmt.Sprintf("invalid replace source module: %s", oldModule))
		}
		if newPath == "" {
			validationErrors = append(validationErrors, fmt.Sprintf("empty replacement path for module: %s", oldModule))
		}
	}

	if len(validationErrors) > 0 {
		return fmt.Errorf("go.mod validation errors: %s", strings.Join(validationErrors, "; "))
	}

	return nil
}

// ParseGoWork parses a go.work file and extracts workspace information
func (p *GoWorkspaceParser) ParseGoWork(ctx context.Context, goWorkPath string) (*GoWorkspaceInfo, error) {
	startTime := time.Now()
	p.logger.WithField("path", goWorkPath).Debug("Starting go.work parsing")

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	// Initialize result
	workspaceInfo := &GoWorkspaceInfo{
		Modules:  []string{},
		Replaces: make(map[string]string),
	}

	// Read file with context cancellation support
	content, err := p.readFileWithContext(timeoutCtx, goWorkPath)
	if err != nil {
		return nil, types.NewDetectionError(types.PROJECT_TYPE_GO, "parsing", goWorkPath,
			fmt.Sprintf("Failed to read go.work file: %v", err), err)
	}

	// Parse content
	if err := p.parseGoWorkContent(content, workspaceInfo); err != nil {
		return nil, types.NewDetectionError(types.PROJECT_TYPE_GO, "parsing", goWorkPath,
			fmt.Sprintf("Failed to parse go.work content: %v", err), err)
	}

	parseTime := time.Since(startTime)
	p.logger.WithFields(map[string]interface{}{
		"path":        goWorkPath,
		"go_version":  workspaceInfo.GoVersion,
		"modules":     len(workspaceInfo.Modules),
		"replaces":    len(workspaceInfo.Replaces),
		"parse_time":  parseTime,
	}).Debug("go.work parsing completed")

	return workspaceInfo, nil
}

// parseGoWorkContent parses the content of a go.work file
func (p *GoWorkspaceParser) parseGoWorkContent(content string, workspaceInfo *GoWorkspaceInfo) error {
	scanner := bufio.NewScanner(strings.NewReader(content))
	lineNum := 0
	inUseBlock := false
	inReplaceBlock := false

	// Regular expressions for parsing
	goVersionRe := regexp.MustCompile(`^go\s+(.+)$`)
	useRe := regexp.MustCompile(`^use\s*\(?(.*)$`)
	replaceRe := regexp.MustCompile(`^replace\s*\(?(.*)$`)

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		// Handle end of blocks
		if line == ")" {
			inUseBlock = false
			inReplaceBlock = false
			continue
		}

		// Parse go version directive
		if matches := goVersionRe.FindStringSubmatch(line); matches != nil {
			workspaceInfo.GoVersion = strings.TrimSpace(matches[1])
			continue
		}

		// Handle use block
		if matches := useRe.FindStringSubmatch(line); matches != nil {
			module := strings.TrimSpace(matches[1])
			if module == "" || strings.HasSuffix(module, "(") {
				inUseBlock = true
			} else {
				// Single line use
				workspaceInfo.Modules = append(workspaceInfo.Modules, p.cleanModulePath(module))
			}
			continue
		}

		// Handle replace block  
		if matches := replaceRe.FindStringSubmatch(line); matches != nil {
			replacement := strings.TrimSpace(matches[1])
			if replacement == "" || strings.HasSuffix(replacement, "(") {
				inReplaceBlock = true
			} else {
				// Single line replace
				if err := p.parseWorkspaceReplaceLine(replacement, workspaceInfo); err != nil {
					p.logger.WithError(err).WithField("line", lineNum).Debug("Failed to parse replace line")
				}
			}
			continue
		}

		// Parse lines within blocks
		if inUseBlock {
			cleanPath := p.cleanModulePath(line)
			if cleanPath != "" {
				workspaceInfo.Modules = append(workspaceInfo.Modules, cleanPath)
			}
		} else if inReplaceBlock {
			if err := p.parseWorkspaceReplaceLine(line, workspaceInfo); err != nil {
				p.logger.WithError(err).WithField("line", lineNum).Debug("Failed to parse replace line in block")
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading go.work content: %w", err)
	}

	return nil
}

// parseWorkspaceReplaceLine parses a replace line in go.work
func (p *GoWorkspaceParser) parseWorkspaceReplaceLine(line string, workspaceInfo *GoWorkspaceInfo) error {
	// Remove inline comments
	if commentIdx := strings.Index(line, "//"); commentIdx != -1 {
		line = strings.TrimSpace(line[:commentIdx])
	}

	// Handle replace directive: old_module => new_module [version]
	arrowIdx := strings.Index(line, "=>")
	if arrowIdx == -1 {
		return fmt.Errorf("invalid replace line format (missing '=>'): %s", line)
	}

	oldModule := strings.TrimSpace(line[:arrowIdx])
	replacement := strings.TrimSpace(line[arrowIdx+2:])

	if oldModule == "" || replacement == "" {
		return fmt.Errorf("invalid replace line format: %s", line)
	}

	workspaceInfo.Replaces[oldModule] = replacement
	return nil
}

// ExtractImports analyzes Go source files and extracts import information
func (p *GoModuleParser) ExtractImports(ctx context.Context, sourceFiles []string) (*ImportAnalysis, error) {
	analysis := &ImportAnalysis{
		StandardLibrary: []string{},
		ThirdParty:      []string{},
		Internal:        []string{},
		Relative:        []string{},
		Dependencies:    make(map[string]int),
	}

	for _, sourceFile := range sourceFiles {
		select {
		case <-ctx.Done():
			return analysis, ctx.Err()
		default:
		}

		if err := p.analyzeImportsInFile(sourceFile, analysis); err != nil {
			p.logger.WithError(err).WithField("file", sourceFile).Debug("Failed to analyze imports")
			continue // Continue with other files
		}
	}

	return analysis, nil
}

// analyzeImportsInFile analyzes imports in a single Go source file
func (p *GoModuleParser) analyzeImportsInFile(filePath string, analysis *ImportAnalysis) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	inImportBlock := false
	
	importRe := regexp.MustCompile(`import\s*"([^"]+)"`)
	importBlockRe := regexp.MustCompile(`import\s*\(`)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		// Detect import block start
		if importBlockRe.MatchString(line) {
			inImportBlock = true
			continue
		}

		// Detect import block end
		if inImportBlock && line == ")" {
			inImportBlock = false
			continue
		}

		// Extract import paths
		var importPath string
		
		if inImportBlock {
			// Inside import block - extract quoted path
			if strings.Contains(line, "\"") {
				start := strings.Index(line, "\"")
				end := strings.LastIndex(line, "\"")
				if start != -1 && end != -1 && start != end {
					importPath = line[start+1 : end]
				}
			}
		} else {
			// Single line import
			if matches := importRe.FindStringSubmatch(line); matches != nil {
				importPath = matches[1]
			}
		}

		if importPath != "" {
			p.categorizeImport(importPath, analysis)
		}
	}

	return nil
}

// categorizeImport categorizes an import path into standard library, third-party, etc.
func (p *GoModuleParser) categorizeImport(importPath string, analysis *ImportAnalysis) {
	analysis.Dependencies[importPath]++

	// Categorize import
	if strings.HasPrefix(importPath, ".") {
		analysis.Relative = append(analysis.Relative, importPath)
	} else if p.isStandardLibraryPackage(importPath) {
		analysis.StandardLibrary = append(analysis.StandardLibrary, importPath)
	} else if strings.Contains(importPath, ".") {
		// Contains dot - likely third party
		analysis.ThirdParty = append(analysis.ThirdParty, importPath)
	} else {
		// No dot - might be internal or relative
		analysis.Internal = append(analysis.Internal, importPath)
	}
}

// AnalyzeDependencies performs dependency graph analysis
func (p *GoModuleParser) AnalyzeDependencies(ctx context.Context, modInfo *GoModInfo) (map[string]*types.GoDependencyInfo, error) {
	dependencies := make(map[string]*types.GoDependencyInfo)

	for module, version := range modInfo.Requires {
		depInfo := &types.GoDependencyInfo{
			Version:      version,
			Indirect:     false, // Would need to parse comments to determine this
			IsPrerelease: p.isPrerelease(version),
			Tags:         []string{},
		}

		dependencies[module] = depInfo
	}

	return dependencies, nil
}

// Helper methods

func (p *GoModuleParser) readFileWithContext(ctx context.Context, filePath string) (string, error) {
	done := make(chan struct{})
	var content []byte
	var err error

	go func() {
		defer close(done)
		content, err = os.ReadFile(filePath)
	}()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-done:
		if err != nil {
			return "", err
		}
		return string(content), nil
	}
}

func (p *GoWorkspaceParser) readFileWithContext(ctx context.Context, filePath string) (string, error) {
	done := make(chan struct{})
	var content []byte
	var err error

	go func() {
		defer close(done)
		content, err = os.ReadFile(filePath)
	}()

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-done:
		if err != nil {
			return "", err
		}
		return string(content), nil
	}
}

func (p *GoModuleParser) isValidModuleName(name string) bool {
	// Basic validation - in practice this would be more comprehensive
	return name != "" && !strings.Contains(name, " ")
}

func (p *GoModuleParser) isValidGoVersion(version string) bool {
	// Basic Go version validation
	goVersionRe := regexp.MustCompile(`^\d+\.\d+(\.\d+)?$`)
	return goVersionRe.MatchString(version)
}

func (p *GoModuleParser) isValidVersion(version string) bool {
	// Basic semantic version validation
	return version != "" && !strings.Contains(version, " ")
}

func (p *GoWorkspaceParser) cleanModulePath(path string) string {
	// Remove quotes and clean path
	path = strings.Trim(path, "\"")
	path = strings.TrimSpace(path)
	return path
}

func (p *GoModuleParser) isStandardLibraryPackage(importPath string) bool {
	// List of common standard library packages
	stdLibPackages := map[string]bool{
		"bufio": true, "bytes": true, "context": true, "crypto": true,
		"database": true, "encoding": true, "errors": true, "fmt": true,
		"hash": true, "html": true, "http": true, "io": true, "json": true,
		"log": true, "math": true, "net": true, "os": true, "path": true,
		"reflect": true, "regexp": true, "runtime": true, "sort": true,
		"strconv": true, "strings": true, "sync": true, "syscall": true,
		"testing": true, "time": true, "unicode": true, "unsafe": true,
	}

	// Check if it's a known standard library package or starts with known prefixes
	if stdLibPackages[importPath] {
		return true
	}

	stdLibPrefixes := []string{
		"archive/", "bufio/", "builtin/", "bytes/", "compress/",
		"container/", "context/", "crypto/", "database/", "debug/",
		"encoding/", "errors/", "expvar/", "flag/", "fmt/", "go/",
		"hash/", "html/", "image/", "index/", "io/", "log/", "math/",
		"mime/", "net/", "os/", "path/", "plugin/", "reflect/",
		"regexp/", "runtime/", "sort/", "strconv/", "strings/",
		"sync/", "syscall/", "testing/", "text/", "time/", "unicode/",
		"unsafe/",
	}

	for _, prefix := range stdLibPrefixes {
		if strings.HasPrefix(importPath, prefix) {
			return true
		}
	}

	return false
}

func (p *GoModuleParser) isPrerelease(version string) bool {
	// Check if version contains prerelease indicators
	prereleaseIndicators := []string{"-alpha", "-beta", "-rc", "-pre", "-dev"}
	lowerVersion := strings.ToLower(version)
	
	for _, indicator := range prereleaseIndicators {
		if strings.Contains(lowerVersion, indicator) {
			return true
		}
	}
	
	return false
}