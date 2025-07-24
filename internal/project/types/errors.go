package types

import (
	"fmt"
	"strings"
	"lsp-gateway/internal/platform"
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

func (e *ProjectError) ToPlatformError() *platform.PlatformError {

	metadata := make(map[string]interface{})
	if e.Metadata != nil {
		for k, v := range e.Metadata {
			metadata[k] = v
		}
	}
	metadata["project_type"] = e.ProjectType
	metadata["project_root"] = e.ProjectRoot
	metadata["error_type"] = string(e.Type)
	if e.Path != "" {
		metadata["path"] = e.Path
	}

	return &platform.PlatformError{
		Code:        ErrCodeProjectDetectionFailed,
		Message:     e.Message,
		UserMessage: fmt.Sprintf("Project %s detection failed at %s", e.ProjectType, e.ProjectRoot),
		Cause:       e.Cause,
		SystemInfo:  metadata,
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

func (e *ProjectError) WithSuggestion(suggestion string) *ProjectError {
	e.Suggestions = append(e.Suggestions, suggestion)
	return e
}

func (e *ProjectError) WithSuggestions(suggestions []string) *ProjectError {
	e.Suggestions = suggestions
	return e
}

// Error constructors

func NewProjectError(errorType ProjectErrorType, projectType, path, message string, cause error) *ProjectError {
	return &ProjectError{
		Type:        errorType,
		ProjectType: projectType,
		ProjectRoot: path,
		Message:     message,
		Path:        path,
		Metadata:    make(map[string]interface{}),
		Suggestions: []string{},
		Cause:       cause,
	}
}

func NewDetectionError(projectType, operation, path, message string, cause error) *ProjectError {
	err := NewProjectError(ProjectErrorTypeDetection, projectType, path, message, cause)
	err = err.WithMetadata("operation", operation)
	return err
}

func NewValidationError(projectType, message string, cause error) *ProjectError {
	return NewProjectError(ProjectErrorTypeValidation, projectType, "", message, cause)
}

func NewProjectNotFoundError(path string) *ProjectError {
	return NewProjectError(ProjectErrorTypeDetection, PROJECT_TYPE_UNKNOWN, path,
		"No supported project types detected", nil).
		WithSuggestion("Ensure the directory contains project files like go.mod, package.json, setup.py, pom.xml, etc.").
		WithSuggestion("Check that the path points to a valid project directory")
}

func NewWorkspaceRootNotFoundError(path string) *ProjectError {
	return NewProjectError(ProjectErrorTypeWorkspace, PROJECT_TYPE_UNKNOWN, path,
		"Could not determine workspace root", nil).
		WithSuggestion("Initialize a version control system (git) in your project").
		WithSuggestion("Ensure project marker files are present")
}

// ValidationError is a specialized error for project validation
type ValidationError struct {
	*ProjectError
	ConfigurationErrors []string
	StructureIssues    []string
	DependencyIssues   []string
}

func (v *ValidationError) Error() string {
	issues := []string{}
	if len(v.ConfigurationErrors) > 0 {
		issues = append(issues, fmt.Sprintf("Configuration: %s", strings.Join(v.ConfigurationErrors, ", ")))
	}
	if len(v.StructureIssues) > 0 {
		issues = append(issues, fmt.Sprintf("Structure: %s", strings.Join(v.StructureIssues, ", ")))
	}
	if len(v.DependencyIssues) > 0 {
		issues = append(issues, fmt.Sprintf("Dependencies: %s", strings.Join(v.DependencyIssues, ", ")))
	}
	
	if len(issues) > 0 {
		return fmt.Sprintf("%s - Issues: %s", v.ProjectError.Error(), strings.Join(issues, "; "))
	}
	return v.ProjectError.Error()
}

func (v *ValidationError) WithConfigurationErrors(errors []string) *ValidationError {
	v.ConfigurationErrors = append(v.ConfigurationErrors, errors...)
	return v
}

func (v *ValidationError) WithStructureIssues(issues []string) *ValidationError {
	v.StructureIssues = append(v.StructureIssues, issues...)
	return v
}

func (v *ValidationError) WithDependencyIssues(issues []string) *ValidationError {
	v.DependencyIssues = append(v.DependencyIssues, issues...)
	return v
}

func NewValidationErrorFromProjectError(projectErr *ProjectError) *ValidationError {
	return &ValidationError{
		ProjectError:        projectErr,
		ConfigurationErrors: []string{},
		StructureIssues:    []string{},
		DependencyIssues:   []string{},
	}
}