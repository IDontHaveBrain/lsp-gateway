package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// LSPTestConfig defines the configuration for LSP testing
type LSPTestConfig struct {
	// General test configuration
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Timeout     time.Duration     `yaml:"timeout"`
	Parallel    bool              `yaml:"parallel"`
	Tags        []string          `yaml:"tags"`
	Env         map[string]string `yaml:"env"`

	// LSP server configuration
	Servers map[string]*ServerConfig `yaml:"servers"`

	// Test repositories
	Repositories []*RepositoryConfig `yaml:"repositories"`

	// Test execution settings
	Execution *ExecutionConfig `yaml:"execution"`

	// Validation settings
	Validation *ValidationConfig `yaml:"validation"`

	// Reporting configuration
	Reporting *ReportingConfig `yaml:"reporting"`
}

// ServerConfig defines LSP server configuration for testing
type ServerConfig struct {
	Name        string            `yaml:"name"`
	Language    string            `yaml:"language"`
	Command     string            `yaml:"command"`
	Args        []string          `yaml:"args"`
	Transport   string            `yaml:"transport"`
	WorkingDir  string            `yaml:"working_dir"`
	Env         map[string]string `yaml:"env"`
	InitOptions map[string]any    `yaml:"init_options"`
	
	// Server lifecycle settings
	StartTimeout    time.Duration `yaml:"start_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
	
	// Testing-specific settings
	PreWarmup       bool     `yaml:"pre_warmup"`
	InitializeDelay time.Duration `yaml:"initialize_delay"`
}

// RepositoryConfig defines a test repository
type RepositoryConfig struct {
	Name        string            `yaml:"name"`
	Path        string            `yaml:"path"`
	Language    string            `yaml:"language"`
	Description string            `yaml:"description"`
	Setup       *RepositorySetup  `yaml:"setup"`
	TestCases   []*TestCaseConfig `yaml:"test_cases"`
	Tags        []string          `yaml:"tags"`
}

// RepositorySetup defines repository setup requirements
type RepositorySetup struct {
	Commands     []string          `yaml:"commands"`
	Dependencies []string          `yaml:"dependencies"`
	BuildCmd     string            `yaml:"build_cmd"`
	Env          map[string]string `yaml:"env"`
	Timeout      time.Duration     `yaml:"timeout"`
}

// TestCaseConfig defines a single test case
type TestCaseConfig struct {
	ID          string                 `yaml:"id"`
	Name        string                 `yaml:"name"`
	Description string                 `yaml:"description"`
	Method      string                 `yaml:"method"`
	File        string                 `yaml:"file"`
	Position    *Position              `yaml:"position"`
	Params      map[string]interface{} `yaml:"params"`
	Expected    *ExpectedResult        `yaml:"expected"`
	Tags        []string               `yaml:"tags"`
	Timeout     time.Duration          `yaml:"timeout"`
	Skip        bool                   `yaml:"skip"`
	SkipReason  string                 `yaml:"skip_reason"`
}

// Position defines a position in a source file
type Position struct {
	Line      int `yaml:"line"`
	Character int `yaml:"character"`
}

// ExpectedResult defines expected test results
type ExpectedResult struct {
	Success     bool                   `yaml:"success"`
	ErrorCode   *int                   `yaml:"error_code,omitempty"`
	ErrorMsg    *string                `yaml:"error_msg,omitempty"`
	ResultCount *int                   `yaml:"result_count,omitempty"`
	Contains    []string               `yaml:"contains,omitempty"`
	Excludes    []string               `yaml:"excludes,omitempty"`
	Properties  map[string]interface{} `yaml:"properties,omitempty"`
	
	// Method-specific expectations
	Definition  *DefinitionExpected  `yaml:"definition,omitempty"`
	References  *ReferencesExpected  `yaml:"references,omitempty"`
	Hover       *HoverExpected       `yaml:"hover,omitempty"`
	Symbols     *SymbolsExpected     `yaml:"symbols,omitempty"`
}

// Method-specific expected result types
type DefinitionExpected struct {
	HasLocation bool     `yaml:"has_location"`
	FileURI     *string  `yaml:"file_uri,omitempty"`
	Range       *Range   `yaml:"range,omitempty"`
}

type ReferencesExpected struct {
	MinCount    int      `yaml:"min_count"`
	MaxCount    int      `yaml:"max_count"`
	IncludeDecl bool     `yaml:"include_declaration"`
}

type HoverExpected struct {
	HasContent bool     `yaml:"has_content"`
	Contains   []string `yaml:"contains,omitempty"`
	Format     string   `yaml:"format,omitempty"`
}

type SymbolsExpected struct {
	MinCount int      `yaml:"min_count"`
	MaxCount int      `yaml:"max_count"`
	Types    []string `yaml:"types,omitempty"`
}

type Range struct {
	Start *Position `yaml:"start"`
	End   *Position `yaml:"end"`
}

// ExecutionConfig defines test execution settings
type ExecutionConfig struct {
	MaxConcurrency   int           `yaml:"max_concurrency"`
	DefaultTimeout   time.Duration `yaml:"default_timeout"`
	RetryAttempts    int           `yaml:"retry_attempts"`
	RetryDelay       time.Duration `yaml:"retry_delay"`
	FailFast         bool          `yaml:"fail_fast"`
	RandomizeOrder   bool          `yaml:"randomize_order"`
	KeepServersAlive bool          `yaml:"keep_servers_alive"`
}

// ValidationConfig defines validation settings
type ValidationConfig struct {
	StrictMode        bool `yaml:"strict_mode"`
	ValidateTypes     bool `yaml:"validate_types"`
	ValidatePositions bool `yaml:"validate_positions"`
	ValidateURIs      bool `yaml:"validate_uris"`
}

// ReportingConfig defines reporting settings
type ReportingConfig struct {
	Formats    []string `yaml:"formats"`    // console, json, junit
	OutputDir  string   `yaml:"output_dir"`
	Verbose    bool     `yaml:"verbose"`
	IncludeTiming bool  `yaml:"include_timing"`
	SaveDetails   bool  `yaml:"save_details"`
}

// DefaultLSPTestConfig returns a default configuration
func DefaultLSPTestConfig() *LSPTestConfig {
	return &LSPTestConfig{
		Name:        "LSP Test Suite",
		Description: "Comprehensive LSP functionality testing",
		Timeout:     5 * time.Minute,
		Parallel:    true,
		Servers:     make(map[string]*ServerConfig),
		Execution: &ExecutionConfig{
			MaxConcurrency:   4,
			DefaultTimeout:   30 * time.Second,
			RetryAttempts:    2,
			RetryDelay:       1 * time.Second,
			FailFast:         false,
			RandomizeOrder:   false,
			KeepServersAlive: true,
		},
		Validation: &ValidationConfig{
			StrictMode:        false,
			ValidateTypes:     true,
			ValidatePositions: true,
			ValidateURIs:      true,
		},
		Reporting: &ReportingConfig{
			Formats:       []string{"console"},
			OutputDir:     "test-results",
			Verbose:       false,
			IncludeTiming: true,
			SaveDetails:   false,
		},
	}
}

// DefaultServerConfigs returns default configurations for common LSP servers
func DefaultServerConfigs() map[string]*ServerConfig {
	return map[string]*ServerConfig{
		"gopls": {
			Name:            "gopls",
			Language:        "go",
			Command:         "gopls",
			Args:            []string{},
			Transport:       "stdio",
			StartTimeout:    10 * time.Second,
			ShutdownTimeout: 5 * time.Second,
			PreWarmup:       true,
			InitializeDelay: 500 * time.Millisecond,
		},
		"pylsp": {
			Name:            "python-lsp-server",
			Language:        "python",
			Command:         "pylsp",
			Args:            []string{},
			Transport:       "stdio",
			StartTimeout:    15 * time.Second,
			ShutdownTimeout: 5 * time.Second,
			PreWarmup:       true,
			InitializeDelay: 1 * time.Second,
		},
		"tsserver": {
			Name:            "typescript-language-server",
			Language:        "typescript",
			Command:         "typescript-language-server",
			Args:            []string{"--stdio"},
			Transport:       "stdio",
			StartTimeout:    10 * time.Second,
			ShutdownTimeout: 5 * time.Second,
			PreWarmup:       true,
			InitializeDelay: 500 * time.Millisecond,
		},
		"jdtls": {
			Name:            "jdtls",
			Language:        "java",
			Command:         "jdtls",
			Args:            []string{},
			Transport:       "stdio",
			StartTimeout:    30 * time.Second,
			ShutdownTimeout: 10 * time.Second,
			PreWarmup:       true,
			InitializeDelay: 2 * time.Second,
		},
	}
}

// LoadConfig loads LSP test configuration from file
func LoadConfig(configPath string) (*LSPTestConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	config := DefaultLSPTestConfig()
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Merge with default server configurations
	defaultServers := DefaultServerConfigs()
	for name, defaultServer := range defaultServers {
		if _, exists := config.Servers[name]; !exists {
			config.Servers[name] = defaultServer
		}
	}

	return config, nil
}

// LoadRepositoriesConfig loads test repositories configuration
func LoadRepositoriesConfig(configPath string) ([]*RepositoryConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read repositories config: %w", err)
	}

	var repositories []*RepositoryConfig
	if err := yaml.Unmarshal(data, &repositories); err != nil {
		return nil, fmt.Errorf("failed to parse repositories config: %w", err)
	}

	return repositories, nil
}

// SaveConfig saves configuration to file
func (c *LSPTestConfig) SaveConfig(configPath string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Validate validates the configuration
func (c *LSPTestConfig) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("config name is required")
	}

	if len(c.Servers) == 0 {
		return fmt.Errorf("at least one server configuration is required")
	}

	for name, server := range c.Servers {
		if err := server.Validate(); err != nil {
			return fmt.Errorf("server %q configuration invalid: %w", name, err)
		}
	}

	if len(c.Repositories) == 0 {
		return fmt.Errorf("at least one test repository is required")
	}

	for i, repo := range c.Repositories {
		if err := repo.Validate(); err != nil {
			return fmt.Errorf("repository %d configuration invalid: %w", i, err)
		}
	}

	return nil
}

// Validate validates server configuration
func (s *ServerConfig) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("server name is required")
	}
	if s.Language == "" {
		return fmt.Errorf("server language is required")
	}
	if s.Command == "" {
		return fmt.Errorf("server command is required")
	}
	if s.Transport == "" {
		s.Transport = "stdio"
	}
	if s.StartTimeout <= 0 {
		s.StartTimeout = 10 * time.Second
	}
	if s.ShutdownTimeout <= 0 {
		s.ShutdownTimeout = 5 * time.Second
	}
	return nil
}

// Validate validates repository configuration
func (r *RepositoryConfig) Validate() error {
	if r.Name == "" {
		return fmt.Errorf("repository name is required")
	}
	if r.Path == "" {
		return fmt.Errorf("repository path is required")
	}
	if r.Language == "" {
		return fmt.Errorf("repository language is required")
	}

	// Validate test cases
	for i, testCase := range r.TestCases {
		if err := testCase.Validate(); err != nil {
			return fmt.Errorf("test case %d invalid: %w", i, err)
		}
	}

	return nil
}

// Validate validates test case configuration
func (t *TestCaseConfig) Validate() error {
	if t.ID == "" {
		return fmt.Errorf("test case ID is required")
	}
	if t.Name == "" {
		return fmt.Errorf("test case name is required")
	}
	if t.Method == "" {
		return fmt.Errorf("test case method is required")
	}
	if t.File == "" {
		return fmt.Errorf("test case file is required")
	}
	if t.Position == nil {
		return fmt.Errorf("test case position is required")
	}
	if t.Timeout <= 0 {
		t.Timeout = 30 * time.Second
	}
	return nil
}

// GetConfigContext creates a context with configuration values
func (c *LSPTestConfig) GetConfigContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, "lsp_test_config", c)
}

// GetConfigFromContext retrieves configuration from context
func GetConfigFromContext(ctx context.Context) *LSPTestConfig {
	if config, ok := ctx.Value("lsp_test_config").(*LSPTestConfig); ok {
		return config
	}
	return nil
}