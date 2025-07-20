package platform

import "fmt"

type ErrorCode string

const (
	ErrCodeUnsupportedPlatform     ErrorCode = "UNSUPPORTED_PLATFORM"
	ErrCodePackageManagerFailed    ErrorCode = "PACKAGE_MANAGER_FAILED"
	ErrCodeInsufficientPermissions ErrorCode = "INSUFFICIENT_PERMISSIONS"
)

type PlatformErrorType string

const (
	PlatformErrorTypeUnsupported PlatformErrorType = "unsupported_platform"

	PlatformErrorTypeDetection PlatformErrorType = "detection_failed"

	PlatformErrorTypeExecution PlatformErrorType = "execution_failed"

	PlatformErrorTypePackageManager PlatformErrorType = "package_manager_error"

	PlatformErrorTypePermission PlatformErrorType = "permission_error"

	PlatformErrorTypeNetwork PlatformErrorType = "network_error"
)

type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
)

type UserGuidance struct {
	Problem       string
	Suggestion    string
	RecoverySteps []string
	RequiredTools []string
	EstimatedTime string
	Documentation string
}

type PlatformError struct {
	Code            ErrorCode
	Type            PlatformErrorType
	Message         string
	UserMessage     string
	Cause           error
	Severity        Severity
	AutoRecoverable bool
	SystemInfo      map[string]interface{}
	UserGuidance    UserGuidance
}

func (e *PlatformError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

func (e *PlatformError) Unwrap() error {
	return e.Cause
}

func (e *PlatformError) Is(target error) bool {
	if t, ok := target.(*PlatformError); ok {
		return e.Type == t.Type
	}
	return false
}

func (e *PlatformError) GetUserGuidance() UserGuidance {
	return e.UserGuidance
}

func (e *PlatformError) WithSystemInfo(info map[string]interface{}) *PlatformError {
	if e.SystemInfo == nil {
		e.SystemInfo = make(map[string]interface{})
	}
	for k, v := range info {
		e.SystemInfo[k] = v
	}
	return e
}

func (e *PlatformError) WithCause(cause error) *PlatformError {
	e.Cause = cause
	return e
}

func NewPlatformError(code ErrorCode, message, problem, suggestion string) *PlatformError {
	return &PlatformError{
		Code:            code,
		Type:            PlatformErrorTypeDetection,
		Message:         message,
		Severity:        SeverityMedium,
		AutoRecoverable: false,
		SystemInfo:      make(map[string]interface{}),
		UserGuidance: UserGuidance{
			Problem:    problem,
			Suggestion: suggestion,
		},
	}
}

type ExecutionError struct {
	Command  string
	ExitCode int
	Stderr   string
	Cause    error
}

func (e *ExecutionError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("command '%s' failed with exit code %d: %s (caused by: %v)",
			e.Command, e.ExitCode, e.Stderr, e.Cause)
	}
	return fmt.Sprintf("command '%s' failed with exit code %d: %s",
		e.Command, e.ExitCode, e.Stderr)
}

func (e *ExecutionError) Unwrap() error {
	return e.Cause
}

func NewExecutionError(command string, exitCode int, stderr string, cause error) *ExecutionError {
	return &ExecutionError{
		Command:  command,
		ExitCode: exitCode,
		Stderr:   stderr,
		Cause:    cause,
	}
}

type PackageManagerError struct {
	Manager   string
	Operation string
	Package   string
	Message   string
	Cause     error
}

func (e *PackageManagerError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s failed to %s package '%s': %s (caused by: %v)",
			e.Manager, e.Operation, e.Package, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s failed to %s package '%s': %s",
		e.Manager, e.Operation, e.Package, e.Message)
}

func (e *PackageManagerError) Unwrap() error {
	return e.Cause
}

func NewPackageManagerError(manager, operation, pkg, message string, cause error) *PackageManagerError {
	return &PackageManagerError{
		Manager:   manager,
		Operation: operation,
		Package:   pkg,
		Message:   message,
		Cause:     cause,
	}
}

func NewUnsupportedPlatformError(platform string) *PlatformError {
	return &PlatformError{
		Code:            ErrCodeUnsupportedPlatform,
		Type:            PlatformErrorTypeUnsupported,
		Message:         fmt.Sprintf("Platform '%s' is not supported", platform),
		UserMessage:     fmt.Sprintf(UNSUPPORTED_PLATFORM_MESSAGE, platform),
		Severity:        SeverityCritical,
		AutoRecoverable: false,
		SystemInfo:      map[string]interface{}{"platform": platform},
		UserGuidance: UserGuidance{
			Problem:    fmt.Sprintf(UNSUPPORTED_PLATFORM_MESSAGE, platform),
			Suggestion: "Please use a supported platform (Linux, macOS, or Windows)",
		},
	}
}

func NewInsufficientPermissionsError(operation, path string) *PlatformError {
	return &PlatformError{
		Code:            ErrCodeInsufficientPermissions,
		Type:            PlatformErrorTypePermission,
		Message:         fmt.Sprintf("Insufficient permissions to %s: %s", operation, path),
		Severity:        SeverityHigh,
		AutoRecoverable: true,
		SystemInfo:      map[string]interface{}{"operation": operation, "path": path},
		UserGuidance: UserGuidance{
			Problem:    fmt.Sprintf("Unable to %s due to insufficient permissions", operation),
			Suggestion: "Try running with elevated privileges (sudo on Unix, Run as Administrator on Windows)",
			RecoverySteps: []string{
				"Check file/directory permissions",
				"Run with appropriate privileges",
				"Ensure the target location is writable",
			},
			EstimatedTime: "1-2 minutes",
		},
	}
}
