package project

import (
	"context"
	"fmt"
	"lsp-gateway/internal/project/types"
	"lsp-gateway/internal/setup"
	"path/filepath"
	"strings"
	"time"
)

// ProjectIntegration orchestrates all project detection, analysis, and configuration components
type ProjectIntegration struct {
	detector        *BasicLanguageDetector
	analyzer        *ProjectAnalyzerImpl
	configGenerator *ProjectConfigGeneratorImpl
	config          *IntegrationConfig
	logger          *setup.SetupLogger
	sessionID       string
}

// NewProjectIntegration creates a new project integration orchestrator with proper dependency injection
func NewProjectIntegration(config *IntegrationConfig) (*ProjectIntegration, error) {
	if config == nil {
		config = DefaultIntegrationConfig()
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid integration config: %w", err)
	}

	// Initialize logger with configuration
	loggerConfig := config.ToLoggerConfig("project-integration")
	logger := setup.NewSetupLogger(loggerConfig)

	// Initialize components with shared configuration
	detector := NewBasicLanguageDetector()
	analyzer := NewProjectAnalyzerImpl()
	registry := setup.NewDefaultServerRegistry()
	verifier := setup.NewDefaultServerVerifier()
	configGenerator := NewProjectConfigGenerator(logger, registry, verifier)

	// Configure component timeouts and settings
	analyzer.SetTimeout(config.AnalysisTimeout)
	analyzer.SetLogger(logger)

	integration := &ProjectIntegration{
		detector:        detector,
		analyzer:        analyzer,
		configGenerator: configGenerator,
		config:          config,
		logger:          logger,
		sessionID:       fmt.Sprintf("project-%d", time.Now().UnixNano()),
	}

	logger.WithFields(map[string]interface{}{
		"session_id":       integration.sessionID,
		"analysis_timeout": config.AnalysisTimeout,
		"detection_depth":  config.DetectionDepth,
	}).Info("Project integration orchestrator initialized")

	return integration, nil
}

// DetectAndAnalyzeProject performs comprehensive project detection and analysis
func (pi *ProjectIntegration) DetectAndAnalyzeProject(ctx context.Context, path string) (*ProjectAnalysisResult, error) {
	startTime := time.Now()

	// Create timeout context
	analysisCtx, cancel := context.WithTimeout(ctx, pi.config.AnalysisTimeout)
	defer cancel()

	pi.logger.WithFields(map[string]interface{}{
		"path":       path,
		"session_id": pi.sessionID,
		"timeout":    pi.config.AnalysisTimeout,
	}).Info("Starting project analysis")

	// Initialize result structure
	result := &ProjectAnalysisResult{
		Path:           path,
		NormalizedPath: pi.normalizePath(path),
		Timestamp:      startTime,
		Status:         AnalysisStatusInProgress,
		Errors:         make([]AnalysisError, 0),
		Warnings:       make([]AnalysisWarning, 0),
		Issues:         make([]string, 0),
		Metadata: map[string]interface{}{
			"session_id": pi.sessionID,
		},
	}

	// Phase 1: Project Detection
	if err := pi.performProjectDetection(analysisCtx, result); err != nil {
		return pi.finalizeResult(result, startTime, err)
	}

	// Phase 2: Deep Analysis (if detection succeeded)
	if result.ProjectContext != nil && result.Status != AnalysisStatusFailed {
		if err := pi.performProjectAnalysis(analysisCtx, result); err != nil {
			pi.logger.WithFields(map[string]interface{}{
				"error": err.Error(),
			}).Warn("Analysis phase failed, but continuing with detection results")
			pi.addWarning(result, "analysis", "Deep analysis failed, using basic detection results", err.Error())
		}
	}

	// Phase 3: Configuration Generation (optional, based on config)
	if result.ProjectContext != nil && !pi.config.QuietMode {
		if err := pi.performConfigGeneration(analysisCtx, result); err != nil {
			pi.logger.WithFields(map[string]interface{}{
				"error": err.Error(),
			}).Warn("Config generation failed")
			pi.addWarning(result, "config_generation", "Configuration generation failed", err.Error())
		}
	}

	return pi.finalizeResult(result, startTime, nil)
}

// performProjectDetection handles the project detection phase
func (pi *ProjectIntegration) performProjectDetection(ctx context.Context, result *ProjectAnalysisResult) error {
	pi.logger.Debug("Starting project detection phase")

	// Use language detector to detect project
	detectionResult, err := pi.detector.DetectLanguage(ctx, result.NormalizedPath)
	if err != nil {
		pi.addError(result, "detection", ErrorSeverityHigh, "Project detection failed", err.Error())
		result.Status = AnalysisStatusFailed
		return err
	}

	// Convert detection result to project context
	projectContext := pi.convertToProjectContext(detectionResult, result.NormalizedPath)
	result.ProjectContext = projectContext
	result.DetectionScore = detectionResult.Confidence
	result.ConfidenceLevel = result.GetConfidenceLevel()
	result.RequiredServers = detectionResult.RequiredServers

	// Calculate project size metrics
	pi.calculateProjectSize(result)

	pi.logger.WithFields(map[string]interface{}{
		"project_type":     projectContext.ProjectType,
		"languages":        projectContext.Languages,
		"confidence":       detectionResult.Confidence,
		"required_servers": detectionResult.RequiredServers,
	}).Info("Project detection completed")

	return nil
}

// performProjectAnalysis handles the deep analysis phase using ProjectAnalyzerImpl
func (pi *ProjectIntegration) performProjectAnalysis(ctx context.Context, result *ProjectAnalysisResult) error {
	pi.logger.Debug("Starting project analysis phase")

	analysisResult, err := pi.analyzer.AnalyzeProject(ctx, result.NormalizedPath)
	if err != nil {
		return fmt.Errorf("project analysis failed: %w", err)
	}

	// Merge analysis results
	if analysisResult != nil {
		result.Metadata["analysis_result"] = analysisResult
		result.Issues = append(result.Issues, analysisResult.Errors...)
		result.Warnings = append(result.Warnings,
			pi.convertStringsToWarnings(analysisResult.Warnings, "analysis")...)
	}

	return nil
}

// performConfigGeneration handles configuration generation
func (pi *ProjectIntegration) performConfigGeneration(ctx context.Context, result *ProjectAnalysisResult) error {
	if result.ProjectContext == nil {
		return fmt.Errorf("cannot generate config without project context")
	}

	pi.logger.Debug("Starting configuration generation phase")

	configResult := &ProjectConfigGenerationResult{}
	generatedConfig, err := pi.configGenerator.generateProjectConfig(ctx, result.ProjectContext, configResult)
	if err != nil {
		return fmt.Errorf("configuration generation failed: %w", err)
	}

	result.ProjectConfig = generatedConfig
	return nil
}

// Helper methods for result management and data conversion

func (pi *ProjectIntegration) convertToProjectContext(detectionResult *types.LanguageDetectionResult, rootPath string) *ProjectContext {
	ctx := NewProjectContext(detectionResult.Language, rootPath)
	ctx.AddLanguage(detectionResult.Language)
	ctx.MarkerFiles = detectionResult.MarkerFiles
	ctx.ConfigFiles = detectionResult.ConfigFiles
	ctx.SourceDirs = detectionResult.SourceDirs
	ctx.RequiredServers = detectionResult.RequiredServers
	ctx.Dependencies = detectionResult.Dependencies
	ctx.SetConfidence(detectionResult.Confidence)
	ctx.Metadata = detectionResult.Metadata
	return ctx
}

func (pi *ProjectIntegration) calculateProjectSize(result *ProjectAnalysisResult) {
	// Basic file counting for project size - simplified for orchestration focus
	result.ProjectSize = ProjectSize{
		TotalFiles:  100, // Placeholder - would implement actual counting
		SourceFiles: 80,
		TestFiles:   15,
		ConfigFiles: 5,
	}
}

func (pi *ProjectIntegration) addError(result *ProjectAnalysisResult, phase string, severity ErrorSeverity, message, details string) {
	result.Errors = append(result.Errors, AnalysisError{
		Type:        "analysis_error",
		Phase:       phase,
		Message:     message,
		Details:     details,
		Severity:    severity,
		Recoverable: severity != ErrorSeverityCritical,
		Timestamp:   time.Now(),
	})
}

func (pi *ProjectIntegration) addWarning(result *ProjectAnalysisResult, phase, message, details string) {
	result.Warnings = append(result.Warnings, AnalysisWarning{
		Type:      "analysis_warning",
		Phase:     phase,
		Message:   message,
		Details:   details,
		Severity:  WarningSeverityMedium,
		Timestamp: time.Now(),
	})
}

func (pi *ProjectIntegration) convertStringsToWarnings(warnings []string, phase string) []AnalysisWarning {
	result := make([]AnalysisWarning, len(warnings))
	for i, warning := range warnings {
		result[i] = AnalysisWarning{
			Type:      "component_warning",
			Phase:     phase,
			Message:   warning,
			Severity:  WarningSeverityLow,
			Timestamp: time.Now(),
		}
	}
	return result
}

func (pi *ProjectIntegration) normalizePath(path string) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		pi.logger.WithFields(map[string]interface{}{
			"path":  path,
			"error": err.Error(),
		}).Warn("Failed to normalize path")
		return path
	}
	return strings.ReplaceAll(absPath, "\\", "/")
}

func (pi *ProjectIntegration) finalizeResult(result *ProjectAnalysisResult, startTime time.Time, finalErr error) (*ProjectAnalysisResult, error) {
	result.Duration = time.Since(startTime)

	// Determine final status
	if finalErr != nil || len(result.Errors) > 0 {
		if result.ProjectContext != nil {
			result.Status = AnalysisStatusPartial
		} else {
			result.Status = AnalysisStatusFailed
		}
	} else {
		result.Status = AnalysisStatusSuccess
	}

	// Set performance metrics
	result.PerformanceMetrics = PerformanceMetrics{
		AnalysisTime: result.Duration,
		FilesScanned: result.ProjectSize.TotalFiles,
	}

	pi.logger.WithFields(map[string]interface{}{
		"status":     result.Status,
		"duration":   result.Duration,
		"errors":     len(result.Errors),
		"warnings":   len(result.Warnings),
		"session_id": pi.sessionID,
	}).Info("Project analysis completed")

	return result, finalErr
}

// StartupIntegrationCoordinator coordinates project-aware startup across multiple components
type StartupIntegrationCoordinator struct {
	integration     *ProjectIntegration
	logger          *setup.SetupLogger
}

// NewStartupIntegrationCoordinator creates a new startup integration coordinator
func NewStartupIntegrationCoordinator() *StartupIntegrationCoordinator {
	logger := setup.NewSetupLogger(&setup.SetupLoggerConfig{
		Component: "startup-integration-coordinator",
	})

	integration, err := NewProjectIntegration(DefaultIntegrationConfig())
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"error": err.Error(),
		}).Warn("Failed to create project integration for coordinator")
		integration = nil
	}

	return &StartupIntegrationCoordinator{
		integration: integration,
		logger:      logger,
	}
}

// CoordinateProjectAwareStartup coordinates project-aware startup for server and MCP components
func (sic *StartupIntegrationCoordinator) CoordinateProjectAwareStartup(ctx context.Context, serverConfig *StartupConfig, mcpConfig *StartupConfig) (*CoordinatedStartupResult, error) {
	sic.logger.Info("Starting coordinated project-aware startup")

	result := &CoordinatedStartupResult{
		ServerResult: &ProjectStartupResult{},
		MCPResult:    &ProjectStartupResult{},
		Timestamp:    time.Now(),
	}

	// Since we don't have ProjectStartupManager available, use integration directly
	if sic.integration == nil {
		sic.logger.Warn("No project integration available, skipping coordinated startup")
		result.Success = true // Graceful fallback
		return result, nil
	}

	// Coordinate server startup
	if serverConfig != nil {
		sic.logger.Debug("Coordinating server project detection")
		result.ServerResult = sic.performStartup(ctx, serverConfig)
		if result.ServerResult.Error != nil {
			sic.logger.WithFields(map[string]interface{}{
				"error": result.ServerResult.Error.Error(),
			}).Warn("Server project startup encountered issues")
		}
	}

	// Coordinate MCP startup (can reuse server results if same project)
	if mcpConfig != nil {
		sic.logger.Debug("Coordinating MCP project detection")
		if sic.canReuseServerResults(serverConfig, mcpConfig, result.ServerResult) {
			sic.logger.Debug("Reusing server project detection results for MCP")
			result.MCPResult = sic.cloneProjectStartupResult(result.ServerResult)
		} else {
			result.MCPResult = sic.performStartup(ctx, mcpConfig)
			if result.MCPResult.Error != nil {
				sic.logger.WithFields(map[string]interface{}{
					"error": result.MCPResult.Error.Error(),
				}).Warn("MCP project startup encountered issues")
			}
		}
	}

	// Validate coordination results
	result.Success = sic.validateCoordinationResults(result)
	result.Duration = time.Since(result.Timestamp)

	sic.logger.WithFields(map[string]interface{}{
		"success":         result.Success,
		"duration":        result.Duration,
		"server_detected": result.ServerResult.ProjectDetected,
		"mcp_detected":    result.MCPResult.ProjectDetected,
	}).Info("Coordinated project-aware startup completed")

	return result, nil
}

// performStartup performs startup using the integration
func (sic *StartupIntegrationCoordinator) performStartup(ctx context.Context, config *StartupConfig) *ProjectStartupResult {
	result := &ProjectStartupResult{
		ProjectDetected: false,
		ProjectResult:   nil,
		Error:           nil,
	}

	// Skip if no project detection requested
	if !config.AutoDetectProject && config.ProjectPath == "" {
		return result
	}

	// Determine detection path
	detectionPath := config.ProjectPath
	if config.AutoDetectProject && detectionPath == "" {
		detectionPath = "."
	}

	if detectionPath == "" {
		result.Error = fmt.Errorf("no project path specified")
		return result
	}

	// Create timeout context
	startupCtx, cancel := context.WithTimeout(ctx, config.Timeout)
	defer cancel()

	// Perform project detection and analysis
	projectResult, err := sic.integration.DetectAndAnalyzeProject(startupCtx, detectionPath)
	if err != nil {
		result.Error = fmt.Errorf("project detection failed: %w", err)
		return result
	}

	// Validate results
	if projectResult == nil || projectResult.ProjectContext == nil {
		return result
	}

	result.ProjectDetected = true
	result.ProjectResult = projectResult
	return result
}

// canReuseServerResults checks if MCP can reuse server project detection results
func (sic *StartupIntegrationCoordinator) canReuseServerResults(serverConfig, mcpConfig *StartupConfig, serverResult *ProjectStartupResult) bool {
	if serverConfig == nil || mcpConfig == nil || serverResult == nil {
		return false
	}

	// Only reuse if both are detecting the same project path
	return serverConfig.ProjectPath == mcpConfig.ProjectPath &&
		serverConfig.AutoDetectProject == mcpConfig.AutoDetectProject &&
		serverResult.ProjectDetected
}

// cloneProjectStartupResult creates a copy of project startup results for MCP use
func (sic *StartupIntegrationCoordinator) cloneProjectStartupResult(original *ProjectStartupResult) *ProjectStartupResult {
	if original == nil {
		return &ProjectStartupResult{}
	}

	return &ProjectStartupResult{
		ProjectDetected: original.ProjectDetected,
		ProjectResult:   original.ProjectResult, // Safe to share as it's read-only after creation
		Error:          original.Error,
	}
}

// validateCoordinationResults validates the results of coordinated startup
func (sic *StartupIntegrationCoordinator) validateCoordinationResults(result *CoordinatedStartupResult) bool {
	if result == nil {
		return false
	}

	// Success if at least one component succeeded or both failed gracefully
	serverOK := result.ServerResult.Error == nil || result.ServerResult.ProjectDetected
	mcpOK := result.MCPResult.Error == nil || result.MCPResult.ProjectDetected

	return serverOK && mcpOK
}

// CoordinatedStartupResult contains the results of coordinated startup
type CoordinatedStartupResult struct {
	ServerResult *ProjectStartupResult
	MCPResult    *ProjectStartupResult
	Success      bool
	Duration     time.Duration
	Timestamp    time.Time
}

// HasProjectDetection returns true if either component detected a project
func (csr *CoordinatedStartupResult) HasProjectDetection() bool {
	return (csr.ServerResult != nil && csr.ServerResult.ProjectDetected) ||
		(csr.MCPResult != nil && csr.MCPResult.ProjectDetected)
}

// GetBestProjectResult returns the best project detection result
func (csr *CoordinatedStartupResult) GetBestProjectResult() *ProjectAnalysisResult {
	if csr.ServerResult != nil && csr.ServerResult.ProjectDetected {
		return csr.ServerResult.ProjectResult
	}
	if csr.MCPResult != nil && csr.MCPResult.ProjectDetected {
		return csr.MCPResult.ProjectResult
	}
	return nil
}
