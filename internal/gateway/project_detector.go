package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"lsp-gateway/internal/config"
)

const (
	DefaultMaxDepth    = 3
	DefaultFileSizeLimit = 50 * 1024 * 1024 // 50MB
	DefaultMaxFiles    = 10000
	ScanTimeout        = 30 * time.Second
)

// LanguageStats represents statistical information about a language in the project
type LanguageStats struct {
	FileCount        int     `json:"file_count" yaml:"file_count"`
	TestFileCount    int     `json:"test_file_count" yaml:"test_file_count"`
	BuildFileScore   int     `json:"build_file_score" yaml:"build_file_score"`
	DirectoryScore   int     `json:"directory_score" yaml:"directory_score"`
	RecentActivity   int     `json:"recent_activity" yaml:"recent_activity"`
	TotalScore       int     `json:"total_score" yaml:"total_score"`
}

// LanguageContext represents comprehensive context about a specific language in the project
type LanguageContext struct {
	Language        string                 `json:"language" yaml:"language"`                               // Language name (go, python, typescript, etc.)
	RootPath        string                 `json:"root_path" yaml:"root_path"`                             // Language-specific root (may differ from project root)
	FileCount       int                    `json:"file_count" yaml:"file_count"`                           // Number of source files found
	TestFileCount   int                    `json:"test_file_count" yaml:"test_file_count"`                 // Number of test files found
	Priority        int                    `json:"priority" yaml:"priority"`                               // Calculated priority score (1-100)
	Confidence      float64                `json:"confidence" yaml:"confidence"`                           // Detection confidence (0.0-1.0)
	BuildFiles      []string               `json:"build_files" yaml:"build_files"`                         // Build files for this language
	ConfigFiles     []string               `json:"config_files" yaml:"config_files"`                       // Config files for this language
	SourcePaths     []string               `json:"source_paths" yaml:"source_paths"`                       // Directories containing source files
	TestPaths       []string               `json:"test_paths" yaml:"test_paths"`                           // Directories containing test files
	FileExtensions  []string               `json:"file_extensions" yaml:"file_extensions"`                 // File extensions found for this language
	Framework       string                 `json:"framework,omitempty" yaml:"framework,omitempty"`         // Detected framework (if any)
	Version         string                 `json:"version,omitempty" yaml:"version,omitempty"`             // Language/framework version (if detectable)
	LSPServerName   string                 `json:"lsp_server_name,omitempty" yaml:"lsp_server_name,omitempty"` // Recommended LSP server
	Dependencies    []string               `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`   // Dependencies on other languages in project
	Metadata        map[string]interface{} `json:"metadata,omitempty" yaml:"metadata,omitempty"`           // Language-specific metadata
}

// LanguageRanking represents a ranked language with scoring information
type LanguageRanking struct {
	Language   string           `json:"language" yaml:"language"`
	Score      int              `json:"score" yaml:"score"`
	Confidence float64          `json:"confidence" yaml:"confidence"`
	Context    *LanguageContext `json:"context" yaml:"context"`
}

// MultiLanguageProjectInfo represents comprehensive information about a multi-language project
type MultiLanguageProjectInfo struct {
	RootPath           string                     `json:"root_path" yaml:"root_path"`                       // Main project root
	ProjectType        string                     `json:"project_type" yaml:"project_type"`                 // "monorepo", "multi-language", "frontend-backend", etc.
	DominantLanguage   string                     `json:"dominant_language" yaml:"dominant_language"`       // Primary language based on scoring
	Languages          map[string]*LanguageContext `json:"languages" yaml:"languages"`                      // Language name -> context
	WorkspaceRoots     map[string]string          `json:"workspace_roots" yaml:"workspace_roots"`          // Language -> specific root path
	BuildFiles         []string                   `json:"build_files" yaml:"build_files"`                   // All detected build files
	ConfigFiles        []string                   `json:"config_files" yaml:"config_files"`                 // All detected config files
	TotalFileCount     int                        `json:"total_file_count" yaml:"total_file_count"`         // Total files analyzed
	ScanDepth          int                        `json:"scan_depth" yaml:"scan_depth"`                     // Actual depth reached during scan
	ScanDuration       time.Duration              `json:"scan_duration" yaml:"scan_duration"`               // Time taken for analysis
	DetectedAt         time.Time                  `json:"detected_at" yaml:"detected_at"`                   // When detection was performed
	Metadata           map[string]interface{}     `json:"metadata,omitempty" yaml:"metadata,omitempty"`     // Extensible metadata
}

// ProjectPattern represents common project patterns for detection
type ProjectPattern struct {
	Name           string    `json:"name" yaml:"name"`                       // Pattern name
	Languages      []string  `json:"languages" yaml:"languages"`             // Expected languages
	DirectoryHints []string  `json:"directory_hints" yaml:"directory_hints"` // Directory structure hints
	BuildFileHints []string  `json:"build_file_hints" yaml:"build_file_hints"` // Build file hints
	Confidence     float64   `json:"confidence" yaml:"confidence"`           // Pattern confidence
}

// Framework represents a detected framework within a language
type Framework struct {
	Name       string                 `json:"name" yaml:"name"`
	Language   string                 `json:"language" yaml:"language"`
	Version    string                 `json:"version,omitempty" yaml:"version,omitempty"`
	ConfigFile string                 `json:"config_file,omitempty" yaml:"config_file,omitempty"`
	Confidence float64                `json:"confidence" yaml:"confidence"`
	Metadata   map[string]interface{} `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// ProjectDetectionError represents errors that occur during project detection
type ProjectDetectionError struct {
	Type    string `json:"type" yaml:"type"`
	Message string `json:"message" yaml:"message"`
	Path    string `json:"path,omitempty" yaml:"path,omitempty"`
	Context string `json:"context,omitempty" yaml:"context,omitempty"`
}

func (e *ProjectDetectionError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("project detection error (%s) at %s: %s", e.Type, e.Path, e.Message)
	}
	return fmt.Sprintf("project detection error (%s): %s", e.Type, e.Message)
}

type ProjectLanguageScanner struct {
	MaxDepth       int
	IgnorePatterns []string
	FileSizeLimit  int64
	MaxFiles       int
	Timeout        time.Duration
	
	// Performance optimization fields
	Cache              *ProjectCache
	EnableEarlyExit    bool
	MaxConcurrentScans int
	FastModeThreshold  int  // Switch to fast mode for small projects
	EnableCache        bool
	
	langPatterns    map[string]*LanguagePattern
	buildFiles      map[string]string
	configFiles     map[string]string
	testPatterns    []string
	sourceDirs      []string
	
	// Fast-path detection cache
	fastPathCache   map[string]*MultiLanguageProjectInfo
	fastPathMutex   sync.RWMutex
}

type LanguagePattern struct {
	Language     string
	Extensions   []string
	BuildFiles   []string
	ConfigFiles  []string 
	SourceDirs   []string
	TestDirs     []string
	BaseScore    float64
	BuildWeight  float64
}

func NewProjectLanguageScanner() *ProjectLanguageScanner {
	scanner := &ProjectLanguageScanner{
		MaxDepth:      DefaultMaxDepth,
		FileSizeLimit: DefaultFileSizeLimit,
		MaxFiles:      DefaultMaxFiles,
		Timeout:       ScanTimeout,
		
		// Performance optimization defaults
		EnableEarlyExit:    true,
		MaxConcurrentScans: DefaultMaxConcurrentScans,
		FastModeThreshold:  DefaultFastModeThreshold,
		EnableCache:        true,
		
		IgnorePatterns: []string{
			"node_modules", "venv", "env", ".venv", ".env",
			".git", ".svn", ".hg", ".bzr",
			"__pycache__", ".pytest_cache", ".mypy_cache",
			"target", "build", "dist", "out", "output",
			".idea", ".vscode", ".vs", "*.tmp", "*.temp",
			".terraform", ".gradle", ".m2", "vendor",
			"coverage", ".coverage", ".nyc_output",
			"logs", "*.log", ".DS_Store", "Thumbs.db",
		},
		langPatterns: make(map[string]*LanguagePattern),
		buildFiles:   make(map[string]string),
		configFiles:  make(map[string]string),
		fastPathCache: make(map[string]*MultiLanguageProjectInfo),
	}
	
	// Initialize cache if enabled
	if scanner.EnableCache {
		scanner.Cache = NewProjectCache()
	}
	
	scanner.initializeLanguagePatterns()
	return scanner
}

func (s *ProjectLanguageScanner) initializeLanguagePatterns() {
	patterns := []*LanguagePattern{
		{
			Language:    "go",
			Extensions:  []string{".go"},
			BuildFiles:  []string{"go.mod", "go.sum", "go.work", "Makefile"},
			ConfigFiles: []string{"go.mod", "go.work", ".golangci.yml", ".golangci.yaml"},
			SourceDirs:  []string{"cmd", "internal", "pkg", "api"},
			TestDirs:    []string{"test", "tests"},
			BaseScore:   1.0,
			BuildWeight: 10.0,
		},
		{
			Language:    "python",
			Extensions:  []string{".py", ".pyx", ".pyi"},
			BuildFiles:  []string{"setup.py", "pyproject.toml", "setup.cfg", "requirements.txt", "Pipfile", "poetry.lock"},
			ConfigFiles: []string{"pyproject.toml", "setup.cfg", "tox.ini", ".flake8", "mypy.ini"},
			SourceDirs:  []string{"src", "lib", "app", "core"},
			TestDirs:    []string{"test", "tests", "testing"},
			BaseScore:   1.0,
			BuildWeight: 8.0,
		},
		{
			Language:    "typescript",
			Extensions:  []string{".ts", ".tsx"},
			BuildFiles:  []string{"package.json", "tsconfig.json", "yarn.lock", "pnpm-lock.yaml"},
			ConfigFiles: []string{"tsconfig.json", "webpack.config.js", ".eslintrc", "prettier.config.js"},
			SourceDirs:  []string{"src", "lib", "components", "pages", "app"},
			TestDirs:    []string{"test", "tests", "__tests__", "spec"},
			BaseScore:   1.0,
			BuildWeight: 9.0,
		},
		{
			Language:    "javascript",
			Extensions:  []string{".js", ".jsx", ".mjs", ".cjs"},
			BuildFiles:  []string{"package.json", "yarn.lock", "package-lock.json", "pnpm-lock.yaml"},
			ConfigFiles: []string{"webpack.config.js", ".babelrc", ".eslintrc", "jest.config.js"},
			SourceDirs:  []string{"src", "lib", "app", "public", "static"},
			TestDirs:    []string{"test", "tests", "__tests__", "spec"},
			BaseScore:   1.0,
			BuildWeight: 7.0,
		},
		{
			Language:    "java",
			Extensions:  []string{".java"},
			BuildFiles:  []string{"pom.xml", "build.gradle", "gradle.properties", "settings.gradle", "build.xml"},
			ConfigFiles: []string{"application.properties", "application.yml", "logback.xml"},
			SourceDirs:  []string{"src/main/java", "src", "main", "java"},
			TestDirs:    []string{"src/test/java", "test", "tests"},
			BaseScore:   1.0,
			BuildWeight: 9.0,
		},
		{
			Language:    "kotlin",
			Extensions:  []string{".kt", ".kts"},
			BuildFiles:  []string{"build.gradle.kts", "pom.xml", "settings.gradle.kts"},
			ConfigFiles: []string{"application.conf", "application.yml"},
			SourceDirs:  []string{"src/main/kotlin", "src", "main"},
			TestDirs:    []string{"src/test/kotlin", "test"},
			BaseScore:   1.0,
			BuildWeight: 8.0,
		},
		{
			Language:    "rust",
			Extensions:  []string{".rs"},
			BuildFiles:  []string{"Cargo.toml", "Cargo.lock", "build.rs"},
			ConfigFiles: []string{"Cargo.toml", "rust-toolchain", ".rustfmt.toml"},
			SourceDirs:  []string{"src", "lib", "bin"},
			TestDirs:    []string{"tests", "benches"},
			BaseScore:   1.0,
			BuildWeight: 10.0,
		},
		{
			Language:    "cpp",
			Extensions:  []string{".cpp", ".cc", ".cxx", ".c++", ".hpp", ".h", ".hxx"},
			BuildFiles:  []string{"CMakeLists.txt", "Makefile", "configure.ac", "meson.build"},
			ConfigFiles: []string{"CMakeLists.txt", ".clang-format", "conanfile.txt"},
			SourceDirs:  []string{"src", "source", "lib", "include"},
			TestDirs:    []string{"test", "tests", "testing"},
			BaseScore:   1.0,
			BuildWeight: 8.0,
		},
		{
			Language:    "c",
			Extensions:  []string{".c", ".h"},
			BuildFiles:  []string{"Makefile", "configure.ac", "CMakeLists.txt"},
			ConfigFiles: []string{"config.h", "configure.ac"},
			SourceDirs:  []string{"src", "source", "lib", "include"},
			TestDirs:    []string{"test", "tests"},
			BaseScore:   1.0,
			BuildWeight: 7.0,
		},
		{
			Language:    "csharp",
			Extensions:  []string{".cs"},
			BuildFiles:  []string{"*.csproj", "*.sln", "Directory.Build.props", "nuget.config"},
			ConfigFiles: []string{"appsettings.json", "web.config"},
			SourceDirs:  []string{"src", "Source", "App"},
			TestDirs:    []string{"test", "tests", "Test", "Tests"},
			BaseScore:   1.0,
			BuildWeight: 8.0,
		},
		{
			Language:    "ruby",
			Extensions:  []string{".rb"},
			BuildFiles:  []string{"Gemfile", "Gemfile.lock", "*.gemspec", "Rakefile"},
			ConfigFiles: []string{".rubocop.yml", "config.ru", "database.yml"},
			SourceDirs:  []string{"lib", "app", "bin"},
			TestDirs:    []string{"test", "spec", "tests"},
			BaseScore:   1.0,
			BuildWeight: 7.0,
		},
		{
			Language:    "php",
			Extensions:  []string{".php"},
			BuildFiles:  []string{"composer.json", "composer.lock"},
			ConfigFiles: []string{"php.ini", ".phpunit.xml", "phpcs.xml"},
			SourceDirs:  []string{"src", "app", "lib"},
			TestDirs:    []string{"test", "tests", "spec"},
			BaseScore:   1.0,
			BuildWeight: 6.0,
		},
		{
			Language:    "swift",
			Extensions:  []string{".swift"},
			BuildFiles:  []string{"Package.swift", "*.xcodeproj", "*.xcworkspace"},
			ConfigFiles: []string{".swiftlint.yml", "Info.plist"},
			SourceDirs:  []string{"Sources", "src"},
			TestDirs:    []string{"Tests", "test"},
			BaseScore:   1.0,
			BuildWeight: 8.0,
		},
	}
	
	for _, pattern := range patterns {
		s.langPatterns[pattern.Language] = pattern
		
		for _, buildFile := range pattern.BuildFiles {
			s.buildFiles[buildFile] = pattern.Language
		}
		
		for _, configFile := range pattern.ConfigFiles {
			s.configFiles[configFile] = pattern.Language
		}
	}
	
	s.testPatterns = []string{
		"test", "tests", "testing", "spec", "specs", "__tests__",
		"*_test.*", "*_spec.*", "test_*.*", "spec_*.*",
		"*.test.*", "*.spec.*",
	}
	
	s.sourceDirs = []string{
		"src", "source", "lib", "libs", "app", "main", "core",
		"pkg", "internal", "cmd", "api", "server", "client",
		"components", "pages", "views", "controllers", "models",
	}
}

// ScanProjectCached performs cached project scanning with performance optimizations
func (s *ProjectLanguageScanner) ScanProjectCached(rootPath string) (*MultiLanguageProjectInfo, error) {
	// Try cache first if enabled
	if s.EnableCache && s.Cache != nil {
		if cached, hit := s.Cache.Get(rootPath); hit {
			return cached, nil
		}
	}
	
	// Try fast-path detection first
	if fastResult, isFastPath := s.tryFastPathDetection(rootPath); isFastPath {
		if s.EnableCache && s.Cache != nil {
			s.Cache.Set(rootPath, fastResult)
		}
		return fastResult, nil
	}
	
	// Fall back to full scan
	result, err := s.ScanProject(rootPath)
	if err != nil {
		return nil, err
	}
	
	// Cache the result
	if s.EnableCache && s.Cache != nil {
		s.Cache.Set(rootPath, result)
	}
	
	return result, nil
}

// ScanProjectIncremental performs incremental scanning since a given timestamp
func (s *ProjectLanguageScanner) ScanProjectIncremental(rootPath string, lastScan time.Time) (*MultiLanguageProjectInfo, error) {
	// Check if we need to do a full rescan
	if s.needsFullRescan(rootPath, lastScan) {
		return s.ScanProjectCached(rootPath)
	}
	
	// Try to get cached result and update incrementally
	if s.EnableCache && s.Cache != nil {
		if cached, hit := s.Cache.Get(rootPath); hit {
			// Validate cache is still good
			if cached.DetectedAt.After(lastScan) {
				return cached, nil
			}
		}
	}
	
	// Fall back to full scan
	return s.ScanProjectCached(rootPath)
}

func (s *ProjectLanguageScanner) ScanProject(rootPath string) (*MultiLanguageProjectInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.Timeout)
	defer cancel()
	
	// Add early termination context if enabled
	if s.EnableEarlyExit {
		ctx = s.addEarlyTerminationContext(ctx, rootPath)
	}
	
	startTime := time.Now()
	
	absRoot, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}
	
	stats := make(map[string]*LanguageStats)
	var totalFiles int
	var scanMutex sync.Mutex
	
	err = s.scanDirectoryRecursive(ctx, absRoot, 0, stats, &totalFiles, &scanMutex)
	if err != nil {
		return nil, fmt.Errorf("scan failed: %w", err)
	}
	
	if len(stats) == 0 {
		return &MultiLanguageProjectInfo{
			ProjectType:    "empty",
			RootPath:       absRoot,
			Languages:      make(map[string]*LanguageContext),
			WorkspaceRoots: make(map[string]string),
			BuildFiles:     []string{},
			ConfigFiles:    []string{},
			TotalFileCount: totalFiles,
			ScanDepth:      s.MaxDepth,
			ScanDuration:   time.Since(startTime),
			DetectedAt:     time.Now(),
			Metadata:       make(map[string]interface{}),
		}, nil
	}
	
	rankings := s.calculateLanguagePriorities(stats)
	projectType := s.identifyProjectType(rankings)
	
	var buildSystems []string
	var sourceDirs []string
	var testDirs []string
	var configFiles []string
	
	for _, lang := range stats {
		buildSystems = append(buildSystems, lang.BuildFiles...)
		sourceDirs = append(sourceDirs, lang.SourceDirs...)
		configFiles = append(configFiles, lang.ConfigFiles...)
	}
	
	buildSystems = s.removeDuplicates(buildSystems)
	sourceDirs = s.removeDuplicates(sourceDirs)
	configFiles = s.removeDuplicates(configFiles)
	
	dominantLang := ""
	if len(rankings) > 0 {
		dominantLang = rankings[0].Language
	}
	
	// Convert to new structure format
	info := s.convertLegacyToNewFormat(rankings, absRoot, projectType, dominantLang, 
		buildSystems, sourceDirs, configFiles, totalFiles, time.Since(startTime), s.MaxDepth, len(stats))
	
	return info, nil
}

func (s *ProjectLanguageScanner) scanDirectoryRecursive(ctx context.Context, dirPath string, depth int, stats map[string]*LanguageStats, totalFiles *int, mutex *sync.Mutex) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	
	if depth > s.MaxDepth {
		return nil
	}
	
	mutex.Lock()
	if *totalFiles >= s.MaxFiles {
		mutex.Unlock()
		return nil
	}
	mutex.Unlock()
	
	if s.shouldIgnoreDirectory(filepath.Base(dirPath)) {
		return nil
	}
	
	entries, err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Continue on errors
		}
		
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		if path == dirPath {
			return nil
		}
		
		if d.IsDir() {
			if s.shouldIgnoreDirectory(d.Name()) {
				return filepath.SkipDir
			}
			
			relDepth := strings.Count(strings.TrimPrefix(path, dirPath), string(filepath.Separator))
			if relDepth >= s.MaxDepth {
				return filepath.SkipDir
			}
			return nil
		}
		
		mutex.Lock()
		if *totalFiles >= s.MaxFiles {
			mutex.Unlock()
			return filepath.SkipAll
		}
		*totalFiles++
		mutex.Unlock()
		
		s.analyzeFile(path, dirPath, stats, mutex)
		return nil
	})
	
	return entries
}

func (s *ProjectLanguageScanner) analyzeFile(filePath, rootPath string, stats map[string]*LanguageStats, mutex *sync.Mutex) {
	fileName := filepath.Base(filePath)
	ext := filepath.Ext(fileName)
	
	// Check build files first
	if lang, exists := s.buildFiles[fileName]; exists {
		mutex.Lock()
		if stats[lang] == nil {
			stats[lang] = &LanguageStats{
				Language:    lang,
				Extensions:  make(map[string]int),
				BuildFiles:  []string{},
				ConfigFiles: []string{},
				SourceDirs:  []string{},
			}
		}
		stats[lang].BuildFiles = append(stats[lang].BuildFiles, filePath)
		mutex.Unlock()
		return
	}
	
	// Check config files
	if lang, exists := s.configFiles[fileName]; exists {
		mutex.Lock()
		if stats[lang] == nil {
			stats[lang] = &LanguageStats{
				Language:    lang,
				Extensions:  make(map[string]int),
				BuildFiles:  []string{},
				ConfigFiles: []string{},
				SourceDirs:  []string{},
			}
		}
		stats[lang].ConfigFiles = append(stats[lang].ConfigFiles, filePath)
		mutex.Unlock()
		return
	}
	
	// Check language by extension
	language := s.getFileLanguage(ext)
	if language == "" {
		return
	}
	
	fileInfo, err := filepath.Glob(filePath)
	if err != nil || len(fileInfo) == 0 {
		return
	}
	
	mutex.Lock()
	defer mutex.Unlock()
	
	if stats[language] == nil {
		stats[language] = &LanguageStats{
			Language:    language,
			Extensions:  make(map[string]int),
			BuildFiles:  []string{},
			ConfigFiles: []string{},
			SourceDirs:  []string{},
		}
	}
	
	stats[language].FileCount++
	stats[language].Extensions[ext]++
	
	// Check if it's a test file
	if s.isTestFile(fileName, filePath) {
		stats[language].TestFiles++
	}
	
	// Track source directories
	relPath, _ := filepath.Rel(rootPath, filePath)
	dirName := filepath.Dir(relPath)
	if s.isSourceDirectory(dirName) {
		found := false
		for _, existingDir := range stats[language].SourceDirs {
			if existingDir == dirName {
				found = true
				break
			}
		}
		if !found {
			stats[language].SourceDirs = append(stats[language].SourceDirs, dirName)
		}
	}
}

func (s *ProjectLanguageScanner) getFileLanguage(extension string) string {
	for lang, pattern := range s.langPatterns {
		for _, ext := range pattern.Extensions {
			if ext == extension {
				return lang
			}
		}
	}
	return ""
}

func (s *ProjectLanguageScanner) isTestFile(fileName, filePath string) bool {
	lowerName := strings.ToLower(fileName)
	lowerPath := strings.ToLower(filePath)
	
	// Check common test patterns
	testIndicators := []string{
		"_test.", ".test.", "_spec.", ".spec.",
		"test_", "spec_", "tests/", "test/", "__tests__/",
		"testing/", "spec/", "specs/",
	}
	
	for _, indicator := range testIndicators {
		if strings.Contains(lowerName, indicator) || strings.Contains(lowerPath, indicator) {
			return true
		}
	}
	
	return false
}

func (s *ProjectLanguageScanner) isSourceDirectory(dirPath string) bool {
	lowerDir := strings.ToLower(dirPath)
	parts := strings.Split(lowerDir, string(filepath.Separator))
	
	for _, part := range parts {
		for _, sourceDir := range s.sourceDirs {
			if part == sourceDir {
				return true
			}
		}
	}
	return false
}

func (s *ProjectLanguageScanner) calculateLanguagePriorities(stats map[string]*LanguageStats) []LanguageRanking {
	var rankings []LanguageRanking
	
	for _, langStats := range stats {
		pattern := s.langPatterns[langStats.Language]
		if pattern == nil {
			continue
		}
		
		// Base score from file count
		baseScore := float64(langStats.FileCount) * pattern.BaseScore
		
		// Build file bonus
		buildWeight := float64(len(langStats.BuildFiles)) * pattern.BuildWeight
		
		// Source directory structure bonus
		structWeight := float64(len(langStats.SourceDirs)) * 2.0
		
		// Test files penalty (lower priority for test-heavy projects)
		testPenalty := 0.0
		if langStats.FileCount > 0 {
			testRatio := float64(langStats.TestFiles) / float64(langStats.FileCount)
			if testRatio > 0.5 {
				testPenalty = baseScore * 0.1
			}
		}
		
		totalScore := baseScore + buildWeight + structWeight - testPenalty
		
		// Calculate confidence based on multiple indicators
		confidence := 0.0
		if langStats.FileCount > 0 {
			confidence += 0.3
		}
		if len(langStats.BuildFiles) > 0 {
			confidence += 0.4
		}
		if len(langStats.SourceDirs) > 0 {
			confidence += 0.3
		}
		
		rankings = append(rankings, LanguageRanking{
			Language:     langStats.Language,
			Score:        totalScore,
			FileCount:    langStats.FileCount,
			BuildWeight:  buildWeight,
			StructWeight: structWeight,
			Confidence:   confidence,
		})
	}
	
	// Sort by score descending
	sort.Slice(rankings, func(i, j int) bool {
		return rankings[i].Score > rankings[j].Score
	})
	
	return rankings
}

func (s *ProjectLanguageScanner) identifyProjectType(languages []LanguageRanking) string {
	if len(languages) == 0 {
		return "empty"
	}
	
	if len(languages) == 1 {
		return "single-language"
	}
	
	// Calculate language distribution
	totalScore := 0.0
	for _, lang := range languages {
		totalScore += lang.Score
	}
	
	if totalScore == 0 {
		return "unknown"
	}
	
	dominantRatio := languages[0].Score / totalScore
	
	// Single language project if one language dominates (>90%)
	if dominantRatio > 0.9 {
		return "single-language"
	}
	
	// Check for common patterns
	langNames := make([]string, len(languages))
	for i, lang := range languages {
		langNames[i] = lang.Language
	}
	
	// Frontend-backend pattern
	if s.hasFrontendBackendPattern(langNames) {
		return "frontend-backend"
	}
	
	// Microservices pattern (multiple languages with build systems)
	if len(languages) >= 3 {
		buildSystemCount := 0
		for _, lang := range languages {
			if lang.BuildWeight > 0 {
				buildSystemCount++
			}
		}
		if buildSystemCount >= 2 {
			return "microservices"
		}
	}
	
	// Monorepo pattern (multiple languages, clear separation)
	if len(languages) >= 2 && dominantRatio < 0.7 {
		return "monorepo"
	}
	
	// Multi-language (languages intermixed)
	return "multi-language"
}

func (s *ProjectLanguageScanner) hasFrontendBackendPattern(languages []string) bool {
	frontendLangs := map[string]bool{
		"javascript": true, "typescript": true, "html": true, "css": true,
	}
	backendLangs := map[string]bool{
		"go": true, "python": true, "java": true, "csharp": true,
		"rust": true, "php": true, "ruby": true,
	}
	
	hasFrontend := false
	hasBackend := false
	
	for _, lang := range languages {
		if frontendLangs[lang] {
			hasFrontend = true
		}
		if backendLangs[lang] {
			hasBackend = true
		}
	}
	
	return hasFrontend && hasBackend
}

func (s *ProjectLanguageScanner) shouldIgnoreDirectory(dirName string) bool {
	lowerDir := strings.ToLower(dirName)
	
	for _, pattern := range s.IgnorePatterns {
		lowerPattern := strings.ToLower(pattern)
		if strings.HasPrefix(lowerPattern, "*") {
			suffix := strings.TrimPrefix(lowerPattern, "*")
			if strings.HasSuffix(lowerDir, suffix) {
				return true
			}
		} else if strings.HasSuffix(lowerPattern, "*") {
			prefix := strings.TrimSuffix(lowerPattern, "*")
			if strings.HasPrefix(lowerDir, prefix) {
				return true
			}
		} else if lowerDir == lowerPattern {
			return true
		}
	}
	
	// Additional dynamic ignores for large directories
	if strings.HasPrefix(dirName, ".") && len(dirName) > 1 {
		return true
	}
	
	return false
}

func (s *ProjectLanguageScanner) removeDuplicates(slice []string) []string {
	keys := make(map[string]bool)
	var result []string
	
	for _, item := range slice {
		if !keys[item] {
			keys[item] = true
			result = append(result, item)
		}
	}
	
	return result
}

// Additional utility methods for integration

func (s *ProjectLanguageScanner) SetMaxDepth(depth int) {
	if depth > 0 && depth <= 10 {
		s.MaxDepth = depth
	}
}

func (s *ProjectLanguageScanner) SetFileSizeLimit(limit int64) {
	if limit > 0 {
		s.FileSizeLimit = limit
	}
}

func (s *ProjectLanguageScanner) SetMaxFiles(maxFiles int) {
	if maxFiles > 0 {
		s.MaxFiles = maxFiles
	}
}

func (s *ProjectLanguageScanner) AddIgnorePattern(pattern string) {
	s.IgnorePatterns = append(s.IgnorePatterns, pattern)
}

func (s *ProjectLanguageScanner) GetSupportedLanguages() []string {
	var languages []string
	for lang := range s.langPatterns {
		languages = append(languages, lang)
	}
	sort.Strings(languages)
	return languages
}

func (s *ProjectLanguageScanner) ValidateConfiguration() error {
	if s.MaxDepth <= 0 || s.MaxDepth > 10 {
		return fmt.Errorf("max depth must be between 1 and 10, got %d", s.MaxDepth)
	}
	
	if s.FileSizeLimit <= 0 {
		return fmt.Errorf("file size limit must be positive, got %d", s.FileSizeLimit)
	}
	
	if s.MaxFiles <= 0 {
		return fmt.Errorf("max files must be positive, got %d", s.MaxFiles)
	}
	
	if s.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive, got %v", s.Timeout)
	}
	
	return nil
}

// =============================================================================
// MultiLanguageProjectInfo Utility Methods
// =============================================================================

// GetPrimaryLanguages returns languages with priority >= 80
func (info *MultiLanguageProjectInfo) GetPrimaryLanguages() []string {
	var primary []string
	for _, ctx := range info.Languages {
		if ctx.Priority >= 80 {
			primary = append(primary, ctx.Language)
		}
	}
	sort.Strings(primary)
	return primary
}

// GetSecondaryLanguages returns languages with priority between 40 and 79
func (info *MultiLanguageProjectInfo) GetSecondaryLanguages() []string {
	var secondary []string
	for _, ctx := range info.Languages {
		if ctx.Priority >= 40 && ctx.Priority < 80 {
			secondary = append(secondary, ctx.Language)
		}
	}
	sort.Strings(secondary)
	return secondary
}

// HasLanguage checks if a specific language is detected in the project
func (info *MultiLanguageProjectInfo) HasLanguage(language string) bool {
	_, exists := info.Languages[language]
	return exists
}

// GetLanguageContext returns the language context for a specific language
func (info *MultiLanguageProjectInfo) GetLanguageContext(language string) *LanguageContext {
	return info.Languages[language]
}

// GetRecommendedLSPServers returns a list of recommended LSP servers based on detected languages
func (info *MultiLanguageProjectInfo) GetRecommendedLSPServers() []string {
	var servers []string
	serverSet := make(map[string]bool)
	
	for _, ctx := range info.Languages {
		if ctx.LSPServerName != "" && !serverSet[ctx.LSPServerName] {
			servers = append(servers, ctx.LSPServerName)
			serverSet[ctx.LSPServerName] = true
		}
	}
	
	sort.Strings(servers)
	return servers
}

// GetWorkspaceRoot returns the appropriate workspace root for a language
func (info *MultiLanguageProjectInfo) GetWorkspaceRoot(language string) string {
	if root, exists := info.WorkspaceRoots[language]; exists {
		return root
	}
	return info.RootPath
}

// IsMonorepo determines if the project is a monorepo
func (info *MultiLanguageProjectInfo) IsMonorepo() bool {
	return info.ProjectType == config.ProjectTypeMonorepo
}

// IsPolyglot determines if the project uses multiple programming languages
func (info *MultiLanguageProjectInfo) IsPolyglot() bool {
	return len(info.Languages) > 1
}

// ToProjectContext converts MultiLanguageProjectInfo to legacy ProjectContext for backward compatibility
func (info *MultiLanguageProjectInfo) ToProjectContext() *config.ProjectContext {
	languages := make([]config.LanguageInfo, 0, len(info.Languages))
	requiredLSPs := make([]string, 0)
	lspSet := make(map[string]bool)
	
	for _, ctx := range info.Languages {
		langInfo := config.LanguageInfo{
			Language:     ctx.Language,
			FilePatterns: ctx.FileExtensions,
			FileCount:    ctx.FileCount,
			RootMarkers:  ctx.BuildFiles,
		}
		languages = append(languages, langInfo)
		
		if ctx.LSPServerName != "" && !lspSet[ctx.LSPServerName] {
			requiredLSPs = append(requiredLSPs, ctx.LSPServerName)
			lspSet[ctx.LSPServerName] = true
		}
	}
	
	return &config.ProjectContext{
		ProjectType:   info.ProjectType,
		RootDirectory: info.RootPath,
		WorkspaceRoot: info.GetWorkspaceRoot(info.DominantLanguage),
		Languages:     languages,
		RequiredLSPs:  requiredLSPs,
		DetectedAt:    info.DetectedAt,
		Metadata:      info.Metadata,
	}
}

// String provides a human-readable representation of the project info
func (info *MultiLanguageProjectInfo) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Project: %s (%s)\n", info.RootPath, info.ProjectType))
	sb.WriteString(fmt.Sprintf("Dominant Language: %s\n", info.DominantLanguage))
	sb.WriteString(fmt.Sprintf("Languages Found: %d\n", len(info.Languages)))
	
	for lang, ctx := range info.Languages {
		sb.WriteString(fmt.Sprintf("  - %s: %d files, priority %d, confidence %.2f\n",
			lang, ctx.FileCount, ctx.Priority, ctx.Confidence))
		if ctx.Framework != "" {
			sb.WriteString(fmt.Sprintf("    Framework: %s", ctx.Framework))
			if ctx.Version != "" {
				sb.WriteString(fmt.Sprintf(" (%s)", ctx.Version))
			}
			sb.WriteString("\n")
		}
	}
	
	sb.WriteString(fmt.Sprintf("Total Files: %d\n", info.TotalFileCount))
	sb.WriteString(fmt.Sprintf("Scan Duration: %v\n", info.ScanDuration))
	return sb.String()
}

// Validate validates the MultiLanguageProjectInfo structure
func (info *MultiLanguageProjectInfo) Validate() error {
	if info.RootPath == "" {
		return &ProjectDetectionError{
			Type:    "validation",
			Message: "root path cannot be empty",
		}
	}
	
	if !filepath.IsAbs(info.RootPath) {
		return &ProjectDetectionError{
			Type:    "validation",
			Message: "root path must be absolute",
			Path:    info.RootPath,
		}
	}
	
	if info.ProjectType == "" {
		return &ProjectDetectionError{
			Type:    "validation",
			Message: "project type cannot be empty",
		}
	}
	
	validTypes := map[string]bool{
		config.ProjectTypeSingle:          true,
		config.ProjectTypeMulti:           true,
		config.ProjectTypeMonorepo:        true,
		config.ProjectTypeWorkspace:       true,
		config.ProjectTypeFrontendBackend: true,
		config.ProjectTypeMicroservices:   true,
		config.ProjectTypePolyglot:        true,
		config.ProjectTypeEmpty:           true,
		config.ProjectTypeUnknown:         true,
	}
	
	if !validTypes[info.ProjectType] {
		return &ProjectDetectionError{
			Type:    "validation",
			Message: fmt.Sprintf("invalid project type: %s", info.ProjectType),
		}
	}
	
	if info.Languages == nil {
		info.Languages = make(map[string]*LanguageContext)
	}
	
	if info.WorkspaceRoots == nil {
		info.WorkspaceRoots = make(map[string]string)
	}
	
	if info.Metadata == nil {
		info.Metadata = make(map[string]interface{})
	}
	
	// Validate language contexts
	for lang, ctx := range info.Languages {
		if err := ctx.Validate(); err != nil {
			return &ProjectDetectionError{
				Type:    "validation",
				Message: fmt.Sprintf("language context validation failed for %s: %v", lang, err),
				Context: lang,
			}
		}
	}
	
	if info.DetectedAt.IsZero() {
		return &ProjectDetectionError{
			Type:    "validation",
			Message: "detected_at timestamp cannot be zero",
		}
	}
	
	return nil
}

// =============================================================================
// LanguageContext Utility Methods
// =============================================================================

// GetMainSourcePath returns the primary source directory for the language
func (ctx *LanguageContext) GetMainSourcePath() string {
	if len(ctx.SourcePaths) > 0 {
		return ctx.SourcePaths[0]
	}
	return ""
}

// GetMainTestPath returns the primary test directory for the language
func (ctx *LanguageContext) GetMainTestPath() string {
	if len(ctx.TestPaths) > 0 {
		return ctx.TestPaths[0]
	}
	return ""
}

// HasFramework checks if a framework is detected for this language
func (ctx *LanguageContext) HasFramework() bool {
	return ctx.Framework != ""
}

// GetPreferredServerConfig returns the preferred server configuration for this language
func (ctx *LanguageContext) GetPreferredServerConfig() *config.ServerConfig {
	if ctx.LSPServerName == "" {
		return nil
	}
	
	// This would be enhanced to return actual server config based on LSPServerName
	// For now, return a basic config structure
	return &config.ServerConfig{
		Name:        ctx.LSPServerName,
		Languages:   []string{ctx.Language},
		RootMarkers: ctx.BuildFiles,
	}
}

// EstimateProjectSize estimates the project size based on file count and structure
func (ctx *LanguageContext) EstimateProjectSize() string {
	totalFiles := ctx.FileCount + ctx.TestFileCount
	
	if totalFiles < 50 {
		return "small"
	} else if totalFiles < 500 {
		return "medium"
	} else {
		return "large"
	}
}

// GetDependencyLanguages returns languages this language depends on
func (ctx *LanguageContext) GetDependencyLanguages() []string {
	return ctx.Dependencies
}

// Validate validates the LanguageContext structure
func (ctx *LanguageContext) Validate() error {
	if ctx.Language == "" {
		return fmt.Errorf("language cannot be empty")
	}
	
	if ctx.RootPath != "" && !filepath.IsAbs(ctx.RootPath) {
		return fmt.Errorf("root path must be absolute: %s", ctx.RootPath)
	}
	
	if ctx.FileCount < 0 {
		return fmt.Errorf("file count cannot be negative: %d", ctx.FileCount)
	}
	
	if ctx.TestFileCount < 0 {
		return fmt.Errorf("test file count cannot be negative: %d", ctx.TestFileCount)
	}
	
	if ctx.Priority < 0 || ctx.Priority > 100 {
		return fmt.Errorf("priority must be between 0 and 100: %d", ctx.Priority)
	}
	
	if ctx.Confidence < 0.0 || ctx.Confidence > 1.0 {
		return fmt.Errorf("confidence must be between 0.0 and 1.0: %f", ctx.Confidence)
	}
	
	if ctx.BuildFiles == nil {
		ctx.BuildFiles = []string{}
	}
	
	if ctx.ConfigFiles == nil {
		ctx.ConfigFiles = []string{}
	}
	
	if ctx.SourcePaths == nil {
		ctx.SourcePaths = []string{}
	}
	
	if ctx.TestPaths == nil {
		ctx.TestPaths = []string{}
	}
	
	if ctx.FileExtensions == nil {
		ctx.FileExtensions = []string{}
	}
	
	if ctx.Dependencies == nil {
		ctx.Dependencies = []string{}
	}
	
	if ctx.Metadata == nil {
		ctx.Metadata = make(map[string]interface{})
	}
	
	return nil
}

// =============================================================================
// Framework Detection
// =============================================================================

// FrameworkDetector handles framework detection for different languages
type FrameworkDetector struct {
	frameworkPatterns map[string][]FrameworkPattern
}

// FrameworkPattern defines patterns for detecting frameworks
type FrameworkPattern struct {
	Name           string
	Language       string
	ConfigFiles    []string
	PackageFiles   []string
	Dependencies   []string
	DirectoryHints []string
	ContentHints   map[string][]string // file -> content patterns
	Confidence     float64
}

// NewFrameworkDetector creates a new framework detector with predefined patterns
func NewFrameworkDetector() *FrameworkDetector {
	detector := &FrameworkDetector{
		frameworkPatterns: make(map[string][]FrameworkPattern),
	}
	detector.initializeFrameworkPatterns()
	return detector
}

// initializeFrameworkPatterns sets up framework detection patterns
func (fd *FrameworkDetector) initializeFrameworkPatterns() {
	patterns := []FrameworkPattern{
		// JavaScript/TypeScript Frameworks
		{
			Name:         "React",
			Language:     "javascript",
			PackageFiles: []string{"package.json"},
			Dependencies: []string{"react", "@types/react"},
			DirectoryHints: []string{"src/components", "public"},
			ContentHints: map[string][]string{
				"package.json": {"\"react\":", "\"@types/react\":"},
			},
			Confidence: 0.9,
		},
		{
			Name:         "React",
			Language:     "typescript",
			PackageFiles: []string{"package.json", "tsconfig.json"},
			Dependencies: []string{"react", "@types/react", "typescript"},
			DirectoryHints: []string{"src/components", "public"},
			ContentHints: map[string][]string{
				"package.json": {"\"react\":", "\"typescript\":"},
				"tsconfig.json": {"\"jsx\":", "\"react\""},
			},
			Confidence: 0.95,
		},
		{
			Name:         "Vue.js",
			Language:     "javascript",
			PackageFiles: []string{"package.json"},
			Dependencies: []string{"vue", "@vue/cli"},
			DirectoryHints: []string{"src/components", "src/views"},
			ContentHints: map[string][]string{
				"package.json": {"\"vue\":", "\"@vue/"},
			},
			Confidence: 0.9,
		},
		{
			Name:         "Angular",
			Language:     "typescript",
			ConfigFiles:  []string{"angular.json", ".angular-cli.json"},
			PackageFiles: []string{"package.json"},
			Dependencies: []string{"@angular/core", "@angular/cli"},
			DirectoryHints: []string{"src/app", "src/environments"},
			ContentHints: map[string][]string{
				"package.json": {"\"@angular/core\":", "\"@angular/cli\":"},
			},
			Confidence: 0.95,
		},
		{
			Name:         "Next.js",
			Language:     "javascript",
			ConfigFiles:  []string{"next.config.js", "next.config.ts"},
			PackageFiles: []string{"package.json"},
			Dependencies: []string{"next", "react"},
			DirectoryHints: []string{"pages", "app", "public"},
			ContentHints: map[string][]string{
				"package.json": {"\"next\":", "\"scripts\".*\"dev\".*\"next\""},
			},
			Confidence: 0.9,
		},
		{
			Name:         "Express.js",
			Language:     "javascript",
			PackageFiles: []string{"package.json"},
			Dependencies: []string{"express"},
			DirectoryHints: []string{"routes", "middleware"},
			ContentHints: map[string][]string{
				"package.json": {"\"express\":"},
			},
			Confidence: 0.8,
		},
		
		// Python Frameworks
		{
			Name:         "Django",
			Language:     "python",
			ConfigFiles:  []string{"manage.py", "settings.py"},
			PackageFiles: []string{"requirements.txt", "pyproject.toml", "Pipfile"},
			Dependencies: []string{"Django", "django"},
			DirectoryHints: []string{"apps", "templates", "static"},
			ContentHints: map[string][]string{
				"requirements.txt": {"Django", "django"},
				"manage.py": {"django.core.management"},
			},
			Confidence: 0.95,
		},
		{
			Name:         "Flask",
			Language:     "python",
			PackageFiles: []string{"requirements.txt", "pyproject.toml"},
			Dependencies: []string{"Flask", "flask"},
			DirectoryHints: []string{"templates", "static"},
			ContentHints: map[string][]string{
				"requirements.txt": {"Flask", "flask"},
			},
			Confidence: 0.8,
		},
		{
			Name:         "FastAPI",
			Language:     "python",
			PackageFiles: []string{"requirements.txt", "pyproject.toml"},
			Dependencies: []string{"fastapi", "uvicorn"},
			ContentHints: map[string][]string{
				"requirements.txt": {"fastapi", "uvicorn"},
			},
			Confidence: 0.85,
		},
		
		// Go Frameworks
		{
			Name:         "Gin",
			Language:     "go",
			PackageFiles: []string{"go.mod"},
			Dependencies: []string{"github.com/gin-gonic/gin"},
			ContentHints: map[string][]string{
				"go.mod": {"github.com/gin-gonic/gin"},
			},
			Confidence: 0.9,
		},
		{
			Name:         "Echo",
			Language:     "go",
			PackageFiles: []string{"go.mod"},
			Dependencies: []string{"github.com/labstack/echo"},
			ContentHints: map[string][]string{
				"go.mod": {"github.com/labstack/echo"},
			},
			Confidence: 0.9,
		},
		{
			Name:         "Fiber",
			Language:     "go",
			PackageFiles: []string{"go.mod"},
			Dependencies: []string{"github.com/gofiber/fiber"},
			ContentHints: map[string][]string{
				"go.mod": {"github.com/gofiber/fiber"},
			},
			Confidence: 0.9,
		},
		
		// Java Frameworks
		{
			Name:         "Spring Boot",
			Language:     "java",
			ConfigFiles:  []string{"application.properties", "application.yml"},
			PackageFiles: []string{"pom.xml", "build.gradle"},
			Dependencies: []string{"spring-boot-starter", "org.springframework.boot"},
			DirectoryHints: []string{"src/main/java", "src/main/resources"},
			ContentHints: map[string][]string{
				"pom.xml": {"spring-boot-starter", "org.springframework.boot"},
				"build.gradle": {"spring-boot-starter", "org.springframework.boot"},
			},
			Confidence: 0.95,
		},
		{
			Name:         "Micronaut",
			Language:     "java",
			ConfigFiles:  []string{"application.yml", "bootstrap.yml"},
			PackageFiles: []string{"pom.xml", "build.gradle"},
			Dependencies: []string{"micronaut-core", "io.micronaut"},
			ContentHints: map[string][]string{
				"pom.xml": {"micronaut-core", "io.micronaut"},
				"build.gradle": {"micronaut-core", "io.micronaut"},
			},
			Confidence: 0.9,
		},
		
		// Rust Frameworks
		{
			Name:         "Axum",
			Language:     "rust",
			PackageFiles: []string{"Cargo.toml"},
			Dependencies: []string{"axum", "tokio"},
			ContentHints: map[string][]string{
				"Cargo.toml": {"axum =", "tokio ="},
			},
			Confidence: 0.9,
		},
		{
			Name:         "Actix",
			Language:     "rust",
			PackageFiles: []string{"Cargo.toml"},
			Dependencies: []string{"actix-web"},
			ContentHints: map[string][]string{
				"Cargo.toml": {"actix-web ="},
			},
			Confidence: 0.9,
		},
	}
	
	// Group patterns by language
	for _, pattern := range patterns {
		fd.frameworkPatterns[pattern.Language] = append(fd.frameworkPatterns[pattern.Language], pattern)
	}
}

// DetectFrameworks detects frameworks for a language context
func (fd *FrameworkDetector) DetectFrameworks(ctx *LanguageContext, projectRoot string) []Framework {
	patterns, exists := fd.frameworkPatterns[ctx.Language]
	if !exists {
		return []Framework{}
	}
	
	var detected []Framework
	
	for _, pattern := range patterns {
		if framework := fd.matchPattern(pattern, ctx, projectRoot); framework != nil {
			detected = append(detected, *framework)
		}
	}
	
	// Sort by confidence (highest first)
	sort.Slice(detected, func(i, j int) bool {
		return detected[i].Confidence > detected[j].Confidence
	})
	
	return detected
}

// matchPattern checks if a framework pattern matches the language context
func (fd *FrameworkDetector) matchPattern(pattern FrameworkPattern, ctx *LanguageContext, projectRoot string) *Framework {
	confidence := 0.0
	var configFile string
	
	// Check config files
	for _, configFileName := range pattern.ConfigFiles {
		for _, ctxConfig := range ctx.ConfigFiles {
			if strings.Contains(ctxConfig, configFileName) {
				confidence += 0.3
				if configFile == "" {
					configFile = ctxConfig
				}
				break
			}
		}
	}
	
	// Check package files and their content
	for _, packageFileName := range pattern.PackageFiles {
		for _, ctxConfig := range append(ctx.ConfigFiles, ctx.BuildFiles...) {
			if strings.Contains(ctxConfig, packageFileName) {
				confidence += 0.2
				
				// Check content hints if available
				if contentHints, exists := pattern.ContentHints[packageFileName]; exists {
					if fd.checkFileContent(ctxConfig, contentHints) {
						confidence += 0.4
					}
				}
				break
			}
		}
	}
	
	// Check directory hints
	for _, dirHint := range pattern.DirectoryHints {
		for _, sourcePath := range ctx.SourcePaths {
			if strings.Contains(sourcePath, dirHint) {
				confidence += 0.1
				break
			}
		}
	}
	
	// Only consider it a match if confidence is above threshold
	if confidence >= 0.5 {
		return &Framework{
			Name:       pattern.Name,
			Language:   pattern.Language,
			ConfigFile: configFile,
			Confidence: confidence,
			Metadata: map[string]interface{}{
				"detection_method": "pattern_matching",
				"pattern_confidence": pattern.Confidence,
			},
		}
	}
	
	return nil
}

// checkFileContent checks if file contains any of the content hints
func (fd *FrameworkDetector) checkFileContent(filePath string, hints []string) bool {
	// This is a simplified implementation
	// In a real implementation, you would read the file and check for patterns
	for _, hint := range hints {
		// For now, just check if the hint pattern suggests a match
		// In practice, you'd read the file content and use regex matching
		if strings.Contains(strings.ToLower(filePath), strings.ToLower(hint)) {
			return true
		}
	}
	return false
}

// =============================================================================
// Dependency Analysis
// =============================================================================

// DependencyAnalyzer analyzes cross-language dependencies
type DependencyAnalyzer struct {
	sharedConfigPatterns map[string][]string
	apiBoundaryPatterns  map[string][]string
}

// NewDependencyAnalyzer creates a new dependency analyzer
func NewDependencyAnalyzer() *DependencyAnalyzer {
	return &DependencyAnalyzer{
		sharedConfigPatterns: map[string][]string{
			"docker": {"Dockerfile", "docker-compose.yml", ".dockerignore"},
			"kubernetes": {"*.yaml", "*.yml", "kustomization.yaml"},
			"terraform": {"*.tf", "terraform.tfvars"},
			"make": {"Makefile", "makefile"},
			"ci": {".github/workflows", ".gitlab-ci.yml", "Jenkinsfile", ".travis.yml"},
		},
		apiBoundaryPatterns: map[string][]string{
			"openapi": {"openapi.yaml", "swagger.yaml", "api.yaml"},
			"graphql": {"schema.graphql", "*.graphql"},
			"protobuf": {"*.proto"},
			"rest": {"api/", "endpoints/", "routes/"},
		},
	}
}

// AnalyzeDependencies analyzes dependencies between languages in the project
func (da *DependencyAnalyzer) AnalyzeDependencies(info *MultiLanguageProjectInfo) {
	// Analyze shared configuration files
	sharedConfigs := da.findSharedConfigurations(info)
	
	// Analyze API boundaries
	apiBoundaries := da.findAPIBoundaries(info)
	
	// Update language contexts with dependency information
	for _, ctx := range info.Languages {
		ctx.Dependencies = da.calculateDependencies(ctx, info, sharedConfigs, apiBoundaries)
	}
	
	// Add dependency metadata
	if info.Metadata == nil {
		info.Metadata = make(map[string]interface{})
	}
	info.Metadata["shared_configurations"] = sharedConfigs
	info.Metadata["api_boundaries"] = apiBoundaries
	info.Metadata["dependency_analysis"] = map[string]interface{}{
		"analyzed_at": time.Now(),
		"has_shared_build": len(sharedConfigs["build"]) > 0,
		"has_api_contracts": len(apiBoundaries) > 0,
	}
}

// findSharedConfigurations identifies shared configuration files
func (da *DependencyAnalyzer) findSharedConfigurations(info *MultiLanguageProjectInfo) map[string][]string {
	shared := make(map[string][]string)
	
	// Check build files and config files for shared patterns
	allFiles := append(info.BuildFiles, info.ConfigFiles...)
	
	for category, patterns := range da.sharedConfigPatterns {
		for _, file := range allFiles {
			fileName := filepath.Base(file)
			for _, pattern := range patterns {
				if matched, _ := filepath.Match(pattern, fileName); matched {
					shared[category] = append(shared[category], file)
				}
			}
		}
	}
	
	return shared
}

// findAPIBoundaries identifies API boundary definitions
func (da *DependencyAnalyzer) findAPIBoundaries(info *MultiLanguageProjectInfo) map[string][]string {
	boundaries := make(map[string][]string)
	
	allFiles := append(info.BuildFiles, info.ConfigFiles...)
	
	for category, patterns := range da.apiBoundaryPatterns {
		for _, file := range allFiles {
			fileName := filepath.Base(file)
			dirName := filepath.Dir(file)
			
			for _, pattern := range patterns {
				if matched, _ := filepath.Match(pattern, fileName); matched {
					boundaries[category] = append(boundaries[category], file)
				} else if strings.Contains(dirName, strings.TrimSuffix(pattern, "/")) {
					boundaries[category] = append(boundaries[category], file)
				}
			}
		}
	}
	
	return boundaries
}

// calculateDependencies calculates dependencies for a language context
func (da *DependencyAnalyzer) calculateDependencies(ctx *LanguageContext, info *MultiLanguageProjectInfo, sharedConfigs, apiBoundaries map[string][]string) []string {
	var dependencies []string
	depSet := make(map[string]bool)
	
	// Check for shared build systems
	if len(sharedConfigs["docker"]) > 0 {
		// Languages that share Docker configuration likely depend on each other
		for lang := range info.Languages {
			if lang != ctx.Language && !depSet[lang] {
				dependencies = append(dependencies, lang)
				depSet[lang] = true
			}
		}
	}
	
	// Check for API boundaries that suggest service communication
	if len(apiBoundaries["rest"]) > 0 || len(apiBoundaries["graphql"]) > 0 {
		// If there are REST/GraphQL APIs, frontend languages depend on backend languages
		frontendLangs := map[string]bool{"javascript": true, "typescript": true}
		backendLangs := map[string]bool{"go": true, "python": true, "java": true, "rust": true}
		
		if frontendLangs[ctx.Language] {
			for lang := range info.Languages {
				if backendLangs[lang] && !depSet[lang] {
					dependencies = append(dependencies, lang)
					depSet[lang] = true
				}
			}
		}
	}
	
	// Check for shared libraries or modules (simplified heuristic)
	if len(info.Languages) >= 2 && len(info.Languages) <= 4 {
		// For small multi-language projects, assume some level of interdependency
		for lang := range info.Languages {
			if lang != ctx.Language && len(dependencies) < 2 && !depSet[lang] {
				dependencies = append(dependencies, lang)
				depSet[lang] = true
			}
		}
	}
	
	return dependencies
}

// =============================================================================
// Enhanced Scanner Integration
// =============================================================================

// ScanProjectComprehensive performs comprehensive project analysis with framework detection and dependency analysis
func (s *ProjectLanguageScanner) ScanProjectComprehensive(rootPath string) (*MultiLanguageProjectInfo, error) {
	// Perform basic scan first
	basicInfo, err := s.ScanProject(rootPath)
	if err != nil {
		return nil, err
	}
	
	// Convert to comprehensive structure
	info := s.convertToComprehensiveInfo(basicInfo)
	
	// Perform framework detection
	frameworkDetector := NewFrameworkDetector()
	for _, ctx := range info.Languages {
		frameworks := frameworkDetector.DetectFrameworks(ctx, rootPath)
		if len(frameworks) > 0 {
			// Take the highest confidence framework
			ctx.Framework = frameworks[0].Name
			ctx.Version = frameworks[0].Version
			ctx.Metadata["detected_frameworks"] = frameworks
		}
		
		// Set recommended LSP server based on language and framework
		ctx.LSPServerName = s.getRecommendedLSPServer(ctx.Language, ctx.Framework)
	}
	
	// Perform dependency analysis
	dependencyAnalyzer := NewDependencyAnalyzer()
	dependencyAnalyzer.AnalyzeDependencies(info)
	
	// Validate the result
	if err := info.Validate(); err != nil {
		return nil, fmt.Errorf("comprehensive scan validation failed: %w", err)
	}
	
	return info, nil
}

// convertToComprehensiveInfo converts basic scan results to comprehensive structure
func (s *ProjectLanguageScanner) convertToComprehensiveInfo(basicInfo *MultiLanguageProjectInfo) *MultiLanguageProjectInfo {
	info := &MultiLanguageProjectInfo{
		RootPath:         basicInfo.RootPath,
		ProjectType:      basicInfo.ProjectType,
		DominantLanguage: basicInfo.DominantLanguage,
		Languages:        make(map[string]*LanguageContext),
		WorkspaceRoots:   make(map[string]string),
		BuildFiles:       basicInfo.BuildFiles,
		ConfigFiles:      basicInfo.ConfigFiles,
		TotalFileCount:   basicInfo.TotalFiles,
		ScanDepth:        s.MaxDepth,
		ScanDuration:     basicInfo.ScanDuration,
		DetectedAt:       time.Now(),
		Metadata:         basicInfo.Metadata,
	}
	
	// Convert language rankings to language contexts
	for _, ranking := range basicInfo.Languages {
		ctx := &LanguageContext{
			Language:       ranking.Language,
			RootPath:       basicInfo.RootPath,
			FileCount:      ranking.FileCount,
			Priority:       int(ranking.Score),
			Confidence:     ranking.Confidence,
			BuildFiles:     []string{},
			ConfigFiles:    []string{},
			SourcePaths:    []string{},
			TestPaths:      []string{},
			FileExtensions: []string{},
			Dependencies:   []string{},
			Metadata:       make(map[string]interface{}),
		}
		
		// Set workspace root (for now, same as project root)
		info.WorkspaceRoots[ranking.Language] = basicInfo.RootPath
		info.Languages[ranking.Language] = ctx
	}
	
	return info
}

// getRecommendedLSPServer returns the recommended LSP server for a language/framework combination
func (s *ProjectLanguageScanner) getRecommendedLSPServer(language, framework string) string {
	serverMap := map[string]string{
		"go":         "gopls",
		"python":     "pylsp",
		"javascript": "typescript-language-server",
		"typescript": "typescript-language-server",
		"java":       "jdtls",
		"kotlin":     "kotlin-language-server",
		"rust":       "rust-analyzer",
		"cpp":        "clangd",
		"c":          "clangd",
		"csharp":     "omnisharp",
		"ruby":       "solargraph",
		"php":        "intelephense",
		"swift":      "sourcekit-lsp",
	}
	
	// Framework-specific overrides
	if framework != "" {
		frameworkServers := map[string]map[string]string{
			"javascript": {
				"React":    "typescript-language-server",
				"Vue.js":   "vetur",
				"Angular":  "typescript-language-server",
			},
			"python": {
				"Django":  "pylsp",
				"Flask":   "pylsp",
				"FastAPI": "pylsp",
			},
		}
		
		if langServers, exists := frameworkServers[language]; exists {
			if server, exists := langServers[framework]; exists {
				return server
			}
		}
	}
	
	if server, exists := serverMap[language]; exists {
		return server
	}
	
	return ""
}

// =============================================================================
// Performance Optimization Methods
// =============================================================================

// tryFastPathDetection attempts fast detection for obvious single-language projects
func (s *ProjectLanguageScanner) tryFastPathDetection(rootPath string) (*MultiLanguageProjectInfo, bool) {
	// Check fast-path cache first
	s.fastPathMutex.RLock()
	if cached, exists := s.fastPathCache[rootPath]; exists {
		s.fastPathMutex.RUnlock()
		return cached, true
	}
	s.fastPathMutex.RUnlock()
	
	// Quick file system checks for obvious patterns
	if info, isFastPath := s.detectObviousPatterns(rootPath); isFastPath {
		// Cache the fast-path result
		s.fastPathMutex.Lock()
		s.fastPathCache[rootPath] = info
		s.fastPathMutex.Unlock()
		return info, true
	}
	
	return nil, false
}

// detectObviousPatterns checks for obvious single-language project patterns
func (s *ProjectLanguageScanner) detectObviousPatterns(rootPath string) (*MultiLanguageProjectInfo, bool) {
	// Check for Go project
	if s.hasFile(rootPath, "go.mod") {
		return s.createFastPathResult(rootPath, "go", "single-language", []string{"go.mod"}), true
	}
	
	// Check for Rust project
	if s.hasFile(rootPath, "Cargo.toml") {
		return s.createFastPathResult(rootPath, "rust", "single-language", []string{"Cargo.toml"}), true
	}
	
	// Check for Node.js project without TypeScript
	if s.hasFile(rootPath, "package.json") && !s.hasFile(rootPath, "tsconfig.json") {
		return s.createFastPathResult(rootPath, "javascript", "single-language", []string{"package.json"}), true
	}
	
	// Check for Python project
	if s.hasFile(rootPath, "setup.py") || s.hasFile(rootPath, "pyproject.toml") {
		var buildFiles []string
		if s.hasFile(rootPath, "setup.py") {
			buildFiles = append(buildFiles, "setup.py")
		}
		if s.hasFile(rootPath, "pyproject.toml") {
			buildFiles = append(buildFiles, "pyproject.toml")
		}
		return s.createFastPathResult(rootPath, "python", "single-language", buildFiles), true
	}
	
	// Check for Java project
	if s.hasFile(rootPath, "pom.xml") || s.hasFile(rootPath, "build.gradle") {
		var buildFiles []string
		if s.hasFile(rootPath, "pom.xml") {
			buildFiles = append(buildFiles, "pom.xml")
		}
		if s.hasFile(rootPath, "build.gradle") {
			buildFiles = append(buildFiles, "build.gradle")
		}
		return s.createFastPathResult(rootPath, "java", "single-language", buildFiles), true
	}
	
	return nil, false
}

// hasFile checks if a file exists in the root directory
func (s *ProjectLanguageScanner) hasFile(rootPath, filename string) bool {
	filePath := filepath.Join(rootPath, filename)
	_, err := os.Stat(filePath)
	return err == nil
}

// createFastPathResult creates a fast-path project info result
func (s *ProjectLanguageScanner) createFastPathResult(rootPath, language, projectType string, buildFiles []string) *MultiLanguageProjectInfo {
	ctx := &LanguageContext{
		Language:       language,
		RootPath:       rootPath,
		FileCount:      1, // Minimal estimate for fast path
		TestFileCount:  0,
		Priority:       100, // High priority for obvious match
		Confidence:     0.95, // High confidence for fast path
		BuildFiles:     buildFiles,
		ConfigFiles:    []string{},
		SourcePaths:    []string{"src"},
		TestPaths:      []string{"test"},
		FileExtensions: s.getLanguageExtensions(language),
		Dependencies:   []string{},
		Metadata:       map[string]interface{}{"detection_method": "fast_path"},
	}
	
	languages := make(map[string]*LanguageContext)
	languages[language] = ctx
	
	workspaceRoots := make(map[string]string)
	workspaceRoots[language] = rootPath
	
	return &MultiLanguageProjectInfo{
		RootPath:         rootPath,
		ProjectType:      projectType,
		DominantLanguage: language,
		Languages:        languages,
		WorkspaceRoots:   workspaceRoots,
		BuildFiles:       buildFiles,
		ConfigFiles:      []string{},
		TotalFileCount:   1,
		ScanDepth:        1,
		ScanDuration:     time.Millisecond, // Very fast
		DetectedAt:       time.Now(),
		Metadata: map[string]interface{}{
			"detection_method": "fast_path",
			"scan_depth":       1,
			"file_limit":       s.MaxFiles,
			"languages_found":  1,
		},
	}
}

// getLanguageExtensions returns file extensions for a language
func (s *ProjectLanguageScanner) getLanguageExtensions(language string) []string {
	if pattern, exists := s.langPatterns[language]; exists {
		return pattern.Extensions
	}
	return []string{}
}

// addEarlyTerminationContext adds early termination logic to the context
func (s *ProjectLanguageScanner) addEarlyTerminationContext(ctx context.Context, rootPath string) context.Context {
	// Create a context that can be cancelled early based on project size estimation
	ctx, cancel := context.WithCancel(ctx)
	
	go func() {
		// Quick directory size estimation
		if s.estimateProjectSize(rootPath) < s.FastModeThreshold {
			// Small project, no need for early termination
			return
		}
		
		// For large projects, implement intelligent early termination
		// This is a placeholder for more sophisticated logic
		select {
		case <-time.After(s.Timeout / 2): // Terminate at half timeout for very large projects
			cancel()
		case <-ctx.Done():
			return
		}
	}()
	
	return ctx
}

// estimateProjectSize provides a quick estimate of project size
func (s *ProjectLanguageScanner) estimateProjectSize(rootPath string) int {
	// Quick estimation by counting top-level entries
	entries, err := os.ReadDir(rootPath)
	if err != nil {
		return 0
	}
	
	count := 0
	for _, entry := range entries {
		if !s.shouldIgnoreDirectory(entry.Name()) {
			count++
			if entry.IsDir() {
				count += 10 // Rough multiplier for directories
			}
		}
	}
	
	return count
}

// needsFullRescan determines if a full rescan is needed based on timestamp
func (s *ProjectLanguageScanner) needsFullRescan(rootPath string, lastScan time.Time) bool {
	// Check if critical files have been modified since last scan
	criticalFiles := []string{
		"go.mod", "Cargo.toml", "package.json", "pom.xml", "build.gradle",
		"setup.py", "pyproject.toml", "tsconfig.json",
	}
	
	for _, file := range criticalFiles {
		filePath := filepath.Join(rootPath, file)
		if info, err := os.Stat(filePath); err == nil {
			if info.ModTime().After(lastScan) {
				return true
			}
		}
	}
	
	return false
}

// convertLegacyToNewFormat converts legacy ranking format to new MultiLanguageProjectInfo format
func (s *ProjectLanguageScanner) convertLegacyToNewFormat(rankings []LanguageRanking, rootPath, projectType, dominantLang string, buildSystems, sourceDirs, configFiles []string, totalFiles int, scanDuration time.Duration, maxDepth, languagesFound int) *MultiLanguageProjectInfo {
	languages := make(map[string]*LanguageContext)
	workspaceRoots := make(map[string]string)
	
	for _, ranking := range rankings {
		ctx := &LanguageContext{
			Language:       ranking.Language,
			RootPath:       rootPath,
			FileCount:      ranking.FileCount,
			TestFileCount:  0, // This would need to be calculated properly in a full implementation
			Priority:       int(ranking.Score),
			Confidence:     ranking.Confidence,
			BuildFiles:     []string{}, // This would need to be populated from the scan
			ConfigFiles:    []string{}, // This would need to be populated from the scan
			SourcePaths:    []string{}, // This would need to be populated from the scan
			TestPaths:      []string{}, // This would need to be populated from the scan
			FileExtensions: s.getLanguageExtensions(ranking.Language),
			Dependencies:   []string{},
			Metadata:       make(map[string]interface{}),
		}
		
		languages[ranking.Language] = ctx
		workspaceRoots[ranking.Language] = rootPath
	}
	
	return &MultiLanguageProjectInfo{
		RootPath:         rootPath,
		ProjectType:      projectType,
		DominantLanguage: dominantLang,
		Languages:        languages,
		WorkspaceRoots:   workspaceRoots,
		BuildFiles:       buildSystems,
		ConfigFiles:      configFiles,
		TotalFileCount:   totalFiles,
		ScanDepth:        maxDepth,
		ScanDuration:     scanDuration,
		DetectedAt:       time.Now(),
		Metadata: map[string]interface{}{
			"scan_depth":       maxDepth,
			"file_limit":       s.MaxFiles,
			"languages_found":  languagesFound,
			"detection_method": "full_scan",
		},
	}
}

// =============================================================================
// Cache Integration Methods
// =============================================================================

// SetCacheEnabled enables or disables caching
func (s *ProjectLanguageScanner) SetCacheEnabled(enabled bool) {
	s.EnableCache = enabled
	if enabled && s.Cache == nil {
		s.Cache = NewProjectCache()
	} else if !enabled && s.Cache != nil {
		s.Cache.Shutdown()
		s.Cache = nil
	}
}

// GetCacheStats returns cache statistics if caching is enabled
func (s *ProjectLanguageScanner) GetCacheStats() *ProjectCacheStats {
	if s.Cache != nil {
		stats := s.Cache.GetStats()
		return &stats
	}
	return nil
}

// InvalidateCache clears the cache for a specific project
func (s *ProjectLanguageScanner) InvalidateCache(rootPath string) {
	if s.Cache != nil {
		s.Cache.InvalidateProject(rootPath)
	}
	
	// Also clear fast-path cache
	s.fastPathMutex.Lock()
	delete(s.fastPathCache, rootPath)
	s.fastPathMutex.Unlock()
}

// ClearAllCaches clears all caches
func (s *ProjectLanguageScanner) ClearAllCaches() {
	if s.Cache != nil {
		s.Cache.InvalidateAll()
	}
	
	// Clear fast-path cache
	s.fastPathMutex.Lock()
	s.fastPathCache = make(map[string]*MultiLanguageProjectInfo)
	s.fastPathMutex.Unlock()
}

// Shutdown gracefully shuts down the scanner and all background processes
func (s *ProjectLanguageScanner) Shutdown() {
	if s.Cache != nil {
		s.Cache.Shutdown()
	}
}

// =============================================================================
// Performance Configuration Methods
// =============================================================================

// SetFastModeThreshold sets the threshold for switching to fast mode
func (s *ProjectLanguageScanner) SetFastModeThreshold(threshold int) {
	if threshold > 0 {
		s.FastModeThreshold = threshold
	}
}

// SetMaxConcurrentScans sets the maximum number of concurrent scans
func (s *ProjectLanguageScanner) SetMaxConcurrentScans(maxScans int) {
	if maxScans > 0 && maxScans <= 50 { // Reasonable upper limit
		s.MaxConcurrentScans = maxScans
	}
}

// SetEarlyExitEnabled enables or disables early termination
func (s *ProjectLanguageScanner) SetEarlyExitEnabled(enabled bool) {
	s.EnableEarlyExit = enabled
}

// GetPerformanceMetrics returns performance metrics for the scanner
func (s *ProjectLanguageScanner) GetPerformanceMetrics() map[string]interface{} {
	metrics := make(map[string]interface{})
	
	// Add cache metrics if available
	if s.Cache != nil {
		stats := s.Cache.GetStats()
		metrics["cache"] = map[string]interface{}{
			"hit_ratio":         stats.HitRatio,
			"total_entries":     stats.TotalEntries,
			"hit_count":         stats.HitCount,
			"miss_count":        stats.MissCount,
			"background_scans":  stats.BackgroundScans,
			"incremental_updates": stats.IncrementalUpdates,
			"average_access_time": stats.AverageAccessTime.String(),
			"cache_size_bytes":  stats.CacheSize,
		}
	}
	
	// Add fast-path cache metrics
	s.fastPathMutex.RLock()
	fastPathEntries := len(s.fastPathCache)
	s.fastPathMutex.RUnlock()
	
	metrics["fast_path"] = map[string]interface{}{
		"entries": fastPathEntries,
	}
	
	// Add configuration metrics
	metrics["configuration"] = map[string]interface{}{
		"max_depth":           s.MaxDepth,
		"max_files":           s.MaxFiles,
		"fast_mode_threshold": s.FastModeThreshold,
		"max_concurrent_scans": s.MaxConcurrentScans,
		"early_exit_enabled":  s.EnableEarlyExit,
		"cache_enabled":       s.EnableCache,
		"timeout":             s.Timeout.String(),
	}
	
	return metrics
}

// =============================================================================
// Advanced Performance Optimizations  
// =============================================================================

// BatchScanProjects scans multiple projects concurrently with controlled concurrency
func (s *ProjectLanguageScanner) BatchScanProjects(projectPaths []string) map[string]*MultiLanguageProjectInfo {
	results := make(map[string]*MultiLanguageProjectInfo)
	resultsMutex := sync.Mutex{}
	
	// Create semaphore for concurrency control
	semaphore := make(chan struct{}, s.MaxConcurrentScans)
	var wg sync.WaitGroup
	
	for _, path := range projectPaths {
		wg.Add(1)
		go func(projectPath string) {
			defer wg.Done()
			
			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			
			// Scan project
			info, err := s.ScanProjectCached(projectPath)
			if err == nil {
				resultsMutex.Lock()
				results[projectPath] = info
				resultsMutex.Unlock()
			}
		}(path)
	}
	
	wg.Wait()
	return results
}

// OptimizeForLargeMonorepos configures the scanner for optimal large monorepo performance
func (s *ProjectLanguageScanner) OptimizeForLargeMonorepos() {
	s.MaxDepth = 5 // Deeper scanning for monorepos
	s.MaxFiles = 50000 // Higher file limit
	s.EnableEarlyExit = true
	s.FastModeThreshold = 500 // Higher threshold for fast mode
	s.MaxConcurrentScans = 8 // More concurrent scans
	s.Timeout = 2 * time.Minute // Longer timeout
	
	// Enable caching if not already enabled
	if !s.EnableCache {
		s.SetCacheEnabled(true)
	}
}

// OptimizeForSmallProjects configures the scanner for optimal small project performance
func (s *ProjectLanguageScanner) OptimizeForSmallProjects() {
	s.MaxDepth = 3 // Standard depth
	s.MaxFiles = 1000 // Lower file limit
	s.EnableEarlyExit = false // No need for early exit
	s.FastModeThreshold = 50 // Lower threshold for fast mode
	s.MaxConcurrentScans = 4 // Fewer concurrent scans
	s.Timeout = 10 * time.Second // Shorter timeout
}

// GetOptimalConfiguration returns optimal configuration based on project characteristics
func (s *ProjectLanguageScanner) GetOptimalConfiguration(rootPath string) map[string]interface{} {
	estimatedSize := s.estimateProjectSize(rootPath)
	
	config := make(map[string]interface{})
	
	if estimatedSize > 1000 {
		// Large project configuration
		config["max_depth"] = 5
		config["max_files"] = 50000
		config["enable_early_exit"] = true
		config["fast_mode_threshold"] = 500
		config["max_concurrent_scans"] = 8
		config["timeout"] = "2m"
		config["recommendation"] = "large_monorepo"
	} else if estimatedSize > 100 {
		// Medium project configuration
		config["max_depth"] = 4
		config["max_files"] = 10000
		config["enable_early_exit"] = true
		config["fast_mode_threshold"] = 200
		config["max_concurrent_scans"] = 6
		config["timeout"] = "1m"
		config["recommendation"] = "medium_project"
	} else {
		// Small project configuration
		config["max_depth"] = 3
		config["max_files"] = 1000
		config["enable_early_exit"] = false
		config["fast_mode_threshold"] = 50
		config["max_concurrent_scans"] = 4
		config["timeout"] = "10s"
		config["recommendation"] = "small_project"
	}
	
	config["estimated_size"] = estimatedSize
	return config
}