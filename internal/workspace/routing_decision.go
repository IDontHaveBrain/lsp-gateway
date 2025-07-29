package workspace

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

type EnhancedRoutingDecision struct {
	Method          string                    `json:"method"`
	FileURI         string                    `json:"file_uri"`
	FileURIs        []string                  `json:"file_uris"`
	TargetProject   *SubProject              `json:"target_project"`
	Language        string                    `json:"language"`
	PrimaryClient   string                    `json:"primary_client"`
	FallbackClients []string                  `json:"fallback_clients"`
	Strategy        string                    `json:"strategy"`
	Timeout         time.Duration             `json:"timeout"`
	Context         map[string]interface{}    `json:"context"`
	CreatedAt       time.Time                `json:"created_at"`
}

type DecisionEngine interface {
	MakeRoutingDecision(ctx context.Context, method string, params interface{}) (*EnhancedRoutingDecision, error)
	DetermineLanguage(fileURI string, project *SubProject) (string, error)
	SelectStrategy(method string, project *SubProject) (string, error)
	CalculateTimeout(method string, project *SubProject) time.Duration
}

type decisionEngine struct {
	uriExtractor      URIExtractor
	subProjectResolver SubProjectResolver
	clientManager     SubProjectClientManager
	workspaceRoot     string
	strategyConfig    *RoutingStrategyConfig
}

type RoutingStrategyConfig struct {
	DefaultTimeout        time.Duration                `json:"default_timeout"`
	MethodTimeouts        map[string]time.Duration     `json:"method_timeouts"`
	LanguageTimeouts      map[string]time.Duration     `json:"language_timeouts"`
	StrategyPriorities    map[string][]string          `json:"strategy_priorities"`
	FallbackEnabled       bool                         `json:"fallback_enabled"`
	MaxFallbackClients    int                          `json:"max_fallback_clients"`
}

func NewDecisionEngine(
	uriExtractor URIExtractor,
	subProjectResolver SubProjectResolver,
	clientManager SubProjectClientManager,
	workspaceRoot string,
	strategyConfig *RoutingStrategyConfig,
) DecisionEngine {
	if strategyConfig == nil {
		strategyConfig = getDefaultStrategyConfig()
	}

	return &decisionEngine{
		uriExtractor:       uriExtractor,
		subProjectResolver: subProjectResolver,
		clientManager:      clientManager,
		workspaceRoot:      workspaceRoot,
		strategyConfig:     strategyConfig,
	}
}

func (de *decisionEngine) MakeRoutingDecision(ctx context.Context, method string, params interface{}) (*EnhancedRoutingDecision, error) {
	decision := &EnhancedRoutingDecision{
		Method:    method,
		CreatedAt: time.Now(),
		Context:   make(map[string]interface{}),
	}

	fileURI, err := de.uriExtractor.ExtractFileURI(method, params)
	if err != nil && !de.uriExtractor.IsWorkspaceMethod(method) {
		return nil, fmt.Errorf("failed to extract file URI: %w", err)
	}

	decision.FileURI = fileURI

	fileURIs, err := de.uriExtractor.ExtractAllFileURIs(method, params)
	if err != nil {
		return nil, fmt.Errorf("failed to extract all file URIs: %w", err)
	}
	decision.FileURIs = fileURIs

	if fileURI != "" {
		project, err := de.subProjectResolver.ResolveSubProjectCached(fileURI)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve sub-project for URI %s: %w", fileURI, err)
		}
		decision.TargetProject = project

		language, err := de.DetermineLanguage(fileURI, project)
		if err != nil {
			return nil, fmt.Errorf("failed to determine language: %w", err)
		}
		decision.Language = language

		strategy, err := de.SelectStrategy(method, project)
		if err != nil {
			return nil, fmt.Errorf("failed to select strategy: %w", err)
		}
		decision.Strategy = strategy

		primaryClient, fallbackClients, err := de.selectClients(project, language)
		if err != nil {
			return nil, fmt.Errorf("failed to select clients: %w", err)
		}
		decision.PrimaryClient = primaryClient
		decision.FallbackClients = fallbackClients
	} else {
		decision.Strategy = "workspace"
		decision.Language = "multi"
	}

	decision.Timeout = de.CalculateTimeout(method, decision.TargetProject)

	decision.Context["extraction_method"] = method
	decision.Context["workspace_root"] = de.workspaceRoot
	if decision.TargetProject != nil {
		decision.Context["project_type"] = decision.TargetProject.ProjectType
		decision.Context["project_root"] = decision.TargetProject.Root
	}

	return decision, nil
}

func (de *decisionEngine) DetermineLanguage(fileURI string, project *SubProject) (string, error) {
	if fileURI == "" {
		return "", fmt.Errorf("empty file URI")
	}

	if err := de.uriExtractor.ValidateURI(fileURI); err != nil {
		return "", fmt.Errorf("invalid URI: %w", err)
	}

	normalizedURI, err := de.uriExtractor.NormalizeURI(fileURI)
	if err != nil {
		return "", fmt.Errorf("failed to normalize URI: %w", err)
	}

	filePath := strings.TrimPrefix(normalizedURI, URIPrefixFile)
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == "" {
		return "", fmt.Errorf("cannot determine file type from URI: %s", fileURI)
	}

	ext = strings.TrimPrefix(ext, ".")

	if project != nil {
		for _, lang := range project.Languages {
			if langExtensions := getLanguageExtensions(lang); contains(langExtensions, ext) {
				return lang, nil
			}
		}
	}

	langExtMap := getGlobalExtensionMapping()
	if language, exists := langExtMap[ext]; exists {
		return language, nil
	}

	return "", fmt.Errorf("unsupported file extension: %s", ext)
}

func (de *decisionEngine) SelectStrategy(method string, project *SubProject) (string, error) {
	if project == nil {
		return "default", nil
	}

	projectStrategies, exists := de.strategyConfig.StrategyPriorities[project.ProjectType]
	if exists && len(projectStrategies) > 0 {
		return projectStrategies[0], nil
	}

	methodStrategies := map[string]string{
		"textDocument/definition":    "definition_first",
		"textDocument/references":    "reference_optimized",
		"textDocument/hover":         "hover_fast",
		"textDocument/documentSymbol": "symbol_cached",
		"textDocument/completion":    "completion_fast",
		"workspace/symbol":           "workspace_wide",
	}

	if strategy, exists := methodStrategies[method]; exists {
		return strategy, nil
	}

	return "default", nil
}

func (de *decisionEngine) CalculateTimeout(method string, project *SubProject) time.Duration {
	if methodTimeout, exists := de.strategyConfig.MethodTimeouts[method]; exists {
		return methodTimeout
	}

	if project != nil {
		for _, language := range project.Languages {
			if langTimeout, exists := de.strategyConfig.LanguageTimeouts[language]; exists {
				return langTimeout
			}
		}
	}

	methodTimeouts := map[string]time.Duration{
		"textDocument/definition":    5 * time.Second,
		"textDocument/references":    10 * time.Second,
		"textDocument/hover":         3 * time.Second,
		"textDocument/documentSymbol": 8 * time.Second,
		"textDocument/completion":    2 * time.Second,
		"workspace/symbol":           15 * time.Second,
	}

	if timeout, exists := methodTimeouts[method]; exists {
		return timeout
	}

	return de.strategyConfig.DefaultTimeout
}

func (de *decisionEngine) selectClients(project *SubProject, language string) (string, []string, error) {
	if project == nil {
		return "", nil, fmt.Errorf("project is nil")
	}

	primaryClientID := fmt.Sprintf("%s_%s", project.ID, language)

	var fallbackClients []string
	if de.strategyConfig.FallbackEnabled {
		for _, lang := range project.Languages {
			if lang != language {
				fallbackID := fmt.Sprintf("%s_%s", project.ID, lang)
				fallbackClients = append(fallbackClients, fallbackID)
				if len(fallbackClients) >= de.strategyConfig.MaxFallbackClients {
					break
				}
			}
		}
	}

	return primaryClientID, fallbackClients, nil
}

func getDefaultStrategyConfig() *RoutingStrategyConfig {
	return &RoutingStrategyConfig{
		DefaultTimeout:     10 * time.Second,
		MethodTimeouts:     make(map[string]time.Duration),
		LanguageTimeouts:   make(map[string]time.Duration),
		StrategyPriorities: make(map[string][]string),
		FallbackEnabled:    true,
		MaxFallbackClients: 2,
	}
}

func getLanguageExtensions(language string) []string {
	extensions := map[string][]string{
		"go":         {"go", "mod", "sum"},
		"python":     {"py", "pyx", "pyi", "pyw"},
		"javascript": {"js", "jsx", "mjs", "cjs"},
		"typescript": {"ts", "tsx", "d.ts"},
		"java":       {"java", "class", "jar"},
		"c":          {"c", "h"},
		"cpp":        {"cpp", "cc", "cxx", "c++", "hpp", "hxx", "h++"},
		"rust":       {"rs"},
		"csharp":     {"cs", "csx"},
	}

	if exts, exists := extensions[language]; exists {
		return exts
	}
	return []string{}
}

func getGlobalExtensionMapping() map[string]string {
	return map[string]string{
		"go":   "go",
		"mod":  "go",
		"sum":  "go",
		"py":   "python",
		"pyx":  "python",
		"pyi":  "python",
		"js":   "javascript",
		"jsx":  "javascript",
		"mjs":  "javascript",
		"ts":   "typescript",
		"tsx":  "typescript",
		"java": "java",
		"class": "java",
		"c":    "c",
		"h":    "c",
		"cpp":  "cpp",
		"cc":   "cpp",
		"cxx":  "cpp",
		"hpp":  "cpp",
		"rs":   "rust",
		"cs":   "csharp",
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}