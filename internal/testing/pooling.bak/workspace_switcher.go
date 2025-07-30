package pooling

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"lsp-gateway/internal/transport"
)

// WorkspaceSwitcher manages workspace switching for LSP servers
type WorkspaceSwitcher struct {
	client           transport.LSPClient
	currentWorkspace string
	switchStrategies map[string]WorkspaceSwitchStrategy
	logger           Logger
	mu               sync.RWMutex
	metrics          *WorkspaceSwitchMetrics
}

// Logger interface for workspace switcher logging
type Logger interface {
	Debug(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
}

// WorkspaceSwitchMetrics tracks workspace switching performance
type WorkspaceSwitchMetrics struct {
	TotalSwitches    int64         `json:"total_switches"`
	SuccessfulSwitches int64       `json:"successful_switches"`
	FailedSwitches   int64         `json:"failed_switches"`
	AverageSwitchTime time.Duration `json:"average_switch_time"`
	MaxSwitchTime    time.Duration `json:"max_switch_time"`
	LanguageStats    map[string]*LanguageSwitchStats `json:"language_stats"`
	mu               sync.RWMutex
}

// LanguageSwitchStats tracks switching performance per language
type LanguageSwitchStats struct {
	Language         string        `json:"language"`
	TotalSwitches    int64         `json:"total_switches"`
	SuccessfulSwitches int64       `json:"successful_switches"`
	FailedSwitches   int64         `json:"failed_switches"`
	AverageSwitchTime time.Duration `json:"average_switch_time"`
	LastSwitchTime   time.Time     `json:"last_switch_time"`
	LastSwitchSuccess bool         `json:"last_switch_success"`
}

// NewWorkspaceSwitcher creates a new workspace switcher with default strategies
func NewWorkspaceSwitcher(client transport.LSPClient, logger Logger) *WorkspaceSwitcher {
	ws := &WorkspaceSwitcher{
		client:           client,
		switchStrategies: make(map[string]WorkspaceSwitchStrategy),
		logger:           logger,
		metrics: &WorkspaceSwitchMetrics{
			LanguageStats: make(map[string]*LanguageSwitchStats),
		},
	}

	// Register default strategies
	ws.RegisterStrategy("java", NewJavaWorkspaceSwitchStrategy(logger))
	ws.RegisterStrategy("typescript", NewTypeScriptWorkspaceSwitchStrategy(logger))
	ws.RegisterStrategy("go", NewGoWorkspaceSwitchStrategy(logger))
	ws.RegisterStrategy("python", NewPythonWorkspaceSwitchStrategy(logger))

	return ws
}

// RegisterStrategy registers a custom workspace switching strategy for a language
func (ws *WorkspaceSwitcher) RegisterStrategy(language string, strategy WorkspaceSwitchStrategy) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	
	ws.switchStrategies[language] = strategy
	
	// Initialize metrics for this language
	ws.metrics.mu.Lock()
	if _, exists := ws.metrics.LanguageStats[language]; !exists {
		ws.metrics.LanguageStats[language] = &LanguageSwitchStats{
			Language: language,
		}
	}
	ws.metrics.mu.Unlock()
}

// SwitchWorkspace switches the LSP server to a new workspace
func (ws *WorkspaceSwitcher) SwitchWorkspace(ctx context.Context, newWorkspace string, language string) error {
	start := time.Now()
	
	ws.mu.Lock()
	oldWorkspace := ws.currentWorkspace
	ws.mu.Unlock()
	
	if oldWorkspace == newWorkspace {
		ws.logger.Debug("Workspace switch requested but already on target workspace: %s", newWorkspace)
		return nil
	}
	
	ws.logger.Info("Starting workspace switch: %s -> %s (language: %s)", oldWorkspace, newWorkspace, language)
	
	// Get the appropriate strategy
	strategy, err := ws.getStrategy(language)
	if err != nil {
		ws.recordSwitchFailure(language, time.Since(start))
		return fmt.Errorf("no workspace switching strategy for language %s: %w", language, err)
	}
	
	// Validate the workspace switch before attempting
	if err := ws.ValidateWorkspaceSwitch(oldWorkspace, newWorkspace); err != nil {
		ws.recordSwitchFailure(language, time.Since(start))
		return fmt.Errorf("workspace switch validation failed: %w", err)
	}
	
	// Check if the strategy can handle this switch
	if !strategy.CanSwitch(oldWorkspace, newWorkspace) {
		ws.recordSwitchFailure(language, time.Since(start))
		return fmt.Errorf("strategy for %s cannot switch from %s to %s", language, oldWorkspace, newWorkspace)
	}
	
	// Execute the workspace switch
	if err := ws.executeSwitchWithTimeout(ctx, strategy, oldWorkspace, newWorkspace, language); err != nil {
		ws.recordSwitchFailure(language, time.Since(start))
		return fmt.Errorf("workspace switch execution failed: %w", err)
	}
	
	// Update current workspace
	ws.mu.Lock()
	ws.currentWorkspace = newWorkspace
	ws.mu.Unlock()
	
	ws.recordSwitchSuccess(language, time.Since(start))
	ws.logger.Info("Workspace switch completed successfully: %s -> %s (took %v)", 
		oldWorkspace, newWorkspace, time.Since(start))
	
	return nil
}

// executeSwitchWithTimeout executes the workspace switch with proper timeout handling
func (ws *WorkspaceSwitcher) executeSwitchWithTimeout(ctx context.Context, strategy WorkspaceSwitchStrategy, 
	oldWorkspace, newWorkspace, language string) error {
	
	// Create timeout context (default 30s, configurable per language)
	switchTimeout := 30 * time.Second
	switchCtx, cancel := context.WithTimeout(ctx, switchTimeout)
	defer cancel()
	
	// Execute pre-switch phase
	ws.logger.Debug("Executing pre-switch phase for %s", language)
	if err := strategy.PreSwitch(ws.client, oldWorkspace, newWorkspace); err != nil {
		return fmt.Errorf("pre-switch failed: %w", err)
	}
	
	// Execute the main switch
	ws.logger.Debug("Executing main workspace switch for %s", language)
	if err := strategy.ExecuteSwitch(switchCtx, ws.client, oldWorkspace, newWorkspace); err != nil {
		return fmt.Errorf("switch execution failed: %w", err)
	}
	
	// Execute post-switch phase
	ws.logger.Debug("Executing post-switch phase for %s", language)
	if err := strategy.PostSwitch(ws.client, oldWorkspace, newWorkspace); err != nil {
		return fmt.Errorf("post-switch failed: %w", err)
	}
	
	// Validate the switch was successful
	ws.logger.Debug("Validating workspace switch for %s", language)
	if err := strategy.ValidateSwitch(switchCtx, ws.client, newWorkspace); err != nil {
		return fmt.Errorf("switch validation failed: %w", err)
	}
	
	return nil
}

// GetCurrentWorkspace returns the current workspace
func (ws *WorkspaceSwitcher) GetCurrentWorkspace() string {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	return ws.currentWorkspace
}

// CanSwitchWorkspace checks if workspace switching is supported for a language
func (ws *WorkspaceSwitcher) CanSwitchWorkspace(language string) bool {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	
	strategy, exists := ws.switchStrategies[language]
	return exists && strategy != nil
}

// ValidateWorkspaceSwitch validates that a workspace switch is valid
func (ws *WorkspaceSwitcher) ValidateWorkspaceSwitch(oldWS, newWS string) error {
	if newWS == "" {
		return fmt.Errorf("new workspace cannot be empty")
	}
	
	// Convert to absolute paths for comparison
	absOld, err := filepath.Abs(oldWS)
	if err != nil && oldWS != "" {
		ws.logger.Warn("Could not resolve absolute path for old workspace %s: %v", oldWS, err)
		absOld = oldWS
	}
	
	absNew, err := filepath.Abs(newWS)
	if err != nil {
		return fmt.Errorf("could not resolve absolute path for new workspace %s: %w", newWS, err)
	}
	
	if absOld == absNew {
		return fmt.Errorf("old and new workspace are the same: %s", absNew)
	}
	
	// Basic workspace validation using the dedicated validator
	validator := NewWorkspaceValidator(ws.logger)
	return validator.ValidateWorkspace(newWS)
}

// getStrategy retrieves the workspace switching strategy for a language
func (ws *WorkspaceSwitcher) getStrategy(language string) (WorkspaceSwitchStrategy, error) {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	
	strategy, exists := ws.switchStrategies[language]
	if !exists {
		return nil, fmt.Errorf("no workspace switching strategy registered for language: %s", language)
	}
	
	return strategy, nil
}

// recordSwitchSuccess records a successful workspace switch in metrics
func (ws *WorkspaceSwitcher) recordSwitchSuccess(language string, duration time.Duration) {
	ws.metrics.mu.Lock()
	defer ws.metrics.mu.Unlock()
	
	// Update global metrics
	ws.metrics.TotalSwitches++
	ws.metrics.SuccessfulSwitches++
	
	// Update average time
	if ws.metrics.TotalSwitches == 1 {
		ws.metrics.AverageSwitchTime = duration
	} else {
		ws.metrics.AverageSwitchTime = time.Duration(
			(int64(ws.metrics.AverageSwitchTime)*ws.metrics.TotalSwitches + int64(duration)) / 
			(ws.metrics.TotalSwitches + 1))
	}
	
	// Update max time
	if duration > ws.metrics.MaxSwitchTime {
		ws.metrics.MaxSwitchTime = duration
	}
	
	// Update language-specific metrics
	langStats := ws.metrics.LanguageStats[language]
	langStats.TotalSwitches++
	langStats.SuccessfulSwitches++
	langStats.LastSwitchTime = time.Now()
	langStats.LastSwitchSuccess = true
	
	// Update language average time
	if langStats.TotalSwitches == 1 {
		langStats.AverageSwitchTime = duration
	} else {
		langStats.AverageSwitchTime = time.Duration(
			(int64(langStats.AverageSwitchTime)*langStats.TotalSwitches + int64(duration)) / 
			(langStats.TotalSwitches + 1))
	}
}

// recordSwitchFailure records a failed workspace switch in metrics
func (ws *WorkspaceSwitcher) recordSwitchFailure(language string, duration time.Duration) {
	ws.metrics.mu.Lock()
	defer ws.metrics.mu.Unlock()
	
	// Update global metrics
	ws.metrics.TotalSwitches++
	ws.metrics.FailedSwitches++
	
	// Update language-specific metrics
	langStats := ws.metrics.LanguageStats[language]
	langStats.TotalSwitches++
	langStats.FailedSwitches++
	langStats.LastSwitchTime = time.Now()
	langStats.LastSwitchSuccess = false
}

// GetMetrics returns current workspace switching metrics
func (ws *WorkspaceSwitcher) GetMetrics() *WorkspaceSwitchMetrics {
	ws.metrics.mu.RLock()
	defer ws.metrics.mu.RUnlock()
	
	// Create a deep copy of metrics to avoid race conditions
	result := &WorkspaceSwitchMetrics{
		TotalSwitches:      ws.metrics.TotalSwitches,
		SuccessfulSwitches: ws.metrics.SuccessfulSwitches,
		FailedSwitches:     ws.metrics.FailedSwitches,
		AverageSwitchTime:  ws.metrics.AverageSwitchTime,
		MaxSwitchTime:      ws.metrics.MaxSwitchTime,
		LanguageStats:      make(map[string]*LanguageSwitchStats),
	}
	
	for lang, stats := range ws.metrics.LanguageStats {
		result.LanguageStats[lang] = &LanguageSwitchStats{
			Language:          stats.Language,
			TotalSwitches:     stats.TotalSwitches,
			SuccessfulSwitches: stats.SuccessfulSwitches,
			FailedSwitches:    stats.FailedSwitches,
			AverageSwitchTime: stats.AverageSwitchTime,
			LastSwitchTime:    stats.LastSwitchTime,
			LastSwitchSuccess: stats.LastSwitchSuccess,
		}
	}
	
	return result
}

// GetSuccessRate returns the overall workspace switching success rate
func (ws *WorkspaceSwitcher) GetSuccessRate() float64 {
	ws.metrics.mu.RLock()
	defer ws.metrics.mu.RUnlock()
	
	if ws.metrics.TotalSwitches == 0 {
		return 0.0
	}
	
	return float64(ws.metrics.SuccessfulSwitches) / float64(ws.metrics.TotalSwitches)
}

// GetLanguageSuccessRate returns the workspace switching success rate for a specific language
func (ws *WorkspaceSwitcher) GetLanguageSuccessRate(language string) float64 {
	ws.metrics.mu.RLock()
	defer ws.metrics.mu.RUnlock()
	
	langStats, exists := ws.metrics.LanguageStats[language]
	if !exists || langStats.TotalSwitches == 0 {
		return 0.0
	}
	
	return float64(langStats.SuccessfulSwitches) / float64(langStats.TotalSwitches)
}

// Reset clears all workspace switching metrics
func (ws *WorkspaceSwitcher) Reset() {
	ws.metrics.mu.Lock()
	defer ws.metrics.mu.Unlock()
	
	ws.metrics.TotalSwitches = 0
	ws.metrics.SuccessfulSwitches = 0
	ws.metrics.FailedSwitches = 0
	ws.metrics.AverageSwitchTime = 0
	ws.metrics.MaxSwitchTime = 0
	
	for lang := range ws.metrics.LanguageStats {
		ws.metrics.LanguageStats[lang] = &LanguageSwitchStats{
			Language: lang,
		}
	}
}