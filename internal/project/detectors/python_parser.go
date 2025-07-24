package detectors

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"lsp-gateway/internal/project/types"
	"lsp-gateway/internal/setup"
)

// PythonConfigParser provides comprehensive Python configuration parsing
type PythonConfigParser struct {
	logger *setup.SetupLogger
}

// PyProjectInfo contains parsed pyproject.toml information (PEP 518/621)
type PyProjectInfo struct {
	BuildSystem     BuildSystemInfo            `json:"build_system,omitempty"`
	Project         ProjectInfo                `json:"project,omitempty"`
	Tool            ToolInfo                   `json:"tool,omitempty"`
	Dependencies    map[string]*types.PythonDependencyInfo `json:"dependencies,omitempty"`
	DevDependencies map[string]*types.PythonDependencyInfo `json:"dev_dependencies,omitempty"`
}

// BuildSystemInfo contains build system configuration
type BuildSystemInfo struct {
	Requires     []string `json:"requires,omitempty"`
	BuildBackend string   `json:"build_backend,omitempty"`
}

// ProjectInfo contains PEP 621 project metadata
type ProjectInfo struct {
	Name            string            `json:"name,omitempty"`
	Version         string            `json:"version,omitempty"`
	Description     string            `json:"description,omitempty"`
	ReadmeFile      string            `json:"readme_file,omitempty"`
	RequiresPython  string            `json:"requires_python,omitempty"`
	License         map[string]string `json:"license,omitempty"`
	Authors         []PersonInfo      `json:"authors,omitempty"`
	Maintainers     []PersonInfo      `json:"maintainers,omitempty"`
	Keywords        []string          `json:"keywords,omitempty"`
	Classifiers     []string          `json:"classifiers,omitempty"`
	URLs            map[string]string `json:"urls,omitempty"`
	EntryPoints     map[string]string `json:"entry_points,omitempty"`
	Dependencies    []string          `json:"dependencies,omitempty"`
	OptionalDeps    map[string][]string `json:"optional_dependencies,omitempty"`
}

// PersonInfo contains author/maintainer information
type PersonInfo struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
}

// ToolInfo contains tool-specific configurations
type ToolInfo struct {
	Poetry     *PoetryConfig     `json:"poetry,omitempty"`
	PDM        *PDMConfig        `json:"pdm,omitempty"`
	Setuptools *SetuptoolsConfig `json:"setuptools,omitempty"`
	Black      *BlackConfig      `json:"black,omitempty"`
	Pytest     *PytestConfig     `json:"pytest,omitempty"`
	MyPy       *MyPyConfig       `json:"mypy,omitempty"`
}

// PoetryConfig contains Poetry-specific configuration
type PoetryConfig struct {
	Name            string                     `json:"name,omitempty"`
	Version         string                     `json:"version,omitempty"`
	Description     string                     `json:"description,omitempty"`
	Authors         []string                   `json:"authors,omitempty"`
	License         string                     `json:"license,omitempty"`
	ReadmeFile      string                     `json:"readme,omitempty"`
	Homepage        string                     `json:"homepage,omitempty"`
	Repository      string                     `json:"repository,omitempty"`
	Keywords        []string                   `json:"keywords,omitempty"`
	Classifiers     []string                   `json:"classifiers,omitempty"`
	Dependencies    map[string]interface{}     `json:"dependencies,omitempty"`
	DevDependencies map[string]interface{}     `json:"dev_dependencies,omitempty"`
	Groups          map[string]map[string]interface{} `json:"groups,omitempty"`
}

// PDMConfig contains PDM-specific configuration
type PDMConfig struct {
	DevDependencies map[string][]string `json:"dev_dependencies,omitempty"`
}

// SetuptoolsConfig contains setuptools-specific configuration
type SetuptoolsConfig struct {
	PackageDir  map[string]string `json:"package_dir,omitempty"`
	PackageData map[string][]string `json:"package_data,omitempty"`
	DataFiles   []interface{}     `json:"data_files,omitempty"`
}

// BlackConfig contains Black formatter configuration
type BlackConfig struct {
	LineLength int      `json:"line_length,omitempty"`
	Include    string   `json:"include,omitempty"`
	Exclude    string   `json:"exclude,omitempty"`
	Extend     []string `json:"extend_exclude,omitempty"`
}

// PytestConfig contains pytest configuration
type PytestConfig struct {
	TestPaths     []string          `json:"testpaths,omitempty"`
	PythonFiles   []string          `json:"python_files,omitempty"`
	PythonClasses []string          `json:"python_classes,omitempty"`
	AddOpts       []string          `json:"addopts,omitempty"`
	Markers       map[string]string `json:"markers,omitempty"`
}

// MyPyConfig contains MyPy type checker configuration
type MyPyConfig struct {
	PythonVersion        string   `json:"python_version,omitempty"`
	WarnReturnAny        bool     `json:"warn_return_any,omitempty"`
	WarnUnusedConfigs    bool     `json:"warn_unused_configs,omitempty"`
	DisallowUntyped      bool     `json:"disallow_untyped_defs,omitempty"`
	Files                []string `json:"files,omitempty"`
	ExcludePatterns      []string `json:"exclude,omitempty"`
}

// SetupPyInfo contains parsed setup.py information
type SetupPyInfo struct {
	Name            string            `json:"name,omitempty"`
	Version         string            `json:"version,omitempty"`
	Description     string            `json:"description,omitempty"`
	LongDescription string            `json:"long_description,omitempty"`
	Author          string            `json:"author,omitempty"`
	AuthorEmail     string            `json:"author_email,omitempty"`
	URL             string            `json:"url,omitempty"`
	License         string            `json:"license,omitempty"`
	Packages        []string          `json:"packages,omitempty"`
	Dependencies    map[string]*types.PythonDependencyInfo `json:"install_requires,omitempty"`
	ExtraDeps       map[string]map[string]*types.PythonDependencyInfo `json:"extras_require,omitempty"`
	PythonRequires  string            `json:"python_requires,omitempty"`
	EntryPoints     map[string]string `json:"entry_points,omitempty"`
	Classifiers     []string          `json:"classifiers,omitempty"`
}

// PipfileInfo contains parsed Pipfile information
type PipfileInfo struct {
	Source       []PipfileSource            `json:"source,omitempty"`
	Packages     map[string]*types.PythonDependencyInfo `json:"packages,omitempty"`
	DevPackages  map[string]*types.PythonDependencyInfo `json:"dev_packages,omitempty"`
	RequiresPython string                   `json:"requires_python,omitempty"`
}

// PipfileSource contains Pipfile source configuration
type PipfileSource struct {
	Name   string `json:"name,omitempty"`
	URL    string `json:"url,omitempty"`
	Verify bool   `json:"verify_ssl,omitempty"`
}

// NewPythonConfigParser creates a new Python configuration parser
func NewPythonConfigParser() *PythonConfigParser {
	return &PythonConfigParser{
		logger: setup.NewSetupLogger(&setup.SetupLoggerConfig{Component: "python-parser"}),
	}
}

// ParsePyproject parses a pyproject.toml file (PEP 518/621)
func (p *PythonConfigParser) ParsePyproject(ctx context.Context, path string) (*PyProjectInfo, error) {
	p.logger.Debug(fmt.Sprintf("Parsing pyproject.toml at path: %s", path))

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read pyproject.toml: %w", err)
	}

	var rawData map[string]interface{}
	if err := toml.Unmarshal(content, &rawData); err != nil {
		return nil, fmt.Errorf("failed to parse TOML: %w", err)
	}

	info := &PyProjectInfo{
		Dependencies:    make(map[string]*types.PythonDependencyInfo),
		DevDependencies: make(map[string]*types.PythonDependencyInfo),
	}

	// Parse build-system section
	if buildSystem, ok := rawData["build-system"].(map[string]interface{}); ok {
		info.BuildSystem = p.parseBuildSystem(buildSystem)
	}

	// Parse project section (PEP 621)
	if project, ok := rawData["project"].(map[string]interface{}); ok {
		info.Project = p.parseProjectInfo(project)
		
		// Extract dependencies from project section
		if deps, ok := project["dependencies"].([]interface{}); ok {
			p.parseDependencyList(deps, info.Dependencies, "pyproject.toml")
		}

		if optionalDeps, ok := project["optional-dependencies"].(map[string]interface{}); ok {
			for group, deps := range optionalDeps {
				if depList, ok := deps.([]interface{}); ok {
					if group == types.SCOPE_DEV || group == "development" || group == types.SCOPE_TEST {
						p.parseDependencyList(depList, info.DevDependencies, "pyproject.toml")
					} else {
						p.parseDependencyList(depList, info.Dependencies, "pyproject.toml")
					}
				}
			}
		}
	}

	// Parse tool section
	if tool, ok := rawData["tool"].(map[string]interface{}); ok {
		info.Tool = p.parseToolInfo(tool)
		
		// Extract Poetry dependencies
		if poetry, ok := tool["poetry"].(map[string]interface{}); ok {
			if deps, ok := poetry["dependencies"].(map[string]interface{}); ok {
				p.parsePoetryDependencies(deps, info.Dependencies, "pyproject.toml")
			}
			if devDeps, ok := poetry["dev-dependencies"].(map[string]interface{}); ok {
				p.parsePoetryDependencies(devDeps, info.DevDependencies, "pyproject.toml")
			}
			// Handle Poetry groups
			if groups, ok := poetry["group"].(map[string]interface{}); ok {
				for groupName, groupData := range groups {
					if groupMap, ok := groupData.(map[string]interface{}); ok {
						if deps, ok := groupMap["dependencies"].(map[string]interface{}); ok {
							if groupName == types.SCOPE_DEV || groupName == "test" {
								p.parsePoetryDependencies(deps, info.DevDependencies, "pyproject.toml")
							} else {
								p.parsePoetryDependencies(deps, info.Dependencies, "pyproject.toml")
							}
						}
					}
				}
			}
		}

		// Extract PDM dev dependencies
		if pdm, ok := tool["pdm"].(map[string]interface{}); ok {
			if devDeps, ok := pdm["dev-dependencies"].(map[string]interface{}); ok {
				for group, deps := range devDeps {
					if depList, ok := deps.([]interface{}); ok {
						if group == types.SCOPE_DEV || group == "test" {
							p.parseDependencyList(depList, info.DevDependencies, "pyproject.toml")
						}
					}
				}
			}
		}
	}

	p.logger.Debug(fmt.Sprintf("Parsed pyproject.toml successfully - dependencies: %d, dev_dependencies: %d", len(info.Dependencies), len(info.DevDependencies)))

	return info, nil
}

// ParseSetupPy parses a setup.py file (legacy format)
func (p *PythonConfigParser) ParseSetupPy(ctx context.Context, path string) (*SetupPyInfo, error) {
	p.logger.Debug(fmt.Sprintf("Parsing setup.py at path: %s", path))

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read setup.py: %w", err)
	}

	info := &SetupPyInfo{
		Dependencies: make(map[string]*types.PythonDependencyInfo),
		ExtraDeps:    make(map[string]map[string]*types.PythonDependencyInfo),
		EntryPoints:  make(map[string]string),
	}

	// Parse setup.py using regex patterns (simple approach)
	contentStr := string(content)
	
	// Extract basic metadata
	info.Name = p.extractSetupValue(contentStr, "name")
	info.Version = p.extractSetupValue(contentStr, "version")
	info.Description = p.extractSetupValue(contentStr, "description")
	info.LongDescription = p.extractSetupValue(contentStr, "long_description")
	info.Author = p.extractSetupValue(contentStr, "author")
	info.AuthorEmail = p.extractSetupValue(contentStr, "author_email")
	info.URL = p.extractSetupValue(contentStr, "url")
	info.License = p.extractSetupValue(contentStr, "license")
	info.PythonRequires = p.extractSetupValue(contentStr, "python_requires")

	// Extract install_requires dependencies
	if installRequires := p.extractSetupList(contentStr, "install_requires"); len(installRequires) > 0 {
		for _, dep := range installRequires {
			depInfo := p.parseSingleDependency(dep, "setup.py")
			info.Dependencies[depInfo.Name] = depInfo
		}
	}

	// Extract extras_require
	if extrasRequire := p.extractSetupDict(contentStr, "extras_require"); len(extrasRequire) > 0 {
		for extra, deps := range extrasRequire {
			extraDeps := make(map[string]*types.PythonDependencyInfo)
			for _, dep := range deps {
				depInfo := p.parseSingleDependency(dep, "setup.py")
				extraDeps[depInfo.Name] = depInfo
			}
			info.ExtraDeps[extra] = extraDeps
		}
	}

	// Extract packages
	if packages := p.extractSetupList(contentStr, "packages"); len(packages) > 0 {
		info.Packages = packages
	}

	// Extract classifiers
	if classifiers := p.extractSetupList(contentStr, "classifiers"); len(classifiers) > 0 {
		info.Classifiers = classifiers
	}

	p.logger.Debug(fmt.Sprintf("Parsed setup.py successfully - dependencies: %d, extras: %d", len(info.Dependencies), len(info.ExtraDeps)))

	return info, nil
}

// ParseRequirements parses a requirements.txt file
func (p *PythonConfigParser) ParseRequirements(ctx context.Context, path string) (map[string]*types.PythonDependencyInfo, error) {
	p.logger.Debug(fmt.Sprintf("Parsing requirements.txt at path: %s", path))

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open requirements.txt: %w", err)
	}
	defer func() { _ = file.Close() }()

	dependencies := make(map[string]*types.PythonDependencyInfo)
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Skip -r includes and other pip options
		if strings.HasPrefix(line, "-") {
			continue
		}

		// Parse dependency
		depInfo := p.parseSingleDependency(line, "requirements.txt")
		if depInfo.Name != "" {
			dependencies[depInfo.Name] = depInfo
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading requirements.txt: %w", err)
	}

	p.logger.Debug(fmt.Sprintf("Parsed requirements.txt successfully - dependencies: %d", len(dependencies)))
	return dependencies, nil
}

// ParsePipfile parses a Pipfile
func (p *PythonConfigParser) ParsePipfile(ctx context.Context, path string) (map[string]*types.PythonDependencyInfo, map[string]*types.PythonDependencyInfo, error) {
	p.logger.Debug(fmt.Sprintf("Parsing Pipfile at path: %s", path))

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read Pipfile: %w", err)
	}

	var rawData map[string]interface{}
	if err := toml.Unmarshal(content, &rawData); err != nil {
		return nil, nil, fmt.Errorf("failed to parse Pipfile TOML: %w", err)
	}

	dependencies := make(map[string]*types.PythonDependencyInfo)
	devDependencies := make(map[string]*types.PythonDependencyInfo)

	// Parse packages section
	if packages, ok := rawData["packages"].(map[string]interface{}); ok {
		p.parsePipfileDependencies(packages, dependencies, "Pipfile")
	}

	// Parse dev-packages section
	if devPackages, ok := rawData["dev-packages"].(map[string]interface{}); ok {
		p.parsePipfileDependencies(devPackages, devDependencies, "Pipfile")
	}

	p.logger.Debug(fmt.Sprintf("Parsed Pipfile successfully - dependencies: %d, dev_dependencies: %d", len(dependencies), len(devDependencies)))

	return dependencies, devDependencies, nil
}

// Helper methods for parsing

func (p *PythonConfigParser) parseBuildSystem(data map[string]interface{}) BuildSystemInfo {
	buildSystem := BuildSystemInfo{}
	
	if requires, ok := data["requires"].([]interface{}); ok {
		for _, req := range requires {
			if reqStr, ok := req.(string); ok {
				buildSystem.Requires = append(buildSystem.Requires, reqStr)
			}
		}
	}
	
	if backend, ok := data["build-backend"].(string); ok {
		buildSystem.BuildBackend = backend
	}
	
	return buildSystem
}

func (p *PythonConfigParser) parseProjectInfo(data map[string]interface{}) ProjectInfo {
	project := ProjectInfo{
		License:      make(map[string]string),
		URLs:         make(map[string]string),
		EntryPoints:  make(map[string]string),
		OptionalDeps: make(map[string][]string),
	}

	if name, ok := data["name"].(string); ok {
		project.Name = name
	}
	if version, ok := data["version"].(string); ok {
		project.Version = version
	}
	if desc, ok := data["description"].(string); ok {
		project.Description = desc
	}
	if readme, ok := data["readme"].(string); ok {
		project.ReadmeFile = readme
	}
	if python, ok := data["requires-python"].(string); ok {
		project.RequiresPython = python
	}

	// Parse license
	if license, ok := data["license"].(map[string]interface{}); ok {
		for k, v := range license {
			if vStr, ok := v.(string); ok {
				project.License[k] = vStr
			}
		}
	}

	// Parse authors
	if authors, ok := data["authors"].([]interface{}); ok {
		for _, author := range authors {
			if authorMap, ok := author.(map[string]interface{}); ok {
				person := PersonInfo{}
				if name, ok := authorMap["name"].(string); ok {
					person.Name = name
				}
				if email, ok := authorMap["email"].(string); ok {
					person.Email = email
				}
				project.Authors = append(project.Authors, person)
			}
		}
	}

	// Parse keywords
	if keywords, ok := data["keywords"].([]interface{}); ok {
		for _, keyword := range keywords {
			if keywordStr, ok := keyword.(string); ok {
				project.Keywords = append(project.Keywords, keywordStr)
			}
		}
	}

	// Parse classifiers
	if classifiers, ok := data["classifiers"].([]interface{}); ok {
		for _, classifier := range classifiers {
			if classifierStr, ok := classifier.(string); ok {
				project.Classifiers = append(project.Classifiers, classifierStr)
			}
		}
	}

	// Parse URLs
	if urls, ok := data["urls"].(map[string]interface{}); ok {
		for k, v := range urls {
			if vStr, ok := v.(string); ok {
				project.URLs[k] = vStr
			}
		}
	}

	return project
}

func (p *PythonConfigParser) parseToolInfo(data map[string]interface{}) ToolInfo {
	tool := ToolInfo{}

	// Parse Poetry configuration
	if poetry, ok := data["poetry"].(map[string]interface{}); ok {
		tool.Poetry = p.parsePoetryConfig(poetry)
	}

	// Parse PDM configuration
	if pdm, ok := data["pdm"].(map[string]interface{}); ok {
		tool.PDM = p.parsePDMConfig(pdm)
	}

	// Parse Black configuration
	if black, ok := data["black"].(map[string]interface{}); ok {
		tool.Black = p.parseBlackConfig(black)
	}

	// Parse pytest configuration
	if pytest, ok := data["pytest"].(map[string]interface{}); ok {
		tool.Pytest = p.parsePytestConfig(pytest)
	}

	// Parse MyPy configuration
	if mypy, ok := data["mypy"].(map[string]interface{}); ok {
		tool.MyPy = p.parseMyPyConfig(mypy)
	}

	return tool
}

func (p *PythonConfigParser) parsePoetryConfig(data map[string]interface{}) *PoetryConfig {
	config := &PoetryConfig{
		Dependencies:    make(map[string]interface{}),
		DevDependencies: make(map[string]interface{}),
		Groups:          make(map[string]map[string]interface{}),
	}

	if name, ok := data["name"].(string); ok {
		config.Name = name
	}
	if version, ok := data["version"].(string); ok {
		config.Version = version
	}
	if desc, ok := data["description"].(string); ok {
		config.Description = desc
	}

	// Parse authors
	if authors, ok := data["authors"].([]interface{}); ok {
		for _, author := range authors {
			if authorStr, ok := author.(string); ok {
				config.Authors = append(config.Authors, authorStr)
			}
		}
	}

	if license, ok := data["license"].(string); ok {
		config.License = license
	}
	if readme, ok := data["readme"].(string); ok {
		config.ReadmeFile = readme
	}
	if homepage, ok := data["homepage"].(string); ok {
		config.Homepage = homepage
	}
	if repo, ok := data["repository"].(string); ok {
		config.Repository = repo
	}

	// Parse dependencies
	if deps, ok := data["dependencies"].(map[string]interface{}); ok {
		config.Dependencies = deps
	}
	if devDeps, ok := data["dev-dependencies"].(map[string]interface{}); ok {
		config.DevDependencies = devDeps
	}

	return config
}

func (p *PythonConfigParser) parsePDMConfig(data map[string]interface{}) *PDMConfig {
	config := &PDMConfig{
		DevDependencies: make(map[string][]string),
	}

	if devDeps, ok := data["dev-dependencies"].(map[string]interface{}); ok {
		for group, deps := range devDeps {
			if depList, ok := deps.([]interface{}); ok {
				var strDeps []string
				for _, dep := range depList {
					if depStr, ok := dep.(string); ok {
						strDeps = append(strDeps, depStr)
					}
				}
				config.DevDependencies[group] = strDeps
			}
		}
	}

	return config
}

func (p *PythonConfigParser) parseBlackConfig(data map[string]interface{}) *BlackConfig {
	config := &BlackConfig{}

	if lineLength, ok := data["line-length"].(int64); ok {
		config.LineLength = int(lineLength)
	}
	if include, ok := data["include"].(string); ok {
		config.Include = include
	}
	if exclude, ok := data["exclude"].(string); ok {
		config.Exclude = exclude
	}

	return config
}

func (p *PythonConfigParser) parsePytestConfig(data map[string]interface{}) *PytestConfig {
	config := &PytestConfig{
		Markers: make(map[string]string),
	}

	if testPaths, ok := data["testpaths"].([]interface{}); ok {
		for _, path := range testPaths {
			if pathStr, ok := path.(string); ok {
				config.TestPaths = append(config.TestPaths, pathStr)
			}
		}
	}

	return config
}

func (p *PythonConfigParser) parseMyPyConfig(data map[string]interface{}) *MyPyConfig {
	config := &MyPyConfig{}

	if pythonVersion, ok := data["python_version"].(string); ok {
		config.PythonVersion = pythonVersion
	}
	if warnReturn, ok := data["warn_return_any"].(bool); ok {
		config.WarnReturnAny = warnReturn
	}

	return config
}

func (p *PythonConfigParser) parseDependencyList(deps []interface{}, target map[string]*types.PythonDependencyInfo, source string) {
	for _, dep := range deps {
		if depStr, ok := dep.(string); ok {
			depInfo := p.parseSingleDependency(depStr, source)
			if depInfo.Name != "" {
				target[depInfo.Name] = depInfo
			}
		}
	}
}

func (p *PythonConfigParser) parsePoetryDependencies(deps map[string]interface{}, target map[string]*types.PythonDependencyInfo, source string) {
	for name, constraint := range deps {
		depInfo := &types.PythonDependencyInfo{
			Name:   name,
			Source: source,
		}

		switch v := constraint.(type) {
		case string:
			// Simple version constraint: "package" = "^1.0.0"
			depInfo.Specifier = v
			depInfo.Version = p.extractVersionFromSpecifier(v)
		case map[string]interface{}:
			// Complex constraint: "package" = { version = "^1.0.0", extras = ["dev"] }
			if version, ok := v["version"].(string); ok {
				depInfo.Specifier = version
				depInfo.Version = p.extractVersionFromSpecifier(version)
			}
			if extras, ok := v["extras"].([]interface{}); ok {
				for _, extra := range extras {
					if extraStr, ok := extra.(string); ok {
						depInfo.Extras = append(depInfo.Extras, extraStr)
					}
				}
			}
		}

		target[name] = depInfo
	}
}

func (p *PythonConfigParser) parsePipfileDependencies(deps map[string]interface{}, target map[string]*types.PythonDependencyInfo, source string) {
	for name, constraint := range deps {
		depInfo := &types.PythonDependencyInfo{
			Name:   name,
			Source: source,
		}

		switch v := constraint.(type) {
		case string:
			// Simple version: "package" = "*" or "package" = "==1.0.0"
			depInfo.Specifier = v
			depInfo.Version = p.extractVersionFromSpecifier(v)
		case map[string]interface{}:
			// Complex constraint with version, extras, etc.
			if version, ok := v["version"].(string); ok {
				depInfo.Specifier = version
				depInfo.Version = p.extractVersionFromSpecifier(version)
			}
		}

		target[name] = depInfo
	}
}

func (p *PythonConfigParser) parseSingleDependency(depStr, source string) *types.PythonDependencyInfo {
	depInfo := &types.PythonDependencyInfo{Source: source}

	// Remove whitespace and comments
	depStr = strings.TrimSpace(strings.Split(depStr, "#")[0])
	if depStr == "" {
		return depInfo
	}

	// Handle URLs (git+https://, https://, etc.)
	if strings.Contains(depStr, "://") {
		// Extract package name from URL if possible
		if strings.Contains(depStr, "#egg=") {
			parts := strings.Split(depStr, "#egg=")
			if len(parts) > 1 {
				depInfo.Name = strings.Split(parts[1], "&")[0]
			}
		}
		depInfo.Specifier = depStr
		return depInfo
	}

	// Handle local paths
	if strings.HasPrefix(depStr, "-e ") || strings.HasPrefix(depStr, "./") || strings.HasPrefix(depStr, "../") {
		depInfo.Specifier = depStr
		return depInfo
	}

	// Parse standard dependency format: package[extra1,extra2]>=1.0.0,<2.0.0
	depRegex := regexp.MustCompile(`^([a-zA-Z0-9_-]+)(?:\[([^\]]+)\])?(.*)$`)
	matches := depRegex.FindStringSubmatch(depStr)
	
	if len(matches) > 1 {
		depInfo.Name = matches[1]
		
		// Parse extras
		if len(matches) > 2 && matches[2] != "" {
			extras := strings.Split(matches[2], ",")
			for _, extra := range extras {
				depInfo.Extras = append(depInfo.Extras, strings.TrimSpace(extra))
			}
		}
		
		// Parse version specifier
		if len(matches) > 3 && matches[3] != "" {
			depInfo.Specifier = strings.TrimSpace(matches[3])
			depInfo.Version = p.extractVersionFromSpecifier(depInfo.Specifier)
		}
	} else {
		// Fallback: treat entire string as package name
		depInfo.Name = depStr
	}

	return depInfo
}

func (p *PythonConfigParser) extractVersionFromSpecifier(specifier string) string {
	// Extract version from specifiers like ">=1.0.0", "^1.0.0", "~=1.0.0", "==1.0.0"
	versionRegex := regexp.MustCompile(`[~^>=<!]*([\d\.]+)`)
	matches := versionRegex.FindStringSubmatch(specifier)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// Setup.py parsing helpers
func (p *PythonConfigParser) extractSetupValue(content, key string) string {
	// Simple regex-based extraction for setup() arguments
	pattern := fmt.Sprintf(`%s\s*=\s*['"](.*?)['"]`, key)
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(content)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func (p *PythonConfigParser) extractSetupList(content, key string) []string {
	// Extract list values from setup.py
	pattern := fmt.Sprintf(`%s\s*=\s*\[(.*?)\]`, key)
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(content)
	if len(matches) > 1 {
		listContent := matches[1]
		// Simple parsing - split by comma and clean up
		items := strings.Split(listContent, ",")
		var result []string
		for _, item := range items {
			cleaned := strings.Trim(strings.TrimSpace(item), `'"`)
			if cleaned != "" {
				result = append(result, cleaned)
			}
		}
		return result
	}
	return nil
}

func (p *PythonConfigParser) extractSetupDict(content, key string) map[string][]string {
	// Extract dictionary values from setup.py (simplified)
	pattern := fmt.Sprintf(`%s\s*=\s*\{(.*?)\}`, key)
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(content)
	if len(matches) > 1 {
		// This is a simplified implementation
		// In practice, you'd want a more robust parser
		return make(map[string][]string)
	}
	return make(map[string][]string)
}