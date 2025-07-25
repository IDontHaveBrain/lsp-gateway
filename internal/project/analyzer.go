package project

import (
	"context"
	"fmt"
	"lsp-gateway/internal/config"
	"lsp-gateway/internal/setup"
	"os"
	"path/filepath"
	"time"
)

type ProjectAnalyzerImpl struct {
	languageDetector *BasicLanguageDetector
	configGenerator  *ProjectConfigGeneratorImpl
	logger           *setup.SetupLogger
	timeout          time.Duration
}

type AnalysisResult struct {
	ProjectContext   *ProjectContext                `json:"project_context"`
	ProjectConfig    *config.ProjectConfig          `json:"project_config,omitempty"`
	ConfigGeneration *ProjectConfigGenerationResult `json:"config_generation,omitempty"`
	AnalysisTime     time.Duration                  `json:"analysis_time"`
	Messages         []string                       `json:"messages"`
	Warnings         []string                       `json:"warnings"`
	Errors           []string                       `json:"errors"`
	AnalyzedAt       time.Time                      `json:"analyzed_at"`
}

func NewProjectAnalyzerImpl() *ProjectAnalyzerImpl {
	logger := setup.NewSetupLogger(&setup.SetupLoggerConfig{Component: "project-analyzer"})
	registry := setup.NewDefaultServerRegistry()
	verifier := setup.NewDefaultServerVerifier()
	return &ProjectAnalyzerImpl{
		languageDetector: NewBasicLanguageDetector(),
		configGenerator:  NewProjectConfigGenerator(logger, registry, verifier),
		logger:           logger,
		timeout:          30 * time.Second,
	}
}

func (a *ProjectAnalyzerImpl) AnalyzeProject(ctx context.Context, rootPath string) (*AnalysisResult, error) {
	startTime := time.Now()
	result := &AnalysisResult{Messages: []string{}, Warnings: []string{}, Errors: []string{}, AnalyzedAt: startTime}

	analysisCtx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()

	projectContext, err := a.detectProject(analysisCtx, rootPath)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Project detection failed: %v", err))
		result.AnalysisTime = time.Since(startTime)
		return result, err
	}

	result.ProjectContext = projectContext
	result.Messages = append(result.Messages, fmt.Sprintf("Detected %s project with %d languages",
		projectContext.ProjectType, len(projectContext.Languages)))

	projectType := a.DetermineProjectType(projectContext)
	result.Messages = append(result.Messages, fmt.Sprintf("Project classified as: %s", projectType))

	if len(projectContext.RequiredServers) > 0 {
		if configResult, err := a.GenerateConfiguration(analysisCtx, projectContext); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Configuration generation failed: %v", err))
		} else {
			result.ConfigGeneration = configResult
			if configResult.ProjectConfig != nil {
				result.ProjectConfig = configResult.ProjectConfig
			}
			result.Messages = append(result.Messages,
				fmt.Sprintf("Generated configuration with %d servers", configResult.ServersGenerated))
		}
	}

	result.AnalysisTime = time.Since(startTime)
	return result, nil
}

func (a *ProjectAnalyzerImpl) DetermineProjectType(projectContext *ProjectContext) string {
	if projectContext == nil {
		return "unknown"
	}
	if projectContext.IsMonorepo || len(projectContext.SubProjects) > 0 {
		return "monorepo"
	}
	if len(projectContext.Languages) > 1 {
		return "multi-language"
	}
	if len(projectContext.Languages) == 1 {
		return "single-language"
	}
	return "unknown"
}

func (a *ProjectAnalyzerImpl) GenerateConfiguration(ctx context.Context, projectContext *ProjectContext) (*ProjectConfigGenerationResult, error) {
	if a.configGenerator == nil {
		return nil, fmt.Errorf("configuration generator not available")
	}
	return a.configGenerator.GenerateFromProject(ctx, projectContext)
}

func (a *ProjectAnalyzerImpl) detectProject(ctx context.Context, rootPath string) (*ProjectContext, error) {
	detectionResult, err := a.languageDetector.DetectLanguage(ctx, rootPath)
	if err != nil {
		return nil, fmt.Errorf("language detection failed: %w", err)
	}

	projectContext := NewProjectContext(detectionResult.Language, rootPath)
	projectContext.AddLanguage(detectionResult.Language)
	projectContext.SetConfidence(detectionResult.Confidence)
	projectContext.MarkerFiles = detectionResult.MarkerFiles
	projectContext.ConfigFiles = detectionResult.ConfigFiles
	projectContext.SourceDirs = detectionResult.SourceDirs
	projectContext.RequiredServers = detectionResult.RequiredServers
	projectContext.Dependencies = detectionResult.Dependencies

	for key, value := range detectionResult.Metadata {
		projectContext.SetMetadata(key, value)
	}

	projectContext.WorkspaceRoot = findWorkspaceRoot(rootPath)
	projectContext.IsMonorepo = isMonorepo(rootPath)
	projectContext.IsValid = a.validateProjectStructure(projectContext)

	return projectContext, nil
}

func (a *ProjectAnalyzerImpl) validateProjectStructure(projectContext *ProjectContext) bool {
	if projectContext.ProjectType == PROJECT_TYPE_UNKNOWN {
		projectContext.AddValidationError("Unknown project type")
		return false
	}
	if len(projectContext.MarkerFiles) == 0 {
		projectContext.AddValidationWarning("No project marker files found")
	}
	return len(projectContext.ValidationErrors) == 0
}

func (a *ProjectAnalyzerImpl) SetLogger(logger *setup.SetupLogger) {
	if logger != nil {
		a.logger = logger
		if a.configGenerator != nil {
			a.configGenerator.SetLogger(logger)
		}
	}
}
func (a *ProjectAnalyzerImpl) SetTimeout(timeout time.Duration) {
	if timeout > 0 {
		a.timeout = timeout
	}
}
func findWorkspaceRoot(projectPath string) string {
	for current := projectPath; current != filepath.Dir(current); current = filepath.Dir(current) {
		if exists(filepath.Join(current, ".git")) {
			return current
		}
	}
	return projectPath
}

func isMonorepo(rootPath string) bool {
	for _, marker := range []string{"lerna.json", "nx.json", "rush.json", "pnpm-workspace.yaml"} {
		if exists(filepath.Join(rootPath, marker)) {
			return true
		}
	}
	return false
}
func exists(path string) bool { _, err := os.Stat(path); return err == nil }
