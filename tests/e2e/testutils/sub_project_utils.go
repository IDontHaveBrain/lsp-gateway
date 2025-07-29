package testutils

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// CrossProjectTestCase represents a test case for cross-project functionality
type CrossProjectTestCase struct {
	SourceProject string   `json:"source_project"`
	TargetProject string   `json:"target_project"`
	SourceFile    string   `json:"source_file"`
	Position      Position `json:"position"`
	ExpectedType  string   `json:"expected_type"`
}

// SubProjectLSPClient provides LSP client functionality scoped to a specific sub-project
type SubProjectLSPClient struct {
	SubProject *SubProjectInfo
	HttpClient *HttpClient
	BaseURI    string
}

// CrossProjectReference represents a reference between projects
type CrossProjectReference struct {
	SourceProject string `json:"source_project"`
	TargetProject string `json:"target_project"`
	SourceFile    string `json:"source_file"`
	TargetFile    string `json:"target_file"`
	ReferenceType string `json:"reference_type"`
}

// SubProjectValidationResult contains validation results for a sub-project
type SubProjectValidationResult struct {
	IsValid      bool     `json:"is_valid"`
	Errors       []string `json:"errors"`
	Warnings     []string `json:"warnings"`
	FileCount    int      `json:"file_count"`
	TestFileCount int     `json:"test_file_count"`
}

// FindFilesInSubProject discovers files matching the given patterns within a sub-project
func FindFilesInSubProject(subProject *SubProjectInfo, patterns []string) ([]string, error) {
	if subProject == nil {
		return nil, fmt.Errorf("subProject cannot be nil")
	}

	if _, err := os.Stat(subProject.ProjectPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("sub-project path does not exist: %s", subProject.ProjectPath)
	}

	var matchedFiles []string
	
	err := filepath.Walk(subProject.ProjectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			// Skip common directories that should be ignored
			dirName := info.Name()
			if strings.HasPrefix(dirName, ".") && dirName != "." {
				return filepath.SkipDir
			}
			if dirName == "node_modules" || dirName == "__pycache__" || 
			   dirName == "target" || dirName == "build" || dirName == "dist" {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file matches any of the provided patterns
		for _, pattern := range patterns {
			matched, err := filepath.Match(pattern, info.Name())
			if err != nil {
				continue // Skip invalid patterns
			}
			if matched {
				relativePath, err := filepath.Rel(subProject.ProjectPath, path)
				if err != nil {
					matchedFiles = append(matchedFiles, path)
				} else {
					matchedFiles = append(matchedFiles, relativePath)
				}
				break
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk sub-project directory: %w", err)
	}

	return matchedFiles, nil
}

// GetTestFilesForSubProject returns test files for a specific sub-project
func GetTestFilesForSubProject(subProject *SubProjectInfo) ([]string, error) {
	if subProject == nil {
		return nil, fmt.Errorf("subProject cannot be nil")
	}

	// If test files are already populated, return them
	if len(subProject.TestFiles) > 0 {
		return subProject.TestFiles, nil
	}

	// Language-specific test file patterns
	var testPatterns []string
	switch strings.ToLower(subProject.Language) {
	case "go", "golang":
		testPatterns = []string{"*_test.go"}
	case "python", "py":
		testPatterns = []string{"test_*.py", "*_test.py"}
	case "javascript", "js":
		testPatterns = []string{"*.test.js", "*.spec.js"}
	case "typescript", "ts":
		testPatterns = []string{"*.test.ts", "*.spec.ts"}
	case "java":
		testPatterns = []string{"*Test.java", "*Tests.java"}
	case "rust", "rs":
		testPatterns = []string{"*_test.rs", "test_*.rs"}
	default:
		testPatterns = []string{"*test*", "*Test*"}
	}

	testFiles, err := FindFilesInSubProject(subProject, testPatterns)
	if err != nil {
		return nil, fmt.Errorf("failed to find test files for %s project: %w", subProject.Language, err)
	}

	return testFiles, nil
}

// ValidateSubProjectStructure validates the structure and integrity of a sub-project
func ValidateSubProjectStructure(subProject *SubProjectInfo) error {
	if subProject == nil {
		return fmt.Errorf("subProject cannot be nil")
	}

	result := &SubProjectValidationResult{
		IsValid:  true,
		Errors:   []string{},
		Warnings: []string{},
	}

	// Check if project path exists
	if _, err := os.Stat(subProject.ProjectPath); os.IsNotExist(err) {
		result.Errors = append(result.Errors, fmt.Sprintf("project path does not exist: %s", subProject.ProjectPath))
		result.IsValid = false
	}

	// Validate language is supported
	supportedLanguages := []string{"go", "golang", "python", "py", "javascript", "js", "typescript", "ts", "java", "rust", "rs"}
	languageSupported := false
	normalizedLang := strings.ToLower(subProject.Language)
	for _, lang := range supportedLanguages {
		if normalizedLang == lang {
			languageSupported = true
			break
		}
	}
	if !languageSupported {
		result.Warnings = append(result.Warnings, fmt.Sprintf("language '%s' may not be fully supported", subProject.Language))
	}

	// Check for root markers
	if len(subProject.RootMarkers) == 0 {
		result.Warnings = append(result.Warnings, "no root markers specified")
	} else {
		for _, marker := range subProject.RootMarkers {
			markerPath := filepath.Join(subProject.ProjectPath, marker)
			if _, err := os.Stat(markerPath); os.IsNotExist(err) {
				result.Warnings = append(result.Warnings, fmt.Sprintf("root marker not found: %s", marker))
			}
		}
	}

	// Count files
	allFiles, err := FindFilesInSubProject(subProject, []string{"*"})
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("failed to count files: %v", err))
	} else {
		result.FileCount = len(allFiles)
	}

	// Count test files
	testFiles, err := GetTestFilesForSubProject(subProject)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("failed to count test files: %v", err))
	} else {
		result.TestFileCount = len(testFiles)
	}

	// Basic sanity checks
	if result.FileCount == 0 {
		result.Warnings = append(result.Warnings, "no files found in project")
	}

	if len(result.Errors) > 0 {
		result.IsValid = false
		return fmt.Errorf("sub-project validation failed: %s", strings.Join(result.Errors, "; "))
	}

	return nil
}

// FindCrossProjectReferences analyzes cross-project references within a workspace
func FindCrossProjectReferences(workspace string, subProjects []*SubProjectInfo) (map[string][]string, error) {
	if len(subProjects) == 0 {
		return make(map[string][]string), nil
	}

	references := make(map[string][]string)
	
	for _, sourceProject := range subProjects {
		projectKey := fmt.Sprintf("%s->", sourceProject.Language)
		references[projectKey] = []string{}

		// Get all source files in the project
		sourceFiles, err := getSourceFilesForLanguage(sourceProject)
		if err != nil {
			continue // Skip projects with errors
		}

		// Analyze each source file for potential cross-project references
		for _, sourceFile := range sourceFiles {
			filePath := filepath.Join(sourceProject.ProjectPath, sourceFile)
			crossRefs, err := analyzeFileForCrossProjectReferences(filePath, sourceProject, subProjects)
			if err != nil {
				continue // Skip files with errors
			}

			for _, ref := range crossRefs {
				refKey := fmt.Sprintf("%s->%s", sourceProject.Language, ref)
				if !contains(references[projectKey], refKey) {
					references[projectKey] = append(references[projectKey], refKey)
				}
			}
		}
	}

	return references, nil
}

// GenerateCrossProjectTestCases generates test cases for cross-project functionality
func GenerateCrossProjectTestCases(subProjects []*SubProjectInfo) ([]CrossProjectTestCase, error) {
	if len(subProjects) < 2 {
		return []CrossProjectTestCase{}, nil
	}

	var testCases []CrossProjectTestCase

	// Generate test cases for each pair of projects
	for i, sourceProject := range subProjects {
		for j, targetProject := range subProjects {
			if i == j {
				continue // Skip same project
			}

			// Get sample files from source project
			sourceFiles, err := getSourceFilesForLanguage(sourceProject)
			if err != nil || len(sourceFiles) == 0 {
				continue
			}

			// Create test cases for different LSP operations
			for _, testType := range []string{"definition", "references", "hover"} {
				testCase := CrossProjectTestCase{
					SourceProject: sourceProject.Language,
					TargetProject: targetProject.Language,
					SourceFile:    sourceFiles[0], // Use first available file
					Position:      Position{Line: 0, Character: 0},
					ExpectedType:  testType,
				}
				testCases = append(testCases, testCase)
			}
		}
	}

	return testCases, nil
}

// NewSubProjectLSPClient creates a new LSP client scoped to a specific sub-project
func NewSubProjectLSPClient(subProject *SubProjectInfo, httpClient *HttpClient) *SubProjectLSPClient {
	if subProject == nil || httpClient == nil {
		return nil
	}

	// Generate base URI for the sub-project
	baseURI := fmt.Sprintf("file://%s", subProject.ProjectPath)

	return &SubProjectLSPClient{
		SubProject: subProject,
		HttpClient: httpClient,
		BaseURI:    baseURI,
	}
}

// GetDefinitionInSubProject retrieves definitions within the sub-project context
func (c *SubProjectLSPClient) GetDefinitionInSubProject(ctx context.Context, fileName string, position Position) ([]Location, error) {
	if c == nil || c.HttpClient == nil {
		return nil, fmt.Errorf("client not properly initialized")
	}

	// Convert relative file path to absolute URI
	fileURI := c.resolveFileURI(fileName)
	
	// Use the HTTP client to make the definition request
	locations, err := c.HttpClient.Definition(ctx, fileURI, position)
	if err != nil {
		return nil, fmt.Errorf("definition request failed for %s in %s project: %w", fileName, c.SubProject.Language, err)
	}

	// Filter results to only include locations within this sub-project
	filteredLocations := c.filterLocationsBySubProject(locations)
	
	return filteredLocations, nil
}

// GetReferencesInSubProject retrieves references within the sub-project context
func (c *SubProjectLSPClient) GetReferencesInSubProject(ctx context.Context, fileName string, position Position) ([]Location, error) {
	if c == nil || c.HttpClient == nil {
		return nil, fmt.Errorf("client not properly initialized")
	}

	// Convert relative file path to absolute URI
	fileURI := c.resolveFileURI(fileName)
	
	// Use the HTTP client to make the references request
	locations, err := c.HttpClient.References(ctx, fileURI, position, true)
	if err != nil {
		return nil, fmt.Errorf("references request failed for %s in %s project: %w", fileName, c.SubProject.Language, err)
	}

	// Filter results to only include locations within this sub-project
	filteredLocations := c.filterLocationsBySubProject(locations)
	
	return filteredLocations, nil
}

// GetHoverInSubProject retrieves hover information within the sub-project context
func (c *SubProjectLSPClient) GetHoverInSubProject(ctx context.Context, fileName string, position Position) (*HoverResult, error) {
	if c == nil || c.HttpClient == nil {
		return nil, fmt.Errorf("client not properly initialized")
	}

	// Convert relative file path to absolute URI
	fileURI := c.resolveFileURI(fileName)
	
	// Use the HTTP client to make the hover request
	hoverResult, err := c.HttpClient.Hover(ctx, fileURI, position)
	if err != nil {
		return nil, fmt.Errorf("hover request failed for %s in %s project: %w", fileName, c.SubProject.Language, err)
	}

	return hoverResult, nil
}

// TestLSPFunctionality performs comprehensive LSP testing on the sub-project
func (c *SubProjectLSPClient) TestLSPFunctionality(ctx context.Context) error {
	if c == nil || c.HttpClient == nil {
		return fmt.Errorf("client not properly initialized")
	}

	// Get test files for this sub-project
	testFiles, err := GetTestFilesForSubProject(c.SubProject)
	if err != nil {
		return fmt.Errorf("failed to get test files: %w", err)
	}

	if len(testFiles) == 0 {
		return fmt.Errorf("no test files found in %s project", c.SubProject.Language)
	}

	// Test basic LSP operations on the first test file
	testFile := testFiles[0]
	testPosition := Position{Line: 0, Character: 0}

	// Test definition
	_, err = c.GetDefinitionInSubProject(ctx, testFile, testPosition)
	if err != nil {
		return fmt.Errorf("definition test failed: %w", err)
	}

	// Test references
	_, err = c.GetReferencesInSubProject(ctx, testFile, testPosition)
	if err != nil {
		return fmt.Errorf("references test failed: %w", err)
	}

	// Test hover
	_, err = c.GetHoverInSubProject(ctx, testFile, testPosition)
	if err != nil {
		return fmt.Errorf("hover test failed: %w", err)
	}

	return nil
}

// Private helper methods

// resolveFileURI converts a relative file path to an absolute file URI
func (c *SubProjectLSPClient) resolveFileURI(fileName string) string {
	if strings.HasPrefix(fileName, "file://") {
		return fileName
	}
	
	// Handle both absolute and relative paths
	var absolutePath string
	if filepath.IsAbs(fileName) {
		absolutePath = fileName
	} else {
		absolutePath = filepath.Join(c.SubProject.ProjectPath, fileName)
	}
	
	return fmt.Sprintf("file://%s", absolutePath)
}

// filterLocationsBySubProject filters locations to only include those within the sub-project
func (c *SubProjectLSPClient) filterLocationsBySubProject(locations []Location) []Location {
	var filtered []Location
	
	for _, location := range locations {
		// Convert URI to file path
		filePath := strings.TrimPrefix(location.URI, "file://")
		
		// Check if the file is within this sub-project
		if strings.HasPrefix(filePath, c.SubProject.ProjectPath) {
			filtered = append(filtered, location)
		}
	}
	
	return filtered
}

// getSourceFilesForLanguage returns source files for a specific language
func getSourceFilesForLanguage(subProject *SubProjectInfo) ([]string, error) {
	var patterns []string
	
	switch strings.ToLower(subProject.Language) {
	case "go", "golang":
		patterns = []string{"*.go"}
	case "python", "py":
		patterns = []string{"*.py"}
	case "javascript", "js":
		patterns = []string{"*.js"}
	case "typescript", "ts":
		patterns = []string{"*.ts"}
	case "java":
		patterns = []string{"*.java"}
	case "rust", "rs":
		patterns = []string{"*.rs"}
	default:
		patterns = []string{"*"}
	}

	return FindFilesInSubProject(subProject, patterns)
}

// analyzeFileForCrossProjectReferences analyzes a file for potential cross-project references
func analyzeFileForCrossProjectReferences(filePath string, sourceProject *SubProjectInfo, allProjects []*SubProjectInfo) ([]string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var references []string
	contentStr := string(content)

	// Look for common import/include patterns that might reference other projects
	for _, targetProject := range allProjects {
		if targetProject.Language == sourceProject.Language {
			continue // Skip same language
		}

		// Simple heuristic: look for references to other project names or languages
		targetLang := strings.ToLower(targetProject.Language)
		patterns := []string{
			fmt.Sprintf(`import.*%s`, targetLang),
			fmt.Sprintf(`require.*%s`, targetLang),
			fmt.Sprintf(`#include.*%s`, targetLang),
			fmt.Sprintf(`from %s`, targetLang),
		}

		for _, pattern := range patterns {
			matched, err := regexp.MatchString(pattern, contentStr)
			if err == nil && matched {
				references = append(references, targetProject.Language)
				break
			}
		}
	}

	return references, nil
}

// contains checks if a slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// GetSubProjectByLanguage returns a sub-project by language from a list
func GetSubProjectByLanguage(subProjects []*SubProjectInfo, language string) (*SubProjectInfo, error) {
	normalizedLang := strings.ToLower(language)
	
	for _, project := range subProjects {
		projectLang := strings.ToLower(project.Language)
		if projectLang == normalizedLang || 
		   (normalizedLang == "go" && projectLang == "golang") ||
		   (normalizedLang == "golang" && projectLang == "go") ||
		   (normalizedLang == "js" && projectLang == "javascript") ||
		   (normalizedLang == "javascript" && projectLang == "js") ||
		   (normalizedLang == "ts" && projectLang == "typescript") ||
		   (normalizedLang == "typescript" && projectLang == "ts") ||
		   (normalizedLang == "py" && projectLang == "python") ||
		   (normalizedLang == "python" && projectLang == "py") ||
		   (normalizedLang == "rs" && projectLang == "rust") ||
		   (normalizedLang == "rust" && projectLang == "rs") {
			return project, nil
		}
	}
	
	return nil, fmt.Errorf("sub-project with language '%s' not found", language)
}

// ValidateAllSubProjects validates all sub-projects in a list
func ValidateAllSubProjects(subProjects []*SubProjectInfo) error {
	if len(subProjects) == 0 {
		return fmt.Errorf("no sub-projects to validate")
	}

	var errors []string
	
	for _, project := range subProjects {
		if err := ValidateSubProjectStructure(project); err != nil {
			errors = append(errors, fmt.Sprintf("%s project: %v", project.Language, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("sub-project validation failures: %s", strings.Join(errors, "; "))
	}

	return nil
}

// GetSubProjectStats returns statistics about sub-projects
func GetSubProjectStats(subProjects []*SubProjectInfo) map[string]interface{} {
	stats := make(map[string]interface{})
	
	stats["total_projects"] = len(subProjects)
	
	languageCount := make(map[string]int)
	totalFiles := 0
	totalTestFiles := 0
	
	for _, project := range subProjects {
		languageCount[project.Language]++
		
		// Count files if possible
		if files, err := FindFilesInSubProject(project, []string{"*"}); err == nil {
			totalFiles += len(files)
		}
		
		// Count test files
		if testFiles, err := GetTestFilesForSubProject(project); err == nil {
			totalTestFiles += len(testFiles)
		}
	}
	
	stats["languages"] = languageCount
	stats["total_files"] = totalFiles
	stats["total_test_files"] = totalTestFiles
	
	return stats
}