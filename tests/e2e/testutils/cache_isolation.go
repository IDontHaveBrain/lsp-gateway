package testutils

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// CacheIsolationManager provides comprehensive cache isolation for E2E tests
type CacheIsolationManager struct {
	baseDir       string
	testID        string
	cacheDir      string
	configPath    string
	cleanupPaths  []string
	isolation     IsolationLevel
	healthTracker *CacheHealthTracker
	mu            sync.RWMutex
}

// IsolationLevel defines the level of cache isolation required
type IsolationLevel int

const (
	// BasicIsolation provides separate cache directories per test
	BasicIsolation IsolationLevel = iota
	// StrictIsolation provides complete cache isolation with validation
	StrictIsolation
	// ParallelIsolation provides isolation suitable for parallel test execution
	ParallelIsolation
)

// CacheHealthTracker monitors cache health throughout test execution
type CacheHealthTracker struct {
	initialState map[string]interface{}
	checkpoints  []CacheCheckpoint
	violations   []CacheViolation
	mu           sync.RWMutex
}

// CacheCheckpoint represents a point-in-time cache state snapshot
type CacheCheckpoint struct {
	Timestamp time.Time
	State     map[string]interface{}
	TestPhase string
	Valid     bool
	Issues    []string
}

// CacheViolation represents a detected cache isolation violation
type CacheViolation struct {
	Timestamp     time.Time
	ViolationType string
	Description   string
	Impact        string
	TestPhase     string
}

// CacheIsolationConfig configures cache isolation behavior
type CacheIsolationConfig struct {
	IsolationLevel        IsolationLevel
	MaxCacheSize          int64
	TTL                   string
	HealthCheckInterval   string
	BackgroundIndexing    bool
	ValidateStateChanges  bool
	ForceCleanupOnFailure bool
	ParallelSafety        bool
	CustomCachePrefix     string
}

// DefaultCacheIsolationConfig returns default isolation configuration
func DefaultCacheIsolationConfig() CacheIsolationConfig {
	return CacheIsolationConfig{
		IsolationLevel:        StrictIsolation,
		MaxCacheSize:          64 * 1024 * 1024, // 64MB
		TTL:                   "10m",
		HealthCheckInterval:   "1m",
		BackgroundIndexing:    true,
		ValidateStateChanges:  true,
		ForceCleanupOnFailure: true,
		ParallelSafety:        true,
		CustomCachePrefix:     "",
	}
}

// NewCacheIsolationManager creates a new cache isolation manager
func NewCacheIsolationManager(baseDir string, config CacheIsolationConfig) (*CacheIsolationManager, error) {
	// Generate unique test ID for this test run
	testID, err := generateUniqueTestID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate test ID: %w", err)
	}

	// Add additional uniqueness for parallel safety
	if config.ParallelSafety {
		testID = fmt.Sprintf("%s-%d-%d", testID, os.Getpid(), time.Now().UnixNano()%10000)
	}

	// Add custom prefix if specified
	if config.CustomCachePrefix != "" {
		testID = fmt.Sprintf("%s-%s", config.CustomCachePrefix, testID)
	}

	manager := &CacheIsolationManager{
		baseDir:      baseDir,
		testID:       testID,
		cacheDir:     filepath.Join(baseDir, fmt.Sprintf("cache-%s", testID)),
		cleanupPaths: make([]string, 0),
		isolation:    config.IsolationLevel,
		healthTracker: &CacheHealthTracker{
			checkpoints: make([]CacheCheckpoint, 0),
			violations:  make([]CacheViolation, 0),
		},
		mu: sync.RWMutex{},
	}

	return manager, nil
}

// InitializeIsolation sets up cache isolation for a test
func (m *CacheIsolationManager) InitializeIsolation(testName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Ensure clean slate - remove any existing cache directory
	if err := m.forceRemoveDirectory(m.cacheDir); err != nil {
		return fmt.Errorf("failed to remove existing cache directory: %w", err)
	}

	// Create isolated cache directory
	if err := os.MkdirAll(m.cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Track for cleanup
	m.cleanupPaths = append(m.cleanupPaths, m.cacheDir)

	// Initialize health tracking
	m.healthTracker.mu.Lock()
	m.healthTracker.initialState = make(map[string]interface{})
	m.healthTracker.checkpoints = make([]CacheCheckpoint, 0)
	m.healthTracker.violations = make([]CacheViolation, 0)
	m.healthTracker.mu.Unlock()

	// Create checkpoint for initialization
	checkpoint := CacheCheckpoint{
		Timestamp: time.Now(),
		TestPhase: fmt.Sprintf("Initialize-%s", testName),
		Valid:     true,
		Issues:    make([]string, 0),
		State: map[string]interface{}{
			"cache_dir":   m.cacheDir,
			"test_id":     m.testID,
			"isolation":   m.isolation,
			"initialized": true,
		},
	}

	m.healthTracker.mu.Lock()
	m.healthTracker.checkpoints = append(m.healthTracker.checkpoints, checkpoint)
	m.healthTracker.mu.Unlock()

	return nil
}

// ValidateCleanState ensures cache starts in a clean state
func (m *CacheIsolationManager) ValidateCleanState() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check cache directory exists and is empty
	entries, err := os.ReadDir(m.cacheDir)
	if err != nil {
		return fmt.Errorf("failed to read cache directory: %w", err)
	}

	if len(entries) > 0 {
		violation := CacheViolation{
			Timestamp:     time.Now(),
			ViolationType: "DIRTY_CACHE_START",
			Description:   fmt.Sprintf("Cache directory not empty at start: %d files", len(entries)),
			Impact:        "Test may be contaminated by previous test data",
			TestPhase:     "PRE_TEST_VALIDATION",
		}

		m.healthTracker.mu.Lock()
		m.healthTracker.violations = append(m.healthTracker.violations, violation)
		m.healthTracker.mu.Unlock()

		if m.isolation == StrictIsolation || m.isolation == ParallelIsolation {
			return fmt.Errorf("cache isolation violation: %s", violation.Description)
		}
	}

	return nil
}

// RecordCacheState captures current cache state for comparison
func (m *CacheIsolationManager) RecordCacheState(healthURL, testPhase string) error {
	if healthURL == "" {
		// Create checkpoint without server state
		checkpoint := CacheCheckpoint{
			Timestamp: time.Now(),
			TestPhase: testPhase,
			Valid:     true,
			Issues:    []string{"No health URL provided"},
			State: map[string]interface{}{
				"cache_dir": m.cacheDir,
				"offline":   true,
			},
		}

		m.healthTracker.mu.Lock()
		m.healthTracker.checkpoints = append(m.healthTracker.checkpoints, checkpoint)
		m.healthTracker.mu.Unlock()

		return nil
	}

	// Get cache metrics from health endpoint
	resp, err := http.Get(healthURL)
	if err != nil {
		checkpoint := CacheCheckpoint{
			Timestamp: time.Now(),
			TestPhase: testPhase,
			Valid:     false,
			Issues:    []string{fmt.Sprintf("Failed to get health status: %v", err)},
			State: map[string]interface{}{
				"cache_dir": m.cacheDir,
				"error":     err.Error(),
			},
		}

		m.healthTracker.mu.Lock()
		m.healthTracker.checkpoints = append(m.healthTracker.checkpoints, checkpoint)
		m.healthTracker.mu.Unlock()

		return err
	}
	defer resp.Body.Close()

	var health map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return fmt.Errorf("failed to decode health response: %w", err)
	}

	// Extract cache-specific state
	cacheState := make(map[string]interface{})
	if cache, ok := health["cache"].(map[string]interface{}); ok {
		cacheState = cache
	}

	// Create checkpoint
	checkpoint := CacheCheckpoint{
		Timestamp: time.Now(),
		TestPhase: testPhase,
		Valid:     true,
		Issues:    make([]string, 0),
		State:     cacheState,
	}

	// Validate state if this is initial recording
	if testPhase == "INITIAL_STATE" {
		m.healthTracker.mu.Lock()
		m.healthTracker.initialState = cacheState
		m.healthTracker.mu.Unlock()
	}

	m.healthTracker.mu.Lock()
	m.healthTracker.checkpoints = append(m.healthTracker.checkpoints, checkpoint)
	m.healthTracker.mu.Unlock()

	return nil
}

// ValidateCacheHealth checks cache health and detects violations
func (m *CacheIsolationManager) ValidateCacheHealth(healthURL string, expectedState string) error {
	if healthURL == "" {
		return nil // Skip validation if no health URL
	}

	resp, err := http.Get(healthURL)
	if err != nil {
		violation := CacheViolation{
			Timestamp:     time.Now(),
			ViolationType: "HEALTH_CHECK_FAILURE",
			Description:   fmt.Sprintf("Failed to connect to health endpoint: %v", err),
			Impact:        "Cannot validate cache health",
			TestPhase:     "HEALTH_VALIDATION",
		}

		m.healthTracker.mu.Lock()
		m.healthTracker.violations = append(m.healthTracker.violations, violation)
		m.healthTracker.mu.Unlock()

		return err
	}
	defer resp.Body.Close()

	var health map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return fmt.Errorf("failed to decode health response: %w", err)
	}

	// Check cache health
	cache, ok := health["cache"].(map[string]interface{})
	if !ok {
		violation := CacheViolation{
			Timestamp:     time.Now(),
			ViolationType: "MISSING_CACHE_METRICS",
			Description:   "Cache metrics not found in health response",
			Impact:        "Cannot validate cache state",
			TestPhase:     "HEALTH_VALIDATION",
		}

		m.healthTracker.mu.Lock()
		m.healthTracker.violations = append(m.healthTracker.violations, violation)
		m.healthTracker.mu.Unlock()

		return fmt.Errorf("cache metrics not available")
	}

	// Validate expected state
	if expectedState != "" {
		if status, hasStatus := cache["status"].(string); hasStatus {
			if status != expectedState {
				violation := CacheViolation{
					Timestamp:     time.Now(),
					ViolationType: "UNEXPECTED_CACHE_STATE",
					Description:   fmt.Sprintf("Expected cache status '%s', got '%s'", expectedState, status),
					Impact:        "Cache may not be in expected state for test",
					TestPhase:     "HEALTH_VALIDATION",
				}

				m.healthTracker.mu.Lock()
				m.healthTracker.violations = append(m.healthTracker.violations, violation)
				m.healthTracker.mu.Unlock()

				if m.isolation == StrictIsolation {
					return fmt.Errorf("cache state validation failed: %s", violation.Description)
				}
			}
		}
	}

	// Check for unhealthy states
	if status, hasStatus := cache["status"].(string); hasStatus {
		unhealthyStates := []string{"critical", "failing"}
		for _, unhealthy := range unhealthyStates {
			if status == unhealthy {
				violation := CacheViolation{
					Timestamp:     time.Now(),
					ViolationType: "UNHEALTHY_CACHE_STATE",
					Description:   fmt.Sprintf("Cache in unhealthy state: %s", status),
					Impact:        "Test results may be unreliable",
					TestPhase:     "HEALTH_VALIDATION",
				}

				m.healthTracker.mu.Lock()
				m.healthTracker.violations = append(m.healthTracker.violations, violation)
				m.healthTracker.mu.Unlock()

				if m.isolation == StrictIsolation || m.isolation == ParallelIsolation {
					return fmt.Errorf("cache health violation: %s", violation.Description)
				}
			}
		}
	}

	return nil
}

// ResetCacheState completely resets cache state for clean test execution
func (m *CacheIsolationManager) ResetCacheState() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Force remove and recreate cache directory
	if err := m.forceRemoveDirectory(m.cacheDir); err != nil {
		return fmt.Errorf("failed to remove cache directory during reset: %w", err)
	}

	if err := os.MkdirAll(m.cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to recreate cache directory: %w", err)
	}

	// Wait for filesystem to stabilize
	time.Sleep(100 * time.Millisecond)

	// Create reset checkpoint
	checkpoint := CacheCheckpoint{
		Timestamp: time.Now(),
		TestPhase: "CACHE_RESET",
		Valid:     true,
		Issues:    make([]string, 0),
		State: map[string]interface{}{
			"cache_dir": m.cacheDir,
			"reset":     true,
		},
	}

	m.healthTracker.mu.Lock()
	m.healthTracker.checkpoints = append(m.healthTracker.checkpoints, checkpoint)
	m.healthTracker.mu.Unlock()

	return nil
}

// GetCacheDirectory returns the isolated cache directory path
func (m *CacheIsolationManager) GetCacheDirectory() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cacheDir
}

// GetConfigPath returns path to generated cache config file
func (m *CacheIsolationManager) GetConfigPath() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.configPath
}

// GenerateIsolatedConfig creates a test-specific configuration with isolated cache
func (m *CacheIsolationManager) GenerateIsolatedConfig(servers map[string]interface{}, config CacheIsolationConfig) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Create config content with isolated cache settings
	configContent := "# Generated isolated cache configuration for E2E testing\n"
	configContent += "servers:\n"

	// Add server configurations
	for name, serverConfig := range servers {
		configContent += fmt.Sprintf("  %s:\n", name)
		if server, ok := serverConfig.(map[string]interface{}); ok {
			if command, hasCmd := server["command"]; hasCmd {
				// Expand ~ paths in command
				expandedCommand := command
				if cmdStr, ok := command.(string); ok {
					if expanded, err := expandPath(cmdStr); err == nil {
						expandedCommand = expanded
					}
				}
				configContent += fmt.Sprintf("    command: \"%s\"\n", expandedCommand)
			}
			if args, hasArgs := server["args"]; hasArgs {
				if argsList, ok := args.([]string); ok {
					configContent += "    args: ["
					for i, arg := range argsList {
						if i > 0 {
							configContent += ", "
						}
						configContent += fmt.Sprintf("\"%s\"", arg)
					}
					configContent += "]\n"
				}
			}
			configContent += "    working_dir: \"\"\n"
			configContent += "    initialization_options: {}\n"
		}
	}

	// Add isolated unified cache configuration
	configContent += "\n# Isolated Unified Cache Configuration\n"
	configContent += "cache:\n"
	configContent += "  enabled: true\n"
	configContent += fmt.Sprintf("  storage_path: \"%s\"\n", m.cacheDir)
	configContent += fmt.Sprintf("  max_memory_mb: %d\n", config.MaxCacheSize/(1024*1024))

	// Convert duration string to hours (simplified units)
	ttlHours := 1 // Default to 1 hour
	if config.TTL == "10m" {
		ttlHours = 1 // Round up short durations to 1 hour
	} else if config.TTL == "24h" {
		ttlHours = 24
	}
	configContent += fmt.Sprintf("  ttl_hours: %d\n", ttlHours)

	configContent += "  languages: [\"go\", \"python\", \"typescript\", \"java\"]\n"
	configContent += fmt.Sprintf("  background_index: %t\n", config.BackgroundIndexing)

	// Convert health check interval to minutes (simplified units)
	healthMinutes := 1 // Default to 1 minute
	if config.HealthCheckInterval == "1m" {
		healthMinutes = 1
	} else if config.HealthCheckInterval == "5m" {
		healthMinutes = 5
	}
	configContent += fmt.Sprintf("  health_check_minutes: %d\n", healthMinutes)
	configContent += "  eviction_policy: \"lru\"\n"
	configContent += "  disk_cache: false\n"
	configContent += "\n# Test isolation metadata\n"
	configContent += fmt.Sprintf("# test_id: \"%s\"\n", m.testID)
	configContent += fmt.Sprintf("# isolation_level: \"%d\"\n", config.IsolationLevel)
	configContent += fmt.Sprintf("# generated_at: \"%s\"\n", time.Now().Format(time.RFC3339))

	// Write config file
	configPath := filepath.Join(m.baseDir, fmt.Sprintf("isolated-config-%s.yaml", m.testID))
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write isolated config: %w", err)
	}

	m.configPath = configPath
	m.cleanupPaths = append(m.cleanupPaths, configPath)

	return configPath, nil
}

// WaitForCacheStabilization waits for cache to reach stable state
func (m *CacheIsolationManager) WaitForCacheStabilization(healthURL string, timeout time.Duration) error {
	if healthURL == "" {
		return nil // Skip if no health URL
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	stableCount := 0
	requiredStableChecks := 3

	for {
		select {
		case <-ctx.Done():
			violation := CacheViolation{
				Timestamp:     time.Now(),
				ViolationType: "CACHE_STABILIZATION_TIMEOUT",
				Description:   fmt.Sprintf("Cache failed to stabilize within %v", timeout),
				Impact:        "Test may start with unstable cache",
				TestPhase:     "STABILIZATION",
			}

			m.healthTracker.mu.Lock()
			m.healthTracker.violations = append(m.healthTracker.violations, violation)
			m.healthTracker.mu.Unlock()

			return fmt.Errorf("cache stabilization timeout after %v", timeout)

		case <-ticker.C:
			resp, err := http.Get(healthURL)
			if err != nil {
				stableCount = 0
				continue
			}

			var health map[string]interface{}
			err = json.NewDecoder(resp.Body).Decode(&health)
			resp.Body.Close()

			if err != nil {
				stableCount = 0
				continue
			}

			cache, ok := health["cache"].(map[string]interface{})
			if !ok {
				stableCount = 0
				continue
			}

			status, hasStatus := cache["status"].(string)
			if !hasStatus {
				stableCount = 0
				continue
			}

			// Check for stable states
			stableStates := []string{"healthy", "initializing"}
			isStable := false
			for _, stable := range stableStates {
				if status == stable {
					isStable = true
					break
				}
			}

			if isStable {
				stableCount++
				if stableCount >= requiredStableChecks {
					return nil // Cache is stable
				}
			} else {
				stableCount = 0
			}
		}
	}
}

// GetViolations returns all detected cache isolation violations
func (m *CacheIsolationManager) GetViolations() []CacheViolation {
	m.healthTracker.mu.RLock()
	defer m.healthTracker.mu.RUnlock()

	// Return copy to prevent external modification
	violations := make([]CacheViolation, len(m.healthTracker.violations))
	copy(violations, m.healthTracker.violations)
	return violations
}

// GetHealthHistory returns cache health checkpoint history
func (m *CacheIsolationManager) GetHealthHistory() []CacheCheckpoint {
	m.healthTracker.mu.RLock()
	defer m.healthTracker.mu.RUnlock()

	// Return copy to prevent external modification
	checkpoints := make([]CacheCheckpoint, len(m.healthTracker.checkpoints))
	copy(checkpoints, m.healthTracker.checkpoints)
	return checkpoints
}

// ValidateTestIsolation performs comprehensive isolation validation
func (m *CacheIsolationManager) ValidateTestIsolation() error {
	violations := m.GetViolations()

	if len(violations) == 0 {
		return nil
	}

	// Count violation types
	criticalViolations := 0
	for _, violation := range violations {
		switch violation.ViolationType {
		case "DIRTY_CACHE_START", "CACHE_CONTAMINATION", "UNHEALTHY_CACHE_STATE":
			criticalViolations++
		}
	}

	if criticalViolations > 0 && (m.isolation == StrictIsolation || m.isolation == ParallelIsolation) {
		return fmt.Errorf("test isolation compromised: %d critical violations detected", criticalViolations)
	}

	return nil
}

// Cleanup performs comprehensive cleanup of isolated cache resources
func (m *CacheIsolationManager) Cleanup() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errors []string

	// Create final checkpoint
	checkpoint := CacheCheckpoint{
		Timestamp: time.Now(),
		TestPhase: "CLEANUP",
		Valid:     true,
		Issues:    make([]string, 0),
		State: map[string]interface{}{
			"cleanup_paths": len(m.cleanupPaths),
		},
	}

	// Clean up all tracked paths
	for _, path := range m.cleanupPaths {
		if err := m.forceRemoveDirectory(path); err != nil {
			error := fmt.Sprintf("Failed to remove %s: %v", path, err)
			errors = append(errors, error)
			checkpoint.Issues = append(checkpoint.Issues, error)
		}
	}

	// Set validity based on cleanup success
	checkpoint.Valid = len(errors) == 0

	m.healthTracker.mu.Lock()
	m.healthTracker.checkpoints = append(m.healthTracker.checkpoints, checkpoint)
	m.healthTracker.mu.Unlock()

	if len(errors) > 0 {
		return fmt.Errorf("cleanup failed: %s", strings.Join(errors, "; "))
	}

	return nil
}

// forceRemoveDirectory forcefully removes a directory and all contents
func (m *CacheIsolationManager) forceRemoveDirectory(path string) error {
	if path == "" || path == "/" {
		return fmt.Errorf("invalid path for removal: %s", path)
	}

	// Check if path exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil // Already removed
	}

	// Try normal removal first
	if err := os.RemoveAll(path); err == nil {
		return nil
	}

	// For Windows and locked files, try more aggressive cleanup
	if runtime.GOOS == "windows" {
		return m.windowsForceRemove(path)
	}

	// For Unix-like systems, try chmod and remove
	return m.unixForceRemove(path)
}

// windowsForceRemove handles Windows-specific file removal challenges
func (m *CacheIsolationManager) windowsForceRemove(path string) error {
	// On Windows, files may be locked by processes
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		if err := os.RemoveAll(path); err == nil {
			return nil
		}

		// Wait and retry
		time.Sleep(time.Duration(i+1) * 200 * time.Millisecond)
	}

	// Final attempt with detailed error
	return fmt.Errorf("failed to remove directory after %d retries: %s", maxRetries, path)
}

// unixForceRemove handles Unix-like system file removal
func (m *CacheIsolationManager) unixForceRemove(path string) error {
	// Try to change permissions first
	if err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue despite individual file errors
		}
		return os.Chmod(filePath, 0755)
	}); err == nil {
		// Try removal after permission change
		if err := os.RemoveAll(path); err == nil {
			return nil
		}
	}

	return fmt.Errorf("failed to force remove directory: %s", path)
}

// generateUniqueTestID creates a unique identifier for test isolation
func generateUniqueTestID() (string, error) {
	// Create random bytes
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Combine timestamp and random data
	timestamp := time.Now().Unix()
	randomHex := hex.EncodeToString(bytes)

	return fmt.Sprintf("%d-%s", timestamp, randomHex[:8]), nil
}

// expandPath expands ~ to the user's home directory in file paths
func expandPath(path string) (string, error) {
	if !strings.HasPrefix(path, "~") {
		return path, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return path, fmt.Errorf("failed to get user home directory: %w", err)
	}

	if path == "~" {
		return homeDir, nil
	}

	if strings.HasPrefix(path, "~/") {
		return filepath.Join(homeDir, path[2:]), nil
	}

	return path, nil
}
