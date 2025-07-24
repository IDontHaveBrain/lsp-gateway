package config

import (
	"fmt"
	"lsp-gateway/internal/transport"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	DefaultConfigFile = "config.yaml"
)

const (
	DefaultTransport = "stdio"
)

const (
	ProjectTypeSingle    = "single-language"
	ProjectTypeMulti     = "multi-language"
	ProjectTypeMonorepo  = "monorepo"
	ProjectTypeWorkspace = "workspace"
)

type LanguageInfo struct {
	Language     string   `yaml:"language" json:"language"`
	FilePatterns []string `yaml:"file_patterns" json:"file_patterns"`
	FileCount    int      `yaml:"file_count" json:"file_count"`
	RootMarkers  []string `yaml:"root_markers,omitempty" json:"root_markers,omitempty"`
}

type ProjectContext struct {
	ProjectType   string         `yaml:"project_type" json:"project_type"`
	RootDirectory string         `yaml:"root_directory" json:"root_directory"`
	WorkspaceRoot string         `yaml:"workspace_root,omitempty" json:"workspace_root,omitempty"`
	Languages     []LanguageInfo `yaml:"languages" json:"languages"`
	RequiredLSPs  []string       `yaml:"required_lsps" json:"required_lsps"`
	DetectedAt    time.Time      `yaml:"detected_at" json:"detected_at"`
	Metadata      map[string]interface{} `yaml:"metadata,omitempty" json:"metadata,omitempty"`
}

type ProjectServerOverride struct {
	Name      string            `yaml:"name" json:"name"`
	Enabled   *bool             `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	Args      []string          `yaml:"args,omitempty" json:"args,omitempty"`
	Settings  map[string]interface{} `yaml:"settings,omitempty" json:"settings,omitempty"`
	Transport string            `yaml:"transport,omitempty" json:"transport,omitempty"`
}

type ProjectConfig struct {
	ProjectID       string                   `yaml:"project_id" json:"project_id"`
	Name            string                   `yaml:"name,omitempty" json:"name,omitempty"`
	RootDirectory   string                   `yaml:"root_directory" json:"root_directory"`
	ServerOverrides []ProjectServerOverride  `yaml:"server_overrides,omitempty" json:"server_overrides,omitempty"`
	EnabledServers  []string                 `yaml:"enabled_servers,omitempty" json:"enabled_servers,omitempty"`
	Optimizations   map[string]interface{}   `yaml:"optimizations,omitempty" json:"optimizations,omitempty"`
	GeneratedAt     time.Time                `yaml:"generated_at" json:"generated_at"`
	Version         string                   `yaml:"version,omitempty" json:"version,omitempty"`
}

type ServerConfig struct {
	Name string `yaml:"name" json:"name"`

	Languages []string `yaml:"languages" json:"languages"`

	Command string `yaml:"command" json:"command"`

	Args []string `yaml:"args" json:"args"`

	Transport string `yaml:"transport" json:"transport"`

	RootMarkers []string `yaml:"root_markers,omitempty" json:"root_markers,omitempty"`

	Settings map[string]interface{} `yaml:"settings,omitempty" json:"settings,omitempty"`
}

type GatewayConfig struct {
	Servers []ServerConfig `yaml:"servers" json:"servers"`

	Port int `yaml:"port" json:"port"`

	Timeout               string         `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	MaxConcurrentRequests int            `yaml:"max_concurrent_requests,omitempty" json:"max_concurrent_requests,omitempty"`
	ProjectContext        *ProjectContext `yaml:"project_context,omitempty" json:"project_context,omitempty"`
	ProjectConfig         *ProjectConfig  `yaml:"project_config,omitempty" json:"project_config,omitempty"`
	ProjectAware          bool           `yaml:"project_aware,omitempty" json:"project_aware,omitempty"`
}

func DefaultConfig() *GatewayConfig {
	return &GatewayConfig{
		Port:                  8080,
		Timeout:               "30s",
		MaxConcurrentRequests: 100,
		ProjectAware:          false,
		Servers: []ServerConfig{
			{
				Name:        "go-lsp",
				Languages:   []string{"go"},
				Command:     "gopls",
				Args:        []string{},
				Transport:   DefaultTransport,
				RootMarkers: []string{"go.mod", "go.sum"},
			},
		},
	}
}

func (c *GatewayConfig) Validate() error {
	if c.Port < 0 || c.Port > 65535 {
		return fmt.Errorf("invalid port: %d, must be between 0 and 65535", c.Port)
	}

	if len(c.Servers) == 0 {
		return fmt.Errorf("at least one server must be configured")
	}

	names := make(map[string]bool)
	for _, server := range c.Servers {
		if err := server.Validate(); err != nil {
			return fmt.Errorf("server %s: %w", server.Name, err)
		}

		if names[server.Name] {
			return fmt.Errorf("duplicate server name: %s", server.Name)
		}
		names[server.Name] = true
	}

	if c.ProjectContext != nil {
		if err := c.ProjectContext.Validate(); err != nil {
			return fmt.Errorf("project context validation failed: %w", err)
		}
	}

	if c.ProjectConfig != nil {
		if err := c.ProjectConfig.Validate(); err != nil {
			return fmt.Errorf("project config validation failed: %w", err)
		}
	}

	return nil
}

func (s *ServerConfig) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("server name cannot be empty")
	}

	// Check for extremely long server names
	if len(s.Name) > 500 {
		return fmt.Errorf("server name too long: %d characters, maximum 500 allowed", len(s.Name))
	}

	if len(s.Languages) == 0 {
		return fmt.Errorf("server must support at least one language")
	}

	// Validate individual language strings
	for i, lang := range s.Languages {
		if strings.TrimSpace(lang) == "" {
			return fmt.Errorf("language at index %d cannot be empty or whitespace-only", i)
		}
	}

	if s.Command == "" {
		return fmt.Errorf("server command cannot be empty")
	}

	// Check for invalid characters in command
	if !utf8.ValidString(s.Command) {
		return fmt.Errorf("command contains invalid UTF-8 characters")
	}
	if strings.ContainsAny(s.Command, "\x00\x01\x02\x03\x04\x05\x06\x07\x08\x0b\x0c\x0e\x0f") {
		return fmt.Errorf("command contains invalid control characters")
	}

	if s.Transport == "" {
		return fmt.Errorf("server transport cannot be empty")
	}

	validTransports := map[string]bool{
		transport.TransportStdio: true,
		transport.TransportTCP:   true,
		transport.TransportHTTP:  true,
	}

	if !validTransports[s.Transport] {
		return fmt.Errorf("invalid transport type: %s, must be one of: stdio, tcp, http", s.Transport)
	}

	return nil
}

func (pc *ProjectContext) Validate() error {
	if pc.ProjectType == "" {
		return fmt.Errorf("project type cannot be empty")
	}

	validTypes := map[string]bool{
		ProjectTypeSingle:    true,
		ProjectTypeMulti:     true,
		ProjectTypeMonorepo:  true,
		ProjectTypeWorkspace: true,
	}

	if !validTypes[pc.ProjectType] {
		return fmt.Errorf("invalid project type: %s, must be one of: single-language, multi-language, monorepo, workspace", pc.ProjectType)
	}

	if pc.RootDirectory == "" {
		return fmt.Errorf("root directory cannot be empty")
	}

	if !filepath.IsAbs(pc.RootDirectory) {
		return fmt.Errorf("root directory must be an absolute path: %s", pc.RootDirectory)
	}

	if pc.WorkspaceRoot != "" && !filepath.IsAbs(pc.WorkspaceRoot) {
		return fmt.Errorf("workspace root must be an absolute path: %s", pc.WorkspaceRoot)
	}

	if len(pc.Languages) == 0 {
		return fmt.Errorf("at least one language must be detected")
	}

	for i, lang := range pc.Languages {
		if err := lang.Validate(); err != nil {
			return fmt.Errorf("language info at index %d: %w", i, err)
		}
	}

	if pc.DetectedAt.IsZero() {
		return fmt.Errorf("detected_at timestamp cannot be zero")
	}

	return nil
}

func (li *LanguageInfo) Validate() error {
	if li.Language == "" {
		return fmt.Errorf("language cannot be empty")
	}

	if strings.TrimSpace(li.Language) == "" {
		return fmt.Errorf("language cannot be whitespace-only")
	}

	if len(li.FilePatterns) == 0 {
		return fmt.Errorf("at least one file pattern must be specified")
	}

	for i, pattern := range li.FilePatterns {
		if strings.TrimSpace(pattern) == "" {
			return fmt.Errorf("file pattern at index %d cannot be empty or whitespace-only", i)
		}
	}

	if li.FileCount < 0 {
		return fmt.Errorf("file count cannot be negative: %d", li.FileCount)
	}

	return nil
}

func (pc *ProjectConfig) Validate() error {
	if pc.ProjectID == "" {
		return fmt.Errorf("project ID cannot be empty")
	}

	if pc.RootDirectory == "" {
		return fmt.Errorf("root directory cannot be empty")
	}

	if !filepath.IsAbs(pc.RootDirectory) {
		return fmt.Errorf("root directory must be an absolute path: %s", pc.RootDirectory)
	}

	for i, override := range pc.ServerOverrides {
		if err := override.Validate(); err != nil {
			return fmt.Errorf("server override at index %d: %w", i, err)
		}
	}

	for i, serverName := range pc.EnabledServers {
		if strings.TrimSpace(serverName) == "" {
			return fmt.Errorf("enabled server name at index %d cannot be empty or whitespace-only", i)
		}
	}

	if pc.GeneratedAt.IsZero() {
		return fmt.Errorf("generated_at timestamp cannot be zero")
	}

	return nil
}

func (pso *ProjectServerOverride) Validate() error {
	if pso.Name == "" {
		return fmt.Errorf("server override name cannot be empty")
	}

	if pso.Transport != "" {
		validTransports := map[string]bool{
			transport.TransportStdio: true,
			transport.TransportTCP:   true,
			transport.TransportHTTP:  true,
		}

		if !validTransports[pso.Transport] {
			return fmt.Errorf("invalid transport type: %s, must be one of: stdio, tcp, http", pso.Transport)
		}
	}

	return nil
}

func (c *GatewayConfig) GetServerByLanguage(language string) (*ServerConfig, error) {
	for _, server := range c.Servers {
		for _, lang := range server.Languages {
			if lang == language {
				return &server, nil
			}
		}
	}
	return nil, fmt.Errorf("no server found for language: %s", language)
}

func (c *GatewayConfig) GetServerByName(name string) (*ServerConfig, error) {
	for _, server := range c.Servers {
		if server.Name == name {
			return &server, nil
		}
	}
	return nil, fmt.Errorf("no server found with name: %s", name)
}

func (c *GatewayConfig) GetProjectAwareServers() []ServerConfig {
	if !c.ProjectAware || c.ProjectConfig == nil {
		return c.Servers
	}

	if len(c.ProjectConfig.EnabledServers) == 0 {
		return c.Servers
	}

	enabledMap := make(map[string]bool)
	for _, name := range c.ProjectConfig.EnabledServers {
		enabledMap[name] = true
	}

	var filteredServers []ServerConfig
	for _, server := range c.Servers {
		if enabledMap[server.Name] {
			filteredServers = append(filteredServers, server)
		}
	}

	return filteredServers
}

func (c *GatewayConfig) ApplyProjectOverrides() error {
	if !c.ProjectAware || c.ProjectConfig == nil {
		return nil
	}

	overridesMap := make(map[string]ProjectServerOverride)
	for _, override := range c.ProjectConfig.ServerOverrides {
		overridesMap[override.Name] = override
	}

	for i, server := range c.Servers {
		if override, exists := overridesMap[server.Name]; exists {
			if override.Enabled != nil && !*override.Enabled {
				continue
			}

			if len(override.Args) > 0 {
				c.Servers[i].Args = override.Args
			}

			if override.Transport != "" {
				c.Servers[i].Transport = override.Transport
			}

			if override.Settings != nil {
				c.Servers[i].Settings = override.Settings
			}
		}
	}

	return nil
}

func (c *GatewayConfig) GetRequiredLSPServers() []string {
	if c.ProjectContext == nil {
		return nil
	}
	return c.ProjectContext.RequiredLSPs
}

func (c *GatewayConfig) GetDetectedLanguages() []string {
	if c.ProjectContext == nil {
		return nil
	}

	var languages []string
	for _, lang := range c.ProjectContext.Languages {
		languages = append(languages, lang.Language)
	}
	return languages
}

func (c *GatewayConfig) IsProjectType(projectType string) bool {
	if c.ProjectContext == nil {
		return false
	}
	return c.ProjectContext.ProjectType == projectType
}

func (c *GatewayConfig) GetProjectRoot() string {
	if c.ProjectContext == nil {
		return ""
	}
	return c.ProjectContext.RootDirectory
}

func (c *GatewayConfig) GetWorkspaceRoot() string {
	if c.ProjectContext == nil {
		return ""
	}
	return c.ProjectContext.WorkspaceRoot
}

func (c *GatewayConfig) HasLanguage(language string) bool {
	if c.ProjectContext == nil {
		return false
	}

	for _, lang := range c.ProjectContext.Languages {
		if lang.Language == language {
			return true
		}
	}
	return false
}

func (c *GatewayConfig) GetLanguageFileCount(language string) int {
	if c.ProjectContext == nil {
		return 0
	}

	for _, lang := range c.ProjectContext.Languages {
		if lang.Language == language {
			return lang.FileCount
		}
	}
	return 0
}

func NewProjectContext(projectType, rootDir string) *ProjectContext {
	absRoot, _ := filepath.Abs(rootDir)
	return &ProjectContext{
		ProjectType:   projectType,
		RootDirectory: absRoot,
		Languages:     []LanguageInfo{},
		RequiredLSPs:  []string{},
		DetectedAt:    time.Now(),
		Metadata:      make(map[string]interface{}),
	}
}

func NewProjectConfig(projectID, rootDir string) *ProjectConfig {
	absRoot, _ := filepath.Abs(rootDir)
	return &ProjectConfig{
		ProjectID:       projectID,
		RootDirectory:   absRoot,
		ServerOverrides: []ProjectServerOverride{},
		EnabledServers:  []string{},
		Optimizations:   make(map[string]interface{}),
		GeneratedAt:     time.Now(),
	}
}
