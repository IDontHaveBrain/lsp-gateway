package indexing

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sourcegraph/scip/bindings/go/scip"
)

// DependencyGraph implements a comprehensive dependency analysis engine for impact assessment
type DependencyGraph struct {
	// Core graph structure
	nodes       map[string]*DependencyNode            // filePath -> node
	symbolIndex map[string]*SymbolDependency          // symbol -> dependency info
	edgeIndex   map[string]map[string]*DependencyEdge // from -> to -> edge

	// Specialized indices for fast lookups
	dependentIndex  map[string][]string // what depends on this file
	dependencyIndex map[string][]string // what this file depends on
	symbolToFiles   map[string][]string // symbol -> files that define/reference it

	// Configuration and components
	config         *DependencyGraphConfig
	symbolResolver *SymbolResolver
	scipStore      SCIPStore

	// Performance optimization
	cache       *DependencyCache
	lastUpdate  time.Time
	updateCount int64

	// Concurrency control
	mutex      sync.RWMutex
	indexMutex sync.RWMutex

	// Statistics and monitoring
	stats *DependencyGraphStats

	// Language-specific parsers
	languageParsers map[string]DependencyParser
}

// DependencyNode represents a file in the dependency graph
type DependencyNode struct {
	FilePath     string    `json:"file_path"`
	Language     string    `json:"language"`
	LastModified time.Time `json:"last_modified"`
	Checksum     string    `json:"checksum,omitempty"`

	// Direct dependencies
	Dependencies []*DependencyEdge `json:"dependencies"`
	Dependents   []*DependencyEdge `json:"dependents"`

	// Symbol information
	DefinedSymbols    []*SymbolDependency `json:"defined_symbols"`
	ReferencedSymbols []*SymbolDependency `json:"referenced_symbols"`

	// Cross-language dependencies
	CrossLanguageDeps []*CrossLanguageDependency `json:"cross_language_deps,omitempty"`

	// Metadata
	ProjectRoot string `json:"project_root,omitempty"`
	Package     string `json:"package,omitempty"`
	Module      string `json:"module,omitempty"`

	// Performance tracking
	AccessCount  int64     `json:"access_count"`
	LastAccessed time.Time `json:"last_accessed"`
}

// DependencyEdge represents a dependency relationship between files
type DependencyEdge struct {
	From           string         `json:"from"`
	To             string         `json:"to"`
	DependencyType DependencyType `json:"dependency_type"`
	Strength       float64        `json:"strength"`   // 0.0-1.0
	Confidence     float64        `json:"confidence"` // 0.0-1.0

	// Context information
	Symbols         []string `json:"symbols,omitempty"`
	LineNumbers     []int    `json:"line_numbers,omitempty"`
	ImportStatement string   `json:"import_statement,omitempty"`

	// Metadata
	CreatedAt   time.Time `json:"created_at"`
	LastUpdated time.Time `json:"last_updated"`
	Source      string    `json:"source"` // "import", "reference", "inheritance", etc.
}

// SymbolDependency represents symbol-level dependency information
type SymbolDependency struct {
	Symbol          string                      `json:"symbol"`
	Kind            scip.SymbolInformation_Kind `json:"kind"`
	DefinitionFile  string                      `json:"definition_file"`
	DefinitionRange *scip.Range                 `json:"definition_range,omitempty"`

	// References across files
	References []*SymbolReference `json:"references"`

	// Dependency relationship type
	RelationType DependencyType `json:"relation_type"`
	Confidence   float64        `json:"confidence"`

	// Impact analysis data
	ImpactRadius     int     `json:"impact_radius"`     // How many files could be affected
	ChangeFrequency  float64 `json:"change_frequency"`  // How often this symbol changes
	CriticalityScore float64 `json:"criticality_score"` // Combined impact metric
}

// CrossLanguageDependency represents dependencies between different languages
type CrossLanguageDependency struct {
	SourceFile     string `json:"source_file"`
	SourceLanguage string `json:"source_language"`
	TargetFile     string `json:"target_file"`
	TargetLanguage string `json:"target_language"`

	// Relationship details
	RelationType  string  `json:"relation_type"`  // "ffi", "config", "data", "script"
	InterfaceType string  `json:"interface_type"` // "api", "file", "network", "database"
	Confidence    float64 `json:"confidence"`

	// Context
	Description string   `json:"description,omitempty"`
	Examples    []string `json:"examples,omitempty"`
}

// DependencyType represents the type of dependency relationship
type DependencyType int

const (
	DependencyTypeImport DependencyType = iota
	DependencyTypeReference
	DependencyTypeInheritance
	DependencyTypeComposition
	DependencyTypeAnnotation
	DependencyTypeConfiguration
	DependencyTypeData
	DependencyTypeGenerated
	DependencyTypeBuild
	DependencyTypeTest
	DependencyTypeRuntime
)

// String returns string representation of DependencyType
func (dt DependencyType) String() string {
	switch dt {
	case DependencyTypeImport:
		return "import"
	case DependencyTypeReference:
		return "reference"
	case DependencyTypeInheritance:
		return "inheritance"
	case DependencyTypeComposition:
		return "composition"
	case DependencyTypeAnnotation:
		return "annotation"
	case DependencyTypeConfiguration:
		return "configuration"
	case DependencyTypeData:
		return "data"
	case DependencyTypeGenerated:
		return "generated"
	case DependencyTypeBuild:
		return "build"
	case DependencyTypeTest:
		return "test"
	case DependencyTypeRuntime:
		return "runtime"
	default:
		return "unknown"
	}
}

// DependencyGraphConfig contains configuration for the dependency graph
type DependencyGraphConfig struct {
	// Performance settings
	MaxGraphSize    int           `yaml:"max_graph_size" json:"max_graph_size"`
	CacheSize       int           `yaml:"cache_size" json:"cache_size"`
	CacheTTL        time.Duration `yaml:"cache_ttl" json:"cache_ttl"`
	UpdateBatchSize int           `yaml:"update_batch_size" json:"update_batch_size"`

	// Analysis settings
	MaxDependencyDepth   int  `yaml:"max_dependency_depth" json:"max_dependency_depth"`
	ImpactAnalysisDepth  int  `yaml:"impact_analysis_depth" json:"impact_analysis_depth"`
	CrossLanguageSupport bool `yaml:"cross_language_support" json:"cross_language_support"`
	SymbolLevelAnalysis  bool `yaml:"symbol_level_analysis" json:"symbol_level_analysis"`

	// Language-specific settings
	LanguageSettings map[string]*DependencyLanguageConfig `yaml:"language_settings,omitempty" json:"language_settings,omitempty"`

	// Performance thresholds
	MaxMemoryUsage   int64         `yaml:"max_memory_usage_mb" json:"max_memory_usage_mb"`
	MaxUpdateTime    time.Duration `yaml:"max_update_time" json:"max_update_time"`
	ParallelAnalysis bool          `yaml:"parallel_analysis" json:"parallel_analysis"`
	MaxConcurrency   int           `yaml:"max_concurrency" json:"max_concurrency"`

	// Impact analysis
	ImpactWeights       *ImpactWeights `yaml:"impact_weights,omitempty" json:"impact_weights,omitempty"`
	ConfidenceThreshold float64        `yaml:"confidence_threshold" json:"confidence_threshold"`
}

// DependencyLanguageConfig contains language-specific dependency analysis configuration
type DependencyLanguageConfig struct {
	Enabled              bool          `yaml:"enabled" json:"enabled"`
	ParserType           string        `yaml:"parser_type" json:"parser_type"`
	ImportPatterns       []string      `yaml:"import_patterns,omitempty" json:"import_patterns,omitempty"`
	ReferencePatterns    []string      `yaml:"reference_patterns,omitempty" json:"reference_patterns,omitempty"`
	CrossLanguageSupport bool          `yaml:"cross_language_support" json:"cross_language_support"`
	Priority             int           `yaml:"priority" json:"priority"`
	AnalysisTimeout      time.Duration `yaml:"analysis_timeout" json:"analysis_timeout"`
}

// ImpactWeights defines weights for different types of impact analysis
type ImpactWeights struct {
	DirectDependency   float64 `yaml:"direct_dependency" json:"direct_dependency"`
	IndirectDependency float64 `yaml:"indirect_dependency" json:"indirect_dependency"`
	SymbolReference    float64 `yaml:"symbol_reference" json:"symbol_reference"`
	InheritanceChain   float64 `yaml:"inheritance_chain" json:"inheritance_chain"`
	CrossLanguage      float64 `yaml:"cross_language" json:"cross_language"`
	TestDependency     float64 `yaml:"test_dependency" json:"test_dependency"`
	ConfigurationFile  float64 `yaml:"configuration_file" json:"configuration_file"`
}

// DependencyGraphStats tracks performance and usage statistics
type DependencyGraphStats struct {
	// Graph size metrics
	NodeCount          int64 `json:"node_count"`
	EdgeCount          int64 `json:"edge_count"`
	SymbolCount        int64 `json:"symbol_count"`
	CrossLanguageCount int64 `json:"cross_language_count"`

	// Performance metrics
	TotalAnalyses      int64         `json:"total_analyses"`
	SuccessfulAnalyses int64         `json:"successful_analyses"`
	FailedAnalyses     int64         `json:"failed_analyses"`
	AvgAnalysisTime    time.Duration `json:"avg_analysis_time"`

	// Cache metrics
	CacheHitRate float64 `json:"cache_hit_rate"`
	CacheSize    int     `json:"cache_size"`

	// Memory usage
	MemoryUsage int64     `json:"memory_usage_bytes"`
	LastUpdate  time.Time `json:"last_update"`

	// Error tracking
	ErrorCount    int64     `json:"error_count"`
	LastError     string    `json:"last_error,omitempty"`
	LastErrorTime time.Time `json:"last_error_time,omitempty"`

	mutex sync.RWMutex
}

// DependencyCache provides efficient caching for dependency analysis results
type DependencyCache struct {
	impactCache     map[string]*ImpactAnalysis
	dependencyCache map[string][]string
	maxSize         int
	ttl             time.Duration
	mutex           sync.RWMutex

	// LRU tracking
	accessOrder []string
	lastAccess  map[string]time.Time

	// Statistics
	hitCount  int64
	missCount int64
}

// ImpactAnalysis represents the result of impact analysis for file changes
type ImpactAnalysis struct {
	// Affected files
	DirectlyAffected   []string `json:"directly_affected"`
	IndirectlyAffected []string `json:"indirectly_affected"`
	TestFiles          []string `json:"test_files"`
	ConfigFiles        []string `json:"config_files"`

	// Priority and cost information
	UpdatePriority map[string]int `json:"update_priority"`
	EstimatedCost  time.Duration  `json:"estimated_cost"`
	RiskLevel      string         `json:"risk_level"`

	// Analysis metadata
	AnalysisDepth int           `json:"analysis_depth"`
	TotalFiles    int           `json:"total_files"`
	Confidence    float64       `json:"confidence"`
	AnalysisTime  time.Duration `json:"analysis_time"`

	// Detailed breakdowns
	SymbolImpacts    []*SymbolImpact    `json:"symbol_impacts,omitempty"`
	DependencyChains []*DependencyChain `json:"dependency_chains,omitempty"`
	Recommendations  []string           `json:"recommendations,omitempty"`
}

// SymbolImpact represents the impact of a symbol change
type SymbolImpact struct {
	Symbol        string   `json:"symbol"`
	ImpactType    string   `json:"impact_type"`
	AffectedFiles []string `json:"affected_files"`
	Severity      string   `json:"severity"`
	Confidence    float64  `json:"confidence"`
	Description   string   `json:"description"`
}

// DependencyChain represents a chain of dependencies
type DependencyChain struct {
	Chain     []string `json:"chain"`
	ChainType string   `json:"chain_type"`
	Strength  float64  `json:"strength"`
	RiskLevel string   `json:"risk_level"`
}

// FileChange represents a file change for impact analysis
type FileChange struct {
	FilePath   string    `json:"file_path"`
	ChangeType string    `json:"change_type"` // "create", "update", "delete", "rename"
	Language   string    `json:"language"`
	Timestamp  time.Time `json:"timestamp"`

	// Optional detailed change information
	ModifiedSymbols []string       `json:"modified_symbols,omitempty"`
	AddedSymbols    []string       `json:"added_symbols,omitempty"`
	RemovedSymbols  []string       `json:"removed_symbols,omitempty"`
	LineChanges     map[int]string `json:"line_changes,omitempty"`
}

// DependencyParser defines the interface for language-specific dependency parsing
type DependencyParser interface {
	// ParseDependencies extracts dependencies from a file
	ParseDependencies(filePath string, content []byte) ([]*DependencyEdge, error)

	// ParseSymbols extracts symbol definitions and references
	ParseSymbols(filePath string, content []byte) ([]*SymbolDependency, error)

	// DetectCrossLanguageDependencies identifies cross-language dependencies
	DetectCrossLanguageDependencies(filePath string, content []byte) ([]*CrossLanguageDependency, error)

	// GetSupportedExtensions returns file extensions this parser supports
	GetSupportedExtensions() []string

	// GetLanguage returns the language this parser handles
	GetLanguage() string
}

// ImpactAnalyzer provides comprehensive impact analysis capabilities
type ImpactAnalyzer struct {
	graph          *DependencyGraph
	config         *DependencyGraphConfig
	cache          *DependencyCache
	symbolResolver *SymbolResolver

	// Analysis state
	analysisDepth int
	visited       map[string]bool
	analysisQueue []string

	mutex sync.RWMutex
}

// NewDependencyGraph creates a new dependency graph with the specified configuration
func NewDependencyGraph(symbolResolver *SymbolResolver, scipStore SCIPStore, config *DependencyGraphConfig) (*DependencyGraph, error) {
	if symbolResolver == nil {
		return nil, fmt.Errorf("symbol resolver cannot be nil")
	}
	if scipStore == nil {
		return nil, fmt.Errorf("SCIP store cannot be nil")
	}
	if config == nil {
		config = DefaultDependencyGraphConfig()
	}

	graph := &DependencyGraph{
		nodes:           make(map[string]*DependencyNode),
		symbolIndex:     make(map[string]*SymbolDependency),
		edgeIndex:       make(map[string]map[string]*DependencyEdge),
		dependentIndex:  make(map[string][]string),
		dependencyIndex: make(map[string][]string),
		symbolToFiles:   make(map[string][]string),
		config:          config,
		symbolResolver:  symbolResolver,
		scipStore:       scipStore,
		cache:           NewDependencyCache(config.CacheSize, config.CacheTTL),
		lastUpdate:      time.Now(),
		stats:           NewDependencyGraphStats(),
		languageParsers: make(map[string]DependencyParser),
	}

	// Initialize language parsers
	if err := graph.initializeLanguageParsers(); err != nil {
		return nil, fmt.Errorf("failed to initialize language parsers: %w", err)
	}

	return graph, nil
}

// DefaultDependencyGraphConfig returns default configuration for dependency graph
func DefaultDependencyGraphConfig() *DependencyGraphConfig {
	return &DependencyGraphConfig{
		MaxGraphSize:         100000,
		CacheSize:            10000,
		CacheTTL:             30 * time.Minute,
		UpdateBatchSize:      50,
		MaxDependencyDepth:   10,
		ImpactAnalysisDepth:  5,
		CrossLanguageSupport: true,
		SymbolLevelAnalysis:  true,
		MaxMemoryUsage:       1024, // 1GB
		MaxUpdateTime:        5 * time.Minute,
		ParallelAnalysis:     true,
		MaxConcurrency:       10,
		ConfidenceThreshold:  0.7,
		ImpactWeights: &ImpactWeights{
			DirectDependency:   1.0,
			IndirectDependency: 0.5,
			SymbolReference:    0.8,
			InheritanceChain:   0.9,
			CrossLanguage:      0.6,
			TestDependency:     0.3,
			ConfigurationFile:  0.7,
		},
		LanguageSettings: map[string]*DependencyLanguageConfig{
			"go": {
				Enabled:              true,
				ParserType:           "ast",
				CrossLanguageSupport: true,
				Priority:             1,
				AnalysisTimeout:      30 * time.Second,
			},
			"python": {
				Enabled:              true,
				ParserType:           "ast",
				CrossLanguageSupport: true,
				Priority:             1,
				AnalysisTimeout:      30 * time.Second,
			},
			"typescript": {
				Enabled:              true,
				ParserType:           "ast",
				CrossLanguageSupport: true,
				Priority:             1,
				AnalysisTimeout:      30 * time.Second,
			},
			"javascript": {
				Enabled:              true,
				ParserType:           "regex",
				CrossLanguageSupport: false,
				Priority:             2,
				AnalysisTimeout:      20 * time.Second,
			},
		},
	}
}

// AddFile adds a file and its dependencies to the graph
func (dg *DependencyGraph) AddFile(filePath string, symbols []Symbol) error {
	dg.mutex.Lock()
	defer dg.mutex.Unlock()

	startTime := time.Now()
	defer func() {
		atomic.AddInt64(&dg.updateCount, 1)
		dg.lastUpdate = time.Now()
		dg.recordAnalysisTime(time.Since(startTime))
	}()

	// Normalize file path
	normalizedPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("failed to normalize file path %s: %w", filePath, err)
	}

	// Detect language
	language := dg.detectLanguage(normalizedPath)

	// Create or update node
	node := &DependencyNode{
		FilePath:          normalizedPath,
		Language:          language,
		LastModified:      time.Now(),
		Dependencies:      make([]*DependencyEdge, 0),
		Dependents:        make([]*DependencyEdge, 0),
		DefinedSymbols:    make([]*SymbolDependency, 0),
		ReferencedSymbols: make([]*SymbolDependency, 0),
		AccessCount:       0,
		LastAccessed:      time.Now(),
	}

	// Process symbols
	for _, symbol := range symbols {
		symDep := &SymbolDependency{
			Symbol:          symbol.Name,
			Kind:            symbol.Kind,
			DefinitionFile:  normalizedPath,
			DefinitionRange: symbol.Range,
			References:      make([]*SymbolReference, 0),
			RelationType:    DependencyTypeReference,
			Confidence:      0.95,
		}

		node.DefinedSymbols = append(node.DefinedSymbols, symDep)

		// Update symbol index
		dg.symbolIndex[symbol.Name] = symDep

		// Update symbol-to-files mapping
		if files, exists := dg.symbolToFiles[symbol.Name]; exists {
			// Add file if not already present
			found := false
			for _, file := range files {
				if file == normalizedPath {
					found = true
					break
				}
			}
			if !found {
				dg.symbolToFiles[symbol.Name] = append(files, normalizedPath)
			}
		} else {
			dg.symbolToFiles[symbol.Name] = []string{normalizedPath}
		}
	}

	// Parse file for dependencies if parser is available
	if parser, exists := dg.languageParsers[language]; exists {
		content, err := dg.readFileContent(normalizedPath)
		if err == nil {
			// Parse dependencies
			edges, err := parser.ParseDependencies(normalizedPath, content)
			if err == nil {
				node.Dependencies = edges

				// Update edge index
				if dg.edgeIndex[normalizedPath] == nil {
					dg.edgeIndex[normalizedPath] = make(map[string]*DependencyEdge)
				}

				for _, edge := range edges {
					dg.edgeIndex[normalizedPath][edge.To] = edge

					// Update reverse index
					if dg.dependencyIndex[normalizedPath] == nil {
						dg.dependencyIndex[normalizedPath] = make([]string, 0)
					}
					dg.dependencyIndex[normalizedPath] = append(dg.dependencyIndex[normalizedPath], edge.To)

					if dg.dependentIndex[edge.To] == nil {
						dg.dependentIndex[edge.To] = make([]string, 0)
					}
					dg.dependentIndex[edge.To] = append(dg.dependentIndex[edge.To], normalizedPath)
				}
			}

			// Parse symbols
			symbolDeps, err := parser.ParseSymbols(normalizedPath, content)
			if err == nil {
				node.ReferencedSymbols = symbolDeps
			}

			// Parse cross-language dependencies if enabled
			if dg.config.CrossLanguageSupport {
				crossDeps, err := parser.DetectCrossLanguageDependencies(normalizedPath, content)
				if err == nil {
					node.CrossLanguageDeps = crossDeps
				}
			}
		}
	}

	// Store node
	dg.nodes[normalizedPath] = node

	// Update statistics
	dg.stats.mutex.Lock()
	dg.stats.NodeCount++
	dg.stats.EdgeCount += int64(len(node.Dependencies))
	dg.stats.SymbolCount += int64(len(node.DefinedSymbols))
	dg.stats.CrossLanguageCount += int64(len(node.CrossLanguageDeps))
	dg.stats.SuccessfulAnalyses++
	dg.stats.LastUpdate = time.Now()
	dg.stats.mutex.Unlock()

	return nil
}

// RemoveFile removes a file from the dependency graph
func (dg *DependencyGraph) RemoveFile(filePath string) error {
	dg.mutex.Lock()
	defer dg.mutex.Unlock()

	normalizedPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("failed to normalize file path %s: %w", filePath, err)
	}

	node, exists := dg.nodes[normalizedPath]
	if !exists {
		return nil // File not in graph, nothing to do
	}

	// Remove from symbol index
	for _, symbol := range node.DefinedSymbols {
		delete(dg.symbolIndex, symbol.Symbol)

		// Update symbol-to-files mapping
		if files, exists := dg.symbolToFiles[symbol.Symbol]; exists {
			filteredFiles := make([]string, 0, len(files))
			for _, file := range files {
				if file != normalizedPath {
					filteredFiles = append(filteredFiles, file)
				}
			}
			if len(filteredFiles) > 0 {
				dg.symbolToFiles[symbol.Symbol] = filteredFiles
			} else {
				delete(dg.symbolToFiles, symbol.Symbol)
			}
		}
	}

	// Remove edges
	delete(dg.edgeIndex, normalizedPath)
	delete(dg.dependencyIndex, normalizedPath)
	delete(dg.dependentIndex, normalizedPath)

	// Remove references to this file from other files' dependent lists
	for _, dependency := range node.Dependencies {
		if dependents, exists := dg.dependentIndex[dependency.To]; exists {
			filteredDependents := make([]string, 0, len(dependents))
			for _, dependent := range dependents {
				if dependent != normalizedPath {
					filteredDependents = append(filteredDependents, dependent)
				}
			}
			dg.dependentIndex[dependency.To] = filteredDependents
		}
	}

	// Remove the node
	delete(dg.nodes, normalizedPath)

	// Update statistics
	dg.stats.mutex.Lock()
	dg.stats.NodeCount--
	dg.stats.EdgeCount -= int64(len(node.Dependencies))
	dg.stats.SymbolCount -= int64(len(node.DefinedSymbols))
	dg.stats.CrossLanguageCount -= int64(len(node.CrossLanguageDeps))
	dg.stats.mutex.Unlock()

	// Clear cache entries related to this file
	dg.cache.InvalidateFile(normalizedPath)

	return nil
}

// UpdateFile updates a file's dependencies in the graph
func (dg *DependencyGraph) UpdateFile(filePath string, symbols []Symbol) error {
	// Remove existing file first
	if err := dg.RemoveFile(filePath); err != nil {
		return fmt.Errorf("failed to remove existing file: %w", err)
	}

	// Add updated file
	return dg.AddFile(filePath, symbols)
}

// GetDependents returns files that depend on the specified file
func (dg *DependencyGraph) GetDependents(filePath string) ([]string, error) {
	dg.mutex.RLock()
	defer dg.mutex.RUnlock()

	normalizedPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to normalize file path %s: %w", filePath, err)
	}

	// Check cache first
	cacheKey := fmt.Sprintf("dependents:%s", normalizedPath)
	if cached := dg.cache.GetDependencies(cacheKey); cached != nil {
		return cached, nil
	}

	dependents := dg.dependentIndex[normalizedPath]
	if dependents == nil {
		dependents = make([]string, 0)
	}

	// Make a copy to prevent external modification
	result := make([]string, len(dependents))
	copy(result, dependents)

	// Cache the result
	dg.cache.SetDependencies(cacheKey, result)

	return result, nil
}

// GetDependencies returns files that the specified file depends on
func (dg *DependencyGraph) GetDependencies(filePath string) ([]string, error) {
	dg.mutex.RLock()
	defer dg.mutex.RUnlock()

	normalizedPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to normalize file path %s: %w", filePath, err)
	}

	// Check cache first
	cacheKey := fmt.Sprintf("dependencies:%s", normalizedPath)
	if cached := dg.cache.GetDependencies(cacheKey); cached != nil {
		return cached, nil
	}

	dependencies := dg.dependencyIndex[normalizedPath]
	if dependencies == nil {
		dependencies = make([]string, 0)
	}

	// Make a copy to prevent external modification
	result := make([]string, len(dependencies))
	copy(result, dependencies)

	// Cache the result
	dg.cache.SetDependencies(cacheKey, result)

	return result, nil
}

// AnalyzeImpact performs comprehensive impact analysis for file changes
func (dg *DependencyGraph) AnalyzeImpact(changes []FileChange) (*ImpactAnalysis, error) {
	startTime := time.Now()

	// Create impact analyzer
	analyzer := &ImpactAnalyzer{
		graph:          dg,
		config:         dg.config,
		cache:          dg.cache,
		symbolResolver: dg.symbolResolver,
		analysisDepth:  0,
		visited:        make(map[string]bool),
		analysisQueue:  make([]string, 0),
	}

	// Generate cache key for this analysis
	cacheKey := analyzer.generateCacheKey(changes)

	// Check cache first
	if cached := dg.cache.GetImpactAnalysis(cacheKey); cached != nil {
		return cached, nil
	}

	// Perform impact analysis
	analysis, err := analyzer.analyzeImpact(changes)
	if err != nil {
		dg.recordError(fmt.Sprintf("Impact analysis failed: %v", err))
		return nil, err
	}

	// Set analysis metadata
	analysis.AnalysisTime = time.Since(startTime)
	analysis.TotalFiles = len(dg.nodes)

	// Cache the result
	dg.cache.SetImpactAnalysis(cacheKey, analysis)

	// Update statistics
	dg.recordAnalysisTime(time.Since(startTime))

	return analysis, nil
}

// GetStats returns comprehensive dependency graph statistics
func (dg *DependencyGraph) GetStats() *DependencyGraphStats {
	dg.stats.mutex.RLock()
	defer dg.stats.mutex.RUnlock()

	// Create a copy to avoid concurrent modification
	stats := *dg.stats

	// Update derived metrics
	stats.MemoryUsage = dg.estimateMemoryUsage()
	stats.CacheHitRate = dg.cache.GetHitRate()
	stats.CacheSize = dg.cache.Size()

	return &stats
}

// Symbol represents a symbol extracted from code analysis
type Symbol struct {
	Name  string
	Kind  scip.SymbolInformation_Kind
	Range *scip.Range
}

// Helper methods and interfaces

// detectLanguage detects the programming language based on file extension
func (dg *DependencyGraph) detectLanguage(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))

	languageMap := map[string]string{
		".go":   "go",
		".py":   "python",
		".pyi":  "python",
		".ts":   "typescript",
		".tsx":  "typescript",
		".js":   "javascript",
		".jsx":  "javascript",
		".java": "java",
		".kt":   "kotlin",
		".rs":   "rust",
		".cpp":  "cpp",
		".cc":   "cpp",
		".cxx":  "cpp",
		".c":    "c",
		".h":    "c",
		".hpp":  "cpp",
		".cs":   "csharp",
	}

	if language, exists := languageMap[ext]; exists {
		return language
	}

	return "unknown"
}

// readFileContent reads the content of a file (placeholder implementation)
func (dg *DependencyGraph) readFileContent(filePath string) ([]byte, error) {
	// This would be implemented to read file content
	// For now, return empty content to avoid file system access in this example
	return []byte{}, nil
}

// initializeLanguageParsers initializes language-specific dependency parsers
func (dg *DependencyGraph) initializeLanguageParsers() error {
	// Initialize parsers for supported languages
	// This would register actual parser implementations
	// For now, using placeholder parsers

	for language, config := range dg.config.LanguageSettings {
		if config.Enabled {
			// Create parser based on language
			parser, err := dg.createLanguageParser(language, config)
			if err != nil {
				return fmt.Errorf("failed to create parser for %s: %w", language, err)
			}
			dg.languageParsers[language] = parser
		}
	}

	return nil
}

// createLanguageParser creates a language-specific parser
func (dg *DependencyGraph) createLanguageParser(language string, config *DependencyLanguageConfig) (DependencyParser, error) {
	// Return a placeholder parser for now
	return &PlaceholderParser{language: language}, nil
}

// estimateMemoryUsage estimates the memory usage of the dependency graph
func (dg *DependencyGraph) estimateMemoryUsage() int64 {
	var total int64

	// Estimate node memory usage
	total += int64(len(dg.nodes) * 1024) // Rough estimate per node

	// Estimate edge memory usage
	total += int64(len(dg.edgeIndex) * 512) // Rough estimate per edge map

	// Estimate symbol index usage
	total += int64(len(dg.symbolIndex) * 256) // Rough estimate per symbol

	// Add base overhead
	total += 1024 * 1024 // 1MB base overhead

	return total
}

// recordAnalysisTime records the time taken for an analysis
func (dg *DependencyGraph) recordAnalysisTime(duration time.Duration) {
	dg.stats.mutex.Lock()
	defer dg.stats.mutex.Unlock()

	dg.stats.TotalAnalyses++

	if dg.stats.AvgAnalysisTime == 0 {
		dg.stats.AvgAnalysisTime = duration
	} else {
		// Exponential moving average
		alpha := 0.1
		dg.stats.AvgAnalysisTime = time.Duration(float64(dg.stats.AvgAnalysisTime)*(1-alpha) + float64(duration)*alpha)
	}
}

// recordError records an error in the statistics
func (dg *DependencyGraph) recordError(errorMsg string) {
	dg.stats.mutex.Lock()
	defer dg.stats.mutex.Unlock()

	dg.stats.ErrorCount++
	dg.stats.FailedAnalyses++
	dg.stats.LastError = errorMsg
	dg.stats.LastErrorTime = time.Now()
}

// PlaceholderParser is a placeholder implementation of DependencyParser
type PlaceholderParser struct {
	language string
}

func (p *PlaceholderParser) ParseDependencies(filePath string, content []byte) ([]*DependencyEdge, error) {
	return []*DependencyEdge{}, nil
}

func (p *PlaceholderParser) ParseSymbols(filePath string, content []byte) ([]*SymbolDependency, error) {
	return []*SymbolDependency{}, nil
}

func (p *PlaceholderParser) DetectCrossLanguageDependencies(filePath string, content []byte) ([]*CrossLanguageDependency, error) {
	return []*CrossLanguageDependency{}, nil
}

func (p *PlaceholderParser) GetSupportedExtensions() []string {
	return []string{}
}

func (p *PlaceholderParser) GetLanguage() string {
	return p.language
}

// NewDependencyCache creates a new dependency cache
func NewDependencyCache(maxSize int, ttl time.Duration) *DependencyCache {
	return &DependencyCache{
		impactCache:     make(map[string]*ImpactAnalysis),
		dependencyCache: make(map[string][]string),
		maxSize:         maxSize,
		ttl:             ttl,
		accessOrder:     make([]string, 0),
		lastAccess:      make(map[string]time.Time),
	}
}

// GetImpactAnalysis retrieves cached impact analysis
func (dc *DependencyCache) GetImpactAnalysis(key string) *ImpactAnalysis {
	dc.mutex.RLock()
	defer dc.mutex.RUnlock()

	if analysis, exists := dc.impactCache[key]; exists {
		// Check TTL
		if lastAccess, ok := dc.lastAccess[key]; ok {
			if time.Since(lastAccess) <= dc.ttl {
				dc.updateAccess(key)
				atomic.AddInt64(&dc.hitCount, 1)
				return analysis
			}
		}
		// Expired, remove
		delete(dc.impactCache, key)
		delete(dc.lastAccess, key)
	}

	atomic.AddInt64(&dc.missCount, 1)
	return nil
}

// SetImpactAnalysis stores impact analysis in cache
func (dc *DependencyCache) SetImpactAnalysis(key string, analysis *ImpactAnalysis) {
	dc.mutex.Lock()
	defer dc.mutex.Unlock()

	// Check if we need to evict
	if len(dc.impactCache) >= dc.maxSize {
		dc.evictLRU()
	}

	dc.impactCache[key] = analysis
	dc.lastAccess[key] = time.Now()
	dc.updateAccessOrder(key)
}

// GetDependencies retrieves cached dependencies
func (dc *DependencyCache) GetDependencies(key string) []string {
	dc.mutex.RLock()
	defer dc.mutex.RUnlock()

	if deps, exists := dc.dependencyCache[key]; exists {
		// Check TTL
		if lastAccess, ok := dc.lastAccess[key]; ok {
			if time.Since(lastAccess) <= dc.ttl {
				dc.updateAccess(key)
				atomic.AddInt64(&dc.hitCount, 1)

				// Return copy to prevent external modification
				result := make([]string, len(deps))
				copy(result, deps)
				return result
			}
		}
		// Expired, remove
		delete(dc.dependencyCache, key)
		delete(dc.lastAccess, key)
	}

	atomic.AddInt64(&dc.missCount, 1)
	return nil
}

// SetDependencies stores dependencies in cache
func (dc *DependencyCache) SetDependencies(key string, deps []string) {
	dc.mutex.Lock()
	defer dc.mutex.Unlock()

	// Check if we need to evict
	if len(dc.dependencyCache) >= dc.maxSize {
		dc.evictLRU()
	}

	// Store copy to prevent external modification
	depsCopy := make([]string, len(deps))
	copy(depsCopy, deps)

	dc.dependencyCache[key] = depsCopy
	dc.lastAccess[key] = time.Now()
	dc.updateAccessOrder(key)
}

// InvalidateFile removes cache entries related to a file
func (dc *DependencyCache) InvalidateFile(filePath string) {
	dc.mutex.Lock()
	defer dc.mutex.Unlock()

	// Remove entries that contain the file path
	for key := range dc.impactCache {
		if strings.Contains(key, filePath) {
			delete(dc.impactCache, key)
			delete(dc.lastAccess, key)
		}
	}

	for key := range dc.dependencyCache {
		if strings.Contains(key, filePath) {
			delete(dc.dependencyCache, key)
			delete(dc.lastAccess, key)
		}
	}

	// Clean up access order
	dc.cleanupAccessOrder()
}

// GetHitRate returns the cache hit rate
func (dc *DependencyCache) GetHitRate() float64 {
	hits := float64(atomic.LoadInt64(&dc.hitCount))
	misses := float64(atomic.LoadInt64(&dc.missCount))
	total := hits + misses

	if total == 0 {
		return 0
	}

	return hits / total
}

// Size returns the current cache size
func (dc *DependencyCache) Size() int {
	dc.mutex.RLock()
	defer dc.mutex.RUnlock()

	return len(dc.impactCache) + len(dc.dependencyCache)
}

// updateAccess updates the access time for a cache entry
func (dc *DependencyCache) updateAccess(key string) {
	dc.lastAccess[key] = time.Now()
	dc.updateAccessOrder(key)
}

// updateAccessOrder updates the LRU access order
func (dc *DependencyCache) updateAccessOrder(key string) {
	// Remove from current position
	for i, k := range dc.accessOrder {
		if k == key {
			dc.accessOrder = append(dc.accessOrder[:i], dc.accessOrder[i+1:]...)
			break
		}
	}

	// Add to front
	dc.accessOrder = append([]string{key}, dc.accessOrder...)
}

// evictLRU evicts the least recently used entry
func (dc *DependencyCache) evictLRU() {
	if len(dc.accessOrder) > 0 {
		// Get least recently used key
		lruKey := dc.accessOrder[len(dc.accessOrder)-1]

		// Remove from all caches
		delete(dc.impactCache, lruKey)
		delete(dc.dependencyCache, lruKey)
		delete(dc.lastAccess, lruKey)

		// Remove from access order
		dc.accessOrder = dc.accessOrder[:len(dc.accessOrder)-1]
	}
}

// cleanupAccessOrder removes invalid keys from access order
func (dc *DependencyCache) cleanupAccessOrder() {
	validKeys := make([]string, 0, len(dc.accessOrder))

	for _, key := range dc.accessOrder {
		if _, impactExists := dc.impactCache[key]; impactExists {
			validKeys = append(validKeys, key)
		} else if _, depExists := dc.dependencyCache[key]; depExists {
			validKeys = append(validKeys, key)
		}
	}

	dc.accessOrder = validKeys
}

// ImpactAnalyzer methods

// analyzeImpact performs the core impact analysis logic
func (ia *ImpactAnalyzer) analyzeImpact(changes []FileChange) (*ImpactAnalysis, error) {
	analysis := &ImpactAnalysis{
		DirectlyAffected:   make([]string, 0),
		IndirectlyAffected: make([]string, 0),
		TestFiles:          make([]string, 0),
		ConfigFiles:        make([]string, 0),
		UpdatePriority:     make(map[string]int),
		SymbolImpacts:      make([]*SymbolImpact, 0),
		DependencyChains:   make([]*DependencyChain, 0),
		Recommendations:    make([]string, 0),
		Confidence:         1.0,
	}

	// Reset analysis state
	ia.visited = make(map[string]bool)
	ia.analysisQueue = make([]string, 0)
	ia.analysisDepth = 0

	// Process each changed file
	for _, change := range changes {
		if err := ia.processFileChange(change, analysis); err != nil {
			return nil, fmt.Errorf("failed to process file change %s: %w", change.FilePath, err)
		}
	}

	// Perform depth-limited traversal to find indirect impacts
	if err := ia.findIndirectImpacts(analysis); err != nil {
		return nil, fmt.Errorf("failed to find indirect impacts: %w", err)
	}

	// Calculate update priorities
	ia.calculateUpdatePriorities(analysis)

	// Estimate cost and risk level
	ia.estimateCostAndRisk(analysis)

	// Generate recommendations
	ia.generateRecommendations(analysis)

	// Sort results
	ia.sortResults(analysis)

	return analysis, nil
}

// processFileChange processes a single file change
func (ia *ImpactAnalyzer) processFileChange(change FileChange, analysis *ImpactAnalysis) error {
	normalizedPath, err := filepath.Abs(change.FilePath)
	if err != nil {
		return fmt.Errorf("failed to normalize path: %w", err)
	}

	// Mark as directly affected
	analysis.DirectlyAffected = append(analysis.DirectlyAffected, normalizedPath)
	ia.visited[normalizedPath] = true

	// Get direct dependents
	dependents, err := ia.graph.GetDependents(normalizedPath)
	if err != nil {
		return fmt.Errorf("failed to get dependents: %w", err)
	}

	// Add direct dependents to queue for further processing
	for _, dependent := range dependents {
		if !ia.visited[dependent] {
			ia.analysisQueue = append(ia.analysisQueue, dependent)

			// Categorize files
			if ia.isTestFile(dependent) {
				analysis.TestFiles = append(analysis.TestFiles, dependent)
			} else if ia.isConfigFile(dependent) {
				analysis.ConfigFiles = append(analysis.ConfigFiles, dependent)
			}
		}
	}

	// Analyze symbol-level impacts if enabled
	if ia.config.SymbolLevelAnalysis {
		symbolImpacts := ia.analyzeSymbolImpacts(change)
		analysis.SymbolImpacts = append(analysis.SymbolImpacts, symbolImpacts...)
	}

	return nil
}

// findIndirectImpacts performs breadth-first traversal to find indirect impacts
func (ia *ImpactAnalyzer) findIndirectImpacts(analysis *ImpactAnalysis) error {
	for len(ia.analysisQueue) > 0 && ia.analysisDepth < ia.config.ImpactAnalysisDepth {
		currentQueue := ia.analysisQueue
		ia.analysisQueue = make([]string, 0)
		ia.analysisDepth++

		for _, filePath := range currentQueue {
			if ia.visited[filePath] {
				continue
			}

			ia.visited[filePath] = true
			analysis.IndirectlyAffected = append(analysis.IndirectlyAffected, filePath)

			// Get dependents of this file
			dependents, err := ia.graph.GetDependents(filePath)
			if err != nil {
				continue // Skip files with errors
			}

			// Add unvisited dependents to next level
			for _, dependent := range dependents {
				if !ia.visited[dependent] {
					ia.analysisQueue = append(ia.analysisQueue, dependent)

					// Categorize files
					if ia.isTestFile(dependent) {
						analysis.TestFiles = append(analysis.TestFiles, dependent)
					} else if ia.isConfigFile(dependent) {
						analysis.ConfigFiles = append(analysis.ConfigFiles, dependent)
					}
				}
			}

			// Create dependency chain
			chain := &DependencyChain{
				Chain:     []string{filePath},
				ChainType: "indirect",
				Strength:  1.0 / float64(ia.analysisDepth),
				RiskLevel: ia.calculateRiskLevel(ia.analysisDepth),
			}
			analysis.DependencyChains = append(analysis.DependencyChains, chain)
		}
	}

	analysis.AnalysisDepth = ia.analysisDepth
	return nil
}

// analyzeSymbolImpacts analyzes symbol-level impacts
func (ia *ImpactAnalyzer) analyzeSymbolImpacts(change FileChange) []*SymbolImpact {
	impacts := make([]*SymbolImpact, 0)

	// Process modified symbols
	for _, symbol := range change.ModifiedSymbols {
		if symbolDep, exists := ia.graph.symbolIndex[symbol]; exists {
			impact := &SymbolImpact{
				Symbol:        symbol,
				ImpactType:    "modification",
				AffectedFiles: ia.getSymbolAffectedFiles(symbol),
				Severity:      ia.calculateSymbolSeverity(symbolDep),
				Confidence:    symbolDep.Confidence,
				Description:   fmt.Sprintf("Symbol %s was modified", symbol),
			}
			impacts = append(impacts, impact)
		}
	}

	// Process added symbols
	for _, symbol := range change.AddedSymbols {
		impact := &SymbolImpact{
			Symbol:        symbol,
			ImpactType:    "addition",
			AffectedFiles: []string{change.FilePath},
			Severity:      "low",
			Confidence:    0.9,
			Description:   fmt.Sprintf("Symbol %s was added", symbol),
		}
		impacts = append(impacts, impact)
	}

	// Process removed symbols
	for _, symbol := range change.RemovedSymbols {
		if symbolDep, exists := ia.graph.symbolIndex[symbol]; exists {
			impact := &SymbolImpact{
				Symbol:        symbol,
				ImpactType:    "removal",
				AffectedFiles: ia.getSymbolAffectedFiles(symbol),
				Severity:      "high", // Removals are typically high impact
				Confidence:    symbolDep.Confidence,
				Description:   fmt.Sprintf("Symbol %s was removed", symbol),
			}
			impacts = append(impacts, impact)
		}
	}

	return impacts
}

// calculateUpdatePriorities calculates update priorities for affected files
func (ia *ImpactAnalyzer) calculateUpdatePriorities(analysis *ImpactAnalysis) {
	// Direct impacts have highest priority
	for _, filePath := range analysis.DirectlyAffected {
		analysis.UpdatePriority[filePath] = 1
	}

	// Indirect impacts have decreasing priority based on depth
	priority := 2
	for _, filePath := range analysis.IndirectlyAffected {
		analysis.UpdatePriority[filePath] = priority
		priority++
	}

	// Test files have lower priority
	for _, filePath := range analysis.TestFiles {
		if currentPriority, exists := analysis.UpdatePriority[filePath]; exists {
			analysis.UpdatePriority[filePath] = currentPriority + 10
		} else {
			analysis.UpdatePriority[filePath] = 15
		}
	}

	// Config files have medium priority
	for _, filePath := range analysis.ConfigFiles {
		if currentPriority, exists := analysis.UpdatePriority[filePath]; exists {
			analysis.UpdatePriority[filePath] = currentPriority + 5
		} else {
			analysis.UpdatePriority[filePath] = 10
		}
	}
}

// estimateCostAndRisk estimates the cost and risk level of the changes
func (ia *ImpactAnalyzer) estimateCostAndRisk(analysis *ImpactAnalysis) {
	totalFiles := len(analysis.DirectlyAffected) + len(analysis.IndirectlyAffected)

	// Estimate cost based on number of affected files and complexity
	baseCost := time.Duration(totalFiles*30) * time.Second // 30 seconds per file
	complexityMultiplier := 1.0

	if len(analysis.SymbolImpacts) > 10 {
		complexityMultiplier += 0.5
	}
	if len(analysis.DependencyChains) > 20 {
		complexityMultiplier += 0.3
	}
	if len(analysis.TestFiles) > 0 {
		complexityMultiplier += 0.2
	}

	analysis.EstimatedCost = time.Duration(float64(baseCost) * complexityMultiplier)

	// Determine risk level
	if totalFiles > 50 || analysis.AnalysisDepth > 3 {
		analysis.RiskLevel = "high"
	} else if totalFiles > 20 || analysis.AnalysisDepth > 2 {
		analysis.RiskLevel = "medium"
	} else {
		analysis.RiskLevel = "low"
	}

	// Adjust confidence based on analysis completeness
	if analysis.AnalysisDepth >= ia.config.ImpactAnalysisDepth {
		analysis.Confidence *= 0.8 // Reduced confidence if we hit depth limit
	}
}

// generateRecommendations generates recommendations for handling the impact
func (ia *ImpactAnalyzer) generateRecommendations(analysis *ImpactAnalysis) {
	totalFiles := len(analysis.DirectlyAffected) + len(analysis.IndirectlyAffected)

	// Basic recommendations
	if totalFiles > 30 {
		analysis.Recommendations = append(analysis.Recommendations,
			"Consider batching updates due to high impact scope")
	}

	if len(analysis.TestFiles) > 0 {
		analysis.Recommendations = append(analysis.Recommendations,
			"Run affected test suites to verify changes")
	}

	if len(analysis.ConfigFiles) > 0 {
		analysis.Recommendations = append(analysis.Recommendations,
			"Review configuration file changes carefully")
	}

	if analysis.RiskLevel == "high" {
		analysis.Recommendations = append(analysis.Recommendations,
			"Consider phased rollout due to high risk level")
	}

	// Symbol-specific recommendations
	for _, symbolImpact := range analysis.SymbolImpacts {
		if symbolImpact.Severity == "high" {
			analysis.Recommendations = append(analysis.Recommendations,
				fmt.Sprintf("Review high-impact symbol change: %s", symbolImpact.Symbol))
		}
	}
}

// sortResults sorts the analysis results for better presentation
func (ia *ImpactAnalyzer) sortResults(analysis *ImpactAnalysis) {
	// Sort affected files alphabetically
	sort.Strings(analysis.DirectlyAffected)
	sort.Strings(analysis.IndirectlyAffected)
	sort.Strings(analysis.TestFiles)
	sort.Strings(analysis.ConfigFiles)

	// Sort symbol impacts by severity
	sort.Slice(analysis.SymbolImpacts, func(i, j int) bool {
		severityOrder := map[string]int{"high": 0, "medium": 1, "low": 2}
		return severityOrder[analysis.SymbolImpacts[i].Severity] < severityOrder[analysis.SymbolImpacts[j].Severity]
	})

	// Sort dependency chains by strength
	sort.Slice(analysis.DependencyChains, func(i, j int) bool {
		return analysis.DependencyChains[i].Strength > analysis.DependencyChains[j].Strength
	})
}

// generateCacheKey generates a cache key for impact analysis
func (ia *ImpactAnalyzer) generateCacheKey(changes []FileChange) string {
	var keyParts []string

	for _, change := range changes {
		keyParts = append(keyParts, fmt.Sprintf("%s:%s:%d",
			change.FilePath, change.ChangeType, change.Timestamp.Unix()))
	}

	return fmt.Sprintf("impact:%s", strings.Join(keyParts, "|"))
}

// Helper methods

// isTestFile determines if a file is a test file
func (ia *ImpactAnalyzer) isTestFile(filePath string) bool {
	fileName := strings.ToLower(filepath.Base(filePath))
	return strings.Contains(fileName, "test") ||
		strings.Contains(fileName, "spec") ||
		strings.HasSuffix(fileName, "_test.go") ||
		strings.HasSuffix(fileName, "_test.py") ||
		strings.HasSuffix(fileName, ".test.js") ||
		strings.HasSuffix(fileName, ".spec.js")
}

// isConfigFile determines if a file is a configuration file
func (ia *ImpactAnalyzer) isConfigFile(filePath string) bool {
	fileName := strings.ToLower(filepath.Base(filePath))
	configNames := []string{"config", "setting", "env", "yaml", "json", "toml", "ini"}

	for _, configName := range configNames {
		if strings.Contains(fileName, configName) {
			return true
		}
	}

	return false
}

// getSymbolAffectedFiles gets files affected by a symbol change
func (ia *ImpactAnalyzer) getSymbolAffectedFiles(symbol string) []string {
	if files, exists := ia.graph.symbolToFiles[symbol]; exists {
		return files
	}
	return []string{}
}

// calculateSymbolSeverity calculates the severity of a symbol impact
func (ia *ImpactAnalyzer) calculateSymbolSeverity(symbolDep *SymbolDependency) string {
	refCount := len(symbolDep.References)

	if refCount > 20 {
		return "high"
	} else if refCount > 5 {
		return "medium"
	}
	return "low"
}

// calculateRiskLevel calculates risk level based on dependency depth
func (ia *ImpactAnalyzer) calculateRiskLevel(depth int) string {
	if depth > 3 {
		return "high"
	} else if depth > 1 {
		return "medium"
	}
	return "low"
}

// NewDependencyGraphStats creates new dependency graph statistics
func NewDependencyGraphStats() *DependencyGraphStats {
	return &DependencyGraphStats{
		LastUpdate: time.Now(),
	}
}
