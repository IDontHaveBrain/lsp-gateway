package e2e_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"lsp-gateway/tests/e2e/mcp/types"
)

// EnhancedLSPServerConfig provides comprehensive LSP server configuration
type EnhancedLSPServerConfig struct {
	ServerName     string
	Language       string
	Command        string
	Args           []string
	Transport      string
	RootMarkers    []string
	WorkspaceRoot  string

	// gopls-specific configuration
	GoplsSettings  map[string]interface{}
	BuildFlags     []string
	Environment    map[string]string

	// Performance and timeout settings
	InitTimeout     time.Duration
	ResponseTimeout time.Duration
	MemoryLimit     int64

	// Test-specific optimizations
	TestMode        bool
	DisableFeatures []string
	EnableFeatures  []string
}

// LSPConfigGenerator generates optimal LSP configurations for test environments
type LSPConfigGenerator struct {
	templateConfig *EnhancedLSPServerConfig
	logger         *log.Logger
}

// NewLSPConfigGenerator creates a new LSP configuration generator
func NewLSPConfigGenerator() *LSPConfigGenerator {
	return &LSPConfigGenerator{
		templateConfig: createOptimalGoplsTemplate(),
		logger:         log.New(os.Stdout, "[LSPConfigGenerator] ", log.LstdFlags),
	}
}

// GenerateConfigForWorkspace creates an optimized LSP configuration for a test workspace
func (gen *LSPConfigGenerator) GenerateConfigForWorkspace(workspace *types.TestWorkspace) (*EnhancedLSPServerConfig, error) {
	if workspace == nil {
		return nil, fmt.Errorf("workspace cannot be nil")
	}

	gen.logger.Printf("Generating LSP configuration for workspace %s at %s", workspace.ID, workspace.RootPath)

	// Start with the optimized template
	config := gen.copyTemplate()

	// Customize for specific workspace
	config.WorkspaceRoot = workspace.RootPath
	config.ServerName = fmt.Sprintf("gopls-%s", workspace.ID)

	// Set workspace-specific environment
	config.Environment["GOCACHE"] = filepath.Join(workspace.RootPath, ".gocache")
	config.Environment["GOMODCACHE"] = filepath.Join(workspace.RootPath, ".gomodcache")

	// Validate gopls binary availability
	if err := gen.validateGoplsBinary(config.Command); err != nil {
		return nil, fmt.Errorf("gopls validation failed: %w", err)
	}

	// Verify workspace structure compatibility
	if err := gen.validateWorkspaceStructure(workspace.RootPath); err != nil {
		return nil, fmt.Errorf("workspace structure validation failed: %w", err)
	}

	// Apply workspace-specific optimizations
	if err := gen.optimizeForWorkspace(config, workspace); err != nil {
		return nil, fmt.Errorf("workspace optimization failed: %w", err)
	}

	gen.logger.Printf("Generated optimized LSP configuration for workspace %s", workspace.ID)
	return config, nil
}

// ValidateLSPConfig validates an LSP configuration for correctness
func (gen *LSPConfigGenerator) ValidateLSPConfig(config *EnhancedLSPServerConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	// Validate required fields
	if config.Command == "" {
		return fmt.Errorf("command cannot be empty")
	}
	if config.WorkspaceRoot == "" {
		return fmt.Errorf("workspace root cannot be empty")
	}
	if config.Language == "" {
		return fmt.Errorf("language cannot be empty")
	}

	// Validate workspace root exists
	if _, err := os.Stat(config.WorkspaceRoot); err != nil {
		return fmt.Errorf("workspace root does not exist: %w", err)
	}

	// Validate gopls binary
	if err := gen.validateGoplsBinary(config.Command); err != nil {
		return fmt.Errorf("gopls binary validation failed: %w", err)
	}

	// Validate Go module structure
	goModPath := filepath.Join(config.WorkspaceRoot, "go.mod")
	if _, err := os.Stat(goModPath); err != nil {
		return fmt.Errorf("go.mod not found in workspace root: %w", err)
	}

	// Validate timeout settings
	if config.InitTimeout <= 0 {
		return fmt.Errorf("init timeout must be positive")
	}
	if config.ResponseTimeout <= 0 {
		return fmt.Errorf("response timeout must be positive")
	}

	// Validate memory limit
	if config.MemoryLimit <= 0 {
		return fmt.Errorf("memory limit must be positive")
	}

	return nil
}

// OptimizeForTesting applies test-specific optimizations to an LSP configuration
func (gen *LSPConfigGenerator) OptimizeForTesting(config *EnhancedLSPServerConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	gen.logger.Printf("Applying test-specific optimizations to LSP configuration")

	// Enable test mode
	config.TestMode = true

	// Apply aggressive performance optimizations
	gen.applyTestPerformanceOptimizations(config)

	// Disable heavy features for testing
	gen.disableHeavyFeaturesForTesting(config)

	// Optimize memory usage
	gen.optimizeMemoryForTesting(config)

	// Set aggressive timeouts for fast test execution  
	config.InitTimeout = 10 * time.Second
	config.ResponseTimeout = 5 * time.Second

	gen.logger.Printf("Applied test-specific optimizations successfully")
	return nil
}

// UpdateWorkspaceLSPConfig updates an existing workspace's LSP configuration
func (gen *LSPConfigGenerator) UpdateWorkspaceLSPConfig(workspace *types.TestWorkspace) error {
	if workspace == nil {
		return fmt.Errorf("workspace cannot be nil")
	}

	enhancedConfig, err := gen.GenerateConfigForWorkspace(workspace)
	if err != nil {
		return fmt.Errorf("failed to generate enhanced config: %w", err)
	}

	// Convert enhanced config to existing LSPServerConfig structure
	workspace.LSPConfig = gen.convertToBasicConfig(enhancedConfig)

	gen.logger.Printf("Updated LSP configuration for workspace %s", workspace.ID)
	return nil
}

// Private helper methods

func createOptimalGoplsTemplate() *EnhancedLSPServerConfig {
	return &EnhancedLSPServerConfig{
		ServerName:  "gopls",
		Language:    "go",
		Command:     "gopls",
		Args:        []string{"serve"},
		Transport:   "stdio",
		RootMarkers: []string{"go.mod", "go.sum", ".git"},

		// Optimal gopls settings for testing
		GoplsSettings: map[string]interface{}{
			// Core settings
			"usePlaceholders":              false,
			"completionDocumentation":      false,
			"hoverKind":                   "FullDocumentation",
			"linkTarget":                  "pkg.go.dev",
			"experimentalPostfixCompletions": false,

			// Performance optimizations
			"diagnosticsDelay":            "2s",
			"staticcheck":                 false,
			"gofumpt":                     false,

			// Disable intensive analyses for performance
			"analyses": map[string]bool{
				"fillstruct":      false,
				"nonewvars":       false,
				"unusedparams":    false,
				"unusedwrite":     false,
				"useany":          false,
				"fieldalignment":  false,
				"nilness":         false,
				"shadow":          false,
			},

			// Memory optimization
			"memoryMode":                  "DegradeClosed",
			"experimentalWorkspaceModule": false,

			// Template/snippet settings
			"completionBudget":            "500ms",
			"matcher":                     "Fuzzy",
			"symbolMatcher":               "FastFuzzy",
			"symbolStyle":                 "Dynamic",

			// Build settings
			"buildFlags":                  []string{"-tags=test"},
			"env": map[string]string{
				"CGO_ENABLED": "1",
			},

			// Gopls-specific optimizations
			"expandWorkspaceToModule":     false,
			"experimentalUseInvalidMetadata": false,
			"allowModfileModifications":   false,
			"allowImplicitNetworkAccess":  false,
		},

		BuildFlags: []string{
			"-tags=test",
			"-mod=readonly",
		},

		Environment: map[string]string{
			"GO111MODULE":       "on",
			"GOPROXY":          "direct",
			"GOSUMDB":          "sum.golang.org",
			"CGO_ENABLED":      "1",
			"GOFLAGS":          "-mod=readonly",
			"GOPRIVATE":        "",
			"GONOPROXY":        "",
			"GONOSUMDB":        "",
		},

		// Performance settings
		InitTimeout:     30 * time.Second,
		ResponseTimeout: 10 * time.Second,
		MemoryLimit:     512 * 1024 * 1024, // 512MB

		// Test mode settings
		TestMode: true,
		DisableFeatures: []string{
			"diagnostics",
			"codelens",
			"documentlink",
			"workspacesymbol.fuzzy",
		},
		EnableFeatures: []string{
			"definition",
			"references",
			"hover",
			"completion",
			"documentsymbol",
		},
	}
}

func (gen *LSPConfigGenerator) copyTemplate() *EnhancedLSPServerConfig {
	template := gen.templateConfig

	// Deep copy the configuration
	config := &EnhancedLSPServerConfig{
		ServerName:      template.ServerName,
		Language:        template.Language,
		Command:         template.Command,
		Args:            make([]string, len(template.Args)),
		Transport:       template.Transport,
		RootMarkers:     make([]string, len(template.RootMarkers)),
		WorkspaceRoot:   template.WorkspaceRoot,
		GoplsSettings:   make(map[string]interface{}),
		BuildFlags:      make([]string, len(template.BuildFlags)),
		Environment:     make(map[string]string),
		InitTimeout:     template.InitTimeout,
		ResponseTimeout: template.ResponseTimeout,
		MemoryLimit:     template.MemoryLimit,
		TestMode:        template.TestMode,
		DisableFeatures: make([]string, len(template.DisableFeatures)),
		EnableFeatures:  make([]string, len(template.EnableFeatures)),
	}

	// Copy slices and maps
	copy(config.Args, template.Args)
	copy(config.RootMarkers, template.RootMarkers)
	copy(config.BuildFlags, template.BuildFlags)
	copy(config.DisableFeatures, template.DisableFeatures)
	copy(config.EnableFeatures, template.EnableFeatures)

	// Deep copy gopls settings
	for k, v := range template.GoplsSettings {
		config.GoplsSettings[k] = v
	}

	// Copy environment variables
	for k, v := range template.Environment {
		config.Environment[k] = v
	}

	return config
}

func (gen *LSPConfigGenerator) validateGoplsBinary(command string) error {
	// Check if gopls binary exists
	if _, err := exec.LookPath(command); err != nil {
		return fmt.Errorf("gopls binary not found in PATH: %w", err)
	}

	// Verify gopls version and functionality
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, command, "version")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get gopls version: %w", err)
	}

	version := strings.TrimSpace(string(output))
	gen.logger.Printf("Detected gopls version: %s", version)

	return nil
}

func (gen *LSPConfigGenerator) validateWorkspaceStructure(workspaceRoot string) error {
	// Check for Go module
	goModPath := filepath.Join(workspaceRoot, "go.mod")
	if _, err := os.Stat(goModPath); err != nil {
		return fmt.Errorf("go.mod not found: %w", err)
	}

	// Check for Go source files
	hasGoFiles := false
	err := filepath.Walk(workspaceRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if strings.HasSuffix(info.Name(), ".go") && !strings.HasSuffix(info.Name(), "_test.go") {
			hasGoFiles = true
			return filepath.SkipDir // Stop walking once we find a Go file
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("error scanning workspace for Go files: %w", err)
	}

	if !hasGoFiles {
		return fmt.Errorf("no Go source files found in workspace")
	}

	return nil
}

func (gen *LSPConfigGenerator) optimizeForWorkspace(config *EnhancedLSPServerConfig, workspace *types.TestWorkspace) error {
	// Set GOPATH and workspace-specific settings
	workspaceGoPath := filepath.Join(workspace.RootPath, ".go")
	config.Environment["GOPATH"] = workspaceGoPath

	// Create workspace-specific cache directories
	goCacheDir := filepath.Join(workspace.RootPath, ".gocache")
	goModCacheDir := filepath.Join(workspace.RootPath, ".gomodcache")

	if err := os.MkdirAll(goCacheDir, 0755); err != nil {
		gen.logger.Printf("Warning: failed to create GOCACHE directory: %v", err)
	}
	if err := os.MkdirAll(goModCacheDir, 0755); err != nil {
		gen.logger.Printf("Warning: failed to create GOMODCACHE directory: %v", err)
	}

	// Optimize for repository characteristics (fatih/color is a small library)
	config.GoplsSettings["experimentalWorkspaceModule"] = false
	config.GoplsSettings["memoryMode"] = "DegradeClosed"

	// Set resource limits based on system
	gen.setResourceLimits(config)

	return nil
}

func (gen *LSPConfigGenerator) applyTestPerformanceOptimizations(config *EnhancedLSPServerConfig) {
	// Aggressive performance settings for testing
	config.GoplsSettings["diagnosticsDelay"] = "5s"
	config.GoplsSettings["completionBudget"] = "100ms"
	config.GoplsSettings["memoryMode"] = "DegradeClosed"

	// Disable expensive features
	config.GoplsSettings["staticcheck"] = false
	config.GoplsSettings["gofumpt"] = false

	// Minimal analyses for speed
	analyses := config.GoplsSettings["analyses"].(map[string]bool)
	for key := range analyses {
		analyses[key] = false
	}

	// Keep only essential analyses
	analyses["unreachable"] = true
	analyses["unusedparams"] = false
}

func (gen *LSPConfigGenerator) disableHeavyFeaturesForTesting(config *EnhancedLSPServerConfig) {
	// Disable features that are not needed for testing
	heavyFeatures := []string{
		"codelens",
		"documentlink", 
		"workspacesymbol.fuzzy",
		"semantictokens",
		"inlayhint",
	}

	for _, feature := range heavyFeatures {
		config.DisableFeatures = append(config.DisableFeatures, feature)
	}

	// Keep only essential LSP features for E2E testing
	config.EnableFeatures = []string{
		"definition",
		"references", 
		"hover",
		"completion",
		"documentsymbol",
		"workspacesymbol",
	}
}

func (gen *LSPConfigGenerator) optimizeMemoryForTesting(config *EnhancedLSPServerConfig) {
	// Set conservative memory limits for test environments
	systemRAM := getSystemRAM()
	
	// Use at most 256MB or 5% of system RAM, whichever is smaller
	maxMemory := int64(256 * 1024 * 1024) // 256MB
	if systemRAM > 0 {
		fivePercent := int64(float64(systemRAM) * 0.05)
		if fivePercent < maxMemory {
			maxMemory = fivePercent
		}
	}

	config.MemoryLimit = maxMemory

	// Configure gopls memory settings
	config.GoplsSettings["memoryMode"] = "DegradeClosed"
	config.Environment["GOGC"] = "100" // Standard garbage collection
}

func (gen *LSPConfigGenerator) setResourceLimits(config *EnhancedLSPServerConfig) {
	// Set reasonable resource limits based on system capabilities
	numCPU := runtime.NumCPU()
	
	// Limit concurrent operations based on CPU count
	if numCPU <= 2 {
		config.GoplsSettings["completionBudget"] = "200ms"
	} else if numCPU <= 4 {
		config.GoplsSettings["completionBudget"] = "300ms"
	} else {
		config.GoplsSettings["completionBudget"] = "500ms"
	}

	// Set GOMAXPROCS for gopls process
	config.Environment["GOMAXPROCS"] = fmt.Sprintf("%d", min(numCPU, 4))
}

func (gen *LSPConfigGenerator) convertToBasicConfig(enhanced *EnhancedLSPServerConfig) *types.LSPServerConfig {
	// Convert enhanced config back to the basic structure used by TestWorkspace
	return &types.LSPServerConfig{
		ServerPath:   enhanced.Command,
		Args:         enhanced.Args,
		InitOptions:  enhanced.GoplsSettings,
		WorkspaceDir: enhanced.WorkspaceRoot,
		Language:     enhanced.Language,
	}
}

func getSystemRAM() int64 {
	// Attempt to get system RAM (simplified approach)
	// This is a basic implementation - in production, you'd want more robust system detection
	
	if runtime.GOOS == "linux" {
		// Try to read from /proc/meminfo
		data, err := os.ReadFile("/proc/meminfo")
		if err == nil {
			lines := strings.Split(string(data), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "MemTotal:") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						// Convert kB to bytes (approximately)
						var kb int64
						if _, err := fmt.Sscanf(fields[1], "%d", &kb); err == nil {
							return kb * 1024
						}
					}
					break
				}
			}
		}
	}

	// Fallback: assume 8GB RAM for reasonable defaults
	return 8 * 1024 * 1024 * 1024
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// SetLogger allows customizing the logger used by the generator
func (gen *LSPConfigGenerator) SetLogger(logger *log.Logger) {
	gen.logger = logger
}

// GetTemplateConfig returns a copy of the template configuration
func (gen *LSPConfigGenerator) GetTemplateConfig() *EnhancedLSPServerConfig {
	return gen.copyTemplate()
}