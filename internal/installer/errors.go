package installer

import "fmt"

type InstallerErrorType string

const (
	InstallerErrorTypeNotFound InstallerErrorType = "not_found"

	InstallerErrorTypeInstallation InstallerErrorType = "installation_failed"

	InstallerErrorTypeVerification InstallerErrorType = "verification_failed"

	InstallerErrorTypeRuntime InstallerErrorType = "runtime_error"

	InstallerErrorTypePermission InstallerErrorType = "permission_error"

	InstallerErrorTypeNetwork InstallerErrorType = "network_error"

	InstallerErrorTypeTimeout InstallerErrorType = "timeout_error"

	InstallerErrorTypeUnsupported InstallerErrorType = "unsupported"

	InstallerErrorTypeDependency InstallerErrorType = "dependency_error"

	InstallerErrorTypeVersionConflict InstallerErrorType = "version_conflict"

	InstallerErrorTypeUnknown InstallerErrorType = "unknown_error"
)

type InstallerError struct {
	Type      InstallerErrorType
	Component string
	Message   string
	Cause     error
}

func (e *InstallerError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s error for %s: %s (caused by: %v)",
			e.Type, e.Component, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s error for %s: %s", e.Type, e.Component, e.Message)
}

func (e *InstallerError) Unwrap() error {
	return e.Cause
}

func (e *InstallerError) Is(target error) bool {
	if t, ok := target.(*InstallerError); ok {
		return e.Type == t.Type
	}
	return false
}

func NewInstallerError(errorType InstallerErrorType, component, message string, cause error) *InstallerError {
	return &InstallerError{
		Type:      errorType,
		Component: component,
		Message:   message,
		Cause:     cause,
	}
}

type RuntimeDependencyError struct {
	Runtime   string
	Required  string
	Installed string
	Missing   bool
}

func (e *RuntimeDependencyError) Error() string {
	if e.Missing {
		return fmt.Sprintf("runtime '%s' is required but not installed", e.Runtime)
	}
	return fmt.Sprintf("runtime '%s' version %s is required, but version %s is installed",
		e.Runtime, e.Required, e.Installed)
}

func NewRuntimeDependencyError(runtime, required, installed string, missing bool) *RuntimeDependencyError {
	return &RuntimeDependencyError{
		Runtime:   runtime,
		Required:  required,
		Installed: installed,
		Missing:   missing,
	}
}

type InstallationError struct {
	Component string
	Phase     string // "download", "install", "configure", "verify"
	ExitCode  int
	Output    string
	Stderr    string
	Cause     error
}

func (e *InstallationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("installation of %s failed during %s phase (exit code %d): %s (caused by: %v)",
			e.Component, e.Phase, e.ExitCode, e.Stderr, e.Cause)
	}
	return fmt.Sprintf("installation of %s failed during %s phase (exit code %d): %s",
		e.Component, e.Phase, e.ExitCode, e.Stderr)
}

func (e *InstallationError) Unwrap() error {
	return e.Cause
}

func NewInstallationError(component, phase string, exitCode int, stderr string, cause error) *InstallationError {
	return &InstallationError{
		Component: component,
		Phase:     phase,
		ExitCode:  exitCode,
		Stderr:    stderr,
		Cause:     cause,
	}
}

type VerificationError struct {
	Component string
	Check     string // "existence", "version", "execution", "communication"
	Expected  string
	Actual    string
	Cause     error
}

func (e *VerificationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("verification of %s failed during %s check: expected %s, got %s (caused by: %v)",
			e.Component, e.Check, e.Expected, e.Actual, e.Cause)
	}
	return fmt.Sprintf("verification of %s failed during %s check: expected %s, got %s",
		e.Component, e.Check, e.Expected, e.Actual)
}

func (e *VerificationError) Unwrap() error {
	return e.Cause
}

func NewVerificationError(component, check, expected, actual string, cause error) *VerificationError {
	return &VerificationError{
		Component: component,
		Check:     check,
		Expected:  expected,
		Actual:    actual,
		Cause:     cause,
	}
}

type DependencyValidationError struct {
	ServerName    string
	MissingDeps   []string
	VersionIssues []string
	Cause         error
}
