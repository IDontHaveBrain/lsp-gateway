package setup

import (
	"fmt"
	"lsp-gateway/internal/config"
	"strings"
	"time"
)

// ConfigurationTemplate defines a template for common project patterns
type ConfigurationTemplate struct {
	Name               string                          `json:"name"`
	Version            string                          `json:"version"`
	Description        string                          `json:"description"`
	ProjectTypes       []string                        `json:"project_types"`       // Project types this template supports
	TargetLanguages    []string                        `json:"target_languages"`    // Languages this template is designed for
	DefaultPort        int                             `json:"default_port"`
	EnableMultiServer  bool                            `json:"enable_multi_server"`
	EnableSmartRouting bool                            `json:"enable_smart_routing"`
	ResourceLimits     *TemplateResourceLimits         `json:"resource_limits"`
	RoutingConfig      *TemplateRoutingConfig          `json:"routing_config"`
	FrameworkConfigs   map[string]*FrameworkConfig     `json:"framework_configs"`   // Framework-specific configurations
	EnvironmentConfigs map[string]*EnvironmentConfig   `json:"environment_configs"` // Environment-specific settings
	ServerTemplates    map[string]*ServerTemplate      `json:"server_templates"`    // Language server templates
	Metadata           map[string]interface{}          `json:"metadata"`
	CreatedAt          time.Time                       `json:"created_at"`
}

// TemplateResourceLimits defines resource limits for templates
type TemplateResourceLimits struct {
	MaxConcurrentRequests int `json:"max_concurrent_requests"`
	TimeoutSeconds        int `json:"timeout_seconds"`
	MaxMemoryMB           int `json:"max_memory_mb"`
	MaxProcesses          int `json:"max_processes"`
}

// TemplateRoutingConfig defines routing configuration for templates
type TemplateRoutingConfig struct {
	EnableSmartRouting         bool   `json:"enable_smart_routing"`
	DefaultStrategy            string `json:"default_strategy"`
	EnablePerformanceMonitoring bool   `json:"enable_performance_monitoring"`
	EnableCircuitBreaker       bool   `json:"enable_circuit_breaker"`
	LoadBalancingStrategy      string `json:"load_balancing_strategy"`
}

// FrameworkConfig defines framework-specific configuration
type FrameworkConfig struct {
	Name           string                 `json:"name"`
	Language       string                 `json:"language"`
	ServerSettings map[string]interface{} `json:"server_settings"`
	RootMarkers    []string               `json:"root_markers"`
	RequiredFiles  []string               `json:"required_files"`
	OptionalFiles  []string               `json:"optional_files"`
	Optimizations  map[string]interface{} `json:"optimizations"`
}

// EnvironmentConfig defines environment-specific configuration
type EnvironmentConfig struct {
	Name           string                 `json:"name"`
	Port           int                    `json:"port"`
	ResourceLimits *TemplateResourceLimits `json:"resource_limits"`
	Features       map[string]bool        `json:"features"`
	Settings       map[string]interface{} `json:"settings"`
}

// ServerTemplate defines a language server template
type ServerTemplate struct {
	Name        string                 `json:"name"`
	Language    string                 `json:"language"`
	Command     string                 `json:"command"`
	Args        []string               `json:"args"`
	Transport   string                 `json:"transport"`
	RootMarkers []string               `json:"root_markers"`
	Settings    map[string]interface{} `json:"settings"`
	Priority    int                    `json:"priority"`
	Weight      float64                `json:"weight"`
}

// ConfigurationTemplateManager manages configuration templates
type ConfigurationTemplateManager struct {
	templates map[string]*ConfigurationTemplate
	logger    *SetupLogger
}

// NewConfigurationTemplateManager creates a new template manager
func NewConfigurationTemplateManager() *ConfigurationTemplateManager {
	manager := &ConfigurationTemplateManager{
		templates: make(map[string]*ConfigurationTemplate),
		logger:    NewSetupLogger(nil),
	}
	
	manager.initializeBuiltinTemplates()
	
	return manager
}

// initializeBuiltinTemplates initializes built-in configuration templates
func (tm *ConfigurationTemplateManager) initializeBuiltinTemplates() {
	// Monorepo template
	tm.templates["monorepo"] = &ConfigurationTemplate{
		Name:               "monorepo",
		Version:            "1.0",
		Description:        "Configuration template for monorepo projects with multiple languages",
		ProjectTypes:       []string{config.ProjectTypeMonorepo},
		TargetLanguages:    []string{"go", "python", "typescript", "java", "rust"},
		DefaultPort:        8080,
		EnableMultiServer:  true,
		EnableSmartRouting: true,
		ResourceLimits: &TemplateResourceLimits{
			MaxConcurrentRequests: 300,
			TimeoutSeconds:        60,
			MaxMemoryMB:           4096,
			MaxProcesses:          10,
		},
		RoutingConfig: &TemplateRoutingConfig{
			EnableSmartRouting:          true,
			DefaultStrategy:             "broadcast_aggregate",
			EnablePerformanceMonitoring: true,
			EnableCircuitBreaker:        true,
			LoadBalancingStrategy:       "weighted_round_robin",
		},
		FrameworkConfigs: map[string]*FrameworkConfig{
			"react": {
				Name:     "react",
				Language: "typescript",
				ServerSettings: map[string]interface{}{
					"typescript": map[string]interface{}{
						"preferences": map[string]interface{}{
							"includeCompletionsForModuleExports": true,
							"includeCompletionsWithSnippetText":  true,
						},
					},
				},
				RootMarkers:   []string{"package.json", "tsconfig.json"},
				RequiredFiles: []string{"package.json"},
			},
			"django": {
				Name:     "django",
				Language: "python",
				ServerSettings: map[string]interface{}{
					"pylsp": map[string]interface{}{
						"plugins": map[string]interface{}{
							"pycodestyle": map[string]bool{"enabled": true},
							"mypy-ls":     map[string]bool{"enabled": true},
						},
					},
				},
				RootMarkers:   []string{"manage.py", "requirements.txt"},
				RequiredFiles: []string{"manage.py"},
			},
		},
		ServerTemplates: map[string]*ServerTemplate{
			"go": {
				Name:        "gopls",
				Language:    "go",
				Command:     "gopls",
				Args:        []string{},
				Transport:   "stdio",
				RootMarkers: []string{"go.mod", "go.sum"},
				Settings: map[string]interface{}{
					"gopls": map[string]interface{}{
						"staticcheck":   true,
						"verboseOutput": false,
					},
				},
				Priority: 5,
				Weight:   1.0,
			},
		},
		CreatedAt: time.Now(),
	}
	
	// Multi-language template
	tm.templates["multi-language"] = &ConfigurationTemplate{
		Name:               "multi-language",
		Version:            "1.0",
		Description:        "Configuration template for multi-language projects",
		ProjectTypes:       []string{config.ProjectTypeMulti},
		TargetLanguages:    []string{"go", "python", "typescript", "java"},
		DefaultPort:        8080,
		EnableMultiServer:  true,
		EnableSmartRouting: false,
		ResourceLimits: &TemplateResourceLimits{
			MaxConcurrentRequests: 200,
			TimeoutSeconds:        30,
			MaxMemoryMB:           2048,
			MaxProcesses:          5,
		},
		RoutingConfig: &TemplateRoutingConfig{
			EnableSmartRouting:          false,
			DefaultStrategy:             "single_target_with_fallback",
			EnablePerformanceMonitoring: false,
			EnableCircuitBreaker:        true,
			LoadBalancingStrategy:       "round_robin",
		},
		CreatedAt: time.Now(),
	}
	
	// Microservices template
	tm.templates["microservices"] = &ConfigurationTemplate{
		Name:               "microservices",
		Version:            "1.0",
		Description:        "Configuration template for microservices architecture",
		ProjectTypes:       []string{config.ProjectTypeMicroservices},
		TargetLanguages:    []string{"go", "python", "typescript", "java"},
		DefaultPort:        8080,
		EnableMultiServer:  true,
		EnableSmartRouting: true,
		ResourceLimits: &TemplateResourceLimits{
			MaxConcurrentRequests: 500,
			TimeoutSeconds:        45,
			MaxMemoryMB:           6144,
			MaxProcesses:          15,
		},
		RoutingConfig: &TemplateRoutingConfig{
			EnableSmartRouting:          true,
			DefaultStrategy:             "service_mesh",
			EnablePerformanceMonitoring: true,
			EnableCircuitBreaker:        true,
			LoadBalancingStrategy:       "least_connections",
		},
		CreatedAt: time.Now(),
	}
	
	// Single language template
	tm.templates["single-language"] = &ConfigurationTemplate{
		Name:               "single-language",
		Version:            "1.0",
		Description:        "Configuration template for single-language projects",
		ProjectTypes:       []string{config.ProjectTypeSingle},
		TargetLanguages:    []string{"go", "python", "typescript", "java", "rust"},
		DefaultPort:        8080,
		EnableMultiServer:  false,
		EnableSmartRouting: false,
		ResourceLimits: &TemplateResourceLimits{
			MaxConcurrentRequests: 100,
			TimeoutSeconds:        30,
			MaxMemoryMB:           1024,
			MaxProcesses:          3,
		},
		RoutingConfig: &TemplateRoutingConfig{
			EnableSmartRouting:          false,
			DefaultStrategy:             "direct",
			EnablePerformanceMonitoring: false,
			EnableCircuitBreaker:        false,
			LoadBalancingStrategy:       "direct",
		},
		CreatedAt: time.Now(),
	}
	
	// Development environment template
	tm.templates["development"] = &ConfigurationTemplate{
		Name:               "development",
		Version:            "1.0",
		Description:        "Configuration template optimized for development environment",
		ProjectTypes:       []string{config.ProjectTypeSingle, config.ProjectTypeMulti},
		TargetLanguages:    []string{"go", "python", "typescript", "java", "rust"},
		DefaultPort:        8080,
		EnableMultiServer:  true,
		EnableSmartRouting: false,
		ResourceLimits: &TemplateResourceLimits{
			MaxConcurrentRequests: 100,
			TimeoutSeconds:        30,
			MaxMemoryMB:           2048,
			MaxProcesses:          5,
		},
		EnvironmentConfigs: map[string]*EnvironmentConfig{
			"development": {
				Name: "development",
				Port: 8080,
				ResourceLimits: &TemplateResourceLimits{
					MaxConcurrentRequests: 100,
					TimeoutSeconds:        30,
					MaxMemoryMB:           2048,
					MaxProcesses:          5,
				},
				Features: map[string]bool{
					"enable_diagnostics":   true,
					"enable_code_lenses":   true,
					"enable_hover_info":    true,
					"enable_debug_logging": true,
				},
			},
		},
		CreatedAt: time.Now(),
	}
	
	// Production environment template
	tm.templates["production"] = &ConfigurationTemplate{
		Name:               "production",
		Version:            "1.0",
		Description:        "Configuration template optimized for production environment",
		ProjectTypes:       []string{config.ProjectTypeSingle, config.ProjectTypeMulti, config.ProjectTypeMonorepo},
		TargetLanguages:    []string{"go", "python", "typescript", "java", "rust"},
		DefaultPort:        8080,
		EnableMultiServer:  true,
		EnableSmartRouting: true,
		ResourceLimits: &TemplateResourceLimits{
			MaxConcurrentRequests: 500,
			TimeoutSeconds:        15,
			MaxMemoryMB:           4096,
			MaxProcesses:          10,
		},
		EnvironmentConfigs: map[string]*EnvironmentConfig{
			"production": {
				Name: "production",
				Port: 8080,
				ResourceLimits: &TemplateResourceLimits{
					MaxConcurrentRequests: 500,
					TimeoutSeconds:        15,
					MaxMemoryMB:           4096,
					MaxProcesses:          10,
				},
				Features: map[string]bool{
					"enable_diagnostics":        false,
					"enable_code_lenses":        false,
					"enable_hover_info":         false,
					"enable_debug_logging":      false,
					"enable_performance_monitoring": true,
					"enable_circuit_breaker":    true,
				},
			},
		},
		CreatedAt: time.Now(),
	}
}

// SelectTemplate selects the most appropriate template based on project analysis
func (tm *ConfigurationTemplateManager) SelectTemplate(analysis *ProjectAnalysis) (*ConfigurationTemplate, error) {
	var bestTemplate *ConfigurationTemplate
	var bestScore float64
	
	for _, template := range tm.templates {
		score := tm.calculateTemplateScore(template, analysis)
		if score > bestScore {
			bestScore = score
			bestTemplate = template
		}
	}
	
	if bestTemplate == nil {
		return nil, fmt.Errorf("no suitable template found for project")
	}
	
	tm.logger.WithFields(map[string]interface{}{
		"template":     bestTemplate.Name,
		"score":        bestScore,
		"project_type": analysis.ProjectType,
		"languages":    len(analysis.DetectedLanguages),
	}).Debug("Selected configuration template")
	
	return bestTemplate, nil
}

// calculateTemplateScore calculates how well a template matches the project analysis
func (tm *ConfigurationTemplateManager) calculateTemplateScore(template *ConfigurationTemplate, analysis *ProjectAnalysis) float64 {
	var score float64
	
	// Project type matching (40% weight)
	for _, projectType := range template.ProjectTypes {
		if projectType == analysis.ProjectType {
			score += 40.0
			break
		}
	}
	
	// Language matching (35% weight)
	languageMatches := 0
	for _, detectedLang := range analysis.DetectedLanguages {
		for _, templateLang := range template.TargetLanguages {
			if detectedLang == templateLang {
				languageMatches++
				break
			}
		}
	}
	
	if len(analysis.DetectedLanguages) > 0 {
		languageScore := float64(languageMatches) / float64(len(analysis.DetectedLanguages)) * 35.0
		score += languageScore
	}
	
	// Framework matching (15% weight)
	frameworkMatches := 0
	for _, framework := range analysis.Frameworks {
		if _, exists := template.FrameworkConfigs[strings.ToLower(framework.Name)]; exists {
			frameworkMatches++
		}
	}
	
	if len(analysis.Frameworks) > 0 {
		frameworkScore := float64(frameworkMatches) / float64(len(analysis.Frameworks)) * 15.0
		score += frameworkScore
	}
	
	// Complexity matching (10% weight)
	complexityScore := tm.calculateComplexityScore(template, analysis.Complexity)
	score += complexityScore * 10.0
	
	return score
}

// calculateComplexityScore calculates complexity matching score
func (tm *ConfigurationTemplateManager) calculateComplexityScore(template *ConfigurationTemplate, complexity string) float64 {
	if template.ResourceLimits == nil {
		return 0.5 // Neutral score if no resource limits defined
	}
	
	switch complexity {
	case ProjectComplexityHigh:
		// High complexity projects need high resource limits
		if template.ResourceLimits.MaxConcurrentRequests >= 300 {
			return 1.0
		} else if template.ResourceLimits.MaxConcurrentRequests >= 200 {
			return 0.7
		}
		return 0.3
		
	case ProjectComplexityMedium:
		// Medium complexity projects need moderate resource limits
		if template.ResourceLimits.MaxConcurrentRequests >= 150 && template.ResourceLimits.MaxConcurrentRequests <= 300 {
			return 1.0
		} else if template.ResourceLimits.MaxConcurrentRequests >= 100 {
			return 0.7
		}
		return 0.4
		
	case ProjectComplexityLow:
		// Low complexity projects need low resource limits
		if template.ResourceLimits.MaxConcurrentRequests <= 150 {
			return 1.0
		} else if template.ResourceLimits.MaxConcurrentRequests <= 200 {
			return 0.7
		}
		return 0.3
		
	default:
		return 0.5
	}
}

// GetTemplate retrieves a template by name
func (tm *ConfigurationTemplateManager) GetTemplate(name string) (*ConfigurationTemplate, error) {
	template, exists := tm.templates[strings.ToLower(name)]
	if !exists {
		return nil, fmt.Errorf("template not found: %s", name)
	}
	return template, nil
}

// GetDefaultTemplate returns the default template
func (tm *ConfigurationTemplateManager) GetDefaultTemplate() *ConfigurationTemplate {
	if template, exists := tm.templates["single-language"]; exists {
		return template
	}
	
	// Fallback template if default is not found
	return &ConfigurationTemplate{
		Name:               "fallback",
		Version:            "1.0",
		Description:        "Fallback configuration template",
		ProjectTypes:       []string{config.ProjectTypeSingle},
		TargetLanguages:    []string{"go"},
		DefaultPort:        8080,
		EnableMultiServer:  false,
		EnableSmartRouting: false,
		ResourceLimits: &TemplateResourceLimits{
			MaxConcurrentRequests: 100,
			TimeoutSeconds:        30,
			MaxMemoryMB:           1024,
			MaxProcesses:          3,
		},
		CreatedAt: time.Now(),
	}
}

// GetEnvironmentTemplate returns a template optimized for a specific environment
func (tm *ConfigurationTemplateManager) GetEnvironmentTemplate(environment string) *ConfigurationTemplate {
	envName := strings.ToLower(environment)
	
	// Try to find environment-specific template
	if template, exists := tm.templates[envName]; exists {
		return template
	}
	
	// Map common environment names to templates
	environmentMap := map[string]string{
		"prod":        "production",
		"production":  "production",
		"dev":         "development",
		"development": "development",
		"test":        "development",
		"testing":     "development",
		"qa":          "development",
		"stage":       "production",
		"staging":     "production",
	}
	
	if templateName, exists := environmentMap[envName]; exists {
		if template, exists := tm.templates[templateName]; exists {
			return template
		}
	}
	
	return nil
}

// GetAvailableTemplates returns all available template names
func (tm *ConfigurationTemplateManager) GetAvailableTemplates() []string {
	var names []string
	for name := range tm.templates {
		names = append(names, name)
	}
	return names
}

// AddTemplate adds a custom template
func (tm *ConfigurationTemplateManager) AddTemplate(template *ConfigurationTemplate) error {
	if template.Name == "" {
		return fmt.Errorf("template name cannot be empty")
	}
	
	if template.Version == "" {
		template.Version = "1.0"
	}
	
	if template.CreatedAt.IsZero() {
		template.CreatedAt = time.Now()
	}
	
	tm.templates[strings.ToLower(template.Name)] = template
	
	tm.logger.WithField("template", template.Name).Info("Added custom configuration template")
	
	return nil
}

// RemoveTemplate removes a template
func (tm *ConfigurationTemplateManager) RemoveTemplate(name string) error {
	templateName := strings.ToLower(name)
	
	if _, exists := tm.templates[templateName]; !exists {
		return fmt.Errorf("template not found: %s", name)
	}
	
	delete(tm.templates, templateName)
	
	tm.logger.WithField("template", name).Info("Removed configuration template")
	
	return nil
}

// ValidateTemplate validates a template configuration
func (tm *ConfigurationTemplateManager) ValidateTemplate(template *ConfigurationTemplate) error {
	if template.Name == "" {
		return fmt.Errorf("template name is required")
	}
	
	if len(template.ProjectTypes) == 0 {
		return fmt.Errorf("template must specify at least one project type")
	}
	
	if len(template.TargetLanguages) == 0 {
		return fmt.Errorf("template must specify at least one target language")
	}
	
	if template.DefaultPort <= 0 || template.DefaultPort > 65535 {
		return fmt.Errorf("invalid default port: %d", template.DefaultPort)
	}
	
	if template.ResourceLimits != nil {
		if template.ResourceLimits.MaxConcurrentRequests <= 0 {
			return fmt.Errorf("max concurrent requests must be positive")
		}
		
		if template.ResourceLimits.TimeoutSeconds <= 0 {
			return fmt.Errorf("timeout seconds must be positive")
		}
		
		if template.ResourceLimits.MaxMemoryMB <= 0 {
			return fmt.Errorf("max memory MB must be positive")
		}
	}
	
	return nil
}

// SetLogger sets the logger for the template manager
func (tm *ConfigurationTemplateManager) SetLogger(logger *SetupLogger) {
	if logger != nil {
		tm.logger = logger
	}
}