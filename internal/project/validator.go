package project

import (
	"context"
	"fmt"
	"lsp-gateway/internal/setup"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// PathValidationOptions configures path validation behavior
type PathValidationOptions struct {
	RequireAbsolute   bool
	AllowSymlinks     bool
	CheckPermissions  bool
	FollowSymlinks    bool
	ValidateStructure bool
	TimeoutDuration   time.Duration
	MaxDepth          int
	ExcludePatterns   []string
	RequiredMarkers   []string
	Logger            *setup.SetupLogger
}

// DefaultPathValidationOptions returns sensible defaults for path validation
func DefaultPathValidationOptions() *PathValidationOptions {
	return &PathValidationOptions{
		RequireAbsolute:   false,
		AllowSymlinks:     true,
		CheckPermissions:  true,
		FollowSymlinks:    true,
		ValidateStructure: false,
		TimeoutDuration:   30 * time.Second,
		MaxDepth:          MAX_DIRECTORY_DEPTH,
		ExcludePatterns:   []string{".git", "node_modules", "vendor", ".venv", "__pycache__"},
		RequiredMarkers:   []string{},
		Logger:            nil,
	}
}

// PathValidator provides comprehensive path validation and normalization
type PathValidator struct {
	options   *PathValidationOptions
	logger    *setup.SetupLogger
	mu        sync.RWMutex
	cache     map[string]*pathValidationResult
	cacheTime time.Duration
}

// pathValidationResult caches validation results for performance
type pathValidationResult struct {
	valid      bool
	normalized string
	error      error
	timestamp  time.Time
	metadata   map[string]interface{}
}

// NewPathValidator creates a new path validator with the given options
func NewPathValidator(options *PathValidationOptions) *PathValidator {
	if options == nil {
		options = DefaultPathValidationOptions()
	}

	logger := options.Logger
	if logger == nil {
		logger = setup.NewSetupLogger(&setup.SetupLoggerConfig{
			Component: "path-validator",
			Level:     setup.LogLevelInfo,
		})
	}

	return &PathValidator{
		options:   options,
		logger:    logger.WithField("component", "path_validator"),
		cache:     make(map[string]*pathValidationResult),
		cacheTime: 5 * time.Minute,
	}
}

// ValidatePath validates a path and returns normalized path or error
func (v *PathValidator) ValidatePath(ctx context.Context, path, fieldName string) (string, error) {
	if path == "" {
		return "", NewValidationError(PROJECT_TYPE_UNKNOWN, "path cannot be empty", nil).
			WithMissingFiles([]string{"path"})
	}

	// Check cache first
	if result := v.getCachedResult(path); result != nil {
		if result.valid {
			return result.normalized, nil
		}
		return "", result.error
	}

	startTime := time.Now()
	v.logger.WithField("path", path).WithField("field", fieldName).
		Debug("Starting path validation")

	// Validate with timeout
	ctx, cancel := context.WithTimeout(ctx, v.options.TimeoutDuration)
	defer cancel()

	resultChan := make(chan *pathValidationResult, 1)
	go func() {
		result := v.validatePathInternal(path, fieldName)
		resultChan <- result
	}()

	select {
	case result := <-resultChan:
		v.cacheResult(path, result)
		duration := time.Since(startTime)

		if result.valid {
			v.logger.WithFields(map[string]interface{}{
				"path":       path,
				"normalized": result.normalized,
				"duration":   duration.String(),
			}).Debug("Path validation successful")
			return result.normalized, nil
		}

		v.logger.WithError(result.error).WithFields(map[string]interface{}{
			"path":     path,
			"duration": duration.String(),
		}).Warn("Path validation failed")
		return "", result.error

	case <-ctx.Done():
		err := NewDetectionTimeoutError(path, v.options.TimeoutDuration)
		v.logger.WithError(err).WithField("path", path).Error("Path validation timeout")
		return "", err
	}
}

// validatePathInternal performs the actual path validation
func (v *PathValidator) validatePathInternal(path, fieldName string) *pathValidationResult {
	result := &pathValidationResult{
		timestamp: time.Now(),
		metadata:  make(map[string]interface{}),
	}

	// Step 1: Normalize the path
	normalized, err := v.NormalizePath(path)
	if err != nil {
		result.error = NewProjectError(
			ProjectErrorTypeValidation,
			PROJECT_TYPE_UNKNOWN,
			path,
			fmt.Sprintf("cannot normalize path: %v", err),
			err,
		).WithSuggestions([]string{
			"Check if the path format is correct",
			"Use absolute paths for clarity: /full/path/to/directory",
			"Use explicit relative paths: ./relative/path",
		})
		return result
	}
	result.normalized = normalized
	result.metadata["original_path"] = path

	// Step 2: Check if path exists
	info, err := os.Stat(normalized)
	if os.IsNotExist(err) {
		result.error = NewProjectError(
			ProjectErrorTypeValidation,
			PROJECT_TYPE_UNKNOWN,
			normalized,
			fmt.Sprintf("path does not exist: %s", normalized),
			err,
		).WithSuggestions([]string{
			"Check if the path is correct",
			"Create the directory if it should exist",
			fmt.Sprintf("List parent directory: ls -la %s", filepath.Dir(normalized)),
		})
		return result
	}

	if err != nil {
		result.error = NewProjectError(
			ProjectErrorTypeFileSystem,
			PROJECT_TYPE_UNKNOWN,
			normalized,
			fmt.Sprintf("cannot access path: %v", err),
			err,
		).WithSuggestions([]string{
			fmt.Sprintf("Check permissions: ls -la %s", normalized),
			"Ensure you have read access to the path",
			"Check if parent directories are accessible",
		})
		return result
	}

	result.metadata["is_dir"] = info.IsDir()
	result.metadata["size"] = info.Size()
	result.metadata["mode"] = info.Mode().String()
	result.metadata["mod_time"] = info.ModTime()

	// Step 3: Handle symlinks
	if info.Mode()&os.ModeSymlink != 0 {
		if !v.options.AllowSymlinks {
			result.error = NewProjectError(
				ProjectErrorTypeValidation,
				PROJECT_TYPE_UNKNOWN,
				normalized,
				"symlinks are not allowed",
				nil,
			).WithSuggestions([]string{
				"Use the actual target path instead of symlink",
				"Enable symlink support in validation options",
			})
			return result
		}

		if v.options.FollowSymlinks {
			resolved, err := v.ResolveSymlinks(normalized)
			if err != nil {
				result.error = NewProjectError(
					ProjectErrorTypeValidation,
					PROJECT_TYPE_UNKNOWN,
					normalized,
					fmt.Sprintf("cannot resolve symlink: %v", err),
					err,
				).WithSuggestions([]string{
					"Check if symlink target exists",
					"Verify symlink is not broken",
					"Use absolute path to symlink target",
				})
				return result
			}
			result.normalized = resolved
			result.metadata["symlink_target"] = resolved
		}
	}

	// Step 4: Check permissions if required
	if v.options.CheckPermissions {
		if err := v.CheckPathPermissions(normalized); err != nil {
			result.error = err
			return result
		}
	}

	// Step 5: Validate as project path if required
	if v.options.ValidateStructure {
		if err := v.ValidateProjectStructure(normalized, fieldName); err != nil {
			result.error = err
			return result
		}
	}

	// Step 6: Check required markers if specified
	if len(v.options.RequiredMarkers) > 0 {
		if err := v.checkRequiredMarkers(normalized); err != nil {
			result.error = err
			return result
		}
	}

	result.valid = true
	return result
}

// NormalizePath normalizes and cleans a path
func (v *PathValidator) NormalizePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path cannot be empty")
	}

	// Handle special cases
	if path == "." {
		return filepath.Abs(".")
	}
	if path == ".." {
		return filepath.Abs("..")
	}

	// Clean the path to remove redundant elements
	cleanPath := filepath.Clean(path)

	// Expand user home directory
	if strings.HasPrefix(cleanPath, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot expand home directory: %w", err)
		}
		cleanPath = filepath.Join(homeDir, cleanPath[2:])
	}

	// Convert to absolute path if required or if relative
	if v.options.RequireAbsolute || !filepath.IsAbs(cleanPath) {
		absPath, err := filepath.Abs(cleanPath)
		if err != nil {
			return "", fmt.Errorf("cannot resolve absolute path: %w", err)
		}
		return absPath, nil
	}

	return cleanPath, nil
}

// ValidateReadAccess validates that a path can be read
func (v *PathValidator) ValidateReadAccess(path string) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return NewProjectError(
			ProjectErrorTypeFileSystem,
			PROJECT_TYPE_UNKNOWN,
			path,
			fmt.Sprintf("path does not exist: %s", path),
			err,
		).WithSuggestions([]string{
			"Check if the path is correct",
			"Create the path if it should exist",
		})
	}

	if err != nil {
		return NewProjectError(
			ProjectErrorTypeFileSystem,
			PROJECT_TYPE_UNKNOWN,
			path,
			fmt.Sprintf("cannot access path: %v", err),
			err,
		).WithSuggestions([]string{
			fmt.Sprintf("Check permissions: ls -la %s", path),
			"Ensure you have read access to the path",
		})
	}

	// Test actual read access
	if info.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			return NewProjectError(
				ProjectErrorTypeFileSystem,
				PROJECT_TYPE_UNKNOWN,
				path,
				fmt.Sprintf("cannot read directory: %v", err),
				err,
			).WithSuggestions([]string{
				fmt.Sprintf("Check directory permissions: ls -la %s", path),
				fmt.Sprintf("Fix permissions: chmod 755 %s", path),
			})
		}
		_ = entries // Just verify we can read
	} else {
		file, err := os.Open(path)
		if err != nil {
			return NewProjectError(
				ProjectErrorTypeFileSystem,
				PROJECT_TYPE_UNKNOWN,
				path,
				fmt.Sprintf("cannot read file: %v", err),
				err,
			).WithSuggestions([]string{
				fmt.Sprintf("Check file permissions: ls -la %s", path),
				fmt.Sprintf("Fix permissions: chmod 644 %s", path),
			})
		}
		if err := file.Close(); err != nil {
			v.logger.WithError(err).Debug("Error closing test file")
		}
	}

	return nil
}

// ValidateWriteAccess validates that a path can be written to
func (v *PathValidator) ValidateWriteAccess(path string) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		// Check if parent directory is writable
		parent := filepath.Dir(path)
		return v.validateDirectoryWritable(parent, "parent directory")
	}

	if err != nil {
		return NewProjectError(
			ProjectErrorTypeFileSystem,
			PROJECT_TYPE_UNKNOWN,
			path,
			fmt.Sprintf("cannot access path: %v", err),
			err,
		)
	}

	if info.IsDir() {
		return v.validateDirectoryWritable(path, "directory")
	}

	// Test file write access
	file, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return NewProjectError(
			ProjectErrorTypeFileSystem,
			PROJECT_TYPE_UNKNOWN,
			path,
			fmt.Sprintf("cannot write to file: %v", err),
			err,
		).WithSuggestions([]string{
			fmt.Sprintf("Check file permissions: ls -la %s", path),
			fmt.Sprintf("Fix permissions: chmod 644 %s", path),
		})
	}
	if err := file.Close(); err != nil {
		v.logger.WithError(err).Debug("Error closing test file")
	}

	return nil
}

// validateDirectoryWritable tests if a directory is writable
func (v *PathValidator) validateDirectoryWritable(dir, contextName string) error {
	tempFile := filepath.Join(dir, ".lsp-gateway-test-write")
	file, err := os.Create(tempFile)
	if err != nil {
		return NewProjectError(
			ProjectErrorTypeFileSystem,
			PROJECT_TYPE_UNKNOWN,
			dir,
			fmt.Sprintf("cannot write to %s: %v", contextName, err),
			err,
		).WithSuggestions([]string{
			fmt.Sprintf("Check directory permissions: ls -la %s", dir),
			fmt.Sprintf("Fix permissions: chmod 755 %s", dir),
		})
	}
	if err := file.Close(); err != nil {
		v.logger.WithError(err).Debug("Error closing test file")
	}
	if err := os.Remove(tempFile); err != nil {
		v.logger.WithError(err).Debug("Error removing test file")
	}

	return nil
}

// IsValidProjectPath checks if a path represents a valid project directory
func (v *PathValidator) IsValidProjectPath(path string) (bool, error) {
	normalized, err := v.NormalizePath(path)
	if err != nil {
		return false, err
	}

	info, err := os.Stat(normalized)
	if err != nil {
		return false, nil // Path doesn't exist, not valid
	}

	if !info.IsDir() {
		return false, nil // Not a directory, not a valid project path
	}

	// Check for any project markers
	projectMarkers := []string{
		MARKER_GO_MOD, MARKER_PACKAGE_JSON, MARKER_PYPROJECT,
		MARKER_SETUP_PY, MARKER_POM_XML, MARKER_BUILD_GRADLE,
		MARKER_TSCONFIG, MARKER_CARGO_TOML, DIR_DOT_GIT,
	}

	for _, marker := range projectMarkers {
		markerPath := filepath.Join(normalized, marker)
		if _, err := os.Stat(markerPath); err == nil {
			return true, nil
		}
	}

	return false, nil
}

// ResolveSymlinks resolves all symlinks in a path
func (v *PathValidator) ResolveSymlinks(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("cannot resolve symlinks in path %s: %w", path, err)
	}

	// Ensure the resolved path is absolute
	if !filepath.IsAbs(resolved) {
		absResolved, err := filepath.Abs(resolved)
		if err != nil {
			return "", fmt.Errorf("cannot make resolved path absolute: %w", err)
		}
		resolved = absResolved
	}

	return resolved, nil
}

// CheckPathPermissions validates path permissions for read/write access
func (v *PathValidator) CheckPathPermissions(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return NewProjectError(
			ProjectErrorTypeFileSystem,
			PROJECT_TYPE_UNKNOWN,
			path,
			fmt.Sprintf("cannot access path: %v", err),
			err,
		)
	}

	mode := info.Mode()

	// Check read permissions
	if mode&0400 == 0 { // Owner read permission
		return NewProjectError(
			ProjectErrorTypeFileSystem,
			PROJECT_TYPE_UNKNOWN,
			path,
			"insufficient read permissions",
			nil,
		).WithSuggestions([]string{
			fmt.Sprintf("Fix read permissions: chmod u+r %s", path),
			fmt.Sprintf("Check current permissions: ls -la %s", path),
		})
	}

	// For directories, check execute permission (needed to enter directory)
	if info.IsDir() && mode&0100 == 0 { // Owner execute permission
		return NewProjectError(
			ProjectErrorTypeFileSystem,
			PROJECT_TYPE_UNKNOWN,
			path,
			"insufficient execute permissions for directory",
			nil,
		).WithSuggestions([]string{
			fmt.Sprintf("Fix execute permissions: chmod u+x %s", path),
			fmt.Sprintf("Fix all permissions: chmod 755 %s", path),
		})
	}

	return nil
}

// ValidateProjectStructure validates that a path contains a valid project structure
func (v *PathValidator) ValidateProjectStructure(path, fieldName string) error {
	normalized, err := v.NormalizePath(path)
	if err != nil {
		return NewProjectError(
			ProjectErrorTypeValidation,
			PROJECT_TYPE_UNKNOWN,
			path,
			fmt.Sprintf("cannot normalize path: %v", err),
			err,
		)
	}

	info, err := os.Stat(normalized)
	if err != nil {
		return NewProjectError(
			ProjectErrorTypeValidation,
			PROJECT_TYPE_UNKNOWN,
			normalized,
			fmt.Sprintf("cannot access path: %v", err),
			err,
		)
	}

	if !info.IsDir() {
		return NewProjectError(
			ProjectErrorTypeStructure,
			PROJECT_TYPE_UNKNOWN,
			normalized,
			"path is not a directory",
			nil,
		).WithSuggestions([]string{
			"Specify a directory path for project validation",
			"Remove filename from the path if present",
		})
	}

	// Check for project markers
	projectMarkers := []string{
		MARKER_GO_MOD, MARKER_PACKAGE_JSON, MARKER_PYPROJECT,
		MARKER_SETUP_PY, MARKER_REQUIREMENTS, MARKER_POM_XML,
		MARKER_BUILD_GRADLE, MARKER_TSCONFIG, MARKER_CARGO_TOML,
		DIR_DOT_GIT,
	}

	foundMarkers := []string{}
	for _, marker := range projectMarkers {
		markerPath := filepath.Join(normalized, marker)
		if _, err := os.Stat(markerPath); err == nil {
			foundMarkers = append(foundMarkers, marker)
		}
	}

	if len(foundMarkers) == 0 {
		return NewProjectError(
			ProjectErrorTypeStructure,
			PROJECT_TYPE_UNKNOWN,
			normalized,
			"directory does not appear to be a project workspace",
			nil,
		).WithSuggestions([]string{
			"Ensure the directory contains project files (go.mod, package.json, etc.)",
			"Initialize a project in this directory if needed",
			"Use a directory that contains a recognized project structure",
			fmt.Sprintf("List directory contents: ls -la %s", normalized),
		}).WithMetadata("expected_markers", projectMarkers)
	}

	v.logger.WithFields(map[string]interface{}{
		"path":          normalized,
		"found_markers": foundMarkers,
		"marker_count":  len(foundMarkers),
	}).Debug("Project structure validation successful")

	return nil
}

// checkRequiredMarkers validates that required markers are present
func (v *PathValidator) checkRequiredMarkers(path string) error {
	missingMarkers := []string{}

	for _, marker := range v.options.RequiredMarkers {
		markerPath := filepath.Join(path, marker)
		if _, err := os.Stat(markerPath); os.IsNotExist(err) {
			missingMarkers = append(missingMarkers, marker)
		}
	}

	if len(missingMarkers) > 0 {
		return NewProjectError(
			ProjectErrorTypeStructure,
			PROJECT_TYPE_UNKNOWN,
			path,
			"required project markers are missing",
			nil,
		).WithSuggestions([]string{
			"Ensure all required project files are present",
			"Check if you're in the correct project directory",
			"Initialize missing project configuration files",
		}).WithMetadata("missing_markers", missingMarkers)
	}

	return nil
}

// getCachedResult retrieves a cached validation result if still valid
func (v *PathValidator) getCachedResult(path string) *pathValidationResult {
	v.mu.RLock()
	defer v.mu.RUnlock()

	result, exists := v.cache[path]
	if !exists {
		return nil
	}

	// Check if cache entry is still valid
	if time.Since(result.timestamp) > v.cacheTime {
		// Remove stale entry
		delete(v.cache, path)
		return nil
	}

	return result
}

// cacheResult stores a validation result in the cache
func (v *PathValidator) cacheResult(path string, result *pathValidationResult) {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Limit cache size
	if len(v.cache) >= 1000 {
		// Remove oldest entries
		oldest := time.Now()
		oldestKey := ""
		for k, v := range v.cache {
			if v.timestamp.Before(oldest) {
				oldest = v.timestamp
				oldestKey = k
			}
		}
		if oldestKey != "" {
			delete(v.cache, oldestKey)
		}
	}

	v.cache[path] = result
}

// ClearCache clears the validation cache
func (v *PathValidator) ClearCache() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.cache = make(map[string]*pathValidationResult)
}

// GetCacheStats returns cache statistics
func (v *PathValidator) GetCacheStats() map[string]interface{} {
	v.mu.RLock()
	defer v.mu.RUnlock()

	validEntries := 0
	expiredEntries := 0
	now := time.Now()

	for _, result := range v.cache {
		if now.Sub(result.timestamp) <= v.cacheTime {
			validEntries++
		} else {
			expiredEntries++
		}
	}

	return map[string]interface{}{
		"total_entries":   len(v.cache),
		"valid_entries":   validEntries,
		"expired_entries": expiredEntries,
		"cache_duration":  v.cacheTime.String(),
	}
}
