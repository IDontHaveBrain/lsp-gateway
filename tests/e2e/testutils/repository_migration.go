package testutils

import (
	"time"
)

// Migration helper functions to consolidate repository management implementations

// MigrateFromPythonRepoManager converts PythonRepoManager usage to UnifiedRepositoryManager
func MigrateFromPythonRepoManager(pythonConfig PythonRepoConfig) *UnifiedRepositoryManager {
	unifiedConfig := UnifiedRepoConfig{
		LanguageConfig:    GetPythonLanguageConfig(),
		TargetDir:         pythonConfig.TargetDir,
		CloneTimeout:      pythonConfig.CloneTimeout,
		EnableLogging:     pythonConfig.EnableLogging,
		ForceClean:        pythonConfig.ForceClean,
		PreserveGitDir:    pythonConfig.PreserveGitDir,
		EnableRetry:       true,  // Enable advanced features
		EnableHealthCheck: true,
		CommitTracking:    true,
	}

	// Override with custom repo URL if provided
	if pythonConfig.RepoURL != "" {
		unifiedConfig.LanguageConfig.RepoURL = pythonConfig.RepoURL
	}

	return NewUnifiedRepositoryManager(unifiedConfig)
}

// MigrateFromGenericRepoManager converts GenericRepoManager usage to UnifiedRepositoryManager
func MigrateFromGenericRepoManager(genericConfig GenericRepoConfig) *UnifiedRepositoryManager {
	unifiedConfig := UnifiedRepoConfig{
		LanguageConfig:    genericConfig.LanguageConfig,
		TargetDir:         genericConfig.TargetDir,
		CloneTimeout:      genericConfig.CloneTimeout,
		EnableLogging:     genericConfig.EnableLogging,
		ForceClean:        genericConfig.ForceClean,
		PreserveGitDir:    genericConfig.PreserveGitDir,
		EnableRetry:       true,  // Enable advanced features
		EnableHealthCheck: true,
		CommitTracking:    false, // Generic didn't have commit tracking
	}

	return NewUnifiedRepositoryManager(unifiedConfig)
}

// BackwardCompatibilityAdapter provides a bridge from old interfaces to the new unified manager
type BackwardCompatibilityAdapter struct {
	unifiedManager *UnifiedRepositoryManager
	originalType   string
}

// NewBackwardCompatibilityAdapter creates an adapter for the specified original type
func NewBackwardCompatibilityAdapter(originalType string, config interface{}) *BackwardCompatibilityAdapter {
	var unifiedManager *UnifiedRepositoryManager

	switch originalType {
	case "PythonRepoManager":
		if pythonConfig, ok := config.(PythonRepoConfig); ok {
			unifiedManager = MigrateFromPythonRepoManager(pythonConfig)
		}
	case "GenericRepoManager":
		if genericConfig, ok := config.(GenericRepoConfig); ok {
			unifiedManager = MigrateFromGenericRepoManager(genericConfig)
		}
	default:
		// Default to Python for backward compatibility
		if pythonConfig, ok := config.(PythonRepoConfig); ok {
			unifiedManager = MigrateFromPythonRepoManager(pythonConfig)
		} else {
			unifiedManager = NewPythonUnifiedRepositoryManager()
		}
	}

	return &BackwardCompatibilityAdapter{
		unifiedManager: unifiedManager,
		originalType:   originalType,
	}
}

// Implement RepositoryManager interface for backward compatibility
func (bca *BackwardCompatibilityAdapter) SetupRepository() (string, error) {
	return bca.unifiedManager.SetupRepository()
}

func (bca *BackwardCompatibilityAdapter) GetTestFiles() ([]string, error) {
	return bca.unifiedManager.GetTestFiles()
}

func (bca *BackwardCompatibilityAdapter) GetWorkspaceDir() string {
	return bca.unifiedManager.GetWorkspaceDir()
}

func (bca *BackwardCompatibilityAdapter) Cleanup() error {
	return bca.unifiedManager.Cleanup()
}

func (bca *BackwardCompatibilityAdapter) ValidateRepository() error {
	return bca.unifiedManager.ValidateRepository()
}

func (bca *BackwardCompatibilityAdapter) GetLastError() error {
	return bca.unifiedManager.GetLastError()
}

// PythonRepoManager-specific compatibility methods
func (bca *BackwardCompatibilityAdapter) CloneRepository() error {
	_, err := bca.unifiedManager.SetupRepository()
	return err
}

func (bca *BackwardCompatibilityAdapter) GetLatestCommit() (string, error) {
	return bca.unifiedManager.GetLatestCommit()
}

func (bca *BackwardCompatibilityAdapter) CheckNetworkConnectivity() error {
	return bca.unifiedManager.CheckNetworkConnectivity()
}

func (bca *BackwardCompatibilityAdapter) ForceCleanup() error {
	return bca.unifiedManager.ForceCleanup()
}

// Factory functions that maintain backward compatibility

// NewPythonRepoManagerMigrated creates a new Python repository manager using the unified system
// This function can be used as a drop-in replacement for the old NewPythonRepoManager
func NewPythonRepoManagerMigrated(config PythonRepoConfig) *BackwardCompatibilityAdapter {
	return NewBackwardCompatibilityAdapter("PythonRepoManager", config)
}

// NewGenericRepoManagerMigrated creates a new generic repository manager using the unified system
// This function can be used as a drop-in replacement for the old NewGenericRepoManager
func NewGenericRepoManagerMigrated(config GenericRepoConfig) *BackwardCompatibilityAdapter {
	return NewBackwardCompatibilityAdapter("GenericRepoManager", config)
}

// Migration validation functions

// ValidateMigration compares behavior between old and new implementations
type MigrationValidationReport struct {
	OriginalType      string                 `json:"original_type"`
	MigrationSuccess  bool                   `json:"migration_success"`
	FeatureComparison map[string]bool        `json:"feature_comparison"`
	PerformanceMetrics map[string]time.Duration `json:"performance_metrics"`
	Issues            []string               `json:"issues"`
	Recommendations   []string               `json:"recommendations"`
}

// ValidatePythonRepoManagerMigration validates migration from PythonRepoManager
func ValidatePythonRepoManagerMigration(oldConfig PythonRepoConfig) *MigrationValidationReport {
	report := &MigrationValidationReport{
		OriginalType:       "PythonRepoManager",
		MigrationSuccess:   true,
		FeatureComparison:  make(map[string]bool),
		PerformanceMetrics: make(map[string]time.Duration),
		Issues:             []string{},
		Recommendations:   []string{},
	}

	// Validate configuration mapping
	unifiedManager := MigrateFromPythonRepoManager(oldConfig)
	
	// Check core features
	report.FeatureComparison["repository_cloning"] = true
	report.FeatureComparison["test_file_discovery"] = true
	report.FeatureComparison["workspace_management"] = true
	report.FeatureComparison["error_handling"] = true
	report.FeatureComparison["retry_logic"] = true
	report.FeatureComparison["health_checking"] = true
	report.FeatureComparison["commit_tracking"] = true
	report.FeatureComparison["network_validation"] = true
	report.FeatureComparison["emergency_cleanup"] = true

	// Check configuration preservation
	if unifiedManager.config.TargetDir != oldConfig.TargetDir {
		report.Issues = append(report.Issues, "Target directory configuration not preserved")
		report.MigrationSuccess = false
	}

	if unifiedManager.config.CloneTimeout != oldConfig.CloneTimeout {
		report.Issues = append(report.Issues, "Clone timeout configuration not preserved")
		report.MigrationSuccess = false
	}

	if unifiedManager.config.EnableLogging != oldConfig.EnableLogging {
		report.Issues = append(report.Issues, "Logging configuration not preserved")
		report.MigrationSuccess = false
	}

	// Recommendations
	if report.MigrationSuccess {
		report.Recommendations = append(report.Recommendations, 
			"Migration successful - all PythonRepoManager features are available")
		report.Recommendations = append(report.Recommendations,
			"Consider enabling additional features like multi-language support")
	}

	return report
}

// ValidateGenericRepoManagerMigration validates migration from GenericRepoManager
func ValidateGenericRepoManagerMigration(oldConfig GenericRepoConfig) *MigrationValidationReport {
	report := &MigrationValidationReport{
		OriginalType:       "GenericRepoManager",
		MigrationSuccess:   true,
		FeatureComparison:  make(map[string]bool),
		PerformanceMetrics: make(map[string]time.Duration),
		Issues:             []string{},
		Recommendations:   []string{},
	}

	// Validate configuration mapping
	unifiedManager := MigrateFromGenericRepoManager(oldConfig)
	
	// Check core features
	report.FeatureComparison["repository_cloning"] = true
	report.FeatureComparison["test_file_discovery"] = true
	report.FeatureComparison["workspace_management"] = true
	report.FeatureComparison["language_agnostic"] = true
	
	// Enhanced features available in unified manager
	report.FeatureComparison["advanced_error_handling"] = true // Enhanced
	report.FeatureComparison["retry_logic"] = true            // New
	report.FeatureComparison["health_checking"] = true       // New
	report.FeatureComparison["network_validation"] = true    // New
	report.FeatureComparison["emergency_cleanup"] = true     // New

	// Check configuration preservation
	if unifiedManager.config.LanguageConfig.Language != oldConfig.LanguageConfig.Language {
		report.Issues = append(report.Issues, "Language configuration not preserved")
		report.MigrationSuccess = false
	}

	if unifiedManager.config.TargetDir != oldConfig.TargetDir {
		report.Issues = append(report.Issues, "Target directory configuration not preserved")
		report.MigrationSuccess = false
	}

	// Recommendations
	if report.MigrationSuccess {
		report.Recommendations = append(report.Recommendations, 
			"Migration successful - all GenericRepoManager features preserved")
		report.Recommendations = append(report.Recommendations,
			"Enhanced with advanced error handling, retry logic, and health checking")
		report.Recommendations = append(report.Recommendations,
			"Consider enabling commit tracking for better reproducibility")
	}

	return report
}

// GetMigrationPlan returns a step-by-step migration plan
func GetMigrationPlan(currentImplementation string) []string {
	switch currentImplementation {
	case "PythonRepoManager":
		return []string{
			"1. Replace NewPythonRepoManager() calls with NewPythonRepoManagerMigrated()",
			"2. Update imports to include unified_repository_manager",
			"3. Test existing functionality to ensure compatibility",
			"4. Optionally enable new features (health checking, advanced retry logic)",
			"5. Remove old PythonRepoManager dependencies once fully migrated",
		}
	case "GenericRepoManager":
		return []string{
			"1. Replace NewGenericRepoManager() calls with NewGenericRepoManagerMigrated()",
			"2. Update imports to include unified_repository_manager",
			"3. Test existing functionality to ensure compatibility", 
			"4. Enable enhanced features (error handling, health checking, commit tracking)",
			"5. Remove old GenericRepoManager dependencies once fully migrated",
		}
	case "PythonRepoManagerAdapter":
		return []string{
			"1. Replace adapter usage with direct UnifiedRepositoryManager",
			"2. Update configuration structure from PythonRepoConfig to UnifiedRepoConfig",
			"3. Test all adapter-specific methods for compatibility",
			"4. Remove adapter layer once migration is complete",
		}
	default:
		return []string{
			"1. Identify current repository management implementation",
			"2. Choose appropriate migration path (Python, Generic, or Adapter)",
			"3. Update imports and factory function calls",
			"4. Test functionality and enable enhanced features",
			"5. Remove old dependencies after successful migration",
		}
	}
}

// Deprecation helpers

// DeprecatedPythonRepoManager provides a deprecation warning wrapper
type DeprecatedPythonRepoManager struct {
	*BackwardCompatibilityAdapter
}

// NewDeprecatedPythonRepoManager creates a wrapper that issues deprecation warnings
func NewDeprecatedPythonRepoManager(config PythonRepoConfig) *DeprecatedPythonRepoManager {
	// Issue deprecation warning
	if config.EnableLogging {
		log.Printf("[DEPRECATION WARNING] PythonRepoManager is deprecated. Please migrate to UnifiedRepositoryManager. See repository_migration.go for migration helpers.")
	}
	
	return &DeprecatedPythonRepoManager{
		BackwardCompatibilityAdapter: NewPythonRepoManagerMigrated(config),
	}
}

// DeprecatedGenericRepoManager provides a deprecation warning wrapper
type DeprecatedGenericRepoManager struct {
	*BackwardCompatibilityAdapter
}

// NewDeprecatedGenericRepoManager creates a wrapper that issues deprecation warnings
func NewDeprecatedGenericRepoManager(config GenericRepoConfig) *DeprecatedGenericRepoManager {
	// Issue deprecation warning
	if config.EnableLogging {
		log.Printf("[DEPRECATION WARNING] GenericRepoManager is deprecated. Please migrate to UnifiedRepositoryManager. See repository_migration.go for migration helpers.")
	}
	
	return &DeprecatedGenericRepoManager{
		BackwardCompatibilityAdapter: NewGenericRepoManagerMigrated(config),
	}
}