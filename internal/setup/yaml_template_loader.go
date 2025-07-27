package setup

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// YAMLTemplateLoader interface defines methods for loading YAML templates
type YAMLTemplateLoader interface {
	LoadTemplate(path string) (*EnhancedConfigurationTemplate, error)
	LoadTemplates(directory string) (map[string]*EnhancedConfigurationTemplate, error)
	LoadTemplateByName(directory, name string) (*EnhancedConfigurationTemplate, error)
	ScanTemplateDirectory(directory string) ([]string, error)
	ValidateTemplateFile(path string) error
	GetTemplateMetadata(path string) (*TemplateMetadata, error)
	ClearCache()
}

// FileSystemTemplateLoader implements YAML template loading from filesystem
type FileSystemTemplateLoader struct {
	cache       map[string]*CachedTemplate
	cacheMutex  sync.RWMutex
	maxCacheAge time.Duration
	logger      *SetupLogger
}

// CachedTemplate represents a cached template with metadata
type CachedTemplate struct {
	Template  *EnhancedConfigurationTemplate
	LoadedAt  time.Time
	FilePath  string
	FileInfo  os.FileInfo
	Checksum  string
}

// TemplateMetadata represents metadata about a template file
type TemplateMetadata struct {
	Name         string    `json:"name"`
	FilePath     string    `json:"file_path"`
	Size         int64     `json:"size"`
	ModifiedAt   time.Time `json:"modified_at"`
	IsValid      bool      `json:"is_valid"`
	Language     string    `json:"language,omitempty"`
	Pattern      string    `json:"pattern,omitempty"`
	Description  string    `json:"description,omitempty"`
	Version      string    `json:"version,omitempty"`
}

// NewFileSystemTemplateLoader creates a new filesystem-based template loader
func NewFileSystemTemplateLoader() *FileSystemTemplateLoader {
	return &FileSystemTemplateLoader{
		cache:       make(map[string]*CachedTemplate),
		maxCacheAge: 10 * time.Minute, // Cache templates for 10 minutes
		logger:      NewSetupLogger(nil),
	}
}

// LoadTemplate loads a single YAML template from the specified path
func (loader *FileSystemTemplateLoader) LoadTemplate(path string) (*EnhancedConfigurationTemplate, error) {
	// Check cache first
	if cached := loader.getCachedTemplate(path); cached != nil {
		loader.logger.WithField("path", path).Debug("Loaded template from cache")
		return cached.Template, nil
	}

	// Validate file exists and is readable
	if err := loader.validateFilePath(path); err != nil {
		return nil, fmt.Errorf("invalid template file path: %w", err)
	}

	// Read file content
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read template file %s: %w", path, err)
	}

	// Parse YAML content
	template, err := loader.parseYAMLTemplate(content, path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template %s: %w", path, err)
	}

	// Cache the template
	if err := loader.cacheTemplate(path, template); err != nil {
		loader.logger.WithError(err).WithField("path", path).Warn("Failed to cache template")
	}

	loader.logger.WithFields(map[string]interface{}{
		"path":         path,
		"name":         template.Name,
		"version":      template.Version,
		"servers":      len(template.Servers),
		"language_pools": len(template.LanguagePools),
	}).Info("Successfully loaded YAML template")

	return template, nil
}

// LoadTemplates loads all YAML templates from the specified directory
func (loader *FileSystemTemplateLoader) LoadTemplates(directory string) (map[string]*EnhancedConfigurationTemplate, error) {
	// Validate directory exists
	if err := loader.validateDirectory(directory); err != nil {
		return nil, fmt.Errorf("invalid template directory: %w", err)
	}

	templates := make(map[string]*EnhancedConfigurationTemplate)
	
	// Scan directory for YAML files
	templatePaths, err := loader.ScanTemplateDirectory(directory)
	if err != nil {
		return nil, fmt.Errorf("failed to scan template directory %s: %w", directory, err)
	}

	// Load each template
	for _, path := range templatePaths {
		template, err := loader.LoadTemplate(path)
		if err != nil {
			loader.logger.WithError(err).WithField("path", path).Error("Failed to load template")
			continue
		}

		// Use filename (without extension) as key if template name is empty
		templateName := template.Name
		if templateName == "" {
			templateName = loader.extractTemplateNameFromPath(path)
		}

		templates[templateName] = template
	}

	loader.logger.WithFields(map[string]interface{}{
		"directory":      directory,
		"templates_found": len(templatePaths),
		"templates_loaded": len(templates),
	}).Info("Loaded YAML templates from directory")

	return templates, nil
}

// LoadTemplateByName loads a specific template by name from a directory
func (loader *FileSystemTemplateLoader) LoadTemplateByName(directory, name string) (*EnhancedConfigurationTemplate, error) {
	// Try different file extensions and patterns
	possiblePaths := []string{
		filepath.Join(directory, name+".yaml"),
		filepath.Join(directory, name+".yml"),
		filepath.Join(directory, "languages", name+".yaml"),
		filepath.Join(directory, "languages", name+".yml"),
		filepath.Join(directory, "patterns", name+".yaml"),
		filepath.Join(directory, "patterns", name+".yml"),
	}

	// Try each possible path
	for _, path := range possiblePaths {
		if loader.fileExists(path) {
			return loader.LoadTemplate(path)
		}
	}

	return nil, fmt.Errorf("template not found: %s in directory %s", name, directory)
}

// ScanTemplateDirectory recursively scans a directory for YAML template files
func (loader *FileSystemTemplateLoader) ScanTemplateDirectory(directory string) ([]string, error) {
	var templatePaths []string

	err := filepath.WalkDir(directory, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			loader.logger.WithError(err).WithField("path", path).Warn("Error walking directory")
			return nil // Continue scanning other files
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Check if file is a YAML template
		if loader.isYAMLTemplateFile(path) {
			templatePaths = append(templatePaths, path)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory %s: %w", directory, err)
	}

	return templatePaths, nil
}

// ValidateTemplateFile validates that a file is a valid YAML template
func (loader *FileSystemTemplateLoader) ValidateTemplateFile(path string) error {
	// Check file exists and is readable
	if err := loader.validateFilePath(path); err != nil {
		return err
	}

	// Try to parse the template
	_, err := loader.LoadTemplate(path)
	return err
}

// GetTemplateMetadata extracts metadata from a template file
func (loader *FileSystemTemplateLoader) GetTemplateMetadata(path string) (*TemplateMetadata, error) {
	// Get file info
	fileInfo, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info for %s: %w", path, err)
	}

	metadata := &TemplateMetadata{
		Name:       loader.extractTemplateNameFromPath(path),
		FilePath:   path,
		Size:       fileInfo.Size(),
		ModifiedAt: fileInfo.ModTime(),
		IsValid:    false,
	}

	// Try to extract template information
	if template, err := loader.LoadTemplate(path); err == nil {
		metadata.IsValid = true
		metadata.Description = template.Description
		metadata.Version = template.Version
		
		// Determine language and pattern from path
		if strings.Contains(path, "languages") {
			metadata.Language = loader.extractLanguageFromPath(path)
		} else if strings.Contains(path, "patterns") {
			metadata.Pattern = loader.extractPatternFromPath(path)
		}
	}

	return metadata, nil
}

// ClearCache clears the template cache
func (loader *FileSystemTemplateLoader) ClearCache() {
	loader.cacheMutex.Lock()
	defer loader.cacheMutex.Unlock()
	
	loader.cache = make(map[string]*CachedTemplate)
	loader.logger.Debug("Template cache cleared")
}

// SetLogger sets the logger for the template loader
func (loader *FileSystemTemplateLoader) SetLogger(logger *SetupLogger) {
	if logger != nil {
		loader.logger = logger
	}
}

// SetMaxCacheAge sets the maximum cache age for templates
func (loader *FileSystemTemplateLoader) SetMaxCacheAge(duration time.Duration) {
	loader.maxCacheAge = duration
}

// parseYAMLTemplate parses YAML content into an EnhancedConfigurationTemplate
func (loader *FileSystemTemplateLoader) parseYAMLTemplate(content []byte, filePath string) (*EnhancedConfigurationTemplate, error) {
	var template EnhancedConfigurationTemplate

	// Parse YAML with strict mode to catch errors
	decoder := yaml.NewDecoder(strings.NewReader(string(content)))
	decoder.KnownFields(true)

	if err := decoder.Decode(&template); err != nil {
		return nil, fmt.Errorf("YAML parsing error in %s: %w", filePath, err)
	}

	// Set default values if not specified
	loader.setTemplateDefaults(&template, filePath)

	// Validate the parsed template
	if err := loader.validateParsedTemplate(&template); err != nil {
		return nil, fmt.Errorf("template validation error in %s: %w", filePath, err)
	}

	return &template, nil
}

// setTemplateDefaults sets default values for template fields
func (loader *FileSystemTemplateLoader) setTemplateDefaults(template *EnhancedConfigurationTemplate, filePath string) {
	// Set name from filename if not specified
	if template.Name == "" {
		template.Name = loader.extractTemplateNameFromPath(filePath)
	}

	// Set version if not specified
	if template.Version == "" {
		template.Version = "1.0"
	}

	// Set creation time
	if template.CreatedAt.IsZero() {
		template.CreatedAt = time.Now()
	}

	// Set default port if not specified
	if template.Port == 0 {
		template.Port = 8080
	}
}

// validateParsedTemplate validates the parsed template structure
func (loader *FileSystemTemplateLoader) validateParsedTemplate(template *EnhancedConfigurationTemplate) error {
	// Basic validation
	if template.Name == "" {
		return fmt.Errorf("template name is required")
	}

	if template.Port < 1 || template.Port > 65535 {
		return fmt.Errorf("invalid port number: %d", template.Port)
	}

	// Validate language pools
	for i, pool := range template.LanguagePools {
		if pool.Language == "" {
			return fmt.Errorf("language pool %d missing language", i)
		}
		if pool.DefaultServer == "" {
			return fmt.Errorf("language pool %s missing default_server", pool.Language)
		}
		if len(pool.Servers) == 0 {
			return fmt.Errorf("language pool %s has no servers defined", pool.Language)
		}
		
		// Validate default server exists in servers map
		if _, exists := pool.Servers[pool.DefaultServer]; !exists {
			return fmt.Errorf("language pool %s default_server '%s' not found in servers", pool.Language, pool.DefaultServer)
		}
	}

	// Validate server instances
	for i, server := range template.Servers {
		if server.Name == "" {
			return fmt.Errorf("server %d missing name", i)
		}
		if server.Command == "" {
			return fmt.Errorf("server %s missing command", server.Name)
		}
		if len(server.Languages) == 0 {
			return fmt.Errorf("server %s has no languages specified", server.Name)
		}
	}

	return nil
}

// getCachedTemplate retrieves a template from cache if valid
func (loader *FileSystemTemplateLoader) getCachedTemplate(path string) *CachedTemplate {
	loader.cacheMutex.RLock()
	defer loader.cacheMutex.RUnlock()

	cached, exists := loader.cache[path]
	if !exists {
		return nil
	}

	// Check if cache is expired
	if time.Since(cached.LoadedAt) > loader.maxCacheAge {
		return nil
	}

	// Check if file has been modified
	fileInfo, err := os.Stat(path)
	if err != nil {
		return nil
	}

	if fileInfo.ModTime().After(cached.FileInfo.ModTime()) {
		return nil
	}

	return cached
}

// cacheTemplate stores a template in cache
func (loader *FileSystemTemplateLoader) cacheTemplate(path string, template *EnhancedConfigurationTemplate) error {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return err
	}

	cached := &CachedTemplate{
		Template: template,
		LoadedAt: time.Now(),
		FilePath: path,
		FileInfo: fileInfo,
	}

	loader.cacheMutex.Lock()
	defer loader.cacheMutex.Unlock()
	
	loader.cache[path] = cached
	return nil
}

// validateFilePath validates that a file path exists and is readable
func (loader *FileSystemTemplateLoader) validateFilePath(path string) error {
	fileInfo, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file does not exist: %s", path)
		}
		return fmt.Errorf("cannot access file %s: %w", path, err)
	}

	if fileInfo.IsDir() {
		return fmt.Errorf("path is a directory, not a file: %s", path)
	}

	// Check if file is readable
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("file is not readable: %s", path)
	}
	file.Close()

	return nil
}

// validateDirectory validates that a directory exists and is accessible
func (loader *FileSystemTemplateLoader) validateDirectory(directory string) error {
	fileInfo, err := os.Stat(directory)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("directory does not exist: %s", directory)
		}
		return fmt.Errorf("cannot access directory %s: %w", directory, err)
	}

	if !fileInfo.IsDir() {
		return fmt.Errorf("path is not a directory: %s", directory)
	}

	return nil
}

// isYAMLTemplateFile checks if a file is a YAML template file
func (loader *FileSystemTemplateLoader) isYAMLTemplateFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".yaml" || ext == ".yml"
}

// fileExists checks if a file exists
func (loader *FileSystemTemplateLoader) fileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// extractTemplateNameFromPath extracts template name from file path
func (loader *FileSystemTemplateLoader) extractTemplateNameFromPath(path string) string {
	base := filepath.Base(path)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	return strings.ToLower(name)
}

// extractLanguageFromPath extracts language name from file path
func (loader *FileSystemTemplateLoader) extractLanguageFromPath(path string) string {
	if strings.Contains(path, "languages") {
		base := filepath.Base(path)
		name := strings.TrimSuffix(base, filepath.Ext(base))
		// Extract language from names like "go-advanced", "python-django"
		parts := strings.Split(name, "-")
		if len(parts) > 0 {
			return parts[0]
		}
		return name
	}
	return ""
}

// extractPatternFromPath extracts pattern name from file path
func (loader *FileSystemTemplateLoader) extractPatternFromPath(path string) string {
	if strings.Contains(path, "patterns") {
		base := filepath.Base(path)
		name := strings.TrimSuffix(base, filepath.Ext(base))
		return name
	}
	return ""
}

// GetCacheStats returns cache statistics
func (loader *FileSystemTemplateLoader) GetCacheStats() map[string]interface{} {
	loader.cacheMutex.RLock()
	defer loader.cacheMutex.RUnlock()

	return map[string]interface{}{
		"cache_entries": len(loader.cache),
		"max_cache_age": loader.maxCacheAge.String(),
	}
}

// GetCachedTemplates returns a list of currently cached templates
func (loader *FileSystemTemplateLoader) GetCachedTemplates() []string {
	loader.cacheMutex.RLock()
	defer loader.cacheMutex.RUnlock()

	paths := make([]string, 0, len(loader.cache))
	for path := range loader.cache {
		paths = append(paths, path)
	}

	return paths
}