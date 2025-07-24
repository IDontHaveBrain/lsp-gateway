package project

import (
	"context"
	"fmt"
	"log"
	"time"

	"lsp-gateway/internal/setup"
)

// ProjectStartupManager coordinates the startup of project-aware components
type ProjectStartupManager struct {
	integration *ProjectIntegration
	logger      *setup.SetupLogger
}

// NewProjectStartupManager creates a new project startup manager
func NewProjectStartupManager() *ProjectStartupManager {
	// Create logger for startup coordination
	logger := setup.NewSetupLogger(&setup.SetupLoggerConfig{
		Component: "project-startup-manager",
	})

	// Create project integration with default config
	integration, err := NewProjectIntegration(DefaultIntegrationConfig())
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"error": err.Error(),
		}).Error("Failed to create project integration")
		// Continue with nil integration - will fall back to non-project mode
		integration = nil
	}

	return &ProjectStartupManager{
		integration: integration,
		logger:      logger,
	}
}

// ProjectStartupResult contains the results of project-aware startup
type ProjectStartupResult struct {
	ProjectDetected bool
	ProjectResult   *ProjectAnalysisResult
	Error           error
}

// StartupConfig contains configuration for project-aware startup
type StartupConfig struct {
	AutoDetectProject     bool
	ProjectPath           string
	GenerateProjectConfig bool
	Timeout               time.Duration
	FallbackOnError       bool
}

// DefaultStartupConfig creates a default startup configuration
func DefaultStartupConfig() *StartupConfig {
	return &StartupConfig{
		AutoDetectProject:     false,
		ProjectPath:           "",
		GenerateProjectConfig: false,  
		Timeout:               60 * time.Second,
		FallbackOnError:       true,
	}
}

// PerformProjectStartup performs comprehensive project startup detection and analysis
func (psm *ProjectStartupManager) PerformProjectStartup(ctx context.Context, config *StartupConfig) *ProjectStartupResult {
	startTime := time.Now()
	
	if config == nil {
		config = DefaultStartupConfig()
	}

	psm.logger.WithFields(map[string]interface{}{
		"auto_detect":     config.AutoDetectProject,
		"project_path":    config.ProjectPath,
		"generate_config": config.GenerateProjectConfig,
		"timeout":         config.Timeout,
	}).Info("Starting project-aware startup process")

	result := &ProjectStartupResult{
		ProjectDetected: false,
		ProjectResult:   nil,
		Error:           nil,
	}

	// Skip if no project detection requested
	if !config.AutoDetectProject && config.ProjectPath == "" {
		psm.logger.Info("No project detection requested, skipping project-aware startup")
		return result
	}

	// Skip if integration is not available
	if psm.integration == nil {
		result.Error = fmt.Errorf("project integration not available")
		if config.FallbackOnError {
			psm.logger.Warn("Project integration unavailable, falling back to traditional mode")
			return result
		}
		return result
	}

	// Determine detection path
	detectionPath, err := psm.resolveProjectPath(config)
	if err != nil {
		result.Error = fmt.Errorf("failed to resolve project path: %w", err)
		if config.FallbackOnError {
			psm.logger.WithFields(map[string]interface{}{
				"error": err.Error(),
			}).Warn("Failed to resolve project path, falling back to traditional mode")
			return result
		}
		return result
	}

	// Create timeout context
	startupCtx, cancel := context.WithTimeout(ctx, config.Timeout)
	defer cancel()

	// Perform project detection and analysis
	psm.logger.WithFields(map[string]interface{}{
		"detection_path": detectionPath,
	}).Info("Performing project detection and analysis")

	projectResult, err := psm.integration.DetectAndAnalyzeProject(startupCtx, detectionPath)
	if err != nil {
		result.Error = fmt.Errorf("project detection failed: %w", err)
		if config.FallbackOnError {
			psm.logger.WithFields(map[string]interface{}{
				"error": err.Error(),
			}).Warn("Project detection failed, falling back to traditional mode")
			return result
		}
		return result
	}

	// Validate project detection results
	if projectResult == nil || projectResult.ProjectContext == nil {
		psm.logger.Info("No project detected, using traditional mode")
		return result
	}

	// Update result with successful detection
	result.ProjectDetected = true
	result.ProjectResult = projectResult

	duration := time.Since(startTime)
	psm.logger.WithFields(map[string]interface{}{
		"project_type":     projectResult.ProjectContext.ProjectType,
		"languages":        projectResult.ProjectContext.Languages,
		"confidence_level": projectResult.ConfidenceLevel,
		"duration":         duration,
		"status":           string(projectResult.Status),
	}).Info("Project-aware startup completed successfully")

	return result
}

// resolveProjectPath resolves the project path from startup configuration
func (psm *ProjectStartupManager) resolveProjectPath(config *StartupConfig) (string, error) {
	// Use explicit project path if provided
	if config.ProjectPath != "" {
		return config.ProjectPath, nil
	}

	// Use current working directory for auto-detection
	if config.AutoDetectProject {
		return ".", nil // Will be resolved to absolute path by project integration
	}

	return "", fmt.Errorf("no project path specified")
}

// ValidateProjectStartupResult validates the results of project startup
func ValidateProjectStartupResult(result *ProjectStartupResult) error {
	if result == nil {
		return fmt.Errorf("project startup result is nil")
	}

	// If error occurred and project not detected, it's a failure
	if result.Error != nil && !result.ProjectDetected {
		return fmt.Errorf("project startup failed: %w", result.Error)
	}

	// If project detected, ensure we have valid project result
	if result.ProjectDetected {
		if result.ProjectResult == nil {
			return fmt.Errorf("project detected but project result is nil")
		}
		if result.ProjectResult.ProjectContext == nil {
			return fmt.Errorf("project detected but project context is nil") 
		}
	}

	return nil
}

// CreateStartupConfigFromCLI creates startup config from CLI flags
func CreateStartupConfigFromCLI(autoDetect bool, projectPath string, generateConfig bool) *StartupConfig {
	return &StartupConfig{
		AutoDetectProject:     autoDetect,
		ProjectPath:           projectPath,
		GenerateProjectConfig: generateConfig,
		Timeout:               60 * time.Second,
		FallbackOnError:       true, // Always fallback for CLI usage
	}
}

// LogProjectStartupResult logs the results of project startup
func LogProjectStartupResult(result *ProjectStartupResult, component string) {
	if result == nil {
		log.Printf("[WARN] %s: Project startup result is nil\n", component)
		return
	}

	if result.Error != nil {
		log.Printf("[WARN] %s: Project startup encountered error: %v\n", component, result.Error)
	}

	if result.ProjectDetected {
		ctx := result.ProjectResult.ProjectContext
		log.Printf("[INFO] %s: Project detected - type=%s, languages=%v, path=%s\n", 
			component, ctx.ProjectType, ctx.Languages, ctx.RootPath)
	} else {
		log.Printf("[INFO] %s: No project detected, using traditional mode\n", component)
	}
}

// GetProjectIntegration returns the project integration instance for advanced usage
func (psm *ProjectStartupManager) GetProjectIntegration() *ProjectIntegration {
	return psm.integration
}

// IsProjectIntegrationAvailable checks if project integration is available
func (psm *ProjectStartupManager) IsProjectIntegrationAvailable() bool {
	return psm.integration != nil
}