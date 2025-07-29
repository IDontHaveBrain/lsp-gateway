package workspace

import (
	"fmt"
	"lsp-gateway/internal/platform"
)

const (
	ErrCodePortAllocationFailed platform.ErrorCode = "PORT_ALLOCATION_FAILED"
	ErrCodePortNotAvailable     platform.ErrorCode = "PORT_NOT_AVAILABLE"
	ErrCodePortLockFailed       platform.ErrorCode = "PORT_LOCK_FAILED"
	ErrCodePortRegistryFailed   platform.ErrorCode = "PORT_REGISTRY_FAILED"
	ErrCodeWorkspacePathInvalid platform.ErrorCode = "WORKSPACE_PATH_INVALID"
	ErrCodePortRangeExhausted   platform.ErrorCode = "PORT_RANGE_EXHAUSTED"
	ErrCodePortFileError        platform.ErrorCode = "PORT_FILE_ERROR"
)

type PortErrorType string

const (
	PortErrorTypeAllocation   PortErrorType = "allocation_failed"
	PortErrorTypeAvailability PortErrorType = "availability_check_failed"
	PortErrorTypeLocking      PortErrorType = "locking_failed"
	PortErrorTypeRegistry     PortErrorType = "registry_failed"
	PortErrorTypeFileSystem   PortErrorType = "filesystem_error"
	PortErrorTypeWorkspace    PortErrorType = "workspace_error"
)

type PortError struct {
	Type          PortErrorType
	Port          int
	WorkspaceRoot string
	Message       string
	Metadata      map[string]interface{}
	Suggestions   []string
	Cause         error
}

func (e *PortError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s error for port %d at workspace %s: %s (caused by: %v)",
			e.Type, e.Port, e.WorkspaceRoot, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s error for port %d at workspace %s: %s",
		e.Type, e.Port, e.WorkspaceRoot, e.Message)
}

func (e *PortError) Unwrap() error {
	return e.Cause
}

func (e *PortError) Is(target error) bool {
	if t, ok := target.(*PortError); ok {
		return e.Type == t.Type
	}
	return false
}

func NewPortError(errorType PortErrorType, port int, workspaceRoot, message string, cause error) *PortError {
	return &PortError{
		Type:          errorType,
		Port:          port,
		WorkspaceRoot: workspaceRoot,
		Message:       message,
		Metadata:      make(map[string]interface{}),
		Suggestions:   make([]string, 0),
		Cause:         cause,
	}
}

func (e *PortError) WithMetadata(key string, value interface{}) *PortError {
	if e.Metadata == nil {
		e.Metadata = make(map[string]interface{})
	}
	e.Metadata[key] = value
	return e
}

func (e *PortError) WithSuggestions(suggestions []string) *PortError {
	e.Suggestions = suggestions
	return e
}

func (e *PortError) WithSuggestion(suggestion string) *PortError {
	e.Suggestions = append(e.Suggestions, suggestion)
	return e
}

func NewPortAllocationFailedError(workspaceRoot string, cause error) *PortError {
	err := NewPortError(
		PortErrorTypeAllocation,
		0,
		workspaceRoot,
		fmt.Sprintf("Failed to allocate port for workspace '%s'", workspaceRoot),
		cause,
	)

	err.Suggestions = []string{
		"Ensure the workspace path is valid and accessible",
		"Check if there are available ports in the range 8080-8100",
		"Verify file system permissions for port registry",
		"Try restarting the LSP gateway service",
	}

	return err
}

func NewPortNotAvailableError(port int, workspaceRoot string) *PortError {
	err := NewPortError(
		PortErrorTypeAvailability,
		port,
		workspaceRoot,
		fmt.Sprintf("Port %d is not available", port),
		nil,
	)

	err.Suggestions = []string{
		"The port may be in use by another process",
		"Check for conflicting services on the same port",
		"Try stopping other LSP gateway instances",
		"Wait a few moments and retry the operation",
	}

	return err
}

func NewPortLockFailedError(port int, workspaceRoot string, cause error) *PortError {
	err := NewPortError(
		PortErrorTypeLocking,
		port,
		workspaceRoot,
		fmt.Sprintf("Failed to acquire lock for port %d", port),
		cause,
	)

	err.Suggestions = []string{
		"Another process may have locked this port",
		"Check if lock files in ~/.lspg/ports/ are stale",
		"Verify file system permissions for lock directory",
		"Try releasing all ports and retrying",
	}

	return err
}

func NewPortRegistryFailedError(port int, workspaceRoot string, cause error) *PortError {
	err := NewPortError(
		PortErrorTypeRegistry,
		port,
		workspaceRoot,
		fmt.Sprintf("Failed to update port registry for port %d", port),
		cause,
	)

	err.Suggestions = []string{
		"Check file system permissions for ~/.lspg/ports/ directory",
		"Ensure sufficient disk space is available",
		"Verify the registry directory is not corrupted",
		"Try clearing the port registry and restarting",
	}

	return err
}

func NewWorkspacePathInvalidError(workspaceRoot string, cause error) *PortError {
	err := NewPortError(
		PortErrorTypeWorkspace,
		0,
		workspaceRoot,
		fmt.Sprintf("Invalid workspace path: %s", workspaceRoot),
		cause,
	)

	err.Suggestions = []string{
		"Ensure the workspace path exists and is accessible",
		"Check file system permissions for the workspace directory",
		"Verify the path is an absolute path",
		"Ensure the directory is not a symbolic link to a non-existent location",
	}

	return err
}

func NewPortRangeExhaustedError(workspaceRoot string) *PortError {
	err := NewPortError(
		PortErrorTypeAllocation,
		0,
		workspaceRoot,
		fmt.Sprintf("No available ports in range %d-%d", PortRangeStart, PortRangeEnd),
		nil,
	)

	err.Suggestions = []string{
		"Stop unused LSP gateway instances to free up ports",
		"Check for processes using ports in the 8080-8100 range",
		"Consider expanding the port range if needed",
		"Clean up stale port registry entries",
	}

	err = err.WithMetadata("port_range_start", PortRangeStart)
	err = err.WithMetadata("port_range_end", PortRangeEnd)
	err = err.WithMetadata("max_concurrent_ports", MaxConcurrentPorts)

	return err
}

func NewPortFileError(workspaceRoot string, operation string, cause error) *PortError {
	err := NewPortError(
		PortErrorTypeFileSystem,
		0,
		workspaceRoot,
		fmt.Sprintf("Port file %s operation failed for workspace %s", operation, workspaceRoot),
		cause,
	)

	err.Suggestions = []string{
		"Check file system permissions for the workspace directory",
		"Ensure the workspace directory is writable",
		"Verify sufficient disk space is available",
		"Check if the .lspg-port file is locked by another process",
	}

	err = err.WithMetadata("operation", operation)
	return err
}

func IsRetryablePortError(err error) bool {
	if pe, ok := err.(*PortError); ok {
		switch pe.Type {
		case PortErrorTypeAvailability:
			return true
		case PortErrorTypeLocking:
			return true
		case PortErrorTypeFileSystem:
			return pe.Cause != nil && 
				(fmt.Sprintf("%v", pe.Cause) == "file exists" || 
				 fmt.Sprintf("%v", pe.Cause) == "resource temporarily unavailable")
		}
	}
	return false
}