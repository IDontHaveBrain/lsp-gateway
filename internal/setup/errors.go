package setup

import (
	"fmt"
	"lsp-gateway/internal/platform"
	"strings"
	"time"
)

const (
	ErrCodeCommandFailed           platform.ErrorCode = "COMMAND_FAILED"
	ErrCodeCommandNotFound         platform.ErrorCode = "COMMAND_NOT_FOUND"
	ErrCodeCommandTimeout          platform.ErrorCode = "COMMAND_TIMEOUT"
	ErrCodeDownloadFailed          platform.ErrorCode = "DOWNLOAD_FAILED"
	ErrCodeNetworkUnavailable      platform.ErrorCode = "NETWORK_UNAVAILABLE"
	ErrCodeServerInstallFailed     platform.ErrorCode = "SERVER_INSTALL_FAILED"
	ErrCodeVersionValidationFailed platform.ErrorCode = "VERSION_VALIDATION_FAILED"

	ErrCodeNodeJSNotFound         platform.ErrorCode = "NODEJS_NOT_FOUND"
	ErrCodeNPMNotFound            platform.ErrorCode = "NPM_NOT_FOUND"
	ErrCodeNPMFunctionalityFailed platform.ErrorCode = "NPM_FUNCTIONALITY_FAILED"
	ErrCodeGlobalInstallFailed    platform.ErrorCode = "GLOBAL_INSTALL_FAILED"
	ErrCodeVersionIncompatible    platform.ErrorCode = "VERSION_INCOMPATIBLE"
	ErrCodePermissionDenied       platform.ErrorCode = "PERMISSION_DENIED"
	ErrCodeRuntimeNotFound        platform.ErrorCode = "RUNTIME_NOT_FOUND"
	ErrCodeConfigValidationFailed platform.ErrorCode = "CONFIG_VALIDATION_FAILED"
)

type SetupErrorType string

const (
	SetupErrorTypeDetection SetupErrorType = "detection_failed"

	SetupErrorTypeInstallation SetupErrorType = "installation_failed"

	SetupErrorTypeVerification SetupErrorType = "verification_failed"

	SetupErrorTypeConfiguration SetupErrorType = "configuration_error"

	SetupErrorTypeVersion SetupErrorType = "version_error"

	SetupErrorTypeDependency SetupErrorType = "dependency_error"

	SetupErrorTypeTimeout SetupErrorType = "timeout_error"
)

type SetupError struct {
	Type         SetupErrorType
	Component    string
	Message      string
	Version      string
	TargetPath   string
	Metadata     map[string]interface{}
	Alternatives []string
	Cause        error
}

func (e *SetupError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s error for %s: %s (caused by: %v)",
			e.Type, e.Component, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s error for %s: %s", e.Type, e.Component, e.Message)
}

func (e *SetupError) Unwrap() error {
	return e.Cause
}

func (e *SetupError) Is(target error) bool {
	if t, ok := target.(*SetupError); ok {
		return e.Type == t.Type
	}
	return false
}

func NewSetupError(errorType SetupErrorType, component, message string, cause error) *SetupError {
	return &SetupError{
		Type:         errorType,
		Component:    component,
		Message:      message,
		Metadata:     make(map[string]interface{}),
		Alternatives: make([]string, 0),
		Cause:        cause,
	}
}

func (e *SetupError) WithVersion(version string) *SetupError {
	e.Version = version
	return e
}

func (e *SetupError) WithTargetPath(path string) *SetupError {
	e.TargetPath = path
	return e
}

func (e *SetupError) WithMetadata(key string, value interface{}) *SetupError {
	if e.Metadata == nil {
		e.Metadata = make(map[string]interface{})
	}
	e.Metadata[key] = value
	return e
}

type DetectionError struct {
	Runtime string
	Phase   string // "version", "path", "execution"
	Output  string
	Cause   error
}

func (e *DetectionError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("detection of %s failed during %s phase: %s (caused by: %v)",
			e.Runtime, e.Phase, e.Output, e.Cause)
	}
	return fmt.Sprintf("detection of %s failed during %s phase: %s",
		e.Runtime, e.Phase, e.Output)
}

func (e *DetectionError) Unwrap() error {
	return e.Cause
}

func NewDetectionError(runtime, phase, output string, cause error) *DetectionError {
	return &DetectionError{
		Runtime: runtime,
		Phase:   phase,
		Output:  output,
		Cause:   cause,
	}
}

type VersionError struct {
	Runtime      string
	Required     string
	Installed    string
	Incompatible bool
}

func (e *VersionError) Error() string {
	if e.Incompatible {
		return fmt.Sprintf("runtime '%s' version %s is incompatible, minimum version %s required",
			e.Runtime, e.Installed, e.Required)
	}
	return fmt.Sprintf("runtime '%s' version %s does not meet minimum requirement of %s",
		e.Runtime, e.Installed, e.Required)
}

func NewVersionError(runtime, required, installed string, incompatible bool) *VersionError {
	return &VersionError{
		Runtime:      runtime,
		Required:     required,
		Installed:    installed,
		Incompatible: incompatible,
	}
}

type ConfigurationError struct {
	File        string
	Field       string
	Value       string
	Expected    string
	ValidValues []string
	Message     string
	Cause       error
}

func (e *ConfigurationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("configuration error in %s field '%s' with value '%s': %s (caused by: %v)",
			e.File, e.Field, e.Value, e.Message, e.Cause)
	}
	return fmt.Sprintf("configuration error in %s field '%s' with value '%s': %s",
		e.File, e.Field, e.Value, e.Message)
}

func (e *ConfigurationError) Unwrap() error {
	return e.Cause
}

func NewConfigurationError(file, field, value, message string, cause error) *ConfigurationError {
	return &ConfigurationError{
		File:        file,
		Field:       field,
		Value:       value,
		ValidValues: make([]string, 0),
		Message:     message,
		Cause:       cause,
	}
}

func (e *ConfigurationError) WithExpected(expected string) *ConfigurationError {
	e.Expected = expected
	return e
}

func (e *ConfigurationError) WithValidValues(values []string) *ConfigurationError {
	e.ValidValues = values
	return e
}

type CommandError struct {
	Code       platform.ErrorCode
	Command    string
	Args       []string
	ExitCode   int
	Signal     string
	Duration   time.Duration
	WorkingDir string
	Stdout     string
	Stderr     string
	Cause      error
}

func (e *CommandError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("command '%s %s' failed with exit code %d: %v",
			e.Command, strings.Join(e.Args, " "), e.ExitCode, e.Cause)
	}
	return fmt.Sprintf("command '%s %s' failed with exit code %d",
		e.Command, strings.Join(e.Args, " "), e.ExitCode)
}

func (e *CommandError) Unwrap() error {
	return e.Cause
}

func (e *CommandError) WithDuration(duration time.Duration) *CommandError {
	e.Duration = duration
	return e
}

func (e *CommandError) WithSignal(signal string) *CommandError {
	e.Signal = signal
	return e
}

func (e *CommandError) WithWorkingDir(dir string) *CommandError {
	e.WorkingDir = dir
	return e
}

func (e *CommandError) WithOutput(stdout, stderr string) *CommandError {
	e.Stdout = stdout
	e.Stderr = stderr
	return e
}

func NewCommandError(code platform.ErrorCode, command string, args []string, exitCode int) *CommandError {
	return &CommandError{
		Code:     code,
		Command:  command,
		Args:     args,
		ExitCode: exitCode,
	}
}

type InstallationError struct {
	Code      platform.ErrorCode
	Component string
	Message   string
	Cause     error
}

type ValidationError struct {
	Code      platform.ErrorCode
	Component string
	Expected  string
	Actual    string
	Message   string
	Cause     error
}

func NewRuntimeNotFoundError(runtime string) *SetupError {
	err := NewSetupError(
		SetupErrorTypeDetection,
		runtime,
		fmt.Sprintf("Runtime '%s' not found on system", runtime),
		nil,
	)

	switch runtime {
	case "go":
		err.Alternatives = []string{
			"Install Go from https://golang.org/dl/",
			"Use package manager: apt install golang-go (Ubuntu/Debian)",
			"Use package manager: brew install go (macOS)",
			"Download and extract Go binary release",
		}
	case "python":
		err.Alternatives = []string{
			"Install Python from https://python.org/downloads/",
			"Use package manager: apt install python3 (Ubuntu/Debian)",
			"Use package manager: brew install python (macOS)",
			"Install via pyenv: pyenv install 3.11.0",
		}
	case "nodejs":
		err.Alternatives = []string{
			"Install Node.js from https://nodejs.org/",
			"Use package manager: apt install nodejs npm (Ubuntu/Debian)",
			"Use package manager: brew install node (macOS)",
			"Install via nvm: nvm install node",
		}
	case "java":
		err.Alternatives = []string{
			"Install OpenJDK from https://openjdk.org/",
			"Use package manager: apt install openjdk-17-jdk (Ubuntu/Debian)",
			"Use package manager: brew install openjdk (macOS)",
			"Install Oracle JDK from https://oracle.com/java/",
		}
	default:
		err.Alternatives = []string{
			fmt.Sprintf("Check if '%s' is installed and in PATH", runtime),
			"Verify installation command and try again",
			"Consult runtime-specific documentation",
		}
	}

	return err
}

func NewConfigValidationFailedError(field, value, message string) *ConfigurationError {
	return NewConfigurationError(
		"config.yaml",
		field,
		value,
		message,
		nil,
	)
}

func NewNetworkError(operation, url string, cause error) *SetupError {
	message := fmt.Sprintf("Network operation '%s' failed for URL '%s'", operation, url)
	return NewSetupError(
		SetupErrorTypeInstallation,
		"network",
		message,
		cause,
	)
}

func IsRetryableError(err error) bool {
	if se, ok := err.(*SetupError); ok {
		switch se.Type {
		case SetupErrorTypeTimeout:
			return true
		case SetupErrorTypeInstallation:
			return strings.Contains(strings.ToLower(se.Message), "network") ||
				strings.Contains(strings.ToLower(se.Message), "download") ||
				strings.Contains(strings.ToLower(se.Message), "timeout")
		}
	}
	return IsTemporaryError(err)
}
