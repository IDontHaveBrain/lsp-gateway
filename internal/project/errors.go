package project

import (
	"fmt"
	"lsp-gateway/internal/platform"
	"strings"
	"time"
)

const (
	ErrCodeProjectNotFound         platform.ErrorCode = "PROJECT_NOT_FOUND"
	ErrCodeProjectDetectionFailed  platform.ErrorCode = "PROJECT_DETECTION_FAILED"
	ErrCodeWorkspaceRootNotFound   platform.ErrorCode = "WORKSPACE_ROOT_NOT_FOUND"
	ErrCodeInvalidProjectStructure platform.ErrorCode = "INVALID_PROJECT_STRUCTURE"
	ErrCodeProjectValidationFailed platform.ErrorCode = "PROJECT_VALIDATION_FAILED"
	ErrCodeMultipleProjectRoots    platform.ErrorCode = "MULTIPLE_PROJECT_ROOTS"
	ErrCodeUnsupportedProjectType  platform.ErrorCode = "UNSUPPORTED_PROJECT_TYPE"
	ErrCodeProjectConfigInvalid    platform.ErrorCode = "PROJECT_CONFIG_INVALID"
	ErrCodeDetectionTimeout        platform.ErrorCode = "DETECTION_TIMEOUT"
	ErrCodeFileSystemAccess        platform.ErrorCode = "FILESYSTEM_ACCESS_ERROR"
)

type ProjectErrorType string

const (
	ProjectErrorTypeDetection    ProjectErrorType = "detection_failed"
	ProjectErrorTypeValidation   ProjectErrorType = "validation_failed"
	ProjectErrorTypeConfiguration ProjectErrorType = "configuration_error"
	ProjectErrorTypeStructure    ProjectErrorType = "structure_error"
	ProjectErrorTypeWorkspace    ProjectErrorType = "workspace_error"
	ProjectErrorTypeTimeout      ProjectErrorType = "timeout_error"
	ProjectErrorTypeFileSystem   ProjectErrorType = "filesystem_error"
)

type ProjectError struct {
	Type        ProjectErrorType
	ProjectType string
	ProjectRoot string
	Message     string
	Path        string
	Metadata    map[string]interface{}
	Suggestions []string
	Cause       error
}

func (e *ProjectError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s error for %s project at %s: %s (caused by: %v)",
			e.Type, e.ProjectType, e.ProjectRoot, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s error for %s project at %s: %s",
		e.Type, e.ProjectType, e.ProjectRoot, e.Message)
}

func (e *ProjectError) Unwrap() error {
	return e.Cause
}

func (e *ProjectError) Is(target error) bool {
	if t, ok := target.(*ProjectError); ok {
		return e.Type == t.Type
	}
	return false
}

func NewProjectError(errorType ProjectErrorType, projectType, projectRoot, message string, cause error) *ProjectError {
	return &ProjectError{
		Type:        errorType,
		ProjectType: projectType,
		ProjectRoot: projectRoot,
		Message:     message,
		Metadata:    make(map[string]interface{}),
		Suggestions: make([]string, 0),
		Cause:       cause,
	}
}

func (e *ProjectError) WithPath(path string) *ProjectError {
	e.Path = path
	return e
}

func (e *ProjectError) WithMetadata(key string, value interface{}) *ProjectError {
	if e.Metadata == nil {
		e.Metadata = make(map[string]interface{})
	}
	e.Metadata[key] = value
	return e
}

func (e *ProjectError) WithSuggestions(suggestions []string) *ProjectError {
	e.Suggestions = suggestions
	return e
}

func (e *ProjectError) WithSuggestion(suggestion string) *ProjectError {
	e.Suggestions = append(e.Suggestions, suggestion)
	return e
}

type DetectionError struct {
	ProjectType string
	Phase       string // "scanning", "parsing", "validation"
	Path        string
	Details     string
	Cause       error
}

func (e *DetectionError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("detection of %s project failed during %s phase at %s: %s (caused by: %v)",
			e.ProjectType, e.Phase, e.Path, e.Details, e.Cause)
	}
	return fmt.Sprintf("detection of %s project failed during %s phase at %s: %s",
		e.ProjectType, e.Phase, e.Path, e.Details)
}

func (e *DetectionError) Unwrap() error {
	return e.Cause
}

func NewDetectionError(projectType, phase, path, details string, cause error) *DetectionError {
	return &DetectionError{
		ProjectType: projectType,
		Phase:       phase,
		Path:        path,
		Details:     details,
		Cause:       cause,
	}
}

type ValidationError struct {
	ProjectType      string
	RequiredFiles    []string
	MissingFiles     []string
	InvalidFiles     []string
	StructureIssues  []string
	ConfigurationErrors []string
	Message          string
	Cause            error
}

func (e *ValidationError) Error() string {
	issues := []string{}
	if len(e.MissingFiles) > 0 {
		issues = append(issues, fmt.Sprintf("missing files: %v", e.MissingFiles))
	}
	if len(e.InvalidFiles) > 0 {
		issues = append(issues, fmt.Sprintf("invalid files: %v", e.InvalidFiles))
	}
	if len(e.StructureIssues) > 0 {
		issues = append(issues, fmt.Sprintf("structure issues: %v", e.StructureIssues))
	}
	if len(e.ConfigurationErrors) > 0 {
		issues = append(issues, fmt.Sprintf("configuration errors: %v", e.ConfigurationErrors))
	}

	issueStr := strings.Join(issues, "; ")
	if e.Cause != nil {
		return fmt.Sprintf("validation of %s project failed: %s (%s) (caused by: %v)",
			e.ProjectType, e.Message, issueStr, e.Cause)
	}
	return fmt.Sprintf("validation of %s project failed: %s (%s)",
		e.ProjectType, e.Message, issueStr)
}

func (e *ValidationError) Unwrap() error {
	return e.Cause
}

func NewValidationError(projectType, message string, cause error) *ValidationError {
	return &ValidationError{
		ProjectType:         projectType,
		RequiredFiles:       make([]string, 0),
		MissingFiles:        make([]string, 0),
		InvalidFiles:        make([]string, 0),
		StructureIssues:     make([]string, 0),
		ConfigurationErrors: make([]string, 0),
		Message:             message,
		Cause:               cause,
	}
}

func (e *ValidationError) WithMissingFiles(files []string) *ValidationError {
	e.MissingFiles = files
	return e
}

func (e *ValidationError) WithInvalidFiles(files []string) *ValidationError {
	e.InvalidFiles = files
	return e
}

func (e *ValidationError) WithStructureIssues(issues []string) *ValidationError {
	e.StructureIssues = issues
	return e
}

func (e *ValidationError) WithConfigurationErrors(errors []string) *ValidationError {
	e.ConfigurationErrors = errors
	return e
}

type WorkspaceError struct {
	Path        string
	Operation   string // "scan", "read", "traverse"
	Permissions string
	Details     string
	Duration    time.Duration
	Cause       error
}

func (e *WorkspaceError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("workspace %s operation failed at %s: %s (permissions: %s, duration: %v) (caused by: %v)",
			e.Operation, e.Path, e.Details, e.Permissions, e.Duration, e.Cause)
	}
	return fmt.Sprintf("workspace %s operation failed at %s: %s (permissions: %s, duration: %v)",
		e.Operation, e.Path, e.Details, e.Permissions, e.Duration)
}

func (e *WorkspaceError) Unwrap() error {
	return e.Cause
}

func NewWorkspaceError(operation, path, details string, cause error) *WorkspaceError {
	return &WorkspaceError{
		Path:      path,
		Operation: operation,
		Details:   details,
		Cause:     cause,
	}
}

func (e *WorkspaceError) WithPermissions(permissions string) *WorkspaceError {
	e.Permissions = permissions
	return e
}

func (e *WorkspaceError) WithDuration(duration time.Duration) *WorkspaceError {
	e.Duration = duration
	return e
}

// Helper functions for creating common errors

func NewProjectNotFoundError(path string) *ProjectError {
	err := NewProjectError(
		ProjectErrorTypeDetection,
		PROJECT_TYPE_UNKNOWN,
		path,
		fmt.Sprintf("No project detected at path '%s'", path),
		nil,
	)

	err.Suggestions = []string{
		"Ensure the path contains a valid project structure",
		"Check for project marker files (go.mod, package.json, setup.py, etc.)",
		"Verify directory permissions and accessibility",
		"Try running detection from the project root directory",
	}

	return err
}

func NewWorkspaceRootNotFoundError(path string) *ProjectError {
	err := NewProjectError(
		ProjectErrorTypeWorkspace,
		PROJECT_TYPE_UNKNOWN,
		path,
		fmt.Sprintf("Could not determine workspace root for path '%s'", path),
		nil,
	)

	err.Suggestions = []string{
		"Ensure you're inside a valid project directory",
		"Check if version control markers (.git) are present",
		"Verify project structure contains recognizable marker files",
		"Try specifying the project root explicitly",
	}

	return err
}

func NewMultipleProjectRootsError(roots []string) *ProjectError {
	err := NewProjectError(
		ProjectErrorTypeStructure,
		PROJECT_TYPE_MIXED,
		strings.Join(roots, ", "),
		"Multiple project roots detected in workspace",
		nil,
	)

	err.Suggestions = []string{
		"Choose a specific project directory to work with",
		"Use workspace configuration to manage multiple projects",
		"Consider using a monorepo structure with proper organization",
		"Specify the primary project root explicitly",
	}

	err = err.WithMetadata("detected_roots", roots)
	return err
}

func NewUnsupportedProjectTypeError(projectType, path string) *ProjectError {
	err := NewProjectError(
		ProjectErrorTypeDetection,
		projectType,
		path,
		fmt.Sprintf("Project type '%s' is not supported", projectType),
		nil,
	)

	err.Suggestions = []string{
		"Check if the project type is correctly detected",
		"Verify project structure matches supported patterns",
		"Consider adding support for this project type",
		"Use a supported project structure (Go, Python, Node.js, Java, TypeScript)",
	}

	return err
}

func NewDetectionTimeoutError(path string, timeout time.Duration) *ProjectError {
	err := NewProjectError(
		ProjectErrorTypeTimeout,
		PROJECT_TYPE_UNKNOWN,
		path,
		fmt.Sprintf("Project detection timed out after %v", timeout),
		nil,
	)

	err.Suggestions = []string{
		"Increase detection timeout in configuration",
		"Reduce the scope of directories to scan",
		"Check for large directories or slow filesystem access",
		"Exclude unnecessary directories from detection",
	}

	err = err.WithMetadata("timeout_duration", timeout)
	return err
}

func IsRetryableProjectError(err error) bool {
	if pe, ok := err.(*ProjectError); ok {
		switch pe.Type {
		case ProjectErrorTypeTimeout:
			return true
		case ProjectErrorTypeFileSystem:
			return strings.Contains(strings.ToLower(pe.Message), "temporary") ||
				strings.Contains(strings.ToLower(pe.Message), "busy")
		}
	}

	if we, ok := err.(*WorkspaceError); ok {
		return strings.Contains(strings.ToLower(we.Details), "timeout") ||
			strings.Contains(strings.ToLower(we.Details), "temporary") ||
			strings.Contains(strings.ToLower(we.Details), "busy")
	}

	// Check error message for timeout patterns
	errorMsg := strings.ToLower(err.Error())
	return strings.Contains(errorMsg, "timeout") || 
		strings.Contains(errorMsg, "timed out") ||
		strings.Contains(errorMsg, "temporary")
}