package gateway

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// RoutingStrategyType defines different routing strategy types
type RoutingStrategyType string

const (
	RoutingStrategySingle                 RoutingStrategyType = "single"
	RoutingStrategyFirst                  RoutingStrategyType = "first"
	RoutingStrategyAggregate              RoutingStrategyType = "aggregate"
	RoutingStrategyMulti                  RoutingStrategyType = "multi"
	RoutingStrategyBroadcast              RoutingStrategyType = "broadcast"
	RoutingStrategyLoadBalanced           RoutingStrategyType = "load_balanced"
	RoutingStrategyRoundRobin             RoutingStrategyType = "round_robin"
	RoutingStrategyPrimaryWithEnhancement RoutingStrategyType = "primary_with_enhancement"
)

// RequestLanguageContext contains language-specific context for a request
type RequestLanguageContext struct {
	PrimaryLanguage    string                    `json:"primary_language"`
	DetectedLanguages  []string                  `json:"detected_languages"`
	SecondaryLanguages []string                  `json:"secondary_languages"`
	EmbeddedLanguages  map[string][]ContentRange `json:"embedded_languages"`
	FileExtension      string                    `json:"file_extension"`
	ContentType        string                    `json:"content_type"`
	ProjectType        string                    `json:"project_type"`
	ProjectLanguages   []string                  `json:"project_languages"`
	Framework          string                    `json:"framework"`
	TemplateEngine     string                    `json:"template_engine"`
	IsMultiLanguage    bool                      `json:"is_multi_language"`
	LanguageFeatures   map[string]bool           `json:"language_features"`
	LanguageConfidence float64                   `json:"language_confidence"`
	AdditionalContext  map[string]string         `json:"additional_context"`
}

// RequestClassifier analyzes LSP requests for intelligent routing decisions
type RequestClassifier interface {
	ClassifyRequest(request *JSONRPCRequest, uri string) (*RequestClassificationResult, error)
	AnalyzeRequestType(method string, params interface{}) *RequestTypeInfo
	ExtractRequestLanguageContext(uri string, params interface{}) (*RequestLanguageContext, error)
	DetermineWorkspaceContext(uri string) (*RequestWorkspaceContext, error)
	DetectCrossLanguageNeeds(method string, context *RequestLanguageContext) bool
}

// RequestClassificationResult contains comprehensive request analysis results
type RequestClassificationResult struct {
	Request                *JSONRPCRequest
	TypeInfo               *RequestTypeInfo
	RequestLanguageContext *RequestLanguageContext
	WorkspaceContext       *RequestWorkspaceContext
	CrossLanguage          bool
	RoutingHints           *RoutingHints
	CacheKey               string
	Timestamp              time.Time
}

// RequestTypeInfo provides detailed information about the LSP request type
type RequestTypeInfo struct {
	Method               string
	Category             string // "definition", "reference", "symbol", "hover", "diagnostic", "completion", "formatting"
	RequiresFileAccess   bool
	SupportsCrossLang    bool
	SupportsAggregation  bool
	DefaultStrategy      RoutingStrategyType
	Priority             int
	ExpectedResponseType string
	TimeoutHint          time.Duration
	CacheableResponse    bool
}

// ContentRange represents a range of embedded language content
type ContentRange struct {
	StartLine  int
	StartChar  int
	EndLine    int
	EndChar    int
	Language   string
	Confidence float64
}

// RequestWorkspaceContext provides workspace and project information
type RequestWorkspaceContext struct {
	WorkspaceRoot      string
	ProjectType        string
	ProjectName        string
	SupportedLanguages []string
	ConfigFiles        []string
	IsMonorepo         bool
	SubProjects        []string
	Dependencies       map[string][]string
	BuildSystem        string
	PackageManager     string
	FrameworkInfo      *FrameworkInfo
}

// FrameworkInfo contains detected framework information
type FrameworkInfo struct {
	Name         string
	Version      string
	Type         string // "web", "api", "desktop", "mobile", "cli"
	ConfigFiles  []string
	EntryPoints  []string
	Dependencies []string
}

// RoutingHints provide optimization suggestions for request routing
type RoutingHints struct {
	PreferredServers    []string
	FallbackServers     []string
	AggregationStrategy string
	CacheStrategy       string
	Timeout             time.Duration
	Priority            int
	RequiresWorkspace   bool
	BenefitsFromCache   bool
}

// defaultRequestClassifier implements RequestClassifier
type defaultRequestClassifier struct {
	methodInfoCache map[string]*RequestTypeInfo
	workspaceCache  map[string]*RequestWorkspaceContext
	languageCache   map[string]*RequestLanguageContext
	cacheMutex      sync.RWMutex
	cacheExpiry     time.Duration
	maxCacheSize    int

	// Language detection patterns
	extensionMap       map[string]string
	contentPatterns    map[string]*regexp.Regexp
	configFilePatterns map[string]string
	frameworkDetectors map[string]*SimpleFrameworkDetector
}

// SimpleFrameworkDetector defines basic framework detection logic for request classification
type SimpleFrameworkDetector struct {
	Name            string
	ConfigFiles     []string
	Dependencies    []string
	FilePatterns    []string
	ContentPatterns []*regexp.Regexp
}

// NewRequestClassifier creates a new request classifier instance
func NewRequestClassifier() RequestClassifier {
	classifier := &defaultRequestClassifier{
		methodInfoCache:    make(map[string]*RequestTypeInfo),
		workspaceCache:     make(map[string]*RequestWorkspaceContext),
		languageCache:      make(map[string]*RequestLanguageContext),
		cacheExpiry:        5 * time.Minute,
		maxCacheSize:       1000,
		extensionMap:       initializeExtensionMap(),
		contentPatterns:    initializeContentPatterns(),
		configFilePatterns: initializeConfigFilePatterns(),
		frameworkDetectors: initializeFrameworkDetectors(),
	}

	// Pre-populate method classification cache
	classifier.initializeMethodCache()

	return classifier
}

// ClassifyRequest performs comprehensive request analysis
func (c *defaultRequestClassifier) ClassifyRequest(request *JSONRPCRequest, uri string) (*RequestClassificationResult, error) {
	if request == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	// Generate cache key
	cacheKey := c.generateCacheKey(request, uri)

	// Analyze request type
	typeInfo := c.AnalyzeRequestType(request.Method, request.Params)
	if typeInfo == nil {
		return nil, fmt.Errorf("unsupported method: %s", request.Method)
	}

	// Extract language context
	languageContext, err := c.ExtractRequestLanguageContext(uri, request.Params)
	if err != nil {
		return nil, fmt.Errorf("failed to extract language context: %w", err)
	}

	// Determine workspace context
	workspaceContext, err := c.DetermineWorkspaceContext(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to determine workspace context: %w", err)
	}

	// Detect cross-language needs
	crossLanguage := c.DetectCrossLanguageNeeds(request.Method, languageContext)

	// Generate routing hints
	routingHints := c.generateRoutingHints(typeInfo, languageContext, workspaceContext, crossLanguage)

	context := &RequestClassificationResult{
		Request:                request,
		TypeInfo:               typeInfo,
		RequestLanguageContext: languageContext,
		WorkspaceContext:       workspaceContext,
		CrossLanguage:          crossLanguage,
		RoutingHints:           routingHints,
		CacheKey:               cacheKey,
		Timestamp:              time.Now(),
	}

	return context, nil
}

// AnalyzeRequestType provides detailed information about the LSP method
func (c *defaultRequestClassifier) AnalyzeRequestType(method string, params interface{}) *RequestTypeInfo {
	c.cacheMutex.RLock()
	if cached, exists := c.methodInfoCache[method]; exists {
		c.cacheMutex.RUnlock()
		return cached
	}
	c.cacheMutex.RUnlock()

	// Create new type info for unknown methods
	typeInfo := c.classifyMethod(method, params)

	// Cache the result
	c.cacheMutex.Lock()
	if len(c.methodInfoCache) < c.maxCacheSize {
		c.methodInfoCache[method] = typeInfo
	}
	c.cacheMutex.Unlock()

	return typeInfo
}

// ExtractRequestLanguageContext analyzes language information from URI and params
func (c *defaultRequestClassifier) ExtractRequestLanguageContext(uri string, params interface{}) (*RequestLanguageContext, error) {
	if uri == "" {
		return &RequestLanguageContext{
			PrimaryLanguage:    "unknown",
			SecondaryLanguages: []string{},
			LanguageConfidence: 0.0,
			EmbeddedLanguages:  make(map[string][]ContentRange),
			TemplateEngine:     "",
		}, nil
	}

	// Check cache first
	cacheKey := fmt.Sprintf("lang:%s", uri)
	c.cacheMutex.RLock()
	if cached, exists := c.languageCache[cacheKey]; exists {
		c.cacheMutex.RUnlock()
		return cached, nil
	}
	c.cacheMutex.RUnlock()

	context, err := c.analyzeLanguageFromURI(uri)
	if err != nil {
		return nil, err
	}

	// Enhance with content analysis if file exists
	if filepath.IsAbs(uri) || strings.HasPrefix(uri, "file://") {
		filePath := strings.TrimPrefix(uri, "file://")
		if err := c.enhanceWithContentAnalysis(context, filePath); err != nil {
			// Log warning but don't fail
		}
	}

	// Enhance with project-level language detection
	if err := c.enhanceWithProjectLanguages(context, uri); err != nil {
		// Log warning but don't fail
	}

	// Cache the result
	c.cacheRequestLanguageContext(cacheKey, context)

	return context, nil
}

// DetermineWorkspaceContext analyzes workspace and project information
func (c *defaultRequestClassifier) DetermineWorkspaceContext(uri string) (*RequestWorkspaceContext, error) {
	if uri == "" {
		return &RequestWorkspaceContext{
			WorkspaceRoot: "",
			ProjectType:   "unknown",
		}, nil
	}

	workspaceRoot := c.findWorkspaceRoot(uri)
	cacheKey := fmt.Sprintf("workspace:%s", workspaceRoot)

	// Check cache
	c.cacheMutex.RLock()
	if cached, exists := c.workspaceCache[cacheKey]; exists {
		c.cacheMutex.RUnlock()
		return cached, nil
	}
	c.cacheMutex.RUnlock()

	context := &RequestWorkspaceContext{
		WorkspaceRoot: workspaceRoot,
	}

	if workspaceRoot != "" {
		if err := c.analyzeWorkspaceStructure(context); err != nil {
			return nil, fmt.Errorf("failed to analyze workspace structure: %w", err)
		}

		if err := c.detectProjectType(context); err != nil {
			// Log warning but continue
		}

		if err := c.detectFrameworks(context); err != nil {
			// Log warning but continue
		}

		if err := c.analyzeDependencies(context); err != nil {
			// Log warning but continue
		}
	}

	// Cache the result
	c.cacheMutex.Lock()
	if len(c.workspaceCache) < c.maxCacheSize {
		c.workspaceCache[cacheKey] = context
	}
	c.cacheMutex.Unlock()

	return context, nil
}

// DetectCrossLanguageNeeds determines if request benefits from multi-language analysis
func (c *defaultRequestClassifier) DetectCrossLanguageNeeds(method string, context *RequestLanguageContext) bool {
	if context == nil {
		return false
	}

	// Methods that commonly benefit from cross-language analysis
	crossLangMethods := map[string]bool{
		"textDocument/definition":     true,
		"textDocument/references":     true,
		"workspace/symbol":            true,
		"textDocument/implementation": true,
		"textDocument/typeDefinition": true,
	}

	// Check if method supports cross-language
	if !crossLangMethods[method] {
		return false
	}

	// Check language context indicators
	return context.IsMultiLanguage ||
		len(context.SecondaryLanguages) > 0 ||
		len(context.EmbeddedLanguages) > 0 ||
		context.TemplateEngine != ""
}

// Helper methods for initialization and analysis

func (c *defaultRequestClassifier) initializeMethodCache() {
	methods := map[string]*RequestTypeInfo{
		"textDocument/definition": {
			Method:               "textDocument/definition",
			Category:             "definition",
			RequiresFileAccess:   true,
			SupportsCrossLang:    true,
			SupportsAggregation:  true,
			DefaultStrategy:      RoutingStrategyFirst,
			Priority:             5,
			ExpectedResponseType: "Location[]",
			TimeoutHint:          5 * time.Second,
			CacheableResponse:    true,
		},
		"textDocument/references": {
			Method:               "textDocument/references",
			Category:             "reference",
			RequiresFileAccess:   true,
			SupportsCrossLang:    true,
			SupportsAggregation:  true,
			DefaultStrategy:      RoutingStrategyAggregate,
			Priority:             4,
			ExpectedResponseType: "Location[]",
			TimeoutHint:          10 * time.Second,
			CacheableResponse:    true,
		},
		"textDocument/hover": {
			Method:               "textDocument/hover",
			Category:             "hover",
			RequiresFileAccess:   true,
			SupportsCrossLang:    false,
			SupportsAggregation:  false,
			DefaultStrategy:      RoutingStrategyFirst,
			Priority:             6,
			ExpectedResponseType: "Hover",
			TimeoutHint:          3 * time.Second,
			CacheableResponse:    true,
		},
		"textDocument/documentSymbol": {
			Method:               "textDocument/documentSymbol",
			Category:             "symbol",
			RequiresFileAccess:   true,
			SupportsCrossLang:    false,
			SupportsAggregation:  false,
			DefaultStrategy:      RoutingStrategyFirst,
			Priority:             3,
			ExpectedResponseType: "DocumentSymbol[]",
			TimeoutHint:          5 * time.Second,
			CacheableResponse:    true,
		},
		"workspace/symbol": {
			Method:               "workspace/symbol",
			Category:             "symbol",
			RequiresFileAccess:   false,
			SupportsCrossLang:    true,
			SupportsAggregation:  true,
			DefaultStrategy:      RoutingStrategyAggregate,
			Priority:             2,
			ExpectedResponseType: "SymbolInformation[]",
			TimeoutHint:          15 * time.Second,
			CacheableResponse:    true,
		},
		"textDocument/completion": {
			Method:               "textDocument/completion",
			Category:             "completion",
			RequiresFileAccess:   true,
			SupportsCrossLang:    true,
			SupportsAggregation:  true,
			DefaultStrategy:      RoutingStrategyAggregate,
			Priority:             8,
			ExpectedResponseType: "CompletionList",
			TimeoutHint:          2 * time.Second,
			CacheableResponse:    false,
		},
		"textDocument/publishDiagnostics": {
			Method:               "textDocument/publishDiagnostics",
			Category:             "diagnostic",
			RequiresFileAccess:   true,
			SupportsCrossLang:    false,
			SupportsAggregation:  true,
			DefaultStrategy:      RoutingStrategyAggregate,
			Priority:             1,
			ExpectedResponseType: "Diagnostic[]",
			TimeoutHint:          10 * time.Second,
			CacheableResponse:    false,
		},
		"textDocument/formatting": {
			Method:               "textDocument/formatting",
			Category:             "formatting",
			RequiresFileAccess:   true,
			SupportsCrossLang:    false,
			SupportsAggregation:  false,
			DefaultStrategy:      RoutingStrategyFirst,
			Priority:             7,
			ExpectedResponseType: "TextEdit[]",
			TimeoutHint:          5 * time.Second,
			CacheableResponse:    false,
		},
	}

	c.cacheMutex.Lock()
	for method, info := range methods {
		c.methodInfoCache[method] = info
	}
	c.cacheMutex.Unlock()
}

func (c *defaultRequestClassifier) classifyMethod(method string, params interface{}) *RequestTypeInfo {
	// Default classification for unknown methods
	return &RequestTypeInfo{
		Method:               method,
		Category:             "unknown",
		RequiresFileAccess:   false,
		SupportsCrossLang:    false,
		SupportsAggregation:  false,
		DefaultStrategy:      RoutingStrategyFirst,
		Priority:             5,
		ExpectedResponseType: "unknown",
		TimeoutHint:          10 * time.Second,
		CacheableResponse:    false,
	}
}

func (c *defaultRequestClassifier) analyzeLanguageFromURI(uri string) (*RequestLanguageContext, error) {
	// Extract file path from URI
	filePath := strings.TrimPrefix(uri, "file://")
	ext := strings.ToLower(filepath.Ext(filePath))

	context := &RequestLanguageContext{
		FileExtension:      ext,
		SecondaryLanguages: []string{},
		EmbeddedLanguages:  make(map[string][]ContentRange),
	}

	// Detect primary language from extension
	if language, exists := c.extensionMap[ext]; exists {
		context.PrimaryLanguage = language
		context.LanguageConfidence = 0.9
	} else {
		context.PrimaryLanguage = "unknown"
		context.LanguageConfidence = 0.0
	}

	// Detect content type
	context.ContentType = c.detectContentType(ext)

	// Check for multi-language files
	context.IsMultiLanguage = c.isMultiLanguageFile(ext)

	// Detect template engine
	context.TemplateEngine = c.detectTemplateEngine(filePath)

	return context, nil
}

func (c *defaultRequestClassifier) enhanceWithContentAnalysis(context *RequestLanguageContext, filePath string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	contentStr := string(content)

	// Analyze content patterns for embedded languages
	for language, pattern := range c.contentPatterns {
		if matches := pattern.FindAllStringIndex(contentStr, -1); len(matches) > 0 {
			if context.EmbeddedLanguages == nil {
				context.EmbeddedLanguages = make(map[string][]ContentRange)
			}

			ranges := make([]ContentRange, 0, len(matches))
			for _, match := range matches {
				// Convert byte positions to line/character positions
				startLine, startChar := c.byteToLineChar(contentStr, match[0])
				endLine, endChar := c.byteToLineChar(contentStr, match[1])

				ranges = append(ranges, ContentRange{
					StartLine:  startLine,
					StartChar:  startChar,
					EndLine:    endLine,
					EndChar:    endChar,
					Language:   language,
					Confidence: 0.8,
				})
			}
			context.EmbeddedLanguages[language] = ranges
		}
	}

	// Update multi-language flag
	if len(context.EmbeddedLanguages) > 0 {
		context.IsMultiLanguage = true
	}

	return nil
}

func (c *defaultRequestClassifier) enhanceWithProjectLanguages(context *RequestLanguageContext, uri string) error {
	workspaceRoot := c.findWorkspaceRoot(uri)
	if workspaceRoot == "" {
		return nil
	}

	// Scan project for configuration files
	configFiles := c.findConfigFiles(workspaceRoot)
	languages := make(map[string]bool)

	for _, configFile := range configFiles {
		if lang := c.getLanguageFromConfig(configFile); lang != "" {
			languages[lang] = true
		}
	}

	// Convert to slice
	projectLanguages := make([]string, 0, len(languages))
	for lang := range languages {
		projectLanguages = append(projectLanguages, lang)
	}

	context.ProjectLanguages = projectLanguages

	return nil
}

func (c *defaultRequestClassifier) analyzeWorkspaceStructure(context *RequestWorkspaceContext) error {
	if context.WorkspaceRoot == "" {
		return nil
	}

	// Find configuration files
	configFiles := c.findConfigFiles(context.WorkspaceRoot)
	context.ConfigFiles = configFiles

	// Detect supported languages
	languages := make(map[string]bool)
	for _, configFile := range configFiles {
		if lang := c.getLanguageFromConfig(configFile); lang != "" {
			languages[lang] = true
		}
	}

	supportedLanguages := make([]string, 0, len(languages))
	for lang := range languages {
		supportedLanguages = append(supportedLanguages, lang)
	}
	context.SupportedLanguages = supportedLanguages

	// Detect monorepo structure
	context.IsMonorepo = c.detectMonorepo(context.WorkspaceRoot)

	if context.IsMonorepo {
		context.SubProjects = c.findSubProjects(context.WorkspaceRoot)
	}

	// Extract project name
	context.ProjectName = filepath.Base(context.WorkspaceRoot)

	return nil
}

func (c *defaultRequestClassifier) detectProjectType(context *RequestWorkspaceContext) error {
	if context.WorkspaceRoot == "" {
		return nil
	}

	// Check for specific project types based on config files
	for _, configFile := range context.ConfigFiles {
		if projectType := c.getProjectTypeFromConfig(configFile); projectType != "" {
			context.ProjectType = projectType
			return nil
		}
	}

	// Default to generic project
	context.ProjectType = "generic"
	return nil
}

func (c *defaultRequestClassifier) detectFrameworks(context *RequestWorkspaceContext) error {
	for _, detector := range c.frameworkDetectors {
		if c.matchesFramework(context, detector) {
			context.FrameworkInfo = &FrameworkInfo{
				Name:        detector.Name,
				Type:        c.getFrameworkType(detector.Name),
				ConfigFiles: c.findFrameworkConfigs(context.WorkspaceRoot, detector),
			}
			break
		}
	}

	return nil
}

func (c *defaultRequestClassifier) analyzeDependencies(context *RequestWorkspaceContext) error {
	context.Dependencies = make(map[string][]string)

	// Analyze different dependency files
	dependencyFiles := map[string]string{
		"package.json":     "npm",
		"requirements.txt": "pip",
		"go.mod":           "go",
		"Cargo.toml":       "cargo",
		"pom.xml":          "maven",
		"build.gradle":     "gradle",
	}

	for file, manager := range dependencyFiles {
		fullPath := filepath.Join(context.WorkspaceRoot, file)
		if _, err := os.Stat(fullPath); err == nil {
			deps, err := c.parseDependencies(fullPath, manager)
			if err == nil {
				context.Dependencies[manager] = deps
				if context.PackageManager == "" {
					context.PackageManager = manager
				}
			}
		}
	}

	return nil
}

// Cache management and utility methods

func (c *defaultRequestClassifier) cacheRequestLanguageContext(key string, context *RequestLanguageContext) {
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()

	if len(c.languageCache) >= c.maxCacheSize {
		// Simple cache eviction - remove first entry found
		for k := range c.languageCache {
			delete(c.languageCache, k)
			break
		}
	}

	c.languageCache[key] = context
}

func (c *defaultRequestClassifier) generateCacheKey(request *JSONRPCRequest, uri string) string {
	return fmt.Sprintf("%s:%s:%d", request.Method, uri, time.Now().Unix()/60) // Cache per minute
}

func (c *defaultRequestClassifier) generateRoutingHints(typeInfo *RequestTypeInfo, langCtx *RequestLanguageContext, wsCtx *RequestWorkspaceContext, crossLang bool) *RoutingHints {
	hints := &RoutingHints{
		PreferredServers:    []string{},
		FallbackServers:     []string{},
		AggregationStrategy: "merge",
		CacheStrategy:       "memory",
		Timeout:             typeInfo.TimeoutHint,
		Priority:            typeInfo.Priority,
		RequiresWorkspace:   typeInfo.RequiresFileAccess,
		BenefitsFromCache:   typeInfo.CacheableResponse,
	}

	// Set preferred servers based on language
	if langCtx != nil && langCtx.PrimaryLanguage != "" {
		hints.PreferredServers = append(hints.PreferredServers, langCtx.PrimaryLanguage+"-lsp")
	}

	// Add secondary language servers for cross-language requests
	if crossLang && langCtx != nil {
		for _, lang := range langCtx.SecondaryLanguages {
			hints.FallbackServers = append(hints.FallbackServers, lang+"-lsp")
		}
	}

	return hints
}

// Utility methods

func (c *defaultRequestClassifier) findWorkspaceRoot(uri string) string {
	filePath := strings.TrimPrefix(uri, "file://")
	if !filepath.IsAbs(filePath) {
		return ""
	}

	dir := filepath.Dir(filePath)

	// Look for common workspace markers
	markers := []string{".git", ".svn", ".hg", "go.mod", "package.json", "Cargo.toml", "pom.xml"}

	for {
		for _, marker := range markers {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				return dir
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return filepath.Dir(filePath) // Fallback to file directory
}

func (c *defaultRequestClassifier) findConfigFiles(workspaceRoot string) []string {
	configFiles := []string{}

	configPatterns := []string{
		"package.json", "go.mod", "Cargo.toml", "pom.xml", "build.gradle",
		"requirements.txt", "setup.py", "pyproject.toml", "composer.json",
		"tsconfig.json", "webpack.config.js", "babel.config.js",
		".eslintrc*", ".prettierrc*", "tslint.json",
	}

	for _, pattern := range configPatterns {
		fullPath := filepath.Join(workspaceRoot, pattern)
		if _, err := os.Stat(fullPath); err == nil {
			configFiles = append(configFiles, fullPath)
		}
	}

	return configFiles
}

func (c *defaultRequestClassifier) byteToLineChar(content string, bytePos int) (int, int) {
	if bytePos >= len(content) {
		bytePos = len(content) - 1
	}

	line := 0
	char := 0

	for i, r := range content {
		if i >= bytePos {
			break
		}
		if r == '\n' {
			line++
			char = 0
		} else {
			char++
		}
	}

	return line, char
}

// Additional utility methods for language detection, project analysis, etc.
// (These would be implemented based on specific requirements)

func (c *defaultRequestClassifier) detectContentType(ext string) string {
	contentTypes := map[string]string{
		".go":   "source",
		".py":   "source",
		".js":   "source",
		".ts":   "source",
		".java": "source",
		".html": "markup",
		".xml":  "markup",
		".md":   "markup",
		".json": "data",
		".yaml": "data",
		".yml":  "data",
	}

	if contentType, exists := contentTypes[ext]; exists {
		return contentType
	}
	return "unknown"
}

func (c *defaultRequestClassifier) isMultiLanguageFile(ext string) bool {
	multiLangExts := map[string]bool{
		".html":   true,
		".vue":    true,
		".svelte": true,
		".jsx":    true,
		".tsx":    true,
		".md":     true,
	}

	return multiLangExts[ext]
}

func (c *defaultRequestClassifier) detectTemplateEngine(filePath string) string {
	engines := map[string]string{
		".ejs":      "ejs",
		".hbs":      "handlebars",
		".mustache": "mustache",
		".pug":      "pug",
		".twig":     "twig",
	}

	ext := filepath.Ext(filePath)
	if engine, exists := engines[ext]; exists {
		return engine
	}

	// Check for template patterns in filename
	if strings.Contains(filePath, ".template.") {
		return "generic"
	}

	return ""
}

func (c *defaultRequestClassifier) getLanguageFromConfig(configFile string) string {
	configMap := map[string]string{
		"package.json":     "javascript",
		"go.mod":           "go",
		"Cargo.toml":       "rust",
		"pom.xml":          "java",
		"requirements.txt": "python",
		"setup.py":         "python",
		"pyproject.toml":   "python",
		"composer.json":    "php",
		"tsconfig.json":    "typescript",
	}

	filename := filepath.Base(configFile)
	return configMap[filename]
}

func (c *defaultRequestClassifier) getProjectTypeFromConfig(configFile string) string {
	typeMap := map[string]string{
		"package.json":     "npm",
		"go.mod":           "go-module",
		"Cargo.toml":       "rust-crate",
		"pom.xml":          "maven",
		"build.gradle":     "gradle",
		"requirements.txt": "python",
		"setup.py":         "python-package",
		"pyproject.toml":   "python-modern",
		"composer.json":    "php-composer",
	}

	filename := filepath.Base(configFile)
	return typeMap[filename]
}

func (c *defaultRequestClassifier) detectMonorepo(workspaceRoot string) bool {
	// Common monorepo indicators
	indicators := []string{
		"lerna.json",
		"nx.json",
		"workspace.json",
		"rush.json",
		"packages",
		"apps",
		"libs",
	}

	for _, indicator := range indicators {
		if _, err := os.Stat(filepath.Join(workspaceRoot, indicator)); err == nil {
			return true
		}
	}

	return false
}

func (c *defaultRequestClassifier) findSubProjects(workspaceRoot string) []string {
	subProjects := []string{}

	// Common sub-project directories
	subDirs := []string{"packages", "apps", "libs", "services", "modules"}

	for _, subDir := range subDirs {
		fullPath := filepath.Join(workspaceRoot, subDir)
		if info, err := os.Stat(fullPath); err == nil && info.IsDir() {
			if entries, err := os.ReadDir(fullPath); err == nil {
				for _, entry := range entries {
					if entry.IsDir() {
						subProjects = append(subProjects, filepath.Join(subDir, entry.Name()))
					}
				}
			}
		}
	}

	return subProjects
}

func (c *defaultRequestClassifier) matchesFramework(context *RequestWorkspaceContext, detector *SimpleFrameworkDetector) bool {
	// Check for required config files
	for _, configFile := range detector.ConfigFiles {
		found := false
		for _, existingConfig := range context.ConfigFiles {
			if filepath.Base(existingConfig) == configFile {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

func (c *defaultRequestClassifier) getFrameworkType(frameworkName string) string {
	typeMap := map[string]string{
		"react":    "web",
		"vue":      "web",
		"angular":  "web",
		"express":  "api",
		"fastapi":  "api",
		"spring":   "api",
		"django":   "web",
		"flask":    "web",
		"electron": "desktop",
		"tauri":    "desktop",
	}

	if ftype, exists := typeMap[frameworkName]; exists {
		return ftype
	}
	return "unknown"
}

func (c *defaultRequestClassifier) findFrameworkConfigs(workspaceRoot string, detector *SimpleFrameworkDetector) []string {
	configs := []string{}

	for _, configFile := range detector.ConfigFiles {
		fullPath := filepath.Join(workspaceRoot, configFile)
		if _, err := os.Stat(fullPath); err == nil {
			configs = append(configs, fullPath)
		}
	}

	return configs
}

func (c *defaultRequestClassifier) parseDependencies(filePath, manager string) ([]string, error) {
	dependencies := []string{}

	switch manager {
	case "npm":
		// Parse package.json
		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil, err
		}

		var packageJSON map[string]interface{}
		if err := json.Unmarshal(content, &packageJSON); err != nil {
			return nil, err
		}

		if deps, ok := packageJSON["dependencies"].(map[string]interface{}); ok {
			for dep := range deps {
				dependencies = append(dependencies, dep)
			}
		}

		if devDeps, ok := packageJSON["devDependencies"].(map[string]interface{}); ok {
			for dep := range devDeps {
				dependencies = append(dependencies, dep)
			}
		}

	case "pip":
		// Parse requirements.txt
		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil, err
		}

		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				// Extract package name (before version specifiers)
				if idx := strings.IndexAny(line, ">=<!="); idx > 0 {
					dependencies = append(dependencies, line[:idx])
				} else {
					dependencies = append(dependencies, line)
				}
			}
		}
	}

	return dependencies, nil
}

// Initialization functions

func initializeExtensionMap() map[string]string {
	return map[string]string{
		".go":         "go",
		".py":         "python",
		".js":         "javascript",
		".ts":         "typescript",
		".jsx":        "javascript",
		".tsx":        "typescript",
		".java":       "java",
		".kt":         "kotlin",
		".rs":         "rust",
		".cpp":        "cpp",
		".cc":         "cpp",
		".cxx":        "cpp",
		".c":          "c",
		".h":          "c",
		".hpp":        "cpp",
		".cs":         "csharp",
		".php":        "php",
		".rb":         "ruby",
		".swift":      "swift",
		".scala":      "scala",
		".clj":        "clojure",
		".hs":         "haskell",
		".ml":         "ocaml",
		".fs":         "fsharp",
		".dart":       "dart",
		".lua":        "lua",
		".r":          "r",
		".m":          "objective-c",
		".mm":         "objective-cpp",
		".html":       "html",
		".htm":        "html",
		".css":        "css",
		".scss":       "scss",
		".sass":       "sass",
		".less":       "less",
		".xml":        "xml",
		".json":       "json",
		".yaml":       "yaml",
		".yml":        "yaml",
		".toml":       "toml",
		".md":         "markdown",
		".sh":         "shell",
		".bash":       "shell",
		".zsh":        "shell",
		".fish":       "shell",
		".ps1":        "powershell",
		".sql":        "sql",
		".dockerfile": "dockerfile",
		".Dockerfile": "dockerfile",
	}
}

func initializeContentPatterns() map[string]*regexp.Regexp {
	patterns := make(map[string]*regexp.Regexp)

	// JavaScript in HTML
	patterns["javascript"] = regexp.MustCompile(`<script[^>]*>(.*?)</script>`)

	// CSS in HTML
	patterns["css"] = regexp.MustCompile(`<style[^>]*>(.*?)</style>`)

	// Code blocks in Markdown
	patterns["code"] = regexp.MustCompile("```(\\w+)\\n([\\s\\S]*?)\\n```")

	// Template literals
	patterns["template"] = regexp.MustCompile("`([^`]*)`")

	return patterns
}

func initializeConfigFilePatterns() map[string]string {
	return map[string]string{
		"package.json":     "npm",
		"go.mod":           "go",
		"Cargo.toml":       "cargo",
		"pom.xml":          "maven",
		"build.gradle":     "gradle",
		"requirements.txt": "pip",
		"setup.py":         "python",
		"pyproject.toml":   "python",
		"composer.json":    "composer",
		"Gemfile":          "bundler",
	}
}

func initializeFrameworkDetectors() map[string]*SimpleFrameworkDetector {
	detectors := make(map[string]*SimpleFrameworkDetector)

	detectors["react"] = &SimpleFrameworkDetector{
		Name:         "react",
		ConfigFiles:  []string{"package.json"},
		Dependencies: []string{"react"},
		FilePatterns: []string{"*.jsx", "*.tsx"},
	}

	detectors["vue"] = &SimpleFrameworkDetector{
		Name:         "vue",
		ConfigFiles:  []string{"package.json"},
		Dependencies: []string{"vue"},
		FilePatterns: []string{"*.vue"},
	}

	detectors["angular"] = &SimpleFrameworkDetector{
		Name:         "angular",
		ConfigFiles:  []string{"package.json", "angular.json"},
		Dependencies: []string{"@angular/core"},
		FilePatterns: []string{"*.component.ts", "*.service.ts"},
	}

	detectors["django"] = &SimpleFrameworkDetector{
		Name:         "django",
		ConfigFiles:  []string{"manage.py", "requirements.txt"},
		Dependencies: []string{"django"},
		FilePatterns: []string{"settings.py", "urls.py"},
	}

	detectors["flask"] = &SimpleFrameworkDetector{
		Name:         "flask",
		ConfigFiles:  []string{"requirements.txt"},
		Dependencies: []string{"flask"},
		FilePatterns: []string{"app.py", "*.py"},
	}

	detectors["spring"] = &SimpleFrameworkDetector{
		Name:         "spring",
		ConfigFiles:  []string{"pom.xml", "build.gradle"},
		Dependencies: []string{"spring-boot", "spring-core"},
		FilePatterns: []string{"*.java"},
	}

	return detectors
}
