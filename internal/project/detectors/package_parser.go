package detectors

import (
	"bufio"
	"encoding/json"
	"fmt"
	"lsp-gateway/internal/project/types"
	"lsp-gateway/internal/setup"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// PackageJsonParser handles parsing of package.json files and related Node.js configurations
type PackageJsonParser struct {
	logger *setup.SetupLogger
}

// TsConfigParser handles parsing of TypeScript configuration files
type TsConfigParser struct {
	logger *setup.SetupLogger
}

// LockfileParser handles parsing of various lockfile formats
type LockfileParser struct {
	logger *setup.SetupLogger
}

// WorkspaceParser handles parsing of workspace configurations
type WorkspaceParser struct {
	logger *setup.SetupLogger
}

// PackageJsonInfo contains parsed package.json information
type PackageJsonInfo struct {
	Name            string                 `json:"name,omitempty"`
	Version         string                 `json:"version,omitempty"`
	Description     string                 `json:"description,omitempty"`
	Main            string                 `json:"main,omitempty"`
	Module          string                 `json:"module,omitempty"`
	Types           string                 `json:"types,omitempty"`
	Type            string                 `json:"type,omitempty"` // "module" or "commonjs"
	Scripts         map[string]string      `json:"scripts,omitempty"`
	Dependencies    map[string]string      `json:"dependencies,omitempty"`
	DevDependencies map[string]string      `json:"devDependencies,omitempty"`
	PeerDependencies map[string]string     `json:"peerDependencies,omitempty"`
	OptionalDependencies map[string]string `json:"optionalDependencies,omitempty"`
	Bin             map[string]string      `json:"bin,omitempty"`
	Exports         map[string]interface{} `json:"exports,omitempty"`
	Imports         map[string]interface{} `json:"imports,omitempty"`
	Workspaces      []string               `json:"workspaces,omitempty"`
	Private         bool                   `json:"private,omitempty"`
	License         string                 `json:"license,omitempty"`
	Author          interface{}            `json:"author,omitempty"`
	Contributors    []interface{}          `json:"contributors,omitempty"`
	Repository      interface{}            `json:"repository,omitempty"`
	Homepage        string                 `json:"homepage,omitempty"`
	Bugs            interface{}            `json:"bugs,omitempty"`
	Keywords        []string               `json:"keywords,omitempty"`
	Engines         map[string]string      `json:"engines,omitempty"`
	OS              []string               `json:"os,omitempty"`
	CPU             []string               `json:"cpu,omitempty"`
	Config          map[string]interface{} `json:"config,omitempty"`
	Files           []string               `json:"files,omitempty"`
	Directories     map[string]string      `json:"directories,omitempty"`
	PackageManager  string                 `json:"packageManager,omitempty"`
	Raw             map[string]interface{} `json:"-"` // Store raw JSON for custom fields
}

// TSConfigInfo contains parsed tsconfig.json information
type TSConfigInfo struct {
	CompilerOptions map[string]interface{} `json:"compilerOptions,omitempty"`
	Include         []string               `json:"include,omitempty"`
	Exclude         []string               `json:"exclude,omitempty"`
	Files           []string               `json:"files,omitempty"`
	Extends         interface{}            `json:"extends,omitempty"` // Can be string or array
	References      []ProjectReference     `json:"references,omitempty"`
	TypeAcquisition map[string]interface{} `json:"typeAcquisition,omitempty"`
	WatchOptions    map[string]interface{} `json:"watchOptions,omitempty"`
	BuildOptions    map[string]interface{} `json:"buildOptions,omitempty"`
	CompileOnSave   bool                   `json:"compileOnSave,omitempty"`
	Raw             map[string]interface{} `json:"-"` // Store raw JSON for custom fields
}

// ProjectReference represents a TypeScript project reference
type ProjectReference struct {
	Path            string `json:"path"`
	Prepend         bool   `json:"prepend,omitempty"`
	Circular        bool   `json:"circular,omitempty"`
}

// LockfileInfo contains parsed lockfile information
type LockfileInfo struct {
	Type          string                       `json:"type"` // "npm", "yarn", "pnpm", "bun"
	Version       string                       `json:"version,omitempty"`
	Dependencies  map[string]*LockedDependency `json:"dependencies,omitempty"`
	Metadata      map[string]interface{}       `json:"metadata,omitempty"`
	Integrity     string                       `json:"integrity,omitempty"`
	Issues        []string                     `json:"issues,omitempty"`
}

// LockedDependency represents a locked dependency with version and integrity info
type LockedDependency struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Resolved     string            `json:"resolved,omitempty"`
	Integrity    string            `json:"integrity,omitempty"`
	Dependencies map[string]string `json:"dependencies,omitempty"`
	DevDependency bool             `json:"dev,omitempty"`
	Optional     bool              `json:"optional,omitempty"`
	Peer         bool              `json:"peer,omitempty"`
	Bundled      bool              `json:"bundled,omitempty"`
	Issues       []string          `json:"issues,omitempty"`
}

// WorkspaceInfo contains parsed workspace configuration
type WorkspaceInfo struct {
	Type            string                 `json:"type"` // "npm", "yarn", "pnpm", "lerna", "nx", "rush"
	Root            string                 `json:"root"`
	Packages        []WorkspacePackage     `json:"packages,omitempty"`
	Configuration   map[string]interface{} `json:"configuration,omitempty"`
	Dependencies    map[string]string      `json:"dependencies,omitempty"`
	DevDependencies map[string]string      `json:"devDependencies,omitempty"`
	Scripts         map[string]string      `json:"scripts,omitempty"`
	Issues          []string               `json:"issues,omitempty"`
}

// WorkspacePackage represents a package within a workspace
type WorkspacePackage struct {
	Name         string            `json:"name"`
	Path         string            `json:"path"`
	Version      string            `json:"version,omitempty"`
	Private      bool              `json:"private,omitempty"`
	Dependencies map[string]string `json:"dependencies,omitempty"`
	Scripts      map[string]string `json:"scripts,omitempty"`
	Issues       []string          `json:"issues,omitempty"`
}

// Constructor functions

func NewPackageJsonParser(logger *setup.SetupLogger) *PackageJsonParser {
	return &PackageJsonParser{logger: logger}
}

func NewTsConfigParser(logger *setup.SetupLogger) *TsConfigParser {
	return &TsConfigParser{logger: logger}
}

func NewLockfileParser(logger *setup.SetupLogger) *LockfileParser {
	return &LockfileParser{logger: logger}
}

func NewWorkspaceParser(logger *setup.SetupLogger) *WorkspaceParser {
	return &WorkspaceParser{logger: logger}
}

// PackageJsonParser methods

// ParsePackageJson parses a package.json file and returns structured information
func (p *PackageJsonParser) ParsePackageJson(filePath string) (*PackageJsonInfo, error) {
	p.logger.WithField("file", filePath).Debug("Parsing package.json")

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, types.NewDetectionError("package-parser", "read_file", filePath, 
			fmt.Sprintf("Failed to read package.json: %v", err), err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, types.NewDetectionError("package-parser", "json_parse", filePath,
			fmt.Sprintf("Invalid JSON in package.json: %v", err), err)
	}

	info := &PackageJsonInfo{
		Dependencies:         make(map[string]string),
		DevDependencies:      make(map[string]string),
		PeerDependencies:     make(map[string]string),
		OptionalDependencies: make(map[string]string),
		Scripts:              make(map[string]string),
		Bin:                  make(map[string]string),
		Engines:              make(map[string]string),
		Config:               make(map[string]interface{}),
		Directories:          make(map[string]string),
		Raw:                  raw,
	}

	// Parse different field types using helper methods
	p.parseBasicStringFields(raw, info)
	p.parseInterfaceFields(raw, info)
	p.parseArrayFields(raw, info)
	p.parseDependencyFields(raw, info)
	p.parseComplexObjectFields(raw, info)

	p.logger.WithFields(map[string]interface{}{
		"name":              info.Name,
		"version":           info.Version,
		"dependencies":      len(info.Dependencies),
		"dev_dependencies":  len(info.DevDependencies),
		"peer_dependencies": len(info.PeerDependencies),
		"scripts":           len(info.Scripts),
		"workspaces":        len(info.Workspaces),
	}).Debug("Package.json parsed successfully")

	return info, nil
}

// ValidatePackageJson validates the structure and content of a package.json file
func (p *PackageJsonParser) ValidatePackageJson(filePath string) error {
	info, err := p.ParsePackageJson(filePath)
	if err != nil {
		return err
	}

	var issues []string

	// Basic validation
	if info.Name == "" {
		issues = append(issues, "package name is required")
	} else if !p.isValidPackageName(info.Name) {
		issues = append(issues, fmt.Sprintf("invalid package name: %s", info.Name))
	}

	if info.Version == "" {
		issues = append(issues, "package version is required")
	} else if !p.isValidSemVer(info.Version) {
		issues = append(issues, fmt.Sprintf("invalid semantic version: %s", info.Version))
	}

	// Validate dependencies
	for name, version := range info.Dependencies {
		if !p.isValidPackageName(name) {
			issues = append(issues, fmt.Sprintf("invalid dependency name: %s", name))
		}
		if !p.isValidVersionRange(version) {
			issues = append(issues, fmt.Sprintf("invalid version range for %s: %s", name, version))
		}
	}

	// Validate scripts
	for name, script := range info.Scripts {
		if strings.TrimSpace(script) == "" {
			issues = append(issues, fmt.Sprintf("empty script: %s", name))
		}
	}

	// Validate engines
	for engine, version := range info.Engines {
		if !p.isValidVersionRange(version) {
			issues = append(issues, fmt.Sprintf("invalid engine version range for %s: %s", engine, version))
		}
	}

	if len(issues) > 0 {
		return types.NewValidationError("package-json", fmt.Sprintf("Package.json validation failed for %s: %s", filePath, strings.Join(issues, ", ")), nil)
	}

	return nil
}

// TsConfigParser methods

// ParseTSConfig parses a tsconfig.json file and returns structured information
func (t *TsConfigParser) ParseTSConfig(filePath string) (*TSConfigInfo, error) {
	t.logger.WithField("file", filePath).Debug("Parsing TypeScript configuration")

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, types.NewDetectionError("tsconfig-parser", "read_file", filePath,
			fmt.Sprintf("Failed to read TypeScript config: %v", err), err)
	}

	// Handle JSON with comments (JSONC format)
	cleanedData := t.removeJSONComments(string(data))

	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(cleanedData), &raw); err != nil {
		return nil, types.NewDetectionError("tsconfig-parser", "json_parse", filePath,
			fmt.Sprintf("Invalid JSON in TypeScript config: %v", err), err)
	}

	info := &TSConfigInfo{
		CompilerOptions: make(map[string]interface{}),
		TypeAcquisition: make(map[string]interface{}),
		WatchOptions:    make(map[string]interface{}),
		BuildOptions:    make(map[string]interface{}),
		Raw:             raw,
	}

	// Parse compiler options
	if compilerOptions, ok := raw["compilerOptions"].(map[string]interface{}); ok {
		info.CompilerOptions = compilerOptions
	}

	// Parse include array
	if include, ok := raw["include"].([]interface{}); ok {
		for _, inc := range include {
			if incStr, ok := inc.(string); ok {
				info.Include = append(info.Include, incStr)
			}
		}
	}

	// Parse exclude array
	if exclude, ok := raw["exclude"].([]interface{}); ok {
		for _, exc := range exclude {
			if excStr, ok := exc.(string); ok {
				info.Exclude = append(info.Exclude, excStr)
			}
		}
	}

	// Parse files array
	if files, ok := raw["files"].([]interface{}); ok {
		for _, file := range files {
			if fileStr, ok := file.(string); ok {
				info.Files = append(info.Files, fileStr)
			}
		}
	}

	// Parse extends (can be string or array)
	if extends := raw["extends"]; extends != nil {
		info.Extends = extends
	}

	// Parse project references
	if references, ok := raw["references"].([]interface{}); ok {
		for _, ref := range references {
			if refMap, ok := ref.(map[string]interface{}); ok {
				var projRef ProjectReference
				if path, ok := refMap["path"].(string); ok {
					projRef.Path = path
				}
				if prepend, ok := refMap["prepend"].(bool); ok {
					projRef.Prepend = prepend
				}
				if circular, ok := refMap["circular"].(bool); ok {
					projRef.Circular = circular
				}
				info.References = append(info.References, projRef)
			}
		}
	}

	// Parse other options
	if typeAcquisition, ok := raw["typeAcquisition"].(map[string]interface{}); ok {
		info.TypeAcquisition = typeAcquisition
	}
	if watchOptions, ok := raw["watchOptions"].(map[string]interface{}); ok {
		info.WatchOptions = watchOptions
	}
	if buildOptions, ok := raw["buildOptions"].(map[string]interface{}); ok {
		info.BuildOptions = buildOptions
	}
	if compileOnSave, ok := raw["compileOnSave"].(bool); ok {
		info.CompileOnSave = compileOnSave
	}

	t.logger.WithFields(map[string]interface{}{
		"compiler_options": len(info.CompilerOptions),
		"include_patterns": len(info.Include),
		"exclude_patterns": len(info.Exclude),
		"files":           len(info.Files),
		"references":      len(info.References),
	}).Debug("TypeScript configuration parsed successfully")

	return info, nil
}

// ValidateTSConfig validates the structure and content of a tsconfig.json file
func (t *TsConfigParser) ValidateTSConfig(filePath string) error {
	info, err := t.ParseTSConfig(filePath)
	if err != nil {
		return err
	}

	var issues []string

	// Validate compiler options
	if len(info.CompilerOptions) == 0 {
		issues = append(issues, "no compiler options specified")
	} else {
		// Check for common required options
		if target, ok := info.CompilerOptions["target"]; ok {
			if targetStr, ok := target.(string); ok {
				validTargets := []string{"ES3", "ES5", "ES6", "ES2015", "ES2016", "ES2017", "ES2018", "ES2019", "ES2020", "ES2021", "ES2022", "ESNext"}
				if !contains(validTargets, strings.ToUpper(targetStr)) {
					issues = append(issues, fmt.Sprintf("invalid target: %s", targetStr))
				}
			}
		}

		// Validate module option
		if module, ok := info.CompilerOptions["module"]; ok {
			if moduleStr, ok := module.(string); ok {
				validModules := []string{"CommonJS", "AMD", "System", "UMD", "ES6", "ES2015", "ES2020", "ESNext", "None"}
				if !contains(validModules, moduleStr) {
					issues = append(issues, fmt.Sprintf("invalid module type: %s", moduleStr))
				}
			}
		}

		// Check for strict mode recommendations
		if strict, ok := info.CompilerOptions["strict"]; !ok || strict != true {
			issues = append(issues, "strict mode is recommended for better type safety")
		}
	}

	// Validate include/exclude patterns
	if len(info.Include) == 0 && len(info.Files) == 0 {
		issues = append(issues, "no include patterns or files specified")
	}

	// Validate project references
	for i, ref := range info.References {
		if ref.Path == "" {
			issues = append(issues, fmt.Sprintf("project reference %d missing path", i))
		}
	}

	if len(issues) > 0 {
		return types.NewValidationError("tsconfig", fmt.Sprintf("TSConfig validation failed for %s: %s", filePath, strings.Join(issues, ", ")), nil)
	}

	return nil
}

// LockfileParser methods

// ParseLockfile parses various lockfile formats and returns structured information
func (l *LockfileParser) ParseLockfile(filePath string) (*LockfileInfo, error) {
	l.logger.WithField("file", filePath).Debug("Parsing lockfile")

	fileName := filepath.Base(filePath)

	switch fileName {
	case "package-lock.json":
		return l.parseNPMLockfile(filePath)
	case "yarn.lock":
		return l.parseYarnLockfile(filePath)
	case "pnpm-lock.yaml":
		return l.parsePNPMLockfile(filePath)
	case "bun.lockb":
		return l.parseBunLockfile(filePath)
	default:
		return nil, types.NewDetectionError("lockfile-parser", "unsupported_format", filePath,
			fmt.Sprintf("Unsupported lockfile format: %s", fileName), nil)
	}
}

// parseNPMLockfile parses package-lock.json files
func (l *LockfileParser) parseNPMLockfile(filePath string) (*LockfileInfo, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	info := &LockfileInfo{
		Type:         "npm",
		Dependencies: make(map[string]*LockedDependency),
		Metadata:     make(map[string]interface{}),
	}

	if version, ok := raw["lockfileVersion"]; ok {
		info.Version = fmt.Sprintf("%v", version)
	}

	if dependencies, ok := raw["dependencies"].(map[string]interface{}); ok {
		l.parseNPMDependencies(dependencies, info.Dependencies)
	}

	l.logger.WithField("dependencies", len(info.Dependencies)).Debug("NPM lockfile parsed")
	return info, nil
}

// parseYarnLockfile parses yarn.lock files
func (l *LockfileParser) parseYarnLockfile(filePath string) (*LockfileInfo, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	info := &LockfileInfo{
		Type:         "yarn",
		Dependencies: make(map[string]*LockedDependency),
		Metadata:     make(map[string]interface{}),
	}

	scanner := bufio.NewScanner(file)
	var currentDep *LockedDependency
	var currentName string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Package declaration line
		if strings.Contains(line, ":") && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			// Save previous dependency
			if currentDep != nil && currentName != "" {
				info.Dependencies[currentName] = currentDep
			}

			// Parse new dependency
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				nameVersion := strings.TrimSpace(parts[0])
				// Handle multiple version specs: "package@^1.0.0", "package@^2.0.0":
				currentName = l.parseYarnPackageName(nameVersion)
				currentDep = &LockedDependency{
					Name:         currentName,
					Dependencies: make(map[string]string),
				}
			}
		} else if currentDep != nil {
			// Property lines
			l.parseYarnProperty(line, currentDep)
		}
	}

	// Save last dependency
	if currentDep != nil && currentName != "" {
		info.Dependencies[currentName] = currentDep
	}

	l.logger.WithField("dependencies", len(info.Dependencies)).Debug("Yarn lockfile parsed")
	return info, nil
}

// parsePNPMLockfile parses pnpm-lock.yaml files
func (l *LockfileParser) parsePNPMLockfile(filePath string) (*LockfileInfo, error) {
	// Note: This is a simplified YAML parser. In production, you'd use a proper YAML library
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	info := &LockfileInfo{
		Type:         "pnpm",
		Dependencies: make(map[string]*LockedDependency),
		Metadata:     make(map[string]interface{}),
	}

	lines := strings.Split(string(data), "\n")
	var inDependencies bool

	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		if line == "dependencies:" {
			inDependencies = true
			continue
		}
		
		if inDependencies && strings.HasPrefix(line, " ") {
			// Parse dependency line: "  package: 1.0.0"
			parts := strings.SplitN(strings.TrimSpace(line), ":", 2)
			if len(parts) == 2 {
				name := strings.TrimSpace(parts[0])
				version := strings.TrimSpace(parts[1])
				info.Dependencies[name] = &LockedDependency{
					Name:         name,
					Version:      version,
					Dependencies: make(map[string]string),
				}
			}
		} else if !strings.HasPrefix(line, " ") && line != "" {
			inDependencies = false
		}
	}

	l.logger.WithField("dependencies", len(info.Dependencies)).Debug("PNPM lockfile parsed")
	return info, nil
}

// parseBunLockfile parses bun.lockb files (binary format - simplified)
func (l *LockfileParser) parseBunLockfile(filePath string) (*LockfileInfo, error) {
	// Note: bun.lockb is a binary format. This is a placeholder implementation.
	// In practice, you'd need to use Bun's tools to parse this file.
	
	info := &LockfileInfo{
		Type:         "bun",
		Dependencies: make(map[string]*LockedDependency),
		Metadata:     make(map[string]interface{}),
	}

	// Binary lockfile - would need special parsing
	l.logger.Debug("Bun lockfile detected (binary format)")
	return info, nil
}

// WorkspaceParser methods

// ParseWorkspaces parses workspace configurations based on the monorepo type
func (w *WorkspaceParser) ParseWorkspaces(rootPath string, monorepoInfo *MonorepoInfo) ([]string, error) {
	w.logger.WithFields(map[string]interface{}{
		"type": monorepoInfo.Type,
		"root": rootPath,
	}).Debug("Parsing workspace configuration")

	switch monorepoInfo.Type {
	case "lerna":
		return w.parseLernaWorkspaces(rootPath)
	case "nx":
		return w.parseNxWorkspaces(rootPath)
	case "rush":
		return w.parseRushWorkspaces(rootPath)
	case "yarn-workspaces", "npm-workspaces":
		return w.parsePackageWorkspaces(rootPath)
	case "pnpm-workspaces":
		return w.parsePNPMWorkspaces(rootPath)
	default:
		return nil, types.NewDetectionError("workspace-parser", "unsupported_type", rootPath,
			fmt.Sprintf("Unsupported workspace type: %s", monorepoInfo.Type), nil)
	}
}

// parseLernaWorkspaces parses Lerna workspace configuration
func (w *WorkspaceParser) parseLernaWorkspaces(rootPath string) ([]string, error) {
	lernaPath := filepath.Join(rootPath, "lerna.json")
	data, err := os.ReadFile(lernaPath)
	if err != nil {
		return nil, err
	}

	var lerna map[string]interface{}
	if err := json.Unmarshal(data, &lerna); err != nil {
		return nil, err
	}

	var packages []string
	if pkgs, ok := lerna["packages"].([]interface{}); ok {
		for _, pkg := range pkgs {
			if pkgStr, ok := pkg.(string); ok {
				packages = append(packages, pkgStr)
			}
		}
	}

	return packages, nil
}

// parseNxWorkspaces parses Nx workspace configuration
func (w *WorkspaceParser) parseNxWorkspaces(rootPath string) ([]string, error) {
	nxPath := filepath.Join(rootPath, "nx.json")
	data, err := os.ReadFile(nxPath)
	if err != nil {
		return nil, err
	}

	var nx map[string]interface{}
	if err := json.Unmarshal(data, &nx); err != nil {
		return nil, err
	}

	var packages []string
	
	// Nx uses different patterns, often based on projects
	if projects, ok := nx["projects"].(map[string]interface{}); ok {
		for projectName := range projects {
			packages = append(packages, projectName)
		}
	}

	return packages, nil
}

// parseRushWorkspaces parses Rush workspace configuration
func (w *WorkspaceParser) parseRushWorkspaces(rootPath string) ([]string, error) {
	rushPath := filepath.Join(rootPath, "rush.json")
	data, err := os.ReadFile(rushPath)
	if err != nil {
		return nil, err
	}

	var rush map[string]interface{}
	if err := json.Unmarshal(data, &rush); err != nil {
		return nil, err
	}

	var packages []string
	if projects, ok := rush["projects"].([]interface{}); ok {
		for _, project := range projects {
			if projectMap, ok := project.(map[string]interface{}); ok {
				if projectFolder, ok := projectMap["projectFolder"].(string); ok {
					packages = append(packages, projectFolder)
				}
			}
		}
	}

	return packages, nil
}

// parsePackageWorkspaces parses npm/yarn workspace configuration from package.json
func (w *WorkspaceParser) parsePackageWorkspaces(rootPath string) ([]string, error) {
	packagePath := filepath.Join(rootPath, "package.json")
	parser := NewPackageJsonParser(w.logger)
	
	packageInfo, err := parser.ParsePackageJson(packagePath)
	if err != nil {
		return nil, err
	}

	return packageInfo.Workspaces, nil
}

// parsePNPMWorkspaces parses pnpm workspace configuration
func (w *WorkspaceParser) parsePNPMWorkspaces(rootPath string) ([]string, error) {
	workspacePath := filepath.Join(rootPath, "pnpm-workspace.yaml")
	data, err := os.ReadFile(workspacePath)
	if err != nil {
		// Try alternative filename
		workspacePath = filepath.Join(rootPath, "pnpm-workspace.yml")
		data, err = os.ReadFile(workspacePath)
		if err != nil {
			return nil, err
		}
	}

	var packages []string
	lines := strings.Split(string(data), "\n")
	var inPackages bool

	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		if line == "packages:" {
			inPackages = true
			continue
		}
		
		if inPackages && strings.HasPrefix(line, "-") {
			// Parse package line: "- 'packages/*'"
			pkg := strings.TrimSpace(strings.TrimPrefix(line, "-"))
			pkg = strings.Trim(pkg, "'\"")
			packages = append(packages, pkg)
		} else if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "-") && line != "" {
			inPackages = false
		}
	}

	return packages, nil
}

// Helper methods for parsing different field types

func (p *PackageJsonParser) parseBasicStringFields(raw map[string]interface{}, info *PackageJsonInfo) {
	if name, ok := raw["name"].(string); ok {
		info.Name = name
	}
	if version, ok := raw["version"].(string); ok {
		info.Version = version
	}
	if desc, ok := raw["description"].(string); ok {
		info.Description = desc
	}
	if main, ok := raw["main"].(string); ok {
		info.Main = main
	}
	if module, ok := raw["module"].(string); ok {
		info.Module = module
	}
	if types, ok := raw["types"].(string); ok {
		info.Types = types
	}
	if pkgType, ok := raw["type"].(string); ok {
		info.Type = pkgType
	}
	if license, ok := raw["license"].(string); ok {
		info.License = license
	}
	if homepage, ok := raw["homepage"].(string); ok {
		info.Homepage = homepage
	}
	if packageManager, ok := raw["packageManager"].(string); ok {
		info.PackageManager = packageManager
	}
	if private, ok := raw["private"].(bool); ok {
		info.Private = private
	}
}

func (p *PackageJsonParser) parseInterfaceFields(raw map[string]interface{}, info *PackageJsonInfo) {
	if author := raw["author"]; author != nil {
		info.Author = author
	}
	if repository := raw["repository"]; repository != nil {
		info.Repository = repository
	}
	if bugs := raw["bugs"]; bugs != nil {
		info.Bugs = bugs
	}
}

func (p *PackageJsonParser) parseArrayFields(raw map[string]interface{}, info *PackageJsonInfo) {
	p.parseStringArrayField(raw, "keywords", &info.Keywords)
	p.parseStringArrayField(raw, "files", &info.Files)
	p.parseStringArrayField(raw, "os", &info.OS)
	p.parseStringArrayField(raw, "cpu", &info.CPU)
	
	if contributors, ok := raw["contributors"].([]interface{}); ok {
		info.Contributors = contributors
	}
}

func (p *PackageJsonParser) parseStringArrayField(raw map[string]interface{}, key string, target *[]string) {
	if array, ok := raw[key].([]interface{}); ok {
		for _, item := range array {
			if str, ok := item.(string); ok {
				*target = append(*target, str)
			}
		}
	}
}

func (p *PackageJsonParser) parseDependencyFields(raw map[string]interface{}, info *PackageJsonInfo) {
	p.parseStringMap(raw, "scripts", info.Scripts)
	p.parseStringMap(raw, "dependencies", info.Dependencies)
	p.parseStringMap(raw, "devDependencies", info.DevDependencies)
	p.parseStringMap(raw, "peerDependencies", info.PeerDependencies)
	p.parseStringMap(raw, "optionalDependencies", info.OptionalDependencies)
	p.parseStringMap(raw, "engines", info.Engines)
	p.parseStringMap(raw, "directories", info.Directories)
}

func (p *PackageJsonParser) parseComplexObjectFields(raw map[string]interface{}, info *PackageJsonInfo) {
	p.parseBinField(raw, info)
	p.parseExportsImportsFields(raw, info)
	p.parseConfigField(raw, info)
	p.parseWorkspacesField(raw, info)
}

func (p *PackageJsonParser) parseBinField(raw map[string]interface{}, info *PackageJsonInfo) {
	bin := raw["bin"]
	if bin == nil {
		return
	}
	
	switch binValue := bin.(type) {
	case string:
		if info.Name != "" {
			info.Bin[info.Name] = binValue
		}
	case map[string]interface{}:
		p.parseStringMapFromInterface(binValue, info.Bin)
	}
}

func (p *PackageJsonParser) parseExportsImportsFields(raw map[string]interface{}, info *PackageJsonInfo) {
	if exports := raw["exports"]; exports != nil {
		info.Exports = exports.(map[string]interface{})
	}
	if imports := raw["imports"]; imports != nil {
		info.Imports = imports.(map[string]interface{})
	}
}

func (p *PackageJsonParser) parseConfigField(raw map[string]interface{}, info *PackageJsonInfo) {
	if config, ok := raw["config"].(map[string]interface{}); ok {
		info.Config = config
	}
}

func (p *PackageJsonParser) parseWorkspacesField(raw map[string]interface{}, info *PackageJsonInfo) {
	workspaces := raw["workspaces"]
	if workspaces == nil {
		return
	}
	
	switch ws := workspaces.(type) {
	case []interface{}:
		for _, w := range ws {
			if wStr, ok := w.(string); ok {
				info.Workspaces = append(info.Workspaces, wStr)
			}
		}
	case map[string]interface{}:
		if packages, ok := ws["packages"].([]interface{}); ok {
			for _, pkg := range packages {
				if pkgStr, ok := pkg.(string); ok {
					info.Workspaces = append(info.Workspaces, pkgStr)
				}
			}
		}
	}
}

func (p *PackageJsonParser) parseStringMap(raw map[string]interface{}, key string, target map[string]string) {
	if obj, ok := raw[key].(map[string]interface{}); ok {
		p.parseStringMapFromInterface(obj, target)
	}
}

func (p *PackageJsonParser) parseStringMapFromInterface(obj map[string]interface{}, target map[string]string) {
	for k, v := range obj {
		if vStr, ok := v.(string); ok {
			target[k] = vStr
		}
	}
}

func (p *PackageJsonParser) isValidPackageName(name string) bool {
	// Basic npm package name validation
	if len(name) == 0 || len(name) > 214 {
		return false
	}
	
	// Check for valid characters
	validNameRegex := regexp.MustCompile(`^(?:@[a-z0-9-*~][a-z0-9-*._~]*/)?[a-z0-9-~][a-z0-9-._~]*$`)
	return validNameRegex.MatchString(name)
}

func (p *PackageJsonParser) isValidSemVer(version string) bool {
	// Basic semantic version validation
	semverRegex := regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)
	return semverRegex.MatchString(version)
}

func (p *PackageJsonParser) isValidVersionRange(version string) bool {
	// Basic version range validation (simplified)
	if version == "" || version == "*" {
		return true
	}
	
	// Handle ranges like ^1.0.0, ~1.0.0, >=1.0.0, etc.
	rangeRegex := regexp.MustCompile(`^([~^>=<]*)(\d+)\.(\d+)\.(\d+)`)
	return rangeRegex.MatchString(version) || p.isValidSemVer(version)
}

func (t *TsConfigParser) removeJSONComments(jsonStr string) string {
	// Remove single-line comments
	lines := strings.Split(jsonStr, "\n")
	var cleanedLines []string

	for _, line := range lines {
		// Remove single-line comments (// ...)
		if idx := strings.Index(line, "//"); idx != -1 {
			line = line[:idx]
		}
		cleanedLines = append(cleanedLines, line)
	}

	cleanedStr := strings.Join(cleanedLines, "\n")

	// Remove multi-line comments (/* ... */)
	multiLineRegex := regexp.MustCompile(`(?s)/\*.*?\*/`)
	cleanedStr = multiLineRegex.ReplaceAllString(cleanedStr, "")

	return cleanedStr
}

func (l *LockfileParser) parseNPMDependencies(deps map[string]interface{}, target map[string]*LockedDependency) {
	for name, depData := range deps {
		if depMap, ok := depData.(map[string]interface{}); ok {
			dep := &LockedDependency{
				Name:         name,
				Dependencies: make(map[string]string),
			}

			if version, ok := depMap["version"].(string); ok {
				dep.Version = version
			}
			if resolved, ok := depMap["resolved"].(string); ok {
				dep.Resolved = resolved
			}
			if integrity, ok := depMap["integrity"].(string); ok {
				dep.Integrity = integrity
			}
			if dev, ok := depMap["dev"].(bool); ok {
				dep.DevDependency = dev
			}
			if optional, ok := depMap["optional"].(bool); ok {
				dep.Optional = optional
			}

			// Parse nested dependencies
			if nestedDeps, ok := depMap["dependencies"].(map[string]interface{}); ok {
				nestedTarget := make(map[string]*LockedDependency)
				l.parseNPMDependencies(nestedDeps, nestedTarget)
				for nestedName, nestedDep := range nestedTarget {
					dep.Dependencies[nestedName] = nestedDep.Version
				}
			}

			target[name] = dep
		}
	}
}

func (l *LockfileParser) parseYarnPackageName(nameVersion string) string {
	// Handle package@version format
	atIndex := strings.Index(nameVersion, "@")
	if atIndex > 0 {
		return nameVersion[:atIndex]
	}
	// Handle scoped packages @scope/package@version
	if strings.HasPrefix(nameVersion, "@") {
		parts := strings.SplitN(nameVersion[1:], "@", 2)
		if len(parts) >= 1 {
			return "@" + parts[0]
		}
	}
	return nameVersion
}

func (l *LockfileParser) parseYarnProperty(line string, dep *LockedDependency) {
	line = strings.TrimSpace(line)
	
	if strings.HasPrefix(line, "version ") {
		dep.Version = strings.TrimSpace(strings.TrimPrefix(line, "version "))
		dep.Version = strings.Trim(dep.Version, `"`)
	} else if strings.HasPrefix(line, "resolved ") {
		dep.Resolved = strings.TrimSpace(strings.TrimPrefix(line, "resolved "))
		dep.Resolved = strings.Trim(dep.Resolved, `"`)
	} else if strings.HasPrefix(line, "integrity ") {
		dep.Integrity = strings.TrimSpace(strings.TrimPrefix(line, "integrity "))
		dep.Integrity = strings.Trim(dep.Integrity, `"`)
	} else if strings.Contains(line, ":") {
		// Handle dependency lines
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			value = strings.Trim(value, `"`)
			if key != "" && value != "" {
				dep.Dependencies[key] = value
			}
		}
	}
}

// Utility functions

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}