package setup

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// TemplateValidator interface defines methods for validating YAML templates
type TemplateValidator interface {
	ValidateTemplate(template *EnhancedConfigurationTemplate) *TemplateValidationResult
	ValidateTemplateFile(path string) *TemplateValidationResult
	ValidateMultipleTemplates(templates map[string]*EnhancedConfigurationTemplate) *MultiTemplateValidationResult
	SetValidationRules(rules *ValidationRules)
}

// EnhancedTemplateValidator provides comprehensive validation for YAML templates
type EnhancedTemplateValidator struct {
	rules  *ValidationRules
	logger *SetupLogger
	loader YAMLTemplateLoader
}

// TemplateValidationResult represents the result of template validation
type TemplateValidationResult struct {
	IsValid      bool                    `json:"is_valid"`
	TemplateName string                  `json:"template_name"`
	Errors       []TemplateValidationError       `json:"errors,omitempty"`
	Warnings     []ValidationWarning     `json:"warnings,omitempty"`
	Suggestions  []ValidationSuggestion  `json:"suggestions,omitempty"`
	Score        int                     `json:"score"` // 0-100 quality score
	Components   map[string]ComponentValidation `json:"components"`
}

// MultiTemplateValidationResult represents validation results for multiple templates
type MultiTemplateValidationResult struct {
	OverallValid    bool                                  `json:"overall_valid"`
	TotalTemplates  int                                   `json:"total_templates"`
	ValidTemplates  int                                   `json:"valid_templates"`
	TemplateResults map[string]*TemplateValidationResult  `json:"template_results"`
	CrossTemplateIssues []CrossTemplateIssue              `json:"cross_template_issues,omitempty"`
}

// TemplateValidationError represents a template validation error
type TemplateValidationError struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	Field       string `json:"field,omitempty"`
	Severity    string `json:"severity"`
	Suggestion  string `json:"suggestion,omitempty"`
}

// ValidationWarning represents a validation warning
type ValidationWarning struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	Field       string `json:"field,omitempty"`
	Recommendation string `json:"recommendation,omitempty"`
}

// ValidationSuggestion represents a validation suggestion
type ValidationSuggestion struct {
	Category    string `json:"category"`
	Message     string `json:"message"`
	Impact      string `json:"impact"`
	Priority    string `json:"priority"`
}

// ComponentValidation represents validation for a specific component
type ComponentValidation struct {
	Name     string   `json:"name"`
	IsValid  bool     `json:"is_valid"`
	Score    int      `json:"score"`
	Issues   []string `json:"issues,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// CrossTemplateIssue represents issues across multiple templates
type CrossTemplateIssue struct {
	Type        string   `json:"type"`
	Message     string   `json:"message"`
	Templates   []string `json:"templates"`
	Severity    string   `json:"severity"`
	Suggestion  string   `json:"suggestion"`
}

// ValidationRules defines the rules used for template validation
type ValidationRules struct {
	// Port validation
	MinPort int `json:"min_port"`
	MaxPort int `json:"max_port"`
	
	// Resource limits
	MaxMemoryMB           int `json:"max_memory_mb"`
	MaxConcurrentRequests int `json:"max_concurrent_requests"`
	MaxProcesses          int `json:"max_processes"`
	
	// Timeout validation
	MinTimeoutSeconds int `json:"min_timeout_seconds"`
	MaxTimeoutSeconds int `json:"max_timeout_seconds"`
	
	// Server validation
	RequiredServerFields  []string `json:"required_server_fields"`
	SupportedTransports   []string `json:"supported_transports"`
	SupportedLanguages    []string `json:"supported_languages"`
	
	// Pool validation
	MinPoolSize           int `json:"min_pool_size"`
	MaxPoolSize           int `json:"max_pool_size"`
	
	// Naming conventions
	NamePattern           string `json:"name_pattern"`
	ServerNamePattern     string `json:"server_name_pattern"`
	
	// Advanced validation flags
	StrictMode            bool `json:"strict_mode"`
	WarnOnDeprecated      bool `json:"warn_on_deprecated"`
	ValidatePerformance   bool `json:"validate_performance"`
	ValidateSecurity      bool `json:"validate_security"`
}

// NewEnhancedTemplateValidator creates a new enhanced template validator
func NewEnhancedTemplateValidator(loader YAMLTemplateLoader) *EnhancedTemplateValidator {
	return &EnhancedTemplateValidator{
		rules:  DefaultValidationRules(),
		logger: NewSetupLogger(nil),
		loader: loader,
	}
}

// DefaultValidationRules returns default validation rules
func DefaultValidationRules() *ValidationRules {
	return &ValidationRules{
		MinPort:               1024,
		MaxPort:               65535,
		MaxMemoryMB:           16384, // 16GB max
		MaxConcurrentRequests: 10000,
		MaxProcesses:          100,
		MinTimeoutSeconds:     1,
		MaxTimeoutSeconds:     300,  // 5 minutes max
		RequiredServerFields:  []string{"name", "command", "languages"},
		SupportedTransports:   []string{"stdio", "tcp", "websocket"},
		SupportedLanguages:    []string{"go", "python", "typescript", "javascript", "java", "rust", "cpp", "c"},
		MinPoolSize:           1,
		MaxPoolSize:           50,
		NamePattern:           `^[a-zA-Z][a-zA-Z0-9_-]*$`,
		ServerNamePattern:     `^[a-zA-Z][a-zA-Z0-9_-]*$`,
		StrictMode:            false,
		WarnOnDeprecated:      true,
		ValidatePerformance:   true,
		ValidateSecurity:      true,
	}
}

// ValidateTemplate validates a single template
func (validator *EnhancedTemplateValidator) ValidateTemplate(template *EnhancedConfigurationTemplate) *TemplateValidationResult {
	result := &TemplateValidationResult{
		TemplateName: template.Name,
		IsValid:      true,
		Errors:       []TemplateValidationError{},
		Warnings:     []ValidationWarning{},
		Suggestions:  []ValidationSuggestion{},
		Components:   make(map[string]ComponentValidation),
		Score:        100,
	}

	// Validate basic template structure
	validator.validateBasicStructure(template, result)
	
	// Validate gateway configuration
	validator.validateGatewayConfig(template, result)
	
	// Validate language pools
	validator.validateLanguagePools(template, result)
	
	// Validate server instances
	validator.validateServerInstances(template, result)
	
	// Validate multi-server configuration
	validator.validateMultiServerConfig(template, result)
	
	// Validate routing configuration
	validator.validateRoutingConfig(template, result)
	
	// Validate performance configuration
	validator.validatePerformanceConfig(template, result)
	
	// Validate pool management
	validator.validatePoolManagement(template, result)
	
	// Validate naming conventions
	validator.validateNamingConventions(template, result)
	
	// Validate cross-references
	validator.validateCrossReferences(template, result)
	
	// Validate security aspects
	if validator.rules.ValidateSecurity {
		validator.validateSecurity(template, result)
	}
	
	// Validate performance aspects
	if validator.rules.ValidatePerformance {
		validator.validatePerformanceAspects(template, result)
	}
	
	// Calculate final validity and score
	validator.calculateFinalScore(result)
	
	return result
}

// ValidateTemplateFile validates a template file using the loader
func (validator *EnhancedTemplateValidator) ValidateTemplateFile(path string) *TemplateValidationResult {
	if validator.loader == nil {
		return &TemplateValidationResult{
			IsValid: false,
			Errors: []TemplateValidationError{{
				Code:     "LOADER_MISSING",
				Message:  "Template loader not available",
				Severity: "error",
			}},
		}
	}

	template, err := validator.loader.LoadTemplate(path)
	if err != nil {
		return &TemplateValidationResult{
			IsValid: false,
			Errors: []TemplateValidationError{{
				Code:     "LOAD_ERROR",
				Message:  fmt.Sprintf("Failed to load template: %v", err),
				Severity: "error",
			}},
		}
	}

	return validator.ValidateTemplate(template)
}

// ValidateMultipleTemplates validates multiple templates and checks for conflicts
func (validator *EnhancedTemplateValidator) ValidateMultipleTemplates(templates map[string]*EnhancedConfigurationTemplate) *MultiTemplateValidationResult {
	result := &MultiTemplateValidationResult{
		TotalTemplates:      len(templates),
		TemplateResults:     make(map[string]*TemplateValidationResult),
		CrossTemplateIssues: []CrossTemplateIssue{},
	}

	validCount := 0

	// Validate each template individually
	for name, template := range templates {
		templateResult := validator.ValidateTemplate(template)
		result.TemplateResults[name] = templateResult
		
		if templateResult.IsValid {
			validCount++
		}
	}

	result.ValidTemplates = validCount
	result.OverallValid = validCount == result.TotalTemplates

	// Check for cross-template issues
	validator.validateCrossTemplateCompatibility(templates, result)

	return result
}

// SetValidationRules sets custom validation rules
func (validator *EnhancedTemplateValidator) SetValidationRules(rules *ValidationRules) {
	if rules != nil {
		validator.rules = rules
	}
}

// SetLogger sets the logger for the validator
func (validator *EnhancedTemplateValidator) SetLogger(logger *SetupLogger) {
	if logger != nil {
		validator.logger = logger
	}
}

// validateBasicStructure validates the basic template structure
func (validator *EnhancedTemplateValidator) validateBasicStructure(template *EnhancedConfigurationTemplate, result *TemplateValidationResult) {
	component := ComponentValidation{
		Name:    "basic_structure",
		IsValid: true,
		Score:   100,
	}

	// Validate required fields
	if template.Name == "" {
		validator.addError(result, "MISSING_NAME", "Template name is required", "name", "Set a descriptive name for the template")
		component.IsValid = false
		component.Issues = append(component.Issues, "Missing template name")
	}

	if template.Version == "" {
		validator.addWarning(result, "MISSING_VERSION", "Template version is recommended", "version", "Add a version field for template versioning")
		component.Warnings = append(component.Warnings, "Missing version")
		component.Score -= 5
	}

	if template.Description == "" {
		validator.addWarning(result, "MISSING_DESCRIPTION", "Template description is recommended", "description", "Add a description explaining the template's purpose")
		component.Warnings = append(component.Warnings, "Missing description")
		component.Score -= 5
	}

	// Validate naming convention
	if template.Name != "" && validator.rules.NamePattern != "" {
		if matched, _ := regexp.MatchString(validator.rules.NamePattern, template.Name); !matched {
			validator.addError(result, "INVALID_NAME_PATTERN", fmt.Sprintf("Template name '%s' doesn't match pattern %s", template.Name, validator.rules.NamePattern), "name", "Use alphanumeric characters, hyphens, and underscores")
			component.IsValid = false
			component.Issues = append(component.Issues, "Invalid name pattern")
		}
	}

	result.Components["basic_structure"] = component
}

// validateGatewayConfig validates gateway configuration
func (validator *EnhancedTemplateValidator) validateGatewayConfig(template *EnhancedConfigurationTemplate, result *TemplateValidationResult) {
	component := ComponentValidation{
		Name:    "gateway_config",
		IsValid: true,
		Score:   100,
	}

	// Validate port
	if template.Port < validator.rules.MinPort || template.Port > validator.rules.MaxPort {
		validator.addError(result, "INVALID_PORT", fmt.Sprintf("Port %d is outside valid range %d-%d", template.Port, validator.rules.MinPort, validator.rules.MaxPort), "port", fmt.Sprintf("Use a port between %d and %d", validator.rules.MinPort, validator.rules.MaxPort))
		component.IsValid = false
		component.Issues = append(component.Issues, "Invalid port range")
	}

	// Validate timeout format
	if template.Timeout != "" {
		if _, err := time.ParseDuration(template.Timeout); err != nil {
			validator.addError(result, "INVALID_TIMEOUT_FORMAT", fmt.Sprintf("Invalid timeout format '%s'", template.Timeout), "timeout", "Use format like '30s', '5m', or '1h'")
			component.IsValid = false
			component.Issues = append(component.Issues, "Invalid timeout format")
		}
	}

	// Validate concurrent requests
	if template.MaxConcurrentRequests > validator.rules.MaxConcurrentRequests {
		validator.addWarning(result, "HIGH_CONCURRENT_REQUESTS", fmt.Sprintf("Max concurrent requests %d is very high", template.MaxConcurrentRequests), "max_concurrent_requests", "Consider if such high concurrency is needed")
		component.Warnings = append(component.Warnings, "High concurrent requests")
		component.Score -= 10
	}

	result.Components["gateway_config"] = component
}

// validateLanguagePools validates language pool configurations
func (validator *EnhancedTemplateValidator) validateLanguagePools(template *EnhancedConfigurationTemplate, result *TemplateValidationResult) {
	component := ComponentValidation{
		Name:    "language_pools",
		IsValid: true,
		Score:   100,
	}

	// Check if any language pools exist
	if len(template.LanguagePools) == 0 && len(template.Servers) == 0 {
		validator.addWarning(result, "NO_LANGUAGE_POOLS", "No language pools or servers defined", "language_pools", "Define at least one language pool or server")
		component.Warnings = append(component.Warnings, "No language pools defined")
		component.Score -= 20
	}

	// Validate each language pool
	for i, pool := range template.LanguagePools {
		poolField := fmt.Sprintf("language_pools[%d]", i)
		
		// Validate language
		if pool.Language == "" {
			validator.addError(result, "MISSING_POOL_LANGUAGE", fmt.Sprintf("Language pool %d missing language", i), poolField+".language", "Specify the programming language for this pool")
			component.IsValid = false
			component.Issues = append(component.Issues, fmt.Sprintf("Pool %d missing language", i))
		} else if !validator.isSupportedLanguage(pool.Language) {
			validator.addWarning(result, "UNSUPPORTED_LANGUAGE", fmt.Sprintf("Language '%s' may not be supported", pool.Language), poolField+".language", "Verify language server availability")
			component.Warnings = append(component.Warnings, fmt.Sprintf("Unsupported language: %s", pool.Language))
			component.Score -= 5
		}

		// Validate default server
		if pool.DefaultServer == "" {
			validator.addError(result, "MISSING_DEFAULT_SERVER", fmt.Sprintf("Language pool '%s' missing default_server", pool.Language), poolField+".default_server", "Specify which server should be used by default")
			component.IsValid = false
			component.Issues = append(component.Issues, fmt.Sprintf("Pool %s missing default server", pool.Language))
		} else if _, exists := pool.Servers[pool.DefaultServer]; !exists {
			validator.addError(result, "INVALID_DEFAULT_SERVER", fmt.Sprintf("Default server '%s' not found in servers map for language '%s'", pool.DefaultServer, pool.Language), poolField+".default_server", "Ensure the default server exists in the servers map")
			component.IsValid = false
			component.Issues = append(component.Issues, fmt.Sprintf("Invalid default server: %s", pool.DefaultServer))
		}

		// Validate servers in pool
		if len(pool.Servers) == 0 {
			validator.addError(result, "EMPTY_POOL_SERVERS", fmt.Sprintf("Language pool '%s' has no servers", pool.Language), poolField+".servers", "Define at least one server for this language")
			component.IsValid = false
			component.Issues = append(component.Issues, fmt.Sprintf("Pool %s has no servers", pool.Language))
		}

		// Validate resource limits
		if pool.ResourceLimits != nil {
			validator.validateResourceLimits(pool.ResourceLimits, poolField+".resource_limits", result, &component)
		}
	}

	result.Components["language_pools"] = component
}

// validateServerInstances validates server instance configurations
func (validator *EnhancedTemplateValidator) validateServerInstances(template *EnhancedConfigurationTemplate, result *TemplateValidationResult) {
	component := ComponentValidation{
		Name:    "server_instances",
		IsValid: true,
		Score:   100,
	}

	serverNames := make(map[string]bool)

	for i, server := range template.Servers {
		serverField := fmt.Sprintf("servers[%d]", i)
		
		// Validate required fields
		if server.Name == "" {
			validator.addError(result, "MISSING_SERVER_NAME", fmt.Sprintf("Server %d missing name", i), serverField+".name", "Provide a unique name for the server")
			component.IsValid = false
			component.Issues = append(component.Issues, fmt.Sprintf("Server %d missing name", i))
		} else {
			// Check for duplicate names
			if serverNames[server.Name] {
				validator.addError(result, "DUPLICATE_SERVER_NAME", fmt.Sprintf("Duplicate server name '%s'", server.Name), serverField+".name", "Server names must be unique")
				component.IsValid = false
				component.Issues = append(component.Issues, fmt.Sprintf("Duplicate server name: %s", server.Name))
			}
			serverNames[server.Name] = true
			
			// Validate naming convention
			if validator.rules.ServerNamePattern != "" {
				if matched, _ := regexp.MatchString(validator.rules.ServerNamePattern, server.Name); !matched {
					validator.addWarning(result, "INVALID_SERVER_NAME_PATTERN", fmt.Sprintf("Server name '%s' doesn't match recommended pattern", server.Name), serverField+".name", "Use descriptive, consistent naming")
					component.Warnings = append(component.Warnings, fmt.Sprintf("Server name pattern: %s", server.Name))
					component.Score -= 5
				}
			}
		}

		if server.Command == "" {
			validator.addError(result, "MISSING_SERVER_COMMAND", fmt.Sprintf("Server '%s' missing command", server.Name), serverField+".command", "Specify the command to start the language server")
			component.IsValid = false
			component.Issues = append(component.Issues, fmt.Sprintf("Server %s missing command", server.Name))
		}

		if len(server.Languages) == 0 {
			validator.addError(result, "MISSING_SERVER_LANGUAGES", fmt.Sprintf("Server '%s' has no languages specified", server.Name), serverField+".languages", "Specify which languages this server supports")
			component.IsValid = false
			component.Issues = append(component.Issues, fmt.Sprintf("Server %s missing languages", server.Name))
		} else {
			// Validate supported languages
			for _, lang := range server.Languages {
				if !validator.isSupportedLanguage(lang) {
					validator.addWarning(result, "UNSUPPORTED_SERVER_LANGUAGE", fmt.Sprintf("Server '%s' uses potentially unsupported language '%s'", server.Name, lang), serverField+".languages", "Verify language server support")
					component.Warnings = append(component.Warnings, fmt.Sprintf("Unsupported language: %s", lang))
					component.Score -= 3
				}
			}
		}

		// Validate transport
		if server.Transport != "" && !validator.isSupportedTransport(server.Transport) {
			validator.addWarning(result, "UNSUPPORTED_TRANSPORT", fmt.Sprintf("Server '%s' uses potentially unsupported transport '%s'", server.Name, server.Transport), serverField+".transport", "Use supported transports: stdio, tcp, websocket")
			component.Warnings = append(component.Warnings, fmt.Sprintf("Unsupported transport: %s", server.Transport))
			component.Score -= 5
		}

		// Validate pool configuration
		if server.PoolConfig != nil {
			validator.validatePoolConfig(server.PoolConfig, serverField+".pool_config", result, &component)
		}

		// Validate health check settings
		if server.HealthCheckSettings != nil {
			validator.validateHealthCheckSettings(server.HealthCheckSettings, serverField+".health_check_settings", result, &component)
		}
	}

	result.Components["server_instances"] = component
}

// validateMultiServerConfig validates multi-server configuration
func (validator *EnhancedTemplateValidator) validateMultiServerConfig(template *EnhancedConfigurationTemplate, result *TemplateValidationResult) {
	if template.MultiServerConfig == nil {
		return
	}

	component := ComponentValidation{
		Name:    "multi_server_config",
		IsValid: true,
		Score:   100,
	}

	config := template.MultiServerConfig

	// Validate selection strategy
	validStrategies := []string{"intelligent_routing", "load_balance", "round_robin", "random", "performance"}
	if config.SelectionStrategy != "" && !validator.containsString(validStrategies, config.SelectionStrategy) {
		validator.addWarning(result, "UNKNOWN_SELECTION_STRATEGY", fmt.Sprintf("Unknown selection strategy '%s'", config.SelectionStrategy), "multi_server_config.selection_strategy", "Use a known strategy")
		component.Warnings = append(component.Warnings, "Unknown selection strategy")
		component.Score -= 10
	}

	// Validate concurrent limit
	if config.ConcurrentLimit > validator.rules.MaxConcurrentRequests {
		validator.addWarning(result, "HIGH_CONCURRENT_LIMIT", fmt.Sprintf("Concurrent limit %d is very high", config.ConcurrentLimit), "multi_server_config.concurrent_limit", "Consider if such high concurrency is needed")
		component.Warnings = append(component.Warnings, "High concurrent limit")
		component.Score -= 5
	}

	// Validate intervals
	if config.HealthCheckInterval != "" {
		if _, err := time.ParseDuration(config.HealthCheckInterval); err != nil {
			validator.addError(result, "INVALID_HEALTH_CHECK_INTERVAL", fmt.Sprintf("Invalid health check interval '%s'", config.HealthCheckInterval), "multi_server_config.health_check_interval", "Use duration format like '30s' or '1m'")
			component.IsValid = false
			component.Issues = append(component.Issues, "Invalid health check interval")
		}
	}

	if config.CacheTTL != "" {
		if _, err := time.ParseDuration(config.CacheTTL); err != nil {
			validator.addError(result, "INVALID_CACHE_TTL", fmt.Sprintf("Invalid cache TTL '%s'", config.CacheTTL), "multi_server_config.cache_ttl", "Use duration format like '10m' or '1h'")
			component.IsValid = false
			component.Issues = append(component.Issues, "Invalid cache TTL")
		}
	}

	result.Components["multi_server_config"] = component
}

// validateRoutingConfig validates routing configuration
func (validator *EnhancedTemplateValidator) validateRoutingConfig(template *EnhancedConfigurationTemplate, result *TemplateValidationResult) {
	component := ComponentValidation{
		Name:    "routing_config",
		IsValid: true,
		Score:   100,
	}

	// Validate smart router config
	if template.SmartRouterConfig != nil {
		config := template.SmartRouterConfig
		
		// Validate method strategies
		if config.MethodStrategies != nil {
			validMethods := []string{
				"textDocument/definition", "textDocument/references", "textDocument/hover",
				"textDocument/completion", "textDocument/documentSymbol", "workspace/symbol",
			}
			
			for method := range config.MethodStrategies {
				if !validator.containsString(validMethods, method) {
					validator.addWarning(result, "UNKNOWN_LSP_METHOD", fmt.Sprintf("Unknown LSP method '%s' in routing", method), "smart_router_config.method_strategies", "Use standard LSP methods")
					component.Warnings = append(component.Warnings, fmt.Sprintf("Unknown LSP method: %s", method))
					component.Score -= 5
				}
			}
		}

		// Validate timeouts
		if config.CircuitBreakerTimeout != "" {
			if _, err := time.ParseDuration(config.CircuitBreakerTimeout); err != nil {
				validator.addError(result, "INVALID_CIRCUIT_BREAKER_TIMEOUT", fmt.Sprintf("Invalid circuit breaker timeout '%s'", config.CircuitBreakerTimeout), "smart_router_config.circuit_breaker_timeout", "Use duration format")
				component.IsValid = false
				component.Issues = append(component.Issues, "Invalid circuit breaker timeout")
			}
		}
	}

	// Validate routing config
	if template.Routing != nil {
		if template.Routing.CacheTTL != "" {
			if _, err := time.ParseDuration(template.Routing.CacheTTL); err != nil {
				validator.addError(result, "INVALID_ROUTING_CACHE_TTL", fmt.Sprintf("Invalid routing cache TTL '%s'", template.Routing.CacheTTL), "routing.cache_ttl", "Use duration format")
				component.IsValid = false
				component.Issues = append(component.Issues, "Invalid routing cache TTL")
			}
		}
	}

	result.Components["routing_config"] = component
}

// validatePerformanceConfig validates performance configuration including SCIP
func (validator *EnhancedTemplateValidator) validatePerformanceConfig(template *EnhancedConfigurationTemplate, result *TemplateValidationResult) {
	if template.PerformanceConfig == nil {
		return
	}

	component := ComponentValidation{
		Name:    "performance_config",
		IsValid: true,
		Score:   100,
	}

	// Validate SCIP configuration
	if template.PerformanceConfig.SCIP != nil {
		scip := template.PerformanceConfig.SCIP
		
		// Validate refresh interval
		if scip.RefreshInterval != "" {
			if _, err := time.ParseDuration(scip.RefreshInterval); err != nil {
				validator.addError(result, "INVALID_SCIP_REFRESH_INTERVAL", fmt.Sprintf("Invalid SCIP refresh interval '%s'", scip.RefreshInterval), "performance_config.scip.refresh_interval", "Use duration format")
				component.IsValid = false
				component.Issues = append(component.Issues, "Invalid SCIP refresh interval")
			}
		}

		// Validate fallback timeout
		if scip.FallbackTimeout != "" {
			if _, err := time.ParseDuration(scip.FallbackTimeout); err != nil {
				validator.addError(result, "INVALID_SCIP_FALLBACK_TIMEOUT", fmt.Sprintf("Invalid SCIP fallback timeout '%s'", scip.FallbackTimeout), "performance_config.scip.fallback_timeout", "Use duration format")
				component.IsValid = false
				component.Issues = append(component.Issues, "Invalid SCIP fallback timeout")
			}
		}

		// Validate cache configuration
		if scip.Cache != nil && scip.Cache.TTL != "" {
			if _, err := time.ParseDuration(scip.Cache.TTL); err != nil {
				validator.addError(result, "INVALID_SCIP_CACHE_TTL", fmt.Sprintf("Invalid SCIP cache TTL '%s'", scip.Cache.TTL), "performance_config.scip.cache.ttl", "Use duration format")
				component.IsValid = false
				component.Issues = append(component.Issues, "Invalid SCIP cache TTL")
			}
		}

		// Validate language settings
		if scip.LanguageSettings != nil {
			for lang, langConfig := range scip.LanguageSettings {
				if !validator.isSupportedLanguage(lang) {
					validator.addWarning(result, "UNSUPPORTED_SCIP_LANGUAGE", fmt.Sprintf("SCIP language '%s' may not be supported", lang), fmt.Sprintf("performance_config.scip.language_settings.%s", lang), "Verify SCIP indexer availability")
					component.Warnings = append(component.Warnings, fmt.Sprintf("Unsupported SCIP language: %s", lang))
					component.Score -= 5
				}

				// Validate timeout format
				if langConfig.IndexTimeout != "" {
					if _, err := time.ParseDuration(langConfig.IndexTimeout); err != nil {
						validator.addError(result, "INVALID_INDEX_TIMEOUT", fmt.Sprintf("Invalid index timeout '%s' for language '%s'", langConfig.IndexTimeout, lang), fmt.Sprintf("performance_config.scip.language_settings.%s.index_timeout", lang), "Use duration format")
						component.IsValid = false
						component.Issues = append(component.Issues, fmt.Sprintf("Invalid index timeout for %s", lang))
					}
				}
			}
		}
	}

	result.Components["performance_config"] = component
}

// validatePoolManagement validates pool management configuration
func (validator *EnhancedTemplateValidator) validatePoolManagement(template *EnhancedConfigurationTemplate, result *TemplateValidationResult) {
	if template.PoolManagement == nil {
		return
	}

	component := ComponentValidation{
		Name:    "pool_management",
		IsValid: true,
		Score:   100,
	}

	pm := template.PoolManagement

	// Validate intervals
	if pm.MonitoringInterval != "" {
		if _, err := time.ParseDuration(pm.MonitoringInterval); err != nil {
			validator.addError(result, "INVALID_MONITORING_INTERVAL", fmt.Sprintf("Invalid monitoring interval '%s'", pm.MonitoringInterval), "pool_management.monitoring_interval", "Use duration format")
			component.IsValid = false
			component.Issues = append(component.Issues, "Invalid monitoring interval")
		}
	}

	if pm.CleanupInterval != "" {
		if _, err := time.ParseDuration(pm.CleanupInterval); err != nil {
			validator.addError(result, "INVALID_CLEANUP_INTERVAL", fmt.Sprintf("Invalid cleanup interval '%s'", pm.CleanupInterval), "pool_management.cleanup_interval", "Use duration format")
			component.IsValid = false
			component.Issues = append(component.Issues, "Invalid cleanup interval")
		}
	}

	if pm.MetricsRetention != "" {
		if _, err := time.ParseDuration(pm.MetricsRetention); err != nil {
			validator.addError(result, "INVALID_METRICS_RETENTION", fmt.Sprintf("Invalid metrics retention '%s'", pm.MetricsRetention), "pool_management.metrics_retention", "Use duration format")
			component.IsValid = false
			component.Issues = append(component.Issues, "Invalid metrics retention")
		}
	}

	// Validate resource limits
	if pm.MaxTotalMemoryMB > validator.rules.MaxMemoryMB {
		validator.addWarning(result, "HIGH_TOTAL_MEMORY", fmt.Sprintf("Total memory limit %d MB is very high", pm.MaxTotalMemoryMB), "pool_management.max_total_memory_mb", "Consider if such high memory usage is needed")
		component.Warnings = append(component.Warnings, "High total memory limit")
		component.Score -= 5
	}

	result.Components["pool_management"] = component
}

// validateNamingConventions validates naming conventions
func (validator *EnhancedTemplateValidator) validateNamingConventions(template *EnhancedConfigurationTemplate, result *TemplateValidationResult) {
	component := ComponentValidation{
		Name:    "naming_conventions",
		IsValid: true,
		Score:   100,
	}

	// Check consistency in naming patterns
	hasInconsistentNaming := false

	// Check server naming consistency
	serverPrefixes := make(map[string]int)
	for _, server := range template.Servers {
		if server.Name != "" {
			parts := strings.Split(server.Name, "-")
			if len(parts) > 0 {
				prefix := parts[0]
				serverPrefixes[prefix]++
			}
		}
	}

	// Check language pool server naming
	for _, pool := range template.LanguagePools {
		for serverName := range pool.Servers {
			parts := strings.Split(serverName, "-")
			if len(parts) > 0 {
				prefix := parts[0]
				if !strings.Contains(serverName, pool.Language) && prefix != pool.Language {
					validator.addSuggestion(result, "naming", fmt.Sprintf("Consider including language '%s' in server name '%s'", pool.Language, serverName), "consistency", "medium")
					hasInconsistentNaming = true
				}
			}
		}
	}

	if hasInconsistentNaming {
		component.Warnings = append(component.Warnings, "Inconsistent naming conventions")
		component.Score -= 10
	}

	result.Components["naming_conventions"] = component
}

// validateCrossReferences validates cross-references within the template
func (validator *EnhancedTemplateValidator) validateCrossReferences(template *EnhancedConfigurationTemplate, result *TemplateValidationResult) {
	component := ComponentValidation{
		Name:    "cross_references",
		IsValid: true,
		Score:   100,
	}

	// Collect all server names from different sections
	serverNames := make(map[string]bool)
	
	// From servers section
	for _, server := range template.Servers {
		if server.Name != "" {
			serverNames[server.Name] = true
		}
	}

	// From language pools
	for _, pool := range template.LanguagePools {
		for serverName := range pool.Servers {
			serverNames[serverName] = true
		}
	}

	// Validate default server references
	for _, pool := range template.LanguagePools {
		if pool.DefaultServer != "" && !serverNames[pool.DefaultServer] {
			validator.addError(result, "INVALID_DEFAULT_SERVER_REF", fmt.Sprintf("Default server '%s' not found for language '%s'", pool.DefaultServer, pool.Language), fmt.Sprintf("language_pools[%s].default_server", pool.Language), "Ensure the default server exists")
			component.IsValid = false
			component.Issues = append(component.Issues, fmt.Sprintf("Invalid default server reference: %s", pool.DefaultServer))
		}
	}

	// Check for orphaned server definitions
	referencedServers := make(map[string]bool)
	for _, pool := range template.LanguagePools {
		if pool.DefaultServer != "" {
			referencedServers[pool.DefaultServer] = true
		}
		for serverName := range pool.Servers {
			referencedServers[serverName] = true
		}
	}

	for _, server := range template.Servers {
		if server.Name != "" && !referencedServers[server.Name] {
			validator.addWarning(result, "ORPHANED_SERVER", fmt.Sprintf("Server '%s' is defined but not referenced", server.Name), "servers", "Remove unused servers or add them to language pools")
			component.Warnings = append(component.Warnings, fmt.Sprintf("Orphaned server: %s", server.Name))
			component.Score -= 5
		}
	}

	result.Components["cross_references"] = component
}

// validateSecurity validates security aspects of the template
func (validator *EnhancedTemplateValidator) validateSecurity(template *EnhancedConfigurationTemplate, result *TemplateValidationResult) {
	component := ComponentValidation{
		Name:    "security",
		IsValid: true,
		Score:   100,
	}

	// Check for security-sensitive configurations
	if template.Port < 1024 {
		validator.addWarning(result, "PRIVILEGED_PORT", fmt.Sprintf("Port %d requires privileged access", template.Port), "port", "Consider using a non-privileged port (>= 1024)")
		component.Warnings = append(component.Warnings, "Privileged port usage")
		component.Score -= 10
	}

	// Check for overly permissive resource limits
	if template.MaxConcurrentRequests > 1000 {
		validator.addWarning(result, "HIGH_CONCURRENCY_RISK", fmt.Sprintf("High concurrent requests (%d) may pose DoS risk", template.MaxConcurrentRequests), "max_concurrent_requests", "Consider implementing rate limiting")
		component.Warnings = append(component.Warnings, "High concurrency risk")
		component.Score -= 5
	}

	// Check for insecure environment variables
	for _, server := range template.Servers {
		for envKey, envValue := range server.Environment {
			if validator.isSensitiveEnvVar(envKey, envValue) {
				validator.addWarning(result, "SENSITIVE_ENV_VAR", fmt.Sprintf("Server '%s' may contain sensitive environment variable '%s'", server.Name, envKey), fmt.Sprintf("servers[%s].environment.%s", server.Name, envKey), "Avoid hardcoding sensitive values")
				component.Warnings = append(component.Warnings, fmt.Sprintf("Sensitive env var in %s", server.Name))
				component.Score -= 15
			}
		}
	}

	result.Components["security"] = component
}

// validatePerformanceAspects validates performance aspects of the template
func (validator *EnhancedTemplateValidator) validatePerformanceAspects(template *EnhancedConfigurationTemplate, result *TemplateValidationResult) {
	component := ComponentValidation{
		Name:    "performance",
		IsValid: true,
		Score:   100,
	}

	// Check for performance anti-patterns
	totalServers := len(template.Servers)
	for _, pool := range template.LanguagePools {
		totalServers += len(pool.Servers)
	}

	if totalServers > 20 {
		validator.addWarning(result, "HIGH_SERVER_COUNT", fmt.Sprintf("High number of servers (%d) may impact performance", totalServers), "servers", "Consider consolidating servers or using dynamic scaling")
		component.Warnings = append(component.Warnings, "High server count")
		component.Score -= 10
	}

	// Check for inefficient configurations
	for _, server := range template.Servers {
		if server.PoolConfig != nil {
			pc := server.PoolConfig
			if pc.MinSize > 10 {
				validator.addWarning(result, "HIGH_MIN_POOL_SIZE", fmt.Sprintf("Server '%s' has high minimum pool size (%d)", server.Name, pc.MinSize), fmt.Sprintf("servers[%s].pool_config.min_size", server.Name), "Consider lower minimum pool size")
				component.Warnings = append(component.Warnings, fmt.Sprintf("High min pool size: %s", server.Name))
				component.Score -= 5
			}
		}
	}

	result.Components["performance"] = component
}

// validateCrossTemplateCompatibility validates compatibility across multiple templates
func (validator *EnhancedTemplateValidator) validateCrossTemplateCompatibility(templates map[string]*EnhancedConfigurationTemplate, result *MultiTemplateValidationResult) {
	// Check for port conflicts
	portUsage := make(map[int][]string)
	for name, template := range templates {
		if template.Port > 0 {
			portUsage[template.Port] = append(portUsage[template.Port], name)
		}
	}

	for port, templateNames := range portUsage {
		if len(templateNames) > 1 {
			result.CrossTemplateIssues = append(result.CrossTemplateIssues, CrossTemplateIssue{
				Type:       "PORT_CONFLICT",
				Message:    fmt.Sprintf("Port %d used by multiple templates", port),
				Templates:  templateNames,
				Severity:   "error",
				Suggestion: "Use different ports for each template",
			})
		}
	}

	// Check for naming conflicts
	serverNames := make(map[string][]string)
	for templateName, template := range templates {
		for _, server := range template.Servers {
			if server.Name != "" {
				serverNames[server.Name] = append(serverNames[server.Name], templateName)
			}
		}
	}

	for serverName, templateNames := range serverNames {
		if len(templateNames) > 1 {
			result.CrossTemplateIssues = append(result.CrossTemplateIssues, CrossTemplateIssue{
				Type:       "SERVER_NAME_CONFLICT",
				Message:    fmt.Sprintf("Server name '%s' used in multiple templates", serverName),
				Templates:  templateNames,
				Severity:   "warning",
				Suggestion: "Use unique server names across templates",
			})
		}
	}
}

// Helper methods

func (validator *EnhancedTemplateValidator) validateResourceLimits(limits *ResourceLimits, field string, result *TemplateValidationResult, component *ComponentValidation) {
	if limits.MaxMemoryMB > validator.rules.MaxMemoryMB {
		validator.addWarning(result, "HIGH_MEMORY_LIMIT", fmt.Sprintf("Memory limit %d MB is very high", limits.MaxMemoryMB), field+".max_memory_mb", "Consider if such high memory usage is needed")
		component.Warnings = append(component.Warnings, "High memory limit")
		component.Score -= 5
	}

	if limits.MaxConcurrentRequests > validator.rules.MaxConcurrentRequests {
		validator.addWarning(result, "HIGH_CONCURRENT_REQUESTS_LIMIT", fmt.Sprintf("Concurrent requests limit %d is very high", limits.MaxConcurrentRequests), field+".max_concurrent_requests", "Consider if such high concurrency is needed")
		component.Warnings = append(component.Warnings, "High concurrent requests limit")
		component.Score -= 5
	}
}

func (validator *EnhancedTemplateValidator) validatePoolConfig(config *PoolConfig, field string, result *TemplateValidationResult, component *ComponentValidation) {
	if config.MinSize < validator.rules.MinPoolSize {
		validator.addWarning(result, "LOW_MIN_POOL_SIZE", fmt.Sprintf("Minimum pool size %d is very low", config.MinSize), field+".min_size", "Consider higher minimum pool size for better performance")
		component.Warnings = append(component.Warnings, "Low min pool size")
		component.Score -= 3
	}

	if config.MaxSize > validator.rules.MaxPoolSize {
		validator.addWarning(result, "HIGH_MAX_POOL_SIZE", fmt.Sprintf("Maximum pool size %d is very high", config.MaxSize), field+".max_size", "Consider if such large pool is needed")
		component.Warnings = append(component.Warnings, "High max pool size")
		component.Score -= 5
	}

	// Validate duration fields
	durationFields := map[string]string{
		"max_lifetime":           config.MaxLifetime,
		"idle_timeout":          config.IdleTimeout,
		"health_check_interval": config.HealthCheckInterval,
		"base_delay":            config.BaseDelay,
		"circuit_timeout":       config.CircuitTimeout,
	}

	for fieldName, duration := range durationFields {
		if duration != "" {
			if _, err := time.ParseDuration(duration); err != nil {
				validator.addError(result, "INVALID_DURATION_FORMAT", fmt.Sprintf("Invalid duration format '%s' in %s", duration, fieldName), field+"."+fieldName, "Use duration format like '30s', '5m', '1h'")
				component.IsValid = false
				component.Issues = append(component.Issues, fmt.Sprintf("Invalid %s duration", fieldName))
			}
		}
	}
}

func (validator *EnhancedTemplateValidator) validateHealthCheckSettings(settings *HealthCheckSettings, field string, result *TemplateValidationResult, component *ComponentValidation) {
	// Validate duration fields
	durationFields := map[string]string{
		"interval":      settings.Interval,
		"timeout":       settings.Timeout,
		"restart_delay": settings.RestartDelay,
	}

	for fieldName, duration := range durationFields {
		if duration != "" {
			if _, err := time.ParseDuration(duration); err != nil {
				validator.addError(result, "INVALID_HEALTH_CHECK_DURATION", fmt.Sprintf("Invalid duration format '%s' in health check %s", duration, fieldName), field+"."+fieldName, "Use duration format")
				component.IsValid = false
				component.Issues = append(component.Issues, fmt.Sprintf("Invalid health check %s", fieldName))
			}
		}
	}

	// Validate thresholds
	if settings.FailureThreshold < 1 {
		validator.addWarning(result, "LOW_FAILURE_THRESHOLD", "Failure threshold less than 1 may cause instability", field+".failure_threshold", "Use failure threshold of at least 1")
		component.Warnings = append(component.Warnings, "Low failure threshold")
		component.Score -= 5
	}

	if settings.FailureThreshold > 10 {
		validator.addWarning(result, "HIGH_FAILURE_THRESHOLD", fmt.Sprintf("Failure threshold %d is very high", settings.FailureThreshold), field+".failure_threshold", "Consider lower failure threshold for faster recovery")
		component.Warnings = append(component.Warnings, "High failure threshold")
		component.Score -= 3
	}
}

func (validator *EnhancedTemplateValidator) calculateFinalScore(result *TemplateValidationResult) {
	totalScore := 0
	componentCount := 0

	for _, component := range result.Components {
		totalScore += component.Score
		componentCount++
	}

	if componentCount > 0 {
		result.Score = totalScore / componentCount
	} else {
		result.Score = 100
	}

	// Reduce score for errors
	result.Score -= len(result.Errors) * 20
	if result.Score < 0 {
		result.Score = 0
	}

	// Set validity based on errors
	result.IsValid = len(result.Errors) == 0
}

func (validator *EnhancedTemplateValidator) addError(result *TemplateValidationResult, code, message, field, suggestion string) {
	result.Errors = append(result.Errors, TemplateValidationError{
		Code:       code,
		Message:    message,
		Field:      field,
		Severity:   "error",
		Suggestion: suggestion,
	})
}

func (validator *EnhancedTemplateValidator) addWarning(result *TemplateValidationResult, code, message, field, recommendation string) {
	result.Warnings = append(result.Warnings, ValidationWarning{
		Code:           code,
		Message:        message,
		Field:          field,
		Recommendation: recommendation,
	})
}

func (validator *EnhancedTemplateValidator) addSuggestion(result *TemplateValidationResult, category, message, impact, priority string) {
	result.Suggestions = append(result.Suggestions, ValidationSuggestion{
		Category: category,
		Message:  message,
		Impact:   impact,
		Priority: priority,
	})
}

func (validator *EnhancedTemplateValidator) isSupportedLanguage(language string) bool {
	return validator.containsString(validator.rules.SupportedLanguages, strings.ToLower(language))
}

func (validator *EnhancedTemplateValidator) isSupportedTransport(transport string) bool {
	return validator.containsString(validator.rules.SupportedTransports, strings.ToLower(transport))
}

func (validator *EnhancedTemplateValidator) isSensitiveEnvVar(key, value string) bool {
	sensitiveKeys := []string{"password", "secret", "key", "token", "credential", "auth"}
	lowerKey := strings.ToLower(key)
	lowerValue := strings.ToLower(value)

	for _, sensitive := range sensitiveKeys {
		if strings.Contains(lowerKey, sensitive) || strings.Contains(lowerValue, sensitive) {
			return true
		}
	}

	return false
}

func (validator *EnhancedTemplateValidator) containsString(slice []string, item string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, item) {
			return true
		}
	}
	return false
}