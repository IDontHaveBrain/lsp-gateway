package config

import (
	"fmt"
	"math"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ValidationSeverity represents the severity of a validation issue
type ValidationSeverity int

const (
	SeverityInfo ValidationSeverity = iota
	SeverityWarning
	SeverityError
	SeverityCritical
)

func (s ValidationSeverity) String() string {
	switch s {
	case SeverityInfo:
		return "INFO"
	case SeverityWarning:
		return "WARNING"
	case SeverityError:
		return "ERROR"
	case SeverityCritical:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

// ValidationError represents a structured validation error
type ValidationError struct {
	Component  string             `json:"component"`
	Field      string             `json:"field"`
	Message    string             `json:"message"`
	Severity   ValidationSeverity `json:"severity"`
	Suggestion string             `json:"suggestion,omitempty"`
	Impact     string             `json:"impact,omitempty"`
	Category   string             `json:"category"`
	RuleID     string             `json:"rule_id"`
}

func (ve ValidationError) Error() string {
	return fmt.Sprintf("[%s] %s.%s: %s", ve.Severity, ve.Component, ve.Field, ve.Message)
}

// ValidationWarning represents a validation warning
type ValidationWarning struct {
	Component  string `json:"component"`
	Field      string `json:"field"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
	Impact     string `json:"impact,omitempty"`
	Category   string `json:"category"`
	RuleID     string `json:"rule_id"`
}

// ValidationRecommendation represents an actionable recommendation
type ValidationRecommendation struct {
	Category    string                 `json:"category"`
	Priority    string                 `json:"priority"` // "high", "medium", "low"
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Action      string                 `json:"action"`
	Benefits    []string               `json:"benefits"`
	Risks       []string               `json:"risks"`
	Config      map[string]interface{} `json:"config,omitempty"`
}

// ComponentValidationResult represents validation results for a specific component
type ComponentValidationResult struct {
	Component       string                     `json:"component"`
	Score           int                        `json:"score"` // 0-100
	Grade           string                     `json:"grade"` // A-F
	IsValid         bool                       `json:"is_valid"`
	Errors          []ValidationError          `json:"errors"`
	Warnings        []ValidationWarning        `json:"warnings"`
	Recommendations []ValidationRecommendation `json:"recommendations"`
	Metrics         map[string]float64         `json:"metrics"`
}

// ValidationResult represents comprehensive validation results
type ValidationResult struct {
	IsValid             bool                                  `json:"is_valid"`
	Score               int                                   `json:"score"` // 0-100
	Grade               string                                `json:"grade"` // A-F
	Errors              []ValidationError                     `json:"errors"`
	Warnings            []ValidationWarning                   `json:"warnings"`
	Recommendations     []ValidationRecommendation            `json:"recommendations"`
	ComponentResults    map[string]*ComponentValidationResult `json:"component_results"`
	PerformanceMetrics  map[string]float64                    `json:"performance_metrics"`
	SecurityAssessment  map[string]interface{}                `json:"security_assessment"`
	CompatibilityStatus map[string]string                     `json:"compatibility_status"`
	ValidatedAt         time.Time                             `json:"validated_at"`
	ValidationDuration  time.Duration                         `json:"validation_duration"`
}

// EnhancedValidator provides comprehensive configuration validation
type EnhancedValidator struct {
	multiLanguageValidator *MultiLanguageValidator
	consistencyValidator   *ConsistencyValidator
	performanceValidator   *PerformanceValidator
	compatibilityValidator *CompatibilityValidator
	securityValidator      *SecurityValidator
	templateValidator      *TemplateValidator
}

// NewEnhancedValidator creates a new enhanced validator instance
func NewEnhancedValidator() *EnhancedValidator {
	return &EnhancedValidator{
		multiLanguageValidator: NewMultiLanguageValidator(),
		consistencyValidator:   NewConsistencyValidator(),
		performanceValidator:   NewPerformanceValidator(),
		compatibilityValidator: NewCompatibilityValidator(),
		securityValidator:      NewSecurityValidator(),
		templateValidator:      NewTemplateValidator(),
	}
}

// ValidateComprehensive performs comprehensive configuration validation
func (ev *EnhancedValidator) ValidateComprehensive(config *GatewayConfig) (*ValidationResult, error) {
	start := time.Now()

	result := &ValidationResult{
		ComponentResults:    make(map[string]*ComponentValidationResult),
		PerformanceMetrics:  make(map[string]float64),
		SecurityAssessment:  make(map[string]interface{}),
		CompatibilityStatus: make(map[string]string),
		ValidatedAt:         start,
	}

	// Run all validation components
	components := []struct {
		name      string
		validator func(*GatewayConfig) (*ComponentValidationResult, error)
	}{
		{"multi_language", ev.validateMultiLanguage},
		{"consistency", ev.validateConsistency},
		{"performance", ev.validatePerformance},
		{"compatibility", ev.validateCompatibility},
		{"security", ev.validateSecurity},
		{"template_compliance", ev.validateTemplateCompliance},
	}

	var allErrors []ValidationError
	var allWarnings []ValidationWarning
	var allRecommendations []ValidationRecommendation
	totalScore := 0

	for _, comp := range components {
		compResult, err := comp.validator(config)
		if err != nil {
			return nil, fmt.Errorf("validation failed for component %s: %w", comp.name, err)
		}

		result.ComponentResults[comp.name] = compResult
		allErrors = append(allErrors, compResult.Errors...)
		allWarnings = append(allWarnings, compResult.Warnings...)
		allRecommendations = append(allRecommendations, compResult.Recommendations...)
		totalScore += compResult.Score

		// Merge component metrics
		for k, v := range compResult.Metrics {
			result.PerformanceMetrics[fmt.Sprintf("%s_%s", comp.name, k)] = v
		}
	}

	// Calculate overall score and grade
	result.Score = totalScore / len(components)
	result.Grade = ev.calculateGrade(result.Score)
	result.IsValid = len(allErrors) == 0
	result.Errors = allErrors
	result.Warnings = allWarnings
	result.Recommendations = ev.prioritizeRecommendations(allRecommendations)
	result.ValidationDuration = time.Since(start)

	// Generate security assessment
	result.SecurityAssessment = ev.generateSecurityAssessment(config, allErrors, allWarnings)

	// Generate compatibility status
	result.CompatibilityStatus = ev.generateCompatibilityStatus(config, allErrors, allWarnings)

	return result, nil
}

// validateMultiLanguage validates multi-language configuration consistency
func (ev *EnhancedValidator) validateMultiLanguage(config *GatewayConfig) (*ComponentValidationResult, error) {
	return ev.multiLanguageValidator.Validate(config)
}

// validateConsistency validates cross-component consistency
func (ev *EnhancedValidator) validateConsistency(config *GatewayConfig) (*ComponentValidationResult, error) {
	return ev.consistencyValidator.Validate(config)
}

// validatePerformance validates performance configuration
func (ev *EnhancedValidator) validatePerformance(config *GatewayConfig) (*ComponentValidationResult, error) {
	return ev.performanceValidator.Validate(config)
}

// validateCompatibility validates backward compatibility
func (ev *EnhancedValidator) validateCompatibility(config *GatewayConfig) (*ComponentValidationResult, error) {
	return ev.compatibilityValidator.Validate(config)
}

// validateSecurity validates security configuration
func (ev *EnhancedValidator) validateSecurity(config *GatewayConfig) (*ComponentValidationResult, error) {
	return ev.securityValidator.Validate(config)
}

// validateTemplateCompliance validates template compliance
func (ev *EnhancedValidator) validateTemplateCompliance(config *GatewayConfig) (*ComponentValidationResult, error) {
	return ev.templateValidator.Validate(config)
}

// calculateGrade converts a numeric score to a letter grade
func (ev *EnhancedValidator) calculateGrade(score int) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 80:
		return "B"
	case score >= 70:
		return "C"
	case score >= 60:
		return "D"
	default:
		return "F"
	}
}

// prioritizeRecommendations sorts recommendations by priority and impact
func (ev *EnhancedValidator) prioritizeRecommendations(recommendations []ValidationRecommendation) []ValidationRecommendation {
	sort.Slice(recommendations, func(i, j int) bool {
		priorityOrder := map[string]int{"high": 3, "medium": 2, "low": 1}
		pi, pj := priorityOrder[recommendations[i].Priority], priorityOrder[recommendations[j].Priority]
		if pi != pj {
			return pi > pj
		}
		return len(recommendations[i].Benefits) > len(recommendations[j].Benefits)
	})
	return recommendations
}

// generateSecurityAssessment creates a security assessment report
func (ev *EnhancedValidator) generateSecurityAssessment(config *GatewayConfig, errors []ValidationError, warnings []ValidationWarning) map[string]interface{} {
	assessment := map[string]interface{}{
		"overall_risk":          "low",
		"risk_factors":          []string{},
		"secure_configurations": []string{},
		"recommendations":       []string{},
	}

	securityErrors := 0
	securityWarnings := 0

	for _, err := range errors {
		if err.Category == "security" {
			securityErrors++
			assessment["risk_factors"] = append(assessment["risk_factors"].([]string), err.Message)
		}
	}

	for _, warn := range warnings {
		if warn.Category == "security" {
			securityWarnings++
		}
	}

	// Determine overall risk level
	if securityErrors > 2 {
		assessment["overall_risk"] = "high"
	} else if securityErrors > 0 || securityWarnings > 3 {
		assessment["overall_risk"] = "medium"
	}

	// Check for secure configurations
	if config.Port >= 1024 && config.Port <= 65535 {
		assessment["secure_configurations"] = append(assessment["secure_configurations"].([]string), "Non-privileged port configured")
	}

	if config.MaxConcurrentRequests > 0 && config.MaxConcurrentRequests <= 1000 {
		assessment["secure_configurations"] = append(assessment["secure_configurations"].([]string), "Reasonable concurrency limits set")
	}

	return assessment
}

// generateCompatibilityStatus creates a compatibility status report
func (ev *EnhancedValidator) generateCompatibilityStatus(config *GatewayConfig, errors []ValidationError, warnings []ValidationWarning) map[string]string {
	status := map[string]string{
		"legacy_format":       "compatible",
		"migration_path":      "available",
		"deprecated_features": "none",
	}

	for _, err := range errors {
		if err.Category == "compatibility" {
			if strings.Contains(err.Message, "legacy") {
				status["legacy_format"] = "incompatible"
			}
			if strings.Contains(err.Message, "migration") {
				status["migration_path"] = "required"
			}
		}
	}

	for _, warn := range warnings {
		if warn.Category == "compatibility" && strings.Contains(warn.Message, "deprecated") {
			status["deprecated_features"] = "detected"
		}
	}

	return status
}

// MultiLanguageValidator validates multi-language configuration consistency
type MultiLanguageValidator struct{}

func NewMultiLanguageValidator() *MultiLanguageValidator {
	return &MultiLanguageValidator{}
}

func (mlv *MultiLanguageValidator) Validate(config *GatewayConfig) (*ComponentValidationResult, error) {
	result := &ComponentValidationResult{
		Component: "multi_language",
		Metrics:   make(map[string]float64),
	}

	var errors []ValidationError
	var warnings []ValidationWarning
	var recommendations []ValidationRecommendation
	score := 100

	// Validate cross-language server configuration compatibility
	if err := mlv.validateCrossLanguageCompatibility(config, &errors, &warnings, &score); err != nil {
		return nil, err
	}

	// Validate multi-language routing strategy
	if err := mlv.validateMultiLanguageRouting(config, &errors, &warnings, &score); err != nil {
		return nil, err
	}

	// Validate language-specific resource limit consistency
	if err := mlv.validateLanguageResourceConsistency(config, &errors, &warnings, &score); err != nil {
		return nil, err
	}

	// Validate framework compatibility across languages
	if err := mlv.validateFrameworkCompatibility(config, &errors, &warnings, &score); err != nil {
		return nil, err
	}

	// Validate workspace root path validation for multi-language projects
	if err := mlv.validateWorkspaceRootPaths(config, &errors, &warnings, &score); err != nil {
		return nil, err
	}

	// Generate recommendations
	recommendations = append(recommendations, mlv.generateMultiLanguageRecommendations(config, errors, warnings)...)

	result.Score = score
	result.Grade = calculateComponentGrade(score)
	result.IsValid = len(errors) == 0
	result.Errors = errors
	result.Warnings = warnings
	result.Recommendations = recommendations

	// Calculate metrics
	result.Metrics["language_count"] = float64(mlv.countSupportedLanguages(config))
	result.Metrics["server_count"] = float64(len(config.Servers))
	result.Metrics["pool_count"] = float64(len(config.LanguagePools))
	result.Metrics["compatibility_score"] = float64(score) / 100.0

	return result, nil
}

func (mlv *MultiLanguageValidator) validateCrossLanguageCompatibility(config *GatewayConfig, errors *[]ValidationError, warnings *[]ValidationWarning, score *int) error {
	languageMap := make(map[string][]string) // language -> server names

	for _, server := range config.Servers {
		for _, lang := range server.Languages {
			languageMap[lang] = append(languageMap[lang], server.Name)
		}
	}

	for _, pool := range config.LanguagePools {
		languageMap[pool.Language] = append(languageMap[pool.Language], pool.DefaultServer)
	}

	// Check for conflicts between different server configurations for same language
	for lang, serverNames := range languageMap {
		if len(serverNames) > 1 {
			uniqueServers := make(map[string]bool)
			for _, name := range serverNames {
				if name != "" {
					uniqueServers[name] = true
				}
			}

			if len(uniqueServers) > 1 {
				*warnings = append(*warnings, ValidationWarning{
					Component:  "multi_language",
					Field:      "cross_language_compatibility",
					Message:    fmt.Sprintf("Language %s is handled by multiple different servers: %v", lang, serverNames),
					Suggestion: "Consider consolidating language handling or ensure server compatibility",
					Impact:     "May cause inconsistent behavior across language boundaries",
					Category:   "consistency",
					RuleID:     "ML001",
				})
				*score -= 5
			}
		}
	}

	// Validate transport compatibility across languages
	transportMap := make(map[string]map[string]bool) // transport -> languages
	for _, server := range config.Servers {
		if transportMap[server.Transport] == nil {
			transportMap[server.Transport] = make(map[string]bool)
		}
		for _, lang := range server.Languages {
			transportMap[server.Transport][lang] = true
		}
	}

	if len(transportMap) > 1 {
		*warnings = append(*warnings, ValidationWarning{
			Component:  "multi_language",
			Field:      "transport_compatibility",
			Message:    "Multiple transport types detected across languages",
			Suggestion: "Consider standardizing on a single transport type for consistency",
			Impact:     "Mixed transports may complicate debugging and monitoring",
			Category:   "consistency",
			RuleID:     "ML002",
		})
		*score -= 3
	}

	return nil
}

func (mlv *MultiLanguageValidator) validateMultiLanguageRouting(config *GatewayConfig, errors *[]ValidationError, warnings *[]ValidationWarning, score *int) error {
	if !config.EnableSmartRouting && len(config.LanguagePools) > 1 {
		_ = ValidationRecommendation{
			Category:    "performance",
			Priority:    "medium",
			Title:       "Enable Smart Routing for Multi-Language Projects",
			Description: "Smart routing can optimize request handling across multiple language pools",
			Action:      "Set enable_smart_routing: true in configuration",
			Benefits:    []string{"Better request routing", "Improved performance", "Load balancing"},
			Risks:       []string{"Slightly increased complexity"},
		}
		*score -= 8
	}

	// Validate routing strategies for multi-language compatibility
	if config.SmartRouterConfig != nil {
		for method, strategy := range config.SmartRouterConfig.MethodStrategies {
			if !mlv.isRoutingStrategyCompatibleWithMultiLanguage(strategy) {
				*errors = append(*errors, ValidationError{
					Component:  "multi_language",
					Field:      "routing_strategy",
					Message:    fmt.Sprintf("Routing strategy '%s' for method '%s' may not be optimal for multi-language projects", strategy, method),
					Severity:   SeverityWarning,
					Suggestion: "Consider using 'primary_with_enhancement' or 'broadcast_aggregate' for better multi-language support",
					Impact:     "Suboptimal request routing across languages",
					Category:   "performance",
					RuleID:     "ML003",
				})
				*score -= 10
			}
		}
	}

	return nil
}

func (mlv *MultiLanguageValidator) validateLanguageResourceConsistency(config *GatewayConfig, errors *[]ValidationError, warnings *[]ValidationWarning, score *int) error {
	// Check resource limits consistency across language pools
	var globalLimits *ResourceLimits
	inconsistentPools := []string{}

	for _, pool := range config.LanguagePools {
		if pool.ResourceLimits != nil {
			if globalLimits == nil {
				globalLimits = pool.ResourceLimits
			} else {
				if !mlv.areResourceLimitsConsistent(globalLimits, pool.ResourceLimits) {
					inconsistentPools = append(inconsistentPools, pool.Language)
				}
			}
		}
	}

	if len(inconsistentPools) > 0 {
		*warnings = append(*warnings, ValidationWarning{
			Component:  "multi_language",
			Field:      "resource_consistency",
			Message:    fmt.Sprintf("Inconsistent resource limits across language pools: %v", inconsistentPools),
			Suggestion: "Standardize resource limits across all language pools or document differences",
			Impact:     "May cause uneven resource utilization and performance issues",
			Category:   "performance",
			RuleID:     "ML004",
		})
		*score -= 7
	}

	// Validate concurrent server limits vs language count
	languageCount := mlv.countSupportedLanguages(config)
	if config.MaxConcurrentServersPerLanguage*languageCount > MAX_CONCURRENT_SERVERS_LIMIT {
		*errors = append(*errors, ValidationError{
			Component:  "multi_language",
			Field:      "concurrent_limits",
			Message:    fmt.Sprintf("Total concurrent servers (%d languages × %d servers) exceeds system limit (%d)", languageCount, config.MaxConcurrentServersPerLanguage, MAX_CONCURRENT_SERVERS_LIMIT),
			Severity:   SeverityError,
			Suggestion: "Reduce max concurrent servers per language or reduce number of supported languages",
			Impact:     "Configuration will fail at runtime",
			Category:   "limits",
			RuleID:     "ML005",
		})
		*score -= 20
	}

	return nil
}

func (mlv *MultiLanguageValidator) validateFrameworkCompatibility(config *GatewayConfig, errors *[]ValidationError, warnings *[]ValidationWarning, score *int) error {
	// Check for framework conflicts across languages
	frameworkMap := make(map[string][]string) // framework -> languages

	for _, server := range config.Servers {
		for _, framework := range server.Frameworks {
			for _, lang := range server.Languages {
				frameworkMap[framework] = append(frameworkMap[framework], lang)
			}
		}
	}

	// Detect potential framework conflicts
	conflictingFrameworks := []string{}
	for framework, languages := range frameworkMap {
		if len(languages) > 1 {
			// Check if framework is typically language-specific
			if mlv.isLanguageSpecificFramework(framework) {
				conflictingFrameworks = append(conflictingFrameworks, framework)
			}
		}
	}

	if len(conflictingFrameworks) > 0 {
		*warnings = append(*warnings, ValidationWarning{
			Component:  "multi_language",
			Field:      "framework_compatibility",
			Message:    fmt.Sprintf("Language-specific frameworks detected across multiple languages: %v", conflictingFrameworks),
			Suggestion: "Verify framework configurations are appropriate for each language",
			Impact:     "May cause unexpected behavior or configuration conflicts",
			Category:   "compatibility",
			RuleID:     "ML006",
		})
		*score -= 5
	}

	return nil
}

func (mlv *MultiLanguageValidator) validateWorkspaceRootPaths(config *GatewayConfig, errors *[]ValidationError, warnings *[]ValidationWarning, score *int) error {
	// Validate workspace root paths for multi-root workspaces
	rootPaths := make(map[string]string) // language -> root path

	for _, server := range config.Servers {
		for _, lang := range server.Languages {
			if len(server.WorkspaceRoots) > 0 {
				if root, exists := server.WorkspaceRoots[lang]; exists {
					if existingRoot, found := rootPaths[lang]; found && existingRoot != root {
						*errors = append(*errors, ValidationError{
							Component:  "multi_language",
							Field:      "workspace_roots",
							Message:    fmt.Sprintf("Conflicting workspace roots for language %s: %s vs %s", lang, existingRoot, root),
							Severity:   SeverityError,
							Suggestion: "Ensure consistent workspace root paths for each language",
							Impact:     "LSP servers may fail to initialize or provide inconsistent results",
							Category:   "configuration",
							RuleID:     "ML007",
						})
						*score -= 15
					} else {
						rootPaths[lang] = root
					}
				}
			}
		}
	}

	// Validate root path accessibility and format
	for lang, root := range rootPaths {
		if !filepath.IsAbs(root) {
			*warnings = append(*warnings, ValidationWarning{
				Component:  "multi_language",
				Field:      "workspace_roots",
				Message:    fmt.Sprintf("Relative workspace root path for language %s: %s", lang, root),
				Suggestion: "Use absolute paths for workspace roots to avoid ambiguity",
				Impact:     "May cause path resolution issues",
				Category:   "configuration",
				RuleID:     "ML008",
			})
			*score -= 3
		}
	}

	return nil
}

func (mlv *MultiLanguageValidator) generateMultiLanguageRecommendations(config *GatewayConfig, errors []ValidationError, warnings []ValidationWarning) []ValidationRecommendation {
	var recommendations []ValidationRecommendation

	languageCount := mlv.countSupportedLanguages(config)

	// Recommend smart routing for multi-language projects
	if languageCount > 1 && !config.EnableSmartRouting {
		recommendations = append(recommendations, ValidationRecommendation{
			Category:    "performance",
			Priority:    "high",
			Title:       "Enable Smart Routing for Multi-Language Support",
			Description: "Smart routing optimizes request handling across multiple languages",
			Action:      "Add 'enable_smart_routing: true' to your configuration",
			Benefits:    []string{"Improved request routing", "Better load balancing", "Enhanced performance"},
			Risks:       []string{"Minimal complexity increase"},
		})
	}

	// Recommend resource limit standardization
	if len(config.LanguagePools) > 1 {
		recommendations = append(recommendations, ValidationRecommendation{
			Category:    "performance",
			Priority:    "medium",
			Title:       "Standardize Resource Limits Across Language Pools",
			Description: "Consistent resource limits improve predictability and management",
			Action:      "Define uniform resource limits in global configuration",
			Benefits:    []string{"Predictable resource usage", "Easier monitoring", "Consistent performance"},
			Risks:       []string{"May over-allocate for simple languages"},
		})
	}

	// Recommend concurrent server optimization
	if languageCount > 3 && config.MaxConcurrentServersPerLanguage == DEFAULT_MAX_CONCURRENT_SERVERS_PER_LANG {
		recommendations = append(recommendations, ValidationRecommendation{
			Category:    "scalability",
			Priority:    "medium",
			Title:       "Optimize Concurrent Server Limits for Multi-Language Project",
			Description: "Adjust concurrent server limits based on actual language usage patterns",
			Action:      "Analyze usage patterns and adjust max_concurrent_servers_per_language",
			Benefits:    []string{"Better resource utilization", "Improved scalability", "Reduced overhead"},
			Risks:       []string{"May need monitoring to ensure sufficient capacity"},
		})
	}

	return recommendations
}

// Helper methods for MultiLanguageValidator

func (mlv *MultiLanguageValidator) countSupportedLanguages(config *GatewayConfig) int {
	languages := make(map[string]bool)

	for _, server := range config.Servers {
		for _, lang := range server.Languages {
			languages[lang] = true
		}
	}

	for _, pool := range config.LanguagePools {
		languages[pool.Language] = true
	}

	return len(languages)
}

func (mlv *MultiLanguageValidator) isRoutingStrategyCompatibleWithMultiLanguage(strategy string) bool {
	compatibleStrategies := map[string]bool{
		"primary_with_enhancement": true,
		"broadcast_aggregate":      true,
		"multi_target_parallel":    true,
		"load_balanced":            true,
	}
	return compatibleStrategies[strategy]
}

func (mlv *MultiLanguageValidator) areResourceLimitsConsistent(a, b *ResourceLimits) bool {
	tolerance := 0.2 // 20% tolerance

	if math.Abs(float64(a.MaxMemoryMB-b.MaxMemoryMB)) > float64(a.MaxMemoryMB)*tolerance {
		return false
	}

	if math.Abs(float64(a.MaxConcurrentRequests-b.MaxConcurrentRequests)) > float64(a.MaxConcurrentRequests)*tolerance {
		return false
	}

	return true
}

func (mlv *MultiLanguageValidator) isLanguageSpecificFramework(framework string) bool {
	languageSpecificFrameworks := map[string]bool{
		"django":  true,
		"flask":   true,
		"rails":   true,
		"spring":  true,
		"gin":     true,
		"express": true,
		"react":   true,
		"angular": true,
		"vue":     true,
		".net":    true,
		"laravel": true,
	}
	return languageSpecificFrameworks[strings.ToLower(framework)]
}

// calculateComponentGrade converts component score to grade
func calculateComponentGrade(score int) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 80:
		return "B"
	case score >= 70:
		return "C"
	case score >= 60:
		return "D"
	default:
		return "F"
	}
}

// ConsistencyValidator validates cross-component consistency
type ConsistencyValidator struct{}

func NewConsistencyValidator() *ConsistencyValidator {
	return &ConsistencyValidator{}
}

func (cv *ConsistencyValidator) Validate(config *GatewayConfig) (*ComponentValidationResult, error) {
	result := &ComponentValidationResult{
		Component: "consistency",
		Metrics:   make(map[string]float64),
	}

	var errors []ValidationError
	var warnings []ValidationWarning
	var recommendations []ValidationRecommendation
	score := 100

	// Validate SmartRouter configuration vs server pool setup
	if err := cv.validateSmartRouterConsistency(config, &errors, &warnings, &score); err != nil {
		return nil, err
	}

	// Validate optimization mode vs performance settings alignment
	if err := cv.validateOptimizationConsistency(config, &errors, &warnings, &score); err != nil {
		return nil, err
	}

	// Validate resource limits vs concurrent server settings
	if err := cv.validateResourceConsistency(config, &errors, &warnings, &score); err != nil {
		return nil, err
	}

	// Validate load balancing strategy vs server pool configuration
	if err := cv.validateLoadBalancingConsistency(config, &errors, &warnings, &score); err != nil {
		return nil, err
	}

	// Generate recommendations
	recommendations = append(recommendations, cv.generateConsistencyRecommendations(config, errors, warnings)...)

	result.Score = score
	result.Grade = calculateComponentGrade(score)
	result.IsValid = len(errors) == 0
	result.Errors = errors
	result.Warnings = warnings
	result.Recommendations = recommendations

	// Calculate metrics
	result.Metrics["consistency_score"] = float64(score) / 100.0
	result.Metrics["config_alignment"] = cv.calculateConfigAlignment(config)

	return result, nil
}

func (cv *ConsistencyValidator) validateSmartRouterConsistency(config *GatewayConfig, errors *[]ValidationError, warnings *[]ValidationWarning, score *int) error {
	if config.EnableSmartRouting && config.SmartRouterConfig == nil {
		*errors = append(*errors, ValidationError{
			Component:  "consistency",
			Field:      "smart_router_config",
			Message:    "Smart routing enabled but SmartRouterConfig is nil",
			Severity:   SeverityError,
			Suggestion: "Provide SmartRouterConfig when EnableSmartRouting is true",
			Impact:     "Smart routing will fail to initialize",
			Category:   "configuration",
			RuleID:     "CS001",
		})
		*score -= 20
	}

	if config.SmartRouterConfig != nil {
		// Validate circuit breaker configuration consistency
		if config.SmartRouterConfig.EnableCircuitBreaker {
			if config.SmartRouterConfig.CircuitBreakerThreshold <= 0 {
				*errors = append(*errors, ValidationError{
					Component:  "consistency",
					Field:      "circuit_breaker_threshold",
					Message:    "Circuit breaker enabled but threshold is not positive",
					Severity:   SeverityError,
					Suggestion: "Set circuit_breaker_threshold to a positive value (recommended: 3-10)",
					Impact:     "Circuit breaker will not function properly",
					Category:   "configuration",
					RuleID:     "CS002",
				})
				*score -= 15
			}

			if config.SmartRouterConfig.CircuitBreakerTimeout == "" {
				*warnings = append(*warnings, ValidationWarning{
					Component:  "consistency",
					Field:      "circuit_breaker_timeout",
					Message:    "Circuit breaker enabled but timeout not specified",
					Suggestion: "Specify circuit_breaker_timeout (recommended: 30s-5m)",
					Impact:     "Will use default timeout which may not be optimal",
					Category:   "configuration",
					RuleID:     "CS003",
				})
				*score -= 5
			}
		}

		// Validate routing strategies match available servers
		for method, strategy := range config.SmartRouterConfig.MethodStrategies {
			if !cv.isRoutingStrategyViableForConfig(strategy, config) {
				*warnings = append(*warnings, ValidationWarning{
					Component:  "consistency",
					Field:      "method_strategies",
					Message:    fmt.Sprintf("Routing strategy '%s' for method '%s' may not be viable with current server configuration", strategy, method),
					Suggestion: "Verify routing strategy matches server pool capabilities",
					Impact:     "Suboptimal routing behavior",
					Category:   "performance",
					RuleID:     "CS004",
				})
				*score -= 8
			}
		}
	}

	return nil
}

func (cv *ConsistencyValidator) validateOptimizationConsistency(config *GatewayConfig, errors *[]ValidationError, warnings *[]ValidationWarning, score *int) error {
	// Check if performance settings align with optimization mode
	if config.ProjectConfig != nil && config.ProjectConfig.Optimizations != nil {
		if mode, exists := config.ProjectConfig.Optimizations["mode"]; exists {
			modeStr, ok := mode.(string)
			if !ok {
				*errors = append(*errors, ValidationError{
					Component:  "consistency",
					Field:      "optimization_mode",
					Message:    "Optimization mode is not a string",
					Severity:   SeverityError,
					Suggestion: "Set optimization mode to 'development', 'production', or 'analysis'",
					Impact:     "Optimization settings will not be applied correctly",
					Category:   "configuration",
					RuleID:     "CS005",
				})
				*score -= 15
				return nil
			}

			// Validate settings consistency with optimization mode
			switch modeStr {
			case "production":
				if err := cv.validateProductionConsistency(config, errors, warnings, score); err != nil {
					return err
				}
			case "development":
				if err := cv.validateDevelopmentConsistency(config, errors, warnings, score); err != nil {
					return err
				}
			case "analysis":
				if err := cv.validateAnalysisConsistency(config, errors, warnings, score); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (cv *ConsistencyValidator) validateResourceConsistency(config *GatewayConfig, errors *[]ValidationError, warnings *[]ValidationWarning, score *int) error {
	// Validate global vs pool-specific resource limits
	totalExpectedRequests := 0
	totalExpectedMemory := 0

	for _, pool := range config.LanguagePools {
		if pool.ResourceLimits != nil {
			totalExpectedRequests += pool.ResourceLimits.MaxConcurrentRequests
			totalExpectedMemory += int(pool.ResourceLimits.MaxMemoryMB)
		}
	}

	// Check if global limits are consistent with pool limits
	if config.MaxConcurrentRequests > 0 && totalExpectedRequests > config.MaxConcurrentRequests {
		*warnings = append(*warnings, ValidationWarning{
			Component:  "consistency",
			Field:      "concurrent_requests",
			Message:    fmt.Sprintf("Sum of pool concurrent request limits (%d) exceeds global limit (%d)", totalExpectedRequests, config.MaxConcurrentRequests),
			Suggestion: "Increase global limit or reduce pool-specific limits",
			Impact:     "May cause request throttling or server overload",
			Category:   "performance",
			RuleID:     "CS006",
		})
		*score -= 10
	}

	// Validate concurrent servers vs available resources
	if config.EnableConcurrentServers {
		estimatedServerCount := len(config.LanguagePools) * config.MaxConcurrentServersPerLanguage
		estimatedMemoryUsage := estimatedServerCount * 256 // Assume 256MB per server

		if totalExpectedMemory > 0 && estimatedMemoryUsage > totalExpectedMemory {
			*warnings = append(*warnings, ValidationWarning{
				Component:  "consistency",
				Field:      "memory_allocation",
				Message:    fmt.Sprintf("Estimated server memory usage (%dMB) may exceed allocated memory (%dMB)", estimatedMemoryUsage, totalExpectedMemory),
				Suggestion: "Increase memory limits or reduce concurrent server count",
				Impact:     "Servers may fail to start or experience memory pressure",
				Category:   "resources",
				RuleID:     "CS007",
			})
			*score -= 12
		}
	}

	return nil
}

func (cv *ConsistencyValidator) validateLoadBalancingConsistency(config *GatewayConfig, errors *[]ValidationError, warnings *[]ValidationWarning, score *int) error {
	for _, pool := range config.LanguagePools {
		if pool.LoadBalancingConfig != nil {
			// Validate load balancing strategy matches server pool size
			serverCount := len(pool.Servers)

			if serverCount < 2 && pool.LoadBalancingConfig.Strategy != "" {
				*warnings = append(*warnings, ValidationWarning{
					Component:  "consistency",
					Field:      "load_balancing",
					Message:    fmt.Sprintf("Load balancing configured for language pool '%s' but only %d server(s) available", pool.Language, serverCount),
					Suggestion: "Add more servers to the pool or remove load balancing configuration",
					Impact:     "Load balancing will have no effect",
					Category:   "configuration",
					RuleID:     "CS008",
				})
				*score -= 5
			}

			// Validate weight factors match available servers
			for serverName := range pool.LoadBalancingConfig.WeightFactors {
				if _, exists := pool.Servers[serverName]; !exists {
					*errors = append(*errors, ValidationError{
						Component:  "consistency",
						Field:      "weight_factors",
						Message:    fmt.Sprintf("Weight factor defined for non-existent server '%s' in pool '%s'", serverName, pool.Language),
						Severity:   SeverityError,
						Suggestion: "Remove weight factor or add missing server to pool",
						Impact:     "Load balancing configuration will be invalid",
						Category:   "configuration",
						RuleID:     "CS009",
					})
					*score -= 10
				}
			}
		}
	}

	return nil
}

func (cv *ConsistencyValidator) validateProductionConsistency(config *GatewayConfig, errors *[]ValidationError, warnings *[]ValidationWarning, score *int) error {
	// Validate production-appropriate settings
	if config.MaxConcurrentRequests < 100 {
		*warnings = append(*warnings, ValidationWarning{
			Component:  "consistency",
			Field:      "max_concurrent_requests",
			Message:    "Low concurrent request limit for production mode",
			Suggestion: "Consider increasing max_concurrent_requests for production workloads",
			Impact:     "May limit throughput in production",
			Category:   "performance",
			RuleID:     "CS010",
		})
		*score -= 5
	}

	// Check if debugging features are disabled in production
	for _, server := range config.Servers {
		if settings, ok := server.Settings["gopls"].(map[string]interface{}); ok {
			if verbose, exists := settings["verboseOutput"]; exists {
				if verboseBool, ok := verbose.(bool); ok && verboseBool {
					*warnings = append(*warnings, ValidationWarning{
						Component:  "consistency",
						Field:      "verbose_output",
						Message:    "Verbose output enabled in production mode",
						Suggestion: "Disable verbose output in production for better performance",
						Impact:     "Increased log volume and potential performance impact",
						Category:   "performance",
						RuleID:     "CS011",
					})
					*score -= 3
				}
			}
		}
	}

	return nil
}

func (cv *ConsistencyValidator) validateDevelopmentConsistency(config *GatewayConfig, errors *[]ValidationError, warnings *[]ValidationWarning, score *int) error {
	// Validate development-appropriate settings
	if config.MaxConcurrentRequests > 50 {
		*warnings = append(*warnings, ValidationWarning{
			Component:  "consistency",
			Field:      "max_concurrent_requests",
			Message:    "High concurrent request limit for development mode",
			Suggestion: "Consider reducing max_concurrent_requests for development",
			Impact:     "May consume unnecessary resources during development",
			Category:   "resources",
			RuleID:     "CS012",
		})
		*score -= 3
	}

	return nil
}

func (cv *ConsistencyValidator) validateAnalysisConsistency(config *GatewayConfig, errors *[]ValidationError, warnings *[]ValidationWarning, score *int) error {
	// Validate analysis-appropriate settings
	for _, server := range config.Servers {
		if settings, ok := server.Settings["gopls"].(map[string]interface{}); ok {
			if analyses, exists := settings["analyses"]; exists {
				if analysesMap, ok := analyses.(map[string]interface{}); ok {
					enabledCount := 0
					for _, enabled := range analysesMap {
						if enabledBool, ok := enabled.(bool); ok && enabledBool {
							enabledCount++
						}
					}

					if enabledCount < 3 {
						*warnings = append(*warnings, ValidationWarning{
							Component:  "consistency",
							Field:      "analyses",
							Message:    "Few analyses enabled for analysis mode",
							Suggestion: "Enable more analyses for comprehensive code analysis",
							Impact:     "May miss potential code issues",
							Category:   "analysis",
							RuleID:     "CS013",
						})
						*score -= 5
					}
				}
			}
		}
	}

	return nil
}

func (cv *ConsistencyValidator) generateConsistencyRecommendations(config *GatewayConfig, errors []ValidationError, warnings []ValidationWarning) []ValidationRecommendation {
	var recommendations []ValidationRecommendation

	// Recommend configuration alignment
	if len(errors) > 0 || len(warnings) > 2 {
		recommendations = append(recommendations, ValidationRecommendation{
			Category:    "consistency",
			Priority:    "high",
			Title:       "Resolve Configuration Inconsistencies",
			Description: "Multiple configuration inconsistencies detected that may affect system reliability",
			Action:      "Review and resolve configuration conflicts identified in validation errors",
			Benefits:    []string{"Improved reliability", "Predictable behavior", "Easier troubleshooting"},
			Risks:       []string{"None"},
		})
	}

	// Recommend optimization mode alignment
	if config.ProjectConfig != nil && config.ProjectConfig.Optimizations != nil {
		recommendations = append(recommendations, ValidationRecommendation{
			Category:    "optimization",
			Priority:    "medium",
			Title:       "Align Configuration with Optimization Mode",
			Description: "Ensure all settings are optimized for the selected mode",
			Action:      "Review server settings and resource limits for optimization mode alignment",
			Benefits:    []string{"Better performance", "Appropriate resource usage", "Mode-specific optimizations"},
			Risks:       []string{"May require configuration changes"},
		})
	}

	return recommendations
}

func (cv *ConsistencyValidator) calculateConfigAlignment(config *GatewayConfig) float64 {
	alignmentScore := 1.0

	// Check smart router alignment
	if config.EnableSmartRouting && config.SmartRouterConfig == nil {
		alignmentScore -= 0.3
	}

	// Check resource alignment
	if config.EnableConcurrentServers && config.MaxConcurrentServersPerLanguage == 0 {
		alignmentScore -= 0.2
	}

	// Check pool consistency
	for _, pool := range config.LanguagePools {
		if len(pool.Servers) > 1 && pool.LoadBalancingConfig == nil {
			alignmentScore -= 0.1
		}
	}

	return math.Max(0.0, alignmentScore)
}

func (cv *ConsistencyValidator) isRoutingStrategyViableForConfig(strategy string, config *GatewayConfig) bool {
	serverCount := len(config.Servers)
	poolCount := len(config.LanguagePools)

	switch strategy {
	case "broadcast_aggregate", "multi_target_parallel":
		return serverCount > 1 || poolCount > 1
	case "load_balanced":
		return serverCount > 1
	case "primary_with_enhancement":
		return serverCount >= 1
	default:
		return true
	}
}

// PerformanceValidator validates performance configuration settings
type PerformanceValidator struct{}

func NewPerformanceValidator() *PerformanceValidator {
	return &PerformanceValidator{}
}

func (pv *PerformanceValidator) Validate(config *GatewayConfig) (*ComponentValidationResult, error) {
	result := &ComponentValidationResult{
		Component: "performance",
		Metrics:   make(map[string]float64),
	}

	var errors []ValidationError
	var warnings []ValidationWarning
	var recommendations []ValidationRecommendation
	score := 100

	// Validate performance configuration
	if err := pv.validatePerformanceSettings(config, &errors, &warnings, &score); err != nil {
		return nil, err
	}

	// Validate scalability settings
	if err := pv.validateScalabilitySettings(config, &errors, &warnings, &score); err != nil {
		return nil, err
	}

	// Validate resource utilization efficiency
	if err := pv.validateResourceEfficiency(config, &errors, &warnings, &score); err != nil {
		return nil, err
	}

	// Generate performance recommendations
	recommendations = append(recommendations, pv.generatePerformanceRecommendations(config, errors, warnings)...)

	result.Score = score
	result.Grade = calculateComponentGrade(score)
	result.IsValid = len(errors) == 0
	result.Errors = errors
	result.Warnings = warnings
	result.Recommendations = recommendations

	// Calculate performance metrics
	result.Metrics["performance_score"] = float64(score) / 100.0
	result.Metrics["estimated_throughput"] = pv.calculateEstimatedThroughput(config)
	result.Metrics["resource_efficiency"] = pv.calculateResourceEfficiency(config)
	result.Metrics["scalability_index"] = pv.calculateScalabilityIndex(config)

	return result, nil
}

func (pv *PerformanceValidator) validatePerformanceSettings(config *GatewayConfig, errors *[]ValidationError, warnings *[]ValidationWarning, score *int) error {
	// Validate concurrent request limits
	if config.MaxConcurrentRequests <= 0 {
		*errors = append(*errors, ValidationError{
			Component:  "performance",
			Field:      "max_concurrent_requests",
			Message:    "Max concurrent requests must be positive",
			Severity:   SeverityError,
			Suggestion: "Set max_concurrent_requests to a positive value (recommended: 50-500)",
			Impact:     "System will not handle concurrent requests properly",
			Category:   "configuration",
			RuleID:     "PF001",
		})
		*score -= 25
	} else if config.MaxConcurrentRequests > MAX_CONCURRENT_REQUESTS_LIMIT {
		*warnings = append(*warnings, ValidationWarning{
			Component:  "performance",
			Field:      "max_concurrent_requests",
			Message:    fmt.Sprintf("Very high concurrent request limit: %d", config.MaxConcurrentRequests),
			Suggestion: "Consider if such high concurrency is necessary and ensure adequate resources",
			Impact:     "May cause resource exhaustion under load",
			Category:   "performance",
			RuleID:     "PF002",
		})
		*score -= 10
	}

	// Validate timeout settings
	if config.Timeout != "" {
		if duration, err := time.ParseDuration(config.Timeout); err != nil {
			*errors = append(*errors, ValidationError{
				Component:  "performance",
				Field:      "timeout",
				Message:    fmt.Sprintf("Invalid timeout format: %s", config.Timeout),
				Severity:   SeverityError,
				Suggestion: "Use valid duration format (e.g., '30s', '5m')",
				Impact:     "Timeout configuration will be ignored",
				Category:   "configuration",
				RuleID:     "PF003",
			})
			*score -= 15
		} else {
			if duration < time.Second {
				*warnings = append(*warnings, ValidationWarning{
					Component:  "performance",
					Field:      "timeout",
					Message:    "Very short timeout may cause frequent timeouts",
					Suggestion: "Consider increasing timeout to at least 1 second",
					Impact:     "Requests may timeout prematurely",
					Category:   "performance",
					RuleID:     "PF004",
				})
				*score -= 8
			} else if duration > time.Hour {
				*warnings = append(*warnings, ValidationWarning{
					Component:  "performance",
					Field:      "timeout",
					Message:    "Very long timeout may cause resource leaks",
					Suggestion: "Consider reducing timeout to reasonable duration (5-30 minutes)",
					Impact:     "Resources may be held for too long",
					Category:   "performance",
					RuleID:     "PF005",
				})
				*score -= 5
			}
		}
	}

	// Validate server-specific performance settings
	for _, server := range config.Servers {
		if server.MaxConcurrentRequests > 0 {
			efficiency := float64(server.MaxConcurrentRequests) / float64(config.MaxConcurrentRequests)
			if efficiency > 0.8 {
				*warnings = append(*warnings, ValidationWarning{
					Component:  "performance",
					Field:      "server_concurrent_requests",
					Message:    fmt.Sprintf("Server %s uses high portion of global concurrent limit", server.Name),
					Suggestion: "Ensure balanced resource allocation across servers",
					Impact:     "May cause resource contention",
					Category:   "performance",
					RuleID:     "PF006",
				})
				*score -= 5
			}
		}
	}

	return nil
}

func (pv *PerformanceValidator) validateScalabilitySettings(config *GatewayConfig, errors *[]ValidationError, warnings *[]ValidationWarning, score *int) error {
	// Validate scaling configuration
	if config.EnableConcurrentServers {
		totalMaxServers := len(config.LanguagePools) * config.MaxConcurrentServersPerLanguage
		if totalMaxServers > MAX_CONCURRENT_SERVERS_LIMIT {
			*errors = append(*errors, ValidationError{
				Component:  "performance",
				Field:      "scaling_limits",
				Message:    fmt.Sprintf("Total possible concurrent servers (%d) exceeds system limit (%d)", totalMaxServers, MAX_CONCURRENT_SERVERS_LIMIT),
				Severity:   SeverityError,
				Suggestion: "Reduce max concurrent servers per language or number of language pools",
				Impact:     "System will fail to scale as configured",
				Category:   "scalability",
				RuleID:     "PF007",
			})
			*score -= 20
		}

		// Check scaling efficiency
		if config.MaxConcurrentServersPerLanguage < 2 && len(config.LanguagePools) > 3 {
			*warnings = append(*warnings, ValidationWarning{
				Component:  "performance",
				Field:      "scaling_efficiency",
				Message:    "Low concurrent server limit with many language pools may limit scalability",
				Suggestion: "Consider increasing max concurrent servers per language for better scaling",
				Impact:     "Limited ability to handle concurrent load per language",
				Category:   "scalability",
				RuleID:     "PF008",
			})
			*score -= 8
		}
	}

	// Validate circuit breaker settings for scalability
	if config.SmartRouterConfig != nil && config.SmartRouterConfig.EnableCircuitBreaker {
		if config.SmartRouterConfig.CircuitBreakerThreshold > 10 {
			*warnings = append(*warnings, ValidationWarning{
				Component:  "performance",
				Field:      "circuit_breaker_threshold",
				Message:    "High circuit breaker threshold may delay failure detection",
				Suggestion: "Consider lower threshold (3-10) for faster failure detection",
				Impact:     "Slower recovery from server failures",
				Category:   "reliability",
				RuleID:     "PF009",
			})
			*score -= 5
		}
	}

	return nil
}

func (pv *PerformanceValidator) validateResourceEfficiency(config *GatewayConfig, errors *[]ValidationError, warnings *[]ValidationWarning, score *int) error {
	// Calculate estimated resource usage
	totalMemoryLimit := 0
	totalConcurrentRequests := 0

	for _, pool := range config.LanguagePools {
		if pool.ResourceLimits != nil {
			totalMemoryLimit += int(pool.ResourceLimits.MaxMemoryMB)
			totalConcurrentRequests += pool.ResourceLimits.MaxConcurrentRequests
		}
	}

	// Validate resource allocation efficiency
	if totalMemoryLimit > 0 && totalConcurrentRequests > 0 {
		memoryPerRequest := float64(totalMemoryLimit) / float64(totalConcurrentRequests)
		if memoryPerRequest > 100 { // > 100MB per request
			*warnings = append(*warnings, ValidationWarning{
				Component:  "performance",
				Field:      "resource_efficiency",
				Message:    fmt.Sprintf("High memory allocation per request: %.1f MB", memoryPerRequest),
				Suggestion: "Review memory limits and concurrent request settings for efficiency",
				Impact:     "May lead to resource waste",
				Category:   "efficiency",
				RuleID:     "PF010",
			})
			*score -= 8
		}
	}

	// Validate server weight distribution
	if len(config.LanguagePools) > 0 {
		for _, pool := range config.LanguagePools {
			if pool.LoadBalancingConfig != nil && len(pool.LoadBalancingConfig.WeightFactors) > 0 {
				totalWeight := 0.0
				for _, weight := range pool.LoadBalancingConfig.WeightFactors {
					totalWeight += weight
				}

				if totalWeight > 0 {
					maxWeight := 0.0
					for _, weight := range pool.LoadBalancingConfig.WeightFactors {
						if weight > maxWeight {
							maxWeight = weight
						}
					}

					if maxWeight/totalWeight > 0.7 { // One server has >70% of weight
						*warnings = append(*warnings, ValidationWarning{
							Component:  "performance",
							Field:      "weight_distribution",
							Message:    fmt.Sprintf("Uneven weight distribution in pool %s", pool.Language),
							Suggestion: "Consider more balanced weight distribution for better load sharing",
							Impact:     "Load may not be distributed evenly",
							Category:   "load_balancing",
							RuleID:     "PF011",
						})
						*score -= 5
					}
				}
			}
		}
	}

	return nil
}

func (pv *PerformanceValidator) generatePerformanceRecommendations(config *GatewayConfig, errors []ValidationError, warnings []ValidationWarning) []ValidationRecommendation {
	var recommendations []ValidationRecommendation

	// Recommend performance optimizations
	if pv.calculateEstimatedThroughput(config) < 100 {
		recommendations = append(recommendations, ValidationRecommendation{
			Category:    "performance",
			Priority:    "high",
			Title:       "Optimize Configuration for Higher Throughput",
			Description: "Current configuration may limit system throughput",
			Action:      "Increase concurrent request limits and optimize server settings",
			Benefits:    []string{"Higher throughput", "Better resource utilization", "Improved responsiveness"},
			Risks:       []string{"Higher resource usage", "Need for monitoring"},
		})
	}

	// Recommend scaling improvements
	if config.EnableConcurrentServers && config.MaxConcurrentServersPerLanguage < 3 {
		recommendations = append(recommendations, ValidationRecommendation{
			Category:    "scalability",
			Priority:    "medium",
			Title:       "Increase Concurrent Server Limits for Better Scaling",
			Description: "Current limits may restrict horizontal scaling capabilities",
			Action:      "Increase max_concurrent_servers_per_language to 3-5",
			Benefits:    []string{"Better scaling", "Higher availability", "Load distribution"},
			Risks:       []string{"Higher memory usage", "Increased complexity"},
		})
	}

	// Recommend circuit breaker optimization
	if config.SmartRouterConfig != nil && !config.SmartRouterConfig.EnableCircuitBreaker {
		recommendations = append(recommendations, ValidationRecommendation{
			Category:    "reliability",
			Priority:    "high",
			Title:       "Enable Circuit Breaker for Better Fault Tolerance",
			Description: "Circuit breaker can prevent cascade failures and improve system resilience",
			Action:      "Enable circuit breaker with appropriate threshold and timeout settings",
			Benefits:    []string{"Better fault tolerance", "Faster failure recovery", "Cascade failure prevention"},
			Risks:       []string{"Slightly increased complexity"},
		})
	}

	return recommendations
}

func (pv *PerformanceValidator) calculateEstimatedThroughput(config *GatewayConfig) float64 {
	baseScore := float64(config.MaxConcurrentRequests)

	// Adjust for concurrent servers
	if config.EnableConcurrentServers {
		multiplier := float64(config.MaxConcurrentServersPerLanguage) * 0.8 // Assume 80% efficiency
		baseScore *= multiplier
	}

	// Adjust for smart routing
	if config.EnableSmartRouting {
		baseScore *= 1.2 // 20% improvement from smart routing
	}

	// Penalize for resource constraints
	for _, pool := range config.LanguagePools {
		if pool.ResourceLimits != nil && pool.ResourceLimits.MaxConcurrentRequests > 0 {
			if pool.ResourceLimits.MaxConcurrentRequests < config.MaxConcurrentRequests {
				baseScore *= 0.9 // 10% penalty for constrained pools
			}
		}
	}

	return baseScore
}

func (pv *PerformanceValidator) calculateResourceEfficiency(config *GatewayConfig) float64 {
	efficiency := 1.0

	// Check for resource over-allocation
	totalMemory := 0
	for _, pool := range config.LanguagePools {
		if pool.ResourceLimits != nil {
			totalMemory += int(pool.ResourceLimits.MaxMemoryMB)
		}
	}

	if totalMemory > 0 {
		estimatedNeedMemory := len(config.Servers) * 512 // Assume 512MB per server
		if totalMemory > estimatedNeedMemory*2 {
			efficiency *= 0.7 // Penalty for over-allocation
		}
	}

	// Check for balanced load
	if len(config.LanguagePools) > 0 {
		maxRequests := 0
		minRequests := math.MaxInt32

		for _, pool := range config.LanguagePools {
			if pool.ResourceLimits != nil {
				if pool.ResourceLimits.MaxConcurrentRequests > maxRequests {
					maxRequests = pool.ResourceLimits.MaxConcurrentRequests
				}
				if pool.ResourceLimits.MaxConcurrentRequests < minRequests {
					minRequests = pool.ResourceLimits.MaxConcurrentRequests
				}
			}
		}

		if maxRequests > 0 && minRequests < math.MaxInt32 {
			balance := float64(minRequests) / float64(maxRequests)
			efficiency *= balance // Better balance = higher efficiency
		}
	}

	return math.Max(0.0, efficiency)
}

func (pv *PerformanceValidator) calculateScalabilityIndex(config *GatewayConfig) float64 {
	index := 0.0

	// Base scalability from concurrent servers
	if config.EnableConcurrentServers {
		index += float64(config.MaxConcurrentServersPerLanguage) * 0.2
	}

	// Scalability from multiple language pools
	index += float64(len(config.LanguagePools)) * 0.1

	// Scalability from smart routing
	if config.EnableSmartRouting {
		index += 0.3
	}

	// Scalability from load balancing
	hasLoadBalancing := false
	for _, pool := range config.LanguagePools {
		if pool.LoadBalancingConfig != nil {
			hasLoadBalancing = true
			break
		}
	}

	if hasLoadBalancing {
		index += 0.2
	}

	// Cap at 1.0
	return math.Min(1.0, index)
}

// CompatibilityValidator validates backward compatibility and migration paths
type CompatibilityValidator struct{}

func NewCompatibilityValidator() *CompatibilityValidator {
	return &CompatibilityValidator{}
}

func (cv *CompatibilityValidator) Validate(config *GatewayConfig) (*ComponentValidationResult, error) {
	result := &ComponentValidationResult{
		Component: "compatibility",
		Metrics:   make(map[string]float64),
	}

	var errors []ValidationError
	var warnings []ValidationWarning
	var recommendations []ValidationRecommendation
	score := 100

	// Validate legacy format compatibility
	if err := cv.validateLegacyCompatibility(config, &errors, &warnings, &score); err != nil {
		return nil, err
	}

	// Validate migration path requirements
	if err := cv.validateMigrationPaths(config, &errors, &warnings, &score); err != nil {
		return nil, err
	}

	// Validate deprecated features
	if err := cv.validateDeprecatedFeatures(config, &errors, &warnings, &score); err != nil {
		return nil, err
	}

	// Validate version compatibility
	if err := cv.validateVersionCompatibility(config, &errors, &warnings, &score); err != nil {
		return nil, err
	}

	// Generate compatibility recommendations
	recommendations = append(recommendations, cv.generateCompatibilityRecommendations(config, errors, warnings)...)

	result.Score = score
	result.Grade = calculateComponentGrade(score)
	result.IsValid = len(errors) == 0
	result.Errors = errors
	result.Warnings = warnings
	result.Recommendations = recommendations

	// Calculate compatibility metrics
	result.Metrics["compatibility_score"] = float64(score) / 100.0
	result.Metrics["migration_complexity"] = cv.calculateMigrationComplexity(config)
	result.Metrics["legacy_support"] = cv.calculateLegacySupport(config)

	return result, nil
}

func (cv *CompatibilityValidator) validateLegacyCompatibility(config *GatewayConfig, errors *[]ValidationError, warnings *[]ValidationWarning, score *int) error {
	// Check for mixed legacy and new configuration formats
	hasLegacyServers := len(config.Servers) > 0
	hasNewPools := len(config.LanguagePools) > 0

	if hasLegacyServers && hasNewPools {
		// Check for overlap
		legacyLanguages := make(map[string]bool)
		for _, server := range config.Servers {
			for _, lang := range server.Languages {
				legacyLanguages[lang] = true
			}
		}

		for _, pool := range config.LanguagePools {
			if legacyLanguages[pool.Language] {
				*warnings = append(*warnings, ValidationWarning{
					Component:  "compatibility",
					Field:      "mixed_formats",
					Message:    fmt.Sprintf("Language %s configured in both legacy servers and new language pools", pool.Language),
					Suggestion: "Migrate fully to language pools format for consistency",
					Impact:     "May cause conflicting configurations",
					Category:   "migration",
					RuleID:     "CP001",
				})
				*score -= 8
			}
		}
	}

	// Validate legacy server configuration completeness
	if hasLegacyServers && !hasNewPools {
		missingFields := []string{}
		for _, server := range config.Servers {
			if server.Priority == 0 && server.Weight == 0.0 {
				missingFields = append(missingFields, "priority/weight")
			}
			if server.MaxConcurrentRequests == 0 {
				missingFields = append(missingFields, "max_concurrent_requests")
			}
		}

		if len(missingFields) > 0 {
			*warnings = append(*warnings, ValidationWarning{
				Component:  "compatibility",
				Field:      "legacy_completeness",
				Message:    fmt.Sprintf("Legacy server configurations missing modern fields: %v", missingFields),
				Suggestion: "Add missing fields or migrate to language pools format",
				Impact:     "Limited functionality compared to modern configuration",
				Category:   "migration",
				RuleID:     "CP002",
			})
			*score -= 10
		}
	}

	return nil
}

func (cv *CompatibilityValidator) validateMigrationPaths(config *GatewayConfig, errors *[]ValidationError, warnings *[]ValidationWarning, score *int) error {
	// Check if configuration can be safely migrated
	if len(config.Servers) > 0 && len(config.LanguagePools) == 0 {
		// Legacy-only configuration - check migration readiness
		migrationIssues := []string{}

		// Check for complex server configurations that may be hard to migrate
		for _, server := range config.Servers {
			if len(server.Languages) > 1 {
				migrationIssues = append(migrationIssues, fmt.Sprintf("Multi-language server: %s", server.Name))
			}

			if len(server.Settings) > 0 {
				complexSettings := false
				for _, value := range server.Settings {
					if subMap, ok := value.(map[string]interface{}); ok && len(subMap) > 3 {
						complexSettings = true
						break
					}
				}
				if complexSettings {
					migrationIssues = append(migrationIssues, fmt.Sprintf("Complex settings in server: %s", server.Name))
				}
			}
		}

		if len(migrationIssues) > 0 {
			*warnings = append(*warnings, ValidationWarning{
				Component:  "compatibility",
				Field:      "migration_complexity",
				Message:    fmt.Sprintf("Migration complexity detected: %v", migrationIssues),
				Suggestion: "Plan migration carefully and test thoroughly",
				Impact:     "Migration may require manual intervention",
				Category:   "migration",
				RuleID:     "CP003",
			})
			*score -= 5
		}
	}

	// Check for unsupported legacy features
	for _, server := range config.Servers {
		if server.Transport == "tcp" || server.Transport == "http" {
			*warnings = append(*warnings, ValidationWarning{
				Component:  "compatibility",
				Field:      "transport_migration",
				Message:    fmt.Sprintf("Server %s uses transport %s which may need migration consideration", server.Name, server.Transport),
				Suggestion: "Verify transport compatibility with target configuration format",
				Impact:     "Transport configuration may need adjustment during migration",
				Category:   "migration",
				RuleID:     "CP004",
			})
			*score -= 3
		}
	}

	return nil
}

func (cv *CompatibilityValidator) validateDeprecatedFeatures(config *GatewayConfig, errors *[]ValidationError, warnings *[]ValidationWarning, score *int) error {
	// Check for deprecated configuration fields or patterns
	deprecatedPatterns := []struct {
		check   func(*GatewayConfig) bool
		message string
		impact  string
		ruleID  string
	}{
		{
			check: func(c *GatewayConfig) bool {
				return !c.ProjectAware && (c.ProjectContext != nil || c.ProjectConfig != nil)
			},
			message: "Project context/config defined but project_aware is false",
			impact:  "Project-specific features will not function",
			ruleID:  "CP005",
		},
		{
			check: func(c *GatewayConfig) bool {
				for _, server := range c.Servers {
					if server.Transport == "" {
						return true
					}
				}
				return false
			},
			message: "Servers with empty transport field detected",
			impact:  "May use deprecated default transport behavior",
			ruleID:  "CP006",
		},
		{
			check: func(c *GatewayConfig) bool {
				return c.Port == 8080 && len(c.Servers) > 5
			},
			message: "Using default port with many servers may indicate legacy setup",
			impact:  "May not be optimized for current configuration",
			ruleID:  "CP007",
		},
	}

	for _, pattern := range deprecatedPatterns {
		if pattern.check(config) {
			*warnings = append(*warnings, ValidationWarning{
				Component:  "compatibility",
				Field:      "deprecated_patterns",
				Message:    pattern.message,
				Suggestion: "Update configuration to use modern patterns",
				Impact:     pattern.impact,
				Category:   "deprecation",
				RuleID:     pattern.ruleID,
			})
			*score -= 5
		}
	}

	return nil
}

func (cv *CompatibilityValidator) validateVersionCompatibility(config *GatewayConfig, errors *[]ValidationError, warnings *[]ValidationWarning, score *int) error {
	// Check project config version compatibility
	if config.ProjectConfig != nil && config.ProjectConfig.Version != "" {
		version := config.ProjectConfig.Version
		if !cv.isVersionSupported(version) {
			*errors = append(*errors, ValidationError{
				Component:  "compatibility",
				Field:      "version",
				Message:    fmt.Sprintf("Unsupported project config version: %s", version),
				Severity:   SeverityError,
				Suggestion: "Update to a supported version or migrate configuration",
				Impact:     "Configuration may not work correctly",
				Category:   "version",
				RuleID:     "CP008",
			})
			*score -= 20
		} else if cv.isVersionDeprecated(version) {
			*warnings = append(*warnings, ValidationWarning{
				Component:  "compatibility",
				Field:      "version",
				Message:    fmt.Sprintf("Project config version %s is deprecated", version),
				Suggestion: "Consider upgrading to latest version",
				Impact:     "May lose support in future releases",
				Category:   "deprecation",
				RuleID:     "CP009",
			})
			*score -= 8
		}
	}

	// Check server version compatibility
	for _, server := range config.Servers {
		if server.Version != "" {
			if !cv.isVersionSupported(server.Version) {
				*warnings = append(*warnings, ValidationWarning{
					Component:  "compatibility",
					Field:      "server_version",
					Message:    fmt.Sprintf("Server %s has unsupported version: %s", server.Name, server.Version),
					Suggestion: "Update server configuration version",
					Impact:     "Server may not function optimally",
					Category:   "version",
					RuleID:     "CP010",
				})
				*score -= 5
			}
		}
	}

	return nil
}

func (cv *CompatibilityValidator) generateCompatibilityRecommendations(config *GatewayConfig, errors []ValidationError, warnings []ValidationWarning) []ValidationRecommendation {
	var recommendations []ValidationRecommendation

	// Recommend migration to modern format
	if len(config.Servers) > 0 && len(config.LanguagePools) == 0 {
		recommendations = append(recommendations, ValidationRecommendation{
			Category:    "migration",
			Priority:    "medium",
			Title:       "Migrate to Modern Language Pools Configuration",
			Description: "Current legacy server configuration can be modernized for better functionality",
			Action:      "Convert server configurations to language pools format",
			Benefits:    []string{"Better multi-language support", "Enhanced load balancing", "Improved scalability"},
			Risks:       []string{"Requires configuration changes", "May need testing"},
		})
	}

	// Recommend deprecated feature updates
	hasDeprecatedFeatures := false
	for _, warning := range warnings {
		if warning.Category == "deprecation" {
			hasDeprecatedFeatures = true
			break
		}
	}

	if hasDeprecatedFeatures {
		recommendations = append(recommendations, ValidationRecommendation{
			Category:    "modernization",
			Priority:    "low",
			Title:       "Update Deprecated Configuration Patterns",
			Description: "Some configuration patterns are deprecated and should be updated",
			Action:      "Review and update deprecated configuration elements",
			Benefits:    []string{"Future compatibility", "Access to new features", "Better support"},
			Risks:       []string{"Minor configuration changes needed"},
		})
	}

	// Recommend version updates
	if config.ProjectConfig != nil && config.ProjectConfig.Version != "" {
		if cv.isVersionDeprecated(config.ProjectConfig.Version) {
			recommendations = append(recommendations, ValidationRecommendation{
				Category:    "version",
				Priority:    "medium",
				Title:       "Update Configuration Version",
				Description: "Configuration version is deprecated and should be updated",
				Action:      "Update version field to latest supported version",
				Benefits:    []string{"Latest features", "Continued support", "Bug fixes"},
				Risks:       []string{"May require configuration review"},
			})
		}
	}

	return recommendations
}

func (cv *CompatibilityValidator) calculateMigrationComplexity(config *GatewayConfig) float64 {
	complexity := 0.0

	// Base complexity from server count
	complexity += float64(len(config.Servers)) * 0.1

	// Complexity from mixed formats
	if len(config.Servers) > 0 && len(config.LanguagePools) > 0 {
		complexity += 0.3
	}

	// Complexity from multi-language servers
	for _, server := range config.Servers {
		if len(server.Languages) > 1 {
			complexity += 0.2
		}
		if len(server.Settings) > 3 {
			complexity += 0.1
		}
	}

	// Cap at 1.0
	return math.Min(1.0, complexity)
}

func (cv *CompatibilityValidator) calculateLegacySupport(config *GatewayConfig) float64 {
	support := 1.0

	// Reduce support for deprecated features
	if len(config.Servers) > 0 && len(config.LanguagePools) == 0 {
		support -= 0.2 // Pure legacy format
	}

	// Check for deprecated patterns
	if !config.ProjectAware && (config.ProjectContext != nil || config.ProjectConfig != nil) {
		support -= 0.1
	}

	return math.Max(0.0, support)
}

func (cv *CompatibilityValidator) isVersionSupported(version string) bool {
	supportedVersions := map[string]bool{
		"1.0": true,
		"1.1": true,
		"2.0": true,
		"":    true, // Empty version is supported (uses default)
	}
	return supportedVersions[version]
}

func (cv *CompatibilityValidator) isVersionDeprecated(version string) bool {
	deprecatedVersions := map[string]bool{
		"0.9": true,
		"1.0": true, // Example: 1.0 is deprecated but still supported
	}
	return deprecatedVersions[version]
}

// SecurityValidator validates security-related configuration
type SecurityValidator struct{}

func NewSecurityValidator() *SecurityValidator {
	return &SecurityValidator{}
}

func (sv *SecurityValidator) Validate(config *GatewayConfig) (*ComponentValidationResult, error) {
	result := &ComponentValidationResult{
		Component: "security",
		Metrics:   make(map[string]float64),
	}

	var errors []ValidationError
	var warnings []ValidationWarning
	var recommendations []ValidationRecommendation
	score := 100

	// Validate port security
	if err := sv.validatePortSecurity(config, &errors, &warnings, &score); err != nil {
		return nil, err
	}

	// Validate resource limits for security
	if err := sv.validateResourceLimitSecurity(config, &errors, &warnings, &score); err != nil {
		return nil, err
	}

	// Validate configuration exposure risks
	if err := sv.validateConfigurationExposure(config, &errors, &warnings, &score); err != nil {
		return nil, err
	}

	// Validate access control
	if err := sv.validateAccessControl(config, &errors, &warnings, &score); err != nil {
		return nil, err
	}

	// Generate security recommendations
	recommendations = append(recommendations, sv.generateSecurityRecommendations(config, errors, warnings)...)

	result.Score = score
	result.Grade = calculateComponentGrade(score)
	result.IsValid = len(errors) == 0
	result.Errors = errors
	result.Warnings = warnings
	result.Recommendations = recommendations

	// Calculate security metrics
	result.Metrics["security_score"] = float64(score) / 100.0
	result.Metrics["risk_level"] = sv.calculateRiskLevel(config)
	result.Metrics["hardening_level"] = sv.calculateHardeningLevel(config)

	return result, nil
}

func (sv *SecurityValidator) validatePortSecurity(config *GatewayConfig, errors *[]ValidationError, warnings *[]ValidationWarning, score *int) error {
	// Check for privileged port usage
	if config.Port < 1024 && config.Port != 0 {
		*warnings = append(*warnings, ValidationWarning{
			Component:  "security",
			Field:      "port",
			Message:    fmt.Sprintf("Using privileged port %d requires elevated permissions", config.Port),
			Suggestion: "Consider using non-privileged port (>= 1024) for better security",
			Impact:     "Requires running with elevated privileges",
			Category:   "security",
			RuleID:     "SC001",
		})
		*score -= 10
	}

	// Check for common/well-known ports that might conflict
	commonPorts := map[int]string{
		80:   "HTTP",
		443:  "HTTPS",
		8080: "HTTP-Alt (common development port)",
		3000: "Common development port",
		9000: "Common application port",
	}

	if desc, isCommon := commonPorts[config.Port]; isCommon && config.Port != 8080 {
		*warnings = append(*warnings, ValidationWarning{
			Component:  "security",
			Field:      "port",
			Message:    fmt.Sprintf("Using common port %d (%s) may cause conflicts", config.Port, desc),
			Suggestion: "Consider using a less common port to avoid conflicts",
			Impact:     "May conflict with other services",
			Category:   "security",
			RuleID:     "SC002",
		})
		*score -= 3
	}

	return nil
}

func (sv *SecurityValidator) validateResourceLimitSecurity(config *GatewayConfig, errors *[]ValidationError, warnings *[]ValidationWarning, score *int) error {
	// Check for excessive resource limits that could be exploited
	if config.MaxConcurrentRequests > 500 {
		*warnings = append(*warnings, ValidationWarning{
			Component:  "security",
			Field:      "max_concurrent_requests",
			Message:    "Very high concurrent request limit may enable DoS attacks",
			Suggestion: "Consider implementing rate limiting and monitoring",
			Impact:     "Vulnerable to resource exhaustion attacks",
			Category:   "security",
			RuleID:     "SC003",
		})
		*score -= 8
	}

	// Check for missing resource limits
	hasResourceLimits := false
	for _, pool := range config.LanguagePools {
		if pool.ResourceLimits != nil {
			hasResourceLimits = true

			// Check for excessive memory limits
			if pool.ResourceLimits.MaxMemoryMB > 8192 { // > 8GB
				*warnings = append(*warnings, ValidationWarning{
					Component:  "security",
					Field:      "memory_limits",
					Message:    fmt.Sprintf("Very high memory limit (%d MB) for pool %s", pool.ResourceLimits.MaxMemoryMB, pool.Language),
					Suggestion: "Review if such high memory limits are necessary",
					Impact:     "May enable memory exhaustion attacks",
					Category:   "security",
					RuleID:     "SC004",
				})
				*score -= 5
			}

			// Check for missing request timeout
			if pool.ResourceLimits.RequestTimeoutSeconds == 0 {
				*warnings = append(*warnings, ValidationWarning{
					Component:  "security",
					Field:      "request_timeout",
					Message:    fmt.Sprintf("No request timeout configured for pool %s", pool.Language),
					Suggestion: "Set reasonable request timeout to prevent hanging requests",
					Impact:     "Vulnerable to slowloris-style attacks",
					Category:   "security",
					RuleID:     "SC005",
				})
				*score -= 8
			}
		}
	}

	if !hasResourceLimits && len(config.LanguagePools) > 0 {
		*warnings = append(*warnings, ValidationWarning{
			Component:  "security",
			Field:      "resource_limits",
			Message:    "No resource limits configured for language pools",
			Suggestion: "Configure resource limits to prevent resource exhaustion",
			Impact:     "Vulnerable to resource exhaustion attacks",
			Category:   "security",
			RuleID:     "SC006",
		})
		*score -= 12
	}

	return nil
}

func (sv *SecurityValidator) validateConfigurationExposure(config *GatewayConfig, errors *[]ValidationError, warnings *[]ValidationWarning, score *int) error {
	// Check for potentially sensitive information in settings
	for _, server := range config.Servers {
		for key, value := range server.Settings {
			if sv.containsSensitiveInfo(key, value) {
				*warnings = append(*warnings, ValidationWarning{
					Component:  "security",
					Field:      "server_settings",
					Message:    fmt.Sprintf("Potentially sensitive information in server %s settings", server.Name),
					Suggestion: "Use environment variables or external configuration for sensitive data",
					Impact:     "Sensitive information may be exposed",
					Category:   "security",
					RuleID:     "SC007",
				})
				*score -= 10
			}
		}
	}

	// Check for health check endpoints that might expose information
	for _, server := range config.Servers {
		if server.HealthCheckEndpoint != "" {
			if !strings.HasPrefix(server.HealthCheckEndpoint, "http://") && !strings.HasPrefix(server.HealthCheckEndpoint, "https://") {
				*errors = append(*errors, ValidationError{
					Component:  "security",
					Field:      "health_check_endpoint",
					Message:    fmt.Sprintf("Invalid health check endpoint format for server %s", server.Name),
					Severity:   SeverityError,
					Suggestion: "Use proper HTTP/HTTPS URL format",
					Impact:     "Health checks will fail",
					Category:   "security",
					RuleID:     "SC008",
				})
				*score -= 15
			}
		}
	}

	return nil
}

func (sv *SecurityValidator) validateAccessControl(config *GatewayConfig, errors *[]ValidationError, warnings *[]ValidationWarning, score *int) error {
	// Check if running in production-like mode without proper access controls
	if config.MaxConcurrentRequests > 100 {
		*warnings = append(*warnings, ValidationWarning{
			Component:  "security",
			Field:      "access_control",
			Message:    "High concurrency configuration detected without explicit access control",
			Suggestion: "Implement authentication, authorization, and rate limiting",
			Impact:     "Service may be accessible without proper controls",
			Category:   "security",
			RuleID:     "SC009",
		})
		*score -= 8
	}

	// Check for circuit breaker configuration from security perspective
	if config.SmartRouterConfig != nil && !config.SmartRouterConfig.EnableCircuitBreaker {
		*warnings = append(*warnings, ValidationWarning{
			Component:  "security",
			Field:      "circuit_breaker",
			Message:    "Circuit breaker disabled - may be vulnerable to cascade failures",
			Suggestion: "Enable circuit breaker to prevent cascade failures and improve resilience",
			Impact:     "System may be vulnerable to cascade failures under attack",
			Category:   "security",
			RuleID:     "SC010",
		})
		*score -= 5
	}

	return nil
}

func (sv *SecurityValidator) generateSecurityRecommendations(config *GatewayConfig, errors []ValidationError, warnings []ValidationWarning) []ValidationRecommendation {
	var recommendations []ValidationRecommendation

	// Recommend security hardening
	riskLevel := sv.calculateRiskLevel(config)
	if riskLevel > 0.5 {
		recommendations = append(recommendations, ValidationRecommendation{
			Category:    "security",
			Priority:    "high",
			Title:       "Implement Security Hardening Measures",
			Description: "Configuration has elevated security risk that should be addressed",
			Action:      "Review and implement recommended security measures",
			Benefits:    []string{"Reduced attack surface", "Better resilience", "Compliance improvement"},
			Risks:       []string{"May require configuration changes", "Could affect performance"},
		})
	}

	// Recommend resource limits
	hasResourceLimits := false
	for _, pool := range config.LanguagePools {
		if pool.ResourceLimits != nil {
			hasResourceLimits = true
			break
		}
	}

	if !hasResourceLimits && len(config.LanguagePools) > 0 {
		recommendations = append(recommendations, ValidationRecommendation{
			Category:    "security",
			Priority:    "high",
			Title:       "Configure Resource Limits for Security",
			Description: "Resource limits help prevent DoS attacks and resource exhaustion",
			Action:      "Add resource limits to all language pools",
			Benefits:    []string{"DoS protection", "Resource management", "System stability"},
			Risks:       []string{"Need to determine appropriate limits"},
		})
	}

	// Recommend monitoring and alerting
	if config.MaxConcurrentRequests > 200 {
		recommendations = append(recommendations, ValidationRecommendation{
			Category:    "security",
			Priority:    "medium",
			Title:       "Implement Security Monitoring",
			Description: "High-throughput configuration should include security monitoring",
			Action:      "Set up monitoring, alerting, and logging for security events",
			Benefits:    []string{"Attack detection", "Incident response", "Compliance"},
			Risks:       []string{"Additional infrastructure needed"},
		})
	}

	return recommendations
}

func (sv *SecurityValidator) calculateRiskLevel(config *GatewayConfig) float64 {
	risk := 0.0

	// Risk from high concurrency without limits
	if config.MaxConcurrentRequests > 300 {
		risk += 0.3
	}

	// Risk from privileged port
	if config.Port < 1024 && config.Port != 0 {
		risk += 0.2
	}

	// Risk from missing resource limits
	hasLimits := false
	for _, pool := range config.LanguagePools {
		if pool.ResourceLimits != nil {
			hasLimits = true
			break
		}
	}
	if !hasLimits && len(config.LanguagePools) > 0 {
		risk += 0.3
	}

	// Risk from disabled circuit breaker
	if config.SmartRouterConfig != nil && !config.SmartRouterConfig.EnableCircuitBreaker {
		risk += 0.2
	}

	return math.Min(1.0, risk)
}

func (sv *SecurityValidator) calculateHardeningLevel(config *GatewayConfig) float64 {
	hardening := 0.0

	// Points for using non-privileged port
	if config.Port >= 1024 {
		hardening += 0.2
	}

	// Points for having resource limits
	for _, pool := range config.LanguagePools {
		if pool.ResourceLimits != nil {
			hardening += 0.2
			break
		}
	}

	// Points for circuit breaker
	if config.SmartRouterConfig != nil && config.SmartRouterConfig.EnableCircuitBreaker {
		hardening += 0.2
	}

	// Points for reasonable concurrency limits
	if config.MaxConcurrentRequests > 0 && config.MaxConcurrentRequests <= 200 {
		hardening += 0.2
	}

	// Points for timeout configuration
	for _, pool := range config.LanguagePools {
		if pool.ResourceLimits != nil && pool.ResourceLimits.RequestTimeoutSeconds > 0 {
			hardening += 0.2
			break
		}
	}

	return math.Min(1.0, hardening)
}

func (sv *SecurityValidator) containsSensitiveInfo(key string, value interface{}) bool {
	sensitivePatterns := []string{
		"password", "secret", "key", "token", "credential",
		"auth", "private", "cert", "ssl", "tls",
	}

	keyLower := strings.ToLower(key)
	for _, pattern := range sensitivePatterns {
		if strings.Contains(keyLower, pattern) {
			return true
		}
	}

	// Check value for sensitive patterns
	if strValue, ok := value.(string); ok {
		if len(strValue) > 20 && (strings.Contains(strings.ToLower(strValue), "secret") ||
			regexp.MustCompile(`^[A-Za-z0-9+/=]+$`).MatchString(strValue)) {
			return true
		}
	}

	return false
}

// TemplateValidator validates template compliance and best practices
type TemplateValidator struct{}

func NewTemplateValidator() *TemplateValidator {
	return &TemplateValidator{}
}

func (tv *TemplateValidator) Validate(config *GatewayConfig) (*ComponentValidationResult, error) {
	result := &ComponentValidationResult{
		Component: "template_compliance",
		Metrics:   make(map[string]float64),
	}

	var errors []ValidationError
	var warnings []ValidationWarning
	var recommendations []ValidationRecommendation
	score := 100

	// Validate configuration completeness
	if err := tv.validateConfigurationCompleteness(config, &errors, &warnings, &score); err != nil {
		return nil, err
	}

	// Validate best practices adherence
	if err := tv.validateBestPractices(config, &errors, &warnings, &score); err != nil {
		return nil, err
	}

	// Validate naming conventions
	if err := tv.validateNamingConventions(config, &errors, &warnings, &score); err != nil {
		return nil, err
	}

	// Validate structure organization
	if err := tv.validateStructureOrganization(config, &errors, &warnings, &score); err != nil {
		return nil, err
	}

	// Generate template recommendations
	recommendations = append(recommendations, tv.generateTemplateRecommendations(config, errors, warnings)...)

	result.Score = score
	result.Grade = calculateComponentGrade(score)
	result.IsValid = len(errors) == 0
	result.Errors = errors
	result.Warnings = warnings
	result.Recommendations = recommendations

	// Calculate template metrics
	result.Metrics["completeness_score"] = float64(score) / 100.0
	result.Metrics["best_practices_adherence"] = tv.calculateBestPracticesAdherence(config)
	result.Metrics["naming_consistency"] = tv.calculateNamingConsistency(config)
	result.Metrics["structure_organization"] = tv.calculateStructureOrganization(config)

	return result, nil
}

func (tv *TemplateValidator) validateConfigurationCompleteness(config *GatewayConfig, errors *[]ValidationError, warnings *[]ValidationWarning, score *int) error {
	// Check for essential configuration fields
	essentialFields := []struct {
		field    string
		present  func(*GatewayConfig) bool
		severity ValidationSeverity
		message  string
		ruleID   string
	}{
		{
			field:    "port",
			present:  func(c *GatewayConfig) bool { return c.Port > 0 },
			severity: SeverityError,
			message:  "Port must be specified and greater than 0",
			ruleID:   "TP001",
		},
		{
			field:    "servers_or_pools",
			present:  func(c *GatewayConfig) bool { return len(c.Servers) > 0 || len(c.LanguagePools) > 0 },
			severity: SeverityError,
			message:  "At least one server or language pool must be configured",
			ruleID:   "TP002",
		},
		{
			field:    "timeout",
			present:  func(c *GatewayConfig) bool { return c.Timeout != "" },
			severity: SeverityWarning,
			message:  "Timeout should be explicitly configured",
			ruleID:   "TP003",
		},
		{
			field:    "max_concurrent_requests",
			present:  func(c *GatewayConfig) bool { return c.MaxConcurrentRequests > 0 },
			severity: SeverityWarning,
			message:  "Max concurrent requests should be explicitly set",
			ruleID:   "TP004",
		},
	}

	for _, field := range essentialFields {
		if !field.present(config) {
			if field.severity == SeverityError {
				*errors = append(*errors, ValidationError{
					Component:  "template_compliance",
					Field:      field.field,
					Message:    field.message,
					Severity:   field.severity,
					Suggestion: fmt.Sprintf("Configure the %s field appropriately", field.field),
					Impact:     "Configuration may not work correctly",
					Category:   "completeness",
					RuleID:     field.ruleID,
				})
				*score -= 20
			} else {
				*warnings = append(*warnings, ValidationWarning{
					Component:  "template_compliance",
					Field:      field.field,
					Message:    field.message,
					Suggestion: fmt.Sprintf("Consider explicitly configuring the %s field", field.field),
					Impact:     "May use default values that are not optimal",
					Category:   "completeness",
					RuleID:     field.ruleID,
				})
				*score -= 5
			}
		}
	}

	// Validate server-specific completeness
	for i, server := range config.Servers {
		if server.Name == "" {
			*errors = append(*errors, ValidationError{
				Component:  "template_compliance",
				Field:      "server_name",
				Message:    fmt.Sprintf("Server at index %d missing name", i),
				Severity:   SeverityError,
				Suggestion: "Provide a unique name for each server",
				Impact:     "Server identification will be problematic",
				Category:   "completeness",
				RuleID:     "TP005",
			})
			*score -= 15
		}

		if server.Command == "" {
			*errors = append(*errors, ValidationError{
				Component:  "template_compliance",
				Field:      "server_command",
				Message:    fmt.Sprintf("Server %s missing command", server.Name),
				Severity:   SeverityError,
				Suggestion: "Specify the command to start the language server",
				Impact:     "Server cannot be started",
				Category:   "completeness",
				RuleID:     "TP006",
			})
			*score -= 15
		}

		if len(server.Languages) == 0 {
			*errors = append(*errors, ValidationError{
				Component:  "template_compliance",
				Field:      "server_languages",
				Message:    fmt.Sprintf("Server %s has no languages configured", server.Name),
				Severity:   SeverityError,
				Suggestion: "Specify at least one language that this server supports",
				Impact:     "Server will not handle any language requests",
				Category:   "completeness",
				RuleID:     "TP007",
			})
			*score -= 15
		}
	}

	return nil
}

func (tv *TemplateValidator) validateBestPractices(config *GatewayConfig, errors *[]ValidationError, warnings *[]ValidationWarning, score *int) error {
	// Check for common best practices
	bestPractices := []struct {
		check   func(*GatewayConfig) bool
		message string
		impact  string
		ruleID  string
		points  int
	}{
		{
			check: func(c *GatewayConfig) bool {
				return c.MaxConcurrentRequests > 0 && c.MaxConcurrentRequests <= 1000
			},
			message: "Max concurrent requests should be reasonably bounded",
			impact:  "Prevents resource exhaustion while allowing adequate throughput",
			ruleID:  "TP008",
			points:  5,
		},
		{
			check: func(c *GatewayConfig) bool {
				return c.Port >= 1024 && c.Port <= 65535
			},
			message: "Should use non-privileged port range",
			impact:  "Avoids requiring elevated privileges",
			ruleID:  "TP009",
			points:  3,
		},
		{
			check: func(c *GatewayConfig) bool {
				if c.Timeout == "" {
					return false
				}
				if duration, err := time.ParseDuration(c.Timeout); err == nil {
					return duration >= time.Second && duration <= time.Hour
				}
				return false
			},
			message: "Timeout should be within reasonable range (1s-1h)",
			impact:  "Balances responsiveness with operation completion time",
			ruleID:  "TP010",
			points:  5,
		},
		{
			check: func(c *GatewayConfig) bool {
				return len(c.LanguagePools) == 0 || tv.allPoolsHaveResourceLimits(c)
			},
			message: "Language pools should have resource limits configured",
			impact:  "Prevents resource exhaustion and improves stability",
			ruleID:  "TP011",
			points:  8,
		},
		{
			check: func(c *GatewayConfig) bool {
				return !c.EnableConcurrentServers || c.MaxConcurrentServersPerLanguage > 0
			},
			message: "Concurrent servers feature should be properly configured",
			impact:  "Ensures concurrent server limits are set when feature is enabled",
			ruleID:  "TP012",
			points:  5,
		},
	}

	for _, practice := range bestPractices {
		if !practice.check(config) {
			*warnings = append(*warnings, ValidationWarning{
				Component:  "template_compliance",
				Field:      "best_practices",
				Message:    practice.message,
				Suggestion: "Follow recommended best practices for better configuration",
				Impact:     practice.impact,
				Category:   "best_practices",
				RuleID:     practice.ruleID,
			})
			*score -= practice.points
		}
	}

	// Check for server-specific best practices
	for _, server := range config.Servers {
		if server.Transport == "" {
			*warnings = append(*warnings, ValidationWarning{
				Component:  "template_compliance",
				Field:      "server_transport",
				Message:    fmt.Sprintf("Server %s has no explicit transport configuration", server.Name),
				Suggestion: "Explicitly specify transport type (stdio, tcp, http)",
				Impact:     "May use default transport which might not be optimal",
				Category:   "best_practices",
				RuleID:     "TP013",
			})
			*score -= 3
		}

		if len(server.RootMarkers) == 0 {
			*warnings = append(*warnings, ValidationWarning{
				Component:  "template_compliance",
				Field:      "server_root_markers",
				Message:    fmt.Sprintf("Server %s has no root markers configured", server.Name),
				Suggestion: "Configure root markers to help with project root detection",
				Impact:     "May have issues with project root detection",
				Category:   "best_practices",
				RuleID:     "TP014",
			})
			*score -= 2
		}
	}

	return nil
}

func (tv *TemplateValidator) validateNamingConventions(config *GatewayConfig, errors *[]ValidationError, warnings *[]ValidationWarning, score *int) error {
	// Check server naming conventions
	serverNames := make(map[string]bool)

	for _, server := range config.Servers {
		if server.Name != "" {
			// Check for duplicate names
			if serverNames[server.Name] {
				*errors = append(*errors, ValidationError{
					Component:  "template_compliance",
					Field:      "server_naming",
					Message:    fmt.Sprintf("Duplicate server name: %s", server.Name),
					Severity:   SeverityError,
					Suggestion: "Ensure all server names are unique",
					Impact:     "Server identification conflicts",
					Category:   "naming",
					RuleID:     "TP015",
				})
				*score -= 20
			}
			serverNames[server.Name] = true

			// Check naming conventions
			if !tv.isValidServerName(server.Name) {
				*warnings = append(*warnings, ValidationWarning{
					Component:  "template_compliance",
					Field:      "server_naming",
					Message:    fmt.Sprintf("Server name '%s' doesn't follow recommended naming convention", server.Name),
					Suggestion: "Use lowercase, hyphen-separated names (e.g., 'go-lsp', 'python-server')",
					Impact:     "Inconsistent naming may cause confusion",
					Category:   "naming",
					RuleID:     "TP016",
				})
				*score -= 3
			}
		}
	}

	// Check language pool naming
	poolLanguages := make(map[string]bool)
	for _, pool := range config.LanguagePools {
		if poolLanguages[pool.Language] {
			*errors = append(*errors, ValidationError{
				Component:  "template_compliance",
				Field:      "pool_naming",
				Message:    fmt.Sprintf("Duplicate language pool: %s", pool.Language),
				Severity:   SeverityError,
				Suggestion: "Each language should have only one pool",
				Impact:     "Language routing conflicts",
				Category:   "naming",
				RuleID:     "TP017",
			})
			*score -= 20
		}
		poolLanguages[pool.Language] = true
	}

	return nil
}

func (tv *TemplateValidator) validateStructureOrganization(config *GatewayConfig, errors *[]ValidationError, warnings *[]ValidationWarning, score *int) error {
	// Check for logical structure organization

	// Validate server grouping by language
	languageServerCount := make(map[string]int)
	for _, server := range config.Servers {
		for _, lang := range server.Languages {
			languageServerCount[lang]++
		}
	}

	// Warn about languages with multiple servers but no pools
	for lang, count := range languageServerCount {
		if count > 1 && len(config.LanguagePools) == 0 {
			*warnings = append(*warnings, ValidationWarning{
				Component:  "template_compliance",
				Field:      "structure_organization",
				Message:    fmt.Sprintf("Language %s has %d servers but no language pool configured", lang, count),
				Suggestion: "Consider using language pools for better organization of multiple servers per language",
				Impact:     "May complicate server management and routing",
				Category:   "organization",
				RuleID:     "TP018",
			})
			*score -= 5
		}
	}

	// Check for mixed configuration approaches
	hasServers := len(config.Servers) > 0
	hasPools := len(config.LanguagePools) > 0

	if hasServers && hasPools {
		*warnings = append(*warnings, ValidationWarning{
			Component:  "template_compliance",
			Field:      "structure_organization",
			Message:    "Mixed configuration approach detected (both servers and language pools)",
			Suggestion: "Consider using only language pools for consistency",
			Impact:     "May cause configuration complexity and maintenance issues",
			Category:   "organization",
			RuleID:     "TP019",
		})
		*score -= 8
	}

	// Validate smart router configuration organization
	if config.EnableSmartRouting && config.SmartRouterConfig == nil {
		*errors = append(*errors, ValidationError{
			Component:  "template_compliance",
			Field:      "structure_organization",
			Message:    "Smart routing enabled but configuration is missing",
			Severity:   SeverityError,
			Suggestion: "Provide SmartRouterConfig when EnableSmartRouting is true",
			Impact:     "Smart routing will not function",
			Category:   "organization",
			RuleID:     "TP020",
		})
		*score -= 20
	}

	// Check for project awareness organization
	if config.ProjectAware && config.ProjectContext == nil && config.ProjectConfig == nil {
		*warnings = append(*warnings, ValidationWarning{
			Component:  "template_compliance",
			Field:      "structure_organization",
			Message:    "Project awareness enabled but no project context/config provided",
			Suggestion: "Provide ProjectContext or ProjectConfig when ProjectAware is true",
			Impact:     "Project-aware features may not work correctly",
			Category:   "organization",
			RuleID:     "TP021",
		})
		*score -= 8
	}

	return nil
}

func (tv *TemplateValidator) generateTemplateRecommendations(config *GatewayConfig, errors []ValidationError, warnings []ValidationWarning) []ValidationRecommendation {
	var recommendations []ValidationRecommendation

	// Recommend configuration completeness improvements
	completenessScore := tv.calculateBestPracticesAdherence(config)
	if completenessScore < 0.8 {
		recommendations = append(recommendations, ValidationRecommendation{
			Category:    "completeness",
			Priority:    "high",
			Title:       "Improve Configuration Completeness",
			Description: "Configuration is missing several important fields and best practices",
			Action:      "Review and add missing configuration fields and best practices",
			Benefits:    []string{"Better reliability", "Improved functionality", "Easier maintenance"},
			Risks:       []string{"May require configuration testing"},
		})
	}

	// Recommend structure organization improvements
	if len(config.Servers) > 0 && len(config.LanguagePools) > 0 {
		recommendations = append(recommendations, ValidationRecommendation{
			Category:    "organization",
			Priority:    "medium",
			Title:       "Consolidate Configuration Approach",
			Description: "Mixed configuration approaches can complicate management",
			Action:      "Migrate to using only language pools for consistency",
			Benefits:    []string{"Simplified management", "Better scalability", "Consistent configuration"},
			Risks:       []string{"Requires migration effort", "Need to test changes"},
		})
	}

	// Recommend naming improvements
	namingScore := tv.calculateNamingConsistency(config)
	if namingScore < 0.9 {
		recommendations = append(recommendations, ValidationRecommendation{
			Category:    "naming",
			Priority:    "low",
			Title:       "Improve Naming Consistency",
			Description: "Server and component names could follow better conventions",
			Action:      "Adopt consistent naming conventions across all components",
			Benefits:    []string{"Better readability", "Easier maintenance", "Reduced confusion"},
			Risks:       []string{"Cosmetic changes only"},
		})
	}

	// Recommend best practices adoption
	if tv.calculateBestPracticesAdherence(config) < 0.9 {
		recommendations = append(recommendations, ValidationRecommendation{
			Category:    "best_practices",
			Priority:    "medium",
			Title:       "Adopt Configuration Best Practices",
			Description: "Several recommended best practices are not being followed",
			Action:      "Implement recommended configuration best practices",
			Benefits:    []string{"Better performance", "Improved security", "Enhanced reliability"},
			Risks:       []string{"May require configuration changes"},
		})
	}

	return recommendations
}

func (tv *TemplateValidator) calculateBestPracticesAdherence(config *GatewayConfig) float64 {
	score := 0.0
	total := 0.0

	// Port best practices
	total += 1.0
	if config.Port >= 1024 && config.Port <= 65535 {
		score += 1.0
	}

	// Timeout best practices
	total += 1.0
	if config.Timeout != "" {
		if duration, err := time.ParseDuration(config.Timeout); err == nil {
			if duration >= time.Second && duration <= time.Hour {
				score += 1.0
			}
		}
	}

	// Concurrent requests best practices
	total += 1.0
	if config.MaxConcurrentRequests > 0 && config.MaxConcurrentRequests <= 1000 {
		score += 1.0
	}

	// Resource limits best practices
	total += 1.0
	if len(config.LanguagePools) == 0 || tv.allPoolsHaveResourceLimits(config) {
		score += 1.0
	}

	// Server configuration best practices
	for _, server := range config.Servers {
		total += 2.0 // 2 points per server

		if server.Transport != "" {
			score += 1.0
		}

		if len(server.RootMarkers) > 0 {
			score += 1.0
		}
	}

	if total > 0 {
		return score / total
	}
	return 1.0
}

func (tv *TemplateValidator) calculateNamingConsistency(config *GatewayConfig) float64 {
	total := 0
	consistent := 0

	// Check server naming consistency
	for _, server := range config.Servers {
		if server.Name != "" {
			total++
			if tv.isValidServerName(server.Name) {
				consistent++
			}
		}
	}

	if total > 0 {
		return float64(consistent) / float64(total)
	}
	return 1.0
}

func (tv *TemplateValidator) calculateStructureOrganization(config *GatewayConfig) float64 {
	score := 1.0

	// Penalize mixed configuration approaches
	if len(config.Servers) > 0 && len(config.LanguagePools) > 0 {
		score -= 0.3
	}

	// Penalize inconsistent smart router configuration
	if config.EnableSmartRouting && config.SmartRouterConfig == nil {
		score -= 0.4
	}

	// Penalize inconsistent project awareness configuration
	if config.ProjectAware && config.ProjectContext == nil && config.ProjectConfig == nil {
		score -= 0.2
	}

	return math.Max(0.0, score)
}

func (tv *TemplateValidator) allPoolsHaveResourceLimits(config *GatewayConfig) bool {
	for _, pool := range config.LanguagePools {
		if pool.ResourceLimits == nil {
			return false
		}
	}
	return true
}

func (tv *TemplateValidator) isValidServerName(name string) bool {
	// Check if name follows kebab-case convention and is reasonable
	validNamePattern := regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)
	return validNamePattern.MatchString(name) && len(name) <= 50
}
